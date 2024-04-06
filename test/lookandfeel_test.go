package test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/davidroman0O/comfylite3"
)

func TestMemory(t *testing.T) {

	var superComfy *comfylite3.ComfyDB
	var err error
	if superComfy, err = comfylite3.Comfy(comfylite3.WithMemory()); err != nil {
		panic(err)
	}

	defer superComfy.Close()
	ticket := superComfy.New(func(db *sql.DB) (interface{}, error) {
		_, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
		return nil, err
	})
	<-superComfy.WaitFor(ticket)

	done := make(chan struct{})

	go func() {
		random := rand.New(rand.NewSource(time.Now().UnixNano()))
		for range 10000 {
			ticket := superComfy.New(func(db *sql.DB) (interface{}, error) {
				_, err := db.Exec("INSERT INTO users (name) VALUES (?)", fmt.Sprintf("user%d", 1))
				return nil, err
			})
			<-superComfy.WaitFor(ticket)
			// simulate random insert with a random sleep
			time.Sleep(time.Duration(random.Intn(5)) * time.Millisecond)
		}
		done <- struct{}{}
	}()

	// Let's measure how many records per second
	metrics := []int{}

	ticker := time.NewTicker(1 * time.Second)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	compute := true
	for compute {
		select {
		case <-done:
			ticker.Stop()
			cancel()
			compute = false
		case <-ctx.Done():
			ticker.Stop()
			cancel()
			compute = false
		case <-ticker.C:
			ticket := superComfy.New(func(db *sql.DB) (interface{}, error) {
				rows, err := db.Query("SELECT COUNT(*) FROM users")
				if err != nil {
					return nil, err
				}
				defer rows.Close()

				var count int
				if rows.Next() {
					err = rows.Scan(&count)
					if err != nil {
						return nil, err
					}
				}

				return count, err
			})
			result := <-superComfy.WaitFor(ticket)
			metrics = append(metrics, result.(int))
		}
	}

	total := 0
	for _, value := range metrics {
		total += value
	}
	average := float64(total) / float64(len(metrics))
	fmt.Printf("Average: %.2f\n", average)
}
