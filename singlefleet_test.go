package singlefleet

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFetchSingle(t *testing.T) {
	f := NewFetcher(func(ids []string) (map[string]interface{}, error) {
		res := make(map[string]interface{})
		for _, id := range ids {
			res[id] = strings.ToUpper(id)
		}
		return res, nil
	}, 100*time.Millisecond, 1)
	v, ok, err := f.Fetch("a")
	if err != nil {
		t.Errorf("Fetch error = %v", err)
	}
	if ok != true {
		t.Errorf("Fetch ok = %v, want true", ok)
	}
	if got, want := fmt.Sprintf("%v (%T)", v, v), "A (string)"; got != want {
		t.Errorf("Fetch = %v, want %v", got, want)
	}
}

func TestFetchErr(t *testing.T) {
	someErr := errors.New("error")
	f := NewFetcher(func(ids []string) (map[string]interface{}, error) {
		return nil, someErr
	}, 100*time.Millisecond, 1)
	v, ok, err := f.Fetch("a")
	if err != someErr {
		t.Errorf("Fetch error = %v; want someError %v", err, someErr)
	}
	if ok != false {
		t.Errorf("Fetch ok = %v, want false", ok)
	}
	if v != nil {
		t.Errorf("unexpected non-nil value %#v", v)
	}
}

func TestFetchSingleBatchByMaxBatch(t *testing.T) {
	var wg sync.WaitGroup
	var ncall = 0
	var db = map[int]string{0: "abc", 2: "def"}
	f := NewFetcher(func(ids []string) (map[string]interface{}, error) {
		ncall += 1
		res := make(map[string]interface{})
		for _, idstr := range ids {
			id, _ := strconv.Atoi(idstr)
			if v, ok := db[id]; ok {
				res[idstr] = v
			}
		}
		time.Sleep(200 * time.Millisecond) // should be long enough to wait for all Fetches
		return res, nil
	}, 5*time.Second, 3)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(id int) {
			retv, retok, err := f.Fetch(strconv.Itoa(id))
			if err != nil {
				t.Errorf("Fetch error = %v", err)
			}
			dbv, dbok := db[id]
			if retok != dbok {
				t.Errorf("Fetch ok = %v, want %v", retok, dbok)
			}
			if got, want := fmt.Sprintf("%v (%T)", retv, retv), fmt.Sprintf("%v (%T)", dbv, dbv); dbok && got != want {
				t.Errorf("Fetch = %v, want %v", got, want)
			}
			wg.Done()
		}(i % 3)
	}
	wg.Wait()
	if ncall != 1 {
		t.Errorf("Fetch called %v times (expected once)", ncall)
	}
}

func TestFetchSingleBatchByMaxWait(t *testing.T) {
	var wg sync.WaitGroup
	var ncall = 0
	var db = map[int]string{0: "abc", 2: "def"}
	f := NewFetcher(func(ids []string) (map[string]interface{}, error) {
		ncall += 1
		res := make(map[string]interface{})
		for _, idstr := range ids {
			id, _ := strconv.Atoi(idstr)
			if v, ok := db[id]; ok {
				res[idstr] = v
			}
		}
		return res, nil
	}, 200*time.Millisecond, 1000)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(id int) {
			retv, retok, err := f.Fetch(strconv.Itoa(id))
			if err != nil {
				t.Errorf("Fetch error = %v", err)
			}
			dbv, dbok := db[id]
			if retok != dbok {
				t.Errorf("Fetch ok = %v, want %v", retok, dbok)
			}
			if got, want := fmt.Sprintf("%v (%T)", retv, retv), fmt.Sprintf("%v (%T)", dbv, dbv); dbok && got != want {
				t.Errorf("Fetch = %v, want %v", got, want)
			}
			wg.Done()
		}(i % 3)
	}
	wg.Wait()
	if ncall != 1 {
		t.Errorf("Fetch called %v times (expected once)", ncall)
	}
}

func TestFetchNow(t *testing.T) {
	var db = map[int]string{0: "abc", 2: "def"}
	f := NewFetcher(func(ids []string) (map[string]interface{}, error) {
		res := make(map[string]interface{})
		for _, idstr := range ids {
			id, _ := strconv.Atoi(idstr)
			if v, ok := db[id]; ok {
				res[idstr] = v
			}
		}
		return res, nil
	}, 5*time.Second, 1000)
	go func() {
		time.Sleep(100 * time.Millisecond)
		fnok := f.FetchNow()
		if fnok != true {
			t.Errorf("FetchNow = %v, want true", fnok)
		}
	}()
	id := 0
	retv, retok, err := f.Fetch(strconv.Itoa(id))
	if err != nil {
		t.Errorf("Fetch error = %v", err)
	}
	dbv, dbok := db[id]
	if retok != dbok {
		t.Errorf("Fetch ok = %v, want %v", retok, dbok)
	}
	if got, want := fmt.Sprintf("%v (%T)", retv, retv), fmt.Sprintf("%v (%T)", dbv, dbv); dbok && got != want {
		t.Errorf("Fetch = %v, want %v", got, want)
	}
}

func TestFetchNowNoQueue(t *testing.T) {
	f := NewFetcher(func(ids []string) (map[string]interface{}, error) {
		return make(map[string]interface{}), nil
	}, 5*time.Second, 1000)
	fnok := f.FetchNow()
	if fnok != false {
		t.Errorf("FetchNow = %v, want false", fnok)
	}
}
