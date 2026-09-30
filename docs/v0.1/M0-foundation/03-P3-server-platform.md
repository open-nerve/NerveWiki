# M0/P3 服务端平台层：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M0/P3 服务端平台层 |
| 状态 | 进行中 |
| 基线 | `9d29741`（P2 完成：工具链、门禁、持续集成；Go 代码只有 `buildinfo`） |
| 上级文档 | [M0 总设计](00-M0-design.md) 第 7 节 |

---

## 1. 基线

P2 留下的：
- `server` 模块（Go 1.27.1），只有 `internal/platform/buildinfo`；
- golangci-lint、`make check`，以及 `server`、`web` 两个持续集成任务；
- 开发用 compose（`postgres:18.6-trixie`，builtin `C.UTF-8`）。

前序 Phase 对本 Phase 的要求（M0 总设计"前序 Phase 对后续 Phase 的要求"）：

1. 第一条迁移 `CREATE EXTENSION pg_trgm`。
2. 迁移之后做数据库 locale 自检，不满足就拒绝启动，并有集成测试，包括 `LC_CTYPE 'C'` 的反例库。
3. `pgtest` 使用与开发库相同的镜像和建库参数。
4. HTTP 平台层支持长连接路由：
   - 不受请求期限约束，并在连接上解除写超时；
   - 用测试路由验证豁免与不豁免两种行为。
5. 环境变量覆盖只处理带 `__` 的 `NWIKI_` 变量，并保留对应测试。
6. 全局可变状态的检查中，`buildinfo.version` 是例外。
7. linter 集合最多加 `errorlint`。

## 2. 目标与范围

**目标**：一个能启动、能迁移、能自检、能优雅停机的 `nervewiki`。平台层各包就位，依赖方向由架构测试守住，之后的 Phase 与 M 只往上加模块。

**做**：
- 平台层：
  - `config`：分层加载与校验，敏感值在日志中脱敏；
  - `logging`、`clock`；
  - `postgres`：连接池、事务管理器、迁移器、数据库自检，以及测试工具 `pgtest`；
  - `httpserver`：服务生命周期、全局中间件链、路由与健康检查、problem+json、长连接路由的支持。
- 迁移：`migrations` 包与第一条迁移（pg_trgm）。
- 共享内核 `shared`：错误模型与 `TxManager` 端口。
- 组合根 `bootstrap`：`serve` 与 `migrate` 的装配、启动与停机顺序。
- 命令行 `cmd/nervewiki`：`serve`、`migrate up|down|status`、`version`。
- 架构测试 `archtest`；`gochecknoglobals`、`errorlint` 两个 linter。
- `make run`；README 的运行说明；持续集成跑集成测试（需要 Docker）。

**不做**，以及与 M0 总设计的差异（本 Phase 完成时同步修订 00 号文档）：

| 内容 | 去处 | 理由 |
|---|---|---|
| 接口操作的逐路由中间件（请求期限、请求体上限）、`APIErrors`，以及配置项 `server.request_timeout`、`server.max_body_bytes` | P4 | 它们只服务于生成的接口代码，P4 才有第一个接口操作。P3 的长连接支持与它们互不依赖 |
| `webui`（内嵌前端、CSP、前端路由兜底） | P5 | 它托管前端的构建产物，并按 `index.html` 生成 CSP；P5 才有前端 |
| 客户端 IP、`server.trusted_proxies`、限流、认证、后台任务，以及对应的配置节 | M1 | 第一个用到它们的 M |
| "非 prod 却对外监听"的告警 | M1 | Nerve 告警的是开放注册与临时签名密钥，M0 没有这两样 |

## 3. 设计

### 3.1 包与职责

| 包 | 职责 | 来源 |
|---|---|---|
| `internal/platform/config` | 分层加载（内置 `config.yaml` → `config.<env>.yaml` → `$NWIKI_CONFIG_DIR` → 个人覆盖文件 → 环境变量）。严格解码：未知键、空值、越界数字、裸数字的时长都报错。一次报告所有无效键。日志中的数据库地址整体脱敏 | Nerve，裁剪到 `server`、`database`、`log` 三节 |
| `configs`（`server/configs`） | 内置配置文件与 `embed` | Nerve，改名、裁剪 |
| `internal/platform/logging` | 按配置建 `slog.Logger`（text / json），不碰默认 logger | Nerve |
| `internal/platform/clock` | 系统时钟：UTC，截到微秒 | Nerve |
| `internal/platform/postgres` | 连接池（`timestamptz` 按 UTC 扫描；地址出错时不回显原文）；事务管理器（COMMIT、ROLLBACK 脱离请求的取消、自带期限）；goose 迁移器；**数据库自检**（新写，3.3） | Nerve + 新写 |
| `internal/platform/postgres/pgtest` | 集成测试的数据库：每个测试二进制一个容器，模板库复制出每个测试的独立库 | Nerve；镜像与建库参数按 P1 结论改 |
| `internal/platform/httpserver` | 服务生命周期（监听、`addr_file`、优雅停机）；全局中间件链（请求 ID → recover → 访问日志 → 安全头）；路由（`/healthz`、`/readyz`、`/api/` 兜底 404）；problem+json；**长连接路由**（新写，3.4） | Nerve，裁剪 + 新写 |
| `migrations`（`server/migrations`） | 内嵌的 SQL 迁移；文件名 `NNNNN_<归属>_<说明>.sql` | Nerve 的机制 + 新的第一条迁移 |
| `internal/shared` | 共享内核：错误模型（`Kind` → HTTP 状态、平台错误码、字段错误码）与 `TxManager` 端口；只依赖标准库 | Nerve，字段错误码裁剪到通用的几种 |
| `internal/bootstrap` | 唯一的组合根：装配、`Serve`、`Migrate*`；平台与共享内核在这里按结构对接（编译期断言） | Nerve 的骨架 |
| `cmd/nervewiki` | cobra 命令行；信号处理：第一次信号优雅停机，第二次立即退出 | Nerve，改名、裁剪 |
| `internal/archtest` | 依赖方向与模块边界的架构测试（3.6） | Nerve，规则按本项目调整；sqlc 相关随 M1 |

### 3.2 依赖方向

```
cmd/nervewiki ──► bootstrap ──► platform/* ,  shared ,  migrations ,  (P4 起) modules/*
                 configs ──► （只有 embed）
platform/*    ──► 标准库、第三方库；平台包之间互不依赖，config 除外
shared        ──► 只有标准库（不含 net/http、database/sql）
```

平台层不导入 `shared`。两者在 `bootstrap` 里按结构对接，并用编译期断言固定下来：`postgres.TxManager` 满足 `shared.TxManager`，`*shared.Error` 满足 `httpserver.ProblemError`（P4 用到）。

### 3.3 数据库

- **连接池**：`database.url`、`database.max_conns`；惰性连接，数据库不可达时在第一次使用（例如 `/readyz`）暴露。
- **事务管理器**：语句随调用方的 context 取消；COMMIT、ROLLBACK 在 `context.WithoutCancel` 下执行，受 `database.commit_timeout` 约束。嵌套调用加入同一个事务；panic 时回滚。
- **迁移**：goose 的 Provider，不用它的全局注册表。`serve` 在 `database.auto_migrate` 为真时（dev、test 默认；prod 关闭）先迁移。第一条迁移 `00001_platform_pg_trgm.sql`：`CREATE EXTENSION IF NOT EXISTS pg_trgm`。它的归属是 `platform`，因为扩展是数据库能力，不属于任何模块的表。
- **数据库自检**（`postgres.CheckDatabase`）：
  - `serve` 在迁移之后、开始服务之前执行，`migrate up` 在迁移之后执行。不满足就返回错误，进程拒绝启动；错误信息里给出正确的建库命令。
  - 检查项（总体设计 7.1）：
    - 编码为 `UTF8`；
    - `datlocprovider = 'b'`；
    - `datctype = 'C.UTF-8'`，`datcollate = 'C.UTF-8'`；
    - pg_trgm 已安装时，`show_trgm('中文')` 不为空。
  - pg_trgm 尚未安装，只可能是 prod 没有先执行 `migrate up` 就启动了服务，这时 `/readyz` 因迁移未完成返回 503。
- **`pgtest`**：
  - 镜像 `postgres:18.6-trixie`，以 `POSTGRES_INITDB_ARGS=--locale-provider=builtin --locale=C.UTF-8` 初始化，与开发库完全一致；
  - 每个测试二进制共用一个容器，模板库装好全部迁移，每个测试复制一个独立的库，测试结束时删除；
  - `go test -short` 跳过。

### 3.4 HTTP

- **服务生命周期**（`httpserver.Server`）：
  - 连接上的读写有上限：`read_header_timeout`、`read_timeout`、`write_timeout`，空闲连接固定 2 分钟。
  - `Serve(ctx)`：ctx 结束时停止接收新连接，给正在处理的请求 `shutdown_timeout` 完成；到期后强制关闭并返回错误。
  - `server.addr_file` 非空时，监听成功后原子地写入实际地址（P6 的端到端测试监听 `:0`）。
- **全局中间件链**（所有路由）：请求 ID（接受安全的 `X-Request-Id`，否则生成 UUIDv7）→ recover（panic 转成 500 problem，丢弃 handler 设过的头；响应已开始时中断连接）→ 访问日志（健康检查降为 debug 级）→ 安全头（`nosniff`、`Referrer-Policy`、`X-Frame-Options: DENY`；`/api/` 下加 `Cache-Control: no-store`）。
- **路由**：
  - `GET /healthz`：只要进程在就是 200，不碰任何依赖。
  - `GET /readyz`：依次执行检查（数据库 `Ping`、迁移已是最新），整体限时 2 秒，第一个失败返回 503 `not_ready`。
  - `/api/` 下没有被模块认领的路径：404 problem+json。
  - 路由器记录注册过的全部 pattern，P4 起用于与接口描述对照。
- **长连接路由**（新写，P1 实验 ③⑤ 的结论）：`httpserver.LongLived(logger, h)` 包装一个会长时间保持响应的 handler（M5 的 SSE、M9 的 MCP 流），做两件事：
  1. 用 `http.ResponseController` 解除这条连接的读、写期限。否则写期限到期后，写入全部失败；读期限到期时，net/http 在后台读连接会出错，取消请求的 context。
  2. 服务开始停机时，取消 handler 的 context。`http.Server.Shutdown` 会一直等连接变为空闲，而长连接永远不会空闲；通知它们主动结束，停机才不会拖到 `shutdown_timeout` 再强制关闭。

  实现方式：`Server` 用 `http.Server.BaseContext` 把一个"开始停机"的信号放进每个请求的 context，用 `RegisterOnShutdown` 在停机开始时发出它。`LongLived` 从请求 context 中取这个信号，所以路由注册不必依赖 `Server` 实例，也不影响普通请求：普通请求在停机时照常完成。

  请求期限（P4 起）只挂在接口操作的逐路由中间件上，长连接路由不经过它们，天然豁免。认证怎样与长连接组合，由 M1、M5 决定。
- **problem+json**：`Problem{status, code, title, detail, errors}`，平台错误码 `bad_request`、`not_found`、`internal_error`、`not_ready` 等（总体设计 6.1）。

### 3.5 组合根与命令

- `nervewiki serve`：
  1. 加载配置（失败时列出全部无效键），建 logger，记录生效的配置（已脱敏）；
  2. 建连接池，记录连接目标（主机、端口、库名、用户，取自 pgx 自己的解析，不含密码）；建迁移器；
  3. `auto_migrate` 时迁移，逐条记录；
  4. 数据库自检；
  5. 在 `server.addr` 上服务，直到收到 SIGINT 或 SIGTERM。

  停机顺序：HTTP（长连接先收到停机信号），然后迁移器，最后连接池。连接池的等待有上限：handler 若忽略了自己的 context，可能还占着连接。启动失败的错误同时写成一条结构化日志。
- `nervewiki migrate up|down|status`：逐条打印；`up` 之后执行数据库自检。
- `nervewiki version`：版本号、提交、提交时间、工作区是否有改动。
- 退出码：成功 0，任何错误 1，错误以 `nervewiki: <原因>` 打印到 stderr。

### 3.6 架构测试与静态检查

架构测试按规则判定导入图上的每一条边。每条规则先用构造的边做单元测试，再作用于真实的导入图：

1. 模块内的层次向内依赖：adapter → app → domain；
2. domain、app 只依赖标准库（不含 `net/http`、`database/sql`）、本模块的内层和 `internal/shared`；
3. 模块之间互不导入；
4. 平台层不导入模块、`bootstrap`、`internal/shared`；
5. 只有 `bootstrap` 导入模块；
6. 生成的代码只被它所属的适配器导入；
7. 平台包之间互不导入，`config` 除外；
8. 测试辅助包（`pgtest`）只被测试导入；
9. 模块内的包只能在 domain、app、adapter 或模块根目录；
10. `internal/shared` 只依赖标准库（不含 `net/http`、`database/sql`）。

另有两个测试检查传递依赖：
- `nervewiki` 二进制不链接测试用的库（testcontainers 等）；
- `internal/shared` 的传递依赖里没有基础设施。

规则 1、2、3、5、6、9 在 P4 有了第一个模块之后才有真实的边，现在由构造的边覆盖。

静态检查新增两项：
- `gochecknoglobals`：落实总体设计 8.1 的"禁止全局可变状态"。例外逐个用带理由的 `//nolint` 标出：`buildinfo.version` 需要 `-ldflags -X` 注入；`pgtest` 的共享容器本来就是"每个测试二进制一个"。
- `errorlint`：错误要用 `errors.Is` / `errors.As` 判断，不能直接比较或断言类型。

### 3.7 配置项（M0）

```yaml
server:
  addr: ":8080"               # dev：127.0.0.1:8080
  read_header_timeout: 5s
  read_timeout: 30s
  write_timeout: 60s
  shutdown_timeout: 20s
  addr_file: ""
database:
  url: ""                     # dev 写在 config.dev.yaml；test、prod 由 NWIKI_DATABASE__URL 提供
  max_conns: 10
  auto_migrate: true          # prod：false，先执行 nervewiki migrate up
  commit_timeout: 2s
log:
  level: info                 # dev：debug；test：warn
  format: json                # dev、test：text
```

控制变量 `NWIKI_ENV`（dev / test / prod，默认 dev）与 `NWIKI_CONFIG_DIR` 不是配置键。个人覆盖文件 `server/configs/config.local.yaml` 只在 dev 生效。

## 4. 实施步骤

在分支 `m0-p3-server-platform` 上，五个 Step 依次进行，每个 Step 结束时 `make check` 为绿。Step 的边界清楚，都直接写计划（`plans/`），不另写 spec。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `config`、`configs`、`logging`、`clock` | [P3-S1-config-logging.md](plans/P3-S1-config-logging.md) |
| S2 | `postgres`（连接池、事务、迁移器、数据库自检）、`pgtest`、`migrations` 与第一条迁移 | [P3-S2-postgres.md](plans/P3-S2-postgres.md) |
| S3 | `httpserver`（生命周期、中间件链、路由与健康检查、problem、长连接路由） | [P3-S3-httpserver.md](plans/P3-S3-httpserver.md) |
| S4 | `shared`、`bootstrap`、`cmd/nervewiki`；`make run` | [P3-S4-bootstrap-cli.md](plans/P3-S4-bootstrap-cli.md) |
| S5 | `archtest`；`gochecknoglobals`、`errorlint`；README 与持续集成 | [P3-S5-archtest-gates.md](plans/P3-S5-archtest-gates.md) |

之后是反向对照、独立审查、修复、合并。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 配置：分层顺序、环境变量映射（跳过不带 `__` 的变量）、各类严格解码错误、一次报告全部无效键、日志脱敏；logging；clock；中间件：请求 ID 的接受与生成、recover 的头处理与已开始响应的中断、访问日志的级别与状态；路由：健康检查、就绪检查的顺序与超时、`/api/` 兜底；problem；`addr_file` 的原子写入 |
| 集成（testcontainers） | 连接池与 UTC 扫描；事务：提交、回滚、嵌套加入、panic 回滚、请求取消后仍能提交；迁移器：up、down、status、部分失败；数据库自检：正确的库通过，`LC_CTYPE 'C'`、libc provider 的库各报出对应问题；`serve` 的完整启动与停机；`migrate` 命令 |
| 服务行为（真实监听） | 普通路由在 `write_timeout` 之后写不出去；长连接路由在读写期限之后仍持续推送；停机时长连接的 context 被取消、`Serve` 很快返回，而同时进行的普通请求照常完成 |
| 架构 | 3.6 的规则与传递依赖检查；规则的构造用例 |
| 命令行 | 参数错误、未知子命令失败；`version` 的输出；配置错误的退出码与信息 |

反向对照（验证后撤销）：
- 平台包导入 `shared`、平台包互相导入 → 架构测试失败；
- 新增一个包级变量 → `gochecknoglobals` 失败；
- 用 `==` 比较错误 → `errorlint` 失败；
- 去掉 `LongLived` 的解除期限 → 长连接测试失败；
- 去掉停机信号 → 停机测试超时。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check` 本地与持续集成为绿（持续集成的 `server` 任务运行集成测试）。
- 本地 `make dev-db` + `make run`：`/healthz`、`/readyz` 为 200；停掉开发库后 `/readyz` 为 503；Ctrl-C 优雅退出。
- 审查完成（`reviews/P3-server-platform-review.md`），发现的问题已修复。
- M0 总设计按第 2 节的差异修订，进度表更新。

## 7. 结果

（完成后补写）
