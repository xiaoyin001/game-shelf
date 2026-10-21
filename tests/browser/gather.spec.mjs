// 文件说明：双浏览器验证采集分数与冷却共享，不直接操纵游戏状态。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";
test("共享采集只奖励一次，离开重进重置本轮分数", async ({ browser }) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("gather");
    await join(pa, name);
    await join(pb, unique("peer"));
    await pa.keyboard.press("e");
    await expect(pa.locator("#score")).toHaveText("1");
    await expect(pb.locator("#players")).toContainText("✦ 1");
    await pb.keyboard.press("e");
    await expect(pb.locator("#notice")).toContainText(/没有可采集|附近没有/);
    await expect(pb.locator("#score")).toHaveText("0");
    await pa.getByRole("button", { name: "离开世界" }).click();
    await expect(pa.locator("#status")).toHaveText("连接已断开");
    await pa.getByRole("button", { name: /进入原野/ }).click();
    await expect(pa.locator("#status")).toContainText("已连接");
    await expect(pa.locator("#score")).toHaveText("0");
  } finally {
    await a.close();
    await b.close();
  }
});
