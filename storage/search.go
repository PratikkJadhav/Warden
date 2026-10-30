package storage

import (
	"fmt"
	"strings"

	"github.com/PratikkJadhav/Warden/tokenizer"
)

func Search(path string, query string) (matches []string, blocksRead int, err error) {
	// 1. Tokenize query
	queryTokens := tokenizer.Tokenize(query)
	if len(queryTokens) == 0 {
		return nil, 0, fmt.Errorf("query contains no searchable tokens")
	}

	// Remove duplicate query tokens.
	uniqueTokens := make(map[string]struct{})
	var tokens []string

	for _, token := range queryTokens {
		if _, exists := uniqueTokens[token]; !exists {
			uniqueTokens[token] = struct{}{}
			tokens = append(tokens, token)
		}
	}

	// 2. Read segment directory
	entries, err := ReadDirectory(path)
	if err != nil {
		return nil, 0, err
	}

	// 3. Examine each block
	for _, entry := range entries {
		blockMightMatch := true

		for _, token := range tokens {
			mightContain, err := MightContain(path, entry, []byte(token))
			if err != nil {
				return nil, blocksRead, err
			}

			if !mightContain {
				blockMightMatch = false
				break
			}
		}

		// Bloom filter says at least one token is absent.
		if !blockMightMatch {
			continue
		}

		// 4. Read surviving block
		blockData, err := ReadBlock(path, entry)
		if err != nil {
			return nil, blocksRead, err
		}

		blocksRead++

		// 5. Verify actual log lines
		lines := strings.Split(string(blockData), "\n")

		for _, line := range lines {
			if lineMatchesTokens(line, tokens) {
				matches = append(matches, line)
			}
		}
	}

	return matches, blocksRead, nil
}

// A line matches only if it contains every query token.
func lineMatchesTokens(line string, queryTokens []string) bool {
	lineTokens := tokenizer.Tokenize(line)

	tokenSet := make(map[string]struct{}, len(lineTokens))
	for _, token := range lineTokens {
		tokenSet[token] = struct{}{}
	}

	for _, queryToken := range queryTokens {
		if _, exists := tokenSet[queryToken]; !exists {
			return false
		}
	}

	return true
}
