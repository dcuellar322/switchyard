package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"

	catalogApplication "switchyard.dev/switchyard/internal/catalog/application"
	environmentsApplication "switchyard.dev/switchyard/internal/environments/application"
	fleetAdapters "switchyard.dev/switchyard/internal/fleet/adapters"
	fleetApplication "switchyard.dev/switchyard/internal/fleet/application"
	fleetDomain "switchyard.dev/switchyard/internal/fleet/domain"
	observabilityAdapters "switchyard.dev/switchyard/internal/observability/adapters"
	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/platform/sqlite"
	pluginsApplication "switchyard.dev/switchyard/internal/plugins/application"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
	teamAdapters "switchyard.dev/switchyard/internal/team/adapters"
	teamApplication "switchyard.dev/switchyard/internal/team/application"
	telemetryAdapters "switchyard.dev/switchyard/internal/telemetry/adapters"
	telemetryApplication "switchyard.dev/switchyard/internal/telemetry/application"
)

type sharedConfiguration struct {
	team      *teamApplication.Service
	telemetry *telemetryApplication.Service
}

type configuredExtensions struct {
	shared sharedConfiguration
	plugin *pluginsApplication.Service
}

func newConfiguredExtensions(
	ctx context.Context,
	database *sqlite.Database,
	catalogService *catalogApplication.Service,
	redactor *observabilityAdapters.Redactor,
	dataDir, version string,
	logger *slog.Logger,
) (configuredExtensions, error) {
	shared, err := newSharedConfiguration(database, version)
	if err != nil {
		return configuredExtensions{}, err
	}
	pluginService, err := newPluginService(ctx, database, catalogService, redactor, dataDir, version, logger)
	if err != nil {
		return configuredExtensions{}, err
	}
	return configuredExtensions{shared: shared, plugin: pluginService}, nil
}

func newSharedConfiguration(database *sqlite.Database, version string) (sharedConfiguration, error) {
	teamService, err := teamApplication.NewService(sqlite.NewTeamRepository(database), teamAdapters.ManifestValidator{})
	if err != nil {
		return sharedConfiguration{}, err
	}
	telemetryService, err := telemetryApplication.NewService(
		sqlite.NewTelemetryRepository(database), telemetryAdapters.NewHTTPSender(),
		telemetryApplication.Build{Version: version}, teamService,
	)
	if err != nil {
		return sharedConfiguration{}, err
	}
	return sharedConfiguration{team: teamService, telemetry: telemetryService}, nil
}

func newFleetExpansion(
	config Config,
	database *sqlite.Database,
	teamService *teamApplication.Service,
	catalogService *catalogApplication.Service,
	runtimeService *runtimeApplication.Service,
	environmentService *environmentsApplication.Service,
	coordinator *operationsApplication.Coordinator,
	version string,
) (*fleetApplication.Service, http.Handler, error) {
	fleetService, err := fleetApplication.NewService(sqlite.NewFleetRepository(database), fleetAdapters.NewHTTPSPeerClient(), teamService)
	if err != nil {
		return nil, nil, err
	}
	if config.RemoteAddr == "" {
		return fleetService, nil, nil
	}
	controllers, err := parseRemoteControllers(config.RemoteControllers)
	if err != nil {
		return nil, nil, err
	}
	agentService, err := fleetApplication.NewAgentService(fleetDomain.Identity{
		ProtocolVersion: fleetDomain.ProtocolVersion, MachineID: config.RemoteMachineID, Name: config.RemoteMachineName,
		Version: version, OS: runtime.GOOS, Architecture: runtime.GOARCH,
		Capabilities: append([]fleetDomain.Capability(nil), fleetDomain.KnownCapabilities...),
	}, fleetAdapters.NewLocalInventory(catalogService, runtimeService, environmentService), fleetAdapters.NewLocalOperator(coordinator), controllers, teamService)
	if err != nil {
		return nil, nil, err
	}
	return fleetService, fleetAdapters.NewAgentHandler(agentService), nil
}
