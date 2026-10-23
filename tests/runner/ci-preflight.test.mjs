// 文件说明：回归共用环境的资源、范围、快照与结果判据；合成报告不充当真实联机证据。
import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import { createServer } from "node:net";
import {
  profile,
  toolRecipeHash,
  verificationPolicy,
  runnerIdentity,
  cpuList,
  checkLimits,
  browserResult,
  ownedProjects,
} from "../../scripts/ci-profile.mjs";
import {
  toolArgs,
  cacheInitArgs,
  guardPorts,
  gitIdentity,
  createSnapshot,
  execute,
} from "../../scripts/ci-verify.mjs";

test("本地推送比CI单job增加浏览器复验与余量，不削减远端原回归", () => {
  assert.deepEqual(verificationPolicy(["source", "playable"]), {
    scope: "preflight",
    rounds: 2,
    maxBrowserMs: 27000,
  });
  assert.deepEqual(verificationPolicy(["playable"]), {
    scope: "job",
    rounds: 1,
    maxBrowserMs: 30000,
  });
  assert.equal(verificationPolicy(["source", "playable"], true).scope, "prepare");
});

test("资源读取拒绝不限额、swap及不足的真实配额", () => {
  assert.deepEqual(cpuList("0-1,4,6-7"), [0, 1, 4, 6, 7]);
  for (const text of ["", "2-0", "1;echo bad"]) assert.throws(() => cpuList(text));
  assert.doesNotThrow(() => checkLimits("200000 100000", String(profile.memoryBytes), "0"));
  assert.throws(() => checkLimits("max 100000", String(profile.memoryBytes), "0"));
  assert.throws(() => checkLimits("400000 100000", String(profile.memoryBytes), "0"));
  assert.throws(() => checkLimits("200000 100000", "max", "0"));
  assert.throws(() => checkLimits("200000 100000", String(profile.memoryBytes), "1024"));
});

test("工具配方包含package与锁文件内容，不能复用旧依赖图像", () => {
  const hash = toolRecipeHash("recipe", "package", "lock");
  assert.notEqual(toolRecipeHash("changed", "package", "lock"), hash);
  assert.notEqual(toolRecipeHash("recipe", "changed", "lock"), hash);
  assert.notEqual(toolRecipeHash("recipe", "package", "changed"), hash);
});

test("宿主身份不写死，缓存初始化仅接触私有卷", () => {
  assert.deepEqual(runnerIdentity("1001", "121", "108"), { uid: 1001, gid: 121, socket_gid: 108 });
  for (const value of ["", "bad", "-1", "1.5"]) assert.throws(() => runnerIdentity(value, 1, 1));
  const args = cacheInitArgs({
    name: "owned-init",
    cache: "owned-cache",
    id: "owned",
    image: "owned-image",
    runner: { uid: 1001, gid: 121 },
  });
  assert.equal(args[args.indexOf("--user") + 1], "0:0");
  assert.ok(args.includes("CHOWN") && args.includes("--read-only"));
  assert.equal(args.filter((arg) => arg === "--mount").length, 1);
  assert.ok(args.includes("type=volume,source=owned-cache,target=/ci-cache"));
  assert.ok(!args.some((arg) => arg.includes("target=/workspace")));
});

test("工具容器受同一限制且只有playable获得本机socket", () => {
  const id = "verify-" + "a".repeat(32) + "-1";
  for (const kind of ["source", "playable"]) {
    const args = toolArgs({
      name: "game-shelf-" + id + "-" + kind,
      image: "game-shelf-ci-" + kind + ":" + "b".repeat(16),
      workspace: "/owned/repo",
      id,
      kind,
      runner: { uid: 1001, gid: 1002, socket_gid: 108 },
    });
    assert.equal(args[args.indexOf("--user") + 1], "1001:1002");
    assert.equal(args.includes("--group-add"), kind === "playable");
    assert.ok(args.includes("HOME=/ci-cache/home"));
    assert.ok(
      args.some((arg) =>
        arg.endsWith("target=/workspace/node_modules,volume-subpath=node_modules"),
      ),
    );
    assert.equal(args[args.indexOf("--cpus") + 1], "2");
    assert.equal(args[args.indexOf("--memory") + 1], String(4 * 1024 ** 3));
    assert.equal(args[args.indexOf("--memory-swap") + 1], String(4 * 1024 ** 3));
    assert.equal(
      args.some((arg) => arg.includes("docker.sock")),
      kind === "playable",
    );
    assert.ok(!args.includes("--privileged"));
  }
  assert.throws(() => ownedProjects("game-shelf"));
  assert.throws(() => ownedProjects(id + ";rm"));
  assert.deepEqual(ownedProjects(id), ["game-shelf-ci-" + id, "game-shelf-ci-" + id + "-failure"]);
});

function sample() {
  return {
    stats: { expected: 19, unexpected: 0, flaky: 0, skipped: 0 },
    errors: [],
    suites: [
      {
        specs: Array.from({ length: 19 }, (_, i) => ({
          title: "合成判据" + i,
          tests: [{ results: [{ status: "passed", retry: 0, duration: 24000 }] }],
        })),
      },
    ],
  };
}
test("全绿但临近30秒、缺例、重试、跳过或报告错误均不合格", () => {
  assert.equal(browserResult(sample()).passed, 19);
  const ci = sample();
  ci.suites[0].specs[0].tests[0].results[0].duration = 29000;
  assert.throws(() => browserResult(ci));
  assert.equal(browserResult(ci, profile.browserTimeoutMs).passed, 19);
  for (const mutate of [
    (r) => {
      r.suites[0].specs[0].tests[0].results[0].duration = 27001;
    },
    (r) => {
      r.stats.expected = 18;
    },
    (r) => {
      r.stats.flaky = 1;
    },
    (r) => {
      r.stats.skipped = 1;
    },
    (r) => {
      r.errors.push({ message: "global error" });
    },
    (r) => {
      r.suites[0].specs[0].tests[0].results[0].retry = 1;
    },
    (r) => {
      r.suites[0].specs[0].tests[0].results.push({ status: "passed" });
    },
    (r) => {
      r.suites[0].specs[0].tests[0].results[0].status = "skipped";
    },
  ]) {
    const r = sample();
    mutate(r);
    assert.throws(() => browserResult(r));
  }
});

test("候选快照只含暂存tree，不复制仓库凭据或漏掉删除", async () => {
  const dir = await mkdtemp(join(tmpdir(), "game-shelf-ci-snapshot-"));
  const source = join(dir, "source"),
    target = join(dir, "snapshot");
  const git = (args, cwd = source) => {
    const r = spawnSync("git", args, { cwd, encoding: "utf8", windowsHide: true });
    assert.equal(r.status, 0, r.stderr);
    return r.stdout.trim();
  };
  try {
    const init = spawnSync("git", ["init", "--quiet", "--initial-branch=master", source]);
    assert.equal(init.status, 0);
    git(["config", "user.email", "fixture@example.invalid"]);
    git(["config", "user.name", "本地快照回归"]);
    git(["config", "http.https://github.com/.extraheader", "CANARY-NOT-A-CREDENTIAL"]);
    await writeFile(join(source, "old.txt"), "old\n");
    git(["add", "."]);
    git(["commit", "--quiet", "-m", "fixture"]);
    assert.equal(gitIdentity(false, source).mode, "commit");
    git(["rm", "--quiet", "old.txt"]);
    await writeFile(join(source, "new.txt"), "candidate\n");
    git(["add", "new.txt"]);
    const identity = gitIdentity(true, source);
    assert.throws(() => gitIdentity(false, source), /干净提交/);
    createSnapshot(source, target, identity);
    assert.equal(await readFile(join(target, "new.txt"), "utf8"), "candidate\n");
    await assert.rejects(readFile(join(target, "old.txt")), { code: "ENOENT" });
    assert.ok(!(await readFile(join(target, ".git/config"), "utf8")).includes("CANARY"));
    assert.equal(git(["write-tree"], target), identity.tree);
    await writeFile(join(source, "new.txt"), "unstaged\n");
    assert.throws(() => gitIdentity(true, source), /未暂存/);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});

test("主机端口占用拒绝验证且原监听继续存活", async () => {
  const server = createServer((socket) => socket.end());
  await new Promise((done) => server.listen(0, "127.0.0.1", done));
  try {
    await assert.rejects(guardPorts([server.address().port]), /主机端口占用/);
    assert.ok(server.listening);
  } finally {
    await new Promise((done) => server.close(done));
  }
});

test("命令超时或主动中断返回失败，不留下自己的子进程", async () => {
  await assert.rejects(
    execute(process.execPath, ["-e", "setInterval(()=>{},1000)"], { timeout: 100 }),
    /超时/,
  );
  const controller = new AbortController();
  setTimeout(() => controller.abort(), 100);
  await assert.rejects(
    execute(process.execPath, ["-e", "setInterval(()=>{},1000)"], {
      timeout: 3000,
      signal: controller.signal,
    }),
    /中断/,
  );
});
