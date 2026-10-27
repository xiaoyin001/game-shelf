// 文件说明：确保全部真实服务回归和180秒等待名额保留，不以改生产时长或浏览器并发节省预算。
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
  serviceTestArgs,
  longServiceTests,
  longServicePattern,
} from "../../scripts/service-tests.mjs";
test("服务入口给三个真实长回合同时等待的名额，完整协议/依赖均保留", () => {
  assert.deepEqual(serviceTestArgs(), [
    "test",
    "-count=1",
    "-v",
    "-parallel",
    "3",
    "./tests/integration",
  ]);
  assert.deepEqual(serviceTestArgs(true), [
    "test",
    "-count=1",
    "-v",
    "-parallel",
    "3",
    "./tests/infrastructure",
    "./tests/integration",
  ]);
  assert.equal(longServiceTests.length, 3);
  for (const name of longServiceTests) assert.match(name, new RegExp(longServicePattern));
  assert.ok(!new RegExp(longServicePattern).test("TestLiveCoopReconnectAndExplicitQuit"));
  assert.deepEqual(serviceTestArgs(true, "short"), [
    "test",
    "-count=1",
    "-v",
    "-parallel",
    "3",
    "-skip",
    longServicePattern,
    "./tests/infrastructure",
    "./tests/integration",
  ]);
  assert.deepEqual(serviceTestArgs(false, "long"), [
    "test",
    "-count=1",
    "-v",
    "-parallel",
    "3",
    "-run",
    longServicePattern,
    "./tests/integration",
  ]);
  assert.throws(() => serviceTestArgs(false, "missing"));
  const verify = readFileSync(new URL("../../scripts/verify.mjs", import.meta.url), "utf8");
  assert.match(verify, /await long.ready/);
  assert.match(verify, /Promise.all\(\[long\?\.done, browser.done\]\)/);
  for (const path of ["verify.mjs", "test-services.mjs"])
    assert.match(
      readFileSync(new URL("../../scripts/" + path, import.meta.url), "utf8"),
      /serviceTestArgs\(/,
    );
});
