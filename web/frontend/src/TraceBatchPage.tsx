import { FileUpload } from "./FileUpload";
import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Progress,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";
import { api, APIError } from "./api";
import { useFeatureText } from "./traceText";
import { VerificationReport, reportHTML } from "./Reports";
import { ErrorNotice } from "./ErrorNotice";
import { saveBlob } from "./jobClient";
interface Item {
  index: number;
  name: string;
  status: string;
  duplicate_of?: number;
  report?: VerificationReport;
  error_code?: string;
}
export interface TraceBatchReport {
  kind: string;
  schema_version: number;
  batch_id: string;
  client_request_id: string;
  tested_at: string;
  status: string;
  total: number;
  done: number;
  items: Item[];
}
export function batchCSV(r: TraceBatchReport) {
  const cell = (v: unknown) => {
    let s = String(v ?? "");
    if (/^[\s\p{Cf}]*[=+@-]/u.test(s)) s = "'" + s;
    return '"' + s.replaceAll('"', '""') + '"';
  };
  const rows = [
    [
      "batch_id",
      "batch_status",
      "total",
      "done",
      "index",
      "name",
      "status",
      "duplicate_of",
      "source_sha256",
      "width",
      "height",
      "error_code",
      "verification_json",
    ],
    ...r.items.map((i) => [
      r.batch_id,
      r.status,
      r.total,
      r.done,
      i.index,
      i.name,
      i.status,
      i.duplicate_of,
      i.report?.source_sha256,
      i.report?.width,
      i.report?.height,
      i.error_code ?? i.report?.error_code,
      i.report ? JSON.stringify(i.report) : "",
    ]),
  ];
  return rows.map((row) => row.map(cell).join(",")).join("\r\n");
}
export function TraceBatchPage() {
  const t = useFeatureText();
  const [files, setFiles] = useState<File[]>([]),
    [report, setReport] = useState<TraceBatchReport>(),
    [busy, setBusy] = useState(false),
    [error, setError] = useState<unknown>();
  const pending = useRef<AbortController | undefined>(undefined);
  const state = useRef<TraceBatchReport | undefined>(undefined);
  const saved = useRef(false);
  function publish(next: TraceBatchReport) {
    state.current = next;
    setReport(next);
    saved.current = false;
  }
  useEffect(
    () => () => {
      pending.current?.abort();
      pending.current = undefined;
    },
    [],
  );
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => {
      if (state.current && !saved.current) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, []);
  function cancel() {
    const c = pending.current;
    if (!c) return;
    pending.current = undefined;
    c.abort();
    setBusy(false);
    if (state.current) publish({ ...state.current, status: "cancelled" });
  }
  async function run() {
    if (pending.current) return;
    if (
      !files.length ||
      files.length > 20 ||
      files.some((f) => f.size > 50 * 1024 * 1024) ||
      files.reduce((s, f) => s + f.size, 0) > 128 * 1024 * 1024
    ) {
      setError(new APIError(422, "trace_limit"));
      return;
    }
    const c = new AbortController();
    pending.current = c;
    setBusy(true);
    setError(undefined);
    publish({
      kind: "batch_verification",
      schema_version: 1,
      batch_id: "",
      client_request_id: crypto.randomUUID(),
      tested_at: new Date().toISOString(),
      status: "uploading",
      total: files.length,
      done: 0,
      items: files.map((f, i) => ({
        index: i + 1,
        name: f.name,
        status: "not_confirmed",
      })),
    });
    let ended = false;
    try {
      const body = new FormData();
      files.forEach((f) => body.append("files", f));
      const response = await api("/workbench/trace-batch", {
        method: "POST",
        body,
        signal: c.signal,
      });
      if (!response.body) throw Error("stream");
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      while (true) {
        const next = await reader.read();
        if (c.signal.aborted) return;
        buffer += decoder.decode(next.value, { stream: !next.done });
        let at: number;
        while ((at = buffer.indexOf("\n")) >= 0) {
          const line = buffer.slice(0, at);
          buffer = buffer.slice(at + 1);
          if (!line.trim()) continue;
          const frame = JSON.parse(line);
          let r = state.current!;
          if (frame.type === "begin") {
            if (
              frame.total !== r.total ||
              !/^[a-f0-9]{32}$/.test(frame.batch_id)
            )
              throw Error("stream");
            publish({ ...r, batch_id: frame.batch_id, status: "running" });
          } else if (frame.type === "item") {
            if (
              !Number.isInteger(frame.index) ||
              frame.index !== r.done + 1 ||
              frame.index > r.total ||
              !frame.report
            )
              throw Error("stream");
            const items = [...r.items];
            items[frame.index - 1] = {
              index: frame.index,
              name: items[frame.index - 1].name,
              status: frame.report.status,
              report: frame.report,
              duplicate_of: frame.duplicate_of,
              error_code: frame.error_code,
            };
            publish({ ...r, done: r.done + 1, items });
          } else if (frame.type === "end") {
            if (
              r.done !== r.total ||
              !["completed", "partial"].includes(frame.status)
            )
              throw Error("stream");
            ended = true;
            publish({ ...r, status: frame.status });
          } else if (frame.type === "fatal")
            throw new APIError(503, frame.code || "requestError");
          else throw Error("stream");
        }
        if (next.done) break;
      }
      if (!ended) throw Error("stream");
    } catch (e) {
      if (!c.signal.aborted && pending.current === c) {
        setError(e);
        if (state.current) publish({ ...state.current, status: "failed" });
      }
    } finally {
      if (pending.current === c) {
        pending.current = undefined;
        setBusy(false);
      }
    }
  }
  function download(format: "json" | "csv" | "html") {
    if (!report) return;
    const text =
      format === "json"
        ? JSON.stringify(report, null, 2)
        : format === "csv"
          ? batchCSV(report)
          : reportHTML(report, t.trace, t.auditHint);
    saveBlob(
      new Blob([text], {
        type:
          format === "json"
            ? "application/json"
            : format === "csv"
              ? "text/csv;charset=utf-8"
              : "text/html;charset=utf-8",
      }),
      `batch-trace.${format}`,
    );
    saved.current = true;
  }
  const label = (s: string) => (s in t ? t[s as keyof typeof t] : s);
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.trace}</Typography.Title>
        <Typography.Text type="secondary">{t.hint}</Typography.Text>
      </div>
      <Alert message={t.temporary} />
      <Card>
        <Space direction="vertical" style={{ width: "100%" }}>
          <FileUpload
            label={t.files}
            value={files}
            disabled={busy}
            maxCount={20}
            maxTotal={128 * 1024 * 1024}
            onChange={(files) => {
              setFiles(files);
              state.current = undefined;
              setReport(undefined);
              setError(undefined);
            }}
          />
          <Space wrap>
            <Button
              type="primary"
              aria-label={t.run}
              loading={busy}
              disabled={!files.length}
              onClick={() => void run()}
            >
              {t.run}
            </Button>
            {busy && (
              <Button danger onClick={cancel}>
                {t.cancel}
              </Button>
            )}
          </Space>
          <ErrorNotice error={error} />
          {error != null && <Alert type="warning" message={t.network} />}
        </Space>
      </Card>
      {report && (
        <Card title={label(report.status)}>
          <Space direction="vertical" style={{ width: "100%" }}>
            <Typography.Text>
              {report.done} / {report.total}
            </Typography.Text>
            <Progress
              percent={Math.round((100 * report.done) / report.total)}
              status={
                report.status === "completed"
                  ? "success"
                  : ["failed", "cancelled", "partial"].includes(report.status)
                    ? "exception"
                    : "active"
              }
            />
            <Typography.Text style={{ overflowWrap: "anywhere" }}>
              {report.batch_id || report.client_request_id}
            </Typography.Text>
            <Space wrap>
              <Button onClick={() => download("json")}>{t.json}</Button>
              <Button onClick={() => download("csv")}>{t.csv}</Button>
              <Button onClick={() => download("html")}>{t.html}</Button>
            </Space>
            <Table<Item>
              rowKey="index"
              dataSource={report.items}
              pagination={false}
              scroll={{ x: 650 }}
              columns={[
                { title: t.index, dataIndex: "index", width: 80 },
                {
                  title: t.name,
                  dataIndex: "name",
                  width: 220,
                  render: (v: string, i) => (
                    <Space direction="vertical">
                      <Typography.Text style={{ overflowWrap: "anywhere" }}>
                        {v}
                      </Typography.Text>
                      {i.duplicate_of && (
                        <Tag>
                          {t.duplicate} {i.duplicate_of}
                        </Tag>
                      )}
                    </Space>
                  ),
                },
                {
                  title: t.status,
                  dataIndex: "status",
                  render: (v: string) => <Tag>{label(v)}</Tag>,
                },
              ]}
              expandable={{
                expandedRowRender: (i) => (
                  <pre
                    style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}
                  >
                    {JSON.stringify(i.report ?? { status: i.status }, null, 2)}
                  </pre>
                ),
              }}
            />
            <Typography.Text type="secondary">{t.auditHint}</Typography.Text>
          </Space>
        </Card>
      )}
    </Space>
  );
}
