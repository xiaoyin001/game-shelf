// 文件说明：短协议先完成，三个真实私有长回合与单worker浏览器重叠；全部通过后才统一回收自建进程。
import { startStack } from "./local-stack.mjs";
import { root, run } from "./infra.mjs";
import { serviceTestArgs, longServiceTests } from "./service-tests.mjs";
import { managedTask } from "./managed-task.mjs";
const controller = new AbortController();
const abort = () => controller.abort();
for (const sig of ["SIGINT", "SIGTERM"]) process.once(sig, abort);
let stop, long, browser;
try {
  stop = await startStack();
  if (!process.argv.includes("--browser-only")) {
    run("go", serviceTestArgs(true, "short"), {
      GAME_SHELF_INFRA: "1",
      GAME_SHELF_INTEGRATION: "1",
    });
    long = managedTask("go", serviceTestArgs(false, "long"), {
      cwd: root,
      env: { ...process.env, GAME_SHELF_INTEGRATION: "1" },
      markers: longServiceTests,
      signal: controller.signal,
    });
    await long.ready;
  }
  browser = managedTask(
    process.execPath,
    [
      "node_modules/@playwright/test/cli.js",
      "test",
      ...process.argv.slice(2).filter((v) => v !== "--browser-only"),
    ],
    {
      cwd: root,
      env: { ...process.env, GAME_SHELF_TEST_STACK: "external" },
      signal: controller.signal,
    },
  );
  await Promise.all([long?.done, browser.done]);
} finally {
  try {
    await Promise.all([long?.stop(), browser?.stop()]);
  } finally {
    if (stop) await stop();
    for (const sig of ["SIGINT", "SIGTERM"]) process.removeListener(sig, abort);
  }
}
