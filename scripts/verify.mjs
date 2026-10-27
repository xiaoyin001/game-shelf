// 文件说明：对当前已实现的真实服务和浏览器能力运行组合回归，统一回收自建进程。
import { spawn } from "node:child_process";
import { startStack } from "./local-stack.mjs";
import { root, run } from "./infra.mjs";
import { serviceTestArgs } from "./service-tests.mjs";
const stop = await startStack();
try {
  if (!process.argv.includes("--browser-only"))
    run("go", serviceTestArgs(true), {
      GAME_SHELF_INFRA: "1",
      GAME_SHELF_INTEGRATION: "1",
    });
  await new Promise((done, fail) => {
    const args = [
      "node_modules/@playwright/test/cli.js",
      "test",
      ...process.argv.slice(2).filter((v) => v !== "--browser-only"),
    ];
    const child = spawn(process.execPath, args, {
      cwd: root,
      stdio: "inherit",
      windowsHide: true,
      env: { ...process.env, GAME_SHELF_TEST_STACK: "external" },
    });
    child.once("error", fail);
    child.once("exit", (code) =>
      code === 0 ? done() : fail(new Error("Playwright exited " + code)),
    );
  });
} finally {
  await stop();
}
