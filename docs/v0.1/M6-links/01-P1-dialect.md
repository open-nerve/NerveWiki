# M6/P1 方言（服务端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P1 方言（服务端） |
| 状态 | 已完成（2026-10-05） |
| 基线 | M6 设计审查的修订提交之后的 main；本文提交之后开分支 `m6-p1` |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.1、4.2、4.9（"P1 到 P3 之间""宽的内容"）、第 3、7、9 节；[M4/P3 给 M6 的移交](handoffs/M4-P3-markdown-extensions.md) 第 1–7、11 项；样例集 [README](../../../tools/md-fixtures/README.md) |

---

## 1. 基线

作者读代码（2026-10-04）：

- **扩展的接口**：`markdown.Extension{Name, Parser, Extract, Fetch, Renderer, Markup}`。
  - `Extract(root, content)` 在 `Parse` 的最后按注册次序调用，拿不到 frontmatter；
  - `Fetch` 的结果只交给本扩展的 `Renderer`；
  - M5 的 `tasks` 是唯一的注册者，样板在 `platform/markdown/tasks/tasks.go`。
- **解析器**：`internal/harden/parser.go` 组装 goldmark 的解析器。行内解析器的优先级（小的先试）：
  - `codeSpanGuard` 90、代码段 100、`footnoteRef` 101、`links` 200、自动链接 300、`rawHTMLGuard` 350、原始 HTML 400、`runs` 500、`linkify` 999；
  - 任务项的 `[` 是 0；
  - 段落转换器：引用定义 100、表格 200；
  - AST 转换器：`emphasisPass` -1、`tableCells` 0、`footnoteList` 999，扩展加的之后，`headingIDs` 100。
- **链接**：`internal/harden/links.go` 做 Markdown 链接与图片。
  - 链接节点只有 `[` 的位置（`SetPos`），没有目标的位置；
  - 行内链接的目标在 `inlineDestination` 里按读到的行切出，起点就是那一行的 `seg.Start`（尖括号的从 `seg.Start+1`）；
  - 引用定义是树里的 `ast.LinkReferenceDefinition`（`refdefs.go` 的 `define`），目标的位置同样没有记下。
- **游程**：`emphasis.go` 的 `runs` 只认 `*`、`_`、`~`。`pair` 按 CommonMark 的 process emphasis 配对，`bottomOf` 有 18 个下界（字符 × 能否开 × 原长模 3）；`match` 为 `~` 做删除线。
- **渲染器的优先级**：goldmark 先按优先级排好，从数值大的往小的依次登记，后登记的覆盖先登记的，所以数值小的胜出。
  - `marks`（100）接管链接、图片、围栏代码与清洗后的 HTML；
  - 表格的渲染器是 goldmark 的（500）。
- **frontmatter**：`frontmatterOf` 找到之后用 `yaml.v3` 解析成 `[]Property`，只有值没有位置。`reader.value` 递归时用 `aliased` 计数别名的展开。
- **钉住今天输出的测试**：
  - `harden_test.go`（与 goldmark 原版逐字相同，含随机输入）、`parts_test.go`；
  - `render_test.go`、`render_check_test.go`（样例集渲染后过 `CheckHTML`）；
  - `fixtures_test.go`（只比 frontmatter）、`tasks_test.go`；
  - `bootstrap/markdown_app_test.go`。
  - 样例 JSON 的 `links`、`tags` 至今没有 Go 测试读。

## 2. 目标与范围

做（总设计第 7 节 P1）：

- 平台：
  - `Extract` 的新签名，带 frontmatter 的标量表与 Markdown 链接目标的范围；
  - `==` 的游程（选项）与 linkify 的防护；
  - 表格的滚动区域。
- `platform/markdown/obsidian`：wikilink、嵌入、标签、行内与块级数学公式、注释、callout 的解析；高亮、callout、公式的渲染；不带状态的 `<span>`；提取；`Markup`。
- 组合根：`markdownExtensions()` 加 `obsidian.Extension()`。
- 样例：`cases/` 的提取期望全部由 Go 测试核对；新加的样例与 Obsidian 核对（本机的应用版本 1.12.7）。

不做：

- 链接的状态（`Fetch`）、Markdown 链接与属性表上的状态标记：P3。
- 前端：P3 起。

## 3. 设计

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `platform/markdown/markdown.go` | `Extract func(t Tree) any`；`Tree{Root, Content, Frontmatter}` 与 `Tree.Destination(node)` |
| `platform/markdown/parse.go` | 建 goldmark 的 `parser.Context` 交给解析，解析之后取 harden 记下的目标范围 |
| `platform/markdown/frontmatter.go`、`yaml.go` | `Frontmatter.Scalars`：写在一行之内的字符串标量（属性路径、值、字节范围、引号风格），别名展开的不收 |
| `platform/markdown/render.go` | 表格的包裹层 |
| `internal/harden/links.go`、`refdefs.go` | 做链接与定义时把目标的范围记进旁表（解析的 context 里） |
| `internal/harden/emphasis.go`、`highlight.go` | `=` 的游程（导出的行内解析器 `HighlightRuns()`）、节点 `Highlight`；`pair` 与 `match` 认 `=` |
| `internal/harden/guards.go` | `linkify` 在前一字节是 `=` 时不识别；方言经 `AddressesEndBefore` 给的序列（`%%`）之前结束网址 |
| `platform/markdown/obsidian/*.go` | 扩展：`obsidian.go`（`Extension`、`Markup`）、`wikilink.go`、`tag.go`、`math.go`、`comment.go`、`callout.go`、`extract.go`、`render.go` |
| `platform/markdown/tasks/tasks.go` | 改用新签名 |
| `bootstrap/registrants.go` | `markdownExtensions()` 加 `obsidian.Extension()` |
| `markdowntest/inputs.go` | 新语法的病态与放大输入；`ordinary` 加方言 |

### 3.2 平台：`Tree`

```go
// Tree is what an extension's Extract reads.
type Tree struct {
	Root        ast.Node
	Content     []byte      // byte for byte
	Frontmatter Frontmatter // with its scalars
	destinations map[ast.Node]Span
}

// Destination is where n's destination is written, n a Markdown link or
// image: its own, or its reference definition's.
func (t Tree) Destination(n ast.Node) (Span, bool)
```

- **目标的范围**：
  - `links.go` 在做出行内链接时记下目标在原文里的范围（尖括号之内，不含 `<>`）；
  - 引用式的记下用到的定义的标签，`refdefs.go` 的 `define` 记下每个定义的目标范围；
  - 解析结束后，`markdown.Parse` 从 context 里取出这两张表，按标签把引用式的连到定义的范围。
  - 存在旁表而不是节点的属性里：goldmark 的 `renderLink` 会写出某些属性，差分测试就不再逐字节相同（设计审查 N4）。
- **标量表**：`reader.value` 走到 `ScalarNode` 时，若是字符串、`aliased == 0`、写在一行之内（plain、单引号、双引号），记下：
  - `Path`：属性路径，照样例的写法（`sources.0`、`l.0.m`）；
  - `Value`：值；
  - `Span`：原文里的字节范围，不含引号；
  - `Quote`：引号风格，`0`、`'`、`"`。
  - 起点由 `yaml.Node` 的行列换算：列按字符计，按 UTF-8 换成字节；终点按风格扫出（单引号的 `''`、双引号的转义）。
  - 属性链接要的是"值里的偏移 → 原文的偏移"：`Scalar.Offset(i)` 按风格换算（单引号的 `'` 对应原文的 `''`）。
- `tasks` 的 `extract` 改为读 `t.Root`。这是改 M4 扩展点的签名（总设计第 8 节）。

### 3.3 平台：高亮与 linkify

- `harden.HighlightRuns()` 是 `=` 的行内解析器（优先级与 `runs` 相同），做 `run{char: '='}`。
  - 规则与 `~` 相同：游程至多两个 `=`；前一字符是 `=` 时不做；左右按 `ScanDelimiter` 判断能否开、能否闭。
  - 精确的规则以样例 025 与新样例为准。
- `pair` 与 `match` 认 `=`：`bottomOf` 加一档（24 个下界），`match` 为 `=` 做 `Highlight` 节点（harden 定义的种类，导出）。
- `harden.NewParser` 不注册它：只有 obsidian 经 `Extension.Parser` 加进来。所以 `markdown.New(nil)` 与差分测试的输出不变。
- `linkify` 的包装：前一字节是 `=` 时不识别。不开高亮时 `=` 不是触发字符，这条防护不改变任何输出（差分测试守住）；开高亮时，`a==www.example.com` 不会变成自动链接（设计审查 M1）。
- `parts_test.go` 的 `pieces()` 加入 `=`、`==`。

### 3.4 平台：表格的滚动区域

- `render.go` 为 `east.KindTable` 登记一个包装：先写 `<div class="nw-scroll">`，再交给 goldmark 的表格渲染函数，最后写 `</div>`（收尾更正：原写带 `tabindex="0"`；P6 起 `tabindex` 由前端的 `scrollRegions` 按是否溢出给）。
- 横向滚动改在这一层（前端的样式与 `reading/scroll-regions.ts` 的 `scrollRegions` 在 P6 跟上；收尾更正：原写 `reading/scroll-focus.ts`，P6 B 以 `scroll-regions.ts` 取代了它）。
- `CheckHTML` 的核心白名单加 `div` 与 class `nw-scroll`（收尾更正：原写还有 `tabindex`）。

### 3.5 扩展：wikilink 与嵌入

- 行内解析器，触发字符 `[`（优先级 150：在 `footnoteRef` 之后、`links` 之前）与 `!`（嵌入，`![[`）。
- 规则 7：
  - 从 `[[` 到同一行里第一个 `]]`，中间不能有 `[`、`]`；
  - 第一个 `|` 或 `\|` 之后是显示文字；
  - 目标里第一个 `#` 之后是锚点；
  - 目标与显示文字去掉首尾的空格与制表符；空的显示文字等于没有。
  - 转义的 `[`、`!` 由 goldmark 的反斜杠处理先吃掉，不会触发。
- 节点 `Wikilink{Embed, Target, Anchor, Display, Range}`：不解析里面的行内内容，所以显示文字里的标签不算（样例 048）。
- 目标为空（只有锚点）的照样是节点、照样渲染，但不进提取（规则 7、样例 008）。
- 在 Markdown 链接的文字里照样做（样例 030、040）。它不是 `*ast.Link`，不计入"链接里不能有链接"。

### 3.6 扩展：标签

- 行内解析器，触发字符 `#`。
- 规则 9：
  - `#` 前面是空白（含全角空格），或位于一段文字的开头（块的开头，或紧跟在强调、删除线、高亮、行内代码、链接、wikilink、行内 HTML、另一个标签之后）；
  - 之后是 `\p{L}`、`\p{M}`、`\p{N}`、`_`、`-`、`/`，不全是数字；
  - 其余字符（ASCII 标点、全角标点、emoji）结束标签。
- "紧跟在行内元素之后"在解析时看父节点的最后一个子节点：
  - 它不是普通文字、并且正好结束在 `#` 之前，就可以；
  - 游程（`*`、`_`、`~`、`=`）在解析时还没配对，所以先接受，emphasis 配对之后由一个 AST 转换复查：前面的游程没配成对、并成了文字的，标签退回文字。
- 节点 `Tag{Name, Range}`；ATX 标题由块解析器先吃掉，不会触发。

### 3.7 扩展：数学公式

规则 5。

- **块级**：块解析器（在段落之前），以 `$$` 开头、这一行后面没有另一个 `$$` 的行开始，到第一个以 `$$` 结尾的行结束，没有结束行时到文末；可以打断段落。内容是原始的。
- **行内 `$$…$$`**：两侧允许空白，可以跨行，不跨段落。
- **行内 `$…$`**：
  - 开头的 `$` 后面不是空白；
  - 结尾的 `$` 前面不是空白、后面不是数字；
  - 不满足的 `$` 不结束公式，继续向后找；
  - `\$` 是普通字符。
- 线性：每个块第一次需要时，扫一遍它的行，记下所有可以结束公式的 `$` 的位置（照 harden 的 `indexOf`），开头的 `$` 二分查找下一个。
- 节点 `Math{Display, Range}`，内容不再解析。

### 3.8 扩展：注释

规则 6。注释只影响显示：

- **标记**：行内解析器（触发 `%`）把 `%%` 做成标记节点，里面的内容照常解析，所以注释里的链接、标签照常提取。
  - 原始内容（代码、公式、HTML、尖括号的自动链接、链接的目标）里的 `%%` 早被吃掉，不参与配对；
  - 字面自动链接在 `%%` 之前结束（`harden.AddressesEndBefore`，网址里的 `%%` 不是转义），所以注释可以以网址结尾；
  - 图片说明文字里的标记是文字（规则 4）。
- **配对**：一个 AST 转换按树的次序配对（即正文的次序，但脚注的定义聚在第一个定义处）。
  - 某行以标记开头、这一行里没有别的标记时，它开始块注释，到第一个"正文的行以它结尾"的标记结束；没有就到文末。只有段落的行能开始：引用、列表项、脚注定义的标记之后算行首；标题、表格单元格里的不能；
  - 其余在同一行里从左到右两两配对；
  - 落单的标记是普通文字。
- **隐藏**：配好的一段，在树的每一层把它盖住的一串节点挪进隐藏节点（块级的 `hiddenBlocks`、行内的 `hidden`）。
  - 整个被盖住的段落、标题、行内节点整个隐藏；容器（引用、列表、列表项、callout）留着，只隐藏里面的；
  - 表格的行与单元格、脚注的列表不包裹：表格的渲染器与脚注的转换器数它们的子节点；
  - 隐藏节点实现平台的 `markdown.Hider`：渲染跳过它的子节点，里面的标题不给 id，里面的文字不进标题的 id 与图片的说明文字；
  - 提取（链接、标签、任务项）照常走进去。
- **已知差异**（写进新样例，`nerve-defined`）：注释里不闭合的围栏延续到文末，吞掉结束的 `%%`，与 Obsidian 按行隐藏不同（074）。

### 3.9 扩展：callout

- AST 转换（emphasis 配对之后）：引用块的第一个子块是段落，它的原文以 `[!type]` 开头（之后可以有 `-` 或 `+`），就把引用块换成 `Callout{Type, Fold}`。
  - 标记那几个字符的文字节点去掉，它不是链接（样例 007）；
  - 第一行剩下的行内节点挪进 `CalloutTitle`，其余是正文。
- 渲染：
  - 带 `-` 或 `+` 的是 `<details class="nw-callout" data-callout="type">`（`+` 带 `open`），标题在 `<summary>`；
  - 其余是 `<div class="nw-callout" data-callout="type">`，标题在 `<div class="nw-callout-title">`。
  - 没有标题时，标题是类型的首字母大写，与 Obsidian 相同。

### 3.10 扩展：渲染（不带状态）

| 节点 | P1 的输出 |
|---|---|
| wikilink | `<span class="nw-wikilink" data-nw-target="…">显示文字或目标</span>`（P3 改为 `<a>`） |
| 嵌入 | `<span class="nw-wikilink nw-embed" data-nw-target="…">…</span>` |
| 标签 | `<span class="nw-tag" data-nw-tag="…">#标签</span>` |
| 高亮 | `<mark>…</mark>` |
| 行内公式 | `<span class="nw-math">TeX</span>` |
| 块级公式 | `<div class="nw-math nw-math-block">TeX</div>` |
| callout | 见 3.9 |
| `Hidden` | 什么都不写 |

- 文字一律经 HTML 转义；没有地址，所以这一 Phase 不经 `SafeURL`。
- `data-nw-target` 是目标的原文（含锚点），供 P3 之前的前端调试；P3 换成节点 id 与状态。
- `Markup`：`span` 的 `class`、`data-nw-target`、`data-nw-tag`；`mark`；`div` 的 `class`、`data-callout`；`details` 的 `class`、`data-callout`、`open`；`summary`；以及上面的 class。

### 3.11 扩展：提取

`Extract(t Tree)` 走一遍树，按 `Range` 的起点排序：

- **wikilink 与嵌入**：节点自己的字段。不走进 `Image` 的子节点：图片的说明文字是原始内容（规则 4）。
- **Markdown 链接与图片**：`*ast.Link`、`*ast.Image`，目标取自 `t.Destination`。
  - 外部的（带协议，或以 `//` 开头）与只有锚点的不计入；
  - 目标与锚点按 `decodeURI` 的规则解码：保留字符的转义保持原样，解码失败时保持原文（规则 8）；
  - 同一个定义被引用多次只算一条。
- **属性链接**：`t.Frontmatter.Scalars` 里整个值恰好是一个 wikilink 或一个 Markdown 链接的（规则 10），范围经 `Scalar.Offset` 换回原文。
- **标签**：每一次出现，名称原样。

结果的类型：

```go
type Extracted struct {
	Links []Link // {Kind, Target, Anchor, Display, Key, Range}
	Tags  []Tag  // {Name, Range}
}
```

## 4. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | 平台：`Tree`、目标的范围、标量表；`tasks` 改签名 | `markdown: an extension reads a Tree … (M6/P1/S1)` |
| S2 | 平台：`==` 的游程、`Highlight`、linkify 的防护、表格的滚动区域 | `markdown: highlight runs, linkify's guard, scrolling tables (M6/P1/S2)` |
| S3 | 扩展：wikilink、嵌入、标签、公式、注释、callout 的解析、渲染与提取；组合根登记 | `markdown: the Obsidian dialect (M6/P1/S3)` |
| S4 | 样例与成本：读 JSON 的测试、新样例与 Obsidian 的核对、病态与放大输入、带扩展的模糊测试、渲染测试 | `markdown, md-fixtures: the dialect's fixtures and costs (M6/P1/S4)` |

## 5. 测试与验证

- **样例**：`TestTheFixturesLinksAndTagsAreTheirs`（obsidian 包）读每个样例的 `links`、`tags`，与提取结果逐字段比较；全部通过（完成时 78 个）。
- **新样例**（与 Obsidian 1.12.7 核对，`verify.mjs`）：
  - `==www.example.com==` 与 `a==www.example.com`；
  - 高亮的边界（`===`、`= =`、跨强调）；
  - 标签紧跟在没配成对的游程之后；
  - 注释与块交叉；
  - callout 的折叠标记；
  - 显示的部分（隐藏、callout 的标题）Obsidian 的 `metadataCache` 读不到，只作渲染测试，`nerve-defined`。
- **平台**：
  - `Tree.Destination` 的表格测试（行内、尖括号、引用式、一个定义多处引用）；
  - 标量表的表格测试（三种风格、`''`、双引号的转义、多字节、CRLF、BOM、别名展开的不收、跨行的不收）；
  - 差分测试：不开高亮时与 goldmark 原版逐字节相同（`harden_test.go` 原样通过）；
  - 开高亮的单独测试。
- **渲染**：每种节点的 HTML（表格测试），`CheckHTML` 用扩展的 `Markup` 跑样例集与生成的输入。
- **成本**：`Pathological()` 加：
  - 不闭合的 `[[`、`![[`；
  - 每行一个 `$` 的长段落、不闭合的 `$$`；
  - 很多的 `%%`、不闭合的块注释；
  - 深层嵌套的 callout；
  - 很多的标签；
  - `==` 的游程；
  - 应用实例的 `CheckCosts` 通过。`Amplifying()` 加很多的嵌入（v0.1 不展开，确认没有放大）。
- **模糊测试**：`FuzzParse`、`FuzzRender` 各加一个带全部扩展的实例；种子加方言。
- **反向对照**（`mut.py`）：每条规则至少一个，例如：
  - 去掉"目标不能含方括号"；
  - 标签不查前一字符；
  - 公式不查结尾 `$` 后的数字；
  - 注释的块级配对改为行内；
  - linkify 不防 `=`；
  - 标量表收别名展开的值。

## 6. 完成标准

- 第 5 节的测试全部通过，`make check`、`make gen-check`、e2e 全量通过（阅读视图里的方言变成了 `<span>`，M4 的故事里涉及的断言随之更新）。
- 与原版的差分测试照旧通过（随机输入的片段加了 `=`、`==`，比较不变）。
- 审查（Opus）的发现处理完，修复经 Opus 核对。

## 7. 结果

完成于 2026-10-05。提交：S1 `a743242`、S2 `fc715cf`、S3 `fea077c`、S4 `f513f45`；审查的修复 `4230ebb`、`c74d5b6`、`cabd489`、`1120efb`；e2e 的失败诊断 `8faed07`。合并 `4f6e419`。

**验证**

- 样例 78 个（新增 068–078），`TestTheFixturesLinksAndTagsAreTheirs` 逐字段核对全部的 `links`、`tags`。
- 与 Obsidian 1.12.7 核对（`verify.mjs`，独立的数据目录）：`obsidian-verified` 的全部一致；`nerve-defined` 的差异都与 note 相符。核对中的发现：
  - 标签"不能全是数字"指 ASCII 数字：`#½`、`#١٢٣` 是标签（075，规则 9 改写）。
  - Obsidian 在任何位置识别网址（`=`、`/`、中文之后都算）；我们照 GFM 的字面自动链接（规则 2），高亮的 `==` 之后不开始网址（068，`nerve-defined`）。
  - `a**#e2**` Obsidian 当作加粗；CommonMark 里这个 `**` 不能开始强调，`#e2` 跟在文字后面（076，`nerve-defined`）。
  - `$$` 的第二个 `$` 可以结束 `$…$`，与 Obsidian 相同（072，规则 5 补了一句）。
  - 网址在注释的 `%%` 之前结束（077）、wikilink 之后的网址照常识别（078），都与 Obsidian 相同。
- 成本：应用实例的 `CheckCosts` 通过；方言的 22 个病态输入最多约为普通文档的 3.1 倍，四倍输入约四倍时间；"很多的嵌入"在 `CheckSize` 之内。
- 模糊测试：带全部扩展的 `FuzzParse`、`FuzzRender` 各 150 秒，约 390 万、300 万次，没有发现；语料里最慢的输入（一千多个 `[`）与不带扩展时同为线性。
- 反向对照 53 个（第 5 节所列、每个解析器与转换器的规则、审查的每处修复），全部被测试抓到；起初漏网的补了测试（块注释里不在行尾的标记、坏转义之后的好转义、属性值末尾的空白）。
- `make check`、`make gen-check`、e2e 全量（183 个）、CI 通过；与原版的差分测试照旧（片段加了 `=`）。

**与设计的出入**

- 公式分为行内的 `inlineMath{display}` 与块级的 `mathBlock`：goldmark 的节点是行内还是块是固定的。提取用不到公式，节点没有 `Range`。
- 节点不导出，只有本包的渲染与提取用它们。wikilink 与标签各带一个子节点，是它显示的文字：标题的 id、图片的说明文字都用到。
- 块注释的开始照规则 6（行以 `%%` 开头、行里没有别的标记，只有段落的行）；3.8 原来写的"某行的第一个标记"不准确，随审查的修复改写。
- 隐藏的粒度：整个在注释里的段落、标题、行内节点整个隐藏；容器（引用、列表、callout）留着，只隐藏里面的；表格的行与单元格、脚注的列表不包裹，因为表格的渲染器与脚注的转换器数它们的子节点。
- 标签名末尾的 `_` 先留给强调的游程，游程成了文字再收回，所以 `_#t5_` 是包着标签 `t5` 的强调（047）。
- 属性表的钩子照第 2 节归 P3（与 `Fetch` 一起）。
- 平台加了 `markdown.Hider`（隐藏的节点）与 `harden.AddressesEndBefore`（网址在方言的标记之前结束），都是审查的修复。
- 已知的限制：callout 的标题到段落顶层的第一个换行为止，跨行的强调或链接把第二行带进标题；块注释盖住脚注定义时，脚注仍列在文末。

**审查**：[P1-dialect-review.md](reviews/P1-dialect-review.md)。High 2（标签的下划线与强调交叉时渲染 panic、一行很多标量时位置是平方的）、Medium 3（YAML 的其他换行之后位置错、网址吞掉注释的 `%%`、标题与单元格里的 `%%` 开始块注释）、Low 7，修复经三轮核对另有 8 点（含一处锚点的回退），都已处理。
