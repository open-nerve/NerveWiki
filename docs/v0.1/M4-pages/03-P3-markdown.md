# M4/P3 Markdown 解析与渲染：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P3 Markdown 解析与渲染 |
| 状态 | 已完成（`3467491` 合并，审查见 [P3 审查](reviews/P3-markdown-review.md)） |
| 基线 | `b75a547`（P2 合并、文档提交之后的 main）；本文与各 Step 计划提交之后开分支 |
| 上级文档 | [M4 总设计](00-M4-design.md) 第 1、4、8、9 节；[总体设计](../v0.1-design.md) 4.1–4.3、4.6、8.2、13.1 第 10–12、15、21 条、13.3；[样例集](../../../tools/md-fixtures/README.md) |

---

## 1. 基线

P2 留下的：page 模块有树的全部写（新建、改名、移动、删除子树）与 `listNodes`、`getPage`；正文表 `page_contents` 已在，但正文只能是空串（`createPage` 不带正文，写正文的接口在 P4）。

仓库里还没有 Markdown 的代码：`server/go.mod` 没有 goldmark、`golang.org/x/net`；YAML 库 `go.yaml.in/yaml/v3` 只经 koanf 间接引入。样例集 `tools/md-fixtures/` 有 61 个样例，期望只有提取结果（frontmatter、链接、标签），没有 HTML；Go 侧没有代码读它。

开工时的实测（本机 darwin/arm64，单次墙钟，探针在仓库之外）：

1. **goldmark v1.8.6（最新版）在病态输入上是平方级的**。普通文档约 20 MB/s；下表的输入只有 100–200 KB，就要几百毫秒到十秒以上：

   | 输入（一个段落里重复） | 原因 | 原版 goldmark | 加固之后（原型） |
   |---|---|---|---|
   | `a*_`、`*a_ `、`a**b` 加 `c* `、`a***`、`~~a`、`a~` | 强调的配对没有 CommonMark 的 `openers_bottom`，每个闭合符向前扫完全部分隔符 | 100 KB：3.3–6.7 秒 | 200 KB：20–50 ms |
   | `[*a*](b) ` | 链接闭合时处理强调，从最后一个分隔符沿兄弟节点走回一个已被移走的"底" | 200 KB：3.6 秒 | 23 ms |
   | `` `a ``、`_www`（任何行内节点之后紧跟长串） | GFM 自动链接在每个行内节点之后把电子邮件的本地部分扫到这串字符的末尾 | 200 KB：1.2–1.3 秒 | 16–21 ms |
   | `[a](`、`![a](`、`[a](<b` | 链接目标扫到行尾：括号不限嵌套，`<…>` 不在 `<` 处停 | 100 KB：3.5 秒；200 KB 超过 10 秒 | 18–23 ms |
   | `<!--`、`<?`、`<!A` 不闭合 | 原始 HTML 每次向后找结束符扫到块尾 | 200 KB：1.4–2.6 秒 | 7–13 ms |
   | `> > > …`、`- - - …`（一行） | 容器块的嵌套没有上限，每一层的处理与层数成正比 | 100–200 KB：3.7–4.9 秒 | 3–8 ms |
   | `[a]` 每行一个 | 链接标签取值时从块的最后一行往回找起始行 | 266 KB：0.8 秒 | 31 ms |
   | 链接引用定义、脚注各几万个 | 每条定义之后复制余下的行；脚注引用逐个比对定义，收尾时的排序是平方的 | 200 KB：60–360 ms，仍按平方增长 | 待 S1 |

   原型（仓库之外）替换了强调与删除线、链接，加了代码段、原始 HTML、自动链接的前置检查与容器的嵌套上限：CommonMark 规范的 652 个例子与 goldmark 自带的扩展例子，通过与失败的集合与原版完全相同；20 万个随机输入与原版逐字节一致。
2. **YAML**：`go.yaml.in/yaml/v3` 解析成节点树是线性的（各种病态输入约 20 MB/s）；直接 `Unmarshal` 到 `any` 过不了样例 055（日期变成时间、`010` 变成 8，`.inf` 变成无法写进 JSON 的浮点），要自己按 YAML 1.2 core schema 解析标量。
3. **chroma 过不了耗时的硬约束**：普通代码 0.2–1 MB/s；它的正则会回溯，`"` 后跟一串反斜杠在 go、js、ts、bash、yaml、json、css 等词法器里单个匹配就要约 400 ms（卡在 chroma 写死的 250 ms 超时上），几十字节就能触发，按字节限长无效。**负责人确认（2026-10-02）：代码高亮改在前端**，P5 的阅读视图交互增强里用 highlight.js 在 Web Worker 中高亮，超时即终止；服务端只给代码块带上语言的 class（第 3.6 节，M4 总设计第 4、11、13 节同步修订）。
4. **bluemonday 不配平、不挡 `//host`、保留 `img`**（UGC 策略实测）：补闭合、按主机判断外站、不放行 `img` 都要另写，余下的只是"分词 + 查表"，第 3.7 节改用自己的白名单。

## 2. 目标与范围

**目标**：平台包 `platform/markdown` 成为 Markdown 唯一的解析入口与渲染器：`Parse` 先把 frontmatter 换成空白再解析，渲染出安全的 HTML；任何输入的耗时与字节数成比例；扩展点能挂上解析、提取、按页取数据与渲染。页面经端口用它提供阅读视图。

**做**：

- `Parse`：frontmatter 的边界与 YAML（1.2 core schema、别名展开、节点数与深度的上限）；BOM 与 frontmatter 换成空白，偏移不变；CommonMark + GFM（表格、任务列表、删除线、自动链接）+ 脚注；扩展的解析器与提取。
- 病态输入的加固（第 3.4 节）：结果与原版 goldmark 相同，只多两条上限（审查之后是四条，见 3.4）。
- `Render`：扩展按页取数据；渲染器自己的标记（标题与脚注的 id、任务项、表格的对齐、代码块的语言、属性表、图片为链接）；用户 HTML 逐节点清洗并补闭合；所有地址经同一份白名单。
- 测试辅助 `markdowntest`：最终 HTML 的不变量、普通文档与病态输入的生成器。
- 耗时：基准、相对耗时的测试、模糊测试的入口。
- 页面：`getPageView`（契约、用例、端口与适配器、读正文的查询）；组合根建一个 `markdown` 实例交给页面（扩展为空集合）；架构规则；矩阵一行。

**不做**：

- 代码高亮与它的样式表：P5 的交互增强（第 1 节第 3 条）。
- Obsidian 方言（wikilink、嵌入、标签、callout、`==高亮==`、注释、数学）：M6，`[[…]]` 照普通文字渲染；附件图片：M7。
- 写正文与它之前的取值检查（P4）；端到端测试（PG5、PG6 的接口版本随 P4，页面版本随 P5）；前端（P5）。
- 不改样例集：README 规则 1 写了而样例没有覆盖的（`.inf`、`.nan`、别名、合并键）由 Go 的表格测试照规则的文字覆盖，加样例要与 Obsidian 核对（M6 开工时重核样例集，总体设计第 14 节）。

## 3. 设计

### 3.1 文件

```
server/internal/platform/markdown/
  markdown.go          包注释；Markdown、New、Extension、Markup、Page、Document
  parse.go             Parse：空白化、解析、扩展的提取；标题 id 的转换
  frontmatter.go       frontmatter 的边界（样例集规则 1）与 Frontmatter、Property
  yaml.go              YAML 1.2 core schema 的标量、别名展开、节点数与深度与别名重复字节数的上限
  render.go            Render：按页取数据、组装渲染器、属性表
  marks.go             渲染器的标记：链接与自动链接、图片为链接、代码块（标题 id 在 parse.go）
  sanitize.go          用户 HTML 的逐节点清洗与补闭合
  inlinetag.go         行内原始 HTML 的标签读取（属性值按分词器在属性里的规则解引用）
  url.go               SafeURL：地址的白名单
  internal/harden/     病态输入的加固：goldmark 的解析器与替换件
    parser.go          组装解析器（默认件、替换件、包装件、上限）
    emphasis.go        强调与删除线：分隔符的游程与按作用域的线性配对
    links.go           链接与图片（改编自 goldmark，结果不变、开销线性）
    refdefs.go         链接引用定义的段落转换（删行在链表上）
    tables.go          表格：单元格数的上限，代码段里转义竖线的转换
    footnotes.go       脚注的定义、引用与收尾的转换
    guards.go          代码段、原始 HTML、自动链接的前置检查；容器与脚注定义的嵌套上限
  markdowntest/        只给测试用：CheckHTML、CheckSize（最终 HTML 的不变量与字节数）、CheckCosts（耗时与分配）、
                       Normal、Pathological、Amplifying（输入的生成器）、Fixtures（样例集）；race.go、norace.go
server/internal/modules/page/
  app/ports.go         Markdown（Parse、Render）、Parsed、PageRef；Nodes.PageContent
  app/get_page_view.go GetPageView、ReadingView
  adapter/markdown/    platform/markdown 接到 app.Markdown（包名 markdownadapter）
  adapter/postgres/queries/contents.sql   PageContent
  adapter/http/handler.go                 getPageView
  module.go            Deps.Markdown
server/internal/bootstrap/
  wire.go、deps.go     markdown.New(markdownExtensions()) 建一个实例交给 page
  registrants.go       markdownExtensions()：M4 为空（M5 任务项、M6 方言、M7 附件）
server/internal/archtest/   规则"Markdown 的第三方库只许 platform/markdown 导入"；markdowntest 登记为测试辅助
api/modules/page.yaml、api/openapi.yaml   getPageView、PageView
Makefile               相对耗时的测试在不带竞态检测的构建里另跑一次
```

测试另在各包的 `_test.go`、`bootstrap/permission_matrix_page_test.go`（一行）、`page_visibility_test.go`（阅读视图与逐项读取一致）、`markdown_app_test.go`（应用的实例过样例集与生成的输入，并跑 `CheckCosts`）、模块根的 `extension_test.go`。

### 3.2 `Parse` 与 `Document`

```go
func New(exts []Extension) (*Markdown, error)          // 扩展名重复时报错
func (m *Markdown) Parse(content []byte) *Document      // 任何字节都能解析，不返回错误
func (m *Markdown) Render(ctx context.Context, doc *Document, page Page) (string, error)

type Page struct{ NotebookID, PageID uuid.UUID }
func (d *Document) Frontmatter() Frontmatter
func (d *Document) Extracted(name string) any           // 扩展的提取结果
```

`Parse` 依次：

1. **frontmatter 的边界**照样例集规则 1，与 `check.mjs` 的 `frontmatterSpan` 相同：跳过开头的 BOM，第一行恰为 `---`（`\n` 或 `\r\n` 结尾），到下一个去掉 `\r` 后恰为 `---` 的行（含它的换行）为止；没有结束行就不是 frontmatter。
2. **解析用的副本**：frontmatter 的字节换成空格，`\r`、`\n` 保留；开头的 BOM（有没有 frontmatter 都一样）换成三个换行：goldmark 把 BOM 当作文字（`﻿# 标题` 会成为段落），换成空格又会让第一行多出缩进（缩进代码多出三个空格），文档开头的空行则不影响解析。长度与偏移都不变（总体设计 4.3）。
3. **YAML**：第 3.3 节。
4. **goldmark**（第 3.4 节的加固配置加扩展的解析器）解析副本；标题的自动 id 用每篇文档自己的生成器（第 3.6 节）。语法树里的文字段落指向原文，frontmatter 的范围不被任何节点引用。
5. **提取**：每个扩展的 `Extract(root, content)`，`content` 是原文、逐字节，语法树的偏移就是它的偏移；结果按扩展名存进 `Document`。

`Document` 带原文、解析用的副本（`Render` 从它渲染：BOM 与 frontmatter 的位置上只有空行与空格，没有节点引用）、语法树、`Frontmatter`、提取结果。它只在一次请求里用：`Render` 会改它的语法树（第 3.7 节把原始 HTML 换成清洗过的节点）。

### 3.3 frontmatter 与 YAML

```go
type Frontmatter struct {
	Present    bool       // 有 frontmatter 这一段（写坏了也算）
	Valid      bool       // 解析成映射且没有超出上限
	Properties []Property // 按原文的键序
}
type Property struct {
	Key   string
	Value any // nil、bool、int64、float64、string、[]any、[]Property
}
```

- 解析成 `yaml.Node`（`go.yaml.in/yaml/v3` 由间接依赖改为直接），自己遍历：
  - 不带引号的标量按 1.2 core schema：`~`、`null`、`Null`、`NULL` 与空值为 null；`true`/`True`/`TRUE`、`false`/`False`/`FALSE` 为布尔；`[-+]?[0-9]+`（十进制，`010` 是 10）、`0o[0-7]+`、`0x[0-9a-fA-F]+` 为整数，超出 int64 的按浮点，再超出浮点的保留原文；浮点按 core 的式子；`.inf`、`-.inf`、`.nan` 这类 JSON 写不出的值保留原文的字符串（规则 1）；其余是字符串（`yes`、日期、时间戳）。带引号的与块标量都是字符串。
  - 显式标签只认 core 的（`!!str`、`!!int`、`!!float`、`!!bool`、`!!null`、`!!seq`、`!!map`），别的标签按解析失败；`!!float` 也收十进制的整数（`!!float 1` 是 1）。已知差异：非特定标签 `! 10` 应是字符串，go-yaml 解析时丢掉这个标签，读出来是整数 10。
  - 键必须是标量，取它解析后的文字形式（`010:` 的键是 `10`；数字照 JavaScript 的写法，`1e6:` 的键是 `1000000`）；重复的键与非标量的键按解析失败；`<<` 是普通的键（1.2 core 没有合并）。
  - 别名展开：遍历时跟进锚点，展开之后的节点数超过 10 000 或嵌套深度超过 64 按解析失败；经别名重复的键与标量累计超过 YAML 的字节数与 100 000 中较大者也按解析失败（节点数挡不住一个长值被别名重复几千次，属性表会把它写几千遍），只写一次的不计。`yaml.v3` 解码成节点树时不展开别名，也不做它在 `Unmarshal` 里的"过多别名"检查，上限只在我们的遍历里。
- 内容为空（含只有注释）是没有属性的有效 frontmatter（样例 058）；不是映射（序列、标量、null）或解析失败为 `Valid=false`（样例 018、057）。YAML 库的错误信息可能带原文，不往外传（13.1 第 10 条）：`Frontmatter` 只有 `Valid`。

### 3.4 病态输入的加固

`internal/harden` 用 goldmark 公开的扩展接口组装解析器：块的解析与语法树、渲染都是 goldmark 的，开销会随输入平方增长的几处换成自己的实现或加前置检查，会让语法树（从而 HTML）比输入增长得快的两处设预算。

| 部分 | 做法 | 与原版的差别 |
|---|---|---|
| 强调与删除线 | 行内解析器只记下分隔符的游程（`*`、`_`、`~`，左右侧的判定用 goldmark 导出的 `ScanDelimiter`），不进 goldmark 的分隔符表；解析之后一个语法树转换按作用域（块、链接、图片的子节点）用 CommonMark 规范附录的算法配对，`openers_bottom` 按（字符、闭合符能否开启、原长模 3）记；配对的规则（同字符、"3 的倍数"、一次用掉 1 或 2 个）照 goldmark | 无 |
| 链接与图片 | 改编自 goldmark 的 `parser/link.go`：括号的状态表、引用式与内联式、标签不超过 999 字等照旧；内联链接的目标里括号嵌套超过 32 层即不成链接（CommonMark 允许实现限制）；`<…>` 的结束位置查块内的索引；标签取值时二分查找起始行；"链接里不能有链接"用块内已成链接的计数代替逐个遍历。引用式链接（完整、折叠、简写）按上下文累计重复的目标与标题的字节数，超过原文字节数与 100 000 中较大者之后的引用当作没有定义（cmark 的做法：引用每用一次就把定义写一遍，几十 KB 的正文能写出 GB 的 HTML） | 括号嵌套超过 32 层的内联目标；超出展开预算的引用是文字 |
| 链接引用定义 | 段落转换一次读完开头的全部定义；删去定义的行照 goldmark 的下标算法（定义之间隔着行时，它保留的行与直觉不同），在链表上做，走到删除处的步数之和不超过最后一条定义的结束行；目标不设括号上限（每段只读一次） | 无 |
| 表格 | 包住 goldmark 的表格段落转换：goldmark 把每一行补到表头的列数，宽表头下的短行是字节数平方的单元格；（行数+1）×列数超过这段表格的字节数就不当表格（cmark-gfm 同样限制补齐）。语法树转换改编自 goldmark：代码段里的 `\|` 只查本单元格的位置（原版查全文所有单元格的） | 单元格多于字节数的表格是段落 |
| 脚注 | 定义、引用、收尾的转换改编自 goldmark 的脚注扩展：引用按标签查表，收尾按序号排序一次；语法树节点与渲染器仍用 goldmark 的；定义计入容器的嵌套（最后一行） | 无 |
| 代码段 | 包在 goldmark 的解析器之前：块内每种长度的反引号串最后出现的位置；后面没有同长的串，这一串直接是文字 | 无 |
| 原始 HTML | 同上，`<!--`、`<?`、`<!X`、`<![CDATA[` 后面没有各自的结束符时 `<` 直接是文字；排在尖括号自动链接之后 | 无 |
| 自动链接（GFM） | 包住 goldmark 的 linkify：它在每个行内节点之后读可能从那里开始的电子邮件地址，读到本地部分那串字符的末尾与 `@` 之后；块的索引给每串字符记下 goldmark 能否把它做成链接（有 `@` 与域名、域名在末字符前有 `.`、其后不是 `-`/`_`），不能就不调用。`www.` 开头时按 goldmark 的式子手工判断（式子的 256 次重复让每次失败都慢），不成立同样走上面的判断。在链接标签里不调用（原版靠 goldmark 链接解析器的状态） | 无 |
| 容器的嵌套 | 包住引用块、列表、列表项与脚注定义的块解析器：已在 32 层（引用块、列表项、脚注定义合计）之内就不再开新的一层，这一行按文字 | 超过 32 层的引用、列表、脚注定义 |

- **核对**：CommonMark 规范的例子与 goldmark 的扩展例子（`spec.json`、`extension/_test/*.txt`，测试时从模块缓存里 goldmark 的目录读，不拷进仓库）在加固配置与原版配置上的输出逐字节相同；固定种子生成的随机输入（Markdown 的各种记号拼接，嵌套不到上限）两边逐字节相同；各部分的边界另列成表与原版对照；四条上限各有恰在上限（与原版相同）与超出一个的测试。第 1 节表格与审查找到的各类在耗时测试里（`markdowntest.Pathological`），放大类在 16 KB 配输出字节数的检查（`Amplifying`、`CheckSize`，第 3.10 节）。
- **许可**：改编自 goldmark 的文件在开头写明来源与 MIT 许可、版权人。
- **对扩展的约束**（`Extension` 的注释）：分隔符式的语法（M6 的 `==高亮==`）走这里的游程与配对，不进 goldmark 的分隔符表（加固之后没有人处理它）；goldmark 的 `Context.IsInLinkLabel` 在加固之后恒为假，扩展做的链接不计入"链接里不能有链接"（M6 需要时由 `platform/markdown` 导出入口）；扩展自己的解析器要是线性的，耗时与输出的检查与每个注册的扩展一起跑（第 3.10 节）；扩展做出的每种节点都要有渲染函数（goldmark 的渲染器按种类的下标取函数，比登记过的都新的种类会 panic）。

### 3.5 扩展

```go
type Extension struct {
	Name     string                                   // 提取结果的键，不能重复
	Parser   []parser.Option                          // goldmark 的解析器、段落与语法树的转换
	Extract  func(root ast.Node, content []byte) any  // 从语法树取提取结果（M6 的链接、标签带字节位置；content 是原文）
	Fetch    func(ctx context.Context, page Page, extracted any) (any, error) // 渲染之前按页取数据，可为空
	Renderer func(data any) []util.PrioritizedValue  // 渲染器，拿到自己 Fetch 的结果
	Markup   Markup                                   // 它的渲染器会输出的元素、属性与 class，给最终 HTML 的不变量
}
```

- `New` 收组合根交来的扩展，页面与 M6 的 linking 用同一个实例（M4 总设计第 8 节）。
- `Fetch` 在 `Render` 里、渲染之前按扩展的顺序调用，在调用方的读取里执行、不加锁；结果只交给它自己的 `Renderer`。出错时 `Render` 返回错误（页面答 500）。
- goldmark 的渲染器在每次 `Render` 时组装：基础渲染器、本包的标记、扩展的渲染器（绑定这次的数据）；组装只是登记函数，开销可以忽略。
- 扩展的标记不经用户 HTML 的清洗；地址必须经 `SafeURL`（导出）；最终 HTML 的不变量按它的 `Markup` 检查。
- 扩展的注册者要导入 goldmark，M6 起在架构规则里放行它们的适配器（第 3.11 节）。

### 3.6 渲染器的标记

| 节点 | 输出 |
|---|---|
| 标题 | `id="nw-<slug>"`：标题文字转小写，保留 Unicode 字母、数字与组合符，空白、`-`、`_` 合成一个 `-`，其余去掉，最长 64 个字符，空的取 `section`；重复的加 `-1`、`-2`……；文字取自解析时的全部文字，含清洗之后看不到的（被丢弃元素之间的）文字。不开 goldmark 的属性语法（`{#id .class}`） |
| 脚注 | goldmark 的脚注渲染器，id 前缀 `nw-`（`nw-fn:1`、`nw-fnref:1`），class `footnote-ref`、`footnote-backref`、`footnotes` |
| 任务项 | `<input type="checkbox" disabled>`（勾选的带 `checked`），M5 再加字节位置 |
| 表格 | 对齐用 `align` 属性，不用 `style` |
| 代码块 | `<pre><code class="language-x">`：`x` 是信息串的第一个词转小写，符合 `[a-z0-9_+#.-]{1,32}` 才输出，否则只有 `<pre><code>`；高亮在前端 |
| 链接、自动链接 | `href` 经 `SafeURL`（取 goldmark 解出转义与实体之后的地址）；不通过时只输出链接文字。电子邮件的自动链接加 `mailto:`，`www.` 开头的加 `http://`，与 goldmark 相同 |
| 图片 | 从不输出 `<img>`：`<span class="nw-image">替代文字 <a href="地址">地址</a></span>`；地址不通过时只有替代文字。外站、相对地址都这样（M7 的附件另行内联） |
| 属性表 | frontmatter 有效且有属性时在正文之前：`<table class="nw-props">`，每个属性一行（`th` 键、`td` 值）；列表是 `<ul>`，嵌套的映射是嵌套的属性表，null 为空，数字照 JavaScript 的写法（1e-6 到 1e21 之间十进制，其外带指数；-0 写 0） |

goldmark 的 HTML 渲染器保持安全模式（不开 `WithUnsafe`）：万一有节点漏到它的默认渲染，原始 HTML 输出为省略的注释、危险的地址被去掉。

### 3.7 用户 HTML 的清洗

- **不用 bluemonday**（第 1 节第 4 条），自己的白名单：每个原始 HTML 节点单独读，按词重新输出（文字与属性值重新转义），从不把原文直接写出。HTML 块用 `golang.org/x/net/html` 的分词器；行内 HTML 是 goldmark 按 CommonMark 的语法给出的一个标签（或注释、处理指令……），用自己的读取（每个节点一个分词器要 4 KB 与一个 map，几千个标签就是几百 MB）。属性值都按分词器在属性里的规则解引用（`&section=` 不解）。
- `Render` 先遍历一遍语法树，把原始 HTML 节点（HTML 块、行内 HTML）换成清洗过的节点，再渲染。
- **白名单**（排版类）：
  - 行内：`a`、`abbr`、`b`、`bdi`、`bdo`、`br`、`cite`、`code`、`del`、`dfn`、`em`、`i`、`ins`、`kbd`、`mark`、`q`、`rp`、`rt`、`ruby`、`s`、`samp`、`small`、`span`、`strong`、`sub`、`sup`、`u`、`var`、`wbr`。
  - 块级（只在 HTML 块里）：另有 `blockquote`、`caption`、`dd`、`details`、`div`、`dl`、`dt`、`figcaption`、`figure`、`h1`–`h6`、`hr`、`li`、`ol`、`p`、`pre`、`summary`、`table`、`tbody`、`td`、`tfoot`、`th`、`thead`、`tr`、`ul`。
  - 属性：`a` 的 `href`（经 `SafeURL`，不通过就去掉）与 `title`；`abbr` 的 `title`；`bdo` 的 `dir`（`ltr`、`rtl`）；`ol` 的 `start`（整数）、`reversed`；`td`、`th` 的 `colspan`、`rowspan`（整数）；`details` 的 `open`。别的属性一律去掉（`id`、`class`、`style`、`data-*`、事件属性……）。
  - 不在白名单的元素去掉标签、保留里面的文字；`script`、`style`、`textarea`、`title`、`xmp`、`iframe`、`noembed`、`noframes`、`noscript`、`plaintext` 连同内容去掉（行内的连同作用域里两个标签之间的 Markdown，浏览器在这些元素里本来就不渲染标记）；注释、doctype、处理指令去掉；链接文字里的 `<a>` 去掉（不嵌套链接）。
- **补闭合**（比"叶子块结束时"更严）：作用域是原始 HTML 节点所在的最内层容器（段落、标题、表格单元、强调、链接……；HTML 块自己是一个作用域）。在一个作用域里打开的元素在作用域结束时依次补上闭合；结束标签在本作用域里找不到打开的同名元素就去掉；找到时先闭合它之上的。因此用户的元素在输出的词元里总是正确嵌套，不会被 HTML 解析器重建进后面的内容、包住渲染器的标记。空元素（`br`、`hr`、`wbr`）不入栈。例外（只影响版式，不越出作用域、不带属性）：用户开着的 `<a>` 里的 Markdown 链接、图片的链接与脚注引用会嵌套锚点，HTML 块里的 `li`、`dt`、`dd` 会让 HTML 解析器提前关闭渲染器的元素。

### 3.8 地址的白名单 `SafeURL`

`func SafeURL(raw string) (string, bool)`，Markdown 的链接、图片、自动链接，用户的 `a[href]`，扩展的输出都经它：

1. 去掉首尾的 C0 控制字符与空格，去掉其中所有的制表符与换行（与浏览器解析地址相同）；之后还有别的控制字符就不放行（浏览器不去掉它们，`mailto` 也不经解析）。
2. 有协议（`^[A-Za-z][A-Za-z0-9+.-]*:`）：只放行 `http`、`https`（解析出主机，主机不能为空）与 `mailto`，不分大小写；`javascript:`、`data:`、`vbscript:`、`file:` 等一律不放行，`https:evil.com` 这样没有主机的也不放行。
3. 没有协议即相对地址：把 `\` 当作 `/` 之后以 `//` 开头的（`//host`、`/\host`、`\\host`）是外站，不放行；能解析、没有主机的放行（路径、`?…`、`#…`、空串）。
4. 放行时返回第 1 步之后的地址，输出时再按属性转义。

### 3.9 页面：`getPageView`

| 操作 | 判定 | 成功 |
|---|---|---|
| `getPageView` `GET /api/v0/pages/{page_id}/view` | `page.read`（已有） | 200 `PageView{html, revision}` |

- 码只有 `page.not_found`（与 `getPage` 相同：不存在、已删除、看不到笔记本）。
- 用例照 `GetPage`，不开事务、不加锁：节点（不是页面答 404）→ 所属工作区 → 判定 → `Nodes.PageContent`（正文与 `revision` 一条语句读出，节点或正文已删答 404）→ `Markdown.Parse` → `Markdown.Render(ctx, parsed, PageRef{NotebookID, PageID})`。不缓存 HTML（M4 总设计第 4 节"解析时机"）。
- 端口：`app.Markdown{Parse(content string) Parsed; Render(ctx, Parsed, PageRef) (string, error)}`；`Parsed` 是 `any` 底层的具名类型，页面的 `app` 不看它的内容，只交回 `Render`（P4 起还交给观察者与守卫，注册者在组合根转换）。适配器 `adapter/markdown` 包住 `*markdown.Markdown`，`Render` 收到不是它产生的值时报错。`app.PageView` 的名字已被 `getPage` 的结构占用，阅读视图的值叫 `ReadingView{HTML, Revision}`，契约里叫 `PageView`。
- 查询 `PageContent :one`：`SELECT content, revision FROM page_contents WHERE node_id = $1 AND deleted_at IS NULL`（删除节点时它的正文同一时刻进回收站）。
- 不记日志（读用例都不记）。渲染是 CPU 上的工作，不受请求期限打断；第 3.10 节的预算保证它与字节数成比例。

### 3.10 耗时

- **基准**（手动运行，结果写进第 7 节）：普通文档 100 KB、1 MB、5 MB 的 `Parse` 与 `Parse` + `Render`。预算在 S4 按实测定，目标是 5 MB 的渲染不超过 1 秒、解析不超过 0.5 秒（本机）。
- **相对耗时的测试**：`markdowntest` 的生成器覆盖第 1 节表格的每一类，另加 YAML（别名、深层嵌套、很多键）、原始 HTML（大量标签、深层嵌套、未闭合、大量属性）、地址、超长行、超大 frontmatter；每个生成 512 KB，取三次中最快的 `Parse` + `Render`（每次之前先回收一次堆），不超过同样大小的普通文档的 K = 10 倍，四分之一的输入的耗时乘 8 再加 1 ms 是全尺寸的上限（线性为 4 倍、平方为 16 倍；先量四分之一，慢的不再量全尺寸）；分配不超过普通文档的 kAlloc = 14 倍（每个标签一个分词器只慢 4.8 倍，分配却多十几倍）。检查在 `markdowntest.CheckCosts`，平台包与组合根（应用的实例）各跑一次；竞态检测下跳过（`markdowntest/race.go`、`norace.go`），`make test-go` 在不带竞态检测的构建里另跑这两个测试。
- **输出的字节数**（`markdowntest.CheckSize`）：HTML 不超过正文的 64 倍加 4 MiB（单位字节最多的是脚注的引用与回链，约 48 倍；两个展开预算之内，再短的正文也能写出约 2 MB 的引用与属性表，常数留它的两倍）。`CheckCosts` 查每个病态输入的全尺寸，放大类（`Amplifying`：宽表头下的短行、被引用很多次的长地址与长标题、被别名重复的长值）在 16 KB 查——退化时它们在 256 KB 会耗尽内存；`FuzzRender` 也查。
- **模糊测试**：`FuzzParse`、`FuzzRender`（任意字节：不 panic，最终 HTML 满足不变量与字节数的上限，frontmatter 的边界与规则 1 一致），种子是样例集与生成器的输出；普通 `go test` 只跑种子，变异用 `-fuzz` 手动跑（仓库的第一个模糊测试）。
- **最终 HTML 的不变量**（`markdowntest.CheckHTML`）：用 `x/net/html` 的分词器读输出的词元（建树会拒绝 512 层以上开着的元素，用户的标签就能到），只许出现白名单与渲染器（含已注册扩展的 `Markup`）的元素与属性；每个结束标签关闭最内层开着的元素，结尾没有开着的（空元素除外）；`href` 都经 `SafeURL`；`id` 都以 `nw-` 开头；`class` 只有渲染器的与 `language-*`。它的允许表与清洗的白名单分开写（测试的判据独立于实现）。随样例集、生成的输入、模糊测试与每个注册的扩展一起跑；组合根的测试用应用的实例再跑一遍。

### 3.11 依赖与架构规则

- 新的直接依赖：`github.com/yuin/goldmark v1.8.6`、`golang.org/x/net`（`html`，取当时最新的补丁版本），`go.yaml.in/yaml/v3 v3.0.5` 由间接改为直接。不加 bluemonday、chroma。
- archtest 新规则：goldmark、`x/net/html`、`go.yaml.in/yaml` 只许 `internal/platform/markdown/...` 导入（与"River 只在 jobs"同一写法；`platform/config` 经 koanf 间接用 YAML 不受影响）。M6 的注册者要导入 goldmark 时改这条规则。
- `markdowntest` 登记为测试辅助（`testHelpersOnlyInTests` 的列表、规则名、反例三处）。
- 组合根的到达列表加 `markdownExtensions`。

### 3.12 前端

本 Phase 不改前端（契约改动后重新生成的 TS 类型除外）。P5 多出：代码高亮的交互增强（highlight.js 在 Web Worker 里、超时终止）与它的样式表（明暗两套，暗色挂在 `.dark` 下）。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `internal/harden`：替换件与包装件、容器的嵌套上限；与原版的核对（规范例子、扩展例子、随机输入）；第 1 节表格各类的耗时 | [P3-S1](plans/P3-S1-harden.md) |
| S2 | `Parse`、`Document`、frontmatter 与 YAML、扩展的注册与提取；样例集的 frontmatter 部分 | [P3-S2](plans/P3-S2-parse.md) |
| S3 | `Render`：标记、清洗与补闭合、`SafeURL`、属性表、按页取数据；`markdowntest.CheckHTML`；样例集全部渲染 | [P3-S3](plans/P3-S3-render.md) |
| S4 | 耗时：生成器、相对耗时的测试、基准与预算、模糊测试；Makefile | [P3-S4](plans/P3-S4-costs.md) |
| S5 | 页面的 `getPageView`；组合根、架构规则、矩阵；应用的实例过不变量 | [P3-S5](plans/P3-S5-page-view.md) |

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 加固 | 规范与扩展的例子、随机输入：加固配置与原版逐字节相同；四条上限（33 层容器、33 层括号、单元格多于字节数的表格、超出展开预算的引用）的行为；每个替换件与前置检查的边界（不闭合、同长与不同长的反引号串、转义的 `>`、链接里的链接、`@` 结尾与否） |
| 解析 | frontmatter 的边界（BOM、CRLF、没有结束行、第一行不是 `---`）；YAML 的标量表（样例 055 之外的 `.inf`、`.nan`、`-0x1`、超出 int64、时间戳）、标签、键（重复、非标量、`<<`）、别名展开与三条上限（节点数、深度、别名重复的字节数）；样例集的 frontmatter 期望全部一致；扩展的提取结果出现在 `Document` 里 |
| 渲染 | 标记的表格（标题 id 的去重与非 ASCII、脚注、任务项、表格对齐、代码块的语言、属性表的各种值）；清洗的表格（事件属性、`style`、`javascript:`、`//host`、`/\host`、拆开与未闭合的标签、跨作用域的结束标签、伪造的渲染器标记 `class`/`id`、原始 `img`、`script` 与内容、注释、链接里的 `<a>`）；`SafeURL` 的表格；样例集全部渲染不出错、满足不变量、frontmatter 不影响正文（与只有正文的那一段单独渲染相同）；扩展的测试替身同时出现在提取与渲染里，按页取的数据到达它的渲染器，`Fetch` 的错误传出 |
| 耗时 | 相对耗时的测试；基准；模糊测试的种子 |
| 页面 | 用例的次序与 404（不存在、已删、不是页面、看不到笔记本、正文已删）、不开事务、解析与渲染收到正文与这一页；适配器拒绝别处的值；仓储的 `PageContent`；HTTP 的转换；模块根：扩展的测试替身经 `markdown.New` 到达阅读视图，按这一页取的数据出现在 HTML 里 |
| 整个程序 | 矩阵一行（三种角色可读）；阅读视图与逐项读取一致；应用的 `markdown` 实例过样例集与生成的输入的不变量；架构规则 |

反向对照（每个新检查各一个，13.4 第 1 条）：强调的配对不按作用域（`*[a*](b)` 跨进链接）；去掉 `openers_bottom`（耗时测试失败）；链接目标不限括号（耗时测试失败）；"链接里的链接"的计数不按块清零；代码段的检查把不同长度的串当作结束；容器上限改成 `>` 之后不检查；YAML 不展开别名、不计数（别名炸弹的测试失败）；`010` 按八进制；frontmatter 的结束行不去 `\r`；清洗不补闭合；结束标签越过作用域；`SafeURL` 不把 `\` 当 `/`；图片输出 `<img>`；标题 id 不带前缀；不变量的检查本身（拿一段含 `onclick`、外站 `href` 的 HTML 必须报错）；架构规则（反例表）。

## 6. 完成标准

- 第 5 节的测试全部通过，`GOFLAGS=-p=3 make check`、`make gen-check`、前端的检查为绿；持续集成为绿。
- 预算与 K 的数字、基准结果写进第 7 节。
- 审查（Opus）完成，发现已处理，记录在 `reviews/P3-markdown-review.md`。
- 00 号文档的进度表与本文的"结果"更新。

## 7. 结果

- 分支 `m4-p3-markdown`：S1 `24dedca`；S2 `9f4d74e`；S3 `e9e1b26`；S4 `aab8cac`；S5 `b94f544`；审查修复 `96fe3b4`（加固）、`2daaab9`（平台包）、`61c5b0c`（markdowntest）、`e05a2e1`（修复核对的发现）；`3467491` 合并（`--no-ff`）。
- 门禁：每个 Step 与审查修复的 `make check` 为绿；`make gen-check`、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P3 审查](reviews/P3-markdown-review.md)。3 个 Critical 都是输出放大（表格补齐的空单元格、引用式链接的展开、YAML 别名重复的长值），5 个 Major 是四处平方的 CPU 与耗时检查不看输出字节数；全部在合并前修好，Minor 与大部分 Nit 一并处理，B3 的一半（非特定标签）、B4、S1 写成已知差异。修复的反向对照 55 项全部失败（其中两项先存活，补了测试）；修复另经 Opus 核对：29 条处置全部属实，另有 1 项 Minor、6 项 Nit，合并前一并修好（审查记录的"修复的核对"）。

**预算与 K**（本机 darwin/arm64，负载 6–7 时测）：

- 普通文档（`markdowntest.Normal`）：`Parse` 20–25 MB/s，5 MB 237–258 ms、分配 310 MB；`Parse` + `Render` 17.6–18.9 MB/s，5 MB 282–289 ms、分配 381 MB（目标 0.5 s 与 1 s）。100 KB、1 MB、5 MB 的吞吐相同。
- 相对耗时：256 KB 的病态输入，K = 10（实测最多约 4.4：电子邮件那串字符后接坏的域名；强调的不配对约 4）；两倍的输入不超过三倍的耗时加 1 ms（实测每翻一倍 1.9–2.6）。合并之后 main 的持续集成两次失败在这一步（推断：没有日志的权限），本机冷缓存下量到 128 KB 到 256 KB 每翻一倍 2.97，小尺寸上线性的开销也长得比尺寸快；改为 512 KB、四分之一到全尺寸不超过 8 倍（本机实测最多约 5.6，与普通文档之比最多约 4.3），持续集成的失败另写成注解（`m4-p3-ci-costs`）。分配 kAlloc = 14（实测最多约 10.3：HTML 块，每块一个 4 KB 的分词器）。普通 1 MB 不超过 1 秒。
- 输出的字节数：不超过正文的 64 倍加 4 MiB；实测最多是脚注的引用与回链，约 48 倍；预算之内最多的组合（别名重复的 `&` 值、别名重复的空映射、`&` 地址的图片引用）3.6 KB 的正文写出 1.9 MB；放大类在 16 KB 都在上限之内（表格超出预算时是段落，约 1 倍；引用约 6.5 倍）。
- 加固的四条上限：容器（含脚注定义）32 层、内联链接目标的括号 32 层、表格的单元格不多于字节数、引用式链接展开 max(原文字节数, 100 000)；frontmatter：节点 10 000、深度 64、别名重复 max(YAML 字节数, 100 000)。

**模糊测试**：`FuzzRender` 在实现时跑 10 分钟（1483 万次，语料 858 条），审查修复之后带上字节数的检查再跑 8 分钟（1771 万次，语料 2066 条），核对者另跑 `FuzzRender` 4 分钟、`FuzzParse` 2 分钟，都没有失败。加固与原版的随机对照另以一百万个更长的输入跑过一次，核对者用自己的生成器跑了 270 万个，都没有差异。

**与计划的出入**（已同步进上文）：

1. 链接的计数不在块结束时清零，比较标签打开时的计数（S1 计划的"清零"与它的反向对照随之改写）。
2. 代码段的前置检查用"到达 goldmark 的开头数"测，不用耗时：256 KB 分不出 O(n^1.5)。
3. 耗时先量一部分（起初是一半，现在是四分之一，见上面"相对耗时"），慢的输入不再量全尺寸；另加分配的检查（每个标签一个分词器只慢 4.8 倍）。基准发现标题 id 的后缀平方，改为每个文字记下次的后缀。
4. 样例集的读取在 `markdowntest.Fixtures`；`Document` 另存解析用的副本，`Render` 从它渲染；BOM 空白化成换行（3.2）。
5. 行内原始 HTML 用自己的读取，不用分词器；结束标签按名字计数；链接的层数在遍历中计数；被丢弃的元素连同之间的 Markdown（3.7）。
6. `SafeURL` 拒绝余下的控制字符（3.8）。
7. `CheckHTML` 读词元不建树，另用一个栈查嵌套；耗时的检查在 `markdowntest.CheckCosts`，组合根用应用的实例再跑（3.10）。
8. 审查之后：表格、引用、脚注定义的三条上限与 frontmatter 别名的字节预算（3.3、3.4）；输出字节数的检查（3.10）；属性值按分词器在属性里的规则解引用（3.7）；数字照 JSON 的写法（3.3、3.6）。

**留给后面的**：扩展的约束（链接的两条、每种节点都要有渲染函数、展开类的语法要设预算）写进 [M6 的移交](../M6-links/handoffs/M4-P3-markdown-extensions.md)（抄送 M5、M7）；P5 的阅读视图要带上 `.nw-props`、`.nw-image` 的样式与高亮的 Worker，GitHub 式的 `<details>` 里写 Markdown 渲染为空的折叠块（HTML 块各自是作用域，P3 审查 Q1），写进 P5 的已知差异；5 MB 的页面一次阅读分配约 380 MB，M12 的压测要包含大页面的并发阅读（P3 审查 Q3）。
