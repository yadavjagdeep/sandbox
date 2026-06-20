package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// Hint entry: [timestamp(8) | key_size(4) | entry_size(4) | offset(8) | key(var)]
const HintHeaderSize = 24

func (bc *Bitcask) writeHintFile(fileID int) error {
	path := filepath.Join(bc.dir, fmt.Sprintf("data_%06d.hint", fileID))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	keys := bc.keyDir.Keys()
	for _, key := range keys {
		kdEntry, _ := bc.keyDir.Get(key)
		if kdEntry.FileID != fileID {
			continue
		}

		buf := make([]byte, HintHeaderSize+len(key))
		binary.LittleEndian.PutUint64(buf[0:8], kdEntry.Timestamp)
		binary.LittleEndian.PutUint32(buf[8:12], uint32(len(key)))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(kdEntry.Size))
		binary.LittleEndian.PutUint64(buf[16:24], uint64(kdEntry.Offset))
		copy(buf[24:], key)

		f.Write(buf)
	}

	return nil
}

func (bc *Bitcask) loadHintFile(fileID int) error {
	path := filepath.Join(bc.dir, fmt.Sprintf("data_%06d.hint", fileID))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	offset := 0
	for offset < len(data) {
		if offset+HintHeaderSize > len(data) {
			break
		}

		timestamp := binary.LittleEndian.Uint64(data[offset : offset+8])
		keySize := binary.LittleEndian.Uint32(data[offset+8 : offset+12])
		entrySize := binary.LittleEndian.Uint32(data[offset+12 : offset+16])
		entryOffset := binary.LittleEndian.Uint64(data[offset+16 : offset+24])
		key := string(data[offset+24 : offset+24+int(keySize)])

		bc.keyDir.Put(key, fileID, int64(entryOffset), int64(entrySize), timestamp)
		offset += HintHeaderSize + int(keySize)
	}

	return nil
}
