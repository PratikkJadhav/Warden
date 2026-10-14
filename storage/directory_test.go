package storage

import (
	"reflect"
	"testing"
)

func TestDirectoryEntryEncodeDecode(t *testing.T) {
	// 1. Create an entry with known values.
	original := DirectoryEntry{
		FilterOffset: 128,
		FilterSize:   256,
		K:            11,
		DataOffset:   384,
		DataSize:     4096,
		MinTimestamp: 1000,
		MaxTimestamp: 2000,
		RowCount:     100,
	}

	// 2. Encode the entry into bytes.
	encoded := original.Encode()

	if len(encoded) != DirectoryEntrySize {
		t.Fatalf(
			"encoded size = %d, want %d",
			len(encoded), DirectoryEntrySize,
		)
	}

	// 3. Decode those bytes back into an entry.
	decoded, err := DecodeDirectoryEntry(encoded)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Verify every field matches.
	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf(
			"round trip mismatch:\noriginal: %+v\ndecoded:  %+v",
			original, decoded,
		)
	}
}

func TestDecodeDirectoryEntryRejectsWrongSize(t *testing.T) {
	_, err := DecodeDirectoryEntry(make([]byte, DirectoryEntrySize-1))
	if err == nil {
		t.Fatal("expected error for an incomplete directory entry")
	}
}
