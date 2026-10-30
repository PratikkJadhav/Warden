package storage

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestReadFilterLine(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a filter containing two 64-byte lines.
	filter := make([]byte, 128)

	// Give each line distinct contents.
	for i := 0; i < 64; i++ {
		filter[i] = 0xAA
		filter[64+i] = 0xBB
	}

	filterOffset, err := s.appendAlignedFilter(filter)
	if err != nil {
		t.Fatal(err)
	}

	dataOffset, err := s.append([]byte("test log data"))
	if err != nil {
		t.Fatal(err)
	}

	entry := DirectoryEntry{
		FilterOffset: uint64(filterOffset),
		FilterSize:   uint64(len(filter)),
		K:            3,
		DataOffset:   uint64(dataOffset),
		DataSize:     uint64(len("test log data")),
		MinTimestamp: 100,
		MaxTimestamp: 200,
		RowCount:     1,
	}

	err = s.Finalize([]DirectoryEntry{entry})
	if err != nil {
		t.Fatal(err)
	}

	// Read the finalized directory to get absolute offsets.
	path := filepath.Join(dir, "seg-000001.dat")
	entries, err := ReadDirectory(path)
	if err != nil {
		t.Fatal(err)
	}

	// Read the second filter line.
	got, err := ReadFilterLine(path, entries[0], 1)
	if err != nil {
		t.Fatal(err)
	}

	want := filter[64:128]

	if !bytes.Equal(got, want) {
		t.Fatalf("filter line mismatch:\ngot:  %v\nwant: %v", got, want)
	}
}

func TestReadFilterLineRejectsOutOfRangeIndex(t *testing.T) {
	entry := DirectoryEntry{
		FilterOffset: 0,
		FilterSize:   128, // Two lines.
	}

	_, err := ReadFilterLine("does-not-matter.dat", entry, 2)
	if err == nil {
		t.Fatal("expected error for out-of-range line index")
	}
}
