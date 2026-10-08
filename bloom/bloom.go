package bloom

import (
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

	k := uint(math.Round(0.69 * bitsPerToken))

	numWords := totalBits / 32

	return &BloomFilter{
		bits: make([]uint32, numWords),
		k:    k,
	}

}

func (b *BloomFilter) Add(data []byte) {
	h := fnv.New64a()
	h.Write(data)
	hash := h.Sum64()
	noLines := len(b.bits) / 16
	bitline := hash % uint64(noLines)

	h1 := uint32(hash >> 32)
	h2 := uint32(hash)
	h2 |= 1

	for i := uint(0); i < b.k; i++ {
		position := (h1 + uint32(i)*h2) % 512
		globalBit := uint64(bitline)*512 + uint64(position)

		word := globalBit / 32
		bit := globalBit % 32

		b.bits[word] |= 1 << bit
	}
}

func (b *BloomFilter) MayContain(data []byte) bool {
	h := fnv.New64a()
	h.Write(data)
	hash := h.Sum64()

	noLines := len(b.bits) / 16
	bitline := hash % uint64(noLines)

	h1 := uint32(hash >> 32)
	h2 := uint32(hash)
	h2 |= 1

	for i := uint(0); i < b.k; i++ {
		position := (h1 + uint32(i)*h2) % 512

		globalBit := uint64(bitline)*512 + uint64(position)

		word := globalBit / 32
		bit := globalBit % 32

		if b.bits[word]&(1<<bit) == 0 {
			return false
		}
	}

	return true
}
