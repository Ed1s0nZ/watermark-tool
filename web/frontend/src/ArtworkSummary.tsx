import { Card, Descriptions, Space, Tag, Typography } from "antd";
import type { DeliveryJob } from "./jobClient";
import { useArtText } from "./artText";
export function ArtworkSummary({ job }: { job: DeliveryJob }) {
  const t = useArtText();
  const art = job.artwork;
  const output = job.output;
  const percent = (v: number) => `${Number((v * 100).toFixed(2))}%`;
  return (
    <Card size="small" title={t.snapshot}>
      <Descriptions
        column={1}
        size="small"
        items={[
          {
            key: "format",
            label: t.output,
            children:
              output?.format === "jpeg"
                ? `JPEG · ${output.jpeg_quality ?? 90}`
                : "PNG",
          },
          {
            key: "layers",
            label: t.layers,
            children:
              job.mode === "blind" ? t.blindOnly : t[art?.layers ?? "text"],
          },
          ...(art?.logo
            ? [
                {
                  key: "logo",
                  label: t.logoPosition,
                  children: `${t[art.logo.position]} · ${t.ratio}: ${percent(art.logo.width_ratio)} · ${t.opacity}: ${percent(art.logo.opacity)}`,
                },
                {
                  key: "digest",
                  label: t.logoDigest,
                  children: (
                    <Typography.Text
                      copyable
                      style={{ overflowWrap: "anywhere" }}
                    >
                      {art.logo.sha256}
                    </Typography.Text>
                  ),
                },
              ]
            : []),
          {
            key: "regions",
            label: t.regions,
            children: (
              <Space direction="vertical" style={{ width: "100%" }}>
                {job.sources.map((s) => (
                  <div key={s.id}>
                    <Typography.Text>
                      {s.name} · {s.source_sha256.slice(0, 8)}
                    </Typography.Text>
                    <div>
                      {art?.exclusions?.[s.source_sha256]?.length ? (
                        art.exclusions[s.source_sha256].map((r, i) => (
                          <Tag
                            key={i}
                            style={{ whiteSpace: "normal", marginTop: 4 }}
                          >
                            {t.number} {i + 1}: {t.x} {percent(r.x)} · {t.y}{" "}
                            {percent(r.y)} · {t.width} {percent(r.width)} ·{" "}
                            {t.height} {percent(r.height)}
                          </Tag>
                        ))
                      ) : (
                        <Typography.Text type="secondary">
                          {" "}
                          · {t.noRegions}
                        </Typography.Text>
                      )}
                    </div>
                  </div>
                ))}
              </Space>
            ),
          },
        ]}
      />
    </Card>
  );
}
