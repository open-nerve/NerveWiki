# M1/P2 会话与限流：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M1/P2 会话与限流 |
| 状态 | 已完成 |
| 基线 | P1 合并之后的 main |
| 上级文档 | [M1 总设计](00-M1-design.md) 第 3–8 节；[P1 文档](01-P1-identity-foundation.md) 第 3 节；[总体设计](../v0.1-design.md) 6.1、6.2、13 |

---

## 1. 基线

P1 留下的：`users`、`auth_sessions` 两张表与 sqlc；JWT、刷新令牌的格式与 MAC、argon2 与并发名额；默认拒绝的认证中间件（公开操作清单、401 与 `WWW-Authenticate`）、客户端 IP 与 `server.trusted_proxies`；`register`、`getMe`；由契约推导的整个程序测试；A1、A2 的接口版本。注册只签发第一代刷新令牌，没有登录、续期、退出，也没有任何限流。

M0 移交给本 Phase 的（M1 总设计第 7 节）：[P3 平台层](handoffs/M0-P3-platform.md)第 2 项（限流）、第 8 项（测试用的固定时钟，加锁）；[P4 接口契约](handoffs/M0-P4-api-contract.md)第 5 项的限流部分。

P1 有意推迟到这里的：`Authenticator` 返回凭证的限流键、过期的访问令牌单独报出（失败闸门据此退还名额）；刷新令牌的判定表。撤销原因的类型随 P3 的批量撤销加入（本 Phase 的两条语句在 SQL 中写明各自的原因）。

## 2. 目标与范围

**目标**：一个会话从登录到退出的完整生命周期，续期带重复使用检测；平台有了限流：公开操作按客户端 IP、其余按凭证，认证之前有失败闸门，登录与注册另有专门的桶。

**做**：
- `platform/ratelimit`（令牌桶）、`clock/clocktest`（加锁）；配置节 `ratelimit` 与 `auth.refresh_deadline`。
- httpserver：请求信息中按 IP 计数的键 `IPKey`（IPv6 按前缀）；限流中间件（`anonymous`、`authenticated`）；认证之前的失败闸门（`auth_failure`）；429 与 `Retry-After`；`Authenticator` 返回凭证的限流键。
- identity：`login`（不存在的邮箱与错误的密码等时、同答；快照重试）、`refresh`（判定表、条件轮换、服务端期限）、`logout`；`register` 接入 `register_ip`；模块的桶 `login_ip`、`login_ip_email`、`register_ip`。
- 端到端：A3–A6、A14 的接口版本。

**不做**：`password_user` 桶（第一个校验密码的已认证操作在 P3：改密码、创建 PAT）；停用账户的登录只在集成测试中覆盖，A3 的这一段随 P3 的停用接口补进端到端；会话清理任务（P4）；前端（P5）。

## 3. 设计

### 3.1 文件

```
server/
  internal/platform/ratelimit/ratelimit.go      Limiter、Bucket（Allow、Reserve）、AllowAll、满桶的清扫
  internal/platform/clock/clocktest/            Fixed：At、Now、Advance，互斥锁保护
  internal/platform/postgres/pgtest/lockwait.go WaitForLockWaits：等到本库有 n 个语句在等锁（确定性的交错测试）
  internal/platform/config/                     ratelimit 节；auth.refresh_deadline 与跨栈的时间约束（不超过 server.request_timeout）
  internal/platform/httpserver/
    clientip.go                                 key：IPv4 取地址，IPv6 取前缀
    api.go                                      请求信息加 IPKey；失败闸门并入认证；限流中间件的位置；逐路由的请求期限（RequestTimeouts）
    limit.go                                    Limiter 端口、限流中间件、429
  internal/modules/identity/
    domain/session.go                           判定表 JudgeRefresh
    domain/errors.go                            invalid_credentials、account_deactivated、refresh_token_invalid
    app/login.go、refresh.go、logout.go；ports.go 加账户行锁的端口（P3 的 CredentialLock 也用它）
    adapter/postgres/                           登录读账户、锁账户行、改哈希；续期读、轮换、撤销；退出
    adapter/http/auth.go、limits.go             三个操作；模块的桶；续期与退出的请求期限（RequestTimeouts）
    adapter/authn/                              凭证键；过期的访问令牌
api/modules/identity.yaml                       login、refresh、logout
api/openapi.yaml                                顶层 x-problem-codes 加 rate_limited
e2e/fixtures/auth.ts（login、refresh）、assert/identity.ts（会话的代数与撤销）；stories/identity/a3–a6、a14
```

### 3.2 限流

- **令牌桶**（`platform/ratelimit`，照 Nerve）：每个桶一个速率（每分钟补充的单位）与容量；新键满桶开始；取不到单位时返回距离下一个单位的时长，成为 `Retry-After`（向上取整到秒）。`AllowAll` 对几个桶全取或全不取；`Reserve` 先取、之后可以退还一次。满桶的键每分钟清扫一次（取单位时顺带进行，没有后台 goroutine），内存只跟随最近的调用方。时间取 `time.Now` 的单调读数，不取 `Clock`：墙上时钟的跳变不能灌满或抽干桶。
- **平台的桶**：公开操作按客户端 IP 取 `anonymous`；其余按凭证取 `authenticated`（键 `session:<id>`，P3 起 PAT 为 `pat:<id>`）。被模块的桶拒绝的请求仍计入平台的桶。
- **IPv6 前缀**：`ratelimit.ipv6_prefix_len`（默认 64，1–128）。按 IP 计数的键：IPv4 是地址本身，IPv6 是这个长度的前缀，因为一台主机通常拿到整个 /64；会话与日志仍记录完整地址。
- **模块的桶**：`login_ip` 与 `login_ip_email`（键为客户端 IP 键加规范化邮箱的 SHA-256，键的大小与客户端发送的无关）全取或全不取：在一个邮箱上失败不会把这个客户端锁在其他邮箱之外；`register_ip` 按客户端 IP。模块用平台的 `ratelimit.Limiter`，拒绝时答 `shared.RateLimited`。
- **日志**：被拒绝时记一行 `bucket`、`ip`，不记邮箱。平台的桶记 debug：访问日志已在 info 级记下 429，再记一行会让不受任何桶约束的拒绝把日志翻倍（审查 M2）；模块的桶（登录、注册）记 info，它们是撞库的信号。

### 3.3 失败闸门与认证

逐路由中间件（P2 的最终顺序）：

```
请求信息 → 请求期限 → 请求体上限 → 失败闸门与认证 → 限流 → 请求体结构检查
```

- **失败闸门**：带令牌的请求在认证之前先从客户端 IP 的 `auth_failure` 预留一个单位，取不到就答 429，不运行认证。凭证失败保留这个单位；认证成功、访问令牌仅仅过期、内部故障都退还。先预留再认证，并发的请求也不会让失败次数超过桶的容量。代价：同一个 IP 同时处于认证中的请求至多是桶的容量（默认 60）；认证只是一次主键查询，名额在它之后立即退还，平时只有一个 NAT 后面极高的并发才会碰到；数据库变慢（连接池耗尽、锁等待）时认证变慢，在途的名额随之增多，同一个 IP 的有效请求也可能先答 429（审查实测：容量 3、表被锁 3 秒时，6 个并发的有效请求 3 个立即 429）。公开操作与没有令牌的请求不经过闸门。
- **`Authenticator`** 改为返回 `(ctx, credentialKey, error)`：键供限流中间件使用；过期的访问令牌是 `ProblemStatus() 401` 且 `ExpiredCredential() true` 的错误。identity 的 `authn` 把 `app.ErrAccessTokenExpired` 包装成这样的错误。
- **为什么过期不计入失败**：访问令牌 15 分钟过期是客户端续期的正常信号，多个标签页会同时碰上；计入会让正常使用的用户被闸门挡住。

### 3.4 登录

`POST /api/v0/auth/login`（公开）：

1. 规范化邮箱；不可能合法的邮箱不查库。
2. 读账户（id、密码哈希），哈希作为快照；是否可用在第 4 步锁下读。不存在的邮箱对启动时生成的**哑哈希**（当前参数、随机密码）校验一次，然后答 401 `identity.invalid_credentials`：耗时与邮箱存在时相同，回答也相同。
3. 在事务外按快照校验密码；参数变化时在事务外重新哈希。
4. 一个事务：锁账户行（`FOR NO KEY UPDATE`，全局加锁顺序 `users → auth_sessions`），核对哈希仍等于快照；账户停用答 403 `identity.account_deactivated`（此时已知密码正确，才透露状态）；写入新哈希（如有）；插入会话。会话与令牌在事务外生成。
5. 哈希在两步之间变了（并发的登录重新哈希了，或密码被改了）：对新哈希再校验一次、再做第 4 步；第二次还变就答 401。

失败记一行 info：原因（`invalid_credentials`、`deactivated`）、IP、有账户时的 `user_id`，从不记邮箱。成功记 `user_id`、`session_id`、IP。

### 3.5 续期

`POST /api/v0/auth/refresh`（公开，只经过平台的 `anonymous` 桶）：

- 解析令牌不查任何东西；格式不对直接 401 `identity.refresh_token_invalid`。
- 一个事务按会话 id 读取会话（代数、当前哈希、撤销、期限），按判定表决定：

  | 会话 | 令牌 | 判定 |
  |---|---|---|
  | 已撤销或已过期 | 任意 | 拒绝 |
  | 有效 | 当前代，密文哈希一致 | 轮换 |
  | 有效 | 当前代，密文不一致 | 拒绝 |
  | 有效 | 更早的一代，MAC 标签有效 | 重复使用：撤销会话（`reuse_detected`），拒绝 |
  | 有效 | 更早的一代，标签无效（伪造、或换钥之前签发） | 拒绝，会话不变 |
  | 有效 | 更晚的一代 | 拒绝 |
  | 不存在 | — | 拒绝 |

- **轮换**是条件更新：只在会话仍处于判定时的代数与哈希、未撤销、未过期时命中。没命中说明并发的续期或撤销先提交了：在同一事务内重读、重判一次；重判之后仍然没命中是故障（500），不是回答。
- 重复使用的撤销在 401 发出之前提交，并记一行 warn：`user_id`、`session_id`、IP。
- **当前代不依赖 MAC**：当前代由存储的哈希证明，所以换钥之后当前代仍能续期，只是旧代的标签全部失效，不会再被认成重复使用。
- **服务端期限**：续期与退出的请求期限是 `auth.refresh_deadline`（默认 4 秒），由平台的期限中间件按路由施加（`APIConfig.RequestTimeouts`，模块声明）。续期协议要求服务端在前端放弃（8 秒，P5 的令牌管理器）之前给出结果：前端放弃之后服务端才提交，客户端手里就是被轮换掉的旧代，下次续期会被当成重复使用而撤销会话。配置校验 `refresh_deadline + database.commit_timeout < 8s`，且不超过 `server.request_timeout`。
- **期限到期**与其他请求的期限一样：记 warn（"API request deadline exceeded"），答 500。到期时语句在 COMMIT 之前被放弃，事务回滚，令牌仍是当前代，客户端重试即可；COMMIT 有自己的 `commit_timeout`，不受这个期限打断。最初由 handler 自己派生期限，到期被记成 error（"API handler failed"），误报为基础设施故障（审查 M1）。
- 会话的期限从登录起算，续期不延长；响应的 `refresh_token_expires_at` 是这个绝对期限。

### 3.6 退出

`POST /api/v0/auth/logout`（公开）：只在令牌是当前代、会话有效时撤销会话（`logout`），一条条件更新，不开事务。其他任何令牌什么也不改，也不是错误：回答总是 204，不透露令牌是什么；重复使用只由续期判定。之后这个会话的访问令牌在下一个请求就失效（认证逐请求核对会话）。

### 3.7 配置

```yaml
auth:
  refresh_deadline: 4s     # 续期与退出的请求期限；加上 database.commit_timeout 须小于前端的 8 秒，且不超过 server.request_timeout
ratelimit:
  ipv6_prefix_len: 64
  anonymous:      {per_minute: 600,  burst: 100}   # 公开操作，按客户端 IP
  auth_failure:   {per_minute: 60,   burst: 60}    # 认证之前的失败闸门，按客户端 IP
  authenticated:  {per_minute: 1200, burst: 200}   # 需要令牌的操作，按凭证
  login_ip:       {per_minute: 30,   burst: 10}
  login_ip_email: {per_minute: 10,   burst: 5}
  register_ip:    {per_minute: 10,   burst: 5}
```

- test 配置把每个桶调到用不完（端到端与集成测试大量注册、登录）；限流本身的测试用小桶。
- 每个桶的 `per_minute`、`burst` 至少为 1；日志照录。

### 3.8 接口

`api/modules/identity.yaml` 增加：

| 操作 | 请求体 | 成功 | 码 |
|---|---|---|---|
| `POST /auth/login` | `{email, password}` | 200 `AuthTokens` | `identity.invalid_credentials`、`identity.account_deactivated`、`server_busy` |
| `POST /auth/refresh` | `{refresh_token}` | 200 `AuthTokens` | `identity.refresh_token_invalid` |
| `POST /auth/logout` | `{refresh_token}` | 204 | — |

顶层 `x-problem-codes` 加 `rate_limited`：每个操作都可能被平台的桶拒绝，模块的桶（登录、注册）答的是同一个码，操作上不再重复列出（契约规则：操作只列顶层之外的码）。登录的邮箱不做格式校验，不答 422：错误的与不存在的都是 401。

### 3.9 集成与整个程序的测试

| 测试 | 守住 |
|---|---|
| 续期的判定表（领域，逐行）与真实数据库上的续期：伪造的旧代不撤销、真实的旧代撤销、换钥之后当前代仍可续期而旧代不再触发撤销 | 3.5 |
| 并发续期：测试持有会话行，两个续期都读到同一代、都等在条件更新上（`WaitForLockWaits`）之后放行；恰有一次轮换，另一次被认成重复使用，会话被撤销 | 条件轮换与重读 |
| 仓储：轮换与退出的每个条件各破坏一次，语句都不碰这一行 | 条件更新的每个条件 |
| 登录的快照重试：用测试替身在校验与加锁之间改哈希（用例的测试；真实数据库上的并发重新哈希只证明结果一致，不证明发生了重试） | 3.4 第 5 步 |
| 续期超过期限：测试持有会话行，续期在期限内答 500，会话不变，放行之后用同一令牌重试答 200 | 期限到期是可重试的 |
| 不存在的邮箱与错误的密码：同一个码、同一个回答（真实数据库）；不存在的邮箱对哑哈希校验恰好一次（用例的测试）。哑哈希由同一个哈希器在启动时生成，参数与之相同（结构上成立，没有单独的测试） | 等时 |
| 失败闸门：50 个并发的无效令牌、容量 3，恰好 3 次认证、47 个 429；过期的访问令牌退还 | 3.3 |
| 按 IPv6 前缀计数：同一 /64 的两个地址共用一个桶 | 3.2 |
| 经可信代理：整个程序在 `trusted_proxies` 下登录，会话记录的是转发的客户端地址 | P1 的客户端 IP 接到会话 |
| 整个程序：没有令牌的操作答 401、公开操作不经过闸门（P1 的测试照旧通过） | 3.3 的顺序 |

`clocktest.Fixed` 加锁后，HTTP 集成测试可以在处理请求的 goroutine 读时钟的同时推进它；架构测试规则 8 把它列为测试辅助包。

### 3.10 端到端

- `e2e/fixtures/auth.ts` 增加 `login`、`refresh`；`assert/identity.ts` 增加按会话的断言（代数、`last_refreshed_at`、撤销与原因）。
- A3（登录；错误的密码与不存在的邮箱同答 401；经可信代理的客户端 IP）、A4（续期：代数递增、新访问令牌可用、旧代失效）、A5（重复使用撤销整个会话；伪造的旧代不能踢人下线）、A6（退出之后刷新令牌与访问令牌都失效）、A14（登录的桶答 429 与 `Retry-After`；失败闸门）的接口版本。限流的故事用 `nervewikiWith` 起一个小桶的服务。

## 4. 实施步骤

在分支 `m1-p2-sessions-ratelimit` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 平台：`clocktest`、`ratelimit`、配置、`IPKey`、限流中间件、失败闸门、`Authenticator` 的凭证键与过期；组合根装配平台的桶 | [P2-S1-ratelimit.md](plans/P2-S1-ratelimit.md) |
| S2 | 登录：查询、账户行锁、哑哈希、快照重试；模块的桶与注册的限流；契约 | [P2-S2-login.md](plans/P2-S2-login.md) |
| S3 | 续期与退出：判定表、条件轮换、重复使用、服务端期限；契约；集成测试 | [P2-S3-refresh-logout.md](plans/P2-S3-refresh-logout.md) |
| S4 | 端到端：A3–A6、A14 的接口版本 | [P2-S4-e2e.md](plans/P2-S4-e2e.md) |

规模估计：生产代码约 1,300 行，测试约 2,800 行。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 令牌桶（补充、容量、`AllowAll` 全有或全无、`Reserve` 与退还、清扫）；判定表；IP 键；配置的校验与时间约束 |
| 集成 | 登录、续期、退出在真实数据库上的各种状态与并发；失败闸门的并发；经可信代理的会话 IP |
| 契约 | 三个新操作的 handler 测试答出每个声明的码；P1 的整个程序测试覆盖新操作 |
| 架构 | `clocktest` 只被测试导入 |
| 端到端 | A3–A6、A14 的接口版本；A1、A2 与 M0 的故事照旧通过 |

反向对照（验证后撤销）：
- 条件轮换去掉代数条件 → 仓储的逐条件测试失败（`another_generation`）；哈希条件同样让第二次更新落空，并发续期的测试只在两个条件都去掉时失败（两次都轮换）；
- 续期去掉重读重判 → 并发续期的测试失败；
- 续期与退出不声明逐路由期限 → 期限的集成测试失败（按 5 秒的请求期限才答）；
- 旧代不核对 MAC 就撤销 → 伪造旧代的测试失败；
- 不存在的邮箱不做哑哈希校验 → 等时的测试失败；
- 失败闸门在认证之后才取单位 → 并发闸门的测试失败；
- 过期的访问令牌不退还 → 闸门的测试失败；
- `refresh_deadline + commit_timeout` 达到 8 秒 → 配置校验拒绝。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查完成（`reviews/P2-sessions-ratelimit-review.md`），发现的问题已修复。
- M1 总设计进度表更新；M0 移交中本 Phase 的各项核对。

## 7. 结果

分支 `m1-p2-sessions-ratelimit`：S1 `83ee161`、S2 `b673a28`、S3 `949bd27`、S4 `6accf06`，审查修复 `fd35140`。第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`（另跑 `--repeat-each 3` 与 `--workers 1 --repeat-each 2`）、`make image-smoke` 本地与持续集成为绿。审查见 [P2 审查记录](reviews/P2-sessions-ratelimit-review.md)：2 项 Minor、5 项 Nit，处置见记录。

与设计的出入（已同步进上文）：

1. `pgtest.WaitForLockWaits` 从 P3 提前（M1 总设计第 6 节），计数版：等到本库有 n 个语句在等锁。并发续期因此是确定性的：最初靠调度碰运气的写法在反向对照下不失败。
2. 撤销原因不建类型，两条语句在 SQL 中写明原因；类型随 P3 的批量撤销加入。
3. 仓储另有逐条件的测试：轮换与退出的每个条件各破坏一次，语句都不碰这一行。代数条件由它守住，并发测试守的是重读与重判（审查 N1）。
4. 失败闸门先预留再认证的代价写进 3.3：同一个 IP 在途的认证至多是桶的容量，数据库变慢时有效请求也可能先答 429；A14 的有效令牌因此串行发出。
5. 端口 `PasswordHasher{Hash}` 与 `PasswordVerifier{Verify}` 拆开（P1 审查 N1）。
6. 续期与退出的期限由平台按路由施加（`APIConfig.RequestTimeouts`），不再由 handler 派生：到期按请求期限记 warn、答 500（审查 M1）。配置另要求 `refresh_deadline` 不超过 `server.request_timeout`。
7. 平台的桶的拒绝记 debug，模块的桶记 info（审查 M2）。
8. A3 另有经可信代理登录的一个测试；A14 另有过期访问令牌的一个测试（`NWIKI_AUTH__ACCESS_TOKEN_TTL=1s`）。
9. README 写明桶在进程内存中、多实例各算各的。
