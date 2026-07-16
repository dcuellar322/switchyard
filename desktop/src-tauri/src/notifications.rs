use std::collections::{HashMap, HashSet};

use crate::model::DesktopSnapshot;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Notice {
    pub title: String,
    pub body: String,
}

#[derive(Default)]
pub struct Tracker {
    initialized: bool,
    operations: HashSet<String>,
    health: HashMap<String, String>,
    active_warnings: HashSet<String>,
    diagnostics: HashMap<String, usize>,
}

impl Tracker {
    pub fn observe(&mut self, snapshot: &DesktopSnapshot) -> Vec<Notice> {
        let operation_ids = snapshot
            .operations
            .iter()
            .map(|operation| operation.id.clone())
            .collect::<HashSet<_>>();
        let health = snapshot
            .projects
            .iter()
            .filter_map(|project| {
                project
                    .health
                    .as_ref()
                    .map(|value| (project.project.id.clone(), value.status.clone()))
            })
            .collect::<HashMap<_, _>>();
        let warnings = warning_keys(snapshot);
        let diagnostics = snapshot
            .diagnostic_notifications
            .iter()
            .map(|notification| (notification.id.clone(), notification.occurrences))
            .collect::<HashMap<_, _>>();

        if !self.initialized {
            self.initialized = true;
            self.operations = operation_ids;
            self.health = health;
            self.active_warnings = warnings.keys().cloned().collect();
            self.diagnostics = diagnostics;
            return Vec::new();
        }

        let mut notices = Vec::new();
        for operation in &snapshot.operations {
            if !self.operations.contains(&operation.id)
                && matches!(operation.state.as_str(), "failed" | "partially_succeeded")
            {
                notices.push(Notice {
                    title: "Switchyard operation needs attention".into(),
                    body: format!(
                        "{} finished with status {}",
                        operation.kind, operation.state
                    ),
                });
            }
        }
        for project in &snapshot.projects {
            let Some(current) = project.health.as_ref().map(|value| value.status.as_str()) else {
                continue;
            };
            let previous = self.health.get(&project.project.id).map(String::as_str);
            if previous != Some(current) && current == "unhealthy" {
                notices.push(Notice {
                    title: format!("{} is unhealthy", project.project.display_name),
                    body: "Open Switchyard to review the failing health checks.".into(),
                });
            } else if previous == Some("unhealthy") && current == "healthy" {
                notices.push(Notice {
                    title: format!("{} recovered", project.project.display_name),
                    body: "All required health checks are healthy again.".into(),
                });
            }
        }
        for (key, notice) in &warnings {
            if !self.active_warnings.contains(key) {
                notices.push(notice.clone());
            }
        }
        for notification in &snapshot.diagnostic_notifications {
            if self
                .diagnostics
                .get(&notification.id)
                .copied()
                .unwrap_or_default()
                < notification.occurrences
            {
                notices.push(Notice {
                    title: notification.title.clone(),
                    body: notification.detail.clone(),
                });
            }
        }

        self.operations.extend(operation_ids);
        self.health = health;
        self.active_warnings = warnings.keys().cloned().collect();
        self.diagnostics = diagnostics;
        notices
    }
}

fn warning_keys(snapshot: &DesktopSnapshot) -> HashMap<String, Notice> {
    let mut warnings = HashMap::new();
    if snapshot.port_conflict_count > 0 {
        warnings.insert(
            "port-conflicts".into(),
            Notice {
                title: "Port conflicts detected".into(),
                body: format!(
                    "{} port conflict(s) need attention.",
                    snapshot.port_conflict_count
                ),
            },
        );
    }
    if let Some(host) = &snapshot.host {
        if host.cpu_percent >= 90.0 {
            warnings.insert(
                "host-cpu".into(),
                Notice {
                    title: "Host CPU usage is high".into(),
                    body: format!("Current CPU usage is {:.0}%.", host.cpu_percent),
                },
            );
        }
        if host.memory_total_bytes > 0
            && host.memory_used_bytes as f64 / host.memory_total_bytes as f64 >= 0.9
        {
            warnings.insert(
                "host-memory".into(),
                Notice {
                    title: "Host memory usage is high".into(),
                    body: "More than 90% of host memory is in use.".into(),
                },
            );
        }
        if !host.docker.connected {
            warnings.insert(
                "docker-disconnected".into(),
                Notice {
                    title: "Container engine disconnected".into(),
                    body: "Switchyard cannot observe or control project runtimes.".into(),
                },
            );
        }
    }
    warnings
}

#[cfg(test)]
mod tests {
    use crate::model::{
        DesktopSnapshot, DiagnosticNotification, DockerObservation, HostObservation, Operation,
        Project, ProjectHealth, ProjectSnapshot, RuntimeObservation, SystemInfo,
    };

    use super::*;

    fn snapshot() -> DesktopSnapshot {
        DesktopSnapshot {
            system: SystemInfo {
                api_version: "switchyard.api/v1".into(),
                database_schema_version: 13,
                status: "ready".into(),
                version: env!("CARGO_PKG_VERSION").into(),
            },
            host: Some(HostObservation {
                cpu_percent: 20.0,
                docker: DockerObservation { connected: true },
                memory_total_bytes: 100,
                memory_used_bytes: 20,
            }),
            projects: vec![ProjectSnapshot {
                project: Project {
                    display_name: "Example".into(),
                    id: "project".into(),
                },
                runtime: Some(RuntimeObservation {
                    state: "running".into(),
                }),
                health: Some(ProjectHealth {
                    status: "healthy".into(),
                }),
            }],
            workspaces: Vec::new(),
            operations: Vec::new(),
            diagnostic_notifications: Vec::new(),
            port_conflict_count: 0,
        }
    }

    #[test]
    fn suppresses_historical_state_and_reports_transitions_once() {
        let mut tracker = Tracker::default();
        let mut current = snapshot();
        assert!(tracker.observe(&current).is_empty());

        current.projects[0].health.as_mut().unwrap().status = "unhealthy".into();
        current.operations.push(Operation {
            id: "operation".into(),
            kind: "runtime.start".into(),
            state: "failed".into(),
        });
        current.port_conflict_count = 2;
        current
            .diagnostic_notifications
            .push(DiagnosticNotification {
                id: "diagnostic".into(),
                title: "API is repeatedly crashing".into(),
                detail: "Four restarts were observed.".into(),
                occurrences: 1,
            });
        assert_eq!(tracker.observe(&current).len(), 4);
        assert!(tracker.observe(&current).is_empty());

        current.diagnostic_notifications[0].occurrences = 2;
        assert_eq!(tracker.observe(&current).len(), 1);

        current.projects[0].health.as_mut().unwrap().status = "healthy".into();
        current.port_conflict_count = 0;
        assert_eq!(tracker.observe(&current).len(), 1);
    }
}
