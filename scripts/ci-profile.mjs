// 文件说明：本地与云端共用资源/期限和真实结果判据，不把主机规格误当容器有效限制。
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
export const profile = JSON.parse(
  readFileSync(new URL("../configs/ci-profile.json", import.meta.url)),
);
export const digest = (value) => createHash("sha256").update(value).digest("hex");
export function toolRecipeHash(recipe, packageJson, packageLock) {
  return digest([recipe, packageJson, packageLock].join("\0"));
}
export function verificationPolicy(jobs, prepareOnly = false) {
  const preflight = jobs.length === 2 && !prepareOnly;
  return {
    scope: prepareOnly ? "prepare" : preflight ? "preflight" : "job",
    rounds: preflight ? profile.rounds : profile.ciRounds,
    maxBrowserMs: preflight ? profile.maxBrowserMs : profile.browserTimeoutMs,
  };
}
export function commandPlan(kind, scope = "full") {
  assert.ok(["source", "playable"].includes(kind));
  assert.ok(["full", "browser-repeat"].includes(scope));
  const install = ["npm", ["ci", "--no-audit", "--no-fund"]];
  if (kind === "source") {
    assert.equal(scope, "full");
    return [
      install,
      ["npm", ["run", "check"]],
      ["npm", ["run", "check:client"]],
      ["go", ["test", "-race", "-count=1", "./..."]],
      ["git", ["diff", "--exit-code"]],
    ];
  }
  return [install, ["npm", ["run", scope === "full" ? "test:ci" : "test:e2e"]]];
}
export function cpuList(text) {
  const cpus = [];
  for (const part of text.trim().split(",")) {
    if (!/^\d+(?:-\d+)?$/.test(part)) throw new Error("无法读取CPU亲和范围");
    const [start, end = start] = part.split("-").map(Number);
    if (end < start || end - start > 65536) throw new Error("CPU范围无效");
    for (let i = start; i <= end; i++) cpus.push(i);
  }
  return cpus;
}
export function checkLimits(cpuMax, memoryMax, memorySwapMax) {
  const [quota, period] = cpuMax.trim().split(/\s+/).map(Number);
  assert.equal(quota / period, profile.cpus, "没有实际限制到配置的CPU额度");
  assert.equal(Number(memoryMax.trim()), profile.memoryBytes, "没有实际限制到配置的内存");
  assert.equal(Number(memorySwapMax.trim()), 0, "不允许依赖额外swap通过");
}
export function browserResult(report, maxDuration = profile.maxBrowserMs) {
  assert.ok(maxDuration > 0 && maxDuration <= profile.browserTimeoutMs);
  const stats = report.stats;
  assert.ok(stats?.expected >= profile.minBrowserTests, "浏览器用例缺失");
  for (const name of ["unexpected", "flaky", "skipped"])
    assert.equal(stats[name], 0, "浏览器存在失败/重试/跳过：" + name);
  assert.equal(report.errors?.length ?? 0, 0, "浏览器报告存在全局错误");
  const tests = [];
  const visit = (suites) => {
    for (const suite of suites ?? []) {
      for (const spec of suite.specs ?? []) {
        for (const test of spec.tests ?? []) {
          assert.equal(test.results.length, 1, "不接受重复尝试后择优");
          const result = test.results[0];
          assert.equal(result.status, "passed", "用例未实际通过：" + spec.title);
          assert.equal(result.retry, 0, "不接受重试");
          assert.ok(
            Number.isFinite(result.duration) &&
              result.duration >= 0 &&
              result.duration <= maxDuration,
            "用例超过本轮耗时预算：" + spec.title,
          );
          tests.push({ title: spec.title, duration_ms: result.duration });
        }
      }
      visit(suite.suites);
    }
  };
  visit(report.suites);
  assert.equal(tests.length, stats.expected, "报告计数不一致");
  return {
    passed: tests.length,
    failed: 0,
    skipped: 0,
    retries: 0,
    slowest: tests.sort((a, b) => b.duration_ms - a.duration_ms)[0],
  };
}
export function ownedProjects(id) {
  if (!/^verify-[a-f0-9]{32}-[12]$/.test(id)) throw new Error("拒绝未知资源scope");
  return ["game-shelf-ci-" + id, "game-shelf-ci-" + id + "-failure"];
}
