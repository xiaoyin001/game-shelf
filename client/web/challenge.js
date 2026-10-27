// 文件说明：呈现权威局面与可选择的手持操作；本地倒计时不提交成绩。
const $ = (id) => document.getElementById(id);
export const itemNames = {
  mushroom: ["🍄", "绯伞菇"],
  pinecone: ["◆", "松果"],
  crystal: ["✦", "蓝晶"],
  berry: ["●", "紫莓"],
  leaf: ["❧", "绿叶"],
  shell: ["◕", "金贝"],
};
const label = (item) => itemNames[item.kind]?.join(" ") || item.kind;
const text = (id, value) => {
  const e = $(id),
    v = String(value);
  if (e.textContent !== v) e.textContent = v;
};
const prop = (id, key, value) => {
  if ($(id)[key] !== value) $(id)[key] = value;
};
const data = (id, key, value) => {
  const v = String(value);
  if ($(id).dataset[key] !== v) $(id).dataset[key] = v;
};
export class ChallengePanel {
  constructor(send, view) {
    this.send = send;
    this.view = view;
    this.selected = "";
    this.stamp = "";
    this.boardStamp = "";
    this.previous = null;
    $("start-solo").addEventListener("click", () => {
      this.unlockAudio();
      send("start_solo");
    });
    $("again").addEventListener("click", () => send("start_solo"));
    $("quit-round").addEventListener("click", () => {
      if (this.round?.phase !== "running" || window.confirm("退出会放弃本局成绩，确定返回大厅？"))
        send("leave_round");
    });
    $("pick-item").addEventListener("click", () => this.key("e"));
    $("put-item").addEventListener("click", () => this.key("f"));
    $("return-item").addEventListener("click", () => this.key("g"));
    $("sound").addEventListener("change", () => {
      if (!$("sound").checked) this.audio?.suspend().catch(() => {});
    });
  }
  unlockAudio() {
    if (this.audio || !$("sound").checked) return;
    const Context = window.AudioContext || window.webkitAudioContext;
    if (Context) {
      this.audio = new Context();
      this.audio.suspend().catch(() => {});
    }
  }
  tone(multiplier) {
    if (!$("sound").checked || !this.audio) return;
    clearTimeout(this.audioSleep);
    const ctx = this.audio;
    ctx
      .resume()
      .then(() => {
        const start = ctx.currentTime;
        for (let i = 0; i < 2; i++) {
          const oscillator = ctx.createOscillator(),
            gain = ctx.createGain();
          oscillator.type = "sine";
          oscillator.frequency.value = (360 + multiplier * 12) * (i ? 1.5 : 1);
          gain.gain.setValueAtTime(0.0001, start + i * 0.075);
          gain.gain.exponentialRampToValueAtTime(0.12, start + i * 0.075 + 0.01);
          gain.gain.exponentialRampToValueAtTime(0.0001, start + i * 0.075 + 0.19);
          oscillator.connect(gain);
          gain.connect(ctx.destination);
          oscillator.start(start + i * 0.075);
          oscillator.stop(start + i * 0.075 + 0.2);
        }
        this.audioSleep = setTimeout(() => ctx.suspend().catch(() => {}), 350);
      })
      .catch(() => {});
  }
  key(key) {
    if (["1", "2", "3"].includes(key)) {
      this.selected = this.hand.find((i) => i.slot === Number(key) - 1)?.id || "";
      this.drawInventory();
      return;
    }
    if (this.round?.phase !== "running") return;
    this.unlockAudio();
    const item = key === "e" ? this.nearest : this.hand.find((i) => i.id === this.selected);
    if (!item) return;
    this.send(key === "e" ? "pick" : key === "f" ? "put" : "return", {
      item_id: item.id,
      round_id: this.round.id,
    });
  }
  update(round, state, id) {
    this.round = round;
    this.selfID = id;
    this.me = state?.players?.find((p) => p.id === id);
    this.hand = (round?.items || [])
      .filter((i) => i.place === "hand" && i.owner_id === id)
      .sort((a, b) => a.slot - b.slot);
    if (!this.hand.some((i) => i.id === this.selected)) this.selected = this.hand[0]?.id || "";
    const active = round?.phase === "running",
      finished = round?.phase === "finished";
    prop("challenge-controls", "hidden", !id);
    prop("start-solo", "hidden", !!round);
    prop("quit-round", "hidden", !round);
    prop("round-panel", "hidden", !round);
    prop("result", "hidden", !finished);
    prop("again", "hidden", !finished);
    data("challenge", "phase", round?.phase || "lobby");
    data("challenge", "roundId", round?.id || "");
    text("score", round?.score || 0);
    text("timer", round ? Math.max(0, Math.ceil(round.remaining_ms / 1000)) : "180");
    data("timer", "endMs", round?.end_ms || "");
    data("timer", "startMs", round?.start_ms || "");
    text("multiplier", "×" + ((round?.multiplier || 10) / 10).toFixed(1));
    text(
      "combo-time",
      active && round.combo_remaining_ms > 0
        ? Math.ceil(round.combo_remaining_ms / 1000) + " 秒内配对续连击"
        : "配对开始连击",
    );
    text("board-number", round?.board || 1);
    text("pairs", round?.pairs || 0);
    text("boards-cleared", round?.boards || 0);
    text(
      "result-summary",
      finished
        ? "本局 " + round.score + " 分 · " + round.pairs + " 对 · 清盘 " + round.boards + " 次"
        : "",
    );
    text("round-rules", round ? round.day + " UTC · " + round.rules : "180 秒 · 同日相同关卡");
    const dist = (i) =>
      this.me ? Math.hypot(this.me.x - i.x, this.me.y - i.y, this.me.z - i.z) : Infinity;
    this.nearest =
      active && this.me?.grounded
        ? (round.items || [])
            .filter(
              (i) =>
                i.place === "ground" &&
                !i.blocked &&
                dist(i) <= this.view.layout.workshop.pickup_range,
            )
            .sort((a, b) => dist(a) - dist(b))[0]
        : null;
    const a = this.view.layout.workshop;
    this.atTable =
      !!this.me?.grounded && Math.hypot(this.me.x - a.x, this.me.y - a.y, this.me.z) <= a.range;
    prop("pick-item", "disabled", !this.nearest || this.hand.length >= 3);
    prop("put-item", "disabled", !active || !this.hand.length || !this.atTable);
    prop("return-item", "disabled", !active || !this.hand.length);
    text(
      "interaction-hint",
      !active
        ? "WASD 移动 · 点击开始挑战"
        : this.nearest
          ? "E 拾取 " + label(this.nearest)
          : this.atTable
            ? "配对台 · 1–3 选择，F 投送 · G 归还会断连击"
            : "向右侧道具靠近 · E 取物 · 三件手持已满请返台",
    );
    text("pick-item", this.nearest ? "拾取 " + label(this.nearest) + " · E" : "靠近道具取物 · E");
    this.drawInventory();
    this.drawGround(active, dist);
    if (round && this.previous?.id === round.id && round.pairs > this.previous.pairs) {
      this.view.pairEffect(round.multiplier);
      this.tone(round.multiplier);
      $("score").classList.remove("score-pop");
      void $("score").offsetWidth;
      $("score").classList.add("score-pop");
      text(
        "pair-feedback",
        "配对 +" +
          (round.score - this.previous.score - (round.boards - this.previous.boards) * 500) +
          " · " +
          (round.multiplier / 10).toFixed(1) +
          " 倍" +
          (round.boards > this.previous.boards ? " · 清盘 +500" : ""),
      );
    } else if (this.previous?.id !== round?.id) text("pair-feedback", "");
    this.previous = round
      ? { id: round.id, pairs: round.pairs, score: round.score, boards: round.boards }
      : null;
  }
  drawInventory() {
    const r = this.round;
    const stamp = JSON.stringify([
      r?.id,
      r?.phase,
      r?.items?.filter((i) => i.place !== "ground"),
      this.selected,
      this.atTable,
    ]);
    if (stamp === this.stamp) return;
    this.stamp = stamp;
    $("hand").replaceChildren(
      ...Array.from({ length: 3 }, (_, slot) => {
        const i = this.hand.find((i) => i.slot === slot),
          button = document.createElement("button");
        button.type = "button";
        button.textContent = slot + 1 + " · " + (i ? label(i) : "空");
        button.disabled = !i;
        button.setAttribute("aria-pressed", String(i?.id === this.selected));
        button.dataset.itemId = i?.id || "";
        button.dataset.kind = i?.kind || "";
        button.dataset.slot = slot;
        button.addEventListener("click", () => {
          this.selected = i.id;
          this.drawInventory();
        });
        return button;
      }),
    );
    $("tray").replaceChildren(
      ...Array.from({ length: 5 }, (_, slot) => {
        const i = r?.items?.find((i) => i.place === "tray" && i.slot === slot),
          b = document.createElement("button");
        b.type = "button";
        b.textContent = i ? label(i) + " · 归还" : "空";
        b.title = "台面归还原位，中断连击";
        b.disabled = !i || !this.atTable || r.phase !== "running";
        b.dataset.itemId = i?.id || "";
        b.dataset.kind = i?.kind || "";
        b.addEventListener("click", () => this.send("return", { item_id: i.id, round_id: r.id }));
        return b;
      }),
    );
  }
  drawGround(active, dist) {
    const items = this.round?.items?.filter((i) => i.place === "ground") || [];
    const available = (i) =>
      active &&
      !i.blocked &&
      this.hand.length < 3 &&
      this.me?.grounded &&
      dist(i) <= this.view.layout.workshop.pickup_range;
    const stamp = JSON.stringify(items.map((i) => [i.id, i.blocked, available(i)]));
    if (stamp === this.boardStamp) return;
    this.boardStamp = stamp;
    $("board-items").replaceChildren(
      ...items.map((i) => {
        const b = document.createElement("button");
        b.type = "button";
        b.textContent = label(i) + (i.blocked ? " · 遮挡" : i.z ? " · 上层" : " · 下层");
        b.disabled = !available(i);
        Object.assign(b.dataset, {
          itemId: i.id,
          kind: i.kind,
          x: i.x,
          y: i.y,
          z: i.z,
          blocked: i.blocked,
        });
        b.addEventListener("click", () =>
          this.send("pick", { item_id: i.id, round_id: this.round.id }),
        );
        return b;
      }),
    );
  }
}
