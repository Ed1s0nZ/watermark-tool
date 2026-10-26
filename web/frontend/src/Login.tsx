import { useState } from "react";
import { Button, Card, Form, Input, Space, Tag, Typography } from "antd";
import {
  ArrowRightOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { api, User } from "./api";
import { useText } from "./i18n";
import { ErrorNotice } from "./ErrorNotice";
import { Preferences, PreferencesProps } from "./Preferences";
export function Login({
  onLogin,
  expired,
  initialError,
  ...preferences
}: PreferencesProps & {
  onLogin: (user: User) => void;
  expired: boolean;
  initialError?: unknown;
}) {
  const t = useText();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  async function login(values: { username: string; password: string }) {
    setBusy(true);
    setError(undefined);
    try {
      const response = await api("/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(values),
      });
      onLogin((await response.json()).user);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="login-page">
      <header className="login-header">
        <a className="brand" href="#overview">
          <span className="brand-mark">
            <SafetyCertificateOutlined aria-hidden="true" />
          </span>
          <span>
            {t.brand}
            <small>DOCUMENT WORKSPACE</small>
          </span>
        </a>
        <Preferences {...preferences} />
      </header>
      <main className="login-main">
        <section className="login-story">
          <Tag color="blue">WATERMARK PLATFORM</Tag>
          <h1>{t.loginFeature}</h1>
          <p>{t.loginFeatureHint}</p>
          <div className="document-art" aria-hidden="true">
            <div className="paper back-paper" />
            <div className="paper front-paper">
              <div className="paper-symbol">
                <SafetyCertificateOutlined aria-hidden="true" />
              </div>
              <div className="paper-line long" />
              <div className="paper-line" />
              <div className="paper-line long" />
              <div className="paper-line short" />
              <div className="paper-stamp">WM · VERIFIED</div>
            </div>
            <div className="art-label">
              <LockOutlined aria-hidden="true" /> INVISIBLE WATERMARK
            </div>
          </div>
        </section>
        <Card className="login-card">
          <Space direction="vertical" size={22} style={{ width: "100%" }}>
            <div>
              <Typography.Title level={2}>{t.loginTitle}</Typography.Title>
              <Typography.Text type="secondary">{t.loginHint}</Typography.Text>
            </div>
            {expired && (
              <Typography.Text type="warning">{t.unauthorized}</Typography.Text>
            )}
            <ErrorNotice error={error || initialError} />
            <Form
              layout="vertical"
              onFinish={login}
              requiredMark={false}
              disabled={busy}
            >
              <Form.Item
                label={t.username}
                name="username"
                rules={[
                  { required: true, whitespace: true, message: t.required },
                ]}
              >
                <Input
                  size="large"
                  prefix={<UserOutlined aria-hidden="true" />}
                  autoComplete="username"
                  maxLength={256}
                />
              </Form.Item>
              <Form.Item
                label={t.password}
                name="password"
                rules={[{ required: true, message: t.required }]}
              >
                <Input.Password
                  size="large"
                  prefix={<LockOutlined aria-hidden="true" />}
                  autoComplete="current-password"
                  maxLength={1024}
                />
              </Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                size="large"
                block
                loading={busy}
                icon={<ArrowRightOutlined aria-hidden="true" />}
              >
                {t.login}
              </Button>
            </Form>
            <Typography.Text type="secondary">{t.loginNote}</Typography.Text>
          </Space>
        </Card>
      </main>
      <footer className="login-footer">{t.footer}</footer>
    </div>
  );
}
