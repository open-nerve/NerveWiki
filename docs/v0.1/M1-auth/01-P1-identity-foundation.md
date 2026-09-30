# M1/P1 身份基础与默认拒绝：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M1/P1 身份基础与默认拒绝 |
| 状态 | 已完成（2026-09-30） |
| 基线 | `bd775ab`（M0 完成，M1 总设计） |
| 上级文档 | [M1 总设计](00-M1-design.md) 第 4–8 节；[总体设计](../v0.1-design.md) 6.1、6.2、7.1、8.1、13 |

---

## 1. 基线

M0 留下的：平台层（config、logging、postgres、httpserver 含 apitest / bodyshape / apigen、webui、clock、buildinfo）、组合根、`instance` 模块、契约与生成、前端外壳、端到端测试与镜像。所有接口操作都是公开的，逐路由中间件只有请求期限、请求体上限、请求体结构检查三层；没有业务表，没有 sqlc。

M0 移交给本 Phase 的（M1 总设计第 7 节）：
- [P3 平台层](handoffs/M0-P3-platform.md)第 1、3、5、7、10 项：客户端 IP 与 `trusted_proxies`、认证的接入、`warnIfExposed`、sqlc 相关的架构测试、机密 `*_file` 键的日志；
- [P4 接口契约](handoffs/M0-P4-api-contract.md)第 1、2、3、5 项：401 与 `WWW-Authenticate`、公开操作清单、参数与请求体的整个程序测试、逐路由中间件加入认证。

## 2. 目标与范围

**目标**：第一个业务模块 `identity` 落地，走通"注册 → 拿到令牌 → 带令牌调用"；平台从此默认拒绝：除了模块声明的公开操作，所有接口操作都要求有效的 Bearer 令牌，由整个程序的测试守住。

**做**：
- 迁移 `users`、`auth_sessions`；sqlc 与它的架构测试；约束与索引名、CHECK 反例的数据库测试。
- `shared.Actor`（调用者）与邮箱规则；identity 的领域规则：显示名、密码规则与常见密码名单、刷新令牌的格式。
- 凭证：JWT（Ed25519）与刷新令牌的 MAC（HKDF 派生）、签名私钥的读取、argon2id 与并发名额。
- httpserver：请求信息（客户端 IP、User-Agent）与 `server.trusted_proxies`、`Authenticator`、公开操作清单、401 与 `WWW-Authenticate`。
- identity 的 `register`、`getMe` 与 JWT 的认证；`instance` 的 `signup_enabled`；组合根的接线与 `warnIfExposed`。
- apitest 推导参数与请求体的破坏用例、两条写法规则；整个程序的测试。
- 端到端：A1、A2 的接口版本；`e2e/fixtures/assert/`（M1 建立）。

**不做**（本 M 之后的 Phase）：登录、续期、退出、限流与失败闸门（P2）；PAT、改资料、改密码、停用（P3）；命令、后台任务（P4）；前端（P5、P6）。请求信息中按 IP 计数的键 `IPKey` 随限流在 P2 加入。

## 3. 设计

### 3.1 文件

```
server/
  sqlc.yaml                                   每个模块一条：只看得到本模块的迁移
  migrations/sql/00002_identity_users.sql
  migrations/sql/00003_identity_auth_sessions.sql
  internal/shared/actor.go                    Actor{UserID, SessionID, APITokenID}：认证放进 ctx 的调用者
  internal/shared/email.go                    邮箱的规范化与校验（模块之间共用：M2 的邀请）
  internal/platform/config/                   auth 节；server.trusted_proxies（列表与 netip.Prefix 的解码）
  internal/platform/httpserver/
    clientip.go                               X-Forwarded-For 与可信代理
    api.go                                    请求信息、认证中间件、APIConfig 的扩充
    apierrors.go                              401 带 WWW-Authenticate
    apitest/operations.go                     Public、Target、ParamCases、BodyCases、HasJSONBody
  internal/modules/identity/
    module.go                                 Deps、New、PublicOperations、Authenticator、Register
    domain/                                   errors、user（账户、由邮箱得出的显示名）、password、session（刷新令牌格式）
    app/                                      ports、issuer、register、get_me、authenticate
    adapter/signing/                          私钥、JWT、刷新令牌的 MAC
    adapter/argon2/                           argon2id 与并发名额
    adapter/authn/                            实现 httpserver.Authenticator
    adapter/postgres/                         store、users、sessions；queries/*.sql；gen/（sqlc）
    adapter/http/                             handler、auth、me；gen/（oapi-codegen）
  internal/archtest/sqlc_test.go、rawsql_test.go（与各自的 cases）
api/modules/identity.yaml                     register、getMe
tools/password-blocklist/                     名单的生成脚本（取数、过滤、校验和）
e2e/fixtures/auth.ts、assert/identity.ts；stories/identity/a1、a2
```

依赖方向照总体设计 8.1：`adapter → app → domain`；identity 只被组合根导入；平台的 `Authenticator` 是平台声明的小接口，identity 的 `authn` 满足它。

### 3.2 表

`users`（账户与资料）：

| 列 | 类型与约束 |
|---|---|
| `id` | `uuid PRIMARY KEY`（应用生成的 v7） |
| `email` | `varchar(255) NOT NULL`，唯一约束 `users_email_key`；CHECK 小写且不含空白（领域先规范化，CHECK 挡住绕过应用的写入） |
| `password` | `varchar(128) NOT NULL`：argon2id 的 PHC 字符串 |
| `display_name` | `varchar(100) NOT NULL`，CHECK 非空 |
| `is_active` | `boolean NOT NULL DEFAULT true` |
| `onboarding_steps` | `text[] NOT NULL DEFAULT '{}'`；具名 CHECK `users_onboarding_steps_check`：最多 32 个、没有 NULL、元素不含逗号、每个 id 形如 `[a-z][a-z0-9_]{0,31}` |
| `created_at`、`updated_at` | `timestamptz NOT NULL`，由用例按它的时钟写入，不设默认值 |

`auth_sessions`（一次登录一行）：`id`、`user_id`（外键，账户删除时级联）、`token_hash`（当前一代密文的 SHA-256，CHECK 32 字节）、`generation`（≥ 0）、`user_agent`、`ip inet`、`expires_at`（登录时刻加 `auth.session_ttl`，之后不变）、`last_refreshed_at`、`revoked_at`、`revoke_reason`（`logout`、`password_changed`、`password_reset`、`email_changed`、`deactivated`、`reuse_detected`）、`created_at`、`updated_at`；具名 CHECK `auth_sessions_revoked_consistent_check`（撤销时刻与原因同时有或同时没有）；索引 `auth_sessions_user_id_idx`、`auth_sessions_expires_at_idx`。旧代的刷新令牌不存，由令牌里的 MAC 标签认出（P2）。

与 Nerve 的差异：没有 `profiles` 表、`first_name`、`last_name`、`user_timezone`；显示名上限 100；时间列不设 `DEFAULT now()`（总体设计 7.1）。

### 3.3 sqlc

- `server/sqlc.yaml` 每个模块一条：`queries` 在模块的 `adapter/postgres/queries/`，`schema` 恰好是本模块的全部迁移（`NNNNN_identity_*.sql`），查询碰到别的模块的表时生成就失败。uuid 映射为标准库的 `uuid.UUID`，`timestamptz` 为 `time.Time`，可空列为指针。
- sqlc 进 `server/tools` 模块（与 oapi-codegen 一起锁定版本），`make gen-go` 执行它，`make gen-check-go` 核对生成物。
- 架构测试：
  - `sqlc_test`：迁移文件名 `NNNNN_<归属>_<说明>.sql`；改表的语句（`ALTER TABLE`、`CREATE INDEX … ON`、触发器、`DROP TABLE`）只作用于同一模块建的表；`sqlc.yaml` 的每条与模块的迁移一致，有查询的模块都有一条。
  - `rawsql_test`：模块中非测试、非生成的代码不直接执行 SQL（`Exec`、`Query` 等），也没有 SQL 字面量；找不到任何 `store.go` 时失败，防止规则空转。
- 数据库测试（`server/migrations`）：约束与索引名的清单；逐条 CHECK 的反例（大写的非 ASCII 邮箱、全角空白、非法的引导步骤 id 等）。

### 3.4 凭证

| 项 | 设计 |
|---|---|
| 访问令牌 | JWT，EdDSA（Ed25519）；claims 只有 `sub`（账户 id）、`sid`（会话 id）、`exp`；有效期 `auth.access_token_ttl`（15 分钟），`exp` 是整秒，向上取整，令牌至少活到响应所说的 `access_token_expires_in`。校验只接受 EdDSA、要求 `exp`、严格解码；失败只报几种固定的原因，不带解析库的错误文本（伪造令牌中的字符串会进日志）；签名有效但过期单独报出（P2 的失败闸门据此退还名额） |
| 刷新令牌 | `nwk_rt_` + base64url(会话 id 16 字节、代数 4 字节、随机密文 32 字节、MAC 标签 16 字节)。标签是 HMAC-SHA256 的前 16 字节，密钥由签名私钥经 HKDF-SHA256 派生（info `nervewiki refresh-token mac v1`）。数据库只存当前一代密文的 SHA-256。P1 只签发（注册时的第一代），校验与轮换在 P2 |
| 签名私钥 | `auth.jwt.private_key_file`：PKCS#8 PEM 的 Ed25519 私钥（`openssl genpkey -algorithm ed25519`）。prod 必须提供（配置校验）；dev、test 为空时生成临时密钥并告警。读取错误不带文件路径与内容；日志只记是否设置 |
| 密码哈希 | argon2id，PHC 字符串；哈希与校验的都是密码的 NFKC 形式（NIST SP 800-63B 5.1.1.2：预组合与分解的重音、全角与半角字母是同一个密码），由 argon2 适配器完成（domain 与 app 只用标准库）；参数 `auth.password.argon2_*`（默认 OWASP 的最低配置：19 MiB、2 次、并行 1）；同时进行的哈希有上限 `max_concurrent_hashes`，拿不到名额最多等 `max_wait`，然后 503 `server_busy`。格式不对的哈希同样做一次计算再判不匹配 |
| 密码规则 | 8–128 个 UTF-16 单位；小写后的整串或其主干（去掉两端的非字母）在常见密码名单中，或主干等于邮箱 @ 之前部分的主干，或整串是同一个字符的重复、只有空白与控制字符，就拒绝（字段码 `common_password`，新增）；长度用已有的 `too_short`、`too_long`。名单由 `tools/password-blocklist` 生成：SecLists 中 NCSC 常用密码前 10 万的固定提交，取 8–128 个字符的条目，另取每个条目的主干（5 个字符以上，使 `Summer2024!` 按 `summer` 被拒绝），小写、去重，脚本核对下载的 SHA-256；生成物 `domain/common_passwords.txt` 嵌入二进制 |

### 3.5 认证的接入

逐路由中间件（P1 的顺序；P2 在认证之后加限流、认证之前加失败闸门）：

```
请求信息（客户端 IP、User-Agent）→ 请求期限 → 请求体上限 → 认证 → 请求体结构检查
```

- **默认拒绝**：`APIConfig.PublicOperations` 是各模块 `PublicOperations()` 的并集（路由模式，如 `POST /api/v0/auth/register`）；认证中间件按 `r.Pattern` 判断，公开操作从不看令牌。其余操作没有令牌答 401 `WWW-Authenticate: Bearer`；令牌无效答 401 `Bearer error="invalid_token"`；失败的原因只进 debug 日志。
- **Authenticator**：平台声明的接口，`Authenticate(ctx, token) (ctx, error)`（限流用的凭证键随 P2 加入）：返回带着 `shared.Actor` 的 ctx；无效令牌是 `ProblemStatus() 401` 的错误；其余错误是内部故障（500）。identity 的 `authn` 实现它：P1 只认 JWT（`nwk_pat_` 开头的在 P3 加入 PAT），按 `sid` 读会话并连同账户核对（未撤销、未过期、账户可用）。
- **客户端 IP**：`server.trusted_proxies`（CIDR 列表，默认空）。从右往左读 `X-Forwarded-For`，取第一个不在可信代理中的地址；遇到不是裸 IP 的条目就停下，客户端记为转发它的代理；IPv4 映射的 IPv6 按 IPv4。不认 `Forwarded`、`X-Real-IP`。三种配置错误各告警一次：没配置可信代理却收到头；可信代理转发的请求没有头；可信代理转发了不是裸 IP 的条目。配置了可信代理之后，其他对端带来的头是客户端自己写的，静默忽略：否则任何客户端都能让告警点名自己，并占掉真正的配置错误需要的那一次。
- **401 的头**：`APIErrors.Write` 对 401 加 `WWW-Authenticate: Bearer`（中间件已设置的不覆盖）；每个模块的 `components.responses.Problem` 声明 `Retry-After` 与 `WWW-Authenticate` 两个头；`securitySchemes.bearer` 写在根文件（打包会丢掉模块文件里的）。

### 3.6 配置

```yaml
server:
  trusted_proxies: []        # 反向代理的 CIDR；逗号分隔的环境变量同样可用
auth:
  signup_enabled: false      # 基础配置（也就是 prod）关闭；dev、test 覆盖为 true
  access_token_ttl: 15m
  session_ttl: 720h          # 从登录起算，续期不延长
  jwt:
    private_key_file: ""     # prod 必填
  password:
    argon2_memory_kib: 19456
    argon2_iterations: 2
    argon2_parallelism: 1
    max_concurrent_hashes: 4
    max_wait: 2s
```

- 列表键的解码：环境变量里逗号分隔，空串是空列表；`netip.Prefix` 由字符串解码，拒绝 `/0` 与 IPv4 映射的前缀。
- test 配置把 argon2 降到 64 KiB、1 次（测试大量注册）。
- 日志：`auth.jwt.private_key_file` 只记 `private_key_file_set`；`server.addr_file` 不是机密，照旧记路径。
- `warnIfExposed`：非 prod 且监听的不是回环地址时告警一次，列出开放的注册与临时密钥。

### 3.7 identity 模块

- **register**：
  1. 注册关闭时答 403 `identity.signup_disabled`，在领域校验（422）之前；请求体的结构检查（400）与上限（413）仍在它之前；
  2. 校验邮箱、密码，全部问题一次答 422；
  3. 在事务外计算哈希；
  4. 在事务外生成会话与令牌，提交之后不会再失败；
  5. 一个事务插入账户与第一个会话；邮箱已占用答 409 `identity.email_taken`。

  显示名取邮箱 @ 之前的部分（截到 100 个字符），`onboarding_steps` 为空。显示名的输入校验与引导步骤 id 的校验随修改它们的操作在 P3 加入，撤销原因随撤销会话的操作在 P2、P3 加入。注册开关是端口 `SignupPolicy`：P1 是配置的开关，M2 加入"持有效邀请可注册"。
- **getMe**：答 `User{id, email, display_name, onboarding_steps}`。
- **authenticate**：见 3.5；会话与账户一次查询读出。
- 端口按用例细分（照 Nerve）：每个用例只依赖自己用到的方法；一个 `postgres.Store` 满足全部仓储端口。
- 时间全部取自注入的 `Clock`，作为 SQL 参数传入；日志只记 `user_id`、`session_id`，不记邮箱。

### 3.8 接口

`api/modules/identity.yaml`：`POST /auth/register`（公开，201 `AuthTokens`，码 `identity.signup_disabled`、`validation_failed`、`identity.email_taken`、`server_busy`）；`GET /me`（Bearer，200 `User`）。`AuthTokens`：`token_type`、`access_token`、`access_token_expires_in`（秒，避开客户端的时钟偏差）、`refresh_token`、`refresh_token_expires_at`（会话的绝对期限）。

`instance.yaml` 的 `InstanceInfo` 加 `signup_enabled`；`instance` 模块从 `Deps` 拿到开关，声明 `GET /instance` 为公开操作。

`common.yaml` 的 `FieldError.code` 加 `common_password`，与 `shared` 的字段码集合一致。

### 3.9 整个程序的测试

| 测试 | 守住 |
|---|---|
| `TestPublicOperationsAreTheContractsPublicOperations` | 行为测试，对接好线的应用逐个操作请求：契约中 `security: []` 的操作不带令牌不被认证中间件拦下；其余操作不带令牌答中间件的 401 与 `WWW-Authenticate`。它测的是应用本身，比对两份清单做不到这一点 |
| `TestParametersThatDoNotBindAnswer400`、`TestBodiesThatBreakTheStructureAnswer400`、`TestTheAnswerToABrokenBodyStaysSmall` | 由契约推导的参数与请求体破坏用例：答 400，回答很小 |
| `TestEveryProblemResponseDeclaresItsHeaders` | 加上 `WWW-Authenticate` |
| `TestEveryKindBecomesItsProblem` | 401 带 `WWW-Authenticate` |

apitest 恢复两条写法规则：参数写 `schema` 不写 `content`；JSON 请求体是对象。

### 3.10 端到端

- `e2e/fixtures/auth.ts`：`emailFor`（按测试生成不重复的邮箱）、`register`、`bearer`。
- `e2e/fixtures/assert/identity.ts`：按表组织的断言：`expectNewAccount`、`expectNewSession`（代数 0、UA 与 IP 已记录、`expires_at - created_at` 正好 30 天、令牌哈希与刷新令牌一致）、`expectNothingAdded`。
- A1（注册）、A2（邮箱已占用 409、密码太弱 422、注册关闭 403：`nervewikiWith` 起一个关闭注册的服务）的接口版本；S3 的 `toEqual` 加上 `signup_enabled`。

## 4. 实施步骤

在分支 `m1-p1-identity-foundation` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 数据：两条迁移、sqlc 与 `sqlc.yaml`、`gen-go` 加入 sqlc、`sqlc_test`、`rawsql_test`、数据库测试 | [P1-S1-data.md](plans/P1-S1-data.md) |
| S2 | 凭证与领域规则：`shared` 的 Actor 与邮箱；identity 的领域；名单工具；signing、argon2 | [P1-S2-credentials.md](plans/P1-S2-credentials.md) |
| S3 | 认证接入与注册：配置、客户端 IP、认证中间件与 401；identity 的 register、getMe、authenticate 与契约；`instance` 的开关；组合根；公开操作与 401 的整个程序测试 | [P1-S3-auth-register.md](plans/P1-S3-auth-register.md) |
| S4 | 契约的整个程序测试：apitest 的参数与请求体用例、两条写法规则、三个整个程序测试 | [P1-S4-contract-tests.md](plans/P1-S4-contract-tests.md) |
| S5 | 端到端：fixtures、`assert/identity.ts`、A1、A2 的接口版本、S3 的调整；README | [P1-S5-e2e.md](plans/P1-S5-e2e.md) |

规模估计（参照 Nerve 的同类 Phase，扣掉裁剪）：生产代码约 2,500 行，测试约 4,000 行，另有生成代码约 1,200 行、常见密码名单一份。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 邮箱、由邮箱得出的显示名、密码规则（含名单、名单的主干与邮箱主干）；刷新令牌的编码与解析；JWT 的签发与各种无效令牌；argon2 的并发名额与超时；客户端 IP（可信代理、畸形条目、IPv4 映射）；列表键的解码 |
| 集成 | register 的事务（占用的邮箱、并发的同邮箱注册只有一个成功）；authenticate 的各种会话状态（撤销、过期、账户停用）；约束与索引名、CHECK 反例 |
| 契约 | identity 的 handler 测试答出声明的每个码；第 3.9 节的整个程序测试 |
| 架构 | `sqlc_test`、`rawsql_test` |
| 端到端 | A1、A2 的接口版本；M0 的故事照旧通过 |

反向对照（验证后撤销）：
- 契约里把 `getMe` 标成公开、模块却不在清单里 → 公开操作的测试失败；
- 认证中间件放过没有令牌的请求 → 401 的测试失败；
- identity 的查询读别的模块的表（例如 `goose_db_version`）→ sqlc 生成失败；
- 模块里直接 `pool.Exec` 一条 SQL → `rawsql_test` 失败；
- 迁移中去掉 `users_onboarding_steps_check` → 数据库测试失败；
- 常见密码名单为空 → 密码规则的测试失败；
- 注册关闭时先校验了请求体（顺序颠倒）→ A2 的 403 断言失败；
- 生产配置不给私钥 → 配置校验拒绝启动。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查完成（`reviews/P1-identity-foundation-review.md`），发现的问题已修复。
- M1 总设计进度表更新；M0 移交中本 Phase 的各项核对。

## 7. 结果

分支 `m1-p1-identity-foundation`：S1 `c7156c9`、S2 `9abc75a`、S3 `c99d8d7`、S4 `59d60a6`、S5 `1ab005e`，审查修复 `51bc900`。第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`（另跑 `--repeat-each 3 --workers 1`）、`make image-smoke` 本地与持续集成为绿。审查见 [P1 审查记录](reviews/P1-identity-foundation-review.md)：1 项 Important、4 项 Minor、7 项 Nit，处置见记录。

与设计的出入（已同步进上文）：

1. `Authenticator` 返回两个值，凭证键随 P2 的限流加入。
2. 撤销原因、续期的判定表、显示名与引导步骤 id 的输入校验推迟到用到它们的 Phase（P2、P3）。
3. 名单另收条目的主干（审查 M1）：只收 8 个字符以上的条目时，主干规则放过了 `Qwerty123!`、`Summer2024!` 这类最常见的修饰形式。名单从 46,483 条变为 67,396 条。
4. 客户端 IP 有三种告警（多一种畸形条目）；"收到头却没配置"只在没配置可信代理时告警（审查 M2）。
5. `TestParametersThatDoNotBindAnswer400` 在 P1 推导不出用例（没有路径参数）；"至少推导出一个用例"的守卫随 P3 的第一个路径参数加入。
6. 公开操作的整个程序测试是行为测试，没有单独的 `TestOperationsThatNeedATokenAnswer401WithoutOne`。
7. prod 的 `migrate up` 也要配置私钥路径：配置只有一条校验路径（只校验已设置，不读取）。
8. `onboarding_steps` 为 nil 时 HTTP 适配器答 `[]`。
9. 测试辅助 `startApp` 分开"服务已退出"与"已停止"两个信号：数据库不可达时原来的写法会挂住。
10. 密码按 NFKC 规范化之后哈希（审查 M4），访问令牌的 `exp` 向上取整到秒（审查 N4），CHECK 挡住含逗号的元素（审查 I1）。
