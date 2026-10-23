// 文件说明：区分保留数据的开发依赖与独立可清理的 CI 依赖，统一真实就绪和失败现场。
import { spawn, spawnSync } from "node:child_process";
import { createConnection } from "node:net";
import { resolve, dirname, join } from "node:path";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { randomUUID } from "node:crypto";
import { setTimeout as delay } from "node:timers/promises";
export const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");

// createInfraSpec 使 CI 只能使用独立 project，不接受可注入 shell 的项目名。
export function createInfraSpec(ci, id = randomUUID()) {
  if (!/^[a-z0-9-]{1,60}$/.test(id)) throw new Error("无效的 CI project 标识");
  const project = ci ? "game-shelf-ci-" + id : "game-shelf";
  const args = ["compose", "-p", project, "-f", "deploy/docker-compose.yml"];
  if (ci) args.push("-f", "deploy/docker-compose.ci.yml");
  return { project, args, disposable: ci };
}
// cleanupArgs 必须同时满足独立 CI scope 与本次创建，普通开发数据永不清理。
export function cleanupArgs(spec, created) {
  if (!spec.disposable || !created) return null;
  if (!spec.project.startsWith("game-shelf-ci-")) throw new Error("拒绝清理非 CI project");
  return [...spec.args, "down", "--volumes", "--remove-orphans", "--timeout", "10"];
}
const ciMode = process.env.GAME_SHELF_CI === "1";
const spec = createInfraSpec(ciMode, ciMode ? process.env.GAME_SHELF_CI_ID : undefined);
let diagnosticCommand = null;
let snapshotSaved = false;

// listening 仅用于占用判断，不等同于数据库协议就绪。
export function listening(port) {
  return new Promise((done) => {
    const conn = createConnection({ host: "127.0.0.1", port });
    const finish = (ok) => {
      conn.destroy();
      done(ok);
    };
    conn.setTimeout(700);
    conn.once("connect", () => finish(true));
    conn.once("error", () => finish(false));
    conn.once("timeout", () => finish(false));
  });
}
// run 执行确定的本地命令，不通过 shell 拼接外部输入。
export function run(file, args, env = {}) {
  const result = spawnSync(file, args, {
    cwd: root,
    stdio: "inherit",
    windowsHide: true,
    env: { ...process.env, ...env },
  });
  if (result.error || result.status !== 0)
    throw result.error || new Error(file + " exited " + result.status);
}
function dockerRead(args) {
  return spawnSync(diagnosticCommand.file, [...diagnosticCommand.prefix, ...args], {
    cwd: root,
    encoding: "utf8",
    windowsHide: true,
    timeout: 15000,
    maxBuffer: 4 * 1024 * 1024,
  });
}
function composeRun(args) {
  if (!diagnosticCommand) throw new Error("没有可用的 Docker Compose");
  run(diagnosticCommand.file, [...diagnosticCommand.prefix, ...spec.args, ...args]);
}
// captureInfraDiagnostics 只读本 project，保留退出码、OOM、端口、镜像 digest 和日志，不输出凭据环境。
export function captureInfraDiagnostics() {
  try {
    mkdirSync(join(root, ".local/logs"), { recursive: true });
    let output =
      new Date().toISOString() + "\nproject=" + spec.project + "\nci=" + spec.disposable + "\n";
    const record = (args) => {
      const result = dockerRead(args);
      output +=
        "\n### " +
        args.join(" ") +
        "\n" +
        (result.stdout || "") +
        (result.stderr || "") +
        (result.error ? String(result.error) : "");
      return result.status === 0 ? result.stdout.trim() : "";
    };
    if (!diagnosticCommand) output += "未检测到可用的 Docker Compose 命令。\n";
    else {
      record([...spec.args, "ps", "--all"]);
      record([...spec.args, "images"]);
      const ids = record([...spec.args, "ps", "--all", "--quiet"])
        .split(/\s+/)
        .filter((id) => /^[a-f0-9]{12,64}$/.test(id));
      if (ids.length) {
        record(["inspect", "--format", "{{.Name}} {{json .State}}", ...ids]);
        record([
          "inspect",
          "--format",
          "{{.Name}} image={{.Image}} ports={{json .NetworkSettings.Ports}}",
          ...ids,
        ]);
        const images = record(["inspect", "--format", "{{.Image}}", ...ids])
          .split(/\s+/)
          .filter((id) => /^sha256:[a-f0-9]{64}$/.test(id));
        if (images.length)
          record([
            "image",
            "inspect",
            "--format",
            "{{.Id}} {{json .RepoDigests}}",
            ...new Set(images),
          ]);
      }
      record([...spec.args, "logs", "--no-color", "--tail", "100"]);
    }
    writeFileSync(join(root, ".local/logs/infra.log"), output, "utf8");
    snapshotSaved = true;
    console.log("依赖现场已保存：.local/logs/infra.log");
  } catch (error) {
    console.error("保存依赖现场失败：" + error.message);
  }
}

// startInfra 先完成工具编译，再准备依赖；返回的回收函数只清理本次拥有的 CI project。
export async function startInfra() {
  if (ciMode && process.platform !== "linux") throw new Error("CI 冷启动必须在 Linux 执行");
  const ports = [2379, 6379, 27017];
  let keeper = null;
  let created = false;
  let stopped = false;
  const stop = async () => {
    if (stopped) return;
    stopped = true;
    let cleanupError = null;
    try {
      const args = cleanupArgs(spec, created);
      if (args) {
        if (!snapshotSaved) captureInfraDiagnostics();
        run(diagnosticCommand.file, [...diagnosticCommand.prefix, ...args]);
        const leftovers = {
          containers: dockerRead([...spec.args, "ps", "--all", "--quiet"]),
          volumes: dockerRead([
            "volume",
            "ls",
            "--filter",
            "label=com.docker.compose.project=" + spec.project,
            "--quiet",
          ]),
          networks: dockerRead([
            "network",
            "ls",
            "--filter",
            "label=com.docker.compose.project=" + spec.project,
            "--quiet",
          ]),
        };
        for (const [kind, result] of Object.entries(leftovers)) {
          if (result.error || result.status !== 0 || result.stdout.trim())
            throw new Error(
              "CI 资源回收未确认：" + kind + " " + (result.stderr || result.stdout || result.error),
            );
        }
        writeFileSync(
          join(root, ".local/logs/ci-cleanup.json"),
          JSON.stringify(
            {
              project: spec.project,
              containers: 0,
              volumes: 0,
              networks: 0,
              checked_at: new Date().toISOString(),
            },
            null,
            2,
          ) + "\n",
        );
      }
    } catch (error) {
      cleanupError = error;
    } finally {
      if (keeper && keeper.exitCode === null && keeper.signalCode === null) {
        keeper.stdin.end();
        await Promise.race([new Promise((done) => keeper.once("exit", done)), delay(3000)]);
        if (keeper.exitCode === null && keeper.signalCode === null) keeper.kill();
      }
    }
    if (cleanupError) throw cleanupError;
  };
  try {
    mkdirSync(join(root, ".local/bin"), { recursive: true });
    const checker = join(
      root,
      ".local/bin",
      "infra-check" + (process.platform === "win32" ? ".exe" : ""),
    );
    run("go", ["build", "-o", checker, "./cmd/infra-check"]);
    const docker = spawnSync("docker", ["compose", "version"], {
      stdio: "ignore",
      windowsHide: true,
    });
    if (!docker.error && docker.status === 0) diagnosticCommand = { file: "docker", prefix: [] };
    else if (process.platform === "win32") {
      const probe = spawnSync("wsl", ["--exec", "docker", "compose", "version"], {
        stdio: "ignore",
        windowsHide: true,
      });
      if (!probe.error && probe.status === 0) {
        diagnosticCommand = { file: "wsl", prefix: ["--exec", "docker"] };
        keeper = spawn("wsl", ["--exec", "cat"], {
          cwd: root,
          stdio: ["pipe", "ignore", "ignore"],
          windowsHide: true,
        });
        await new Promise((done, fail) => {
          keeper.once("spawn", done);
          keeper.once("error", fail);
        });
      }
    }
    const found = await Promise.all(ports.map(listening));
    if (ciMode) {
      if (found.some(Boolean)) throw new Error("CI 依赖端口已占用，拒绝复用或停止已有服务");
      if (!diagnosticCommand) throw new Error("CI 需要原生 Docker Compose");
      const existing = dockerRead([...spec.args, "ps", "--all", "--quiet"]);
      if (existing.error || existing.status !== 0)
        throw existing.error || new Error(existing.stderr);
      if (existing.stdout.trim()) throw new Error("CI project 已有容器，拒绝接管和清理");
      composeRun(["pull"]);
      created = true;
      console.log("创建独立 CI project：" + spec.project);
      composeRun(["up", "-d", "--wait", "--wait-timeout", "60"]);
    } else if (!found.every(Boolean)) {
      if (found.some(Boolean)) throw new Error("依赖端口部分被占用，拒绝覆盖已有实例");
      composeRun(["up", "-d"]);
      const deadline = Date.now() + 45000;
      while (!(await Promise.all(ports.map(listening))).every(Boolean)) {
        if (Date.now() > deadline) throw new Error("依赖未在 45 秒内就绪");
        await delay(300);
      }
    }
    run(checker, ["-config", "configs/local/development.json"]);
    return stop;
  } catch (error) {
    captureInfraDiagnostics();
    try {
      await stop();
    } catch (cleanupError) {
      console.error("清理本次依赖失败：" + cleanupError.message);
    }
    throw error;
  }
}
