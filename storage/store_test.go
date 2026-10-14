package storage

import (
	"bytes"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreAppendAndReadBack(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatalf("newStore() error: %v", err)
	}
	defer s.close()

	first := []byte("first block")
	second := []byte("second block")

	// Append the first block.
	offset1, err := s.append(first)
	if err != nil {
		t.Fatalf("append(first) error: %v", err)
	}

	if offset1 != 0 {
		t.Fatalf("first offset = %d, want 0", offset1)
	}

	// Append the second block.
	offset2, err := s.append(second)
	if err != nil {
		t.Fatalf("append(second) error: %v", err)
	}

	if offset2 != int64(len(first)) {
		t.Fatalf("second offset = %d, want %d",
			offset2, len(first))
	}

	// Read both blocks directly from the segment file.
	gotFirst := make([]byte, len(first))
	n, err := s.file.ReadAt(gotFirst, offset1)
	if err != nil {
		t.Fatalf("ReadAt(first) error: %v", err)
	}
	if n != len(first) {
		t.Fatalf("read %d bytes for first block, want %d",
			n, len(first))
	}
	if !bytes.Equal(gotFirst, first) {
		t.Fatalf("first block = %q, want %q", gotFirst, first)
	}

	gotSecond := make([]byte, len(second))
	n, err = s.file.ReadAt(gotSecond, offset2)
	if err != nil {
		t.Fatalf("ReadAt(second) error: %v", err)
	}
	if n != len(second) {
		t.Fatalf("read %d bytes for second block, want %d",
			n, len(second))
	}
	if !bytes.Equal(gotSecond, second) {
		t.Fatalf("second block = %q, want %q", gotSecond, second)
	}
}

func TestNewStoreReopensExistingSegment(t *testing.T) {
	dir := t.TempDir()
	expected := []byte("existing segment data")

	// Write data and close the store.
	s, err := newStore(dir)
	if err != nil {
		t.Fatalf("newStore() error: %v", err)
	}

	offset, err := s.append(expected)
	if err != nil {
		t.Fatalf("append() error: %v", err)
	}
	if err := s.close(); err != nil {
		t.Fatalf("close() error: %v", err)
	}

	// Reopen the same segment.
	s2, err := newStore(dir)
	if err != nil {
		t.Fatalf("reopening store error: %v", err)
	}
	defer s2.close()

	if s2.offset != int64(len(expected)) {
		t.Fatalf("reopened offset = %d, want %d",
			s2.offset, len(expected))
	}

	// New data should begin after the existing data.
	next := []byte("more data")
	nextOffset, err := s2.append(next)
	if err != nil {
		t.Fatalf("append after reopen error: %v", err)
	}

	if nextOffset != int64(len(expected)) {
		t.Fatalf("next offset = %d, want %d",
			nextOffset, len(expected))
	}

	// Verify that the original data remains intact.
	got := make([]byte, len(expected))
	n, err := s2.file.ReadAt(got, offset)
	if err != nil {
		t.Fatalf("ReadAt() error: %v", err)
	}
	if n != len(expected) || !bytes.Equal(got, expected) {
		t.Fatalf("original data = %q, want %q", got, expected)
	}

	// Verify the segment file exists.
	path := filepath.Join(dir, "seg-000001.dat")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Size() != int64(len(expected)+len(next)) {
		t.Fatalf("file size = %d, want %d",
			info.Size(), len(expected)+len(next))
	}
}

func TestStoreFinalizeWritesDirectoryAndFooter(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatalf("newStore() error: %v", err)
	}

	// Simulate two blocks already written to the segment.
	block1 := []byte("block one")
	block2 := []byte("block two")

	offset1, err := s.append(block1)
	if err != nil {
		t.Fatalf("append block1: %v", err)
	}

	offset2, err := s.append(block2)
	if err != nil {
		t.Fatalf("append block2: %v", err)
	}

	entries := []DirectoryEntry{
		{
			FilterOffset: uint64(offset1),
			FilterSize:   64,
			K:            4,
			DataOffset:   uint64(offset1),
			DataSize:     uint64(len(block1)),
			MinTimestamp: 100,
			MaxTimestamp: 200,
			RowCount:     1,
		},
		{
			FilterOffset: uint64(offset2),
			FilterSize:   64,
			K:            4,
			DataOffset:   uint64(offset2),
			DataSize:     uint64(len(block2)),
			MinTimestamp: 201,
			MaxTimestamp: 300,
			RowCount:     1,
		},
	}

	if err := s.Finalize(entries); err != nil {
		t.Fatalf("Finalize() error: %v", err)
	}

	path := filepath.Join(dir, "seg-000001.dat")

	// Reopen the finalized segment independently.
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open finalized segment: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	// Read the fixed-size footer from the end of the file.
	footerBytes := make([]byte, FooterSize)
	_, err = file.ReadAt(footerBytes, info.Size()-FooterSize)
	if err != nil {
		t.Fatalf("read footer: %v", err)
	}

	f, err := DecodeFooter(footerBytes)
	if err != nil {
		t.Fatalf("DecodeFooter() error: %v", err)
	}

	if f.DirectoryOffset != uint64(len(block1)+len(block2)) {
		t.Fatalf("directory offset = %d, want %d",
			f.DirectoryOffset, len(block1)+len(block2))
	}

	if f.EntryCount != uint64(len(entries)) {
		t.Fatalf("entry count = %d, want %d",
			f.EntryCount, len(entries))
	}

	// Read and verify the directory using footer metadata.
	directorySize := int64(f.EntryCount) * DirectoryEntrySize
	directoryBytes := make([]byte, directorySize)

	_, err = file.ReadAt(directoryBytes, int64(f.DirectoryOffset))
	if err != nil {
		t.Fatalf("read directory: %v", err)
	}

	gotCRC := crc32.ChecksumIEEE(directoryBytes)
	if gotCRC != uint32(f.DirectoryCRC) {
		t.Fatalf("directory checksum = %d, want %d",
			gotCRC, f.DirectoryCRC)
	}

	// Decode every directory entry and compare it with the original.
	for i, want := range entries {
		start := i * DirectoryEntrySize
		end := start + DirectoryEntrySize

		got, err := DecodeDirectoryEntry(directoryBytes[start:end])
		if err != nil {
			t.Fatalf("decode directory entry %d: %v", i, err)
		}

		if got != want {
			t.Fatalf("entry %d = %+v, want %+v", i, got, want)
		}
	}
}
