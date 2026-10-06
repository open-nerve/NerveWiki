# M4/P6 源码编辑器：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P6 前端：源码编辑器 |
| 状态 | 进行中（已合并，待负责人执行[输入法清单](manual/P6-ime-checklist.md)） |
| 基线 | `31723c8`（P5 合并、文档提交之后的 main）；本文与各 Step 计划提交之后开分支 |
| 上级文档 | [M4 总设计](00-M4-design.md) 第 3、4、7、8 节；[P4](04-P4-content-sessions.md) 3.4、3.5、第 7 节；[P5](05-P5-tree-reading.md) 3.4、3.5、3.10；[M0/P1 编辑器移交](handoffs/M0-P1-editor.md)；[M0/P1 实验 ④](../M0-foundation/01-P1-spikes.md)；[总体设计](../v0.1-design.md) 9.3、9.4 |

---

## 1. 基线

后端齐了（P4）：`getPageContent`（原样的正文、`revision`、`content_hash`）、`putPageContent`（`content`、`base_revision`、可选的 `edit_session_id`；答 `Page`，带新的 `revision`）、`openEditSession`（201，租约 60 秒）、`heartbeatEditSession`、`endEditSession`。码：正文写 422 `validation_failed`（超过 5 MiB 或含 NUL）、403、404 `page.not_found`、409 `page.edit_session_ended`、409 `page.revision_mismatch`、503 `server_busy`（带 `Retry-After`），请求体在 `read_timeout` 之后才到齐答 400 `bad_request`；心跳与结束 404 `page.edit_session_not_found`，心跳另有 403。服务端的常量在 `server/internal/modules/page/domain/session.go`（`EditSessionLease` 60 秒、`EditSessionHeartbeat` 20 秒），注释已说网页的编辑器持有同样的两个数。

前端（P5 留下的）：

- 页面外壳 `PageLayout` → `PageShell`（按页面 id 重新挂载）：面包屑、`h1`（`useArrivalFocus`）、`ReadingView`（SWR 键 `["page-view", id]`）、子页面列表。`writesPages(role)` 在 `app/effective-role.ts`；`isMod`、`onMac` 在 `app/shortcuts.ts`；快速切换在 `NotebookLayout` 里接 `Mod+O`。
- 没有编辑器，依赖里没有 CodeMirror；没有 `useBlocker`、`beforeunload`。`page.save`、`page.saved` 与 `page.*` 的 problem 文案已有。
- CSP 是 `style-src 'self' 'unsafe-inline'`：CodeMirror 经 `style-mod` 注入的样式（`adoptedStyleSheets`，不支持时 `<style>`）不受阻。
- M0/P1 实验 ④：`EditorView` 只建一次、属性经事务传入、StrictMode 下 DOM 里只有一个编辑器；CDP 模拟的拼音组合在 Chromium 上正常；`markdown()` 的分包 528 KB（gzip 183 KB），大头是它静态依赖的 `lang-html`（带进 CSS、JavaScript 的语言包）。
- e2e：PG7–PG10 只有接口版本；PG12 的页面版本核对了阅读者没有新建、菜单与拖拽，还没有编辑入口。

## 2. 目标与范围

**目标**：写者在页面上按 `Mod+E` 进入源码编辑，写 Markdown，`Mod+S` 保存，再按 `Mod+E` 回到阅读视图；每次编辑只改动到的字节，换行写法与 BOM 照旧；一次编辑是一个会话、一个变更集；别人先写了时看得到差异，可以保留自己的或放弃；离开前有未保存的修改先提醒。

**做**：

- CodeMirror 6 的编辑器分包（只在进入编辑时加载）：Markdown 高亮（标题加粗加大、符号保留）、`Mod+B`、`Mod+K`、列表续行、Tab 缩进、查找替换。
- 换行写法与 BOM 的记录（M4 总设计第 4 节"编辑器的字节保真"）。
- 编辑器扩展管线（M4 总设计第 8 节），以 M5、M6、M7 形态的示例扩展测试（M0/P1 移交第 2 项）。
- 编辑模式：正文的读取、会话的开启、心跳、重开与结束；保存（`Mod+S`、按钮）、退出前保存、未保存提醒（`useBlocker`、`beforeunload`）；输入法组合中不保存。
- 冲突：409 `page.revision_mismatch` 时读出当前正文，用 `@codemirror/merge` 显示差异（再按需加载），"保留我的"或"放弃我的"。
- 分包体积的评估（M0/P1 移交第 3 项）；人工输入法验收清单（第 1 项）；给 M5 的输入法移交的内容。
- 文案；vitest；e2e 的 PG7–PG10 页面版本、PG12 的编辑入口、Chromium 上 CDP 模拟的组合输入。

**不做**：自动保存、编辑锁、推送（M5）；`[[` 与 `#` 的补全（M6）；粘贴与拖入上传（M7）；Live Preview（总体设计 9.3 不做）；编辑模式进地址（M4 总设计第 4 节：刷新回到阅读视图）。

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  main.tsx                                组合根：把编辑器扩展（editorExtensions，M4 为空）经 EditorExtensions 交给编辑器
  vite.config.ts                          + 构建时的检查：主包不含 CodeMirror；CodeMirror（除 merge）一个命名的分包（3.11）
  app/shortcuts.ts                        + dialogOpen（快速切换与页面外壳共用）
  services/page.service.ts                + getPageContent、putPageContent、openEditSession、heartbeatEditSession、endEditSession
  stores/page-editing.ts                  编辑的状态：正文的基准版本、会话（开启、心跳、重开、结束）、保存的队列与结果、冲突；
                                          editSessionHeartbeat（与服务端的常量互指）；不引用 CodeMirror
  stores/root.store.ts                    + editPage(id)：新的一次编辑（context.tsx 的 useNewPageEditing，组件持有）
  pages/page/page-layout.tsx              阅读与编辑的切换；阅读时的 Mod+E；"编辑"按钮（写者）
  pages/page/page-edit.tsx                编辑模式：加载编辑器分包、编辑时的 Mod+S 与 Mod+E、保存、离开、冲突的两个按钮
  pages/page/page-editing-bar.tsx         编辑时的状态与按钮：保存、完成；状态在 <output> 里
  pages/page/conflict-panel.tsx           冲突区：标题、差异（再按需加载）、保留我的、放弃我的
  pages/page/unsaved-guard.tsx            有未保存的修改时：路由内的离开先确认（useBlocker），关标签页先提醒（beforeunload）
  editor/registry.ts                      编辑器扩展的类型、EditorExtensions、注册表（只有类型引用 CodeMirror）
  editor/source-editor.tsx                分包入口：SourceEditor（EditorView 只建一次；正文换了新建 EditorState）
  editor/line-breaks.ts                   换行写法与 BOM：拆开、拼回、随改动映射的 StateField、撤销时恢复
  editor/markdown.ts                      Markdown 语言、高亮样式、列表续行与删除标记的命令
  editor/commands.ts                      Mod+B、Mod+K 的命令
  editor/extensions.ts                    扩展管线：每个扩展一个 Compartment，按注册顺序组合，单独换掉或卸下
  editor/theme.ts                         取应用的 CSS 变量，明暗两套共用一份
  editor/phrases.ts                       CodeMirror 的界面短语（查找替换面板）按界面语言
  editor/conflict-view.tsx                冲突的差异（@codemirror/merge 的 unifiedMergeView，再按需加载）
  test/page-server.ts                     + 正文与编辑会话的假服务
e2e/
  fixtures/wiki-pages.ts                  + 编辑器：进入、正文、状态、保存、冲突区
docs/v0.1/M4-pages/manual/P6-ime-checklist.md   人工输入法验收清单
```

### 3.2 编辑器分包与语言

- 依赖（锁定版本）：`@codemirror/state`、`view`、`commands`、`language`、`search`、`lang-markdown`、`merge`，`@lezer/highlight`（`@lezer/markdown` 原先只经 `lang-markdown` 间接引入；M6 收尾起直接声明，只做类型导入）。`editor/source-editor.tsx` 是分包入口，由页面外壳 `lazy(() => import(...))` 加载；冲突视图 `editor/conflict-view.tsx` 是另一个入口，冲突区（`pages/page/conflict-panel.tsx`）出现时才 `import()` 它，`@codemirror/merge` 随它进自己的分包。主包不引用 CodeMirror 的运行时（`editor/registry.ts` 只有类型），由构建产物的检查守住（3.11）。
- 语言：不调用 `markdown()`（它带进 `lang-html`、CSS、JavaScript 的语言包），用 `lang-markdown` 导出的 `markdownLanguage`（CommonMark + GFM 的 lezer 解析器，另带下标、上标与 Emoji：`~x~` 在编辑器里高亮成下标，阅读视图里是删除线，只差在高亮；换成 `commonmarkLanguage` 加 GFM 要另带 `@lezer/markdown`、照抄 `lang-markdown` 的语言包装，不值得）与它的命令 `insertNewlineContinueMarkup`、`deleteMarkupBackward`；代码块里的嵌套高亮放弃。三个包都标了 `sideEffects: false`，tree-shaking 去掉了 `lang-html`：编辑器分包 341 kB（gzip 111 kB），`markdown()` 是 527 kB（gzip 183 kB），选前者（第 7 节；M4 总设计第 4 节"编辑器分包"）。M6 收尾修订：`editor/markdown.ts` 以 `markdownLanguage` 的 data 与加了两个上限的解析器自建 `Language`（`markdownEditor`），命令按 data 认语言，照常可用（总体设计 13.2 第 25 条）。CodeMirror（除 `@codemirror/merge`）放进命名的 `codemirror` 分包，像 React 一样跨版本缓存；merge 随差异视图另成一个分包。
- 高亮：`HighlightStyle`，标题一到六级加粗、逐级加大，强调、粗体、删除线、行内代码与代码块等宽，链接与地址着色，符号（`#`、`*`、`` ` ``、`[`）照原样显示、颜色淡一些。颜色取应用的 CSS 变量（`--foreground`、`--muted-foreground`、`--border` 等），明暗两套不必切换扩展。应用里没有链接色与高亮色（`--primary` 是中性灰）：链接与查找匹配用编辑器自己的变量，`.dark` 下另一套，与阅读视图的代码高亮相同做法；差异的折叠行取 `--accent` 与 `--muted-foreground`（merge 自带的是浅色编辑器的颜色）。
- lezer 的语法树只用于编辑辅助（高亮、列表续行），不做语义判断（总体设计 9.3）。

### 3.3 换行写法与 BOM

CodeMirror 把 `\r\n`、`\r`、`\n` 都当作换行，取文本时一律写成 `\n`。`editor/line-breaks.ts`：

- **拆开**（`splitBreaks(raw)`）：去掉开头的 BOM（记下有没有）；按 `/\r\n|\r|\n/` 找出每个换行的写法；**主写法**是出现最多的那种，并列时取文档里先出现的，没有换行时是 `\n`。文档交给 CodeMirror 的是去掉 BOM、换行都是 `\n` 的文字。
- **记录**：一个 `StateField`，值是 BOM 的有无、主写法与一个 `RangeSet`：每个写法不同于主写法的换行，在它所在行的行尾放一个点标记（值是写法）。标记随改动映射，映射方式 `MapMode.TrackAfter`：行尾打字、删掉行尾的字，标记仍在行尾；换行本身被删掉（行首退格、选区跨过它、替换），标记随之消失。新插入的换行（回车、列表续行、粘贴、替换）没有标记，即主写法。
- **撤销**：一次改动删掉了标记，经 `invertedEffects`（`@codemirror/commands`）把它们记进历史；撤销时恢复，重做时照常删掉。撤销删掉的换行，回来的是原来的写法。恢复的位置就是撤销重新插入换行的位置（历史按之后的改动映射过），不另核对。
- **拼回**（`joinBreaks(state)`）：BOM（若有）加上每一行与它之后的换行（有标记取标记的写法，否则主写法）。只在保存时调用一次（O(n)），不随每次按键。
- **没有看不见的字符**：`\r` 与 BOM 都不在文档里：光标落不到 `\r` 之后，行首退格删掉的是整个换行，`Mod+Home` 之后打的字在 BOM 之后。
- 往返：没改动时 `joinBreaks(splitBreaks(raw))` 与 `raw` 逐字节相同，CRLF、单独的 `\r`、混合的换行、`\r\r\n`、末尾有无换行、空文档、只有 BOM 都有测试；行尾空白、NFD 的文字 CodeMirror 原样保留（PG9）。

### 3.4 编辑器扩展管线

`editor/registry.ts`：

```ts
type EditorContext = { workspace: string; notebook: string; page: string; role: NotebookRole };
type EditorControls = {
  save(): Promise<void>;              // 与 Mod+S 相同：等输入法组合结束
  readonly saving: () => boolean;     // 会话与保存的状态（M5 的自动保存据此）
  setReadOnly(readOnly: boolean): void;
};
type EditorExtension = { name: string; extension(context: EditorContext, controls: EditorControls): Extension };
```

- 注册表 `editorExtensions`（M4 为空，组合根交空集合），经 `EditorExtensions` context 交给编辑器，与阅读视图的增强同一做法（P5 3.8）。
- `editor/extensions.ts` 的 `composeExtensions`：每个扩展放进自己的 `Compartment`，排在编辑器自己的扩展（语言、键位、历史、换行记录）之后、按注册顺序；返回的 `reconfigure` 可以按名字换掉或卸下（换成空）；M4 没有调用者，编辑器不对外给它，M5 需要从外面换掉扩展时再接到句柄上（[给 M5 的移交](../M5-collab-editing/handoffs/M4-P6-editor.md)第 5 项）。一个扩展的构造抛错时记 `console.error`、不装它，其余照常。
- 只读：编辑器自己的一个 `Compartment` 放 `EditorState.readOnly` 与 `EditorView.editable`（总体设计 9.3）。扩展的 `setReadOnly` 与句柄的 `hold`（离开编辑时，3.7）各记一份，任一为真即只读；记在编辑器里，载入新的正文照样只读；扩展构造时就能调用（视图先建、再设 state）。
- 测试（M0/P1 移交第 2 项）：M5 形态（只读的切换、更新之后去抖调用 `save`）、M6 形态（`autocompletion` 的补全源）、M7 形态（`paste` 的 DOM 事件处理）三个示例按顺序组合，各自生效、互不干扰，卸下其中一个其余照常，换掉一个之后新的生效。

### 3.5 SourceEditor

- `EditorView` 只建一次（`useLayoutEffect`）；正文换了（放弃我的）用 `view.setState` 换新的 `EditorState`，撤销历史不跨正文（总体设计 9.3）。视图、改动的计数与等组合结束的动作在一个不是组件的 `EditorHost` 里，组件只经 ref 把最新的属性交给它：扩展不因渲染重建。`focusOnOpen` 时建好就聚焦（用户要编辑时）。StrictMode 下 DOM 里只有一个编辑器（M0 实验 ④ 的检查）。
- 对外的句柄（`ref`）：`text()`（`joinBreaks`）、`version()`、`focus()`、`whenComposed(act)`（没有组合就立刻做，有就等它结束：之后的第一次更新，或 `compositionend` 之后 50 毫秒，有的浏览器在它之后才给文字）、`load(raw)`、`hold(on)`。改动经 `onChange(version)` 告诉外面：`version` 每次改动加一，保存记下发出时的 `version`，成功之后与当前的相同才算没有未保存的修改；撤销回到保存时的文字仍算未保存（不比较文字，5 MiB 的正文不在每次按键时拼回）。
- 无障碍：内容区 `aria-label`（"页面正文"）、`aria-multiline`；Tab 缩进（`indentWithTab`），按 Esc 之后 Tab 离开编辑器（CodeMirror 的 tab focus mode）：编辑器下方一行可见的说明，内容区的 `aria-describedby` 指向它。标签与面板的短语随界面语言重配。
- 键位：编辑器自己的 `Mod+B`、`Mod+K`、列表续行（`Enter`）、删除标记（`Backspace`）、Tab，`defaultKeymap`、`historyKeymap`、`searchKeymap`。`Mod+S`、`Mod+E` 不进编辑器的键位：由页面外壳在 `document` 上接（3.7），焦点在按钮上时一样生效。
- `Mod+B`：选区两边加 `**`，已有就去掉；空选区插入 `****`、光标在中间。`Mod+K`：选区变成 `[选区]()`、光标在括号里；空选区插入 `[]()`、光标在方括号里。列表续行照 CodeMirror：空项上回车先在它前面空一行，再回车结束列表；标记后退格去掉标记、留下缩进。
- 查找替换：`@codemirror/search` 的面板（`Mod+F`；替换在同一面板里）；面板的短语经 `EditorState.phrases` 按界面语言（`editor/phrases.ts`，中英两份同键的测试）。

### 3.6 编辑的状态（`stores/page-editing.ts`）

一页一次编辑一个 `PageEditing`（MobX；进入编辑时由 `RootStore.editPage` 新建，编辑模式的组件持有，退出时丢），不进 `RootStore` 的缓存。不引用 CodeMirror：正文以字符串进出。

- **开始**：同时读正文（`getPageContent`，每次进入都读，不用缓存：旧的正文会让第一次保存就冲突）与开启会话；正文到了编辑器才显示（之前是 `NotLoaded`，带"重试"）。会话没开成不挡编辑：保存时再开。结束了还能再开始（StrictMode 下 effect 跑两次）：正文只读一次（在途的读复用，读到之后再开始不再读），正在开的会话照常留下。
- **心跳**：每 `editSessionHeartbeat`（20 秒）一次；答 404 `page.edit_session_not_found` 就重开；标签页重新可见时（`visibilitychange`）立刻补一次（后台标签页的计时器会被节流，M4 总设计第 10 节）。答 403 时停止心跳，显示失去编辑权限，文字留在编辑器里可以复制；之后的保存成功就复位，心跳重新开始。
- **保存**：`save(text)` 排队，一次只有一个在途；在途时再按保存，答复之后若还有新的修改再存一次。请求 `{content, base_revision, edit_session_id}`：
  - 200：基准版本换成答复的 `revision`，记下保存时的 `version`；缓存里的 `["page-view", id]` 清掉，回到阅读视图时重新读（清除时不带 `revalidate: false`：那样不撤销 SWR 的去重，两秒内回来会拿到旧的那次读）。
  - 409 `page.edit_session_ended`：重开会话，用同一份正文再存一次（只重试一次）；会话已被心跳换成新开的就用它，不再开；PG10。
  - 409 `page.revision_mismatch`：读当前正文，进入冲突（3.8）。
  - 503 `server_busy`：按 `Retry-After` 自动再存，至多三次，状态写"服务器繁忙，稍后自动重试"（P4 留给 P6）。
  - 400 `bad_request`：说明网络太慢、正文没有及时传完（P4 留给 P6 的带宽前提）。
  - 422、403、404：`errorText`，文字留在编辑器里；正文超过 5 MiB、含 NUL 的 422 说明是哪一条（`field.content.*`）。
- **结束**：退出编辑、离开页面（组件卸载）时 `endEditSession`，不等答复；404 当作已结束。结束之后什么也不再发：排队的、等 503 的（立刻醒来）、等会话开启的保存都以失败结束，结束之后才开成的会话随即结束；在途的保存答复之后不重试、不读冲突，成功了也不再起心跳。关掉标签页的会话一个租约之内过期（M4 总设计第 4 节）。
- 常量 `editSessionHeartbeat`（20 秒）在 `stores/page-editing.ts`，注释指向服务端的 `EditSessionHeartbeat` 与 `EditSessionLease`，测试钉住数值（13.1 第 15 条）；租约前端用不到，只在服务端。

### 3.7 进入与退出

- 写者（`writesPages(notebook.role)`）的页面标题旁有"编辑"按钮；阅读者没有，`Mod+E` 也不做什么（PG12）。
- 页面外壳在 `document` 上接 `Mod+E`、`Mod+S`（`isMod`），有对话框开着时不接（与快速切换相同）：
  - 阅读时 `Mod+E` 进入编辑（按住不放只进一次），加载分包之后焦点进编辑器；之前焦点在页面标题上（"编辑"按钮已卸载），读正文失败时停在那里，下面是未加载与"重试"。`Mod+S` 在阅读时不拦（浏览器的"保存网页"照常）。
  - 编辑时 `Mod+S` 保存，`Mod+E` 与"完成"按钮：有未保存的修改先保存，成功之后结束会话、把重新读到的阅读视图直接放进 SWR 的缓存（hook 刚卸载又挂上时 SWR 的 2 秒去重会给旧的；读失败也照常回去），回到阅读视图，焦点给"编辑"按钮；保存失败时留在编辑。离开一次只有一个（连按"完成"、按住 `Mod+E` 也只离开一次），从头到尾编辑器只读（`hold`；只读的内容区 `tabindex="-1"`，焦点留得住），离开的保存与重读之间打的字不会无声地丢掉；保存失败时放开，焦点回编辑器（有冲突时在冲突区的标题）；保存成功之后仍有未保存的修改（扩展用代码改的）就留在编辑；离开途中组件卸载了就到此为止。
  - 输入法组合中（`view.composing`）按 `Mod+S`、`Mod+E`，或按"保存""完成""保留我的"：阻止浏览器的默认行为，记下要做的事，组合结束（`compositionend` 之后的第一次更新）再做，存进去的是确认后的文字，不是半截拼音（M0/P1 移交第 1 项）。
- 编辑时的状态栏（`page-editing-bar.tsx`）：`<output>` 里是"有未保存的修改"、"保存中…"、"已保存"或失败的原因；"保存"与"完成"按钮。编辑器替换阅读视图，面包屑、标题与子页面列表照旧。
- **未保存提醒**（`unsaved-guard.tsx`，随编辑模式常驻，有未保存的修改时才拦）：路由内的离开（树、面包屑、快速切换、后退）由 `useBlocker` 拦下，确认框"离开而不保存？"：留下（焦点回编辑器，冲突区开着时回它的标题）、离开（离开时会话随组件卸载结束，等着的保存不再发）；确认框开着时保存落地，就放行这次导航；只换了查询或锚点不拦。关标签页、刷新由 `beforeunload` 提醒。本标签页删掉这一页时外壳去父页（`removedTo`），别处删掉的在树重读之后显示 404；两种都是编辑器随之卸载，不拦（3.10）。

### 3.8 冲突

- 409 `page.revision_mismatch`：编辑器保留自己的文字，`getPageContent` 读出当前正文与 `revision`（读失败时 409 是这次保存的失败），编辑器上方出现冲突区（以标题命名的 `section`，标题"这一页在你编辑时被改过"）：`@codemirror/merge` 的 `unifiedMergeView` 在一个只读的视图里显示当前正文到"我的"（被拒的那次保存发出的文字）的差异，没改的长段折起（折叠行的文字按界面语言），点它展开一段，"显示未改动的行"（`aria-pressed`，键盘可达）全部展开；按 CodeMirror 的文字比较，只差在换行写法的不显示。
- "保留我的"：以冲突区所依据的那次读取的 `revision` 为 `base_revision` 再存编辑器里现在的文字（冲突之后的修改不丢，说明里写明），之后焦点回编辑器（失败时也一样，原因在状态栏）；其间又有别人写，就再次 409、冲突区换成新的差异（M4 总设计第 4 节"保存冲突"）。
- "放弃我的"：编辑器载入当前正文（新的 `EditorState`，换行记录按它重新拆），基准版本换成它的 `revision`，没有未保存的修改。
- 冲突区开着时，`Mod+S`、"保存"、`Mod+E`、"完成"都不发请求，只把焦点移到冲突区：先在两个按钮里选一个。冲突区出现时焦点移到它的标题。
- 差异视图在自己的分包里按需加载；加载失败时冲突区说明并给"重试"，两个按钮仍可用（不看差异也能选）。不用 `lazy`：它的失败会到路由的错误边界，换掉整个页面，编辑器里的文字也就没了。

### 3.9 文案

新键放在 `editor.*` 与 `page.*`：编辑、完成、保存、有未保存的修改、保存中、已保存、服务器繁忙会自动重试、网络太慢、失去编辑权限、冲突区的标题与两个按钮、离开确认、页面正文的标签，查找替换面板的短语（`editor.phrase.*`）。中英两份同键（类型与占位符的测试）。

### 3.10 已知差异

- M4 没有锁：两个人（网页或 PAT）同时编辑同一页，后存的人看到冲突（PG8）；M5 的锁之后冲突只来自接口的写。
- 别的标签页或别人删掉了正在编辑的页：树重读之后显示 404（只有本标签页删掉的才去父页，`removedTo` 只记本代删除的页），编辑器随之卸载，未保存的文字不提醒就丢了（保存本来也会答 404）。M5 的推送与锁之后再看（[给 M5 的编辑器移交](../M5-collab-editing/handoffs/M4-P6-editor.md)第 6 项）。
- 差异按 CodeMirror 的文字比较，只差在换行写法的地方不显示。
- 后台标签页的心跳被节流时会话会过期，下一次保存重开会话（变更集分成两个，PG10）；M5 有锁之后锁会被别人拿走，见[给 M5 的移交](../M5-collab-editing/handoffs/M4-P4-edit-sessions.md)第 6 项。

### 3.11 构建与安全

- 主包不含 CodeMirror：`vite.config.ts` 里一个小插件在 `generateBundle` 时从入口出发，沿静态与动态 import 走到的 chunk（路由的也在内），到编辑器自己的两个入口（`editor/source-editor`、`editor/conflict-view`）为止（编辑器再按需加载的算编辑器的），含 `@codemirror`、`@lezer`、`style-mod`、`w3c-keyname`、`crelt` 的模块就让构建失败，报错写出从入口到它的路径（`make check` 的前端构建因此守住）。反向对照：编辑模式静态 `import` 一个编辑器模块，构建失败。主包 236 kB（gzip 74 kB），P6 之前 232 kB。
- CodeMirror 与 merge 的样式经 `style-mod` 注入，CSP 的 `style-src 'unsafe-inline'` 允许；不新增 Worker、不放宽 CSP。PG7 的页面版本核对编辑时没有 CSP 违规（e2e 的页面监视自动核对）。
- 正文只经 `putPageContent` 发出；冲突区与编辑器都只显示文字，不把正文当 HTML 挂进页面。

### 3.12 e2e 的页面版本

- 页面对象在 `fixtures/wiki-pages.ts`：进入编辑、编辑器的输入（`page.keyboard`）、保存、完成、冲突区、离开确认。
- PG7：`Mod+E` 进入、输入、`Mod+S`、再输入、`Mod+S`、`Mod+E` 回到阅读视图看到新内容；落库：两次保存一个变更集、一个正文版本，`revision`、`content_hash`、`byte_size`；未保存时点树里别的页先确认，留下之后文字还在。
- PG8：编辑时 PAT 先写；`Mod+S` 显示差异；"保留我的"之后正文是我的；另一页"放弃我的"之后编辑器是 PAT 的正文、再存不冲突。
- PG9：CRLF、单独的 `\r`、混合换行、BOM、行尾空白、NFD 的页面各一个测试，在编辑器里 `Mod+Home` 打字、行尾打字、回车、行首打字、行首退格之后保存，经接口读回的字节与预期逐字节相同（预期由测试按同样的规则写出；CodeMirror 的 Home 先停在行首的空白之后，退格的那一行不以空白开头）。
- PG10：心跳（`page.clock` 快进 20 秒，看到心跳请求、库里的 `expires_at` 后移）；完成之后会话的行没了；把会话在库里推到过期之后 `Mod+S`，编辑器重开会话、保存成功、没有丢字。
- PG12：阅读者的页面没有"编辑"，`Mod+E` 不进入编辑。
- 组合输入（PG9 的页面版本，Chromium，CDP 的 `Input.imeSetComposition` 与 `Input.insertText`）：组合中 `Mod+S` 不发请求，组合确认之后存进确认的文字；落在 CRLF 页面的行尾，其余字节不变。
- 预期的 409（冲突、过期的会话）由页面监视声明为预期的控制台错误。

### 3.13 人工验收与移交

- `docs/v0.1/M4-pages/manual/P6-ime-checklist.md`：在 Chromium、Safari、Firefox 上用真实的中文输入法：确认后的文字与光标；组合中按 `Mod+S`、`Mod+E` 不存进半截拼音、组合结束后再存；组合中点"完成"；在 CRLF 页面的行尾组合输入，保存后经接口核对字节。负责人在 P6 完成时执行，结果写进 P6 的审查记录（M4 总设计第 3 节）。
- 给 M5 的移交（已写进[移交文件](../M5-collab-editing/handoffs/M4-P6-editor.md)）：组合中不触发自动保存、组合结束后补存；组合进行中收到外部更新（推送）时不打断组合；后台标签页的心跳节流与锁；编辑器扩展管线的最后一跳测试（M5 的只读与自动保存是第一个注册者）。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 编辑器分包：依赖、语言与高亮、换行与 BOM 的记录、命令与键位、查找替换、扩展管线、`SourceEditor`；分包体积的评估；主包不含 CodeMirror 的检查 | [P6-S1](plans/P6-S1-editor-core.md) |
| S2 | 编辑模式：service、`PageEditing`（读取、会话、保存）、进入与退出、`Mod+E`/`Mod+S`、状态栏、未保存提醒、组合中不保存 | [P6-S2](plans/P6-S2-editing.md) |
| S3 | 冲突：409 的两种、差异视图、保留我的与放弃我的 | [P6-S3](plans/P6-S3-conflict.md) |
| S4 | e2e 的页面版本（PG7–PG10、PG12）与组合输入；人工验收清单 | [P6-S4](plans/P6-S4-e2e-manual.md) |

## 5. 测试与验证

| 层 | 内容 |
|---|---|
| vitest：换行与 BOM | 拆开与拼回的往返（各种换行、BOM、末尾、空文档）；行尾、行首、BOM 之后的输入；回车用主写法；行首退格、选区删除去掉标记；撤销恢复原写法、重做再删；粘贴多行 |
| vitest：编辑器 | 扩展管线（三个示例的组合、卸下、换掉、抛错的隔离）；`Mod+B`、`Mod+K`、列表续行；只读的切换；短语中英同键 |
| vitest：编辑的状态 | 开始（正文与会话并行，会话失败不挡）；心跳的间隔、404 重开、403 停止、可见时补一次；保存的队列、200 换基准、409 会话结束重开再存一次、409 版本不一致进冲突、503 按 `Retry-After` 至多三次、400 与 422 的文案；结束不等答复；常量的数值 |
| vitest：组件 | 写者有"编辑"、阅读者没有；`Mod+E` 进入与退出、退出先保存、保存失败留下；组合中的 `Mod+S` 延到组合结束；未保存时的离开确认与 `beforeunload`；冲突区的两个按钮 |
| 构建 | 主包不含 CodeMirror 的构建插件 |
| e2e | 3.12 的页面版本；之前的故事全部通过 |
| 人工 | 输入法清单（3.13） |
| 反向对照 | 每个新检查一个：如拼回不用标记、标记映射用 `TrackDel`、撤销不恢复、保存不排队、会话结束不重开、心跳不重开、组合中照存、退出不先保存、离开不确认、保留我的取 409 之后的版本 |

## 6. 完成标准

- S1–S4 的检查全部为绿：`make check`、`make gen-check`、`make e2e`、`make image-smoke`；持续集成为绿。
- M0/P1 编辑器移交三项：分包体积量出、写进第 7 节并选定；扩展管线的组合有测试；人工输入法清单写好，负责人执行通过、结果进审查记录。
- Opus 审查与修复，审查记录在 `reviews/P6-source-editor-review.md`。
- 给 M5 的输入法与会话的移交内容写进第 7 节，M4 收尾时已写进[移交文件](../M5-collab-editing/handoffs/M4-P6-editor.md)。

## 7. 结果

- 分支 `m4-p6-source-editor`：S1 `2cc081d`；S2 `6c6b7bd`；S3 `20c7b25`；S4 `66d32fa`；审查修复 `aa02f26`、`96fc5eb`、`500bb60`；`9a69fb2` 合并（`--no-ff`）。
- 门禁：每个 Step 与审查修复的 `make check` 为绿（web 的 vitest 最后 1350 个）；`make gen-check`、`make e2e`（151 个）、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P6 审查](reviews/P6-source-editor-review.md)。Critical 1、Major 1、Minor 12、Nit 10、疑问 6，合并之前全部处置（C1：离开编辑时打的字被丢掉；M1：StrictMode 下会话一开就被结束）；修复的核对另有 Major 1（两次离开交错时仍丢字）、Minor 2、Nit 7，第二轮修复处置。
- 反向对照：S1 21、S2 27、S3 13、S4（e2e）6，审查修复 49，全部失败。
- **人工验收**：[输入法清单](manual/P6-ime-checklist.md) 写好，待负责人在 Chromium、Safari、Firefox 上执行，结果写进审查记录的"人工验收"一节；通过之前 M4 不收尾。

**分包体积**（M0/P1 移交第 3 项；Vite 8.3.1、rolldown 1.2.11）：

| 写法 | 编辑器的分包 | gzip |
|---|---|---|
| `markdown()`（带进 `lang-html`、CSS、JavaScript） | 527.20 kB | 182.77 kB |
| `markdownLanguage` 加 `lang-markdown` 的命令（选用） | 341.45 kB | 111.10 kB |

选用的写法只少了代码块里的嵌套高亮（设计已放弃）。合并之后，第一次编辑下载 `codemirror` 分包 341 kB（gzip 110.5 kB）与编辑器自己的 8 kB；冲突的差异 20 kB（gzip 7.6 kB，含 merge）只在冲突时下载；主包 236 kB（gzip 74 kB），P6 之前 232 kB。

**与计划的出入**（已同步进上文）：

1. 心跳的常量只有一个 `editSessionHeartbeat`，在 `stores/page-editing.ts`，没有 `edit-session-timing.ts`；租约前端用不到（3.6）。
2. 编辑由 `RootStore.editPage` 新建、编辑模式的组件持有，不经页面树的 store（3.6）；编辑模式在 `page-edit.tsx`，冲突区在 `conflict-panel.tsx`（3.1）。
3. 离开编辑时把重新读到的阅读视图放进 SWR 的缓存（3.7）；保存成功时清掉缓存里的（审查 Q6：起初只在离开时放，编辑中直接去别的页再回来会先看到旧的）（3.6）。
4. 冲突里"我的"是被拒的那次保存发出的文字；差异的分包不用 `lazy`，加载失败不走路由的错误边界（3.8）。
5. CodeMirror（除 merge）是命名的分包，跨版本缓存（3.2、3.11）。
6. 撤销恢复的换行写法不另核对位置：撤销总在那里重新插入换行（3.3）。
7. 列表续行照 CodeMirror 的行为（3.5）；`SourceEditor` 的 CodeMirror 部分在 `EditorHost` 里，句柄多了 `version()`、`whenComposed()` 与 `hold()`（审查 C1），聚焦的属性叫 `focusOnOpen`（3.5）。
8. 组件测试用真的 CodeMirror（jsdom 里能跑），组合中用 spy 让 `EditorView.composing` 为真；PG9 的页面版本每种正文一个测试，组合输入的 e2e 在 PG9 里（3.12）。
9. `dialogOpen()` 抽到 `app/shortcuts.ts`，快速切换与页面外壳共用（3.1）。
10. 审查修复（见审查记录）：离开编辑一次一个、从头到尾只读，只在没有未保存的修改时离开；`UnsavedGuard` 常驻；编辑器下方写明怎样用键盘离开；冲突区的"显示未改动的行"；`PageEditing` 结束之后可以再开始、结束之后什么也不发；构建检查沿静态与动态 import 走到编辑器的入口为止（3.4–3.8、3.11）。

**留给后面的**：

- **给 M5**（已写进[移交](../M5-collab-editing/handoffs/M4-P6-editor.md)，M0/P1 编辑器移交第 1 项的后半）：
  - 自动保存在组合中不触发、组合结束后补存：`SourceEditor` 的 `whenComposed` 就是为它准备的；
  - 组合进行中收到推送的外部更新时不打断组合（总体设计 9.3 的"只替换差异、不进撤销历史"）；外部更新不进历史时，撤销恢复的换行写法按历史映射的位置放回，推送接进来时要复核这一点；
  - 后台标签页的心跳会被节流，会话过期之后下一次保存重开（变更集分成两个）；有锁之后锁会被别人拿走，要按页面的可见性调整心跳或租约；
  - 编辑器扩展管线的最后一跳测试（只读与自动保存是第一个注册者），按人工清单补验自动保存与推送的部分；
  - `composeExtensions` 的 `reconfigure` 编辑器没有对外给，M5 要从外面换掉或卸下扩展时接到 `SourceEditorHandle` 上（审查 m9①）；
  - `controls.save()` 等输入法组合结束；组合中编辑器被销毁，它的 promise 不 settle（修复的核对 nt-3），自动保存若要等它，先补上；
  - 别人删掉正在编辑的页、或失去访问时，未保存的文字随编辑器卸载而丢（3.10，审查 Q2），有推送与锁之后再看。
- **给 M6、M7**：编辑器扩展的上下文与控制（`EditorContext`、`EditorControls`）照 3.4；补全（M6）与粘贴上传（M7）各放一个扩展，管线的测试里有它们形态的示例。
- **已落实的移交**：M0/P1 编辑器移交第 2、3 项（扩展管线的组合测试、分包体积）；第 1 项的清单写好，待负责人执行。
