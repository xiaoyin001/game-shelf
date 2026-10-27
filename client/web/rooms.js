// 文件说明：呈现本房的准备、短断线与队友手持，不暴露其他房间或自行判定资格。
import { itemNames } from "./items.js";
const $ = (id) => document.getElementById(id);
const text = (id, value) => {
  const e = $(id),
    v = String(value);
  if (e.textContent !== v) e.textContent = v;
};
const prop = (id, key, value) => {
  if ($(id)[key] !== value) $(id)[key] = value;
};
export class RoomPanel {
  constructor(send, unlockAudio) {
    this.send = send;
    this.unlockAudio = unlockAudio;
    this.stamp = "";
    $("create-room").addEventListener("click", () => send("create_room"));
    $("join-room-form").addEventListener("submit", (e) => {
      e.preventDefault();
      send("join_room", { room_code: $("room-code-input").value.trim().toUpperCase() });
    });
    $("ready-room").addEventListener("click", () => {
      unlockAudio();
      send("set_ready", {
        room_id: this.room.id,
        ready: !this.room.members.find((m) => m.id === this.selfID)?.ready,
      });
    });
    $("start-duo").addEventListener("click", () => {
      unlockAudio();
      send("start_duo", { room_id: this.room.id });
    });
  }
  update(room, round, id) {
    this.room = room;
    this.selfID = id;
    const duo = room?.mode === "duo",
      waiting = room?.phase === "waiting";
    prop("mode-entry", "hidden", !id || !!room);
    prop("room-panel", "hidden", !duo);
    if (!duo) return;
    const me = room.members.find((m) => m.id === id),
      host = room.host_id === id;
    text("room-code", room.code);
    text("room-heading", (host ? "房主 · " : "队友 · ") + room.code);
    const phaseKey = room.id + ":" + room.phase;
    if (this.phaseKey !== phaseKey) {
      this.phaseKey = phaseKey;
      prop("room-lobby", "open", waiting);
    }
    prop("room-help", "hidden", !waiting);
    prop("ready-room", "hidden", !waiting);
    prop("start-duo", "hidden", !waiting || !host);
    prop(
      "start-duo",
      "disabled",
      room.members.length !== 2 || room.members.some((m) => !m.ready || !m.online || m.quit),
    );
    text("ready-room", me?.ready ? "取消准备" : "准备");
    const pressed = String(!!me?.ready);
    if ($("ready-room").getAttribute("aria-pressed") !== pressed)
      $("ready-room").setAttribute("aria-pressed", pressed);
    text(
      "room-state",
      room.eligibility === "practice"
        ? "练习局 · 成员已退出或重连超期，不提交双人榜"
        : room.eligibility === "reconnecting"
          ? "原成员离线 · 20秒内重新登录可恢复，计时不停"
          : waiting
            ? "双方准备后，由房主开始"
            : "共同挑战 · 共享分数、台面与连击",
    );
    const others = room.members.filter((m) => m.id !== id);
    const stamp = JSON.stringify([
      room.members.map((m) => ({ ...m, reconnect_ms: Math.ceil(m.reconnect_ms / 1000) })),
      round?.members,
      round?.items?.filter((i) => i.place === "hand" && i.owner_id !== id),
    ]);
    if (stamp === this.stamp) return;
    this.stamp = stamp;
    $("room-members").replaceChildren(
      ...room.members.map((m) => {
        const li = document.createElement("li");
        li.textContent =
          m.name +
          (m.id === room.host_id ? " · 房主" : "") +
          (m.id === id ? " · 你" : "") +
          " · " +
          (m.quit
            ? "已退出"
            : !m.online
              ? "离线 " + Math.ceil(m.reconnect_ms / 1000) + " 秒"
              : m.ready
                ? "已准备"
                : "在线");
        Object.assign(li.dataset, {
          memberId: m.id,
          online: m.online,
          ready: m.ready,
          quit: m.quit,
        });
        return li;
      }),
    );
    $("teammate-hand").replaceChildren(
      ...others.map((m) => {
        const group = document.createElement("div"),
          title = document.createElement("p"),
          slots = document.createElement("div");
        const pairs = round?.members?.find((v) => v.id === m.id)?.pairs || 0;
        title.textContent = m.name + " · 完成配对 " + pairs + " 次" + (!m.online ? " · 离线" : "");
        slots.className = "slots hand";
        for (let slot = 0; slot < 3; slot++) {
          const item = round?.items?.find(
              (i) => i.place === "hand" && i.owner_id === m.id && i.slot === slot,
            ),
            s = document.createElement("span");
          s.textContent = item ? itemNames[item.kind]?.join(" ") : "空";
          s.dataset.kind = item?.kind || "";
          s.dataset.itemId = item?.id || "";
          slots.append(s);
        }
        group.append(title, slots);
        return group;
      }),
    );
  }
}
