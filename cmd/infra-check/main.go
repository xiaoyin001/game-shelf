// 文件说明：分发一次性依赖就绪检查，失败返回非零状态而不启动业务。
package main

import (
	"log/slog"
	"os"
)

func main() {
	if err := run(); err != nil {
		slog.Error("依赖协议检查失败", "error", err)
		os.Exit(1)
	}
}
