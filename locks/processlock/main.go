package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// Process lock using flock (file lock)
// Run this program in TWO separate terminals to see it in action.
// Terminal 1 will acquire the lock, Terminal 2 will wait until Terminal 1 releases it.

func main() {

	lockFile := "/tmp/processLock.lock"

	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)

	if err != nil {
		fmt.Println("Error opening lock file:", err)
		return
	}

	defer f.Close()

	fmt.Printf("[PID %d] Trying to acquire lock...\n", os.Getpid())

	// LOCK - blocks if any other process holds the lock

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)

	if err != nil {
		fmt.Println("Error acquiring lock:", err)
		return
	}

	fmt.Printf("[PID %d] Lock acquired! Working for 10 seconds...\n", os.Getpid())

	for i := 1; i <= 10; i++ {
		fmt.Printf("[PID %d] working... %d/10\n", os.Getpid(), i)
		time.Sleep(1 * time.Second)
	}

	// UNLOCK
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	fmt.Printf("[PID %d] Lock released!\n", os.Getpid())
}
