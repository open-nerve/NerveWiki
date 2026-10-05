# M6/P3 索引与解析：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P3 索引与解析 |
| 状态 | 已完成（2026-10-05，A 合并 `b802987`，B 合并 `23ce06e`） |
| 基线 | P2 合并之后的 main；本文提交之后开分支 `m6-p3a`，A 部分合并之后开 `m6-p3b` |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.3–4.5、4.7（"阅读视图的一致性"）、4.8–4.10、第 7–9 节；[M4/P3 给 M6 的移交](handoffs/M4-P3-markdown-extensions.md)第 8、10、11 项；[M5 的事件移交](handoffs/M5-events.md)第 1–3 项；[M9 的批量移交](../M9-mcp/handoffs/M4-P2-unit-merge.md)第 2 项；样例集 [README](../../../tools/md-fixtures/README.md) |

---

## 0. 分两部分

P3 是 M6 最大的一个 Phase。为了让每次审查的范围可控，它分两部分，各自实现、审查、核对修复、合并：

- **A：解析与索引（服务端）**：`resolve/` 样例与 Obsidian 的核对；`linking` 模块（解析规则、索引表、索引的观察者、按笔记本的锁）；page 的读端口；笔记本删除的注册；`links` 事件；`nervewiki reindex`。
- **B：链接的渲染与跳转**：`Fetch`（链接改为 `<a>`，带解析状态）；Markdown 链接与属性链接的状态（平台的钩子）；锚点的 id；前端的最小增强（`href`、经路由跳转）与 `links` 事件的处理。

第 1–5 节是 A，第 6 节是 B，第 7–9 节两部分共用。

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
- 最后一段以 `.md` 结尾（不分大小写，Obsidian 把整个目标转成小写）时有两种写法：去掉 `.md`（`[[x.md]]` 是页面 `x`），或原样（标题为 `x.md` 的页）。没有 `.md` 的只有原样一种。
- **只读作一种**（P3A 审查 H3，照 Obsidian 的 `getLinkpathDest`）：笔记本里任何地方有标题键等于去掉 `.md` 的那个名称的页时，读作去掉 `.md` 的；没有时读作原样。之后每一步只用这一种：`[[x.md]]` 在有 `A/x` 与根下的 `x.md` 时解析到 `A/x`；`[[B/x.md]]` 在有根下的 `x` 时解析不到，哪怕有 `B/x.md`。

**次序**（前一步找到就停）：

1. **相对**：从出发文件夹起，`..` 每个上一层（到根为止），再按段往下：节点从根起的标题键路径恰好等于算出的路径，就是它。找不到就解析不到，不往下试。
2. **从根起**：以 `/` 开头的，或者普通的目标恰好是某个节点从根起的路径（单个名称也算：`[[note]]` 先找根下的 `note`），就是它。
3. **名称与路径后缀**：节点的标题键路径以这些段结尾（按整段对齐：`[[ote]]`、`[[A/deep]]` 都不匹配 `A/B/deep`）。多个时：
   - 先选在出发文件夹的整棵子树里的，文件夹自己的页也算（出发页在根下时，全部都算）：导出时页 `A` 是 `A.md`，与它的文件夹 `A/` 并排，Obsidian 按字符串比较也把 `A.md` 算在 `A` 里。从 `Docs/API/Overview` 出发的 `[[API]]` 是 `Docs/API`，不是更深的 `Docs/API/v2/API`；从 `P/q/q` 出发的 `[[q]]` 是 `P/q`，不是出发页自己（P3A 修复核对第一轮）。没有正文、只有子页的页导出时只有文件夹、没有 `.md`（总体设计 3.5），Obsidian 里没有它这个文件，这一条与第 2 步解析到它的链接在 Obsidian 里会解析到别处；导出（M7）是否给被链接的这种页写一个空的 `.md`，在 M6 收尾时移交 M7；
   - 再选路径短的：导出路径（`A/B/x.md`）的字符数，按 JavaScript 的 `String.length` 计（UTF-16 码元：`é` 一个，`😀` 两个），与层数无关（Obsidian 按 `path.length` 排序；P3A 审查 H3）。`Longfoldername/x` 与 `a/b/x` 选后者；
   - 再按 id，并标记歧义（Obsidian 这时没有稳定的规则：`aa/sib` 与 `ab/sib` 从根起选了 `ab`）。
4. **别名**：只对不带 `/` 的普通目标，按页面的别名的标题键匹配，以 `.md` 结尾的先按去掉 `.md` 的键、再按原样的键，多个时同第 3 步。Obsidian 不解析别名（`[[Al]]` 解析不到），这是总体设计 4.4 有意的差异，样例标 `nerve-defined`。

只解析到页面；附件（M7）的嵌入解析不到。M7 加附件时，Obsidian 的规则是"最后一段带 `.` 的，先找文件名恰好如此的文件，没有再找加上 `.md` 的页"，附件照此并入"只读作一种"。

**Obsidian 按字符串比较的三处，这里按整段**（P3A 审查，样例 015，`nerve-defined`）：Obsidian 用小写路径的 `startsWith`、`endsWith` 比较，`YZ/Q/dup` 算在 `Y` 的子树里，`XB/item` 以 `B/item` 结尾；相对路径找不到时，它再按算出的路径找后缀（`./leaf` 从 `C` 下找到 `G/C/leaf`）。这些是字符串比较的副作用，Obsidian 生成的链接（最短的、能唯一解析的写法）不依赖它们。

**与 M6 总设计 4.4 原文的出入**（总设计与总体设计 4.4 随之修订）：

- "再选层数少的" 改为 "再选路径短的（字符数）"；以 `.md` 结尾的目标在整个笔记本范围内只读作一种（P3A 审查 H3，修复时与 Obsidian 再次核对）。
- "先选同一父节点下的" 改为 "先选在出发文件夹的整棵子树里的"：`[[dup]]` 从 `Y` 下的页出发，选 `Y/Z/dup` 而不是 `X/dup`；`[[B/note]]` 从 `G` 下的页出发，选 `G/A/B/note`。
- 第 2 步对单个名称也成立：`[[note]]` 从 `A` 下的页出发，选根下的 `note`，不选同一文件夹的 `A/note`。
- 不带 `./`、`../` 的 Markdown 链接与 wikilink 同样处理（`[t](note.md)` 从 `A/B` 出发是根下的 `note`），已确认。

**落点**（`domain.Landing`，P6 点击新建时用；原定 B 渲染 `data-nw-parent`，见 6.1）：解析不到的普通单名，落在出发页的父节点下；带路径的，落在路径的父页下（父页要存在，按同样的规则解析）；相对的照相对算；名称不是合法标题的没有落点。

## 3. A：`linking` 模块

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `migrations/sql/00019_page_nodes_name_key.sql` | `nodes (notebook_id, name_key) WHERE deleted_at IS NULL` 的索引（page 的迁移） |
| `migrations/sql/00020`–`00024_linking_*.sql` | linking 的五张表，照约定一张表一个迁移 |
| `modules/page/adapter/postgres/queries/links.sql`、`page/link_targets.go`（模块根） | page 的读端口 `LinkTargets`：按标题键找节点（带从根起的路径）、一组节点的路径、子树、一个笔记本的页面与正文、重算标题键 |
| `modules/linking/domain/` | `target.go`（切分）、`resolve.go`（解析；落点在 P6）、`facts.go`（一页的链接、标签、属性、别名）、`change.go`（受影响的键与节点） |
| `modules/linking/app/` | `ports.go`、`index.go`（观察者）、`resolver.go`（取候选、解析一批链接）、`reindex.go`、`deletion.go` |
| `modules/linking/adapter/postgres/` | 索引表的 store、按笔记本的锁（`pg_advisory_xact_lock`） |
| `modules/linking/adapter/markdown/` | `PageFacts`：`markdown.Facts` → `domain.Facts`（obsidian 的提取结果、frontmatter 的 `aliases`、`tags` 与属性，见 3.4 末）；`Parser`：reindex 在预算之内的解析 |
| `modules/linking/` 模块根 | `module.go`（`NewIndex`：观察者；`NewNotebookDeletion`；`PageFacts`）、`admin.go`（`NewAdmin`：reindex，命令行用）。观察者不叫 `New`：archtest 把模块根的 `New` 当作它的 HTTP 端（命令行的组装不许构造它），P4 的接口用这个名字 |
| `bootstrap/` | `linking.go`（page 事件、读端口与 `links` 事件的转换）、`registrants.go`、`reindex.go` |
| `modules/notebook/catalog.go` | `notebook.NewCatalog(pool)`：活着的笔记本的 id，按 id（reindex 逐个重建） |
| `platform/markdown/obsidian`、`platform/postgres/pgtest` | `obsidian.CountedTag`（Obsidian 标签面板计入的标签，索引的标签用）；`pgtest.WaitForAdvisoryLockWaits`（交错测试按索引的锁排先后） |
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
- **键的上限**（P3A 审查 H2）：标题键长于 `domain.MaxKey`（1024 字节）的不记：这条链接的键为空（解析不到，任何标题的键都到不了这么长：255 字节的标题，键至多约 510 字节，`TestNoTitlesKeyComesNearMaxKey` 核对），这样的别名、标签不记。B-tree 的一项至多约 2.7 KB，超过的写入会失败。
- `property_key` 为空是正文里的链接。YAML 键为空串的属性链接，它的属性路径也是空串，同样记为空：P4 的改写按链接的范围是否在 frontmatter 里区分两者（它本来就在单元里重新解析要改写的页），不靠这一列（P3A 审查）。
- **U+0000**：Markdown 链接的 `%00` 与 YAML 字符串的转义会写出 U+0000，PostgreSQL 的 `text` 与 `jsonb` 都不收；`PageFacts` 把每个事实里的它记作 U+FFFD（P3A 审查 H1：否则这样的正文保存答 500，reindex 也停在这一页）。
- **不带外键**（与 M6 总设计 4.3 的出入）：指向 `nodes`、`notebooks` 的外键与 `purge_test` 的两条规则冲突（见第 1 节）。照 `edit_sessions` 的先例，索引行由观察者与笔记本删除的注册者在同一个事务里删掉，"只指向活着的节点"由测试的不变式 `checkLinks` 核对。
- `extractor` 是提取规则的版本（`linking/domain` 的常量，从 1 起）；改了提取规则的发布提升它，要求 reindex。

### 3.3 page 的读端口

模块根 `page.NewLinkTargets(pool) LinkTargets`，全部在调用方的事务里，只读活着的页（附件与已删的页都不在其中）：

```go
type LinkTargets interface {
	// ByKeys: 笔记本里标题键在 keys 中的页，各带从根起的路径（id、标题键、名称）。
	ByKeys(ctx, notebookID uuid.UUID, keys []string) ([]LinkNode, error)
	// Paths: 一组页从根起的路径。
	Paths(ctx, notebookID uuid.UUID, ids []uuid.UUID) ([]LinkNode, error)
	// Subtree: 一页与它下面的页（id、标题键、名称）。
	Subtree(ctx, notebookID, id uuid.UUID) ([]LinkStep, error)
	// reindex 用：笔记本的页（按 id）；一页的正文与版本；
	PageIDs(ctx, notebookID uuid.UUID) ([]uuid.UUID, error)
	Content(ctx, id uuid.UUID) (string, int, error)
	// Rekey: 按当前的 Unicode 数据重算全部活着的节点的标题键；同一父节点下会撞键时一个也不改，答撞键的节点（每组按 id）。
	Rekey(ctx, notebookID uuid.UUID) ([][]NamedNode, error)
}
```

`LinkNode{ID, Path []LinkStep{ID, Key, Name}}`（路径的最后一步是它自己；名称给第 3 步的路径长度）。`ByKeys`、`Paths` 是递归 CTE：先找到页，再沿父节点上溯，上限 64 层，遇到已删的节点就停；路径到不了根（只有缺陷造成的环、或活着的页挂在已删的页下）是错误，不答截断的路径。`Rekey` 的规则在 `page/domain`（`domain.Rekey`），写回只改 `name_key`（派生的列：不动 `updated_at`，不记变更集），分两条语句：先把要改的键改成各自独有的值（`chr(1) || id`，标题不含控制字符），再改成新键，因为兄弟间的唯一索引逐行检查，一个键接过另一个的旧键会在那个键让出之前撞上（P3A 审查）。

### 3.4 观察者：增量维护

linking 的 `Index` 实现 `page.PageObserver`，经组合根登记（`pageRegistrants`）。`PagesChanged(ctx, e)`：

1. **跳过**：没有正文写、也没有新建、改名、移动、删除的单元（只调次序的移动，前后的父节点与名称都相同；改名到同一个键、同样长度的名称），什么都不做；前后都空的变更（M9 移交第 2 项）同样。改名到同一个键而长度不同的（`Straße` → `STRASSE`）算改名：它的子树里的路径长度变了。
2. **锁**：`pg_advisory_xact_lock`，键由笔记本 id 算出（同一笔记本的索引维护串行，总设计 4.5）。这是单元最后取的锁。
3. **写了正文的页**：按 `Facts` 换掉它的链接、标签、属性、别名与 `indexed_pages` 的行；记下它原来的链接解析到的节点与原来的别名。
4. **删除的节点**：删掉它们作为出发页的全部行。
5. **受影响的标题键与节点**（`domain.Affected`）。只写正文、不移动任何节点的单元，只到它写的页自己的链接，与它增删的别名的键（两次的对称差）：正文写不改任何页的位置、名称与路径，到它的链接与按它没变的别名的链接都不变（P3A 审查：此前每次自动保存都重新解析这一页的全部反链，5,000 条反链约 20 ms，都在索引的锁里）。有移动的单元（M9 的批量里可能同时有正文写）照下面的全部：
   - 新建：新节点的键；
   - 改名：旧键、新键，以及经端口取的子树里每个节点的键；
   - 移动：列出的每个节点的键（子树已在变更里）；
   - 删除：列出的每个节点的键；
   - 别名变了的页：增删的别名的键；
   - 以上触及的每一页现有别名的键（实现时由性质测试发现：别名之间同样按路径取舍，移动一页会改变按它的别名解析的结果，哪怕它的别名没变）。
6. **候选链接**：`target_key` 或 `target_alt_key` 在这些键里的；解析到受影响节点（改名、移动、删除的子树）的；出发页在改名、移动的子树里的（出发文件夹变了）；以及第 3 步写入的全部链接。
7. **解析**（`app.resolver`）：一次切分全部目标，一次 `ByKeys` 取全部候选，一次取别名，一次 `Paths` 取出发页的路径，逐条 `domain.Resolve`。
8. **更新**：只写解析结果真的变了的行。
9. **事件**：`pages` 是链接状态（解析到的节点）变了的出发页，不含本单元写了正文的页；`targets` 是反链变了的页（变了的行的新旧目标，加上写了正文的页的新旧目标）。都空时不发；是否发按截断之前的两组判断（P3A 审查：两组都多于 20 时曾经不发）。

**为什么候选是全的**：解析结果只取决于（一）最后一段的标题键对应的节点集合，（二）这些节点从根起的路径（键与名称：长度看名称），（三）出发页的位置，（四）别名与有别名的页的路径。（一）由新建、删除、改名、移动的键覆盖；（二）只有改名（含同键不同长度）、移动会改，它们的子树全部计入；（三）只有移动会改，子树计入；（四）由正文写的新旧别名，以及路径变了的页的别名覆盖。这是推理，不是证明，所以有"增量等于重建"的性质测试（第 5 节）；它在第一次运行时就找到了（四）原先漏掉的后半句。

**frontmatter 的别名与标签**照 Obsidian 1.12.7 自己的代码读（`parseFrontMatterAliases`、`parseFrontMatterTags`、`getAllTags`，从它安装包里的代码读出，只读）：第一个名为 `aliases`、`tags` 的键（ASCII 字母不分大小写，同 JavaScript 不带 `u` 的 `/i`），字符串是一个（不按逗号拆），列表取其中的字符串，按 JavaScript 的 `trim` 去掉首尾空白，空的不算；`alias`、`tag` 不读；第一个键没有字符串就没有，不往后找。标签（frontmatter 的去掉开头的一个 `#`，正文的也一样）照 Obsidian 标签面板的 `getTags` 计（`obsidian.CountedTag`）：去掉结尾的一个 `/`，不空、不全是 ASCII 数字，不含 JavaScript `\s` 的空白、通用与补充标点（U+2000–U+206F、U+2E00–U+2E7F）和 `-`、`_`、`/` 以外的 ASCII 标点（P3A 审查：此前写的"规则 9"是正文标签的提取规则，面板不用它，`a😀`、`a→b` 面板计入、规则 9 不计）；frontmatter 的标签排在正文的之前。Obsidian 不按别名解析（样例 012），所以只有别名的读法照它。

### 3.5 笔记本删除、`links` 事件、组合根

- **笔记本删除**：linking 登记笔记本删除事件（`notebookRegistrants`，排在 page 之后、事件流的 `reset` 之前），在同一个事务里删掉这些笔记本的全部索引行。
- **`links` 事件**：linking 的端口 `Publisher.LinksChanged(ctx, LinksChanged{WorkspaceID, NotebookID, Pages, Targets})`；多于 20 页的一组为 `null`（linking 里先判断，不靠 `ErrTooLong`）。组合根的 `linkEvents` 经 `events.Publisher.Publish` 发出类型 `links`，载荷 `{"pages": […]|null, "targets": […]|null}`；契约 `api/modules/events.yaml` 加这个类型。
- **组合根**：
  - `pageRegistrants(pool)` 加 linking 的观察者（只凭连接池与发布者就能构造：它不解析正文），排在事件流之后（一个单元的 `pages` 帧先于 `links` 帧），所以 `users`、`workspaces` 两个命令的笔记本删除也删索引行；
  - page 的 `Event` 经 `bootstrap` 转成 linking 的（两个模块互不导入），`Facts` 经 linking 的模块根 `linking.PageFacts(any)` 转成 linking 的提取结果（实现在 `adapter/markdown`）。

### 3.6 `nervewiki reindex`

- `nervewiki reindex [--notebook <id>]`，照 `workspaces` 的样板经 `bootstrap.Reindex` 组装，不起 HTTP 与后台任务；Markdown 与预算同 `serve`（`parsing`）；archtest 的命令组装规则加上它。
- 列出笔记本：`notebook.NewCatalog(pool)`（活着的笔记本 id，按 id）；锁与工作区：`notebook.NewNotebooks`（`WorkspaceOf`、`LockByID`）。
- 每个笔记本一个事务（`linking` 的 `app.Rebuild`）：
  1. 笔记本行 `FOR NO KEY UPDATE`（与改名同级），linking 的锁；
  2. `Rekey`：会撞键时这个笔记本什么也不改，答撞键的节点；
  3. 删掉这个笔记本的索引行，逐页取正文、在预算之内解析、写行（只插入，`Store.AddPage`：删过之后不再逐页删，P3A 审查测得一万页的笔记本从 17 秒降到 11 秒）；
  4. 解析全部链接；
  5. 发一条 `links`（`pages: null, targets: null`）。
- 输出：每个笔记本一行（`notebook <id>: N pages, M links, K unresolved`，单数时 `1 page`）；撞键的笔记本在标准错误上列出（标题与 id），别的原因重建失败的连同原因列出（P3A 审查：此前第一个失败就停下，后面的笔记本都不重建），其余照常，命令最后以退出码 1 结束；`--notebook` 不是 id（零 id、空串也算）或笔记本不存在时立即失败；列出之后被删除的笔记本跳过。
- README 写明：升级到 M6 之后、启动服务之前运行一次（Docker 的写法）；reindex 期间，这个笔记本的写在等待，大的笔记本等不到的保存答 500。

### 3.7 `resolve/` 样例

```json
{
  "description": "…",
  "source": "obsidian-verified",
  "pages": ["note", "A", "A/note", "A/B", "A/B/note", "src", "A/B/src"],
  "aliases": { "A/B": ["Al"] },
  "links": [
    { "from": "A/B/src", "link": "[[note]]", "to": "note" },
    { "from": "src", "link": "[[Al]]", "to": "A/B", "source": "nerve-defined" }
  ],
  "note": "…"
}
```

- 键只有这些（`check.mjs` 逐个核对，P3A 审查）；`pages` 是页面从根起的路径（父页要列出，每段是服务端收的标题，兄弟不撞键）；`from` 是出发页（也在 `pages` 里）；`to` 为 `null` 时解析不到；`ambiguous: true` 时有歧义。
- Go 的测试（`linking/domain/resolve_cases_test.go`）用应用的扩展（任务项与方言）提取 `link`，建树，逐条 `domain.Resolve`。
- `obsidian/verify-resolve.mjs prepare|check`：每个样例一个库（一个窗口），按 `pages` 建文件夹与文件，每条链接在 `from` 所在的文件夹里放一个只含这条链接的 `qNNN.md`，等每个文件都有了 `resolvedLinks` 与 `unresolvedLinks`，要求每个文件恰好一条链接，再比较。别名在 frontmatter 里。
- 样例覆盖第 2 节的每一步与 99 条探针里的典型情形。

## 4. A：实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | `resolve/` 样例、`check.mjs` 的格式检查、`verify-resolve.mjs`；与 Obsidian 核对 | `md-fixtures: link resolution cases, checked with Obsidian (M6/P3/S1)` |
| S2 | `linking/domain`：切分与解析；表格测试与样例测试（受影响的键随 S4 的观察者，落点随 P6 的新建，各自与用它的代码一起测） | `linking: the resolution rules (M6/P3/S2)` |
| S3 | 迁移、sqlc、page 的读端口、linking 的 store 与锁 | `page, linking: the index tables and the page's link targets (M6/P3/S3)` |
| S4 | 观察者、笔记本删除、`links` 事件、组合根；整个程序的测试 | `linking, bootstrap: the index follows every write (M6/P3/S4)` |
| S5 | `nervewiki reindex`；性质测试"增量等于重建"；交错 | `linking, cmd: reindex, and the index equals its rebuild (M6/P3/S5)` |

## 5. A：测试与验证

- **解析**：`domain` 的表格测试（每一步、`.md` 的两种写法、`..` 到根为止、空段、歧义、别名在名称之后；落点在 P6）；`resolve/` 样例全部通过，`obsidian-verified` 的与 Obsidian 一致。
- **store 与端口**：`ByKeys` 的路径、跨笔记本不串、已删的不算、`Rekey` 的撞键；数据库测试。
- **每条写入路径**（整个程序，`serve` 上）：新建、改名（带子页）、移动（带子页）、删除子树、写正文（含改别名）、勾选任务、笔记本删除（三条路径），各有一个测试：索引行随之更新，`links` 事件到达流（`s.expect(t, "links")`），载荷的 `pages`、`targets` 对；组合根不登记观察者时失败。
- **性质测试**：随机的树与一串操作（新建、改名、移动、删除，带子页；写正文，带链接、别名、相对路径与重名），每步之后增量维护的索引等于 `reindex` 从头重建的结果（同一事务里比较）。
- **性质测试**（app 层，替身之上，P3A 审查加）：300 个种子 × 40 步，每个单元一到三个写，按节点合并（M9 的批量），每步之后增量等于重建，约 1 秒。
- **不变式** `checkLinks`：每个活着的页都有 `indexed_pages`，版本等于正文的、提取规则是当前的；索引行只属于活着的页、它自己的笔记本；`resolved_id` 只指向同一笔记本里活着的页。随 `checkPages` 核对（45 处）。
- **交错**：两次正文写（一页加别名 `x`，另一页写 `[[x]]`），按索引的锁排先后（`pgtest.WaitForAdvisoryLockWaits`），结果与先后无关；正文写（链接指向 B）与删除 B（笔记本行）；reindex 排在一个正文写之后（正文写持笔记本行的 `FOR SHARE`、等在正文行上），reindex 要先等笔记本行、再取索引的锁，反过来就死锁（P3A 审查加）。
- **任何正文都能保存**：`%00`、YAML 的 `\0`、长于上限的目标、别名与标签（P3A 审查加）。
- **reindex**：命令测试（单个笔记本、全部、撞键与失败时原样保留、不发事件、报告并退出 1、`--notebook` 不合法）；运行用户的权限下可用（`runtime_role_test`）；发出 `links`。
- **反向对照**：解析的每一步、候选的每一类、锁、事件的退化，各有一个变体要有测试失败。
- `make check`、`make gen-check`、e2e 全量。

## 6. B：链接的渲染与跳转

### 6.1 取舍（梳理之后定）

作者读代码（2026-10-05，一位 Opus 只读梳理）之后，B 的范围与总设计 4.7–4.9 有几处出入：

- **地址仍由前端给**（总设计 4.9）：服务端不写笔记本里的链接的 `href`，只写状态。
  - 解析到的写 `data-nw-node`，由增强给出应用内的地址；解析不到的没有地址。
  - 提取到的 Markdown 链接与图片也一样：现在写的相对地址（`note.md`）在应用里落到 404。
  - 外部地址、只有锚点的（`#h`）、自动链接照旧写 `href`。
- **根路径是笔记本里的路径**：`[x](/slug/notebooks/…)` 照规则 8 是笔记本里的路径，解析不到时显示为未建。本站的完整地址（`https://…`）不是笔记本里的链接，照旧是 `<a href>`。
- **推到 P6**：
  - **落点（`data-nw-parent`）与点击新建**。B 不新建页面，落点没有使用者。而且落点变了（路径的父页新建或删除、来源页移动）不改任何解析，不发 `links` 事件，现在算出来会过时。P6 在点击时问服务端，第 2 节的 `domain.Landing` 一起移过去。
  - **属性表里的链接**。要平台写属性值的钩子，还要先解决属性路径的歧义：`{"a.b": …, a: {b: …}}` 两个都是 `a.b`。右栏（P5）先让属性链接可以点。
  - **本站完整地址经路由跳转**（[M4/P3 移交](handoffs/M4-P3-markdown-extensions.md)第 11 项）。P6 按路由表判断哪些是应用的页面。
- **`Fetch` 先读索引**：
  - 版本与提取规则都与这次渲染一致时，一条语句按主键取到这一页全部链接的状态。
  - 不一致、没有索引（M6 之前的页，`reindex` 之前）或缺某条链接时，即时解析。
  - 即时解析不在事务里。其间页被删不算错误：那一页的链接解析不到，那个别名不算。下一次事件会重读。
  - 即时解析的几条语句（候选、别名、路径）各读一个快照：其间移动或改名的页，可能让一条链接这一次解析到旧处或解析不到，下一次重读更正。不放进一个事务（P3B 审查 R2 L2，接受）。
  - 一页里写到同一目标的链接只解析一次：按目标的解析结果（相对、根路径、上几层、各段的键）算同一个，大小写与写法不同也是（修复核对）。每 4096 条看一次上下文；没有一个目标是路径时什么也不读（P3B 审查 R2 M1：一页写十万次同一条链接、一千个同名页时原来要 5 秒）。成本测试在不带竞态检测的构建里跑（`make test-go`）。
  - 不在索引里的页（升级之后、`reindex` 之前）不会出现在 `links` 事件里，它的阅读视图只随别的重读更新；契约写明（P3B 审查 R2 L1）。
- **歧义照解析到的渲染**：只有歧义变了时，A 不发事件，渲染也不区分。

### 6.2 标记

| 链接 | 解析到 | 解析不到 |
|---|---|---|
| wikilink `[[x#h\|y]]` | `<a class="nw-wikilink" data-nw-node="ID" data-nw-anchor="nw-h">y</a>` | `<a class="nw-wikilink nw-unresolved" data-nw-target="x">y</a>` |
| 嵌入 `![[x]]` | 同上，class 加 `nw-embed` | 同上 |
| 只有锚点 `[[#h]]` | `<a class="nw-wikilink" href="#nw-h">h</a>`：同一页，照脚注，不经 `Fetch` | — |
| 链接文字里的 wikilink | `<span class="nw-wikilink">y</span>`：不嵌套 `<a>`，没有状态 | 同左 |
| Markdown 链接 `[t](x.md#h "T")` | `<a data-nw-node="ID" data-nw-anchor="nw-h" title="T">t</a>` | `<a class="nw-unresolved" data-nw-target="x.md" title="T">t</a>` |
| Markdown 图片 `![a](x.png)` | `<span class="nw-image">a <a data-nw-node="ID">x.png</a></span>` | 里面的 `<a>` 是 `class="nw-unresolved" data-nw-target="x.png"` |

- `data-nw-target` 是提取结果的 `Target`：wikilink 去掉首尾空白的，Markdown 链接解码之后的。
- 链接里的图片照旧没有里面的 `<a>`；外部地址、`#h`、自动链接照旧；标签照旧是 `<span class="nw-tag">`，P6 改为链接。
- **一个链接里没有链接**（P3B 审查 R1 L1）：
  - 用户写的 `<a>` 在 Markdown 链接里、在另一个用户写的 `<a>` 里，或者它的结束标签之前（同一个范围里）有渲染为链接的节点时，丢掉这个标签，留下里面的内容。渲染为链接的节点是 Markdown 链接、自动链接、图片、脚注的引用与返回链接，以及扩展的 `Linker`（wikilink、嵌入）。被丢弃的元素（`<script>`、`<textarea>` 等）里的 `</a>` 不结束用户的 `<a>`（修复核对）。
  - 链接里的自动链接只写它的文字，脚注引用只写 `<sup>`。
  - `CheckHTML` 拒绝 `<a>` 套 `<a>`。
- Markdown 链接的地址不安全、被去掉时，它里面的一切照在链接里渲染：wikilink 是 `<span>`，自动链接只写文字，脚注引用只写 `<sup>`，图片没有里面的 `<a>`（P3B 审查 R1 L4 与修复核对，接受，写在代码的注释里）。
- **锚点**：`data-nw-anchor` 是 `markdown.HeadingID(锚点的最后一段)`。
  - `HeadingID` 与标题的 id 是同一个函数：`nw-` 加 slug。
  - `H1#H2` 取 `H2`。空的，或以 `^` 开头的块引用（v0.1 没有块的 id），不带锚点；只有这样一个锚点的 wikilink（`[[#^b]]`）是没有状态的 `<span>`。
  - 去重的后缀算不出，所以指向同名标题里的第一个。
  - 标题里有链接或格式时，可能对不上：标题的 id 来自显示的文字，锚点来自写下的文字。

### 6.3 平台（`platform/markdown`）

- **钩子**：`Attr{Name, Value}`，`Extension` 加一项：

  ```go
  Links func(data any) func(start int) ([]Attr, bool)
  ```

  - 给定 `Fetch` 的结果，按目标在正文里的起点（`Tree.Destination`，即提取结果的 `Range.Start`）答出一个 Markdown 链接或图片里面的 `<a>` 带的属性。
  - 答 true 时，这些属性代替地址，`title` 照旧；答 false 时地址照旧。
  - 几个扩展都答时，取注册在前的那个。
  - 值由平台转义（`WriteAttrs`）：名称只能是小写字母、数字与 `-`，`href`、`src` 经 `SafeURL`，不合的不写（P3B 审查 R1 L2）。属性名要写进扩展的 `Markup`。
- **`Document` 留着目标的位置表**（`harden.Destinations`）到渲染。它不进 `Facts`，所以写入路径不受影响。
- **`marks` 拿到位置表与钩子**：链接进入与离开时判断一致；图片里面的 `<a>` 同样处理。
- **导出 `HeadingID(text string) string`**。
- **`Linker`**（与 `Hider` 并列）：扩展渲染为链接的节点实现它（`RendersLink()`），净化据此丢掉围着它的用户的 `<a>`。在 Markdown 链接的文字里它必须不渲染链接：平台只丢用户写的 `<a>`。P6 把标签改为链接时，标签的节点也要实现它。
- 没有扩展给 `Links` 时，输出逐字节不变（现有的 `render_test` 不改）。

### 6.4 obsidian 扩展

- **入口**：`Extension(Options)`，`Options{Resolve Resolve}`；M7 的附件在这里加一项。

  ```go
  type Resolve func(ctx context.Context, page markdown.Page, links []Link) (map[int]uuid.UUID, error)
  ```

  - 答出每条链接按 `Range.Start` 解析到的页，不在表里的是解析不到。
  - 类型只用平台的类型（archtest 的 `platformIsBusinessFree`、`markdownLibrariesStayInMarkdown`）。
- **`Fetch`**：
  - 没有链接，或 `Resolve` 为 nil 时，不读任何东西。
  - 否则把全部提取结果交给 `Resolve`。属性链接也在其中，P6 用。
  - 结果按起点记着：解析到的页，以及那条链接（目标、锚点）。
- **渲染**：
  - wikilink 与嵌入照 6.2 的表。
  - 链接文字里的 wikilink 由新的变换（优先级 40）在一次遍历里标出：它是否在 `*ast.Link` 之下。`harden` 的 `inLinkLabel` 是解析时"在方括号里"，方括号最终不成链接时就是错的。
- **`Links` 钩子**：起点是提取到的链接时，照 6.2 答；其余答 false。
- **`Markup`**：
  - `a` 加 `class`、`href`、`data-nw-node`、`data-nw-anchor`、`data-nw-target`；
  - class 加 `nw-unresolved`；
  - `span` 不再带 `data-nw-target`。

### 6.5 linking

- **即时解析的公共部分**：`Index.resolve` 拆出
  `resolutions(ctx, Store, Pages, notebookID, links) ([]domain.Resolution, missing []uuid.UUID, error)`。
  - 链接所在的页不在笔记本里时，那条链接解析不到；别名所在的页不在时，那个别名不算。两种都列进 `missing`。
  - `Index.resolve` 遇到 `missing` 仍然报错：持着锁时缺页是缺陷。
- **索引的读取**：`Store.View(ctx, pageID)` 答 `Indexed{Revision, Extractor, Resolutions map[int]domain.Resolution}`，以及是否有索引。
  - 查询是 `indexed_pages` 左连 `page_links`，都按主键。
  - 没有行，就是没有索引。
- **用例**：`app.Views{Store, Pages}.Resolve(ctx, Page, []Link) (map[int]domain.Resolution, error)`。
  - 没有链接时，不读。
  - 索引的 `revision` 与 `extractor` 都一致时，用它的解析；它没有的起点逐条即时解析。
  - 否则全部即时解析。`missing` 不算错误。
- **适配器**（`adapter/markdown`）：`Resolve(app.Views) obsidian.Resolve`。链接的转换与 `PageFacts` 是同一个（U+0000 记作 U+FFFD），所以即时解析与索引的答案相同。
- **模块根**：`ResolveLinks(pool, pages Pages) obsidian.Resolve`。不能叫 `New`：`reindex` 的组装也到达 `markdownExtensions`（archtest `composesMore`）。

### 6.6 组合根

- `markdownExtensions(resolve obsidian.Resolve)`。
- `parsing(cfg, logger, pool)` 交给它 `linking.ResolveLinks(pool, linkTargets{page.NewLinkTargets(pool)})`，`serve` 与 `reindex` 一样（`reindex` 不渲染）。
- 不连库的测试交一个替身。
- `GetPageView` 在 `Fetch` 期间仍占着解析预算（P2 审查的提醒）。索引一致时，`Fetch` 只是一条按主键的语句。

### 6.7 前端

- **跳转的手段**：`ReadingContext` 加 `navigate(to)`，由 `ReadingView` 用 `useNavigate` 给出。它是必填项，测试里写死的上下文随之补上。
  - 没有锚点的地址带 `arrived`，页面的标题拿到焦点，照快速切换。由 `ReadingView` 加上：`reading/` 不导入 `app/`。
- **增强 `pageLinks`**，排在 `taskToggle` 之后：
  - 给 `a[data-nw-node]` 设 `href`：`/{slug}/notebooks/{nb}/pages/{id}`，有锚点时加 `#锚点`。
  - 在容器上委托点击：没被处理过的左键、不带修饰键时，`preventDefault` 并经路由跳转；修饰键与中键照浏览器。
  - 撤销时去掉监听与它设的 `href`。
- **定位到锚点**：`ReadingView` 在 HTML 放进去之后，每次导航一次：
  - 找到 id 是地址里锚点（解码之后）的元素，滚动到它并给它焦点（`tabindex="-1"`）。
  - 页面打开时（页面的第一次导航：载入，或从别的页来）找不到（标题改了、转义写错）：焦点无处可去时（点的链接随前一页去了，或刚载入），页面的标题马上拿到焦点，由页面外壳给 `unanchored`（P3B 审查 R3 M2）；读者已经把焦点放在别处的，不动。
  - 视图来自缓存的，再重读一次：显示的可能比链接旧。重读带来那个元素、其间读者什么也没做（滚轮、触摸、点按、按键；浏览器自己的滚动不算）、焦点还在原处时，元素拿到焦点并滚到可见；重读结束或有新的导航，就不再等（修复核对；比滚动位置会把 Safari 延后的焦点滚动与后退时恢复的滚动当作读者的，第五轮在 WebKit 里实测）。
  - 同一页里指向不存在的标题的链接：不重读，焦点留在浏览器放的地方，页面不动（修复核对；Safari 点链接不给它焦点，第二轮在 WebKit 里实测过）。
  - "每次导航"按 `location.key` 加锚点，没有锚点的导航也记下：浏览器给手输的地址同一个 key（P3B 审查 R3 L3）；回到带锚点的那一条历史时再定位（修复核对）。记录由页面外壳保管：编辑时阅读视图卸下，回来不是新的导航，不再定位。
  - 接受：重读失败（网络、503）时不再等，之后的重试带来那个元素也不定位；后退、前进到已有的锚点时，浏览器恢复的滚动位置盖过视图的滚动，拿到焦点的标题可能不在视口里（这两条留给 M12 的打磨）；不给页面输入事件的移动（读屏软件的虚拟光标、语音控制的滚动、已打开的查找栏的"下一个"）看不到，什么也不移动的输入（只按修饰键、Escape）也结束等待（修复核对第六轮）。
- **重读不动位置**：事件或重新聚焦引起的重读不再滚动。
  - 带 id、拿着焦点的元素（定位到的标题也是）在新的 HTML 里重新拿到焦点（P3B 审查 R3 M1）。
  - 原来在窗口里、现在不在的，再滚到可见（`nearest`）；仍然可见的不滚，免得宽内容的横向位置被重置（修复核对）。
  - 任务项的复选框照旧（M5/P6）。
  - 接受：同名的标题因前面新加了一个而换了 id 时，焦点落到拿到这个 id 的那个；没有 id 的元素（链接）重读之后失去焦点，留给 M12 的打磨。
- **`links` 的处理**（`eventHandlers` 加一项）：
  - `pages` 里各页挂着的阅读视图（有数据或正在读）一律重读，不比版本，经 refresher、与 `pages` 同样的键；
  - `null` 时重读这个笔记本的全部阅读视图；
  - `targets` 留给反链（P5、P7）。
- **类型与测试**：`event.service.ts` 导出 `EventLinks`；`event-stream.test.tsx` 里"后来的类型"的例子改用别的类型名。
- **样式**：`.nw-unresolved` 颜色淡、虚线下划线。B 里它不可聚焦；P6 给 `role="button"`、`tabindex` 与点击。

### 6.8 测试

- **平台**：
  - 钩子用测试替身：属性与转义、`title`、图片里面的 `<a>`、先注册的优先、引用式链接的各次使用同一个状态、没有钩子时逐字节不变。
  - 一个链接里没有链接：用户的 `<a>` 围着各种链接、跨范围嵌套，脚注定义里没闭合的 `<a>`，被丢弃的元素里的 `</a>`，链接里的自动链接与脚注引用；`CheckHTML` 拒绝 `<a>` 套 `<a>`；`WriteAttrs` 不写不合的名称与地址。病态输入加上这几类（也是模糊测试的种子）。
  - `HeadingID` 与纯文字标题的 id 相同。
- **obsidian**：渲染的表格测试用替身 `Resolve`，覆盖 6.2 的每一行、锚点的规则、链接文字里的 wikilink、callout 的标题，并跑 `CheckHTML`。
- **linking 的 app**（替身）：
  - 没有链接时不读；
  - 索引一致时不读 `Pages`；
  - 版本不同、提取规则不同、没有索引时，即时解析；
  - 缺起点时，只即时解析那几条；
  - 来源页不见了，全部解析不到，不报错；
  - 别名的页不见了，那个别名不算；
  - `Index.resolve` 仍然报错。
- **linking 的 postgres**：`View` 的几种情况：没有索引、没有链接、有解析与没有解析。
- **组合根的 Markdown**：
  - 样例集、病态输入与放大类输入在两个替身下跑 `CheckHTML` 与 `CheckSize`：全部解析到（带最长的锚点），与全部解析不到。
  - 放大类加一条：引用式链接多次使用同一个长目标。
- **最后一跳**：经 `serve` 的阅读视图显示状态；`markdownExtensions` 不给 `Resolve` 时失败。改索引来证明读的是哪一条路：
  - 版本一致时把 `resolved_id` 置空，显示解析不到（读的是索引）；
  - 再改 `revision`、改 `extractor`、删掉 `indexed_pages` 的行，各自显示解析到（即时解析）。
- **vitest**：
  - `pageLinks`：地址；普通点击带与不带锚点；修饰键、中键、已处理的不拦；撤销。
  - `ReadingView`：每次导航定位一次，同一个 key 换了锚点也定位，回到带锚点的历史也定位，从编辑回来不定位；打开时视图来自缓存、只有重读之后才有的锚点也定位；打开时找不到，焦点无处可去时标题拿到焦点，焦点在别处时不动，来自缓存的重读一次，重读带来的元素在读者没做什么、焦点没动过时拿到焦点（滚轮、触摸、点按、按键各一例，浏览器自己的滚动不算），等待时同一页的链接与另一个地址结束等待，之后的读取不再定位；同一页的链接找不到时不重读、焦点不动（链接拿着焦点与什么都没拿着两种）；重读之后焦点留在原来的元素上，原来可见、现在不可见的再滚到可见。
  - `links` 的处理经组合根的 `eventHandlers`：同版本也重读，`null` 时重读全部，别的笔记本与没挂着的不读。
- **e2e**：`stories/links/l1-links.spec.ts`，L1 的一半。
  - API：状态的标记。
  - 页面：点击跳转与锚点；未建的没有地址；Ctrl/⌘ 点击开新标签页。
  - 页面，另一个故事：先挡住事件流，等连上时的刷新重读之后，在另一处新建那一页，不刷新，未建的变成链接。照 C7 的做法，只能经 `links` 事件（P3B 审查 R3 L1）。同一个故事里：同一页指向不存在的标题的链接，页面的标题不拿焦点、页面不动；指向标题的，标题拿到焦点；之后的重读里，标题的焦点与位置不变。
  - 改名之后仍跳到那一页，在 P4。
- **反向对照**：S1–S5 各自的变体都让测试失败。

### 6.9 实施步骤

1. **S1 平台**：`Attr`、`Extension.Links`、`Document` 留位置表、`marks`、`HeadingID`。
2. **S2 obsidian**：`Options` 与 `Resolve`、`Fetch`、渲染、链接文字里的变换、钩子、`Markup`；`Extension()` 的各处调用改为 `Extension(Options{})`。
3. **S3 linking**：`resolutions`、`Store.View` 与查询、`Views`、适配器、模块根。
4. **S4 组合根**：接线、Markdown 的检查、最后一跳。
5. **S5 前端**：`navigate`、`pageLinks`、定位到锚点、`links` 的处理、样式、vitest、e2e。

### 6.10 留给 P6

- 落点与点击新建、`role="button"` 与键盘、"页面不存在"的说明；
- 属性表里的链接；
- 本站完整地址经路由跳转；
- Markdown 里只有锚点的链接（`[t](#H)`，Obsidian 与 GitHub 的写法）按标题的 id 写地址：现在照旧写 `href="#H"`，标题的 id 却是 `nw-h`，点了不动（P3B 修复核对第二轮）；
- 标签改为链接；
- 附件的嵌入在 M7：之前 `![[img.png]]` 显示为未建，影响导入的库。

## 7. 与 M6 总设计的出入（本文定稿，总设计随 A 的合并修订）

- 解析规则：第 2 节的三处。
- 索引表不带外键：第 3.2 节。
- 迁移编号：page 的索引是 00019，linking 的五张表是 00020–00024（总设计写的是一个 00019）。
- 受影响的范围加上"触及的页的别名的键"（第 3.4 节，性质测试发现）；总设计 4.4 的观察者一节随之修订。
- frontmatter 的别名与标签照 Obsidian 的读法（第 3.4 节末），总设计只写了"frontmatter 的 aliases"；标签照 Obsidian 标签面板的计法。
- 第 3 步的并列按路径的字符数，不按层数；`.md` 只读作一种；出发文件夹的子树包括文件夹自己的页（第 2 节，P3A 审查 H3 与修复核对）。
- 只写正文的单元只到它自己的链接与增删的别名（第 3.4 节第 5 步）。
- 索引的键有上限，U+0000 记作 U+FFFD（第 3.2 节）。
- P3 分 A、B 两部分合并。
- B（第 6.1 节）：落点与点击新建、属性表里的链接、本站完整地址经路由跳转推到 P6；提取到的 Markdown 链接不再写相对地址；锚点取最后一段，块引用不带锚点；`[[#h]]` 写同页的 `href`；链接文字里的 wikilink 是没有状态的 `<span>`，一个链接里没有链接；`links` 的最小处理（只重读阅读视图）从 P6 提前到 B；即时解析不在一个快照里。总设计 4.7–4.9 与 Phase 表随 B 的合并已修订。

## 8. 完成标准

- A：第 5 节的测试全部通过；`resolve/` 的 `obsidian-verified` 样例与 Obsidian 一致；`make check`、`make gen-check`、e2e 全量通过；审查的发现处理完，修复经 Opus 核对。
- B：第 6 节的测试通过，同样的流程。

## 9. 结果

### A：解析与索引（2026-10-05，合并 `b802987`）

- **提交**：S1 `299c883`（+ `02784b9`）、S2 `95d2a02`、S3 `31cb5c1` + `6177e46`、S4 `6d52e3c`、S5 `c455784`；审查的修复 `573ee6f`；两轮修复核对，第一轮的修复 `9f35fb5`、`b0516ff`，第二轮没有行为上的发现。审查记录：[P3A-index-review.md](reviews/P3A-index-review.md)。
- **与 Obsidian 核对**：15 个解析样例 72 条链接，`obsidian-verified` 的与真实的 Obsidian 1.12.7 全部一致（修复之后又核对两次）；`nerve-defined` 的是别名（012）、并列按 id（009）与按整段比较的三处（015）。修复核对另用 Obsidian 自己的 `getLinkpathDest` 比较了约 52 万条随机链接，没有未记录的差异。
- **增量等于重建**：整个程序上 6 个种子 × 60 步，app 层的替身上 300 个种子 × 40 步、每个单元至多三个写（审查与核对时分别加大到 40 个种子与 3,000 个种子，没有反例）。性质测试第一次运行就找到了别名一项的遗漏（3.4 第 5 步的最后一条）。
- **反向对照**：S3–S5 与修复的变体都让测试失败（修复 43 个加核对之后的 3 个；其间写错的变体与起初存活的一个写在审查记录里）。
- **规模**（审查者测得）：一万页、二十万条链接的笔记本 reindex 约 11 秒，Go 堆峰值约 95 MiB；一页有 5,000 条反链时，自动保存的索引维护 2–3 ms。
- **负责人可以推翻的决定**：解析在两处跟 Obsidian（路径短按字符数、`.md` 只读作一种）、文件夹自己的页算在它的子树里；标签照 Obsidian 标签面板的计法；键的上限 1024 字节与 U+0000 记作 U+FFFD；reindex 遇到失败的笔记本继续。
- **留给后续**：
  - B：渲染与跳转（第 6 节）。
  - P4：属性链接按范围是否在 frontmatter 里区分（3.2），不靠 `property_key`。
  - P5：嵌套属性的键序在 `jsonb` 里丢失，只有顶层的次序（`position`）。
  - M7（经 M6 收尾的移交）：没有正文、只有子页、被链接的页，导出时是否写一个空的 `.md`（第 2 节）。
  - M12：链接很多的页的自动保存（[M12 性能移交](../M12-release/handoffs/M4-performance.md)第 6 项）。

### B：渲染与跳转（2026-10-05，合并 `23ce06e`）

- **提交**：S1 `c077b27`、S2 `c515494`、S3 `c806c8d`、S4 `347b9c0`、S5 `52ac987` + `e5bfe1a`；审查的修复 `cf40870`、`d2997fc`、`7c2a378`、`cce5916`；六轮修复核对，各轮之后的修复：第一轮 `abb7ed1`、`c8346e3`、`4317c6f`，第二轮 `e68fa63`、`d105473`、`9c85f89`、`3d8399f`，第三轮 `5193fbf`、`1c755c1`，第四轮 `af4a5f8`，第五轮 `a01eb31`，第六轮没有行为上的发现，只补了测试与注释（`e938d91`、`7681131`）。另修了与 B 无关的一次 CI 失败（W12 的竞态，`c3dcf75`）。审查记录：[P3B-render-review.md](reviews/P3B-render-review.md)。
- **一个链接里没有链接**：模糊测试（核对时最多平台 1,680 万次、obsidian 728 万次）与核对者写的语法驱动随机输入（第三轮 130 万个 × 三种设置）里没有 `<a>` 套 `<a>`，丢掉标签时文字一字不少。
- **定位到锚点**：核对者在 Chromium、Firefox、WebKit 里用探针实测（第五、六轮）：从滚动过的页打开、后退、改窗口大小时元素拿到焦点；滚轮、翻页键、横向滚动、触摸、拖动滚动条（Chromium、WebKit；Firefox 没能验证）时读者留在原处；Enter 跟随链接、只移动鼠标不算读者的输入。Safari 点链接不给它焦点、为 `focus()` 滚动在下一次渲染，两处都是在 WebKit 里实测才发现的。
- **成本**：一页写十万次同一条链接、一千个同名页，原来要 5 秒；现在每个目标只解析一次，成本测试在 `make test-go` 里跑。
- **镜像**：合并之后 `make image-smoke` 通过。第一次因构建上下文失败：Claude Code 的个人文件只在用户的全局 ignore 里，构建阶段看作未跟踪，提交信息成了 `modified=true`；两个 ignore 文件写明之后通过（`0800f94`）。
- **反向对照**：S1–S5 65 个变体，审查的修复与六轮核对 99 个，全部让测试失败（端到端的一个是构建之后在新的断言处失败）。第二轮发现一个测试从第一次运行就失败、变体是对着它量的，之后每组变体先在未改的代码上跑一遍（审查记录）。
- **负责人可以推翻的决定**：
  - 用户写的 `<a>` 围着渲染为链接的节点时丢掉这个标签、留下文字；更深一层或注释里的链接也照丢（保守的方向）。
  - 锚点找不到时，只在页面打开、焦点无处可去时页面的标题拿到焦点；同一页的链接找不到时什么也不动；来自缓存的视图重读一次，读回来的元素只在读者没有输入、焦点没动过时拿到焦点。
  - 即时解析不在一个快照里：其间的移动可能让一次的答案过时，下一次重读更正。
  - 地址被拒的 Markdown 链接里的一切照在链接里渲染（里面的 wikilink 不跳转）。
  - `links` 事件只重读阅读视图，列出的各页一律重读，不比版本。
- **留给后续**：
  - P6：点击未建的链接时问一句再新建（落点由服务端在点击时算）；标签改为链接（标签的节点实现 `Linker`）；属性表里的链接；本站完整地址经路由跳转；Markdown 只有锚点的 `[t](#H)` 按标题的 id 写地址（6.10）。
  - M12 打磨：[M5 打磨移交](../M12-release/handoffs/M5-polish.md)第 8 项（后退时浏览器恢复的滚动、没有 id 的元素重读之后失去焦点、重读失败、读屏软件的移动）。
  - A 留下的 P4、P5、M7 各项不变。
