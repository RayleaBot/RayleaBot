package qqofficial

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

// Media kinds the upload endpoint accepts.
const (
	mediaTypeImage = 1
	mediaTypeVideo = 2
	mediaTypeVoice = 3
	mediaTypeFile  = 4
)

// maxMediaBytes bounds local and decoded base64 media before upload encoding.
const maxMediaBytes = 32 << 20

type uploadMediaRequest struct {
	FileType   int    `json:"file_type"`
	URL        string `json:"url,omitempty"`
	FileData   string `json:"file_data,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	SrvSendMsg bool   `json:"srv_send_msg"`
}

type uploadMediaResponse struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int    `json:"ttl"`
	Message  string `json:"message"`
	Code     int    `json:"code"`
}

// mediaFileType maps a neutral segment kind onto the platform's media kinds.
func mediaFileType(segmentType string) (int, bool) {
	switch segmentType {
	case "image":
		return mediaTypeImage, true
	case "video":
		return mediaTypeVideo, true
	case "record":
		return mediaTypeVoice, true
	case "file":
		return mediaTypeFile, true
	default:
		return 0, false
	}
}

func prepareMedia(segment chatevent.MessageSegment) (uploadMediaRequest, error) {
	fileType, ok := mediaFileType(segment.Type)
	if !ok {
		return uploadMediaRequest{}, unsupportedSegment(segment.Type)
	}

	body := uploadMediaRequest{FileType: fileType}
	if segment.Type == "file" {
		body.FileName, _ = segment.Data["name"].(string)
	}
	source := mediaSource(segment)
	switch {
	case source == "":
		return body, &chatevent.SendError{
			Code:    CodeCapabilityUnsupported,
			Message: fmt.Sprintf("%q 消息段没有可用的地址或文件。", segment.Type),
		}
	case isRemoteMediaSource(source):
		body.URL = source
	case strings.HasPrefix(source, "base64://"):
		data, err := io.ReadAll(io.LimitReader(base64.NewDecoder(base64.StdEncoding, strings.NewReader(strings.TrimPrefix(source, "base64://"))), maxMediaBytes+1))
		if err != nil || len(data) > maxMediaBytes {
			return body, &chatevent.SendError{Code: CodeCapabilityUnsupported, Message: "媒体 base64 无效或超过大小上限。"}
		}
		body.FileData = base64.StdEncoding.EncodeToString(data)
	default:
		data, err := readLocalMedia(source)
		if err != nil {
			return body, err
		}
		body.FileData = base64.StdEncoding.EncodeToString(data)
	}
	return body, nil
}

// uploadMedia obtains a handle without making the media visible to recipients.
func (c *Client) uploadMedia(ctx context.Context, settings requestSettings, targetType, targetID string, body uploadMediaRequest) (string, error) {
	endpoint, err := mediaEndpoint(settings.apiBase, targetType, targetID)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	token, err := settings.tokens.Token(ctx)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", AuthorizationHeader(token))
	request.Header.Set("X-Union-Appid", settings.appID)
	request.Header.Set("Content-Type", "application/json")

	response, err := settings.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("qqofficial: upload media: %w", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)

	var decoded uploadMediaResponse
	decodeErr := json.NewDecoder(response.Body).Decode(&decoded)
	if response.StatusCode < 200 || response.StatusCode >= 300 || decodeErr != nil || decoded.FileInfo == "" {
		message := decoded.Message
		if message == "" {
			message = "平台拒绝了媒体上传。"
		}
		return "", &chatevent.SendError{
			Code:    sendErrorCode(response.StatusCode, decoded.Code, false),
			Message: message,
		}
	}
	return decoded.FileInfo, nil
}

// mediaEndpoint mirrors the message endpoints: media uploaded for a group can
// only be sent to a group, and likewise for a single chat.
func mediaEndpoint(base, targetType, targetID string) (string, error) {
	id := strings.TrimSpace(targetID)
	if id == "" {
		return "", &chatevent.SendError{Code: CodeCapabilityUnsupported, Message: "媒体目标缺少标识。"}
	}
	switch strings.TrimSpace(targetType) {
	case "group":
		return base + "/v2/groups/" + id + "/files", nil
	case "private":
		return base + "/v2/users/" + id + "/files", nil
	default:
		return "", &chatevent.SendError{
			Code:    CodeCapabilityUnsupported,
			Message: fmt.Sprintf("当前适配器无法向会话种类 %q 上传媒体。", targetType),
		}
	}
}

func mediaSource(segment chatevent.MessageSegment) string {
	for _, key := range []string{"url", "file", "path"} {
		if value, ok := segment.Data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isRemoteMediaSource(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}

// readLocalMedia accepts both a file:// URL, which is what the render service
// produces, and a plain filesystem path.
func readLocalMedia(source string) ([]byte, error) {
	path := source
	if strings.HasPrefix(source, "file://") {
		parsed, err := url.Parse(source)
		if err != nil {
			return nil, &chatevent.SendError{Code: CodeCapabilityUnsupported, Message: "媒体地址无法解析。"}
		}
		path = parsed.Path
		// A Windows path arrives as /C:/... after URL parsing.
		if len(path) > 2 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
	}
	info, err := os.Stat(filepath.FromSlash(path))
	if err != nil {
		return nil, &chatevent.SendError{Code: CodeCapabilityUnsupported, Message: "找不到要发送的媒体文件。"}
	}
	if info.Size() > maxMediaBytes {
		return nil, &chatevent.SendError{
			Code:    CodeCapabilityUnsupported,
			Message: fmt.Sprintf("媒体文件超过 %d MiB 上限。", maxMediaBytes>>20),
		}
	}
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return nil, &chatevent.SendError{Code: CodeCapabilityUnsupported, Message: "无法读取要发送的媒体文件。"}
	}
	return data, nil
}
