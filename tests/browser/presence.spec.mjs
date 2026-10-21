// 文件说明：当前阶段仅验收真实双浏览器在线、离开重进与同名顶替。
import { test, expect } from "@playwright/test";
export const unique = (label) => "test-" + label + "-" + Date.now().toString(36);
export async function join(page, name) {
  await page.goto("/");
  await page.getByLabel("旅人名字").fill(name);
  await page.getByRole("button", { name: /进入原野/ }).click();
  await expect(page.locator("#status")).toContainText("已连接");
}
test("双浏览器同世界，退出移除并可重新进入", async ({ browser }) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  const pa = await a.newPage(),
    pb = await b.newPage(),
    errors = [];
  for (const p of [pa, pb]) p.on("pageerror", (e) => errors.push(e.message));
  try {
    const na = unique("a"),
      nb = unique("b");
    await join(pa, na);
    await join(pb, nb);
    await expect(pa.locator("#players")).toContainText(nb);
    await expect(pb.locator("#players")).toContainText(na);
    await expect(pa.locator("#online-count")).toHaveText("2 人在线");
    await pb.getByRole("button", { name: "离开世界" }).click();
    await expect(pa.locator("#online-count")).toHaveText("1 人在线");
    await pb.getByRole("button", { name: /进入原野/ }).click();
    await expect(pb.locator("#status")).toContainText("已连接");
    await expect(pa.locator("#online-count")).toHaveText("2 人在线");
    expect(errors).toEqual([]);
  } finally {
    await a.close();
    await b.close();
  }
});
test("同名新会话顶替旧连接", async ({ browser }) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("same");
    await join(pa, name);
    await join(pb, name);
    await expect(pa.locator("#status")).toHaveText("连接已断开");
    await expect(pb.locator("#online-count")).toHaveText("1 人在线");
    await expect(pb.locator("#players")).toContainText(name);
  } finally {
    await a.close();
    await b.close();
  }
});
