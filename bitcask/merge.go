package main

import (
	"fmt"
	"os"
)

func (bc *Bitcask) Merge() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(bc.oldFiles) == 0 {
		fmt.Println("nothing to merge")
		return nil
	}

	mergedDF, err := NewDataFile(bc.dir, bc.nextFileID, false)
	if err != nil {
		return err
	}
	bc.nextFileID++

	keys := bc.keyDir.Keys()
	var merged int

	for _, key := range keys {
		kdEntry, _ := bc.keyDir.Get(key)

		// Skip if it's in the active file
		if kdEntry.FileID == bc.activeFile.ID() {
			continue
		}

		oldDF := bc.oldFiles[kdEntry.FileID]
		if oldDF == nil {
			continue
		}

		entry, err := oldDF.ReadAt(kdEntry.Offset, kdEntry.Size)
		if err != nil {
			continue
		}

		newOffset, err := mergedDF.Write(entry)
		if err != nil {
			continue
		}

		bc.keyDir.Put(key, mergedDF.ID(), newOffset, entry.Size(), entry.Timestamp)
		merged++
	}

	mergedDF.Sync()

	// Delete old files
	for id, df := range bc.oldFiles {
		if id == mergedDF.ID() {
			continue
		}
		path := df.Path()
		df.Close()
		os.Remove(path)
	}

	bc.oldFiles = map[int]*DataFile{
		mergedDF.ID(): mergedDF,
	}

	bc.writeHintFile(mergedDF.ID())

	fmt.Printf("Merge complete: %d keys compacted into %s\n", merged, mergedDF.Path())
	return nil
}
