// Package assets 在二进制里嵌入前端构建产物（Vue 3 SPA），并对外暴露 http.Handler。
//
// 构建产物路径：web/ → (npm run build) → internal/assets/dist/
// 嵌入路径：该包的 //go:embed 指令从本包目录拾取 dist/。
//
// SPA fallback（§5.3）：路径未找到时一律返回 dist/index.html，让 Vue Router
// history 模式在浏览器侧接管 /setup / /login / /dashboard 等路由。
package assets

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler 返回嵌入前端资源的 http.Handler，含 SPA fallback。
//
// 请求处理优先级：
//  1. 精确匹配 dist/<path> → 直接返回文件（JS/CSS/favicon 等）。
//  2. 未匹配 → 返回 dist/index.html（Vue Router history mode fallback）。
func Handler() http.Handler {
	// 剥去 "dist" 前缀，让外层 "/" 映射到 dist/index.html。
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// embed 目录不存在时的安全降级：返回 503 而不是 panic。
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "前端资源未嵌入，请先运行 scripts/build", http.StatusServiceUnavailable)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 尝试在 embed.FS 里找到该文件。
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, ferr := fs.Stat(sub, path); ferr == nil {
			// 文件存在：交给 fileServer。
			fileServer.ServeHTTP(w, r)
			return
		}
		// 文件不存在：SPA fallback → index.html。
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
