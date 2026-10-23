// 文件说明：用合成报告和临时Git仓库验证防漏检判据，不将这些数据当成本仓库真实CI证明。
import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile, readFile, copyFile, rm } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { root, git, gitIdentity, repositoryKey } from "../../scripts/ci-git.mjs";
import { profile, digest, commandPlan, ownedProjects } from "../../scripts/ci-profile.mjs";
import {
  validateReport,
  recordProof,
  checkProof,
  invalidateProof,
  checkUpdates,
  recipeFor,
} from "../../scripts/push-proof.mjs";
import { installHooks } from "../../scripts/install-hooks.mjs";

function sample(identity, recipe, now = Date.now()) {
  const id = "a".repeat(32);
  return {
    version: 1,
    id,
    scope: "preflight",
    mode: "commit",
    status: "passed",
    ...identity,
    profile_hash: digest(JSON.stringify(profile)),
    recipe_hash: recipe,
    started_at: new Date(now - 10000).toISOString(),
    finished_at: new Date(now - 1000).toISOString(),
    jobs: ["source", "playable"].map((kind) => ({
      kind,
      status: "passed",
      cache_volumes: 0,
      elapsed_ms: 3000,
      duration_ms: 2000,
      prepared: {
        image: "game-shelf-ci-" + kind + ":" + recipe.slice(0, 16),
        image_id: "sha256:" + "b".repeat(64),
        duration_ms: 1000,
      },
      rounds: Array.from({ length: kind === "source" ? 1 : profile.rounds }, (_, i) => {
        const scope = kind === "playable" && i > 0 ? "browser-repeat" : "full";
        const runId = "verify-" + id + "-" + (i + 1);
        const projects = kind === "playable" ? ownedProjects(runId) : [];
        return {
          round: i + 1,
          status: "passed",
          error: undefined,
          cold_tool_cache: i === 0,
          duration_ms: 1000,
          duration_limit_ms: profile.maxBrowserMs,
          projects,
          cleanup: Object.fromEntries(
            projects.map((project) => [project, { containers: 0, volumes: 0, networks: 0 }]),
          ),
          environment: {
            kind,
            scope,
            status: "passed",
            run_id: runId,
            node: profile.node,
            go: profile.go,
            cgo: kind === "source" ? 1 : 0,
            cpus: profile.cpus,
            memory_bytes: profile.memoryBytes,
            swap_bytes: 0,
            steps: commandPlan(kind, scope).map(([file, args]) => ({
              command: [file, ...args],
              duration_ms: 10,
            })),
            browser_binary: { revision: "synthetic1248", sha256: "c".repeat(64) },
          },
          browser: {
            passed: 19,
            failed: 0,
            skipped: 0,
            retries: 0,
            slowest: { title: "合成判据", duration_ms: 20000 },
          },
        };
      }),
    })),
  };
}
async function fixture(run) {
  const dir = await mkdtemp(join(tmpdir(), "game-shelf-push-proof-"));
  try {
    git(["init", "--quiet", "--initial-branch=master", "--object-format=sha1", dir]);
    git(["config", "user.email", "fixture@example.invalid"], dir);
    git(["config", "user.name", "门禁单元回归"], dir);
    git(["config", "remote.origin.url", "https://github.com/test/fixture.git"], dir);
    for (const path of ["deploy", "configs", ".githooks"]) await mkdir(join(dir, path));
    for (const file of [
      "deploy/ci-tools.Dockerfile",
      "package.json",
      "package-lock.json",
      "configs/ci-profile.json",
    ])
      await copyFile(join(root, file), join(dir, file));
    await writeFile(join(dir, ".gitignore"), ".local/\n");
    await writeFile(join(dir, ".githooks/pre-push"), "#!/bin/sh\nexit 0\n");
    git(["add", "."], dir);
    git(["update-index", "--chmod=+x", ".githooks/pre-push"], dir);
    git(["commit", "--quiet", "-m", "fixture"], dir);
    const make = async () => {
      const report = sample(gitIdentity(false, dir), await recipeFor(dir));
      const path = join(dir, ".local/logs/ci-verify-" + report.id + "/report.json");
      await mkdir(join(path, ".."), { recursive: true });
      await writeFile(path, JSON.stringify(report, null, 2) + "\n");
      return { report, path, proof: await recordProof(report, path, dir) };
    };
    await run(dir, make);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}

test("仓库身份兼容标准HTTPS/SSH且不保留URL凭据", () => {
  const expected = "github.com/test/fixture";
  for (const value of [
    "https://github.com/Test/Fixture.git",
    "git@github.com:test/fixture.git",
    "ssh://git@github.com/test/fixture.git",
    "https://name:secret@github.com/test/fixture.git",
  ])
    assert.equal(repositoryKey(value), expected);
  assert.throws(() => repositoryKey("https://example.invalid/test/fixture.git"));
});

test("合成判据拒绝候选/部分job/缺race/错误资源/临界耗时/非零残留", () => {
  const identity = { mode: "commit", commit: "1".repeat(40), tree: "2".repeat(40) },
    recipe = "3".repeat(64),
    now = Date.now();
  assert.doesNotThrow(() => validateReport(sample(identity, recipe, now), identity, recipe, now));
  for (const mutate of [
    (r) => {
      r.mode = "candidate";
    },
    (r) => {
      r.scope = "job";
    },
    (r) => {
      r.jobs.pop();
    },
    (r) => {
      r.jobs[0].rounds[0].environment.steps.splice(3, 1);
    },
    (r) => {
      r.jobs[0].rounds[0].environment.cgo = 0;
    },
    (r) => {
      r.jobs[1].rounds[0].environment.cpus = 4;
    },
    (r) => {
      r.jobs[1].rounds[0].environment.swap_bytes = 1024;
    },
    (r) => {
      r.jobs[1].rounds[1].browser.slowest.duration_ms = 27001;
    },
    (r) => {
      r.jobs[1].rounds[1].browser.skipped = 1;
    },
    (r) => {
      r.jobs[1].rounds[1].browser.retries = 1;
    },
    (r) => {
      r.jobs[1].rounds[1].environment.browser_binary.sha256 = "d".repeat(64);
    },
    (r) => {
      r.jobs[0].cache_volumes = 1;
    },
    (r) => {
      Object.values(r.jobs[1].rounds[0].cleanup)[0].volumes = 1;
    },
    (r) => {
      r.finished_at = new Date(now + 60000).toISOString();
    },
    (r) => {
      r.started_at = new Date(now - profile.receiptMaxAgeMs - 20000).toISOString();
      r.finished_at = new Date(now - profile.receiptMaxAgeMs - 1000).toISOString();
    },
  ]) {
    const report = sample(identity, recipe, now);
    mutate(report);
    assert.throws(() => validateReport(report, identity, recipe, now));
  }
});

test("记录绑定报告字节且运行中/失败尝试会阻止旧结果放行", async () =>
  fixture(async (dir, make) => {
    const { path } = await make();
    assert.equal((await checkProof(dir)).mode, "commit");
    await writeFile(join(dir, ".local/ci-verification.lock"), "fixture\n");
    await assert.rejects(checkProof(dir), /验收仍在运行/);
    await rm(join(dir, ".local/ci-verification.lock"));
    await writeFile(path, (await readFile(path, "utf8")) + " ");
    await assert.rejects(checkProof(dir), /报告字节/);
    await make();
    await invalidateProof(dir);
    await assert.rejects(checkProof(dir), /缺少最新完整验收/);
  }));

test("新commit/脏tree/旧SHA/删除/异仓库/非快进都不能推送", async () =>
  fixture(async (dir, make) => {
    const first = await make();
    await writeFile(join(dir, "change.txt"), "new\n");
    await assert.rejects(checkProof(dir), /未跟踪/);
    git(["add", "change.txt"], dir);
    git(["commit", "--quiet", "-m", "next fixture"], dir);
    await assert.rejects(checkProof(dir));
    const current = await make(),
      zero = "0".repeat(40),
      url = "git@github.com:test/fixture.git";
    const line = (local, ref = "refs/heads/master", remote = first.proof.commit) =>
      "refs/heads/master " + local + " " + ref + " " + remote + "\n";
    assert.doesNotThrow(() => checkUpdates(line(current.proof.commit), current.proof, url, dir));
    assert.doesNotThrow(() =>
      checkUpdates(
        line(current.proof.commit, "refs/heads/feat/example", zero),
        current.proof,
        url,
        dir,
      ),
    );
    for (const value of [
      line(first.proof.commit),
      line(zero),
      line(current.proof.commit, "refs/tags/v1"),
      line(current.proof.commit, "refs/heads/main"),
      line(current.proof.commit, "refs/heads/no-ci"),
      line(current.proof.commit, "refs/heads/master", "f".repeat(40)),
    ])
      assert.throws(() => checkUpdates(value, current.proof, url, dir));
    assert.throws(() =>
      checkUpdates(
        line(first.proof.commit, "refs/heads/master", current.proof.commit),
        first.proof,
        url,
        dir,
      ),
    );
    assert.throws(() =>
      checkUpdates(
        line(current.proof.commit),
        current.proof,
        "https://github.com/other/repo.git",
        dir,
      ),
    );
  }));

test("hook安装幂等且保留已有自定义hooksPath/活动pre-push", async () =>
  fixture(async (dir) => {
    git(["config", "--local", "core.hooksPath", ".existing-hooks"], dir);
    assert.throws(() => installHooks(dir), /拒绝覆盖/);
    assert.equal(git(["config", "--local", "--get", "core.hooksPath"], dir), ".existing-hooks");
    git(["config", "--local", "--unset", "core.hooksPath"], dir);
    await writeFile(join(dir, ".git/hooks/pre-push"), "#!/bin/sh\nexit 0\n");
    assert.throws(() => installHooks(dir), /已有活动pre-push/);
    await rm(join(dir, ".git/hooks/pre-push"));
    installHooks(dir);
    installHooks(dir);
    assert.equal(git(["config", "--local", "--get", "core.hooksPath"], dir), ".githooks");
  }));
