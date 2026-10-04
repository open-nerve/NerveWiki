# M5/P6 任务项：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m5-p6`（`b4384ae..5d7a4b3`：6 个提交，S1 `f899f9e` `bfc1e1c`、S2 `310976b`、S3 `51b86bc` `89b5e0f`、S4 `5d7a4b3`；63 个文件，+2225/−81），对照 [06-P6-task-items.md](../06-P6-task-items.md) 第 1–6 节、四份 Step 计划、[M5 总设计](../00-M5-design.md)第 4.4、4.12 节与第 3 节 C10、[M4/P3 给 M6 的移交](../../M6-links/handoffs/M4-P3-markdown-extensions.md)第 1–7 项、样例集的 README，以及实现者列出的 11 处有意的出入 |
| 审查方式 | 两位独立审查者（Opus）在仓库上只读：A 看服务端（任务项扩展、harden 的替换、样例集、勾选的用例、契约、处理器、矩阵、最后一跳），B 看前端与端到端（增强、`ReadingView`、`PageShell`、服务与 store、假服务器、组件测试、C10、PG5）。两位都跑了测试与类型检查，并在临时目录写了探针（A 经 `go test -overlay` 跑了 6 万个随机输入，逐个勾一遍再解析）。修复之后另由一位 Opus 核对（见"修复的核对"） |
| 日期 | 2026-10-04 |
| 结论 | 没有阻断合并的问题。服务端：位置（BOM、frontmatter、CRLF、制表符、引用、嵌套、脚注、setext、懒惰行）与 goldmark 的任务项逐字节一致，勾选只改一个 ASCII 字节，码的次序、预算、守卫与观察者都成立。合并之前修 B 的两个 Important（焦点回到移了位的别的项、放开的复选框没有名称），其余 Minor 修掉。A：Minor 5；B：Important 2、Minor 2 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | **焦点回到同一位置上的别的项，还会滚动**：视图换 HTML 时按 `data-task` 还焦点；自己的勾选不改长度，但 409 `revision_mismatch` 的重读与事件流的刷新里位置已经移了。探针：视图在版本 1（a 在 3、b 在 11），Bob 在前面加了一项（z 3、a 11、b 19），Ada 聚焦 b 按空格，409、重读，焦点落到 a，再按空格勾的是 a。`focus()` 不带 `preventScroll`：A 编辑时每次自动保存的刷新都把 B 的页面滚回那个复选框 | 已修：记下焦点所在复选框的位置与它那一项的文字（`taskText`），只有新 HTML 同一位置上的仍是同一项（文字相同）才还焦点，且不滚动。组件测试"版本落后的勾选：焦点不到移到那个位置上的项"、"勾选之后焦点在原处，不滚动"；focus-any-item、focus-scrolls 失败 |
| B-I2 | Important | **放开的复选框没有名称**：成了 Tab 停靠点，读屏只读"复选框，未选中"（WCAG 4.1.2；v0.1 设计第 13 节前端规则 17） | 已修：增强给每个放开的复选框 `aria-label`，取它那一项的可见文字（所在段落、标题或列表项里它之后的文字，子列表不算），撤销时去掉。单元测试（紧凑、松散、标题、嵌套、空的项）；no-name、name-kept、name-with-sublist 失败 |
| B-m3 | Minor | **"同一页同时一个"只在一次 HTML 里成立**：`out` 在增强的实例里，HTML 每换一次就是新的；探针：a 的勾选在外时别人的写入刷新了视图，b 的勾选照样发出，b 成功之后 a 的 409 才到，"这一页在你读之后改过"出现在刚成功的点击之后。离开再回来也一样（违反 P6 3.5、M5 4.12 与第 13 节前端规则 1） | 已修：闸门移到页面树 store（`oneToggle`，按页，含之后的重读）；`ReadingView` 的 `toggleTask` 经它走，增强不再自己计数。store 测试与页面测试"离开又回来时一个勾选在外，点击不发"；gate-open、gate-unused 失败 |
| B-m4 | Minor | **锁的拒绝是笼统的提示，锁结束了还在**："这一页正在另一个会话里编辑"读起来像 B 自己的另一个会话；A 结束编辑之后提示还在 | 已修：勾选的 `page.locked` 照"编辑"被拒那样处理（M5/P4 3.6）：不出提示，重读锁，有人持锁时锁的说明（写着是谁）取得焦点；C10 随之改。组件测试；locked-alert、locked-no-focus 失败 |
| B 的小点 | — | 勾 `- [ ]: /u` 被拒时只说"有些值无效"；失去角色的编辑者在笔记本列表重读之前复选框仍放开（服务端答 403，与"编辑"按钮相同） | 不改：文案留给 v0.1 收官之后的统一打磨；角色的问题不是新的 |
| A-m1 | Minor | **`Extract` 的次序是阅读视图的，不是正文的**：goldmark 把脚注定义挪到最后，`a[^1]\n\n[^1]: - [ ] x\n\n- [ ] y\n` 取出 33 在 16 之前；样例按正文次序写会让 Go 的测试失败，倒过来写又过不了 `check.mjs` | 已修：`Extract` 按位置排序（设计说"按文档的次序"）；规则 11 写明次序以正文为准；新样例 067（脚注里的、没被引用的脚注里的、标题里的）；与 goldmark 的比较按位置比；extract-unsorted 失败 |
| A-m2 | Minor | **规则 11 漏了标题**：列表项的第一个块是 ATX 或 setext 标题时，标题开头的方括号也是任务项（与 goldmark 相同） | 已改：规则 11 写"段落或标题"，并写明没被引用的脚注定义里的不算；样例 067 |
| A-m3 | Minor | **勾了会消失的项的 422 说错了**：`- [ ]: /u` 在 3 上**是**任务项，回答却说"不是任务项的位置" | 已修：第二次解析的拒绝另用 `TaskWouldGo`（`not_allowed`，"勾上或取消会让那里不再有任务项"），不是任务项的位置仍是 `out_of_range`；契约的说明写明两种。单元测试；would-go-out-of-range 失败 |
| A-m4 | Minor | **"已经是那个状态"可能答出较新的版本**：在版本 5 判断，`readPage` 之前有人写了 6（那一项又不是勾上的了），答 200 带着 6 | 已修：已经是那个状态时，读到的页面不在 base 上就答 409 `page.revision_mismatch`，与在 base 上写入一样。单元测试（判断之后有人写入）；unchanged-no-recheck 失败 |
| A-m5 | Minor（测试） | 码的次序的测试只有最后一条带拒绝的守卫，钉不住"版本与新正文在守卫之前" | 已补：两条带拒绝守卫的（版本落后、勾了会消失），分别期望 409 `revision_mismatch` 与 422 |

## 对出入的判断

A 认为服务端的出入成立：1（不查 padding）：goldmark 在行内解析器之前已吃掉 padding，`Segment.Value` 把 padding 以空格放在行前，匹配从 `[` 起就说明 padding 为 0；2–5、7 成立，矩阵核过；10（已经是那个状态在守卫之前答）与 M5 总设计 4.4"与当前正文相同的写……锁不必拒"一致，A-m4 之外。B 认为 6、8、9、11 成立：`checked` 属性是视图那个版本里服务端的状态；先记焦点再撤销在 Chromium 里不必要、在立即执行焦点修正的引擎里必要，组件测试模拟后者。

## 修复的核对

修复（`2248ba9`）之后另由一位 Opus 只读核对：跑了服务端与前端的测试和类型检查，用临时目录里的 Go 探针（`go test -overlay`）渲染了 13 份输入。结论：九处修复都对（焦点的记录与恢复在同一次提交里，`PageShell` 按页挂载，拿不走别处的焦点；名称在紧凑、松散、嵌套、标题、行内标记、链接、脚注引用、图片里都对，`aria-label` 只在客户端；闸门在 `try/finally` 之外不会抛出，卸下的视图的 `mutate` 立即返回；样例 067 的位置逐字节对；`unchanged()` 的两次读读的是同一行），没有 Important。Minor 6，处置如下。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-m1 | Minor | **同样文字的另一项仍可能拿到焦点**：`- [ ] a\n- [x] a\n`，焦点在勾上的那项（11）；有人在前面加了 `- [ ] a`，刷新之后 11 上是原来的第一项（没勾、文字也是 a），焦点过去，空格勾错了项。文字为空的项同理 | 已修：还要状态相同，视图自己刚改了那一项时除外（`toggled`）。组件测试"焦点不到写入挪到它位置上的同样文字的另一项"；state-ignored、flip-ignored 失败 |
| C-m2 | Minor | **紧凑列表项的名称带上了它后面的块**：`- [ ] a` 之后的代码块渲染在同一个 `<li>` 里，名称成了"a 代码……"；引用、HTML 块同理 | 已修：名称到下一个块（子列表、代码、引用、段落、标题、表格等）为止。单元测试；name-past-blocks 失败 |
| C-m3 | Minor | 锁的拒绝在重读锁之后把焦点拿到说明上，即使用户其间去了别处（搜索框、对话框） | 已修：只在焦点仍在任务项的复选框上或在 body 上时才拿。组件测试（重读锁时按住答复，其间焦点到 Edit）；focus-anywhere 失败 |
| C-m4 | Minor | **闸门可能一直关着**：请求没有超时，连接断了没有答复的勾选让这一页之后的点击都不做事，直到重新登录（之前重新挂载会重置） | 已修：在外超过一分钟（`toggleLimit`）的勾选不再挡住；它结束时不清掉后来者的。store 测试（假时钟）；gate-forever、end-clears-next 失败 |
| C-m5 | Minor（文字） | `ReadingContext` 的说明还说 `toggleTask` 重读锁、`report` 显示在视图上方；样例的 note 还说第一个块是段落；P6 文档第 3 节有四处与代码不符 | 已改：说明与样例的 note；P6 文档在第 3 节同步时改 |
| C-m6 | Minor（修复之前就有） | 编辑时到达的勾选拒绝存进 `refusal`，编辑结束时显示出来，已过时 | 已修：编辑结束时清掉。组件测试；refusal-after-edit 失败 |
| 说明 | — | 勾选开始时清掉提示可能藏起之前"编辑"的拒绝：提示一直只说最近一次动作，接受；阅读者的禁用复选框没有名称：P6 之前就如此，不能操作；被挪走的项焦点落到 body：设计如此 | 不改 |

## 持续集成

- 核对之后的修复（`ce020b4`）：web、server、image 绿，e2e 失败。main 上只改了文档的 `b4384ae`（代码与通过的 `5b991e6` 相同）的 e2e 同样失败，两次都跑完了全程；本地以默认并发与 CI 的 2 个 worker 各跑全套都通过。任务的日志不公开，注释只有退出码，看不出是哪个用例。
- `4e0b38d`：CI 里加上 Playwright 的 `github` 报告器，每个失败写成运行的注释。推上之后四个任务都绿（e2e 179 个），合并为 `7bd630f`。前两次记为偶发，原因未明；之后的失败能从注释读到，M5 收尾的 e2e 审查看一遍可能不稳的用例。

## 反向对照

- S1：offset-plus-one、offset-bracket、no-data-task、no-data-task-last-hop、no-extensions、harden-keeps-goldmark、lowercase-only、any-block、after-text、not-in-list、advance-short、extract-none、fixture-offset-go、fixture-offset-mjs、fixture-checked-mjs、fixture-missing-tasks-go、check-allows-input。
- S2：422-before-409、writes-same-state（最初通过：测试里已经是那个状态的项只有 `[x]`，换成写入照样答 200；补了 `[X]` 与 `[\t]` 的项之后失败：写入会把它们改成 `x`、空格）、no-new-check、flip-wrong-byte、flip-uppercase、base-unchecked、budget-held、undecided、wrong-action、rule-readers、adapter-offset、no-extensions-toggle、handler-swaps、no-log、no-wiring。
- S3：reader-enabled、reader-enabled-page、no-prevent、two-at-once、checked-wrong、no-report、undo-enabled、undo-listening、focus-lost、focus-read-after-undo、lock-not-reread、mismatch-not-reread、refusal-kept、not-registered、not-reread-after。
- S4（e2e）：not-registered、offset-plus-one、guard-skips-sessionless、no-prevent 都让 C10 失败；focus-read-after-undo 通过：Chromium 在下一次渲染时才执行焦点修正，撤销之后再读焦点也读得到。说明随之改正（`89b5e0f`），这个次序由组件测试的 `focusFixup` 钉住（S3 的同名变体失败）。
- 审查修复：extract-unsorted、would-go-out-of-range、unchanged-no-recheck、focus-any-item、focus-scrolls、no-name、no-name-page、name-kept、name-with-sublist、gate-open、gate-open-page、gate-unused、locked-alert、locked-no-focus。
- 核对之后的修复：state-ignored、flip-ignored、name-past-blocks、focus-anywhere、refusal-after-edit、gate-forever、end-clears-next。
- 除上面说明的两项，都失败。
