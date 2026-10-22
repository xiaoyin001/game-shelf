// 文件说明：区分网络安装/job总预算与真实行为验收时限，禁止通过放宽单例或重试掩盖问题。
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

test("CI环境与完整回归有独立期限，浏览器仍30秒零重试", () => {
  const workflow = readFileSync(".github/workflows/ci.yml", "utf8");
  const playable = workflow.slice(workflow.indexOf("  playable:"));
  assert.match(playable, /timeout-minutes: 25/);
  assert.match(
    playable,
    /name: 安装浏览器工具\s+timeout-minutes: 10\s+run: npm ci && npx playwright install --with-deps --only-shell chromium/,
  );
  assert.match(
    playable,
    /name: 当前真实依赖与浏览器完整回归\s+timeout-minutes: 12\s+run: npm run test:ci/,
  );
  const browser = readFileSync("playwright.config.mjs", "utf8");
  assert.match(browser, /timeout: 30000/);
  assert.match(browser, /retries: 0/);
  assert.match(browser, /workers: 1/);
});
