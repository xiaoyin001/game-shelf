// 文件说明：防止漏跑/旧结果误用的提交绑定记录，不替代人工Push授权或第三方签名。
import assert from "node:assert/strict";
import { readFile, writeFile, mkdir, rename, rm, access } from "node:fs/promises";
import { join, relative } from "node:path";
import { randomUUID } from "node:crypto";
import { profile, digest, toolRecipeHash, commandPlan, ownedProjects } from "./ci-profile.mjs";
import { root, git, gitIdentity, repositoryKey } from "./ci-git.mjs";
const filename = ".local/push-verification.json";
const finite = (value) => Number.isFinite(value) && value >= 0;
export async function invalidateProof(repository = root) {
  await mkdir(join(repository, ".local"), { recursive: true });
  await writeFile(
    join(repository, filename),
    JSON.stringify({ version: 1, status: "invalidated", attempted_at: new Date().toISOString() }) +
      "\n",
  );
}
export async function recipeFor(repository = root) {
  const files = await Promise.all(
    ["deploy/ci-tools.Dockerfile", "package.json", "package-lock.json"].map((name) =>
      readFile(join(repository, name), "utf8"),
    ),
  );
  return toolRecipeHash(...files);
}
export function validateReport(report, identity, recipeHash, now = Date.now()) {
  assert.equal(report.version, 1);
  assert.equal(report.status, "passed", "完整验收没有通过");
  assert.equal(report.scope, "preflight", "单job/准备结果不能作为推送证据");
  assert.equal(report.mode, "commit", "候选tree不是已验收提交");
  assert.equal(report.commit, identity.commit, "报告不是当前提交");
  assert.equal(report.tree, identity.tree, "报告不是当前源码tree");
  assert.match(report.id, /^[a-f0-9]{32}$/);
  assert.equal(report.profile_hash, digest(JSON.stringify(profile)), "验收配置已改变");
  assert.equal(report.recipe_hash, recipeHash, "验收工具配方已改变");
  const finished = Date.parse(report.finished_at),
    started = Date.parse(report.started_at);
  assert.ok(
    Number.isFinite(finished) && Number.isFinite(started) && finished >= started,
    "验收时间无效",
  );
  assert.ok(
    finished <= now + 5000 && now - finished <= profile.receiptMaxAgeMs,
    "验收记录超出24小时窗口或来自未来",
  );
  assert.equal(report.jobs.length, 2, "缺少完整source/playable");
  assert.deepEqual(report.jobs.map((job) => job.kind).sort(), ["playable", "source"]);
  for (const job of report.jobs) {
    assert.equal(job.status, "passed");
    assert.equal(job.cache_volumes, 0, "job缓存未回收");
    assert.ok(finite(job.elapsed_ms) && job.elapsed_ms <= profile.jobMs);
    assert.ok(
      finite(job.duration_ms) && job.duration_ms <= profile.runMs - profile.cleanupReserveMs,
    );
    assert.equal(job.prepared.image, "game-shelf-ci-" + job.kind + ":" + recipeHash.slice(0, 16));
    assert.match(job.prepared.image_id, /^sha256:[a-f0-9]{64}$/);
    assert.ok(finite(job.prepared.duration_ms) && job.prepared.duration_ms <= profile.prepareMs);
    assert.equal(job.rounds.length, job.kind === "source" ? 1 : profile.rounds);
    for (const [i, round] of job.rounds.entries()) {
      const scope = job.kind === "playable" && i > 0 ? "browser-repeat" : "full";
      const id = "verify-" + report.id + "-" + (i + 1);
      const env = round.environment;
      assert.equal(round.status, "passed");
      assert.equal(round.round, i + 1);
      assert.equal(round.cold_tool_cache, i === 0);
      assert.ok(
        finite(round.duration_ms) && round.duration_ms <= profile.runMs - profile.cleanupReserveMs,
      );
      assert.equal(env.status, "passed");
      assert.equal(env.kind, job.kind);
      assert.equal(env.run_id, id);
      assert.equal(env.scope, scope);
      assert.equal(env.node, profile.node);
      assert.equal(env.go, profile.go);
      assert.equal(env.cgo, job.kind === "source" ? 1 : 0);
      assert.equal(env.cpus, profile.cpus);
      assert.equal(env.memory_bytes, profile.memoryBytes);
      assert.equal(env.swap_bytes, 0);
      assert.deepEqual(
        env.steps.map((step) => step.command),
        commandPlan(job.kind, scope).map(([file, args]) => [file, ...args]),
        "缺少实际完整命令计划",
      );
      const projects = job.kind === "playable" ? ownedProjects(id) : [];
      assert.deepEqual(round.projects, projects);
      assert.deepEqual(Object.keys(round.cleanup).sort(), [...projects].sort());
      for (const value of Object.values(round.cleanup))
        assert.deepEqual(value, { containers: 0, volumes: 0, networks: 0 }, "本轮依赖未回收");
      if (job.kind === "playable") {
        assert.equal(round.duration_limit_ms, profile.maxBrowserMs, "缺少本地3秒余量门槛");
        assert.ok(round.browser.passed >= profile.minBrowserTests);
        assert.equal(round.browser.failed, 0);
        assert.equal(round.browser.skipped, 0);
        assert.equal(round.browser.retries, 0);
        assert.ok(
          finite(round.browser.slowest.duration_ms) &&
            round.browser.slowest.duration_ms <= profile.maxBrowserMs,
        );
        assert.match(env.browser_binary.sha256, /^[a-f0-9]{64}$/);
      }
    }
  }
  const browsers = report.jobs
    .find((job) => job.kind === "playable")
    .rounds.map((round) => round.environment.browser_binary);
  assert.ok(
    browsers.every(
      (binary) => binary.sha256 === browsers[0].sha256 && binary.revision === browsers[0].revision,
    ),
    "浏览器字节/版本发生漂移",
  );
}
export async function recordProof(report, reportPath, repository = root) {
  const identity = gitIdentity(false, repository);
  validateReport(report, identity, await recipeFor(repository));
  const path = relative(repository, reportPath).replaceAll("\\", "/");
  assert.equal(path, ".local/logs/ci-verify-" + report.id + "/report.json");
  const contents = await readFile(reportPath);
  assert.deepEqual(
    JSON.parse(contents),
    JSON.parse(JSON.stringify(report)),
    "磁盘报告与本次结果不同",
  );
  const proof = {
    version: 1,
    status: "passed",
    repository: repositoryKey(git(["remote", "get-url", "origin"], repository)),
    ...identity,
    report: path,
    report_sha256: digest(contents),
    verified_at: report.finished_at,
  };
  const target = join(repository, filename),
    temp = target + "." + randomUUID();
  try {
    await writeFile(temp, JSON.stringify(proof, null, 2) + "\n");
    await rename(temp, target);
  } finally {
    await rm(temp, { force: true });
  }
  return proof;
}
export async function checkProof(repository = root, now = Date.now()) {
  try {
    await access(join(repository, ".local/ci-verification.lock"));
    throw new Error("验收仍在运行或锁未确认，拒绝推送");
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  const identity = gitIdentity(false, repository);
  const proof = JSON.parse(await readFile(join(repository, filename), "utf8"));
  assert.equal(proof.version, 1);
  assert.equal(proof.status, "passed", "缺少最新完整验收，请运行npm run verify:ci");
  assert.equal(proof.mode, "commit");
  assert.equal(proof.commit, identity.commit);
  assert.equal(proof.tree, identity.tree);
  assert.equal(
    proof.repository,
    repositoryKey(git(["remote", "get-url", "origin"], repository)),
    "记录来自其他仓库",
  );
  assert.match(proof.report, /^\.local\/logs\/ci-verify-[a-f0-9]{32}\/report\.json$/);
  const contents = await readFile(join(repository, proof.report));
  assert.equal(digest(contents), proof.report_sha256, "报告字节已改变");
  const report = JSON.parse(contents);
  assert.equal(proof.report, ".local/logs/ci-verify-" + report.id + "/report.json");
  assert.equal(proof.verified_at, report.finished_at);
  validateReport(report, identity, await recipeFor(repository), now);
  return proof;
}
export function checkUpdates(text, proof, remoteUrl, repository = root) {
  assert.equal(repositoryKey(remoteUrl), proof.repository, "目标仓库不是已验收仓库");
  const lines = text.trim().split(/\r?\n/).filter(Boolean);
  for (const line of lines) {
    const fields = line.split(/\s+/);
    assert.equal(fields.length, 4, "pre-push更新记录无效");
    const [, local, ref, remote] = fields;
    assert.match(local, /^[a-f0-9]{40}$/);
    assert.match(remote, /^[a-f0-9]{40}$/);
    assert.equal(local, proof.commit, "只允许推送当前已验收HEAD，不接受删除/旧SHA/标签");
    const branch = ref.startsWith("refs/heads/") ? ref.slice(11) : "";
    assert.ok(
      profile.pushBranches.some((pattern) =>
        pattern.endsWith("/**")
          ? branch.startsWith(pattern.slice(0, -2)) && branch.length > pattern.length - 2
          : branch === pattern,
      ),
      "目标分支不在CI覆盖范围",
    );
    if (!/^0{40}$/.test(remote))
      assert.equal(
        git(["merge-base", "--is-ancestor", remote, local], repository, true).status,
        0,
        "远端祖先未确认或不是快进，请先fetch核对；门禁不授权强推",
      );
  }
}
