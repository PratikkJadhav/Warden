package storage

import (
	"fmt"
	"hash/crc32"
	"os"
)

func ReadDirectory(path string) ([]DirectoryEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileSize := info.Size()

	if fileSize < FooterSize {
		return nil, fmt.Errorf("segment too small to contain footer")
	}

	footerBytes := make([]byte, FooterSize)

	_, err = file.ReadAt(footerBytes, fileSize-FooterSize)
	if err != nil {
		return nil, err
	}

	f, err := DecodeFooter(footerBytes)
	if err != nil {
		return nil, err
	}

	if f.DirectoryOffset > uint64(fileSize-FooterSize) {
		return nil, fmt.Errorf("directory offset is outside segment")
	}

	directoryBytesAvailable := uint64(fileSize-FooterSize) - f.DirectoryOffset

	if f.EntryCount > directoryBytesAvailable/DirectoryEntrySize {
		return nil, fmt.Errorf("directory entry count exceeds segment bounds")
	}

	directorySize := f.EntryCount * DirectoryEntrySize

	if f.DirectoryOffset+directorySize+FooterSize != uint64(fileSize) {
		return nil, fmt.Errorf("invalid directory bounds")
	}
	maxInt := uint64(^uint(0) >> 1)
	if directorySize > maxInt {
		return nil, fmt.Errorf("directory is too large to fit in memory")
	}

	directoryBytes := make([]byte, int(directorySize))

	_, err = file.ReadAt(directoryBytes, int64(f.DirectoryOffset))
	if err != nil {
		return nil, err
	}

	gotCRC := crc32.ChecksumIEEE(directoryBytes)

	if gotCRC != f.DirectoryCRC {
		return nil, fmt.Errorf("directory checksum mismatch")
	}

	if f.EntryCount > maxInt {
		return nil, fmt.Errorf("too many directory entries")
	}

	entries := make([]DirectoryEntry, int(f.EntryCount))

	for i := range entries {
		start := i * DirectoryEntrySize
		end := start + DirectoryEntrySize

		entry, err := DecodeDirectoryEntry(directoryBytes[start:end])
		if err != nil {
			return nil, err
		}

		if err := validateEntry(entry, f.DirectoryOffset); err != nil {
			return nil, fmt.Errorf("invalid directory entry %d: %w", i, err)
		}

		entries[i] = entry
	}

	return entries, nil

}

func validateEntry(entry DirectoryEntry, directoryOffset uint64) error {
	if entry.FilterSize == 0 {
		return fmt.Errorf("filter size must be greater than zero")
	}

	if entry.FilterSize%uint64(BloomLineSize) != 0 {
		return fmt.Errorf("filter size must be a multiple of %d", BloomLineSize)
	}

	if entry.FilterOffset%uint64(BloomLineSize) != 0 {
		return fmt.Errorf("filter offset must be aligned to %d bytes", BloomLineSize)
	}

	if entry.FilterOffset > directoryOffset ||
		entry.FilterSize > directoryOffset-entry.FilterOffset {
		return fmt.Errorf("filter range exceeds segment data")
	}

	if entry.DataOffset > directoryOffset ||
		entry.DataSize > directoryOffset-entry.DataOffset {
		return fmt.Errorf("data range exceeds segment data")
	}

	return nil
}
