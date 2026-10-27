import { configure, review } from "./editor-helpers";
import { navigate } from "./navigation";
import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import {
  login,
  choose,
  logoBytes,
  downloaded,
  drawRegion,
} from "./artwork-helpers";
test("transparent logo, exact visible exclusions, touch editor and JPEG final verification", async ({
  page,
}, info) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page);
  const original = await readFile("../../internal/blind/testdata/clean.png"),
    logo = await logoBytes(page);
  const rawHash = createHash("sha256").update(original).digest("hex"),
    logoHash = createHash("sha256").update(logo).digest("hex");
  await navigate(page, "双水印模板");
  await configure(page);
  await page.getByLabel("双水印原图", { exact: true }).setInputFiles({
    name: "source.png",
    mimeType: "image/png",
    buffer: original,
  });
  await page.getByLabel("隐形载荷", { exact: true }).fill("Logo-verified");
  await choose(page, "可见层组合", "仅Logo");
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: logo });
  await drawRegion(page);
  await expect(
    page.getByRole("spinbutton", { name: "区域 1 x %", exact: true }),
  ).toBeVisible();
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "生成预览", exact: true }),
  ).toBeVisible();
  const unchanged = await page.evaluate(
    async ({ raw }) => {
      const marked = await createImageBitmap(
        await (
          await fetch(
            (document.querySelector('img[alt="生成预览"]') as HTMLImageElement)
              .src,
          )
        ).blob(),
      );
      const source = await createImageBitmap(
        new Blob([new Uint8Array(raw)], { type: "image/png" }),
      );
      const pixels = (img: ImageBitmap) => {
        const c = document.createElement("canvas");
        c.width = img.width;
        c.height = img.height;
        const x = c.getContext("2d")!;
        x.drawImage(img, 0, 0);
        return x.getImageData(0, 0, c.width, c.height).data;
      };
      const a = pixels(source),
        b = pixels(marked);
      let inside = 0,
        outside = 0;
      for (let y = 0; y < source.height; y++)
        for (let x = 0; x < source.width; x++) {
          const offset = (y * source.width + x) * 4;
          const changed =
            a[offset] !== b[offset] ||
            a[offset + 1] !== b[offset + 1] ||
            a[offset + 2] !== b[offset + 2];
          if (
            x >= Math.floor(0.7 * source.width) &&
            x < Math.ceil(0.95 * source.width) &&
            y >= Math.floor(0.7 * source.height) &&
            y < Math.ceil(0.95 * source.height)
          ) {
            if (changed) inside++;
          } else if (changed) outside++;
        }
      source.close();
      marked.close();
      return { inside, outside };
    },
    { raw: Array.from(original) },
  );
  expect(unchanged.inside).toBe(0);
  expect(unchanged.outside).toBeGreaterThan(20);
  await page
    .getByRole("img", { name: "生成预览", exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: info.outputPath("logo-exclusion-preview.png"),
    fullPage: true,
    animations: "disabled",
  });
  const jpgLogo = Buffer.from(
    await page.evaluate(async (raw) => {
      const img = await createImageBitmap(
        new Blob([new Uint8Array(raw)], { type: "image/png" }),
      );
      const c = document.createElement("canvas");
      c.width = img.width;
      c.height = img.height;
      const x = c.getContext("2d")!;
      x.fillStyle = "#ffffff";
      x.fillRect(0, 0, c.width, c.height);
      x.drawImage(img, 0, 0);
      img.close();
      const b = await new Promise<Blob>((r) =>
        c.toBlob((b) => r(b!), "image/jpeg", 0.9),
      );
      return Array.from(new Uint8Array(await b.arrayBuffer()));
    }, Array.from(logo)),
  );
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({
      name: "logo.jpg",
      mimeType: "image/jpeg",
      buffer: jpgLogo,
    });
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "生成预览", exact: true }),
  ).toBeVisible();
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: logo });
  // Both layers are previewed from the same independent source.
  await choose(page, "可见层组合", "文字＋Logo");
  await choose(page, "Logo位置", "右上");
  await page.getByLabel("可见文字", { exact: true }).fill("内部资料 Alice");
  await choose(page, "成品格式", "JPEG");
  await page
    .getByRole("spinbutton", { name: "JPEG编码质量", exact: true })
    .fill("90");
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await page.getByRole("button", { name: "生成双水印", exact: true }).click();
  await expect(page.getByText("通过", { exact: true })).toBeVisible();
  let download = page.waitForEvent("download");
  await page.getByRole("link", { name: "下载图片", exact: true }).click();
  const d = await download;
  expect(d.suggestedFilename()).toMatch(/\.jpg$/);
  const jpeg = await downloaded(d);
  expect(jpeg.subarray(0, 2)).toEqual(Buffer.from([255, 216]));
  const verify = await page.request.post("/api/workbench/verify", {
    multipart: {
      file: { name: "actual.jpg", mimeType: "image/jpeg", buffer: jpeg },
    },
  });
  expect((await verify.json()).result.watermark).toBe("Logo-verified");
  await expect(
    page.getByText(new RegExp(createHash("sha256").update(jpeg).digest("hex"))),
  ).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 1000 });
  await page
    .locator(".ant-card")
    .filter({ has: page.getByText("质量摘要", { exact: true }) })
    .last()
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: info.outputPath("logo-jpeg-quality.png"),
    fullPage: true,
    animations: "disabled",
  });
  await choose(page, "样式", "平铺");
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "生成预览", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(async (raw) => {
      const marked = await createImageBitmap(
        await (
          await fetch(
            (document.querySelector('img[alt="生成预览"]') as HTMLImageElement)
              .src,
          )
        ).blob(),
      );
      const source = await createImageBitmap(
        new Blob([new Uint8Array(raw)], { type: "image/png" }),
      );
      const read = (img: ImageBitmap) => {
        const c = document.createElement("canvas");
        c.width = img.width;
        c.height = img.height;
        const x = c.getContext("2d")!;
        x.drawImage(img, 0, 0);
        return x.getImageData(
          Math.floor(img.width * 0.75),
          Math.floor(img.height * 0.75),
          Math.floor(img.width * 0.15),
          Math.floor(img.height * 0.15),
        ).data;
      };
      const a = read(source),
        b = read(marked);
      const same = a.every((v, i) => v === b[i]);
      source.close();
      marked.close();
      return same;
    }, Array.from(original)),
  ).toBe(true);
  await choose(page, "样式", "单处");
  await page.getByLabel("Logo文件", { exact: true }).setInputFiles({
    name: "broken.png",
    mimeType: "image/png",
    buffer: Buffer.from("broken"),
  });
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByText("Logo、布局或区域无效，请检查尺寸、透明度和矩形范围。", {
      exact: true,
    }),
  ).toBeVisible();
  await page.getByLabel("Logo文件", { exact: true }).setInputFiles({
    name: "large.png",
    mimeType: "image/png",
    buffer: Buffer.alloc(5 * 1024 * 1024 + 1),
  });
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByText("Logo、布局或区域无效，请检查尺寸、透明度和矩形范围。", {
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: logo });
  const oversizedLogo = Buffer.from(
    await page.evaluate(async () => {
      const c = document.createElement("canvas");
      c.width = 1024;
      c.height = 2048;
      c.getContext("2d")!.fillRect(0, 0, c.width, c.height);
      const b = await new Promise<Blob>((r) =>
        c.toBlob((b) => r(b!), "image/png"),
      );
      return Array.from(new Uint8Array(await b.arrayBuffer()));
    }),
  );
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({
      name: "pixels.png",
      mimeType: "image/png",
      buffer: oversizedLogo,
    });
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByText("Logo、布局或区域无效，请检查尺寸、透明度和矩形范围。", {
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: logo });
  // Full-image exclusion rejects before any task/output is created.
  await page.getByRole("button", { name: "清空本图区域", exact: true }).click();
  await drawRegion(page, 0.01, 0.01, 0.98, 0.98);
  for (const [k, value] of [
    ["x", "0"],
    ["y", "0"],
    ["width", "100"],
    ["height", "100"],
  ])
    await page
      .getByRole("spinbutton", { name: `区域 1 ${k} %`, exact: true })
      .fill(value);
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByText("可见水印没有可用完整位置，请缩小水印或调整避让区域。", {
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole("button", { name: "删除区域 1", exact: true }).click();
  // Real touch pointer stream in a 390px viewport, then numeric editing and clear.
  await page.setViewportSize({ width: 390, height: 844 });
  const canvas = page.getByTestId("exclusion-canvas");
  await canvas.scrollIntoViewIfNeeded();
  const b = (await canvas.boundingBox())!;
  const cdp = await page.context().newCDPSession(page);
  const start = { x: b.x + b.width * 0.2, y: b.y + b.height * 0.2 },
    end = { x: b.x + b.width * 0.5, y: b.y + b.height * 0.5 };
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchStart",
    touchPoints: [start],
  });
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchMove",
    touchPoints: [end],
  });
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchEnd",
    touchPoints: [],
  });
  await cdp.detach();
  await expect(
    page.getByRole("spinbutton", { name: "区域 1 width %", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("spinbutton", { name: "区域 1 width %", exact: true })
    .fill("25");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await choose(page, "语言", "English");
  await choose(page, "Appearance", "Dark");
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  await canvas.scrollIntoViewIfNeeded();
  await page.screenshot({
    path: info.outputPath("logo-exclusion-mobile.png"),
    fullPage: true,
    animations: "disabled",
  });
  for (let i = 0; i < 8; i++) await drawRegion(page, 0.1, 0.1, 0.03, 0.03);
  await expect(
    page.getByRole("spinbutton", { name: "Region 8 x %", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("spinbutton", { name: "Region 9 x %", exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("button", { name: "Clear image regions", exact: true })
    .click();
  expect(createHash("sha256").update(original).digest("hex")).toBe(rawHash);
  expect(createHash("sha256").update(logo).digest("hex")).toBe(logoHash);
  expect(errors).toEqual([]);
});
