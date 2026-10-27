import { useEffect, useRef, useState } from "react";
import { Alert, Button, Space, Tag, Typography } from "antd";
import { FileOutlined, InboxOutlined, PlusOutlined } from "@ant-design/icons";
import { useWorkspaceText } from "./workspaceText";
export type FileRejection = "invalid" | "large" | "many" | "totalLarge";
export const imageTypes = ".png,.jpg,.jpeg";
export function fileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / (bytes < 1024 * 1024 ? 1024 : 1024 * 1024)).toFixed(1)} ${bytes < 1024 * 1024 ? "KB" : "MB"}`;
}
function accepts(file: File, accept: string) {
  return accept.split(",").some((item) => {
    const type = item.trim().toLowerCase();
    return type.startsWith(".")
      ? file.name.toLowerCase().endsWith(type)
      : type.endsWith("/*")
        ? file.type.toLowerCase().startsWith(type.slice(0, -1))
        : file.type.toLowerCase() === type;
  });
}
function Thumbnail({ file }: { file: File }) {
  const t = useWorkspaceText();
  const [url, setURL] = useState("");
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setFailed(false);
    if (!/\.(png|jpe?g)$/i.test(file.name)) {
      setURL("");
      return;
    }
    const next = URL.createObjectURL(file);
    setURL(next);
    return () => URL.revokeObjectURL(next);
  }, [file]);
  return (
    <span className="file-thumbnail">
      {url && !failed ? (
        <img
          src={url}
          alt={`${t.thumbnail}: ${file.name}`}
          onError={() => setFailed(true)}
        />
      ) : (
        <FileOutlined aria-hidden="true" />
      )}
    </span>
  );
}
export function FileUpload({
  label,
  value,
  onChange,
  accept = imageTypes,
  maxSize = 50 * 1024 * 1024,
  maxCount = 1,
  maxTotal,
  disabled = false,
  onReject,
  showError = true,
}: {
  label: string;
  value: File[];
  onChange: (files: File[]) => void;
  accept?: string;
  maxSize?: number;
  maxCount?: number;
  maxTotal?: number;
  disabled?: boolean;
  onReject?: (reason: FileRejection) => void;
  showError?: boolean;
}) {
  const t = useWorkspaceText();
  const input = useRef<HTMLInputElement>(null);
  const mode = useRef<"replace" | "append" | number>("replace");
  const [error, setError] = useState<FileRejection>();
  const [dragging, setDragging] = useState(false);
  const multiple = maxCount > 1;
  function choose(next: "replace" | "append" | number) {
    if (disabled) return;
    mode.current = next;
    if (input.current) {
      input.current.multiple = multiple && typeof next !== "number";
      input.current.click();
    }
  }
  function receive(files: File[], action: "replace" | "append" | number) {
    if (disabled || !files.length) return;
    const next =
      typeof action === "number"
        ? value.flatMap((file, index) => (index === action ? files : [file]))
        : action === "append"
          ? [...value, ...files]
          : files;
    const rejection: FileRejection | undefined = next.some(
      (file) => !accepts(file, accept),
    )
      ? "invalid"
      : next.some((file) => file.size > maxSize)
        ? "large"
        : next.length > maxCount
          ? "many"
          : maxTotal !== undefined &&
              next.reduce((sum, file) => sum + file.size, 0) > maxTotal
            ? "totalLarge"
            : undefined;
    if (rejection) {
      setError(rejection);
      onReject?.(rejection);
      return;
    }
    setError(undefined);
    onChange(next);
  }
  return (
    <div className="file-upload" data-upload-label={label}>
      <input
        ref={input}
        type="file"
        hidden
        aria-label={label}
        accept={accept}
        multiple={multiple}
        disabled={disabled}
        onChange={(event) => {
          const files = Array.from(event.currentTarget.files ?? []);
          event.currentTarget.value = "";
          receive(files, mode.current);
          mode.current = "replace";
          event.currentTarget.multiple = multiple;
        }}
      />
      <div
        role="button"
        tabIndex={disabled ? -1 : 0}
        aria-label={`${t.select}: ${label}`}
        aria-disabled={disabled}
        className={`file-dropzone${dragging ? " dragging" : ""}${disabled ? " disabled" : ""}`}
        onClick={() => choose("replace")}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            choose("replace");
          }
        }}
        onDragOver={(event) => {
          event.preventDefault();
          if (!disabled) setDragging(true);
        }}
        onDragLeave={(event) => {
          if (
            !(event.relatedTarget instanceof Node) ||
            !event.currentTarget.contains(event.relatedTarget)
          )
            setDragging(false);
        }}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          receive(
            Array.from(event.dataTransfer.files),
            multiple ? "append" : "replace",
          );
        }}
      >
        <InboxOutlined className="file-drop-icon" aria-hidden="true" />
        <strong>{t.drop}</strong>
        <Typography.Text type="secondary">
          {t.format}: {accept} · {t.each}: {fileSize(maxSize)}
        </Typography.Text>
        {multiple && (
          <Typography.Text type="secondary">
            {t.count}: {maxCount}
            {maxTotal ? ` · ${t.total}: ${fileSize(maxTotal)}` : ""}
          </Typography.Text>
        )}
      </div>
      <Typography.Paragraph type="secondary" className="upload-local-note">
        {t.local}
      </Typography.Paragraph>
      {accept.includes(".csv") && (
        <Typography.Paragraph type="secondary" className="upload-local-note">
          {t.csvNote}
        </Typography.Paragraph>
      )}
      {showError && error && <Alert showIcon type="error" message={t[error]} />}
      <ul className="selected-files" aria-label={`${t.selected}: ${label}`}>
        {value.map((file, index) => (
          <li key={index} className="selected-file">
            <Thumbnail file={file} />
            <div className="file-metadata">
              <Typography.Text className="file-name" title={file.name}>
                {file.name}
              </Typography.Text>
              <Space size={4} wrap>
                <Tag>{file.name.split(".").pop()?.toUpperCase() || "FILE"}</Tag>
                <Typography.Text type="secondary">
                  {fileSize(file.size)}
                </Typography.Text>
              </Space>
            </div>
            <Space size={0} className="file-actions">
              <Button
                type="text"
                size="small"
                disabled={disabled}
                aria-label={`${t.replace}: ${label}: ${file.name}`}
                onClick={() => choose(index)}
              >
                {t.replace}
              </Button>
              <Button
                type="text"
                size="small"
                danger
                disabled={disabled}
                aria-label={`${t.remove}: ${label}: ${file.name}`}
                onClick={() => {
                  setError(undefined);
                  onChange(value.filter((_, n) => n !== index));
                }}
              >
                {t.remove}
              </Button>
            </Space>
          </li>
        ))}
      </ul>
      {multiple && (
        <Button
          icon={<PlusOutlined />}
          disabled={disabled || value.length >= maxCount}
          onClick={() => choose("append")}
          aria-label={`${t.add}: ${label}`}
        >
          {t.add}
        </Button>
      )}
    </div>
  );
}
