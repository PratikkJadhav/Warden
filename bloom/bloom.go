package bloom

type BloomFilter struct {
	bits []uint32 //bit array
	k    uint     // how many positions per item
}

func NewBloom(n uint32, p float32) *BloomFilter {

}

func (b *BloomFilter) Add(data []byte) {

}

func (b *BloomFilter) Maybe(data []byte) bool {

}
