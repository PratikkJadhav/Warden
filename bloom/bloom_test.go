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

	for _, p := range []float64{0.05, 0.001} {
		t.Run(fmt.Sprintf("p=%g", p), func(t *testing.T) {
			b := NewBloom(n, p)

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
