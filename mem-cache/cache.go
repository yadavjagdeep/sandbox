package main

import (
	"fmt"
	"time"
)

type Cache struct {
	data        map[string]Entry
	usedBytes   int64
	maxBytes    int64
	commandChan chan Command
}

func NewCache(maxBytes int64) *Cache {
	c := &Cache{
		data:        make(map[string]Entry),
		maxBytes:    maxBytes,
		commandChan: make(chan Command, 1024),
	}
	go c.run()
	return c
}

func (c *Cache) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-c.commandChan:
			switch cmd.Op {
			case "GET":
				cmd.Response <- c.handleGet(cmd.Key)
			case "PUT":
				cmd.Response <- c.handlePut(cmd.Key, cmd.Value, cmd.TTL)
			case "DEL":
				cmd.Response <- c.handleDel(cmd.Key)
			default:
				cmd.Response <- Result{Err: fmt.Errorf("unknown command: %s", cmd.Op)}
			}
		case <-ticker.C:
			c.activeExpire()
		}
	}

}

func (c *Cache) handleGet(Key string) Result {
	entry, exists := c.data[Key]
	if !exists {
		return Result{Found: false}
	}

	// lazy TTL check
	if entry.expireAt > 0 && nowMs() > entry.expireAt {
		c.usedBytes -= c.entrySize(Key, entry)
		delete(c.data, Key)
		return Result{Found: false}
	}

	// update accessedAt for LRU

	entry.accessedAt = nowMs()
	return Result{Found: true, Value: entry.value}
}

func (c *Cache) entrySize(key string, entry Entry) int64 {
	return int64(len(key) + len(entry.value) + 16)
}

func (c *Cache) handlePut(key string, value []byte, ttl int64) Result {
	newEntry := Entry{
		value:      value,
		accessedAt: nowMs(),
	}

	if ttl > 0 {

		newEntry.expireAt = nowMs() + ttl*1000

	}

	newSize := c.entrySize(key, newEntry)

	// if overwriting, substract old size
	if old, exists := c.data[key]; exists {
		c.usedBytes -= c.entrySize(key, old)
	}

	// evict until there's room

	for c.usedBytes+newSize > c.maxBytes {
		if !c.evict() {
			break
		}
	}

	c.data[key] = newEntry
	c.usedBytes += newSize

	return Result{Found: true}
}

func (c *Cache) handleDel(key string) Result {
	entry, exists := c.data[key]
	if !exists {
		return Result{Found: false}
	}
	c.usedBytes -= c.entrySize(key, entry)
	delete(c.data, key)
	return Result{Found: true}
}

func (c *Cache) evict() bool {
	var oldestKey string
	var oldestTime int64 = 1<<63 - 1

	count := 0
	for k, entry := range c.data {
		if entry.accessedAt < oldestTime {
			oldestTime = entry.accessedAt
			oldestKey = k
		}
		count++

		if count >= 5 {
			break
		}
	}

	if oldestKey == "" {
		return false
	}

	c.usedBytes -= c.entrySize(oldestKey, c.data[oldestKey])
	delete(c.data, oldestKey)
	return true
}

func (c *Cache) activeExpire() {
	now := nowMs()

	for {
		sampled := 0
		expired := 0

		for k, entry := range c.data {
			if sampled >= 20 {
				break
			}
			sampled++

			if entry.expireAt > 0 && now > entry.expireAt {
				expired++
				delete(c.data, k)
				c.usedBytes -= c.entrySize(k, entry)
			}

		}

		// if less than 25% expired, stop
		if sampled == 0 || expired <= sampled/4 {
			break
		}
	}
}
