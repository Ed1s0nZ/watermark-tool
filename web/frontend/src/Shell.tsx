import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Avatar,
  Button,
  Drawer,
  Dropdown,
  Layout,
  Space,
  Typography,
  theme,
} from "antd";
import {
  LogoutOutlined,
  MenuFoldOutlined,
  MenuOutlined,
  MenuUnfoldOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import { api, User } from "./api";
import { useText } from "./i18n";
import { Preferences, PreferencesProps } from "./Preferences";
import { Dashboard } from "./Dashboard";
import { WatermarkPage } from "./WatermarkPage";
import { LabPage } from "./LabPage";
import { DualPage } from "./DualPage";
import { useWorkbenchText } from "./workbenchText";
import { TraceBatchPage } from "./TraceBatchPage";
import { useFeatureText } from "./traceText";
import { JobsPage } from "./JobsPage";
import { AuditPage } from "./AuditPage";
import { AuditBanner } from "./AuditBanner";
import { useJobText } from "./jobText";
import { Settings } from "./Settings";
import { BatchPage } from "./BatchPage";
import { RecordsPage } from "./RecordsPage";
import { KeysPage } from "./KeysPage";
import { ErrorNotice } from "./ErrorNotice";
import { SidebarNav } from "./SidebarNav";
import { isPage, parentGroup, type Page } from "./navigation";
function currentPage(): Page {
  const page = location.hash.slice(1);
  return isPage(page) ? page : "overview";
}
export function Shell({
  user,
  active = true,
  onLogout,
  ...preferences
}: PreferencesProps & {
  user: User;
  active?: boolean;
  onLogout: () => Promise<void>;
}) {
  const t = useText();
  const wt = useWorkbenchText();
  const jt = useJobText();
  const ft = useFeatureText();
  const contentRef = useRef<HTMLDivElement>(null);
  const { token } = theme.useToken();
  const [page, setPage] = useState<Page>(currentPage);
  const [collapsed, setCollapsed] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [openGroups, setOpenGroups] = useState<string[]>(() => {
    const parent = parentGroup(currentPage());
    return parent ? [parent] : [];
  });
  const [popupGroups, setPopupGroups] = useState<string[]>([]);
  useEffect(() => {
    const parent = parentGroup(page);
    if (parent) {
      setOpenGroups(keys => keys.includes(parent) ? keys : [...keys, parent]);
    }
    setPopupGroups([]);
  }, [page, collapsed]);
  const [types, setTypes] = useState<string[]>([]);
  const [typeError, setTypeError] = useState(false);
  const [loading, setLoading] = useState(false);
  const [logoutError, setLogoutError] = useState<unknown>();
  const [signingOut, setSigningOut] = useState(false);
  useEffect(() => {
    const change = () => setPage(currentPage());
    window.addEventListener("hashchange", change);
    return () => window.removeEventListener("hashchange", change);
  }, []);
  async function loadTypes() {
    setTypeError(false);
    setLoading(true);
    try {
      const response = await api("/supported-types");
      const data = await response.json();
      if (!Array.isArray(data.types)) throw new Error();
      setTypes(
        data.types.filter((type: unknown) => typeof type === "string").sort(),
      );
    } catch {
      setTypeError(true);
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void loadTypes();
  }, []);
  const navigate = (value: string) => {
    if (!isPage(value)) return;
    location.hash = value;
    setPage(value);
    const parent = parentGroup(value);
    if (parent) {
      setOpenGroups(keys => keys.includes(parent) ? keys : [...keys, parent]);
    }
    setPopupGroups([]);
    setMobile(false);
  };
  useEffect(() => {
    contentRef.current?.scrollTo({ top: 0 });
  }, [page]);
  const brand = (
    <a href="#overview" className="brand sidebar-brand">
      <span className="brand-mark">
        <SafetyCertificateOutlined aria-hidden="true" />
      </span>
      {!collapsed && (
        <span>
          {t.brand}
          <small>WORKSPACE</small>
        </span>
      )}
    </a>
  );
  async function logout() {
    setSigningOut(true);
    setLogoutError(undefined);
    try {
      await onLogout();
    } catch (e) {
      setLogoutError(e);
    } finally {
      setSigningOut(false);
    }
  }
  return (
    <Layout className="platform">
      <Layout.Sider
        className="desktop-sidebar"
        width={224}
        collapsedWidth={80}
        collapsed={collapsed}
        style={{
          background: token.colorBgContainer,
          borderRight: `1px solid ${token.colorBorderSecondary}`,
        }}
      >
        {brand}
        <div className="sidebar-section">
          {!collapsed ? t.workspace : "···"}
        </div>
        <div className="sidebar-navigation">
          <SidebarNav page={page} collapsed={collapsed}
            openKeys={collapsed ? popupGroups : openGroups}
            onOpenChange={collapsed ? setPopupGroups : setOpenGroups}
            onNavigate={navigate} />
        </div>
        <div className="sidebar-bottom">
          <div className="sidebar-status">
            <span className="status-dot" />
            {!collapsed && t.secureArea}
          </div>
          <Button
            block
            type="text"
            aria-label={collapsed ? t.expand : t.collapse}
            icon={
              collapsed ? (
                <MenuUnfoldOutlined aria-hidden="true" />
              ) : (
                <MenuFoldOutlined aria-hidden="true" />
              )
            }
            onClick={() => setCollapsed(!collapsed)}
          >
            {!collapsed && t.collapse}
          </Button>
        </div>
      </Layout.Sider>
      <Drawer
        title={t.brand}
        placement="left"
        width="min(320px, calc(100vw - 32px))"
        open={mobile}
        onClose={() => setMobile(false)}
        closeIcon={<span aria-label={t.close}>×</span>}
      >
        <SidebarNav page={page} mobile openKeys={openGroups}
          onOpenChange={setOpenGroups} onNavigate={navigate} />
      </Drawer>
      <Layout className="platform-main">
        <Layout.Header
          className="topbar"
          style={{
            background: token.colorBgContainer,
            borderBottom: `1px solid ${token.colorBorderSecondary}`,
          }}
        >
          <Space>
            <Button
              className="mobile-menu"
              aria-label={t.menu}
              type="text"
              icon={<MenuOutlined aria-hidden="true" />}
              onClick={() => setMobile(true)}
            />
            <Typography.Text strong>
              {page === "lab"
                ? wt.lab
                : page === "dual"
                  ? wt.dual
                  : page === "jobs"
                    ? jt.jobs
                    : page === "audit"
                      ? jt.audit
                      : page === "trace"
                        ? ft.trace
                        : t[page]}
            </Typography.Text>
          </Space>
          <div className="topbar-actions">
            <Preferences {...preferences} />
            <Dropdown
              trigger={["click"]}
              menu={{
                items: [
                  {
                    key: "settings",
                    label: t.settings,
                    icon: <SettingOutlined aria-hidden="true" />,
                    onClick: () => navigate("settings"),
                  },
                  { type: "divider" },
                  {
                    key: "logout",
                    label: t.logout,
                    icon: <LogoutOutlined aria-hidden="true" />,
                    disabled: signingOut,
                    onClick: () => void logout(),
                  },
                ],
              }}
            >
              <Button
                type="text"
                className="account-button"
                aria-label={t.account}
              >
                <Avatar
                  size={28}
                  style={{
                    background: token.colorPrimaryBg,
                    color: token.colorPrimary,
                  }}
                  icon={<UserOutlined aria-hidden="true" />}
                />
                <span className="account-name">{user.username}</span>
              </Button>
            </Dropdown>
          </div>
        </Layout.Header>
        <Layout.Content className="content" ref={contentRef}>
          <div className="content-inner">
            <AuditBanner active={active} />
            <ErrorNotice error={logoutError} />
            {typeError && (
              <Alert
                className="format-error"
                type="error"
                showIcon
                message={t.formatError}
                action={
                  <Button loading={loading} onClick={() => void loadTypes()}>
                    {t.retry}
                  </Button>
                }
              />
            )}
            <div hidden={page !== "overview"}>
              <Dashboard types={types} onNavigate={navigate} />
            </div>
            <div hidden={page !== "add"}>
              <WatermarkPage kind="add" types={types} />
            </div>
            <div hidden={page !== "extract"}>
              <WatermarkPage kind="extract" types={types} />
            </div>
            <div hidden={page !== "batches"}>
              <BatchPage />
            </div>
            <div hidden={page !== "records"}>
              <RecordsPage active={active && page === "records"} />
            </div>
            <div hidden={page !== "keys"}>
              <KeysPage active={active && page === "keys"} />
            </div>
            {active && page === "trace" && <TraceBatchPage />}
            {active && page === "jobs" && <JobsPage />}
            {active && page === "audit" && <AuditPage />}
            {active && page === "lab" && <LabPage active />}
            {active && page === "dual" && <DualPage active />}
            <div hidden={page !== "settings"}>
              <Settings username={user.username} {...preferences} />
            </div>
          </div>
        </Layout.Content>
        <Layout.Footer className="platform-footer">{t.footer}</Layout.Footer>
      </Layout>
    </Layout>
  );
}
