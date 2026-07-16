// Package httpapi translates local HTTP requests into application use cases.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// Dependencies are the application ports exposed through local transports.
type Dependencies struct {
	System                  systemQuery
	Host                    hostQuery
	Operations              operationService
	Sessions                sessionService
	Catalog                 catalogService
	Runtime                 runtimeService
	Health                  healthService
	LogService              logService
	Ports                   portService
	Git                     gitService
	Actions                 actionService
	AI                      aiOnboardingService
	Resources               resourceService
	Plugins                 pluginService
	Diagnostics             diagnosticService
	Automations             automationService
	Workspaces              workspaceService
	Environments            environmentService
	EnvironmentRegistration environmentRegistrationService
	Routes                  routeService
	Terminals               terminalService
	Fleet                   fleetService
	Team                    teamService
	Telemetry               telemetryService
	Events                  http.Handler
	Logs                    http.Handler
	Terminal                http.Handler
	Web                     http.Handler
	Logger                  *slog.Logger
}

// NewBrowser constructs the authenticated loopback router and embedded UI.
func NewBrowser(dependencies Dependencies) http.Handler {
	return newRouter(dependencies, accessBrowser, true)
}

// NewIPC constructs the privileged local IPC router.
func NewIPC(dependencies Dependencies) http.Handler {
	return newRouter(dependencies, accessIPC, false)
}

func newRouter(dependencies Dependencies, access accessKind, serveWeb bool) http.Handler {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler { return withCorrelation(dependencies.Logger, next) })
	router.Use(func(next http.Handler) http.Handler { return withAccess(access, next) })
	if access == accessBrowser {
		router.Use(withBrowserHeaders)
		router.Use(func(next http.Handler) http.Handler { return withBrowserSecurity(dependencies.Sessions, next) })
	}
	router.Use(withIdempotencyKey)

	api := chi.NewRouter()
	generated.HandlerFromMux(&handler{
		system: dependencies.System, host: dependencies.Host, operations: dependencies.Operations, sessions: dependencies.Sessions, catalog: dependencies.Catalog,
		runtime: dependencies.Runtime, health: dependencies.Health, logs: dependencies.LogService,
		ports: dependencies.Ports, git: dependencies.Git, actions: dependencies.Actions, ai: dependencies.AI, resources: dependencies.Resources, plugins: dependencies.Plugins,
		diagnostics: dependencies.Diagnostics, automations: dependencies.Automations,
		workspaces:   dependencies.Workspaces,
		environments: dependencies.Environments, environmentRegistration: dependencies.EnvironmentRegistration, routes: dependencies.Routes,
		terminals: dependencies.Terminals, fleet: dependencies.Fleet, team: dependencies.Team, telemetry: dependencies.Telemetry,
	}, api)
	router.Mount("/api/v1", api)
	if dependencies.Events != nil {
		router.Handle("/ws/v1/events", dependencies.Events)
	}
	if dependencies.Logs != nil {
		router.Handle("/ws/v1/logs", dependencies.Logs)
	}
	if serveWeb && dependencies.Terminal != nil {
		router.Handle("/ws/v1/terminal/{sessionId}", dependencies.Terminal)
		router.Handle("/ws/v1/agent-sessions/{sessionId}", dependencies.Terminal)
	}
	if serveWeb && dependencies.Web != nil {
		router.Handle("/*", dependencies.Web)
		router.Handle("/", dependencies.Web)
	}
	return router
}
