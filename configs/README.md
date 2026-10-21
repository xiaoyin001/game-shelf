# 配置说明

`configs/**/*.json` 是各服务启动配置。JSON 不支持注释，字段说明集中写在这里。

## 目录

- `configs/local/development.json`：当前基础设施烟测使用的独立身份与依赖连接配置；尚无业务进程配置。
- 地址、监听与数据库连接使用本机参数；依赖默认仅开放回环端口。

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
