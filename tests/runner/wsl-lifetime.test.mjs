// 文件说明：Windows实测WSL控制管道关闭会回收本轮命令及其子进程；Linux明确跳过专有分支。
import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { setTimeout as delay } from "node:timers/promises";
import { execute } from "../../scripts/ci-verify.mjs";

test(
  "Windows超时实际终止WSL命令组，不只结束wsl.exe",
  { skip: process.platform !== "win32", timeout: 20000 },
  async () => {
    const path = (value) => {
      const r = spawnSync("wsl", ["--exec", "wslpath", "-a", "-u", value], {
        encoding: "utf8",
        timeout: 15000,
      });
      assert.equal(r.status, 0, r.stderr);
      return r.stdout.trim();
    };
    const dir = await mkdtemp(join(tmpdir(), "game-shelf-wsl-lifetime-"));
    try {
      const wrapper = path(resolve("scripts/wsl-command.sh"));
      const parentFile = join(dir, "parent.pid"),
        childFile = join(dir, "child.pid");
      const command =
        'printf "%s\\n" "$$" > "$1"; sleep 60 & child=$!; printf "%s\\n" "$child" > "$2"; wait';
      await assert.rejects(
        execute(
          "wsl",
          [
            "--exec",
            "sh",
            wrapper,
            "sh",
            "-c",
            command,
            "fixture",
            path(parentFile),
            path(childFile),
          ],
          { timeout: 3000 },
        ),
        /超时/,
      );
      const pids = [
        (await readFile(parentFile, "utf8")).trim(),
        (await readFile(childFile, "utf8")).trim(),
      ];
      for (const pid of pids) assert.match(pid, /^\d+$/);
      const end = Date.now() + 3000;
      let result;
      do {
        result = spawnSync("wsl", ["--exec", "ps", "-p", pids.join(","), "-o", "pid="], {
          encoding: "utf8",
          timeout: 5000,
        });
        if (result.status === 1 && !result.stdout.trim()) break;
        await delay(100);
      } while (Date.now() < end);
      assert.equal(result.status, 1, "本轮WSL进程仍存在：" + result.stdout);
    } finally {
      await rm(dir, { recursive: true, force: true });
    }
  },
);
