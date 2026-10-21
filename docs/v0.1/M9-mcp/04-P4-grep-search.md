# M9/P4 grep、search、index、backlinks：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M9/P4 grep、search、index、backlinks（两个 Step：S1 page 的 `grep` 与 `search`，搜索的行的 `partial` 与迁移；S2 linking 的 `backlinks` 与 `index`） |
| 日期 | 2026-10-21 |
| 状态 | 设计已审查（三位审查者），照主持者的处置修订四次，复核四轮（前三轮由三位，第四轮由 B、C 与主持者）；审查记录 [P4-design-review.md](reviews/P4-design-review.md)；S1、S2 未开始 |
| 上级文档 | [M9 总设计](00-M9-design.md) 4.1、4.2、4.6、4.7、4.8、4.9、4.13、4.14、4.15、第 5、7、9、10、11 节与"移交的落实"；[P1](01-P1-platform-reads.md) 3.4.4、3.9、3.10、3.13、3.15 与第 8 节；[P2](02-P2-writes-history.md) 3.7、3.10、3.13、3.16 与第 8 节；[P3](03-P3-batch-revert.md) 3.10.1、3.12、3.13、6.5 与第 8 节；[v0.1 总体设计](../v0.1-design.md) 5.5、6.1–6.4、13.1、13.2、13.4；[M8/P5](../M8-history-search/05-P5-search.md) 3.2–3.9；移交 [M8/P5 grep 用搜索的索引](handoffs/M8-P5-search.md)第 1、3、4 条、[M6/P4 链接改写](handoffs/M6-P4-rewrite.md)第 4 条 |
| 代码位置的写法 | 同 P1–P3：模块写作 `page/app/search.go:87`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；迁移写作 `00041_page_search_scope.sql:90`（`server/migrations/sql/`）；e2e 写作 `e2e/fixtures/mcp.ts:114`；Go 的标准库写作 `regexp/syntax/parse.go:212`（go1.27.1 的工具链，`~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.1.darwin-arm64/src/` 之下，与仓库 `server/go.mod` 的 `toolchain go1.27.1` 相同）。代码的行号是 `56ef34c8` 的（P3 的 S3 合并），文档的行号是 `HEAD` 的提交 |
| 实验 | 全部离线、不要数据库，在 `scratchpad/m9/p4/exp/` 之下（以模块缓存编译，`GOFLAGS=-mod=mod GOPROXY=off GOTOOLCHAIN=local`）：`tok/`（分词照抄 `page/domain/search.go:96-375`，被截掉的页的比例与迁移的筛法，3.3）、`keys/`（键的原型与超集的性质试验，3.5）、`claims/`（Unicode 的几条断言，3.5.3）、`cost/`（指令的估计、编译的大小、每单位的耗时与解析、编译的分配，3.6）、`instr/` 与 `instr4/`（说明的字节，3.14）、`bom/`（BOM、`\r` 的分词类别）；审查之后另有 `keys2/`（审查 C 的 `keysmut` 加可组合的变异、定向的字母表、每种情形的计数与每处修正的反例，3.5.4、6.1）、`bound2/`（每个码位的界加流安全的 CGJ、门槛、跨上限的两行与 240 KiB 的随机 CJK，3.2.2、6.1）、`tokb2/`（审查 B 的 `tokb`：门槛 74,000 时迁移选中的页、字节与词，被标出的页的工作，3.2.2、3.3.4）、`parseq/`（解析的队列下 200 个同时的最坏的解析，3.6.1）；核对之后另有 `lineoh/`（`GrepLineOverhead` 的离线校准：`grep` 每行的固定开销对最贵的指令的每单位的耗时，3.6.4）、`tokb3/`（`tokb2` 加每行的开销作参数，被标出的页的工作，3.3.4）、`instr6/`（说明的第三行改过之后的字节，3.14）；第二轮核对之后另有 `corpus/` 与 `freeze/`（冻结的语料：每个路径的 SHA-256、内容的快照与重算，3.3.4）、`lineoh2/`（类的短的拼法、各种形状的每单位与满负载的估计，3.6.4）、`bpat/`（开头相同的选择被合成一支，3.6.4）；第三轮核对之后另有 `importset/`（以 M7 的导入的规则的副本算第 3 步的预期的集合，3.3.4）与 `freeze` 的每字节的词随页的大小的曲线（`corpus/curve-out.txt`，3.2.2）。审查者的 `review-A/`、`review-B/`、`review-C/` 原样留着。语料只用仓库的 Markdown（`docs/`、README）、Go 模块缓存与 `node_modules` 里的 Markdown，合在一起冻结成一份（3.3.4；主副本在 scratchpad 之外）；M8 的语料已删掉，不下载 |

---

## 0. 分两个 Step

00 号文档第 7 节说 P4 中等。它的四个工具分在两个模块里，两边几乎不相交：`grep`、`search`、搜索的行与迁移全在 page（`page/domain`、`page/app`、page 的存储与 `adapter/mcp`）；`backlinks`、`index` 在 linking（它今天没有 `adapter/mcp`，`linking/module.go:46-57` 的 `Deps` 也没有 `NodePaths`），只经 page 的一个新的模块根类型 `page.NewNodePaths(pool)` 碰 page。所以分两个 Step，各自一个分支、一次审查与修复核对、一次合并，后一个从前一个合并之后的 `main` 开始；每个 Step 里的每个提交都是绿的（第 5 节），各自带它的 e2e 故事。

- **S1 page 的 `grep` 与 `search`**（分支 `m9-p4-s1`）：
  - 搜索的行的 `partial` 与保留的键 `!cut`：迁移 00046（列与检查、只把可能被截过的活着的页批量入队）、分词的版本 5、`SearchTokens` 答出 `partial`、任务写它（3.2、3.3）；**被标出的比例**由设计照规则定下（标出三种截断；在冻结的语料上、`GrepLineOverhead` 离线校准之后算，3.3.4），S1 在任何 `grep` 的代码之前在满负载下量出常量、在冻结的语料上再数一次，作记录与核对；
  - 键：`regexp/syntax` 的树上必须出现的字面串，CJK 二元组、三元组、两边都是边界的 1–2 个字的整词，`(?i)` 与 `ignore_case` 的折叠，NFC 的两端的修剪；超集的证明与性质测试（3.5）；
  - 程序与工作：进程的解析的队列（解析、估计与键，`GOMAXPROCS` 个名额，在调用的 ctx 里等、不答 503）、编译之前的指令数估计（`calcSize` 的副本）、一行与一次的工作、软期限与预先判断、并发的名额与 503、不持连接（3.6）；
  - 用例 `Grep`：码的次序、候选（键的位图、队列里准确的节点、这本笔记本的 `!cut` 页）、glob 在走查之前取交、按路径的键走查、正文按 id 一批一条语句、行号同 `read`、`\r` 与 BOM、`partial_lines`、游标、`building` 与 `incomplete`（3.7）；
  - `Search.InNotebook` 与 `SearchHit.Tier`（3.8）；
  - 工具 `grep`、`search`，`tools.json` 的四个条目，内置的说明的三行，码的表、矩阵、工具的字符串的合法参数（3.10–3.14）；
  - 测试：键的表格与性质测试、`partial` 的独立判定、迁移的筛法是超集（真的库上）、走查与游标、行号与 `read` 一致、每个界线处、块数、计划与时间的守卫（6.1）；e2e：MC6（第 4 节）。
- **S2 linking 的 `backlinks` 与 `index`**（分支 `m9-p4-s2`）：
  - 模块根的 `page.NewNodePaths(pool)`：段的检查、按路径找页（错误照 page 的工具的文字）、按 id 给路径（复用 P2 的 `NodePaths.Paths`，即 `PathStore.PathsOf` 经 `LinkTargetsByIDs`，不另写给路径的语句）、子页；linking 的 `Deps.NodePaths`（3.9.1）；
  - `ListBacklinks.AtPath`（`Execute` 的后半共用，来源页带路径，3.9.2）；
  - linking 的 `ListIndex` 与新动作 `index.list`（规则表一行）：筛选、次序、入链数、字符串的截断与标出、游标；迁移 00047 给 `page_properties` 加 `type` 与 `summary` 两个部分索引，一页的属性一次探查（3.9.3）；
  - linking 的新包 `adapter/mcp` 与 `(*linking.Module).Tools()`、`bootstrap/mcp.go` 拼接 linking 的工具；`tools.json` 的四个条目、说明的两行、码的表、矩阵、合法参数（3.10–3.14）；
  - 测试：`index` 的表格测试、"列表等于逐项读"、入链数与计划测试、`index` 的时间守卫、`backlinks` 的路径与附件、48 KiB 处（6.1）；e2e：MC7（第 4 节）。

为什么不是一个 Step：S1 里有 P4 几乎全部的风险（键是不是超集、迁移的筛法、`grep` 的代价与守卫的实测、被标出的比例的核对），S2 是照已有的用例接线、加一个有界的查询；合在一起的审查要同时读两个模块与四类测试，修复核对的范围也大一倍。两边的 CI 负担也分开落在两次合并上（6.5）。为什么 `search` 在 S1：它与 `grep` 同在 page、同读搜索的索引，MC6 一个故事同时核对两者（"MCP 刚写的立即两者都找到"），内置的说明里两者的分工是一句话。

两个 Step 都在服务端与 e2e；没有网页的改动（00 号文档第 7 节"唯一的网页改动是 P3 的一条失败的文字"）。

## 1. 基线

`main` 在 `56ef34c8`（P3 的 S3 合并）。与 P4 有关的现状：

**搜索的索引（M8/P5）**：

- 行与队列：`page_search`（`node_id`、`revision`、`name`、`tokenizer`、`built_at`、`title`、`body`，`00040_page_search.sql:7-23`），没有 `notebook_id`：范围只经词的前缀（`00041_page_search_scope.sql:1-23`：每本笔记本一个 base 62 加 `#` 的前缀，"No lexeme holds a '#'"）；`page_search_queue`（`node_id`、`notebook_id`、`version`、`bulk`，`00040:58-65`，索引 `(notebook_id, bulk)` 与 00041 的 `(notebook_id, node_id) WHERE bulk`）。
- 分词：`SearchTokens(name, content) (title, body []string)`（`page/domain/search.go:252-261`）：正文是 `searchFold(name + "\n" + content)` 的扫描（`:293-339`），CJK 二元组、字母与数字的连续串的三元组、1–2 个字的整词（`:358-375`）；超过 64 个、全是 ASCII 的连续串不收（`longASCII`，`:351-353`）；长于 64 字节的 `data:` 串跳过（`:344-346` 的 `inDataURL` 只收 ASCII）；词的字节（每个另算 `SearchPrefixBytes` = 7）到 `MaxSearchLexemeBytes` = 1,000,000 就停（`:53-62`、`add` 的 `:272-289`）；`tsvector` 自己的界线 1,048,575（M8/P5 3.3 审查 A 在 PG 18.6 上核对）。版本 `searchTokenizerVersion = "4"`（`:88`），`SearchTokenizer` 另带 Go 与 x/text 的 Unicode 版本（`:94`）。
- 任务：只有名称、版本号与分词都相同的行才留着（`page/app/index_search.go:170-173`），否则照 `SearchTokens` 重建（`:211`）；`nervewiki reindex` 把分词不同的行入队（`queries/search.sql:23-30` 的 `StaleSearchNodes`）。
- 读：`page_search_hits(notebooks, body_keys, title_keys)`（`00041:90-103`）只读 GIN 的位图，每个键按每本笔记本加前缀、以 `page_search_queries` 引成 tsquery 的操作数（`00041:35-46`）；P3 的 S1 起 `batch` 的写批量入队、答复与搜索带 `building`（第 11 节第 44 条，`SearchBuilding`，`search.sql:265-267`）。
- 用例 `Search`：只有按工作区的 `Execute`（`page/app/search.go:87-151`）；次序是游标 400 → `q` 422 → 工作区 → `page.search` → `VisibleIn` → `notebook_id` → `limit` 422（`:88-130`）；`SearchHit` 没有层（`:28-35`），游标由最后一行的层、`SortAt` 与 id 重建（`:242-248`），`ChangedAt` 就是 `SortAt`（`:353`）。`SearchDeps` 有 `Workspaces`（按 slug），没有 `Notebooks`（`:51-61`）；page 的 `Notebooks.WorkspaceOf` 已有（`page/app/ports.go:54-63`）。

**路径与 glob（P1、P2）**：`NodePaths.Find`（每段一次点查）、`Walk`、`Paths`（`page/app/node_paths.go:35-58`，`Paths` 是 `PathStore.PathsOf` 一条语句，P2）；`domain.ParseGlob` 与位并行的 `Glob.Walk`（`page/domain/glob.go:79-123`、`:353-371`），`MaxGlobWork` = 3 亿；`ListByPattern` 的次序是调用者 → 模式 422 → 游标 400 → `limit` 422 → 笔记本（`page/app/list_by_pattern.go:68-124`），读整棵树（`ListNodes`，`queries/nodes.sql:30-34`）、在 Go 里走（`treeOf`、`keyOf`、`pathOf`，`:222-270`）。`PageStates` 与导出的 `Contents` 都按 id 的数组读正文行（`queries/contents.sql:40-49`）。

**linking**：`ListBacklinks.Execute(ctx, id, limit, cursor)`：游标 400 → 页与判定（`page.not_found`）→ `limit` 422（`shared.PageSize`，1–100）（`linking/app/backlinks.go:45-93`）；它的语句一步一步地沿 `page_links_resolved_id_source_id_idx` 找来源页，不读它们的全部链接（`linking/adapter/postgres/queries/reads.sql:1-58`）；`MaxCount` = 1,000、`MaxContexts` = 10、`MaxContext` = 240 字节、`MaxContentRead` = 32 MiB（`linking/domain/context.go:10-26`）。`page_properties`（主键 `(source_id, position)`，另一个索引只有 `notebook_id`；`value` 是 jsonb，`00023_linking_page_properties.sql:5-15`）、`page_tags`（主键 `(source_id, tag_key)`、`(notebook_id, tag_key)`，`00022`）、`indexed_pages`（`node_id` 主键、`notebook_id` 的索引，`00020`）；`TagPages` 含其下的标签（`reads.sql:96-105`）；`GetTag` 的标签键 `TagKey` 对没有页的写法答 `false`、不查库（`linking/app/tags.go:43-56`）。linking 自己的 `PageTree.Paths` 给的是链接的路径（`linking/app/access.go:21-36`），不是 MCP 的路径（页不带 `.md`）。规则表的 linking 一段只有读者的六个动作（`access/domain/rules.go:117-125`、`linking/domain/actions.go:7-27`）。

**MCP（P1–P3）**：page 的 `adapter/mcp` 有 11 个工具（`page/adapter/mcp/tools.go:101-129`），`Fit` 与逐项的游标（`log.go:31-62` 的写法），`toolErr`、`notFound`（`text.go:22-63`）；`tools.json` 两个入口各 10、12 个工具；`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:79-99`）、`mcpRow`（`:27-41`）、`validToolArguments`（`bootstrap/mcp_reads_test.go:672-687`）、NUL 的检查（`:715-760`：不答 `internal_error`、不被 schema 拒）；`TestTheBuiltInInstructionsNameEveryTool` 要每个工具一行 `- <名>:`（`bootstrap/mcp_tools_test.go:175-187`）；`TestTheBuiltInInstructionsAreShort` 守 4 KiB（`page/adapter/mcp/instructions_test.go:15-27`）。说明现在：笔记本入口 3,386 字节（剩 710）、账户入口 3,456（剩 640），`pathsAndTools` 3,074（`TextBytes` 的数法，`exp/instr`）。`bootstrap/mcp.go` 的 `mcpTools` 收 linking 却不用（"linking's come with P4"）。

**CI**：组合根三份 `BOOTSTRAP_FIRST = ^Test([A-D]|E([a-u]|very[A-R]))`、`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])`（`Makefile:217-219`）；P3 的 S3 之后第一份最近两次 6:12、7:03，第二份 5:02–6:00（P3 的 S3 要下一个往它加测试的 Phase 先估计），第三份 3:39–5:18（[P3](03-P3-batch-revert.md) 6.5、9.3），线在约 7:30。变异编号到 136（[P3](03-P3-batch-revert.md) 6.3，S3 的文档加了 104–136），P4 的从 137 起。迁移到 00045。`main` 今天在 `bbd18d82`，只比 `56ef34c8` 多 P3 的 S3 的文档，代码的行号照旧。

## 2. 目标与范围

P4 做 00 号文档第 7 节 P4 一行的全部：

1. **`grep`**（4.13）：Go 的 RE2 按行匹配一本笔记本的页的正文；模式在进程的解析的队列里解析、估计、取键；键取自 `regexp/syntax` 的树，候选只读这本笔记本的倒排、并上队列里准确的节点与这本笔记本里被截掉词的页；glob 在走查之前取交；程序、一行、一次的工作，软期限与预先判断，并发的名额，不持连接；走查与游标、`building`、`incomplete`、`partial_lines`；行号同 `read`。
2. **搜索的行的 `partial` 与 `!cut`**（4.2）：迁移 00046、分词的版本 5、只把可能被截过的页入队；被标出的比例（第 11 节主持者的决定第 12 条：审查之后照主持者的决定离线地量，校准每行的开销之后定下标出三种截断，3.3.4）。
3. **`search`**（4.13，第 11 节第 4 条）：`Search.InNotebook`、`SearchHit` 的层、工具。
4. **`backlinks` 与 `index`**（4.9，M6/P4 的移交第 4 条）：`page.NewNodePaths(pool)` 与 linking 的 `Deps.NodePaths`；`ListBacklinks.AtPath`；`ListIndex` 与 `index.list`、迁移 00047 的两个部分索引；linking 的 `adapter/mcp` 与 `Tools()`。
5. 四个工具的 `tools.json` 条目、内置的说明的五行、码的表、矩阵、工具的字符串的合法参数；块数、计划与时间的守卫（13.4 第 8、9、12 条）；e2e：MC6、MC7。

不做：MCP 之外的搜索的改动（网页的搜索照 M8）；`grep` 的"穷尽"模式（第 11 节主持者的决定第 8 条）；虚拟的 `index.md`（M10，复用 `ListIndex`，第 8 节）；接入文档与人工清单（P5）。

### 2.1 对 00 号文档的修订

P4 照实改 00 号文档的这些处（行号是 `HEAD` 的 00 号文档的，`bbd18d82`；`apply-00m9.py` 随这份设计的提交落下它们，第 7 节）。

1. **解析的代价与解析的队列**（4.13 `:318` 的"只解析约 35 KB"、`:323` 的名额；第 10 节 `grep` 占 CPU 的一行 `:616`；"写给后面的 M"的性能一条 `:467`）：解析在笔记本的判定与名额之前没有界线，而 1,024 字节之内最贵的解析远不止 35 KB：许多个分开的 `\pL` 一次分配约 7.25 MB，折叠的大范围一次约 52 ms 的 CPU（设计审查 A 的实测与队列下的数在 3.6.1，这份设计只在那里写全）。**规则**：解析、指令的估计与取键在一个进程的**解析的队列**里做（`semaphore.Weighted`，`GOMAXPROCS` 个名额，同解析的预算的写法）；调用在它自己的 ctx 里等，队列从不答 503，所以 4.15 的码的次序（422 在 404 之前、404 在 503 之前）照旧，答什么不随负载，只有快慢随它；ctx 在等的时候结束的调用同任何过了期限的调用：mcpserver 答 `internal_error`，文字是"the call ran out of time"、warn 级记"MCP call deadline exceeded"，客户端走了的是"the client went away"、debug 级（`platform/httpserver/mcpserver/errors.go:75-99`）。内存于是至多名额数乘最坏的一次解析（约 7.3 MB），CPU 至多名额数。编译照旧在拿到 `grep` 的名额之后（同一个模式再解析一次，在名额里）。00 号文档 4.13 写这些数（没有界线时 200 个同时的调用的峰值 +242 MB，每棵树多留 20 ms 时 +754 MB，审查 A2-n2），第 10 节、"写给后面的 M"与 M12 的性能移交引 4.13（第 7 节）。
2. **1–2 个字的整词**（4.13 `:325` 的"1–2 个的整词不取"）：改为"只在模式本身给出两边的边界时取"：边界是一个分词里属于"别的"一类的字面字符（标点、空白、`#` 一类）、一个 CJK 的字面字符（分词里 CJK 结束字母与数字的串：`中ai中` 给出 `ai`，审查 A2-n3）、一个每个字符都属于"别的"的字符类（`\s`、`[ ,.]`）、`^`、`$`、`\A`、`\z`；`\b`、`\B` 从不算边界（4.13 的理由照旧：RE2 的 `\b` 只认 ASCII 的单词字符）。所以 `" ai "`、`^ai$`、`#ai,` 取得出 `ai`，`\bai\b` 取不出。M8/P5 的移交第 1 条的"1–2 个的只在两边必是边界时给整词"由此兑现（3.5.2）。
3. **键的两端**（4.13 `:325`，补）：键按每个字面字符各自折叠、连起来再 NFC 算（与分词的 `searchFold` 对整段正文算可以不同的那几处，3.5.3）；一段字面串开头的、NFC 下前面没有边界的字符（组合记号一类）去掉；右边不是已知的边界时，最后一个组合序列在它可能与后面未知的字符合成时去掉：它的起始字符（`(?i)` 下轨道里任何一个）在 96 个"危险的结尾"里（如 `H`：`H` 加 U+0331 在 NFC 里不合成，折叠成 `h` 之后合成 U+1E96），或折叠里有前面没有边界的码位，或它之后的字符折叠之后变了（NFC 的文字里只有 U+0345：`(?i)` 的 ι 的轨道含它）。这些都只是少取键，仍是超集（3.5.4 的证明）。在 U+0345 的轨道处把字面串切开的一步不要（与最后一条各自多余，两条都去掉时有反例，3.5.4）。4.13 的"解析、化简"改为取 `Simplify` 之前的树（同指令数的估计）：`Simplify` 只展开重复，事实不变，而它分配。
4. **一行的工作的数**（4.13 `:324` 的"上限约 3 × 10⁸ 再留 CI 约 2 倍的慢"与"2,000 条指令的模式于是只匹配一行开头的约 150 KB"）：两句不一致，取前一句：`MaxGrepLineWork` 暂定 1.5 × 10⁸（离线：3.6.4 的 36 支的选择在长行上每单位 8.6 ns，`\pL` 与大的字符类 7.9–8.1 ns，1.5 × 10⁸ 单线程约 1.3 秒），2,000 条指令的模式于是匹配一行开头的约 75 KB，10 条指令的匹配得完 5 MiB 的一行（15 MB）。`MaxGrepWork` 暂定 4 × 10⁸。两者的终值由 P4 照 3.6.4 在满负载下实测：S1 的第 1 步离线、第 9 步以真的用例，一次的界线照较慢的形状（每行 4 KiB，审查 A3-m1）；离线的满负载在能效核上的 p99 指向 2 × 10⁸ 与 0.75 × 10⁸（3.6.4 的表）。离线的每单位的耗时（Apple M5 Max，GOMAXPROCS 1，主机负载 1.4–3.8）：字面量 4.3、`.` 4.2、`[a-z]` 4.6、`(?i)` 的字面量 5.4、`\pL` 7.9、`[\p{L}\p{N}\p{M}\p{S}\p{P}]` 8.1、选择 4.9、嵌套的星号 2.7 ns；CJK 的正文上 `\pL` 与大的字符类按字节 2.9–3.0 ns；1,000 条指令的 `[a-z]{999}` 与大的字符类的每单位同 200 条的（4.7、8.0 ns）。设计复核的"`.` 约 4 ns、`[a-z]` 约 5 ns、大的字符类 8–9 ns"相符。
5. **BOM**（4.13 `:315` 的"行尾的 `\r` 不参与匹配、也不在答复里"，补）：第 1 行开头的 BOM（U+FEFF）同样不参与匹配、不在答复的文字里（ripgrep 默认也去掉它；否则 `^# ` 在带 BOM 的页的第一行上找不到）；答复的文字不从这一行的第一个字节起时（BOM、截取）带 `column`：文字的第一个字节在这一行里的位置，与 `read` 的 `column` 同一种数法，`read(offset=n, column=c)` 从那里读起。BOM 与 `\r` 在分词里都属于"别的"（`exp/bom`），键的边界照旧。
6. **截断处的命中**（4.13 `:319` 的"恰好结束在截断处的命中丢掉"，写细）：一行被截短时，最左的命中结束在截断处，这一行进 `partial_lines`、不算命中，不论它是不是真的命中（`foo` 恰好在截断处结束也是）：agent 由 `read` 读到整行；结束在截断处之前的命中是整行的真命中（RE2 没有向后看，结束之前的断言只看截断之前的字符，3.6.2）。
7. **`incomplete`**（4.13 `:327`，写定）：走查在检查完每个候选之前停在它的一个界线上（500 页、32 MiB、一次的工作、软期限、glob 的工作）时为真；停在 `limit` 或答复的 48 KiB 不是（那时答复是满的，`next_cursor` 照样给）。所以"有键而答空、带 `incomplete`"是"还没找完"，不是"没有"。
8. **迁移的筛法与门槛**（4.2 `:97` 的"筛法由 P4 实测定……其余的由升级之后的 `nervewiki reindex` 重建（筛漏了的也由它补上）"）：筛法写定（3.2.2），并证明是超集：分词会截掉内容的页一定被它选中（以 4 种条件之一）。所以没被选中的页在版本 5 下的词与版本 4 的相同，行只差 `tokenizer` 一列：**正确性不要 reindex**。条件 1 的门槛是正文**多于 74,000 字节**：一个输入的字节至多给出 13 + 13/30 字节的词与前缀（每个码位至多 13，U+0344、U+0587、U+FB2C、U+FB2D；Go 的 NFC 在 30 个不是起始的字符之后插一个 CGJ（U+034F，2 字节、一个单词字符，再一个三元组与前缀，13 字节），而一个输入的字节至多折叠出一个这样的字符，`exp/bound2` 对全部码位算），所以名称加 `\n` 加正文至多 74,441 字节时到不了 1,000,000，名称至多 255 字节，正文到 74,185 字节都到不了；取它之下的整数 74,000（审查 B-M1：76,667 没算 CGJ；审查 B2-n1）。实际要到上限要不同的词：最小的约 117 KB（审查 B 的构造），随机的 CJK 约 230 KB（3.2.2）。
9. **reindex 的代价、它的副作用与迁移的重建**（4.2 `:97` 的"全部重建的代价同 00041（30 万页约 16 分钟、约 130 GB 的 WAL）"；第 10 节升级一行 `:617`）：`nervewiki reindex` 重建的是已有的行，照 M8/P5 量的约 26 分钟、约 290 GB 的 WAL（30 万页，`README.md:328` 已这样写），不是 00041 从空建的约 16 分钟、约 130 GB；升级之后它不是必须的。但它是链接的索引的重建，每本笔记本之后把分词不是当前的行入队（`bootstrap/reindex.go:23-36`、`queries/search.sql:23-30`）：**之后不论为了什么跑的第一次 `nervewiki reindex`，也重建所有仍是版本 4 的搜索的行**（30 万页约 26 分钟、约 290 GB 的 WAL，其间各笔记本答 `building`），README 的升级一条与总体设计 11 节写明（审查 B-M6；`StaleSearchNodes` 不改）。迁移自己入队的页的重建按词估（审查 B-M2）：离线的语料上筛法选中 3,086 个文件里的 84 个（2.72%），却是正文的 34.2%、正文的词的 19.2%（选中的多是大页，`exp/tokb2`）；任务的代价随词（GIN 的项），照 M8/P5 的已有的行的重建折算，30 万页的库若像这些语料，约 5 分钟、约 56 GB 的 WAL（按页数的约 41 秒、约 7.8 GB 是下界）。S1 在 30 万页的合成的库上量入队的页数、字节、迁移的用时与它们的重建的用时与 WAL（M8 的语料不在了、不下载，审查 C3-I1；种子对准入队的页的字节与词的份额，审查 B4-m1；3.2.2、9.1）；README 与 00 号文档写这个数、标明是合成的，按词的估计写在旁边。
10. **被标出的比例：规则、语料、校准与定下的一支**（4.2 `:97` 与 4.13 `:327` 的"P4 在一个真实的、导入的笔记库上量"、第 10 节 `:605` 一行、第 11 节主持者的决定第 12 条 `:654`）：**不下载语料**（主持者在审查之后的决定，改第 12 条的"真实的、导入的笔记库"；审查 B-M5：计划的两种语料定不了这个决定；M8 的语料在它跑完之后删掉了，再有它要重新下载，审查 C3-I1）。规则在**一份冻结的语料**上判：仓库、Go 模块缓存与 `node_modules` 的 Markdown 合在一起（3,834 个路径、3,086 个不同的内容、29.2 MB；每个路径的 SHA-256 与内容的快照在 `exp/corpus/`，主副本在 scratchpad 之外，审查 B3-M1、B4-n2）。按一本 10,000 页的笔记本折算：被标出的页至多 100 个、正文合计至多 8 MiB、**以 20 条指令的模式走查它们的工作（各行的 NFC 字节加 `GrepLineOverhead`，乘 20）至多 `MaxGrepWork` 的二分之一**（主持者在第二轮复核之后定的二分之一；审查 B-M5 的四分之一只是例子）；成立时标出三种截断，否则只标出 `MaxSearchLexemeBytes` 处的。`GrepLineOverhead` 离线校准为 2（3.6.4；审查 A2-m2 = B2-M1）。冻结的语料上：61.6 页、3.61 MB、7.40 × 10⁷；3.6.4 的各档（2–6 × 10⁸）都成立，2 × 10⁸（二分之一 1 × 10⁸）时是界线的 74%，`GrepLineOverhead` 到 17 才不成立；预计的一档是 2 × 10⁸（较慢的形状，3.6.4）。所以**设计定下标出三种截断**：被截掉的页总被走查，`grep` 不悄悄漏找，走不完的答 `incomplete`。S1 的第 1 步在满负载下离线地量出四个常量、在冻结的语料上数三条，第 3 步在导入冻结的语料的库上与照导入自己的规则算出的预期的集合比（审查 C4-M1），第 9 步以真的用例重算；哪一次让规则选另一支时 S1 停下、报告主持者（3.3.4）。一次走查盖得住多大的笔记本与持平点写在 3.3.4。第 10 节 `:605` 一行的"在真实的、导入的笔记库上量"随之。
11. **`partial` 只看正文的部分**（4.2 `:97`，写细）：正文的词取自 `name + "\n" + content`，名称里超过 64 个的 ASCII 连续串同样不收；`partial` 只在 `content` 的部分有文字被留在外面时为真，`grep` 不搜名称（4.13）。名称至多 255 字节，到不了词的上限。
12. **`index` 的 `type` 不照段的检查**（4.8 `:186` 的"`index` 的 `type` 与 `parent` 同样检查"）：`parent` 是路径，照段的检查；`type` 是 frontmatter 里的字符串，可以有 `/`、`:`、`#`（`type: project/active`），`CheckTitle` 拒绝它们，照段的检查就筛不出这些页。`type` 只查不带 NUL（PostgreSQL 的 `text` 收不下）与至多 1,024 字节（3.9.3）。
13. **`index` 的 `type`、次序与筛选的取值**（4.9 `:222`，写细；4.14 的 `index` 一行 `:354`）：`type` 同 `summary`，取 frontmatter 里 ASCII 不区分大小写的第一个 `type` 键，值是字符串时才有；给了 `type` 时在语句里筛（`value = to_jsonb($type)`），只读相等的页。次序的键是 `TitleKey(type)` **截到 64 字节**（字符的边界），在 Go 里算：语句读每个 `type` 的前 128 个字符就够（一个字符的键至少 2/3 个字节，64 字节至多 96 个字符，加流安全的 30 个，3.9.3；审查 B-M3、B-N2）；答复的 `type` 照旧截到 256 字节，由列出的那一页的项另读。三个筛选都不收空的字符串（422，文字说"不传它"；模型常以 `""` 代替"没有"）；`tag` 去掉开头的一个 `#`（正文里的写法，frontmatter 的标签也这样存），带 NUL 的 `tag` 同 `getTag` 是空的一页；`parent` 为 `/` 是根下的页（审查 B-M7）。
14. **`index` 读属性经部分索引**（4.2 `:98` 的"`index` 的读要不要新的索引……由 P4 的计划测试定"）：S2 无条件加迁移 00047：`page_properties (source_id, position)` 上 `WHERE translate(key, 'TYPE', 'type') = 'type'` 与 `WHERE translate(key, 'SUMARY', 'sumary') = 'summary'` 的两个部分索引；一页的属性一次探查，不论它有多少属性（没有它时一页没有 `type` 的 1,000 个属性全读：2,000 页 580 ms，有它 0.9 ms，审查 B-I2 在 PG 18.6 上量）。计划测试与守卫照它种下（3.9.3、6.1）。
15. **`search` 的项**（4.13 `:334`）：最后改动的时刻就是层的排序时刻（`SortAt`，`page/app/search.go:353`）；`search` 也找名称里有词的附件（M8 的两层），项的 `kind` 是 `page` 或 `attachment`，附件没有摘要。
16. **码的次序**（4.15 `:366` 的通用次序，写定各工具的）：`grep` 照通用的次序：游标 400 → 模式 422 → `glob` 422 → `limit` 422 → 笔记本 → 503（名额；解析的队列只等、不答码）（审查 A-m1：`list` 把游标排在模式之后的理由是游标随模式的种类解码，`grep` 只有一种游标）；`Search.InNotebook`：游标 400 → `q` 422 → `limit` 422 → 笔记本（`limit` 在笔记本之前，按工作区的 `Execute` 在它之后，照旧）；`backlinks`：游标 400 → `path` 422 → `limit` 422 → 笔记本 → 页 404；`index`：游标 400 → `type`、`tag`、`parent` 的 422 → `limit` 422 → 笔记本 → `parent` 的页 404（3.7.1、3.8、3.9）。
17. **名额管着读与匹配；一次调用的内存**（4.13 `:323` 的"进程内同时匹配的调用至多 `max(1, GOMAXPROCS/2)` 个"，写细；第 10 节 `:616`、"写给后面的 M"的性能一条 `:467`）：名额在笔记本的判定之后取，持到答复建好：读树、候选、正文与匹配都在名额里，所以一个进程里同时读进内存的 `grep` 的正文至多名额数乘一批（4 MiB，一页大于它时 5 MiB）加一行的 NFC 副本（只规整截断能到的开头：至多 `GrepLineCut` 加一段，10 条指令的模式时至多约 15 MB，NFC 能把一行变长到 3 倍，审查 A-m4）（3.6.5）；名额外的只有解析（在它的队列里）与到判定为止的几次点查。`index` 一次调用至多约 65 MB（10 万页的 128 个字符的 `type` 与它们的 64 字节的键），第 10 节与 M12 的性能移交写上（审查 B-M3）。
18. **`backlinks` 的来源页没有路径时**（4.9 `:221`，补）：读了反链之后进了回收站的来源页没有路径（`NodePaths.Paths` 只给活着的），它这一项不列出，游标照样越过它。
19. **`MaxGrepRead`**（4.13 `:328` 的"同反链一次读的上限，`linking/domain/context.go:23-26`"）：page 不能导入 linking，`page/domain` 自己定义 `MaxGrepRead = 32 << 20`，注释引那一处。
20. **M8/P5 的移交第 1 条的读法**（"移交的落实" `:450` 的"各自的修法照旧"，写细）：`grep` 不在 SQL 里连 `nodes` 与 `page_contents`：候选的 id 由 `page_search_hits` 与队列读出（一条语句），与树在 Go 里取交，正文按主键读；移交的两个陷阱（规划器的整表扫描、照写的连接）都碰不到（3.4.2、6.1 的计划测试）。
21. **MC6 写一次**（第 9 节 `:597` 的"MC6、MC7、MC8 不写库"）：MC6 的"MCP 刚写的立即两者都找到"经 MCP 的 `write` 写一页，那一次写照 MC3 的断言核对库（`expectNewPage` 与 `expectChangesetOf`，客户端是 `mcp:e2e-agent`，`e2e/stories/mcp/mc3-write-edit.spec.ts:69-88`）；MC7 照旧不写库（第 4 节）。
22. **说明的余地**（4.6 `:154`，[P3](03-P3-batch-revert.md) 第 8 节的"P4 之后说明离 4 KiB 只剩约 50 字节"）：P4 的五行合计 571 字节（`search` 一行不写"一个汉字只找标题"，它在工具的说明里；`building` 一行写出批量入队的各种来源，只说 `search` 与有键的 `grep`，审查 C2-N5），账户入口剩 69 字节、笔记本入口剩 139（3.14）。之后在说明里加字的 Phase 照旧先缩短已有的行。
23. **4.14 的 `grep` 一行**（`:359`）：加"解析在进程的队列里（`GOMAXPROCS` 个名额，只等、不答码）"，"一次的工作 `MaxGrepWork`（实测）"写上 3.6.4 量出的值与 `MaxGrepLineWork`、`GrepUnitTime` 的值（S1 合并时；随这份设计的提交只加队列一句）。
24. **成本那一份的时限**（第 11 节第 54 条 `:708` 的"S1 不加成本的行，下一个往里加的 Step 提高时限"）：P4 的 S1 往成本那一份加 `grep` 在 P1 的守卫树上的一行（6.1、6.5），所以照这一条把它的时限由 20 分钟提到 25 分钟（`ci.yml` 一行，作业约 6–10.5 分钟的约两倍）；S2 再加 `index` 的守卫一行，之后约 11 分钟，在 25 分钟的一半之下。
25. **组合根第三份的余地**（第 11 节第 64 条 `:721` 的"过了线时 `TestEvery[M-R]` 挪到第三份"）：P4 的测试都落在第三份（6.5），之后第三份收不下这次挪动（约 7:14 加约 53 秒）；所以第一份过线时 `TestEvery[M-R]` 挪到第四份 `BOOTSTRAP_FOURTH = ^TestEvery[M-R]`（`ci.yml` 的矩阵加一项），第一份的表达式照第 64 条去掉它；第 64 条的文字随之（审查 C-M7）。第三份自己过线时另把 `^TestThe(M|P)` 放进第四份（第一份还没挪过时第四份只是 `^TestThe(M|P)`，否则 `TestEvery[M-R]` 跑两次，审查 C3-N1）、第三份的表达式去掉它们（6.5，审查 C2-N2）。

### 2.2 需要负责人确认

无。负责人在第 11 节的决定都照办：`grep` 之外的 `search`（第 4 条）；没有穷尽模式与批量入队的 `building` 照主持者的第 8、44 条。主持者的决定里改了的两处都是主持者的，不是负责人的：被截掉的页总被走查（主持者的第 11 条）照办；被标出的比例（主持者的第 12 条）照主持者在审查之后的决定改为离线地量、规则加上工作，在冻结的语料上、以 `MaxGrepWork` 的二分之一、校准每行的开销之后照规则定下标出三种截断（2.1 第 10 条），负责人可以改判。2.1 的其余各条是写细或照代码改正。

## 3. 服务端

### 3.1 文件

| 文件 | 包 | Step | 新/改 | 内容 |
|---|---|---|---|---|
| `server/migrations/sql/00046_page_search_partial.sql` | — | S1 | 新 | `page_search.partial` 与检查；只把可能被截过的活着的页批量入队（3.2） |
| `page/domain/search.go` | page/domain | S1 | 改 | `searchTokenizerVersion = "5"`；`SearchTokens` 多答 `partial`，有时正文的词末尾加 `SearchCutKey`；`BoundFreeContentBytes`（迁移的门槛，3.2.2、3.3） |
| `page/domain/grep.go` | page/domain | S1 | 新 | `grep` 的常量；`ParseGrepPattern`（字节、换行、语法、指令数）、`GrepInsts`、`GrepLines`（同 `read` 的分行，去掉 `\r` 与 BOM）、`GrepCursor`、`CheckGrepLimit`、`GrepWork`、`GrepText`（以命中为中心截到 1,000 字节）（3.4、3.6、3.7） |
| `page/domain/grep_keys.go` | page/domain | S1 | 新 | `GrepKeys`：树上的事实、段、片、键（3.5）；挨着分词，用它未导出的 `classOf`、`searchFold`、`lexemes` |
| `page/domain/grep_tables.go` | page/domain | S1 | 新 | 生成的表：96 个危险的结尾（3.5.3）；`grep_tables_test.go` 按定义重算、比较 |
| `page/domain/grep_test.go`、`grep_keys_test.go`、`grep_keys_property_test.go`、`grep_keys_tails_test.go`、`search_test.go`、`search_bound_test.go` | page/domain | S1 | 新、改 | 6.1（键的表格、性质与穷举的测试；上限的两行与码位的界） |
| `page/app/grep.go` | page/app | S1 | 新 | 用例 `Grep`、`GrepDeps`、`GrepSlots`、解析的队列 `GrepParses`（3.6、3.7） |
| `page/app/grep_test.go`、`grep_cost_test.go` | page/app | S1 | 新 | 用例的表格测试（假的存储）、名额与队列；`//go:build !race`：P1 的守卫树上的 `grep`（6.1） |
| `page/app/ports.go` | page/app | S1 | 改 | `GrepReads`（候选、正文、`SearchBuilding`）；`SearchRow.Partial`；`SearchDeps.Notebooks`（3.4.3、3.8） |
| `page/app/search.go` | page/app | S1 | 改 | `Search.InNotebook`、`SearchHit.Tier` 与 `Cursor()`（3.8） |
| `page/app/index_search.go` | page/app | S1 | 改 | 任务写 `row.Partial`（3.3.2） |
| `page/adapter/postgres/queries/search.sql`、`search.go`、`grep.go` | page/adapter/postgres | S1 | 改、新 | `WriteSearchRow` 写 `partial`；`GrepCandidates`；存储的方法（3.4） |
| `page/adapter/postgres/search_scope_test.go`、`grep_test.go`、`grep_plan_test.go`、`migrations_test.go` | page/adapter/postgres | S1 | 改、新 | 块数、`GrepCandidates`、计划、迁移的筛法（6.1） |
| `page/adapter/mcp/grep.go`、`search.go`、`tools.go`、`instructions.go` 与测试 | page/adapter/mcp | S1 | 新、改 | 两个工具、说明的三行（3.10、3.14） |
| `page/module.go` | page | S1 | 改 | `Deps` 加 `GrepSlots`（nil 是按 `GOMAXPROCS` 建的）、`MaxGrepWork`、`MaxGrepLineWork`、`MaxGrepRead`、`GrepWalk`（0 是 domain 的，生产留 0；整个程序的测试经它们注入）；解析的队列按 `GOMAXPROCS` 建一个，不经 `Deps`；`page.NewGrepSlots(n)`（`pageapp.NewGrepSlots` 的转交，码的测试用，3.12）；用例交给 `adapter/mcp` |
| `page/node_paths.go` | page | S2 | 新 | 模块根的 `NodePaths` 与 `NewNodePaths(pool)`：`Check`、`FindPage`、`Paths`、`Pages`、`ChildPages`（3.9.1） |
| `page/adapter/postgres/queries/nodes.sql` | page/adapter/postgres | S2 | 改 | `RootPageIDs`、`ChildPageIDs`（3.9.1） |
| `linking/domain/actions.go`、`linking/domain/index.go` | linking/domain | S2 | 改、新 | `ActionListIndex = "index.list"`；`index` 的常量、`IndexCursor`、`IndexKey`、`CutIndexString`（3.9.3） |
| `linking/app/ports.go`、`backlinks.go`、`index.go` | linking/app | S2 | 改、新 | `NodePaths` 端口、`IndexReads`；`ListBacklinks.AtPath`；`ListIndex`（3.9） |
| `server/migrations/sql/00047_linking_page_properties_keys.sql` | — | S2 | 新 | `page_properties` 上 `type` 与 `summary` 的两个部分索引（3.9.3） |
| `linking/adapter/postgres/queries/index.sql`、`index.go`、`index_plan_test.go`、`index_cost_test.go` | linking/adapter/postgres | S2 | 新 | `IndexPages`、`IndexItems`、`InboundCounts`、计划测试；`//go:build !race`：`TestIndexOfAHundredThousandPagesTakesItsTime`（3.9.3、6.1） |
| `linking/adapter/mcp/`（`tools.go`、`backlinks.go`、`index.go`、`text.go` 与测试） | linking/adapter/mcp | S2 | 新 | 两个工具（3.10） |
| `linking/module.go` | linking | S2 | 改 | `Deps.NodePaths`；`(*Module).Tools()` |
| `access/domain/rules.go` | access/domain | S2 | 改 | `"index.list": {Level: LevelNotebook, Notebook: readers()}` |
| `bootstrap/deps.go`、`bootstrap/mcp.go` | bootstrap | S2 | 改 | `linkingDeps` 接上 `NodePaths`；`mcpTools` 拼 linking 的工具 |
| `bootstrap/permission_matrix_mcp_test.go`、`mcp_reads_test.go`、`mcp_search_test.go`（新）、`mcp_links_test.go`（新）、`search_test.go`、`trash_test.go` | bootstrap | S1、S2 | 改、新 | 码的表、矩阵、合法参数、整个程序测试（6.1） |
| `bootstrap/grep_cost_test.go` | bootstrap | S1 | 新 | `//go:build !race`：`grep` 的三个时间守卫（6.1） |
| `api/mcp/tools.json` | — | S1、S2 | 改 | 每个工具两个条目（3.11） |
| `Makefile` | — | S1、S2 | 改 | S1：`test-go-guards` 加 `grep` 的一行、`test-go-costs` 加 `page/app` 的守卫一行；S2：`test-go-costs` 加 `index` 的守卫一行；都带 `-v`（6.5） |
| `.github/workflows/ci.yml` | — | S1 | 改 | 成本那一份的时限由 20 分钟提到 25 分钟（第 11 节第 54 条，6.5） |
| `e2e/stories/mcp/mc6-grep-search.spec.ts`、`mc7-index-backlinks.spec.ts`、`e2e/fixtures/mcp.ts` | e2e | S1、S2 | 新、改 | 第 4 节 |

archtest 不改：`linking/adapter/mcp` 只导入 mcpserver（不导入 SDK，`archtest/rules_test.go:338-345`），同 page 的。

### 3.2 迁移 00046

#### 3.2.1 列与检查

```sql
-- page_search.partial: whether the body's lexemes leave out text of the page's content (M9 design 4.2; M9/P4
-- design 3.3): a run of more than 64 ASCII letters and digits, a data: URL longer than 64 bytes, or what follows
-- MaxSearchLexemeBytes. Such a row's body holds the reserved key !cut behind its notebook's prefix too, so that grep
-- reads a notebook's partial rows by the GIN index as it reads its keys (page_search_hits), never another notebook's.
-- A name's run left out does not count: grep searches contents alone. An attachment's row, without a body, is never
-- partial.

-- +goose Up
ALTER TABLE page_search ADD COLUMN partial boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT page_search_partial_check CHECK (NOT partial OR body IS NOT NULL);
```

`ADD COLUMN … DEFAULT false` 是 PostgreSQL 11 起的只改目录（不重写表）；`ADD CONSTRAINT … CHECK` 要读一遍 `page_search` 的堆核对（全部行为 false；在 goose 的事务里持着 ACCESS EXCLUSIVE 锁，只在启动时；用时由 S1 在 30 万页的合成的库上量（3.2.2），写进 9.1 与 M12 的升级说明）。不加 `NOT VALID`：要它对已有的行也成立，否则升级之后的一行不合时没有谁发现。

#### 3.2.2 只把可能被截过的页入队

分词截掉内容的三种情形（3.3.1）各有一个只看正文的必要条件；四个条件的或就是筛法：

```sql
-- The live pages whose content the tokenizer may cut, queued in bulk for its version 5 (M9/P4 design 3.2.2): every
-- page SearchTokens marks partial meets one of the four conditions, so a page not queued has the lexemes of 4 under
-- 5, and its row differs only by its tokenizer, which nervewiki reindex rewrites whenever it runs. The conditions:
--   1. the bound: a byte of name + "\n" + content costs at most 13 + 13/30 bytes of lexemes and prefixes (13 at
--      U+0344, U+0587, U+FB2C, U+FB2D; and Go's NFC writes a CGJ, a word character of 2 bytes, after 30 runes that
--      are no starters, which no fewer bytes give), so MaxSearchLexemeBytes (1,000,000) takes more than 74,441 bytes of them,
--      a name at most 255: a content of at most 74,000 bytes (BoundFreeContentBytes) never reaches it;
--   2. a run of more than 64 ASCII letters and digits, as written;
--   3. a data: URL of more than 64 bytes: its characters are ASCII, so 60 after "data:", in any case;
--   4. a run or a URL made long by folding: the eleven runes whose fold is ASCII letters alone (U+00DF, U+017F,
--      U+1E9E, U+212A, U+FB00-U+FB06) fold to three at most, so 22 of them and of ASCII letters and digits make 65.
-- PostgreSQL's bracket ranges here are of code points; a wider reading of any of them only queues more.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT n.id, n.notebook_id, true
FROM nodes n JOIN page_contents c ON c.node_id = n.id
WHERE n.deleted_at IS NULL AND n.kind = 'page' AND c.deleted_at IS NULL
    AND (c.byte_size > 74000
        OR c.content ~ '[0-9A-Za-z]{65}'
        OR c.content ~* 'data:[^\t\n\v\f\r )"''<>\]`]{60}'
        OR c.content ~ '[\u00DF\u017F\u1E9E\u212A\uFB00-\uFB06]'
            AND (c.content ~ '[0-9A-Za-z\u00DF\u017F\u1E9E\u212A\uFB00-\uFB06]{22}' OR c.content ~* 'data:'))
ORDER BY n.id
ON CONFLICT (node_id) DO UPDATE SET version = excluded.version, bulk = page_search_queue.bulk AND excluded.bulk;

-- +goose Down
ALTER TABLE page_search DROP CONSTRAINT page_search_partial_check, DROP COLUMN partial;
```

- **为什么是超集**（迁移的测试照这几条核对，6.1）：
  - 词的上限：一个输入的码位至多给出每字节 13 字节的词与前缀（`exp/tok` 与 `exp/bound2` 对每个码位算 `(3F + 7n) / r`：`F` 是它折叠之后不属于"别的"的字节、`n` 是这样的字符数、`r` 是它自己的字节；一个字符至多进 3 个三元组或 2 个二元组，每个词另算 7 字节的前缀；最贵的是 U+0344、U+0587、U+FB2C、U+FB2D）。另有输入里没有的字符：Go 的 NFC 写流安全的文字，30 个不是起始的字符之后插一个 CGJ（U+034F，属于记号，是 2 字节的单词字符），它至多多一个三元组与前缀，13 字节；一个输入的字节至多折叠出一个不是起始的字符（最多的是 U+0344，两个字节两个，`exp/bound2` 对全部码位算），所以每 30 个字节至多一个 CGJ。每字节至多 13 + 13/30 ≈ 13.4333（审查 B-M1：30,000 个 U+0344 折叠出 1,999 个 CGJ，量得 13.4331）。所以 `name + "\n" + content` 至多 74,441 字节时到不了 1,000,000（74,441 × (13 + 13/30) = 999,991）；名称至多 255 字节，正文至多 74,185 字节时到不了；门槛取它之下的整数 74,000（`BoundFreeContentBytes`；(74,000 + 256) × 13.4333 = 997,506）。`search_bound_test.go` 对全部码位（含 CGJ 自己）照这样算，断言门槛乘这个界不过上限（6.1）。实际上，到上限要那么多**不同的**词（`add` 每个只收一次）：一个码位重复不行（U+0587 重复到 80,000 字节只有 3 个词），审查 B 构造的最小的约 117 KB，随机的 CJK 约 230 KB（`exp/bound2`）。
  - 长的连续串：分词在 `searchFold` 之后的文字里找"超过 64 个、而且每个字符都是 ASCII 的单词字符"的串（`search.go:351-353`）。ASCII 的字母与数字折叠之后仍是一个 ASCII 字母或数字；非 ASCII 的码位里折叠之后全是 ASCII 字母与数字的只有那 11 个（`exp/tok` 对全部码位算出）；别的非 ASCII 字符折叠之后至少有一个非 ASCII 的字节，串就不"全是 ASCII"。所以这样的串来自正文里一段只由 ASCII 字母、数字与那 11 个组成的连续字符：没有那 11 个时它就是 65 个以上的 ASCII 字母与数字（条件 2），有时每个至多折叠成 3 个，22 个以上（条件 4）。
  - `data:`：`inDataURL` 只收 ASCII（`search.go:344-346`），串的字节数就是字符数；折叠之后的 `data:` 只能来自 ASCII 的 `data:` 不论大小写（那 11 个的折叠里没有能拼出 `data` 的：`ﬅ`、`ﬆ` 给 `st`，别的给 `s`、`ss`、`k`、`f…`）。串里没有那 11 个时它在正文里就是 `data:` 加至少 60 个这样的字符（条件 3，`~*` 不区分大小写，`[^…]` 还收非 ASCII，只是多选）；有时条件 4 的第二支。
  - 离线核对（`exp/tok`）：仓库与两个缓存里去重之后的 3,086 个 Markdown 文件，被截的 19 个全被选中、漏选 0；另以五个对抗的正文核对（22 个 `ﬃ`、64 个 `a` 加 `K`、33 个 `ß`、`data:` 加 20 个 `ﬃ`、`DATA:` 加 60 个 `Q`），都被截、都被选中。审查 B 另以 30 万个靠近各条件的边的正文（`tokb filter-fuzz`）与 PostgreSQL 18.6 对这段谓词本身的读法（3 万个，其中 5,315 个被截）核对：0 个被截而漏选，PostgreSQL 与 Go 的选法 0 处不同；非 ASCII 而折叠之后全是 `data:` 串的字符的是 12 个（那 11 个与 U+037E，它折叠成一个 `;`，`[^…]` 照样收）。
- **代价**：整表读一遍 `page_contents` 的活着的正文（TOAST 的解压）与四个正则：审查 B 在 PG 18.6 上 146 MB 的语料的 Markdown（TOAST 之后 63 MB）上整个谓词 667–670 ms（解压 220 ms），约每 GB 4.6 秒，M8 的 3.1 GB 约 15 秒的 CPU 加读盘；在 goose 的事务里，`page_search` 的 ACCESS EXCLUSIVE 锁只挡着启动（v0.1 是单实例）。选中的页批量入队，任务重建的是已有的行，代价照 M8/P5 量的"满的索引里的 reindex 式重建"（299,200 行 25 分 50 秒、193 行/秒、291 GB 的 WAL，[M8/P5](../M8-history-search/05-P5-search.md) 3.9 的表），不是 00041 从空建的（15 分 52 秒、131 GB）。它的代价随词（GIN 的项），不随页数（审查 B-M2）：离线的语料上（`exp/tokb2`，门槛 74,000）筛法选中 3,086 个文件里的 84 个（2.72%；最先满足的条件：大小 59、长串 12、折叠 12、`data:` 1），它们是正文的 34.2%、正文的词的 19.2%（选中的多是大页）；照词折算，30 万页的库若像这些语料，约 5 分钟、约 56 GB 的 WAL（按页数的约 41 秒、约 7.8 GB 是下界）。重建完之前这些页所在的笔记本答 `building`，它们的 `grep` 与搜索照旧读老的行（老的行仍是对的：版本 4 的词就是这些页现在的，只少了 `!cut`）。
- **大的库上的实测**（M8 的语料不在了，3.3.4；审查 C3-I1）：S1 在合并之前以 `main` 的二进制种一个 30 万页、200 本笔记本的**合成的库**（M8 的规模），任务建好版本 4 的行（像升级之前的库）；再以 S1 的二进制执行 00046、任务追上，记迁移的用时（筛法的扫描、入队、`CHECK`）、入队的页数与字节、任务重建它们的用时与 WAL，写进 9.1 与 M12 的升级说明。README 与 00 号文档写这个数、标明是合成的库上量的，按词的估计照旧写在旁边（第 7 节）。合成的库照冻结的语料（3.3.4）的份额造，**对准的是迁移要重建的那部分**（审查 B4-m1：重建的代价随入队的页的词，入队的多是大页，它们每字节的词远低于平均；只对准平均的词表可能把大页的词放在下面的曲线上的任何地方）：页的大小取它的分布（中位数 2,553 字节、p10 270、p90 22,238、p99 100,995、最大 621,199、平均 9,456；M8 的语料平均 8.1 KB）；按冻结的语料的份额让页满足筛法的各条件：2.72% 被选中，最先满足的条件是大小的 1.91%（取大页的那一段分布）、长串 0.39%、折叠 0.39%、`data:` 0.03%；被标出的 0.62%（长串 0.55%、`data:` 0.06%）。**入队的页的份额是种子的目标**：它们是正文的 34.2%、词与前缀的 19.2%（每字节 0.43 字节，全部的是 0.77；按最先满足的条件：大小 30.8% 与 15.3%、长串 1.3% 与 1.0%、折叠 2.0% 与 2.8%、`data:` 0.1% 与 0.1%）。正文从 Zipf 分布的词表取词，词表随页的大小定，让每字节的词与前缀照冻结的语料的曲线走（`exp/corpus/curve-out.txt`，`exp/freeze` 从快照算：不到 1 KiB 2.65、1–4 KiB 1.72、4–16 KiB 1.05、16–64 KiB 0.68、64 KiB 到 74,000 字节 0.59、74,000 字节到 128 KiB 0.48、128–256 KiB 0.34、256 KiB 以上 0.32 字节；审查 B 的 `tokb curve` 得出同样的）。种子在量之前以分词数出自己的份额（入队的页数、字节、词与前缀的字节，各占全部的多少），写进 9.1，量得的数才算；入队的页的字节或词的份额与目标差过约 20% 时调词表重种，或把量得的用时与 WAL 按入队的页的词与目标的比折算、写明折算过。种子以 SQL 或测试的帮助函数写下页与正文，搜索的行由 `main` 的任务建。
- **其余的行**：不入队、不重写。`nervewiki reindex` 照旧把分词不是当前的行入队（`search.sql:23-30`），升级之后不是必须的；它的代价是 README 已写的"重建一个已有的索引"（30 万页约 26 分钟、约 290 GB 的 WAL，重建之后 `page_search` 约是原来的 1.7 倍，`README.md:328`；M8/P5 3.9），不是 00 号文档 4.2 引的 00041 从空建的约 16 分钟、约 130 GB。**副作用**：`nervewiki reindex` 是链接的索引的重建，每本笔记本之后把分词不是当前的行入队（`bootstrap/reindex.go:23-36`），所以之后不论为了什么跑它（M6 的那一条、以后的提取器的升级），第一次也重建所有仍是版本 4 的搜索的行（约是整个索引），其间各笔记本答 `building`、约 26 分钟、约 290 GB 的 WAL（30 万页）。README 的升级一条与总体设计 11 节写明这一句（审查 B-M6）。`StaleSearchNodes` 不改（主持者的决定）：超集的证明允许它把当前 Unicode 下的版本 4 的行当作当前的，但那要它分辨两种版本，留给以后再换分词时一起看。
- **降级**：`Down` 去掉列与检查；版本 5 的行留着 `!cut` 键，版本 4 的代码不读它（它的查询的键不会以 `!` 开头，3.3.1），`nervewiki reindex` 照常重建。

### 3.3 分词的版本 5、`partial` 与 `!cut`

#### 3.3.1 分词

```go
// searchTokenizerVersion: 5 marks the rows whose body leaves out text of
// the content, with SearchCutKey (M9/P4 design 3.3).
const searchTokenizerVersion = "5"

// SearchCutKey is the reserved key of the body of a page whose content's
// text the lexemes leave out (partial): no lexeme holds a "!", which is of
// the class other, so a query for it finds the partial rows of the
// notebooks it reads alone, behind their prefixes (M9 design 4.2).
const SearchCutKey = "!cut"

// BoundFreeContentBytes is the most bytes of a content whose body never
// reaches MaxSearchLexemeBytes, whatever its name: a byte of name, "\n"
// and content gives at most 13 + 13/30 bytes of lexemes and prefixes, the
// CGJs Go's NFC writes counted (M9/P4 design 3.2.2). Migration 00046
// queues the pages past it; search_bound_test.go keeps it under the bound.
const BoundFreeContentBytes = 74_000

// SearchTokens is the lexemes of a node's name, the title's, and of a
// page's name and content, the body's (nil for an attachment, content
// nil), and whether the body leaves out text of the content: a run of more
// than MaxSearchWordRunes ASCII letters and digits, a data: URL longer
// than 64 bytes, or what follows MaxSearchLexemeBytes. A partial body ends
// with SearchCutKey, which counts toward no bound: the tsvector's own,
// 1,048,575 bytes, holds it above MaxSearchLexemeBytes.
func SearchTokens(name string, content *string) (title, body []string, partial bool)
```

- `lexemes` 加一个字段 `from int`（正文的部分在折叠之后的文字里的第一个字节，`len(searchFold(name)) + 1`：`searchFold(name + "\n" + content)` 等于 `searchFold(name) + "\n" + searchFold(content)`，`\n` 是 NFC 的边界、不与任何字符合成，折叠也不碰它）与 `left bool`；`scan` 在三处设它：`data:` 的跳过处 `i >= from`、`word` 里 `longASCII` 不收的串 `start >= from`、`add` 到上限时（`full`，正文的部分里；名称到不了上限）。`SearchTokens` 在 `left` 时把 `SearchCutKey` 加到 `b.out` 的末尾（不经 `add`：不占上限，也不会因为 `full` 而丢掉）。
- 标题的词不变（名称的长串照旧不收，与 `partial` 无关）。
- 版本 5：`SearchTokenizer` 随之变，任务重建一切被入队的行（`index_search.go:170-173` 只留分词相同的行）。之前每次改词都升（M8/P5 3.3）。
- **三种截断都标出**（3.3.4 照规则定下）：被截掉的页总是 `grep` 的候选（3.4.2），它们的哪一种文字都找得到。

#### 3.3.2 任务记下什么，页什么时候离开 `!cut`

- `SearchRow` 加 `Partial bool`；任务 `row.Title, row.Body, row.Partial = domain.SearchTokens(s.Name, content)`（`index_search.go:211`）。`WriteSearchRow` 加一列：

```sql
-- name: WriteSearchRow :batchexec
-- A node's row as the job built it: its lexemes as tsvectors, without positions, each after its notebook's prefix
-- (M8 Codex review R1), and whether the body leaves out text of the content, its lexemes holding !cut then (M9/P4
-- design 3.3.2). A node purged since it was read has no notebook and gets no row.
INSERT INTO page_search (node_id, revision, name, tokenizer, built_at, title, body, partial)
SELECT n.id, sqlc.narg(revision)::integer, sqlc.arg(name)::text, sqlc.arg(tokenizer)::text, sqlc.arg(built_at)::timestamptz,
    array_to_tsvector(ARRAY(SELECT t.prefix || l FROM unnest(sqlc.arg(title)::text[]) l)),
    CASE WHEN sqlc.narg(body)::text[] IS NOT NULL
        THEN array_to_tsvector(ARRAY(SELECT t.prefix || l FROM unnest(sqlc.narg(body)::text[]) l)) END,
    sqlc.arg(partial)::boolean
FROM nodes n JOIN page_search_tags t ON t.notebook_id = n.notebook_id
WHERE n.id = sqlc.arg(node_id)::uuid
ON CONFLICT (node_id) DO UPDATE SET revision = excluded.revision, name = excluded.name, tokenizer = excluded.tokenizer,
    built_at = excluded.built_at, title = excluded.title, body = excluded.body, partial = excluded.partial;
```

- `partial` 与 `!cut` 同在一行、同一次写：一行有 `!cut` 当且仅当 `partial`（`checkSearchBuilt` 加这一条，6.1）。
- **离开**：页的正文或名称每次改动都准确入队（单元；`batch` 批量），任务重建这一行时按新的正文重算 `partial`：不再有长串、长的 `data:` 串、不到上限的页的新行没有 `!cut`。在重建之前它在队列里：准确的照样是 `grep` 的候选（3.4.2），批量的让它的笔记本答 `building`。`MinSearchRebuild`（30 秒）跳过的页留在队列里，同样是候选。进了回收站的页的行由任务删掉（M8），它不再是候选也不再在树里。
- 搜索（M8 的两层）从不以 `!` 开头的键读：`ParseSearchQuery` 的键只有字母、数字、记号与 CJK（`search.go:430-489`），所以 `!cut` 不改变 M8 的任何结果；名称层的 `title` 里没有它。

#### 3.3.3 `!cut` 的读

`page_search_hits(ARRAY[这本], ARRAY['!cut'], NULL)`：函数照 00041 给每本笔记本加前缀、引成操作数（`'<前缀>!cut'`，`page_search_queries` 把 `'` 与 `\` 加倍，`!` 照原样），只读这本笔记本的键的位图（`00041:90-103`）。前缀是 base 62 加 `#`（`00041:17-23`），词里没有 `#` 与 `!`，所以 `<前缀>!cut` 不会是任何一本笔记本的真词。

#### 3.3.4 被标出的比例：规则、语料、校准与定下的一支

**规则**（第 11 节主持者的决定第 12 条的"比例高时"写成数；审查 B-M5 之后加上工作）：有键的 `grep` 每次都要走查这本笔记本里全部被标出的页（它们总是候选，3.4.2），走查一次至多 500 页、32 MiB、`MaxGrepWork` 个单位（`MaxSearchWalk`、`MaxGrepRead`，3.6.3）。按一本 10,000 页的笔记本（"一个团队的笔记本"的量级；M12 的压测另量 10 万页的）折算，三条都成立时**标出三种截断**：

1. 被标出的页至多 100 个（1%）；
2. 它们的正文合计至多 8 MiB（`MaxGrepRead` 的四分之一）；
3. 以 20 条指令的模式走查它们的工作至多 `MaxGrepWork` 的**二分之一**：各行的 NFC 的字节加 `GrepLineOverhead`，合计乘 20（3.6.2 的数法；常见的模式 5–22 条，审查 B 以 Go 的编译数：`foo` 5、`TODO: \w+` 10、`(?i)(error|warning|fail)` 22）。二分之一是主持者在第二轮复核之后定的（审查 B-M5 的"四分之一"是一个例子，不是量过的选择）：这一条的用意是被标出的页给真的候选留下足够的工作，一半做得到，走不完的走查答 `incomplete` 与游标。负责人可以改判；改判时要改的是本节最后一段。

规则在**一份**语料上判：仓库、Go 模块缓存与 `node_modules` 的 Markdown 合在一起（下面的冻结的语料），不按来源各自判（审查 B3-M1：按来源判时模块缓存的第 3 条在暂定的四分之一下是界线的 99.8%、`node_modules` 的第 1 条是 100 里的 91.4，那是三份小的样本各自的出入）。不成立时只标出 `MaxSearchLexemeBytes` 处的截断。

- **一次走查盖得住多大的笔记本**：在规则的边上（每 10,000 页 1%、8 MiB、`MaxGrepWork` 的二分之一），被标出的页自己在约 2 万页的笔记本里用完一次走查的工作（字节在约 4 万页、页数在 5 万页用完）；更大的笔记本里每个有键的 `grep` 都先走它们、答 `incomplete`、以游标接着走。
- **持平点**（写进 9.1 的决定的记录）：一页带一张约 200 KB 的 base64 图片（`data:` 串，一行）按工作约 20 × 200,000 = 4 × 10⁶ 个单位；`MaxGrepWork` 是预计的 2 × 10⁸ 时（二分之一 1 × 10⁸），**每 10,000 页约 25 页这样的页（0.25%）**、别的什么都不被标出时就到规则的边上；4 × 10⁸ 时先到的是字节的一条（约 40 页）。读这个决定的人可以拿它对着自己的笔记库判断；负责人愿意时可以在自己的库上跑只给计数的程序（不是 P4 的条件）。Nerve Wiki 的编辑器把贴进来的图片存成附件（`web/apps/web/src/lib/file-transfer.ts:1`，M7/P4 3.6），base64 只经导入、贴进的 Markdown 文字或 agent 的写到达正文；常见的长令牌不到 65 个（git 的哈希 40、SHA-256 的十六进制 64：不截；base64 与 PEM 在 `+`、`/` 或每 64 个字符处断开）。

**冻结的语料**（审查 C3-I1、B3-M1 之后，主持者的决定）：**主副本**是主持者在 scratchpad 之外保管的 `m9-p4-corpus/`（夜里的清理不碰它；审查 B4-n2 = C4-N1），`scratchpad/m9/p4/exp/corpus/` 只是工作的副本。两处都有：

- `frozen-files.tsv`：3,834 个路径（Go 模块缓存 2,499、`node_modules` 956、仓库的 `docs/` 与 README 379），每个的来源、路径、字节、SHA-256、计不计（同一内容的第一个路径是 `used`，之后的是 `dup:<n>`；没有一个因过大、不是 UTF-8 或带 NUL 而跳过）、截断的种类、筛法的条件、行数与 NFC 的字节；
- `snapshot.tar.gz`：3,086 个不同的内容，以 SHA-256 为名（29.2 MB，压缩之后 10.1 MB）；
- `SHA256SUMS`：两者的 SHA-256（`7336d140…5768` 与 `a6cb38ae…27ff`）；
- 主副本另有 `freeze/`、`lineoh2/` 与 `importset/` 的源码（`exp/` 里同名的目录：重算与 `verify`、满负载的校准、第 3 步的预期的集合）。

`exp/freeze` 的 `verify` 只从快照重算下面的数，3,834 行 0 处不符。S1 从主副本把这一组复制进自己的 scratch，以 `SHA256SUMS` 与 `verify` 核对，第 1、3 步都在它上面；那时的缓存另跑一次只作记录（模块与包会更新）。

**M8 的语料不在了**：M8/P5 的语料（zhwiki 与 enwiki 20261001 的 294,055 页、3.1 GB）在它跑完之后删掉了（`05-P5-search.md:886`），它的基准的库每次由 `BenchmarkSearch` 从那些文件在 testcontainers 里重建（`bootstrap/search_bench_test.go:31`、`:67`）；再有它要重新下载，主持者定 P4 不下载（第 11 节第 12 条，2.1 第 10 条）。所以 S1 不用它：比例在冻结的语料上判，迁移在大的库上的代价在 30 万页的合成的库上量（3.2.2）。

**离线的数**（`exp/freeze` 与 `exp/tokb3`：分词照抄 `search.go:96-375`；每行的开销取 3.6.4 校准的 `GrepLineOverhead` = 2）：

| 来源 | 文件（去重） | MB | 被标出 | 上限 | 长串 | `data:` | 被标出的 MB |
|---|---|---|---|---|---|---|---|
| Go 模块缓存的 Markdown | 1,832 | 14.2 | 10（0.55%） | 0 | 8 | 2 | 0.89 |
| `node_modules` 的 Markdown | 875 | 6.5 | 8（0.91%） | 0 | 8 | 0 | 0.18 |
| 仓库的 `docs/` 与 README | 379 | 8.4 | 1（0.26%） | 0 | 1 | 0 | 0.05 |
| **合在一起（规则判的）** | 3,086 | 29.1 | 19（0.62%） | 0 | 17 | 2 | 1.11 |

折算到 10,000 页：

| 条 | 量得 | 界线（暂定的 `MaxGrepWork` 4 × 10⁸） | 占界线 |
|---|---|---|---|
| 1. 被标出的页 | 61.6 | 100 | 62% |
| 2. 它们的正文 | 3.61 MB | 8 MiB | 43% |
| 3. 20 条指令走查它们的工作 | 7.40 × 10⁷ | 2 × 10⁸ | 37% |

第 3 条的来历：被标出的 19 页有 27,879 行、1,086,525 个 NFC 的字节，一条指令的工作 1,086,525 + 2 × 27,879 = 1,142,283 个单位，每 10,000 页 3.70 × 10⁶，乘 20。被标出的是 CHANGELOG 与 API 文档里的哈希与长的标识符、两个 README 里的 base64 图片（`data:`）。每字节的词与前缀：中位数 1.64、p99 4.51、最大 6.67 字节，没有一个到上限。

**各档的余地**：第 3 条随两个常量动。`GrepLineOverhead` 每多 1，它多 20 × 27,879 × 10,000 / 3,086 = 1.81 × 10⁶；只按字节是 7.04 × 10⁷。

| `MaxGrepWork` | 二分之一 | L = 2 时占 | 余地 | 不成立于 |
|---|---|---|---|---|
| 2 × 10⁸ | 1 × 10⁸ | 74% | 26% | L ≥ 17 |
| 3 × 10⁸ | 1.5 × 10⁸ | 49% | 51% | L ≥ 45 |
| 4 × 10⁸ | 2 × 10⁸ | 37% | 63% | L ≥ 72 |

3.6.4 的各档至少 2 × 10⁸，所以在校准的 L（2，满负载下若到 3 也一样）下每一档都成立；只有比各档都低的 1 × 10⁸（二分之一 5 × 10⁷）时不论 L 都不成立。**预计的一档是 2 × 10⁸**：3.6.4 以较慢的形状定 `MaxGrepWork`（审查 A3-m1），离线的满负载在每行 4 KiB 的页上 p99 每单位 22.5 ns（能效核），4.5 秒（3.6.4 的表）；那时第 3 条是界线的 74%。

三条都成立，所以**定下标出三种截断**（审查 A2-m2 = B2-M1 之后，主持者的决定：先校准每行的开销、再照规则定，设计只写这一支）：上限、长的连续串与长的 `data:` 串都让页 `partial`、带 `!cut`，有键的 `grep` 总走查这样的页；`grep` 不会悄悄漏找一页，走不完的答 `incomplete` 与游标。

**这个估计能推多远**：这些是代码库的文档，不是笔记：笔记库里网页的剪藏（`data:` 的内联图片、带长的令牌的链接）、贴进来的哈希与密钥更多，大的单页（导出的聊天、日志）也更多；反过来代码的片段少。持平点是给这一面的：它说笔记库里要多少这样的页才到规则的边上。

**S1 的核对**（不下载；规则选另一支时 S1 停下、报告主持者，由主持者修订设计，S1 不自己换）：

- **第 1 步**（任何与这一支有关的代码之前；审查 C3-M1）：把冻结的语料从主副本复制进 S1 的 scratch，以 `SHA256SUMS` 与 `verify` 核对每个哈希；照 3.6.4 在满负载下离线地量 `MaxGrepWork`、`MaxGrepLineWork`、`GrepUnitTime` 与 `GrepLineOverhead`；以量出的常量在冻结的语料上数三条（只给计数的程序，`exp/freeze` 的数法）。数写进 S1 的审查记录与 9.1。那时的缓存另跑一次，只作记录。
- **第 3 步之后的核对**（审查 C-M4、B3-n3、C4-M1、B4-n1）：以 `main` 的二进制起一个库，把冻结的语料打成一个 zip、经 M7 的导入（`startImport`）导进一本笔记本的根，任务建好版本 4 的行（`page_search_queue` 空了）；再以 S1 的二进制 `nervewiki migrate up` 到 00046，**在 `serve` 之前**读 `page_search_queue` 的行作入队的集合（`serve` 起的任务会把它取空）；之后 `serve`、任务追上，读 `page_search` 里 `partial` 为真的行作 `partial` 的集合。
  - **zip 的布局**：每个路径一个文件的项，内容是快照里以它的哈希为名的文件；项名是三个顶层的文件夹之一加它在来源里的路径：`repo/`（仓库之下）、`gomod/`（`~/go/pkg/mod` 之下）、`node/`（`node_modules/.pnpm` 之下：`node_modules` 的 956 个路径都在 `.pnpm` 里，以 `node_modules/` 为顶层时它们全被忽略）。
  - **预期的集合照导入自己的规则算**（审查 C4-M1：M7 的导入不报告地忽略路径里有以 "." 开头的名字的项，`transfer/domain/entry.go:130`、`ignored` `:212-216`）：`exp/importset` 以 `bbd18d82` 的 `Classify`、`NewImportPlan`、`FixTitle` 与 `TitleKey` 的副本对 zip 的项名离线地算，写出 `expected-pnpm.tsv`（每个路径的项名、哈希、去向、截断的种类、筛法的条件）。3,834 个路径里 3,654 个成页（仓库 379、模块缓存 2,330、`node_modules` 945）；177 个被忽略（模块缓存 168 个在 `.github`、`.claude` 一类的文件夹里，`node_modules` 9 个在 `.github` 里）；2 个超过 11 个名字，`Classify` 以 `too_deep` 跳过；1 个是第 11 层的页，计划以 `too_deep` 跳过。这 180 个里没有被标出的、也没有被选中的，所以预期的 `partial` 的集合是 20 个路径（19 个内容）、入队的集合是 86 个（大小 60、长串 13、折叠 12、`data:` 1），与整个冻结的语料的相同。对照：以 `node_modules/` 为顶层时被忽略的是 1,124 个，含被标出的 9 个、被选中的 22 个（审查 C4-M1 量的，`exp/importset` 得出同样的）。
  - **对应**：这个布局里 `FixTitle` 不改任何名字，同一父页之下也没有键相同的兄弟，所以库里一页的路径就是它的项名去掉 `.md`（导入的报告只有那 3 个 `too_deep`；若有改名的，照报告的 `To` 对应）；对上的每一页再以正文的 SHA-256 核对（导入按字节存正文，`page/domain/content.go:14`）。只有文件夹的页（3,848 个）没有正文，不在比较里。
  - 比**集合**，不只比总数，每个路径一页、不去重：库里入队的集合等于预期的入队的集合，`partial` 的集合等于预期的 `partial` 的集合、且包含于入队的（筛法是超集）。只在一边的页让 S1 停下、报告主持者；数与两边的总数写进 9.1。决定（第 1 步）照旧在整个冻结的语料上判；只有这里对库的核对用导入的那部分，两边是同样的页。
- **第 9 步**：照 3.6.4 以真的用例重量四个常量，以量出的 `MaxGrepWork` 与 `GrepLineOverhead` 重算第 3 条；规则选另一支时 S1 在合并之前停下、报告主持者。

**若负责人或主持者改判为只标出上限处的截断**（这里只记要改什么，设计不写那一支）：`left` 只在 `full` 时设，迁移的条件只留第 1 条；`GrepKeys` 不取 `f` 里超过 64 个、全是 ASCII 的串的三元组，也不取一片里从 `data:` 起的部分的键（`scan` 把整个 `data:` 串连同 `data`、`image`、`base64` 跳过，否则 `data:image/png` 的键哪一页都没有，审查 A2-m3）；只靠长串或 `data:` 串成为 `partial` 的测试的页换成上限的页（审查 C2-M2）；工具的说明与 README 写上漏找：伸进这样一串的模式可能漏掉它的页（审查 B2-n2），agent 分不出"没有"与"没进索引"。由主持者修订设计。

### 3.4 `grep` 的端口与语句

#### 3.4.1 端口

```go
// GrepReads are what grep reads of a notebook (M9/P4 design 3.4), each
// statement on the pool, outside any transaction: grep wants no snapshot,
// and holds no connection while it matches (M9 design 4.13).
type GrepReads interface {
	// GrepCandidates is the pages of notebookID whose body holds every one
	// of keys, those queued exact, and those whose row is partial: ids, in
	// no order, some maybe deleted, which the tree leaves out.
	GrepCandidates(ctx context.Context, notebookID uuid.UUID, keys []string) ([]uuid.UUID, error)
	// GrepContents is the contents not deleted of the pages ids, by id
	// (export's Contents).
	GrepContents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
	// SearchBuilding tells whether the notebooks hold nodes queued in bulk.
	SearchBuilding(ctx context.Context, notebooks []uuid.UUID) (bool, error)
}
```

`Grep` 另用已有的 `Notebooks`（判定，`decideNotebook`，`page/app/authorize.go:28`）、`Nodes.ListNodes`（树）、`PageStates`（页的字节数，`ports.go:289-300`）。存储（`*postgresadapter.Store`）已经实现后三者；`SearchBuilding` 已是 `SearchReads` 的（`ports.go:894-921`），`GrepReads` 重复它的签名，同一个方法。

#### 3.4.2 候选的语句

```sql
-- name: GrepCandidates :many
-- grep's candidates in a notebook (M9/P4 design 3.4.2): the pages whose body holds every key, those queued exact,
-- and those whose row leaves text of the content out (!cut); ids alone, in no order. Each branch reads this
-- notebook's alone: the keys' and !cut's bitmaps behind its prefix (page_search_hits, 00041), the queue by
-- page_search_queue_notebook_id_bulk_idx. Nothing is joined to nodes or page_contents: the use case takes the live
-- pages, and their order, from the tree it read (M8/P5's two traps of the join never arise). Bulk nodes are not
-- candidates: building says so.
SELECT h.id::uuid AS id
FROM page_search_hits(ARRAY[sqlc.arg(notebook_id)::uuid], sqlc.arg(keys)::text[], NULL::text[]) h (id)
UNION
SELECT q.node_id FROM page_search_queue q WHERE q.notebook_id = sqlc.arg(notebook_id)::uuid AND NOT q.bulk
UNION
SELECT c.id::uuid FROM page_search_hits(ARRAY[sqlc.arg(notebook_id)::uuid], ARRAY['!cut']::text[], NULL::text[]) c (id);
```

- 不取键时不调它：候选是树里活着的全部页（3.7.3）。
- 命中是位图的读（函数自己的 `SET` 关掉别的扫描，`00041:90-103`），键越多交集越小；常见的键（`the` 的三元组）在一本 10 万页的笔记本里给出几万个 id：语句只传 id（16 字节一个），10 万个约 1.6 MB。与树取交在 Go 里（一个 `map[uuid.UUID]struct{}`），00 号文档 4.13 要 P4 量 10 万个候选时取交的代价：守卫的一行（6.1）以 10 万页、每页都有同样的 32 个键（`MaxGrepKeys` 个位图的交是这条语句最坏的情形，审查 A-n4；行以 SQL 写下）量这条语句与取交；离线只能估 Go 的一半（10 万次查表约 5 ms）。
- 存储照 M8 的 `stateOnly` 的写法只记 SQLSTATE（13.1 第 10 条），不记键（键是正文的片段）。

#### 3.4.3 正文的语句

复用导出的 `Contents`（`queries/contents.sql:47-49`，`node_id = ANY(@node_ids)`，主键），注释改为"an export's batch, and grep's"。一批的 id 由用例按 `PageStates` 的字节数选，至多 `GrepBatchBytes` = 4 MiB（一页大于它时一批只有它，至多 5 MiB）：内存里同时只有一批（3.6.5）。读到的正文是那一刻的：`PageStates` 之后被写大的页照读到的算字节（3.7.3），被删除的页不在其中、跳过。页的状态在走查开始时读一次（至多 501 页），之后的各批在约 12 秒里读：这期间被写大的页让一批多过 `GrepBatchBytes`，所以"同时至多一批"的内存只在没有并发的大的写时成立；一页至多 5 MiB，大的请求体整个服务同时至多两个（4.14），实际多出的有限（审查 A-n3），3.6.5 照这样写。

### 3.5 键

#### 3.5.1 函数

```go
// GrepKeys is the keys of re, a grep's pattern parsed in Go's NFC (Perl's
// flags, FoldCase with ignore_case): lexemes of the body index that every
// page holding a line re matches holds, unless the page is partial (M9/P4
// design 3.5); at most MaxGrepKeys of them, in the order of the segments
// they come from, each once; none for a pattern no key reads, as TODO|FIXME
// or \d{4}, whose grep walks every page of the notebook within its bounds.
func GrepKeys(re *syntax.Regexp) []string

// MaxGrepKeys bounds a pattern's keys: dropping the others leaves a
// superset (M9 design 4.13).
const MaxGrepKeys = 32
```

在 `page/domain/grep_keys.go`，挨着分词：它用 `classOf`、`searchFold`、`lexemes.word` 与 `longASCII`（都未导出，`search.go:206-375`），键与词由同一份代码算出。树是解析之后、`Simplify` 之前的（同指令数的估计，3.6.1）：`Simplify` 把 `x{3}` 展开成 `xxx`、`x{2,}` 展开成 `xx+`，对事实没有影响，而它分配。

#### 3.5.2 树上的事实

一个字面的"原子"是一个码位，或者带折叠的标记（`(?i)` 的字面量：它代表这个码位的简单折叠的轨道上的每一个，`unicode.SimpleFold` 走一圈），或者**边界**：一个只匹配分词里属于"别的"一类的字符的位置（这样的字符本身，或一行的开头、结尾）。每个节点的事实（`grepFacts`）：

- `exact`：每个匹配都恰好是这一串原子时给出（`isExact`）；
- `pre`、`suf`：每个匹配都以它们开头、结尾；
- `must`：每个匹配都含有的若干串（连续的原子）。

按节点的种类（`syntax.Op`）：

| 节点 | 事实 |
|---|---|
| `OpLiteral` | `exact` 是它的码位（`FoldCase` 时每个带折叠的标记） |
| `OpCharClass` | 类里每个码位都属于"别的"时 `exact` 是一个边界；否则什么也没有 |
| `OpBeginLine`、`OpEndLine`、`OpBeginText`、`OpEndText` | `exact` 是一个边界（零宽：一行的开头与结尾两边在分词的文字里是 `\n`、正文的开头前是名称加 `\n`，`\r` 与 BOM 都属于"别的"） |
| `OpWordBoundary`、`OpNoWordBoundary`、`OpEmptyMatch` | `exact` 为空（不消耗字符，两边的原子在匹配里相邻；不是边界） |
| `OpCapture` | 子节点的 |
| `OpConcat` | 相邻的 `exact` 连起来；一个不是 `exact` 的子节点之前的那一串连上它的 `pre` 成为一个 `must`（只要它在最前面，也是 `pre`），它的 `must` 照收，之后的那一串从它的 `suf` 起；全部子节点都是 `exact` 时整个是 `exact` |
| `OpAlternate` | 各支都是同一个 `exact` 时就是它；否则 `pre`、`suf` 是各支的公共的前缀、后缀，`must` 只有这两串 |
| `OpPlus`、`OpRepeat` 而 `Min` ≥ 1 | 子节点的 `pre`、`suf`、`must`（第一次以 `pre` 起、最后一次以 `suf` 止、每次都含 `must`）；`x{n}` 而 `x` 是 `exact` 时是 `n` 份连起来的 `exact`（至多 1,024 个原子） |
| `OpStar`、`OpQuest`、`OpRepeat` 而 `Min` = 0、`OpAnyChar`、`OpAnyCharNotNL`、`OpNoMatch` | 什么也没有（可以匹配任何东西或什么也不匹配） |

整个模式的段：`isExact` 时是 `exact` 一段，否则是 `must`。每一段在每个匹配里出现、是这一行里连续的文字（边界的原子除外：它们只说那里是一个"别的"字符或行的两端）。

#### 3.5.3 从段到键

一段在边界处分成**片**（片的左边有边界时 `left`、右边有时 `right`），每片：

1. **去掉开头的组合的部分**：从第一个"轨道里每个码位前面都有 NFC 的边界"的原子起；去掉过就不再 `left`。一个以组合记号开头的片在正文里与前面的未知的字符合成，键说不准。`(?i)` 的 ι 的轨道（U+0345、U+0399、U+03B9、U+1FBE）里 U+0345 前面没有边界，所以开头的 `(?i)ι` 也去掉。
2. **右边不是边界时，去掉最后一个组合序列**（最后一个"轨道里每个码位前面都有边界"的原子与它之后的原子）——当它可能与后面未知的字符合成时：它的起始的原子的轨道里有一个码位在 96 个"危险的结尾"里（码位 r 与某个记号 m 在 NFC 里不合成，`searchFold(r)` 与 `searchFold(m)` 在 NFC 里合成；如 `H`、`J`、`T`、`W`、`Y`、`ß`、`İ`、`ŉ` 与带 iota 的希腊字母），或它的折叠里有前面没有边界的码位（70 个，如 `İ` 折叠成 `i` 加 U+0307），或**它之后的原子的轨道里有折叠之后变了的码位**。NFC 的文字里这样的码位只有 U+0345（折叠成 ι，一个起始字符，能与后面的记号合成）：前面没有边界、折叠之后变了的码位共 8 个，另 7 个（U+0340、U+0341、U+0343、U+0344、U+0F73、U+0F75、U+0F81）都被 NFC 拆开，不在 NFC 的模式里（`exp/keys2` 的 `marks_test.go`）。例：`(?i)abΉͅ` 的结尾是 `Ή` 与 `(?i)ͅ`，行 `abΉΙ͂`（`Ι` 是轨道上的另一个，后面跟 U+0342）在正文里折叠成 `abήῖ`；不去掉就取出 `bήι`，正文里没有它（3.5.4 的离线核对；`TestGrepKeys` 的一行）。
3. **折叠**：每个原子各自 `searchFold`（带折叠的取轨道的第一个：轨道上每个码位的 `searchFold` 相同，`exp/keys` 对全部码位核对，0 对不同），连起来，再 NFC：这就是这一片在分词的文字里的样子 `f`。
4. **从 `f` 取键**，照分词的规则（`search.go:293-375`）：CJK 的连续两个字给二元组；字母与数字的一串，3 个起给它的三元组（`lexemes.word`；串超过 64 个而全是 ASCII 时照样给三元组：正文里这样的串让页成为 `partial`，页靠 `!cut` 是候选，键多了无害）；1–2 个的串只在两边都是已知的边界时给整词：串的左边是 `f` 里一个"别的"或 CJK 的字符，或者它在片的开头而片 `left`；右边同样。

在 `(?i)` 的 ι 的轨道处把片切开的一步不要：它与第 2 步的最后一条互为多余，两条都去掉时才有反例（3.5.4；审查 A-m8、C-I2）。

96 个危险的结尾是 `grep_tables.go` 里生成的表（`go generate`）；`grep_tables_test.go` 按定义对全部码位与全部记号重算、比较（约 1 秒，`-short` 也跑：13.4 第 8 条只让按耗时判定的界线在 `-short` 下跳过，审查 C-N5）。属于"别的"而折叠之后不是的码位只有 14 个（U+2ADC 与 13 个音乐符号 U+1D15E–U+1D164、U+1D1BB–U+1D1C0，`exp/claims`），它们都被 NFC 拆开、不出现在 NFC 的行里，所以一个"全是别的"的类在行里匹配的字符折叠之后仍是"别的"，不用除外。轨道的检查、"前面没有 NFC 的边界"由 `norm.NFC.PropertiesString(…).BoundaryBefore()` 当场算。

**例子**（`exp/keys/samples_test.go`）：`foo` → `foo`；`(?i)FOO` → `foo`；`func\s+\w+\(` → `fun`、`unc`；`TODO|FIXME` → 无（各支没有公共的部分）；`数据库设计` → `数据`、`据库`、`库设`、`设计`；`\bai\b` → 无；`" ai "` 与 `^ai$` → `ai`；`^# Title$` → `tit`、`itl`、`tle`；`Straße` 与 `(?i)STRASSE` → `str`、`tra`、`ras`、`ass`、`sse`；`a{3}` → `aaa`；`(abc)+def` → `abc`、`bcd`、`cde`、`def`；`(?:foo|foobar)baz` → `foo`、`baz`；`\.md$` → `md`；`README` → `rea`、`ead`、`adm`、`dme`。

#### 3.5.4 为什么是超集

要证明的（性质测试照它判定，6.1）：一页的正文里有一行 ℓ，`re` 匹配 `NFC(ℓ)`；页不是 `partial`；那么 `GrepKeys(re)` 的每个键都在 `SearchTokens(name, &content)` 的正文的词里。

1. **事实成立**（对树做归纳）：对 `re` 的每个节点的每个匹配 m（`NFC(ℓ)` 的一段 `[s, e)`），`exact` 给出时 m 就是那串原子（每个字面原子由它的轨道里的一个码位匹配，边界由一个"别的"字符或一行的两端）；m 以 `pre` 起、以 `suf` 止，含有每个 `must`。表里每一行照 RE2 的语义直接成立：拼接的子匹配首尾相接；选择的匹配是某一支的；`+` 与下限至少 1 的重复至少有一次；零宽的 `\b`、`\B` 两边相邻；`^`、`$` 在一行的两端（逐行匹配，`(?m)` 与 `(?s)` 对没有 `\n` 的一行不改变什么）。
2. **一片在正文的折叠里的样子**：分词的文字是 `searchFold(name + "\n" + content)`，它等于 `searchFold(name) + "\n" + …`，各行由 `\n` 隔开（`\n` 是 NFC 的边界，折叠也不碰它）。设一片的原子在 `NFC(ℓ)` 里匹配文字 t，t 的左邻是 a、右邻是 b（或行的两端）。`searchFold` 是 NFC ∘ 折叠 ∘ NFC 再换切罗基文的大小写；对 `a t b` 折叠再 NFC，t 的部分是不是 `f`（第 3 步）只取决于 t 两端的组合序列与 a、b 合不合成：第 1 步保证 t 的第一个原子前面有边界（它与 a 不合成，折叠之后的第一个码位前面也有边界：没有一个前面有边界的码位折叠成以组合记号开头的串，`exp/claims` 对全部码位核对）；第 2 步在 b 未知时去掉了会与 b 合成的结尾（`riskyTails` 按定义对每个码位与每个记号算出；结尾里是 U+0345 的轨道时，它折叠成 ι，ι 能与 b 合成，由那一步的最后一条去掉），右边是边界时 b 是"别的"（`classOther` 的字符前面都有 NFC 的边界，`exp/claims` 核对），也不合成。t 的中间：每个原子由它的轨道里的一个码位匹配，轨道上每个码位的 `searchFold` 相同，逐个原子折叠再 NFC 与这一段在正文里的折叠相同；U+0345 的轨道折叠成 ι，是起始字符，不与前面的字符合成（在 NFC 的行里 U+0345 与它能合成的字符已经合成，那时 `re` 不匹配它们分开的原子）。所以折叠之后的文字里有 `f` 这一段。
3. **键在词里**：`f` 里两个相邻的 CJK 字在正文的文字里也相邻，分词给出这个二元组（`scan` 对每对相邻的 CJK 字都给，正文不收单字）；`f` 里一串 n ≥ 3 个的字母与数字在正文的文字里是一串更长或一样长的同类字符的一段，那串的三元组包含这一段的三元组（`word` 对每三个相邻的字符给一个），除非那串超过 64 个而全是 ASCII（那时页是 `partial`，不在这个比较里）；1–2 个的整词只在两边都是已知的边界时取，那时正文的串恰好就是它，分词给出整词。`data:` 的跳过与上限之后的词也只发生在 `partial` 的页上（3.3.1）。

不进这个比较的：`partial` 的页（它们靠 `!cut` 总是候选，3.4.2，另有"被截掉的页总被走查"的测试）；名称（`grep` 不搜名称，正文的词含名称的只是多出的候选）。

**离线的核对**（`exp/keys` 是原型，`main.go` 是性质测试的原型；`exp/keys2` 是审查 C 的 `keysmut`，即同一个原型加 `-mutant`，变异可以组合，这里的规则是它的 `-mutant 106`）：随机的模式（拼接、选择、`*`、`+`、`?`、`{1,3}`、`{2}`、`{0,2}`、`+?`、`^`、`$`、`\b`、`\B`、分组、13 种字符类，四分之一带 `(?i)`）与随机的行（字母表有 ASCII、组合记号、折叠成 ASCII 的码位、会合成的希腊字母、CJK、谚文与字母、切罗基文、标题大小写、全角、软连字符，两边随机地加上下文），只取 Go 的 `regexp` 匹配 `NFC(行)` 的。

- **修订的规则**：4 个种子各 30 万次，与审查 C 的定向的字母表（`-d`：危险的结尾、使它们危险的记号、iota 的轨道与它能合成的希腊字母）4 个种子各 30 万次，共 240 万次，0 次漏键；穷举（`-e`，审查 C 的：`ab` 加 X 加 [M] 的模式，带与不带 `(?i)`，X 取 96 个危险的结尾与一些希腊字母，M 取 U+0300–U+036F 与 U+0345；行取每一种轨道的替换与每一个跟着的记号、iota 与几个字母）306,951 行，0 次漏键，约 1 秒。审查 A 另以自己的对抗的生成器（410 万次）、iota 的定向试验与右边的穷举（171,558 种）核对，对这里的规则另跑 60 万次与 200 万次 iota 的（对修订的核对 A2），0 次漏键。
- **每处修正的变异**（6.3 的第 137–141 条；数是首次失败的试验，括号里是漏键的次数，"—"是 30 万次里没有失败）：

| # | 变异 | 穷举 | 随机，种子 1、2 | 定向，种子 1、2 | 性质测试（6.1 的 2 万次，10 个种子里失败的） |
|---|---|---|---|---|---|
| 137 | 结尾的风险只看最后一个码位 | 365,554 次漏键 | 8,521（13）、26,447（13） | 26,533（59）、2,690（50） | 6 个 |
| 138 | 带折叠的原子的风险只看存下的码位 | 4,386 | 52,298（2）、137,928（3） | —、88,874（1） | 2 个 |
| 139 | 结尾不看之后的原子的折叠 | 820 | — | — | 0 个 |
| 140 | 对整片 `searchFold` | 0 | 282,592（4）、85,407（5） | 50,042（51）、28,318（67） | 10 个（首次 261–7,441） |
| 141 | `\b`、`\B` 当作边界 | 0 | 392、345 | 79、2,516 | 10 个（首次 38–496） |

  所以每一条都有一个必定失败的测试：`TestGrepKeys` 里每处修正一行，取下面的反例；穷举成为 `TestGrepKeysOverTheRiskyTails`（不在 `-short` 下跑，约 1 秒），抓第 137–139 条；性质测试另抓第 140、141 条与一部分第 137、138 条（6.1、6.3）。
- **每处修正的反例**（`exp/keys2/rows_test.go`：修订的规则下键都在词里，那条变异下有一个不在）：第 137 条 `abſ́` 在 `abſ́̇` 上（变异取 `abś`）；第 138 条 `(?i)abſ` 在 `abſ́` 上（取 `abs`）；第 139 条 `(?i)abΉͅ` 在 `abΉΙ͂` 上（取 `bήι`）；第 140 条审查 C 的定向的反例 `(?i)ᾼ͂ḢＡ가ﬆ` 在 `ᾼ͂ḢＡ가ﬅ̊` 上（修订的规则取 `αῖḣ`、`ῖḣａ`，变异取 `ᾶιḣ`、`ιḣａ`）；第 141 条 `文(e)\b` 在 `B文eς` 上（取整词 `e`）与 `\bai\b`。
- **去掉的一步**："在 U+0345 的轨道处切开"（原型的 `-mutant 106`）与第 2 步的"之后的原子折叠之后变了"（审查 C 的 `-mutant 1041`）各自去掉时都没有反例（审查 A、C：几百万次随机与定向的试验与穷举；穷举 306,951 与 311,727 行），审查以为两条都可以去掉；**两条一起去掉时穷举有 820 次漏键**（上面第 139 条的反例就是其中之一）：中间的 U+0345 的轨道由逐个原子的折叠管，结尾的由两条里任何一条管。照主持者的规则（有反例就留着那条、以反例为一行），留下第 2 步的这一条（它只在结尾去掉一个组合序列；切开要丢掉跨过它的全部键），去掉切开。
- 去掉那 14 个码位的除外之后再跑 20 万次（种子 3），仍是 0 次漏键（`exp/keys14`），与上一段的推理相符。

### 3.6 程序、工作与名额

#### 3.6.1 模式与指令数

```go
// The bounds of a grep (M9 design 4.13, 4.14; M9/P4 design 3.6): the
// work ones are measured at full load (3.6.4) and a test's less.
const (
	MaxGrepPatternBytes = 1024
	// MaxGrepInsts bounds a pattern's program as GrepInsts estimates it,
	// before it is compiled.
	MaxGrepInsts = 2000
	DefaultGrepLimit, MaxGrepLimit = 100, 500
	// MaxGrepLineText is the most bytes of a line an answer shows.
	MaxGrepLineText = 1000
	// MaxGrepRead is about the most bytes of contents a call reads, as
	// backlinks' (linking/domain/context.go:23-26), which page cannot import.
	MaxGrepRead = 32 << 20
	// GrepBatchBytes is about the most bytes of contents a statement reads.
	GrepBatchBytes = 4 << 20
	// MaxGrepLineWork bounds a line's work, MaxGrepWork a call's: a unit
	// is a byte of a line by an instruction of the program.
	MaxGrepLineWork int64 = 150_000_000
	MaxGrepWork     int64 = 400_000_000
	// GrepLineOverhead is a line's work beyond its bytes, by instruction:
	// the loop's and a match's fixed cost, under 2 bytes of the costliest
	// instruction offline (M9/P4 design 3.6.4) and again at full load.
	GrepLineOverhead = 2
	// GrepSoftMargin is how long before the call's deadline grep starts no
	// more lines; GrepUnitTime what a unit is thought to take before the
	// call has timed its own.
	GrepSoftMargin = 3 * time.Second
	GrepUnitTime   = 10 * time.Nanosecond
	// GrepSlotWait is how long a call waits for its slot before 503.
	GrepSlotWait = time.Second
)

// GrepPattern is a pattern grep took: its expression as it compiles, its
// program's instructions as estimated and its keys.
type GrepPattern struct {
	expr  string
	Insts int64
	Keys  []string
}

// ParseGrepPattern parses a grep's pattern (M9/P4 design 3.6.1): 422 on
// pattern when it is longer than MaxGrepPatternBytes (too_long), holds a
// newline (invalid_format: a line never does), does not parse
// (invalid_format, its syntax.ErrorCode alone: never the pattern's text),
// or is past Go's bounds (too_long: syntax.ErrLarge, ErrNestingDepth, a
// repeat over 1,000) or MaxGrepInsts as GrepInsts estimates it (too_long:
// too complex). It parses the pattern's NFC with regexp.Compile's flags,
// "(?i)" before it with ignoreCase, and compiles nothing.
func ParseGrepPattern(pattern string, ignoreCase bool) (GrepPattern, error)

// GrepInsts is the instructions of re's program as calcSize estimates them
// (regexp/syntax/parse.go:212), and the program's start and match: never
// fewer than syntax.Compile makes (M9/P4 design 3.6.1).
func GrepInsts(re *syntax.Regexp) int64

// Compile is p's program, once the call holds its slot.
func (p GrepPattern) Compile() (*regexp.Regexp, error)
```

```go
// GrepParses is the process's queue of grep's parses (M9/P4 design
// 3.6.1): a pattern's parse, its estimate and its keys, at most
// GOMAXPROCS at once, so that the memory of the parses is at most so many
// of the worst (about 7.3 MB each) and their CPU at most so many cores. A
// call waits for its turn within its ctx and is never refused: what it
// answers does not depend on the load, only when.
type GrepParses struct {
	sem *semaphore.Weighted
	// parse is domain.ParseGrepPattern; a test's fake counts the parses
	// in flight.
	parse func(pattern string, ignoreCase bool) (domain.GrepPattern, error)
}

func NewGrepParses(n int) *GrepParses

// Parse is q.parse in its turn, the turn held until it returns; ctx's
// error when ctx ends while it waits, which mcpserver answers as any call
// past its deadline.
func (q *GrepParses) Parse(ctx context.Context, pattern string, ignoreCase bool) (domain.GrepPattern, error)
```

- **NFC**：模式按 Go 的 NFC 规整之后解析（4.13 的"模式里的字面量都按 Go 的 NFC 比"）；`\uXXXX` 一类的转义不变，只有字面的字符合成或分解。NFC 不会跨过元字符重排（元字符都是前面有边界的 ASCII）。1,024 字节是给出的模式的；它的 NFC 至多长到 3 倍，但会变长的只有非 ASCII 的字面字符，解析的分配随字符类而不是字面字符（2.1 第 1 条），界线照旧。
- **`calcSize` 的副本**：go1.27.1 的 `parse.go:212`（与 GOROOT 的 1.26.2 相同）对每个节点：字面量是码位数，`OpCapture` 与 `OpStar` 是 2 加子节点，`OpPlus`、`OpQuest` 是 1 加子节点，拼接与选择相加（选择另加支数减 1），重复照 `x{n,m}` 的展开，至少 1。它在 `parse.go` 里是 `parser` 的未导出的方法，只在 `maxSize` 的检查里用（`:103`），所以复制约 40 行，注释写出处。估计从不小于程序：多出的是程序的开头与匹配两条，加上每个操作数不能为空的星号一条（`calcSize` 按 2 加子节点算，编译只要一个 `alt`），重复里的星号乘以次数（`a*b*c*` 估计 11、程序 8；`(?:a*){1000}` 3,002 对 2,002）；差不止 2（审查 A-m6 = C-M1：随机的生成器的 2 万个里 7,877 个多出 2 以上，最多 321）。`grep_test.go` 以 2 万个随机的模式与这些边上的模式核对 `len(syntax.Compile(Simplify(re)).Inst) ≤ GrepInsts(re) ≤ 2 × 它 + 2`（`exp/insts2`：2 万个里比值最大恰是 2，边上的都在其内）：下界守住估计（Go 升级时），上界只抓粗的多算。
- **拒绝的码与文字**：`too_long` 的消息说"too complex: about N instructions, at most 2000; large repeats such as [a-z]{999} cost the most"，不复述模式；语法错误的消息是 `syntax.ErrorCode` 的文字（"missing closing )"一类），不带 `syntax.Error.Expr`（它是模式的片段）。
- **为什么先估计**：拒绝在名额之前、与资源无关（4.15）；不编译就不分配编译的 14–27 MB。通过的模式在拿到名额之后才 `Compile`（同一个表达式再解析一次，在名额里）。
- **解析的队列**（2.1 第 1 条，审查 A-I1；这份设计里解析的代价与队列的数只写在这里，别处引它）：解析本身也不便宜，而它在笔记本的判定与名额之前。1,024 字节之内最贵的是许多个分开的 `\pL`（341 个）：一次分配 7.25 MB、留着 2.84 MB 的树、约 1.15 ms；折叠的大范围（40 个 `(?i:[\x{100}-\x{10ffff}])`）不费内存，一次 52 ms 的 CPU（`appendFoldedRange` 逐个折叠约 12.4 万个码位）；`\pL{999}` × 113 是 2.75 MB（重复共用一份范围）。没有界线时一个 PAT 的 200 个同时的调用（`exp/parseq`，GOMAXPROCS 18）：`\pL` × 341 峰值 +242 MB（每棵树多留 20 ms 时 +754 MB，审查 A 量的）、分配 1,451 MB；折叠的范围约 10 秒的 CPU。所以 `Grep` 经 `GrepParses.Parse` 解析（`page.New` 以 `runtime.GOMAXPROCS(0)` 建一个，进程里只有一个；同解析的预算，`platform/markdown/budget.go:47-101`，但不答 503）：调用在自己的 ctx 里等（`semaphore.Weighted.Acquire`），ctx 结束时答它的错误，mcpserver 照"过了期限"答 `internal_error` "the call ran out of time"（warn）或"the client went away"（debug，`mcpserver/errors.go:75-99`）；解析、估计与取键做完就放还，树不留（`GrepPattern` 只留表达式、指令数与键）。4.15 的码的次序不变：422 仍在 404 之前，队列只改快慢。同一组调用在队列下：`\pL` × 341 峰值 +126 MB（18 个名额；8 个时 +50 MB），分配照旧 1,451 MB，最长等 158 ms；折叠的范围合计 0.9 秒、最长等 0.82 秒（8 个名额 1.5 秒）；`\pL{999}` × 113（被估计拒绝）峰值 +55 MB（不排队 +114 MB）。所以内存至多名额数乘约 7.3 MB，CPU 至多名额数；00 号文档 4.13 写这两个数，它的第 10 节、"写给后面的 M"与 M12 的性能移交引 4.13（第 7 节）。
- **测试的钩子**：`GrepParses.parse` 默认是 `domain.ParseGrepPattern`，测试换成一个数同时在做的解析、每次停几毫秒的假的（审查 A2-m1 = C2-M1）：钩子包着解析本身，所以解析挪到名额之外时（第 144 条变异）同时的解析多过名额数、测试失败；钩子若只在持着名额时被叫，它数的是名额，抓不到这条变异。

#### 3.6.2 一行的工作与截断

```go
// GrepWork is a line's work: its bytes, and GrepLineOverhead, by the
// program's instructions.
func GrepWork(lineBytes int, insts int64) int64

// GrepLineCut is the most bytes of a line grep matches within
// maxLineWork: a longer line is matched in its first so many bytes, cut
// at a character's boundary, and listed in partial_lines unless a match
// ends before the cut (M9 design 4.13).
func GrepLineCut(insts, maxLineWork int64) int
```

- 一行在匹配之前是 `NFC(行)`（已是 NFC 的行不复制，`norm.NFC.IsNormalString`）；字节数是 NFC 之后的。
- 截断：`NFC(行)` 长于 `GrepLineCut` 时只匹配它开头的那么多字节（退到字符的边界）。命中结束在截断之前：是整行的真命中（RE2 没有向后看，结束之前的 `$`、`\z` 到不了，`\b`、`\B` 只看截断之前的字符）。最左的命中结束在截断处、或没有命中：这一行进 `partial_lines`（路径、行号），不算命中（2.1 第 6 条）。所以 `foo$` 不会在截断处"命中"，`partial_lines` 里的行 agent 以 `read` 读全。
- 2,000 条指令的模式匹配一行开头的约 75 KB；10 条的匹配得完 5 MiB 的一行。

#### 3.6.3 一次调用的工作、软期限与预先判断

- 一次调用的工作是它匹配过的各行的 `GrepWork` 之和（截断的行按截断的字节）。
- **预先判断**：每一行开始之前估它的工作 w；本次调用已经查过至少一行时，`已做 + w > MaxGrepWork`，或 `现在 + w × 每单位的耗时 > 软期限` 就不开始这一行，停在这一行之前，答已找到的与游标，`incomplete: true`。软期限是调用的 ctx 的期限减 `GrepSoftMargin`（3 秒；15 秒的 `server.request_timeout` 时是开始之后约 12 秒）；ctx 没有期限时只有工作的界线（测试的直接调用）。每单位的耗时取 `max(这次调用自己量的, GrepUnitTime)`（量的是匹配的用时除以做过的工作；`GrepUnitTime` 是 10 ns，比离线最贵的 8.6 ns 略高，3.6.4）：负载重时它自己变慢、估计随之变大；`regexp` 以字面的前缀跳读时一段便宜的行量出的远低于 1 ns，取大的一个就不让下一行在软期限之前一刻开始而越过它（审查 A-n1；`MaxGrepWork` × 10 ns = 4 秒，在 12 秒之内）。
- **至少一行**：第一行不预先判断，所以每次调用至少前进一行；它的工作至多 `MaxGrepLineWork`（暂定的 1.5 × 10⁸ 单线程约 1.3 秒；3.6.4 照满负载下一行不过约 2 秒定它），在软期限的 3 秒之内。
- 一行一旦开始就做完（`regexp` 不看 ctx），所以是预先判断，不是开始之后再看时间（4.13）。读正文的语句之前同样看软期限：过了而这次调用**已经查过至少一行**时就不读（同 `MaxGrepRead` 的例外）；还没查过一行时照读，第一行照查（否则名额的等待与前面的几条语句用掉软期限时，调用什么也不查、以带来的游标答 `incomplete`，agent 拿同一个游标打转；审查 A-m3）。

#### 3.6.4 P4 怎样量 `MaxGrepWork`、`MaxGrepLineWork` 与 `GrepLineOverhead`

照 M8 的规则（满负载下一次调用的匹配不超过 5 秒、一行不超过约 2 秒的最大一档；[M8/P5](../M8-history-search/05-P5-search.md) 3.9 的做法）。量两次（审查 C3-M1）：**S1 的第 1 步**，离线、在任何与 3.3.4 的那一支有关的代码之前，以 `grep` 每行的循环的副本（`exp/lineoh2` 的：为预先判断读时钟、切行、去掉 `\r`、NFC 的检查、匹配）定四个常量，并以它们核对 3.3.4 的规则；**第 9 步**以真的用例（守卫的测试与只在本机跑的 `BenchmarkGrepAtFullLoad`）重量，作最后的核对。两次的做法相同：

1. **满负载**：`max(1, GOMAXPROCS/2)` 个同时的 `grep`（占满名额），另有同样多的 goroutine 做别的 CPU 的活（第 1 步是别的正则，第 9 步是同一组正文上的 `search` 与 `list` 的最坏的 glob），即主机的全部核都忙。
2. **模式**：三种形状都用 **36 支的选择** `(?:[\pL\pN\pM\pS\pP]{70}QZ|[\pL\pN\pM\pS\pP]{69}QZ|…|[\pL\pN\pM\pS\pP]{35}QZ)`。
   - 类写成 `[\pL\pN\pM\pS\pP]`：与 `[\p{L}\p{N}\p{M}\p{S}\p{P}]` 是同一个类、同一棵树，整个模式 867 字节，在 `MaxGrepPatternBytes` 之内（长的拼法是 1,227 字节，用例答 `too_long`，审查 C3-M2）。
   - 每支的次数不同，解析器不把它们的开头提出来（开头相同的各支会被合成一支：审查 B2-M2 的例子 `[…]{70}AA|[…]{70}BA|…` 的 28 支只编译出 155 条）；`GrepInsts` 与编译的程序都是 1,999 条。
   - 最短的匹配 37 个字符，放得进一行：输入短于最短的匹配时 `regexp` 立刻返回（`regexp/exec.go:527`），重复到 2,000 条的类的最短的匹配是 1,990 个字符，在 80 字节的行上每行 2 ns，量不到真的工作（审查 B2-M2）。
   - 正文是类处处匹配、没有 `Q` 的 ASCII（小写字母与数字）。它在长行上是量过的最贵的指令（8.58 ns 一单位，重复到 2,000 条的类 7.75 ns）；ASCII 是每单位最贵的文字（审查 A 的核对：多字节的文字按字节算，每单位更便宜）。
3. **形状**（审查 A3-m1）：
   - **一行的界线**：一行 5 MiB 的页（匹配它开头的 `GrepLineCut` 字节）；取 0.5、0.75、1、1.5、2、3 × 10⁸ 里满负载下每行的用时的 p99 不过 2 秒的最大一档。
   - **一次的界线**：500 页各 64 KiB、每行 4 KiB 的页，与每行 80 字节的页，以两者里每单位更贵的那一个定（4 KiB 的行上线程长满；80 字节的行上线程在一行的开头几十个字符里才长满，每单位约是长行的三分之二，只用它定会高一档）；取 2、3、4、5、6 × 10⁸ 里一次调用的匹配的 p99 不过 5 秒的最大一档。
   - **`GrepLineOverhead`**：只由每行 80 字节的页与下面的离线的校准的各种短行定，照下面的数法。

   量的是 M 系列的本机（同 M8），CI 的倍数由守卫的时限照 3 倍的规则留（6.4）。
4. 结果写进 9.1，常量改成量出的；`GrepUnitTime` 取满负载下量出的每单位的耗时。之后照 3.3.4 以新的 `MaxGrepWork` 与 `GrepLineOverhead` 重算被标出的比例的第 3 条。

**`GrepLineOverhead` 的数法与离线的校准**（`exp/lineoh`，GOMAXPROCS 1；审查 A2-m2 = B2-M1 之后）：工作的数法要让每一行的用时不过 `(NFC 的字节 + L) × 指令数 × U`，U 是最贵的指令在长行上每单位的耗时；L 是让每个量过的模式的每种行都成立的最小的整数。量的是 `grep` 对一行做的全部：为预先判断读时钟（3.6.3）、在 `\n` 处切开并去掉 `\r`、NFC 的检查、匹配。

- **U**：长行上每单位的耗时，36 支的选择 8.58 ns（64 KiB 的一行）、重复到 2,000 条的类 7.75、`[…]{200}Q` 7.33；U 取 8.58。
- **每行**：24 个模式（3 条指令的 `Q`、`.`、`\w`、`^$` 到 2,000 条的两个，含 `foo`、`TODO: \w+`、`(?i)todo:\s+\w+`、`(?i)(error|warning|fail)Q`、`\d{4}-\d{2}-\d{2}Q`、`[…]{17}Q`、36 支的选择），0、1、4、16、40、80、200 字节的行，单词与空格的文字与密的文字两种。一行的固定的用时：空行 31–33 ns，1 字节的行 35–75 ns；按指令折算，要的 L 最大的是最小的模式：`\w` 在 1 字节的行上 75 ns，对 3 条指令 × 8.58 ns 是 L = 1.89；10 条以上的模式都不到 0.4；大的模式在短行上比它们的字节还便宜（36 支的选择在 80 字节的密的行上 0.98 ms，它的 80 个字节按 U 是 1.37 ms，审查 B）。所以 **`GrepLineOverhead` = 2**（第一次修订的 64 没有量过）。
- **核对**：每行 80 字节、10 页 8,200 行，36 支的选择每行 993 µs、L = 2 时每单位 6.13 ns，在 U 之下；3.3.4 的被标出的 19 页上（27,879 行）六个模式的用时是工作乘 U 的 0.04–0.34。
- **NFC 的副本不在校准里**（审查 A3-n1）：校准量的是 NFC 的检查；不是 NFC 的行先规整再匹配，规整每个输出的字节 7–33 ns（`e` 加 U+0301 29.7、谚文的字母 32.9、U+1D160 7.2），3–4 条指令的模式按 `(字节 + 2) × 指令数 × U` 记的少了至多约 28%。`MaxGrepRead` 界住它：一次调用至多 32 MiB 的输入、每字节至多约 20 ns，约 0.7 秒，不另记。
- 满负载下全部的耗时一起变慢，比值大致不变；第 1 步在满负载下照上面第 1–3 条重新量 L。

**离线的满负载的估计**（`exp/lineoh2`：GOMAXPROCS 18，6 个性能核、12 个能效核；9 个 `grep` 的与 9 个别的 CPU 的 worker，每种形状 3 秒，一块约 5 × 10⁷ 个单位，共 18 块；第 1 步量够长的时间与次数）：

| 形状 | 单线程的每单位 | 满负载的中位数 | p99 | p99 下 4 × 10⁸ | 界线之内的最大一档 |
|---|---|---|---|---|---|
| 每行 80 字节 | 5.86 ns | 11.83 ns | 14.82 ns | 5.93 秒 | 一次 3 × 10⁸ |
| 每行 4 KiB | 8.96 ns | 13.95 ns | 22.52 ns | 9.01 秒 | 一次 2 × 10⁸（4.5 秒） |
| 5 MiB 的一行的截断（75 KB） | 9.06 ns | 21.03 ns | 21.19 ns | — | 一行 0.75 × 10⁸（1.6 秒；1 × 10⁸ 是 2.1 秒） |

中位数在性能核上，p99 在能效核上。所以预计 `MaxGrepWork` 是 2 × 10⁸（较慢的形状，每行 4 KiB）、`MaxGrepLineWork` 是 0.75 × 10⁸（2,000 条指令的模式匹配一行开头的约 37 KB）；3.3.4 的第 3 条在 2 × 10⁸ 下是界线的 74%，仍成立。暂定的 4 × 10⁸ 与 1.5 × 10⁸ 照旧写在常量里，到第 1 步量出为止；一行的界线低于 1.5 × 10⁸ 时，"至少一行"的工作（3.6.3）随之更短。

#### 3.6.5 名额、连接与内存

```go
// GrepSlots are the calls that read and match at once in the process,
// max(1, GOMAXPROCS/2) as the parse budget's (M9 design 4.13): one waits
// GrepSlotWait at most for its slot, then answers 503 server_busy.
type GrepSlots struct{ sem *semaphore.Weighted }

func NewGrepSlots(n int) *GrepSlots

// Take is a slot, released by the func it answers: shared.ServerBusy(1 s)
// after GrepSlotWait, ctx's error when ctx ends first.
func (s *GrepSlots) Take(ctx context.Context) (func(), error)
```

- `page.New` 以 `max(1, runtime.GOMAXPROCS(0)/2)` 建一个（`Deps.GrepSlots` 为 nil 时；测试交自己的，码的测试交一个只有一个名额的并持着它，6.1）。同解析的预算（`platform/markdown/budget.go:47-101`）的做法：`semaphore.Weighted`，等过时答 `shared.ServerBusy`（`shared/error.go:190`，稍后再试 1 秒），日志在 info 级记"grep is saturated"（同解析的预算的那一行）。
- 名额在笔记本的判定之后取（503 在 404 之后，4.15），持到答复建好（2.1 第 17 条）。
- **连接**：每条语句各自从池里取、读完就还；匹配时不持连接，不在事务里（4.13）。一次调用至多 4 + ⌈读的字节 / 4 MiB⌉ 条语句（树、候选、`building`、页的状态，正文每批一条）。
- **内存**：一次调用同时至多一批正文（≤ 5 MiB；`PageStates` 之后被别人写大的页让一批多出它们长的，3.4.3）、一行的 NFC 副本、树（同 `list` 每次读的那一棵，M12 的性能移交）、候选的集合（10 万个约 3 MB）与编译的程序（2,000 条指令约几百 KB；编译时 14–27 MB 的分配，名额里）；名额数乘它们是进程里匹配的上限，解析另在它的队列里（3.6.1）。NFC 副本：NFC 能把一行变长到 3 倍（U+1D160 的 4 字节变成 12 字节，审查 A-m4），所以只规整截断能到的开头（`GrepLineCut` 字节，以 `norm.NFC` 逐段写到它为止，退到段的边界）：2,000 条指令的模式对 5 MiB 的一行只复制约 75 KB；10 条指令的模式的截断是 15 MB，副本至多约 15 MB。

### 3.7 用例 `Grep`

#### 3.7.1 形状与次序

```go
// GrepRequest is a grep's arguments (M9 design 4.13).
type GrepRequest struct {
	Pattern    string
	Glob       *string
	IgnoreCase bool
	Limit      *int
	Cursor     *string
}

// Grepped is a page of grep: its hits and partial lines in path-key and
// line order, each with the cursor after it, so that the adapter can stop
// at the answer's bound; Next, when more may follow, the cursor after the
// page; Building, when the pattern had keys and the notebook holds pages
// queued in bulk, which are not searched; Incomplete, when the walk
// stopped at one of its bounds before it checked every candidate (not at
// limit); and the walk's tally, for the log.
type Grepped struct {
	Found      []GrepFound
	Next       string
	Building   bool
	Incomplete bool
	Walk       GrepWalk
}

// GrepFound is a line grep found: a hit, with its text, or, Hit nil, a
// line it matched only the start of (partial_lines).
type GrepFound struct {
	Path   string
	Line   int
	Hit    *domain.GrepHit
	Cursor string
}

// Grep finds the lines of a notebook's pages' contents a pattern matches
// (M9 design 4.13; M9/P4 design 3.7).
type Grep struct{ d GrepDeps }

// GrepDeps are what grep reads with; the bounds 0 are the domain's, which
// production leaves them.
type GrepDeps struct {
	Notebooks Notebooks
	Nodes     Nodes
	States    PageStates
	Reads     GrepReads
	Auth      shared.Authorizer
	Parses    *GrepParses
	Slots     *GrepSlots
	Logger    *slog.Logger
	Clock     shared.Clock
	MaxWork, MaxLineWork, MaxRead int64
	Walk      int
	GlobWork  int64
}

func NewGrep(d GrepDeps) *Grep

// Execute is the page of grep r in notebookID asks for (M9/P4 design
// 3.7.1): the actor; the cursor, 400; the pattern, 422 (Parses.Parse,
// which waits for its turn and refuses nothing); the glob, 422
// (domain.ParseGlob); the limit, 422; the notebook, notebook.not_found,
// page.read; the slot, 503; then the reads and the match, unlocked,
// without a transaction, no connection held while it matches.
func (g *Grep) Execute(ctx context.Context, notebookID uuid.UUID, r GrepRequest) (Grepped, error)
```

- 次序是 4.15 的通用次序（审查 A-m1）：游标是参数的结构（`bad_request`），在一切取值之前；模式与 glob 是与资源无关的取值；`limit` 1–500（`CheckGrepLimit`，默认 100）；笔记本照 `decideNotebook`（`page/app/authorize.go:28`）判定 `page.read`（读正文，4.15 的"`list`、`read`、`grep` 是 `node.list` 或 `page.read`"）；名额的 503 在一切 404 之后。`list` 把模式与 glob 排在游标之前，是因为它的游标随模式的种类（子节点或 glob）解码成不同的东西（[P1](01-P1-platform-reads.md) 的设计审查 B-M6）；`grep` 只有一种游标，没有这个理由。解析的队列只让调用等，不答码，所以不在这个次序里。没有 403：读者以上都读得到。
- 游标：`domain.GrepCursor{K string, N int}`（`k`、`n`，经 `shared.EncodeCursor`）：`N` ≥ 1 是"从路径的键为 `K` 的页的第 `N` 行起"，`N` = 0 是"在 `K` 之后"；`N` < 0 或解码不了答 400 `bad_request`（`shared.InvalidCursor`）。

#### 3.7.2 读

拿到名额、编译之后，依次（每条语句各自取连接、读完就还）：

1. **树**：`ListNodes`（同 `list`），`treeOf`（`list_by_pattern.go:222-238`）；
2. **候选**：有键时先 `SearchBuilding([notebookID])`，再 `GrepCandidates(notebookID, keys)`（3.4.2）：两条语句不在一个快照里，反过来时一个批量入队的页可能在候选的语句里还没有行、之后任务建好它的行并出了队列，`SearchBuilding` 再看已没有批量的项，答复就既漏了它又说 `building: false`；先读 `building`，其间新进来的批量的项只让答复多说一次 `building`（审查 A-m7；`TestGrep` 以假的存储在两次调用之间清空队列核对这个次序，6.1）。没有键时候选是树里活着的全部页，`building` 为假（批量的页照样在树里、照样被走查）；
3. **走查**（3.7.3）里：被选中的页的 `PageStates`（一条语句，至多 501 个 id），正文按批 `GrepContents`。

候选与树之间、树与正文之间别人写了：树之后新建的页不在这次走查里（下一次调用在）；被删除的页没有状态或正文、跳过；被改名、移动的页按树里的位置走（路径是树里的）。这些是一次调用之内的，`grep` 不要快照（4.13）。

#### 3.7.3 走查

```go
// walk checks the candidates in path-key order from the cursor: the page
// at K from line N first when N ≥ 1, then those after K the glob matches
// (all pages without a glob: **/*.md), at most MaxSearchWalk pages read,
// MaxGrepRead bytes, MaxGrepWork units of work, before the soft deadline,
// to limit hits; the glob's matching counts toward MaxGlobWork apart
// (M9/P1 design 3.10.1).
```

- **glob 在走查之前取交**（4.13，设计审查 C-M5）：`Glob.Walk(ctx, t.roots, K, MaxGlobWork, each)`（`page/domain/glob.go:353-371`）按路径的键的次序访问节点，`each` 只收是页、而且在候选里（没有键时每页都是）的节点，收到 `MaxSearchWalk` + 1 个就停：第 501 个说"还有"。没有 `glob` 时用 `**/*.md`（每页；它的工作随节点数，远在 `MaxGlobWork` 之下）。glob 的工作与 `grep` 的工作分开计（P1 第 8 节）；glob 的工作用完时（`w.Stopped`）走查照常做完已收到的页，游标是 `(w.Last, 0)`。
- **续走**：`N` ≥ 1 时先找树里键为 `K` 的节点（`tree` 加一个按键的查找，只在续走时建），它仍是页、仍在候选里、glob 仍匹配它时从第 `N` 行起查它，再从 `K` 之后走；它不在了（改名、移动、删除）就直接从 `K` 之后走。所以页在两次调用之间被挪动时，它与它的子页（路径的键随它变）可能被跳过或再查一遍，别的页不会（同 `list` 的游标，4.9；审查 A-n2）；它的正文变了时从现在的第 `N` 行起，可能漏掉或重复它的几行。工具的说明写这一句。
- **读正文**：按次序取页，以 `PageStates` 的字节数合成一批（至多 `GrepBatchBytes`；一页大于它时一批只有它）；一批之前：这次调用读过的字节加这一批超过 `MaxGrepRead`、而已经读过至少一页时，停在这一批的第一页之前（游标 `(它的键, 1)`、`incomplete`）：界线在读这一批之前判，不在读了之后。读到的正文的字节照实计（`PageStates` 之后被写大的页），下一批之前照这个数判。
- **一页的行**：`GrepLines`（3.7.4）从第 `N` 行（续走的第一页）或第 1 行起；每行先预先判断（3.6.3），再匹配；命中给 `GrepFound{Hit}`，截断而没有在截断之前的命中给 `GrepFound{Hit: nil}`（`partial_lines`）；每项的游标是 `(这页的键, 行号 + 1)`。
- **停**：
  - 命中到了 `limit`：`Next` 是最后一个命中之后，不是 `incomplete`；
  - 一次的工作、软期限（预先判断）：`Next` 是 `(这页的键, 这一行)`，`incomplete`；
  - `MaxGrepRead`：如上，`incomplete`；
  - 500 页查完而有第 501 个：`Next` 是 `(第 500 页的键, 0)`，`incomplete`；
  - glob 的工作：`(w.Last, 0)`，`incomplete`；
  - 都查完：没有 `Next`。
- 每次调用至少查一行（3.6.3），所以 `Next` 总是往后。

#### 3.7.4 行号与行的文字

```go
// GrepLines yields the lines of content from line from on, numbered as
// read numbers them (M9 design 4.9): parted by "\n", the last "\n" ending
// the last line, an empty content no line; each without its trailing
// "\r", the first without its byte order mark, column the byte of the
// line it starts at (3 after the mark, else 0). It stops when yield does.
func GrepLines(content string, from int, yield func(line int, text string, column int) bool)

// GrepHit is a line's hit as an answer shows it: the line's text, cut to
// MaxGrepLineText bytes centred on its first match when longer (Cut), and
// the byte of the line it starts at, as read's column (M9/P4 design
// 3.7.4).
type GrepHit struct {
	Text   string
	Column int
	Cut    bool
}

// GrepHitOf is the hit of the line text (GrepLines'), at column, whose
// first match is text[start:end].
func GrepHitOf(text string, column, start, end int) GrepHit
```

- 与 `read` 同一种数法（`read` 的切行在 `ReadPath`，`page/app` 里自己的一份）：第 n 行是第 n − 1 个 `\n` 之后的那一段；最后的 `\n` 结束最后一行、不另起空行；空的正文没有行。`\r` 只去掉行尾的一个（`\r\r\n` 留一个 `\r`，同 ripgrep 的 CRLF 模式）；BOM 只在第 1 行的开头。
- 匹配的是 `NFC(text)`；命中的位置由 NFC 的偏移映回原样的字节：已是 NFC 的行偏移不变；否则按 NFC 的段（`norm.NFC.NextBoundaryInString`）建偏移的对照，命中的开头映到它所在的段的开头、结尾映到段的结尾。答复的文字总是原样的字节（4.13：`edit` 拿它去匹配不会因为规整而找不到）。
- 截取：文字长于 1,000 字节时取以命中的中点为中心的 1,000 字节，两端退到字符的边界；`Cut` 为真，`Column` 加上截取的起点。`read(offset=行号, column=Column)` 从这段文字的第一个字节读起（性质测试核对，6.1）。

#### 3.7.5 日志

用例在每次调用之后记一行（info，"grep walked"）：`notebook_id`、`keys`（键的个数）、`candidates`（候选的个数，没有键时 -1）、`pages`、`bytes`、`work`、`lines`、`found`、`stop`（`done`、`limit`、`pages`、`bytes`、`work`、`deadline`、`glob`）、`building`。不记模式、键、路径与行（13.1 第 10 条，4.15）。名额等过时的 503 记 info "grep is saturated"，带等了多久。

### 3.8 `Search.InNotebook` 与 `SearchHit` 的层

```go
// SearchHit is a node a search found, and the tier that found it, which a
// page of hits cut short rebuilds its cursor from (M9 design 4.7).
type SearchHit struct {
	Node       SearchNode
	Ancestors  []domain.Ancestor
	TitleHits  []domain.TextRange
	Excerpt    *domain.SearchExcerpt
	ChangedAt  time.Time
	Attachment *AttachmentInfo
	Tier       int
}

// Cursor is the cursor of the page after h: below its tier, time and id,
// as Execute's next_cursor after the limit-th hit.
func (h SearchHit) Cursor() (string, error)

// SearchDeps gains the notebooks' workspaces: InNotebook's way to the
// workspace a notebook is in.
type SearchDeps struct {
	// ...as today...
	Notebooks Notebooks
}

// InNotebook returns the page of the search r of the notebook notebookID
// alone, as an MCP call asks for it (M9 design 4.13; M9/P4 design 3.8):
// the cursor, 400; q, 422; the limit, 422; then the notebook's workspace
// (Notebooks.WorkspaceOf), page.search there and the notebooks the caller
// sees in it, which must hold notebookID: no such notebook, a deleted
// one, one of a workspace the caller is not a member of and one there it
// does not see are all notebook.not_found, never workspace.not_found;
// then Execute's reads, in one read-only snapshot, its scope notebookID.
// r.NotebookID is not read.
func (s *Search) InNotebook(ctx context.Context, notebookID uuid.UUID, r SearchRequest) (SearchPage, error)
```

- `Execute` 与 `InNotebook` 共用前后两段：`parsed(r)`（游标与 `q`）与 `run(ctx, notebooks, q, below, limit, first)`（`:138-150` 的快照与读）；中间各自的：`Execute` 照旧（工作区按 slug、`limit` 在后），`InNotebook` 的 `limit` 在笔记本之前（2.1 第 16 条）。
- `authorize` 的 `notFound` 传 `domain.ErrNotebookNotFound`（今天的 `Execute` 传 `ErrWorkspaceNotFound`，`:112-113`）；`VisibleIn` 不含它也是这个码。用例的表格测试四行各一（13.1 第 4 条，4.13）。
- 零值的 id（账户入口把不是 UUID 的 `notebook` 当作零值交来）不查库就答 `notebook.not_found`，同 `decideNotebook` 的第一步（`page/app/authorize.go:24-33`；审查 A-n7），码的表的 `search` 一行因此不碰库。
- `read` 给每行的层（`tierRow.tier`，`:181-184`），`page` 把它抄进 `SearchHit.Tier`；`Cursor` 与 `:244` 的写法相同（`SearchCursor{Tier, At: ChangedAt, ID}`，`ChangedAt` 就是 `SortAt`，`:353`）。REST 的答复不变（`Tier` 不进 OpenAPI 的形状）。
- 范围只有这一本：`SearchScope.Notebooks = [notebookID]`，M8 的读法照旧只读它的键（`00041:90-118`）；块数的测试照旧覆盖它（它就是 M8 的语句）。

### 3.9 linking：路径的端口、`backlinks` 与 `index`

#### 3.9.1 `page.NewNodePaths(pool)` 与 linking 的 `Deps.NodePaths`

```go
// NodePaths is what another module reads of a notebook's paths (M9 design
// 4.1, 4.8): linking's backlinks and index, on the pool alone. It decides
// nothing: its callers decided the notebook.
type NodePaths interface {
	// Check is the path's 422 at field, before any read, as page's tools
	// check theirs (domain.ParsePath: the segments' check, M9 design 4.8).
	Check(field, path string) error
	// FindPage is the id of the page at path in notebookID, a point lookup
	// a segment (app.NodePaths.Find): page.not_found in page's tools' words,
	// its hint while the answer holds it, for none, and for an attachment's
	// path, which names no page.
	FindPage(ctx context.Context, notebookID uuid.UUID, field, path string) (uuid.UUID, error)
	// Paths is the paths of ids' nodes alive in notebookID, by id
	// (app.NodePaths.Paths: one statement, PathStore.PathsOf).
	Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error)
	// Pages is Paths of ids' pages, each with when its content last
	// changed (PageStates): two statements.
	Pages(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]PagePath, error)
	// ChildPages is the pages alive of notebookID whose parent is parent,
	// the root's for uuid.UUID{}, in no order.
	ChildPages(ctx context.Context, notebookID, parent uuid.UUID) ([]uuid.UUID, error)
}

// PagePath is a page's path and its content's last change.
type PagePath struct {
	Path      string
	ChangedAt time.Time
}

// NewNodePaths returns NodePaths over pool alone.
func NewNodePaths(pool *pgxpool.Pool) NodePaths
```

- 在 `page/node_paths.go`，照 `NewLinkTargets(pool)` 的写法（`page/link_targets.go:97-100`）：`app.NewNodePaths(postgresadapter.New(pool))` 加 `PageStates`。给路径就是 P2 的 `NodePaths.Paths`（`node_paths.go:56-58`，`PathStore.PathsOf` 经 `LinkTargetsByIDs`，`queries/links.sql:25`），不另写给路径的语句（P2 第 8 节）；只为问到的 id 拼路径（P1 第 8 节，B-M2）。
- `FindPage` 的错误：`app.NodePaths.Find` 的 `*domain.PathNotFound` 经 page 的 `adapter/mcp` 的 `notFound`（`text.go:49-63`）成为 `*mcpserver.ToolError`，文字与 page 的工具相同（"page.not_found: Nothing is at "A/B.md"." 与提示），`Err` 是 page 的 `page.not_found`（mcpserver 照它分类）。路径指着附件时也是 `page.not_found`，文字是 `page.not_found: "A/x.png" is an attachment; a page's path ends with .md.`（4.9 的"附件答 `page.not_found`"）。模块根可以导入自己的适配器；linking 的用例原样返回这个错误，linking 的 `adapter/mcp` 不改写它（3.10.4）。
- 两条新语句，走 `nodes_notebook_id_parent_id_idx`（`parent_id IS NULL` 是索引的条件，`IS NOT DISTINCT FROM` 不是，`nodes.sql:93-95` 的注释）：

```sql
-- name: RootPageIDs :many
-- The root's pages not deleted (M9/P4 design 3.9.1): index's parent "/".
SELECT id FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND kind = 'page' AND deleted_at IS NULL;

-- name: ChildPageIDs :many
-- A page's child pages not deleted (M9/P4 design 3.9.1): index's parent.
SELECT id FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id = sqlc.arg(parent_id) AND kind = 'page' AND deleted_at IS NULL;
```

- linking 的 `app.NodePaths` 是同样的五个方法（类型是 linking 的：`PagePath` 抄一份；审查 B、C 都看过五个是否多了：`Paths` 可以由 `Pages` 代替、多读一条语句，两位都说不值得改，留着）；`linking.Deps.NodePaths` 由组合根接上：`bootstrap/deps.go:204-215` 的 `linkingDeps` 加 `NodePaths: linkingNodePaths{page.NewNodePaths(pool)}`（同 `linkTargets{targets}` 的适配）。整个程序的最后一跳（13.1 第 21 条）：`TestTheMCPLinksReachTheirUseCases`（6.1）对每个方法各有一个交空时失败的调用。

#### 3.9.2 `ListBacklinks.AtPath`

```go
// BacklinksAt is a page of the pages that link to the page at a path:
// each with its path and the cursor after it, so that the adapter can stop
// at the answer's bound; NextCursor after the page, "" after the last.
type BacklinksAt struct {
	Pages      []BacklinkingAt
	NextCursor string
}

// BacklinkingAt is a page that links there, at its path now.
type BacklinkingAt struct {
	Backlinking
	Path   string
	Cursor string
}

// AtPath is Execute for the page at path in notebookID, as an MCP call
// asks for it (M9 design 4.9; M9/P4 design 3.9.2): the cursor, 400; the
// path, 422 (NodePaths.Check); the limit, 422 (shared.PageSize); the
// notebook, backlink.list, notebook.not_found; the page at the path,
// page.not_found, an attachment's too; then Execute's reads, and the
// source pages' paths in one statement, a source deleted since left out.
func (l ListBacklinks) AtPath(ctx context.Context, notebookID uuid.UUID, path string, limit *int, cursor *string) (BacklinksAt, error)
```

- `ListBacklinks` 加字段 `Paths NodePaths`；`Execute` 的后半（`backlinks.go:66-92`：读行、游标、上下文）抽成 `read(ctx, id, after, size)`，两个入口共用。判定用 `Access.notebook`（`access.go:62-64`），不经 `Access.page`（那是按 id 找页的）。
- 来源页的路径：一页的来源至多 100 个（`limit`），一次 `Paths`。来源页在读反链之后进了回收站：没有路径，这一项不列出（2.1 第 18 条），它之后的项照样列出、游标照样越过它。
- 每项的游标是 `shared.EncodeCursor(SourceID)`（同 `Execute` 的 `NextCursor`，`:72-75`）：适配器的 `Fit` 留下 k 项时 `next_cursor` 是第 k 项的。一项至多约 33 KB（页的路径至多 2,562 字节：10 段各 255 字节、9 个 `/` 与 `.md`，全是 `&` 时按 `MemberBytes` 6 倍约 15.4 KB；十个 240 字节的上下文全是 C0 的控制字符时各 7 倍约 16.8 KB，加省略号；审查 B-M8），总放得下。
- `Execute`（REST 的 `listBacklinks`）不变。

#### 3.9.3 `ListIndex` 与 `index.list`

**动作与规则**：`linking/domain/actions.go` 加 `ActionListIndex shared.Action = "index.list"`（读一本笔记本的索引里的页：属性、标签与入链数），`Actions()` 加它；规则表 linking 的一段加一行（`access/domain/rules.go:118-125`）：

```go
		// The index of a notebook's pages over MCP (M9 design 4.9; M9/P4
		// design 3.9.3): whoever reads the notebook.
		"index.list": {Level: LevelNotebook, Notebook: readers()},
```

`bootstrap/actions_test.go` 的 `TestTheRuleTableIsTheModulesActions`（"规则 = 动作"）随之（一个动作一行，13.1 第 3 条）；它没有 REST 的操作，REST 的矩阵不涉及，MCP 的矩阵有 `index` 一行（3.13）。

**常量与游标**（`linking/domain/index.go`）：

```go
const (
	// MaxIndexString is the most bytes of a type, a summary or a tag an
	// index's answer shows, cut at a character's boundary and marked.
	MaxIndexString = 256
	// MaxIndexTags is the most tags of a page an index's answer shows.
	MaxIndexTags = 20
	// MaxInbound is the most pages linking to a page an index counts.
	MaxInbound = 1000
	// MaxIndexType is the most bytes of index's type filter.
	MaxIndexType = 1024
	// MaxIndexKey is the most bytes of a type's key, the order's and the
	// cursor's; IndexTypeChars the characters of a type the key is taken
	// from: a key's byte takes at most 1.5 characters (u U+0308 U+0304 is
	// U+01D6, two bytes), and Go's NFC looks 30 further.
	MaxIndexKey    = 64
	IndexTypeChars = 128
	DefaultIndexLimit, MaxIndexLimit = 50, 100
)

// IndexCursor is the cursor after an index's item: its type's key and
// whether it had a type, its page's id. The pages with a type come first,
// by the key, then by id; those without, by id.
type IndexCursor struct {
	K     string    `json:"k"`
	Typed bool      `json:"t"`
	ID    uuid.UUID `json:"id"`
}

// IndexKey is a type's key: the title key (shared.TitleKey) of its first
// IndexTypeChars characters, cut to MaxIndexKey bytes at a character's
// boundary, so that a cursor is bounded (M9 design 4.9).
func IndexKey(typ string) string

// CutIndexString is s cut to MaxIndexString bytes at a character's
// boundary, and whether it was cut.
func CutIndexString(s string) (string, bool)
```

`IndexKey` 由类型的前 128 个字符算：键的一个字节至多来自 1.5 个字符（合成把三个字符合成两个字节：`u` U+0308 U+0304 是 U+01D6；对全部码位的 NFD 算，最多的是 1.5，审查 B-N2 的 `tokb keychars`），所以 64 字节的键至多来自 96 个字符，Go 的 NFC 再往后看至多 30 个（流安全），128 个够。键截到 64 字节而不是 256（审查 B-M3）：4.9 的 256 是答复的截断，不是次序的需要；游标照样有界，一次调用读的 `type` 也少了四分之三。`TestIndexKey` 以长的类型（U+01D6 一类的三个字符的合成、组合记号、谚文的字母、`ß`）核对"128 个字符的键的前 64 字节等于整个的"。

**用例**：

```go
// IndexQuery is index's arguments (M9 design 4.9).
type IndexQuery struct {
	Type, Tag, Parent *string
	Limit             *int
	Cursor            *string
}

// IndexItem is a page of the index: its path, type, summary and tags, cut
// as Cut names them, when its content last changed, how many other pages
// link to it (to MaxInbound), its parent page's path (nil at the root), and
// the cursor after it.
type IndexItem struct {
	PageID     uuid.UUID
	Path       string
	Type       *string
	Summary    *string
	Tags       []string
	ChangedAt  time.Time
	Inbound    int
	ParentPath *string
	Cut        []string
	Cursor     string
}

// IndexPage is a page of index, and the cursor after it, "" after the last.
type IndexPage struct {
	Items []IndexItem
	Next  string
}

// ListIndex lists the pages of a notebook the link index holds, filtered
// by type, tag and parent page (M9 design 4.9; M9/P4 design 3.9.3).
type ListIndex struct {
	Access Access
	Reads  IndexReads
	Paths  NodePaths
	TagKey func(name string) (string, bool)
}

// Execute is the page of q in notebookID: the cursor, 400; type (NUL,
// longer than MaxIndexType) and parent (NodePaths.Check, "/" the root),
// 422 at their fields; "" refused for type, tag and parent; the limit, 422, 1 to
// MaxIndexLimit, DefaultIndexLimit; the notebook, index.list,
// notebook.not_found; parent's page, page.not_found; then the index's
// pages read whole with the starts of their types (of type alone when it
// is given), filtered, ordered, the page of them after the cursor, and the
// page's types, summaries, tags, inbound counts, paths and last changes, a
// page deleted since left out. A tag loses one leading "#"; one no page can
// have (NUL among them) gives an empty page, as GetTag. A read takes no
// lock and opens no transaction.
func (l ListIndex) Execute(ctx context.Context, notebookID uuid.UUID, q IndexQuery) (IndexPage, error)
```

- 筛选：`type` 是 jsonb 的值相等，在语句里筛（`IndexPages` 的 `type_is`：只答这种类型的页，审查 B-M3）；`tag` 先去掉开头的一个 `#`（正文里的写法；frontmatter 里写成 `#plan` 的标签存作 `plan`，`linking/adapter/markdown/facts.go:61-63`；`getTag` 的路径段不带它），再经 `TagKey`（`linking/adapter/markdown/tag.go:19`），`false` 时（带 NUL 的也是）是空的一页、不查库（同 `GetTag`，`tags.go:51-54`），否则 `TagPages`（`reads.sql:96-105`，含其下的标签）；`parent` 经 `ChildPages`。三者取交，在 Go 里。三个都不收空的字符串：422，文字说"omit it"（`parent` 的另说"or use "/""）；模型常以 `""` 代替"没有"，照值筛会悄悄答空（审查 B-M7）。
- 次序：有类型的在前，按（`IndexKey`、id），没有类型的在后、按 id；游标之后取 `limit` + 1 个；`limit` 个里最后读的那一页被留下（它在读了之后进了回收站、没有路径）时 `Next` 照样在它之后（同 3.9.2，审查 B-N3）。读全部的页是为了次序：键要 Go 的标题键，SQL 给不出；它与 `list` 读整棵树是同一类代价。最坏的一次调用：10 万页、每个 `type` 是 128 个 4 字节的字符，`IndexPages` 答约 51 MB 的 `type` 的开头，键约 6.4 MB，合计约 65 MB 活着的内存，`TitleKey` 由几百毫秒（ASCII）到约 0.7 秒（一个字母与一串记号；审查 B-M3 量 512 个字符的 2.65 秒，按字符数折算；审查 B 对修订的核对量 128 个字符的 10 万个约 0.24–0.71 秒）。第 10 节与 M12 的性能移交写这一行（第 7 节）；守卫取这个形状（6.1）。
- 每项：`Pages` 给路径与正文的最后改动（没有的页不列出）；父页的路径由路径去掉最后一段、加 `.md` 得到（根下的为 null）；`type` 与 `summary` 由 `CutIndexString` 截、标出（`cut` 里的 `type`、`summary`）；标签至多 20 个（按标签键），每个截到 256 字节（`cut` 里的 `tags[i]`），多于 20 个时 `cut` 里有 `tags`；入链数由 `InboundCounts`。
- 索引里没有行的页（M6 之前写的、`nervewiki reindex` 之前的）不在其中；提取器旧了的照旧行在（`reads.sql:84-105` 的写法）：工具的说明写这一句。

**端口与语句**（`linking/app/ports.go` 的 `IndexReads`，`linking/adapter/postgres/queries/index.sql`）：

```go
// IndexReads are what index reads of the link index (M9/P4 design 3.9.3).
type IndexReads interface {
	// IndexPages is the pages of notebookID the index holds, each with
	// the start of its type (IndexTypeChars characters); with typeIs, only
	// those whose type is that string.
	IndexPages(ctx context.Context, notebookID uuid.UUID, typeIs *string) ([]IndexedPage, error)
	// IndexItems is the types, summaries and first tags of the pages ids.
	IndexItems(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]IndexFacts, error)
	// InboundCounts is how many other pages link to each of ids, at most
	// max; a page none links to is not among them.
	InboundCounts(ctx context.Context, ids []uuid.UUID, max int) (map[uuid.UUID]int, error)
	// TagPages is as GetTag reads it.
	TagPages(ctx context.Context, notebookID uuid.UUID, key string) ([]uuid.UUID, error)
}
```

```sql
-- name: IndexPages :many
-- The pages of a notebook the index holds (M9/P4 design 3.9.3), each with the start of its type: the first of its
-- properties whose key is "type" but for ASCII case (translate leaves any other key unequal), when that property's
-- value is a string, its first 128 characters, which the order's key of 64 bytes needs. With type_is, only the pages
-- whose type is that string. Each page's properties are read by page_properties_type_idx (00047), one probe a page
-- whatever properties it has: never the notebook's others, nor another notebook's. The pages are read whole: the order
-- is Go's title keys.
SELECT ip.node_id, t.type_start
FROM indexed_pages ip
LEFT JOIN LATERAL (
    SELECT CASE WHEN jsonb_typeof(p.value) = 'string' THEN left(p.value #>> '{}', 128) END AS type_start, p.value
    FROM page_properties p
    WHERE p.source_id = ip.node_id AND translate(p.key, 'TYPE', 'type') = 'type'
    ORDER BY p.position
    LIMIT 1
) t ON true
WHERE ip.notebook_id = sqlc.arg(notebook_id)
    AND (sqlc.narg(type_is)::text IS NULL OR t.value = to_jsonb(sqlc.narg(type_is)::text));

-- name: IndexItems :many
-- The types, summaries and tags of the index's pages ids (M9/P4 design 3.9.3): the first property whose key is "type",
-- and the first whose key is "summary", but for ASCII case, each's first 512 characters when its value is a string, by
-- their partial indexes (00047); and the first 21 tags by key, as most written, each's first 512 characters, the 21st
-- telling that more follow, by the primary key.
SELECT ip.node_id,
    ty.type_text,
    s.summary_start,
    ARRAY(
        SELECT left(t.tag, 512) FROM page_tags t WHERE t.source_id = ip.node_id ORDER BY t.tag_key LIMIT 21
    )::text[] AS tags
FROM indexed_pages ip
LEFT JOIN LATERAL (
    SELECT CASE WHEN jsonb_typeof(p.value) = 'string' THEN left(p.value #>> '{}', 512) END AS type_text
    FROM page_properties p
    WHERE p.source_id = ip.node_id AND translate(p.key, 'TYPE', 'type') = 'type'
    ORDER BY p.position
    LIMIT 1
) ty ON true
LEFT JOIN LATERAL (
    SELECT CASE WHEN jsonb_typeof(p.value) = 'string' THEN left(p.value #>> '{}', 512) END AS summary_start
    FROM page_properties p
    WHERE p.source_id = ip.node_id AND translate(p.key, 'SUMARY', 'sumary') = 'summary'
    ORDER BY p.position
    LIMIT 1
) s ON true
WHERE ip.node_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: InboundCounts :many
-- How many pages but itself link to each of the pages ids, at most max_count (M9/P4 design 3.9.3): the sources found
-- a step each, the next after the last on page_links_resolved_id_source_id_idx, as Backlinks finds them, never all of
-- a page's links: a page may write a million (M6/P5 review r1-1). A page no other links to has no row.
WITH RECURSIVE steps (target, source_id, n) AS (
    SELECT t.id, x.source_id, CASE WHEN x.source_id = t.id THEN 0 ELSE 1 END
    FROM unnest(sqlc.arg(ids)::uuid[]) t (id)
    CROSS JOIN LATERAL (
        SELECT l.source_id FROM page_links l
        WHERE l.resolved_id = t.id
        ORDER BY l.source_id
        LIMIT 1
    ) x
    UNION ALL
    SELECT s.target, x.source_id, s.n + CASE WHEN x.source_id = s.target THEN 0 ELSE 1 END
    FROM steps s
    CROSS JOIN LATERAL (
        SELECT l.source_id FROM page_links l
        WHERE l.resolved_id = s.target AND l.source_id > s.source_id
        ORDER BY l.source_id
        LIMIT 1
    ) x
    WHERE s.n < sqlc.arg(max_count)::integer
)
SELECT target::uuid AS page_id, max(n)::integer AS inbound FROM steps GROUP BY target;
```

- **迁移 00047**（S2，无条件；审查 B-I2）：

```sql
-- page_properties' type and summary, read by index a probe a page (M9/P4 design 3.9.3): a page's first property whose
-- key is "type" (or "summary") but for ASCII case, whatever other properties it has. Without them index's lateral reads
-- every property of a page without a type, up to the frontmatter's 10,000 nodes. A key that matches but for ASCII case
-- has 16 (or 128) spellings, so each index holds a few rows a page at most.

-- +goose Up
CREATE INDEX page_properties_type_idx ON page_properties (source_id, position)
    WHERE translate(key, 'TYPE', 'type') = 'type';
CREATE INDEX page_properties_summary_idx ON page_properties (source_id, position)
    WHERE translate(key, 'SUMARY', 'sumary') = 'summary';

-- +goose Down
DROP INDEX page_properties_summary_idx;
DROP INDEX page_properties_type_idx;
```

  一页的属性不论多少都只探查一次：没有它时，一页没有 `type` 就读它的全部属性（frontmatter 至多约 5,000 个顶层的属性，`platform/markdown/yaml.go:17-20` 的 1 万个节点），一次 `index` 读整本笔记本的这些，不论 `limit`、`tag` 与 `parent`：审查 B 在 PG 18.6 上 2,000 页各 1,000 个属性、都没有 `type` 时 `IndexPages` 580 ms（每页 0.29 ms，10 万页这样的页约 29 秒，过了请求的 15 秒），加了它 0.9 ms，语句不用改（lateral 的条件蕴含索引的条件）、五个属性的笔记本上也用它（0.74 ms）。建它要扫一遍 `page_properties`，在 goose 的事务里、只在启动时；M8 的语料没有 frontmatter，几乎没有行。
- "ASCII 不区分大小写的第一个"：`translate(key, 'TYPE', 'type') = 'type'` 当且仅当 `key` 只差 ASCII 的大小写（`translate` 逐字符换，与区域设置无关；`lower()` 随排序规则），同 linking 的提取的 `asciiFold`（`linking/adapter/markdown/facts.go`）；表格测试对 `Type`、`TYPE`、`tYpE`、`types`、`ｔｙｐｅ`、`Type` 两次（第一个是数组）各一行。值不是字符串时就没有，不再往后找（4.9 的"值是字符串时才有"）。
- 入链数：一个目标至多 1,001 次下降（含它自己），一页 100 项至多约 10 万次；数的是来源页（不是链接，4.9），与 `Backlinks` 一样不数自己。
- **计划测试**（`index_plan_test.go`，13.4 第 9 条）：两本笔记本：`A` 1,000 页各 5 个属性（有 `type` 与 `summary`）、3 个标签、链接若干，另 200 页各 1,000 个属性而没有 `type` 与 `summary`；别的一本 50,000 页各 5 个属性。无统计、`ANALYZE` 之后与通用计划三种状态：`IndexPages` 对 `A` 的外层读 `indexed_pages_notebook_id_idx`（索引或位图的扫描都行；只断言小的 `A`：别的那本占了大半的行时顺序扫描是对的计划，13.1 第 4 条不管大小与统计，审查 B-M4）；lateral 用的索引是 `page_properties_type_idx`（按索引的名字断言，不按节点的种类：分析之后是 `Limit → Sort → Bitmap Heap Scan` 的形状，审查 B-M4）；`IndexItems` 的两个 lateral 是两个部分索引，标签是 `page_tags_pkey`，外层是 `indexed_pages_pkey`；`InboundCounts` 是 `page_links_resolved_id_source_id_idx` 的只读索引扫描。没有 00047 时 lateral 的索引是 `page_properties_pkey`，测试失败（第 184 条变异）。结果写进 9.2；时间在守卫里量（6.1）。

### 3.10 四个工具的输入、答复与语义

四个都是只读的（`readOnlyHint: true`、`idempotentHint: false`、`openWorldHint: false`，同 P1–P3 的读），两个入口都有（`mcpserver.BothEntries`，`Notebook: true`：账户入口另带必填的 `notebook`）。答复是结构的（`structuredContent` 与它的 JSON 文字，4.7），经 `mcpserver.Fit` 停在 48 KiB，逐项的游标照 `log` 的写法（`page/adapter/mcp/log.go:31-62`）：留下 k 项而 k 小于项数时 `next_cursor` 是第 k 项之后的，否则是用例的。

#### 3.10.1 `grep`（page 的 `adapter/mcp/grep.go`）

```go
// grepIn is grep's arguments (M9 design 4.13; M9/P4 design 3.10.1).
type grepIn struct {
	Pattern    string  `json:"pattern" jsonschema:"The Go regular expression (RE2), at most 1024 bytes, matched against each line alone."`
	Glob       *string `json:"glob,omitempty" jsonschema:"Only the pages whose paths match this glob, as list's: \"Projects/**\", \"**/*.md\". Default: every page."`
	IgnoreCase *bool   `json:"ignore_case,omitempty" jsonschema:"Match without case, as (?i). Default false."`
	Limit      *int    `json:"limit,omitempty" jsonschema:"At most this many lines with a hit, 1 to 500. Default 100."`
	Cursor     *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// grepAnswer is grep's answer: the hits and the partial lines, each in
// path and line order; next_cursor null when nothing follows.
type grepAnswer struct {
	Hits         []grepHit  `json:"hits"`
	PartialLines []grepLine `json:"partial_lines"`
	NextCursor   *string    `json:"next_cursor"`
	Building     bool       `json:"building"`
	Incomplete   bool       `json:"incomplete"`
}

type grepHit struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Column *int   `json:"column,omitempty"` // when the text does not start at the line's first byte
	Cut    bool   `json:"cut,omitempty"`
}

type grepLine struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}
```

- `Fit` 数的是 `Found` 的项（命中与部分匹配的行合在一个次序里），留下 k 项时两个列表各取前 k 项里自己的；`incomplete` 只在 k 等于项数时照用例的（被 48 KiB 截短的答复不是 `incomplete`，2.1 第 7 条）。一项总放得下，但余地不大（审查 A-n6，`review-A/misc` 以 mcpserver 的编码与 `TextBytes` 量）：深 10 层、11 段各 255 字节的 `&` 的路径（2,818 字节，P1 的路径的上限；页的路径至多 2,562 字节）、1,000 字节的 C0 的控制字符的文字与它的 `next_cursor`（`EncodeCursor` 把 `&` 写成 `\u0026` 再 base64，22,499 字节）合计 49,152 里的 46,514，剩 2,638 字节，游标占一半。适配器的 `grep_test.go` 造这一项、断言它放得下：以后改了游标或项的成员而过了界线时（`Fit` 答 `errNothingFits`、工具答 `internal_error`）失败。
- 错误：用例的 422（`pattern`、`glob`、`limit`）、400（游标）、404（笔记本）、503（名额）照 mcpserver 的分类与文字；`toolErr` 不认识它们，原样（`text.go:22-47`）。`pattern` 的 422 的消息从不复述模式（3.6.1）。
- 例：`{"hits":[{"path":"Notes/Alpha.md","line":3,"text":"TODO: alpha"}],"partial_lines":[],"next_cursor":null,"building":false,"incomplete":false}`。

#### 3.10.2 `search`（page 的 `adapter/mcp/search.go`）

```go
// searchIn is search's arguments (M9 design 4.13).
type searchIn struct {
	Q      string  `json:"q" jsonschema:"The words to find, parted by spaces: at most 256 bytes and 8 words."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many results, 1 to 50. Default 20."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

// searchAnswer is search's answer: the results, best first; the first
// page's count, null on the others; next_cursor null when nothing follows.
type searchAnswer struct {
	Results    []searchResult `json:"results"`
	Count      *searchCount   `json:"count"`
	NextCursor *string        `json:"next_cursor"`
	Building   bool           `json:"building"`
}

type searchResult struct {
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`    // page or attachment
	Excerpt   *string   `json:"excerpt"` // plain text, "…" where it is cut; null without a hit in the content
	ChangedAt time.Time `json:"changed_at"`
}

type searchCount struct {
	Value int    `json:"value"`
	Kind  string `json:"kind"` // exact, about or at_least
}
```

- 路径：`domain.PathOfNames(祖先的名称与节点的名称, 类型)`（`page/domain/path.go:111`），同 `list` 与 `changeset` 的路径。摘要是 `SearchExcerpt.Text`，`CutStart`、`CutEnd` 时在那一端加 `…`；不带网页用的命中范围（4.13）。
- `count` 只在第一页（`SearchPage.Count`），照 M8 的三种。`Fit` 留下 k 项而 k 小于项数时 `next_cursor` 是 `Hits[k-1].Cursor()`（3.8）；一项至多路径加约 160 个字符的摘要，总放得下。
- 错误：`q` 的 422 照 `ParseSearchQuery` 的（`required`、`invalid_format`、`too_long`、`not_allowed`，`search.go:420-430`），NUL 是它的 `invalid_format`。

#### 3.10.3 `backlinks`（linking 的 `adapter/mcp/backlinks.go`）

```go
type backlinksIn struct {
	Path   string  `json:"path" jsonschema:"The page's path from the notebook's root, ending with .md."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many pages, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

type backlinksAnswer struct {
	Pages      []backlinkPage `json:"pages"`
	NextCursor *string        `json:"next_cursor"`
}

type backlinkPage struct {
	Path     string   `json:"path"`
	Links    int      `json:"links"`    // to linking's MaxCount, 1000
	Contexts []string `json:"contexts"` // the lines of its first MaxContexts links, each MaxContext bytes at most
}
```

来源页的路径同 `changeset` 的 `node.path`（P2 第 8 节），由 `NodePaths.Paths` 给（3.9.2）。`path` 的 422 照段的检查；找不到与附件的 `page.not_found` 是 page 的文字（3.9.1），文字只引这次调用自己的路径，原样引起（P2 3.10.3）。

#### 3.10.4 `index`（linking 的 `adapter/mcp/index.go`）

```go
type indexIn struct {
	Type   *string `json:"type,omitempty" jsonschema:"Only the pages whose frontmatter's type is exactly this."`
	Tag    *string `json:"tag,omitempty" jsonschema:"Only the pages with this tag or one under it, with or without its leading #."`
	Parent *string `json:"parent,omitempty" jsonschema:"Only the child pages of this page, by its path ending with .md; \"/\" for the root's."`
	Limit  *int    `json:"limit,omitempty" jsonschema:"At most this many pages, 1 to 100. Default 50."`
	Cursor *string `json:"cursor,omitempty" jsonschema:"next_cursor of the previous answer, to go on from there."`
}

type indexAnswer struct {
	Pages      []indexPage `json:"pages"`
	NextCursor *string     `json:"next_cursor"`
}

type indexPage struct {
	Path         string    `json:"path"`
	Type         *string   `json:"type"`
	Summary      *string   `json:"summary"`
	Tags         []string  `json:"tags"`
	ChangedAt    time.Time `json:"changed_at"`
	InboundLinks int       `json:"inbound_links"` // to MaxInbound, 1000
	Parent       *string   `json:"parent"`        // the parent page's path, null at the root
	Cut          []string  `json:"cut"`           // "type", "summary", "tags[i]", and "tags" for more than 20
}
```

一项至多（页的路径至多 2,562 字节、父页的 2,306，审查 B-M8）：路径与父页的路径全是 `&`（`MemberBytes` 6 倍）约 15.4 KB 与 13.8 KB，`type`、`summary` 各 256 字节的 C0 的控制字符（7 倍）各约 1.8 KB，合计约 33 KB，放得下；放不下的是标签：frontmatter 的标签可以带 C0 的控制字符（YAML 的 `"\x01"` 一类，`CountedTag` 只拒空白），20 个 256 字节的约 35.8 KB，这一项约 68.6 KB。照 `log` 的退回（`log.go:46-61`）：连第一项都放不下时，第一项去掉标签（`cut` 加 `tags`）再试，那时总放得下；`next_cursor` 是这一项之后的，每次调用至少前进一项；再去掉 `summary` 的一步到不了，不要（审查 B-M8）。适配器的测试以 20 个 256 字节的 C0 的控制字符的标签造这样一项（6.1）。

#### 3.10.5 `tools.json` 的条目与 `Tools()`

- page 的 `Tools`（`tools.go:101-129`）加两个 `mcpserver.NewTool`：`grep`、`search`，`Annotations{ReadOnly: true}`、`BothEntries`、`Notebook: true`；`UseCases` 加 `Grep`、`Search`（接口 `GrepUseCase`、`SearchUseCase`，`page.New` 把建一次的实例同时交给两个适配器）。
- linking 的新包 `linking/adapter/mcp`（`mcpadapter`）：`Tools(uc UseCases) []mcpserver.Tool`，`backlinks`、`index`；模块根 `(*Module).Tools()`（同 page、notebook）；`bootstrap/mcp.go` 的 `mcpTools` 拼成 `slices.Concat(pg.Tools(…), ln.Tools(), nb.Tools())`，注释改掉"linking's come with P4"。

### 3.11 `tools.json` 的增加

照 P2、P3：每个数组按名称排，四个工具插在各自的位置（笔记本入口：`backlinks` 在 `batch` 之前、`grep` 在 `edit` 之后、`index` 在 `grep` 之后、`search` 在 `read` 之后；账户入口同样，`notebooks`、`open_notebook` 照旧在 `move` 与 `read` 之间）。账户入口的条目多一个最先的 `notebook`（"The notebook's id, from notebooks."）且它必填。S1 加 `grep`、`search` 的四个条目，S2 加 `backlinks`、`index` 的四个。文件照仓库的 oxfmt 排版（下面是它的内容，排版由 `make lint-web` 定）。

笔记本入口（`"notebook"` 的数组）的四个对象：

```json
{
  "name": "grep",
  "title": "Find lines by a regular expression",
  "description": "Find the lines of the notebook's pages that a Go regular expression (RE2 syntax, as ripgrep's default) matches, page by page in path order. Each hit has the page's path, the line's number as read counts lines (read with that offset and limit 1 gives the line) and the line's text as stored, without its \\r and, on the first line, its byte order mark; a line over 1000 bytes is cut around its first match, cut true; column, when the text does not start at the line's first byte, is the byte it starts at, as read's column. Contents include their frontmatter; titles are not searched (list's glob finds them), nor attachments. glob keeps the pages whose paths it matches, as list's; ignore_case matches without case, as (?i). The pattern is at most 1024 bytes, without a newline, each line matched alone; one too complex, as large repeats, is refused. A call checks a bounded amount: go on with next_cursor while it is set. incomplete says the call stopped at its bounds before it checked every page it had to, so no hit is then no proof of none. partial_lines are lines too long to match whole within a call: read them. building says pages queued in bulk (by a batch, an import, a background restore or revert, an upgrade) are not searched until the index catches up. A page moved or written between calls, with its subpages, may be skipped or checked again. For words and Chinese text, search ranks pages and gives excerpts.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "pattern": {
        "type": "string",
        "description": "The Go regular expression (RE2), at most 1024 bytes, matched against each line alone."
      },
      "glob": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the pages whose paths match this glob, as list's: \"Projects/**\", \"**/*.md\". Default: every page."
      },
      "ignore_case": {
        "type": [
          "null",
          "boolean"
        ],
        "description": "Match without case, as (?i). Default false."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many lines with a hit, 1 to 500. Default 100."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "required": [
      "pattern"
    ],
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
{
  "name": "search",
  "title": "Search the notebook's words",
  "description": "Full-text search of the notebook for words: the pages and attachments whose titles hold every term come first, then the pages whose contents do, each newest first, a page with an excerpt around its first hit. Terms are parted by spaces and match without case; Chinese text matches by pairs of characters, so a term of one Chinese character finds titles only. q is at most 256 bytes and 8 terms. The first page has count: exact, about or at_least. A page may hold fewer results than limit, even none, and still a next_cursor: a call checks a bounded number of contents; go on while next_cursor is set. building says pages queued in bulk (by a batch, an import, a background restore or revert, an upgrade) are not searched until the index catches up. For patterns, exact text and line numbers, use grep.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "q": {
        "type": "string",
        "description": "The words to find, parted by spaces: at most 256 bytes and 8 words."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many results, 1 to 50. Default 20."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "required": [
      "q"
    ],
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
{
  "name": "backlinks",
  "title": "List the pages linking to a page",
  "description": "List the pages that link to a page: each with its path, how many of its links lead there (1000 means 1000 or more) and the lines around the first ten of them, each at most 240 bytes. Only a page has backlinks: an attachment's path is not found. A page's links to itself are left out; a page written since the link index read it has no lines yet. Go on with next_cursor while it is set.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "The page's path from the notebook's root, ending with .md."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many pages, 1 to 100. Default 50."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "required": [
      "path"
    ],
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
{
  "name": "index",
  "title": "List the notebook's pages by type, tag or parent",
  "description": "List the notebook's pages as its link index holds them: each with its path; its frontmatter's type and summary, the first key of that name whatever its case, when its value is text; its tags, 20 at most; when its content last changed; how many other pages link to it (1000 means 1000 or more); and its parent page's path. type, summary and each tag are cut to 256 bytes, cut naming what was cut. type keeps the pages of exactly that type; tag those with that tag or one under it (\"plan\" takes \"plan/q4\"; a leading # is dropped); parent the child pages of that page, \"/\" those at the root. Omit a filter rather than give it empty: \"\" is refused. The pages with a type come first, by type, then the others. A page the index has not read yet, as one written before nervewiki reindex after an upgrade, is not listed. Go on with next_cursor while it is set.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "type": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the pages whose frontmatter's type is exactly this."
      },
      "tag": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the pages with this tag or one under it, with or without its leading #."
      },
      "parent": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the child pages of this page, by its path ending with .md; \"/\" for the root's."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many pages, 1 to 100. Default 50."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
```

账户入口（`"account"` 的数组）的四个对象：

```json
{
  "name": "grep",
  "title": "Find lines by a regular expression",
  "description": "Find the lines of the notebook's pages that a Go regular expression (RE2 syntax, as ripgrep's default) matches, page by page in path order. Each hit has the page's path, the line's number as read counts lines (read with that offset and limit 1 gives the line) and the line's text as stored, without its \\r and, on the first line, its byte order mark; a line over 1000 bytes is cut around its first match, cut true; column, when the text does not start at the line's first byte, is the byte it starts at, as read's column. Contents include their frontmatter; titles are not searched (list's glob finds them), nor attachments. glob keeps the pages whose paths it matches, as list's; ignore_case matches without case, as (?i). The pattern is at most 1024 bytes, without a newline, each line matched alone; one too complex, as large repeats, is refused. A call checks a bounded amount: go on with next_cursor while it is set. incomplete says the call stopped at its bounds before it checked every page it had to, so no hit is then no proof of none. partial_lines are lines too long to match whole within a call: read them. building says pages queued in bulk (by a batch, an import, a background restore or revert, an upgrade) are not searched until the index catches up. A page moved or written between calls, with its subpages, may be skipped or checked again. For words and Chinese text, search ranks pages and gives excerpts.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "notebook": {
        "type": "string",
        "description": "The notebook's id, from notebooks."
      },
      "pattern": {
        "type": "string",
        "description": "The Go regular expression (RE2), at most 1024 bytes, matched against each line alone."
      },
      "glob": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the pages whose paths match this glob, as list's: \"Projects/**\", \"**/*.md\". Default: every page."
      },
      "ignore_case": {
        "type": [
          "null",
          "boolean"
        ],
        "description": "Match without case, as (?i). Default false."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many lines with a hit, 1 to 500. Default 100."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "required": [
      "notebook",
      "pattern"
    ],
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
{
  "name": "search",
  "title": "Search the notebook's words",
  "description": "Full-text search of the notebook for words: the pages and attachments whose titles hold every term come first, then the pages whose contents do, each newest first, a page with an excerpt around its first hit. Terms are parted by spaces and match without case; Chinese text matches by pairs of characters, so a term of one Chinese character finds titles only. q is at most 256 bytes and 8 terms. The first page has count: exact, about or at_least. A page may hold fewer results than limit, even none, and still a next_cursor: a call checks a bounded number of contents; go on while next_cursor is set. building says pages queued in bulk (by a batch, an import, a background restore or revert, an upgrade) are not searched until the index catches up. For patterns, exact text and line numbers, use grep.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "notebook": {
        "type": "string",
        "description": "The notebook's id, from notebooks."
      },
      "q": {
        "type": "string",
        "description": "The words to find, parted by spaces: at most 256 bytes and 8 words."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many results, 1 to 50. Default 20."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "required": [
      "notebook",
      "q"
    ],
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
{
  "name": "backlinks",
  "title": "List the pages linking to a page",
  "description": "List the pages that link to a page: each with its path, how many of its links lead there (1000 means 1000 or more) and the lines around the first ten of them, each at most 240 bytes. Only a page has backlinks: an attachment's path is not found. A page's links to itself are left out; a page written since the link index read it has no lines yet. Go on with next_cursor while it is set.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "notebook": {
        "type": "string",
        "description": "The notebook's id, from notebooks."
      },
      "path": {
        "type": "string",
        "description": "The page's path from the notebook's root, ending with .md."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many pages, 1 to 100. Default 50."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "required": [
      "notebook",
      "path"
    ],
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
{
  "name": "index",
  "title": "List the notebook's pages by type, tag or parent",
  "description": "List the notebook's pages as its link index holds them: each with its path; its frontmatter's type and summary, the first key of that name whatever its case, when its value is text; its tags, 20 at most; when its content last changed; how many other pages link to it (1000 means 1000 or more); and its parent page's path. type, summary and each tag are cut to 256 bytes, cut naming what was cut. type keeps the pages of exactly that type; tag those with that tag or one under it (\"plan\" takes \"plan/q4\"; a leading # is dropped); parent the child pages of that page, \"/\" those at the root. Omit a filter rather than give it empty: \"\" is refused. The pages with a type come first, by type, then the others. A page the index has not read yet, as one written before nervewiki reindex after an upgrade, is not listed. Go on with next_cursor while it is set.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "notebook": {
        "type": "string",
        "description": "The notebook's id, from notebooks."
      },
      "type": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the pages whose frontmatter's type is exactly this."
      },
      "tag": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the pages with this tag or one under it, with or without its leading #."
      },
      "parent": {
        "type": [
          "null",
          "string"
        ],
        "description": "Only the child pages of this page, by its path ending with .md; \"/\" for the root's."
      },
      "limit": {
        "type": [
          "null",
          "integer"
        ],
        "description": "At most this many pages, 1 to 100. Default 50."
      },
      "cursor": {
        "type": [
          "null",
          "string"
        ],
        "description": "next_cursor of the previous answer, to go on from there."
      }
    },
    "required": [
      "notebook"
    ],
    "additionalProperties": false
  },
  "annotations": {
    "idempotentHint": false,
    "openWorldHint": false,
    "readOnlyHint": true
  }
}
```

`bootstrap/mcp_tools_test.go` 照旧逐个比较整个工具对象；说明里的 `\r` 是两个字符（反斜杠与 `r`），不是回车。

审查之后改的（审查 C-N4、A-n2、B-M7，`gen/tools5.py`）：`grep` 的文字写出第一行的 BOM 也不在文字里、`column` 在文字不从这一行的第一个字节起时都有（截取的与带 BOM 的第一行）；`building` 在 `grep` 与 `search` 里写成"批量入队的（batch、导入、后台的恢复与撤销、升级）在索引追上之前不被搜索"（导入的积压以每秒 190–300 页建，要几分钟，不是"几秒"）；挪动的页"与它的子页"；`index` 的 `tag` 去掉开头的 `#`、三个筛选不收 `""`。三种截断都标出（3.3.4），`grep` 的说明不写漏找：被截掉的页总被走查，走不完的答 `incomplete`。

### 3.12 码的表

`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:79-99`）加四行（S1 两行、S2 两行）：

```go
		"grep":      {"bad_request", "validation_failed", "notebook.not_found", "server_busy"},
		"search":    {"bad_request", "validation_failed", "notebook.not_found"},
		"backlinks": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
		"index":     {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
```

`TestTheMCPToolsErrorsCarryTheirCodes`（`mcp_reads_test.go:403-450`）答出每行的每个码：`bad_request` 是坏的游标（四个都有）；`validation_failed` 是 `grep` 的 `limit: 0`、`search` 的 `q: ""`、`backlinks` 的 `path: "a//b.md"`、`index` 的 `parent: ""`；`notebook.not_found` 是账户入口的不是 UUID 的 `notebook`；`page.not_found` 是 `backlinks` 的不存在的路径、`index` 的不存在的父页；`server_busy` 是 `grep` 在测试的应用里只有一个名额、测试持着它时（`page.Deps.GrepSlots` 交 `page.NewGrepSlots(1)`，测试经它 `Take`；等过 1 秒答 503，带 `retry_after_seconds`）。`TestEveryMCPToolHasItsCodes` 要每个工具有一行。

`validToolArguments`（`mcp_reads_test.go:672-687`）加：`"grep": {"pattern": "x"}`、`"search": {"q": "x"}`（S1），`"backlinks": {"path": "Strings.md"}`、`"index": {}`（S2）。NUL 的检查（`:715-760`）由此覆盖每个字符串参数：`grep` 的 `pattern` 带 NUL 照收（正文里没有 NUL，什么也找不到：键里从不含 NUL，NUL 属于"别的"，3.5.3；这个调用不碰 PostgreSQL 的 `text`），`glob` 带 NUL 是 422；`search` 的 `q` 是 422；`backlinks` 的 `path` 是 422；`index` 的 `type` 与 `parent` 是 422，`tag` 是空的一页（`TagKey` 答 `false`，不查库）。都不是 `internal_error`、都到了用例。

### 3.13 矩阵

`pageMCPRows()`（S1）与新的 `linkingMCPRows()`（S2，`TestThePermissionMatrixCoversEveryTool` 要每个工具一行）各加两行，都是 `want: readers`（读者以上读得到，看不到的列答 404：笔记本入口在 HTTP，账户入口是工具的 `notebook.not_found`），两个入口：

| 工具 | `args` | `check`（成功的格） |
|---|---|---|
| `grep` | `{"pattern": matrixMark}` | `hits` 恰好是这一列的笔记本里活着的种下的页，各在 `matrixContent` 放标记的那一行（第 1 行；任务的页 `tasksIn` 是第 3 行，`permission_matrix_seeded_test.go:185-200`），文字是 `quillwort`；`next_cursor` 为 null，`incomplete` 为假 |
| `search` | `{"q": matrixMark}` | `results` 恰好是 `markedNodes(s, notebookOf(c))`（页与名称带标记的附件 `quillwort-<nb>.png`，`permission_matrix_asset_test.go:37-42`、`permission_matrix_page_test.go:782-797`），`count` 是 `{len, "exact"}` |
| `backlinks` | `{"path": pagePathOf(c)}` | `pages` 是空的数组：种下的页没有链接，REST 的 `listBacklinks` 同样是空的；这一格只核对码，内容是 `TestTheMCPBacklinksAreRESTs` 的（审查 B-M9） |
| `index` | `{}` | 列出的路径恰好是这一列的笔记本里活着的种下的页（由矩阵自己种下的清单得出：`seededPageHistory` 给每个没有删除的种下的页插一行 `indexed_pages`，`permission_matrix_seeded_test.go:553-563`，不经 REST 的写），每项的 `inbound_links` 为 0（没有链接）；入链数与属性的内容是 `TestTheIndexIsEachPagesReads` 的（审查 B-M9） |

种下的节点不要搜索的任务：`prepareMatrix` 把每个种下的节点准确地入队（`queueSeeded`，`permission_matrix_seeded_test.go:721`），准确入队的节点是 `grep` 的候选，`search` 经队列找到它们，同 REST 的搜索的行（"through the queue … the reads' app runs no job"；审查 C-M3）。四行都不写，不要库的副本。矩阵多 4 × 13 × 2 = 104 格。

### 3.14 说明的五行

`pathsAndTools` 的"Tools"一段，在 `read` 一行之后（`page/adapter/mcp/instructions.go`；`exp/instr4` 以 `TextBytes` 量，每行另算换行的 2 个字节）：

| Step | 行 | 字节 |
|---|---|---|
| S1 | `- grep: the lines matching a Go regular expression, with their paths and line numbers as read counts them; glob narrows the pages.` | 132 |
| S1 | `- search: words and Chinese text in titles and contents, best first, with excerpts. Use search for words, grep for patterns and line numbers.` | 143 |
| S1 | `- Pages queued in bulk (a batch, an import, a background restore or revert, an upgrade) reach search and a keyed grep once indexed: building says so.` | 151 |
| S2 | `- backlinks: the pages linking to a page, with context.` | 57 |
| S2 | `- index: pages by type, tag or parent page, with summary, tags and inbound link count.` | 88 |

合计 571 字节（`exp/instr6`）。账户入口 3,456 → 4,027（剩 69），笔记本入口 3,386 → 3,957（剩 139）；S1 之后各剩 214 与 284。`search` 一行不写"one Chinese character finds titles only"：工具的说明里有它（[P3](03-P3-batch-revert.md) 第 8 节把这类提醒放在那里，审查 C 的过度设计一条）。第三行写出批量入队的各种来源（审查 C-N4：后台的恢复与撤销、00046 的升级也批量入队，导入的积压要几分钟），只说 `search` 与有键的 `grep`：不取键的 `grep` 立即走查这些页（3.7.2，审查 C2-N5）。`grep` 与 `search` 的分工是 `search` 一行的后一句（4.6、4.13）；"刚写的可能还没进索引"是第三行（[P3](03-P3-batch-revert.md) 第 8 节、3.10.1：单个的 `write`、`edit` 准确入队，`grep` 与 `search` 立即找到）；短的第一页、一个汉字只找标题、`incomplete` 与 `partial_lines` 在工具的说明里（3.11）。`TestTheBuiltInInstructionsNameEveryTool` 要的 `- grep:`、`- search:`、`- backlinks:`、`- index:` 四行在其中（`mcp_tools_test.go:175-187`）。

### 3.15 日志

- mcpserver 照旧每次调用记一行（工具名、码、耗时……，4.15），不记参数。
- `grep` 的"grep walked"（3.7.5）与"grep is saturated"（3.6.5）。
- `search`、`backlinks`、`index` 不另记（用例照旧只在错误时记）。
- 迁移 00046 不记；任务照旧记它的 `run` 的计数（`Written` 里含被入队重建的页）。

### 3.16 配置

不加配置：`grep` 的界线是代码里的常量（4.2 的"MCP 的其余量都是代码里的常量"），名额与解析的队列随 `GOMAXPROCS`；测试经 `page.Deps` 注入（`GrepSlots`、`MaxGrepWork`、`MaxGrepLineWork`、`MaxGrepRead`、`GrepWalk`，0 与 nil 是 domain 的）。`page.Deps` 的新字段进 `bootstrap` 的 `pageDeps` 的"生产留 0"的检查（`trash_test.go:471-473` 的写法）。

### 3.17 新检查的反向对照（13.4 第 1 条）

| 检查 | 反向对照 |
|---|---|
| 键的表格、穷举与性质测试 | 3.5.4 的每处修正各去掉一处（变异 137–140）与整词以 `\b` 为边界（变异 141）：每一条都有 `TestGrepKeys` 的一行失败（它的反例），第 137–139 条另有穷举、第 140、141 条另有性质测试失败（3.5.4 的表）；性质测试的下限数的是变异会显出来的情形（6.1），去掉定向的一半时下限失败 |
| `grep_tables_test.go` 重算危险的结尾的表 | 表里少一个、多一个码位时失败（变异 142） |
| `GrepInsts` 不少于编译的程序 | 估计去掉 `+2` 或把重复按 `Min` 算时失败（变异 143） |
| 解析的队列 | 解析在队列之外做时，测试的假的解析（钩子包着解析本身，3.6.1）数到多于名额数的同时的解析，`TestGrepParseQueue` 失败（变异 144） |
| `partial` 的独立判定 | `scan` 不在 `data:` 处设 `left`、正文的长串不设、名称的长串也设时失败（变异 145、171、146）；`!cut` 经 `add` 加时跨上限的那一行失败（变异 172） |
| 门槛的码位的界 | `BoundFreeContentBytes` 改成 75,000 时 `TestTheThresholdIsUnderTheBound` 失败（变异 149；74,186 起就失败） |
| 迁移的筛法是超集 | 去掉条件 1（240 KiB 的随机 CJK 的页只因上限而 `partial`）、条件 4 或把 SQL 里的 74000 改成 76667（74,001 字节的页不入队）时迁移的测试失败（变异 148、147、150） |
| 块数的测试加 `grep` 的读 | `GrepCandidates` 的 `!cut` 一支不加笔记本（读全部笔记本的 `!cut`）时失败（变异 151：看不到的笔记本在第二段写下每种截断各一页，`!cut` 的位图在那一段多读，6.1） |
| 计划测试 | `IndexPages` 的 lateral 去掉 `LIMIT 1`（成为散列连接）时、没有 00047 时失败（变异 173、184） |
| 码的表的 `server_busy` | 名额不等就答 503 或等到期限时失败（变异 154） |
| 矩阵的四行 | `TestThePermissionMatrixCoversEveryTool` 少一行时失败（P1 的反向对照照旧） |
| 说明的四行 | `TestUnnamedToolsCatchesEachGap` 照旧（P1） |
| `index.list` 的规则行 | `actions_test` 的"规则 = 动作"：少这一行时失败 |

## 4. e2e

两个故事经 `e2e/fixtures/mcp.ts` 的 `mcpClient`（笔记本入口，`clientInfo.name` 是 `e2e-agent`）调用，`resultOf`、`errorOf` 读答复；种下的页经 REST 的 fixture（`createPage` 一类）。断言照 MC8 的做法：MCP 的答复与 REST 的同一个读比较（4.13、4.9 说 MCP 是 M8、M6 的读法原样），不与库比较；MC6 唯一的写另照 MC3 核对库（2.1 第 21 条）。

`fixtures/mcp.ts` 加一个帮助函数 `lineAt(agent, path, line)`：`read(path, offset=line, limit=1)` 的第二块（原样的一行，含 `\r\n`），供两个故事核对行号；答复的类型（`GrepAnswer`、`SearchAnswer`、`IndexAnswer`、`BacklinksAnswer`）在各自的故事里定义（knip 照 P1 的注释：故事导入时才导出，`mcp.ts:12-14`）。

### 4.1 MC6 grep 与 search（S1，`e2e/stories/mcp/mc6-grep-search.spec.ts`）

种下（REST）：页 `Notes`、`Notes/Alpha`（frontmatter `type: note`，之后以 CRLF 写 `# Alpha`、`TODO: alpha one`、`plain`）、`Notes/Beta`（`todo: beta`、`数据库设计说明`）、`Other`、`Other/Gamma`（`TODO: gamma`）、`库存`（标题是两个汉字，正文没有它们）、`Dates/D01` 到 `Dates/D12`（各一行 `2026-10-2x`）。

1. **正则的行号**：`grep {pattern: "TODO: \\w+"}` 答 `Notes/Alpha.md` 第 5 行 `TODO: alpha one`（没有 `\r`）与 `Other/Gamma.md` 的那一行，按路径排，没有 `Notes/Beta.md`（小写）；每个命中的 `lineAt` 是它的文字加原来的行尾（`\r\n` 或 `\n`）；`incomplete`、`building` 为假，`next_cursor` 为 null。
2. **glob**：同一个模式加 `glob: "Notes/**"` 只答 `Notes/Alpha.md` 的那一行。
3. **`ignore_case`**：`grep {pattern: "todo: \\w+", ignore_case: true}` 答三页的三行。
4. **没有字面量的走查与游标**：`grep {pattern: "\\d{4}-\\d{2}-\\d{2}", limit: 5}` 答 `Dates/D01.md` 到 `D05.md` 与 `next_cursor`；带着它接着调用直到 `next_cursor` 为 null：合起来恰好 12 个命中、按路径排、不重复；每一页的 `building` 为假。
5. **search 的摘要与计数**：`search {q: "alpha"}` 的 `results` 与 REST 的搜索（fixture `search(api, credential, slug, q, {notebookId})`，`e2e/fixtures/search.ts:16`）的结果是同样的节点、同样的次序（MCP 的路径换成 id 比较），`Notes/Alpha.md` 的 `excerpt` 含 `alpha`，`count` 等于 REST 的 `count`（`exact`）；`search {q: "数据库"}` 答 `Notes/Beta.md`；`search {q: "库"}` 只答标题里有它的 `库存.md`（一个汉字只找标题），不答 `Notes/Beta.md`。
6. **MCP 刚写的立即两者都找到**：`write {path: "Notes/Fresh.md", content: "zebra crossing\n"}`，紧接着 `grep {pattern: "zebra"}` 答 `Notes/Fresh.md` 第 1 行、`search {q: "zebra"}` 答它（准确入队，候选里有它，不等任务）；照 MC3 核对库（`e2e/stories/mcp/mc3-write-edit.spec.ts:69-88`）：`expectNewPage(db, {id, notebook_id, parent_id, name, created_at}, adminId, "mcp:e2e-agent", content)`（父页 `Notes`，`created_at` 是答复的 `at`）与 `expectChangesetOf(db, changeset_id, {client: "mcp:e2e-agent", byId, items, versions})`（审查 C-M2：`expectContentWritten` 对着 REST 的 `Page` 读、不看变更集与客户端）。

### 4.2 MC7 index 与 backlinks（S2，`e2e/stories/mcp/mc7-index-backlinks.spec.ts`）

种下（REST）：页 `Projects`（`type: area`）、`Projects/Roadmap`（`type: project`、`tags: [plan, plan/q4]`、`summary` 是 300 字节的一句）、`Projects/Spec`（`type: project`、`tags: [plan]`）、`Projects/Notes`（正文 `[[Projects/Roadmap]] and [[Roadmap]]`）、`Inbox`（正文 `see [[Projects/Roadmap]]`）；附件 `Projects/diagram.png`（asset 的 fixture 上传）。

1. **按类型**：`index {type: "project"}` 答 `Projects/Roadmap.md`、`Projects/Spec.md`（同类型按 id），每项的 `type` 是 `project`、`parent` 是 `Projects.md`；`Roadmap` 的 `summary` 是那一句的前 256 字节（字符的边界）、`cut` 是 `["summary"]`，`tags` 是 `plan`、`plan/q4`，`inbound_links` 是 2（`Projects/Notes` 与 `Inbox`，`Notes` 的两条链接算一页）。
2. **按标签**：`index {tag: "plan"}` 答两页（含其下的 `plan/q4`）；`index {tag: "#plan"}` 答同样的两页（开头的 `#` 去掉）；`index {tag: "plan/q4"}` 只答 `Roadmap`。
3. **按父页**：`index {parent: "Projects.md"}` 答三个子页（附件不在其中）；`index {parent: "/"}` 答 `Inbox.md` 与 `Projects.md`；`index {parent: ""}` 答 422 `parent`，`index {type: ""}` 答 422 `type`。
4. **次序与逐项**：`index {}` 先答有类型的（`area` 的 `Projects.md`，再 `project` 的两页），再答没有类型的（按 id）；每项与 REST 逐项读的相同：`getPageProperties` 的 `type`、`summary`（截之前），`getTag` 的成员，`listBacklinks` 的页数就是 `inbound_links`。
5. **backlinks 带路径**：`backlinks {path: "Projects/Roadmap.md"}` 的 `pages` 等于 REST 的 `listBacklinks` 对这一页的答复（`source_id` 换成路径：`Inbox.md` 一条、`Projects/Notes.md` 两条，上下文相同）；`limit: 1` 时答一页与 `next_cursor`，接着答另一页。
6. **附件**：`backlinks {path: "Projects/diagram.png"}` 答 `page.not_found`，文字说它是附件、页的路径以 `.md` 结尾。

MC7 不写库，没有库的断言（第 9 节的例外）。

## 5. 实施步骤

每个 Step 自己合并时是绿的，每个提交也都是绿的：编过、`go vet`、通过不要数据库的测试；需要数据库的测试在那个提交的说明里写明跑过哪些（同 P3）。改一个签名的提交同时改它的每个调用者；工具的各部分（代码、`tools.json` 的条目、说明的行、码的表、矩阵、合法参数）在一个提交里；新的守卫与它的 Makefile 的一行在一个提交里（13.4 第 8 条）。

### 5.1 S1（`m9-p4-s1`，从 `main` 起）

1. **先量常量、再核对被标出的比例**（不改服务端的代码，不下载；审查 C3-I1、C3-M1）：把冻结的语料从主副本（3.3.4）复制进 S1 的 scratch，以 `SHA256SUMS` 与 `verify` 核对每个哈希；照 3.6.4 以 `exp/lineoh2` 的每行的循环在满负载下离线地量 `MaxGrepWork`（较慢的形状）、`MaxGrepLineWork`、`GrepUnitTime` 与 `GrepLineOverhead`，之后的提交用这些值；以它们在冻结的语料上数三条（`exp/freeze` 的数法），数与持平点写进 S1 的审查记录与 9.1。设计定下的是标出三种截断；规则选另一支时 S1 停在这里、报告主持者，由主持者修订设计。那时的缓存另跑一次，只作记录。
2. `feat(page/domain)`：分词的版本 5、`SearchTokens` 答 `partial`、`SearchCutKey`、`BoundFreeContentBytes`；`search_test.go` 的独立判定与跨上限的两行、`search_bound_test.go` 的码位的界（6.1）；同一个提交里每个调用者照新的签名改（任务暂写 `_`；M8 的搜索的测试只为这个新的结果改，审查 C-M13）。
3. `feat(page)`：迁移 00046、`SearchRow.Partial`、`WriteSearchRow`、任务写它；迁移的测试（筛法是超集、每个条件的下限、门槛的两页，真的库上）；`checkSearchBuilt` 加 `partial` 与 `!cut`。之后、第 4 步之前：以 `main` 的二进制把冻结的语料经 M7 的导入导进一个库、任务建好版本 4 的行，再以这一步的二进制 `nervewiki migrate up` 到 00046、在 `serve` 之前读入队的集合，之后 `serve`、任务追上，照 3.3.4 比集合（zip 的布局照 3.3.4；每个路径一页，不去重；预期的集合由 `exp/importset` 照导入自己的规则算；审查 C-M4、B3-n3、C4-M1、B4-n1）：`partial` 的页、入队的页各与预期的集合相同，只在一边的页说明筛法或离线的程序有错，S1 停下、报告主持者。迁移在大的库上的代价在 30 万页的合成的库上量（3.2.2；M8 的语料不在了，不下载）。
4. `feat(page/domain)`：`GrepKeys` 与危险的结尾的表（`go generate`）；`TestGrepKeys`（含每处修正的反例与多于 32 个键的一行）、`TestGrepKeysOverTheRiskyTails`（穷举）、性质测试、`TestGrepTablesAreTheirDefinition`。
5. `feat(page/domain)`：`ParseGrepPattern`、`GrepInsts`、`GrepLines`、`GrepHitOf`、`GrepCursor`、`GrepWork`、`GrepLineCut` 与它们的测试（含审查 A 的最坏的解析的几行）。
6. `feat(page)`：`GrepReads`、`GrepCandidates`、`GrepParses`、`GrepSlots`、用例 `Grep`、`page.Deps` 的五个字段（`GrepSlots`、`MaxGrepWork`、`MaxGrepLineWork`、`MaxGrepRead`、`GrepWalk`，审查 C-N2）与 `page.NewGrepSlots`；用例的表格测试（假的存储）、队列与名额的测试、存储的测试、块数与计划测试；`page/app` 的守卫（P1 的守卫树上的 `grep`，`grep_cost_test.go`）与 `test-go-costs` 的一行、`ci.yml` 里成本那一份的时限 25 分钟（6.5）。
7. `feat(page)`：`Search.InNotebook`、`SearchHit.Tier` 与 `Cursor()`、`SearchDeps.Notebooks`；表格测试（四种看不到与零值的 id）。
8. `feat(mcp)`：page 的 `grep`、`search` 工具、`tools.json` 的四个条目、说明的三行、码的表、矩阵的两行、合法参数，一个提交；整个程序的测试（6.1）。
9. `test(page)`：组合根的 `grep` 的三个时间守卫（`bootstrap/grep_cost_test.go`）与 `test-go-guards` 的一行；照 3.6.4 以真的用例重量 `MaxGrepWork`、`MaxGrepLineWork`、`GrepUnitTime` 与满负载下的 `GrepLineOverhead`，改常量（若量出的与第 1 步的不同）；以量出的 `MaxGrepWork` 与 `GrepLineOverhead` 重算 3.3.4 的第 3 条，数写进审查记录与 9.1；规则因此选另一支时，S1 在合并之前停下、报告主持者。
10. `test(e2e)`：MC6。
11. 审查、修复、核对、合并、推送 `main`；之后的文档提交（第 7 节的 S1 那几处、审查记录、9.1）。

### 5.2 S2（`m9-p4-s2`，从 S1 合并之后的 `main` 起）

1. `feat(page)`：模块根的 `NodePaths` 与 `NewNodePaths(pool)`、`RootPageIDs`、`ChildPageIDs`；测试（每个方法、`FindPage` 的文字与附件）。
2. `feat(linking)`：`Deps.NodePaths`、`ListBacklinks.AtPath`（`Execute` 的后半共用）；组合根接线；测试。
3. `feat(linking)`：`index.list` 的动作与规则行、`linking/domain/index.go`、迁移 00047、`IndexReads` 与三条语句、`ListIndex`；表格测试、计划测试、存储的测试；`index` 的守卫（`linking/adapter/postgres/index_cost_test.go`）与 `test-go-costs` 的一行（审查 C-I3）。
4. `feat(mcp)`：linking 的 `adapter/mcp`、`Tools()`、`mcpTools` 的拼接、`tools.json` 的四个条目、说明的两行、码的表、矩阵的两行、合法参数；整个程序的测试（6.1）。
5. `test(e2e)`：MC7。
6. 审查、修复、核对、合并、推送 `main`；之后的文档提交（第 7 节的全部、P4 的状态、审查记录、9.2）。

## 6. 测试与验证

### 6.1 各项与它们的测试

组合根的新测试都取 `TestThe` 加 F–Z 起头的名字，落在第三份（6.5）；守卫在 `//go:build !race` 的文件里，各在 `test-go-guards`（组合根的）或 `test-go-costs`（包的）自己的一行、带 `-v`，时限都是本机的用时的三倍（P1 第 21 条；第 11 节第 54 条的规则），满负载下的目标（一行约 2 秒、一次约 5 秒）只用于 3.6.4 的实测（审查 C-M5）。

| 项 | 单元与包的测试 | 整个程序（`bootstrap`） | e2e |
|---|---|---|---|
| 分词的 `partial` 与 `!cut`（3.3） | `TestSearchTokensTellWhatTheyLeaveOut`（`page/domain`）：随机的正文（长串、11 个折叠成 ASCII 的码位拼成的串、`data:` 串的各种大小写与长度 59–70、名称里的长串）对照一个独立的判定（在 `searchFold` 之后的文字上以正则找"全是 ASCII 的 65 个以上的单词字符"与"非单词字符之后的 `data:` 加 60 个以上 `inDataURL` 的字符"），`partial` 相同、`!cut` 当且仅当 `partial` 且在最后、不占上限；每种截断（上限、正文里的长串、长的 `data:` 串）都另有照构造的一行，名称里的长串的一行断言不 `partial`（审查 C2-M2）；**上限**以不同的词造（审查 B-I1 = C-I1：一个码位重复只有几个词）：名称 `x`（一个 8 字节的词）加相邻两两不同的 CJK（de Bruijn 序列的前 k + 1 个字，每个二元组 13 字节），k = 76,922（999,994 字节，不到）与 76,923（1,000,007，过了）两行：前一行不 `partial`、76,923 个词，后一行 `partial`、`!cut` 在末尾、词的字节不过 1,000,000（`exp/bound2`）；240 KiB 的随机 CJK（约 230,802 字节起到上限）`partial`，其后的 `needle` 不在词里。`TestTheThresholdIsUnderTheBound`（`search_bound_test.go`）：对全部码位（含 CGJ，U+034F）算 `(3F + 7n) / r` 的最大与折叠出的不是起始的字符每字节的最多，断言 `(BoundFreeContentBytes + 256) × (最大 + 13 × 最多 / 30) ≤ MaxSearchLexemeBytes`（今天 13 与 1，997,506；约 0.5 秒，`-short` 也跑：它不按耗时判定） | `TestTheSearchIndexIsItsRebuild` 加 `partial` 的页，种子里每种截断各至少一页 `partial`（以不同的 CJK 二元组过上限的、正文里有长串的、有长的 `data:` 串的；审查 C-N7、C2-M2，同它的 batch 的下限）：重建的行与原来的相同，含 `partial`；`checkSearchBuilt` 核对每行的 `partial` 与 `!cut` 同真同假、`partial` 等于 `SearchTokens` 的 | — |
| 迁移 00046（3.2） | `TestTheMigrationQueuesEveryPartialPage`（`page/adapter/postgres`，真的库）：迁移到 00045，种下 2,000 页（随机正文同上一行）、3,000 页不会被截的，另**照构造**种下每个条件各一页只由它选中的 `partial` 的页（审查 C-M8）：条件 1 是 240 KiB 的随机 CJK（只因上限而 `partial`），条件 2 是 65 个 ASCII 字母的串，条件 3 是 `data:` 加 60 个字符，条件 4 是 22 个 `ﬃ`；门槛的两页：74,001 与 74,000 字节的 ASCII 的散文（没有别的条件），前者入队、后者不（钉住 SQL 里的数等于 `BoundFreeContentBytes`）；执行 00046，队列里的批量节点 ⊇ `SearchTokens` 判为 `partial` 的页（Go 算），每个条件的那一页都在其中；列的默认值与检查（`partial` 而没有 `body` 的插入被拒）；Down 之后再 Up | — | — |
| 键（3.5） | `TestGrepKeys`（表格：3.5.3 的例子与每种节点、每种修剪各一行；**每处修正一行**，取它的反例（3.5.4：`abſ́` 在 `abſ́̇` 上、`(?i)abſ` 在 `abſ́` 上、`(?i)abΉͅ` 在 `abΉΙ͂` 上、`(?i)ᾼ͂ḢＡ가ﬆ` 在 `ᾼ͂ḢＡ가ﬅ̊` 上、`文(e)\b` 在 `B文eς` 上与 `\bai\b`），断言键与"每个键都在这一行所在的页的词里"；`H` 加 U+0331 一行；多于 32 个键的模式只留按段的次序的前 32 个（审查 C-N6））；`TestGrepKeysOverTheRiskyTails`（`grep_keys_tails_test.go`：审查 C 的穷举，约 31 万行、约 1 秒，`-short` 时跳过）；`TestGrepKeysAreASuperset`（性质，2 万次，`-short` 2,000 次；判定是 Go 的 `regexp` 与 `SearchTokens`，不是被测的代码：Go 匹配 `NFC(行)`、页不 `partial` 时键都在词里；`partial` 的页不进比较、其个数有上限 1%；一半的试验用一般的字母表，一半用定向的（`exp/keys2` 的 `-t`：折叠变长的码位、iota 的轨道、它们的折叠能合成的记号、危险的结尾与它们小写的同轨道的码位，四分之三带 `(?i)`）；行放在 `x\n` 与 `\ny` 之间，或第一行在 BOM 之后、在 CRLF 的行之间、最后一行没有换行，各四分之一（审查 A-n5）；**下限数的是变异会显出来的情形**（审查 C-I2），互不蕴含：结尾的情形（行里一个起始的字符的折叠与它后面的记号的折叠合成）至少 150 次、轨道的情形（`(?i)` 的原子在行里由轨道上另一个、危险或 NFC 的边界与存下的不同的码位匹配）至少 400 次、折叠的情形（逐个原子与整片的折叠给出不同的键）至少 1 次，`-short` 下前两种各至少 10 与 40 次（`exp/keys2/floors.sh`，40 个种子的最少：完整的 301、879、3，`-short` 的 19、78）；另照旧数二元组、三元组与整词（两边是字面的边界）的键，各至少 1% 的有键的次数）；`TestGrepTablesAreTheirDefinition`（约 1 秒，`-short` 也跑，审查 C-N5） | `TestTheMCPGrepFindsWhatItsKeysFind`：经 MCP 写的页与任务建好的行，每种形状的模式（字面、`(?i)`、CJK、整词、没有键）各找到一次；队列里准确的页（任务未跑）是候选；`batch` 写的页答 `building`、不在命中里，任务之后在 | MC6 第 1–3 步 |
| 被截掉的页总被走查 | `TestGrepWalksEveryPartialPage`（`page/app`，假的存储）：键不在 `partial` 的页里，它仍被查、命中 | `TestTheMCPGrepWalksEveryPartialPage`：上限之后的文字：一页 240 KiB 的随机 CJK，`needle` 写在 `full` 设上的那一处之后，测试在 `grep` 之前读库断言这一页的行 `partial`、`needle` 的三元组不在词里（审查 B-I1 = C-I1），有键的 `grep` 找到它；另有长串之中的与 `data:` 串之后的（每种截断各一页）；`Walk` 注入为 5 时 6 个 `partial` 的页答 `incomplete` 与游标，接着找完 | — |
| 解析、指令数与模式（3.6.1） | `TestParseGrepPattern`（表格：1,024 与 1,025 字节、字面的换行、`\n` 的转义照收、NUL 照收、语法错误的消息不含模式、Go 的三种界线、2,000 与 2,001 条；审查 A 的最坏的解析各一行：`\pL`、`\PL` 各 341 个、`(?i)\pL` 340 个、`^\pL?` 255 个、`[\pL0]` 170 个照收（估计 343、343、342、513、172），40 个 `(?i:[\x{100}-\x{10ffff}])` 照收（42），113 个 `\pL{999}` 答 `too_long`；每一行的解析至多分配 8 MB，以 `runtime.MemStats` 的 `TotalAlloc` 前后之差判，不按耗时；`TotalAlloc` 是进程的，所以这个测试与它的行都不 `t.Parallel`，别的测试的分配不落进差里（审查 A2-n1））；`TestGrepInstsIsNoLessThanTheProgram`（2 万个随机的模式与 3.6.1 的边上的模式，`len(syntax.Compile(Simplify(re)).Inst) ≤ GrepInsts(re) ≤ 2 × 它 + 2`，审查 A-m6 = C-M1）；`TestGrepParseQueue`（`page/app`：k = 2 个名额、20 个 goroutine 同时 `Parse`，`GrepParses.parse` 换成假的：数同时在做的解析、每次停几毫秒；断言同时的解析至多 2 个、而确有调用等过（钩子包着解析本身，解析挪到名额之外时数到 20，审查 A2-m1 = C2-M1）；等的时候 ctx 结束的答 ctx 的错误；从不答 503；第 144 条变异） | 码的调用照旧（解析的队列不答码） | — |
| 一行、一次的工作与软期限（3.6.2、3.6.3） | `TestGrepLineCut`、`TestGrepStopsBeforeALineItsWorkWouldPass`（注入小的 `MaxWork`、`MaxLineWork` 与假的时钟：停在不开始的那一行，游标是它；第一行总做；假的时钟在第一次读正文之前就过了软期限时照读、照查第一行（审查 A-m3，第 157 条变异）；一段便宜的行之后每单位的耗时取 `GrepUnitTime`）、`TestGrepCutLines`（`foo$` 恰好在截断处的行进 `partial_lines`，截断之前的命中算命中；一行只规整截断能到的开头） | `TestTheMCPGrepStopsAtItsBounds`：注入的 500 页→5 页、32 MiB→1 MiB、工作、一行的工作各停一次，`incomplete` 与游标、接着走完不重复不遗漏；`limit` 处不 `incomplete`；48 KiB 处的 `Fit`（`limit: 500` 的长行）不 `incomplete`、游标接得上 | MC6 第 4 步 |
| 名额（3.6.5） | `TestGrepSlots`（等的时长注入为几毫秒（审查 C-N10）：等过答 `server_busy`、ctx 先结束答它的错误、放还之后下一个拿到） | 码的调用的 `server_busy`（3.12，真的 1 秒） | — |
| 走查、glob、游标（3.7.3） | `TestGrep`（表格：码的次序每一步（游标在最先）、候选的交、glob 在界线之前取交：`glob: "A/**"` 而 `A` 之前有 600 个候选时一次调用就找到 `A` 的命中、续走从第 `N` 行起、游标的页被改名或删除之后、`building` 只在有键时；假的存储在 `SearchBuilding` 与 `GrepCandidates` 之间清空队列时答复说 `building`（审查 A-m7，第 166 条变异）；假的 `PageStates` 的大小让走查跨两批以上：次序跨批不变、每批一次 `GrepContents`、读的界线在读一批之前判（第 167 条变异）；读回的正文比状态说的大的一页照实计、停在下一批之前（审查 C-M10）） | — | — |
| 行号与 `read`（3.7.4） | `TestGrepLinesAreReads`（`page/app`，性质：随机的正文（CRLF、单独的 `\r`、BOM、空行、没有结尾的换行、空的正文、长行、非 NFC 的行），预言是独立地分行得到的（行号、文字）里 NFC 被模式匹配的那些，与 `grep` 的答复**比集合**（审查 C-M9：只核对每个命中漏不掉"该有而没有"的行）；模式含 `^$` 与 `^`；每个命中经 `ReadPath(offset=行号, limit=1)` 读到的一行去掉 `\r` 与 BOM 等于命中的文字（未截），截取的命中经 `ReadPath(offset, column)` 读到的以它开头；下限：长于 1,000 字节、命中在非 NFC 的部分之后而被截取的行至少 1%）；`TestGrepHitOf`；`TestGrepLines`（表格：`\r\n`、单独的 `\r`、`\r\r\n`、BOM 只在第 1 行、空的正文、最后的 `\n`；审查 C-M12） | `TestTheMCPGrepLinesAreReads`：同一性质的几十个正文经 MCP 的 `write` 与 `read` | MC6 第 1 步 |
| 存储 | `TestGrepCandidates`（`page/adapter/postgres`，真的库；审查 C-M12）：键的交、准确入队的节点、`!cut` 的页（每种截断各一页：上限、长串、`data:` 串，审查 C2-M2）、批量入队的不在其中、别的笔记本的不在其中 | — | — |
| 范围与块数（13.4 第 12 条） | `TestHiddenNotebooksLeaveTheSearchsReadsAlone`（`search_scope_test.go:81-146`）加 `grep` 的读：在测试自己的事务里调 `ListNodes`、`GrepCandidates`（有键、键是 `!cut` 的两种）、`SearchBuilding`、`PageStates`、`Contents`；看不到的笔记本在 A、B、A 三段里交替写不被截与被截的页（第二段写下每种截断各一页：以不同的 CJK 二元组过上限的、65 个字符的串的、长的 `data:` 串的；审查 C2-M2：第 151 条变异下 `!cut` 的读在第二段多读它们的位图）、批量与准确地入队，量块数时它的队列的项**留着**（审查 A-m5 的第一种：这样才量得到队列的一支在有看不到的项时的读）；读的块数在三段里相等，`nodes` 与队列的两个索引照 GIN 的写法容许 ±2 块（看不到的插入让与这本相邻的叶子分裂；B 树长高时每次下降多一块，测试的规模不让它长高，用 `pg_relation_size` 核对三段的大小在同一层） | — | — |
| 计划（13.4 第 9 条） | `TestGrepPlans`（`page/adapter/postgres`）：以 SQL 种下两本笔记本：大的一本 5 万个节点与 10 万个批量的队列的项，读者的一本 1,000 个节点与 3 个准确的项（审查 C-M6：空表上的计划是任意的）；无统计、`ANALYZE` 之后、通用计划三种：`GrepCandidates` 的两个 `page_search_hits` 的支是位图（函数自己的 `SET`），队列的一支是 `page_search_queue_notebook_id_bulk_idx` 的索引条件；`Contents` 是主键、`ListNodes` 是笔记本的索引 | — | — |
| `search`（3.8） | `TestSearchInNotebook`（四种看不到各一行都答 `notebook.not_found`；零值的 id 不查库；次序；`Tier` 与 `Cursor()` 等于 `Execute` 的 `next_cursor`） | `TestTheMCPSearchIsTheNotebooksSearch`：与 REST 的搜索（`searchWorkspace` 带 `notebook_id`）同样的节点与次序、同样的 `count`；`Fit` 在 48 KiB 处截在第二层（正文层）的项时游标接得上 | MC6 第 5、6 步 |
| `NodePaths`（3.9.1） | `page/node_paths_test.go`：每个方法；`FindPage` 的文字与附件 | `TestTheMCPLinksReachTheirUseCases`（交空时失败） | — |
| `backlinks`（3.9.2） | `TestListBacklinksAtPath`（次序、附件、进了回收站的来源不列、游标）；适配器的测试造最大的一项（约 33 KB）断言放得下 | `TestTheMCPBacklinksAreRESTs`（每页的来源、数与上下文等于 `listBacklinks`） | MC7 第 5、6 步 |
| `index`（3.9.3） | `TestListIndex`（表格：每个 422，三个筛选各有 `""` 的一行；`tag` 的 `#plan` 与带 NUL 的（空的一页、不查库）；`parent` 为 `/`；筛选、次序、截断与 `cut`、游标；`limit` 个里最后读的一页没有路径时 `Next` 在它之后（审查 B-N3））、`TestIndexKey`（128 个字符的键的前 64 字节等于整个的：U+01D6 一类的三个字符的合成、组合记号、谚文的字母、`ß`）、`TestIndexReads`（真的库：`translate` 的六行、`type` 在语句里筛、入链数不数链接与自己、到 1,000）、`TestIndexPlans`（3.9.3 的种子与断言）、`linking/adapter/mcp` 的退回（一项 20 个 256 字节的 C0 的控制字符的标签，放不下 48 KiB，去掉标签之后放得下，`cut` 有 `tags`） | `TestTheIndexIsEachPagesReads`（"列表等于逐项读"：每项等于 `getPageProperties`、`listTags`/`getTag`、`listBacklinks` 的页数与 `list` 的路径；看不到的笔记本 404 由矩阵逐列核对） | MC7 第 1–4 步 |
| 工具与契约 | 适配器的 `grep_test.go`（含 3.10.1 的最大的一项）、`search_test.go`、linking 的 `tools_test.go` | `TestTheToolListsAreTheContract`（`tools.json`）、`TestTheBuiltInInstructionsNameEveryTool`、码的调用、矩阵、工具的字符串（3.11–3.14） | MC1 的 `tools/list` 照文件 |
| 时间的守卫（13.4 第 8 条） | `page/app` 的 `grep_cost_test.go`（S1，`test-go-costs` 的一行，审查 C-I3）：`TestGrepOverTheGlobGuardTreeTakesItsTime`：用例 `Grep` 在假的存储上，它的 `ListNodes` 答 P1 的守卫树的形状（10 万条 11 段、每段约 255 字节、互不共享的路径，110 万个节点；P1 的 `guardTree` 是 `page/domain` 的测试的帮助函数，`page/app` 的测试照它另写一份），正文都是一行短的；不取键的模式与 P1 的最坏的 glob 之一，走到 `MaxGlobWork` 停下、答游标，用时照 P1 的守卫的做法比三倍的时限。`linking/adapter/postgres` 的 `index_cost_test.go`（S2，`test-go-costs` 的一行，第 5.2 节第 3 步）：`TestIndexOfAHundredThousandPagesTakesItsTime`：以 SQL 种下 10 万页各 5 个属性，`type` 是 128 个字符的最坏的一种（一个字母与一串记号，审查 B-M3），另 2,000 页各 1,000 个属性而没有 `type`（审查 B-I2）；`IndexPages`、Go 里的次序与一页 100 项的 `InboundCounts` 各自计时、各自比三倍的时限；没有 00047 时 `IndexPages` 多约 2 秒（审查 B 对修订的核对在测试的库的设置下量：2.1–2.2 秒对 0.14–0.19 秒；种子约 6.1 秒），过了它的时限（第 184 条变异） | 组合根的 `grep_cost_test.go`（S1，`test-go-guards` 的一行）：`TestGrepOfTheWorstLineTakesItsTime`（5 MiB 一行的页、3.6.4 的 36 支的选择：一行截在 `GrepLineCut`，审查 A3-m1）、`TestGrepAtItsWorkBoundTakesItsTime`（3.6.4 的一次的界线的形状：500 页各 64 KiB、每行 4 KiB 的密的 ASCII，审查 A3-m1；模式是 36 支的选择，类写成 `[\pL\pN\pM\pS\pP]`，867 字节（审查 C3-M2）：停在 `MaxGrepWork`；测试断言模式不长于 `MaxGrepPatternBytes`、用例收它（不答 422），并断言它做了它界住的工作：编译的程序有 1,999 条指令（各支的开头没有被解析器提出来：开头相同的各支会被合成一支，审查 B2-M2 的例子的 28 支只编译出 155 条）、这个模式匹配 35 个字母加 `QZ` 的一行（最短的匹配放得进一行，`regexp` 不因输入短于它而立刻返回）、正文里没有一行匹配（每行都走完）、"grep walked" 的 `stop` 是 `work` 而 `work` 多于 `MaxGrepWork − GrepWork(4095, 1999)`（预先判断停在一行之内，审查 A3-n2））、`TestGrepOfAHundredThousandCandidatesTakesItsTime`（10 万页的搜索的行以 SQL 写下：照这本笔记本的前缀写的 32 个三元组，每页都有（审查 A-n4），`search_scope_test` 的造法，不跑任务（审查 C-M5：任务每秒 190–300 页要 5–9 分钟）；模式是这 32 个三元组拼成的字面串：候选的语句与取交，在无统计、`ANALYZE` 之后、通用计划三种状态下各计时） | — |

### 6.2 P4 怎样量它的界线

- **被标出的比例**：设计照规则定下（标出三种，3.3.4；在冻结的语料上，`GrepLineOverhead` 离线校准为 2，3.6.4）。S1 的第 1 步以满负载下量出的常量在冻结的语料上再数、第 3 步之后在导入冻结的语料的库上与照导入的规则算出的预期的集合比、第 9 步以真的用例量出的 `MaxGrepWork` 与 `GrepLineOverhead` 重算第 3 条：规则选另一支时停下、报告主持者。
- **`MaxGrepWork`、`MaxGrepLineWork`、`GrepUnitTime`、`GrepLineOverhead`**：3.6.4，满负载、本机，两次：S1 的第 1 步以 `exp/lineoh2` 的每行的循环离线地量（审查 C3-M1），第 9 步以守卫的那几个测试加一个只在本机跑的基准（`BenchmarkGrepAtFullLoad`，不进 CI）重量；一次的界线照较慢的形状（每行 4 KiB，审查 A3-m1）。
- **解析的队列**：S1 以 `exp/parseq` 的做法在本机重量 200 个同时的最坏的解析（峰值、分配、最长的等待），写进 9.1。
- **迁移的代价**：S1 在 30 万页的合成的库上执行 00046（M8 的语料不在了，不下载；3.2.2、3.3.4），种子先报出它的份额（入队的页的字节与词，对准冻结的语料的，3.2.2），再分别记入队的页数、它们的字节、迁移的用时、任务重建它们的用时与 WAL（9.1），标明是合成的库上量的。
- **`page_properties` 的索引**：00047 无条件加（3.9.3），计划测试与守卫核对它被用上。
- **10 万个候选的取交、10 万页的 `index`、守卫树上的 `grep`**：守卫的本机用时写进 9.1、9.2，M12 的性能移交加这几行。

### 6.3 变异

实施者逐个做下面的改动（每个之后还原），确认右边的测试失败，结果写进第 9 节（照 P3 的 6.3）。编号接着 [P3](03-P3-batch-revert.md) 6.3 的 136（S3 的文档把 P3 的变异排到了 136，审查 C-M11）。第 137–141 条另有 `TestGrepKeys` 的一行必定失败（3.5.4）。

| # | 变异 | 应失败的测试 |
|---|---|---|
| 137 | 结尾的风险只看最后一个码位，不看整个组合序列 | `TestGrepKeys`（`abſ́` 一行）；`TestGrepKeysOverTheRiskyTails`；`TestGrepKeysAreASuperset`（10 个种子里 6 个） |
| 138 | 带折叠的原子的风险只看存下的码位，不看轨道 | `TestGrepKeys`（`(?i)abſ` 一行）；`TestGrepKeysOverTheRiskyTails` |
| 139 | 结尾不看它之后的原子的折叠（只看起始的） | `TestGrepKeys`（`(?i)abΉͅ` 一行）；`TestGrepKeysOverTheRiskyTails` |
| 140 | 对整片 `searchFold`，不逐个原子折叠再 NFC | `TestGrepKeys`（`(?i)ᾼ͂ḢＡ가ﬆ` 一行）；`TestGrepKeysAreASuperset` |
| 141 | `\b`、`\B` 当作边界取整词 | `TestGrepKeys`（`\bai\b`、`文(e)\b` 两行）；`TestGrepKeysAreASuperset` |
| 142 | 危险的结尾的表少 `H` | `TestGrepTablesAreTheirDefinition`；`TestGrepKeys`（`H` 加 U+0331 一行） |
| 143 | `GrepInsts` 不加 2 | `TestGrepInstsIsNoLessThanTheProgram` |
| 144 | 解析在解析的队列之外（先解析、再取它的名额） | `TestGrepParseQueue`（假的解析数到同时多于 2 个） |
| 145 | `data:` 的跳过不设 `left` | `TestSearchTokensTellWhatTheyLeaveOut`；`TestTheMCPGrepWalksEveryPartialPage` |
| 146 | 名称里的长串也设 `left` | `TestSearchTokensTellWhatTheyLeaveOut` |
| 147 | 迁移去掉条件 4 | `TestTheMigrationQueuesEveryPartialPage`（22 个 `ﬃ` 的那一页） |
| 148 | 迁移去掉条件 1 | 同上（240 KiB 的随机 CJK 的那一页，只因上限而 `partial`） |
| 149 | `BoundFreeContentBytes` 改成 75,000 | `TestTheThresholdIsUnderTheBound` |
| 150 | 迁移的 SQL 的 74000 改成 76667 | `TestTheMigrationQueuesEveryPartialPage`（74,001 字节的那一页不入队） |
| 151 | `GrepCandidates` 的 `!cut` 一支读全部笔记本（`page_search_tags` 的全部） | `TestHiddenNotebooksLeaveTheSearchsReadsAlone`（第二段的每种截断的页） |
| 152 | 批量的节点也当候选 | `TestGrepCandidates`；`TestTheMCPGrepFindsWhatItsKeysFind`（`building` 时的命中） |
| 153 | 先收 500 个候选、再按 glob 筛 | `TestGrep`（`A/**` 一行） |
| 154 | 名额不等，取不到立刻 503 | `TestGrepSlots` |
| 155 | 预先判断改为做完一行之后再看 | `TestGrepStopsBeforeALineItsWorkWouldPass` |
| 156 | 第一行也预先判断 | 同上（第一行就过界的一次调用答零行而游标不动） |
| 157 | 读正文之前的软期限不论查没查过一行 | 同上（第一次读之前就过了软期限的一行） |
| 158 | 结束在截断处的命中算命中 | `TestGrepCutLines` |
| 159 | 最后的 `\n` 之后另起一个空行 | `TestGrepLinesAreReads`（`^$` 的模式）；`TestGrepLines` |
| 160 | 不去掉行尾的 `\r` | 同上；MC6 第 1 步 |
| 161 | 不去掉 BOM | `TestGrepLines`；`TestGrepLinesAreReads`（`^` 的模式） |
| 162 | 截取用 NFC 的偏移，不映回原样的字节 | `TestGrepHitOf`；`TestGrepLinesAreReads`（被截取的长的非 NFC 的行） |
| 163 | 停在 `limit` 也答 `incomplete` | `TestGrep`；`TestTheMCPGrepStopsAtItsBounds` |
| 164 | 不取键的模式也答 `building` | `TestGrep` |
| 165 | 续走的页从第 1 行起 | `TestGrep`；`TestTheMCPGrepStopsAtItsBounds`（重复的命中） |
| 166 | `SearchBuilding` 在 `GrepCandidates` 之后读 | `TestGrep`（两次调用之间清空队列的一行） |
| 167 | 读的界线在读了一批之后判 | `TestGrep`（跨批的一行） |
| 168 | `InNotebook` 对别的工作区的笔记本答 `workspace.not_found` | `TestSearchInNotebook` |
| 169 | `SearchHit.Tier` 不设（游标都按标题层重建） | `TestSearchInNotebook`；`TestTheMCPSearchIsTheNotebooksSearch`（48 KiB 处） |
| 170 | `grep` 的答复被 48 KiB 截短时照样 `incomplete` | 适配器的 `grep_test.go`；`TestTheMCPGrepStopsAtItsBounds` |
| 171 | 正文里超过 64 个的 ASCII 串不设 `left` | `TestSearchTokensTellWhatTheyLeaveOut`（长串的一行）；`TestTheMCPGrepWalksEveryPartialPage`（长串之中的那一页） |
| 172 | `!cut` 经 `add` 加（到上限时丢掉） | `TestSearchTokensTellWhatTheyLeaveOut`（过了上限的那一行：`partial` 而没有 `!cut`） |
| 173 | `IndexPages` 的 lateral 去掉 `LIMIT 1`（取第一个之外的也连上） | `TestIndexPlans`；`TestIndexReads`（重复的页） |
| 174 | 没有类型的页排在前面 | `TestListIndex`；MC7 第 4 步 |
| 175 | `IndexKey` 由前 64 个字符算 | `TestIndexKey`（U+01D6 一类的合成） |
| 176 | 第一个 `type` 键不是字符串时往后找 | `TestIndexReads` |
| 177 | 入链数数链接，不数来源页 | `TestIndexReads`；`TestTheIndexIsEachPagesReads` |
| 178 | 入链数数自己 | `TestIndexReads` |
| 179 | `backlinks` 对附件的路径答它的反链 | `TestListBacklinksAtPath`；MC7 第 6 步 |
| 180 | 没有路径的来源页以空的路径列出 | `TestListBacklinksAtPath` |
| 181 | `type` 为 `""` 当作没有 | `TestListIndex` |
| 182 | `index` 的第一项放不下时不退回 | linking 的适配器的测试（`internal_error`） |
| 183 | 规则表少 `index.list` | `TestTheRuleTableIsTheModulesActions`；矩阵的 `index` 一行 |
| 184 | 迁移 00047 不建 `type` 的部分索引 | `TestIndexPlans`（索引的名字）；`TestIndexOfAHundredThousandPagesTakesItsTime` |
| 185 | `tag` 不去掉开头的 `#` | `TestListIndex`（`#plan` 一行）；MC7 第 2 步 |

### 6.4 门禁

每个 Step 合并之前，本机：`make lint`、`make knip`、`make gen-check`、`make test-go`（竞态检测下的全部、组合根的三份、`test-go-costs` 与 `test-go-guards`）、`make test-go-short`、`make test-web`、`make build`、`make e2e`，都通过；CI 的各份都是绿的。之后照 M9 的做法：审查（Opus）、修复、核对、合并、推送 `main`（负责人的规则：完成并核对过的单元由主持者推送）。守卫的时限照 3 倍的规则由本机的用时定（P1 第 21 条）；CI 上守卫失败时照 M8 的规则重看它的时限（第 11 节第 54 条）。

### 6.5 CI 的余量

**现在**（`main` 在 `56ef34c8` 的 run 39555357038 与 S3 的分支的运行，[P3](03-P3-batch-revert.md) 6.5 与 9.3）：组合根第一份最近两次 6:12、7:03（平均 6:37.5），第二份 5:02–6:00（P3 的 S3 要下一个往第二份加测试的 Phase 先估计：离 6:30 约 30 秒），第三份 3:39–5:18；线在约 7:30（`-timeout 15m` 的一半）。CI 与本机之比约 1.2–1.4、2.2–2.4、1.8–1.9。守卫的作业 11:25–12:55（时限 30 分钟），成本的作业约 6–10 分钟（时限 20 分钟，第 11 节第 54 条），竞态 8:41–9:07（20 分钟），e2e 的 E2E 一步 3:28–3:37。

**规则照旧**（第 11 节第 28 条，[P3](03-P3-batch-revert.md) 6.5）：一份过了线，只在一个邻居挪过之后两次运行的平均仍在线之下时才挪界线；否则那个 Step 按测试名再分一份；从不提高 `-timeout`。第三份余地最多，所以 **P4 的每个新的组合根测试都取 `TestThe` 加 F–Z 起头的名字**（`TestTheMCPGrep…`、`TestTheMCPSearch…`、`TestTheMCPLinks…`、`TestTheMCPBacklinks…`、`TestTheIndexIsEachPagesReads`、`TestTheMigration…` 不在组合根）；矩阵的新行在 `TestThePermissionMatrixOverMCP`（第三份）里；没有一个新的测试叫 `TestA…`–`TestD…`、`TestE[a-u]…`、`TestEvery[A-R]…`。实施者以 `cd server && go test -race -list '<表达式>' ./internal/bootstrap` 对 `BOOTSTRAP_FIRST`、`BOOTSTRAP_THIRD` 各列一次（第二份是其余的），每个新的测试各在哪一份写进审查记录；守卫的文件是 `!race`：带 `-race` 的分份里它们不编进去，不算；不带 `-race` 列出时它们在第二份（其余的）之下，两个表达式都不匹配它们（审查 C2-N1）。

| 份 | 现在（测试一步） | S1 加的（本机，竞态） | S2 加的 | P4 之后的估计（CI） |
|---|---|---|---|---|
| `bootstrap-1` | 6:12、7:03 | 无 | 无 | 不变 |
| `bootstrap-2` | 5:02–6:00 | 无 | 无 | 不变 |
| `bootstrap-3` | 3:39–5:18 | 矩阵 +52 格约 8–14 秒；码的两行约 2 秒（含名额的 1 秒等待）；工具的字符串约 1 秒；`TestTheMCPGrep…` 四个约 13–18 秒（各种下一两本笔记本、跑一次任务；上限的页 240 KiB、约 7.7 万个词）；`TestTheMCPSearchIsTheNotebooksSearch` 约 2 秒；`TestTheSearchIndexIsItsRebuild` +1 秒：约 +27–38 秒 | 矩阵 +52 格约 8–14 秒；码约 1 秒；字符串约 1 秒；`TestTheMCPLinksReachTheirUseCases`、`TestTheMCPBacklinksAreRESTs`、`TestTheIndexIsEachPagesReads` 约 5–7 秒：约 +15–23 秒 | 乘约 1.9：S1 之后约 4:30–6:30，S2 之后约 5:00–7:14 |
| `guards` | 作业 11:25–12:55 / 30 | 组合根的三个 `grep` 的守卫一行（本机约 15–25 秒：5 MiB 与 32 MiB 的种下、10 万页的行以 SQL 写下、三种统计），乘 2.4 约 36–60 秒 | 无 | 作业约 +1 分钟 |
| `costs` | 作业约 6–10 分钟 / 20 | `page/app` 的守卫树上的 `grep` 一行（本机约 5–10 秒：建 110 万个节点的行与树、走到 `MaxGlobWork`），乘 2.4 约 12–24 秒 | `index` 的守卫一行（本机约 10–15 秒：以 SQL 种下约 250 万个属性的行，审查 B 量得约 6.1 秒，再三次计时），乘 2.4 约 25–35 秒 | 作业约 6.5–11 分钟；S1 照第 11 节第 54 条把时限提到 25 分钟（2.1 第 24 条） |
| `race`（组合根之外） | 作业 8:41–9:07 / 20 | 键的性质测试 2 万次（不带竞态约 1 秒，竞态约 4–8 秒）、穷举（约 1 秒，竞态约 5–10 秒）、表的重算（约 1 秒，竞态约 5–10 秒）、码位的界（约 0.5 秒，竞态约 3–5 秒）、`page/app`（含队列的测试）、存储（迁移、块数、计划）与适配器约 10–15 秒 | linking 的约 5 秒 | 约 +35–50 秒 |
| `short` | 1:53–3:12 | 键的性质测试 2,000 次、表的重算、码位的界；穷举跳过 | — | 约 +5 秒 |
| `e2e` | E2E 一步 3:28–3:37 | MC6 约 6–8 秒 | MC7 约 5–7 秒 | 约 3:40–3:55 |

第三份在 P4 之后的估计的上沿（约 7:14）离线只约 16 秒：**S2 合并之前**以本机每份单独跑两次的平均乘这一份的比估计，合并之后以 CI 的两次运行核对（同 P3 的每个 Step）；估计过线时 S2 按测试名把第三份的一部分分到第四份（下一段），不提高 `-timeout`，写进 S2 的审查记录。下沿（S1 的测试本机多是一两秒的、矩阵的格各几十毫秒）更可能：P3 的 S3 的四个测试估计 18–20 秒、实测 7.4 秒。

**第四份，现在就写下**（审查 C-M7）：第 11 节第 64 条说第一份过了线时 `TestEvery[M-R]`（本机竞态下约 28.4 秒，CI 约 +53 秒）挪到第三份；P4 之后第三份收不下它（约 7:14 加 0:53 约 8:07，过了线）。所以 P4 之后的规则是：第一份过线时它挪到第四份 `BOOTSTRAP_FOURTH = ^TestEvery[M-R]`（第一份的表达式去掉 `very[M-R]`，第三份不变，第二份照旧以三者的并 `-skip`），`ci.yml` 的矩阵加 `bootstrap-4`（时限 25 分钟）；第三份自己过线时 S2 把 `^TestThe(M|P)` 放进第四份：第一份已经挪过时 `BOOTSTRAP_FOURTH = ^Test(Every[M-R]|The(M|P))`，还没挪过时只是 `^TestThe(M|P)`（那时第一份的 `very[A-R]` 仍含 `TestEvery[M-R]`，写成两者的或会让那 9 个测试跑两次，审查 C3-N1）；并把第三份改成 `BOOTSTRAP_THIRD = ^Test(The[F-LNOQ-Z]|T[i-z]|[U-Z])`：RE2 没有向后看，第三份要自己去掉它们，否则它们跑两次、第三份也不变小（审查 C2-N2）。第四份在 `Makefile` 里是 `test-go-bootstrap-4`：竞态的几份并行的 `-j4` 改成 `-j5`（`Makefile:189`），`test-go-bootstrap` 的汇总加上它，第二份的 `-skip` 是另三份的并；在 `ci.yml` 的矩阵里是 `bootstrap-4`。第四份的用时是估计、要量：`TestEvery[M-R]` 本机竞态约 28.4 秒；`TestThe(M|P)` 是今天第三份 117 个测试里的 39 个（含三个权限矩阵的测试），挪之前在本机量它们。第 64 条的文字随设计的提交改成这样（2.1 第 25 条）。
## 7. 完成标准

**S1**：
- `grep` 与 `search` 在两个入口里，`tools.json` 的四个条目与代码相同；内置的说明有它们的三行（426 字节），两个入口都在 4 KiB 之内（账户入口 3,882、笔记本入口 3,812）。
- 被标出的比例照设计标出三种截断（3.3.4）：第 1 步在满负载下离线地量过四个常量（3.6.4），以它们在冻结的语料上数过三条（不下载语料），数与持平点写进审查记录与 9.1，规则没有选另一支；第 3 步之后在导入冻结的语料的库上，`partial` 的页与入队的页的集合各与 `exp/importset` 照导入自己的规则算出的预期的集合相同（3.3.4）；第 9 步以真的用例重量过四个常量，规则的工作的一条以量出的 `MaxGrepWork` 与 `GrepLineOverhead` 重算过、仍成立（不成立时 S1 停下、报告主持者，不合并）；守卫在 CI 上；迁移 00046 在 30 万页的合成的库上的入队的页数、字节、用时与 WAL、任务重建它们的用时写进 9.1，标明是合成的库上量的，种子的份额（入队的页的字节与词）在目标的约 20% 之内或写明折算过（3.2.2）；解析的队列的实测写进 9.1；成本那一份的时限是 25 分钟。
- 6.1 里 S1 的每个测试、6.3 的第 137–172 条变异、MC6 通过或被抓到；M8 的搜索的测试只为 `SearchTokens` 的新的结果改、改了之后通过，搜索的答复不变（`!cut` 不改变搜索的任何结果；审查 C-M13：`SearchTokens` 多了一个结果，它的调用者都要改）；迁移 00046 上下都通过。

**S2**：
- `backlinks` 与 `index` 在两个入口里，`tools.json` 的四个条目与代码相同；说明的两行（145 字节）之后两个入口 4,027 与 3,957 字节；`index.list` 在规则表里。
- 迁移 00047 的两个部分索引上下都通过，计划测试与守卫核对它们被用上，结论写进 9.2；6.1 里 S2 的每个测试、第 173–185 条变异、MC7 通过或被抓到；REST 的 `listBacklinks`、`getTag` 一类不变。

**文档**（P4 的文档提交，照 P1–P3 以脚本落下、每段原文恰好出现一次才改）：
- **00 号文档**：2.1 的 25 条（`scratchpad/m9/p4/apply-00m9.py`，随这份设计的提交落下：第 12 节的 P4 一行写"进行中（设计已审查）"，第 13 节加一行）；各 Step 合并时第 12 节的 P4 一行与第 13 节的变更记录；4.14 的 `grep` 一行写上量出的值（2.1 第 23 条，S1）；"移交的落实"里 P4 的两行写上落实（M8/P5 第 1、3、4 条，M6/P4 第 4 条）；第 11 节接着 S3 的第 76 条加 P4 的主持者的决定（被标出的比例的结论与持平点、实测的界线）。
- **v0.1 总体设计**（随各 Step 的合并；今天没有与 P4 无关的错，不写 `apply-v01.py`，见审查记录）：
  - 7.2 `page_search` 一行（`v0.1-design.md:722`）**已经**写着 `partial` 与 `!cut`（"被截掉或留在外面的词的 `partial`……分词的版本 5，M9/P4"，M9 的设计的提交 `e16a98f7` 写的），不改（审查 B-N1 = C-M13）：三种截断都标出，它的"被截掉或留在外面的词"照旧对。
  - 第 11 节"搜索的索引"一条（`:902`）的"其余的由升级之后的 `nervewiki reindex` 重建，全部重建的代价同 00041（30 万页约 16 分钟、约 130 GB 的 WAL）"改为"其余的不入队：它们的词在版本 5 下不变，升级之后的 `nervewiki reindex` 不是必须的；但之后不论为了什么跑的第一次 `nervewiki reindex` 也重建所有仍是版本 4 的搜索的行（30 万页约 26 分钟、约 290 GB 的 WAL，其间各笔记本答 `building`）"，前一句的"只把可能被截过的页入队"后加"（正文多于 74,000 字节的、有长串或长的 `data:` 串的；像离线的语料时约是词的五分之一，按词估 30 万页约 5 分钟、约 56 GB 的 WAL，S1 在 30 万页的合成的库上量的数写在旁边）"（S1；同一条里前文已写着重建已有的行的 26 分钟与 290 GB，这一句今天与它自相矛盾，审查 B-M6）。
  - 6.3 的 `grep` 一行（`:663`）加"被截掉词的页总被走查，走不完答 `incomplete`；一行过长时只匹配开头、列在 `partial_lines`"（S1）。
  - 13.1 第 4 条（`:1040` 一段）："搜索一类的候选读"之后加 `grep`：键与 `!cut` 只读这本笔记本的、树与正文按笔记本与 id 读，块数的测试含它的语句；`search` 的入口里看不到的一律 `notebook.not_found`（S1）。第 10 条：`grep` 的"grep walked"一行（S1）。第 11 条：`page.NewNodePaths(pool)` 与 linking 的 `Deps.NodePaths`（S2）。第 31 条的 MCP 一段加 `grep` 的界线（模式 1,024 字节、解析的队列 `GOMAXPROCS`、2,000 条指令、32 个键、一行与一次的工作的实测值、500 页与 32 MiB、软期限 3 秒、名额、`limit` 1–500、一行 1,000 字节）（S1）与 `index` 的（`type` 的筛选 1,024 字节、次序的键 64 字节与 128 个字符、20 个标签、字符串 256 字节、入链数到 1,000、`limit` 1–100）（S2）。
  - 13.4 第 8 条（`grep` 的四个守卫与 `index` 的守卫，各在 `test-go-costs` 或 `test-go-guards` 自己的一行、带 `-v`，时限本机的三倍）、第 9 条（`grep` 的读、`ListIndex` 与入链数的计划测试，种子与断言照 6.1、3.9.3）、第 10/13 条（`grep` 的行号与 `read`）、第 12 条（`grep` 的读进块数的测试，`nodes` 与队列的索引的 ±2 块）：照建成的写（S1、S2）。
- **README**：MCP 一节的"工具"一条（`README.md:210`）的"`grep`、`search`、`index`、`backlinks` 随 M9 之后的 Phase 加上"改为"读与找见下两条"（S2），之后加"**找**"一条（S1：`grep` 的语法、行号同 `read`、`\r` 与 BOM、glob、`ignore_case`、界线与 `incomplete`、`partial_lines`、`building`（批量入队的各种来源）；`search` 限一本笔记本、一个汉字只找标题、第一页可能为空而带游标；分工）与"**索引与反链**"一条（S2：`index` 的筛选（`tag` 的 `#`、空的字符串被拒）、次序、截断与 `cut`、入链数、索引里没有的页；`backlinks` 带路径、附件答 `page.not_found`）；部署一节的"升级到带搜索的版本（M8）"一条（`:328`）之后加 P4 的升级：迁移只把可能被截过的页入队（正文多于 74,000 字节的、有长串或长的 `data:` 串的），像离线的语料时约是全部的词的五分之一（按词估 30 万页约 5 分钟、约 56 GB 的 WAL；S1 在 30 万页的合成的库上量的数写在旁边、标明是合成的），其间它们所在的笔记本答 `building`；之后的 `nervewiki reindex` 不是必须的，**但之后不论为了什么跑的第一次也重建所有仍是版本 4 的搜索的行**（约 26 分钟、约 290 GB，审查 B-M6）（S1）。
- **M12 的移交**：[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项加：`grep` 的解析在进程的队列里（`GOMAXPROCS` 个名额；最坏的解析与队列下的实测见 P4 3.6.1、00 号文档 4.13），每次读整棵树与候选的 id（10 万个候选的守卫的用时）、正文至多 32 MiB 一次、一次调用同时至多一批正文（5 MiB）、一行的 NFC 副本（至多约 15 MB）与编译的程序，名额与工作按真实的负载再量；`index` 每次读这本笔记本的全部索引的页与每页的类型的前 128 个字符（10 万页的守卫的用时；最坏一次约 65 MB 活着的内存、标题键约 0.7 秒）、一页 100 项的入链数至多约 10 万次下降、属性经 00047 一页一次探查；迁移 00046 在 30 万页的合成的库上、00047 在守卫的种子上的用时（S1、S2 合并时各写上）。

### 7.1 前面的 Phase 与移交交给 P4 的

| 来源 | 项 | 落实 |
|---|---|---|
| [P1](01-P1-platform-reads.md) 第 8 节 | `NodePaths.Paths` 与 `page.NewNodePaths(pool)`，只为问到的 id 拼路径；linking 的 `Deps.NodePaths`、`backlinks` 的最后一跳 | 3.9.1（`Paths` 是 P2 建好的，P4 只加模块根与 linking 的端口）；`TestTheMCPLinksReachTheirUseCases` |
| P1 第 8 节 | `grep` 的 glob 复用 `domain.Glob` 的位并行的匹配与按树走，在走查之前与候选取交，同一个 `MaxGlobWork` 管它、与 `grep` 的工作分开计；守卫加 `grep` 的形状 | 3.7.3；`page/app` 的 `TestGrepOverTheGlobGuardTreeTakesItsTime`（P1 的守卫树在假的存储上，`test-go-costs`；审查 C-I3：在组合根里要种 110 万个节点） |
| P1 第 8 节 | `grep` 与 `search` 的答复经 `Fit` 停在 48 KiB | 3.10.1、3.10.2 |
| [P2](02-P2-writes-history.md) 第 8 节 | `page.NewNodePaths(pool)` 与 linking 的 `Deps.NodePaths` 复用 `NodePaths.Paths`（`PathStore.PathsOf` 经 `LinkTargetsByIDs`），不另写给路径的语句 | 3.9.1 |
| P2 第 8 节 | 四个工具的说明的行与工具的字符串的合法参数 | 3.14、3.12 |
| P2 第 8 节 | `backlinks` 的来源页带路径同 `changeset` 的 `node.path`；错误照 3.10.3（只引这次调用自己的路径，原样引起） | 3.10.3、3.9.1 |
| [P3](03-P3-batch-revert.md) 第 8 节 | 四行与分工在账户入口的 640 字节之内，4.13 的提醒进工具的说明 | 3.14（571 字节，剩 69）、3.11 |
| P3 第 8 节 | P4 之后说明离 4 KiB 只剩约 50 字节，之后加字的先缩短 | 实际剩 69（账户入口），第 8 节照旧交给 P5、M10 |
| P3 第 8 节 | `grep` 与 `search` 的说明写"刚写的可能还没进索引" | 说明的第三行、两个工具的 `building` 的一句（批量入队的各种来源） |
| P3 第 8 节 | 码的表、矩阵、工具的字符串、`tools.json` 照 P2、P3 的写法；新的整个程序测试照 6.5 取名、以 `-race` 的 `go test -list` 核对 | 3.11–3.13、6.5 |
| [M8/P5 的移交](handoffs/M8-P5-search.md)第 1 条 | 键与候选（只读这本笔记本的键、并上准确的节点、批量的不是候选而答 `building`、取不出键的有界地走查）；SQL 里的复查与 PostgreSQL 的 NFC 的问题 | 3.4、3.5、3.7；复查在 Go 里、两边都是 Go 的 NFC，`nfcGaps` 一类的空隙不存在（4.13）；不在 SQL 里连接，两个陷阱碰不到（2.1 第 20 条） |
| M8/P5 的移交第 3 条 | `search`：同一个 `Search` 用例、两层、约数、`building`、第一页可能为空、被隔开的单字只找名称 | 3.8、3.10.2、3.11 的说明 |
| M8/P5 的移交第 4 条 | `q` 的上限照旧；`grep` 的正则另定上限并在上限处测试 | 3.6.1、6.1 |
| [M6/P4 的移交](handoffs/M6-P4-rewrite.md)第 4 条 | `backlinks` 调反链的查询（分页、1,000、上下文）；`index` 读 `page_properties`、`page_tags`，标签含其下的，属性按 jsonb 的值，不另写 SQL、不在 linking 之外连它的表；索引里没有的页不在其中 | 3.9.2（`Execute` 的后半原样）；3.9.3：移交的"不另写 SQL"已由 00 号文档 4.9 取代（`ListIndex` 是新的查询，入链数与次序要新的读，`InboundCounts` 是移交没有料到的一条语句），"不在 linking 之外连它的表"照办（审查 B-N5） |

## 8. 交给后面的 Phase

- **P5（其余故事的 MCP 版本与接入文档）**：
  - 接入文档写 `grep` 与 `search` 的分工、`incomplete` 与 `partial_lines` 怎样读、`building`、一个汉字只找标题；被截掉词的页总被走查，`grep` 没有悄悄的漏找，走不完答 `incomplete`（3.3.4）。
  - 人工清单加：两种客户端里 `grep` 的答复（48 KiB 的命中、`partial_lines`）是否完整交给模型；agent 是否照说明以 `read(offset, column)` 读截取的行与部分匹配的行；`readOnlyHint` 的四个工具在各审批选项下不要确认。
  - 其余故事的 MCP 版本里：L4 的标签由 `index` 读、L6 的右栏由 `backlinks` 读、SE1 由 `search` 读（第 9 节的表），照 MC6、MC7 的比较 REST 的写法。
  - 说明在 P4 之后账户入口剩 69 字节：P5 若在说明里加字，先缩短已有的行（`TestTheBuiltInInstructionsAreShort` 照旧守 4 KiB）。
- **M10**（[M9-P1 的移交](../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md)）：
  - 虚拟的 `index.md` 复用 `ListIndex`，承袭它的界线（每页 20 个标签、入链数到 1,000、一页 100 项、字符串 256 字节与 `cut`），一个文件要读全部，要有它自己的界线与截断的说法；`ListIndex` 每次读这本笔记本的全部索引的页与类型（3.9.3），虚拟的文件按页读时同样。
  - `grep` 只搜真的页的正文：虚拟的 `index.md`、`log.md` 不在树里，`grep` 的 glob `**/*.md` 不含它们；M10 定要不要让 `grep` 找到它们（`NodePaths.Find` 与 `ListByPattern` 在树之外认得它们时，`grep` 的走查同样要加）。
  - 说明剩 69 字节，同上。
- **M12（发布）**：第 7 节的性能移交各项（解析的队列、`grep` 与 `index` 一次调用的内存）；`grep` 的界线与名额按真实的负载再量（00 号文档"写给后面的 M"已列）；`index` 对三个筛选的 `""` 的 422 与 `log {path: ""}` 一起进打磨的清单（第 11 节第 41 条的同一类）；被标出的比例的规则在真实的笔记库上的持平点（3.3.4：每 10,000 页约 25 页带 200 KB 的 base64 图片的页），有用户的库时再看。

## 9. 结果

（实施之后写：S1 第 1 步的四个常量（满负载、离线）与冻结的语料上的三条（页数、字节、工作，持平点）、那时的缓存上的数（记录）、导入冻结的语料的库上的两个集合、第 9 步的常量与规则的重算、解析的队列的实测、迁移在合成的库上的代价（种子的份额、入队的页、字节、用时、WAL）、提交、门禁、CI（成本那一份的作业时间）、变异的结果、偏离；S2 的计划测试与守卫的结论、00047 的用时、提交、门禁、CI、变异的结果、偏离。）

### 9.1 S1

### 9.2 S2

## 附录：在代码里核对过的（`56ef34c8`）

| 决定 | 核对的代码 |
|---|---|
| 分词与 `partial` 的三种情形（3.3） | `page/domain/search.go:53-62`（`MaxSearchLexemeBytes`、`SearchPrefixBytes`）、`:63-68`（`MaxSearchWordRunes`）、`:88`、`:94`（版本与 `SearchTokenizer`）、`:206-221`（`classOf`）、`:230-240`（`searchFold`）、`:252-261`（`SearchTokens`：正文是 `name + "\n" + content`）、`:272-289`（`add` 与 `full`）、`:293-339`（`scan`：`data:` 的跳过在 `word < 0` 时）、`:344-346`（`inDataURL` 只收 ASCII）、`:351-353`（`longASCII`）、`:358-375`（`word`）；`exp/tok/tok.go` 是 `:96-375` 的照抄，每个文件的正文的词与它相同才计数 |
| 版本 5 让任务重建入队的行（3.3.1） | `page/app/index_search.go:170-173`（名称、版本号、分词都相同才留）、`:211`（`SearchTokens`） |
| `page_search` 没有 `notebook_id`，`!cut` 要是键（3.2、3.3.3） | `00040_page_search.sql:7-25`；`00041_page_search_scope.sql:9-23`（前缀是 base 62 加 `#`，"No lexeme holds a '#'"）、`:35-46`（`page_search_queries` 引成操作数）、`:90-103`（`page_search_hits` 只读位图） |
| `tsvector` 放得下 `!cut` | `page/domain/search.go:53-56` 的注释（1,048,575）；M8/P5 3.3 的审查 A 的核对 |
| `WriteSearchRow`、`StaleSearchNodes`、`SearchBuilding`、`QueueSearch` | `page/adapter/postgres/queries/search.sql:5-12`、`:23-30`、`:88-99`、`:265-267` |
| 队列的索引 | `00040:65`（`(notebook_id, bulk)`）、`00041:62-63`（`(notebook_id, node_id) WHERE bulk`） |
| `Search` 的次序、层与游标（3.8） | `page/app/search.go:28-35`（`SearchHit` 没有层）、`:51-61`（`SearchDeps`）、`:87-130`（次序：`limit` 在工作区之后）、`:112-113`（`ErrWorkspaceNotFound`）、`:138-150`（快照）、`:181-184`（`tierRow`）、`:242-248`（游标）、`:353`（`ChangedAt` 就是 `SortAt`）；`page/app/ports.go:54-63`（`Notebooks.WorkspaceOf`）、`:855-860`（`SearchScope`）、`:894-921`（`SearchReads`）；`page/domain/search.go:420-430`（`q` 的 422）；`search_hits.go:29-42`（摘要）、`:169-197`（层与 `SearchCursor`） |
| 路径与 glob（3.7、3.9.1） | `page/app/node_paths.go:35-41`（`Find`）、`:56-58`（`Paths`）；`page/link_targets.go:97-100`（`NewLinkTargets` 的写法）；`page/adapter/postgres/store.go:140-160`（`PathsOf`）；`queries/links.sql:25-40`（`LinkTargetsByIDs`）；`page/domain/path.go:37`（`ParsePath`）、`:100-115`（`PathOf`、`PathOfNames`）；`page/domain/glob.go:79-123`、`:353-371`；`page/app/list_by_pattern.go:68-124`（次序）、`:126-149`（走与游标）、`:215-270`（`tree`）；`queries/nodes.sql:30-34`（`ListNodes`）、`:93-101`（`FirstChildren` 与 `parent_id IS NULL` 的注释） |
| 正文与页的状态（3.4.3） | `queries/contents.sql:40-49`（`PageStates`、`Contents`） |
| 判定 | `page/app/authorize.go:28`（`decideNotebook`）；`access/domain/rules.go:92-93`（`node.list`、`page.read`）、`:116`（`page.search`）、`:118-125`（linking 的读） |
| 名额照解析的预算（3.6.5） | `platform/markdown/budget.go:15-17`、`:47-101`；`shared/error.go:188-190`（`ServerBusy`） |
| 解析的队列等的时候 ctx 结束（3.6.1） | `platform/httpserver/mcpserver/errors.go:75-99`（`FaultDeadline` 答 `internal_error` "the call ran out of time"、warn；`FaultGone` 是"the client went away"、debug）；go1.27.1 的 `regexp/syntax/parse.go:107-121`（各个字符类各带一份范围）、`appendFoldedRange` |
| `nervewiki reindex` 的副作用（3.2.2） | `bootstrap/reindex.go:23-36`（每本笔记本之后把搜索的行缺的或分词不同的节点批量入队）、`queries/search.sql:23-30` |
| `index` 的属性与标签（3.9.3） | `platform/markdown/yaml.go:17-20`（`maxYAMLNodes` = 10,000）；`linking/adapter/markdown/facts.go:61-63`（frontmatter 的标签去掉开头的一个 `#`）；`platform/markdown/obsidian/tag.go:106-119`（标签里不收 `#`） |
| 矩阵的种子（3.13） | `bootstrap/permission_matrix_seeded_test.go:185-200`（`matrixContent`：任务的页的标记在第 3 行）、`:553-563`（`seededPageHistory`：每个没有删除的种下的页一行 `indexed_pages`）、`:721`（`queueSeeded`）；`permission_matrix_asset_test.go:37-42`（`markedIn`）；`permission_matrix_page_test.go:782-797`（`markedNodes`） |
| P1 的守卫树（6.1） | `page/domain/glob_cost_test.go:36-60`（`guardTree`）、`:163-203`（`TestTheWorstGlobsStopWithinTheirWork`）；`Makefile:239-258`（`test-go-costs`）；`.github/workflows/ci.yml` 的成本那一份 `timeout: 20` |
| `page.Deps` 的界线的写法 | `page/module.go:177-194`（"0 is the domain's"）；`bootstrap/trash_test.go:471-473` |
| 解析、`calcSize` 与 Go 的界线（3.6.1） | go1.27.1 的 `regexp/syntax/parse.go:94`（`maxHeight`）、`:103`（`maxSize`）、`:212`（`calcSize`）、`:436`（嵌套的重复至多 1,000）、`:1003`（`{m,n}` 至多 1,000）；与 GOROOT 1.26.2 的相同；`exp/cost` |
| `linking` 的反链（3.9.2） | `linking/app/backlinks.go:45-93`；`linking/app/access.go:21-36`（`PageTree.Paths` 是链接的路径）、`:47-57`（`page`）、`:62-64`（`notebook`）；`linking/domain/context.go:10-26`；`linking/adapter/postgres/queries/reads.sql:1-58` |
| `index` 的表与读（3.9.3） | `00020_linking_indexed_pages.sql`（`node_id` 主键、`notebook_id` 的索引）、`00022_linking_page_tags.sql`（主键 `(source_id, tag_key)`、`(notebook_id, tag_key)`）、`00023_linking_page_properties.sql:5-15`（主键 `(source_id, position)`、`value` 是 jsonb、只有 `notebook_id` 的索引）、`00026_linking_backlinks_index.sql`（`page_links_resolved_id_source_id_idx`）；`reads.sql:60-85`（`PageProperties`）、`:96-105`（`TagPages`）；`linking/app/tags.go:43-56`；`linking/adapter/markdown/tag.go:19`；`linking/domain/actions.go:7-27`；`linking/module.go:46-57`、`:73` |
| linking 的接线 | `bootstrap/deps.go:198-215`（`linkingDeps`）；`bootstrap/mcp.go:43-48`（`mcpTools` 不用 linking） |
| MCP 的适配器与契约 | `page/adapter/mcp/tools.go:101-129`、`log.go:31-62`（`Fit` 与逐项的游标）、`text.go:22-63`（`toolErr`、`notFound`）、`instructions_test.go:15-27`；`api/mcp/tools.json`（条目的形状与次序）；`bootstrap/mcp_tools_test.go:156`、`:175-205`；`bootstrap/permission_matrix_mcp_test.go:27-41`、`:79-104`、`:137-175`、`:332`、`:447`；`bootstrap/mcp_reads_test.go:403-450`、`:672-687`、`:715-760`；`bootstrap/actions_test.go:20`；`bootstrap/search_test.go:198`；`bootstrap/search_rebuild_test.go:20`；`archtest/rules_test.go:338-345` |
| 块数的测试（6.1） | `page/adapter/postgres/search_scope_test.go:49`、`:81-129`（每个事务的块数）、`:132-146`（GIN 的 ±2） |
| 说明的字节（3.14） | `exp/instr`（今天的两个入口 3,386 与 3,456）、`exp/instr6`（P4 的五行之后 3,957 与 4,027）；`mcpserver.TextBytes` 的数法照 `platform/httpserver/mcpserver/answer.go:25` |
| 工作的数法与守卫的模式（3.6.4） | go1.27.1 的 `regexp/exec.go:527`（输入短于模式的最短的匹配时立刻返回）；`exp/lineoh` |
| CI 的份 | `Makefile:179`（`-timeout 15m`）、`:189`（竞态的几份 `-j4`）、`:217-219`（两个表达式）、`:259-288`（守卫与成本的行）；[P3](03-P3-batch-revert.md) 6.5 与 9.3 的运行 |
| e2e | `e2e/fixtures/mcp.ts:12-14`、`:40-171`；`e2e/fixtures/links.ts:8-60`；`e2e/fixtures/search.ts:16`（`search(api, credential, slug, q, {notebookId})`）；`e2e/stories/mcp/mc3-write-edit.spec.ts:69-88`（`expectNewPage`、`expectChangesetOf`）；`e2e/stories/mcp/mc8-history.spec.ts`（比较 REST 的写法） |
