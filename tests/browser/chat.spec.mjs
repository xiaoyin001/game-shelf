// 文件说明：通过实际聊天表单验证跨浏览器广播，HTML 作为纯文本显示。
import { test, expect } from "@playwright/test";
import { join, unique } from "./helpers.mjs";
test("世界聊天广播且不解释 HTML", async ({ browser }) => {
  const a = await browser.newContext(),
    b = await browser.newContext();
  const pa = await a.newPage(),
    pb = await b.newPage(),
    errors = [],
    dialogs = [];
  for (const p of [pa, pb]) {
    p.on("pageerror", (e) => errors.push(e.message));
    p.on("dialog", (d) => {
      dialogs.push(d.message());
      d.dismiss();
    });
  }
  try {
    await join(pa, unique("chat"));
    await join(pb, unique("listener"));
    const text = "<img src=x onerror=alert(1)> 你好，原野";
    await pa.keyboard.press("Enter");
    await expect(pa.getByLabel("聊天消息")).toBeFocused();
    await pa.getByLabel("聊天消息").fill(text);
    await pa.getByRole("button", { name: "发送聊天" }).click();
    await expect(pb.locator("#messages")).toContainText(text);
    await expect(pb.locator("#messages img")).toHaveCount(0);
    expect(errors).toEqual([]);
    expect(dialogs).toEqual([]);
  } finally {
    await Promise.all([a.close(), b.close()]);
  }
});
