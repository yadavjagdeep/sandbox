package main

import (
	"fmt"
	"sync"
)

var counter int

func main() {
	fmt.Println("=== WITHOUT MUTEX (Race Condition) ===")
	counter = 0
	runWithoutMutex()

	fmt.Println("\n=== WITH MUTEX (Safe) ===")
	counter = 0
	runWithMutex()
}

func runWithoutMutex() {
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			temp := counter
			temp++
			counter = temp
		}()
	}
	wg.Wait()
	fmt.Printf("Expected: 1000, Got: %d\n", counter)
}

func runWithMutex() {
	var wg sync.WaitGroup

	var mu sync.Mutex

	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			mu.Lock()
			temp := counter
			temp++
			counter = temp
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Printf("Expected: 1000, Got: %d\n", counter)
}
