import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Select,
  Space,
  Table,
  Typography,
} from "antd";
import { api, APIError } from "./api";
import { ErrorNotice } from "./ErrorNotice";
import { useJob } from "./useJob";
import {
  defaultStyle,
  loadTemplates,
  Style,
  Template,
} from "./visibleWatermark";
import { VisibleStyleFields } from "./VisibleStyleFields";
import {
  createJob,
  DeliveryJob,
  Plan,
  post,
  prepare,
  saveBlob,
  uploadAndStart,
  matchesPlan,
} from "./jobClient";
import { useJobText, taskLabel } from "./jobText";
import { JobPreview } from "./JobPreview";
import { JobDetails } from "./JobDetails";
const selectionKey = "watermark.jobs.current.v1";
function saved() {
  try {
    const id = localStorage.getItem(selectionKey) ?? "";
    return /^[a-f0-9]{32}$/.test(id) ? id : "";
  } catch {
    return "";
  }
}
export function JobsPage() {
  const t = useJobText();
  const [files, setFiles] = useState<File[]>([]);
  const [csv, setCSV] = useState("recipient_id,name\n001,Alice\n002,张三");
  const [mode, setMode] = useState<"blind" | "dual">("blind");
  const [text, setText] = useState("内部资料 · {name} · {recipient_id}");
  const [style, setStyle] = useState<Style>(defaultStyle);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [storageError, setStorageError] = useState(false);
  const [selected, setSelected] = useState(saved);
  const current = useRef(selected);
  const [task, setTask] = useState<DeliveryJob>();
  const [pollError, setPollError] = useState<unknown>();
  const [progress, setProgress] = useState("");
  const key = useRef(crypto.randomUUID());
  const plan = useJob<Plan>();
  const action = useJob<DeliveryJob>();
  const controls = useJob<null>();
  const listing = useJob<{ items: DeliveryJob[]; total: number }>();
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  useEffect(() => {
    try {
      setTemplates(loadTemplates());
    } catch {
      setStorageError(true);
    }
  }, []);
  const csvInput = useRef<HTMLInputElement>(null);
  const busy = plan.busy || action.busy || controls.busy;
  function change() {
    plan.reset();
    action.reset();
    key.current = crypto.randomUUID();
    setProgress("");
  }
  function select(job: DeliveryJob) {
    current.current = job.id;
    setSelected(job.id);
    setTask(job);
    setPollError(undefined);
    try {
      localStorage.setItem(selectionKey, job.id);
    } catch {}
  }
  function update(job: DeliveryJob) {
    if (current.current !== job.id) return;
    setTask((prev) => {
      if (
        prev?.id === job.id &&
        (Date.parse(prev.updated_at) > Date.parse(job.updated_at) ||
          (Date.parse(prev.updated_at) === Date.parse(job.updated_at) &&
            prev.attempts === job.attempts &&
            prev.done > job.done))
      )
        return prev;
      return job;
    });
  }
  async function list(signal: AbortSignal) {
    return post<{ items: DeliveryJob[]; total: number }>(
      "/jobs/search",
      { q, page, page_size: 10 },
      signal,
    );
  }
  useEffect(() => {
    listing.reset();
    void listing.run(list);
  }, [q, page]);
  useEffect(() => {
    if (!selected) return;
    const controller = new AbortController();
    let pending = false;
    let stopped = false;
    const fetch = async () => {
      if (pending || stopped) return;
      pending = true;
      try {
        const job = await (
          await api("/jobs/" + selected, { signal: controller.signal })
        ).json();
        if (!controller.signal.aborted) {
          update(job);
          setPollError(undefined);
        }
      } catch (e) {
        if (!controller.signal.aborted) {
          setPollError(e);
          if (e instanceof APIError && e.status === 404) stopped = true;
        }
      } finally {
        pending = false;
      }
    };
    void fetch();
    const timer = setInterval(fetch, 1000);
    return () => {
      controller.abort();
      clearInterval(timer);
    };
  }, [selected]);
  useEffect(() => {
    const timer = setInterval(() => void listing.run(list), 5000);
    return () => clearInterval(timer);
  }, [q, page]);
  function restore(job: DeliveryJob) {
    change();
    setMode(job.mode);
    setText(job.template_text ?? "{name}");
    setStyle(job.style ? { ...job.style } : defaultStyle);
    const recipients = Array.from(
      new Map(
        job.copies.map((r) => [
          r.recipient_id,
          { recipient_id: r.recipient_id, name: r.name },
        ]),
      ).values(),
    );
    const cell = (value: string) => '"' + value.replaceAll('"', '""') + '"';
    setCSV(
      "recipient_id,name\n" +
        recipients
          .map((r) => cell(r.recipient_id) + "," + cell(r.name))
          .join("\n"),
    );
  }
  async function transfer(signal: AbortSignal, existing?: DeliveryJob) {
    if (!plan.value) throw new APIError(422, "job_invalid");
    let job = existing ?? (await createJob(plan.value, key.current, signal));
    if (signal.aborted) throw new DOMException("Aborted", "AbortError");
    select(job);
    job = await uploadAndStart(
      plan.value,
      job,
      signal,
      (value, done, total) => {
        if (!signal.aborted) {
          update(value);
          setProgress(`${t.uploading} ${done}/${total}`);
        }
      },
    );
    if (!signal.aborted) {
      update(job);
      plan.reset();
      key.current = crypto.randomUUID();
      setProgress("");
      void listing.run(list);
    }
    return job;
  }
  return (
    <Space direction="vertical" size={24} style={{ width: "100%" }}>
      <div className="page-heading">
        <Typography.Title level={2}>{t.jobs}</Typography.Title>
        <Typography.Text type="secondary">{t.hint}</Typography.Text>
      </div>
      <Alert message={t.limits} />
      <Card>
        <Form layout="vertical">
          <Form.Item label={t.files}>
            <input
              aria-label={t.files}
              type="file"
              multiple
              accept=".png,.jpg,.jpeg"
              disabled={busy}
              onChange={(e) => {
                change();
                setFiles(Array.from(e.target.files ?? []));
              }}
            />
          </Form.Item>
          <Form.Item label={t.csv}>
            <Input.TextArea
              aria-label={t.csv}
              value={csv}
              rows={4}
              disabled={busy}
              onChange={(e) => {
                change();
                setCSV(e.target.value);
              }}
            />
          </Form.Item>
          <input
            ref={csvInput}
            aria-label={t.importCSV}
            type="file"
            accept=".csv,text/csv"
            style={{ display: "none" }}
            disabled={busy}
            onChange={(e) => {
              const file = e.target.files?.[0];
              e.currentTarget.value = "";
              if (!file) return;
              change();
              void controls.run(async (signal) => {
                if (file.size > 16 * 1024)
                  throw new APIError(422, "job_invalid");
                let data: string;
                try {
                  data = new TextDecoder("utf-8", { fatal: true }).decode(
                    await file.arrayBuffer(),
                  );
                } catch {
                  throw new APIError(422, "job_invalid");
                }
                if (!signal.aborted) setCSV(data);
                return null;
              });
            }}
          />
          <Form.Item>
            <Button disabled={busy} onClick={() => csvInput.current?.click()}>
              {t.importCSV}
            </Button>
          </Form.Item>
          <Form.Item label={t.mode}>
            <Select
              aria-label={t.mode}
              disabled={busy}
              value={mode}
              onChange={(v) => {
                change();
                setMode(v);
              }}
              options={[
                { value: "blind", label: t.blind },
                { value: "dual", label: t.dual },
              ]}
            />
          </Form.Item>
          {mode === "dual" && (
            <>
              <Form.Item label={t.template} extra={t.templateHint}>
                <Input
                  aria-label={t.template}
                  value={text}
                  disabled={busy}
                  onChange={(e) => {
                    change();
                    setText(e.target.value);
                  }}
                />
              </Form.Item>
              <Form.Item label={t.styles}>
                <Select
                  aria-label={t.styles}
                  style={{ width: 240, maxWidth: "100%" }}
                  disabled={busy}
                  placeholder={t.styles}
                  onChange={(v) => {
                    change();
                    setStyle({ ...templates[Number(v)].style });
                  }}
                  options={templates.map((item, n) => ({
                    value: String(n),
                    label: item.name,
                  }))}
                />
              </Form.Item>
              <VisibleStyleFields
                style={style}
                busy={busy}
                onChange={(value) => {
                  change();
                  setStyle({ ...style, ...value });
                }}
              />
            </>
          )}
          {storageError && <Alert type="warning" message={t.invalid} />}
          <Space wrap>
            <Button
              type="primary"
              aria-label={t.prepare}
              loading={plan.busy}
              disabled={action.busy}
              onClick={() =>
                void plan.run((signal) =>
                  prepare(
                    files,
                    csv,
                    mode,
                    text,
                    style,
                    signal,
                    (done, total) => {
                      if (!signal.aborted)
                        setProgress(`${t.rendering} ${done}/${total}`);
                    },
                  ).catch((e) => {
                    if (e instanceof APIError) throw e;
                    if (e instanceof Error && e.message === "font")
                      throw new APIError(422, "job_font");
                    throw new APIError(422, "job_invalid");
                  }),
                )
              }
            >
              {t.prepare}
            </Button>
            <Button
              type="primary"
              aria-label={t.create}
              disabled={!plan.value || plan.busy}
              loading={action.busy}
              onClick={() => void action.run((signal) => transfer(signal))}
            >
              {t.create}
            </Button>
            {busy && (
              <Button
                onClick={() => {
                  plan.cancel();
                  action.cancel();
                }}
              >
                {t.stopUpload}
              </Button>
            )}
          </Space>
        </Form>
        {progress && (
          <Typography.Paragraph role="status">{progress}</Typography.Paragraph>
        )}
        {(plan.cancelled || action.cancelled) && <Alert message={t.stopped} />}
        <ErrorNotice error={plan.error} />
        <ErrorNotice error={action.error} />
        <ErrorNotice error={controls.error} />
      </Card>
      {plan.value && <JobPreview plan={plan.value} />}
      <ErrorNotice error={pollError} />
      {task ? (
        <JobDetails
          job={task}
          onRestore={() => restore(task)}
          retryReady={!!plan.value && matchesPlan(plan.value, task)}
          busy={action.busy || controls.busy}
          onRetry={() =>
            void action.run(async (signal) => {
              const job = await (
                await api("/jobs/" + task.id, { signal })
              ).json();
              return transfer(signal, job);
            })
          }
          onCancel={() => {
            action.cancel();
            void controls.run(async (signal) => {
              const value = await post<DeliveryJob>(
                "/jobs/" + task.id + "/cancel",
                {},
                signal,
              );
              if (!signal.aborted) update(value);
              return null;
            });
          }}
          onDownload={() =>
            void controls.run(async (signal) => {
              const response = await api("/jobs/" + task.id + "/archive", {
                signal,
              });
              const blob = await response.blob();
              if (!signal.aborted) saveBlob(blob, task.id + ".zip");
              return null;
            })
          }
        />
      ) : (
        <Alert message={t.noFiles} />
      )}
      <Card title={t.history}>
        <Space direction="vertical" style={{ width: "100%" }}>
          <Typography.Text type="secondary">{t.historyHint}</Typography.Text>
          <Input.Search
            aria-label={t.search}
            placeholder={t.search}
            onSearch={(value) => {
              setPage(1);
              setQ(value);
            }}
          />
          <ErrorNotice error={listing.error} />
          <Table<DeliveryJob>
            rowKey="id"
            loading={listing.busy}
            dataSource={listing.value?.items}
            scroll={{ x: 850 }}
            pagination={{
              current: page,
              pageSize: 10,
              showSizeChanger: false,
              total: listing.value?.total ?? 0,
              onChange: setPage,
            }}
            columns={[
              {
                title: t.taskID,
                dataIndex: "id",
                render: (value) => (
                  <Typography.Text style={{ overflowWrap: "anywhere" }}>
                    {value}
                  </Typography.Text>
                ),
              },
              {
                title: t.status,
                dataIndex: "status",
                render: (value) => taskLabel(t, value),
              },
              {
                title: t.verified,
                render: (_, job) => `${job.done}/${job.total}`,
              },
              { title: t.time, dataIndex: "created_at" },
              {
                title: t.open,
                render: (_, job) => (
                  <Button onClick={() => select(job)}>{t.open}</Button>
                ),
              },
            ]}
          />
        </Space>
      </Card>
    </Space>
  );
}
