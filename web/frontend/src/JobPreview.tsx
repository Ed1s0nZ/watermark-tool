import { useEffect, useState } from "react";
import { Alert, Image, Select, Space, Typography } from "antd";
import { Plan, substitute } from "./jobClient";
import { useWorkbenchText } from "./workbenchText";
import { useJobText } from "./jobText";
export function JobPreview({ plan }: { plan: Plan }) {
  const t = useJobText();
  const wt = useWorkbenchText();
  const [source, setSource] = useState(0);
  const [recipient, setRecipient] = useState(0);
  const [url, setURL] = useState("");
  useEffect(() => {
    setSource(0);
    setRecipient(0);
  }, [plan]);
  const src = plan.sources[source] ?? plan.sources[0];
  const person = plan.recipients[recipient] ?? plan.recipients[0];
  const blob =
    plan.mode === "dual"
      ? plan.renders.get(src.source_sha256 + "/" + person.recipient_id)
      : src.file;
  useEffect(() => {
    if (!blob) {
      setURL("");
      return;
    }
    const next = URL.createObjectURL(blob);
    setURL(next);
    return () => URL.revokeObjectURL(next);
  }, [blob]);
  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      {plan.mode === "dual" && <Alert type="info" message={wt.systemFont} />}
      <Space wrap>
        <Select
          getPopupContainer={(node) => node.parentElement!}
          aria-label={t.source}
          style={{ width: 240, maxWidth: "100%" }}
          value={source}
          onChange={setSource}
          options={plan.sources.map((s, i) => ({ value: i, label: s.name }))}
        />
        <Select
          getPopupContainer={(node) => node.parentElement!}
          aria-label={t.recipient}
          style={{ width: 240, maxWidth: "100%" }}
          value={recipient}
          onChange={setRecipient}
          options={plan.recipients.map((r, i) => ({
            value: i,
            label: `${r.recipient_id} · ${r.name}`,
          }))}
        />
      </Space>
      {plan.mode === "dual" && (
        <Typography.Text>{substitute(plan.template, person)}</Typography.Text>
      )}
      {url && (
        <Image
          src={url}
          alt={t.preview}
          style={{ maxWidth: "100%", maxHeight: 420, objectFit: "contain" }}
        />
      )}
    </Space>
  );
}
