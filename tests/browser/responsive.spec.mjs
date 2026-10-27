// 文件说明：验证常用窗口的完整密码表单和窄屏真实注册，不将此视为触屏移动支持。
import { test, expect } from "@playwright/test";
import { unique, password } from "./helpers.mjs";
test("常用窗口密码表单不裁切，窄屏可进入且不横向溢出", async ({ page }) => {
  for (const [width, height] of [
    [1150, 900],
    [1280, 720],
    [390, 844],
  ]) {
    await page.setViewportSize({ width, height });
    await page.goto("/");
    const panel = await page.locator("#entry-panel").boundingBox();
    const frame = await page.locator(".canvas-wrap").boundingBox();
    expect(panel.y).toBeGreaterThanOrEqual(frame.y);
    expect(panel.y + panel.height).toBeLessThanOrEqual(frame.y + frame.height);
    await expect(page.locator("#password-hint")).toBeVisible();
    await expect(page.locator("#register-button")).toBeVisible();
  }
  await expect(page.getByLabel("账号", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
  await page.getByLabel("账号", { exact: true }).fill(unique("mobile"));
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: "注册并进入原野" }).click();
  await expect(page.locator("#status")).toContainText("已连接");
  await expect(page.locator("#world")).toBeVisible();
  await page.getByRole("tab", { name: "微光展台", exact: true }).click();
  await expect(page.getByLabel("查看谁的展台")).toBeVisible();
  await expect(page.getByLabel("左台摆件")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
  const camp = await page.locator("#camp-panel").boundingBox();
  expect(camp.x).toBeGreaterThanOrEqual(0);
  expect(camp.x + camp.width).toBeLessThanOrEqual(390);
});
