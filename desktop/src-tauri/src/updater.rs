#[cfg(not(debug_assertions))]
use tauri::plugin::TauriPlugin;
use tauri::{AppHandle, Runtime};
use tauri_plugin_notification::NotificationExt;

#[cfg(not(debug_assertions))]
const PUBLIC_KEY: &str = env!("SWITCHYARD_UPDATER_PUBLIC_KEY");
#[cfg(not(debug_assertions))]
const ENDPOINT: &str = env!("SWITCHYARD_UPDATER_ENDPOINT");

#[cfg(any(not(debug_assertions), test))]
fn validate_endpoint(value: &str) -> Result<url::Url, String> {
    let endpoint = url::Url::parse(value)
        .map_err(|_| "the configured update endpoint is invalid".to_owned())?;
    if endpoint.scheme() != "https"
        || endpoint.host_str().is_none()
        || !endpoint.username().is_empty()
        || endpoint.password().is_some()
        || endpoint.fragment().is_some()
    {
        return Err(
            "the configured update endpoint must be an HTTPS URL without credentials or a fragment"
                .to_owned(),
        );
    }
    Ok(endpoint)
}

#[cfg(not(debug_assertions))]
pub fn plugin<R: Runtime>() -> TauriPlugin<R, tauri_plugin_updater::Config> {
    tauri_plugin_updater::Builder::new()
        .pubkey(PUBLIC_KEY)
        .build()
}

pub async fn check_and_install<R: Runtime>(app: AppHandle<R>) {
    #[cfg(debug_assertions)]
    {
        let _ = app
            .notification()
            .builder()
            .title("Switchyard updates")
            .body("Update checks are enabled only in signed release builds.")
            .show();
    }

    #[cfg(not(debug_assertions))]
    {
        use tauri_plugin_updater::UpdaterExt;

        let result = async {
            let endpoint = validate_endpoint(ENDPOINT)?;
            let updater = app
                .updater_builder()
                .endpoints(vec![endpoint])
                .map_err(|error| error.to_string())?
                .build()
                .map_err(|error| error.to_string())?;
            let Some(update) = updater.check().await.map_err(|error| error.to_string())? else {
                return Ok::<_, String>(false);
            };
            app.notification()
                .builder()
                .title("Switchyard update available")
                .body(format!("Downloading signed version {}…", update.version))
                .show()
                .map_err(|error| error.to_string())?;
            update
                .download_and_install(|_, _| {}, || {})
                .await
                .map_err(|error| error.to_string())?;
            Ok(true)
        }
        .await;

        match result {
            Ok(true) => app.restart(),
            Ok(false) => {
                let _ = app
                    .notification()
                    .builder()
                    .title("Switchyard is up to date")
                    .body("No newer signed release is available.")
                    .show();
            }
            Err(error) => {
                let _ = app
                    .notification()
                    .builder()
                    .title("Switchyard update failed")
                    .body(error)
                    .show();
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_https_update_endpoint() {
        let endpoint = validate_endpoint(
            "https://updates.example.com/stable/{{target}}/{{arch}}/{{current_version}}",
        )
        .unwrap();
        assert_eq!(endpoint.scheme(), "https");
    }

    #[test]
    fn rejects_unsafe_update_endpoints() {
        for endpoint in [
            "http://updates.example.com/latest.json",
            "https://user:secret@updates.example.com/latest.json",
            "https://updates.example.com/latest.json#unsigned",
            "file:///tmp/latest.json",
        ] {
            assert!(validate_endpoint(endpoint).is_err(), "accepted {endpoint}");
        }
    }
}
