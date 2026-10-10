package storage

import (
	"fmt"
	"testing"

	"github.com/PratikkJadhav/Warden/bloom"
)

func TestDiskBloomLookup(t *testing.T) {
	const (
		insertCount = 10_000
		absentCount = 100_000
		lineSize    = 64
	)

	bf := bloom.NewBloom(insertCount, 0.0005)

	// Insert 10,000 known tokens.
	for i := 0; i < insertCount; i++ {
		token := fmt.Sprintf("inserted-token-%d", i)
		bf.Add([]byte(token))
	}

	serialized := bf.Bytes()

	if len(serialized)%lineSize != 0 {
		t.Fatalf("serialized filter size %d is not 64-byte aligned", len(serialized))
	}

	// Use an isolated temporary directory for this test.
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.file.Close()

	// Write the complete filter and remember where it starts.
	start, err := s.append(serialized)
	if err != nil {
		t.Fatal(err)
	}

	readLine := func(token string) []byte {
		t.Helper()

		lineIndex := bf.LineIndex([]byte(token))
		lineOffset := start + int64(lineIndex*lineSize)

		line := make([]byte, lineSize)
		n, err := s.file.ReadAt(line, lineOffset)
		if err != nil {
			t.Fatalf("ReadAt failed for %q: %v", token, err)
		}
		if n != lineSize {
			t.Fatalf("read %d bytes for %q; expected %d", n, token, lineSize)
		}

		return line
	}

	// Every inserted token must be found using only its disk line.
	for i := 0; i < insertCount; i++ {
		token := fmt.Sprintf("inserted-token-%d", i)
		line := readLine(token)

		diskAnswer := bf.MayContainInLine([]byte(token), line)
		memoryAnswer := bf.MayContain([]byte(token))

		if !diskAnswer {
			t.Fatalf("false negative for inserted token %q", token)
		}
		if diskAnswer != memoryAnswer {
			t.Fatalf("disk and memory answers differ for %q", token)
		}
	}

	// For absent tokens, disk and memory must agree.
	// Both answers may legitimately be true due to false positives.
	for i := 0; i < absentCount; i++ {
		token := fmt.Sprintf("absent-token-%d", i)
		line := readLine(token)

		diskAnswer := bf.MayContainInLine([]byte(token), line)
		memoryAnswer := bf.MayContain([]byte(token))

		if diskAnswer != memoryAnswer {
			t.Fatalf(
				"disk and memory answers differ for %q: disk=%v memory=%v",
				token, diskAnswer, memoryAnswer,
			)
		}
	}

	t.Logf(
		"verified %d inserted tokens and %d absent tokens",
		insertCount, absentCount,
	)
}
