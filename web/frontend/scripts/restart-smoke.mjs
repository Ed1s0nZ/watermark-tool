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
  WATERMARK_JOB_ARCHIVE_TTL_SECONDS: "8",
  WATERMARK_COOKIE_SECURE: "false",
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
  // A real background task retains its completed archive across process restart,
  // then expires the file without revoking its registered copies.
  await start();
  await login();
  const taskResponse = await fetch(base + "/api/jobs", {
    method: "POST",
    headers: {
      Cookie: cookie,
      "Content-Type": "application/json",
      "Idempotency-Key": "process-job-request-0001",
    },
    body: JSON.stringify({
      sources: [
        {
          name: "public-job.png",
          source_sha256: before,
          width: source.readUInt32BE(16),
          height: source.readUInt32BE(20),
        },
      ],
      recipients_csv:
        "recipient_id,name\n001,PUBLIC_JOB_PERSON_A\n002,PUBLIC_JOB_PERSON_B",
      mode: "blind",
    }),
  });
  assert.equal(taskResponse.status, 201);
  const task = await taskResponse.json();
  const upload = new FormData();
  upload.append("file", new Blob([source]), "original.png");
  upload.append("source_id", task.sources[0].id);
  const uploaded = await fetch(base + "/api/jobs/" + task.id + "/inputs", {
    method: "POST",
    headers: { Cookie: cookie, "X-Upload-Token": task.upload_token },
    body: upload,
  });
  assert.equal(uploaded.status, 200);
  await uploaded.json();
  await json("/jobs/" + task.id + "/start", {});
  let completed;
  for (let i = 0; i < 100; i++) {
    completed = await (
      await fetch(base + "/api/jobs/" + task.id, {
        headers: { Cookie: cookie },
      })
    ).json();
    if (completed.status === "completed") break;
    assert.notEqual(completed.status, "failed");
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  assert.equal(completed.generated, true);
  const firstArchive = await fetch(base + "/api/jobs/" + task.id + "/archive", {
    headers: { Cookie: cookie },
  });
  assert.equal(firstArchive.status, 200);
  const taskZIP = Buffer.from(await firstArchive.arrayBuffer());
  assert.equal(hash(taskZIP), completed.archive_sha256);
  await stop();
  assert.equal(existsSync(join(data, "artifacts", task.id + ".zip")), true);
  await start();
  await login();
  const again = await fetch(base + "/api/jobs/" + task.id + "/archive", {
    headers: { Cookie: cookie },
  });
  assert.equal(again.status, 200);
  assert.equal(hash(Buffer.from(await again.arrayBuffer())), hash(taskZIP));
  for (
    let i = 0;
    i < 120 && existsSync(join(data, "artifacts", task.id + ".zip"));
    i++
  )
    await new Promise((resolve) => setTimeout(resolve, 100));
  const expired = await fetch(base + "/api/jobs/" + task.id + "/archive", {
    headers: { Cookie: cookie },
  });
  assert.equal(expired.status, 410);
  await expired.json();
  assert.equal(existsSync(join(data, "artifacts", task.id + ".zip")), false);
  const traceBody = new FormData();
  traceBody.append("file", new Blob([firstPNG(taskZIP)]), "old-copy.png");
  const registered = await fetch(base + "/api/platform/trace", {
    method: "POST",
    headers: { Cookie: cookie },
    body: traceBody,
  });
  assert.equal(registered.status, 200);
  const retained = await registered.json();
  assert.equal(retained.match, "registered");
  assert.equal(retained.record.job_id, task.id);
  const audit = await json("/audit/search", {
    action: "task.create",
    page: 1,
    page_size: 100,
  });
  assert.equal(audit.items.filter((e) => e.target === task.id).length, 1);
  assert.ok(!JSON.stringify(audit).includes("PUBLIC_JOB_PERSON"));

  const crashCSV =
    "recipient_id,name\n" +
    Array.from({ length: 20 }, (_, i) => `${i + 1},PUBLIC_CRASH_${i + 1}`).join(
      "\n",
    );
  const pendingResponse = await fetch(base + "/api/jobs", {
    method: "POST",
    headers: {
      Cookie: cookie,
      "Content-Type": "application/json",
      "Idempotency-Key": "crash-job-request-0001",
    },
    body: JSON.stringify({
      sources: [
        {
          name: "crash-source.png",
          source_sha256: before,
          width: source.readUInt32BE(16),
          height: source.readUInt32BE(20),
        },
      ],
      recipients_csv: crashCSV,
      mode: "blind",
    }),
  });
  assert.equal(pendingResponse.status, 201);
  const pendingTask = await pendingResponse.json();
  async function sendInput(task) {
    const form = new FormData();
    form.append("source_id", task.sources[0].id);
    form.append("file", new Blob([source]), "original.png");
    const response = await fetch(base + "/api/jobs/" + task.id + "/inputs", {
      method: "POST",
      headers: { Cookie: cookie, "X-Upload-Token": task.upload_token },
      body: form,
    });
    assert.equal(response.status, 200);
    await response.json();
  }
  await sendInput(pendingTask);
  await json("/jobs/" + pendingTask.id + "/start", {});
  let processing;
  for (let i = 0; i < 200; i++) {
    processing = await (
      await fetch(base + "/api/jobs/" + pendingTask.id, {
        headers: { Cookie: cookie },
      })
    ).json();
    if (processing.status === "processing") break;
    assert.notEqual(processing.status, "completed");
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  assert.equal(processing.status, "processing");
  const killed = child;
  child = undefined;
  killed.kill("SIGKILL");
  await once(killed, "exit");
  await start();
  await login();
  const recovered = await (
    await fetch(base + "/api/jobs/" + pendingTask.id, {
      headers: { Cookie: cookie },
    })
  ).json();
  assert.equal(recovered.status, "interrupted");
  assert.notEqual(recovered.upload_token, pendingTask.upload_token);
  assert.equal(recovered.generated, false);
  assert.deepEqual(
    recovered.copies.map((c) => c.id),
    pendingTask.copies.map((c) => c.id),
  );
  const noPartial = await fetch(
    base + "/api/jobs/" + pendingTask.id + "/archive",
    { headers: { Cookie: cookie } },
  );
  assert.equal(noPartial.status, 409);
  await noPartial.json();
  await sendInput(recovered);
  await json("/jobs/" + pendingTask.id + "/retry", {});
  let resumed;
  for (let i = 0; i < 200; i++) {
    resumed = await (
      await fetch(base + "/api/jobs/" + pendingTask.id, {
        headers: { Cookie: cookie },
      })
    ).json();
    if (resumed.status === "completed") break;
    assert.notEqual(resumed.status, "failed");
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  assert.equal(resumed.generated, true);
  assert.equal(resumed.attempts, 2);
  assert.deepEqual(
    resumed.copies.map((c) => c.id),
    pendingTask.copies.map((c) => c.id),
  );
  await stop();
  assert.equal(
    hash(readFileSync(join(root, "internal/blind/testdata/clean.png"))),
    before,
  );
  assert.ok(!logs.includes("PUBLIC_JOB_PERSON_A"));
  assert.ok(!logs.includes(master));
  assert.ok(!logs.includes("PUBLIC_RECIPIENT_A"));
  console.log(
    "Restart smoke passed: background archive survives restart then expires with registration intact; SIGKILL recovers interrupted with same IDs and successful retry; records + rotation persist; old-copy trace + exact idempotent ZIP survive restart; wrong master fails closed; runtime cleaned; original intact; no recipient/master in logs.",
  );
} finally {
  await stop();
  rmSync(scratch, { recursive: true, force: true });
}
