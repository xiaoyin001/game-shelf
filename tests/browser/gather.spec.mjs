// 文件说明：双浏览器验证采集分数与冷却共享，不直接操纵游戏状态。
import { test, expect } from "@playwright/test";
import { join, unique, password } from "./helpers.mjs";
test("共享采集只奖励一次，离开重进重置本轮分数", async ({ browser }, info) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("gather");
    await join(pa, name);
    await join(pb, unique("peer"));
    const start = Number(await pa.locator("#coordinates").getAttribute("data-x"));
    await pa.keyboard.down("d");
    await expect
      .poll(async () => Number(await pa.locator("#coordinates").getAttribute("data-x")))
      .toBeGreaterThan(start + 10);
    await pa.keyboard.up("d");
    // 实体晶簇前停止后仍应能采集，不能靠穿进资源内部完成测试。
    expect(Number(await pa.locator("#coordinates").getAttribute("data-x"))).toBeLessThanOrEqual(
      578.01,
    );
    await pa.keyboard.press("e");
    await expect(pa.locator("#score")).toHaveText("1");
    await expect(pb.locator("#players")).toContainText("✦ 1");
    await pb.keyboard.press("e");
    await expect(pb.locator("#notice")).toContainText(/没有可采集|附近没有/);
    await expect(pb.locator("#score")).toHaveText("0");
    await pa.screenshot({ path: info.outputPath("two-player-world.png"), fullPage: true });
    await pa.getByRole("button", { name: "离开世界" }).click();
    await expect(pa.locator("#status")).toHaveText("连接已断开");
    const ownRow = pa.locator("#players li").filter({ hasText: name });
    const peerRow = pb.locator("#players li").filter({ hasText: name });
    await expect(peerRow).toHaveCount(0);
    // 原页表单重进不需要重新加载 3D；整页重载仍由 presence 用例覆盖。
    await pa.getByLabel("账号", { exact: true }).fill(name);
    await pa.getByLabel("密码", { exact: true }).fill(password);
    await pa.getByRole("button", { name: /登录并进入原野/ }).click();
    await expect(pa.locator("#status")).toContainText("已连接");
    await expect(pa.locator("#score")).toHaveText("0");
    await expect(ownRow.locator(".player-score")).toHaveText("✦ 0");
    await expect(peerRow.locator(".player-score")).toHaveText("✦ 0");
  } finally {
    await Promise.all([a.close(), b.close()]);
  }
});
