# M2/P4 停用、管理命令与清理：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P4 停用、管理命令与清理 |
| 状态 | 已完成 |
| 基线 | `751af91`（P3 合并、P3 文档更新之后的 main） |
| 上级文档 | [M2 总设计](00-M2-design.md) 第 4、7、8、9 节；[P2 文档](02-P2-workspace-members.md)第 7 节与 [P2 审查](reviews/P2-workspace-members-review.md) Q1、Q2；[P3 文档](03-P3-invitations.md)第 7 节与 [P3 审查](reviews/P3-invitations-review.md) Q1、Q4；[M1 移交](handoffs/M1-identity.md)第 1、2、9、10 项；[总体设计](../v0.1-design.md) 8.5、12.4、13.1 第 5、6、21、22、23 条 |

---

## 1. 基线

P3 留下的：
- 邀请的五个操作、带邀请注册；成员身份恢复事件，由接受触发。
- `MembershipEnder.End`：否决者 → 删邀请 → 写成员行 → 订阅者。移出与离开共用它。
- 交错 1–8；`interleaveOn` 可以持任意表的行，也可以在进程内运行命令。
- identity 的停用扩展点还没有注册者：`deactivationRegistrants()` 返回空。

本 Phase 接手：
- M1 移交：
  - 第 1 项：停用的第一个注册者，要有经接口与经命令行的行为测试。
  - 第 2 项：否决者的码。
  - 第 9 项的停用部分：交错 9–13。
  - 第 10 项：转给 M7。
- P2 审查：
  - Q1：`End` 拆成否决与写两步，分别挂在停用的两个阶段；还要一个按 `id` 升序锁多个工作区的端口。
  - Q2：组合检查断言 serve 与命令行都到达 `workspaceRegistrants`。
- P3 审查：
  - Q4：删邀请放在写的那一步，用停用在账户行锁下读到的邮箱。
  - Q1：停用进入工作区一支之后，重新核对外键检查与加锁顺序（3.2）。

## 2. 目标与范围

**目标**：
- 停用账户要经过工作区的规则二，并结束他的成员关系。
- 服务器管理员能创建工作区、恢复成员关系。
- 软删除的行过了保留期，由后台任务物理删除。

**做**：
- **停用的注册者**（workspace）：
  - 规则二；
  - 结束他全部的有效成员关系，连带删除发给他邮箱的待接受邀请；
  - 转调成员身份结束的否决者与订阅者。
  - `MembershipEnder` 拆成 `Veto` 与 `Write`。
  - `deactivateMe` 的 `x-problem-codes` 加 `workspace.sole_admin`。
- **管理命令**：
  - `nervewiki workspaces create`、`nervewiki workspaces reactivate-member`；
  - identity 的 `ShareActiveAccountByEmail`。
- **清理**：
  - 注册表与定时任务：`platform/jobs` 的 `Purger` 与 `PurgeJob`；
  - workspace 的三个清理器；
  - 配置 `jobs.purge_interval`、`jobs.purge_retention`。
- **测试**：
  - 交错 9–13；
  - 组合检查：新起点 `Workspaces`；serve 与两个命令行入口都到达 `workspaceRegistrants`。
- **e2e**：W2 的命令行部分；W10 的接口与命令行部分；W12。
- **M7 的 handoff**：M1 移交第 10 项。

**不做**：

| 事项 | 理由 |
|---|---|
| 停用对话框显示 `workspace.sole_admin` 的原因 | 页面在 P6。前端文案表里这个码现在说的是"离开"，停用对话框的文案在 P6 按场景区分 |
| 一次恢复他全部的成员关系（`reactivate-member --all`） | 命令一次恢复一个工作区。也可以由工作区的管理员重新邀请他，接受会恢复那一行（P3 3.5）。没有使用者要求批量 |
| 记录成员关系结束的原因 | `ended_at` 已能说明是哪一次结束（3.3 的输出）；扩展点的值里本来就带原因 |

## 3. 设计

文件：

```
server/
  internal/modules/identity/
    accounts.go、app/accounts.go、app/ports.go     Accounts.ShareActiveAccountByEmail；端口 ShareAccountByEmail
    adapter/postgres/queries/users.sql、users.go   ShareAccountByEmail（FOR SHARE，按邮箱）
  internal/modules/workspace/
    domain/member.go                            Member.EndedAt、Active()；Standing 与规则二（`ErrSoleAdminOf` 在 `domain/errors.go`）
    adapter/postgres/store.go、members.go、purge.go   工作区（LockWorkspacesOf）、成员关系（ListStandings；FindMembership 带回 ended_at）、清理
    adapter/postgres/queries/workspaces.sql、members.sql、purge.sql   对应的查询
    app/end_membership.go                       MembershipEnder：Veto、Write、End
    app/deactivation.go                         停用的两个阶段
    app/create_workspace_for.go、reactivate_member.go  管理员的两个用例
    deactivation.go、admin.go、purgers.go       模块根：NewDeactivation、NewAdmin、Purgers
  internal/platform/jobs/purge.go               Purger、PurgeJob
  internal/platform/config/                     jobs.purge_interval、jobs.purge_retention
  internal/bootstrap/
    registrants.go                              deactivationRegistrants(pool)；purgers(pool)；停用的适配器
    commands_admin.go                           Users 与 Workspaces 共用的一段（日志、连接池、一行输出）
    workspaces.go                               Workspaces、CreateWorkspace、ReactivateMember
    wire.go、deps.go                            注册者与清理任务的接线
  cmd/nervewiki/workspaces.go                   nervewiki workspaces create、reactivate-member
api/modules/identity.yaml                       deactivateMe 的 x-problem-codes
e2e/                                            W2（命令行）、W10、W12
docs/v0.1/M7-assets-transfer/handoffs/M2-P4-insert-only-client.md
```

### 3.1 停用的注册者（M1 移交第 1、2 项；P2 审查 Q1；P3 审查 Q4）

identity 的停用分两个阶段，workspace 在每个阶段各做一件事：

| identity 的阶段 | workspace 做的 |
|---|---|
| 否决：账户行已锁，任何写入之前 | 1. 锁住他有效成员关系所在的工作区（`FOR NO KEY UPDATE`，按 `id` 升序，一条语句）。<br>2. 在锁下重读他在其中的有效成员关系，以及各工作区有效的管理员、成员人数。<br>3. 判定规则二。<br>4. 调用成员身份结束的否决者 |
| 订阅：账户行已写、会话已撤销，同一事务 | 1. 重读他有效成员关系所在的工作区。<br>2. 删除这些工作区里发给他邮箱的待接受邀请。<br>3. 写 `ended_at`。<br>4. 调用成员身份结束的订阅者 |

**两个阶段读到的集合相同**：
- 增长路径的第一条语句是 `ShareActiveAccount`，被停用持有的账户行挡住，集合不会变大。
- 移出、离开与删除工作区都先锁工作区行，被否决阶段的锁挡住，集合不会变小。
- 所以订阅阶段重读即可，两次调用之间不传状态。identity 的扩展点本来就是两次独立的调用。

**规则二**：
- `domain.Standing{WorkspaceID, Slug, Role, Admins, Members}` 是锁下读到的一份有效成员关系。
- `BlocksDeactivation()`：他是管理员，有效管理员只有他一个，而有效成员不止他一个。
- 拒绝答 409 `workspace.sole_admin`，与离开用同一个码，原因不同。detail 列出这些工作区的 slug（按 slug 排序），命令行打印的就是它。对调用者没有泄露：自助停用时，他就是这些工作区的管理员。
- 他一个有效成员关系都没有时，两个阶段什么都不做，也不调用成员身份结束的注册者（13.1 第 22 条："只在状态真正变化时触发"）。

**成员身份结束的值**：
- 账户、这些工作区、原因 `deactivated`；
- `By` 是账户自己，与 `updated_by_id` 一致；
- 时刻是停用的时刻。

**邮箱**：用停用在账户行锁下读到的 `Deactivation.Email`（P3 审查 Q4）。移出与离开仍经目录读，不加锁（P3 3.4）。

**`MembershipEnder`**：
- `Veto(ctx, e)`：调用否决者。
- `Write(ctx, e, email)`：删邀请、写成员行、调用订阅者。
- `End(ctx, e)`：`Veto` → 读邮箱 → `Write`。移出与离开照旧调用它。

**模块根**：
- `workspace.NewDeactivation(pool, endVetoers, endSubscribers)` 返回 `Deactivation`，方法是 `VetoDeactivation` 与 `AccountDeactivated`，参数是 workspace 自己的 `Deactivated{UserID, Email, At}`。
- 构造函数不叫 `New`（13.1 第 21 条）。

**组合根**：
- `deactivationRegistrants(pool)` 取 `workspaceRegistrants()` 中成员身份结束的注册者，构造 workspace 的停用注册者，作为唯一的否决者与订阅者交给 identity。
- 适配器 `deactivation` 把 `identity.Deactivation` 逐字段转换成 workspace 的值，与 `directory` 是同一种做法。
- serve 与 `bootstrap.Users` 都调用 `deactivationRegistrants(pool)`。

**契约**：
- `deactivateMe` 的 `x-problem-codes` 加 `workspace.sole_admin`。
- identity 的 HTTP 测试经替身用例返回否决者的这个错误，答出这个码一次（M1 移交第 2 项）。
- 前端文案表已有这个码。

**行为测试**（M1 移交第 1 项，13.1 第 21 条）：在整个程序上，他是某个还有别的成员的工作区唯一的管理员时：
- 经 `POST /me/deactivate`：答 409 `workspace.sole_admin`，会话照旧可用；
- 经 `nervewiki users deactivate`：失败并打印原因，账户照旧可用。

把 `deactivationRegistrants` 换成空列表，这两个测试失败。这正是 M1 收尾审查做过、当时没有测试失败的那个对照。

### 3.2 加锁顺序（P3 审查 Q1）

停用的顺序：

> `users`（他的行，`FOR NO KEY UPDATE`）→ `workspaces`（`id` 升序，`FOR NO KEY UPDATE`）→ 写 `users` → `auth_sessions` → `workspace_invitations` → `workspace_members`

不成环，理由如下：

1. **等账户行的工作区一支事务**：只有增长路径，即创建工作区、接受邀请、`reactivate-member`。它们在锁任何工作区之前就等，手里没有工作区锁。
2. **外键检查**：工作区一支写入引用账户的列（`updated_by_id` 等）时，外键检查取 `FOR KEY SHARE`，与停用的 `FOR NO KEY UPDATE` 不冲突。停用写账户行，不改有唯一约束的列，仍是 `FOR NO KEY UPDATE`。
3. **改邮箱**：取 FOR UPDATE 级的行锁，会挡住外键检查，但它只碰账户与会话，不等工作区一支的锁。停用与改同一账户的邮箱，在账户行上排队。
4. **多个停用**：各自按 `id` 升序锁工作区。
5. **会话一支**：凡是要等停用所持会话行的事务，都先要停用已经持有的账户行（M2 总设计第 8 节）。

M2 收尾时把这一节补进总体设计 13.1 第 5 条。

### 3.3 管理命令

**`nervewiki workspaces create --slug <slug> --name <name> --admin <email>`**：
- 不看 `workspace.creation_enabled`：关闭时，工作区就由这条命令创建。
- 事务：先 `ShareActiveAccountByEmail`（第一条语句，`FOR SHARE`），再插入工作区与管理员的成员行。
- `created_by_id` 是这位管理员：命令行没有自己的账户。
- 输出：`created workspace <slug> (<id>) with admin <email>`。
- 失败时退出码 1，数据库不变：
  - 名称、slug 的字段错误（以 `--name`、`--slug` 称呼）；
  - slug 已被占用；
  - 账户不存在（`identity.account_not_found`）或已停用（`identity.account_deactivated`）。

**`nervewiki workspaces reactivate-member --workspace <slug> --email <email>`**：
- 事务：`ShareActiveAccountByEmail` → 锁工作区（`FOR NO KEY UPDATE`）→ `FindMembership`，之后按成员关系的状态：

  | 成员关系 | 处理 | 输出 |
  |---|---|---|
  | 已结束 | 恢复：角色沿用结束时的，`created_at` 不变；发布成员身份恢复事件，`By` 是账户自己 | `reactivated <email> in <slug> as <role>; the membership had ended at <RFC 3339>` |
  | 有效 | 不变，不发事件 | `<email> is already a member of <slug>` |
  | 没有这一行 | `workspace.member_not_found` | 退出码 1 |

- 要求账户可用，所以要先 `users activate`（M2 总设计第 4 节）。
- 不限于停用结束的成员关系：命令是服务器管理员的显式操作，被移出的人也可以恢复。输出带结束的时刻，管理员看得出恢复的是哪一次结束。

**identity**：
- 新增 `Accounts.ShareActiveAccountByEmail(ctx, email) (uuid.UUID, error)`：按规范化的邮箱取 `FOR SHARE`。
- 等锁期间邮箱被改走时，PostgreSQL 会按新的行重新判断条件，结果是 `account_not_found`。
- workspace 在消费方声明端口 `AccountsByEmail`。

**领域**：
- `Member` 加 `EndedAt *time.Time`（P1 的类型没有它）。
- `FindMembership` 不再另外交回 `active`，改为 `Member.Active()`。

**组合根**：
- `bootstrap.Workspaces(ctx, cfg, logOut, out, cmd WorkspaceCommand)`：组合连接池、`identity.NewAccounts` 与 `workspace.NewAdmin`（带成员身份恢复的注册者）。
- 与 `Users` 共用一段：日志、连接池、等数据库、一行输出与错误。
- `cliFieldName` 加 `slug`、`name`。
- 命令行入口在 `cmd/nervewiki/workspaces.go`。

**组合检查**：
- 起点加 `Workspaces`，它要到达 `workspace.NewAdmin` 与 `workspaceRegistrants`。
- `Users` 与 `newApp` 也要到达 `workspaceRegistrants`（P2 审查 Q2）。

### 3.4 清理

**`platform/jobs`**：

```go
type Purger struct {
    Table string                                                           // 被清理的表
    Purge func(ctx context.Context, before time.Time, batch int) (int, error) // 删一批 deleted_at 早于 before 的行
}
func PurgeJob(purgers []Purger, cfg PurgeConfig) Job // PurgeConfig{Interval, Retention, Logger}
```

- 每次运行时，`before` 取"现在减去保留期"，然后按注册的顺序调用清理器。每个清理器一批一批地删，直到某一批不满。
- 任何一个清理器失败就停下，由 River 重试。这样父表不会先于子表被清。
- 删了行才记一条日志：表名与行数。清理器中途失败时，也先记下它已删的行数。
- 任务的 kind 是 `platform.purge_soft_deleted`，启动时运行一次，之后每隔 `jobs.purge_interval` 运行一次。

**配置**：

| 项 | 默认 | 约束 |
|---|---|---|
| `jobs.purge_interval` | 1h（测试配置 2s） | 至少 1s |
| `jobs.purge_retention` | 1440h（60 天） | 至少 1h |

**workspace**：
- `workspace.Purgers(pool) []jobs.Purger` 依次是 `workspace_invitations`、`workspace_members`、`workspaces`。
- 每个清理器是一条语句：`DELETE … WHERE id IN (SELECT … WHERE deleted_at < $1 LIMIT $2 FOR UPDATE SKIP LOCKED)`。别的事务持有的行跳过，下一次运行再删。
- 成员与邀请指向工作区的外键是 `ON DELETE CASCADE`（P1、P3 的迁移），保留不改。工作区的清理器只删已没有成员与邀请行的工作区（`NOT EXISTS`）：子行被跳过时，工作区随它推迟到之后的运行，级联实际不会触发。
- 所以清理不等锁，不进入加锁顺序（13.1 第 5、23 条）。`jobs.Purger` 的契约写明"跳过仍被之前的清理器跳过的行引用的行"，M3 起的清理器照此办理（审查 T1）。这只在同一模块之内做得到：父表的清理器看不到别的模块的表（sqlc 按模块限定），所以跨模块的外键用 `ON DELETE RESTRICT`，父行仍被引用时这一批失败，之后的运行再完成（M2 收尾审查 A-I1，总体设计 13.1 第 6 条）。

**组合根**：
- `purgers(pool)` 在 `registrants.go`，从叶到根排列。M3 起，新模块的清理器加在 workspace 之前。
- `newApp` 把 `PurgeJob` 与 identity 的任务一起交给 `jobs.New`。

**数据库测试**（组合根）：
1. 每张有 `deleted_at` 的表恰好有一个清理器；每个清理器的表都有 `deleted_at`（13.1 第 6 条的登记）。
2. 每条指向被清理表的外键，引用它的表也被清理，并且排在前面。

**结果**：保留期内的行不动；超过的连同子行一起删除。同一时刻软删除的父行与子行，在同一次运行中按顺序删除；子行被别的事务持有时，父行留到之后的运行。

### 3.5 确定性的交错

沿用 P2、P3 的办法：测试持锁，PostgreSQL 按到达的先后交出行锁，每个交错跑两种先后。停用经接口（`POST /me/deactivate`）或经 `bootstrap.Users` 在进程内运行；`reactivate-member` 经 `bootstrap.Workspaces` 在进程内运行。

| # | 交错 | 持的行 | 先后与结果 |
|---|---|---|---|
| 9 | 两位管理员同时停用（工作区另有一位成员）：一位经接口，一位经命令行 | 工作区 | 先到的：成功，成员关系结束。<br>后到的：在锁下数到自己是唯一的管理员，且还有别的成员，于是 409 `workspace.sole_admin`（命令行是退出码 1 与原因），账户照旧可用 |
| 10 | 停用与接受邀请（一次插入新行，一次恢复已结束的行） | 账户行 | 接受先：他成为成员（或恢复），停用随后结束它。<br>停用先：接受答 403 `identity.account_deactivated`，邀请仍待接受 |
| 11 | 停用与创建工作区 | 账户行 | 创建先：工作区只有他一人，停用允许，并结束这份成员关系。<br>停用先：创建答 403，没有新工作区 |
| 12 | 停用与 `reactivate-member` | 账户行 | 恢复先：停用随后再结束它。<br>停用先：命令失败（账户已停用），成员关系仍是结束的 |
| 13 | 停用与移出他 | 工作区 | 移出先：停用在锁下读不到这份成员关系，204，成员关系由移出结束（`updated_by_id` 是管理员）。<br>停用先：移出答 404 `workspace.member_not_found` |

### 3.6 端到端

**W2（命令行）**：
- 在 `workspace.creation_enabled = false` 的配置下，`workspaces create` 创建工作区并指定管理员；管理员经接口看到它，角色是管理员。
- 账户不存在、已停用，或 slug 已被占用时：退出码 1，打印原因，数据库不变。

**W10（接口、命令行）**：
- 他是某个还有别的成员的工作区唯一的管理员：
  - 经 PAT 停用，答 409 `workspace.sole_admin`，凭证照旧可用；
  - `users deactivate` 退出码 1，打印的原因里有 slug。
- 管理员把另一位成员升为管理员之后，停用成功。他的成员关系全部结束，包括只有他一人的那个工作区；他看不到这些工作区。
- `users activate` 之后，`workspaces reactivate-member` 逐个恢复：
  - 角色沿用，加入的时刻不变；
  - 再运行一次，说他已是成员；
  - 账户停用时运行，退出码 1。

**W12（后台任务）**：
- 准备：
  - 两个已删除的工作区，各带成员与邀请。用 SQL 把一个的删除时刻改到 61 天前，另一个改到 59 天前。
  - 一个有效的工作区里，有一份 61 天前撤回的邀请。
- 任务删掉 61 天前的工作区，连同它的成员与邀请，也删掉那份撤回的邀请；59 天前的工作区不动。

### 3.7 M1 移交第 10 项

M2 没有由请求投递的任务：清理是定时的。写 `docs/v0.1/M7-assets-transfer/handoffs/M2-P4-insert-only-client.md`（`status: open`，`from: M2/P4`，`to: M7`），内容取自 M1 移交第 10 项。

## 4. 实施步骤

分支 `m2-p4-deactivation-commands-purge`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 数据：<br>• identity 的 `ShareActiveAccountByEmail`；<br>• workspace 的 `Member.EndedAt`、`LockWorkspacesOf`、`ListStandings`、三个清理的查询；<br>• 规则二的领域；<br>• 仓储测试 | [P4-S1](plans/P4-S1-data.md) |
| S2 | 停用的注册者：<br>• `MembershipEnder` 拆分、停用的两个阶段、模块根与组合根；<br>• 契约的码与 identity 的 HTTP 测试；<br>• 行为测试；组合检查 | [P4-S2](plans/P4-S2-deactivation.md) |
| S3 | 管理命令：<br>• 两个用例、`workspace.NewAdmin`、`bootstrap.Workspaces`、`nervewiki workspaces`；<br>• 组合检查的起点 | [P4-S3](plans/P4-S3-commands.md) |
| S4 | 清理：<br>• `jobs.Purger`、`PurgeJob`、配置、`workspace.Purgers`、组合根的注册表；<br>• 数据库测试；<br>• 运行时角色的测试跑一次清理，并以运行时角色运行 `nervewiki workspaces` 的两条命令 | [P4-S4](plans/P4-S4-purge.md) |
| S5 | 交错 9–13 | [P4-S5](plans/P4-S5-interleavings.md) |
| S6 | e2e：W2（命令行）、W10、W12；README；M7 的 handoff | [P4-S6](plans/P4-S6-e2e.md) |

每个 Step 结束时 `make check` 为绿；`make gen-check` 为绿；S6 之后 `make e2e`、`make image-smoke` 为绿。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 规则二（`Standing` 的表格）；停用两个阶段的顺序：锁在判定之前，否决在写之前，空集合不调用注册者，邮箱取自停用；`MembershipEnder` 的三个入口；两个管理员用例的顺序与结果；清理的批与顺序，失败即停 |
| 集成 | 仓储：`LockWorkspacesOf` 的模式、顺序与 `deleted_at`；`ListStandings` 的人数；清理的保留期边界、批的上限、`SKIP LOCKED`（三个清理器各测），工作区等它被持有的子行；`FindMembership` 带回 `ended_at`；identity：`ShareAccountByEmail` 的锁模式，以及等锁期间邮箱被改走的情形 |
| 扩展点 | 替身测试：停用时成员身份结束的否决者在写之前、整体回滚并答出它的码；订阅者在同一事务；`reactivate-member` 发布恢复事件 |
| 行为 | 3.1 的两条：经接口、经命令行被规则二否决 |
| 并发 | 3.5 |
| 契约 | `workspace.sole_admin` 在 identity 的 HTTP 测试中答出 |
| 组合 | 起点 `Users`、`Workspaces` 不构建 HTTP 与后台任务；`Users`、`Workspaces`、`newApp` 都到达 `workspaceRegistrants`；清理器的登记与外键顺序（3.4） |
| 端到端 | 3.6 |
| 日志 | 管理命令的日志带 `by=cli`，不记邮箱 |

**反向对照**（13.4 第 1 条）：

| 改动 | 应当失败的测试 |
|---|---|
| 规则二恒为允许 | 交错 9、W10、两条行为测试 |
| 否决阶段用锁外读到的人数判定 | 交错 9 |
| `deactivationRegistrants` 返回空 | 两条行为测试 |
| 订阅阶段不删邀请 | 停用的模块测试 |
| `ShareAccountByEmail` 去掉 `FOR SHARE` | 交错 12 与 identity 的锁测试 |
| 清理器的顺序颠倒（工作区在成员之前） | 外键顺序的数据库测试 |
| 清理不看保留期 | 仓储测试与 W12 的"59 天前的不动" |
| 去掉一个清理器 | 登记测试 |
| 工作区的清理器不看子行（审查 T1） | 工作区等子行的仓储测试 |
| `LockWorkspacesOf` 去掉 `ORDER BY id`（审查 T4） | 加锁顺序的仓储测试 |

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成为绿。
- M1 移交第 1、2、9、10 项已落实，在 M2 总设计第 7 节的表中核对。
- 审查记录 `reviews/P4-deactivation-commands-purge-review.md`；本文第 7 节、M2 总设计的进度表已更新。

## 7. 结果

分支 `m2-p4-deactivation-commands-purge`：S1 `997082e`、S2 `3b16d5c`、S3 `b67b8a2`、S4 `05f9d77`、S5 `b157128`、S6 `308dfd0`，审查修复 `d816b29`，合并 `a4b509b`。第 5 节全部通过，反向对照按预期失败（作者在各 Step 13 项、其中 2 项也在 e2e 上做，审查修复另 18 项；审查者 36 项）；`make check`（vitest 446 个）、`make gen-check`、`make e2e`（62 个；新故事另跑 `--repeat-each 3`）、`make image-smoke` 本地与持续集成为绿；交错 9–13 在 `-race` 下重复 5 次（审查者 1–13 重复 10 次）全部通过。审查见 [P4 审查记录](reviews/P4-deactivation-commands-purge-review.md)：没有 Major；4 项 Minor 与 5 项 Nit 已处理，T9 不改；6 个疑问中 Q1、Q2 写进 M2 收尾时的移交，Q6 的措辞在收尾补进 13.1 第 5 条，其余不改。规模（新增行数，不含生成的代码）：生产代码约 1,300 行（含从 `store.go` 挪出的约 140 行、配置与契约），测试约 2,100 行，端到端约 290 行。

与设计的出入（已同步进上文）：

1. 仓储拆出 `members.go`、`purge.go` 与查询 `members.sql`、`purge.sql`（3 节文件表）：P4 的方法加进 `store.go` 会超过 300 行，拆分之后它是 191 行。
2. 工作区的清理器只删已没有成员与邀请行的工作区（3.4，审查 T1）：原设计说"它的清理器排在后面，删工作区时已没有子行"，但子行被 `SKIP LOCKED` 跳过时，级联会去等它的锁。
3. 清理器中途失败时，也先记下它已删的行数（3.4，审查 T10）。
4. 运行时角色的测试另以运行时角色运行 `nervewiki workspaces` 的两条命令（第 4 节 S4，13.1 第 16 条）。
5. identity 的 HTTP 测试经替身用例返回否决者的错误（3.1），不是替身否决者：HTTP 层只看用例的结果，效果相同。
6. S2 计划的"账户仍可用、会话仍在"由整个程序上的规则二测试核对：模块测试不运行 identity。S3 计划的"恢复的订阅者失败时整体回滚"做在模块根（真实数据库）：组合根还没有注册者可接。
7. e2e 夹具 `users.ts` 改名 `admin.ts`，`nervewiki users` 与 `nervewiki workspaces` 共用（S6 计划）。

审查之后的修复（详见审查记录）：T1 工作区的清理器等它的子行，`jobs.Purger` 的契约写明；T2 否决者与订阅者只听到锁下剩下的工作区；T3 清理的仓储测试覆盖三个清理器；T4 加锁顺序的测试让 slug 与表里的顺序都与 id 相反；T5 两处排序可观测；T6、T7 `nervewiki workspaces` 的帮助与 debug 级日志；T8 注释与一行夹具；T10 失败前删的行也记日志。

留给之后的（已写进 [M3 的移交](../M3-notebook/handoffs/M2-workspace.md)、[M4 的移交](../M4-pages/handoffs/M2-P4-purge-page-tree.md)与 M7 的两份移交：[只投递的客户端](../M7-assets-transfer/handoffs/M2-P4-insert-only-client.md)、[附件的清理](../M7-assets-transfer/handoffs/M2-P4-attachment-purge.md)）：

- **M2 收尾，写进 M3 的移交（审查 Q1）**：M3 第一个成员身份结束或恢复的注册者，要在整个程序上经每条路径各有行为测试：移出、离开、停用（接口与命令行）、接受邀请、`reactivate-member`、删除工作区。现在没有注册者，组合检查只证明静态可达。
- **M2 收尾，写进 M3/M7 的移交（审查 T1、Q2）**：
  - 清理器跳过仍被别的表引用的行（`jobs.Purger` 的契约，同一模块之内；跨模块的外键用 `ON DELETE RESTRICT`，M2 收尾审查 A-I1），级联不连带删除；M7 的附件清理器先删文件、再删行。
  - 外键顺序的测试排除了自引用外键（M4 的页面树）；只靠 CASCADE、没有 `deleted_at` 的子表过不了它，要决定是否豁免。
  - "失败即停"下，一个永久失败的清理器让之后的都不跑，只在日志里看得到。
- **M2 收尾（审查 Q6）**：3.2 补进 13.1 第 5 条时，第 1 点补上"外键检查等改邮箱"（第 3 点已覆盖），写明 `KEY SHARE` 会越过排队的独占等待者；M3 的订阅者若写引用别的账户的列，审查时复核。
- **之后有大表时（审查 Q3）**：表上没有 `deleted_at` 的索引，每批顺序扫描；River 的任务期限 1 分钟，超时取消重试，已删的批不丢。v0.1 的规模不需要，到时加部分索引或调长清理的期限。
