package dynamic

const (
	FeedURL      = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?timezone_offset=-480&type=all&page=1"
	SpaceFeedURL = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?host_mid=%s&timezone_offset=-480&features=itemOpusStyle"
	RelationURL  = "https://api.bilibili.com/x/relation?fid=%s"
	FollowURL    = "https://api.bilibili.com/x/relation/modify"
)

type FeedDocument struct {
	Code int `json:"code"`
	Data struct {
		Items []map[string]any `json:"items"`
		Cards []map[string]any `json:"cards"`
	} `json:"data"`
}

type RelationDocument struct {
	Code int `json:"code"`
	Data struct {
		Attribute int `json:"attribute"`
	} `json:"data"`
}

type MonitorCandidate struct {
	Event  BilibiliEvent
	Pinned bool
	Index  int
}

type dynamicContentData struct {
	Title       string
	Summary     string
	SummaryHTML string
	URL         string
	Images      []Image
	Topic       *BilibiliTopic
	Original    *BilibiliOriginal
}
