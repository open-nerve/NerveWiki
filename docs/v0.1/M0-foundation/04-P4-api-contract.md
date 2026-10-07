# M0/P4 接口契约与代码生成：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M0/P4 接口契约与代码生成 |
| 状态 | 已完成 |
| 基线 | `91c6fb7`（P3 完成：平台层、组合根、命令行、架构测试） |
| 上级文档 | [M0 总设计](00-M0-design.md) 第 7 节；[总体设计](../v0.1-design.md) 6.1 |

---

## 1. 基线

P3 留下的：
- 平台层 `config`、`logging`、`clock`、`postgres`、`httpserver`（中间件链、`/healthz`、`/readyz`、`/api/` 兜底、problem+json、`LongLived`）；
- 共享内核 `shared`（错误模型、`TxManager`）；组合根 `bootstrap`；命令行 `cmd/nervewiki`；架构测试 `archtest`。

还没有任何接口操作，也没有前端代码；pnpm 工作区只有根目录。

前序 Phase 对本 Phase 的要求（M0 总设计"前序 Phase 对后续 Phase 的要求"）：

1. 接口操作的逐路由中间件（请求期限、请求体上限）与 `APIErrors`；配置项 `server.request_timeout`（必须短于 `write_timeout`）、`server.max_body_bytes`。
2. `httpserver.ProblemError`，组合根中 `*shared.Error` 满足它的编译期断言。
3. 契约测试核对 `shared.FieldCodes()` 与接口描述的字段错误码，以及 `Router.Patterns()` 与接口描述的路径。
4. 长连接路由（`LongLived`）不经过逐路由中间件，用测试固定。
5. `apitest` 引入 kin-openapi 时，把它加进架构测试 `binary_test` 的禁用清单；引入生成代码时，恢复"生成代码只用标准库的 `uuid`"的检查。

## 2. 目标与范围

**目标**：走通"写接口描述 → 生成代码 → 实现 → 契约测试 → TS 客户端"的全链路，定下以后每个模块照着做的写法。第一个模块 `instance` 提供 `GET /api/v0/instance`。

**做**：
- 接口描述 `api/`：入口、公共组件、`instance` 模块的描述、Redocly 打包出的 `api/dist/openapi.yaml`。
- 独立的 Go 工具模块 `server/tools`：oapi-codegen，以及请求体结构表的生成器 `bodyshapegen`。
- 平台：`APIErrors`（生成代码的三个错误出口与处理器返回的错误都答成 problem+json）、`ProblemError`、接口操作的逐路由中间件（请求期限 → 请求体上限 → 请求体结构检查）、`bodyshape`、`apigen`（公共组件的 Go 类型）。
- 契约测试工具 `apitest`（kin-openapi），以及平台、模块、整个程序三层的契约测试。
- 试点模块 `instance`，它的目录结构就是以后每个模块的模板。
- TS 客户端 `web/packages/api-client`（openapi-typescript + openapi-fetch）。
- `make gen`、`make gen-check` 与按工具链拆分的命令；持续集成检查生成物已提交。

**不做**，以及与 M0 总设计的差异（本 Phase 完成时同步修订 00 号文档）：

| 内容 | 去处 | 理由 |
|---|---|---|
| 认证相关：`Authenticator`、公开操作清单 `PublicOperations()`、401 的 `WWW-Authenticate`（以及 problem 响应对这个头的声明）、`security: [bearer]` 的 scheme | M1 | 第一个需要令牌的操作在 M1。本 Phase 要求每个操作显式声明 `security`（`instance` 是 `[]`），为 M1 留好位置 |
| 整个程序的"参数绑定失败答 400""请求体结构不对答 400"两项测试，`apitest` 为它们推导用例的部分，以及只为它们服务的两条写法规则（参数写 `schema` 不写 `content`；JSON 请求体是对象）；oapi-codegen `runtime` 在二进制依赖检查中的例外 | M1 | `instance` 没有参数、没有请求体，这两项测试没有对象；第一个带参数或请求体的操作引入 `runtime`，也在 M1。`bodyshape` 与 `bodyshapegen` 本身在本 Phase 就位，由它们自己的单元测试和样例覆盖 |
| 分页的公共组件（`Limit`、`Cursor`、`NextCursor`） | 第一个列表接口 | 没有使用者 |

## 3. 设计

### 3.1 目录与职责

```
api/
  openapi.yaml             打包入口：info、tags、顶层 x-problem-codes；每个路径指向模块文件
  common.yaml              公共组件：Problem、FieldError（只放 schemas、parameters）
  modules/instance.yaml    一个模块一个文件，本身是完整的 OpenAPI 3.1 文档
  redocly.yaml             Redocly 配置
  dist/openapi.yaml        打包结果（生成，提交）：服务端契约测试与 TS 类型的唯一依据
server/
  tools/                   独立的 Go 模块：oapi-codegen（tool 指令）、bodyshapegen
  internal/platform/httpserver/
    api.go                 API：逐路由中间件
    apierrors.go           ProblemError、APIErrors
    apigen/                公共组件的 Go 类型（生成）与 oapi-codegen 配置
    bodyshape/             请求体结构检查（只依赖标准库）
    apitest/               契约测试工具，只被测试导入
  internal/modules/instance/
    domain/  app/  adapter/buildinfo/  adapter/http/（包名 httpadapter）  adapter/http/gen/（生成）  module.go
web/packages/api-client/   @nervewiki/api-client：生成的类型与 createClient
```

依赖方向（新增的边）：

```
bootstrap ──► modules/instance ──► instance/{app, adapter/*}, platform/httpserver
instance/adapter/http ──► instance/app, instance/adapter/http/gen, platform/httpserver
instance/adapter/http/gen ──► platform/httpserver/{apigen, bodyshape}
instance/adapter/buildinfo ──► instance/domain, platform/buildinfo
instance/app ──► instance/domain
platform/httpserver ──► platform/httpserver/bodyshape
platform/httpserver/apitest ──► kin-openapi（只被测试导入）
```

`platform/httpserver/bodyshape` 与 `apigen` 是 `httpserver` 的子包，不违反"平台包之间互不导入"（规则按平台包的第一级目录判断）。

### 3.2 接口描述的写法

入口文件 `api/openapi.yaml` 是整个接口的目录：`paths` 中每个路径写成 `$ref: 'modules/<模块>.yaml#/paths/~1api~1v0~1…'`，新路径必须在这里加一行（漏加时测试失败）。模块文件路径写全（`/api/v0/…`），可以单独交给 oapi-codegen。

每个操作：
- `operationId` 是首字母小写的 camelCase；`tags: [<模块>]`，tag 在入口文件中声明；
- 显式声明 `security`（公开操作写 `[]`）；
- 声明 `x-problem-codes`：它自己可能返回的错误码；入口文件顶层的 `x-problem-codes` 是每个操作都可能返回的平台码（`bad_request`、`payload_too_large`、`internal_error`）；
- 有 `default` 响应，引用本模块文件的 `components.responses.Problem`，它的 schema 是 `../common.yaml#/components/schemas/Problem`，并声明 `Retry-After` 头（`RetryAfter()` 为正时出现）。

写法约定（Nerve 用 OpenAPI 3.1 在 Redocly、oapi-codegen、kin-openapi、openapi-typescript 整条链路上验证过，本项目沿用，由 `apitest` 的规则测试检查）：

| 场景 | 这样写 | 不要这样写 | 原因 |
|---|---|---|---|
| 可为空的标量 | `type: [string, 'null']` | `nullable: true` | 3.0 的写法 |
| 可为空的枚举或对象 | `anyOf: [{$ref: …}, {type: 'null'}]` | `enum: [a, b, null]`；`oneOf` | 前者让 Go 多出一个 `"<nil>"` 常量；bodyshapegen 在请求体中只认 `anyOf`（两者生成的 Go 与 TS 相同） |
| 固定值、`discriminator` 的属性 | `type: string` + 单值 `enum` | `const` | `const` 在 Go 中生成 `interface{}` |
| 引用公共组件 | `../common.yaml#/components/schemas/…`、`…/parameters/…` | `…/responses/…` | 跨文件引用 response，oapi-codegen 生成的代码编译失败 |
| 组件名 | PascalCase，所有模块文件中唯一 | 同名不同义 | Redocly 打包时静默改名为 `Name-2` |
| 对象 schema | `additionalProperties: false` | — | 契约测试才能发现多出来的字段 |
| 路径 | 不以 `/` 结尾 | `/api/v0/pages/` | ServeMux 把它注册成子树，吞掉其下所有路径，也遮住 `/api/` 兜底 |

### 3.3 代码生成

| 描述 | 生成器与配置 | 输出 |
|---|---|---|
| `api/common.yaml` | oapi-codegen，`platform/httpserver/apigen/oapi-codegen.yaml`（只生成 models，`skip-prune`；决定类型的选项与模块模板相同） | `apigen/components.gen.go` |
| `api/modules/<m>.yaml` | oapi-codegen，`modules/<m>/adapter/http/gen/oapi-codegen.yaml`（models + std-http-server + strict-server；`import-mapping` 把 `../common.yaml` 指向 `apigen`） | `gen/server.gen.go` |
| `api/modules/<m>.yaml` | bodyshapegen，读同一份 oapi-codegen 配置 | `gen/bodyshape.gen.go` |
| `api/openapi.yaml` | Redocly `bundle` | `api/dist/openapi.yaml` |
| `api/dist/openapi.yaml` | openapi-typescript（`--root-types --root-types-no-schema-prefix`） | `web/packages/api-client/src/schema.gen.ts` |

模块的 oapi-codegen 配置是每个模块照抄的模板，一开始就定下以后改了会改名的选项：
- `always-prefix-enum-values: true`、`name-normalizer: ToCamelCaseWithInitialisms`；
- `nullable-type: true`：可为空又可省略的字段生成 `nullable.Nullable[T]`，PATCH 能区分"没传"和"传 `null`"；
- `type-mapping`：`format: uuid` 映射到标准库 `uuid.UUID`（否则引入 `github.com/google/uuid`）；`format: email` 生成 `string`（格式由领域层校验）；不带 format 的 `number` 生成 `float64`（`bodyshape` 覆盖得到它的范围）。

模块的请求体引用公共组件时，strict handler 解码进 `apigen` 的类型，而 bodyshapegen 按模块配置建结构表，所以 `apigen` 与每个模块的配置在 `compatibility`、`name-normalizer`、`nullable-type`、`type-mapping` 上必须一致，由 bodyshapegen 的测试核对。

Go 的生成只需要 Go（直接读 `api/common.yaml` 与模块文件，不读 `dist`）；`dist` 与 TS 类型需要 Node。生成的文件都以 `Code generated … DO NOT EDIT.` 开头，golangci-lint 跳过它们。

### 3.4 平台：错误与逐路由中间件

**`ProblemError`**：`error` 加 `ProblemStatus() int`、`ProblemCode() string`；可选的 `ProblemFields() []error`、`RetryAfter() time.Duration`。`*shared.Error` 按结构满足它，组合根有编译期断言。

**`APIErrors`**：从错误到 problem 只有一条路。

| 出口 | 何时 | 回答 |
|---|---|---|
| `BadRequest`（生成代码的 `ErrorHandlerFunc`） | 路径、查询、头部参数绑定失败 | 400 `bad_request`，`errors` 中给出参数名；debug 日志只记参数名与错误的类型：错误消息含 Go 的类型名与原值，不记（M6 Codex 评审修复核对 B3-M2） |
| `BodyError`（`RequestErrorHandlerFunc` 与 bodyshape） | 请求体读不出、不是 JSON、结构不对 | `ProblemError` 与 `*http.MaxBytesError` 交给 `Write`；其余 400 `bad_request`，细节只进 debug 日志 |
| `Write`（`ResponseErrorHandlerFunc`，以及中间件） | 处理器返回的错误 | `ProblemError`：它的状态、码、detail、fields、`Retry-After`；`*http.MaxBytesError`：413 `payload_too_large`；客户端已断开的 `context.Canceled`：debug 日志，不记 500；请求已过期限的 `context.DeadlineExceeded`：warn 日志，500 `internal_error`；其余：error 日志，500 `internal_error`，不带 detail |

响应已经开始时（`statusRecorder` 显示状态已发出），`Write` 记 warn 日志并中断连接，不在已发出的响应后面追加 problem。

**`API`**：平台交给每个模块 HTTP 适配器的值，含 `Errors` 与 `Middlewares(bodies)`。逐路由中间件按顺序：

```
请求期限（server.request_timeout，取消请求的 context）→ 请求体上限（server.max_body_bytes）→ 请求体结构检查（bodyshape）
```

它们只经由生成代码的 `StdHTTPServerOptions.Middlewares` 挂在接口操作上（生成代码把列表的最后一个包在最外层，所以 `Middlewares` 按相反顺序返回）。请求期限在最外层：读请求体的时间（以及 M1 的认证、限流）都算在期限内，由测试固定。长连接路由直接注册在路由器上，不经过它们；测试固定"接口操作的 context 带请求期限、长连接路由的 context 没有"。

**`bodyshape`**：在生成的 strict 处理器解码之前检查 JSON 请求体的结构：能否唯一地读出（同名成员、非法 UTF-8）、JSON 类型、未声明的属性、不允许的 `null`、缺少的必填属性，以及生成代码会解码成 Go 类型的字符串格式（`date-time`、`uuid`）。同一类问题一次收集，最多 16 个，路径最长 256 字节。取值（长度、枚举、范围）归领域层。每个模块的结构表由 `bodyshapegen` 生成。

**配置**：

```yaml
server:
  # 每个接口操作的期限：到期取消请求的 context，数据库调用随之结束。必须短于 write_timeout
  request_timeout: 15s
  # JSON 接口的请求体上限（字节），超出时 413
  max_body_bytes: 1048576
```

### 3.5 模块的写法（`instance`）

```
modules/instance/
  domain/info.go                Product、APIVersion 常量；Build、Info 值对象
  app/ports.go                  InfoSource 端口（由使用方声明）
  app/get_info.go               GetInfo 用例
  adapter/buildinfo/source.go   InfoSource 的实现：读取 platform/buildinfo
  adapter/http/handler.go       实现生成的 StrictServerInterface；Register 挂载路由
  adapter/http/gen/             oapi-codegen.yaml；server.gen.go、bodyshape.gen.go（生成）
  module.go                     New() *Module；(*Module).Register(router, api)
```

- `module.go` 是模块唯一的入口：`New` 装配用例与适配器（`instance` 没有依赖；有依赖的模块写成 `New(Deps)`），`Register` 把生成的路由挂在根路由器上（比平台的 `/api/` 兜底更具体），接上 `api.Errors` 与 `api.Middlewares(gen.BodyShapes())`。
- 处理器只做类型转换；用例没有 I/O 就不带 `ctx` 和 `error`。
- `GET /api/v0/instance` 返回 `product`（`Nerve Wiki`）、`version`、`commit`（没有 VCS 信息时为 `unknown`）、`api_version`（`v0`）。实例的设置项（例如是否开放注册）随提供它们的 M 加入。

### 3.6 契约测试

**`apitest`**（只被测试导入；kin-openapi）：
- `Load`：读取并校验 `api/dist/openapi.yaml`；位置由 `runtime.Caller` 求出。`go test` 的缓存不跟踪 `server/` 之外的文件，`make test` 已经带 `-count=1`。
- `CheckResponse`：按请求找到操作，校验状态码、`Content-Type`、响应体；problem 的码必须在该操作可能返回的码中（操作的 `x-problem-codes` 加顶层的），并记下来。
- `CheckRequest`、`CheckSchema`、`Enum`。
- `Main(m)`：模块 HTTP 适配器的 `TestMain`，模块名取自调用者所在的目录。测试跑完后，该模块每个操作声明的每个码都必须被某个测试答过一次（总体设计 6.1 的"双向核对"：答出的码必须已声明，声明的码必须被答过）。每个模块的 `adapter/http` 都必须调用它，由测试检查。
- 规则测试：3.2 的写法约定，以及每个路径以 `/api/v0/` 开头、`operationId`、tag、显式的 `security`、`x-problem-codes`、`default` 响应与它的头、组件名。每条规则先在手写的小文档上证明会报错，再作用于 `dist`。
- 入口文件列出模块文件的全部路径。

**三层测试**：

| 测试 | 内容 |
|---|---|
| 平台 `httpserver/contract_test` | 平台写出的每一种 problem（兜底 404、`/readyz` 503、panic 500、`BadRequest`、`BodyError`、`Write` 的各分支、带 `errors` 的）都符合 `Problem` schema |
| 模块 `instance/adapter/http` | 响应符合契约，内容正确；`apitest.Main` 核对错误码 |
| 整个程序 `bootstrap` | `GET /api/v0/instance` 经完整的中间件链返回 200 并符合契约；挂上模块后兜底仍然有效（未知路径、方法不对都是 404 problem）；`/api/` 下注册的路由（平台兜底除外）正好是契约中的操作；`shared` 的每个 `Kind` 答成对应的 problem；`shared.FieldCodes()` 与契约的 `FieldError.code` 枚举是同一个集合；每个操作的 problem 响应声明了它可能带的头 |

**架构测试**：规则 8 的测试辅助包加入 `apitest`；二进制禁用清单加入 kin-openapi；恢复"生成代码不使用 `oapi-codegen/runtime/types` 的 `UUID`"的检查。

### 3.7 TS 客户端

`web/packages/api-client`，包名 `@nervewiki/api-client`：
- `src/index.ts` 导出 `createClient(options)`（openapi-fetch 的 `createClient<paths>`）、类型 `ApiClient` 与生成的全部类型；不写转换层。
- `src/schema.gen.ts` 由 `make gen-web` 生成。
- `test/client.typecheck.ts` 只参与类型检查：断言 `data.api_version` 的类型是 `"v0"`、`error.code` 可用；用 `@ts-expect-error` 断言不存在的路径与 `POST /api/v0/instance` 无法编译。
- 脚本 `gen`、`check:types`（`tsc --noEmit`）。`make lint-web` 执行各包的 `check:types`；oxlint 覆盖 `web/`；oxfmt、oxlint 排除生成的 `schema.gen.ts` 与 `api/dist/`。
- TypeScript 用 5.9.3：openapi-typescript 7.13.0 调用 TypeScript 的 JS API（peer `^5.x`），而 TypeScript 7 只提供原生编译器、没有 JS API。前端整体用哪个版本由 P5 决定。

### 3.8 命令与持续集成

| 命令 | 作用 |
|---|---|
| `make gen` | `gen-go` + `gen-web` |
| `make gen-go` | 删除旧的 `*.gen.go`，生成 `apigen` 与每个模块的 `gen`（只需要 Go） |
| `make gen-web` | Redocly 打包 `api/dist/openapi.yaml`，生成 TS 类型（需要 Node） |
| `make gen-check`、`gen-check-go`、`gen-check-web` | 重新生成后，生成物必须已提交且没有差异（`git status --porcelain`，未跟踪的新文件也算） |

- `make lint-go` 另跑 `server/tools` 模块；`make test` 另跑 `go -C server/tools test`。
- 持续集成：`server` 任务加 `make gen-check-go`；`web` 任务加 `make gen-check-web`，`lint-web` 含类型检查。

### 3.9 依赖版本

| 工具 / 库 | 版本 | 位置 |
|---|---|---|
| oapi-codegen | v2.8.0 | `server/tools/go.mod`（`tool` 指令） |
| kin-openapi | v0.149.0 | `server/go.mod`（只有 `apitest` 导入）；`server/tools` 中随 oapi-codegen |
| `@redocly/cli` | 2.55.0（2.56.0 发布不满 1 天，被 `minimumReleaseAge` 挡住） | 根 `package.json` |
| openapi-typescript | 7.13.0 | api-client 的 devDependencies |
| openapi-fetch | 0.17.0 | api-client 的 dependencies |
| typescript | 5.9.3 | api-client 的 devDependencies |

## 4. 实施步骤

在分支 `m0-p4-api-contract` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `server/tools`（oapi-codegen、bodyshapegen）；`api/` 与 Redocly；`apigen`；`make gen*` | [P4-S1-toolchain.md](plans/P4-S1-toolchain.md) |
| S2 | `ProblemError`、`APIErrors`、`API` 与配置项；`bodyshape` | [P4-S2-api-errors.md](plans/P4-S2-api-errors.md) |
| S3 | `apitest` 与平台契约测试；架构测试的更新 | [P4-S3-apitest.md](plans/P4-S3-apitest.md) |
| S4 | `instance` 模块与组合根的接线；整个程序的契约测试 | [P4-S4-instance.md](plans/P4-S4-instance.md) |
| S5 | TS 客户端；持续集成；README | [P4-S5-api-client.md](plans/P4-S5-api-client.md) |

之后是反向对照、独立审查、修复、合并。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | `APIErrors` 的每个分支与日志；`API` 中间件的顺序、期限、上限；`bodyshape` 的检查与代价上限；`bodyshapegen` 的样例（golden 文件）；配置的新键与交叉校验 |
| 契约 | 3.6 的三层；`apitest` 的规则与反例 |
| 架构 | 3.6 的更新 |
| TS | `tsc --noEmit` 通过；类型检查用例中的 `@ts-expect-error` 都生效 |

反向对照（验证后撤销）：
- 改接口描述不重新生成 → `make gen-check-go`、`gen-check-web` 都失败；新模块的生成物未提交 → 失败；
- 改掉 `Problem` 的一个 JSON 字段名 → 平台契约测试失败；
- 响应多出一个字段、缺一个必填字段 → 模块契约测试失败；
- 操作声明一个码而没有测试答它 → `apitest.Main` 失败；答出一个未声明的码 → `CheckResponse` 失败；
- 契约中多一个没有实现的操作、实现中多一个契约没有的路由 → 整个程序的测试失败；
- `shared` 多一个字段错误码而契约没有 → 失败；
- 非测试代码导入 `apitest` → 架构测试失败；
- 把长连接路由包进逐路由中间件 → 测试失败；
- TS 中调用不存在的操作 → 类型检查失败。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check` 本地与持续集成为绿。
- `make run` 后 `curl localhost:8080/api/v0/instance` 返回 200 与版本信息；`POST /api/v0/instance` 与未知的 `/api/` 路径返回 404 problem+json。
- 审查完成（`reviews/P4-api-contract-review.md`），发现的问题已修复。
- M0 总设计按第 2 节的差异修订，进度表更新。

## 7. 结果

**完成**：第 5 节全部通过，反向对照按预期失败。`make check`、`make gen-check` 本地与持续集成（run 36702688431）为绿。`make run` 之后第 6 节的三个请求如设计：`GET /api/v0/instance` 答 200 与版本信息，`POST /api/v0/instance` 与 `/api/v0/nope` 答 404 problem+json。

**与设计的差异**：

1. bodyshape、bodyshapegen 与 api-client 的骨架提前到 S1：生成代码导入 bodyshape，`gen-web` 需要 api-client 这个包。
2. `instance.New()` 不带参数，因为模块没有依赖；有依赖的模块写 `New(Deps)`（3.5 已改）。
3. 长连接路由的测试经过全局中间件链，验证的是真实的组合，而不只是路由器。
4. apitest 的平台码清单、码的拼写规则、模块清单只有规则测试使用，放在测试文件中。
5. 审查之后的修订，第 3 节已是修订后的版本：
   - `apigen` 决定类型的选项与模块模板一致（3.3）；
   - 可为空的对象写 `anyOf`，路径不以 `/` 结尾（3.2）；
   - 请求期限在最外层由测试固定，已过期限的请求单独记 warn 日志（3.4）；
   - `Main(m)` 自己得出模块名，每个模块必须调用它（3.6）；
   - 整个程序的测试比较 `/api/` 下的全部路由（3.6）。

**审查**：[P4 审查记录](reviews/P4-api-contract-review.md)，1 项 Important、7 项 Minor、6 项 Nit，全部处理。

**移交**：[M1 的移交](../M1-auth/handoffs/M0-P4-api-contract.md)，包括以下几项：
- 认证与公开操作；
- 参数与请求体的整个程序测试，以及只为它们服务的两条写法规则；
- 二进制禁用清单对 runtime 的例外；
- 逐路由中间件的扩充。
