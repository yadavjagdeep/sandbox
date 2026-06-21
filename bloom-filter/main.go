package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	// Create a bloom filter for 10000 items with 1% false positive rate
	bf := New(10000, 0.01)

	fmt.Printf("Bloom Filter created:\n")
	fmt.Printf("  Bit array size (m): %d bits (%.2f KB)\n", bf.Size(), float64(bf.Size())/8/1024)
	fmt.Printf("  Hash functions (k): %d\n", bf.HashNum())
	fmt.Printf("  Target FP rate: 1%%\n\n")

	fmt.Println("Commands: add <word> | check <word> | load <file> | test | stats | quit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		switch cmd {
		case "add":
			if len(parts) < 2 {
				fmt.Println("usage: add <word>")
				continue
			}
			bf.Add([]byte(parts[1]))
			fmt.Printf("Added: %s\n", parts[1])

		case "check":
			if len(parts) < 2 {
				fmt.Println("usage: check <word>")
				continue
			}
			if bf.Check([]byte(parts[1])) {
				fmt.Printf("%s → PROBABLY IN SET (could be false positive)\n", parts[1])
			} else {
				fmt.Printf("%s → DEFINITELY NOT IN SET\n", parts[1])
			}

		case "load":
			if len(parts) < 2 {
				fmt.Println("usage: load <file>")
				continue
			}
			count := loadFile(bf, parts[1])
			fmt.Printf("Loaded %d words from %s\n", count, parts[1])

		case "test":
			runFPTest(bf)

		case "stats":
			fmt.Printf("Items added: %d\n", bf.Count())
			fmt.Printf("Bit array size: %d bits (%.2f KB)\n", bf.Size(), float64(bf.Size())/8/1024)
			fmt.Printf("Hash functions: %d\n", bf.HashNum())
			fmt.Printf("Estimated FP rate: %.4f%%\n", bf.EstimateFPRate()*100)

		case "quit", "exit":
			return

		default:
			fmt.Println("unknown command")
		}
	}
}

func loadFile(bf *BloomFilter, path string) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			bf.Add([]byte(word))
			count++
		}
	}
	return count
}

// runFPTest tests 1000 random strings that were never added
// and counts how many return "probably in set" (false positives).
func runFPTest(bf *BloomFilter) {
	if bf.Count() == 0 {
		fmt.Println("Add some items first")
		return
	}

	falsePositives := 0
	tests := 10000

	for i := 0; i < tests; i++ {
		// Generate a string that was almost certainly never added
		fake := fmt.Sprintf("__fake_nonexistent_key_%d__", i)
		if bf.Check([]byte(fake)) {
			falsePositives++
		}
	}

	actual := float64(falsePositives) / float64(tests) * 100
	expected := bf.EstimateFPRate() * 100

	fmt.Printf("Tested %d non-existent keys:\n", tests)
	fmt.Printf("  False positives: %d\n", falsePositives)
	fmt.Printf("  Actual FP rate: %.2f%%\n", actual)
	fmt.Printf("  Expected FP rate: %.2f%%\n", expected)
}
