// 文件说明：本轮反向桥接显式HTTP代理；不改daemon/防火墙，不向物理网卡开放，不记录流量。
import assert from "node:assert/strict";
import { createConnection } from "node:net";
import { spawn, spawnSync } from "node:child_process";
import { mkdtemp, readFile, writeFile, rm } from "node:fs/promises";
import { join } from "node:path";
import { randomBytes, randomUUID } from "node:crypto";
export function proxyEnvironment() {
  const value =
    process.env.HTTPS_PROXY ||
    process.env.https_proxy ||
    process.env.HTTP_PROXY ||
    process.env.http_proxy;
  if (!value) return null;
  const url = new URL(value);
  assert.equal(url.protocol, "http:", "当前仅支持显式HTTP代理");
  assert.ok(!url.username && !url.password, "不读取或转存带凭据的代理");
  return url;
}
export async function reverseClient({ host, port, secret, targetHost, targetPort }) {
  const sockets = new Set();
  let closed = false,
    ready;
  const connected = new Promise((done, fail) => {
    ready = done;
    setTimeout(() => fail(new Error("本轮WSL反向通道未就绪")), 10000).unref();
  });
  const own = (socket) => {
    sockets.add(socket);
    socket.on("error", () => socket.destroy());
    socket.on("close", () => sockets.delete(socket));
    return socket;
  };
  function connect() {
    if (closed) return;
    const backhaul = own(createConnection({ host, port }));
    let used = false;
    backhaul.once("connect", () => {
      backhaul.write(secret + "\n");
      ready();
    });
    backhaul.on("close", () => {
      if (!used && !closed) setTimeout(connect, 250).unref();
    });
    backhaul.once("data", (chunk) => {
      if (chunk[0] !== 71) {
        backhaul.destroy();
        return;
      }
      used = true;
      connect();
      backhaul.pause();
      const upstream = own(createConnection({ host: targetHost, port: targetPort }));
      backhaul.on("close", () => upstream.destroy());
      upstream.on("close", () => backhaul.destroy());
      upstream.once("connect", () => {
        if (chunk.length > 1) upstream.write(chunk.subarray(1));
        backhaul.pipe(upstream);
        upstream.pipe(backhaul);
      });
    });
  }
  for (let i = 0; i < 8; i++) connect();
  const close = async () => {
    closed = true;
    for (const socket of sockets) socket.destroy();
  };
  try {
    await connected;
  } catch (error) {
    await close();
    throw error;
  }
  return { close };
}
export async function taskNetwork({ image, bridge, root }) {
  const proxy = proxyEnvironment();
  if (!proxy) return { env: {}, enabled: false, close: async () => {} };
  let close = async () => {};
  if (process.platform === "win32" && ["localhost", "127.0.0.1"].includes(proxy.hostname)) {
    const json = (args) => {
      const r = spawnSync("wsl", ["--exec", "ip", "-j", ...args], {
        encoding: "utf8",
        windowsHide: true,
        timeout: 10000,
      });
      assert.equal(r.status, 0, "无法读取本机WSL虚拟网络");
      return JSON.parse(r.stdout);
    };
    const route = json(["route", "show", "default"])[0];
    const address = json(["-4", "address", "show", route.dev])[0]?.addr_info.find(
      (a) => a.family === "inet",
    )?.local;
    assert.ok(route?.gateway && address, "WSL虚拟地址缺失");
    const temp = await mkdtemp(join(root, ".local/ci-network-"));
    const name = "game-shelf-net-tools-" + randomUUID();
    let worker, client;
    try {
      await writeFile(
        join(temp, "server.mjs"),
        await readFile(new URL("./ci-network-server.mjs", import.meta.url)),
      );
      await bridge.run(
        [
          "run",
          "--rm",
          "--init",
          "--name",
          name,
          "--label",
          "io.game-shelf.network=" + name,
          "--network",
          "none",
          "--read-only",
          "--cpus",
          "0.25",
          "--memory",
          "128m",
          "--memory-swap",
          "128m",
          "--pids-limit",
          "64",
          "--mount",
          "type=bind,source=" + bridge.path(temp) + ",target=/out",
          image,
          "node",
          "-e",
          "require('fs').copyFileSync(process.execPath,'/out/node');require('fs').chmodSync('/out/node',0o755)",
        ],
        { timeout: 30000 },
      );
      const secret = randomBytes(32).toString("hex");
      worker = spawn(
        "wsl",
        ["--exec", bridge.path(join(temp, "node")), bridge.path(join(temp, "server.mjs"))],
        { stdio: ["pipe", "pipe", "pipe"], windowsHide: true },
      );
      worker.stdin.on("error", () => {});
      let stderr = "";
      worker.stderr.on("data", (chunk) => {
        stderr += chunk;
      });
      const ports = await new Promise((done, fail) => {
        const timer = setTimeout(() => fail(new Error("WSL本轮网络工具启动超时")), 15000);
        let output = "";
        worker.once("error", fail);
        worker.once("exit", () => {
          clearTimeout(timer);
          fail(new Error("WSL网络工具退出：" + stderr));
        });
        worker.stdout.on("data", (chunk) => {
          output += chunk;
          if (output.includes("\n")) {
            clearTimeout(timer);
            try {
              done(JSON.parse(output.split("\n")[0]));
            } catch (error) {
              fail(error);
            }
          }
        });
        worker.stdin.write(
          JSON.stringify({ host: address, allowedAddress: route.gateway, secret }) + "\n",
        );
      });
      client = await reverseClient({
        host: address,
        port: ports.backhaul,
        secret,
        targetHost: proxy.hostname,
        targetPort: Number(proxy.port || 80),
      });
      proxy.hostname = "127.0.0.1";
      proxy.port = String(ports.port);
    } catch (error) {
      await client?.close();
      worker?.stdin.end();
      if (worker && worker.exitCode === null) worker.kill();
      const present = bridge.read(["ps", "-aq", "--filter", "label=io.game-shelf.network=" + name]);
      if (present) {
        bridge.read(["stop", "--timeout", "5", present]);
        if (bridge.read(["ps", "-aq", "--filter", "label=io.game-shelf.network=" + name]))
          bridge.read(["rm", present]);
      }
      await rm(temp, { recursive: true, force: true });
      throw error;
    }
    close = async () => {
      await client.close();
      worker.stdin.end();
      await Promise.race([
        new Promise((done) => worker.once("exit", done)),
        new Promise((done) => setTimeout(done, 3000)),
      ]);
      if (worker.exitCode === null && worker.signalCode === null) {
        worker.kill();
        throw new Error("本轮WSL网络进程未确认结束");
      }
      await rm(temp, { recursive: true, force: true });
    };
  }
  const noProxy = [
    "localhost",
    "127.0.0.1",
    "::1",
    process.env.NO_PROXY || process.env.no_proxy || "",
  ].join(",");
  return {
    enabled: true,
    env: {
      HTTP_PROXY: proxy.href,
      HTTPS_PROXY: proxy.href,
      http_proxy: proxy.href,
      https_proxy: proxy.href,
      NO_PROXY: noProxy,
      no_proxy: noProxy,
    },
    close,
  };
}
