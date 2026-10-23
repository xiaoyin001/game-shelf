// 文件说明：本地与Actions共用有界工具容器、干净源码副本和真实CI；只清理本轮已保留的scope。
import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { createWriteStream } from "node:fs";
import { readFile, writeFile, mkdir, mkdtemp, cp, rm } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { randomUUID } from "node:crypto";
import { profile, digest, toolRecipeHash, browserResult, ownedProjects } from "./ci-profile.mjs";
import { taskNetwork } from "./ci-network.mjs";
import { clientImages } from "./ci-images.mjs";
import { listening } from "./infra.mjs";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const ownerLabel = "io.game-shelf.verify.id";
export async function guardPorts(
  ports = [11000, 11020, 12000, 12001, 12010, 12020, 13010, 13020, 14010, 14020, 2379, 6379, 27017],
) {
  const found = await Promise.all(ports.map(listening));
  const used = ports.filter((_, i) => found[i]);
  assert.equal(used.length, 0, "主机端口占用，拒绝接管：" + used.join(","));
}
function sync(file, args, cwd = root) {
  const r = spawnSync(file, args, {
    cwd,
    encoding: "utf8",
    windowsHide: true,
    timeout: 20000,
    maxBuffer: 8 * 1024 * 1024,
  });
  if (r.error || r.status !== 0)
    throw r.error || new Error(file + " exited " + r.status + ": " + r.stderr);
  return r.stdout.trim();
}
export function gitIdentity(candidate = false, repository = root) {
  const git = (args) => sync("git", args, repository);
  assert.equal(git(["diff", "--name-only"]), "", "存在未暂存改动，先完整暂存候选切片");
  assert.equal(
    git(["ls-files", "--others", "--exclude-standard"]),
    "",
    "存在未跟踪文件，拒绝遗漏源码",
  );
  if (!candidate)
    assert.equal(git(["diff", "--cached", "--name-only"]), "", "正式验证只接受干净提交");
  return {
    mode: candidate ? "candidate" : "commit",
    commit: git(["rev-parse", "HEAD"]),
    tree: git(["write-tree"]),
  };
}
export function createSnapshot(repository, workspace, identity) {
  sync("git", [
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
  sync("git", ["update-ref", "--no-deref", "HEAD", identity.commit], workspace);
  sync("git", ["read-tree", identity.tree], workspace);
  sync("git", ["checkout-index", "--all", "--force"], workspace);
}
export function toolArgs({ name, image, workspace, id, kind }) {
  assert.match(id, /^verify-[a-f0-9]{32}-[12]$/);
  assert.ok(["source", "playable"].includes(kind));
  assert.equal(name, "game-shelf-" + id + "-" + kind);
  assert.match(image, /^game-shelf-ci-(?:source|playable):[a-f0-9]{16}$/);
  assert.ok(!/[\r\n,]/.test(workspace), "工作目录不支持换行或逗号");
  const args = [
    "run",
    "--rm",
    "--init",
    "--name",
    name,
    "--label",
    ownerLabel + "=" + id,
    "--network",
    "host",
    "--cpus",
    String(profile.cpus),
    "--memory",
    String(profile.memoryBytes),
    "--memory-swap",
    String(profile.memoryBytes),
    "--pids-limit",
    String(profile.pids),
    "--shm-size",
    "1g",
    "--mount",
    "type=bind,source=" + workspace + ",target=/workspace",
    "--mount",
    "type=volume,source=game-shelf-ci-cache-" + id.slice(7, -2) + "-" + kind + ",target=/ci-cache",
    "--env",
    "TINI_KILL_PROCESS_GROUP=1",
    "--env",
    "GIT_CONFIG_COUNT=1",
    "--env",
    "GIT_CONFIG_KEY_0=safe.directory",
    "--env",
    "GIT_CONFIG_VALUE_0=/workspace",
    "--env",
    "GAME_SHELF_CI_ID=" + id,
  ];
  for (const key of [
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "http_proxy",
    "https_proxy",
    "NO_PROXY",
    "no_proxy",
  ])
    args.push("--env", key);
  if (kind === "playable")
    args.push(
      "--env",
      "GAME_SHELF_CI=1",
      "--mount",
      "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
    );
  return [...args, image, "node", "scripts/ci-in-container.mjs", kind];
}
// execute 不把超时留作后台运行；外层finally仍负责停止自己的工具容器与依赖。
export async function execute(
  file,
  args,
  { timeout, log, signal, cwd = root, env = process.env } = {},
) {
  const started = Date.now();
  const output = log ? createWriteStream(log) : null;
  let timer,
    hardTimer,
    active,
    timedOut = false;
  const stop = () => {
    active?.stdin?.end();
    if (file !== "wsl") active?.kill("SIGTERM");
    hardTimer ??= setTimeout(() => active?.kill("SIGKILL"), 6000);
  };
  try {
    if (signal?.aborted) throw new Error("CI验证已中断");
    await new Promise((done, fail) => {
      active = spawn(file, args, {
        cwd,
        env,
        windowsHide: true,
        stdio: ["pipe", "pipe", "pipe"],
      });
      active.stdin.on("error", () => {});
      for (const stream of [active.stdout, active.stderr])
        stream.on("data", (chunk) => {
          process.stdout.write(chunk);
          output?.write(chunk);
        });
      signal?.addEventListener("abort", stop, { once: true });
      timer = setTimeout(() => {
        timedOut = true;
        stop();
      }, timeout);
      active.once("error", fail);
      active.once("close", (code) =>
        code === 0 && !timedOut && !signal?.aborted
          ? done()
          : fail(
              new Error(
                timedOut
                  ? "CI命令超时"
                  : signal?.aborted
                    ? "CI验证已中断"
                    : file + " exited " + code,
              ),
            ),
      );
    });
    return Date.now() - started;
  } finally {
    clearTimeout(timer);
    clearTimeout(hardTimer);
    signal?.removeEventListener("abort", stop);
    if (output) await new Promise((done) => output.end(done));
  }
}
async function dockerBridge() {
  if (process.env.DOCKER_HOST && !process.env.DOCKER_HOST.startsWith("unix://"))
    throw new Error("拒绝连接远端Docker daemon");
  const file = process.platform === "win32" ? "wsl" : "docker";
  const prefix =
    process.platform === "win32"
      ? [
          "--exec",
          "sh",
          sync("wsl", ["--exec", "wslpath", "-a", "-u", join(root, "scripts/wsl-command.sh")]),
          "docker",
        ]
      : [];
  const readPrefix =
    process.platform === "win32"
      ? ["--exec", "timeout", "--signal=TERM", "--kill-after=5s", "15s", "docker"]
      : [];
  const read = (args) => sync(file, [...readPrefix, ...args]);
  assert.equal(
    read(["context", "inspect", "--format", "{{.Endpoints.docker.Host}}"]),
    "unix:///var/run/docker.sock",
    "仅支持本机默认Docker socket；不自动改context",
  );
  const info = JSON.parse(read(["info", "--format", "{{json .}}"]));
  assert.equal(info.OSType, "linux");
  assert.ok(["x86_64", "amd64"].includes(info.Architecture), "CI需要x64 Docker");
  assert.equal(info.CgroupVersion, "2", "需要cgroup v2实际资源限制");
  assert.ok(
    info.NCPU >= profile.cpus && info.MemTotal >= 6 * 1024 ** 3,
    "Docker至少需要2CPU和6GiB可用内存",
  );
  let network;
  const commandEnv = () => {
    const env = { ...process.env, ...network?.env };
    if (process.platform === "win32")
      env.WSLENV = [env.WSLENV, ...Object.keys(network?.env ?? {}).map((key) => key + "/u")]
        .filter(Boolean)
        .join(":");
    return env;
  };
  let keeper;
  if (process.platform === "win32")
    keeper = spawn("wsl", ["--exec", "cat"], {
      stdio: ["pipe", "ignore", "ignore"],
      windowsHide: true,
    });
  const bridge = {
    read,
    info,
    get proxy_enabled() {
      return network?.enabled ?? false;
    },
    connectNetwork: async (image) => {
      network ??= await taskNetwork({ image, bridge, root });
    },
    path: (p) =>
      process.platform === "win32" ? sync("wsl", ["--exec", "wslpath", "-a", "-u", p]) : p,
    run: (args, options) => execute(file, [...prefix, ...args], { env: commandEnv(), ...options }),
    close: async () => {
      let failure;
      try {
        await network?.close();
      } catch (error) {
        failure = error;
      }
      if (!keeper) {
        if (failure) throw failure;
        return;
      }
      keeper.stdin.end();
      await Promise.race([
        new Promise((done) => keeper.once("exit", done)),
        new Promise((done) => setTimeout(done, 3000)),
      ]);
      if (keeper.exitCode === null && keeper.signalCode === null) keeper.kill();
      if (failure) throw failure;
    },
  };
  return bridge;
}
function counts(bridge, project) {
  const label = "label=com.docker.compose.project=" + project;
  return Object.fromEntries(
    [
      ["containers", ["ps", "-aq", "--filter", label]],
      ["volumes", ["volume", "ls", "-q", "--filter", label]],
      ["networks", ["network", "ls", "-q", "--filter", label]],
    ].map(([kind, args]) => [kind, bridge.read(args).split(/\s+/).filter(Boolean).length]),
  );
}
function empty(value) {
  assert.ok(
    Object.values(value).every((n) => n === 0),
    "本轮资源未确认回收为0",
  );
}
async function copyEvidence(workspace, folder) {
  await mkdir(folder, { recursive: true });
  for (const [source, target] of [
    [".local/logs", "logs"],
    ["test-results", "test-results"],
    ["playwright-report", "playwright-report"],
  ]) {
    try {
      await cp(join(workspace, source), join(folder, target), { recursive: true });
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
}
async function prepare(bridge, kind, folder, recipe, recipeHash, inputs, signal) {
  const started = Date.now();
  const remaining = () => {
    const value = profile.prepareMs - (Date.now() - started);
    assert.ok(value > 0, "工具准备期限已耗尽");
    return value;
  };
  const context = await mkdtemp(join(root, ".local/ci-build-"));
  const image = "game-shelf-ci-" + kind + ":" + recipeHash.slice(0, 16);
  const cached = bridge.read(["image", "ls", "-q", "--filter", "reference=" + image]) !== "";
  try {
    await writeFile(join(context, "Dockerfile"), recipe);
    for (const [name, value] of Object.entries(inputs)) await writeFile(join(context, name), value);
    const transfers = await clientImages({
      root,
      context,
      recipe,
      kind,
      bridge,
      execute,
      remaining,
      signal,
    });
    const buildArgs = (target, tag) => [
      "build",
      "--force-rm",
      "--network",
      "host",
      ...["HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy", "NO_PROXY", "no_proxy"].flatMap(
        (key) => ["--build-arg", key],
      ),
      "--platform",
      "linux/amd64",
      "--target",
      target,
      "--label",
      "io.game-shelf.ci.recipe=" + recipeHash,
      "-t",
      tag,
      bridge.path(context),
    ];
    if (kind === "playable") {
      const source = "game-shelf-ci-source:" + recipeHash.slice(0, 16);
      await bridge.run(buildArgs("source", source), {
        timeout: remaining(),
        log: join(folder, "prepare-bootstrap.log"),
        signal,
      });
      await bridge.connectNetwork(source);
    }
    await bridge.run(buildArgs(kind, image), {
      timeout: remaining(),
      log: join(folder, "prepare-" + kind + ".log"),
      signal,
    });
    return {
      image,
      image_id: bridge.read(["image", "inspect", "--format", "{{.Id}}", image]),
      cached_image: cached,
      client_transfers: transfers,
      duration_ms: Date.now() - started,
    };
  } finally {
    await rm(context, { recursive: true, force: true });
  }
}
async function runJob(bridge, kind, workspace, folder, runId, image, signal) {
  const cache = "game-shelf-ci-cache-" + runId + "-" + kind;
  assert.equal(
    bridge.read(["volume", "ls", "-q", "--filter", "name=^" + cache + "$"]),
    "",
    "拒绝复用已有job缓存",
  );
  bridge.read(["volume", "create", "--label", ownerLabel + "=" + runId + "/" + kind, cache]);
  let result, failure;
  try {
    result = await runRounds(bridge, kind, workspace, folder, runId, image, signal);
  } catch (error) {
    failure = error;
  } finally {
    try {
      assert.equal(
        bridge.read([
          "volume",
          "inspect",
          "--format",
          '{{index .Labels "' + ownerLabel + '"}}',
          cache,
        ]),
        runId + "/" + kind,
        "拒绝清理未知缓存卷",
      );
      bridge.read(["volume", "rm", cache]);
      assert.equal(
        bridge.read(["volume", "ls", "-q", "--filter", "name=^" + cache + "$"]),
        "",
        "job缓存未回收",
      );
    } catch (error) {
      failure = failure ? new AggregateError([failure, error], "验证失败且缓存回收失败") : error;
    }
  }
  if (failure) throw failure;
  return { ...result, cache_volumes: 0 };
}
async function runRounds(bridge, kind, workspace, folder, runId, image, signal) {
  const started = Date.now();
  const rounds = [];
  for (let round = 1; round <= (kind === "playable" ? profile.rounds : 1); round++) {
    const id = "verify-" + runId + "-" + round;
    const projects = kind === "playable" ? ownedProjects(id) : [];
    for (const project of projects) empty(counts(bridge, project));
    for (const path of [".local/logs", "test-results", "playwright-report"])
      await rm(join(workspace, path), { recursive: true, force: true });
    const name = "game-shelf-" + id + "-" + kind;
    const evidence = join(folder, kind + "-" + round);
    await mkdir(evidence, { recursive: true });
    const result = {
      round,
      cold_tool_cache: round === 1,
      status: "running",
      projects,
      started_at: new Date().toISOString(),
    };
    let failure;
    try {
      const remaining = profile.runMs - profile.cleanupReserveMs - (Date.now() - started);
      assert.ok(remaining > 0, "job执行预算已耗尽");
      result.duration_ms = await bridge.run(
        toolArgs({ name, image, workspace: bridge.path(workspace), id, kind }),
        { timeout: remaining, log: join(evidence, "runner.log"), signal },
      );
      result.environment = JSON.parse(
        await readFile(join(workspace, ".local/logs/ci-tool-result.json"), "utf8"),
      );
      assert.equal(result.environment.status, "passed");
      assert.equal(result.environment.run_id, id, "不是本轮工具报告");
      assert.equal(
        result.environment.scope,
        kind === "playable" && round === 2 ? "browser-repeat" : "full",
      );
      if (kind === "playable") {
        result.browser = browserResult(
          JSON.parse(await readFile(join(workspace, ".local/logs/playwright.json"), "utf8")),
        );
        for (const file of round === 1
          ? ["ci-cleanup.json", "ci-failure-cleanup.json"]
          : ["ci-cleanup.json"]) {
          const cleanup = JSON.parse(await readFile(join(workspace, ".local/logs", file), "utf8"));
          assert.equal(
            cleanup.project,
            projects[file.includes("failure") ? 1 : 0],
            "不是本轮清理报告",
          );
          empty({
            containers: cleanup.containers,
            volumes: cleanup.volumes,
            networks: cleanup.networks,
          });
          if (file.includes("failure"))
            assert.equal(cleanup.expected_failure, "readiness checker exit 23");
        }
      }
    } catch (error) {
      failure = error;
    } finally {
      try {
        const present = bridge.read(["ps", "-aq", "--filter", "name=^/" + name + "$"]);
        if (present) {
          assert.equal(
            bridge.read([
              "inspect",
              "--format",
              '{{index .Config.Labels "' + ownerLabel + '"}}',
              present,
            ]),
            id,
            "拒绝停止非本轮工具容器",
          );
          bridge.read(["stop", "--timeout", "10", present]);
          if (bridge.read(["ps", "-aq", "--filter", "name=^/" + name + "$"]))
            bridge.read(["rm", present]);
        }
        result.cleanup = {};
        for (const project of projects) {
          const before = counts(bridge, project);
          if (Object.values(before).some(Boolean)) {
            const args = [
              "compose",
              "-p",
              project,
              "-f",
              bridge.path(join(workspace, "deploy/docker-compose.yml")),
              "-f",
              bridge.path(join(workspace, "deploy/docker-compose.ci.yml")),
            ];
            await writeFile(
              join(evidence, project + ".log"),
              bridge.read([...args, "ps", "--all"]) +
                "\n" +
                bridge.read([...args, "logs", "--no-color", "--tail", "100"]),
            );
            bridge.read([...args, "down", "--volumes", "--remove-orphans", "--timeout", "10"]);
          }
          result.cleanup[project] = counts(bridge, project);
          empty(result.cleanup[project]);
        }
        assert.equal(
          bridge.read(["ps", "-aq", "--filter", "name=^/" + name + "$"]),
          "",
          "工具容器未回收",
        );
      } catch (error) {
        failure = failure ? new AggregateError([failure, error], "验证失败且回收未确认") : error;
      }
      try {
        await copyEvidence(workspace, evidence);
      } catch (error) {
        failure ??= error;
      }
      result.status = failure ? "failed" : "passed";
      result.error = failure?.message;
      await writeFile(join(evidence, "result.json"), JSON.stringify(result, null, 2) + "\n");
    }
    rounds.push(result);
    if (failure) throw failure;
  }
  return { kind, status: "passed", rounds, duration_ms: Date.now() - started };
}
export async function verify({
  jobs = ["source", "playable"],
  candidate = false,
  prepareOnly = false,
  runOnly = false,
  signal,
} = {}) {
  assert.ok(jobs.length && jobs.every((job) => ["source", "playable"].includes(job)));
  assert.ok(!(prepareOnly && runOnly));
  const identity = gitIdentity(candidate);
  const runId = randomUUID().replaceAll("-", "");
  const folder = join(root, ".local/logs/ci-verify-" + runId);
  await mkdir(folder, { recursive: true });
  const report = {
    version: 1,
    id: runId,
    ...identity,
    profile_hash: digest(JSON.stringify(profile)),
    status: "running",
    started_at: new Date().toISOString(),
    jobs: [],
  };
  const save = () => writeFile(join(folder, "report.json"), JSON.stringify(report, null, 2) + "\n");
  await save();
  console.log("CI验证报告：" + join(folder, "report.json"));
  let bridge, temp;
  try {
    if (!prepareOnly && jobs.includes("playable")) await guardPorts();
    bridge = await dockerBridge();
    report.host = {
      kernel: bridge.info.KernelVersion,
      docker: bridge.info.ServerVersion,
      cpus: bridge.info.NCPU,
      memory_bytes: bridge.info.MemTotal,
      task_proxy: bridge.proxy_enabled,
    };
    const recipe = await readFile(join(root, "deploy/ci-tools.Dockerfile"), "utf8");
    const inputs = Object.fromEntries(
      await Promise.all(
        ["package.json", "package-lock.json"].map(async (name) => [
          name,
          await readFile(join(root, name), "utf8"),
        ]),
      ),
    );
    report.recipe_hash = toolRecipeHash(
      recipe,
      inputs["package.json"],
      inputs["package-lock.json"],
    );
    if (!prepareOnly) {
      temp = await mkdtemp(join(root, ".local/ci-work-"));
      const workspace = join(temp, "repo");
      createSnapshot(root, workspace, identity);
    }
    for (const kind of jobs) {
      const started = Date.now();
      const prepared = runOnly
        ? { image: "game-shelf-ci-" + kind + ":" + report.recipe_hash.slice(0, 16) }
        : await prepare(bridge, kind, folder, recipe, report.recipe_hash, inputs, signal);
      assert.equal(
        bridge.read([
          "image",
          "inspect",
          "--format",
          '{{index .Config.Labels "io.game-shelf.ci.recipe"}}',
          prepared.image,
        ]),
        report.recipe_hash,
        "工具镜像不是当前配方",
      );
      prepared.image_id ??= bridge.read([
        "image",
        "inspect",
        "--format",
        "{{.Id}}",
        prepared.image,
      ]);
      const runStarted = Date.now();
      if (!prepareOnly) await bridge.connectNetwork(prepared.image);
      report.host.task_proxy = bridge.proxy_enabled;
      const job = prepareOnly
        ? { kind, status: "prepared" }
        : await runJob(bridge, kind, join(temp, "repo"), folder, runId, prepared.image, signal);
      if (!prepareOnly)
        assert.ok(
          Date.now() - runStarted <= profile.runMs - profile.cleanupReserveMs,
          "运行未给退出/取证预留一分钟",
        );
      Object.assign(job, { prepared, elapsed_ms: Date.now() - started });
      report.jobs.push(job);
      assert.ok(job.elapsed_ms <= profile.jobMs, "job总预算超限");
      await save();
    }
    assert.deepEqual(gitIdentity(candidate), identity, "验证期间源码发生变化，不能使用结果");
    report.status = prepareOnly ? "prepared" : "passed";
  } catch (error) {
    report.status = "failed";
    report.error = error.stack;
    throw error;
  } finally {
    try {
      try {
        if (temp) await rm(temp, { recursive: true, force: true });
      } finally {
        await bridge?.close();
      }
    } catch (error) {
      report.status = "failed";
      report.error = [report.error, error.stack].filter(Boolean).join("\n");
      throw error;
    } finally {
      report.finished_at = new Date().toISOString();
      await save();
    }
  }
  console.log(
    "CI验证" + report.status + "：" + report.mode + " " + report.commit + " tree=" + report.tree,
  );
  return { report, path: join(folder, "report.json") };
}
if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  const args = process.argv.slice(2);
  const job = args.includes("--job") ? args[args.indexOf("--job") + 1] : null;
  assert.ok(
    args.every(
      (arg, i) =>
        ["--candidate", "--prepare", "--run", "--job"].includes(arg) ||
        (args[i - 1] === "--job" && ["source", "playable"].includes(arg)),
    ),
    "未知CI参数",
  );
  const controller = new AbortController();
  for (const signal of ["SIGINT", "SIGTERM"]) process.once(signal, () => controller.abort());
  try {
    await verify({
      jobs: job ? [job] : undefined,
      candidate: args.includes("--candidate"),
      prepareOnly: args.includes("--prepare"),
      runOnly: args.includes("--run"),
      signal: controller.signal,
    });
  } catch (error) {
    console.error(error);
    process.exitCode = 1;
  }
}
