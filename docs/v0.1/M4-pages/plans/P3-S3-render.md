# M4/P3/S3 渲染：实施计划

上级：[P3 文档](../03-P3-markdown.md) 3.5–3.8、3.10 的不变量。

## 任务

1. 依赖：`golang.org/x/net`（取当时最新的补丁版本），只用 `html`。
2. `render.go`：`Render(ctx, doc, page)`：扩展按顺序 `Fetch`（错误原样返回）；组装渲染器（goldmark 的 HTML 渲染器，安全模式；本包的标记；表格、任务列表、删除线、脚注的渲染器与选项；扩展的渲染器）；清洗的遍历；属性表在前、正文在后。
3. `marks.go`：标题（id 由 S2 的生成器写进属性）、链接与自动链接、图片为链接、代码块的语言 class；表格的 `align`；脚注的 id 前缀。
4. `sanitize.go`：分词、白名单、作用域与补闭合、连同内容去掉的元素、链接文字里的 `<a>`；把原始 HTML 节点换成清洗过的节点（新节点类型与它的渲染函数）。
5. `url.go`：`SafeURL`。
6. `markdowntest/`：包注释写"只给测试用（internal/archtest 守住）"；`CheckHTML(html string, exts ...markdown.Extension) error`，允许表独立于清洗的白名单。

## 测试

- 标记的表格：标题 id（ASCII、中文、混合、只有符号、重复三次、与已有的 `x-1` 撞上、超过 64 字）；脚注（多处引用、id 前缀）；任务项；表格对齐；代码块（`go`、`C++`、`  Go  extra`、`<script>`、超长、无信息串、缩进代码）；属性表（字符串、数字、布尔、null、列表、嵌套映射、无效与空的 frontmatter 不出表）。
- 清洗的表格：`<b onclick=x>`、`<span style>`、`<a href="javascript:…">`、`<a href="//evil">`、`<a href="/\evil">`、实体与制表符混淆的 `javascript:`、`<img src=x onerror=…>`、`<script>…</script>` 与内容、`<style>`、注释、`<p>` 在段落里（行内不放块级）、拆开的 `<b>` 与 `</b>` 在同一段、`<b>` 在一段未闭合（下一段不粗）、`*<b>*` 的结束标签越过强调、伪造的 `class="footnote-ref"`、`id="nw-x"`、链接文字里的 `<a>`、`<plaintext>`、`<svg><script>`。
- `SafeURL` 的表格：P3 文档 3.8 的每一条，含大小写、首尾控制字符、`https:evil.com`、`mailto:`、空串、`?a`、`#a`、`../a`。
- 渲染：外站、相对、`data:` 的图片；被拒的链接只剩文字；电子邮件与 `www.` 的自动链接。
- 样例集全部渲染不出错、`CheckHTML` 通过；有 frontmatter 的样例，正文部分与"frontmatter 之后的那一段单独渲染"相同。
- 扩展：测试替身的 `Fetch` 收到页面与它的提取结果，渲染器拿到 `Fetch` 的结果；`Fetch` 的错误由 `Render` 返回；它的标记按 `Markup` 通过 `CheckHTML`，不在 `Markup` 里的属性让 `CheckHTML` 报错。
- 反向对照：不补闭合；结束标签越过作用域；`SafeURL` 不把 `\` 当 `/`；图片输出 `<img>`；标题 id 不带前缀；`CheckHTML` 放过 `onclick` 与外站 `href`。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
