// 文件说明：通过真实按键和双浏览器验证跳跃模型、互撞和静态阻挡，不直改世界坐标。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";

const coordinate = async (page, axis) =>
  Number(await page.locator("#coordinates").getAttribute("data-" + axis));

test("空格让真实角色跳起，另一旅人看到高度，按住不空中连跳", async ({ browser }, info) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("jump");
    await join(pa, name);
    await join(pb, unique("watch"));
    await expect(pb.locator("#players")).toContainText(name);
    // 登录成功不是 GPU 首帧完成；真实模型已绘制后才触发短暂的跳跃。
    await Promise.all([
      expect(pa.locator("#world")).toHaveAttribute("data-self-height", "0.00"),
      expect(pb.locator("#world")).toHaveAttribute("data-self-height", "0.00"),
    ]);
    await pb.evaluate((name) => {
      // 只观察已展示的权威数据，防止跨进程轮询漏掉不足一秒的高度变化。
      window.jumpEvidence = { peak: 0 };
      const record = () => {
        const row = [...document.querySelectorAll("#players li")].find((row) =>
          row.textContent.includes(name),
        );
        window.jumpEvidence.peak = Math.max(window.jumpEvidence.peak, Number(row?.dataset.z || 0));
      };
      new MutationObserver(record).observe(document.getElementById("players"), {
        childList: true,
        subtree: true,
        attributes: true,
      });
      record();
    }, name);
    // 页面原有连接已建立，用可观察的键盘 repeat 分支验证，不安装改状态的接口。
    await pa.keyboard.down("Space");
    await pa.keyboard.down("Space");
    await Promise.all([
      expect.poll(() => coordinate(pa, "z"), { intervals: [25] }).toBeGreaterThan(15),
      expect.poll(() => pb.evaluate(() => window.jumpEvidence.peak)).toBeGreaterThan(15),
      expect
        .poll(async () => Number(await pa.locator("#world").getAttribute("data-self-height")))
        .toBeGreaterThan(10),
    ]);
    await pa.screenshot({ path: info.outputPath("authoritative-jump.png"), fullPage: true });
    await expect(pa.locator("#coordinates")).toHaveAttribute("data-grounded", "true");
    await expect.poll(() => coordinate(pa, "z")).toBe(0);
    // 同一按住状态再次产生 repeat，不能落地后偷偷再起跳。
    await pa.keyboard.down("Space");
    await pa.waitForTimeout(250);
    expect(await coordinate(pa, "z")).toBe(0);
    await pa.keyboard.up("Space");
    await pa.keyboard.press("Enter");
    await pa.keyboard.press("Space");
    expect(await coordinate(pa, "z")).toBe(0);
    await expect(pa.locator("#chat-input")).toHaveValue(" ");
  } finally {
    await a.close();
    await b.close();
  }
});

test("场景不可用或版本不匹配时拒绝假进入", async ({ page }) => {
  await page.route("**/scene.json", (route) => route.fulfill({ status: 503, body: "unavailable" }));
  await page.goto("/");
  await expect(page.locator("#status")).toHaveText("3D 不可用");
  await expect(page.locator("#register-button")).toBeDisabled();
  await page.unroute("**/scene.json");
  await page.route("**/scene.json", async (route) => {
    const response = await route.fetch();
    const layout = await response.json();
    layout.version = "incompatible-test-scene";
    await route.fulfill({ response, json: layout });
  });
  await page.goto("/");
  await expect(page.locator("#renderer-status")).toHaveText("3D · WebGL2");
  await page.getByLabel("账号", { exact: true }).fill(unique("scene"));
  await page.getByLabel("密码", { exact: true }).fill("123456");
  await page.getByRole("button", { name: "注册并进入原野" }).click();
  await expect(page.locator("#status")).toHaveText("3D 不可用");
  await expect(page.locator("#notice")).toContainText("场景与服务器版本不一致");
  await expect(page.locator("#join-button")).toBeDisabled();
});

test("地面被晶簇阻挡，绕到空地后玩家身体也不能互穿", async ({ browser }, info) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage();
    await join(pa, unique("solid-a"));
    await join(pb, unique("solid-b"));
    const start = await coordinate(pa, "x");
    await pa.keyboard.down("d");
    await pa.waitForTimeout(750);
    await pa.keyboard.up("d");
    const stopped = await coordinate(pa, "x");
    expect(stopped).toBeGreaterThan(start);
    expect(stopped).toBeLessThanOrEqual(578.01);
    for (const page of [pa, pb]) {
      await page.keyboard.down("w");
      await expect.poll(() => coordinate(page, "y"), { intervals: [50] }).toBeLessThan(320);
      await page.keyboard.up("w");
    }
    const bx = await coordinate(pb, "x");
    const distance = async () =>
      Math.hypot(
        (await coordinate(pa, "x")) - bx,
        (await coordinate(pa, "y")) - (await coordinate(pb, "y")),
      );
    await pa.keyboard.down("d");
    // 软件渲染时按住一秒不保证发出足够输入；先等真实接触，再持续施压验证阻挡。
    await expect.poll(distance, { intervals: [50] }).toBeLessThan(43);
    await pa.waitForTimeout(300);
    await pa.keyboard.up("d");
    await expect.poll(() => coordinate(pa, "x")).toBeGreaterThan(stopped + 25);
    expect(await coordinate(pa, "x")).toBeLessThan(bx);
    const gap = await distance();
    expect(gap).toBeGreaterThanOrEqual(39.99);
    expect(gap).toBeLessThan(43);
    await pa.screenshot({ path: info.outputPath("player-collision.png"), fullPage: true });
  } finally {
    await a.close();
    await b.close();
  }
});
