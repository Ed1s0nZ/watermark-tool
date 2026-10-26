import { useEffect, useState } from "react";
import { Alert } from "antd";
import { api } from "./api";
import { useJobText } from "./jobText";
export function AuditBanner({ active }: { active: boolean }) {
  const t = useJobText();
  const [state, setState] = useState<{
    configured: boolean;
    degraded: boolean;
    task_degraded: boolean;
  }>();
  useEffect(() => {
    if (!active) return;
    const controller = new AbortController();
    let pending = false;
    const refresh = async () => {
      if (pending) return;
      pending = true;
      try {
        const value = await (
          await api("/audit/status", { signal: controller.signal })
        ).json();
        if (!controller.signal.aborted) setState(value);
      } catch {
      } finally {
        pending = false;
      }
    };
    const warning = () =>
      setState((prev) => ({
        ...prev,
        configured: true,
        degraded: true,
        task_degraded: prev?.task_degraded ?? false,
      }));
    window.addEventListener("audit-degraded", warning);
    void refresh();
    const timer = setInterval(refresh, 5000);
    return () => {
      controller.abort();
      clearInterval(timer);
      window.removeEventListener("audit-degraded", warning);
    };
  }, [active]);
  return state?.degraded || state?.task_degraded ? (
    <Alert
      type="error"
      showIcon
      message={state.degraded ? t.degraded : t.taskDegraded}
      style={{ marginBottom: 20 }}
    />
  ) : null;
}
