import { test, expect } from "@playwright/test";
import { login, choose } from "./artwork-helpers";
import { navigate } from "./navigation";
test("recipient CRUD, atomic import, explicit task snapshot and safe export", async ({
  page,
}, info) => {
  await login(page);
  await navigate(page, "接收人库");
  await page.getByRole("button", { name: "新增接收人", exact: true }).click();
  let modal = page.getByRole("dialog");
  await modal.getByLabel("接收人编号", { exact: true }).fill("directory-001");
  await modal.getByLabel("姓名", { exact: true }).fill("Original Alice");
  await modal
    .getByRole("combobox", { name: "分组", exact: true })
    .fill("Sales");
  await page.keyboard.press("Enter");
  await modal.getByRole("button", { name: "保存", exact: true }).click();
  await expect(
    page.getByRole("row").filter({ hasText: "Original Alice" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "导入 CSV", exact: true }).click();
  modal = page.getByRole("dialog");
  await modal
    .getByLabel("CSV 内容")
    .fill(
      "recipient_id,name,groups\ndirectory-002,Bob,Sales\ndirectory-001,Duplicate,Sales",
    );
  await modal.getByRole("button", { name: "导入 CSV", exact: true }).click();
  await expect(modal.getByText("错误行: 3")).toBeVisible();
  await modal
    .getByLabel("CSV 内容")
    .fill("recipient_id,name,groups\ndirectory-002,Bob,Sales");
  await modal.getByRole("button", { name: "导入 CSV", exact: true }).click();
  await expect(page.getByRole("row").filter({ hasText: "Bob" })).toHaveCount(1);
  const exportReady = page.waitForEvent("download");
  await page.getByRole("button", { name: "导出 JSON", exact: true }).click();
  expect((await exportReady).suggestedFilename()).toBe("recipients.json");
  await navigate(page, "批量副本");
  await page.getByRole("button", { name: "从接收人库选择" }).click();
  modal = page.getByRole("dialog");
  await modal
    .getByRole("row")
    .filter({ hasText: "Original Alice" })
    .getByRole("checkbox")
    .check();
  await modal.getByRole("button", { name: "填入当前名单" }).click();
  await expect(page.getByLabel("接收人名单 CSV", { exact: true })).toHaveValue(
    "recipient_id,name\ndirectory-001,Original Alice\n",
  );
  // A directory edit does not change an already selected snapshot or server selection response.
  const base = "/api/workspace/teams/personal/recipients";
  const found = await (
    await page.request.post(base + "/search", {
      data: { q: "directory-001", page: 1, page_size: 20 },
    })
  ).json();
  const contact = found.items[0];
  expect(
    (
      await page.request.put(base + "/" + contact.id, {
        data: {
          recipient_id: contact.recipient_id,
          name: "Renamed Alice",
          groups: contact.groups,
          revision: contact.revision,
        },
      })
    ).ok(),
  ).toBeTruthy();
  await expect(page.getByLabel("接收人名单 CSV", { exact: true })).toHaveValue(
    "recipient_id,name\ndirectory-001,Original Alice\n",
  );
  await page
    .getByLabel("批次原图", { exact: true })
    .setInputFiles("../../internal/blind/testdata/clean.png");
  const generated = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/platform/batches") &&
      response.request().method() === "POST",
  );
  await page.getByRole("button", { name: "生成批量副本", exact: true }).click();
  expect((await generated).status()).toBe(200);
  await expect(
    page.getByRole("link", { name: "下载 ZIP", exact: true }),
  ).toBeVisible();
  await navigate(page, "接收人库");
  const row = page.getByRole("row").filter({ hasText: "Renamed Alice" });
  await expect(row).toBeVisible();
  await row.getByRole("button", { name: "删除", exact: true }).click();
  await page.locator(".ant-popconfirm").getByRole("button").last().click();
  await expect(row).toHaveCount(0);
  await expect(page.getByRole("row").filter({ hasText: "Bob" })).toBeVisible();
  await page.screenshot({
    path: info.outputPath("recipients-desktop.png"),
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await choose(page, "语言", "English");
  await choose(page, "Appearance", "Dark");
  await expect(
    page.getByRole("heading", { name: "Recipients", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: info.outputPath("recipients-mobile-dark.png"),
    animations: "disabled",
  });
});
