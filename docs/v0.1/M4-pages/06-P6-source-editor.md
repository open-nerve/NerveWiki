# M4/P6 源码编辑器：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M4/P6 前端：源码编辑器 |
| 状态 | 进行中 |
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
  vite.config.ts                          + 构建时的检查：主包不含 CodeMirror（3.11）
  services/page.service.ts                + getPageContent、putPageContent、openEditSession、heartbeatEditSession、endEditSession
  stores/page-editing.ts                  编辑的状态：正文的基准版本、会话（开启、心跳、重开、结束）、保存的队列与结果；不引用 CodeMirror
  stores/edit-session-timing.ts           editSessionLease、editSessionHeartbeat（与服务端的常量互指）
  pages/page/page-layout.tsx              阅读与编辑的切换；Mod+E；"编辑"按钮（写者）
  pages/page/page-editing-bar.tsx         编辑时的状态与按钮：保存、完成；状态在 <output> 里
  pages/page/unsaved-guard.tsx            有未保存的修改时：路由内的离开先确认（useBlocker），关标签页先提醒（beforeunload）
  editor/registry.ts                      编辑器扩展的类型、EditorExtensions、注册表（只有类型引用 CodeMirror）
  editor/source-editor.tsx                分包入口：SourceEditor（EditorView 只建一次；正文换了新建 EditorState）
  editor/line-breaks.ts                   换行写法与 BOM：拆开、拼回、随改动映射的 StateField、撤销时恢复
  editor/markdown.ts                      Markdown 语言、高亮样式、列表续行与删除标记的命令
  editor/commands.ts                      Mod+B、Mod+K 的命令
  editor/extensions.ts                    扩展管线：每个扩展一个 Compartment，按注册顺序组合，单独换掉或卸下
  editor/theme.ts                         取应用的 CSS 变量，明暗两套共用一份
  editor/phrases.ts                       CodeMirror 的界面短语（查找替换面板）按界面语言
  editor/conflict-view.tsx                冲突的差异（@codemirror/merge，再按需加载）
e2e/
  fixtures/wiki-pages.ts                  + 编辑器：进入、输入、保存、完成、冲突的对话框
docs/v0.1/M4-pages/manual/P6-ime-checklist.md   人工输入法验收清单
```

### 3.2 编辑器分包与语言

- 依赖（锁定版本）：`@codemirror/state`、`view`、`commands`、`language`、`search`、`lang-markdown`、`merge`，`@lezer/markdown`、`@lezer/highlight`。`editor/source-editor.tsx` 是分包入口，由页面外壳 `lazy(() => import(...))` 加载；冲突视图在分包里再 `import()` 一次 `@codemirror/merge`。主包不引用 CodeMirror 的运行时（`editor/registry.ts` 只有类型），由构建产物的检查守住（3.11）。
- 语言：不调用 `markdown()`（它带进 `lang-html`、CSS、JavaScript 的语言包），用 `lang-markdown` 导出的 `markdownLanguage`（CommonMark + GFM 的 lezer 解析器）与它的命令 `insertNewlineContinueMarkup`、`deleteMarkupBackward`；代码块里的嵌套高亮放弃。三个包都标了 `sideEffects: false`，期望 tree-shaking 去掉 `lang-html`。S1 量出两种写法的分包体积（gzip 前后），写进第 7 节，选小的那个，除非它丢了第 2 节的某项功能（M4 总设计第 4 节"编辑器分包"）。
- 高亮：`HighlightStyle`，标题一到六级加粗、逐级加大，强调、粗体、删除线、行内代码与代码块等宽，链接与地址着色，符号（`#`、`*`、`` ` ``、`[`）照原样显示、颜色淡一些。颜色取应用的 CSS 变量（`--foreground`、`--muted-foreground`、`--primary` 等），明暗两套不必切换扩展。
- lezer 的语法树只用于编辑辅助（高亮、列表续行），不做语义判断（总体设计 9.3）。

### 3.3 换行写法与 BOM

CodeMirror 把 `\r\n`、`\r`、`\n` 都当作换行，取文本时一律写成 `\n`。`editor/line-breaks.ts`：

- **拆开**（`splitBreaks(raw)`）：去掉开头的 BOM（记下有没有）；按 `/\r\n|\r|\n/` 找出每个换行的写法；**主写法**是出现最多的那种，并列时取文档里先出现的，没有换行时是 `\n`。文档交给 CodeMirror 的是去掉 BOM、换行都是 `\n` 的文字。
- **记录**：一个 `StateField`，值是 BOM 的有无、主写法与一个 `RangeSet`：每个写法不同于主写法的换行，在它所在行的行尾放一个点标记（值是写法）。标记随改动映射，映射方式 `MapMode.TrackAfter`：行尾打字、删掉行尾的字，标记仍在行尾；换行本身被删掉（行首退格、选区跨过它、替换），标记随之消失。新插入的换行（回车、列表续行、粘贴、替换）没有标记，即主写法。
- **撤销**：一次改动删掉了标记，经 `invertedEffects`（`@codemirror/commands`）把它们记进历史；撤销时恢复，重做时照常删掉。撤销删掉的换行，回来的是原来的写法。
- **拼回**（`joinBreaks(state)`）：BOM（若有）加上每一行与它之后的换行（有标记取标记的写法，否则主写法）。只在保存时调用一次（O(n)），不随每次按键。
- **没有看不见的字符**：`\r` 与 BOM 都不在文档里：光标落不到 `\r` 之后，行首退格删掉的是整个换行，`Mod+Home` 之后打的字在 BOM 之后。
- 往返：没改动时 `joinBreaks(splitBreaks(raw))` 与 `raw` 逐字节相同，CRLF、单独的 `\r`、混合的换行、`\r\r\n`、末尾有无换行、空文档、只有 BOM 都有测试；行尾空白、NFD 的文字 CodeMirror 原样保留（PG9）。

### 3.4 编辑器扩展管线

`editor/registry.ts`：

```ts
type EditorContext = { workspace: string; notebook: string; page: string; role: NotebookRole };
type EditorControls = {
  save(): Promise<void>;              // 与 Mod+S 相同
  readonly saving: () => boolean;     // 会话与保存的状态（M5 的自动保存据此）
  setReadOnly(readOnly: boolean): void;
};
type EditorExtension = { name: string; extension(context: EditorContext, controls: EditorControls): Extension };
```

- 注册表 `editorExtensions`（M4 为空，组合根交空集合），经 `EditorExtensions` context 交给编辑器，与阅读视图的增强同一做法（P5 3.8）。
- `editor/extensions.ts` 的 `composeExtensions`：每个扩展放进自己的 `Compartment`，排在编辑器自己的扩展（语言、键位、历史、换行记录）之后、按注册顺序；返回的句柄可以按名字换掉（`reconfigure`）或卸下（换成空）。一个扩展的构造抛错时记 `console.error`、不装它，其余照常。
- 只读：编辑器自己的一个 `Compartment` 放 `EditorState.readOnly` 与 `EditorView.editable`，`setReadOnly` 切换它（总体设计 9.3）。
- 测试（M0/P1 移交第 2 项）：M5 形态（只读的切换、更新之后去抖调用 `save`）、M6 形态（`autocompletion` 的补全源）、M7 形态（`paste` 的 DOM 事件处理）三个示例按顺序组合，各自生效、互不干扰，卸下其中一个其余照常，换掉一个之后新的生效。

### 3.5 SourceEditor

- `EditorView` 只建一次（`useLayoutEffect`）；正文换了（进入编辑、放弃我的）用 `view.setState` 换新的 `EditorState`，撤销历史不跨正文（总体设计 9.3）。回调放在 ref 里，扩展不因渲染重建。StrictMode 下 DOM 里只有一个编辑器（M0 实验 ④ 的检查）。
- 对外的句柄（`ref`）：`text()`（`joinBreaks`）、`focus()`、`composing()`（`view.composing`）、`load(raw)`。改动经 `onChange(version)` 告诉外面：`version` 每次改动加一，保存记下发出时的 `version`，成功之后与当前的相同才算没有未保存的修改；撤销回到保存时的文字仍算未保存（不比较文字，5 MiB 的正文不在每次按键时拼回）。
- 无障碍：内容区 `aria-label`（"页面正文"）、`aria-multiline`；Tab 缩进（`indentWithTab`），按 Esc 之后 Tab 离开编辑器（CodeMirror 的 tab focus mode），写进帮助。
- 键位：编辑器自己的 `Mod+B`、`Mod+K`、列表续行（`Enter`）、删除标记（`Backspace`）、Tab，`defaultKeymap`、`historyKeymap`、`searchKeymap`。`Mod+S`、`Mod+E` 不进编辑器的键位：由页面外壳在 `document` 上接（3.7），焦点在按钮上时一样生效。
- `Mod+B`：选区两边加 `**`，已有就去掉；空选区插入 `****`、光标在中间。`Mod+K`：选区变成 `[选区]()`、光标在括号里；空选区插入 `[]()`、光标在方括号里。
- 查找替换：`@codemirror/search` 的面板（`Mod+F`；替换在同一面板里）；面板的短语经 `EditorState.phrases` 按界面语言（`editor/phrases.ts`，中英两份同键的测试）。

### 3.6 编辑的状态（`stores/page-editing.ts`）

一页一次编辑一个 `PageEditing`（MobX；进入编辑时建，退出时丢），由页面外壳持有，不进 `RootStore` 的缓存。不引用 CodeMirror：正文以字符串进出。

- **开始**：同时读正文（`getPageContent`，每次进入都读，不用缓存：旧的正文会让第一次保存就冲突）与开启会话；正文到了编辑器才显示（之前是 `NotLoaded`，带"重试"）。会话没开成不挡编辑：保存时再开。
- **心跳**：每 `editSessionHeartbeat`（20 秒）一次；答 404 `page.edit_session_not_found` 就重开；标签页重新可见时（`visibilitychange`）立刻补一次（后台标签页的计时器会被节流，M4 总设计第 10 节）。答 403 时停止心跳，显示失去编辑权限，文字留在编辑器里可以复制。
- **保存**：`save(text)` 排队，一次只有一个在途；在途时再按保存，答复之后若还有新的修改再存一次。请求 `{content, base_revision, edit_session_id}`：
  - 200：基准版本换成答复的 `revision`，记下保存时的 `version`；`["page-view", id]` 标为过期。
  - 409 `page.edit_session_ended`：重开会话，用同一份正文再存一次（只重试一次）；PG10。
  - 409 `page.revision_mismatch`：读当前正文，进入冲突（3.8）。
  - 503 `server_busy`：按 `Retry-After` 自动再存，至多三次，状态写"服务器繁忙，稍后自动重试"（P4 留给 P6）。
  - 400 `bad_request`：说明网络太慢、正文没有及时传完（P4 留给 P6 的带宽前提）。
  - 422、403、404：`errorText`，文字留在编辑器里。
- **结束**：退出编辑、离开页面（组件卸载）时 `endEditSession`，不等答复；404 当作已结束。关掉标签页的会话一个租约之内过期（M4 总设计第 4 节）。
- 常量 `editSessionLease`、`editSessionHeartbeat` 在 `stores/edit-session-timing.ts`，注释指向服务端的两个常量，测试钉住数值（13.1 第 15 条）。

### 3.7 进入与退出

- 写者（`writesPages(notebook.role)`）的页面标题旁有"编辑"按钮；阅读者没有，`Mod+E` 也不做什么（PG12）。
- 页面外壳在 `document` 上接 `Mod+E`、`Mod+S`（`isMod`），有对话框开着时不接（与快速切换相同）：
  - 阅读时 `Mod+E` 进入编辑，加载分包之后焦点进编辑器。`Mod+S` 在阅读时不拦（浏览器的"保存网页"照常）。
  - 编辑时 `Mod+S` 保存，`Mod+E` 与"完成"按钮：有未保存的修改先保存，成功之后结束会话、等阅读视图重读（失败也照常回去，由它自己显示未加载），回到阅读视图，焦点给"编辑"按钮；保存失败时留在编辑。
  - 输入法组合中（`composing()`）按 `Mod+S` 或 `Mod+E`：阻止浏览器的默认行为，记下要做的事，组合结束（`compositionend` 之后的第一次更新）再做，存进去的是确认后的文字，不是半截拼音（M0/P1 移交第 1 项）。
- 编辑时的状态栏（`page-editing-bar.tsx`）：`<output>` 里是"有未保存的修改"、"保存中…"、"已保存"或失败的原因；"保存"与"完成"按钮。编辑器替换阅读视图，面包屑、标题与子页面列表照旧。
- **未保存提醒**（`unsaved-guard.tsx`，有未保存的修改时才挂）：路由内的离开（树、面包屑、快速切换、后退）由 `useBlocker` 拦下，确认框"离开而不保存？"：留下、离开（离开时会话随组件卸载结束）；只换了查询或锚点不拦。关标签页、刷新由 `beforeunload` 提醒。这一页被删掉（本标签页或树重读发现）时外壳去父页，编辑器随之卸载，不拦（3.12）。

### 3.8 冲突

- 409 `page.revision_mismatch`：编辑器保留自己的文字，`getPageContent` 读出当前正文与 `revision`，编辑器上方出现冲突区（`role="region"`，有标题"这一页在你编辑时被改过"）：`@codemirror/merge` 的 `unifiedMergeView` 在一个只读的视图里显示当前正文到我的文字的差异（按 CodeMirror 的文字比较：只差在换行写法的不显示）。
- "保留我的"：以冲突区所依据的那次读取的 `revision` 为 `base_revision` 再存；其间又有别人写，就再次 409、冲突区换成新的差异（M4 总设计第 4 节"保存冲突"）。
- "放弃我的"：编辑器载入当前正文（新的 `EditorState`，换行记录按它重新拆），基准版本换成它的 `revision`，没有未保存的修改。
- 冲突区开着时，`Mod+S`、"保存"、`Mod+E`、"完成"都不发请求，只把焦点移到冲突区：先在两个按钮里选一个。冲突区出现时焦点移到它的标题。
- `@codemirror/merge` 在分包里再按需加载；加载失败时冲突区显示"重试"，两个按钮仍可用（不看差异也能选）。

### 3.9 文案

新键放在 `editor.*` 与 `page.*`：编辑、完成、保存、有未保存的修改、保存中、已保存、服务器繁忙会自动重试、网络太慢、失去编辑权限、冲突区的标题与两个按钮、离开确认、页面正文的标签，查找替换面板的短语（`editor.phrase.*`）。中英两份同键（类型与占位符的测试）。

### 3.10 已知差异

- M4 没有锁：两个人（网页或 PAT）同时编辑同一页，后存的人看到冲突（PG8）；M5 的锁之后冲突只来自接口的写。
- 别的标签页或别人删掉了正在编辑的页：树重读之后外壳去父页，未保存的文字丢失（保存本来也会答 404）。M5 的推送与锁之后再看。
- 差异按 CodeMirror 的文字比较，只差在换行写法的地方不显示。
- 后台标签页的心跳被节流时会话会过期，下一次保存重开会话（变更集分成两个，PG10）；M5 有锁之后锁会被别人拿走，写进给 M5 的移交。

### 3.11 构建与安全

- 主包不含 CodeMirror：`vite.config.ts` 里一个小插件在 `generateBundle` 时检查入口 chunk 与它静态引用的 chunk，含 `@codemirror`、`@lezer` 的模块就让构建失败（`make check` 的前端构建因此守住）。反向对照：主包静态 `import` 一个 CodeMirror 的模块，构建失败。
- CodeMirror 与 merge 的样式经 `style-mod` 注入，CSP 的 `style-src 'unsafe-inline'` 允许；不新增 Worker、不放宽 CSP。PG7 的页面版本核对编辑时没有 CSP 违规（e2e 的页面监视自动核对）。
- 正文只经 `putPageContent` 发出；冲突区与编辑器都只显示文字，不把正文当 HTML 挂进页面。

### 3.12 e2e 的页面版本

- 页面对象在 `fixtures/wiki-pages.ts`：进入编辑、编辑器的输入（`page.keyboard`）、保存、完成、冲突区、离开确认。
- PG7：`Mod+E` 进入、输入、`Mod+S`、再输入、`Mod+S`、`Mod+E` 回到阅读视图看到新内容；落库：两次保存一个变更集、一个正文版本，`revision`、`content_hash`、`byte_size`；未保存时点树里别的页先确认，留下之后文字还在。
- PG8：编辑时 PAT 先写；`Mod+S` 显示差异；"保留我的"之后正文是我的；另一页"放弃我的"之后编辑器是 PAT 的正文、再存不冲突。
- PG9：CRLF、单独的 `\r`、混合换行、BOM、行尾空白、NFD 的页面，在编辑器里行尾打字、行首打字、`Mod+Home` 打字、回车、行首退格之后保存，经接口读回的字节与预期逐字节相同（预期由测试按同样的规则写出）。
- PG10：心跳（`page.clock` 快进 20 秒，看到心跳请求、库里的 `expires_at` 后移）；完成之后会话的行没了；把会话在库里推到过期之后 `Mod+S`，编辑器重开会话、保存成功、没有丢字。
- PG12：阅读者的页面没有"编辑"，`Mod+E` 不进入编辑。
- 组合输入（Chromium，CDP 的 `Input.imeSetComposition`）：组合中 `Mod+S` 不发请求，组合确认之后存进确认的文字；落在 CRLF 页面的行尾，其余字节不变。

### 3.13 人工验收与移交

- `docs/v0.1/M4-pages/manual/P6-ime-checklist.md`：在 Chromium、Safari、Firefox 上用真实的中文输入法：确认后的文字与光标；组合中按 `Mod+S`、`Mod+E` 不存进半截拼音、组合结束后再存；组合中点"完成"；在 CRLF 页面的行尾组合输入，保存后经接口核对字节。负责人在 P6 完成时执行，结果写进 P6 的审查记录（M4 总设计第 3 节）。
- 给 M5 的移交（M4 收尾时写进移交文件）：组合中不触发自动保存、组合结束后补存；组合进行中收到外部更新（推送）时不打断组合；后台标签页的心跳节流与锁；编辑器扩展管线的最后一跳测试（M5 的只读与自动保存是第一个注册者）。

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
- 给 M5 的输入法与会话的移交内容写进第 7 节，M4 收尾时写进移交文件。

## 7. 结果

（完成后填写）
