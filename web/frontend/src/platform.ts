import { useEffect, useRef, useState } from "react";
import { api } from "./api";
export interface KeyVersion {
  id: string;
  state: "active" | "decrypt-only" | "disabled";
  created_at: string;
  updated_at: string;
  created_by: string;
}
export interface CopyRecord {
  generated?: boolean;
  job_id?: string;
  source_id?: string;
  source_name?: string;
  rendered_sha256?: string;
  id: string;
  recipient_id: string;
  name: string;
  batch_id: string;
  key_id: string;
  key_state: string;
  algorithm: string;
  created_at: string;
  operator: string;
  status: string;
  source_sha256: string;
}
export interface BatchRecord {
  id: string;
  key_id: string;
  created_at: string;
  operator: string;
  status: string;
  error?: string;
  attempts: number;
  source_sha256: string;
  copies: { id: string; recipient_id: string; name: string }[];
}
export interface TraceResult {
  watermark: string;
  timestamp: string;
  algorithm: string;
  key_id: string;
  match: "registered" | "unregistered";
  record: CopyRecord | null;
}
export async function jsonRequest(path: string, data: unknown) {
  const response = await api(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  return response.json();
}
export function useArchive() {
  const [archive, setArchive] = useState<{ url: string; id: string }>();
  const url = useRef<string | undefined>(undefined);
  const mounted = useRef(true);
  const clear = () => {
    if (url.current) URL.revokeObjectURL(url.current);
    url.current = undefined;
    setArchive(undefined);
  };
  useEffect(
    () => () => {
      mounted.current = false;
      if (url.current) URL.revokeObjectURL(url.current);
    },
    [],
  );
  const receive = async (response: Response) => {
    const blob = await response.blob();
    if (!mounted.current) return;
    clear();
    url.current = URL.createObjectURL(blob);
    setArchive({
      url: url.current,
      id: response.headers.get("X-Batch-ID") || "copies",
    });
  };
  return { archive, clear, receive };
}
