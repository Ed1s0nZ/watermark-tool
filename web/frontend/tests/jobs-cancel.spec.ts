import { configure, review } from "./editor-helpers";
import { navigate } from "./navigation";
import { test, expect } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
test("cancel preparing upload, preserve identity on retry and recover on mobile refresh", async ({
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
  await navigate(page, "多图分发任务");
  await configure(page);
  await page
    .getByLabel("任务原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page
    .getByRole("textbox", { name: "任务接收人 CSV", exact: true })
    .fill("recipient_id,name\n001,Alice\n002,Bob");
  await review(page);
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "任务副本预览", exact: true }),
  ).toBeVisible();
  let release!: () => void;
  const waiting = new Promise<void>((resolve) => (release = resolve));
  await page.route("**/api/jobs/*/inputs", async (route) => {
    await waiting;
    await route.abort().catch(() => {});
  });
  const made = page.waitForResponse(
    (r) => r.url().endsWith("/api/jobs") && r.request().method() === "POST",
  );
  await page
    .getByRole("button", { name: "创建并开始任务", exact: true })
    .click();
  const created = await (await made).json();
  await expect(
    page.getByRole("button", { name: "取消后台任务", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "取消后台任务", exact: true }).click();
  await expect(page.getByText("已取消", { exact: true }).first()).toBeVisible();
  release();
  await page.unroute("**/api/jobs/*/inputs");
  const cancelled = await (
    await page.request.get("/api/jobs/" + created.id)
  ).json();
  expect(cancelled.status).toBe("cancelled");
  expect(cancelled.upload_token).not.toBe(created.upload_token);
  expect(cancelled.generated).toBe(false);
  await page.reload();
  await configure(page);
  await expect(page.getByText("已取消", { exact: true }).first()).toBeVisible();
  await expect(
    page.getByRole("button", { name: "按当前预览继续上传并重试", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "载入任务参数", exact: true }).click();
  await page
    .getByLabel("任务原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await review(page);
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByRole("img", { name: "任务副本预览", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "按当前预览继续上传并重试", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "下载整套 ZIP", exact: true }),
  ).toBeVisible({ timeout: 30000 });
  const completed = await (
    await page.request.get("/api/jobs/" + created.id)
  ).json();
  expect(completed.copies.map((c: { id: string }) => c.id)).toEqual(
    created.copies.map((c: { id: string }) => c.id),
  );
  expect(completed.created_at).toBe(created.created_at);
  expect(completed.generated).toBe(true);
  const archive = await page.request.get(
    "/api/jobs/" + created.id + "/archive",
  );
  expect(archive.status()).toBe(200);
  expect(
    createHash("sha256")
      .update(await archive.body())
      .digest("hex"),
  ).toBe(completed.archive_sha256);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await configure(page);
  await expect(
    page.getByRole("button", { name: "下载整套 ZIP", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page
    .getByRole("combobox", { name: "语言", exact: true })
    .first()
    .press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("English", { exact: true })
    .click();
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  await page
    .getByRole("combobox", { name: "Appearance", exact: true })
    .first()
    .press("ArrowDown");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle("Dark", { exact: true })
    .click();
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect(
    page.getByRole("button", { name: "Download complete ZIP", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("task-mobile-dark.png"),
    animations: "disabled",
  });
  const source = await readFile("../../internal/blind/testdata/clean.png");
  expect(createHash("sha256").update(source).digest("hex")).toBe(
    created.sources[0].source_sha256,
  );
  expect(errors).toEqual([]);
});
