import { createContext, useContext } from "react";
import { Select, Tooltip } from "antd";
import { useCollaborationText } from "./collaborationText";
export type TeamRole = "owner" | "operator" | "reviewer";
export interface Team {
  id: string;
  name: string;
  enabled: boolean;
  members: Record<string, TeamRole>;
  revision: number;
  updated_at: string;
}
export interface Account {
  username: string;
  enabled: boolean;
  revision: number;
  updated_at: string;
}
export interface WorkspaceSession {
  username: string;
  administrator: boolean;
  environment_admin: string;
  teams: Team[];
}
export interface WorkspaceContextValue {
  administrator: boolean;
  environmentAdmin: string;
  role: TeamRole;
  team?: Team;
  teams: Team[];
  available: boolean;
  select: (id: string) => void;
  refresh: () => Promise<void>;
}
export const WorkspaceContext = createContext<WorkspaceContextValue>({
  administrator: true,
  environmentAdmin: "",
  role: "owner",
  teams: [],
  available: false,
  select: () => {},
  refresh: async () => {},
});
export const useWorkspace = () => useContext(WorkspaceContext);
export function WorkspaceSelector() {
  const t = useCollaborationText();
  const context = useWorkspace();
  if (!context.available) return null;
  return (
    <Tooltip title={t.switchHint}>
      <Select
        className="workspace-selector"
        aria-label={t.scope}
        value={context.team?.id}
        onChange={context.select}
        options={context.teams
          .filter((team) => team.enabled)
          .map((team) => ({
            value: team.id,
            label: team.id === "personal" ? t.personal : team.name,
          }))}
      />
    </Tooltip>
  );
}
