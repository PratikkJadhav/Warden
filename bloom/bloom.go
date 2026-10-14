package bloom

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
)

const (
	lineBits  = 512
	lineBytes = 64
	lineWords = 16
)

type BloomFilter struct {
	bits []uint32 //bit array
	k    uint     // how many positions per item
}

func NewBloom(n uint32, p float64) *BloomFilter {
	bitsPerToken := 1.44 * math.Log2(1/p)

	totalBits := uint32(math.Ceil(float64(n) * bitsPerToken))
	totalBits = ((totalBits + 511) / 512) * 512

	if totalBits < 512 {
		totalBits = 512
	}

	k := uint(math.Round(0.69 * bitsPerToken))

	numWords := totalBits / 32

	return &BloomFilter{
		bits: make([]uint32, numWords),
		k:    k,
	}

}

func (b *BloomFilter) Add(data []byte) {
	bitline, hash := b.hash(data)
	for i := uint(0); i < b.k; i++ {
		pos := position(hash, i)
		globalBit := uint64(bitline)*512 + uint64(pos)

		word := globalBit / 32
		bit := globalBit % 32

		b.bits[word] |= 1 << bit
	}
}

func hashToken(data []byte) uint64 {
	h := fnv.New64a()
	h.Write(data)
	hash := h.Sum64()

	return fmix64(hash)
}

func (b *BloomFilter) hash(data []byte) (uint64, uint64) {
	h := fnv.New64a()
	h.Write(data)
	hash := h.Sum64()

	hash = fmix64(hash)

	noLines := len(b.bits) / 16
	bitline := hash % uint64(noLines)

	return bitline, hash
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}

func (b *BloomFilter) K() uint {
	return b.k
}

func position(h uint64, i uint) uint32 {
	const step uint64 = 0x9e3779b97f4a7c15

	x := h + uint64(i)*step
	x = fmix64(x)

	return uint32(x & 511)
}

func (b *BloomFilter) NumLines() uint64 {
	return uint64(len(b.bits) / lineWords)
}

func (b *BloomFilter) Bytes() []byte {
	data := make([]byte, len(b.bits)*4)

	for i, word := range b.bits {
		binary.LittleEndian.PutUint32(data[i*4:], word)
	}

	return data
}

func LineIndex(data []byte, numLines uint64) (uint64, error) {
	if numLines == 0 {
		return 0, fmt.Errorf("line count must be greater than zero")
	}

	return hashToken(data) % numLines, nil
}

func (b *BloomFilter) MayContain(data []byte) bool {
	lineIndex, hash := b.hash(data)

	for i := uint(0); i < b.k; i++ {
		pos := position(hash, i)
		globalBit := lineIndex*lineBits + uint64(pos)

		word := globalBit / 32
		bit := globalBit % 32

		if b.bits[word]&(uint32(1)<<bit) == 0 {
			return false
		}
	}

	return true
}
func MayContainInLine(token []byte, k uint, line []byte) (bool, error) {
	if len(line) != lineBytes {
		return false, fmt.Errorf("invalid line length: got %d bytes, want %d", len(line), lineBytes)
	}

	hash := hashToken(token)

	for i := uint(0); i < k; i++ {
		pos := position(hash, i)
		byteIndex := pos / 8
		bitIndex := pos % 8

		if line[byteIndex]&(byte(1)<<bitIndex) == 0 {
			return false, nil
		}
	}

	return true, nil
}

func (b *BloomFilter) lineByte(lineIndex uint64) []byte {
	start := int(lineIndex) * lineWords
	line := make([]byte, lineBytes)
	for i := 0; i < lineWords; i++ {
		binary.LittleEndian.PutUint32(
			line[i*4:],
			b.bits[start+i],
		)
	}
	return line
}
