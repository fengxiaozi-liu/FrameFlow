import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30000,
  use: {
    baseURL: "http://127.0.0.1:15173",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: [
    {
      command: "go run ./cmd/server",
      cwd: "../backend",
      url: "http://127.0.0.1:18080/health",
      reuseExistingServer: false,
      env: {
        FRAMEFLOW_ADDRESS: "127.0.0.1:18080",
        FRAMEFLOW_DATABASE_PATH: "data/e2e.db",
      },
    },
    {
      command: "npm run dev -- --host 127.0.0.1 --port 15173",
      url: "http://127.0.0.1:15173",
      reuseExistingServer: false,
      env: { VITE_PROXY_TARGET: "http://127.0.0.1:18080" },
    },
  ],
  projects: [
    {
      name: "chromium",
      use: {
        browserName: "chromium",
        channel: "chromium",
        viewport: { width: 1440, height: 1000 },
      },
    },
  ],
});
