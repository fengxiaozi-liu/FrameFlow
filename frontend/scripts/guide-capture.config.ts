import { defineConfig } from "@playwright/test";
import base from "../playwright.config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  ...base,
  testDir: ".",
  testMatch: "guide-capture.ts",
  timeout: 60000,
  workers: 1,
  webServer: {
    command: "npm run dev -- --host 127.0.0.1 --port 15173",
    cwd: fileURLToPath(new URL("..", import.meta.url)),
    url: "http://127.0.0.1:15173",
    reuseExistingServer: false,
  },
});
