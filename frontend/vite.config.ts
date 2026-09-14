import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
const proxyTarget = process.env.VITE_PROXY_TARGET || "http://127.0.0.1:8080";
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      "/api": { target: proxyTarget, ws: true },
      "/health": proxyTarget,
      "/media": proxyTarget,
    },
  },
  test: {
    environment: "jsdom",
    exclude: ["e2e/**", "node_modules/**", "dist/**"],
  },
});
