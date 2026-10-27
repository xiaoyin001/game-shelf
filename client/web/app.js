// 文件说明：管理账号、联机和真实键盘意图；分数、坐标与时钟只读权威快照。
import { WorldView } from "./view3d.js";
import { ChallengePanel } from "./challenge.js";
const $ = (id) => document.getElementById(id);
const canvas = $("world");
let view = null,
  panel = null,
  rendererFailure = "";
let socket = null,
  state = null,
  round = null,
  selfID = "",
  connected = false,
  chatID = 0;
let joining = false,
  joinTimer = null,
  playersStamp = "",
  selfStamp = "";
const keys = new Set();
const errorMessages = {
  invalid_or_expired_token: "登录凭据已过期，请重新进入。",
  invalid_credentials: "账号或密码错误。",
  account_exists: "账号已存在，请使用原密码登录。",
  invalid_account: "账号需要 2–24 个字母、数字、下划线或连字符。",
  invalid_password: "密码需 6–32 位，仅限英文字母、数字和英文符号，不含空格。",
  auth_rate_limited: "登录尝试过多，请一分钟后再试。",
  auth_busy: "账号验证繁忙，请稍后再试。",
  db_unavailable: "账号服务暂时不可用，请稍后再试。",
  chat_too_fast: "说慢一点，让原野听清楚。",
  invalid_chat: "消息不能为空，最多 140 个字。",
  session_replaced_or_expired: "会话已结束，请重新进入。",
  world_full: "原野暂时已满，请稍后再试。",
  jump_not_grounded: "落地或站稳后才能再次跳跃。",
  single_world_unavailable: "世界服务尚未就绪，请稍后再试。",
  rate_limited: "请求过于频繁，连接已关闭。",
  already_in_round: "本局尚未结束，先完成挑战或退出。",
  no_round: "请先开始挑战。",
  stale_round: "局面已更新，请重新操作。",
  round_finished: "时间已到，本局已结算。",
  item_blocked: "先取走它上面的道具。",
  item_unavailable: "道具已被取走。",
  item_out_of_range: "走近道具并站稳后再取物。",
  unknown_item: "道具已经消除或盘面已更新。",
  hand_full: "手持已满三件，请先投送或归还。",
  item_not_held: "请选择自己手中的道具。",
  table_out_of_range: "走近配对台并站稳后再操作。",
  tray_full: "台面已满；仍可投送同类配对，或归还一件整理空间。",
};
function setText(id, value) {
  const text = String(value);
  if ($(id).textContent !== text) $(id).textContent = text;
}
function notice(text) {
  $("notice").textContent = text;
}
function status(text, live) {
  $("status").textContent = text;
  $("status-dot").classList.toggle("live", live);
}
function send(type, data = {}) {
  if (socket?.readyState === WebSocket.OPEN)
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
  round = null;
  selfID = "";
  playersStamp = selfStamp = "";
  $("online-count").textContent = "0 人在线";
  $("player-count").textContent = "0";
  $("players").replaceChildren();
  $("coordinates").textContent = "坐标 —";
  $("player-name").textContent = "尚未出发";
  $("join-button").textContent = "登录并进入原野 ↗";
  panel?.update(null, null, "");
  view?.setChallenge(null);
  status(rendererFailure ? "3D 不可用" : "连接已断开", false);
  notice(rendererFailure || text);
}
// 整次拒绝非法输入，不能把粘贴口令截成或清理成另一口令；提交时仍要再次验证。
const passwordInput = $("password");
let acceptedPassword = "";
const passwordDraft = /^[!-~]{0,32}$/;
function rejectPassword() {
  passwordInput.setCustomValidity(errorMessages.invalid_password);
  notice(errorMessages.invalid_password);
}
function canInsertPassword(text) {
  const candidate =
    passwordInput.value.slice(0, passwordInput.selectionStart) +
    text +
    passwordInput.value.slice(passwordInput.selectionEnd);
  return passwordDraft.test(candidate);
}
passwordInput.addEventListener("beforeinput", (event) => {
  if (!event.isComposing && event.data !== null && !canInsertPassword(event.data)) {
    event.preventDefault();
    rejectPassword();
  }
});
passwordInput.addEventListener("paste", (event) => {
  if (event.clipboardData && !canInsertPassword(event.clipboardData.getData("text"))) {
    event.preventDefault();
    rejectPassword();
  }
});
passwordInput.addEventListener("input", () => {
  if (!passwordDraft.test(passwordInput.value)) {
    passwordInput.value = acceptedPassword;
    rejectPassword();
    return;
  }
  acceptedPassword = passwordInput.value;
  passwordInput.setCustomValidity("");
});
$("login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (joining || !view || view.failed) return;
  const account = $("account").value.trim();
  const password = passwordInput.value;
  if (!account) return;
  if (!/^[!-~]{6,32}$/.test(password) || passwordInput.validity.customError) {
    rejectPassword();
    passwordInput.focus();
    return;
  }
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
    passwordInput.value = "";
    acceptedPassword = "";
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
        notice("点击开始单人挑战；E 取物，1–3 选择，靠近台按 F 投送。归还会断连击。");
      }
      if (event.state) {
        if (event.state.scene_version !== view.layout.version) {
          fail3D("场景与服务器版本不一致，请刷新页面。");
          return;
        }
        state = event.state;
        round = event.challenge || null;
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
  if (
    event.ctrlKey ||
    event.altKey ||
    event.metaKey ||
    !connected ||
    /INPUT|TEXTAREA|BUTTON|SELECT/.test(event.target.tagName)
  )
    return;
  const key = event.key.toLowerCase();
  if (movementKeys.has(key)) {
    event.preventDefault();
    keys.add(key);
  }
  if (!event.repeat && ["e", "f", "g", "1", "2", "3"].includes(key)) {
    event.preventDefault();
    panel?.key(key);
  }
  if (event.code === "Space") {
    event.preventDefault();
    if (!event.repeat) send("jump");
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
  setText("online-count", players.length + " 人在线");
  setText("player-count", players.length);
  const next = JSON.stringify(players);
  if (next !== playersStamp) {
    playersStamp = next;
    $("players").replaceChildren(
      ...players.map((p) => {
        const li = document.createElement("li");
        li.textContent = p.name + (p.id === selfID ? " · 你" : "");
        Object.assign(li.dataset, { x: p.x, y: p.y, z: p.z, grounded: p.grounded });
        return li;
      }),
    );
  }
  const self = JSON.stringify(me);
  if (me && self !== selfStamp) {
    selfStamp = self;
    setText("player-name", me.name);
    setText(
      "coordinates",
      "坐标 " + Math.round(me.x) + " / " + Math.round(me.y) + " · 高度 " + Math.round(me.z),
    );
    Object.assign($("coordinates").dataset, { x: me.x, y: me.y, z: me.z, grounded: me.grounded });
  }
  view.setChallenge(round);
  panel.update(round, state, selfID);
  const chats = state.chats || [],
    latest = chats.at(-1)?.id || 0;
  if (latest !== chatID) {
    chatID = latest;
    $("messages").replaceChildren(
      ...chats.map((c) => {
        const li = document.createElement("li"),
          n = document.createElement("strong"),
          t = document.createElement("span");
        n.textContent = c.name;
        t.textContent = c.text;
        li.append(n, t);
        return li;
      }),
    );
    $("messages").scrollTop = $("messages").scrollHeight;
  }
}
// 渲染失败必须阻止假进入，不用冻结画面或 2D 替代。
function fail3D(message) {
  rendererFailure = message;
  if (view) view.failed = true;
  socket?.close();
  $("join-button").disabled = true;
  $("register-button").disabled = true;
  $("renderer-status").textContent = "3D 不可用";
  status("3D 不可用", false);
  notice(message);
}
try {
  const response = await fetch("/scene.json", { cache: "no-store" });
  if (!response.ok) throw new Error("场景加载失败");
  const layout = await response.json();
  view = new WorldView(canvas, fail3D, layout);
  await view.prepare();
  $("join-button").disabled = false;
  $("register-button").disabled = false;
  panel = new ChallengePanel((type, data) => {
    keys.clear();
    send(type, data);
    canvas.focus({ preventScroll: true });
  }, view);
  $("renderer-status").textContent = "3D · WebGL2";
} catch (error) {
  fail3D("无法启动 3D / WebGL2：" + error.message + "；请启用硬件加速或使用支持 WebGL2 的浏览器。");
}
let lastRender = 0;
function render(now) {
  try {
    const delay = view && !view.failed ? view.frameDelay(now, state) : 1000 / 12;
    if (now - lastRender < delay) {
      requestAnimationFrame(render);
      return;
    }
    lastRender = now;
    if (view && !view.failed) {
      const camera = view.render(now, state, selfID);
      view.setCanvasData("renderMode", delay === 1000 / 30 ? "active" : "idle");
      setText("camera-readout", "视角 " + camera.heading + "° · 距离 " + camera.distance);
    }
  } catch (error) {
    fail3D("3D 渲染失败：" + error.message);
  }
  requestAnimationFrame(render);
}
requestAnimationFrame(render);
