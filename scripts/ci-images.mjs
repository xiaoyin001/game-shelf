// 文件说明：有显式主机代理时用官方校验的客户端导出同一OCI内容，不修改共享daemon代理。
import assert from "node:assert/strict";
import { readFile, writeFile, mkdir, rename, rm, chmod } from "node:fs/promises";
import { createReadStream } from "node:fs";
import { createHash, randomUUID } from "node:crypto";
import { join } from "node:path";
import { proxyEnvironment } from "./ci-network.mjs";
const assets = {
  win32: [
    "regctl-windows-amd64.exe",
    "70f195ff4923082340738c796880864f66a296e6c5d3296bf743310653ad761f",
  ],
  linux: ["regctl-linux-amd64", "3737fbbdd791e04e7a831640b681f3397a376498e80e2ad9b1b2d27296f239bc"],
};
async function hashFile(file) {
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest("hex");
}
export async function clientImages({
  root,
  context,
  recipe,
  kind,
  bridge,
  execute,
  remaining,
  signal,
}) {
  if (!proxyEnvironment()) return [];
  assert.ok(assets[process.platform], "代理客户端只支持Windows/Linux x64");
  assert.equal(process.arch, "x64");
  const [asset, checksum] = assets[process.platform];
  const tools = join(root, ".local/ci-tools");
  await mkdir(tools, { recursive: true });
  const binary = join(tools, "regctl-v0.11.7" + (process.platform === "win32" ? ".exe" : ""));
  try {
    assert.equal(await hashFile(binary), checksum, "regctl缓存校验失败，不接管未知文件");
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    const partial = binary + "." + randomUUID() + ".partial";
    try {
      await execute(
        process.platform === "win32" ? "curl.exe" : "curl",
        [
          "--fail",
          "--location",
          "--silent",
          "--show-error",
          "--max-time",
          "90",
          "--output",
          partial,
          "https://github.com/regclient/regclient/releases/download/v0.11.7/" + asset,
        ],
        { timeout: Math.min(remaining(), 95000), signal },
      );
      assert.equal(await hashFile(partial), checksum, "regctl官方release SHA256不符");
      if (process.platform !== "win32") await chmod(partial, 0o755);
      await rename(partial, binary);
    } finally {
      await rm(partial, { force: true });
    }
  }
  const env = { ...process.env, REGCTL_CONFIG: join(context, "regctl.json") };
  await execute(binary, ["config", "set", "--docker-cred=false", "--docker-cert=false"], {
    env,
    timeout: remaining(),
    signal,
  });
  const bases = [...recipe.matchAll(/^FROM ([^\s]+@sha256:[a-f0-9]{64}) AS /gm)].map((m) => m[1]);
  assert.equal(bases.length, 4, "固定工具镜像清单无法解析");
  let refs = kind === "source" ? bases.slice(0, 3) : bases;
  if (kind === "playable") {
    const compose = await readFile(join(root, "deploy/docker-compose.ci.yml"), "utf8");
    const dependencies = [...compose.matchAll(/^    image: ([^\s]+@sha256:[a-f0-9]{64})$/gm)].map(
      (m) => m[1],
    );
    assert.equal(dependencies.length, 3, "固定数据库镜像清单无法解析");
    refs = [...refs, ...dependencies];
  }
  const cache = join(root, ".local/ci-image-cache");
  await mkdir(cache, { recursive: true });
  const results = [];
  for (const ref of refs) {
    if (
      bridge
        .read(["image", "ls", "--digests", "--no-trunc", "--format", "{{.Digest}}"])
        .split(/\s+/)
        .includes("sha256:" + ref.split("@sha256:")[1])
    ) {
      results.push({ ref, cached: true });
      continue;
    }
    const manifest = join(context, "manifest.json");
    // 只获取内容地址固定的公开manifest；regctl不读取主机Docker私有凭据。
    await execute(
      binary,
      ["manifest", "get", "--platform", "linux/amd64", "--format", "raw-body", ref],
      { env, timeout: remaining(), signal, log: manifest },
    );
    const config = JSON.parse(await readFile(manifest, "utf8")).config?.digest;
    assert.match(config ?? "", /^sha256:[a-f0-9]{64}$/);
    const expected = new Set([
      config,
      "sha256:" +
        createHash("sha256")
          .update(await readFile(manifest))
          .digest("hex"),
      "sha256:" + ref.split("@sha256:")[1],
    ]);
    const loaded = () =>
      bridge
        .read(["image", "ls", "--no-trunc", "--format", "{{.ID}}"])
        .split(/\s+/)
        .some((id) => expected.has(id));
    if (loaded()) {
      results.push({ ref, cached: true });
      continue;
    }
    const file = join(cache, ref.split("@sha256:")[1] + ".tar");
    const marker = file + ".json";
    try {
      const saved = JSON.parse(await readFile(marker, "utf8"));
      assert.equal(saved.ref, ref);
      assert.equal(await hashFile(file), saved.sha256, "OCI缓存字节校验失败");
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
      const partial = file + "." + randomUUID() + ".partial";
      try {
        console.log("客户端导出固定镜像：" + ref);
        await execute(
          binary,
          [
            "image",
            "export",
            "--platform",
            "linux/amd64",
            "--name",
            ref.split("@")[0],
            ref,
            partial,
          ],
          { env, timeout: remaining(), signal },
        );
        const sha256 = await hashFile(partial);
        await rename(partial, file);
        await writeFile(marker, JSON.stringify({ ref, sha256 }) + "\n");
      } finally {
        await rm(partial, { force: true });
      }
    }
    await bridge.run(["load", "--input", bridge.path(file)], { timeout: remaining(), signal });
    assert.ok(loaded(), "Docker没有加载固定manifest/config内容");
    results.push({ ref, cached: false });
  }
  return results;
}
