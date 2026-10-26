import { useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Space,
  Typography,
  Upload,
} from "antd";
import { api } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useText } from "./i18n";
import { useArchive } from "./platform";
export function BatchPage() {
  const t = useText();
  const [file, setFile] = useState<File>();
  const [csv, setCSV] = useState("recipient_id,name\n001,Alice\n002,Bob");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  const [invalid, setInvalid] = useState(false);
  const request = useRef(crypto.randomUUID());
  const pending = useRef(false);
  const { archive, clear, receive } = useArchive();
  const changed = () => {
    request.current = crypto.randomUUID();
    clear();
    setInvalid(false);
    setError(undefined);
  };
  async function submit() {
    if (pending.current) return;
    if (!file) {
      setInvalid(true);
      return;
    }
    setError(undefined);
    pending.current = true;
    setBusy(true);
    const body = new FormData();
    body.append("file", file);
    body.append("recipients_csv", csv);
    try {
      await receive(
        await api("/platform/batches", {
          method: "POST",
          headers: { "Idempotency-Key": request.current },
          body,
        }),
      );
    } catch (e) {
      setError(e);
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.batches}</Typography.Title>
        <Typography.Text type="secondary">{t.batchHint}</Typography.Text>
      </div>
      <Card>
        <Form layout="vertical" onFinish={submit}>
          <Form.Item label={t.upload}>
            <input
              aria-label={t.batchImage}
              type="file"
              accept=".png,.jpg,.jpeg"
              disabled={busy}
              onChange={(e) => {
                changed();
                setFile(e.target.files?.[0]);
              }}
            />
          </Form.Item>
          <Form.Item label={t.recipientCSV} extra={t.csvHint}>
            <Input.TextArea
              aria-label={t.recipientCSV}
              rows={7}
              value={csv}
              disabled={busy}
              onChange={(e) => {
                changed();
                setCSV(e.target.value);
              }}
            />
            <Upload
              accept=".csv"
              showUploadList={false}
              disabled={busy}
              beforeUpload={async (selected) => {
                try {
                  if (selected.size > 16384) throw new Error();
                  const text = new TextDecoder("utf-8", { fatal: true }).decode(
                    await selected.arrayBuffer(),
                  );
                  changed();
                  setCSV(text);
                } catch {
                  setInvalid(true);
                }
                return false;
              }}
            >
              <Button style={{ marginTop: 12 }}>{t.importCSV}</Button>
            </Upload>
          </Form.Item>
          <Space direction="vertical" style={{ width: "100%" }} size={16}>
            {invalid && (
              <Alert type="error" showIcon message={t.batchValidation} />
            )}
            <ErrorNotice error={error} />
            <Button
              type="primary"
              aria-label={t.generateBatch}
              htmlType="submit"
              loading={busy}
            >
              {t.generateBatch}
            </Button>
            {busy && (
              <Typography.Text role="status">
                {t.batchProcessing}
              </Typography.Text>
            )}
            {archive && (
              <Alert
                type="success"
                message={t.batchComplete}
                description={
                  <Space direction="vertical">
                    <Typography.Text copyable>{archive.id}</Typography.Text>
                    <Button href={archive.url} download={`${archive.id}.zip`}>
                      {t.downloadZIP}
                    </Button>
                  </Space>
                }
              />
            )}
          </Space>
        </Form>
      </Card>
      <Alert type="info" showIcon message={t.batchPrivacy} />
    </Space>
  );
}
