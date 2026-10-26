import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  workers: 1,
  timeout: 60000,
  use: {
    baseURL: "http://127.0.0.1:18080",
    browserName: "chromium",
    headless: true,
    actionTimeout: 10000,
    screenshot: "only-on-failure",
    launchOptions: process.env.PLAYWRIGHT_CHROME_PATH
      ? { executablePath: process.env.PLAYWRIGHT_CHROME_PATH }
      : {},
  },
  webServer: {
    command: "go run ./cmd/server",
    cwd: "../..",
    url: "http://127.0.0.1:18080",
    reuseExistingServer: false,
    env: {
      PORT: "18080",
      WATERMARK_ADMIN_USER: "admin",
      WATERMARK_BLIND_KEY: "07".repeat(32),
      WATERMARK_ADMIN_PASSWORD: "local-test-password-2026",
    },
  },
});
