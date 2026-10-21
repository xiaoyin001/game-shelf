// 文件说明：发送真实键盘方向，依据服务器权威快照显示世界。
"use strict";
const $ = (id) => document.getElementById(id),
  canvas = $("world"),
  ctx = canvas.getContext("2d");
const keys = new Set();
let socket = null,
  state = null,
  selfID = "",
  connected = false,
  joining = false,
  joinTimer = null,
  chatID = 0;
function notice(text) {
  $("notice").textContent = text;
}
function status(text) {
  $("status").textContent = text;
}
function send(type, data = {}) {
  if (socket?.readyState === WebSocket.OPEN)
    socket.send(JSON.stringify({ version: 1, type, ...data }));
}
function disconnected(text) {
  connected = false;
  keys.clear();
  joining = false;
  clearTimeout(joinTimer);
  $("entry-panel").hidden = false;
  $("leave-button").hidden = true;
  $("chat-input").disabled = true;
  $("chat-send").disabled = true;
  $("join-button").disabled = false;
  $("join-button").textContent = "重新进入原野 ↗";
  status("连接已断开");
  notice(text);
}
$("login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (joining) return;
  const account = $("account").value.trim();
  if (!account) return;
  joining = true;
  $("join-button").disabled = true;
  status("正在连接…");
  try {
    const response = await fetch("/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ account, channel: "browser" }),
      signal: AbortSignal.timeout(5000),
    });
    const login = await response.json();
    if (!response.ok || !login.gate_websocket_url)
      throw new Error(login.error || "websocket_unavailable");
    socket?.close();
    const current = new WebSocket(login.gate_websocket_url);
    socket = current;
    joinTimer = setTimeout(() => {
      if (socket === current && !connected) {
        current.close();
        disconnected("连接超时，请检查服务就绪状态。");
      }
    }, 6000);
    current.addEventListener("open", () => {
      if (socket === current) send("auth", { token: login.token });
    });
    current.addEventListener("message", (message) => {
      if (socket !== current) return;
      let event;
      try {
        event = JSON.parse(message.data);
      } catch {
        current.close();
        return;
      }
      if (event.version !== 1) {
        current.close();
        return;
      }
      if (event.type === "error") {
        notice("请求未完成：" + event.error);
        return;
      }
      if (event.type === "ready") {
        selfID = event.user_id;
        connected = true;
        joining = false;
        clearTimeout(joinTimer);
        $("entry-panel").hidden = true;
        $("leave-button").hidden = false;
        $("chat-input").disabled = false;
        $("chat-send").disabled = false;
        chatID = 0;
        $("join-button").disabled = false;
        $("account").blur();
        status("已连接 · 原野 01");
        notice("已进入共享原野。使用 WASD 或方向键移动，其他旅人会看到你。");
      }
      if (event.state) {
        state = event.state;
        updateUI();
        updateChat();
        render();
      }
    });
    current.addEventListener("close", () => {
      if (socket === current) disconnected("连接已断开；同名进入会顶替旧连接，可以重新进入。");
    });
    current.addEventListener("error", () => {
      if (socket === current) notice("暂时无法连接网关。");
    });
  } catch (error) {
    disconnected("进入失败：" + error.message);
  }
});
$("leave-button").addEventListener("click", () => socket?.close());
$("chat-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const text = $("chat-input").value.trim();
  if (!connected || !text) return;
  send("chat", { text });
  $("chat-input").value = "";
  $("chat-input").blur();
});
function updateChat() {
  const chats = state.chats || [],
    latest = chats.at(-1)?.id || 0;
  if (latest === chatID) return;
  chatID = latest;
  $("messages").replaceChildren(
    ...chats.map((chat) => {
      const li = document.createElement("li"),
        name = document.createElement("strong"),
        text = document.createElement("span");
      name.textContent = chat.name;
      text.textContent = chat.text;
      li.append(name, text);
      return li;
    }),
  );
  $("messages").scrollTop = $("messages").scrollHeight;
}
const movementKeys = new Set([
  "w",
  "a",
  "s",
  "d",
  "arrowup",
  "arrowdown",
  "arrowleft",
  "arrowright",
]);
document.addEventListener("keydown", (event) => {
  if (!connected || /INPUT|TEXTAREA/.test(event.target.tagName)) return;
  const key = event.key.toLowerCase();
  if (key === "enter") {
    event.preventDefault();
    keys.clear();
    $("chat-input").focus();
  }
  if (movementKeys.has(key)) {
    event.preventDefault();
    keys.add(key);
  }
});
document.addEventListener("keyup", (event) => keys.delete(event.key.toLowerCase()));
window.addEventListener("blur", () => keys.clear());
document.addEventListener("focusin", () => keys.clear());
document.addEventListener("visibilitychange", () => keys.clear());
setInterval(() => {
  if (!connected) return;
  const dx =
    Number(keys.has("d") || keys.has("arrowright")) -
    Number(keys.has("a") || keys.has("arrowleft"));
  const dy =
    Number(keys.has("s") || keys.has("arrowdown")) - Number(keys.has("w") || keys.has("arrowup"));
  send("input", { dx, dy });
}, 100);
function updateUI() {
  const players = state.players || [],
    me = players.find((p) => p.id === selfID);
  $("online-count").textContent = players.length + " 人在线";
  $("player-count").textContent = players.length;
  $("players").replaceChildren(
    ...players.map((player) => {
      const li = document.createElement("li");
      li.textContent = player.name + (player.id === selfID ? " · 你" : "");
      li.dataset.x = player.x;
      li.dataset.y = player.y;
      return li;
    }),
  );
  if (me) {
    $("player-name").textContent = me.name;
    $("coordinates").textContent = "坐标 " + Math.round(me.x) + " / " + Math.round(me.y);
    $("coordinates").dataset.x = me.x;
    $("coordinates").dataset.y = me.y;
  }
}
function render() {
  ctx.fillStyle = "#244b3c";
  ctx.fillRect(0, 0, 1200, 800);
  ctx.strokeStyle = "#b2efc80b";
  for (let x = 0; x < 1200; x += 60) {
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, 800);
    ctx.stroke();
  }
  for (const player of state?.players || []) {
    ctx.fillStyle = player.id === selfID ? "#b2efc8" : "#a4b8e5";
    ctx.beginPath();
    ctx.arc(player.x, player.y, 13, 0, Math.PI * 2);
    ctx.fill();
    ctx.font = "14px sans-serif";
    ctx.textAlign = "center";
    ctx.fillText(player.name, player.x, player.y - 24);
  }
}
render();
