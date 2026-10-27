import { test, expect } from "@playwright/test";
import { login, choose } from "./artwork-helpers";
import { configure } from "./editor-helpers";
import { navigate } from "./navigation";
test("team policy draft, publish, new version, rollback and operator application", async ({
  page,
  browser,
}) => {
  await login(page);
  const created = await page.request.post("/api/workspace/teams", {
    data: { name: "Policy team" },
  });
  expect(created.ok()).toBeTruthy();
  const team = await created.json();
  expect(
    (
      await page.request.post("/api/workspace/users", {
        data: {
          username: "policy-operator",
          password: "policy-operator-password-2026",
        },
      })
    ).ok(),
  ).toBeTruthy();
  expect(
    (
      await page.request.put(
        `/api/workspace/teams/${team.id}/members/policy-operator`,
        { data: { role: "operator", revision: team.revision } },
      )
    ).ok(),
  ).toBeTruthy();
  await page.reload();
  await choose(page, "选择团队空间", "Policy team");
  await navigate(page, "多图分发任务");
  await configure(page);
  await choose(page,"任务水印模式","可见与隐形双水印");
  await page.getByLabel("接收人文字模板", { exact: true }).fill("v1 {name}");
  await page.getByLabel("共享策略名称", { exact: true }).fill("Team standard");
  await page.getByRole("button", { name: "保存为团队策略", exact: true }).click();
  await expect(
    page.getByRole("combobox", { name: "策略版本", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "发布所选版本", exact: true }).click();
  await page.locator(".ant-popconfirm").getByRole("button").last().click();
  await expect(
    page.getByRole("button", { name: "发布所选版本", exact: true }),
  ).toBeDisabled();
  await page
    .getByLabel("接收人文字模板", { exact: true })
    .fill("v2 {recipient_id}");
  await page.getByRole("button", { name: "追加当前配置", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "发布所选版本", exact: true }),
  ).toBeEnabled();
  const member = await browser.newContext();
  const operator = await member.newPage();
  await operator.goto("/");
  await operator.getByLabel("用户名", { exact: true }).fill("policy-operator");
  await operator
    .getByLabel("密码", { exact: true })
    .fill("policy-operator-password-2026");
  await operator.getByRole("button", { name: "登录工作台", exact: true }).click();
  await navigate(operator, "多图分发任务");
  await configure(operator);
  await choose(operator, "选择共享策略", "Team standard");
  await operator
    .getByRole("button", { name: "应用所选版本", exact: true })
    .click();
  await expect(
    operator.getByLabel("接收人文字模板", { exact: true }),
  ).toHaveValue("v1 {name}");
  await expect(
    operator.getByRole("button", { name: "保存为团队策略", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "发布所选版本", exact: true }).click();
  await page.locator(".ant-popconfirm").getByRole("button").last().click();
  await expect(
    page.getByRole("button", { name: "发布所选版本", exact: true }),
  ).toBeDisabled();
  await choose(page, "策略版本", "v1 · 草稿");
  await page.getByRole("button", { name: "发布所选版本", exact: true }).click();
  await page.locator(".ant-popconfirm").getByRole("button").last().click();
  await expect(
    page.getByRole("button", { name: "发布所选版本", exact: true }),
  ).toBeDisabled();
  const policies = await (
    await page.request.get(`/api/workspace/teams/${team.id}/policies`)
  ).json();
  expect(policies.items[0].published).toBe(1);
  expect(policies.items[0].versions).toHaveLength(2);
  await member.close();
});
