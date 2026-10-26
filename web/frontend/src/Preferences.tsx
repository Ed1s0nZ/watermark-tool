import { Select, Space } from "antd";
import { GlobalOutlined, SkinOutlined } from "@ant-design/icons";
import { useText } from "./i18n";
export type Mode = "light" | "dark" | "system";
export type Language = "zh" | "en";
export interface PreferencesProps {
  mode: Mode;
  setMode: (mode: Mode) => void;
  language: Language;
  setLanguage: (language: Language) => void;
}
export function Preferences({
  mode,
  setMode,
  language,
  setLanguage,
}: PreferencesProps) {
  const t = useText();
  return (
    <Space wrap size={8}>
      <Select
        aria-label={t.theme}
        value={mode}
        onChange={setMode}
        suffixIcon={<SkinOutlined aria-hidden="true" />}
        style={{ width: 128 }}
        options={(["light", "dark", "system"] as const).map((value) => ({
          value,
          label: t[value],
        }))}
      />
      <Select
        aria-label={t.language}
        value={language}
        onChange={setLanguage}
        suffixIcon={<GlobalOutlined aria-hidden="true" />}
        style={{ width: 112 }}
        options={[
          { value: "zh", label: "简体中文" },
          { value: "en", label: "English" },
        ]}
      />
    </Space>
  );
}
