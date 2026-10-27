export const navigationGroups = [
  { key: "watermark", pages: ["add", "dual"] },
  { key: "distribution", pages: ["jobs", "batches", "records", "recipients", "approvals"] },
  { key: "analysis", pages: ["extract", "trace", "lab"] },
  { key: "system", pages: ["keys", "audit", "teams", "settings"] },
] as const;
export type GroupKey = (typeof navigationGroups)[number]["key"];
export type Page =
  | "overview"
  | (typeof navigationGroups)[number]["pages"][number];
export function isPage(value: string): value is Page {
  return (
    value === "overview" ||
    navigationGroups.some((group) =>
      (group.pages as readonly string[]).includes(value),
    )
  );
}
export function parentGroup(page: Page): GroupKey | undefined {
  return navigationGroups.find((group) =>
    (group.pages as readonly string[]).includes(page),
  )?.key;
}

export function canNavigate(page: Page, role: string, administrator: boolean) {
  if (administrator) return true;
  if (page === "keys") return role === "owner";
  return (
    role !== "reviewer" ||
    !["add", "dual", "jobs", "batches", "lab"].includes(page)
  );
}
