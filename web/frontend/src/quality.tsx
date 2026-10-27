import { Card, Form, InputNumber, Space, Switch, Tag, Typography } from "antd";
import { useArtText } from "./artText";
import { useFeatureText } from "./traceText";
export interface QualityPolicy {
  minimum_psnr_db?: number;
}
export interface Metric {
  status: "finite" | "infinite" | "unavailable";
  psnr_db?: number;
}
export interface Quality {
  encoding?: Metric;
  final?: Metric;
  output_sha256?: string;
  output_bytes?: number;
  format?: string;
  jpeg_quality?: number;
  width: number;
  height: number;
  capacity_bytes: number;
  verified: boolean;
  embedding: Metric;
  visible?: Metric;
  accepted: boolean;
}
export function validQuality(v: unknown): v is QualityPolicy {
  if (!v || typeof v !== "object" || Array.isArray(v)) return false;
  const p = v as QualityPolicy;
  return (
    Object.keys(p).every((k) => k === "minimum_psnr_db") &&
    (p.minimum_psnr_db === undefined ||
      (Number.isFinite(p.minimum_psnr_db) &&
        p.minimum_psnr_db >= 20 &&
        p.minimum_psnr_db <= 60))
  );
}
export function QualityFields({
  value,
  onChange,
  disabled = false,
}: {
  value: QualityPolicy;
  onChange: (v: QualityPolicy) => void;
  disabled?: boolean;
}) {
  const t = useFeatureText();
  return (
    <Form.Item label={t.qualityPolicy} extra={t.qualityHint}>
      <Space wrap>
        <Switch
          aria-label={t.gate}
          checked={value.minimum_psnr_db !== undefined}
          disabled={disabled}
          onChange={(v) => onChange(v ? { minimum_psnr_db: 35 } : {})}
        />
        <Typography.Text>{t.gate}</Typography.Text>
        {value.minimum_psnr_db !== undefined && (
          <InputNumber
            aria-label={t.minimum}
            min={20}
            max={60}
            step={0.1}
            value={value.minimum_psnr_db}
            disabled={disabled}
            onChange={(v) => {
              if (v !== null) onChange({ minimum_psnr_db: v });
            }}
          />
        )}
      </Space>
    </Form.Item>
  );
}
export function QualitySummary({ value }: { value?: Quality }) {
  const t = useFeatureText();
  const at = useArtText();
  const metric = (m?: Metric) =>
    !m
      ? t.unrecorded
      : m.status === "infinite"
        ? t.infinite
        : m.status === "unavailable"
          ? t.unavailable
          : `${m.psnr_db?.toFixed(2)} dB`;
  return (
    <Card size="small" title={t.quality}>
      <Space direction="vertical" style={{ width: "100%" }}>
        {!value ? (
          <Typography.Text>{t.unrecorded}</Typography.Text>
        ) : (
          <>
            <Space wrap>
              <Tag color={value.accepted ? "success" : "error"}>
                {value.accepted ? t.accepted : t.rejected}
              </Tag>
              <Typography.Text>
                {value.width} × {value.height} · {t.capacity}{" "}
                {value.capacity_bytes} bytes ·{" "}
                {value.verified ? t.verified : at.notVerified}
              </Typography.Text>
            </Space>
            <Typography.Text>
              {t.embedding}: {metric(value.embedding)}
            </Typography.Text>
            {value.encoding && (
              <Typography.Text>
                {at.encoding}: {metric(value.encoding)}
              </Typography.Text>
            )}
            {value.final && (
              <Typography.Text>
                {at.final}: {metric(value.final)}
              </Typography.Text>
            )}
            {value.output_sha256 && (
              <>
                <Typography.Text>
                  {value.format?.toUpperCase()} {value.jpeg_quality ?? ""} ·{" "}
                  {at.bytes}: {value.output_bytes}
                </Typography.Text>
                <Typography.Paragraph
                  style={{ overflowWrap: "anywhere" }}
                  copyable
                >
                  {at.digest}: {value.output_sha256}
                </Typography.Paragraph>
              </>
            )}
            {value.visible && (
              <Typography.Text>
                {t.visible}: {metric(value.visible)}
              </Typography.Text>
            )}
          </>
        )}
        <Typography.Text type="secondary">{t.metricHint}</Typography.Text>
      </Space>
    </Card>
  );
}
