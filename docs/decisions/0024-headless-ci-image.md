# ADR-0024：用冻结Ubuntu依赖和唯一headless shell替代完整Playwright工具基底

状态：采用（2026-10-23，本地首次图形镜像准备及完整候选回归通过；新SHA云端尚未验收）。替换 [ADR-0023](0023-shared-ci-preflight.md) 的playable完整官方Playwright基底，保留共用入口、资源、原SDK、两轮真实验收、期限与正常/故障定向回收。

## 背景
ADR-0023首次daemon直连固定Node镜像拉取10分钟超时；客户端OCI导出已实际在本地完成同一固定Go/Node/CLI内容导入并构建source镜像，未修改共享daemon。随后完整Playwright1.64.1-jammy的891.7MiB压缩层在单独10分钟准备期限仍未完成，已记录失败，不加时或把残缺文件当成功。该基底还包含实际不运行的headed Chromium、Firefox和WebKit。

网络探测另确认：Windows回环代理不能从WSL直连，Windows虚拟网卡入站也超时；任务内反向通道已用真实WSL curl拿到registry正常401认证响应并正常结束，无需改防火墙。这是网络探测，不是浏览器或完整CI通过。

## 比较
- 保持完整官方Playwright基底：OS/图形依赖固定简单，但本地实际冷准备超10分钟且下载无用浏览器；当前不采用。
- 延长安装时限/反复暖缓存后声称冷安装通过：不能闭合冷准备缺口，不采用。
- 用现有私有prepared镜像直接验收：克隆仓库无法重建，延续原问题，不采用。
- 固定官方Ubuntu22.04 manifest + 冻结日期的官方apt快照 + 锁文件Playwright的唯一headless shell：采用，只删除未运行的下载，不删除任何测试或真实WebGL能力。实际体积、下载/构建耗时待测。

## 决定
1. source仍是固定官方Go1.25.0-bookworm（真实GCC/CGO/race）及Node22.23.3。playable改为Ubuntu22.04 amd64官方manifest `sha256:98510ac07bb4545e389be502a8918b385c018bfe4ceeb5ac7e0375c782268b04`；SDK与Docker CLI从原固定digest阶段复制，不换运行工具版本。
2. 图形库/字体来自官方Ubuntu apt快照 `20261021T000000Z`；固定历史快照有意关闭Release新鲜度期限，但保留HTTPS、Ubuntu签名与包哈希验证，不允许未认证包。CA bundle从固定Go SDK基底复制以启动HTTPS验证；依赖补丁升级需主动更改快照并整套复验。
3. Docker构建上下文只包含配方与本仓库package.json/package-lock.json，执行原生Playwright安装器 `--with-deps --only-shell chromium`。所有NPM版本/完整性由锁文件决定；`npm ci --no-audit --no-fund`保留全部安装和完整性检查，但不附带无独立期限的公告/资助网络请求，不冒称执行了漏洞审计；Go/Node/浏览器版本在运行时再校验。配方hash同时包含两个package文件，避免工具图像偷偷保留旧锁文件。浏览器实际revision/digest与系统包清单记录在环境证据，不把可变CDN URL宣称内容哈希锁定。
4. 有主机显式代理时先构建固定source目标，使用其中校验过的Linux Node在本轮WSL目录启动反向通道；playable构建使用host网络和标准临时proxy build-args，代理值不进入Dockerfile/报告，构建后关闭本轮通道。云端无代理时仍用同一Dockerfile和安装器直连。整个准备仍10分钟；source/bootstrap依赖包含在期限里，不额外放宽。
5. 每job工具缓存是新建、本轮专用卷，两轮间只复用已校验的工具缓存；DB每轮独立空卷。普通开发数据、共享系统库、daemon和防火墙不更改；不会发布远端工具镜像。

### 两轮范围的预算复核（实现前补充）
共用入口实测source/race160.3s通过；第一轮完整test:ci390.5s、19/19（3.6m，最慢20.67s）、正常/故障零残留。第二次再跑完整test:ci时达到11分钟运行部分期限，已通过15/19后被实际中断，外层仍确认两个project及缓存零残留；整次记失败，不能择取第一轮声称全套通过。

比较“重复全部源码/协议/故障”与“完整回归一次，额外重复已出现时限问题的真实浏览器”：重复检查未提供新的行为覆盖，却挤占浏览器余量。因此调整为第一轮保留全部原test:ci与故障清理，第二轮在另一新DB/新四服务上额外执行全部浏览器，仍19例、30秒/零重试/≤27秒判据；明确标记`browser-repeat`，不把未重复的Go/协议/故障宣称为再次通过。不扩大12分钟，不跳过任何原有回归，第二轮也必须通过才能放行；这不是失败后自动重试。

## 验证
验证冷准备、Node/Go/Playwright实际版本、CGO分别为1/0、cgroup2CPU/4GiB/无swap、完整source/race、完整test:ci及额外一轮新DB真实浏览器。查看中文与3D截图，核对19项无跳过/重试、每例≤27秒以及正常/故障/外层中断零残留。首次安装失败照实记录，全部实际通过才可提交，Push另行授权。

### 实际结果
新图形镜像首次构建288.1s（Node/Go/CLI SDK层已有缓存，Ubuntu导入、冻结图形库/字体和headless shell是新准备）；不把它称为所有SDK/镜像层完全无缓存下载。source约157.7s通过全量CGO race；完整test:ci与额外新DB浏览器分别19/19（3.6m/3.7m），最大20.541s/21.756s，实际2CPU/4GiB/无swap，两轮浏览器SHA256均为`4fa61f0ba0b4f89b7bec242c32ca16d3bccf69dbfef0651924a9ba8fede5f9d6`（headless shell1248）。中文、真实3D与委托截图已查看，全部本轮资源定向回收为0。完整MCR基底与重复两次全套的失败仍保留，非统计性能基准。

## 代价与迁移
比官方完整镜像多维护一份最小构建配方和apt快照；不再能声称所有工具字节都仅由基础镜像digest固定，浏览器仍走官方锁版本安装器。快照是验证工具环境，不替代生产安全更新。没有游戏/账号数据迁移；从未成功的MCR路线改用较少下载的路线，不清原缓存或重写失败记录。

## 重评条件
若仍不能在10分钟准备完成，按真实下载/构建阶段分析，不用预置私有prepared镜像假称可重建。若长期维护快照成本过高，再比较经授权发布的专用镜像或恢复完整官方基底，必须保留真实回归与失败记录。
