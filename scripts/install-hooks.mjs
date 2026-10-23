// 文件说明：仅安装本仓库防漏检hook，不覆盖已有自定义钩子或修改全局/远端设置。
import assert from "node:assert/strict";
import { lstatSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { git, root } from "./ci-git.mjs";
export function installHooks(repository = root) {
  const config = git(["config", "--get", "core.hooksPath"], repository, true);
  assert.ok(!config.error && [0, 1].includes(config.status), "无法读取有效hooksPath");
  const current = config.stdout.trim();
  assert.ok(!current || current === ".githooks", "已有其他hooksPath，拒绝覆盖，请先审核整合");
  if (!current) {
    const dir = resolve(repository, git(["rev-parse", "--git-common-dir"], repository));
    try {
      lstatSync(join(dir, "hooks/pre-push"));
      throw new Error("已有活动pre-push，拒绝覆盖");
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  const entry = git(["ls-files", "--stage", "--", ".githooks/pre-push"], repository);
  assert.ok(entry.startsWith("100755 "), "pre-push必须入库并具备Git执行位");
  git(["config", "--local", "core.hooksPath", ".githooks"], repository);
  return current;
}
if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  try {
    installHooks();
    console.log("本仓库pre-push已安装；未改全局或远端配置。仍需每次Push明确授权。");
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
