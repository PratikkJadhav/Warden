package storage

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/PratikkJadhav/Warden/tokenizer"
)

func TestSearchLargeDataset(t *testing.T) {
	dir := t.TempDir()

	const (
		totalLines    = 20000
		linesPerBlock = 100
	)

	rng := rand.New(rand.NewSource(1))

	// Generate deterministic log lines.
	words := []string{
		"error", "payment", "timeout", "database",
		"request", "service", "connection", "failed",
		"success", "user", "login", "server",
	}

	lines := make([]string, totalLines)

	for i := range lines {
		word1 := words[rng.Intn(len(words))]
		word2 := words[rng.Intn(len(words))]
		word3 := words[rng.Intn(len(words))]

		lines[i] = fmt.Sprintf(
			"INFO service=%s action=%s status=%s request=%d",
			word1, word2, word3, i,
		)
	}

	// Plant five unique needles in five different blocks.
	needles := []string{
		"needle0001",
		"needle0002",
		"needle0003",
		"needle0004",
		"needle0005",
	}

	for i, needle := range needles {
		blockIndex := i * 40
		lineIndex := blockIndex*linesPerBlock + 50

		lines[lineIndex] = fmt.Sprintf(
			"ERROR payment timeout %s request=%d",
			needle, lineIndex,
		)
	}

	// Build the segment.
	path, err := BuildSegment(dir, lines, linesPerBlock)
	if err != nil {
		t.Fatalf("BuildSegment failed: %v", err)
	}

	// Verify the number of blocks.
	entries, err := ReadDirectory(path)
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}

	totalBlocks := len(entries)

	if totalBlocks != totalLines/linesPerBlock {
		t.Fatalf("expected 200 blocks, got %d", totalBlocks)
	}

	// Queries to test.
	queries := []string{
		"needle0001",
		"needle0002",
		"needle0003",
		"needle0004",
		"needle0005",
		"doesnotexist999",
		"error",
		"payment timeout",
	}

	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			// Indexed search.
			got, blocksRead, err := Search(path, query)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			// Brute-force search.
			queryTokens := tokenizer.Tokenize(query)

			want := make([]string, 0)
			for _, line := range lines {
				if lineMatchesTokens(line, queryTokens) {
					want = append(want, line)
				}
			}

			// Compare results.
			if !slices.Equal(got, want) {
				t.Fatalf(
					"results mismatch for query %q\nGot %d matches\nWant %d matches",
					query, len(got), len(want),
				)
			}

			t.Logf(
				"Query: %s | Matches: %d | Blocks read: %d / %d",
				query, len(got), blocksRead, totalBlocks,
			)

			// Each planted needle should require reading only its block.
			if len(query) >= 6 && query[:6] == "needle" {
				if blocksRead != 1 {
					t.Errorf(
						"expected 1 block read for %s, got %d",
						query, blocksRead,
					)
				}
			}
		})
	}
}
