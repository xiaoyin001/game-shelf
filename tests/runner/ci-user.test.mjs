// 文件说明：在正式SDK中实测执行身份与新建目录属主；非SDK入口明确不冒称验证了宿主Unix权限。
import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, mkdtemp, writeFile, stat, rm } from "node:fs/promises";
import { join } from "node:path";

test(
  "SDK沿用宿主UID/GID且生成目录可由同一身份删除",
  { skip: process.platform !== "linux" || !process.env.GAME_SHELF_RUN_UID },
  async () => {
    const uid = Number(process.env.GAME_SHELF_RUN_UID),
      gid = Number(process.env.GAME_SHELF_RUN_GID);
    assert.equal(process.getuid(), uid);
    assert.equal(process.getgid(), gid);
    await mkdir(".local", { recursive: true });
    const dir = await mkdtemp(".local/ci-user-test-");
    try {
      await mkdir(join(dir, "logs"));
      await writeFile(join(dir, "logs/data"), "owned by runner\n");
      for (const file of [dir, join(dir, "logs"), join(dir, "logs/data")]) {
        const info = await stat(file);
        assert.equal(info.uid, uid, "SDK生成文件的属主错误");
        assert.equal(info.gid, gid, "SDK生成文件的属组错误");
      }
    } finally {
      await rm(dir, { recursive: true, force: true });
    }
  },
);
