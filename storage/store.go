package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type store struct {
	file          *os.File
	currentfileID uint32
	dir           string
	offset        int64
}

func newStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	fileID := uint32(1)
	filename := fmt.Sprintf("seg-%06d.dat", fileID)
	path := filepath.Join(dir, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, err
	}

	return &store{
		file:          file,
		currentfileID: fileID,
		offset:        offset,
		dir:           dir,
	}, err
}

func (s *store) append(data []byte) (int64, error) {
	start := s.offset

	n, err := s.file.Write(data)
	s.offset += int64(n)

	if err != nil {
		return start, err
	}
	if n != len(data) {
		return start, io.ErrShortWrite
	}

	return start, nil
}
