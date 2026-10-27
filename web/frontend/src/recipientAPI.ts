import { api } from "./api";
export interface Contact {
  id: string;
  team_id: string;
  recipient_id: string;
  name: string;
  groups: string[];
  revision: number;
  updated_at: string;
}
export interface ContactSearch {
  items: Contact[];
  total: number;
  groups: string[];
  team_total: number;
}
export const contactPath = (team: string) =>
  `/workspace/teams/${encodeURIComponent(team)}/recipients`;
export async function contactJSON<T>(
  path: string,
  value: unknown,
  signal?: AbortSignal,
  method = "POST",
): Promise<T> {
  const response = await api(path, {
    method,
    signal,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(value),
  });
  return response.status === 204 ? (undefined as T) : response.json();
}
export function downloadRecipients(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob),
    link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
