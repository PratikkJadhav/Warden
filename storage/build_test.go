package storage

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSegment(t *testing.T) {
	dir := t.TempDir()

	// Generate 250 log lines.
	lines := make([]string, 250)
	for i := range lines {
		lines[i] = fmt.Sprintf(
			"INFO service payment request=%d status=200",
			i,
		)
	}

	// Build segment with 100 lines per block.
	path, err := BuildSegment(dir, lines, 100)
	if err != nil {
		t.Fatalf("BuildSegment failed: %v", err)
	}

	// Read directory entries.
	entries, err := ReadDirectory(path)
	if err != nil {
		t.Fatalf("ReadDirectory failed: %v", err)
	}

	// Expect 3 blocks.
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify row counts.
	wantRowCounts := []uint64{100, 100, 50}

	for i, entry := range entries {
		if entry.RowCount != wantRowCounts[i] {
			t.Errorf(
				"block %d: expected %d rows, got %d",
				i,
				wantRowCounts[i],
				entry.RowCount,
			)
		}
	}

	// Read all blocks and reconstruct original lines.
	var gotLines []string

	for i, entry := range entries {
		block, err := ReadBlock(path, entry)
		if err != nil {
			t.Fatalf("ReadBlock failed for block %d: %v", i, err)
		}

		blockLines := strings.Split(string(block), "\n")
		gotLines = append(gotLines, blockLines...)
	}

	// Verify original order and content.
	if !reflect.DeepEqual(gotLines, lines) {
		t.Fatal("reconstructed lines do not match original input")
	}
}
