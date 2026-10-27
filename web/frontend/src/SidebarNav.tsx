import { useWorkspace } from "./workspaceContext";
import { Menu } from "antd";
import {
  AppstoreOutlined,
  CopyOutlined,
  FileProtectOutlined,
  ScanOutlined,
  SettingOutlined,
  RightOutlined,
  DownOutlined,
} from "@ant-design/icons";
import { useText } from "./i18n";
import { useWorkbenchText } from "./workbenchText";
import { useJobText } from "./jobText";
import { useFeatureText } from "./traceText";
import { navigationGroups, canNavigate, type Page } from "./navigation";

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
  const workspace = useWorkspace();
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
    teams: t.teams,
  };
  const groups = {
    watermark: {
      label: t.navWatermark,
      icon: <FileProtectOutlined aria-hidden="true" />,
    },
    distribution: {
      label: t.navDistribution,
      icon: <CopyOutlined aria-hidden="true" />,
    },
    analysis: {
      label: t.navAnalysis,
      icon: <ScanOutlined aria-hidden="true" />,
    },
    system: {
      label: t.navSystem,
      icon: <SettingOutlined aria-hidden="true" />,
    },
  };
  return (
    <Menu
      aria-label={mobile ? t.mobileNavigation : t.primaryNavigation}
      mode="inline"
      // Avoid a pending hover timer reopening the popup after a leaf is selected.
      subMenuOpenDelay={0}
      inlineIndent={20}
      inlineCollapsed={collapsed}
      expandIcon={({ isOpen }) =>
        isOpen && !collapsed ? (
          <DownOutlined aria-hidden="true" style={{ fontSize: 12 }} />
        ) : (
          <RightOutlined aria-hidden="true" style={{ fontSize: 12 }} />
        )
      }
      selectedKeys={[page]}
      openKeys={openKeys}
      onOpenChange={onOpenChange}
      items={[
        {
          key: "overview",
          label: labels.overview,
          icon: <AppstoreOutlined aria-hidden="true" />,
        },
        ...navigationGroups
          .filter((group) =>
            group.pages.some((key) =>
              canNavigate(key, workspace.role, workspace.administrator),
            ),
          )
          .map((group) => ({
            key: group.key,
            ...groups[group.key],
            children: group.pages
              .filter((key) =>
                canNavigate(key, workspace.role, workspace.administrator),
              )
              .map((key) => ({ key, label: labels[key], title: labels[key] })),
          })),
      ]}
      onClick={({ key }) => onNavigate(key)}
      style={{ border: 0, background: "transparent" }}
    />
  );
}
