import { expect, type Page } from "@playwright/test";
const groups: Record<string, string[]> = {
  水印制作: ["添加水印", "双水印模板"],
  分发管理: ["多图分发任务", "批量副本", "分发记录"],
  验证分析: ["提取水印", "批量溯源", "效果实验室"],
  系统管理: ["密钥版本", "操作审计", "设置"],
  "Watermark creation": ["Add watermark", "Dual watermark templates"],
  Distribution: ["Multi-image delivery", "Batch copies", "Distribution records"],
  Verification: ["Extract watermark", "Batch tracing", "Effect laboratory"],
  "System management": ["Key versions", "Operation audit", "Settings"],
};
export async function navigate(page: Page, label: string) {
  const target = page.getByRole("menuitem", { name: label, exact: true });
  if (!(await target.isVisible())) {
    const parent = Object.entries(groups).find(([, leaves]) => leaves.includes(label))?.[0];
    if (parent) {
      const group = page.getByRole("menuitem", { name: parent, exact: true });
      await expect(group).toBeVisible();
      await group.click();
    }
  }
  await target.click();
}
