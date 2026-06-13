package bilibili

import (
	"fmt"
	"strconv"
	"strings"
)

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
