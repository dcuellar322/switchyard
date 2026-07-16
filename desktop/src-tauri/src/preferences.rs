use std::{fs, io, path::PathBuf};

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Manager, Runtime};

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(default, rename_all = "camelCase")]
pub struct Preferences {
    pub keep_running: bool,
}

impl Default for Preferences {
    fn default() -> Self {
        Self { keep_running: true }
    }
}

impl Preferences {
    pub fn load<R: Runtime>(app: &AppHandle<R>) -> Self {
        let Ok(path) = path(app) else {
            return Self::default();
        };
        fs::read(path)
            .ok()
            .and_then(|bytes| serde_json::from_slice(&bytes).ok())
            .unwrap_or_default()
    }

    pub fn save<R: Runtime>(&self, app: &AppHandle<R>) -> io::Result<()> {
        let path = path(app).map_err(io::Error::other)?;
        let parent = path
            .parent()
            .ok_or_else(|| io::Error::other("preference path has no parent"))?;
        fs::create_dir_all(parent)?;
        let temporary = path.with_extension("tmp");
        fs::write(&temporary, serde_json::to_vec_pretty(self)?)?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&temporary, fs::Permissions::from_mode(0o600))?;
        }
        fs::rename(temporary, path)
    }
}

fn path<R: Runtime>(app: &AppHandle<R>) -> Result<PathBuf, String> {
    app.path()
        .app_config_dir()
        .map(|directory| directory.join("desktop.json"))
        .map_err(|error| format!("cannot resolve desktop preferences: {error}"))
}
