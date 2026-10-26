export interface Style {
  mode: "single" | "tile";
  position: "topLeft" | "topRight" | "center" | "bottomLeft" | "bottomRight";
  size: number;
  opacity: number;
  color: string;
  angle: number;
}
export interface Template {
  name: string;
  style: Style;
}
export const defaultStyle: Style = {
  mode: "single",
  position: "bottomRight",
  size: 28,
  opacity: 0.35,
  color: "#365bdb",
  angle: -30,
};
const storageKey = "watermark.visible.templates.v1";
export function validStyle(v: unknown): v is Style {
  const s = v as Style;
  return (
    !!s &&
    ["single", "tile"].includes(s.mode) &&
    ["topLeft", "topRight", "center", "bottomLeft", "bottomRight"].includes(
      s.position,
    ) &&
    Number.isFinite(s.size) &&
    s.size >= 12 &&
    s.size <= 96 &&
    Number.isFinite(s.opacity) &&
    s.opacity >= 0.05 &&
    s.opacity <= 1 &&
    /^#[0-9a-f]{6}$/i.test(s.color) &&
    Number.isFinite(s.angle) &&
    s.angle >= -90 &&
    s.angle <= 90
  );
}
export function loadTemplates(): Template[] {
  const raw = localStorage.getItem(storageKey);
  if (!raw) return [];
  const v = JSON.parse(raw);
  if (
    v.version !== 1 ||
    !Array.isArray(v.items) ||
    v.items.length > 20 ||
    v.items.some(
      (i: Template) =>
        !i ||
        typeof i.name !== "string" ||
        !i.name.trim() ||
        i.name.length > 40 ||
        !validStyle(i.style),
    )
  )
    throw Error("storage");
  return v.items;
}
export function storeTemplates(items: Template[]) {
  if (
    items.length > 20 ||
    items.some(
      (i) => !i.name.trim() || i.name.length > 40 || !validStyle(i.style),
    )
  )
    throw Error("storage");
  localStorage.setItem(
    storageKey,
    JSON.stringify({
      version: 1,
      items: items.map((i) => ({ name: i.name, style: i.style })),
    }),
  );
}
function dimensions(b: Uint8Array): [number, number] {
  const v = new DataView(b.buffer, b.byteOffset, b.byteLength);
  if (
    b.length >= 24 &&
    v.getUint32(0) === 0x89504e47 &&
    v.getUint32(4) === 0x0d0a1a0a
  )
    return [v.getUint32(16), v.getUint32(20)];
  if (b[0] === 255 && b[1] === 216) {
    let i = 2;
    while (i + 3 < b.length) {
      if (b[i++] !== 255) continue;
      while (b[i] === 255) i++;
      const marker = b[i++];
      if (marker === 0xda || marker === 0xd9) break;
      const length = (b[i] << 8) | b[i + 1];
      if (length < 2 || i + length > b.length) break;
      if (
        [
          0xc0, 0xc1, 0xc2, 0xc3, 0xc5, 0xc6, 0xc7, 0xc9, 0xca, 0xcb, 0xcd,
          0xce, 0xcf,
        ].includes(marker)
      )
        return [(b[i + 5] << 8) | b[i + 6], (b[i + 3] << 8) | b[i + 4]];
      i += length;
    }
  }
  throw Error("invalid");
}
export async function renderVisible(
  file: File,
  text: string,
  style: Style,
): Promise<Blob> {
  if (
    file.size > 50 * 1024 * 1024 ||
    !text.trim() ||
    Array.from(text).length > 80 ||
    /[\r\n\t\p{Cc}\p{Cf}]/u.test(text) ||
    !validStyle(style)
  )
    throw Error("invalid");
  const [w, h] = dimensions(new Uint8Array(await file.arrayBuffer()));
  if (w <= 0 || h <= 0 || w * h > 8_000_000) throw Error("invalid");
  const bitmap = await createImageBitmap(file);
  try {
    if (bitmap.width !== w || bitmap.height !== h) throw Error("invalid");
    const canvas = document.createElement("canvas");
    canvas.width = w;
    canvas.height = h;
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    if (!ctx) throw Error("invalid");
    ctx.drawImage(bitmap, 0, 0);
    const pixels = ctx.getImageData(0, 0, w, h).data;
    for (let i = 3; i < pixels.length; i += 4)
      if (pixels[i] !== 255) throw Error("invalid");
    await document.fonts.ready;
    ctx.font = `${style.size}px system-ui, sans-serif`;
    ctx.textBaseline = "middle";
    const tw = ctx.measureText(text).width;
    if (tw > w - 24 || style.size > h - 24) throw Error("font");
    // Reject glyphs that rasterize exactly like the missing-glyph box or replacement character.
    const glyph = document.createElement("canvas");
    glyph.width = 128;
    glyph.height = 128;
    const g = glyph.getContext("2d")!;
    g.font = "32px system-ui, sans-serif";
    g.textBaseline = "top";
    function raster(c: string) {
      g.clearRect(0, 0, 128, 128);
      g.fillText(c, 4, 4);
      return Array.from(g.getImageData(0, 0, 128, 128).data).join(",");
    }
    const missing = [raster("\uffff"), raster("\ufffd")];
    for (const c of new Set(text)) {
      if (!/\s/u.test(c) && missing.includes(raster(c))) throw Error("font");
    }
    ctx.fillStyle = style.color;
    ctx.globalAlpha = style.opacity;
    if (style.mode === "single") {
      let x = 12 + tw / 2,
        y = 12 + style.size / 2;
      if (style.position.includes("Right")) x = w - 12 - tw / 2;
      if (style.position.startsWith("bottom")) y = h - 12 - style.size / 2;
      if (style.position === "center") {
        x = w / 2;
        y = h / 2;
      }
      ctx.textAlign = "center";
      ctx.fillText(text, x, y);
    } else {
      ctx.save();
      ctx.translate(w / 2, h / 2);
      ctx.rotate((style.angle * Math.PI) / 180);
      ctx.textAlign = "center";
      const extent = Math.ceil(Math.hypot(w, h));
      const dx = tw + 48,
        dy = style.size * 3;
      for (let y = -extent; y <= extent; y += dy)
        for (let x = -extent; x <= extent; x += dx) ctx.fillText(text, x, y);
      ctx.restore();
    }
    return await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob(
        (b) =>
          b && b.size <= 50 * 1024 * 1024
            ? resolve(b)
            : reject(Error("invalid")),
        "image/png",
      ),
    );
  } finally {
    bitmap.close();
  }
}
