# M3/P1 笔记本与权限：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M3/P1 笔记本与权限 |
| 状态 | 进行中 |
| 基线 | M3 总设计按设计审查修订之后的 main |
| 上级文档 | [M3 总设计](00-M3-design.md) 第 4、5、7、8 节；[M2 移交](handoffs/M2-workspace.md)第 4 项；[总体设计](../v0.1-design.md) 3.3、3.5、6.1、7、13 |

---

## 1. 基线

M2 留下的：

- workspace 模块：工作区、成员、邀请；四个扩展点（成员身份结束、恢复、删除，都还没有注册者）；`workspace.NewMemberships(pool)` 是 access 的事实端口。
- access 模块：规则表只有工作区级（`LevelWorkspace`），判定是纯函数，事实经端口读。
- 清理注册表：`bootstrap/registrants.go` 的 `purgers(pool)` 只有 workspace 的两个清理器。
- 权限矩阵：六列都是工作区级的身份，每行都按这六列。
- 前端没有笔记本的入口。

本 Phase 接手 M2 移交第 4 项（清理器）。

## 2. 目标与范围

**目标**：笔记本的第一批能力（建、列、读、改、删）跑在笔记本级的判定之上。笔记本级的规则、事实端口、有效角色与矩阵的笔记本列一次建好，后面的 Phase 只加行。

**做**：

- `shared`：`CheckTitle`（页面标题的规则，笔记本名称先用）；`NotebookRole`、`WorkspaceAccess` 与有效角色；`Target`、`Grant` 的笔记本字段。
- access 的笔记本级：`LevelNotebook`、事实端口 `NotebookFacts`、判定。
- workspace 给笔记本模块的端口：按 slug 找工作区（答它的 id），按 id 以 `FOR SHARE` 锁工作区行。
- notebook 模块：迁移 `notebooks`、`notebook_members`；`listNotebooks`、`createNotebook`、`getNotebook`、`updateNotebook`、`deleteNotebook`；笔记本删除事件；订阅工作区删除；清理器。
- 组合根：notebook 的 `Deps`、access 的第二个端口（经 `bootstrap/notebook_facts.go` 转换类型）、工作区删除的注册者、清理器的顺序。
- 新码 `notebook.not_found` 的中英文案（前端 `problem-messages.test.ts` 要求契约里的每个码都有）。
- 测试：名称规则、判定的表格、列表与逐个判定一致、矩阵的笔记本列、跨模块外键是 RESTRICT、交错 15、16 与笔记本的不变量（M3 总设计第 4 节）；工作区删除的订阅者经删除工作区的行为测试（M2 移交第 1 项的这一条路径）。
- e2e：N1、N3、N6 的接口版本，N2 的私密部分（"第二位成员"在 P2），N13 的笔记本部分与清理（审计在 P3）。

**不做**：成员的操作与可见性事件（P2）；级联、无主、审计、游标（P3）；页面（P4、P5）。

## 3. 设计

### 3.1 文件

```
server/
  migrations/sql/00009_notebook_notebooks.sql、00010_notebook_notebook_members.sql
  migrations/schema_test.go                    新的约束与索引名、CHECK 的反例
  sqlc.yaml                                    notebook 一条
  internal/shared/title.go                     CheckTitle
  internal/shared/notebook_role.go             NotebookRole、WorkspaceAccess、EffectiveNotebookRole
  internal/shared/authorize.go                 Target.NotebookID、Grant.NotebookRole
  internal/modules/access/
    module.go                                  Deps 加 Notebooks
    domain/rules.go、decide.go                 LevelNotebook；本 Phase 的五行
    app/ports.go、authorizer.go                NotebookFacts；按级别读事实
  internal/modules/workspace/
    workspaces.go                              NewWorkspaces(pool)：给笔记本模块的端口
    adapter/postgres/queries/workspaces.sql    ShareWorkspaceByID
  internal/modules/notebook/
    module.go                                  New(Deps)、Register、Actions()
    facts.go                                   NewFacts(pool)：access 的笔记本级事实
    deletion.go                                NewWorkspaceDeletion(pool, subscribers)：工作区删除的注册者
    purgers.go                                 Purgers(pool)
    domain/notebook.go、actions.go、errors.go
    app/ports.go、extension.go、authorize.go、create_notebook.go、list_notebooks.go、get_notebook.go、
        update_notebook.go、delete_notebook.go、workspace_deletion.go
    adapter/postgres/（store、queries/notebooks.sql、members.sql、gen）
    adapter/http/（handler、gen、main_test.go）
  internal/bootstrap/
    deps.go、wire.go、registrants.go           notebookDeps；access 的两个端口；注册者与清理器的顺序
    actions_test.go                            并集加上 notebook.Actions()
    purge_test.go                              跨模块外键不是 CASCADE
    permission_matrix_test.go                  行按级别取列
    permission_matrix_seeded_test.go           笔记本列的账户与笔记本
    permission_matrix_notebook_test.go         本 Phase 的行
    notebook_visibility_test.go                列表与逐个判定一致
    interleavings_notebook_test.go             交错 15、16
api/modules/notebook.yaml、api/openapi.yaml
deploy/runtime-grants.sql
e2e/fixtures/notebooks.ts、assert/notebook.ts；e2e/stories/notebook/n1、n2、n3、n6、n13
```

### 3.2 名称规则：`shared.CheckTitle`

总体设计 3.5 的规则，笔记本名称先用，M4 的页面标题复用：

1. 去掉首尾空白（与 `CheckName` 相同：页面上输入的空格不让保存失败）。
2. NFC 规范化（`golang.org/x/text/unicode/norm`，13.1 第 12 条允许的例外）。
3. 依次判断，每个只报一个码：
   - 空：`required`；
   - 超过 255 字节（UTF-8）：`too_long`；
   - 含 `/ \ : * ? " < > | # ^ [ ]`、控制字符、行与段落分隔符、双向控制字符：`invalid_format`；
   - 以 `.` 开头或结尾：`invalid_format`；
   - Windows 保留名（`CON`、`PRN`、`AUX`、`NUL`、`COM1`–`COM9`、`LPT1`–`LPT9`，不区分大小写；第一个 `.` 之前的部分是保留名也算，如 `con.txt`）：`not_allowed`。
4. 返回规范化之后的名称。
- 规范化在判断之前：组合字符序列的字节数以 NFC 为准，存进库的也是 NFC。
- 不做标题键：笔记本名称不要求唯一，键由 M4 随页面一起加。
- 表格测试：每种禁止的字符各一例、全角字符允许、255 与 256 字节（多字节字符跨界）、NFD 输入得到 NFC、保留名的大小写与扩展名、`COM10` 允许、只有空白、首尾的点。

### 3.3 笔记本的角色与有效角色（`shared`）

- `NotebookRole`：`admin`、`editor`、`reader`；`NotebookRoles()`。与 `WorkspaceRole` 是不同的类型：两套词汇里都有 `admin`。
- `WorkspaceAccess`：`none`、`viewer`、`editor`；`WorkspaceAccesses()`。
- `EffectiveNotebookRole(explicit NotebookRole, access WorkspaceAccess, workspace WorkspaceRole) NotebookRole`：
  - 默认角色：工作区角色是 `admin` 或 `member` 时，`viewer` 给 `reader`，`editor` 给 `editor`；其余没有。
  - 返回显式角色与默认角色中较高的一个，高低经次序表（`reader` 1、`editor` 2、`admin` 3）；三个值以外的角色排在最低，什么都不给；两者都没有时返回空。
- 放在 `shared`：access 的判定与笔记本列表都用它，两处必须一致（Nerve 11.4：两个模块共用的纯取值规则）。

### 3.4 权限：access 的笔记本级

- **`shared/authorize.go`**：`Target{WorkspaceID, NotebookID}`；`Grant{WorkspaceRole, NotebookRole}`，`NotebookRole` 是有效角色。工作区级的调用不填笔记本的字段，照旧。
- **`access/domain/rules.go`**：
  - `LevelNotebook`；`Rule` 加 `Notebook []shared.NotebookRole`。
  - 本 Phase 的五行：

    | 操作 | 级别 | 允许 |
    |---|---|---|
    | `notebook.list` | 工作区 | 三种角色（列表按可见性过滤） |
    | `notebook.create` | 工作区 | 管理员、成员 |
    | `notebook.read` | 笔记本 | 三种笔记本角色 |
    | `notebook.update` | 笔记本 | 管理员 |
    | `notebook.delete` | 笔记本 | 管理员 |

- **`access/domain/decide.go`**：`Facts` 加 `Notebook Notebook{Found bool; Access WorkspaceAccess; Role NotebookRole}`（`Role` 是显式角色）。笔记本级：
  1. 不是工作区的有效成员 → `ErrNotVisible`；
  2. 笔记本不存在、已删除、不属于目标的工作区 → `ErrNotVisible`；
  3. 有效角色为空 → `ErrNotVisible`（工作区管理员看不到别人的私密笔记本在这里成立）；
  4. 有效角色不在规则的集合里 → 403；
  5. 否则 `Grant{WorkspaceRole, NotebookRole: 有效角色}`。
- **`access/app/ports.go`**：`NotebookFacts.NotebookFacts(ctx, notebookID, userID) (NotebookFact, error)`，`NotebookFact{Found, WorkspaceID, Access, Role}`：未删除的笔记本，以及调用者在其中有效的显式成员关系。
- **`access/app/authorizer.go`**：按规则的级别读事实。笔记本级先读笔记本的事实（得到它所属的工作区），工作区与目标不同时当作不存在；再读工作区的角色。每次调用都读，不缓存。
- **实现**：`notebook.NewFacts(pool)`，一条查询，经 `postgres.DB(ctx, pool)` 进入调用方的事务。
- **装配**：`access.New(access.Deps{Memberships: workspace.NewMemberships(pool), Notebooks: notebook.NewFacts(pool)})`。

### 3.5 workspace 给笔记本模块的端口

笔记本模块不读工作区的表（sqlc 按模块限定）。workspace 模块根加只凭连接池的 `NewWorkspaces(pool) Workspaces`：

- `FindBySlug(ctx, slug) (uuid.UUID, bool, error)`：未删除的工作区的 id，不加锁；slug 不合格式时直接答没有，不查库（与 `getWorkspace` 相同，路径里的 NUL 进不了数据库）。笔记本的用例只要 id：回答里是 `workspace_id`。
- `ShareByID(ctx, id) (bool, error)`：以 `FOR SHARE` 锁住未删除的工作区行，在调用方的事务里；事务之外调用报错（同 `ShareActiveAccount`，事务之外的锁随语句结束）。读到 0 行（等锁期间被删除）答没有。
- 两个方法只用 `uuid` 与基本类型：笔记本模块在 `app/ports.go` 声明同形的端口，组合根直接把 workspace 的实现交给它，不必转换。
- access 的笔记本级事实不同：`notebook.Fact` 与 `access.NotebookFact` 逐字段相同、属于两个模块，由 `bootstrap/notebook_facts.go` 转换（照 `bootstrap/directory.go`）。

### 3.6 数据

**`notebooks`**：

| 列 | 类型与约束 |
|---|---|
| `id` | `uuid PRIMARY KEY` |
| `workspace_id` | `uuid NOT NULL REFERENCES workspaces ON DELETE RESTRICT`：跨模块，每一行只由自己模块的清理器删除（13.1 第 6 条） |
| `name` | `text NOT NULL`，`notebooks_name_check CHECK (name <> '' AND octet_length(name) <= 255)`；其余规则在 `CheckTitle` |
| `workspace_access` | `text NOT NULL DEFAULT 'none'`，`notebooks_workspace_access_check CHECK (workspace_access IN ('none', 'viewer', 'editor'))` |
| `ownerless_since` | `timestamptz`：成为无主的时刻（P3 写） |
| `former_owner_id` | `uuid REFERENCES users`：原所有者（P3 写）；`notebooks_ownerless_check CHECK ((ownerless_since IS NULL) = (former_owner_id IS NULL))` |
| `created_by_id`、`updated_by_id` | `uuid NOT NULL REFERENCES users` |
| `created_at`、`updated_at` | `timestamptz NOT NULL` |
| `deleted_at` | `timestamptz` |

- `notebooks_workspace_id_idx ON (workspace_id)`：列表，以及清理工作区时外键的反向查找（含已删除的行，不用部分索引）。
- 无主的两列在本 Phase 就建：同一张表在同一个 M 里不另写一条 `ALTER`。P1 的查询不读它们，P3 读写；CHECK 的反例在本 Phase 测。

**`notebook_members`**：

| 列 | 类型与约束 |
|---|---|
| `id` | `uuid PRIMARY KEY` |
| `notebook_id` | `uuid NOT NULL REFERENCES notebooks ON DELETE CASCADE`：同一模块，清理器先删成员行，级联只是后备（13.1 第 6 条） |
| `user_id` | `uuid NOT NULL REFERENCES users` |
| `role` | `text NOT NULL`，`notebook_members_role_check CHECK (role IN ('admin', 'editor', 'reader'))` |
| `ended_at` | `timestamptz`：移出、离开、级联的时刻；恢复时清空 |
| `created_by_id`、`updated_by_id`、`created_at`、`updated_at`、`deleted_at` | 同上；`deleted_at` 只随笔记本 |

- `notebook_members_notebook_id_user_id_key ON (notebook_id, user_id) WHERE deleted_at IS NULL`：每对只有一行。
- `notebook_members_user_id_idx ON (user_id) WHERE deleted_at IS NULL AND ended_at IS NULL`：级联时列举他的成员关系（P3）。
- `notebook_members_notebook_id_idx ON (notebook_id)`：成员数，以及清理时的反向查找。

**其他**：`sqlc.yaml` 加 notebook 一条，只列 00009、00010；`runtime-grants.sql` 给两张表 DML；`schema_test` 列出新的约束与索引名，CHECK 的反例：空名称、256 字节的名称、第四种开放程度、大写的开放程度、第四种角色、只有 `ownerless_since` 而没有原所有者（与反过来）。

### 3.7 接口与用例

`api/modules/notebook.yaml` 的五个操作，都是 Bearer：

| 操作 | 成功 | `x-problem-codes` |
|---|---|---|
| `GET /api/v0/workspaces/{slug}/notebooks`（`listNotebooks`） | 200 `{data: Notebook[]}` | `workspace.not_found` |
| `POST /api/v0/workspaces/{slug}/notebooks`（`createNotebook`），`{name, workspace_access?}` | 201 `Notebook` | `workspace.not_found`、`forbidden`、`validation_failed` |
| `GET /api/v0/notebooks/{notebook_id}`（`getNotebook`） | 200 `Notebook` | `notebook.not_found` |
| `PATCH /api/v0/notebooks/{notebook_id}`（`updateNotebook`），`{name?, workspace_access?}` | 200 `Notebook` | `notebook.not_found`、`forbidden`、`validation_failed` |
| `DELETE /api/v0/notebooks/{notebook_id}`（`deleteNotebook`） | 204 | `notebook.not_found`、`forbidden` |

- **结构**：`Notebook {id, workspace_id, name, workspace_access, role, member_count, created_at, updated_at}`。`workspace.not_found` 由笔记本模块答出：它的 `domain/errors.go` 有同码的错误值（码是字符串，按调用方点名的东西给）。
- **`listNotebooks`**：
  1. `FindBySlug`，没有就 404 `workspace.not_found`；
  2. `Authorize(notebook.list, {WorkspaceID})`：`ErrNotVisible` 答同一个 404；
  3. 一条查询：这个工作区里未删除的笔记本中，调用者有有效的显式成员关系的，或者（他的工作区角色是管理员或成员，且开放程度不是 `none`）的；带显式角色、开放程度、有效的显式成员数。按名称不分大小写、名称、`id` 排序；
  4. 每一项的 `role` 由 `EffectiveNotebookRole` 算出。
  - 读操作不开事务。
- **`createNotebook`**：一个事务：
  1. `FindBySlug`（不加锁）、`ShareByID`：都答没有就 404；
  2. `Authorize(notebook.create)`；
  3. `CheckTitle`、开放程度的取值（缺省 `none`）：一次列出全部字段问题（422）；
  4. 插入笔记本与创建者的成员行（`admin`），同一个时刻。
  - 回答带 `role: admin`、`member_count: 1`；日志记 `workspace_id`、`notebook_id`、`user_id`。
  - 取值的检查在锁与判定之后（13.1 第 4 条）：不是成员的人发来不合规则的名称，得到 404 而不是 422。
- **`getNotebook`**：`Authorize(notebook.read, {工作区, 笔记本})` 之前要知道工作区：先不加锁读笔记本（未删除），没有就 404；判定的 `ErrNotVisible` 答同一个 404 `notebook.not_found`。回答带 `Grant` 的有效角色与成员数。
- **`updateNotebook`**：
  1. 不加锁读笔记本，得到工作区；没有就 404；
  2. 事务：`ShareByID(工作区)` → 以 `FOR NO KEY UPDATE` 锁笔记本行（带 `deleted_at IS NULL`，读到 0 行答 404）→ `Authorize(notebook.update)` → 校验（422；空的请求体是 400，由契约的 `minProperties: 1` 拒绝）→ 改；
  3. 回答带有效角色（管理员）与成员数。
  - 名称与开放程度都没有变化时不写，回答照旧（`updated_at` 不动）。
- **`deleteNotebook`**：同样先读、再锁工作区与笔记本、判定；以同一个时刻软删除笔记本与它的全部成员行（含已结束的），然后调用笔记本删除事件的订阅者。204。
- **按 id 寻址的参数**：`{notebook_id}` 是 `format: uuid`，不合格式由边界答 400（与 `{workspace_member_id}` 相同）。

### 3.8 笔记本删除事件与工作区删除的注册者

- **`app/extension.go`**：`NotebookDeletion{WorkspaceID, NotebookIDs []uuid.UUID, By, At}`；`NotebookDeletionSubscriber.NotebookDeleted(ctx, d) error`。在笔记本行软删除之后、同一事务内调用；错误整体回滚。模块根以类型别名公开，`Deps.DeletionSubscribers` 交进去。
- **工作区删除的注册者**：`notebook.NewWorkspaceDeletion(pool, subscribers) WorkspaceDeletion`，只凭连接池构造（13.1 第 21 条），`WorkspaceDeleted(ctx, d)`：
  1. 以事件的时刻、删除者软删除这个工作区里未删除的全部笔记本，返回它们的 id；
  2. 同一时刻软删除这些笔记本的成员行；
  3. 有笔记本时调用笔记本删除事件的订阅者一次，带全部 id。
  - 工作区行已由删除持有 `FOR NO KEY UPDATE`，同一工作区下不会有别的笔记本写在进行（它们都要工作区行的 `FOR SHARE`）：批量的 `UPDATE` 按扫描顺序加锁没有风险（Nerve 约定五）。
  - 组合根：`workspaceRegistrants(pool)` 返回的 `deletionSubscribers` 加上它（值逐字段转换，`bootstrap/registrants.go`）。workspace 的命令行组合不涉及删除，不变。
- **测试替身**：模块根的测试（接好线的模块与真实数据库）证明订阅者在事务内、看得到已删除的行、失败整体回滚；两个订阅者的分发由 `app/registrants_test.go` 那样的测试守住（13.1 第 21 条）。

### 3.9 清理

- `notebook.Purgers(pool)`：先 `notebook_members`，再 `notebooks`（只删已经没有成员行的，`NOT EXISTS`，P4 审查 T1 的写法）。
- 组合根 `purgers(pool)`：`slices.Concat(notebook.Purgers(pool), workspace.Purgers(pool))`。
- **跨模块外键不是 CASCADE**（M2 移交第 4 项）：`purge_test.go` 加 `TestCrossModuleForeignKeysToPurgedTablesRestrict`：指向被清理表、来自别的模块迁移的外键，`confdeltype` 不是 `c`；模块取自定义约束的迁移文件名（`NNNNN_<归属>_…`）。反向对照：把 `notebooks.workspace_id` 改成 `ON DELETE CASCADE`，测试失败。

### 3.10 加锁

- 笔记本的管理写：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定（总体设计 13.1 第 5 条）。
- 创建只锁工作区行：新行在本事务里插入，别人还看不到。
- 交错（第 5 节）：
  - **15 删除工作区与建笔记本**：删除先，建笔记本等锁之后读到工作区已删除，答 404，没有新笔记本；建先，删除连带软删除这个新笔记本与它的成员行。
  - **16 删除工作区与改、删笔记本**：删除工作区先，改或删笔记本的一方锁工作区行时读到它已删除，答 404，笔记本保持随工作区软删除的样子；改或删笔记本先，删除工作区等它提交，再连带软删除（改过的笔记本以工作区的时刻删除；已删的那本保留它自己的时刻，不重复删除）。
  - 每个交错两种先后各一个用例，结束时核对笔记本的不变量（M3 总设计第 4 节）：未删除的笔记本都有有效的管理员（本 Phase 没有无主）。断言放在 `bootstrap` 的交错辅助里，P2、P3 的交错共用。

### 3.11 权限矩阵

- **框架**：`matrixRow` 加 `columns []caller`，缺省是工作区级的六列；笔记本级的行用 `notebookColumns()`。覆盖检查与执行都按行的列，每行仍要给出它每一列的答案。
- **笔记本列**（每列一个账户，目标在 acme 里）：

  | 列 | 账户与准备 | 目标 |
  |---|---|---|
  | 笔记本管理员 | 工作区成员，私密笔记本 `priv` 的显式 `admin` | `priv` |
  | 笔记本编辑者 | 工作区成员，`priv` 的显式 `editor` | `priv` |
  | 笔记本阅读者 | 工作区**访客**，`priv` 的显式 `reader`：访客经显式加入访问 | `priv` |
  | 工作区管理员（局外） | 工作区管理员，不是 `priv` 的成员 | `priv` |
  | 工作区成员（局外） | 工作区成员，不是 `priv` 的成员 | `priv` |
  | 默认编辑者 | 工作区成员，`team`（`editor` 开放）没有显式成员关系 | `team` |
  | 默认阅读者 | 工作区管理员，`wiki`（`viewer` 开放）没有显式成员关系 | `wiki` |
  | 访客（局外） | 工作区访客，不是 `team` 的成员 | `team` |
  | 已结束的成员 | 工作区成员，`priv` 的成员关系已结束 | `priv` |
  | 已删除的笔记本 | 工作区成员，已删除的笔记本 `gone-nb` 的管理员 | `gone-nb` |
  | 工作区之外 | 不是 acme 的成员，在别的工作区里是 `other-nb` 的管理员：读错工作区的角色会让他进来 | `priv` |
  | 访客（显式阅读者） | 工作区访客，`team`（`editor` 开放）的显式 `reader`：有效角色是 `reader` 而不是 `editor`，默认角色不给访客只有这一列抓得到 | `team` |

- **数据**：笔记本、成员行、结束、删除由 SQL 写入（M2 矩阵的做法：id 在准备之前定好）。P2 有了成员的用例之后不改：矩阵关心的是判定，不是准备的途径。
- **本 Phase 的行**：
  - `listNotebooks`：笔记本列；工作区之外答 404 `workspace.not_found`，其余 200，`check` 核对每列看到的正好是它看得到的那几个。
  - `createNotebook`：工作区列，写；管理员、成员 201，访客 403，其余 404。
  - `getNotebook`：笔记本列；前三列、两个默认角色与访客（显式阅读者）200，`check` 核对有效角色；其余 404。
  - `updateNotebook`：笔记本列，写；管理员 200，编辑者、阅读者、两个默认角色与访客（显式阅读者）403，其余 404。
  - `deleteNotebook`：同上，管理员 204。
- **列表与逐个判定一致**（`notebook_visibility_test.go`）：在矩阵的数据上，对每个账户，`listNotebooks` 的结果等于"对 acme 的每个笔记本逐个 `getNotebook`，答 200 的那些"，有效角色也相同。SQL 的过滤与 `EffectiveNotebookRole` 走散时它失败。

### 3.12 端到端

- `e2e/fixtures/notebooks.ts`（经接口建、改、删）、`assert/notebook.ts`（`expectNewNotebook`、`expectNotebook`、`expectNotebookDeletedWithItsMembers`、`countNotebooks`），读数据库，只比较业务列；时刻在 SQL 里比较。
- **N1 的接口版本**：成员建笔记本：201、落库的笔记本与管理员成员行；列表里它是"我的"（`none`，一个成员）；访客答 403；名称的三种 422（禁止的字符、保留名、超长）。
- **N2 的私密部分**：私密笔记本对工作区管理员与别的成员：列表里没有，读取答 404。"第二位成员"在 P2。
- **N3 的接口版本**：改为 `viewer`：成员与管理员读到 `reader`，访客 404；改为 `editor`：成员读到 `editor`。
- **N6 的接口版本**：改名；编辑者改名答 403；删除之后读取答 404，落库：笔记本与成员行同一时刻软删除。
- **N13 的笔记本部分**：删除工作区，它的笔记本与成员行以工作区的时刻软删除；清理之后行都消失，保留期内的不动（照 W12 的写法，经数据库把时刻推到边界）。审计记录的部分在 P3。
- 页面版本在 P4、P5，加进同一个故事文件。

## 4. 实施步骤

分支 `m3-p1-notebooks-access`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `shared`：`CheckTitle`、笔记本角色与有效角色、`Target` 与 `Grant` 的字段；access 的笔记本级（规则、判定、端口、`Authorizer`）；notebook 的操作名（与规则表的五行同一步，操作名的一致性一直是绿的） | [P1-S1](plans/P1-S1-shared-access.md) |
| S2 | 数据：两条迁移、sqlc、授权、schema 测试；notebook 模块的领域、仓储、事实端口的实现与接线；清理器与它们的顺序、跨模块外键是 RESTRICT 的检查（迁移一加上表，`bootstrap` 的清理测试就要求它们）；workspace 的 `NewWorkspaces` | [P1-S2](plans/P1-S2-notebook-data.md) |
| S3 | 接口与用例：`notebook.yaml`、五个用例、笔记本删除事件、工作区删除的注册者与它经删除工作区的行为测试、HTTP 适配器、模块根、组合根的接线、新码的文案 | [P1-S3](plans/P1-S3-notebook-api.md) |
| S4 | 矩阵的笔记本列与本 Phase 的行；列表与逐个判定一致；交错 15、16 与不变量的断言 | [P1-S4](plans/P1-S4-matrix-interleavings.md) |
| S5 | 端到端：N1、N3、N6 的接口版本，N2 的私密部分，N13 的笔记本部分 | [P1-S5](plans/P1-S5-e2e.md) |

每个 Step 结束时 `make check` 为绿；S3 之后 `make gen-check` 为绿；S5 之后 `make e2e` 为绿。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元 | `CheckTitle` 的表格（3.2）；`EffectiveNotebookRole` 的表格（三种显式角色与空 × 三种开放程度 × 三种工作区角色，未知的值）；判定的表格（笔记本级的五种结果）；`Authorizer`：笔记本级先读笔记本、工作区不符当作不存在、端口出错 |
| 集成 | 仓储：插入、列表的过滤与顺序（已删除的笔记本、已结束的成员关系、访客没有默认角色）、成员数只数有效的、锁的语句（已删除读到 0 行、别的事务持锁时等待）、软删除连带成员行的时刻；`ShareByID` 在事务之外报错；CHECK 的反例；模块根：删除事件在事务内、失败回滚；工作区删除的注册者 |
| 契约 | 五个操作的每个码在 notebook 的 `adapter/http` 测试中答出；整个程序的测试自动覆盖新操作 |
| 架构 | notebook 不导入别的模块；access 仍只导入 `shared`；新模块只经模块根接入；sqlc 的范围 |
| 一致性 | 操作名的并集 = 规则表的键；列表 = 逐个判定 |
| 矩阵 | 3.11 |
| 交错 | 15、16 |
| 端到端 | 3.12 |

**反向对照**（13.4 第 1 条）：

- `EffectiveNotebookRole` 把默认角色也给访客 → 矩阵的"访客（局外）"与"访客（显式阅读者）"两列失败；
- 列表的 SQL 去掉"工作区角色是管理员或成员"的条件 → 列表与逐个判定一致的测试失败；
- `decideNotebook` 去掉"有效角色为空即看不到" → 矩阵的"工作区管理员（局外）"一列失败；
- `notebooks.workspace_id` 改为 `ON DELETE CASCADE` → 跨模块外键的检查失败；清理器的顺序反过来 → 顺序的检查与清理的整个程序测试失败；
- 删除笔记本不连带成员行 → 仓储与 N6 的断言失败；
- 建笔记本不锁工作区行 → 交错 15 失败。

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成四个任务为绿。
- M2 移交第 4 项已落实，在 M3 总设计第 7 节的表中核对。
- 审查记录 `reviews/P1-notebooks-access-review.md`；发现的问题已修复或明确移交。
- 本文第 7 节、M3 总设计的进度表已更新。

## 7. 结果

（完成后补写）
