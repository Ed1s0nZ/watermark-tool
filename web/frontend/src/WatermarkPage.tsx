import { OutputFields, OutputOptions } from "./OutputFields";
import {
  Quality,
  QualityFields,
  QualityPolicy,
  QualitySummary,
} from "./quality";
import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Col,
  Empty,
  Form,
  Input,
  Row,
  Select,
  Space,
  Steps,
  Tag,
  Typography,
  Upload,
} from "antd";
import {
  CheckCircleOutlined,
  DownloadOutlined,
  InboxOutlined,
} from "@ant-design/icons";
import { RecordOrigin } from "./RecordOrigin";
import { ReportVerifier } from "./Reports";
import { api } from "./api";
import { useText } from "./i18n";
import type { CopyRecord } from "./platform";
import { ErrorNotice } from "./ErrorNotice";
export function WatermarkPage({
  kind,
  types,
}: {
  kind: "add" | "extract";
  types: string[];
}) {
  const t = useText();
  const [output, setOutput] = useState<OutputOptions>({ format: "png" });
  const [qualityPolicy, setQualityPolicy] = useState<QualityPolicy>({});
  const [quality, setQuality] = useState<Quality>();
  const [algorithm, setAlgorithm] = useState("legacy");
  const blindMode = algorithm === "go-dct-v1";
  const allowedTypes = blindMode ? ["png", "jpg", "jpeg"] : types;
  const textLimit = blindMode ? 34 : 100;
  const [file, setFile] = useState<File>();
  const [text, setText] = useState("");
  const [timestamp, setTimestamp] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  const [validation, setValidation] = useState<
    | "fileRequired"
    | "invalidFile"
    | "tooLarge"
    | "textRequired"
    | "textLong"
    | "blind_text"
  >();
  const [result, setResult] = useState<{
    watermark: string;
    algorithm?: string;
    match?: string;
    record?: CopyRecord | null;
    timestamp?: string;
  }>();
  const [download, setDownload] = useState<{ url: string; name: string }>();
  const url = useRef<string | undefined>(undefined);
  const pending = useRef(false);
  useEffect(
    () => () => {
      if (url.current) URL.revokeObjectURL(url.current);
    },
    [],
  );
  const byteLength = new TextEncoder().encode(text).length;
  function clearResult() {
    setQuality(undefined);
    setResult(undefined);
    setDownload(undefined);
    if (url.current) {
      URL.revokeObjectURL(url.current);
      url.current = undefined;
    }
  }
  async function submit() {
    if (pending.current) return;
    setValidation(undefined);
    setError(undefined);
    if (!file) {
      setValidation("fileRequired");
      return;
    }
    if (kind === "add" && !text.trim()) {
      setValidation("textRequired");
      return;
    }
    if (kind === "add" && byteLength > textLimit) {
      setValidation(blindMode ? "blind_text" : "textLong");
      return;
    }
    pending.current = true;
    setBusy(true);
    clearResult();
    const body = new FormData();
    body.append("file", file);
    body.append("algorithm", algorithm);
    if (kind === "add" && blindMode) {
      body.append("output", JSON.stringify(output));
      body.append("quality_policy", JSON.stringify(qualityPolicy));
    }
    if (kind === "add") body.append("watermark", text);
    try {
      const response = await api(
        kind === "add"
          ? "/add-watermark"
          : `/extract-watermark?show_timestamp=${timestamp}`,
        { method: "POST", body },
      );
      if (kind === "add") {
        const blob = await response.blob();
        setQuality(
          response.headers.get("X-Watermark-Quality")
            ? JSON.parse(response.headers.get("X-Watermark-Quality")!)
            : undefined,
        );
        url.current = URL.createObjectURL(blob);
        const dot = file.name.lastIndexOf(".");
        setDownload({
          url: url.current,
          name: `${file.name.slice(0, dot)}_watermarked${blindMode ? (output.format === "jpeg" ? ".jpg" : ".png") : file.name.slice(dot)}`,
        });
      } else {
        setResult(await response.json());
      }
    } catch (e) {
      if (
        e instanceof APIError &&
        e.data &&
        typeof e.data === "object" &&
        "quality" in e.data
      )
        setQuality((e.data as { quality: Quality }).quality);
      setError(e);
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  return (
    <Space direction="vertical" size={22} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>
          {kind === "add" ? t.addTitle : t.extractTitle}
        </Typography.Title>
        <Typography.Text type="secondary">
          {kind === "add" ? t.addHint : t.extractHint}
        </Typography.Text>
      </div>
      <Row gutter={[24, 24]}>
        <Col xs={24} lg={16}>
          <Card>
            <Form layout="vertical" onFinish={submit} requiredMark={false}>
              <Form.Item label={t.algorithm}>
                <Select
                  aria-label={t.algorithm}
                  value={algorithm}
                  disabled={busy}
                  options={[
                    { value: "legacy", label: t.legacyMode },
                    { value: "go-dct-v1", label: t.blindMode },
                  ]}
                  onChange={(value) => {
                    setAlgorithm(value);
                    clearResult();
                    setError(undefined);
                    setValidation(undefined);
                    if (
                      value !== "legacy" &&
                      file &&
                      !/\.(png|jpe?g)$/i.test(file.name)
                    )
                      setFile(undefined);
                  }}
                />
                {blindMode && (
                  <Alert
                    style={{ marginTop: 12 }}
                    type="info"
                    showIcon
                    message={t.blindHint}
                  />
                )}
              </Form.Item>
              {kind === "add" && blindMode && (
                <>
                  <OutputFields
                    value={output}
                    disabled={busy}
                    onChange={(v) => {
                      clearResult();
                      setOutput(v);
                    }}
                  />
                  <QualityFields
                    value={qualityPolicy}
                    disabled={busy}
                    onChange={(v) => {
                      clearResult();
                      setQualityPolicy(v);
                    }}
                  />
                </>
              )}
              <Form.Item label={t.upload}>
                <Upload.Dragger
                  accept={allowedTypes.map((type) => "." + type).join(",")}
                  multiple={false}
                  maxCount={1}
                  disabled={busy || !types.length}
                  fileList={
                    file
                      ? [{ uid: "selected", name: file.name, status: "done" }]
                      : []
                  }
                  beforeUpload={(selected) => {
                    const extension = selected.name
                      .split(".")
                      .pop()
                      ?.toLowerCase();
                    if (!extension || !allowedTypes.includes(extension)) {
                      setValidation("invalidFile");
                      return Upload.LIST_IGNORE;
                    }
                    if (selected.size > 50 * 1024 * 1024) {
                      setValidation("tooLarge");
                      return Upload.LIST_IGNORE;
                    }
                    setFile(selected);
                    setValidation(undefined);
                    setError(undefined);
                    clearResult();
                    return false;
                  }}
                  onRemove={() => {
                    setFile(undefined);
                    clearResult();
                    setValidation(undefined);
                  }}
                >
                  <p className="ant-upload-drag-icon">
                    <InboxOutlined aria-hidden="true" />
                  </p>
                  <p className="ant-upload-text">{t.drop}</p>
                  <p className="ant-upload-hint">{t.uploadHint}</p>
                </Upload.Dragger>
                <div className="format-list">
                  <Space wrap size={[4, 6]}>
                    {allowedTypes.map((type) => (
                      <Tag key={type}>{type.toUpperCase()}</Tag>
                    ))}
                  </Space>
                </div>
              </Form.Item>
              {kind === "add" ? (
                <Form.Item
                  label={t.text}
                  extra={t.textHint}
                  validateStatus={byteLength > textLimit ? "error" : undefined}
                >
                  <Input.TextArea
                    value={text}
                    disabled={busy}
                    onChange={(e) => {
                      setText(e.target.value);
                      clearResult();
                    }}
                    placeholder={t.textPlaceholder}
                    rows={4}
                  />
                  <div
                    className={`byte-count ${byteLength > textLimit ? "over-limit" : ""}`}
                  >
                    {byteLength} / {textLimit} {t.bytes}
                  </div>
                </Form.Item>
              ) : (
                <Form.Item>
                  <Checkbox
                    checked={timestamp}
                    disabled={busy}
                    onChange={(e) => {
                      setTimestamp(e.target.checked);
                      clearResult();
                    }}
                  >
                    {t.timestamp}
                  </Checkbox>
                </Form.Item>
              )}
              <Space direction="vertical" style={{ width: "100%" }} size={16}>
                {validation && (
                  <Alert
                    role="alert"
                    type="error"
                    showIcon
                    message={t[validation]}
                  />
                )}
                <ErrorNotice error={error} />
                <Button
                  type="primary"
                  htmlType="submit"
                  size="large"
                  loading={busy}
                  disabled={!types.length}
                  icon={
                    kind === "add" ? (
                      <DownloadOutlined aria-hidden="true" />
                    ) : (
                      <InboxOutlined aria-hidden="true" />
                    )
                  }
                >
                  {kind === "add" ? t.process : t.extractAction}
                </Button>
                {busy && (
                  <Typography.Text role="status" type="secondary">
                    {t.processing}
                  </Typography.Text>
                )}
              </Space>
            </Form>
          </Card>
          {kind === "extract" && blindMode && (
            <ReportVerifier file={file} disabled={busy} />
          )}
          <Card
            title={kind === "add" ? t.success : t.result}
            style={{ marginTop: 24 }}
          >
            <div aria-live="polite">
              {quality && <QualitySummary value={quality} />}
              {download ? (
                <Space direction="vertical" size={16}>
                  <Typography.Text type="success">
                    <CheckCircleOutlined aria-hidden="true" /> {t.success}
                  </Typography.Text>
                  <Button
                    type="primary"
                    href={download.url}
                    download={download.name}
                    icon={<DownloadOutlined aria-hidden="true" />}
                  >
                    {t.download}
                  </Button>
                </Space>
              ) : result ? (
                <Space direction="vertical" size={16} style={{ width: "100%" }}>
                  <div>
                    <Typography.Text type="secondary">
                      {t.watermark}{" "}
                      {result.algorithm === "go-dct-v1" && (
                        <Tag color="success">{t.blindVerified}</Tag>
                      )}
                    </Typography.Text>
                    <Typography.Paragraph
                      className="result-text"
                      copyable={{ tooltips: [t.copy, t.success] }}
                    >
                      {result.watermark}
                    </Typography.Paragraph>
                  </div>
                  {result.match && (
                    <Alert
                      type={result.match === "registered" ? "success" : "info"}
                      message={
                        result.match === "registered"
                          ? t.traceRegistered
                          : t.traceUnregistered
                      }
                    />
                  )}
                  {result.record && (
                    <Descriptions
                      column={1}
                      items={[
                        {
                          key: "recipient",
                          label: t.registeredRecipient,
                          children: result.record.name,
                        },
                        {
                          key: "business",
                          label: t.recipientID,
                          children: result.record.recipient_id,
                        },
                        {
                          key: "batch",
                          label: result.record.job_id ? t.taskID : t.batchID,
                          children:
                            result.record.job_id || result.record.batch_id,
                        },
                      ]}
                    />
                  )}
                  {result.record?.job_id && (
                    <RecordOrigin record={result.record} />
                  )}
                  {timestamp && (
                    <div>
                      <Typography.Text type="secondary">
                        {t.addedAt}
                      </Typography.Text>
                      <Typography.Paragraph>
                        {result.timestamp || t.noTimestamp}
                      </Typography.Paragraph>
                    </div>
                  )}
                </Space>
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    <>
                      <div>{t.emptyResult}</div>
                      <Typography.Text type="secondary">
                        {t.emptyHint}
                      </Typography.Text>
                    </>
                  }
                />
              )}
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title={t.steps}>
            <Steps
              direction="vertical"
              size="small"
              items={[
                t.step1,
                kind === "add" ? t.step2 : t.extractStep2,
                kind === "add" ? t.step3 : t.extractStep3,
              ].map((title) => ({ title }))}
            />
          </Card>
          <Alert
            className="privacy-note"
            type="info"
            showIcon
            message={t.privacy}
          />
        </Col>
      </Row>
    </Space>
  );
}
