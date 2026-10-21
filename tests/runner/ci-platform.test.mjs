// 文件说明：发行版标签不能保证宿主内核，已知风险组合必须在启动数据库前明确失败。
import test from "node:test";
import assert from "node:assert/strict";
import { checkCiKernel } from "../../scripts/ci-platform.mjs";
test("当前 CI 接受已验收范围，拒绝内核漂移与未知版本", () => {
  for (const release of ["5.15.0-azure", "6.8.0-1064-azure", "6.18.54.1-microsoft-standard-WSL2"])
    assert.doesNotThrow(() => checkCiKernel(release));
  for (const release of [
    "6.19.0",
    "6.19.12-generic",
    "7.0.0-1017-azure",
    "7.0.14",
    "7.1.0",
    "unknown",
    "6.19x",
    "",
  ])
    assert.throws(() => checkCiKernel(release), /SERVER-131779/);
});
