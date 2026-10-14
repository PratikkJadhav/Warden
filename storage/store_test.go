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

	offset1, err := s.append(first)
	if err != nil {
		t.Fatalf("append(first): %v", err)
	}
	if offset1 != 0 {
		t.Fatalf("first offset = %d, want 0", offset1)
	}

	offset2, err := s.append(second)
	if err != nil {
		t.Fatalf("append(second): %v", err)
	}
	if offset2 != int64(len(first)) {
		t.Fatalf("second offset = %d, want %d",
			offset2, len(first))
	}

	gotFirst := make([]byte, len(first))
	if _, err := s.file.ReadAt(gotFirst, offset1); err != nil {
		t.Fatalf("ReadAt(first): %v", err)
	}
	if !bytes.Equal(gotFirst, first) {
		t.Fatalf("first data = %q, want %q", gotFirst, first)
	}

	gotSecond := make([]byte, len(second))
	if _, err := s.file.ReadAt(gotSecond, offset2); err != nil {
		t.Fatalf("ReadAt(second): %v", err)
	}
	if !bytes.Equal(gotSecond, second) {
		t.Fatalf("second data = %q, want %q", gotSecond, second)
	}
}

func TestAppendAlignedFilter(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore(): %v", err)
	}
	defer s.close()

	// Move the current offset to 13 to force padding.
	if _, err := s.append(make([]byte, 13)); err != nil {
		t.Fatalf("append prefix: %v", err)
	}

	filter := make([]byte, 64)
	filter[0] = 0xAB

	offset, err := s.appendAlignedFilter(filter)
	if err != nil {
		t.Fatalf("appendAlignedFilter(): %v", err)
	}

	if offset%BloomLineSize != 0 {
		t.Fatalf("filter offset = %d, want a multiple of %d",
			offset, BloomLineSize)
	}

	if offset != 64 {
		t.Fatalf("filter offset = %d, want 64", offset)
	}

	got := make([]byte, len(filter))
	if _, err := s.file.ReadAt(got, offset); err != nil {
		t.Fatalf("ReadAt(filter): %v", err)
	}

	if !bytes.Equal(got, filter) {
		t.Fatalf("filter bytes do not match")
	}
}

func TestStoreFinalizeWritesDirectoryAndFooter(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatalf("newStore(): %v", err)
	}

	filter1 := make([]byte, 64)
	filter2 := make([]byte, 64)
	filter1[0] = 0x11
	filter2[0] = 0x22

	filterOffset1, err := s.appendAlignedFilter(filter1)
	if err != nil {
		t.Fatalf("write filter1: %v", err)
	}

	data1 := []byte("block one")
	dataOffset1, err := s.append(data1)
	if err != nil {
		t.Fatalf("write data1: %v", err)
	}

	filterOffset2, err := s.appendAlignedFilter(filter2)
	if err != nil {
		t.Fatalf("write filter2: %v", err)
	}

	data2 := []byte("block two")
	dataOffset2, err := s.append(data2)
	if err != nil {
		t.Fatalf("write data2: %v", err)
	}

	entries := []DirectoryEntry{
		{
			FilterOffset: uint64(filterOffset1),
			FilterSize:   uint64(len(filter1)),
			K:            4,
			DataOffset:   uint64(dataOffset1),
			DataSize:     uint64(len(data1)),
			MinTimestamp: 100,
			MaxTimestamp: 200,
			RowCount:     1,
		},
		{
			FilterOffset: uint64(filterOffset2),
			FilterSize:   uint64(len(filter2)),
			K:            4,
			DataOffset:   uint64(dataOffset2),
			DataSize:     uint64(len(data2)),
			MinTimestamp: 201,
			MaxTimestamp: 300,
			RowCount:     1,
		},
	}

	for i, entry := range entries {
		if entry.FilterOffset%uint64(BloomLineSize) != 0 {
			t.Fatalf("entry %d filter offset %d is not aligned",
				i, entry.FilterOffset)
		}
	}

	if err := s.Finalize(entries); err != nil {
		t.Fatalf("Finalize(): %v", err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open finalized segment: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		t.Fatalf("Stat(): %v", err)
	}

	// Check that the footer occupies the final FooterSize bytes.
	footerBytes := make([]byte, FooterSize)
	if _, err := file.ReadAt(footerBytes, info.Size()-FooterSize); err != nil {
		t.Fatalf("read footer: %v", err)
	}

	f, err := DecodeFooter(footerBytes)
	if err != nil {
		t.Fatalf("DecodeFooter(): %v", err)
	}

	if f.EntryCount != uint64(len(entries)) {
		t.Fatalf("entry count = %d, want %d",
			f.EntryCount, len(entries))
	}

	directorySize := len(entries) * DirectoryEntrySize
	directoryBytes := make([]byte, directorySize)

	if _, err := file.ReadAt(
		directoryBytes,
		int64(f.DirectoryOffset),
	); err != nil {
		t.Fatalf("read directory: %v", err)
	}

	gotCRC := crc32.ChecksumIEEE(directoryBytes)
	if gotCRC != f.DirectoryCRC {
		t.Fatalf("directory CRC = %d, want %d",
			gotCRC, f.DirectoryCRC)
	}

	for i, want := range entries {
		start := i * DirectoryEntrySize
		end := start + DirectoryEntrySize

		got, err := DecodeDirectoryEntry(directoryBytes[start:end])
		if err != nil {
			t.Fatalf("decode entry %d: %v", i, err)
		}
		if got != want {
			t.Fatalf("entry %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestNewStoreRefusesExistingSegment(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatalf("newStore(): %v", err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatalf("Finalize(): %v", err)
	}

	// The segment already exists and is finalized.
	// Creating another store must not reopen it.
	s2, err := newStore(dir)
	if err == nil {
		s2.close()
		t.Fatal("expected error when reopening an existing segment")
	}
}
