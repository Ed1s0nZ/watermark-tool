import { useEffect, useState } from "react";
import { Alert, Button, Card, Input, Select, Space, Typography } from "antd";
import {
  builtins,
  loadPolicies,
  savePolicies,
  PolicyValue,
  Preset,
} from "./policies";
import { useArtText } from "./artText";
import { useFeatureText } from "./traceText";
export function PolicyControls({
  value,
  onApply,
  disabled = false,
}: {
  value: PolicyValue;
  onApply: (v: PolicyValue) => void;
  disabled?: boolean;
}) {
  const t = useFeatureText();
  const at = useArtText();
  const [custom, setCustom] = useState<Preset[]>([]),
    [selected, setSelected] = useState<string>(),
    [name, setName] = useState(""),
    [error, setError] = useState(false),
    [applied, setApplied] = useState(false);
  const defaults = builtins([t.internal, t.preview, t.public]);
  const all = [...defaults, ...custom];
  const chosen = all.find((p) => p.id === selected);
  const editable = custom.some((p) => p.id === selected);
  useEffect(() => {
    try {
      setCustom(loadPolicies());
    } catch {
      setError(true);
    }
  }, []);
  function persist(next: Preset[]) {
    try {
      savePolicies(next);
      setCustom(next);
      setError(false);
      return true;
    } catch {
      setError(true);
      return false;
    }
  }
  return (
    <Card size="small" title={t.policies}>
      <Space direction="vertical" style={{ width: "100%" }}>
        <Typography.Text type="secondary">
          {t.policyHint} {at.newPolicyHint}
        </Typography.Text>
        <Space wrap>
          <Select
            aria-label={t.select}
            placeholder={t.select}
            value={selected}
            disabled={disabled}
            style={{ width: 220 }}
            onChange={(v) => {
              setSelected(v);
              setApplied(false);
            }}
            options={all.map((p) => ({ value: p.id, label: p.name }))}
          />
          <Button
            disabled={disabled || !chosen}
            onClick={() => {
              if (!chosen) return;
              const { id, name, ...p } = chosen;
              onApply(structuredClone(p));
              setApplied(true);
            }}
          >
            {t.apply}
          </Button>
        </Space>
        <Input
          aria-label={t.policyName}
          placeholder={t.policyName}
          maxLength={40}
          value={name}
          disabled={disabled}
          onChange={(e) => setName(e.target.value)}
        />
        <Space wrap>
          <Button
            disabled={disabled || !name.trim()}
            onClick={() => {
              const id = crypto.randomUUID();
              if (
                persist([
                  ...custom,
                  { ...structuredClone(value), id, name: name.trim() },
                ])
              ) {
                setSelected(id);
                setName("");
              }
            }}
          >
            {t.save}
          </Button>
          <Button
            disabled={disabled || !editable}
            onClick={() =>
              persist(
                custom.map((p) =>
                  p.id === selected
                    ? { ...structuredClone(value), id: p.id, name: p.name }
                    : p,
                ),
              )
            }
          >
            {t.update}
          </Button>
          <Button
            disabled={disabled || !editable || !name.trim()}
            onClick={() => {
              if (
                persist(
                  custom.map((p) =>
                    p.id === selected ? { ...p, name: name.trim() } : p,
                  ),
                )
              )
                setName("");
            }}
          >
            {t.rename}
          </Button>
          <Button
            disabled={disabled || !editable}
            onClick={() => {
              if (persist(custom.filter((p) => p.id !== selected)))
                setSelected(undefined);
            }}
          >
            {t.remove}
          </Button>
          <Button
            disabled={disabled}
            onClick={() => {
              setSelected(defaults[0].id);
              setApplied(false);
            }}
          >
            {t.restore}
          </Button>
        </Space>
        {error && <Alert role="alert" type="error" message={t.storage} />}{" "}
        {applied && <Alert type="info" message={t.applied} />}
      </Space>
    </Card>
  );
}
