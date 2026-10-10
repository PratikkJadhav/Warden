package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type store struct {
	file          *os.File
	currentfileID uint32
	dir           string
	offset        int64
}

func newStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	fileID := uint32(1)
	filename := fmt.Sprintf("seg-%06d.dat", fileID)
	path := filepath.Join(dir, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, err
	}

	return &store{
		file:          file,
		currentfileID: fileID,
		offset:        offset,
		dir:           dir,
	}, err
}

// func TestBloomSample() {
// 	tokens1 := tokenizer.Tokenize("test123 key value time10")

// 	bf := bloom.NewBloom(100, 0.0005)
// 	for _, token := range tokens1 {
// 		bf.Add([]byte(token))
// 	}

// 	s1, err := newStore("/mnt/d/Pratik/Projects/Warden")
// 	if err != nil {
// 		return
// 	}

// 	s1.file.Write(bf.Bytes().Bytes())

// }
