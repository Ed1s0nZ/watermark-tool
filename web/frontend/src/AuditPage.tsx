import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Form,
  Input,
  Space,
  Table,
  Typography,
} from "antd";
import { api } from "./api";
import { post, saveBlob } from "./jobClient";
import { useJobText } from "./jobText";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
interface Event {
  sequence: number;
  id: string;
  at: string;
  actor: string;
  action: string;
  target?: string;
  outcome: string;
  code?: string;
}
interface Filter {
  from?: string;
  to?: string;
  actor?: string;
  action?: string;
  outcome?: string;
}
export function AuditPage() {
  const t = useJobText();
  const [draft, setDraft] = useState<Filter>({});
  const [filter, setFilter] = useState<Filter>({});
  const [page, setPage] = useState(1);
  const [configured, setConfigured] = useState<boolean>();
  const rows = useJob<{ items: Event[]; total: number; degraded: boolean }>();
  const exporting = useJob<null>();
  useEffect(() => {
    const c = new AbortController();
    void api("/audit/status", { signal: c.signal })
      .then((r) => r.json())
      .then((v) => {
        if (!c.signal.aborted) setConfigured(v.configured);
      })
      .catch(() => {});
    return () => c.abort();
  }, []);
  useEffect(() => {
    if (configured === false) return;
    rows.reset();
    void rows.run((signal) =>
      post("/audit/search", { ...filter, page, page_size: 20 }, signal),
    );
  }, [filter, page, configured]);
  function download(format: "json" | "csv") {
    void exporting.run(async (signal) => {
      const response = await api("/audit/export", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...filter, format }),
        signal,
      });
      const blob = await response.blob();
      if (!signal.aborted) saveBlob(blob, "audit." + format);
      return null;
    });
  }
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.audit}</Typography.Title>
        <Typography.Text type="secondary">{t.auditHint}</Typography.Text>
      </div>
      {configured === false ? (
        <Alert message={t.noAudit} />
      ) : (
        <>
          <Card>
            <Form
              layout="vertical"
              onFinish={() => {
                setPage(1);
                setFilter({ ...draft });
              }}
            >
              <Form.Item label={t.auditAt}>
                <DatePicker.RangePicker
                  style={{ width: 280, maxWidth: "100%" }}
                  onChange={(values) =>
                    setDraft({
                      ...draft,
                      from: values?.[0]?.startOf("day").toISOString(),
                      to: values?.[1]?.endOf("day").toISOString().replace(".999Z",".999999999Z"),
                    })
                  }
                />
              </Form.Item>
              <Space wrap style={{ width: "100%" }}>
                <Form.Item label={t.actor}>
                  <Input
                    aria-label={t.actor}
                    value={draft.actor}
                    onChange={(e) =>
                      setDraft({ ...draft, actor: e.target.value })
                    }
                  />
                </Form.Item>
                <Form.Item label={t.action}>
                  <Input
                    aria-label={t.action}
                    value={draft.action}
                    placeholder="task.create"
                    onChange={(e) =>
                      setDraft({ ...draft, action: e.target.value })
                    }
                  />
                </Form.Item>
                <Form.Item label={t.outcome}>
                  <Input
                    aria-label={t.outcome}
                    value={draft.outcome}
                    placeholder="completed"
                    onChange={(e) =>
                      setDraft({ ...draft, outcome: e.target.value })
                    }
                  />
                </Form.Item>
              </Space>
              <Space wrap>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={rows.busy}
                  aria-label={t.filter}
                >
                  {t.filter}
                </Button>
                <Button
                  disabled={!rows.value}
                  loading={exporting.busy}
                  onClick={() => download("json")}
                >
                  {t.exportJSON}
                </Button>
                <Button
                  disabled={!rows.value}
                  loading={exporting.busy}
                  onClick={() => download("csv")}
                >
                  {t.exportCSV}
                </Button>
              </Space>
            </Form>
            <ErrorNotice error={exporting.error} />
          </Card>
          <ErrorNotice error={rows.error} />
          {rows.value?.degraded && <Alert type="error" message={t.degraded} />}
          <Card>
            <Table<Event>
              rowKey="id"
              dataSource={rows.value?.items}
              loading={rows.busy}
              scroll={{ x: 1100 }}
              pagination={{
                current: page,
                pageSize: 20,
                showSizeChanger: false,
                total: rows.value?.total ?? 0,
                onChange: setPage,
              }}
              columns={[
                { title: t.sequence, dataIndex: "sequence", width: 80 },
                { title: t.auditAt, dataIndex: "at" },
                {
                  title: t.actor,
                  dataIndex: "actor",
                  render: (value) => (
                    <Typography.Text style={{ overflowWrap: "anywhere" }}>
                      {value}
                    </Typography.Text>
                  ),
                },
                { title: t.action, dataIndex: "action" },
                {
                  title: t.target,
                  dataIndex: "target",
                  render: (value) => (
                    <Typography.Text style={{ overflowWrap: "anywhere" }}>
                      {value || "—"}
                    </Typography.Text>
                  ),
                },
                { title: t.outcome, dataIndex: "outcome" },
                { title: t.error, dataIndex: "code" },
              ]}
            />
          </Card>
        </>
      )}
    </Space>
  );
}
