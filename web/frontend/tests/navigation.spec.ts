import { test, expect } from "@playwright/test";
import { login, choose } from "./artwork-helpers";
import { navigate } from "./navigation";

test("two-level navigation, history, collapsed flyout and independent mobile drawer", async ({ page }, info) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.setViewportSize({ width: 1280, height: 720 });
  await login(page);
  const nav = page.getByRole("menu", { name: "主导航", exact: true });
  await expect(nav.getByRole("menuitem")).toHaveCount(5);
  const groups = [
    ["水印制作", [["添加水印", "add"], ["双水印模板", "dual"]]],
    ["分发管理", [["多图分发任务", "jobs"], ["批量副本", "batches"], ["分发记录", "records"]]],
    ["验证分析", [["提取水印", "extract"], ["批量溯源", "trace"], ["效果实验室", "lab"]]],
    ["系统管理", [["密钥版本", "keys"], ["操作审计", "audit"], ["设置", "settings"]]],
  ] as const;
  for (const [group, leaves] of groups) {
    await nav.getByRole("menuitem", { name: group, exact: true }).click();
    for (const [label, hash] of leaves) {
      await navigate(page, label);
      await expect(page).toHaveURL(new RegExp(`#${hash}$`));
      await expect(nav.locator(".ant-menu-item-selected")).toHaveText(label);
    }
  }
  await expect(nav.locator(".ant-menu-item")).toHaveCount(12);
  await expect(page.getByRole("button", { name: "折叠侧边栏" })).toBeInViewport();
  await page.setViewportSize({ width: 1280, height: 480 });
  await expect(page.getByRole("button", { name: "折叠侧边栏" })).toBeInViewport();
  expect(await page.locator(".desktop-sidebar").evaluate(el => el.getBoundingClientRect().bottom)).toBe(480);
  expect(await page.locator(".sidebar-navigation").evaluate(el => el.scrollHeight > el.clientHeight)).toBe(true);
  await page.setViewportSize({ width: 1280, height: 720 });
  // Direct link + reload expands only the active group in a new Shell.
  await page.goto("/#dual");
  await page.reload();
  await expect(nav.getByRole("menuitem", { name: "水印制作", exact: true })).toHaveAttribute("aria-expanded", "true");
  await expect(nav.locator(".ant-menu-item-selected")).toHaveText("双水印模板");
  await page.screenshot({ path: info.outputPath("grouped-desktop.png"), animations: "disabled" });
  await navigate(page, "操作审计");
  await page.goBack();
  await expect(page).toHaveURL(/#dual$/);
  await expect(nav.locator(".ant-menu-item-selected")).toHaveText("双水印模板");
  await page.goForward();
  await expect(nav.locator(".ant-menu-item-selected")).toHaveText("操作审计");
  const system = nav.getByRole("menuitem", { name: "系统管理", exact: true });
  await system.click();
  await expect(system).toHaveAttribute("aria-expanded", "false");
  await system.click();
  await expect(system).toHaveAttribute("aria-expanded", "true");
  await page.getByRole("button", { name: "折叠侧边栏" }).click();
  await nav.getByRole("menuitem", { name: "验证分析", exact: true }).hover();
  await expect(page.getByRole("menuitem", { name: "效果实验室", exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath("collapsed-flyout.png"), animations: "disabled" });
  await page.getByRole("menuitem", { name: "效果实验室", exact: true }).click();
  await expect(page).toHaveURL(/#lab$/);
  await expect(page.locator(".ant-menu-submenu-popup:visible")).toHaveCount(0);
  await page.getByRole("button", { name: "展开侧边栏" }).click();
  await expect(nav.locator(".ant-menu-item-selected")).toHaveText("效果实验室");
  await page.locator(".sidebar-brand").click();
  await expect(nav.locator(".ant-menu-item-selected")).toHaveText("工作台");
  await page.getByRole("button", { name: "开始添加" }).click();
  await expect(nav.locator(".ant-menu-item-selected")).toHaveText("添加水印");
  await page.getByRole("button", { name: "当前账号" }).click();
  await page.locator(".ant-dropdown:visible").getByText("设置", { exact: true }).click();
  await expect(nav.locator(".ant-menu-item-selected")).toHaveText("设置");
  await system.focus();
  await page.keyboard.press("Enter");
  await expect(system).toHaveAttribute("aria-expanded", "false");
  await page.keyboard.press("Enter");
  await expect(system).toHaveAttribute("aria-expanded", "true");
  await page.getByRole("button", { name: "折叠侧边栏" }).click();
  // The desktop stays collapsed while mobile uses a full inline menu.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "打开导航" }).click();
  const drawer = page.getByRole("dialog");
  const mobile = drawer.getByRole("menu", { name: "移动导航", exact: true });
  await expect(mobile).not.toHaveClass(/ant-menu-inline-collapsed/);
  await navigate(page, "双水印模板");
  await expect(drawer).not.toBeVisible();
  await expect(page).toHaveURL(/#dual$/);
  await page.getByRole("button", { name: "打开导航" }).click();
  await expect(mobile.locator(".ant-menu-item-selected")).toHaveText("双水印模板");
  await drawer.getByRole("button", { name: "关闭" }).click();
  await expect(drawer).not.toBeVisible();
  await choose(page, "语言", "English");
  await choose(page, "Appearance", "Dark");
  await page.getByRole("button", { name: "Open navigation" }).click();
  await navigate(page, "Batch tracing");
  await page.getByRole("button", { name: "Open navigation" }).click();
  await expect(drawer.getByRole("menuitem", { name: "Verification", exact: true })).toHaveAttribute("aria-expanded", "true");
  await expect(drawer.locator(".ant-menu-item-selected")).toHaveText("Batch tracing");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath("mobile-navigation-dark.png"), animations: "disabled" });
  expect(errors).toEqual([]);
});
