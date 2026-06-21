package fingerprint

import "fmt"

func murmurHash128(key string, seed uint32) string {
	remainder := len(key) % 16
	bytes := len(key) - remainder
	h1 := [2]uint32{0, seed}
	h2 := [2]uint32{0, seed}
	c1 := [2]uint32{0x87c37b91, 0x114253d5}
	c2 := [2]uint32{0x4cf5ad43, 0x2745937f}

	for i := 0; i < bytes; i += 16 {
		k1 := [2]uint32{
			uint32(key[i+4]) | (uint32(key[i+5]) << 8) | (uint32(key[i+6]) << 16) | (uint32(key[i+7]) << 24),
			uint32(key[i]) | (uint32(key[i+1]) << 8) | (uint32(key[i+2]) << 16) | (uint32(key[i+3]) << 24),
		}
		k2 := [2]uint32{
			uint32(key[i+12]) | (uint32(key[i+13]) << 8) | (uint32(key[i+14]) << 16) | (uint32(key[i+15]) << 24),
			uint32(key[i+8]) | (uint32(key[i+9]) << 8) | (uint32(key[i+10]) << 16) | (uint32(key[i+11]) << 24),
		}
		k1 = murmurX64Multiply(k1, c1)
		k1 = murmurX64Rotl(k1, 31)
		k1 = murmurX64Multiply(k1, c2)
		h1 = murmurX64Xor(h1, k1)
		h1 = murmurX64Rotl(h1, 27)
		h1 = murmurX64Add(h1, h2)
		h1 = murmurX64Add(murmurX64Multiply(h1, [2]uint32{0, 5}), [2]uint32{0, 0x52dce729})
		k2 = murmurX64Multiply(k2, c2)
		k2 = murmurX64Rotl(k2, 33)
		k2 = murmurX64Multiply(k2, c1)
		h2 = murmurX64Xor(h2, k2)
		h2 = murmurX64Rotl(h2, 31)
		h2 = murmurX64Add(h2, h1)
		h2 = murmurX64Add(murmurX64Multiply(h2, [2]uint32{0, 5}), [2]uint32{0, 0x38495ab5})
	}

	var k1, k2 [2]uint32
	i := bytes
	switch remainder {
	case 15:
		k2 = murmurX64Xor(k2, murmurX64LeftShift([2]uint32{0, uint32(key[i+14])}, 48))
		fallthrough
	case 14:
		k2 = murmurX64Xor(k2, murmurX64LeftShift([2]uint32{0, uint32(key[i+13])}, 40))
		fallthrough
	case 13:
		k2 = murmurX64Xor(k2, murmurX64LeftShift([2]uint32{0, uint32(key[i+12])}, 32))
		fallthrough
	case 12:
		k2 = murmurX64Xor(k2, murmurX64LeftShift([2]uint32{0, uint32(key[i+11])}, 24))
		fallthrough
	case 11:
		k2 = murmurX64Xor(k2, murmurX64LeftShift([2]uint32{0, uint32(key[i+10])}, 16))
		fallthrough
	case 10:
		k2 = murmurX64Xor(k2, murmurX64LeftShift([2]uint32{0, uint32(key[i+9])}, 8))
		fallthrough
	case 9:
		k2 = murmurX64Xor(k2, [2]uint32{0, uint32(key[i+8])})
		k2 = murmurX64Multiply(k2, c2)
		k2 = murmurX64Rotl(k2, 33)
		k2 = murmurX64Multiply(k2, c1)
		h2 = murmurX64Xor(h2, k2)
		fallthrough
	case 8:
		k1 = murmurX64Xor(k1, murmurX64LeftShift([2]uint32{0, uint32(key[i+7])}, 56))
		fallthrough
	case 7:
		k1 = murmurX64Xor(k1, murmurX64LeftShift([2]uint32{0, uint32(key[i+6])}, 48))
		fallthrough
	case 6:
		k1 = murmurX64Xor(k1, murmurX64LeftShift([2]uint32{0, uint32(key[i+5])}, 40))
		fallthrough
	case 5:
		k1 = murmurX64Xor(k1, murmurX64LeftShift([2]uint32{0, uint32(key[i+4])}, 32))
		fallthrough
	case 4:
		k1 = murmurX64Xor(k1, murmurX64LeftShift([2]uint32{0, uint32(key[i+3])}, 24))
		fallthrough
	case 3:
		k1 = murmurX64Xor(k1, murmurX64LeftShift([2]uint32{0, uint32(key[i+2])}, 16))
		fallthrough
	case 2:
		k1 = murmurX64Xor(k1, murmurX64LeftShift([2]uint32{0, uint32(key[i+1])}, 8))
		fallthrough
	case 1:
		k1 = murmurX64Xor(k1, [2]uint32{0, uint32(key[i])})
		k1 = murmurX64Multiply(k1, c1)
		k1 = murmurX64Rotl(k1, 31)
		k1 = murmurX64Multiply(k1, c2)
		h1 = murmurX64Xor(h1, k1)
	}

	h1 = murmurX64Xor(h1, [2]uint32{0, uint32(len(key))})
	h2 = murmurX64Xor(h2, [2]uint32{0, uint32(len(key))})
	h1 = murmurX64Add(h1, h2)
	h2 = murmurX64Add(h2, h1)
	h1 = murmurX64Fmix(h1)
	h2 = murmurX64Fmix(h2)
	h1 = murmurX64Add(h1, h2)
	h2 = murmurX64Add(h2, h1)

	return fmt.Sprintf("%08x%08x%08x%08x", h1[0], h1[1], h2[0], h2[1])
}
