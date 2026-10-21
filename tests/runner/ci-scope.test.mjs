// 文件说明：验证 CI 必须使用独立 project，且普通开发或未创建资源时不能清理卷。
import test from "node:test";
import assert from "node:assert/strict";
import { createInfraSpec, cleanupArgs } from "../../scripts/infra.mjs";
test("CI 使用固定配置覆盖和独立 project", () => {
  const ci = createInfraSpec(true, "test-one"),
    other = createInfraSpec(true, "test-two");
  assert.notEqual(ci.project, other.project);
  assert.deepEqual(ci.args, [
    "compose",
    "-p",
    "game-shelf-ci-test-one",
    "-f",
    "deploy/docker-compose.yml",
    "-f",
    "deploy/docker-compose.ci.yml",
  ]);
});
test("普通开发和未拥有资源不能清理", () => {
  assert.equal(cleanupArgs(createInfraSpec(false, "test"), true), null);
  assert.equal(cleanupArgs(createInfraSpec(true, "test"), false), null);
  assert.throws(() => cleanupArgs({ project: "game-shelf", args: [], disposable: true }, true));
});
test("清理只指向本次 CI project，拒绝外部命令注入", () => {
  const ci = createInfraSpec(true, "test-one");
  assert.deepEqual(cleanupArgs(ci, true), [
    ...ci.args,
    "down",
    "--volumes",
    "--remove-orphans",
    "--timeout",
    "10",
  ]);
  for (const id of ["../shared", "test;rm", "OTHER", "", "test one"])
    assert.throws(() => createInfraSpec(true, id));
});
