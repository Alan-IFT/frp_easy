// Package assets 在二进制里嵌入前端构建产物，并对外暴露 http.Handler。
//
// 第 1 轮（dev-backend round 1）：占位实现。前端工程 web/ 尚未启动 ——
// 暂时返回一段中文 HTML 提示，告知用户运行构建脚本。
//
// TODO(round-2)：在 dev-frontend 完成 + 产物落入 internal/assets/dist/ 后，
// 由 dev-backend 第 2 轮替换为 embed.FS（含 SPA fallback：未匹配文件 →
// 返回 index.html，让 Vue Router history 模式接管前端路由）。设计契约见
// docs/features/web-ui-mvp/02_SOLUTION_DESIGN.md §3.10 / §5.3。
package assets

import (
	"net/http"
)

const placeholderHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>frp_easy UI（占位页）</title>
<style>body{font-family:system-ui,sans-serif;max-width:680px;margin:48px auto;padding:0 20px;color:#222;line-height:1.6}code{background:#f3f3f3;padding:2px 6px;border-radius:3px}</style>
</head>
<body>
<h1>frp_easy UI</h1>
<p>前端尚未构建。请运行 <code>scripts\build.ps1</code>（Windows）或 <code>npm --prefix web run build</code>（Linux / macOS）后重启 frp_easy。</p>
<p>API 路径仍可访问：<a href="/api/v1/system/ready">/api/v1/system/ready</a></p>
</body>
</html>
`

// Handler 返回静态资源 handler。
// 第 1 轮：对任意 GET 请求都回上面的占位 HTML（200 + Content-Type: text/html）。
// 第 2 轮：替换为 http.FileServer(http.FS(distFS)) + SPA fallback。
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(placeholderHTML))
	})
}
