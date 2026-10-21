// 文件说明：用独立浏览器上下文验收真实客户端，服务由 Node 运行器统一托管。
import { defineConfig } from "@playwright/test";
if (process.env.GAME_SHELF_TEST_STACK !== "external")
  throw new Error("使用 npm run test:e2e 或 npm run test:all，避免误测旧服务。");
export default defineConfig({
  testDir: "./tests/browser",
  timeout: 30000,
  expect: { timeout: 8000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:11000",
    viewport: { width: 1440, height: 1080 },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
