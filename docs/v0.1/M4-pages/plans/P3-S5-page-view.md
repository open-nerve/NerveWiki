# M4/P3/S5 页面的阅读视图：实施计划

上级：[P3 文档](../03-P3-markdown.md) 3.9、3.11。

## 任务

1. 契约：`api/modules/page.yaml` 加 `getPageView`（`GET /api/v0/pages/{page_id}/view`，`page.not_found`，Bearer）与 `PageView`（`html`、`revision`，必填，封闭）；`api/openapi.yaml` 登记路径；`make gen`。
2. 仓储：`queries/contents.sql` 的 `PageContent :one`；`store.go` 的 `PageContent`（没有行答 `app.ErrNotFound`）。
3. `app`：`ports.go` 的 `Markdown`、`Parsed`、`PageRef`、`Nodes.PageContent`；`get_page_view.go` 的 `GetPageView` 与 `ReadingView`。
4. `adapter/markdown`：`New(*markdown.Markdown)`，实现 `app.Markdown`；收到别处的 `Parsed` 报错。
5. HTTP：`handler.go` 的 `GetPageView`；`UseCases` 加一项。
6. 模块根：`Deps.Markdown`；`New` 接上用例。
7. 组合根：`newApp` 里 `markdown.New(markdownExtensions())` 一次，交给 `pageDeps`；`registrants.go` 的 `markdownExtensions()` 返回空（注释：M5 任务项的字节位置、M6 方言、M7 附件嵌入）。
8. 架构：规则"goldmark、`x/net/html`、`go.yaml.in/yaml` 只许 `internal/platform/markdown/...` 导入"与它的反例；`markdowntest` 登记三处；组合根的到达列表加 `markdownExtensions`。

## 测试

- 用例：调用次序（节点、工作区、判定、正文、解析、渲染）、不开事务；不存在、已删、不是页面、看不到笔记本、正文已删都答 `page.not_found`；渲染收到这一页的笔记本与页面；渲染的错误原样返回。
- 适配器：解析再渲染得到 HTML；别处的 `Parsed` 报错。
- 仓储：`PageContent` 读到正文与版本；删除的节点（正文随之进回收站）答 `ErrNotFound`。
- HTTP：200 的转换；`page.not_found`（每个声明的码在本包答出一次）。
- 模块根：`markdown.New` 带一个测试替身扩展，经 `page.New` 到达阅读视图，它按这一页取的数据出现在 HTML 里；正文用 SQL 写入（P4 之前没有写正文的接口）。
- 整个程序：矩阵一行（`notebookColumns()`、三种角色可读）；`page_visibility_test.go` 里阅读视图与逐项读取一致；`markdown_app_test.go`：`markdown.New(markdownExtensions())` 过样例集与生成的输入，`CheckHTML` 通过。
- 反向对照：架构规则的反例（`page/app` 导入 goldmark、别的平台包导入 `x/net/html`）；阅读视图不判定（矩阵失败）；`PageContent` 不看 `deleted_at`。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿；镜像冒烟。
