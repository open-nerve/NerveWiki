# M5/P6 任务项：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M5/P6 任务项 |
| 状态 | 完成 |
| 基线 | `5b991e6`（P5 合并与它的文档提交之后的 main）；本文与各 Step 计划提交之后开分支 `m5-p6` |
| 上级文档 | [M5 总设计](00-M5-design.md) 第 3 节（C10）、4.4、4.12、第 5、7、8、9 节；[M4/P3 给 M5 的移交](handoffs/M4-P3-markdown-extensions.md)；[M4/P3 给 M6 的移交](../M6-links/handoffs/M4-P3-markdown-extensions.md) 第 1–7 项（扩展的约束，同样适用）；[M4/P3 文档](../M4-pages/03-P3-markdown.md) |

---

## 1. 基线

调研（Opus，2026-10-04）：

- **goldmark 的任务项**：`internal/harden/parser.go:84` 注册 goldmark 的 `TaskCheckBoxParser`（优先级 0），`render.go:28` 注册它的渲染器（优先级 500）。解析器在列表项第一个子块的第一个行内处按 `^\[([\sxX])\]\s*` 匹配，节点 `TaskCheckBox{IsChecked}` 不带位置；渲染器写 `<input checked="" disabled="" type="checkbox"> `。goldmark 没有"去掉"的选项：扩展盖不掉 harden 里的解析器。
- **位置**：匹配时段的 padding 为 0，`seg.Start + m[2]` 就是方括号里那个字符在正文里的字节位置（探针核实：LF、CRLF、引用里的制表符、`-` 后的制表符、`[\t]`、`[X]`、有序与嵌套列表）。BOM 与 frontmatter 由 `blank` 原地换成等长的空白，位置不变；frontmatter 里不会有任务项。正文不做 Unicode 规范化，标记是一个 ASCII 字节，换掉它 UTF-8 仍然有效。
- **一个边角**：`- [ ]: /u` 渲染为任务项，勾上之后变成 `- [x]: /u`：链接引用定义，复选框没了，`[x]` 在整页被定义（段落转换器先于行内解析）。
- **扩展的接口**：`markdown.Extension{Name, Parser, Extract, Fetch, Renderer, Markup}`；`Document.Extracted(name)`；`CheckHTML` 已允许 `input` 的 `type`、`checked`、`disabled`，扩展的 `Markup.Elements` 并进去。用户写的 HTML 的白名单没有 `input`，`data-*` 被去掉：用户伪造不出带 `data-task` 的复选框。
- **钉住今天输出的测试**：`render_test.go`（task items）、`markdowntest/check_test.go`、`harden_test.go`（harden 与 GFM 在 `tasklist.txt` 与随机的 `- [ ] ` 上逐字相同）；`tools/md-fixtures/cases` 没有任务项。`markdownExtensions()` 今天是 nil。
- **写正文**：`PutPageContent` 判定、解析（`ContentParser.Parse`：`Allowed` 之后取预算再解析，事务之外），`writer.Run` 里 `WriteContent`（锁正文行，同样的正文不写，`Base` 不等答 409 `page.revision_mismatch`），守卫（`edit_lock.go`：有活着的会话、而写入不带它，答 409 `page.locked`，本人在别处也一样），观察者经 `Change.Parsed` 得到解析的结果。`Parsed` 对用例不透明，`app.Markdown` 只有 `Parse` 与 `Render`。
- **前端**：`ReadingContext = {workspace, notebook, page, revision, role, reload}`；增强 `(container, ctx) => undo`；`readingEnhancements = [codeHighlight(worker), scrollFocus]`。`ReadingView` 以 `innerHTML` 原样放入服务端的 HTML（没有客户端的清洗），`<article>` 一直在，HTML 或 revision 变了就重跑增强。`PageShell` 有 `refusal` 的提示；`EditLockNote` 读 `["edit-lock", id]`。
- **e2e**：PG5 断言了 `<input disabled="" type="checkbox"> open` 与页面上的复选框，要改。

## 2. 目标与范围

**目标**：阅读视图里能写的人勾选、取消任务项，写回正文，其余字节不变；有人编辑时、版本落后时、不是任务项时说明原因；阅读者不能勾。

**做**：

- `platform/markdown/tasks`：任务项扩展（解析器、节点、渲染器、`Extract`、`Markup`），替换 harden 里 goldmark 的；`markdownExtensions()` 注册它。
- 页面模块：`ToggleTask` 用例、`page.toggle_task` 动作、`POST /api/v0/pages/{page_id}/toggle-task`。
- 前端：`ReadingContext` 加 `toggleTask` 与 `report`；任务勾选的增强；阅读视图重读之后焦点回到同一个复选框。
- e2e：C10 的接口与页面版本；改 PG5。

**不做**：编辑器里的勾选（源码编辑器里直接改 `[ ]`）；任务项的统计、筛选（以后的 M）。

## 3. 设计

### 3.1 文件

```
server/internal/platform/markdown/
  tasks/tasks.go、tasks_test.go             任务项扩展（3.2）
  internal/harden/parser.go、render.go      去掉 goldmark 的任务项（3.2）
  render_test.go、markdowntest、harden_test 随之改
server/internal/bootstrap/registrants.go    markdownExtensions() 注册 tasks（3.2）
tools/md-fixtures                           样例的 tasks 字段（README 规则 11、check.mjs），样例 062–067
server/internal/modules/page/
  domain/actions.go、task.go、errors.go     page.toggle_task；Flip；NotATask、TaskWouldGo（3.3）
  app/toggle_task.go、content.go、ports.go  ToggleTask 用例；Decided；Markdown.Tasks（3.3）
  adapter/markdown/markdown.go              Tasks（3.3）
  adapter/http/handler.go、module.go        toggleTask 的处理器（3.4）
server/internal/modules/access/domain/rules.go  page.toggle_task 的规则
api/modules/page.yaml、api/openapi.yaml     契约（3.4）
web/apps/web/src/
  services/page.service.ts                  toggleTask（3.5）
  stores/page-tree.store.ts                 toggleTask、oneToggle（3.5）
  reading/enhancement.ts、task-toggle.ts    ReadingContext 的 toggleTask、report；任务勾选（3.5）
  pages/page/reading-view.tsx、page-layout.tsx  上下文、报错、焦点（3.5）
  test/page-server.ts                       假服务器的任务项与勾选
e2e/fixtures/pages.ts                       toggleTask（3.6）
e2e/stories/collab/c10-tasks.spec.ts        C10（新）
e2e/stories/page/pg5-reading-view.spec.ts   data-task
```

### 3.2 任务项扩展（`platform/markdown/tasks`）

- **解析器**：照 goldmark 的任务项解析器（同一个正则 `^\[([\sxX])\]\s*`、同样只在列表项第一个子块的第一个行内，那个块是段落或标题；优先级 0），节点是自己的 `Task{Offset, Checked}`：`Offset` 是方括号里那个字符在正文里的字节位置（`seg.Start + m[2]`）。不另查 padding（原设想是 padding 不为 0 时不当任务项）：goldmark 在行内解析之前已把 padding 以空格放在行前，正则只从行的第一个字节 `[` 起匹配，匹配到的行 padding 必为 0。
- **渲染器**：`<input checked="" disabled="" type="checkbox" data-task="<位置>"> `：服务端的 HTML 对谁都一样（阅读视图按页缓存），复选框都是 `disabled`，能写的人由前端的增强放开（3.5）。`Markup`：`input` 带 `type`、`checked`、`disabled`、`data-task`；`CheckHTML` 的核心白名单不再有 `input`，注册了任务项扩展才允许。
- **`Extract`**：按正文的次序（按位置排序）给出 `[]Task`：goldmark 把脚注定义挪到文档最后，树的次序不是正文的。
- **替换**：harden 去掉 goldmark 的任务项解析器与渲染器；`markdown.New(nil)` 从此不认任务项，应用的实例经 `markdownExtensions()` 认。`harden_test` 比较的是"harden 加 goldmark 的任务项"与 GFM（harden 自己的那部分不变）；`render_test` 的任务项用例改为"没有扩展时不是任务项"，`markdowntest` 拒绝没有扩展的复选框。
- **检查**：`TestTheAppsMarkdownRendersCheckedHTML`、`TestTheAppsMarkdownCostsAboutItsSize` 用注册了它的实例；`Pathological` 加两份（满是 `- [ ] a` 的、引用里满是 `- [x]` 的）。样例集加可选的 `tasks` 字段（README 规则 11：每一项的位置与状态，按正文的次序，没有时不写；没被引用的脚注定义里的不算），`check.mjs` 逐字节核对；样例 062–067（任务项、不是任务项的、链接引用定义、嵌套与引用、CRLF 与 BOM 与 frontmatter 之后、脚注与标题）。扩展的测试比较每份样例，并在 `tasklist.txt`、规范的样例与两万个随机输入上与 goldmark 的任务项逐字比较（位置排序之后）。
- **最后一跳**：经 `newApp` 读阅读视图，复选框带 `data-task` 且位置对；`markdownExtensions()` 交空时失败。

M4/P3 给 M6 的移交第 7 项说"用 `Extract` 取 `TaskCheckBox` 节点所在的段落"推出位置；那是经验上的对应，这里按 M5 总设计 4.12 换成自己的解析器，位置由构造保证。

### 3.3 勾选的用例

`ToggleTask{PageID, BaseRevision, Offset, Checked}`，动作 `page.toggle_task`（规则同写正文：编辑者与管理员），次序：

1. 判定：`FindNode` 不是页答 404；`Allowed(page.toggle_task)` 答 404、403。
2. 读正文与 revision（事务之外）；`BaseRevision` 不等答 409 `page.revision_mismatch`（总体设计 4.12：位置只在它所依据的版本里有意义，排在 422 之前）。
3. 取预算（503）、解析（`ContentParser.Decided`：调用方已判定过，不再判定），解析完即放回预算；`Offset` 不是一个任务项的位置答 422（`offset`，`out_of_range`），负数、越界也一样。
4. 已经是那个状态：正文不写，答这一页（revision 不变）；读到的页已不在 `BaseRevision` 上（判断之后有人写过）答 409 `page.revision_mismatch`，与在 base 上写入一样。这一步在守卫之前：有人持锁时也答 200，与写入相同的正文一样（总体设计 4.4）；那个字符原样留着（大写的 `X`、制表符）。
5. `domain.Flip(content, offset, checked)` 把那个字节换成 `x` 或空格，其余不变；再解析新的正文（`Decided`，预算）：新正文在 `Offset` 上没有那个状态的任务项（`- [ ]: /u` 勾上之后成了链接引用定义）答 422（`offset`，`not_allowed`："勾上或取消会让那里不再有任务项"），不写。
6. `writer.Run(ActionToggleTask)` 里 `WriteContent{Base: revision, Parsed: 新的}`：守卫照写正文（有活着的会话答 409 `page.locked`，带 `lock`，本人在别处也一样）；其间有人写过答 409 `page.revision_mismatch`。观察者照写正文发 `pages` 事件。
7. 答 200，`Page`（与写正文的答复相同）。日志"page task toggled"。

`app.Markdown` 加 `Tasks(Parsed) []Task`（`Task{Offset, Checked}` 在 `app` 里），适配器从 `*markdown.Document` 取 `tasks` 的结果；用例只看这个，不看 `Parsed` 的内部。`domain.NotATask()`、`TaskWouldGo()` 是函数，与 `NotAllowed` 相同。

### 3.4 接口

`POST /api/v0/pages/{page_id}/toggle-task`（`toggleTask`），请求体 `TaskToggle{base_revision, offset, checked}`（`additionalProperties: false`），200 答 `Page`；码 401、403、404 `page.not_found`、409 `page.revision_mismatch`、409 `page.locked`、422 `validation_failed`（`offset`：`out_of_range` 不是任务项，`not_allowed` 勾了会消失）、503 `server_busy`。契约的说明写明已经是那个状态时答 200 不写、其间有人写过答 409。处理器照 `releaseEditLock` 的形状；每个码在处理器测试里各答一次；权限矩阵加一行。种子给每个笔记本加一页任务项（`- [ ] a\n`），不动现有的空正文的页；无主笔记本那一行原来期望 0 字节（"M3 没有活动的来源"），改为期望无主笔记本那一页任务项的字节数。

### 3.5 前端

- `PageService.toggleTask(pageId, TaskToggle)`：契约的请求体（`base_revision`、`offset`、`checked`），与 `putPageContent` 取 `PageContentWrite` 相同。
- **按页的闸门**：页面树 store 的 `oneToggle(pageId, run)`。这一页有勾选在外时不跑、答 false，跨视图成立（HTML 换过、离开又回来）；在外超过一分钟（`toggleLimit`：请求没有超时，断了的连接不该一直挡着）的不再挡住；它结束时不清掉后来者的。
- `ReadingContext` 加：
  - `toggleTask?(offset, checked): Promise<void>`：只给能写的人。`writesPages(role)` 在 `ReadingView` 里判断：`reading/` 不得引用 `app/` 的规则（oxlint 的引用限制），增强只看上下文有没有它。经 `oneToggle`：开始时清掉提示，以视图的 revision 为 base 发出勾选，成功之后重读阅读视图（等它答复）；409 `page.revision_mismatch` 先重读再拒绝；其余失败直接拒绝；
  - `report(error): void`：交给页面，照"编辑"被拒那样说（`PageShell`，M5/P4 3.6）：`page.locked` 不出提示，重读这一页的锁，有人持锁、而焦点仍在任务项的复选框上或在 body 上时，写着是谁的锁的说明取得焦点；其余的进 `refusal` 的提示（`errorText`）。编辑结束时清掉提示：编辑中到达的拒绝已过时。
- **任务勾选的增强**（`reading/task-toggle.ts`，注册在最后）：上下文有 `toggleTask` 时，`input[data-task]` 去掉 `disabled`，各以它那一项的可见文字为 `aria-label`（`taskText`：所在段落、标题或列表项里它之后的文字，到下一个块为止：子列表、代码、引用、段落、标题、表格等；WCAG 2.5.3、4.1.2）。点击（含空格键）`preventDefault`：复选框的状态只听服务端的；要的是 `checked` **属性**的反面（视图那个版本里服务端的状态），不是点击已翻过来的 `checked` 值；拒绝交给 `report`。撤销时恢复 `disabled`，去掉名称与监听。阅读者的复选框照旧 `disabled`、没有名称。
- **焦点**：`ReadingView` 撤销增强之前记下焦点所在的复选框：位置、它那一项的文字与状态。撤销会再禁用复选框，HTML 的焦点修正随之拿走焦点：Chromium 在下一次渲染时，别的引擎可能立刻，所以先记。换了 HTML 之后，同一位置上的仍是同一项才聚焦它，不滚动（`preventScroll`）："同一项"是文字相同、状态相同（视图自己刚改了那一项时相反）。自己的勾选不改长度、不挪位置；别人的写入（409 之后的重读、事件流的刷新）可能挪，挪了焦点不跟。
- **最后一跳**：经组合根的 `readingEnhancements` 的页面测试，注册表交空时失败。

### 3.6 端到端

- **夹具**：`pages.ts` 加 `toggleTask(api, credential, pageId, body)`（原样的答复）。
- **C10（接口）**：编辑者勾选 CRLF、BOM、NFD 的正文里的任务项：只有那个字节变了；已经是那个状态答 200、revision 不变；阅读者 403；不是任务项的位置、负数、`- [ ]: /u` 答 422（`offset`）；版本落后答 409 `page.revision_mismatch`；A 持锁时 B 与 A 自己的勾选都答 409 `page.locked`（带 `lock`），A 结束之后能勾；勾选发 `pages` 事件不在这里测（P2 的接口版本有事件流的夹具，留给最后一跳的测试）。
- **C10（页面）**：编辑者按名称（"open"）点复选框：勾上，正文写回，焦点在同一个复选框上；空格键取消；阅读者的复选框禁用；A 编辑时 B 点：照"编辑"被拒那样，写着 A 正在编辑的说明取得焦点，没有提示，复选框不变（控制台的 409 声明）。
- **PG5**：复选框带 `data-task`，位置对；管理员的复选框放开。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 任务项扩展：解析、渲染、`Extract`、替换 harden 里的；注册；检查与阅读视图的最后一跳 | [P6-S1](plans/P6-S1-task-extension.md) |
| S2 | 勾选的用例、动作、契约、处理器、矩阵；守卫与观察者的最后一跳 | [P6-S2](plans/P6-S2-toggle-task.md) |
| S3 | 前端：服务、`ReadingContext`、任务勾选的增强、焦点 | [P6-S3](plans/P6-S3-web.md) |
| S4 | 端到端：C10 的两个版本，改 PG5 | [P6-S4](plans/P6-S4-e2e.md) |

## 5. 测试与验证

- **单元**（Go）：解析的位置（LF、CRLF、BOM、frontmatter 之后、引用、制表符、嵌套、有序、`[X]`、不是任务项的 `[é]`、`[  ]`、代码块里的）；渲染的 `data-task`；`Extract` 的次序；`Flip` 只换一个字节；harden 与 GFM 的比较照旧成立。
- **用例**（Go，表格测试）：码的次序；已经是那个状态不写；不是任务项、负数、越界、`- [ ]: /u` 答 422；CRLF、BOM、NFD 的其余字节不变；守卫（会话在别处，本人与别人）；观察者得到新的 `Parsed`。
- **整个程序**（`newApp`）：阅读视图带 `data-task`；勾选经守卫（409 `page.locked`）与观察者（`pages` 事件）；`markdownExtensions()` 交空时失败。
- **前端**：增强（能写的人放开、阅读者不放开、点击的请求、同时一个、撤销）；`ReadingView` 的焦点与报错；经组合根的最后一跳。
- **e2e**：3.6。
- **反向对照**：
  - 位置偏一个字节（单元、C10 失败）；
  - 不判断新正文（`- [ ]: /u` 的测试失败）；
  - 422 排在 409 之前（版本落后的测试失败）；
  - 勾选不经守卫（持锁的测试失败）；
  - 增强给阅读者放开（组件测试失败）；
  - 不 `preventDefault`（状态的组件测试失败）；
  - 焦点不回来（组件测试、C10 失败）；
  - 任务项扩展不注册（最后一跳失败）。

## 6. 完成标准

- `GOFLAGS=-p=3 make check`、`make gen-check`、`make e2e` 为绿；上面的反向对照都按预期失败。
- 合并之前经 Opus 审查，修复之后另由 Opus 核对。

## 7. 结果

- 分支 `m5-p6`：S1 `f899f9e`、`bfc1e1c`；S2 `310976b`；S3 `51b86bc`、`89b5e0f`；S4 `5d7a4b3`；审查修复 `2248ba9`、核对之后的修复 `ce020b4`；CI 的失败写成注释 `4e0b38d`；`7bd630f` 合并（`--no-ff`）。
- 门禁：每个 Step 与两轮修复的 `make check` 为绿（前端 1594 个测试）；`make gen-check`、`make e2e`（179 个）、`make image-smoke` 为绿；C10（页面）在 S4 与审查修复之后各压测 10 次为绿；新的组件测试连跑 10 次为绿；持续集成为绿（`ce020b4` 的 e2e 失败、main 上只改文档的 `b4384ae` 同样失败，看不到是哪个用例，本地复现不了；加上注释之后 `4e0b38d` 全绿，记为偶发）。
- 审查：[P6 审查](reviews/P6-task-items-review.md)。两位审查者，没有阻断合并的问题；服务端的位置与 goldmark 的任务项逐字节一致（6 万个随机输入）。Important 2：B-I1（焦点回到移了位的别的项，还会滚动）、B-I2（放开的复选框没有名称）；Minor 7（A 5、B 2），都修掉。修复的核对没有 Important，Minor 6 都修掉。
- 反向对照：S1 17；S2 15（一项最初通过：已经是那个状态也写入，测试只有 `[x]`，补了 `[X]` 与制表符的项）；S3 15；S4 的 e2e 5（一项通过：撤销之后才读焦点在 Chromium 里同样成立，它在下一次渲染时才执行焦点修正，说明随之改正，`89b5e0f`；那个次序由组件测试的 `focusFixup` 钉住）；审查修复 14；核对之后的修复 7。其余都没有通过。

**与计划的出入**（已同步进上文）：

1. 解析器不另查 padding（3.2）。
2. 样例集加 `tasks` 字段与规则 11；`CheckHTML` 的核心白名单去掉 `input`，由扩展的 `Markup` 声明（3.2）。
3. `ContentParser.Decided`；第一次解析完即放回预算，再解析新的正文（3.3）。
4. `NotATask()` 是函数；勾了会消失另用 `TaskWouldGo`（`not_allowed`）（3.3）。
5. 权限矩阵的种子与无主笔记本那一行（3.4）。
6. `toggleTask` 只给能写的人，由 `ReadingView` 判断（3.5）。
7. `PageService.toggleTask` 取契约的请求体（3.5）。
8. 勾选的拒绝照"编辑"被拒那样说：锁的重读与说明的焦点在 `PageShell`，其余进 `refusal`（3.5）。
9. 焦点在撤销增强之前记下（3.5）。
10. 已经是那个状态在守卫之前答（3.3）。
11. 增强要的是 `checked` 属性的反面（3.5）。
12. 审查与核对的修复：`Extract` 按正文的次序；已经是那个状态时版本的再查；焦点只还给同一项（文字与状态）、不滚动；复选框的名称；按页的闸门移到页面树 store，带一分钟的期限；编辑结束时清掉提示（3.2、3.3、3.5）。
13. CI 里 e2e 的每个失败写成运行的注释（`playwright.config.ts` 的 `github` 报告器）：任务的日志不公开。

**留给后面的**：

- **给 M5 收尾**：负责人执行整份输入法清单（M4/P6 的第 1–9 步与 M5 的第 10–12 步）；e2e 在 CI 上的两次偶发失败，收尾的 e2e 审查看一遍可能不稳的用例，之后的失败从注释读。
- **接受**，留给 v0.1 收官之后的统一打磨：勾了会消失（`- [ ]: /u`）的拒绝只说"有些值无效"；失去角色的编辑者在笔记本列表重读之前复选框仍放开（服务端答 403，与"编辑"按钮相同）；阅读者禁用的复选框没有名称（P6 之前就如此）；勾选开始时清掉提示，可能藏起之前"编辑"的拒绝（提示一直只说最近一次动作）。
