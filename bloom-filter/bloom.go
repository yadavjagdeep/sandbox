package main

import (
	"hash"
	"hash/fnv"
	"math"
)

type BloomFilter struct {
	bits    []bool
	size    uint64 // m - size of bits array
	hashNum uint64 // k - number of hash functions
	count   uint64 // number of items added
}

func New(expectedItems uint64, falsePositiverate float64) *BloomFilter {
	m := optimalSize(expectedItems, falsePositiverate)
	k := optimalHashNum(m, expectedItems)

	return &BloomFilter{
		bits:    make([]bool, m),
		size:    m,
		hashNum: k,
	}
}

func (bf *BloomFilter) Add(item []byte) {
	for i := uint64(0); i < bf.hashNum; i++ {
		pos := bf.hash(item, i) % bf.size
		bf.bits[pos] = true
	}
	bf.count++
}

func (bf *BloomFilter) Check(item []byte) bool {
	for i := uint64(0); i < bf.hashNum; i++ {
		pos := bf.hash(item, i) % bf.size
		if !bf.bits[pos] {
			return false // definitely not in set
		}
	}
	return true
}

// EstimateFPRate returns the current estimated false positive rate
// based on how many items have been added.
func (bf *BloomFilter) EstimateFPRate() float64 {
	// (1 - e^(-kn/m))^k
	exponent := -float64(bf.hashNum) * float64(bf.count) / float64(bf.size)
	return math.Pow(1-math.Exp(exponent), float64(bf.hashNum))
}

func (bf *BloomFilter) Count() uint64 {
	return bf.count
}

func (bf *BloomFilter) Size() uint64 {
	return bf.size
}

func (bf *BloomFilter) HashNum() uint64 {
	return bf.hashNum
}

// hash generates the i-th hash of an item using double hashing technique.
// Instead of k independent hash functions, we use: h(i) = h1 + i*h2
func (bf *BloomFilter) hash(item []byte, i uint64) uint64 {
	h1 := hashWith(item, 0)
	h2 := hashWith(item, h1)
	return h1 + i*h2
}

func hashWith(item []byte, seed uint64) uint64 {
	var h hash.Hash64 = fnv.New64a()
	// Mix seed into the data
	seedBytes := []byte{
		byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24),
		byte(seed >> 32), byte(seed >> 40), byte(seed >> 48), byte(seed >> 56),
	}
	h.Write(seedBytes)
	h.Write(item)
	return h.Sum64()
}

// m = -(n * ln(p)) / (ln(2)^2)
func optimalSize(n uint64, p float64) uint64 {
	m := -float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)
	return uint64(math.Ceil(m))
}

// k = (m/n) * ln(2)
func optimalHashNum(m, n uint64) uint64 {
	k := float64(m) / float64(n) * math.Ln2
	return uint64(math.Ceil(k))
}
