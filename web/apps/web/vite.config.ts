import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// make web-dev serves the app with hot reload and forwards everything that is
// the backend's to make run, so the pages talk to the same origin as in
// production.
const backend = "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: { "/api": backend, "/healthz": backend, "/readyz": backend },
  },
  test: {
    environment: "jsdom",
    restoreMocks: true,
    unstubGlobals: true,
  },
});
