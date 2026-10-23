// 文件说明：真实TCP验证反向通道认证、字节转发及退出，不接触或修改系统代理。
import test from "node:test";
import assert from "node:assert/strict";
import { createServer, createConnection } from "node:net";
import { reverseClient } from "../../scripts/ci-network.mjs";
import { reverseServer } from "../../scripts/ci-network-server.mjs";

test("任务反向通道拒绝错误令牌/源地址，正确连接透明转发且退出回收", { timeout: 5000 }, async () => {
  let accepted = 0;
  const upstream = createServer((socket) => {
    accepted++;
    socket.on("error", () => {});
    socket.pipe(socket);
  });
  await new Promise((done) => upstream.listen(0, "127.0.0.1", done));
  const secret = "a".repeat(64);
  const server = await reverseServer({ host: "127.0.0.1", allowedAddress: "127.0.0.1", secret });
  const denied = await reverseServer({ host: "127.0.0.1", allowedAddress: "127.0.0.2", secret });
  let client;
  const rejected = (port, value) =>
    new Promise((done) => {
      const socket = createConnection({ host: "127.0.0.1", port }, () =>
        socket.write(value + "\n"),
      );
      socket.on("error", () => {});
      socket.on("close", done);
    });
  try {
    await rejected(server.backhaul, "wrong-token");
    await rejected(denied.backhaul, secret);
    assert.equal(accepted, 0);
    client = await reverseClient({
      host: "127.0.0.1",
      port: server.backhaul,
      secret,
      targetHost: "127.0.0.1",
      targetPort: upstream.address().port,
    });
    const bytes = Buffer.from([0, 1, 127, 128, 255, 13, 10]);
    const echoed = await new Promise((done, fail) => {
      const socket = createConnection({ host: "127.0.0.1", port: server.port }, () =>
        socket.write(bytes),
      );
      let result = Buffer.alloc(0);
      socket.on("error", fail);
      socket.on("data", (chunk) => {
        result = Buffer.concat([result, chunk]);
        if (result.length === bytes.length) {
          socket.destroy();
          done(result);
        }
      });
    });
    assert.deepEqual(echoed, bytes);
    assert.equal(accepted, 1);
  } finally {
    await client?.close();
    await server.close();
    await denied.close();
    await new Promise((done) => upstream.close(done));
  }
  await assert.rejects(
    new Promise((done, fail) => {
      const socket = createConnection({ host: "127.0.0.1", port: server.port }, () => {
        socket.destroy();
        done();
      });
      socket.on("error", fail);
    }),
  );
});
