import { FileUpload } from "./FileUpload";
import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Image,
  Input,
  Space,
  Table,
  Typography,
} from "antd";
import { api } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useJob } from "./useJob";
import { useWorkbenchText } from "./workbenchText";
import { ReportDownloads } from "./Reports";
interface Row {
  case_id: string;
  control_passed?: boolean;
  parameters: string;
  width: number;
  height: number;
  recovered: boolean;
  expected_recovery: boolean;
  error_code?: string;
  elapsed_ms: number;
  psnr_db: number | null;
  preview?: string;
  output_sha256: string;
}
interface Report {
  kind: string;
  source_sha256: string;
  baseline_ok: boolean;
  original_intact: boolean;
  cases: Row[];
}
export function LabPage({ active }: { active: boolean }) {
  const t = useWorkbenchText();
  const [file, setFile] = useState<File>();
  const [text, setText] = useState("");
  const [invalid, setInvalid] = useState(false);
  const job = useJob<Report>();
  const [originalURL, setOriginalURL] = useState("");
  useEffect(() => {
    if (!file) {
      setOriginalURL("");
      return;
    }
    const u = URL.createObjectURL(file);
    setOriginalURL(u);
    return () => URL.revokeObjectURL(u);
  }, [file]);
  useEffect(() => {
    if (!active) job.reset();
  }, [active]);
  if (!active) return null;
  const report = job.value
    ? { ...job.value, cases: job.value.cases.map(({ preview, ...row }) => row) }
    : undefined;
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.lab}</Typography.Title>
        <Typography.Text type="secondary">{t.labHint}</Typography.Text>
      </div>
      <Card>
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Typography.Text>{t.file}</Typography.Text>
          <FileUpload
            label={t.file}
            value={file ? [file] : []}
            disabled={job.busy}
            onChange={(files) => {
              job.reset();
              setInvalid(false);
              setFile(files[0]);
            }}
          />
          <Typography.Text>{t.payload}</Typography.Text>
          <Input
            aria-label={t.payload}
            placeholder={t.payload}
            value={text}
            disabled={job.busy}
            onChange={(e) => {
              job.reset();
              setInvalid(false);
              setText(e.target.value);
            }}
          />
          <Space wrap>
            <Button
              aria-label={t.run}
              type="primary"
              loading={job.busy}
              onClick={() => {
                if (
                  !file ||
                  file.size > 50 * 1024 * 1024 ||
                  !text.trim() ||
                  new TextEncoder().encode(text).length > 34
                ) {
                  setInvalid(true);
                  return;
                }
                void job.run(async (signal) => {
                  const body = new FormData();
                  body.append("file", file);
                  body.append("watermark", text);
                  return (
                    await api("/workbench/lab", {
                      method: "POST",
                      body,
                      signal,
                    })
                  ).json();
                });
              }}
            >
              {t.run}
            </Button>
            {job.busy && <Button onClick={job.cancel}>{t.cancel}</Button>}
          </Space>
          {invalid && <Alert type="error" message={t.invalid} />}
          <ErrorNotice error={job.error} />
          {job.busy && (
            <Typography.Text role="status">{t.loading}</Typography.Text>
          )}
          {job.cancelled && <Alert message={t.cancelled} />}
        </Space>
      </Card>
      {originalURL && (
        <Card title={t.source}>
          <Image
            src={originalURL}
            alt={t.source}
            width={120}
            style={{ maxWidth: "100%" }}
          />
        </Card>
      )}
      <Alert message={t.labNote} type="info" />
      {job.value && (
        <Card>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <Alert
              type={
                job.value.baseline_ok && job.value.original_intact
                  ? "success"
                  : "error"
              }
              message={t.baseline}
            />
            <ReportDownloads report={report} />
            <Table<Row>
              rowKey="case_id"
              pagination={false}
              scroll={{ x: 900 }}
              dataSource={job.value.cases}
              columns={[
                { title: t.row, dataIndex: "case_id" },
                {
                  title: t.dimensions,
                  render: (_, r) => `${r.width}×${r.height}`,
                },
                {
                  title: t.result,
                  render: (_, r) => (
                    <span>
                      {r.control_passed === false
                        ? t.controlWarning
                        : r.recovered
                          ? t.recovered
                          : t.rejected}
                      {r.error_code && (
                        <div>
                          <Typography.Text type="secondary">
                            {r.error_code}
                          </Typography.Text>
                        </div>
                      )}
                    </span>
                  ),
                },
                {
                  title: t.expected,
                  render: (_, r) =>
                    r.expected_recovery ? t.recovered : t.rejected,
                },
                {
                  title: "PSNR dB",
                  render: (_, r) => r.psnr_db?.toFixed(2) ?? "—",
                },
                { title: t.time, dataIndex: "elapsed_ms" },
                {
                  title: t.preview,
                  render: (_, r) =>
                    r.preview ? (
                      <Image width={110} src={r.preview} alt={r.case_id} />
                    ) : (
                      t.controls
                    ),
                },
              ]}
            />
          </Space>
        </Card>
      )}
    </Space>
  );
}
