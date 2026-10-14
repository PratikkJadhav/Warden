package storage

import (
	"fmt"
	"hash/crc32"
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
	}, nil
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

func (s *store) close() error {
	if s.file == nil {
		return nil
	}

	err := s.file.Close()
	s.file = nil
	return err
}

func (s *store) Finalize(entries []DirectoryEntry) error {
	if s.file == nil {
		return fmt.Errorf("store is already closed")
	}

	directory := make([]byte, 0, len(entries)*DirectoryEntrySize)
	for _, entry := range entries {
		directory = append(directory, entry.Encode()...)
	}

	directoryOffset := uint64(s.offset)

	// 3. Calculate a checksum over the encoded directory.
	directoryCRC := crc32.ChecksumIEEE(directory)

	if _, err := s.append(directory); err != nil {
		return fmt.Errorf("write directory: %w", err)
	}

	f := footer{
		Magic:           FooterMagic,
		Version:         FormatVersion,
		DirectoryOffset: directoryOffset,
		EntryCount:      uint64(len(entries)),
		DirectoryCRC:    directoryCRC,
	}

	if _, err := s.append(f.Encode()); err != nil {
		return fmt.Errorf("write footer: %w", err)
	}

	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("sync segment: %w", err)
	}

	if err := s.file.Close(); err != nil {
		s.file = nil
		return fmt.Errorf("close segment: %w", err)
	}

	s.file = nil
	return nil
}
