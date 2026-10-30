package storage

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadDirectoryReturnsEntries(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	filter1 := make([]byte, 64)
	filter2 := make([]byte, 64)
	filter1[0], filter2[0] = 0x11, 0x22

	filterOffset1, err := s.appendAlignedFilter(filter1)
	if err != nil {
		t.Fatal(err)
	}

	data1 := []byte("first log block")
	dataOffset1, err := s.append(data1)
	if err != nil {
		t.Fatal(err)
	}

	filterOffset2, err := s.appendAlignedFilter(filter2)
	if err != nil {
		t.Fatal(err)
	}

	data2 := []byte("second log block")
	dataOffset2, err := s.append(data2)
	if err != nil {
		t.Fatal(err)
	}

	entries := []DirectoryEntry{
		{
			FilterOffset: uint64(filterOffset1),
			FilterSize:   uint64(len(filter1)),
			K:            3,
			DataOffset:   uint64(dataOffset1),
			DataSize:     uint64(len(data1)),
			MinTimestamp: 100,
			MaxTimestamp: 200,
			RowCount:     10,
		},
		{
			FilterOffset: uint64(filterOffset2),
			FilterSize:   uint64(len(filter2)),
			K:            4,
			DataOffset:   uint64(dataOffset2),
			DataSize:     uint64(len(data2)),
			MinTimestamp: 201,
			MaxTimestamp: 300,
			RowCount:     20,
		},
	}

	if err := s.Finalize(entries); err != nil {
		t.Fatal(err)
	}

	// Filters occupy [0, 128); data follows them.
	want := []DirectoryEntry{
		{
			FilterOffset: 0,
			FilterSize:   64,
			K:            3,
			DataOffset:   128,
			DataSize:     uint64(len(data1)),
			MinTimestamp: 100,
			MaxTimestamp: 200,
			RowCount:     10,
		},
		{
			FilterOffset: 64,
			FilterSize:   64,
			K:            4,
			DataOffset:   128 + uint64(len(data1)),
			DataSize:     uint64(len(data2)),
			MinTimestamp: 201,
			MaxTimestamp: 300,
			RowCount:     20,
		},
	}

	path := filepath.Join(dir, "seg-000001.dat")
	got, err := ReadDirectory(path)
	if err != nil {
		t.Fatalf("ReadDirectory(): %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadDirectory() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestReadDirectoryRejectsFileSmallerThanFooter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "too-small.dat")

	if err := os.WriteFile(path, []byte("tiny"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for file smaller than footer")
	}
}

func TestReadDirectoryRejectsInvalidFooter(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.append([]byte("payload before footer")); err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")

	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	footerOffset := info.Size() - FooterSize

	if _, err := file.WriteAt([]byte("BROKEN!!"), footerOffset); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for invalid footer")
	}
}

func TestReadDirectoryRejectsInvalidDirectoryBounds(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	footerOffset := info.Size() - FooterSize

	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(info.Size()))

	if _, err := file.WriteAt(buf[:], footerOffset+12); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for invalid directory offset")
	}
}

func TestReadDirectoryRejectsCorruptedDirectoryChecksum(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	filter := make([]byte, 64)
	filterOffset, err := s.appendAlignedFilter(filter)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("some log data")
	dataOffset, err := s.append(data)
	if err != nil {
		t.Fatal(err)
	}

	entry := DirectoryEntry{
		FilterOffset: uint64(filterOffset),
		FilterSize:   uint64(len(filter)),
		K:            3,
		DataOffset:   uint64(dataOffset),
		DataSize:     uint64(len(data)),
		MinTimestamp: 1,
		MaxTimestamp: 2,
		RowCount:     1,
	}

	if err := s.Finalize([]DirectoryEntry{entry}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	footerOffset := info.Size() - FooterSize
	footerBytes := make([]byte, FooterSize)

	if _, err := file.ReadAt(footerBytes, footerOffset); err != nil {
		file.Close()
		t.Fatal(err)
	}

	f, err := DecodeFooter(footerBytes)
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	var oneByte [1]byte
	if _, err := file.ReadAt(oneByte[:], int64(f.DirectoryOffset)); err != nil {
		file.Close()
		t.Fatal(err)
	}

	oneByte[0] ^= 0xFF

	if _, err := file.WriteAt(oneByte[:], int64(f.DirectoryOffset)); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for corrupted directory checksum")
	}
}

func TestReadDirectoryAcceptsEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	got, err := ReadDirectory(path)
	if err != nil {
		t.Fatalf("ReadDirectory(): %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty directory, got %d entries", len(got))
	}
}

func TestReadDirectoryRejectsHugeEntryCount(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	footerOffset := info.Size() - FooterSize

	var count [8]byte
	binary.LittleEndian.PutUint64(count[:], uint64(1)<<60)

	if _, err := file.WriteAt(count[:], footerOffset+20); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for enormous entry count")
	}
}

func TestReadDirectoryRejectsExtraBytesBeforeFooter(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.append([]byte("unaccounted bytes")); err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	footerOffset := info.Size() - FooterSize

	var offset [8]byte
	binary.LittleEndian.PutUint64(offset[:], 0)

	if _, err := file.WriteAt(offset[:], footerOffset+12); err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for extra bytes before footer")
	}
}

func rewriteFirstDirectoryEntry(
	t *testing.T,
	path string,
	mutate func([]byte),
) {
	t.Helper()

	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	footerOffset := info.Size() - FooterSize
	footerBytes := make([]byte, FooterSize)

	if _, err := file.ReadAt(footerBytes, footerOffset); err != nil {
		t.Fatal(err)
	}

	f, err := DecodeFooter(footerBytes)
	if err != nil {
		t.Fatal(err)
	}

	if f.EntryCount == 0 {
		t.Fatal("expected at least one directory entry")
	}

	directorySize := f.EntryCount * DirectoryEntrySize
	directoryBytes := make([]byte, int(directorySize))

	if _, err := file.ReadAt(directoryBytes, int64(f.DirectoryOffset)); err != nil {
		t.Fatal(err)
	}

	mutate(directoryBytes[:DirectoryEntrySize])

	if _, err := file.WriteAt(directoryBytes, int64(f.DirectoryOffset)); err != nil {
		t.Fatal(err)
	}

	f.DirectoryCRC = crc32.ChecksumIEEE(directoryBytes)

	if _, err := file.WriteAt(f.Encode(), footerOffset); err != nil {
		t.Fatal(err)
	}

	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
}

func createSegmentWithOneEntry(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	filter := make([]byte, 64)
	filterOffset, err := s.appendAlignedFilter(filter)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("x")
	dataOffset, err := s.append(data)
	if err != nil {
		t.Fatal(err)
	}

	entry := DirectoryEntry{
		FilterOffset: uint64(filterOffset),
		FilterSize:   uint64(len(filter)),
		K:            3,
		DataOffset:   uint64(dataOffset),
		DataSize:     uint64(len(data)),
		MinTimestamp: 1,
		MaxTimestamp: 2,
		RowCount:     1,
	}

	if err := s.Finalize([]DirectoryEntry{entry}); err != nil {
		t.Fatal(err)
	}

	return filepath.Join(dir, "seg-000001.dat")
}

func TestReadDirectoryRejectsZeroFilterSize(t *testing.T) {
	path := createSegmentWithOneEntry(t)

	rewriteFirstDirectoryEntry(t, path, func(entry []byte) {
		binary.LittleEndian.PutUint64(entry[8:16], 0)
	})

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for zero filter size")
	}
}

func TestReadDirectoryRejectsNonMultipleFilterSize(t *testing.T) {
	path := createSegmentWithOneEntry(t)

	rewriteFirstDirectoryEntry(t, path, func(entry []byte) {
		binary.LittleEndian.PutUint64(entry[8:16], 65)
	})

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for filter size not divisible by 64")
	}
}

func TestReadDirectoryRejectsFilterRangeBeyondDirectory(t *testing.T) {
	path := createSegmentWithOneEntry(t)

	rewriteFirstDirectoryEntry(t, path, func(entry []byte) {
		binary.LittleEndian.PutUint64(entry[0:8], 64)
		binary.LittleEndian.PutUint64(entry[8:16], 64)
	})

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for filter range beyond directory")
	}
}

func TestReadDirectoryRejectsDataRangeBeyondDirectory(t *testing.T) {
	path := createSegmentWithOneEntry(t)

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	footerBytes := make([]byte, FooterSize)
	if _, err := file.ReadAt(footerBytes, info.Size()-FooterSize); err != nil {
		file.Close()
		t.Fatal(err)
	}

	f, err := DecodeFooter(footerBytes)
	if err != nil {
		file.Close()
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	rewriteFirstDirectoryEntry(t, path, func(entry []byte) {
		binary.LittleEndian.PutUint64(entry[20:28], f.DirectoryOffset)
		binary.LittleEndian.PutUint64(entry[28:36], 1)
	})

	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected error for data range beyond directory")
	}
}

func TestReadBlock(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	filter := make([]byte, 64)
	filterOffset, err := s.appendAlignedFilter(filter)
	if err != nil {
		t.Fatal(err)
	}

	want := []byte("ERROR payment timeout req=abc123\nINFO request completed status=200\n")

	dataOffset, err := s.append(want)
	if err != nil {
		t.Fatal(err)
	}

	entry := DirectoryEntry{
		FilterOffset: uint64(filterOffset),
		FilterSize:   uint64(len(filter)),
		K:            3,
		DataOffset:   uint64(dataOffset),
		DataSize:     uint64(len(want)),
		RowCount:     2,
	}

	if err := s.Finalize([]DirectoryEntry{entry}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")

	entries, err := ReadDirectory(path)
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}

	got, err := ReadBlock(path, entries[0])
	if err != nil {
		t.Fatalf("ReadBlock failed: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadBlock mismatch\nGot:  %q\nWant: %q", got, want)
	}
}
