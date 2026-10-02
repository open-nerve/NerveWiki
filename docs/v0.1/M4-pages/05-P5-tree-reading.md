# M4/P5 页面树与阅读视图：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P5 前端：页面树与阅读视图 |
| 状态 | 进行中 |
| 基线 | `f0a90e9`（P4 合并、文档提交之后的 main）；本文与各 Step 计划提交之后开分支 |
| 上级文档 | [M4 总设计](00-M4-design.md) 第 3、4、5、7、8 节；[P3](03-P3-markdown.md) 3.6、3.12、已知差异；[P4](04-P4-content-sessions.md) 3.6、第 7 节；[M3 移交](handoffs/M3-notebooks.md)第 1–3 项；[总体设计](../v0.1-design.md) 9.2、9.4、13.2 |

---

## 1. 基线

后端的页面已经齐了：`listNodes`（整棵树，先序、兄弟按次序）、`createPage`、`renameNode`、`moveNode`（`after_id` 为 null 是第一个、省略是最后一个）、`deleteNode`、`getPage`、`getPageView`（`html` 与所依据的 `revision`；可能答 503 `server_busy`，P4 的解析预算）、正文与编辑会话（P4）。生成的类型里已有 `TreeNode`、`Page`、`PageView`、`NodeMove`；`problem-messages.ts` 已有全部 `page.*` 码的文案。

前端（M3 留下的）：

- 路由：`Layout` → `SignedIn` → `Onboarded` → `:slug`（`WorkspaceLayout`）→ `notebooks/:id`（`NotebookLayout`：首页与设置）。还没有页面的路由。
- 外壳：`Layout` 的 `<main>` 包着整个工作区外壳，左栏（切换器、`nav aria-label=工作区名`、笔记本两组）在 `main` 里（M3 移交第 1 项）；笔记本首页只有 `h1` 与"还没有页面"。
- store：MobX，按代（登录）一个 `RootStore`，按 id 缓存子 store（`once()`）；`notebook.store` 是范例：`changesAnswered` 计数丢弃与写重叠的读、`removed` 与 `wasRemoved`、`oneAtATimeById`。service 只有 `services/` 能引用 `@nervewiki/api-client`。
- 组件：`ConfirmDialog`、`useForm`、`NotLoaded`、`arrived` 与 `useArrivalFocus`、`useMounted`；`components/ui` 有 dialog、alert-dialog、dropdown-menu、native-select，没有树、命令面板。没有任何快捷键。
- 重复的组件：工作区与笔记本的成员行、两个改名表单（M3 移交第 3 项）；成员行只显示显式角色（第 2 项）。
- 依赖里没有拖拽库与 highlight.js；CSP 是 `script-src 'self'`，没有 `worker-src`（回落到 `script-src`），所以 Worker 必须是同源的独立文件；`assets/*` 不带 CSP 头。没有排版样式，没有 Worker。
- e2e：页面的故事 PG1–PG14 都只有接口版本；`fixtures/pages.ts` 是页面的接口 fixture。

## 2. 目标与范围

**目标**：在笔记本里看得到、走得通页面：左栏有当前笔记本的页面树，可以新建、改名、移动（拖拽与"移动到…"）、删除；页面有面包屑、标题、阅读视图与子页面列表；代码在 Worker 里高亮，超时即放弃；`Ctrl+O` 在已加载的树上快速切换。顺带收掉 M3 移交的三项。

**做**：

- 外壳：左栏移出 `main`，页面区域是 `main`；合并成员行与改名表单；成员行注明有效角色（M3 移交第 1–3 项）。
- 页面的 service 与按笔记本的树 store；路由 `/:slug/notebooks/:id/pages/:pageId`；页面外壳（面包屑、标题、阅读视图、子页面列表）；笔记本首页列出根下的页面。
- 左栏的页面树：展开与收起、当前页、每项的菜单（新建子页、改名、移动到…、删除）；拖拽（`@atlaskit/pragmatic-drag-and-drop` 与树形命中区）；"移动到…"对话框；删除的确认写明子页面数。
- 阅读视图与交互增强的管线；排版与渲染器标记的样式（明暗两套）；代码高亮的 Worker（highlight.js）与超时。
- `Ctrl+O` 快速切换。
- 文案；vitest；e2e 的 PG1–PG6、PG11、PG13、PG14 页面版本与 PG12 的新建、移动、删除入口。

**不做**：编辑器、编辑会话的前端、`Ctrl+E`/`Ctrl+S`（P6）；正文里同站链接的应用内跳转（M6 的链接跳转）；大树的虚拟列表（M12 的压测之后再定）；`document.title`（现在没有任何页面设它，另立一项，见第 6 节）。

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  app/layout.tsx                         外壳由路由的 handle 声明；有外壳时 Layout 不包 main
  app/rename-form.tsx                    合并的改名表单（工作区、笔记本、页面）
  app/member-row.tsx                     合并的成员行（工作区、笔记本），有效角色的注明
  app/effective-role.ts                  shared.EffectiveNotebookRole 的同一规则
  app/shortcuts.ts                       Mod+O 等快捷键的识别（Mod：macOS 上 Cmd，其余 Ctrl）
  services/page.service.ts               listNodes、createPage、renameNode、moveNode、deleteNode、getPageView
  stores/page-tree.store.ts              按笔记本：树、一个写队列、答复后重读、removed
  stores/page-tree.ts                    树的纯函数：子页、祖先、深度、子树、子树高度、可用的"未命名 N"
  stores/root.store.ts                   pagesOf(notebook)
  pages/notebook/page-tree.tsx           左栏的页面树（nav、嵌套列表、展开、当前页）
  pages/notebook/page-tree-item.tsx      一项：展开按钮、链接、菜单、拖拽
  pages/notebook/page-drag.ts            拖拽：命中区的指令 → NodeMove 或被挡下
  pages/notebook/move-page-dialog.tsx    "移动到…"
  pages/notebook/rename-page-dialog.tsx  改名（合并的改名表单）
  pages/notebook/delete-page-dialog.tsx  删除（子页面数）
  pages/notebook/new-page.ts             新建：取空闲的"未命名 N"，409 换下一个，至多三次
  pages/notebook/notebook-home.tsx       根下的页面列表与"新建页面"
  pages/page/page-layout.tsx             页面外壳：在树里找到它才显示；删除之后去父页或首页
  pages/page/breadcrumbs.tsx
  pages/page/subpage-list.tsx
  pages/page/reading-view.tsx            挂上服务端 HTML，按注册顺序运行交互增强
  reading/enhancement.ts                 交互增强的类型与注册表（M5、M6、M7 在这里注册）
  reading/highlight.ts                   代码高亮的增强：找代码块、交给 Worker、核对答复、超时终止
  reading/highlight.worker.ts            Worker：highlight.js/lib/common
  reading/highlight-markup.ts            答复的白名单：只有 span.hljs-* 与文字
  reading/reading.css                    排版、.nw-props、.nw-image、脚注、任务项；高亮的明暗两套
  pages/notebook/quick-switch.tsx        Ctrl+O 的对话框（输入框加列表框）
e2e/
  fixtures/wiki-pages.ts                 页面树与页面的页面对象（fixtures/pages.ts 已是接口 fixture）
  stories/page/pg1…pg6, pg11…pg14        加 "(page)" 版本
```

### 3.2 外壳与 `main`（M3 移交第 1 项）

`WorkspaceLayout` 的路由带 `handle: { shell: true }`。`Layout` 用 `useMatches()` 看匹配里有没有外壳：没有时照旧用 `<main>` 包着 `Outlet`；有时只给一个容器，外壳自己布局：左栏是 `<div>`（里面的 `nav` 照旧），右边的内容区是 `<main>`。地标与布局对应：左栏是导航，页面区域是主体。W3 断言的 `main` 的内边距随之改为内容区的；`[data-shell]` 留着给页面对象找切换器。

### 3.3 合并的组件与有效角色（M3 移交第 2、3 项）

- `app/member-row.tsx`：一个成员行，参数是角色的列表与文案前缀、移除的标题与正文；工作区与笔记本的成员页都用它。
- `app/rename-form.tsx`：一个改名表单，参数是检查函数、`fieldTexts`、提示、`autoComplete`、提交（返回 Promise）与按钮文案；工作区、笔记本的设置页与页面的改名对话框都用它。页面标题与笔记本名称是同一个规则（`shared.CheckTitle`），检查与 `fieldTexts` 用 `notebookNameProblem`、`notebookNameTexts`，改名接口的字段是 `name`。
- 有效角色：笔记本的成员页另读工作区的成员列表（`useMembers(workspace)`，访客也能读，没有地址），按 `shared.EffectiveNotebookRole` 的同一规则（`app/effective-role.ts`，与 `test/notebook-server.ts` 的规则共用测试用例）算出；显式角色低于有效角色时行上注明（"阅读者 · 经工作区开放为编辑者"）。工作区成员列表没读到之前或读取失败时不注明：注明是补充，不挡住成员列表本身。

### 3.4 页面的数据

- `PageService`：`listNodes`、`createPage`、`renameNode`、`moveNode`、`deleteNode`、`getPageView`；类型从生成的客户端转出。
- `PageTreeStore`（`RootStore.pagesOf(notebook)`，按代、按笔记本 id 缓存）：
  - `nodes`（先序）与派生的 `byId`、`childrenOf`；读取经 SWR，键 `["pages", notebook.id]`，fetcher 是 store 的 `load()`。
  - 写（新建、改名、移动、删除）走同一个 `oneAtATime` 队列；每次答复之后（成功或失败）重读整棵树，答复带回的节点不在本地套用（M4 总设计第 4 节"前端的页面数据"：不同节点的写互相改变位置，客户端不推算次序）。
  - 与写重叠的读照 `notebook.store` 的 `changesAnswered` 丢弃；删除成功的 id 进 `removed`，页面外壳用 `wasRemoved` 区分"本标签页删的"与"不存在"。
- 阅读视图：SWR 键 `["page-view", id]`，不进 store（只读、按页面）。
- 纯函数（`stores/page-tree.ts`）：子页、祖先链、深度、子树与子树的高度（拖拽与"移动到…"在发出之前挡下成环与超过 10 层）、兄弟里第一个空闲的"未命名 N"（按标题键比较：NFC 后转小写，近似服务端的键；不准时服务端答 409，换下一个）。

### 3.5 路由与页面外壳

- `notebooks/:id/pages/:pageId` 在 `NotebookLayout` 之下，懒加载。`PageLayout` 在树里找这一页：树没读到时 `NotLoaded`；找到时显示；`wasRemoved` 时 `Navigate` 去它的父页（父页也删了就去笔记本首页），带 `arrived`；不在树里时显示工作区内的 404。
- 页面外壳：面包屑（`nav aria-label` + `ol`，笔记本、祖先，当前页带 `aria-current="page"`）；`h1` 是节点名（`useArrivalFocus`）；阅读视图；子页面列表（`ul aria-label`，来自树）。标题、祖先与子页都取自树，不另调 `getPage`：树的重读即刷新它们。
- 笔记本首页：根下的页面列表（与子页面列表同一个组件）；编辑者与管理员有"新建页面"；空的时候照旧"还没有页面"。
- 删除当前页（或它的祖先）之后外壳去父页或首页，焦点照 13.2 第 12 条落在目的页的 `h1`。

### 3.6 左栏的页面树

- 位置：打开一本笔记本时，左栏在笔记本两组之下多一个 `nav`（`aria-label` 是"<笔记本名>的页面"），按笔记本 id 重新挂载（13.2 第 16 条）。不用 ARIA 的 `tree` 角色：项是链接，用嵌套的列表与展开按钮（`aria-expanded`、`aria-controls`），键盘走 Tab 即可，屏幕阅读器读出层级。
- 每项：展开按钮（有子页时）、链接（当前页 `aria-current="page"`）、编辑者与管理员的菜单（新建子页、改名、移动到…、删除）。阅读者没有菜单、没有拖拽、没有"新建页面"（PG12）。
- 展开的状态按笔记本存在内存里（随代），当前页的祖先自动展开；不进 `localStorage`。
- 树没读到时这一块是 `NotLoaded`（带"重试"）；读到之后，别处的改动在重读之后出现（13.2 第 1 条的"重新读取显示别处的改动"）。

### 3.7 树的写

- **新建**：根下（树头与首页的"新建页面"）或某页下（菜单的"新建子页"）；标题是兄弟里第一个空闲的"未命名"、"未命名 2"……，答 409 `page.title_taken` 换下一个，至多三次（M4 总设计第 4 节）；成功之后去新页面（`useMounted` 之后才导航），父页展开。
- **改名**：菜单的"改名"打开对话框，里面是合并的改名表单；422 与 409 留在表单里。
- **删除**：确认对话框写明子页面数（从树里数）；成功之后若当前页在被删的子树里，外壳去父页或首页（3.5）。404 当作已删除（照 `notebook.store`）。
- **移动到…**：对话框里两个下拉框：父页（"笔记本的根"与其余页面，排除自己与子树、排除放进去会超过 10 层的）与位置（"最前"、"在 X 之后"……、"最后"）；键盘完全可用（WCAG 2.2 的 2.5.7）。答 409（`page.cycle`、`page.too_deep`、`page.title_taken`）留在对话框里。
- **拖拽**：`@atlaskit/pragmatic-drag-and-drop` 与它的树形命中区（`attachInstruction`、`extractInstruction`）：放在前、放在后、成为子页（成为最后一个子页，省略 `after_id`）。指令到请求是纯函数（`page-drag.ts`）：放在某项之前是 `after_id` = 它的前一个兄弟（没有则 null），之后是 `after_id` = 它；拖到自己或自己的子树里、或会超过 10 层的，指令被挡下（`instruction-blocked`），不发请求。拖拽中显示落点的指示线。
- 任何一个写失败时的提示用 `errorText`；树都会重读。

### 3.8 阅读视图与交互增强

- `ReadingView` 把服务端的 HTML 挂进 `article`（服务端已清洗，前端不再改它的结构），挂上之后按注册顺序运行交互增强：每个增强拿到容器元素与上下文（工作区、笔记本、页面 id、`revision`、调用者的角色、重新读取阅读视图的方法），返回自己的清理函数；HTML 换掉之前先按相反的顺序清理，再重跑（M4 总设计第 8 节）。
- 注册表在 `reading/enhancement.ts`：一个数组，组合根（`app/providers.tsx` 一侧）交给 `ReadingView`；M4 只有代码高亮。一个增强抛错只记日志（`console.error` 之外的上报 M12 再说）、不影响其余的增强与正文。
- 阅读视图的 SWR 读取有 `NotLoaded` 与重读；`revision` 换了（别处写了）重读之后 HTML 与增强都换。
- 阅读视图答 503 `server_busy`（P4 的解析预算，带 `Retry-After`）时，`NotLoaded` 显示"服务器繁忙"，SWR 照 `Retry-After` 自动重读（`app/retry.ts`，M1/P5 的规则）；vitest 一格。
- 样式（`reading/reading.css`，Tailwind 的 `@layer components`）：标题、段落、列表、引用、表格（`align` 属性）、代码块、任务项、脚注（`footnote-ref`、`footnotes`）、`.nw-props`（可嵌套）、`.nw-image`（替代文字与地址）；高亮的明暗两套（暗色在 `.dark` 之下）。

### 3.9 代码高亮

- 增强找出 `pre > code[class^="language-"]`；没有就不建 Worker。有时建一个 Worker（每个阅读视图一个，清理时终止），把各块的文字与语言发过去；语言不在 highlight.js 的 `common` 集里的照常显示。
- 超时：一页的高亮总共 2 秒（常量，有测试钉住）；到时终止 Worker，没着色的块照常显示，不重试。单块超过 100 KB 的不送去高亮。
- 答复是 highlight.js 的 HTML；主线程用 `<template>` 解析，只接受文字与 `span class="hljs-…"`（`highlight-markup.ts` 的白名单），解析出的文字必须与原文一致，否则这块不着色。前端因此不信任 highlight.js 的输出。
- Worker 是同源的独立文件（Vite 的 `new Worker(new URL(...), { type: "module" })`），满足 `script-src 'self'`；不用内联与 blob。构建产物的 Worker 文件在 `assets/` 下，e2e 的 PG5 在页面上看到着色、没有 CSP 违规。
- vitest：jsdom 没有 Worker，高亮的增强接受一个 Worker 工厂；测试用同步的假 Worker（直接调 highlight.js）与不答复的假 Worker（超时）。

### 3.10 快速切换

- `Mod+O`（macOS 上 Cmd，其余 Ctrl）在打开笔记本时生效，阻止浏览器的"打开文件"；对话框里一个输入框与一个列表框（`role="listbox"`、`aria-activedescendant`），上下键移动、回车前往、Esc 关闭。
- 在已加载的树上按标题查找（NFC 后转小写的子串），每项的次要文字是祖先链；最多显示 50 项。树没读到时对话框里说明并给"重试"。

### 3.11 文案

页面的新键放在 `page.*`（`page.untitled`、`page.untitledN`、`page.new`、`page.newSubpage`、`page.rename`、`page.moveTo`、`page.delete`、`page.deleteBody`（带子页面数）、`page.tree`（nav 的标签）、`page.subpages`、`page.breadcrumb`、`page.quickSwitch*`……）；中英两份同键（类型与占位符的测试）。字段 `title` 的错误文案 `field.title.*` 补上（`createPage`）。

### 3.12 已知差异与安全

- GitHub 式的 `<details><summary>` 里写 Markdown，渲染为空的折叠块（HTML 块各自是作用域，P3 审查 Q1）：写进帮助的已知差异。
- 外部图片是链接、不发请求（PG6 的页面版本另挂一个请求监听）；同站的链接点开是整页导航（应用内跳转是 M6 的链接跳转）。
- 页面版本的 PG6 在真实浏览器里核对：服务端 HTML 挂上之后没有脚本执行、没有 CSP 违规（P3 审查第 115 行留给 P5 的真实浏览器检查）。

### 3.13 e2e 的页面版本

- 页面对象在 `fixtures/wiki-pages.ts`：树（展开、菜单、拖拽）、页面外壳、"移动到…"、快速切换。
- PG1：树头与菜单新建，"未命名"与"未命名 2"，落库 `web`；PG2：改名对话框，422 与 409 留在表单；PG3：拖拽排序与换父页、"移动到…"、成环与过深在对话框里被挡下或答 409；PG4：删除子树，确认写明子页面数，之后外壳去父页；PG5：阅读视图的元素与着色、属性表在正文之上；PG6：没有脚本、没有 CSP 违规、外部图片没有请求；PG11：面包屑、子页面列表、`Ctrl+O`；PG12：阅读者没有新建、菜单、拖拽；PG13：删除笔记本之后它的页面在左栏与地址里都没了；PG14：无主清单显示页面正文的大小。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 外壳的 `main`；合并成员行与改名表单；有效角色的注明 | [P5-S1](plans/P5-S1-shell-merges.md) |
| S2 | 页面的 service、树 store 与纯函数；路由与页面外壳；左栏的页面树（只读）；笔记本首页；阅读视图（无增强） | [P5-S2](plans/P5-S2-tree-shell.md) |
| S3 | 树的写：新建、改名、删除、"移动到…"、拖拽 | [P5-S3](plans/P5-S3-tree-writes.md) |
| S4 | 交互增强的管线；排版与标记的样式；代码高亮的 Worker | [P5-S4](plans/P5-S4-reading.md) |
| S5 | `Ctrl+O`；e2e 的页面版本 | [P5-S5](plans/P5-S5-switch-e2e.md) |

## 5. 测试与验证

| 层 | 内容 |
|---|---|
| vitest：store | 树的读、写的队列（同一笔记本的写一个接一个）、每次答复之后重读、与写重叠的读被丢弃、按代与按笔记本缓存、`removed` |
| vitest：纯函数 | 祖先、深度、子树高度、"未命名 N"、拖拽指令 → `NodeMove`（前、后、成为子页、被挡下） |
| vitest：组件 | 外壳的 `main`；合并的成员行与改名表单（两处原有的测试照旧通过）；有效角色的注明；树的展开、当前页、阅读者没有入口；新建的 409 重试；删除的子页面数与之后的导航；"移动到…"；快速切换的键盘；阅读视图的增强顺序与清理、抛错的增强不影响其余；高亮的超时终止、白名单拒绝、文字不一致不着色；每块 SWR 的 `NotLoaded` 与重读 |
| e2e | 3.13 的页面版本；之前的故事全部通过 |
| 反向对照 | 每个新检查一个：如树的写不重读、拖拽不挡成环、高亮不终止 Worker、白名单放过别的元素、外壳仍把左栏包进 `main` |

## 6. 完成标准

- S1–S5 的检查全部为绿：`make check`、`make gen-check`、`make e2e`、`make image-smoke`；持续集成为绿。
- M3 移交第 1–3 项处理完（第 4 项：M4 没有只锁笔记本行又改成员行的写，推理不变，收尾时写明）。
- Opus 审查与修复，审查记录在 `reviews/P5-tree-reading-review.md`。
- `document.title` 与正文里同站链接的应用内跳转写进 M4 收尾的待定项。

## 7. 结果

（完成后填写）
