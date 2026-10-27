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
  Typography,
} from "antd";
import { APIError, api } from "./api";
import { useWorkspace } from "./workspaceContext";
import { useRecipientText } from "./recipientText";
import { RecipientTable } from "./RecipientTable";
import {
  contactJSON,
  contactPath,
  downloadRecipients,
  type Contact,
} from "./recipientAPI";
import { useJob } from "./useJob";
import { ErrorNotice } from "./ErrorNotice";
import { FileUpload } from "./FileUpload";
export function RecipientsPage() {
  const context = useWorkspace(),
    t = useRecipientText(),
    job = useJob<null>();
  const [revision, setRevision] = useState(0),
    [editing, setEditing] = useState<Contact | null | undefined>(),
    [importing, setImporting] = useState(false);
  const [csv, setCSV] = useState(""),
    [file, setFile] = useState<File>(),
    [form] = Form.useForm();
  if (!context.available || !context.team)
    return <Alert message={t.unavailable} />;
  const base = contactPath(context.team.id),
    write = context.administrator || context.role !== "reviewer";
  const refresh = () => setRevision((value) => value + 1);
  const errorRow =
    job.error instanceof APIError
      ? (job.error.data as { row?: number })?.row
      : undefined;
  function edit(item: Contact | null) {
    job.reset();
    form.resetFields();
    form.setFieldsValue(item ?? { recipient_id: "", name: "", groups: [] });
    setEditing(item);
  }
  async function exportFile(format: string) {
    await job.run(async (signal) => {
      const response = await api(base + "/export?format=" + format, { signal });
      const blob = await response.blob();
      if (!signal.aborted) downloadRecipients(blob, "recipients." + format);
      return null;
    });
  }
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.title}</Typography.Title>
        <Typography.Text type="secondary">{t.hint}</Typography.Text>
      </div>
      <Card>
        <Space wrap style={{ marginBottom: 16 }}>
          {write && (
            <>
              <Button type="primary" onClick={() => edit(null)}>
                {t.add}
              </Button>
              <Button
                onClick={() => {
                  job.reset();
                  setCSV("");
                  setFile(undefined);
                  setImporting(true);
                }}
              >
                {t.import}
              </Button>
            </>
          )}
          <Button disabled={job.busy} onClick={() => void exportFile("json")}>
            {t.exportJSON}
          </Button>
          <Button disabled={job.busy} onClick={() => void exportFile("csv")}>
            {t.exportCSV}
          </Button>
          <Button onClick={refresh}>{t.refresh}</Button>
        </Space>
        <Typography.Paragraph type="secondary">
          {t.exportHint}
        </Typography.Paragraph>
        {!importing && editing === undefined && (
          <ErrorNotice error={job.error} />
        )}
        <RecipientTable
          team={context.team.id}
          revision={revision}
          actions={
            write
              ? (item) => (
                  <Space>
                    <Button
                      aria-label={t.edit}
                      disabled={job.busy}
                      onClick={() => edit(item)}
                    >
                      {t.edit}
                    </Button>
                    <Popconfirm
                      title={t.confirm}
                      okText={t.remove}
                      cancelText={t.close}
                      onConfirm={() =>
                        job.run(async (signal) => {
                          await contactJSON(
                            base + "/" + item.id,
                            { revision: item.revision },
                            signal,
                            "DELETE",
                          );
                          if (!signal.aborted) refresh();
                          return null;
                        })
                      }
                    >
                      <Button aria-label={t.remove} danger disabled={job.busy}>
                        {t.remove}
                      </Button>
                    </Popconfirm>
                  </Space>
                )
              : undefined
          }
        />
      </Card>
      <Modal
        title={editing ? t.edit : t.add}
        open={editing !== undefined}
        destroyOnHidden
        footer={null}
        onCancel={() => {
          if (!job.busy) setEditing(undefined);
        }}
      >
        <Form
          form={form}
          layout="vertical"
          disabled={job.busy}
          onFinish={(value) =>
            void job.run(async (signal) => {
              await contactJSON(
                base + (editing ? "/" + editing.id : ""),
                {
                  ...value,
                  groups: value.groups ?? [],
                  ...(editing ? { revision: editing.revision } : {}),
                },
                signal,
                editing ? "PUT" : "POST",
              );
              if (!signal.aborted) {
                setEditing(undefined);
                refresh();
              }
              return null;
            })
          }
        >
          <Form.Item
            name="recipient_id"
            label={t.id}
            rules={[{ required: true }]}
          >
            <Input aria-label={t.id} maxLength={64} />
          </Form.Item>
          <Form.Item name="name" label={t.name} rules={[{ required: true }]}>
            <Input aria-label={t.name} maxLength={128} />
          </Form.Item>
          <Form.Item name="groups" label={t.groups} extra={t.groupHint}>
            <Select
              aria-label={t.groups}
              mode="tags"
              maxCount={10}
              tokenSeparators={["|"]}
            />
          </Form.Item>
          <ErrorNotice error={job.error} />
          <Button
            type="primary"
            htmlType="submit"
            aria-label={t.save}
            loading={job.busy}
          >
            {t.save}
          </Button>
        </Form>
      </Modal>
      <Modal
        title={t.import}
        open={importing}
        destroyOnHidden
        onCancel={() => {
          if (!job.busy) setImporting(false);
        }}
        footer={
          <Button
            type="primary"
            aria-label={t.import}
            disabled={!csv || job.busy}
            loading={job.busy}
            onClick={() =>
              void job.run(async (signal) => {
                await contactJSON(base + "/import", { csv }, signal);
                if (!signal.aborted) {
                  setImporting(false);
                  refresh();
                }
                return null;
              })
            }
          >
            {t.import}
          </Button>
        }
      >
        <Typography.Paragraph>{t.importHint}</Typography.Paragraph>
        <FileUpload
          label={t.import}
          accept=".csv,text/csv"
          maxSize={1 << 20}
          value={file ? [file] : []}
          disabled={job.busy}
          onChange={(files) => {
            const selected = files[0];
            setFile(selected);
            setCSV("");
            job.reset();
            if (selected)
              void job.run(async (signal) => {
                const bytes = await selected.arrayBuffer();
                let text: string;
                try {
                  text = new TextDecoder("utf-8", { fatal: true }).decode(
                    bytes,
                  );
                } catch {
                  throw new APIError(422, "workspace_invalid");
                }
                if (!signal.aborted) setCSV(text);
                return null;
              });
          }}
        />
        <Input.TextArea
          aria-label={t.csv}
          value={csv}
          rows={8}
          disabled={job.busy}
          onChange={(e) => setCSV(e.target.value)}
        />
        <ErrorNotice error={job.error} />
        {errorRow !== undefined && (
          <Alert type="error" message={`${t.row}: ${errorRow}`} />
        )}
      </Modal>
    </Space>
  );
}
