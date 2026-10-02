```yaml
status: open
from: M4/P3
to: M6
cc: [M5, M7]
created: 2026-10-02
```

# Markdown 的扩展：注册、语法、耗时与渲染

M4/P3 交付了唯一的 Markdown 解析与渲染 `platform/markdown`（[P3 文档](../../M4-pages/03-P3-markdown.md) 3.2–3.8、3.10）。组合根用 `bootstrap/registrants.go` 的 `markdownExtensions()` 建一个实例，页面的阅读视图与 M6 的链接共用它；M4 的扩展为空集合。M6 注册 Obsidian 方言（M5 的任务项字节位置、M7 的附件内联同理）时：

1. **注册**：`markdown.Extension{Name, Parser, Extract, Fetch, Renderer, Markup}` 加进 `markdownExtensions()`。`Extract(root, content)` 从语法树取结果（链接、标签带字节位置；偏移就是正文的偏移，frontmatter 已换成空白）；`Fetch(ctx, page, extracted)` 在 `Render` 里、渲染之前按页取数据（目标页是否存在等），在调用方的读取里、不加锁，出错时阅读视图答 500；`Renderer(data)` 拿到自己 `Fetch` 的结果。
2. **分隔符式的语法**（`==高亮==`、`%%注释%%`）**不能用 goldmark 的分隔符表**：加固之后没有人处理它（`internal/harden` 的包注释）。照 `internal/harden/emphasis.go` 的游程与按作用域的配对加一种字符，保持线性；`[[…]]` 与 `![[…]]` 是括号式的，注意与 `links.go` 的标签状态（`[` 的开启表）的先后与交互。
3. **链接的两条约束**（P3 审查 N3，`Extension.Parser` 的注释）：链接由 `internal/harden/links.go` 解析，goldmark 的 `parser.Context.IsInLinkLabel()` 在加固之后**永远为假**；"链接里不能有链接"只数 `links.go` 做的链接，扩展做的 `*ast.Link` 不计入（`[a @@x@@ b](/y)` 这样的替身会做出嵌套的 `<a>`）。wikilink 要在链接文字里识别（规则 4、样例 030/040）时，在 `platform/markdown` 导出入口（例如"是否在链接的标签里""记一个扩展做的链接"），转到 `harden` 的 `inLinkLabel` 与 `linksKey`，并加与原版逐字节对照的测试；`internal/harden` 是 `platform/markdown` 的内部包，注册者不能直接导入。
4. **耗时与输出**：扩展的解析器也要线性。`bootstrap/markdown_app_test.go` 的 `TestTheAppsMarkdownCostsAboutItsSize` 用注册了扩展的实例跑 `markdowntest.CheckCosts`（256 KB 的病态输入不超过普通文档的 10 倍耗时、14 倍分配，两倍的输入约两倍耗时），`make test-go` 在不带竞态检测的构建里跑它；新语法的病态输入（不闭合的 `[[`、`==`，深层嵌套，很多的嵌入）加进 `markdowntest.Pathological()`。输出也有上限：`markdowntest.CheckSize`（HTML 不超过正文的 64 倍加 4 MiB）在 `CheckCosts` 与 `FuzzRender` 里检查；嵌入（`![[…]]`）每用一次就把目标的内容写一遍时，要像引用式链接那样设展开的预算（`harden.MinExpansion`：原文字节数与 100 000 中较大者），放大类的输入加进 `markdowntest.Amplifying()`（在 16 KB 检查）。
5. **渲染**：扩展的解析器做出的每种节点都要有渲染函数——goldmark 的渲染器按节点种类的下标取函数，比它登记过的种类都新的节点会让 `Render` panic（审查 N4，`Extension.Renderer` 的注释）。地址一律经 `markdown.SafeURL`；输出的元素、属性与 class 写进 `Markup`，`TestTheAppsMarkdownRendersCheckedHTML` 用它跑 `markdowntest.CheckHTML`（样例集与生成的输入）。扩展的标记不经用户 HTML 的清洗；id 带 `nw-` 前缀。
6. **架构规则**：现在 goldmark、`golang.org/x/net/html`、`go.yaml.in/yaml` 只许 `internal/platform/markdown/...` 导入（`archtest` 的 `markdownLibrariesStayInMarkdown`）。注册者（例如 `modules/link/adapter/markdown`）要导入 goldmark 时改这条规则，给它开一个口子，并在反例表里加一行。
7. **M4 的现状**：`[[…]]` 照普通文字渲染；外部与相对图片都渲染为链接（`<span class="nw-image">`），M7 的附件另行内联；任务项是 goldmark 的 `<input type="checkbox" disabled>`，M5 加字节位置时用扩展的 `Extract` 取 `TaskCheckBox` 节点所在的段落。
