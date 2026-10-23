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
  - `make test` 依次执行 `make test-go` 和 `make test-web`。前者运行 Go 测试，开启竞态检测（需要 cgo：macOS 装有 Xcode 命令行工具即可）。集成测试用 testcontainers 启动与开发库相同的 PostgreSQL 镜像，需要 Docker；只跑单元测试用 `make test-go-short`（即 `cd server && go test -short ./...`，`make check` 与持续集成的 `short` 一份跑它）。后者运行前端各包的 vitest。
  - 端到端测试另跑 `make e2e`，见下文"端到端测试"。
- 格式有问题时执行 `make fmt`，它修正 Go 与其余文件的格式。`docs/` 不参与格式化。
- 开发数据库以 builtin provider 的 `C.UTF-8` 初始化（`LC_CTYPE` 同为 `C.UTF-8`），与生产环境的要求相同，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。

## 运行后端

```bash
make dev-db   # 先启动开发数据库
make run      # 以 dev 配置启动 nervewiki serve，监听 127.0.0.1:8080；Ctrl-C 优雅停止
```

`make build` 构建前端并把它内嵌进 `bin/nervewiki`（版本号取 `VERSION`，默认 `0.1.0-dev`）。`make run` 启动的服务提供上一次 `make build` 复制进去的前端，从未构建时页面路径答 404 并提示；开发前端用 `make web-dev` 或 `make dev`，见下文"前端"。

`serve` 启动时要连上数据库（最多等 10 秒，连不上就退出），按配置执行迁移（dev、test 默认执行，prod 默认不执行），然后自检数据库的编码与 locale，不满足就拒绝启动并给出建库命令。它还打开附件目录（`storage.dir`，`make run` 时是 `server/_data/`，不进仓库），不可写就拒绝启动，见下文"部署"的附件目录；其他命令不碰它。`GET /healthz` 表示进程存活；`GET /readyz` 在数据库可用、迁移已是最新时返回 200，否则 503。`GET /api/v0/instance` 返回产品名、版本、提交与接口版本，以及是否开放注册（`signup_enabled`）、是否开放创建工作区（`workspace_creation_enabled`），单个附件、导入包的上限（`asset_max_bytes`、`import_max_bytes`）与导出保留多久（`export_ttl_seconds`）、回收站里的一项可以恢复多久（`trash_retention_seconds`，保留期减 1 小时）；`/api/` 下没有的路径返回 404 problem+json。

其他命令在 `server/` 下用 `go run ./cmd/nervewiki <命令>` 执行：

| 命令             | 作用                                                           |
| ---------------- | -------------------------------------------------------------- |
| `serve`          | 运行 HTTP 服务，直到收到 SIGINT 或 SIGTERM；第二次信号立即退出 |
| `migrate up`     | 执行全部待执行的迁移，然后自检数据库                           |
| `migrate down`   | 回滚最近一条迁移                                               |
| `migrate status` | 列出迁移及其状态                                               |
| `users …`        | 服务器管理员的账户命令，见下文"账户与认证"                     |
| `workspaces …`   | 服务器管理员的工作区命令，见下文"账户与认证"                   |
| `reindex …`      | 从页面重建链接索引，让搜索的行重新入队，见下文"链接索引"       |
| `version`        | 打印版本号与构建信息                                           |

### 配置

配置按层合并，后面的覆盖前面的：

1. 内置的 `server/configs/config.yaml`（列出全部配置项及默认值）；
2. 内置的 `server/configs/config.<env>.yaml`；
3. `$NWIKI_CONFIG_DIR` 下的 `config.yaml`、`config.<env>.yaml`（设置了且文件存在时）；
4. 个人覆盖文件 `configs/config.local.yaml`，相对于工作目录，也就是 `server/` 下启动时（`make run` 就是）的 `server/configs/config.local.yaml`；只在 dev 生效，不进仓库；
5. 环境变量 `NWIKI_<节>__<键>`，例如 `database.url` 对应 `NWIKI_DATABASE__URL`。

`NWIKI_ENV` 选择环境（`dev`、`test`、`prod`，默认 `dev`）。未知的键、空值、越界的数字、不带单位的时长都会报错，所有无效的键一次列出；`server.read_timeout` 加 `server.request_timeout` 必须短于 `server.write_timeout`（期限到了之后还要写出错误响应），`page.parse_max_wait` 与 `auth.password.max_wait` 必须短于 `server.request_timeout`，且应明显短于它（等不到解析额度或哈希名额时答 503，而不是先到期限；认证与权限判定也要时间）。日志里的数据库地址整体脱敏。

### 账户与认证

除了 `POST /api/v0/auth/{register,login,refresh,logout}`、`GET /api/v0/instance` 与预览邀请的 `POST /api/v0/workspace-invitations/{workspace_invitation_id}/preview`，每个接口都要求 `Authorization: Bearer <访问令牌或个人访问令牌>`，否则答 401。

- **注册**：`auth.signup_enabled`，dev、test 开放，prod 关闭（`GET /api/v0/instance` 的 `signup_enabled` 告诉客户端）。关闭时，带着工作区邀请（请求体的 `invitation: {id, token}`）的注册仍然可以，只要邀请还待接受、注册邮箱就是被邀请的那个；注册不替用户接受邀请。密码 8–128 个字符，不能是常见密码，也不能由邮箱 @ 之前的部分构成；常见密码名单由 `node tools/password-blocklist/build.mjs` 生成（取 SecLists 固定提交中的 NCSC 名单并核对校验和）。
- **会话**：注册与登录各开一个会话，返回访问令牌（15 分钟，`auth.access_token_ttl`）与刷新令牌。访问令牌到期后用刷新令牌换下一对（`/auth/refresh`），旧的刷新令牌随之作废；作废的刷新令牌再被使用，整个会话被撤销：这可能是它被别人拿到了，也可能是续期的答复在返回途中丢失（服务端已经换了令牌，客户端还拿着旧的），后者让用户重新登录，是严格轮换的代价（RFC 9700 4.14.2）。会话从登录起 30 天（`auth.session_ttl`）结束，续期不延长。`/auth/logout` 结束当前会话。续期与退出在 `auth.refresh_deadline`（4 秒）内完成，它加上 `database.commit_timeout` 必须小于前端放弃续期的 8 秒，且不超过 `server.request_timeout`；超时答 500，事务回滚，令牌不变，可以重试。
- **个人访问令牌（PAT）**：给脚本与集成用。`POST /api/v0/me/api-tokens` 创建，要求当前密码（`current_password`），可选期限 `expires_at`；响应里的 `token`（`nwk_pat_` 开头）只出现这一次，服务端只存它的 SHA-256。`GET /api/v0/me/api-tokens` 列出未撤销的令牌（不含令牌本身，含 `last_used_at`，每分钟至多更新一次），`DELETE /api/v0/api-tokens/{token_id}` 撤销，立即失效。PAT 与访问令牌一样用在 `Authorization: Bearer`，能做账户能做的一切，包括再创建 PAT。
- **账户**：`PATCH /api/v0/me` 改显示名；`POST /api/v0/me/onboarding-steps` 记录完成的引导步骤（步骤由前端定义）；`POST /api/v0/me/change-password` 要求当前密码，改后其他会话全部结束，当前会话保留（用 PAT 调用时全部结束），PAT 照常可用；`POST /api/v0/me/deactivate` 停用账户，所有会话结束，PAT 不能再用，登录答 403 `identity.account_deactivated`，只有管理员能重新启用；工作区里的规则见"工作区"一节的"停用账户"。
- **限流**：`ratelimit` 节的令牌桶，超出时答 429 `rate_limited` 与 `Retry-After`。公开操作按客户端 IP（`anonymous`），其余按凭证（会话或 PAT，`authenticated`）；校验当前密码的操作（改密码、创建 PAT）另按账户（`password_user`）；带令牌的请求在认证之前先过失败闸门（`auth_failure`，按客户端 IP：只有认证失败的令牌消耗名额，过期的访问令牌不算）；登录另按 IP 与"IP 加邮箱"（`login_ip`、`login_ip_email`），注册按 IP（`register_ip`）。IPv6 客户端按 `/64` 前缀计数（`ratelimit.ipv6_prefix_len`）。桶在进程内存中：多实例部署时每个实例各算各的。被拒绝的请求在访问日志中是 429；哪个桶拒绝的，平台的桶记在 debug 级，登录、注册与 `password_user` 的桶记在 info 级（`password_user` 另记账户 `user_id`）。
- **会话清理**：过期的会话由后台任务删除，服务启动时一次，之后每 `auth.session_cleanup_interval`（默认 1 小时）一次；多个实例时只有一个执行。
- **管理员命令**：服务器管理员在能连上数据库的机器上执行，与 `migrate` 一样加载配置（带上服务的那些变量），只连数据库，不启动 HTTP 与后台任务，服务不必停。账户用 `--email` 指定：

  | 命令                                                  | 作用                                                                                           |
  | ----------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
  | `users create --email <地址>`                         | 建账户，不看注册开关，不建会话                                                                 |
  | `users reset-password --email <地址>`                 | 设新密码，撤销账户的全部会话与 PAT                                                             |
  | `users set-email --email <地址> --new-email <新地址>` | 改邮箱，撤销全部会话；PAT 照常可用，账户可能被盗时另执行 `reset-password`                      |
  | `users deactivate --email <地址>`                     | 停用，与自助停用相同：会话全部结束，PAT 在重新启用之前不能用，工作区与笔记本的成员关系全部结束 |
  | `users activate --email <地址>`                       | 重新启用：未撤销、未过期的 PAT 恢复可用，会话不恢复；账户可能被盗时另执行 `reset-password`     |

  `create` 与 `reset-password` 的密码从标准输入读，不接受参数与环境变量（会从 `ps` 泄露）：标准输入是终端时不回显地提示两次，两次要相同；否则读一行，只去掉行尾的换行，首尾空格是密码的一部分，例如 `printf '%s\n' "$PASSWORD" | nervewiki users create --email alice@corp.com`。结果一行写到标准输出（如 `password reset for alice@corp.com: revoked 2 sessions, 1 API token`），日志写到标准错误；失败时打印 `nervewiki: <原因>`，退出码 1。输出与日志里没有密码，日志只记账户的 `user_id`，不记邮箱。停用、启用在状态不变时什么也不做（`… is already deactivated`、`… is already active`）。

  工作区的命令（M2/P4），日志带 `by=cli`，同样不记邮箱：

  | 命令                                                             | 作用                                                                                                                                                                                                                                                                          |
  | ---------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
  | `workspaces create --slug <slug> --name <名称> --admin <地址>`   | 建工作区，那个账户是它的管理员；不看 `workspace.creation_enabled`，关闭创建时工作区就这样建                                                                                                                                                                                   |
  | `workspaces reactivate-member --workspace <slug> --email <地址>` | 恢复已结束的成员关系，角色沿用，加入的时刻不变，他名下的无主笔记本归还给他；输出它结束的时刻与归还的笔记本数（`… the membership had ended at …; ownerless notebooks returned: N`）。账户要先 `users activate`；工作区没有管理员时，先恢复一位管理员（规则见下面的"停用账户"） |

  账户不存在、已停用，slug 已被占用或不合规则，工作区不存在，或者他从来不是这个工作区的成员时，退出码 1，数据库不变。恢复不限于停用结束的成员关系：被移出、离开的也可以恢复，看输出的结束时刻确认恢复的是哪一次。成员关系已是有效的，什么也不做（`… is already a member of …`）。

- **签名私钥**：`auth.jwt.private_key_file`，PKCS#8 PEM 的 Ed25519 私钥，用 `openssl genpkey -algorithm ed25519 -out jwt.pem` 生成。prod 必须提供，缺了拒绝启动；dev、test 不提供时每次启动生成临时密钥，重启后已签发的令牌与邀请链接全部失效。刷新令牌与邀请令牌的 MAC 密钥都由它派生（HKDF，各用各的 info）：换私钥之后，待接受的邀请链接全部失效，要重新邀请。附件与导出的签名地址的密钥同样由它派生：换私钥之后，已签出的地址全部失效（打开着的页面重读时拿到新的）；要立刻收回已签出的地址时就这样做（见"附件"的"签名地址"）。日志只记是否设置，不记路径。
- **反向代理**：`server.trusted_proxies` 列出代理的 CIDR（环境变量用逗号分隔，例如 `NWIKI_SERVER__TRUSTED_PROXIES=10.0.0.0/8`）。只有来自它们的 `X-Forwarded-For` 被采信，代理写入的必须是不带端口的 IP；会话记录的就是这样认出的客户端 IP。配置不对时服务各告警一次。
- 非 prod 的服务监听在回环地址之外时，启动时告警：这多半是忘了设 `NWIKI_ENV=prod` 的部署，注册开放、签名密钥是临时的。

### 工作区

- **创建**：`workspace.creation_enabled`，默认开启：每个账户都能创建工作区，创建者是它的管理员（`GET /api/v0/instance` 的 `workspace_creation_enabled` 告诉客户端）。关闭后创建答 403 `workspace.creation_disabled`。
- **slug**：工作区的地址段，1–48 个 a–z、0–9、`_`、`-`，创建后不能改；站点的顶层路径与留作以后用的名字不能用，名单在 `server/internal/modules/workspace/domain/reserved_slugs.txt`。
- **成员**：角色是 admin、member、guest。管理员改别人的角色、移出成员，不能改或移出自己；成员可以离开，唯一的管理员不能（`workspace.sole_admin`），先让别人成为管理员，或者删除工作区；还有别的成员的笔记本的唯一管理员也不能离开（`notebook.sole_admin`，见"笔记本"）。移出不因笔记本被拒：他独自管理的笔记本成为无主。成员列表对访客隐藏邮箱。
- **停用账户**（自助停用与 `users deactivate` 相同）：他是某个还有别的有效成员的工作区唯一的管理员时，停用被拒（409 `workspace.sole_admin`，原因里列出这些工作区的 slug），先让那里的另一位成员成为管理员；只有他一人的工作区不挡停用。他是还有别的成员的笔记本唯一的管理员时同样被拒（409 `notebook.sole_admin`，见"笔记本"）：先在那些笔记本里让另一位成员成为管理员，或者删除它们；他自己做不了时（如服务器管理员用 `users deactivate`），由工作区的管理员把他移出那个工作区，移出不被拒，那些笔记本成为无主。停用结束他全部的成员关系（工作区与笔记本，他独自管理的笔记本成为无主），并删除这些工作区里发给他邮箱的待接受邀请。恢复：`users activate`，再对每个工作区 `workspaces reactivate-member`，或者由工作区的管理员重新邀请他。唯一的管理员独自停用之后，工作区没有管理员：接受成员、访客邀请答 409 `workspace.no_admin`，`reactivate-member` 恢复成员、访客也被拒（`The workspace has no admin`）；先恢复那位管理员，或者有人接受一份管理员邀请，别人才能加入。注册关闭时凭这样的邀请照样能注册，邀请留着，等工作区重新有管理员之后再接受。
- **软删除与清理**：删除工作区是软删除，连同它的成员、邀请、笔记本、笔记本的成员、页面与审计记录；撤回、接受的邀请也是软删除。超过 `jobs.purge_retention`（默认 1440 小时，即 60 天，至少 1 天）的，由后台任务物理删除，服务启动时一次，之后每 `jobs.purge_interval`（默认 1 小时）一次；多个实例时只有一个执行。删除的页与附件在回收站里可以恢复的期限是保留期减 1 小时（`GET /api/v0/instance` 的 `trash_retention_seconds`）；其余的，保留期内运维可以从数据库恢复。
- **邀请**：管理员按邮箱邀请（`POST /api/v0/workspaces/{slug}/invitations`），把邀请的 id 与令牌（`nwk_inv_` 开头）发给对方；一个工作区里一个邮箱至多一份待接受的邀请，有效成员的邮箱不能邀请。令牌是 id 的 MAC，不存库，管理员随时可以在邀请列表里再看到它。任何拿到链接的人都能预览（工作区的名称与 slug、角色，不含邮箱）；接受要求用被邀请的邮箱登录，接受之后成为成员（已结束的成员关系恢复，保留第一次加入的时刻，他名下的无主笔记本归还给他；已是成员的角色不变）。预览与接受都把令牌放在请求体里，不放在 URL 中；链接由页面拼出（M2/P5、P6），令牌放在 URL 片段（`#`）里，浏览器不把片段发给服务器。撤回邀请（`DELETE /api/v0/workspace-invitations/{workspace_invitation_id}`）、成员关系结束（对他邮箱的待接受邀请一并删除）、删除工作区之后，链接答 404 `workspace.invitation_not_found`。

### 笔记本

- **建立与开放程度**：工作区的管理员与成员建笔记本（`POST /api/v0/workspaces/{slug}/notebooks`），建的人是它的管理员；访客建答 403 `forbidden`。`workspace_access` 是它对工作区的开放程度：`none`（私密，只有它的成员看得到，工作区的管理员也看不到）、`viewer`、`editor`（工作区的管理员与成员默认是阅读者、编辑者；访客没有默认角色，只能被加为成员）。
- **有效角色**：笔记本的角色是 admin、editor、reader；一个人的有效角色是显式角色与默认角色中较高的一个，`Notebook` 的 `role` 是调用者的有效角色。看不到的笔记本按 id 答 404 `notebook.not_found`，与不存在相同。
- **名称**：与页面标题同一规则（`shared.CheckTitle`）：去掉首尾空白、NFC 规范化之后 1–255 字节，不含 `/ \ : * ? " < > | # ^ [ ]` 与控制字符，不以 `.` 开头或结尾，不是 Windows 保留名；不要求唯一。不合规则是 `name` 的字段错误（422）。
- **成员**：笔记本的管理员从工作区的有效成员中添加成员（访客也可以）、改角色、移出，不能改或移出自己（409 `notebook.own_membership`）；添加不是工作区有效成员的人是 `user_id: not_allowed`，已是成员的是 `user_id: duplicate`，成员关系已结束的（离开、被移出，或随工作区的成员关系结束）恢复原来那一行，取这次给的角色，保留第一次加入的时刻。成员可以离开，唯一的管理员不能，哪怕只有他一人（409 `notebook.sole_admin`）：先让别人成为管理员，或者删除笔记本。改名、改开放程度（`PATCH /api/v0/notebooks/{notebook_id}`）与删除（`DELETE` 同一地址）也只有笔记本的管理员能做，编辑者、阅读者答 403 `forbidden`；删除是软删除，连同它的成员行与页面，页面的编辑会话随之删除。
- **离开工作区、停用与移出**：离开工作区或停用账户（自助与 `users deactivate`）时，他是某个还有别的有效显式成员的笔记本唯一的管理员，就被拒（409 `notebook.sole_admin`）；原因只给这些笔记本所在工作区的 slug 与数量（`… (1 in acme)`），不给名称。只靠 `workspace_access` 使用它的人不算别的成员。工作区自己的规则（`workspace.sole_admin`）先判断。成员关系结束时，他的笔记本成员关系一并结束；他是唯一管理员的笔记本成为**无主**：剩下的成员与按开放程度看得到它的人照常使用，只是没有人能管理它的设置与成员。工作区管理员移出成员不被拒，他独自管理的笔记本同样成为无主。
- **无主笔记本**：只有工作区的管理员看得到、处理得了：`GET /api/v0/workspaces/{slug}/ownerless-notebooks` 列出（原所有者、成为无主的时刻、最后活动与大小：笔记本自己最后一次修改与它的页面最后一次写入（新建、改名、移动、删除、写正文）中较晚的时刻，没删除的页面正文与附件的总字节数），`POST /api/v0/ownerless-notebooks/{notebook_id}/take-over` 接管（成为它的管理员，开放程度不变），`DELETE /api/v0/ownerless-notebooks/{notebook_id}` 删除。工作区的成员与访客读清单答 403；按 id 的两个操作对管理员之外的任何人答 404 `notebook.not_found`，与不存在、不是无主的相同。原所有者经接受邀请或 `workspaces reactivate-member` 回到工作区时，他名下还没被接管或删除的无主笔记本归还给他，以访客身份回来也一样。页面上是工作区设置的"无主笔记本"一页，管理员的首页另有提醒。
- **审计记录**：接管、删除、归还三种，记执行者、时刻、笔记本名称的快照与原所有者：`GET /api/v0/workspaces/{slug}/notebook-audit-events?limit=&cursor=`，只给工作区的管理员，新的在前。`limit` 1–100、默认 50；下一页用答复的 `next_cursor`，最后一页它是 `null`；读不出的游标（不是服务器写出的拼法；游标不签名，不防改写）答 400 `bad_request`，先于其余判断。审计记录比笔记本活得久，随工作区删除与清理。

### 页面

- **树**：一个笔记本的页面是一棵树，`GET /api/v0/notebooks/{notebook_id}/nodes` 一次列出整棵（父页在子页之前）。新建（`POST /api/v0/notebooks/{notebook_id}/pages`，可以带正文）、改名（`PATCH /api/v0/nodes/{node_id}`）、移动（`POST /api/v0/nodes/{node_id}/move`：换父页或在兄弟之间排序，连同它下面的页）、删除（`DELETE /api/v0/nodes/{node_id}`：连同它下面的页与附件一起移到回收站，成为可以恢复的一项，见下文"历史与回收站"；保留期过后由清理任务物理删除，见"工作区"的"软删除与清理"）；`GET /api/v0/pages/{page_id}` 读一页与它的祖先。位置用 `after_id`：排在哪个兄弟之后，`null` 排最前，不给排最后。标题与笔记本的名称同一规则；同一父页下（根下也一样）按标题键唯一（NFC、Unicode 大小写折叠、再 NFC），重名答 409 `page.title_taken`；树至多十层（`page.too_deep`），不能移到自己下面（`page.cycle`），也不能移到别的笔记本。笔记本的编辑者与管理员能写，阅读者只读（403 `forbidden`）；看不到的页答 404 `page.not_found`，与不存在相同。
- **正文**：`GET /api/v0/pages/{page_id}/content` 读出正文、它的版本 `revision` 与 `content_hash`；`PUT` 同一地址写入 `{content, base_revision, edit_session_id?}`，逐字节保存（换行写法、BOM、空白都原样），至多 5 MiB（5,242,880 字节）、不含 NUL（`content` 的字段错误，422）。`base_revision` 不是当前版本答 409 `page.revision_mismatch`：期间有人写过，重新读出再决定；与当前正文相同的写什么也不改。两个正文路由的请求体上限另算（正文加上 JSON 的转义），期限另加 `server.read_timeout`：5 MiB 的正文在 30 秒内传完约需 1.4 Mbit/s 的上行，更慢的链路调大 `read_timeout` 与 `write_timeout`；请求体没有及时传完答 400 `bad_request`。
- **编辑会话与编辑锁**：编辑器开启会话（`POST /api/v0/pages/{page_id}/edit-sessions`），租约 120 秒，每 20 秒心跳一次续上（`POST /api/v0/edit-sessions/{edit_session_id}/heartbeat`），退出时结束（`DELETE /api/v0/edit-sessions/{edit_session_id}`）。带着同一个会话的写是一个变更集。
  - **锁**：一页活着的会话就是它的编辑锁，同一时刻至多一个。有人持锁时，再开启答 409 `page.locked`，problem 的 `lock` 成员给出页与持锁人（`user_id`、`display_name`），持锁的是自己在别处的会话也一样；不带这个会话的写正文（令牌、别的标签页）也答 `page.locked`；删除一页或它的上级页时，子树里有**别的账户**持锁就答 `page.locked`（按层取第一个），自己的会话随删除结束；经 MCP 的删除被子树里任何活着的会话拒绝，自己的也算，列出被持着的页（至多 20 个，另给总数），会话照旧活着（见下文"MCP"）。新建不受锁限制；改名、移动要改写别的页里的链接时，改写所需的页有人持锁（包括本人在编辑）就整个答 409 `linking.pages_locked`，什么都不改（见下文"链接索引"）。
  - **接管**：开启时带 `{"take_over": true}`，先结束自己在这一页的会话（别处的标签页、令牌都算），再开新会话；别人的锁接管不了。被接管的会话再心跳或保存答 409 `page.edit_session_taken_over`。
  - **强制解锁**：笔记本的管理员 `DELETE /api/v0/pages/{page_id}/edit-lock` 结束持锁的会话（没人持锁也答 204）；那个会话再心跳或保存答 409 `page.edit_session_unlocked`，`ended_by` 成员给出解除者。
  - **读锁**：能读这一页的人 `GET /api/v0/pages/{page_id}/edit-lock`，答复 `{holder, expires_in}`：持锁人与租约剩余的秒数（向上取整），没人持锁时两者都是 `null`。
  - 会话过期，或不是写的人在这一页、用同一种客户端（网页，或任一令牌）开的，写答 409 `page.edit_session_ended`，心跳与结束答 404 `page.edit_session_not_found`，编辑器重开一个。被接管、被解锁的会话至少保留一个租约，让原来的标签页得知原因（这期间心跳与保存答原因，结束答 204；会话的主人不再能编辑这个笔记本时，心跳照常答 404 或 403），之后与过期的会话一起删除：这一页下一次开启或强制解锁时，或后台任务每 `page.edit_session_cleanup_interval`（默认 10 分钟）一次；删页、删子树、删笔记本时它们的会话一并删除。
- **阅读视图**：`GET /api/v0/pages/{page_id}/view` 给出渲染好的 HTML 与它所依据的 `revision`：CommonMark 加 GFM（表格、任务项、删除线、自动链接）与脚注，frontmatter 的属性在最前面显示成表格；段落里的单个换行显示为换行（`<br>`），与 Obsidian 关闭"严格换行"（它的默认）时相同；正文里的 HTML 只留排版用的标签与属性，地址只留本站、http(s) 与 mailto，图片不加载、显示为链接。Obsidian 的写法（规则与样例见下文"Markdown 样例集"）：标签是 `<a class="nw-tag" data-nw-tag>`；callout 是带 `data-callout` 的 `<div>`，可折叠的是 `<details>`；`==高亮==` 是 `<mark>`；`%%注释%%` 不输出；公式 `$…$`、`$$…$$` 输出原文，包在 `nw-math`（块的另加 `nw-math-block`）里，由网页用 KaTeX 排版；`mermaid` 代码块由网页画图；嵌入（`![[…]]`）指向页面的照链接显示，指向附件的见下文"附件"；`[文字](#标题)` 指向本页的标题 id。表格、块公式与属性表包一层 `nw-scroll`，宽的时候自己横向滚动。任务项的复选框都是 `disabled`，`data-task` 是方括号里那个字符在正文里的字节位置。指向笔记本里的页的链接（wikilink、嵌入，以及 Markdown 链接与图片里不是外部地址的）不带地址，由网页给出：解析到的带 `data-nw-node`（那一页的 id）与 `data-nw-anchor`（锚点按标题 id 的规则算出，指向同名标题里的第一个），解析不到的带 `class="nw-unresolved"` 与 `data-nw-target`。链接的状态读自链接索引；索引不是这一版本的（`reindex` 之前，或读的期间有写）时当场解析（见下文"链接索引"）。
- **勾选任务项**：`POST /api/v0/pages/{page_id}/toggle-task`，`{base_revision, offset, checked}`：把位置 `offset` 上的那个字节换成 `x` 或空格，其余字节不变，答这一页；权限与写正文相同，有人持锁时同样答 409 `page.locked`。`base_revision` 不是当前版本答 409 `page.revision_mismatch`（位置只在它所依据的版本里有意义，先于 422）；`offset` 上不是任务项答 422（`offset`，`out_of_range`），勾了会让那里不再有任务项（例如 `- [ ]: /u` 勾上之后成了链接引用定义）答 422（`offset`，`not_allowed`）；已经是那个状态的什么也不写，有人持锁时也答 200。
- **解析预算**：服务端同时解析的正文字节数有上限（`page.parse_budget_bytes`，默认 8 MiB，每次至少记 4 KiB），取不到额度的请求最多等 `page.parse_max_wait`（默认 2 秒），然后答 503 `server_busy`（带 `Retry-After`）；写正文、新建带正文的页与阅读视图都经它。最坏的正文解析时约占它字节数 300 倍的内存（默认预算约 2.4 GB），普通的约 40 倍：内存小的机器调小预算（不能小于 5 MiB），并设置 `GOMEMLIMIT`。

### 附件

- 附件是笔记本树里的节点（`kind: "asset"`），在一页下或根下，与页面同一套改名、移动、删除（`/api/v0/nodes/{node_id}`）。它不算树的一层；名称与兄弟页面同一规则、同一唯一性，不能以 `.md` 结尾。`GET /api/v0/notebooks/{notebook_id}/nodes` 列出它们，网页的左栏只显示页面。
- **上传**：`POST /api/v0/notebooks/{notebook_id}/assets`，`multipart/form-data`，部分依次是 `parent_id`、`name`（都可省；名称省了或为空白时取文件名）、`file`；笔记本的编辑者与管理员能传。单个附件至多 `asset.max_bytes`（默认 50 MiB，即 `GET /api/v0/instance` 的 `asset_max_bytes`），超过答 413；存储没有余量答 507 `storage_full`。文件边收边写；读文件之前先核对笔记本、权限、名称、父页与存储的余量，不通过就立即答复、不读文件，答复之后关闭连接（仍在发送文件的客户端可能只看到连接被重置，而收不到这个答复）。类型按扩展名与文件头测定，PNG、JPEG、GIF、WebP 读出宽高。
- **读**：`GET /api/v0/assets/{node_id}` 读一个，`GET /api/v0/notebooks/{notebook_id}/assets`（`parent_id` 可选）按名称分页列出一页下或根下的附件，能读这本笔记本的人都可以。答复的 `content_url` 与 `download_url` 是签名的地址，不带令牌也能打开，1 到 2 小时内有效（`expires_at`，同一小时签出的地址相同）；改动任何参数、过期、附件已删除都答 404。
- **内容**：图片、音频、视频与 PDF 在浏览器里显示（`inline`），其余（包括 HTML）一律以 `application/octet-stream` 下载，`download_url` 总是下载。每个答复都带 `Content-Security-Policy: sandbox; …`：SVG 里的脚本不执行，也不向别的站请求；私有缓存到地址过期，`ETag` 是 SHA-256，支持 `Range` 与条件请求。PDF 在 Chromium 的内置阅读器里照常显示（其他浏览器未实测；显示不了时用 `download_url` 下载）。下载按客户端 IP 限速（`ratelimit.asset_content`），不占匿名请求的桶。
- 删除附件、它的上级页或笔记本时，它的行一起软删除（从回收站恢复时随节点回来）；保留期过后清理任务先删文件、再删行。没有行的文件（例如上传中途失败留下的）一天之后由每天的清扫删掉。笔记本的活动（无主笔记本列表的大小与最后活动）算上附件。
- **在正文里**：链接与嵌入按名称解析到附件（规则见下文"链接索引"）。阅读视图里，嵌入的图片是 `<img>`，音频与视频带播放控件（一个视图至多 20 个，之后写成附件的链接），其余类型与指向附件的链接是附件的链接，网页在新标签页打开并显示大小；嵌入的显示写成 `300` 或 `300x200` 时是尺寸，其余是说明。一个视图至多写 2000 个附件的地址，之后的显示为文字。属性链接指向附件时，右栏给出它的地址。改名、移动附件时，指向它的链接照页面的规则改写。
- **签名地址**：签名只在核对过读权限之后发出，至多两小时有效：失去访问之后，已经拿到的地址到期之前仍能读，要立刻收回就换签名私钥（见"账户与认证"，所有地址一起失效）。地址会进反向代理的访问日志（它是"秘密不进地址"的例外），访问日志要照敏感数据保管。MCP 的 `read`（见下文"MCP"）给的是以 `server.public_url`（实例对外的地址，只到源，默认空）起头的绝对地址，同样签名、同样的期限；它为空时没有下载的地址：图片照样直接放进答复，别的附件只给元数据。

### 链接索引

- **索引**：每一页正文里的链接（wikilink、嵌入、Markdown 链接与图片，以及 frontmatter 里的属性链接）、标签、属性与别名，和每条链接解析到的页，随每次写入在同一个事务里更新：新建、改名、移动、删除（连同子页）、从回收站恢复、写正文、勾选任务项（还原版本与整组撤销由这几种写组成），删除笔记本时一并删除。同一笔记本的索引维护一个接一个进行（索引自己按笔记本的锁），所以同一笔记本的保存在提交处排队。
- **解析**：链接按目标最后一段的标题键找页。以 `.md` 结尾（不分大小写）的，笔记本里有去掉 `.md` 的那个名称的页时就是去掉 `.md` 的，没有时才是标题以 `.md` 结尾的页。`./`、`../` 开头的从出发页所在的文件夹（它的父页）算起；`/` 开头的从根算起；否则先找从根起路径恰好如此的页，再找路径以这几段结尾的页（出发文件夹的子树里的优先，文件夹自己的页也算，再按路径短的，即导出路径的字符数，再按 id，最后这一步时算有歧义）；只有一段的名称最后才找别名。规则与样例在 `tools/md-fixtures/resolve/`，与 Obsidian 1.12.7 核对过；不同的有：别名（Obsidian 不按别名解析），标题键用 Unicode 的大小写折叠（Obsidian 用 `toLowerCase`，`ß` 与 `ss` 它不当作一样），路径一样长时按 id 选（Obsidian 没有稳定的次序），以及三处 Obsidian 按字符串比较路径的地方（这里按整段）。
- **附件**：笔记本里有名称等于目标最后一段的附件时，这条链接只读作附件（按附件的次序找，找不到即解析不到）；名叫 `x.png` 的页面写作 `x.png.md`；没有扩展名的附件没有链接的写法。索引记下解析到附件的链接（`page_links.resolved_asset`）。规则与样例在 `tools/md-fixtures/resolve/` 016–027。
- **别名与标签**照 Obsidian 的读法：第一个名为 `aliases`、`tags` 的键（ASCII 字母不分大小写），字符串是一个（不按逗号拆开），列表取其中的字符串，去掉首尾空白，空的不算；`alias`、`tag` 不读。标签（frontmatter 的去掉开头的一个 `#`）是 Obsidian 的标签面板计入的：去掉结尾的一个 `/`，不含空白、ASCII 标点（`-`、`_`、`/` 除外）与 U+2000–U+206F、U+2E00–U+2E7F 两段标点，不全是 ASCII 数字。
- 正文里经 `%00` 或 YAML 的转义写出的 U+0000 在索引里记作 U+FFFD；目标、别名、标签的标题键长于 1024 字节（任何标题的键都到不了）时，这条链接解析不到，这个别名、标签不记。
- **改名与移动改写链接**：改名、移动一页（连同子页）时，指向它们的链接，以及被移动的页里的相对链接，不再解析到原来的页的，改写成解析到它的写法（名称在笔记本里只有这一页时写名称，否则从根起的完整路径），锚点不动，显示文字与链接文字照 Obsidian 的规则跟着改（[M6/P4 文档](docs/v0.1/M6-links/04-P4-rewrite.md)）；改写是同一个变更集里的正文写，作者是改名、移动的人。改写所需的页有人持锁（包括本人在编辑）时，整个改名、移动答 409 `linking.pages_locked`，problem 的 `locks` 成员列出每一页与持锁人（`page_id`、`user_id`、`display_name`）；改写要的解析预算取不到时立即答 503 `server_busy`。改写之后会超过 5 MiB 的页、读回的链接不对（依次退回更保守的写法之后仍不对）的页不改，写一条日志，改名、移动照常。MCP 的 `move` 带 `update_links: false` 时不改写，也不因改写所需的页有人持锁而拒绝：指向它的链接照索引重新解析，可能断开或指向别处（见下文"MCP"）；网页与 REST 总是改写。
- **读接口**（笔记本里有角色的人都能读，落点除外）：
  - `GET /api/v0/pages/{page_id}/backlinks?limit=&cursor=`：链接到这一页的页，按出发页的 id 分页；每页给出链接数（至多数到 1000）与前 10 条链接所在的行（长于 240 字节的截取，截掉的一端加 `…`）；
  - `GET /api/v0/pages/{page_id}/properties`：frontmatter 是否合法、各属性的值，以及属性链接解析到的页；
  - `GET /api/v0/notebooks/{notebook_id}/tags`：笔记本的标签与带它的页数；`GET /api/v0/notebooks/{notebook_id}/tags/{tag}`：带这个标签或它下面的标签的页（标签整个编码成一段）；
  - `GET /api/v0/notebooks/{notebook_id}/link-targets`：笔记本的每一页与附件（`kind`）、它的 `link`（链接写成什么才只解析到它；没有扩展名的附件为 `null`）与别名，编辑器的补全用它；
  - `GET /api/v0/pages/{page_id}/link-landing?target=`：只给笔记本的管理员与编辑者，为这一页里一条解析不到的链接新建页时，新页该建在哪个父页下、叫什么，或者已有的页，或者为什么不能建（`reason`；目标是附件的写法时是 `target_is_asset`）。
- **`nervewiki reindex [--notebook <id>]`**：从页面重建链接索引，不给 `--notebook` 时逐个重建每个没删除的笔记本，每个一行（页数、链接数、解析不到的数，以及搜索入队的节点数）。它同时让这个笔记本里还没有搜索的行、或行的分词版本不同的节点以批量重新入队，由服务在后台重建（见下文"搜索"）：建好之前搜索找不到它们，结果页写明索引正在建立、结果可能不全。升级到带链接索引的版本（M6）之后、或发布说明要求时（提取规则的版本 `indexed_pages.extractor` 变了；搜索的分词版本变了：升级 Go 或 `golang.org/x/text` 的 Unicode 数据之后）执行一次。
  - 每个笔记本一个事务：重建期间这个笔记本的写入在等待；页数上万的笔记本要十几秒，等不到的保存答 500。所以在没人写的时候执行，最好在 `migrate up` 之后、启动服务之前。
  - 它同时按当前的 Unicode 数据重算标题键：同一父页下有两页会撞键时，这个笔记本不重建、什么也不改，标准错误上列出这些页的标题与 id；改掉其中一个标题之后再执行。别的原因重建失败的笔记本同样原样保留，连同原因列在标准错误上。其余笔记本照常，命令最后以退出码 1 结束。撞键的笔记本的搜索照样入队；搜索的入队失败的笔记本在它那一行写明没有入队，命令接着做别的，最后同样以退出码 1 结束。

### 历史与回收站

- **版本**：一页的每个版本是一个变更集写下的正文：一次编辑会话的保存合为一个版本（它的版本号随每次保存前进，会话活着时标出 `editing`），改名、移动不是版本。`GET /api/v0/pages/{page_id}/versions` 新的在前、按游标分页列出（不带正文，带写下它的变更集：类型、客户端、执行者与时刻）；`GET /api/v0/pages/{page_id}/versions/{revision}` 读一个版本与它逐字节的正文，`…/versions/{revision}/view` 是它的阅读视图（链接按现在的树解析）。`revision` 是这个版本所含的任何一个版本号（会话中途的也算），答复带它自己的版本号；不在任何版本里答 404 `page.version_not_found`。回收站里的页同样读得到，链接按它被删时的位置解析。比较在网页上做，接口给两个版本的原样正文。
- **还原版本**：`POST /api/v0/pages/{page_id}/versions/{revision}/restore`，`{base_revision}`（读到的这一页现在的版本号）：把这个版本的正文写成新的版本，记为一个类型为 `version_restore` 的变更集，答 `{revision, changeset_id, changed}`。正文与现在相同时什么也不写（`changed: false`，不看 `base_revision`）；`base_revision` 不是现在的版本答 409 `page.revision_mismatch`；有编辑会话持着这一页的锁（包括自己在别处的）答 409 `page.locked`；回收站里的页答 404 `page.not_found`，先从回收站恢复。
- **日志**：`GET /api/v0/notebooks/{notebook_id}/changesets` 列出笔记本的变更集，按第一次写入的时刻新的在前、按游标分页：一个编辑会话、一次请求的写、一次导入或后台任务各是一个，类型是 `edit`、`import`、`version_restore`、`trash_restore`、`revert`。MCP 的一次写（见下文"MCP"）是一个 `edit` 的变更集，客户端是 `mcp:<名>`。筛选：`actor`（账户 id）、`client`（`web`、`api`、`cli`、`mcp:<名>`，或 `mcp` 表示全部 MCP 客户端）、`page_id`（碰过这个页或附件的，回收站里的也算；不是这本笔记本的节点、已被清理的答 404 `page.not_found`）。每条带客户端、执行者、说明、它撤销的那一个与撤销过它的（新的在前，至多 100 个）、是否被截掉、节点数与正文数（数到 1,000）与前三项变更。`GET /api/v0/changesets/{changeset_id}` 读一个（另答它有没有改名、移动），`GET /api/v0/changesets/{changeset_id}/changes` 按节点分页列出它对每个节点做了什么（前后的父页与名称、写下的版本号）。
- **整组撤销**：`POST /api/v0/changesets/{changeset_id}/revert`（请求体可以没有，或带 `message`：1–4,096 字节）把这个变更集碰过的每个节点放回它之前的样子，记为一个类型为 `revert` 的新变更集，答 `{changeset_id, changed}`（什么都不用改时不写，`changed: false`）。谁做的变更集都可以撤销，撤销也可以再撤销，从新到旧连续撤销成立。按状态比较，整组成立或什么都不写：它新建、恢复的节点进回收站，删除的整项回到原来的位置与名称（放不回去是冲突，不加序号、不改放根下），改名、移动的改回，写了正文的页写回原来的正文（新的版本）；每个节点都要仍是它留下的样子，否则是冲突，答 409 `page.revert_conflict`，`conflicts` 列出前 100 个（节点、名称与原因 `changed`、`expired`、`parent_gone`、`cycle`、`too_deep`、`children_added`、`title_taken`，最后一种带占着名称的 `holder_id`），`conflict_count` 是总数。被清理截掉过一部分的变更集不能撤销（409 `page.changeset_trimmed`）；要删除的页有别的账户在编辑、要写回的页有人在编辑（包括自己在别处）答 409 `page.locked`。撤销不经链接的改写：改名、移动当时改写的页逐字节写回，之后新写的、指向新名称的链接不跟着改回，会断开或指向别处。batch 一类有几种操作的变更集按依赖分轮撤销：必要时把一个节点暂时改成带 `~` 与它的 id 一段的名称、或暂时移到根下，结束时都回到原来的名称与位置，这些步也算进 1,500 步；先改正文、再删除的页先恢复、再写回原来的正文。一次至多放回 10,000 个节点、1,500 步，恢复的历史（版本与条目）至多 400,000 行，超过一页时至多 100 页、8 MiB 正文、20,000 条链接，超过答 422 `page.revert_too_large`，可以在后台撤销（`POST /api/v0/notebooks/{notebook_id}/reverts`，至多 60,000 个条目，见下文"导入与导出"的"任务"）。
- **回收站**：删除一页（连同它下面的页与附件）或一个附件，一起删除的节点是笔记本回收站里的一项。`GET /api/v0/notebooks/{notebook_id}/trash` 按删除的时刻新的在前、按游标分页列出还能恢复的各项的根：原位置、删除者与时刻、可以恢复到何时（`restorable_until`）、这一项的页数与附件数、恢复之后放到哪里（`restore_to`）；`GET /api/v0/nodes/{node_id}/trash-item` 读一项（另带页的版本号、附件的类型与大小）。`POST /api/v0/nodes/{node_id}/restore` 恢复一项：节点连同正文、版本、附件与链接索引回来，记为一个类型为 `trash_restore` 的变更集；放回原来的父页之下、兄弟之间原来的位置（原来在根下的回到根下），原父页在回收站里（`parent_in_trash`）或放回去会超过十层（`too_deep`）时放到根下的最后；那里已有同名的时加序号（"名称 2"，附件的序号在扩展名之前），答复的 `placed`、`renamed` 写明。不在回收站答 409 `page.not_in_trash`，不是一项的根答 409 `page.not_trash_root`（`root_id` 是那一项的根），过了期限答 409 `page.restore_expired`，读过之后这一项变了（又被恢复、删除或部分被清理）答 409 `page.trash_changed`，重读回收站。一次至多 10,000 个节点、400,000 行历史，超过一页时至多 100 页、8 MiB 正文、20,000 条链接，超过答 422 `page.restore_too_large`，可以在后台恢复（`POST /api/v0/notebooks/{notebook_id}/trash-restores`）。期限是删除的时刻加 `jobs.purge_retention` 减 1 小时（`GET /api/v0/instance` 的 `trash_retention_seconds`，见"工作区"的"软删除与清理"），之后由清理任务物理删除；清理删掉的版本与条目所属的变更集被截掉（`trimmed`）。
- **权限**：笔记本里有角色的人都能读版本、日志与回收站（读者在删除之前就读得到这些页）；还原版本、整组撤销与恢复要编辑者或管理员，撤销别人的变更集不另要管理员，阅读者答 403 `forbidden`。v0.1 没有永久删除：回收站里的一项只由清理任务在期限之后删除，活着的页的版本一直留着。

### 搜索

- `GET /api/v0/workspaces/{slug}/search?q=&notebook_id=&cursor=&limit=`：工作区的任何成员（访客只在被加入的笔记本里）按名称与正文搜索看得到的笔记本里活着的页与附件（附件只按名称），可以限定一本笔记本。名称里有每个词的排在前面，其余的在后，各按最后改动从新到旧；每页 1–50 条（默认 20），正文命中的带摘要与高亮的位置（UTF-16 码元），第一页带约数（`exact`、`about`、`at_least`）。
- 词按空白分开，每个词都要命中：单个汉字只在名称里找；汉字 2 个起、字母与数字 3 个起按子串（`api` 找得到 `rapid`）；字母与数字 1–2 个按整词（`go` 找得到 `Go语言`）；大小写不敏感；繁简、重音、全角与半角不互通。`q` 至多 256 字节、8 个词、64 个连续的 ASCII 字母与数字。一页至多查 500 页的正文：可能少于 `limit` 条、甚至一条也没有而带 `next_cursor`。
- 索引由服务的后台任务建（`page.search_index_interval`，默认每 5 秒）：用户在请求里的写立即搜得到；导入、后台的恢复与撤销、升级、`nervewiki reindex` 写的节点在任务建好之前搜不到，答复的 `building` 为真，结果页写明索引正在建立、结果可能不全。索引里的每个词带它的笔记本的前缀：一次搜索只读它搜的那些笔记本的词，看不到的笔记本里写了什么，既不改变它读什么，也不改变它读多久。

### 导入与导出

- **导出**（`POST /api/v0/notebooks/{notebook_id}/exports`，可带 `root_id` 只导出一页及其子页）：能读这本笔记本的人都能导出。后台任务在一个快照里把页面写成 `.md`、附件写成原来的文件，按 Obsidian 库的布局打成 zip（有子页的页是同名的文件夹；`.nerve/meta.json` 记着次序），报告列出改了名的与找不到文件的附件。成功之后经签名地址下载（`GET /api/v0/transfer-jobs/{job_id}/download`，不带令牌，计入 `anonymous` 桶，按客户端 IP），保留 `transfer.export_ttl`（默认 24 小时）；每人每本笔记本只留最新的一份，同时只有一个在进行（409 `transfer.busy`）。导出贡献者（之后的 M 注册）加的文件也进 zip，记在 `meta.json` 的 `contributed`，导入时不导入。
- **导入**（`POST /api/v0/notebooks/{notebook_id}/imports`，`multipart/form-data`：可选的 `parent_id`，然后是 `file`）：笔记本的编辑者与管理员把一个 zip（Obsidian 的库、本系统的导出，或任何 Markdown 文件夹）导入到根或一页之下。`.md` 是页面，文件夹是同名页面的子页面，其余文件是附件；隐藏的文件与文件夹（如 `.obsidian`）、`__MACOSX`、`Thumbs.db` 不导入；不能作为标题的名称修正、已占用的加序号，不能安全导入的条目（越出目录、符号链接、加密、压缩比过大、名称不是 UTF-8、过深、过大等）跳过，报告逐条列出。zip 至多 `transfer.import_max_bytes`（默认 512 MiB）、`import_max_entries` 个条目、解压后 `import_max_unpacked_bytes`，建的节点（条目加路径隐含的文件夹）至多 60,000 个（一次撤销能撤销的上限），超过的整包失败、什么都不写。一本笔记本同时只有一个导入（上传中的也算，第二个在读文件之前答 409 `transfer.busy`）。导入分批写进笔记本（一批至多 100 个节点），整个导入是一个变更集；中途失败、取消或服务重启时，已写的部分留在笔记本里，报告写明。
- **任务**：从回收站恢复一项（`POST /api/v0/notebooks/{notebook_id}/trash-restores`）与整组撤销一个变更集（`POST /api/v0/notebooks/{notebook_id}/reverts`）在一次请求里做不完时（422 `page.restore_too_large`、`page.revert_too_large`）可以在后台分批进行，同导入是任务：一本笔记本同时只有一个导入、恢复或撤销（409 `transfer.busy`），每批在锁下重新核对，途中停下时已完成的部分留着，报告写明；后台的撤销至多 60,000 个条目，超过答 422。撤销里必须一起做的一段（一个节点暂时的名称或位置与它的放回、一页的恢复与写回原来的正文、撤销的撤销里一页的写回与删除它）总在同一批里，两批之间别人看不到暂时的名称；一段放不进一批（它带回的页、附件、历史或链接一批写不下）时任务停下（`revert_too_large`），在它之前撤销的部分留着，报告写明。`GET /api/v0/notebooks/{notebook_id}/transfer-jobs` 按游标列出（自己的；笔记本的管理员看到所有人的），`GET /api/v0/transfer-jobs/{job_id}` 读一个与它的报告，`POST /api/v0/transfer-jobs/{job_id}/cancel` 取消（排队的立即取消，运行中的导出在一秒之内，导入、恢复与撤销在两批之间停下）。全部排队与运行中的任务至多 `transfer.max_queued`（默认 20），超出答 503 `server_busy`；一个任务至多运行 `transfer.job_timeout`（默认 6 小时）。不自动重试：失败的说明原因，重新开始即可。服务停机时运行中的任务被打断，记成失败 `interrupted`（导入、恢复与撤销已写的部分留着），重启之后不再运行。进程恰在 River 取到任务、任务开始之前停掉时，任务一直排队，直到 River 的救援（`transfer.job_timeout` 加 1 小时，默认 7 小时）之后才记成失败：其间它占着队列的一个名额，这本笔记本开始不了新的导入、恢复与撤销（导出是同一个人在这本笔记本），答 409 `transfer.busy`；开始它的人与笔记本的管理员可以取消它。
- **后台任务**：每 5 分钟把心跳停了超过 `transfer.heartbeat_timeout` 的任务记成失败（服务启动时也做一次），每 15 分钟让到期的导出过期并删掉 zip，每天清扫存储里不属于任何任务的 zip。导出与导入在各自的队列里运行（从回收站恢复与整组撤销在导入的队列里），`jobs.export_workers` 与 `jobs.import_workers` 加上搜索的建索引任务（运行时占一个连接）合起来至多 `database.max_conns` 的一半（默认各 1，所以 `max_conns` 至少 6；更小的配置启动失败）。

### 事件流

- `GET /api/v0/events` 是一条 SSE（`text/event-stream`），一个账户一条：推送它看得到的笔记本里的变化，只带 id，客户端收到后自己重新取数。访问令牌与 PAT 都可以（`Authorization: Bearer`）；浏览器的 `EventSource` 带不了这个头，用 `fetch` 流式读取。看得到的笔记本与 `GET /api/v0/notebooks/{notebook_id}` 答 200 的相同，在连接建立时算好；帧按提交的次序到达，回滚的事务什么也不发。
- **帧**：`event: <类型>` 与一行 `data: <JSON>`，`data` 带 `workspace_id` 与 `notebook_id`。
  - `hello`：第一帧，`{"heartbeat_seconds"}`；
  - `pages`：一个写入单元，`tree` 说明是否改了树（新建、改名、移动、删除、从回收站恢复），`pages` 是写了正文的页与新版本 `[{id, revision}]`（新建的页在版本 1）；多于 20 页时是 `null`，当作每一页都写过；
  - `lock`：一页的编辑会话开启或结束，`{page_id, session_id}`，去读锁；接管是先结束、再开启的两帧；
  - `links`：一个写入单元改变了链接索引或别名，`{pages, targets, aliases}`：`pages` 是链接改指别的页的页（不含这个单元写了正文的页，它们在 `pages` 帧里），`targets` 是反链变了的页；各自多于 20 页时是 `null`，当作这个笔记本的每一页都变了。`aliases` 说明这个单元是否增加或去掉了某一页的别名（随正文，或随这一页本身）：版本的链接不在索引里、按现在的别名解析，客户端于是重读这个笔记本显示着的每个版本的阅读视图（回收站的预览也是）；别名变了而没有链接改指别处时，`pages`、`targets` 都是空列表。`nervewiki reindex` 每个笔记本发一帧，两个列表都是 `null`，`aliases` 为假（`null` 已当作每一页都变了）；
  - `reset`：`{reason}`，流的最后一帧；
  - 心跳是一行注释 `: heartbeat`。不认识的类型跳过：之后的 M 会加自己的类型。
- **`reset`**：服务端随即关闭连接，客户端重连并整体刷新（树与打开的页），不补发事件。原因：`access`（看得到的笔记本可能变了：工作区或笔记本的成员身份、笔记本的开放程度、停用与恢复）、`notebooks_deleted`（看得到的笔记本被删除，删工作区也是）、`expired`（凭证到期，续期之后重连）、`unauthenticated`（心跳时重新认证失败：撤销、别处退出登录、停用）、`reconnected`（服务端接收通知的连接断开又连上，期间的事件丢了）、`overflow`（客户端读得太慢，缓冲的 64 个事件满了）。因 `access`、`notebooks_deleted`、`reconnected`、`overflow` 而 `reset` 时，之前已经送到这条流的事件先写出；凭证到期或失效时不再写出。流也可能不带 `reset` 就结束：服务停机、心跳时认证服务出错、客户端一个心跳之内收不下一帧，客户端同样重连并刷新。
- **心跳**：每 `events.heartbeat_interval`（默认 20 秒，5–50 秒）一次，同时重新认证凭证，所以撤销与别处的退出登录在一个心跳之内生效。
- 服务刚启动、接收通知的连接还没连上或正在重连时，打开流答 503 `not_ready`（`Retry-After: 1`）；`/readyz` 不看这条连接，这期间实例照常就绪，其余接口不受影响。接收通知的连接安静 30 秒就 ping 一次，悄悄断掉的连接（数据库切换、NAT 忘了它）由此发现并重连。
- 打开一条流在限流中算一次请求，读它看得到的笔记本受 `server.request_timeout` 约束；连着的流不受请求期限与服务端写超时约束，每一帧要在一个心跳之内写出，停机开始时关闭。

### MCP

- **两种地址**：`/mcp` 是整个账户（账户入口），`/mcp?notebook=<笔记本 id>` 是一本笔记本（笔记本入口）。`notebook` 只能出现一次、是 UUID，否则答 404 `notebook.not_found`，不查库；看不到或已删除的笔记本同样答 404，在握手之前。
- **只收个人访问令牌**：`Authorization: Bearer <PAT>`。会话的访问令牌答 403 `forbidden`（它 15 分钟就过期，服务端没有 OAuth，经 MCP 续不了期），没有、无效、已撤销的令牌答 401。服务端没有 OAuth：`/.well-known/` 之下都答 404，`POST /register` 答 OAuth 的错误 `invalid_client_metadata`。一个 POST 在限流里算一次请求，与 REST 共用这个令牌的桶。
- **传输**：无状态的 Streamable HTTP，每个 POST 一个 JSON 答复，没有会话：服务重启之后客户端照常调用，不用重新连接。协议版本到 2025-11-25，更新的版本协商降到它；GET 答 405，JSON-RPC 的批答 400；每个答复带 `Cache-Control: no-store`。一个工具的答复至多 48 KiB，到了就停并给出继续的游标或标出截断；超过 64 KiB 的请求体只能是这个入口的工具的调用，否则答 413。这样大的请求体整个服务同时至多两个（超过 1 MiB 的正文写，网页的保存与 REST 的，也算在内）、每个凭证（令牌或登录会话）一个，取不到名额的等 2 秒后答 503 `server_busy`（`Retry-After: 1`）：这是 HTTP 的答复，不是工具的结果（官方 TypeScript SDK 的客户端把它当作传输的错误；Claude Code、Codex 怎样显示未核对，见下文"HTTP 的答复"），稍后重发即可；一个 agent 的大的写一个接一个地发。请求体至多 10,000 个 JSON 值（每个对象、数组、字符串、数字与字面量各算一个），过了答 400。
- **工具**：笔记本入口有 `list`（根或一页的子节点，或按 glob 列路径）与 `read`（页的行；附件的元数据与图片或下载的地址）；账户入口另有 `notebooks`（看得到的笔记本，带工作区与角色）与 `open_notebook`（一本笔记本的说明与概况），这个入口里 `list`、`read` 与 `open_notebook` 要 `notebook`（笔记本的 id）。路径从笔记本的根写起，段之间是 `/`：页是它的标题加 `.md`（`Projects/Roadmap.md`），附件是它的名称，每段不分大小写。清单与输入的 schema 在 `api/mcp/tools.json`。写的四个工具、`batch` 与历史的读见下三条；读与找（`grep`、`search`、`index`、`backlinks`）见再下两条。
- **写**：两个入口都有 `write`（整页的正文）、`edit`（替换正文里的一段）、`move`（移动、改名或两者）与 `delete`（进回收站），账户入口里同样要 `notebook`；路径照上一条写，页的版本是 `list`、`read`、`write` 的答复里不透明的 `version`。`write` 不带 `base_version` 是新建：缺的上级页一并建成空的页，在同一个变更集里；路径已是一页时，内容相同答未改动（`changed: false`），不同答 `page.title_taken`。带 `base_version` 是覆盖正文：这个路径现在是另一页时答 `page.revision_mismatch`（带现在的版本）；是这一页时，内容与它现在的相同就什么也不写、答未改动（`changed: false`），`base_version` 过时了、页正被编辑也一样；不同时，这一页之后被写过答 `page.revision_mismatch`，重读再写。正文至多 5 MiB、不含 NUL。`edit` 的 `old_string`（至多 1 MiB）要逐字节出现恰好一次（重叠的也算；`replace_all` 从左到右替换每一处），从 `read` 给的照抄，引号与空白照原样匹配；全是 CRLF 的页上，不带 `\r` 的 `old_string` 里的 `\n` 照 `\r\n` 找，`new_string` 里的照 `\r\n` 写。不带 `base_version` 的 `edit` 改现在的正文（做的期间被写过就在新的正文上再做一次，再做的期间又被写过时答 `page.revision_mismatch`），带了的在那之后被写过答 `page.revision_mismatch`。`move` 的 `to` 是节点完整的新路径（不是移进去的文件夹），上级页要已存在；附件改名要带一个扩展名，扩展名可以换；链接照"改名与移动改写链接"改写。有子页或附件的页，`delete` 要 `recursive: true`。正在网页上编辑的页（包括自己在别处编辑的）拒绝 MCP 的写（与页现在的内容相同的 `write` 除外：它什么也不写）、编辑与删除，答 `page.locked` 并说出持锁人（删除列出子树里每个被持着的页，至多 20 个，另给总数）；`move` 要改写的页正被编辑时答 `linking.pages_locked`。锁不能经 MCP 接管：结束编辑，或请笔记本的管理员强制解锁。
- **batch**：两个入口都有，账户入口里同样要 `notebook`。至多 100 项写、编辑、移动与删除（`op` 是工具的名称，别的成员是那个工具的参数）与一句说明（`message`，1–4,096 字节，历史里随变更集显示），一个变更集，全成或全不成；每项看到前面各项留下的路径与正文，`base_version` 是 batch 之前的版本，batch 新建的页不带。字符串合计至多 6 MiB，其中正文与 `new_string` 合计至多 5 MiB；新建、删除、移动与改名的节点至多 10,000 个，不止一次移到别的父节点下时，这些移动带着走的节点合计也至多 10,000 个；要读正文的页（`edit` 的，与之前的改名、移动碰到它的链接的 `write` 的）合计至多 14 MiB，过了时提示说为什么、把改名或移动放进另一个 batch；写了不止一页时，这些页最后的正文合计至多 100 页、8 MiB、20,000 条链接；跟着前面的改名、移动改写要写的页时，计划为此解析的正文也算在里面（同一页的另一份正文多算一页），这些解析另外合计至多 10 MiB（只写一页时前一条不限，10 MiB 仍限）。改名与移动的链接改写在读树之前数三样：改写的页至多 400 个（batch 写了、它的改名或移动碰到其链接的页算 2，写回 batch 之前的字节的算 1）、链接至多 120,000 条、页合计至多 34,406,400 字节（400 × 84 KiB，写了的页按它最后的大小算一次）；三个之内，逐页改写时一次解析在已改写的页留着的解析预算的份额旁放不下，也拒绝（别人占着预算时照旧答 `server_busy`）。过了这些答 `page.batch_too_large`，说出过了什么、界线与怎样拆开。一项被拒时答出第几项（`op`，从 0 起）与那个工具的码；batch 整体的界线（项数、字符串与正文的合计、说明、写的页的合计与它们的链接）被拒时不带 `op`；被拒时什么都不写。先删子孙、再删祖先合成一次删除（前面那一项答 `merged_into`）；不能删除这个 batch 新建、改名或移动的。与分开调用的不同在两处。一是链接：链接跟着 batch 按节点合并的改名与移动，从 batch 之前、或写了的页最后一次写时的去向跟起，其间别的项对去向做的（新建、改名、移动或删除一页）与 batch 改的别名都看不到，`update_links` 为假的节点的链接在每一页里都不改。二是锁：batch 里写一页已有的字节、而这一页正被编辑时，整个 batch 答 `page.locked`（单独的 `write` 答 `changed: false`）。batch 写或删除的页正被编辑时，在第一个写或删除它的项答 `page.locked`；只被链接的改写碰到的页在它的改名、移动之后才查，在第一个改名或移动的项答 `linking.pages_locked`。答复有变更集的 id 与时刻和各项的结果（放得下的那些，`results_count` 是全部的数，其余用 `changeset` 读）；每项的 `changed` 是它的工具在那一步会答的，batch 的 `changed` 说它留没留下改动；batch 写的要几秒之后才搜得到，其间答复的 `building` 为真。batch 的变更集在网页上可以整组撤销（上文"整组撤销"）。
- **历史**：两个入口都有 `versions`（一页的版本列表，带 `revision` 时读那个版本的行，`offset`、`column` 与 `limit` 同 `read`）、`log`（这本笔记本的变更集，新的在前，可按 `path`、`actor`（账户的 id，`"me"` 是自己）与 `client`（`mcp` 是全部的 agent）筛选）与 `changeset`（一个变更集与它对每个节点做了什么，节点带它现在的路径），账户入口里同样要 `notebook`。只读：还原版本、从回收站恢复与撤销变更集是人在网页上做的，经 MCP 的一次移动加改名在网页上一步撤销，一个 `batch` 在网页上整组撤销。`changeset` 只认这本笔记本的，别的笔记本的变更集答 `page.changeset_not_found`；回收站里的页与附件没有路径（`versions`、`log` 按路径找不到它，`changeset` 里它的路径为 null），它们的变更集在不筛选的 `log` 里；一个答复放不下的变更集在 `log` 里单独来、变化截短并带 `changes_cut: true`，用 `changeset` 读全。
- **找**：两个入口都有 `grep` 与 `search`，账户入口里同样要 `notebook`。`grep` 以 Go 的正则（RE2，同 ripgrep 默认的语法）逐行找这本笔记本里页的正文（含 frontmatter；不找标题与附件，标题用 `list` 的 glob 找），按路径的次序一页接一页：每个命中带页的路径、行号（同 `read` 的数法：以它为 `offset`、`limit` 为 1 读到的就是这一行）与这一行存下的文字（去掉 `\r`，第一行去掉 BOM）；长于 1,000 字节的行截在第一个匹配的周围（`cut: true`），不从行首起时 `column` 是它起头的字节（同 `read` 的 `column`）。`glob` 只留路径匹配它的页（同 `list`），`ignore_case` 同 `(?i)`，每页至多 `limit` 个命中（1–500，默认 100）。模式至多 1,024 字节、不含换行，每行单独匹配；它同正文的行一样按 NFC 读：组合的记号要与前一个字符分开时写成转义（如 `\x{301}`）；太复杂的（如大的重复）答 `validation_failed`。一次调用查的有界（至多 500 页、32 MiB 的正文与有界的匹配的工作，在调用的期限之前 3 秒停下；同时匹配的调用有名额，等 1 秒取不到答 `server_busy`）：`next_cursor` 还在就接着调；`incomplete` 为真说这次调用在查完它该查的页之前停在界线上，这时没有命中不说明没有；`partial_lines` 是长得一次调用匹配不完的行，只匹配了开头，用 `read` 读它们。模式里取得出词时，`grep` 以搜索的索引挑出要查的页，词可能被截掉的页总被查；批量入队的页（`batch`、导入、后台的恢复与撤销、升级写的）在索引建好之前不查，其间 `building` 为真。取不出词的模式（如 `\d{4}-\d{2}-\d{2}`）一页接一页地查每一页，批量入队的也查，`building` 为假。两次调用之间被移动或写过的页（连同子页）可能被跳过或再查一次。`search` 是这一本笔记本的全文搜索（同上文"搜索"：名称里有每个词的在前，其余的在后，各按最后改动从新到旧；摘要是纯文本，截处是 `…`；第一页带约数；每页 1–50 条，默认 20）：单个汉字只找标题；一页可能少于 `limit` 条、甚至一条也没有而带 `next_cursor`，接着调；`building` 同 `grep`。按词找、找中文用 `search`；按模式、原样的文字与行号用 `grep`。
- **索引与反链**：两个入口都有 `index` 与 `backlinks`，账户入口里同样要 `notebook`；都只读，读的是链接索引（上文"链接索引"）。`index` 列出这本笔记本里索引有的页，每项：路径、frontmatter 的 `type` 与 `summary`（不分大小写的第一个这样的键，值是文字时）、标签（至多 20 个）、正文的最后改动、链到它的别的页数（1,000 是 1,000 或更多）与父页的路径；`type`、`summary` 与每个标签截到 256 字节，`cut` 说出截了哪些。`type` 只留这种类型的页（整个相等），`tag` 留有这个标签或它之下的标签的页（`plan` 含 `plan/q4`，开头的 `#` 去掉），`parent` 留这一页的子页（`/` 是根下的）；不筛就不传，传 `""` 答 `validation_failed`。有类型的页在前、按类型排，其余的在后；每页 1–100 项，默认 50，`next_cursor` 还在就接着调。索引还没读过的页不在其中（升级之后 `nervewiki reindex` 之前写的那种）。`backlinks` 列出链到一页的页：每项带它的路径、它链到那里的链接数（1,000 是 1,000 或更多）与前十个链接所在的行（各至多 240 字节）；一页链到自己的不算，索引读过它之后又写过的页还没有行；每页 1–100 项，默认 50。只有页有反链：附件的路径答 `page.not_found`。
- **说明**：笔记本入口握手时给出这本笔记本根下标题为 `AGENTS` 的页（不分大小写）去掉 frontmatter 的正文，至多 32 KiB：截在最后一个换行，末尾一行说出怎样用 `read` 读全文。没有这一页、或它去掉 frontmatter 之后只剩空白时，给内置的英文说明（路径的写法与工具）；改了、替换、改名或删掉这一页之后，下一次连接就是新的。团队要别的约定（例如中文）就写一个 `AGENTS` 页，它整个取代内置的说明。
- **客户端的名称**：取客户端报的 `clientInfo.name`（清理之后至多 128 个字符），记在这个令牌上，日志的每次调用带它；重启之后、客户端没有再报时用记下的。一个 agent 用一个 PAT：两个客户端同时用一个时，记的是最后报的那个。
- **`server.public_url`**：实例对外的地址（只到源，默认空）。`read` 给附件的下载地址时以它起头（签名同"附件"）；为空时图片照样直接放进答复，别的附件只给元数据。
- 接上 Claude Code、Codex 见下一节"接入 agent（MCP）"；反向代理的要求见"部署"。

## 接入 agent（MCP）

Claude Code、Codex 一类的 agent 经 `/mcp` 读写笔记本。工具、路径与它们的规则见上文"MCP"一节；这一节讲怎样接上、审批怎样设、有哪些上限，以及出错时看到什么。下面的样例照两个客户端的文档与 M0/P1 的实验（Claude Code 2.1.285、Codex 0.157.1）写成，还没有在真实的客户端上照[人工清单](docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md)走过；清单在 v0.1 的最后走一遍，之后这里写明核对过的客户端的版本，并照它改正标出的几句：标着"照文档"的照客户端自己的文档写，标着"未核对"的是推断。

### 先准备

- **一个 agent 一个 PAT**：在"设置"→"访问令牌"（`/settings/tokens`）创建，名称写明给哪个 agent、在哪台机器上（如 `claude-code-laptop`），最好设一个期限；也可以 `POST /api/v0/me/api-tokens`（上文"账户与认证"）。令牌只显示这一次。PAT 能做这个账户能做的一切：笔记本的地址只是"进到这本笔记本"，不是权限的边界。一个 agent 一个令牌，撤销它只切断这一个 agent；agent 的变更集记为 `mcp:<客户端报的名称>`（网页的历史显示"MCP（名称）"），名称记在令牌上，两个客户端共用一个令牌时记的是最后报名称的那个。
- **令牌放在环境变量里**，一个 agent 一个变量：下面的样例里 Claude Code 用 `NERVEWIKI_CLAUDE_TOKEN`、Codex 用 `NERVEWIKI_CODEX_TOKEN`。同一个变量就是同一个 PAT：两个 agent 的写在历史里分不清，撤销它两个一起断。在终端里设时用 `read -rs NERVEWIKI_CLAUDE_TOKEN && export NERVEWIKI_CLAUDE_TOKEN`（粘贴令牌、回车：不回显，也不进 shell 的历史）；长期用的写在只有自己读得到的 shell 配置里，或由密钥管理器给出。不要把令牌写进会提交的文件。
- **只经 HTTPS 连 `/mcp`**（本机试用除外）：每个请求的头里都是 PAT。
- **实例设好 `server.public_url`**（`NWIKI_SERVER__PUBLIC_URL`，只到源，如 `https://wiki.example.com`）：否则 `read` 读附件时只给图片，别的附件没有下载的地址。
- **两种地址**（`wiki.example.com` 换成实例对外的地址）：
  - 笔记本入口 `https://wiki.example.com/mcp?notebook=<笔记本 id>`：笔记本的 id 是网页地址里 `/notebooks/` 之后的那一段。握手时下发这本笔记本的 `AGENTS` 页（上文"MCP"的"说明"）。一个 agent 在一本笔记本里工作时用它。
  - 账户入口 `https://wiki.example.com/mcp`：agent 先以 `notebooks` 看有哪些笔记本，再以 `open_notebook` 读一本的说明与概况，之后每个工具都带 `notebook`。
  - 地址里有 `?`：写在 shell 里时加引号（zsh 把 `?` 当作通配）。

### Claude Code

- **只给自己**（默认的本地范围：服务与令牌存在 `~/.claude.json`，只在当前的目录里、只有自己用；照 Claude Code 的文档）：

  ```sh
  claude mcp add --transport http nervewiki "https://wiki.example.com/mcp?notebook=<笔记本 id>" \
    --header "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN"
  ```

  shell 先展开 `$NERVEWIKI_CLAUDE_TOKEN`，存下的是令牌本身；加 `--scope user` 则在每个目录里都有它（照 Claude Code 的文档）。

- **随项目共享**（项目的范围：项目根目录的 `.mcp.json`，它通常会提交）：`.mcp.json` 里**绝不要写令牌**，写环境变量的展开（照 Claude Code 的文档），每个人在自己的环境里设 `NERVEWIKI_CLAUDE_TOKEN`：

  ```json
  {
    "mcpServers": {
      "nervewiki": {
        "type": "http",
        "url": "https://wiki.example.com/mcp?notebook=<笔记本 id>",
        "headers": {
          "Authorization": "Bearer ${NERVEWIKI_CLAUDE_TOKEN}"
        }
      }
    }
  }
  ```

  用命令写它时加 `--scope project`，令牌那一段用**单引号**，让 `${NERVEWIKI_CLAUDE_TOKEN}` 原样进文件：

  ```sh
  claude mcp add --scope project --transport http nervewiki "https://wiki.example.com/mcp?notebook=<笔记本 id>" \
    --header 'Authorization: Bearer ${NERVEWIKI_CLAUDE_TOKEN}'
  ```

  写好之后 `grep nwk_pat_ .mcp.json` 应当什么也找不到。项目里的服务第一次用时，Claude Code 先问要不要启用它（照 Claude Code 的文档）。

- **看连上没有**：`claude mcp list`，或在会话里输入 `/mcp`：`nervewiki` 已连上，笔记本入口有 14 个工具（账户入口 16 个）。工具在 Claude Code 里叫 `mcp__nervewiki__read` 一类（照 Claude Code 的文档）。
- Claude Code 把一次 MCP 工具的结果限在约 25,000 个 token，环境变量 `MAX_MCP_OUTPUT_TOKENS` 可以调（照 Claude Code 的文档）；服务端的每个答复的文字至多 48 KiB（`read` 直接放进答复的图片另算，至多 3 MiB），`read` 一次至多约 45 KiB 的正文，中文约 1.5 万字。

### Codex

- 写进 `~/.codex/config.toml`（设了 `CODEX_HOME` 时是 `$CODEX_HOME/config.toml`；照 Codex 的文档）：

  ```toml
  [mcp_servers.nervewiki]
  url = "https://wiki.example.com/mcp?notebook=<笔记本 id>"
  bearer_token_env_var = "NERVEWIKI_CODEX_TOKEN"
  default_tools_approval_mode = "prompt"
  ```

  文件里没有令牌：Codex 启动时从 `NERVEWIKI_CODEX_TOKEN` 读它，所以启动 Codex 的 shell 里要有这个变量。`default_tools_approval_mode` 见下面的"审批"。

- **看连上没有**：在会话里输入 `/mcp`，列出 `nervewiki` 与它的工具（照 Codex 的文档）。

### 审批

工具带着 MCP 的注解：读的九个（`list`、`read`、`grep`、`search`、`index`、`backlinks`、`versions`、`log`、`changeset`；账户入口另有 `notebooks`、`open_notebook`）是 `readOnlyHint`；写的五个（`write`、`edit`、`move`、`delete`、`batch`）是 `destructiveHint`，其中 `write`、`move`、`delete` 另是 `idempotentHint`（同样的参数再调一次不再改变什么）。调用之前问不问人由客户端与它的设置定；两个客户端是否照注解区别对待未核对。

- **不要一律放行**：放行了写的工具，agent 不经人确认就能以这个账户的权限改写、删除、移动它看得到的每一本笔记本里的页。读的工具可以免问；写的工具照常问，除非这个 agent 要无人值守地工作，而你接受事后在网页的历史里检查、撤销它做的（下面的"历史只读"）。审批只管 MCP 的工具：agent 能执行命令时，它启动的命令照常继承启动它的环境（这是 shell 的），里面就有令牌的变量，可以直接调 REST；客户端是否把自己的环境交给它执行的命令（或滤掉一些变量）未核对，当作交了。
- **Claude Code**：一个工具在调用之前问；回答"不再问"之后这个工具不再问（照 Claude Code 的文档）。要事先放行读的工具，写进设置（项目的 `.claude/settings.json` 或自己的 `~/.claude/settings.json`）的 `permissions.allow`：

  ```json
  {
    "permissions": {
      "allow": [
        "mcp__nervewiki__list",
        "mcp__nervewiki__read",
        "mcp__nervewiki__grep",
        "mcp__nervewiki__search",
        "mcp__nervewiki__index",
        "mcp__nervewiki__backlinks",
        "mcp__nervewiki__versions",
        "mcp__nervewiki__log",
        "mcp__nervewiki__changeset"
      ]
    }
  }
  ```

  账户入口另加 `mcp__nervewiki__notebooks` 与 `mcp__nervewiki__open_notebook`。`claude -p`（不交互）问不了人，没有放行的工具不调用：以 `--allowedTools` 给它这次要的工具，写的工具只给确实要写的任务（照 Claude Code 的文档）。

- **Codex**：一个服务的 `default_tools_approval_mode` 决定调用它的工具之前问不问人，取值是 `auto`、`prompt`、`writes` 与 `approve`；`approve` 是都不问。`codex exec`（不交互）问不了人，要审批的调用都被拒。交互的会话用 `prompt`；只读的无人值守的任务不必放行写的工具；`approve` 只给确实要无人值守地写的任务，并照上面的第一条接受它的代价。

### 上限

| 什么            | 上限                                                                                                                                                                                                   |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 一次调用        | 15 秒（`server.request_timeout`，从读完请求体算起）；请求体至多 37,814,272 字节（约 36 MiB），超过 64 KiB 的只能是这个入口的工具的调用；请求体里至多 10,000 个 JSON 值                                 |
| 每个工具的答复  | 48 KiB：到了就停，给出 `next_cursor`（或 `next_offset`、`next_column`）接着调，没有游标的标出截断                                                                                                      |
| `read`          | 一次至多约 45 KiB 的正文（中文约 1.5 万字）；长的一行经 `column` 分几次读完。图片（PNG、JPEG、GIF、WebP）至多 3 MiB、每边 8,000 像素时直接放进答复，别的附件给下载的地址（要配置 `server.public_url`） |
| `write`、`edit` | 正文至多 5 MiB；`edit` 的 `old_string` 至多 1 MiB                                                                                                                                                      |
| `batch`         | 至多 100 项，字符串合计至多 6 MiB、正文与 `new_string` 合计至多 5 MiB，别的界线见上文"MCP"的"batch"                                                                                                    |
| 说明            | 见上文"MCP"的"说明"                                                                                                                                                                                    |
| 限流            | 一个 POST 算一次请求，与这个令牌的 REST 请求共用一个桶（`ratelimit.authenticated`）                                                                                                                    |

### 大的请求体的名额

超过 64 KiB 的调用（大的 `write`、`edit`、`batch`）在整个服务里同时至多两个、每个令牌至多一个；取不到名额的等 2 秒，然后答 503 `server_busy`（`Retry-After: 1`），稍后重发即可。所以一个 agent 的大的写一个接一个地发（内置的说明也这样告诉 agent）。名额从读请求体之前占到这次调用答完：慢的上行占得久（1 Mbit/s 上过约 250 KB 就过 2 秒），这时同一个令牌同时发出的两个大的写，第二个多半拿到 503。反向代理照"部署"一节为 `/mcp` 缓冲请求体时，请求在代理收完之后才到服务，名额只占服务端读请求体与做这次调用的时间，不再随上行的快慢。

### HTTP 的答复

这些是 HTTP 一层的答复，在任何工具运行之前：它们不是工具的结果，客户端把它们当作连接或传输的错误（官方 TypeScript SDK 的客户端是这样）；Claude Code、Codex 怎样显示、模型看不看得到未核对。工具自己的错误不在其中：它们是带 `isError` 的结果，模型读得到码与说明（`page.locked`、`page.revision_mismatch` 一类），照说明改了再调。

| 答复                                                                                                                  | 什么时候                                                                                                                                                                                                                                                                                                                                                                                                                                                               | 怎么办                                                                                                                                                                             |
| --------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 401 `unauthorized`，"This server has no OAuth: MCP takes a personal access token in the Authorization header."        | 没有带令牌，或带的 `Authorization` 不是 `Bearer <令牌>`（`Bearer` 之后是空的、别的方案、令牌里有空白）                                                                                                                                                                                                                                                                                                                                                                 | 在客户端的配置里加 `Authorization: Bearer <PAT>`（下一段）；已经加了的，看令牌的变量在启动客户端（或执行 `claude mcp add`）的 shell 里设了没有（变量没有设时客户端发出什么未核对） |
| 401 `unauthorized`，"The bearer token is invalid or has expired."（`WWW-Authenticate: Bearer error="invalid_token"`） | 令牌写错了、过期了或被撤销了                                                                                                                                                                                                                                                                                                                                                                                                                                           | 新建一个 PAT，换上                                                                                                                                                                 |
| 403 `forbidden`，"MCP takes a personal access token, not a session's access token."                                   | 带的是网页登录的访问令牌                                                                                                                                                                                                                                                                                                                                                                                                                                               | 换成 PAT                                                                                                                                                                           |
| 404 `notebook.not_found`                                                                                              | 地址里的笔记本 id 不对、为空、出现两次，或你看不到这本笔记本、它已被删除                                                                                                                                                                                                                                                                                                                                                                                               | 改正 id；或用账户入口的 `notebooks` 看有哪些                                                                                                                                       |
| 404 `not_found`                                                                                                       | 地址是 `/mcp/` 之下的路径                                                                                                                                                                                                                                                                                                                                                                                                                                              | 地址是 `/mcp` 本身                                                                                                                                                                 |
| 400 `bad_request`                                                                                                     | 一次发了 JSON-RPC 的批、请求体不是一个合法的 JSON 对象（空的请求体见下一行），或 JSON 值过了 10,000 个                                                                                                                                                                                                                                                                                                                                                                 | 一条消息一个请求                                                                                                                                                                   |
| 400、415（纯文字，不是上面的 JSON）                                                                                   | 请求头不对：`Accept` 不是同时认 `application/json` 与 `text/event-stream`（`application/*`、`text/*` 与 `*/*` 也算，所以 `curl` 默认的 `*/*` 可以）、`Content-Type` 不是 `application/json`，或 `MCP-Protocol-Version` 是服务端不说的、排在 `2026-07-28` 之前的版本（服务端说 `2025-11-25`、`2025-06-18`、`2025-03-26`、`2024-11-05`；不带这个头也行）；或请求体不是 SDK 认得的请求（空的、方法名写错、少了 `id`、`jsonrpc` 不是 `"2.0"`、`tools/call` 没有 `params`） | 照下面的 `curl` 写这几个头与请求体                                                                                                                                                 |
| 400（JSON-RPC 的错误，`application/json`）                                                                            | `MCP-Protocol-Version` 是 `2026-07-28` 或更新的版本：它们过得了上一行的检查，SDK 再以 JSON-RPC 的错误拒绝（缺那个版本要的 `_meta` 时 `-32602`，带了时 `-32022` `unsupported protocol version`，列出服务端说的版本）                                                                                                                                                                                                                                                    | 用服务端说的版本，如下面的 `curl` 的 `2025-11-25`                                                                                                                                  |
| 413 `payload_too_large`                                                                                               | 请求体过了约 36 MiB，或超过 64 KiB 而不是工具的调用                                                                                                                                                                                                                                                                                                                                                                                                                    | 拆小：一页的正文至多 5 MiB                                                                                                                                                         |
| 429 `rate_limited`                                                                                                    | 这个令牌的请求过了限流；或同一个客户端 IP 上认证失败的令牌太多（失败闸门 `auth_failure`，按 IP：反向代理之后要设 `server.trusted_proxies`，否则所有人共用代理的 IP）                                                                                                                                                                                                                                                                                                   | 等 `Retry-After` 的秒数；后一种先换掉失败的令牌（上面 401 的第二行）                                                                                                               |
| 503 `server_busy`（`Retry-After: 1`）                                                                                 | 大的请求体没有名额（上一段）                                                                                                                                                                                                                                                                                                                                                                                                                                           | 稍后重发，大的写一个接一个地发                                                                                                                                                     |

**没有配置令牌时**：客户端看到的都是同一句："This server has no OAuth: MCP takes a personal access token in the Authorization header."。不走 OAuth 的客户端从 401 的说明里读到它；会走 OAuth 的客户端（例如官方 TypeScript SDK 的 OAuth 客户端）先找 OAuth 的元数据（`/.well-known/` 之下都答 404）、再试注册（`POST /register` 答 OAuth 的错误 `invalid_client_metadata`），从注册的错误里读到它。Claude Code、Codex 各走哪一条未核对。照它在配置里加上 `Authorization: Bearer <PAT>` 即可。

**自己试一下**（不握手，不改这个令牌记下的客户端名称）：

```sh
curl -sS "https://wiki.example.com/mcp?notebook=<笔记本 id>" \
  -H "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN" -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-11-25' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

（只给 Codex 设了令牌的，换成 `$NERVEWIKI_CODEX_TOKEN`。）答 200 与工具的清单就是通的；别的答复照上表。

### 写的规矩

- **`base_version`**：`list`、`read`、`write`、`edit` 的答复里有页的 `version`（`"<节点 id>:<版本号>"`），它说出读到的是哪一版。覆盖一页用 `write` 带读到的 `version` 作 `base_version`；`edit` 不带它时在现在的正文上做（做的期间被写过就在新的正文上再做一次，再做的期间又被写过就拒绝），带了的在那之后被写过就拒绝。拒绝是工具的错误 `page.revision_mismatch`（带 `isError` 的结果，不是 HTTP 的答复），`current_version` 是现在的版本：重读再写。同一个路径换成了另一页（原来的被改名、移走之后有人新建了同名的页）时同样拒绝，不会写到新的页上。与这一页现在的内容相同的 `write` 什么也不写、答未改动（`changed: false`），带的 `base_version` 过时了也一样。
- **锁**：正在网页上编辑的页（包括你自己在别的浏览器里编辑的）拒绝 agent 的写（与页现在的内容相同的 `write` 除外：它什么也不写）、编辑与删除，答 `page.locked`，说出谁在编辑；`move` 要改写链接的页正被编辑时答 `linking.pages_locked`。agent 不能接管锁：等编辑的人离开编辑（关掉浏览器之后锁至多约 2 分钟过期），或请笔记本的管理员强制解锁。agent 自己不开编辑会话，不挡网页上的人。
- **历史只读**：agent 读得到历史（`log`、`changeset`、`versions`），改不了它：还原版本、从回收站恢复、撤销变更集是人在网页上做的。agent 的每次写是一个变更集，一次 `batch` 也是一个（带它的说明）；网页的历史可以按"MCP"筛选，一次移动加改名在网页上一步撤销，一个 `batch` 整组撤销。
- `batch` 与分开调用只有一条规则不同、`grep` 与 `search` 怎样分工与怎样读 `incomplete`、`partial_lines`、`building`：见上文"MCP"的"batch"与"找"。

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
- **新手引导**：步骤注册在 `src/onboarding/steps.ts`，服务端只记录完成的步骤 id。加一步就是写它的组件、追加到 `onboardingSteps`（同时加进 e2e 的 `fixtures/auth.ts`）；已经完成前面步骤的用户下次访问只看到新的一步。现在三步：资料；工作区（已有工作区的账户，比如经邀请加入的，直接继续；没有的在这里创建一个；关闭创建时说明怎么加入，再继续）；笔记本（在要落到的、能建笔记本的工作区里：已经看得到笔记本的直接继续，看不到的建一本私密笔记本，名称默认按页面的语言是"我的笔记"或"My notes"；没有这样的工作区，即没有工作区或处处只是访客的，说明笔记本在哪里建，再继续）。M2 时完成了前两步的账户，下次访问只看到笔记本一步。
- **工作区的外壳**：`/` 落到这台设备最后访问的工作区（localStorage 的 `nwiki.workspace`）、按名称的第一个，或者创建页；`/:slug` 是工作区的外壳，左栏切换工作区，不是成员的 slug 显示 404。新的顶层页面要把它的路径段加进保留名单 `server/internal/modules/workspace/domain/reserved_slugs.txt` 的 `[app]`，vitest 核对路由与名单一致。
- **工作区的页面**：`/create-workspace` 建工作区，slug 随名称生成，停止输入之后问服务端是否可用；关闭创建时只说明怎么加入一个工作区。`/:slug/settings/general` 是工作区的名称与地址：管理员改名，删除要先输入 slug，删除之后回到 `/`；其他成员只能看。`/:slug/settings/members` 列出成员（访客看不到邮箱），管理员改别人的角色、移出、邀请（复制链接自己发出，浏览器不能复制时显示链接框；链接用管理员访问本站的地址拼出，经内网地址访问时复制的就是内网链接，对外发链接请从公开地址打开）、撤回邀请；每个成员都能离开，离开之后回到 `/`。
- **笔记本的页面**：工作区的左栏分"我的笔记本"（只有自己是成员的私密笔记本）与"团队笔记本"，"新建笔记本"在对话框里起名、选开放程度，建好之后进入它；换到别的工作区时左栏随之重建，对话框里没发出的草稿不带过去，在途的创建答复之后不再跳转。`/:slug/notebooks/:id` 是笔记本的外壳：在工作区的笔记本列表里找到它才显示，自己删除或离开之后回到工作区首页，看不到的显示 404。`settings/general` 是名称与开放程度（开放程度点"保存"才发出），管理员可以删除（先输入名称）；`settings/members` 列出成员，管理员从工作区成员中添加、改角色、移出，成员可以离开，没有管理员时页面说明一句。工作区设置的"无主笔记本"一页只给管理员：接管、删除（先输入名称），名称与原所有者都相同的两本另显示 id 的末六位；下面是审计记录，"加载更多"一页一页往下读。管理员的工作区首页提醒还有几本无主笔记本。
- **页面**：笔记本的左栏是它的页面树：展开与折叠，拖拽改变位置（或用"移动到…"对话框，键盘可用），新建子页、改名、删除（移到回收站）在每一项的菜单里；`Ctrl+O`（macOS 上 `Cmd+O`）快速切换页面。`/:slug/notebooks/:id/pages/:pageId` 是页面：面包屑、标题、阅读视图与子页面列表；阅读视图里代码块的高亮在 Worker 里做，超时就不着色；能写的人在阅读视图里勾选、取消任务项（复选框以它那一项的文字为名称，空格键也可以）。页面树、阅读视图与编辑锁随事件流实时更新：一个浏览器一条流，由一个标签页持有、转给同一登录的别的标签页；别人连续保存时一页的阅读视图至多 5 秒重读一次，隐藏的标签页在重新可见时重读；每次连上都整体刷新。别的标签页或别人删掉的页显示 404，本标签页删掉的去它的父页；正在编辑而有未保存的修改时页面留在原处，直到离开：页面、笔记本、工作区被删（之后又有同名的新工作区也一样），或被移出笔记本、工作区，都一样。
- **编辑**：写者点"编辑"或按 `Ctrl+E`（`Cmd+E`）先拿这一页的编辑锁，在原处换成源码编辑器（CodeMirror，第一次编辑时才下载）。别人正在编辑时页面写着"某某正在编辑这一页"，不能编辑；自己在别处编辑时可以"在这里编辑"接管；笔记本的管理员可以"解除锁定"。停顿约 2 秒自动保存，30 分钟没有输入就保存并退出编辑，关闭标签页时释放锁（释放没发出去时由租约兜底，至多 2 分钟）；编辑中失去锁（被接管、被解除，或过期之后被别人拿到）时编辑器只读，上方说明原因，"回到阅读"离开。`Ctrl+S` 保存，`Ctrl+E` 或"完成"保存之后回到阅读视图；保存的是编辑器里的文字加上原来的换行写法与 BOM，输入法组合中按的保存等组合结束再做。有未保存的修改时去别的页先确认，关标签页由浏览器提醒；退出登录（在哪个标签页都一样）先保存同一登录各标签页未保存的修改、结束它们的编辑，至多等 2 秒再登出。保存时这一页已被别人改过，编辑器上方显示差异，"保留我的"覆盖、"放弃我的"载入现在的正文。编辑模式不进地址：刷新回到阅读视图。
- **链接与右栏**：阅读视图里指向页的链接经路由在应用内打开（带修饰键或中键时由浏览器在新标签页打开），带锚点的去那个标题；解析不到的链接，写者点了先确认、再按服务端给的落点新建并打开，读者只看到"页面不存在"的说明。标签去 `/:slug/notebooks/:id/tags/:tag`，列出带它（及其下的标签）的页。公式（KaTeX）与图（mermaid）在第一次用到时才下载，各有上限：定义宏的公式、太深太大的公式与图显示原文。编辑器里 `[[` 补全页面的标题与别名，`#` 补全标签，输入法组合中不弹出。页面旁边是右栏（宽屏在右侧，窄屏在正文之后）：大纲（阅读时，至多前 1,000 个标题）、反链（"更多反向链接"一页页往下读）、属性（属性链接去那一页）、版本（见下面的"历史与回收站"），随事件流实时更新。
- **附件**：页面的中栏与笔记本首页有附件一节：列出一页下（首页是根下）的附件，上传（选文件或拖进来，有进度、可以取消，一次多个）、改名、删除、下载；文件夹与 `.md` 不上传，提示打成 zip 导入。编辑器里粘贴、拖入的文件上传到这一页下，在光标处插入 `![[名称]]`；"完成"等上传与输入法组合结束再离开。阅读视图里的附件在签名地址到期之前重读，正在播放的音视频换地址时不中断。
- **历史与回收站**：右栏的"版本"列出这一页最近的版本（一次编辑一个版本，编辑中的标出，"更多版本"往下读），点开是版本页 `/:slug/notebooks/:id/pages/:pageId/versions/:revision`：阅读、与上一版比较、与当前比较（比较在 Web Worker 里，行数、大小与时间有上限，超过时两个版本并排显示、不逐行标出），写者可以"还原到这个版本"（确认框里是还原会做的改动）。笔记本的左栏在页面树之下有"历史"与"回收站"，页面的菜单有"查看这一页的历史"。历史页 `/:slug/notebooks/:id/history` 按人、客户端与页面筛选（筛选在地址里），点开一条是它的变更集页：改动的节点一页页往下读，写者可以"撤销这次改动"，冲突时列出每个节点与可以怎么办，太大时可以"在后台撤销"。回收站页 `/:slug/notebooks/:id/trash` 列出各项与恢复之后放到哪里，预览只读（附件恢复之后才能下载），写者"恢复"，太大时"在后台恢复"；后台的任务列在"导入、导出与恢复"一页。删除页面与附件的确认框写明移到回收站、多少天内可以恢复、能读这本笔记本的人在回收站里仍看得到它。
- **导入与导出**：笔记本设置的"导入、导出与恢复"一页（回收站与单个变更集页上太大的恢复与撤销在后台进行时，任务也列在这里）：导入 zip（选位置，上传有进度、可以停止；上传期间关闭对话框、离开这一页或关闭标签页先问），导出整本笔记本，页面菜单的"导出此页"导出一页及其子页；下面是任务列表（进度、取消、报告、下载），有任务在进行时每秒刷新。
- **搜索**：工作区左栏的最上面是搜索框（`Ctrl+Shift+F`，macOS 上 `Cmd+Shift+F`；微软拼音、搜狗、fcitx 里这个键是简繁切换，用鼠标或 Tab 到框里），在一本笔记本里时默认只搜这一本，框下写明在哪里找；回车去 `/:slug/search?q=&notebook_id=`：可以换"在哪里找"，列出名称与正文里找到的页与附件（命中处高亮、正文的摘要、约数），"更多结果"往下读，一段正文里没找到时"继续查找"。结果不随事件更新，再搜一次时重读；从一条结果后退回来显示原来的结果。
- **邀请页** `/invitations/:id`：在守卫之外，令牌只在地址的片段里，不进 `next`，请求都把它放在请求体里。未登录时先看预览，在页内登录，或带着邀请注册（注册关闭时只有受邀的邮箱能注册）；会话变了页面仍停在原地址，登录之后接受，进入工作区（没完成引导的先走引导）。登录的邮箱不是受邀的，页面说明原因，可以在页内退出换账户。
- **请求的错误**：problem 码与字段码到文案的映射在 `src/app/problem-messages.ts`；vitest 读 `api/dist/openapi.yaml`，契约中任何一个操作列出的码没有文案时失败（只有页面不显示其错误的续期与退出除外）。
- **表单**：`src/app/form.ts` 的 `useForm` 是所有表单的发送：本地检查不通过就不发；服务端的字段错误在字段下方（个别 problem 码也可以指定字段，例如当前密码不对），其余在表单上方；发送中按钮禁用；失败之后焦点移到第一个有错误的字段。同一个 problem 码在个别页面要换一种说法时，页面把 `texts`（码到文案键）交给 `useForm`、`ConfirmDialog` 或 `CredentialsForm`，例如邀请页的注册被拒。
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
- 失败的测试在 `e2e/playwright-report/` 中带着 trace 与截图；`e2e/test-results/` 中有每个 worker 的服务日志和失败时导出的数据库。持续集成在失败时把两者作为 artifact 上传，并把每个失败的测试与错误写成这次运行的注释（Playwright 的 `github` 报告器），不看任务日志也读得到。
- `make e2e` 把 `VERSION` 交给故事核对注入的版本号；持续集成用 `0.0.0-ci.<运行号>`，与默认值不同。

## 部署

镜像由 `deploy/Dockerfile` 构建：前端与服务端都在其中，运行时是 distroless 镜像，只有 `/nervewiki` 一个程序，以非 root 用户（uid 65532）运行，监听 8080。

```bash
make image VERSION=0.1.0         # 构建 nervewiki:0.1.0
make image-smoke VERSION=0.1.0   # 在镜像上跑 S1、S3：迁移、探针、前端、实例与提交信息、注册关闭、管理员建账户、非 root、附件在重启之后读回、优雅停机、附件目录不可写时拒绝启动（另需 curl、jq、openssl）
```

镜像的提交信息取自构建上下文中的 `.git`，所以要在普通的克隆中构建：`git worktree` 的 `.git` 是指向别处的文件，`make image` 会直接报错。`.dockerignore` 排除的正好是 `.gitignore` 忽略的，改一个时同步另一个：否则镜像里的二进制报告的 `modified` 与工作区不符，`make image-smoke` 失败。

- 镜像默认 `NWIKI_ENV=prod`。配置用环境变量提供（也可以挂载一个目录并设置 `NWIKI_CONFIG_DIR`），至少要有数据库地址 `NWIKI_DATABASE__URL` 与签名私钥 `NWIKI_AUTH__JWT__PRIVATE_KEY_FILE`（见上文"账户与认证"；私钥文件要能被 uid 65532 读取）；其余配置项见 `server/configs/config.yaml`，合并规则见上文"配置"。
- prod 配置不自动迁移。每次升级先执行迁移，再启动服务（每个命令都校验整份配置，所以迁移也带上同样的变量）：

  ```bash
  key=(-v "$PWD/jwt.pem:/run/secrets/jwt.pem:ro" -e NWIKI_AUTH__JWT__PRIVATE_KEY_FILE=/run/secrets/jwt.pem)
  docker run --rm -e NWIKI_DATABASE__URL=… "${key[@]}" nervewiki:0.1.0 migrate up
  docker run -d -p 8080:8080 -v nervewiki-data:/data -e NWIKI_DATABASE__URL=… "${key[@]}" nervewiki:0.1.0
  ```

- **附件目录**（M7）：附件与导入导出的文件在 `/data`（`storage.dir`，镜像以 `NWIKI_STORAGE__DIR=/data` 设定），它是镜像声明的卷。环境变量优先于配置文件：挂载了配置目录也要改目录时，改 `NWIKI_STORAGE__DIR`。
  - 要挂载具名卷（如上例的 `-v nervewiki-data:/data`）或宿主机目录（`-v /srv/nervewiki/data:/data`）。不挂载时 Docker 建一个匿名卷，删掉容器之后就找不回来了。
  - 进程以 uid 65532 运行：宿主机目录的属主要是它（`sudo chown -R 65532:65532 /srv/nervewiki/data`；Kubernetes 里设 `securityContext.fsGroup: 65532`）。目录或其中的区（`blobs/` 等）不可写时 `serve` 拒绝启动，错误写明目录与 uid（`storage: cannot write in /data as uid 65532, gid 65532: …`）。
  - 一个目录只给一个 `serve` 进程用（v0.1 只支持单实例）：启动时它删掉上次中途退出留下的半截文件。升级时先停旧的再起新的（Kubernetes 的 `strategy: Recreate`），不要让两个进程同时挂着它。
  - 剩余空间低于 `storage.min_free_bytes`（默认 1 GiB）时不再接受写入，答 507。磁盘或配额已经写满时同样照常启动，写入答 507；这两种情况启动时都记一条 WARN（`storage is full`）。监控这块磁盘。
  - 备份：先 `pg_dump`，再复制 `/data/blobs`（附件写入后不变，物理清除要等 60 天，按这个次序得到的备份里数据库引用的附件都在；其余子目录是过程中的文件）。恢复时用同一次备份的两份，先恢复数据库，再放回目录；目录里多出的文件（数据库里没有的）由每天的清扫删掉。

- 升级到带链接索引的版本（M6）时，`migrate up` 之后、启动服务之前执行一次 `nervewiki reindex`（见上文"链接索引"），已有页面的链接才进入索引；之后的写入自己维护索引。用两个数据库角色时，先执行授权文件（见下）：

  ```bash
  docker run --rm -e NWIKI_DATABASE__URL=… "${key[@]}" nervewiki:0.1.0 reindex
  ```

- 升级到带搜索的版本（M8）时，迁移让已有的全部节点排队，服务起来之后由后台任务一个笔记本接一个笔记本地建索引（早建的笔记本在前）：一个笔记本建完之前，搜索找不到它里面的这些页、页上写明索引正在建立（其间改过的页立即找得到）。任务每次运行约 1 分钟，停下时所在的那个笔记本余下的页等这一轮转回来再建，一次建完全部的升级里那是最后：30 万页时约 15 个笔记本（多是大的）到全部建完才建好。重建的进度看还在排队的节点数：`SELECT count(*) FROM page_search_queue WHERE bulk`。在我们的测试机上，30 万页（1.9 GB 正文、5,000 个笔记本）约 16 分钟建完，写约 130 GB 的 WAL（每页约 0.44 MB），`max_wal_size` 调到 16 GB 只少约 3%。`nervewiki reindex` 重建一个已有的索引更贵：同样 30 万页约 26 分钟、约 290 GB 的 WAL（每页约 1 MB），重建之后 `page_search` 在磁盘上约是原来的 1.7 倍，空出的地方不还给系统。索引约是正文的 5 倍（笔记本少时更小）：升级时留出约正文 5 倍的空闲磁盘（正文大小：`SELECT pg_size_pretty(sum(byte_size)) FROM page_contents`）；`nervewiki reindex` 前留出至少 `page_search` 现在那么大；有 WAL 归档或复制槽时，归档与 `pg_wal` 要容得下这些 WAL。

- 升级到带 `grep` 的版本（M9/P4，迁移 00046，分词的版本 5）时，迁移只让可能被截掉词的页排队（正文多于 74,000 字节的、有长串或长的 `data:` 串的），服务起来之后由后台任务重建它们的索引，其间它们所在的笔记本答 `building`。迁移读一遍全部活着的正文，在它的事务里锁着 `page_search`，只挡着启动。被选中的页像我们量过的语料（2.72% 的页、正文的约 34%、词的约 19%）时，按词估 30 万页约 5 分钟、约 56 GB 的 WAL；在照那份语料的份额造的 30 万页、200 个笔记本的**合成的**库上，入队 8,149 页、954 MB，迁移共 14.1 秒、WAL 4.6 MB，任务重建它们 97.6 秒、WAL 9.76 GB。其余的页不排队：它们的词在版本 5 下不变，升级之后不必执行 `nervewiki reindex`；**但之后不论为了什么执行的第一次 `nervewiki reindex` 也重建全部仍是版本 4 的搜索的行**（约是整个索引：30 万页约 26 分钟、约 290 GB 的 WAL，同上一条"重建一个已有的索引"），照上一条留出磁盘与 WAL 的地方。回退 00046 再升级时，留着的版本 5 的行里被截过的，`page_search.partial` 读作 false，直到它的页再写；v0.1 里没有读这一列的（`grep` 读的是行里的词）。

- 升级到带 `index` 的版本（M9/P4，迁移 00047）时，迁移给 `page_properties` 加两个存储的生成列（`type` 与 `summary` 的值的开头与长度，`index` 只读它们）与两个部分索引，**重写整张表**：其间这张表的读与写都等（ACCESS EXCLUSIVE 锁），要一份表的副本的磁盘，写约表那么大的 WAL。在我们的测试机上，600 万行（922 MB）约 9 秒、1.06 GB 的 WAL，2.6 GB 的表约 17 秒、2.87 GB；回退（Down）约 1 毫秒。所以**先停旧的服务再执行 `migrate up`**（同上文附件目录一条的"升级时先停旧的再起新的"）：旧的还在跑时，每次读属性、每次保存页都要等完重写，大的实例上会过请求的期限。

- 升级到带 MCP 的版本（M9）时，照下面部署一节的"样例（整个站点）"改代理的配置：`/mcp` 与页面正文的两个写缓冲请求体，附件的上传与导入不缓冲。照之前的样例在 `location /`、`server` 或 `http` 里写的 `proxy_request_buffering off` 只留在上传与导入的 `location` 里，`client_max_body_size 512m` 换成按路由的几个（导入的是 `513m`）。照之前的 README 设了 `client_max_body_size 512m` 的，不改时 `/mcp` 照常能用，但它与正文的写一直不缓冲；留着 nginx 默认的 1m 的，过 1 MiB 的 MCP 调用由 nginx 答 413。

- **迁移与服务分用两个数据库角色时**（表的所有者执行迁移，服务用另一个角色登录，`database.auto_migrate` 关闭），服务需要的权限全部写在 [`deploy/runtime-grants.sql`](deploy/runtime-grants.sql)，授予组角色 `nervewiki_runtime`：业务表逐表的读写（不给 `TRUNCATE`、`REFERENCES`、`TRIGGER` 与 DDL），搜索的两个序列 `page_search_queue_version_seq`（入队时从它取版本）与 `page_search_tags_seq`（建索引的任务第一次写一个笔记本的行时从它取这个笔记本的前缀）的 `USAGE`，River 的表与序列，`river_job` 的 `MAINTAIN`（River 每天用 `REINDEX INDEX CONCURRENTLY` 重建它的索引，需要 PostgreSQL 17 起），导入与后台的恢复、撤销写入的 11 张表的 `MAINTAIN`（这些任务每写 10,000 个节点与结束时对它们 `ANALYZE`；少了它 PostgreSQL 跳过这张表、只发一条 WARNING，任务照常完成，统计等 autovacuum 更新），`page_search` 的 `MAINTAIN`（搜索的建索引任务每写 10,000 行对它 `ANALYZE`），`goose_db_version` 的读（`/readyz` 靠它判断迁移是否执行完）。函数与类型不在文件里，靠 PostgreSQL 默认给 PUBLIC 的权限。
  - 组角色建一次，服务登录的角色加入它：`CREATE ROLE nervewiki_runtime NOLOGIN;`、`CREATE ROLE nervewiki_app LOGIN PASSWORD '…' IN ROLE nervewiki_runtime;`。
  - **每次 `migrate up` 之后、启动服务之前**，以表的所有者执行一次这个文件，例如 `psql -v ON_ERROR_STOP=1 -f deploy/runtime-grants.sql`：新的迁移可能加了表，文件逐个列出；重复执行没有影响。
  - 少了业务表的权限时接口答 500；少了 `goose_db_version` 的读时 `/readyz` 是 503，后台任务一直等着不启动。少了 `river_queue` 的权限时 River 启动失败，服务随之退出（`start the jobs: … permission denied`）；少了 River 其他表的，`/readyz` 仍是 200，会话清理、软删除清理与索引重建因 `permission denied` 失败，只记在日志里（`river_notification` 目前 River 不写，少了它看不出来）。`server/internal/bootstrap/runtime_role_test.go` 以恰好这些权限的角色运行服务与管理命令，`public` 里任何表、视图、序列或函数的权限与文件不符时失败。
  - 索引属于表的所有者：停机打断 River 的索引重建时留下的 `*_ccnew` 索引，服务的角色删不掉，River 此后每天记 WARN `Found reindex artifact`，由表的所有者执行 `DROP INDEX CONCURRENTLY` 删除。

- 在反向代理之后运行时设置 `NWIKI_SERVER__TRUSTED_PROXIES`，否则每个客户端都被当成代理。
  - 搜索的词在请求行里（`GET /api/v0/workspaces/{slug}/search?q=…`），会进反向代理的访问日志（服务自己的访问日志只记路由）：同签名地址，访问日志要照敏感数据保管。
- 反向代理不能缓冲事件流，读超时要长于心跳间隔（`events.heartbeat_interval`，默认 20 秒）。服务端在事件流的答复上设了 `X-Accel-Buffering: no`；只有事件流要它，`/mcp` 是普通的 JSON 答复（下面"反向代理与 MCP"）。下一条的两份样例里，事件流经过的那一部分（nginx 的 `location /` 与 `server` 里共用的几行、Caddy 的最后一个 `handle`）与验证过的配置是同一组指令：帧在写入答复的同一毫秒到达，心跳让连接一直活着（M5/P2）。Caddy 开了 `encode`（`zstd gzip`）也一样：它不压缩 `text/event-stream`，不必排除。nginx 用 HTTP/1.1 连上游；`proxy_read_timeout` 默认 60 秒，长于默认的心跳，调大心跳时一起调大。
- 样例（整个站点）：请求体按路由处理，`/mcp` 与页面正文的两个写缓冲，附件的上传与导入不缓冲（下面两条）。
  - nginx：共用的几行写在 `server` 里，每个 `location` 都继承它们。在某个 `location` 里加 `proxy_set_header` 时，它不再继承 `server` 里的三行 `proxy_set_header`，要在那里一起写上（少了 `X-Forwarded-For`，每个客户端都成了代理的 IP）：

    ```nginx
    server {
        server_name wiki.example.com;
        # listen 443 ssl; ssl_certificate …; ssl_certificate_key …; 照你的部署

        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 60s;

        # 页面、接口与事件流；1m 等于 server.max_body_bytes（默认 1 MiB），改了它时一起改
        location / {
            proxy_pass http://127.0.0.1:8080;
            client_max_body_size 1m;
        }

        # MCP：缓冲请求体，上限是 /mcp 的 37,814,272 字节
        location = /mcp {
            proxy_pass http://127.0.0.1:8080;
            proxy_request_buffering on;  # 显式写：server 或 http 一层的 off 会被继承
            client_max_body_size 37m;
        }

        # 页面正文的两个写：缓冲请求体，上限是它们的 31,522,816 字节
        location ~ ^/api/v0/(pages/[^/]+/content|notebooks/[^/]+/pages)$ {
            proxy_pass http://127.0.0.1:8080;
            proxy_request_buffering on;  # 同上
            client_max_body_size 31m;
        }

        # 附件的上传与导入的 zip：不缓冲；上限是导入包的上限（默认 512 MiB）加 64 KiB 的表单
        location ~ ^/api/v0/notebooks/[^/]+/(assets|imports)$ {
            proxy_pass http://127.0.0.1:8080;
            proxy_request_buffering off;
            client_max_body_size 513m;
        }
    }
    ```

  - Caddy（默认不缓冲请求体，`request_buffers` 只给要缓冲的三个路由；自动的 HTTPS 照 Caddy 的默认）：

    ```caddyfile
    wiki.example.com {
        # /mcp 与页面正文的两个写：缓冲请求体（37MiB 不小于 /mcp 的 37,814,272 字节）
        @buffered path_regexp ^/(mcp|api/v0/pages/[^/]+/content|api/v0/notebooks/[^/]+/pages)$
        handle @buffered {
            reverse_proxy 127.0.0.1:8080 {
                request_buffers 37MiB
            }
        }

        # 别的路由（页面、接口、事件流、上传与导入）：不缓冲，Caddy 的默认
        handle {
            reverse_proxy 127.0.0.1:8080
        }
    }
    ```

    经 Caddy 时，服务端在读请求体之前就答的拒绝（503 `server_busy`、401、429）对几 MiB 以上的请求体常常到客户端成了 502、没有 `Retry-After`（我们在 Caddy 2.10.2 上量过这三种，用的是在读请求体之前就答它们的替身，不是 nervewiki 本身；nginx 照原样转发了替身的 503 与它的 `Retry-After`）：网页的大的保存看到的是一次失败、不会等一等再重发；agent 看不到 `Retry-After`（客户端怎样对待 502 未核对）。

- 反向代理与上传（M7 起）：附件的上传（`POST /api/v0/notebooks/{notebook_id}/assets`）与导入的 zip（`POST /api/v0/notebooks/{notebook_id}/imports`）走流式的路由，服务端按字节放宽它们的读写期限（不低于每秒 64 KiB）。
  - 反向代理不要缓冲这两个路由的请求体（nginx 的 `proxy_request_buffering off` 只写在它们的 `location` 里；Caddy 默认不缓冲），否则整个文件传到代理之后服务端才开始读，提前的拒绝（无权、重名、超过上限）也要等传完。别的路由不要关掉缓冲（下一条）。
  - 它们的 `client_max_body_size`（nginx 默认只有 1 MB）不小于导入包的上限（`transfer.import_max_bytes`，默认 512 MiB）加 64 KiB：请求体是 zip 加表单的几个部分。附件的上限（`asset.max_bytes`，默认 50 MiB）加 64 KiB 在它之内（`import_max_bytes` 不小于 `asset.max_bytes`）。改了 `transfer.import_max_bytes` 的，这一行照它改。nginx 的 `client_body_timeout`、`proxy_send_timeout`、`proxy_read_timeout` 是两次读写之间的间隔，不是整个传输的时长，默认的 60 秒就够；按整个请求计时的代理或负载均衡，要容得下最慢的传输（50 MiB 的附件在每秒 64 KiB 下约 13 分钟，512 MiB 的导入包约 2 小时 17 分钟；`transfer.import_max_bytes` 按 `asset.upload_min_rate` 传完不能超过 3 小时）。
- 反向代理与 MCP（M9 起）：`/mcp` 是普通的请求与答复，每个 POST 一个 JSON 答复，不是 SSE，也不持住连接。
  - 请求体的上限不小于 `/mcp` 的 37,814,272 字节（约 36 MiB；nginx 写 `client_max_body_size 37m`，Caddy 的 `request_buffers 37MiB` 同样不小于它）：一次调用至多 5 MiB 的正文加 1 MiB 的别的参数，每个字节按 JSON 最长的转义算 6 个。
  - 读超时不短于 45 秒：一次调用的工作至多 15 秒（`server.request_timeout`），之前读请求体至多 30 秒（`server.read_timeout`）；改了这两个的，不短于它们的和。nginx 默认的 `proxy_read_timeout 60s` 够；Caddy 默认不限（照 Caddy 的文档）。
  - `Host` 照常转发（nginx 的 `proxy_set_header Host $host`；Caddy 默认就转发）：服务端关掉了 MCP SDK 对不是回环地址的 `Host` 的拒绝（同一台机器上的代理转来的 `Host` 是对外的名称），由 Bearer 令牌把关。
  - **`/mcp` 与页面正文的两个写（`PUT /api/v0/pages/{page_id}/content`、`POST /api/v0/notebooks/{notebook_id}/pages`）要缓冲请求体**：大的请求体（MCP 超过 64 KiB 的调用、超过 `server.max_body_bytes` 的正文写）整个服务同时只有两个名额、每个凭证一个（上文"MCP"的"传输"与"接入 agent（MCP）"），名额从读请求体之前占到答完；代理收完请求体再一次转给服务，慢的发送者就占不住名额。上面的两份样例都这样做。缓冲的请求体在服务端认证之前就收下了，放在代理那里：nginx 把过了 `client_body_buffer_size` 的写到 `client_body_temp_path` 的磁盘上（照 nginx 的文档），Caddy 放在内存里（我们在 Caddy 2.10.2 上量过：收着一个 30 MiB 的请求体时代理的内存涨了约 66 MB，一个接近 `/mcp` 上限的约 116 MB，没有写临时文件），所以几个慢的发送者不必有有效的令牌，就能让代理各占至多约 37 MiB 的磁盘（nginx）或一百多 MB 的内存（Caddy）：给代理留出这些地方。
  - 不缓冲时（代理关掉了缓冲，或直接暴露服务）：两个慢的发送者（两个令牌，或两个登录的会话）各占一个名额至多约 45 秒（请求体至多 `server.read_timeout` 30 秒，之后这次调用至多 `server.request_timeout` 15 秒），其间别人的大的写等 2 秒后答 503 `server_busy`。这样被拒的每个请求，服务端记一行 info 的日志 `large request bodies are saturated`（名额只是满着、没有人被拒时不记），`holders` 是这时持着名额的凭证（`pat:<令牌 id>` 或 `session:<会话 id>`）与路由：由 `api_tokens`（PAT）或 `auth_sessions`（会话）那一行的 `user_id` 找到账户，请他在"访问令牌"里撤销那个令牌，或以 `nervewiki users reset-password` 撤销这个账户的全部会话与 PAT（上文"管理员命令"）。
- 内存：页面正文的解析预算（`page.parse_budget_bytes`，默认 8 MiB）最坏时约占 2.4 GB，见上文"页面"的"解析预算"。大的请求体（MCP 超过 64 KiB 的调用、超过 1 MiB 的正文写）整个服务同时至多两个，各约 +0.5 GB，合起来约 1 GB（M9/P2 实测两个加一个等着的 +680 到 +961 MiB，见上文"MCP"的"传输"）；请求体的 JSON 值有上限（`/mcp` 10,000 个，REST 1,000 个），所以许多小值的请求体不比一个长字符串贵。内存小的机器调小预算（至少 5 MiB，最坏约 1.5 GB），并用 `GOMEMLIMIT` 给运行时一个略低于容器上限的目标。

- 数据库必须以 builtin provider 的 `C.UTF-8` 初始化，否则服务拒绝启动，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。
- 探针：存活用 `GET /healthz`（不访问任何依赖），就绪用 `GET /readyz`（数据库可用、迁移已执行完）。镜像里没有 shell 与 curl，所以没有写 `HEALTHCHECK`，由编排系统探测。
- 后台任务（River：每小时一次的过期会话清理 `auth.session_cleanup_interval`，每小时一次的软删除清理 `jobs.purge_interval`，每 10 分钟一次的过期编辑会话清理 `page.edit_session_cleanup_interval`，每天一次的附件目录清扫，导入导出的收拾、到期与清扫（见上文"导入与导出"），每 5 秒一次的搜索的建索引 `page.search_index_interval`（见上文"搜索"））随 `serve` 运行，表在同一条迁移链上。关闭自动迁移时，服务在迁移执行完之前不启动后台任务，迁移之后自动启动，不必重启。River 从连接池里借走一个连接专门监听通知，事件流也借走一个（`LISTEN nwiki_events`，断开后自动重连），数据库要为每个实例多留两个连接（`database.max_conns` + 2）。运行中的导出与导入各占一个连接，搜索的建索引任务运行时也占一个，所以它们合起来至多 `database.max_conns` 的一半（默认的配置下 `max_conns` 至少 6）。搜索的建索引任务被不干净地停下（OOM、`docker kill`、宽限期太短）留在 River 里"运行中"的那一行，下一次 `serve` 启动时丢掉（WARN `jobs left running by a stopped server discarded`）。
- 停止时发 SIGTERM：服务停止接收新连接，关闭开着的事件流，等正在处理的请求结束（最多 `server.shutdown_timeout`，默认 20 秒），关闭接收通知的连接（最多 2 秒），再等正在执行的后台任务（最多 `jobs.shutdown_timeout`，默认 10 秒，之后取消它们，再宽限 1 秒），最后关闭连接池（最多 5 秒）后退出。停机的宽限期要比这些之和长：`docker stop` 默认只等 10 秒，用 `docker stop -t 40`。启动之后不久就停止时（重启循环、端到端测试），River 通常记一条 ERROR `maintenance.PeriodicJobEnqueuer: Error starting transaction`（`context canceled`，它启动时的定时任务入队被停机打断），退出码仍是 0；运行了一段时间的服务停止时一般没有。

## Markdown 样例集

`tools/md-fixtures/` 定义了 Markdown 的提取、渲染与改写规则，是服务端实现的验收标准。样例的输入逐字节有意义（CRLF、BOM、行尾空白），`.gitattributes` 与 `.editorconfig` 已经禁止工具改动它们。修改规则前先读它的 [README](tools/md-fixtures/README.md)。`cases/` 是提取与渲染的样例，`resolve/` 是解析，`rename/` 是改名、移动时的改写，`render/` 是阅读视图里块与换行的显示；`node tools/md-fixtures/check.mjs` 自检样例，`obsidian/` 下的五个脚本（解析、改名、渲染、导出的库各一个，加上共用的 `verify.mjs`）在隔离的数据目录里启动 Obsidian 核对（不碰本机已有的库）。
