package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const MaxFileSize = 10 * 1024 * 1024 // 10MB before rotating

type Bitcask struct {
	mu         sync.RWMutex
	dir        string
	activeFile *DataFile
	oldFiles   map[int]*DataFile
	keyDir     *KeyDir
	nextFileID int
}

func Open(dir string) (*Bitcask, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	bc := &Bitcask{
		dir:      dir,
		oldFiles: make(map[int]*DataFile),
		keyDir:   NewKeyDir(),
	}

	if err := bc.recover(); err != nil {
		return nil, err
	}

	if err := bc.rotateActiveFile(); err != nil {
		return nil, err
	}

	return bc, nil
}

func (bc *Bitcask) Put(key, value []byte) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	entry := NewEntry(key, value)

	if bc.activeFile.Size()+entry.Size() > MaxFileSize {
		if err := bc.rotateActiveFile(); err != nil {
			return err
		}
	}

	offset, err := bc.activeFile.Write(entry)
	if err != nil {
		return err
	}

	bc.keyDir.Put(string(key), bc.activeFile.ID(), offset, entry.Size(), entry.Timestamp)
	return nil
}

func (bc *Bitcask) Get(key []byte) ([]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	kdEntry, found := bc.keyDir.Get(string(key))
	if !found {
		return nil, fmt.Errorf("key not found")
	}

	var df *DataFile
	if kdEntry.FileID == bc.activeFile.ID() {
		df = bc.activeFile
	} else {
		df = bc.oldFiles[kdEntry.FileID]
	}

	if df == nil {
		return nil, fmt.Errorf("datafile %d not found", kdEntry.FileID)
	}

	entry, err := df.ReadAt(kdEntry.Offset, kdEntry.Size)
	if err != nil {
		return nil, err
	}

	if !entry.ISValid() {
		return nil, fmt.Errorf("CRC mismatch: data corrupted")
	}

	if entry.IsTombstone() {
		return nil, fmt.Errorf("key not found")
	}

	return entry.Value, nil
}

func (bc *Bitcask) Delete(key []byte) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	entry := NewEntry(key, []byte{})

	if bc.activeFile.Size()+entry.Size() > MaxFileSize {
		if err := bc.rotateActiveFile(); err != nil {
			return err
		}
	}

	_, err := bc.activeFile.Write(entry)
	if err != nil {
		return err
	}

	bc.keyDir.Delete(string(key))
	return nil
}

func (bc *Bitcask) Close() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.activeFile != nil {
		bc.activeFile.Sync()
		bc.activeFile.Close()
	}

	for _, df := range bc.oldFiles {
		df.Close()
	}

	return nil
}

func (bc *Bitcask) rotateActiveFile() error {
	if bc.activeFile != nil {
		bc.activeFile.Sync()
		bc.oldFiles[bc.activeFile.ID()] = bc.activeFile
	}

	df, err := NewDataFile(bc.dir, bc.nextFileID, false)
	if err != nil {
		return err
	}

	bc.activeFile = df
	bc.nextFileID++
	return nil
}

func (bc *Bitcask) recover() error {
	files, err := filepath.Glob(filepath.Join(bc.dir, "data_*.db"))
	if err != nil {
		return err
	}

	if len(files) == 0 {
		bc.nextFileID = 0
		return nil
	}

	sort.Strings(files)

	var maxID int
	for _, f := range files {
		base := filepath.Base(f)
		idStr := strings.TrimPrefix(base, "data_")
		idStr = strings.TrimSuffix(idStr, ".db")
		id, _ := strconv.Atoi(idStr)

		if id > maxID {
			maxID = id
		}

		df, err := NewDataFile(bc.dir, id, true)
		if err != nil {
			return err
		}

		entries, err := df.ReadAll()
		if err != nil {
			df.Close()
			continue
		}

		var offset int64
		for _, entry := range entries {
			if entry.IsTombstone() {
				bc.keyDir.Delete(string(entry.Key))
			} else {
				bc.keyDir.Put(string(entry.Key), id, offset, entry.Size(), entry.Timestamp)
			}
			offset += entry.Size()
		}

		bc.oldFiles[id] = df
	}

	bc.nextFileID = maxID + 1
	return nil
}
