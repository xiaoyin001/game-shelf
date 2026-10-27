// 文件说明：真实子进程验证私有就绪/完整双等待/失败/超时/中断与定向父子进程回收，不当作游戏通过。
import test from "node:test";
import assert from "node:assert/strict";
import { setTimeout as delay } from "node:timers/promises";
import { managedTask } from "../../scripts/managed-task.mjs";

const task = (code, options = {}) =>
  managedTask(process.execPath, ["-e", code], { output: () => {}, ...options });
const alive = (pid) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch (e) {
    if (e.code === "ESRCH") return false;
    throw e;
  }
};
async function gone(pid) {
  for (let i = 0; i < 30; i++) {
    if (!alive(pid)) return;
    await delay(40);
  }
  assert.fail("owned descendant still alive: " + pid);
}
test(
  "分段/重复/无关信号不能冒充全部私有局就绪；正常双任务均须完成",
  { timeout: 5000 },
  async () => {
    let done = false;
    const a = task(
      'console.log("GAME_SHELF_PRIVATE_READY:Unrelated");process.stdout.write("GAME_SHELF_PRIVATE_");setTimeout(()=>{console.log("READY:RoundA");console.log("GAME_SHELF_PRIVATE_READY:RoundA");},20);setTimeout(()=>console.log("GAME_SHELF_PRIVATE_READY:RoundB"),100);setTimeout(()=>{},160);',
      { markers: ["RoundA", "RoundB"], readyTimeout: 2000 },
    );
    const b = task("setTimeout(()=>{},220);");
    try {
      a.ready.then(() => {
        done = true;
      });
      await delay(60);
      assert.equal(done, false);
      await a.ready;
      assert.equal(done, true);
      await Promise.all([a.done, b.done]);
    } finally {
      await Promise.all([a.stop(), b.stop()]);
    }
  },
);
test("缺信号/非零退出/就绪超时明确拒绝，不将退出当通过", { timeout: 10000 }, async () => {
  const missing = task("console.log('ordinary output');", { markers: ["RoundA"] });
  try {
    await assert.rejects(missing.ready, /before private/);
    await missing.done;
  } finally {
    await missing.stop();
  }
  const failed = task(
    'console.log("GAME_SHELF_PRIVATE_READY:RoundA");setTimeout(()=>process.exit(7),30);',
    { markers: ["RoundA"] },
  );
  try {
    await failed.ready;
    await assert.rejects(failed.done, /exited 7/);
  } finally {
    await failed.stop();
  }
  const timeout = task("setInterval(()=>{},1000);", { markers: ["RoundA"], readyTimeout: 40 });
  try {
    await assert.rejects(timeout.ready, /not ready/);
  } finally {
    await timeout.stop();
  }
  await assert.rejects(timeout.done, /exited/);
});
const treeCode = `
const {spawn}=require("node:child_process");
const child=spawn(process.execPath,["-e","setInterval(()=>{},1000)"],{stdio:"ignore"});
console.log("DESCENDANT_PID:"+child.pid);
console.log("GAME_SHELF_PRIVATE_READY:Bound");
const timer=setInterval(()=>{},1000);
process.on("SIGTERM",()=>clearInterval(timer));
`;
test("主动中断定向结束本任务父子树，不留下Go包装器之外的子进程", { timeout: 15000 }, async () => {
  const controller = new AbortController();
  let descendant;
  const active = task(treeCode, {
    markers: ["Bound"],
    signal: controller.signal,
    output: (chunk) => {
      const m = /DESCENDANT_PID:(\d+)/.exec(chunk.toString());
      if (m) descendant = Number(m[1]);
    },
  });
  try {
    await active.ready;
    assert.ok(descendant && alive(descendant));
    controller.abort();
    await assert.rejects(active.done, /aborted/);
  } finally {
    await active.stop();
  }
  await gone(descendant);
});
test("另一任务失败时finally回收仍在等待的进程树", { timeout: 15000 }, async () => {
  let descendant;
  const long = task(treeCode, {
    markers: ["Bound"],
    output: (chunk) => {
      const m = /DESCENDANT_PID:(\d+)/.exec(chunk.toString());
      if (m) descendant = Number(m[1]);
    },
  });
  let failed;
  try {
    await long.ready;
    failed = task("process.exit(9)");
    await assert.rejects(Promise.all([long.done, failed.done]), /exited 9/);
  } finally {
    await Promise.all([long.stop(), failed?.stop()]);
  }
  await gone(descendant);
});
test("已中断的请求不启动可遗留的等待", { timeout: 10000 }, async () => {
  const controller = new AbortController();
  controller.abort();
  const active = task("setInterval(()=>{},1000)", {
    signal: controller.signal,
    markers: ["Bound"],
  });
  try {
    await assert.rejects(active.ready, /aborted/);
    await assert.rejects(active.done, /aborted/);
  } finally {
    await active.stop();
  }
});
