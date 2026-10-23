---
name: prepare-commit
description: 准备或创建本地阶段提交，以及交付 master 前使用。
---
# 阶段提交

1. 核实当前仓库、功能分支与已有改动，不在 main 或 master 直接开发。
2. 本次切片实现与相关回归完整通过后，立即更新成长记录并 commit，不攒到最终版本再拆。
3. 只暂存本阶段的完整源代码、测试、决策与必要说明；不提交凭据、旧版备份、构建缓存或测试账号现场。
4. 提交格式：`<type>(<scope>): 中文具体描述`；常用 chore/docs/build/feat/fix/test/refactor/ci。
5. 正文说明当前变更、原因、实际验证与边界，不写未来能力已经可用；时间使用真实当前时间。
6. commit 后核对结果与剩余工作区。master 仅在完整回归通过后快进。
7. 提交后在干净最终HEAD执行 `npm run verify:ci`；只有默认完整preflight通过才自动生成24小时有效的提交/tree/报告绑定记录。确认本仓库hook已安装（`node scripts/install-hooks.mjs`，不覆盖其他hook）且 `node scripts/prepush-guard.mjs --check` 有效；新提交/源码变化/失败尝试使旧证据不能放行，不手写报告或 `--no-verify` 绕过。
8. Push 前另展示最终完整提交列表、仓库、目标 origin/master 与本次验证，取得明确授权；门禁不代替授权或同SHA远端CI复验，不自动创建 PR。
