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
| `users …`        | 服务器管理员的账户命令，见下文"账户与认证"                     |
| `version`        | 打印版本号与构建信息                                           |

### 配置

配置按层合并，后面的覆盖前面的：

1. 内置的 `server/configs/config.yaml`（列出全部配置项及默认值）；
2. 内置的 `server/configs/config.<env>.yaml`；
3. `$NWIKI_CONFIG_DIR` 下的 `config.yaml`、`config.<env>.yaml`（设置了且文件存在时）；
4. 个人覆盖文件 `configs/config.local.yaml`，相对于工作目录，也就是 `server/` 下启动时（`make run` 就是）的 `server/configs/config.local.yaml`；只在 dev 生效，不进仓库；
5. 环境变量 `NWIKI_<节>__<键>`，例如 `database.url` 对应 `NWIKI_DATABASE__URL`。

`NWIKI_ENV` 选择环境（`dev`、`test`、`prod`，默认 `dev`）。未知的键、空值、越界的数字、不带单位的时长都会报错，所有无效的键一次列出。日志里的数据库地址整体脱敏。

### 账户与认证

除了 `POST /api/v0/auth/{register,login,refresh,logout}` 与 `GET /api/v0/instance`，每个接口都要求 `Authorization: Bearer <访问令牌或个人访问令牌>`，否则答 401。

- **注册**：`auth.signup_enabled`，dev、test 开放，prod 关闭（`GET /api/v0/instance` 的 `signup_enabled` 告诉客户端）。关闭时，带着工作区邀请（请求体的 `invitation: {id, token}`）的注册仍然可以，只要邀请还待接受、注册邮箱就是被邀请的那个；注册不替用户接受邀请。密码 8–128 个字符，不能是常见密码，也不能由邮箱 @ 之前的部分构成；常见密码名单由 `node tools/password-blocklist/build.mjs` 生成（取 SecLists 固定提交中的 NCSC 名单并核对校验和）。
- **会话**：注册与登录各开一个会话，返回访问令牌（15 分钟，`auth.access_token_ttl`）与刷新令牌。访问令牌到期后用刷新令牌换下一对（`/auth/refresh`），旧的刷新令牌随之作废；作废的刷新令牌再被使用，整个会话被撤销：这可能是它被别人拿到了，也可能是续期的答复在返回途中丢失（服务端已经换了令牌，客户端还拿着旧的），后者让用户重新登录，是严格轮换的代价（RFC 9700 4.14.2）。会话从登录起 30 天（`auth.session_ttl`）结束，续期不延长。`/auth/logout` 结束当前会话。续期与退出在 `auth.refresh_deadline`（4 秒）内完成，它加上 `database.commit_timeout` 必须小于前端放弃续期的 8 秒，且不超过 `server.request_timeout`；超时答 500，事务回滚，令牌不变，可以重试。
- **个人访问令牌（PAT）**：给脚本与集成用。`POST /api/v0/me/api-tokens` 创建，要求当前密码（`current_password`），可选期限 `expires_at`；响应里的 `token`（`nwk_pat_` 开头）只出现这一次，服务端只存它的 SHA-256。`GET /api/v0/me/api-tokens` 列出未撤销的令牌（不含令牌本身，含 `last_used_at`，每分钟至多更新一次），`DELETE /api/v0/api-tokens/{token_id}` 撤销，立即失效。PAT 与访问令牌一样用在 `Authorization: Bearer`，能做账户能做的一切，包括再创建 PAT。
- **账户**：`PATCH /api/v0/me` 改显示名；`POST /api/v0/me/onboarding-steps` 记录完成的引导步骤（步骤由前端定义）；`POST /api/v0/me/change-password` 要求当前密码，改后其他会话全部结束，当前会话保留（用 PAT 调用时全部结束），PAT 照常可用；`POST /api/v0/me/deactivate` 停用账户，所有会话结束，PAT 不能再用，登录答 403 `identity.account_deactivated`，只有管理员能重新启用；工作区里的规则见"工作区"一节的"停用账户"。
- **限流**：`ratelimit` 节的令牌桶，超出时答 429 `rate_limited` 与 `Retry-After`。公开操作按客户端 IP（`anonymous`），其余按凭证（会话或 PAT，`authenticated`）；校验当前密码的操作（改密码、创建 PAT）另按账户（`password_user`）；带令牌的请求在认证之前先过失败闸门（`auth_failure`，按客户端 IP：只有认证失败的令牌消耗名额，过期的访问令牌不算）；登录另按 IP 与"IP 加邮箱"（`login_ip`、`login_ip_email`），注册按 IP（`register_ip`）。IPv6 客户端按 `/64` 前缀计数（`ratelimit.ipv6_prefix_len`）。桶在进程内存中：多实例部署时每个实例各算各的。被拒绝的请求在访问日志中是 429；哪个桶拒绝的，平台的桶记在 debug 级，登录、注册与 `password_user` 的桶记在 info 级（`password_user` 另记账户 `user_id`）。
- **会话清理**：过期的会话由后台任务删除，服务启动时一次，之后每 `auth.session_cleanup_interval`（默认 1 小时）一次；多个实例时只有一个执行。
- **管理员命令**：服务器管理员在能连上数据库的机器上执行，与 `migrate` 一样加载配置（带上服务的那些变量），只连数据库，不启动 HTTP 与后台任务，服务不必停。账户用 `--email` 指定：

  | 命令                                                  | 作用                                                                                       |
  | ----------------------------------------------------- | ------------------------------------------------------------------------------------------ |
  | `users create --email <地址>`                         | 建账户，不看注册开关，不建会话                                                             |
  | `users reset-password --email <地址>`                 | 设新密码，撤销账户的全部会话与 PAT                                                         |
  | `users set-email --email <地址> --new-email <新地址>` | 改邮箱，撤销全部会话；PAT 照常可用，账户可能被盗时另执行 `reset-password`                  |
  | `users deactivate --email <地址>`                     | 停用，与自助停用相同：会话全部结束，PAT 在重新启用之前不能用，工作区的成员关系全部结束     |
  | `users activate --email <地址>`                       | 重新启用：未撤销、未过期的 PAT 恢复可用，会话不恢复；账户可能被盗时另执行 `reset-password` |

  `create` 与 `reset-password` 的密码从标准输入读，不接受参数与环境变量（会从 `ps` 泄露）：标准输入是终端时不回显地提示两次，两次要相同；否则读一行，只去掉行尾的换行，首尾空格是密码的一部分，例如 `printf '%s\n' "$PASSWORD" | nervewiki users create --email alice@corp.com`。结果一行写到标准输出（如 `password reset for alice@corp.com: revoked 2 sessions, 1 API token`），日志写到标准错误；失败时打印 `nervewiki: <原因>`，退出码 1。输出与日志里没有密码，日志只记账户的 `user_id`，不记邮箱。停用、启用在状态不变时什么也不做（`… is already deactivated`、`… is already active`）。

  工作区的命令（M2/P4），日志带 `by=cli`，同样不记邮箱：

  | 命令                                                             | 作用                                                                                                                             |
  | ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
  | `workspaces create --slug <slug> --name <名称> --admin <地址>`   | 建工作区，那个账户是它的管理员；不看 `workspace.creation_enabled`，关闭创建时工作区就这样建                                      |
  | `workspaces reactivate-member --workspace <slug> --email <地址>` | 恢复已结束的成员关系，角色沿用，加入的时刻不变；输出它结束的时刻（`… the membership had ended at …`）。账户要先 `users activate` |

  账户不存在、已停用，slug 已被占用或不合规则时，退出码 1，数据库不变。恢复不限于停用结束的成员关系：被移出、离开的也可以恢复，看输出的结束时刻确认恢复的是哪一次。成员关系已是有效的，什么也不做（`… is already a member of …`）。

- **签名私钥**：`auth.jwt.private_key_file`，PKCS#8 PEM 的 Ed25519 私钥，用 `openssl genpkey -algorithm ed25519 -out jwt.pem` 生成。prod 必须提供，缺了拒绝启动；dev、test 不提供时每次启动生成临时密钥，重启后已签发的令牌与邀请链接全部失效。刷新令牌与邀请令牌的 MAC 密钥都由它派生（HKDF，各用各的 info）：换私钥之后，待接受的邀请链接全部失效，要重新邀请。日志只记是否设置，不记路径。
- **反向代理**：`server.trusted_proxies` 列出代理的 CIDR（环境变量用逗号分隔，例如 `NWIKI_SERVER__TRUSTED_PROXIES=10.0.0.0/8`）。只有来自它们的 `X-Forwarded-For` 被采信，代理写入的必须是不带端口的 IP；会话记录的就是这样认出的客户端 IP。配置不对时服务各告警一次。
- 非 prod 的服务监听在回环地址之外时，启动时告警：这多半是忘了设 `NWIKI_ENV=prod` 的部署，注册开放、签名密钥是临时的。

### 工作区

- **创建**：`workspace.creation_enabled`，默认开启：每个账户都能创建工作区，创建者是它的管理员（`GET /api/v0/instance` 的 `workspace_creation_enabled` 告诉客户端）。关闭后创建答 403 `workspace.creation_disabled`。
- **slug**：工作区的地址段，1–48 个 a–z、0–9、`_`、`-`，创建后不能改；站点的顶层路径与留作以后用的名字不能用，名单在 `server/internal/modules/workspace/domain/reserved_slugs.txt`。
- **成员**：角色是 admin、member、guest。管理员改别人的角色、移出成员，不能改或移出自己；成员可以离开，唯一的管理员不能（`workspace.sole_admin`），先让别人成为管理员，或者删除工作区。成员列表对访客隐藏邮箱。
- **停用账户**（自助停用与 `users deactivate` 相同）：他是某个还有别的有效成员的工作区唯一的管理员时，停用被拒（409 `workspace.sole_admin`，原因里列出这些工作区的 slug），先让那里的另一位成员成为管理员；只有他一人的工作区不挡停用。停用结束他全部的成员关系，并删除这些工作区里发给他邮箱的待接受邀请。恢复：`users activate`，再对每个工作区 `workspaces reactivate-member`，或者由工作区的管理员重新邀请他。
- **软删除与清理**：删除工作区是软删除，连同它的成员与邀请；撤回、接受的邀请也是软删除。超过 `jobs.purge_retention`（默认 1440 小时，即 60 天）的，由后台任务物理删除，服务启动时一次，之后每 `jobs.purge_interval`（默认 1 小时）一次；多个实例时只有一个执行。保留期内运维可以从数据库恢复。
- **邀请**：管理员按邮箱邀请（`POST /api/v0/workspaces/{slug}/invitations`），把邀请的 id 与令牌（`nwk_inv_` 开头）发给对方；一个工作区里一个邮箱至多一份待接受的邀请，有效成员的邮箱不能邀请。令牌是 id 的 MAC，不存库，管理员随时可以在邀请列表里再看到它。任何拿到链接的人都能预览（工作区的名称与 slug、角色，不含邮箱）；接受要求用被邀请的邮箱登录，接受之后成为成员（已结束的成员关系恢复，保留第一次加入的时刻；已是成员的角色不变）。预览与接受都把令牌放在请求体里，不放在 URL 中；链接由页面拼出（M2/P5、P6），令牌放在 URL 片段（`#`）里，浏览器不把片段发给服务器。撤回邀请（`DELETE /api/v0/workspace-invitations/{id}`）、成员关系结束（对他邮箱的待接受邀请一并删除）、删除工作区之后，链接答 404 `workspace.invitation_not_found`。

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
3. 在 `adapter/http` 中实现生成的 `StrictServerInterface`，由模块的 `Register` 挂到路由器上，在 `bootstrap` 中调用。接口默认要求令牌：不要令牌的操作在契约中写 `security: []`，同时列进模块的 `PublicOperations()`，由 `bootstrap` 并进 `APIConfig`；整个程序的测试核对两者一致。
4. `adapter/http` 的测试以 `apitest.Main(m)` 为 `TestMain`（模块名取自测试所在的路径）：操作声明的每个错误码都要有测试经 `apitest.CheckResponse` 答过。

## 前端

`web/apps/web` 是 React 应用（Vite、React Router 的数据路由、TypeScript、Tailwind CSS、shadcn/ui 的组件写法、MobX、SWR），构建后内嵌进 `nervewiki`，与接口同源。

```bash
make dev       # 开发数据库 + 后端 + 前端热更新：打开 http://127.0.0.1:5173
make web-dev   # 只起前端开发服务器；/api、/healthz、/readyz 代理到 127.0.0.1:8080 上 make run 起的后端
make build     # 构建前端并内嵌进 bin/nervewiki
```

写法：

- 组件经由 store 取数据，store 经由 service 调接口，service 从构造函数拿 API 客户端。`src/stores/root.store.ts` 是唯一装配它们的地方：页面一生的 `AppStores` 与每次登录一代的 `RootStore`。组件不导入 `@nervewiki/api-client`，oxlint 检查这一点，接口类型从 service 导出。
- 加载由 SWR 驱动：页面 `useSWR(key, () => store.x.load())`，store 保存结果。示例见 `src/pages/landing.tsx`。
- 文案在 `src/i18n/messages/`：`en.ts` 是源头，`zh-CN.ts` 缺键、多键时类型检查失败，占位符不一致时 vitest 失败。组件用 `useT()`。
- 页面在 `src/app/routes.tsx` 中按需加载，写成 `const { Page } = await import(…)`，knip 才看得出用到了哪些导出。

登录、会话、引导与设置：

- **会话**：`src/session/` 是唯一创建 API 客户端的地方（oxlint 检查）：公开客户端（登录、注册、实例信息）与带访问令牌的客户端。浏览器只在 localStorage 的 `nwiki.auth` 中存刷新令牌与本次登录的 `login_id`；访问令牌只在内存中，到期前 30 秒续期。同一浏览器的标签页经 Web Locks 一次一个续期，没有 `navigator.locks` 的非安全上下文（如局域网地址的 HTTP）退回 localStorage 租约：租约只能尽力串行，极少数情况下两个标签页同时续期，会话被当作重复使用而结束，用户重新登录；需要严格串行的部署请用 HTTPS。一个标签页登录、退出或换了账户，其他标签页跟着变。浏览器写不进存储（本站的存储已满或被阻止）时无法保持会话：标签页注销它、回到登录页，登录时说明原因。
- **每次登录一代**：`SessionRoot` 按 `loginId` 新建一代 `RootStore`，SWR 缓存随之清空；上一代没有完成的请求以 `SessionChangedError` 结束，不写入新一代。设备偏好与实例信息跨代保留。
- **路由与守卫**（`src/app/guards.tsx`）：除登录、注册与邀请页外所有页面都要登录，不存在的路径也是先登录再显示 404。去向只由守卫决定：页面在登录、注册、退出之后不自己跳转。登录页的 `next` 只接受本站路径，否则去 `/`。
- **新手引导**：步骤注册在 `src/onboarding/steps.ts`，服务端只记录完成的步骤 id。加一步就是写它的组件、追加到 `onboardingSteps`（同时加进 e2e 的 `fixtures/auth.ts`）；已经完成前面步骤的用户下次访问只看到新的一步。现在两步：资料，工作区（已有工作区的账户，比如经邀请加入的，直接继续；没有的在这里创建一个；关闭创建时说明怎么加入，再继续）。
- **工作区的外壳**：`/` 落到这台设备最后访问的工作区（localStorage 的 `nwiki.workspace`）、按名称的第一个，或者创建页；`/:slug` 是工作区的外壳，左栏切换工作区，不是成员的 slug 显示 404。新的顶层页面要把它的路径段加进保留名单 `server/internal/modules/workspace/domain/reserved_slugs.txt` 的 `[app]`，vitest 核对路由与名单一致。
- **工作区的页面**：`/create-workspace` 建工作区，slug 随名称生成，停止输入之后问服务端是否可用；关闭创建时只说明怎么加入一个工作区。`/:slug/settings/general` 是工作区的名称与地址：管理员改名，删除要先输入 slug，删除之后回到 `/`；其他成员只能看。`/:slug/settings/members` 列出成员（访客看不到邮箱），管理员改别人的角色、移出、邀请（复制链接自己发出，浏览器不能复制时显示链接框）、撤回邀请；每个成员都能离开，离开之后回到 `/`。
- **邀请页** `/invitations/:id`：在守卫之外，令牌只在地址的片段里，不进 `next`，请求都把它放在请求体里。未登录时先看预览，在页内登录，或带着邀请注册（注册关闭时只有受邀的邮箱能注册）；会话变了页面仍停在原地址，登录之后接受，进入工作区（没完成引导的先走引导）。登录的邮箱不是受邀的，页面说明原因，可以在页内退出换账户。
- **请求的错误**：problem 码与字段码到文案的映射在 `src/app/problem-messages.ts`；vitest 读 `api/dist/openapi.yaml`，契约中任何一个操作列出的码没有文案时失败（只有页面不显示其错误的续期与退出除外）。
- **表单**：`src/app/form.ts` 的 `useForm` 是所有表单的发送：本地检查不通过就不发；服务端的字段错误在字段下方（个别 problem 码也可以指定字段，例如当前密码不对），其余在表单上方；发送中按钮禁用；失败之后焦点移到第一个有错误的字段。
- **设置**（`src/pages/settings/`）：`/settings/profile`（显示名；主题与语言是这个浏览器的偏好，与顶栏的菜单是同一份，只存在设备上）、`/settings/security`（改密码、停用账户）、`/settings/tokens`（个人访问令牌）。新令牌只在创建对话框中显示一次，对话框关闭即卸载，列表从不持有令牌本身；停用成功之后本浏览器忘掉会话（服务端已经结束了它），所有标签页回到登录页。顶栏的用户菜单里显示服务器的版本。

## 端到端测试

`e2e/` 用 Playwright（Chromium）驱动 `make build` 构建的 `bin/nervewiki`，数据库由 testcontainers 启动（需要 Docker）。一次运行启动一个 PostgreSQL、迁移一个模板库；每个 worker 复制出自己的库，运行自己的 `nervewiki serve`。

```bash
pnpm --filter @nervewiki/e2e exec playwright install chromium   # 第一次运行前安装浏览器
make e2e                                                        # make build，然后运行 e2e/stories 下的全部故事
cd e2e && pnpm exec playwright show-report                      # 查看上一次运行的报告
```

- 故事在 `e2e/stories/<分组>/`，从 `e2e/fixtures/test.ts` 取 `test` 与 `expect`：`db`（本 worker 的库）、`nervewiki`（本 worker 的服务）、`api`（类型化的客户端）、`newDatabase` 与 `nervewikiWith`（另起一个库、一个服务）、`pageWatch`（页面发出的接口请求与失败）、`signedInPage`（用接口得到的令牌让页面处于登录状态：第一次加载前写入一次记录，之后页面自己续期）。
- 每个测试的 `page` 从第一次导航之前就被监视；测试通过时，fixture 还核对页面是安静的：没有未捕获的异常、CSP 违规，控制台没有错误与警告，除了故事用 `pageWatch.expectConsole` 按顺序声明的（例如刻意引起的 4xx 在 Chromium 控制台的报告，`failedToLoad(status)`）。故事不用自己调用。
- 失败的测试在 `e2e/playwright-report/` 中带着 trace 与截图；`e2e/test-results/` 中有每个 worker 的服务日志和失败时导出的数据库。持续集成在失败时把两者作为 artifact 上传。
- `make e2e` 把 `VERSION` 交给故事核对注入的版本号；持续集成用 `0.0.0-ci.<运行号>`，与默认值不同。

## 部署

镜像由 `deploy/Dockerfile` 构建：前端与服务端都在其中，运行时是 distroless 镜像，只有 `/nervewiki` 一个程序，以非 root 用户（uid 65532）运行，监听 8080。

```bash
make image VERSION=0.1.0         # 构建 nervewiki:0.1.0
make image-smoke VERSION=0.1.0   # 在镜像上跑 S1、S3：迁移、探针、前端、实例与提交信息、注册关闭、管理员建账户、非 root、优雅停机（另需 curl、jq、openssl）
```

镜像的提交信息取自构建上下文中的 `.git`，所以要在普通的克隆中构建：`git worktree` 的 `.git` 是指向别处的文件，`make image` 会直接报错。`.dockerignore` 排除的正好是 `.gitignore` 忽略的，改一个时同步另一个：否则镜像里的二进制报告的 `modified` 与工作区不符，`make image-smoke` 失败。

- 镜像默认 `NWIKI_ENV=prod`。配置用环境变量提供（也可以挂载一个目录并设置 `NWIKI_CONFIG_DIR`），至少要有数据库地址 `NWIKI_DATABASE__URL` 与签名私钥 `NWIKI_AUTH__JWT__PRIVATE_KEY_FILE`（见上文"账户与认证"；私钥文件要能被 uid 65532 读取）；其余配置项见 `server/configs/config.yaml`，合并规则见上文"配置"。
- prod 配置不自动迁移。每次升级先执行迁移，再启动服务（每个命令都校验整份配置，所以迁移也带上同样的变量）：

  ```bash
  key=(-v "$PWD/jwt.pem:/run/secrets/jwt.pem:ro" -e NWIKI_AUTH__JWT__PRIVATE_KEY_FILE=/run/secrets/jwt.pem)
  docker run --rm -e NWIKI_DATABASE__URL=… "${key[@]}" nervewiki:0.1.0 migrate up
  docker run -d -p 8080:8080 -e NWIKI_DATABASE__URL=… "${key[@]}" nervewiki:0.1.0
  ```

- **迁移与服务分用两个数据库角色时**（表的所有者执行迁移，服务用另一个角色登录，`database.auto_migrate` 关闭），服务需要的权限全部写在 [`deploy/runtime-grants.sql`](deploy/runtime-grants.sql)，授予组角色 `nervewiki_runtime`：业务表逐表的读写（不给 `TRUNCATE`、`REFERENCES`、`TRIGGER` 与 DDL），River 的表与序列，`river_job` 的 `MAINTAIN`（River 每天用 `REINDEX INDEX CONCURRENTLY` 重建它的索引，需要 PostgreSQL 17 起），`goose_db_version` 的读（`/readyz` 靠它判断迁移是否执行完）。函数与类型不在文件里，靠 PostgreSQL 默认给 PUBLIC 的权限。
  - 组角色建一次，服务登录的角色加入它：`CREATE ROLE nervewiki_runtime NOLOGIN;`、`CREATE ROLE nervewiki_app LOGIN PASSWORD '…' IN ROLE nervewiki_runtime;`。
  - **每次 `migrate up` 之后、启动服务之前**，以表的所有者执行一次这个文件，例如 `psql -v ON_ERROR_STOP=1 -f deploy/runtime-grants.sql`：新的迁移可能加了表，文件逐个列出；重复执行没有影响。
  - 少了业务表的权限时接口答 500；少了 `goose_db_version` 的读时 `/readyz` 是 503，后台任务一直等着不启动。少了 `river_queue` 的权限时 River 启动失败，服务随之退出（`start the jobs: … permission denied`）；少了 River 其他表的，`/readyz` 仍是 200，会话清理、软删除清理与索引重建因 `permission denied` 失败，只记在日志里（`river_notification` 目前 River 不写，少了它看不出来）。`server/internal/bootstrap/runtime_role_test.go` 以恰好这些权限的角色运行服务与管理命令，`public` 里任何表、视图、序列或函数的权限与文件不符时失败。
  - 索引属于表的所有者：停机打断 River 的索引重建时留下的 `*_ccnew` 索引，服务的角色删不掉，River 此后每天记 WARN `Found reindex artifact`，由表的所有者执行 `DROP INDEX CONCURRENTLY` 删除。

- 在反向代理之后运行时设置 `NWIKI_SERVER__TRUSTED_PROXIES`，否则每个客户端都被当成代理。

- 数据库必须以 builtin provider 的 `C.UTF-8` 初始化，否则服务拒绝启动，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。
- 探针：存活用 `GET /healthz`（不访问任何依赖），就绪用 `GET /readyz`（数据库可用、迁移已执行完）。镜像里没有 shell 与 curl，所以没有写 `HEALTHCHECK`，由编排系统探测。
- 后台任务（River：每小时一次的过期会话清理 `auth.session_cleanup_interval`，每小时一次的软删除清理 `jobs.purge_interval`）随 `serve` 运行，表在同一条迁移链上。关闭自动迁移时，服务在迁移执行完之前不启动后台任务，迁移之后自动启动，不必重启。River 从连接池里借走一个连接专门监听通知，数据库要为每个实例多留一个连接（`database.max_conns` + 1）。
- 停止时发 SIGTERM：服务停止接收新连接，等正在处理的请求结束（最多 `server.shutdown_timeout`，默认 20 秒），再等正在执行的后台任务（最多 `jobs.shutdown_timeout`，默认 10 秒，之后取消它们，再宽限 1 秒），最后关闭连接池（最多 5 秒）后退出。停机的宽限期要比这些之和长：`docker stop` 默认只等 10 秒，用 `docker stop -t 40`。启动之后不久就停止时（重启循环、端到端测试），River 通常记一条 ERROR `maintenance.PeriodicJobEnqueuer: Error starting transaction`（`context canceled`，它启动时的定时任务入队被停机打断），退出码仍是 0；运行了一段时间的服务停止时一般没有。

## Markdown 样例集

`tools/md-fixtures/` 定义了 Markdown 的提取与改写规则，是服务端实现的验收标准。样例的输入逐字节有意义（CRLF、BOM、行尾空白），`.gitattributes` 与 `.editorconfig` 已经禁止工具改动它们。修改规则前先读它的 [README](tools/md-fixtures/README.md)。
