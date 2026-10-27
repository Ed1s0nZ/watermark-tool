import { test, expect, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { login, choose } from "./artwork-helpers";
import { configure, review } from "./editor-helpers";

const fixture = "../../internal/blind/testdata/clean.png";
async function dropImage(
  page: Page,
  label: string,
  name: string,
  bytes: Buffer,
) {
  const data = await page.evaluateHandle(
    ({ name, bytes }) => {
      const transfer = new DataTransfer();
      transfer.items.add(
        new File([new Uint8Array(bytes)], name, { type: "image/png" }),
      );
      return transfer;
    },
    { name, bytes: Array.from(bytes) },
  );
  await page
    .locator(`[data-upload-label="${label}"] .file-dropzone`)
    .dispatchEvent("drop", { dataTransfer: data });
  await data.dispose();
}

test("layered dual form: sticky preview, mobile state, local upload and stale result invalidation", async ({
  page,
}, info) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(() => {
    const original = URL.revokeObjectURL;
    (window as any).revokedURLs = [];
    URL.revokeObjectURL = (url) => {
      (window as any).revokedURLs.push(url);
      original.call(URL, url);
    };
  });
  await page.setViewportSize({ width: 1280, height: 900 });
  await login(page);
  await page.goto("/#dual");
  const advanced = page.locator(".advanced-settings .ant-collapse-header");
  await expect(advanced).toHaveAttribute("aria-expanded", "false");
  await expect(page.getByLabel("可见文字", { exact: true })).toBeVisible();
  const upload = page.locator('[data-upload-label="双水印原图"]');
  const original = await readFile(fixture);
  let processing = 0;
  page.on("request", (request) => {
    if (request.method() === "POST" && request.url().includes("/workbench/"))
      processing++;
  });
  await dropImage(page, "双水印原图", "source.png", original);
  await expect(upload.getByText("source.png", { exact: true })).toBeVisible();
  const thumb = upload.getByRole("img");
  await expect(thumb).toBeVisible();
  const oldURL = await thumb.getAttribute("src");
  await page.getByLabel("可见文字", { exact: true }).fill("Team");
  await page.getByLabel("隐形载荷", { exact: true }).fill("workspace-2026");
  expect(processing).toBe(0);
  await configure(page);
  await page.getByRole("spinbutton", { name: "字号", exact: true }).fill("20");
  await advanced.click();
  await expect(advanced).toHaveAttribute("aria-expanded", "false");
  await page.locator(".content").evaluate((el) => (el.scrollTop = 280));
  const bounds = await page.locator(".workspace-preview").boundingBox();
  const content = await page.locator(".content").boundingBox();
  const config = await page.locator(".workspace-configuration").boundingBox();
  expect(bounds!.x).toBeGreaterThan(config!.x + config!.width);
  const padding = await page
    .locator(".content")
    .evaluate((el) => parseFloat(getComputedStyle(el).paddingTop));
  expect(Math.abs(bounds!.y - content!.y - padding)).toBeLessThan(3);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "生成预览", exact: true }),
  ).toBeVisible();
  await page.locator(".content").evaluate((el) => (el.scrollTop = 0));
  await page.screenshot({
    path: info.outputPath("dual-split-desktop.png"),
    animations: "disabled",
  });
  await page.setViewportSize({ width: 1024, height: 768 });
  await review(page);
  await expect(page.locator(".workspace-preview")).toBeVisible();
  await expect(page.locator(".workspace-configuration")).not.toBeVisible();
  await configure(page, false);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.setViewportSize({ width: 390, height: 844 });
  // Resize keeps configuration and the existing preview in the same component tree.
  await expect(page.getByLabel("可见文字", { exact: true })).toHaveValue(
    "Team",
  );
  await review(page);
  await expect(
    page.getByRole("img", { name: "生成预览", exact: true }),
  ).toBeVisible();
  await configure(page, false);
  await expect(page.getByLabel("隐形载荷", { exact: true })).toHaveValue(
    "workspace-2026",
  );
  await review(page);
  await page.route("**/api/workbench/dual", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 500));
    await route.continue();
  });
  await page.getByRole("button", { name: "生成双水印", exact: true }).click();
  await expect(upload.locator(".file-actions button").first()).toBeDisabled();
  await expect(upload.locator(".file-actions button").last()).toBeDisabled();
  await expect(
    page.getByRole("link", { name: "下载图片", exact: true }),
  ).toBeVisible();
  expect(processing).toBe(1);
  await configure(page, false);
  await choose(page, "语言", "English");
  await choose(page, "Appearance", "Dark");
  await review(page);
  await expect(
    page.getByRole("link", { name: "Download image", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("dual-mobile-preview-dark.png"),
    animations: "disabled",
  });
  await page
    .getByRole("link", { name: "Download image", exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    path: info.outputPath("dual-mobile-result-dark.png"),
    animations: "disabled",
  });
  await configure(page, false);
  const englishUpload = page.locator(
    '[data-upload-label="Dual watermark source"]',
  );
  await page
    .getByLabel("Dual watermark source", { exact: true })
    .setInputFiles({
      name: "bad.txt",
      mimeType: "text/plain",
      buffer: Buffer.from("bad"),
    });
  await expect(englishUpload.getByRole("alert")).toContainText(
    "Unsupported format",
  );
  await expect(
    englishUpload.getByText("source.png", { exact: true }),
  ).toBeVisible();
  // A rejected choice keeps the accepted file and its verified result.
  await review(page);
  await expect(
    page.getByRole("link", { name: "Download image", exact: true }),
  ).toBeVisible();
  await configure(page, false);
  const chooser = page.waitForEvent("filechooser");
  await englishUpload.getByRole("button", { name: /^Replace:/ }).click();
  await (
    await chooser
  ).setFiles({
    name: "replacement.png",
    mimeType: "image/png",
    buffer: original,
  });
  await expect(
    englishUpload.getByText("replacement.png", { exact: true }),
  ).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate((url) => (window as any).revokedURLs.includes(url), oldURL),
    )
    .toBe(true);
  await review(page);
  await expect(
    page.getByRole("link", { name: "Download image", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("img", { name: "Create preview", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Generate dual watermark", exact: true }),
  ).toBeDisabled();
  await configure(page, false);
  await englishUpload.getByRole("button", { name: /^Remove:/ }).click();
  await expect(englishUpload.locator(".selected-file")).toHaveCount(0);
  await configure(page);
  await expect(
    page.getByRole("spinbutton", { name: "Font size", exact: true }),
  ).toHaveValue("20");
  const keyboard = page.waitForEvent("filechooser");
  const zone = englishUpload.getByRole("button", {
    name: "Select files: Dual watermark source",
    exact: true,
  });
  await zone.focus();
  await zone.press("Enter");
  await (
    await keyboard
  ).setFiles({ name: "source.png", mimeType: "image/png", buffer: original });
  await expect(
    englishUpload.getByText("source.png", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
});

test("shared upload: multi-file management, atomic limits, CSV and legacy document formats", async ({
  page,
}, info) => {
  test.setTimeout(120000);
  await login(page);
  await page.goto("/#jobs");
  const advanced = page.locator(".advanced-settings .ant-collapse-header");
  await expect(advanced).toHaveAttribute("aria-expanded", "false");
  const raw = await readFile(fixture);
  const input = page.getByLabel("任务原图", { exact: true });
  const upload = page.locator('[data-upload-label="任务原图"]');
  await input.setInputFiles([
    { name: "one.png", mimeType: "image/png", buffer: raw },
    { name: "two.png", mimeType: "image/png", buffer: raw },
  ]);
  await expect(upload.locator(".selected-file")).toHaveCount(2);
  await input.setInputFiles(
    Array.from({ length: 6 }, (_, n) => ({
      name: `${n}.png`,
      mimeType: "image/png",
      buffer: raw,
    })),
  );
  await expect(upload.getByRole("alert")).toContainText("数量超限");
  await expect(upload.locator(".selected-file")).toHaveCount(2);
  // Client-size validation uses real File objects; no processing request is sent.
  await input.evaluate((el: HTMLInputElement) => {
    const data = new DataTransfer();
    data.items.add(
      new File([new Uint8Array(50 * 1024 * 1024 + 1)], "large.png", {
        type: "image/png",
      }),
    );
    el.files = data.files;
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await expect(upload.getByRole("alert")).toContainText("文件过大");
  await expect(upload.locator(".selected-file")).toHaveCount(2);
  await input.evaluate((el: HTMLInputElement) => {
    const data = new DataTransfer();
    const bytes = new Uint8Array(45 * 1024 * 1024);
    for (let n = 0; n < 3; n++)
      data.items.add(new File([bytes], `${n}.png`, { type: "image/png" }));
    el.files = data.files;
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await expect(upload.getByRole("alert")).toContainText("合计大小超限");
  await expect(upload.locator(".selected-file")).toHaveCount(2);
  const add = page.waitForEvent("filechooser");
  await upload
    .getByRole("button", { name: "添加文件: 任务原图", exact: true })
    .click();
  await (
    await add
  ).setFiles({ name: "three.png", mimeType: "image/png", buffer: raw });
  await dropImage(page, "任务原图", "four.png", raw);
  await expect(upload.locator(".selected-file")).toHaveCount(4);
  const replace = page.waitForEvent("filechooser");
  await upload
    .getByRole("button", { name: "替换: 任务原图: two.png", exact: true })
    .click();
  await (
    await replace
  ).setFiles({ name: "new.png", mimeType: "image/png", buffer: raw });
  await expect(upload.locator(".selected-file")).toHaveCount(4);
  await expect(upload.getByText("two.png", { exact: true })).toHaveCount(0);
  await upload
    .getByRole("button", { name: "删除: 任务原图: one.png", exact: true })
    .click();
  await expect(upload.locator(".selected-file")).toHaveCount(3);
  const csv = page.getByLabel("导入任务名单", { exact: true });
  await csv.setInputFiles({
    name: "people.csv",
    mimeType: "text/csv",
    buffer: Buffer.from("recipient_id,name\n01,Alice"),
  });
  await expect(
    page.getByRole("textbox", { name: "任务接收人 CSV", exact: true }),
  ).toHaveValue("recipient_id,name\n01,Alice");
  await page
    .getByRole("button", {
      name: "删除: 导入任务名单: people.csv",
      exact: true,
    })
    .click();
  await expect(
    page.getByRole("textbox", { name: "任务接收人 CSV", exact: true }),
  ).toHaveValue("recipient_id,name\n01,Alice");
  await csv.setInputFiles({
    name: "large.csv",
    mimeType: "text/csv",
    buffer: Buffer.alloc(16385),
  });
  await expect(
    page.locator('[data-upload-label="导入任务名单"]').getByRole("alert"),
  ).toContainText("文件过大");
  await page.screenshot({
    path: info.outputPath("multi-upload-limits.png"),
    animations: "disabled",
  });
  await csv.setInputFiles({
    name: "invalid.csv",
    mimeType: "text/csv",
    buffer: Buffer.from([0xff, 0xfe]),
  });
  await expect(
    page.getByText(
      "请检查图片数量/格式/容量、名单和可见文字；过长文字或缺字请先调整。",
      { exact: true },
    ),
  ).toBeVisible();
  await input.setInputFiles({
    name: "one.png",
    mimeType: "image/png",
    buffer: raw,
  });
  await csv.setInputFiles({
    name: "people.csv",
    mimeType: "text/csv",
    buffer: Buffer.from("recipient_id,name\n01,Alice"),
  });
  await expect(
    page.getByRole("textbox", { name: "任务接收人 CSV", exact: true }),
  ).toHaveValue("recipient_id,name\n01,Alice");
  await review(page);
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "任务副本预览", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "创建并开始任务", exact: true }),
  ).toBeEnabled();
  await configure(page, false);
  await upload
    .getByRole("button", { name: "删除: 任务原图: one.png", exact: true })
    .click();
  await review(page);
  await expect(
    page.getByRole("img", { name: "任务副本预览", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "创建并开始任务", exact: true }),
  ).toBeDisabled();
  // Legacy document support remains available through the exact same component.
  await page.goto("/#add");
  const document = page
    .locator(".content-inner > div")
    .filter({
      has: page.getByRole("heading", {
        name: "为文档添加隐形水印",
        exact: true,
      }),
    })
    .locator(".file-upload");
  await document.locator('input[type="file"]').setInputFiles({
    name: "source.docx",
    mimeType:
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    buffer: Buffer.from("fixture"),
  });
  await expect(
    document.getByText("source.docx", { exact: true }),
  ).toBeVisible();
  await expect(document.locator(".file-thumbnail .anticon-file")).toBeVisible();
  await document
    .locator('input[type="file"]')
    .setInputFiles({ name: "draft.png", mimeType: "image/png", buffer: raw });
  const thumbnail = document.getByRole("img", {
    name: "所选文件: draft.png",
    exact: true,
  });
  await expect(thumbnail).toBeVisible();
  const draftURL = await thumbnail.getAttribute("src");
  await page.evaluate(() => (location.hash = "overview"));
  await expect(
    page.getByRole("heading", { name: "工作台", exact: true }),
  ).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(
        (url) =>
          fetch(url!).then(
            () => false,
            () => true,
          ),
        draftURL,
      ),
    )
    .toBe(true);
  await page.evaluate(() => (location.hash = "add"));
  await expect(document.getByText("draft.png", { exact: true })).toBeVisible();
  await expect(thumbnail).toBeVisible();
  await expect(thumbnail).not.toHaveAttribute("src", draftURL!);
});
