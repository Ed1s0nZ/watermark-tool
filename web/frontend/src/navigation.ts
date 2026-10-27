export const navigationGroups = [
  { key: "watermark", pages: ["add", "dual"] },
  { key: "distribution", pages: ["jobs", "batches", "records"] },
  { key: "analysis", pages: ["extract", "trace", "lab"] },
  { key: "system", pages: ["keys", "audit", "settings"] },
] as const;
export type GroupKey = (typeof navigationGroups)[number]["key"];
export type Page = "overview" | (typeof navigationGroups)[number]["pages"][number];
export function isPage(value: string): value is Page {
  return value === "overview" || navigationGroups.some(group =>
    (group.pages as readonly string[]).includes(value));
}
export function parentGroup(page: Page): GroupKey | undefined {
  return navigationGroups.find(group =>
    (group.pages as readonly string[]).includes(page))?.key;
}
