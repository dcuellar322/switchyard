mod compatibility;
mod model;
mod navigation;
mod notifications;
mod preferences;
mod sidecar;
mod tray;
mod updater;

use std::{sync::Mutex, time::Duration};

use tauri::{AppHandle, Manager, Runtime, WindowEvent};
use tauri_plugin_deep_link::DeepLinkExt;
use tauri_plugin_notification::NotificationExt;

use model::DesktopSnapshot;
use notifications::Tracker;
use preferences::Preferences;
use sidecar::Sidecar;

pub(crate) struct DesktopState {
    preferences: Mutex<Preferences>,
}

pub fn run() {
    let builder = tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, args, _| {
            if let Some(route) = args
                .iter()
                .find(|argument| argument.starts_with("switchyard://"))
                .and_then(|argument| navigation::parse_deep_link(argument).ok())
                .map(|destination| destination.route())
            {
                open_route(app.clone(), route);
            } else {
                tray::show_main(app);
            }
        }))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(
            tauri_plugin_autostart::Builder::new()
                .macos_launcher(tauri_plugin_autostart::MacosLauncher::LaunchAgent)
                .build(),
        )
        .plugin(tauri_plugin_deep_link::init());
    #[cfg(not(debug_assertions))]
    let builder = builder.plugin(updater::plugin());

    builder
        .setup(|app| {
            let handle = app.handle().clone();
            handle.manage(DesktopState {
                preferences: Mutex::new(Preferences::load(&handle)),
            });
            tray::install(&handle, None)?;

            let initial_route = handle
                .deep_link()
                .get_current()
                .ok()
                .flatten()
                .and_then(|urls| urls.into_iter().next())
                .and_then(|url| navigation::parse_deep_link(url.as_str()).ok())
                .map(|destination| destination.route())
                .unwrap_or_else(|| "/".into());

            let deep_link_handle = handle.clone();
            handle.deep_link().on_open_url(move |event| {
                if let Some(route) = event
                    .urls()
                    .into_iter()
                    .next()
                    .and_then(|url| navigation::parse_deep_link(url.as_str()).ok())
                    .map(|destination| destination.route())
                {
                    open_route(deep_link_handle.clone(), route);
                }
            });

            tauri::async_runtime::spawn(async move {
                if let Err(error) = bootstrap(handle.clone(), initial_route).await {
                    show_startup_error(&handle, &error);
                }
            });
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let keep_running = window
                    .app_handle()
                    .try_state::<DesktopState>()
                    .and_then(|state| {
                        state
                            .preferences
                            .lock()
                            .ok()
                            .map(|preferences| preferences.keep_running)
                    })
                    .unwrap_or(true);
                if keep_running {
                    let _ = window.hide();
                } else {
                    window.app_handle().exit(0);
                }
            }
        })
        .run(tauri::generate_context!())
        .expect("failed to run Switchyard desktop shell");
}

async fn bootstrap<R: Runtime>(app: AppHandle<R>, route: String) -> Result<(), String> {
    let snapshot = ensure_compatible(&app).await?;
    navigate(&app, &route).await?;
    tray::refresh(&app, &snapshot).map_err(|error| error.to_string())?;
    tray::show_main(&app);

    let mut tracker = Tracker::default();
    tracker.observe(&snapshot);
    tauri::async_runtime::spawn(poll(app, tracker));
    Ok(())
}

async fn poll<R: Runtime>(app: AppHandle<R>, mut tracker: Tracker) {
    let mut disconnected = false;
    loop {
        let _ =
            tauri::async_runtime::spawn_blocking(|| std::thread::sleep(Duration::from_secs(15)))
                .await;
        let sidecar = Sidecar::new(app.clone());
        match sidecar.snapshot().await {
            Ok(snapshot) => {
                if disconnected {
                    let _ = app
                        .notification()
                        .builder()
                        .title("Switchyard daemon reconnected")
                        .body("Local project observation and controls are available again.")
                        .show();
                    disconnected = false;
                }
                for notice in tracker.observe(&snapshot) {
                    let _ = app
                        .notification()
                        .builder()
                        .title(notice.title)
                        .body(notice.body)
                        .show();
                }
                let refresh_app = app.clone();
                let _ = app.run_on_main_thread(move || {
                    let _ = tray::refresh(&refresh_app, &snapshot);
                });
            }
            Err(error) => {
                if !disconnected {
                    disconnected = true;
                    let _ = app
                        .notification()
                        .builder()
                        .title("Switchyard daemon disconnected")
                        .body(error.to_string())
                        .show();
                }
            }
        }
    }
}

pub(crate) fn open_route<R: Runtime>(app: AppHandle<R>, route: String) {
    tauri::async_runtime::spawn(async move {
        if let Err(error) = ensure_compatible(&app).await {
            let _ = app
                .notification()
                .builder()
                .title("Switchyard could not open")
                .body(error)
                .show();
            return;
        }
        if let Err(error) = navigate(&app, &route).await {
            let _ = app
                .notification()
                .builder()
                .title("Switchyard could not open")
                .body(error)
                .show();
            return;
        }
        tray::show_main(&app);
    });
}

pub(crate) async fn ensure_compatible<R: Runtime>(
    app: &AppHandle<R>,
) -> Result<DesktopSnapshot, String> {
    let sidecar = Sidecar::new(app.clone());
    let version = sidecar.version().await.map_err(|error| error.to_string())?;
    let snapshot = sidecar
        .snapshot()
        .await
        .map_err(|error| error.to_string())?;
    compatibility::verify(&version, &snapshot.system)?;
    Ok(snapshot)
}

async fn navigate<R: Runtime>(app: &AppHandle<R>, route: &str) -> Result<(), String> {
    let sidecar = Sidecar::new(app.clone());
    let address = sidecar
        .ui_url(route)
        .await
        .map_err(|error| error.to_string())?;
    let url = address
        .parse()
        .map_err(|_| "sidecar returned an invalid local application URL".to_owned())?;
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "main desktop window is unavailable".to_owned())?;
    window.navigate(url).map_err(|error| error.to_string())
}

fn show_startup_error<R: Runtime>(app: &AppHandle<R>, error: &str) {
    let payload =
        serde_json::to_string(error).unwrap_or_else(|_| "\"Unknown startup error\"".into());
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.eval(format!("window.switchyardStartupError({payload})"));
        let _ = window.show();
        let _ = window.set_focus();
    }
}
