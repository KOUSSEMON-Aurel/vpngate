// The vpngate desktop app is a thin shell: it spawns the Go binary as a
// sidecar (`vpngate serve`) and the React frontend talks to its HTTP API
// over http://127.0.0.1:1865 — the same API the mobile app will use
// against a remote daemon. When the app exits, the sidecar's stdin pipe
// closes and `vpngate serve` shuts the tunnel down gracefully.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri_plugin_shell::ShellExt;

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
            app.shell()
                .sidecar("vpngate")
                .expect("vpngate sidecar not configured (see bundle.externalBin)")
                .args(["serve"])
                .spawn()
                .expect("failed to spawn the vpngate sidecar");
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}