package main

import (
	"io"
	"os"
)

type Store interface {
	ReadAt(path string, offset int64, length int32) ([]byte, error)
	ReadSection(path string, offset int64, length int64) (io.ReadCloser, error)
}

type LocalStore struct{}

func (s *LocalStore) ReadAt(path string, offset int64, length int32) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, length)
	_, err = f.ReadAt(buf, offset)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *LocalStore) ReadSection(path string, offset int64, length int64) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		f.Close()
		return nil, err
	}

	return &limitedReadCloser{
		Reader: io.LimitReader(f, length),
		Closer: f,
	}, nil
}

type limitedReadCloser struct {
	io.Reader
	Closer io.Closer
}

func (l *limitedReadCloser) Close() error {
	return l.Closer.Close()
}
