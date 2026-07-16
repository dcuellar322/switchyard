use tauri::{
    AppHandle, Manager, Runtime,
    menu::{CheckMenuItemBuilder, Menu, MenuBuilder, MenuEvent, SubmenuBuilder},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
};
use tauri_plugin_autostart::ManagerExt;
use tauri_plugin_notification::NotificationExt;

use crate::{
    DesktopState, ensure_compatible, model::DesktopSnapshot, open_route, sidecar::Sidecar, updater,
};

const TRAY_ID: &str = "switchyard-tray";
const RESOURCE_LIMIT: usize = 8;

pub fn install<R: Runtime>(
    app: &AppHandle<R>,
    snapshot: Option<&DesktopSnapshot>,
) -> tauri::Result<()> {
    let menu = build_menu(app, snapshot)?;
    TrayIconBuilder::with_id(TRAY_ID)
        .menu(&menu)
        .icon(
            app.default_window_icon()
                .cloned()
                .ok_or_else(|| tauri::Error::AssetNotFound("default window icon".into()))?,
        )
        .tooltip("Switchyard — connecting")
        .on_menu_event(handle_menu)
        .on_tray_icon_event(|tray, event| {
            if matches!(
                event,
                TrayIconEvent::Click {
                    button: MouseButton::Left,
                    button_state: MouseButtonState::Up,
                    ..
                }
            ) {
                show_main(tray.app_handle());
            }
        })
        .build(app)?;
    Ok(())
}

pub fn refresh<R: Runtime>(app: &AppHandle<R>, snapshot: &DesktopSnapshot) -> tauri::Result<()> {
    let menu = build_menu(app, Some(snapshot))?;
    if let Some(tray) = app.tray_by_id(TRAY_ID) {
        tray.set_menu(Some(menu))?;
        tray.set_tooltip(Some(format!(
            "Switchyard — {} project(s), daemon {}",
            snapshot.projects.len(),
            snapshot.system.status
        )))?;
    }
    Ok(())
}

fn build_menu<R: Runtime>(
    app: &AppHandle<R>,
    snapshot: Option<&DesktopSnapshot>,
) -> tauri::Result<Menu<R>> {
    let preferences = app
        .try_state::<DesktopState>()
        .and_then(|state| state.preferences.lock().ok().map(|value| value.clone()))
        .unwrap_or_default();
    let autostart = app.autolaunch().is_enabled().unwrap_or(false);

    let status = snapshot
        .map(|value| format!("Daemon: {} ({})", value.system.status, value.system.version))
        .unwrap_or_else(|| "Daemon: connecting…".into());
    let status_item = tauri::menu::MenuItemBuilder::with_id("status", status)
        .enabled(false)
        .build(app)?;
    let keep_running =
        CheckMenuItemBuilder::with_id("preference:keep-running", "Keep running when window closes")
            .checked(preferences.keep_running)
            .build(app)?;
    let launch_at_login = CheckMenuItemBuilder::with_id("preference:autostart", "Launch at login")
        .checked(autostart)
        .build(app)?;

    let mut builder = MenuBuilder::new(app)
        .item(&status_item)
        .separator()
        .text("open:home", "Open Switchyard");

    if let Some(snapshot) = snapshot {
        let mut projects = SubmenuBuilder::new(app, "Projects");
        for item in snapshot.projects.iter().take(RESOURCE_LIMIT) {
            let id = &item.project.id;
            let state = item
                .runtime
                .as_ref()
                .map(|runtime| runtime.state.as_str())
                .unwrap_or("not observed");
            let project =
                SubmenuBuilder::new(app, format!("{} — {state}", item.project.display_name))
                    .text(format!("open:project:{id}"), "Open")
                    .text(format!("start:project:{id}"), "Start")
                    .text(format!("stop:project:{id}"), "Stop")
                    .build()?;
            projects = projects.item(&project);
        }
        if snapshot.projects.len() > RESOURCE_LIMIT {
            projects = projects.text("open:home", "Open all projects…");
        }
        let projects = projects.build()?;
        builder = builder.item(&projects);

        let mut workspaces = SubmenuBuilder::new(app, "Workspaces");
        for workspace in snapshot.workspaces.iter().take(RESOURCE_LIMIT) {
            let id = &workspace.id;
            let workspace_menu = SubmenuBuilder::new(app, &workspace.name)
                .text(format!("open:workspace:{id}"), "Open")
                .text(format!("start:workspace:{id}"), "Start")
                .text(format!("stop:workspace:{id}"), "Stop")
                .build()?;
            workspaces = workspaces.item(&workspace_menu);
        }
        if snapshot.workspaces.len() > RESOURCE_LIMIT {
            workspaces = workspaces.text("open:home", "Open all workspaces…");
        }
        let workspaces = workspaces.build()?;
        builder = builder.item(&workspaces);
    }

    builder
        .separator()
        .item(&keep_running)
        .item(&launch_at_login)
        .text("update:check", "Check for Updates…")
        .separator()
        .text("app:quit", "Quit Switchyard")
        .build()
}

fn handle_menu<R: Runtime>(app: &AppHandle<R>, event: MenuEvent) {
    let id = event.id().as_ref();
    match id {
        "open:home" => open_route(app.clone(), "/".into()),
        "preference:keep-running" => toggle_keep_running(app),
        "preference:autostart" => toggle_autostart(app),
        "update:check" => {
            let app = app.clone();
            tauri::async_runtime::spawn(async move { updater::check_and_install(app).await });
        }
        "app:quit" => app.exit(0),
        _ => {
            let parts = id.split(':').collect::<Vec<_>>();
            if parts.len() != 3 {
                return;
            }
            let action = parts[0];
            let kind = parts[1];
            let resource_id = parts[2].to_owned();
            if action == "open" {
                let route = match kind {
                    "project" => format!("/projects/{resource_id}"),
                    "workspace" => format!("/workspaces?workspace={resource_id}"),
                    _ => return,
                };
                open_route(app.clone(), route);
                return;
            }
            if !matches!(action, "start" | "stop") {
                return;
            }
            let app = app.clone();
            let action = action.to_owned();
            let kind = kind.to_owned();
            tauri::async_runtime::spawn(async move {
                let sidecar = Sidecar::new(app.clone());
                let result = match ensure_compatible(&app).await {
                    Ok(_) => match kind.as_str() {
                        "project" => sidecar.project_action(&resource_id, &action).await,
                        "workspace" => sidecar.workspace_action(&resource_id, &action).await,
                        _ => return,
                    },
                    Err(error) => {
                        let _ = app
                            .notification()
                            .builder()
                            .title("Switchyard action blocked")
                            .body(error)
                            .show();
                        return;
                    }
                };
                if let Err(error) = result {
                    let _ = app
                        .notification()
                        .builder()
                        .title("Switchyard action failed")
                        .body(error.to_string())
                        .show();
                }
            });
        }
    }
}

fn toggle_keep_running<R: Runtime>(app: &AppHandle<R>) {
    let Some(state) = app.try_state::<DesktopState>() else {
        return;
    };
    let Ok(mut preferences) = state.preferences.lock() else {
        return;
    };
    preferences.keep_running = !preferences.keep_running;
    if let Err(error) = preferences.save(app) {
        let _ = app
            .notification()
            .builder()
            .title("Switchyard preference was not saved")
            .body(error.to_string())
            .show();
    }
}

fn toggle_autostart<R: Runtime>(app: &AppHandle<R>) {
    let manager = app.autolaunch();
    let result = if manager.is_enabled().unwrap_or(false) {
        manager.disable()
    } else {
        manager.enable()
    };
    if let Err(error) = result {
        let _ = app
            .notification()
            .builder()
            .title("Launch at login could not be changed")
            .body(error.to_string())
            .show();
    }
}

pub fn show_main<R: Runtime>(app: &AppHandle<R>) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
}
