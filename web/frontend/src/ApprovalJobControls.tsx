import { useEffect, useState } from "react";
import { Alert, Button, Card, Space, Tag, Typography } from "antd";
import { api } from "./api";
import { type DeliveryJob, post } from "./jobClient";
import { useWorkspace } from "./workspaceContext";
import { useApprovalText } from "./approvalText";
import { approvalBase, type Approval } from "./approvalAPI";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
export function ApprovalJobControls({
  job,
  onUpdate,
}: {
  job: DeliveryJob;
  onUpdate: (job: DeliveryJob) => void;
}) {
  const context = useWorkspace(),
    t = useApprovalText(),
    query = useJob<{ approval: Approval | null }>(),
    action = useJob<null>();
  const [revision, setRevision] = useState(0);
  const base = approvalBase(context.team!.id) + "/" + job.id;
  useEffect(() => {
    let stopped = false;
    const refresh = () => {
      if (!stopped)
        void query.run(async (signal) => (await api(base, { signal })).json());
    };
    refresh();
    const timer = setInterval(refresh, 5000);
    return () => {
      stopped = true;
      clearInterval(timer);
      query.reset();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [base, revision]);
  const approval = query.value?.approval,
    uploaded =
      job.sources.every((s) => s.uploaded) &&
      (job.mode !== "dual" || job.copies.every((c) => c.input_uploaded));
  return (
    <Card size="small" title={t.required}>
      <Space direction="vertical" style={{ width: "100%" }}>
        <Alert message={t.ready} />
        <Tag>{approval ? t[approval.status] : t.none}</Tag>
        {approval && (
          <Typography.Text copyable style={{ overflowWrap: "anywhere" }}>
            {t.snapshot}: {approval.snapshot}
          </Typography.Text>
        )}
        <Space wrap>
          <Button
            aria-label={t.refresh}
            disabled={query.busy || action.busy}
            onClick={() => setRevision((n) => n + 1)}
          >
            {t.refresh}
          </Button>
          <Button
            aria-label={t.request}
            disabled={
              action.busy ||
              query.busy ||
              !uploaded ||
              job.generated ||
              job.operator !== context.username ||
              approval?.status === "pending" ||
              approval?.status === "approved"
            }
            onClick={() =>
              void action.run(async (signal) => {
                await post(
                  base + "/request",
                  { revision: approval?.revision ?? 0 },
                  signal,
                );
                if (!signal.aborted) setRevision((n) => n + 1);
                return null;
              })
            }
          >
            {t.request}
          </Button>
          <Button
            aria-label={t.start}
            type="primary"
            disabled={
              action.busy ||
              approval?.status !== "approved" ||
              job.generated ||
              !["preparing", "failed", "cancelled", "interrupted"].includes(
                job.status,
              )
            }
            onClick={() =>
              void action.run(async (signal) => {
                const result = await post<DeliveryJob>(
                  "/jobs/" +
                    job.id +
                    (job.status === "preparing" ? "/start" : "/retry"),
                  {},
                  signal,
                );
                if (!signal.aborted) onUpdate(result);
                return null;
              })
            }
          >
            {t.start}
          </Button>
        </Space>
        <ErrorNotice error={query.error} />
        <ErrorNotice error={action.error} />
      </Space>
    </Card>
  );
}
