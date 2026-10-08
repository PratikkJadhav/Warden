package bloom

import (
	"testing"
)

func TestBloomContainsInsertedData(t *testing.T) {
	b := NewBloom(1000, 0.001)

	b.Add([]byte("abc123"))

	if !b.MayContain([]byte("abc123")) {
		t.Error("Bloom filter says inserted item is not present")
	}
}

func TestBloomDoesNotContainMissingData(t *testing.T) {
	b := NewBloom(1000, 0.001)

	b.Add([]byte("abc123"))

	if b.MayContain([]byte("xyz789")) {
		t.Error("Bloom filter says missing item may be present")
	}
}
