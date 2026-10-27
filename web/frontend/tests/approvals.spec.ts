import { test, expect } from "@playwright/test";
import { login, choose } from "./artwork-helpers";
import { navigate } from "./navigation";
test("server-verified review, self approval denied, protected routes and approved generation", async ({
  page,
  browser,
}, info) => {
  test.setTimeout(150000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await login(page);
  const created = await page.request.post("/api/workspace/teams", {
    data: { name: "Approval team" },
  });
  expect(created.ok()).toBeTruthy();
  let team = await created.json();
  expect(
    (
      await page.request.post("/api/workspace/users", {
        data: {
          username: "task-reviewer",
          password: "task-reviewer-password-2026",
        },
      })
    ).ok(),
  ).toBeTruthy();
  const assigned = await page.request.put(
    `/api/workspace/teams/${team.id}/members/task-reviewer`,
    { data: { role: "reviewer", revision: team.revision } },
  );
  expect(assigned.ok()).toBeTruthy();
  team = await assigned.json();
  const setting = await page.request.put(
    `/api/workspace/teams/${team.id}/approval-settings`,
    { data: { require_approval: true, revision: team.revision } },
  );
  expect(setting.ok()).toBeTruthy();
  await page.reload();
  await choose(page, "选择团队空间", "Approval team");
  for (const path of [
    "/add-watermark",
    "/workbench/dual",
    "/workbench/lab",
    "/platform/batches",
    "/platform/batches/00000000000000000000000000000000/download",
  ]) {
    const result = await page.request.post(
      `/api/workspace/teams/${team.id}/engine/api${path}`,
    );
    expect(result.status()).toBe(409);
    expect((await result.json()).code).toBe("approval_required");
  }
  await navigate(page, "多图分发任务");
  await page
    .getByLabel("任务原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page
    .getByLabel("任务接收人 CSV", { exact: true })
    .fill("recipient_id,name\n001,Reviewed recipient");
  await page.getByRole("button", { name: "校验并预览", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "创建并上传待审任务", exact: true }),
  ).toBeEnabled();
  await page
    .getByRole("button", { name: "创建并上传待审任务", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "提交审批", exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole("button", { name: "执行已批准任务", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "提交审批", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "提交审批", exact: true }),
  ).toBeDisabled();
  const member = await browser.newContext();
  const reviewer = await member.newPage();
  reviewer.on("pageerror", (e) => errors.push(e.message));
  await reviewer.goto("/");
  await reviewer.getByLabel("用户名", { exact: true }).fill("task-reviewer");
  await reviewer
    .getByLabel("密码", { exact: true })
    .fill("task-reviewer-password-2026");
  await reviewer
    .getByRole("button", { name: "登录工作台", exact: true })
    .click();
  await navigate(reviewer, "审批中心");
  await reviewer
    .getByRole("button", { name: "查看与审批", exact: true })
    .click();
  const modal = reviewer.getByRole("dialog");
  await expect(
    modal.getByRole("img", { name: "服务器输入预览", exact: true }),
  ).toBeVisible();
  expect(
    await modal
      .getByRole("img", { name: "服务器输入预览", exact: true })
      .evaluate((img: HTMLImageElement) => img.naturalWidth),
  ).toBeLessThanOrEqual(256);
  await modal
    .getByLabel("审阅说明")
    .fill("Reviewed actual uploaded original and recipient snapshot");
  await reviewer.screenshot({
    path: info.outputPath("approval-review-desktop.png"),
    animations: "disabled",
  });
  await modal.getByRole("button", { name: "批准", exact: true }).click();
  await reviewer.locator(".ant-popconfirm").getByRole("button").last().click();
  await expect(modal).toHaveCount(0);
  await page.getByRole("button", { name: "刷新审批", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "执行已批准任务", exact: true }),
  ).toBeEnabled();
  await page
    .getByRole("button", { name: "执行已批准任务", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "下载整套 ZIP", exact: true }),
  ).toBeEnabled({ timeout: 30000 });
  const download = page.waitForEvent("download", { timeout: 90000 });
  const responseReady = page.waitForResponse((response) =>
    response.url().endsWith("/archive"),
  );
  await page.getByRole("button", { name: "下载整套 ZIP", exact: true }).click();
  const response = await responseReady;
  if (response.status() === 429) {
    const retryAfter = Number(response.headers()["retry-after"]);
    expect(retryAfter).toBeGreaterThan(0);
    expect(retryAfter).toBeLessThanOrEqual(60);
    // Honor the real production rate window after account/team setup; do not weaken limits for tests.
    await page.waitForTimeout(retryAfter * 1000 + 100);
    await page
      .getByRole("button", { name: "下载整套 ZIP", exact: true })
      .click();
  } else expect(response.status()).toBe(200);
  expect((await download).suggestedFilename()).toMatch(/\.zip$/);
  await reviewer.setViewportSize({ width: 390, height: 844 });
  await choose(reviewer, "语言", "English");
  await choose(reviewer, "Appearance", "Dark");
  await expect(
    reviewer.getByRole("heading", { name: "Approvals", exact: true }),
  ).toBeVisible();
  expect(
    await reviewer.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await reviewer.screenshot({
    path: info.outputPath("approvals-mobile-dark.png"),
    animations: "disabled",
  });
  await member.close();
  expect(errors).toEqual([]);
});
