// 文件说明：复用真实键盘和只读权威道具观察，不写入坐标或伪造局面。
import { expect } from "@playwright/test";
import { route } from "./navigation.mjs";
const layouts = new WeakMap();
export const coordinate = async (page, axis) =>
  Number(await page.locator("#coordinates").getAttribute("data-" + axis));
export async function walk(page, x, y) {
  if (!layouts.has(page)) layouts.set(page, await (await page.request.get("/scene.json")).json());
  const start = { x: await coordinate(page, "x"), y: await coordinate(page, "y") };
  const bodies = await page
    .locator("#players li")
    .evaluateAll((es) =>
      es
        .filter((e) => !e.textContent.endsWith(" · 你"))
        .map((e) => ({ x: Number(e.dataset.x), y: Number(e.dataset.y) })),
    );
  for (const p of route(layouts.get(page), start, { x, y }, bodies)) await waypoint(page, p.x, p.y);
}
async function waypoint(page, x, y) {
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
export async function ground(page) {
  return page.locator("#board-items button").evaluateAll((bs) =>
    bs.map((b) => ({
      ...b.dataset,
      x: Number(b.dataset.x),
      y: Number(b.dataset.y),
      z: Number(b.dataset.z),
    })),
  );
}
export async function pick(page, item, keyboard = false) {
  await walk(page, item.x, item.y);
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
export async function put(page, item, keyboard = false, target = { x: 700, y: 350 }) {
  await walk(page, target.x, target.y);
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
