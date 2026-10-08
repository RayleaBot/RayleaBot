package qqofficial

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

func TestUndeliverableSegmentsRejectTheWholeMessage(t *testing.T) {
	for _, segment := range []chatevent.MessageSegment{
		{Type: "face", Data: map[string]any{"face_id": "1"}},
		{Type: "at_all"},
		{Type: "reply", Data: map[string]any{"message_id": "other"}},
		{Type: "image", Data: map[string]any{"file": "base64://%%%"}},
	} {
		t.Run(segment.Type, func(t *testing.T) {
			client, captured := newTestClient(t, okResponse)
			_, err := client.SendReply(t.Context(), chatevent.OutboundMessageReply{
				TargetType: "group", TargetID: "target", ReplyToMessageID: "inbound",
				Segments: []chatevent.MessageSegment{
					{Type: "text", Data: map[string]any{"text": "must not arrive"}},
					{Type: "image", Data: map[string]any{"url": "https://example.invalid/ok.png"}},
					segment,
				},
			})
			if chatevent.SendErrorCode(err) != CodeCapabilityUnsupported || len(*captured) != 0 {
				t.Fatalf("requests=%+v, err=%v", *captured, err)
			}
		})
	}
}

func TestUploadFailureLeavesNoVisibleMessage(t *testing.T) {
	uploads, sends := 0, 0
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages") {
			sends++
		}
		if strings.HasSuffix(r.URL.Path, "/files") {
			uploads++
			if uploads == 2 {
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"code":304082}`))
				return
			}
		}
		okResponse(w, r)
	})
	_, err := client.SendMessage(t.Context(), chatevent.OutboundMessageSend{TargetType: "group", TargetID: "target", Segments: []chatevent.MessageSegment{
		{Type: "text", Data: map[string]any{"text": "must not arrive"}},
		{Type: "image", Data: map[string]any{"url": "https://example.invalid/first.png"}},
		{Type: "image", Data: map[string]any{"url": "https://example.invalid/second.png"}},
	}})
	if err == nil || sends != 0 || uploads != 2 {
		t.Fatalf("uploads=%d, sends=%d, err=%v", uploads, sends, err)
	}
}

func TestBase64ImagesAndFilesUploadDecodedBytes(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		fileType int
	}{{"image", mediaTypeImage}, {"file", mediaTypeFile}} {
		t.Run(tc.kind, func(t *testing.T) {
			client, uploads, sends := newMediaClient(t)
			encoded := base64.StdEncoding.EncodeToString([]byte("fixture binary\x00\xff"))
			_, err := client.SendMessage(t.Context(), chatevent.OutboundMessageSend{TargetType: "group", TargetID: "target", Segments: []chatevent.MessageSegment{
				{Type: tc.kind, Data: map[string]any{"file": "base64://" + encoded, "name": "fixture.bin"}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			if len(*uploads) != 1 || len(*sends) != 1 {
				t.Fatalf("uploads=%d sends=%d", len(*uploads), len(*sends))
			}
			upload := (*uploads)[0].body
			if upload.FileType != tc.fileType || upload.FileData != encoded || upload.URL != "" || upload.SrvSendMsg {
				t.Fatalf("upload=%+v", upload)
			}
			if tc.kind == "file" && upload.FileName != "fixture.bin" {
				t.Fatal("file name lost")
			}
		})
	}
}

func TestBase64MediaEnforcesDecodedSizeLimit(t *testing.T) {
	encoded := "base64://" + base64.StdEncoding.EncodeToString(make([]byte, maxMediaBytes+1))
	for _, kind := range []string{"image", "file"} {
		client, captured := newTestClient(t, okResponse)
		_, err := client.SendMessage(t.Context(), chatevent.OutboundMessageSend{TargetType: "group", TargetID: "target", Segments: []chatevent.MessageSegment{{Type: kind, Data: map[string]any{"file": encoded}}}})
		if chatevent.SendErrorCode(err) != CodeCapabilityUnsupported || len(*captured) != 0 {
			t.Fatalf("kind=%s, requests=%d, err=%v", kind, len(*captured), err)
		}
	}
}

func TestAtAndReplySegmentsBecomeOnePassiveTextMessage(t *testing.T) {
	client, captured := newTestClient(t, okResponse)
	_, err := client.SendReply(t.Context(), chatevent.OutboundMessageReply{TargetType: "group", TargetID: "target", ReplyToMessageID: "inbound", Segments: []chatevent.MessageSegment{
		{Type: "reply", Data: map[string]any{"message_id": "inbound"}},
		{Type: "text", Data: map[string]any{"text": "hello "}},
		{Type: "at", Data: map[string]any{"user_id": "member"}},
		{Type: "text", Data: map[string]any{"text": "!"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(*captured) != 1 || (*captured)[0].body.Content != `hello <qqbot-at-user id="member" />!` || (*captured)[0].body.MsgID != "inbound" {
		t.Fatalf("requests=%+v", *captured)
	}
}
