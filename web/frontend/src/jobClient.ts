import { Quality, QualityPolicy } from "./quality";
import { api, APIError } from "./api";
import { dimensions, renderVisible, Style } from "./visibleWatermark";
export interface Recipient {
  recipient_id: string;
  name: string;
}
export interface JobSource {
  id: string;
  name: string;
  source_sha256: string;
  width: number;
  height: number;
  bytes: number;
  uploaded: boolean;
}
export interface JobCopy extends Recipient {
  quality?: Quality;
  id: string;
  source_id: string;
  rendered_sha256?: string;
  input_uploaded: boolean;
}
export interface DeliveryJob {
  quality_policy?: QualityPolicy;
  id: string;
  upload_token: string;
  operator: string;
  created_at: string;
  updated_at: string;
  key_id: string;
  mode: "blind" | "dual";
  template_text?: string;
  style?: Style;
  sources: JobSource[];
  copies: JobCopy[];
  status: string;
  phase: string;
  done: number;
  total: number;
  attempts: number;
  error?: string;
  generated: boolean;
  download_state?: string;
  expires_at: string;
}
interface SourceInfo {
  file: File;
  name: string;
  source_sha256: string;
  width: number;
  height: number;
}
export interface Plan {
  quality: QualityPolicy;
  sources: SourceInfo[];
  recipients: Recipient[];
  csv: string;
  mode: "blind" | "dual";
  template: string;
  style: Style;
  renders: Map<string, Blob>;
}
export async function post<T>(
  path: string,
  value: unknown,
  signal?: AbortSignal,
): Promise<T> {
  return (
    await api(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(value),
      signal,
    })
  ).json();
}
function alive(signal: AbortSignal) {
  if (signal.aborted) throw new DOMException("Aborted", "AbortError");
}
export function substitute(text: string, r: Recipient) {
  return text.replace(/\{(name|recipient_id)\}/g, (_match, key) =>
    key === "name" ? r.name : r.recipient_id,
  );
}

async function inspect(file: File): Promise<SourceInfo> {
  if (
    file.size > 50 * 1024 * 1024 ||
    new TextEncoder().encode(file.name).length > 255
  )
    throw new APIError(422, "job_invalid");
  const data = await file.arrayBuffer();
  const [width, height] = dimensions(new Uint8Array(data));
  if (
    width <= 0 ||
    height <= 0 ||
    width * height > 8_000_000 ||
    Math.floor(width / 8) * Math.floor(height / 8) < 1536
  )
    throw new APIError(422, "job_invalid");
  const bitmap = await createImageBitmap(file);
  try {
    if (bitmap.width !== width || bitmap.height !== height)
      throw new APIError(422, "job_invalid");
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d")!;
    ctx.drawImage(bitmap, 0, 0);
    const pixels = ctx.getImageData(0, 0, width, height).data;
    for (let i = 3; i < pixels.length; i += 4)
      if (pixels[i] !== 255) throw new APIError(422, "job_invalid");
  } finally {
    bitmap.close();
  }
  const digest = await crypto.subtle.digest("SHA-256", data);
  return {
    file,
    name: file.name,
    source_sha256: Array.from(new Uint8Array(digest), (b) =>
      b.toString(16).padStart(2, "0"),
    ).join(""),
    width,
    height,
  };
}
export async function prepare(
  files: File[],
  csv: string,
  mode: "blind" | "dual",
  template: string,
  style: Style,
  signal: AbortSignal,
  progress: (done: number, total: number) => void,
  quality: QualityPolicy = {},
): Promise<Plan> {
  if (
    !files.length ||
    files.length > 5 ||
    files.reduce((sum, f) => sum + f.size, 0) > 128 * 1024 * 1024
  )
    throw new APIError(422, "job_invalid");
  const sources: SourceInfo[] = [];
  for (const file of files) {
    alive(signal);
    sources.push(await inspect(file));
  }
  if (new Set(sources.map((s) => s.source_sha256)).size !== sources.length)
    throw new APIError(422, "job_invalid");
  const result = await post<{ recipients: Recipient[] }>(
    "/jobs/preview-plan",
    {
      recipients_csv: csv,
      mode,
      template_text: template,
      style,
      quality_policy: quality,
    },
    signal,
  );
  alive(signal);
  const renders = new Map<string, Blob>();
  const total = sources.length * result.recipients.length;
  let done = 0;
  let size = files.reduce((sum, f) => sum + f.size, 0);
  for (const source of sources)
    for (const recipient of result.recipients) {
      alive(signal);
      if (mode === "dual") {
        const blob = await renderVisible(
          source.file,
          substitute(template, recipient),
          style,
        );
        alive(signal);
        size += blob.size;
        if (size > 512 * 1024 * 1024) throw new APIError(422, "platform_limit");
        renders.set(source.source_sha256 + "/" + recipient.recipient_id, blob);
      }
      progress(++done, total);
    }
  return {
    quality: { ...quality },
    sources,
    recipients: result.recipients,
    csv,
    mode,
    template,
    style: { ...style },
    renders,
  };
}
export async function createJob(
  plan: Plan,
  key: string,
  signal: AbortSignal,
): Promise<DeliveryJob> {
  return (
    await api("/jobs", {
      method: "POST",
      headers: { "Content-Type": "application/json", "Idempotency-Key": key },
      body: JSON.stringify({
        sources: plan.sources.map(({ file, ...s }) => s),
        recipients_csv: plan.csv,
        mode: plan.mode,
        template_text: plan.template,
        style: plan.style,
        quality_policy: plan.quality,
      }),
      signal,
    })
  ).json();
}
export function matchesPlan(plan: Plan, job: DeliveryJob) {
  const recipients = new Map(job.copies.map((r) => [r.recipient_id, r.name]));
  return (
    plan.mode === job.mode &&
    plan.quality.minimum_psnr_db === job.quality_policy?.minimum_psnr_db &&
    plan.sources.length === job.sources.length &&
    job.sources.every((s) =>
      plan.sources.some((p) => p.source_sha256 === s.source_sha256),
    ) &&
    recipients.size === plan.recipients.length &&
    plan.recipients.every((r) => recipients.get(r.recipient_id) === r.name) &&
    (job.mode === "blind" ||
      (plan.template === job.template_text &&
        Object.entries(job.style ?? {}).every(
          ([k, v]) => plan.style[k as keyof Style] === v,
        )))
  );
}
async function upload(
  job: DeliveryJob,
  source: string,
  copy: string,
  blob: Blob,
  signal: AbortSignal,
): Promise<DeliveryJob> {
  for (let attempt = 0; ; attempt++) {
    alive(signal);
    const body = new FormData();
    body.append("source_id", source);
    if (copy) body.append("copy_id", copy);
    body.append("file", blob, "input.png");
    try {
      return (
        await api("/jobs/" + job.id + "/inputs", {
          method: "POST",
          headers: { "X-Upload-Token": job.upload_token },
          body,
          signal,
        })
      ).json();
    } catch (e) {
      if (!(e instanceof APIError) || e.status !== 429 || attempt >= 30)
        throw e;
      await new Promise<void>((resolve) => {
        const timer = setTimeout(done, 2000);
        function done() {
          clearTimeout(timer);
          signal.removeEventListener("abort", done);
          resolve();
        }
        signal.addEventListener("abort", done, { once: true });
      });
    }
  }
}
export async function uploadAndStart(
  plan: Plan,
  job: DeliveryJob,
  signal: AbortSignal,
  update: (job: DeliveryJob, done: number, total: number) => void,
): Promise<DeliveryJob> {
  if (!matchesPlan(plan, job)) throw new APIError(409, "request_conflict");
  let current = job;
  let done = 0;
  const total =
    job.sources.length + (job.mode === "dual" ? job.copies.length : 0);
  for (const source of job.sources) {
    alive(signal);
    if (!source.uploaded)
      current = await upload(
        current,
        source.id,
        "",
        plan.sources.find((s) => s.source_sha256 === source.source_sha256)!
          .file,
        signal,
      );
    update(current, ++done, total);
  }
  if (job.mode === "dual")
    for (const copy of job.copies) {
      alive(signal);
      if (!copy.input_uploaded) {
        const source = job.sources.find((s) => s.id === copy.source_id)!;
        current = await upload(
          current,
          source.id,
          copy.id,
          plan.renders.get(source.source_sha256 + "/" + copy.recipient_id)!,
          signal,
        );
      }
      update(current, ++done, total);
    }
  alive(signal);
  return post<DeliveryJob>(
    "/jobs/" + job.id + (job.status === "preparing" ? "/start" : "/retry"),
    {},
    signal,
  );
}
export function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
