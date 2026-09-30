# M0/P1 技术验证：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M0/P1 技术验证 |
| 状态 | 已完成 |
| 基线 | `2ed0380`（只有文档，没有代码） |
| 上级文档 | [M0 总设计](00-M0-design.md) 第 7 节 |

---

## 1. 基线

仓库里只有设计文档。本 Phase 的实验代码写在仓库之外的临时目录中，用完即弃，不进入产品代码；进入仓库的只有结论（本文第 7 节）、复现材料（第 8 节）和 Markdown 样例集（`tools/md-fixtures/`，含自检与 Obsidian 核对工具）。

## 2. 目标与范围

在打地基（P2–P6）之前，回答五个会影响架构的问题。每一项的产出是一个明确的结论：**可行**、**不可行**，或者**可行但需要某种做法**，并写明对后续 Phase 或 M 的具体影响。

不做：任何产品代码；性能调优；超出"能否这样做、需要注意什么"的深入实现。

## 3. 设计：五项实验

### ① pg_trgm 与中文、数据库 locale、大小写折叠

**问题**：
1. 在哪种 locale 下，pg_trgm 能把中文切成三元组？
2. 1、2、3 个字和中英混合的查询，能否用上 GIN 索引？
3. 用不上索引时，ILIKE 顺序扫描的代价有多大？
4. 标题"不区分大小写唯一"应该用 `lower()` 还是 PostgreSQL 18 的 `casefold()`？各自受 locale 影响吗？

**做法**：
- 用 `postgres:18.6` 分别以三种方式初始化数据库：libc `en_US.UTF-8`（官方镜像的默认值）、builtin `C.UTF-8`、`C`。
- 在每个库里：
  - 用 `show_trgm()` 看中文、中英混合文本的切分；
  - 造 5 万行、平均约 3 KB 的中英混合正文，建 GIN（`gin_trgm_ops`）索引；
  - 对 1、2、3、4 个字以及中英混合的 `LIKE` / `ILIKE` 查询，看 `EXPLAIN ANALYZE` 的计划与耗时。
- 对比 `lower()`、`casefold()` 在德语 ß、土耳其语 İ、希腊语 Σ、全角字母上的结果。

**判定**：
- 能定出所有环境统一使用的 locale；
- 能给出"几个字以上走索引、几个字以下兜底"的明确界线，以及兜底的实测代价；
- 定出标题查重使用的函数。

### ② goldmark 与 remark 的提取一致性

**问题**：
1. 服务端（goldmark）与前端（remark）对同一段 Markdown，识别出的链接、嵌入、标签、frontmatter 是否一致？
2. goldmark 能否给出每个链接在原文中的精确字节范围，供 M6 改写链接使用？

**做法**：
- 编写约 30 个样例，覆盖以下边界：
  - 代码块与行内代码中的 `[[x]]`、转义的 `\[[x]]`；
  - 表格里的 `[[a\|b]]`；
  - `[[a|b]]`、`[[a#h]]`、`[[#h]]`、`![[img.png|100]]`；
  - 标题与列表中的链接；
  - URL 中的 `#`、`a#b`、`#123`、`#中文`、`#a/b`；
  - 注释 `%%[[x]]%%`；
  - HTML 块；
  - frontmatter 中的链接与写坏的 YAML；
  - 带空格的 Markdown 链接 `[t](<a b.md>)`；
  - 跨行的 `[[`。
- 每个样例写一份期望结果（以 Obsidian 的行为为准，Obsidian 行为不明确的由我们定义并写明）。
- Go 侧：goldmark + GFM + 候选的 wikilink 扩展；不满足时，评估自写 inline parser 的代价。
- TS 侧：unified + remark-parse + remark-gfm + remark-frontmatter + 候选的 wikilink 插件。
- 两边各写一个提取器，把结果与期望逐项对比。

**判定**：
- 得出一份差异清单，每条差异有处理办法（选定扩展，或者自写解析器）；
- 确认字节范围能拿到；
- 样例集的格式定稿，放进 `tools/md-fixtures/`。

### ③ 事务中的 NOTIFY → SSE

**问题**：
1. `NOTIFY` 是否只在提交后送达、回滚时不送达？
2. pgx 的 `LISTEN` 连接怎样管理，断线后怎样重连？
3. 浏览器能否用 `fetch` 流式读取带 `Authorization` 头的 SSE？
4. 经过 Caddy 时有没有缓冲？
5. HTTP 服务的 `WriteTimeout` 与请求期限中间件，对长连接有什么影响？
6. `NOTIFY` 的负载上限是多少？

**做法**：
- 一个最小的 Go 服务：
  - 写入接口在事务里调用 `pg_notify` 后提交，或者回滚；
  - 一条专用连接做 `LISTEN`，把事件分发给 SSE 连接；
  - SSE 接口校验 Bearer；
  - 用 `http.ResponseController` 处理写超时。
- 用 `pg_terminate_backend` 杀掉 `LISTEN` 连接，观察重连。
- 在真实的浏览器（Playwright 驱动的 Chromium）里用 `fetch` 读流；再在前面加一层 `caddy:2.10-alpine` 重复一遍。
- 超长负载单独测试。

**判定**：
- 提交与回滚的语义得到确认；
- 定出 SSE 路由对中间件的要求（期限、写超时、响应头）；
- 定出重连策略；
- 确认 Caddy 的默认配置能用，或者写出需要的配置。

### ④ CodeMirror 6 与 React 19

**问题**：
1. 在 `StrictMode` 下的挂载与卸载是否干净？
2. 外部更新文档（阅读时收到推送、切换页面）怎样做才不打断编辑？
3. 用 `Compartment` 动态切换只读（对应编辑锁）是否可行？
4. 扩展能否按"管线"组合（对应总体设计的编辑器扩展管线）？
5. 中文输入法的组合输入是否正常？
6. 打包体积有多大？

**做法**：
- 一个最小的 Vite + React 19 应用，引入 `@codemirror/{state,view,commands,language,lang-markdown,search,autocomplete}`。
- 用 Playwright 驱动 Chromium 验证挂载与卸载、外部更新、只读切换。
- 中文输入法：通过 Chrome DevTools Protocol 的 `Input.imeSetComposition` 与 `Input.insertText` 模拟组合输入，检查文档内容与光标位置。
- 构建后统计编辑器相关代码的体积（gzip）。

**判定**：
- 得出 React 集成的推荐写法；
- 给出外部更新与只读切换的做法；
- 输入法组合输入正常（模拟不可行时，列为 M4 的人工验证项）；
- 给出体积数据。

### ⑤ Go 的 MCP SDK

**问题**：
1. 官方 Go SDK（`github.com/modelcontextprotocol/go-sdk`）能否：
   - 把 Streamable HTTP 挂在我们自己的路由上；
   - 由我们的中间件校验 `Authorization` 头，并把当前账户传进工具处理函数；
   - 按 `?notebook=` 下发不同的 instructions；
   - 提供 prompts；
   - 在工具处理函数里拿到 clientInfo？
2. Claude Code 与 Codex 能否实际连上并调用工具？

**做法**：
- 一个最小的 Go 服务，用 SDK 暴露两个工具与一个 prompt，前面加 Bearer 中间件。
- 先用 SDK 自带的客户端做完整的协议测试；再用 `claude mcp add --transport http … --header "Authorization: Bearer …"` 与 Codex 的 MCP 配置实际连接，检查连接状态与工具列表。

**判定**：
- SDK 可用，或者给出替代方案；
- 定出挂载方式、认证接入方式、按请求区分笔记本的做法；
- 写出客户端配置样例。

## 4. 实施步骤

1. 在分支 `m0-p1-spikes` 上工作；实验代码放在仓库之外的临时目录。
2. 依次完成 ①、③、⑤、④、②，每完成一项，就把结论写进第 7 节。② 最费时，放在最后；它的样例集进入仓库。
3. 汇总结论对后续 Phase 与 M 的影响，修订 [M0 总设计](00-M0-design.md)、[v0.1 总体设计](../v0.1-design.md)，并在变更记录中写明。
4. 审查：由另一个独立的审查者核对每项结论的证据是否充分、是否有遗漏的风险、样例集的期望结果是否符合 Obsidian 的行为；修复审查发现的问题。
5. 合并回 `main`，推送。

## 5. 测试与验证

本 Phase 没有产品代码。验证的方式是：每项结论都附上可复现的证据（命令、查询计划、耗时、日志片段），写在第 7 节。样例集的期望结果由 Go 与 TS 两个提取器分别跑过；审查后改为与真实的 Obsidian 逐项核对（第 7 节 ②）。

## 6. 完成标准

- 五项实验都有结论与证据。
- `tools/md-fixtures/` 的格式与首批样例进入仓库。
- 受影响的上级文档已修订，变更记录已写明。
- 审查完成，发现的问题已修复。

## 7. 结果

实验在 2026-09-30 完成，实验代码已丢弃（复现材料见第 8 节）。下面每项给出结论、关键证据和影响；影响已经落实到[总体设计](../v0.1-design.md)（见其变更记录）和 [M0 总设计](00-M0-design.md)。

独立审查（[审查记录](reviews/P1-spikes-review.md)）没有推翻任何一项可行性结论，但纠正了 ① 的一处错误结论，并指出若干结论外推过度、样例集没有对照 Obsidian 验证。本节已按审查修订；修订的地方写明"审查后"。

### ① pg_trgm、数据库 locale 与大小写折叠

**结论：可行。** 数据库统一使用 builtin provider 的 `C.UTF-8`，**`LC_COLLATE` 与 `LC_CTYPE` 也必须是 `C.UTF-8`**。

**证据**（`postgres:18.6`）：

| locale | 中文三元组 | `casefold()` |
|---|---|---|
| libc `en_US.utf8`（官方镜像默认） | ✓ | 与 `lower()` 相同，没有完整折叠 |
| builtin `C.UTF-8`，`LC_CTYPE` 为 `C.UTF-8` | ✓ | 简单折叠 |
| builtin `C.UTF-8`，`LC_CTYPE` 为 `C`（审查后补测） | ✗ `show_trgm('中文搜索')` 为空，`Café Straße` 丢掉 é 和 ß | 简单折叠 |
| libc `C` | ✗ | 非 ASCII 字符不变 |
| builtin `PG_UNICODE_FAST` | ✓ | 完整折叠（`Straße` → `strasse`） |

审查后更正：pg_trgm 判断哪些字符是字母，靠的是 libc 的 `LC_CTYPE`，与 builtin provider 无关。原结论"不依赖 glibc"只对排序规则和大小写转换成立。另外实测了两种建库写法，结果都是 provider `b`、collate 与 ctype 均为 `C.UTF-8`：

- `CREATE DATABASE … TEMPLATE template0 ENCODING 'UTF8' LOCALE_PROVIDER builtin LOCALE 'C.UTF-8'`；
- 官方镜像设 `POSTGRES_INITDB_ARGS="--locale-provider=builtin --locale=C.UTF-8"`，之后用默认参数新建的库也继承这个设置。

5 万行、平均 2.5 KB 的中英混合正文（表 135 MB），关闭并行时的实测：

| 查询 | 计划 | 耗时 |
|---|---|---|
| 3 个字（罕见） | GIN 索引 | 0.4 ms |
| 罕见的 4 个字 / 中英混合 | GIN 索引 | 0.05 / 0.15 ms |
| 每一行都包含的词（`index`） | GIN 索引，复查 5 万行 | 约 220 ms |
| 1 或 2 个字 | 顺序扫描 | 约 250 ms（ILIKE 约 400 ms） |
| 只搜标题（5 万个），2 个字 | 顺序扫描 | 7 ms |
| 正则中没有 3 个字以上的字面片段 | 顺序扫描 | 约 220 ms |

索引构建 58 秒，GIN 索引 711 MB，是表的 5.3 倍。

**结论的适用范围**（审查后补充）：测试文本从 3000 个常用字里随机取字，三元组几乎不重复，所以"3 个字 0.4 ms"只代表罕见词；真实语料里常见的 3 字词要复查大量行，可能接近上表"每一行都包含的词"。2 个字是中文最常见的查询长度，却只能顺序扫描，耗时随数据量线性增长；工作区全局搜索跨多个笔记本，无法靠"限定笔记本"兜底。索引体积同样是随机文本下的最坏情况。

**影响**：
- 所有环境（compose、测试容器、持续集成、部署文档）按上面的写法建库。
- `nervewiki` 启动时（迁移之后）自检：编码为 UTF8、`datlocprovider = 'b'`、`datctype` 为 `C.UTF-8`、`show_trgm('中文')` 不为空，否则拒绝启动，并给出正确的建库命令。`pg_trgm` 扩展由第一条迁移创建。
- 三元组切分依赖 glibc：部署文档写明升级 glibc 大版本后对三元组索引 `REINDEX`（M12）。
- 标题"不区分大小写唯一"的键由应用计算：`NFC(fold(NFC(s)))`，存成单独的列，列上用 `COLLATE "C"`。唯一性因此不依赖数据库 locale，Go 侧链接解析用的也是同一个函数。
- 搜索方案在 M8 设计之前用真实中文语料复核；1–2 个字的查询不达标时，改用应用计算的 CJK 二元组列加 `tsvector('simple')` 的 GIN 索引（仍然只需要 PG）。

### ② Markdown 提取的一致性

**结论：两个解析器做不到"对任意输入都一致"。改为服务端（goldmark）作为 Markdown 语义的唯一权威：服务端负责提取，也负责渲染阅读视图，两者经过同一个解析入口 `Parse`。前端不做 Markdown 的语义解析。**

**实验证据**：
- 两边采用同一套算法：先用标准解析器求出排除区间，再在原文上扫描 wikilink、标签和注释。当时的 32 个样例在 Go 与 TS 两边都全部通过；故意改错两处期望值的反向对照，两边都准确报错。
- 500 篇随机拼出的对抗性文档做差分测试：第一轮只有 402/500 一致；去掉 GFM 字面自动链接、按 micromark 规则重写 Go 侧的数学公式之后是 487/500（97.4%）。审查后更正：剩下的差异不全来自病态结构，其中也有 Go 提取器自身的缺陷（引用定义用正则查找）。
- 样例 031 抓到一个隐蔽缺陷：frontmatter 如果直接交给 Markdown 解析器，YAML 里一行缩进的 ` ``` ` 会开启代码块，吞掉整个正文里的链接。解析前把 frontmatter 的字节替换成空格（保留换行），问题消失，偏移也不变。
- goldmark v1.8.6 的语法树里有 `LinkReferenceDefinition` 节点，引用定义的位置可以直接从语法树取得。
- 渲染的代价：goldmark 渲染（GFM + 脚注）加上 bluemonday 清洗，10 KB 的页面 0.37 ms，100 KB 的页面 3.5 ms（约 15 MB/s）。审查后补充：这个基准不含原始 HTML、代码高亮和链接解析，而且用 `UGCPolicy` 整页清洗会删掉任务复选框和代码块的 class，并不是最终的清洗方式（见总体设计 4.6）。

**审查后：样例集改为与真实的 Obsidian 核对。** 原样例集的期望结果来自我们对 Obsidian 行为的推测；两个提取器出自同一套算法，32/32 只能证明彼此自洽。审查后用 `tools/md-fixtures/obsidian/verify.mjs` 启动一个隔离的 Obsidian（独立的数据目录与临时库），通过 DevTools 协议读取它的 `metadataCache`，逐项比较：

- 74 个探针文件（另加原有的 32 个样例）逐一核对了审查提出的所有"待核实"项，结论改写了几条规则：
  - 数学公式改用 Obsidian 的规则：`$` 内侧不能是空白，结尾的 `$` 后面不能是数字。原来的 micromark 规则会把 `价格 $5 和 [[预算]] 以及 $10` 里的链接吞进公式。
  - 行内结构从左到右识别，先开始的优先：`[[Page]](2023)` 是 wikilink；``[[a `b` c]]`` 是 wikilink；`www.a.com/[[x]]` 是网址。原来的算法让 Markdown 链接优先，会把 `[[Page]]` 丢掉。
  - 注释只影响显示，不影响提取：Obsidian 照样索引 `%%` 里的链接和标签，否则重命名时这些链接会变成断链。
  - 标签：字符集加上组合用字符 `\p{M}`；`#` 位于一段文字的开头时也是标签，例如 `**#t**`、`[[a]]#t`。
  - 属性链接：只有整个值恰好是一个 wikilink 或一个 Markdown 链接的字符串才算，并记下属性路径（`sources.0`）；YAML 注释、嵌套列表、夹带文字的值都不算。
  - YAML 按 1.2 core schema：日期是字符串，`010` 是十进制。
- 结果：61 个样例，其中 52 个 `obsidian-verified`，与 Obsidian 1.12.7、1.13.7 完全一致；9 个 `nerve-defined`，是有意的偏离，每个都写明理由，例如：
  - 全角标点结束标签；
  - 目标不允许方括号；
  - 引用式链接照常索引。

  `verify.mjs` 报告的差异与这些理由逐条吻合。另外新增 4 个重命名改写样例（`rename/`）。
- `tools/md-fixtures/check.mjs` 自检每个 `range` 确实指向目标的原文写法；故意改坏三处的反向对照，三处都报错。

**影响**：
- 唯一的解析入口 `Parse(原文) → (语法树, 提取结果)`：wikilink、标签、注释等都是 goldmark 语法树上的节点，提取结果从语法树上取得。P1 用的"排除区间加原文扫描"只是实验手段，优先级与 Obsidian 不同，不再采用。
- 提取只依赖原文，在开启写事务之前完成。
- 阅读视图由服务端渲染：链接的解析状态按字节位置取自链接索引；用户 HTML 严格清洗，渲染器自己的标记在清洗之外生成，前端依据这些标记发起的操作由服务端核对（总体设计 4.3、4.6）。
- 前端去掉 unified / remark / rehype，只负责展示和交互增强；编辑器里的 lezer 只用于编辑辅助。
- `tools/md-fixtures/` 是规范：M6 的提取器必须通过全部样例；M4 与 M6 的渲染测试也使用它；修改样例时与 Obsidian 核对。

### ③ 事务中的 NOTIFY → SSE

**结论：可行。**

**证据**（Go + pgx v5.11.0；直连与经 `caddy:2.10-alpine` 的默认反向代理结果相同）：
- 事务调用 `pg_notify` 之后等待 300 ms 再提交，事件在提交后约 2 ms 送达；回滚的事务不送达；别的笔记本的事件不送达。
- 服务端 `WriteTimeout` 为 4 秒时，对这条连接调用 `http.ResponseController.SetWriteDeadline(time.Time{})`，空闲 6 秒之后连接依然可用。
- 用 `pg_terminate_backend` 杀掉 `LISTEN` 连接：自动重连并广播 `reset` 事件，之后的事件照常送达。
- 负载接近 8000 字节时，PostgreSQL 报 `payload string too long`。
- 对照路由加了 3 秒的请求期限：连接在 3005 ms 时被切断。
- 真实的 Chromium 用 `fetch` 带 `Authorization` 头流式读取，只收到了应该收到的那一个事件。
- Caddy 默认配置对 `text/event-stream` 不缓冲，日志里的 `Authorization` 已脱敏。

**实验没有覆盖的**（审查后补充）：
- 连接只在建立时校验令牌，之后令牌过期、成员被移出，流仍在推送。
- HTTP/1.1 下每个源最多 6 条连接，每个标签页、每个笔记本各一条流会把连接耗尽。
- 带 `NOTIFY` 的提交在全局队列锁上串行。
- Caddy 常见的 `encode` 压缩配置没有测。

**影响**：
- P3 的 HTTP 平台层要允许个别路由豁免请求期限，并在连接上解除写超时。
- 事件负载只带标识；`LISTEN` 断线重连后广播 `reset`，客户端据此整体刷新；每 15–30 秒发一次心跳注释，响应头带 `X-Accel-Buffering: no`。
- 审查后的设计（总体设计 3.11、12.4）：
  - 每个浏览器一条面向账户的事件流，由一个标签页持有，其他标签页经 `BroadcastChannel` 共享；
  - 令牌到期时关闭连接；M3 的可见性变化事件触发关闭相关账户的连接；
  - 每个事务最多一条 `NOTIFY`。
- Caddy 的 `encode` 与其他反向代理由 M5 实测（[handoff](../M5-collab-editing/handoffs/M0-P1-sse-proxies.md)）。

### ④ CodeMirror 6 与 React 19

**结论：可行。**

**证据**（React 19.3、Vite 8.3、`@codemirror/view` 6.43，由 Playwright 驱动 Chromium）：

推荐的写法：`EditorView` 在 `useEffect([])` 中只创建一次，在清理函数中销毁；之后的属性变化都通过事务传进去：
- 只读：`Compartment.reconfigure`；
- 外部文档：只替换差异部分；
- 回调：放在 ref 里；变更监听只上报用户事件。

验证结果：

| 检查 | 结果 |
|---|---|
| StrictMode | 挂载 2 次、销毁 1 次，DOM 中始终只有一个编辑器；卸载后为 0 |
| 外部更新 | 在开头插入 8 个字符，光标从 30 正确映射到 38，没有误触发变更回调 |
| 只读切换 | `contenteditable=false`，无法输入；切回之后可以输入 |
| 中文输入法 | 用 CDP 模拟拼音组合输入，确认后得到"你好"，后续输入正常（只在 Chromium 上模拟） |
| 打包体积 | 编辑器单独分包 528 KB（gzip 183 KB） |

两点需要注意：
- 组合输入过程中，拼音会临时出现在文档里，每一步都触发文档变更。
- 编辑器分包的体积主要来自 `lang-markdown` 静态依赖的 `lang-html`，后者又带进 CSS 和 JavaScript 的语言包。

**审查后补充**：
- "只替换差异部分"只适用于同一个页面的外部更新。切换页面也这样做的话，`Ctrl+Z` 会把上一个页面的内容撤销进来，再被自动保存。
- 输入法只在 Chromium 上用 CDP 模拟过，Safari、Firefox 与真实输入法都没有测。
- 问题 4"扩展能否按管线组合"没有给出答案。

**影响**：
- 切换页面时新建 `EditorState`；同一页面的外部替换标记为不进撤销历史（总体设计 9.3）。
- M4 的自动保存在 `view.composing` 为真时跳过，组合结束后补一次保存（总体设计 3.9）。
- 编辑器只在进入编辑模式时按需加载。
- 真实输入法的人工验证、扩展管线的组合、去掉 `lang-html` 依赖的评估，移交 M4（[handoff](../M4-pages/handoffs/M0-P1-editor.md)）。

### ⑤ Go 的 MCP SDK

**结论：可行。** 采用官方 SDK `github.com/modelcontextprotocol/go-sdk` v1.8.0（协议版本 2025-11-25）。

**证据**：
- `mcp.NewStreamableHTTPHandler` 可以挂在我们自己的路由上，外面套我们的中间件和 `auth.RequireBearerToken`。
- 工具处理函数里拿得到 `TokenInfo.UserID`，也拿得到我们中间件放进 context 的值；clientInfo 通过 `InitializeParams()` 取得。
- 按 `?notebook=` 返回不同的 server，就能下发不同的 instructions。
- 泛型的 `mcp.AddTool` 能自动推导输入 schema；处理函数返回错误时，结果标记为 `isError`，错误文字对 agent 可读。prompts 可用。
- 不带令牌返回 401；拿别人的会话 id 冒用返回 403（SDK 把会话绑定到 `UserID`）。
- `getServer` 回调**每个请求都会被调用**（每个请求一开始都要用它校验协议版本）。
- SDK 默认开启防 DNS 重绑定保护：服务监听在回环地址、`Host` 头不是回环地址时返回 403。这会挡住同一台机器上的反向代理转发过来的请求。
- Claude Code 2.1.285 通过 `.mcp.json` 的 `headers` 连接成功，`claude -p` 实际调用了工具，也收到了 instructions。
- Codex 0.157.1 用 `bearer_token_env_var` 连接成功；它的非交互模式默认拒绝需要审批的工具，要把服务器的 `default_tools_approval_mode` 设为 `approve`（可选值 `auto` / `prompt` / `writes` / `approve`）。

**实验没有覆盖的**（审查后补充）：
- 会话的生命周期：`SessionTimeout` 为零值时空闲会话永不关闭；会话存在进程内，重启后客户端能否自动重新握手没有测。
- 缓存的 server 怎样回收。

**影响**：
- `getServer` 必须廉价、结果确定：按"笔记本 + AGENTS 页面的版本"缓存 server，按 LRU 回收；`?notebook=` 的权限每个请求都判定，而且在查缓存之前。
- 设置 `DisableLocalhostProtection: true`，由 Bearer 认证把关；设置 `SessionTimeout`。
- 认证接入复用平台层的 Authenticator，通过 SDK 的校验函数交给 SDK。
- 工具带上 `readOnlyHint` / `destructiveHint` 注解；接入文档说明 Codex 各审批选项的含义，不一律推荐 `approve`。
- P3 的路由要能挂载 MCP 这样的长连接处理器（与 ③ 的要求相同）。
- 重启后的恢复与无状态模式的评估移交 M9（[handoff](../M9-mcp/handoffs/M0-P1-mcp-sessions.md)）。

## 8. 复现材料

实验代码已丢弃，以下两份材料保留，供 M6 的模糊测试和 M8 的搜索实测复用。

**① 的造数 SQL**（每个库先 `CREATE EXTENSION pg_trgm`）：

```sql
create table docs(id int primary key, body text not null);
insert into docs
select g, (select string_agg(case when random() < 0.03 then ' wiki ' when random() < 0.02 then ' Index '
                                  else chr(19968 + floor(random()*3000)::int) end, '')
           from generate_series(1, 800) where g = g)
from generate_series(1, 50000) g;
update docs set body = body || ' 量子纠缠实验记录 ' where id % 5000 = 0;
update docs set body = body || ' nervewiki知识库 ' where id % 10000 = 0;
create index docs_body_trgm_idx on docs using gin (body gin_trgm_ops);
analyze docs;
set max_parallel_workers_per_gather = 0;
explain analyze select id from docs where body like '%量子纠%';
```

**② 的对抗性文档生成器**（Python，种子 `20260930`，500 篇）：

```python
random.seed(20260930)
toks = ["[[a]]", "![[b.png]]", "[[c|显示]]", "[[e#f]]", "[[g\\|h]]", "`", "``", "$", "$$", "%%", "#t", "#中文", "#1",
        "[t](x.md)", "[t](<y z.md>)", "![i](p.png)", "[r]", "[r][]", "[^1]", "\n", "\n", "\n\n", "\n\n", "> ", "- ",
        "1. ", "```\n", "~~~\n", "    ", "<div>", "</div>", "<span>", "\\", "|", " | ", "---", " ", "中文", "==", "**",
        "_", "http://x.com/#y", "www.a.com", "[r]: r.md\n", "[^1]: n [[fn]]\n", "\r\n", "[[", "]]", "[", "]", "(", ")",
        "#", "!", "😀"]
for k in range(500):
    doc = "".join(random.choice(toks) for _ in range(random.randint(5, 60)))
    if random.random() < 0.2:
        doc = "---\ntags: [a]\nx: \"[[p]]\"\n---\n" + doc
```
