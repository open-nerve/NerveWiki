# M0/P5/S2 webui 与组合根的挂载：实施计划

上级：[P5 文档](../05-P5-web-shell.md) 3.6、3.8。

## 任务

1. `server/internal/platform/webui`：从 Nerve 拷贝 `embed.go`、`handler.go` 与测试，改名；`csp.go` 改为固定的策略（本项目的页面没有内联脚本），测试随之改写。`dist/.gitkeep` 提交，其余忽略。
2. 组合根：`newApp` 接收前端的文件系统，`webui.Handler(files)` 挂在 `/`；命令行传入 `webui.FS()`。
3. Makefile：`build` = `build-web` → 把 `web/apps/web/dist` 复制到 `webui/dist`（先清空，保留 `.gitkeep`）→ `go build -o bin/nervewiki ./cmd/nervewiki`。
4. README："运行后端"一节加 `make build`。

## 测试

- `webui` 单元测试：文件与缓存头、SPA 兜底、`assets/` 下的 404、隐藏文件、方法、未构建时的提示、CSP 只加在 HTML 上。
- 整个程序：挂上 `webui` 后，`/` 与任意前端路径答 `index.html`，带 CSP 与安全头；`/api/` 下的未知路径仍是 404 problem+json；`/healthz` 不受影响。
- 反向对照：`webui` 挂在遮住 `/api/` 兜底的模式上 → 整个程序的测试失败；CSP 去掉 `script-src 'self'` → 失败。

## 完成检查

`make check` 为绿；`make build` 后 `bin/nervewiki serve`，`curl /` 返回带 CSP 的 HTML，`/assets/*` 带长缓存。
