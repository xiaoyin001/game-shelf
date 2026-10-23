---
name: validate-change
description: 每个实现阶段完成与提交前，选择并执行当前阶段回归时使用。
---
# 阶段验证

- 所有阶段执行 npm run check:workflow，检查规则、ADR、文档入口与 Git 忽略边界。
- 当前 `npm run check` / `make check` 已包含规则、成长历史与 Go 格式检查、测试、vet 与构建；`make fmt` 负责格式化。
- 尚无业务服务或客户端时，不假装联机/浏览器运行通过；相关能力出现后补充对应入口。
- 服务链路改变执行 `npm run test:services`，依赖封装改变执行 `npm run test:infra`，运行器改变执行 `npm run test:runner`。
- 浏览器、WebSocket、会话或世界改变还要执行 `npm run check:client` 和 `npm run test:all`，首次 `npm ci`、`npx playwright install chromium`。浏览器回归随阶段扩充，当前包含在线、顶号、移动、聊天、采集和窄屏；不能用早期仅在线用例替代当前完整玩法回归。
- 新增行为同时增加实际回归。默认测试不能把外部依赖静默替换为假服务并宣称真实联机通过。
- 编译、单元测试、真实服务链路、真实浏览器与 CI 分开记录，不沿用旧版报告。
- 申请Push必须在最终提交的干净HEAD执行 `npm run verify:ci`（默认完整source+playable及额外新DB浏览器），读取本轮报告和 `node scripts/prepush-guard.mjs --check`，不能用candidate/test:all/单job替代。候选阶段可先暂存完整切片后使用 `--candidate`，但不生成可推送记录。
- 运行器实际核对2CPU/4GiB/无swap、锁定SDK、宿主/SDK UID与GID、完整命令及零残留。Docker运行器或文件清理改变还要用 `node scripts/test-ci-owner.mjs --candidate --jobs source,playable` 在独立Linux卷的普通UID1001父进程执行真实job，不能用Windows/NTFS或root父进程替代Unix权限回归。本地额外浏览器轮/27s余量不等于云端额外测试；CI单job仍完整原回归一次、30s/零重试。记录准备/工具缓存是否冷启动，不拿旧私有prepared镜像当可重建证据。
- 本阶段相关测试全部通过才允许 commit；更新 docs/progress.md 记录能力、命令、结果和未覆盖项。
- 测试失败继续调查，不能删弱用例或事后拆分大提交来冒充阶段验证。
