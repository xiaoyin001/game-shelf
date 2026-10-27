// 文件说明：在真实当前服务组合上验证 HTTP 与 TCP 链路，结束后回收自己创建的进程。
import { startStack } from "./local-stack.mjs";
import { run } from "./infra.mjs";
import { serviceTestArgs } from "./service-tests.mjs";
const stop = await startStack();
try {
  run("go", serviceTestArgs(), { GAME_SHELF_INTEGRATION: "1" });
} finally {
  await stop();
}
