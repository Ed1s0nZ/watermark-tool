import { useEffect, useState } from "react";
import { Alert, Button, Select, Space, Spin } from "antd";
import { api } from "./api";
import { type DeliveryJob } from "./jobClient";
import { approvalBase } from "./approvalAPI";
import { useApprovalText } from "./approvalText";
export function ApprovalPreview({
  team,
  job,
  onReceipt,
}: {
  team: string;
  job: DeliveryJob;
  onReceipt: (receipt: string) => void;
}) {
  const t = useApprovalText(),
    [selected, setSelected] = useState("source/" + job.sources[0]?.id),
    [url, setURL] = useState(""),
    [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const abort = new AbortController();
    let objectURL = "";
    setURL("");
    onReceipt("");
    setFailed(false);
    const [kind, id] = selected.split("/");
    const copy =
      kind === "copy" ? job.copies.find((c) => c.id === id) : undefined;
    const sourceID = copy?.source_id ?? id;
    void (async () => {
      try {
        const response = await api(
          approvalBase(team) +
            "/" +
            job.id +
            "/thumbnail?" +
            new URLSearchParams({
              source_id: sourceID,
              copy_id: copy?.id ?? "",
            }),
          { signal: abort.signal },
        );
        const blob = await response.blob();
        if (!abort.signal.aborted) {
          objectURL = URL.createObjectURL(blob);
          setURL(objectURL);
          onReceipt(response.headers.get("X-Review-Token") ?? "");
        }
      } catch {
        if (!abort.signal.aborted) setFailed(true);
      }
    })();
    return () => {
      abort.abort();
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [team, job.id, selected, onReceipt, attempt]);
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Select
        aria-label={t.select}
        style={{ width: "100%" }}
        value={selected}
        onChange={setSelected}
        options={[
          ...job.sources.map((s) => ({
            value: "source/" + s.id,
            label: `${t.source}: ${s.name}`,
          })),
          ...(job.mode === "dual"
            ? job.copies.map((c) => ({
                value: "copy/" + c.id,
                label: `${t.copy}: ${job.sources.find((s) => s.id === c.source_id)?.name} · ${c.name}`,
              }))
            : []),
        ]}
      />
      {failed ? (
        <Alert
          message={t.unavailable}
          type="warning"
          action={
            <Button onClick={() => setAttempt((n) => n + 1)}>
              {t.retryPreview}
            </Button>
          }
        />
      ) : url ? (
        <img
          src={url}
          alt={t.preview}
          style={{ maxWidth: 256, maxHeight: 256, objectFit: "contain" }}
        />
      ) : (
        <Spin />
      )}
    </Space>
  );
}
