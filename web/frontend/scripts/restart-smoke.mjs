// Real process restart smoke. All identities, master keys and files are disposable test data.
import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { once } from "node:events";
import { mkdtempSync, readFileSync, rmSync, existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";
import { createServer } from "node:net";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const scratch = mkdtempSync(join(tmpdir(), "watermark-restart-"));
const bin = join(scratch, "server");
const data = join(scratch, "private");
const master = "2a".repeat(32);
const socket = createServer();
socket.listen(0, "127.0.0.1");
await once(socket, "listening");
const port = socket.address().port;
await new Promise((resolve) => socket.close(resolve));
const base = `http://127.0.0.1:${port}`;
let child;
let logs = "";
let cookie = "";
const env = {
  ...process.env,
  PORT: String(port),
  WATERMARK_ADMIN_USER: "admin",
  WATERMARK_ADMIN_PASSWORD: "local-test-password-2026",
  WATERMARK_DATA_DIR: data,
  WATERMARK_DATA_KEY: master,
  WATERMARK_BLIND_KEY: "07".repeat(32),
};
const hash = (b) => createHash("sha256").update(b).digest("hex");
function launch(configuration) {
  const p = spawn(bin, [], {
    cwd: root,
    env: configuration,
    stdio: ["ignore", "pipe", "pipe"],
  });
  p.stdout.on("data", (b) => (logs += b));
  p.stderr.on("data", (b) => (logs += b));
  return p;
}
async function start() {
  child = launch(env);
  for (let i = 0; i < 80; i++) {
    if (child.exitCode !== null) throw Error("server exited before readiness");
    try {
      const response = await fetch(base + "/", {
        signal: AbortSignal.timeout(300),
      });
      if (response.status === 200) return;
    } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
  throw Error("readiness timeout");
}
async function stop() {
  if (!child || child.exitCode !== null) return;
  const p = child;
  child = undefined;
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      p.kill("SIGKILL");
      reject(Error("graceful shutdown timeout"));
    }, 12000);
    p.once("exit", () => {
      clearTimeout(timer);
      resolve();
    });
    p.kill("SIGTERM");
  });
}
async function login() {
  const response = await fetch(base + "/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      username: "admin",
      password: "local-test-password-2026",
    }),
  });
  assert.equal(response.status, 200);
  cookie = response.headers.get("set-cookie").split(";")[0];
  await response.json();
}
async function json(path, body) {
  const response = await fetch(base + "/api" + path, {
    method: "POST",
    headers: { Cookie: cookie, "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  assert.ok(response.ok, `HTTP ${response.status}`);
  return response.json();
}
async function batch(source) {
  const body = new FormData();
  body.append("file", new Blob([source]), "source.png");
  body.append(
    "recipients_csv",
    "recipient_id,name\n001,PUBLIC_RECIPIENT_A\n002,PUBLIC_RECIPIENT_B",
  );
  const response = await fetch(base + "/api/platform/batches", {
    method: "POST",
    headers: {
      Cookie: cookie,
      "Idempotency-Key": "restart-smoke-request-0001",
    },
    body,
  });
  assert.equal(response.status, 200);
  return {
    id: response.headers.get("x-batch-id"),
    zip: Buffer.from(await response.arrayBuffer()),
  };
}
function firstPNG(zip) {
  for (let i = 0; i < zip.length - 46; i++) {
    if (zip.readUInt32LE(i) !== 0x02014b50) continue;
    const n = zip.readUInt16LE(i + 28);
    if (
      !zip
        .subarray(i + 46, i + 46 + n)
        .toString()
        .endsWith(".png")
    )
      continue;
    const at = zip.readUInt32LE(i + 42);
    const begin =
      at + 30 + zip.readUInt16LE(at + 26) + zip.readUInt16LE(at + 28);
    return zip.subarray(begin, begin + zip.readUInt32LE(i + 20));
  }
  throw Error("missing PNG");
}
try {
  execFileSync("go", ["build", "-o", bin, "./cmd/server"], { cwd: root });
  const source = readFileSync(join(root, "internal/blind/testdata/clean.png"));
  const before = hash(source);
  await start();
  await login();
  const original = await batch(source);
  const key = await json("/platform/keys", undefined);
  assert.equal(key.state, "decrypt-only");
  assert.ok(!("secret" in key));
  await json(`/platform/keys/${key.id}/state`, { state: "active" });
  await stop();
  assert.equal(existsSync(join(data, "runtime")), false);
  await start();
  await login();
  const records = await json("/platform/batches/search", {
    q: original.id,
    page: 1,
    page_size: 20,
  });
  assert.equal(records.total, 1);
  assert.equal(records.items[0].generated, true);
  const available = await (
    await fetch(base + "/api/platform/keys", { headers: { Cookie: cookie } })
  ).json();
  assert.equal(available.items.find((k) => k.state === "active").id, key.id);
  const body = new FormData();
  body.append("file", new Blob([firstPNG(original.zip)]), "copy.png");
  const response = await fetch(base + "/api/platform/trace", {
    method: "POST",
    headers: { Cookie: cookie },
    body,
  });
  assert.equal(response.status, 200);
  const trace = await response.json();
  assert.equal(trace.match, "registered");
  assert.equal(trace.record.name, "PUBLIC_RECIPIENT_A");
  assert.equal(trace.key_id, "legacy-env");
  const retried = await batch(source);
  assert.equal(retried.id, original.id);
  assert.equal(hash(retried.zip), hash(original.zip));
  await stop();
  assert.equal(existsSync(join(data, "runtime")), false);
  const stateBefore = hash(readFileSync(join(data, "platform.json")));
  const keysBefore = hash(readFileSync(join(data, "keys.enc")));
  const wrong = launch({ ...env, WATERMARK_DATA_KEY: "2b".repeat(32) });
  const [code] = await once(wrong, "exit");
  assert.notEqual(code, 0);
  assert.equal(hash(readFileSync(join(data, "platform.json"))), stateBefore);
  assert.equal(hash(readFileSync(join(data, "keys.enc"))), keysBefore);
  assert.equal(
    hash(readFileSync(join(root, "internal/blind/testdata/clean.png"))),
    before,
  );
  assert.ok(!logs.includes(master));
  assert.ok(!logs.includes("PUBLIC_RECIPIENT_A"));
  console.log(
    "Restart smoke passed: records + rotation persist; old-copy trace + exact idempotent ZIP survive restart; wrong master fails closed; runtime cleaned; original intact; no recipient/master in logs.",
  );
} finally {
  await stop();
  rmSync(scratch, { recursive: true, force: true });
}
