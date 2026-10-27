// 文件说明：真实浏览器领线索、实地调查、断线续接与收藏保存，不改写客户端或服务端状态。
import { test, expect } from "@playwright/test";
import { join, unique, password } from "./helpers.mjs";

async function walk(page, key, axis, target, greater = true) {
  await page.locator("#world").focus();
  await page.keyboard.down(key);
  try {
    await expect
      .poll(
        async () => {
          const value = Number(await page.locator("#coordinates").getAttribute("data-" + axis));
          return greater ? value >= target : value <= target;
        },
        { intervals: [100], timeout: 5000 },
      )
      .toBe(true);
  } finally {
    await page.keyboard.up(key);
  }
}
async function rejoin(page) {
  await page.getByRole("button", { name: "离开世界" }).click();
  await expect(page.locator("#status")).toHaveText("连接已断开");
  await expect(page.locator("#field-book")).toBeHidden();
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: /登录并进入原野/ }).click();
  await expect(page.locator("#status")).toContainText("已连接");
}

test("两步寻宝中途退出可续接，旧物与故事在密码重登后仍保留", async ({ page }, info) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await join(page, unique("book"));
  await expect(page.locator("#collection-count")).toHaveText("0 / 3");
  await walk(page, "s", "y", 440);
  await page.keyboard.press("e");
  await page.getByRole("button", { name: "寻找原野旧物 · 领取 / 回顾线索" }).click();
  await expect(page.locator("#trail-progress")).toHaveAttribute("data-step", "1");
  await page.keyboard.press("Escape");
  await walk(page, "d", "x", 600);
  await walk(page, "s", "y", 500);
  await expect(page.locator("#world")).toHaveAttribute("data-clue-visible", "true");
  await page.keyboard.press("f");
  await expect(page.locator("#trail-progress")).toHaveAttribute("data-step", "2");
  await rejoin(page);
  await expect(page.locator("#trail-progress")).toHaveAttribute("data-step", "2");
  await expect(page.locator("#trail-clue")).toContainText("圆石");
  await walk(page, "s", "y", 450);
  await walk(page, "d", "x", 610);
  await page.getByRole("button", { name: "调查线索 · F" }).click();
  await expect(page.locator("#collection-count")).toHaveText("1 / 3");
  const bell = page.locator('#collections details[data-id="bell"]');
  await expect(bell).toHaveAttribute("data-found", "true");
  await bell.locator("summary").click();
  await expect(bell.locator("p")).toContainText("第一位守灯人");
  await page.screenshot({ path: info.outputPath("field-book-found.png"), fullPage: false });
  await rejoin(page);
  await expect(page.locator("#collection-count")).toHaveText("1 / 3");
  await expect(page.locator("#score")).toHaveText("0");
  await expect(page.locator("#world")).toHaveAttribute("data-clue-id", "");
  expect(errors).toEqual([]);
});
