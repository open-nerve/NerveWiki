# M8/P1 版本：架构设计 & 实施规划

## 0. 分两部分

照 M7 的做法分成 A（服务端：迁移、读、还原、契约、权限）与 B（前端：比较、右栏的版本列表、版本页、还原的流程），各自一个分支、一次审查与修复核对、一次合并。B 以 A 合并之后的契约为准。

## 1. 基线

- 从 `main` 的 M8 设计修订（`151d49c`）开始；代码与 M7 的 Codex 评审修复合并之后（`b1e362b`）相同。
- M4 留下：每个（变更集、页面）一行 `page_revisions`，存整篇正文、哈希、字节数、`base_revision` 与 `revision`；会话的再次保存更新同一行（它的 `revision` 随之改变）；同一页的版本链连续（每行的 `base_revision` 是上一行的 `revision`）。`getPageView` 用 `PageRef{Revision}` 渲染，链接的视图在索引的版本与之不同时即时解析（M6 4.7）。
- 前端：`@codemirror/merge` 6.12.2 已是依赖，`ConflictDiff`（M4/P6）用它的统一视图，随编辑器懒加载。

## 2. 目标与范围

做（M8 设计 4.1–4.4、4.13 的版本部分、4.14 的版本部分）：

- 迁移：`page_revisions` 的唯一索引；`changesets` 的新类型与 `reverts_id`、`trimmed` 两列（后面的 Phase 用，P1 只写 `version_restore`）。
- `Write` 加 `Kind`、`RevertsID`（守卫与观察者看得到单元的类型）。
- 读：版本列表、一个版本（正文）、一个版本的阅读视图；回收站里的页同样读得到（它的链接在 P2 之前按现在的规则解析，P2 改为按原来的路径）。
- 写：还原版本。
- 权限：`page.history`、`page.restore_version`。
- 前端：Web Worker 里有上限的按行比较（`ConflictDiff` 改用它）；右栏的版本列表；版本页（阅读、与上一版比较、与当前比较、还原）。
- e2e：HI1、HI2。

不做：日志与单个变更集的页（P3；P1 的版本页只显示版本所属变更集的类型、执行者与时刻）；回收站（P2）；搜索（P5）。

## 3. A：服务端

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `server/migrations/sql/00033_page_revisions_node_revision.sql` | 唯一索引 `page_revisions_node_id_revision_key (node_id, revision)`；去掉 `page_revisions_node_id_idx`（被它覆盖：删除子树与清理按 `node_id` 找版本行，前缀相同） |
| `server/migrations/sql/00034_page_changesets_history.sql` | `changesets_kind_check` 加 `version_restore`、`trash_restore`、`revert`；`reverts_id uuid` 与 `changesets_reverts_check CHECK ((kind = 'revert') = (reverts_id IS NOT NULL))`；`trimmed boolean NOT NULL DEFAULT false`。Down 先把新类型的行改回 `edit`、清掉 `reverts_id`（照 00030 的写法），再收回约束与列 |
| `page/domain/changeset.go` | 三个新类型 |
| `page/domain/actions.go` | `ActionHistory = "page.history"`、`ActionRestoreVersion = "page.restore_version"` |
| `page/domain/errors.go` | `ErrVersionNotFound`（404 `page.version_not_found`） |
| `page/domain/version.go` | `Version`（一行版本的元数据与它的变更集）、`VersionCursor` |
| `page/app/extension.go`、`unit.go` | `Write.Kind`、`Write.RevertsID`；`UnitSpec.RevertsID`；`ensureChangeset` 写 `reverts_id` |
| `page/app/ports.go` | `Versions` 端口（4 个读）；`Nodes` 加 `FindAnyNode`（不论是否已删） |
| `page/app/readable.go`（从 `get_page.go` 移出） | `readable`（活着的页）与 `readableAny`（活着或在回收站里的页，笔记本活着），同样的 404 与判定 |
| `page/app/list_versions.go`、`get_version.go`、`get_version_view.go`、`restore_version.go` | 四个用例 |
| `page/adapter/postgres/queries/versions.sql`、`versions.go` | 查询与端口的实现 |
| `page/adapter/postgres/queries/nodes.sql` | `FindAnyNode` |
| `page/adapter/http/handler.go`（与新文件 `versions.go`） | 四个处理器 |
| `api/modules/page.yaml` | 四个操作与它们的结构 |
| `access/domain/rules.go` | 两条规则 |
| `bootstrap` | 用例的接线；权限矩阵的新行；整个程序上的测试 |

### 3.2 迁移

- **唯一索引**建在含已删行的全部行上：一页（不论是否已删）的版本号各不相同，现有的数据由写入的方式保证（M4 的不变式：每次写 `revision + 1`，版本行的 `revision` 是写出的那个），迁移之前没有重复。迁移测试在已有的 schema 测试里核对索引与约束的名字（`schema_test.go`）。
- **`changesets` 的约束**：`kind` 的取值、`reverts_id` 与 `kind = 'revert'` 同在；`trimmed` 默认 `false`。`reverts_id` 不建外键（M8 设计 4.2）。

### 3.3 `Write.Kind` 与 `Write.RevertsID`

`UnitSpec.Kind`（已有，默认 `edit`）与新的 `UnitSpec.RevertsID` 进 `Write`，在 `Run` 里与时刻一起定下；`ensureChangeset` 新建变更集时写 `reverts_id`，并入别的变更集时核对它与目标的相同（与 `kind` 的核对一样）。守卫与观察者不必改：今天的守卫不看它们，M10、M11 用（M8 设计 4.1）。单元测试：一个 `revert` 的单元没有 `RevertsID` 是调用者的错误（与"没有凭据的执行者"同理，返回错误，不写）。

### 3.4 读：端口与查询

`Versions` 端口（`page/app/ports.go`）：

```go
type Versions interface {
	// ListVersions is the page's versions newer-first, below the revision below (domain.MaxRevision + 1: from the
	// newest), at most limit; each with its changeset. No content.
	ListVersions(ctx context.Context, pageID uuid.UUID, below int, limit int) ([]domain.Version, error)
	// FindVersion is the first version row at or above revision, with its content, read in one statement; the
	// caller checks its base.
	FindVersion(ctx context.Context, pageID uuid.UUID, revision int) (domain.Version, string, error)
	// AliveChangesetOf is the changeset of the page's alive edit session, zero when none: the newest version's row
	// is "being edited" when it is that changeset's.
	AliveChangesetOf(ctx context.Context, pageID uuid.UUID, now time.Time) (uuid.UUID, error)
}
```

- `ListVersions`：`SELECT r.id, r.changeset_id, r.base_revision, r.revision, r.byte_size, r.created_at, r.updated_at, c.kind, c.client, c.created_by_id, c.created_at FROM page_revisions r JOIN changesets c ON c.id = r.changeset_id WHERE r.node_id = $1 AND r.revision < $2::bigint ORDER BY r.revision DESC LIMIT $3`，走唯一索引倒序；从最新的读时用例传 `domain.MaxRevision + 1`，不用"0 表示不限"的写法，保留下来的通用计划同样是索引上的范围（审查 C-S1）。不读 `content`（至多 5 MiB 一行，M8 设计审查 A-N8）。不按 `deleted_at` 过滤：回收站里的页的版本照列（M4 设计的表："历史不按它过滤"）。
- `FindVersion`：`… WHERE r.node_id = $1 AND r.revision >= $2 ORDER BY r.revision LIMIT 1`，连同 `content` 一条语句读出（编辑会话的保存原地更新同一行，分两条读会让版本号与正文不一致，审查 A-2、B-M1），再在 Go 里核对 `base_revision` 为空或小于请求的版本号，不然 `ErrVersionNotFound`（版本链连续，第一个 `revision >= N` 的行就是唯一可能含 N 的行）。
- `AliveChangesetOf`：这一页活着的会话（`AliveSessionsOf` 的同一条件）的 `changeset_id`。
- 计划：这几条都按 `(node_id, revision)` 的唯一索引；13.4 第 9 条的计划测试不必另加（单表、等值加范围，与已有的同类查询一样），在适配器的测试里对一页上千个版本测倒序与游标。

`Nodes.FindAnyNode`：`FindNode` 去掉 `deleted_at IS NULL`，带上 `deleted_at`。`readableAny` 用它：节点是页面（不论是否已删）、笔记本活着（`WorkspaceOf` 对已删的笔记本答 `ok = false`，照旧 404）、能读这本笔记本（`page.history`）。

### 3.5 用例

- **`ListVersions`**（`GET /pages/{page_id}/versions?limit&cursor`）：游标先核对（400），`readableAny`（404、`page.history` 的 403 照"看不到答 404"：读者以上都能读，所以实际只有 404），`limit` 取值（422，`shared.PageSize`）；读 `limit + 1` 行；最新的一行若属于活着的会话，标 `editing`；执行者的显示名经 `Names` 一次读出。游标是版本号（`domain.VersionCursor`，经 `shared.EncodeCursor`）。
- **`GetVersion`**（`GET /pages/{page_id}/versions/{revision}`）：`readableAny` → `FindVersion`（404 `page.version_not_found`，正文同一条语句读出）；`editing` 照列表：这一行是最新的且属于活着的会话；答复带这一行真实的 `revision`（请求的版本号落在它的区间里时与请求的不同，网页随之替换地址，M8 设计 4.3）。
- **`GetVersionView`**（`GET /pages/{page_id}/versions/{revision}/view`）：同上找到行，再照 `GetPageView` 取预算、渲染（`PageRef{Revision: row.Revision}`）；答 `ReadingView`，`Revision` 是这一行的。
- **`RestoreVersion`**（`POST /pages/{page_id}/versions/{revision}/restore`，`{base_revision}`）：
  1. 找节点（`readableAny` 以 `page.history` 先判定，未加锁；之后已删的页 404 `page.not_found`：先从回收站恢复。读者看得到的页才区分已删与否，不泄露）；
  2. `FindVersion`（404 `page.version_not_found`）与它的正文；
  3. `ContentParser.Parse(spec, content)`，`spec` 是 `{NotebookID, Action: ActionRestoreVersion, Client, Options{UpdateLinks: true}, NotFound: ErrNotFound, Kind: ChangesetVersionRestore}`：先未加锁地判定（404、403），再取预算解析（503）；
  4. `Writer.Run(spec, …)` 里 `u.WriteContent(ContentWrite{NodeID, Content, Facts, Base: base_revision})`：与当前相同时不写（`Outcome.ChangesetID` 为零）；否则核对 `base_revision`（409 `page.revision_mismatch`），经守卫（409 `page.locked`，调用者自己在别处编辑同样被拒），写新版本；
  5. 答 `{revision, changeset_id, changed}`；写了的记日志（照 `PutPageContent`：ids、`revision`、客户端，不记正文）。
  - 码的次序：404 → 403 → 503 → 409 `page.revision_mismatch` → 409 `page.locked`（M8 设计 4.14）。`UpdateLinks` 与正文的写相同：还原不是改名、移动，参与者本来就不动作。

### 3.6 契约（`api/modules/page.yaml`）

- `listPageVersions`：`PageVersionPage {data: PageVersion[], next_cursor}`（与其他分页的答复同名，审查之后由 `PageVersionList` 改名）；`PageVersion {id, revision, base_revision|null, byte_size, created_at, updated_at, editing, changeset: VersionChangeset}`；`VersionChangeset {id, kind, client, created_at, created_by: ChangesetAuthor}`；`ChangesetAuthor {user_id, display_name}`（6.1 的例外：版本、日志、回收站内嵌执行者的显示名，M8 设计 4.1）。`x-problem-codes: [bad_request(游标), page.not_found, validation_failed]`。
- `getPageVersion`：`PageVersionContent`（`PageVersion` 的字段加 `content`）；`x-problem-codes: [page.not_found, page.version_not_found]`。
- `getPageVersionView`：`PageView`（与 `getPageView` 相同）；`[page.not_found, page.version_not_found, server_busy]`。
- `restorePageVersion`：请求 `VersionRestore {base_revision}`（整数，未知字段、`null`、小数 400；不设 `minimum`：与当前版本号不符的照第 4 步答 409 `page.revision_mismatch`，与 `PageContentWrite` 相同）；答 `VersionRestored {revision, changeset_id|null, changed}`；`[page.not_found, page.version_not_found, forbidden, server_busy, page.revision_mismatch, page.locked]`。
- `{revision}` 路径参数：整数，不设 `minimum`：0 及以下与没有的版本号一样答 404 `page.version_not_found`（契约的描述写明），不让同一类错误有两个码。`kind` 的枚举写全五种（`edit`、`import`、`version_restore`、`trash_restore`、`revert`），前端照全部的取值显示。
- 描述照现有操作的写法：读者以上都能读；回收站里的页同样读得到；中间版本的规则；还原的码。

### 3.7 权限、日志与交错

- `page.history`：读者、编辑者、管理员；`page.restore_version`：编辑者、管理员（`access/domain/rules.go`）；`boot/actions_test.go` 的"规则 = 动作"；权限矩阵（`permission_matrix_test.go`）加四个操作的行，`permission_matrix_coverage_test.go` 随之通过。
- 日志：还原记 `page content restored`，带 ids、版本号、恢复自的版本号、客户端。读不记。
- 交错（`bootstrap/interleavings_page_test.go` 的写法）：还原与同一页的编辑会话保存（会话持正文行时，还原等它，之后 409 `page.revision_mismatch` 或 `page.locked`）；还原与删除这一页（删除先拿到笔记本的锁时，还原 404）。结束时 `checkPages`、`checkLinks`。

### 3.8 测试

- **用例**（`page/app`，假的端口）：列表的游标、`limit`、`editing` 的标记、显示名；中间版本的寻址（落在区间里、不在任何区间里）；回收站里的页读得到、笔记本已删时 404；还原的每个码与次序、相同正文不写、`Kind` 是 `version_restore`。
- **适配器**（真的 PostgreSQL）：倒序与游标（上千个版本）、`FindVersion` 的区间、唯一索引、已删的行照列、不读正文（查询的列）。
- **handler**：契约的结构、未知字段、路径参数、各码。
- **整个程序**（`bootstrap`）：还原之后正文、版本、索引（`checkLinks`）与 `pages`、`links` 事件；一个 `version_restore` 变更集；被锁时 409 带持锁人。
- **e2e 的接口版本**：HI1、HI2 的 PAT 一半（第 4.7 节）。

### 3.9 审查之后的改动

三位审查者（A：正确性与并发；B：契约、数据与测试；C：设计的符合与演进）的处置见 `reviews/P1A-versions-server-review.md`。与上文最初的设计不同之处已改在各节里，另有两处是新加的：

- **版本链的守卫**：`RecordRevision` 的 `ON CONFLICT … DO UPDATE` 只在已有的行的 `revision` 等于这次写的 `base_revision` 时更新（`:execrows`，0 行即错误，单元失败）。今天的写入者都满足（会话续写在正文行的锁下核对版本号），守卫让以后分批的写（P2 的恢复、P3 的撤销）不会悄悄让一行跨过别的变更集的版本（审查 C4、A 的潜在风险）。`checkPages` 加"版本不连成链"一项，调用它的交错测试都核对 `FindVersion` 依赖的这条不变式。
- **`changesetOf`** 只接受 `edit` 的单元：编辑会话的变更集只有编辑写。

## 4. B：前端

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/lib/line-diff.ts` | 按行比较（行的哈希上的 Myers），纯函数，带步数的上限 |
| `web/apps/web/src/lib/line-diff.worker.ts`、`line-diff-client.ts` | Worker 与它的调用：硬性的期限一到就 `terminate()`，答"太大" |
| `web/apps/web/src/editor/text-diff.tsx` | 只读的比较视图：统一视图（经 `diffConfig.override` 用算好的改动）或并排；摘要、上一处与下一处、标记 |
| `web/apps/web/src/editor/conflict-view.tsx` | `ConflictDiff` 改用 `TextDiff` |
| `web/apps/web/src/services/page.service.ts` | 四个操作的调用与类型 |
| `web/apps/web/src/pages/page/page-versions.tsx` | 右栏"版本"一节 |
| `web/apps/web/src/pages/page/version-page.tsx`、`version-restore.tsx` | 版本页、还原的确认框 |
| `web/apps/web/src/app/routes.tsx` | `pages/:pageId/versions/:revision` |
| `web/apps/web/src/events/handlers.ts`、`app/event-stream.tsx` | 版本列表与版本视图的重读、连上时的重读 |
| `web/apps/web/src/i18n/messages/{zh-CN,en}.ts` | 文字 |

### 4.2 比较

- **`lineDiff(a, b, {maxSteps})`**：两段文字先按行切开（`\n`，比较之前与编辑器一样把 CRLF、CR 规整为 LF，并记下两边是否只差换行与 BOM），行映射为整数，去掉共同的头尾，在中间做 Myers；步数超过 `maxSteps` 时答"太大"。答改动的行区间，再换算为字符的偏移。
- **Worker**：`lineDiffInWorker(a, b)` 在 Web Worker 里跑 `lineDiff`，2 秒没有答复就 `terminate()` 并答"太大"；同一时刻至多一个（新的比较取消旧的）。jsdom 里测纯函数与调用的超时（假的 Worker）。
- **细分**：一处改动两边各不超过 2,000 个字符时，用 `@codemirror/merge` 导出的 `diff` 在这一处里细分到字符；大的改动按行显示。
- **上限**：两边合计超过 5 MiB、改动的行超过 5,000，或 Worker 超时，就不逐行比较：显示"+a −b 行，太大，不能逐行比较"（能数出时）与两个版本的只读编辑器并排。
- **视图**：统一视图（`unifiedMergeView`，`diffConfig.override` 返回算好的改动，`mergeControls: false`，未改动的段落折叠）；删除的大段不作为一整块 DOM（超过上限时走并排，上限之内的删除段落至多 5,000 行）；改动不只靠颜色：加的行前 `+`、删的行前 `−` 与删除线；"N 处改动"的摘要；"上一处、下一处"的按钮（`goToPreviousChunk`、`goToNextChunk`）。
- **只差换行或 BOM** 时说明这一点（视图里没有改动）。
- **`ConflictDiff`** 改用同一个视图，保留它现有的折叠与标签（M4/P6 的测试照旧通过）。

### 4.3 右栏的版本列表

- `PageVersions`：SWR 键 `["page-versions", notebookId, pageId]`，"按已读的页数重读"（13.2 第 19 条的 `readPages` 那种：重读时读回同样多的页），每页 20 项，"更多"读下一页；项以行 id 为键。每项：时刻（相对时间与完整时间的提示）、执行者、类型（"编辑""导入""还原""恢复""撤销"）、"编辑中"的标记；最新的一项标"当前"。点开进版本页。列表的名称"版本"，标题下写"一次编辑一个版本"。
- 重读：`pages` 事件点到这一页（或 `pages` 为空）时按 `refresher` 的间隔重读；本标签页自己保存、离开编辑之后立即更新；连上时重读（`refreshedOnConnect`）。

### 4.4 版本页

- 路由 `/:slug/notebooks/:id/pages/:pageId/versions/:revision`，在页面的外壳里（P1 只有活着的页的版本页；回收站里的在 P2 的预览里）。文档标题"版本 N · 页面名"。
- 版本的信息：版本号、时刻、执行者、类型；答复的版本号与地址不同时 `replace` 地址。
- 三个标签页（有名称的一组）："阅读"（只读的阅读视图：不能勾任务、未解析的链接不给"新建这一页"）、"与上一版比较"（第一个版本没有这一项）、"与当前比较"（方向是"当前 → 这个版本"，即还原会做的改动，图例写明）。比较读两个版本的正文（`getPageVersion`），视图读 `getPageVersionView`，各用自己的 SWR 键，离开版本页时从缓存去掉。
- 中栏的锁提示（`EditLockNote`）同样显示。
- 写者有"还原到这个版本"（最新的版本没有）：确认框（4.5）。

### 4.5 还原的流程

- 确认框读当前的正文（`getPageContent`），显示"当前 → 这个版本"的比较；`base_revision` 就是这次读到的版本号。
- 确认：`restorePageVersion`。
  - `changed: false`：说明"与当前相同，没有改动"，关闭确认框。
  - 成功：回到页面，焦点在标题，状态行"已还原到版本 N"（`useNotice` 的礼貌播报），版本列表更新。
  - 409 `page.revision_mismatch`：重读当前、刷新比较，确认框留着并说明"页面在这期间改过了"（不自动重发，13.2 第 7 条）。
  - 409 `page.locked`：照现有的锁的说明（持锁人）。
  - 404：页已不在（被删或移走），说明并关闭。
- 还原经 `oneAtATime`（同一个确认框里的再次点击不重发）。

### 4.6 测试（vitest）

- `lineDiff`：一般的改动、只差换行或 BOM、步数的上限、空的一边、只有头尾的改动；与 `@codemirror/merge` 的 `diff` 在小输入上逐行一致的性质测试。
- Worker 的调用：超时之后 `terminate()` 并答"太大"；新的比较取消旧的。
- `TextDiff`：上限处（5 MiB、5,000 行、超时）的并排与摘要；标记、摘要与上一处、下一处；B-I1 的形状（3 万字改 8 个词）显示为 8 处改动。
- `PageVersions`：分页、"编辑中"、"当前"、重读的条件；经组合根到达右栏。
- 版本页：三个标签页、地址的替换、只读、第一个版本没有"与上一版"、锁提示；还原的每种答复。
- `ConflictDiff` 现有的测试照旧通过。

### 4.7 e2e

- **HI1**（`e2e/stories/history/hi1-versions.spec.ts`）：两次编辑会话之间离开编辑，右栏的版本列表有三个版本；打开旧版本阅读（只读，没有任务复选框的勾选）；与上一版、与当前比较显示改动；接口版本：PAT 读列表与两个版本的正文，内容与写入的逐字节相同。
- **HI2**（`hi2-version-restore.spec.ts`）：还原到旧版本，正文与索引随之改变（`links.ts` 的断言），版本列表多一个"还原"的版本（数据库：一个 `version_restore` 变更集）；另一个账户在编辑时还原被拒并显示持锁人；与当前相同时说明没有改动；接口版本：PAT 还原与 409。
- 数据库断言放在 `e2e/fixtures/assert/page.ts`（13.4 第 3 条按模块）。

## 5. 实施步骤

A（分支 `m8-p1a`）：

1. 迁移两条与 schema 测试；`changesets` 的新类型；`Write.Kind`、`RevertsID`。
2. `Versions` 端口、查询与适配器测试；`FindAnyNode`、`readableAny`。
3. 三个读的用例与测试；还原的用例与测试。
4. 契约、处理器、handler 测试；`make gen`。
5. 权限规则、矩阵、组合根；整个程序上的测试与交错。
6. 反向对照（每个用例与查询各改坏一处）、门禁、审查。

B（分支 `m8-p1b`，A 合并之后）：

1. `lineDiff` 与 Worker；`TextDiff`；`ConflictDiff` 改用它。
2. 服务与类型；右栏的版本列表；事件的重读。
3. 版本页与还原的流程；文字。
4. vitest、e2e HI1、HI2；反向对照、门禁、审查。

## 6. 测试与验证

- 门禁：`make lint-go lint-web knip gen-check test-go test-web build`、e2e 全量。
- 反向对照：A 的每个用例与查询（区间的条件、倒序、`editing`、码的次序、`Kind`）、B 的比较（上限、超时、细分）、列表的重读、还原的每种答复，各改坏一处，测试要失败。
- 审查：每部分三位 Opus 审查者并行（A：数据与用例、契约与权限、测试；B：比较与性能、界面与无障碍、测试与 e2e），之后 Opus 核对修复，直到一轮没有行为上的发现。

## 7. 完成标准

- 第 2 节的每一项做完，第 3.8、4.6、4.7 节的测试通过，门禁全绿。
- 00 号文档的进度表、本文档的"结果"一节更新；审查记录 `reviews/P1A-versions-server-review.md`、`reviews/P1B-versions-web-review.md`。

## 8. 交给 P2 与后面的 Phase

- P2：回收站里的页的链接按原来的路径解析（`Paths` 的来源变体）；回收站的预览用 `getPageVersionView`。
- P3：日志与单个变更集的页；版本页的变更集信息链到它。

## 9. 结果

### 9.1 A：服务端（2026-10-13，合并 `12fc5d7`）

- 提交：实施 `8609c58`；CI 首轮的修补 `23be652`（网页缺 `page.version_not_found` 的文案、适配器夹具的时刻类型、权限矩阵的覆盖认 `{revision}`）；审查的修复 `78955b4`；两轮修复核对的修复 `d363d77`、`ce841f4`。合并 `12fc5d7`。
- 审查：三位审查者（Opus）。高 1（适配器的版本测试跑不通，CI 是红的：夹具的 `node_id` 参数被推成 text）；其余是低与测试的缺口：版本的元数据与正文分两条语句读、版本链只靠调用者维持、几处测试分不出它声称的、契约的措辞与命名。修复核对两轮，没有行为问题。逐条见[审查记录](reviews/P1A-versions-server-review.md)。
- 与设计不同之处已改在第 3 节（3.4–3.6、3.9）：`FindVersion` 一条语句连正文读出、没有 `VersionContent`；`ListVersions` 读 `below` 之下（最新的从 `MaxRevision + 1`）；`RecordRevision` 的版本链守卫与 `checkPages` 的链；`changesetOf` 只接受编辑；`PageVersionPage`、`ChangesetAuthor`；路径的版本号与 `base_revision` 不设 `minimum`。
- 反向对照：用例 28 处、数据库与规则 17 处、核对之后新加 3 处，都被测试抓到。

### 9.2 B：前端

（完成后补写。）
