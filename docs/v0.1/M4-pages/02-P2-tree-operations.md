# M4/P2 树操作：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P2 树操作 |
| 状态 | 进行中 |
| 基线 | `e4840f8`（P1 合并、文档提交之后的 main）；本文与各 Step 计划提交之后开分支 |
| 上级文档 | [M4 总设计](00-M4-design.md) 第 4、5、8、9 节；[P1](01-P1-page-module-pipeline.md) 3.5、3.6、3.10–3.13；[总体设计](../v0.1-design.md) 3.5、8.3、13.1 |

---

## 1. 基线

P1 留下的：

- page 模块：五张表、领域（标题与键、次序 `Place`、先序、层级、改动的合并）、仓储与清理器；写入单元（`app.Writer.Run`）与两种操作（`CreatePage`、`Rename`），守卫、参与者、观察者三个扩展点（组合根交空集合）；`listNodes`、`createPage`、`getPage`、`renameNode`。
- 单元的操作经同一条路：锁下读出状态 → 422 → 409 → 守卫（`app.Step`）→ 写 → 条目与版本 → 参与者；单元结束时一次事件。
- 权限矩阵的四行（笔记本级 13 列）；树与逐项读取一致；交错 30–33 与 `checkPages`；e2e 的 PG1、PG2、PG11、PG12（新建与改名）、PG13。
- P1 审查留下的：`unit.go` 已有四百多行，P2 开工时按操作拆分（Q1）；交错持笔记本行的 `FOR SHARE`，才证明得了树操作因为锁树而串行（T1）；落库的断言比较条目的父页与次序、改动的执行者与时刻，并由建页之外的人去改（T4、T5）。

## 2. 目标与范围

**目标**：树的其余两个写——移动（换父页与排序）与删除子树——作为单元里的两个新操作，在笔记本行的独占锁下判断环、层级与重名；删除让子树与跟随的行在同一时刻进回收站。

**做**：

- 先把 `app/unit.go` 按操作拆开，不改行为（P1 审查 Q1）。
- 领域：操作 `move`、`delete`；码 `page.cycle`（409）；移动的位置（排除自己的兄弟、原地不动的判断）；子树的高度。
- 仓储：子树的递归查询（带相对深度）；移动（父页、次序、审计列，23505 → `page.title_taken`）；删除子树（节点与跟随的行，同一时刻）；删除操作的条目随节点一起进回收站。
- 单元的操作 `Move`、`Delete`；用例 `moveNode`、`deleteNode`；契约、HTTP、规则表（`node.move`、`node.delete`）、文案 `page.cycle`。
- 测试：领域的表格（环、深度、位置、原地不动、连续移动后的重排）；仓储；用例的码的次序与写；矩阵两行；树与逐项读取在删除之后一致；交错 34–37；e2e 的 PG3、PG4 接口版本，PG12 的移动与删除。

**不做**：删除时结束编辑会话（P4，会话在 P4 才有）；回收站的恢复（M8）；前端（P5）。

## 3. 设计

### 3.1 文件

```
server/
  internal/modules/page/
    domain/change.go、errors.go、tree.go          OpMove、OpDelete；ErrCycle；Height、移动的位置
    adapter/postgres/queries/nodes.sql            Subtree、MoveNode、DeleteNodes；RecordItem 删除的条目随节点进回收站
    adapter/postgres/store.go、changesets.go
    app/ports.go                                  Nodes.Subtree；NodeWriter.MoveNode、DeleteNodes
    app/unit.go                                   拆分之后：Writer、Run、Unit、apply、变更集、条目、版本与事件、appender
    app/unit_place.go                             Position、First、After；放进父页的检查 lineOf、slotOf、titleFree（新建、改名、移动共用）
    app/unit_create.go、unit_rename.go            PageDraft 与 CreatePage；Rename（从 unit.go 移出）
    app/unit_move.go、unit_delete.go              Unit.Move、Unit.Delete
    app/move_node.go、delete_node.go
    adapter/http/handler.go                       moveNode、deleteNode
  internal/modules/access/domain/rules.go         node.move、node.delete
  internal/bootstrap/
    permission_matrix_page_test.go                两行
    page_visibility_test.go                       经接口删除一棵子树之后仍一致
    interleavings_page_test.go                    交错 34–37
api/modules/page.yaml                             两个操作；NodeMove
web/apps/web/src/app/problem-messages.ts、en.ts、zh-CN.ts   page.cycle
e2e/fixtures/pages.ts、assert/page.ts；e2e/stories/page/pg3、pg4；pg12 加移动与删除
```

### 3.2 领域

- **操作**：`OpMove`、`OpDelete` 加进 `domain.Operation`。
- **码**：`ErrCycle`（`page.cycle`，409）：新的父页是被移动的页自己或它的后代。
- **层级**：`Height(subtree)`：子树里最深的相对深度（只有自己为 1）。移动之后最深的一页的层级 = 新父页的层级 + 高度；超过 `MaxDepth` 答 `page.too_deep`。新父页为空（移到根下）时新父页的层级为 0。
- **移动的位置**：兄弟按次序排好之后先去掉被移动的页自己，再按 P1 的 `slotOf`、`Place` 取值；`after_id` 是被移动的页自己时答 422 `after_id`（不是"兄弟"）。父页不变、取到的位置与原来的相同（排在同一个兄弟之后，或都在最前）时不写，与改成同名相同：不插变更集、不调用观察者。
- **合并**：同一单元先建后删的节点，合并之后前后都为空，条目的 `changeset_items_state_check` 不收。M4 没有这样的单元（用例只做一种操作，参与者只追加改名与正文写），P2 不处理，写进单元的注释。
- **改动**：移动是被移动的页一条（前、后状态都在，`Moves()` 为真）加上它的每个后代一条（前后相同：位置没变，路径变了；不记条目，进事件与守卫的值，M6 的链接按路径解析，要知道哪些页的路径变了）；删除是子树的每个节点一条（后状态为空）。与总体设计 8.3、M4 总设计第 8 节"移动与删除是整棵子树"一致。

### 3.3 数据

- `Subtree :many`：从一个未删的节点往下的递归查询，带相对深度（自己为 1）与先序需要的列；深度上界 64，只防缺陷造成的环（同 `Ancestors`）。
- `MoveNode :exec`：改 `parent_id`、`sort_order`、`updated_by_id`、`updated_at`；23505 译为 `page.title_taken`（兜底，单元里先查）。
- `DeleteNodes :exec`：一组节点 id，在同一时刻软删除节点（`updated_by_id`、`updated_at` 同时写）以及这些节点未删的正文、版本、条目；与笔记本删除的写法相同（CTE），只改未删的行。
- **删除操作的条目**：单元在写之后记条目，删除操作的条目写下时它的节点已进回收站。`RecordItem` 对"后状态为空"的条目在插入（或合并更新）时同时写 `deleted_at = 单元的时刻`：生命周期表里条目随节点，包括删除它的那个变更集里的条目（M4 总设计第 4 节）；否则这些条目永远不进回收站，节点的清理器因 `NOT EXISTS` 跟随的行而永远跳过它们。
- 不加迁移、不加索引：`(notebook_id, parent_id)` 的索引已在。

### 3.4 写入单元的两个操作

**`Unit.Move(ctx, nodeID, parentID *uuid.UUID, Position)`**：

1. 锁下读出节点（`FindNodeIn`，没有答 `page.not_found`）。
2. 422：新父页（P1 的 `lineOf`：不是本笔记本未删的页面答 `parent_id`）；兄弟（新父页下，去掉自己）与 `after_id`（不是这些兄弟之一答 `after_id`）。
3. 原地不动：不写，返回节点。
4. 409：新父页是自己或在自己的子树里（`Subtree` 的 id 集合）答 `page.cycle`；新的兄弟里有同键的答 `page.title_taken`（换父页时才查；同一父页内排序不会撞上）；新父页的层级 + 子树高度超过 10 答 `page.too_deep`。
5. 守卫（`Step{OpMove, 改动}`）→ 重排（若 `Place` 给出）→ `MoveNode` → 条目（只有被移动的页）→ 参与者。

**`Unit.Delete(ctx, nodeID)`**：

1. 锁下读出节点（没有答 `page.not_found`）；`Subtree` 读出整棵子树。
2. 守卫（`Step{OpDelete, 子树每个节点的改动}`）→ `DeleteNodes` → 条目（每个节点一条，随节点进回收站）→ 参与者。
3. 兄弟不重排：删除不改变别的兄弟的相对位置。

参与者追加的操作（`Appender`）本 Phase 不加移动与删除：M6 只追加正文写（P4）。

### 3.5 接口与用例

| 操作 | 寻址 | 判定 | 单元 | 成功 |
|---|---|---|---|---|
| `moveNode` `POST /nodes/{node_id}/move` | 节点 | `node.move` | 树 | 200 `TreeNode` |
| `deleteNode` `DELETE /nodes/{node_id}` | 节点 | `node.delete` | 树 | 204 |

- `NodeMove`：`parent_id`（必填、可为 `null`：移到根下）、`after_id`（可省略、可为 `null`，与 `createPage` 相同：省略在最后，`null` 在最前）。
- 码的次序：404 → 403 → 422（`parent_id`、`after_id`）→ 409（`page.cycle`、`page.title_taken`、`page.too_deep`）→ 守卫。
- 两个用例照 `renameNode`：事务之外先不加锁读出节点得到笔记本（`page.not_found`），单元在锁下重读。`moveNode` 在单元里重读节点作答（参与者可能改了它）。
- 日志：`node moved`、`node deleted`（带子树的节点数），id 与客户端，不带标题；没有写时（原地不动）不记。

### 3.6 权限

`node.move`、`node.delete` 给 `writers()`，笔记本级。矩阵两行（`editorsOnly`）：移动把每列的页移到根下的最前（`after_id: null`）：priv 的子页换父页（与根页不同名，不撞）；已在根下最前的页原地不动，同样答 200（判定在原地不动的判断之前），格子只比较状态码。删除每列的页。

### 3.7 树与逐项读取

`page_visibility_test.go` 照旧在矩阵的数据上比较；这一份数据另经接口删除 team 的页（带一个子页）：删除之后两个都不在树里、逐项读取答 404，对每一列都一致。

### 3.8 交错

| # | 交错 | 持有的行 | 断言 |
|---|---|---|---|
| 34 | A 移到 B 下与 B 移到 A 下 | 笔记本行，`FOR SHARE` | 先到的 200，后到的 409 `page.cycle`；无环 |
| 35 | 移动把子树推深与在子树底部新建 | 笔记本行，`FOR SHARE` | 每个单独都不超过 10 层，两个都做就超过；先到的成功，后到的 409 `page.too_deep` |
| 36 | 删除子树与在其中新建、改名 | 笔记本行，`FOR SHARE` | 删除在先：新建答 422 `parent_id`，改名答 404；写在先：新写的页随子树以同一时刻删除 |
| 37 | 改名与把另一页移进同一父页成为同名 | 笔记本行，`FOR SHARE` | 先到的成功，后到的 409 `page.title_taken` |

两种先后各一个用例，结束时 `checkPages` 与 `checkNotebooks`。测试持笔记本行的 `FOR SHARE`（P1 的 `sharedNotebookRow`）：两个写只因为锁树才等它、才一个接一个；树锁若弱化成 `FOR SHARE`，它们不再等，测试在等锁的期限失败（P1 审查 T1）。

### 3.9 端到端

- PG3（接口）：管理员建页，工作区成员（笔记本对工作区开放为 `editor`）移动：同一父页内排序（最前、某页之后、最后）、换父页、移到根下；移动之后 `getPage` 的祖先链是新的；移进自己的子树 409 `page.cycle`；把高 3 的子树移到第 9 层的页下 409 `page.too_deep`；落库：被移动的页的条目的前后父页与次序，节点的 `updated_by_id` 是移动的人、`updated_at` 等于这次变更集的时刻。
- PG4（接口）：管理员建页，工作区成员删除一页连同两层子页，之后谁都读不到、树里没有；落库：子树的节点、正文、版本、条目以同一时刻软删除，删除自己的条目也在内；节点的 `updated_by_id` 是删除的人。
- PG12：阅读者移动、删除答 403；看不到笔记本的人 404；`workspace_access = editor` 的成员可以移动与删除。
- 断言（`assert/page.ts`）：`expectMoved`（条目的前后父页与次序、执行者与时刻）、`expectSubtreeDeleted`（同一时刻、删除的人、删除自己的条目）。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 拆分 `unit.go`（一个提交，不改行为）；领域（操作、`ErrCycle`、高度、移动的位置与原地不动）与仓储（`Subtree`、`MoveNode`、`DeleteNodes`、删除的条目进回收站），各自的测试 | [P2-S1](plans/P2-S1-domain-data.md) |
| S2 | 单元的 `Move`、`Delete`；两个用例；契约、HTTP、规则表、文案；矩阵两行 | [P2-S2](plans/P2-S2-unit-api.md) |
| S3 | 树与逐项读取（删除之后）；交错 34–37 | [P2-S3](plans/P2-S3-interleavings.md) |
| S4 | e2e 的 fixture 与 PG3、PG4，PG12 的移动与删除 | [P2-S4](plans/P2-S4-e2e.md) |

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 环（移到自己、子页、孙页之下）；高度与层级上限（恰好 10 层、11 层）；位置（去掉自己之后的最前、某页之后、最后；`after_id` 是自己）；原地不动；表格驱动 |
| 仓储 | `Subtree` 的先序与深度、不含已删的；`MoveNode` 的 23505；`DeleteNodes` 只改未删的行、同一时刻；删除的条目带 `deleted_at`，合并更新也带 |
| 用例 | 码的次序；移动的条目只记被移动的页、后代进事件不进条目；原地不动不写；删除的事件含整棵子树；日志 |
| 整个程序 | 矩阵两行；删除之后树与逐项读取一致；交错 34–37 |
| 端到端 | PG3、PG4 的接口版本，PG12 的移动与删除 |

反向对照（每个新检查各一个，13.4 第 1 条）：树锁改成 `FOR SHARE`（交错 34–37 失败）；防环只看新父页本身（孙页之下的移动不被拒）；高度按 1 算（推深的移动不被拒）；原地不动也写；删除的条目不进回收站（整个清理任务的测试里，删掉的子树清不掉）；`DeleteNodes` 也改已删的行；移动的后代不进事件。

## 6. 完成标准

- 第 5 节的测试全部通过，`GOFLAGS=-p=3 make check`、`make gen-check`、前端的检查为绿；持续集成为绿。
- 审查（Opus）完成，发现已处理，记录在 `reviews/P2-tree-operations-review.md`。
- 00 号文档的进度表与本文的"结果"更新。

## 7. 结果

（完成后补写）
