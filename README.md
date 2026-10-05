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

`serve` 启动时要连上数据库（最多等 10 秒，连不上就退出），按配置执行迁移（dev、test 默认执行，prod 默认不执行），然后自检数据库的编码与 locale，不满足就拒绝启动并给出建库命令。`GET /healthz` 表示进程存活；`GET /readyz` 在数据库可用、迁移已是最新时返回 200，否则 503。`GET /api/v0/instance` 返回产品名、版本、提交与接口版本，以及是否开放注册（`signup_enabled`）、是否开放创建工作区（`workspace_creation_enabled`）；`/api/` 下没有的路径返回 404 problem+json。

其他命令在 `server/` 下用 `go run ./cmd/nervewiki <命令>` 执行：

| 命令             | 作用                                                           |
| ---------------- | -------------------------------------------------------------- |
| `serve`          | 运行 HTTP 服务，直到收到 SIGINT 或 SIGTERM；第二次信号立即退出 |
| `migrate up`     | 执行全部待执行的迁移，然后自检数据库                           |
| `migrate down`   | 回滚最近一条迁移                                               |
| `migrate status` | 列出迁移及其状态                                               |
| `users …`        | 服务器管理员的账户命令，见下文"账户与认证"                     |
| `workspaces …`   | 服务器管理员的工作区命令，见下文"账户与认证"                   |
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

- **签名私钥**：`auth.jwt.private_key_file`，PKCS#8 PEM 的 Ed25519 私钥，用 `openssl genpkey -algorithm ed25519 -out jwt.pem` 生成。prod 必须提供，缺了拒绝启动；dev、test 不提供时每次启动生成临时密钥，重启后已签发的令牌与邀请链接全部失效。刷新令牌与邀请令牌的 MAC 密钥都由它派生（HKDF，各用各的 info）：换私钥之后，待接受的邀请链接全部失效，要重新邀请。日志只记是否设置，不记路径。
- **反向代理**：`server.trusted_proxies` 列出代理的 CIDR（环境变量用逗号分隔，例如 `NWIKI_SERVER__TRUSTED_PROXIES=10.0.0.0/8`）。只有来自它们的 `X-Forwarded-For` 被采信，代理写入的必须是不带端口的 IP；会话记录的就是这样认出的客户端 IP。配置不对时服务各告警一次。
- 非 prod 的服务监听在回环地址之外时，启动时告警：这多半是忘了设 `NWIKI_ENV=prod` 的部署，注册开放、签名密钥是临时的。

### 工作区

- **创建**：`workspace.creation_enabled`，默认开启：每个账户都能创建工作区，创建者是它的管理员（`GET /api/v0/instance` 的 `workspace_creation_enabled` 告诉客户端）。关闭后创建答 403 `workspace.creation_disabled`。
- **slug**：工作区的地址段，1–48 个 a–z、0–9、`_`、`-`，创建后不能改；站点的顶层路径与留作以后用的名字不能用，名单在 `server/internal/modules/workspace/domain/reserved_slugs.txt`。
- **成员**：角色是 admin、member、guest。管理员改别人的角色、移出成员，不能改或移出自己；成员可以离开，唯一的管理员不能（`workspace.sole_admin`），先让别人成为管理员，或者删除工作区；还有别的成员的笔记本的唯一管理员也不能离开（`notebook.sole_admin`，见"笔记本"）。移出不因笔记本被拒：他独自管理的笔记本成为无主。成员列表对访客隐藏邮箱。
- **停用账户**（自助停用与 `users deactivate` 相同）：他是某个还有别的有效成员的工作区唯一的管理员时，停用被拒（409 `workspace.sole_admin`，原因里列出这些工作区的 slug），先让那里的另一位成员成为管理员；只有他一人的工作区不挡停用。他是还有别的成员的笔记本唯一的管理员时同样被拒（409 `notebook.sole_admin`，见"笔记本"）：先在那些笔记本里让另一位成员成为管理员，或者删除它们；他自己做不了时（如服务器管理员用 `users deactivate`），由工作区的管理员把他移出那个工作区，移出不被拒，那些笔记本成为无主。停用结束他全部的成员关系（工作区与笔记本，他独自管理的笔记本成为无主），并删除这些工作区里发给他邮箱的待接受邀请。恢复：`users activate`，再对每个工作区 `workspaces reactivate-member`，或者由工作区的管理员重新邀请他。唯一的管理员独自停用之后，工作区没有管理员：接受成员、访客邀请答 409 `workspace.no_admin`，`reactivate-member` 恢复成员、访客也被拒（`The workspace has no admin`）；先恢复那位管理员，或者有人接受一份管理员邀请，别人才能加入。注册关闭时凭这样的邀请照样能注册，邀请留着，等工作区重新有管理员之后再接受。
- **软删除与清理**：删除工作区是软删除，连同它的成员、邀请、笔记本、笔记本的成员、页面与审计记录；撤回、接受的邀请也是软删除。超过 `jobs.purge_retention`（默认 1440 小时，即 60 天）的，由后台任务物理删除，服务启动时一次，之后每 `jobs.purge_interval`（默认 1 小时）一次；多个实例时只有一个执行。保留期内运维可以从数据库恢复。
- **邀请**：管理员按邮箱邀请（`POST /api/v0/workspaces/{slug}/invitations`），把邀请的 id 与令牌（`nwk_inv_` 开头）发给对方；一个工作区里一个邮箱至多一份待接受的邀请，有效成员的邮箱不能邀请。令牌是 id 的 MAC，不存库，管理员随时可以在邀请列表里再看到它。任何拿到链接的人都能预览（工作区的名称与 slug、角色，不含邮箱）；接受要求用被邀请的邮箱登录，接受之后成为成员（已结束的成员关系恢复，保留第一次加入的时刻，他名下的无主笔记本归还给他；已是成员的角色不变）。预览与接受都把令牌放在请求体里，不放在 URL 中；链接由页面拼出（M2/P5、P6），令牌放在 URL 片段（`#`）里，浏览器不把片段发给服务器。撤回邀请（`DELETE /api/v0/workspace-invitations/{workspace_invitation_id}`）、成员关系结束（对他邮箱的待接受邀请一并删除）、删除工作区之后，链接答 404 `workspace.invitation_not_found`。

### 笔记本

- **建立与开放程度**：工作区的管理员与成员建笔记本（`POST /api/v0/workspaces/{slug}/notebooks`），建的人是它的管理员；访客建答 403 `forbidden`。`workspace_access` 是它对工作区的开放程度：`none`（私密，只有它的成员看得到，工作区的管理员也看不到）、`viewer`、`editor`（工作区的管理员与成员默认是阅读者、编辑者；访客没有默认角色，只能被加为成员）。
- **有效角色**：笔记本的角色是 admin、editor、reader；一个人的有效角色是显式角色与默认角色中较高的一个，`Notebook` 的 `role` 是调用者的有效角色。看不到的笔记本按 id 答 404 `notebook.not_found`，与不存在相同。
- **名称**：与页面标题同一规则（`shared.CheckTitle`）：去掉首尾空白、NFC 规范化之后 1–255 字节，不含 `/ \ : * ? " < > | # ^ [ ]` 与控制字符，不以 `.` 开头或结尾，不是 Windows 保留名；不要求唯一。不合规则是 `name` 的字段错误（422）。
- **成员**：笔记本的管理员从工作区的有效成员中添加成员（访客也可以）、改角色、移出，不能改或移出自己（409 `notebook.own_membership`）；添加不是工作区有效成员的人是 `user_id: not_allowed`，已是成员的是 `user_id: duplicate`，成员关系已结束的（离开、被移出，或随工作区的成员关系结束）恢复原来那一行，取这次给的角色，保留第一次加入的时刻。成员可以离开，唯一的管理员不能，哪怕只有他一人（409 `notebook.sole_admin`）：先让别人成为管理员，或者删除笔记本。改名、改开放程度（`PATCH /api/v0/notebooks/{notebook_id}`）与删除（`DELETE` 同一地址）也只有笔记本的管理员能做，编辑者、阅读者答 403 `forbidden`；删除是软删除，连同它的成员行与页面，页面的编辑会话随之删除。
- **离开工作区、停用与移出**：离开工作区或停用账户（自助与 `users deactivate`）时，他是某个还有别的有效显式成员的笔记本唯一的管理员，就被拒（409 `notebook.sole_admin`）；原因只给这些笔记本所在工作区的 slug 与数量（`… (1 in acme)`），不给名称。只靠 `workspace_access` 使用它的人不算别的成员。工作区自己的规则（`workspace.sole_admin`）先判断。成员关系结束时，他的笔记本成员关系一并结束；他是唯一管理员的笔记本成为**无主**：剩下的成员与按开放程度看得到它的人照常使用，只是没有人能管理它的设置与成员。工作区管理员移出成员不被拒，他独自管理的笔记本同样成为无主。
- **无主笔记本**：只有工作区的管理员看得到、处理得了：`GET /api/v0/workspaces/{slug}/ownerless-notebooks` 列出（原所有者、成为无主的时刻、最后活动与大小：笔记本自己最后一次修改与它的页面最后一次写入（新建、改名、移动、删除、写正文）中较晚的时刻，没删除的页面正文的总字节数），`POST /api/v0/ownerless-notebooks/{notebook_id}/take-over` 接管（成为它的管理员，开放程度不变），`DELETE /api/v0/ownerless-notebooks/{notebook_id}` 删除。工作区的成员与访客读清单答 403；按 id 的两个操作对管理员之外的任何人答 404 `notebook.not_found`，与不存在、不是无主的相同。原所有者经接受邀请或 `workspaces reactivate-member` 回到工作区时，他名下还没被接管或删除的无主笔记本归还给他，以访客身份回来也一样。页面上是工作区设置的"无主笔记本"一页，管理员的首页另有提醒。
- **审计记录**：接管、删除、归还三种，记执行者、时刻、笔记本名称的快照与原所有者：`GET /api/v0/workspaces/{slug}/notebook-audit-events?limit=&cursor=`，只给工作区的管理员，新的在前。`limit` 1–100、默认 50；下一页用答复的 `next_cursor`，最后一页它是 `null`；读不出的游标（不是服务器写出的拼法；游标不签名，不防改写）答 400 `bad_request`，先于其余判断。审计记录比笔记本活得久，随工作区删除与清理。

### 页面

- **树**：一个笔记本的页面是一棵树，`GET /api/v0/notebooks/{notebook_id}/nodes` 一次列出整棵（父页在子页之前）。新建（`POST /api/v0/notebooks/{notebook_id}/pages`，可以带正文）、改名（`PATCH /api/v0/nodes/{node_id}`）、移动（`POST /api/v0/nodes/{node_id}/move`：换父页或在兄弟之间排序，连同它下面的页）、删除（`DELETE /api/v0/nodes/{node_id}`：连同它下面的页一起软删除，保留期过后由清理任务物理删除，见"工作区"的"软删除与清理"）；`GET /api/v0/pages/{page_id}` 读一页与它的祖先。位置用 `after_id`：排在哪个兄弟之后，`null` 排最前，不给排最后。标题与笔记本的名称同一规则；同一父页下（根下也一样）按标题键唯一（NFC、Unicode 大小写折叠、再 NFC），重名答 409 `page.title_taken`；树至多十层（`page.too_deep`），不能移到自己下面（`page.cycle`），也不能移到别的笔记本。笔记本的编辑者与管理员能写，阅读者只读（403 `forbidden`）；看不到的页答 404 `page.not_found`，与不存在相同。
- **正文**：`GET /api/v0/pages/{page_id}/content` 读出正文、它的版本 `revision` 与 `content_hash`；`PUT` 同一地址写入 `{content, base_revision, edit_session_id?}`，逐字节保存（换行写法、BOM、空白都原样），至多 5 MiB（5,242,880 字节）、不含 NUL（`content` 的字段错误，422）。`base_revision` 不是当前版本答 409 `page.revision_mismatch`：期间有人写过，重新读出再决定；与当前正文相同的写什么也不改。两个正文路由的请求体上限另算（正文加上 JSON 的转义），期限另加 `server.read_timeout`：5 MiB 的正文在 30 秒内传完约需 1.4 Mbit/s 的上行，更慢的链路调大 `read_timeout` 与 `write_timeout`；请求体没有及时传完答 400 `bad_request`。
- **编辑会话与编辑锁**：编辑器开启会话（`POST /api/v0/pages/{page_id}/edit-sessions`），租约 120 秒，每 20 秒心跳一次续上（`POST /api/v0/edit-sessions/{edit_session_id}/heartbeat`），退出时结束（`DELETE /api/v0/edit-sessions/{edit_session_id}`）。带着同一个会话的写是一个变更集。
  - **锁**：一页活着的会话就是它的编辑锁，同一时刻至多一个。有人持锁时，再开启答 409 `page.locked`，problem 的 `lock` 成员给出页与持锁人（`user_id`、`display_name`），持锁的是自己在别处的会话也一样；不带这个会话的写正文（令牌、别的标签页）也答 `page.locked`；删除一页或它的上级页时，子树里有**别的账户**持锁就答 `page.locked`（按层取第一个），自己的会话随删除结束。新建、改名、移动不受锁限制。
  - **接管**：开启时带 `{"take_over": true}`，先结束自己在这一页的会话（别处的标签页、令牌都算），再开新会话；别人的锁接管不了。被接管的会话再心跳或保存答 409 `page.edit_session_taken_over`。
  - **强制解锁**：笔记本的管理员 `DELETE /api/v0/pages/{page_id}/edit-lock` 结束持锁的会话（没人持锁也答 204）；那个会话再心跳或保存答 409 `page.edit_session_unlocked`，`ended_by` 成员给出解除者。
  - **读锁**：能读这一页的人 `GET /api/v0/pages/{page_id}/edit-lock`，答复 `{holder, expires_in}`：持锁人与租约剩余的秒数（向上取整），没人持锁时两者都是 `null`。
  - 会话过期，或不是写的人在这一页、用同一种客户端（网页，或任一令牌）开的，写答 409 `page.edit_session_ended`，心跳与结束答 404 `page.edit_session_not_found`，编辑器重开一个。被接管、被解锁的会话至少保留一个租约，让原来的标签页得知原因（这期间心跳与保存答原因，结束答 204；会话的主人不再能编辑这个笔记本时，心跳照常答 404 或 403），之后与过期的会话一起删除：这一页下一次开启或强制解锁时，或后台任务每 `page.edit_session_cleanup_interval`（默认 10 分钟）一次；删页、删子树、删笔记本时它们的会话一并删除。
- **阅读视图**：`GET /api/v0/pages/{page_id}/view` 给出渲染好的 HTML 与它所依据的 `revision`：CommonMark 加 GFM（表格、任务项、删除线、自动链接）与脚注，frontmatter 的属性在最前面显示成表格；正文里的 HTML 只留排版用的标签与属性，地址只留本站、http(s) 与 mailto，图片不加载、显示为链接。任务项的复选框都是 `disabled`，`data-task` 是方括号里那个字符在正文里的字节位置。
- **勾选任务项**：`POST /api/v0/pages/{page_id}/toggle-task`，`{base_revision, offset, checked}`：把位置 `offset` 上的那个字节换成 `x` 或空格，其余字节不变，答这一页；权限与写正文相同，有人持锁时同样答 409 `page.locked`。`base_revision` 不是当前版本答 409 `page.revision_mismatch`（位置只在它所依据的版本里有意义，先于 422）；`offset` 上不是任务项答 422（`offset`，`out_of_range`），勾了会让那里不再有任务项（例如 `- [ ]: /u` 勾上之后成了链接引用定义）答 422（`offset`，`not_allowed`）；已经是那个状态的什么也不写，有人持锁时也答 200。
- **解析预算**：服务端同时解析的正文字节数有上限（`page.parse_budget_bytes`，默认 8 MiB，每次至少记 4 KiB），取不到额度的请求最多等 `page.parse_max_wait`（默认 2 秒），然后答 503 `server_busy`（带 `Retry-After`）；写正文、新建带正文的页与阅读视图都经它。最坏的正文解析时约占它字节数 300 倍的内存（默认预算约 2.4 GB），普通的约 40 倍：内存小的机器调小预算（不能小于 5 MiB），并设置 `GOMEMLIMIT`。

### 链接索引

- **索引**：每一页正文里的链接（wikilink、嵌入、Markdown 链接与图片，以及 frontmatter 里的属性链接）、标签、属性与别名，和每条链接解析到的页，随每次写入在同一个事务里更新：新建、改名、移动、删除（连同子页）、写正文、勾选任务项，删除笔记本时一并删除。同一笔记本的索引维护一个接一个进行（索引自己按笔记本的锁），所以同一笔记本的保存在提交处排队。
- **解析**：链接按目标最后一段的标题键找页。以 `.md` 结尾（不分大小写）的，笔记本里有去掉 `.md` 的那个名称的页时就是去掉 `.md` 的，没有时才是标题以 `.md` 结尾的页。`./`、`../` 开头的从出发页所在的文件夹（它的父页）算起；`/` 开头的从根算起；否则先找从根起路径恰好如此的页，再找路径以这几段结尾的页（出发文件夹的子树里的优先，文件夹自己的页也算，再按路径短的，即导出路径的字符数，再按 id，最后这一步时算有歧义）；只有一段的名称最后才找别名。规则与样例在 `tools/md-fixtures/resolve/`，与 Obsidian 1.12.7 核对过；不同的有：别名（Obsidian 不按别名解析），标题键用 Unicode 的大小写折叠（Obsidian 用 `toLowerCase`，`ß` 与 `ss` 它不当作一样），路径一样长时按 id 选（Obsidian 没有稳定的次序），以及三处 Obsidian 按字符串比较路径的地方（这里按整段）。
- **别名与标签**照 Obsidian 的读法：第一个名为 `aliases`、`tags` 的键（ASCII 字母不分大小写），字符串是一个（不按逗号拆开），列表取其中的字符串，去掉首尾空白，空的不算；`alias`、`tag` 不读。标签（frontmatter 的去掉开头的一个 `#`）是 Obsidian 的标签面板计入的：去掉结尾的一个 `/`，不含空白、ASCII 标点（`-`、`_`、`/` 除外）与 U+2000–U+206F、U+2E00–U+2E7F 两段标点，不全是 ASCII 数字。
- 正文里经 `%00` 或 YAML 的转义写出的 U+0000 在索引里记作 U+FFFD；目标、别名、标签的标题键长于 1024 字节（任何标题的键都到不了）时，这条链接解析不到，这个别名、标签不记。
- **`nervewiki reindex [--notebook <id>]`**：从页面重建链接索引，不给 `--notebook` 时逐个重建每个没删除的笔记本，每个一行（页数、链接数、解析不到的数）。升级到带链接索引的版本（M6）之后、或发布说明要求时（提取规则的版本 `indexed_pages.extractor` 变了）执行一次。
  - 每个笔记本一个事务：重建期间这个笔记本的写入在等待；页数上万的笔记本要十几秒，等不到的保存答 500。所以在没人写的时候执行，最好在 `migrate up` 之后、启动服务之前。
  - 它同时按当前的 Unicode 数据重算标题键：同一父页下有两页会撞键时，这个笔记本不重建、什么也不改，标准错误上列出这些页的标题与 id；改掉其中一个标题之后再执行。别的原因重建失败的笔记本同样原样保留，连同原因列在标准错误上。其余笔记本照常，命令最后以退出码 1 结束。

### 事件流

- `GET /api/v0/events` 是一条 SSE（`text/event-stream`），一个账户一条：推送它看得到的笔记本里的变化，只带 id，客户端收到后自己重新取数。访问令牌与 PAT 都可以（`Authorization: Bearer`）；浏览器的 `EventSource` 带不了这个头，用 `fetch` 流式读取。看得到的笔记本与 `GET /api/v0/notebooks/{notebook_id}` 答 200 的相同，在连接建立时算好；帧按提交的次序到达，回滚的事务什么也不发。
- **帧**：`event: <类型>` 与一行 `data: <JSON>`，`data` 带 `workspace_id` 与 `notebook_id`。
  - `hello`：第一帧，`{"heartbeat_seconds"}`；
  - `pages`：一个写入单元，`tree` 说明是否改了树（新建、改名、移动、删除），`pages` 是写了正文的页与新版本 `[{id, revision}]`（新建的页在版本 1）；多于 20 页时是 `null`，当作每一页都写过；
  - `lock`：一页的编辑会话开启或结束，`{page_id, session_id}`，去读锁；接管是先结束、再开启的两帧；
  - `links`：一个写入单元改变了链接索引，`{pages, targets}`：`pages` 是链接改指别的页的页（不含这个单元写了正文的页，它们在 `pages` 帧里），`targets` 是反链变了的页；各自多于 20 页时是 `null`，当作这个笔记本的每一页都变了。`nervewiki reindex` 每个笔记本发一帧，两者都是 `null`；
  - `reset`：`{reason}`，流的最后一帧；
  - 心跳是一行注释 `: heartbeat`。不认识的类型跳过：之后的 M 会加自己的类型。
- **`reset`**：服务端随即关闭连接，客户端重连并整体刷新（树与打开的页），不补发事件。原因：`access`（看得到的笔记本可能变了：工作区或笔记本的成员身份、笔记本的开放程度、停用与恢复）、`notebooks_deleted`（看得到的笔记本被删除，删工作区也是）、`expired`（凭证到期，续期之后重连）、`unauthenticated`（心跳时重新认证失败：撤销、别处退出登录、停用）、`reconnected`（服务端接收通知的连接断开又连上，期间的事件丢了）、`overflow`（客户端读得太慢，缓冲的 64 个事件满了）。因 `access`、`notebooks_deleted`、`reconnected`、`overflow` 而 `reset` 时，之前已经送到这条流的事件先写出；凭证到期或失效时不再写出。流也可能不带 `reset` 就结束：服务停机、心跳时认证服务出错、客户端一个心跳之内收不下一帧，客户端同样重连并刷新。
- **心跳**：每 `events.heartbeat_interval`（默认 20 秒，5–50 秒）一次，同时重新认证凭证，所以撤销与别处的退出登录在一个心跳之内生效。
- 服务刚启动、接收通知的连接还没连上或正在重连时，打开流答 503 `not_ready`（`Retry-After: 1`）；`/readyz` 不看这条连接，这期间实例照常就绪，其余接口不受影响。接收通知的连接安静 30 秒就 ping 一次，悄悄断掉的连接（数据库切换、NAT 忘了它）由此发现并重连。
- 打开一条流在限流中算一次请求，读它看得到的笔记本受 `server.request_timeout` 约束；连着的流不受请求期限与服务端写超时约束，每一帧要在一个心跳之内写出，停机开始时关闭。

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
- **页面**：笔记本的左栏是它的页面树：展开与折叠，拖拽改变位置（或用"移动到…"对话框，键盘可用），新建子页、改名、删除在每一项的菜单里；`Ctrl+O`（macOS 上 `Cmd+O`）快速切换页面。`/:slug/notebooks/:id/pages/:pageId` 是页面：面包屑、标题、阅读视图与子页面列表；阅读视图里代码块的高亮在 Worker 里做，超时就不着色；能写的人在阅读视图里勾选、取消任务项（复选框以它那一项的文字为名称，空格键也可以）。页面树、阅读视图与编辑锁随事件流实时更新：一个浏览器一条流，由一个标签页持有、转给同一登录的别的标签页；别人连续保存时一页的阅读视图至多 5 秒重读一次，隐藏的标签页在重新可见时重读；每次连上都整体刷新。别的标签页或别人删掉的页显示 404，本标签页删掉的去它的父页；正在编辑而有未保存的修改时页面留在原处，直到离开：页面、笔记本、工作区被删（之后又有同名的新工作区也一样），或被移出笔记本、工作区，都一样。
- **编辑**：写者点"编辑"或按 `Ctrl+E`（`Cmd+E`）先拿这一页的编辑锁，在原处换成源码编辑器（CodeMirror，第一次编辑时才下载）。别人正在编辑时页面写着"某某正在编辑这一页"，不能编辑；自己在别处编辑时可以"在这里编辑"接管；笔记本的管理员可以"解除锁定"。停顿约 2 秒自动保存，30 分钟没有输入就保存并退出编辑，关闭标签页时释放锁（释放没发出去时由租约兜底，至多 2 分钟）；编辑中失去锁（被接管、被解除，或过期之后被别人拿到）时编辑器只读，上方说明原因，"回到阅读"离开。`Ctrl+S` 保存，`Ctrl+E` 或"完成"保存之后回到阅读视图；保存的是编辑器里的文字加上原来的换行写法与 BOM，输入法组合中按的保存等组合结束再做。有未保存的修改时去别的页先确认，关标签页由浏览器提醒；退出登录（在哪个标签页都一样）先保存同一登录各标签页未保存的修改、结束它们的编辑，至多等 2 秒再登出。保存时这一页已被别人改过，编辑器上方显示差异，"保留我的"覆盖、"放弃我的"载入现在的正文。编辑模式不进地址：刷新回到阅读视图。
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

- 升级到带链接索引的版本（M6）时，`migrate up` 之后、启动服务之前执行一次 `nervewiki reindex`（见上文"链接索引"），已有页面的链接才进入索引；之后的写入自己维护索引。用两个数据库角色时，先执行授权文件（见下）：

  ```bash
  docker run --rm -e NWIKI_DATABASE__URL=… "${key[@]}" nervewiki:0.1.0 reindex
  ```

- **迁移与服务分用两个数据库角色时**（表的所有者执行迁移，服务用另一个角色登录，`database.auto_migrate` 关闭），服务需要的权限全部写在 [`deploy/runtime-grants.sql`](deploy/runtime-grants.sql)，授予组角色 `nervewiki_runtime`：业务表逐表的读写（不给 `TRUNCATE`、`REFERENCES`、`TRIGGER` 与 DDL），River 的表与序列，`river_job` 的 `MAINTAIN`（River 每天用 `REINDEX INDEX CONCURRENTLY` 重建它的索引，需要 PostgreSQL 17 起），`goose_db_version` 的读（`/readyz` 靠它判断迁移是否执行完）。函数与类型不在文件里，靠 PostgreSQL 默认给 PUBLIC 的权限。
  - 组角色建一次，服务登录的角色加入它：`CREATE ROLE nervewiki_runtime NOLOGIN;`、`CREATE ROLE nervewiki_app LOGIN PASSWORD '…' IN ROLE nervewiki_runtime;`。
  - **每次 `migrate up` 之后、启动服务之前**，以表的所有者执行一次这个文件，例如 `psql -v ON_ERROR_STOP=1 -f deploy/runtime-grants.sql`：新的迁移可能加了表，文件逐个列出；重复执行没有影响。
  - 少了业务表的权限时接口答 500；少了 `goose_db_version` 的读时 `/readyz` 是 503，后台任务一直等着不启动。少了 `river_queue` 的权限时 River 启动失败，服务随之退出（`start the jobs: … permission denied`）；少了 River 其他表的，`/readyz` 仍是 200，会话清理、软删除清理与索引重建因 `permission denied` 失败，只记在日志里（`river_notification` 目前 River 不写，少了它看不出来）。`server/internal/bootstrap/runtime_role_test.go` 以恰好这些权限的角色运行服务与管理命令，`public` 里任何表、视图、序列或函数的权限与文件不符时失败。
  - 索引属于表的所有者：停机打断 River 的索引重建时留下的 `*_ccnew` 索引，服务的角色删不掉，River 此后每天记 WARN `Found reindex artifact`，由表的所有者执行 `DROP INDEX CONCURRENTLY` 删除。

- 在反向代理之后运行时设置 `NWIKI_SERVER__TRUSTED_PROXIES`，否则每个客户端都被当成代理。
- 反向代理不能缓冲事件流，读超时要长于心跳间隔（`events.heartbeat_interval`，默认 20 秒）。服务端在流的答复上设了 `X-Accel-Buffering: no`。下面两份配置都验证过：帧在写入答复的同一毫秒到达，心跳让连接一直活着（M5/P2）。
  - Caddy 的默认配置即可；开了 `encode`（`zstd gzip`）也一样：Caddy 不压缩 `text/event-stream`，不必排除它：

    ```caddyfile
    wiki.example.com {
        reverse_proxy 127.0.0.1:8080
    }
    ```

  - nginx 用 HTTP/1.1 连上游；`proxy_read_timeout` 默认 60 秒，长于默认的心跳，调大心跳时一起调大：

    ```nginx
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 60s;
    }
    ```
- 内存：页面正文的解析预算（`page.parse_budget_bytes`，默认 8 MiB）最坏时约占 2.4 GB，见上文"页面"的"解析预算"。内存小的机器调小预算（至少 5 MiB，最坏约 1.5 GB），并用 `GOMEMLIMIT` 给运行时一个略低于容器上限的目标。

- 数据库必须以 builtin provider 的 `C.UTF-8` 初始化，否则服务拒绝启动，见[总体设计](docs/v0.1/v0.1-design.md) 7.1。
- 探针：存活用 `GET /healthz`（不访问任何依赖），就绪用 `GET /readyz`（数据库可用、迁移已执行完）。镜像里没有 shell 与 curl，所以没有写 `HEALTHCHECK`，由编排系统探测。
- 后台任务（River：每小时一次的过期会话清理 `auth.session_cleanup_interval`，每小时一次的软删除清理 `jobs.purge_interval`，每 10 分钟一次的过期编辑会话清理 `page.edit_session_cleanup_interval`）随 `serve` 运行，表在同一条迁移链上。关闭自动迁移时，服务在迁移执行完之前不启动后台任务，迁移之后自动启动，不必重启。River 从连接池里借走一个连接专门监听通知，事件流也借走一个（`LISTEN nwiki_events`，断开后自动重连），数据库要为每个实例多留两个连接（`database.max_conns` + 2）。
- 停止时发 SIGTERM：服务停止接收新连接，关闭开着的事件流，等正在处理的请求结束（最多 `server.shutdown_timeout`，默认 20 秒），关闭接收通知的连接（最多 2 秒），再等正在执行的后台任务（最多 `jobs.shutdown_timeout`，默认 10 秒，之后取消它们，再宽限 1 秒），最后关闭连接池（最多 5 秒）后退出。停机的宽限期要比这些之和长：`docker stop` 默认只等 10 秒，用 `docker stop -t 40`。启动之后不久就停止时（重启循环、端到端测试），River 通常记一条 ERROR `maintenance.PeriodicJobEnqueuer: Error starting transaction`（`context canceled`，它启动时的定时任务入队被停机打断），退出码仍是 0；运行了一段时间的服务停止时一般没有。

## Markdown 样例集

`tools/md-fixtures/` 定义了 Markdown 的提取与改写规则，是服务端实现的验收标准。样例的输入逐字节有意义（CRLF、BOM、行尾空白），`.gitattributes` 与 `.editorconfig` 已经禁止工具改动它们。修改规则前先读它的 [README](tools/md-fixtures/README.md)。
