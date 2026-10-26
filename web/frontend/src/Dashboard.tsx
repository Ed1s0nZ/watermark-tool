import { Alert, Button, Card, Col, Row, Space, Tag, Typography } from "antd";
import {
  ArrowRightOutlined,
  FileProtectOutlined,
  ScanOutlined,
  SafetyCertificateOutlined,
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
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <section className="welcome-panel">
        <div>
          <span className="eyebrow">DOCUMENT PROTECTION</span>
          <h1>{t.welcome}</h1>
          <p>{t.welcomeHint}</p>
          <Space wrap>
            <Button
              type="primary"
              size="large"
              onClick={() => onNavigate("add")}
              icon={<ArrowRightOutlined aria-hidden="true" />}
            >
              {t.start}
            </Button>
            <Button size="large" onClick={() => onNavigate("extract")}>
              {t.inspect}
            </Button>
          </Space>
        </div>
        <div className="welcome-emblem" aria-hidden="true">
          <SafetyCertificateOutlined aria-hidden="true" />
        </div>
      </section>
      <Row gutter={[20, 20]}>
        {[
          { label: t.supported, value: types.length || "—", unit: "" },
          { label: t.maxSize, value: 50, unit: "MB" },
          { label: t.limit, value: 100, unit: t.bytes },
        ].map((item) => (
          <Col xs={24} sm={8} key={item.label}>
            <Card className="metric-card">
              <Typography.Text type="secondary">{item.label}</Typography.Text>
              <div className="metric-value">
                {item.value}
                <small>{item.unit}</small>
              </div>
            </Card>
          </Col>
        ))}
      </Row>
      <div>
        <Typography.Title level={4}>{t.capabilities}</Typography.Title>
        <Row gutter={[20, 20]}>
          <Col xs={24} md={12}>
            <Card className="capability-card">
              <span className="capability-icon">
                <FileProtectOutlined aria-hidden="true" />
              </span>
              <Typography.Title level={4}>{t.invisible}</Typography.Title>
              <p>{t.invisibleHint}</p>
              <Button
                onClick={() => onNavigate("add")}
                icon={<ArrowRightOutlined aria-hidden="true" />}
              >
                {t.add}
              </Button>
            </Card>
          </Col>
          <Col xs={24} md={12}>
            <Card className="capability-card">
              <span className="capability-icon teal">
                <ScanOutlined aria-hidden="true" />
              </span>
              <Typography.Title level={4}>{t.verify}</Typography.Title>
              <p>{t.verifyHint}</p>
              <Button
                onClick={() => onNavigate("extract")}
                icon={<ArrowRightOutlined aria-hidden="true" />}
              >
                {t.extract}
              </Button>
            </Card>
          </Col>
        </Row>
      </div>
      <Card
        title={t.supported}
        extra={
          <Typography.Text type="secondary">{t.supportedHint}</Typography.Text>
        }
      >
        <Space wrap>
          {types.map((type) => (
            <Tag key={type}>{type.toUpperCase()}</Tag>
          ))}
        </Space>
      </Card>
      <Alert showIcon type="info" message={t.privacy} />
    </Space>
  );
}
