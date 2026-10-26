import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Select,
  Space,
  Typography,
} from "antd";
import { RecordOrigin } from "./RecordOrigin";
import { ReportVerifier } from "./Reports";
import { api } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useText } from "./i18n";
import { KeyVersion, TraceResult } from "./platform";
export function TraceForm({ active }: { active: boolean }) {
  const t = useText();
  const [file, setFile] = useState<File>();
  const [key, setKey] = useState("");
  const [keys, setKeys] = useState<KeyVersion[]>([]);
  const [result, setResult] = useState<TraceResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  useEffect(() => {
    if (active)
      api("/platform/keys")
        .then((r) => r.json())
        .then((data) => setKeys(data.items))
        .catch(setError);
  }, [active]);
  async function trace() {
    if (!file) return;
    setBusy(true);
    setError(undefined);
    setResult(undefined);
    const body = new FormData();
    body.append("file", file);
    if (key) body.append("key_id", key);
    try {
      const response = await api("/platform/trace", { method: "POST", body });
      setResult(await response.json());
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card>
      <Space direction="vertical" size={20} style={{ width: "100%" }}>
        <Typography.Text type="secondary">{t.traceHint}</Typography.Text>
        <input
          aria-label={t.traceImage}
          type="file"
          accept=".png,.jpg,.jpeg"
          disabled={busy}
          onChange={(e) => {
            setFile(e.target.files?.[0]);
            setResult(undefined);
          }}
        />
        <Select
          aria-label={t.keyID}
          style={{ width: "100%", maxWidth: 440 }}
          value={key}
          disabled={busy}
          onChange={(v) => {
            setKey(v);
            setResult(undefined);
            setError(undefined);
          }}
          options={[
            { value: "", label: t.autoKeys },
            ...keys.map((k) => ({
              value: k.id,
              label: `${k.id} · ${k.state}`,
            })),
          ]}
        />
        <ErrorNotice error={error} />
        <Button
          type="primary"
          disabled={!file}
          loading={busy}
          onClick={() => void trace()}
        >
          {t.traceAction}
        </Button>
        <ReportVerifier file={file} keyID={key} disabled={busy} />
        {result && (
          <>
            <Alert
              type={result.match === "registered" ? "success" : "info"}
              message={
                result.match === "registered"
                  ? t.traceRegistered
                  : t.traceUnregistered
              }
            />
            <Descriptions
              bordered
              column={1}
              items={[
                { key: "id", label: t.watermarkID, children: result.watermark },
                { key: "key", label: t.keyID, children: result.key_id },
                { key: "date", label: t.createdAt, children: result.timestamp },
                ...(result.record
                  ? [
                      {
                        key: "name",
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
                    ]
                  : []),
              ]}
            />
            {result.record?.job_id && <RecordOrigin record={result.record} />}
            <Typography.Text type="secondary">{t.traceCaution}</Typography.Text>
          </>
        )}
      </Space>
    </Card>
  );
}
