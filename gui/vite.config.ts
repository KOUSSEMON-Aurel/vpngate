import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The vpngate GUI is transport-agnostic on purpose: the desktop build
// talks to the local sidecar over http://127.0.0.1:1865, and the mobile
// build points the same frontend at a remote daemon (see src/api.ts).
export default defineConfig({
  plugins: [react()],
  clearScreen: false,
  server: {
    port: 5173,
    strictPort: true,
  },
  envPrefix: ["VITE_", "TAURI_"],
  build: {
    target: "es2022",
  },
});