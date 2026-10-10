package storage

import (
	"bytes"
	"testing"

	"github.com/PratikkJadhav/Warden/bloom"
	"github.com/PratikkJadhav/Warden/tokenizer"
)

func TestBloomSample(t *testing.T) {
	tokens := tokenizer.Tokenize("test123 key value time10")

	bf := bloom.NewBloom(100, 0.0005)
	for _, token := range tokens {
		bf.Add([]byte(token))
	}

	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.file.Close()

	serialized := bf.Bytes().Bytes()

	n, err := s.file.Write(serialized)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(serialized) {
		t.Fatalf("wrote %d bytes; expected %d", n, len(serialized))
	}

	// Compare the first 64-byte line.
	expected := serialized[:64]
	actual := make([]byte, 64)

	_, err = s.file.ReadAt(actual, s.offset)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(expected, actual) {
		t.Fatal("disk line does not match in-memory line")
	}

	t.Log("first 64-byte line matches")
}
