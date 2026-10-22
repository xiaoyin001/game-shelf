// 文件说明：用跨平台命令检查当前已存在的规则和 Go 源码，不修改工作树。
import { spawnSync } from "node:child_process";
import { readdir } from "node:fs/promises";
function run(file, args) {
  const r = spawnSync(file, args, { stdio: "inherit", windowsHide: true });
  if (r.error || r.status !== 0) throw r.error || new Error(file + " exited " + r.status);
}
async function goFiles(dir) {
  const result = [];
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch (error) {
    if (error.code === "ENOENT") return result;
    throw error;
  }
  for (const e of entries) {
    const path = dir + "/" + e.name;
    if (e.isDirectory()) result.push(...(await goFiles(path)));
    else if (e.name.endsWith(".go")) result.push(path);
  }
  return result;
}
const files = (
  await Promise.all(["cmd", "internal", "pkg", "client", "tests"].map(goFiles))
).flat();
if (process.argv.includes("--format")) {
  if (files.length) run("gofmt", ["-w", ...files]);
} else {
  run(process.execPath, ["--test", "tests/workflow/governance.test.mjs"]);
  run(process.execPath, ["scripts/check-history.mjs"]);
  run(process.execPath, ["scripts/client-vendor.mjs", "--check"]);
  const fmt = spawnSync("gofmt", ["-l", ...files], { encoding: "utf8", windowsHide: true });
  if (fmt.error || fmt.status !== 0 || fmt.stdout.trim())
    throw new Error("Go 格式检查失败：" + (fmt.stdout || fmt.stderr || fmt.error));
  run("go", ["test", "./..."]);
  run("go", ["vet", "./..."]);
  run("go", ["build", "./..."]);
}
