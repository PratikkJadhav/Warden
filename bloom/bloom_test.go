package bloom

import (
	"fmt"
	"testing"
)

func TestNoFalseNegatives(t *testing.T) {
	const n = 100_000

	b := NewBloom(n, 0.01)

	for i := 0; i < n; i++ {
		b.Add([]byte(fmt.Sprintf("key-%d", i)))
	}

	for i := 0; i < n; i++ {
		key := []byte(fmt.Sprintf("key-%d", i))

		if !b.MayContain(key) {
			t.Fatalf("false negative for %s", key)
		}
	}
}

func TestFalsePositiveRate(t *testing.T) {
	const n = 1_000_000

	for _, p := range []float64{0.05, 0.0007, 0.0006, 0.0005} {
		t.Run(fmt.Sprintf("p=%g", p), func(t *testing.T) {
			b := NewBloom(n, p)

			actualBitsPerToken := float64(len(b.bits)*32) / float64(n)
			t.Logf("target=%f bits_per_token=%.2f", p, actualBitsPerToken)

			// Insert 1M keys.
			for i := 0; i < n; i++ {
				b.Add([]byte(fmt.Sprintf("key-%d", i)))
			}

			// Query 1M keys that were never inserted.
			falsePositives := 0

			for i := 0; i < n; i++ {
				key := []byte(fmt.Sprintf("other-%d", i))

				if b.MayContain(key) {
					falsePositives++
				}
			}

			rate := float64(falsePositives) / float64(n)

			t.Logf(
				"target=%f false_positives=%d rate=%f",
				p,
				falsePositives,
				rate,
			)
		})
	}
}

func TestMayContainInLineMatchesMayContain(t *testing.T) {
	filter := NewBloom(1000, 0.01)

	tokens := [][]byte{
		[]byte("payment"),
		[]byte("timeout"),
		[]byte("request"),
		[]byte("status"),
	}

	for _, token := range tokens {
		filter.Add(token)
	}

	for _, token := range tokens {
		lineIndex, err := LineIndex(token, filter.NumLines())
		if err != nil {
			t.Fatal(err)
		}

		line := filter.lineByte(lineIndex)

		got, err := MayContainInLine(token, filter.K(), line)
		if err != nil {
			t.Fatal(err)
		}

		want := filter.MayContain(token)
		if got != want {
			t.Errorf(
				"token %q: MayContainInLine() = %v, MayContain() = %v",
				token, got, want,
			)
		}

		if !got {
			t.Errorf("inserted token %q was reported absent", token)
		}
	}
}

func TestMayContainInLineMatchesMayContainForAbsentTokens(t *testing.T) {
	filter := NewBloom(1000, 0.01)

	filter.Add([]byte("payment"))
	filter.Add([]byte("timeout"))

	tokens := [][]byte{
		[]byte("database"),
		[]byte("authentication"),
		[]byte("missing-request"),
	}

	for _, token := range tokens {
		lineIndex, err := LineIndex(token, filter.NumLines())
		if err != nil {
			t.Fatal(err)
		}

		line := filter.lineByte(lineIndex)

		got, err := MayContainInLine(token, filter.K(), line)
		if err != nil {
			t.Fatal(err)
		}

		want := filter.MayContain(token)
		if got != want {
			t.Errorf(
				"token %q: standalone probe = %v, in-memory probe = %v",
				token, got, want,
			)
		}
	}
}
