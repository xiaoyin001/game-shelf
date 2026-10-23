// 文件说明：只读确认验收的Git身份，并创建不携带源配置/凭据的暂存tree副本。
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
export const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
export function git(args, repository = root, allowFailure = false) {
  const r = spawnSync("git", args, {
    cwd: repository,
    encoding: "utf8",
    windowsHide: true,
    timeout: 20000,
    maxBuffer: 8 * 1024 * 1024,
  });
  if (!allowFailure && (r.error || r.status !== 0))
    throw r.error || new Error("Git检查失败：" + r.stderr);
  return allowFailure ? r : r.stdout.trim();
}
export function gitIdentity(candidate = false, repository = root) {
  assert.equal(git(["diff", "--name-only"], repository), "", "存在未暂存改动，先完整暂存候选切片");
  assert.equal(
    git(["ls-files", "--others", "--exclude-standard"], repository),
    "",
    "存在未跟踪文件，拒绝遗漏源码",
  );
  if (!candidate)
    assert.equal(
      git(["diff", "--cached", "--name-only"], repository),
      "",
      "正式验证只接受干净提交",
    );
  return {
    mode: candidate ? "candidate" : "commit",
    commit: git(["rev-parse", "HEAD"], repository),
    tree: git(["write-tree"], repository),
  };
}
export function createSnapshot(repository, workspace, identity) {
  git([
    "-c",
    "init.templateDir=",
    "clone",
    "--quiet",
    "--local",
    "--no-hardlinks",
    "--no-checkout",
    repository,
    workspace,
  ]);
  git(["update-ref", "--no-deref", "HEAD", identity.commit], workspace);
  git(["read-tree", identity.tree], workspace);
  git(["checkout-index", "--all", "--force"], workspace);
}
export function repositoryKey(value) {
  let url;
  if (value.startsWith("git@github.com:")) url = new URL("https://github.com/" + value.slice(15));
  else {
    try {
      url = new URL(value);
    } catch {
      throw new Error("只接受标准GitHub仓库地址，不打印原始凭据地址");
    }
  }
  assert.ok(
    ["https:", "ssh:"].includes(url.protocol) && url.hostname.toLowerCase() === "github.com",
    "仓库不在当前GitHub验收范围",
  );
  const path = url.pathname
    .replace(/^\//, "")
    .replace(/\.git\/?$/, "")
    .replace(/\/$/, "");
  assert.match(path, /^[\w.-]+\/[\w.-]+$/, "仓库路径无效");
  return "github.com/" + path.toLowerCase();
}
