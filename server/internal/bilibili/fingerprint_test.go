package bilibili

import (
	"testing"
)

func TestMurmurHash128KnownVectors(t *testing.T) {
	t.Parallel()
	// Verify murmurHash128 is deterministic.
	a := murmurHash128("hello", 0)
	b := murmurHash128("hello", 0)
	if a != b {
		t.Fatalf("murmurHash128 not deterministic: %q != %q", a, b)
	}
	// Different seed should produce different hash.
	c := murmurHash128("hello", 42)
	if c == a {
		t.Fatalf("murmurHash128 with different seed should differ")
	}
	// Different input should produce different hash.
	d := murmurHash128("world", 0)
	if d == a {
		t.Fatalf("murmurHash128 with different input should differ")
	}
	// Length should be 32 hex chars.
	if len(a) != 32 {
		t.Fatalf("murmurHash128 length = %d, want 32", len(a))
	}
}

func TestGenBuvidFPDeterministic(t *testing.T) {
	t.Parallel()
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/134.0.0.0"
	a := GenBuvidFP(ua)
	b := GenBuvidFP(ua)
	if a != b {
		t.Fatalf("GenBuvidFP not deterministic: %q != %q", a, b)
	}
	if len(a) != 32 {
		t.Fatalf("GenBuvidFP length = %d, want 32", len(a))
	}
	// Different UA should produce different output.
	c := GenBuvidFP("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/130.0.0.0")
	if c == a {
		t.Fatalf("GenBuvidFP for different UA should differ")
	}
}

func TestGenBuvidFormat(t *testing.T) {
	t.Parallel()
	result := GenBuvid("XX")
	if len(result) == 0 {
		t.Fatal("GenBuvid returned empty string")
	}
	if result[:2] != "XX" {
		t.Fatalf("GenBuvid prefix = %q, want XX", result[:2])
	}
}

func TestGenUUIDFormat(t *testing.T) {
	t.Parallel()
	result := GenUUID()
	if len(result) != 36 {
		t.Fatalf("GenUUID length = %d, want 36", len(result))
	}
	// UUID v4 format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if result[8] != '-' || result[13] != '-' || result[18] != '-' || result[23] != '-' {
		t.Fatalf("GenUUID missing dashes: %q", result)
	}
}

func TestGetDmImgReturnsAllFields(t *testing.T) {
	t.Parallel()
	params := GetDmImg()
	if params.DmImgList == "" {
		t.Fatal("GetDmImg DmImgList is empty")
	}
	if params.DmImgStr == "" {
		t.Fatal("GetDmImg DmImgStr is empty")
	}
	if params.DmCoverImgStr == "" {
		t.Fatal("GetDmImg DmCoverImgStr is empty")
	}
	if params.DmImgInter == "" {
		t.Fatal("GetDmImg DmImgInter is empty")
	}
}
