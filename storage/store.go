package storage

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
)

const BloomLineSize int64 = 64

type store struct {
	file          *os.File
	currentfileID uint32
	dir           string

	filterBuf []byte
	dataBuf   []byte
}

func newStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	fileID := uint32(1)
	path := filepath.Join(dir, fmt.Sprintf("seg-%06d.dat", fileID))

	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_EXCL|os.O_RDWR,
		0644,
	)
	if err != nil {
		return nil, err
	}

	return &store{
		file:          file,
		currentfileID: fileID,
		dir:           dir,
	}, nil
}

// appendAlignedFilter appends a filter to the filter region.
// The returned offset is relative to the start of that region.
func (s *store) appendAlignedFilter(data []byte) (int64, error) {
	if s.file == nil {
		return 0, fmt.Errorf("store is closed or finalized")
	}

	padding := (BloomLineSize -
		int64(len(s.filterBuf))%BloomLineSize) % BloomLineSize

	if padding > 0 {
		s.filterBuf = append(s.filterBuf, make([]byte, int(padding))...)
	}

	offset := int64(len(s.filterBuf))
	s.filterBuf = append(s.filterBuf, data...)

	return offset, nil
}

// append buffers log data separately from Bloom filters.
// The returned offset is relative to the start of the data region.
func (s *store) append(data []byte) (int64, error) {
	if s.file == nil {
		return 0, fmt.Errorf("store is closed or finalized")
	}

	offset := int64(len(s.dataBuf))
	s.dataBuf = append(s.dataBuf, data...)

	return offset, nil
}

func writeAll(file *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := file.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("short write: no progress")
		}
		data = data[n:]
	}
	return nil
}

// Finalize writes filters, log data, directory, and footer in that order.
// Entry offsets supplied here must be relative to their respective regions.
func (s *store) Finalize(entries []DirectoryEntry) error {
	if s.file == nil {
		return fmt.Errorf("store is already closed or finalized")
	}

	// The filters start at file offset 0. Data follows all filters.
	filterBase := uint64(0)
	dataBase := uint64(len(s.filterBuf))

	// Do not modify the caller's slice.
	finalEntries := append([]DirectoryEntry(nil), entries...)

	for i := range finalEntries {
		finalEntries[i].FilterOffset += filterBase
		finalEntries[i].DataOffset += dataBase
	}

	// Write the two contiguous regions.
	if err := writeAll(s.file, s.filterBuf); err != nil {
		return err
	}
	if err := writeAll(s.file, s.dataBuf); err != nil {
		return err
	}

	directory := make([]byte, 0, len(finalEntries)*DirectoryEntrySize)
	for _, entry := range finalEntries {
		directory = append(directory, entry.Encode()...)
	}

	directoryOffset := uint64(len(s.filterBuf) + len(s.dataBuf))
	directoryCRC := crc32.ChecksumIEEE(directory)

	if err := writeAll(s.file, directory); err != nil {
		return err
	}

	f := footer{
		Magic:           FooterMagic,
		Version:         FormatVersion,
		DirectoryOffset: directoryOffset,
		EntryCount:      uint64(len(finalEntries)),
		DirectoryCRC:    directoryCRC,
	}

	if err := writeAll(s.file, f.Encode()); err != nil {
		return err
	}

	if err := s.file.Sync(); err != nil {
		return err
	}

	if err := s.file.Close(); err != nil {
		s.file = nil
		return err
	}

	s.file = nil
	return nil
}

func (s *store) close() error {
	if s.file == nil {
		return nil
	}

	err := s.file.Close()
	s.file = nil
	return err
}
