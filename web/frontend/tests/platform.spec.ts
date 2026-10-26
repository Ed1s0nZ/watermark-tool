import { test, expect } from "@playwright/test";
import { Buffer } from "node:buffer";
import { readFile, open } from "node:fs/promises";

test("authenticated platform, watermark round trip, preferences and mobile navigation", async ({
  page,
  request,
}, testInfo) => {
  async function chooseWithKeyboard(label: string, option: string) {
    const field = page.getByRole("combobox", { name: label }).first();
    await field.press("ArrowDown");
    await expect(field).toHaveAttribute("aria-expanded", "true");
    const active = page.locator(
      ".ant-select-dropdown:visible .ant-select-item-option-active",
    );
    await expect(active).toBeVisible();
    for (
      let i = 0;
      i < 6 && (await active.getAttribute("title")) !== option;
      i++
    )
      await field.press("ArrowDown");
    await expect(active).toHaveAttribute("title", option);
    await field.press("Enter");
    await expect(
      page.locator(".topbar [role=combobox][aria-expanded=true]"),
    ).toHaveCount(0);
    await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  }
  async function expectFullHeightSidebar() {
    const bounds = await page
      .locator(".desktop-sidebar")
      .evaluate((el) => ({
        top: el.getBoundingClientRect().top,
        bottom: el.getBoundingClientRect().bottom,
        height: innerHeight,
      }));
    expect(bounds.top).toBe(0);
    expect(bounds.bottom).toBe(bounds.height);
  }
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  expect((await request.get("/api/supported-types")).status()).toBe(401);
  expect((await request.post("/api/add-watermark")).status()).toBe(401);
  expect(
    (
      await request.post("/api/auth/login", {
        headers: { Origin: "https://other.example" },
        data: {},
      })
    ).status(),
  ).toBe(403);
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "欢迎回来" })).toBeVisible();
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath("login.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.getByRole("button", { name: "登录工作台", exact: true }).click();
  await expect(page.getByText("请填写此项").first()).toBeVisible();
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page.getByLabel("密码", { exact: true }).fill("wrong");
  await page.getByRole("button", { name: "登录工作台", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("用户名或密码错误");
  await page
    .getByLabel("密码", { exact: true })
    .fill("local-test-password-2026");
  await page.getByRole("button", { name: "登录工作台", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "工作台", exact: true }),
  ).toBeVisible();
  expect(
    (
      await (await page.request.get("/api/supported-types")).json()
    ).types.sort(),
  ).toEqual(["docx", "jpg", "odt", "pdf", "png", "pptx", "rtf", "xlsx"]);
  await page.getByRole("button", { name: "折叠侧边栏" }).click();
  await expect(page.getByRole("button", { name: "展开侧边栏" })).toBeVisible();
  await page.getByRole("button", { name: "展开侧边栏" }).click();
  await expectFullHeightSidebar();
  await page.setViewportSize({ width: 1280, height: 900 });
  await expectFullHeightSidebar();
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.getByRole("button", { name: "开始添加" }).click();
  await page
    .locator(".content")
    .evaluate((el) => el.scrollTo(0, el.scrollHeight));
  await expectFullHeightSidebar();
  await expect(page.locator(".topbar")).toBeInViewport();
  await page.locator(".content").evaluate((el) => el.scrollTo(0, 0));
  await page.getByRole("button", { name: "生成水印文件" }).click();
  await expect(page.getByText("请先选择文件", { exact: true })).toBeVisible();
  await page
    .getByPlaceholder("例如：内部资料 · 项目编号 · 所属人")
    .fill("内部资料-2026");
  const file = Buffer.from(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg==",
    "base64",
  );

  const add = page
    .locator(".content-inner > div")
    .filter({ has: page.getByRole("heading", { name: "为文档添加隐形水印" }) });
  await add.locator("input[type=file]").setInputFiles({
    name: "invalid.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("no"),
  });
  await expect(
    page.getByText("不支持此文件格式，请选择列表中的格式。", { exact: true }),
  ).toBeVisible();
  const oversizedPath = testInfo.outputPath("oversized.png");
  const oversized = await open(oversizedPath, "w");
  await oversized.truncate(50 * 1024 * 1024 + 1);
  await oversized.close();
  await add.locator("input[type=file]").setInputFiles(oversizedPath);
  await expect(
    page.getByText("文件过大，请上传不超过 50 MB 的文件。", { exact: true }),
  ).toBeVisible();
  await add
    .locator("input[type=file]")
    .setInputFiles({ name: "source.png", mimeType: "image/png", buffer: file });
  await page
    .getByPlaceholder("例如：内部资料 · 项目编号 · 所属人")
    .fill("中".repeat(34));
  await page.getByRole("button", { name: "生成水印文件" }).click();
  await expect(
    page.getByText("水印内容不能超过 100 字节", { exact: true }),
  ).toBeVisible();
  await page
    .getByPlaceholder("例如：内部资料 · 项目编号 · 所属人")
    .fill("内部资料-2026");
  await chooseWithKeyboard("语言", "English");
  await expect(
    page.getByPlaceholder("e.g. Internal · Project ID · Document owner"),
  ).toHaveValue("内部资料-2026");
  await chooseWithKeyboard("Appearance", "Dark");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.route("**/api/add-watermark", (route) => route.abort(), {
    times: 1,
  });
  await page.getByRole("button", { name: "Create watermarked file" }).click();
  await expect(
    page.getByText(
      "Unable to reach the server. Check your connection and try again.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(
    page.getByPlaceholder("e.g. Internal · Project ID · Document owner"),
  ).toHaveValue("内部资料-2026");
  // Delay one real operation so the in-flight disabled state is observable.
  await page.route("**/api/add-watermark", async (route) => {
    await new Promise((r) => setTimeout(r, 300));
    await route.continue();
  });
  await page.getByRole("button", { name: "Create watermarked file" }).click();
  await expect(
    page.getByText("Processing your file. Please wait…"),
  ).toBeVisible();
  const link = page.getByRole("link", { name: "Download watermarked file" });
  await expect(link).toBeVisible();
  const downloadPromise = page.waitForEvent("download");
  await link.click();
  const download = await downloadPromise;
  const downloadPath = await download.path();
  expect(downloadPath).toBeTruthy();
  await page.getByRole("menuitem", { name: "Extract watermark" }).click();
  const extract = page.locator(".content-inner > div").filter({
    has: page.getByRole("heading", {
      name: "Extract a watermark",
      exact: true,
    }),
  });
  await extract.locator("input[type=file]").setInputFiles({
    name: "watermarked.png",
    mimeType: "image/png",
    buffer: await readFile(downloadPath!),
  });
  await page
    .getByRole("button", { name: "Extract watermark", exact: true })
    .click();
  await expect(page.locator(".result-text")).toHaveText("内部资料-2026");
  await expect(page.getByText("Created at", { exact: true })).toBeVisible();
  await expect(extract).not.toContainText("No time information returned");
  await page
    .getByRole("checkbox", { name: "Show watermark creation time" })
    .uncheck();
  await page
    .getByRole("button", { name: "Extract watermark", exact: true })
    .click();
  await expect(page.locator(".result-text")).toHaveText("内部资料-2026");
  await expect(page.getByText("Created at", { exact: true })).not.toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Extract a watermark", exact: true }),
  ).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await chooseWithKeyboard("Appearance", "System");
  await page.emulateMedia({ colorScheme: "light" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("menuitem", { name: "Overview" }).click();
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath("desktop-dark.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.emulateMedia({ colorScheme: "light" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath("desktop-light.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(
    page.getByRole("button", { name: "Open navigation" }),
  ).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
    )
    .toBe(true);
  await page.getByRole("button", { name: "Open navigation" }).click();
  await page
    .getByRole("dialog")
    .getByRole("menuitem", { name: "Settings" })
    .click();
  await expect(
    page.getByRole("heading", { name: "Settings", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).not.toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: testInfo.outputPath("mobile-settings.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.getByRole("button", { name: "Open navigation" }).click();
  await page
    .getByRole("dialog")
    .getByRole("menuitem", { name: "Add watermark" })
    .click();
  await page
    .getByPlaceholder("e.g. Internal · Project ID · Document owner")
    .fill("draft after expiry");
  await page
    .locator(".content-inner > div")
    .filter({
      has: page.getByRole("heading", { name: "Add an invisible watermark" }),
    })
    .locator("input[type=file]")
    .setInputFiles({ name: "draft.png", mimeType: "image/png", buffer: file });
  await page.request.post("/api/auth/logout");
  await page.getByRole("button", { name: "Create watermarked file" }).click();
  await expect(
    page
      .getByText("Your session has expired. Please sign in again.", {
        exact: true,
      })
      .first(),
  ).toBeVisible();
  await page.getByLabel("Username", { exact: true }).fill("admin");
  await page
    .getByLabel("Password", { exact: true })
    .fill("local-test-password-2026");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByPlaceholder("e.g. Internal · Project ID · Document owner"),
  ).toHaveValue("draft after expiry");
  await page.getByRole("button", { name: "Current account" }).click();
  await page.getByText("Sign out", { exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sign in", exact: true }),
  ).toBeVisible();
  expect((await page.request.get("/api/supported-types")).status()).toBe(401);
  expect(errors).toEqual([]);
});
