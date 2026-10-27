import {
  dimensions,
  Style,
  checkVisibleText,
  renderVisible,
} from "./visibleWatermark";
export interface Rect {
  x: number;
  y: number;
  width: number;
  height: number;
}
export interface LogoLayout {
  position: Style["position"];
  width_ratio: number;
  opacity: number;
}
export interface Layout {
  layers: "text" | "logo" | "both";
  logo: LogoLayout;
}
export interface Artwork {
  layers: Layout["layers"];
  logo?: LogoLayout & { sha256: string };
  exclusions?: Record<string, Rect[]>;
}
export interface ArtworkState extends Layout {
  logoFile?: File;
  expectedSHA?: string;
  exclusions: Record<string, Rect[]>;
}
export const logoDefault: LogoLayout = {
  position: "bottomRight",
  width_ratio: 0.15,
  opacity: 0.35,
};
export function emptyArtwork(): ArtworkState {
  return { layers: "text", logo: { ...logoDefault }, exclusions: {} };
}
export function validLayout(v: unknown): v is Layout {
  if (!v || typeof v !== "object" || Array.isArray(v)) return false;
  const p = v as Layout;
  return (
    Object.keys(p).sort().join() === "layers,logo" &&
    ["text", "logo", "both"].includes(p.layers) &&
    !!p.logo &&
    Object.keys(p.logo).sort().join() === "opacity,position,width_ratio" &&
    positions.includes(p.logo.position) &&
    Number.isFinite(p.logo.width_ratio) &&
    p.logo.width_ratio >= 0.05 &&
    p.logo.width_ratio <= 0.4 &&
    Number.isFinite(p.logo.opacity) &&
    p.logo.opacity >= 0.05 &&
    p.logo.opacity <= 1
  );
}
export const positions: Style["position"][] = [
  "topLeft",
  "topRight",
  "center",
  "bottomLeft",
  "bottomRight",
];
export function validRect(r: Rect) {
  return (
    [r.x, r.y, r.width, r.height].every(Number.isFinite) &&
    r.x >= 0 &&
    r.y >= 0 &&
    r.width > 0 &&
    r.height > 0 &&
    r.x + r.width <= 1.000000001 &&
    r.y + r.height <= 1.000000001
  );
}
const descriptions = new WeakMap<
  File,
  Promise<{ sha256: string; width: number; height: number }>
>();
export function describe(file: File) {
  let p = descriptions.get(file);
  if (!p) {
    p = (async () => {
      if (file.size > 50 * 1024 * 1024) throw Error("artwork_invalid");
      const data = await file.arrayBuffer();
      const [width, height] = dimensions(new Uint8Array(data));
      if (width <= 0 || height <= 0 || width * height > 8_000_000)
        throw Error("artwork_invalid");
      const digest = await crypto.subtle.digest("SHA-256", data);
      return {
        width,
        height,
        sha256: Array.from(new Uint8Array(digest), (b) =>
          b.toString(16).padStart(2, "0"),
        ).join(""),
      };
    })().catch(() => {
      throw Error("artwork_invalid");
    });
    descriptions.set(file, p);
  }
  return p;
}
export async function metadata(
  value: ArtworkState,
  keys: string[],
): Promise<Artwork | undefined> {
  if (!validLayout({ layers: value.layers, logo: value.logo }))
    throw Error("artwork_invalid");
  const exclusions: Record<string, Rect[]> = {};
  for (const key of keys) {
    const rs = value.exclusions[key] ?? [];
    if (rs.length > 8 || rs.some((r) => !validRect(r)))
      throw Error("artwork_invalid");
    if (rs.length) exclusions[key] = rs.map((r) => ({ ...r }));
  }
  let logo: Artwork["logo"];
  if (value.layers !== "text") {
    if (!value.logoFile || value.logoFile.size > 5 * 1024 * 1024)
      throw Error("artwork_invalid");
    const l = await describe(value.logoFile);
    if (l.width * l.height > 2_000_000) throw Error("artwork_invalid");
    if (value.expectedSHA && value.expectedSHA !== l.sha256)
      throw Error("artwork_logo_mismatch");
    logo = { ...value.logo, sha256: l.sha256 };
  }
  if (value.layers === "text" && !Object.keys(exclusions).length)
    return undefined;
  return {
    layers: value.layers,
    ...(logo ? { logo } : {}),
    ...(Object.keys(exclusions).length ? { exclusions } : {}),
  };
}
function point(
  position: Style["position"],
  w: number,
  h: number,
  iw: number,
  ih: number,
) {
  return {
    x:
      position === "center"
        ? w / 2
        : position.includes("Right")
          ? w - 12 - iw / 2
          : 12 + iw / 2,
    y:
      position === "center"
        ? h / 2
        : position.startsWith("bottom")
          ? h - 12 - ih / 2
          : 12 + ih / 2,
  };
}
async function decodeBitmap(file: File) {
  try {
    return await createImageBitmap(file);
  } catch {
    throw Error("artwork_invalid");
  }
}
export async function renderArtwork(
  file: File,
  text: string,
  style: Style,
  state: ArtworkState,
): Promise<Blob> {
  const info = await describe(file);
  const art = await metadata(state, [info.sha256]);
  if (!art) return renderVisible(file, text, style);
  const w = info.width,
    h = info.height;
  const bitmap = await decodeBitmap(file);
  let logo: ImageBitmap | undefined;
  try {
    if (bitmap.width !== w || bitmap.height !== h)
      throw Error("artwork_invalid");
    const canvas = document.createElement("canvas");
    canvas.width = w;
    canvas.height = h;
    const base = canvas.getContext("2d", { willReadFrequently: true })!;
    base.drawImage(bitmap, 0, 0);
    const raw = base.getImageData(0, 0, w, h).data;
    for (let i = 3; i < raw.length; i += 4)
      if (raw[i] !== 255) throw Error("artwork_invalid");
    const layer = document.createElement("canvas");
    layer.width = w;
    layer.height = h;
    const ctx = layer.getContext("2d")!;
    const regions = (art.exclusions?.[info.sha256] ?? []).map((r) => ({
      x: Math.floor(r.x * w),
      y: Math.floor(r.y * h),
      width: Math.ceil((r.x + r.width) * w) - Math.floor(r.x * w),
      height: Math.ceil((r.y + r.height) * h) - Math.floor(r.y * h),
    }));
    const blocked = (b: Rect) =>
      regions.some(
        (r) =>
          b.x < r.x + r.width &&
          b.x + b.width > r.x &&
          b.y < r.y + r.height &&
          b.y + b.height > r.y,
      );
    if (art.layers !== "logo") {
      const tw = await checkVisibleText(ctx, text, style, w, h);
      ctx.fillStyle = style.color;
      ctx.globalAlpha = style.opacity;
      ctx.textAlign = "center";
      const m = ctx.measureText(text);
      const box = {
        x: -m.actualBoundingBoxLeft - 3,
        y: -m.actualBoundingBoxAscent - 3,
        width: m.actualBoundingBoxLeft + m.actualBoundingBoxRight + 6,
        height: m.actualBoundingBoxAscent + m.actualBoundingBoxDescent + 6,
      };
      let count = 0;
      if (style.mode === "single") {
        for (const p of [
          style.position,
          ...positions.filter((p) => p !== style.position),
        ]) {
          const c = point(p, w, h, tw, style.size);
          if (blocked({ ...box, x: box.x + c.x, y: box.y + c.y })) continue;
          ctx.fillText(text, c.x, c.y);
          count++;
          break;
        }
      } else {
        const a = (style.angle * Math.PI) / 180,
          cos = Math.cos(a),
          sin = Math.sin(a),
          extent = Math.ceil(Math.hypot(w, h));
        for (let y = -extent; y <= extent; y += style.size * 3)
          for (let x = -extent; x <= extent; x += tw + 48) {
            const corners = [
              [box.x, box.y],
              [box.x + box.width, box.y],
              [box.x, box.y + box.height],
              [box.x + box.width, box.y + box.height],
            ].map(([bx, by]) => ({
              x: w / 2 + (x + bx) * cos - (y + by) * sin,
              y: h / 2 + (x + bx) * sin + (y + by) * cos,
            }));
            const xs = corners.map((p) => p.x),
              ys = corners.map((p) => p.y);
            const b = {
              x: Math.min(...xs),
              y: Math.min(...ys),
              width: Math.max(...xs) - Math.min(...xs),
              height: Math.max(...ys) - Math.min(...ys),
            };
            if (
              b.x < 0 ||
              b.y < 0 ||
              b.x + b.width > w ||
              b.y + b.height > h ||
              blocked(b)
            )
              continue;
            ctx.save();
            ctx.translate(w / 2, h / 2);
            ctx.rotate(a);
            ctx.fillText(text, x, y);
            ctx.restore();
            count++;
          }
      }
      if (!count) throw Error("artwork_blocked");
    }
    if (art.logo) {
      logo = await decodeBitmap(state.logoFile!);
      const test = document.createElement("canvas");
      test.width = logo.width;
      test.height = logo.height;
      const g = test.getContext("2d")!;
      g.drawImage(logo, 0, 0);
      if (
        !g
          .getImageData(0, 0, logo.width, logo.height)
          .data.some((v, i) => i % 4 === 3 && v > 0)
      )
        throw Error("artwork_invalid");
      const lw = w * art.logo.width_ratio,
        lh = (lw * logo.height) / logo.width;
      if (lw > w - 24 || lh > h - 24) throw Error("artwork_blocked");
      let drawn = false;
      for (const p of [
        art.logo.position,
        ...positions.filter((p) => p !== art.logo!.position),
      ]) {
        const c = point(p, w, h, lw, lh);
        const b = {
          x: c.x - lw / 2 - 2,
          y: c.y - lh / 2 - 2,
          width: lw + 4,
          height: lh + 4,
        };
        if (blocked(b)) continue;
        ctx.globalAlpha = art.logo.opacity;
        ctx.drawImage(logo, c.x - lw / 2, c.y - lh / 2, lw, lh);
        drawn = true;
        break;
      }
      if (!drawn) throw Error("artwork_blocked");
    }
    ctx.globalAlpha = 1;
    for (const r of regions) ctx.clearRect(r.x, r.y, r.width, r.height);
    base.drawImage(layer, 0, 0);
    return await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob(
        (b) =>
          b && b.size <= 50 * 1024 * 1024
            ? resolve(b)
            : reject(Error("artwork_invalid")),
        "image/png",
      ),
    );
  } finally {
    bitmap.close();
    logo?.close();
  }
}
