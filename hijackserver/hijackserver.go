package main

import (
	// "bufio"
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
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	results := make([]Result, 10)
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(r.Context())
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

	// Send headers
	bufrw.WriteString("HTTP/1.1 200 OK\r\n")
	bufrw.WriteString("Content-Type: application/json\r\n")
	bufrw.WriteString("\r\n")

	// Send "waitforquorum" message
	json.NewEncoder(bufrw).Encode(map[string]string{"message": "waitforquorum complete"})
	bufrw.Flush()

	// Close the connection and cancel the context
	conn.Close()
	cancel()

	// Wait for all goroutines to complete
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
