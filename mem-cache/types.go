package main

import "time"

type Entry struct {
	value      []byte
	expireAt   int64
	accessedAt int64
}

type Command struct {
	Op       string // GET, PUT, DEL
	Key      string
	Value    []byte
	TTL      int64
	Response chan Result
}

type Result struct {
	Value []byte
	Found bool
	Err   error
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}
