# M6/P7 编辑器与右栏：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M6/P7 编辑器与右栏（前端） |
| 状态 | 实施中 |
| 基线 | P6 合并之后的 main（`e488f58`），分支 `m6-p7` |
| 上级文档 | [M6 总设计](00-M6-design.md) 4.11–4.13、第 7 节 P7、第 9 节 L5、L6；[P5 文档](05-P5-api.md)；[P6 文档](06-P6-reading-view.md)第 10、11 节；[M4/P6 编辑器移交](../M5-collab-editing/handoffs/M4-P6-editor.md)第 5 项；[输入法清单](../M4-pages/manual/P6-ime-checklist.md)；[总体设计](../v0.1-design.md) 12.4、13.1 第 21 条、13.2 第 8 条 |

---

## 0. 梳理

作者读了编辑器（`editor/registry.ts`、`extensions.ts`、`source-editor.tsx`）、页面的外壳（`pages/page/page-layout.tsx`）、事件的处理（`events/handlers.ts`、`app/event-stream.tsx`）、P5 的契约（`api/modules/linking.yaml`）、构建检查（`build/out-of-main.ts`）与 `@codemirror/autocomplete` 6.20.3 的源码（它已在依赖树里，随 `@codemirror/lang-markdown` 来）。

- **编辑器的扩展**同步返回 CodeMirror 的扩展；注册表由 `main.tsx` 静态导入，只许引用 CodeMirror 的类型。补全要构造运行时的对象，所以要先改契约（总设计 4.13）。
- **构建检查**把编辑器的入口写死为 `source-editor.tsx` 与 `conflict-view.tsx`：别的模块经动态导入到达，照样跟进去，会报"编辑器进了主包"。扩展的模块要算进编辑器的入口。
- **`@codemirror/autocomplete`** 已经处理输入法的一部分：组合中输入的文字会激活补全（`input.type.compose` 也是 `input.type`）；组合改了文字又移了光标时，结束后它自己重新开始补全。组合进行中弹出、回车选中，要我们自己挡。`CompletionContext` 带 `view`，补全的来源能看到 `view.composing`。
- **页面的外壳**是一列：面包屑与标题、阅读视图或编辑器、子页。右栏要在外壳里加一栏。
- **事件**：`links` 带 `targets`（反链变了的页），P6 的处理还没有用；`pages` 带写过的页与 revision。
- **测试的组合根**：`test/render.tsx` 的 `renderApp` 已经能注入编辑器的扩展（M5 加的）。

## 1. 范围

- 编辑器扩展可以带 `load`（第 2 节）。
- 补全的数据经 `EditorContext`（第 3 节）；`[[` 补全页面与别名（第 4 节），`#` 补全标签（第 5 节）；输入法（第 6 节）。
- 右栏：布局（第 7 节）、大纲（第 8 节）、反链（第 9 节）、属性（第 10 节）；事件与重连时的重读（第 11 节）。
- e2e L5、L6；输入法清单第 16–18 步（第 14 节）。

不做：

- `[[页面#标题` 与 `[[页面^块` 的补全：Obsidian 有，总设计没有要求（第 15 节）。
- 编辑时的大纲：总设计 4.12 交给 M12。
- 反链之外的"没有链接的提及"：总设计第 2 节不做。

## 2. 编辑器扩展的 `load`

**契约**（`editor/registry.ts`）：

```ts
type Build = (context: EditorContext, controls: EditorControls) => Extension;
export type EditorExtension =
  | { name: string; extension: Build }
  | { name: string; load: () => Promise<Build> };
```

- 带 `load` 的扩展，`load` 动态导入编辑器 chunk 里的一个模块，答出它的 `Build`。那个模块可以导入 CodeMirror 的运行时值；注册表本身仍只引用类型。
- 编辑器在造视图之前等全部的 `load`（`editor/extensions.ts` 的 `loadExtensions`，按注册表缓存一份 promise），再按注册表的次序组合。`SourceEditor` 用 React 的 `use` 等它，`PageEdit` 已有的 `Suspense` 显示"载入中"。
- 一个 `load` 失败的扩展照"构造时抛错"处理：写控制台，留在外面，别的照常（`composeExtensions` 现在的做法）。
- 这是改 M4 扩展点的契约（总设计第 8 节已写明）。M5 的三个扩展不变。

**构建检查**（`build/out-of-main.ts`）：编辑器的入口加 `src/editor/loaded/` 下的模块。约定：带 `load` 的扩展，它导入的模块放在 `editor/loaded/`。`lazyLeak` 的测试加一例：主包动态导入的 `editor/loaded/x.ts` 是编辑器的，不报；它被静态导入时报。

## 3. 补全的数据

`EditorContext` 加：

```ts
/** linkTargets reads the notebook's link targets (listLinkTargets): its pages, each with its link and aliases. */
linkTargets(): Promise<readonly LinkTarget[]>;
/** tags reads the notebook's tags (listTags), each with how many pages have it. */
tags(): Promise<readonly TagCount[]>;
```

- `PageEdit` 给出，经页面树的 store 调 `LinkingService` 的新方法 `linkTargets`、`tags`。
- **每次补全读一次**：`[[` 或 `#` 开始一次补全时读，补全开着时按 `validFor` 在本地过滤，不再读；下一次 `[[` 再读。所以总是新的（别的标签页刚建的页、刚加的标签），不需要事件去失效缓存。
- 代价：一次 `[[` 读一次整个笔记本的目标。一万页约 1 MB；写进第 15 节（负责人可以改判：按事件失效的缓存）。
- 读失败：这次补全不弹出，不报错（写控制台）。

## 4. `[[` 补全

扩展 `linkCompletion`（`editor/loaded/link-completion.ts`），登记在 `editorExtensions` 的最后（M5 的三个之后）。

- **何时**：光标之前、同一行里最后一个 `[[`，其后到光标没有 `]`、`[`、`|`、`#`、`^` 与换行。那段文字是查询。
- **选项**：
  - 每页一项：显示标题，`detail` 是 `link`（与标题不同时：重名的页写的是路径）；
  - 每个别名一项：显示别名，`detail` 是"→ 标题"。
- **过滤**：CodeMirror 自己的模糊匹配，按显示的文字。中文逐字匹配。
- **插入**（与 Obsidian 相同）：
  - 页：把 `[[` 之后到光标换成 `link]]`；
  - 别名：换成 `link|别名]]`；
  - 光标之后已有 `]]` 时一并换掉，不写出 `]]]]`；
  - 光标落在 `]]` 之后。
- **不弹出**：在代码里（lezer 的 `InlineCode`、`FencedCode`、`CodeBlock` 及其子节点）；只读时；组合进行中（第 6 节）。

## 5. `#` 补全

同一个扩展的第二个来源。

- **何时**：`#` 在行首，或前面是空白（含全角空格）；`#` 之后到光标都是标签的字符：Unicode 字母、组合用字符、数字、`_`、`-`、`/`（样例集规则 9，与服务端的 `isTagRune` 相同）。
  - 规则 9 还认紧跟在行内元素之后的 `#`（`**x**#tag`），补全不认：要解析行内的结构，得不偿失（第 15 节）。
  - 行首的 `# ` 是标题：一打空格就不再是标签，补全随之关掉。
- **选项**：`listTags` 的每个标签，`detail` 是页数。
- **插入**：把 `#` 之后到光标换成标签。
- **不弹出**：同第 4 节。

## 6. 输入法

- **组合进行中不弹出**：补全的来源在 `context.view.composing` 时答 `null`；`compositionstart` 时关掉已开着的补全。
- **组合中按回车不误选**：补全自带的键位关掉（`defaultKeymap: false`），换成自己的：方向键、翻页、Escape 照旧；回车与 Tab 在组合进行中、或组合刚结束（50 毫秒之内，与编辑器的 `compositionSettles` 相同：Safari 先发 `compositionend` 再发回车的 `keydown`）时不接受，交给输入法。
- **组合结束之后**：`@codemirror/autocomplete` 在组合改了文字、移了光标时自己重新开始补全，这时来源照常答（中文标题的 `[[`、中文标签的 `#`）。
- 输入法清单加第 16–18 步（第 14 节）；自动的测试只能模拟 `view.composing`。

## 7. 右栏的布局

- `PageShell` 的正文（阅读视图或编辑器，子页）与右栏并排：`xl` 及以上两栏，右栏 15rem，靠上粘住（`sticky`），超出时自己纵向滚动；窄于 `xl` 时右栏排在正文之后。
  - `xl`：左边的工作区栏 15rem，`lg` 时正文只剩约 460px，太窄。
- 右栏是 `<aside>`，名称"页面信息"（i18n）。三节：大纲、反链、属性，各是 `<details open>`，`<summary>` 里是节的标题；没有内容的节显示一句说明（大纲没有标题时整节不显示）。
- 编辑时：大纲不显示（总设计 4.12），反链与属性照旧。

## 8. 大纲

- 来自阅读视图读到的 HTML（同一个 SWR 键 `["page-view", notebook, page]`，不另读）：解析进 `<template>`，取文章里带 `nw-` id 的 `h1`–`h6`，按级缩进。
- 文字是标题的文字（公式是原文，同服务端写的）。
- 点击经路由去 `#id`：与文章里的 `[t](#h)` 同一条路，阅读视图的锚点处理让标题拿到焦点、滚到那里（P6 第 6、9 节）。
- 是 `<nav>`，名称"大纲"。

## 9. 反链

- `listBacklinks`，SWR 键 `["backlinks", notebook, page]`，第一页 50 个。
- 每项：出发页的标题（取自已加载的树，链接到那一页），链接数大于 1 时写"N 处"（1000 写"1000+ 处"），然后每行上下文（纯文字，原样，`…` 是服务端加的）。
  - 树里还没有的出发页（别的标签页刚建的，树还没重读）先不显示，树重读之后出现。
- "更多"读下一页（`next_cursor`），接在后面；重读时回到第一页。
- 没有反链：一句说明。

## 10. 属性

- `getPageProperties`，SWR 键 `["page-properties", notebook, page]`。
- 每个属性一行：键，值。
  - 字符串照写；数字、布尔照写；`null` 写空；列表逐项；对象写成 JSON。
  - 属性链接（`links` 里有这个键：标量是键本身，列表的项是 `键.序号`）：解析到页面的写成链接（显示链接的文字：`[[x|y]]` 是 `y`，`[[x]]` 是 `x`，`[t](u)` 是 `t`），解析不到的照写，带未建的样式。
- frontmatter 不合法：一句说明。没有属性：一句说明。
- 阅读视图顶部 M4 的属性表照旧（总设计 4.12）。

## 11. 事件与重读

`events/handlers.ts`：

- `links`：
  - `targets` 里的每一页读反链；`null` 读笔记本的全部反链；
  - `pages` 里的每一页读属性（属性链接的解析变了）；`null` 读全部。
- `pages`：写过的每一页（`pages` 里的）读属性；`null` 读全部。
- 都经 refresher（同阅读视图的重读，合并、限频）。
- 重连时（`app/event-stream.tsx` 的 `refreshedOnConnect`）：`backlinks`、`page-properties` 与 `page-view` 同一层。
- 补全的数据不经事件：每次补全都读（第 3 节）。

## 12. 样式与文字

- 右栏的节标题同子页那一节（小号、浅色）；链接同正文；上下文用小号的浅色字，逐行。
- i18n（中、英）：右栏的名称，三节的标题，没有标题、没有反链、没有属性、frontmatter 不合法的说明，"N 处"，"更多"；补全的 `detail` 里"N 页"。

## 13. 模块与组合

- `editor/registry.ts`：`EditorExtension` 的联合类型；`EditorContext` 的两个函数；`editorExtensions` 加 `linkCompletion`（`{ name, load: () => import("./loaded/link-completion").then((m) => m.linkCompletion) }`）。
- `editor/extensions.ts`：`loadExtensions`。
- `editor/loaded/link-completion.ts`：两个来源、键位、输入法。
- `services/linking.service.ts`：`backlinks`、`properties`、`linkTargets`、`tags`。
- `stores/page-tree.store.ts`：转给服务（同 `tagPages`）。
- `pages/page/page-panel.tsx`（右栏）、`page-outline.tsx`、`page-backlinks.tsx`、`page-properties.tsx`；`page-layout.tsx` 的布局；`page-edit.tsx` 的 `EditorContext`。
- `events/handlers.ts`、`app/event-stream.tsx`。
- `build/out-of-main.ts`。
- `package.json`：`@codemirror/autocomplete` 改为直接依赖（6.20.3，与锁文件里的相同）。

## 14. 测试

**vitest**：

- `extensions.test.ts`：`load` 的扩展按注册表的次序组合；`load` 失败的留在外面，别的照常；同一个注册表只载一次。
- `link-completion.test.ts`（真实的 CodeMirror，jsdom）：
  - `[[` 的触发与查询、选项（页、别名、重名的 `detail`）、插入（页、别名、光标之后已有 `]]`）；
  - `#` 的触发（行首、空白之后；不在词中间、不在 `##`、不在 `# ` 之后）、插入；
  - 代码里、只读时不弹出；
  - 组合进行中不弹出、回车不接受，组合刚结束 50 毫秒之内回车不接受；
  - 读失败不弹出。
- **最后一跳**（M4/P6 编辑器移交第 5 项，总设计 13.1 第 21 条）：页面上的测试经组合根的 `editorExtensions` 打开编辑器，输入 `[[`，补全列出页面；注册表交空时失败。
- `out-of-main.test.ts`：`editor/loaded/` 的例子；`make build-web` 本身是检查。
- 右栏：大纲（标题、级、点击去锚点、编辑时不显示）、反链（名称取自树、"N 处"、上下文、更多、树里没有的不显示、没有反链）、属性（各种值、链接、解析不到的、不合法、没有）。
- 事件：`links` 的 `targets` 读反链、`pages` 读属性，`null` 读全部；`pages` 读属性；经组合根的处理表。

**e2e**：

- L5：编辑一页，`[[` 补全另一页的标题、别名（插入 `[[link|别名]]`），`#` 补全已有的标签；在行内代码里不弹出；保存之后阅读视图里是解析到的链接与标签。
- L6：右栏的大纲点击去标题；属性里的链接去那一页；另一个会话（API）往别的页写一条指向这一页的链接，这一页的反链随即出现（推送），上下文是那一行。

**输入法清单**第 16–18 步（`docs/v0.1/M4-pages/manual/P6-ime-checklist.md`）：

- 16：`[[` 之后用拼音输入中文标题的一部分，确认之后补全列出那一页，回车插入 `[[标题]]`；
- 17：`#` 之后用拼音输入中文标签的一部分，同上；
- 18：补全开着时用拼音输入，组合中按回车只确认输入法的候选，不选补全的项。

## 15. 与总设计的出入、负责人可以推翻的决定

- 补全的数据每次补全读一次，不缓存（第 3 节）。
- `[[页面#标题` 的补全不做（Obsidian 有）。
- `#` 补全不认紧跟在行内元素之后的标签（第 5 节）。
- 右栏在 `xl` 以上才在旁边（第 7 节）。
- 反链"更多"之后的重读回到第一页（第 9 节）。

## 16. 实施步骤

| 步 | 内容 |
|---|---|
| S1 | `load` 的契约、`loadExtensions`、`SourceEditor` 等载入；构建检查的入口；测试 |
| S2 | 服务与 store；`EditorContext` 的两个函数；`linkCompletion` 的 `[[` 与 `#`、代码里不弹出；直接依赖；单元测试与最后一跳 |
| S3 | 输入法：组合中不弹出、自己的键位；测试；清单第 16–18 步 |
| S4 | 右栏的布局与大纲 |
| S5 | 反链、属性；事件与重连；测试 |
| S6 | e2e L5、L6 |

## 17. 完成标准

- 第 14 节的测试通过；`make check`、`make gen-check`、e2e 全量通过。
- 审查的发现处理完，修复经 Opus 核对，直到一轮没有行为上的发现；`make image-smoke` 通过。

## 18. 结果

（合并时填写。）
