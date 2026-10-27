// 文件说明：用真实注册和公开WebSocket准备已存在对局；不伪造坐标、时钟或成绩，不改生产客户端状态。
import { expect } from "@playwright/test";
import { password } from "./helpers.mjs";
export async function protocolPeer(page, account) {
  await page.goto("/scene.json");
  const response = await page.request.post("/register", {
    data: { account, password, channel: "browser" },
  });
  expect(response.ok()).toBe(true);
  const login = await response.json();
  await page.evaluate(
    (login) =>
      new Promise((resolve, reject) => {
        const socket = new WebSocket(login.gate_websocket_url);
        const peer = {
          last: null,
          ready: false,
          errors: [],
          send: (type, fields = {}) => socket.send(JSON.stringify({ version: 1, type, ...fields })),
        };
        window.protocolPeer = peer;
        socket.onopen = () => peer.send("auth", { token: login.token });
        socket.onerror = () => reject(new Error("protocol fixture socket error"));
        socket.onmessage = (msg) => {
          const e = JSON.parse(msg.data);
          if (e.error) {
            peer.errors.push(e.error);
            reject(new Error(e.error));
          }
          if (e.state) peer.last = e;
          if (e.type === "ready") {
            peer.ready = true;
            resolve();
          }
        };
      }),
    login,
  );
}
export async function command(page, type, fields = {}) {
  await page.evaluate(({ type, fields }) => window.protocolPeer.send(type, fields), {
    type,
    fields,
  });
}
export async function snapshot(page) {
  return page.evaluate(() => window.protocolPeer.last);
}
export async function existingCoop(a, b, an, bn) {
  await Promise.all([protocolPeer(a, an), protocolPeer(b, bn)]);
  await command(a, "create_room");
  await expect.poll(async () => (await snapshot(a))?.room?.phase).toBe("waiting");
  const room = (await snapshot(a)).room;
  await command(b, "join_room", { room_code: room.code });
  await expect.poll(async () => (await snapshot(a))?.room?.members?.length).toBe(2);
  await command(a, "set_ready", { room_id: room.id, ready: true });
  await command(b, "set_ready", { room_id: room.id, ready: true });
  await expect.poll(async () => (await snapshot(a)).room.members.every((m) => m.ready)).toBe(true);
  await command(a, "start_duo", { room_id: room.id });
  await expect.poll(async () => (await snapshot(b))?.challenge?.phase).toBe("running");
  const r = (await snapshot(b)).challenge;
  const item = r.items.find((i) => i.x === 740 && i.y === 360 && i.z > 0);
  // 只发与按键一致的方向和真实等待；最终位置、容量、物品归属仍由服务器裁定。
  await b.evaluate(async () => {
    for (let i = 0; i < 4; i++) {
      await new Promise((resolve) => setTimeout(resolve, 110));
      window.protocolPeer.send("input", { dx: 1 });
    }
    window.protocolPeer.send("input", { dx: 0, dy: 0 });
  });
  await command(b, "pick", { round_id: r.id, item_id: item.id });
  await expect
    .poll(async () => (await snapshot(b)).challenge.items.find((i) => i.id === item.id)?.place)
    .toBe("hand");
  return { room, round: r, item };
}
