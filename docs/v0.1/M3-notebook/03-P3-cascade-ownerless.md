# M3/P3 级联与无主：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M3/P3 级联与无主 |
| 状态 | 进行中 |
| 基线 | `6a422a7`（P2 合并、P2 文档更新之后的 main） |
| 上级文档 | [M3 总设计](00-M3-design.md) 第 3、4、5、7、8、9 节；[P1 文档](01-P1-notebooks-access.md)、[P2 文档](02-P2-notebook-members.md)第 7 节；[M2 移交](handoffs/M2-workspace.md)第 1、2、3、4 项；[总体设计](../v0.1-design.md) 3.3、3.4、6.1、8.2、12.4、13 |

---

## 1. 基线

P1、P2 留下的：

- notebook 模块：笔记本与成员的十个操作，规则一，可见性变化事件与它的触发（建、开放程度跨过 `none`、添加、移出、离开、工作区的加入与角色变化），笔记本删除事件，工作区删除的注册者（按 `id` 升序锁笔记本行），清理器。
- `notebooks` 已有 `ownerless_since`、`former_owner_id` 两列与它们的 CHECK，P1、P2 没有读写它们。
- 不变量的检查 `checkNotebooks`（有效的管理员与无主恰好其一）在交错的辅助里运行。
- workspace 模块：成员身份结束（否决者、订阅者）、恢复（订阅者）两个扩展点还没有注册者；`EndCause` 的三个常量没有从模块根导出（M2 移交第 2 项）。

本 Phase 接手 M2 移交第 1 项（其余每条路径的行为测试）、第 2 项（`EndCause`）、第 3 项（订阅者写引用别的账户的列）。

## 2. 目标与范围

**目标**：工作区成员身份的结束与恢复连带笔记本：规则二拒绝自己离开或停用的唯一管理员；被移出时笔记本成为无主；回来时归还。工作区管理员看到无主清单，接管或删除，三种处理写进审计记录，可以分页查看。

**做**：

- 注册工作区成员身份结束：否决者（规则二，只对离开与停用）、订阅者（结束他在这些工作区的笔记本成员行、设置无主、可见性）。
- 注册工作区成员身份恢复：归还他名下的无主笔记本，写审计，可见性。
- 表 `notebook_audit_events`，随工作区删除与清理；`shared` 的游标封套，`common.yaml` 的 `Limit`、`Cursor` 参数。
- 四个操作：`listOwnerlessNotebooks`、`takeOverNotebook`、`deleteOwnerlessNotebook`、`listNotebookAuditEvents`；规则表的行。
- 笔记本的活动：只读的扩展点（字节数与最后写入），M3 没有注册者。
- 别的模块：`leaveWorkspace`、`deactivateMe` 的 `x-problem-codes` 加 `notebook.sole_admin`；`reactivate-member` 的输出带归还的数量；workspace 模块根导出 `EndCause` 的常量；workspace 给笔记本模块的端口加按 id 读 slug。
- 测试：规则二的表格；注册者经每条路径的行为测试（M2 移交第 1 项）；交错 20–29；矩阵的四行与按 id 的三种变体；e2e：N7–N11 的接口与命令行版本，N13 的审计部分。

**不做**：页面（P4、P5）。

## 3. 设计

### 3.1 文件

```
server/
  migrations/sql/00011_notebook_notebook_audit_events.sql
  internal/shared/cursor.go                         EncodeCursor、DecodeCursor、InvalidCursor、PageSize（照 Nerve 的封套）
  internal/modules/access/domain/rules.go           本 Phase 的四行
  internal/modules/workspace/
    module.go                                       EndRemoved、EndLeft、EndDeactivated
    workspaces.go                                   Workspaces 加 Slugs
  internal/modules/notebook/
    domain/ownerless.go、audit.go                   规则二；无主；审计的动作与游标
    app/cascade.go                                  成员身份结束的否决者与订阅者、恢复的订阅者
    app/list_ownerless.go、take_over.go、delete_ownerless.go、list_audit_events.go
    app/extension.go                                笔记本的活动（只读的扩展点）
    adapter/postgres/queries/ownerless.sql、audit.sql
    module.go、cascade.go                            Deps 加活动的注册者；NewMembershipEnd、NewMembershipRestore
  internal/bootstrap/
    registrants.go                                  工作区成员身份结束、恢复的注册者；活动的注册者（M3 没有）
    workspaces.go                                   reactivate-member 的输出带归还的数量
    notebook_cascade_test.go                        每条路径的行为测试
    interleavings_notebook_cascade_test.go          交错 20–29
    permission_matrix_*                             本 Phase 的行
deploy/runtime-grants.sql（notebook_audit_events）
api/common.yaml（Limit、Cursor、NextCursor）、api/modules/notebook.yaml、workspace.yaml、identity.yaml
web/...（notebook.sole_admin 的文案改为不指某一个笔记本）
e2e/fixtures/ownerless.ts、assert/notebook.ts；e2e/stories/notebook/n7–n11、n13
```

### 3.2 规则二与级联（M3 总设计第 4 节）

**成员身份结束的否决者**（`VetoMembershipEnd`，在工作区行已锁之后、任何写入之前）：

1. 原因是移出时什么都不做（组合根按 `EndCause` 转换为 `Voluntary`：离开、停用）。
2. 以 `FOR NO KEY UPDATE`、按 `id` 升序锁住他在这些工作区里有有效成员关系的未删除笔记本。
3. 在锁下逐本判断：他是唯一的有效管理员，且还有别的有效显式成员 → 阻止。只靠默认角色使用它的人不算别的成员。
4. 有阻止的就拒绝：409 `notebook.sole_admin`，原因给出这些笔记本所在工作区的 slug 与各自的数量（经 workspace 的端口读 slug），不列名称。

**成员身份结束的订阅者**（`MembershipEnded`，在工作区的成员行结束之后）：

1. 同样锁住他在这些工作区里的笔记本（否决者已锁过的，再锁不等待；移出时由这里第一次锁）。
2. 结束这些成员行（`ended_at` 为事件的时刻，执行者为事件的 `By`）。
3. 他原是唯一有效管理员的笔记本设置无主：`ownerless_since` 为事件的时刻、`former_owner_id` 为他，`updated_at` 不变（M3 总设计第 4 节"笔记本的活动"）。
4. 每个工作区一次可见性变化：`{工作区, [他], At}`（他失去这个工作区里的一切笔记本，含只靠默认角色看到的）。

**成员身份恢复的订阅者**（`MembershipRestored`，在工作区成员行恢复之后）：

1. 以 `FOR NO KEY UPDATE`、按 `id` 升序锁住这个工作区里 `former_owner_id` 是他、`ownerless_since` 不为空、未删除的笔记本。
2. 每本：恢复他原来那一行（角色 `admin`，`ended_at` 清空；只认已结束的行，数目不对就是缺陷，报错回滚），清除无主（两列为空），写审计 `returned`（执行者是事件的 `By`，即他本人）。
3. 他重新看得到笔记本时，一次可见性变化 `{工作区, [他], At}`：有笔记本归还，或他以管理员、成员回来（开放的笔记本给他默认角色）；以访客回来又没有归还的，什么都没变，不发（照 P2 对加入的处理）。
4. 返回归还的数量：组合根的命令行一侧把它加到计数上，`reactivate-member` 打印。

- 无主期间笔记本的成员只减不增（添加成员要管理员；接管则清除无主），所以归还时不会遇到别的管理员。
- 他在别的笔记本里结束的成员关系不恢复（M3 总设计第 4 节）。

**订阅者写引用别的账户的列**（M2 移交第 3 项）：`updated_by_id`、`former_owner_id`、审计的执行者与原所有者，由外键检查取被引用账户行的 `FOR KEY SHARE`；它与停用持有的 `FOR NO KEY UPDATE` 不冲突，与改邮箱的 `FOR UPDATE` 冲突时等待，而改邮箱只碰账户与会话、从不等工作区一支，不成环（总体设计 13.1 第 5 条）。

### 3.3 无主笔记本的操作

| 操作 | 成功 | `x-problem-codes` |
|---|---|---|
| `GET /api/v0/workspaces/{slug}/ownerless-notebooks`（`listOwnerlessNotebooks`） | 200 `{data: OwnerlessNotebook[]}`，成为无主早的在前，再按 `id` | `workspace.not_found`、`forbidden` |
| `POST /api/v0/ownerless-notebooks/{notebook_id}/take-over`（`takeOverNotebook`） | 200 `Notebook` | `notebook.not_found` |
| `DELETE /api/v0/ownerless-notebooks/{notebook_id}`（`deleteOwnerlessNotebook`） | 204 | `notebook.not_found` |
| `GET /api/v0/workspaces/{slug}/notebook-audit-events`（`listNotebookAuditEvents`），`?limit=&cursor=` | 200 `{data: NotebookAuditEvent[], next_cursor}`，新的在前 | `workspace.not_found`、`forbidden`、`bad_request`、`validation_failed` |

- **规则**：四个操作都是工作区级、只给管理员（`notebook_ownerless.list`、`notebook_ownerless.take_over`、`notebook_ownerless.delete`、`notebook_audit.list`）。两个清单对工作区的成员与访客答 403（他们看得到工作区）。按 id 的两个操作把判定的 403 与"看不到"都答 404 `notebook.not_found`，与不存在、已删除、不是无主的相同（M3 总设计第 4 节）：被移出私密笔记本的前成员拿着旧 id 也问不出它是否还在。
- **按 id 的加锁**：先不加锁读出未删除的笔记本（得到工作区）；事务里锁工作区行 `FOR SHARE`、笔记本行 `FOR NO KEY UPDATE`，判定，锁下仍是无主才继续，否则 404。
- **接管**：他成为管理员——没有成员行就插入，有已结束的行就恢复为 `admin`，有有效的行就把角色升为 `admin`；清除无主；`workspace_access` 不变；审计 `taken_over`；可见性 `{工作区, [他], At}`。回答是 `Notebook`，他的有效角色 `admin`。
- **删除无主**：与 `deleteNotebook` 相同（同一时刻软删除笔记本与成员行，发布笔记本删除事件），另写审计 `deleted`。
- **`OwnerlessNotebook`**：`id`、`name`、`workspace_access`、`member_count`、`former_owner`（显示名与邮箱）、`ownerless_since`、`last_activity_at`、`size_bytes`。
- **笔记本的活动**（扩展点，只读）：`NotebookActivity{Bytes int64; LastWriteAt *time.Time}`，注册者 `NotebookActivities(ctx, ids) (map[uuid.UUID]NotebookActivity, error)`，在调用方的读取里执行，不加锁。清单的 `size_bytes` 是各注册者字节数之和，`last_activity_at` 是各注册者的时刻与 `notebooks.updated_at` 中最晚的。M3 没有注册者：大小如实为 0。设置、清除无主不改 `updated_at`。

### 3.4 审计记录

- **表** `notebook_audit_events`：

  | 列 | 类型与约束 |
  |---|---|
  | `id` | `uuid PRIMARY KEY` |
  | `workspace_id` | `uuid NOT NULL REFERENCES workspaces ON DELETE RESTRICT`（跨模块） |
  | `notebook_id` | `uuid NOT NULL`，没有外键：审计比笔记本活得久 |
  | `notebook_name` | `text NOT NULL`：名称的快照 |
  | `action` | `text NOT NULL`，`notebook_audit_events_action_check CHECK (action IN ('taken_over', 'deleted', 'returned'))` |
  | `former_owner_id` | `uuid NOT NULL REFERENCES users` |
  | `created_by_id`、`updated_by_id` | `uuid NOT NULL REFERENCES users`：执行者（归还是原所有者本人） |
  | `created_at`、`updated_at`、`deleted_at` | `timestamptz`；`deleted_at` 只随工作区 |

  - `notebook_audit_events_workspace_id_created_at_idx ON (workspace_id, created_at DESC, id DESC) WHERE deleted_at IS NULL`：分页；`notebook_audit_events_workspace_id_idx ON (workspace_id)`：清理工作区时外键的反向查找。
  - 工作区删除的注册者同一时刻软删除它们；清理器排在 notebook 的清理器里（它不引用笔记本，与笔记本的先后不论）。
- **`NotebookAuditEvent`**：`id`、`action`、`notebook_id`、`notebook_name`、`former_owner`、`actor`（公开资料：显示名与邮箱，只给工作区管理员）、`created_at`。
- **游标**：`shared.EncodeCursor`、`DecodeCursor`、`InvalidCursor`（照 Nerve 的封套 `{"v":1,"p":…}`，无填充的 base64url，只接受它写出的拼法）。审计的载荷是 `{created_at, id}`；多读一行判断是否还有下一页。`limit` 1–100，缺省 50，越界 422 `limit`；游标不合 400 `cursor`，先于 `limit` 判断。

### 3.5 别的模块的修改

- **workspace**：模块根导出 `EndRemoved`、`EndLeft`、`EndDeactivated`；`NewWorkspaces` 加 `Slugs(ctx, ids) (map[uuid.UUID]string, error)`（未删除的工作区，在调用方的事务里读）。
- **契约**：`leaveWorkspace`、`deactivateMe` 的 `x-problem-codes` 加 `notebook.sole_admin`；`removeWorkspaceMember` 不加。前端 `notebook.sole_admin` 的文案改为不指某一个笔记本（它也来自离开工作区与停用）。
- **命令行**：`reactivate-member` 的输出末尾加 `; ownerless notebooks returned: N`。组合根的恢复注册者带一个可选的计数（`workspaceRegistrants` 的第二个参数，服务端交 nil；命令行经它取注册者，架构测试核对这一点），`WorkspaceCommand` 多一个参数读它。`users deactivate` 被规则二拒绝时打印原因（M2 已有的失败路径）。
- **契约的描述**：`leaveWorkspace`、`removeWorkspaceMember`、`deactivateMe` 写明笔记本的成员关系随之结束、唯一管理员的笔记本成为无主。

### 3.6 加锁

- 成员身份的结束、恢复已持有工作区行的 `FOR NO KEY UPDATE`（停用持有他全部的工作区），注册者直接锁笔记本行，多行按 `id` 升序（M3 总设计第 8 节）。
- 接管、删除无主：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定，与 P1、P2 的管理写相同；所以它们与成员身份的结束、恢复串行。

### 3.7 权限矩阵

- **新的列**：无主笔记本的剩余成员（M3 总设计第 9 节）。种子在 lab 加一本私密的无主笔记本 `orphan`：剩余成员是这一列（编辑者），原所有者是一个已结束的管理员行，`ownerless_since`、`former_owner_id` 已设置。这一列面对 `orphan`：读、列成员 200，管理写 403，离开 204（可见性照常，没有人能管理）。P1、P2 的笔记本行都加这一列的格子；别的列看不到 `orphan`（私密），`listNotebooks` 的内容核对不变。
- **两个清单**：笔记本的列；lab 的两位工作区管理员（`workspace admin outside`、`default reader admin`）200，`check` 核对内容（`orphan` 一本、原所有者、大小 0；审计在种子里没有记录，是空页），lab 的成员与访客 403，局外人 404 `workspace.not_found`。
- **按 id 的两个操作**：笔记本的列，三种变体：目标是 `orphan`、非无主的私密笔记本 `priv`、不存在的 id。lab 的两位工作区管理员只在第一种答 200/204；其余每列每种都是 404 `notebook.not_found`，所以成员与访客在后两种上答复相同。
### 3.8 交错

照 M3 总设计第 9 节（20–29）。持锁的一方：工作区一侧的变化持工作区行，笔记本一侧持笔记本行；双方都只取工作区行 `FOR SHARE` 的（25）持笔记本行。每个交错两种先后，结束时 `checkNotebooks`。

### 3.9 端到端

- N7、N8 的接口版本：离开工作区被拒、通过；移出成为无主；自助停用被拒（原因）。N8 的命令行：`users deactivate` 被拒（原因）。
- N9、N10：清单（名称、原所有者、时刻、大小 0）、接管、删除；成员与访客的 403、404；审计。
- N11：接受邀请归还；`workspaces reactivate-member` 归还（输出带数量）；已被接管的不归还。
- N13 的审计部分：删除工作区连带审计记录，清理之后消失。

## 4. 实施步骤

分支 `m3-p3-cascade-ownerless`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `shared` 的游标；迁移 `notebook_audit_events`、它的清理与随工作区的删除；workspace 的 `EndCause` 常量与 `Slugs`；规则二与审计的领域 | [P3-S1](plans/P3-S1-foundations.md) |
| S2 | 级联：成员身份结束的否决者与订阅者、恢复的订阅者；组合根；`leaveWorkspace`、`deactivateMe` 的码；`reactivate-member` 的数量；每条路径的行为测试 | [P3-S2](plans/P3-S2-cascade.md) |
| S3 | 无主的四个操作、笔记本的活动、审计的读；契约、HTTP、规则表；矩阵的新列与行 | [P3-S3](plans/P3-S3-ownerless.md) |
| S4 | 交错 20–29 | [P3-S4](plans/P3-S4-interleavings.md) |
| S5 | 端到端：N7–N11、N13 的审计部分 | [P3-S5](plans/P3-S5-e2e.md) |

每个 Step 结束时 `make check` 为绿；有生成物的 Step 之后 `make gen-check` 为绿；S5 之后 `make e2e` 为绿。

## 5. 测试与验证

（照 P1、P2：单元、集成、契约、矩阵、交错、端到端；反向对照：规则二不看"别的成员"、移出也拒绝、订阅者不设无主、归还不看是否已接管、接管不清除无主、清单给成员看、按 id 的操作对成员答 403、游标接受别的拼法。）

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成四个任务为绿。
- M2 移交第 1、2、3 项已落实，在 M3 总设计第 7 节的表中核对。
- 审查记录 `reviews/P3-cascade-ownerless-review.md`；本文第 7 节、M3 总设计的进度表已更新。

## 7. 结果

（完成后补写）
