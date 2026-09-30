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
    // Keys starting with ^ are regular expressions: a page path such as
    // /api-tokens stays the app's, as it is in production.
    proxy: { "^/api(/|$)": backend, "^/healthz$": backend, "^/readyz$": backend },
  },
  build: {
    rolldownOptions: {
      output: {
        // React and the router change less often than the app: their own chunk
        // stays cached across releases that change only the app.
        codeSplitting: {
          groups: [{ name: "react", test: /node_modules[\\/](react|react-dom|react-router|scheduler)[\\/]/ }],
        },
      },
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["src/test/setup.ts"],
    restoreMocks: true,
    unstubGlobals: true,
  },
});
