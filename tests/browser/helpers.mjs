// 文件说明：只封装真实页面进入动作，不注册测试或绕过用户输入。
import { expect } from "@playwright/test";
export const unique = (label) => "test-" + label + "-" + Date.now().toString(36);
export async function join(page, name) {
  await page.goto("/");
  await page.getByLabel("旅人名字").fill(name);
  await page.getByRole("button", { name: /进入原野/ }).click();
  await expect(page.locator("#status")).toContainText("已连接");
}
