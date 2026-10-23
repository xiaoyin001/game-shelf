# 技术决策索引

每次技术选择先记录，再实现。小决策可写在同一记录中；替换必须另建记录，保留旧方案和原因。使用 [模板](template.md)。

- [ADR-0001：从空仓库逐段形成可复跑成长历史](0001-incremental-rebuild.md)

- [ADR-0002：Go 与 TCP 通信基础](0002-go-transport.md)

- [ADR-0003：真实本地依赖与存储边界](0003-local-infrastructure.md)

- [ADR-0004：HTTP 登录与 TCP 大厅链路](0004-service-login.md)

- [ADR-0005：浏览器与共享世界在线切片](0005-browser-presence.md)

- [ADR-0006：权威移动输入](0006-authoritative-movement.md)

- [ADR-0007：世界文字聊天](0007-world-chat.md)

- [ADR-0008：共享资源采集](0008-shared-gathering.md)

- [ADR-0009：客户端呈现与可访问界面](0009-client-presentation.md)

- [ADR-0010：成长历史核对与交付](0010-delivery-verification.md)

- [ADR-0011：真实协议就绪与失败现场](0011-semantic-dependency-readiness.md)

- [ADR-0012：可复现的 Linux 冷启动 CI](0012-reproducible-ci.md)

- [ADR-0013：MongoDB 与联机 CI 的内核兼容](0013-mongodb-kernel-compatibility.md)

- [ADR-0014：账号密码验证与试玩身份隔离](0014-password-authentication.md)

- [ADR-0015：本地 Three.js 与真正 3D 场景](0015-browser-3d.md)

- [ADR-0016：6–32 位 ASCII 密码与一致输入限制](0016-password-input-policy.md)

- [ADR-0017：服务端跳跃与实体碰撞](0017-authoritative-jump-collision.md)

- [ADR-0018：采集原页重进与 CI 时限](0018-browser-gather-ci.md)

- [ADR-0019：近距离 NPC 对话与本轮采集委托](0019-npc-interactions.md)

- [ADR-0020：protobuf 补丁与 proto3 标量 presence](0020-protobuf-scalar-presence.md)

- [ADR-0021：服务端低台阶自动迈步](0021-authoritative-step-up.md)

- [ADR-0022：空闲渲染预算与 CI 分层时限](0022-ci-render-budget.md)

- [ADR-0023：共用有界 CI 环境与提交绑定预验收](0023-shared-ci-preflight.md)

- [ADR-0024：冻结 Ubuntu 依赖与唯一 headless shell](0024-headless-ci-image.md)

状态“采用”不代表所有产品目标完成。实际结果见 [成长记录](../progress.md)。
