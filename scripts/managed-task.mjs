// 文件说明：只管理本次spawn的测试进程树，等待真实私有就绪信号，失败/中断有界回收而不按端口杀进程。
import { spawn, spawnSync } from "node:child_process";
import { waitForExit } from "./process-exit.mjs";
export function managedTask(
  file,
  args,
  {
    cwd = process.cwd(),
    env = process.env,
    markers = [],
    readyTimeout = 30000,
    signal,
    output = (chunk) => process.stdout.write(chunk),
  } = {},
) {
  const wanted = new Set(markers),
    seen = new Set();
  let resolveReady,
    rejectReady,
    readyDone = false,
    timer,
    buffer = "",
    stopping,
    aborted = false;
  const ready = new Promise((resolve, reject) => {
    resolveReady = resolve;
    rejectReady = reject;
  });
  const child = spawn(file, args, {
    cwd,
    env,
    windowsHide: true,
    detached: process.platform !== "win32",
    stdio: ["ignore", "pipe", "pipe"],
  });
  const failReady = (err) => {
    if (!readyDone) {
      readyDone = true;
      clearTimeout(timer);
      rejectReady(err);
    }
  };
  const passReady = () => {
    if (!readyDone) {
      readyDone = true;
      clearTimeout(timer);
      resolveReady();
    }
  };
  const parse = (chunk) => {
    output(chunk);
    buffer += chunk.toString();
    const lines = buffer.split("\n");
    buffer = lines.pop().slice(-16384);
    for (const line of lines) {
      const match = /^GAME_SHELF_PRIVATE_READY:([A-Za-z0-9_]+)\r?$/.exec(line);
      if (match && wanted.has(match[1])) seen.add(match[1]);
    }
    if (seen.size === wanted.size) passReady();
  };
  child.stdout.on("data", parse);
  child.stderr.on("data", output);
  const done = new Promise((resolve, reject) => {
    child.once("error", (error) => {
      failReady(error);
      reject(error);
    });
    child.once("close", (code, sig) => {
      clearTimeout(timer);
      signal?.removeEventListener("abort", abort);
      failReady(new Error(file + " exited before private rounds were ready"));
      if (code === 0 && !aborted) resolve();
      else
        reject(
          new Error(file + " exited " + code + " signal=" + sig + (aborted ? " aborted" : "")),
        );
    });
  });
  async function stop() {
    if (stopping) return stopping;
    stopping = (async () => {
      if (!child.pid) return;
      if (process.platform === "win32") {
        if (child.exitCode === null && child.signalCode === null) {
          spawnSync("taskkill", ["/PID", String(child.pid), "/T", "/F"], {
            windowsHide: true,
            stdio: "ignore",
            timeout: 5000,
          });
        }
      } else {
        const kill = (sig) => {
          try {
            process.kill(-child.pid, sig);
          } catch (e) {
            if (e.code !== "ESRCH") throw e;
          }
        };
        kill("SIGTERM");
        await waitForExit(child, 3000);
        kill("SIGKILL");
      }
      if (!(await waitForExit(child, 3000)))
        throw new Error("owned test process did not exit: " + child.pid);
    })();
    return stopping;
  }
  function abort() {
    aborted = true;
    failReady(new Error("test task aborted"));
    void stop().catch(() => {});
  }
  // 立即登记拒绝处理，调用方仍必须await原Promise；不将错误转换成成功。
  ready.catch(() => {});
  done.catch(() => {});
  if (wanted.size === 0) passReady();
  else
    timer = setTimeout(
      () => failReady(new Error("private rounds not ready within " + readyTimeout + "ms")),
      readyTimeout,
    );
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) abort();
  return { child, ready, done, stop };
}
