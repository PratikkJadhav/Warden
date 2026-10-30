package storage

import (
	"fmt"
	"os"
)

// ReadFilterLine reads one 64-byte Bloom filter line from a segment.
func ReadFilterLine(path string, entry DirectoryEntry, lineIndex uint64) ([]byte, error) {

	numFilter := entry.FilterSize / uint64(BloomLineSize)

	if lineIndex >= numFilter {
		return nil, fmt.Errorf("LineIndex is greater than number of filters")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	offset := entry.FilterOffset + lineIndex*uint64(BloomLineSize)

	line := make([]byte, 64)
	n, err := file.ReadAt(line, int64(offset))

	if err != nil {
		return nil, err
	}

	if n != len(line) {
		return nil, fmt.Errorf("incomplete filter line: read %d bytes", n)
	}
	return line, nil
}
