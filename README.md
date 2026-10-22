# game-shelf · 苔光原野

同仓提供 Go 服务端和浏览器 3D 联机沙盒：在同一地图相遇、移动、聊天与共享采集。代码和 AI 规范从空仓库逐段实现、验证、提交，过程见 [成长记录](docs/progress.md)，不是拆分旧大提交伪造的开发历史。

## 本地体验

需要 Go 1.25+、Node.js 22+ 和 Docker Compose；Windows 可使用 WSL 中的 Docker。

```text
npm ci
npm run dev
```

打开 **http://127.0.0.1:11000**，先填写账号与 **6–32 位密码**（仅英文字母、数字和英文符号，不含空格；中文、表情与非法粘贴拒绝），点击“注册并进入原野”；以后使用“登录并进入原野”。用两个窗口、不同账号进入：
- WASD / 方向键相对镜头移动，最终坐标由服务器决定。
- 右键拖动环绕镜头、滚轮缩放、R 复位；树林、营地、湖面、旅人和晶簇均为真实 3D 场景。
- Enter 聚焦世界聊天，所有在线旅人可见。
- 靠近发光晶簇按 E 采集，获得本轮分数；共享资源 10 秒后重生。
- 同账号通过正确密码验证后顶替旧连接；错误密码不影响原会话。离开后需重新输入密码登录。

运行器先编译当前工具和全部四服务，必要时启动本地依赖，再用同款 Go 驱动验证 MongoDB/Redis/etcd 的真实协议就绪，最后统一启动业务进程，日志在 `.local/logs/`。编译不占用服务启动期限，TCP 端口接通不能单独放行；依赖或起服失败会保存本项目容器状态（含退出码/OOM）、端口、实际镜像 digest 与日志到 `.local/logs/infra.log`。Ctrl+C 只回收自建服务和保活管道；端口被占用时拒绝接管，不清库、不删卷。WSL 依赖可能随宿主空闲停止，下次运行会再启动，命名卷保留。

默认公布回环地址，只验收同机联机。注册和登录都验证密码，db 只保存带随机盐的 Argon2id 慢哈希，入口有限频；但当前是可信本机 HTTP，**不能直接开放互联网或复用其他网站密码**。旧 players 试玩记录保留，同名新注册不会继承旧身份，见 [ADR-0014](docs/decisions/0014-password-authentication.md)。本轮位置、分数、资源与聊天是临时沙盒，离开/重启重置；基础账号经 db 保存到 MongoDB。这不是成熟 MMO、大规模容量或生产账号安全承诺。

## 自动验收

首次安装浏览器：

```text
npx playwright install chromium
```

完整本地检查：

```text
make check
npm run check:client
npm run test:all
```

- `make check` / `npm run check`：AI 入口/ADR/文档、成长历史、Go 格式、包测试、vet 与构建，不修改工作树。
- `npm run test:all`：运行器所有权检查，构建并启动真实四服务，执行真实依赖/协议及双浏览器回归，最后回收自建进程。
- `npm run test:infra` / `test:services`：分别复跑真实依赖或当前四服务链路。
- `npm run test:e2e`：独立启动浏览器验收，可附加 `-- --headed`。
- 普通 `go test ./...` 会明确跳过需起服的测试，不能单独作为可玩通过证据。

### 与 GitHub 相同的 Linux CI

需要 Linux x64/AVX、当前固定数据库组合已验收的内核 **<6.19**、Go **1.25.0**、Node.js **22.23.3** 和支持 `--wait` 的 Docker Compose v2：

```text
npm ci
npx playwright install --with-deps --only-shell chromium
npm run test:ci
```

此入口执行源码与依赖产物检查、真实基础设施/协议、全部 11 个浏览器用例，以及预期失败后的资源清理回归。CI 使用 [独立覆盖配置](deploy/docker-compose.ci.yml) 固定 MongoDB 8.0.34、Redis 7.4.11-alpine、etcd 3.5.18 的 digest，每轮生成独立 project 与新卷。端口已有监听就失败，不复用或停止未知服务。成功与失败均只回收本轮自建容器、卷和网络，并核对残留为 0；普通 `dev`/`test:all` 不删除开发卷。

容器 health 后还需通过主机侧真实 Go 协议检查，随后才启动业务。浏览器仍使用 Playwright 1.64.1 默认的真实 headless shell，`--only-shell` 只省略未使用的 headed 浏览器下载，不减少测试。单独复跑失败清理用 `npm run test:ci-cleanup`：真实依赖健康后故意让检查工具退出 23，验证原错误、现场及定向回收；不会把故障注入当作正向业务通过。

环境与清理结果在 `.local/logs/ci-environment.json`、`ci-cleanup.json`；预期失败证据分别保存在 `ci-failure-infra.log`、`ci-failure-cleanup.json`。隔离配置理由和回退见 [ADR-0012](docs/decisions/0012-reproducible-ci.md)。联机 job 临时使用 Ubuntu 22.04，源码 job 保持 24.04：MongoDB 8.0.34 在实际 GitHub 7.0 内核主动拒绝启动，而官方 Ubuntu 修复 8.0.35 尚未发布。入口记录实际发行版/内核并拒绝超出已验收范围；不关闭数据库保护、不跳过测试。临时方案的弃用期限与恢复较新 runner 的条件见 [ADR-0013](docs/decisions/0013-mongodb-kernel-compatibility.md)。

截图/trace 在 `test-results/`，HTML 报告在 `playwright-report/`，均不提交。若异常强杀留下 `.local/stack.lock`，必须先确认原运行器已退出，再移除过期锁；不要按端口杀进程。

## 3D 客户端与依赖

客户端需要 WebGL2，使用 Three.js 0.186.1 的真实透视场景、光照/阴影和立体角色。地面世界的服务器 X/Y 映射为场景 X/Z，不宣称已经实现跳跃或垂直物理；场景装饰不新增碰撞规则。无 WebGL2 或图形上下文丢失会明确提示并停止会话，不用 2D 或截图代替。

官方模块与 MIT 许可原样保存在 `client/web/vendor/` 并内嵌到 Go；没有 CDN 或额外 bundler/dev server。升级精确依赖后显式执行 `npm run vendor:sync`；`npm run vendor:check` 和默认源码检查只核对字节，不修改文件。第三方文件不手改、不全量格式化；选择、开销与回退见 [ADR-0015](docs/decisions/0015-browser-3d.md)。

## 代码与边界

- `cmd/login`：HTTP 账号注册/密码登录、限频与内嵌页面。
- `internal/credentials`：账号规范化、受限 Argon2id 和 db 专属 MongoDB 凭据存储。
- `cmd/gate`：原始 TCP 与 WebSocket JSON v1，认证和转发，不模拟游戏。
- `cmd/lobby` / `internal/world`：单世界权威在线、移动、聊天与采集。
- `cmd/db`：统一 MongoDB 账号写入。
- `client/web/view3d.js`：3D 场景、镜头、立体角色/资源与 GPU 生命周期；不决定游戏结果。
- `cmd/infra-check`：一次性依赖协议就绪工具，不是第五个业务服务，不写账号或游戏数据。
- `internal/session`：Redis key、单次凭据与会话生命周期。
- `internal/protocol` / `transport`：内部 TCP 16 字节大端头 + protobuf。
- etcd 注册发现；当前世界要求恰好一个 ready lobby，不支持多世界路由或 AOI。

配置在 `configs/local/`，字段见 [配置说明](configs/README.md)。当前只验收本机联机；正式账号、长期养成、跨服、生产安全和规模测试不在本轮。

## AI 与成长历史

当前实际结果见 [最终验收](docs/verification.md)，阶段提交索引见 [成长历史](docs/history.json)。GitHub Actions 已配置 master 和开发分支检查。基线 3ea3baf 的 [CI 39616755008](https://github.com/xiaoyin001/game-shelf/actions/runs/39616755008) 已核对 source、playable 与提交检查汇总全部成功。账号密码与 3D 阶段已独立提交并通过本地真实服务，真实双人 3D 截图已查看；随后密码输入改为 6–32 位 ASCII，Windows/Linux 最新真实服务/协议与浏览器各 11/11 通过，Go race 与清理通过，见 [ADR-0016](docs/decisions/0016-password-input-policy.md)。新增提交未 Push，不把基线 CI 或本地通过冒称新版本远端通过。

先读 [AGENTS.md](AGENTS.md)、[产品目标](docs/playable-mvp.md)、[技术决策](docs/decisions/README.md) 与 [成长记录](docs/progress.md)。每次决定记录原因、候选、代价与验证；替换新增记录，保留旧理由与迁移/回退。

开发只在功能分支，master 接收完整通过后的快进交付，不使用 main。每个阶段测试通过立即 commit，不积攒到最后才拆分；旧实现仅作验收参照。没有常驻 AI 代理、自动付费调用或远端发布；Push、PR、远端配置与部署分别授权。
