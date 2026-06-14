package live

const (
	StatusBatchURL = "https://api.live.bilibili.com/room/v1/Room/get_status_info_by_uids"
	DanmuInfoURL   = "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?id=%s&type=0"
)

type Image struct {
	URL    string
	Width  int
	Height int
}

type StatusDocument struct {
	Code int                   `json:"code"`
	Msg  string                `json:"message"`
	Data map[string]StatusItem `json:"data"`
}

type StatusItem struct {
	UID            any    `json:"uid"`
	UName          string `json:"uname"`
	Face           string `json:"face"`
	RoomID         any    `json:"room_id"`
	Title          string `json:"title"`
	LiveStatus     int    `json:"live_status"`
	LiveTime       any    `json:"live_time"`
	LiveTimeCompat any    `json:"liveTime"`
	URL            string `json:"url"`
	CoverFromUser  string `json:"cover_from_user"`
	UserCover      string `json:"user_cover"`
}

type DanmuInfoDocument struct {
	Code int `json:"code"`
	Data struct {
		Token    string `json:"token"`
		HostList []struct {
			Host    string `json:"host"`
			WSSPort int    `json:"wss_port"`
			WSPort  int    `json:"ws_port"`
		} `json:"host_list"`
	} `json:"data"`
}
