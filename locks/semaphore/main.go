package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	fmt.Println("===== SEMAPHORE: Only 3 workers at a time =========")
	fmt.Println()

	sem := make(chan struct{}, 3)

	var wg sync.WaitGroup

	for i := 1; i <= 10; i++ {
		wg.Add(1)

		go func(id int) {

			defer wg.Done()

			sem <- struct{}{} // Acquire: take a key (block if all 3 keys are taken)
			fmt.Printf("[%s] Worker %d: ENTERED (acquired slot)\n", time.Now().Format("15:04:05.000"), id)

			time.Sleep(1 * time.Second)

			fmt.Printf("[%s] Worker %d: LEAVING (releasing slot)\n", time.Now().Format("15:04:05.000"), id)
			<-sem // Release: put the key back on the wall

		}(i)
	}

	wg.Wait()
	fmt.Println("\nAll workers done!")
	fmt.Println("Notice: workers entered in batches of 3, not all 10 at once")
}
