import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
  Typography,
} from "antd";
import { VisibleStyleFields } from "./VisibleStyleFields";
import { PolicyControls } from "./PolicyControls";
import {
  Quality,
  QualityFields,
  QualityPolicy,
  QualitySummary,
} from "./quality";
import { api, APIError } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useJob } from "./useJob";
import { useWorkbenchText } from "./workbenchText";
import {
  defaultStyle,
  loadTemplates,
  renderVisible,
  storeTemplates,
  validStyle,
  Style,
  Template,
} from "./visibleWatermark";
export function DualPage({ active }: { active: boolean }) {
  const t = useWorkbenchText();
  const [file, setFile] = useState<File>();
  const [text, setText] = useState("");
  const [payload, setPayload] = useState("");
  const [quality, setQuality] = useState<QualityPolicy>({});
  const [style, setStyle] = useState<Style>(defaultStyle);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [selected, setSelected] = useState("");
  const [name, setName] = useState("");
  const [storageError, setStorageError] = useState(false);
  const [message, setMessage] = useState("");
  const preview = useJob<Blob>();
  const output = useJob<{ blob: Blob; quality: Quality }>();
  const [previewURL, setPreviewURL] = useState("");
  const [outputURL, setOutputURL] = useState("");
  const imageInput = useRef<HTMLInputElement>(null);
  useEffect(() => {
    try {
      setTemplates(loadTemplates());
    } catch {
      setStorageError(true);
    }
  }, []);
  useEffect(() => {
    if (!preview.value) {
      setPreviewURL("");
      return;
    }
    const url = URL.createObjectURL(preview.value);
    setPreviewURL(url);
    return () => URL.revokeObjectURL(url);
  }, [preview.value]);
  useEffect(() => {
    if (!output.value) {
      setOutputURL("");
      return;
    }
    const url = URL.createObjectURL(output.value.blob);
    setOutputURL(url);
    return () => URL.revokeObjectURL(url);
  }, [output.value]);
  function reset() {
    preview.reset();
    output.reset();
    setMessage("");
  }
  useEffect(() => {
    if (!active) {
      preview.reset();
      output.reset();
    }
  }, [active]);
  function changeStyle(next: Partial<Style>) {
    reset();
    setStyle({ ...style, ...next });
  }
  function persist(items: Template[]) {
    try {
      storeTemplates(items);
      setTemplates(items);
      setStorageError(false);
      return true;
    } catch {
      setStorageError(true);
      return false;
    }
  }
  if (!active) return null;
  const busy = preview.busy || output.busy;
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.dual}</Typography.Title>
        <Typography.Text type="secondary">{t.dualHint}</Typography.Text>
      </div>
      <PolicyControls
        value={{ mode: "dual", template: "{name}", style, quality }}
        disabled={busy}
        onApply={(p) => {
          reset();
          setStyle(p.style);
          setQuality(p.quality);
        }}
      />
      <Card>
        <Form layout="vertical">
          <QualityFields
            value={quality}
            disabled={busy}
            onChange={(v) => {
              reset();
              setQuality(v);
            }}
          />
          <Form.Item label={t.dualFile}>
            <input
              ref={imageInput}
              aria-label={t.dualFile}
              type="file"
              accept=".png,.jpg,.jpeg"
              disabled={busy}
              onChange={(e) => {
                reset();
                setFile(e.target.files?.[0]);
              }}
            />
          </Form.Item>
          <Form.Item label={t.visible}>
            <Input
              aria-label={t.visible}
              value={text}
              disabled={busy}
              onChange={(e) => {
                reset();
                setText(e.target.value);
              }}
            />
          </Form.Item>
          <Form.Item label={t.payload}>
            <Input
              aria-label={t.payload}
              value={payload}
              disabled={busy}
              onChange={(e) => {
                reset();
                setPayload(e.target.value);
              }}
            />
          </Form.Item>
          <Space wrap align="start">
            <Form.Item label={t.template}>
              <Select
                aria-label={t.template}
                disabled={busy}
                style={{ width: 240 }}
                value={selected || undefined}
                placeholder={t.builtin}
                onChange={(v) => {
                  reset();
                  setSelected(v);
                  setStyle(
                    v === "corner"
                      ? defaultStyle
                      : v === "diagonal"
                        ? { ...defaultStyle, mode: "tile" }
                        : templates[Number(v)].style,
                  );
                }}
                options={[
                  { value: "corner", label: t.presetCorner },
                  { value: "diagonal", label: t.presetTile },
                  ...templates.map((i, n) => ({
                    value: String(n),
                    label: i.name,
                  })),
                ]}
              />
            </Form.Item>
            <VisibleStyleFields
              style={style}
              busy={busy}
              onChange={changeStyle}
            />
          </Space>
          <Space wrap>
            <Input
              aria-label={t.templateName}
              placeholder={t.templateName}
              maxLength={40}
              value={name}
              onChange={(e) => setName(e.target.value)}
              style={{ width: 200 }}
            />
            <Button
              disabled={busy || !name.trim() || !validStyle(style)}
              onClick={() => {
                const n = name.trim();
                const items = templates.filter((i) => i.name !== n);
                if (persist([...items, { name: n, style: { ...style } }])) {
                  setSelected(String(items.length));
                  setName("");
                }
              }}
            >
              {t.save}
            </Button>
            <Button
              disabled={busy || !/^\d+$/.test(selected)}
              onClick={() => {
                if (
                  persist(templates.filter((_, n) => String(n) !== selected))
                ) {
                  reset();
                  setSelected("");
                }
              }}
            >
              {t.remove}
            </Button>
          </Space>
        </Form>
        <Typography.Paragraph type="secondary">
          {t.templateHint}
        </Typography.Paragraph>
        {storageError && <Alert type="error" message={t.storageError} />}
        <Space wrap>
          <Button
            type="primary"
            aria-label={t.preview}
            loading={preview.busy}
            disabled={output.busy}
            onClick={() => {
              output.reset();
              setMessage("");
              if (
                !file ||
                !payload.trim() ||
                new TextEncoder().encode(payload).length > 34
              ) {
                setMessage(t.invalid);
                return;
              }
              void preview.run(() => renderVisible(file, text, style));
            }}
          >
            {t.preview}
          </Button>
          <Button
            type="primary"
            disabled={!preview.value || preview.busy}
            aria-label={t.generate}
            loading={output.busy}
            onClick={() =>
              void output.run(async (signal) => {
                const body = new FormData();
                body.append("file", preview.value!, "visible.png");
                body.append("watermark", payload);
                body.append("original", file!);
                body.append("quality_policy", JSON.stringify(quality));
                const response = await api("/workbench/dual", {
                  method: "POST",
                  body,
                  signal,
                });
                return {
                  blob: await response.blob(),
                  quality: JSON.parse(
                    response.headers.get("X-Watermark-Quality") || "null",
                  ),
                };
              })
            }
          >
            {t.generate}
          </Button>
          {busy && (
            <Button
              onClick={() => {
                preview.cancel();
                output.cancel();
              }}
            >
              {t.cancel}
            </Button>
          )}
        </Space>
        {preview.error != null && (
          <Alert
            type="error"
            message={
              preview.error instanceof Error && preview.error.message === "font"
                ? t.fontError
                : t.invalid
            }
          />
        )}
        {message && (
          <Alert style={{ marginTop: 16 }} type="error" message={message} />
        )}
        <ErrorNotice error={output.error} />
        {output.error instanceof APIError &&
          output.error.code === "quality_failed" && (
            <QualitySummary
              value={(output.error.data as { quality?: Quality })?.quality}
            />
          )}
        {(preview.cancelled || output.cancelled) && (
          <Alert message={t.cancelled} />
        )}
        <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
          {t.systemFont}
        </Typography.Paragraph>
      </Card>
      {previewURL && (
        <Card title={t.preview}>
          <Alert message={t.ready} />
          <img
            src={previewURL}
            alt={t.preview}
            style={{
              maxWidth: "100%",
              maxHeight: 540,
              objectFit: "contain",
              marginTop: 16,
            }}
          />
        </Card>
      )}
      {output.value && <QualitySummary value={output.value.quality} />}
      {outputURL && (
        <Card>
          <Space direction="vertical">
            <Alert type="success" message={t.verified} />
            <Button href={outputURL} download="dual-watermark.png">
              {t.download}
            </Button>
          </Space>
        </Card>
      )}
    </Space>
  );
}
