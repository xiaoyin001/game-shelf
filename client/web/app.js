// 文件说明：连接真实服务器，发送输入并以 Canvas 绘制权威世界，不在客户端决定奖励或坐标。
"use strict";
const $ = (id) => document.getElementById(id);
const canvas = $("world"),
  ctx = canvas.getContext("2d");
const keys = new Set();
let socket = null,
  state = null,
  selfID = "",
  connected = false,
  chatID = 0;
let joining = false,
  joinTimer = null;
const errorMessages = {
  invalid_or_expired_token: "登录凭据已过期，请重新进入。",
  invalid_credentials: "账号或密码错误。",
  account_exists: "账号已存在，请使用原密码登录。",
  invalid_account: "账号需要 2–24 个字母、数字、下划线或连字符。",
  invalid_password: "密码需 12–128 个字符，不能全空白或包含控制字符。",
  auth_rate_limited: "登录尝试过多，请一分钟后再试。",
  auth_busy: "账号验证繁忙，请稍后再试。",
  db_unavailable: "账号服务暂时不可用，请稍后再试。",
  no_resource_in_range: "附近没有可采集的晶簇，试试向发光处走近一点。",
  chat_too_fast: "说慢一点，让原野听清楚。",
  invalid_chat: "消息不能为空，最多 140 个字。",
  session_replaced_or_expired: "会话已结束，请重新进入。",
  world_full: "原野暂时已满，请稍后再试。",
  single_world_unavailable: "世界服务尚未就绪，请稍后再试。",
  rate_limited: "请求过于频繁，连接已关闭。",
};
function notice(text) {
  $("notice").textContent = text;
}
function status(text, live) {
  $("status").textContent = text;
  $("status-dot").classList.toggle("live", live);
}
function send(type, data = {}) {
  if (socket && socket.readyState === WebSocket.OPEN)
    socket.send(JSON.stringify({ version: 1, type, ...data }));
}
function disconnected(text) {
  connected = false;
  joining = false;
  keys.clear();
  clearTimeout(joinTimer);
  $("entry-panel").hidden = false;
  $("game-hint").hidden = true;
  $("leave-button").hidden = true;
  $("chat-input").disabled = true;
  $("chat-send").disabled = true;
  $("join-button").disabled = false;
  $("register-button").disabled = false;
  $("join-button").textContent = "登录并进入原野 ↗";
  status("连接已断开", false);
  notice(text);
}
$("login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (joining) return;
  const account = $("account").value.trim();
  const password = $("password").value;
  if (!account || !password) return;
  const registering = event.submitter?.value === "register";
  joining = true;
  $("join-button").disabled = true;
  $("register-button").disabled = true;
  status("正在连接…", false);
  try {
    const response = await fetch(registering ? "/register" : "/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ account, password, channel: "browser" }),
      signal: AbortSignal.timeout(5000),
    });
    const login = await response.json();
    if (!response.ok || !login.gate_websocket_url)
      throw new Error(login.error || "websocket_unavailable");
    $("password").value = "";
    if (socket) socket.close();
    const current = new WebSocket(login.gate_websocket_url);
    socket = current;
    joinTimer = setTimeout(() => {
      if (socket === current && !connected) {
        current.close();
        disconnected("连接超时，请检查服务是否就绪。");
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
        notice("客户端协议版本不兼容。");
        current.close();
        return;
      }
      if (event.type === "error") {
        notice(errorMessages[event.error] || "请求未完成：" + event.error);
        return;
      }
      if (event.type === "ready") {
        selfID = event.user_id;
        connected = true;
        joining = false;
        chatID = 0;
        clearTimeout(joinTimer);
        $("entry-panel").hidden = true;
        $("game-hint").hidden = false;
        $("leave-button").hidden = false;
        $("chat-input").disabled = false;
        $("chat-send").disabled = false;
        $("join-button").disabled = false;
        $("register-button").disabled = false;
        $("account").blur();
        $("password").blur();
        status("已连接 · 原野 01", true);
        notice("你已进入共享原野。靠近晶簇按 E 采集，按 Enter 与旅人聊天。");
      }
      if (event.state) {
        state = event.state;
        updateUI();
      }
    });
    current.addEventListener("close", () => {
      if (socket === current)
        disconnected("连接已断开；若同账号通过验证后在别处进入，旧连接会被顶替。可重新登录。");
    });
    current.addEventListener("error", () => {
      if (socket === current) notice("暂时无法连接网关，请确认服务已经启动。");
    });
  } catch (error) {
    disconnected("进入失败：" + (errorMessages[error.message] || error.message));
  }
});
$("leave-button").addEventListener("click", () => {
  if (socket) socket.close();
});
$("chat-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const text = $("chat-input").value.trim();
  if (!connected || !text) return;
  send("chat", { text });
  $("chat-input").value = "";
  $("chat-input").blur();
});
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
  if (movementKeys.has(key)) {
    event.preventDefault();
    keys.add(key);
  }
  if (key === "e" && !event.repeat) {
    event.preventDefault();
    send("gather");
  }
  if (key === "enter") {
    event.preventDefault();
    keys.clear();
    $("chat-input").focus();
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
      const li = document.createElement("li"),
        name = document.createElement("span"),
        score = document.createElement("span");
      name.textContent = player.name + (player.id === selfID ? " · 你" : "");
      score.textContent = "✦ " + player.score;
      score.className = "player-score";
      li.dataset.x = player.x;
      li.dataset.y = player.y;
      li.title = "坐标 " + Math.round(player.x) + " / " + Math.round(player.y);
      li.append(name, score);
      return li;
    }),
  );
  if (me) {
    const previous = Number($("score").textContent);
    $("score").textContent = me.score;
    $("player-name").textContent = me.name;
    $("coordinates").textContent = "坐标 " + Math.round(me.x) + " / " + Math.round(me.y);
    $("coordinates").dataset.x = me.x;
    $("coordinates").dataset.y = me.y;
    if (me.score > previous) notice("收集到一枚微光晶簇 ✦ 共享资源将在 10 秒后重生。");
  }
  const chats = state.chats || [],
    latest = chats.at(-1)?.id || 0;
  if (latest !== chatID) {
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
}
function path(points, fill, stroke) {
  ctx.beginPath();
  points.forEach(([x, y], i) => (i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)));
  ctx.closePath();
  ctx.fillStyle = fill;
  ctx.fill();
  if (stroke) {
    ctx.strokeStyle = stroke;
    ctx.stroke();
  }
}
function ellipse(x, y, rx, ry, fill) {
  ctx.beginPath();
  ctx.ellipse(x, y, rx, ry, 0, 0, Math.PI * 2);
  ctx.fillStyle = fill;
  ctx.fill();
}
function drawTree(x, y, size) {
  ellipse(x, y + size * 0.6, size * 0.65, size * 0.18, "#091e204f");
  ctx.fillStyle = "#456052";
  ctx.fillRect(x - 3, y, 6, size * 0.65);
  ellipse(x - 13, y - size * 0.1, size * 0.48, size * 0.5, "#1d493b");
  ellipse(x + 13, y - size * 0.22, size * 0.5, size * 0.55, "#285b45");
  ellipse(x, y - size * 0.45, size * 0.5, size * 0.52, "#34694c");
  ellipse(x - 5, y - size * 0.6, size * 0.3, size * 0.28, "#3e7655");
}
const trees = Array.from({ length: 42 }, (_, i) => {
  const edge = i % 4,
    t = ((i * 137) % 1000) / 1000;
  return edge < 2
    ? [edge === 0 ? 50 + (i % 3) * 25 : 1100 + (i % 3) * 22, 70 + t * 660, 42 + (i % 4) * 8]
    : [120 + t * 950, edge === 2 ? 75 + (i % 3) * 10 : 730 + (i % 3) * 10, 38 + (i % 4) * 7];
});
function render(now) {
  const gradient = ctx.createLinearGradient(0, 0, 1200, 800);
  gradient.addColorStop(0, "#193f38");
  gradient.addColorStop(0.5, "#264d3b");
  gradient.addColorStop(1, "#173b35");
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, 1200, 800);
  ellipse(600, 420, 380, 230, "#42644a33");
  ellipse(530, 400, 250, 180, "#7d875512");
  ctx.strokeStyle = "#b5c68708";
  ctx.lineWidth = 1;
  for (let x = 0; x < 1200; x += 60) {
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, 800);
    ctx.stroke();
  }
  for (let y = 0; y < 800; y += 60) {
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(1200, y);
    ctx.stroke();
  }
  ctx.strokeStyle = "#a19b6a19";
  ctx.lineWidth = 67;
  ctx.lineCap = "round";
  ctx.beginPath();
  ctx.moveTo(460, 820);
  ctx.bezierCurveTo(430, 600, 700, 500, 560, 300);
  ctx.bezierCurveTo(470, 190, 620, 60, 660, -30);
  ctx.stroke();
  ellipse(910, 470, 115, 57, "#163e43");
  ellipse(910, 465, 101, 46, "#24575a");
  ctx.strokeStyle = "#7bb6a12b";
  ctx.lineWidth = 2;
  ctx.beginPath();
  ctx.ellipse(910, 465, 83, 31, 0, 0, Math.PI * 2);
  ctx.stroke();
  for (let i = 0; i < 150; i++) {
    const x = (i * 173 + 47) % 1200,
      y = (i * 97 + 29) % 800;
    ctx.fillStyle = i % 4 ? "#74a47024" : "#b8cda24f";
    ctx.fillRect(x, y, 2, (i % 3) + 2);
  }
  for (const [x, y, s] of trees) drawTree(x, y, s);
  ellipse(520, 420, 38, 17, "#132d2a50");
  ctx.strokeStyle = "#7d8b6866";
  ctx.lineWidth = 5;
  ctx.beginPath();
  ctx.ellipse(520, 417, 31, 12, 0, 0, Math.PI * 2);
  ctx.stroke();
  path(
    [
      [503, 412],
      [520, 389],
      [534, 413],
    ],
    "#e6b875",
  );
  path(
    [
      [511, 413],
      [521, 399],
      [525, 414],
    ],
    "#f5dfa0",
  );
  const nodes = state?.nodes || [
    { x: 620, y: 400, ready_at_ms: 0 },
    { x: 300, y: 260, ready_at_ms: 0 },
    { x: 900, y: 260, ready_at_ms: 0 },
    { x: 350, y: 600, ready_at_ms: 0 },
    { x: 850, y: 600, ready_at_ms: 0 },
  ];
  for (const node of nodes) {
    const ready = node.ready_at_ms <= (state?.time_ms || 0),
      pulse = Math.sin(now / 800 + node.x) * 0.13 + 0.6;
    ellipse(node.x, node.y + 12, 23, 8, "#0d2a2b77");
    if (ready) {
      const glow = ctx.createRadialGradient(node.x, node.y, 1, node.x, node.y, 50);
      glow.addColorStop(0, "rgba(155,240,189," + pulse * 0.2 + ")");
      glow.addColorStop(1, "#adf2cb00");
      ctx.fillStyle = glow;
      ctx.fillRect(node.x - 50, node.y - 50, 100, 100);
    }
    path(
      [
        [node.x, node.y - 22],
        [node.x + 13, node.y - 4],
        [node.x, node.y + 13],
        [node.x - 12, node.y - 4],
      ],
      ready ? "#a3e3bb" : "#355b50",
      ready ? "#ddffe1" : "#4a6f5e",
    );
    path(
      [
        [node.x, node.y - 22],
        [node.x + 13, node.y - 4],
        [node.x, node.y + 13],
      ],
      ready ? "#6dac96" : "#284a43",
    );
    if (!ready) {
      ctx.font = "10px sans-serif";
      ctx.textAlign = "center";
      ctx.fillStyle = "#8fac98";
      ctx.fillText(
        Math.max(1, Math.ceil((node.ready_at_ms - state.time_ms) / 1000)) + "s",
        node.x,
        node.y - 32,
      );
    }
  }
  const players = [...(state?.players || [])].sort((a, b) => a.y - b.y);
  for (const player of players) {
    const own = player.id === selfID;
    ellipse(player.x, player.y + 15, 16, 6, "#081e1b88");
    if (own) {
      ctx.strokeStyle = "#adf2cb88";
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.ellipse(player.x, player.y + 15, 22, 9, 0, 0, Math.PI * 2);
      ctx.stroke();
    }
    ctx.fillStyle = own ? "#9bd6bc" : "#a5b8df";
    ctx.beginPath();
    ctx.roundRect(player.x - 10, player.y - 4, 20, 23, 6);
    ctx.fill();
    ellipse(player.x, player.y - 11, 8, 9, "#e5d5b7");
    path(
      [
        [player.x - 13, player.y - 14],
        [player.x, player.y - 27],
        [player.x + 13, player.y - 14],
      ],
      own ? "#66885e" : "#727ca1",
    );
    ctx.font = "12px 'Segoe UI',sans-serif";
    ctx.textAlign = "center";
    const label = player.name + (own ? " · 你" : "");
    const labelWidth = ctx.measureText(label).width + 16;
    ctx.fillStyle = "#0b2225c9";
    ctx.beginPath();
    ctx.roundRect(player.x - labelWidth / 2, player.y - 55, labelWidth, 22, 5);
    ctx.fill();
    ctx.fillStyle = own ? "#d2f5d9" : "#d0dae3";
    ctx.fillText(label, player.x, player.y - 40);
  }
  requestAnimationFrame(render);
}
requestAnimationFrame(render);
