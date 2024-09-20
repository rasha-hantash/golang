package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func main() {
	// Create a parent context with a 5-second timeout
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer parentCancel()

	var wg sync.WaitGroup
	
	// Create a channel to synchronize printing
	printChan := make(chan string)

	// Launch a goroutine to handle printing
	go func() {
		for msg := range printChan {
			fmt.Println(msg)
		}
	}()

	// Launch 50 goroutines
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Simulate some work
			duration := time.Duration(rand.Intn(8000)) * time.Millisecond
			
			select {
			case <-time.After(duration):
				printChan <- fmt.Sprintf("Goroutine %d completed after %v", id, duration)
			case <-parentCtx.Done():
				// Parent context timed out, but we continue the work
				time.Sleep(duration - time.Since(time.Now()))
				printChan <- fmt.Sprintf("Goroutine %d completed after %v (parent ctx timed out)", id, duration)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(printChan)
	}()

	// Wait for the parent context to timeout
	<-parentCtx.Done()
	fmt.Println("Parent context timed out, but goroutines will continue...")

	// Wait for all goroutines to actually complete
	wg.Wait()
	fmt.Println("All goroutines have completed")
}