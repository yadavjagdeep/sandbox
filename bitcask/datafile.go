package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

type DataFile struct {
	id     int
	file   *os.File
	offset int64
	dir    string
}

func NewDataFile(dir string, id int, readOnly bool) (*DataFile, error) {
	path := filepath.Join(dir, fmt.Sprintf("data_%06d.db", id))

	var flag int
	if readOnly {
		flag = os.O_RDONLY
	} else {
		flag = os.O_CREATE | os.O_RDWR | os.O_APPEND
	}

	file, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &DataFile{
		id:     id,
		file:   file,
		offset: info.Size(),
		dir:    dir,
	}, nil
}

func (df *DataFile) Write(entry *Entry) (int64, error) {
	data := entry.Encode()
	offset := df.offset

	_, err := df.file.Write(data)
	if err != nil {
		return 0, err
	}

	df.offset += int64(len(data))
	return offset, nil
}

func (df *DataFile) ReadAt(offset int64, size int64) (*Entry, error) {
	buf := make([]byte, size)
	_, err := df.file.ReadAt(buf, offset)
	if err != nil {
		return nil, err
	}
	return DecodeEntry(buf), nil
}

func (df *DataFile) ReadAll() ([]*Entry, error) {
	var entries []*Entry
	var offset int64

	for {
		header := make([]byte, HeaderSize)
		_, err := df.file.ReadAt(header, offset)
		if err != nil {
			break
		}

		keySize := binary.LittleEndian.Uint32(header[12:16])
		valueSize := binary.LittleEndian.Uint32(header[16:20])
		totalSize := int64(HeaderSize) + int64(keySize) + int64(valueSize)

		buf := make([]byte, totalSize)
		_, err = df.file.ReadAt(buf, offset)
		if err != nil {
			break
		}

		entry := DecodeEntry(buf)
		entries = append(entries, entry)
		offset += totalSize
	}

	return entries, nil
}

func (df *DataFile) Size() int64 {
	return df.offset
}

func (df *DataFile) ID() int {
	return df.id
}

func (df *DataFile) Close() error {
	return df.file.Close()
}

func (df *DataFile) Path() string {
	return filepath.Join(df.dir, fmt.Sprintf("data_%06d.db", df.id))
}

func (df *DataFile) Sync() error {
	return df.file.Sync()
}
