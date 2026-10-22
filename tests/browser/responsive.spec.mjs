// 文件说明：验证窄屏真实账号注册和可见地图，不将此视为触屏移动支持。
import { test, expect } from "@playwright/test";
import { unique, password } from "./helpers.mjs";
test("窄屏可操作进入且不横向溢出", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(page.getByLabel("账号", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
  await page.getByLabel("账号", { exact: true }).fill(unique("mobile"));
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "注册并进入原野" }).click();
  await expect(page.locator("#status")).toContainText("已连接");
  await expect(page.locator("#world")).toBeVisible();
});
