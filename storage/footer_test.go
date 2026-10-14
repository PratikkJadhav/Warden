package storage

import (
	"testing"
)

func TestFooterEncodeDecode(t *testing.T) {
	original := footer{
		Magic:           FooterMagic,
		Version:         FormatVersion,
		DirectoryOffset: 1200,
		EntryCount:      3,
		DirectoryCRC:    123456,
	}

	encoded := original.Encode()

	if len(encoded) != FooterSize {
		t.Fatalf("encoded size = %d, want %d", len(encoded), FooterSize)
	}

	decoded, err := DecodeFooter(encoded)
	if err != nil {
		t.Fatalf("DecodeFooter() error: %v", err)
	}

	if decoded != original {
		t.Fatalf("decoded footer = %+v, want %+v", decoded, original)
	}
}

func TestDecodeFooterRejectsInvalidSize(t *testing.T) {
	_, err := DecodeFooter(make([]byte, FooterSize-1))
	if err == nil {
		t.Fatal("expected error for invalid footer size")
	}
}

func TestDecodeFooterRejectsInvalidMagic(t *testing.T) {
	f := footer{
		Magic:   [8]byte{'B', 'A', 'D', 'M', 'A', 'G', 'I', 'C'},
		Version: FormatVersion,
	}

	_, err := DecodeFooter(f.Encode())
	if err == nil {
		t.Fatal("expected error for invalid magic")
	}
}

func TestDecodeFooterRejectsUnsupportedVersion(t *testing.T) {
	f := footer{
		Magic:   FooterMagic,
		Version: 2,
	}

	_, err := DecodeFooter(f.Encode())
	if err == nil {
		t.Fatal("expected error for unsupported footer version")
	}
}
