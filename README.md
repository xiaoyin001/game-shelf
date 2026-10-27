# game-shelf · 苔光原野

同仓提供 Go 服务端和浏览器 3D 联机沙盒：在同一地图相遇、移动、跳跃、实体碰撞、聊天、共享采集、NPC 委托、按账号保存的两步寻宝/收藏册与微光展台。代码和 AI 规范从空仓库逐段实现、验证、提交，过程见 [成长记录](docs/progress.md)，不是拆分旧大提交伪造的开发历史。

## 本地体验

需要 Go 1.25+、Node.js 22+ 和 Docker Compose；Windows 可使用 WSL 中的 Docker。

```text
npm ci
npm run dev
```

打开 **http://127.0.0.1:11000**，先填写账号与 **6–32 位密码**（仅英文字母、数字和英文符号，不含空格；中文、表情与非法粘贴拒绝），点击“注册并进入原野”；以后使用“登录并进入原野”。用两个窗口、不同账号进入：
- WASD / 方向键相对镜头移动，**空格跳跃**，最终位置、重力和碰撞由服务器决定；空中不能再次起跳，按住不自动连跳。
- 站稳后步行会自动迈上每级高度差 **不超过18单位** 的静态矮台阶，脚底到达实体顶面，不穿石；现有矮石和篝火低矮实体可直接走过，连续台阶按每级高度差判断。高于阈值的台阶、晶簇、帐篷和树干仍阻挡，需要跳跃或绕行；玩家/NPC不能当自动迈步台阶。头顶没有足够空间时不能硬挤上去，空中不自动抬高，走离支撑后真实下落。
- 右键拖动环绕镜头、滚轮缩放、R 复位；树林、营地、湖面、旅人和晶簇均为真实 3D 场景。
- Enter 聚焦世界聊天，所有在线旅人可见。
- 靠近发光晶簇按 E 采集，获得本轮分数；共享资源 10 秒后重生，附近提示显示倒计时。
- 靠近 **守灯人阿苔（540 / 500）** 按 E 或点“交谈”，接取“点亮营灯”：接取后采集两次晶簇，再回来领取额外 **3 分微光**；侧栏显示进度。**巡林员林哨（700 / 230）** 提供营地/晶簇提示。NPC 有实体，金色名牌与地面标记，不计入玩家人数。
- E 在 NPC 附近优先交谈，其他地方仍为采集；Esc 或“结束交谈”关闭面板，对话时停止移动/跳跃。委托每轮只能完成一次，不消耗分数；退出/顶号/重启重置，对话与进度仅自己可见，奖励分数跨端可见。
- 与阿苔或林哨交谈，选择“寻找原野旧物 · 领取 / 回顾线索”，依次寻找**营灯旧铃、湖畔来信、林间风笛**。每条先调查线索再找到旧物；靠近私人叶纹标记按 **F** 或点“调查线索”。风笛需要跳上西北晶簇并站稳，E 原交谈/采集不变。
- 收藏册显示线索、可展开的方向帮助和已发现的故事。线索步骤与收藏由 db 同步保存到 MongoDB，退出/顶号后恢复；存储不可用时明确拒绝确认，不覆盖为空档。没有每日限制、争抢或失败扣分。首版只有三条固定故事，不宣称无限内容或已验证长期留存。
- 每件旧物分别解锁**归途铃灯、湖色花盏、晶叶风铃**。切换右侧“微光展台”，站稳在营地展台 **730 / 530** 附近，可在左/中/右三个槽位布置或收起；同一摆件放到新位置会自动移走旧投影，不消耗分数。布置随账号保存。
- “查看谁的展台”可参观任一在线旅人的真实3D布置。每次只在同一营地位置观看一人的**无碰撞微光投影**，不是独立房屋、永久地块或自由建造；参观不获得编辑权限，离线旅人暂不可访问。
- 同账号通过正确密码验证后顶替旧连接；错误密码不影响原会话。离开后需重新输入密码登录。

运行器先编译当前工具和全部四服务，必要时启动本地依赖，再用同款 Go 驱动验证 MongoDB/Redis/etcd 的真实协议就绪，最后统一启动业务进程，日志在 `.local/logs/`。编译不占用服务启动期限，TCP 端口接通不能单独放行；依赖或起服失败会保存本项目容器状态（含退出码/OOM）、端口、实际镜像 digest 与日志到 `.local/logs/infra.log`。Ctrl+C 只回收自建服务和保活管道；端口被占用时拒绝接管，不清库、不删卷。WSL 依赖可能随宿主空闲停止，下次运行会再启动，命名卷保留。

默认公布回环地址，只验收同机联机。注册和登录都验证密码，db 只保存带随机盐的 Argon2id 慢哈希，入口有限频；但当前是可信本机 HTTP，**不能直接开放互联网或复用其他网站密码**。旧 players 试玩记录保留，同名新注册不会继承旧身份，见 [ADR-0014](docs/decisions/0014-password-authentication.md)。本轮位置、分数、营灯委托、资源与聊天是临时沙盒，离开/重启重置；基础账号、私人寻宝收藏与展台布置经 db 保存到 MongoDB。这不是成熟 MMO、大规模容量或生产账号安全承诺。

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

### 共用有界 CI 预验收

本地和 GitHub 共用运行器、SDK及资源配置；本地完整preflight额外加一轮浏览器/余量检查，GitHub单job保留全部原回归一次。不再以 `test:all` 或仓库外脚本作为推送充分证据：

```text
npm run verify:ci
```

需要 Node.js 22+、本机 Linux x64/AVX Docker（至少2CPU、6GiB、cgroup v2），以及支持 `--wait` 的 Compose；Windows入口使用WSL Docker。[共用工具构建配方](deploy/ci-tools.Dockerfile)使用固定digest的SDK/CGO工具，Ubuntu官方冻结快照的图形库/字体，以及锁文件Playwright安装器的唯一headless shell，不改主机系统库。source执行规则/历史/Go/客户端与全量CGO race；playable完整`test:ci`包括真实三依赖/四服务/协议、全部浏览器用例（当前21例）及预期故障清理。本地默认完整preflight再在另一新DB/新四服务上额外重复全部用例，任一失败整次失败，不是失败重试；额外轮不冒称再次执行源码/原始协议/故障注入。

每个工具容器实际限制 **2CPU/4GiB/无swap** 并绑定两个CPU；SDK运行UID/GID沿用实际宿主（Windows取本次WSL身份），仅playable添加实际Docker socket组，HOME位于本job私有缓存。新缓存卷的短命root初始化只设置该卷根/home/node_modules属主，不挂工作副本、不改原仓库权限；数据库也有资源上限。每job从空npm/Go模块/编译缓存开始，node_modules也只位于该job私有卷、两轮仍各自npm ci，不读取宿主依赖；第二轮只复用本job已校验的工具缓存，数据库每轮仍为独立project/空卷。固定镜像层可缓存并记录，不清全局镜像。每job准备10分钟、执行12分钟（预留1分钟退出取证）、总预算25分钟；CI浏览器仍30秒、零重试、单worker；本地完整preflight另要求所有用例不超过27秒，留3秒余量，不把本地余量阈值当远端业务失败条件。单一参数见 [CI配置](configs/README.md#ci-profilejson)。

默认只接受干净提交；提交前先完整暂存目标切片，再用 `npm run verify:ci -- --candidate` 验证暂存tree。候选通过不等于某个新提交已通过，正式推送前还需验证最终commit。源码在不含源仓库Git配置/凭据的临时Git副本运行，报告绑定commit/tree/配方与实际资源。证据在 `.local/logs/ci-verify-<随机ID>/`，按job/轮次保存，正常/失败/超时都定向回收本轮工具容器、缓存卷和数据库资源并检查零残留。主机强杀/断电不保证自动回收，不能按端口杀进程或清共享库。

本地显式HTTP代理仅用于本任务下载/子进程。有显式HTTP代理时，使用校验过的regctl导出同一固定digest的官方镜像；Windows回环代理另通过主机主动建立的反向通道临时桥接，WSL代理入口仅在回环，回传通道限制网关源IP和一次性令牌；不重启daemon、不改系统代理/防火墙、不接受带凭据的代理。首次冷准备包含SDK与真实浏览器下载。完整官方Playwright基底因891.7MiB下载超10分钟已放弃，改为仅安装实际运行的headless shell，见 [ADR-0024](docs/decisions/0024-headless-ci-image.md)；准备失败也属于阻止推送的失败，不拿旧私有工具镜像冒充可重建。

`test:ci`保留为镜像内部/合格Linux环境的底层回归入口，不单独证明资源/期限门禁通过。三依赖仍由 [独立配置](deploy/docker-compose.ci.yml) 固定digest；health后再做真实Go协议检查，端口占用拒绝复用/接管，不自动停止试玩。只回收本轮资源，普通dev数据保留。当前工具容器仍共享宿主内核，Mongo组合继续要求 **<6.19**，容器不能复刻Azure内核/网络/宿主负载；最终必须观察同一SHA的GitHub结果。宿主22.04退役和迁移要求见 [ADR-0013](docs/decisions/0013-mongodb-kernel-compatibility.md)，本次取舍见 [ADR-0023](docs/decisions/0023-shared-ci-preflight.md)。

Windows长命令以控制管道绑定WSL进程组，超时不只结束wsl.exe；实际父子进程回收由Windows专用测试验证，Linux明确不运行这个专有分支。

共用入口阶段22本地候选验收通过：完整source/race，完整19/19及额外新DB19/19，最慢20.54s/21.76s、实际资源与零残留已核对。首次图形工具准备4m48（SDK层已有缓存），不称全部工具无缓存下载。新入口GitHub仍须在推送后核对同一SHA，不以旧3e65411全绿代替。

### 非特权 Linux 属主回归

8ffe6aa的云端源码/race与19/19已通过，但root SDK生成文件令普通runner删除临时目录时EACCES，两个job最终失败；不能把业务通过写成CI全绿。Windows/NTFS未覆盖这个属主限制，见 [ADR-0026](docs/decisions/0026-ci-runner-ownership.md)。

工具准备后可在本轮独立Linux卷、普通UID1001父进程上复跑真实job（原仓库属主/开发卷不改）：

```text
node scripts/test-ci-owner.mjs --candidate --jobs source,playable
```

历史故障复现用 `--ref 8ffe6aa --jobs source --expect-eacces`；“成功复现EACCES”不是修复通过，也不能产生推送记录。Linux父进程/SDK UID分别实际记录；单独Windows删除成功或root Linux运行不能替代该项。

### 提交绑定的推送门禁

每个克隆首次显式安装本仓库hook（只改local配置，已有其他hook拒绝覆盖）：

```text
node scripts/install-hooks.mjs
```

提交前候选验收通过不能放行；创建最终提交、确认工作树干净后运行`npm run verify:ci`。只有默认完整preflight全部通过并回收成功才自动生成 `.local/push-verification.json`：绑定最终commit/tree、仓库、配置/配方、完整报告SHA256和时间，24小时有效。`node scripts/prepush-guard.mjs --check`只读检查该记录，不触发Push。

任何新的验收尝试先失效旧记录；candidate、准备、单job、失败或旧SHA都不能恢复有效记录，历史报告保留。pre-push快速核对实际待推送refs，不隐式跑长测试：只接受已验收HEAD、相同仓库、CI覆盖分支及可确认快进；删除/标签/强推等不是普通门禁授权范围。门禁是防漏检，不是第三方签名或防恶意绕过的安全边界，也不授予额外远端权限。owner已于2026-10-26持续明确授权本仓库任务相关、完整验收通过的**普通快进Push至origin/master**，后续读取本仓库规则的会话无需逐次询问；仍展示提交/仓库/目标/结果并核对同SHA CI。强推、删除/标签、其他仓库/分支、PR、远端配置、远端CI重跑/取消和部署另行授权，撤回/超范围则停止。技术门禁见 [ADR-0025](docs/decisions/0025-push-verification-gate.md)，授权更新见 [ADR-0030](docs/decisions/0030-standing-push-consent.md)；不声称同步到无关仓库。

`.local/ci-verification.lock`记录本次验收PID/平台。运行中或旧锁未确认时拒绝推送；异常强杀后先核实所属进程已结束再处理过期锁，不自动清理未知任务或按端口杀进程。

截图/trace 在 `test-results/`，HTML 报告在 `playwright-report/`，均不提交。若异常强杀留下 `.local/stack.lock`，必须先确认原运行器已退出，再移除过期锁；不要按端口杀进程。

## 3D 客户端与依赖

客户端需要 WebGL2，使用 Three.js 0.186.1 的真实透视场景、光照/阴影和立体角色。活动时以30fps为绘制目标，静止/表单阶段12fps继续真实绘制；角色位置/高度、镜头或尺寸变化会恢复活动预算并保留750ms插值余量，网络输入/轮询仍为100ms。这是调度目标，不保证所有硬件的实际帧率。服务器 X/Y/Z 映射为场景 X/Z/Y，客户端只插值高度，不自行决定轨迹或碰撞。服务端简化圆柱/盒体与绘制共用 `pkg/scene/layout.json`，以只读 `/scene.json` 提供，版本不匹配必须刷新；见 [ADR-0017](docs/decisions/0017-authoritative-jump-collision.md)。这不是完整网格刚体引擎，没有推箱、载具、物体破坏或游泳。无 WebGL2 或图形上下文丢失会明确提示并停止会话，不用 2D 或截图代替。

官方模块与 MIT 许可原样保存在 `client/web/vendor/` 并内嵌到 Go；没有 CDN 或额外 bundler/dev server。升级精确依赖后显式执行 `npm run vendor:sync`；`npm run vendor:check` 和默认源码检查只核对字节，不修改文件。第三方文件不手改、不全量格式化；选择、开销与回退见 [ADR-0015](docs/decisions/0015-browser-3d.md)。

## 代码与边界

- `cmd/login`：HTTP 账号注册/密码登录、限频与内嵌页面。
- `internal/credentials`：账号规范化、受限 Argon2id 和 db 专属 MongoDB 凭据存储。
- `cmd/gate`：原始 TCP 与 WebSocket JSON v1，认证和转发，不模拟游戏。
- `cmd/lobby` / `internal/world`：单世界权威在线、移动、聊天、采集与 NPC 对话/委托。
- `cmd/db`：统一 MongoDB 账号与探索存档写入；`field_books` 用版本 CAS 与落盘确认保存私人收藏。
- `pkg/scene`：唯一的场景实体/碰撞布局；Go 内嵌并由 login 只读提供给客户端。
- `client/web/view3d.js`：3D 场景、镜头、立体角色/资源与 GPU 生命周期；不决定游戏结果。
- `cmd/infra-check`：一次性依赖协议就绪工具，不是第五个业务服务，不写账号或游戏数据。
- `internal/session`：Redis key、单次凭据与会话生命周期。
- `internal/protocol` / `transport`：内部 TCP 16 字节大端头 + protobuf。
- etcd 注册发现；当前世界要求恰好一个 ready lobby，不支持多世界路由或 AOI。

配置在 `configs/local/`，字段见 [配置说明](configs/README.md)。当前只验收本机联机；包含有限的账号收藏与固定槽位展台存档；新服务兼容旧schema1并在合法写入时升级为2，未知版本拒绝而不清零。旧31版本不能继续写入2档，回退边界见[ADR-0032](docs/decisions/0032-collection-camp-display.md)。公网账号、数值型长期养成、跨服、生产安全和规模测试不在本轮。

## AI 与成长历史

当前实际结果见 [最终验收](docs/verification.md)，阶段提交索引见 [成长历史](docs/history.json)。GitHub Actions 已配置 master 和开发分支检查。基线 3ea3baf 的 [CI 39616755008](https://github.com/xiaoyin001/game-shelf/actions/runs/39616755008) 已核对 source、playable 与提交检查汇总全部成功。账号密码与 3D 阶段已独立提交并通过本地真实服务，真实双人 3D 截图已查看；随后密码输入改为 6–32 位 ASCII，Windows/Linux 最新真实服务/协议与浏览器各 11/11 通过，Go race 与清理通过，见 [ADR-0016](docs/decisions/0016-password-input-policy.md)。最新跳跃/碰撞阶段已通过 Windows/Linux 真实服务/协议与各 14/14 浏览器、Go race 及清理，角色首帧与短暂高度采用实际只读观察验收。上述4个提交已普通推送至 a5963e3；[CI 39741132203](https://github.com/xiaoyin001/game-shelf/actions/runs/39741132203) 的 source 成功、playable 仅采集用例在收尾时超过30s，其余13项及真实协议通过。采集原页重进/碰撞等待已按 [ADR-0018](docs/decisions/0018-browser-gather-ci.md) 在 Windows 与双CPU Linux 各14/14验证；这些是当次本地结果；后续已推送并完成云端复验，见本节最后的3e65411记录。

NPC 阶段已在独立 Linux 网络命名空间与空依赖上完成真实四服务、独立协议、全部 **17/17** 浏览器及 Go race 回归；Windows 源码/客户端检查通过，原生 Windows 全套联机未重跑。包含两位实体角色、选项对话、营灯委托与唯一奖励，详见 [ADR-0019](docs/decisions/0019-npc-interactions.md)。最后 race 发现的 protobuf 标量 presence 问题已用官方补丁修复并整套复验，见 [ADR-0020](docs/decisions/0020-protobuf-scalar-presence.md)。上述是该阶段本地验收记录；随后已按授权推送至516a117并正常重启试玩，远端未全部通过，详见下面CI修复记录。角色为预设对话，无巡逻、生成式 AI 或长期任务存档。

低台阶迈步阶段已通过 Windows 源码/客户端检查、Linux 隔离真实四服务与全部 **18/18** 浏览器、独立协议回归和全量 Go race；原17项保持不变。当前每级步高18，脚底到真实顶面、走离自然落下，详见 [ADR-0021](docs/decisions/0021-authoritative-step-up.md)。原生Windows完整联机未跑。516a117已按授权推送，云端source成功、playable超时取消；正常重建重启本地四服务加载新规则，不更改账号数据。

最新CI修复见 [ADR-0022](docs/decisions/0022-ci-render-budget.md)：原版在双CPU真实环境复现17/18、NPC超30s；修复后双CPU真实四服务/协议及 **19/19（3.9m）**、独立服务回归和Go race通过，30s/零重试不变。新增独立重生验证，保留完整委托和presence整页重进。后续3e65411已按授权推送，[CI39792750622](https://github.com/xiaoyin001/game-shelf/actions/runs/39792750622)的source/race和playable全部SUCCESS，云端19/19（3.0m）、正常/故障容器/卷/网络均0，制品已完整核对。这是原执行方式下该次提交的结果，不代表本阶段共用入口已通过；原生Windows新版全套仍未跑。

先读 [AGENTS.md](AGENTS.md)、[产品目标](docs/playable-mvp.md)、[技术决策](docs/decisions/README.md) 与 [成长记录](docs/progress.md)。每次决定记录原因、候选、代价与验证；替换新增记录，保留旧理由与迁移/回退。

开发只在功能分支，master 接收完整通过后的快进交付，不使用 main。每个阶段测试通过立即 commit，不积攒到最后才拆分；旧实现仅作验收参照。没有常驻 AI 代理、自动付费调用或部署；本仓库普通快进Push按AGENTS.md的持续明确授权执行，PR、其他远端写入和部署仍单独授权。
