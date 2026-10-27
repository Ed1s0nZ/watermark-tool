import { Page, Download, expect } from "@playwright/test";
export async function login(page: Page) {
  await page.goto("/");
  await page.getByLabel("用户名", { exact: true }).fill("admin");
  await page
    .getByLabel("密码", { exact: true })
    .fill("local-test-password-2026");
  await page.getByRole("button", { name: "登录工作台", exact: true }).click();
  await expect(
    page.getByRole("menuitem", { name: "多图分发任务", exact: true }),
  ).toBeVisible();
}
export async function choose(page: Page, label: string, title: string) {
  const control = page.getByRole("combobox", { name: label, exact: true });
  await control.scrollIntoViewIfNeeded();
  await control.press("Enter");
  await page
    .locator(".ant-select-dropdown:visible")
    .getByTitle(title, { exact: true })
    .click();
}
export async function logoBytes(page: Page) {
  return Buffer.from(
    await page.evaluate(async () => {
      const c = document.createElement("canvas");
      c.width = 160;
      c.height = 64;
      const x = c.getContext("2d")!;
      x.fillStyle = "#ef6020";
      x.globalAlpha = 0.8;
      x.fillRect(12, 8, 136, 48);
      x.globalAlpha = 1;
      x.fillStyle = "#ffffff";
      x.font = "bold 28px sans-serif";
      x.fillText("WM", 42, 44);
      const b = await new Promise<Blob>((r) =>
        c.toBlob((b) => r(b!), "image/png"),
      );
      return Array.from(new Uint8Array(await b.arrayBuffer()));
    }),
  );
}
export async function downloaded(d: Download) {
  const parts: Buffer[] = [];
  for await (const part of (await d.createReadStream())!)
    parts.push(Buffer.from(part));
  return Buffer.concat(parts);
}
export async function drawRegion(
  page: Page,
  x = 0.7,
  y = 0.7,
  width = 0.25,
  height = 0.25,
) {
  const c = page.getByTestId("exclusion-canvas");
  await c.scrollIntoViewIfNeeded();
  const b = (await c.boundingBox())!;
  await page.mouse.move(b.x + b.width * x, b.y + b.height * y);
  await page.mouse.down();
  await page.mouse.move(
    b.x + b.width * (x + width),
    b.y + b.height * (y + height),
    { steps: 8 },
  );
  await page.mouse.up();
}
