package main

import (
	"encoding/binary"
	"hash/crc32"
	"time"
)

const HeaderSize = 20 // CRC(4) + Timestamp(8) + keysize(4) + valueSize(4)

type Entry struct {
	CRC       uint32
	Timestamp uint64
	KeySize   uint32
	ValueSize uint32
	Key       []byte
	Value     []byte
}

func NewEntry(key, value []byte) *Entry {
	e := &Entry{
		Timestamp: uint64(time.Now().Unix()),
		KeySize:   uint32(len(key)),
		ValueSize: uint32(len(value)),
		Key:       key,
		Value:     value,
	}
	e.CRC = e.computeCRC()
	return e
}

func (e *Entry) computeCRC() uint32 {
	buf := make([]byte, 8+4+4+len(e.Key)+len(e.Value))
	binary.LittleEndian.PutUint64(buf[0:8], e.Timestamp)
	binary.LittleEndian.PutUint32(buf[8:12], e.KeySize)
	binary.LittleEndian.PutUint32(buf[12:16], e.ValueSize)
	copy(buf[16:16+len(e.Key)], e.Key)
	copy(buf[16+len(e.Key):], e.Value)
	return crc32.ChecksumIEEE(buf)
}

func (e *Entry) Encode() []byte {
	size := HeaderSize + len(e.Key) + len(e.Value)
	buf := make([]byte, size)

	binary.LittleEndian.PutUint32(buf[0:4], e.CRC)
	binary.LittleEndian.PutUint64(buf[4:12], e.Timestamp)
	binary.LittleEndian.PutUint32(buf[12:16], e.KeySize)
	binary.LittleEndian.PutUint32(buf[16:20], e.ValueSize)
	copy(buf[20:20+e.KeySize], e.Key)
	copy(buf[20+e.KeySize:], e.Value)

	return buf
}

func DecodeEntry(buf []byte) *Entry {
	e := &Entry{
		CRC:       binary.LittleEndian.Uint32(buf[0:4]),
		Timestamp: binary.LittleEndian.Uint64(buf[4:12]),
		KeySize:   binary.LittleEndian.Uint32(buf[12:16]),
		ValueSize: binary.LittleEndian.Uint32(buf[16:20]),
	}
	e.Key = buf[20 : 20+e.KeySize]
	e.Value = buf[20+e.KeySize : 20+e.KeySize+e.ValueSize]
	return e
}

func (e *Entry) Size() int64 {
	return int64(HeaderSize + e.KeySize + e.ValueSize)
}

func (e *Entry) ISValid() bool {
	return e.CRC == e.computeCRC()
}

func (e *Entry) IsTombstone() bool {
	return e.ValueSize == 0
}
