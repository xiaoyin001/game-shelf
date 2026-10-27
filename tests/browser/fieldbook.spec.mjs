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
// 以真实权威坐标闭环靠近，松键后等零输入生效，不把180步可见距离当成34步调查距离。
async function approachClue(page) {
  await page.locator("#world").focus();
  await page.waitForTimeout(150);
  for (let i = 0; i < 20; i++) {
    const position = await page.evaluate(() => {
      const me = document.getElementById("coordinates").dataset;
      const target = document.getElementById("trail-progress").dataset;
      return {
        dx: Number(target.x) - Number(me.x),
        dy: Number(target.y) - Number(me.y),
        dz: Number(target.z) - Number(me.z),
        grounded: me.grounded === "true",
      };
    });
    if (
      position.grounded &&
      Math.hypot(position.dx, position.dy, position.dz) <= 24 &&
      Math.abs(position.dz) <= 10
    ) {
      await expect(page.getByRole("button", { name: "调查线索 · F" })).toBeVisible();
      return;
    }
    const key =
      Math.abs(position.dx) > Math.abs(position.dy)
        ? position.dx > 0
          ? "d"
          : "a"
        : position.dy > 0
          ? "s"
          : "w";
    await page.keyboard.down(key);
    try {
      await page.waitForTimeout(110);
    } finally {
      await page.keyboard.up(key);
    }
    await page.waitForTimeout(140);
  }
  throw new Error("未能通过真实移动走近并站稳在线索旁");
}
async function rejoin(page) {
  await page.getByRole("button", { name: "离开世界" }).click();
  await expect(page.locator("#status")).toHaveText("连接已断开");
  await expect(page.locator("#field-book")).toBeHidden();
  await page.getByLabel("密码", { exact: true }).fill(password);
  await page.getByRole("button", { name: /登录并进入原野/ }).click();
  await expect(page.locator("#status")).toContainText("已连接");
}

async function beginBell(page) {
  await expect(page.locator("#collection-count")).toHaveText("0 / 3");
  await walk(page, "s", "y", 440);
  await page.keyboard.press("e");
  await page.getByRole("button", { name: "寻找原野旧物 · 领取 / 回顾线索" }).click();
  await expect(page.locator("#trail-progress")).toHaveAttribute("data-step", "1");
  await page.keyboard.press("Escape");
  await walk(page, "d", "x", 600);
  await walk(page, "s", "y", 500);
  await expect(page.locator("#world")).toHaveAttribute("data-clue-visible", "true");
  await approachClue(page);
  await page.keyboard.press("f");
  await expect(page.locator("#trail-progress")).toHaveAttribute("data-step", "2");
}
async function findBell(page) {
  await walk(page, "d", "x", 610);
  await approachClue(page);
  await page.getByRole("button", { name: "调查线索 · F" }).click();
  await expect(page.locator("#collection-count")).toHaveText("1 / 3");
  await expect(page.locator('#collections details[data-id="bell"]')).toHaveAttribute(
    "data-found",
    "true",
  );
}

test("两步寻宝中途退出可续接，旧物与故事在密码重登后仍保留", async ({ page }, info) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await join(page, unique("book"));
  await beginBell(page);
  await rejoin(page);
  await expect(page.locator("#trail-progress")).toHaveAttribute("data-step", "2");
  await expect(page.locator("#trail-clue")).toContainText("圆石");
  await walk(page, "s", "y", 450);
  await findBell(page);
  const bell = page.locator('#collections details[data-id="bell"]');
  await bell.locator("summary").click();
  await expect(bell.locator("p")).toContainText("第一位守灯人");
  await page.screenshot({ path: info.outputPath("field-book-found.png"), fullPage: false });
  await rejoin(page);
  await expect(page.locator("#collection-count")).toHaveText("1 / 3");
  await expect(page.locator("#score")).toHaveText("0");
  await expect(page.locator("#world")).toHaveAttribute("data-clue-id", "");
  expect(errors).toEqual([]);
});

test("真实寻宝解锁展台，布置收起可持久保存并供另一旅人只读参观", async ({
  page,
  browser,
}, info) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const ownerName = unique("camp");
  await join(page, ownerName);
  await beginBell(page);
  await walk(page, "w", "y", 460, false);
  await findBell(page);
  await walk(page, "s", "y", 510);
  await walk(page, "d", "x", 700);
  await page.getByRole("tab", { name: "微光展台", exact: true }).click();
  await expect(page.locator("#camp-slot-left option[value='bell-lantern']")).toBeEnabled();
  await expect(page.locator("#camp-slot-left option[value='lake-vase']")).toBeDisabled();
  await page.getByLabel("中台摆件").selectOption("bell-lantern");
  await page.getByRole("button", { name: "布置中台" }).click();
  await expect(page.locator("#world")).toHaveAttribute("data-camp-count", "1");
  await page.getByLabel("中台摆件").selectOption("");
  await page.getByRole("button", { name: "布置中台" }).click();
  await expect(page.locator("#world")).toHaveAttribute("data-camp-count", "0");
  await page.getByLabel("左台摆件").selectOption("bell-lantern");
  await page.getByRole("button", { name: "布置左台" }).click();
  await expect(page.locator("#world")).toHaveAttribute("data-camp-kinds", "bell-lantern");
  const ownerID = await page.locator("#camp-summary").getAttribute("data-owner");
  await page.screenshot({ path: info.outputPath("camp-display.png"), fullPage: false });
  const peerContext = await browser.newContext();
  try {
    const peer = await peerContext.newPage();
    peer.on("pageerror", (error) => errors.push(error.message));
    await join(peer, unique("campwatch"));
    await peer.getByRole("tab", { name: "微光展台", exact: true }).click();
    await peer.getByLabel("查看谁的展台").selectOption({ label: ownerName });
    await expect(peer.locator("#camp-summary")).toHaveAttribute("data-owner", ownerID);
    await expect(peer.locator("#world")).toHaveAttribute("data-camp-owner", ownerID);
    await expect(peer.locator("#world")).toHaveAttribute("data-camp-kinds", "bell-lantern");
    await expect(peer.locator("#camp-controls")).toBeHidden();
    await expect(peer.locator("#collection-count")).toHaveText("0 / 3");
  } finally {
    await peerContext.close();
  }
  await rejoin(page);
  await expect(page.locator("#collection-count")).toHaveText("1 / 3");
  await expect(page.locator("#score")).toHaveText("0");
  await expect(page.locator("#world")).toHaveAttribute("data-clue-id", "");
  await expect(page.locator("#world")).toHaveAttribute("data-camp-kinds", "bell-lantern");
  await page.getByRole("tab", { name: "微光展台", exact: true }).click();
  await expect(page.getByLabel("左台摆件")).toHaveValue("bell-lantern");
  expect(errors).toEqual([]);
});
