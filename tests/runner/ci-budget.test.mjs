// 文件说明：约束本地/CI共用环境和期限；静态配置检查不冒充真实Docker或浏览器验收。
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { profile } from "../../scripts/ci-profile.mjs";

test("两个CI job都经共用运行器，分层期限与本地配置一致", () => {
  const workflow = readFileSync(".github/workflows/ci.yml", "utf8");
  for (const kind of ["source", "playable"]) {
    const section = workflow.split("  " + kind + ":\n")[1].split(/\n  \w+:\n/)[0];
    assert.match(section, new RegExp("timeout-minutes: " + profile.jobMs / 60000));
    assert.match(
      section,
      new RegExp(
        "timeout-minutes: " +
          profile.prepareMs / 60000 +
          "\\s+run: npm run verify:ci -- --job " +
          kind +
          " --prepare",
      ),
    );
    assert.match(
      section,
      new RegExp(
        "timeout-minutes: " +
          profile.runMs / 60000 +
          "\\s+run: npm run verify:ci -- --job " +
          kind +
          " --run",
      ),
    );
    assert.match(section, /persist-credentials: false/);
  }
  assert.equal(profile.rounds, 2);
  assert.equal(profile.maxBrowserMs, 27000);
  const browser = readFileSync("playwright.config.mjs", "utf8");
  assert.match(browser, /timeout: 30000/);
  assert.match(browser, /retries: 0/);
  assert.match(browser, /workers: 1/);
  assert.match(browser, /playwright.json/);
});

test("工具基线均固定digest并匹配当前Go、Node与Playwright", () => {
  const dockerfile = readFileSync("deploy/ci-tools.Dockerfile", "utf8");
  const bases = [...dockerfile.matchAll(/^FROM ([^\s]+) AS (\w+)$/gm)]
    .map((m) => m[1])
    .filter((name) => name.includes(":"));
  assert.equal(bases.length, 4);
  for (const base of bases) assert.match(base, /@sha256:[a-f0-9]{64}$/);
  assert.ok(dockerfile.includes("node:" + profile.node + "-"));
  assert.ok(dockerfile.includes("golang:" + profile.go.slice(2) + "-"));
  assert.match(dockerfile, /FROM ubuntu:22\.04@sha256:[a-f0-9]{64}/);
  assert.match(dockerfile, /20261021T000000Z/);
  assert.match(dockerfile, /COPY package\.json package-lock\.json/);
  assert.match(dockerfile, /playwright install --with-deps --only-shell chromium/);
  const pkg = JSON.parse(readFileSync("package.json", "utf8"));
  assert.equal(pkg.devDependencies["@playwright/test"], profile.playwright);
  assert.ok(readFileSync("go.mod", "utf8").includes("go " + profile.go.slice(2)));
  const entry = readFileSync("scripts/ci-in-container.mjs", "utf8");
  assert.match(entry, /"-race"/);
  assert.match(entry, /"test:ci"/);
});
