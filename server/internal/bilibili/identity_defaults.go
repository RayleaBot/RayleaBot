package bilibili

type uaEntry struct {
	UA          string
	ChromeMajor int
	Platform    string
}

var defaultUAPool = []uaEntry{
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
		ChromeMajor: 134,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
		ChromeMajor: 130,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
		ChromeMajor: 126,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		ChromeMajor: 124,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		ChromeMajor: 120,
		Platform:    "Windows",
	},
	{
		UA:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
		ChromeMajor: 134,
		Platform:    "macOS",
	},
	{
		UA:          "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
		ChromeMajor: 130,
		Platform:    "macOS",
	},
}

var defaultLanguages = []string{
	"zh-CN,zh;q=0.9,en;q=0.8",
	"zh-CN,zh;q=0.9,en-US;q=0.5,en;q=0.3",
	"zh-CN,en;q=0.7,en-US;q=0.5",
	"zh-CN,zh-Hans;q=0.9,en;q=0.8,en-GB;q=0.6",
}
