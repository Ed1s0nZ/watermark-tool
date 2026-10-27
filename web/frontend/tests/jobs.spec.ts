import { test, expect, Download } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
async function bytes(download: Download) {
  const stream = await download.createReadStream();
  const chunks: Buffer[] = [];
  for await (const c of stream!) chunks.push(Buffer.from(c));
  return Buffer.concat(chunks);
}
function copies(zip: Buffer) {
  const result: { name: string; data: Buffer }[] = [];
  for (let i = 0; i < zip.length - 46; i++) {
    if (zip.readUInt32LE(i) !== 0x02014b50) continue;
    const length = zip.readUInt16LE(i + 28);
    const name = zip.subarray(i + 46, i + 46 + length).toString();
    if (!name.endsWith(".png")) continue;
    expect(zip.readUInt16LE(i + 10)).toBe(0);
    const size = zip.readUInt32LE(i + 20);
    const local = zip.readUInt32LE(i + 42);
    const start =
      local + 30 + zip.readUInt16LE(local + 26) + zip.readUInt16LE(local + 28);
    result.push({ name, data: zip.subarray(start, start + size) });
  }
  return result;
}
test("two images × three recipients: dual-watermark delivery, trace and audit", async ({
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
  const initialCount = (
    await (
      await page.request.post("/api/jobs/search", {
        data: { page: 1, page_size: 10 },
      })
    ).json()
  ).total;
  const original = await readFile("../../internal/blind/testdata/clean.png");
  const second = Buffer.from(
    await page.evaluate(async (data) => {
      const image = await createImageBitmap(
        new Blob([new Uint8Array(data)], { type: "image/png" }),
      );
      const canvas = document.createElement("canvas");
      canvas.width = image.width;
      canvas.height = image.height;
      const ctx = canvas.getContext("2d")!;
      ctx.drawImage(image, 0, 0);
      ctx.fillStyle = "#e33a77";
      ctx.fillRect(0, 0, 30, 30);
      image.close();
      const blob = await new Promise<Blob>((resolve) =>
        canvas.toBlob((b) => resolve(b!), "image/png"),
      );
      return Array.from(new Uint8Array(await blob.arrayBuffer()));
    }, Array.from(original)),
  );
  await page
    .getByRole("menuitem", { name: "多图分发任务", exact: true })
    .click();
  await page.getByLabel("任务原图", { exact: true }).setInputFiles([
    { name: "one.png", mimeType: "image/png", buffer: original },
    { name: "two.png", mimeType: "image/png", buffer: second },
  ]);
  await page.getByLabel("导入任务名单", { exact: true }).setInputFiles({
    name: "recipients.csv",
    mimeType: "text/csv",
    buffer: Buffer.from("recipient_id,name\n001,Alice\n002,张三\n003,Bob"),
  });
  await expect(
    page.getByRole("textbox", { name: "任务接收人 CSV", exact: true }),
  ).toHaveValue("recipient_id,name\n001,Alice\n002,张三\n003,Bob");
  await page
    .getByRole("combobox", { name: "任务水印模式", exact: true })
    .press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("可见与隐形双水印", { exact: true })
    .click();
  await page
    .getByRole("textbox", { name: "接收人文字模板", exact: true })
    .fill("A".repeat(60));
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByText(
      "可见文字过宽或存在缺字，请缩小字号、更换文字或检查设备字体后重新预览。",
      { exact: true },
    ),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "接收人文字模板", exact: true })
    .fill("内部资料 {name}/{recipient_id}");
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "任务副本预览", exact: true }),
  ).toBeVisible();
  const before = await page.request.post("/api/jobs/search", {
    data: { page: 1, page_size: 10 },
  });
  expect((await before.json()).total).toBe(initialCount);
  await page
    .getByRole("combobox", { name: "原图", exact: true })
    .press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("two.png", { exact: true })
    .click();
  await page
    .getByRole("combobox", { name: "接收人", exact: true })
    .press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("002 · 张三", { exact: true })
    .click();
  await expect(
    page.getByText("内部资料 张三/002", { exact: true }),
  ).toBeVisible();
  const made = page.waitForResponse(
    (r) => r.url().endsWith("/api/jobs") && r.request().method() === "POST",
  );
  await page
    .getByRole("button", { name: "创建并开始任务", exact: true })
    .click();
  const created = await (await made).json();
  expect(created.copies).toHaveLength(6);
  await expect(
    page.getByRole("button", { name: "下载整套 ZIP", exact: true }),
  ).toBeVisible({ timeout: 30000 });
  const dl = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载整套 ZIP", exact: true }).click();
  const archive = await bytes(await dl);
  const files = copies(archive);
  expect(files).toHaveLength(6);
  expect(
    files.every((f) =>
      /^recipient-00[123]\/[a-f0-9]{32}-[a-f0-9]{32}\.png$/.test(f.name),
    ),
  ).toBe(true);
  const seen = new Set<string>();
  for (const copy of files) {
    const verification = await page.request.post("/api/workbench/verify", {
      multipart: {
        file: { name: "copy.png", mimeType: "image/png", buffer: copy.data },
      },
    });
    expect(verification.status()).toBe(200);
    const report = await verification.json();
    expect(report.status).toBe("registered");
    expect(report.record.job_id).toBe(created.id);
    expect(
      created.copies.some(
        (c: { id: string; source_id: string; recipient_id: string }) =>
          c.id === report.result.watermark &&
          c.source_id === report.record.source_id &&
          c.recipient_id === report.record.recipient_id,
      ),
    ).toBe(true);
    seen.add(report.result.watermark);
  }
  expect(seen.size).toBe(6);
  const current = await (
    await page.request.get("/api/jobs/" + created.id)
  ).json();
  expect(current.generated).toBe(true);
  expect(current.quality_policy).toEqual({});
  for (const copy of current.copies) {
    expect(copy.quality.verified).toBe(true);
    expect(copy.quality.accepted).toBe(true);
    expect(copy.quality.embedding.status).toBe("finite");
    expect(copy.quality.visible.status).toBe("finite");
    expect(copy.quality.width).toBe(512);
  }
  expect(
    current.sources.map((s: { source_sha256: string }) => s.source_sha256),
  ).toEqual([
    createHash("sha256").update(original).digest("hex"),
    createHash("sha256").update(second).digest("hex"),
  ]);
  expect(
    current.copies.every((c: { rendered_sha256: string }) =>
      /^[a-f0-9]{64}$/.test(c.rendered_sha256),
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("delivery-complete.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.reload();
  await expect(
    page.getByRole("button", { name: "下载整套 ZIP", exact: true }),
  ).toBeVisible();
  await page.getByRole("menuitem", { name: "操作审计", exact: true }).click();
  await page
    .getByRole("textbox", { name: "动作", exact: true })
    .fill("task.create");
  await page.getByRole("button", { name: "查询审计", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: created.id }),
  ).toHaveCount(1);
  const auditDL = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "导出审计 JSON", exact: true })
    .click();
  const audit = JSON.parse((await bytes(await auditDL)).toString());
  expect(
    audit.items.filter((e: { target: string }) => e.target === created.id),
  ).toHaveLength(1);
  expect(
    audit.items.every((e: { action: string }) => e.action === "task.create"),
  ).toBe(true);
  expect(JSON.stringify(audit)).not.toContain("张三");
  expect(JSON.stringify(audit)).not.toContain("local-test-password-2026");
  await page.screenshot({
    path: info.outputPath("audit-filter.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page
    .getByRole("combobox", { name: "语言", exact: true })
    .first()
    .press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("English", { exact: true })
    .click();
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("audit-mobile.png"),
    animations: "disabled",
  });
  expect(errors).toEqual([]);
});
