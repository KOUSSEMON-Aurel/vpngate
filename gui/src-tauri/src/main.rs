#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::sync::Mutex;
use tauri::Manager;
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;

struct SidecarState(Mutex<Option<CommandChild>>);

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .manage(SidecarState(Mutex::new(None)))
        .setup(|app| {
            if let Ok((_rx, child)) = app
                .shell()
                .sidecar("vpngate")
                .expect("vpngate sidecar not configured (see bundle.externalBin)")
                .args(["serve"])
                .spawn()
            {
                let state = app.state::<SidecarState>();
                *state.0.lock().unwrap() = Some(child);
            }
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}