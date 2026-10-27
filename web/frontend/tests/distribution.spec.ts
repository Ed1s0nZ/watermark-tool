import { navigate } from "./navigation";
import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
function pngFromZIP(zip: Buffer): Buffer {
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
    return zip.subarray(start, start + size);
  }
  throw new Error("PNG not found in ZIP");
}
test("distribution lifecycle with registration, rotation and regenerated ZIP", async ({
  page,
}, testInfo) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page
    .getByLabel("密码", { exact: true })
    .fill("local-test-password-2026");
  await page.getByRole("button", { name: "登录工作台", exact: true }).click();
  await navigate(page, "批量副本");
  await page
    .getByLabel("批次原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page
    .getByLabel("接收人名单 CSV", { exact: true })
    .fill("recipient_id,name\n1,Alice\n1,Bob");
  await page.getByRole("button", { name: "生成批量副本", exact: true }).click();
  await expect(page.getByText(/名单或分页参数不正确/)).toBeVisible();
  await page
    .getByLabel("接收人名单 CSV", { exact: true })
    .fill("recipient_id,name\n1,Alice\n2,名字\n3,Bob");
  const responsePromise = page.waitForResponse((r) =>
    r.url().endsWith("/api/platform/batches"),
  );
  await page.getByRole("button", { name: "生成批量副本", exact: true }).click();
  const response = await responsePromise;
  expect(response.status()).toBe(200);
  const batch = await response.headerValue("x-batch-id");
  expect(batch).toBeTruthy();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("link", { name: "下载 ZIP", exact: true }).click();
  const download = await downloadPromise;
  const bytes = await readFile((await download.path())!);
  const png = pngFromZIP(bytes);
  await page.setViewportSize({ width: 1440, height: 1100 });
  await page.locator(".content").evaluate((el) => el.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath("batch-copies.png"),
    fullPage: true,
    animations: "disabled",
  });

  await navigate(page, "分发记录");
  await expect(
    page
      .locator(".content-inner > div")
      .filter({
        has: page.getByRole("heading", { name: "分发记录", exact: true }),
      })
      .getByText(batch!, { exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "副本", exact: true }).click();
  await expect(
    page.getByRole("cell", { name: "Alice", exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "图片溯源", exact: true }).click();
  await page
    .getByLabel("待溯源图片", { exact: true })
    .setInputFiles({ name: "copy.png", mimeType: "image/png", buffer: png });
  await page.getByRole("button", { name: "验证并查询", exact: true }).click();
  await expect(
    page.getByText("水印已验证，找到对应登记记录", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Alice", { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.locator(".content").evaluate((el) => el.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath("registered-trace.png"),
    fullPage: true,
    animations: "disabled",
  });
  await navigate(page, "密钥版本");
  await page.getByRole("button", { name: "创建密钥版本", exact: true }).click();
  const candidate = page.getByRole("row").filter({
    has: page.getByRole("button", { name: "设为当前", exact: true }),
  });
  await expect(candidate).toHaveCount(1);
  const newID = await candidate.locator("code").innerText();
  const newRow = page.getByRole("row").filter({ hasText: newID });
  await newRow.getByRole("button", { name: "设为当前", exact: true }).click();
  await page.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(newRow).toContainText("当前嵌入");
  const legacy = page.getByRole("row").filter({ hasText: "legacy-env" });
  await expect(legacy).toContainText("仅提取");
  await navigate(page, "批量副本");
  await page
    .getByLabel("接收人名单 CSV", { exact: true })
    .fill("recipient_id,name\n4,新版本接收人");
  const nextResponsePromise = page.waitForResponse((r) =>
    r.url().endsWith("/api/platform/batches"),
  );
  await page.getByRole("button", { name: "生成批量副本", exact: true }).click();
  const nextResponse = await nextResponsePromise;
  expect(nextResponse.status()).toBe(200);
  const nextBatchID = await nextResponse.headerValue("x-batch-id");
  const query = await page.request.post("/api/platform/batches/search", {
    data: { q: nextBatchID, page: 1, page_size: 20 },
  });
  const nextBatch = (await query.json()).items[0];
  expect(nextBatch.key_id).toBe(newID);
  await navigate(page, "分发记录");
  await page.getByRole("tab", { name: "批次", exact: true }).click();
  const row = page.getByRole("row").filter({ hasText: batch! });
  await row.getByRole("button", { name: "重新生成 ZIP", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByLabel("原始图片", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await dialog
    .getByRole("button", { name: "重新生成 ZIP", exact: true })
    .click();
  const rebuiltPromise = page.waitForEvent("download");
  await dialog.getByRole("link", { name: "下载 ZIP", exact: true }).click();
  const rebuilt = await readFile((await (await rebuiltPromise).path())!);
  expect(createHash("sha256").update(rebuilt).digest("hex")).toBe(
    createHash("sha256").update(bytes).digest("hex"),
  );
  await dialog.getByRole("button", { name: "Close", exact: true }).click();
  await navigate(page, "密钥版本");
  await page
    .getByRole("row")
    .filter({ hasText: "legacy-env" })
    .getByRole("button", { name: "停用", exact: true })
    .click();
  await page.getByRole("button", { name: "确认变更", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "legacy-env" }),
  ).toContainText("已停用");
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.getByRole("combobox", { name: "语言" }).first().press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("English", { exact: true })
    .click();
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: "Key versions", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("keys-mobile.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.getByRole("button", { name: "Open navigation" }).click();
  await navigate(page, "Batch copies");
  await expect(
    page.getByRole("heading", { name: "Batch copies", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  expect(errors).toEqual([]);
});
