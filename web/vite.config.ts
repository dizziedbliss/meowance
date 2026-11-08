/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The Go dev server. MEOWANCE_API overrides it (e.g. a second, fixture-mode server for tests).
const api =
  (globalThis as { process?: { env: Record<string, string | undefined> } }).process?.env.MEOWANCE_API ??
  "http://127.0.0.1:8787";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Listen on all addresses so a Windows browser can reach it inside WSL, and on a fixed
  // port so it never silently drifts when another project already has 5173.
  server: {
    host: "0.0.0.0",
    port: 5180,
    strictPort: true,
    // the Go dev server (go run ./cmd/devserver) answers /api
    proxy: { "/api": api },
  },
  preview: {
    host: "0.0.0.0",
    port: 5181,
    strictPort: true,
    proxy: { "/api": api },
  },
  test: { environment: "node" },
});
