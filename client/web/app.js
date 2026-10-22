// 文件说明：管理账号与真实联机输入，把权威快照交给 3D 视图，不在客户端决定奖励或坐标。
import { WorldView } from "./view3d.js";
const $ = (id) => document.getElementById(id);
const canvas = $("world");
let view = null;
let rendererFailure = "";
let dialogNPC = "";
let dialogDismissed = false;
let nearestNPC = null;
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
  invalid_password: "密码需 6–32 位，仅限英文字母、数字和英文符号，不含空格。",
  auth_rate_limited: "登录尝试过多，请一分钟后再试。",
  auth_busy: "账号验证繁忙，请稍后再试。",
  db_unavailable: "账号服务暂时不可用，请稍后再试。",
  no_resource_in_range: "附近没有可采集的晶簇，试试向发光处走近一点。",
  chat_too_fast: "说慢一点，让原野听清楚。",
  invalid_chat: "消息不能为空，最多 140 个字。",
  unknown_npc: "这位营地人物不在原野中。",
  npc_out_of_range: "请再走近一点，才能与这位人物交谈。",
  invalid_npc_choice: "这个对话选项不可用，请重新交谈。",
  quest_not_ready: "先接取委托，再成功采集两次晶簇，回来领取奖励。",
  session_replaced_or_expired: "会话已结束，请重新进入。",
  world_full: "原野暂时已满，请稍后再试。",
  jump_not_grounded: "落地或站稳后才能再次跳跃。",
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
  closeDialog(false);
  nearestNPC = null;
  $("npc-talk").hidden = true;
  updateQuest(null);
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
        notice("靠近守灯人阿苔接取委托；E 交谈 / 采集，Enter 与旅人聊天。");
      }
      if (event.state) {
        if (event.state.scene_version !== view.layout.version) {
          fail3D("场景与服务器版本不一致，请刷新页面。");
          return;
        }
        state = event.state;
        updateUI();
      }
      if (event.quest) updateQuest(event.quest);
      if (event.dialog && !dialogDismissed) showDialog(event.dialog);
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
  if (!connected) return;
  if (event.key === "Escape" && dialogNPC) {
    event.preventDefault();
    closeDialog();
    return;
  }
  if (dialogNPC || /INPUT|TEXTAREA|BUTTON/.test(event.target.tagName)) return;
  const key = event.key.toLowerCase();
  if (movementKeys.has(key)) {
    event.preventDefault();
    keys.add(key);
  }
  if (key === "e" && !event.repeat) {
    event.preventDefault();
    if (nearestNPC) talkToNPC();
    else send("gather");
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
  send("input", view.input(dialogNPC ? 0 : dx, dialogNPC ? 0 : dy));
}, 100);
function updateQuest(quest) {
  const journal = $("quest-progress");
  journal.dataset.status = quest?.status || "";
  journal.dataset.progress = quest?.progress || 0;
  const text =
    quest?.status === "completed"
      ? "委托已完成 · 已领取额外 3 分微光"
      : quest?.status === "active"
        ? "采集进度 " +
          quest.progress +
          " / " +
          quest.goal +
          (quest.progress >= quest.goal ? " · 返回阿苔领取奖励" : " · 寻找发光晶簇")
        : "与守灯人阿苔交谈，接取采集委托。";
  if (journal.textContent !== text) journal.textContent = text;
}
function updateInteraction(me) {
  const distance = (npc) => Math.hypot(me.x - npc.x, me.y - npc.y, me.z);
  nearestNPC =
    view.layout.npcs
      .filter((npc) => distance(npc) <= npc.range)
      .sort((a, b) => distance(a) - distance(b))[0] || null;
  if (dialogNPC) {
    const npc = view.layout.npcs.find((n) => n.id === dialogNPC);
    if (!npc || distance(npc) > npc.range) closeDialog();
  }
  $("npc-talk").hidden = !nearestNPC || !!dialogNPC;
  const node = (state.nodes || []).find((n) => Math.hypot(me.x - n.x, me.y - n.y, me.z) <= 72);
  const resourceHint = node
    ? node.ready_at_ms <= state.time_ms
      ? "E 采集微光"
      : "晶簇重生中 · " + Math.ceil((node.ready_at_ms - state.time_ms) / 1000) + " 秒"
    : "矮台阶自动迈上 · 高障碍按空格跳跃 · E 交谈 / 采集";
  $("interaction-hint").textContent = dialogNPC
    ? "正在交谈 · Esc 结束"
    : nearestNPC
      ? "E 与" +
        nearestNPC.role +
        "「" +
        nearestNPC.name +
        "」交谈 · " +
        Math.round(distance(nearestNPC)) +
        " 步"
      : resourceHint;
}
function talkToNPC() {
  if (!connected || !nearestNPC) return;
  keys.clear();
  dialogDismissed = false;
  send("interact", { npc_id: nearestNPC.id });
}
function showDialog(dialog) {
  const opening = !dialogNPC;
  dialogNPC = dialog.npc_id;
  keys.clear();
  $("npc-name").textContent = dialog.name;
  $("npc-text").textContent = dialog.text;
  $("npc-choices").replaceChildren(
    ...(dialog.choices || []).map((choice) => {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = choice.label;
      button.addEventListener("click", () => {
        send("choose", { npc_id: dialogNPC, choice_id: choice.id });
      });
      return button;
    }),
  );
  $("npc-dialog").hidden = false;
  $("npc-talk").hidden = true;
  // 响应替换选项会移除原焦点；始终放回当前面板，不让空格误触跳跃。
  if (opening || !$("npc-dialog").contains(document.activeElement))
    ($("npc-choices").firstElementChild || $("npc-close")).focus({ preventScroll: true });
}
function closeDialog(focus = true) {
  const wasOpen = !!dialogNPC;
  dialogNPC = "";
  dialogDismissed = true;
  keys.clear();
  $("npc-dialog").hidden = true;
  $("npc-choices").replaceChildren();
  if (focus && wasOpen) canvas.focus({ preventScroll: true });
}
$("npc-talk").addEventListener("click", talkToNPC);
$("npc-close").addEventListener("click", () => closeDialog());
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
      li.dataset.z = player.z;
      li.dataset.grounded = player.grounded;
      li.title = "坐标 " + Math.round(player.x) + " / " + Math.round(player.y);
      li.append(name, score);
      return li;
    }),
  );
  if (me) {
    const previous = Number($("score").textContent);
    $("score").textContent = me.score;
    $("player-name").textContent = me.name;
    $("coordinates").textContent =
      "坐标 " + Math.round(me.x) + " / " + Math.round(me.y) + " · 高度 " + Math.round(me.z);
    $("coordinates").dataset.x = me.x;
    $("coordinates").dataset.y = me.y;
    $("coordinates").dataset.z = me.z;
    $("coordinates").dataset.grounded = me.grounded;
    if (me.score > previous)
      notice("获得 " + (me.score - previous) + " 分微光 ✦ 采集与委托奖励由服务器确认。");
    updateInteraction(me);
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
  const response = await fetch("/scene.json", { cache: "no-store" });
  if (!response.ok) throw new Error("场景加载失败");
  const layout = await response.json();
  view = new WorldView(canvas, fail3D, layout);
  $("npc-list").replaceChildren(
    ...layout.npcs.map((npc) => {
      const li = document.createElement("li"),
        position = document.createElement("small");
      li.textContent = npc.role + " · " + npc.name;
      position.textContent = "坐标 " + npc.x + " / " + npc.y + " · 靠近按 E 交谈";
      li.append(position);
      return li;
    }),
  );
  $("renderer-status").textContent = "3D · WebGL2";
} catch (error) {
  fail3D("无法启动 3D / WebGL2：" + error.message + "；请启用硬件加速或使用支持 WebGL2 的浏览器。");
}
let lastRender = 0;
function render(now) {
  // 活动时维持30fps目标插值，静止时12fps继续真实绘制，给双窗口表单和网络留出预算。
  try {
    const delay = view && !view.failed ? view.frameDelay(now, state) : 1000 / 12;
    if (now - lastRender < delay) {
      requestAnimationFrame(render);
      return;
    }
    lastRender = now;
    if (view && !view.failed) {
      const camera = view.render(now, state, selfID);
      canvas.dataset.renderMode = delay === 1000 / 30 ? "active" : "idle";
      $("camera-readout").textContent = "视角 " + camera.heading + "° · 距离 " + camera.distance;
    }
  } catch (error) {
    fail3D("3D 渲染失败：" + error.message);
  }
  requestAnimationFrame(render);
}
requestAnimationFrame(render);
