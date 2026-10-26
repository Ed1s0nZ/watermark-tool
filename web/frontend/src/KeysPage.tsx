import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Popconfirm,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";
import { api } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useText } from "./i18n";
import { jsonRequest, KeyVersion } from "./platform";
export function KeysPage({ active }: { active: boolean }) {
  const t = useText();
  const [keys, setKeys] = useState<KeyVersion[]>([]);
  const [loading, setLoading] = useState(false);
  const [changing, setChanging] = useState(false);
  const [error, setError] = useState<unknown>();
  const label = (state: string) =>
    state === "active"
      ? t.keyActive
      : state === "disabled"
        ? t.keyDisabled
        : t.keyDecrypt;
  async function load() {
    setLoading(true);
    setError(undefined);
    try {
      const response = await api("/platform/keys");
      setKeys((await response.json()).items);
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    if (active) void load();
  }, [active]);
  async function create() {
    setChanging(true);
    setError(undefined);
    try {
      const response = await api("/platform/keys", { method: "POST" });
      await response.json();
      await load();
    } catch (e) {
      setError(e);
    } finally {
      setChanging(false);
    }
  }
  async function transition(id: string, state: string) {
    setChanging(true);
    setError(undefined);
    try {
      await jsonRequest(`/platform/keys/${id}/state`, { state });
      await load();
    } catch (e) {
      setError(e);
    } finally {
      setChanging(false);
    }
  }
  if (!active) return null;
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.keys}</Typography.Title>
        <Typography.Text type="secondary">{t.keyHint}</Typography.Text>
      </div>
      <Alert type="info" showIcon message={t.keyPrivacy} />
      <ErrorNotice error={error} />
      <Card>
        <Space style={{ marginBottom: 20 }}>
          <Button
            type="primary"
            loading={changing}
            aria-label={t.createKey}
            onClick={() => void create()}
          >
            {t.createKey}
          </Button>
          <Button onClick={() => void load()} loading={loading}>
            {t.retry}
          </Button>
        </Space>
        <Table<KeyVersion>
          rowKey="id"
          scroll={{ x: 700 }}
          loading={loading}
          dataSource={keys}
          pagination={false}
          columns={[
            {
              title: t.keyID,
              dataIndex: "id",
              render: (id: string) => (
                <Typography.Text code>{id}</Typography.Text>
              ),
            },
            {
              title: t.status,
              dataIndex: "state",
              render: (state: string) => (
                <Tag
                  color={
                    state === "active"
                      ? "green"
                      : state === "disabled"
                        ? "default"
                        : "blue"
                  }
                >
                  {label(state)}
                </Tag>
              ),
            },
            {
              title: t.createdAt,
              dataIndex: "created_at",
              render: (value: string) => new Date(value).toLocaleString(),
            },
            {
              title: t.actions,
              render: (_, row) => (
                <Space wrap>
                  {row.state !== "active" && (
                    <Popconfirm
                      title={t.confirmKeyChange}
                      description={t.activateWarning}
                      okText={t.confirmAction}
                      cancelText={t.close}
                      onConfirm={() => transition(row.id, "active")}
                    >
                      <Button aria-label={t.activateKey} disabled={changing}>
                        {t.activateKey}
                      </Button>
                    </Popconfirm>
                  )}
                  {row.state === "decrypt-only" && (
                    <Popconfirm
                      title={t.confirmKeyChange}
                      description={t.disableWarning}
                      okText={t.confirmAction}
                      cancelText={t.close}
                      onConfirm={() => transition(row.id, "disabled")}
                    >
                      <Button
                        aria-label={t.disableKey}
                        danger
                        disabled={changing}
                      >
                        {t.disableKey}
                      </Button>
                    </Popconfirm>
                  )}
                  {row.state === "disabled" && (
                    <Popconfirm
                      title={t.confirmKeyChange}
                      description={t.restoreWarning}
                      okText={t.confirmAction}
                      cancelText={t.close}
                      onConfirm={() => transition(row.id, "decrypt-only")}
                    >
                      <Button aria-label={t.restoreKey} disabled={changing}>
                        {t.restoreKey}
                      </Button>
                    </Popconfirm>
                  )}
                </Space>
              ),
            },
          ]}
        />
      </Card>
    </Space>
  );
}
