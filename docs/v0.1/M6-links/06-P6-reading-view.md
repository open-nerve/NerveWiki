# M6/P6 阅读视图：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P6 阅读视图 |
| 状态 | 已完成：A（2026-10-06，合并 `4181768`）、B（2026-10-06，合并 `ccfc395`） |
| 基线 | P5 合并之后的 main（`b0e2668`）。分 A（服务端）、B（前端）两部分，照 P3 的先例各开分支（`m6-p6a`、`m6-p6b`），各自审查、合并；B 在 A 合并之后开始 |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.2、4.4、4.8、4.9、第 7 节 P6、第 11 节第 2 项；[P3 文档](03-P3-index.md)第 2 节（落点）、6.1、6.2、6.10；[P5 文档](05-P5-api.md)；[M4/P3 移交](handoffs/M4-P3-markdown-extensions.md)第 11 项；[M5 的事件](handoffs/M5-events.md)；[总体设计](../v0.1-design.md) 4.6、13.1、13.2 |

---

## 0. 梳理

作者先请两位 Opus 只读梳理（2026-10-06）：

- 一位读前端：阅读视图的增强与组合根、路由、新建页面、对话框、事件与重连、生成的客户端；在自己的克隆里装上 KaTeX 与 mermaid，在生产的 CSP 头下用 Chromium 实测。
- 一位读服务端：解析与落点、页面的新建、标签与属性表的渲染、只有锚点的链接、CSP；写探针测试量落点的几种写法。

两人得出同一个结论：P6 不只是前端。落点没有实现，属性表没有钩子（P1 推到 P3，P3 推到 P6），标签还是 `<span>`，`[t](#H)` 照旧写 `href="#H"`。Phase 表里的 P6 一项也没写这些，所以照 P3 分成 A、B（第 15 节）。

## 1. 范围

**A（服务端）**：

- 落点：`domain.Landing` 与接口 `getLinkLanding`（第 2 节）。
- 标签渲染为链接（第 3 节）。
- 属性表的钩子与表里的链接（第 4 节）。
- 只有锚点的 Markdown 链接按标题的 id 写地址（第 5 节）。
- 宽的内容：块公式与属性表也由服务端包 `nw-scroll`，`tabindex` 改由前端给（第 6 节）。
- 契约 `getPageView` 的描述随之修订。

**B（前端）**：

- 未建的链接：按钮、键盘，写者确认之后在落点新建，读者得到说明（第 7 节）。
- 链接的增强一般化：标签、属性表里的链接、本站完整地址（第 8、9 节）。
- 标签页的路由（第 8 节）。
- KaTeX 与 mermaid 的增强、chunk、上限，CSP 下实测（第 10 节）。
- 宽内容的可访问名称与滚动（第 6 节）；标签、callout、高亮、嵌入、公式与图的样式（第 12 节）。
- 事件与重连时的重读（第 11 节）。

**不做**：

- 右栏（大纲、反链、属性）与补全：P7。
- 全部标签的总览页（`listTags`）：交给 M12 的打磨（第 15 节，负责人可以改判）。
- frontmatter 的 `tags` 在属性表里显示为标签链接：同上。
- 附件的嵌入：M7。

## 2. 落点（A）

**问题**：一条解析不到的链接，写者点击之后新建的页放在哪里、叫什么，使这条链接新建之后解析到它（总设计 4.9、第 11 节第 2 项）。点击时由服务端算，不写进 HTML（P3 B：落点变了不改任何解析、不发事件，写进去会过时）。

**P3 文档第 2 节的规则不够**："父页按同样的规则解析"有两处会新建出解析不到它的页（梳理的探针）：

- 路径的前段经别名找到父页（`[[Al/x]]`，`Al` 是 `P` 的别名）：别名只用于单名，新建的 `P/x` 不被 `Al/x` 解析到；
- 段里有空格（`[[B/ x]]`）：`CheckTitle` 去掉首尾空白，标题键不去，新建的 `x` 不被 ` x` 解析到。

**`domain.Land(t, from, candidates, parents, aliased, maxDepth)`**，答 `Landing{Node, Parent, Title, Reason}`：已解析到的节点、落点、或没有落点的原因之一。`candidates` 是键为最后一段的页，`parents` 是键为前一段的页，`aliased` 是单名的别名指向的页（实施时的写法）：

1. 现在就解析得到（两次读之间别人新建了）：答那个节点，前端直接去。
2. 标题：写的最后一段，去掉 `.md`（同 Obsidian），经 `shared.CheckTitle`；不合法答 `title_invalid`。`Target` 加字段 `Name`（写的最后一段）：现在的 `Target` 只留标题键。
3. 父页：
   - 单名：出发页的父节点（根上的页即笔记本的根）；
   - 相对的：上溯 `Up` 层的文件夹，再按写的前段逐段精确匹配；
   - 根路径：从根逐段精确匹配；
   - 其余的路径：用前段的标题键（`t.Keys[:n-1]`，不重新切分：`A.md/x` 里的 `.md` 不是后缀）按解析的前三步（相对、从根、后缀）找，**不用别名**。
   - 父页就是前段解析到的页：它已经在最深一层时答 `too_deep`，不另找子树之外放得下的同名页（审查 r1-1，第 15 节）。
   - 找不到答 `parent_missing`。
4. 深度：父页的深度加一超过 `MaxDepth`（页面模块导出）答 `too_deep`。
5. **后置条件**：把假想的新页加进候选，`Resolve` 必须只解析到它（不歧义），否则答 `not_resolvable`。前两种情形里，别名的前段在第 3 步已答 `parent_missing`（父页不用别名），段里的空格由这一步拦下；规则以后变了也拦得住（审查 r1-3 修订）。新页的 id 取最大的 UUID，同分时输给 id 最小的页，所以"解析到它"已含"只解析到它"。
6. 切分失败、含 NUL 或不是合法 UTF-8 的目标答 `target_invalid`，不查询（否则 `ByKeys` 答 500）。这一条在用例里判定，`Land` 只收切分好的目标。
7. 别名只为单名读（`Target.ByAlias`：一段、不是相对的、不从根；`Resolve` 也按它决定用不用别名，审查 r1-4）。

**接口** `GET /api/v0/pages/{page_id}/link-landing?target=…`，操作 `getLinkLanding`，契约在 `api/modules/linking.yaml`：

- 回答 `{node_id, landing, reason}`，三者恰有一个不为 `null`：`landing` 是 `{parent_id, title}`（`parent_id` 为 `null` 是根）；`reason` 的枚举是上面五种。
- 动作 `link_landing.read`，笔记本级，`writers()`：落点是写的一半，读者用不上，前端不为读者问。
- 次序：页面不可见答 404 `page.not_found`；读者答 403 `forbidden`；`target` 缺失或长于 4096 字节答 422 `validation_failed`。"没有落点"是 200，不加新码（照 P5 的 `getTag`）。
- `target` 是自由文本的查询参数：契约测试 `TestFreeTextParametersDoNotAnswer5xx` 会发 NUL 与 `\xff`，但那时页面不存在，只到 404；含 NUL、`\xff` 的目标由整个程序的测试（`links_landing_test.go`）核对。
- `target` 在契约里是可选的参数，缺失的 422 由用例给出（码 `required`，同其他 422）。
- 读不开事务、不取锁（总体设计 8.3）。

**新建**：前端在确认之后调已有的 `createPage(parent_id, title)`，不加新的写入口（13.1 第 1 条）；它的检查、码、事件与仓库照旧。两次调用之间的竞争：

- 父页删了：422 `parent_id`；标题撞了：409 `page.title_taken`，前端再问一次落点（多半已解析到）；太深：409 `page.too_deep`；角色降了：403。都在对话框里说明。
- 出发页移动、路径的父页改名：在旧的落点新建，链接可能仍解析不到。罕见、看得见，接受。
- 新建之后观察者重新解析，`links` 事件让出发页的阅读视图重读，链接变成已解析。

**用例** `linking/app.GetLinkLanding`：`Access.page`（同 P5）、写者的判定，再按目标最后一段与前一段的键读候选与父页（`PageTree.ByKeys`），单名时读别名（`Reads.Aliases`，实施时加进 `Reads`），最后读出发页与别名页的路径（`PageTree.Paths`）。不复用 `resolutions`：那是一批链接、取锁的读，这里是一个目标、带父页的键、只为单名读别名（审查 r3，不合并）。`linking.Deps.Pages` 要 `ByKeys`、`Paths`：`PageTree` 加这两项（组合根已经传入完整的 `linkTargets`）；`linking.Deps.MaxDepth` 由组合根给页面模块的 `MaxDepth`。

## 3. 标签（A）

- 计数的标签（`obsidian.CountedTag` 接受的）渲染为 `<a class="nw-tag" data-nw-tag="名">#名</a>`，`名` 是 `CountedTag` 给出的这一页的写法（去掉末尾一个 `/`），不是键：前端不折叠大小写，`getTag` 在服务端算键。嵌套的标签整个写（`a/b`）。
- `getTag` 的 `{tag}` 按 `listTags` 列出的写法取键：`TagKey(名)` 是 `CountedTag(名 + "/")`。P5 对 `#a//`（记为 `a/`）又去掉一个 `/`，读成 `a`（P5 的遗漏，审查 r2-L3）。
- `CountedTag` 不接受的（`#/`、`#1/`）照旧是 `<span class="nw-tag">`：索引里没有它们，点了也没有页。标签 `/`（`#//`）也是 span：路由把单独的 `%2F` 读成末尾的斜杠，`getTag` 的路径叫不到它（修复核对 f1-F1）；`listTags` 仍列出它，契约写明。
- 一页第 1000 个之后的标签、键超过 1024 字节的标签仍是链接，`getTag` 不列这一页（P5 的上限），契约写明。
- 标签与 wikilink 的节点实现 `markdown.Linker`，它改为按节点问的 `RendersLink() bool`：渲染成链接的，净化丢掉用户围着它的 `<a>`（P3 B）；渲染成 span 的不丢（审查 r2-L2）。
- Markdown 链接的文字里的标签是 `<span>`：`linkedWikilinks` 这一变换（`obsidian/view.go`，优先级 40）改名 `inLinks`，一并标出链接里的标签。
- `Markup`：`a` 的属性加 `data-nw-tag`。

## 4. 属性表里的链接（A）

**现在**：`writeProperties`、`writeValue`（`platform/markdown/render.go`）把值写成转义的文字，没有钩子；属性链接已经在 `Resolve` 的输入里（P3）。

**歧义**：`{"a.b": "[[P]]", a: {b: "[[Q]]"}}` 的两个值路径都是 `a.b`（P5 文档第 4 节）。表里按**值的身份**对齐，不按路径，歧义就不存在：

- `markdown.Scalar` 加一个序号（不导出），读 frontmatter 时按文档的次序给每个字符串值编号（经锚点引用的也算一次，与写表的次序一致）。
- 平台的扩展加钩子 `Properties func(data any) func(Scalar) ([]Attr, string, bool)`（实施时的形状）：写表时按同样的次序把每个字符串值交给它，扩展答"写成链接"（属性与文字）或不管；先答的扩展算数。
- obsidian 在渲染时用抽取的同一个解析器重新解析这个值（属性链接本来只在抽取里），是指向页的链接才答应：wikilink 要有目标（`[[#h]]` 是文字），Markdown 链接要是抽取留下的。属性同正文的链接（`class="nw-wikilink"`、`data-nw-node`、`data-nw-anchor` 或 `nw-unresolved`、`data-nw-target`），文字是链接显示的文字：wikilink 的显示文字或名称，Markdown 链接的文字照 goldmark 写出的文字（`markdown.ShownText`：转义与字符引用解出、代码原样、自动链接写标签、U+0000 写 U+FFFD；审查 r2-L1、修复核对 f2-L1）。正文里图片的文字也改用它；标题的 id 仍由源文算（`PlainText`），不变。
- 属性链接的文字不经净化：Markdown 链接文字里原始 HTML 的标签不写，标签之间的文字照样显示（`<script>b</script>` 显示 `b`，转义一次），正文里净化连文字一起去掉（修复核对 f2-L2，接受）。
- 这改了 M4 的扩展点（核心属性表的钩子），记进总设计第 8 节。
- 病态输入：9000 个属性链接（YAML 一个值的上限之内，约 512 KB）进 `CheckCosts`、`CheckSize`；`ShownText` 的分配另由"引用很多次的图片"钉住（修复核对 f2-M1）。
- P5 的 `links: [{key, node_id}]` 按路径，歧义仍在：交给 P7 的右栏决定（第 15 节）。

契约 `getPageView` 的"属性是文字"改为写明链接、标签链接与锚点的地址。

## 5. 只有锚点的链接（A）

- `[t](#Heading%20Two)` 现在写 `href="#Heading%20Two"`，标题的 id 却是 `nw-heading-two`，点了不动（P3 文档 6.10）。
- 核心的 `marks`（写链接的地方）：地址以 `#` 开头时，按百分号解码，取最后一个 `#` 之后的部分，写 `#` 加 `HeadingID` 的结果：`AnchorID(DecodeURI(锚点))`，两个函数从 obsidian 移进核心，wikilink 的锚点用同一个。标题的 id 本来就是核心算的，地址也由核心写。
  - `^` 开头的块引用、空的锚点（`[t](#)`）：只写文字，不写链接（同 `[[#^b]]`）。
  - 只有锚点的图片不变，照旧写原地址（审查 r2-S1，接受）。
  - 用户写的 `#nw-…` 也经同一个函数，与 `[[#…]]` 一致。
- 与 goldmark 逐字节对照的测试比的是 goldmark 自己的渲染器，不经 `marks`，不受影响。

## 6. 宽的内容（A、B）

**A**：

- 块公式（`div.nw-math-block`）与属性表（`.nw-props`）由服务端包 `<div class="nw-scroll">`，同表格：增强不改结构（13.2 第 23 条）。mermaid 的图由增强包（总设计 4.9 已写明的例外）。
- `tabindex` 不再由服务端写：不溢出的包装不该是一个 Tab 停留点，服务端不知道是否溢出。`CheckHTML` 不让 `div` 带 `tabindex`。
- e2e PG5 里钉住 HTML 形状的两处（属性表是文章的第一个子元素）随 A 改为包装里的表，否则 A 的 e2e 失败；滚动的断言仍是 B 的。

**B**：

- 增强 `scrollRegions` 跟着每个 `.nw-scroll`、`pre` 与段落里的展示公式（`span.nw-math-block`：没有包装，自己横向滚动）：溢出时给 `tabindex="0"`、`role="region"` 与名称（"表格"、"公式"、"图"、"代码"、"属性"，认不出的叫"宽的内容"；i18n，经 `ReadingContext.t`），不溢出时去掉。图里只有包装算，标签里的是写者的标记。宽度随窗口变（`ResizeObserver`），也随增强渲染的内容变（`MutationObserver`：只看变更所在处外层的滚动区与新加的，审查 r3-6）；后来加的包装（mermaid）也跟着。代替只管 `pre` 的 `scrollFocus`。
- 横向滚动从整个阅读视图移到这一层（`reading.css`）；e2e PG5 断言滚动的是包装，不是文章。
- 文章仍横向滚动没有自己滚动区的宽内容（用户自写 HTML 的表格、行内很长的公式），溢出时得到 `tabindex="0"`（不加角色与名称：它是页面的 article，名称是页名），并裁掉公式画到外面的部分（负的 `\kern`），不盖住应用（审查 r3-2、r3-3）。内边距 0.25rem 放下焦点环，负的外边距保持位置。
- 文章与滚动区都不纵向滚动：公式的支柱超出底部时会接走滚轮（`overflow-y: hidden`，修复核对 f2-4）；锚点的 `scrollIntoView`、页内查找仍能滚动隐藏的溢出，读者却滚不回来，`scrollRegions` 在捕获阶段听 `scroll`，把文章与它跟着的滚动区的 `scrollTop` 复原为 0，窗口滚动同样的距离：锚点带到眼前的仍在眼前（第二、三轮核对）。用户的样式表设了 `scroll-behavior: smooth` 时窗口滚不够，接受（第四轮核对）。
- 宽的布局里工作区的侧栏与页面一样高，不作窗口的滚动锚点（`overflow-anchor: none`）：否则页面里上方的变化（排好的公式、画好的图）从不调整滚动，带片段打开的标题落到视野外（第四轮核对）。重读时公式立即放回，高度不变（第 10 节），锚定不会把勾选的方框移出视野（第五轮核对）。

## 7. 未建的链接（B）

**哪些可以新建**：`a.nw-unresolved`，不是嵌入（`nw-embed`），不在 Markdown 图片的链接里。嵌入与图片只说明不存在（总设计 4.9）。附件那样的名称（`[[report.pdf]]`）在 M7 之前照样新建为页（同 Obsidian 新建同名的笔记），M7 让落点拒绝读作附件的目标（第 15 节）。

**增强** `unresolvedLinks`：

- 给这些链接 `role="button"`、`tabindex="0"`、`aria-haspopup="dialog"`（读者也得到对话框，审查 r2）。点击、Enter（按下时）、空格（在按下的那条链接上抬起时，同按钮；按下时 `preventDefault`，不滚动页面；审查 r2、修复核对 f1-2）调 `ReadingContext.unresolved({target, kind, element})`。图里的链接是 mermaid 的，不管（第 10 节）。
- `ReadingContext` 加 `unresolved`：阅读视图持对话框，增强不导入 `app/`（`reading/` 的导入规则）。

**写者**：

1. 问落点（`getLinkLanding`），其间链接标为忙（`aria-busy`）；之前的拒绝提示清掉，同"编辑"与勾选（审查 r2-2）。
2. 已解析到：经路由去那一页；本标签页的树还没有它（别的标签页刚建的）时先读树（审查 r2-1）。
3. 有落点：确认对话框"新建页面「标题」？"，说明写"在「父页的路径」下"或"在笔记本的根上"。`ConfirmDialog` 加不是破坏性的样式。确认中按钮禁用（防两次新建）。
4. 确认：`createPage`，成功后去新页（`arrived`，同"新建页面"），并让出发页的视图重读（回来时链接已解析）。
5. 失败（第 2 节的竞争、离线）：对话框里经 `errorText` 说明；`page.title_taken` 时再问一次落点，已解析到就去那里。
6. 没有落点：一个按钮的说明对话框，按原因写（"「x」不能作为页面的标题"、"路径里的页不存在"（`parent_missing` 不说是哪一段）、"太深了"、"新建之后也不会链接到它"）；目标长于服务端所收（422）或代理所收（414）的，说明目标不合法。

**读者**：说明对话框"页面「x」不存在"，不问服务端。

**焦点**：对话框关闭时焦点回到那条链接；重读替换了它时按 `data-nw-target` 与它在同名链接里的序号找回，找不到就落在文章上（M5 打磨移交第 8 项的同一个问题）。

## 8. 链接的增强与标签页（B）

**链接的增强**：P3 的 `pageLinks` 只管 `a[data-nw-node]`。一般化为 `appLinks`：

- `a[data-nw-node]`：页面的地址（带锚点的加 `#nw-…`），同 P3；属性表里的链接一样。
- `a[data-nw-tag]`：标签页的地址。
- 本站完整地址（第 9 节）。
- 点击在容器上委托，修饰键与中键照浏览器（地址是真的）；mermaid 图里的 `<a>` 也经它（读 `href` 或 `xlink:href`），只按地址：图里的不当作服务端的标记（审查 r1-2）。

**标签页**：路由 `/:workspace/notebooks/:id/tags/:tag`，整个名字编码成一段（`encodeURIComponent`；A 的移交）。React Router 把参数里的 `%2F` 读回 `/`，`a/b`、`a/`、`a//b`、非 ASCII 的名字都原样往返（审查 r1 的探针）；标签 `/` 没有链接（第 3 节）。

- 标题 `#标签`，列出 `getTag` 的页面（含下层的标签），名称取自已加载的树，同名的照 `distinctName`；空时说明没有页面。
- 不分页（同 `getTag`）；SWR 的键 `["tag-pages", nb, tag]`。
- 文档标题同页面的写法（`#标签 · 笔记本`）；到达时标题拿到焦点。

## 9. 本站完整地址（B）

普通 Markdown 里写的本站完整地址（`https://wiki.example/acme/notebooks/…`）经路由跳转（M4/P3 移交第 11 项）：

- 同源；路径不以服务端的前缀开头（`/api/`、`/healthz`、`/readyz`、`/assets/`）；最后一段不带 `.`（静态文件）。前缀与文件按逐段解码之后的路径判断（同服务端）；以 `//` 开头的路径不经路由（路由当作别的站；审查 r1-1）。
- 路由表里的 `:slug` 与 `*` 几乎匹配一切，按路由表判断没有更多信息，所以用上面的规则。
- 服务端不变：没有"公开地址"的设置（邀请链接用的也是 `location.origin`）。

## 10. KaTeX 与 mermaid（B）

**版本**：`katex` 0.16.47、`mermaid` 11.17.2，精确版本。梳理的实测：mermaid 12.1 的流程图要带 1.46 MB 的 elk，且自带另一份 KaTeX（嵌进二进制的 `dist` 8.0 MB 对 6.1 MB；第一张流程图 2.2 MB 对 784 KB）。

**加载**：各自单独的 chunk，用到时 `import()`。构建检查 `build/out-of-main.ts` 的 `lazyLeak` 代替 `editor-out-of-main`，编辑器、KaTeX、mermaid 一起查：从入口沿静态与动态导入走，只有经动态导入到达的懒加载入口才跳过，静态导入照样跟进去（审查 r3-4）。加载器可以注入（照代码高亮的 worker），jsdom 里不真渲染。

**KaTeX** 增强 `math`：

- `.nw-math` 的原文在脱离页面的元素里用 `katex.render` 排：`trust: false`、`maxSize: 50`、`maxExpand: 1000`、`throwOnError: true`、`strict: false`（不对写者不标准的 TeX 出警告；KaTeX 对没有字形度量的字符（€、希伯来文、emoji）仍出一条自己的警告，只带那一个字符，接受，第三轮核对）；块的（`nw-math-block`）`displayMode`。
- 显示原文的：
  - 出错的，原文长于上限（4,000 字节）的；
  - 定义宏的：写了 `\def`、`\gdef`、`\edef`、`\xdef`、`\let`、`\futurelet`、`\global`、`\long`、`\newcommand`、`\renewcommand`、`\providecommand`，或带 `@` 的控制词（KaTeX 自己的，`\tag` 定义的 `\df@tag` 在内）。`maxExpand` 只数展开的次数，不数展开出来的长度：反复使用的宏让 290 字节的公式排 37 秒（审查 r3-1）。控制序列照 KaTeX 的词法读（`\\@` 是行尾加 `@`，修复核对 f2-6）；KaTeX 拼不出别的控制词（没有 `\csname`），内置的宏不重复参数；
  - 排出的元素嵌套深于 150 层的：Chromium 的布局在 800 层左右崩溃（135 层的 `x^{x^…}`，541 字节；修复核对 f2-2），远在这之前，层层带样式的组（`\pmb{`、`{\Huge `、`\textcolor`）让布局的时间比深度增长得快：489 层 1.3 秒，150 层最坏 129 毫秒（第二轮核对）。十层嵌套的分数深 76 层，30×30 的矩阵 21 层；
  - 写到读者控制台的：`\message`、`\errmessage`、`\show`。
- KaTeX 打了一个补丁（`patches/katex@0.16.47.patch`，`pnpm patch`）：`alignedat`、`alignat` 的列数超过 100 时报解析错误。它照参数造列，与行无关，`{30000000}` 四十几个字节就耗尽页面的内存，数字有许多写法（注释、`\char`、`\noexpand`），看原文拦不住（修复核对 f2-1）。升级 KaTeX 时重新核对。
- 每个任务排 50 毫秒，之间让出主线程（修复核对 f2-9）；计时连布局一起算，布局可能比 KaTeX 慢得多（第二轮核对）。
- 每个公式先在视图里一个流外、看不见的盒子（`nw-math-measure`）里单独布局，任务的计时算它：在段落里逐个布局时，每个都重排整段（一段 2,000 个 12.8 秒，第三轮核对）。一个任务的公式在任务结束时一起放回原处并布局。每个任务先布局视图里别的变化（别的增强改的），算进任务的时间，不算进公式的。
- 视图的公式在原处的布局至多 1 秒（`layoutBudget`，每个任务结束时量），之后的显示原文：视图重新布局（缩放、打印）时各公式在一个任务里一起布局，40 个四条 `\pmb{`×144 的公式缩放 5 秒、打印 10 秒（第三轮核对）；有了上限，排出 9 个，缩放 1.1 秒。预算是读者机器上的时间：慢的 CPU 上公式很多的页排出的少一些（4 倍降速时审查者 1,550 个公式的讲义有 325 个显示原文，我们的全部排出），在哪台机器上都只卡 1 秒左右（第四轮核对）。
- 打印不受预算限制：打印一次布局并绘制整页，预算只量布局。1 MB 不断行的长词、一个公式也没有的页打印 9.6 秒；绘制重的公式（成串的 `\boxed{\cancel{x}}`、`\pmb` 的阴影）在预算之内另加 2–5 秒。接受（第四轮核对），交给 M12。
- 撤销时排过的显示原文，没排到的不动，盒子拿掉。视图留下它的公式排出的样子（按容器）：在同一视图重新增强（重读：勾选任务、另一会话的保存）时，排过的原文立即放回，每份只用一次，其余的照常排；每个公式分得它那个任务原处布局的一份，预算接着放回的那些的份额算（第六轮核对）。排好的公式在 `data-tex` 里留着原文，任务的名称（`taskText`）读它，每次读都一样（第六轮核对）。重读前后高度不变，其间也不显示原文；否则公式先成原文（矮）再排好（高），窗口在页面里的锚点只补偿后一步，勾选的方框跑出视野（第五轮核对）。
- 实测：2,000 个普通公式，一段里、各一段、一张表里，都在 0.7–0.8 秒里全部排出，最长的任务约 90 毫秒，缩放 0.2 秒；首次加载约 288 KB（JS、CSS、字体）。

**mermaid** 增强 `diagrams`：

- `pre > code.language-mermaid`：`securityLevel: 'strict'`、`suppressErrorRendering: true`、`maxTextSize` 20,000、`maxEdges` 200（450 条边在主线程 1.16 秒）。`maxEdges` 只限流程图的边。
- mindmap 的布局比节点增长得快（800 个子节点 14.6 秒，19.9 KB 的 2,000 个 116 秒）：超过 150 行（`mindmapLines`，一行一个节点，注释与首行也算）的显示原文，类型按 mermaid 自己的 `detectType`（第二轮核对）。行按 JavaScript 的行尾分（`\r`、`\n`、U+2028、U+2029）：mindmap 的注释 `%%` 读到行尾为止，之后就是下一个节点，U+2028 隔开的 1,000 个节点只算一行，画了 19 秒（第三轮核对）。
- 别的图在 20,000 字节之内要画几秒，接受（一次一张、看得见时才画，写进 M12 的打磨移交；负责人可以改判）：block 5.7 秒，architecture 4.1 秒，多层的 mindmap 5.0 秒，state 3.0 秒，class、kanban 2.7 秒，gitGraph 2.0 秒，3,300 个节点、没有边的流程图 2.0 秒，1,200 条转移的状态图 2.8 秒（审查 r3-5、第二轮核对）。
- 标签的 HTML 经 `dompurifyConfig` 不留 `<style>`（mermaid 自己的规则）、`id`（会抢走页面标题的锚点）、`data-*`（服务端的标记只会是服务端的：缓存的图先于其他增强放回，伪造的任务框会勾掉页里真的任务项；审查 r3-8、修复核对 f1-1）。标签里的链接随之丢了 `target`（mermaid 存在一个 data 属性里）：在本页打开，同页面自己的链接。`secure` 加 `layout`、`dompurifyConfig`：图自己的指令改不了它们。
- 标签里的 `$$…$$` 由 mermaid 自己调 KaTeX 的 `renderToString`。加载 mermaid 时一并加载 KaTeX，把它换成同一道拦截与选项（`guardLabels`）：它收到的是 mermaid 净化之后的标签，看原文拦不住（`\d<x></x>ef` 到那时是 `\def`，修复核对 f2-3）。长于 4,000 字节的、排出来（解析进 `<template>` 量）嵌套深于 150 层的也拒绝（第二轮核对）。拒绝时这张图画不出，显示原文。
- `maxTextSize` 超出时 mermaid 不报错，而是画一张替身图：增强自己先比长度（20,000 字节），超出、出错时显示原文。
- 只在可见时渲染（`IntersectionObserver`，提前 200px），各视图一共一次一张。元素的 id 以 `nw_mermaid_<n>` 开头（标题"mermaid 0"的 id 是 `nw-mermaid-0`，不能撞），每次放进视图都换一个新的（`withOwnIds`），同一原文的两张、缓存的不撞（审查 r3-7）。时序图、桑基图有几个 id 不以它开头，同一页两张这样的图仍会重复（mermaid 自己的，交给 M12）。
- 代码高亮跳过 `language-mermaid`。
- 每次重读都换掉整个 HTML：按（主题、原文）缓存渲染出的 SVG，重读时原文没变就直接放（模块级，至多 50 张，最久没用的先丢）。视图撤销之后才画完的也存，同一原文的两张只画一次（审查 r3-9）。
- 主题：`ReadingContext.theme()` 是当前显示的主题，`onThemeChange` 在它变时通知（视图用 MobX 的 reaction 给）。换主题不再重新增强：每张图看得见时在原来的包装里重画，画好之前旧图留着，画的时候才读主题（审查 r1-3）。换语言仍重新增强：名称与标签用它，读者从菜单里换，焦点在菜单上。

**CSP**：梳理在生产的头下实测，十余种图、公式、错误输入、`%%{init: securityLevel: loose}%%` 都没有违规，不需要 `unsafe-eval`，字体可以载入。e2e 的每个页面一遇 CSP 违规就失败，L4 一并核对；结果写进总体设计 4.6（"具体写法在 M6 确定"）。

结果：CSP 不改。KaTeX 的 `style` 属性与字体、mermaid 的 SVG 里的 `<style>` 在 `style-src 'self' 'unsafe-inline'`、`font-src 'self' data:` 之内；callout 的图标是 `data:` 的 SVG 遮罩，`img-src` 放行 `data:`。审查者在 Chromium 里用十一种恶意的图探过（指令、`click … javascript:`、带处理器的标签、`javascript:` 链接、`<style>`、`position: fixed`），没有违规，标签里的东西出不了图的包装。

## 11. 事件（B）

- `pages` 事件：树变了或笔记本里有页写过时，重读这个笔记本的标签页（`["tag-pages", nb, *]`）。
- `links` 事件 `pages: null`（`reindex` 之后）：同上，并照旧重读全部阅读视图。
- 重连：`["tag-pages"]` 加进与 `["pages"]` 同一级的重读。
- 新建之后让出发页的视图重读（第 7 节）。
- 属性、反链、补全的数据随 P7。

## 12. 样式（B）

M6 的目标 1 是"看起来像 Obsidian"，各 Phase 都没有认领这些样式，P6 认领：

- 标签：小的圆角块，可聚焦时有焦点环（保持圆角）；
- callout：按类型的颜色与图标（lucide 的 SVG 作 CSS 遮罩，`data:` 地址），可折叠的 `details`；
- `<mark>`、嵌入（`nw-embed`）、未建的链接（已有）；
- 公式与图在暗色下可读（KaTeX 用当前的文字颜色，mermaid 用暗色主题）；
- `.nw-scroll` 的横向滚动。

## 13. 模块与组合

**A**：

- `linking/domain`：`Target.Name`、`Landing`；`linking/app`：`GetLinkLanding`；`linking/adapter/http`：一个操作；动作 `link_landing.read` 进 `Actions()` 与 access 的规则表（`writers()`）。
- 页面模块导出 `MaxDepth`；`linking.Deps.Pages` 的端口加 `ByKeys`、`Paths`。
- `platform/markdown`：`Scalar` 的序号、`Properties` 钩子、`nw-scroll` 包装、锚点地址；`obsidian`：标签的链接、`Linker`、属性链接的回答、`Markup`。
- 权限矩阵一行（写者 200，读者 403，看不见的 404）；`targetViolation` 不用改（`{page_id}` 已认得）。

**B**：

- `services/linking.service.ts`（落点、标签下的页）；经每个笔记本的 `PageTreeStore` 露出。
- `reading/`：`appLinks`（代替 `pageLinks`）、`unresolvedLinks`、`scrollRegions`（代替 `scrollFocus`）、`math`、`diagrams`；经 `main.tsx` 的 `readingEnhancements` 登记。
- `ReadingContext` 加 `unresolved`、`t`（名称的文字）、`theme()` 与 `onThemeChange`。
- `build/out-of-main.ts` 代替 `build/editor-out-of-main.ts`。
- 依赖的补丁：`patches/katex@0.16.47.patch`（`pnpm-workspace.yaml` 的 `patchedDependencies`；镜像的构建先复制 `patches/` 再安装）。
- 路由加标签页；`events/handlers.ts`、`app/event-stream.tsx` 加标签页的重读。

## 14. 测试

**A**：

- `domain`：`Landing` 的表格测试（每种写法、别名、空格、`.md`、`A.md/x`、太深、根、相对到根之上）；随机测试：随机的树与目标，有落点的新建之后一定只解析到它（复用改写的随机树）。
- `app`：替身；次序（404、403、422）；不合法的目标不读；已解析到的情形。
- `http`：每个声明的码（`apitest`）。
- 渲染：标签、属性表的链接（含 `a.b` 的歧义）、锚点、包装的金样；病态输入进 `CheckCosts`、`CheckSize`；`CheckHTML` 与模糊测试的种子。
- 整个程序：落点 → `createPage` → 阅读视图里链接已解析、`links` 事件；权限矩阵；自由文本参数不答 5xx。

**B**（vitest，经组合根的表）：

- `appLinks`：三种链接的地址与路由跳转、修饰键、本站地址的判定（前缀、静态文件、别的源）。
- `unresolvedLinks`：角色、键盘、写者与读者、嵌入与图片、焦点回到链接（含重读替换之后）。
- 对话框：确认、取消、失败的说明、`title_taken` 之后再问、没有落点的各种原因。
- 标签页、事件的重读、重连。
- `math`、`diagrams`：注入的加载器；上限、出错显示原文、可见时才渲染、一次一张、缓存、主题；定义宏的、列数过多的、嵌套过深的公式（按应用加载的 KaTeX），`guardLabels`。
- `scrollRegions`：溢出时才可聚焦、名称。
- 构建检查：KaTeX、mermaid 不在主 chunk。

**e2e**：

- L2：写者点击未建的链接，确认之后新建并打开，回来时链接已解析；读者得到说明。
- L4：标签链接到标签页；公式与图渲染（CSP 之下，页面的固件一遇违规就失败），KaTeX 的样式表在，换主题时图原地重画、焦点不动。第二个故事：写者的公式与图对读者的页做不到的（撑宽页面、画到外面、无尽展开、让标签页崩溃、伪造服务端的标记）。第三个故事：视图的公式布局至多 1 秒，之后的显示原文。第四个故事：在 20 个矩阵之下的标题打开页面，公式排好之后标题仍在视野里；勾选其下带公式的任务，重读时公式立即放回，方框不动、有焦点、名称不变。
- PG5 随第 6 节修订。

**负对照**：每步的关键分支做变异，记在第 18 节。

## 15. 与总设计的出入

（本文定稿，总设计随 A、B 的合并修订。）

- **第 7 节 Phase 表**：P6 分 A（服务端）、B（前端）。
- **4.9**："没有落点的"补上太深、目标不合法、新建之后也解析不到；落点只给写者。
- **P3 文档第 2 节**：落点的规则改为本文第 2 节（父页不用别名、后置条件）。
- **4.9 宽的内容**：`tabindex` 由前端按是否溢出给；块公式与属性表由服务端包。
- **第 8 节扩展点**：核心属性表的钩子 `Properties`（改了 M4 的扩展点）；`Linker` 改为 `RendersLink() bool`；`WriteAttrs` 写进 `markdown.Writer`。
- **4.9 KaTeX、mermaid**：版本、上限的数值、只在可见时渲染与缓存；定义宏的公式不排、KaTeX 的补丁、嵌套的上限；标签的净化与标签里的公式；跟随主题。
- **总体设计 4.6**：CSP 的实测结果（不改）。

**负责人可以改判**（作者的判断）：

- 全部标签的总览页、frontmatter 的 `tags` 显示为标签链接，交给 M12 的打磨。
- 附件那样的名称在 M7 之前照样新建为页。
- 落点只给写者；读者不问服务端，只说明不存在。
- mermaid 12 不用；`maxEdges` 200、原文 20,000 字节、公式 4,000 字节的上限。
- 属性表按值的身份对齐，右栏按路径的歧义交给 P7。
- 父页在最深一层时答 `too_deep`，不另找放得下的同名页（第 2 节，审查 r1-1）。
- 只有锚点的图片照旧写原地址；`[t](#)` 只写文字（第 5 节）。
- 属性链接文字里原始 HTML 标签之间的文字照样显示，不经净化（第 4 节）。
- 定义宏的公式（`\def`、`\newcommand` 等）显示原文，不排版：宏的展开没有上界。要支持，得把 KaTeX 放进带时限的 worker（第 10 节）。
- KaTeX 打补丁限住 `alignedat` 的列数（100）；公式排出的元素至多嵌套 150 层；视图的公式布局至多 1 秒，之后的显示原文（慢的机器上排出的少一些）；打印不受它限制；mindmap 至多 150 行。
- 别的图在 20,000 字节之内画几秒（第 10 节的实测），接受。
- 换语言仍重新增强整个视图（第 10 节）。

## 16. 实施步骤

| 部分 | 步 | 内容 |
|---|---|---|
| A | S1 | `linking/domain`：`Target.Name`、`Landing`；表格与随机测试 |
| A | S2 | 用例、契约、生成、HTTP、动作与规则表、端口、页面模块导出 `MaxDepth`；权限矩阵；整个程序的测试 |
| A | S3 | 标签的链接、`Linker`、链接里的标签、`Markup` |
| A | S4 | `Scalar` 的序号、`Properties` 钩子、属性表里的链接；病态输入 |
| A | S5 | 锚点的地址；块公式与属性表的包装，`tabindex` 去掉；契约 `getPageView` 的描述 |
| B | S1 | `appLinks`（标签、属性表、本站地址）；标签页的路由、服务、事件与重连 |
| B | S2 | `unresolvedLinks`、`ReadingContext.unresolved`、对话框、落点的服务；`ConfirmDialog` 的样式 |
| B | S3 | `scrollRegions`、`reading.css` 的滚动与样式；PG5 |
| B | S4 | KaTeX：依赖、chunk、构建检查、增强 |
| B | S5 | mermaid：增强、可见时渲染、缓存、主题；代码高亮跳过 |
| B | S6 | e2e L2、L4；CSP 的结果 |

## 17. 完成标准

- 第 14 节的测试通过；`make check`、`make gen-check`、e2e 全量通过。
- 审查的发现处理完，修复经 Opus 核对，直到一轮没有行为上的发现；`make image-smoke` 通过。

## 18. 结果

（A、B 合并时各自填写。）

### A（服务端）

- **提交**：S1 `0087b8e`、S2 `e2aae59`、S3 `7089453`、S4 `855b264`、S5 `dfbf0a4`；审查的修复 `7630c11`、`7c8c767`、`80ba307`；修复核对两轮，第一轮之后的修复 `459184b`，第二轮没有行为上的发现，只改契约的措辞（`7d1fba1`）。合并 `4181768`。审查记录：[P6A-reading-server-review.md](reviews/P6A-reading-server-review.md)。
- **与本文的出入**（已改进上文）：`Land` 的签名与 `parents`（第 2 节）；别名经 `Reads.Aliases`、只为单名读（第 2 节）；`linkedWikilinks` 改名 `inLinks`，`Linker` 按节点问（第 3 节）；钩子的形状、渲染时重新解析、`ShownText`（第 4 节）；`AnchorID`、`DecodeURI` 移进核心（第 5 节）；PG5 钉住形状的两处随 A 改（第 6 节）。另加了属性链接经整个程序的测试（`links_view_test.go`）。
- **测量**（作者的笔记本电脑，macOS、Apple 芯片）：
  - 成本（`CheckCosts`，对普通文档 512 KB 的倍数，上限时间 10 倍、分配 14 倍）：表里的 9000 个属性链接时间 1.0 倍、分配 1.6 倍；引用很多次的图片 5.5 倍、5.6 倍（`ShownText` 每次 4 KB 的缓冲时是 17.8 倍分配）。
  - 大小（`CheckSize`，上限 64 倍加 4 MB）：约 512 KB 的属性链接，全部解析到时 2.4 倍，都解析不到时 2.9 倍。
- **负对照**：S1–S5 52 个变异，50 个让测试失败，2 个等价（第 2 节第 5 步：新页的 id 取最大，"解析到它"已含"不歧义"；多给 `Resolve` 的父页按路径结尾匹配，不会被选中）。审查的修复 10 个、核对的修复 3 个，都让测试失败。
- **镜像**：合并之后 `make image-smoke` 通过（版本 0.1.0-dev，提交 `4181768`，`modified=false`）。
- **负责人可以推翻的决定**：第 15 节末的各条。
- **留给 B 与后续**：
  - 标签的路由要把整个名字编码成一段（`a/`、`a//b`；`encodeURIComponent`），标签 `/` 没有链接。
  - 前端发 `encodeURIComponent(target)`，`target` 就是视图里的 `data-nw-target`；长于 4096 字节的目标答 422，前端当作"没有落点"说明。4096 字节的目标编码之后可能超过 nginx 默认 8 KB 的请求行（已知的限制）。
  - `parent_missing` 不说是哪一段不存在：说明对话框只能写"路径里的页不存在"。
  - 段落里的展示公式（行内写的 `$$…$$`，`span.nw-math.nw-math-block`）不在块里，服务端没有包它：宽的时候由 B 的样式处理。
  - A 合并到 B 的 S1 之间，标签是没有地址的 `<a>`：点了不动（前端还没有它的增强）。
  - 同名的附件（M7）：落点答一个标题，`createPage` 答 409 `page.title_taken`，再问一次落点又是它；M7 让落点拒绝读作附件的目标时一并处理。
  - M12：能叫到标签 `/` 的路由（随标签总览，`{tag...}`）；标题的 id 丢了自动链接与原始 HTML 的文字（修复核对 f3-2）；属性链接文字里 `<script>` 之间的文字显示出来（f2-L2）。已写进 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)第 10 项。

### B（前端）

- **提交**：S1 `13c9ae6`、S2 `2cf062b`、S3 `e1e9ba3`、S4 `56bab99`、S5 `253b9c1`、S6 `5663112`；负对照补的边界测试 `035529d`；审查的修复 `0ab08f1`、`0c2babe`，七轮核对的修复 `5ebaa63`、`01bb601`、`40e4f53`、`cab48f8`、`39160c5`、`c5f8972`、`cb2ab7b`。合并 `ccfc395`。审查记录：[P6B-reading-front-review.md](reviews/P6B-reading-front-review.md)。
- **与本文的出入**（已改进上文）：
  - 标签页的路由是 `tags/:tag`，整个名字一段（第 8 节）；`ReadingContext` 另加 `t`、`theme()` 与 `onThemeChange`（第 13 节）；`aria-haspopup` 给每条未建的链接（第 7 节）。
  - 滚动区多了段落里的展示公式与"宽的内容"；文章仍兜底横向滚动、裁掉画到外面的部分；什么都不纵向滚动（第 6 节）。
  - KaTeX 与 mermaid 的上限比本文原来写的多得多：定义宏的、写控制台的公式不排，KaTeX 的补丁，嵌套 150 层，按时间连布局分批，各自在流外布局、视图的公式布局至多 1 秒；标签的净化、标签里的公式经同一道拦截、同一原文的图各有 id、mindmap 至多 150 行；换主题原地重画（第 10 节）。审查与七轮修复的核对一次次找到几十个字节就能卡住或弄崩读者标签页的输入，见审查记录。
  - 构建检查改名 `out-of-main`，一并查编辑器、KaTeX、mermaid，静态导入照样跟进去（第 10 节）。
  - 重读时排过的公式立即放回，高度不变；工作区的侧栏不作窗口的滚动锚点：带片段打开的标题、勾选的方框在公式排好之后都留在原处（第 6、10 节）。
  - callout 的图标是 lucide 的 SVG 遮罩（第 12 节）。
- **测量**（作者的笔记本电脑，macOS、Apple 芯片，Chromium）：
  - 2,000 个普通公式（一段里、各一段、一张表里）0.7–0.8 秒全部排出，最长的任务约 90 毫秒，缩放 0.2 秒；1,550 个公式的讲义 0.8 秒，4 倍降速时 3.7 秒，都全部排出；一个 4,000 字节之内、嵌套 150 层的公式布局最坏 129 毫秒，视图的公式布局至多 1 秒。
  - 修之前：290 字节的宏炸弹排 36.7 秒；45 字节的 `alignedat` 与 135 层的嵌套让标签页崩溃；19.9 KB 的 mindmap 116 秒，U+2028 隔开的 9 KB 的 19 秒；40 个带样式的深公式缩放 5 秒；一段 2,000 个公式 12.8 秒。
  - 图在 20,000 字节之内画几秒（第 10 节的表），接受。
  - `scrollRegions` 在 2,000 个表格、代码、公式的页上：每批变更全部重读时脚本 5.0 秒，只看变更处之后 0.12 秒（审查者的测量）。
- **e2e**：L2、L4（四个故事）新增，PG5 随第 6 节修订；全量在 CI 上通过。
- **负对照**：S1–S6 56 个变异，全部让测试失败（四处边界是这时补的，`035529d`）；审查的修复 37 个，核对的修复 62 个，全部让测试失败（其中 20 个跑 e2e L4 或真实的构建）。
- **镜像**：合并之后 `make image-smoke` 通过（版本 0.1.0-dev，提交 `ccfc395`，`modified=false`）；镜像的构建先复制 `patches/` 再安装依赖。
- **负责人可以推翻的决定**：第 15 节末的各条，新加：定义宏的公式不排；KaTeX 的补丁与 150 层；视图的公式布局至多 1 秒，打印不受它限制；mindmap 至多 150 行，别的图画几秒；换语言重新增强。
- **留给后续**：
  - P7：右栏（大纲、反链、属性）与补全（已完成，见 [P7 文档](07-P7-editor-panel.md)）。P7 另改了阅读视图两处：去锚点、勾选的任务框拿回焦点之前，先打开它所在的、关着的折叠 callout（P7 审查 r3-6、修复核对 fc2-c2-4）；读者做了什么（`readersInput`）挪进 `pages/page/readers-input.ts`，锚点的等待与反链的焦点共用 `watchReader({ on, acts })`。
  - M12：[打磨移交](../M12-release/handoffs/M5-polish.md)第 11 项（时序图、桑基图的 id，大图画几秒，定义宏的公式，换语言，对话框开着时的重读，mermaid 的临时元素，KaTeX 的字形警告，打印绘制重的公式）。
  - 升级 KaTeX 时重新核对补丁（`pnpm-workspace.yaml` 写明）。
