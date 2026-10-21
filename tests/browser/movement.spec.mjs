// 文件说明：用真实键盘验证移动同步到另一独立浏览器，不直接改写客户端状态。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";
test("键盘权威移动同步到另一旅人", async ({ browser }) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  try {
    const pa = await a.newPage(),
      pb = await b.newPage(),
      name = unique("move");
    await join(pa, name);
    await join(pb, unique("peer"));
    const start = Number(await pa.locator("#coordinates").getAttribute("data-x"));
    await pa.keyboard.down("d");
    await expect
      .poll(async () => Number(await pa.locator("#coordinates").getAttribute("data-x")))
      .toBeGreaterThan(start + 35);
    await pa.keyboard.up("d");
    await expect
      .poll(async () =>
        Number(await pb.locator("#players li").filter({ hasText: name }).getAttribute("data-x")),
      )
      .toBeGreaterThan(start + 35);
  } finally {
    await a.close();
    await b.close();
  }
});
