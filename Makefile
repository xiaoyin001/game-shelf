# 文件说明：提供当前阶段可复跑的检查、格式化、测试和构建入口。
.DEFAULT_GOAL := help
.PHONY: help check fmt test build
# 显示当前可用命令，不描述未实现能力。
help:
	@echo "make check - 规则、格式、Go 测试、vet 与构建"
	@echo "make fmt   - 格式化项目 Go 源码"
	@echo "make test  - 执行 Go 包测试"
	@echo "make build - 构建当前所有 Go 包"
# 检查当前代码，不自动格式化或写工作区。
check:
	node scripts/check.mjs
# 只处理项目内实际存在的 Go 目录。
fmt:
	node scripts/check.mjs --format
# 执行当前包测试，不冒充后续真实联机验收。
test:
	go test ./...
# 编译当前所有 Go 包，后续有服务入口时一起编译。
build:
	go build ./...
