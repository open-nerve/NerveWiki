# M4/P3/S2 解析：实施计划

上级：[P3 文档](../03-P3-markdown.md) 3.2、3.3、3.5。

## 任务

1. `go.yaml.in/yaml/v3 v3.0.5` 改为直接依赖。
2. `markdown.go`：包注释（职责、引用总体设计 4.3 与 M4 总设计第 4、8 节、导入了哪些第三方库）；`Markdown`、`New(exts []Extension) (*Markdown, error)`（扩展名为空或重复报错）、`Extension`、`Markup`、`Page`、`Document`（`Frontmatter()`、`Extracted(name)`）。
3. `frontmatter.go`：边界（规则 1，与 `check.mjs` 的 `frontmatterSpan` 相同）；`Frontmatter`、`Property`。
4. `yaml.go`：节点树的遍历；1.2 core schema 的标量；标签；键；别名展开与上限（节点 10 000、深度 64）。
5. `parse.go`：BOM 与 frontmatter 的空白化（保留 `\r`、`\n`）；goldmark 解析（S1 的解析器加扩展的选项，标题 id 的生成器每篇一个，生成规则见 P3 文档 3.6）；扩展的提取。

## 测试

- 边界：BOM、CRLF、没有结束行、第一行前有空行、`---` 后有空格、只有开头一行、结束行在文末没有换行。
- YAML 的表格：样例 055 的全部；`.inf`、`-.Inf`、`.NaN` 保留原文；`+12`、`-0`、`0o17`、`0x1F`、`0x`（字符串）、`1e3`、`.5`；超出 int64 的整数为浮点；时间戳为字符串；`!!str 10`、`!!int "10"`、自定义标签（失败）；重复的键、序列作键（失败）；`<<` 为普通键；别名与锚点展开、互相嵌套的别名；节点数与深度恰好在上限与超出一个；空、只有注释（有效、无属性）；序列、标量、null（无效）。
- 样例集：`tools/md-fixtures/cases/*.md` 的 frontmatter 与 `.json` 的期望全部一致（属性按 JSON 归一比较）；frontmatter 的有无与 `check.mjs` 相同。
- 空白化：偏移不变（副本与原文等长，frontmatter 之外逐字节相同）；样例 031 的 ```` ``` ```` 不吞正文；BOM 开头的标题是标题。
- 扩展：测试替身（一个识别 `@@词@@` 的行内解析器）的提取结果出现在 `Document.Extracted`；没有它的文档里提取结果为空；扩展名重复时 `New` 报错。
- 反向对照：不展开别名；不计节点数；`010` 按八进制；结束行不去 `\r`；BOM 不空白化。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
