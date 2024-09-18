package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Result struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

func main() {
	http.HandleFunc("/concurrent", handleConcurrent)
	http.ListenAndServe(":8080", nil)
}

func handleConcurrent(w http.ResponseWriter, r *http.Request) {
	results := make([]Result, 10)
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			// Simulate different request durations
			duration := time.Second
			if id%2 == 0 {
				duration = 5 * time.Second
			}

			value, err := mockedHTTPCall(reqCtx, duration)
			if err != nil {
				results[id] = Result{ID: id, Value: fmt.Sprintf("Error: %v", err)}
			} else {
				results[id] = Result{ID: id, Value: value}
			}
		}(i)
	}

	fmt.Println("waitforquorum")
	time.Sleep(time.Second * 1)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "waitforquorum complete"})
	time.Sleep(time.Second * 3)
	w.(http.Flusher).Flush()
	fmt.Println("flushed!")
	
	
	// time.Sleep(time.Second * 10)
	wg.Wait()
	fmt.Println("all goroutines complete")
	fmt.Println(results)
}

func mockedHTTPCall(ctx context.Context, duration time.Duration) (string, error) {
	select {
	case <-time.After(duration):
		return fmt.Sprintf("Response after %v", duration), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}