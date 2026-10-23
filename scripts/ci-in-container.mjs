// 文件说明：在同一有界容器内执行正式检查；限制不成立就失败，不按环境缩减测试。
import assert from "node:assert/strict";
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { join } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import os from "node:os";
import { profile, cpuList, checkLimits } from "./ci-profile.mjs";
import { checkCiKernel } from "./ci-platform.mjs";
const kind = process.argv[2];
assert.ok(["source", "playable"].includes(kind), "未知CI job");
assert.equal(process.platform, "linux");
assert.equal(process.arch, "x64");
checkLimits(
  readFileSync("/sys/fs/cgroup/cpu.max", "utf8"),
  readFileSync("/sys/fs/cgroup/memory.max", "utf8"),
  readFileSync("/sys/fs/cgroup/memory.swap.max", "utf8"),
);
const allowed = cpuList(
  readFileSync("/proc/self/status", "utf8").match(/^Cpus_allowed_list:\s*(.+)$/m)[1],
);
assert.ok(allowed.length >= profile.cpus, "可用CPU不足");
let active;
let interrupted = false;
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => {
    interrupted = true;
    active?.kill(signal);
  });
async function command(file, args) {
  if (interrupted) throw new Error("验证已中断");
  await new Promise((done, fail) => {
    active = spawn(file, args, { stdio: "inherit" });
    active.once("error", fail);
    active.once("exit", (code, signal) =>
      code === 0 ? done() : fail(new Error(file + " exited " + code + " signal=" + signal)),
    );
  });
  active = null;
}
if (!process.argv.includes("--bounded")) {
  await command("taskset", [
    "-c",
    allowed.slice(0, profile.cpus).join(","),
    process.execPath,
    process.argv[1],
    kind,
    "--bounded",
  ]);
} else {
  assert.equal(os.availableParallelism(), profile.cpus, "CPU亲和限制未生效");
  assert.equal(process.versions.node, profile.node);
  if (kind === "playable") {
    checkCiKernel(os.release());
    assert.match(readFileSync("/proc/cpuinfo", "utf8"), /\bavx\b/, "MongoDB需要AVX");
  }
  const scope =
    kind === "playable" && process.env.GAME_SHELF_CI_ID?.endsWith("-2") ? "browser-repeat" : "full";
  const go = spawnSync("go", ["env", "GOVERSION", "CGO_ENABLED"], { encoding: "utf8" });
  assert.equal(go.status, 0, go.stderr);
  assert.deepEqual(go.stdout.trim().split(/\s+/), [profile.go, kind === "source" ? "1" : "0"]);
  mkdirSync(".local/logs", { recursive: true });
  const result = {
    kind,
    scope,
    run_id: process.env.GAME_SHELF_CI_ID,
    status: "running",
    started_at: new Date().toISOString(),
    kernel: os.release(),
    distribution: readFileSync("/etc/os-release", "utf8"),
    node: process.versions.node,
    go: profile.go,
    cgo: kind === "source" ? 1 : 0,
    cpus: os.availableParallelism(),
    memory_bytes: profile.memoryBytes,
    swap_bytes: 0,
    steps: [],
  };
  const packages = spawnSync("dpkg-query", ["-W", "-f", "${binary:Package}=${Version}\\n"], {
    encoding: "utf8",
  });
  assert.equal(packages.status, 0, packages.stderr);
  result.system_packages = packages.stdout;
  const save = () =>
    writeFileSync(".local/logs/ci-tool-result.json", JSON.stringify(result, null, 2) + "\n");
  save();
  try {
    const commands =
      kind === "source"
        ? [
            ["npm", ["ci", "--no-audit", "--no-fund"]],
            ["npm", ["run", "check"]],
            ["npm", ["run", "check:client"]],
            ["go", ["test", "-race", "-count=1", "./..."]],
            ["git", ["diff", "--exit-code"]],
          ]
        : [
            ["npm", ["ci", "--no-audit", "--no-fund"]],
            ["npm", ["run", scope === "full" ? "test:ci" : "test:e2e"]],
          ];
    for (const [file, args] of commands) {
      const started = Date.now();
      await command(file, args);
      result.steps.push({ command: [file, ...args], duration_ms: Date.now() - started });
      save();
    }
    if (kind === "playable") {
      assert.equal(
        JSON.parse(readFileSync("node_modules/@playwright/test/package.json", "utf8")).version,
        profile.playwright,
      );
      const folder = readdirSync("/ms-playwright").find((name) =>
        name.startsWith("chromium_headless_shell-"),
      );
      assert.ok(folder, "没有安装真实headless shell");
      const find = (dir) =>
        readdirSync(dir, { withFileTypes: true }).flatMap((entry) =>
          entry.isDirectory()
            ? find(join(dir, entry.name))
            : ["headless_shell", "chrome-headless-shell"].includes(entry.name)
              ? [join(dir, entry.name)]
              : [],
        );
      const files = find(join("/ms-playwright", folder));
      assert.equal(files.length, 1, "headless shell文件不唯一");
      result.browser_binary = {
        revision: folder,
        sha256: createHash("sha256").update(readFileSync(files[0])).digest("hex"),
      };
    }
    result.status = "passed";
  } finally {
    if (result.status !== "passed") result.status = "failed";
    result.finished_at = new Date().toISOString();
    save();
  }
}
