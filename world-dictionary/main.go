package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/labstack/echo"
)

type Meta struct {
	Version int    `json:"version"`
	File    string `json:"file"`
}

func main() {
	metaData, err := os.ReadFile("data/meta.json")
	if err != nil {
		log.Fatalf("failed to read meta file: %v", err)
	}
	var meta Meta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		log.Fatalf("failed to parse meta file: %v", err)
	}

	store := &LocalStore{}
	filepath := meta.File

	// Read header
	headerData, err := store.ReadAt(filepath, 0, HeaderSize)
	if err != nil {
		log.Fatalf("failed to read header: %v", err)
	}

	header, err := ReadHeader(bytes.NewReader(headerData))
	if err != nil {
		log.Fatalf("failed to parse header: %v", err)
	}

	fmt.Printf("Dictionary v%d - %d words\n", header.Version, header.WordCount)

	// Load index
	indexReader, err := store.ReadSection(filepath, int64(header.IndexOffset), int64(header.IndexLength))
	if err != nil {
		log.Fatalf("failed to read index: %v", err)
	}
	defer indexReader.Close()

	index, err := LoadIndex(indexReader)
	if err != nil {
		log.Fatalf("failed to load index: %v", err)
	}

	fmt.Printf("Index loaded: %d entries\n", index.Count())

	// Start server
	server := NewDictServer(index, store, filepath, header)

	e := echo.New()
	e.GET("/words/:word", server.HandleLookup)

	e.Logger.Fatal(e.Start(":8080"))
}
