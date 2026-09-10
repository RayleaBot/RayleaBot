package actions

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
)

type PermissionView interface {
	PermissionDeclared(context.Context, string, string) bool
	PermissionPlatforms(context.Context, string, string) []string
	ListPluginSnapshots() []plugins.Snapshot
}

type PluginConfigRepository interface {
	Read(context.Context, string, []string) (map[string]any, error)
	ReadAll(context.Context, string) (map[string]any, error)
	Write(context.Context, string, map[string]any) ([]string, error)
}

type OneBotAdapter interface {
	CallAPIAny(context.Context, string, map[string]any) (any, error)
	DetectedProvider() string
}

type ConfigChangeDispatchResult struct {
	Delivered bool
	Outcome   string
	ErrorCode string
}

type ConfigChangeDispatcher func(context.Context, string, map[string]any, []string) ConfigChangeDispatchResult

type MessageSendFunc func(context.Context, string, string, chatevent.Event, chatevent.MessageCommand) (map[string]any, error)

type ScheduledTask struct {
	JobID   string
	NextRun time.Time
}

type SchedulerCreateFunc func(context.Context, string, string, string, string, []byte) (ScheduledTask, error)

type SecretReader interface {
	ReadPluginSecret(context.Context, string) (string, bool, error)
}

type ThirdPartyAccountReader interface {
	ListEnabled(context.Context, string) ([]thirdparty.Account, error)
	ReadCookie(context.Context, thirdparty.Account) (string, error)
}

type ThirdPartyAccountValidationRequester interface {
	RequestPluginValidation(context.Context, string, string, string, string, int) (bool, string, error)
}

// ThirdPartyResolver 用平台侧的登录环境（浏览器会话与设备信誉）把昵称
// 关键词解析为候选用户列表。resolve 是插件纯 HTTP 搜索被风控拦截后的
// 回退路径，只有宿主持有登录浏览器资源的平台（douyin）提供实现。
type ThirdPartyResolver interface {
	ResolveUser(context.Context, string, []map[string]string) ([]thirdparty.AccountProfile, bool, error)
}

type Renderer interface {
	ResolvePluginTemplate(context.Context, string, string) (string, error)
	RenderImage(context.Context, RenderImageRequest) (RenderImageResult, error)
	TemplateAcceptsRenderIdentity(context.Context, string) bool
}

type RenderImageRequest struct {
	Template  string
	Theme     string
	Output    string
	Data      map[string]any
	Resources []RenderImageResource
	Plugin    RenderPluginContext
}

type RenderImageResource struct {
	ID     string
	Path   string
	MIME   string
	SHA256 string
	Size   int64
}

type RenderPluginContext struct {
	Name    string
	Version string
}

type RenderImageResult struct {
	ArtifactID string
	ImagePath  string
	MIME       string
	CacheKey   string
}

type RenderTemplateError struct {
	Code    string
	Message string
	Err     error
}

func (e *RenderTemplateError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *RenderTemplateError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type KVRepository interface {
	Get(context.Context, string, string) (any, bool, error)
	Set(context.Context, string, string, any, pluginstore.KVLimits) error
	Delete(context.Context, string, string) (bool, error)
	List(context.Context, string, string) ([]string, error)
}

type FileStore interface {
	Read(string, string) (pluginstore.FileReadResult, error)
	WriteWithResult(string, string, []byte, pluginstore.FileLimits) (pluginstore.FileWriteResult, error)
	Delete(string, string) (bool, error)
	List(string, string) ([]string, error)
}
