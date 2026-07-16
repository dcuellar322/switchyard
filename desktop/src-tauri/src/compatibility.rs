use semver::Version;

use crate::model::{BuildInfo, SystemInfo};

pub const API_VERSION: &str = "v1";
pub const MIN_DATABASE_SCHEMA_VERSION: i64 = 13;

pub fn verify(sidecar: &BuildInfo, daemon: &SystemInfo) -> Result<(), String> {
    let desktop = Version::parse(env!("CARGO_PKG_VERSION"))
        .map_err(|_| "desktop package has an invalid version".to_owned())?;
    let sidecar_version = parse_product_version(&sidecar.version, "bundled sidecar")?;
    let daemon_version = parse_product_version(&daemon.version, "running daemon")?;

    if desktop != sidecar_version {
        return Err(format!(
            "Bundled sidecar {} is incompatible with desktop {}. Reinstall Switchyard.",
            sidecar.version, desktop
        ));
    }
    if desktop.major != daemon_version.major {
        return Err(format!(
            "Running daemon {} is incompatible with desktop {}. Install a daemon with the same major version.",
            daemon.version, desktop
        ));
    }
    if daemon.api_version != API_VERSION {
        return Err(format!(
            "Daemon API {} is not supported; expected {API_VERSION}.",
            daemon.api_version
        ));
    }
    if daemon.database_schema_version < MIN_DATABASE_SCHEMA_VERSION {
        return Err(format!(
            "Database schema {} is not supported; expected at least {}. No mutation was attempted.",
            daemon.database_schema_version, MIN_DATABASE_SCHEMA_VERSION
        ));
    }
    Ok(())
}

fn parse_product_version(value: &str, label: &str) -> Result<Version, String> {
    Version::parse(value.trim_start_matches('v'))
        .map_err(|_| format!("{label} reported an invalid version"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn build(version: &str) -> BuildInfo {
        BuildInfo {
            version: version.into(),
        }
    }

    fn system(version: &str) -> SystemInfo {
        SystemInfo {
            api_version: API_VERSION.into(),
            database_schema_version: MIN_DATABASE_SCHEMA_VERSION,
            status: "ready".into(),
            version: version.into(),
        }
    }

    #[test]
    fn accepts_exact_product_contract() {
        assert!(
            verify(
                &build(env!("CARGO_PKG_VERSION")),
                &system(env!("CARGO_PKG_VERSION"))
            )
            .is_ok()
        );
    }

    #[test]
    fn rejects_daemon_before_any_native_mutation() {
        assert!(verify(&build(env!("CARGO_PKG_VERSION")), &system("9.0.0")).is_err());
        let mut wrong_schema = system(env!("CARGO_PKG_VERSION"));
        wrong_schema.database_schema_version = MIN_DATABASE_SCHEMA_VERSION - 1;
        assert!(verify(&build(env!("CARGO_PKG_VERSION")), &wrong_schema).is_err());
    }

    #[test]
    fn accepts_compatible_v1_daemon_and_additive_schema() {
        let mut daemon = system("1.9.0");
        daemon.database_schema_version = MIN_DATABASE_SCHEMA_VERSION + 2;
        assert!(verify(&build(env!("CARGO_PKG_VERSION")), &daemon).is_ok());
    }
}
