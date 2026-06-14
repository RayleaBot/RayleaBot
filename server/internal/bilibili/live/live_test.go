package live

import (
	"encoding/json"
	"net/http"
	"testing"
)

type testIdentity struct{}

func (testIdentity) UserAgent() string { return "raylea-test-agent" }

func TestVerifyPayloadUsesCookieIdentity(t *testing.T) {
	payload := VerifyPayload("12345", "token-value", "DedeUserID=42; buvid3=buvid-value")

	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if doc["key"] != "token-value" {
		t.Fatalf("key = %v", doc["key"])
	}
	if doc["buvid"] != "buvid-value" {
		t.Fatalf("buvid = %v", doc["buvid"])
	}
	if doc["protover"] != float64(WSProtoBrotli) {
		t.Fatalf("protover = %v", doc["protover"])
	}
	if doc["roomid"] != float64(12345) {
		t.Fatalf("roomid = %v", doc["roomid"])
	}
	if doc["uid"] != float64(42) {
		t.Fatalf("uid = %v", doc["uid"])
	}
}

func TestPackUnpackNotice(t *testing.T) {
	packet := Pack([]byte(`{"cmd":"LIVE"}`), WSProtoRaw, WSOpNotice)

	events, err := Unpack(packet)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events length = %d", len(events))
	}
	if events[0]["cmd"] != "LIVE" {
		t.Fatalf("cmd = %v", events[0]["cmd"])
	}
}

func TestStatusValues(t *testing.T) {
	item := StatusItem{
		CoverFromUser: "//i0.hdslb.com/live-cover.jpg",
		UserCover:     "https://ignored.example/cover.jpg",
		LiveTime:      "2026-06-14 10:11:12",
	}

	if got := NormalizeStatus(2); got != 0 {
		t.Fatalf("NormalizeStatus(2) = %d", got)
	}
	if got := FirstImageURL(item); got != "https://i0.hdslb.com/live-cover.jpg" {
		t.Fatalf("FirstImageURL = %q", got)
	}
	if images := Images(item); len(images) != 1 || images[0].URL != "https://i0.hdslb.com/live-cover.jpg" {
		t.Fatalf("Images = %#v", images)
	}
	if got := TimeFromItem(item); got <= 0 {
		t.Fatalf("TimeFromItem = %d", got)
	}
	if got := ParseInt(" 123 "); got != 123 {
		t.Fatalf("ParseInt = %d", got)
	}
}

func TestHeaders(t *testing.T) {
	headers := Headers(testIdentity{}, "SESSDATA=value")

	if got := headers.Get("User-Agent"); got != "raylea-test-agent" {
		t.Fatalf("User-Agent = %q", got)
	}
	if got := headers.Get("Cookie"); got != "SESSDATA=value" {
		t.Fatalf("Cookie = %q", got)
	}
	if got := headers.Get("Origin"); got != "https://live.bilibili.com" {
		t.Fatalf("Origin = %q", got)
	}
	if headers.Values("Accept-Language") == nil || headers.Get("Accept-Language") == "" {
		t.Fatalf("Accept-Language missing: %#v", http.Header(headers))
	}
}
