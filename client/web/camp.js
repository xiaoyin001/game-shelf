// 文件说明：呈现服务器确认的个人展台与在线参观选择，表单只发送自己的槽位意图。
const $ = (id) => document.getElementById(id);

export class CampPanel {
  constructor(view, decorate) {
    this.view = view;
    this.book = null;
    this.selfID = "";
    this.owner = "";
    this.state = null;
    this.ownersStamp = "";
    this.open = false;
    for (const id of ["book-tab", "camp-tab"]) {
      $(id).addEventListener("click", () => {
        this.open = id === "camp-tab";
        this.refresh();
      });
    }
    this.controls = view.layout.camp.slots.map((slot) => {
      const row = document.createElement("div");
      const label = document.createElement("label");
      const select = document.createElement("select");
      const button = document.createElement("button");
      row.className = "camp-slot";
      select.id = "camp-slot-" + slot.id;
      label.htmlFor = select.id;
      label.textContent = slot.name + "摆件";
      button.type = "button";
      button.textContent = "布置" + slot.name;
      button.addEventListener("click", () => {
        decorate({ slot_id: slot.id, decoration_id: select.value });
      });
      row.append(label, select, button);
      $("camp-controls").append(row);
      return { slot, select, button };
    });
    $("camp-owner").addEventListener("change", () => {
      this.owner = $("camp-owner").value;
      this.refresh();
    });
  }

  updateBook(book) {
    this.book = book;
    if (!book) {
      this.selfID = "";
      this.owner = "";
      this.open = false;
      this.state = null;
      this.ownersStamp = "";
      this.view.setCamp(null);
    }
    for (const { slot, select } of this.controls) {
      const empty = document.createElement("option");
      empty.value = "";
      empty.textContent = "收起 / 空位";
      select.replaceChildren(
        empty,
        ...(book?.decorations || []).map((decoration) => {
          const option = document.createElement("option");
          option.value = decoration.id;
          option.textContent = decoration.name + (decoration.unlocked ? "" : " · 未解锁");
          option.disabled = !decoration.unlocked;
          return option;
        }),
      );
      select.value = book?.camp?.find((placed) => placed.slot_id === slot.id)?.decoration_id || "";
    }
    this.refresh();
  }

  updateState(state, selfID) {
    this.state = state;
    this.selfID = selfID;
    const camps = state.camps || [];
    if (!camps.some((camp) => camp.owner_id === this.owner)) this.owner = selfID;
    const stamp = JSON.stringify(camps.map((camp) => [camp.owner_id, camp.name]));
    if (stamp !== this.ownersStamp) {
      this.ownersStamp = stamp;
      $("camp-owner").replaceChildren(
        ...camps.map((camp) => {
          const option = document.createElement("option");
          option.value = camp.owner_id;
          option.textContent = camp.name + (camp.owner_id === selfID ? " · 你" : "");
          return option;
        }),
      );
    }
    $("camp-owner").value = this.owner;
    this.refresh();
  }

  refresh() {
    const offline = !this.book || !this.selfID;
    $("journal-tabs").hidden = offline;
    $("field-book").hidden = offline || this.open;
    $("camp-panel").hidden = offline || !this.open;
    $("book-tab").setAttribute("aria-selected", String(!this.open));
    $("camp-tab").setAttribute("aria-selected", String(this.open));
    const display = this.state?.camps?.find((camp) => camp.owner_id === this.owner);
    this.view.setCamp(display || null);
    const own = this.owner === this.selfID;
    const me = this.state?.players?.find((player) => player.id === this.selfID);
    const camp = this.view.layout.camp;
    const inRange = me?.grounded && Math.hypot(me.x - camp.x, me.y - camp.y, me.z) <= camp.range;
    for (const { select, button } of this.controls) {
      select.disabled = button.disabled = !own || !inRange;
    }
    $("camp-controls").hidden = !own;
    const placed = display?.slots || [];
    const summary = $("camp-summary");
    summary.dataset.owner = display?.owner_id || "";
    summary.dataset.count = placed.length;
    summary.textContent = display
      ? display.name + "的展台 · " + placed.length + " / 3"
      : "尚未选择展台";
    $("camp-location").textContent = !own
      ? "正在只读参观。切回自己的展台，才能布置。"
      : inRange
        ? "已在营地展台附近，可布置、移动或收起。变更由服务器保存。"
        : "展台在 " + camp.x + " / " + camp.y + "，走近并站稳后才能布置。";
    const text =
      placed
        .map((slot) => {
          const position =
            camp.slots.find((item) => item.id === slot.slot_id)?.name || slot.slot_id;
          const name =
            this.book?.decorations?.find((item) => item.id === slot.decoration_id)?.name ||
            slot.decoration_id;
          return position + "：" + name;
        })
        .join(" · ") || "还没有摆件。每件旧物都会解锁一款微光投影。";
    if ($("camp-placed").textContent !== text) $("camp-placed").textContent = text;
  }
}
