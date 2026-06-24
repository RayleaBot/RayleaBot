package menu

import (
	"context"
	"log/slog"
	"strings"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/command"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

const (
	builtinMenuTemplateID = "help.menu"
	builtinMenuFallback   = "菜单生成失败，请稍后重试。"
)

type Sender interface {
	SendMessage(context.Context, adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error)
	SendReply(context.Context, adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error)
}

type Deps struct {
	CurrentConfig func() config.Config
	Plugins       plugins.CatalogView
	Renderer      *renderservice.Service
	Sender        Sender
	WaitOutbound  func(context.Context, outbound.MessageLimitRequest) error
	Logger        *slog.Logger
}

type Service struct {
	currentConfig func() config.Config
	plugins       plugins.CatalogView
	renderer      *renderservice.Service
	sender        Sender
	waitOutbound  func(context.Context, outbound.MessageLimitRequest) error
	logger        *slog.Logger
}

type Request struct {
	Matched bool
	Target  string
	Prefix  string
	Command string
}

type builtinMenuRenderData struct {
	Data   map[string]any
	Plugin *renderservice.PluginContext
}

func New(deps Deps) *Service {
	return &Service{
		currentConfig: deps.CurrentConfig,
		plugins:       deps.Plugins,
		renderer:      deps.Renderer,
		sender:        deps.Sender,
		waitOutbound:  deps.WaitOutbound,
		logger:        deps.Logger,
	}
}

func (s *Service) Handle(ctx context.Context, event adapterintake.NormalizedEvent) bool {
	request := s.Match(event)
	if !request.Matched {
		return false
	}
	if s.sender == nil {
		return true
	}

	payload := s.buildBuiltinMenuData(event, request.Target)
	if len(payload.Data) == 0 {
		return true
	}
	s.logBuiltinMenuTrigger(ctx, event, request)

	result, err := s.renderBuiltinMenu(ctx, payload)
	if err != nil || strings.TrimSpace(result.ImagePath) == "" {
		s.logBuiltinMenuError(err)
		s.sendBuiltinMenuText(ctx, event, request.Command, builtinMenuFallback)
		return true
	}

	s.sendBuiltinMenuImage(ctx, event, request.Command, result.ImagePath)
	return true
}

func (s *Service) Match(event adapterintake.NormalizedEvent) Request {
	if s == nil || strings.TrimSpace(event.PlainText) == "" {
		return Request{}
	}
	cfg := s.config()
	prefixes := builtinMenuPrefixes(cfg)
	commands := builtinMenuCommands(cfg)
	parsed := command.NewParser(prefixes).Parse(event.PlainText)
	if !parsed.IsCommand {
		return Request{}
	}

	commandName := strings.TrimSpace(parsed.Command)
	for _, name := range commands {
		if commandName == name {
			return Request{
				Matched: true,
				Target:  strings.TrimSpace(strings.Join(parsed.Args, " ")),
				Prefix:  parsed.Prefix,
				Command: commandName,
			}
		}
		if strings.HasSuffix(commandName, name) {
			target := strings.TrimSpace(strings.TrimSuffix(commandName, name))
			if target != "" {
				if s.hasExactPluginCommand(commandName) {
					continue
				}
				return Request{
					Matched: true,
					Target:  target,
					Prefix:  parsed.Prefix,
					Command: commandName,
				}
			}
		}
	}
	return Request{}
}

func (s *Service) hasExactPluginCommand(commandName string) bool {
	commandName = strings.TrimSpace(commandName)
	if commandName == "" || s == nil || s.plugins == nil {
		return false
	}
	for _, snapshot := range s.plugins.List() {
		if !pluginParticipatesInCommandPolicy(snapshot) {
			continue
		}
		for _, commandItem := range snapshot.Commands {
			if commandMatches(commandItem, commandName) {
				return true
			}
		}
	}
	return false
}

func (s *Service) config() config.Config {
	if s == nil || s.currentConfig == nil {
		return config.Config{}
	}
	return s.currentConfig()
}

func pluginParticipatesInCommandPolicy(snapshot plugins.Snapshot) bool {
	return snapshot.Valid &&
		snapshot.RegistrationState == "installed" &&
		snapshot.DesiredState == "enabled"
}

func commandMatches(command plugins.Command, commandName string) bool {
	if strings.TrimSpace(command.Name) == commandName {
		return true
	}
	for _, alias := range command.Aliases {
		if strings.TrimSpace(alias) == commandName {
			return true
		}
	}
	return false
}
