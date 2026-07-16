use serde::Deserialize;

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Envelope<T> {
    pub schema_version: String,
    pub command: String,
    pub data: T,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct BuildInfo {
    pub version: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DesktopSnapshot {
    pub system: SystemInfo,
    pub host: Option<HostObservation>,
    pub projects: Vec<ProjectSnapshot>,
    pub workspaces: Vec<Workspace>,
    pub operations: Vec<Operation>,
    #[serde(default)]
    pub diagnostic_notifications: Vec<DiagnosticNotification>,
    pub port_conflict_count: usize,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SystemInfo {
    pub api_version: String,
    pub database_schema_version: i64,
    pub status: String,
    pub version: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HostObservation {
    pub cpu_percent: f64,
    pub docker: DockerObservation,
    pub memory_total_bytes: i64,
    pub memory_used_bytes: i64,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DockerObservation {
    pub connected: bool,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProjectSnapshot {
    pub project: Project,
    pub runtime: Option<RuntimeObservation>,
    pub health: Option<ProjectHealth>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Project {
    pub display_name: String,
    pub id: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RuntimeObservation {
    pub state: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProjectHealth {
    pub status: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Workspace {
    pub id: String,
    pub name: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Operation {
    pub id: String,
    pub kind: String,
    pub state: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DiagnosticNotification {
    pub id: String,
    pub title: String,
    pub detail: String,
    pub occurrences: usize,
}

#[derive(Debug, Deserialize)]
pub struct UIAddress {
    pub url: String,
}
