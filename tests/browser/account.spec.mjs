// 文件说明：真实表单验证密码拒绝、不错误顶号和合法重登，不用脚本直改世界状态。
import { test, expect } from "@playwright/test";
import { join, unique, password } from "./helpers.mjs";
test("账号密码注册、错密拒绝且不顶号、正确密码重登", async ({ browser }) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("password");
    await join(pa, name);
    await expect(pa.locator("#password")).toHaveValue("");
    await pb.goto("/");
    await expect(pb.getByLabel("密码", { exact: true })).toHaveAttribute("type", "password");
    await pb.getByLabel("账号", { exact: true }).fill(name);
    await pb.getByLabel("密码", { exact: true }).fill("Wrong-passphrase-33!");
    await pb.getByRole("button", { name: /登录并进入原野/ }).click();
    await expect(pb.locator("#notice")).toContainText("账号或密码错误");
    await expect(pa.locator("#status")).toContainText("已连接");
    await pb.getByLabel("密码", { exact: true }).fill(password);
    await pb.getByRole("button", { name: /登录并进入原野/ }).click();
    await expect(pb.locator("#status")).toContainText("已连接");
    await expect(pa.locator("#status")).toHaveText("连接已断开");
  } finally {
    await a.close();
    await b.close();
  }
});
