import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Input,
  Modal,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
} from "antd";
import { RecordOrigin } from "./RecordOrigin";
import { api } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useText } from "./i18n";
import { BatchRecord, CopyRecord, jsonRequest, useArchive } from "./platform";
import { TraceForm } from "./TraceForm";
export function RecordsPage({ active }: { active: boolean }) {
  const t = useText();
  const [tab, setTab] = useState("batches");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const [batches, setBatches] = useState<BatchRecord[]>([]);
  const [records, setRecords] = useState<CopyRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<unknown>();
  const sequence = useRef(0);
  const [rebuild, setRebuild] = useState<BatchRecord>();
  const [file, setFile] = useState<File>();
  const [busy, setBusy] = useState(false);
  const { archive, clear, receive } = useArchive();
  const stateLabel = (state: string) =>
    state === "completed"
      ? t.completed
      : state === "processing"
        ? t.processing
        : t.failed;
  async function load() {
    if (tab === "trace") return;
    const id = ++sequence.current;
    setLoading(true);
    setError(undefined);
    try {
      const data = await jsonRequest(`/platform/${tab}/search`, {
        q,
        page,
        page_size: 20,
      });
      if (id !== sequence.current) return;
      if (tab === "batches") setBatches(data.items);
      else setRecords(data.items);
      setTotal(data.total);
    } catch (e) {
      if (id === sequence.current) setError(e);
    } finally {
      if (id === sequence.current) setLoading(false);
    }
  }
  useEffect(() => {
    if (active) void load();
  }, [active, tab, page, q]);
  async function regenerate() {
    if (!rebuild || !file) return;
    setBusy(true);
    setError(undefined);
    const body = new FormData();
    body.append("file", file);
    try {
      await receive(
        await api(`/platform/batches/${rebuild.id}/download`, {
          method: "POST",
          body,
        }),
      );
      await load();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  if (!active) return null;
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.records}</Typography.Title>
        <Typography.Text type="secondary">{t.recordsHint}</Typography.Text>
      </div>
      <Tabs
        activeKey={tab}
        onChange={(value) => {
          setTab(value);
          setPage(1);
          setError(undefined);
        }}
        items={[
          { key: "batches", label: t.batchList },
          { key: "records", label: t.copyList },
          { key: "trace", label: t.traceTab },
        ]}
      />
      {tab === "trace" ? (
        <TraceForm active={active} />
      ) : (
        <Card>
          <Space style={{ marginBottom: 20 }} wrap>
            <Input.Search
              aria-label={t.searchRecords}
              placeholder={t.searchRecords}
              allowClear
              onSearch={(value) => {
                setQ(value);
                setPage(1);
              }}
              enterButton
            />
            <Button loading={loading} onClick={() => void load()}>
              {t.retry}
            </Button>
          </Space>
          <ErrorNotice error={error} />
          {tab === "batches" ? (
            <Table<BatchRecord>
              rowKey="id"
              scroll={{ x: 950 }}
              dataSource={batches}
              loading={loading}
              pagination={{
                current: page,
                pageSize: 20,
                total,
                showSizeChanger: false,
                onChange: setPage,
              }}
              expandable={{
                expandedRowRender: (b) => (
                  <Space direction="vertical" style={{ width: "100%" }}>
                    <Typography.Text>
                      {t.sourceDigest}: {b.source_sha256}
                    </Typography.Text>
                    {b.error && <Alert type="warning" message={b.error} />}
                    <Table
                      rowKey="id"
                      dataSource={b.copies}
                      pagination={false}
                      columns={[
                        { title: t.recipientID, dataIndex: "recipient_id" },
                        { title: t.registeredRecipient, dataIndex: "name" },
                        { title: t.watermarkID, dataIndex: "id" },
                      ]}
                    />
                  </Space>
                ),
              }}
              columns={[
                {
                  title: t.batchID,
                  dataIndex: "id",
                  render: (id: string) => (
                    <Typography.Text code copyable>
                      {id}
                    </Typography.Text>
                  ),
                },
                {
                  title: t.status,
                  dataIndex: "status",
                  render: (state: string) => (
                    <Tag
                      color={
                        state === "completed"
                          ? "green"
                          : state === "failed"
                            ? "red"
                            : "blue"
                      }
                    >
                      {stateLabel(state)}
                    </Tag>
                  ),
                },
                { title: t.copies, render: (_, b) => b.copies.length },
                { title: t.keyID, dataIndex: "key_id" },
                {
                  title: t.createdAt,
                  dataIndex: "created_at",
                  render: (v: string) => new Date(v).toLocaleString(),
                },
                {
                  title: t.actions,
                  render: (_, b) => (
                    <Button
                      disabled={b.status === "processing"}
                      onClick={() => {
                        setRebuild(b);
                        setFile(undefined);
                        clear();
                        setError(undefined);
                      }}
                    >
                      {t.rebuildZIP}
                    </Button>
                  ),
                },
              ]}
            />
          ) : (
            <Table<CopyRecord>
              rowKey="id"
              scroll={{ x: 1050 }}
              dataSource={records}
              loading={loading}
              pagination={{
                current: page,
                pageSize: 20,
                total,
                showSizeChanger: false,
                onChange: setPage,
              }}
              expandable={{
                expandedRowRender: (r) => <RecordOrigin record={r} />,
              }}
              columns={[
                {
                  title: t.watermarkID,
                  dataIndex: "id",
                  render: (id: string) => (
                    <Typography.Text code copyable>
                      {id}
                    </Typography.Text>
                  ),
                },
                { title: t.recipientID, dataIndex: "recipient_id" },
                { title: t.registeredRecipient, dataIndex: "name" },
                {
                  title: `${t.batchID} / ${t.taskID}`,
                  render: (_, r) => r.job_id || r.batch_id,
                },
                { title: t.sourceName, dataIndex: "source_name" },
                { title: t.keyID, dataIndex: "key_id" },
                {
                  title: t.status,
                  render: (_, r) => (
                    <Tag color={r.generated ? "green" : "orange"}>
                      {r.generated ? t.generatedCopy : t.pendingCopy}
                    </Tag>
                  ),
                },
              ]}
            />
          )}
        </Card>
      )}
      <Modal
        title={t.rebuildZIP}
        open={!!rebuild}
        onCancel={() => {
          if (!busy) setRebuild(undefined);
        }}
        footer={null}
        destroyOnHidden
      >
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Alert showIcon type="info" message={t.rebuildHint} />
          <input
            aria-label={t.originalImage}
            type="file"
            disabled={busy}
            accept=".png,.jpg,.jpeg"
            onChange={(e) => setFile(e.target.files?.[0])}
          />
          <ErrorNotice error={error} />
          <Button
            type="primary"
            loading={busy}
            disabled={!file}
            onClick={() => void regenerate()}
          >
            {t.rebuildZIP}
          </Button>
          {archive && (
            <Button href={archive.url} download={`${archive.id}.zip`}>
              {t.downloadZIP}
            </Button>
          )}
        </Space>
      </Modal>
    </Space>
  );
}
