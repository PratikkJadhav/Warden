package tokenizer

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	got := Tokenize("ERROR payment timeout req=abc123 status=503")
	want := []string{"error", "payment", "timeout", "req", "abc123", "status", "503"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
