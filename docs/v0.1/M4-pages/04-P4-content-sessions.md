# M4/P4 正文与编辑会话：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P4 正文与编辑会话 |
| 状态 | 已完成（`59d7702` 合并） |
| 基线 | `baef37a`（P3 合并、文档提交之后的 main）；本文与各 Step 计划提交之后开分支 |
| 上级文档 | [M4 总设计](00-M4-design.md) 第 3、4、5、7、8、9 节；[P1](01-P1-page-module-pipeline.md) 3.6、3.9；[P2](02-P2-tree-operations.md) 3.3、3.4；[P3](03-P3-markdown.md) 3.2、3.9；[M3/P1 移交](handoffs/M3-P1-notebook-deletion.md)第 1 项；[M3 移交](handoffs/M3-notebooks.md)第 5 项；[总体设计](../v0.1-design.md) 3.8、3.9、6.1、8.5、13.1 |

---

## 1. 基线

P1–P3 留下的：

- 写入单元（`app.Writer.Run`）：工作区行 `FOR SHARE` → 笔记本行（树的写 `FOR NO KEY UPDATE`，其余 `FOR SHARE`）→ 判定 → 操作；每个操作经守卫、写入、条目与版本、参与者，单元结束时一次事件。操作有新建（空正文、版本 1）、改名、移动、删除子树。`Unit.content` 算好哈希与字节数，`recordRevision` 记版本，`RecordRevision` 的 SQL 在同一变更集里更新这一页的版本行、保留它的 `base_revision`（为会话预备）。`changesets.updated_at` 的注释已写明"编辑会话之后的写把它往后推（M4/P4）"。
- 读：`getPage`（`page_contents` 的元数据）、`getPageView`（`PageContent` 一条语句读出正文与版本，`Parse` 加 `Render`）。
- 扩展点：守卫、参与者（`Appender` 只有 `Rename`，注释写明 P4 加正文写）、观察者；组合根交空集合。笔记本删除的注册者 `page.NewNotebookDeletion(pool)` 在三条发布路径上都有整个程序的测试（`page_registrants_test.go`）。
- `platform/markdown`：`Parse` 不出错、耗时线性；`app.Markdown` 端口与适配器已接好，`app.Parsed` 是不透明的值。
- 平台：`bodyshape` 已在边界答 400：非法的 UTF-8、`\u` 转义的孤立代理项（`pairedSurrogates`），不会被换成 U+FFFD。请求体上限只有全局的 `server.max_body_bytes`（1 MiB）；按路由的只有 `RequestTimeouts`（identity 的续期与退出）。
- notebook 的活动扩展点 `NotebookActivitySource` 没有注册者，`listOwnerlessNotebooks` 的大小恒为 0、最后活动是笔记本行的 `updated_at`。
- 定时任务：identity 的 `CleanupSessions`（`SKIP LOCKED`、一批 1000、`auth.session_cleanup_interval`）与 River 的 `CleanupJob` 是先例。

## 2. 目标与范围

**目标**：页面有了可读写的正文与编辑会话。正文的写是写入单元里的一个新操作，在笔记本行 `FOR SHARE` 与这一页正文行的 `FOR NO KEY UPDATE` 之下比较 `base_revision`；编辑会话是租约，一次会话的多次保存是一个变更集、一个版本，夹进别人的写时另起一个；删页、删子树、删笔记本时会话随之删除，订阅者得到原因。

**做**：

- 迁移 `edit_sessions`；领域：正文的取值检查与上限、租约与心跳的常量、三个新码、两个新操作的规则、`OpContent`、结束的原因。
- 写入单元的操作 `WriteContent`（会话的变更集、"夹进别人的写另起一个"、与当前正文相同时不写）与 `OpenSession`；`Appender.WriteContent`（M6 的链接改写追加正文写）；改动带上解析结果，守卫的值带上编辑会话。
- 用例：`getPageContent`、`putPageContent`、`createPage` 的 `content`、`openEditSession`、`heartbeatEditSession`、`endEditSession`；过期会话的定时清理与配置项 `page.edit_session_cleanup_interval`。
- 删页、删子树、笔记本删除时删除会话，并以"页面被删除"调用结束的订阅者；编辑会话的扩展点（开启的否决者、结束的订阅者），组合根交空集合。
- 注册笔记本的活动（M3 移交第 5 项）。
- 平台的按路由请求体上限 `APIConfig.BodyLimits` 与模块根的 `BodyLimits()`（正文写入、带正文的新建）。
- 契约、HTTP、规则表、三个新码的中英文案；权限矩阵五行。
- 测试：领域与仓储；用例（码的次序、会话的变更集、租约的时钟、日志）；字节保真的往返；扩展点的测试替身；整个程序（活动经 `listOwnerlessNotebooks`、三条删除路径删会话、请求体上限）；交错 38–43；e2e 的 PG5–PG10 接口版本、PG12 的正文与会话、PG14。

**不做**：会话的排他与锁、强制解锁、`sendBeacon` 释放（M5）；自动保存（M5）；前端的编辑器与会话的前端常量（P6）；阅读视图与页面树的前端（P5）。

## 3. 设计

### 3.1 文件

```
server/
  migrations/sql/00017_page_edit_sessions.sql
  migrations/schema_test.go                      edit_sessions 的约束与索引名
  sqlc.yaml                                      page 一条加上 00017
  internal/platform/httpserver/api.go、api_test.go
                                                 APIConfig.BodyLimits：按路由放宽请求体上限；BodyReadTimeout：
                                                 这些路由的期限另加请求体的读取时间（3.8）
  internal/platform/config/                      PageConfig：page.edit_session_cleanup_interval、parse_budget_bytes、
                                                 parse_max_wait；read_timeout + request_timeout < write_timeout
  configs/config.yaml 与各 profile               page 一节
  internal/modules/access/domain/rules.go        page.write、page.edit
  internal/modules/page/
    domain/content.go                            MaxContentBytes、CheckContent
    domain/session.go                            EditSessionLease、EditSessionHeartbeat、EndReason
    domain/errors.go、actions.go、change.go      三个码；page.write、page.edit；OpContent、Change.Parsed
    adapter/postgres/queries/contents.sql        LockContent、WriteContent；PageContent 带哈希
    adapter/postgres/queries/sessions.sql        会话的增、锁、改、心跳、结束、随页面与笔记本删除、过期清理
    adapter/postgres/queries/activity.sql        笔记本的字节数与最后写入
    adapter/postgres/queries/changesets.sql      TouchChangeset
    adapter/postgres/store.go、sessions.go、activity.go
    adapter/river/cleanup.go                     过期会话的定时任务（照 identity 的 riveradapter）
    adapter/markdown/budget.go                   解析预算：x/sync/semaphore 按字节加权（3.6）
    app/ports.go                                 Nodes.LockContent；NodeWriter.WriteContent；ChangesetWriter.TouchChangeset；
                                                 Sessions、SessionWriter、ParseBudget
    app/extension.go                             Step.EditSessionID；Appender.WriteContent；EditSessionVetoer、
                                                 EditSessionSubscriber 与它们的值；NotebookDeletion 删会话
    app/unit.go                                  WriterDeps 加会话的端口与两组注册者
    app/unit_content.go                          ContentWrite、Unit.WriteContent
    app/unit_session.go                          Unit.OpenSession；ended（删除之后告诉订阅者）
    app/unit_create.go、unit_delete.go           新建带正文；删除子树时删会话
    app/content.go                               ContentParser：事务之前预判定（Writer.Allowed）、取解析预算、Parse
    app/get_page_content.go、put_page_content.go、create_page.go
    app/open_edit_session.go、heartbeat_edit_session.go、end_edit_session.go、cleanup_edit_sessions.go
    app/activity.go                              NotebookActivity、Activities：模块根交给组合根的形状，不是用例的端口
    adapter/http/handler.go、limits.go           五个操作；BodyLimits
    module.go、deletion.go、activity.go          模块根：Deps 的新字段、Jobs()、BodyLimits()、
                                                 NewNotebookDeletion(pool, subscribers)、NewNotebookActivity(pool)
  internal/bootstrap/
    wire.go、deps.go、registrants.go             定时任务、请求体上限、活动来源、会话的注册者（空）
    permission_matrix_page_test.go               五行
    page_registrants_test.go                     三条删除路径也删会话；活动经 listOwnerlessNotebooks
    page_content_test.go                         字节保真的往返；请求体上限；会话的保存夹着别人的写；删子树删各页的会话
    interleavings_content_test.go                交错 38–44（interleavings_page_test.go 的 checkPages 加会话）
api/modules/page.yaml、api/openapi.yaml           五个操作；PageCreate.content；PageContent、PageContentWrite、EditSession
web/apps/web/src/app/problem-messages.ts、en.ts、zh-CN.ts   三个码
e2e/fixtures/pages.ts、assert/page.ts；e2e/stories/page/pg5–pg10、pg14；pg12 加正文与会话
```

测试另在 `domain/content_test.go`、`session_test.go`，`app/content_test.go`、`session_test.go`（与 `fakes_test.go` 的改动），`adapter/postgres/store_test.go`、`sessions_test.go`，`adapter/river/cleanup_test.go`，`adapter/http/handler_test.go`，模块根的 `extension_test.go`。

### 3.2 数据

**`edit_sessions`**（00017）：

| 列 | 类型 | 说明 |
|---|---|---|
| `id` | `uuid` 主键 | v7 |
| `node_id` | `uuid NOT NULL` | 会话的页面；不建外键（M4 总设计第 4 节"页面的表"） |
| `notebook_id` | `uuid NOT NULL` | 页面所在的笔记本：笔记本删除按它删会话；不建外键 |
| `user_id` | `uuid NOT NULL REFERENCES users` | 开启的人；账户不被物理删除，外键只取 `FOR KEY SHARE`（13.1 第 5 条"外键检查"） |
| `client` | `text NOT NULL` | 开启时的客户端，与 `changesets.client` 同一个 CHECK |
| `changeset_id` | `uuid` | 会话的变更集，第一次写入时设上；不建外键 |
| `revision` | `int` | 本会话上次写出的 `revision`，与 `changeset_id` 同时设上（CHECK 两者同为空或同不为空） |
| `created_at` | `timestamptz NOT NULL` | 开启的时刻 |
| `expires_at` | `timestamptz NOT NULL` | 租约的到期；`expires_at > now` 为活着 |

- 索引：`(node_id)`（删页、删子树）、`(notebook_id)`（删笔记本）。`expires_at` 不建索引：心跳每 20 秒改它，有索引就不是 HOT 更新；清理扫的是一张只有活着的会话的小表（P4 审查 P5）。
- CHECK：`changeset_id` 与 `revision` 同为空或同不为空、`revision >= 1`；`expires_at > created_at`。
- 没有 `deleted_at`：结束即删除行（总体设计 6.1 的例外），不进清理注册表；`TestEverySoftDeletedTableHasAPurger` 与 `TestPurgersComeBeforeTheTablesTheyReference` 因此不涉及它。
- `checkPages` 加一条不变量：没有指向已删节点（或不存在的节点）的会话。

**正文的语句**（`contents.sql`）：

- `LockContent :one`：本笔记本一个未删的页面的正文行 `FOR NO KEY UPDATE OF c`（一页的闸门），读出 `revision`、`content_hash`、`byte_size`；节点与正文都要未删、节点在这个笔记本、是页面，读不到答 `ErrNotFound`。不锁节点行（"正文的写不碰节点行"）。
- `WriteContent :exec`：改 `content`、`revision`、`content_hash`、`byte_size`、`updated_by_id`、`updated_at`。
- `PageContent` 加上 `content_hash`。

**变更集**：`TouchChangeset :exec` 把会话的变更集的 `updated_at` 推到单元的时刻（活动的"最后写入"按它算）。

**活动**（`activity.sql`）：一组笔记本里未删页面的 `byte_size` 之和；这些笔记本未删的变更集最晚的 `updated_at`。一条语句：从变更集按 `notebook_id` 分组取最晚的 `updated_at`，字节数是相关子查询（没有变更集的笔记本不在答复里），走 `changesets_notebook_id_idx` 与 `nodes` 的 `(notebook_id, parent_id)` 索引。最后写入要读完笔记本的全部变更集：只有工作区管理员的无主清单调用，数据量大时再加 `(notebook_id, updated_at) WHERE deleted_at IS NULL` 的部分索引（P4 审查 P4）。

### 3.3 领域

- **正文**（`content.go`）：`MaxContentBytes = 5 << 20`；`CheckContent(field, content)`：超过上限是 `too_long`，含 NUL 或不是合法的 UTF-8 是 `invalid_format`（HTTP 的非法 UTF-8 在边界已答 400，这里守住别的调用方）。
- **会话**（`session.go`）：`EditSessionLease = 60 * time.Second`、`EditSessionHeartbeat = 20 * time.Second`；注释指向 P6 前端的同名常量（P6 加上之后互指，各有测试钉住，13.1 第 15 条），测试钉住两个数与"租约至少是三次心跳"。`EndReason`：`EndedByOwner`（`ended`）、`EndedWithPage`（`page_deleted`）；M5 加强制解锁。
- **码**：`ErrRevisionMismatch`（409 `page.revision_mismatch`）、`ErrEditSessionEnded`（409 `page.edit_session_ended`）、`ErrEditSessionNotFound`（404 `page.edit_session_not_found`）。
- **操作与规则**：`page.write`（正文的写）、`page.edit`（开启会话、心跳），都给 `writers()`；读正文用 `page.read`；结束不经判定，只要求会话是本人的。
- **改动**：`OpContent`；正文的写是这一页一条改动，前后的树状态相同（`Moves()` 为假：不记条目），`Revision` 是新版本。`Change` 加 `Parsed any`：这次写出的正文的解析结果（页面的 `app` 里不透明，M4 总设计第 4 节"解析时机"），新建与正文的写都带上；`Then` 合并时随 `Revision` 取后一个的。

### 3.4 写入单元的两个操作

**`Unit.WriteContent(ctx, ContentWrite{NodeID, Content, Parsed, Base, EditSession uuid.UUID})`**（会话为零值时不带），返回写出的 `revision`（不写时为当前的）。哈希与字节数由单元在正文行的锁下算（`Unit.content`，5 MiB 的 SHA-256 几毫秒；参与者追加的正文写本来就在事务里，P4 审查 N3）：

1. `LockContent`（读不到答 `page.not_found`）；`FindNodeIn` 读出节点的树状态（笔记本行的 `FOR SHARE` 挡住了一切树的写，不必锁节点行）。
2. 带会话时以 `FOR UPDATE` 锁会话行：没有、不是本人的、不是单元的客户端开启的、不是这一页的、`expires_at` 不晚于单元的时刻，答 409 `page.edit_session_ended`。客户端也要相同：网页开的会话不能由同一账户经令牌的写点名，否则这些写记在会话的变更集、也就是第一次写的客户端名下（13.1 第 2 条，P4 审查 D1）。
3. 与当前正文相同（哈希与字节数相同）：不写、不调用守卫，返回当前的 `revision`，不论 `base_revision`：写的意图已经是现状（回应丢失之后的重试因此成功）。
4. `Base` 不等于当前的 `revision`：409 `page.revision_mismatch`。
5. 变更集：带会话、会话已有变更集、并且当前的 `revision` 等于会话上次写出的：用会话的变更集（单元不另建），`TouchChangeset`；否则单元自己的变更集（第一次写时新建）。所以一次会话的连续保存是一个变更集、这一页一个版本行（`RecordRevision` 更新它、保留最初的 `base_revision`）；中间夹进了别人的写，下一次保存另起一个，每个版本行的"前"与"后"之间没有别人的改动。
6. 守卫（`Step{OpContent, 改动, EditSessionID}`）→ `WriteContent`（`revision + 1`）→ 版本 → 带会话时把会话的 `changeset_id`、`revision` 设为这一次的（`SetSessionWrite` 核对改到一行：行在锁下，改不到是故障）→ 参与者。会话行的 `FOR UPDATE` 让本人的结束等这次保存提交：结束之后不再有这个会话的写（交错 44）。

同一单元里写多页正文的（M6 的参与者在 `FOR SHARE` 下追加时）按节点 id 升序锁正文行：M4 的用例每个单元只写一页，规则写在 `WriteContent` 的注释里，留给 M6。

**`Appender.WriteContent`**：参与者追加的正文写走同一个操作，不带会话，不再调用参与者；解析结果由参与者给出（M4 总设计第 4 节）。

**`Unit.OpenSession(ctx, nodeID)`**：`LockContent`（一页的闸门；读不到答 `page.not_found`）→ 否决者（在正文行的锁下；M5 判断别人有没有活着的会话）→ 插入会话：`expires_at = 单元的时刻 + EditSessionLease`，客户端是单元的。不建变更集、不调用观察者（单元没有改动）。

**新建带正文**：`PageDraft` 加 `Content`、`Parsed`；新建的正文是给出的（缺省为空），版本 1，改动带解析结果。

**删除子树**：`DeleteNodes` 之后删掉子树各页的会话（`DeleteNodeSessions :many`，按 `node_id`），对其中在单元的时刻仍活着的，以 `EndedWithPage` 调用结束的订阅者（过期不是事件，M4 总设计第 4 节"编辑会话"）。加锁次序 `nodes → page_contents → edit_sessions` 不变：删除的单元持笔记本行的独占锁，带会话的写在笔记本行上等它。

### 3.5 编辑会话的用例

| 用例 | 判定与加锁 | 码的次序 |
|---|---|---|
| `openEditSession` | 单元（笔记本行 `FOR SHARE`），`page.edit`，正文行 `FOR NO KEY UPDATE` | 404 `page.not_found` → 403 → 否决者 |
| `heartbeatEditSession` | 不加锁读出本人活着的会话 → 它的笔记本与工作区 → 判定 `page.edit`（按读，不取工作区行与笔记本行）→ 一条 `UPDATE … WHERE id AND user_id AND expires_at > now RETURNING` | 404 `page.edit_session_not_found`（没有、过期、别人的、看不到笔记本）→ 403（降为阅读者）→ 语句没改到行：404 |
| `endEditSession` | 一个事务：`DELETE … WHERE id AND user_id AND expires_at > now RETURNING`，不加锁读出笔记本的工作区（会话在，笔记本就在；读不到是故障），再以 `EndedByOwner` 调用订阅者；不判定、不取工作区行与笔记本行 | 404 `page.edit_session_not_found`（没有、过期、别人的） |

- 时刻都取自 `Clock`：开启的到期是单元的时刻加租约，心跳把 `expires_at` 推到"现在加租约"，判断活着用同一个"现在"。租约的时钟测试用假时钟钉住边界：恰好到期时已过期，到期前一刻的心跳成功并从那一刻起算。
- 失去编辑权限之后心跳答 403 或 404，会话在一个租约之内过期（M4 总设计第 4 节"加锁"）；结束过期的会话答 404，行留给定时清理。
- 日志：`edit session opened`、`edit session ended`（`edit_session_id`、`node_id`、`notebook_id`、`workspace_id`、`user_id`、客户端）；心跳不记。

**定时清理**（`CleanupEditSessions`，照 identity 的 `CleanupSessions`）：一批 1000，`DELETE … WHERE id IN (SELECT … WHERE expires_at <= now LIMIT batch FOR UPDATE SKIP LOCKED)`（`<=`：与"`expires_at > now` 为活着"互补），直到某批不满；不调用订阅者；删了才记日志（只记个数）。任务 `page.cleanup_expired_edit_sessions`，启动时与每 `page.edit_session_cleanup_interval`（默认 10 分钟，至少 1 秒）运行。

### 3.6 正文的用例

| 用例 | 判定与加锁 | 码的次序 |
|---|---|---|
| `getPageContent` | 照 `getPage`：不加锁，`page.read` | 404 `page.not_found` |
| `putPageContent` | 事务之前：取值检查、不加锁读出节点、预判定（`Writer.Allowed`：单元的 404 与 403，不加锁）、取解析预算、`Parse`；单元（笔记本行 `FOR SHARE`），`page.write`，在锁下再判定 | 422 `content` → 404 `page.not_found` → 403 → 503 `server_busy` → 409 `page.edit_session_ended` → 409 `page.revision_mismatch` → 守卫 |
| `createPage`（带 `content`） | 事务之前：取值检查、预判定、取解析预算、`Parse`；其余同 P1 | 422 `content` → 404 → 403 → 503 → 422（标题、父页、位置）→ 409 → 守卫 |

- 正文的 422 先于 404、403（M4 总设计第 5 节"码的次序"）。
- **解析在判定之后**：只有能写的人让服务端解析正文。5 MiB 的病态正文解析约 1.4 秒、堆峰值多出约 1.5 GB（P4 审查 P1 的实测），判定在前，看不到页面的账户只得到 404。预判定与单元的判定用同一个 `workspaceOf` 与授权，两者之间的变化由单元在锁下再判定一次。空正文不预判定、不取预算。
- **解析预算**（`app.ParseBudget`，适配器 `adapter/markdown.Budget`）：同时解析、渲染的正文字节数不超过 `page.parse_budget_bytes`（默认 8 MiB，至少一页正文的上限 5 MiB），按正文的字节数取额度，单元结束之后放回；取不到最多等 `page.parse_max_wait`（默认 2 秒），然后 503 `server_busy`（`Retry-After` 1 秒），记 `content parsing is saturated`；请求自己被取消或到期答它自己的错误。阅读视图（`getPageView`）的解析与渲染同样取额度。照 `auth.password.max_concurrent_hashes` 的先例（P4 审查 P2）。
- `putPageContent` 答 200 `Page`（单元结束之后重读）；没写时同样答当前的页面。
- 日志：`page content written`（ids、`changeset_id`、`revision`、客户端，带会话时 `edit_session_id`）；没写时不记；正文与标题都不进日志。

### 3.7 扩展点

- **`Step.EditSessionID`**：带会话的正文写有，其余为零值（M5 的锁据此判断写的人持有这一页的会话）。
- **`EditSessionVetoer.VetoEditSession(ctx, SessionOpening)`**：`SessionOpening` 是单元的 `Write` 加页面 id；错误是 `*shared.Error`，单元回滚并答出它（M5 的 `page.locked`、M11 的冻结）。
- **`EditSessionSubscriber.EditSessionEnded(ctx, SessionEnded)`**：`SessionEnded{SessionID, WorkspaceID, NotebookID, PageID, UserID, Reason, By, At}`；在结束的事务里调用，错误整体回滚。三条路径：本人结束、删页与删子树（单元里）、笔记本删除（注册者里，`By` 与 `At` 是删除事件的）。
- **`NotebookDeletion`**：`DeleteNotebooksPages` 之后按 `notebook_id` 删这些笔记本的会话，活着的以 `EndedWithPage` 告诉订阅者。`page.NewNotebookDeletion(pool, subscribers)`：组合根的 `notebookRegistrants(pool)` 从 `pageRegistrants()` 取订阅者（M4 为空）。
- 组合根交空集合；模块根的 `extension_test.go` 用测试替身证明：否决者在正文行的锁下被调用、它的错误答出且没有会话；三条路径各调用一次订阅者、原因与执行者对；观察者收到正文写的解析结果。参与者追加的正文写经守卫、加版本、记进同一变更集、并进改动集，只在 `app/content_test.go` 以假端口证明（`TestAParticipantWritesAContent`）；模块根的参与者替身追加的是改名（`TestAParticipantAddsToTheUnit`）。
- 最后一跳的测试（编辑会话的否决者与订阅者，M5）写进给 M5 的[编辑会话移交](../M5-collab-editing/handoffs/M4-P4-edit-sessions.md)第 8 项（M4 总设计第 8 节）。

### 3.8 请求体上限

- 平台：`APIConfig.BodyLimits map[string]int64`，与 `RequestTimeouts` 并列：按路由放宽上限。`NewAPI` 核对每个值为正；中间件取路由的值与 `MaxBodyBytes` 中大的那个：运维调大全局值不会让这两个路由反而更紧。
- 模块根的 `BodyLimits()`：`PUT /api/v0/pages/{page_id}/content` 与 `POST /api/v0/notebooks/{notebook_id}/pages` 各为 `6 × MaxContentBytes + 64 KiB`：一个字节在 JSON 字符串里最长写成 6 个字节（控制字符的 `\u00XX`），64 KiB 留给其余字段。
- 期限：请求的期限在读请求体之前就开始计时，这两个路由的期限因此是 `server.request_timeout` 加 `APIConfig.BodyReadTimeout`（组合根给 `server.read_timeout`），配置要求 `read_timeout + request_timeout < write_timeout`（默认 30 + 15 < 60 秒）。请求体本身受 `read_timeout` 约束：5 MiB 的正文在 30 秒内传完约需 1.4 Mbit/s 的上行，转义成 30 MiB 的最坏情况约 8.4 Mbit/s；更慢的链路调大 `read_timeout` 与 `write_timeout`（`config.yaml` 写明；P4 审查 P3）。
- 整个程序的测试：每条转义成 6 字节的 5 MiB 正文在两个路由上都收（200、201）；多一个字节答 422 `content`；其余路由超过 1 MiB 仍答 413；`BodyLimits()` 的每个键都是路由器上的路由（`Router.Patterns()`）。

### 3.9 契约

| 操作 | 方法与路径 | 成功 | 码 |
|---|---|---|---|
| `getPageContent` | `GET /pages/{page_id}/content` | 200 `PageContent`（`content`、`revision`、`content_hash`：正文字节的 SHA-256，小写十六进制） | `page.not_found` |
| `putPageContent` | `PUT /pages/{page_id}/content` | 200 `Page` | `validation_failed`、`page.not_found`、`forbidden`、`page.edit_session_ended`、`page.revision_mismatch`、`server_busy` |
| `openEditSession` | `POST /pages/{page_id}/edit-sessions` | 201 `EditSession`（`id`、`page_id`、`expires_at`） | `page.not_found`、`forbidden` |
| `heartbeatEditSession` | `POST /edit-sessions/{edit_session_id}/heartbeat` | 200 `EditSession` | `page.edit_session_not_found`、`forbidden` |
| `endEditSession` | `DELETE /edit-sessions/{edit_session_id}` | 204 | `page.edit_session_not_found` |

- `PageContentWrite`：`content`（必填）、`base_revision`（必填，整数）、`edit_session_id`（可省略）。`PageCreate` 加可省略的 `content`。
- `createPage` 与 `getPageView` 也加 `server_busy`（解析预算）。
- 描述写明：租约 60 秒、心跳每 20 秒；同一会话的保存是一个变更集，会话里的写要来自开启它的客户端（网页或令牌）；与当前正文相同的保存不写；结束只要求会话是本人的；正文的大小上限写作"5 MiB (5,242,880 bytes)"与码的次序。

### 3.10 活动

`page.NewNotebookActivity(pool)` 实现 notebook 的 `NotebookActivitySource`（组合根转换值）：字节数是未删页面 `byte_size` 之和，最后写入是未删变更集最晚的 `updated_at`，没有变更集的笔记本不在答复里。`notebookRegistrants(pool)` 交出它。整个程序的测试经 `listOwnerlessNotebooks`：无主笔记本的 `size_bytes` 是页面正文的字节数之和、`last_activity_at` 是最晚的一次写（会话的第二次保存推后了它）；组合根不交来源时失败（反向对照）。

### 3.11 交错

| # | 交错 | 持有的行 | 断言 |
|---|---|---|---|
| 38 | 两个以同一 `base_revision` 的保存 | 正文行，`FOR NO KEY UPDATE`（双方只取笔记本行 `FOR SHARE`，`WaitForLockWaitsOn(…, "page_contents", 2)`） | 先到的 200（版本 2），后到的 409 `page.revision_mismatch` |
| 39 | 保存与删除这一页 | 笔记本行（保存 `FOR SHARE`、删除 `FOR NO KEY UPDATE`） | 删除在先：保存 404 `page.not_found`；保存在先：新版本随页面以同一时刻删除 |
| 40 | 带会话的保存与删除笔记本、删除工作区 | 笔记本行（删笔记本）、工作区行（删工作区） | 删除在先：保存 404；保存在先：页面以笔记本的删除时刻删除，会话也删除 |
| 41 | 保存与把 `workspace_access` 改为 `none`（写的人只靠默认角色） | 笔记本行 | 改在先：保存 404 `page.not_found`；保存在先：200，之后读不到 |
| 42 | 同一页的两个会话的开启；开启会话与删除页面 | 正文行（第一种，等两个排上）；笔记本行（第二种） | 两个开启都 201、两行会话（M4 不排他，证明两者都经正文行的闸门）；删除在先：开启 404；开启在先：会话随页面删除 |
| 43 | 过期会话的清理与带会话的保存 | 会话行，`FOR UPDATE`（如带会话的保存持有它） | 清理不等被持有的行、也不删它，持有期间完成两次运行；放开之后的运行删掉。之后心跳 404 `page.edit_session_not_found`、带它的保存 409 `page.edit_session_ended`：这两条是被删的会话的码，不是清理的性质 |
| 44 | 本人结束会话与会话里的保存（P4 审查 T1） | 会话的变更集行（第二次保存在 `TouchChangeset` 上等，结束在会话行上等：`interleaveBehind`） | 结束等保存提交：保存 200、结束 204，会话没了，两次保存一个版本行 |

38–42 每种先后一个用例，43、44 各只有一种，结束时 `checkPages`（加上会话的不变量）与 `checkNotebooks`。39–41 持笔记本行而不是工作区行：放开工作区行时，排队的两个 `FOR SHARE` 彼此兼容，一起往下走，到笔记本行才分先后，先后不定。43 用 serve 的定时任务（间隔 1 秒），会话由 SQL 推到过期。

### 3.12 端到端（接口版本）

- PG5：带正文新建（表格、任务列表、删除线、自动链接、脚注、带语言的代码块、frontmatter）→ 阅读视图有这些元素、属性表在正文之上、frontmatter 不成为分隔线与标题，`revision` 对。
- PG6：正文带 `<script>`、事件属性、`style`、`javascript:` 与 `//外站` 的链接、原始 `<img>`、没闭合的行内标签 → 阅读视图里都没有，标签不越出所在的块。
- PG7：开启会话，带会话保存两次 → 一个变更集、一个版本行（`base_revision` 是第一次的），`revision`、`content_hash`、`byte_size` 落库。
- PG8：PAT 先写 → 带旧版本的保存 409 `page.revision_mismatch`；读出当前正文；以读到的版本"保留我的"成功。
- PG9：CRLF、只用 `\r`、混合换行、BOM、行尾空白、NFD 的正文经接口往返逐字节相同，`content_hash` 是这些字节的 SHA-256。
- PG10：心跳推后 `expires_at`；结束之后心跳 404；经数据库把会话推到过期（13.4 第 3 条，不靠墙钟）→ 带它的保存 409 `page.edit_session_ended`，开启新会话之后保存成功、正文完整。
- PG12：阅读者读正文 200，写正文、开启会话 403；看不到笔记本的人 404；`workspace_access = editor` 的成员可以写；别人的会话的心跳与结束 404。
- PG14：无主笔记本的大小是页面正文的字节数之和，最后更新是最晚的页面写入。
- fixture（`pages.ts`）：`putContent`、`getContent`、`openSession`、`heartbeat`、`endSession`；断言（`assert/page.ts`）：`expectContentWritten`、`expectOneSessionRevision`、`expectSessionGone`。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 迁移 `edit_sessions`；领域（正文检查、租约常量、码、规则、`OpContent`、`Change.Parsed`）；仓储（正文的锁与写、会话、活动、`TouchChangeset`），各自的测试 | [P4-S1](plans/P4-S1-domain-data.md) |
| S2 | 正文：`Unit.WriteContent`、`Appender.WriteContent`、新建带正文；`getPageContent`、`putPageContent`；平台的 `BodyLimits` 与模块根的；契约、HTTP、规则表、文案；矩阵两行 | [P4-S2](plans/P4-S2-content.md) |
| S3 | 编辑会话：`Unit.OpenSession`，心跳与结束，删页、删子树与笔记本删除时删会话，否决者与订阅者；定时清理与配置；契约、HTTP；矩阵三行 | [P4-S3](plans/P4-S3-sessions.md) |
| S4 | 整个程序：活动的注册与 `listOwnerlessNotebooks`、三条删除路径删会话、字节保真的往返、请求体上限；交错 38–43 | [P4-S4](plans/P4-S4-whole-program.md) |
| S5 | e2e 的 fixture 与 PG5–PG10、PG14，PG12 的正文与会话 | [P4-S5](plans/P4-S5-e2e.md) |

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 领域 | `CheckContent` 的边界（恰好 5 MiB、多一个字节、NUL、非法 UTF-8、空）；租约与心跳的常量；`Change.Then` 带解析结果 |
| 仓储 | `LockContent` 只锁未删页面的正文行、别的笔记本读不到；`WriteContent`；`PageContent` 的哈希；会话的增、锁、心跳（过期与别人的不改）、结束（同上）、随页面与笔记本删除（返回删掉的行）、过期清理跳过被持有的行；活动的和与最晚、只算未删的；`TouchChangeset` |
| 用例 | 码的次序（正文的 422 先于 404、403；会话先于版本）；与当前正文相同不写；会话的变更集：连续保存一个变更集与一个版本行、夹进别人的写另起一个、过期与别人的与别的页的会话；`Appender.WriteContent`；租约的时钟（假时钟：开启、到期的边界、心跳从那一刻起算、结束过期的 404）；否决者在锁下；删除时订阅者只收活着的会话；日志（id 在、标题与正文不在、没写时没有） |
| 整个程序 | 矩阵五行（心跳与结束另有"别人的会话"）；活动经 `listOwnerlessNotebooks`；三条删除路径删会话，删子树删到非根页的会话；会话的保存夹着别人的写另起变更集（版本行的 `base_revision`）；字节保真的往返（含孤立代理项与非法 UTF-8 答 400）；请求体上限；交错 38–44 |
| 端到端 | PG5–PG10 的接口版本，PG12 的正文与会话，PG14 |

反向对照（每个新检查各一个，13.4 第 1 条）：`LockContent` 不加锁（交错 38 不再等正文行）；不比较 `base_revision`；会话的变更集不看 `revision`（夹进别人的写仍原地更新）；会话的检查不看 `user_id`、`node_id` 或 `expires_at`；与当前正文相同仍写；删除子树或笔记本时不删会话（`checkPages`、`page_registrants_test`）；订阅者收到过期的会话；清理不带 `SKIP LOCKED`（交错 43 等锁）；心跳不检查过期；活动来源不登记（`listOwnerlessNotebooks` 的测试失败）；`BodyLimits` 不经中间件（5 MiB 的正文 413）；取值检查放到判定之后（码的次序）。

## 6. 完成标准

- 第 5 节的测试全部通过，`GOFLAGS=-p=3 make check`、`make gen-check`、前端的检查为绿；`make e2e`、`make image-smoke` 为绿；持续集成为绿。
- M3/P1 移交第 1 项（会话随三条路径删除）与 M3 移交第 5 项（活动）落实，00 号文档第 7 节的移交表更新。
- 审查（Opus）完成，发现已处理，记录在 `reviews/P4-content-sessions-review.md`。
- 00 号文档的进度表与本文的"结果"更新。

## 7. 结果

- 分支 `m4-p4-content-sessions`：S1 `c05624a`；S2 `9f7f0cd`；S3 `3e652c5`；S4 `4c00580`；S5 `d03c993`；持续集成修复 `5959fb3`（最大请求体的测试在竞态检测下等足）；审查修复 `8dd2534`、`209ed11`（修复核对的发现）；`59d7702` 合并（`--no-ff`）。
- 门禁：每个 Step 与审查修复的 `make check` 为绿；`make gen-check`、`make e2e`（131 个）、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P4 审查](reviews/P4-content-sessions-review.md)。
  - Major 2：正文写入在 404、403 之前解析（任何已登录的账户都能让服务端解析 5 MiB 的病态正文，约 1.4 秒、2 GB 分配）；最大正文的解析与渲染没有并发上限。改为判定之后解析，并加按字节的全局解析预算（3.6）。
  - Minor 3：慢链路上大正文存不进去（期限另加 `read_timeout`，3.8）；会话行的 `FOR UPDATE` 没有测试（交错 44）；会话的客户端只记不比（不同答 409，3.4）。Nit 全部处理或写明理由。
  - 修复另经 Opus 核对：处置全部属实；另有 3 项 Minor、2 项 Nit，合并前处理（解析 panic 时放回预算、`NewBudget` 拒绝空预算、预算接线的整个程序测试、错误路径上的释放）。
  - 反向对照：S1–S5 共 94 项，审查修复 24 项，核对之后 7 项，全部失败。

**负责人可改的决定**：解析预算用按字节的全局预算（审查 P2 的方案 ①）。
- 默认 `page.parse_budget_bytes` 8 MiB、`page.parse_max_wait` 2 秒，取不到答 503 `server_busy`。
- 8 MiB 是一页最大正文再加 3 MiB：最坏（病态正文约占 300 倍）约 2.4 GB 堆，普通正文约 40 倍。
- 部署按它配内存（与 `GOMEMLIMIT`）；不能小于 5 MiB。

**风险**（Opus 核对 C3）：写入在单元里等锁的整段时间都占着预算（解析结果要活到单元结束），预算按到达的先后服务，小请求排在大请求之后。一页的锁争用（如大笔记本的删除持笔记本行数秒，其间几页大正文的保存在等）可以让全实例的大页面阅读视图 503、小页面多等最多 2 秒。团队规模下少见；缓解（正文写入的单元设 `lock_timeout`，超时答 503）随 M12 的压测一并考虑。

**与计划的出入**（已同步进上文）：

1. S2 已实现正文写里会话的部分（锁会话、续用变更集），S3 补开启、心跳、结束、删除与清理。
2. `Jobs()` 在 `module.go`，没有 `jobs.go`；活动的两个类型审查之后从 `ports.go` 移到 `app/activity.go`（3.1）。
3. 矩阵五行：每列在目标页上用 SQL 种一个活着一小时的会话，"别人的"属于 acme 的管理员；已删笔记本那一列的会话随笔记本删掉。
4. 测试配置加 `page.edit_session_cleanup_interval: 1h`：为 0 时 River 不停地排这个任务，饿死了 identity 的清理（`testConfig` 不经校验）。审查之后，测试配置的 `write_timeout` 改为 10 秒，满足新的交叉规则。
5. 定时清理的接线另有整个程序的测试，运行时角色的测试加一个过期会话。
6. 交错 38–44 在 `interleavings_content_test.go`。39–41 与 42 的第二种持笔记本行，不持工作区行（3.11）；43 用 serve 的任务（间隔 1 秒），会话用 SQL 推到过期。
7. "`BodyLimits()` 的每个键都是路由"的测试在模块根。最大请求体的测试用几分钟的期限与客户端：竞态检测下解码 30 MB 的转义要几秒，持续集成上更久（`5959fb3`）。
8. PG12 另核对笔记本管理员也看不到别人的会话。
9. 审查之后：
   - 解析在预判定之后、预算之内（3.6）；
   - `BodyReadTimeout` 与配置的交叉规则（3.8）；
   - 会话里的写比较客户端（3.4）；
   - `SetSessionWrite` 核对行数（3.4）；
   - `SessionEnded` 与结束的日志带工作区（3.5）；
   - `edit_sessions` 不建 `expires_at` 的索引（3.2）；
   - 哈希仍在单元里算（3.4，审查 N3 不采纳）；
   - 活动是一条语句（3.2）；
   - 清理删 `expires_at <= now`（3.5）；
   - 交错 44 与整个程序的三个测试（会话的保存夹着别人的写、删子树删到非根页的会话、预算的接线）。

**留给后面的**：

- **给 M5**（已写进[编辑会话移交](../M5-collab-editing/handoffs/M4-P4-edit-sessions.md)，审查 Q1–Q4）：
  - 结束的订阅者在删掉会话行之后、持着它的行锁时被调用，不得再去锁 `page_contents`、`nodes`：会话里的保存是 `page_contents → edit_sessions`，反过来会死锁。强制解锁要先取正文行，再删会话。
  - 与当前正文相同的保存不调用守卫（答 200、不写），锁守卫拒绝不了它。
  - 被移出工作区或降为阅读者的人，会话不结束、不告诉订阅者，一个租约之内过期；锁随之最多再留 60 秒。
  - 会话的时刻都取应用的时钟：多实例的时钟偏差超过一个租约时，心跳可能违反 `expires_at > created_at`。
- **给 M6**（已写进 [M6 的移交](../M6-links/handoffs/M4-P3-markdown-extensions.md)第 9 项）：参与者追加的正文写由参与者给出解析结果，不经解析预算；观察者不得在调用结束之后留着 `Parsed`，否则那部分内存不在预算之内。
- **给 P5、P6**：
  - `getPageView`、`putPageContent`、`createPage` 可能答 503 `server_busy`（带 `Retry-After`）：阅读视图显示未加载与重试，编辑器保留文字、按 `Retry-After` 重试。
  - 带宽前提：5 MiB 在 `read_timeout` 的 30 秒内传完约需 1.4 Mbit/s 上行。请求体在 `read_timeout` 之后才到齐时答 400 `bad_request`，编辑器要说清是网络太慢。
  - 会话里的保存要用开启它的客户端（网页的会话由网页保存）。
- **活动**：活动的"最后写入"要读完笔记本的全部变更集（审查 P4）；数据量大时加 `(notebook_id, updated_at) WHERE deleted_at IS NULL` 的部分索引，M12 的压测时看。
- **已落实的移交**：M3 移交第 5 项（活动）与 M3/P1 移交第 1 项（三条删除路径删会话）落实；`LockHoldings` 的推理不受影响（页面的写不取账户行、不改成员行）。
