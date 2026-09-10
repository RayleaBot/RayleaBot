package netease_music

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

// Offline vectors use synthetic input. AES results were produced independently
// with Node crypto (OpenSSL AES-128-CBC); RSA with modular BigInt exponentiation.
func TestWEAPICryptoOfflineVector(t *testing.T) {
	const payload = `{"csrf_token":"offline-csrf","type":1}`
	const secret = "0123456789abcdef"
	first, err := aesCBCBase64([]byte(payload), []byte(neteaseNonce), []byte(neteaseIV))
	if err != nil || first != "sYZUD9XoSZgHU/3jHtpkLrZArr0rX/kINGXWcelQJKg/7W5qB0QzHrJxflUOu3dK" {
		t.Fatalf("first AES layer: %q, %v", first, err)
	}
	second, err := aesCBCBase64([]byte(first), []byte(secret), []byte(neteaseIV))
	if err != nil || second != "VB4ggcEujMcmqnXSk7PAR9Oj7PctLVULhqXlv/01VNuuK+tEcP2mLRwndXvxGHzSqPSG8EpQCZf+R0rRwbBlkI7DCshHTCvWMakUtrRh+54=" {
		t.Fatalf("second AES layer: %q, %v", second, err)
	}
	const encryptedSecret = "35701388baf89fed412e11269b9c76625d095ecaf17f03fa018abe19ea2d38b949debf242ee39a71ca1f6cda71b1b86a45aa909ee27f7e78e267d34e732f0de948206c3340a788d0003372183e2f753c1f78b66ac23d134ac1fc9b993156520ea826b8aa89a962d4491b4b8d7e08738e1da9b07aa39bf4a7ef0b1c210728cd52"
	if got := neteaseEncSecKey(secret); got != encryptedSecret {
		t.Fatalf("RSA encrypted secret: %q", got)
	}
}

func TestWEAPIFormUsesFreshKeys(t *testing.T) {
	first, err := neteaseWEAPIForm(`{"type":1}`)
	if err != nil {
		t.Fatal(err)
	}
	second, err := neteaseWEAPIForm(`{"type":1}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"params", "encSecKey"} {
		if first.Get(key) == "" || first.Get(key) == second.Get(key) {
			t.Fatalf("requests reused %s", key)
		}
	}
	if data, err := base64.StdEncoding.DecodeString(first.Get("params")); err != nil || len(data)%16 != 0 {
		t.Fatalf("AES payload: len=%d, %v", len(data), err)
	}
	if key := first.Get("encSecKey"); len(key) != 256 || strings.Trim(key, "0123456789abcdef") != "" {
		t.Fatalf("RSA encoding: %q", key)
	}
}

func TestPKCS7PaddingBoundaries(t *testing.T) {
	for _, size := range []int{0, 15, 16, 17} {
		input := bytes.Repeat([]byte{'x'}, size)
		padded, err := pkcs7Pad(input, 16)
		padding := 16 - size%16
		if err != nil || !bytes.Equal(padded[:size], input) || !bytes.Equal(padded[size:], bytes.Repeat([]byte{byte(padding)}, padding)) {
			t.Fatalf("size=%d: %x, %v", size, padded, err)
		}
		if len(input) != 0 {
			padded[0] = 'z'
			if input[0] != 'x' {
				t.Fatal("padding mutated caller input")
			}
		}
	}
	for _, size := range []int{-1, 0, 256} {
		if _, err := pkcs7Pad(nil, size); err == nil {
			t.Fatalf("accepted invalid block size %d", size)
		}
	}
}
