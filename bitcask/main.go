package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	db, err := Open("./data")
	if err != nil {
		fmt.Printf("Failed to open: %v\n", err)
		return
	}
	defer db.Close()

	fmt.Printf("Bitcask opened. Keys in store: %d\n", db.keyDir.Count())
	fmt.Println("Commands: put <key> <value> | get <key> | del <key> | merge | keys | quit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.SplitN(line, " ", 3)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		switch cmd {
		case "put":
			if len(parts) < 3 {
				fmt.Println("usage: put <key> <value>")
				continue
			}
			if err := db.Put([]byte(parts[1]), []byte(parts[2])); err != nil {
				fmt.Printf("error: %v\n", err)
			} else {
				fmt.Println("OK")
			}

		case "get":
			if len(parts) < 2 {
				fmt.Println("usage: get <key>")
				continue
			}
			val, err := db.Get([]byte(parts[1]))
			if err != nil {
				fmt.Printf("error: %v\n", err)
			} else {
				fmt.Printf("%s\n", string(val))
			}

		case "del":
			if len(parts) < 2 {
				fmt.Println("usage: del <key>")
				continue
			}
			if err := db.Delete([]byte(parts[1])); err != nil {
				fmt.Printf("error: %v\n", err)
			} else {
				fmt.Println("DELETED")
			}

		case "merge":
			if err := db.Merge(); err != nil {
				fmt.Printf("error: %v\n", err)
			}

		case "keys":
			keys := db.keyDir.Keys()
			fmt.Printf("%d keys: %v\n", len(keys), keys)

		case "quit", "exit":
			return

		default:
			fmt.Println("unknown command")
		}
	}
}
