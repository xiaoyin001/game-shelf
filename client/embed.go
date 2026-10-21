// 文件说明：将无需构建的浏览器客户端嵌入 login 二进制。
package client

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var assets embed.FS

// Handler 提供仓库自带页面，固定资源目录不依赖运行工作目录。
func Handler() http.Handler {
	root, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(root))
}
