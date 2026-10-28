// 文件说明：真实键盘穿过可见水池，比较服务器时间/坐标及入水离水提示，不改速度或坐标。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";
import { coordinate, walk } from "./challenge-helpers.mjs";
const sample = (page) =>
  page
    .locator("#coordinates")
    .evaluate((e) => ({ x: Number(e.dataset.x), t: Number(e.dataset.timeMs) }));
test("真实水池入水步行55%，离水恢复，HUD与权威位移一致", async ({ page }, info) => {
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await join(page, unique("water"));
  await page.locator("#start-solo").click();
  await expect(page.locator("#challenge")).toHaveAttribute("data-phase", "running");
  await walk(page, 1050, 465);
  await expect(page.locator("#coordinates")).toHaveAttribute("data-wading", "false");
  await page.locator("#world").focus();
  await page.keyboard.down("a");
  try {
    await expect(page.locator("#coordinates")).toHaveAttribute("data-wading", "true");
    await expect(page.locator("#terrain-status")).toContainText("55%");
    await expect.poll(() => coordinate(page, "x"), { intervals: [25] }).toBeLessThan(1000);
    const wetStart = await sample(page);
    await expect
      .poll(() => coordinate(page, "x"), { intervals: [25] })
      .toBeLessThan(wetStart.x - 45);
    const wetEnd = await sample(page),
      wetSpeed = ((wetStart.x - wetEnd.x) * 1000) / (wetEnd.t - wetStart.t);
    expect(wetSpeed).toBeGreaterThan(75);
    expect(wetSpeed).toBeLessThan(120);
    await expect.poll(() => coordinate(page, "x"), { intervals: [25] }).toBeLessThan(825);
    await expect(page.locator("#coordinates")).toHaveAttribute("data-wading", "false");
    await expect(page.locator("#terrain-status")).toContainText("正常移速");
    const dryStart = await sample(page);
    await expect
      .poll(() => coordinate(page, "x"), { intervals: [25] })
      .toBeLessThan(dryStart.x - 55);
    const dryEnd = await sample(page),
      drySpeed = ((dryStart.x - dryEnd.x) * 1000) / (dryEnd.t - dryStart.t);
    expect(drySpeed).toBeGreaterThan(145);
    expect(drySpeed).toBeLessThan(200);
    expect(drySpeed / wetSpeed).toBeGreaterThan(1.5);
  } finally {
    await page.keyboard.up("a");
  }
  await page.screenshot({ path: info.outputPath("water-crossing.png"), fullPage: false });
  expect(errors).toEqual([]);
});
