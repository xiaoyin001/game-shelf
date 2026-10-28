// 文件说明：真实双浏览器通过房间码合作，覆盖准备、公开队友手持、重连和退出练习。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";
import { ground, pick, put, walk } from "./challenge-helpers.mjs";
import { existingCoop, snapshot } from "./room-protocol.mjs";
async function pair(browser) {
  const ca = await browser.newContext(),
    cb = await browser.newContext();
  const a = await ca.newPage(),
    b = await cb.newPage(),
    an = unique("room-a"),
    bn = unique("room-b");
  const errors = [];
  for (const p of [a, b]) p.on("pageerror", (e) => errors.push(e.message));
  try {
    await Promise.all([join(a, an), join(b, bn)]);
    await a.locator("#create-room").click();
    await expect(a.locator("#challenge")).toHaveAttribute("data-phase", "waiting");
    const code = await a.locator("#room-code").textContent();
    await b.getByLabel("房间码", { exact: true }).fill(code);
    await b.getByRole("button", { name: "加入房间", exact: true }).click();
    await expect(a.locator("#room-members li")).toHaveCount(2);
    await expect(b.locator("#room-members li")).toHaveCount(2);
    await expect(a.locator("#start-duo")).toBeDisabled();
    await a.locator("#ready-room").click();
    await expect(a.locator("#start-duo")).toBeDisabled();
    await b.locator("#ready-room").click();
    await expect(a.locator("#start-duo")).toBeEnabled();
    await expect(b.locator("#start-duo")).toBeHidden();
    await a.locator("#start-duo").click();
    for (const p of [a, b])
      await expect(p.locator("#challenge")).toHaveAttribute("data-phase", "running");
    return { a, b, bn, ca, cb, errors };
  } catch (error) {
    await Promise.all([ca.close(), cb.close()]);
    throw error;
  }
}
test("双人房间码与双方准备后合作，队友手持和团队分数一致", async ({ browser }, info) => {
  const { a, b, ca, cb, errors } = await pair(browser);
  try {
    const items = await ground(a),
      first = items
        .filter((i) => i.z > 0)
        .sort((a, b) => Math.hypot(a.x - 660, a.y - 320) - Math.hypot(b.x - 660, b.y - 320))[0],
      mate = items.find((i) => i.z > 0 && i.kind === first.kind && i.itemId !== first.itemId);
    const deadline = await a.locator("#timer").getAttribute("data-end-ms");
    // 两人同时走各自真实路线取物，不让队友无意义等第一人全部返台。
    await Promise.all([pick(a, first), pick(b, mate)]);
    await expect(a.locator('#teammate-hand span[data-item-id="' + mate.itemId + '"]')).toHaveCount(
      1,
    );
    await expect(a.locator("#world")).toHaveAttribute("data-held-items", "2");
    await expect(b.locator("#world")).toHaveAttribute("data-held-items", "2");
    await expect(b.locator('#teammate-hand span[data-item-id="' + first.itemId + '"]')).toHaveCount(
      1,
    );
    await put(a, first, true, { x: 720, y: 350 });
    await expect(b.locator('#tray button[data-item-id="' + first.itemId + '"]')).toHaveCount(1);
    // 向东让开同伴的返台路线，仍需绕过真实静态实体及队友身体。
    await walk(a, 840, 350);
    await expect(a.locator('#teammate-hand span[data-item-id="' + mate.itemId + '"]')).toHaveCount(
      1,
    );
    await expect(a.locator("#world")).toHaveAttribute("data-held-items", "1");
    await put(b, mate, true, { x: 720, y: 350 });
    for (const p of [a, b]) {
      await expect(p.locator("#score")).toHaveText("100");
      await expect(p.locator("#pairs")).toHaveText("1");
      expect(await p.locator("#timer").getAttribute("data-end-ms")).toBe(deadline);
    }
    await expect(a.locator("#teammate-hand")).toContainText("完成配对 1 次");
    const boardHelp = a.locator("details").filter({ has: a.locator("#board-items") });
    if (await boardHelp.evaluate((e) => e.open)) await boardHelp.locator("summary").click();
    await a.locator("#world").scrollIntoViewIfNeeded();
    await a.screenshot({ path: info.outputPath("cooperative-pairing.png"), fullPage: false });
    expect(errors).toEqual([]);
  } finally {
    await Promise.all([ca.close(), cb.close()]);
  }
});
test("双人刷新后原账号自动回房，截止和手持不重置", async ({ browser }) => {
  const ca = await browser.newContext(),
    cb = await browser.newContext();
  try {
    const a = await ca.newPage(),
      b = await cb.newPage(),
      an = unique("resume-a"),
      bn = unique("resume-b"),
      errors = [];
    for (const p of [a, b]) p.on("pageerror", (e) => errors.push(e.message));
    // 准备交互已由相邻两个完整UI用例覆盖；此处通过同一公开协议进入真正存在的局，再验证表单重连。
    const { round, item } = await existingCoop(a, b, an, bn);
    await join(b, bn, { register: false });
    await expect(b.locator("#challenge")).toHaveAttribute("data-round-id", round.id);
    await expect(b.locator("#challenge")).toHaveAttribute("data-eligibility", "eligible");
    await expect(b.locator('#hand button[data-item-id="' + item.id + '"]')).toHaveCount(1);
    await expect(b.locator("#world")).toHaveAttribute("data-held-items", "1");
    expect(await b.locator("#timer").getAttribute("data-end-ms")).toBe(String(round.end_ms));
    await expect.poll(async () => (await snapshot(a)).room.eligibility).toBe("eligible");
    expect((await snapshot(a)).challenge.items.find((i) => i.id === item.id).place).toBe("hand");
    expect(errors).toEqual([]);
  } finally {
    await Promise.all([ca.close(), cb.close()]);
  }
});
test("主动退出双人转练习，剩余成员继续且不能用房间码换人", async ({ browser }) => {
  const { a, b, ca, cb, errors } = await pair(browser);
  try {
    const code = await b.locator("#room-code").textContent(),
      deadline = await a.locator("#timer").getAttribute("data-end-ms");
    await b.setViewportSize({ width: 390, height: 844 });
    expect(await b.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
      true,
    );
    b.once("dialog", (d) => d.accept());
    await b.locator("#quit-round").click();
    await expect(b.locator("#challenge")).toHaveAttribute("data-phase", "lobby");
    await expect(a.locator("#challenge")).toHaveAttribute("data-eligibility", "practice");
    await expect(a.locator("#room-state")).toContainText("不提交双人榜");
    expect(await a.locator("#timer").getAttribute("data-end-ms")).toBe(deadline);
    await b.getByLabel("房间码", { exact: true }).fill(code);
    await b.getByRole("button", { name: "加入房间", exact: true }).click();
    await expect(b.locator("#notice")).toContainText("不能中途加入");
    await expect(a.locator("#challenge")).toHaveAttribute("data-phase", "running");
    expect(errors).toEqual([]);
  } finally {
    await Promise.all([ca.close(), cb.close()]);
  }
});
