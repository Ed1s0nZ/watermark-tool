import { useRef, useState, type ReactNode } from "react";
import {
  Button,
  Card,
  Collapse,
  Empty,
  Segmented,
  Space,
  Typography,
} from "antd";
import { useWorkspaceText } from "./workspaceText";
export function FormStage({
  number,
  title,
  children,
}: {
  number: number;
  title: string;
  children: ReactNode;
}) {
  return (
    <Card
      className="form-stage"
      title={
        <span className="form-stage-title">
          <span className="form-stage-number">{number}</span>
          {title}
        </span>
      }
    >
      {children}
    </Card>
  );
}
export function AdvancedSettings({ children }: { children: ReactNode }) {
  const t = useWorkspaceText();
  return (
    <Collapse
      className="advanced-settings"
      items={[
        {
          key: "advanced",
          label: (
            <span>
              {t.advanced}
              <Typography.Text type="secondary" className="advanced-hint">
                {t.advancedHint}
              </Typography.Text>
            </span>
          ),
          forceRender: true,
          children,
        },
      ]}
    />
  );
}
export function EmptyPreview() {
  const t = useWorkspaceText();
  return (
    <div className="empty-preview">
      <Empty
        description={t.emptyPreview}
        image={Empty.PRESENTED_IMAGE_SIMPLE}
      />
      <Typography.Paragraph type="secondary">
        {t.previewHint}
      </Typography.Paragraph>
    </div>
  );
}
export function FormWorkspace({
  configuration,
  preview,
}: {
  configuration: ReactNode;
  preview: ReactNode;
}) {
  const t = useWorkspaceText();
  const [panel, setPanel] = useState("configuration");
  const root = useRef<HTMLDivElement>(null);
  function switchTo(next: string) {
    setPanel(next);
    const scroll = root.current?.closest(".content");
    if (scroll && root.current)
      scroll.scrollTop +=
        root.current.getBoundingClientRect().top -
        scroll.getBoundingClientRect().top;
  }
  return (
    <div className="form-workspace" ref={root} data-panel={panel}>
      <div className="workspace-mobile-switch">
        <Segmented
          block
          aria-label={t.view}
          value={panel}
          onChange={switchTo}
          options={[
            { value: "configuration", label: t.configuration },
            { value: "preview", label: t.previewView },
          ]}
        />
      </div>
      <div className="form-workspace-grid">
        <div className="workspace-configuration">
          <Space direction="vertical" size={20} style={{ width: "100%" }}>
            {configuration}
          </Space>
          <Button
            className="workspace-mobile-next"
            type="primary"
            block
            onClick={() => switchTo("preview")}
          >
            {t.previewView}
          </Button>
        </div>
        <aside className="workspace-preview" aria-label={t.previewView}>
          <Button
            className="workspace-mobile-back"
            onClick={() => switchTo("configuration")}
          >
            {t.back}
          </Button>
          <Space direction="vertical" size={20} style={{ width: "100%" }}>
            {preview}
          </Space>
        </aside>
      </div>
    </div>
  );
}
