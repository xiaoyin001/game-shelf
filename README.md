# game-shelf

以可验收的浏览器联机沙盒为目标，从空仓库逐段建立客户端、服务器、AI 开发规范和自动回归。

## 当前能力

当前浏览器可以进入同一个世界、查看彼此在线、离开重进和同名顶替；四个 Go 服务与账号保存链路继续可用。支持 WASD / 方向键权威移动，其他浏览器看到同步坐标；还没有聊天和采集。后续每个真实实现阶段都会独立验证和提交；最新事实见 [成长记录](docs/progress.md)。

## 本阶段验证

需要 Node.js 22+ 和 Go 1.25+。Node 检查无第三方依赖，Go 依赖通过 go.mod 固定：

```text
npm run check
```

真实基础设施烟测：`npm run test:infra`。需要 Docker Compose；Windows 可以使用 WSL 内 Docker，运行器只回收自己的保活管道，不清数据库或卷。

启动当前服务：`npm run dev`；登录接口为 `POST http://127.0.0.1:11000/login`。浏览器打开 http://127.0.0.1:11000，以两个不同名字验证共享在线。

组合验证：`npm run test:services`；运行器安全验证：`npm run test:runner`。停止时只回收自建服务，日志在 `.local/logs/`。

首次浏览器测试需 `npm ci` 与 `npx playwright install chromium`。执行 `npm run test:all` 可验证当前真实链路与双浏览器在线；`npm run check:client` 检查客户端与脚本格式。

## 开发入口

- [产品目标与边界](docs/playable-mvp.md)
- [技术决策记录](docs/decisions/README.md)
- [AI 开发流程](docs/ai-development.md)
- [成长与阶段验证](docs/progress.md)

开发使用独立功能分支，完整通过后交付到 master，不使用 main。旧实现只是验收对照，不进入新历史；不倒填时间、不用拆分旧大提交冒充逐步开发。远端推送仍须单独确认。
