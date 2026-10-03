# M5/P5 自动保存与闲置（前端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m5-p5`（`b0c3d4f..dd31e91`：3 个提交，S1 `1857c41`、S2 `27d7110`、S3 `dd31e91`；25 个文件，+847/−54），对照 [05-P5-autosave-idle.md](../05-P5-autosave-idle.md) 第 1–6 节、三份 Step 计划、[M5 总设计](../00-M5-design.md)第 4.7–4.9、4.13 节、[P4 文档](../04-P4-edit-lock-web.md)与 [M4/P6 移交](../handoffs/M4-P6-editor.md)第 1、3、5 项，以及实现者列出的 8 处有意的出入 |
| 审查方式 | 两位独立审查者（Opus）在仓库上只读：A 看编辑器与 store（控制的约定、宿主的订阅与关闭、nt-3、两个扩展、结束时放弃在途的心跳），B 看页面与端到端（`PageEdit` 的控制、闲置的说明、组件测试、`holdContentWrites`、C4–C6、PG7–PG10、人工清单）。两位都跑了单元、组件测试与类型检查。修复之后另由一位 Opus 核对（见"修复的核对"） |
| 日期 | 2026-10-04 |
| 结论 | 没有阻断合并的问题，没有丢文字的路径。宿主的订阅与关闭（载入、销毁、StrictMode 的第一个宿主）、nt-3、自动保存的频率、组合中的保存、闲置的重新计时、结束时放弃在途的心跳都成立。合并之前修 B 的 Important I1（离不开的闲置退出每 30 分钟挪一次焦点），其余 Minor 修掉。A：Minor 6；B：Important 1、Minor 5（与 A 重合 1） |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | **离不开的闲置退出每 30 分钟挪一次焦点**：`leaveIdle` 经 `leave()` 再经 `save()`：有冲突时 `save()` 把焦点移到冲突的标题，保存失败时 `leave()` 把焦点移回编辑器。这两处对 Done、Mod+E 是对的（用户刚按了键），对没有用户动作的闲置退出不对，也违反 3.6"自动保存不把焦点移到冲突的标题"。B 用探针复现（冲突：焦点到了标题；离线：焦点从 Save 到了正文） | 已修：闲置的离开安静地保存（`save(true)`：有冲突时什么也不发、不挪焦点），失败时不把焦点移回编辑器。组件测试"有冲突时闲置什么也不发、不挪焦点"、"保存失败时留下，焦点仍在 Save"；idle-quiet-save-loud、idle-failure-focus 失败 |
| A-m3 | Minor | 同 B-I1 的冲突一半（A 用探针复现） | 同上 |
| A-m1 | Minor | **自动保存撞上冲突，焦点在打字中途被拿走**：租约在睡眠中过期、别人写过，A 醒来继续打字，自动保存答 409 `revision_mismatch`，冲突的 effect 把焦点移到标题，之后的按键落在不能编辑的标题上。M4 里冲突只来自 Mod+S 或 Done。A 用探针复现 | 已修：每次保存记下是不是用户要的；冲突的 effect 只为用户要的保存移焦点；自动保存、闲置的保存撞上冲突时面板照样出现、焦点留在原处，状态栏说"这一页在你编辑时被改过：见上方"（`<output>` 播报）。组件测试；conflict-focus-unasked、quiet-save-asked、bar-no-conflict 失败 |
| A-m2、B-m2 | Minor | **等组合结束的闲置退出，结束时已经过时**：计时到时组合开着，离开排在组合之后；用户回来在组合里继续输入（每一步只重新计时扩展自己的计时器），确认之后照样保存、离开，说"30 分钟没有输入"。B 另指出：等待期间失锁，结束之后照样走 `leave`，焦点离开横幅。两位都用探针复现 | 已修：等到组合结束时再判断一次：其间正文变了（版本不同）或会话已失，就留下。组件测试"等组合时用户回来：留下，30 分钟后才离开"、"等组合时失锁：横幅留着焦点"；idle-stale、idle-lost-left 失败 |
| A-m4 | Minor | 闲置的离开等组合时编辑器先没了，`controls.leave()` 的 promise 永不完成（违反它的说明"照样完成"）；今天无害，闲置的 `onClose` 同时清掉了 | 已修：句柄的 `whenComposed` 也接 `drop`，`leaveIdle` 以完成作为它的 `drop`。组件测试"等组合的闲置离开在编辑器先走时完成"、编辑器测试"句柄等组合的动作随编辑器走，告诉它的 drop"；idle-not-dropped、handle-drop-lost 失败 |
| A-m5 | Minor | **正文读不出来的编辑一直持锁**：会话开了、心跳着，读正文失败，显示"重试"，没有编辑器也就没有闲置计时，锁一直被持有，违反负责人的决定 3 | 已修：正文没读到时 `PageEdit` 自己计 30 分钟，到时照闲置离开。组件测试；unread-not-timed 失败 |
| A-m6 | Minor（文档） | `Composed.reconfigure(name, null)` 卸不下经控制做的事：自动保存、闲置的订阅与计时器不在它的 compartment 里 | 已改：`reconfigure` 的说明写明只去掉扩展放进编辑器状态的部分，经控制做的留到状态结束 |
| B-m3 | Minor（文档） | 人工清单第 11、12 步写错：停顿超过 2 秒时自动保存已在等组合结束，确认之后随即保存，不是"约 2 秒之后"；第 12 步离开标签页去改名会让浏览器确认或取消组合 | 已改：第 11、12 步写"确认之后随即保存"；第 12 步用终端里延迟的 `curl` 改名，标签页不离开 |
| B-m4 | Minor（e2e） | PG7 仍钉死中间的 revision（3、4），S3 计划说只断言最后的；键之间停顿 2 秒以上时自动保存多写一版 | 已改：revision 取答复里的 |
| B-m5 | Minor | 闲置的说明挂上时就带着文字，读屏多半不播报；焦点落在 Edit 上，只读出"编辑，按钮" | 已修：说明显示时 Edit 以 `aria-describedby` 指向它。组件测试；note-not-describing 失败 |
| B-m6 | Minor（测试） | 组件测试证明得比标题少：保存失败的测试的 `puts.at(-1)` 与失败时记下的相同；"30 分钟之后保存并离开"的保存其实是自动保存的；页面上没有"有输入时重新计时" | 已补：失败的测试断言存下的正文与 PUT 的次数；页面上的"有输入时重新计时"；闲置自己的保存由保存失败的测试覆盖 |
| A 的测试缺口 | — | 没有钉住 3.3 的"载入新正文不动等组合的动作"；`onChange` 的测试没有撤销 | 已补：编辑器测试"载入新正文留着等组合的保存，组合结束之后照样保存"；`onChange` 的测试加了撤销；load-drops-waiting 失败 |

## 对出入的判断

两位都认为 8 处有意的出入成立。A 更正出入 2 的理由：在途的心跳也可能答 200 或 `taken_over`，但结束之后这些答复本来就被丢弃，放弃它什么也不失去；注释随之改为"结束的编辑不听它的答复"。A 另指出出入 4 正说明了 A-m6。B 逐一追过 PG9、PG10：它们数的窗口里没有 2 秒的空档，晚到的自动保存要么并进在途的保存，要么无事可发；C2、C3 的快进 20 秒触发的自动保存也无事可发。

## 修复的核对

修复（`dde58ae`）之后另由一位 Opus 只读核对：跑了页面、编辑器、store、应用与文案的测试和类型检查，在临时目录的副本里逐项撤掉修复，确认新测试都因此失败（例外见 C-N1），并写了几个探针。结论：没有 Important，九处修复都对（状态栏的次序成立：失败与冲突不会同时有，保存中、忙排在冲突之后；`drop` 的路径里 `close()` 先于扩展的续接，关闭之后不再计时），A-m1 与 A-m2 不完整。Minor 3、Nit 2，处置如下。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-m1 | Minor | **安静的保存替用户要的保存说了话**：`asked` 由最后一次保存决定；Ctrl+S、Save、Done 的保存（或 409→重开→409→读的链）还在外时，之前按键排的自动保存到时、并在它之后，把 `asked` 置为假，用户自己的冲突不再拿焦点；Keep mine 之后 2 秒内的自动保存同理，新面板的焦点落到 `<body>`。PG8（页面）正是这个形状，链超过约 2 秒就会失败。探针复现 | 已修：`PageEdit` 数着用户要的保存还有几个在外，安静的保存只在一个也没有时才把 `asked` 置为假。组件测试"Ctrl+S 撞上的冲突拿焦点，后面并着自动保存也一样；Keep mine 的再一次也一样"（扣住 PUT 过 2 秒）；quiet-lowers-always、keepmine-uncounted 失败 |
| C-m2 | Minor | **A-m2 的版本检查漏了两种回来**：组合结束而正文没变（按原样确认拼音、点别处确认），没有新的事务，照样离开；等待期间按"放弃我的"，`load()` 不加版本，照样离开（浏览器里很窄：按下鼠标就结束了组合）。探针复现 | 已改：闲置的离开等过组合，组合一结束就留下：只有用户结束组合，那就是输入；扩展 30 分钟后再试。版本检查随之去掉。组件测试"等组合的闲置在组合结束时留下（正文没变），字保存，30 分钟后离开"；waited-ignored 失败 |
| C-m3 | Minor | 正文没读到的计时不管"重试"：29 分钟时按重试、再失败，30 分钟照样离开，重试还在外也一样 | 已修：从最后一次读起算（计时随 `readFailure` 重来）。组件测试"重试让未读的 30 分钟重新算"；unread-not-restarted 失败 |
| C-N1 | Nit | "等组合时失锁"的测试钉住的是 B-I1 的不挪焦点，不是失锁的再判断：组合总有未保存的修改，安静的保存在失锁时失败，照样留下 | 不改测试：等过组合的离开现在一律留下（C-m2），失锁的判断只对不等组合的一条路有用，由"失锁、没有未保存的修改"的测试钉住 |
| C-N2 | Nit | "见上方"方向反了：状态栏在冲突的面板之上；文档 3.6、3.8 已不符 | 已改为"见下方"；文档在第 3 节同步时改 |

## 持续集成

- S3（`dd31e91`）、审查修复（`dde58ae`）推上之后四个任务都绿。
- 核对之后的修复（`637cb39`）：四个任务都绿，合并为 `46db699`。

## 反向对照

- S1：autosave-not-reset、autosave-close-keeps、autosave-uncaught（最初通过：`vi.fn` 给它答的 promise 挂了处理，拒绝不算未处理；测试改用普通函数之后失败）、idle-no-rearm、idle-change-ignored、idle-close-keeps、idle-close-rearms、nt3-no-reject、nt3-not-dropped、load-is-change、load-not-closing、destroy-not-closing、change-kept-after-close、leave-not-reached。
- S2：autosave-unregistered、idle-unregistered、lock-unregistered、conflict-as-mod-s、idle-when-lost（只有"失锁而没有未保存的修改"能看出来，测试因此加了这一种）、idle-not-composed、idle-not-said、notice-dropped、notice-kept（最初通过：再进入编辑时阅读视图整个卸下，只有被拒时看得出来，测试改为被拒）、notice-alert。
- S3：end-keeps-beat（单元）；e2e 的 autosave-unregistered、save-not-waiting-composition、idle-never-leaves、idle-not-said、c4-no-hold（撤掉扣住只在最后失败：本地 B 的步骤快于 2 秒）与 c4-no-hold-slow（撤掉扣住、输入之后等 2.5 秒：横幅那一步失败；保留扣住、同样等 2.5 秒照样通过）。
- 审查修复：conflict-focus-unasked、quiet-save-asked、idle-stale、idle-lost-left、idle-failure-focus、idle-quiet-save-loud、idle-not-dropped、unread-not-timed、note-not-describing、bar-no-conflict、load-drops-waiting、handle-drop-lost。`leaveIdle` 里"有冲突就留下"的判断去掉：安静的保存已让它留下，撤掉它的变体没有测试能失败。
- 核对之后的修复：quiet-lowers-always、keepmine-uncounted、waited-ignored、unread-not-restarted。
- 都失败。
