// 文件说明：本地 Linux 与 GitHub 共用的冷启动验证入口，不以本地通过代替远端结果。
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import os from "node:os";
import { checkCiKernel } from "./ci-platform.mjs";
if (process.platform !== "linux" || process.arch !== "x64") throw new Error("CI 需要 Linux x64");
if (process.versions.node !== "22.23.3") throw new Error("CI 需要 Node.js 22.23.3");
const go = spawnSync("go", ["env", "GOVERSION"], { encoding: "utf8" });
if (go.status !== 0 || go.stdout.trim() !== "go1.25.0") throw new Error("CI 需要 Go 1.25.0");
if (!/\bavx\b/.test(readFileSync("/proc/cpuinfo", "utf8")))
  throw new Error("MongoDB 8 x64 需要 AVX");
mkdirSync(".local/logs", { recursive: true });
writeFileSync(
  ".local/logs/ci-environment.json",
  JSON.stringify(
    {
      os: os.release(),
      distribution: readFileSync("/etc/os-release", "utf8"),
      platform: process.platform,
      arch: process.arch,
      node: process.versions.node,
      go: go.stdout.trim(),
      memory_bytes: os.totalmem(),
      started_at: new Date().toISOString(),
    },
    null,
    2,
  ) + "\n",
);
checkCiKernel(os.release());
process.env.GAME_SHELF_CI = "1";
function command(file, args) {
  const r = spawnSync(file, args, { stdio: "inherit", env: process.env });
  if (r.error || r.status !== 0) throw r.error || new Error(file + " exited " + r.status);
}
command(process.execPath, ["scripts/check.mjs"]);
command("npm", ["run", "check:client"]);
command("npm", ["run", "test:all"]);
command(process.execPath, ["scripts/test-ci-cleanup.mjs"]);
