# M7/P3 附件与链接（服务端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P3 附件与链接（服务端） |
| 状态 | 进行中 |
| 基线 | `4926432`（P2 合并、文档补完之后的 main）；本文提交之后开分支 `m7-p3a`，A 部分合并之后开 `m7-p3b` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.7、第 5、7–9 节；移交：[M6 链接与附件](handoffs/M6-links.md)第 1–3、6 项，[M4 附件的扩展](handoffs/M4-extensions.md)第 1、3 项，[M4/P3 Markdown 的扩展](../M6-links/handoffs/M4-P3-markdown-extensions.md)第 1–8 项（已关闭，抄送 M7）；总体设计 13.1 第 31 条，13.3 第 2、4、6 条；样例集 [README](../../../tools/md-fixtures/README.md) |

---

## 0. 分两部分

P3 改的面很宽：解析规则、索引、改写、落点、补全、渲染、平台的扩展点与检查。照 M6/P3 的先例分两部分，各自实现、审查、核对修复、合并：

- **A：解析、索引与改写**：`resolve/`、`rename/` 的附件样例与 Obsidian 的核对；page 的读端口带上附件；linking 的解析规则（三种读法）、索引记下解析到的是附件、改名与移动的改写、落点的新原因、补全的类型；附件的 `link`；`checkLinks` 与性质测试。阅读视图在 A 里照旧不把附件写成链接（3.6）。
- **B：渲染**：平台的图片钩子与 `View` 的到期；obsidian 扩展的 `Resolve` 带类型、`Assets`；附件的标记（图片、音频、视频、附件的链接，尺寸与说明，媒体的上限）；属性链接的类型与地址；`PageView.assets_expire_at`；`CheckHTML`、`CheckSize`；渲染样例的媒体记法；最后一跳。

第 1–3 节两部分共用，第 4 节是 A 的设计，第 5 节是 B 的设计，第 6–9 节共用。

## 1. 基线

- **page 的读端口**（`modules/page/link_targets.go`，`queries/links.sql`）：`ByKeys`、`Paths`、`Subtree`、`All` 都只读 `kind = 'page'`；`LinkNode` 没有类型。P2 之后附件与页面共用命名空间（兄弟之间标题键唯一），附件只能挂在页面或根下，所以路径上除了最后一个都是页面。
- **linking 的解析**（`domain/resolve.go`、`target.go`）：`Target.form` 只有 M6 的 `.md` 规则（带 `.md` 写的，笔记本里任何地方有去掉 `.md` 的那一页时读作它）；`Suffixes` 按路径的末尾建一棵树；`Resolution{ID, Ambiguous}`。索引表 `page_links` 只有 `resolved_id`。
- **观察者**（`app/index.go`）：附件的新建、改名、移动、删除已经经 page 的写入单元到达（`domain.Affected` 按变更的键与 id 圈出要重解析的链接），只是 `ByKeys` 读不到附件，所以什么都不变。
- **改写**（`domain/rewrite.go`）：Markdown 链接的目标一律加 `.md`；wikilink 的显示文字照"目标的最后一段去掉 `.md`"判断是否跟着改；`Linktext` 依次试名称、完整路径、完整路径加 `.md`。
- **落点**（`domain/landing.go`）：原因五种；同名的附件会让"问落点 → `createPage` 答 409 → 再问又是它"一直循环（M6 的移交第 3 项）。
- **补全**（`app/link_targets.go`）：`LinkTargetKind` 只有 `page`。
- **渲染**（`platform/markdown`）：核心的 Markdown 图片一律是 `<span class="nw-image">`（说明文字加一个链接）；扩展只能经 `Extension.Links` 换掉 `<a>` 的属性。`obsidian.Resolve` 答 `map[int]uuid.UUID`；`Options` 只有 `Resolve`。`Render` 只答 HTML。`CheckHTML` 允许 `href` 与 `src` 指向本站、http(s) 与 mailto；`CheckSize` 的倍数是 64。
- **组合根**：`markdownExtensions(resolve)`；`parsing()` 在 serve（`wire.go`）与 `nervewiki reindex`（`reindex.go`）里各建一个 Markdown，都传 `linking.ResolveLinks`。page 的模块在 asset 之前建（asset 要 `TreeWrites`），Markdown 又在 page 之前建。
- **前端**：`reading/app-links.ts` 只改写带 `data-nw-node`、`data-nw-tag` 的 `<a>`，`/api/` 开头的地址不在应用内跳转；`pages/page/unresolved-link.tsx` 逐个列出落点的原因；`page-properties.tsx` 把属性链接的 `node_id` 一律当作页面。

## 2. 与 Obsidian 的实测（1.12.7，2026-10-09）

在草稿目录里起了三个隔离的 Obsidian（各自的数据目录，打开"检测所有类型的文件"，附件是真的文件），探了解析 13 组 60 条链接、改名与移动 14 组、阅读视图里的 32 种嵌入写法。结论（逐条进第 4.8、5.8 节的样例）：

**解析**：

1. 最后一段带扩展名、笔记本里任何地方有这个名称的附件时，**只读作附件**：`[[x.png]]` 是 `A/x.png`，哪怕根下或同一文件夹里有标题为 `x.png` 的页（`x.png.md`）；`[[B/x.png]]` 在 `x.png` 只在 `A` 里、`B` 里有页面 `x.png` 时**解析不到**；`[t](./x.png)`、`[[/B/x.png]]` 同样。
2. 没有这个名称的附件时读作页面：`[[x.png]]`、`[t](x.png)`、`![](x.png)`、`![[x.png]]` 都是页面 `P/x.png`。
3. `[[x.png.md]]`、`[t](x.png.md)` 只指向页面 `x.png`，从不是附件 `x.png`（M6 的 `.md` 规则，只数页面）。
4. 没有扩展名的附件不是候选：`[[noext]]`、`[[A/noext]]`、`[t](A/noext)`、`![[noext]]` 都解析不到；`[[data]]` 是页面 `data`，不是附件 `A/data`。
5. 名称里的点都算扩展名的分隔：附件 `A/v1.2` 存在时，`[[v1.2]]` 是这个附件，不是根下的页面 `v1.2`；`[[v1.2.md]]` 是页面。
6. 大小写、Unicode 的规范化不分（`[[X.PNG]]`、NFC 与 NFD 的 `Café 2.png`）；子树优先、路径短的优先、相对与从根起，都与页面相同（`A/B/src` 里的 `[[x.png]]` 是 `A/B/x.png`；`[[B/x.png]]` 按后缀找到 `A/B/x.png`）。
7. pdf、mp3、webm 与没见过的扩展名都解析（打开"检测所有类型的文件"时）。
8. Obsidian 不按别名解析（别名在这里是 `nerve-defined`，只对页面，第 4.2 节）。

**改名与移动**（打开"始终更新内部链接"、最短的链接格式）：

1. 附件改名、换扩展名、移动时，指向它的嵌入、wikilink、Markdown 链接与图片、frontmatter 的属性链接都改写；名称在笔记本里唯一时写名称，不唯一时写完整路径（与页面相同）；尖括号的写法保留；只改大小写的改名也改写。
2. Markdown 链接指向附件时**不加 `.md`**。
3. wikilink 的显示文字：等于附件名称**去掉扩展名**的（`[[A/x.png|x]]`）跟着改成新名称去掉扩展名（`[[y.png|y]]`），等于带扩展名的名称的（`|x.png`）不改；嵌入的显示是尺寸或说明，不改。Markdown 链接的文字等于原来的名称（带扩展名）或完整路径的，换成新名称（与页面相同）。
4. 移动一页时，它子树里的附件随之移动，指向它们的链接改写；移出子树的页里指向附件的链接按新的位置改写。
5. Obsidian 写出解析不到的链接的两处（这里 `nerve-defined`）：一页改名成与某个附件同名（`B/old` → `B/x.png`，`x.png` 在 `A`）时，它把 `[[old]]` 写成 `[[B/x.png]]`，而这读作附件、解析不到；附件改名成与某页同名（`A/x.png` → `A/note.png`，有页面 `P/note.png`）时，它把指向那页的 `[[note.png]]` 写成 `[[P/note.png]]`，同样解析不到。两处的 Markdown 链接它写 `x.png.md`、`note.png.md`，能解析。这里 wikilink 也写能解析到那一页的 `.md` 写法（4.4）。
6. 相对与从根起的 Markdown 链接，Obsidian 换成最短的写法；这里照 M6 保留原来的写法（M6 的样例 008、011，`nerve-defined`）。属性链接的引号风格 Obsidian 改成双引号；这里照 M6 保留（样例 004）。

**阅读视图的嵌入**：

1. 图片：`![[x.png]]` 的 `alt` 是名称；`![[x.png|说明]]` 的 `alt` 是说明；`|300` 是宽、`|300x200` 是宽和高；`![[x.png|说明|300]]` 是说明加宽（最后一个 `|` 之后是尺寸）；`|300x`、`|x200` 是说明。Markdown 图片 `![说明|300](x.png)` 同样；`![](x.png)` 没有 `alt`。Obsidian 不限尺寸的数值（`|0`、`|20000` 照写）。
2. 音频（`![[a.mp3]]`、`![](a.mp3)`）是 `<audio controls>`，视频（`![[v.webm]]`、`![说明](v.webm)`）是 `<video controls>`，`|300` 给视频宽；说明进外层的 `alt`。
3. PDF 的嵌入是内联的阅读器；没见过的类型是一个带名称的文件框；解析不到的嵌入显示"找不到…"。
4. 链接里的图片（`[![](x.png)](url)`、`[![[x.png]]](url)`）是 `<a>` 里的 `<img>`。
5. `[[x.png]]`、`[t](x.png)` 是指向文件的链接，显示写的文字。
6. 图片与媒体都在行内，不另起一块：`text ![[x.png]] text` 是一段。

## 3. 目标与范围

**做**（总设计第 7 节 P3 一行、4.7）：

1. 附件进解析（第 4.2 节的三种读法）；索引记下解析到的是附件；附件的新建、改名、移动、删除经观察者重新解析；`checkLinks` 认附件；M6 的性质测试加附件的操作。
2. 改名、移动的改写认附件；`Linktext` 给被同名附件遮住的页写 `.md` 的写法。
3. 落点：读作附件的目标答 `target_is_asset`；补全：`LinkTarget.kind` 有 `asset`；附件的 `link`（元数据、列表、上传的答复）。
4. 附件的标记与核心的图片钩子；`obsidian.Options` 的 `Resolve` 带类型、`Assets`；属性链接带类型与地址；`PageView.assets_expire_at`；一个视图至多内联 20 个音频、视频。
5. `CheckHTML` 只许 `img`、`audio`、`video` 的地址是本站的路径，应用的测试加"全部解析到附件"的模式并核对是附件内容的签名路径；`CheckSize` 的放大输入加附件的嵌入。
6. 样例：`resolve/`、`rename/` 加附件，核对脚本建真的文件、打开"检测所有类型的文件"，与 Obsidian 1.12.7 核对；`render/` 加媒体的记法。
7. 最后一跳：附件的解析、索引、改写、渲染各在整个程序上有行为测试，组合根交空时失败。
8. 前端只跟上契约的最小改动（第 4.7、5.7 节）：落点的新原因、属性链接里的附件；不做面板、上传、增强（P4）。

**不做**：

- 附件的反链、按附件列出谁嵌入了它：没有附件的页面视图（P4 的面板也不列）。
- 前端的 `assets` 增强（新标签页、到期重读、保留媒体）、补全里附件的排序与标记：P4。
- e2e 的 AS3（嵌入的显示、改名之后的改写、`expectIndexedLinks` 带附件）：P4，届时有页面上的显示可看。P3 的整个程序的测试在 Go 里。
- 已有库里的旧链接：P3 之前写下、指向附件名称的链接，索引在下一次触及它们的写入或 `nervewiki reindex` 之前仍按旧的解析（v0.1 没有发布，只影响开发库；阅读视图对索引过的正文照信索引）。

## 4. A：解析、索引与改写

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `tools/md-fixtures/resolve/016-…`–`02x-…`（新）、`rename/033-…`–`04x-…`（新）、`README.md`、`check.mjs`、`obsidian/verify-resolve.mjs`、`verify-rename.mjs` | 样例的 `assets`，核对脚本建附件（4.8） |
| `server/internal/modules/page/link_targets.go`、`adapter/postgres/links.go`、`adapter/postgres/queries/links.sql` | 读端口带上附件与类型（4.3）；附件的路径与同键附件的个数（`AttachmentsByIDs`，4.6） |
| `server/internal/modules/linking/domain/`：`resolve.go`（`Node`、`Resolution`、`Linkable`）、`target.go`、`landing.go`、`rewrite.go`（`AssetLinktext`）、`change.go`（`PathBefore` 带上类型） | 三种读法、按类型的候选、落点、改写（4.2、4.4、4.5） |
| `server/internal/modules/linking/app/`：`ports.go`、`access.go`、`link_targets.go`、`asset_links.go`（新） | 解析到的类型、补全的类型与过滤、附件的 `link`（4.5、4.6） |
| `server/internal/modules/linking/adapter/postgres/`、`server/migrations/sql/00028_linking_page_links_resolved_asset.sql`（新）、`server/migrations/schema_test.go` | `page_links.resolved_asset`（4.3） |
| `server/internal/modules/linking/adapter/markdown/views.go` | A 里不把附件交给渲染（4.6） |
| `server/internal/modules/linking/module.go` | `AssetLinks`（4.6） |
| `server/internal/modules/asset/app/`（`upload.go`、`reads.go`、`ports.go`）、`adapter/http` | 答复带 `link`（4.6） |
| `api/modules/linking.yaml`、`api/modules/asset.yaml`、`api/modules/events.yaml`、`api/modules/page.yaml` | `LinkTargetKind`、`LandingReason`、`PropertyLink.kind`、`Asset.link`；links 事件的两个集合、改名与移动的说明（4.7） |
| `server/internal/bootstrap/`：`linking.go`、`deps.go`、`links_test.go`、`links_assets_test.go`（新）、`links_rebuild_test.go`、`permission_matrix_linking_test.go` | 读端口的类型、`AssetLinks` 的组合、`checkLinks`、整个程序的测试（4.9） |
| `web/apps/web/src/pages/page/unresolved-link.tsx`、`page-properties.tsx`、`i18n/messages` | 落点的新原因、属性里的附件不当作页面（4.7） |
| `e2e/stories/links/l5-completion.spec.ts`、`l6-panel.spec.ts` | 属性链接的 `kind`（4.7） |

### 4.2 解析：三种读法

目标先定读法，之后每一步只用这一种（Obsidian 的 `getLinkpathDest`，第 2 节解析的 1–5）：

| 写法 | 读法 | 候选 |
|---|---|---|
| 最后一段以 `.md` 结尾（不分大小写） | M6 的规则：笔记本里任何地方有去掉 `.md` 的那个标题的页时读作它，没有时读作原样 | 页面 |
| 其余，且笔记本里任何地方有标题键等于最后一段的附件 | 附件 | 附件 |
| 其余 | 页面 | 页面 |

- "附件"只算有扩展名的：名称里有一个不在首尾的点（`page/domain` 的 `hasExtension`）。没有扩展名的附件从不进候选（第 2 节解析的 4），所以不带点的目标自然只读作页面，不必另判目标的写法。
- 读法定了之后，次序与 M6 相同：相对、从根起或恰好是完整路径、路径后缀（子树优先，再路径短的，再 id，并列时标记歧义）；别名（第 4 步）只对页面的读法：附件的读法下单个名称在第 3 步总能找到。
- 附件的路径长度同页面（名称与 `/` 的 UTF-16 码元数）；页面的路径不数 `.md`，附件的名称带着扩展名。两种只在各自的候选里比较，常数不影响。
- 实现：`domain.Node` 加 `Asset bool`，`Resolution` 加 `Asset bool`（解析到的是附件）。`Target.form(candidates)` 答键与类型；`candidateSet` 的三个方法带类型；`Suffixes` 按类型各建一棵（`has` 仍是一次查表，M6 收尾 FA-I1 的代价不变）；`nodeList` 按类型过滤。`NewSuffixes` 跳过没有扩展名的附件。

### 4.3 索引：读端口、`resolved_asset`、观察者

- **page 的读端口**：`LinkTargetsByKeys`、`LinkTargetsByIDs` 去掉 `kind = 'page'`，答出 `kind`；`LinkNode` 加 `Asset`；`Subtree` 带上子树里的附件（改名、移动一页时它们的路径随之变，指向它们的链接要重解析，总设计 4.7）；`All` 带上附件（补全）。`PageIDs`、`NotebookOf`、`Content` 仍只认页面（它们读正文）。
- **`page_links.resolved_asset`**（迁移 `00028`，linking 的表）：`boolean NOT NULL DEFAULT false`，`CHECK (NOT resolved_asset OR resolved_id IS NOT NULL)`。节点的类型不会变，所以它跟着 `resolved_id` 定；`SetResolutions` 写它，`Links`、`View`、`Properties` 读它。不提升 `Extractor`：提取结果没变，P3 之前的行都解析到页面，`false` 是对的。down 先把指向附件的链接置为解析不到，旧程序不会把附件的 id 当作页面，之后由它的 `nervewiki reindex` 按它的规则重新解析（审查 B7）。
- **观察者**不改：附件的新建（`Before` 为空）、删除（`After` 为空）、改名、移动经 `Affected` 按键与 id 圈出链接，`ByKeys` 读到附件之后它们就重新解析；一页改名、移动时 `spread` 经 `Subtree` 圈进子树里的附件。`DeletePages` 收到附件的 id 时什么都不删（附件没有行）。
- **`checkLinks`**（`bootstrap/links_test.go` 的不变式）：每条解析到的链接指向同一本笔记本里未删除的节点，类型与 `resolved_asset` 相符。从头解析的对照是 `TestTheIndexIsItsRebuild`（增量维护等于重建，随机的写入加了附件的上传、改名、移动、删除与指向附件的目标）与附件的整个程序测试每一步之后的 `checkRebuilt`（审查 C3）。

### 4.4 改名、移动的改写

改写的参与者照旧（M6/P4），附件的改名与移动经同一个单元到达（P2 的 `unit_rename` 已认附件的名称规则）。要改的是写法：

1. **Markdown 链接指向附件时不加 `.md`**：`markdownTarget` 的三种写法（相对、从根起、名称或完整路径）对附件都不加；尖括号与编码规则不变。
2. **wikilink 的显示文字**：M6 的规则"目标带 `/`、没有锚点、显示文字等于目标最后一段去掉 `.md`"里的"去掉 `.md`"对附件是"去掉扩展名"，新的显示文字同样是新名称去掉扩展名（第 2 节改名的 3）。Markdown 链接的文字照旧比较完整的名称与路径。
3. **`Written` 只数同类型的**：`[[x.png]]` 的唯一性只看附件 `x.png`，同名的页面不算；`Tree.Named` 两种都放，`candidates`、`leads` 照第 4.2 节定读法。
4. **被同名附件遮住的页**：`Linktext` 依次试名称、完整路径、完整路径加 `.md`，读作附件的前两种不通，写出 `B/x.png.md`（根下的是 `x.png.md`）；Markdown 链接写名称加 `.md`（`x.png.md`）。这是第 2 节改名的 5 的 `nerve-defined`：Obsidian 写出解析不到的 wikilink。
5. **`WrittenKeys`**：附件只有它自己的键（它的写法只读作附件）。
6. 附件改名不能去掉扩展名（P2 的规则），所以改名之后它仍是候选。改名、移动之后指向别处的同名附件、被抢走的页（附件改名成某页的标题），照 M6 的"之后解析到别处的改写成仍指向原来那个"。

### 4.5 落点与补全

- **落点**：目标读作附件（第 4.2 节）时答 `target_is_asset`，`node_id` 为空，不论它是否解析到——解析到附件的不是"已有的页"，解析不到的（`[[B/x.png]]`）新建页面也解析不到它。`parents` 只取页面。读作页面的目标（写了 `.md`，或附件没有扩展名），新页要放的地方已有同标题键的附件时同样答 `target_is_asset`（实施中随机测试找到：`../v1.2.md` 落在附件 `v1.2` 旁边会撞名）。这解决 M6 的移交第 3 项的循环：同名的附件存在时，不再提议新建一个 `createPage` 会答 409 的页。
- **补全**：`LinkTarget` 加 `Kind`（`page`、`asset`）；附件的 `aliases` 为空；`link` 按第 4.4 节的写法（附件：名称在附件里唯一时写名称，否则写完整路径；被遮住的页写 `.md` 的写法）。没有扩展名的附件不列：没有链接引向它，它的路径可能按后缀引向同路径的页（审查 A1）。`Linktexts` 收全部节点、按标题键分组；附件按 `AssetLinktext`（同键附件的个数）写，代价随它们线性，与逐个的 `Linktext` 相同（性质测试）。

### 4.6 附件的 `link`、A 里的阅读视图

- **附件的 `link`**：与 `LinkTarget.link` 同一个写法。linking 的模块根给 `NewAssetLinks(attachments)`：page 的 `AttachmentsByIDs` 一条语句读附件的路径与同标题键的附件个数（数到 2，一个快照），`domain.AssetLinktext` 只有它一个时写名称、否则写完整路径，与 `Linktext` 一致（随机树的性质测试）；没有扩展名的附件 `link` 为 null。原定按标题键读全部候选（`ByKeys`）再走 `Linktext`：同名的附件多时读得多、上传时在锁里，池上分两次读时并发的改名可能写出指向另一个附件的名称（审查 B4、B6）。asset 的 app 加端口 `Links`，组合根把它接上。
  - 元数据与列表：读完节点之后按笔记本一次读。
  - 上传：在单元的 `after` 里、写完行之后读（同一个事务，看得到新的节点，持着单元的锁，没有别的同名节点同时出现）；读失败时单元回滚，文件留给孤儿清扫（P2 的规则）。
  - `link` 是读取时的写法：之后别处出现同名的附件，旧的答复里的写法就不再唯一，粘贴插入时（P4）以当时读到的为准。
- **A 里的阅读视图**：`linking.ResolveLinks` 跳过解析到附件的链接（照 P2 渲染为未解析），B 部分换成带类型的答复。这样 A 合并之后，阅读视图不会把附件写成 `data-nw-node`（前端会当作页面，去 `/…/pages/<id>`，那是 404）。有测试钉住，B 里改掉。
- **属性链接**：`PropertyLink` 加 `Kind`；A 里接口答 `kind`（`page`、`asset`，解析不到时为空），前端把 `asset` 的显示为文字，B 里给地址（5.6）。

### 4.7 契约与前端

- `linking.yaml`：`LinkTargetKind` 加 `asset`，说明改写；`LinkTarget.name`、`link` 的说明（附件的名称带扩展名；被遮住的页的 `.md`）；`LandingReason` 加 `target_is_asset`，`getLinkLanding` 的说明；`PropertyLink.kind`（`LinkTargetKind` 或 `null`）。
- `asset.yaml`：`Asset.link`（必有，没有扩展名的附件为 null），说明它是读取时的写法。
- `events.yaml`：links 事件的 `pages`、`targets` 可以含附件。`page.yaml`：renameNode、moveNode 的说明写进附件：改名、移动附件会改写指向它的链接，引用它的页正在编辑时整个拒绝（审查 B5）。
- 前端：`unresolved-link.tsx` 给 `target_is_asset` 一句说明（"这个名称是附件"），照"没有落点"处理并重读视图；`page-properties.tsx` 对 `kind = asset` 的属性链接只显示文字；`LinkTarget.kind` 不影响补全的插入（附件的标记与排序在 P4）。

### 4.8 样例与核对

- **`resolve/`**：JSON 加可选的 `assets`（附件从根起的路径，父页先于它列在 `pages` 里）；`to` 可以是附件的路径。`check.mjs` 核对 `to` 是页面或附件、附件的父页在 `pages` 里、兄弟之间标题键唯一（页面与附件共用）。`verify-resolve.mjs` 建附件：图片是一个真的 7×5 的 PNG，其余类型写几个字节；库的 `.obsidian/app.json` 打开 `showUnsupportedFiles`；比较时附件按原路径、页面按路径加 `.md`。
- **`rename/`**：JSON 加可选的 `assets`；`from`、`to` 是附件的路径时是附件的改名或移动（Obsidian 里一次 `renameFile`）。`verify-rename.mjs` 用 `createBinary` 建附件，同样打开 `showUnsupportedFiles`。
- 新样例（第 2 节的实测逐组写成，`obsidian-verified`，除非注明）：
  - 解析：附件遮住页面（根下、同文件夹、`[[B/x.png]]`、相对、从根起）；只有页面时读作页面；`.md` 的写法；没有扩展名的附件；名称里的点；大小写与 NFC、NFD；子树、路径长短、后缀；与附件同名的别名（`nerve-defined`）；pdf、音视频、没见过的扩展名；Markdown 链接、图片、嵌入。
  - 改写：附件改名（各种写法，相对与从根起的那几条 `nerve-defined`）；换扩展名；只改大小写；移动（名称不再唯一）；移动一页带着附件；页面移出子树；显示文字的去掉扩展名；Markdown 链接的文字；frontmatter；被遮住的页、被抢走的页（`nerve-defined`）。
- 服务端的样例测试（`resolve_cases_test.go`、`rename_cases_test.go`）读 `assets`，附件的 id 接着页面的递增。
- 实施中改写样例 038 拆成两条：038（`obsidian-verified`，`![[x.png]]` 随页面移动写成 `![[B/A/x.png]]`）与 046（`nerve-defined`：按后缀仍能解析的 `[[A/x.png]]`、`[t](A/x.png)` 不改写，M6 的规则）。

### 4.9 A 的测试与最后一跳

- **domain**：读法的表格（第 2 节解析的每一条，加"附件只有没有扩展名的"、"附件与页面都在、`.md` 的写法"）；`Suffixes` 与 `nodeList` 对同一组输入答同样的结果（M6 的对照测试加附件）；落点的 `target_is_asset`（解析到、解析不到、只有页面时照旧）；改写的样例；M6 的改写随机测试加附件的改名与移动。
- **app**：索引的随机测试（增量维护等于重建）加附件的新建、改名、移动、删除；`Views.Resolve` 答出 `Asset`；补全的类型；`AssetLinks`。
- **整个程序**（组合根交空时失败）：
  - 观察者：经接口上传、改名、移动、删除附件，指向它的链接的 `resolved_id`、`resolved_asset` 随之变，`checkLinks` 通过；一页改名时子树里的附件随之重解析。
  - 改写：经接口改名、移动附件，另一页的嵌入、Markdown 图片随之改写，进同一个变更集与同一次事件。
  - 落点：同名的附件存在时答 `target_is_asset`，不再循环。
  - 附件的 `link`：上传、元数据、列表的答复；同名的第二个附件之后写完整路径。
  - A 里的阅读视图不写指向附件的 `data-nw-node`。
  - 引用附件的页正在编辑时，附件的改名答 409 `linking.pages_locked`，什么都不变（审查 B5）。
  - 索引等于重建：`TestTheIndexIsItsRebuild` 的随机写入加附件的上传、改名、移动、删除，目标加附件的写法。
- 权限矩阵不加格：没有新操作；`listLinkTargets` 的核对认附件与类型。

## 5. B：渲染

### 5.1 文件

| 文件 | 内容 |
|---|---|
| `server/internal/platform/markdown/`：`markdown.go`、`render.go`、`marks.go` | `Extension.Images`、`Extension.Expires`、`View`（5.2） |
| `server/internal/platform/markdown/markdowntest/`：`check.go`、`inputs.go` | `src` 的规则；放大与病态输入加附件的嵌入（5.9） |
| `server/internal/platform/markdown/obsidian/`：`obsidian.go`、`view.go`、`render.go`、`asset.go`（新）、`size.go`（新） | `Target`、`Assets`、附件的标记、尺寸与说明、媒体的上限（5.3–5.5） |
| `server/internal/modules/asset/`：`embeds.go`（新，模块根）、`app/embeds.go`（新） | `NewEmbeds`：只凭连接池与密钥，答附件的类型、名称、大小、宽高与签名地址（5.4） |
| `server/internal/modules/linking/`：`adapter/markdown/views.go`、`app/properties.go`、`module.go`、`adapter/http` | `ResolveLinks` 带类型；属性链接的地址（5.6） |
| `server/internal/modules/page/`：`app/get_page_view.go`、`app/ports.go`、`adapter/markdown`、`adapter/http` | `PageView.assets_expire_at`（5.7） |
| `api/modules/page.yaml`、`linking.yaml` | `PageView.assets_expire_at`、`PropertyLink.url` |
| `server/internal/bootstrap/`：`registrants.go`、`deps.go`、`reindex.go`、`markdown_app_test.go`、`links_view_test.go`、`assets_view_test.go`（新） | `markdownExtensions(resolve, assets)`；最后一跳（5.10） |
| `tools/md-fixtures/render/`、`obsidian/verify-render.mjs`、`server/internal/platform/markdown/obsidian/render_fixtures_test.go` | 媒体的记法（5.8） |
| `web/apps/web/src/pages/page/page-properties.tsx`、`reading/app-links.test.ts` | 属性里附件的地址；`appLinks` 不碰附件的链接（有测试） |

### 5.2 平台：图片钩子与到期

- **`Extension.Images`**：`func(data any) func(start int) (Image, bool)`。核心渲染 Markdown 图片时按目标在正文里的起点（`Tree.Destination`）问扩展，第一个答 `true` 的写它：`Image` 是一个函数，拿到图片的说明文字（`ShownText`）与是否在链接里，写出整个元素。没有扩展认领的图片照旧是 `<span class="nw-image">`。
  - 写出的地址照旧经 `WriteAttrs`（`href`、`src` 过 `SafeURL`）；元素与属性登记在 `Markup`。
  - 链接里的图片：钩子拿到"在链接里"，写 `<img>` 不写链接；不是图片的（音视频、附件的链接）在链接里写成不带地址的文字（链接里不能有交互的元素）。
- **`Extension.Expires`**：`func(data any) time.Time`，扩展的数据在什么时刻之后不再有效（零值为一直有效）。
- **`Render` 答 `View{HTML string; Expires time.Time}`**：`Expires` 是各扩展的最早者。调用方（page 的适配器、测试）改读 `.HTML`。
- 13.3 第 6 条的钩子表随之加 `Images`、`Expires`（12.1 第 6 条的例外，逐项写明）。

### 5.3 obsidian 扩展的参数

```go
// Target is where a link leads: a node, an attachment's or a page's.
type Target struct {
    Node  uuid.UUID
    Asset bool
}
type Resolve func(ctx context.Context, page markdown.Page, links []Link) (map[int]Target, error)

// Asset is what a reading view shows of an attachment.
type Asset struct {
    Name          string
    MIME          string
    Bytes         int64
    Width, Height int       // 0: unknown
    URL           string    // the content's address, signed, relative
    Expires       time.Time
}
type Assets func(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Asset, error)

type Options struct {
    Resolve Resolve
    Assets  Assets
}
```

- `Fetch`：先 `Resolve`，再把解析到附件的 id（去重）交给 `Assets`，一次。`Assets` 只答这本笔记本里活着、有行的附件；没答的、`Assets` 为空的，渲染为不带地址的文字（5.5）。`Expires` 是答出的附件里最早的到期。
- 页面的目标照 M6 写 `data-nw-node`；解析到附件的**从不**写 `data-nw-node`。

### 5.4 asset 给渲染的端口

- `asset.NewEmbeds(pool, nodes, contentKey, clock)`：只凭连接池、page 的读端口（`page.NewAssetNodes`）与派生的密钥构造，因为 Markdown 在 page 与 asset 的模块之前建（`parsing()`）；13.1 第 11 条的常规（端口只凭连接池），不是例外。
- `Embeds(ctx, notebookID, ids)`：读节点（类型是附件、在这本笔记本、没删）与行，读一次时钟签出全部地址（内联的地址，`d` 不设），答名称、类型、字节数、宽高。读不到的不答，不报错；数据库的错误照常返回，阅读视图答 500。
- 不再核对读权限：渲染的是读者能读的页，链接只解析到同一本笔记本里的节点，附件的 `asset.read` 与页面的读同为读者（P2 的规则表）。`Embeds` 的注释写明它只给阅读视图与属性用，ids 必须来自这本笔记本的解析。
- 组合根把它适配成 `obsidian.Assets`，并交给 linking 的属性读取（5.6）。

### 5.5 附件的标记

解析到附件、`Assets` 答出了它的每一种写法，按类型：

| 写法 | 图片（`image/*`） | 音频（`audio/*`） | 视频（`video/*`） | 其余（含 pdf） |
|---|---|---|---|---|
| `![[…]]`、`![…](…)` | `<img>` | `<audio>` | `<video>` | 附件的链接 |
| `[[…]]`、`[…](…)`、属性链接 | 附件的链接 | 附件的链接 | 附件的链接 | 附件的链接 |

- `<img class="nw-asset" src=… alt=… data-nw-asset=… [width] [height] loading="lazy">`：`alt` 是说明，没有时是名称（`![](x.png)` 在 Obsidian 里没有 `alt`，这里给名称，`nerve-defined`，可访问性更好）；`width`、`height` 是写下的尺寸，没写尺寸时是附件的宽高（知道时），让排版不跳。
- `<audio class="nw-asset" controls preload="none" src=… data-nw-asset=… aria-label=…>`、`<video …同上… [width] [height]>`：`aria-label` 是说明或名称。
- 附件的链接：`<a class="nw-asset" href=… data-nw-asset=… data-nw-size=…>`（wikilink 另有 `nw-wikilink`），文字照写法（wikilink 的显示、Markdown 链接的文字；嵌入与 Markdown 图片是说明或名称）。文件名与大小不写成界面的文字，前端按语言格式化 `data-nw-size`（P4）。新标签页由 P4 的增强给。
- **不带地址的文字**（`Assets` 为空、没答这个附件）：`<span class="nw-asset">`，文字同上，不写成页面的链接。
- **尺寸与说明**（`size.go`）：嵌入的显示（第一个 `|` 之后）与 Markdown 图片的说明，最后一个 `|` 之后是 `宽` 或 `宽x高`（十进制数字）时是尺寸，其余是说明；`300x`、`x200` 是说明。数值在 1–10,000 之外的照尺寸的形状认，但不写（`nerve-defined`：Obsidian 照写 `0`、`20000`）。Wikilink（不是嵌入）的显示是文字，不读尺寸。
- **媒体的上限**：一个视图里按文档的次序至多 20 个 `<audio>`、`<video>`，第 21 个起写附件的链接（总体设计 13.1 第 31 条加一项）。图片不限（`loading="lazy"`）。
- **锚点**：`![[x.png#a]]` 的锚点不影响附件的标记（Obsidian 只把它写进 `alt`）。
- `Markup` 登记：`img`（`class`、`src`、`alt`、`width`、`height`、`loading`、`data-nw-asset`），`audio`（`class`、`controls`、`preload`、`src`、`aria-label`、`data-nw-asset`），`video`（再加 `width`、`height`），`a` 加 `data-nw-asset`、`data-nw-size`，`span` 不加属性（不带地址的文字只有 class）；class `nw-asset`；`URLs` 加 `src`。`img` 是空元素，`audio`、`video` 有结束标签。

### 5.6 属性链接

- 正文的属性表（服务端的 HTML）：属性链接经 `view.property` → `lead`，解析到附件的写附件的链接（同 5.5），不读尺寸。
- 右栏（`GET …/properties`）：`PropertyLink` 加 `url`：附件的内容地址（签名的），页面与解析不到的为 `null`。linking 的 `GetPageProperties` 收附件的 id，一次调用组合根给的 `Embeds`（同 5.4）；前端对 `kind = asset` 的写成指向 `url` 的链接，不经 `href(lead)`。

### 5.7 `PageView.assets_expire_at`

- page 的 `Markdown` 端口的 `Render` 答 HTML 与 `Expires`，`GetPageView` 把它交给接口：`assets_expire_at`（`date-time`，视图里没有附件时为 `null`）。签名的地址按小时取整、有效一到两小时（P2），一个视图里的附件同时签出，到期相同。
- 前端在 P4 据它重读；B 里只加进契约与类型。

### 5.8 渲染样例的媒体记法

- `render/` 的 JSON 加可选的 `assets`（库根下的附件名称）；`verify-render.mjs` 把它们建进库（图片是真的 PNG）。
- 记法：读 DOM 时，图片写 `⟨img alt w×h⟩`（没有的部分省去，`w×h` 只写写下的尺寸），音频 `⟨audio 说明⟩`，视频 `⟨video 说明 w×h⟩`；Obsidian 一侧读嵌入外层的 `span.internal-embed` 的 `alt`、`width`、`height`，服务端一侧读 `img` 的 `alt`、写下的尺寸（样例的 `Assets` 不给宽高）与 `aria-label`。附件的链接照文字读。PDF 与没见过的类型不进样例（Obsidian 是阅读器与文件框，这里是链接，`nerve-defined`）。
- 新样例：嵌入与 Markdown 图片的尺寸与说明（第 2 节阅读视图的 1）、音视频、链接里的图片、行内的图片不另起一块。
- `render_fixtures_test.go`：`Resolve` 按名称在 `assets` 里找，`Assets` 按扩展名给类型。

### 5.9 检查

- **`CheckHTML`**：`src` 只许出现在扩展登记了它的元素上，值必须是本站的路径（不带协议与主机）；外站的图片照旧不加载（总体设计 4.3）。
- **应用的测试**（`TestTheAppsMarkdownRendersCheckedHTML`）加"全部解析到附件"的模式：图片、音频、视频、pdf 各一轮，`Assets` 用 asset 的真的签名（`NewEmbeds` 的签名部分，一把固定的密钥），并另核每个 `src`、附件链接的 `href` 都是附件内容的签名路径（`/api/v0/assets/<id>/content?e=…&s=…`）；从不出现指向附件的 `data-nw-node`。
- **`CheckSize`**：`Amplifying()` 加附件的嵌入（几个字节的 `![[x]]`、引用式的 `![x]` 写出带签名地址的整个标记）、病态输入加很多的嵌入；`TestTheAppsLinksAreWithinTheirBound` 加"全部解析到附件"。现在的 64 倍若不够，先缩短标记（例如去掉可以省的属性），再按实测提高倍数，写进第 9 节与 13.3 第 4 条。

### 5.10 组合根与最后一跳

- `markdownExtensions(resolve, assets)`；serve 的 `parsing()` 给 `asset.NewEmbeds`；`nervewiki reindex` 给 `nil`（它只提取），有测试证明提取不受它影响（同一正文在两种组合下的 `Facts` 相同）。
- 整个程序的测试（组合根交空时失败）：
  - 上传一张图片、一段音频与一个 pdf，正文里嵌入与链接它们，经 `GET …/view` 读到附件的标记，地址能下载到同样的字节；`assets_expire_at` 是签名的到期。
  - 删掉附件之后同一视图是未解析的链接；`Assets` 交空的组合里是不带地址的文字。
  - 属性链接指向附件时，右栏答 `kind = asset` 与能下载的 `url`。

## 6. 实施步骤

**A**（分支 `m7-p3a`）：

1. A1 样例与核对：`resolve/`、`rename/` 的附件样例，`check.mjs`，两个核对脚本；在隔离的 Obsidian 里跑通（第 7 节的约束）。
2. A2 page 的读端口带上附件与类型（SQL、`LinkNode.Asset`、`Subtree`、`All`），数据库测试。
3. A3 linking 的 domain：读法、按类型的候选与 `Suffixes`、落点、改写、`Linktexts`；样例测试全过。
4. A4 linking 的 app 与表：迁移 `00028`，`Resolution.Asset` 进出表，`Views`，补全的类型，`AssetLinks`，属性链接的 `Kind`；`ResolveLinks` 在 A 里跳过附件；随机测试加附件的操作。
5. A5 契约、asset 的 `link`、组合根、前端的最小改动；整个程序的测试与 `checkLinks`。
6. 反向对照、审查、修复核对、合并、main 上的文档。

**B**（分支 `m7-p3b`，A 合并之后）：

1. B1 平台：`Images`、`Expires`、`View`、`CheckHTML` 的 `src`、放大与病态输入。
2. B2 obsidian：`Target`、`Assets`、附件的标记、尺寸与说明、媒体的上限、属性表。
3. B3 asset 的 `NewEmbeds`；linking 的 `ResolveLinks` 带类型、属性链接的 `url`。
4. B4 page 的 `assets_expire_at`、契约、前端的属性。
5. B5 组合根、最后一跳、应用的三种模式与 `CheckSize`；渲染样例的媒体记法与 `verify-render.mjs`。
6. 反向对照、审查、修复核对、合并、main 上的文档。

## 7. 测试与验证

- 每部分合并之前：`make lint-web knip test-web build-web`、`golangci-lint`、`make test-go`、`make gen-check`、`make e2e`（本机的 Docker 起不来时以分支的 CI 为准，写进结果）、`node tools/md-fixtures/check.mjs`。
- **Obsidian 的核对**：只在草稿目录里、各自的数据目录起 Obsidian（`open -n -g -a Obsidian --args --user-data-dir=… --remote-debugging-port=…`），按进程号只停自己起的那个；不碰负责人的库与正在运行的 Obsidian；核对脚本只清空自己准备的目录，写库的先核对连的是草稿库。A 跑 `verify-resolve.mjs`、`verify-rename.mjs`，B 跑 `verify-render.mjs`；M6 的四套样例同时重跑一遍，确认没有回退。
- **反向对照**（`mut.py`）：读法的三行各去掉一行；没有扩展名的附件进候选；`Suffixes` 不分类型；`resolved_asset` 不写或不读；`Subtree` 漏掉附件；Markdown 链接给附件加 `.md`；显示文字照 `.md` 去；`Written` 数了别的类型；落点不答 `target_is_asset`；`ResolveLinks` 在 A 里不跳过附件；B 里：钩子不认领、`data-nw-node` 写给附件、媒体上限差一、尺寸的范围、链接里写出 `<audio>`、`Assets` 为空时写成页面的链接、`Expires` 取最晚、组合根交空。
- 审查：每部分三位 Opus 审查者并行（服务端的正确性、渲染与安全、测试与文档），然后 Opus 修复核对，直到一轮没有行为问题。

## 8. 完成标准

1. 第 2 节的每一条都有 `obsidian-verified` 的样例并与 Obsidian 1.12.7 一致，`nerve-defined` 的写明差异与理由；M6 的样例没有回退。
2. 附件的新建、改名、移动、删除之后索引等于重建（随机测试与整个程序的 `checkLinks`）；改名、移动附件时链接照 Obsidian 改写。
3. 落点对读作附件的目标答 `target_is_asset`；补全与附件的答复带类型与 `link`。
4. 每种写法指向附件时的渲染有测试与样例，HTML 里从不出现指向附件的 `data-nw-node`；`CheckHTML` 只许 `img`、`audio`、`video` 的地址是本站的路径，应用的测试核对它们是附件内容的签名路径；`CheckSize` 有附件的放大输入。
5. 整个程序上的最后一跳：解析、索引、改写、渲染、属性，各在组合根交空时失败。
6. 总体设计 13.1 第 31 条、13.3 第 2、4、6 条随实现改写；移交 M6-links 第 1–3、6 项（e2e 部分随 P4）与 M4-extensions 第 1、3 项（服务端）写明落实。

## 9. 结果

### 9.1 A：解析、索引与改写（2026-10-09，合并 `5138ad6`）

- 提交：
  - 实施：样例 `b45e9a3`、page 的读端口 `acb9208`、linking 的领域规则 `1264619`、索引与读 `7d4dbc0`、asset 的 `link` `7cee2c4`、整个程序的测试 `d551e4d`、前端 `1a3a715`、契约的生成物 `44a3eb5`。
  - 负对照的补测 `fecde45`；CI 首轮的修补 `5af8112`（schema 的约束名、属性的三次扫描、附件的列表按父节点、`format` 把布尔写成 `t`、权限矩阵的补全）。
  - 审查的修复 `4cc138b`；两轮修复核对的修复 `4ea331e`、`99a279e`、`852aac1`。合并 `5138ad6`。
- 审查：三位审查者（Opus），没有高、中。
  - A（领域规则与索引）：中低 1（没有扩展名的附件的 `link` 可能引向同路径的页），低 7。
  - B（应用层、接口、asset 与组合根）：CI 必挂的测试期望 3，中低 2（附件的 `link` 读同键的全部节点；契约没写附件的改名、移动会改写与被拒），低 8。
  - C（测试、前端、样例与文档）：CI 必挂的测试期望 2，中低 1（`checkLinks` 比设计说的弱），低 10。
  - 修复核对两轮，都没有行为问题；第一轮的两条性能建议（同键的计数数到 2、补全对附件按计数写）照做。逐条见[审查记录](reviews/P3A-assets-links-review.md)。
- 审查之后改了的设计（第 4 节已改写）：没有扩展名的附件 `link` 为 null、补全不列它；附件的 `link` 由一条语句的路径与计数写出（`AssetLinktext`）；落点的第二种 `target_is_asset`；down 迁移先把指向附件的链接置为解析不到；契约里改名、移动的说明。
- 与 Obsidian 1.12.7 的核对：实施时一次；审查修复（核对脚本的 PNG 换成合法的）之后重跑，`verify-resolve` 27 例、`verify-rename` 46 例，`obsidian-verified` 全部一致，差异都是 `nerve-defined` 的（解析 7 处，改写 20 处）。都在隔离的数据目录里，用完按 PID 关掉。
- 样例集：`78 cases, 46 rename cases, 27 resolution cases, 46 render cases`。
- 反向对照：本机 44 个（实施 35，其中 2 个等价：标题不以点开头、附件多给的 `WrittenKeys` 只多读节点；修复 7；核对后 2），都被抓到或说明；数据库上的 7 个由三条临时分支在 CI 上跑（`SetResolutions` 不写、page 的路径丢类型、三处读不读、`Subtree` 漏附件、组合根丢类型），都被抓到，分支已删。
- CI 与发布：分支的 CI 在 `4cc138b`、`852aac1` 上全部通过（server、web、image、e2e；`image` 一步跑 `make image-smoke`）。本机的 Docker Desktop 起不来，数据库的测试、e2e 与合并之后的 `image-smoke` 都由 CI 跑。
- 交给 B 与后面的：
  1. B：`linking/adapter/markdown/views.go` 的 `Resolve` 不再跳过附件，换成带类型的答复（5.3）；`TestAnAttachmentsNameThroughServe` 里"阅读视图把三条链接写成未解析"的断言随之改成附件的标记；属性链接的 `url`（5.6）。
  2. P4：粘贴插入用 `Asset.link`，为 null 时（没有扩展名）不插入嵌入；补全里附件的标记与排序。
  3. 补全里很多同名的页仍是平方的代价，见 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 8 项；附件已是线性。
- 负责人可以改判的取舍：没有扩展名的附件 `link` 为 null、补全不列（也可以照完整路径给出并在契约里写明可能引向同路径的页）；改名、移动附件时引用它的页正在编辑就整个拒绝（与页面相同）；links 事件的 `targets` 含附件的 id；A 期间指向附件的链接在阅读视图里是未解析的，未解析对话框对它说"这个名称是附件"并重读视图。

### 9.2 B：渲染

（合并之后填写。）
