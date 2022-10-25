# Singlefleet

Batching mechanism for your simple item fetch routines.

## Use cases
Where
- network round-trip overhead,
- number of open connections,
- etc.
are significant constraints for your application.

## Usage
Example below demonstrates how to use singlefleet in an HTTP API that fetches employee info by employee ID.
```go
package main

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "time"

    "github.com/dadanhrn/singlefleet"
    "github.com/lib/pq"
)

type Employee struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Salary       int       `json:"salary"`
	StartingDate time.Time `json:"starting_date"`
}

func main() {
    // Initialize PostgreSQL connection
    db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		fmt.Println(err)
		return
	}

    // Initialize singlefleet fetcher
    // (define fetch operation and batching params)
    sf := singlefleet.NewFetcher(func(ids []string) (map[string]interface{}, error) {
        // Fetch all IDs in one go using PostgreSQL's ANY()
        rows, err := db.Query(`SELECT id, name, salary, starting_date FROM employee WHERE id=ANY($1)`, pq.Array(ids))
        if err != nil {
            return nil, err
        }

        // Map results to their corresponding IDs
        result := make(map[string]interface{})
        for rows.Next() {
            var emp Employee
            if err := rows.Scan(&emp.ID, &emp.Name, &emp.Salary, &emp.StartingDate); err != nil {
                continue
            }
            result[emp.ID] = emp
        }

        return result, nil
    }, 200 * time.Millisecond, 10)

    // Define HTTP handler
    http.HandleFunc("/employee", func(w http.ResponseWriter, r *http.Request) {
        employeeID := r.URL.Query().Get("id")
        emp, ok, err := sf.Fetch(employeeID)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        if !ok {
            w.WriteHeader(http.StatusNotFound)
            return
        }

        body, _ := json.Marshal(data)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
        return
    })

    http.ListenAndServe(":8080", nil)
}
```

## How it works
Basically singlefleet is like [singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight), except that on top of the duplicate call suppression, singlefleet also waits for multiple calls and executes them in an execution batch. So, for example, if your call fetches an `Employee` entity, you define the call as if you are fetching multiple `Employee`s.

Not just database calls, singlefleet can also fetch data from anything that supports _multiple get_ (i.e. fetching "multiple rows" in one go) such as
- [Redis](https://redis.io/commands/mget/)
- [Elasticsearch](https://www.elastic.co/guide/en/elasticsearch/reference/current/docs-multi-get.html)
- [MongoDB](https://www.mongodb.com/docs/manual/reference/operator/query/in/)
- specially-designed REST API endpoints
- etc.

One thing to note is that the _multiple get_ mechanism should be natively supported by the data source. If you are, for example, simulating the _multiple get_ by executing multiple _single gets_ in multiple goroutines, then you are missing the entire point of this library.

singlefleet's execution batching takes two params:
- `maxWait` or maximum waiting time, and
- `maxBatch` or maximum batch size.

`Fetcher` adds all `Fetch` calls to its batch pool. Fetch operation will not be executed until
- at least `maxWait` has passed since the first `Fetch` call in current batch,
- there are at least `maxBatch` calls in current batch, or
- `FetchNow` is called;

whichever comes first. If a given call (identified by their respective `id` argument) is currently in-flight (called and not yet returned), another incoming identical call will not initiate a new batch or be added to a newer pending batch; it will just wait for the result of the previous call (think singleflight).

## Future plans
- Panic handling
- Make use of generics to enable custom types for `id` and return value