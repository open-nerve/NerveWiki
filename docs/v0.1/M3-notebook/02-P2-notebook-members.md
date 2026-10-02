# M3/P2 笔记本成员：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M3/P2 笔记本成员 |
| 状态 | 已完成（`aa350dc` 合并，审查见 [P2 审查](reviews/P2-notebook-members-review.md)） |
| 基线 | `9c1d670`（P1 合并、P1 文档更新之后的 main） |
| 上级文档 | [M3 总设计](00-M3-design.md) 第 4、5、7、8、9 节；[P1 文档](01-P1-notebooks-access.md)第 7 节与 [P1 审查](reviews/P1-notebooks-access-review.md) Q1；[M2 移交](handoffs/M2-workspace.md)第 6 项；[总体设计](../v0.1-design.md) 3.3、6.1、12.1、12.4、13 |

---

## 1. 基线

P1 留下的：

- notebook 模块：`notebooks`、`notebook_members` 两张表（成员行每对一行，`ended_at` 记结束），五个笔记本的操作，笔记本删除事件，工作区删除的注册者，清理器。
- access 的笔记本级：规则表有 `notebook.*` 五行；事实端口读调用者的显式角色与开放程度。
- 权限矩阵：笔记本列在工作区 `lab` 里，十二个账户，四个笔记本（`priv`、`team`、`wiki`、`gone-nb`）。
- 不变量的检查（`checkNotebooks`：有效的管理员）在交错辅助每建一本笔记本之后与每个交错结束时运行；P1 的交错结束时没有存活的笔记本，本 Phase 起它才对结局起作用。

本 Phase 接手 M2 移交第 6 项（加入与改角色没有事件）。

## 2. 目标与范围

**目标**：笔记本的成员可以管理：列出、添加、改角色、移出、离开；唯一的管理员规则（规则一）在锁下成立。笔记本自己的可见性变化发出事件，工作区的两个新事件（成员加入、角色变化）建好，笔记本模块转发它们。

**做**：

- 五个操作：`listNotebookMembers`、`addNotebookMember`、`updateNotebookMember`、`removeNotebookMember`、`leaveNotebook`；规则表的五行。
- 规则一：唯一的有效管理员不能离开，哪怕只有他一人；不能改自己的角色，不能移出自己。
- 可见性变化事件（`notebook` 模块建立）：笔记本自己的触发。
- workspace 模块的两个事件：成员加入（接受邀请插入新行）、成员角色变化（改角色），总体设计 12.1 第 6 条的例外；笔记本模块注册它们，转发为可见性变化。
- 端口：notebook 读工作区的有效成员关系（添加成员），读账户的公开资料（成员列表）。
- 测试：规则一的表格；矩阵的七行（五个操作，改与移出各加一行"自己的"）；交错 17–19；可见性事件在每个触发点的测试替身；e2e：N4、N5 的接口版本，N2 的第二位成员。

**不做**：级联、无主、审计、游标（P3）；工作区成员关系的结束与恢复的可见性，随 P3 的注册者；页面（P4、P5）。

## 3. 设计

### 3.1 文件

```
server/
  internal/modules/access/domain/rules.go          本 Phase 的五行
  internal/modules/workspace/
    app/extension.go                               MembershipAddition、MemberRoleChange 与它们的订阅者
    app/accept_invitation.go、update_member.go     写入之后调用订阅者
    app/{invitations,members,team,registrants}_test.go   调用的次序与值、同角色不调用、两个注册者的分发
    module.go                                      Deps 加两组订阅者，类型别名
    member_events_test.go                          两个事件在事务内、值、失败回滚（真实数据库，照恢复事件）
  internal/modules/notebook/
    domain/member.go、errors.go、actions.go        CheckRole、CheckAddition、CheckLeave（规则一）；三个码；五个操作名
    domain/member_test.go
    app/ports.go                                   WorkspaceMembers、MemberProfiles；MemberFinder、MemberWriter
    app/extension.go                               VisibilityChange 与订阅者；WorkspaceMemberEvents（转发工作区的两个事件）
    app/manage.go                                  lock 带上调用方的 404
    app/list_members.go、add_member.go、update_member.go、remove_member.go、leave_notebook.go、members.go（成员与资料）
    app/create_notebook.go、update_notebook.go     可见性的触发
    app/members_test.go、visibility_test.go、fakes_members_test.go
    adapter/postgres/members.go、members_test.go、queries/members.sql   成员的读、写；有效管理员的计数
    adapter/http/members.go、members_test.go、handler.go   五个处理器
    module.go、member_events.go                    Deps 加可见性的订阅者与两个端口；NewWorkspaceMemberEvents
    visibility_test.go                             可见性的每个触发点（接好线的模块与真实数据库）
  internal/bootstrap/
    deps.go、registrants.go                        notebook 的两个端口（资料经转换）；工作区两个事件的注册者；可见性的订阅者（M3 没有）
    notebook_profiles.go                           identity.Profile 转 notebook.Profile
    notebook_registrants_test.go                   工作区两个事件交到笔记本的转发、再交到可见性的订阅者
    permission_matrix_notebook_members_test.go、permission_matrix_seeded_test.go   本 Phase 的行；成员行的 id
    interleavings_notebook_members_test.go         交错 17–19
api/modules/notebook.yaml、api/openapi.yaml
web/apps/web/src/app/problem-messages.ts、i18n/messages/en.ts、zh-CN.ts   三个码的文案
e2e/fixtures/notebook-members.ts、assert/notebook.ts；e2e/stories/notebook/n2、n4、n5
```

### 3.2 接口

| 操作 | 成功 | `x-problem-codes` |
|---|---|---|
| `GET /api/v0/notebooks/{notebook_id}/members`（`listNotebookMembers`） | 200 `{data: NotebookMember[]}` | `notebook.not_found` |
| `POST /api/v0/notebooks/{notebook_id}/members`（`addNotebookMember`），`{user_id, role}` | 201 `NotebookMember` | `notebook.not_found`、`forbidden`、`validation_failed` |
| `PATCH /api/v0/notebook-members/{notebook_member_id}`（`updateNotebookMember`），`{role}` | 200 `NotebookMember` | `notebook.member_not_found`、`forbidden`、`validation_failed`、`notebook.own_membership` |
| `DELETE /api/v0/notebook-members/{notebook_member_id}`（`removeNotebookMember`） | 204 | `notebook.member_not_found`、`forbidden`、`notebook.own_membership` |
| `POST /api/v0/notebooks/{notebook_id}/leave`（`leaveNotebook`） | 204 | `notebook.not_found`、`notebook.member_not_found`、`notebook.sole_admin` |

- **`NotebookMember`**：`id`（成员关系的 id，成员的操作用它）、`user_id`、`role`、`display_name`、`email`（调用者是工作区的管理员或成员时给，访客是 `null`，与工作区成员列表相同）、`created_at`（第一次加入的时刻，恢复的行保留它）。
- **列表**：有效的显式成员，按加入的时刻、再按 id。只靠默认角色的人不在列表里：他们不是成员。
- **按成员关系寻址**：`{notebook_member_id}` 是 `format: uuid`。成员关系不存在、已结束、所在的笔记本已删除或调用者在其中没有角色，都答 `notebook.member_not_found`。
- **离开**：调用者在这个笔记本里没有有效的显式成员关系（只靠默认角色，或已经离开）答 `notebook.member_not_found`。

### 3.3 规则（access 与 domain）

| 操作 | 级别 | 允许 |
|---|---|---|
| `notebook_member.list` | 笔记本 | 三种角色 |
| `notebook_member.add` | 笔记本 | 管理员 |
| `notebook_member.update` | 笔记本 | 管理员 |
| `notebook_member.remove` | 笔记本 | 管理员 |
| `notebook.leave` | 笔记本 | 三种角色 |

- **规则一**：`domain/member.go` 的 `CheckLeave`；在笔记本行的锁下数有效的管理员（`app/leave_notebook.go`）；不能改、移出自己的在 `update_member.go`、`remove_member.go`。
  - 改角色、移出：对象是调用者自己答 409 `notebook.own_membership`（管理员离开用 `leaveNotebook`）。调用者是管理员且不能改自己，所以改完之后至少还有他一个管理员。
  - 离开：调用者是唯一的有效管理员答 409 `notebook.sole_admin`，哪怕只有他一人（他可以删除笔记本）。两位管理员同时离开时，后一位在锁下数到的只剩他自己（交错 18）。
- **添加**：
  1. `user_id` 必须是这个工作区的有效成员（含访客），否则 422 `user_id: not_allowed`；
  2. 已经是有效的显式成员答 422 `user_id: duplicate`；
  3. 有已结束的行就恢复它（`ended_at` 清空，角色取这次给的，`created_at` 保留）；否则插入新行。
  - 角色的取值与上面两条一起，一次列出全部字段问题。
  - 工作区的成员关系在工作区行的 `FOR SHARE` 下读：工作区成员关系的变化持 `FOR NO KEY UPDATE`，不会在本事务提交之前插进来，所以"笔记本成员 ⊆ 工作区有效成员"成立（M3 总设计第 4 节）。

### 3.4 端口

- **`WorkspaceMembers.RoleOf(ctx, workspaceID, userID) (shared.WorkspaceRole, bool, error)`**：workspace 模块已有的 `NewMemberships(pool)`（access 的工作区级事实）方法集相同，组合根直接传。
- **`MemberProfiles.MemberProfiles(ctx, ids) (map[uuid.UUID]Profile, error)`**（方法名与 workspace 的相同）：identity 的 `Directory`。`notebook.Profile` 与 `identity.Profile` 逐字段相同，由 `bootstrap/notebook_profiles.go` 转换（照 workspace 的 `directory`）。资料在事务里、写入之后读：读不到或缺资料时整体回滚，不在提交之后答 500（13.1 第 19 条）。

### 3.5 加锁

- 按笔记本寻址的（列出不加锁；添加、离开）：P1 的 `manager.lock`（工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定）。
- 按成员关系寻址的（改角色、移出）：先不加锁读出成员关系（得到笔记本）与笔记本（得到工作区）；事务里同样先锁工作区行、笔记本行、判定，再以成员关系的 id 重读（锁下已结束或不存在答 404），再校验与规则一。成员行不另加锁：同一笔记本的成员写都持笔记本行的 `FOR NO KEY UPDATE`。先判定、后重读（workspace 的 `lockMember` 是先重读、后判定）：成员关系在两次读之间结束时，有角色的非管理员得到 403 而不是 404；两种次序都不泄露，有角色的人本来就能列出成员（P2 审查 Q3）。
- `manager.lock` 带上调用方的 404：按成员关系寻址的操作里，看不到的笔记本答 `notebook.member_not_found`，不泄露笔记本的存在。
- 规则一的计数在笔记本行的锁下，所以两位管理员互相降级、同时离开不会让笔记本一个管理员都不剩（交错 17、18）。

### 3.6 可见性变化事件

- **值**（`app/extension.go`）：`VisibilityChange{WorkspaceID, UserIDs []uuid.UUID, Reached bool, At}`，订阅者 `VisibilitySubscriber.VisibilityChanged(ctx, v)`。`Reached` 表示"这个工作区的有效成员与管理员，以本事务所见为准"（M3 总设计第 4、8 节）。订阅者在写入之后、同一事务内调用，错误整体回滚。M3 没有订阅者；模块根以类型别名公开，`Deps.VisibilitySubscribers` 交进去。
- **笔记本自己的触发**：

  | 写入 | 值 |
  |---|---|
  | 建笔记本 | 创建者；开放程度不是 `none` 时 `Reached` |
  | `workspace_access` 跨过 `none`（`none` ↔ `viewer`/`editor`） | `Reached`，`UserIDs` 为空：订阅者在同一事务里按工作区的有效管理员与成员解析 |
  | 添加、恢复成员 | 被添加的人 |
  | 移出、离开 | 那个人 |

  `viewer` 与 `editor` 之间、改笔记本内的角色不触发：可见的集合不变。删除由笔记本删除事件告知。只靠默认角色还看得到的人被移出时也发一次：多发无害，少发则 M5 的事件流可能残留。
- **工作区的两个新事件**（workspace 模块的 `app/extension.go`）：
  - `MembershipAddition{WorkspaceID, UserID, Role, By, At}`，订阅者 `MembershipAdded(ctx, a)`：接受邀请插入新的成员行之后（`joinAdded`）；恢复已结束的行仍是原来的恢复事件。创建工作区的成员行不发（M3 总设计第 4 节）。
  - `MemberRoleChange{WorkspaceID, UserID, From, To, By, At}`，订阅者 `MemberRoleChanged(ctx, c)`：`updateWorkspaceMember` 写入之后，只在角色确有变化时（同角色的请求照旧写 `updated_at`，但没有变化可告知）。
  - 照 M2 的形状（值是名词、方法是过去分词：`MembershipRestore`/`MembershipRestored`）：在写入之后、同一事务内调用，模块根以类型别名公开，`Deps` 交进去。`AcceptInvitationDeps` 原来的 `Subscribers`（恢复）改名 `Restored`，加 `Added`；`UpdateMemberDeps` 加 `Subscribers`。
- **笔记本模块注册这两个事件**：成员加入且角色是管理员或成员 → `VisibilityChange{工作区, [他], At}`（访客加入不得到任何默认角色，不发）；角色变化跨过访客（访客 ↔ 管理员、成员）→ 同上。注册者只凭订阅者列表构造（`notebook.NewWorkspaceMemberEvents(subscribers)`），不读库；值由 `bootstrap` 逐字段转换（照 `workspaceDeletion`）。
- **测试**：模块根以测试替身证明每个触发点在事务内、带对的值、失败整体回滚；两个工作区事件在 workspace 模块根有同样的证明；两个注册者的分发由 app 的测试守住（13.1 第 21 条）；转发的规则（访客加入不发、角色变化不跨访客不发）在 app 的表格测试；组合根的测试把一个替身订阅者交给 `notebookExtensions`，经 `workspaceRegistrantsWith`（`workspaceRegistrants` 交 `notebookRegistrants()` 给它，测试交自己的）调用两个工作区事件，证明转发接上了线、值逐字段转换。`deps.go` 到 `workspaceRegistrants` 的最后一跳 M3 测不到（没有订阅者），写进 [M5 的移交](../M5-collab-editing/handoffs/M3-P2-visibility.md)。
- **值没有执行者**：M5 关事件流用不到；这是 13.1 第 21 条的例外，M3 总设计第 8 节写明。

### 3.7 数据

不加迁移。`notebook_members` 的部分唯一索引已经保证每对只有一行；`notebook_members_notebook_id_idx` 供成员列表与计数。新增查询：

- `ListMembers(notebook_id)`：有效的，按 `created_at, id`；
- `FindActiveMember(id)`、`FindMemberOf(notebook_id, user_id)`（含已结束的，供恢复）；
- `CountAdmins(notebook_id)`：有效的管理员；
- `UpdateMemberRole`、`EndMember`（`ended_at`、`updated_*`）、`RestoreMember`（清空 `ended_at`，角色）。

### 3.8 权限矩阵

> P2 合并时的样子；P3 加了无主的笔记本 `orphan`（角色列七个，lab 里五本笔记本，自己的行多一个目标，四列答 204），见 P3 文档。

- **成员行的 id**：种子给每个笔记本成员关系定好 id（`seeded.notebookMembers`，键为"笔记本/列"），`workspaceOfRow` 认得它们；覆盖测试不连库就能核对每格指向的成员关系在该列的工作区里。账户的 id 由注册决定，`prepareMatrix` 注册之后填进 `seeded.accounts`，`check` 用它核对列表与答复里的 `user_id`；成员的显示名是邮箱的本地部分（注册时的默认值）。
- **本 Phase 的行**（笔记本列，各列面对 `notebookOf` 给的笔记本；"有角色的"是 `roleIn` 给出角色的六列）：

  | 行 | 目标 | 答复 |
  |---|---|---|
  | `listNotebookMembers` | 该列的笔记本 | 有角色的 200，`check` 核对列出的正好是那个笔记本的有效成员、邮箱只在调用者是工作区的管理员或成员时给；其余 404 `notebook.not_found` |
  | `addNotebookMember`（写） | 把"工作区成员（局外）"加为 `reader`：他是 lab 的有效成员，不是这四个笔记本的显式成员 | 笔记本管理员 201；有角色的其余 403；其余 404 `notebook.not_found` |
  | `updateNotebookMember`（写） | 该列的笔记本里一位成员的行（`priv` 的编辑者那一行，`team` 的访客阅读者，`wiki`、`gone-nb` 的管理员）；`priv` 的编辑者列与 `team` 的访客阅读者列因此指向自己的行，答复同样是判定的 403 | 笔记本管理员 200；有角色的其余 403；其余 404 `notebook.member_not_found` |
  | `updateNotebookMember`"自己的"（写） | 调用者自己的行，没有的列用该列笔记本的管理员那一行 | 笔记本管理员 409 `notebook.own_membership`；其余同上一行：判定在规则一之前 |
  | `removeNotebookMember`、它的"自己的"（写） | 同上两行 | 同上两行，成功是 204 |
  | `leaveNotebook`（写） | 该列的笔记本 | `priv` 的管理员是唯一的，409 `notebook.sole_admin`；其余显式成员三列 204；只靠默认角色的两列 404 `notebook.member_not_found`；没有角色的 404 `notebook.not_found` |

  规则一中离开成功的一支（还有别的管理员）由 app 测试与 e2e N5 覆盖：给 `priv` 第二位管理员会改动 P1 各行的答复。

### 3.9 交错

照 M3 总设计第 9 节，持笔记本行（两边都只取工作区行 `FOR SHARE`，持工作区行分不出先后），用 `WaitForLockWaitsOn(…, "notebooks", n)`：

- **17 两位笔记本管理员互相降级**：先到的一方成功；后到的一方在锁下发现自己已不是管理员，答 403。结束时笔记本仍有一位管理员。
- **18 两位笔记本管理员同时离开**：先到的离开成功；后到的数到只剩自己，答 409 `notebook.sole_admin`。
- **19 添加成员与删除笔记本**：删除先，添加的一方锁笔记本行时读到 0 行，答 404，没有新成员行；添加先，删除连带软删除新成员行。

每个交错结束时核对不变量（P1 的 `checkNotebooks`）。

### 3.10 端到端

- `e2e/fixtures/notebook-members.ts`（列、加、改、移出、离开），`assert/notebook.ts` 加 `expectNotebookMember`（行、角色、结束与否、由谁写的）。
- **N2 的第二位成员**：私密笔记本加了第二位成员，两人的列表里它都是 `member_count` 2（页面上它移到"团队笔记本"，P4）。
- **N4 的接口版本**：管理员从工作区成员中添加成员（含访客）、改角色、移出；有效角色取较高的（`editor` 开放的笔记本里加为 `reader` 的成员读到 `editor`）；非管理员答 403；改自己的角色答 409；添加不是工作区成员的人答 422。落库：成员行的角色、结束、恢复保留 `created_at`。
- **N5 的接口版本**：成员离开；唯一的管理员离开答 409 `notebook.sole_admin`，哪怕只有他一人；另一位成为管理员之后他可以离开。

## 4. 实施步骤

分支 `m3-p2-notebook-members`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | workspace 的两个事件（值、订阅者、两个用例的调用、模块根）与它们的测试 | [P2-S1](plans/P2-S1-workspace-events.md) |
| S2 | notebook：成员的查询与仓储、规则一、端口；五个用例、可见性事件与笔记本自己的触发、工作区事件的转发；契约、HTTP、组合根、规则表、文案；矩阵的行（契约一有新操作，覆盖检查就要求它们） | [P2-S2](plans/P2-S2-notebook-members.md) |
| S3 | 交错 17–19 | [P2-S3](plans/P2-S3-interleavings.md) |
| S4 | 端到端：N2 的第二位成员，N4、N5 的接口版本 | [P2-S4](plans/P2-S4-e2e.md) |

每个 Step 结束时 `make check` 为绿；S2 之后 `make gen-check` 为绿；S4 之后 `make e2e` 为绿。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 规则一的表格；添加的三种情形（不是工作区成员、已是成员、恢复）；可见性的转发规则 |
| 集成 | 仓储：成员列表的顺序与过滤、恢复保留 `created_at`、计数只数有效的管理员；模块根：每个可见性触发点在事务内、失败回滚；workspace 模块根：两个新事件 |
| 契约 | 五个操作的每个码在 `adapter/http` 测试中答出 |
| 矩阵 | 3.8 |
| 交错 | 17–19 |
| 端到端 | 3.10 |

**反向对照**：规则一的计数不在锁下 → 交错 17、18 失败；添加不核对工作区成员关系 → 矩阵或 app 测试失败；恢复时改写 `created_at` → 仓储测试失败；访客加入也转发 → 转发的表格失败；可见性在提交之后调用 → 模块根的回滚测试失败。

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成四个任务为绿。
- 审查记录 `reviews/P2-notebook-members-review.md`；发现的问题已修复或明确移交。
- 本文第 7 节、M3 总设计的进度表已更新。

## 7. 结果

- 分支 `m3-p2-notebook-members`：S1 `a347e2a`、S2 `20aa9a2`、S3 `7f0bf0d`、S4 `7e9b94e`；审查修复 `84aba1e`；`aa350dc` 合并（`--no-ff`）。
- 门禁：每个 Step 的 `make check` 为绿；`make gen-check`、`make e2e`（90 个）、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P2 审查](reviews/P2-notebook-members-review.md)，没有 Critical、Major；1 项 Minor 与 4 项 Nit 已修复，Q1 写进 [M5 的移交](../M5-collab-editing/handoffs/M3-P2-visibility.md)。
- M2 移交第 6 项（加入与改角色没有事件）落实，M3 收尾时改为 done。

**与设计的偏差**（已同步进上文）：

1. 矩阵的改、移出一行，`priv` 的编辑者列与 `team` 的访客阅读者列的目标是自己的行（3.8）。
2. `seeded.accounts` 在 `prepareMatrix` 注册之后填（3.8）。
3. 组合根的测试经 `workspaceRegistrantsWith`，测试交自己的注册者（3.6）。
4. 资料端口的方法名是 `MemberProfiles`；`AcceptInvitationDeps.Subscribers` 改名 `Restored`，加 `Added`（3.4、3.6）。
5. 成员角色变化只在角色确有变化时发（3.6）。
6. `CheckAddition` 一次列出全部问题；不在工作区时只报 `not_allowed`，不再报"已是成员"（3.3）。
7. 开放程度跨过 `none` 时 `UserIDs` 为空（3.6）。
8. 改、移出的目标写作"一位成员"：有角色的非管理员可能指向自己的行（3.8）。
9. N4 另核对访客看到的邮箱是 `null`、访客的显示名、已结束的成员关系不能再改或移出（3.10）。

**留给后面的**：M5 的第一个可见性订阅者（[M5 的移交](../M5-collab-editing/handoffs/M3-P2-visibility.md)）；`notebook.member_not_found` 用在离开上的文案（P4，P2 审查 Q5）；13.1 第 11 条的例子补 `bootstrap/notebook_profiles.go`、第 21 条补 `notebook.NewWorkspaceMemberEvents` 与可见性事件不带执行者的例外（M3 收尾）。
