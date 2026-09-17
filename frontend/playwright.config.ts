import { defineConfig } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { tmpdir } from "node:os";
import { join } from "node:path";

const e2eDatabase = join(tmpdir(), `frameflow-e2e-${randomUUID()}.db`);
const e2eUploadDir = join(tmpdir(), `frameflow-e2e-uploads-${randomUUID()}`);
const e2eVaultDir = join(tmpdir(), `frameflow-e2e-vault-${randomUUID()}`);

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
        FRAMEFLOW_DATABASE_PATH: e2eDatabase,
        FRAMEFLOW_UPLOAD_DIR: e2eUploadDir,
        FRAMEFLOW_VAULT_DIR: e2eVaultDir,
        FRAMEFLOW_RATE_LIMIT: "10000",
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
