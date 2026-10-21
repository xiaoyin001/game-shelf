// 文件说明：从真实 Git 对象核对本次成长序列，不将源码检查冒充历史运行测试。
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
function git(args) {
  const r = spawnSync("git", args, { encoding: "utf8", windowsHide: true });
  if (r.error || r.status !== 0)
    throw r.error || new Error(r.stderr + "；需要完整历史，请先补齐浅克隆。");
  return r.stdout.trim();
}
const { stages } = JSON.parse(await readFile("docs/history.json", "utf8"));
const sequence = git(["rev-list", "--reverse", "--first-parent", "HEAD"]).split("\n");
assert.ok(sequence.length >= stages.length, "成长历史不完整");
for (const [i, stage] of stages.entries()) {
  assert.ok(sequence[i].startsWith(stage.commit), "阶段祖先顺序不符：" + stage.stage);
  assert.equal(git(["show", "-s", "--format=%s", stage.commit]), stage.subject);
  assert.ok(stage.verification.length > 0, "缺少当次验证记录");
}
assert.equal(git(["rev-list", "--max-parents=0", "HEAD"]), sequence[0], "新历史不是单独空根");
function file(stage, path) {
  const r = spawnSync("git", ["show", stages[stage - 1].commit + ":" + path], {
    encoding: "utf8",
    windowsHide: true,
  });
  return r.status === 0 ? r.stdout : null;
}
assert.equal(file(1, "go.mod"), null, "规则阶段提前包含代码");
assert.ok(file(2, "internal/protocol/protocol.go"));
assert.equal(file(2, "cmd/login/main.go"), null);
assert.ok(file(3, "tests/infrastructure/infra_test.go"));
assert.equal(file(3, "cmd/login/main.go"), null);
assert.ok(file(4, "cmd/login/main.go"));
assert.equal(file(4, "client/embed.go"), null);
assert.ok(file(5, "client/embed.go"));
assert.doesNotMatch(file(5, "internal/world/world.go"), /func .*\) (move|chat|gather)\(/);
assert.match(file(6, "internal/world/world.go"), /func .*\) move\(/);
assert.doesNotMatch(file(6, "internal/world/world.go"), /func .*\) (chat|gather)\(/);
assert.match(file(7, "internal/world/world.go"), /func .*\) chat\(/);
assert.doesNotMatch(file(7, "internal/world/world.go"), /func .*\) gather\(/);
assert.match(file(8, "internal/world/world.go"), /func .*\) gather\(/);
assert.match(file(9, "client/web/app.js"), /function drawTree\(/);
console.log("成长历史核对通过：9 个真实阶段、独立空根与功能出现顺序一致。");
