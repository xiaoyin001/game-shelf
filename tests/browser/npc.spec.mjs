// 文件说明：通过真实键盘和对话按钮完成营灯委托，验证联机奖励、交互输入隔离与窄屏。
import { test, expect } from "@playwright/test";
import { join, unique, password } from "./helpers.mjs";

async function walk(page, key, axis, threshold, greater = true) {
  await page.keyboard.down(key);
  try {
    await expect
      .poll(
        async () => {
          const value = Number(await page.locator("#coordinates").getAttribute("data-" + axis));
          return greater ? value >= threshold : value <= threshold;
        },
        { intervals: [100], timeout: 5000 },
      )
      .toBe(true);
  } finally {
    await page.keyboard.up(key);
  }
}

test("守灯人委托从交谈、采集到领取奖励，另一旅人只看到公共分数", async ({ browser }, info) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("npc");
    const errors = [];
    pa.on("pageerror", (e) => errors.push(e.message));
    pb.on("pageerror", (e) => errors.push(e.message));
    await join(pa, name);
    await join(pb, unique("watch"));
    await expect(pa.locator("#world")).toHaveAttribute("data-npc-count", "2");
    await expect(pa.locator("#npc-list")).toContainText("守灯人 · 阿苔");
    await walk(pa, "s", "y", 440);
    await expect(pa.locator("#interaction-hint")).toContainText("阿苔");
    await pa.keyboard.press("e");
    await expect(pa.getByRole("dialog")).toBeVisible();
    await pa.getByRole("button", { name: "接取委托：点亮营灯" }).click();
    await expect(pa.locator("#quest-progress")).toContainText("0 / 2");
    await expect(pb.locator("#quest-progress")).toHaveAttribute("data-status", "");
    await expect(pb.getByRole("dialog")).toBeHidden();
    await pa.keyboard.press("Escape");
    await expect(pa.getByRole("dialog")).toBeHidden();
    await walk(pa, "w", "y", 405, false);
    await walk(pa, "d", "x", 573);
    await expect(pa.locator("#interaction-hint")).toHaveText("E 采集微光", { timeout: 12000 });
    await pa.keyboard.press("e");
    await expect(pa.locator("#quest-progress")).toContainText("1 / 2");
    await expect(pa.locator("#interaction-hint")).toContainText("重生中");
    // 探索东南方第二处真实晶簇；同地等待重生由独立完整用例覆盖，不与整段委托耦合。
    await pa.keyboard.press("Space");
    await walk(pa, "s", "y", 450);
    await walk(pa, "d", "x", 820);
    await walk(pa, "s", "y", 550);
    await pa.keyboard.press("e");
    await expect(pa.locator("#quest-progress")).toContainText("2 / 2");
    // 松键余量可能停在晶簇北侧实体边缘；落地后真实跳过，不假定刚好停在切线。
    await expect(pa.locator("#coordinates")).toHaveAttribute("data-grounded", "true");
    await pa.keyboard.press("Space");
    await walk(pa, "a", "x", 580, false);
    await expect(pa.locator("#interaction-hint")).toContainText("阿苔");
    await pa.keyboard.press("e");
    await pa.getByRole("button", { name: "完成委托 · 领取 3 分微光" }).click();
    await expect(pa.locator("#score")).toHaveText("5");
    await expect(pa.locator("#quest-progress")).toContainText("已完成");
    await expect(
      pb.locator("#players li").filter({ hasText: name }).locator(".player-score"),
    ).toHaveText("✦ 5");
    await expect(pa.getByRole("button", { name: /完成委托 · 领取/ })).toHaveCount(0);
    await pa.screenshot({ path: info.outputPath("keeper-quest-completed.png"), fullPage: true });
    await pa.getByRole("button", { name: "结束交谈 · Esc" }).click();
    await pa.getByRole("button", { name: "离开世界" }).click();
    await expect(pa.locator("#status")).toHaveText("连接已断开");
    await pa.getByLabel("密码", { exact: true }).fill(password);
    await pa.getByRole("button", { name: /登录并进入原野/ }).click();
    await expect(pa.locator("#status")).toContainText("已连接");
    await expect(pa.locator("#quest-progress")).toHaveAttribute("data-status", "");
    await expect(pa.locator("#score")).toHaveText("0");
    expect(errors).toEqual([]);
  } finally {
    await Promise.all([a.close(), b.close()]);
  }
});

test("晶簇自然重生后可以在原地再次采集", async ({ page }) => {
  await join(page, unique("regrow"));
  await walk(page, "d", "x", 573);
  await expect(page.locator("#interaction-hint")).toHaveText("E 采集微光", { timeout: 12000 });
  await page.keyboard.press("e");
  await expect(page.locator("#score")).toHaveText("1");
  await expect(page.locator("#interaction-hint")).toContainText("重生中");
  await expect(page.locator("#interaction-hint")).toHaveText("E 采集微光", { timeout: 12000 });
  await page.keyboard.press("e");
  await expect(page.locator("#score")).toHaveText("2");
  await expect(page.locator("#quest-progress")).toHaveAttribute("data-status", "");
});

test("巡林员提供不同提示，对话时不移动或跳跃，关闭后恢复输入", async ({ page }, info) => {
  await join(page, unique("ranger"));
  await walk(page, "w", "y", 260, false);
  await walk(page, "d", "x", 640);
  await expect(page.locator("#interaction-hint")).toContainText("林哨");
  await page.getByRole("button", { name: "交谈", exact: true }).click();
  await page.getByRole("button", { name: "哪里能找到晶簇？" }).click();
  await expect(page.locator("#npc-text")).toContainText("620 / 400");
  const x = Number(await page.locator("#coordinates").getAttribute("data-x"));
  await page.keyboard.down("d");
  await page.keyboard.press("Space");
  await page.waitForTimeout(400);
  await page.keyboard.up("d");
  expect(Number(await page.locator("#coordinates").getAttribute("data-x"))).toBe(x);
  await expect(page.locator("#coordinates")).toHaveAttribute("data-z", "0");
  await page.getByRole("button", { name: "营地里有什么？" }).click();
  await expect(page.locator("#npc-text")).toContainText("540 / 500");
  await page.screenshot({ path: info.outputPath("ranger-dialog.png"), fullPage: true });
  await page.keyboard.press("Escape");
  await walk(page, "a", "x", x - 20, false);
});

test("窄屏可用按钮交谈、查看委托提示并关闭面板", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await join(page, unique("npcsmall"));
  await walk(page, "s", "y", 440);
  await page.getByRole("button", { name: "交谈", exact: true }).click();
  await page.getByRole("button", { name: "告诉我采集方法" }).click();
  await expect(page.locator("#npc-text")).toContainText("10 秒");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const worldBounds = await page.locator(".canvas-wrap").boundingBox();
  expect(worldBounds.width).toBeLessThanOrEqual(390);
  const bounds = await page.getByRole("dialog").boundingBox();
  expect(bounds.x).toBeGreaterThanOrEqual(0);
  expect(bounds.x + bounds.width).toBeLessThanOrEqual(390);
  await page.getByRole("button", { name: "结束交谈 · Esc" }).click();
  await expect(page.getByRole("dialog")).toBeHidden();
});
