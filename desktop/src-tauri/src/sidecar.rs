use std::fmt;

use serde::de::DeserializeOwned;
use tauri::{AppHandle, Runtime};
use tauri_plugin_shell::ShellExt;

use crate::model::{BuildInfo, DesktopSnapshot, Envelope, UIAddress};

const OUTPUT_LIMIT: usize = 1024 * 1024;

#[derive(Clone)]
pub struct Sidecar<R: Runtime> {
    app: AppHandle<R>,
}

#[derive(Debug)]
pub struct SidecarError(String);

impl fmt::Display for SidecarError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.0)
    }
}

impl std::error::Error for SidecarError {}

impl<R: Runtime> Sidecar<R> {
    pub fn new(app: AppHandle<R>) -> Self {
        Self { app }
    }

    pub async fn version(&self) -> Result<BuildInfo, SidecarError> {
        self.read("version", &["version"]).await
    }

    pub async fn snapshot(&self) -> Result<DesktopSnapshot, SidecarError> {
        self.read("desktop.snapshot", &["desktop", "snapshot"])
            .await
    }

    pub async fn ui_url(&self, route: &str) -> Result<String, SidecarError> {
        let envelope: Envelope<UIAddress> = self.run(&["ui", "--path", route]).await?;
        validate_envelope(&envelope, "ui")?;
        validate_ui_url(&envelope.data.url)?;
        Ok(envelope.data.url)
    }

    pub async fn project_action(&self, id: &str, action: &str) -> Result<(), SidecarError> {
        if !matches!(action, "start" | "stop") {
            return Err(SidecarError("unsupported project action".into()));
        }
        validate_identifier(id)?;
        self.mutate("runtime.operation", &[action, id]).await
    }

    pub async fn workspace_action(&self, id: &str, action: &str) -> Result<(), SidecarError> {
        if !matches!(action, "start" | "stop") {
            return Err(SidecarError("unsupported workspace action".into()));
        }
        validate_identifier(id)?;
        self.mutate(&format!("workspace.{action}"), &["workspace", action, id])
            .await
    }

    async fn read<T: DeserializeOwned>(
        &self,
        command: &str,
        args: &[&str],
    ) -> Result<T, SidecarError> {
        let envelope: Envelope<T> = self.run(args).await?;
        validate_envelope(&envelope, command)?;
        Ok(envelope.data)
    }

    async fn mutate(&self, command: &str, args: &[&str]) -> Result<(), SidecarError> {
        let envelope: Envelope<serde_json::Value> = self.run(args).await?;
        validate_envelope(&envelope, command)
    }

    async fn run<T: DeserializeOwned>(&self, args: &[&str]) -> Result<T, SidecarError> {
        let mut complete_args = vec!["--json", "--non-interactive", "--no-color"];
        complete_args.extend_from_slice(args);
        let output = self
            .app
            .shell()
            .sidecar("switchyard")
            .map_err(|error| SidecarError(format!("bundled sidecar unavailable: {error}")))?
            .args(complete_args)
            .output()
            .await
            .map_err(|error| SidecarError(format!("sidecar launch failed: {error}")))?;

        if !output.status.success() {
            return Err(SidecarError(format!(
                "sidecar command failed with status {} (details are available in the local daemon log)",
                output.status.code().unwrap_or(-1)
            )));
        }
        if output.stdout.len() > OUTPUT_LIMIT {
            return Err(SidecarError(
                "sidecar response exceeded the safety limit".into(),
            ));
        }
        serde_json::from_slice(&output.stdout)
            .map_err(|_| SidecarError("sidecar returned an invalid structured response".into()))
    }
}

fn validate_ui_url(value: &str) -> Result<url::Url, SidecarError> {
    let parsed = url::Url::parse(value)
        .map_err(|_| SidecarError("sidecar returned an invalid application URL".into()))?;
    let loopback = parsed
        .host_str()
        .is_some_and(|host| host == "127.0.0.1" || host == "localhost" || host == "::1");
    let bootstrap = parsed
        .query_pairs()
        .find_map(|(key, value)| (key == "bootstrap").then_some(value))
        .is_some_and(|value| !value.is_empty());
    if parsed.scheme() != "http"
        || !loopback
        || parsed.port().is_none()
        || !parsed.username().is_empty()
        || parsed.password().is_some()
        || parsed.fragment().is_some()
        || !bootstrap
    {
        return Err(SidecarError(
            "sidecar refused to return an authenticated loopback URL".into(),
        ));
    }
    Ok(parsed)
}

fn validate_envelope<T>(
    envelope: &Envelope<T>,
    expected_command: &str,
) -> Result<(), SidecarError> {
    if envelope.schema_version != "switchyard.cli/v1" || envelope.command != expected_command {
        return Err(SidecarError(
            "sidecar response is incompatible with this desktop shell".into(),
        ));
    }
    Ok(())
}

fn validate_identifier(value: &str) -> Result<(), SidecarError> {
    let valid = !value.is_empty()
        && value.len() <= 128
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_'));
    valid
        .then_some(())
        .ok_or_else(|| SidecarError("invalid local resource identifier".into()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_unbounded_or_shell_like_identifiers() {
        assert!(validate_identifier("project-01").is_ok());
        assert!(validate_identifier("../project").is_err());
        assert!(validate_identifier("a;open").is_err());
        assert!(validate_identifier(&"a".repeat(129)).is_err());
    }

    #[test]
    fn validates_authenticated_loopback_url_shape() {
        assert!(validate_ui_url("http://127.0.0.1:49152/?bootstrap=one-time").is_ok());
        assert!(validate_ui_url("http://127.0.0.1/?bootstrap=one-time").is_err());
        assert!(validate_ui_url("http://attacker@127.0.0.1:49152/?bootstrap=one-time").is_err());
        assert!(validate_ui_url("http://127.0.0.1:49152/").is_err());
    }
}
