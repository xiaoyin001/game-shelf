// 文件说明：当前仅通过认证连接与权威快照显示共享在线，不预置后续玩法。
"use strict";
const $ = (id) => document.getElementById(id),
  canvas = $("world"),
  ctx = canvas.getContext("2d");
let socket = null,
  state = null,
  selfID = "",
  connected = false,
  joining = false,
  joinTimer = null;
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
  joining = false;
  clearTimeout(joinTimer);
  $("entry-panel").hidden = false;
  $("leave-button").hidden = true;
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
        $("join-button").disabled = false;
        $("account").blur();
        status("已连接 · 原野 01");
        notice("已进入共享原野。当前支持在线观察、退出和重新进入。");
      }
      if (event.state) {
        state = event.state;
        updateUI();
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
setInterval(() => {
  if (connected) send("ping");
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
