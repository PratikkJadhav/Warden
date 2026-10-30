package storage

import (
	"path/filepath"
	"testing"

	"github.com/PratikkJadhav/Warden/bloom"
)

func TestMightContainReadsFilterFromDisk(t *testing.T) {
	dir := t.TempDir()

	// Build an actual Bloom filter.
	filter := bloom.NewBloom(100, 0.01)

	filter.Add([]byte("payment"))
	filter.Add([]byte("timeout"))
	filter.Add([]byte("request"))

	filterBytes := filter.Bytes()

	// Create a segment.
	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	filterOffset, err := s.appendAlignedFilter(filterBytes)
	if err != nil {
		t.Fatal(err)
	}

	logData := []byte("ERROR payment timeout req=abc123\n")

	dataOffset, err := s.append(logData)
	if err != nil {
		t.Fatal(err)
	}

	entry := DirectoryEntry{
		FilterOffset: uint64(filterOffset),
		FilterSize:   uint64(len(filterBytes)),
		K:            uint32(filter.K()),
		DataOffset:   uint64(dataOffset),
		DataSize:     uint64(len(logData)),
		MinTimestamp: 100,
		MaxTimestamp: 200,
		RowCount:     1,
	}

	if err := s.Finalize([]DirectoryEntry{entry}); err != nil {
		t.Fatal(err)
	}

	// Reopen the finalized segment.
	path := filepath.Join(dir, "seg-000001.dat")

	entries, err := ReadDirectory(path)
	if err != nil {
		t.Fatal(err)
	}

	// Query the filter from disk.
	got, err := MightContain(path, entries[0], []byte("payment"))
	if err != nil {
		t.Fatal(err)
	}

	if !got {
		t.Fatal("expected Bloom filter to report payment as possibly present")
	}
}
