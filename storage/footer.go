package storage

import (
	"encoding/binary"
	"fmt"
)

const (
	FooterSize    = 32
	FormatVersion = uint32(1)
)

var FooterMagic = [8]byte{'W', 'A', 'R', 'D', 'E', 'N', '0', '1'}

type footer struct {
	Magic           [8]byte
	Version         uint32
	DirectoryOffset uint64
	EntryCount      uint64
	DirectoryCRC    uint32
}

func (f *footer) Encode() []byte {
	buf := make([]byte, FooterSize)
	pos := 0

	copy(buf[pos:pos+8], f.Magic[:])
	pos += 8

	binary.LittleEndian.PutUint32(buf[pos:], f.Version)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], f.DirectoryOffset)
	pos += 8

	binary.LittleEndian.PutUint64(buf[pos:], f.EntryCount)
	pos += 8

	binary.LittleEndian.PutUint32(buf[pos:], f.DirectoryCRC)

	return buf
}

func DecodeFooter(buf []byte) (footer, error) {
	if len(buf) != FooterSize {
		return footer{}, fmt.Errorf("invalid footer entry size: got %d, want %d", len(buf), FooterSize)
	}

	var f footer
	pos := 0
	copy(f.Magic[:], buf[pos:pos+8])
	pos += 8

	f.Version = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	f.DirectoryOffset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	f.EntryCount = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	f.DirectoryCRC = binary.LittleEndian.Uint32(buf[pos:])

	if f.Magic != FooterMagic {
		return footer{}, fmt.Errorf("invalid footer magic")
	}

	if f.Version != FormatVersion {
		return footer{}, fmt.Errorf("unsupported footer version: %d", f.Version)
	}

	return footer{}, fmt.Errorf(
		"invalid footer size: got %d, want %d",
		len(buf), FooterSize,
	)
}
