import { expect, type Page } from "@playwright/test";
export async function configure(page: Page, advanced = true) {
  const back = page.locator(".workspace-mobile-back");
  if (await back.isVisible()) await back.click();
  const header = page.locator(".advanced-settings .ant-collapse-header");
  if (
    advanced &&
    (await header.isVisible()) &&
    (await header.getAttribute("aria-expanded")) === "false"
  ) {
    await header.click();
    await expect(header).toHaveAttribute("aria-expanded", "true");
  }
}
export async function review(page: Page) {
  const next = page.locator(".workspace-mobile-next");
  if (await next.isVisible()) await next.click();
}
