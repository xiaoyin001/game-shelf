// 文件说明：用真实注册和公开WebSocket准备已存在对局；不伪造坐标、时钟或成绩，不改生产客户端状态。
import { expect } from "@playwright/test";
import { password } from "./helpers.mjs";
import { route } from "./navigation.mjs";
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
            peer.id = e.user_id;
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
  const item = r.items
    .filter((i) => i.z > 0)
    .sort((a, b) => Math.hypot(a.x - 660, a.y - 320) - Math.hypot(b.x - 660, b.y - 320))[0];
  const layout = await (await b.request.get("/scene.json")).json(),
    id = await b.evaluate(() => window.protocolPeer.id),
    state = (await snapshot(b)).state,
    me = state.players.find((p) => p.id === id);
  for (const target of route(
    layout,
    me,
    item,
    state.players.filter((p) => p.id !== id),
  )) {
    let reached = false;
    for (let i = 0; i < 150; i++) {
      const p = (await snapshot(b)).state.players.find((p) => p.id === id),
        dx = target.x - p.x,
        dy = target.y - p.y,
        length = Math.hypot(dx, dy);
      if (length < 3) {
        reached = true;
        break;
      }
      // 真实方向与真实时间，模拟合法模拟量输入，不注入最终坐标。
      await command(b, "input", { dx: dx / Math.max(18, length), dy: dy / Math.max(18, length) });
      await b.waitForTimeout(110);
    }
    expect(reached).toBe(true);
  }
  await command(b, "input", { dx: 0, dy: 0 });
  await command(b, "pick", { round_id: r.id, item_id: item.id });
  await expect
    .poll(async () => (await snapshot(b)).challenge.items.find((i) => i.id === item.id)?.place)
    .toBe("hand");
  return { room, round: r, item };
}
