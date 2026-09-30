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
make dev-db   # 启动本地 PostgreSQL 18（端口 55433；用 NWIKI_DEV_DB_PORT 修改时，同时覆盖 database.url，见下文"配置"）
make check    # 持续集成的全部门禁：静态检查、未使用代码检查、测试
make          # 查看所有命令
```

- 命令按工具链分区：`*-go` 只需要 Go，`*-web` 需要 Node。
  - `make lint` 依次执行 `make lint-go` 和 `make lint-web`。前者校验 golangci-lint 的配置并运行它（含格式检查），并检查 `go.mod` 是否整洁；后者做 Markdown 样例集自检、`tools/` 下脚本的 oxlint（零警告）和格式检查。
  - `make knip` 检查未使用的文件、导出与依赖；配置里过时的条目也算失败。
  - `make test` 运行 Go 测试，开启竞态检测（需要 cgo：macOS 装有 Xcode 命令行工具即可）。集成测试用 testcontainers 启动与开发库相同的 PostgreSQL 镜像，需要 Docker；只跑单元测试用 `cd server && go test -short ./...`。
- 格式有问题时执行 `make fmt`，它修正 Go 与其余文件的格式。`docs/` 不参与格式化。
- 开发数据库以 builtin provider 的 `C.UTF-8` 初始化（`LC_CTYPE` 同为 `C.UTF-8`），与生产环境的要求相同，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。

## 运行后端

```bash
make dev-db   # 先启动开发数据库
make run      # 以 dev 配置启动 nervewiki serve，监听 127.0.0.1:8080；Ctrl-C 优雅停止
```

`serve` 启动时要连上数据库（最多等 10 秒，连不上就退出），按配置执行迁移（dev、test 默认执行，prod 默认不执行），然后自检数据库的编码与 locale，不满足就拒绝启动并给出建库命令。`GET /healthz` 表示进程存活；`GET /readyz` 在数据库可用、迁移已是最新时返回 200，否则 503。

其他命令在 `server/` 下用 `go run ./cmd/nervewiki <命令>` 执行：

| 命令             | 作用                                                           |
| ---------------- | -------------------------------------------------------------- |
| `serve`          | 运行 HTTP 服务，直到收到 SIGINT 或 SIGTERM；第二次信号立即退出 |
| `migrate up`     | 执行全部待执行的迁移，然后自检数据库                           |
| `migrate down`   | 回滚最近一条迁移                                               |
| `migrate status` | 列出迁移及其状态                                               |
| `version`        | 打印版本号与构建信息                                           |

### 配置

配置按层合并，后面的覆盖前面的：

1. 内置的 `server/configs/config.yaml`（列出全部配置项及默认值）；
2. 内置的 `server/configs/config.<env>.yaml`；
3. `$NWIKI_CONFIG_DIR` 下的 `config.yaml`、`config.<env>.yaml`（设置了且文件存在时）；
4. 个人覆盖文件 `configs/config.local.yaml`，相对于工作目录，也就是 `server/` 下启动时（`make run` 就是）的 `server/configs/config.local.yaml`；只在 dev 生效，不进仓库；
5. 环境变量 `NWIKI_<节>__<键>`，例如 `database.url` 对应 `NWIKI_DATABASE__URL`。

`NWIKI_ENV` 选择环境（`dev`、`test`、`prod`，默认 `dev`）。未知的键、空值、越界的数字、不带单位的时长都会报错，所有无效的键一次列出。日志里的数据库地址整体脱敏。

## Markdown 样例集

`tools/md-fixtures/` 定义了 Markdown 的提取与改写规则，是服务端实现的验收标准。样例的输入逐字节有意义（CRLF、BOM、行尾空白），`.gitattributes` 与 `.editorconfig` 已经禁止工具改动它们。修改规则前先读它的 [README](tools/md-fixtures/README.md)。
