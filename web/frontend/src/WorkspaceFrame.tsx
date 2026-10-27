import { useEffect, useRef, useState } from "react";
import { Button, Result, Space, Spin } from "antd";
import {
  api,
  APIError,
  advanceAuthEpoch,
  setEngineTeam,
  type User,
} from "./api";
import { Shell } from "./Shell";
import type { PreferencesProps } from "./Preferences";
import { ErrorNotice } from "./ErrorNotice";
import { WorkspaceContext, type WorkspaceSession } from "./workspaceContext";
import { useCollaborationText } from "./collaborationText";
export function WorkspaceFrame(
  props: PreferencesProps & {
    user: User;
    active?: boolean;
    onLogout: () => Promise<void>;
  },
) {
  const t = useCollaborationText();
  const [session, setSession] = useState<WorkspaceSession>();
  const [selected, setSelected] = useState("");
  const [ready, setReady] = useState(false),
    [available, setAvailable] = useState(false),
    [error, setError] = useState<unknown>();
  const current = useRef("");
  const mounted = useRef(true);
  const generation = useRef(0);
  function select(id: string) {
    if (id === current.current) return;
    current.current = id;
    setEngineTeam(id, props.user.username);
    advanceAuthEpoch();
    setSelected(id);
    try {
      localStorage.setItem(`watermark.team.${props.user.username}`, id);
    } catch {}
  }
  async function refresh(signal?: AbortSignal) {
    const requested = ++generation.current;
    try {
      const response = await api("/workspace/me", { signal });
      const next = (await response.json()) as WorkspaceSession;
      if (
        !mounted.current ||
        signal?.aborted ||
        requested !== generation.current
      )
        return;
      setSession(next);
      setAvailable(true);
      setError(undefined);
      let stored = "";
      try {
        stored =
          localStorage.getItem(`watermark.team.${props.user.username}`) ?? "";
      } catch {}
      const wanted = current.current || stored;
      const valid = next.teams.find(
        (team) => team.id === wanted && team.enabled,
      );
      select(
        valid?.id ??
          (next.administrator
            ? next.teams.find((team) => team.id === "personal" && team.enabled)
                ?.id
            : undefined) ??
          next.teams.find((team) => team.enabled)?.id ??
          "",
      );
      setReady(true);
    } catch (e) {
      if (
        !mounted.current ||
        signal?.aborted ||
        requested !== generation.current
      )
        return;
      if (
        e instanceof APIError &&
        [404, 503].includes(e.status) &&
        props.user.administrator !== false
      ) {
        setAvailable(false);
        setSession({
          username: props.user.username,
          administrator: true,
          environment_admin: props.user.username,
          teams: [],
        });
        select("personal");
        setReady(true);
      } else setError(e);
    }
  }
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      setEngineTeam("");
    };
  }, []);
  useEffect(() => {
    if (props.active === false) return;
    const controller = new AbortController();
    void refresh(controller.signal);
    const timer = setInterval(() => void refresh(controller.signal), 30000);
    return () => {
      controller.abort();
      clearInterval(timer);
    };
  }, [props.active]);
  if (!ready)
    return (
      <div className="connection-state">
        <Spin tip={t.loading} />
        <ErrorNotice error={error} />
        {!!error && (
          <Space>
            <Button onClick={() => void refresh()}>{t.retry}</Button>
            <Button onClick={() => void props.onLogout()}>{t.logout}</Button>
          </Space>
        )}
      </div>
    );
  const team = session?.teams.find((item) => item.id === selected);
  const administrator = session?.administrator ?? false;
  if (available && !team)
    return (
      <Result
        status="info"
        title={t.noTeam}
        subTitle={t.contactAdmin}
        extra={
          <Space>
            <Button onClick={() => void refresh()}>{t.refresh}</Button>
            <Button onClick={() => void props.onLogout()}>{t.logout}</Button>
          </Space>
        }
      />
    );
  return (
    <WorkspaceContext.Provider
      value={{
        administrator,
        environmentAdmin: session?.environment_admin ?? props.user.username,
        role: administrator
          ? "owner"
          : (team?.members[props.user.username] ?? "reviewer"),
        team,
        teams: session?.teams ?? [],
        available,
        select,
        refresh: () => refresh(),
      }}
    >
      <Shell {...props} key={`${props.user.username}/${selected}`} />
    </WorkspaceContext.Provider>
  );
}
