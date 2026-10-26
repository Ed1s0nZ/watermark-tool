import { Form, Select, InputNumber, Space } from "antd";
import { Style } from "./visibleWatermark";
import { useWorkbenchText } from "./workbenchText";
export function VisibleStyleFields({
  style,
  busy,
  onChange,
}: {
  style: Style;
  busy: boolean;
  onChange: (style: Partial<Style>) => void;
}) {
  const t = useWorkbenchText();
  const changeStyle = onChange;
  return (
    <Space wrap align="start" style={{ width: "100%", maxWidth: "100%" }}>
      {" "}
      <Form.Item label={t.style}>
        <Select
          aria-label={t.style}
          value={style.mode}
          disabled={busy}
          style={{ width: 130 }}
          onChange={(mode) => changeStyle({ mode })}
          options={[
            { value: "single", label: t.single },
            { value: "tile", label: t.tile },
          ]}
        />
      </Form.Item>
      <Form.Item label={t.position}>
        <Select
          aria-label={t.position}
          style={{ width: 140 }}
          disabled={busy || style.mode !== "single"}
          value={style.position}
          onChange={(position) => changeStyle({ position })}
          options={(
            [
              "topLeft",
              "topRight",
              "center",
              "bottomLeft",
              "bottomRight",
            ] as const
          ).map((value) => ({ value, label: t[value] }))}
        />
      </Form.Item>
      <Form.Item label={t.fontSize}>
        <InputNumber
          aria-label={t.fontSize}
          min={12}
          max={96}
          value={style.size}
          disabled={busy}
          onChange={(v) => changeStyle({ size: v ?? 28 })}
        />
      </Form.Item>
      <Form.Item label={t.opacity}>
        <InputNumber
          aria-label={t.opacity}
          min={0.05}
          max={1}
          step={0.05}
          value={style.opacity}
          disabled={busy}
          onChange={(v) => changeStyle({ opacity: v ?? 0.35 })}
        />
      </Form.Item>
      <Form.Item label={t.angle}>
        <InputNumber
          aria-label={t.angle}
          min={-90}
          max={90}
          value={style.angle}
          disabled={busy || style.mode !== "tile"}
          onChange={(v) => changeStyle({ angle: v ?? 0 })}
        />
      </Form.Item>
      <Form.Item label={t.color}>
        <input
          aria-label={t.color}
          type="color"
          value={style.color}
          disabled={busy}
          onChange={(e) => changeStyle({ color: e.target.value })}
        />
      </Form.Item>
    </Space>
  );
}
