import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { defineConfig } from "@playwright/test";
const ownsDirectory = !process.env.WATERMARK_E2E_DATA_DIR;
const dataDirectory =
  process.env.WATERMARK_E2E_DATA_DIR ||
  mkdtempSync(join(tmpdir(), "watermark-pw-private-"));
if (ownsDirectory)
  process.once("exit", () =>
    rmSync(dataDirectory, { recursive: true, force: true }),
  );
export default defineConfig({
  testDir: "./tests",
  outputDir:
    process.env.WATERMARK_E2E_OUTPUT_DIR || "/tmp/watermark-ui-results",
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
      WATERMARK_COOKIE_SECURE: "false",
      WATERMARK_JOB_ARCHIVE_TTL_SECONDS: "3600",
      WATERMARK_DATA_DIR: dataDirectory,
      WATERMARK_DATA_KEY: "2a".repeat(32),
      WATERMARK_ADMIN_USER: "admin",
      WATERMARK_BLIND_KEY: "07".repeat(32),
      WATERMARK_ADMIN_PASSWORD: "local-test-password-2026",
    },
  },
});
