// 文件说明：管理账号与真实联机输入，把权威快照交给 3D 视图，不在客户端决定奖励或坐标。
import { WorldView } from "./view3d.js";
const $ = (id) => document.getElementById(id);
const canvas = $("world");
let view = null;
let rendererFailure = "";
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
  $("join-button").disabled = !view || view.failed;
  $("register-button").disabled = !view || view.failed;
  state = null;
  selfID = "";
  $("online-count").textContent = "0 人在线";
  $("player-count").textContent = "0";
  $("players").replaceChildren();
  $("score").textContent = "0";
  $("coordinates").textContent = "坐标 —";
  $("player-name").textContent = "尚未出发";
  $("join-button").textContent = "登录并进入原野 ↗";
  status(rendererFailure ? "3D 不可用" : "连接已断开", false);
  notice(rendererFailure || text);
}
$("login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (joining || !view || view.failed) return;
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
  if (event.ctrlKey || event.altKey || event.metaKey) return;
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
  if (key === "r") {
    event.preventDefault();
    view?.resetCamera();
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
  send("input", view.input(dx, dy));
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
// 渲染错误必须可见且阻止假进入，不回退为 2D 或静态截图。
function fail3D(message) {
  rendererFailure = message;
  if (view) view.failed = true;
  if (socket) socket.close();
  $("join-button").disabled = true;
  $("register-button").disabled = true;
  $("renderer-status").textContent = "3D 不可用";
  status("3D 不可用", false);
  notice(message);
}
try {
  view = new WorldView(canvas, fail3D);
  $("renderer-status").textContent = "3D · WebGL2";
} catch (error) {
  fail3D("无法启动 3D / WebGL2：" + error.message + "；请启用硬件加速或使用支持 WebGL2 的浏览器。");
}
let lastRender = 0;
function render(now) {
  // 30fps 足够呈现 10Hz 权威快照的插值，避免软件渲染挤占表单和网络事件。
  if (now - lastRender < 1000 / 30) {
    requestAnimationFrame(render);
    return;
  }
  lastRender = now;
  if (view && !view.failed) {
    try {
      const camera = view.render(now, state, selfID);
      $("camera-readout").textContent = "视角 " + camera.heading + "° · 距离 " + camera.distance;
    } catch (error) {
      fail3D("3D 渲染失败：" + error.message);
    }
  }
  requestAnimationFrame(render);
}
requestAnimationFrame(render);
