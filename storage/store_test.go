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

	// Insert known tokens.
	for i := 0; i < insertCount; i++ {
		token := fmt.Sprintf("inserted-token-%d", i)
		bf.Add([]byte(token))
	}

	serialized := bf.Bytes()

	if len(serialized)%lineSize != 0 {
		t.Fatalf("filter size %d is not line-aligned", len(serialized))
	}

	// Metadata that a future directory entry will store.
	k := bf.K()
	numLines := uint64(len(serialized) / lineSize)

	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.file.Close()

	start, err := s.append(serialized)
	if err != nil {
		t.Fatal(err)
	}

	readLine := func(token string) []byte {
		t.Helper()

		lineIndex, err := bloom.LineIndex(
			[]byte(token),
			numLines,
		)
		if err != nil {
			t.Fatal(err)
		}

		offset := start + int64(lineIndex)*lineSize
		line := make([]byte, lineSize)

		n, err := s.file.ReadAt(line, offset)
		if err != nil {
			t.Fatalf("ReadAt for %q: %v", token, err)
		}
		if n != lineSize {
			t.Fatalf("read %d bytes; expected %d", n, lineSize)
		}

		return line
	}

	// Inserted tokens must never be reported absent.
	for i := 0; i < insertCount; i++ {
		token := fmt.Sprintf("inserted-token-%d", i)
		line := readLine(token)

		diskAnswer, err := bloom.MayContainInLine(
			[]byte(token), k, line,
		)
		if err != nil {
			t.Fatal(err)
		}

		if !diskAnswer {
			t.Fatalf("false negative for %q", token)
		}

		if diskAnswer != bf.MayContain([]byte(token)) {
			t.Fatalf("disk/memory mismatch for %q", token)
		}
	}

	// Absent tokens may produce false positives, but disk
	// and memory must always return the same answer.
	for i := 0; i < absentCount; i++ {
		token := fmt.Sprintf("absent-token-%d", i)
		line := readLine(token)

		diskAnswer, err := bloom.MayContainInLine(
			[]byte(token), k, line,
		)
		if err != nil {
			t.Fatal(err)
		}

		memoryAnswer := bf.MayContain([]byte(token))

		if diskAnswer != memoryAnswer {
			t.Fatalf(
				"disk/memory mismatch for %q: disk=%v memory=%v",
				token, diskAnswer, memoryAnswer,
			)
		}
	}

	t.Logf(
		"verified %d inserted and %d absent tokens",
		insertCount, absentCount,
	)
}
