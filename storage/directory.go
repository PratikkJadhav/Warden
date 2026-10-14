package storage

import (
	"encoding/binary"
	"fmt"
)

const DirectoryEntrySize = 60

type DirectoryEntry struct {
	FilterOffset uint64
	FilterSize   uint64
	K            uint32
	DataOffset   uint64
	DataSize     uint64
	MinTimestamp int64
	MaxTimestamp int64
	RowCount     uint64
}

func (e DirectoryEntry) Encode() []byte {
	buf := make([]byte, DirectoryEntrySize)
	pos := 0

	binary.LittleEndian.PutUint64(buf[pos:], e.FilterOffset)
	pos += 8

	binary.LittleEndian.PutUint64(buf[pos:], e.FilterSize)
	pos += 8

	binary.LittleEndian.PutUint32(buf[pos:], e.K)
	pos += 4

	binary.LittleEndian.PutUint64(buf[pos:], e.DataOffset)
	pos += 8

	binary.LittleEndian.PutUint64(buf[pos:], e.DataSize)
	pos += 8

	binary.LittleEndian.PutUint64(buf[pos:], uint64(e.MinTimestamp))
	pos += 8

	binary.LittleEndian.PutUint64(buf[pos:], uint64(e.MaxTimestamp))
	pos += 8

	binary.LittleEndian.PutUint64(buf[pos:], e.RowCount)

	return buf
}

func DecodeDirectoryEntry(buf []byte) (DirectoryEntry, error) {
	if len(buf) != DirectoryEntrySize {
		return DirectoryEntry{}, fmt.Errorf("invalid directory entry size: got %d, want %d", len(buf), DirectoryEntrySize)
	}

	pos := 0
	var e DirectoryEntry
	e.FilterOffset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	e.FilterSize = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	e.K = binary.LittleEndian.Uint32(buf[pos:])
	pos += 4

	e.DataOffset = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	e.DataSize = binary.LittleEndian.Uint64(buf[pos:])
	pos += 8

	e.MinTimestamp = int64(binary.LittleEndian.Uint64(buf[pos:]))
	pos += 8

	e.MaxTimestamp = int64(binary.LittleEndian.Uint64(buf[pos:]))
	pos += 8

	e.RowCount = binary.LittleEndian.Uint64(buf[pos:])

	return e, nil
}
