// 文件说明：验证运行器拒绝接管占用端口，且不会杀死已有监听进程。
import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:net";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { resolve } from "node:path";

test("端口被占用时明确失败，并保留原监听", { timeout: 15000 }, async () => {
  const server = createServer((socket) => socket.end());
  await new Promise((done, fail) => {
    server.once("error", fail);
    server.listen(11000, "127.0.0.1", done);
  });
  try {
    const child = spawn(process.execPath, ["scripts/local-stack.mjs"], {
      cwd: resolve("."),
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    });
    let output = "";
    child.stdout.on("data", (data) => {
      output += data;
    });
    child.stderr.on("data", (data) => {
      output += data;
    });
    const [code] = await once(child, "exit");
    assert.equal(code, 1);
    assert.match(output, /拒绝接管旧实例/);
    assert.equal(server.listening, true);
  } finally {
    await new Promise((done) => server.close(done));
  }
});
