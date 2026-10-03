# M4/P6 源码编辑器：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m4-p6-source-editor`（`main...66d32fa`：4 个提交，S1 `2cc081d`、S2 `6c6b7bd`、S3 `20c7b25`、S4 `66d32fa`），对照 [06-P6-source-editor.md](../06-P6-source-editor.md) 第 1–6 节、四份 Step 计划、[M4 总设计](../00-M4-design.md)第 3、4、8 节、[P4 文档](../04-P4-content-sessions.md)第 3、7 节、[人工输入法清单](../manual/P6-ime-checklist.md)，以及作者的偏差说明（as-built 笔记，14 条） |
| 审查方式 | 独立审查者（Opus）在 `git archive 66d32fa` 的快照上以读代码为主；另拷一份快照装依赖，跑了 P6 相关的 vitest（11 个文件、111 个测试通过）与 `vite build`，并用几个探针测试复现了 C1、M1、m1、m3、m4、m9②。审查者调试时曾把一个文件误复制到快照目录之外的临时目录，随即删掉；仓库、别的进程与 Docker 容器都没动过。修复之后另由一位 Opus 核对修复（见"修复的核对"） |
| 日期 | 2026-10-03 |
| 结论 | 换行与 BOM 的记录（含撤销恢复）、保存的队列与冲突流程整体正确。合并之前修一个 Critical（按"完成"或 `Mod+E` 之后、编辑器关闭之前输入的文字被静默丢弃）与一个 Major（StrictMode 下编辑会话一开就被结束，开发构建里保存多半失败），其余 Minor 一并处理。共 Critical 1、Major 1、Minor 12、Nit 10、疑问 6 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C1 | Critical | **按"完成"或 `Mod+E` 之后、编辑器关闭之前输入的文字被静默丢弃**：`leave()` 先保存，成功之后不再看有没有新的修改，就结束会话、重读阅读视图、回到阅读。这段时间（503 的等待、慢网络下 5 MiB 的上传、阅读视图的重读）编辑器照常可以输入，新打的字没有发出，也没有提醒（`done()` 不是导航，`useBlocker` 拦不到） | 已修：离开一次只有一个（"离开中"的标记，连按"完成"、按住 `Mod+E` 都只离开一次），从头到尾编辑器只读（句柄的 `hold`，与扩展的 `setReadOnly` 分开记，两者任一为真即只读；只读时内容区 `tabindex="-1"`，焦点不掉）；保存失败时放开、焦点回到编辑器（有冲突时在冲突区的标题）；保存成功之后仍有未保存的修改（扩展用代码改的）就留在编辑；离开途中组件卸载了就到此为止。"完成""保存"按钮与 `Mod+S`、`Mod+E` 一样等输入法组合结束（m8）。组件测试：保存挂起时按"完成"，编辑器只读，失败之后可编辑、焦点在编辑器；再按一次"完成"、第一次失败之后打的字留在编辑器里、不再发出；离开中被代码改了就留下；离开途中去了别的页，不再读这一页的阅读视图。FIX-HOLD、FIX-RELEASE、FIX-FAIL-FOCUS、FIX2-LEAVING、FIX2-RECHECK、FIX2-UNMOUNTED、FIX2-TABINDEX 失败（第二轮见修复的核对 MJ-1） |
| M1 | Major | **StrictMode 下编辑会话一开就被结束**：开发构建里 effect 跑 mount→cleanup→mount，同一个 `PageEditing` 经历 `start()`、`end()`、`start()`，`ended` 不再复位：没有心跳，每次保存开一个会话又立刻结束、PUT 与 DELETE 竞速，多半 409；正文读两次，两次之间有别人写时编辑器的文字与基准版本来自不同的答复 | 已修：`start()` 复位 `ended`，正在开的会话照常留下；正文只读一次（在途的读复用，读到之后再开始不再读）。PageEdit 的组件测试改在 StrictMode 下跑（与 onboarding 的测试相同）；StrictMode 下第一个编辑器在 effect 重跑时被换掉，测试取编辑器之前先让 effect 跑完。store 测试"结束又开始"：只开一个会话、只读一次正文、心跳照常。FIX-START-ENDED（store 与 PageEdit 的测试都失败）、FIX-READ-DEDUPE、FIX-READ-ONCE 失败；冲突区的组件测试也改在 StrictMode 下跑（差异分包加载失败的那一个除外：vitest 里抛错的 mock 只失败一次，StrictMode 的第二次加载拿到真模块）。建议把会话的生命周期抽成一个类：不拆，会话与保存经"已结束"与重开互相依赖，拆开要多一层接口；M5 的锁以会话为载体时再看 |
| m1 | Minor | **结束之后仍会保存、开会话**：503 等待中点了确认框的"离开"、有排队的保存时离开，保存照样发出（开会话、立刻结束、用它 PUT、409、再来一遍），PUT 抢在 DELETE 之前就写进库；503 的计时器与 compositionend 之后的计时器都不清理 | 已修：结束之后什么也不再发：`sessionId()` 拒绝，结束之后才开成的会话结束掉、不交给保存；`end()` 唤醒 503 的等待，等待的、排队的、在途答复之后的保存都以失败结束，不重试、不读冲突；`EditorHost` 销毁时清掉 compositionend 的计时器；结束之后答复的保存不再起心跳，`start()` 先清掉旧的心跳；503 的等待因结束而醒的就以失败结束，即使随后又开始（修复的核对 mn-1、nt-4）。store 测试"结束之后什么也不发"（排队的、新的、503 等待中的、会话开启中的、在途答复 503 的、结束又开始的）、"结束之后没有计时器"与编辑器测试"等组合的动作随编辑器销毁"。FIX-SESSION-ENDED、FIX-OPENED-AFTER-END、FIX-ENDED-STOPS、FIX-WAKE、FIX-SETTLING-CLEARED、FIX2-ENDED-BEATS、FIX2-START-CLEARS、FIX2-WOKEN-ENDED 失败 |
| m2 | Minor | 保存在途时心跳 404 重开了会话 Y，保存答 409 `page.edit_session_ended` 时把 Y 丢掉（不心跳、不结束）又开一个 | 已修：只在 `this.session` 仍是这次保存用的会话时清掉，与心跳的做法一致；重发用 Y。store 测试；FIX-KEEP-NEW-SESSION 失败 |
| m3 | Minor | 冲突差异的折叠行没有本地化（`"$ unchanged lines"` 在短语表里，差异视图却没装短语） | 已修：差异视图装上 `editorPhrases(t)`。组件测试用中文核对折叠行；FIX-DIFF-PHRASES 失败 |
| m4 | Minor | 离开确认框开着时保存落地，确认框随 `UnsavedGuard` 卸载而消失，用户点的链接没有生效 | 已修：`UnsavedGuard` 常驻，按 `unsaved` 决定拦不拦；拦下之后保存落地（不再有未保存的修改）就放行这次导航；`beforeunload` 也只在未保存时挂。组件测试：保存挂起、点树里别的页、确认框出现、保存答复，去了那一页。FIX-GUARD-PROCEED、FIX-GUARD-UNSAVED、FIX-BEFOREUNLOAD-UNSAVED 失败 |
| m5 | Minor | 没有告诉用户怎样用键盘离开编辑器（Tab 被缩进占用，要先按 Esc 再按 Tab），WCAG 2.1.2 | 已修：编辑器下方一行说明"Tab 键缩进。要离开编辑区，先按 Esc，再按 Tab。"，内容区的 `aria-describedby` 指向它。组件测试；FIX-HINT 失败 |
| m6 | Minor | 焦点：① 点"编辑"之后按钮卸载，焦点落到 body，读正文失败时停在那里；② "保留我的"失败（不是新的冲突）时冲突区连同获焦的按钮卸载；③ 确认框点"留下"之后焦点回到点过的树链接，不回编辑器 | 已修：① 进入编辑时焦点先给页面标题，编辑器打开时它接过；② "保留我的"答复之后没有冲突就把焦点给编辑器（失败原因在状态栏）；③ "留下"把焦点还给编辑器（`onCloseAutoFocus`），冲突区开着时给它的标题（修复的核对 nt-5）。四个组件测试；FIX-HEADING-FOCUS、FIX-KEEP-FOCUS、FIX-STAY-FOCUS、FIX2-STAY-CONFLICT 失败 |
| m7 | Minor | 差异视图的折叠行只响应点击，键盘展开不了 | 已修：冲突区在差异下方加"显示未改动的行"（`aria-pressed`），按下之后差异不再折叠。按钮在冲突区（主包）而不在差异的分包里：放进差异的分包会让主包再拆出一个公共分包。组件测试；FIX-UNFOLD 失败 |
| m8 | Minor | "保存""完成"按钮与扩展的 `controls.save()` 不等输入法组合结束（注释说"与 Mod+S 相同"，实际不同） | 已修：两个按钮与"保留我的"经 `whenComposed`；`controls.save()` 在编辑器里包一层 `whenComposed`，M5 的自动保存照注释调用即可。组件测试（组合中按"保存""完成"不发请求，组合结束之后存进确认的文字）与编辑器测试；FIX-BUTTONS-COMPOSED、FIX-DONE-COMPOSED、FIX-SAVE-COMPOSED 失败 |
| m9 | Minor | 扩展管线：① `composeExtensions` 返回的 `reconfigure` 运行时谁也拿不到；② 扩展构造时调 `setReadOnly` 抛 TypeError（`this.view` 还没赋值），整个扩展被丢掉；③ "放弃我的"载入新正文时只读被重置为可编辑 | ②③ 已修：`EditorView` 先建、再设 state，只读的状态记在编辑器里，新的 state 按它建；扩展构造时设的只读直接生效。编辑器测试；FIX-VIEW-FIRST、FIX-LOCK-KEPT、FIX-HOLD-APART 失败。① 不改：M4 没有调用者，组合测试里能按名字换掉、卸下（M0/P1 移交第 2 项）；M5 需要从外面换掉扩展时再接到句柄上，写进给 M5 的移交，3.4 节照此改写 |
| m10 | Minor | 构建检查只守入口 chunk：路由的 chunk 本身是动态加载的，`page-edit.tsx` 静态 import 一个编辑器模块，每次看页面都下载 CodeMirror，构建照样绿；正则也漏了 `style-mod`、`w3c-keyname`、`crelt` | 已修：从入口出发，沿静态与动态 import 走到的 chunk（路由的也在内），到编辑器自己的两个入口（`editor/source-editor`、`editor/conflict-view`）为止，都不许含编辑器的模块；报错写出从入口到它的路径。正则补上三个包。编辑器自己再按需加载的分包不算（修复的核对 mn-2：第一轮把所有动态入口当起点，会误报）。FIX-BUILD-ROUTE（`page-edit.tsx` 静态 import `editor/line-breaks`）构建失败，报 `index → page-layout → codemirror`；FIX2-EDITOR-ENTRIES-SKIPPED（不在编辑器入口停下）构建失败；编辑器里 `import()` 一个只用 CodeMirror 的模块，构建通过（手工核对） |
| m11 | Minor | 正文超过 5 MiB 或含 NUL 的 422 只显示"有些值无效" | 已修：`content` 字段的 422 用 `field.content.too_long`、`field.content.invalid_format` 的文案（"正文超过 5 MiB，删减之后才能保存。"等）。组件测试；FIX-422 失败 |
| m12 | Minor | 测试缺口：PageEdit 不在 StrictMode 下测；C1、m1、"结束又开始"没有覆盖；状态栏的 busy、tooSlow、lostAccess 只在 store 层测了状态位；"扩展在编辑器自己的之后"没有核对顺序 | 已补：StrictMode（见 M1）；C1、m1 的测试见上；状态栏的五种文字（503、400、两种 422、心跳 403）在组件层核对；顺序用两边都绑的 `Mod+B` 核对（编辑器的先跑）。FIX-STATUS-TOO-SLOW、FIX-ORDER 失败 |
| n1 | Nit | `usePageTree` 的注释挂到了 `useNewPageEditing` 上 | 已改 |
| n2 | Nit | 按住 `Mod+E` 在阅读与编辑之间来回切换 | 已改：阅读时重复的按键只拦下浏览器的行为、不进入编辑；编辑中重复的 `Mod+E` 由"离开中"的标记挡住，只离开一次（第一轮的处置说"离开的保存挡住了后面的"，不属实，见修复的核对）。组件测试核对只保存一次、只读一次阅读视图；FIX-REPEAT、FIX2-LEAVING 失败 |
| n3 | Nit | `toggleStrong`、`insertLink` 不看 `state.readOnly` | 已改：只读时返回 false。命令测试；FIX-STRONG-READONLY、FIX-LINK-READONLY 失败 |
| n4 | Nit | 内容区的 `aria-label` 只在建 state 时取一次，切换界面语言之后不变 | 已改：标签与短语放在同一个随语言重配的 compartment 里。编辑器测试；FIX-WORDING-LABEL 失败 |
| n5 | Nit | `lostAccess` 为真之后不再复位，之后保存成功状态栏仍写"不能再编辑" | 已改：保存成功时复位，心跳重新开始。store 测试；FIX-ACCESS-BACK、FIX-BEATS-BACK 失败 |
| n6 | Nit | `markdownLanguage` 是 GFM 加下标、上标、Emoji，不是设计写的 CommonMark + GFM；`~x~` 在编辑器里高亮成下标 | 不改，改文档：只影响高亮；换成 `commonmarkLanguage` 加 GFM 要另带 `@lezer/markdown` 并照抄 `lang-markdown` 的语言包装。3.2 节照实写 |
| n7 | Nit | 链接与查找匹配的颜色写死成 oklch；主题没声明暗色，merge 的暗色规则不生效（折叠行仍是浅色底） | 部分采纳：折叠行的颜色取应用的变量（`--accent`、`--muted-foreground`），明暗都对。链接与查找匹配仍用编辑器自己的变量：应用的 `--primary` 是中性灰，没有链接色与高亮色；两者在 `.dark` 下另有一套（查找匹配的暗色是修复的核对补的：第一轮只有链接有，暗色下对比度约 3.3:1），与阅读视图的代码高亮相同做法。3.2 节照实写。只是样式，没有自动测试 |
| n8 | Nit | 设计 3.5 的句柄是 `text/focus/composing/load`，实现是 `text/version/focus/whenComposed/load`（现在还有 `hold`） | 已改文档：3.5 节同步 |
| n9 | Nit | 人工清单第 7 步：macOS 上 `Ctrl+Home`、`End` 不是跳到文档开头与行尾 | 已改：写明 macOS 上用 `Cmd+↑` 再 `Cmd+→` |
| n10 | Nit | `beforeunload` 只调 `preventDefault()`，Chromium 119 之前还要 `returnValue = ""` | 不改：目标是各浏览器的当前正式版（构建目标 ES2023），不用已废弃的 `returnValue` |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 冲突区的"我的"是被拒那次保存的文字，"保留我的"存的却是点击时编辑器里的文字，冲突之后继续修改时两者对不上 | 有意：冲突之后的修改不能丢（可以先看差异、在编辑器里手工合并，再"保留我的"）。冲突区的说明改为"保留你的，就用编辑器里的文字（连同之后的修改）覆盖这一页"。差异不随编辑刷新：大文档每次按键重算差异太贵 |
| Q2 | 页面被别人删除或失去访问后，树一重读编辑器就卸载，未保存的文字不提醒就丢了 | 维持 3.10 的已知差异：要在页面不在树里时留住编辑器，页面外壳的结构要改；M4 没有推送，只有树重读（切回标签页等）时才会发生。M5 的推送与锁之后再看，写进给 M5 的移交。负责人可以改判 |
| Q3 | 心跳 403 或页面 404 时，"完成"每次先存都失败，离开不了 | 不改：去别的页时确认框的"离开"就是放弃修改的出口；状态栏说明了原因（403 时提示先复制文字） |
| Q4 | 浏览器始终不发 compositionend 时，排队的 `Mod+S`、`Mod+E` 永远不执行 | 不加上限：上限一到只能在组合中保存（存进半截拼音）或丢掉这个动作，都比等待更糟；状态栏仍是"有未保存的修改"。人工清单在三种浏览器上核对组合的结束；M5 的自动保存再看是否在状态栏提示"等输入法确认" |
| Q5 | 差异分包加载失败后"重试"：部分浏览器缓存失败的模块请求，不刷新永远重试不成 | 不改：重试不成时两个按钮照常可用，刷新页面之后也能看差异；给地址加查询串要绕开 Vite 对 `import()` 的分包解析。人工清单没有覆盖，记在这里 |
| Q6 | 保存之后不标阅读视图过期（as-built 3）：从编辑直接去别的页再回来，先看到编辑前的阅读视图 | 采纳（回到设计 3.6 的做法）：保存成功之后清掉缓存里的阅读视图，回来时重新读；清除时不带 `revalidate: false`，否则 SWR 的去重会把两秒内旧的那次读的结果交回来。离开编辑时照旧把重读的阅读视图放进缓存。组件测试；FIX-CACHE、FIX-CACHE-DEDUPE 失败 |

## 修复的核对

第一轮修复（`aa02f26`）之后，另一位 Opus 在快照上核对修复。它跑了 web 的全部测试（87 个文件、1344 个测试）、tsc、oxlint、oxfmt 与 `vite build`，把 38 个反向对照逐个改回去跑（都按声称的那样失败），用探针测试复现了下面的问题。P6 的测试连跑 8 轮、再并发跑 4 轮，都没有不稳定。结论：没有 Critical；1 个 Major（C1 的修法在两次离开交错、第一次保存失败时仍会丢字），2 个 Minor，7 个 Nit，另有 5 条处置不属实。第二轮修复（`96fc5eb`）处置如下。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| MJ-1 | Major | **两次离开交错时，第一次保存失败之后打的字被丢掉**：双击"完成"、按住 `Mod+E`、`Mod+E` 之后紧接着点"完成"，第二次离开在第一次保存在途时进入、把保存排进队列；第一次失败放开只读、焦点回编辑器，用户接着打字，排队的那次以旧文字保存成功，第二次离开照样结束。按住 `Mod+E` 时每次重复都完整跑一遍离开 | 已修：见 C1 一行（"离开中"的标记；保存成功之后再看有没有未保存的修改）。FIX2-LEAVING、FIX2-RECHECK 失败 |
| mn-1 | Minor | 失去权限之后，在途的保存在 `end()` 之后才成功，再起一个永远不清的心跳；`start()` 不先清掉旧的心跳 | 已修：见 m1 一行。FIX2-ENDED-BEATS、FIX2-START-CLEARS 失败 |
| mn-2 | Minor | 构建检查把只由编辑器加载的动态 chunk 当成"编辑器之前加载"，误报；报错指不出是谁把 CodeMirror 拉进来的 | 已修：见 m10 一行 |
| nt-1 | Nit | 离开的保存在途时在确认框点"离开"，组件已卸载，答复之后仍读一次阅读视图、调 `done()` | 已修：卸载了就到此为止（`useMounted`）。组件测试；FIX2-UNMOUNTED 失败 |
| nt-2 | Nit | `hold` 让内容区不可聚焦，真实浏览器里焦点可能在保存期间掉到 body | 已修：只读时内容区 `tabindex="-1"`，不进 Tab 顺序，焦点留得住。jsdom 不做焦点修正，测试只核对属性；FIX2-TABINDEX 失败 |
| nt-3 | Nit | 组合进行中编辑器被销毁时，扩展拿到的 `controls.save()` 永远不 settle | 不改：编辑器销毁时扩展随之销毁，没有人再等它。M5 的自动保存若要等这个 promise，写进给 M5 的移交 |
| nt-4 | Nit | 503 等待中先结束再同步开始（开发时模块热替换重跑 effect），醒来的保存不等 `Retry-After` 就开新会话重发 | 已修：见 m1 一行。FIX2-WOKEN-ENDED 失败 |
| nt-5 | Nit | 冲突区开着时点"留下"，焦点给了编辑器，不是冲突区的标题 | 已修：见 m6 一行 |
| nt-6 | Nit | 没有任何折叠时"显示未改动的行"也在，按了没有变化；每次切换都重建差异视图，滚动位置丢失 | 不改：按了无害；要知道有没有折叠，得让差异视图把它报给冲突区；切换不常用 |
| nt-7 | Nit | 冲突区的测试不在 StrictMode 下跑；"保留我的"失败只断言状态栏不为空 | 已改：冲突区的测试在 StrictMode 下跑（加载失败的那一个除外，见 M1 一行）；"保留我的"失败核对失败的文案 |

处置不属实的 5 条：
- C1 与 n2：两次离开交错时，"离开从头到尾只读"和"离开的保存挡住了后面的"都不成立。已按 MJ-1 修正，处置据此改写。
- m10：第一轮实际报的是 codemirror 分包，不是"报出路由的 chunk"。第二轮的报错写出从入口开始的整条路径。
- n7："在 `.dark` 下另有一套"第一轮只对链接色成立，第二轮补上了查找匹配的暗色。
- n8 及 3.2、3.4、3.6、3.7 的文档同步，随合并之后 main 上的文档提交落地。

第二轮修复之后，持续集成上"按住 `Ctrl+E` 只离开一次"的测试按时序失败了一次：它数阅读视图的读取，而阅读视图重新挂上之后，SWR 隔一帧还会再验证一次，慢的机器上这次读取落在断言之前。`500bb60` 改为数保存的次数；离开的防重另由"再按一次'完成'"的测试钉住，FIX2-LEAVING、FIX-REPEAT 仍然失败。

## 人工验收

[输入法清单](../manual/P6-ime-checklist.md)由负责人在 Chromium、Safari、Firefox 上执行，结果写在这里。

待负责人执行。
