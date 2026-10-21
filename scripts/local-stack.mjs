// 文件说明：构建并管理自己的四服务进程，拒绝接管占用端口的旧实例。
import { spawn } from "node:child_process";
import { mkdir, open, unlink } from "node:fs/promises";
import { createWriteStream } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { setTimeout as delay } from "node:timers/promises";
import { root, startInfra, listening, run } from "./infra.mjs";
const ports = [11000, 11020, 12000, 12001, 12010, 12020, 13010, 13020, 14010, 14020];
const children = [],
  logs = [];
let lock = null,
  stopInfra = null,
  stopping = false;
// startStack 构建当前服务，等待真实 ready 后才返回回收函数。
export async function startStack() {
  await mkdir(join(root, ".local/bin"), { recursive: true });
  await mkdir(join(root, ".local/logs"), { recursive: true });
  const checks = await Promise.all(ports.map(listening));
  const used = ports.filter((_, i) => checks[i]);
  if (used.length) throw new Error("端口已占用，拒绝接管旧实例：" + used.join(","));
  lock = await open(join(root, ".local/stack.lock"), "wx");
  await lock.writeFile(String(process.pid));
  try {
    stopInfra = await startInfra();
    for (const name of ["db", "lobby", "gate", "login"]) {
      const binary = join(root, ".local/bin", name + (process.platform === "win32" ? ".exe" : ""));
      run("go", ["build", "-o", binary, "./cmd/" + name]);
      const output = createWriteStream(join(root, ".local/logs", name + ".log"));
      logs.push(output);
      const child = spawn(binary, ["-config", "configs/local/" + name + ".json"], {
        cwd: root,
        stdio: ["ignore", "pipe", "pipe"],
        windowsHide: true,
      });
      child.stdout.pipe(output, { end: false });
      child.stderr.pipe(output, { end: false });
      children.push(child);
      child.once("error", (error) => {
        console.error(error);
      });
    }
    const end = Date.now() + 45000;
    while (Date.now() < end) {
      if (children.some((c) => c.exitCode !== null || c.signalCode !== null))
        throw new Error("服务提前退出，查看 .local/logs。");
      const ready = await Promise.all(
        [11020, 12020, 13020, 14020].map(async (port) => {
          try {
            return (
              await fetch("http://127.0.0.1:" + port + "/readyz", {
                signal: AbortSignal.timeout(1000),
              })
            ).ok;
          } catch {
            return false;
          }
        }),
      );
      if (ready.every(Boolean)) {
        console.log("game-shelf ready: http://127.0.0.1:11000");
        return stopStack;
      }
      await delay(250);
    }
    throw new Error("四服务未在 45 秒内就绪。");
  } catch (error) {
    await stopStack();
    throw error;
  }
}
// stopStack 只回收本运行器创建的服务，不按端口杀进程或清理数据卷。
export async function stopStack() {
  if (stopping) return;
  stopping = true;
  for (const child of children.toReversed()) {
    if (child.exitCode !== null || child.signalCode !== null) continue;
    child.kill("SIGTERM");
    await Promise.race([new Promise((done) => child.once("exit", done)), delay(5000)]);
    if (child.exitCode === null && child.signalCode === null) {
      child.kill("SIGKILL");
      await Promise.race([new Promise((done) => child.once("exit", done)), delay(1000)]);
    }
  }
  logs.forEach((log) => log.end());
  if (stopInfra) await stopInfra();
  if (lock) {
    await lock.close();
    await unlink(join(root, ".local/stack.lock"));
    lock = null;
  }
}
if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  try {
    await startStack();
    for (const signal of ["SIGINT", "SIGTERM"])
      process.once(signal, () => stopStack().then(() => process.exit(0)));
    for (const child of children)
      child.once("exit", () => {
        if (!stopping) stopStack().then(() => process.exit(1));
      });
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
