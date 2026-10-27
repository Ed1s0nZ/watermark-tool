import { scopedBrowserKey } from "./api";
import { Layout, validLayout } from "./artwork";
import { OutputOptions, validOutput } from "./OutputFields";
import { defaultStyle, validStyle, Style } from "./visibleWatermark";
import { validQuality, QualityPolicy } from "./quality";
export interface PolicyValue {
  layout?: Layout;
  output?: OutputOptions;
  mode: "blind" | "dual";
  template: string;
  style: Style;
  quality: QualityPolicy;
}
export interface Preset extends PolicyValue {
  id: string;
  name: string;
}
export const policyKey = "watermark.policies.v1";
const keys = (v: object, list: string[]) =>
  Object.keys(v).sort().join(",") === list.sort().join(",");
export function validPolicy(v: unknown): v is PolicyValue {
  if (!v || typeof v !== "object" || Array.isArray(v)) return false;
  const p = v as PolicyValue;
  if (
    !Object.keys(p).every((k) =>
      ["mode", "template", "style", "quality", "layout", "output"].includes(k),
    ) ||
    !["mode", "template", "style", "quality"].every((k) =>
      Object.hasOwn(p, k),
    ) ||
    !["blind", "dual"].includes(p.mode) ||
    typeof p.template !== "string" ||
    Array.from(p.template).length > 80 ||
    (!p.template.trim() && p.layout?.layers !== "logo") ||
    /[{}]/.test(p.template.replace(/\{(name|recipient_id)\}/g, "")) ||
    /[\p{Cc}\p{Cf}]/u.test(p.template)
  )
    return false;
  return (
    validStyle(p.style) &&
    validQuality(p.quality) &&
    (p.layout === undefined || validLayout(p.layout)) &&
    (p.output === undefined || validOutput(p.output))
  );
}
export function loadPolicies(): Preset[] {
  const raw = localStorage.getItem(scopedBrowserKey(policyKey));
  if (!raw) return [];
  const v = JSON.parse(raw);
  if (
    !v ||
    !keys(v, ["version", "items"]) ||
    v.version !== 1 ||
    !Array.isArray(v.items) ||
    v.items.length > 20
  )
    throw Error("storage");
  const ids = new Set<string>(),
    names = new Set<string>();
  for (const p of v.items) {
    if (
      !p ||
      typeof p !== "object" ||
      !Object.keys(p).every((k) =>
        [
          "id",
          "name",
          "mode",
          "template",
          "style",
          "quality",
          "layout",
          "output",
        ].includes(k),
      ) ||
      typeof p.id !== "string" ||
      !/^[a-f0-9-]{36}$/.test(p.id) ||
      typeof p.name !== "string" ||
      !p.name.trim() ||
      p.name.length > 40 ||
      ids.has(p.id) ||
      names.has(p.name)
    )
      throw Error("storage");
    const { id, name, ...value } = p;
    if (!validPolicy(value)) throw Error("storage");
    ids.add(id);
    names.add(name);
  }
  return v.items;
}
export function savePolicies(items: Preset[]) {
  // Validate through the same strict parser without touching storage before validation.
  if (
    items.length > 20 ||
    new Set(items.map((p) => p.name)).size !== items.length ||
    new Set(items.map((p) => p.id)).size !== items.length ||
    items.some(
      ({ id, name, ...p }) =>
        !validPolicy(p) || !id || !name.trim() || name.length > 40,
    )
  )
    throw Error("storage");
  localStorage.setItem(
    scopedBrowserKey(policyKey),
    JSON.stringify({ version: 1, items }),
  );
}
export function builtins(labels: string[]): Preset[] {
  return [
    {
      id: "internal",
      name: labels[0],
      mode: "dual",
      template: "{name} · {recipient_id}",
      style: { ...defaultStyle, mode: "tile" },
      quality: {},
    },
    {
      id: "preview",
      name: labels[1],
      mode: "dual",
      template: "Preview · {name}",
      style: { ...defaultStyle, opacity: 0.25 },
      quality: {},
    },
    {
      id: "public",
      name: labels[2],
      mode: "blind",
      template: "{name}",
      style: { ...defaultStyle },
      quality: {},
    },
  ];
}
