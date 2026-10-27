import { useState } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from "antd";
import { api } from "./api";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
import { AccountsPanel } from "./AccountsPanel";
import { useCollaborationText } from "./collaborationText";
import { useWorkspace, type Team, type TeamRole } from "./workspaceContext";
export function TeamsPage() {
  const t = useCollaborationText(),
    context = useWorkspace();
  const [form] = Form.useForm();
  const [memberForm] = Form.useForm();
  const action = useJob<null>();
  const [editing, setEditing] = useState<Team>();
  const [name, setName] = useState("");
  const [enabled, setEnabled] = useState(true);
  const team = context.team;
  const manage = context.administrator || context.role === "owner";
  const options = (["owner", "operator", "reviewer"] as const).map((role) => ({
    value: role,
    label: t[role],
  }));
  async function member(username: string, role: TeamRole | "") {
    if (!team) return;
    await action.run(async (signal) => {
      await api(
        `/workspace/teams/${team.id}/members/${encodeURIComponent(username)}`,
        {
          method: "PUT",
          signal,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ role, revision: team.revision }),
        },
      );
      if (!signal.aborted) {
        memberForm.resetFields();
        await context.refresh();
      }
      return null;
    });
  }
  if (!context.available) return <Alert type="info" message={t.unavailable} />;
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.teams}</Typography.Title>
        <Typography.Text type="secondary">{t.hint}</Typography.Text>
      </div>
      <Card
        title={t.teams}
        extra={
          <Button onClick={() => void context.refresh()}>{t.refresh}</Button>
        }
      >
        {context.administrator && (
          <Form
            layout="inline"
            form={form}
            disabled={action.busy}
            onFinish={(value) =>
              void action.run(async (signal) => {
                const created = await (
                  await api("/workspace/teams", {
                    method: "POST",
                    signal,
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(value),
                  })
                ).json();
                if (!signal.aborted) {
                  form.resetFields();
                  await context.refresh();
                  context.select(created.id);
                }
                return null;
              })
            }
          >
            <Form.Item name="name" label={t.name} rules={[{ required: true }]}>
              <Input aria-label={t.name} maxLength={120} />
            </Form.Item>
            <Form.Item>
              <Button
                aria-label={t.createTeam}
                type="primary"
                htmlType="submit"
                loading={action.busy}
              >
                {t.createTeam}
              </Button>
            </Form.Item>
          </Form>
        )}
        {!editing && <ErrorNotice error={action.error} />}
        <Table
          rowKey="id"
          dataSource={context.teams}
          pagination={false}
          scroll={{ x: 500 }}
          columns={[
            {
              title: t.name,
              render: (_, item) =>
                item.id === "personal" ? t.personal : item.name,
            },
            {
              title: t.teamState,
              render: (_, item) => (
                <Tag color={item.enabled ? "green" : "default"}>
                  {item.enabled ? t.enabled : t.disabled}
                </Tag>
              ),
            },
            {
              title: t.actions,
              render: (_, item) => (
                <Space wrap>
                  <Button
                    disabled={!item.enabled || item.id === team?.id}
                    onClick={() => context.select(item.id)}
                  >
                    {t.scope}
                  </Button>
                  {context.administrator && item.id !== "personal" && (
                    <Button
                      onClick={() => {
                        action.reset();
                        setName(item.name);
                        setEnabled(item.enabled);
                        setEditing(item);
                      }}
                    >
                      {t.editTeam}
                    </Button>
                  )}
                </Space>
              ),
            },
          ]}
        />
        {manage && team?.id !== "personal" && (
          <Button
            disabled={action.busy}
            onClick={() => {
              action.reset();
              setName(team?.name ?? "");
              setEnabled(team?.enabled ?? true);
              setEditing(team);
            }}
          >
            {t.editTeam}
          </Button>
        )}
      </Card>
      <Card title={t.members}>
        {team?.id === "personal" ? (
          <Alert message={t.personalHint} />
        ) : (
          <>
            {manage && (
              <Form
                layout="vertical"
                form={memberForm}
                disabled={action.busy}
                onFinish={(value) => void member(value.username, value.role)}
              >
                <Space wrap align="start">
                  <Form.Item
                    name="username"
                    label={t.memberName}
                    extra={t.memberHint}
                    rules={[{ required: true }]}
                  >
                    <Input aria-label={t.memberName} />
                  </Form.Item>
                  <Form.Item name="role" label={t.role} initialValue="operator">
                    <Select
                      aria-label={t.role}
                      style={{ width: 160 }}
                      options={options}
                    />
                  </Form.Item>
                </Space>
                <Button
                  aria-label={t.setMember}
                  htmlType="submit"
                  loading={action.busy}
                >
                  {t.setMember}
                </Button>
              </Form>
            )}
            <Table
              rowKey="username"
              dataSource={Object.entries(team?.members ?? {}).map(
                ([username, role]) => ({ username, role }),
              )}
              pagination={false}
              scroll={{ x: 450 }}
              columns={[
                { title: t.username, dataIndex: "username" },
                {
                  title: t.role,
                  render: (_, item) =>
                    manage && item.username !== context.environmentAdmin ? (
                      <Select
                        aria-label={`${t.role}: ${item.username}`}
                        disabled={action.busy}
                        style={{ width: 160 }}
                        value={item.role}
                        options={options}
                        onChange={(role) => void member(item.username, role)}
                      />
                    ) : (
                      t[item.role]
                    ),
                },
                {
                  title: t.actions,
                  render: (_, item) =>
                    manage &&
                    item.username !== context.environmentAdmin && (
                      <Popconfirm
                        title={t.confirmRemove}
                        onConfirm={() => member(item.username, "")}
                      >
                        <Button
                          aria-label={t.remove}
                          danger
                          disabled={action.busy}
                        >
                          {t.remove}
                        </Button>
                      </Popconfirm>
                    ),
                },
              ]}
            />
          </>
        )}
      </Card>
      {context.administrator && <AccountsPanel />}
      <Modal
        title={t.editTeam}
        open={!!editing}
        footer={null}
        onCancel={() => {
          if (!action.busy) setEditing(undefined);
        }}
        destroyOnHidden
      >
        <Form
          layout="vertical"
          disabled={action.busy}
          onFinish={() =>
            editing &&
            void action.run(async (signal) => {
              await api(`/workspace/teams/${editing.id}`, {
                method: "PUT",
                signal,
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                  name,
                  enabled,
                  revision: editing.revision,
                }),
              });
              if (!signal.aborted) {
                setEditing(undefined);
                await context.refresh();
              }
              return null;
            })
          }
        >
          <Form.Item label={t.name}>
            <Input
              aria-label={t.name}
              value={name}
              onChange={(event) => setName(event.target.value)}
              maxLength={120}
            />
          </Form.Item>
          <Form.Item label={t.teamState}>
            <Switch
              aria-label={t.teamState}
              checked={enabled}
              onChange={setEnabled}
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
    </Space>
  );
}
