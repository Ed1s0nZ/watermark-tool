import { Menu } from "antd";
import {
  AppstoreOutlined,
  CopyOutlined,
  FileProtectOutlined,
  ScanOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import { useText } from "./i18n";
import { useWorkbenchText } from "./workbenchText";
import { useJobText } from "./jobText";
import { useFeatureText } from "./traceText";
import { navigationGroups, type Page } from "./navigation";

export function SidebarNav({
  page,
  openKeys,
  collapsed = false,
  onOpenChange,
  onNavigate,
  mobile = false,
}: {
  page: Page;
  openKeys: string[];
  collapsed?: boolean;
  onOpenChange: (keys: string[]) => void;
  onNavigate: (key: string) => void;
  mobile?: boolean;
}) {
  const t = useText();
  const wt = useWorkbenchText();
  const jt = useJobText();
  const ft = useFeatureText();
  const labels: Record<Page, string> = {
    overview: t.overview,
    add: t.add,
    dual: wt.dual,
    jobs: jt.jobs,
    batches: t.batches,
    records: t.records,
    extract: t.extract,
    trace: ft.trace,
    lab: wt.lab,
    keys: t.keys,
    audit: jt.audit,
    settings: t.settings,
  };
  const groups = {
    watermark: { label: t.navWatermark, icon: <FileProtectOutlined aria-hidden="true" /> },
    distribution: { label: t.navDistribution, icon: <CopyOutlined aria-hidden="true" /> },
    analysis: { label: t.navAnalysis, icon: <ScanOutlined aria-hidden="true" /> },
    system: { label: t.navSystem, icon: <SettingOutlined aria-hidden="true" /> },
  };
  return (
    <Menu
      aria-label={mobile ? t.mobileNavigation : t.primaryNavigation}
      mode="inline"
      // Avoid a pending hover timer reopening the popup after a leaf is selected.
      subMenuOpenDelay={0}
      inlineIndent={20}
      inlineCollapsed={collapsed}
      selectedKeys={[page]}
      openKeys={openKeys}
      onOpenChange={onOpenChange}
      items={[
        { key: "overview", label: labels.overview, icon: <AppstoreOutlined aria-hidden="true" /> },
        ...navigationGroups.map(group => ({
          key: group.key,
          ...groups[group.key],
          children: group.pages.map(key => ({ key, label: labels[key], title: labels[key] })),
        })),
      ]}
      onClick={({ key }) => onNavigate(key)}
      style={{ border: 0, background: "transparent" }}
    />
  );
}
