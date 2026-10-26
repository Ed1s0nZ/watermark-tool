// Each suite gets a fresh server/rate-limit window; persisted data remains for restart validation.
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
const directory = mkdtempSync(join(tmpdir(), "watermark-e2e-"));
const cli = fileURLToPath(
  new URL("../node_modules/@playwright/test/cli.js", import.meta.url),
);
const suites =
  process.argv.length > 2
    ? [process.argv.slice(2)]
    : [["platform.spec.ts"], ["distribution.spec.ts"]];
try {
  for (const [index, suite] of suites.entries()) {
    const run = spawnSync(process.execPath, [cli, "test", ...suite], {
      stdio: "inherit",
      env: {
        ...process.env,
        WATERMARK_E2E_DATA_DIR: directory,
        WATERMARK_E2E_OUTPUT_DIR: join(
          tmpdir(),
          "watermark-ui-results",
          String(index),
        ),
      },
    });
    if (run.status !== 0) {
      process.exitCode = run.status || 1;
      break;
    }
  }
} finally {
  rmSync(directory, { recursive: true, force: true });
}
