// 文件说明：验证等待只针对已拥有的子进程，超时/已退出均取消监听，不留fallback计时器拖延运行器。
import test from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { spawnSync } from "node:child_process";
import { waitForExit } from "../../scripts/process-exit.mjs";

test("子进程超时与已退出都不残留退出监听", async () => {
  const child = new EventEmitter();
  child.exitCode = child.signalCode = null;
  assert.equal(await waitForExit(child, 10), false);
  assert.equal(child.listenerCount("exit"), 0);
  child.exitCode = 0;
  assert.equal(await waitForExit(child, 3000), true);
  assert.equal(child.listenerCount("exit"), 0);
});

test("真实快速退出不被已无用的三秒fallback拖住", () => {
  const url = new URL("../../scripts/process-exit.mjs", import.meta.url).href;
  const code = `import {spawn} from 'node:child_process';import {waitForExit} from ${JSON.stringify(url)};const child=spawn(process.execPath,['-e','']);if(!await waitForExit(child,3000))process.exitCode=1;`;
  const result = spawnSync(process.execPath, ["--input-type=module", "-e", code], {
    encoding: "utf8",
    timeout: 1800,
  });
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
});
