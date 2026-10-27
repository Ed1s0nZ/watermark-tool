import { useEffect, useState } from "react";
import {
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Space,
  Table,
  Tag,
  Typography,
} from "antd";
import { api } from "./api";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
import { useCollaborationText } from "./collaborationText";
import type { Account } from "./workspaceContext";
const passwordRule = {
  validator: async (_: unknown, value: string) => {
    const bytes = new TextEncoder().encode(value ?? "").length;
    if (bytes < 12 || bytes > 72) throw new Error("12–72 bytes");
  },
};
export function AccountsPanel() {
  const t = useCollaborationText();
  const [form] = Form.useForm();
  const [resetForm] = Form.useForm();
  const listing = useJob<Account[]>(),
    action = useJob<null>();
  const [resetting, setResetting] = useState<Account>();
  async function reload() {
    await listing.run(
      async (signal) =>
        (await (await api("/workspace/users", { signal })).json()).items,
    );
  }
  useEffect(() => {
    void reload();
  }, []);
  async function update(user: Account, enabled: boolean, password = "") {
    await action.run(async (signal) => {
      await api(`/workspace/users/${encodeURIComponent(user.username)}`, {
        method: "PUT",
        signal,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ revision: user.revision, enabled, password }),
      });
      if (!signal.aborted) {
        setResetting(undefined);
        resetForm.resetFields();
        await reload();
      }
      return null;
    });
  }
  return (
    <Card title={t.users}>
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <Form
          layout="vertical"
          form={form}
          onFinish={(value) =>
            void action.run(async (signal) => {
              await api("/workspace/users", {
                method: "POST",
                signal,
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(value),
              });
              if (!signal.aborted) {
                form.resetFields();
                await reload();
              }
              return null;
            })
          }
          disabled={action.busy}
        >
          <Space wrap align="start">
            <Form.Item
              name="username"
              label={t.newUsername}
              rules={[
                {
                  required: true,
                  pattern: /^[A-Za-z0-9][A-Za-z0-9_.@-]{0,63}$/,
                },
              ]}
            >
              <Input
                aria-label={t.newUsername}
                autoComplete="off"
                maxLength={64}
              />
            </Form.Item>
            <Form.Item
              name="password"
              label={t.password}
              rules={[{ required: true }, passwordRule]}
            >
              <Input.Password
                aria-label={t.password}
                autoComplete="new-password"
              />
            </Form.Item>
          </Space>
          <Typography.Paragraph type="secondary">
            {t.passwordHint}
          </Typography.Paragraph>
          <Button
            aria-label={t.createUser}
            type="primary"
            htmlType="submit"
            loading={action.busy}
          >
            {t.createUser}
          </Button>
        </Form>
        {!resetting && <ErrorNotice error={action.error} />}
        <ErrorNotice error={listing.error} />
        <Button onClick={() => void reload()} loading={listing.busy}>
          {t.refresh}
        </Button>
        <Table<Account>
          rowKey="username"
          dataSource={listing.value}
          loading={listing.busy}
          scroll={{ x: 600 }}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          columns={[
            { title: t.username, dataIndex: "username" },
            {
              title: t.userState,
              render: (_, user) => (
                <Tag color={user.enabled ? "green" : "default"}>
                  {user.enabled ? t.enabled : t.disabled}
                </Tag>
              ),
            },
            { title: t.revision, dataIndex: "revision" },
            {
              title: t.actions,
              render: (_, user) => (
                <Space wrap>
                  <Popconfirm
                    title={user.enabled ? t.disableConfirm : t.enableUser}
                    onConfirm={() => update(user, !user.enabled)}
                  >
                    <Button danger={user.enabled} disabled={action.busy}>
                      {user.enabled ? t.disableUser : t.enableUser}
                    </Button>
                  </Popconfirm>
                  <Button
                    disabled={action.busy}
                    onClick={() => {
                      action.reset();
                      resetForm.resetFields();
                      setResetting(user);
                    }}
                  >
                    {t.resetPassword}
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Space>
      <Modal
        title={t.resetPassword}
        open={!!resetting}
        onCancel={() => {
          if (!action.busy) {
            setResetting(undefined);
            resetForm.resetFields();
            action.reset();
          }
        }}
        footer={null}
        destroyOnHidden
      >
        <Form
          layout="vertical"
          form={resetForm}
          disabled={action.busy}
          onFinish={(value) =>
            resetting &&
            void update(resetting, resetting.enabled, value.password)
          }
        >
          <Typography.Paragraph>{resetting?.username}</Typography.Paragraph>
          <Form.Item
            name="password"
            label={t.newPassword}
            rules={[{ required: true }, passwordRule]}
          >
            <Input.Password
              aria-label={t.newPassword}
              autoComplete="new-password"
            />
          </Form.Item>
          <ErrorNotice error={action.error} />
          <Button
            aria-label={t.save}
            type="primary"
            htmlType="submit"
            loading={action.busy}
          >
            {t.save}
          </Button>
        </Form>
      </Modal>
    </Card>
  );
}
