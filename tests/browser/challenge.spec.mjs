// 文件说明：通过真实表单、键盘和道具按钮验证单人搬运配对，不修改游戏状态或时钟。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";
const coordinate = async (page, axis) =>
  Number(await page.locator("#coordinates").getAttribute("data-" + axis));
async function walk(page, x, y) {
  await page.locator("#world").focus();
  for (const [axis, target, positive, negative] of [
    ["x", x, "d", "a"],
    ["y", y, "s", "w"],
  ]) {
    const delta = target - (await coordinate(page, axis));
    if (Math.abs(delta) < 18) continue;
    const sign = Math.sign(delta),
      key = sign > 0 ? positive : negative;
    await page.keyboard.down(key);
    try {
      await expect
        .poll(async () => ((await coordinate(page, axis)) - target) * sign, { intervals: [25] })
        .toBeGreaterThan(-14);
    } finally {
      await page.keyboard.up(key);
    }
  }
}
async function start(page, label) {
  await join(page, unique(label));
  await page.locator("#start-solo").click();
  await expect(page.locator("#challenge")).toHaveAttribute("data-phase", "running");
  await expect(page.locator("#board-items button")).toHaveCount(16);
}
async function ground(page) {
  return page.locator("#board-items button").evaluateAll((bs) =>
    bs.map((b) => ({
      ...b.dataset,
      x: Number(b.dataset.x),
      y: Number(b.dataset.y),
      z: Number(b.dataset.z),
    })),
  );
}
async function pick(page, item, keyboard = false) {
  const x = await coordinate(page, "x"),
    y = await coordinate(page, "y");
  const length = Math.hypot(item.x - x, item.y - y);
  const ratio = length > 40 ? (length - 40) / length : 0;
  await walk(page, x + (item.x - x) * ratio, y + (item.y - y) * ratio);
  // 只点击对应真实道具；数据属性来自公开权威快照，不是坐标写入后门。
  await page.locator(".rules").evaluate((e) => {
    e.open = false;
  });
  const details = page.locator("details").filter({ has: page.locator("#board-items") });
  await details.evaluate((e) => {
    e.open = true;
  });
  if (keyboard) await page.keyboard.press("e");
  else await page.locator('#board-items button[data-item-id="' + item.itemId + '"]').click();
  await expect(page.locator('#hand button[data-item-id="' + item.itemId + '"]')).toHaveCount(1);
}
async function put(page, item, keyboard = false) {
  await walk(page, 700, 350);
  if (keyboard) {
    const slot = Number(
      await page
        .locator('#hand button[data-item-id="' + item.itemId + '"]')
        .getAttribute("data-slot"),
    );
    await page.keyboard.press(String(slot + 1));
    await page.keyboard.press("f");
  } else {
    await page.locator('#hand button[data-item-id="' + item.itemId + '"]').click();
    await page.locator("#put-item").click();
  }
  await expect(page.locator('#hand button[data-item-id="' + item.itemId + '"]')).toHaveCount(0);
}
test("真实3D取物投送成对，计分、微光与时间均由服务器确认", async ({ page }, info) => {
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await start(page, "pair");
  const items = await ground(page),
    a = items
      .filter((i) => i.z > 0)
      .sort((a, b) => Math.hypot(a.x - 660, a.y - 320) - Math.hypot(b.x - 660, b.y - 320))[0],
    b = items.find((i) => i.itemId !== a.itemId && i.z > 0 && i.kind === a.kind);
  const startMS = Number(await page.locator("#timer").getAttribute("data-start-ms")),
    endMS = Number(await page.locator("#timer").getAttribute("data-end-ms"));
  expect(endMS - startMS).toBe(180000);
  await pick(page, a, true);
  await pick(page, b);
  await put(page, a, true);
  await put(page, b, true);
  await expect(page.locator("#score")).toHaveText("100");
  await expect(page.locator("#pairs")).toHaveText("1");
  await expect(page.locator("#world")).toHaveAttribute("data-feedback-pairs", "1");
  await expect(page.locator("#world")).toHaveAttribute("data-ground-items", "14");
  expect(Number(await page.locator("#timer").getAttribute("data-end-ms"))).toBe(endMS);
  await expect(page.locator("#pair-feedback")).toContainText("配对 +100");
  await page.screenshot({ path: info.outputPath("pairing-authoritative.png"), fullPage: false });
  expect(errors).toEqual([]);
});
test("上下遮挡由真实取走开放，手持三格满后不能继续取物", async ({ page }) => {
  await start(page, "layers");
  const items = await ground(page),
    top = items[0],
    base = items[1];
  await expect(
    page.locator('#board-items button[data-item-id="' + base.itemId + '"]'),
  ).toHaveAttribute("data-blocked", "true");
  await pick(page, top);
  await expect(
    page.locator('#board-items button[data-item-id="' + base.itemId + '"]'),
  ).toHaveAttribute("data-blocked", "false");
  await pick(page, base);
  await pick(page, items[2]);
  await expect(page.locator("#hand button[data-item-id]:not([data-item-id=''])")).toHaveCount(3);
  await expect(page.locator("#pick-item")).toBeDisabled();
  await expect(page.locator("#score")).toHaveText("0");
});
test("台面可归还原位，物品不丢失且归还不加分", async ({ page }) => {
  await start(page, "return");
  const item = (await ground(page))[0];
  await pick(page, item);
  await put(page, item);
  await expect(page.locator("#world")).toHaveAttribute("data-tray-items", "1");
  await page.locator('#tray button[data-item-id="' + item.itemId + '"]').click();
  await expect(page.locator('#board-items button[data-item-id="' + item.itemId + '"]')).toHaveCount(
    1,
  );
  await expect(page.locator("#world")).toHaveAttribute("data-tray-items", "0");
  await expect(page.locator("#score")).toHaveText("0");
  await expect(page.locator("#multiplier")).toHaveText("×1.0");
});
test("手持物可选择放回原位再拾取，不靠重复搬运刷分", async ({ page }) => {
  await start(page, "drop");
  const item = (await ground(page))[0];
  await pick(page, item);
  await page.locator("#return-item").click();
  await expect(page.locator('#hand button[data-item-id="' + item.itemId + '"]')).toHaveCount(0);
  await pick(page, item);
  await expect(page.locator("#score")).toHaveText("0");
  await expect(page.locator("#pairs")).toHaveText("0");
});
test("同日单人同盘但互不抢物，两个独立浏览器可占相同位置", async ({ browser }) => {
  const ca = await browser.newContext(),
    cb = await browser.newContext();
  try {
    const a = await ca.newPage(),
      b = await cb.newPage();
    await start(a, "solo-a");
    await start(b, "solo-b");
    const items = await ground(a);
    expect(await ground(b)).toEqual(items);
    expect(await a.locator("#challenge").getAttribute("data-round-id")).not.toBe(
      await b.locator("#challenge").getAttribute("data-round-id"),
    );
    const nearest = items
      .filter((i) => i.z > 0)
      .sort((a, b) => Math.hypot(a.x - 660, a.y - 320) - Math.hypot(b.x - 660, b.y - 320))[0];
    await pick(a, nearest);
    await pick(b, nearest);
    await expect(a.locator("#players li")).toHaveCount(1);
    await expect(b.locator("#players li")).toHaveCount(1);
    expect(
      Math.hypot(
        (await coordinate(a, "x")) - (await coordinate(b, "x")),
        (await coordinate(a, "y")) - (await coordinate(b, "y")),
      ),
    ).toBeLessThan(30);
    await expect(a.locator("#world")).toHaveAttribute("data-held-items", "1");
    await expect(b.locator("#world")).toHaveAttribute("data-held-items", "1");
  } finally {
    await Promise.all([ca.close(), cb.close()]);
  }
});
test("旧对话收藏与展台不再是玩法入口，保留真实背景实体", async ({ page }) => {
  await join(page, unique("retired"));
  await expect(page.locator("#npc-talk,#field-book,#camp-panel,#npc-dialog")).toHaveCount(0);
  await expect(page.locator("#world")).toHaveAttribute("data-npc-count", "2");
  await expect(page.locator("#start-solo")).toBeVisible();
  await expect(page.locator("#world")).toHaveAttribute("data-renderer", "webgl2");
});
