package bilibili

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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

// BrowserFingerprint holds all browser properties used for buvid_fp generation.
type BrowserFingerprint struct {
	UserAgent                 string
	Webdriver                 bool
	Language                  string
	ColorDepth                int
	DeviceMemory              int
	PixelRatio                int
	HardwareConcurrency       int
	ScreenResolution          [2]int
	AvailableScreenResolution [2]int
	TimezoneOffset            int
	Timezone                  string
	SessionStorage            bool
	LocalStorage              bool
	IndexedDB                 bool
	AddBehavior               bool
	OpenDatabase              bool
	CPUClass                  string
	Platform                  string
	DoNotTrack                string
	Plugins                   []string
	Canvas                    string
	WebGL                     string
	WebGLVendorAndRenderer    string
	AdBlock                   bool
	HasLiedLanguages          bool
	HasLiedResolution         bool
	HasLiedOS                 bool
	HasLiedBrowser            bool
	TouchSupport              int
	Fonts                     []string
	Audio                     string
	EnumerateDevices          []string
}

func defaultFingerprint(ua string) BrowserFingerprint {
	return BrowserFingerprint{
		UserAgent:                 ua,
		Webdriver:                 false,
		Language:                  "zh-CN",
		ColorDepth:                24,
		DeviceMemory:              8,
		PixelRatio:                1,
		HardwareConcurrency:       8,
		ScreenResolution:          [2]int{1920, 1080},
		AvailableScreenResolution: [2]int{1920, 1040},
		TimezoneOffset:            -480,
		Timezone:                  "Asia/Shanghai",
		SessionStorage:            true,
		LocalStorage:              true,
		IndexedDB:                 true,
		AddBehavior:               false,
		OpenDatabase:              true,
		CPUClass:                  "",
		Platform:                  "Win32",
		DoNotTrack:                "1",
		Plugins: []string{
			"Chrome PDF Plugin", "Chrome PDF Viewer", "Native Client",
		},
		Canvas:                 "canvas winding:yes~canvas",
		WebGL:                  "webgl fingerprint",
		WebGLVendorAndRenderer: "Google Inc. (Intel)~ANGLE (Intel, Intel(R) UHD Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)",
		AdBlock:                false,
		HasLiedLanguages:       false,
		HasLiedResolution:      false,
		HasLiedOS:              false,
		HasLiedBrowser:         false,
		TouchSupport:           0,
		Fonts: []string{
			"Arial", "ArialBlack", "ArialNarrow", "BookAntiqua", "BookmanOldStyle",
			"Calibri", "Cambria", "CambriaMath", "Century", "CenturyGothic",
			"ComicSansMS", "Consolas", "Courier", "CourierNew", "Garamond",
			"Georgia", "Helvetica", "Impact", "LucidaConsole", "LucidaSansUnicode",
			"MicrosoftSansSerif", "PalatinoLinotype", "SegoePrint", "SegoeScript",
			"SegoeUI", "Symbol", "Tahoma", "Times", "TimesNewRoman", "TrebuchetMS",
			"Verdana", "Wingdings",
		},
		Audio:            "audio fingerprint data",
		EnumerateDevices: []string{},
	}
}

func genBuvidFP(browserData BrowserFingerprint) string {
	boolInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	components := []string{
		browserData.UserAgent,
		strconv.FormatBool(browserData.Webdriver),
		browserData.Language,
		strconv.Itoa(browserData.ColorDepth),
		strconv.Itoa(browserData.DeviceMemory),
		strconv.Itoa(browserData.PixelRatio),
		strconv.Itoa(browserData.HardwareConcurrency),
		fmt.Sprintf("%d,%d", browserData.ScreenResolution[0], browserData.ScreenResolution[1]),
		fmt.Sprintf("%d,%d", browserData.AvailableScreenResolution[0], browserData.AvailableScreenResolution[1]),
		strconv.Itoa(browserData.TimezoneOffset),
		browserData.Timezone,
		strconv.Itoa(boolInt(browserData.SessionStorage)),
		strconv.Itoa(boolInt(browserData.LocalStorage)),
		strconv.Itoa(boolInt(browserData.IndexedDB)),
		strconv.Itoa(boolInt(browserData.AddBehavior)),
		strconv.Itoa(boolInt(browserData.OpenDatabase)),
		browserData.CPUClass,
		browserData.Platform,
		browserData.DoNotTrack,
		strings.Join(browserData.Plugins, ","),
		browserData.Canvas,
		browserData.WebGL,
		browserData.WebGLVendorAndRenderer,
		strconv.Itoa(boolInt(browserData.AdBlock)),
		strconv.Itoa(boolInt(browserData.HasLiedLanguages)),
		strconv.Itoa(boolInt(browserData.HasLiedResolution)),
		strconv.Itoa(boolInt(browserData.HasLiedOS)),
		strconv.Itoa(boolInt(browserData.HasLiedBrowser)),
		strconv.Itoa(browserData.TouchSupport),
		strings.Join(browserData.Fonts, ","),
		strings.Join(browserData.Fonts, ","),
		browserData.Audio,
		strings.Join(browserData.EnumerateDevices, ","),
	}
	values := strings.Join(components, "~~~")
	return murmurHash128(values, 31)
}

func GenBuvidFP(ua string) string {
	return genBuvidFP(defaultFingerprint(ua))
}

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
