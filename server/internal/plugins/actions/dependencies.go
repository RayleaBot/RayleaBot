package actions

import (
	"context"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
	pluginfile "github.com/RayleaBot/RayleaBot/server/internal/plugins/filestore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
)

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
	StorageRootAllowed(context.Context, string, string) bool
	HTTPHosts(context.Context, string) []string
	WebhookParameters(context.Context, string, string) (plugins.WebhookScope, bool)
	ListPluginSnapshots() []plugins.Snapshot
}

type WebhookGateway interface {
	Expose(context.Context, string, runtimeaction.Action) (map[string]any, error)
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

type ConfigChangeDispatcher func(context.Context, string) ConfigChangeDispatchResult

type ScheduledTask struct {
	JobID   string
	NextRun time.Time
}

type SchedulerCreateFunc func(context.Context, string, string, string, string, []byte) (ScheduledTask, error)

type SecretReader interface {
	ReadPluginSecret(context.Context, string) (string, bool, error)
}

type Renderer interface {
	ResolvePluginTemplate(context.Context, string, string) (string, error)
	RenderImage(context.Context, RenderImageRequest) (RenderImageResult, error)
	TemplateAcceptsRenderIdentity(context.Context, string) bool
}

type RenderImageRequest struct {
	Template string
	Theme    string
	Output   string
	Data     map[string]any
	Plugin   RenderPluginContext
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

type GovernanceService interface{}

type HTTPCredentialInjector = httpaction.CredentialInjector

type KVRepository interface {
	Get(context.Context, string, string) (any, bool, error)
	Set(context.Context, string, string, any, pluginkv.Limits) error
	Delete(context.Context, string, string) (bool, error)
	List(context.Context, string, string) ([]string, error)
}

type FileStore interface {
	Read(string, string) (pluginfile.ReadResult, error)
	Write(string, string, []byte, pluginfile.Limits) error
	Delete(string, string) (bool, error)
	List(string, string) ([]string, error)
}
