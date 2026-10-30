package storage

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PratikkJadhav/Warden/bloom"
	"github.com/PratikkJadhav/Warden/tokenizer"
)

func BuildSegment(dir string, lines []string, linesPerBlock int) (path string, err error) {
	if linesPerBlock <= 0 {
		return "", fmt.Errorf("linesPerBlock must be greater than zero")
	}

	// 1. Create store
	s, err := newStore(dir)
	if err != nil {
		return "", err
	}

	// 2. Divide lines into blocks
	var entries []DirectoryEntry

	for start := 0; start < len(lines); start += linesPerBlock {
		end := min(start+linesPerBlock, len(lines))
		blockLines := lines[start:end]

		// 3. Tokenize each line and collect distinct tokens
		distinctTokens := make(map[string]struct{})

		for _, line := range blockLines {
			tokens := tokenizer.Tokenize(line)

			for _, token := range tokens {
				distinctTokens[token] = struct{}{}
			}
		}

		// Create Bloom filter
		if len(distinctTokens) == 0 {
			return "", fmt.Errorf("block starting at line %d contains no tokens", start)
		}

		filter := bloom.NewBloom(uint32(len(distinctTokens)), 0.0005)

		for token := range distinctTokens {
			filter.Add([]byte(token))
		}

		filterBytes := filter.Bytes()

		// 4. Write filter and block data
		filterOffset, err := s.appendAlignedFilter(filterBytes)
		if err != nil {
			return "", err
		}

		blockData := []byte(strings.Join(blockLines, "\n"))

		dataOffset, err := s.append(blockData)
		if err != nil {
			return "", err
		}

		// 5. Create directory entry
		entry := DirectoryEntry{
			FilterOffset: uint64(filterOffset),
			FilterSize:   uint64(len(filterBytes)),
			K:            uint32(filter.K()),
			DataOffset:   uint64(dataOffset),
			DataSize:     uint64(len(blockData)),
			RowCount:     uint64(len(blockLines)),
		}

		entries = append(entries, entry)
	}

	// 6. Finalize segment
	if err := s.Finalize(entries); err != nil {
		return "", err
	}

	// 7. Return segment path
	path = filepath.Join(dir, "seg-000001.dat")
	return path, nil
}
