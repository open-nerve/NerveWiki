# M1/P3 账户、PAT 与停用：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M1/P3 账户、PAT 与停用 |
| 状态 | 已完成 |
| 基线 | P2 合并之后的 main |
| 上级文档 | [M1 总设计](00-M1-design.md) 第 3–8 节；[P1 文档](01-P1-identity-foundation.md)、[P2 文档](02-P2-sessions-ratelimit.md) 第 3 节；[总体设计](../v0.1-design.md) 6.1、6.2、12.4、13 |

---

## 1. 基线

P2 留下的：会话从登录到退出的完整生命周期（登录的账户行锁与快照重试、续期的判定表与条件轮换、退出，续期与退出的逐路由请求期限）；平台的限流与认证之前的失败闸门，凭证的限流键 `session:<id>`；模块的桶 `login_ip`、`login_ip_email`、`register_ip`；`clocktest`；`pgtest.WaitForLockWaits`（从本 Phase 提前到 P2）。需要令牌的操作只有 `getMe`，凭证只有访问令牌。

M0 移交给本 Phase 的（M1 总设计第 7 节）：[P4 接口契约](handoffs/M0-P4-api-contract.md)第 4 项（oapi-codegen runtime 的例外）。

前面的 Phase 推迟到这里的：
- P1 审查 N7：第一个路径参数出现时，`TestParametersThatDoNotBindAnswer400` 加"至少推导出一个用例"的守卫。
- P2：`password_user` 桶（第一批校验当前密码的已认证操作）；撤销原因的类型（随批量撤销）；停用账户的登录进入端到端（A3 的这一段）。

## 2. 目标与范围

**目标**：登录之后的账户操作齐全；PAT 成为第二种凭证，每个需要登录的操作都能用它调用；账户停用有了 M2 要挂接的扩展点，并用测试替身证明挂上去的注册者按约定工作。

**做**：
- 迁移 `api_tokens`；PAT 的格式、创建（要求当前密码）、列出、撤销；PAT 的认证与 `last_used_at`；限流键 `pat:<id>`。
- `updateMe`（显示名）、`recordOnboardingStep`、`changePassword`、`deactivateMe`。
- 账户行锁协议：`CredentialLock`（锁账户行，锁下复核调用方的凭证）；当前密码的校验与快照重试，由改密码与创建 PAT 共用。
- 停用的扩展点：否决者、事务内事件；增长路径的 `ShareActiveAccount`。
- `password_user` 桶；撤销原因的类型与批量撤销。
- oapi-codegen runtime 的例外；参数用例的守卫。
- 整个程序的测试：每个需要登录的操作都接受 PAT。
- 端到端：A7–A11 的 PAT 接口版本；A3 补上停用账户的登录。

**不做**：`nervewiki users` 命令与管理员的启用、重置密码（P4，A11 的启用部分随它进入端到端）；会话清理任务（P4）；前端（P5、P6）；M2 的真实注册者（唯一管理员检查等）。

## 3. 设计

### 3.1 文件

```
server/
  migrations/sql/00004_identity_api_tokens.sql
  internal/archtest/binary_test.go                    google/uuid 只允许由 oapi-codegen/runtime 导入
  internal/platform/config/                           ratelimit.password_user
  internal/modules/identity/
    module.go                                         Deps 加否决者、订阅者、password_user
    accounts.go                                       NewAccounts：ShareActiveAccount，只凭连接池构造；扩展点的类型别名
    domain/api_token.go                               PAT 的格式、解析、哈希；名称与期限的规则
    domain/name.go                                    名称的规则 CheckName（显示名与 PAT 名共用）
    domain/user.go                                    资料的补丁；引导步骤 id 的规则与个数上限
    domain/session.go                                 撤销原因 RevokeReason
    domain/errors.go                                  current_password_incorrect、api_token_not_found、account_not_found、引导步骤超限
    app/credential_lock.go                            CredentialLock
    app/current_password.go                           当前密码的校验与快照重试
    app/deactivation.go                               扩展点：Deactivation、DeactivationVetoer、DeactivationSubscriber
    app/update_me.go、record_onboarding_step.go、change_password.go、deactivate.go、accounts.go
    app/create_api_token.go、list_api_tokens.go、revoke_api_token.go
    app/authenticate.go                               PAT 的路径与 last_used_at
    app/ports_api_tokens.go                           PAT 的端口（ports.go 放账户与会话）
    adapter/postgres/api_tokens.go、queries/api_tokens.sql；users、sessions 的新查询
    adapter/http/me.go、api_tokens.go、limits.go
    adapter/authn/                                    PAT 的限流键
    interleavings_test.go                             账户行锁的确定性交错
    deactivate_test.go                                停用的注册者替身与 FOR SHARE 的交错
api/modules/identity.yaml                             七个操作
e2e/fixtures/auth.ts、assert/identity.ts；stories/identity/a7–a11，a3 补一段
```

### 3.2 PAT

- **格式**：`nwk_pat_` 加 32 个随机字节的 base64url（无填充），共 51 个字符。数据库只存整个令牌的 SHA-256：随机串有 256 位熵，不需要慢哈希。解析只接受这一种写法（前缀、43 个字符、末位未用的比特为零），不查库。
- **表 `api_tokens`**：`id`、`user_id`、`token_hash`（唯一，32 字节）、`name`、`expires_at`（空为永不过期，晚于 `created_at`）、`last_used_at`、`revoked_at`、`created_at`、`updated_at`。撤销记下 `revoked_at`、行留下（接口的 DELETE，但不是总体设计 7.1 那种会被清理的软删除，见 13.1 第 6 条）：认证能说出"已撤销"而不是"不存在"（只进调试日志）。列表用部分索引 `(user_id, created_at DESC, id DESC) WHERE revoked_at IS NULL`。
- **名称**：去掉两端空白之后 1–100 个字符，不含改变周围文字读法的字符：控制字符（Cc）、行与段落分隔符（会断行）、双向控制字符（能把名字倒着显示、冒充别人）；其他格式字符保留，emoji 序列与波斯文等要用连接符。同形字不由服务端处理。显示名用同一条规则（`CheckName`）。**期限**：可省略；给出时必须晚于当前时刻，按数据库的精度（UTC、微秒）截断之后再比较与存储，创建时的回答与之后的列表读到同一个时刻。
- **创建**（`POST /me/api-tokens`，任何凭证，PAT 也可以）：`{name, expires_at?, current_password}`。先查名称与期限（422），再按 3.4 校验当前密码，锁下插入。明文只在 201 的回答中出现一次；日志记 `user_id`、`token_id`。
- **列出**（`GET /me/api-tokens`）：未撤销的令牌，已过期的也列出（用户据此撤销），新的在前；不分页（总体设计 6.1 的小集合）。不设个数上限：创建要求密码，并受 `password_user` 限制。
- **撤销**（`DELETE /api-tokens/{token_id}`）：一条条件更新，只命中调用方自己未撤销的令牌；不存在、已撤销、别人的都答 404 `identity.api_token_not_found`，不透露令牌是否存在。请求所用的 PAT 可以撤销自己。
- **认证**：`nwk_pat_` 开头的令牌走 PAT 的路径：解析、按哈希查一次（连同账户是否可用）；不存在、已撤销、已过期、账户停用都答 401。过期的 PAT 不像访问令牌那样退还失败闸门的名额：它不能续期，是真正的失败。`last_used_at` 最多每分钟写一次（条件更新），被别的事务持有的行跳过、不等（`SKIP LOCKED`），写失败只记 warn 与 `token_id`，令牌照样通过；使用不改 `updated_at`。限流键 `pat:<id>`。

### 3.3 账户行锁协议

P2 的登录已经先锁账户行（`FOR NO KEY UPDATE`）。本 Phase 把它推广为 `CredentialLock`：每个签发或改变凭证、且由调用方的凭证发起的事务（改密码、创建 PAT、停用），第一条语句锁账户行，然后在锁下复核：账户可用，调用方的会话未撤销未过期，或调用方的 PAT 未撤销未过期；否则 401。

- **为什么要复核**：认证发生在事务之前。改密码撤销了别的会话之后，被撤销的会话上还在途的"创建 PAT"会等到锁，复核时看到自己的会话已撤销，不会留下一个由已失效凭证创建的永久凭证。P4 的管理员重置密码同理。
- **全局加锁顺序** `users → auth_sessions → api_tokens`：事务里第一个锁总是账户行。续期与退出只碰自己的会话行、撤销 PAT 与 `last_used_at` 只碰一个 PAT 行，都不持有别的锁，不会与之交叉成环。
- 账户行锁返回锁下读到的邮箱、哈希、是否可用：停用事件的邮箱、快照的比较都取它。

### 3.4 当前密码

改密码与创建 PAT 共用 `CurrentPassword`：

1. 事务外按快照（事务之前读到的哈希）校验当前密码；错误答 422 `identity.current_password_incorrect`。不是 401：前端的令牌管理器会把 401 当作会话失效。
2. 事务外做只需做一次的准备（改密码在这里哈希新密码）。
3. 一个事务：`CredentialLock`，核对锁下的哈希仍等于快照，执行写入。
4. 哈希在两步之间变了（并发的登录重新哈希了，或密码被改了）：对新哈希再校验一次、再做第 3 步；第二次还变就答 `current_password_incorrect`。

与登录的快照重试同构（P2 3.4）；两处的循环各自保留：登录没有调用方的凭证，也不做复核。每个请求在 HTTP 层先从 `password_user`（按账户，不分凭证）取一个单位，早于任何校验，答 422 的也计数：拿到访问令牌或 PAT 的人不能借这两个操作无限次猜密码，持有多个凭证也不多得。拒绝的日志记账户。

### 3.5 账户操作

- **`updateMe`**（`PATCH /me`）：`{display_name?}`，规则同 PAT 的名称（3.2，422）；空的补丁不写库，返回当前账户。一条语句，不开事务。回答完整的 `User`。
- **`recordOnboardingStep`**（`POST /me/onboarding-steps`）：`{step}`，id 的格式由领域检查（小写字母开头，字母、数字、下划线，最长 32）。一条语句：已记录的不改（`updated_at` 也不动），否则追加。个数上限由数据库的 CHECK 裁决（并发的追加也不会越过），仓储把这个约束的违反译成 422 `validation_failed`（`step`，`out_of_range`）：领域先检查了格式，且与 CHECK 的格式一致，违反只能来自个数。回答完整的 `User`。
- **`changePassword`**（`POST /me/change-password`）：`{current_password, new_password}`。新密码先按密码规则检查（邮箱取自账户，422 在 `new_password` 上），再按 3.4 校验当前密码；锁下写入新哈希，撤销其他会话（`password_changed`）：调用方是会话时保留它，是 PAT 时撤销全部会话。PAT 不受影响（M1 总设计第 4 节）。204。
- **撤销原因** `domain.RevokeReason` 覆盖 `auth_sessions_revoke_reason_check` 的六个值；批量撤销 `RevokeSessions(user, keep, reason, now)` 只撤销未撤销、未过期的会话，返回个数。续期与退出的两条语句仍在 SQL 中写明各自的原因。

### 3.6 停用与扩展点

`POST /me/deactivate`（任何凭证，不要求密码：停用可由管理员恢复，会话之外什么也不销毁）。一个事务：

```
CredentialLock（锁账户行、复核凭证）
→ 否决者依次 VetoDeactivation(ctx, d)       任何一个返回错误：整体回滚，回答它
→ 写入：is_active = false；撤销全部会话（deactivated）
→ 订阅者依次 AccountDeactivated(ctx, d)      任何一个返回错误：整体回滚
提交；记 user_id、revoked_sessions
```

- `Deactivation{UserID, Email, At}`：邮箱取自锁下读到的行，`At` 是用例的时刻。否决者与订阅者收到同一个值。
- 否决者返回 `*shared.Error` 时接口答出它的码；注册者把自己的码追加到 `deactivateMe` 的 `x-problem-codes`（M2）。其他错误是故障（500），同样回滚。
- 注册者在同一事务内运行：它们只凭连接池构造，语句经 `postgres.DB(ctx, pool)` 进入上下文里的事务；HTTP 与 P4 的命令行共用同一组注册者。
- **不变量**：停用撤销账户的全部会话。续期不读 `users.is_active`，撤销是会话失效的唯一依据（P2 审查 N3）：P4 的 `users activate` 之后旧会话不会复活，只有 PAT 恢复。
- PAT 保留：账户停用期间认证拒绝它们，P4 的 `users activate` 之后恢复。引导步骤保留。
- **类型的位置**：接口与 `Deactivation` 在 `app`（用例依赖它们），模块根以类型别名公开；M2 的模块不导入 identity（架构测试"模块互不导入"），由 bootstrap 把它们的实现适配成这些接口（照 Nerve `bootstrap/ports.go`）。
- **`ShareActiveAccount(ctx, userID)`**（`identity.NewAccounts(pool)`，只凭连接池构造）：对账户行取 `FOR SHARE` 并确认可用：停用答 403 `identity.account_deactivated`，不存在答 404 `identity.account_not_found`。任何让账户获得新访问的写事务（M2 的加入工作区、接受邀请）以它作为第一条语句：`FOR SHARE` 与停用的 `FOR NO KEY UPDATE` 冲突，两者串行，否决者总能看到已提交的成员关系；两个 `FOR SHARE` 互不等待。

### 3.7 配置

```yaml
ratelimit:
  password_user: {per_minute: 5, burst: 5}   # 校验当前密码的已认证操作（改密码、创建 PAT），按账户
```

test 配置同其他桶，调到用不完。

### 3.8 接口

`api/modules/identity.yaml` 增加：

| 操作 | 请求体 | 成功 | 码 |
|---|---|---|---|
| `PATCH /me`（`updateMe`） | `{display_name?}` | 200 `User` | `validation_failed` |
| `POST /me/onboarding-steps`（`recordOnboardingStep`） | `{step}` | 200 `User` | `validation_failed` |
| `POST /me/change-password`（`changePassword`） | `{current_password, new_password}` | 204 | `validation_failed`、`identity.current_password_incorrect`、`server_busy` |
| `POST /me/deactivate`（`deactivateMe`） | — | 204 | —（否决者追加） |
| `GET /me/api-tokens`（`listApiTokens`） | — | 200 `{data: ApiToken[]}` | — |
| `POST /me/api-tokens`（`createApiToken`） | `{name, expires_at?, current_password}` | 201 `ApiTokenCreated` | `validation_failed`、`identity.current_password_incorrect`、`server_busy` |
| `DELETE /api-tokens/{token_id}`（`revokeApiToken`） | — | 204 | `identity.api_token_not_found` |

- `ApiToken`：`id`、`name`、`expires_at`（null 为永不过期）、`last_used_at`（null 为从未使用）、`created_at`；`ApiTokenCreated` 另有 `token`，只此一次。
- `token_id` 是第一个路径参数：生成代码开始导入 `oapi-codegen/runtime`，它把 `github.com/google/uuid` 带进二进制。`binary_test` 的禁用判断改为按导入者：只有 runtime 及其子包可以导入它，其他路径照旧报出（照 Nerve `isBannedFromBinary`）。`generated_test` 已保证生成代码不用 runtime 的 `UUID` 类型。
- `TestParametersThatDoNotBindAnswer400` 在推导出 0 个用例时失败（P1 审查 N7）。

### 3.9 集成与整个程序的测试

| 测试 | 守住 |
|---|---|
| 整个程序：契约中每个需要令牌的操作，用新账户的有效 PAT 调用，回答不是 401 | 每个需要登录的操作都接受 PAT |
| PAT 的认证：不存在、已撤销、已过期（`clocktest`）、账户停用都答 401；`last_used_at` 首次使用写入，一分钟内不再写，之后再写；被持有的行跳过（仓储）。写失败时仍然通过由用例的单元测试守住 | 3.2 |
| 撤销：别人的、已撤销的、不存在的答同一个 404；自己的撤销之后答 401 | 3.2 |
| 改密码：会话调用时保留当前会话、撤销其他（`password_changed`）；PAT 调用时撤销全部；PAT 不受影响；旧密码不能再登录 | 3.5 |
| 交错一：登录持有账户行锁（在插入会话之前停住），改密码等待它（`WaitForLockWaits`），放行之后改密码撤销登录刚建的会话 | 改密码撤销的范围包括登录刚建的会话（改密码的 UPDATE 本身也等登录的锁：它守不住显式锁） |
| 交错二：改密码持有锁（在撤销会话之前停住），被撤销的会话发起的创建 PAT 等待它，放行之后答 401，没有插入令牌 | 锁下先复核凭证、再比较快照 |
| 会话在认证之后、事务之前结束（退出不取账户锁）：创建 PAT 答 401，没有插入令牌 | `CredentialLock` 的复核 |
| 交错三：改密码校验当前密码之后、事务之前，并发的登录重新哈希了密码：改密码对新哈希再校验一次，然后成功 | 当前密码的快照重试 |
| 交错四：两个改密码，后者用 PAT（不被前者撤销）；前者持锁停在撤销之前，后者等锁，放行之后后者的当前密码对新哈希不成立，答 422，哈希是前者的 | 账户行锁：没有它，后者只在 UPDATE 处等待，之后覆盖前者的哈希 |
| 停用：否决者返回 `*shared.Error` → 接口答出它的码，账户、会话都不变；订阅者在事务内看到 `is_active = false`；订阅者失败 → 整体回滚 | 3.6 的顺序与事务 |
| 停用之后：访问令牌、刷新令牌（续期）、PAT 都答 401，登录答 403 | 3.6 的不变量 |
| `FOR SHARE`：持有 `ShareActiveAccount` 的事务写入一个标记，停用等待它（`WaitForLockWaits`），提交之后停用的否决者看到标记而否决；反过来停用先持锁，`ShareActiveAccount` 等到提交之后答 `account_deactivated` | 增长路径与停用串行 |
| 引导步骤：幂等；格式错误 422；第 33 个不同的步骤 422 | 3.5 |
| 迁移：`api_tokens` 的约束与索引名、CHECK 的反例（P1 的 `schema_test` 照旧覆盖） | 表的约束 |

测试替身的否决者与订阅者只凭连接池构造，与 M2 的注册者相同。

### 3.10 端到端

- `e2e/fixtures/auth.ts` 增加 `createToken`、按 PAT 调用的辅助；`assert/identity.ts` 增加 `api_tokens` 的断言（新令牌的哈希与字段、撤销、`last_used_at`）与按原因的会话撤销。
- A7（改密码：PAT 调用之后全部会话失效、PAT 照常可用、新密码登录）、A8（改显示名，422）、A9（记录引导步骤，幂等）、A10（创建要求当前密码、明文只出现一次、列表不含明文、`last_used_at`、撤销与过期之后 401）、A11（停用：会话失效、PAT 答 401、登录答 403 `identity.account_deactivated`）的 PAT 接口版本；A3 补上停用账户的登录。

## 4. 实施步骤

在分支 `m1-p3-accounts-tokens` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | PAT 的数据与认证：迁移、领域、查询、认证的 PAT 路径与 `last_used_at`、限流键；撤销原因；`CredentialLock` | [P3-S1-tokens-data.md](plans/P3-S1-tokens-data.md) |
| S2 | PAT 的操作：当前密码、创建、列出、撤销；`password_user`；契约与 runtime 的例外、参数用例的守卫；整个程序的 PAT 测试 | [P3-S2-token-operations.md](plans/P3-S2-token-operations.md) |
| S3 | 账户操作：`updateMe`、引导步骤、改密码；交错一至三 | [P3-S3-account.md](plans/P3-S3-account.md) |
| S4 | 停用与扩展点：否决者、订阅者、`ShareActiveAccount`；测试替身与 `FOR SHARE` 的交错 | [P3-S4-deactivation.md](plans/P3-S4-deactivation.md) |
| S5 | 端到端：A7–A11 的 PAT 接口版本、A3 的停用一段；README | [P3-S5-e2e.md](plans/P3-S5-e2e.md) |

规模估计：生产代码约 1,500 行，测试约 3,200 行。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | PAT 的格式与解析（每种畸形）；名称、期限、显示名、引导步骤 id 的规则；撤销原因与 CHECK 一致；各用例对端口的调用顺序与错误 |
| 集成 | 第 3.9 节 |
| 契约 | 七个新操作的 handler 测试答出每个声明的码；P1 的整个程序测试覆盖新操作，参数用例不再为空 |
| 架构 | google/uuid 只经 runtime 进入二进制 |
| 端到端 | A7–A11 的 PAT 接口版本、A3 的停用一段；之前的故事照旧通过 |

反向对照（验证后撤销）：
- `CredentialLock` 不复核凭证 → 会话在途结束的测试失败（插入了令牌），交错二答 422 而不是 401；
- 改密码与创建 PAT 的锁换成不加锁的读取（登录照旧加锁）→ 交错四失败（后者覆盖了前者的哈希）；交错一照样通过；
- 停用的订阅者在提交之后才调用 → 订阅者失败回滚的测试失败；
- `ShareActiveAccount` 不取 `FOR SHARE` → `FOR SHARE` 的交错失败；
- PAT 的认证不看账户是否可用 → 停用之后 PAT 仍通过；
- 撤销不带 `user_id` 条件 → 撤销别人令牌的测试失败；
- runtime 的例外按被导入的路径而不是按导入者判断 → `TestBannedImports` 失败；
- 契约去掉路径参数 → 参数用例的守卫失败；
- `password_user` 按凭证计数 → 限流测试失败；
- 名称只挡 Cc → 名称测试失败；
- 失败的 PAT 退还闸门的名额 → A14 失败；
- `last_used_at` 等待被持有的行 → 仓储测试以期限超时失败。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查完成（`reviews/P3-accounts-tokens-review.md`），发现的问题已修复。
- M1 总设计进度表更新；M0 移交中本 Phase 的一项核对。

## 7. 结果

分支 `m1-p3-accounts-tokens`：S1 `fd58259`、S2 `de3a8d2`、S3 `631d5db`、S4 `ba07d71`、S5 `1b6b22b`，审查修复 `12ebddb`。第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`（另跑 `--repeat-each 3` 与 `--workers 1`）、`make image-smoke` 本地与持续集成为绿。审查见 [P3 审查记录](reviews/P3-accounts-tokens-review.md)：3 项 Minor、6 项 Nit，处置见记录。规模与估计相符：生产代码（Go 与 SQL，不含生成物）约 1,500 行，测试约 3,100 行，端到端约 440 行。

与设计的出入（已同步进上文）：

1. 新增"会话在认证之后、事务之前结束"的测试：真正需要锁下复核的是它。不复核时交错二并不插入令牌，而是在快照的比较处答 422。
2. 交错一守不住显式的账户行锁（改密码的 UPDATE 本身就等登录的锁），审查之后新增交错四（两个改密码，丢失更新），交错一改为守撤销的范围（审查 M1）。
3. `CreateAPIToken` 的仓储查询在 S1 加入：S1 的认证测试用它插入令牌。
4. 持锁的测试都有期限：释放的等待 10 秒，握手经 `awaitHolding`（审查 N3），P2 的 `TestLockForCredentials` 一并改过。
5. 引导步骤的 CHECK 违反映射为 `ErrTooManyOnboardingSteps`（422，`step`，`out_of_range`）。
6. 停用的拒绝也记日志（info，带码），成功记 `revoked_sessions`。
7. 注册与改密码共用一个 `domain.NewPasswordRules()`（名单只切分一次）；组合根先构造 `CredentialLock`，分别交给当前密码与停用（审查 N6）。`New` 已约 95 行，P4 加管理用例时把用例的装配抽成辅助函数。
8. 名称另拒绝行与段落分隔符、双向控制字符（审查 M3）；`last_used_at` 跳过被持有的行（审查 N2）；`password_user` 的拒绝记账户（审查 N4）。
9. 端到端比 3.10 多几段：A7 先用访问令牌改（保留本会话）、再用 PAT 改（撤销全部）；A10 另测别的账户看不到、撤销不了，过去的期限答 422；A3 另测停用账户的错误密码答 401（先校验密码，不透露账户已停用）；A14 的失败闸门用一个已撤销的 PAT（审查 N1）。
