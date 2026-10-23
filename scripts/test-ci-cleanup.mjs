// 文件说明：在真实独立依赖已健康后注入检查工具失败，验证原错误、现场与定向清理均保留。
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtemp, readFile, writeFile, copyFile, chmod, stat, rm, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { root } from "./infra.mjs";
if (process.platform !== "linux") throw new Error("CI 失败清理回归仅在 Linux 执行");
const go = spawnSync("go", ["env", "GOROOT"], { encoding: "utf8" });
if (go.status !== 0) throw new Error(go.stderr);
await mkdir(join(root, ".local"), { recursive: true });
const temp = await mkdtemp(join(root, ".local/ci-cleanup-test-"));
const checker = join(root, ".local/bin/infra-check");
const snapshotFiles = ["infra.log", "ci-cleanup.json"];
const snapshots = new Map();
for (const name of snapshotFiles) {
  try {
    snapshots.set(name, await readFile(join(root, ".local/logs", name)));
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
let original = null;
try {
  original = await stat(checker);
  await copyFile(checker, join(temp, "original-checker"));
} catch (error) {
  if (error.code !== "ENOENT") throw error;
}
try {
  const realGo = join(go.stdout.trim(), "bin/go");
  const wrapper = `#!/usr/bin/env node
import{writeFileSync,chmodSync}from"node:fs";
import{spawnSync}from"node:child_process";
const args=process.argv.slice(2);
if(args[0]==="build"&&args.at(-1)==="./cmd/infra-check"){
 const output=args[args.indexOf("-o")+1];
 writeFileSync(output,"#!/bin/sh\\necho GAME_SHELF_EXPECTED_READINESS_FAILURE >&2\\nexit 23\\n");
 chmodSync(output,0o755);process.exit(0);
}
const r=spawnSync(${JSON.stringify(realGo)},args,{stdio:"inherit"});
process.exit(r.status??1);
`;
  await writeFile(join(temp, "go"), wrapper);
  await chmod(join(temp, "go"), 0o755);
  const start = Date.now();
  const result = spawnSync(
    process.execPath,
    [
      "--input-type=module",
      "-e",
      'import {startInfra} from "./scripts/infra.mjs"; const stop=await startInfra(); await stop();',
    ],
    {
      cwd: root,
      encoding: "utf8",
      timeout: 240000,
      maxBuffer: 8 * 1024 * 1024,
      env: {
        ...process.env,
        GAME_SHELF_CI: "1",
        ...(process.env.GAME_SHELF_CI_ID
          ? { GAME_SHELF_CI_ID: process.env.GAME_SHELF_CI_ID + "-failure" }
          : {}),
        PATH: temp + ":" + process.env.PATH,
      },
    },
  );
  const output = (result.stdout || "") + (result.stderr || "");
  await writeFile(
    join(root, ".local/logs/ci-failure-runner.log"),
    "started_at=" +
      new Date(start).toISOString() +
      "\nexit=" +
      result.status +
      "\nerror=" +
      (result.error || "") +
      "\n" +
      output,
  );
  // 先保存当轮原始证据，即使后面的回归断言失败，也不能用上轮成功现场覆盖它。
  for (const [name, target] of [
    ["infra.log", "ci-failure-infra.log"],
    ["ci-cleanup.json", "ci-failure-cleanup.json"],
  ]) {
    const source = join(root, ".local/logs", name);
    let fresh = false;
    try {
      fresh = (await stat(source)).mtimeMs >= start;
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    if (fresh) await copyFile(source, join(root, ".local/logs", target));
    else
      await writeFile(
        join(root, ".local/logs", target),
        JSON.stringify({
          available: false,
          test_started_at: new Date(start).toISOString(),
          reason: "当轮未产生新现场，不能使用上轮结果",
        }) + "\n",
      );
  }
  if (result.error) throw result.error;
  assert.notEqual(result.status, 0, "故障检查器被误当成成功");
  assert.match(output, /GAME_SHELF_EXPECTED_READINESS_FAILURE/, "未执行预期的故障注入");
  assert.match(output, /infra-check exited 23/, "原始失败被清理错误遮盖");
  const cleanupPath = join(root, ".local/logs/ci-cleanup.json");
  assert.ok((await stat(cleanupPath)).mtimeMs >= start, "使用了上一轮的清理结果");
  const cleanup = JSON.parse(await readFile(cleanupPath, "utf8"));
  assert.match(cleanup.project, /^game-shelf-ci-/);
  assert.equal(cleanup.containers, 0);
  assert.equal(cleanup.volumes, 0);
  assert.equal(cleanup.networks, 0);
  const infra = await readFile(join(root, ".local/logs/infra.log"), "utf8");
  assert.ok(infra.includes(cleanup.project));
  assert.match(infra, /OOMKilled/);
  assert.match(infra, /RepoDigests/);
  assert.match(infra, /mongo@sha256:[a-f0-9]{64}/, "没有保存真实 MongoDB 镜像 digest");
  assert.match(infra, /\"OOMKilled\":false/, "没有保存容器真实退出/OOM 状态");
  await writeFile(
    join(root, ".local/logs/ci-failure-cleanup.json"),
    JSON.stringify(
      {
        ...cleanup,
        expected_failure: "readiness checker exit 23",
        verified_at: new Date().toISOString(),
      },
      null,
      2,
    ) + "\n",
  );
  console.log("CI 失败路径通过：预期错误保留，容器现场保存，本轮容器/卷/网络均为 0。");
} finally {
  if (original) {
    await copyFile(join(temp, "original-checker"), checker);
    await chmod(checker, original.mode);
  } else await rm(checker, { force: true });
  for (const [name, contents] of snapshots)
    await writeFile(join(root, ".local/logs", name), contents);
  await rm(temp, { recursive: true, force: true });
}
