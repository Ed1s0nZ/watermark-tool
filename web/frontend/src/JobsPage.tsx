import { scopedBrowserKey } from "./api";
import {
  AdvancedSettings,
  EmptyPreview,
  FormStage,
  FormWorkspace,
} from "./FormWorkspace";
import { useWorkspaceText } from "./workspaceText";
import { FileUpload } from "./FileUpload";
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
import { ArtworkControls } from "./ArtworkControls";
import { ExclusionEditor } from "./ExclusionEditor";
import { ArtworkState, emptyArtwork, logoDefault } from "./artwork";
import { OutputFields, OutputOptions, normalizedOutput } from "./OutputFields";
import { PolicyControls } from "./PolicyControls";
import { QualityFields, QualityPolicy } from "./quality";
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
    const id = localStorage.getItem(scopedBrowserKey(selectionKey)) ?? "";
    return /^[a-f0-9]{32}$/.test(id) ? id : "";
  } catch {
    return "";
  }
}
export function JobsPage() {
  const t = useJobText();
  const ui = useWorkspaceText();
  const [files, setFiles] = useState<File[]>([]);
  const [csv, setCSV] = useState("recipient_id,name\n001,Alice\n002,张三");
  const [mode, setMode] = useState<"blind" | "dual">("blind");
  const [text, setText] = useState("内部资料 · {name} · {recipient_id}");
  const [artwork, setArtwork] = useState<ArtworkState>(emptyArtwork);
  const [output, setOutput] = useState<OutputOptions>({ format: "png" });
  const [quality, setQuality] = useState<QualityPolicy>({});
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
  const csvImport = useJob<null>();
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
  const [csvFile, setCSVFile] = useState<File>();
  const busy = plan.busy || action.busy || controls.busy || csvImport.busy;
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
      localStorage.setItem(scopedBrowserKey(selectionKey), job.id);
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
    setOutput(normalizedOutput(job.output));
    setArtwork({
      layers: job.artwork?.layers ?? "text",
      logo: job.artwork?.logo
        ? {
            position: job.artwork.logo.position,
            width_ratio: job.artwork.logo.width_ratio,
            opacity: job.artwork.logo.opacity,
          }
        : { ...logoDefault },
      expectedSHA: job.artwork?.logo?.sha256,
      exclusions: structuredClone(job.artwork?.exclusions ?? {}),
    });
    setQuality(job.quality_policy ? { ...job.quality_policy } : {});
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
      <FormWorkspace
        configuration={
          <>
            <FormStage number={1} title={ui.materials}>
              <Form layout="vertical">
                <Form.Item label={t.files}>
                  <FileUpload
                    label={t.files}
                    value={files}
                    disabled={busy}
                    maxCount={5}
                    maxTotal={128 * 1024 * 1024}
                    onChange={(files) => {
                      change();
                      setFiles(files);
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
                <Form.Item>
                  <FileUpload
                    label={t.importCSV}
                    accept=".csv,text/csv"
                    maxSize={16384}
                    value={csvFile ? [csvFile] : []}
                    disabled={busy}
                    onChange={(files) => {
                      const file = files[0];
                      setCSVFile(file);
                      csvImport.reset();
                      if (!file) return;
                      change();
                      void csvImport.run(async (signal) => {
                        let data: string;
                        try {
                          data = new TextDecoder("utf-8", {
                            fatal: true,
                          }).decode(await file.arrayBuffer());
                        } catch {
                          throw new APIError(422, "job_invalid");
                        }
                        if (!signal.aborted) setCSV(data);
                        return null;
                      });
                    }}
                  />
                </Form.Item>
              </Form>
              <ErrorNotice error={csvImport.error} />
            </FormStage>
            <FormStage number={2} title={ui.configure}>
              <Form layout="vertical">
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
                  </>
                )}
                {mode === "dual" && (
                  <ArtworkControls
                    value={artwork}
                    disabled={busy}
                    onChange={(v) => {
                      change();
                      setArtwork(v);
                    }}
                  />
                )}
              </Form>
              <AdvancedSettings>
                <PolicyControls
                  value={{
                    mode,
                    template: text,
                    style,
                    quality,
                    layout: { layers: artwork.layers, logo: artwork.logo },
                    output,
                  }}
                  disabled={busy}
                  onApply={(p) => {
                    change();
                    setMode(p.mode);
                    setText(p.template);
                    setStyle(p.style);
                    setQuality(p.quality);
                    setOutput(normalizedOutput(p.output));
                    setArtwork({
                      ...emptyArtwork(),
                      ...(p.layout ?? {}),
                      exclusions: artwork.exclusions,
                    });
                  }}
                />

                <Form layout="vertical">
                  {mode === "dual" && (
                    <>
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
                  {mode === "dual" && (
                    <ExclusionEditor
                      files={files}
                      value={artwork.exclusions}
                      disabled={busy}
                      onChange={(v) => {
                        change();
                        setArtwork({ ...artwork, exclusions: v });
                      }}
                    />
                  )}
                  <OutputFields
                    value={output}
                    disabled={busy}
                    onChange={(v) => {
                      change();
                      setOutput(v);
                    }}
                  />
                  <QualityFields
                    value={quality}
                    disabled={busy}
                    onChange={(v) => {
                      change();
                      setQuality(v);
                    }}
                  />
                </Form>
                {storageError && <Alert type="warning" message={t.invalid} />}
              </AdvancedSettings>
            </FormStage>
          </>
        }
        preview={
          <>
            <FormStage number={3} title={ui.preview}>
              {!plan.value && <EmptyPreview />}
              {plan.value && <JobPreview plan={plan.value} />}
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
                      quality,
                      artwork,
                      output,
                    ).catch((e) => {
                      if (e instanceof APIError) throw e;
                      if (
                        e instanceof Error &&
                        e.message.startsWith("artwork_")
                      )
                        throw new APIError(422, e.message);
                      if (e instanceof Error && e.message === "font")
                        throw new APIError(422, "job_font");
                      throw new APIError(422, "job_invalid");
                    }),
                  )
                }
              >
                {t.prepare}
              </Button>
              {progress && (
                <Typography.Paragraph role="status">
                  {progress}
                </Typography.Paragraph>
              )}
              {(plan.cancelled || action.cancelled) && (
                <Alert message={t.stopped} />
              )}
              <ErrorNotice error={plan.error} />
              <ErrorNotice error={action.error} />
              <ErrorNotice error={controls.error} />
            </FormStage>
            <FormStage number={4} title={ui.deliver}>
              <Typography.Paragraph type="secondary">
                {ui.waiting}
              </Typography.Paragraph>
              <Space wrap>
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
            </FormStage>
          </>
        }
      />
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
