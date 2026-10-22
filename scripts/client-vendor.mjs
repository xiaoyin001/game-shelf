// 文件说明：从精确锁定的 Three.js 同步内嵌模块与版权，检查模式只比较原始字节，不修改文件。
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { createHash } from "node:crypto";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const pkg = JSON.parse(await readFile(join(root, "package.json"), "utf8"));
const source = join(root, "node_modules/three");
const installed = JSON.parse(await readFile(join(source, "package.json"), "utf8"));
if (installed.version !== pkg.dependencies.three) throw new Error("Three.js 版本与精确依赖不符");
const target = join(root, "client/web/vendor");
const files = [
  ["build/three.module.js", "three.module.js"],
  ["build/three.core.js", "three.core.js"],
  ["LICENSE", "LICENSE.three.txt"],
];
const check = process.argv.includes("--check");
if (!check) await mkdir(target, { recursive: true });
for (const [input, output] of files) {
  const expected = await readFile(join(source, input));
  if (check) {
    const actual = await readFile(join(target, output));
    if (!actual.equals(expected))
      throw new Error("第三方资源漂移：" + output + "；显式执行 npm run vendor:sync");
  } else await writeFile(join(target, output), expected);
}
const metadata =
  JSON.stringify(
    {
      package: "three",
      version: installed.version,
      license: "MIT",
      source: "https://www.npmjs.com/package/three/v/" + installed.version,
      files: await Promise.all(
        files.map(async ([input, output]) => ({
          path: output,
          sha256: createHash("sha256")
            .update(await readFile(join(source, input)))
            .digest("hex"),
        })),
      ),
    },
    null,
    2,
  ) + "\n";
if (check) {
  if ((await readFile(join(target, "manifest.json"), "utf8")) !== metadata)
    throw new Error("资源清单不符");
} else await writeFile(join(target, "manifest.json"), metadata);
console.log(
  "Three.js " + installed.version + " 内嵌模块及许可字节" + (check ? "检查通过" : "同步完成"),
);
