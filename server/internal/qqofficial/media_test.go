package qqofficial

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

type capturedUpload struct {
	path string
	body uploadMediaRequest
}

func newMediaClient(t *testing.T) (*Client, *[]capturedUpload, *[]sendMessageRequest) {
	t.Helper()
	uploads := &[]capturedUpload{}
	sends := &[]sendMessageRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/files") {
			var body uploadMediaRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			*uploads = append(*uploads, capturedUpload{path: r.URL.Path, body: body})
			_ = json.NewEncoder(w).Encode(map[string]any{"file_info": "file-info-1", "ttl": 600})
			return
		}
		var body sendMessageRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		*sends = append(*sends, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "sent-1"})
	}))
	t.Cleanup(server.Close)

	tokens := NewTokenSource("app", "secret", server.Client())
	tokens.token = "canned-token"
	tokens.expiresAt = tokens.now().Add(3600 * 1e9)
	client := &Client{
		appID: "app", apiBase: server.URL, http: server.Client(),
		tokens: tokens, replies: newReplySequences(), logger: discardLogger(),
	}
	return client, uploads, sends
}

func TestSendUploadsLocalMediaAsBase64(t *testing.T) {
	t.Parallel()

	// The render service hands plugins a file:// URL for a locally rendered
	// image, so this is the path a self-hosted bot actually takes. It must not
	// need a publicly reachable address.
	dir := t.TempDir()
	path := filepath.Join(dir, "card.png")
	payload := []byte("fake-png-bytes")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write fixture image: %v", err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()

	client, uploads, sends := newMediaClient(t)
	if _, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
		Segments: []chatevent.MessageSegment{{Type: "image", Data: map[string]any{"file": fileURL}}},
	}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if len(*uploads) != 1 {
		t.Fatalf("performed %d uploads, want 1", len(*uploads))
	}
	upload := (*uploads)[0]
	if upload.path != "/v2/groups/G1/files" {
		t.Fatalf("upload endpoint = %q, want the group media endpoint", upload.path)
	}
	if upload.body.FileType != mediaTypeImage {
		t.Fatalf("file_type = %d, want the image kind", upload.body.FileType)
	}
	if upload.body.URL != "" {
		t.Fatalf("local media was sent as a url: %q", upload.body.URL)
	}
	decoded, err := base64.StdEncoding.DecodeString(upload.body.FileData)
	if err != nil || string(decoded) != string(payload) {
		t.Fatalf("file_data did not round-trip the file bytes: %v", err)
	}
	// Uploading must not send the message by itself; that would spend the
	// active push quota before the send call.
	if upload.body.SrvSendMsg {
		t.Fatal("upload asked the platform to send the message directly")
	}

	if len(*sends) != 1 || (*sends)[0].MsgType != msgTypeMedia || (*sends)[0].Media == nil {
		t.Fatalf("sends = %+v, want one media message", *sends)
	}
	if (*sends)[0].Media.FileInfo != "file-info-1" {
		t.Fatalf("media file_info = %q, want the uploaded handle", (*sends)[0].Media.FileInfo)
	}
}

func TestSendHandsRemoteMediaToThePlatform(t *testing.T) {
	t.Parallel()

	client, uploads, _ := newMediaClient(t)
	if _, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "private", TargetID: "U1",
		Segments: []chatevent.MessageSegment{{Type: "image", Data: map[string]any{"url": "https://example.invalid/a.png"}}},
	}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	upload := (*uploads)[0]
	// A reachable address is cheaper to hand over than to fetch and re-encode.
	if upload.body.URL != "https://example.invalid/a.png" || upload.body.FileData != "" {
		t.Fatalf("remote media upload = %+v, want the url passed through", upload.body)
	}
	if upload.path != "/v2/users/U1/files" {
		t.Fatalf("upload endpoint = %q, want the single-chat media endpoint", upload.path)
	}
}

func TestSendSplitsTextAndMediaIntoSeparateMessages(t *testing.T) {
	t.Parallel()

	client, _, sends := newMediaClient(t)
	if _, err := client.SendReply(context.Background(), chatevent.OutboundMessageReply{
		TargetType: "group", TargetID: "G1", ReplyToMessageID: "ROBOT1.0_abc",
		Segments: []chatevent.MessageSegment{
			{Type: "text", Data: map[string]any{"text": "看图"}},
			{Type: "image", Data: map[string]any{"url": "https://example.invalid/a.png"}},
		},
	}); err != nil {
		t.Fatalf("SendReply: %v", err)
	}

	if len(*sends) != 2 {
		t.Fatalf("sent %d messages, want text and media separately", len(*sends))
	}
	if (*sends)[0].Content != "看图" || (*sends)[1].MsgType != msgTypeMedia {
		t.Fatalf("sends = %+v, want text then media", *sends)
	}
	// Both answer the same inbound message, so their sequences must differ or
	// the platform rejects the second as a duplicate.
	if (*sends)[0].MsgSeq == (*sends)[1].MsgSeq {
		t.Fatalf("both parts reused msg_seq %d", (*sends)[0].MsgSeq)
	}
}

func TestUploadRejectsMediaItCannotRead(t *testing.T) {
	t.Parallel()

	client, _, _ := newMediaClient(t)
	var sendErr *SendError
	_, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
		Segments: []chatevent.MessageSegment{{Type: "image", Data: map[string]any{"file": "file:///no/such/image.png"}}},
	})
	if !errors.As(err, &sendErr) || sendErr.Code != CodeCapabilityUnsupported {
		t.Fatalf("missing file = %v, want %s", err, CodeCapabilityUnsupported)
	}
}

func TestMediaFileTypeCoversThePlatformKinds(t *testing.T) {
	t.Parallel()

	for segment, want := range map[string]int{"image": mediaTypeImage, "video": mediaTypeVideo, "record": mediaTypeVoice} {
		if got, ok := mediaFileType(segment); !ok || got != want {
			t.Fatalf("mediaFileType(%q) = %d/%v, want %d", segment, got, ok, want)
		}
	}
	if _, ok := mediaFileType("poke"); ok {
		t.Fatal("poke was mapped to a media kind the platform does not have")
	}
}
