// 文件说明：WSL本轮反向通道，仅监听虚拟网卡和回环，凭一次性令牌接入主机连接，不记录流量。
import { createServer } from "node:net";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
export async function reverseServer({ host, allowedAddress, secret }) {
  const sockets = new Set(),
    idle = [],
    pending = [];
  const own = (socket) => {
    sockets.add(socket);
    socket.on("error", () => socket.destroy());
    socket.on("close", () => sockets.delete(socket));
    return socket;
  };
  function pair() {
    while (pending.length && idle.length) {
      const client = pending.shift(),
        backhaul = idle.shift();
      if (client.destroyed || backhaul.destroyed) {
        if (!client.destroyed) pending.unshift(client);
        if (!backhaul.destroyed) idle.unshift(backhaul);
        continue;
      }
      client.setTimeout(0);
      backhaul.setTimeout(0);
      backhaul.write("G");
      client.pipe(backhaul);
      backhaul.pipe(client);
      client.on("close", () => backhaul.destroy());
      backhaul.on("close", () => client.destroy());
    }
  }
  const control = createServer((incoming) => {
    const socket = own(incoming);
    if (socket.remoteAddress !== allowedAddress) {
      socket.destroy();
      return;
    }
    socket.setTimeout(10000, () => socket.destroy());
    let hello = "";
    const auth = (chunk) => {
      hello += chunk.toString();
      if (hello.length > 128 || (hello.includes("\n") && hello !== secret + "\n")) {
        socket.destroy();
        return;
      }
      if (hello === secret + "\n") {
        socket.removeListener("data", auth);
        socket.pause();
        socket.setTimeout(0);
        idle.push(socket);
        pair();
      }
    };
    socket.on("data", auth);
  });
  const local = createServer((incoming) => {
    const socket = own(incoming);
    if (sockets.size > 128) {
      socket.destroy();
      return;
    }
    socket.setTimeout(10000, () => socket.destroy());
    socket.pause();
    pending.push(socket);
    pair();
  });
  const listen = (server, address) =>
    new Promise((done, fail) => {
      server.once("error", fail);
      server.listen(0, address, done);
    });
  try {
    await listen(control, host);
    await listen(local, "127.0.0.1");
  } catch (error) {
    for (const socket of sockets) socket.destroy();
    if (control.listening) control.close();
    if (local.listening) local.close();
    throw error;
  }
  return {
    port: local.address().port,
    backhaul: control.address().port,
    close: async () => {
      for (const socket of sockets) socket.destroy();
      await Promise.all(
        [control, local].map((server) => new Promise((done) => server.close(done))),
      );
    },
  };
}
if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  let input = "";
  const config = await new Promise((done) => {
    const read = (chunk) => {
      input += chunk;
      const end = input.indexOf("\n");
      if (end >= 0) {
        process.stdin.removeListener("data", read);
        done(JSON.parse(input.slice(0, end)));
      }
    };
    process.stdin.on("data", read);
  });
  const server = await reverseServer(config);
  console.log(JSON.stringify({ port: server.port, backhaul: server.backhaul }));
  let closing = false;
  const close = async () => {
    if (closing) return;
    closing = true;
    await server.close();
    process.stdin.destroy();
  };
  process.stdin.on("end", close);
  for (const signal of ["SIGINT", "SIGTERM"]) process.once(signal, close);
}
