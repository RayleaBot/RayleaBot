package bilibili

const (
	dynamicFeedURL      = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all?timezone_offset=-480&type=all&page=1"
	dynamicSpaceFeedURL = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?host_mid=%s&timezone_offset=-480&features=itemOpusStyle"
	relationURL         = "https://api.bilibili.com/x/relation?fid=%s"
	followURL           = "https://api.bilibili.com/x/relation/modify"
)

type dynamicFeedDocument struct {
	Code int `json:"code"`
	Data struct {
		Items []map[string]any `json:"items"`
		Cards []map[string]any `json:"cards"`
	} `json:"data"`
}
type relationDocument struct {
	Code int `json:"code"`
	Data struct {
		Attribute int `json:"attribute"`
	} `json:"data"`
}
type monitorDynamicCandidate struct {
	event  BilibiliEvent
	pinned bool
	index  int
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
