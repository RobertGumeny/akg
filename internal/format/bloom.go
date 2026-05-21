package format

import (
	"encoding/binary"
	"math/bits"
)

const (
	BloomBitsPerKey      = 10
	BloomHashFunctionCnt = 7
	BloomHashSeed        = 0
)

var ErrInvalidBloomSection = ErrInvalidSectionTable

// BloomFilter is the deterministic AKG v1 Bloom-section payload shape.
type BloomFilter struct {
	KeyCount          uint64
	BitCount          uint64
	HashFunctionCount uint8
	HashSeed          uint32
	Bits              []byte
}

// EncodeBloom builds the canonical AKG v1 Bloom payload for keys.
func EncodeBloom(keys [][]byte) ([]byte, error) {
	keyCount := uint64(len(keys))
	bitCount := keyCount * BloomBitsPerKey
	bitsLen := int((bitCount + 7) / 8)
	bf := BloomFilter{KeyCount: keyCount, BitCount: bitCount, HashFunctionCount: BloomHashFunctionCnt, HashSeed: BloomHashSeed, Bits: make([]byte, bitsLen)}
	for _, key := range keys {
		setBloomKey(&bf, key)
	}
	buf := make([]byte, 8+8+1+4+len(bf.Bits))
	binary.LittleEndian.PutUint64(buf[0:8], bf.KeyCount)
	binary.LittleEndian.PutUint64(buf[8:16], bf.BitCount)
	buf[16] = bf.HashFunctionCount
	binary.LittleEndian.PutUint32(buf[17:21], bf.HashSeed)
	copy(buf[21:], bf.Bits)
	return buf, nil
}

// DecodeBloom decodes and validates AKG v1 Bloom fixed parameters.
func DecodeBloom(payload []byte) (BloomFilter, error) {
	if len(payload) < 21 {
		return BloomFilter{}, ErrInvalidBloomSection
	}
	bf := BloomFilter{
		KeyCount:          binary.LittleEndian.Uint64(payload[0:8]),
		BitCount:          binary.LittleEndian.Uint64(payload[8:16]),
		HashFunctionCount: payload[16],
		HashSeed:          binary.LittleEndian.Uint32(payload[17:21]),
		Bits:              append([]byte(nil), payload[21:]...),
	}
	if bf.HashFunctionCount != BloomHashFunctionCnt || bf.HashSeed != BloomHashSeed || bf.BitCount != bf.KeyCount*BloomBitsPerKey {
		return BloomFilter{}, ErrInvalidBloomSection
	}
	if uint64(len(bf.Bits)) != (bf.BitCount+7)/8 {
		return BloomFilter{}, ErrInvalidBloomSection
	}
	if extra := bf.BitCount % 8; extra != 0 && len(bf.Bits) > 0 {
		mask := byte(0xff << extra)
		if bf.Bits[len(bf.Bits)-1]&mask != 0 {
			return BloomFilter{}, ErrInvalidBloomSection
		}
	}
	return bf, nil
}

// MayContain reports Bloom membership. A false result is definitive absence.
func (bf BloomFilter) MayContain(key []byte) bool {
	if bf.BitCount == 0 {
		return false
	}
	h1, h2 := murmur3x64_128(key, bf.HashSeed)
	for i := uint8(0); i < bf.HashFunctionCount; i++ {
		idx := (h1 + uint64(i)*h2) % bf.BitCount
		if bf.Bits[idx/8]&(1<<(idx%8)) == 0 {
			return false
		}
	}
	return true
}

func setBloomKey(bf *BloomFilter, key []byte) {
	if bf.BitCount == 0 {
		return
	}
	h1, h2 := murmur3x64_128(key, bf.HashSeed)
	for i := uint8(0); i < bf.HashFunctionCount; i++ {
		idx := (h1 + uint64(i)*h2) % bf.BitCount
		bf.Bits[idx/8] |= 1 << (idx % 8)
	}
}

func murmur3x64_128(data []byte, seed uint32) (uint64, uint64) {
	const c1 uint64 = 0x87c37b91114253d5
	const c2 uint64 = 0x4cf5ad432745937f
	h1, h2 := uint64(seed), uint64(seed)
	nblocks := len(data) / 16
	for i := 0; i < nblocks; i++ {
		block := data[i*16:]
		k1 := binary.LittleEndian.Uint64(block[0:8])
		k2 := binary.LittleEndian.Uint64(block[8:16])
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
		h1 = bits.RotateLeft64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
		h2 = bits.RotateLeft64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}
	var k1, k2 uint64
	tail := data[nblocks*16:]
	for i := 0; i < len(tail) && i < 8; i++ {
		k1 |= uint64(tail[i]) << (8 * i)
	}
	for i := 8; i < len(tail); i++ {
		k2 |= uint64(tail[i]) << (8 * (i - 8))
	}
	if k2 != 0 {
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
	}
	if k1 != 0 {
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint64(len(data))
	h2 ^= uint64(len(data))
	h1 += h2
	h2 += h1
	h1 = fmix64(h1)
	h2 = fmix64(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}
