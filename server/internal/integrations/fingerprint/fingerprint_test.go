package fingerprint

import "testing"

func TestMurmurHash128KnownVectors(t *testing.T) {
	t.Parallel()

	a := MurmurHash128("hello", 0)
	b := MurmurHash128("hello", 0)
	if a != b {
		t.Fatalf("MurmurHash128 not deterministic: %q != %q", a, b)
	}
	c := MurmurHash128("hello", 42)
	if c == a {
		t.Fatalf("MurmurHash128 with different seed should differ")
	}
	d := MurmurHash128("world", 0)
	if d == a {
		t.Fatalf("MurmurHash128 with different input should differ")
	}
	if len(a) != 32 {
		t.Fatalf("MurmurHash128 length = %d, want 32", len(a))
	}
}

func TestGenBuvidFPDeterministic(t *testing.T) {
	t.Parallel()

	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/134.0.0.0"
	a := GenBuvidFP(DefaultFingerprint(ua))
	b := GenBuvidFP(DefaultFingerprint(ua))
	if a != b {
		t.Fatalf("GenBuvidFP not deterministic: %q != %q", a, b)
	}
	if len(a) != 32 {
		t.Fatalf("GenBuvidFP length = %d, want 32", len(a))
	}
	c := GenBuvidFP(DefaultFingerprint("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/130.0.0.0"))
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
