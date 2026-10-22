// 文件说明：验证真实 WebGL2、独立双人、镜头操作及相对镜头的权威位移，不提供改状态后门。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";
test("真正 3D、本地资源、镜头相对移动与双人快照", async ({ browser }, info) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      errors = [],
      external = [];
    pa.on("pageerror", (e) => errors.push(e.message));
    pa.on("request", (r) => {
      if (/^https?:/.test(r.url()) && !r.url().startsWith("http://127.0.0.1:11000/"))
        external.push(r.url());
    });
    const na = unique("3d-a"),
      nb = unique("3d-b");
    await join(pa, na);
    await join(pb, nb);
    const canvas = pa.locator("#world");
    await expect(canvas).toHaveAttribute("data-renderer", "webgl2");
    const gl = await canvas.evaluate((el) => {
      const ctx = el.getContext("webgl2");
      return {
        real: ctx instanceof WebGL2RenderingContext,
        depth: ctx.getParameter(ctx.DEPTH_BITS),
        lost: ctx.isContextLost(),
        width: el.width,
        height: el.height,
      };
    });
    expect(gl.real).toBe(true);
    expect(gl.depth).toBeGreaterThanOrEqual(16);
    expect(gl.lost).toBe(false);
    expect(gl.width).toBeGreaterThan(300);
    await expect(pa.locator("#players")).toContainText(nb);
    await expect(canvas).toHaveAttribute("data-render-mode", "idle");
    const idle = await canvas.evaluate(async (element) => {
      const start = performance.now(),
        count = Number(element.dataset.renderCount);
      await new Promise((resolve) => setTimeout(resolve, 500));
      return {
        elapsed: performance.now() - start,
        frames: Number(element.dataset.renderCount) - count,
      };
    });
    expect(idle.frames).toBeGreaterThan(0);
    expect(idle.frames).toBeLessThanOrEqual(Math.ceil((idle.elapsed * 12) / 1000) + 1);
    await canvas.evaluate((element) => {
      // 只读记录短暂的活动模式，不能因跨进程采样延迟漏掉已经绘制的镜头动画。
      window.renderEvidence = { active: false };
      new MutationObserver(() => {
        if (element.dataset.renderMode === "active") window.renderEvidence.active = true;
      }).observe(element, { attributes: true, attributeFilter: ["data-render-mode"] });
    });
    const initial = Number(await canvas.getAttribute("data-camera-distance"));
    const box = await canvas.boundingBox();
    await pa.mouse.move(box.x + box.width * 0.5, box.y + box.height * 0.6);
    await pa.mouse.wheel(0, -200);
    await expect
      .poll(async () => Number(await canvas.getAttribute("data-camera-distance")))
      .toBeLessThan(initial);
    await expect.poll(() => pa.evaluate(() => window.renderEvidence.active)).toBe(true);
    await pa.mouse.down({ button: "right" });
    await pa.mouse.move(box.x + box.width * 0.5 + 160, box.y + box.height * 0.6 - 40, { steps: 8 });
    await pa.mouse.up({ button: "right" });
    await expect
      .poll(async () => Math.abs(Number(await canvas.getAttribute("data-camera-yaw"))))
      .toBeGreaterThan(0.8);
    const x = Number(await pa.locator("#coordinates").getAttribute("data-x")),
      z = Number(await pa.locator("#coordinates").getAttribute("data-y"));
    await pa.keyboard.down("d");
    await expect
      .poll(async () => Number(await pa.locator("#coordinates").getAttribute("data-y")))
      .toBeGreaterThan(z + 30);
    await pa.keyboard.up("d");
    const peer = pb.locator("#players li").filter({ hasText: na });
    await expect
      .poll(async () => Number(await peer.getAttribute("data-y")))
      .toBeGreaterThan(z + 30);
    expect(Number(await pa.locator("#coordinates").getAttribute("data-x"))).toBeGreaterThan(x);
    const intercepted = await pa.evaluate(() => {
      const event = new KeyboardEvent("keydown", {
        key: "r",
        ctrlKey: true,
        bubbles: true,
        cancelable: true,
      });
      document.dispatchEvent(event);
      return event.defaultPrevented;
    });
    expect(intercepted).toBe(false);
    await pa.screenshot({ path: info.outputPath("three-dimensional-orbit.png"), fullPage: true });
    await pa.keyboard.press("r");
    await expect(canvas).toHaveAttribute("data-camera-yaw", "0.0000");
    await pa.screenshot({ path: info.outputPath("three-dimensional-world.png"), fullPage: true });
    expect(errors).toEqual([]);
    expect(external).toEqual([]);
    // 真实丢失 GL 上下文后必须断开会话并保留原因，不能把画面冻结当作仍在游戏。
    const canLose = await canvas.evaluate((el) => {
      const extension = el.getContext("webgl2").getExtension("WEBGL_lose_context");
      if (!extension) return false;
      extension.loseContext();
      return true;
    });
    expect(canLose).toBe(true);
    await expect(pa.locator("#status")).toHaveText("3D 不可用");
    await expect(pa.locator("#notice")).toContainText("上下文已丢失");
    await expect(pa.locator("#join-button")).toBeDisabled();
    await expect(pb.locator("#online-count")).toHaveText("1 人在线");
  } finally {
    await a.close();
    await b.close();
  }
});
test("无 WebGL2 时明确失败，不降级成 2D 也不能假进入", async ({ page }) => {
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (kind, ...args) {
      if (this.id === "world" && String(kind).startsWith("webgl")) return null;
      return original.call(this, kind, ...args);
    };
  });
  await page.goto("/");
  await expect(page.locator("#notice")).toContainText("WebGL2");
  await expect(page.locator("#renderer-status")).toHaveText("3D 不可用");
  await expect(page.locator("#join-button")).toBeDisabled();
  await expect(page.locator("#register-button")).toBeDisabled();
});
