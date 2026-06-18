package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type IndexEntry struct {
	Offset int64
	Length int32
}

type Index struct {
	entries map[string]IndexEntry
}

func NewIndex() *Index {
	return &Index{entries: make(map[string]IndexEntry)}
}

func (idx *Index) Add(word string, offset int64, length int32) {
	idx.entries[word] = IndexEntry{Offset: offset, Length: length}
}

func (idx *Index) Lookup(word string) (IndexEntry, bool) {
	entry, ok := idx.entries[word]
	return entry, ok
}

func (idx *Index) Count() int {
	return len(idx.entries)
}

func LoadIndex(r io.Reader) (*Index, error) {
	idx := NewIndex()
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ",", 3)
		if len(parts) != 3 {
			continue
		}

		word := parts[0]
		offset, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad offset for word %q: %w", word, err)
		}
		length, err := strconv.ParseInt(parts[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("bad length for word %q: %w", word, err)
		}

		idx.Add(word, offset, int32(length))
	}

	return idx, scanner.Err()
}

// usecase of this  ????
func SerializeIndex(w io.Writer, idx *Index) error {
	bw := bufio.NewWriter(w)
	for word, entry := range idx.entries {
		_, err := fmt.Fprintf(bw, "%s,%d,%d\n", word, entry.Offset, entry.Length)
		if err != nil {
			return err
		}
	}
	return bw.Flush()
}
