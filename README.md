# game-shelf

以可验收的浏览器联机沙盒为目标，从空仓库逐段建立客户端、服务器、AI 开发规范和自动回归。

## 当前能力

当前有 AI 规范、Go TCP/protobuf 基础、公共配置/日志/发现/数据库与会话封装；真实 etcd、Redis、MongoDB 烟测可运行。还没有业务服务器或游戏。后续每个真实实现阶段都会独立验证和提交；最新事实见 [成长记录](docs/progress.md)。

## 本阶段验证

需要 Node.js 22+ 和 Go 1.25+。Node 检查无第三方依赖，Go 依赖通过 go.mod 固定：

```text
npm run check
```

真实基础设施烟测：`npm run test:infra`。需要 Docker Compose；Windows 可以使用 WSL 内 Docker，运行器只回收自己的保活管道，不清数据库或卷。

## 开发入口

- [产品目标与边界](docs/playable-mvp.md)
- [技术决策记录](docs/decisions/README.md)
- [AI 开发流程](docs/ai-development.md)
- [成长与阶段验证](docs/progress.md)

开发使用独立功能分支，完整通过后交付到 master，不使用 main。旧实现只是验收对照，不进入新历史；不倒填时间、不用拆分旧大提交冒充逐步开发。远端推送仍须单独确认。
