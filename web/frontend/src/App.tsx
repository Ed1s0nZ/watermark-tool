import { useEffect, useState } from "react";
import { Button, ConfigProvider, Space, Spin, theme } from "antd";
import zhCN from "antd/locale/zh_CN";
import enUS from "antd/locale/en_US";
import { api, APIError, User } from "./api";
import { en, I18n, zh } from "./i18n";
import { Language, Mode } from "./Preferences";
import { Login } from "./Login";
import { Shell } from "./Shell";
function saved(key: string) {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}
function persist(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    /* Browsers can disable storage; preferences still work in memory. */
  }
}
export function App() {
  const [mode, setMode] = useState<Mode>(() => {
    const value = saved("watermark.theme");
    return value === "light" || value === "dark" ? value : "system";
  });
  const [language, setLanguage] = useState<Language>(() =>
    saved("watermark.language") === "en" ? "en" : "zh",
  );
  const [systemDark, setSystemDark] = useState(
    () => matchMedia("(prefers-color-scheme: dark)").matches,
  );
  const [user, setUser] = useState<User>();
  const [lastUser, setLastUser] = useState<User>();
  const [loading, setLoading] = useState(true);
  const [initialError, setInitialError] = useState<unknown>();
  const [expired, setExpired] = useState(false);
  const dark = mode === "dark" || (mode === "system" && systemDark);
  const t = language === "zh" ? zh : en;
  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemDark(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    persist("watermark.theme", mode);
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    document.documentElement.style.colorScheme = dark ? "dark" : "light";
  }, [mode, dark]);
  useEffect(() => {
    persist("watermark.language", language);
    document.documentElement.lang = language === "zh" ? "zh-CN" : "en";
    document.title =
      language === "zh"
        ? "Watermark · 水印平台"
        : "Watermark · Document workspace";
  }, [language]);
  useEffect(() => {
    const expire = () => {
      setUser(undefined);
      setExpired(true);
    };
    window.addEventListener("session-expired", expire);
    return () => window.removeEventListener("session-expired", expire);
  }, []);
  function onLogin(next: User) {
    setUser(next);
    setLastUser(next);
    setExpired(false);
    setInitialError(undefined);
  }
  async function session() {
    setLoading(true);
    setInitialError(undefined);
    try {
      const response = await api("/auth/session");
      onLogin((await response.json()).user);
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        setExpired(false);
      } else {
        setInitialError(e);
      }
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    void session();
  }, []);
  const preferences = { mode, setMode, language, setLanguage };
  return (
    <ConfigProvider
      locale={language === "zh" ? zhCN : enUS}
      theme={{
        algorithm: dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: "#3864e8",
          borderRadius: 10,
          fontFamily:
            'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
          colorBgLayout: dark ? "#10141d" : "#f5f7fb",
        },
        components: { Button: { controlHeight: 36 }, Card: { paddingLG: 24 } },
      }}
    >
      <I18n.Provider value={t}>
        {loading ? (
          <div className="connection-state">
            <Spin size="large" />
            <p>{t.loading}</p>
          </div>
        ) : (
          <>
            {!user && (
              <>
                <Login
                  {...preferences}
                  onLogin={onLogin}
                  expired={expired}
                  initialError={initialError}
                />
                {initialError && (
                  <div className="connection-retry">
                    <Space direction="vertical">
                      <Button onClick={() => void session()}>{t.retry}</Button>
                    </Space>
                  </div>
                )}
              </>
            )}
            {lastUser && (
              <div hidden={!user}>
                <Shell
                  user={lastUser}
                  {...preferences}
                  onLogout={async () => {
                    await api("/auth/logout", { method: "POST" });
                    setUser(undefined);
                    setLastUser(undefined);
                    setExpired(false);
                    location.hash = "overview";
                  }}
                />
              </div>
            )}
          </>
        )}
      </I18n.Provider>
    </ConfigProvider>
  );
}
