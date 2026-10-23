# ADR-0026：运行容器沿用宿主UID/GID，补验非特权Linux父进程

状态：采用（2026-10-23，普通Linux父进程先复现旧EACCES，修复候选的真实source/race、完整test:ci/19例通过；最终commit正式记录由提交后命令生成，新SHA云端未验收）。接续 [ADR-0023](0023-shared-ci-preflight.md) 与 [ADR-0025](0025-push-verification-gate.md)，修正工具容器默认root造成的宿主属主差异；不改游戏业务或原验收范围。

## 背景
8ffe6aa已获授权推送。[CI39918335777](https://github.com/xiaoyin001/game-shelf/actions/runs/39918335777) source、playable最终均failure，不能以门禁/推送成功冒充交付成功。完整日志与两份制品确认：
- source工具准备约19s，真实源码/race通过，最后宿主删除`.local/ci-work-.../repo/.local/logs`报EACCES。
- playable工具准备约84s，原协议与19/19浏览器（4.3m）通过，最后删除临时副本`.local/bin`同样EACCES。
- SDK容器默认root，新产生目录/文件归root；GitHub普通runner用户不能清理这些子目录。本地Windows通过NTFS删除，掩盖了Unix属主限制。固定SDK、资源和软件库并不等价于固定父进程权限/文件系统。
- 本机WSL默认UID/GID为1000:1000，Docker socket为0:108、0660。不得据此假定GitHub也是同一数值，更不能只把父进程提权。

## 比较
- 保持root SDK、让宿主sudo rm或忽略清理错误：扩大权限/掩盖残留，不采用。
- 每轮结束把整个临时源码递归chown回宿主：可补救，但正常/失败/超时都需额外特权恢复，容易遗漏，不采用为默认。
- SDK运行时使用实际宿主UID/GID、仅给playable补Docker socket组、新建私有缓存单独初始化属主：采用。文件自创建即归父进程，不碰原仓库/开发目录的属主。

## 决定
1. Linux读取调用进程UID/GID；Windows读取本次WSL用户UID/GID。Docker socket组由本机socket实际stat得出，不写死GitHub/本机ID。SDK明确`--user uid:gid`，仅需要Docker socket的playable添加该组；HOME指向本job私有缓存，避免沿用/root。
2. 新缓存卷由本轮受控短命root初始化容器仅设置卷根和home的属主；不挂原仓库/工作副本，不递归修改共享目录，不启用privileged。初始化容器按随机ID归属检查并回收，失败不能留下未确认的缓存/初始化进程。
3. 运行时实际核对UID/GID并记录，推送证据检查与宿主执行身份一致。新增参数、身份记录、缓存初始化及权限回归；不将Windows源码检查或root Linux父进程当成普通Linux runner验证。
4. 本地构造真实Unix权限、普通UID1001父进程与root旧SDK的源job，先复现同类EACCES，再用同样普通父进程执行修复后的真实source/playable。父测试目录/卷均本轮新建，使用相同绝对host路径使子容器绑定语义与CI一致；只处理本轮标签资源，不动共享daemon、原仓库属主或开发卷。
5. 修复后在最终commit继续完整preflight并生成新绑定记录；8ffe6aa的旧通过不算新结果，不自动Push/重跑远端/PR。云端业务通过与最终job失败分开记录，不改写原失败。

## 验证
Windows与Linux运行器测试、实际运行UID/GID/文件属主/父进程删除，以及缓存归属和旧/新执行参数。普通Linux父进程先复现旧故障，再执行真实source/race和完整test:ci；最终本地commit的默认完整preflight与额外新DB浏览器全部通过、所有本轮资源为0、真实hook只接受新记录后才申请新的Push授权。若普通父进程或新SHA云端没跑，必须明确列为未覆盖。

### 实际本地结果
版本化`test-ci-owner.mjs --ref 8ffe6aa --jobs source --expect-eacces`在新Linux卷、UID1001/GID1001父进程上先完成旧源码/race，随后原样复现删除`.local/logs`的EACCES；输出expected-eacces-reproduced，不作为修复通过。该次父容器/卷均已回收。

修复候选tree718744b68d90b9435c1fd9abb62a9af93351a780以同样UID1001/GID1001父进程实际运行source与playable：源码/race、原真实协议、全部19浏览器（3.8m）、正常/故障清理均通过，SDK运行UID/GID实际核对为1001:1001。父测试卷retained=false，本轮父容器与卷独立查询为0，没有修改原仓库属主/开发卷。Windows运行器23通过、SDK专属Unix身份1项明确不运行；Linux正式SDK运行器23通过、Windows专属1项明确不运行，其中新文件属主测试实际执行。

以上是候选tree和普通父进程回归，不是假称最终commit或远端已通过；最终提交仍须默认完整preflight和额外浏览器生成新绑定记录，8ffe6aa云端失败保留。

## 代价、迁移与回退
增加一个极短的私有缓存初始化步骤及显式socket组；Docker socket仍仅用于可信本项目脚本，非多租户安全边界。无账号/游戏数据迁移，不改主机文件系统/daemon配置。UID/GID依本机实际值而变，数值相同不是跨机器证明，必须验证普通用户Unix语义。原有root写文件方案记录保留，不能回退到忽略清理错误或对未知目录sudo rm；确需回退时暂停推送并保留完整诊断。

## 重评条件
若rootless Docker/userns或非默认本机socket带来新限制，按实际身份/ACL验证，不盲目加入root组或提升整个运行器；另立迁移方案。若后续需要完全同宿主内核/文件系统的VM验收，再比较成本；本轮不伪称容器消除了宿主差异。
