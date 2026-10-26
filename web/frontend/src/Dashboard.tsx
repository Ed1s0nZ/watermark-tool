import { Button, Card, Col, Divider, Row, Space, Tag, Typography } from "antd";
import {
  ArrowRightOutlined,
  FileProtectOutlined,
  ScanOutlined,
  InfoCircleOutlined,
} from "@ant-design/icons";
import { useText } from "./i18n";

export function Dashboard({
  types,
  onNavigate,
}: {
  types: string[];
  onNavigate: (page: string) => void;
}) {
  const t = useText();
  return (
    <div className="dashboard">
      <div className="page-heading dashboard-heading">
        <Typography.Title level={2}>{t.overview}</Typography.Title>
        <Typography.Text type="secondary">{t.overviewHint}</Typography.Text>
      </div>
      <Row gutter={[20, 20]}>
        {[
          {
            page: "add",
            title: t.add,
            description: t.addHint,
            action: t.start,
            icon: <FileProtectOutlined aria-hidden="true" />,
          },
          {
            page: "extract",
            title: t.extract,
            description: t.extractHint,
            action: t.inspect,
            icon: <ScanOutlined aria-hidden="true" />,
          },
        ].map((item) => (
          <Col xs={24} md={12} key={item.page}>
            <Card className="workspace-action">
              <div className="workspace-action-heading">
                <span className={`action-icon ${item.page}`}>{item.icon}</span>
                <Typography.Title level={4}>{item.title}</Typography.Title>
              </div>
              <Typography.Paragraph type="secondary">
                {item.description}
              </Typography.Paragraph>
              <Button
                type={item.page === "add" ? "primary" : "default"}
                onClick={() => onNavigate(item.page)}
                icon={<ArrowRightOutlined aria-hidden="true" />}
              >
                {item.action}
              </Button>
            </Card>
          </Col>
        ))}
      </Row>
      <Card className="workspace-reference" title={t.supported}>
        <Space wrap size={[6, 8]}>
          {types.map((type) => (
            <Tag key={type}>{type.toUpperCase()}</Tag>
          ))}
        </Space>
        <Divider />
        <div className="workspace-limits">
          <span>
            {t.maxSize}
            <strong>50 MB</strong>
          </span>
          <span>
            {t.limit}
            <strong>100 {t.bytes}</strong>
          </span>
        </div>
        <Typography.Paragraph type="secondary" className="workspace-privacy">
          <InfoCircleOutlined aria-hidden="true" /> {t.privacy}
        </Typography.Paragraph>
      </Card>
    </div>
  );
}
