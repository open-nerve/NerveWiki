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
make check    # 静态检查、未使用代码检查、测试、前端构建；与提交之后的 make gen-check 合起来是持续集成 server、web 任务的门禁
make          # 查看所有命令
```

- 命令按工具链分区：`*-go` 只需要 Go，`*-web` 需要 Node。
  - `make lint` 依次执行 `make lint-go` 和 `make lint-web`。前者校验 golangci-lint 的配置并运行它（含格式检查），并检查 `server/` 与 `server/tools/` 的 `go.mod` 是否整洁；后者做 Markdown 样例集自检、`tools/`、`web/` 与 `e2e/` 的 oxlint（零警告）、格式检查和各 Node 包的类型检查。
  - `make knip` 检查未使用的文件、导出与依赖；配置里过时的条目也算失败。
  - `make test` 依次执行 `make test-go` 和 `make test-web`。前者运行 Go 测试，开启竞态检测（需要 cgo：macOS 装有 Xcode 命令行工具即可）。集成测试用 testcontainers 启动与开发库相同的 PostgreSQL 镜像，需要 Docker；只跑单元测试用 `cd server && go test -short ./...`。后者运行前端各包的 vitest。
  - 端到端测试另跑 `make e2e`，见下文"端到端测试"。
- 格式有问题时执行 `make fmt`，它修正 Go 与其余文件的格式。`docs/` 不参与格式化。
- 开发数据库以 builtin provider 的 `C.UTF-8` 初始化（`LC_CTYPE` 同为 `C.UTF-8`），与生产环境的要求相同，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。

## 运行后端

```bash
make dev-db   # 先启动开发数据库
make run      # 以 dev 配置启动 nervewiki serve，监听 127.0.0.1:8080；Ctrl-C 优雅停止
```

`make build` 构建前端并把它内嵌进 `bin/nervewiki`（版本号取 `VERSION`，默认 `0.1.0-dev`）。`make run` 启动的服务提供上一次 `make build` 复制进去的前端，从未构建时页面路径答 404 并提示；开发前端用 `make web-dev` 或 `make dev`，见下文"前端"。

`serve` 启动时要连上数据库（最多等 10 秒，连不上就退出），按配置执行迁移（dev、test 默认执行，prod 默认不执行），然后自检数据库的编码与 locale，不满足就拒绝启动并给出建库命令。`GET /healthz` 表示进程存活；`GET /readyz` 在数据库可用、迁移已是最新时返回 200，否则 503。`GET /api/v0/instance` 返回产品名、版本与接口版本；`/api/` 下没有的路径返回 404 problem+json。

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

## 接口与代码生成

接口用 OpenAPI 3.1 描述，服务端与前端都以它为准：

- `api/openapi.yaml` 是入口，列出全部路径；`api/common.yaml` 是公共组件（`Problem`、`FieldError`）；`api/modules/<模块>.yaml` 每个模块一个文件。
- `api/dist/openapi.yaml` 是 Redocly 打包的结果，契约测试和 TS 类型都读它。

改了接口描述后执行 `make gen`，并把生成物一起提交：

| 命令           | 生成                                                                                                                                      | 需要 |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ---- |
| `make gen-go`  | `server/internal/platform/httpserver/apigen`，每个模块的 `adapter/http/gen`（oapi-codegen 生成的接口层、bodyshapegen 生成的请求体结构表） | Go   |
| `make gen-web` | `api/dist/openapi.yaml`，`web/packages/api-client/src/schema.gen.ts`                                                                      | Node |

`make gen-check`（或分开的 `gen-check-go`、`gen-check-web`）重新生成后检查生成物已提交且没有差异，持续集成也执行它。oapi-codegen 锁定在独立的 Go 模块 `server/tools` 中，不进入 `nervewiki` 的依赖。

新增一个模块的接口：

1. 写 `api/modules/<模块>.yaml`，在 `api/openapi.yaml` 的 `tags` 与 `paths` 中列出；写法约定由 `apitest` 的规则测试（`server/internal/platform/httpserver/apitest/rules_test.go`）检查，要点见[总体设计](docs/v0.1/v0.1-design.md) 13.1。
2. 照抄 `instance` 的 `adapter/http/gen/oapi-codegen.yaml`，改掉输出路径，执行 `make gen`。
3. 在 `adapter/http` 中实现生成的 `StrictServerInterface`，由模块的 `Register` 挂到路由器上，在 `bootstrap` 中调用。
4. `adapter/http` 的测试以 `apitest.Main(m)` 为 `TestMain`（模块名取自测试所在的路径）：操作声明的每个错误码都要有测试经 `apitest.CheckResponse` 答过。

## 前端

`web/apps/web` 是 React 应用（Vite、React Router 的数据路由、TypeScript、Tailwind CSS、shadcn/ui 的组件写法、MobX、SWR），构建后内嵌进 `nervewiki`，与接口同源。

```bash
make dev       # 开发数据库 + 后端 + 前端热更新：打开 http://127.0.0.1:5173
make web-dev   # 只起前端开发服务器；/api、/healthz、/readyz 代理到 127.0.0.1:8080 上 make run 起的后端
make build     # 构建前端并内嵌进 bin/nervewiki
```

写法：

- 组件经由 store 取数据，store 经由 service 调接口，service 从构造函数拿 API 客户端。`RootStore`（`src/stores/root.store.ts`）是唯一装配它们的地方。组件不导入 `@nervewiki/api-client`，oxlint 检查这一点，接口类型从 service 导出。
- 加载由 SWR 驱动：页面 `useSWR(key, () => store.x.fetch())`，store 保存结果。示例见 `src/pages/home.tsx`。
- 文案在 `src/i18n/messages/`：`en.ts` 是源头，`zh-CN.ts` 缺键、多键时类型检查失败，占位符不一致时 vitest 失败。组件用 `useT()`。
- 页面在 `src/app/routes.tsx` 中按需加载，写成 `const { Page } = await import(…)`，knip 才看得出用到了哪些导出。

## 端到端测试

`e2e/` 用 Playwright（Chromium）驱动 `make build` 构建的 `bin/nervewiki`，数据库由 testcontainers 启动（需要 Docker）。一次运行启动一个 PostgreSQL、迁移一个模板库；每个 worker 复制出自己的库，运行自己的 `nervewiki serve`。

```bash
pnpm --filter @nervewiki/e2e exec playwright install chromium   # 第一次运行前安装浏览器
make e2e                                                        # make build，然后运行 e2e/stories 下的全部故事
cd e2e && pnpm exec playwright show-report                      # 查看上一次运行的报告
```

- 故事在 `e2e/stories/<分组>/`，从 `e2e/fixtures/test.ts` 取 `test` 与 `expect`：`db`（本 worker 的库）、`nervewiki`（本 worker 的服务）、`api`（类型化的客户端）、`newDatabase` 与 `nervewikiWith`（另起一个库、一个服务）、`pageWatch`（页面发出的接口请求与失败）。
- 每个测试的 `page` 从第一次导航之前就被监视；测试通过时，fixture 还核对页面是安静的：没有未捕获的异常、CSP 违规，控制台没有错误与警告。故事不用自己调用。
- 失败的测试在 `e2e/playwright-report/` 中带着 trace 与截图；`e2e/test-results/` 中有每个 worker 的服务日志和失败时导出的数据库。持续集成在失败时把两者作为 artifact 上传。
- `make e2e` 把 `VERSION` 交给故事核对注入的版本号；持续集成用 `0.0.0-ci.<运行号>`，与默认值不同。

## 部署

镜像由 `deploy/Dockerfile` 构建：前端与服务端都在其中，运行时是 distroless 镜像，只有 `/nervewiki` 一个程序，以非 root 用户（uid 65532）运行，监听 8080。

```bash
make image VERSION=0.1.0         # 构建 nervewiki:0.1.0
make image-smoke VERSION=0.1.0   # 在镜像上跑 S1、S3：迁移、探针、前端、实例与提交信息、非 root、优雅停机（另需 curl、jq）
```

镜像的提交信息取自构建上下文中的 `.git`，所以要在普通的克隆中构建：`git worktree` 的 `.git` 是指向别处的文件，`make image` 会直接报错。`.dockerignore` 排除的正好是 `.gitignore` 忽略的，改一个时同步另一个：否则镜像里的二进制报告的 `modified` 与工作区不符，`make image-smoke` 失败。

- 镜像默认 `NWIKI_ENV=prod`。配置用环境变量提供（也可以挂载一个目录并设置 `NWIKI_CONFIG_DIR`），至少要有数据库地址 `NWIKI_DATABASE__URL`；其余配置项见 `server/configs/config.yaml`，合并规则见上文"配置"。
- prod 配置不自动迁移。每次升级先执行迁移，再启动服务：

  ```bash
  docker run --rm -e NWIKI_DATABASE__URL=… nervewiki:0.1.0 migrate up
  docker run -d -p 8080:8080 -e NWIKI_DATABASE__URL=… nervewiki:0.1.0
  ```

- 数据库必须以 builtin provider 的 `C.UTF-8` 初始化，否则服务拒绝启动，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。
- 探针：存活用 `GET /healthz`（不访问任何依赖），就绪用 `GET /readyz`（数据库可用、迁移已执行完）。镜像里没有 shell 与 curl，所以没有写 `HEALTHCHECK`，由编排系统探测。
- 停止时发 SIGTERM：服务停止接收新连接，等正在处理的请求结束（最多 `server.shutdown_timeout`，默认 20 秒）后退出。停机的宽限期要比它长：`docker stop` 默认只等 10 秒，用 `docker stop -t 30`。

## Markdown 样例集

`tools/md-fixtures/` 定义了 Markdown 的提取与改写规则，是服务端实现的验收标准。样例的输入逐字节有意义（CRLF、BOM、行尾空白），`.gitattributes` 与 `.editorconfig` 已经禁止工具改动它们。修改规则前先读它的 [README](tools/md-fixtures/README.md)。
