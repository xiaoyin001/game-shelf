// 文件说明：真实表单/公开查询验证两类榜单、个人最佳、窄屏和查询丢包的明确失败，不注入分数。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";

test("真实每日榜可切日期与模式，个人最佳独立且390px无溢出", async ({ page }, info) => {
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await join(page, unique("rank"));
  await page.locator("#rankings > summary").click();
  await expect(page.locator("#ranking-state")).toContainText("UTC");
  await expect(page.locator("#best-solo")).toContainText("尚无已保存有效成绩");
  await expect(page.locator("#best-duo")).toContainText("尚无已保存有效成绩");
  await page.locator("#rankings-day").fill("2099-12-31");
  await page.locator("#rankings-mode").selectOption("duo");
  await page.locator("#rankings-submit").click();
  await expect(page.locator("#ranking-state")).toContainText("2099-12-31 UTC");
  await expect(page.locator("#ranking-state")).toContainText("双人固定组合");
  await expect(page.locator("#ranking-state")).toContainText("暂无有效成绩");
  await expect(page.locator("#ranking-list li")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
  await page.locator("#rankings-mode").selectOption("solo");
  await page.locator("#rankings-submit").click();
  await expect(page.locator("#rankings")).toHaveAttribute("data-mode", "solo");
  await expect(page.locator("#ranking-state")).toContainText("单人每日");
  await page.locator("#rankings").scrollIntoViewIfNeeded();
  await page.screenshot({ path: info.outputPath("daily-rankings-mobile.png"), fullPage: false });
  expect(errors).toEqual([]);
});
test("榜单查询丢包明确显示未完成，不伪造空榜或个人最佳", async ({ page }) => {
  await page.routeWebSocket("**/ws", (route) => {
    const server = route.connectToServer();
    route.onMessage((data) => {
      if (JSON.parse(data).type !== "leaderboard") server.send(data);
    });
  });
  await join(page, unique("rank-fail"));
  await page.locator("#rankings > summary").click();
  await expect(page.locator("#ranking-state")).toContainText("正在读取");
  await expect(page.locator("#ranking-state")).toContainText("查询未完成", { timeout: 8000 });
  await expect(page.locator("#ranking-state")).not.toContainText("暂无有效成绩");
  await expect(page.locator("#best-solo")).toContainText("未能读取");
  await expect(page.locator("#best-duo")).toContainText("未能读取");
  await expect(page.locator("#rankings-submit")).toBeEnabled();
  await expect(page.locator("#world")).toHaveAttribute("data-renderer", "webgl2");
});
