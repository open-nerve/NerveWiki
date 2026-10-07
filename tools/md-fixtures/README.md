# Markdown 样例集

这里的样例定义了 Nerve Wiki 怎样从 Markdown 中提取链接、标签和 frontmatter，以及重命名时怎样改写链接。它是规范：服务端的提取器（M6）必须通过全部样例；阅读视图的渲染测试也使用这些输入。

规则有疑问时，以样例为准；新增或修改规则时，先加样例。

## 布局

```
cases/
  NNN-<slug>.md        输入：逐字节的原文（换行符、BOM 都按原样保存）
  NNN-<slug>.json      期望的提取结果
rename/
  NNN-<slug>.md        重命名之前的原文
  NNN-<slug>.out.md    期望的改写结果
  NNN-<slug>.json      说明、页面树与改名或移动
resolve/
  NNN-<slug>.json      一棵页面树、从其中各页写出的链接与它们应当解析到的页面
render/
  NNN-<slug>.md        阅读视图的输入：逐字节的原文
  NNN-<slug>.json      期望的显示：各块与块里的换行
check.mjs              自检：格式正确，每个 range 确实指向目标的原文写法，解析样例的链接都在它的页面之间，渲染样例的显示写法正确
obsidian/verify.mjs    提取结果与真实的 Obsidian 核对
obsidian/verify-resolve.mjs  解析与真实的 Obsidian 核对
obsidian/verify-rename.mjs   改名、移动时的改写与真实的 Obsidian 核对
obsidian/verify-render.mjs   阅读视图的显示与真实的 Obsidian 核对
```

## 来源

每个样例都标明规则的来源：

- `obsidian-verified`：结果与 Obsidian 一致，由 `obsidian/verify.mjs` 核对（最近一次是 M6 的 Obsidian 1.12.7；M0 时的样例另与 1.13.7 核对过）；解析样例由 `obsidian/verify-resolve.mjs` 核对，改写样例由 `obsidian/verify-rename.mjs` 核对，渲染样例由 `obsidian/verify-render.mjs` 核对（都是 Obsidian 1.12.7）。
- `nerve-defined`：我们有意与 Obsidian 不同，或 Obsidian 没有对应的行为；`note` 写明差异和理由。

与 Obsidian 保持一致是默认选择：用户会从 Obsidian 导入笔记，agent 也按 Obsidian 的习惯书写。偏离必须有明确的好处。

## 提取结果的格式

```json
{
  "description": "这个样例在验证什么",
  "source": "obsidian-verified",
  "frontmatter": null,
  "links": [{ "kind": "wikilink", "target": "a", "anchor": null, "display": null, "key": null, "range": [3, 4] }],
  "tags": ["tag"]
}
```

- `note`：仅 `nerve-defined` 样例有，写明与 Obsidian 的差异和理由。
- `frontmatter`：没有 frontmatter 时为 `null`；YAML 解析失败或不是映射时为 `{"valid": false}`；成功时为 `{"valid": true, "properties": {…}}`。
- `links`：按 `range` 的起点排序，同一段字节只出现一次。
  - `kind` 表示写法：`wikilink`（`[[…]]`）、`embed`（`![[…]]`）、`link`（Markdown 链接）、`image`（Markdown 图片）。
  - `target`：用于解析的目标。wikilink 的目标去掉首尾空白，其余按写法记录（不去扩展名，不做 Unicode 规范化，这些留给链接解析）；Markdown 链接的目标做了解码（规则 8）。
  - `anchor`：`#` 之后的部分，没有则为 `null`。
  - `display`：`|` 之后的部分（嵌入附件时是尺寸），没有或为空时为 `null`。Markdown 链接不记录链接文字。
  - `key`：正文中的链接为 `null`；frontmatter 里的属性链接为属性路径，如 `sources.0`、`l.0.m`。
  - `range`：目标**在原文中的写法**所占的字节范围 `[起点, 终点)`，不含锚点，按文件的原始字节计算（含 BOM）。重命名时改写的就是这段字节。
- `tags`：正文中的标签，按出现顺序列出每一次出现，不含 `#`，大小写原样。frontmatter 里的 `tags` 在 `properties` 中。
- `tasks`：任务项（规则 11），只有含任务项的样例有，没有这个字段即没有任务项。按在原文中的位置列出每一个（阅读视图把脚注里的放在最后，这里不）：`offset` 是方括号里那个字符的字节位置，按文件的原始字节计算（含 BOM）；`checked` 是否勾上。勾选时改写的就是这个字节。

## 规则

1. **frontmatter**
   - 必须从第一个字节开始（前面可以有 BOM）：第一行是 `---`，到下一个只有 `---` 的行结束；没有结束行就不是 frontmatter。
   - 内容按 YAML 1.2 core schema 解析成映射：日期是字符串，`010` 是十进制，`yes` 是字符串，别名展开；`.inf`、`.nan` 这类 JSON 无法表示的值保留为原文字符串。
   - 解析失败或不是映射：没有属性，但这一段仍然是 frontmatter。内容为空：没有属性的有效 frontmatter。
   - frontmatter 永远不影响正文的解析。
2. **语法范围**：CommonMark，外加 GFM 的表格、删除线、任务列表、字面自动链接，以及脚注、`==高亮==`、callout、数学公式、`%%` 注释、wikilink、嵌入和标签。不支持的语法（例如行内脚注 `^[…]`）按普通文字处理，其中的链接和标签照常识别。
3. **识别顺序**：块结构按 CommonMark；行内结构从左到右识别，先开始的优先。
   - `` `[[x]]` `` 是代码；``[[a `b` c]]`` 是 wikilink；`www.a.com/[[x]]` 是网址；`[[Page]](2023)` 是 wikilink 后跟文字。
   - 表格先按没有转义的 `|` 切分单元格，再识别单元格里的行内结构。
4. **原始内容**：以下内容不产生链接和标签：
   - 代码：行内代码、围栏代码块、缩进代码块；
   - 数学公式；
   - HTML 块、行内 HTML 标签和 HTML 注释；
   - 自动链接：尖括号自动链接和字面自动链接；
   - Markdown 链接和图片的目标与标题；
   - 图片的说明文字。

   Markdown 链接的文字照常识别：里面的 wikilink 和标签都算。

5. **数学公式**
   - 行内 `$…$`：开头的 `$` 后面不是空白；结尾的 `$` 前面不是空白，后面也不是数字（`$$` 的第二个 `$` 也可以结尾）。不满足条件的 `$` 不结束公式，继续向后找。可以跨行，但不跨段落。`\$` 是普通字符。
   - 行内 `$$…$$`：两侧允许有空白，可以跨行。
   - 公式块：某行以 `$$` 开头，且这一行后面没有另一个 `$$`，就开始公式块；遇到第一个以 `$$` 结尾的行结束。没有结束行时延续到文末。公式块可以打断段落。
6. **注释**：注释只影响阅读视图的显示，**不影响提取**，注释里的链接和标签照常计入，重命名时照常改写。显示规则：
   - 行内注释：同一行里的 `%%…%%`，从左到右两两配对；
   - 块注释：某行以 `%%` 开头，且这一行后面没有另一个 `%%`，就开始块注释；遇到第一个以 `%%` 结尾的行结束。没有结束行时延续到文末。引用、列表项和脚注定义的标记之后算行首；标题和表格单元格里的 `%%` 不开始块注释；
   - 其余落单的 `%%` 是普通文字；原始内容（包括图片的说明文字）里的 `%%` 不参与配对。字面自动链接在 `%%` 之前结束，所以注释可以以网址结尾。
   - 块注释的各行不显示，也不留空行（渲染样例 013、020）；只有一个行内注释的行留下一个空行（012），与 Obsidian 相同。
   - 与 Obsidian 不同（`nerve-defined`）：Obsidian 的块注释在下一个 `%%` 处结束，不管它是否在行尾，这一行剩下的照常显示；这里到以 `%%` 结尾的行才结束（渲染样例 021）。块注释在一段的中间时，Obsidian 把这一段分成两段，这里是一段的两行（014、018）。
7. **wikilink**
   - 写法是 `[[…]]`，必须在同一行内，内部不能含 `[` 或 `]`；`![[…]]` 是嵌入。
   - `[` 前面有奇数个反斜杠时，按普通文字处理；`!` 被转义时，是普通的 wikilink 而不是嵌入。
   - 第一个 `|` 或 `\|` 之后是显示文字；目标中第一个 `#` 之后是锚点。
   - 目标和显示文字去掉首尾的空格和制表符。
   - 目标为空（只有锚点，指向本页）时不计入。
8. **Markdown 链接和图片**
   - 目标按原文中的写法取出，去掉 `<…>`；`#` 之后是锚点。目标和锚点按 JavaScript `decodeURI` 的规则解码：`%2F` 这类保留字符的转义保持原样，解码失败时保持原文。
   - 外部链接（带协议，或以 `//` 开头）和只有锚点的链接不计入。
   - 引用式链接：被使用的定义是一条链接，`range` 在定义里；同一个定义被引用多次只算一条；没有被使用的定义不是链接。
9. **标签**
   - `#` 前面必须是空白（含全角空格），或者 `#` 位于一段文字的开头：块的开头，或紧跟在强调、删除线、高亮、行内代码、链接、wikilink、行内 HTML、另一个标签等行内元素之后。
   - `#` 之后由 Unicode 字母（`\p{L}`）、组合用字符（`\p{M}`）、数字（`\p{N}`）以及 `_`、`-`、`/` 组成，不能全是 ASCII 数字（0–9）。其余字符结束标签，包括 ASCII 标点、全角标点和 emoji。
10. **属性链接**：YAML 解析成功时，写在一行之内的字符串标量，如果整个值恰好是一个 wikilink，或恰好是一个 Markdown 链接，就是属性链接。
    - 嵌入、夹带其他文字、YAML 注释里的链接、被 YAML 解析成嵌套列表的 `[[x]]` 都不算。
    - `range` 是目标在原文中的写法，单引号标量里的 `''` 按原样计入。
11. **任务项**：GFM 的任务列表，照 goldmark 的识别（阅读视图用它）。
    - 列表项的第一个块是段落或标题（ATX 或 setext），它以 `[` 加一个字符加 `]` 开头：那个字符是空白（空格、制表符等）就是没勾的任务项，`x` 或 `X` 是勾上的。`]` 之后不要求空格。
    - 其余都不是：不在列表里、不在那个块的开头、列表项的第二个块、方括号里是别的字符（Obsidian 把 `[-]`、`[?]` 也当任务项，这里不当）。
    - 脚注定义里的列表照样有任务项；没被引用的脚注定义不显示，其中的任务项也不算。
    - `- [x]: /u` 是引用定义，不是任务项；`- [ ]: /u` 是任务项（只有空白的标签不能作定义）。
    - 任务项不影响链接和标签的提取。
12. **换行**（阅读视图）：段落、列表项、引用、callout 的正文、脚注里的单个换行显示为换行，与 Obsidian 关闭"严格换行"（它的默认）时相同；行尾的两个空格或反斜杠同样是一个换行。
    - 行尾是标签、wikilink、强调、代码、高亮、链接时也是（渲染样例 003）；跨行的链接文字、显示文字照样换行（008）。
    - callout 的标题到第一个换行为止（006）。
    - setext 标题：Obsidian 只把一行的当标题（023），两行的连同 `===` 显示为一段；这里照 CommonMark，两行的也是标题，显示为标题里的两行（007，`nerve-defined`）。
    - 换行不影响提取：标题的 id、图片的说明文字、属性链接的文字都把它读成空格。

## 重命名改写样例

`rename/` 里的每个样例：一棵页面树，其中一页的正文是 `.md`；树里的一页改名或移动之后，这一页的正文应当是 `.out.md`（v0.1 设计 4.5，M6/P4 设计第 2、3 节）。

```json
{
  "description": "这个样例在验证什么",
  "source": "obsidian-verified",
  "pages": ["top", "A", "A/x", "A/y", "s", "B", "B/C"],
  "page": "s",
  "from": "A/x",
  "to": "B/C/x"
}
```

- `pages`：照解析样例；可选，没有时树只有 `from` 与 `page` 两页，都在根下。`aliases` 同解析样例。
- `page`：`.md` 是哪一页的正文，默认 `src`。一个样例一页正文。
- `from`、`to`：页面的路径。父页相同是改名，名称相同是移动；两者都变的不是一次操作。
- `nerve-defined` 的样例在 `note` 里写明 Obsidian 的结果与理由。

改写哪些链接：

- 之前解析到一页、改名或移动之后解析不到、解析到别的页，或新有歧义的，改写成仍指向那一页的写法；只改大小写的改名，按标题指向它、写法与新标题不逐字相同的也改写。
- 不改写：之前解析不到的；只有锚点的；之后仍解析到同一页、没有新歧义的（Obsidian 在候选的路径变了时也改，样例 024、027）；`aliases` 的值（样例 032）；代码里的。注释里的照改，引用式链接改它的定义。
- 之前有歧义的，照之前解析到的那一个（并列按 id，样例里是先列出的）。

写法：

- wikilink 与嵌入：名称在笔记本里只有这一页（或它在根下）时写名称，否则写从根起的完整路径（不带开头的 `/`）；原来的相对、从根起、带 `.md` 的写法不保留（Obsidian 的 `fileToLinktext`）。只换目标那一段：锚点、显示文字、目标两侧的空白保留。
- 显示文字：目标带 `/`、没有锚点、显示文字与目标的最后一段逐字相同的，换成新标题（样例 006）；经别名解析到、被别的页抢走的，写成那一页并加上原来写的别名作显示文字（样例 028）。
- Markdown 链接：普通的写法同 wikilink，加 `.md`；`./`、`../` 开头的从出发页的父页重新算相对路径（在它之下的以 `./` 开头），`/` 开头的写完整路径。尖括号写法原样写入；其他写法只编码空格（`%20`）、`%`、`(`、`)`，其余字符（包括中文）原样。地址两侧的空白、标题、锚点保留。纯文本的链接文字等于原来的标题（带 `/` 时等于原来的完整路径）的，换成新标题。
- 属性链接按标量的引号风格编码：单引号里的 `'` 写成 `''`。标题不允许 `"` 和 `\`，所以双引号里不需要转义。frontmatter 的其余字节不动。
- 只替换链接的那几段字节，其余字节不动，包括换行符风格。

核对：

```sh
node tools/md-fixtures/obsidian/verify-rename.mjs prepare /tmp/nwiki-rename
# 按提示用独立的数据目录启动 Obsidian，它不会碰你自己的库
node tools/md-fixtures/obsidian/verify-rename.mjs check /tmp/nwiki-rename
```

每个样例清空库，建出它的页面（页面 `A` 是 `A.md`，有子页的另有文件夹 `A/`），打开"始终更新内部链接"、最短的链接格式，经 `app.fileManager.renameFile` 改名或移动（有子页的页是两次：`A.md` 与 `A/`），读那一页之后的正文。`obsidian-verified` 必须一致；`nerve-defined` 只报告差异。

## 链接解析样例

`resolve/` 里的每个样例是一棵页面树和从其中各页写出的链接，以及每条链接应当解析到的页面（v0.1 设计 4.4，M6/P3 设计第 2 节）。

```json
{
  "description": "这个样例在验证什么",
  "source": "obsidian-verified",
  "pages": ["note", "A", "A/note", "A/B", "A/B/note", "src", "A/B/src"],
  "aliases": { "A/B": ["Al"] },
  "links": [
    { "from": "A/B/src", "link": "[[note]]", "to": "note" },
    { "from": "src", "link": "[[Al]]", "to": "A/B", "source": "nerve-defined" }
  ]
}
```

- `pages`：页面从笔记本根起的路径，父页排在子页之前。导出时页面 `A` 是 `A.md`，它的子页在 `A/` 下。
- `aliases`：可选，页面在 frontmatter 的 `aliases` 里写的别名。
- `links`：每条一个链接。
  - `from` 是写出它的页面；
  - `link` 是链接的写法，可以是 wikilink、嵌入或 Markdown 链接；
  - `to` 是解析到的页面，解析不到时为 `null`；
  - `ambiguous: true` 表示有歧义：多个候选在每一项偏好上都相同，按 id 选；
  - `source` 可选，覆盖样例的来源。
- 样例或其中的链接是 `nerve-defined` 时，样例的 `note` 写明差异与理由。
- 样例里页面的 id 按 `pages` 的次序递增，与新建的先后一致。

先定目标的写法：最后一段以 `.md` 结尾（不分大小写）的，笔记本里任何地方有去掉 `.md` 的那个名称的页时，读作去掉 `.md` 的；没有时读作原样（标题以 `.md` 结尾的页）。之后每一步只用这一种写法（Obsidian 的 `getLinkpathDest` 如此）。

解析的次序（前一步找到就停）：

1. **相对**：以 `./`、`../` 开头的，从出发页的父页（根下的页是笔记本根）起，`..` 每个上一层（到根为止），再按段往下。找不到就解析不到。
2. **从根起**：以 `/` 开头的，或者目标恰好是某页从根起的路径（单个名称也算）。
3. **路径后缀**：页面的路径以目标的各段结尾（按整段对齐）。多个时先选在出发页父页的子树里的（父页自己也算：导出时 `A.md` 与 `A/` 并排），再选路径短的（导出路径的字符数，按 JavaScript 的计法，即 UTF-16 码元，与层数无关），再按 id，并标记歧义。
4. **别名**：只对单个名称；以 `.md` 结尾的，先按去掉 `.md` 的别名，再按原样的。多个时同第 3 步。

Obsidian 用小写路径的字符串前缀、后缀比较子树与路径后缀，相对路径找不到时还按算出的路径找后缀；这里按整段比较，相对路径找不到就解析不到（样例 015，`nerve-defined`）。

核对：

```sh
node tools/md-fixtures/obsidian/verify-resolve.mjs prepare /tmp/nwiki-resolve
# 按提示用独立的数据目录启动 Obsidian：每个样例一个库、一个窗口，不碰你自己的库
node tools/md-fixtures/obsidian/verify-resolve.mjs check /tmp/nwiki-resolve
```

每条链接单独放在出发页所在的文件夹里的一个文件中，读 Obsidian 的 `resolvedLinks` 与 `unresolvedLinks`（等每个文件都有了它们）；每个文件要恰好一条链接。`obsidian-verified` 必须一致；`nerve-defined` 只报告差异。`prepare` 会清空工作目录，所以只接受它自己准备过的目录或空目录。

## 渲染样例

`render/` 里的每个样例是一页的原文与它在阅读视图里的显示（规则 6、12；M6/P8 设计第 4 节）。

```json
{
  "description": "段落里的单个换行显示为换行；空行分段",
  "source": "obsidian-verified",
  "rendered": "第一行⏎第二行⏎第三行¶另一段"
}
```

- `rendered`：显示的文字。块之间是一个 `¶`，块里的换行（`<br>`）是 `⏎`；块末尾的换行不显示，不写；连续的空白写成一个空格，`¶`、`⏎` 两侧不留空白。
- 块是段落、标题、列表与列表项、引用、callout 与它的标题、代码块、表格与它的行和单元格、分隔线这类元素，它们的嵌套只算一个 `¶`。所以样例只比较块与换行，不比较块的种类、链接的地址与样式。
- `nerve-defined` 的样例在 `note` 里写明 Obsidian 的显示与理由。

服务端的测试（`obsidian/render_fixtures_test.go`）把阅读视图的 HTML 按同样的规则读成文字，与 `rendered` 比较。

核对：

```sh
node tools/md-fixtures/obsidian/verify-render.mjs prepare /tmp/nwiki-render
# 按提示用独立的数据目录启动 Obsidian，它不会碰你自己的库
node tools/md-fixtures/obsidian/verify-render.mjs check /tmp/nwiki-render
```

所有样例放进一个库，逐个在阅读视图里打开，按同样的规则读它的 DOM（跳过页头、页脚、文件名的标题、属性区与反链区）。`obsidian-verified` 必须一致；`nerve-defined` 只报告差异。`prepare` 会清空工作目录，所以只接受它自己准备过的目录或空目录。

## 新增样例

1. 手写 `.md` 与 `.json`。`range` 是 UTF-8 字节偏移，可以用 `python3 -c 'print(len("前缀".encode()))'` 之类的办法计算。
2. 运行 `node tools/md-fixtures/check.mjs`。
3. 与 Obsidian 核对（需要本机装有 Obsidian）：

   ```sh
   node tools/md-fixtures/obsidian/verify.mjs prepare /tmp/nwiki-obsidian
   # 按提示用独立的数据目录启动 Obsidian，它不会碰你自己的库
   node tools/md-fixtures/obsidian/verify.mjs check /tmp/nwiki-obsidian
   ```

   `obsidian-verified` 样例必须一致。`nerve-defined` 样例只报告差异；如果 Obsidian 其实一致，把来源改成 `obsidian-verified`。

4. 运行提取器的测试。结果与期望不一致时，先判断是样例写错了还是实现有问题。

解析样例照同样的步骤：手写 `resolve/` 的 `.json`，运行 `check.mjs`，用 `verify-resolve.mjs` 与 Obsidian 核对，再运行 linking 的解析测试。改写样例同样：`rename/` 的三个文件，`check.mjs`，`verify-rename.mjs`，再运行 linking 的改写测试。渲染样例同样：`render/` 的两个文件，`check.mjs`，`verify-render.mjs`，再运行 `platform/markdown/obsidian` 的渲染测试。
