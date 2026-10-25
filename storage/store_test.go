package storage

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoreAppendAndReadBack(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	filter := bytes.Repeat([]byte{0xAB}, 64)
	filterOffset, err := s.appendAlignedFilter(filter)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("hello Warden")
	dataOffset, err := s.append(data)
	if err != nil {
		t.Fatal(err)
	}

	if filterOffset != 0 {
		t.Fatalf("first filter offset = %d, want 0", filterOffset)
	}
	if dataOffset != 0 {
		t.Fatalf("first data offset = %d, want 0 relative to data region", dataOffset)
	}

	entry := DirectoryEntry{
		FilterOffset: uint64(filterOffset),
		FilterSize:   uint64(len(filter)),
		K:            3,
		DataOffset:   uint64(dataOffset),
		DataSize:     uint64(len(data)),
		MinTimestamp: 10,
		MaxTimestamp: 20,
		RowCount:     1,
	}

	if err := s.Finalize([]DirectoryEntry{entry}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	gotEntries, err := ReadDirectory(path)
	if err != nil {
		t.Fatal(err)
	}

	wantEntry := entry
	wantEntry.FilterOffset = 0
	wantEntry.DataOffset = uint64(len(filter))

	if !reflect.DeepEqual(gotEntries, []DirectoryEntry{wantEntry}) {
		t.Fatalf("directory entries = %#v, want %#v", gotEntries, []DirectoryEntry{wantEntry})
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	gotFilter := make([]byte, len(filter))
	if _, err := file.ReadAt(gotFilter, int64(gotEntries[0].FilterOffset)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotFilter, filter) {
		t.Fatal("filter bytes differ from original")
	}

	gotData := make([]byte, len(data))
	if _, err := file.ReadAt(gotData, int64(gotEntries[0].DataOffset)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotData, data) {
		t.Fatal("log data differs from original")
	}
}

func TestAppendAlignedFilter(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	first := []byte("short")
	offset1, err := s.appendAlignedFilter(first)
	if err != nil {
		t.Fatal(err)
	}

	second := make([]byte, 64)
	offset2, err := s.appendAlignedFilter(second)
	if err != nil {
		t.Fatal(err)
	}

	if offset1 != 0 {
		t.Fatalf("first filter offset = %d, want 0", offset1)
	}
	if offset2 != 64 {
		t.Fatalf("second filter offset = %d, want 64", offset2)
	}
	if offset2%BloomLineSize != 0 {
		t.Fatalf("second filter offset %d is not 64-byte aligned", offset2)
	}

	entry := DirectoryEntry{
		FilterOffset: uint64(offset1),
		FilterSize:   uint64(len(first)),
		K:            3,
		DataOffset:   0,
		DataSize:     0,
		MinTimestamp: 0,
		MaxTimestamp: 0,
		RowCount:     0,
	}

	// This test focuses on relative offsets and alignment.
	// Close without finalizing because the first filter is intentionally
	// not a valid whole-line Bloom filter.
	if err := s.close(); err != nil {
		t.Fatal(err)
	}

	_ = entry
}

func TestStoreFinalizeWritesContiguousRegionsDirectoryAndFooter(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	filter1 := bytes.Repeat([]byte{0x11}, 64)
	filter2 := bytes.Repeat([]byte{0x22}, 64)
	data1 := []byte("block one")
	data2 := []byte("block two")

	filterOffset1, err := s.appendAlignedFilter(filter1)
	if err != nil {
		t.Fatal(err)
	}
	dataOffset1, err := s.append(data1)
	if err != nil {
		t.Fatal(err)
	}
	filterOffset2, err := s.appendAlignedFilter(filter2)
	if err != nil {
		t.Fatal(err)
	}
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
			MinTimestamp: 1,
			MaxTimestamp: 2,
			RowCount:     1,
		},
		{
			FilterOffset: uint64(filterOffset2),
			FilterSize:   uint64(len(filter2)),
			K:            4,
			DataOffset:   uint64(dataOffset2),
			DataSize:     uint64(len(data2)),
			MinTimestamp: 3,
			MaxTimestamp: 4,
			RowCount:     1,
		},
	}

	if err := s.Finalize(entries); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "seg-000001.dat")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	footerBytes := make([]byte, FooterSize)
	if _, err := file.ReadAt(footerBytes, info.Size()-FooterSize); err != nil {
		t.Fatal(err)
	}

	f, err := DecodeFooter(footerBytes)
	if err != nil {
		t.Fatal(err)
	}

	if f.EntryCount != 2 {
		t.Fatalf("footer entry count = %d, want 2", f.EntryCount)
	}

	// Two filters occupy bytes [0, 128); data follows.
	dataBase := uint64(len(filter1) + len(filter2))
	want := []DirectoryEntry{
		{
			FilterOffset: 0,
			FilterSize:   64,
			K:            3,
			DataOffset:   dataBase,
			DataSize:     uint64(len(data1)),
			MinTimestamp: 1,
			MaxTimestamp: 2,
			RowCount:     1,
		},
		{
			FilterOffset: 64,
			FilterSize:   64,
			K:            4,
			DataOffset:   dataBase + uint64(len(data1)),
			DataSize:     uint64(len(data2)),
			MinTimestamp: 3,
			MaxTimestamp: 4,
			RowCount:     1,
		},
	}

	got, err := ReadDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("directory entries mismatch\n got: %#v\nwant: %#v", got, want)
	}

	directorySize := uint64(len(want) * DirectoryEntrySize)
	if f.DirectoryOffset+directorySize+FooterSize != uint64(info.Size()) {
		t.Fatalf("directory/footer bounds inconsistent with file size")
	}

	directoryBytes := make([]byte, directorySize)
	if _, err := file.ReadAt(directoryBytes, int64(f.DirectoryOffset)); err != nil {
		t.Fatal(err)
	}
	if crc32.ChecksumIEEE(directoryBytes) != f.DirectoryCRC {
		t.Fatal("directory CRC mismatch")
	}

	// Verify filters are contiguous at the beginning of the file.
	gotFilter1 := make([]byte, 64)
	gotFilter2 := make([]byte, 64)

	if _, err := file.ReadAt(gotFilter1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := file.ReadAt(gotFilter2, 64); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gotFilter1, filter1) {
		t.Fatal("first filter is not at offset 0")
	}
	if !bytes.Equal(gotFilter2, filter2) {
		t.Fatal("second filter is not at offset 64")
	}

	// Verify the log data region begins after both filters.
	gotData1 := make([]byte, len(data1))
	gotData2 := make([]byte, len(data2))

	if _, err := file.ReadAt(gotData1, int64(dataBase)); err != nil {
		t.Fatal(err)
	}
	if _, err := file.ReadAt(gotData2, int64(dataBase)+int64(len(data1))); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(gotData1, data1) || !bytes.Equal(gotData2, data2) {
		t.Fatal("log data region does not match original blocks")
	}
}

func TestNewStoreRefusesExistingSegment(t *testing.T) {
	dir := t.TempDir()

	s, err := newStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatal(err)
	}

	_, err = newStore(dir)
	if err == nil {
		t.Fatal("expected newStore to reject existing segment file")
	}
}

func TestStoreAppendRejectsClosedStore(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := s.close(); err != nil {
		t.Fatal(err)
	}

	if _, err := s.append([]byte("data")); err == nil {
		t.Fatal("expected append to fail after close")
	}
	if _, err := s.appendAlignedFilter(make([]byte, 64)); err == nil {
		t.Fatal("expected appendAlignedFilter to fail after close")
	}
}

func TestStoreFinalizeRejectsSecondFinalize(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err != nil {
		t.Fatal(err)
	}

	if err := s.Finalize(nil); err == nil {
		t.Fatal("expected second Finalize to fail")
	}
}

// Ensure the footer's fixed-width integer fields use little-endian encoding.
func TestFooterBinaryLayout(t *testing.T) {
	f := footer{
		Magic:           FooterMagic,
		Version:         FormatVersion,
		DirectoryOffset: 0x0807060504030201,
		EntryCount:      0x1817161514131211,
		DirectoryCRC:    0x24232221,
	}

	buf := f.Encode()
	if len(buf) != FooterSize {
		t.Fatalf("footer size = %d, want %d", len(buf), FooterSize)
	}

	if !bytes.Equal(buf[0:8], FooterMagic[:]) {
		t.Fatal("footer magic bytes mismatch")
	}
	if binary.LittleEndian.Uint32(buf[8:12]) != FormatVersion {
		t.Fatal("footer version encoding mismatch")
	}
	if binary.LittleEndian.Uint64(buf[12:20]) != f.DirectoryOffset {
		t.Fatal("directory offset encoding mismatch")
	}
	if binary.LittleEndian.Uint64(buf[20:28]) != f.EntryCount {
		t.Fatal("entry count encoding mismatch")
	}
	if binary.LittleEndian.Uint32(buf[28:32]) != f.DirectoryCRC {
		t.Fatal("directory CRC encoding mismatch")
	}
}
