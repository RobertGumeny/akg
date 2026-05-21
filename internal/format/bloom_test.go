package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestBloomEncodeDecodeDeterministicAndValidatesV1Params(t *testing.T) {
	keys := [][]byte{[]byte("n:note:a"), []byte("n:note:b"), []byte("e:a:links:b")}
	one, err := EncodeBloom(keys)
	if err != nil {
		t.Fatal(err)
	}
	two, err := EncodeBloom(keys)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one, two) {
		t.Fatal("bloom encoding is not deterministic")
	}
	if got := binary.LittleEndian.Uint64(one[0:8]); got != 3 {
		t.Fatalf("key_count = %d", got)
	}
	if !bytes.Equal(one[0:8], []byte{3, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("key_count bytes are not little-endian: % x", one[0:8])
	}
	if got := binary.LittleEndian.Uint64(one[8:16]); got != 30 {
		t.Fatalf("bit_count = %d", got)
	}
	if !bytes.Equal(one[8:16], []byte{30, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("bit_count bytes are not little-endian: % x", one[8:16])
	}
	if one[16] != BloomHashFunctionCnt || binary.LittleEndian.Uint32(one[17:21]) != BloomHashSeed {
		t.Fatalf("bad fixed params")
	}
	if !bytes.Equal(one[17:21], []byte{0, 0, 0, 0}) {
		t.Fatalf("hash_seed bytes are not little-endian: % x", one[17:21])
	}
	bf, err := DecodeBloom(one)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if !bf.MayContain(key) {
			t.Fatalf("bloom false negative for %q", key)
		}
	}

	for _, keySet := range [][][]byte{
		{},
		{[]byte("single")},
		{[]byte{0x00}, []byte{0xff}, []byte("prefix"), []byte("prefix-longer")},
	} {
		encoded, err := EncodeBloom(keySet)
		if err != nil {
			t.Fatal(err)
		}
		again, err := EncodeBloom(keySet)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, again) {
			t.Fatalf("bloom encoding is not deterministic for keys %q", keySet)
		}
		decoded, err := DecodeBloom(encoded)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keySet {
			if !decoded.MayContain(key) {
				t.Fatalf("bloom false negative for %q", key)
			}
		}
	}

	tests := []struct {
		name string
		bad  []byte
	}{
		{"payload shorter than fixed header", one[:20]},
		{"wrong hash_function_count", corruptBloom(one, func(b []byte) { b[16] = 6 })},
		{"wrong hash_seed", corruptBloom(one, func(b []byte) { binary.LittleEndian.PutUint32(b[17:21], 1) })},
		{"wrong bit_count for key_count", corruptBloom(one, func(b []byte) { binary.LittleEndian.PutUint64(b[8:16], 31) })},
		{"wrong bit-array byte length", one[:len(one)-1]},
		{"non-zero padding bits beyond bit_count", corruptBloom(one, func(b []byte) { b[len(b)-1] |= 0xc0 })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeBloom(tt.bad); !errors.Is(err, ErrInvalidBloomSection) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func corruptBloom(in []byte, fn func([]byte)) []byte {
	out := append([]byte(nil), in...)
	fn(out)
	return out
}
