# M4/P1 页面模块与写入管线：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P1 页面模块与写入管线 |
| 状态 | 已完成（`bb90d03` 合并，审查见 [P1 审查](reviews/P1-page-module-pipeline-review.md)） |
| 基线 | `c9216db`（M4 总设计按设计审查修订之后的 main）；本文与各 Step 计划提交之后开分支 |
| 上级文档 | [M4 总设计](00-M4-design.md) 第 4、5、7、8、9 节；[M2/P4 移交](handoffs/M2-P4-purge-page-tree.md)；[M3/P1 移交](handoffs/M3-P1-notebook-deletion.md)；[总体设计](../v0.1-design.md) 3.5、3.8、7、8.3、13 |

---

## 1. 基线

M3 留下的：

- notebook 模块：笔记本与成员、无主、审计；三个扩展点（笔记本删除事件、可见性变化事件、笔记本的活动），都还没有注册者（组合根的 `notebookRegistrants()` 返回空集合）。
- access 的笔记本级：`Target{WorkspaceID, NotebookID}` 交给 `Authorize`，得到带有效角色的 `Grant`；事实经 `notebook.NewFacts(pool)`。
- 清理注册表：notebook 的三个清理器排在 workspace 的之前；`TestPurgersComeBeforeTheTablesTheyReference` 不看自引用的外键。
- 权限矩阵：工作区级六列、笔记本级 13 列（`notebookColumns()`），目标在工作区 `lab` 的五本笔记本里。
- 交错 1–29；笔记本的不变式 `checkNotebooks`。

代码勘察的细节（文件与行号）见本 Phase 的工作记录；下面只写决定。

## 2. 目标与范围

**目标**：页面模块的骨架与写入单元一次建好，后面的 Phase 只在单元里加操作；树的头四个操作（列出、新建、读取、改名）跑在笔记本级的判定之上；笔记本删除连带页面，页面树的清理从叶到根一次清完。

**做**：

- `shared.TitleKey`；archtest 放行 `golang.org/x/text/cases`。
- notebook 模块给页面的端口 `notebook.NewNotebooks(pool)`：读出笔记本所属的工作区（不加锁）、以 `FOR SHARE` 或 `FOR NO KEY UPDATE` 锁笔记本行。
- page 模块：迁移 `nodes`、`page_contents`、`changesets`、`changeset_items`、`page_revisions`；领域（节点、标题、次序、祖先与层级、改动）；写入单元（锁、判定、校验、守卫、写入、条目与版本、参与者、观察者、客户端与写入选项）；`listNodes`、`createPage`（不带正文）、`getPage`、`renameNode`；笔记本删除的注册者；清理器。
- 组合根：page 的 `Deps`、注册者的组合（`notebookRegistrants(pool)` 交出页面的删除订阅者；`pageRegistrants()` 交空的守卫、观察者、参与者）、清理器的顺序。
- 契约 `api/modules/page.yaml`；新码 `page.not_found`、`page.title_taken` 的中英文案。
- 测试：标题键、领域规则、仓储与清理、扩展点的测试替身、矩阵的四行、树与逐项读取一致、笔记本删除的三条路径、整个清理任务、交错 30–33 与页面树的不变量；e2e 的接口版本。

**不做**：移动、删除子树（P2）；`platform/markdown` 与阅读视图（P3）；正文的读写、编辑会话、带正文的新建、笔记本的活动、请求体上限（P4）；前端（P5、P6，本 Phase 只加文案）。

## 3. 设计

### 3.1 文件

```
server/
  migrations/sql/00012_page_nodes.sql … 00016_page_page_revisions.sql
  migrations/schema_test.go                      新的约束与索引名（加 NULLS NOT DISTINCT 的标记）、CHECK 的反例
  sqlc.yaml                                      page 一条，只列 00012–00016
  internal/shared/title_key.go                   TitleKey
  internal/archtest/rules_test.go、purity_test.go、rules_cases_test.go
                                                 直接导入放行 x/text/cases；传递依赖放行 golang.org/x/text/
  internal/platform/postgres/tx.go、tx_test.go   TxManager.InTx（page 的 Tx 端口，3.11）
  internal/modules/notebook/
    notebooks.go、notebooks_test.go              NewNotebooks(pool)：WorkspaceOf、ShareByID、LockByID
    adapter/postgres/store.go、queries/notebooks.sql、store_test.go
                                                 ShareNotebook（FOR SHARE）；过时的注释改正
  internal/modules/access/domain/rules.go        writers()；本 Phase 的四行
  internal/modules/page/
    module.go                                    New(Deps)、Register、Actions()；扩展点的类型别名
    deletion.go                                  NewNotebookDeletion(pool)：笔记本删除的注册者
    purgers.go                                   Purgers(pool)
    extension_test.go                            守卫、观察者、参与者、两操作的单元、客户端（接好线的模块与真实数据库）
    domain/actions.go、client.go、errors.go、node.go、order.go、tree.go、change.go
    app/ports.go、extension.go、unit.go、authorize.go、view.go、
        list_nodes.go、create_page.go、get_page.go、rename_node.go
    adapter/postgres/（store、changesets、purge；queries 的 nodes、contents、changesets、purge；gen）
    adapter/http/（handler、gen、main_test.go、handler_test.go）
  internal/bootstrap/
    deps.go、wire.go、registrants.go             pageDeps；注册；notebookRegistrants(pool)、pageRegistrants()；purgers 的顺序
    actions_test.go                              并集加上 page.Actions()
    page_registrants_test.go                     删除笔记本、删除工作区、删除无主笔记本连带页面
    purge_test.go                                整个清理任务带上三层页面、任务没有错误
    permission_matrix_seeded_test.go             种下页面；workspaceOfRow 认得页面
    permission_matrix_page_test.go               本 Phase 的四行
    permission_matrix_test.go                    matrixRows 加上这四行
    page_visibility_test.go                      树与逐项读取一致
    interleavings_page_test.go                   交错 30–33；checkPages
api/modules/page.yaml、api/openapi.yaml
deploy/runtime-grants.sql
web/apps/web/src/app/problem-messages.ts、i18n/messages/en.ts、zh-CN.ts
e2e/fixtures/pages.ts、assert/page.ts、purge.ts（从 n13 移出的"把删除推到保留期之前"）
e2e/stories/page/pg1、pg2、pg11、pg12、pg13；e2e/stories/notebook/n13 改用 purge.ts
```

### 3.2 标题键：`shared.TitleKey`

- `TitleKey(s string) string = NFC(fold(NFC(s)))`：`norm.NFC`、`cases.Fold()`、再 `norm.NFC`（总体设计 3.5）。输入是已经过 `CheckTitle` 的标题（M6 的链接解析传入链接的目标文本，同一个函数）。
- `cases.Caser` 有状态、不能并发共用，也不能是包级变量（gochecknoglobals）：每次调用新建一个。
- 表格测试：大小写（`A`/`a`）、完整折叠（`Straße` 与 `STRASSE`、`ﬁ` 与 `fi`）、希腊文的终止形（`Σ`、`σ`、`ς` 同键）、NFC 与 NFD 的同一标题、折叠之后不是 NFC 的例子（再规范化一次才相等）、全角与半角不同键（`Ａ` 与 `A`：完整折叠不做兼容分解）、土耳其语的 `İ`（语言无关的折叠，与 Obsidian 的 `toLowerCase` 不同，写进注释）。
- **archtest**：纯层（`domain`、`app`、`shared`）的直接导入放行 `golang.org/x/text/cases`（与 `norm`、`transform` 并列）；传递依赖放行 `golang.org/x/text/` 下的包（`cases` 会带进 `language` 与 `internal/…`）。`rules_cases_test.go` 里 `page/domain → x/text/language` 仍被拒：直接导入只认这三个包。规则的说明文字同步。修订总体设计 13.1 第 12 条（M4 总设计已写）。

### 3.3 notebook 给页面的端口

照 `workspace.NewWorkspaces(pool)` 的形状：接口与未导出的实现在 notebook 的模块根（`notebooks.go`），实现持有 postgres 的 store。

```go
type Notebooks interface {
    // WorkspaceOf reads, unlocked, the workspace of the notebook not deleted with id.
    WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
    // ShareByID locks that notebook's row FOR SHARE until the transaction ctx carries ends.
    ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
    // LockByID locks it FOR NO KEY UPDATE.
    LockByID(ctx context.Context, id uuid.UUID) (bool, error)
}
func NewNotebooks(pool *pgxpool.Pool) Notebooks
```

- `WorkspaceOf` 用已有的 `FindNotebook`；`LockByID` 用已有的 `LockNotebook`；新加 `ShareNotebook`（照 `LockNotebook`，`FOR SHARE`，带 `deleted_at IS NULL`）。两个锁方法在事务之外调用即报错（`postgres.InTx`）。删除了或不存在的答 `false`，不答错误。
- 方法只用 `uuid` 与基本类型：page 的 `app/ports.go` 声明同形的接口，组合根直接交，不转换（13.1 第 11 条）。page 的 `Workspaces` 端口只声明 `ShareByID`，组合根交 `workspace.NewWorkspaces(pool)`。
- `notebooks.sql`（与它的生成代码）、`store_test.go` 里"M4 的页面写只以 FOR SHARE 锁笔记本行、不锁工作区行"的注释改为现在的规则（13.1 第 5 条）。

### 3.4 数据

五条迁移，每表一条，按外键的依赖排：

**`nodes`**（00012）

| 列 | 说明 |
|---|---|
| `id uuid PK` | |
| `notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT` | 跨模块（13.1 第 6 条） |
| `parent_id uuid` | 根下为空；复合外键 `nodes_notebook_id_parent_id_fkey (notebook_id, parent_id) REFERENCES nodes (notebook_id, id) ON DELETE RESTRICT`：父页在同一个笔记本 |
| `kind text NOT NULL` | `nodes_kind_check`：`page`、`asset` |
| `name text NOT NULL` | `nodes_name_check`：1–255 字节（规则在应用，`CheckTitle`） |
| `name_key text COLLATE "C" NOT NULL` | `nodes_name_key_check`：不为空 |
| `sort_order double precision NOT NULL` | `nodes_sort_order_check`：有限的数（排除 NaN 与 ±Infinity） |
| `created_by_id`、`updated_by_id uuid NOT NULL REFERENCES users`；`created_at`、`updated_at timestamptz NOT NULL`；`deleted_at timestamptz` | 审计字段由用例写 |

- `nodes_parent_check`：`parent_id <> id`。
- `nodes_notebook_id_id_key UNIQUE (notebook_id, id)`：复合外键要它。
- `nodes_notebook_id_parent_id_name_key_idx`：`UNIQUE (notebook_id, parent_id, name_key) NULLS NOT DISTINCT WHERE deleted_at IS NULL`。
- `nodes_notebook_id_parent_id_idx (notebook_id, parent_id)`，不带条件：子节点（含已删的）的查找、"删叶子"的 `NOT EXISTS`、外键检查。
- `nodes_deleted_at_idx (deleted_at) WHERE deleted_at IS NOT NULL`。

**`page_contents`**（00013）：`node_id uuid PK REFERENCES nodes ON DELETE CASCADE`（模块内，清理器先删它，级联只是兜底，与 `notebook_members` 相同）；`content text NOT NULL`；`revision integer NOT NULL`（`page_contents_revision_check`：≥ 1）；`content_hash bytea NOT NULL`（`page_contents_content_hash_check`：32 字节）；`byte_size integer NOT NULL`（`page_contents_byte_size_check`：等于 `octet_length(content)`，不超过 5 MB）；`updated_by_id uuid NOT NULL REFERENCES users`；`updated_at`；`deleted_at`；`page_contents_deleted_at_idx`。P1 只在新建时写空正文（版本 1）。

**`changesets`**（00014）：`id`；`notebook_id REFERENCES notebooks ON DELETE RESTRICT`；`kind text`（`changesets_kind_check`：M4 只有 `edit`，后面的 M 在 page 模块的迁移里扩充）；`client text`（`changesets_client_check`：`web`、`api`、`cli`，或 `mcp:` 加 1–128 个非控制字符）；`message text`（可空；`changesets_message_check`：1–4096 字节）；`created_by_id REFERENCES users`；`created_at`、`updated_at`（会话里的再次写入推进 `updated_at`，P4）；`deleted_at`；`changesets_notebook_id_idx`、`changesets_deleted_at_idx`。

**`changeset_items`**（00015）：`id`；`changeset_id REFERENCES changesets ON DELETE CASCADE`；`node_id REFERENCES nodes ON DELETE CASCADE`；`before_parent_id`、`before_name`、`before_sort_order`（新建时三者为空）；`after_parent_id`、`after_name`、`after_sort_order`（删除时三者为空）；`created_at`、`updated_at`、`deleted_at`。

- `changeset_items_changeset_id_node_id_key UNIQUE`：每个（变更集、节点）一行。
- `changeset_items_state_check`：`before_name` 与 `before_sort_order` 同为空或同不为空，`after_*` 同理，前后至少有一个。父页的列可以为空（根），不进这条检查。
- `changeset_items_node_id_idx`（随节点软删除、清理的 `NOT EXISTS`）、`changeset_items_deleted_at_idx`。

**`page_revisions`**（00016）：`id`；`changeset_id`、`node_id`（同上，模块内 `CASCADE`）；`base_revision integer`（新建时为空）；`revision integer NOT NULL`；`content text`、`content_hash bytea`、`byte_size integer`（与 `page_contents` 相同的三条检查，名字带本表的前缀）；`created_at`、`updated_at`、`deleted_at`。`page_revisions_revision_check`：`revision ≥ 1` 且 `base_revision` 为空或小于 `revision`。`page_revisions_changeset_id_node_id_key UNIQUE`、`page_revisions_node_id_idx`、`page_revisions_deleted_at_idx`。

其他：

- `deploy/runtime-grants.sql` 给五张表 DML。
- `schema_test` 的索引描述加一个字母标出 `NULLS NOT DISTINCT`（读 `pg_index.indnullsnotdistinct`），否则这个承重的选项没有测试守住；每条 CHECK 各一个反例，有两个分支的各分支一个（消息的空串与超长、版本的大小不符与超过 5 MB）。
- 约束名翻译：`nodes_notebook_id_parent_id_name_key_idx` 的 23505 → `domain.ErrTitleTaken`（仓储测试直接插入重复行证明）。交错 32 碰不到它：两次新建都持笔记本行的独占锁，第二个在锁下就看得到第一个。测试持笔记本行的 `FOR SHARE`（不改树的写所持的锁），新建只因为锁树才等它：树锁若弱化成 `FOR SHARE`，两次新建并行、后插入的撞上唯一索引，答复同样是 201 与 409，但测试在等锁的期限失败（P1 审查 T1）。

### 3.5 领域

- `Node`：id、笔记本、父页（`*uuid.UUID`）、种类、名称、名称键、次序、审计字段。`TreeState{ParentID *uuid.UUID, Name string, SortOrder float64}`（`change.go`）：节点在树里的位置。
- **标题**：`CheckTitle(field, s)` 之后算 `TitleKey`，两者一起放进 `Title{Name, Key}`，建页与改名共用。
- **次序**（`order.go`）：`Place(siblings []float64（已按次序）, after int) (value float64, renumber []float64)`：放在第 `after` 个之后（`-1` 为最前，`len-1` 为最后）；取前后两个的中间值，最前是第一个减 1，最后是最后一个加 1，空的父页是 0；间隔小于 `1e-9`（或中间值等于某一端）时给出整组的重新编号（步长 1，从 0 起）与新值。表格测试：连续在同一位置插入 200 次，次序始终严格递增。
- **树**（`tree.go`）：`PreOrder(nodes)`：父在子前、兄弟按（次序、id），给 `listNodes`；`Depth`（根为 1）。层级上限 `MaxDepth = 10` 在 `node.go`。`Ancestors` 由仓储的递归查询给出，链是否完整到根也由仓储核对（`Store.Ancestors`）。
- **改动**（`change.go`）：`Change{NodeID, Before, After *TreeState, Revision int}`：前为空是新建，后为空是删除，`Revision` 为 0 是正文没动；前后位置不同（`Moves()`）的改动才记条目。操作的种类是 `Operation`（P1 有 `create`、`rename`，后面的 Phase 加移动、删除与正文写），放在守卫与参与者的值 `app.Step` 上，不在改动上。同一单元里同一节点的多次改动按"最早的前、最新的后"合并（与条目相同）。
- **操作名**（`actions.go`，13.1 第 3 条的 `<资源>.<动词>`）：`node.list`、`page.read`、`page.create`、`node.rename`（P2 加 `node.move`、`node.delete`，P4 加 `page.write`、`page.edit`）。节点的操作用 `node`：M7 的附件也是节点。
- **错误**（`errors.go`）：`page.not_found`（404，地址里的页面或节点：没有、已删除、或调用者看不到这本笔记本）；按笔记本寻址的操作（`listNodes`、`createPage`）答 notebook 的 `notebook.not_found`（码按调用方点名的东西给，notebook 对按 slug 寻址的操作答 `workspace.not_found` 是先例）；`page.title_taken`（409）。父页不可用是 `parent_id: not_allowed`（422），层级超过上限是 `page.too_deep`（409，P2 的移动也用，本 Phase 在新建时就用到）。

### 3.6 写入单元

`app/unit.go`。用例不自己开事务、加锁：

```go
type UnitSpec struct {
    NotebookID uuid.UUID
    Action     shared.Action   // 判定用的操作
    Tree       bool            // 含树操作：笔记本行 FOR NO KEY UPDATE；否则 FOR SHARE
    Client     domain.Client   // domain/client.go
    Options    Options         // app/extension.go；UpdateLinks（M4 恒为真）
    NotFound   error           // 笔记本不可用时答的码：notebook.not_found 或 page.not_found
}
type Outcome struct {          // 用例的日志用它；没有改动时 ChangesetID 为零
    WorkspaceID, By, ChangesetID uuid.UUID
    At                           time.Time
}
func (w *Writer) Run(ctx context.Context, spec UnitSpec, do func(ctx context.Context, u *Unit) error) (Outcome, error)
```

`Run` 的顺序：

1. 事务之外：`Notebooks.WorkspaceOf`（没有就是 `spec.NotFound`）。
2. 事务之内（`Tx.WithinTx`；单元是最外层的事务，见 3.11）：`Workspaces.ShareByID` → `Notebooks.ShareByID` 或 `LockByID`（读到 0 行即 `spec.NotFound`）→ `Authorize(actor, spec.Action, Target{WorkspaceID, NotebookID})`，看不到的换成 `spec.NotFound`。
3. `do(ctx, u)`：用例在单元里执行操作（`u.CreatePage`、`u.Rename`，P2 起还有移动、删除，P4 起还有正文写），把交来的 ctx（带着单元的事务）交给每个操作。操作在 ctx 不带单元的事务时报错：否则它在连接池上写，既不在单元里、也不在锁下（P1 审查 D-a）。每个操作：
   1. 在锁下读出它要的状态（父页、兄弟、节点本身），做取值的检查（422：标题、父页与 `after_id` 是否可用）；
   2. 与当前状态的冲突（409：重名、层级），在单元里先查，不靠唯一索引的 23505（23505 会让整个事务失败，多操作的单元接不下去；唯一索引只是兜底）；
   3. 调用写入守卫，按登记的顺序，第一个错误即停：值是 `app.Step`，即写入的上下文（`Write`：工作区、笔记本、执行者、客户端、选项、时刻）、操作的种类、每个涉及节点的前后状态；守卫的错误原样答出；
   4. 写入；第一次写入时插入变更集（`kind = edit`、客户端、执行者、时刻）；按（变更集、节点）插入或更新条目（合并规则同 3.5）；正文有变化的写版本行（新建页面写一行：`base_revision` 空、`revision` 1、空正文）；改动并进单元的改动集；
   5. 按登记的顺序调用参与者：值是同一个 `app.Step`（守卫与参与者要的字段相同，不分成两个同形的类型），参与者可以经 `Appender` 追加操作（M6 追加正文写；本 Phase 的测试替身追加一次改名），追加的操作同样走 1–5，但不再调用参与者（参与者不递归）。
4. `do` 返回之后，有改动时按登记的顺序调用观察者，值是 `Event`：`Write`、变更集与 3.5 合并后的改动集（一个单元一次）。
5. 提交。任何一步出错都整体回滚，单元答出那个错误。
6. 事务之后：用例写日志（3.7 的"日志"）。

- 一个单元只读一次时钟（13.1 第 9 条）：变更集、条目、版本、节点与正文的时刻都是它。正文的哈希与字节数由单元按内容一起算（`u.content`），正文行与版本行取同一个值。
- 新建与改名的用例在单元里、操作之后重读这一页作答复：参与者可能改了它，答复是单元结束时的状态；重读在观察者之前、事务之内。
- 守卫在领域的检查之后、写入之前：它看到的是算好的"之后"状态（审查 M6 要求的形状），所以它的码排在领域的码之后。修订 M4 总设计第 5 节与总体设计 8.3、13.1 第 4 条：00 号文档原写"守卫在领域校验之前、守卫的码先于 422"，实现时发现守卫要的"之后"状态正是校验算出来的。
- 客户端（`domain.Client`）：`web`、`api`、`cli`、`mcp:<名>`，领域校验长度与字符（与 `changesets_client_check` 相同）。HTTP 适配器按 `Actor` 定：`APITokenID` 不为零是 `api`，否则 `web`。
- 单元没有改动（例如改名成同一个名字）时不插变更集、不调用观察者。改名改成只差大小写的同一键：键相同，也不冲突（不和自己比），名称变了，照常写。

### 3.7 接口与用例

| 操作 | 寻址 | 判定 | 单元 | 成功 |
|---|---|---|---|---|
| `listNodes` `GET /notebooks/{notebook_id}/nodes` | 笔记本 | `node.list`（读，不开事务） | — | 200 `{data: TreeNode[]}`，先序 |
| `createPage` `POST /notebooks/{notebook_id}/pages` | 笔记本 | `page.create` | 树 | 201 `Page` |
| `getPage` `GET /pages/{page_id}` | 页面 | `page.read`（读） | — | 200 `Page` |
| `renameNode` `PATCH /nodes/{node_id}` | 节点 | `node.rename` | 树 | 200 `TreeNode` |

- **结构**（契约里的名字）：`TreeNode`（`id`、`notebook_id`、`parent_id`、`kind`、`name`、`created_at`、`updated_at`；不叫 `Node`：生成的 TS 根类型会与 DOM 的 `Node` 同名）；`Page`（`TreeNode` 的字段，加 `ancestors: {id, name}[]`（从根到父页）、`revision`、`byte_size`、`content_updated_at`、`content_updated_by`）。
- **`createPage` 的请求体**：`parent_id`（必填、可为 `null`：根下）、`title`（必填）、`after_id`（可省略、可为 `null`：省略放在最后，`null` 放在最前，否则放在这个兄弟之后）。用例的 `Position` 是可比较的值：零值是最后，`First()`、`After(id)`。修订 M4 总设计第 4 节"次序"（原写"空为最前"，省略与 `null` 没有分开）。
- **`renameNode`**：`name`（必填）。
- **读**：不开事务，先读出节点所属的笔记本与工作区（已删除的节点答 `page.not_found`），再判定，再读。`getPage` 的祖先链由一条递归查询给出（到根为止，链断了是内部错误）。
- **码的次序**：404（笔记本或节点）→ 403 → 422（标题、父页、`after_id`）→ 409（`page.title_taken`、`page.too_deep`）→ 守卫的码。
- **日志**：`page created`、`node renamed`，带 `workspace_id`、`notebook_id`、`node_id`、`changeset_id`、`user_id` 与客户端；标题不进日志（每个写用例的日志测试核对，标题用一个不会出现在别处的字符串）。
- **契约**：`api/modules/page.yaml` 照 `notebook.yaml` 的写法（每个操作 `security`、`x-problem-codes`、`default` 的 Problem 响应；`responses.Problem` 带两个头；对象 `additionalProperties: false`；可空用 `type: [..., "null"]`）；`api/openapi.yaml` 登记 tag 与路径。先手写 `adapter/http/gen/oapi-codegen.yaml`，再 `make gen`。必填又可空的响应字段（`parent_id`）用 `nullable.NewNullNullable` 显式写空。

### 3.8 权限

- `access/domain/rules.go` 加 `writers()`（笔记本的管理员与编辑者）与四行：`node.list`、`page.read` 给 `readers()`，`page.create`、`node.rename` 给 `writers()`，都是 `LevelNotebook`。
- `page.Actions()` 加进 `bootstrap/actions_test.go` 的并集，与规则表同一步。
- 判定读的是锁下的事实：页面的写先锁工作区行与笔记本行，再 `Authorize`（3.6），工作区成员关系、笔记本成员关系与 `workspace_access` 的变化都与它串行。

### 3.9 笔记本删除的注册者

- `page.NewNotebookDeletion(pool)`（P4 起是 `NewNotebookDeletion(pool, subscribers)`：组合根从 `pageRegistrants().sessionSubscribers` 交来编辑会话的订阅者，M4/P4 3.7）：订阅 notebook 的笔记本删除事件，在同一事务里、以事件的时刻：软删除这些笔记本里未删的节点，以及这些节点未删的正文、版本、条目，再软删除这些笔记本未删的变更集。已经在回收站里的节点与它们跟随的行保留原来的时刻（按自己的保留期清理）。
- 值逐字段与 `notebook.NotebookDeletion` 相同，组合根转换（`registrants.go` 的 `pageNotebookDeletion`）。
- 组合根：`notebookRegistrants()` 改为 `notebookRegistrants(pool)`，`deletionSubscribers` 交出页面的注册者；两个调用处（`deps.go` 与 `workspaceRegistrantsWith` 的调用）同步。命令行的组合也会经 `workspaceRegistrants` 走到它，所以它只凭连接池构造，不叫 `New`（`archtest/composition_test.go`）。
- 加锁：三条发布路径都已持笔记本行的独占锁（删除工作区时还有工作区行），注册者直接写节点（13.1 第 5 条）。

### 3.10 清理

`Purgers(pool)`，叶在前：`changeset_items`、`page_revisions`、`page_contents`、`nodes`、`changesets`。组合根的 `purgers(pool)` 改为 `page`、`notebook`、`workspace` 的顺序。

- 前三个照 notebook 的写法：一条语句删一批 `deleted_at` 早于保留期的行（`FOR UPDATE SKIP LOCKED`）。
- `nodes`："删叶子"的语句只删没有任何子节点（不论是否删除）、也没有跟随的行（正文、版本、条目，不论是否删除）的节点，`FOR UPDATE SKIP LOCKED`。清理器在一次调用里反复执行它，每次的 `LIMIT` 是这一批剩下的数，直到某次删不出或凑满一批；ctx 里没有事务，每条语句各自提交、看得见前一条的删除（不能包进 `WithinTx`）。跟随的行被前面的清理器跳过时，`NOT EXISTS` 让它们的节点也留下，节点的删除从不去碰被持有的行。
- `changesets`：只删已没有条目与版本的（`NOT EXISTS`）。
- 测试（page 的适配器，照 `notebook/adapter/postgres/purge_test.go`）：保留期前后与边界；按批（`batch = 1` 时调用次数）；三层的树一次调用清完；子页被别的事务持有时，别的叶子照删，被持有的子页与它的父页留下、不等锁（带期限的上下文），放开之后下一次调用清完；正文、版本、条目或变更集被别的事务持有时（表格测试，各持一种），需要它的节点与变更集留下、不等锁，下一次调用清完。
- 整个清理任务（`bootstrap/purge_test.go` 的 `TestThePurgeDeletesWhatOutlivedTheRetention`）：每本笔记本下种三层节点与跟随的行，时刻随笔记本；`live` 里另种一个 61 天前删除的子树。断言超期的页面行全部消失、其余不动；另断言没有任何清理任务带 `errors`（失败一次再重试成功的，状态照样是 `completed`，只等完成测不出"任务没有错误"；另一条查询给出错误本身，比等 `errors IS NULL` 超时好）。反向对照：只删当前叶子、不循环，这个测试失败（notebook 的清理器因外键失败）。
- `TestPurgersComeBeforeTheTablesTheyReference` 与 `TestCrossModuleForeignKeysToPurgedTablesRestrict` 不用改：它们按外键检查新表（自引用与复合自引用不在其内）。

### 3.11 加锁

- 写：工作区行 `FOR SHARE` → 笔记本行（树：`FOR NO KEY UPDATE`）→ 判定 → 节点与跟随的行。新建与改名都是树操作，同一笔记本里串行。
- 单元总是最外层的事务：`TxManager` 嵌套时直接加入外层事务、没有保存点（`platform/postgres/tx.go`），单元若嵌在别的事务里就提交不了、也不能保证"一个事务一次事件"。`Run` 在已有事务时报错，操作在 ctx 不带单元的事务时报错，都由测试守住。page 的 `Tx` 端口是 `WithinTx` 加 `InTx`，组合根交 `TxManager`：`app` 不能导入平台包，`TxManager.InTx` 是 `postgres.InTx` 的一行方法。
- 读不开事务、不加锁（8.3）。

### 3.12 权限矩阵与树的一致

- **种子**：`seeded` 加 `pages`，`newSeeded` 定好 id。priv 放根页面与它的子页面（核对 `getPage` 的祖先链），team、wiki、orphan 各一个页面，gone-nb 的页面与笔记本同一时刻经 SQL 软删除；每个页面同时写正文行，与它自己的变更集、条目与版本 1（`seededPageHistory`），页面的不变量 `checkPages` 在这份数据上成立。`workspaceOfRow` 认得页面（经它的笔记本）。
- **四行**（`permission_matrix_page_test.go`，`notebookColumns()`）：
  - `listNodes`、`getPage`：有角色的列 200，其余 404（`notebook.not_found` 与 `page.not_found`）。
  - `createPage`、`renameNode`（`write`）：笔记本管理员、编辑者、默认编辑者、无主笔记本的成员 2xx；阅读者访客、默认阅读者、显式阅读者访客 403 `forbidden`；其余 404。加一个 `editorsOnly` 辅助。
- **树与逐项读取一致**（`page_visibility_test.go`，照 `notebook_visibility_test.go`）：对每一列，列出每本笔记本的树，再对种下的每个页面逐个 `getPage`；答 200 的集合等于树里的集合，名称与父页相同，`ancestors` 等于从树上推出的链。种子含子页面与已删除的页面，测试在这份数据上调用 `checkPages`。

### 3.13 交错与不变量

- `checkPages(t, pool)`：一组计数为 0 的查询：未删节点的父页未删、在同一笔记本（复合外键已保证同一笔记本，这里查"父页未删"）；未删的兄弟名称键唯一（唯一索引已保证，查询照样写，防索引被改）；每个未删的页面恰有一行未删的正文，它的 `revision` 等于这一页最新的版本行的 `revision`；层级不超过 10；无环（递归查询走到根，P2 起有意义）。每个交错测试结束时与 `checkNotebooks` 一起调用。
- 交错（`interleavings_page_test.go`）：

  | # | 交错 | 持有的行 | 断言 |
  |---|---|---|---|
  | 30 | 删除笔记本与新建页面、与改名 | 笔记本行 | 删除在先：新建、改名答 404；新建在先：新建成功、页面随笔记本以同一时刻软删除 |
  | 31 | 删除工作区与新建页面 | 工作区行 | 同上，经工作区删除的注册者 |
  | 32 | 同一父页下同时新建同名的页面 | 笔记本行，`FOR SHARE`（3.4） | 一个 201、一个 409 `page.title_taken` |
  | 33 | 新建页面与把他移出工作区 | 工作区行 | 移出在先：新建答 404；新建在先：页面在，之后他看不到 |

  每个交错两种先后各一个用例。

### 3.14 端到端

- **fixture**：`e2e/fixtures/pages.ts`（`postPage`、`getTree` 返回答复，`createPage`、`listNodes` 核对状态码之后返回数据；`renameNode`、`getPage` 返回答复）；`assert/page.ts`（`expectNewPage`：节点、空正文、版本 1、变更集的客户端与执行者、条目的父页与次序等于节点的；`expectRenamed`：条目的前后标题与位置、改名的人与时刻（`updated_at` 等于变更集的时刻）；`expectPagesDeletedWith`：节点与跟随的行的 `deleted_at` 等于笔记本的）；`purge.ts`：把 n13 的"把删除推到保留期之前"移出来并加上页面的表，从叶到根推（只推笔记本而不推它的页面，清理任务会因外键连续失败）。推后的语句之间运行的清理看不到半推后的树；跨越推后的清理仍可能看到、失败、由 River 重试，所以 PG13 只看推后之后入队的清理任务。
- **故事**（接口版本，页面版本在 P5）：
  - PG1：编辑者在根下与子页下新建；阅读者 403；三种 422（空标题、禁止的字符、保留名）；不存在的父页 422；落库。
  - PG2：工作区成员（笔记本对工作区开放为 `editor`）改管理员建的页；422；`Straße` 与 `STRASSE`、NFC 与 NFD 答 409；只差大小写的改名成功；落库的条目。
  - PG11：三层页面的 `getPage` 祖先链；`listNodes` 的先序。
  - PG12（新建与改名的部分）：阅读者 403（工作区访客加显式的阅读者成员：开放为 `editor` 的笔记本里工作区成员默认是编辑者，高于显式的阅读者）、看不到笔记本的人 404、经 `workspace_access = editor` 的成员可以新建与改名。
  - PG13（接口部分）：删除笔记本、删除工作区、删除无主笔记本各自连带页面（同一时刻）；推到保留期之前之后，清理任务清掉三层页面、笔记本与工作区，推后之后入队的清理任务没有错误。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `shared.TitleKey` 与 archtest；notebook 的 `NewNotebooks` 端口；`schema_test` 的 `NULLS NOT DISTINCT` 标记 | [P1-S1](plans/P1-S1-title-key-ports.md) |
| S2 | 迁移、sqlc、grants、schema 测试；page 的领域与仓储；清理器与它们的测试；组合根的清理顺序 | [P1-S2](plans/P1-S2-page-data.md) |
| S3 | 写入单元、四个用例、扩展点、契约与 HTTP、规则表、组合根与笔记本删除的注册者、文案；矩阵的四行与种子 | [P1-S3](plans/P1-S3-unit-api.md) |
| S4 | 树的一致、笔记本删除的三条路径、整个清理任务、交错 30–33 与 `checkPages` | [P1-S4](plans/P1-S4-matrix-interleavings.md) |
| S5 | e2e 的 fixture 与五个故事的接口版本 | [P1-S5](plans/P1-S5-e2e.md) |

迁移一加上表，清理的注册表测试、`schema_test` 与运行时角色的测试就要求清理器、名单与授权齐全，所以它们都在 S2。`page.yaml` 一进入 `api/dist`，矩阵的覆盖检查、路由与契约一致的检查、文案的测试就要求行、注册与文案齐全，所以它们都在 S3 与 S4 的同一个提交序列里，S3 的提交带上矩阵的四行与种子。S1、S3 各两个提交（S3 分成写入单元与用例、接口与接线）。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | 标题键；次序（中间值、两端、连续插入与重排）；先序；层级；改动的合并；客户端的取值；表格驱动 |
| 仓储 | 插入节点、正文、变更集、条目、版本，同一时刻；按笔记本列出未删的节点；祖先的递归查询；锁笔记本行（等锁、看见删除）；23505 → `page.title_taken`；笔记本删除连带的软删除只动未删的行；清理（3.10） |
| 用例 | 码的次序；单元的顺序（判定在锁下、守卫在检查之后、观察者一次）；多个守卫、参与者、观察者按登记的顺序，第一个守卫拒绝即停；操作拒绝不带单元事务的 ctx；答复是单元结束时的页面；日志；变更集与条目、版本；改成同名不写 |
| 扩展点 | 模块根（接好线的模块与真实数据库）：守卫拒绝时整体回滚并答出它的码，守卫看到前后状态；观察者在事务里收到一次事件，出错回滚；参与者追加的操作经守卫、进同一变更集、并进事件；测试里的两操作单元一个变更集、一次事件；客户端按凭证；单元在已有事务里报错 |
| 契约 | `apitest.Main`：每个声明的码答出一次；id 不是 uuid 时 400 |
| 整个程序 | 矩阵四行；树与逐项读取一致；笔记本删除的三条路径（反向对照：组合根任一路径交空，对应的测试失败）；整个清理任务；交错 30–33 |
| 端到端 | PG1、PG2、PG11 的接口版本，PG12、PG13 的接口部分 |

反向对照（每个新检查各一个，13.4 第 1 条）：`TitleKey` 不做第二次 NFC；`ShareByID` 不检查事务；单元先调守卫再做检查；观察者每个操作调一次；参与者追加的写不经守卫；`nodes` 的清理器不循环；"删叶子"不看跟随的行；笔记本删除的注册者也改已删的行；`workspaceOfRow` 不认页面；`checkPages` 的某条查询写错。实际做过的反向对照与结果见[审查记录](reviews/P1-page-module-pipeline-review.md)的"反向对照"与"发现与处置"。

## 6. 完成标准

- 第 5 节的测试全部通过，`GOFLAGS=-p=3 make check`、`make gen-check`、前端的检查（lint、format、types、knip）为绿；持续集成为绿。
- 审查（Opus）完成，发现已处理，记录在 `reviews/P1-page-module-pipeline-review.md`。
- M3/P1 移交第 1、3 项与 M2/P4 移交第 1、2、4 项落实，00 号文档第 7 节的落实一栏核对。
- 00 号文档的进度表与本文的"结果"更新。

## 7. 结果

- 分支 `m4-p1-page-module-pipeline`：S1 `f005847`、`9d81bcf`；S2 `b1e2f9e`；S3 `ff86cb6`、`e065bcd`；S4 `b520cae`；S5 `4f64a84`；审查修复 `3ec2c9d`；`bb90d03` 合并（`--no-ff`）。
- 门禁：每个 Step 的 `make check` 为绿；`make gen-check`、`make e2e`（122 个）、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P1 审查](reviews/P1-page-module-pipeline-review.md)，没有 Critical、Major，没有生产代码的缺陷；8 项 Minor 与 7 项 Nit 是测试守不住的行为、会偶发失败的断言与小的整理，全部在合并前处理，每个新检查都做了反向对照。
- 移交：M2/P4 第 1、2、4 项，M3/P1 第 1、3 项落实（00 号文档第 7 节）。

**与设计的偏差**（已同步进上文）：

1. `Run` 把 ctx 交给 `do`，用例再交给各操作；操作在 ctx 不带单元的事务时报错（3.6）。
2. 守卫与参与者的值合成一个 `app.Step`；观察者的值是 `Event`（3.6）。
3. page 的 `Tx` 端口是 `WithinTx` 加 `InTx`，`platform/postgres` 加 `TxManager.InTx`（3.1、3.11）。
4. 矩阵的四行与种子在 S3：`page.yaml` 一进入 `api/dist`，覆盖检查就要求它们（第 4 节）。
5. `Outcome` 带执行者；`Position` 是可比较的值，零值是最后（3.6、3.7）。
6. 新建与改名在单元里、操作之后重读这一页作答复（3.6）。
7. 整个清理任务的测试另断言没有清理任务带 `errors`；PG13 只看推后之后入队的清理任务（3.10、3.14）。
8. PG12 的阅读者是工作区访客加显式的阅读者成员（3.14）。
9. `Change` 不带种类，操作的种类是 `Operation`，在 `app.Step` 上；`TreeState` 在 `change.go`，`MaxDepth` 在 `node.go`；祖先链的完整由仓储核对（3.5）。
10. 交错 32 持笔记本行的 `FOR SHARE`，证明新建因为锁树而串行（3.4、3.13）。
11. 3.1 的文件清单、3.3 改注释的文件、3.4 的 `nodes_sort_order_check`、3.14 的 fixture 按实际改写。

**留给后面的**：`unit.go` 在 P2 开工时按操作拆分（P1 审查 Q1）；事件里没有重排的兄弟，客户端收到事件就重读整棵树（[M5 的移交](../M5-collab-editing/handoffs/M4-P1-tree-refresh.md)）；`changesets_client_check` 的 `[:cntrl:]` 随数据库的区域设置（[M9 的移交](../M9-mcp/handoffs/M4-P1-client-check.md)）；13.1 第 6、11、21 条的例子补上本 Phase 的（M4 收尾，P1 审查 D11）。
