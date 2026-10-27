import {
  Alert,
  Button,
  Card,
  Descriptions,
  Progress,
  Space,
  Tag,
  Typography,
} from "antd";
import { useArtText } from "./artText";
import { QualitySummary } from "./quality";
import { Collapse } from "antd";
import { useFeatureText } from "./traceText";
import { DeliveryJob } from "./jobClient";
import { taskLabel, useJobText } from "./jobText";
export function JobDetails({
  job,
  busy,
  onCancel,
  onDownload,
  onRetry,
  onRestore,
  retryReady,
}: {
  job: DeliveryJob;
  busy: boolean;
  onCancel: () => void;
  onDownload: () => void;
  onRetry: () => void;
  onRestore: () => void;
  retryReady: boolean;
}) {
  const t = useJobText();
  const ft = useFeatureText();
  const at = useArtText();
  const running = ["preparing", "queued", "processing", "cancelling"].includes(
    job.status,
  );
  const retry =
    ["preparing", "failed", "cancelled", "interrupted"].includes(job.status) &&
    !job.generated;
  return (
    <Card title={t.selected}>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <Typography.Paragraph style={{ overflowWrap: "anywhere" }} copyable>
          {job.id}
        </Typography.Paragraph>
        <Space wrap>
          <Tag
            color={
              job.generated ? "success" : running ? "processing" : "warning"
            }
          >
            {taskLabel(t, job.status)}
          </Tag>
          <Tag>{taskLabel(t, job.phase)}</Tag>
          <Tag>{taskLabel(t, job.download_state ?? "pending")}</Tag>
        </Space>
        <Progress
          percent={Math.round((100 * job.done) / Math.max(1, job.total))}
          status={
            job.status === "completed"
              ? "success"
              : ["failed", "cancelled", "interrupted"].includes(job.status)
                ? "exception"
                : "active"
          }
        />
        <Typography.Text>
          {t.verified}: {job.done} / {job.total}
        </Typography.Text>
        <Descriptions
          column={1}
          size="small"
          items={[
            { key: "key", label: t.key, children: job.key_id },
            { key: "attempt", label: t.attempts, children: job.attempts },
            { key: "time", label: t.time, children: job.created_at },
            {
              key: "count",
              label: t.count,
              children: `${job.sources.length} / ${new Set(job.copies.map((c) => c.recipient_id)).size} / ${job.total}`,
            },
            ...(job.generated
              ? [
                  {
                    key: "expires",
                    label: t.expiresAt,
                    children: job.expires_at,
                  },
                ]
              : []),
            ...(job.error
              ? [{ key: "error", label: t.error, children: job.error }]
              : []),
          ]}
        />
        {(job.artwork || job.output) && (
          <Descriptions
            column={1}
            size="small"
            items={[
              {
                key: "render",
                label: at.snapshot,
                children: (
                  <pre
                    style={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}
                  >
                    {JSON.stringify(
                      {
                        output: job.output ?? { format: "png" },
                        artwork: job.artwork ?? null,
                      },
                      null,
                      2,
                    )}
                  </pre>
                ),
              },
            ]}
          />
        )}
        <Button disabled={busy} onClick={onRestore}>
          {t.restore}
        </Button>
        <Space direction="vertical">
          {job.sources.map((src) => (
            <Typography.Text key={src.id} style={{ overflowWrap: "anywhere" }}>
              {src.name} · {src.source_sha256}
            </Typography.Text>
          ))}
        </Space>
        <Typography.Text>
          {ft.minimum}: {job.quality_policy?.minimum_psnr_db ?? ft.qualityHint}
        </Typography.Text>
        <Collapse
          items={job.copies.map((c) => ({
            key: c.id,
            label: `${c.name} · ${c.id}`,
            children: <QualitySummary value={c.quality} />,
          }))}
        />
        {running && <Alert message={t.working} />}
        <Space wrap>
          {running && (
            <Button
              danger
              disabled={job.status === "cancelling"}
              onClick={onCancel}
            >
              {t.cancel}
            </Button>
          )}
          {retry && (
            <Button disabled={busy || !retryReady} onClick={onRetry}>
              {t.resume}
            </Button>
          )}
          {job.download_state === "ready" && (
            <Button
              type="primary"
              loading={busy}
              aria-label={t.download}
              onClick={onDownload}
            >
              {t.download}
            </Button>
          )}
        </Space>
        {retry && (
          <Typography.Text type="secondary">{t.retryHint}</Typography.Text>
        )}
        {job.generated && (
          <Alert
            type={job.download_state === "ready" ? "info" : "warning"}
            message={t.retention}
          />
        )}
      </Space>
    </Card>
  );
}
