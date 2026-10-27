import { Form, Select, InputNumber, Space } from "antd";
import { useArtText } from "./artText";
export interface OutputOptions {
  format: "png" | "jpeg";
  jpeg_quality?: number;
}
export function normalizedOutput(p?: OutputOptions): OutputOptions {
  return p?.format === "jpeg"
    ? { format: "jpeg", jpeg_quality: p.jpeg_quality ?? 90 }
    : { format: "png" };
}
export function validOutput(p: unknown): p is OutputOptions {
  if (!p || typeof p !== "object" || Array.isArray(p)) return false;
  const v = p as OutputOptions;
  return (
    Object.keys(v).every((k) => ["format", "jpeg_quality"].includes(k)) &&
    ((v.format === "png" && v.jpeg_quality === undefined) ||
      (v.format === "jpeg" &&
        (v.jpeg_quality === undefined ||
          (Number.isInteger(v.jpeg_quality) &&
            v.jpeg_quality >= 60 &&
            v.jpeg_quality <= 100))))
  );
}
export function OutputFields({
  value,
  onChange,
  disabled = false,
}: {
  value: OutputOptions;
  onChange: (p: OutputOptions) => void;
  disabled?: boolean;
}) {
  const t = useArtText();
  return (
    <Form.Item label={t.output} extra={t.outputHint}>
      <Space wrap>
        <Select
          aria-label={t.output}
          value={value.format}
          disabled={disabled}
          style={{ width: 150 }}
          options={[
            { value: "png", label: "PNG" },
            { value: "jpeg", label: "JPEG" },
          ]}
          onChange={(v) =>
            onChange(
              v === "png"
                ? { format: "png" }
                : { format: "jpeg", jpeg_quality: 90 },
            )
          }
        />
        {value.format === "jpeg" && (
          <InputNumber
            aria-label={t.jpegQuality}
            min={60}
            max={100}
            precision={0}
            value={value.jpeg_quality ?? 90}
            disabled={disabled}
            onChange={(v) => {
              if (v !== null) onChange({ ...value, jpeg_quality: v });
            }}
          />
        )}
      </Space>
    </Form.Item>
  );
}
