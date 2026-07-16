package domain

import "time"

// StorageClassification describes how confidently bytes belong to one project.
type StorageClassification string

const (
	// StorageExclusive is uniquely attributable to one project.
	StorageExclusive StorageClassification = "exclusive"
	// StorageShared is used by multiple projects or contains shared layers.
	StorageShared StorageClassification = "shared"
	// StorageEstimated is attributable but not an exact exclusive byte count.
	StorageEstimated StorageClassification = "estimated"
	// StorageUnknown lacks enough ownership or size evidence.
	StorageUnknown StorageClassification = "unknown"
)

// ResourceBudget is an advisory project-level sustained usage threshold.
type ResourceBudget struct {
	CPUPercent   float64 `json:"cpuPercent"`
	MemoryBytes  uint64  `json:"memoryBytes"`
	StorageBytes int64   `json:"storageBytes"`
}

// ProjectDescriptor is the non-secret catalog input needed for resource policy.
type ProjectDescriptor struct {
	ID                 string
	Name               string
	Driver             string
	ComposeProjectName string
	Budget             ResourceBudget
}

// MetricPoint is one persisted aggregate for a project or one service.
type MetricPoint struct {
	Timestamp             time.Time             `json:"timestamp"`
	ProjectID             string                `json:"projectId"`
	ServiceID             string                `json:"serviceId"`
	ResolutionSeconds     int                   `json:"resolutionSeconds"`
	SampleCount           int                   `json:"sampleCount"`
	CPUPercent            float64               `json:"cpuPercent"`
	CPUMaxPercent         float64               `json:"cpuMaxPercent"`
	CPUAvailable          bool                  `json:"cpuAvailable"`
	MemoryBytes           uint64                `json:"memoryBytes"`
	MemoryMaxBytes        uint64                `json:"memoryMaxBytes"`
	MemoryLimit           uint64                `json:"memoryLimit"`
	MemoryAvailable       bool                  `json:"memoryAvailable"`
	NetworkRxBytes        uint64                `json:"networkRxBytes"`
	NetworkTxBytes        uint64                `json:"networkTxBytes"`
	NetworkAvailable      bool                  `json:"networkAvailable"`
	DiskReadBytes         uint64                `json:"diskReadBytes"`
	DiskWriteBytes        uint64                `json:"diskWriteBytes"`
	DiskAvailable         bool                  `json:"diskAvailable"`
	ProcessCount          int                   `json:"processCount"`
	RestartCount          int                   `json:"restartCount"`
	HealthLatencyMS       int64                 `json:"healthLatencyMs"`
	HealthAvailable       bool                  `json:"healthAvailable"`
	StorageBytes          *int64                `json:"storageBytes,omitempty"`
	StorageClassification StorageClassification `json:"storageClassification"`
	Partial               bool                  `json:"partial"`
}

// RuntimeSnapshot is the provider-neutral output of one active runtime sample.
type RuntimeSnapshot struct {
	ProjectID string
	Driver    string
	State     string
	Active    bool
	Partial   bool
	Samples   []MetricPoint
	Warnings  []string
}

// BudgetWarning records a sustained threshold rather than a single spike.
type BudgetWarning struct {
	Code          string    `json:"code"`
	Resource      string    `json:"resource"`
	Limit         float64   `json:"limit"`
	Observed      float64   `json:"observed"`
	Unit          string    `json:"unit"`
	Samples       int       `json:"samples"`
	SustainedFrom time.Time `json:"sustainedFrom"`
	Message       string    `json:"message"`
}

// ServiceSnapshot exposes the latest service aggregate.
type ServiceSnapshot struct {
	ServiceID string      `json:"serviceId"`
	Metric    MetricPoint `json:"metric"`
}

// ProjectSnapshot exposes one latest aggregate and its service attribution.
type ProjectSnapshot struct {
	ProjectID string            `json:"projectId"`
	Name      string            `json:"name"`
	Driver    string            `json:"driver"`
	State     string            `json:"state"`
	Active    bool              `json:"active"`
	Metric    MetricPoint       `json:"metric"`
	Services  []ServiceSnapshot `json:"services"`
	Budget    ResourceBudget    `json:"budget"`
	Warnings  []BudgetWarning   `json:"warnings"`
}

// MetricHistory is a bounded chronological time series at one stored tier.
type MetricHistory struct {
	ProjectID         string        `json:"projectId"`
	ServiceID         string        `json:"serviceId,omitempty"`
	ResolutionSeconds int           `json:"resolutionSeconds"`
	From              time.Time     `json:"from"`
	To                time.Time     `json:"to"`
	Points            []MetricPoint `json:"points"`
}

// StorageResource is one Docker resource with explicit attribution evidence.
type StorageResource struct {
	Kind           string                `json:"kind"`
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	ProjectIDs     []string              `json:"projectIds"`
	Bytes          *int64                `json:"bytes,omitempty"`
	Reclaimable    bool                  `json:"reclaimable"`
	Classification StorageClassification `json:"classification"`
	Reason         string                `json:"reason"`
}

// StorageSummary is a total whose attribution remains explicit.
type StorageSummary struct {
	Bytes            int64                 `json:"bytes"`
	ReclaimableBytes int64                 `json:"reclaimableBytes"`
	Classification   StorageClassification `json:"classification"`
	ResourceCount    int                   `json:"resourceCount"`
}

// ProjectStorage is one project's intentionally conservative storage estimate.
type ProjectStorage struct {
	ProjectID       string         `json:"projectId"`
	Summary         StorageSummary `json:"summary"`
	UnknownSizes    int            `json:"unknownSizes"`
	SharedResources int            `json:"sharedResources"`
}

// StorageInventory is a read-only Docker disk-usage observation.
type StorageInventory struct {
	Connected  bool              `json:"connected"`
	ObservedAt time.Time         `json:"observedAt"`
	Summary    StorageSummary    `json:"summary"`
	Projects   []ProjectStorage  `json:"projects"`
	Resources  []StorageResource `json:"resources"`
	Warnings   []string          `json:"warnings"`
}

// CleanupPreview lists exact candidates but grants no deletion capability.
type CleanupPreview struct {
	ProjectID      string            `json:"projectId"`
	Risk           string            `json:"risk"`
	Executable     bool              `json:"executable"`
	EstimatedBytes int64             `json:"estimatedBytes"`
	UnknownSizes   int               `json:"unknownSizes"`
	Resources      []StorageResource `json:"resources"`
	Warnings       []string          `json:"warnings"`
	ObservedAt     time.Time         `json:"observedAt"`
}

// Footprint reports Switchyard-owned data without walking project roots.
type Footprint struct {
	DatabaseBytes    int64      `json:"databaseBytes"`
	DatabaseWALBytes int64      `json:"databaseWalBytes"`
	DatabaseSHMBytes int64      `json:"databaseShmBytes"`
	LogBytes         int64      `json:"logBytes"`
	LogSegments      int        `json:"logSegments"`
	MetricRows       int64      `json:"metricRows"`
	OldestMetricAt   *time.Time `json:"oldestMetricAt,omitempty"`
	Classification   string     `json:"classification"`
}

// RetentionPolicy documents the active metric tiers and self-storage cap.
type RetentionPolicy struct {
	SampleIntervalSeconds int   `json:"sampleIntervalSeconds"`
	RawSeconds            int64 `json:"rawSeconds"`
	MinuteSeconds         int64 `json:"minuteSeconds"`
	QuarterHourSeconds    int64 `json:"quarterHourSeconds"`
	MaximumHistoryPoints  int   `json:"maximumHistoryPoints"`
	LogSeconds            int64 `json:"logSeconds"`
	LogBytes              int64 `json:"logBytes"`
}

// ResourceOverview is the aggregate read model used by the dashboard.
type ResourceOverview struct {
	ObservedAt time.Time         `json:"observedAt"`
	Projects   []ProjectSnapshot `json:"projects"`
	Storage    StorageSummary    `json:"storage"`
	Footprint  Footprint         `json:"footprint"`
	Retention  RetentionPolicy   `json:"retention"`
	Warnings   []string          `json:"warnings"`
}
