// 文件说明：在独立Linux卷里用普通UID1001父进程复跑真实job，保留基线EACCES；不改原仓库属主或共享数据。
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, readFile, writeFile, readdir, rm } from "node:fs/promises";
import { join } from "node:path";
import { randomUUID } from "node:crypto";
import { root, git, gitIdentity, createSnapshot } from "./ci-git.mjs";
import { execute } from "./ci-verify.mjs";
import { toolRecipeHash } from "./ci-profile.mjs";
import { taskNetwork } from "./ci-network.mjs";
const args = process.argv.slice(2);
const value = (key) => (args.includes(key) ? args[args.indexOf(key) + 1] : undefined);
assert.ok(
  args.every(
    (arg, i) =>
      ["--ref", "--jobs", "--candidate", "--expect-eacces"].includes(arg) ||
      ["--ref", "--jobs"].includes(args[i - 1]),
  ),
  "未知权限回归参数",
);
const jobs = (value("--jobs") || "source,playable").split(",");
assert.ok(jobs.length && jobs.every((job) => ["source", "playable"].includes(job)));
const ref = value("--ref");
if (ref) assert.match(ref, /^[a-f0-9]{7,40}$/);
const identity = ref
  ? { mode: "commit", commit: git(["rev-parse", ref]), tree: git(["rev-parse", ref + "^{tree}"]) }
  : gitIdentity(args.includes("--candidate"));
const id = randomUUID().replaceAll("-", "");
const volume = "game-shelf-ci-owner-" + id;
const label = "io.game-shelf.ci-owner=" + id;
const folder = join(root, ".local/logs/ci-owner-" + id);
await mkdir(folder, { recursive: true });
const temp = await mkdtemp(join(root, ".local/ci-owner-copy-"));
const copy = join(temp, "repo");
createSnapshot(root, copy, identity);
const file = process.platform === "win32" ? "wsl" : "docker";
const sync = (program, parameters) => {
  const result = spawnSync(program, parameters, {
    encoding: "utf8",
    windowsHide: true,
    timeout: 20000,
    maxBuffer: 8 * 1024 * 1024,
  });
  if (result.error || result.status !== 0)
    throw result.error || new Error("权限回归命令失败：" + result.stderr);
  return result.stdout.trim();
};
const prefix =
  process.platform === "win32"
    ? [
        "--exec",
        "sh",
        sync("wsl", ["--exec", "wslpath", "-a", "-u", join(root, "scripts/wsl-command.sh")]),
        "docker",
      ]
    : [];
const readPrefix =
  process.platform === "win32"
    ? ["--exec", "timeout", "--signal=TERM", "--kill-after=5s", "15s", "docker"]
    : [];
let network;
const env = () => {
  const result = { ...process.env, ...network?.env };
  if (process.platform === "win32")
    result.WSLENV = [result.WSLENV, ...Object.keys(network?.env ?? {}).map((key) => key + "/u")]
      .filter(Boolean)
      .join(":");
  return result;
};
const bridge = {
  read: (parameters) => sync(file, [...readPrefix, ...parameters]),
  run: (parameters, options) =>
    execute(file, [...prefix, ...parameters], { env: env(), ...options }),
  path: (path) =>
    process.platform === "win32" ? sync("wsl", ["--exec", "wslpath", "-a", "-u", path]) : path,
};
const hash = toolRecipeHash(
  ...(await Promise.all(
    ["deploy/ci-tools.Dockerfile", "package.json", "package-lock.json"].map((name) =>
      readFile(join(copy, name), "utf8"),
    ),
  )),
);
const image = "game-shelf-ci-playable:" + hash.slice(0, 16);
const report = {
  version: 1,
  id,
  ...identity,
  parent_uid: 1001,
  parent_gid: 1001,
  status: "running",
  started_at: new Date().toISOString(),
  jobs: [],
  retained_volume: false,
};
const names = [];
let created = false,
  failure;
async function reportsIn(dir) {
  const result = [];
  try {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const path = join(dir, entry.name);
      if (entry.isDirectory()) result.push(...(await reportsIn(path)));
      else if (entry.name === "report.json" && /ci-verify-[a-f0-9]{32}/.test(path))
        result.push(JSON.parse(await readFile(path, "utf8")));
    }
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  return result;
}
try {
  assert.equal(
    bridge.read(["volume", "ls", "-q", "--filter", "name=^" + volume + "$"]),
    "",
    "拒绝接管已有权限测试卷",
  );
  bridge.read(["image", "inspect", image]);
  network = await taskNetwork({ image, bridge, root });
  bridge.read(["volume", "create", "--label", label, volume]);
  created = true;
  const mount = bridge.read(["volume", "inspect", "--format", "{{.Mountpoint}}", volume]);
  assert.ok(
    mount.endsWith("/" + volume + "/_data") && !/[\r\n,]/.test(mount),
    "测试卷宿主路径无效",
  );
  const bootstrap = "game-shelf-owner-copy-" + id;
  names.push(bootstrap);
  const code =
    "const fs=require('fs');fs.cpSync('/in','/out/repo',{recursive:true,verbatimSymlinks:true});const own=p=>{const s=fs.lstatSync(p);if(s.isDirectory())for(const n of fs.readdirSync(p))own(p+'/'+n);fs.lchownSync(p,1001,1001)};own('/out')";
  await bridge.run(
    [
      "run",
      "--rm",
      "--init",
      "--name",
      bootstrap,
      "--label",
      label,
      "--network",
      "none",
      "--mount",
      "type=bind,source=" + bridge.path(copy) + ",target=/in,readonly",
      "--mount",
      "type=volume,source=" + volume + ",target=/out",
      image,
      "node",
      "-e",
      code,
    ],
    { timeout: 60000, log: join(folder, "copy.log") },
  );
  const socketGid =
    process.platform === "win32"
      ? sync("wsl", ["--exec", "stat", "-c", "%g", "/var/run/docker.sock"])
      : sync("stat", ["-c", "%g", "/var/run/docker.sock"]);
  assert.match(socketGid, /^\d+$/);
  for (const job of jobs) {
    const name = "game-shelf-owner-parent-" + id + "-" + job;
    names.push(name);
    const parameters = [
      "run",
      "--rm",
      "--init",
      "--name",
      name,
      "--label",
      label,
      "--user",
      "1001:1001",
      "--group-add",
      socketGid,
      "--network",
      "host",
      "--memory",
      "512m",
      "--memory-swap",
      "512m",
      "--pids-limit",
      "128",
      "--env",
      "HOME=/tmp",
      "--env",
      "GIT_CONFIG_COUNT=1",
      "--env",
      "GIT_CONFIG_KEY_0=safe.directory",
      "--env",
      "GIT_CONFIG_VALUE_0=" + mount + "/repo",
      "--mount",
      "type=volume,source=" + volume + ",target=" + mount,
      "--mount",
      "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
      "--workdir",
      mount + "/repo",
    ];
    for (const key of [
      "HTTP_PROXY",
      "HTTPS_PROXY",
      "http_proxy",
      "https_proxy",
      "NO_PROXY",
      "no_proxy",
    ])
      parameters.push("--env", key);
    parameters.push(image, "node", "scripts/ci-verify.mjs", "--job", job, "--run");
    if (identity.mode === "candidate") parameters.push("--candidate");
    let error;
    try {
      await bridge.run(parameters, { timeout: 1500000, log: join(folder, job + ".log") });
    } catch (value) {
      error = value;
    }
    const exporter = "game-shelf-owner-export-" + id + "-" + job;
    names.push(exporter);
    bridge.read([
      "create",
      "--name",
      exporter,
      "--label",
      label,
      "--mount",
      "type=volume,source=" + volume + ",target=/data,readonly",
      image,
    ]);
    const evidence = join(folder, job);
    await mkdir(evidence);
    await bridge.run(["cp", exporter + ":/data/repo/.local/logs", bridge.path(evidence)], {
      timeout: 60000,
    });
    bridge.read(["rm", exporter]);
    const actual = (await reportsIn(evidence)).filter((r) => r.jobs?.some((j) => j.kind === job));
    assert.ok(actual.length, "缺少普通父进程真实报告");
    const latest = actual.sort((a, b) => b.started_at.localeCompare(a.started_at))[0];
    if (args.includes("--expect-eacces")) {
      assert.ok(
        error && latest.status === "failed" && latest.error?.includes("EACCES"),
        "没有复现预期的普通父进程删除拒绝",
      );
      assert.ok(
        latest.jobs.some((j) => j.kind === job && j.status === "passed"),
        "业务/源码未通过，不能算属主故障复现",
      );
      report.jobs.push({ kind: job, status: "expected-eacces-reproduced", report_id: latest.id });
    } else {
      if (error) throw error;
      assert.equal(latest.status, "passed");
      assert.equal(latest.runner.uid, 1001);
      assert.equal(latest.runner.gid, 1001);
      report.jobs.push({ kind: job, status: "passed", report_id: latest.id });
    }
  }
  report.status = args.includes("--expect-eacces") ? "expected-eacces-reproduced" : "passed";
} catch (error) {
  failure = error;
  report.status = "failed";
  report.error = error.stack;
} finally {
  try {
    for (const name of names) {
      const present = bridge.read(["ps", "-aq", "--filter", "name=^/" + name + "$"]);
      if (!present) continue;
      assert.equal(
        bridge.read([
          "inspect",
          "--format",
          '{{index .Config.Labels "io.game-shelf.ci-owner"}}',
          present,
        ]),
        id,
        "拒绝清理非本轮父测试容器",
      );
      bridge.read(["stop", "--timeout", "10", present]);
      if (bridge.read(["ps", "-aq", "--filter", "name=^/" + name + "$"]))
        bridge.read(["rm", present]);
    }
    const inner = await reportsIn(folder);
    let safe = inner.length > 0;
    for (const item of inner) {
      if (!/^[a-f0-9]{32}$/.test(item.id)) {
        safe = false;
        continue;
      }
      for (const args of [
        ["ps", "-aq", "--filter", "name=game-shelf-verify-" + item.id],
        ["volume", "ls", "-q", "--filter", "name=game-shelf-ci-cache-" + item.id],
        [
          "ps",
          "-aq",
          "--filter",
          "label=com.docker.compose.project=game-shelf-ci-verify-" + item.id + "-1",
        ],
        [
          "volume",
          "ls",
          "-q",
          "--filter",
          "label=com.docker.compose.project=game-shelf-ci-verify-" + item.id + "-1",
        ],
      ])
        if (bridge.read(args)) safe = false;
    }
    if (created && safe) {
      assert.equal(
        bridge.read([
          "volume",
          "inspect",
          "--format",
          '{{index .Labels "io.game-shelf.ci-owner"}}',
          volume,
        ]),
        id,
      );
      bridge.read(["volume", "rm", volume]);
      assert.equal(bridge.read(["volume", "ls", "-q", "--filter", "name=^" + volume + "$"]), "");
    } else if (created) {
      report.retained_volume = true;
      throw new Error("嵌套资源未确认结束，保留本轮父测试卷供诊断，不强制删除");
    }
  } catch (error) {
    failure ??= error;
    report.status = "failed";
    report.cleanup_error = error.stack;
  }
  try {
    await network?.close();
  } catch (error) {
    failure ??= error;
    report.status = "failed";
    report.network_error = error.stack;
  }
  await rm(temp, { recursive: true, force: true });
  report.finished_at = new Date().toISOString();
  await writeFile(join(folder, "ownership.json"), JSON.stringify(report, null, 2) + "\n");
}
console.log("普通Linux父进程回归：" + report.status + "，证据：" + folder);
if (failure) {
  console.error(failure);
  process.exitCode = 1;
}
