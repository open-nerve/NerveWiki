# M0 基础骨架：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M0 基础骨架（`M0-foundation`） |
| 日期 | 2026-09-30 |
| 状态 | 进行中 |
| 依赖 | 无 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 12.2 节 |

---

## 1. 目标

M0 结束时，项目具备以下条件，此后每个 M 只需要在上面加业务：

1. **一条命令起开发环境**：本地 PostgreSQL、后端、前端热更新。
2. **一个可发布的产物**：`nervewiki` 单个可执行文件（内嵌前端），以及 Docker 镜像。
3. **后端骨架**：平台层与组合根就位；第一个模块 `instance` 走通"写接口描述 → 生成代码 → 实现 → 契约测试"的全链路。
4. **前端骨架**：路由、UI 基座、多语言、"service → store → 组件"的分层样板、全部门禁。
5. **测试骨架**：单元测试、集成测试（testcontainers + 模板库）、契约测试、架构测试、端到端测试（Playwright）都能运行，并进入持续集成。
6. **五项技术风险已验证**，结论写进文档，必要时修订总体设计。

## 2. 范围

**做**：

- 仓库、工具链、持续集成、开发用 compose。
- 服务端平台层：配置、日志、PostgreSQL 连接池与事务管理、迁移、数据库 locale 自检、HTTP 服务（中间件链、problem+json、请求体结构检查、长连接路由的豁免）、时钟、构建信息、前端内嵌；组合根；命令行（`serve`、`migrate`）；`/healthz`、`/readyz`；优雅停机；架构测试。
- 接口契约工具链与试点模块 `instance`（`GET /api/v0/instance`）。
- 前端外壳：应用骨架、路由、Tailwind + shadcn/ui、zh-CN 与 en、生成的 TS 客户端、错误边界与 404、明暗主题。
- 端到端测试骨架与冒烟故事；Dockerfile 与镜像构建。
- 技术验证（P1）。

**不做**（由第一个用到它的 M 接入，接入时有真实用例验证它）：

- 业务表、sqlc、River、限流、认证与令牌管理器、分页游标——随 M1 接入。
- 权限框架——随 M2 接入。

## 3. 完成标准

- 冒烟故事全部通过（本地 `make e2e` 与持续集成）：

  | 故事 | 内容 |
  |---|---|
  | S1 健康检查 | 迁移完成后 `/healthz`、`/readyz` 返回 200；数据库不可用或迁移未完成时 `/readyz` 返回 503 |
  | S2 首页 | 打开首页：页面由内嵌前端提供，带 CSP；只请求同源接口；没有失败的请求、未处理的异常、CSP 违规，控制台没有错误和警告；页面显示实例版本 |
  | S3 实例接口 | `GET /api/v0/instance` 返回版本等信息，响应符合接口描述 |
  | S4 路由兜底 | 任意前端路由直接打开或刷新都返回应用页面；未知的 `/api/` 路径返回 problem+json 404 |

- Docker 镜像能启动，并通过 S1、S3。
- 持续集成的全部任务为绿：Go 的 lint 与测试（含架构测试和集成测试）、生成物一致性、前端的类型检查 / oxlint / oxfmt / knip / vitest / 构建、端到端测试、镜像构建。
- P1 的五项技术验证都有结论，需要修订的总体设计已修订。
- 满足[文档约定](../../README.md)中"M 完成"的全部条件。

## 4. M0 结束时的仓库布局

```
nerve-wiki/
  .github/workflows/ci.yml
  api/
    openapi.yaml              入口：列出全部路径，声明平台错误码
    common.yaml               公共组件：Problem、字段错误等
    modules/instance.yaml
    dist/openapi.yaml         打包产物（生成，提交）
  server/
    cmd/nervewiki/            命令行入口
    configs/                  config.yaml、config.{dev,test,prod}.yaml（go:embed）
    migrations/               goose 迁移（M0 只有一条：建 pg_trgm 扩展）
    internal/
      bootstrap/              组合根
      platform/               buildinfo、clock、config、httpserver（含 apitest、bodyshape）、
                              logging、postgres（含 pgtest）、webui
      shared/                 共享内核（M0 只有错误模型与 TxManager 端口）
      modules/instance/       试点模块
      archtest/               架构测试
    tools/                    独立的 Go 模块：代码生成工具（bodyshapegen 等）
  web/
    apps/web/                 前端应用
    packages/api-client/      生成的 TS 客户端
    packages/i18n/            文案与一致性检查
    packages/ui/              shadcn/ui 组件与主题
    packages/tsconfig/        共享的 TypeScript 配置
  e2e/                        Playwright：fixtures、global-setup、stories/smoke
  deploy/
    compose.dev.yaml          本地 PostgreSQL 18
    Dockerfile
  tools/md-fixtures/          Markdown 样例集：规范、自检、与 Obsidian 核对的工具（P1 建立，M4、M6 使用）
  docs/
  Makefile                    所有命令的入口
  LICENSE                     AGPL-3.0
  README.md                   项目说明与开发环境
```

`web/packages/` 下的具体划分在 P5 定稿；只有被两个以上的使用方共用、或者有独立的检查规则时才拆成包，否则留在 `apps/web` 里。

## 5. 关键选型

| 方面 | 选型 |
|---|---|
| 后端 | Go 1.27；PostgreSQL 18；pgx v5；goose v3；koanf v2；cobra；oapi-codegen v2（strict server）；kin-openapi；testcontainers-go；golangci-lint（含 depguard） |
| 前端 | Node 24；pnpm + turbo；React 19；React Router 7（SPA 模式）；Vite；TypeScript；Tailwind CSS 4；shadcn/ui（Radix）；MobX；SWR；openapi-typescript + openapi-fetch；vitest；oxlint；oxfmt；knip |
| 端到端 | Playwright（Chromium）+ testcontainers |
| 交付 | 多阶段 Dockerfile；镜像以非 root 用户运行 |

具体版本在 P2 锁定：取当时的最新稳定版，写进 P2 文档；Nerve 在 2026-09 核实过的版本作为参考。

## 6. 从 Nerve 拷贝的范围

原则：

- **只拷贝本 M 用得到的**，其余随用到它的 M 一起拷贝（例如限流、River、认证接入点随 M1）。
- **拷贝即接管**：每个拷贝进来的文件都按本项目的代码逐个审查；改名（`nerve` → `nervewiki`、`NERVE_` → `NWIKI_`、模块路径）；删掉指向 Nerve 文档的注释和编号引用，必要的说明改写成自足的注释；删掉本项目用不到的分支和选项。
- 拷贝是一次性的，之后不与 Nerve 同步。

| Nerve 中的位置 | 处理 | 所在 Phase |
|---|---|---|
| `Makefile`、`.editorconfig`、`.gitignore`、`.node-version`、`.oxlintrc.json`、`.oxfmtrc.json`、`pnpm-workspace.yaml`、`knip.jsonc`、`.github/workflows/ci.yml`、`deploy/compose.dev.yaml` | 拷贝后按本项目裁剪：去掉 Plane 相关的目标、关键词守卫、oxlint 警告上限（本项目从零警告起步） | P2 |
| `server/internal/platform/buildinfo` | 拷贝、改名（门禁需要一个真实的 Go 包，从 P3 提前） | P2 |
| `turbo.json` | 随第一个工作区包引入，按需裁剪 | P5 |
| `server/internal/platform/{clock,config,logging,postgres}` | 拷贝、改名、裁剪 | P3 |
| `server/internal/platform/httpserver`（不含认证与限流的接入） | 拷贝、改名、裁剪；SSE 所需的调整按 P1 的结论处理。接口操作的逐路由中间件（请求期限、请求体上限）与 `APIErrors` 随 P4 | P3、P4 |
| `server/internal/platform/webui` | 拷贝、改名、裁剪 | P5 |
| `server/internal/archtest` | 拷贝；规则按本项目的模块清单调整；sqlc 相关规则随 M1 | P3 |
| `server/internal/bootstrap`（组合根骨架与命令） | 只拷贝骨架 | P3 |
| `server/internal/shared/{error,tx}.go` | 拷贝 | P3 |
| `server/cmd/nerve` → `server/cmd/nervewiki`（`serve`、`migrate`） | 拷贝、改名 | P3 |
| `server/configs` | 拷贝、改名、只保留 M0 的配置项 | P3 |
| `server/tools/bodyshapegen`、`platform/httpserver/{apitest,bodyshape,apigen}` | 拷贝 | P4 |
| `server/internal/modules/instance`（只取实例信息，不取时区列表） | 拷贝、裁剪 | P4 |
| `api/{openapi.yaml,common.yaml,redocly.yaml,modules/instance.yaml}` | 拷贝、裁剪 | P4 |
| `e2e/{playwright.config.ts,global-setup.ts,fixtures/{server,db,browser,api,test}.ts,stories/smoke}` | 拷贝、改写 | P6 |
| `web/` | **不拷贝**（Plane 系代码）；前端外壳全新编写 | P5 |

## 7. Phase 划分

每个 Phase 按[文档约定](../../README.md)执行：开工时写 Phase 文档 → 实现 → 测试 → 审查 → 修复 → 合并。

| P | 名称 | 目标 | 主要交付 | 验证 |
|---|---|---|---|---|
| P1 | 技术验证 | 在打地基之前验证五项风险 | 五份结论（写在 P1 文档的"结果"一节）；`tools/md-fixtures/`（与 Obsidian 核对）；必要时修订总体设计。实验代码是一次性的，不进入产品代码 | 每项有明确的"可行 / 不可行 / 替代方案"结论 |
| P2 | 仓库与工具链 | 空仓库能跑通全部门禁 | 仓库布局、LICENSE、README 开发环境一节；Go 模块 `server`（第一个包 `buildinfo`）、golangci-lint；pnpm 工作区、oxlint、oxfmt、knip；Makefile；开发用 compose（PostgreSQL 18，数据库 locale 按 P1 结论）；持续集成的 lint 任务（含样例集自检） | 本地与持续集成的门禁为绿 |
| P3 | 服务端平台层 | 一个能启动、能迁移、能优雅停机的 `nervewiki` | 平台层各包、组合根、`serve` 与 `migrate` 命令、`/healthz` 与 `/readyz`、数据库 locale 自检、长连接路由的豁免、集成测试工具（`pgtest`：模板库复制）、架构测试 | 单元、集成、架构测试为绿；二进制启动后健康检查可用 |
| P4 | 接口契约与代码生成 | 走通"描述 → 生成 → 实现 → 契约测试" | `server/tools` 模块、`api/` 结构、oapi-codegen 与 bodyshape 生成、接口操作的逐路由中间件与 `APIErrors`、`apitest`、`instance` 模块、TS 客户端生成、`make gen` 与 `make gen-check` | 生成物一致性检查为绿；`instance` 的 handler 测试与契约测试为绿 |
| P5 | 前端外壳与内嵌 | 前端能构建、内嵌进二进制、在浏览器里运行 | `webui`（内嵌前端、页面 CSP、前端路由兜底）、turbo、应用骨架、路由与兜底、UI 基座与主题、zh-CN 与 en、分层样板（instance 的 service / store / 组件）、错误边界与 404、页面 CSP、`make build` | 前端全部门禁为绿；二进制提供页面并显示实例版本 |
| P6 | 端到端测试与交付 | 冒烟故事在本地和持续集成里通过，产出镜像 | e2e 包（模板库、每个 worker 一个 `nervewiki`、页面与数据库 fixture、控制台与 CSP 监视）、S1–S4、持续集成的 e2e 任务与失败时的产物上传、Dockerfile、镜像构建 | `make e2e` 本地与持续集成为绿；镜像通过 S1、S3 |

P1 放在最前面：它的结论会影响 P2（数据库 locale）、P3（SSE 与 MCP 对 HTTP 中间件的要求，例如长连接不受请求期限限制），以及 M4 之后的多个 M，先验证可以避免返工。

### P1 的五项技术验证

| # | 验证什么 | 影响哪里 | 结论（详见 [P1 文档](01-P1-spikes.md) 第 7 节） |
|---|---|---|---|
| ① | PostgreSQL 18 在不同 locale / collation 下，pg_trgm 对中文的三元组切分；1、2、3 个字及中英混合的查询；是否用上 GIN 索引（`EXPLAIN`）。定出所有环境统一使用的数据库 locale | P2 的 compose、测试容器与部署文档；M8 搜索 | 可行：builtin `C.UTF-8`，`LC_CTYPE` 同为 `C.UTF-8`；启动时自检。1–2 个字的查询只能顺序扫描，M8 前用真实语料复核 |
| ② | goldmark 与 remark 对同一批 Markdown 的链接、嵌入、标签、frontmatter 提取是否一致（覆盖嵌套、转义、代码块中的链接、URL 中的 `#`、`[[a\|b]]`、`[[a#h]]` 等边界）；goldmark 能否给出精确的字节位置，供链接改写使用 | M6 的解析与改写；`tools/md-fixtures/` | 做不到对任意输入一致：改为服务端唯一解析（提取与渲染共用 `Parse`），前端不做语义解析；样例集与 Obsidian 核对 |
| ③ | 写入事务中的 `NOTIFY` 在提交后送达；pgx 的 `LISTEN` 连接管理与断线重连；浏览器用 `fetch` 流式读取带 Bearer 的 SSE；经过 Caddy 时的缓冲与超时；请求期限中间件对长连接的影响；`NOTIFY` 负载上限 | P3 的 HTTP 中间件；M5 推送 | 可行：长连接路由豁免请求期限、解除写超时；每个浏览器一条事件流，令牌到期时关闭 |
| ④ | CodeMirror 6 与 React 19 的集成：挂载与卸载、受控与非受控、Markdown 语言包与高亮、扩展的组合方式（对应编辑器扩展管线）、**中文输入法**的组合输入 | M4 编辑器 | 可行：只创建一次 `EditorView`，切换页面时新建 `EditorState`；真实输入法与扩展管线移交 M4 |
| ⑤ | Go 的 MCP SDK：Streamable HTTP 挂在自己的路由上、`Authorization` 头认证、每个请求取得当前账户、instructions、prompts、clientInfo；用 Claude Code 与 Codex 实际连接 | P3 的路由挂载方式；M9 | 可行：go-sdk v1.8.0；关闭回环保护、设置会话超时；重启恢复移交 M9 |

### 前序 Phase 对后续 Phase 的要求

P1 的实验结论、P2 到 P4 的审查和实施留下的要求，开工时逐条落实（M0 之外的留给 M1，见 M1 的移交：[P3 平台层](../M1-auth/handoffs/M0-P3-platform.md)、[P4 接口契约](../M1-auth/handoffs/M0-P4-api-contract.md)）：

| Phase | 要求 | 来源 |
|---|---|---|
| P2 | 开发用 compose 以 `POSTGRES_INITDB_ARGS="--locale-provider=builtin --locale=C.UTF-8"` 初始化 PostgreSQL；持续集成的 lint 任务运行 `node tools/md-fixtures/check.mjs`；`.gitignore` 忽略 `.playwright-mcp/` 这类工具产物 | ①、② |
| P3 | 第一条迁移 `CREATE EXTENSION IF NOT EXISTS pg_trgm`。迁移之后做数据库 locale 自检（编码 UTF8、`datlocprovider = 'b'`、`datctype = 'C.UTF-8'`、`show_trgm('中文')` 不为空），不满足就拒绝启动并给出建库命令；自检有集成测试（包括一个 `LC_CTYPE 'C'` 的反例库）。`pgtest` 的容器同样按 builtin `C.UTF-8` 初始化 | ① |
| P3 | HTTP 平台层支持按路由豁免请求期限；处理器可以通过 `http.ResponseController` 在连接上解除写超时。M5 的 SSE 与 M9 的 MCP 都挂在这类路由上；M0 用测试路由验证豁免与不豁免两种行为 | ③、⑤ |
| P5 | 前端不引入 unified / remark / rehype；编辑器（CodeMirror 6）不在 M0 引入 | ②、④ |
| P6 | e2e 的 PostgreSQL 容器同样按 builtin `C.UTF-8` 初始化 | ① |
| P3 | `pgtest` 使用与开发库相同的镜像 `postgres:18.6-trixie`（镜像带发行版名，固定 glibc 基线） | P2 审查 |
| P3 | 配置的环境变量覆盖只处理带 `__` 的 `NWIKI_` 变量（例如 `NWIKI_DATABASE__URL`），并保留对应测试；否则开发用的 `NWIKI_DEV_DB_PORT` 会被当成未知配置键，服务拒绝启动 | P2 审查 |
| P3 | 架构测试"禁止全局可变状态"要把 `buildinfo.version` 列为例外：它是 `-ldflags -X` 注入所必需的包级变量 | P2 审查 |
| P3 | 需要时只加 `errorlint`；P2 审查在 Nerve 平台层的副本上试过另外 14 个 linter，非测试代码的报告基本是噪音 | P2 审查 |
| P5 | oxlint 加入 React、jsx-a11y 插件与浏览器全局变量的限制，按路径设定 `web/**` 的运行环境；`.gitignore`、`.oxlintrc.json` 加入前端产物目录；评估 turbo 是否真的需要（`pnpm -r` 可能够用） | P2 审查 |
| P5 | 升级到 Node 26 时注意：Node 25 起不再自带 corepack，`corepack enable` 这一步与 README 要调整 | P2 审查 |
| P6 | e2e 与镜像使用 `postgres:18.6-trixie`；在 Docker 中构建时没有 `.git`，`buildinfo` 的提交信息要用 ldflags 注入，或者把 `.git` 带进构建上下文 | P2 审查 |
| P4 | 接口操作的逐路由中间件（请求期限、请求体上限）与 `APIErrors`，配置项 `server.request_timeout`（必须短于 `write_timeout`）、`server.max_body_bytes`；`httpserver.ProblemError` 与组合根中 `*shared.Error` 满足它的编译期断言；契约测试核对 `shared.FieldCodes()` 与接口描述的字段错误码、`Router.Patterns()` 与接口描述的路径。长连接路由（`LongLived`）不经过逐路由中间件，用测试固定这一点 | P3 |
| P4 | `apitest` 引入 kin-openapi 时，把它加进架构测试 `binary_test` 的禁用清单；引入生成代码时，恢复 Nerve 的 `generated_test`（生成代码只用标准库的 `uuid`）。oapi-codegen runtime 的例外随第一个带参数的操作移到 M1 | P3 |
| P5 | `webui` 挂在组合根的 `/`（不带方法），不遮住平台的 `/api/` 兜底；页面 CSP 与静态文件的缓存由 `webui` 设置，安全头由中间件链统一设置 | P3 |
| P5 | 前端整体用哪个 TypeScript 版本：api-client 因 openapi-typescript 调用 TypeScript 的 JS API 停在 5.9.3；`make lint-web` 已执行各包的 `check:types` | P4 |
| P6 | `serve` 启动时要连上数据库（先迁移、再自检），连不上就拒绝启动。S1 的"数据库不可用"要在启动之后制造（例如删掉该 worker 的数据库）；"迁移未完成"用 `auto_migrate: false` 启动在未迁移的库上 | P3 |

## 8. 本 M 建立的平台约定

总体设计的扩展点表（12.4）中没有 M0 的条目。M0 建立的是每个模块都要遵守的平台约定，在 M0 收尾时补进总体设计第 13 节：

- **模块接入契约**：`module.go` 提供 `New`（有依赖时 `New(Deps)`）与 `Register(router, api)`，M1 加入认证时再加 `PublicOperations()`；只有组合根导入模块。
- **接口契约**：每个操作声明 `security` 与 `x-problem-codes`；契约测试双向核对错误码。
- **配置**：每个模块的配置是 `config.yaml` 中的一节，强类型、启动时校验。
- **测试**：集成测试从模板库复制独立的数据库；端到端故事的写法（页面版本 + 接口版本、数据库断言函数按表放在 `e2e/fixtures/assert/`）。

## 9. 测试策略

| 层次 | M0 覆盖 |
|---|---|
| 单元 | 平台层各包：配置加载与校验、problem+json、中间件顺序、请求体结构检查、迁移器、webui 的 CSP 与路由兜底；表格驱动 |
| 集成 | 连接池、事务管理器（提交、回滚、提交不受请求期限取消）、迁移、数据库 locale 自检；testcontainers 启动 PostgreSQL 18（builtin `C.UTF-8`），每个测试从模板库复制独立数据库 |
| 契约 | `instance` 的响应符合打包后的接口描述；声明的错误码都被测试返回过 |
| 架构 | 依赖方向、模块边界、平台包之间互不依赖、生成代码只被所属适配器导入 |
| 前端 | vitest：文案一致性检查、客户端封装、instance store |
| 端到端 | S1–S4 |

测试针对行为，不针对实现细节；琐碎的接线代码不单独写测试，由集成测试和端到端测试覆盖。

## 10. 风险

| 风险 | 应对 |
|---|---|
| 拷贝的代码带着 Nerve 的隐含假设 | "拷贝即接管"：P3、P4、P6 的审查逐个文件过一遍 |
| P1 的结论推翻总体设计的某个选择（例如 pg_trgm 不能处理中文） | P1 在最前面；结论直接修订总体设计并记入变更记录，再开始 P2。已发生：② 改为服务端唯一解析，① 固定 `LC_CTYPE`，均已修订 |
| 持续集成里的 testcontainers 与 Playwright 耗时 | 集成测试共用一个容器、按测试复制模板库；e2e 按 worker 并行 |

## 11. Phase 进度表

| P | 名称 | 状态 | Phase 文档 | 审查 |
|---|---|---|---|---|
| P1 | 技术验证 | 已完成 | [01-P1-spikes.md](01-P1-spikes.md) | [P1-spikes-review.md](reviews/P1-spikes-review.md) |
| P2 | 仓库与工具链 | 已完成 | [02-P2-repo-toolchain.md](02-P2-repo-toolchain.md) | [P2-repo-toolchain-review.md](reviews/P2-repo-toolchain-review.md) |
| P3 | 服务端平台层 | 已完成 | [03-P3-server-platform.md](03-P3-server-platform.md) | [P3-server-platform-review.md](reviews/P3-server-platform-review.md) |
| P4 | 接口契约与代码生成 | 已完成 | [04-P4-api-contract.md](04-P4-api-contract.md) | [P4-api-contract-review.md](reviews/P4-api-contract-review.md) |
| P5 | 前端外壳与内嵌 | 进行中 | [05-P5-web-shell.md](05-P5-web-shell.md) | — |
| P6 | 端到端测试与交付 | 未开始 | — | — |
| — | M0 收尾审查 | 未开始 | — | — |

## 12. 变更记录

| 日期 | 修订 | 原因 |
|---|---|---|
| 2026-09-30 | 初版 | M0 启动 |
| 2026-09-30 | 接住 P1 的结论：P1 验证表加"结论"一列；新增"P1 结论对 M0 各 Phase 的要求"（P2 的建库参数与样例集自检，P3 的 pg_trgm 迁移、locale 自检、长连接路由的豁免，P5 不引入 remark，P6 的建库参数）；范围、仓库布局、测试策略、风险与进度表相应更新 | M0/P1 完成，见 [P1 审查记录](reviews/P1-spikes-review.md) |
| 2026-09-30 | P2 完成：`server/tools` 推迟到 P4、turbo 推迟到 P5、`buildinfo` 提前到 P2（第 6、7 节）；"P1 结论对 M0 各 Phase 的要求"改为"前序 Phase 对后续 Phase 的要求"，加入 P2 审查给 P3、P5、P6 的提示 | P2 的实施与审查，见 [P2 审查记录](reviews/P2-repo-toolchain-review.md) |
| 2026-09-30 | P3 完成：`webui` 移到 P5，接口操作的逐路由中间件与 `APIErrors` 移到 P4（第 6、7 节）；"前序 Phase 对后续 Phase 的要求"加入 P3 给 P4、P5、P6 的要求；M0 之外的移交 M1 | P3 的实施与审查，见 [P3 审查记录](reviews/P3-server-platform-review.md) |
| 2026-09-30 | P4 完成：认证相关、参数与请求体的整个程序测试、oapi-codegen runtime 的例外移到 M1（第 7 节）；模块接入契约的措辞按实现修订（第 8 节）；"前序 Phase 对后续 Phase 的要求"加入 P4 给 P5 的 TypeScript 版本 | P4 的实施与审查，见 [P4 审查记录](reviews/P4-api-contract-review.md) |
