import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  InputNumber,
  Select,
  Space,
  Typography,
} from "antd";
import { describe, Rect, validRect } from "./artwork";
import { useArtText } from "./artText";
export function ExclusionEditor({
  files,
  value,
  onChange,
  disabled = false,
}: {
  files: File[];
  value: Record<string, Rect[]>;
  onChange: (v: Record<string, Rect[]>) => void;
  disabled?: boolean;
}) {
  const t = useArtText();
  const [items, setItems] = useState<
      { file: File; sha256: string; width: number; height: number }[]
    >([]),
    [selected, setSelected] = useState(""),
    [url, setURL] = useState(""),
    [error, setError] = useState(false),
    [drag, setDrag] = useState<Rect>();
  const start = useRef<{ x: number; y: number } | undefined>(undefined);
  useEffect(() => {
    let active = true;
    setError(false);
    void Promise.all(
      files.map(async (file) => ({ file, ...(await describe(file)) })),
    )
      .then((rows) => {
        if (active) {
          setItems(rows);
          setSelected((prev) =>
            rows.some((r) => r.sha256 === prev)
              ? prev
              : (rows[0]?.sha256 ?? ""),
          );
        }
      })
      .catch(() => {
        if (active) {
          setItems([]);
          setError(true);
        }
      });
    return () => {
      active = false;
    };
  }, [files]);
  const current = items.find((i) => i.sha256 === selected);
  const regions = value[selected] ?? [];
  useEffect(() => {
    if (!current) {
      setURL("");
      return;
    }
    const u = URL.createObjectURL(current.file);
    setURL(u);
    return () => URL.revokeObjectURL(u);
  }, [current?.file]);
  function update(rs: Rect[]) {
    setError(false);
    onChange({ ...value, [selected]: rs });
  }
  const point = (e: React.PointerEvent<HTMLDivElement>) => {
    const b = e.currentTarget.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(1, (e.clientX - b.left) / b.width)),
      y: Math.max(0, Math.min(1, (e.clientY - b.top) / b.height)),
    };
  };
  return (
    <Card size="small" title={t.regions}>
      <Space direction="vertical" style={{ width: "100%" }}>
        <Typography.Text type="secondary">{t.regionHint}</Typography.Text>
        {error && <Alert type="error" message={t.invalid} />}
        <Select
          aria-label={t.source}
          value={selected || undefined}
          disabled={disabled || !items.length}
          style={{ width: "100%" }}
          options={items.map((r, i) => ({
            value: r.sha256,
            label: `${i + 1}. ${r.file.name} · ${r.sha256.slice(0, 8)}`,
          }))}
          onChange={(s) => {
            setSelected(s);
            setDrag(undefined);
          }}
        />
        {url && current ? (
          <div
            data-testid="exclusion-canvas"
            style={{
              position: "relative",
              width: Math.min(current.width, 640),
              maxWidth: "100%",
              touchAction: "none",
              cursor: disabled ? "default" : "crosshair",
            }}
            onPointerDown={(e) => {
              if (disabled || regions.length >= 8) return;
              e.preventDefault();
              e.currentTarget.setPointerCapture(e.pointerId);
              start.current = point(e);
              setDrag(undefined);
            }}
            onPointerMove={(e) => {
              if (!start.current) return;
              const p = point(e),
                s = start.current;
              setDrag({
                x: Math.min(s.x, p.x),
                y: Math.min(s.y, p.y),
                width: Math.abs(s.x - p.x),
                height: Math.abs(s.y - p.y),
              });
            }}
            onPointerUp={(e) => {
              if (!start.current) return;
              const p = point(e),
                s = start.current;
              start.current = undefined;
              setDrag(undefined);
              const r = {
                x: Math.min(s.x, p.x),
                y: Math.min(s.y, p.y),
                width: Math.abs(s.x - p.x),
                height: Math.abs(s.y - p.y),
              };
              if (r.width > 0.005 && r.height > 0.005 && validRect(r))
                update([...regions, r]);
            }}
            onPointerCancel={() => {
              start.current = undefined;
              setDrag(undefined);
            }}
          >
            <img
              src={url}
              alt={t.regions}
              draggable={false}
              style={{ display: "block", width: "100%", pointerEvents: "none" }}
            />
            {[...regions, ...(drag ? [drag] : [])].map((r, i) => (
              <div
                key={i}
                style={{
                  position: "absolute",
                  pointerEvents: "none",
                  left: `${r.x * 100}%`,
                  top: `${r.y * 100}%`,
                  width: `${r.width * 100}%`,
                  height: `${r.height * 100}%`,
                  border: "2px solid #fa8c16",
                  background: "#fa8c1633",
                  boxSizing: "border-box",
                }}
              >
                {i + 1}
              </div>
            ))}
          </div>
        ) : (
          <Typography.Text>{t.empty}</Typography.Text>
        )}
        {regions.map((r, i) => (
          <Space key={i} wrap>
            <Typography.Text>
              {t.number} {i + 1}
            </Typography.Text>
            {(["x", "y", "width", "height"] as const).map((k) => (
              <InputNumber
                key={k}
                aria-label={`${t.number} ${i + 1} ${k} %`}
                min={0}
                max={100}
                step={0.1}
                precision={2}
                value={r[k] * 100}
                disabled={disabled}
                style={{ width: 95 }}
                onChange={(v) => {
                  if (v === null) return;
                  const next = { ...r, [k]: v / 100 };
                  if (validRect(next))
                    update(regions.map((old, n) => (n === i ? next : old)));
                  else setError(true);
                }}
              />
            ))}
            <Button
              disabled={disabled}
              onClick={() => update(regions.filter((_, n) => n !== i))}
            >
              {t.remove} {i + 1}
            </Button>
          </Space>
        ))}
        <Button
          disabled={disabled || !regions.length}
          onClick={() => update([])}
        >
          {t.clear}
        </Button>
      </Space>
    </Card>
  );
}
