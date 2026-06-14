package fingerprint

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenBuvid generates a local buvid3/buvid4 fallback when the SPI endpoint is unavailable.
// Format: "XX" + 16 hex chars (8 random bytes) + timestamp hex
func GenBuvid(prefix string) string {
	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	ts := time.Now().UTC().UnixMilli()
	return strings.ToUpper(fmt.Sprintf("%s%s%x", prefix, hex.EncodeToString(randomBytes), ts))
}

// DmImgParams holds the dm_img query parameters for the dynamic feed API.
type DmImgParams struct {
	DmImgList     string
	DmImgStr      string
	DmCoverImgStr string
	DmImgInter    string
}

// GenUUID generates a random _uuid hex string for Bilibili device fingerprinting.
func GenUUID() string {
	randomBytes := make([]byte, 16)
	_, _ = rand.Read(randomBytes)
	randomBytes[6] = (randomBytes[6] & 0x0f) | 0x40
	randomBytes[8] = (randomBytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", randomBytes[0:4], randomBytes[4:6], randomBytes[6:8], randomBytes[8:10], randomBytes[10:])
}

// GetDmImg returns dm_img parameters for anti-risk-control on the dynamic feed API.
func GetDmImg() DmImgParams {
	return DmImgParams{
		DmImgList:     "[]",
		DmImgStr:      base64.StdEncoding.EncodeToString([]byte("V2ViR0wgMS4wIChPcGVuR0wgRVMgMi4wIENocm9taXVtKQ")),
		DmCoverImgStr: base64.StdEncoding.EncodeToString([]byte("R29vZ2xlIEluYy4gKEludGVsKUFOR0xFIChJbnRlbCwgSW50ZWwoUikgVUhEIEdyYXBoaWNzIERpcmVjdDNEMTEgdnNfNV8wIHBzXzVfMCwgRDNEMTEp")),
		DmImgInter:    `{"ds":[],"wh":[0,0,0],"of":[0,0,0]}`,
	}
}
