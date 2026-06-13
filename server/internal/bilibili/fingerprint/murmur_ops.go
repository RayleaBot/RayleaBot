package fingerprint

// MurmurHash3 x64 128-bit implementation, ported from the JS reference
// in yuki-plugin-main3/models/bilibili/bilibili.risk.buid.fp.js
func murmurX64Add(m, n [2]uint32) [2]uint32 {
	m0, m1 := m[0]>>16, m[0]&0xffff
	m2, m3 := m[1]>>16, m[1]&0xffff
	n0, n1 := n[0]>>16, n[0]&0xffff
	n2, n3 := n[1]>>16, n[1]&0xffff
	o3 := m3 + n3
	o2 := m2 + n2 + (o3 >> 16)
	o1 := m1 + n1 + (o2 >> 16)
	o0 := m0 + n0 + (o1 >> 16)
	return [2]uint32{
		((o0 & 0xffff) << 16) | (o1 & 0xffff),
		((o2 & 0xffff) << 16) | (o3 & 0xffff),
	}
}

func murmurX64Multiply(m, n [2]uint32) [2]uint32 {
	m0, m1 := m[0]>>16, m[0]&0xffff
	m2, m3 := m[1]>>16, m[1]&0xffff
	n0, n1 := n[0]>>16, n[0]&0xffff
	n2, n3 := n[1]>>16, n[1]&0xffff
	o3 := m3 * n3
	o2 := m2*n3 + (o3 >> 16)
	o1 := m2*n2 + (o2 >> 16)
	o2 += m3 * n2
	o1 += o2 >> 16
	o1 += m1*n3 + m3*n1
	o0 := m0*n3 + m1*n2 + m2*n1 + m3*n0 + (o1 >> 16)
	return [2]uint32{
		((o0 & 0xffff) << 16) | (o1 & 0xffff),
		((o2 & 0xffff) << 16) | (o3 & 0xffff),
	}
}

func murmurX64Rotl(m [2]uint32, n int) [2]uint32 {
	n %= 64
	if n == 32 {
		return [2]uint32{m[1], m[0]}
	}
	if n < 32 {
		return [2]uint32{
			(m[0] << n) | (m[1] >> (32 - n)),
			(m[1] << n) | (m[0] >> (32 - n)),
		}
	}
	n -= 32
	return [2]uint32{
		(m[1] << n) | (m[0] >> (32 - n)),
		(m[0] << n) | (m[1] >> (32 - n)),
	}
}

func murmurX64LeftShift(m [2]uint32, n int) [2]uint32 {
	n %= 64
	if n == 0 {
		return m
	}
	if n < 32 {
		return [2]uint32{
			(m[0] << n) | (m[1] >> (32 - n)),
			m[1] << n,
		}
	}
	return [2]uint32{m[1] << (n - 32), 0}
}

func murmurX64Xor(m, n [2]uint32) [2]uint32 {
	return [2]uint32{m[0] ^ n[0], m[1] ^ n[1]}
}

func murmurX64Fmix(h [2]uint32) [2]uint32 {
	h = murmurX64Xor(h, [2]uint32{0, h[0] >> 1})
	h = murmurX64Multiply(h, [2]uint32{0xff51afd7, 0xed558ccd})
	h = murmurX64Xor(h, [2]uint32{0, h[0] >> 1})
	h = murmurX64Multiply(h, [2]uint32{0xc4ceb9fe, 0x1a85ec53})
	h = murmurX64Xor(h, [2]uint32{0, h[0] >> 1})
	return h
}
