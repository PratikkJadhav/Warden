package bloom

import (
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"math"
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

func (b *BloomFilter) MayContain(data []byte) bool {
	bitline, hash := b.hash(data)

	for i := uint(0); i < b.k; i++ {
		pos := position(hash, i)

		globalBit := uint64(bitline)*512 + uint64(pos)

		word := globalBit / 32
		bit := globalBit % 32

		if b.bits[word]&(1<<bit) == 0 {
			return false
		}
	}

	return true
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

func position(h uint64, i uint) uint32 {
	const step uint64 = 0x9e3779b97f4a7c15

	x := h + uint64(i)*step
	x = fmix64(x)

	return uint32(x & 511)
}

func (b *BloomFilter) Bytes() *bytes.Buffer {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, b.bits)

	if err != nil {
		return nil
	}

	return buf
}
