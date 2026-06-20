package main

import "sync"

type KeyDirEntry struct {
	FileID    int
	Offset    int64
	Size      int64
	Timestamp uint64
}

type KeyDir struct {
	mu      sync.RWMutex
	entries map[string]KeyDirEntry
}

func NewKeyDir() *KeyDir {
	return &KeyDir{
		entries: make(map[string]KeyDirEntry),
	}
}

func (kd *KeyDir) Put(key string, fileID int, offset int64, size int64, timestamp uint64) {
	kd.mu.Lock()
	defer kd.mu.Unlock()
	kd.entries[key] = KeyDirEntry{
		FileID:    fileID,
		Offset:    offset,
		Size:      size,
		Timestamp: timestamp,
	}
}

func (kd *KeyDir) Get(key string) (KeyDirEntry, bool) {
	kd.mu.RLock()
	defer kd.mu.RUnlock()
	entry, ok := kd.entries[key]
	return entry, ok
}

func (kd *KeyDir) Delete(key string) {
	kd.mu.Lock()
	defer kd.mu.Unlock()
	delete(kd.entries, key)
}

func (kd *KeyDir) Keys() []string {
	kd.mu.RLock()
	defer kd.mu.RUnlock()
	keys := make([]string, 0, len(kd.entries))
	for k := range kd.entries {
		keys = append(keys, k)
	}
	return keys
}

func (kd *KeyDir) Count() int {
	kd.mu.RLock()
	defer kd.mu.RUnlock()
	return len(kd.entries)
}
