# M4/P5 页面树与阅读视图：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P5 前端：页面树与阅读视图 |
| 状态 | 已完成（`b0eb02e` 合并） |
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
  main.tsx                               组合根：把阅读视图的增强（readingEnhancements）经 Enhancements 交给阅读视图
  app/layout.tsx                         唯一的 main 总包着 Outlet；左栏的位置（ShellColumn，display: contents）在 main 之外
  app/rename-form.tsx                    合并的改名表单（工作区、笔记本、页面）
  app/member-row.tsx                     合并的成员行（工作区、笔记本），有效角色的注明
  app/effective-role.ts                  shared.EffectiveNotebookRole 的同一规则；writesPages(role)
  app/confirm-dialog.tsx                 由触发按钮打开，或由调用者持有（held）
  app/held-dialog.ts                     调用者持有的对话框的类型（菜单里的改名、移动、删除）
  app/shortcuts.ts                       Mod+O 等快捷键的识别（Mod：macOS 上 Cmd，其余 Ctrl）
  services/page.service.ts               listNodes、createPage、renameNode、moveNode、deleteNode、getPageView
  stores/page-tree.store.ts              按笔记本：树、一个写队列、答复后重读、removedTo
  stores/page-tree.ts                    树的纯函数：子页、祖先、深度、子树、子树高度、canHold、可用的"未命名 N"、按标题查找
  stores/root.store.ts                   pagesOf(notebook)
  pages/workspace/workspace-layout.tsx   左栏：工作区的 nav（切换器、各节、笔记本两组），其下给页面树的位置（NotebookColumn）
  pages/notebook/notebook-layout.tsx     把页面树放进左栏的位置；挂快速切换
  pages/notebook/page-tree.tsx           左栏的页面树：列表、一项（展开、链接、菜单、拖拽）、菜单持有的对话框
  pages/notebook/page-drag.ts            拖拽：落点的操作 → NodeMove，或被挡下；命中区提供的操作
  pages/notebook/move-page-dialog.tsx    "移动到…"
  pages/notebook/rename-page-dialog.tsx  改名（合并的改名表单）
  pages/notebook/new-page.ts             新建：取空闲的"未命名 N"，409 换下一个，至多三个标题
  pages/notebook/notebook-home.tsx       根下的页面列表与"新建页面"
  pages/notebook/quick-switch.tsx        Mod+O 的对话框（combobox 加 listbox）
  pages/page/page-layout.tsx             页面外壳：在树里找到它才显示；删除之后去父页或首页
  pages/page/breadcrumbs.tsx
  pages/page/subpage-list.tsx
  pages/page/reading-view.tsx            挂上服务端 HTML，按注册顺序运行交互增强，换 HTML 之前按相反顺序撤销
  reading/enhancement.ts                 交互增强的类型、注册表（readingEnhancements）、Enhancements、enhance
  reading/highlight.ts                   代码高亮的增强：找代码块、交给 Worker、核对答复、超时终止
  reading/highlight.worker.ts            Worker：把每块交给 highlight-block
  reading/highlight-block.ts             highlight.js/lib/common 的一块（Worker 与测试的同步假 Worker 共用）
  reading/highlight-markup.ts            答复的白名单：只有文字与 highlight.js 的 span
  reading/reading.css                    排版、.nw-props、.nw-image、脚注、任务项；高亮的明暗两套
server/internal/platform/webui/csp.go    workerPolicy：assets/ 下的脚本带 default-src 'none'（Worker 用它）
e2e/
  fixtures/wiki-pages.ts                 页面树、菜单、拖拽、对话框、外壳、快速切换的页面对象（fixtures/pages.ts 是接口的）
  stories/page/pg1…pg6, pg11…pg14        加 "(page)" 版本
```

一项（`PageItem`、`useDrag`、`PageMenu`）没有单独成 `page-tree-item.tsx`，在 `page-tree.tsx` 里；删除没有 `delete-page-dialog.tsx`，用 `ConfirmDialog` 的 held 方式。

### 3.2 外壳与 `main`（M3 移交第 1 项）

`Layout` 只有一个 `<main>`，总包着 `Outlet`；它之前给左栏一个不生成盒子的位置（`div.contents`，经 `ShellColumn` 的 context 交出去）。`WorkspaceLayout` 用 `createPortal` 把左栏放进这个位置，左栏于是在 `main` 之外、页面区域是 `main`；外壳自己的 `NotLoaded`、404 与子路由的错误边界照样在 `main` 里。不用路由的 `handle` 与 `useMatches`：错误边界在 `:slug` 之上时，`useMatches` 仍带着外壳的 `handle`，那时就没有 `main` 了。

左栏外层不是地标（M3 定过它是主导航，不是补充内容）：工作区的 `nav`（名称是工作区名）里依次是切换器、各节、笔记本两组；其下是给页面树的位置（`NotebookColumn`），`NotebookLayout` 同样用 `createPortal` 把树放进去。W3 断言唯一的 `main` 之外有左栏；`[data-shell]` 留着给页面对象找切换器。

### 3.3 合并的组件与有效角色（M3 移交第 2、3 项）

- `app/member-row.tsx`：一个成员行，参数是成员、角色的列表与文案、注明、移除的标题与正文、改角色与移除；工作区与笔记本的成员页都用它。
- `app/rename-form.tsx`：一个改名表单，参数是当前名、标签、提示、`autoComplete`、检查函数、`fieldTexts`、提交（返回 Promise）、按钮文案与保存之后的回调；工作区、笔记本的设置页与页面的改名对话框都用它。页面标题与笔记本名称是同一个规则（`shared.CheckTitle`），检查与 `fieldTexts` 用 `notebookNameProblem`、`notebookNameTexts`，改名接口的字段是 `name`。
- 有效角色：笔记本的成员页另读工作区的成员列表（`useMembers(workspace)`），按 `shared.EffectiveNotebookRole` 的同一规则（`app/effective-role.ts`，与 `test/notebook-server.ts` 共用测试用例）算出；显式角色低于有效角色时行上注明（"阅读者 · 经工作区开放为编辑者"）。工作区成员列表没读到之前或读取失败时不注明。
- 页面的写权限 `writesPages(role)` 也在 `app/effective-role.ts`：编辑者与管理员，别的角色（含以后新增、这个客户端还不认识的）都不写。

### 3.4 页面的数据

- `PageService`：`listNodes`、`createPage`、`renameNode`、`moveNode`、`deleteNode`、`getPageView`；类型从生成的客户端转出。
- `PageTreeStore`（`RootStore.pagesOf(notebook)`，按代、按笔记本 id 缓存）：
  - `nodes`（服务端的列表，`observableRef` 整体替换）与派生的 `tree`（`byId`、`childrenOf`）；读取经 SWR，键 `["pages", notebook.id]`，fetcher 是 store 的 `load()`。读到的与原来相同时保留原数组，聚焦引起的重读不再重渲染。
  - 写（新建、改名、移动、删除）走同一个 `oneAtATime` 队列；每次答复之后（成功或失败）重读整棵树再结束，答复带回的节点不在本地套用（M4 总设计第 4 节）。重读失败时树保持原样，等 SWR 再读。
  - 与写重叠的读照 `notebook.store` 的 `changesAnswered` 丢弃，再读一次。
  - 删除：子树与它的父页在删除轮到时才从树里取（排在前面的新建创建的页已在里面）；答复之后（404 `page.not_found` 当作已删除）子树的每页记进 `removed`，值是子树的父页（null：首页）。`removedTo(id)` 告诉外壳去哪里；`tree` 滤掉这些页，重读成功与否，删掉的页都立刻离开树。
- 阅读视图：SWR 键 `["page-view", id]`，不进 store（只读、按页面）。
- 纯函数（`stores/page-tree.ts`）：子页、祖先链、深度、子树与子树的高度、`canHold`（拖拽与"移动到…"在发出之前挡下成环与超过 10 层）、兄弟里第一个空闲的"未命名 N"（按标题键比较：NFC 后转小写，近似服务端的键；不准时服务端答 409，换下一个）、`findPages`（快速切换）。

### 3.5 路由与页面外壳

- `notebooks/:id/pages/:pageId` 在 `NotebookLayout` 之下。`PageLayout` 在树里找这一页：树没读到时 `NotLoaded`；找到时显示；不在树里而 `removedTo` 有值时 `Navigate` 去那个父页（父页也不在树里就去笔记本首页），带 `arrived`；否则显示工作区内的 404。
- 页面外壳：面包屑（`nav aria-label` + `ol`，笔记本、祖先，当前页带 `aria-current="page"`；分隔符是 `aria-hidden` 的元素）；`h1` 是节点名（`useArrivalFocus`）；阅读视图；子页面列表（`ul aria-label`，来自树）。标题、祖先与子页都取自树，不另调 `getPage`：树的重读即刷新它们。
- 笔记本首页：根下的页面列表（与子页面列表同一个组件）；编辑者与管理员有"新建页面"；空的时候照旧"还没有页面"。
- 删除当前页（或它的祖先）之后外壳去父页或首页，焦点照 13.2 第 12 条落在目的页的 `h1`。

### 3.6 左栏的页面树

- 位置：打开一本笔记本时，`NotebookLayout` 把树放进左栏工作区 `nav` 之下的位置（3.2），树是自己的 `nav`（`aria-label` 是"<笔记本名>的页面"），按笔记本 id 重新挂载（13.2 第 16 条）。不用 ARIA 的 `tree` 角色：项是链接，用嵌套的列表与展开按钮（`aria-expanded`、`aria-controls`），键盘走 Tab 即可，屏幕阅读器读出层级。
- 每项：展开按钮（有子页时）、链接（当前页 `aria-current="page"`）、编辑者与管理员的菜单按钮（"<页名>的操作"：新建子页、改名、移动到…、删除）。菜单按钮平时在悬停、获得焦点或菜单打开时显示，有手指指针的设备上（`any-pointer: coarse`，触屏笔记本也算）一直显示。阅读者没有菜单、没有拖拽、没有"新建页面"（PG12）。
- 编辑者的树里链接不可拖（`draggable={false}`）：链接默认可拖，浏览器把离起点最近的可拖元素当作拖拽源，从标题上拖起来的会是链接而不是这一行。代价是编辑者不能把树里的链接拖到书签栏（右键仍可复制、在新标签页打开）；阅读者的链接照常可拖。
- 展开的状态按笔记本存在内存里（随代），当前页的祖先自动展开；不进 `localStorage`。
- 树没读到时这一块是 `NotLoaded`（带"重试"）；读到之后，别处的改动在重读之后出现（13.2 第 1 条）。

### 3.7 树的写

- **新建**：根下（树头与首页的"新建页面"）或某页下（菜单的"新建子页"）；标题是兄弟里第一个空闲的"未命名"、"未命名 2"……，答 409 `page.title_taken` 换下一个，至多试三个标题（M4 总设计第 4 节）。成功之后，发出的组件还在、地址也没变（location key），才去新页面（`arrived`）；用户在答复之前去了别处就留在那里（13.2 第 16 条）。深度已到 10 的页面没有"新建子页"。`useNewPage().create` 答失败的原因，由调用处显示：树里与拖拽共用"最近一次写的失败"一个状态，首页有自己的。
- **改名**：菜单的"改名"打开对话框，里面是合并的改名表单；422 与 409 留在表单里，保存之后对话框关闭。
- **删除**：`ConfirmDialog` 的 held 方式（菜单持有，没有触发按钮）；确认写明子页面数（从树里数）。删掉的页立刻离开树（3.4）：当前页在被删的子树里时外壳去父页或首页（3.5），焦点由那一页的标题接过；否则焦点给树头。对话框关闭时焦点的去处按 store 判断（`removedTo`），不按确认框的结果：删除一答复，这一项与它持有的确认框就随树卸载，可能早于确认框知道删除已成。
- **移动到…**：对话框里两个下拉框：父页（"笔记本的顶层"与其余页面，以祖先链加页名显示，排除自己与子树、排除放进去会超过 10 层的）与位置（"最前"、"在 X 之后"……、"最后"）；打开时在这一页现在的位置。树在对话框开着时重读、所选的父页或位置不再出现时，回到这一页的父页与"最后"：选择框显示的就是发出的。键盘完全可用（WCAG 2.2 的 2.5.7）。答 409（`page.cycle`、`page.too_deep`、`page.title_taken`）留在对话框里。
- **焦点**：改名、移动的对话框关闭之后与取消删除之后，焦点给这一页的菜单按钮；移到别的父页之后这一项是新挂的，树按 `data-actions-of` 找到它（找不到给树头）。
- **拖拽**：`@atlaskit/pragmatic-drag-and-drop` 4.0.0 与命中区 3.0.0 的 list-item：之前、之后、放进去（成为最后一个子页，省略 `after_id`），每种可以单独标为不可用或被挡下。不用 tree-item：它多一种按指针横向位置推算的 reparent，我们的规则只有这三种。命中区是行的上 1/4（之前）、下 1/4（之后）与中间（放进去）。落点到请求是纯函数（`page-drag.ts`）：放在某项之前是 `after_id` = 它的前一个兄弟（除去被拖的那页；没有则 null），之后是 `after_id` = 它；拖到自己、自己的子树里、或会超过 10 层的被挡下（显示为红色），不发请求。展开着、显示子页的那一项不提供"之后"：线会画在它与第一个子页之间，页却会落在整棵子树之后；下半部因此归"放进去"，要放在它的子树之后就放在下一页之前；它是最后一个兄弟、后面没有下一页时，先收起，或用"移动到…"。拖拽中显示落点：之前、之后是从页名的层级起画的线，放进去是这一行的边框。
- 任何一个写失败时的提示用 `errorText`；树都会重读。

### 3.8 阅读视图与交互增强

- `ReadingView` 把服务端的 HTML 挂进 `article`（服务端已清洗，前端不再改它的结构），挂上之后按注册顺序运行交互增强：每个增强拿到容器元素与上下文（工作区、笔记本、页面 id、`revision`、调用者的角色、重新读取阅读视图的方法），返回撤销它的函数或什么都不返回；HTML 换掉之前、阅读视图离开时，按相反的顺序撤销（M4 总设计第 8 节）。
- 注册表 `readingEnhancements` 在 `reading/enhancement.ts`；组合根 `main.tsx` 用 `Enhancements` 的 context 包住 `SessionRoot`，把它交给阅读视图。context 的默认值是空列表：测试默认没有增强，`renderApp` 的第四个参数给假的（jsdom 没有 Worker）。M4 只有代码高亮。一个增强运行或撤销时抛错只记日志（`console.error` 之外的上报 M12 再说）、不影响其余的增强与正文。
- 阅读视图的 SWR 读取有 `NotLoaded` 与重读；`revision` 换了（别处写了）重读之后 HTML 与增强都换。
- 阅读视图答 503 `server_busy`（P4 的解析预算，带 `Retry-After`）时，`NotLoaded` 显示"服务器繁忙"，SWR 照 `Retry-After` 自动重读（`app/retry.ts`）。
- 样式（`reading/reading.css`，`@layer components`，由 `styles.css` 引入）：标题、段落、列表、引用、表格（列的 `align` 属性定对齐，其余从头对齐；表格保持表格：显示为块的表格在一些读屏器里丢掉表格语义；宽的由阅读视图整体横向滚动（`.nw-reading` 的 `overflow-x: auto`），页面本身不横向滚动）、代码块、任务项、脚注、`.nw-props`（可嵌套）、`.nw-image`（替代文字与地址）；高亮的明暗两套（暗色在 `.dark` 之下）。

### 3.9 代码高亮

- 增强找出 `pre > code[class^="language-"]`；没有就不建 Worker。有时建一个 Worker（每个阅读视图一个），把各块的序号、语言与文字一次发过去，Worker 每块答一次；语言不在 highlight.js 的 `common` 集里的答 null，照常显示。全部答完、到时或撤销时终止 Worker。
- 超时：一页的高亮总共 2 秒（常量，有测试钉住），从建 Worker 起算，含 Worker 脚本的下载与解析：慢网络第一次加载超过 2 秒时这一页不着色，之后脚本在缓存里（`immutable`）。到时没着色的块照常显示，不重试。单块超过 100 KB（UTF-8 字节）的不送去高亮。
- 答复是 highlight.js 的 HTML；主线程用 `<template>` 解析，只接受文字与 `span`，`span` 只能有 `class`，每个 class 是 highlight.js 的：作用域 `hljs-*`、子作用域的后缀（`function_`、`class_`、`inherited__`）、嵌入语言的 `language-*`（`highlight-markup.ts`）；解析出的文字必须与原文一致，否则这块不着色。前端因此不信任 highlight.js 的输出。
- Worker 是同源的独立文件（`new Worker(new URL("./highlight.worker.ts", import.meta.url), { type: "module" })`），满足页面的 `script-src 'self'`；不用内联与 blob。Vite 按默认的 iife 打成 `assets/highlight.worker-*.js`（约 156 KB）。专用 Worker 用的是自己脚本响应的策略，不继承页面的：服务端给 `assets/` 下的脚本加 `Content-Security-Policy: default-src 'none'`（页面加载的脚本忽略它），Worker 因此不能取网络。PG5 在真实浏览器里看到着色、Worker 同源且带这个策略、没有 CSP 违规。
- Worker 的文件只给自己用到的那部分全局作用域（`addEventListener`、`postMessage`）写类型，经 `globalThis` 取得：没有给 tsconfig 加 WebWorker 的 lib（它与 DOM 的 lib 冲突）。
- vitest：jsdom 没有 Worker，高亮的增强接受一个 Worker 工厂；测试用同步的假 Worker（调用与 Worker 相同的 `highlight-block.ts`）与不答复的假 Worker（超时）。

### 3.10 快速切换

- `Mod+O`（macOS 上 Cmd，其余 Ctrl；只有这一个修饰键）在打开笔记本时生效（挂在 `NotebookLayout`：首页、设置、页面都算），阻止浏览器的"打开文件"；已有对话框开着时只阻止、不打开。按键取 `key` 的拉丁字母；不是拉丁字母的布局（俄语、希腊语）取 `code` 所在位置的拉丁字母。
- 对话框里一个输入框（`role="combobox"`、`aria-activedescendant`；有列表时 `aria-controls`）与一个列表框（`role="listbox"`），上下键移动、回车前往（`arrived`）、Esc 关闭；活动项在列表变短（树重读）时夹在长度之内。关闭时焦点回到打开之前的元素（它还在文档里时）；前往了别的页面就不还原，由那一页的标题接过焦点：到达的焦点与关闭的还原谁先谁后不定，页面路由已加载时到达在前。选的正是显示中的页时不导航、只关闭。oxlint 的 jsx-a11y 有四条规则偏好原生元素，逐处带理由禁用：原生 `select` 的选项不能显示祖先链。
- 在已加载的树上按标题查找（NFC 后转小写的子串，从上到下），每项的次要文字是祖先链；最多显示 50 项。"没有找到"与"只显示前 50 个"在 `<output>`（status）里，空时 `sr-only`、一直在无障碍树里（不在树里的 live region 与内容同时出现，多数读屏器不播报）；结果数的每次变化不播报。树没读到时对话框里说明并给"重试"。

### 3.11 文案

页面的新键放在 `page.*`（`page.untitled`、`page.untitledN`、`page.new`、`page.newSubpage`、`page.actions`、`page.rename`、`page.moveTo`、`page.move*`、`page.delete*`（带子页面数）、`page.tree`（nav 的标签）、`page.subpages`、`page.breadcrumb`、`page.quickSwitch*`……）；中英两份同键（类型与占位符的测试）。`field.title.*` 没有补：页面的标题只有两处输入，新建取生成的"未命名 N"，改名复用笔记本名的规则与文案（同一个 `shared.CheckTitle`）。

### 3.12 已知差异与安全

- GitHub 式的 `<details><summary>` 里写 Markdown，渲染为空的折叠块（HTML 块各自是作用域，P3 审查 Q1）：写进帮助的已知差异。
- 外部图片是链接、不发请求（PG6 的页面版本另挂一个请求监听）；同站的链接点开是整页导航（应用内跳转是 M6 的链接跳转）。
- 页面版本的 PG6 在真实浏览器里核对：服务端 HTML 挂上之后没有脚本执行、没有对话框、没有 CSP 违规、没有发往别的主机的请求（P3 审查第 115 行留给 P5 的真实浏览器检查）。
- 高亮的 Worker 在自己的 CSP（`default-src 'none'`）之下运行（3.9）。

### 3.13 e2e 的页面版本

- 页面对象在 `fixtures/wiki-pages.ts`：树（展开、菜单、拖拽）、页面外壳、"移动到…"、快速切换。
- PG1：树头与菜单新建，"未命名"与"未命名 2"，落库 `web`；PG2：改名对话框，422 与 409 留在表单；PG3：拖拽排序与换父页、拖到展开着的页的下沿进入它的子页、"移动到…"、成环与过深在对话框里被挡下或答 409；PG4：删除子树，确认写明子页面数，之后外壳去父页；PG5：阅读视图的元素与着色、属性表在正文之上，Worker 同源、带 `default-src 'none'`；PG6：没有脚本、没有 CSP 违规、外部图片没有请求；PG11：面包屑、子页面列表、`Ctrl+O`；PG12：阅读者没有新建、菜单、拖拽；PG13：删除笔记本之后它的页面在左栏与地址里都没了；PG14：无主清单显示页面正文的大小。

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

- 分支 `m4-p5-tree-reading`：S1 `2cafdb6`；S2 `a0fd8df`；S3 `6a156f2`；S4 `54d95ed`；S5 `bc1f507`；审查修复 `8037df5`、`2d1a855`（修复核对的发现）；`b0eb02e` 合并（`--no-ff`）。
- 门禁：每个 Step 与审查修复的 `make check` 为绿（web 的 vitest 最后 1176 个）；`make gen-check`、`make e2e`（141 个）、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P5 审查](reviews/P5-tree-reading-review.md)。
  - Major 3：跨父页"移动到…"之后焦点丢到 body；快速切换关闭之后焦点丢到 body；展开着的页上"放在之后"，指示线与落点不一致。
  - Minor 12：删除排在新建之后漏掉新页；显示中的页被删、重读又失败时停在已删的页上；新建答复之前离开了仍被拉去新页；失败提示互相覆盖；"移动到…"的选项在重读之后失效；触屏上看不到菜单按钮；切换器不在地标里；Worker 没有 CSP；e2e 不轮询的读取；快速切换的三处；重读引起的重渲染；代码质量。疑问 7，采纳 4。
  - 修复另经 Opus 核对：2 个 Major 是修复带进来的焦点回归（删掉的不是当前页、重读慢时焦点到 body；快速切换前往别的页之后焦点被还原回去），另有 3 Minor、6 Nit 与 4 条处置不属实，第二轮修复处理。
  - 反向对照：S1 7、S2 9、S3 44、S4 26、S5 23（另有 e2e 6），审查修复 26，核对之后 8，全部失败。

**负责人可改的决定**：

- 编辑者树里的链接不可拖（3.6）：行整体是拖拽源，代价是不能把链接拖到书签栏；阅读者的照常可拖。
- 展开着的页的下半部是"放进去"（3.7）：要放在它的子树之后就放在下一页之前；它是最后一个兄弟时先收起，或用"移动到…"。
- 一页的高亮总共 2 秒，含 Worker 脚本的第一次下载（3.9）。

**与计划的出入**（已同步进上文）：

1. 外壳的 `main`：不用路由的 handle，改为布局给一个不生成盒子的位置（`display: contents`）与 context，工作区外壳经 `createPortal` 把左栏放进去；`main` 只有一个、总包着 `Outlet`（3.2）。页面树同样经 `NotebookColumn` 放进左栏，随笔记本换代（3.6）。
2. 一项（`PageItem`、`useDrag`、`PageMenu`）在 `page-tree.tsx` 里，没有 `page-tree-item.tsx`；删除用 `ConfirmDialog` 新的 held 方式，没有 `delete-page-dialog.tsx`（3.1、3.7）。
3. 拖拽用命中区的 list-item，不用 tree-item（3.7）。jsdom 里不能拖：落点是纯函数、有单测，接线由 PG3 在真实浏览器里覆盖。
4. `field.title.*` 没有补：改名复用笔记本名的规则与文案（3.11）。
5. 增强的注册表在 `reading/enhancement.ts`，由组合根 `main.tsx` 经 context 交给阅读视图；context 默认是空列表，测试默认没有增强（3.8）。
6. Worker 不加 WebWorker 的 lib（与 DOM 的冲突），只给用到的全局作用域写类型；高亮函数 Worker 与测试共用。白名单另放过子作用域的后缀与嵌入语言的 `language-*`（3.9）。
7. 编辑者树里的链接 `draggable={false}`：S5 的真实浏览器拖拽发现，从标题上拖起的是链接，不是登记过的那一行（3.6）。
8. 快速切换挂在 `NotebookLayout`，笔记本的首页、设置、页面都生效（3.10）。
9. 审查之后：
   - 删掉的页立刻离开树，`removed` 可观察；读到的树与原来相同时保留原数组（3.4）；
   - 对话框关闭之后的焦点按 `data-actions-of` 找到这一页的菜单按钮，删除的去处按 store 判断（3.7）；
   - 展开项不提供"之后"，指示线从页名的层级起画；深度 10 的页没有"新建子页"；新建只在地址没变时前往新页；树里一个"最近一次写的失败"；"移动到…"的选项失效时回到原位（3.7）；
   - 快速切换不叠在别的对话框上、活动项夹在长度之内、状态在 `<output>`、前往了别的页不还原焦点、取 `code` 位置的拉丁字母（3.10）；
   - `assets/` 下的脚本带 `default-src 'none'`（3.9、3.12）；
   - 切换器在工作区的 `nav` 里（3.2）；面包屑的分隔符对读屏器隐藏（3.5）；
   - 表格保持表格，宽的随阅读视图横向滚动（3.8）；
   - `ConfirmDialog` 由触发按钮打开或由调用者持有、二者择一，`HeldDialog` 一处（3.1）；`writesPages` 在 `app/effective-role.ts`，只认编辑者与管理员（3.3）。

**留给后面的**：

- **M4 收尾的待定项**（写进 M4 总设计第 8 节）：
  - `document.title` 随页面变化（第 6 节）；
  - 正文里同站链接的应用内跳转（现在是整页导航，3.12）；
  - 删除之后的导航改在答复处统一做：旧位置渲染出的 `<Navigate>` 可能盖过同时发起的导航（审查 Q3）；新建的答复只看已提交的地址，导航还在 transition 里时看不到（修复核对的 Nit）；
  - 宽表格外包一层可滚动、可聚焦的区域，由渲染器输出（审查 Q4、修复核对 mn-1）。
- **给 P6**：页面外壳与阅读视图的 SWR 键是 `["page-view", id]`，编辑器保存之后经它重读。
- **已落实的移交**：M3 移交第 1–3 项（3.2、3.3）；P4 留给 P5 的 503 `server_busy`：阅读视图按 `Retry-After` 重读，新建的失败显示"服务器繁忙"。
