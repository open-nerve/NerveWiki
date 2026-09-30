# M1/P4 管理命令与后台任务：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M1/P4 管理命令与后台任务 |
| 状态 | 已完成 |
| 基线 | P3 合并之后的 main |
| 上级文档 | [M1 总设计](00-M1-design.md) 第 1、3、6–8 节；[P3 文档](03-P3-accounts-tokens.md) 3.3、3.6；[总体设计](../v0.1-design.md) 7.1、12.4 |

---

## 1. 基线

P3 留下的：账户与 PAT 的全部接口操作；账户行锁协议（`CredentialLock`，全局加锁顺序 `users → auth_sessions → api_tokens`）；停用的私有步骤 `deactivate(ctx, d)`（否决者 → 写入 → 订阅者），P4 的命令行停用与它共用；`RevokeSessions` 与撤销原因（`password_reset`、`email_changed` 已有常量与 CHECK）；`ErrAccountNotFound`。服务只有 HTTP，没有后台任务；停机顺序是 HTTP → 迁移器 → 连接池。

M0 移交给本 Phase 的（M1 总设计第 7 节）：[P3 平台层](handoffs/M0-P3-platform.md)第 4 项（`platform/jobs`、`jobs` 配置节、停机顺序、River 与业务表同一条迁移链）、第 6 项（命令行的组合与 `composition_test`）。

前面的 Phase 推迟到这里的：
- P3 审查 N6：`identity.New` 约 95 行，加管理用例时把用例的装配抽成辅助函数。
- A11 的启用部分（管理员 `users activate` 之后 PAT 恢复）。

## 2. 目标与范围

**目标**：服务器管理员能在命令行创建账户、重置密码、改邮箱、停用与启用账户，命令只组合连接池与管理用例；服务有了后台任务，过期的会话被定期删除；停机顺序是 HTTP → 后台任务 → 迁移器 → 连接池；部署可以用最小权限的运行时角色。

**做**：
- River：迁移（与业务表同一条链）、`platform/jobs`、`jobs` 配置节；serve 在没有待执行的迁移之后启动后台任务；停机顺序。
- 会话清理任务（`auth.session_cleanup_interval`）。
- identity 的管理用例：`CreateUser`、`ResetPassword`（撤销全部会话与 PAT）、`SetEmail`、按邮箱的停用与启用；`identity.NewAdmin`；与注册共用的"建账户"一步；`New` 的装配拆分。
- 登录在锁下另核对邮箱（3.6）。
- `nervewiki users` 五个命令；密码从终端（不回显）或标准输入读取；命令的错误格式。
- `archtest/composition_test.go`。
- `deploy/runtime-grants.sql` 与它的测试。
- 端到端：A11 的命令行部分、A12、A13；README。

**不做**：前端（P5、P6）；M2 的真实注册者；只投递的 River 客户端（M2 以后第一个由请求投递的任务出现时）；会话之外的清理（撤销的 PAT 与会话保留，理由见 3.5）。

## 3. 设计

### 3.1 文件

```
server/
  go.mod                                          river、riverpgxv5 v0.47.0；golang.org/x/term 进 require
  migrations/sql/00005_river_main_v2_to_v7.sql    River 迁移，锁定版本导出、原样
  migrations/river_test.go                        迁移文件与 go.mod 中 River 版本内嵌的 SQL 一致
  internal/platform/jobs/jobs.go                  River 客户端：注册、启动、停止
  internal/platform/config/                       jobs 节；auth.session_cleanup_interval
  internal/platform/postgres/pgtest/lockwait.go   WaitForLockWaitsOn：按表计数
  internal/archtest/composition_test.go           命令行的组合到不了 HTTP、限流、后台任务
  internal/bootstrap/
    app.go                                        后台任务的启动（等迁移）与停机顺序
    users.go                                      管理命令的组合、输出与错误格式
    registrants.go                                停用的注册者：serve 与命令行共用一处
  internal/modules/identity/
    module.go                                     New；Jobs()
    admin.go                                      NewAdmin、AdminDeps、Admin 的五个方法
    parts.go                                      New 与 NewAdmin 共用的部件（store、argon2、规则、停用的步骤）
    app/admin.go                                  管理用例共用的部件：按邮箱锁账户（lockAccount）、执行者 cli（byCLI）
    app/create_account.go                         注册与 CreateUser 共用的"建账户"
    app/create_user.go、reset_password.go、set_email.go、activate.go
    app/deactivate.go                             按邮箱的停用（与自助停用共用 deactivate）
    app/cleanup_sessions.go                       删除过期会话
    app/login.go                                  锁下核对邮箱
    domain/user.go                                NewEmail；errors.go：email_unchanged
    adapter/postgres/                             按邮箱锁账户、改邮箱、启用、全部撤销 PAT、可用 PAT 计数、删除过期会话
    adapter/river/cleanup.go                      清理的 worker 与定时任务
  cmd/nervewiki/users.go、password.go             五个命令；读取密码（终端经小接口）
deploy/runtime-grants.sql                         运行时角色的授权
e2e/fixtures/server.ts、users.ts；stories/identity/a11（命令行一段）、a12、a13
```

### 3.2 River 与迁移

- **版本**：River v0.47.0 与 `riverdriver/riverpgxv5` v0.47.0（与 Nerve 相同；River 要求 Go ≥1.26、pgx ≥5.10，都满足）。
- **迁移** `00005_river_main_v2_to_v7.sql`：`river migrate-get --line main --all --exclude-version 1 --up/--down`（锁定版本的命令行）的输出原样放进一个 goose 文件；Up 与 Down 各包在一对 `StatementBegin/End` 里（goose 按分号切分会切断 `$$` 函数体）。第 1 版（`river_migration` 表）不导出，第 5 版容忍它不存在。建出 `river_job`、`river_leader`、`river_queue`、`river_notification` 与它们的类型、函数。以后升级 River 另写一个迁移（`--version N`）。
- **核对**：`migrations/river_test.go` 用 River 的 `rivermigrate` 取 go.mod 中版本内嵌的 v2–v7 的 SQL，与文件的 Up、Down 两段逐字比较：文件被改动、或者 River 升级而迁移没跟上，测试失败。
- 迁移文件有行尾空格：`.gitattributes`（`linguist-generated=true`；全局的 `* text=auto eol=lf` 保证 LF）与 `.editorconfig` 各加一段保护它。
- `schema_test`：约束与索引名的测试排除 `river_` 开头的表（River 的命名不归我们管）；up/down 的对象清单加上类型与函数（排除扩展的成员），确认 down 之后什么也不剩。
- 架构测试：`github.com/riverqueue/river` 只由 `platform/jobs` 与模块的 `adapter/river` 导入（领域与用例不知道 River；bootstrap 只经 `platform/jobs`）。

### 3.3 `platform/jobs`

从 Nerve 拷贝、裁剪：

- `Job{Add func(*river.Workers) error; Periodic *river.PeriodicJob}`；模块经 `Jobs() []jobs.Job` 提供，bootstrap 汇总给 `jobs.New(pool, Config{ShutdownTimeout, Logger}, jobs)`。同一种任务的两个 worker 报错（`AddWorkerSafely`）。只有默认队列，最多 2 个 worker；River 的日志进同一个 logger。
- **去掉启动的重试循环**：serve 启动前已确认数据库可用（M0 的 `awaitDatabase`），`Start` 同步返回错误。
- **保留 Nerve 的 C1 修正**：River 的 `Start` 拿到的 ctx 由 `context.WithoutCancel` 派生，不随 serve 的 ctx 取消；停止只经 `client.Stop`（River 以 `ErrStop` 为原因停下，它的 reindexer 据此删掉没建完的 `_ccnew` 索引），`Stop` 返回之后再释放启动时的 ctx。
- `Stop`：期限 `jobs.shutdown_timeout` 加 1 秒的宽限；超时报错"jobs still running …"。记 info "jobs started"（带 `shutdown_timeout`）与 "jobs stopped"。没有启动过时 `Stop` 什么也不做。

### 3.4 serve 的启动与停机

```
awaitDatabase → auto_migrate 时迁移 → CheckDatabase
→ 并发：HTTP 服务到 ctx 结束；后台任务在没有待执行的迁移之后启动
ctx 结束：HTTP 先停（等请求结束）→ 后台任务停 → 迁移器 → 连接池
```

- **等迁移再启动后台任务**：M0 允许 `auto_migrate` 关闭时服务照常启动、就绪检查答 503，由运维另行 `migrate up`。River 的 `Start` 在没有 River 表的库上同步失败（`relation "river_queue" does not exist`），而启动失败会停下 serve。所以迁移检查不通过时记一次 warn "background jobs wait for the migration check to pass"（带原因：待执行的迁移，或还不能读 `goose_db_version` 的角色；serve 已在停止时不记），每 2 秒用 `migrator.CheckUpToDate` 查一次，通过之后启动；运维迁移之后不必重启，任务也不会悄悄缺席。没有待执行的迁移（通常情况）时第一次检查就启动。
- 启动失败是故障：取消 serve 的 ctx（带原因），HTTP 随之停下，serve 以这个错误退出。例如运行时角色少了 `river_queue` 的权限（3.9）。HTTP 自己先停下时（地址被占用），同样不再等迁移。
- **停机顺序**：HTTP 的 `ListenAndServe` 返回（请求已结束）之后，等"等迁移再启动"的协程返回，再 `jobs.Stop`，然后 `close`（迁移器 → 连接池）。HTTP 先于任务停：正在处理的请求可能投递任务（M2 以后），任务先停会让它失败。
- 迁移器先于连接池关闭，功能上看不出来（`stdlib.OpenDBFromPool` 不留空闲连接），只由日志与代码顺序保证，与 Nerve 相同。
- 最坏停机时间：`server.shutdown_timeout`（20 秒）+ `jobs.shutdown_timeout`（10 秒）+ 1 秒 + 连接池的 5 秒 = 36 秒。README 与 `image-smoke.sh` 的 `docker stop -t` 改为 40。
- River 的 notifier 从池里拿走一个连接：实际连接数是 `database.max_conns` + 1，README 写明。

### 3.5 会话清理

- 查询 `DeleteExpiredSessions`：`DELETE … WHERE id IN (SELECT s.id FROM auth_sessions s WHERE s.expires_at < @now LIMIT @batch FOR UPDATE SKIP LOCKED)`；用已有的 `auth_sessions_expires_at_idx`。被持有的行（正在续期或退出的会话）跳过，下一次再删。
- 用例 `CleanupSessions`：时钟读一次，每批 1000 行，一批不满即停；删了才记 info "expired sessions deleted"（`deleted`）；某一批失败时返回错误与之前的个数。
- 只删过期的：撤销而未过期的会话保留到过期，续期对它们答 `refresh_token_invalid` 所需的只是撤销状态，过期之后一并删除。撤销的 PAT 保留（列表不显示，行留作记录）。
- River 的定时任务：kind 与 id 都是 `identity.cleanup_expired_sessions`，`PeriodicInterval(auth.session_cleanup_interval)`，`RunOnStart`。间隔默认 1 小时，至少 1 秒（River 的建议下限），test 配置 2 秒。

### 3.6 管理用例

都按邮箱锁账户行（新查询 `LockUserByEmail … FOR NO KEY UPDATE`，返回 id、邮箱、哈希、是否可用），哈希在事务之外算；日志记 `user_id` 与 `by: "cli"`，从不记邮箱。不可能合法的地址（`ValidEmail` 不通过，包括不是合法 UTF-8 的）不查库，直接 `account_not_found`（与登录的 `find` 相同）。

| 用例 | 做什么 | 结果 |
|---|---|---|
| `CreateUser{Email, Password}` | 与注册共用的"建账户"：`domain.NewAccount` 一次报出全部问题 → 哈希 → 插入（显示名取邮箱 @ 之前的部分）。不经过注册开关，不建会话 | 账户 id 与邮箱；已占用答 `identity.email_taken` |
| `ResetPassword{Email, Password}` | 按账户的邮箱检查密码规则 → 哈希 → 事务：锁 → 写哈希 → 撤销全部会话（`password_reset`）→ 撤销全部 PAT（过期的也撤销） | 撤销的会话数与 PAT 数 |
| `SetEmail{Email, NewEmail}` | 检查新地址（`domain.NewEmail`，字段 `new_email`）；规范化之后相同答 `identity.email_unchanged`（锁之前）→ 事务：锁 → 改邮箱（冲突答 `email_taken`）→ 撤销全部会话（`email_changed`）。PAT 保留 | 撤销的会话数 |
| `Deactivate{Email}` | 事务：锁 → 已停用则什么也不做 → 否则与自助停用共用的 `deactivate`（否决者 → 写入 → 订阅者） | 撤销的会话数，或"本来就停用" |
| `Activate{Email}` | 事务：锁 → 已可用则什么也不做 → 否则启用，数一数可用的 PAT（未撤销、未过期） | 恢复可用的 PAT 数，或"本来就可用" |

- **停用、启用只在状态变化时生效**：已停用的账户再停用，不再跑否决者与订阅者（Nerve 是整段重跑）。扩展点的事件对应真实的状态变化，M2 的订阅者不必自己判断是否重复。
- 重置密码与改邮箱对已停用的账户照样执行（管理员可能先重置再启用）。
- **登录在锁下另核对邮箱**：登录按地址找到账户、在事务外校验密码，锁下原本只比较哈希的快照。改邮箱在两者之间提交时，用旧地址的登录仍会建出会话，而改邮箱刚撤销了全部会话。锁下读到的邮箱（P3 已让 `LockForCredentials` 返回它）与找到账户时的不同，就答 `invalid_credentials`，与不存在的地址相同。
- 错误码：`identity.account_not_found`（已有，说明 "The account does not exist."）、新增 `identity.email_unchanged`（只由命令行答出，不进契约）。
- **"建账户"一步**：`app/create_account.go` 的 `accountCreator{Rules, Hasher}`：`prepare(ctx, email, password)` 检查并哈希，返回待插入的 `NewUser`；注册在自己的事务里插入账户与会话，`CreateUser` 只插入账户。
- **组合**：`identity.NewAdmin(AdminDeps{Pool, Tx, Clock, Logger, Password, DeactivationVetoers, DeactivationSubscribers}) *Admin`，只凭连接池，不要签名密钥、限流与注册开关。它是模块根的包级函数：命令行不能先 `New` 出 HTTP 那一侧。`Admin` 的五个方法是命令行唯一的入口，参数与结果的类型在模块根以别名公开。`New` 与 `NewAdmin` 共用 `parts.go` 的部件（store、argon2 哈希器、密码规则、停用的步骤 `DeactivationSteps`）；`CredentialLock` 只有 `New` 的用例用到，在它的装配辅助函数 `useCases` 中构造（P3 审查 N6）。
- **注册者一处组合**：bootstrap 的 `registrants.go` 构造停用的否决者与订阅者（M1 没有，返回空），serve 与命令行都从它取，M2 加注册者时两边不会漏掉一边。

### 3.7 命令行

```
nervewiki users create --email <addr>
nervewiki users reset-password --email <addr>
nervewiki users set-email --email <addr> --new-email <addr>
nervewiki users deactivate --email <addr>
nervewiki users activate --email <addr>
```

- cobra，与已有的 `serve`、`migrate` 同样加载配置（`NWIKI_` 前缀，校验整份配置：prod 下同样要求签名私钥文件，README 已为 `migrate` 写明）。`run` 增加标准输入参数。结果一行写到 stdout，日志写到 stderr；失败打印 `nervewiki: <message>`、退出码 1。只输入 `users` 打印帮助、退出码 0。
- **密码**（`create`、`reset-password`）：不接受参数或环境变量（会从 `ps`、`/proc` 泄露）。标准输入是终端时不回显地提示两次（提示写到 stderr），两次不同就失败；否则读一行，只去掉行尾的 `\n` 或 `\r\n`（首尾空格保留，是密码的一部分）；什么也读不到报 EOF。空密码交给密码规则。
- **终端与 Ctrl-C**：`term.ReadPassword` 期间终端的回显是关着的，而 SIGINT 被 serve 的信号处理接住、读取继续阻塞，第二次 Ctrl-C 杀掉进程时回显仍然关着。读取放在协程里，ctx 取消时用 `term.Restore` 恢复终端并返回。终端的操作经一个小接口（`IsTerminal`、`GetState`、`ReadPassword`、`Restore`），这条路径用替身测试。
- 顺序：加载配置 → 读取密码 → `awaitDatabase` → 组合与执行。配置错误在提示输入密码之前报出。
- **错误格式**（bootstrap `users.go` 的 `commandError`）：`*shared.Error` 有字段时每个写成 `<命令行的名字> <message>`，用 `; ` 连接（`email` → `--email`，`new_email` → `--new-email`，`password` → `the password`）；没有字段时是它的说明。其他错误原样。
- **输出**：`created alice@corp.com (<id>)`、`password reset for alice@corp.com: revoked 2 sessions, 1 API token`、`e-mail changed to …: revoked 1 session`、`deactivated alice@corp.com: revoked 2 sessions` 或 `alice@corp.com is already deactivated`、`activated alice@corp.com: 1 API token is usable again` 或 `… is already active`。邮箱只出现在管理员自己终端的输出里，不进日志。

### 3.8 命令行的组合检查

`archtest/composition_test.go`（照 Nerve）：`go/packages` 加载 `internal/bootstrap`，`ssautil` 建 SSA，`callgraph/static` 建静态调用图，从 `bootstrap.Users` 出发广度优先遍历：

- 必须到达 `identity.NewAdmin`（证明遍历看到了组合）；
- 不得到达任何模块根包中名为 `New` 的函数、`platform/httpserver`、`platform/ratelimit`、`platform/jobs`、`github.com/riverqueue/river`。
- 起点是一个列表（M2 加 `workspaces` 命令）。只跟静态调用：函数值与接口调用不跟，这正是命令把 `*identity.Admin` 交给命令函数的写法。
- M2 的注册者构造函数不能叫 `New`（`NewAccounts` 这类名字可以），写进 M1→M2 的移交。
- 依赖只有已在 go.mod 的 `golang.org/x/tools`。耗时在 `-race` 下实测，超过 30 秒再考虑拆出单独的目标。

### 3.9 运行时角色

`deploy/runtime-grants.sql`：授权给组角色 `nervewiki_runtime`：`USAGE ON SCHEMA public`；逐表列出业务表（`users`、`auth_sessions`、`api_tokens`）与 River 四张表的 DML；River 两个序列的 `USAGE`；`river_job` 的 `MAINTAIN`（River 每天 `REINDEX INDEX CONCURRENTLY` 它的索引，PostgreSQL 17 起可授予）；`goose_db_version` 的 `SELECT`（就绪检查）。函数与类型靠 PUBLIC 的默认权限。

- 用法：表的所有者执行 `migrate up`，然后执行这个文件；服务以 `nervewiki_runtime` 的成员登录，`auto_migrate` 关闭。每次迁移之后重跑一次（幂等）。README 的部署一节写明。
- 逐表列出而不是 `ALTER DEFAULT PRIVILEGES`：每张新表都要有人决定运行时角色能做什么，目录测试逼着这件事发生。
- **测试**（bootstrap，照 Nerve）：建所有者角色并让它拥有库（pg_trgm 是受信任的扩展，要求库的 CREATE 权限）；幂等地建 `nervewiki_runtime`（角色是集群级的）；服务的登录角色 `IN ROLE nervewiki_runtime`；所有者迁移、执行授权文件。
  - 以运行时角色服务：`/readyz` 200；清理任务完成一次；注册、登录、创建 PAT、改密码、停用各走一遍；日志没有 "permission denied"；River 的每个索引 `REINDEX INDEX CONCURRENTLY` 成功。
  - 目录：`public` 中每个表、视图、序列、函数，运行时角色的权限恰好等于预期；新表没有进授权文件或预期，测试失败。

### 3.10 配置

```yaml
auth:
  session_cleanup_interval: 1h   # 删除过期会话的间隔，至少 1 秒
jobs:
  shutdown_timeout: 10s          # 停机时等正在执行的任务结束
```

test 配置：`session_cleanup_interval: 2s`。两项都进 `LogValue`。

### 3.11 集成与整个程序的测试

| 测试 | 守住 |
|---|---|
| River 迁移与内嵌的 SQL 一致；up/down 之后什么也不剩 | 3.2 |
| `platform/jobs`：定时任务按间隔运行；停机等正在执行的任务、超过期限报错；最多 2 个 worker；同种 worker 重复被拒；启动之后 serve 的 ctx 取消不停止 River，只有 `Stop` 停止它 | 3.3 |
| serve：有待执行的迁移时不启动任务、迁移之后启动（不重启）；没有待执行的迁移时立即启动 | 3.4 |
| 停机：一个请求卡在账户行锁上时取消 serve，任务在请求结束之前不停止（`WaitForLockWaitsOn("users")`，River 也在这个库上，按表计数）；日志顺序 `http server stopped` → `jobs stopped` → `database pool closed` | 3.4 |
| 清理：删除的边界（`expires_at` 等于现在的不删）；分批；持有行锁时跳过（2 秒的期限之内返回）；worker 的失败让任务失败；按配置运行（bootstrap） | 3.5 |
| 管理用例：每个用例的成功与每种失败；重置撤销全部会话与 PAT（其他账户不变）；改邮箱保留 PAT；停用、启用在状态不变时什么也不做、否决者与订阅者不被调用 | 3.6 |
| 交错：重置等待持锁的创建 PAT（插入之后停住），放行之后撤销它的令牌；重置等待持锁的登录，放行之后撤销它的会话；在重置之前校验了旧密码的创建 PAT 答 401、没有令牌；改邮箱在登录的校验与事务之间提交，登录答 401、没有会话 | 3.6 的锁与核对 |
| 命令行：五个命令依次作用于一个账户，另一个账户不变；标准输入的各种行尾，存下的 argon2 参数来自配置；每种失败的退出码与消息；只输入 `users` 打印帮助；终端路径（替身）：两次不同失败、ctx 取消时恢复终端 | 3.7 |
| 没有机密：日志开到 debug，跑 `create` 与 `reset-password`；stdout、stderr 中查不到每一步的密码、存下的哈希与其派生密钥的原文、引号转义、大小写十六进制、base64 与 base64url | 3.7 |
| 组合检查 | 3.8 |
| 运行时角色 | 3.9 |

### 3.12 端到端

- `e2e/fixtures/server.ts` 的 `runNervewiki` 增加标准输入与额外的环境变量；`fixtures/users.ts`：`nervewikiUsers(db, args, password?)` 以 debug 日志运行；设置密码的命令断言 stderr 有日志行、stdout 与 stderr 都查不到密码（原文、大小写十六进制、base64、base64url），状态没变的命令不记日志；`nervewikiUsersFails` 断言退出码 1、stdout 为空、stderr 以 `nervewiki: <message>` 结尾。
- A11（命令行一段）：自助停用之后 `users activate`：两个 PAT 恢复 200，旧的访问令牌与刷新令牌仍然 401，密码登录 200。
- A12：`users create`（之后能登录）；`reset-password`（全部会话与 PAT 撤销，旧密码不能登录，新密码能）；`set-email`（会话撤销，PAT 照常，新地址登录）；失败：不存在的账户、地址不变、地址已占用、弱密码。
- A13：把一个会话的 `expires_at` 改到过去，`expect.poll` 等到它被删除（test 配置每 2 秒一次）；未过期的会话仍在。

## 4. 实施步骤

在分支 `m1-p4-admin-jobs` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | River 与后台任务：依赖、迁移与它的核对、`schema_test`、`platform/jobs`、`jobs` 配置、`WaitForLockWaitsOn`（serve 的接线随 S2，见第 7 节） | [P4-S1-jobs.md](plans/P4-S1-jobs.md) |
| S2 | 会话清理：查询、用例、River 的 worker 与定时任务、`Jobs()`、配置；serve 等迁移再启动任务、停机顺序 | [P4-S2-session-cleanup.md](plans/P4-S2-session-cleanup.md) |
| S3 | 管理用例：按邮箱锁、五个用例、"建账户"、`NewAdmin` 与部件、`New` 的拆分、登录核对邮箱；交错 | [P4-S3-admin.md](plans/P4-S3-admin.md) |
| S4 | 命令行：`users` 五个命令、读取密码、错误格式、注册者一处组合、`composition_test`、没有机密的测试 | [P4-S4-cli.md](plans/P4-S4-cli.md) |
| S5 | 运行时角色：`runtime-grants.sql` 与测试；README 的部署 | [P4-S5-runtime-role.md](plans/P4-S5-runtime-role.md) |
| S6 | 端到端：A11 的命令行一段、A12、A13；README | [P4-S6-e2e.md](plans/P4-S6-e2e.md) |

规模估计：生产代码约 1,150 行（不含 River 迁移的约 520 行），测试约 3,000 行，端到端约 300 行。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 管理用例对端口的调用顺序与错误；清理的分批；读取密码（行尾、终端替身）；命令的错误格式；配置的校验 |
| 集成 | 第 3.11 节 |
| 架构 | River 的导入范围；命令行的组合检查 |
| 端到端 | A11 的命令行一段、A12、A13；之前的故事照旧通过（包括在同一个库上另起一个 nervewiki 的 A2、A3、A14：两个 River 客户端） |

反向对照（验证后撤销）：
- 任务先于 HTTP 停止 → 停机测试失败；
- River 的 `Start` 用 serve 的 ctx → "只有 Stop 停止它"的测试失败；
- 不等迁移就启动任务 → 待执行迁移的测试失败；
- 清理不带 `SKIP LOCKED` → 持有行锁的测试失败；
- 重置先撤销、后锁账户 → 与创建 PAT 的交错失败；
- 登录在锁下不核对邮箱 → 改邮箱的交错失败；
- 已停用的账户再停用时跑扩展点 → 停用的测试失败；
- 命令行的组合调用 `identity.New` → 组合检查失败；
- 授权文件漏一张表 → 目录测试失败；
- 命令把密码写进日志 → 没有机密的测试失败；
- River 迁移改动一行 → 迁移核对失败。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查完成（`reviews/P4-admin-jobs-review.md`），发现的问题已修复。
- M1 总设计进度表更新；M0 移交中本 Phase 的两项核对。

## 7. 结果

分支 `m1-p4-admin-jobs`：S1 `cbc8923`、S2 `5da0f15`、S3 `dc36cb9`、S4 `af1aaad`、S5 `6905ae6`、S6 `0678c93`，审查修复 `8335740`。第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`（另跑 `--repeat-each 3` 与 `--workers 1`）、`make image-smoke`（在克隆上，`modified=false`）本地与持续集成为绿。组合检查本身约 0.5 秒，`-race` 下整个 archtest 包约 4 秒，不必拆出单独的目标。审查见 [P4 审查记录](reviews/P4-admin-jobs-review.md)：2 项 Minor、4 项 Nit，N4 不做，其余已修复。规模：生产代码（Go、SQL 与配置，不含生成物与 River 迁移）约 1,600 行，比估计多出的主要是注释与五个管理用例各自的文件；测试约 2,900 行；端到端约 260 行；River 迁移 521 行，生成代码 130 行。

与设计的出入（已同步进上文）：

1. 后台任务的接线从 S1 移到 S2：River 拒绝启动没有 worker 的客户端，serve 要启动任务就得先有清理的 worker。
2. 停用的步骤抽成 `DeactivationSteps`（否决者 → 写入 → 订阅者），自助停用与管理员的停用共用；拒绝与成功的日志都带 `by`（`self` 或 `cli`）。
3. 按邮箱加锁要返回账户的 id：`LockedAccount` 增加 `ID`。
4. 错误格式在 bootstrap 的 `users.go`，读取密码在 `cmd/nervewiki/password.go`（设计 3.1 原写 `commands.go`、`users.go`）。
5. 组合检查另断言 `bootstrap.Users` 与 `bootstrap.newApp` 都到达 `deactivationRegistrants`。它只证明调用了；M2 的第一个注册者要加经命令行的行为测试（写进 M1→M2 的移交）。
6. 端到端的 `nervewikiUsers` 只对设置密码的命令断言有日志行：状态没变的命令（启用已可用的账户）什么也不记。
7. `image-smoke` 另以 `users create` 建第一个账户并登录：prod 关闭注册时这就是第一个账户的来历。
8. 运行时角色的测试另以该角色走一遍接口（注册、登录、续期、PAT、引导步骤、改密码、停用）与五个管理命令，授权文件执行两次（幂等）；审查之后另加"缺 `river_queue` 的授权时 serve 以错误退出"（审查 M2）。
9. `WaitForLockWaits` 与 `WaitForLockWaitsOn` 共用一个轮询函数；`migrations/embed.go` 的注释写明 River 的迁移归 River。
10. 审查之后：管理员停用的 `FOR SHARE` 交错（M1）；等迁移的 warn 改为 "background jobs wait for the migration check to pass"，serve 停止时不记（N1）；HTTP 自己停下时 `run` 返回的测试（N2）。
11. README：River 在停机时可能记的 ERROR 与它的触发条件；缺少各项授权的表现；授权文件在启动服务之前执行。

留给之后的：M2 的第一个停用注册者要有经命令行的行为测试（上文第 5 项）；注册者的构造函数不能叫 `New`（组合检查把模块根的 `New` 当作 HTTP 一侧）。
