# M6/P6 阅读视图：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P6 阅读视图 |
| 状态 | 设计定稿，实施中 |
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

**`domain.Landing(t, from, candidates, aliased)`**，答已解析到的节点、落点、或没有落点的原因之一：

1. 现在就解析得到（两次读之间别人新建了）：答那个节点，前端直接去。
2. 标题：写的最后一段，去掉 `.md`（同 Obsidian），经 `shared.CheckTitle`；不合法答 `title_invalid`。`Target` 加字段 `Name`（写的最后一段）：现在的 `Target` 只留标题键。
3. 父页：
   - 单名：出发页的父节点（根上的页即笔记本的根）；
   - 相对的：上溯 `Up` 层的文件夹，再按写的前段逐段精确匹配；
   - 根路径：从根逐段精确匹配；
   - 其余的路径：用前段的标题键（`t.Keys[:n-1]`，不重新切分：`A.md/x` 里的 `.md` 不是后缀）按解析的前三步（相对、从根、后缀）找，**不用别名**。
   - 找不到答 `parent_missing`。
4. 深度：父页的深度加一超过 `MaxDepth`（页面模块导出）答 `too_deep`。
5. **后置条件**：把假想的新页加进候选，`Resolve` 必须只解析到它（不歧义），否则答 `not_resolvable`。前两种情形由这一步拦下，规则以后变了也拦得住。
6. 切分失败、含 NUL 或不是合法 UTF-8 的目标答 `target_invalid`，不查询（否则 `ByKeys` 答 500）。

**接口** `GET /api/v0/pages/{page_id}/link-landing?target=…`，操作 `getLinkLanding`，契约在 `api/modules/linking.yaml`：

- 回答 `{node_id, landing, reason}`，三者恰有一个不为 `null`：`landing` 是 `{parent_id, title}`（`parent_id` 为 `null` 是根）；`reason` 的枚举是上面五种。
- 动作 `link_landing.read`，笔记本级，`writers()`：落点是写的一半，读者用不上，前端不为读者问。
- 次序：页面不可见答 404 `page.not_found`；读者答 403 `forbidden`；`target` 缺失或长于 4096 字节答 422 `validation_failed`。"没有落点"是 200，不加新码（照 P5 的 `getTag`）。
- `target` 是自由文本的查询参数：契约测试 `TestFreeTextParametersDoNotAnswer5xx` 会发 NUL 与 `\xff`。
- 读不开事务、不取锁（总体设计 8.3）。

**新建**：前端在确认之后调已有的 `createPage(parent_id, title)`，不加新的写入口（13.1 第 1 条）；它的检查、码、事件与仓库照旧。两次调用之间的竞争：

- 父页删了：422 `parent_id`；标题撞了：409 `page.title_taken`，前端再问一次落点（多半已解析到）；太深：409 `page.too_deep`；角色降了：403。都在对话框里说明。
- 出发页移动、路径的父页改名：在旧的落点新建，链接可能仍解析不到。罕见、看得见，接受。
- 新建之后观察者重新解析，`links` 事件让出发页的阅读视图重读，链接变成已解析。

**用例** `linking/app.GetLinkLanding`：`Access.page`（同 P5）、写者的判定，再按目标与父页的键读候选、别名与出发页的路径（复用 `resolutions` 的读法）。`linking.Deps.Pages` 要 `ByKeys`、`Paths`：`PageTree` 加这两项（组合根已经传入完整的 `linkTargets`）。

## 3. 标签（A）

- 计数的标签（`obsidian.CountedTag` 接受的）渲染为 `<a class="nw-tag" data-nw-tag="名">#名</a>`，`名` 是 `CountedTag` 给出的写法（去掉末尾的 `/`），不是键：前端不折叠大小写，`getTag` 在服务端算键。嵌套的标签整个写（`a/b`）。
- `CountedTag` 不接受的（`#/`、`#1/`）照旧是 `<span class="nw-tag">`：索引里没有它们，点了也没有页。
- 标签的节点实现 `markdown.Linker`：净化丢掉用户围着它的 `<a>`（P3 B）。
- Markdown 链接的文字里的标签是 `<span>`：`linkedWikilinks` 这一变换（`obsidian/view.go`，优先级 40）一并标出链接里的标签。
- `Markup`：`a` 的属性加 `data-nw-tag`。

## 4. 属性表里的链接（A）

**现在**：`writeProperties`、`writeValue`（`platform/markdown/render.go`）把值写成转义的文字，没有钩子；属性链接已经在 `Resolve` 的输入里（P3）。

**歧义**：`{"a.b": "[[P]]", a: {b: "[[Q]]"}}` 的两个值路径都是 `a.b`（P5 文档第 4 节）。表里按**值的身份**对齐，不按路径，歧义就不存在：

- `markdown.Scalar` 加一个序号（不导出），读 frontmatter 时按文档的次序给每个字符串值编号（经锚点引用的也算一次，与写表的次序一致）。
- 平台的扩展加钩子 `Properties`：`writeValue` 写每个字符串值之前按同样的次序问它，扩展答"写成链接"（属性与文字）或不管。
- obsidian 对范围落在这个值里的属性链接答应：属性同正文的链接（`data-nw-node`、`data-nw-anchor` 或 `nw-unresolved`、`data-nw-target`），文字是链接显示的文字（wikilink 的显示文字或名称，Markdown 链接的文字）。
- 这改了 M4 的扩展点（核心属性表的钩子），记进总设计第 8 节。
- 病态输入：一万个属性链接进 `CheckCosts`、`CheckSize`。
- P5 的 `links: [{key, node_id}]` 按路径，歧义仍在：交给 P7 的右栏决定（第 15 节）。

契约 `getPageView` 的"属性是文字"改为写明链接、标签链接与锚点的地址。

## 5. 只有锚点的链接（A）

- `[t](#Heading%20Two)` 现在写 `href="#Heading%20Two"`，标题的 id 却是 `nw-heading-two`，点了不动（P3 文档 6.10）。
- 核心的 `marks`（写链接的地方）：地址以 `#` 开头时，按百分号解码，取最后一个 `#` 之后的部分，写 `#` 加 `HeadingID` 的结果。标题的 id 本来就是核心算的，地址也由核心写。
  - `^` 开头的块引用、算出空 id 的：只写文字，不写链接（同 `[[#^b]]`）。
  - 用户写的 `#nw-…` 也经同一个函数，与 `[[#…]]` 一致。
- 与 goldmark 逐字节对照的测试比的是 goldmark 自己的渲染器，不经 `marks`，不受影响。

## 6. 宽的内容（A、B）

**A**：

- 块公式（`div.nw-math-block`）与属性表（`.nw-props`）由服务端包 `<div class="nw-scroll">`，同表格：增强不改结构（13.2 第 23 条）。mermaid 的图由增强包（总设计 4.9 已写明的例外）。
- `tabindex` 不再由服务端写：不溢出的包装不该是一个 Tab 停留点，服务端不知道是否溢出。

**B**：

- 增强 `scrollRegions` 观察每个 `.nw-scroll` 与 `pre`（`ResizeObserver`），溢出时给 `tabindex="0"`、`role="region"` 与名称（"表格"、"公式"、"图"、"代码"、"属性"，i18n），不溢出时去掉；后来加的包装（mermaid）也观察。代替现在只管 `pre` 的 `scrollFocus`。
- 横向滚动从整个阅读视图移到这一层（`reading.css`）；e2e PG5 断言滚动的是包装，不是文章。

## 7. 未建的链接（B）

**哪些可以新建**：`a.nw-unresolved`，不是嵌入（`nw-embed`），不在 Markdown 图片的链接里。嵌入与图片只说明不存在（总设计 4.9）。附件那样的名称（`[[report.pdf]]`）在 M7 之前照样新建为页（同 Obsidian 新建同名的笔记），M7 让落点拒绝读作附件的目标（第 15 节）。

**增强** `unresolvedLinks`：

- 给这些链接 `role="button"`、`tabindex="0"`；写者另有 `aria-haspopup="dialog"`。点击、Enter、空格（`preventDefault`）调 `ReadingContext.unresolved(target, kind, element)`。
- `ReadingContext` 加 `unresolved`：阅读视图持对话框，增强不导入 `app/`（`reading/` 的导入规则）。

**写者**：

1. 问落点（`getLinkLanding`），其间链接标为忙。
2. 已解析到：经路由去那一页。
3. 有落点：确认对话框"新建页面「标题」？"，说明写"在「父页的路径」下"或"在笔记本的根上"。`ConfirmDialog` 加不是破坏性的样式。确认中按钮禁用（防两次新建）。
4. 确认：`createPage`，成功后去新页（`arrived`，同"新建页面"），并让出发页的视图重读（回来时链接已解析）。
5. 失败（第 2 节的竞争、离线）：对话框里经 `errorText` 说明；`page.title_taken` 时再问一次落点，已解析到就去那里。
6. 没有落点：一个按钮的说明对话框，按原因写（"「x」不能作为页面的标题"、"路径里的「A」不存在"、"太深了"、"新建之后也不会链接到它"）。

**读者**：说明对话框"页面「x」不存在"，不问服务端。

**焦点**：对话框关闭时焦点回到那条链接；重读替换了它时按 `data-nw-target` 与它在同名链接里的序号找回，找不到就落在文章上（M5 打磨移交第 8 项的同一个问题）。

## 8. 链接的增强与标签页（B）

**链接的增强**：P3 的 `pageLinks` 只管 `a[data-nw-node]`。一般化为 `appLinks`：

- `a[data-nw-node]`：页面的地址（带锚点的加 `#nw-…`），同 P3；属性表里的链接一样。
- `a[data-nw-tag]`：标签页的地址。
- 本站完整地址（第 9 节）。
- 点击在容器上委托，修饰键与中键照浏览器（地址是真的）；mermaid 图里的 `<a>` 也经它（读 `href` 或 `xlink:href`）。

**标签页**：路由 `/:workspace/notebooks/:id/tags/*`（通配，`a/b` 读得出来；每段编码）。

- 标题 `#标签`，列出 `getTag` 的页面（含下层的标签），名称取自已加载的树，同名的照 `distinctName`；空时说明没有页面。
- 不分页（同 `getTag`）；SWR 的键 `["tag-pages", nb, tag]`。
- 文档标题同页面的写法。

## 9. 本站完整地址（B）

普通 Markdown 里写的本站完整地址（`https://wiki.example/acme/notebooks/…`）经路由跳转（M4/P3 移交第 11 项）：

- 同源；路径不以服务端的前缀开头（`/api/`、`/healthz`、`/readyz`、`/assets/`）；最后一段不带 `.`（静态文件）。
- 路由表里的 `:slug` 与 `*` 几乎匹配一切，按路由表判断没有更多信息，所以用上面的规则。
- 服务端不变：没有"公开地址"的设置（邀请链接用的也是 `location.origin`）。

## 10. KaTeX 与 mermaid（B）

**版本**：`katex` 0.16.47、`mermaid` 11.17.2，精确版本。梳理的实测：mermaid 12.1 的流程图要带 1.46 MB 的 elk，且自带另一份 KaTeX（嵌进二进制的 `dist` 8.0 MB 对 6.1 MB；第一张流程图 2.2 MB 对 784 KB）。

**加载**：各自单独的 chunk，用到时 `import()`；构建检查加一条"KaTeX、mermaid 不在主 chunk 里"（照 `editor-out-of-main`）。加载器可以注入（照代码高亮的 worker），jsdom 里不真渲染。

**KaTeX** 增强 `math`：

- `.nw-math` 的原文用 `katex.render`：`trust: false`、`maxSize`、`maxExpand`、`throwOnError: true`；出错或原文长于上限（4,000 字节）时显示原文。
- 实测：2,000 个公式约 0.4 秒；首次加载约 288 KB（JS、CSS、字体）。

**mermaid** 增强 `diagrams`：

- `pre > code.language-mermaid`：`securityLevel: 'strict'`、`suppressErrorRendering: true`，`secure` 加 `layout`；`maxEdges` 200（450 条边在主线程 1.16 秒）。
- `maxTextSize` 超出时 mermaid 不报错，而是画一张替身图：增强自己先比长度（20,000 字节），超出、出错时显示原文。
- 只在可见时渲染（`IntersectionObserver`），一次一张；元素 id 用 `nw_mermaid_<n>`（标题"mermaid 0"的 id 是 `nw-mermaid-0`，不能撞）。
- 代码高亮跳过 `language-mermaid`。
- 每次重读都换掉整个 HTML：按（原文、主题）缓存渲染出的 SVG，重读时原文没变就直接用（模块级，至多 50 张）。
- 主题：`ReadingContext` 加 `theme`，换主题时重新增强（布局的副作用依赖它）。

**CSP**：梳理在生产的头下实测，十余种图、公式、错误输入、`%%{init: securityLevel: loose}%%` 都没有违规，不需要 `unsafe-eval`，字体可以载入。e2e 的每个页面一遇 CSP 违规就失败，L4 一并核对；结果写进总体设计 4.6（"具体写法在 M6 确定"）。

## 11. 事件（B）

- `pages` 事件：树变了或笔记本里有页写过时，重读这个笔记本的标签页（`["tag-pages", nb, *]`）。
- `links` 事件 `pages: null`（`reindex` 之后）：同上，并照旧重读全部阅读视图。
- 重连：`["tag-pages"]` 加进与 `["pages"]` 同一级的重读。
- 新建之后让出发页的视图重读（第 7 节）。
- 属性、反链、补全的数据随 P7。

## 12. 样式（B）

M6 的目标 1 是"看起来像 Obsidian"，各 Phase 都没有认领这些样式，P6 认领：

- 标签：小的圆角块，可聚焦时有焦点环；
- callout：按类型的颜色与图标，可折叠的 `details`；
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
- `ReadingContext` 加 `unresolved`、`theme`。
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
- `math`、`diagrams`：注入的加载器；上限、出错显示原文、可见时才渲染、一次一张、缓存、主题。
- `scrollRegions`：溢出时才可聚焦、名称。
- 构建检查：KaTeX、mermaid 不在主 chunk。

**e2e**：

- L2：写者点击未建的链接，确认之后新建并打开，回来时链接已解析；读者得到说明。
- L4：标签链接到标签页；公式与图渲染（CSP 之下，页面的固件一遇违规就失败）。
- PG5 随第 6 节修订。

**负对照**：每步的关键分支做变异，记在第 18 节。

## 15. 与总设计的出入

（本文定稿，总设计随 A、B 的合并修订。）

- **第 7 节 Phase 表**：P6 分 A（服务端）、B（前端）。
- **4.9**："没有落点的"补上太深、目标不合法、新建之后也解析不到；落点只给写者。
- **P3 文档第 2 节**：落点的规则改为本文第 2 节（父页不用别名、后置条件）。
- **4.9 宽的内容**：`tabindex` 由前端按是否溢出给；块公式与属性表由服务端包。
- **第 8 节扩展点**：核心属性表的钩子 `Properties`（改了 M4 的扩展点）。
- **4.9 KaTeX、mermaid**：版本、上限的数值、只在可见时渲染与缓存。
- **总体设计 4.6**：CSP 的实测结果（不改）。

**负责人可以改判**（作者的判断）：

- 全部标签的总览页、frontmatter 的 `tags` 显示为标签链接，交给 M12 的打磨。
- 附件那样的名称在 M7 之前照样新建为页。
- 落点只给写者；读者不问服务端，只说明不存在。
- mermaid 12 不用；`maxEdges` 200、原文 20,000 字节、公式 4,000 字节的上限。
- 属性表按值的身份对齐，右栏按路径的歧义交给 P7。

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
