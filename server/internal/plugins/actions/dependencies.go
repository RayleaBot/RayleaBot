package actions

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/browser"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
)

type PermissionView interface {
	PermissionDeclared(context.Context, string, string) bool
	ListPluginSnapshots() []plugins.Snapshot
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

// BrowserSessionManager starts and stops host-managed browser sessions for
// plugins. The host owns process launch, profile isolation, and shutdown; the
// plugin interacts with the returned CDP endpoint directly.
type BrowserSessionManager interface {
	Launch(context.Context, string, browser.LaunchRequest) (browser.Session, error)
	Close(pluginID, sessionID string) bool
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
