# M2/P2 工作区管理与成员：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P2 工作区管理与成员 |
| 状态 | 进行中 |
| 基线 | `8cc5ae4`（P1 合并、P1 文档更新之后的 main） |
| 上级文档 | [M2 总设计](00-M2-design.md) 第 4、5、7、8、9 节；[P1 文档](01-P1-access-workspaces.md)第 3.4、7 节；[M1 移交](handoffs/M1-identity.md)第 9 项；[总体设计](../v0.1-design.md) 3.2、13.1 |

---

## 1. 基线

P1 留下的：
- access 模块：规则表（只有 `workspace.read`）、判定、事实端口；`shared` 的权限端口。
- workspace 模块：`workspaces`、`workspace_members` 两张表；创建、列表、读取、检查 slug；按用例分开的仓储端口；`domain.ValidSlug`。
- 权限矩阵的框架与覆盖检查；矩阵的数据全部由 SQL 准备。
- 整个程序的测试：自由文本参数的 NUL 与非 UTF-8 不答 5xx，P2 的新操作自动纳入。

P1 第 7 节留给本 Phase 的：`deleteWorkspace` 有了之后，矩阵的结束与删除改经接口准备；加"删除工作区在同一事务里软删成员行"的集成测试，`RoleOf` 不连表正依赖它。

本 Phase 接手 M1 移交第 9 项的前一半：把工作区的行接进全局加锁顺序，用确定性的并发测试守住"最后一个管理员"。

## 2. 目标与范围

**目标**：
- 管理员能管理工作区与成员，成员能离开。
- 唯一管理员规则一成立，并发之下也成立。
- 成员身份结束与工作区删除两个扩展点建好，测试替身证明注册者能挂上去。

**做**：
- `updateWorkspace`（改名）、`deleteWorkspace`（软删除，连带成员行）。
- `listWorkspaceMembers`：内嵌公开资料，经 identity 提供的不加锁端口读取。
- `updateWorkspaceMember`（改角色）、`removeWorkspaceMember`、`leaveWorkspace`。
- 两个扩展点：
  - 成员身份结束：否决者与事件；移出与离开调用，停用在 P4 接上。
  - 工作区删除：事件。
- 规则表加六行；矩阵的新行；矩阵的结束与删除改经接口准备。
- 确定性的交错：两位管理员同时离开、两位管理员互相降级、移出管理员与他删除工作区。
- e2e：W4 的接口版本中管理员能走的部分。

**不做**：
- 邀请（P3）。结束成员关系时删除待接受的邀请、删除工作区连带邀请，随 P3 加入。
- 停用的注册者、`reactivate-member`、清理（P4）。
- 页面（P5、P6）。
- **W8、W9 与 W4 的"非管理员答 403"移到 P3**：它们要有第二位成员，而成员只能经邀请加入。在 P2 用 SQL 插入成员行，就不是用户能走的路径了（总体设计 10.1）。M2 总设计第 7 节随之修订。

## 3. 设计

### 3.1 文件

```
server/internal/
  modules/workspace/
    module.go                 Deps 加 Profiles 与三组注册者；扩展点与 Profile 的类型别名
    domain/actions.go         workspace.update、workspace.delete、workspace.leave、
                              workspace_member.list、workspace_member.update、workspace_member.remove
    domain/member.go          角色的取值检查
    domain/errors.go          member_not_found、own_membership、sole_admin
    app/ports.go              新的窄端口（3.2）
    app/extension.go          MembershipEnd、MembershipEndVetoer、MembershipEndSubscriber、
                              WorkspaceDeletion、WorkspaceDeletionSubscriber
    app/end_membership.go     移出与离开共用的一步（P4 的停用也用）
    app/update_workspace.go、delete_workspace.go、list_members.go、
        update_member.go、remove_member.go、leave_workspace.go
    adapter/postgres/         新查询：锁工作区行、成员行的读与写、管理员计数、删除连带
    adapter/http/             六个操作
    extension_test.go         扩展点的替身测试（真实数据库）
  modules/identity/profiles.go   NewProfiles(pool)：按 id 批量读显示名与邮箱，不加锁
  modules/access/domain/rules.go 六行
  bootstrap/registrants.go       workspaceRegistrants()：本 Phase 为空
  bootstrap/deps.go              workspaceDeps 交出 Profiles（经适配）与注册者
  bootstrap/interleavings_workspace_test.go   三个确定性的交错
  bootstrap/permission_matrix_*  新行；结束与删除经接口准备
api/modules/workspace.yaml
e2e/stories/workspace/w4-rename-delete.spec.ts
```

### 3.2 加锁、判定与检查的顺序

所有写都先锁工作区行，再判定（总体设计 13.1 第 5 条；M2 总设计第 8 节）。锁工作区的语句带 `deleted_at IS NULL`：等锁之后读到 0 行，答 404。

| 操作 | 依次 |
|---|---|
| 改名 | slug 的格式（不合格式答 404，不查库）→ 事务：工作区 `FOR NO KEY UPDATE`（按 slug）→ 判定 → 名称的规则（422）→ 写 `name`、`updated_by_id`、`updated_at` |
| 删除 | slug 的格式 → 事务：工作区 N → 判定 → 软删除工作区的全部成员行（P3 起先删邀请）→ 软删除工作区（`deleted_at`、`updated_by_id`、`updated_at`）→ 删除事件的订阅者 |
| 成员列表 | slug 的格式 → 按 slug 读工作区（不加锁、不开事务）→ 判定 → 读有效成员 → 批量读公开资料 |
| 改角色 | 读成员行（不加锁，得到工作区）→ 事务：工作区 N（按 `id`）→ 重读成员行（仍有效、未删除）→ 判定 → 角色的取值（422）→ 不能改自己（409）→ 写 |
| 移出 | 读成员行 → 事务：工作区 N → 重读 → 判定 → 不能移出自己（409）→ 结束成员关系（3.4） |
| 离开 | slug 的格式 → 事务：工作区 N（按 slug）→ 判定（`workspace.leave`，三种角色）→ 规则一：调用者是唯一的有效管理员时答 409 `workspace.sole_admin` → 结束成员关系（3.4） |

- **不可见的统一答 404**：
  - 按 slug 寻址的操作答 `workspace.not_found`。
  - 按成员行寻址的操作：成员行不存在、已结束、所在的工作区已删除、调用者看不到那个工作区，都答 `workspace.member_not_found`。判定的 `ErrNotVisible` 也译成这个码：调用者点名的是成员关系。
- **422 的位置**：名称与角色的取值都在判定之后检查（总体设计 13.1 第 4 条）：看不到目标的人得到 404，不是 422。`role` 在契约里是枚举，但请求体的结构检查不核对枚举的取值，所以领域再查一次（`domain.CheckRole`）。
- **改角色不需要唯一管理员的检查**：发起者在锁下被判定为管理员，又不能改自己，所以改完之后至少还剩发起者这一位管理员。移出同理。
- **成员行不另加锁**：同一工作区成员关系的一切变化都先持有工作区行的 `FOR NO KEY UPDATE`，彼此串行。成员行的 `UPDATE` 自己取行锁，按全局顺序在工作区之后。锁下重读成员行，读到的就是提交了的最新状态。
- **端口按用例分开**（P1 审查 N1 的约定）：`WorkspaceLocker`（按 slug、按 id 锁工作区行）、`WorkspaceUpdater`（改名、软删除）、`MemberFinder`（按 id 读成员行、有效成员列表、管理员计数）、`MemberUpdater`（改角色、结束、随工作区软删除）、`MemberProfiles`（公开资料）。postgres 的 `Store` 实现前四个。

### 3.3 唯一管理员

- 规则一（本 Phase）：唯一的有效管理员不能离开，哪怕只有他一人。人数在工作区行锁之下数：`role = 'admin' AND ended_at IS NULL AND deleted_at IS NULL`。
- 规则二（停用，P4）复用同一个计数，加上"还有别的有效成员"。
- 只剩自己一人、又想离开的管理员，可以删除工作区。

### 3.4 成员身份结束：扩展点

```go
type MembershipEnd struct {
    UserID       uuid.UUID
    WorkspaceIDs []uuid.UUID   // 停用一次结束多个（P4）；移出与离开只有一个
    Cause        EndCause      // removed | left | deactivated
    By           uuid.UUID     // 发起者：移出是管理员，离开与停用是本人
    At           time.Time
}
type MembershipEndVetoer interface {      // 工作区行锁住之后、任何写入之前
    VetoMembershipEnd(ctx context.Context, e MembershipEnd) error
}
type MembershipEndSubscriber interface {  // 写入之后、同一事务
    MembershipEnded(ctx context.Context, e MembershipEnd) error
}
```

- `app/end_membership.go` 的 `membershipEnder` 是移出与离开共用的一步，P4 的停用注册者也调用它。调用方已经锁住了 `WorkspaceIDs` 的工作区行。
  1. 否决者依次调用，第一个拒绝即返回它的错误，整个事务回滚；
  2. 写这些工作区里这个账户的有效成员行：`ended_at`、`updated_by_id`、`updated_at`；
  3. 订阅者依次调用，任何错误都让整个事务回滚。
- 注册者只凭连接池构造，在 `bootstrap/registrants.go` 一处组合（总体设计 13.1 第 21 条）。本 Phase 没有注册者，`workspaceRegistrants()` 返回空。
- 否决者的码由注册者写进 `removeWorkspaceMember`、`leaveWorkspace`（P4 起还有 `deactivateMe`）的 `x-problem-codes`。
- 测试替身（`workspace/extension_test.go`，真实数据库，模块根经 `workspace.New` 装配）证明：
  - 否决者在锁下读到已提交的状态；
  - 否决时成员关系不变，答出否决者的码；
  - 订阅者在同一事务里，读得到 `ended_at`；
  - 订阅者失败时整体回滚。

### 3.5 工作区删除：扩展点

```go
type WorkspaceDeletion struct {
    WorkspaceID uuid.UUID
    By          uuid.UUID
    At          time.Time   // 订阅者以它为自己的行的 deleted_at，与工作区一侧相同
}
type WorkspaceDeletionSubscriber interface {   // 软删除之后、同一事务
    WorkspaceDeleted(ctx context.Context, d WorkspaceDeletion) error
}
```

- 没有否决者（M2 总设计第 4 节）。
- 删除连带的成员行与工作区用同一个 `At` 作 `deleted_at`，删除者记在两边的 `updated_by_id`。
- 集成测试证明：成员行与工作区在同一事务里软删除、时刻相同；订阅者在同一事务里读得到工作区的 `deleted_at`；订阅者失败时什么都没有删除（P1 第 7 节留下的义务）。

### 3.6 成员列表与公开资料

- identity 新增 `identity.NewProfiles(pool)`：`Profiles(ctx, ids) (map[uuid.UUID]identity.Profile, error)`，`Profile{DisplayName, Email}`。一条按 `id = ANY($1)` 的查询，不加锁，只凭连接池构造。
- workspace 在 `app/ports.go` 声明自己要的端口 `MemberProfiles`，值是本模块的 `Profile`。组合根用一个小适配器把 identity 的转成 workspace 的：两个模块互不导入，类型不同，结构上对不上（与 M1 的停用扩展点一样，由组合根转换）。
- `listWorkspaceMembers`：
  - 判定 `workspace_member.list`，三种角色都允许；
  - 读有效成员，按 `created_at` 再按 `id` 排序（加入的先后）；
  - 一次读出这些人的公开资料；资料缺了某个账户是内部错误（成员行的外键保证账户存在）；
  - 调用者是访客时，每个人的邮箱都是 `null`。
- `WorkspaceMember {id, user_id, role, display_name, email, created_at}`：`id` 是成员关系行的 id；`email` 可空；`created_at` 是加入的时刻。

### 3.7 接口

在 `api/modules/workspace.yaml` 中新增六个操作，都是 Bearer：

| 操作 | 成功 | `x-problem-codes` |
|---|---|---|
| `PATCH /api/v0/workspaces/{slug}`（`updateWorkspace`），`{name}` | 200 `Workspace` | `workspace.not_found`、`forbidden`、`validation_failed` |
| `DELETE /api/v0/workspaces/{slug}`（`deleteWorkspace`） | 204 | `workspace.not_found`、`forbidden` |
| `GET /api/v0/workspaces/{slug}/members`（`listWorkspaceMembers`） | 200 `{data: WorkspaceMember[]}` | `workspace.not_found` |
| `PATCH /api/v0/workspace-members/{workspace_member_id}`（`updateWorkspaceMember`），`{role}` | 200 `WorkspaceMember` | `validation_failed`、`workspace.member_not_found`、`forbidden`、`workspace.own_membership` |
| `DELETE /api/v0/workspace-members/{workspace_member_id}`（`removeWorkspaceMember`） | 204 | `workspace.member_not_found`、`forbidden`、`workspace.own_membership` |
| `POST /api/v0/workspaces/{slug}/leave`（`leaveWorkspace`） | 204 | `workspace.not_found`、`workspace.sole_admin` |

- 路径参数叫 `workspace_member_id`（`format: uuid`）：它是成员关系行的 id，不是账户的 id。
- `forbidden` 写进需要它的操作：它是平台码，不在顶层的 `x-problem-codes` 里。
- `updateWorkspace` 的 `name` 必填：现在只有它可改；以后加可改的字段时放宽为可选，不破坏现有的客户端。
- 改角色之后的回答是改过的成员，带公开资料，邮箱按调用者（管理员）给出。

### 3.8 规则表

| 操作名 | 允许 |
|---|---|
| `workspace.update`、`workspace.delete` | 管理员 |
| `workspace.leave` | 三种角色 |
| `workspace_member.list` | 三种角色 |
| `workspace_member.update`、`workspace_member.remove` | 管理员 |

`workspace.leave` 允许三种角色，却仍要一行：它让"不是有效成员"答 404（`ErrNotVisible`），与其他操作同一个判定。

### 3.9 确定性的交错

在 `bootstrap` 里，经 HTTP 调用接好线的应用（真实的 access 模块与 PostgreSQL），不给用例加测试用的接缝：

- 测试自己开一个事务，`SELECT … FOR NO KEY UPDATE` 锁住工作区行。
- 发出 A 的请求，`pgtest.WaitForLockWaitsOn(pool, "workspaces", 1)` 确认 A 在等这一行；再发出 B 的请求，等到 2。
- 提交测试的事务。PostgreSQL 的行锁按到达的先后交给等待者（第一个等待者持有元组锁，后来者排在它后面），所以 A 先、B 后，结果是确定的。两种先后各跑一次（交换 A、B 的发出顺序）。

| # | 交错 | A 先 | B 先 |
|---|---|---|---|
| 1 | 两位管理员同时离开（还有一位成员） | A 离开（204）；B 在锁下数到自己是唯一的管理员，409 `workspace.sole_admin` | 对称 |
| 2 | 两位管理员互相降级 | A 把 B 降为成员（200）；B 在锁下被判定为成员，403 | 对称 |
| 3 | 管理员 A 移出管理员 B，B 删除工作区 | 移出先提交（204）；删除时 B 在锁下已不是有效成员，404 `workspace.not_found` | 删除先提交（204）；移出锁工作区读到 0 行，404 `workspace.member_not_found` |

每个交错之后核对数据库：工作区仍有至少一位有效管理员（交错 1、2），或者成员行与工作区的状态与先提交的一方一致（交错 3）。

反向对照：把离开与改角色的判定挪到锁之前（先按 slug 读工作区、判定，再锁），交错 1、2 失败：两位管理员都离开，或者互相降级之后一个管理员都不剩。

### 3.10 矩阵

- 新的六个操作各一行，列与 P1 相同。改名、删除、改角色、移出、离开是写，各格用自己的副本。
- 按成员行寻址的行，目标是本列工作区里成员一列的成员关系（已删除的工作区里另种一个成员）。另有"改自己""移出自己"的行：管理员 409 `workspace.own_membership`。
- **准备数据**：
  - 账户经接口注册；工作区与成员行仍由 SQL 插入（id 在准备之前定好，行的请求与覆盖检查都用得到；成员要到 P3 才能经邀请加入）。
  - "已结束的成员"由管理员经 `removeWorkspaceMember` 移出；"已删除的工作区"由它的管理员经 `deleteWorkspace` 删除（P1 第 7 节的义务）。
- 矩阵之外，`deleteWorkspace` 的仓储集成测试证明成员行随工作区软删除（3.5）。

### 3.11 端到端

W4 的接口版本，管理员能走的部分（非管理员的 403 随 P3，见第 2 节）：
- 管理员用 PAT 改名，回答与落库（`name`、`updated_by_id`）；
- 删除：答 204；工作区与成员行软删除，同一个时刻，删除者记在 `updated_by_id`；
- 之后 `getWorkspace` 答 404，列表里没有它；同一个 slug 可以再创建。

## 4. 实施步骤

分支 `m2-p2-workspace-members`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 领域与数据：六个操作名与规则表的六行、领域错误与角色检查、新查询与仓储、identity 的 `Profiles` | [P2-S1](plans/P2-S1-domain-data.md) |
| S2 | 用例与扩展点：两个扩展点的类型、`membershipEnder`、六个用例及其单元测试 | [P2-S2](plans/P2-S2-use-cases.md) |
| S3 | 接口与接线：契约、HTTP 适配器、模块根、组合根与注册者、扩展点的替身测试 | [P2-S3](plans/P2-S3-api-wiring.md) |
| S4 | 交错、矩阵与端到端 | [P2-S4](plans/P2-S4-interleavings-matrix-e2e.md) |

每个 Step 结束时 `make check` 为绿；S3 之后 `make gen-check` 为绿；S4 之后 `make e2e`、`make image-smoke` 为绿。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 每个用例的顺序：slug 的格式、锁、判定、检查、写；`ErrNotVisible` 的翻译；唯一管理员；改自己与移出自己；角色的取值；`membershipEnder` 的调用顺序与失败；访客看不到邮箱 |
| 集成 | 仓储：锁的语句带 `deleted_at IS NULL`；删除连带成员行、同一个时刻；管理员的计数只数有效、未删除的；结束只改有效的行；成员列表的顺序；公开资料的批量读取 |
| 扩展点 | 3.4、3.5 的替身测试 |
| 并发 | 3.9 |
| 契约 | 每个新码在 workspace 的 HTTP 测试中答出；整个程序的测试自动覆盖新操作（PAT、400、自由文本参数） |
| 矩阵 | 3.10 |
| 端到端 | W4 的接口版本 |

**反向对照**（13.4 第 1 条）：
- 离开、改角色先判定后加锁 → 交错 1、2 失败；
- 规则表的 `workspace_member.update` 加上成员 → 矩阵失败；
- 删除不连带成员行 → 集成测试与矩阵的"已删除"一列失败；
- 否决者的错误被忽略 → 替身测试失败；
- 访客的邮箱不置空 → 用例测试失败。

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成为绿。
- M1 移交第 9 项的前一半、P1 第 7 节留给 P2 的义务已落实。
- 审查记录 `reviews/P2-workspace-members-review.md`；发现的问题已修复或明确移交。
- 本文第 7 节、M2 总设计的进度表已更新。

## 7. 结果

（完成后补写）
