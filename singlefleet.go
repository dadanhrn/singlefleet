// Package singlefleet provides a batching mechanism that can be incorporated
// in any simple item fetch routine. Helps in scenarios where network round-
// trip overhead, database CPU time, etc. are significant constraints.
package singlefleet

import (
	"sync"
	"time"
)

// A Job defines a batched fetch operation. ids contains the IDs of items to be
// fetched in a given batch (guaranteed to be unique). Resulting values from
// the fetch operation, if found/exists, must be mapped to their corresponding
// ID. Error returned by this operation will be passed to the calling
// Fetch(es).
type Job func(ids []string) (vals map[string]interface{}, err error)

// batch represents a collection of fetch operations.
type batch struct {
	vals map[string]interface{}
	err  error

	ids  []string
	mu   sync.Mutex
	wg   sync.WaitGroup
	csig chan struct{}
}

// A Fetcher represents a fetch operation and contains the batch pool. Must be
// created with NewFetcher.
type Fetcher struct {
	maxw time.Duration
	maxb int

	mu sync.Mutex
	m  map[string]*batch
	b  *batch
	f  Job
}

// NewFetcher creates a new Fetcher. It holds the execution of jobs in the
// batch pool until at least maxWait has passed, there are at least maxBatch
// requested items in the batch pool, or FetchNow is called; whichever comes
// first.
//
// To ignore the maxWait rule simply set a sufficiently long duration.
// Likewise, to ignore the maxBatch rule simply set a sufficiently large
// integer value.
func NewFetcher(job Job, maxWait time.Duration, maxBatch int) *Fetcher {
	return &Fetcher{
		f:    job,
		maxw: maxWait,
		maxb: maxBatch,
		m:    make(map[string]*batch),
	}
}

// Fetch places a fetch job in the batch pool and returns the result of the
// operation.
func (fc *Fetcher) Fetch(id string) (val interface{}, ok bool, err error) {
	fc.mu.Lock()

	// Check if given ID is already queued in current batch
	if b, ok := fc.m[id]; ok {
		// It is, then just wait for call
		fc.mu.Unlock()
		b.wg.Wait()
		val, ok = b.vals[id]
		return val, ok, b.err
	}

	// Check if this group has a pending batch
	if fc.b != nil {
		// It does, then init first call for given ID
		b := fc.b
		fc.m[id] = b
		fc.mu.Unlock()
		b.mu.Lock()
		b.ids = append(b.ids, id)
		if len(b.ids) >= fc.maxb {
			fc.mu.Lock()
			b.csig <- struct{}{}
		}
		b.mu.Unlock()

		// Wait for call
		b.wg.Wait()

		// Cleanup
		fc.mu.Lock()
		delete(fc.m, id)
		fc.mu.Unlock()

		val, ok = b.vals[id]
		return val, ok, b.err
	}

	// Init first call of its batch
	b := new(batch)
	b.mu.Lock()
	b.wg.Add(1)
	b.ids = make([]string, 0, fc.maxb)
	b.ids = append(b.ids, id)
	b.csig = make(chan struct{})
	b.mu.Unlock()
	fc.b = b
	fc.m[id] = b
	fc.mu.Unlock()

	// Wait for signal
	t := time.NewTimer(fc.maxw)
	select {
	case <-t.C:
		fc.mu.Lock()
		close(b.csig)
	case <-b.csig:
		t.Stop()
	}
	fc.b = nil
	fc.mu.Unlock()

	// Do call
	fc.doFetch(b)
	b.wg.Done()

	// Cleanup
	fc.mu.Lock()
	delete(fc.m, id)
	fc.mu.Unlock()

	val, ok = b.vals[id]
	return val, ok, b.err
}

// FetchNow forces the current pending job batch to be executed, disregarding
// the maxWait and maxBatch rules.
func (fc *Fetcher) FetchNow() bool {
	fc.mu.Lock()

	// Return immediately if there is no pending batch
	if fc.b == nil {
		fc.mu.Unlock()
		return false
	}

	// Send signal to execute fetch immediately
	fc.b.csig <- struct{}{}

	return true
}

func (fc *Fetcher) doFetch(c *batch) {
	// TODO: handle panic?
	c.vals, c.err = fc.f(c.ids)
}
