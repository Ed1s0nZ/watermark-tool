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
function entries(zip: Buffer) {
  const result: { name: string; data: Buffer }[] = [];
  for (let i = 0; i < zip.length - 46; i++) {
    if (zip.readUInt32LE(i) !== 0x02014b50) continue;
    const length = zip.readUInt16LE(i + 28),
      name = zip.subarray(i + 46, i + 46 + length).toString();
    const size = zip.readUInt32LE(i + 20),
      local = zip.readUInt32LE(i + 42),
      start =
        local +
        30 +
        zip.readUInt16LE(local + 26) +
        zip.readUInt16LE(local + 28);
    if (name.endsWith(".jpg") || name === "manifest.csv")
      result.push({ name, data: zip.subarray(start, start + size) });
  }
  return result;
}
test("two source sizes, three recipients: pinned logo/regions, six real JPEG copies and reusable policy", async ({
  page,
}, info) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page);
  const original = await readFile("../../internal/blind/testdata/clean.png"),
    logo = await logoBytes(page);
  const second = Buffer.from(
    await page.evaluate(async (raw) => {
      const source = await createImageBitmap(
        new Blob([new Uint8Array(raw)], { type: "image/png" }),
      );
      const c = document.createElement("canvas");
      c.width = 640;
      c.height = 384;
      c.getContext("2d")!.drawImage(source, 0, 0, 640, 384);
      source.close();
      const b = await new Promise<Blob>((r) =>
        c.toBlob((b) => r(b!), "image/png"),
      );
      return Array.from(new Uint8Array(await b.arrayBuffer()));
    }, Array.from(original)),
  );
  await page
    .getByRole("menuitem", { name: "多图分发任务", exact: true })
    .click();
  await page.evaluate(() =>
    localStorage.setItem(
      "watermark.policies.v1",
      JSON.stringify({
        version: 1,
        items: [
          {
            id: crypto.randomUUID(),
            name: "Legacy v1",
            mode: "dual",
            template: "{name}",
            style: {
              mode: "single",
              position: "bottomRight",
              size: 28,
              opacity: 0.35,
              color: "#365bdb",
              angle: -30,
            },
            quality: {},
          },
        ],
      }),
    ),
  );
  await page.reload();
  await choose(page, "选择策略", "Legacy v1");
  await page
    .getByRole("button", { name: "明确应用所选策略", exact: true })
    .click();

  await page.getByLabel("任务原图", { exact: true }).setInputFiles([
    { name: "one.png", mimeType: "image/png", buffer: original },
    { name: "two.png", mimeType: "image/png", buffer: second },
  ]);
  await page
    .getByLabel("任务接收人 CSV", { exact: true })
    .fill("recipient_id,name\n001,Alice\n002,张三\n003,Bob");
  await choose(page, "任务水印模式", "可见与隐形双水印");
  await choose(page, "可见层组合", "文字＋Logo");
  await choose(page, "Logo位置", "右上");
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: logo });
  await choose(page, "成品格式", "JPEG");
  await drawRegion(page);
  await choose(
    page,
    "避让原图",
    `2. two.png · ${createHash("sha256").update(second).digest("hex").slice(0, 8)}`,
  );
  await expect(
    page.getByRole("spinbutton", { name: "区域 1 x %", exact: true }),
  ).toHaveCount(0);
  await drawRegion(page, 0.05, 0.05, 0.2, 0.2);
  await page.getByLabel("策略名称", { exact: true }).fill("Brand JPEG");
  await page.getByRole("button", { name: "保存为新策略", exact: true }).click();
  const policy = await page.evaluate(() =>
    JSON.parse(localStorage.getItem("watermark.policies.v1")!).items.find(
      (p: { name: string }) => p.name === "Brand JPEG",
    ),
  );
  expect(policy.layout.layers).toBe("both");
  expect(policy.output).toEqual({ format: "jpeg", jpeg_quality: 90 });
  expect(JSON.stringify(policy)).not.toContain(
    createHash("sha256").update(logo).digest("hex"),
  );
  expect(policy).not.toHaveProperty("exclusions");
  expect(policy.layout.logo).not.toHaveProperty("sha256");
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "任务副本预览", exact: true }),
  ).toBeVisible();
  const made = page.waitForResponse(
    (r) => r.url().endsWith("/api/jobs") && r.request().method() === "POST",
  );
  await page
    .getByRole("button", { name: "创建并开始任务", exact: true })
    .click();
  const created = await (await made).json();
  expect(created.output).toEqual({ format: "jpeg", jpeg_quality: 90 });
  expect(created.artwork.logo.sha256).toBe(
    createHash("sha256").update(logo).digest("hex"),
  );
  expect(Object.keys(created.artwork.exclusions)).toHaveLength(2);
  await expect(
    page.getByRole("button", { name: "下载整套 ZIP", exact: true }),
  ).toBeVisible({ timeout: 30000 });
  const dl = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载整套 ZIP", exact: true }).click();
  const files = entries(await downloaded(await dl));
  expect(files.filter((f) => f.name.endsWith(".jpg"))).toHaveLength(6);
  const current = await (
    await page.request.get("/api/jobs/" + created.id)
  ).json();
  for (const f of files.filter((f) => f.name.endsWith(".jpg"))) {
    expect(f.data.subarray(0, 2)).toEqual(Buffer.from([255, 216]));
    const check = await page.request.post("/api/workbench/verify", {
      multipart: {
        file: { name: "actual.jpg", mimeType: "image/jpeg", buffer: f.data },
      },
    });
    expect(check.status()).toBe(200);
    const report = await check.json();
    expect(report.status).toBe("registered");
    expect(report.record.job_id).toBe(created.id);
    const copy = current.copies.find(
      (c: { id: string }) => c.id === report.result.watermark,
    );
    expect(copy.quality.output_sha256).toBe(
      createHash("sha256").update(f.data).digest("hex"),
    );
    expect(copy.quality.format).toBe("jpeg");
    expect(copy.quality.final.status).toBe("finite");
  }
  await page.getByRole("button", { name: "载入任务参数", exact: true }).click();
  expect(
    await page
      .getByLabel("Logo文件", { exact: true })
      .evaluate((e: HTMLInputElement) => e.files?.length),
  ).toBe(0);
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({
      name: "wrong-logo.png",
      mimeType: "image/png",
      buffer: original,
    });
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByText("恢复任务需要相同Logo，请核对摘要并重新选择。", {
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByLabel("Logo文件", { exact: true })
    .setInputFiles({ name: "logo.png", mimeType: "image/png", buffer: logo });
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "任务副本预览", exact: true }),
  ).toBeVisible();

  await expect(
    page.getByText(new RegExp("恢复Logo摘要: " + created.artwork.logo.sha256)),
  ).toBeVisible();
  await page.screenshot({
    path: info.outputPath("jpeg-job-snapshot.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.reload();
  await expect(
    page.getByRole("button", { name: "下载整套 ZIP", exact: true }),
  ).toBeVisible();
  await choose(page, "选择策略", "Brand JPEG");
  await page
    .getByRole("button", { name: "明确应用所选策略", exact: true })
    .click();
  await expect(page.getByLabel("Logo文件", { exact: true })).toBeVisible();
  expect(
    await page
      .getByLabel("Logo文件", { exact: true })
      .evaluate((e: HTMLInputElement) => e.files?.length),
  ).toBe(0);
  expect(errors).toEqual([]);
});
