# M5/P1 编辑锁（后端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M5/P1 编辑锁（后端） |
| 状态 | 进行中 |
| 基线 | `edb319e`（M5 总设计与它的审查提交之后的 main）；本文与各 Step 计划提交之后开分支 `m5-p1` |
| 上级文档 | [M5 总设计](00-M5-design.md) 第 3、4.1–4.6、5、7–9 节；[M4/P4 移交](handoffs/M4-P4-edit-sessions.md)；[M4/P4 文档](../M4-pages/04-P4-content-sessions.md) 3.2–3.7；[总体设计](../v0.1-design.md) 3.9、6.1、13.1 |

---

## 1. 基线

M4 留下的：

- **编辑会话**（`page/app/unit_session.go`、`*_edit_session.go`、`queries/sessions.sql`）：
  - 开启在写入单元里，先锁正文行，再问否决者，然后建会话；
  - 心跳、结束各是一条语句，只碰本人的会话行；
  - 带会话的写在正文行之后以 `FOR UPDATE` 锁会话行（`writersSession`），核对本人、客户端、页与租约；
  - 删页、删子树、删笔记本删掉会话，告诉订阅者；过期的由定时任务清理，不告诉谁。
- **活着**：`EditSession.Alive(now)` 只看 `expires_at > now`；租约 60 秒、心跳 20 秒（`domain/session.go`）。
- **扩展点**：会话的否决者（`SessionOpening{Write, PageID}`）、会话结束的订阅者、写入守卫（`Step` 带 `EditSessionID`）。组合根 `pageRegistrants()` 交空集合，`notebookRegistrants(pool)` 从它取订阅者。
- **problem**：`httpserver.Problem` 的成员是固定的（`status`、`code`、`title`、`detail`、`errors`）；契约 `Problem` 是 `additionalProperties: false`；平台经可选接口取字段错误与 `Retry-After`。
- **名称**：identity 的 `Directory.Profiles`，notebook 经组合根的 `notebookProfiles` 转换使用。

## 2. 目标与范围

**目标**：一页至多一个活着的编辑会话，锁就是它；不带持锁会话的正文写、别的账户的删除被拒，答复带持锁人；同一账户可以接管，笔记本的管理员可以强制解锁；被接管、被解锁的会话留下墓碑，它们的保存与心跳答各自的码。

**做**：

- 迁移 00018：`ended_reason`、`ended_by_id`、`ended_at` 与检查。
- 领域：租约 120 秒；结束的原因 `taken_over`、`unlocked`；三个新码；新动作 `page.release_edit_lock`；"活着"的统一定义。
- 写入单元：开启时删掉这一页过期的行、接管、`Unlock`；`writersSession` 认出墓碑；心跳与结束认出墓碑。
- 会话的订阅者另得到"开启"（扩充 M4 的扩展点）。
- 模块根的 `NewEditLock(pool, names)`：否决者与守卫；组合根 `pageRegistrants(pool)` 登记它。
- 用例：`getEditLock`、`releaseEditLock`；`openEditSession` 的请求体 `{take_over}`。
- problem 的 `lock`、`ended_by`：`shared.Error`、平台、契约。
- 契约、HTTP、规则表、文案（三个新码的中英文案；名称的插值在 P4）。权限矩阵两行。
- README 的编辑会话一节。
- 测试：领域与仓储；用例；守卫按操作的表格；交错 45–51 与改写 42；`checkPages` 的新不变量；最后一跳；e2e 的 C1–C4、C6（过期）的接口版本；改写 PG4、PG8 与第 4 节列的 M4 测试。

**不做**：事件流与订阅者的注册（P2）；前端（P3–P5；P1 之后网页编辑器遇到 `page.locked` 只显示通用错误，不丢数据，总设计第 3 节）；任务项（P6）。

## 3. 设计

### 3.1 文件

```
server/
  migrations/sql/00018_page_edit_session_ends.sql
  migrations/schema_test.go                       新的检查与列
  sqlc.yaml                                       page 一条加上 00018
  internal/shared/error.go、error_test.go          Error 的 Lock、EndedBy 与它们的方法
  internal/platform/httpserver/problem.go、apierrors.go、contract_test.go
                                                  Problem 的 lock、ended_by；可选接口
  internal/modules/access/domain/rules.go         page.release_edit_lock
  internal/modules/page/
    domain/session.go、errors.go、actions.go       租约 120 秒；EndReason；三个码与 Locked、Unlocked 的构造；新动作
    app/ports.go                                  EditSession 的三个新字段与 Alive；Sessions、SessionWriter 的新方法；Names 端口
    app/extension.go                              SessionOpening.TakeOver；EditSessionSubscriber.EditSessionOpened
    app/unit_session.go                           OpenSession 的过期清理、接管、告诉开启；Unlock；tellEnded 的变体
    app/unit_content.go                           writersSession 认出墓碑
    app/heartbeat_edit_session.go、end_edit_session.go、open_edit_session.go
    app/get_edit_lock.go、release_edit_lock.go     两个新用例
    app/edit_lock.go                              EditLock：否决者与守卫
    adapter/postgres/queries/sessions.sql         见 3.3
    adapter/http/                                 两个新操作、openEditSession 的请求体
    edit_lock.go                                  模块根的 NewEditLock
    module.go、deps                               Deps.Names
  internal/bootstrap/registrants.go、deps.go、page_names.go
                                                  pageRegistrants(pool)；identity 的目录转成 page.Names
api/modules/page.yaml、api/common.yaml            见 3.6
web/apps/web/src/i18n/messages/*.ts、app/problem-messages.test.ts
                                                  三个新码的文案；测试换一个没有文案的示例码
README.md                                         编辑会话一节
e2e/stories/page/、e2e/stories/collab/             见 3.9
```

### 3.2 数据

迁移 00018（`ALTER TABLE edit_sessions`）：

- `ended_reason text`，`CHECK (ended_reason IN ('taken_over', 'unlocked'))`；
- `ended_by_id uuid REFERENCES users`；
- `ended_at timestamptz`；
- `edit_sessions_ended_check`：三者同为空，或同不为空，且 `ended_at >= created_at`。

`expires_at > created_at` 不变：墓碑的 `expires_at` 只会变大（总设计 4.3）。不加索引：墓碑与活着的会话一样少。

### 3.3 查询

| 查询 | 改动 |
|---|---|
| `AliveSessionsOf(node_ids, now)` | 新：这些页活着的会话（`ended_reason IS NULL AND expires_at > now`），不加锁。否决者、守卫、读锁用 |
| `DeleteExpiredSessionsOf(node_id, now)` | 新：开启时，在正文行的锁下删掉这一页 `expires_at <= now` 的行（含过期的墓碑） |
| `EndAliveSessions(node_id, user_id?, reason, by, at, until)` | 新：把这一页活着的会话（接管时只取本人的）标为墓碑，`expires_at = greatest(expires_at, until)`，`until` 是结束时刻加一个租约；返回更新的行 |
| `FindEndedSession(id, user_id)` | 新：本人的墓碑（心跳、结束在找不到活着的会话之后查它） |
| `FindLiveSession`、`HeartbeatSession` | 加 `ended_reason IS NULL` |
| `EndSession` | 改：本人活着的会话，或本人的墓碑，都删掉；返回它（墓碑不活着，不告诉订阅者） |
| `LockSession` | 返回新的三列 |
| 其余（`DeleteNodeSessions`、`DeleteNotebookSessions`、`DeleteExpiredSessions`） | 返回新的三列；行为不变 |

### 3.4 领域

- `EditSessionLease = 120 * time.Second`，注释写明为什么（总设计 4.6）。
- `EndReason` 加 `EndedTakenOver = "taken_over"`、`EndedUnlocked = "unlocked"`。
- 码：
  - `page.locked`（409）：`domain.Locked(pageID, userID, name)` 返回带 `Lock` 成员的 `*shared.Error`；
  - `page.edit_session_taken_over`（409）；
  - `page.edit_session_unlocked`（409）：`domain.Unlocked(userID, name)` 带 `EndedBy` 成员。
- 动作 `page.release_edit_lock`：笔记本的管理员（规则表一行）。
- `EditSession` 加 `EndedReason`、`EndedByID`、`EndedAt`；`Alive(now)` 改为 `EndedReason == "" && ExpiresAt.After(now)`（总设计 4.1）。所有判断活着的地方都经它，或经带 `ended_reason IS NULL` 的查询。

### 3.5 写入单元与用例

**开启**（`Unit.OpenSession(ctx, id, takeOver)`），在正文行的锁下：

1. `DeleteExpiredSessionsOf`：删掉这一页过期的行。
2. 接管：`EndAliveSessions(id, 本人, taken_over, …)`，告诉结束的订阅者（这些行在更新之前都活着）。
3. 否决者（`SessionOpening` 带 `TakeOver`）。锁的否决者查这一页活着的会话：有就 `Locked(持锁人)`。接管时本人的已在第 2 步结束，剩下的只能是别人的。
4. 建会话，告诉订阅者"开启"（`EditSessionOpened`，带会话 id、页、笔记本、本人、时刻）。

**强制解锁**（`ReleaseEditLock`）：`Writer.Run`，动作 `page.release_edit_lock`，不改树；单元里 `Unit.Unlock(ctx, id)`：

1. 锁正文行（不是页面：404）；
2. `EndAliveSessions(id, 任何人, unlocked, 执行者, …)`；
3. 告诉订阅者。

不写变更集，不告诉观察者。没有活着的会话也答 204。

**读锁**（`GetEditLock`）：找到页（404），判定 `page.read`（不加锁的同一判定，与读正文相同），`AliveSessionsOf([id], now)`，取持锁人的名称；答复 `{holder | null, expires_in}`，`expires_in` 是向上取整的秒数。

**带会话的写**（`writersSession`）：锁住会话行之后：

1. 不是本人、同一客户端、这一页的：`page.edit_session_ended`（同一个码问不出会话属于谁）；
2. 墓碑：`taken_over` 答 `page.edit_session_taken_over`，`unlocked` 答 `Unlocked(解除者)`；
3. 不活着：`page.edit_session_ended`。

**心跳**：`FindLiveSession` 找不到时查 `FindEndedSession`。本人的墓碑答 409（`taken_over` 或 `unlocked`），否则照旧 404。`HeartbeatSession` 更新不到行时同样处理：它在行锁上等到接管提交之后，按新版本重判 `ended_reason IS NULL`，不会把墓碑续成活的。

**结束**：`EndSession` 删掉本人活着的会话或墓碑。活着的告诉订阅者，墓碑不告诉，都答 204；其余照旧 404。

**锁**（`app.EditLock`，模块根 `page.NewEditLock(pool, names)` 建它，`names` 取显示名）：

- `VetoEditSession`：这一页有活着的会话就 `Locked`。
- `GuardWrite`：

  | 操作 | 规则 |
  |---|---|
  | `OpContent` | 这一页有活着的会话，且不是 `Step.EditSessionID`：`Locked` |
  | `OpDelete` | `Changes` 里的页（`Subtree` 按层排）有别的账户活着的会话：`Locked`，取第一个 |
  | 其余 | 放行 |

- 时刻取 `Step.At` 与 `SessionOpening.At`。读不加锁（总设计 4.4 的论证）。

**扩展点的改动**（M4 建立、M5 第一个注册，改形状不影响别人）：

- `SessionOpening` 加 `TakeOver bool`；
- `EditSessionSubscriber` 加 `EditSessionOpened(ctx, SessionOpened)`；
- `tellEnded` 接受"这些行在更新之前都活着"的调用（接管、解锁），其余照旧按 `Alive` 过滤。

**组合根**：

- `pageRegistrants(pool)` 返回 `sessionVetoers: [lock]`、`guards: [lock]`；`notebookRegistrants(pool)` 与 `pageDeps` 都从它取；
- `page.Deps.Names` 由 `pageNames{identity.Directory}` 给。

### 3.6 契约

- `api/common.yaml` 的 `Problem` 加可选成员：
  - `lock`：`{page_id, user_id, display_name}`，全部必填；
  - `ended_by`：`{user_id, display_name}`。
  - 描述写明只有哪个码带它。
- `api/modules/page.yaml`：
  - `openEditSession`：可选的请求体 `EditSessionOpening {take_over: boolean}`，默认 `false`；码加 `page.locked`。描述写明租约 120 秒、接管只越过本人、锁按会话；
  - `heartbeatEditSession`：码加 `page.edit_session_taken_over`、`page.edit_session_unlocked`；
  - `endEditSession`：墓碑也是 204；
  - `putPageContent`：码加 `page.locked` 与两个墓碑的码；
  - `deleteNode`：码加 `page.locked`，描述写明"别的账户"；
  - 新操作 `getEditLock`（`GET /api/v0/pages/{page_id}/edit-lock` → `EditLock {holder: EditLockHolder | null, expires_in: integer | null}`）、`releaseEditLock`（`DELETE` 同一地址 → 204）。
- `make gen`；HTTP 的处理器；平台的契约测试加 `lock` 与 `ended_by` 两行。

### 3.7 权限与矩阵

- 规则表：`page.release_edit_lock`（笔记本的管理员）。
- 矩阵：`getEditLock` 用读的列，`releaseEditLock` 用笔记本级的列。
- 种子改为每页至多一个活着的会话；`leased()` 改为租约 120 秒。

### 3.8 交错

在 `bootstrap/interleavings_content_test.go` 之后接着编号，每个两种先后（只有一种的写明）：

| # | 交错 | 期望 |
|---|---|---|
| 42 改 | 两个开启 | 后一个答 `page.locked`，持锁人是先一个 |
| 45 | 接管与旧会话的保存 | 保存先：写进旧会话的变更集，接管随后；接管先：保存答 `taken_over` |
| 46 | 强制解锁与保存 | 同上，答 `unlocked`，带解除者 |
| 47 | 心跳与接管 | 心跳先：续上旧会话，接管照常结束它；接管先：心跳答 `taken_over`，墓碑没被续活 |
| 48 | 心跳与强制解锁 | 同上，答 `unlocked` |
| 49 | 晚到的心跳与开启（只有一种：心跳读的时刻早于会话到期，等在开启删掉的行上） | 心跳答 404，开启成功，只有一个活着的会话 |
| 50 | 别人的删除与开启 | 删除先：开启答 `page.not_found`；开启先：删除答 `page.locked` |
| 51 | 不带会话的写与开启 | 写先：正文改了，开启照常；开启先：写答 `page.locked` |

`checkPages` 加"每页至多一个活着的会话"（按测试的时钟）与"墓碑的三列同在"。

### 3.9 端到端（接口版本）

新分组 `e2e/stories/collab/`，夹具 `fixtures/collab.ts`（读锁、强制解锁、带 `take_over` 的开启），断言 `fixtures/assert/collab.ts`（活着的会话、墓碑）：

- **C1**：A 开启，B 开启答 409 `page.locked`（`lock` 是 A 与这一页）；A 结束，B 开启成功；读锁先是 A、后是 B。落库：这一页至多一个活着的会话。
- **C2**：同一账户再开启答 409 `page.locked`（持锁人是自己）；带 `take_over` 成功；旧会话的心跳与保存答 409 `page.edit_session_taken_over`；旧会话结束答 204。落库：旧会话是墓碑。
- **C3**：编辑者、阅读者强制解锁答 403；管理员答 204；A 的保存答 409 `page.edit_session_unlocked`（`ended_by` 是管理员）；读锁为空。
- **C4**：A 持锁时，PAT 写正文答 409 `page.locked`；B 删这一页与它的上级页答 409；改名、移动、与当前正文相同的写照常；A 自己删除成功，会话随之删除；删笔记本成功。
- **C6（过期）**：把 A 的会话推到过期（库里改时刻），B 开启成功，A 的保存答 `page.edit_session_ended`。

改写 M4 的：

- **PG4**：子树里有别人活着的会话时删除被拒；本人的会话与过期的随删除删掉。
- **PG8**：两个版本都改为"会话过期、PAT 写入、带会话的保存答 `ended`、重开、保存答 `revision_mismatch`"；页面版本的冲突区照旧。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 数据、领域与 problem 的成员 | [P1-S1](plans/P1-S1-data-domain-problem.md) |
| S2 | 单元、用例与锁 | [P1-S2](plans/P1-S2-unit-lock.md) |
| S3 | 契约、HTTP、矩阵与文案 | [P1-S3](plans/P1-S3-contract-matrix.md) |
| S4 | 交错、改写的 M4 测试与最后一跳 | [P1-S4](plans/P1-S4-interleavings-whole.md) |
| S5 | 端到端 | [P1-S5](plans/P1-S5-e2e.md) |

## 5. 测试与验证

- **领域**：`Alive` 的表格（活着、过期、墓碑、恰好到期）；`Locked`、`Unlocked` 带成员。
- **仓储**（测试数据库）：每条新查询与改过的查询，尤其墓碑不被心跳续活、`EndAliveSessions` 只动活着的、`DeleteExpiredSessionsOf` 只动这一页过期的。
- **用例**（假端口与假时钟）：
  - 开启：过期的先删、接管只动本人的、别人持锁时接管照样被拒，"开启"在建会话之后告诉；
  - 解锁：404、403、204，告诉订阅者，带执行者；
  - 读锁：`expires_in` 向上取整，阅读者能读；
  - `writersSession` 的次序：别人的墓碑答 `ended`；
  - 心跳与结束对墓碑的答复；
  - 守卫按操作的表格：写正文带与不带会话，删除时本人与别人、子树里第一个，改名、移动放行。
- **整个程序**：
  - 否决者与守卫的最后一跳：开启（带与不带接管），新建、改名、移动、删除、写正文（带与不带会话）各经整个程序，`pageRegistrants(pool)` 交空时失败；
  - problem 的 `lock`、`ended_by` 经 HTTP 答出，符合契约。
- **交错**：3.8。
- **e2e**：3.9。
- **反向对照**：
  - `Alive` 不看 `ended_reason`；
  - 墓碑把 `expires_at` 改为结束时刻；
  - 开启不删过期的行；
  - 接管不限本人；
  - 守卫在写正文时不比较会话；
  - 删除按会话而不按账户；
  - 解锁不先锁正文行；
  - `writersSession` 先认墓碑再核对本人；
  - 心跳的更新不带 `ended_reason IS NULL`；
  - `pageRegistrants(pool)` 交空。

## 6. 完成标准

- `GOFLAGS=-p=3 make check`、`make gen-check`、`make e2e` 为绿；上面的反向对照都按预期失败。
- Opus 审查与修复；修复的差异较大时另经核对。
- 持续集成为绿；`--no-ff` 合并；`make image-smoke`；文档提交（本文的结果一节、总设计第 12 节）。

## 7. 结果

（合并之后填写。）
