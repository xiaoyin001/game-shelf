// 文件说明：将无需构建的浏览器客户端嵌入 login 二进制。
package client

import (
	"embed"
	"game-shelf/pkg/scene"
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
	mux := http.NewServeMux()
	mux.HandleFunc("/scene.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		data, _ := scene.Files.ReadFile("layout.json")
		_, _ = w.Write(data)
	})
	mux.Handle("/", http.FileServer(http.FS(root)))
	return mux
}
