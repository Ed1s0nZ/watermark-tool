import { configure, review } from "./editor-helpers";
import { navigate } from "./navigation";
import { test, expect, Download } from "@playwright/test";
import { readFile } from "node:fs/promises";
async function bytes(d: Download) {
  const chunks: Buffer[] = [];
  for await (const c of (await d.createReadStream())!)
    chunks.push(Buffer.from(c));
  return Buffer.concat(chunks);
}
test("policies, streamed mixed tracing, cancellation and quality gating", async ({
  page,
}, info) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page
    .getByLabel("密码", { exact: true })
    .fill("local-test-password-2026");
  await page.getByRole("button", { name: "登录工作台", exact: true }).click();
  await navigate(page, "多图分发任务");
  await configure(page);
  await page.getByLabel("策略名称", { exact: true }).fill("TeamPreset");
  await page.getByRole("button", { name: "保存为新策略", exact: true }).click();
  const stored = await page.evaluate(() =>
    JSON.parse(localStorage.getItem("watermark.policies.v1")!),
  );
  expect(stored.items).toHaveLength(1);
  expect(Object.keys(stored.items[0]).sort()).toEqual([
    "id",
    "layout",
    "mode",
    "name",
    "output",
    "quality",
    "style",
    "template",
  ]);
  await page.getByLabel("策略名称", { exact: true }).fill("RenamedPreset");
  await page
    .getByRole("button", { name: "重命名所选策略", exact: true })
    .click();
  await page
    .getByRole("switch", { name: "启用最终成品最低PSNR", exact: true })
    .click();
  await page
    .getByRole("spinbutton", { name: "最低PSNR dB", exact: true })
    .fill("36");
  await page.getByRole("button", { name: "更新所选策略", exact: true }).click();
  await page.reload();
  await configure(page);
  await page.getByRole("combobox", { name: "选择策略", exact: true }).click();
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("RenamedPreset", { exact: true })
    .click();
  await page
    .getByRole("button", { name: "明确应用所选策略", exact: true })
    .click();
  await expect(
    page.getByRole("spinbutton", { name: "最低PSNR dB", exact: true }),
  ).toHaveValue(/^36(?:\.0)?$/);
  await page.getByRole("button", { name: "删除所选策略", exact: true }).click();
  expect(
    await page.evaluate(
      () =>
        JSON.parse(localStorage.getItem("watermark.policies.v1")!).items.length,
    ),
  ).toBe(0);
  await page
    .getByRole("button", { name: "恢复内置示例选择", exact: true })
    .click();
  await page
    .getByRole("button", { name: "明确应用所选策略", exact: true })
    .click();
  await expect(
    page.getByRole("textbox", { name: "接收人文字模板", exact: true }),
  ).toHaveValue("{name} · {recipient_id}");
  // Storage denial must never claim success, and old style-template data remains untouched.
  await page.evaluate(() => {
    const w = window as typeof window & {
      originalSet?: typeof Storage.prototype.setItem;
    };
    w.originalSet = Storage.prototype.setItem;
    Storage.prototype.setItem = function (k, v) {
      if (k === "watermark.policies.v1")
        throw new DOMException("full", "QuotaExceededError");
      return w.originalSet!.call(this, k, v);
    };
  });
  await page.getByLabel("策略名称", { exact: true }).fill("Rejected");
  await page.getByRole("button", { name: "保存为新策略", exact: true }).click();
  await expect(page.getByText(/策略存储不可用/)).toBeVisible();
  expect(
    await page.evaluate(
      () =>
        JSON.parse(localStorage.getItem("watermark.policies.v1")!).items.length,
    ),
  ).toBe(0);
  await page.evaluate(() => {
    const w = window as typeof window & {
      originalSet?: typeof Storage.prototype.setItem;
    };
    Storage.prototype.setItem = w.originalSet!;
    localStorage.setItem(
      "watermark.policies.v1",
      '{"version":1,"items":[{"unknown":true}]}',
    );
  });
  await page.reload();
  await configure(page);
  await expect(page.getByText(/策略存储不可用/)).toBeVisible();
  await page.getByLabel("策略名称", { exact: true }).fill("Recovered");
  await page.getByRole("button", { name: "保存为新策略", exact: true }).click();
  await expect(page.getByText(/策略存储不可用/)).toHaveCount(0);
  // A full policy list refuses an additional save without discarding prior entries.
  await page.evaluate(() => {
    const key = "watermark.policies.v1";
    const item = JSON.parse(localStorage.getItem(key)!).items[0];
    localStorage.setItem(
      key,
      JSON.stringify({
        version: 1,
        items: Array.from({ length: 20 }, (_, i) => ({
          ...item,
          id: crypto.randomUUID(),
          name: `Policy-${i}`,
        })),
      }),
    );
  });
  await page.reload();
  await configure(page);
  await page.getByLabel("策略名称", { exact: true }).fill("TwentyFirst");
  await page.getByRole("button", { name: "保存为新策略", exact: true }).click();
  await expect(page.getByText(/策略存储不可用/)).toBeVisible();
  expect(
    await page.evaluate(
      () =>
        JSON.parse(localStorage.getItem("watermark.policies.v1")!).items.length,
    ),
  ).toBe(20);
  // A real server-produced image is a valid but unregistered watermark.
  const original = await readFile("../../internal/blind/testdata/clean.png");
  const generated = await page.request.post("/api/workbench/dual", {
    multipart: {
      file: { name: "raw.png", mimeType: "image/png", buffer: original },
      watermark: "private-test",
    },
  });
  expect(generated.status()).toBe(200);
  const marked = await generated.body();
  await navigate(page, "批量溯源");
  await page.getByLabel("待溯源图片", { exact: true }).setInputFiles([
    { name: "same.png", mimeType: "image/png", buffer: original },
    { name: "same.png", mimeType: "image/png", buffer: marked },
    {
      name: "=1+1<script>.png",
      mimeType: "image/png",
      buffer: Buffer.from("broken"),
    },
    { name: "duplicate.png", mimeType: "image/png", buffer: original },
  ]);
  await page.getByRole("button", { name: "开始批量溯源", exact: true }).click();
  await expect(page.getByText("部分项目出错", { exact: true })).toBeVisible();
  let dl = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "导出汇总 JSON", exact: true })
    .click();
  const result = JSON.parse((await bytes(await dl)).toString());
  expect(result.done).toBe(4);
  expect(result.items.map((i: { status: string }) => i.status)).toEqual([
    "not_found",
    "unregistered",
    "error",
    "not_found",
  ]);
  expect(result.items[3].duplicate_of).toBe(1);
  expect(result.items[1].report.result.watermark).toBe("private-test");
  dl = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出汇总 CSV", exact: true }).click();
  const csv = (await bytes(await dl)).toString();
  expect(csv).toContain('"\'=1+1<script>.png"');
  dl = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "导出汇总 HTML", exact: true })
    .click();
  const html = (await bytes(await dl)).toString();
  expect(html).not.toContain("<script>");
  expect(html).toContain("&lt;script&gt;");
  expect(html).toContain("Content-Security-Policy");
  await page.setViewportSize({ width: 1280, height: 1000 });
  await page.getByRole("table").scrollIntoViewIfNeeded();
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("trace-batch.png"),
    fullPage: true,
  });
  // Cancel before submission reaches server: preserve truthful unexecuted rows.
  let blocked = false;
  let abort: () => void = () => {};
  await page.route("**/api/workbench/trace-batch", async (route) => {
    blocked = true;
    await new Promise<void>((resolve) => {
      abort = resolve;
    });
    await route.abort().catch(() => {});
  });
  await page.getByRole("button", { name: "开始批量溯源", exact: true }).click();
  await expect.poll(() => blocked).toBe(true);
  await page.getByRole("button", { name: "取消本批", exact: true }).click();
  abort();
  await page.unroute("**/api/workbench/trace-batch");
  await expect(page.getByText("已取消", { exact: true })).toBeVisible();
  dl = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "导出汇总 JSON", exact: true })
    .click();
  const cancelled = JSON.parse((await bytes(await dl)).toString());
  expect(cancelled.done).toBe(0);
  expect(
    cancelled.items.every(
      (i: { status: string }) => i.status === "not_confirmed",
    ),
  ).toBe(true);
  await navigate(page, "双水印模板");
  await configure(page);
  await page.getByLabel("双水印原图", { exact: true }).setInputFiles({
    name: "clean.png",
    mimeType: "image/png",
    buffer: original,
  });
  await page.getByLabel("可见文字", { exact: true }).fill("测试 Alice");
  await page.getByLabel("隐形载荷", { exact: true }).fill("quality-text");
  await page
    .getByRole("switch", { name: "启用最终成品最低PSNR", exact: true })
    .click();
  await page
    .getByRole("spinbutton", { name: "最低PSNR dB", exact: true })
    .fill("60");
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await page.getByRole("button", { name: "生成双水印", exact: true }).click();
  await expect(page.getByText("未过质量门槛", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "下载图片", exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("switch", { name: "启用最终成品最低PSNR", exact: true })
    .click();
  await review(page);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await page.getByRole("button", { name: "生成双水印", exact: true }).click();
  await expect(page.getByText("通过", { exact: true })).toBeVisible();
  await expect(page.getByText(/可见层变化（参考原图）/)).toBeVisible();
  await page
    .locator(".ant-card")
    .filter({ has: page.getByText("质量摘要", { exact: true }) })
    .last()
    .scrollIntoViewIfNeeded();
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("quality-dual.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await review(page);
  await page
    .getByRole("combobox", { name: "语言", exact: true })
    .press("Enter");
  await page.getByTitle("English", { exact: true }).click();
  await page
    .getByRole("combobox", { name: "Appearance", exact: true })
    .press("Enter");
  await page.getByTitle("Dark", { exact: true }).click();
  await expect(
    page.getByText("Quality summary", { exact: true }),
  ).toBeVisible();
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  await page
    .locator(".ant-card")
    .filter({ has: page.getByText("Quality summary", { exact: true }) })
    .last()
    .scrollIntoViewIfNeeded();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    animations: "disabled",
    path: info.outputPath("quality-mobile-dark.png"),
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  await navigate(page, "Batch tracing");
  await page.getByLabel("Images to trace", { exact: true }).setInputFiles([
    { name: "raw.png", mimeType: "image/png", buffer: original },
    { name: "marked.png", mimeType: "image/png", buffer: marked },
  ]);
  await page
    .getByRole("button", { name: "Start batch tracing", exact: true })
    .click();
  await expect(
    page.getByText("All items executed", { exact: true }),
  ).toBeVisible();
  await page.getByRole("table").scrollIntoViewIfNeeded();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("trace-mobile-dark.png"),
    fullPage: true,
    animations: "disabled",
  });
  expect(errors).toEqual([]);
});
