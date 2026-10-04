```yaml
status: open
from: M4/P3, M4 收尾
to: M6
cc: [M5, M7]
created: 2026-10-02
```

# Markdown 的扩展：注册、语法、耗时与渲染

> M4 收尾时修订（2026-10-03）：第 4 项的耗时判据改为与代码一致；加第 8–11 项（[M4 收尾审查](../../M4-pages/reviews/M4-closeout-review.md) A-I1、A-N6、A-Q1、A-Q2、C-I3、C-Q1）。M5 的任务项比 M6 先注册，M5 那边另有[一份指过来的移交](../../M5-collab-editing/handoffs/M4-P3-markdown-extensions.md)。

M4/P3 交付了唯一的 Markdown 解析与渲染 `platform/markdown`（[P3 文档](../../M4-pages/03-P3-markdown.md) 3.2–3.8、3.10）。组合根用 `bootstrap/registrants.go` 的 `markdownExtensions()` 建一个实例，页面的阅读视图与 M6 的链接共用它；M4 的扩展为空集合。M6 注册 Obsidian 方言（M5 的任务项字节位置、M7 的附件内联同理）时：

1. **注册**：`markdown.Extension{Name, Parser, Extract, Fetch, Renderer, Markup}` 加进 `markdownExtensions()`。`Extract(root, content)` 从语法树取结果（链接、标签带字节位置；偏移就是正文的偏移，frontmatter 已换成空白）；`Fetch(ctx, page, extracted)` 在 `Render` 里、渲染之前按页取数据（目标页是否存在等），在调用方的读取里、不加锁，出错时阅读视图答 500；`Renderer(data)` 拿到自己 `Fetch` 的结果。
2. **分隔符式的语法**（`==高亮==`、`%%注释%%`）**不能用 goldmark 的分隔符表**：加固之后没有人处理它（`internal/harden` 的包注释）。照 `internal/harden/emphasis.go` 的游程与按作用域的配对加一种字符，保持线性；`[[…]]` 与 `![[…]]` 是括号式的，注意与 `links.go` 的标签状态（`[` 的开启表）的先后与交互。
3. **链接的两条约束**（P3 审查 N3，`Extension.Parser` 的注释）：链接由 `internal/harden/links.go` 解析，goldmark 的 `parser.Context.IsInLinkLabel()` 在加固之后**永远为假**；"链接里不能有链接"只数 `links.go` 做的链接，扩展做的 `*ast.Link` 不计入（`[a @@x@@ b](/y)` 这样的替身会做出嵌套的 `<a>`）。wikilink 要在链接文字里识别（规则 4、样例 030/040）时，在 `platform/markdown` 导出入口（例如"是否在链接的标签里""记一个扩展做的链接"），转到 `harden` 的 `inLinkLabel` 与 `linksKey`，并加与原版逐字节对照的测试；`internal/harden` 是 `platform/markdown` 的内部包，注册者不能直接导入。
4. **耗时与输出**：扩展的解析器也要线性。`bootstrap/markdown_app_test.go` 的 `TestTheAppsMarkdownCostsAboutItsSize` 用注册了扩展的实例跑 `markdowntest.CheckCosts`（512 KB 的病态输入：先量它的四分之一，不超过同样大小的普通文档的 10 倍耗时；全尺寸不超过普通文档的 10 倍耗时、14 倍分配，也不超过四分之一的 8 倍加 1 毫秒；普通的 1 MB 不超过 1 秒；`markdowntest/costs.go`，P3 文档 3.10），`make test-go` 在不带竞态检测的构建里跑它；新语法的病态输入（不闭合的 `[[`、`==`，深层嵌套，很多的嵌入）加进 `markdowntest.Pathological()`。输出也有上限：`markdowntest.CheckSize`（HTML 不超过正文的 64 倍加 4 MiB）在 `CheckCosts` 与 `FuzzRender` 里检查；嵌入（`![[…]]`）每用一次就把目标的内容写一遍时，要像引用式链接那样设展开的预算（`harden.MinExpansion`：原文字节数与 100 000 中较大者），放大类的输入加进 `markdowntest.Amplifying()`（在 16 KB 检查）。
5. **渲染**：扩展的解析器做出的每种节点都要有渲染函数——goldmark 的渲染器按节点种类的下标取函数，比它登记过的种类都新的节点会让 `Render` panic（审查 N4，`Extension.Renderer` 的注释）。地址一律经 `markdown.SafeURL`；输出的元素、属性与 class 写进 `Markup`，`TestTheAppsMarkdownRendersCheckedHTML` 用它跑 `markdowntest.CheckHTML`（样例集与生成的输入）。扩展的标记不经用户 HTML 的清洗；id 带 `nw-` 前缀。
6. **架构规则**：现在 goldmark、`golang.org/x/net/html`、`go.yaml.in/yaml` 只许 `internal/platform/markdown/...` 导入（`archtest` 的 `markdownLibrariesStayInMarkdown`）。注册者（例如 `modules/link/adapter/markdown`）要导入 goldmark 时改这条规则，给它开一个口子，并在反例表里加一行。
7. **M4 的现状**：`[[…]]` 照普通文字渲染；外部与相对图片都渲染为链接（`<span class="nw-image">`），M7 的附件另行内联；任务项已是 M5 的扩展（`platform/markdown/tasks`，[M5/P6 文档](../../M5-collab-editing/06-P6-task-items.md) 3.2）：harden 不再注册 goldmark 的任务项，复选框带 `data-task`（方括号里那个字符的字节位置），`CheckHTML` 的核心白名单里没有 `input`，由它的 `Markup` 声明。它是第一个注册者，注册的写法照 `bootstrap/registrants.go` 的 `markdownExtensions()`。
8. **最后一跳的测试**（总体设计 13.1 第 21 条）：M4 只有替身测试。M6 注册时，在整个程序上各加一个行为测试，组合根交空时失败：
   - 参与者（链接改写）：经 `serve` 改名或移动一页，链接到它的页面的正文随之改写，追加的写经守卫、进同一个变更集与同一次事件（`page/app` 的 appender）。
   - 观察者（索引）：经每条写入路径到达，见 [M5 的观察者移交](../../M5-collab-editing/handoffs/M4-P1-tree-refresh.md)第 4 项的路径表。
   - Markdown 扩展：经 `markdownExtensions()` 到达阅读视图（`GET …/view` 用组合根的实例）与 `Extract`。
   - 补全的编辑器扩展：经组合根的 `editorExtensions` 到达页面上的编辑器；`web/apps/web/src/test/render.tsx` 的 `renderApp` 有 `editorExtensions` 选项（M5/P4 加的；自动保存的 `pages/page/page-autosave.test.tsx` 是一个例子）。
9. **P4 留下的两条**（[P4 文档](../../M4-pages/04-P4-content-sessions.md)第 7 节）：参与者追加的正文写自带解析结果，不经解析预算，且持着笔记本行的锁解析；观察者调用之后不得留着 `Parsed`。多页的正文写按节点 id 升序加锁（现在只写在 `page/app/unit_content.go` 的注释里）。解析预算是 page 模块自己的（`page.Module`）：参与者的改写多了，考虑把预算移到平台、由组合根共享（A-Q1 第 3 点）。
10. **笔记本删除不发页面事件**：删子树发事件，删笔记本只软删页面、告诉会话的订阅者（`page/app/extension.go` 的 `NotebookDeletion`）。M6 的链接与索引若有引用节点的行，二选一，并修订总体设计 12.4 笔记本删除的那一行：自己注册笔记本删除并软删，连同清理的先后（`RESTRICT` 的外键会挡住页面的清理）；或者读取时按 `nodes.deleted_at` 过滤（A-Q2）。
11. **阅读视图**（M4 收尾的待定项）：
    - **同站链接的应用内跳转**：现在正文里的同站链接整页导航（重新加载应用、多一次续期，没有正确性问题）。M6 的"链接跳转"增强也覆盖普通 Markdown 的同站地址，保留修饰键与中键的点击；`ReadingContext` 没有应用内导航的手段，加一个（M4 收尾审查 B-Q1）。
    - **宽表格**：现在宽表格让整个阅读视图横向滚动，视图在比显示的宽时可以聚焦（`reading/scroll-focus.ts`，代码块同样）。更好的是渲染器给每个表格包一层自己滚动、可以聚焦的区域：带 `nw-` 前缀的 class 与 `tabindex="0"`，登记进 `Markup`，`CheckHTML` 与样例集随之更新，横向滚动从 `.nw-reading` 移到包裹层，可访问名称由前端的增强补上（[P5 审查](../../M4-pages/reviews/P5-tree-reading-review.md) Q4）。现在阅读视图以页面的标题为名称，代码块可以聚焦却还没有名称（[M4 收尾审查](../../M4-pages/reviews/M4-closeout-review.md)的修复核对 MN-3），随包裹层一起补上。KaTeX、mermaid 一类同样宽的内容一起考虑。
