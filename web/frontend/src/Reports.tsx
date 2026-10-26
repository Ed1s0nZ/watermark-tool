import { useEffect } from "react";
import { Alert, Button, Card, Space, Typography } from "antd";
import { api } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useJob } from "./useJob";
import { useWorkbenchText } from "./workbenchText";
export interface VerificationReport {
  kind: string;
  schema_version: number;
  source_sha256: string;
  width: number;
  height: number;
  tested_at: string;
  elapsed_ms: number;
  algorithm: string;
  status: "registered" | "unregistered" | "not_found" | "invalid";
  error_code?: string;
  result?: { watermark: string; timestamp: string };
  key_id?: string;
  record?: unknown;
}
function escapeHTML(s: string) {
  return s.replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ]!,
  );
}
export function reportHTML(report: unknown, title: string, hint: string) {
  const data = report as Record<string, unknown>;
  const value = (v: unknown) =>
    escapeHTML(typeof v === "object" ? JSON.stringify(v) : String(v ?? "—"));
  const metadata = Object.entries(data)
    .filter(([key]) => !["cases", "result", "record"].includes(key))
    .map(
      ([key, v]) => `<tr><th>${escapeHTML(key)}</th><td>${value(v)}</td></tr>`,
    )
    .join("");
  const cases = Array.isArray(data.cases)
    ? `<h2>Experiments / 实验结果</h2><table><thead><tr><th>case_id</th><th>width × height</th><th>recovered</th><th>error_code</th><th>control_passed</th><th>PSNR dB</th><th>elapsed_ms</th></tr></thead><tbody>${data.cases.map((row: Record<string, unknown>) => `<tr><td>${value(row.case_id)}</td><td>${value(row.width)} × ${value(row.height)}</td><td>${value(row.recovered)}</td><td>${value(row.error_code)}</td><td>${value(row.control_passed)}</td><td>${value(row.psnr_db)}</td><td>${value(row.elapsed_ms)}</td></tr>`).join("")}</tbody></table>`
    : "";
  return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'"><title>${escapeHTML(title)}</title><style>body{font:15px system-ui;max-width:1000px;margin:40px auto;padding:20px;color:#182238}h1{border-bottom:3px solid #3864e8;padding-bottom:16px}pre,td{overflow-wrap:anywhere}pre{white-space:pre-wrap;background:#f5f7fa;padding:20px}p{line-height:1.6}table{width:100%;border-collapse:collapse;margin:24px 0;table-layout:fixed}th,td{border:1px solid #dde3ed;padding:10px;text-align:left;vertical-align:top}th{background:#f5f7fa}h2{font-size:18px;margin-top:28px}@media print{body{margin:0;padding:0}pre{background:white}tr{break-inside:avoid}h2{break-after:avoid}}</style></head><body><h1>${escapeHTML(title)}</h1><p>${escapeHTML(hint)}</p><table>${metadata}</table>${cases}<h2>Full result / 完整结果</h2><pre>${escapeHTML(JSON.stringify(report, null, 2))}</pre></body></html>`;
}

function download(data: string, type: string, name: string) {
  const url = URL.createObjectURL(new Blob([data], { type }));
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export function ReportDownloads({ report }: { report: unknown }) {
  const t = useWorkbenchText();
  return (
    <Space wrap>
      <Button
        onClick={() =>
          download(
            JSON.stringify(report, null, 2),
            "application/json",
            "watermark-report.json",
          )
        }
      >
        {t.json}
      </Button>
      <Button
        onClick={() =>
          download(
            reportHTML(report, t.report, t.reportHint),
            "text/html;charset=utf-8",
            "watermark-report.html",
          )
        }
      >
        {t.html}
      </Button>
    </Space>
  );
}
export function ReportVerifier({
  file,
  keyID = "",
  disabled = false,
}: {
  file?: File;
  keyID?: string;
  disabled?: boolean;
}) {
  const t = useWorkbenchText();
  const job = useJob<VerificationReport>();
  useEffect(() => {
    job.reset();
  }, [file, keyID]);
  return (
    <Card size="small" title={t.report}>
      <Space direction="vertical" style={{ width: "100%" }}>
        <Typography.Text type="secondary">{t.reportHint}</Typography.Text>
        <Button
          aria-label={t.verify}
          disabled={!file || disabled}
          loading={job.busy}
          onClick={() =>
            void job.run(async (signal) => {
              const body = new FormData();
              body.append("file", file!);
              if (keyID) body.append("key_id", keyID);
              return (
                await api("/workbench/verify", { method: "POST", body, signal })
              ).json();
            })
          }
        >
          {t.verify}
        </Button>
        {job.busy && <Button onClick={job.cancel}>{t.cancel}</Button>}
        {job.cancelled && <Alert message={t.cancelled} />}
        <ErrorNotice error={job.error} />
        {job.value && (
          <>
            <Alert
              type={
                job.value.status === "registered" ||
                job.value.status === "unregistered"
                  ? "success"
                  : "warning"
              }
              message={
                job.value.status === "invalid"
                  ? t.invalidStatus
                  : t[job.value.status]
              }
            />
            <Typography.Paragraph style={{ overflowWrap: "anywhere" }} copyable>
              {job.value.source_sha256}
            </Typography.Paragraph>
            <ReportDownloads report={job.value} />
          </>
        )}
      </Space>
    </Card>
  );
}
