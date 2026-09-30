# Nerve Wiki

一个自托管的团队在线笔记系统：工作区 → 笔记本 → 层级页面。正文就是原样的 Markdown，导出成文件夹不丢信息；同一个账户既可以由人在网页上使用，也可以由 Agent 通过接口与内置的 MCP 使用。笔记本可以采用 LLM Wiki 模式，由 Agent 按确定的契约维护知识库。

- 后端：Go（单个可执行文件，内嵌前端）+ PostgreSQL
- 前端：React + TypeScript
- 协议：[AGPL-3.0](LICENSE)

项目处于 v0.1 开发阶段，从 [docs/](docs/README.md) 开始阅读。

## 开发环境

需要安装：

- Docker（含 Compose v2）
- Go 1.26 或更高。第一次在 `server/` 下执行 Go 命令时，会自动下载 `server/go.mod` 指定的 Go 1.27.1。前提是 `GOTOOLCHAIN=auto`，这是 Go 官方安装包的默认值；部分 Linux 发行版自带的 Go 默认是 `local`，需要先执行 `go env -w GOTOOLCHAIN=auto`
- Node.js 24，并执行一次 `corepack enable`（pnpm 的版本由 `package.json` 锁定）

第一次启动：

```bash
pnpm install  # 安装 Node 依赖（检查工具要用）
make dev-db   # 启动本地 PostgreSQL 18（端口 55433，可用 NWIKI_DEV_DB_PORT 修改）
make check    # 持续集成的全部门禁：静态检查、未使用代码检查、测试
make          # 查看所有命令
```

- 命令按工具链分区：`*-go` 只需要 Go，`*-web` 需要 Node。
  - `make lint` 依次执行 `make lint-go` 和 `make lint-web`。前者校验 golangci-lint 的配置并运行它（含格式检查），并检查 `go.mod` 是否整洁；后者做 Markdown 样例集自检、`tools/` 下脚本的 oxlint（零警告）和格式检查。
  - `make knip` 检查未使用的文件、导出与依赖；配置里过时的条目也算失败。
  - `make test` 运行 Go 测试，开启竞态检测（需要 cgo：macOS 装有 Xcode 命令行工具即可）。
- 格式有问题时执行 `make fmt`，它修正 Go 与其余文件的格式。`docs/` 不参与格式化。
- 开发数据库以 builtin provider 的 `C.UTF-8` 初始化（`LC_CTYPE` 同为 `C.UTF-8`），与生产环境的要求相同，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。

## Markdown 样例集

`tools/md-fixtures/` 定义了 Markdown 的提取与改写规则，是服务端实现的验收标准。样例的输入逐字节有意义（CRLF、BOM、行尾空白），`.gitattributes` 与 `.editorconfig` 已经禁止工具改动它们。修改规则前先读它的 [README](tools/md-fixtures/README.md)。
