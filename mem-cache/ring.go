package main

import (
	"fmt"
	"hash/crc32"
	"sort"
)

type Ring struct {
	hashRing []uint32
	nodeMap  map[uint32]string
	replicas int
}

func NewRing(replicas int) *Ring{
	return &Ring{
		hashRing: []uint32{},
		nodeMap:  make(map[uint32]string),
		replicas: replicas,
	}
}

func (r *Ring) AddNode(nodeID string) {
	for i := 0; i < r.replicas; i++ {
		// create a unique hash for each virtual node
		hash := hashKey(fmt.Sprintf("%s-%d", nodeID, i))
		r.hashRing = append(r.hashRing, hash)
		r.nodeMap[hash] = nodeID
	}
	sort.Slice(r.hashRing, func(i, j int) bool {
		return r.hashRing[i] < r.hashRing[j]
	})
}

func (r *Ring) RemoveNode(nodeID string) {
	newRing := []uint32{}
	for _, hash := range r.hashRing {
		if r.nodeMap[hash] == nodeID {
			delete(r.nodeMap, hash)
		} else {
			newRing = append(newRing, hash)
		}
	}
	r.hashRing = newRing
}

func (r *Ring) GetNode(key string) string {
	if len(r.hashRing) == 0 {
		return ""
	}
	hash := hashKey(key)

	idx := sort.Search(len(r.hashRing), func(i int) bool {
		return r.hashRing[i] >= hash
	})

	// wrap around to the start of the ring
	if idx == len(r.hashRing) {
		idx = 0
	}
	return r.nodeMap[r.hashRing[idx]]
}

func hashKey(key string) uint32 {
	h := crc32.ChecksumIEEE([]byte(key))
	return h
}
