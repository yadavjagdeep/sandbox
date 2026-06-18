package main

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	HeaderSize = 64
	Magic      = "WDICT"
)

type Header struct {
	Version     uint32
	WordCount   uint32
	DataOffset  uint64
	IndexOffset uint64
	IndexLength uint64
}

func WriteHeader(w io.Writer, h Header) error {
	buf := make([]byte, HeaderSize)

	// Magic bytes (5 bytes)
	copy(buf[0:5], []byte(Magic))

	// Version (4 bytes)
	binary.LittleEndian.PutUint32(buf[5:9], h.Version)

	// Word count (4 bytes)
	binary.LittleEndian.PutUint32(buf[9:13], h.WordCount)

	binary.LittleEndian.PutUint64(buf[13:21], h.IndexOffset)
	binary.LittleEndian.PutUint64(buf[21:29], h.IndexLength)
	binary.LittleEndian.PutUint64(buf[29:37], h.DataOffset)

	// bytes 37-63: reserved

	_, err := w.Write(buf)
	return err
}

func ReadHeader(r io.Reader) (Header, error) {
	buf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Header{}, err
	}

	if string(buf[0:5]) != Magic {
		return Header{}, errors.New("invalid file: magic bytes mismatch")
	}

	return Header{
		Version:     binary.LittleEndian.Uint32(buf[5:9]),
		WordCount:   binary.LittleEndian.Uint32(buf[9:13]),
		IndexOffset: binary.LittleEndian.Uint64(buf[13:21]),
		IndexLength: binary.LittleEndian.Uint64(buf[21:29]),
		DataOffset:  binary.LittleEndian.Uint64(buf[29:37]),
	}, nil
}
