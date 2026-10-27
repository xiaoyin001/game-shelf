// 文件说明：显式读取真实已保存的每日榜和本人最佳，不在世界轮询中读库或伪造空榜。
const $ = (id) => document.getElementById(id);
export class RankingsPanel {
  constructor(send) {
    this.send = send;
    this.online = false;
    $("rankings").addEventListener("toggle", () => {
      if ($("rankings").open && this.online) this.query();
    });
    $("ranking-form").addEventListener("submit", (e) => {
      e.preventDefault();
      this.query();
    });
  }
  connect(online, time) {
    this.online = online;
    $("rankings-submit").disabled = !online;
    $("rankings-mode").disabled = !online;
    $("rankings-day").disabled = !online;
    clearTimeout(this.timer);
    clearTimeout(this.cooldown);
    this.loading = false;
    this.nextQuery = 0;
    $("ranking-list").replaceChildren();
    $("best-solo").textContent = "个人单人最佳：尚未查询";
    $("best-duo").textContent = "个人双人最佳：尚未查询";
    $("ranking-state").textContent = online ? "展开后查询真实成绩" : "登录后查看排行榜";
    if (online) {
      $("rankings-day").value = new Date(time).toISOString().slice(0, 10);
      if ($("rankings").open) this.query();
    }
  }
  query() {
    if (
      !this.online ||
      this.loading ||
      Date.now() < this.nextQuery ||
      !$("ranking-form").reportValidity()
    )
      return;
    this.loading = true;
    this.nextQuery = Date.now() + 850;
    $("rankings-submit").disabled = true;
    $("rankings-mode").disabled = true;
    $("rankings-day").disabled = true;
    $("best-solo").textContent = "个人单人最佳：读取中";
    $("best-duo").textContent = "个人双人最佳：读取中";
    $("ranking-state").textContent = "正在读取已保存成绩…";
    $("ranking-list").replaceChildren();
    this.send("leaderboard", { mode: $("rankings-mode").value, day: $("rankings-day").value });
    clearTimeout(this.timer);
    this.timer = setTimeout(() => this.fail("查询未完成，请重试；没有把旧数据当成新结果。"), 6000);
  }
  release() {
    $("rankings-mode").disabled = !this.online;
    $("rankings-day").disabled = !this.online;
    clearTimeout(this.cooldown);
    this.cooldown = setTimeout(
      () => {
        $("rankings-submit").disabled = !this.online;
      },
      Math.max(0, this.nextQuery - Date.now()),
    );
  }
  fail(message) {
    if (!this.loading) return;
    clearTimeout(this.timer);
    this.loading = false;
    this.release();
    $("best-solo").textContent = "个人单人最佳：未能读取";
    $("best-duo").textContent = "个人双人最佳：未能读取";
    $("ranking-state").textContent = message;
  }
  update(board) {
    if (!board) return;
    clearTimeout(this.timer);
    this.loading = false;
    this.release();
    if (board.error) {
      $("best-solo").textContent = "个人单人最佳：未能读取";
      $("best-duo").textContent = "个人双人最佳：未能读取";
      $("ranking-state").textContent = "成绩服务暂时不可用，请稍后刷新。";
      $("ranking-list").replaceChildren();
      return;
    }
    $("rankings").dataset.mode = board.mode;
    $("rankings").dataset.day = board.day;
    $("rankings").dataset.rules = board.rules;
    const entries = board.entries || [];
    $("ranking-state").textContent =
      board.day +
      " UTC · " +
      board.rules +
      " · " +
      (board.mode === "duo" ? "双人固定组合" : "单人每日") +
      " · " +
      (entries.length ? "前20名，每账号/组合仅最高分" : "暂无有效成绩");
    $("ranking-list").replaceChildren(
      ...entries.map((r, i) => {
        const li = document.createElement("li"),
          names = document.createElement("span"),
          score = document.createElement("strong");
        names.textContent = i + 1 + ". " + r.members.map((m) => m.name).join(" + ");
        score.textContent = r.score + " 分";
        li.dataset.roundId = r.round_id;
        li.append(names, score);
        return li;
      }),
    );
    const best = (r) =>
      r
        ? r.score + " 分 · " + r.members.map((m) => m.name).join(" + ") + " · " + r.day + " UTC"
        : "尚无已保存有效成绩";
    $("best-solo").textContent = "个人单人最佳：" + best(board.best_solo);
    $("best-duo").textContent = "个人双人最佳（团队分）：" + best(board.best_duo);
  }
}
