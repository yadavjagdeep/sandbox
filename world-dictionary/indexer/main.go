package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	HeaderSize = 64
	Magic      = "WDICT"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatal("usage: indexer <input.csv> <output.wdict> <meta.json>")
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]
	metaPath := os.Args[3]

	// Read current meta to get version
	var currentVersion int
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		// Parse existing version
		for _, line := range strings.Split(string(metaData), "\n") {
			if strings.Contains(line, "version") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					numStr := strings.Trim(parts[1], " ,\t\"")
					fmt.Sscanf(numStr, "%d", &currentVersion)
				}
			}
		}
	}
	newVersion := currentVersion + 1

	// First pass: read all lines, compute offsets
	input, err := os.Open(inputPath)
	if err != nil {
		log.Fatalf("failed to open input: %v", err)
	}

	type entry struct {
		word string
		line string
	}

	seen := make(map[string]int) // word → index in entries slice
	var entries []entry

	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}

		word := strings.ToLower(strings.TrimSpace(parts[0]))
		lineWithNewline := line + "\n"

		if idx, exists := seen[word]; exists {
			// Duplicate: keep the latest meaning
			entries[idx] = entry{word: word, line: lineWithNewline}
		} else {
			seen[word] = len(entries)
			entries = append(entries, entry{word: word, line: lineWithNewline})
		}
	}
	input.Close()

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading input: %v", err)
	}

	// Compute offsets after dedup
	type indexedEntry struct {
		word   string
		line   string
		offset int64
		length int32
	}

	var finalEntries []indexedEntry
	var dataSize int64
	for _, e := range entries {
		finalEntries = append(finalEntries, indexedEntry{
			word:   e.word,
			line:   e.line,
			offset: dataSize,
			length: int32(len(e.line)),
		})
		dataSize += int64(len(e.line))
	}

	// Build index bytes
	var indexBuf bytes.Buffer
	for _, e := range finalEntries {
		fmt.Fprintf(&indexBuf, "%s,%d,%d\n", e.word, e.offset, e.length)
	}
	indexBytes := indexBuf.Bytes()

	// Calculate offsets
	// Layout: [Header 64] [Index] [Data]
	indexOffset := int64(HeaderSize)
	indexLength := int64(len(indexBytes))
	dataOffset := indexOffset + indexLength

	// Write output file
	output, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("failed to create output: %v", err)
	}
	defer output.Close()

	// Write header
	header := make([]byte, HeaderSize)
	copy(header[0:5], Magic)
	binary.LittleEndian.PutUint32(header[5:9], uint32(newVersion)) // version
	binary.LittleEndian.PutUint32(header[9:13], uint32(len(finalEntries)))
	binary.LittleEndian.PutUint64(header[13:21], uint64(indexOffset))
	binary.LittleEndian.PutUint64(header[21:29], uint64(indexLength))
	binary.LittleEndian.PutUint64(header[29:37], uint64(dataOffset))
	output.Write(header)

	// Write index
	output.Write(indexBytes)

	// Write data
	for _, e := range finalEntries {
		output.WriteString(e.line)
	}

	fmt.Printf("Built %s: %d words, index@%d (%d bytes), data@%d (%d bytes)\n",
		outputPath, len(finalEntries), indexOffset, indexLength, dataOffset, dataSize)

	// Update meta.json to point to the new file
	newMeta := fmt.Sprintf("{\n  \"version\": %d,\n  \"file\": \"%s\"\n}\n", newVersion, outputPath)
	if err := os.WriteFile(metaPath, []byte(newMeta), 0644); err != nil {
		log.Fatalf("failed to update meta file: %v", err)
	}

	fmt.Printf("Updated %s: version=%d, file=%s\n", metaPath, newVersion, outputPath)
}
