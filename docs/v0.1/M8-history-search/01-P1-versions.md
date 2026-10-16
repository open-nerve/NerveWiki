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

- `listPageVersions`：`PageVersionPage {data: PageVersion[], next_cursor}`（与其他分页的答复同名，审查之后由 `PageVersionList` 改名）；`PageVersion {id, revision, base_revision|null, byte_size, created_at, updated_at, editing, changeset: VersionChangeset}`；`VersionChangeset {id, kind, client, created_at, created_by: UserRef}`；`UserRef {user_id, display_name}`（6.1 的例外：版本、日志、回收站内嵌执行者的显示名，M8 设计 4.1）是 `common.yaml` 的，与持锁人、任务的发起人、回收站的删除者共用（P1A 时叫 `ChangesetAuthor`，M8/P2 3.9 并入）。`x-problem-codes: [bad_request(游标), page.not_found, validation_failed]`。
- `getPageVersion`：`PageVersionContent`（`PageVersion` 的字段加 `content`）；`x-problem-codes: [page.not_found, page.version_not_found]`。
- `getPageVersionView`：`PageView`（与 `getPageView` 相同）；`[page.not_found, page.version_not_found, server_busy]`。
- `restorePageVersion`：请求 `VersionRestore {base_revision}`（整数，未知字段、`null`、小数 400；不设 `minimum`：与当前版本号不符的照第 4 步答 409 `page.revision_mismatch`，与 `PageContentWrite` 相同）；答 `VersionRestored {revision, changeset_id|null, changed}`；`[page.not_found, page.version_not_found, forbidden, server_busy, page.revision_mismatch, page.locked]`。
- `{revision}` 路径参数：整数，不设 `minimum`：0 及以下与没有的版本号一样答 404 `page.version_not_found`（契约的描述写明），不让同一类错误有两个码。`kind` 的枚举写全五种（`edit`、`import`、`version_restore`、`trash_restore`、`revert`），前端照全部的取值显示。
- 描述照现有操作的写法：读者以上都能读；回收站里的页同样读得到；中间版本的规则；还原的码。

### 3.7 权限、日志与交错

- `page.history`：读者、编辑者、管理员；`page.restore_version`：编辑者、管理员（`access/domain/rules.go`）；`bootstrap/actions_test.go` 的"规则 = 动作"；权限矩阵（`permission_matrix_test.go`）加四个操作的行，`permission_matrix_coverage_test.go` 随之通过。
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
| `web/apps/web/src/lib/line-diff.ts` | 按行比较（行编号上的 Myers，线性空间），带步数与改动行数的上限；插入、删除滑过重复的行；小的改动细分到词并合并（4.2） |
| `web/apps/web/src/lib/line-diff.worker.ts`、`line-diff-client.ts` | Worker 与它的调用 `compareInWorker`：5 MiB 以上不起 Worker，硬性的期限一到就 `terminate()`，答"太大"；Worker 失败时拒绝；小的文字在主线程上比较（`compareNow`） |
| `web/apps/web/src/editor/text-diff.tsx` | 只读的比较视图：规整换行与 BOM，统一视图（经 `diffConfig.override` 用算好的改动）或并排；摘要、上一处与下一处、标记；失败与重试；新的比较进行时留着上一次的 |
| `web/apps/web/src/editor/conflict-view.tsx` | `ConflictDiff` 改用 `TextDiff` |
| `web/apps/web/src/pages/page/comparison.tsx`（新） | 懒加载 `TextDiff`（随编辑器的分包，`build/out-of-main.ts` 把 `text-diff.tsx` 算作编辑器的入口），载入失败时说明并重试；"显示未改动的行" |
| `web/apps/web/src/components/ui/tabs.tsx`（新） | Radix 的标签页 |
| `web/apps/web/src/services/page.service.ts`、`stores/page-tree.store.ts` | 四个操作的调用与类型；store 的 `content`（`getPageContent`）、`versions`、`version`、`versionView`、`restoreVersion`（按页的 `oneAtATimeById`） |
| `web/apps/web/src/pages/page/page-versions.tsx` | 右栏"版本"一节；`useVersions`（版本页用同一个键知道最新的版本） |
| `web/apps/web/src/pages/page/version-page.tsx`、`version-restore.tsx` | 版本页、还原的确认框 |
| `web/apps/web/src/app/routes.tsx` | `pages/:pageId/versions/:revision`（笔记本的外壳之下，懒加载） |
| `web/apps/web/src/events/handlers.ts`、`app/event-stream.tsx`、`pages/page/edit-lock-note.tsx`、`page-edit.tsx` | 版本列表、版本的正文与视图、页面的当前正文的重读；连上时的重读；租约过期之后、离开编辑之后的重读 |
| `web/apps/web/src/pages/page/page-layout.tsx`、`breadcrumbs.tsx`、`unresolved-link.tsx`、`page-panel.tsx`、`i18n/format.ts` | 共用的改动：`PageGone`（树里找不到页时的去向，版本页同用）与还原之后的状态行；面包屑的 `part`；`useUnresolvedLinks({ readOnly })`；右栏挂上版本；`formatRelative` |
| `web/apps/web/src/i18n/messages/{zh-CN,en}.ts` | 文字 |
| `web/apps/web/src/test/page-server.ts` | 假的服务端的四个操作：区间的寻址、`version_not_found`、还原的码的次序、游标是下面的版本号（同服务端） |

### 4.2 比较

- **`lineDiff(a, b, limits)`**：两段文字由调用者先照编辑器规整（`TextDiff` 用 `splitBreaks`：CRLF、CR 改为 LF，去掉 BOM，并记下两边是否只差这些），按行（连同行尾的 LF：最后一行有没有换行算不同）映射为整数，去掉共同的头尾，在中间做 Myers 的线性空间版本（未做的部分放在栈上，不递归）；步数超过 `maxSteps`（2,000 万，笔记本上约 150 毫秒）时答"太大"，不带行数。之后每处纯插入、纯删除往下滑过与它之后相同的行，直到下一处改动（碰到就并上）：不从两边都是空行的地方开始，`@codemirror/merge` 会把这样的改动画晚一行，只有那个空行时什么也画不出（审查 A-M1）。改动的行合计超过 `maxChangedLines`（5,000）时答"太大"，带行数。答改动的字符区间。
- **细分**：一处改动两边各不超过 `maxRefined`（2,000 个码元）、细分的合计不超过 `maxRefinedTotal`（1 MiB，只算细分了的，审查 A-M2）时，用 `@codemirror/merge` 导出的 `presentableDiff`（`scanLimit` 500）细分到词，并重复到一遍不再合并为止；相距不到 3 个码元的改动合并、空的去掉。这些都在 Worker 里做：视图在主线程上对全文的改动还要做一遍 `makePresentable`，它的合并逐个 `splice` 整个数组，是平方级的（一页上万处改动时几秒到几十秒），交给它的改动已经没有可合并的（审查 A-I1、修复核对第一轮 N2）。大的改动与超出合计之后的按整行显示。
- **Worker**：`compareInWorker(a, b, signal)` 在 Web Worker 里跑 `lineDiff`：两边 UTF-8 合计超过 5 MiB 不起 Worker，答"太大"；2 秒没有答复就 `terminate()` 并答"太大"；Worker 起不来、脚本载不到（升级之后旧的分包答 404）、答复读不出（`messageerror`）时结束它并拒绝，视图说"比较失败"并给"重试"（审查 A-M3）。合计不超过 1,000 行且不超过 32K 个码元的文字在主线程上直接比较（`compareNow`），不起 Worker。"同一时刻至多一个"是按视图的：文字变了、重试、卸载时中止这个视图的上一个；不是全局的：版本页的"与当前比较"开着时打开还原的确认框，两个视图各比较一次同一对文字（都有上限、都会结束，审查 A 的细节，接受）。jsdom 里测纯函数与调用（假的 Worker）。
- **上限**：两边合计超过 5 MiB、改动的行超过 5,000，或 Worker 超时，就不逐行比较：显示"+a −b 行，太大，不能逐行比较"（能数出时）与两个版本的只读编辑器并排。视图会把只隔一个空行的改动并成一块，删除的一块超过 5,000 行时同样并排（`removedLines`，审查 A-M5）：上限之内的删除段落至多 5,000 行。
- **视图**：统一视图（`unifiedMergeView`，`diffConfig.override` 只对算过的那一对文字返回算好的改动，`mergeControls: false`，未改动的段落折叠）；改动不只靠颜色：加的行前 `+`、删的行前 `−` 与删除线；"N 处改动"（视图的块数）与行数的摘要；"上一处、下一处"的按钮（`goToPreviousChunk`、`goToNextChunk`）；图例"之前 → 之后"。新的文字还在 Worker 里比较时，上一次的比较留着（`aria-busy`、变淡，摘要旁写"正在比较…"），上一处、下一处与它们的焦点不丢（审查 A-M4、修复核对第一轮 N1）；迟到的、已中止的答复丢弃，旧文字的失败不显示在新文字上。`shown` 告诉调用者正显示的是哪一份"之前"的比较（比较进行中、失败时没有），还原的确认框据此启用"还原"（4.5）。
- **只差换行或 BOM** 时说明这一点（视图里没有改动）；两边相同时说"没有改动"。
- **`ConflictDiff`** 改用 `TextDiff`，保留它的折叠与标签（M4/P6 的测试照旧通过）；它新有摘要与上一处、下一处，相同或只差换行时说明，不再显示空的编辑器。
- **懒加载**：`pages/page/comparison.tsx` 动态导入 `editor/text-diff`，载入失败时说明并"重试"；它带"显示未改动的行"的按钮（键盘到得了，折叠到不了）。

### 4.3 右栏的版本列表

- `PageVersions`（右栏的最后一节）：SWR 键 `["page-versions", notebookId, pageId]`，"按已读的页数重读"（13.2 第 19 条的 `readPages` 那种：重读时读回同样多的页），每页 20 项；"更多"读下一页、接在它读的那一页之后，接上时有重读在途的再整体读一次（反链的做法），焦点照 M7 的 `useLoadMore`、`LoadMoreButton`；项以行 id 为键。每项：相对时间（`formatRelative`；`<time>` 的提示是完整时间）是到版本页的链接，名称以它开头、带版本号与完整时间（"1 小时前，版本 7，…"：同一小时的两个分得开，审查 B-M3）；"当前"（最新的一项）与"编辑中"的标记；"执行者 · 类型"（"编辑""导入""还原""恢复""撤销"；英文 Edit、Import、Restore、Restore from trash、Undo change）。列表的名称"版本"，标题下写"一次编辑一个版本"。版本页经同一个键（`useVersions`）知道最新的版本。
- 重读：`pages` 事件点到这一页（或 `pages` 为空）时经 `refresher` 重读；`lock` 事件在树之后立即重读；本标签页离开编辑之后立即重读（自己的每次自动保存交给事件与 `refresher`，不另读）；锁的说明读到空闲而之前有人持有时重读（租约过期不是事件，`EditLockNote` 的 `readEnded`，审查 B-M2）；连上时重读（`refreshedOnConnect`）。

### 4.4 版本页

- 路由 `/:slug/notebooks/:id/pages/:pageId/versions/:revision` 是笔记本的外壳之下自己的一条（懒加载），不在页面的外壳里，没有右栏与子页面：照 `PageLayout` 在树里找页，找不到时经共用的 `PageGone`（这一代删除的去被删子树的父页，其余 404）。地址的版本号不是 `^[1-9]\d{0,14}$` 的（`versions/0`、`versions/01`）显示 404。P1 只有活着的页的版本页；回收站里的在 P2 的预览里。标题"版本 N"，文档标题"版本 N · 页面名 · 笔记本名"，面包屑以页面的链接与"版本 N"结束（`Breadcrumbs` 的 `part`）；读到版本之后标题用答复的版本号。
- 版本的信息：完整的时刻、"执行者 · 类型"、"当前"与"编辑中"的标记。答复的版本号与地址不同时先把答复放进它自己版本号的键，再 `replace` 地址；地址的版本号变了（替换、编辑会话的再次保存、到别的版本），之前那个版本号的正文与视图从缓存去掉（`useLeftRevision`，审查 B-I1）。
- 三个标签页（有名称的一组，Radix 的 tabs；手动激活：方向键、Home、End 只移焦点，Enter、空格才切换，每个比较可能读 5 MiB；选的标签页不进地址；只挂载选中的，切换即中止它的比较）："阅读"（只读的阅读视图，按答复的版本号读：增强照常运行但不给 `toggleTask`，写者也不能勾；未解析的链接经 `useUnresolvedLinks({ readOnly: true })` 说明版本只供阅读，不问服务端、不给"新建这一页"；附件地址的到期照 `page-view.ts`：`stamped`、`eachRead`、`useAssetsExpiry`；增强报告的错误在视图换了之后清掉）、"与上一版比较"（第一个版本没有这一项；读 `["page-version", 笔记本 id, 页 id, base_revision]`）、"与当前比较"（读页面的当前正文 `["page-content", 笔记本 id, 页 id]`，与还原的确认框共用；方向是"当前 → 这个版本"，即还原会做的改动，图例写明）。
- 缓存：版本的正文 `["page-version", 笔记本 id, 页 id, 版本号]`、视图 `["page-version-view", 笔记本 id, 页 id, 版本号]` 与 `page-content` 离开版本页时从缓存去掉（下一轮才去，StrictMode 的重新挂载不清；带默认的重读，回来时不被去重挡住）。版本的正文挂载时与窗口取得焦点时不重读（`revalidateIfStale`、`revalidateOnFocus` 为 false，审查 B-M1）：它只在编辑中才变，由事件重读；视图照常在焦点时重读（附件的地址）。
- 中栏的锁提示（`EditLockNote`）同样显示。
- 写者有"还原到这个版本"：最新的版本没有；"最新"从版本列表的键读，列表读不到时没有这个按钮。确认框见 4.5。

### 4.5 还原的流程

- 确认框打开时读当前的正文（`["page-content", …]`，与"与当前比较"共用），显示"当前 → 这个版本"的比较；`base_revision` 就是这次读到的版本号。"还原"只在屏幕上的比较正是这份正文的时候可用（`TextDiff` 的 `shown`）：比较进行中（上一次的还留着）或失败时不可用，送出的版本号总是显示着的比较的（修复核对第一轮 N1）。
- 确认：经 `PageTreeStore.restoreVersion`（按页的 `oneAtATimeById`，13.2 第 1 条）；发送中两个按钮禁用，Esc 不关闭。
  - `changed: false`：关闭确认框，版本页一直挂载的 `<output>` 说"与当前相同，没有改动"，焦点回到"还原"。
  - 成功：去掉页面视图的缓存、重读版本列表；版本页仍挂载时回到页面（路由状态 `{arrived, restored: N}`），焦点在标题。状态行"已还原到版本 N"是页面外壳里的 `<output>`（不是 `useNotice`），一开始就有文字，标题经 `aria-describedby` 指向它，取得焦点时一起读出；它在这一条历史里，刷新、返回时再说一次。
  - 409 `page.revision_mismatch`：重读当前、刷新比较，确认框留着；重读到别的版本号、且它的比较显示出来之后才说明"页面在这期间改过了"，重读失败或版本号没变时说明被拒的原因（不自动重发，13.2 第 7 条）。
  - 409 `page.locked`：照现有的锁的说明（持锁人，`lockedText`），并重读锁。
  - 404：`page.not_found` 时说明页已不在并关闭；别的 404 说明原因并关闭。
  - 别的拒绝留在确认框里。

### 4.6 测试（vitest）

- `lineDiff`：一般的改动、最后一行有没有换行、步数的上限（小的预算）、空的一边、只有头尾的改动；性质测试：随机的文字对，改动应用到 a 得到 b、改动的行数等于按行的最短编辑（取代原写的"与 `@codemirror/merge` 的 `diff` 在小输入上逐行一致"：对照最短同样说明正确与最小，审查 C 记为偏离）；视图的块标出每一行改动的行（审查 A-M1）；一页密集的改动交给视图之后没有可合并的（英文九成的词换字母、中文三成的字加删助词、词里加删字母，审查 A-I1、修复核对第一轮 N2）；B-I1 的形状（3 万字改 8 个词）是 8 处改动；每个上限在它的值上：5,000 与 5,001 行、2,000 与 2,001 个码元、1 MiB 的细分合计（审查 C-F2）。
- Worker 的调用：默认的 2 秒之后 `terminate()` 并答"太大"；正好 5 MiB 与多一个字节；`error`、`messageerror`、起不来时拒绝；中止时结束；小的文字在主线程上（`smallDiff` 的值）。
- `TextDiff`：只差换行或 BOM、相同；标记、摘要与上一处、下一处；并排与摘要；删除的一块 5,000 行统一显示、5,001 行并排；失败与重试（重试时显示"正在比较…"）；新的文字比较时留着上一次的、迟到的答复丢弃、文字变了时中止旧的（经信号断言）。
- `PageVersions`：两种语言；分页与焦点；"更多"与重读的三种竞争（照反链）；读不到与重试；"编辑中"、"当前"；租约过期与离开编辑之后的重读；经组合根到达右栏。
- 版本页：信息与只读的视图；未建的链接；标签页与键盘、两个比较的方向（删的、加的行，审查 C-F1）；焦点时只重读视图；第一个版本没有"与上一版"；地址的替换（视图读一次、之前的键去掉）与标题的版本号；到别的版本时它的基版本照常显示；404 与 `version_not_found`；版本、视图、上一版、当前与比较的分包读不到时说明、按"重试"之后加载（还原的确认框里的当前同样，审查 C-F3）；锁提示与 `readEnded` 的守卫；StrictMode 的重新挂载不重读；离开时去掉缓存。还原：每种答复、只发一次、比较进行中与失败时不能还原、从页面开始时旧的视图不再显示（审查 C-F6）。
- 事件流（经真的处理函数）：`pages` 事件的列表与编辑中的版本（地址随之；"与上一版"开着时基版本留着）、不在编辑的不读、`page-content` 的重读（审查 C-F4）、锁事件、链接与树、连上时；`handlers.test.ts` 核对 `refresher` 的键与"只为读过列表的页"。
- `ConflictDiff` 现有的测试照旧通过。

### 4.7 e2e

- **HI1**（`e2e/stories/history/hi1-versions.spec.ts`）：两次编辑会话之间离开编辑，右栏的版本列表有三个版本，最新的标"当前"；打开旧版本阅读（只读，任务复选框不能勾），文档标题，第一个版本没有"与上一版"；与上一版、与当前比较按标记的行核对方向（审查 C-F1）；接口版本：编辑会话即版本、中间的版本号答它所在的那一行、活着的会话标 `editing`、列表不带正文、正文与写入的逐字节相同（BOM、CRLF、行尾空格）、404 `page.version_not_found`。
- **HI2**（`hi2-version-restore.spec.ts`）：还原到旧版本，正文与索引随之改变（`links.ts` 的断言），版本列表多一个"还原"的版本（数据库：一个 `version_restore` 变更集），焦点在标题，状态行；与当前相同时说明没有改动、焦点回到"还原"；另一个账户在编辑时还原被拒并显示持锁人，什么都没写；确认框的比较按标记的行核对方向；接口版本：PAT 还原、未改动、409 `page.revision_mismatch`、409 `page.locked`（带 `lock`）。
- 数据库断言放在 `e2e/fixtures/assert/page.ts`（13.4 第 3 条按模块）：`expectVersions` 核对每个变更集一行、正文、字节数与哈希、类型、执行者、客户端与 `reverts_id`（P1 里都为空，HI3、HI4 沿用）。夹具：`fixtures/pages.ts` 的 `getVersions`、`getVersion`、`postRestore`，`fixtures/wiki-pages.ts` 的 `versionPath`、`versionsListed`、`restoreDialog`、`restoreWith`。
- 页面版本不断言不存在的版本的 404：连上时的重读与第一次读赛跑，控制台的 404 数不定（接口版本与 vitest 覆盖）。比较的页都小，只走主线程的路径，Worker 的路径没有 e2e（审查 A 的存疑，接受）。
- C4（笔记本被删）的 404 由 8 个变为 10 个：未保存的编辑留住页面时，右栏的版本列表在锁事件与重连时各读一次（反链、属性在重连时已经如此）。

### 4.8 审查之后的改动

三位审查者（A：比较与性能；B：界面、数据流与无障碍；C：测试、e2e 与设计的符合）与修复核对的处置见 `reviews/P1B-versions-web-review.md`。与上文最初的设计不同之处已改在各节里：

- **实施时定下的**（设计没写或留了余地）：版本页是笔记本的外壳之下自己的路由，`PageGone` 与页面的外壳共用；新的 SWR 键 `["page-content", 笔记本 id, 页 id]`（"与当前比较"与还原共用）；换行与 BOM 的规整在 `TextDiff`，小的文字在主线程上比较，调用叫 `compareInWorker`，"同一时刻至多一个"按视图；自己的保存只在离开编辑时立即重读列表，锁事件也重读列表与编辑中的版本，`tree: true` 重读版本的视图（00 号文档 4.13 有，本文原来没写），"编辑中"按同一版本号缓存的正文判断；版本的正文挂载时不重读；"还原"与"当前"随版本列表的读；标签页手动激活、不进地址；信息行也有"当前""编辑中"；状态行是 `<output>`，不是 `useNotice`；还原经按页的 `oneAtATimeById`，不是 `oneAtATime`；性质测试对照最短编辑。
- **审查与修复核对带来的**：细分与合并在 Worker 里做到视图无可合并（A-I1、核对 N2）；插入、删除滑过重复的行（A-M1）；细分的合计只算细分了的（A-M2）；Worker 失败时的"比较失败"与"重试"（A-M3）；新的比较进行时留着上一次的，写着"正在比较…"（A-M4、核对 N1）；删除的一块按视图的块限 5,000 行（A-M5）；视图按答复的版本号读，地址的版本号变了即去掉之前的正文与视图，"编辑中"的规则只重读正文（B-I1）；版本的正文焦点时不重读（B-M1）；租约过期之后重读版本（B-M2）；版本链接的名称（B-M3）；英文的计数写成"Changes: N"（B-M4）；状态行经标题的 `aria-describedby` 读出；"还原"只在显示的比较是它送出的版本号的正文时可用，"在这期间改过了"只在重读到别的版本号、且它的比较显示之后（核对 N1、B 的细节）。

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
- 与设计不同之处已改在第 3 节（3.4–3.6、3.9）：`FindVersion` 一条语句连正文读出、没有 `VersionContent`；`ListVersions` 读 `below` 之下（最新的从 `MaxRevision + 1`）；`RecordRevision` 的版本链守卫与 `checkPages` 的链；`changesetOf` 只接受编辑；`PageVersionPage`、`ChangesetAuthor`（P2A 起是共用的 `UserRef`）；路径的版本号与 `base_revision` 不设 `minimum`。
- 反向对照：用例 28 处、数据库与规则 17 处、核对之后新加 3 处，都被测试抓到。

### 9.2 B：前端（2026-10-13，合并 `df306e3`）

- 提交：实施：比较、Worker 与 `TextDiff` `1d4f252`、`c7fbe9a`，服务、store、右栏、版本页、还原与文字 `101c8d4`，vitest `fe6999a`、`d635bf8`，e2e HI1、HI2 `de2a06e`，C4 的计数 `3ecca93`，`PageVersionPage` `9920cad`（审查时是 P1A 的修复之上的 `70f6620`，之后变基到 `main` 的 `4891f87`）；审查的修复 `c355a11`、`99e1b85`（A）、`b744598`（B）、`7e77b50`、`e7b8010`（C）；修复核对第一轮的修复 `56962f4`、`18f7215`、`0d252e6`。合并 `df306e3`。
- 审查：三位审查者（Opus）。没有高。中高 3 条：交给视图的改动在主线程上还要经 `@codemirror/merge` 平方级的合并，一页密集的改动（简繁转换）在 Worker 答复之后卡 4–11 秒（A-I1）；看着编辑中的版本时，每次保存在缓存里多留一份正文与视图、视图读两次（B-I1）；比较的方向没有测试，三处反过来 vitest 与 e2e 都通过（C-F1）。中低 3 条：插入、删除从空行开始时画晚一行或画不出（A-M1）；Worker 失败时说"太大"或一直"正在比较…"（A-M3）；编辑中的页上"与当前比较"每次重读都变回"正在比较…"、丢焦点（A-M4）。其余是低与细节。修复核对：第一轮中低 2 条（A-M4 的修复带进的回归：还原的确认框留着旧的比较时"还原"可用、送出新的版本号，N1；词里密集的插入与删除仍把合并留给主线程，N2）与细节 2 条；第二轮中低 1 条（永远成功不了的比较让“还原”一直不可用，也不说重新加载能解决，R2-2）与细节；第三轮中低 1 条（修前就有的：按“重试”时焦点掉到 `body`，R3-2）与细节 2 条（去掉的 `.catch` 带进的未处理的拒绝、“再次失败”不复位）；第四轮中低 1 条（交接焦点的目标没有名称，读屏把整段比较读一遍，R4-2）与细节；第五轮中低 1 条（两层同名的 `group`，推到打磨），没有别的发现。逐条见[审查记录](reviews/P1B-versions-web-review.md)。
- 与设计不同之处已改在第 4 节（4.1–4.7），4.8 分实施时定下的与审查带来的列出。
- 反向对照：实施之后 27 个、审查的修复之后 58 个（含 C 存活的 24 个）、核对第一轮的修复之后 20 个，都被测试抓到；手工 2 个（去掉两处发送的守卫；静态导入 `editor/text-diff` 让构建检查失败）；e2e 1 个（三个比较的方向对调），HI1、HI2 的页面版本失败。
- 门禁（本机）：`make lint-web knip test-web build-web`（`0d252e6`，2,652 个测试）；整个 e2e 234 个通过（`e7b8010`）；`stories/history` 连跑三次通过（`0d252e6`）。
- 交给后面的：P2：回收站里的页的版本在 P2 的预览里（版本页只找活着的页），`trash_restore` 的标签已有；P3：`revert` 的标签已有，`expectVersions` 已核对 `reverts_id`，HI3、HI4 沿用；打磨的十余条见审查记录，已写进 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)第 21 条（[收尾审查](reviews/M8-closeout-review.md) C-N6）。
- 负责人可以改判的取舍：同一对文字在两个视图里各比较一次（不做全局的一个）；Worker 的路径没有 e2e；自己的保存只在离开编辑时立即重读版本列表；标签页手动激活、选的标签页不进地址；"已还原到版本 N"随历史在刷新、返回时再说一次。
