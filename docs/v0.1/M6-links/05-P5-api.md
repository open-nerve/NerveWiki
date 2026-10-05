# M6/P5 接口：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P5 接口 |
| 状态 | 设计定稿，实施中 |
| 基线 | P4 合并之后的 main（`5fcf7b9`）；本文提交之后开分支 `m6-p5` |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.8、4.11、第 5 节、第 7 节 P5；[总体设计](../v0.1-design.md) 6.1、13.1 第 3、4、8、11、13、27 条，13.4 第 6 条；[P3 文档](03-P3-index.md) 3.2（索引表）、6.5（阅读视图即时解析）；[P4 文档](04-P4-rewrite.md) 3.1（写法） |

---

## 0. 梳理

作者先请两位 Opus 只读梳理（2026-10-05）：

- 一位读接口一侧：模块怎样露出 HTTP（模块根的 `New`、生成、组合根）、access 的规则表与权限矩阵、游标分页的约定、契约测试。
- 一位读数据一侧：索引表与它们的索引、反链的上下文怎样取、标签与属性的查询、补全数据的来源与开销。他在自己起的 PostgreSQL 18 里造了两个各 10,000 页、共 40 万条链接的笔记本做测量。

下面的取舍据此定稿。与总设计的出入在第 10 节。

## 1. 范围

P5 做：

- 五个读接口：反链、页面属性、标签、标签下的页面、链接目标（第 2–6 节）。
- `linking` 模块根的 `New`、`Register`、`Actions`；access 的规则表加五个动作；权限矩阵各一行（第 7 节）。
- 契约 `api/modules/linking.yaml` 与生成；迁移 00026（反链分页与属性链接的索引，第 8 节）。
- 页面模块给 linking 的读端口加两项（第 7 节）。

不做：

- 前端：右栏（P7）、标签页与属性表里的链接（P6）、补全（P7）。P5 只重新生成前端的类型。
- 事件：`links` 已有（P3）；哪个事件让哪个接口重读，由 P6、P7 接上（总设计 4.8）。
- 没有进索引的页的即时计算：这些接口都按索引回答（第 9 节"过时"）。

## 2. 接口

| 方法与路径 | 操作 | 动作 | 回答 | 问题码 |
|---|---|---|---|---|
| `GET /api/v0/pages/{page_id}/backlinks` | `listBacklinks` | `backlink.list` | `{data: [{id, count, contexts}], next_cursor}` | `bad_request`、`page.not_found`、`validation_failed` |
| `GET /api/v0/pages/{page_id}/properties` | `getPageProperties` | `page_property.read` | `{valid, properties: [{key, value}], links: [{key, node_id}]}` | `page.not_found` |
| `GET /api/v0/notebooks/{notebook_id}/tags` | `listTags` | `tag.list` | `{data: [{tag, count}]}` | `notebook.not_found` |
| `GET /api/v0/notebooks/{notebook_id}/tags/{tag}` | `getTag` | `tag.read` | `{data: [{id}]}` | `notebook.not_found` |
| `GET /api/v0/notebooks/{notebook_id}/link-targets` | `listLinkTargets` | `link_target.list` | `{data: [{id, kind, name, link, aliases}]}` | `notebook.not_found` |

**权限**：五个动作都是笔记本级、三种角色都可以（`readers()`），所以不会答 403。

**次序**（13.1 第 4 条，照 `listNotebookAuditEvents`）：

1. 游标的格式（只有反链）：400 `bad_request`。
2. 页面的路由：
   - 页面没有、已删除、不是页面，或它的笔记本已删除，答 `page.not_found`；
   - 调用者在那个笔记本里没有角色，同样答 `page.not_found`（不暴露存在）。
3. 笔记本的路由：笔记本没有、已删除，或调用者在它里面没有角色，答 `notebook.not_found`。
4. `limit`（只有反链）：1–100 之外，答 422 `validation_failed`。默认 50（`shared.PageSize`）。

这些码都已有前端的文案，不加新码。

**契约**：

- 新文件 `api/modules/linking.yaml`，`api/openapi.yaml` 加 `linking` 标签与五条路径。
- 模块文件之间不互相引用，所以 `NotebookID`、`PageID`、`Problem` 的响应与安全方案照 `page.yaml` 逐字节抄过来（同名同内容的在 `api/dist` 里合并）。
- 种类的枚举另起名字 `LinkTargetKind`（`[page]`），不与 `page.yaml` 的 `NodeKind` 同名：M7 改其中一个时不会撞。
- 属性的 `value` 是任意 JSON：它不写 `type`，因为"对象都要 `additionalProperties: false`"那条规则只查写了 `type: object` 的。
- 读都不开事务，不取锁（总体设计 8.3），只有一条语句的读是一个快照。

## 3. 反链

**哪些链接**：

- `page_links` 里 `resolved_id` 是这一页的行：
  - 正文、属性、`aliases` 的值、注释里的；
  - 嵌入与图片；
  - 有歧义、解析到这一页的（索引记着胜出的那一页）。
- 不算这一页自己的链接（`[[#标题]]` 之类在右栏里只是噪声）。
- 出发页都在同一个笔记本里（链接只在笔记本内解析），一次笔记本的判定就够，反链不会跨权限。
- 删除的页不在索引里，所以不会出现。

**分页**：

- 按出发页的 id 升序，游标是上一页最后一个出发页的 id（`{"v":1,"p":"<id>"}`，13.1 第 27 条）。
  - 页面的 id 是 UUIDv7，所以大致是出发页新建的先后。
  - 读 `limit + 1` 个出发页，多出来的那个只说明还有下一页。
- 名称由前端取自已加载的树（总设计 4.11）。

**一项**：`{id, count, contexts}`

- `count`：这一页里指向它的链接数。
- `contexts`：按位置取前 10 条链接，同一行的只留一处，每处是那一行的文字。
  - 一页可以写上百万条指向同一页的链接（P2 的测量：内容的约 20 倍），所以设上限。
  - `count` 让右栏可以写"还有 N 处"。

**上下文**（`domain.Context(content, start, end)`）：

- 行：链接目标的起点所在的那一行。行在 `\n`、`\r` 处断开（CommonMark 的三种行尾）。
- 不超过 240 字节的行原样给出。
- 更长的行取一个不超过 240 字节、含链接起点的窗口：
  - 链接整个在行的前 240 字节里时，从行首取；
  - 否则窗口从链接之前约 80 字节处开始，但不越过行尾之前 240 字节，也不早于行首。
  - 窗口的两端挪到 UTF-8 字符的边界上（起点向后，终点向前）。
  - 截掉的一端加 `…`；`…` 不算在 240 字节里。
- 给出的是原文，不渲染：
  - 表格行、引用与 callout 的 `>`、列表的标记都照原样；
  - 属性链接的行是 YAML 的那一行（如 `  - "[[x]]"`）；
  - 引用式的 Markdown 链接，范围在定义里（样例集规则 8），所以上下文是定义那一行 `[ref]: x.md`。
- 结果复制一份（`strings.Clone`），不切在整页的内容上：否则每个出发页的整页内容（至多 5 MiB）都留到回答写完。

**读法**：

1. linking 的一条语句（一个快照）：
   - 这一页之后的 `limit + 1` 个出发页，在新的索引上是一次只读索引的范围扫描（第 8 节）；
   - 每个出发页的 `revision`（`indexed_pages`）、链接数、前 10 条链接的范围。
2. 逐个出发页读它的内容与修订（页面模块的 `Content`），切出上下文，再丢掉内容。
   - 一次只有一页的内容在内存里，峰值同 `getPageContent`。
   - 不用 SQL 切字节窗口：要在 SQL 里把整个值转成字节，测量里并不更快，UTF-8 的边界仍要在 Go 里切。
3. 内容的修订与第 1 步的不同（两步之间有写入），或那一页已删除：这一项照样给出，`contexts` 为空。
   - 随后的 `links` 事件（`targets`）让右栏重读。

**开销**：

- 第 1 步与内容无关。测量里一个有 3 万行反链的页，最后一页的读取在原来的索引上要扫 20 万行别的笔记本的行（12 ms），在新索引上是 0.12 ms。
- 第 2 步与出发页内容的字节数成正比：最坏是 100 个 5 MiB 的出发页，约 500 MB，耗时写进第 13 节。
- 不走解析的预算：只读不解析。

## 4. 属性

**一条语句**（一个快照）读 `indexed_pages`、`page_properties` 与属性链接：

- `valid`：`frontmatter_valid`。没有 frontmatter 的页是 `true`；无效的 frontmatter 没有属性，也没有属性链接。
- `properties`：顶层的键，按写的次序（`position`），`value` 是索引里的 JSON。
  - 嵌套对象的键序不保留：提取时（`jsonOf`）已经按键排序，jsonb 再按长度排序。
  - Obsidian 的属性面板不编辑嵌套对象，右栏把它显示为 JSON。所以不为此改提取、不重建索引。
- `links`：按位置给出属性里的每条链接，`{key, node_id}`。
  - `key` 是属性的完整路径，列表的项是 `sources.0`、`sources.1`：一个值整个是一条链接才算属性链接，所以一条路径至多一条。
  - 解析不到的也给出，`node_id` 为 `null`，右栏照阅读视图显示为未建的链接。
- 已知的限制（P3 3.2 已写，罕见，不改）：
  - 空键 `"": "[[x]]"` 的属性链接在索引里与正文的链接分不开，不给出；
  - 键里有 `.` 时路径可能相撞（`{"a.b": …}` 与 `{a: {b: …}}`），两条都给出。
- 页面没有进索引（升级之后、`nervewiki reindex` 之前）：答 `{valid: true, properties: [], links: []}`。

属性链接只在 frontmatter 里，位置最靠前，但页的主键 `(source_id, range_start)` 上的扫描仍要读完这一页的每一行。所以迁移 00026 加一个部分索引（第 8 节）。

## 5. 标签

**`listTags`**：

- 笔记本里每个标签键一项，按键的字节序。
- `tag`：这个键在各页里最常见的写法，一样多时取字节序最前的。不同的页可以写 `#Project` 与 `#project`。
- `count`：有这个标签的页数。
- 不合成父标签：只用了 `a/b` 时没有 `a` 这一项。父标签的页数不能由子标签的页数相加（一页可以同时有 `a/b` 与 `a/c`）；补全按前缀过滤就够。
- 不分页，同 `listNodes`：标签数受笔记本内容的约束。

**`getTag`**：

- 有这个标签或它下层 `tag/…` 的页的 id，按 id，不分页。
- `{tag}` 按提取的同一条规则变成键：
  - `obsidian.CountedTag`（去掉末尾一个 `/`，拒绝的字符）；
  - `shared.TitleKey`；
  - 不超过 `MaxKey`。
- 不是合法 UTF-8、含 NUL，或者不是标签的：答空列表，不查询。
  - PostgreSQL 的文本存不下这些字节，契约的测试（`TestFreeTextParametersDoNotAnswer5xx`）会发过来。
  - 次序在笔记本的判定之后：笔记本不可见时仍答 404。
  - 不加新码：它是筛选的条件，不是地址里的资源。
- 嵌套的标签在路径里写作 `%2F`（`a%2Fb`）：Go 的路由解出 `a/b`，openapi-fetch 用 `encodeURIComponent`。
- 查询：`tag_key = k OR (tag_key >= k || '/' AND tag_key < k || '0')`。`'0'` 是 `/` 之后的那个字节，`tag_key` 是 `COLLATE "C"`，所以范围正好是 `k/…`。不用 `LIKE`：`_` 可以在标签里，是 `LIKE` 的通配符。

## 6. 链接目标

**一项**：`{id, kind, name, link, aliases}`

- `kind`：v0.1 只有 `page`（M7 加附件）。
- `name`：标题。
- `aliases`：索引里这一页的别名，按键排序（写的次序没有存）。没有进索引的页没有别名。
- 按 id 排序，不分页，同 `listNodes`。

**`link`：与改写同一个函数**

- P4 的 `relink` 写 wikilink 时依次试三种写法，取第一个"在树里只解析到它"的：
  - `Written`：名称在笔记本里唯一时是名称，否则是完整路径；
  - 完整路径；
  - 完整路径加 `.md`：只对以 `.md` 结尾的标题有用，如 `x.md` 旁边有 `x` 时。
- P5 把这一选择提出来，成为导出的 `domain.Linktext(n, from, tree)`。`relink` 写 wikilink 时调它，`listLinkTargets` 也调它。
  - `listLinkTargets` 的树：笔记本里每一页，按标题键分组（`Tree.Named`）。
  - `from` 是根。这三种写法都不是相对的，解析不依赖出发的位置；随机测试从每一页核对（第 9 节）。
- 兄弟节点的标题键唯一，所以完整路径总能只解析到它，`link` 总是有。
- 前端不重算标题键：JavaScript 没有完整的大小写折叠（总设计 4.11）。

**开销**：

- 页与路径：页面模块的新端口，一条语句读整个笔记本的节点，在 Go 里拼路径（第 7 节）。
- 别名：一条语句，按 `(notebook_id, alias_key)` 的索引。
- `Linktext`：名称唯一时是常数；同名的一组里，每页要在组里核对一次，所以是组大小的平方。
  - 一万页同名（如每个文件夹里都有 `index`）的耗时写进第 13 节，并有成本测试（第 9 节）。
- 回答：一万页约 2 MB 的 JSON，同整棵树（总体设计的风险表已把一万页以上的树交给 M12）。

## 7. 模块与组合

**linking 模块根**（13.1 第 11 条）：

- `Deps`：连接池、授权者、笔记本的读（`WorkspaceOf`）、页面的读（`Pages`、`PageContents`）。
- `New(Deps) *Module`、`Register`、`Actions()`。
- `New` 只在 HTTP 那一侧：命令行的组合（`Reindex`）不经过它，组合检查（`archtest`）照旧通过。

**动作**（`linking/domain/actions.go`）：

- `backlink.list`、`page_property.read`、`tag.list`、`tag.read`、`link_target.list`。
- 规则表各一行，`{Level: LevelNotebook, Notebook: readers()}`；`bootstrap/actions_test.go` 的并集加上 `linking.Actions()`。

**错误**：

- `linking/domain/errors.go` 加 `ErrPageNotFound`（码 `page.not_found`）与 `ErrNotebookNotFound`（码 `notebook.not_found`）。
- 码的前缀可以是别的模块（`apitest` 的 `codeModules`），页面模块的 `notebook.not_found` 已是先例。

**用例**（`linking/app`）：`ListBacklinks`、`GetPageProperties`、`ListTags`、`GetTag`、`ListLinkTargets`。

- 页面的路由：
  1. 页面的笔记本：新端口 `Pages.NotebookOf`；
  2. 笔记本的工作区：`WorkspaceOf`，只认没删除的笔记本；
  3. 判定。
- 不用索引找页面的笔记本：没进索引的页会被错答成 404。

**页面模块的端口**（`page.LinkTargets`，经 `bootstrap/linking.go` 转换）：

- `NotebookOf(ctx, id) (uuid.UUID, bool, error)`：没删除的页 `id` 所在的笔记本。
- `All(ctx, notebookID) ([]LinkNode, error)`：笔记本里没删除的每一页，带从根起的路径。
- `Content` 加一个返回值 `ok`：页已删除时不再是错误。
  - 改写与 `reindex` 照旧把它当错误：它们在锁里读，页不会不见。
  - 反链把它当作"上下文为空"。

**HTTP**：

- `linking/adapter/http`：`handler.go` 每个操作一个用例接口，`gen/` 照 `page` 的配置。
- 处理函数只做转换，用例的错误原样返回。

**组合根**：

- `wire.go` 建 `linking.New(linkingDeps(...))` 并 `Register`。
- 权限矩阵：五行，页面的照 `getPageContent`，笔记本的照 `listNodes`。
  - `targetViolation` 认得 `{tag}`：它不是目标，目标是它前面的 `{notebook_id}`。
  - 不用 `notTargets`，那会连 `{notebook_id}` 一起跳过。

## 8. 迁移 00026

`00026_linking_backlinks_index.sql`：

- `page_links (resolved_id, source_id, range_start) WHERE resolved_id IS NOT NULL`，代替 `page_links_resolved_id_idx`。
  - 观察者的 `LinksReached`（`resolved_id = ANY(…)`）用它的前缀，照旧走索引（实现时用 `EXPLAIN` 核对）。
- `page_links (source_id, range_start) WHERE property_key IS NOT NULL`：属性链接。
- 加进 `sqlc.yaml` 的 linking 模式列表。只加索引，不改提取，所以不升 `Extractor`，不需要 `reindex`。

## 9. 测试

**领域**（`linking/domain`）：

- `Context` 的表格测试：短行、长行的三种窗口、多字节字符落在边界上、`\r\n` 与单独的 `\r`、首行与末行、空行里的链接不存在（范围总在某一行里）。
- `Context` 的模糊测试，不变量：
  - 去掉 `…` 之后是这一行的一段，不超过 240 字节，是合法的 UTF-8；
  - 含链接的起点；
  - 行不超过 240 字节时就是整行；
  - 结果不与内容共享内存。
- 游标的编码与解码（只认一种写法）。
- `Linktext`：
  - P4 的测试照旧通过（`relink` 改为调它）；
  - 随机测试：随机的多层、重名、`.md` 结尾的树，每一页的 `Linktext` 从每一页解析都只到它；
  - 成本测试：一万页同名、在一万个文件夹里，算出全部 `link` 在 1 秒之内（不带竞态检测，照 `cost_test.go`）。

**存储**（`linking/adapter/postgres`，testcontainers）：

- 反链：
  - 跨页的出发页、正好满的最后一页（`next_cursor` 为 `null`）、不算自己；
  - 每个出发页至多 10 条范围、`count` 是全部；
  - 有歧义的算、别的目标的不算。
- 属性：次序、`valid`、解析不到的链接、没有进索引的页。
- 标签：最常见的写法与一样多时的次序、页数、前缀（`a` 有 `a`、`a/b`，没有 `ab`、`a0`、`a.b`；`a_b` 不当作通配）。
- 别名按页、按键。
- 新索引：反链的查询与 `LinksReached` 用上它（`EXPLAIN`）。

**用例**（`linking/app`，替身）：

- 错误的次序：游标、页面或笔记本、`limit`；不可见答 404。
- 修订不同或页已删除时 `contexts` 为空。
- 内容一次只读一页：替身记着同时持有的内容。
- `getTag` 的非法输入不查询。

**HTTP**（`linking/adapter/http`）：每个操作的成功与每个声明的码（`apitest`）。

**整个程序**（`bootstrap`）：

- 经 HTTP 建页、写正文，读五个接口：
  - 反链的分页与上下文（中文的长行）；
  - 属性与它的链接；
  - 标签与嵌套的 `%2F`；
  - 链接目标的 `link`（同名时是路径）与别名。
- 改名之后反链跟着变（索引是 P3、P4 的，这里只核对接口读到的是它）。
- 权限矩阵五行；动作的并集；契约的路由与操作一致；自由文本参数不答 5xx。
- 组合根不登记 linking 的 HTTP 时，路由测试失败。

**测量**（写进第 13 节）：

- 反链一页 100 个出发页、每页 5 MiB 的耗时与 Go 堆的峰值；
- 一万页的笔记本，`listLinkTargets` 的耗时与回答的大小。

**负对照**：每步的关键分支做变异，记在第 13 节。

## 10. 与总设计的出入

（本文定稿，总设计随 P5 的合并修订。）

- **4.11 反链**：
  - 一项是一个出发页，`{id, count, contexts}`；
  - 上下文取前 10 条链接、同一行只留一处；
  - 长行取含链接的窗口，截掉的一端加 `…`；
  - 不算这一页自己。
- **4.11 补全的数据**："最短写法"写明为 P4 3.1 的三种写法（`Linktext`）：不试路径的后缀，所以不总是最短。
- **4.11 属性**：`links` 的 `key` 是完整路径，解析不到的也给出（`node_id` 为 `null`）。
- **第 5 节**：
  - `getTag` 答 `{data: [{id}]}`，不是标签的输入答空列表；
  - `listTags` 不合成父标签，`tag` 是最常见的写法；
  - `listLinkTargets` 的 `kind` 枚举另起名字。
- **P3 文档 3.2**：迁移 00026，反链按出发页分页的索引与属性链接的部分索引。
- **P3 的移交**："嵌套的键序在 jsonb 里丢失"写准：提取时已经丢了。P5 不保留，理由见第 4 节。

**负责人可以改判**（作者的判断）：

- 每个出发页至多 10 条链接的上下文，其余只计数（第 3 节）。
- 长行的窗口从链接之前约 80 字节开始（第 3 节）。
- 不算这一页自己的链接（第 3 节）。
- 没有进索引的页按索引回答，不即时解析，不报告覆盖率（第 4、9 节）。
- `getTag` 的非法输入答空列表，不加 404 的码（第 5 节）。
- 嵌套属性的键序不保留（第 4 节）。

## 11. 实施步骤

| 步 | 内容 |
|---|---|
| S1 | `linking/domain`：`Linktext`（`relink` 改为调它）、`Context`、反链的游标、动作、错误；表格、模糊、随机与成本测试 |
| S2 | 迁移 00026；linking 的查询：反链、属性、标签、标签下的页、别名；存储的测试 |
| S3 | 页面模块的端口：`NotebookOf`、`All`、`Content` 的 `ok`；改写与 `reindex` 随之改；`bootstrap/linking.go` 的转换 |
| S4 | 用例与它们的测试 |
| S5 | 契约 `linking.yaml` 与 `openapi.yaml`；`make gen`；HTTP 适配器与它的测试；模块根的 `New`；access 的规则表；组合根 |
| S6 | 权限矩阵与 `{tag}`；整个程序的测试；测量 |

## 12. 完成标准

- 第 9 节的测试通过；`make check`、`make gen-check`、e2e 全量通过。
- 审查的发现处理完，修复经 Opus 核对，直到一轮没有行为上的发现；`make image-smoke` 通过。

## 13. 结果

（合并时填写。）
