import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Input,
  Popconfirm,
  Select,
  Space,
  Typography,
} from "antd";
import { api, APIError } from "./api";
import { useWorkspace } from "./workspaceContext";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
import { contactJSON } from "./recipientAPI";
import { validPolicy, type PolicyValue } from "./policies";
import { useSharedPolicyText } from "./sharedPolicyText";
interface PolicyVersion {
  number: number;
  content: PolicyValue;
  sha256: string;
  actor: string;
  created_at: string;
}
interface SharedPolicy {
  id: string;
  team_id: string;
  name: string;
  revision: number;
  published: number;
  versions: PolicyVersion[];
}
export function SharedPolicyControls({
  value,
  onApply,
  disabled = false,
}: {
  value: PolicyValue;
  onApply: (value: PolicyValue) => void;
  disabled?: boolean;
}) {
  const context = useWorkspace(),
    t = useSharedPolicyText(),
    query = useJob<{ items: SharedPolicy[] }>(),
    action = useJob<null>();
  const [revision, setRevision] = useState(0),
    [selected, setSelected] = useState<string>(),
    [version, setVersion] = useState<number>(),
    [name, setName] = useState(""),
    [applied, setApplied] = useState(false);
  const team = context.team?.id,
    base = `/workspace/teams/${encodeURIComponent(team ?? "")}/policies`;
  useEffect(() => {
    if (!context.available || !team) return;
    query.reset();
    void query.run(async (signal) => (await api(base, { signal })).json());
    return query.reset;
    // A team switch remounts the workspace. Refreshes explicitly replace stale reads.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [team, context.available, revision]);
  if (!context.available || !team) return null;
  const owner = context.administrator || context.role === "owner",
    busy = disabled || action.busy;
  const chosen = query.value?.items.find((p) => p.id === selected),
    chosenVersion = chosen?.versions.find((v) => v.number === version);
  function select(p: SharedPolicy) {
    setSelected(p.id);
    setVersion(p.published || p.versions.at(-1)?.number);
    setName(p.name);
    setApplied(false);
  }
  async function save(create: boolean) {
    await action.run(async (signal) => {
      if (!validPolicy(value)) throw new APIError(422, "workspace_invalid");
      const saved = await contactJSON<SharedPolicy>(
        base + (create ? "" : "/" + chosen!.id),
        {
          name: name.trim(),
          revision: create ? 0 : chosen!.revision,
          content: value,
        },
        signal,
        create ? "POST" : "PUT",
      );
      if (!signal.aborted) {
        setSelected(saved.id);
        setVersion(saved.versions.at(-1)?.number);
        setName(saved.name);
        setRevision((n) => n + 1);
        setApplied(false);
      }
      return null;
    });
  }
  return (
    <Card size="small" title={t.title}>
      <Space direction="vertical" style={{ width: "100%" }}>
        <Typography.Text type="secondary">{t.hint}</Typography.Text>
        <Space wrap>
          <Select
            aria-label={t.select}
            placeholder={t.select}
            style={{ width: 220 }}
            loading={query.busy}
            disabled={busy}
            value={selected}
            options={(query.value?.items ?? []).map((p) => ({
              value: p.id,
              label: p.name,
            }))}
            onChange={(id) => {
              const p = query.value?.items.find((p) => p.id === id);
              if (p) select(p);
            }}
          />
          <Select
            aria-label={t.version}
            placeholder={t.version}
            style={{ width: 200 }}
            disabled={busy || !chosen}
            value={version}
            options={(chosen?.versions ?? []).map((v) => ({
              value: v.number,
              label: `v${v.number} · ${chosen?.published === v.number ? t.published : t.draft}`,
            }))}
            onChange={(v) => {
              setVersion(v);
              setApplied(false);
            }}
          />
          <Button
            aria-label={t.refresh}
            disabled={busy || query.busy}
            onClick={() => setRevision((n) => n + 1)}
          >
            {t.refresh}
          </Button>
          <Button
            aria-label={t.apply}
            disabled={busy || query.busy || !chosenVersion}
            onClick={() => {
              if (chosenVersion && validPolicy(chosenVersion.content)) {
                onApply(structuredClone(chosenVersion.content));
                setApplied(true);
              } else
                void action.run(async () => {
                  throw new APIError(422, "workspace_invalid");
                });
            }}
          >
            {t.apply}
          </Button>
        </Space>
        {chosenVersion && (
          <Typography.Text type="secondary">
            v{chosenVersion.number} · {chosenVersion.actor} ·{" "}
            {new Date(chosenVersion.created_at).toLocaleString()} · SHA-256{" "}
            {chosenVersion.sha256.slice(0, 12)}
          </Typography.Text>
        )}
        {owner && (
          <>
            <Input
              aria-label={t.name}
              placeholder={t.name}
              value={name}
              maxLength={120}
              disabled={busy}
              onChange={(e) => setName(e.target.value)}
            />
            <Space wrap>
              <Button
                aria-label={t.create}
                disabled={busy || !name.trim()}
                onClick={() => void save(true)}
              >
                {t.create}
              </Button>
              <Button
                aria-label={t.append}
                disabled={busy || query.busy || !chosen || !name.trim()}
                onClick={() => void save(false)}
              >
                {t.append}
              </Button>
              <Popconfirm
                title={t.confirm}
                okText={t.publish}
                cancelText={t.cancel}
                onConfirm={() =>
                  action.run(async (signal) => {
                    await contactJSON(
                      base + "/" + chosen!.id + "/publish",
                      { revision: chosen!.revision, version },
                      signal,
                    );
                    if (!signal.aborted) setRevision((n) => n + 1);
                    return null;
                  })
                }
              >
                <Button
                  aria-label={t.publish}
                  disabled={
                    busy ||
                    query.busy ||
                    !chosenVersion ||
                    chosen?.published === version
                  }
                >
                  {t.publish}
                </Button>
              </Popconfirm>
            </Space>
          </>
        )}
        <ErrorNotice error={query.error} />
        <ErrorNotice error={action.error} />
        {applied && <Alert type="success" message={t.applied} />}{" "}
        {action.value === null && !applied && (
          <Alert type="success" message={t.success} />
        )}
      </Space>
    </Card>
  );
}
