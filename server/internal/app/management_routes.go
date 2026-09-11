package app

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementapi "github.com/RayleaBot/RayleaBot/server/internal/management"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

type httpHandlers struct {
	Auth       *managementapi.AuthHandlers
	Management *managementapi.CoreHandlers
	EventsWS   *managementapi.EventsHandler
}

type managementRouteState struct {
	RouterDeps  managementapi.RouteDeps
	RequireAuth func(http.Handler) http.Handler
	Handlers    httpHandlers
}

type managementUIModule interface {
	managementapi.PublicRouteModule
	managementapi.ProtectedRouteModule
}

func buildManagementRoutes(deps httpBuildDeps, configService managementapi.ConfigService, pluginManagementUI managementUIModule) (managementRouteState, error) {
	runtimeState := deps.Runtime
	platformState := deps.Platform
	pluginState := deps.Plugins
	eventState := deps.Events
	services := deps.ServiceBuild.Services

	authConfig := authConfigSource{source: runtimeState}
	authHandler := managementapi.NewAuthHandlers(managementapi.AuthDeps{
		Config:        authConfig,
		Auth:          platformState.Auth,
		LoginFailures: platformState.LoginFailures,
		SetupToken:    managementapi.NewOneTimeToken(deps.SetupToken),
	})
	managementHandler := managementapi.NewCoreHandlers(managementapi.CoreDeps{
		Auth:                 platformState.Auth,
		System:               services.System,
		RequestShutdown:      deps.RequestShutdown,
		LauncherControlToken: managementapi.NewStaticToken(deps.LauncherControlToken),
	})
	governanceHandler := managementapi.NewGovernanceHandlersWithService(services.Governance)
	logHandler := managementapi.NewLogHandlers(services.Logs)
	renderHandler := managementapi.NewRenderHandlers(deps.Renderer, pluginNameResolver(pluginState.Plugins.Get, true))
	systemHandlers := managementapi.NewSystemHandlers(services.System)
	if platformState.Scheduler != nil {
		schedulerView, err := scheduler.NewView(platformState.Scheduler, pluginNameResolver(pluginState.Plugins.Get, false))
		if err != nil {
			return managementRouteState{}, err
		}
		systemHandlers = managementapi.NewSystemHandlers(services.System, schedulerView)
	}
	systemRoutes := managementapi.NewSystemRoutes(systemHandlers, deps.Metrics.HTTPHandler())
	protocolHandler := managementapi.NewProtocolHandlers(services.Protocol)
	thirdPartyOptions := []managementapi.ThirdPartyHandlersOption{
		managementapi.WithThirdPartyAccountValidation(services.AccountValidation),
		managementapi.WithThirdPartyAvatarTransport(deps.HTTPTransport),
	}
	var thirdPartyHandler *managementapi.ThirdPartyHandlers
	if services.ThirdPartyQRLogin == nil {
		thirdPartyHandler = managementapi.NewThirdPartyHandlers(
			services.ThirdParty,
			deps.ServiceBuild.ThirdPartyAccountValidator,
			nil,
			thirdPartyOptions...,
		)
	} else {
		thirdPartyHandler = managementapi.NewThirdPartyHandlers(
			services.ThirdParty,
			deps.ServiceBuild.ThirdPartyAccountValidator,
			services.ThirdPartyQRLogin,
			thirdPartyOptions...,
		)
	}
	updateHandler, err := managementapi.NewUpdateHandlers(releaseupdate.NewEmbeddedService(runtimeState.RepoRoot()))
	if err != nil {
		return managementRouteState{}, err
	}
	eventsWS, err := managementapi.NewEventsHandler(managementevents.Sources{
		Bridge: eventState.Bridge, Plugins: pluginState.Plugins, Adapters: services.Protocol,
		Status: deps.ServiceBuild.Status, Governance: services.GovernanceEvents, ThirdParty: services.ThirdPartyEvents,
	})
	if err != nil {
		return managementRouteState{}, err
	}
	logsWS := managementapi.NewLogsHandler(services.Logs)
	consoleWS := managementapi.NewConsoleHandler(platformState.Console, pluginState.Plugins)
	configHandler := managementapi.NewConfigHandlers(configService)
	pluginRoutes, err := managementapi.NewPluginRoutes(managementapi.PluginRouteDeps{
		Catalog:     pluginState.Plugins,
		Installer:   pluginState.PluginInstaller,
		Uninstaller: pluginState.PluginUninstaller,
		Lifecycle:   services.PluginLifecycle,
	})
	if err != nil {
		return managementRouteState{}, err
	}
	pluginStoreRoutes := managementapi.PluginStoreRoutes{Service: pluginState.PluginStore}
	developmentRoutes := managementapi.DevelopmentRoutes{
		ArtifactRoot: deps.DevelopmentArtifactRoot,
		Token:        managementapi.NewStaticToken(deps.LauncherControlToken),
		Tasks:        platformState.Tasks,
	}
	if deps.DevelopmentArtifactRoot != "" {
		developmentRoutes.Installer = pluginState.PluginInstaller
	}

	handlers := httpHandlers{
		Auth:       authHandler,
		Management: managementHandler,
		EventsWS:   eventsWS,
	}

	return managementRouteState{
		Handlers:    handlers,
		RequireAuth: managementapi.RequireAuthWithConfig(platformState.Auth, authConfig),
		RouterDeps: managementapi.RouteDeps{
			RepoRoot: runtimeState.RepoRoot(),
			Readiness: func() systemsvc.ReadinessReport {
				return systemHandlers.CurrentReadiness()
			},
			PublicRoutes: []managementapi.PublicRouteModule{
				authHandler,
				managementHandler,
				developmentRoutes,
				protocolHandler,
				services.PluginWebhooks,
				pluginManagementUI,
			},
			ProtectedRoutes: []managementapi.ProtectedRouteModule{
				authHandler,
				managementHandler,
				configHandler,
				protocolHandler,
				governanceHandler,
				logHandler,
				systemRoutes,
				renderHandler,
				thirdPartyHandler,
				updateHandler,
				pluginManagementUI,
				managementapi.ProtectedRouteFunc(func(r chi.Router) {
					r.Get("/ws/events", eventsWS.HandleEventsWebSocket())
					r.Get("/ws/logs", logsWS.HandleLogsWebSocket())
					r.Get("/ws/plugins/{id}/console", consoleWS.HandlePluginConsoleWebSocket())
				}),
				pluginRoutes,
				pluginStoreRoutes,
			},
		},
	}, nil
}

// pluginNameResolver maps a plugin id to its display name; fallbackToID
// controls whether an unknown id is echoed back or rendered empty.
func pluginNameResolver(lookup func(string) (plugins.Snapshot, bool), fallbackToID bool) func(string) string {
	return func(pluginID string) string {
		if snapshot, ok := lookup(pluginID); ok {
			return snapshot.Name
		}
		if fallbackToID {
			return pluginID
		}
		return ""
	}
}

type authConfigSource struct {
	source configSource
}

type configSource interface {
	CurrentConfig() config.Config
}

func (s authConfigSource) AuthConfig() managementapi.AuthConfig {
	if s.source == nil {
		return managementapi.AuthConfig{}
	}
	cfg := s.source.CurrentConfig()
	allowedHosts, allowedOrigins, secureCookie := managementBrowserOrigins(cfg, os.Getenv("RAYLEA_WEB_UI_BASE_URL"))
	return managementapi.AuthConfig{
		SetupLocalOnly:     cfg.Web.SetupLocalOnly,
		LoginFailureLimit:  managementapi.LoginFailureLimit(cfg),
		LoginFailureWindow: managementapi.LoginFailureWindow(cfg),
		AllowedHosts:       allowedHosts,
		AllowedOrigins:     allowedOrigins,
		SecureCookie:       secureCookie,
	}
}

func managementBrowserOrigins(cfg config.Config, developmentUIOrigin string) ([]string, []string, bool) {
	port := strconv.Itoa(cfg.Server.Port)
	directAuthority := net.JoinHostPort(cfg.Server.Host, port)
	hosts := []string{directAuthority}
	origins := []string{"http://" + directAuthority}
	secureCookie := false

	if parsed, err := url.Parse(strings.TrimSpace(cfg.Web.PublicOrigin)); err == nil && parsed.Host != "" {
		hosts = appendUniqueString(hosts, parsed.Host)
		origins = appendUniqueString(origins, strings.TrimRight(parsed.String(), "/"))
		secureCookie = strings.EqualFold(parsed.Scheme, "https")
	}

	if isLoopbackHost(cfg.Server.Host) {
		for _, host := range []string{
			net.JoinHostPort("127.0.0.1", port),
			net.JoinHostPort("localhost", port),
			net.JoinHostPort("::1", port),
		} {
			hosts = appendUniqueString(hosts, host)
			origins = appendUniqueString(origins, "http://"+host)
		}
		if developmentOrigin, developmentHost, ok := localDevelopmentUIOrigin(developmentUIOrigin); ok {
			hosts = appendUniqueString(hosts, developmentHost)
			origins = appendUniqueString(origins, developmentOrigin)
		}
	}

	return hosts, origins, secureCookie
}

func localDevelopmentUIOrigin(raw string) (string, string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Scheme, "http") || parsed.User != nil || parsed.Host == "" ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || !isLoopbackHost(parsed.Hostname()) {
		return "", "", false
	}
	if _, err := strconv.Atoi(parsed.Port()); err != nil {
		return "", "", false
	}
	return "http://" + parsed.Host, parsed.Host, true
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func appendUniqueString(values []string, candidate string) []string {
	for _, value := range values {
		if strings.EqualFold(value, candidate) {
			return values
		}
	}
	return append(values, candidate)
}
