# 配置说明

`configs/local/**/*.json` 是各服务启动配置，`configs/ci-profile.json` 是验收运行器配置。JSON 不支持注释，字段说明集中写在这里。

## 目录

- `configs/local/development.json`：基础设施烟测使用的独立身份与依赖连接配置。
- `configs/local/login.json`、`gate.json`、`lobby.json`、`db.json`：当前四个本地进程的实际配置，监听均为本机回环地址。
- 地址、监听与数据库连接使用本机参数；依赖默认仅开放回环端口。

## ci-profile.json

本地 `verify:ci` 与 GitHub共用，不影响普通dev。`version`是报告/配置格式版本；`node`、`go`、`playwright`是精确运行版本，必须同时匹配固定SDK构建配方和锁文件。source保留CGO=1并执行race，playable为CGO=0。

- `cpus`：工具容器CPU额度及CPU亲和个数，当前2；运行时读取cgroup，不以主机核心数代替。
- `memoryBytes`：工具容器内存上限，字节，当前4GiB；memory与memory+swap同额，实际swap必须0。Docker宿主至少6GiB，给独立数据库留空间。
- `pids`：每个工具容器进程/线程上限，当前1024。
- `rounds`：本地默认完整preflight必须成功的浏览器轮数，当前2：先完整回归，再在新DB/新四服务重复全部浏览器，不是失败重试；第二轮不重复源码/原始协议/故障注入。每轮DB空卷，工具缓存仅在同job复用。
- `ciRounds`：CI/单job完整回归轮数，当前1，执行全部原test:ci；单job不能生成推送记录。避免把本地额外压力轮重复放到远端挤占时限。
- `browserTimeoutMs`：CI/单job真实浏览器硬上限30000ms，匹配Playwright；本地完整preflight另用maxBrowserMs留余量。
- `receiptMaxAgeMs`：提交绑定记录有效期，当前86400000ms（24小时），不表示环境24小时必然稳定。
- `pushBranches`：门禁允许的工作流覆盖分支模式；与YAML由运行器测试核对，不允许main、标签或删除被普通门禁隐式放行。
- `minBrowserTests`：当前必须通过的最少浏览器用例数19；零跳过、零重试、零全局错误。
- `maxBrowserMs`：本地完整preflight每例实际耗时上限27000ms，另保留Playwright硬上限30000ms，留出至少3秒余量；不是把CI业务硬上限偷偷降到27秒。
- `prepareMs` / `runMs` / `jobMs`：毫秒，分别600000/720000/1500000；YAML独立step/job期限由运行器测试检查一致。
- `cleanupReserveMs`：runMs中预留退出/取证时间，当前60000ms，不额外加到12分钟上。

UID/GID不写死在JSON：Linux取调用进程身份，Windows取实际WSL身份；socket组取本机socket的stat值。SDK只用该UID/GID和必要socket组，HOME为本job私有缓存；短命初始化容器只设置新缓存卷根/home/node_modules的属主，不改工作副本或共享目录。node_modules按volume-subpath挂载，不读取宿主依赖，两轮仍各自npm ci；卷在job开始为空、结束定向回收，见[ADR-0028](../docs/decisions/0028-ci-dependency-io.md)。环境/推送报告要求SDK UID/GID与本轮执行身份一致。

数据库资源上限在仅CI使用的Compose覆盖里：Mongo0.5CPU/1GiB、Redis0.5CPU/128MiB、etcd0.5CPU/256MiB；普通开发配置与数据卷不改。实际宿主、容器资源、SDK、系统包和浏览器字节信息写入每轮环境证据，见 [ADR-0023](../docs/decisions/0023-shared-ci-preflight.md) 与 [ADR-0024](../docs/decisions/0024-headless-ci-image.md)。

## service

- `name`：服务名，用于日志、健康检查和服务发现。
- `instance_id`：实例 ID，多开时必须唯一。
- `env`：运行环境，例如 `local`、`private`、`prod`。
- `version`：服务版本号。
- `zone`：逻辑区域或机房标识。
- `gray`：是否为灰度实例。
- `weight`：服务发现负载均衡权重。

## public

- `listen_addr`：暴露给客户端的监听地址。
- `advertise_addr`：返回给客户端的连接地址。

只有 `login` 和 `gate` 默认使用 `public`。

## websocket

仅 gate 使用。`listen_addr` 为浏览器监听地址，空值禁用；`advertise_url` 为返回客户端的完整 ws(s)://host:port/ws 地址；`origins` 是明确的页面 host:port 列表，不含 scheme 或通配符。启用时要求单端会话。当前返回本机回环地址，不承诺跨机接入；监听配置变更需要重启。

## internal

- `listen_addr`：服务间 TCP 通信监听地址。
- `advertise_addr`：注册到 etcd 后供其他服务连接的内网地址。

## health

- `listen_addr`：健康检查 HTTP 监听地址。

健康检查端口可暴露给内网监控，不对公网暴露。

## discovery

- `base_path`：etcd 服务注册根路径。
- `etcd`：etcd endpoint 列表。
- `lease_ttl_seconds`：服务实例注册 lease TTL。

## dependency

- `check_interval_ms`：依赖检查间隔。
- `startup_grace_ms`：启动期最多等待依赖就绪的时间。
- `unavailable_exit_after_ms`：运行期依赖持续不可用后退出进程的时间。

## heartbeat

- `client_interval_ms`：客户端连接心跳发送间隔。
- `client_idle_timeout_ms`：客户端连接最大空闲时间。
- `internal_interval_ms`：内部服务连接心跳发送间隔。
- `internal_idle_timeout_ms`：内部服务连接最大空闲时间。

当前连接级心跳协议已经预留，连接接受方只响应心跳，连接发起方负责主动检查。

## limit

- `max_body_bytes`：单个协议包 body 最大字节数。
- `max_conn`：服务实例最大连接数。

## session

- `login_token_ttl_ms`：login token 有效期。
- `online_session_ttl_ms`：在线 session 续期 TTL。
- `reconnect_ttl_ms`：断线重连窗口。
- `multi_login`：是否允许同账号多端登录。

## shutdown

- `timeout_ms`：优雅下线最大等待时间。
- `gate_kick_message`：gate 停服或踢人时返回客户端的原因。

## redis

- `addr`：Redis 地址。
- `password`：Redis 密码，开发环境可为空。
- `db`：Redis database 编号。

Redis 连接、key、TTL 和 session 读写必须通过公共模块封装。

## mongo

- `uri`：MongoDB 连接串。
- `database`：业务数据使用的数据库名。

MongoDB 读写必须通过 `db` 服务收敛。

## log

- `level`：日志级别，支持 `debug`、`info`、`warn`、`error`。
- `output`：日志输出方式，支持 `stdout`、`file`。
- `file_path`：文件日志路径。
- `max_size_mb`：单个日志文件最大尺寸。
- `max_backups`：日志滚动后保留文件数量。
- `max_age_days`：日志文件最长保留天数。
- `compress`：是否压缩滚动日志。

## config

- `watch_interval_ms`：本地 JSON 配置热更新轮询间隔。
