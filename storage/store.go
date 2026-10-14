package storage

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
)

const BloomLineSize int64 = 64

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

	// O_EXCL prevents reopening or overwriting an existing segment.
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("create segment %s: %w", path, err)
	}

	return &store{
		file:          file,
		currentfileID: fileID,
		dir:           dir,
		offset:        0,
	}, nil
}

// append writes data and returns its starting byte offset.
func (s *store) append(data []byte) (int64, error) {
	if s.file == nil {
		return 0, fmt.Errorf("store is closed")
	}

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

// appendAlignedFilter pads the file to a 64-byte boundary,
// then writes the filter and returns the filter's starting offset.
func (s *store) appendAlignedFilter(data []byte) (int64, error) {
	if s.file == nil {
		return 0, fmt.Errorf("store is closed")
	}

	padding := (BloomLineSize - s.offset%BloomLineSize) % BloomLineSize

	if padding > 0 {
		if _, err := s.append(make([]byte, int(padding))); err != nil {
			return 0, fmt.Errorf("write filter padding: %w", err)
		}
	}

	start, err := s.append(data)
	if err != nil {
		return start, fmt.Errorf("write filter: %w", err)
	}

	return start, nil
}

// Finalize writes the directory and footer, syncs the file,
// and closes the segment. Call it after all blocks are written.
func (s *store) Finalize(entries []DirectoryEntry) error {
	if s.file == nil {
		return fmt.Errorf("store is already closed")
	}

	// Encode all directory entries.
	directory := make([]byte, 0, len(entries)*DirectoryEntrySize)
	for _, entry := range entries {
		directory = append(directory, entry.Encode()...)
	}

	// Record the directory's starting offset.
	directoryOffset := uint64(s.offset)

	// Calculate the directory checksum.
	directoryCRC := crc32.ChecksumIEEE(directory)

	// Write the directory.
	if _, err := s.append(directory); err != nil {
		return fmt.Errorf("write directory: %w", err)
	}

	// Write the footer last.
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

// close closes the file without finalizing the segment.
// An unfinalized segment must not be reopened for appending.
func (s *store) close() error {
	if s.file == nil {
		return nil
	}

	err := s.file.Close()
	s.file = nil
	return err
}
