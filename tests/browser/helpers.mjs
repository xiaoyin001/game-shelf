// 文件说明：通过实际账号表单注册或密码登录，不绕过验证或注册测试后门。
import { expect } from "@playwright/test";
export const password = "Testing-passphrase-93!";
export const unique = (label) => "test-" + label + "-" + Date.now().toString(36);
export async function join(page, name, { register = true } = {}) {
  await page.goto("/");
  await page.getByLabel("账号", { exact: true }).fill(name);
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: register ? "注册并进入原野" : /登录并进入原野/ }).click();
  await expect(page.locator("#status")).toContainText("已连接");
}
