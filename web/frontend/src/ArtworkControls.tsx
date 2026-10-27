import { useEffect, useRef } from "react";
import { Form, InputNumber, Select, Space, Typography } from "antd";
import { ArtworkState, positions } from "./artwork";
import { useArtText } from "./artText";
export function ArtworkControls({
  value,
  onChange,
  disabled = false,
}: {
  value: ArtworkState;
  onChange: (v: ArtworkState) => void;
  disabled?: boolean;
}) {
  const t = useArtText();
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (!value.logoFile && input.current) input.current.value = "";
  }, [value.logoFile]);
  return (
    <>
      <Form.Item label={t.layers}>
        <Select
          aria-label={t.layers}
          value={value.layers}
          disabled={disabled}
          onChange={(layers) => onChange({ ...value, layers })}
          options={[
            { value: "text", label: t.text },
            { value: "logo", label: t.logo },
            { value: "both", label: t.both },
          ]}
        />
      </Form.Item>
      {value.layers !== "text" && (
        <>
          <Form.Item label={t.logoFile} extra={t.logoHint}>
            <input
              ref={input}
              aria-label={t.logoFile}
              type="file"
              accept=".png,.jpg,.jpeg"
              disabled={disabled}
              onChange={(e) =>
                onChange({ ...value, logoFile: e.target.files?.[0] })
              }
            />
          </Form.Item>
          {value.expectedSHA && (
            <Typography.Paragraph style={{ overflowWrap: "anywhere" }}>
              {t.expected}: {value.expectedSHA}
            </Typography.Paragraph>
          )}
          <Space wrap>
            <Form.Item label={t.logoPosition}>
              <Select
                aria-label={t.logoPosition}
                value={value.logo.position}
                disabled={disabled}
                style={{ width: 130 }}
                onChange={(position) =>
                  onChange({ ...value, logo: { ...value.logo, position } })
                }
                options={positions.map((p) => ({ value: p, label: t[p] }))}
              />
            </Form.Item>
            <Form.Item label={t.ratio}>
              <InputNumber
                aria-label={t.ratio}
                value={value.logo.width_ratio}
                min={0.05}
                max={0.4}
                step={0.01}
                disabled={disabled}
                onChange={(v) => {
                  if (v !== null)
                    onChange({
                      ...value,
                      logo: { ...value.logo, width_ratio: v },
                    });
                }}
              />
            </Form.Item>
            <Form.Item label={t.opacity}>
              <InputNumber
                aria-label={t.opacity}
                value={value.logo.opacity}
                min={0.05}
                max={1}
                step={0.05}
                disabled={disabled}
                onChange={(v) => {
                  if (v !== null)
                    onChange({ ...value, logo: { ...value.logo, opacity: v } });
                }}
              />
            </Form.Item>
          </Space>
        </>
      )}
    </>
  );
}
