# M2/P3 邀请与带邀请注册：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P3 邀请与带邀请注册 |
| 状态 | 已完成 |
| 基线 | `19b4710`（P2 合并、P2 文档更新之后的 main） |
| 上级文档 | [M2 总设计](00-M2-design.md) 第 4、5、8、9 节；[P2 文档](02-P2-workspace-members.md)第 7 节；[M1 移交](handoffs/M1-identity.md)第 3、4 项；[总体设计](../v0.1-design.md) 3.2、12.4、13.1 |

---

## 1. 基线

P2 留下的：
- 工作区的改名、删除，成员列表、改角色、移出、离开；唯一管理员规则一。
- 成员身份结束（否决者与事件）、工作区删除（事件）两个扩展点；`MembershipEnder` 是移出与离开共用的一步。
- 三个确定性的交错（经接好线的应用、测试持锁）；矩阵的"已结束""已删除"两列经接口准备。
- 成员行仍只能由 SQL 写入：矩阵与交错的种子、W8、W9 都等着邀请。

本 Phase 接手：
- M1 移交第 3 项的后一半：`ShareActiveAccount` 交回锁下读到的邮箱，接受邀请拿它与邀请的邮箱比较。
- M1 移交第 4 项：注册策略的端口带上注册邮箱与可选的邀请，`register` 的请求体加 `invitation`。
- P2 第 7 节：恢复的成员关系的"加入时刻"（审查 Q3，见 3.5）。

## 2. 目标与范围

**目标**：管理员按邮箱邀请，复制链接发给对方；被邀请人预览、接受，注册关闭时也能凭邀请注册。成员从此经用户能走的路径加入。

**做**：
- 迁移 `workspace_invitations`；派生的 MAC 密钥与邀请令牌。
- `listWorkspaceInvitations`、`createWorkspaceInvitation`、`deleteWorkspaceInvitation`、`previewWorkspaceInvitation`（公开）、`acceptWorkspaceInvitation`。
- 成员身份恢复事件。
- 结束成员关系时删除这个工作区里发给他邮箱的待接受邀请；删除工作区连带邀请。
- 注册策略与 `register` 的请求体；组合根组合"配置开关"与 workspace 的邀请检查。
- 规则表三行；矩阵的新行（预览是公开的、接受按令牌与邮箱判定，列入豁免）。
- 交错 4–8。
- e2e：W5–W9 的接口版本；W4 的"非管理员答 403"与连带邀请。

**不做**：停用的注册者、`reactivate-member`、清理（P4）；页面（P5、P6）。

## 3. 设计

文件：

```
server/
  migrations/sql/00008_workspace_workspace_invitations.sql；schema_test；deploy/runtime-grants.sql
  internal/shared/email.go                     CheckEmail（从 identity/domain 移来）
  internal/platform/postgres/pgtest/lockwait.go WaitForKeyWaitOn
  internal/modules/identity/
    module.go                                  LoadSigningKeys(pem, logger)；Deps.SigningKeys
    accounts.go、app/accounts.go                ShareActiveAccount 交回邮箱
    directory.go                               NewDirectory：公开资料、按邮箱查账户（P2 的 profiles.go 改名）
    app/ports.go、app/register.go               SignupPolicy 的 SignupAttempt；请求体的 invitation
    adapter/signing/keys.go                    Derive(info)
  internal/modules/workspace/
    module.go                                  Deps 加 InvitationKey、Directory、恢复事件的订阅者；PublicOperations()
    invitations.go                             NewInvitationCheck(pool, key)：注册策略用；InvitationKeyInfo
    domain/invitation.go、actions.go、errors.go  邀请的规则、三个操作名、两个码与两个 422
    app/{list,create,delete,preview,accept}_invitation(s).go  五个用例；check_invitation.go 注册策略的检查；
                                               invitations.go 带令牌的邀请；ports.go 加端口；extension.go 加恢复事件
    app/end_membership.go、delete_workspace.go  连带邀请
    adapter/mac/                               邀请令牌
    adapter/postgres/queries/invitations.sql   仓储
    adapter/http/invitations.go                五个操作
  internal/modules/access/domain/rules.go      三行
  internal/bootstrap/
    wire.go                                    先加载签名密钥，派生邀请密钥，再建 identity 与 workspace
    signup.go                                  注册策略：开关或邀请
    directory.go                               identity 的目录转成 workspace 的两个端口（P2 的 profiles.go 改名）
    interleavings_invitations_test.go          交错 4–8
    permission_matrix_*                        新行、豁免、种子的邀请
api/modules/workspace.yaml、identity.yaml、openapi.yaml
web/apps/web/src/app/problem-messages.ts、i18n
e2e/fixtures/invitations.ts；stories/workspace/w5–w9
```

### 3.1 数据

**`workspace_invitations`**：

| 列 | 类型与约束 |
|---|---|
| `id` | `uuid PRIMARY KEY` |
| `workspace_id` | `uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE` |
| `email` | `varchar(255) NOT NULL`，CHECK 与 `users.email` 相同：规范化之后的邮箱 |
| `role` | `text NOT NULL`，CHECK 三种角色 |
| `accepted_at` | `timestamptz`：接受的时刻，与 `deleted_at` 相同；管理员删除、成员关系结束、工作区删除时为空 |
| `created_by_id`、`updated_by_id` | `uuid NOT NULL REFERENCES users` |
| `created_at`、`updated_at` | `timestamptz NOT NULL` |
| `deleted_at` | `timestamptz`：不再待接受的时刻（接受、删除） |

- 部分唯一索引 `workspace_invitations_workspace_id_email_key ON (workspace_id, email) WHERE deleted_at IS NULL`：一个工作区里一个邮箱至多一份待接受的邀请（422 `email: duplicate`）。成员关系结束时按（工作区，邮箱）删除也用它。
- `workspace_invitations_workspace_id_idx ON (workspace_id)`：物理清除的外键连带。
- CHECK `workspace_invitations_accepted_check`：`accepted_at IS NULL OR (deleted_at IS NOT NULL AND accepted_at = deleted_at)`。不写 `deleted_at IS NOT NULL` 时，`deleted_at` 为空的行让等式为 NULL，CHECK 照样放行（迁移的反例测试抓到）。
- 加锁顺序：`users → workspaces → workspace_invitations → workspace_members`（M2 总设计第 8 节）。

### 3.2 邀请令牌

- identity 的签名密钥派生出邀请的 MAC 密钥：`HKDF-SHA256(种子, info "nervewiki workspace-invitation mac v1")`，与刷新令牌的 MAC 同一个办法、不同的 info（info 是 workspace 模块根的 `InvitationKeyInfo`）。签名密钥由组合根先加载（`identity.LoadSigningKeys`，坏的私钥在连数据库之前就报错），交给 identity 的 `Deps.SigningKeys`；组合根用它的 `Derive` 派生邀请密钥，交给 workspace 的 `Deps.InvitationKey` 与注册策略的 `NewInvitationCheck`。不由 identity 建好之后再取：注册策略是 identity 的 Dep，它要的密钥若等 identity 建好才有，就成了环。
- `workspace/adapter/mac`：`Tokens{key}`，`Token(id) = "nwk_inv_" + base64url(HMAC-SHA256(key, id)[:16])`，`Valid(id, token)` 严格解码（最后一个字符未用的位必须为零，一个令牌只有一种拼法，与 identity 的令牌一致），以 `hmac.Equal` 比较。app 经端口 `InvitationTokens` 用它：密钥只在组合根与这个适配器之间传递。
- 令牌不存库；列出邀请时重新计算，管理员随时可以复制。换签名密钥之后待接受的链接全部失效（M2 总设计第 4 节）。dev 与 test 不配私钥时每次启动生成临时密钥，邀请链接随之失效。
- 令牌不对与邀请不存在一样答 404 `workspace.invitation_not_found`；令牌在任何查询之前核对。

### 3.3 操作

| 操作 | 认证 | 依次 |
|---|---|---|
| 列出 `GET /workspaces/{slug}/invitations` | Bearer | slug 的格式 → 按 slug 读工作区 → 判定 `workspace_invitation.list`（管理员）→ 读待接受的邀请，新的在前 → 带上令牌 |
| 创建 `POST /workspaces/{slug}/invitations` `{email, role}` | Bearer | slug → 事务：工作区 `FOR SHARE` → 判定（管理员）→ 邮箱与角色的规则（422）→ 邮箱属于一个有效成员 → `email: not_allowed` → 插入；唯一索引冲突 → `email: duplicate` |
| 删除 `DELETE /workspace-invitations/{workspace_invitation_id}` | Bearer | 读邀请（不加锁）→ 事务：工作区 `FOR SHARE`（按 id）→ 邀请 `FOR UPDATE` 重读 → 判定（管理员）→ 软删除 |
| 预览 `POST /workspace-invitations/{workspace_invitation_id}/preview` `{token}` | 公开 | 令牌 → 读邀请与它的工作区（都未删除）→ `{workspace: {name, slug}, role}`，不含邮箱 |
| 接受 `POST /workspace-invitations/{workspace_invitation_id}/accept` `{token}` | Bearer | 令牌 → 读邀请（不加锁，得到工作区）→ 事务：`ShareActiveAccount(调用者)` 交回锁下的邮箱 → 工作区 `FOR NO KEY UPDATE` → 邀请 `FOR UPDATE` 重读 → 邮箱不符 403 `workspace.invitation_email_mismatch` → 成员关系：有效的不变（只消费邀请）；已结束的恢复（角色取邀请的，发布恢复事件）；没有的插入 → 消费邀请（`accepted_at = deleted_at`）→ 回答 `Workspace` |

- 接受不经规则表：调用者还不是成员，令牌与邮箱就是凭据（M2 总设计第 9 节"按令牌与邮箱判定"）。矩阵把接受与预览列为豁免，写明理由。
- 邀请已是有效成员的邮箱，答 422 `email: not_allowed`：先经 identity 的目录（3.6）按邮箱查到账户 id（不加锁），再看它在这个工作区有没有有效的成员行（仓储的 `FindMembership`，创建与接受共用；P4 起它带回 `ended_at`，由 `Member.Active()` 判断，见 [P4 文档](04-P4-deactivation-commands-purge.md) 3.3）。工作区的 `FOR SHARE` 让成员关系在这期间不变；没有账户不是问题，邀请就是给还没有账户的人的。
- 邮箱的规则只有一份：identity 领域的 `checkEmail` 移到 `shared.CheckEmail`（第二个使用者出现了，与 P1 的 `CheckName` 同理）；规范化用已在 `shared` 的 `NormalizeEmail`。
- 创建与删除持工作区的 `FOR SHARE`：两位管理员可以同时邀请（交错 7 由唯一索引决定先后），而成员关系的变化（`FOR NO KEY UPDATE`）与之串行。

### 3.4 成员关系结束与工作区删除连带邀请

- `MembershipEnder` 在否决者之后、写成员行之前，删除这些工作区里发给这个账户邮箱的待接受邀请（加锁顺序：邀请在成员之前）。邮箱经 identity 的目录读出（不加锁）：调用方锁住的是工作区行，不是账户行；与 `users set-email` 并发时，旧邮箱的邀请留着，只有旧邮箱新的持有者能接受，这正是邀请的语义。
- `DeleteWorkspace` 在成员行之前删除工作区的全部待接受邀请，同一个时刻。
- 目的：被移出的人不能凭先前发出、尚未接受的邀请回到工作区（交错 6）。

### 3.5 成员身份恢复：扩展点

```go
type MembershipRestore struct {
    WorkspaceID, UserID uuid.UUID
    Role                shared.WorkspaceRole
    By                  uuid.UUID
    At                  time.Time
}
type MembershipRestoreSubscriber interface {   // 恢复的写入之后、同一事务
    MembershipRestored(ctx context.Context, r MembershipRestore) error
}
```

接受恢复已结束的行时触发；P4 的 `reactivate-member` 也触发。本 Phase 没有注册者；替身测试证明它在同一事务、失败时整体回滚。

恢复改的是同一行：清空 `ended_at`，角色取邀请的，写 `updated_by_id`、`updated_at`；`created_at` 不变（P2 审查 Q3）。审计列只记这一行何时、由谁建立，成员列表"按加入的先后"因此按第一次加入排序。契约里 `WorkspaceMember.created_at` 的描述改为"第一次加入的时刻；恢复的成员关系保留它"。另加一列"最近一次加入"没有使用者。

### 3.6 注册策略（M1 移交第 4 项）

- identity：`SignupPolicy.AllowSignup(ctx, SignupAttempt{Email, Invitation *SignupInvitation{ID, Token}}) (bool, error)`。`Email` 是规范化之后的。`register` 的请求体加可选的 `invitation {id, token}`。开关之前的检查顺序不变：策略仍是第一步。
- workspace：`workspace.NewInvitationCheck(pool, key)`：`Admits(ctx, id, token, email)`，令牌对、邀请待接受、工作区未删除、邮箱相同才是 true。不加锁：注册不替用户接受，页面接着调用接受，接受在锁下再查一遍。
- identity 的目录：P2 的 `identity.NewProfiles` 改名 `NewDirectory`，在按 id 读公开资料之外，加按邮箱查账户 id（`AccountIDByEmail`）：别的模块对账户的不加锁读取都在这里。workspace 在消费方声明两个端口（`MemberProfiles`、`AccountFinder`），组合根的适配器把目录转过去。
- 组合根：`signupPolicy{open: cfg.Auth.SignupEnabled, invitations: workspace.NewInvitationCheck(...)}`：开着就允许；关着时，带了邀请且 `Admits` 才允许。其余一律 403 `identity.signup_disabled`。
- `GET /instance` 的 `signup_enabled` 不变：它说的是公开注册。

### 3.7 接口

在 `api/modules/workspace.yaml` 中新增五个操作：

| 操作 | 成功 | `x-problem-codes` |
|---|---|---|
| `listWorkspaceInvitations` | 200 `{data: WorkspaceInvitation[]}` | `workspace.not_found`、`forbidden` |
| `createWorkspaceInvitation`，`{email, role}` | 201 `WorkspaceInvitation` | `workspace.not_found`、`forbidden`、`validation_failed` |
| `deleteWorkspaceInvitation` | 204 | `workspace.invitation_not_found`、`forbidden` |
| `previewWorkspaceInvitation`，`{token}`（公开） | 200 `InvitationPreview` | `workspace.invitation_not_found` |
| `acceptWorkspaceInvitation`，`{token}` | 200 `Workspace` | `workspace.invitation_not_found`、`workspace.invitation_email_mismatch`、`identity.account_deactivated` |

- `WorkspaceInvitation {id, email, role, token, created_at}`；`InvitationPreview {workspace: {name, slug}, role}`。
- 预览是 workspace 模块第一个公开操作：模块根加 `PublicOperations()`，组合根把它并进接口的公开操作。
- `WorkspaceMember.created_at` 的描述改为"第一次加入的时刻"（3.5）。
- identity：`register` 的请求体加可选的 `invitation`。
- 公开的预览按 IP 计数（`anonymous`）：令牌是 128 位的 MAC，没有可猜的空间。

### 3.8 规则表

`workspace_invitation.list`、`workspace_invitation.create`、`workspace_invitation.delete`：管理员。

### 3.9 确定性的交错

沿用 P2 的办法（测试持锁，PostgreSQL 按到达的先后交出行锁）。交错 7 等的是唯一索引上的插入者：两位管理员都持工作区的 `FOR SHARE`，互不阻塞，后到的插入等先到的事务结束，拷贝 Nerve 的 `pgtest.WaitForKeyWaitOn`，改为本仓库的计数版。要让先到的停在插入之后、提交之前，用例里不留缝：测试数据库里建一个只在测试中存在的 `AFTER INSERT` 触发器，让插入停在一张测试表的行锁上（测试持有它）。测试自己先插入同一个键不行：两个插入都等测试的事务，醒来之后谁先插入是竞争。交错 8 持的是账户行：接受的第一条语句 `FOR SHARE` 与 `users set-email` 的 `FOR NO KEY UPDATE` 都在 `users` 上排队；改邮箱经 `bootstrap.Users` 在进程内运行。

| # | 交错 | 先后与结果 |
|---|---|---|
| 4 | 接受邀请与删除工作区 | 接受先：成为成员，随后删除连带他；删除先：接受锁工作区读到 0 行，404 `invitation_not_found` |
| 5 | 接受邀请与删除邀请 | 接受先：200，删除 404；删除先：接受 404 |
| 6 | 接受邀请与移出他（他已是成员、另有一份发给他的待接受邀请） | 接受先：角色不变、邀请消费，随后移出；移出先：邀请随之删除，接受 404 |
| 7 | 两位管理员同时邀请同一邮箱 | 先到的 201，后到的等唯一索引，之后 422 `email: duplicate` |
| 8 | 接受邀请与 `users set-email` | 改邮箱先：接受在锁下读到新邮箱，403 `invitation_email_mismatch`；接受先：200，改邮箱随后 |

### 3.10 矩阵与种子

- 新的行：列出、创建、删除邀请（管理员 200/201/204，成员与访客 403，其余 404：列出与创建 `workspace.not_found`，删除 `workspace.invitation_not_found`）。删除指向种在本列工作区里的一份邀请（种子加邀请，id 预先定好，`workspaceOfRow` 认得它）。
- 豁免：预览列入 `public`；接受是新的一类"凭所持判定"：调用者还不是成员，令牌与邮箱就是凭据。覆盖检查的豁免加这一类，各写理由，并补反例。
- 种子：工作区与成员行仍由 SQL 插入。覆盖检查不连数据库，要在准备之前就知道每一行指向的 id（P1 3.10）；经接口加入的成员关系，id 由服务端生成。矩阵测的是判定，成员怎样加入由 W6 与交错 4–6 的接口路径证明。P2 文档 3.10 说"P3 有了邀请之后再换掉"，按此改判，第 7 节记下。交错的种子同理。

### 3.11 端到端

- W5：管理员邀请、列出（含令牌）、删除；已是成员、已有邀请答 422；非管理员答 403；数据库里没有令牌。
- W6：被邀请人预览、接受，之后进入工作区；登录邮箱不符答 403；已是成员时角色不变；邀请已删除、令牌不对答 404；离开之后凭新的邀请回来，保留第一次加入的时刻。
- W7：注册关闭时，凭邀请用被邀请的邮箱注册并接受；别的邮箱答 403 `identity.signup_disabled`。
- W8：成员列表（访客看不到邮箱）；管理员改角色，不能改自己的。
- W9：管理员移出成员；成员离开（接口版本里离开的是访客）；唯一管理员不能离开；成员关系结束时发给他的待接受邀请被删除；之后他看不到这个工作区。
- "有效成员同时有一份发给他邮箱的待接受邀请"（W6 的"已是成员"、W9 的连带删除、交错 6）在 P3 里只有一条用户能走的路：先邀请一个地址，管理员再用 `users set-email` 把成员的邮箱改成它。创建会拒绝有效成员的邮箱，唯一索引不许第二份待接受的邀请；P4 的 `reactivate-member` 是另一条路。
- W4 补上：非管理员改名、删除答 403；删除连带邀请。

## 4. 实施步骤

分支 `m2-p3-invitations`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 数据与令牌：迁移、grants、schema 测试；`shared.CheckEmail`；identity 的 `Derive`、`adapter/mac`；领域的邀请规则、错误、操作名与规则表三行；仓储；`pgtest.WaitForKeyWaitOn` | [P3-S1](plans/P3-S1-data-tokens.md) |
| S2 | identity：`ShareActiveAccount` 交回邮箱、目录、注册策略与请求体；workspace 的 `InvitationCheck`；组合根的策略 | [P3-S2](plans/P3-S2-identity-signup.md) |
| S3 | 五个用例、恢复事件、成员关系结束与工作区删除连带邀请；契约、HTTP、接线、文案；矩阵的行与豁免；替身测试 | [P3-S3](plans/P3-S3-invitations.md) |
| S4 | 交错 4–8 | [P3-S4](plans/P3-S4-interleavings.md) |
| S5 | e2e：W5–W9 的接口版本，W4 补上；README 的邀请一节 | [P3-S5](plans/P3-S5-e2e.md) |

每个 Step 结束时 `make check` 为绿；S2 起 `make gen-check` 为绿；S5 之后 `make e2e`、`make image-smoke` 为绿。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 令牌：同一 id 同一令牌、不同 id 或不同密钥不同、篡改一个字符不通过、长度；邀请的邮箱与角色规则；每个用例的顺序（令牌在查询之前，锁、判定、检查、写）；接受的三种成员关系；邮箱不符；`MembershipEnder` 先删邀请再写成员行 |
| 集成 | 仓储：插入与唯一索引的翻译、删除之后邮箱可再邀请、按（工作区，邮箱）删除、锁的语句带 `deleted_at IS NULL`、`accepted_at` 的 CHECK；identity：`ShareAccount` 交回邮箱、`AccountIDByEmail`；迁移的约束名与 CHECK 的反例 |
| 扩展点 | 恢复事件的替身测试：同一事务、失败整体回滚 |
| 并发 | 3.9 |
| 契约 | 每个新码在 workspace 或 identity 的 HTTP 测试中答出；整个程序的测试覆盖新操作（PAT、400、自由文本参数、公开操作） |
| 矩阵 | 3.10 |
| 端到端 | 3.11 |
| 日志 | 邀请令牌、邀请的邮箱不进日志：用例的日志测试 |

**反向对照**（13.4 第 1 条）：
- 令牌比较用 `==` 而不是 `hmac.Equal`：没有测试能可靠地测出计时差异，由代码审查与注释守住，在审查记录里写明；
- 接受不在锁下重读邀请 → 交错 5 失败；
- 成员关系结束不删邀请 → 交错 6 与 W9 失败；
- 接受不比较邮箱，或者用锁外读到的邮箱（`ShareAccount` 去掉 `FOR SHARE`）→ 交错 8 失败；
- 规则表的 `workspace_invitation.create` 加上成员 → 矩阵失败；
- 注册策略在关闭时忽略邮箱是否相同 → W7 与策略的单元测试失败。

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成为绿。
- M1 移交第 3、4 项已落实，在 M2 总设计第 7 节的表中核对。
- 审查记录 `reviews/P3-invitations-review.md`；本文第 7 节、M2 总设计的进度表已更新。

## 7. 结果

分支 `m2-p3-invitations`：S1 `49edab8`、S2 `d84d684`、S3 `cd5a983`、S4 `c12fe0f`、S5 `8ea051f`，审查修复 `81f7f26`，合并 `b58cc47`。第 5 节全部通过，反向对照按预期失败（作者 18 项，审查者 27 项）；`make check`（vitest 446 个）、`make gen-check`、`make e2e`（58 个；新故事另跑 `--repeat-each 3`）、`make image-smoke` 本地与持续集成为绿；交错 1–8 在 `-race` 下重复 5 次（审查者 10 次）全部通过。审查见 [P3 审查记录](reviews/P3-invitations-review.md)：没有 Major、Minor，5 项 Nit 已处理；5 个疑问中 Q1 的外键一句改进 M2 总设计第 8 节，Q4 交给 P4，其余不改。规模（新增行数，不含生成的代码）：生产代码约 1,780 行（含迁移、查询、契约与文案），测试约 2,690 行，端到端约 640 行。权限矩阵 16 行 96 格，单跑约 1.5 秒。

与设计的出入（已同步进上文）：

1. `accepted_check` 加上 `deleted_at IS NOT NULL`（3.1）：原写法在 `deleted_at` 为空时放行，迁移的反例测试抓到。
2. 签名密钥由组合根先加载，邀请密钥由组合根派生（3.2）：原设计的 `Module.DerivedKey` 要等 identity 建好，而 identity 的注册策略要用它，成环。identity 的 `Deps.SigningKeyPEM` 换成 `SigningKeys`，坏私钥的报错提前到连数据库之前。
3. 令牌严格解码（3.2）：非严格时最后一个字符未用的位有多种拼法，都会通过；identity 的令牌本来就是严格的。由 `TestTokens` 守住。
4. 用例文件按包里已有的"动词_名词"命名（3 节文件表）；仓储的"按账户是否有效成员"是 `FindMembership`（带回是否有效，创建与接受共用；P4 改为带回 `ended_at`），S1 计划第 6 项写的是另一种拆法；`WorkspaceFinder` 加 `FindWorkspaceByID`（预览），`MemberUpdater` 加 `AddMember`、`RestoreMember`。
5. S2 计划的"整个程序：注册关闭时凭邀请注册"挪到 S3：要用邀请的接口创建邀请才能拿到令牌。bootstrap 的 `directory` 实现 `AccountFinder` 也在 S3（端口随用例出现）。
6. 交错 7 用测试数据库里的触发器让先到的插入停住（3.9），不在用例里留缝；P2 的 `interleave` 泛化为可持任意表的行、可在进程内运行命令（交错 8 的 `users set-email`）。
7. 矩阵的新豁免类叫 `byCredential`，覆盖检查补了 4 个反例；路径参数叫 `{workspace_invitation_id}`，与 `{workspace_member_id}` 一致（3.3、M2 总设计第 5 节原写 `{invitation_id}`）。
8. 第 5 节"接受用请求里带的邮箱"的反向对照：请求里没有邮箱，实际做的是"不比较"与"`ShareAccount` 去掉 `FOR SHARE`"，都让交错 8 失败。
9. "有效成员同时有一份发给他邮箱的待接受邀请"经 `users set-email` 达到（3.11）。

审查之后的修复（详见审查记录）：T1 用例测试的"没有消费邀请"按前缀匹配（原断言永远成立）；T2 整个程序的错误令牌改中间的字符（只改最后一个字符可能只动未用的位）；T3 契约描述与注释重新折行；T4 模块根的包注释；T5 `app.InvitationCheck` 改名 `CheckInvitation`，与模块根的接口分开。

留给之后的：

- **P4（审查 Q4）**：拆 `MembershipEnder.End`（P2 审查 Q1）时，删邀请放在写的那一步：停用持有账户行，那条路上读到的邮箱是稳定的。
- **P4（审查 Q1）**：`users set-email` 改的是有唯一约束的列，取 FOR UPDATE 级的行锁，会挡住工作区一支插入时的外键检查（`FOR KEY SHARE`）。现在不成环：改邮箱只碰账户与会话，从不等工作区一支的锁。P4 若让改邮箱或停用进入工作区一支，要重新核对。
- **M2 收尾**：13.1 第 5 条补进加锁顺序时，写明外键检查与改邮箱的冲突（M2 总设计第 8 节已改）。
