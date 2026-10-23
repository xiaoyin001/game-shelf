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
    await Promise.all([a.close(), b.close()]);
  }
});

test("密码输入拒绝中文、非法粘贴和绕过 HTML 的提交，合法粘贴保留原样", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/");
  await expect(page.locator("#renderer-status")).toHaveText("3D · WebGL2");
  const input = page.getByLabel("密码", { exact: true });
  await expect(input).toHaveAttribute("minlength", "6");
  await expect(input).toHaveAttribute("maxlength", "32");
  await expect(page.locator("#password-hint")).toContainText("6–32");
  const posted = [];
  page.on("request", (request) => {
    if (request.method() === "POST" && /\/(register|login)$/.test(new URL(request.url()).pathname))
      posted.push(request.url());
  });
  await page.getByLabel("账号", { exact: true }).fill(unique("input"));
  await input.fill("Abc123");
  await input.press("End");
  await page.keyboard.insertText("中文");
  await expect(input).toHaveValue("Abc123");
  await expect(page.locator("#notice")).toContainText("仅限英文字母");
  // 模拟输入法或自动填充直接触发 input 的分支，不把事件模拟称为操作真实系统输入法。
  await input.evaluate((element) => {
    element.value = "Abc123汉";
    element.dispatchEvent(new InputEvent("input", { bubbles: true, isComposing: true }));
  });
  await expect(input).toHaveValue("Abc123");
  for (const value of ["Password中文123!", "Pass😀123", "Pass 123", "a".repeat(33)]) {
    await input.fill("Abc123");
    await input.selectText();
    await page.evaluate((value) => navigator.clipboard.writeText(value), value);
    await page.keyboard.press("Control+V");
    await expect(input).toHaveValue("Abc123");
    expect(await input.evaluate((element) => element.validity.customError)).toBe(true);
    await page.getByRole("button", { name: "注册并进入原野" }).click();
  }
  expect(posted).toEqual([]);
  await input.fill("");
  const legal = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~";
  await input.focus();
  await page.evaluate((value) => navigator.clipboard.writeText(value), legal);
  await page.keyboard.press("Control+V");
  await expect(input).toHaveValue(legal);
  expect(await input.evaluate((element) => element.checkValidity())).toBe(true);
  for (const value of ["12345", "a".repeat(33), "Password中文123!"]) {
    await page.evaluate((value) => {
      document.getElementById("password").value = value;
      document.getElementById("login-form").dispatchEvent(
        new SubmitEvent("submit", {
          cancelable: true,
          submitter: document.getElementById("join-button"),
        }),
      );
    }, value);
    await expect(page.locator("#notice")).toContainText("6–32");
  }
  expect(posted).toEqual([]);
});

test("6 位和 32 位密码可以通过真实表单注册并重新登录", async ({ page }) => {
  for (const value of ["123456", "A".repeat(32)]) {
    await page.goto("/");
    await page.getByLabel("账号", { exact: true }).fill(unique("bound"));
    await page.getByLabel("密码", { exact: true }).fill(value);
    await page.getByRole("button", { name: "注册并进入原野" }).click();
    await expect(page.locator("#status")).toContainText("已连接");
    await page.getByRole("button", { name: "离开世界" }).click();
    await expect(page.locator("#entry-panel")).toBeVisible();
    await page.getByLabel("密码", { exact: true }).fill(value);
    await page.getByRole("button", { name: /登录并进入原野/ }).click();
    await expect(page.locator("#status")).toContainText("已连接");
    await page.getByRole("button", { name: "离开世界" }).click();
    await expect(page.locator("#entry-panel")).toBeVisible();
  }
});
