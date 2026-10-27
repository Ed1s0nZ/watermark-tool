import { useRecipientText } from "./recipientText";
import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Timeline,
  Typography,
} from "antd";
import { api } from "./api";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
import { useWorkspace } from "./workspaceContext";
import { approvalBase, type Approval } from "./approvalAPI";
import { useApprovalText } from "./approvalText";
import { type DeliveryJob, post } from "./jobClient";
import { ApprovalPreview } from "./ApprovalPreview";
export function ApprovalsPage() {
  const recipientText = useRecipientText();
  const context = useWorkspace(),
    t = useApprovalText(),
    query = useJob<{ items: Approval[]; total: number }>(),
    detail = useJob<DeliveryJob>(),
    action = useJob<null>();
  const [page, setPage] = useState(1),
    [status, setStatus] = useState("pending"),
    [revision, setRevision] = useState(0),
    [selected, setSelected] = useState<Approval>(),
    [reason, setReason] = useState("");
  const [receipt, setReceipt] = useState("");
  const team = context.team?.id,
    base = approvalBase(team ?? "");
  useEffect(() => {
    if (!context.available || !team) return;
    query.reset();
    void query.run((signal) =>
      post(base + "/search", { page, page_size: 20, status }, signal),
    );
    return query.reset;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [team, page, status, revision]);
  if (!context.available || !team) return <Alert message={t.hint} />;
  const canApprove =
    selected &&
    selected.status === "pending" &&
    selected.operator !== context.username &&
    (context.administrator ||
      context.role === "owner" ||
      context.role === "reviewer");
  const canWithdraw =
    selected &&
    selected.status !== "withdrawn" &&
    (selected.requester === context.username ||
      selected.reviewer === context.username ||
      context.administrator ||
      context.role === "owner");
  async function decide(status: string) {
    await action.run(async (signal) => {
      await post(
        base + "/" + selected!.job_id + "/decision",
        {
          revision: selected!.revision,
          status,
          reason,
          review_token: status === "approved" ? receipt : undefined,
        },
        signal,
      );
      if (!signal.aborted) {
        setSelected(undefined);
        setRevision((n) => n + 1);
      }
      return null;
    });
  }
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.title}</Typography.Title>
        <Typography.Text type="secondary">{t.hint}</Typography.Text>
      </div>
      <Card>
        <Space wrap style={{ marginBottom: 16 }}>
          <Select
            aria-label={t.filter}
            style={{ minWidth: 180 }}
            value={status}
            onChange={(value) => {
              setStatus(value);
              setPage(1);
            }}
            options={[
              { value: "", label: t.all },
              ...(
                ["pending", "approved", "rejected", "withdrawn"] as const
              ).map((value) => ({ value, label: t[value] })),
            ]}
          />
          <Button
            aria-label={t.refresh}
            onClick={() => setRevision((n) => n + 1)}
          >
            {t.refresh}
          </Button>
        </Space>
        <ErrorNotice error={query.error} />
        <Table<Approval>
          rowKey="job_id"
          scroll={{ x: 650 }}
          loading={query.busy}
          dataSource={query.value?.items ?? []}
          pagination={{
            current: page,
            pageSize: 20,
            total: query.value?.total ?? 0,
            showSizeChanger: false,
            onChange: setPage,
          }}
          columns={[
            { title: t.job, dataIndex: "job_id", ellipsis: true },
            { title: t.requester, dataIndex: "requester" },
            { title: t.status, render: (_, row) => <Tag>{t[row.status]}</Tag> },
            { title: t.reviewer, dataIndex: "reviewer" },
            {
              title: t.actions,
              render: (_, row) => (
                <Button
                  aria-label={t.open}
                  onClick={() => {
                    setSelected(row);
                    setReceipt("");
                    setReason("");
                    action.reset();
                    detail.reset();
                    void detail.run(async (signal) =>
                      (await api("/jobs/" + row.job_id, { signal })).json(),
                    );
                  }}
                >
                  {t.open}
                </Button>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        title={t.open}
        open={!!selected}
        width={800}
        destroyOnHidden
        footer={null}
        onCancel={() => {
          if (!action.busy) {
            setSelected(undefined);
            detail.reset();
          }
        }}
      >
        {selected && (
          <Space direction="vertical" style={{ width: "100%" }}>
            <Descriptions
              column={1}
              items={[
                { key: "id", label: t.job, children: selected.job_id },
                {
                  key: "requester",
                  label: t.requester,
                  children: selected.requester,
                },
                {
                  key: "snapshot",
                  label: t.snapshot,
                  children: (
                    <Typography.Text
                      copyable
                      style={{ overflowWrap: "anywhere" }}
                    >
                      {selected.snapshot}
                    </Typography.Text>
                  ),
                },
              ]}
            />
            <ErrorNotice error={detail.error} />
            {detail.value && (
              <>
                <Typography.Text>
                  {detail.value.mode} · {detail.value.sources.length} ×{" "}
                  {new Set(detail.value.copies.map((c) => c.recipient_id)).size}
                </Typography.Text>
                <Descriptions
                  column={1}
                  size="small"
                  items={[
                    { key: "mode", label: t.mode, children: detail.value.mode },
                    {
                      key: "template",
                      label: t.template,
                      children: detail.value.template_text ?? "—",
                    },
                    {
                      key: "output",
                      label: t.output,
                      children:
                        detail.value.output?.format === "jpeg"
                          ? `JPEG (${detail.value.output.jpeg_quality ?? 90})`
                          : "PNG",
                    },
                    {
                      key: "quality",
                      label: t.quality,
                      children:
                        detail.value.quality_policy?.minimum_psnr_db ===
                        undefined
                          ? "—"
                          : `${detail.value.quality_policy.minimum_psnr_db} dB`,
                    },
                    ...(detail.value.style
                      ? [
                          {
                            key: "style",
                            label: t.style,
                            children: `${detail.value.style.mode} · ${detail.value.style.position} · ${detail.value.style.size}px · ${Math.round(detail.value.style.opacity * 100)}% · ${detail.value.style.color} · ${detail.value.style.angle}°`,
                          },
                        ]
                      : []),
                  ]}
                />

                <ApprovalPreview
                  key={selected.job_id}
                  team={team}
                  job={detail.value}
                  onReceipt={setReceipt}
                />
                <Table
                  rowKey="id"
                  size="small"
                  dataSource={detail.value.copies}
                  pagination={{ pageSize: 5 }}
                  scroll={{ x: 450 }}
                  columns={[
                    { title: t.job, dataIndex: "id", ellipsis: true },
                    { title: recipientText.id, dataIndex: "recipient_id" },
                    { title: recipientText.name, dataIndex: "name" },
                  ]}
                />
              </>
            )}
            <Timeline
              items={selected.changes.map((change) => ({
                children: (
                  <>
                    <strong>{t[change.status]}</strong> · {change.actor} ·{" "}
                    {new Date(change.at).toLocaleString()}
                    <p>{change.reason}</p>
                  </>
                ),
              }))}
            />
            {selected.operator === context.username && (
              <Alert message={t.noSelf} />
            )}
            <Input.TextArea
              aria-label={t.reason}
              placeholder={t.reason}
              value={reason}
              maxLength={500}
              disabled={action.busy}
              onChange={(e) => setReason(e.target.value)}
            />
            <Space wrap>
              {canApprove &&
                (["approved", "rejected"] as const).map((status) => (
                  <Popconfirm
                    key={status}
                    title={t.decisionConfirm}
                    okText={t[status]}
                    cancelText={t.close}
                    onConfirm={() => decide(status)}
                  >
                    <Button
                      aria-label={status === "approved" ? t.approve : t.reject}
                      disabled={
                        action.busy ||
                        detail.busy ||
                        !detail.value ||
                        (status === "approved" && !receipt)
                      }
                    >
                      {status === "approved" ? t.approve : t.reject}
                    </Button>
                  </Popconfirm>
                ))}
              {canWithdraw && (
                <Button
                  aria-label={t.withdraw}
                  disabled={action.busy}
                  onClick={() => void decide("withdrawn")}
                >
                  {t.withdraw}
                </Button>
              )}
            </Space>
            <ErrorNotice error={action.error} />
          </Space>
        )}
      </Modal>
    </Space>
  );
}
