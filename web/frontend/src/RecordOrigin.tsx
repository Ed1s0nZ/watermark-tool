import { Descriptions, Typography } from "antd";
import { CopyRecord } from "./platform";
import { useText } from "./i18n";
export function RecordOrigin({ record }: { record: CopyRecord }) {
  const t = useText();
  const items = [
    ...(record.source_name
      ? [{ key: "name", label: t.sourceName, children: record.source_name }]
      : []),
    ...(record.source_id
      ? [{ key: "id", label: t.sourceID, children: record.source_id }]
      : []),
    { key: "original", label: t.sourceDigest, children: record.source_sha256 },
    ...(record.rendered_sha256
      ? [
          {
            key: "rendered",
            label: t.renderedDigest,
            children: record.rendered_sha256,
          },
        ]
      : []),
  ];
  return (
    <Descriptions
      size="small"
      column={1}
      items={items.map((item) => ({
        ...item,
        children: (
          <Typography.Text style={{ overflowWrap: "anywhere" }} copyable>
            {item.children}
          </Typography.Text>
        ),
      }))}
    />
  );
}
