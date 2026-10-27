export interface Approval {
  team_id: string;
  job_id: string;
  operator: string;
  requester: string;
  snapshot: string;
  revision: number;
  status: "pending" | "approved" | "rejected" | "withdrawn";
  reviewer: string;
  changes: {
    status: Approval["status"];
    actor: string;
    reason: string;
    at: string;
  }[];
}
export const approvalBase = (team: string) =>
  `/workspace/teams/${encodeURIComponent(team)}/approvals`;
