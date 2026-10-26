import { Card, Descriptions, Space, Typography } from "antd";
import { Preferences, PreferencesProps } from "./Preferences";
import { useText } from "./i18n";
export function Settings({
  username,
  ...preferences
}: PreferencesProps & { username: string }) {
  const t = useText();
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.settings}</Typography.Title>
        <Typography.Text type="secondary">{t.preferenceHint}</Typography.Text>
      </div>
      <Card title={t.preferences}>
        <Preferences {...preferences} />
      </Card>
      <Card title={t.account}>
        <Descriptions
          column={1}
          items={[{ key: "username", label: t.username, children: username }]}
        />
        <Typography.Text type="secondary">{t.sessionInfo}</Typography.Text>
      </Card>
      <Card title={t.setup}>
        <Typography.Paragraph>{t.setupHint}</Typography.Paragraph>
      </Card>
    </Space>
  );
}
