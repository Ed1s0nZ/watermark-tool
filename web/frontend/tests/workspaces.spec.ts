import { configure } from "./editor-helpers";
import { test, expect } from "@playwright/test";
import { login, choose } from "./artwork-helpers";
import { navigate } from "./navigation";

test("team UI, scoped generation, member isolation, live role downgrade and account revocation", async ({
  page,
  browser,
}, info) => {
  test.setTimeout(120000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await login(page);
  await navigate(page, "团队与成员");
  await page.getByLabel("新成员用户名", { exact: true }).fill("team-alice");
  await page
    .getByLabel("初始密码", { exact: true })
    .fill("team-alice-password-2026");
  await page.getByRole("button", { name: "创建账号", exact: true }).click();
  const accounts = page.locator(".ant-card").filter({
    has: page
      .locator(".ant-card-head-title")
      .getByText("环境账号", { exact: true }),
  });
  await expect(
    accounts.getByRole("row").filter({ hasText: "team-alice" }),
  ).toBeVisible();
  await expect(page.getByLabel("初始密码", { exact: true })).toHaveValue("");
  expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain(
    "team-alice-password",
  );
  await page.getByLabel("团队名称", { exact: true }).fill("Team Alpha");
  await page.getByRole("button", { name: "新建团队", exact: true }).click();
  await expect(
    page.locator(".workspace-selector .ant-select-selection-item"),
  ).toHaveText("Team Alpha");
  await page.getByLabel("成员账号", { exact: true }).fill("team-alice");
  await page.getByRole("button", { name: "设置成员角色", exact: true }).click();
  await expect(
    page.getByRole("combobox", { name: "角色: team-alice", exact: true }),
  ).toBeVisible();
  let me = await (await page.request.get("/api/workspace/me")).json();
  const alpha = me.teams.find((team: any) => team.name === "Team Alpha").id;
  await navigate(page, "双水印模板");
  await configure(page);
  await page
    .getByLabel("模板名称", { exact: true })
    .fill("Alpha private style");
  await page.getByRole("button", { name: "保存样式", exact: true }).click();
  await navigate(page, "团队与成员");
  await page.getByLabel("团队名称", { exact: true }).fill("Team Beta");
  await page.getByRole("button", { name: "新建团队", exact: true }).click();
  await expect(
    page.locator(".workspace-selector .ant-select-selection-item"),
  ).toHaveText("Team Beta");
  me = await (await page.request.get("/api/workspace/me")).json();
  const beta = me.teams.find((team: any) => team.name === "Team Beta").id;
  await choose(page, "选择团队空间", "Team Alpha");
  await navigate(page, "批量副本");
  await page
    .getByLabel("批次原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  await page
    .getByLabel("接收人名单 CSV", { exact: true })
    .fill("recipient_id,name\n001,Member Alpha");
  const result = page.waitForResponse(
    (response) =>
      response
        .url()
        .endsWith(`/workspace/teams/${alpha}/engine/api/platform/batches`) &&
      response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "生成批量副本", exact: true }).click();
  const generated = await result;
  expect(generated.status()).toBe(200);
  const batch = await generated.headerValue("x-batch-id");
  expect(batch).toBeTruthy();
  await expect(
    page.getByRole("link", { name: "下载 ZIP", exact: true }),
  ).toBeVisible();
  await choose(page, "选择团队空间", "Team Beta");
  await expect(
    page.getByRole("link", { name: "下载 ZIP", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.locator('[data-upload-label="批次原图"] .selected-file'),
  ).toHaveCount(0);
  await navigate(page, "双水印模板");
  await configure(page);
  await page
    .getByRole("combobox", { name: "模板", exact: true })
    .press("Enter");
  await expect(
    page.getByTitle("Alpha private style", { exact: true }),
  ).toHaveCount(0);
  await page.keyboard.press("Escape");
  await navigate(page, "批量副本");
  const separate = await (
    await page.request.post(
      `/api/workspace/teams/${beta}/engine/api/platform/records/search`,
      { data: {} },
    )
  ).json();
  expect(separate.total).toBe(0);
  const memberContext = await browser.newContext();
  const member = await memberContext.newPage();
  member.on("pageerror", (error) => errors.push(error.message));
  await member.goto("http://127.0.0.1:18080/");
  await member.getByLabel("用户名", { exact: true }).fill("team-alice");
  await member
    .getByLabel("密码", { exact: true })
    .fill("team-alice-password-2026");
  await member.getByRole("button", { name: "登录工作台", exact: true }).click();
  await expect(
    member.locator(".workspace-selector .ant-select-selection-item"),
  ).toHaveText("Team Alpha");
  const visible = await (
    await member.request.get("http://127.0.0.1:18080/api/workspace/me")
  ).json();
  expect(visible.teams.map((team: any) => team.id)).toEqual([alpha]);
  expect(
    (
      await member.request.get(
        `http://127.0.0.1:18080/api/workspace/teams/${beta}/engine/api/platform/keys`,
      )
    ).status(),
  ).toBe(403);
  expect(
    (
      await member.request.post(
        "http://127.0.0.1:18080/api/platform/records/search",
        { data: {} },
      )
    ).status(),
  ).toBe(403);
  await navigate(member, "分发记录");
  await expect(member.getByText(batch!, { exact: true }).first()).toBeVisible();
  await choose(page, "选择团队空间", "Team Alpha");
  await navigate(page, "团队与成员");
  await choose(page, "角色: team-alice", "审阅员");
  await member.reload();
  await expect(
    member.getByRole("menuitem", { name: "添加水印", exact: true }),
  ).toHaveCount(0);
  expect(
    (
      await member.request.post(
        `http://127.0.0.1:18080/api/workspace/teams/${alpha}/engine/api/jobs`,
        { data: {} },
      )
    ).status(),
  ).toBe(403);
  await member.goto("http://127.0.0.1:18080/#add");
  await expect(member).toHaveURL(/#overview$/);
  await expect(
    member.getByRole("heading", { name: "工作台", exact: true }),
  ).toBeVisible();
  const userRow = page
    .locator(".ant-card")
    .filter({
      has: page
        .locator(".ant-card-head-title")
        .getByText("环境账号", { exact: true }),
    })
    .getByRole("row")
    .filter({ hasText: "team-alice" });
  await userRow.getByRole("button", { name: "停用账号", exact: true }).click();
  await page.getByRole("button", { name: /确\s*定/ }).click();
  await expect(userRow.getByText("已停用", { exact: true })).toBeVisible();
  await member.reload();
  await expect(
    member.getByRole("heading", { name: "欢迎回来", exact: true }),
  ).toBeVisible();
  await memberContext.close();
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.locator(".content").evaluate((element) => (element.scrollTop = 0));
  await page.screenshot({
    path: info.outputPath("teams-desktop.png"),
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await choose(page, "语言", "English");
  await choose(page, "Appearance", "Dark");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("teams-mobile-dark.png"),
    animations: "disabled",
  });
  expect(errors).toEqual([]);
});
