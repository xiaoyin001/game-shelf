# ADR-0020：升级 protobuf 补丁以修正手写 proto3 标量 presence

状态：采用（2026-10-22，presence/零值编码、全量 Go race、真实四服务与17/17浏览器重跑通过）。替换 [ADR-0002](0002-go-transport.md) 所用 protobuf 实现的补丁版本，不更换帧、协议号、字段编号或 legacy API；配套 [ADR-0019](0019-npc-interactions.md)。

## 背景与证据
NPC 功能的真实四服务、独立协议与17/17浏览器已通过，但 Linux Go1.25.0 的全量 race 在 WorldResp 往返测试中触发 checkptr misaligned pointer conversion，不能视为交付通过。

最小独立诊断确认：google.golang.org/protobuf v1.36.5 对带 proto3 标签的 WorldQuest 描述虽然显示 proto3，所有标量 HasPresence 却为 true。该版本 internal/encoding/tag/tag.go 在读标签前从 SurrogateProto2 设置 EditionFeatures，读到 proto3 后只改变 ParentFile，没有更新这些特性；legacy coder 因而把连续 int32 标量当 presence 指针检查，offset20 非8字节对齐，race/checkptr 报错。此前部分手写消息布局碰巧对齐，不代表语义正确。

本机缓存的官方 v1.36.11 源码在标签解析后才根据最终 ParentFile 设置 EditionFeatures，保留 packed 标记，直接修正同一错误。以上是实际源码/描述与最小复现证据，不把编译成功或旧版 race 通过替代本轮结果。

## 比较
- 保持 v1.36.5、关闭 checkptr/race、给结构补 padding 或把所有计数扩大为 int64：不采用，掩盖错误的 presence 语义，布局改变仍可能再次触发。
- 现在把全部契约迁移为 protoc 生成的 v2 类型/描述符：正规长远方向，但需迁移既有手写类型、生成工具与构建规则；当前补丁能直接修正已定位问题，不扩大本轮范围。
- 升级官方 google.golang.org/protobuf 至精确 v1.36.11：采用。保持 github.com/golang/protobuf v1.5.4 的兼容桥与所有业务契约，用 go.sum 校验官方模块，不修改依赖源码。新增描述符 presence 语义测试，保留原往返与完整回归。

## 决定
固定 v1.36.11 并增加语义回归，不调整字段对齐、不关闭 race/checkptr；完成下述全部验证后再交付。

## 代价、迁移与回退
补丁影响所有使用兼容桥的消息，必须全量 Go/race、真实账号/原TCP/世界协议与浏览器重跑；性能/内存差异未实测。正确的 proto3 默认零值可以被省略，旧版显式零值也可正常解码，字段与 JSON 形状不变，无账号/数据迁移。四服务仍成套构建，不混用本轮二进制。

回退本补丁必须同时撤回依赖它的新字段或改成完整正确的生成描述符方案；不允许靠关闭检查、删除测试或加无意义对齐字段宣称回退可用。保留失败日志作为重评依据，不清任何开发数据。

## 验证计划
先复跑最小 presence 断言与 WorldResp 往返的 race，再执行全量源码/产物检查、Go race、真实三依赖/四服务、test:all 的全部17项与独立 test:services。所有通过后才更新阶段完成记录并提交。

## 实际验证
v1.36.11 的所有新增/世界标量 HasPresence=false，空消息编码为空；保留完整 WorldResp/WorldReq 往返测试。最小 race、WSL 实际源码全量 race、最终独立 Ubuntu22.04 工具容器全量 race 均通过，没有禁用 checkptr 或调整字段对齐。go mod verify 通过，go.mod 仅此一个补丁版本变化。

升级后的 Windows check/check:client 与 Linux check/check:client/test:all（真实依赖、原TCP/账号/世界/恶意请求与17/17浏览器，2.2m）、独立 test:services 全部通过，最终容器回收为0。既有浏览器14项未修改或删减；本轮没有执行新版原生Windows全套联机、完整test:ci故障注入或云端复验。

## 重评条件
新增复杂契约、oneof、跨语言客户端或生成工具链需求时，另立 ADR 比较全部迁移 protoc/v2 与继续手写的代价；任何再次出现的描述符问题必须按实际 presence/wire 语义修正，不能依赖内存对齐碰巧通过。
