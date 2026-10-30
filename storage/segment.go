package storage

import (
	"github.com/PratikkJadhav/Warden/bloom"
)

// MightContain checks whether a block may contain the given token.
func MightContain(path string, entry DirectoryEntry, token []byte) (bool, error) {
	numLines := entry.FilterSize / uint64(BloomLineSize)

	lineIndex, err := bloom.LineIndex(token, numLines)
	if err != nil {
		return false, err
	}

	line, err := ReadFilterLine(path, entry, lineIndex)
	if err != nil {
		return false, err
	}

	return bloom.MayContainInLine(token, uint(entry.K), line)
}
