import { test, expect, Download } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
async function bytes(download: Download) {
  const stream = await download.createReadStream();
  const chunks: Buffer[] = [];
  for await (const chunk of stream!) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks);
}
test("experiment, bilingual dual watermark, templates and truthful reports", async ({
  page,
}, info) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page
    .getByLabel("密码", { exact: true })
    .fill("local-test-password-2026");
  await page.getByRole("button", { name: "登录工作台", exact: true }).click();
  await page.getByRole("menuitem", { name: "效果实验室", exact: true }).click();
  await page
    .getByLabel("实验原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page.getByLabel("隐形载荷", { exact: true }).fill("项目-测试");
  await page.getByRole("button", { name: "运行实验", exact: true }).click();
  await expect(
    page.getByText("基线验证通过，原图保持不变。", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("row")).toHaveCount(11);
  const dl = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载 JSON", exact: true }).click();
  const report = JSON.parse((await bytes(await dl)).toString());
  expect(report.cases).toHaveLength(10);
  expect(report.baseline_ok).toBe(true);
  expect(report.original_intact).toBe(true);
  expect(report.cases[0].recovered).toBe(true);
  expect(
    report.cases.slice(-2).every((r: { recovered: boolean }) => !r.recovered),
  ).toBe(true);
  const original = await readFile("../../internal/blind/testdata/clean.png");
  expect(report.source_sha256).toBe(
    createHash("sha256").update(original).digest("hex"),
  );
  await page.locator(".content").evaluate((el) => (el.scrollTop = 0));
  await page.screenshot({
    path: info.outputPath("laboratory.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.getByLabel("隐形载荷", { exact: true }).fill("重试");
  await expect(page.getByRole("table")).toHaveCount(0);
  await page.route(
    "**/api/workbench/lab",
    (route) =>
      new Promise<void>((resolve) => {
        setTimeout(() => {
          void route
            .abort()
            .catch(() => {})
            .finally(resolve);
        }, 1200);
      }),
  );
  await page.getByRole("button", { name: "运行实验", exact: true }).click();
  await page.getByRole("button", { name: "取消任务", exact: true }).click();
  await expect(
    page.getByText("任务已取消，可以重试。", { exact: true }),
  ).toBeVisible();
  await page.unroute("**/api/workbench/lab");
  await page.getByRole("menuitem", { name: "双水印模板", exact: true }).click();
  await page
    .getByLabel("双水印原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page.getByLabel("可见文字", { exact: true }).fill("内部资料 Team");
  await page
    .getByLabel("隐形载荷", { exact: true })
    .fill("<script>测试</script>");
  await page.getByLabel("模板名称", { exact: true }).fill("内部模板");
  await page.getByRole("button", { name: "保存样式", exact: true }).click();
  const stored = await page.evaluate(() =>
    localStorage.getItem("watermark.visible.templates.v1"),
  );
  expect(stored).toContain("内部模板");
  expect(stored).not.toContain("内部资料");
  expect(stored).not.toContain("<script>");
  await page.reload();
  await page.getByRole("combobox", { name: "模板", exact: true }).click();
  await page.getByTitle("内部模板", { exact: true }).click();
  await page
    .getByLabel("双水印原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page.getByLabel("可见文字", { exact: true }).fill("内部资料 Team");
  await page
    .getByLabel("隐形载荷", { exact: true })
    .fill("<script>测试</script>");
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByText("预览完成，请核对文字后生成。", { exact: true }),
  ).toBeVisible();
  const preview = await page
    .getByRole("img", { name: "生成预览", exact: true })
    .evaluate(async (img: HTMLImageElement) =>
      Array.from(new Uint8Array(await (await fetch(img.src)).arrayBuffer())),
    );
  expect(Buffer.from(preview)).not.toEqual(original);
  await page
    .getByRole("combobox", { name: "样式", exact: true })
    .press("Enter");
  await page.getByTitle("平铺", { exact: true }).click();
  await page
    .getByRole("spinbutton", { name: "平铺角度", exact: true })
    .fill("20");
  await page.getByRole("spinbutton", { name: "字号", exact: true }).fill("20");
  await page
    .getByRole("spinbutton", { name: "不透明度", exact: true })
    .fill("0.5");
  await page.getByLabel("颜色", { exact: true }).fill("#aa2222");
  await expect(
    page.getByRole("img", { name: "生成预览", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "生成预览", exact: true }).click();
  await expect(
    page.getByText("预览完成，请核对文字后生成。", { exact: true }),
  ).toBeVisible();
  const tiled = await page
    .getByRole("img", { name: "生成预览", exact: true })
    .evaluate(async (img: HTMLImageElement) =>
      Array.from(new Uint8Array(await (await fetch(img.src)).arrayBuffer())),
    );
  expect(tiled).not.toEqual(preview);
  await page.getByRole("button", { name: "生成双水印", exact: true }).click();
  await expect(
    page.getByText("已生成并验证隐形载荷。", { exact: true }),
  ).toBeVisible();
  const imageDL = page.waitForEvent("download");
  await page.getByRole("link", { name: "下载图片", exact: true }).click();
  const marked = await bytes(await imageDL);
  expect(marked).not.toEqual(Buffer.from(preview));
  await page.screenshot({
    path: info.outputPath("dual-watermark.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.getByLabel("可见文字", { exact: true }).fill("Changed");
  await expect(
    page.getByRole("link", { name: "下载图片", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "删除模板", exact: true }).click();
  expect(
    await page.evaluate(() =>
      localStorage.getItem("watermark.visible.templates.v1"),
    ),
  ).not.toContain("内部模板");
  await page.getByRole("menuitem", { name: "分发记录", exact: true }).click();
  await page.getByRole("tab", { name: "图片溯源", exact: true }).click();
  await page
    .getByLabel("待溯源图片", { exact: true })
    .setInputFiles({ name: "dual.png", mimeType: "image/png", buffer: marked });
  await page
    .getByRole("button", { name: "验证并生成报告", exact: true })
    .click();
  await expect(page.getByText("已验证，未登记", { exact: true })).toBeVisible();
  const jsonDL = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载 JSON", exact: true }).click();
  const verification = JSON.parse((await bytes(await jsonDL)).toString());
  expect(verification.result.watermark).toBe("<script>测试</script>");
  expect(verification.source_sha256).toBe(
    createHash("sha256").update(marked).digest("hex"),
  );
  expect(verification.status).toBe("unregistered");
  const htmlDL = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载 HTML", exact: true }).click();
  const html = (await bytes(await htmlDL)).toString();
  expect(html).toContain("&lt;script&gt;测试&lt;/script&gt;");
  expect(html).not.toContain("<script>");
  const offline = await page.context().newPage();
  await offline.setContent(html);
  await expect(offline.locator("script")).toHaveCount(0);
  await expect(offline.locator("pre")).toContainText("<script>测试</script>");
  await offline.emulateMedia({ media: "print" });
  await offline.screenshot({
    path: info.outputPath("verification-report.png"),
    fullPage: true,
  });
  await offline.close();
  await page
    .getByLabel("待溯源图片", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await expect(
    page.getByRole("button", { name: "下载 JSON", exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("button", { name: "验证并生成报告", exact: true })
    .click();
  await expect(page.getByText("未检出有效水印", { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await page
    .getByRole("combobox", { name: "语言", exact: true })
    .first()
    .press("Enter");
  await page.getByTitle("English", { exact: true }).click();
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Dual watermark templates", exact: true })
    .last()
    .click();
  await expect(
    page.getByRole("heading", {
      name: "Dual watermark templates",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Open navigation", exact: true }),
  ).toBeFocused();
  await page
    .getByRole("combobox", { name: "Appearance", exact: true })
    .first()
    .press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("Dark", { exact: true })
    .click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  await expect(
    page.getByRole("textbox", { name: "Visible text", exact: true }),
  ).toHaveCSS("background-color", "rgb(23, 29, 41)");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("workbench-mobile.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Effect laboratory", exact: true })
    .last()
    .click();
  await expect(
    page.getByRole("heading", { name: "Effect laboratory", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page
    .getByLabel("Experiment source", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page
    .getByRole("textbox", { name: "Invisible payload", exact: true })
    .fill("Mobile-A");
  await page
    .getByRole("button", { name: "Run experiment", exact: true })
    .click();
  await expect(
    page.getByText("Baseline verified; original preserved.", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("laboratory-mobile.png"),
    animations: "disabled",
  });
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Dual watermark templates", exact: true })
    .last()
    .click();
  await page
    .getByLabel("Dual watermark source", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page
    .getByRole("textbox", { name: "Visible text", exact: true })
    .fill("Mobile Team");
  await page
    .getByRole("textbox", { name: "Invisible payload", exact: true })
    .fill("Mobile-A");
  await page
    .getByRole("button", { name: "Create preview", exact: true })
    .click();
  await expect(
    page.getByText("Preview ready. Check the text before generating.", {
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Generate dual watermark", exact: true })
    .click();
  await expect(
    page.getByText("Generated and invisible payload verified.", {
      exact: true,
    }),
  ).toBeVisible();
  const mobileDL = page.waitForEvent("download");
  await page.getByRole("link", { name: "Download image", exact: true }).click();
  const mobileImage = await bytes(await mobileDL);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  await page
    .getByRole("menuitem", { name: "Distribution records", exact: true })
    .last()
    .click();
  await page.getByRole("tab", { name: "Image trace", exact: true }).click();
  await page
    .getByLabel("Image to trace", { exact: true })
    .setInputFiles({
      name: "mobile.png",
      mimeType: "image/png",
      buffer: mobileImage,
    });
  await page
    .getByRole("button", { name: "Verify and create report", exact: true })
    .click();
  await expect(
    page.getByText("Verified, unregistered", { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("report-mobile.png"),
    animations: "disabled",
  });
  expect(errors).toEqual([]);
});
