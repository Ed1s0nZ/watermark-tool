export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    public detail = "",
  ) {
    super(code);
  }
}
let authEpoch = 0;
export function advanceAuthEpoch() {
  authEpoch++;
}
export async function api(path: string, init?: RequestInit): Promise<Response> {
  const requestedEpoch = authEpoch;
  let response: Response;
  try {
    response = await fetch(`/api${path}`, {
      ...init,
      credentials: "same-origin",
    });
  } catch {
    throw new APIError(0, "network");
  }
  if (
    requestedEpoch === authEpoch &&
    response.headers.get("X-Audit-Degraded") === "true"
  )
    window.dispatchEvent(new Event("audit-degraded"));
  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    if (
      requestedEpoch === authEpoch &&
      response.status === 401 &&
      path !== "/auth/login"
    )
      window.dispatchEvent(new Event("session-expired"));
    throw new APIError(
      response.status,
      data.code ||
        (response.status === 429
          ? "tooMany"
          : response.status === 413
            ? "tooLarge"
            : "requestError"),
      data.error || "",
    );
  }
  return response;
}
export interface User {
  username: string;
}
