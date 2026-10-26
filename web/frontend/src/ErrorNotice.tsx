import { Alert } from "antd";
import { APIError } from "./api";
import { useText } from "./i18n";
export function ErrorNotice({ error }: { error: unknown }) {
  const t = useText();
  if (!error) return null;
  const code = error instanceof APIError ? error.code : "requestError";
  const message = code in t ? t[code as keyof typeof t] : t.requestError;
  return <Alert role="alert" type="error" showIcon message={message} />;
}
