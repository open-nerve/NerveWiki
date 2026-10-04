# M6/P3 索引与解析：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P3 索引与解析 |
| 状态 | 进行中 |
| 基线 | P2 合并之后的 main；本文提交之后开分支 `m6-p3a`，A 部分合并之后开 `m6-p3b` |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.3–4.5、4.7（"阅读视图的一致性"）、4.8–4.10、第 7–9 节；[M4/P3 给 M6 的移交](handoffs/M4-P3-markdown-extensions.md)第 8、10、11 项；[M5 的事件移交](handoffs/M5-events.md)第 1–3 项；[M9 的批量移交](../M9-mcp/handoffs/M4-P2-unit-merge.md)第 2 项；样例集 [README](../../../tools/md-fixtures/README.md) |

---

## 0. 分两部分

P3 是 M6 最大的一个 Phase。为了让每次审查的范围可控，它分两部分，各自实现、审查、核对修复、合并：

- **A：解析与索引（服务端）**：`resolve/` 样例与 Obsidian 的核对；`linking` 模块（解析规则、索引表、索引的观察者、按笔记本的锁）；page 的读端口；笔记本删除的注册；`links` 事件；`nervewiki reindex`。
- **B：链接的渲染与跳转**：`Fetch`（链接改为 `<a>`，带解析状态）；Markdown 链接与属性链接的状态（平台的钩子）；锚点的 id；前端的最小增强（`href`、经路由跳转）与 `links` 事件的处理。

第 1–5 节是 A，第 6 节是 B 的设计（B 开始时细化），第 7–9 节两部分共用。

## 1. 基线

作者读代码（2026-10-05，一位 Opus 只读梳理）：

- **树**：`nodes` 有 `name_key`（`shared.TitleKey`：NFC、Unicode 大小写折叠、NFC），唯一索引 `(notebook_id, parent_id, name_key) WHERE deleted_at IS NULL`；没有按 `name_key` 查的查询与索引。`page` 的 store 有 `Ancestors(id)`（递归 CTE）、`Subtree(nb, id)`、`PageContent(id)`，都在调用方的事务里（`postgres.DB(ctx, pool)`）。
- **写入单元的变更**（`domain.Change`，`TreeState` 只有父节点、名称、次序）：
  - 新建：新节点，带 `Revision` 与 `Facts`；
  - 写正文：那一页，前后相同，带 `Revision` 与 `Facts`；相同的正文不记；
  - **改名：只有被改名的那一个**；
  - **移动：被移动的节点，加上它全部活着的后代（前后相同）**，只调次序的移动也一样；
  - **删除：子树里每个活着的节点**，`After` 为空；
  - 观察者在 `tell` 里、提交之前、持着全部锁调用一次。
- **组合根**：`pageRegistrants(pool)` 只凭连接池构造，`pageDeps` 与 `notebookRegistrants` 都调它（后者也到达 `nervewiki users`、`workspaces` 两个命令）。
- **事件**：`Publisher.Publish(ctx, events.Event{Type, WorkspaceID, NotebookID, Data})` 在调用方的事务里 `NOTIFY`；`PagesWritten` 多于 20 页时 `pages` 为 `null`。整个程序的流测试有 `openStream`、`s.expect`、`tm.quiet`。
- **命令行**：`workspaces` 的样板（`bootstrap.Workspaces`、`openAdminCommand`、`printResult`）；archtest `TestCommandsComposeNoServerAndNoJobs` 不许命令的组装调用模块的 `New`、HTTP 服务、限流、后台任务。没有列出全部笔记本的查询。
- **约束**：
  - archtest `TestSQLCSchemaScope`：一个迁移只改它所属模块建的表，所以 `nodes` 上的新索引要写在 page 的迁移里；
  - `purge_test`：指向被清理表（`nodes`、`notebooks`）的外键要求有一个排在前面的清理器，而没有 `deleted_at` 的表不许有清理器。`edit_sessions` 因此不带指向页面表的外键（"会话不比它的页面活得久，清理没有东西可以排序"）。
- **Obsidian 的解析**：在独立的数据目录里探了 1.12.7（三轮共 99 条链接），结果见第 2 节。

## 2. A：解析规则（与 Obsidian 核对之后定稿）

导出的位置照总体设计 3.5：页面 `A` 是 `A.md`，它的子节点在 `A/` 下。一条链接的**出发文件夹**是出发页所在的文件夹，即它的父节点（根下的页是笔记本根），不是它自己的子节点。

**目标的切分**（`domain.ParseTarget`）：

- `./`、`../` 开头的是相对的；`/` 开头的从根起；其余是普通的。
- 按 `/` 切成段，每段算标题键（`shared.TitleKey`）。空段、中间的 `.`、`..` 解析不到（Obsidian 的行为没有核对，nerve-defined）。
- 最后一段以 `.md` 结尾时有两种写法：先去掉 `.md`（`[[x.md]]` 是页面 `x`），再原样（标题为 `x.md` 的页）。没有 `.md` 的只有原样一种。

**次序**（前一步找到就停）：

1. **相对**：从出发文件夹起，`..` 每个上一层（到根为止），再按段往下：节点从根起的标题键路径恰好等于算出的路径，就是它。找不到就解析不到，不往下试。
2. **从根起**：以 `/` 开头的，或者普通的目标恰好是某个节点从根起的路径（单个名称也算：`[[note]]` 先找根下的 `note`），就是它。
3. **名称与路径后缀**：节点的标题键路径以这些段结尾（按整段对齐：`[[ote]]`、`[[A/deep]]` 都不匹配 `A/B/deep`）。多个时：
   - 先选在出发文件夹的整棵子树里的（出发页在根下时，全部都算）；
   - 再选层数少的；
   - 再按 id，并标记歧义（Obsidian 这时没有稳定的规则：`aa/sib` 与 `ab/sib` 从根起选了 `ab`）。
4. **别名**：只对不带 `/` 的普通目标，按页面的别名的标题键匹配，多个时同第 3 步。Obsidian 不解析别名（`[[Al]]` 解析不到），这是总体设计 4.4 有意的差异，样例标 `nerve-defined`。

两种写法（`.md`）在每一步里依次试。只解析到页面；附件（M7）的嵌入解析不到。

**与 M6 总设计 4.4 原文的出入**（总设计与总体设计 4.4 随之修订）：

- "先选同一父节点下的" 改为 "先选在出发文件夹的整棵子树里的"：`[[dup]]` 从 `Y` 下的页出发，选 `Y/Z/dup` 而不是 `X/dup`；`[[B/note]]` 从 `G` 下的页出发，选 `G/A/B/note`。
- 第 2 步对单个名称也成立：`[[note]]` 从 `A` 下的页出发，选根下的 `note`，不选同一文件夹的 `A/note`。
- 不带 `./`、`../` 的 Markdown 链接与 wikilink 同样处理（`[t](note.md)` 从 `A/B` 出发是根下的 `note`），已确认。

**落点**（`domain.Landing`，B 部分渲染 `data-nw-parent` 用）：解析不到的普通单名，落在出发页的父节点下；带路径的，落在路径的父页下（父页要存在，按同样的规则解析）；相对的照相对算；名称不是合法标题的没有落点。

## 3. A：`linking` 模块

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `migrations/sql/00019_page_nodes_name_key.sql` | `nodes (notebook_id, name_key) WHERE deleted_at IS NULL` 的索引（page 的迁移） |
| `migrations/sql/00020_linking_index.sql` | linking 的五张表 |
| `modules/page/adapter/postgres/queries/links.sql`、`page/link_targets.go`（模块根） | page 的读端口 `LinkTargets`：按标题键找节点（带从根起的路径）、一组节点的路径、子树、一个笔记本的页面与正文、重算标题键 |
| `modules/linking/domain/` | `target.go`（切分）、`resolve.go`（解析与落点）、`facts.go`（一页的链接、标签、属性、别名）、`change.go`（受影响的键与节点） |
| `modules/linking/app/` | `ports.go`、`index.go`（观察者）、`resolver.go`（取候选、解析一批链接）、`reindex.go`、`deletion.go` |
| `modules/linking/adapter/postgres/` | 索引表的 store、按笔记本的锁（`pg_advisory_xact_lock`） |
| `modules/linking/adapter/markdown/` | `markdown.Facts` → `domain.PageFacts`（obsidian 的提取结果、frontmatter 的 `aliases`、`tags` 与属性）；reindex 的解析 |
| `modules/linking/` 模块根 | `module.go`（`New`：观察者、笔记本删除的注册者）、`admin.go`（`NewAdmin`：reindex，命令行用） |
| `bootstrap/` | `linking.go`（组装、page 事件与 `links` 事件的转换）、`registrants.go`、`reindex.go` |
| `cmd/nervewiki/reindex.go` | `nervewiki reindex [--notebook <id>]` |
| `tools/md-fixtures/resolve/`、`check.mjs`、`obsidian/verify-resolve.mjs`、`README.md` | 解析样例与核对（解析要每个样例一个库，与提取的单库核对分开写） |
| `deploy/runtime-grants.sql`、`sqlc.yaml`、`migrations/schema_test.go` | 新表 |

### 3.2 表

都是派生数据，没有 `deleted_at`，可以随时重建：

| 表 | 列 | 键与索引 |
|---|---|---|
| `indexed_pages` | `node_id`、`notebook_id`、`revision`、`extractor`、`frontmatter_valid` | 主键 `node_id`；`(notebook_id)` |
| `page_links` | `source_id`、`range_start`、`range_end`、`notebook_id`、`kind`、`property_key`、`target`、`anchor`、`display`、`target_key`、`target_alt_key`、`resolved_id`、`ambiguous` | 主键 `(source_id, range_start)`（一段字节只有一条链接）；`(notebook_id, target_key)`、`(notebook_id, target_alt_key)`、`(resolved_id)` |
| `page_tags` | `source_id`、`notebook_id`、`tag`（第一次出现时的写法）、`tag_key`、`count` | 主键 `(source_id, tag_key)`；`(notebook_id, tag_key)` |
| `page_properties` | `source_id`、`notebook_id`、`position`、`key`、`value`（jsonb） | 主键 `(source_id, position)` |
| `page_aliases` | `source_id`、`notebook_id`、`alias`、`alias_key` | 主键 `(source_id, alias_key)`；`(notebook_id, alias_key)` |

- `target_key` 是目标最后一段的标题键（去掉 `.md` 之后），`target_alt_key` 是带 `.md` 的那一种（没有时为空）。节点出现、消失、改名、移动时，按这两列找可能受影响的链接。
- **不带外键**（与 M6 总设计 4.3 的出入）：指向 `nodes`、`notebooks` 的外键与 `purge_test` 的两条规则冲突（见第 1 节）。照 `edit_sessions` 的先例，索引行由观察者与笔记本删除的注册者在同一个事务里删掉，"只指向活着的节点"由测试的不变式 `checkLinks` 核对。
- `extractor` 是提取规则的版本（`linking/domain` 的常量，从 1 起）；改了提取规则的发布提升它，要求 reindex。

### 3.3 page 的读端口

模块根 `page.NewLinkTargets(pool) LinkTargets`，全部在调用方的事务里，只读活着的节点：

```go
type LinkTargets interface {
	// ByKeys: 笔记本里标题键在 keys 中的节点，各带从根起的路径（id、标题键）。
	ByKeys(ctx, notebookID uuid.UUID, keys []string) ([]LinkNode, error)
	// Paths: 一组节点从根起的路径。
	Paths(ctx, notebookID uuid.UUID, ids []uuid.UUID) ([]LinkNode, error)
	// Subtree: 一个节点与它活着的后代。
	Subtree(ctx, notebookID, id uuid.UUID) ([]LinkNode, error)
	// Pages: 笔记本里每一页的正文与版本，按 id 逐页读（reindex）。
	PageIDs(ctx, notebookID uuid.UUID) ([]uuid.UUID, error)
	Content(ctx, id uuid.UUID) (string, int, error)
	// Rekey: 按当前的 Unicode 数据重算标题键；同一父节点下撞键时不改，答撞键的节点。
	Rekey(ctx, notebookID uuid.UUID) ([][]uuid.UUID, error)
}
```

`LinkNode{ID, Kind, Path []PathStep{ID, Key}}`（路径的最后一步是它自己）。`ByKeys` 是一条递归 CTE：先按 `(notebook_id, name_key)` 找到节点，再沿父节点上溯，层数受 `MaxDepth` 约束。

### 3.4 观察者：增量维护

linking 的 `Index` 实现 `page.PageObserver`，经组合根登记（`pageRegistrants`）。`PagesChanged(ctx, e)`：

1. **跳过**：没有正文写、也没有新建、改名、移动、删除的单元（只调次序的移动，前后的父节点与名称都相同），什么都不做；前后都空的变更（M9 移交第 2 项）同样。
2. **锁**：`pg_advisory_xact_lock`，键由笔记本 id 算出（同一笔记本的索引维护串行，总设计 4.5）。这是单元最后取的锁。
3. **写了正文的页**：按 `Facts` 换掉它的链接、标签、属性、别名与 `indexed_pages` 的行；记下它原来的链接解析到的节点与原来的别名。
4. **删除的节点**：删掉它们作为出发页的全部行。
5. **受影响的标题键与节点**（`domain.Affected`）：
   - 新建：新节点的键；
   - 改名：旧键、新键，以及经端口取的子树里每个节点的键；
   - 移动：列出的每个节点的键（子树已在变更里）；
   - 删除：列出的每个节点的键；
   - 别名变了的页：旧别名与新别名的键。
6. **候选链接**：`target_key` 或 `target_alt_key` 在这些键里的；解析到受影响节点（改名、移动、删除的子树）的；出发页在改名、移动的子树里的（出发文件夹变了）；以及第 3 步写入的全部链接。
7. **解析**（`app.resolver`）：一次切分全部目标，一次 `ByKeys` 取全部候选，一次取别名，一次 `Paths` 取出发页的路径，逐条 `domain.Resolve`。
8. **更新**：只写解析结果真的变了的行。
9. **事件**：`pages` 是链接状态（解析到的节点）变了的出发页，不含本单元写了正文的页；`targets` 是反链变了的页（变了的行的新旧目标，加上写了正文的页的新旧目标）。都空时不发。

**为什么候选是全的**：解析结果只取决于（一）最后一段的标题键对应的节点集合，（二）这些节点从根起的路径，（三）出发页的位置，（四）别名。（一）由新建、删除、改名、移动的键覆盖；（二）只有改名、移动会改，它们的子树全部计入；（三）只有移动会改，子树计入；（四）由正文写的别名覆盖。这是推理，不是证明，所以有"增量等于重建"的性质测试（第 5 节）。

### 3.5 笔记本删除、`links` 事件、组合根

- **笔记本删除**：linking 登记笔记本删除事件（`notebookRegistrants`，排在 page 之后、事件流的 `reset` 之前），在同一个事务里删掉这些笔记本的全部索引行。
- **`links` 事件**：linking 的端口 `Publisher.LinksChanged(ctx, LinksChanged{WorkspaceID, NotebookID, Pages, Targets})`；多于 20 页的一组为 `null`（linking 里先判断，不靠 `ErrTooLong`）。组合根的 `linkEvents` 经 `events.Publisher.Publish` 发出类型 `links`，载荷 `{"pages": […]|null, "targets": […]|null}`；契约 `api/modules/events.yaml` 加这个类型。
- **组合根**：
  - `pageRegistrants(pool)` 加 linking 的观察者（只凭连接池与发布者就能构造：它不解析正文），所以 `users`、`workspaces` 两个命令的笔记本删除也删索引行；
  - page 的 `Event` 经 `bootstrap` 转成 linking 的（两个模块互不导入），`Facts` 经 linking 的模块根 `linking.PageFacts(any)` 转成 linking 的提取结果（实现在 `adapter/markdown`）。

### 3.6 `nervewiki reindex`

- `nervewiki reindex [--notebook <id>]`，照 `workspaces` 的样板经 `bootstrap.Reindex` 组装，不起 HTTP 与后台任务；archtest 的命令清单加上它。
- 列出笔记本：notebook 模块根加 `notebook.NewCatalog(pool)`（活着的笔记本 id 与工作区，按 id）。
- 每个笔记本一个事务：
  1. 笔记本行 `FOR NO KEY UPDATE`（与改名同级），linking 的锁；
  2. `Rekey`：撞键时报告这些节点（标题与 id），这个笔记本不做，命令最后以退出码 1 结束；
  3. 删掉这个笔记本的索引行，逐页取预算、解析、写行（`adapter/markdown` 用平台的 Markdown 与预算）；
  4. 解析全部链接；
  5. 发一条 `links`（`pages: null, targets: null`）。
- 输出：每个笔记本一行（页数、链接数、解析不到的数）；撞键的笔记本在标准错误上列出。
- README 写明：升级到 M6 之后运行一次；reindex 期间，这个笔记本的写在等待。

### 3.7 `resolve/` 样例

```json
{
  "description": "…",
  "source": "obsidian-verified",
  "pages": ["note", "A", "A/note", "A/B", "A/B/note"],
  "aliases": { "A/B": ["Al"] },
  "links": [
    { "from": "A/B/src", "link": "[[note]]", "to": "note" },
    { "from": "src", "link": "[[Al]]", "to": "A/B", "source": "nerve-defined", "note": "…" }
  ]
}
```

- `pages` 是页面从根起的路径（父页要列出）；`from` 是出发页（也在 `pages` 里）；`to` 为 `null` 时解析不到；`ambiguous: true` 时有歧义。
- Go 的测试（`linking/adapter/markdown`）用应用的扩展提取 `link`，建树，逐条 `domain.Resolve`。
- `obsidian/verify-resolve.mjs prepare|check`：每个样例一个库（一个窗口），按 `pages` 建文件夹与文件，每条链接在 `from` 所在的文件夹里放一个只含这条链接的 `qNNN.md`，读 `metadataCache.resolvedLinks`。别名在 frontmatter 里。
- 样例覆盖第 2 节的每一步与 99 条探针里的典型情形。

## 4. A：实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | `resolve/` 样例、`check.mjs` 的格式检查、`verify-resolve.mjs`；与 Obsidian 核对 | `md-fixtures: link resolution cases, checked with Obsidian (M6/P3/S1)` |
| S2 | `linking/domain`：切分与解析；表格测试与样例测试（受影响的键随 S4 的观察者，落点随 B 的渲染，各自与用它的代码一起测） | `linking: the resolution rules (M6/P3/S2)` |
| S3 | 迁移、sqlc、page 的读端口、linking 的 store 与锁 | `page, linking: the index tables and the page's link targets (M6/P3/S3)` |
| S4 | 观察者、笔记本删除、`links` 事件、组合根；整个程序的测试 | `linking, bootstrap: the index follows every write (M6/P3/S4)` |
| S5 | `nervewiki reindex`；性质测试"增量等于重建"；交错 | `linking, cmd: reindex, and the index equals its rebuild (M6/P3/S5)` |

## 5. A：测试与验证

- **解析**：`domain` 的表格测试（每一步、`.md` 的两种写法、`..` 到根为止、空段、歧义、别名在名称之后；落点在 B）；`resolve/` 样例全部通过，`obsidian-verified` 的与 Obsidian 一致。
- **store 与端口**：`ByKeys` 的路径、跨笔记本不串、已删的不算、`Rekey` 的撞键；数据库测试。
- **每条写入路径**（整个程序，`serve` 上）：新建、改名（带子页）、移动（带子页）、删除子树、写正文（含改别名）、勾选任务、笔记本删除（三条路径），各有一个测试：索引行随之更新，`links` 事件到达流（`s.expect(t, "links")`），载荷的 `pages`、`targets` 对；组合根不登记观察者时失败。
- **性质测试**：随机的树与一串操作（新建、改名、移动、删除，带子页；写正文，带链接、别名、相对路径与重名），每步之后增量维护的索引等于 `reindex` 从头重建的结果（同一事务里比较）。
- **不变式** `checkLinks`：`indexed_pages` 的版本等于正文的；`resolved_id` 只指向活着的节点；每个活着的页都有 `indexed_pages`。每个整个程序的测试结束时核对。
- **交错**（`pgtest.WaitForLockWaitsOn`）：两次正文写（一页加别名 `x`，另一页写 `[[x]]`），结果与先后无关；正文写（链接指向 B）与删除 B。
- **reindex**：命令测试（单个笔记本、全部、撞键时报告并退出 1、`--notebook` 不合法）；运行用户的权限下可用（`runtime_role_test`）；发出 `links`。
- **反向对照**：解析的每一步、候选的每一类、锁、事件的退化，各有一个变体要有测试失败。
- `make check`、`make gen-check`、e2e 全量。

## 6. B：链接的渲染与跳转（B 开始时细化）

- **`Fetch`**：`obsidian.Extension(fetch)`，`fetch` 由 linking 给出：一条语句读 `indexed_pages` 与这一页的链接行；`revision` 与 `extractor` 都相同时按字节位置取状态，不同时即时解析（用 A 的 `resolver`）。`Render` 在 `Fetch` 读库期间仍占着预算（P2 审查的提醒；每次至少 4 KiB，P2 第四轮修复核对）。
- **wikilink 与嵌入**：`<a class="nw-wikilink" data-nw-node="…" data-nw-anchor="nw-…">`；解析不到的 `nw-unresolved`、`data-nw-target`、`data-nw-parent`（落点）；链接文字里的 wikilink 仍是 `<span>`（不嵌套 `<a>`）。
- **Markdown 链接与图片**：平台加一个钩子，扩展按目标的范围给核心渲染的 `<a>` 加属性（`data-nw-node`、`nw-unresolved`）；`Document` 保留目标范围的旁表到渲染。
- **属性表**：平台加写属性值的钩子，属性链接同样渲染为链接。
- **锚点**：平台导出标题 id 的函数（`markdown.HeadingID`），`data-nw-anchor` 与标题的 id 一致（去重的后缀算不出，指向第一个）。
- **前端**：增强给 `a[data-nw-node]` 设应用内的地址（`/{slug}/notebooks/{nb}/pages/{id}#…`），普通点击经路由跳转，修饰键与中键照浏览器；`eventHandlers` 加 `links`（重读 `pages` 里各页的阅读视图，`null` 时重读这个笔记本的全部）；`refreshedOnConnect` 不变（阅读视图已在其中）。标签仍是 `<span>`，P6 改为链接。
- **测试**：渲染的表格测试与 `CheckHTML`；`Fetch` 的版本一致与不一致；vitest 的增强与事件处理；e2e：wikilink 跳转、解析不到的显示为未建（L1 的一半，改名之后仍跳到那一页在 P4）。

## 7. 与 M6 总设计的出入（本文定稿，总设计随 A 的合并修订）

- 解析规则：第 2 节的三处。
- 索引表不带外键：第 3.2 节。
- 迁移编号：page 的索引是 00019，linking 的表是 00020（总设计写的是 00019）。
- P3 分 A、B 两部分合并。

## 8. 完成标准

- A：第 5 节的测试全部通过；`resolve/` 的 `obsidian-verified` 样例与 Obsidian 一致；`make check`、`make gen-check`、e2e 全量通过；审查的发现处理完，修复经 Opus 核对。
- B：第 6 节的测试通过，同样的流程。

## 9. 结果

（各部分完成后填写）
