# 分阶段成长记录

本轮从空仓库重新构建，不是旧版历史重排。每条记录由对应阶段实际运行后写入，不预填后续完成状态。

## 当前阶段

10 个阶段已推送 GitHub master。首次 CI 的 source 成功、playable 因 MongoDB 不可达起服失败；独立修复分支已完成真实就绪、固定 CI 环境、先编译后启动与定向回收。本地两轮 Linux 新卷冷启动、故障路径、源码/race 和 Windows 兼容回归通过；GitHub 最终提交的 source/playable 仍待新 Push 授权后实跑，目标尚未闭环。

## 阶段记录

### 01 · 建立 AI 规范与规则检查

- 已实现：跨 AI 入口、分支/阶段提交规则、产品验收目标、ADR 模板及文档入库检查。
- 实际验证：`npm run check`，4 个规则测试全部通过；Git 的“无忽略项”返回码按其真实语义处理。
- 当前仍没有 Go、服务进程或客户端；未把编译、联机或旧版测试写成通过。
- 提交：`39ae07e`，完成后立即提交，日期使用 Git 当前时间。

### 02 · 可单独运行验证的通信基础

- 已实现：Go 模块、大端 16 字节帧与 protobuf、请求序号关联、调用截止时间及取消时回收连接；没有加入业务契约。
- 实际验证：`npm run check` 通过规则检查与 Go 格式、测试、vet、构建；回环 TCP 客户端/服务端真实收发成功。
- 异常回归：短帧、超大包、protobuf 往返、短 context 截止时间与空闲连接取消全部通过。
- 当前无登录服务、数据库连接或游戏，不把库测试称为完整联机。
- 决策：[ADR-0002](decisions/0002-go-transport.md)，提交 `2a3b545`。

### 03 · 真实基础设施与会话边界

- 已实现：统一配置、JSON 日志、发现/租约/健康与依赖状态模块、MongoDB/Redis 连接、单次 token 与原子会话替换。
- 实际验证：`make check` 通过；`npm run test:infra` 在真实 etcd、MongoDB、Redis 上通过注册发现、驱动连接、消费/过期/重放拒绝和旧会话清理保护。
- Windows WSL 保活直接复用已知经验，本阶段没有重演旧故障或沿用旧运行结果；没有清库、删卷或接管其他任务服务。
- 当前仍未加入 HTTP 登录、网关或游戏界面。
- 决策：[ADR-0003](decisions/0003-local-infrastructure.md)，提交 `230c42d`。

### 04 · 登录、网关、大厅与账号保存

- 已实现：四服务启动、真实 ready 依赖状态、HTTP 开发登录、MongoDB 账号保存、单次 token、TCP 认证/心跳/大厅列表。
- 当前大厅只提供开发列表，不使用永久 Redis 集合假装在线人数；世界在线状态尚未实现。
- 实际验证：`make check`、`npm run test:services` 与 `npm run test:runner` 均通过；真实账号保存唯一且重复登录身份稳定；占用端口时原监听不被接管或杀死。
- 运行器每次构建当前代码，测试后回收自建进程与锁。不把 HTTP/TCP 通过写成浏览器可玩。
- 决策：[ADR-0004](decisions/0004-service-login.md)，提交 `af8ed4e`。

### 05 · 浏览器共享在线世界

- 已实现：内嵌页面、gate WebSocket JSON v1、lobby 独立权威在线状态，进入/轮询/离开与 5 秒角色租约。
- 新消息仅包含本阶段在线能力，没有提前加入移动、聊天或资源字段。显式 Origin、消息限制、单次凭据和旧连接保护已执行真实回归。
- 实际验证：`make check`、`npm run check:client` 和 `npm run test:all` 通过。真实协议含账号/TCP兼容、顶号/重放、过期/伪造凭据、Origin、身份伪造、超大包、版本和频率限制；浏览器 2/2 通过在线、退出重进、顶号且无 pageerror。
- 当前 UI 是可观察的在线地图，仍不能移动、聊天或采集，不把最终目标写成已完成。
- 决策：[ADR-0005](decisions/0005-browser-presence.md)，提交 `d17dde7`。

### 06 · 服务器权威移动

- 已实现：WASD/方向键输入，100ms 输入、服务器时间预算、斜向归一化、速度/边界限制与失焦清键；只增加本阶段需要的输入字段。
- 实际验证：`make check`、`npm run check:client`、`npm run test:all` 通过；可控时钟测试覆盖高频、闲置、非法方向与边界，浏览器 3/3 通过，真实键盘移动的坐标由另一浏览器观察到。
- 账号、原 TCP、在线与安全回归继续通过；仍无聊天和采集。
- 决策：[ADR-0006](decisions/0006-authoritative-movement.md)，提交 `37ccfac`。

### 07 · 有界世界文字聊天

- 已实现：世界广播、140 字上限、800ms 频率限制、最近 30 条历史、Enter 聚焦与文字节点显示；消息不持久化。
- 实际验证：`make check`、`npm run check:client`、`npm run test:all` 均通过；聊天长度/频率/历史测试通过，浏览器 4/4 通过，HTML 字符串没有生成 img、触发弹窗或 pageerror。
- 既有在线、移动、账号与协议回归保持通过。尚无采集。
- 决策：[ADR-0007](decisions/0007-world-chat.md)，提交 `178cb1a`。

### 08 · 原子共享采集与本轮分数

- 已实现：五个共享晶簇、72 单位采集距离、10 秒共享冷却、服务器原子加分、资源和分数快照；E 只发送动作。
- 实际验证：`make check`、`npm run check:client`、`npm run test:all` 均通过；并发争抢只奖励一次、重复/远程采集拒绝、时间冷却重生与容量防护测试通过；浏览器 5/5 通过，分数跨端可见且退出重进为 0。
- 所有已有在线、移动、聊天、账号/TCP 与安全回归继续通过；没有引入永久资产或清理账号数据。
- 决策：[ADR-0008](decisions/0008-shared-gathering.md)，提交 `081b2a4`。

### 09 · 完成原野界面与窄屏呈现

- 已实现：程序化树林、营地、湖面、晶簇/冷却与角色画面，DOM 名单、分数、聊天及操作提示，达到本轮验收对照的界面形式。
- 实际验证：`make check`、`npm run check:client`、`npm run test:all` 通过，浏览器 6/6 通过；新增窄屏无横向溢出与进入测试，真实双人地图截图已生成并查看。
- 所有原有行为回归保持通过，不把截图代替联机行为；窄屏可见/可进入不代表触屏移动支持。
- 决策：[ADR-0009](decisions/0009-client-presentation.md)，提交 `615b860`。

### 10 · 最终验收与 master 交付准备

- 已实现：新成长序列核对、完整历史 CI 定义、PR 验证入口和当前版本验收报告，不混入私人备份或旧历史。
- 实际验证：`make check`、`npm run check:client`、`npm run test:all`、独立 `npm run test:e2e` 全通过；6/6 浏览器用例重复通过。Linux `go test -race -count=1 ./...` 与四服务构建通过；Linux 完整联机未执行。
- Git 对象确认：第一阶段是独立空根；基础/服务/在线/移动/聊天/采集分别在其真实阶段出现，没有把旧大提交装回祖先。
- 正常回收核对：自建 Go 进程为 0、锁不存在；未清库或删卷。GitHub 当次只读核对为空仓库，推送 master 无需强推，但仍等待最终明确授权。
- 新代码一直在功能分支实现与提交；本阶段提交后由 master 快进接收，不在 master 开发或制造大合并提交。
- 详细结果：[最终验收](verification.md)；决策：[ADR-0010](decisions/0010-delivery-verification.md)。

### 11 · 首次 CI 失败后的依赖就绪配套（本地准备）

- 实际远端：原 10 个提交已完整推送，master 为 `4986865`；[CI 39572389734](https://github.com/xiaoyin001/game-shelf/actions/runs/39572389734) 的 source 通过，playable 起服期间持续 MongoDB connection refused，未进入浏览器测试。
- 已证实原就绪检查只看 TCP，不足以证明数据库可用；容器失败现场缺失，最终原因尚未确认，不猜测换镜像或盲目加超时。
- 当前配套：主机侧同款 Go 驱动协议检查与失败前保存 Compose 状态、镜像和容器日志，保留全部既有行为和测试。
- 实际本地验证：`make check`、`check:client`、`test:all`、独立 `test:infra`/`test:services` 通过；6/6 浏览器用例通过。新就绪检查的假 TCP 服务、重试、超时、取消，以及 Linux race/真实依赖检查通过；诊断文件已验证可保存。
- 修复分支独立提交，不改写已推送的成长历史；远端效果未复验，额外 Push/重跑需另行授权。
- 决策：[ADR-0011](decisions/0011-semantic-dependency-readiness.md)，本地提交 `ed586de`。

### 12 · 可冷启动、可取证和定向清理的 CI 环境（本地完成）

- 已实现：Ubuntu 24.04、Go 1.25.0、Node.js 22.23.3、Playwright 1.64.1；CI 独立 Compose 覆盖，固定三依赖补丁及 digest，唯一 project/新卷；全部程序编译完成后才启动依赖和四服务。
- 实际 Linux：两轮各自使用新 Go 构建缓存、project 和空数据卷的冷启动，规则/历史/格式/Go、真实依赖/协议及全部 6 个浏览器用例均通过；第二轮还通过最终 `test:ci` 入口中的失败清理回归。
- 失败回归：真实三依赖健康后注入检查工具退出 23；保留原始错误与容器 State/OOM/实际 digest/端口/日志，清理后容器、卷、网络均为 0。未把此故障检查器当成真实依赖或正向业务验收。
- 独立 `go test -race -count=1 ./...` 与 actionlint 1.7.7 通过；scope/清理所有权/命令注入和原端口占用保护共 4 个运行器测试通过。
- Windows 普通入口在另建独立依赖/新卷上执行真实四服务、全部协议与 6/6 浏览器回归通过；WSL 诊断参数和内容实测通过。本轮临时资源已回收，原开发三容器仍停止，原三数据卷保留，未用于此次测试。
- 本机下载工具曾遇到超时，改为官方校验的私有 SDK/缓存及局部解包库，未改全局系统库；具体取舍见 ADR。工具准备失败不冒充业务失败或通过。
- 完成边界：这不是 GitHub 全绿证据，旧 CI 的最终 MongoDB 故障原因仍未知。下一步仅在展示最终提交并获准 Push 后，观察同一 SHA 的 source 和 playable；失败继续依据现场修复。
- 决策：[ADR-0012](decisions/0012-reproducible-ci.md)。
