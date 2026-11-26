# M13/P4/S1 页头、属性条与右栏：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p4-s1`（从 `m13-ui-refresh` 的 `92cbffd9` 拉出）：`f7eb1100`（照 6.1 提交 1：`main` 的留白，`ColumnTop` 与升级横幅进内容列，页的两栏与头一行（面包屑、`HeadMeta`、`PanelToggle`、页面菜单、`EditButton`），右栏粘住、可收起（`nwiki.panel`），小节的图标与箭头、反链卡片、两列的属性、版本一行，`useCurrentHeading` 与大纲读到的一节，`useBeside`；PG17 第 1 格第 1–14 步、UP1 的两句、hi1、hi2，`threeBrowsers` 与 `REQUIRED` 两行；38 个文件）、`717f5334`（照 6.1 提交 2：属性条与 `propertyRoles`，编辑时头一行的状态与按钮（三个落点、`EditHeadSlots`），标题下的编辑说明，锁说明的样子与 `bdi`，`asset-dialogs.tsx:221` 的标签组；PG17 第 15 步与第 3 格、`REQUIRED` 两行、pg5 的标题、读锁说明的九处；24 个文件）、`d29b68b0`（照实加：PG17 第 5 步把标题滚到窗口顶下 100 px、第 15 步读属性表的角色，变异 12e、t6 活下来之后补；1 个文件）、`ed95b2cf`（第一轮的修复：没有列出的标题时不量，`settings-page.tsx` 过时的注释，`main` 的观察者一格、897 一格、中文那次的打开照它们说的写；5 个文件）、`2ec9ce68`（第一轮的修复：`.nw-reading` 的 `isolation: isolate`，小块 12 px 的圆角，视图离标题 14 px，锁与句子一项，嵌套表的 `inline-flex`，"当前"的透明边框；5 个文件）、`addcfa69`（第一轮的修复：决定 899 的"编辑过"与 `Isolated` 的 `nameClass`，决定 898 的 `settingsColumnClass`，PG17 第 2 步；7 个文件）、`19518390`（修复核对第 1 轮之后：PG17 第 12 步先等锚点的标题拿到焦点；1 个文件）、`c6297488`（同上：锁说明与版本页一行的 `min-w-0 wrap-anywhere`，`justify-end-safe`，收回 4 px 只作用在视图上，PG17 第 4 格；6 个文件）、`f698d37e`（同上：读到的一节看视图、不看 `main`；2 个文件）、`286d01b7`（同上：PG17 第 14 步进不带锚点的地址；1 个文件）、`e2fd05b0`（修复核对第 2 轮之后：第 4 格的 `REQUIRED` 两行，版本页一行的不断行空格，`current-heading.ts` 的注释；5 个文件）；设计 [04-P4](../04-P4-reading.md) 4.1、5.1、5.4、5.5、6.1、7.1、7.2 的 S1 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的分离工作树里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（读 diff 与设计、构建分支头；l6 大页的每帧代价以临时的 Playwright 探针在 Chromium 里量，1440×900，每帧滚 40 px、120 帧，`browser.startTracing` 取 Paint、PrePaint 与 Layout，四种页、两种 DPR、十三种 CSS 对照；偏离 2 照设计的写法加栈追踪跑 4 个 vitest 文件；修复核对时以三种引擎的探针量窄宽度的"编辑过"、静态页的 `safe` 对齐、视图外插入块时读到的一节）；B 管外观（`make build` 之后以临时故事在三种引擎里截图、量盒子，对照 `mockups/reading.html` 与 7.4 的 S1：亮暗、中英，宽 1440、1304、1303、1280、1279、1024、820、375、320，强制色（Chromium、Firefox）；变异 20、35、106 的截图对照；静态探针（小块、锁说明、Firefox 的断行、版本一行）；新增的约 95 个类名查进构建出的 CSS；两轮修复核对逐项重量，isolation 开与关逐像素比）；C 管测试、变异、门禁与完成度（独立跑 `d29b68b0`、`addcfa69`、`286d01b7` 上合并之前的整条命令，抽样重跑实施者的变异、自写变异，探针 `probe-follow`、`probe-unfollowed`、扣住视图的探针 3 与 4，以 `apply82.py` 重跑 8.2 第 1–11 条的替换核对，列出文档提交要带的）。重的运行都经 `heavy.sh` |
| 结论 | 实施中：提交 1 的第一次门禁 7 格红（6 格是设计的 `useFocusLeaving` 在 StrictMode 下把焦点给了标题，1 格是等待的监听），第一次 `make e2e` 另红 hi1、hi2 与 PG17 的亚像素；变异 12e、t6 活下来，照实加 `d29b68b0`；l6 的大页开着右栏约 20 fps，交给审查。第一轮：A 1 Minor、2 Nit、1 Q；B 3 Minor、3 Nit、4 Q；C 2 Minor、10 Nit、1 Q；没有 Important。A 的 Q1 与 C 的 M2 是同一件；C 的 Q1 由 A 的 M1 回答。主持者的决定 897–899 答了 A 的 Q1（C 的 M2）、B 的 Q2、Q3；其余接受，B 的 Q1 交给 P5，B 的 Q4 照现状。修复在 `ed95b2cf`、`2ec9ce68`、`addcfa69`。修复核对第 1 轮：A 2 Nit；B 1 Minor、2 Nit，另报一次 WebKit 的 ResizeObserver 报错；C 1 Important（PG17 第 12 步的竞态）、2 Nit、1 Q（决定 900）；修在 `19518390`、`c6297488`、`f698d37e`，实施者另修第 14 步的同类竞态（`286d01b7`）与版本页一行在 Firefox 里的溢出（M8 起就有）。修复核对第 2 轮：A 1 Nit；B 1 Nit；C 1 Minor（第 4 格没有 `REQUIRED` 的行）、1 Nit；修在 `e2fd05b0`，只动文档的随本文档提交；没有未了的 Important、Minor。另有一次过程的事故（下文）。合并 `715923dd` |

## 基线

S1 不另跑基线（决定 883）：P3/S2 合并之前最后一次门禁的代码与 `072dcec2` 相同，第一个改行为的提交（`f7eb1100`）的 `make e2e` 兼作核对。对照的数是 04-P4 第 1 节的：vitest 221 个文件、5325 格；E2E 624 过、3 败、1 跳过。已知的本机失败同 [P3/S1 的记录](P3-S1-settings-mode-review.md)：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S1 的每次 `make e2e` 与合并之前的整条命令都败 SE2、LW8 两格，同行同消息；PG15 有时过：实施者十次 `make e2e` 与四次合并之前的整条命令里红 6 次，审查 C 的三次门禁里红 2 次，红时都同行同消息（审查 C 的 P4S1C-N4）。另有的红都查清了：提交 1 的第一次 `make e2e`（实施中的发现）；`c6297488` 的 lw8:443、lw9:60 与 `f698d37e` 的 W6:128 [chromium]，都是造数的 API 在负载（约 33）下答 500 `internal_error`，单独重跑都绿；`f698d37e` 的 PG17 `:278` [webkit]，是第 14 步的竞态（`286d01b7`）；审查 C 在 `addcfa69` 上的 PG17 `:242` [chromium]，是第 12 步的竞态（P4S1C-F1-I1，`19518390`）；审查 C 在 `286d01b7` 上的 LW6 `lw6:151` 与 LW9 `fixtures/pages.ts:169` [chromium]，服务端日志都是 `commit transaction: timeout: context deadline exceeded`，重跑这两个文件 12 过。PG17 不碰 PG15、LW8 的文件，那三格的行号不变。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 提交 1 的第一次 `heavy.sh make lint-web knip test-web` 7 格红：`page-edit`（3 格）、`page-autosave`（2 格）、`page-lock`（1 格）的"离开编辑之后焦点在'编辑'上"落在 `h1`；`reading-view.test.tsx` 的"the wait's listeners"多出 4 个监听 | 前 6 格：这三个文件开着 `reactStrictMode`，StrictMode 在"回到阅读，焦点给'编辑'"那个被动效果之后重演新挂组件的 layout 清理，那时"编辑"拿着焦点，设计的 `useFocusLeaving(edit, left)` 立刻把焦点给了标题；改成卸下时在微任务里交出，条件 `edit.current === null && focusLost()`（`f7eb1100`；04-P4 4.1.3 的照实；审查 A 核实，偏离 2）。后 1 格：大纲的 `watchReader` 在列表挂上时加了 4 个捕获监听，那一格改为按监听函数认出等待的那几个，并断言大纲的 4 个在（偏离 3，7.1 的照实） |
| 提交 1 的第一次 `CI=1 make e2e` 623 过、7 败：基线三格之外，hi1:258、hi2:113 的整项文字（标记挪到了谁写的之后），PG17 第 1 格在 Chromium 差 −0.375 px（右栏跟着时切掉项的边）、在 WebKit 差 −0.078 px（页底时右栏的顶） | hi1、hi2 照新的次序改（5.1 漏改，偏离 4）；右栏跟着取 `Math.ceil`（偏离 7，变异 t2）；PG17 右栏的顶留 0.5 px 的容差（偏离 8）。重跑 627 / 3 / 1（`f7eb1100`） |
| 变异 12e 与 t6 在 `717f5334` 上活下来：第 4 步点过的标题被 `pin` 着，第 5 步把标题滚到 0，线在 0 时也读到它；PG17 不读属性表的角色 | 照实加 `d29b68b0`（只动 e2e）：第 5 步把标题滚到窗口顶下 100 px，12e 红在 `:149`；第 15 步读 `role="table"` 与 `rowgroup`，t6 红在 `:270`（5.1 的照实） |
| 7.2 的 16 照 `scrollIntoView({ block: "nearest" })` 写时 PG17 抓不到（右栏整个在视口里，窗口不动），照默认的 `scrollIntoView()` 写时红在 `:183`；18 只有 `page-panel` 抓（`current-heading` 的那一格页上没有 `main`）；31 等价；79、113 在第 3 格红在 375 宽 | 交审查 C（P4S1C-M1、N1、N2），7.2 的行照实改 |
| `upgrade-banner` 的三格在没改的树上本来就绿（横幅原先就画在 `Outlet` 之前） | 由变异 24、25、80 看到红；清单由审查 C 的 P4S1C-N3 改准 |
| UP1 的标题：正文里自己有一个 `# Plan`，`main` 里有两个名为"Plan"的一级标题 | 定位用 `.first()`（壳的 `h1` 在前；偏离 6，5.1 的照实） |
| l6 的大页（14 万个标题，列出 1,000 个）开着右栏从页顶滚时帧间隔中位 50 ms（约 20 fps）：钩子量一次 1,000 个标题中位 0.4 ms，脚本每帧 1.3–2.1 ms，慢的是当前项每帧都变引起的整页重绘（Paint 约 24 ms、PrePaint 约 10 ms） | 交给审查（是不是产品问题），A 找到原因与改法：P4S1A-M1 |
| `addcfa69` 提交之前两次门禁没过：oxlint 的 `no-array-index-key`；898 的新格以 `queryByText` 找横幅，通用页的行里也有那句话，`MutationObserver` 的回调里抛了未捕获的错误 | 改成按 `output` 找（`addcfa69`）。`ed95b2cf` 之前 oxlint 报过一次 `no-shadow` |
| `c6297488` 新写的 PG17 第 4 格先只在 Firefox 红在页上（锁说明），锁说明修好之后再红在版本页（`scrollWidth` 660）：版本页标题下"时间 · 作者"一行的作者名同样冲出页面（375 宽 54 px、320 宽 109 px），M8 起的 `version-page.tsx` | 同加 `min-w-0 wrap-anywhere`，第 4 格也给管理员改长名、核对版本页（主持者在修复核对第 2 轮接受）；oxlint 对第 4 格报的 `consistent-function-scoping`、`no-shadow`、`no-await-in-loop` 改成模块级的 `unbroken()` 与 `fitsAt` |
| `f698d37e` 的 `make e2e` 里 PG17 第 1 格在 WebKit 红在 `:278`（"how far Edit moved down"，22461 px）：480 宽那一步刷新的地址带着 `#nw-part-39`，视图在"Edit"的盒子记下之后才到，窗口滚到"Part 39"、"Edit"跟着走（与 P4S1C-F1-I1 同类） | 那一步改成 `goto` 不带锚点的地址（`286d01b7`）；探针 4（扣住视图，读完"Edit"的盒子再放）改前 WebKit 22461、Chromium 22462 px 红，改后两种都绿、窗口在 0（变异 t23，主持者在修复核对第 2 轮接受） |

## 第一轮的发现与处置

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S1A-M1 | Minor | l6 那种 14 万个块的页上右栏开着滚动约 20 fps（`W/pages/page/page-outline.tsx:77-96,150-160` 每次当前项变了就改右栏，代价在 `W/reading/reading.css:20` 的 `.nw-reading` 不是自己的绘制层）：右栏里任何一次改动（`aria-current`、类、`font-medium`、`scrollTop`）都让根绘制层重画、走遍正文的每一块（每次约 36–37 ms，约 0.26 µs/块）；`font-medium` 改了项的排版，粘住的右栏一排版就让根滚动容器走一遍 PrePaint（约 15 ms）。普通的页碰不到（40 节、1,000 节的页开着与收起都是 60 fps）。十三种 CSS 对照里只有正文自成一层的几种（`position: relative`、`isolation: isolate`、`contain: paint`）回到 16.7 ms | `.nw-reading` 加 `isolation: isolate`（不改正文里 `.nw-math-measure` 的包含块，`position: relative` 会改；`contain: paint` 裁掉贴边的焦点圈）（`2ec9ce68`）。实施者在 Chromium 里重量改前改后、开着收起与普通页（因下文的事故，不靠 A 第二轮的读数）：l6 开着帧间隔中位 50 → 16.7 ms、超过 20 ms 的帧 73 → 2–3、Paint 22.4 → 11.0–11.3 ms/帧；收起 Paint 2.6 → 1.2；普通页不变（04-P4 10.1）。全部 vitest 与 e2e 看过副作用（KaTeX、mermaid、滚动锚定的 PG15、PG17 第 7 步）；B 逐像素比过焦点圈与贴边的东西 |
| P4S1A-N1 | Nit | 没有标题的页（`ids` 为空，`PageOutline` 什么也不画）上钩子照样挂 `scroll`、`resize` 与 `main` 的 `ResizeObserver`，每次滚动排一帧、`read()` 循环 0 次 | `active && ids.length > 0` 才量，否则走不活跃那一支，答的 `current` 同样（`ed95b2cf`）；`current-heading.test.ts` 新格"with no heading listed…"先红（没改时挂了监听、观察了 `main`），变异 t11 |
| P4S1A-N2 | Nit | `W/app/settings-page.tsx:152` 的注释"md:text-right: text-end would read as a colour"在 S1 之后不成立：S1 把 `text-end` 加进了 `noColour` | 删掉过时的那半句，`md:text-right` 不动（`ed95b2cf`） |
| P4S1A-Q1 | Q | 读者在右栏里滚过、不再跟之后，进编辑再回到阅读：`PageOutline` 随 `!editing` 重挂，回调 ref 新建 `watchReader`，右栏又开始跟；决定 886 写"到下一次换页为止"，4.1.7 只说"大纲变空又有项时读者的记录重起"。照实记，还是把"动过"记在 `PageShell` 按页保留 | 决定 897：照实（同 P4S1C-M2）。代码不变；`page-panel.test.tsx` 加一格（取自 C 的探针），不会先红，变异 t12 看到它红（`ed95b2cf`）；886、4.1.7、8.2 第 2、7 条照它改（本文档提交） |
| 偏离 1–10、12 | — | 实施者报告的与设计不同之处（04-P4 4.2 之前的"照实"、5.1、7.1） | 接受。A 核过 2 的原因（照设计写，StrictMode 的三个文件 6 格红，栈是 `doubleInvokeEffectsOnFiber → disappearLayoutEffects → commitHookLayoutUnmountEffects`；生产里结局相同，与 `enter()` 不竞争）、1 的懒加载路由（导航提交之前仍是旧的匹配，横幅不会在旧页上闪、也不会两份）、7 的 `Math.ceil`（多滚不到 1 px、下一帧不再动） |
| 偏离 11 | — | `settings-page.tsx` 过时的注释没改 | 不接受"不改"，见 P4S1A-N2 |

A 另核实：`useCurrentHeading` 对 4.1.7（第一次在 layout effect 里同步量；之后只在答的值变了才设状态；三种监听都只排一帧，清理时取消帧、拆监听、断开观察者；不活跃时结束 `pin` 并清当前项，`pin` 在不活跃时什么也不做；`pin` 看过之后出了视口或读者一动就照线算；关着的 `details` 以 `closest` 跳过；"到底"只在页能滚时用；卸下时结束 `pin` 的读者）；大纲（`sameView` 比 `active`，`OutlineItem` 是 `memo`，`plainPress` 不认修饰键，右栏只改自己的 `scrollTop`、从不 `scrollIntoView`，`watchReader` 在列表的回调 ref 里建、在 React 19 的清理里止，没有泄漏的监听）；开关与 `nwiki.panel`（读写经 `read`、`write`，默认展开，右栏藏起时焦点在里面就以 `preventScroll` 交给开关）；头一行（`HeadMeta` 与右栏同键读版本，只一次 `GET`）；编辑的门户（三个落点由回调 ref 记进 `PageShell`，`meta` 里 React 自己的子元素与门户的 `<output>` 不同时存在，失锁时三个落点都空、横幅取焦点，`stayed()` 认两个 `contents` 的 `span`，⌘S 照旧在 `document` 上）；锁说明的 `bdi`；`propertyRoles` 与就地更新（`patchBlocks` 之前 `undo()`，清理只去掉自己加的 `role`）；`ColumnTop` 的各支。

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S1B-M1 | Minor | 折行的属性小块是 9999 px 的胶囊（`W/reading/reading.css:295`），长的值折成两行以上时左上的键与右端的字落到弧线外、压在边框上；375 宽四行的 `summary`、三行的 `code` 在三种引擎里都这样，320 宽更严重 | 小块的圆角 12 px（`2ec9ce68`）：单行 24.75 px 高，看不出差别；折行的 43.5、62.25 px，字都在框里（B 的修复核对，三种引擎） |
| P4S1B-M2 | Minor | 阅读时标题到属性条（或正文第一段）28 px，样稿 14 px：没人拿锁时锁说明的外层（`page-layout.tsx:451`）是空的却仍是 `space-y-6` 的子元素，24 px 的外边距与 `h1` 的 14 px 折叠成 24 px，再加视图自己 4 px 的内边距 | 外层 `empty:mb-0`（不用 `empty:hidden`：`enter()` 与 `toggleRefused` 读到锁之后立刻聚焦它，那时说明可能还没画出，`display: none` 会让这一下落空，A 的修复核对认同）；视图的 4 px 由视图在标题下收回（`2ec9ce68` 写在 `AttachmentDrop` 上，`c6297488` 改成只作用在视图上，见 P4S1B-F2）。标题到属性条 14.00 px，1440、1024、820、375、320、中英文，三种引擎相同；有锁时标题到锁说明 14、锁说明到属性条 28，照旧 |
| P4S1B-M3 | Minor | 锁说明在窄屏上锁的图标单独占一行（`edit-lock-note.tsx:61` 的三个并列的弹性项，句子比图标右边的空间宽时整个换到下一行）；页与版本页同一个组件 | 图标与 `output` 包进 `div.flex.min-w-0.items-center.gap-x-3`，作为一项一起换行（`2ec9ce68`）；三种引擎里图标都和句子在同一行（Firefox 里长名字的溢出见修复核对第 1 轮的 P4S1B-F1） |
| P4S1B-N1 | Nit | 属性条里嵌套表的小块在 Firefox、WebKit 里键比值高约 1.5 px，小块也比同一行的高 1.4–1.7 px（`reading.css:340-342` 的 `inline-block`） | 嵌套表与它的 `tbody` 改 `display: inline-flex`（`2ec9ce68`；B 提的 `tr` 设 `align-items: center` 量过没用）：三种引擎都是 24.75 px，键对齐（偏差 0） |
| P4S1B-N2 | Nit | 强制色下右栏的"当前"没有框（`page-versions.tsx:156` 没有边框），旁边的"编辑中"有 | "当前"加 `border border-transparent`，`px-1.75` 改 `px-1.5` 保住宽度（`2ec9ce68`）：高 18.5 px（样稿 `.tag` 也是），Chromium 与 Firefox 的强制色下有框 |
| P4S1B-N3 | Nit | 320 宽英文 NUL 的失败四行，标题下移 32 px；"too slow"三行下移 16 px（4.1.9 只记了 375 宽的三行），三种引擎相同 | 照实记进 04-P4 10.1，不截断（本文档提交） |
| P4S1B-Q1 | Q | P3 的 `sectionAt`（`W/app/settings-directory.tsx:53`）精确比较路径：笔记本设置的地址带尾随斜杠时路由匹配到"基本信息"，`sectionAt` 认不出，走 `shown === undefined` 一支，页没有 `h1`、不在列里，S1 起升级中时横幅铺满 `main` | 不在 S1 改，记进 04-P4 第 9 节交给 P5：P5 做其余页面，先核对工作区与个人设置是否同样（本文档提交）。决定 898 之后那一支的横幅已在设置的列里（B 的修复核对：带斜杠的地址也是 x 498、y 44、宽 696） |
| P4S1B-Q2 | Q | 决定 892 那一支的横幅在全宽，进设置的索引地址时横幅先在全宽、再跳进 696 px 的列（三种引擎里 `MutationObserver` 记下的次序） | 决定 898：那一支的 `ColumnTop` 包进 `settingsColumnClass` 的列，没有横幅时 `empty:hidden`（`addcfa69`）；`upgrade-banner.test.tsx` 新格先红（第一次见到横幅时 `{column:false,title:false}`），变异 106、t13；e2e 里改前三种引擎都是先在全宽（x=292、宽 1108）再进列（x=498、宽 696），改后两次都在列里 |
| P4S1B-Q3 | Q | 名字长时"编辑过"省略掉的是时间（`page-head.tsx:49` 整句 `truncate`；英文、中文都是名字在前），1440 宽时三种引擎都看不到时间 | 决定 899：只截名字、保住时间（`addcfa69`）。句子是 `flex justify-end overflow-hidden whitespace-pre`（`c6297488` 起 `justify-end-safe`），名字的 `bdi` 是 `min-w-0 truncate`（`Isolated` 加 `nameClass`）；中英两格与 PG17 第 2 步先红（三种浏览器红在"how much of the sentence is cut"），变异 t14–t16、t19；三种引擎、中英文、1440 与 375 宽，句子截掉 0 px，名字截掉 163–243 px，时间完整 |
| P4S1B-Q4 | Q | 长面包屑在宽屏上折很多行（三层较长的路径，1440 宽 4 行、头一行 78 px，1024 宽 6 行、117 px），右边一组照旧占 416 px，里面只有一句被省略的"编辑过" | 照现状：决定 894 为"行数只看宽度、不看模式"付的代价，记进 04-P4 10.1（本文档提交） |
| 变异 20、35、106 的截图 | — | 三条都成立：20 写在 `line-clamp-2` 之后时长标题由两行（35.75 px）变四行（71.5 px），写回原来的位置（类名最前）是等价的（`cn` 的 tailwind-merge 把 `line-clamp` 与 `block` 当作 `display` 的一组，合并掉前面的 `block`）；35 让两行的失败说明的盒子与组 37.5 px、标题下移 5.5 px；106 让索引那一段与带尾随斜杠的地址没有横幅 | 接受；7.2 第 20 行写明位置（本文档提交）；106 在决定 898 之后有了格 |

B 另核对过、没有问题的：三种引擎相差不到 1 px，亮暗、中英文；头一行在 1280 宽与样稿的 1280 舞台逐像素一致（面包屑 x=292、头一行 y=12、高 36、下到标题 30；"编辑"30 px、13 px、500、胶囊；开关与菜单 30×30；`meta` 12.5 px、行高 16、`--faint`、靠右；组内间距 6 px、面包屑分隔两边 4 px 是设计写定的）；标题在阅读、刚进编辑、空、打了字、已保存、两行的失败、冲突、失锁时都不动（1440、1024 是 78，820 是 99.5，375、320 是 155.5），组一直 32 px；标题 32 px、650、行高 40、下 14 px，编辑说明 12.5 px、在标题下 2 px、失锁时不画；属性条（小块 24.75 px、9 px 的左右内边距、`surface-2` 底、12.5 px，键 `--faint`、值正文色，属性条下到正文 30 px，`role="table"` 在，375、320 宽折行、不横向滚动）；右栏（宽 252、贴右、`sticky`、内边距 64/16/20/18，小节标题 12 px、图标 13 px、箭头 14 px、小节间 24 px，大纲 13 px、当前项 2 px 的琥珀线、强制色下改为下划线，长标题两行截断，反链卡片、属性两列、版本一行、"当前"11 px 在第一行最右）；收起之后 1440、1304、1303、1280 的列 720 px、居中，1303 展开时 719；`main` 的留白 1280 起左右 40 px；升级横幅在页里 720 列里、设置页 696 列里、找不到的页在 `main` 顶；锁说明 14 px、`surface-2`、9 px 的圆角、名字在 `bdi` 里、按钮 30 px 的胶囊；新类名都在构建出的 CSS 里，`design-tokens.test.ts` 6 格全绿，`cn` 合并大纲、状态、外框的几组类名不丢东西；决定 896 在 S1 不适用（S1 的"保存"编辑时一直显着，状态到"保存"10 px）。

### 审查 C（测试、变异、门禁与完成度）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S1C-M1 | Minor | `current-heading.test.ts:260-265` 的"without a ResizeObserver it reads all the same"走不到它要守的 `typeof ResizeObserver` 守卫：第一次 `probe()` 时页上没有 `main`，`main === null` 先短路；C 的 18-ch 只跑这个文件，14 格全绿；7.2 第 18 行写它抓得住，不对 | 那一格在第一次 `probe()` 之前放一个 `main`（`ed95b2cf`；`f698d37e` 之后放 `main > article.nw-reading`）；18-ch 之后 2 格红（那一格 `ReferenceError`，连带的"unmounted, it listens no more"：effect 在加了监听之后才抛，监听漏到下一格）；7.2 第 18 行照实改（本文档提交） |
| P4S1C-M2 | Minor | 8.2 第 2、7 条的新文字说读者动过之后"不再跟，到换页为止""自打开这一页起"，代码里读者的记录跟着大纲的列表走（编辑时不画大纲，进编辑再回来右栏又跟；C 的 `probe-follow` 坐实：`scrollTop` 708） | 同 P4S1A-Q1，决定 897：两条照代码改，第 7 条写"自大纲出现起"（本文档提交）；C 的探针成了 `page-panel.test.tsx` 的一格 |
| P4S1C-N1 | Nit | "113 只在 375 宽红"不成立：第 3 格先跑 375 宽，红在那里就停了；C 把第 3 格改成只跑 820，对照绿，79-820、113-820 都红在 `:288` | 10.1 与 7.2 第 79、113 行写"375 宽红；820 宽由审查 C 单独跑，也红"（本文档提交） |
| P4S1C-N2 | Nit | 7.2 第 16 行的"PG17 第 8 步（窗口跟着动）"：照 `nearest` 写 PG17 照过，照默认写法红在 `:183`（项的底），窗口也不动；`nearest` 只有 `page-panel` 的 `scrollIntoView` 桩抓 | 7.2 第 16 行照实改（本文档提交） |
| P4S1C-N3 | Nit | "不是先红"的清单不准：`general-page.test.tsx:665-688` 原有、没改；另两格新格在没改的树上也绿，没列（`upgrade-banner` 的版本页一格、`page-panel` 的"folded away… measures none"） | 10.1 与 7.1 的照实照 C 改，各写为什么不会先红、哪条变异看到它红（25、80、73、24；版本页那一格是"照旧"）（本文档提交） |
| P4S1C-N4 | Nit | 10.1 草稿的"之后两次 pg15:113 都过了"：C 在 `d29b68b0` 上的门禁里它又红了，同行同消息 | 10.1 记下它仍有时过（本文档提交） |
| P4S1C-N5 | Nit | 10.1 草稿没有 PG17 每格的时长（7.5 要的"照实记一次"） | 10.1 与本文的门禁一节记下（本文档提交） |
| P4S1C-N6 | Nit | 草稿"做完"写"892–894"：决定 894 的后一半在 S3；S3 的变异 108 的抓手在 S1 的第 3 格里，C 在 S1 的代码上做了一条（没有修改时卸下"保存"），红在 `:316` | 10.1 写"894 的 S1 一半"，记下 108s1（本文档提交） |
| P4S1C-N7 | Nit | t1、t8 的"等价"没写依据（05-lessons 第 1 节）；C 重跑：t1 在四个文件 103 格全绿，t8 在 `page-panel` 17 格全绿 | 7.2 的 t1、t8 两行写出依据（C 给的两条）（本文档提交） |
| P4S1C-N8 | Nit | 7.1"点按"一条说不挂 `unfollowed` 时 jsdom 的"Not implemented"经 `setup.ts` 的桩判红；C 去掉挂载再跑：打出四次，格照样绿（那句话不经被桩住的 `console.error`） | 7.1 那一句照实改；`unfollowed` 照旧有用（本文档提交） |
| P4S1C-N9 | Nit | `page-edit.test.tsx:578` 中文那一半的最后一句读的是英文那次的 `server`，中文那次的打开没断言到 | 断言改成两次的打开都在（`[server.sent, chinese.sent]`，`ed95b2cf`） |
| P4S1C-N10 | Nit | 10.1 草稿的"变异""视觉""门禁"写"见上"，指的是报告；没有照实改的各节的清单 | 10.1 写全，列出照实改的各节（本文档提交） |
| P4S1C-Q1 | Q | l6 大页开着右栏约 20 fps，S1 合并之前要有去向 | 由 P4S1A-M1 回答（`2ec9ce68`） |
| handoff 的"处理进展" | — | C 给了 `docs/v0.2/handoffs/M12-polish.md` 一行的草稿（12.10 的截断一半、第 6 项"编辑"的一半、第 5、11 项里 P4 的页上的 `bdi`、第 7 项照实看过） | 照草稿写，加决定 900 与"编辑于现在"的措辞（本文档提交） |

C 另核实：`d29b68b0` 上合并之前的整条命令（下文门禁一节）；`REQUIRED` 四行与 `threeBrowsers` 的 `page\/pg17-`，变异 26 下 `check.mjs` 退出 1、列出四格；三个提交的说明（前两个的标题与 6.1 逐字相同，作用域与改到的目录一致，`d29b68b0` 写 `e2e:`）；提交 1、2 的门禁日志与提交时间相符，提交 1 的 `page-head.tsx` 只导出 `HeadMeta`、`PanelToggle`，knip 不会在提交 1 报出没人用的导出；8.2 第 1–11 条的旧文字在 `92cbffd9` 的总体设计里依次各出现一次，新文字在那一步之后也各一次，第 1、3、4、5、6、8、9、10、11 条与代码相符（第 2 条的 1280–1303 px 与代码的类算得上），第 2、7 条见 P4S1C-M2；10.1 草稿里提交、测试的数与先红的格对得上日志；抽到的变异里，除 M1、N1、N2 之外都与报告一致，31、t1、t8 的等价照说法活下来；`current-heading.test.ts` 的 14 格、`page-panel.test.tsx` 的 10 格新格、`page-layout.test.tsx` 的 6 格、`page-edit`、`edit-lock-note`、`property-roles`、`preferences.store`、`upgrade-banner`、`column-top` 的格都在测它们标题说的，桩都还原；改了的旧格保住原意（`page-versions.test.tsx:103`、`version-restore.test.tsx:156`、`reading-view.test.tsx` 的等待监听、九处 `lockNote`、hi1、hi2、UP1 的 `.first()`）；PG17 的否定断言都在一个肯定的之后，第 15 步的区域隔两帧再数，第 3 格每遍各扣一次内容的 `GET`、只扣 `GET`，换文档都经 `goto`、`reload`；设计第 9 节交给实施者的四件都做了。

## 过程的事故

第一轮里，审查 B 在 16:31:25 想停掉自己排队的 `heavy.sh`，按 PID 发了 `kill`，没先核对命令行：那个 PID 是审查 A 正持锁的 `heavy.sh …/p4s1-rev-a/run-perf.sh`（16:28 起）。它的 EXIT trap 删了锁，A 的 `run-perf.sh` 失去父进程、照跑完；B 排队的运行随即拿到锁，B 的 Chromium 故事与 A 的第二轮测帧并行了约 45 秒（16:31:25–16:32:15）。A 第二轮 `perf2-dpr1.json` 的末段读数不可信；A 说重叠段（收起、再展开、不跟三格）的读数与第一轮相同，结论不变。处理（主持者）：l6 的数由实施者在修复提交之后重量（改前改后、开着收起），不靠 A 的第二轮（P4S1A-M1）；之后的审查简报加一句：只停自己起的进程，停之前以 `ps -o command= -p PID` 核对命令行。修复核对两轮里三位与实施者都照此做，没有再出事。

## 修复核对第 1 轮

在 `addcfa69` 上（对照 `d29b68b0`），三位各在自己的分离工作树里（经 `heavy.sh`）。

- **A**：M1、N1、N2、Q1（决定 897）都已处理。M1 的数照实施者的重量（与 A 第一轮以注入的 CSS 量的一致，没有再量）；N1 的变异改回 `active` 时新格红（返回值里那个 `ids.length > 0` 改掉不会红：`ids` 为空时 `read()` 本来就答 `undefined`，多余的守卫，无害）；897 的格先让读者在右栏里滚轮，再进出编辑，断言右栏重新跟到第 35 项，t12 让它与视图晚到一格红。新行为的核对：锁说明外层的 `empty:mb-0` 只改外边距，元素照旧在、照旧 `tabIndex={-1}`，`enter()` 与 `toggleRefused` 照旧聚焦得到；`Isolated` 的 `nameClass` 别处不带，`className={undefined}` 时 React 不写 `class`；`isolation: isolate` 的副作用（阅读视图里没有 `z-index`、`fixed`、`sticky`、`mix-blend-mode`，KaTeX 的 CSS 也没有；菜单与对话框经 Radix 挂到 `body`；S2 代码块头的 `sticky` 在 `pre` 里；`.nw-reading` 本来就 `overflow-x: auto`，焦点圈靠 4 px 的内边距；版本页与回收站的预览同样隔离，无害；滚动锚定与它无关）；决定 898 那一层有横幅时与 `SettingsPage` 同列同内边距、不取焦点；决定 899 在 320、340、375、480 宽另量（F1-N2）；17 个相关的 vitest 文件 507 格全绿。
- **B**：各项已处理（三种引擎、浅色深色，宽 1440、1024、820、375、320、中英文）：M1 单行 24.75、多行 43.5、62.25 px；M2 读到之后标题到属性条 14.00；M3 图标与句子同一行；N1 三种都是 24.75；N2 18.5 px、强制色下有框；898 从索引地址与带斜杠的地址进来横幅都在 (x 498, y 44, 宽 696)；899 名字的 `bdi` 宽 174、210、121、66 px，时间完整。isolation 的 A/B（同一场景截一张，再注入 `.nw-reading{isolation:auto!important}` 截一张，逐像素比七个场景）：Firefox、WebKit 每个相同，Chromium 最大差 3/255，只是栅格化。别处没动（标题的 y、头一行与右栏同第 0 轮，进出编辑标题不动，变异 35 的场景不变，大纲的长标题 35.75）。
- **C**：M1（18-ch 之后 2 格红）、M2（897 一格就是 C 的探针；t12 让它与视图晚到一格红）、N9 已处理；N1–N8、N10 与 handoff 一行待文档提交；Q1 由 A-M1 回答。新格是否测它说的：A-N1 一格数监听与 `observe`，t11 红；898 一格的 `max-w-178` 只有 `settingsColumnClass` 用，检查不空，t13、106 都红；899 两格读类名、PG17 第 2 步核对行为，t14 两格红与 PG17 `:134`；t16 见 F1-N1。门禁（负载 19 → 62）629 / 4 / 1：基线三格与 PG17 `:242`（F1-I1），不算绿。

新的发现：

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S1C-F1-I1 | Important | PG17 第 12 步偶发红：第 10 步点了"Part 39"，地址带着 `#nw-part-39`，刷新之后页第一次拿到视图时 `focusOn(target)` 把焦点给锚点的标题（`view-anchors.ts:72-80`），不看读者其间做了什么；视图答得慢时，故事先 `toggle.focus()`、Enter 展开，视图这时才到、焦点被拿走，第二次 Enter 打在 h2 上，右栏不收起，红在 `:242`。C 的门禁在负载 62 时就这样红；`probe3.sh`（刷新之后扣住视图的 `GET`，第一次 Enter 之后才放）稳定复现。第 12 步自 `f7eb1100` 起没变，设计 5.1 的"刷新之后焦点在 body"对带锚点的地址不成立 | 刷新之后先等 `heading("Part 39")` 拿到焦点，再 `toggle.focus()`（`19518390`）；实施者照 C 的探针写 `probe-c3.sh`（另加 3 秒的兜底，会等视图的写法也能跑完）：改前红在 `toBeHidden`、焦点在 `<h2 id="nw-part-39">`，改后绿、焦点在开关上（变异 t22）；5.1 第 12 步照实改（本文档提交） |
| P4S1C-F1-Q1 | Q | F1-I1 背后的应用行为：带锚点打开的页，视图第一次到时总把焦点给锚点，读者在视图到之前按过头一行的按钮（S1 加了开关）也照样被拿走；13.2 第 26 条说异步的答复之后移动焦点要看读者动没动。M6/M12 的写法，不是 S1 带来的 | 决定 900：照现状，记给 v0.2（04-P4 第 9 节、`docs/v0.2/handoffs/M12-polish.md`，本文档提交） |
| P4S1B-F1 | Minor | Firefox 里长的锁持有人名字（连字符接数字，生成的名字、拿邮箱当名字；UAX #14 的 HY×NU 不断）冲出锁说明，页面横向滚动：375 宽 57 px、320 宽 112 px，页与版本页一样；第 0 轮就有 31、56 px，B 当时漏看，图标占一列之后更大；Chromium、WebKit 在连字符处折行 | 锁说明的 `<output>` 加 `min-w-0 wrap-anywhere`（`c6297488`）；实施者顺带找到版本页"时间 · 作者"一行同样溢出（M8 起），同加；新的 PG17 第 4 格在 375 宽 `expectFits` 页与版本页（只在 Firefox 先红，变异 t17、t18）。实量（三种引擎，页与版本页，固定的长名与像邮箱的名字，375 与 320）：Firefox 改前 283、338 px 与 56、111 px，改后 0；Chromium、WebKit 改前改后都是 0 |
| P4S1A-F1-N2 | Nit | 320 宽、能编辑的账户（"编辑"与菜单都在）：`meta` 盒子 152 px，时间是"10–59 minutes ago"时名字缩到 0 px，省略号也画不出，"Edited by"的头被裁 0.7–0.9 px（`justify-end` 加 `overflow-hidden`，三种引擎同样）；340 宽以上与 375、480 宽正常 | `justify-end-safe`（`c6297488`；899 的两格加读这个类名，先红，变异 t19）：改前开头被裁 0.7（Chromium、WebKit）、0.92 px（Firefox），改后开头不裁、裁的是结尾的同样的量；名字仍是 0 宽，照实记进 10.1（本文档提交）。`safe` 在 Safari 17.6 之前整条声明被丢掉，退成靠左 |
| P4S1B-F2 | Nit | 读取中与读取失败时内容离标题 10 px，不是 14 px：`AttachmentDrop` 的 `[:first-child:empty+&]:-mt-1` 对 `NotLoaded` 也生效，它没有那 4 px 内边距（1440 宽：标题底 118，读取中的盒子与失败的提示框 128，读到之后的属性条 132） | 收回 4 px 的类改成只作用在视图上：`[:first-child:empty+&>.nw-reading:first-child]:-mt-1`（`c6297488`；没用 `pt-1` 的外层）。三种引擎、1440、375、320、带属性条与不带的页：改前读取中、失败 10，改后 14；读到之后的属性条与第一段 14，同改前（变异 t20，只由实量看） |
| P4S1A-F1-N1 | Nit | "编辑过"一句改成 flex 之后三段都成了块级的弹性项，三种引擎里 `innerText` 与选中复制都是三行（`"Edited by \n<名字>\n yesterday"`）；读屏按行读时会不会拆成三段，没核实 | 照实记进 10.1（决定 899 的代价）；读屏的一条记进 04-P4 第 9 节"收尾"的读屏清单（本文档提交） |
| P4S1B-F3 | Nit | 刚编辑过的页读作"<名字> 编辑于现在""Edited by <名字> now"（`page.editedBy` 加相对时间一直有的措辞），"刚刚编辑""just now"更顺；省略号与时间之间约 10 px 的空 | 照实记下，留给以后（04-P4 第 9 节 v0.2 一条与 M12-polish 的进展一行，本文档提交） |
| P4S1C-F1-N1 | Nit | t16（`bdi` 只留 `truncate`）不能简单写"等价"：行为上三种引擎等价（`truncate` 带 `overflow: hidden`，这样的弹性项自动最小宽度是 0），但 899 的两格 vitest 照类名断言，t16 下红 | 7.2 的 t16 一行照此写（本文档提交） |
| P4S1C-F1-N2 | Nit | 照决定 897 改 8.2 第 2 条时，S3 第 22 条的旧文字"长标题至多两行；编辑时没有大纲（M13/P4/S1）。"就在它的新文字里 | 原样保住那一句；本文档提交的脚本改完 S1 的第 1–11 条之后，把 S2、S3 的第 12–26 条在结果上依次再换一次，旧文字各一次、新文字各一次 |
| B 顺带：WebKit 的 ResizeObserver 报错 | — | B 的临时故事（反复改窗口宽、开关抽屉、切语言）在 WebKit 里报了一次"ResizeObserver loop completed with undelivered notifications"，夹具算成未捕获的异常；来源没查 | 实施者查出是 S1 的观察（04-P4 4.2 之前的"照实"：每次报错时 `katex-*.css` 都在途，`main` 在滚动区的回调同步读布局之后又变了、它不比那一轮最浅的深，被判为没送到；改前 120 次 23 次报错，只拦掉 `main` 的观察 0 次）；改看视图 `main article.nw-reading`，与滚动区自己的目标一样深（`f698d37e`），改后 0 次；vitest 的观察者一格改成"看视图、不看 main"，先红（变异 t21） |

另外，实施者在 `f698d37e` 的门禁里撞上第 14 步的同类竞态（实施中的发现，`286d01b7`）。B-F1 之外两处超出处置表的改动（版本页一行的 `wrap-anywhere`、第 14 步的写法）由主持者在第 2 轮的处置里接受。

## 修复核对第 2 轮

在 `286d01b7` 上（对照 `addcfa69`），三位各在自己的分离工作树里（经 `heavy.sh`）。

- **A**：各项都已处理、处理得对。`f698d37e` 的诊断成立：实施者打点的记录（`ro-6-before/webkit`）里，KaTeX 的样式表还在途时同一轮先交出滚动区的 `article@11` 与区块 `@12`，再交出钩子的 `main@5`，报错就在这一刻，下一帧 `main` 从 1591 变成 1662；规范只在下一轮交出比上一轮最浅的那个更深的观察，这一轮里 `main` 又变了，深度 5 不比 5 深，于是被跳过、报错。改后观察的深度只剩 {11, 12}，与 M12 起滚动区自己的相同，风险回到 S1 之前。没有不损失的更好目标：比视图浅的都可能撞上同一种跳过；同时看右栏的列表没用（右栏粘住、高同窗口，它的尺寸从不挪动标题）；等 CSS 载入之后才 `observe` 也去不掉这个条件。代价三种引擎里量了（04-P4 10.1 的两种过时），可以接受。`c6297488`：焦点的目标仍是外层 `div`，`output` 不可聚焦；`justify-end-safe` 构建出 `justify-content: safe flex-end`，静态页探针在三种引擎里盒子 110–150 px 时开头不再被裁；上提 4 px 的选择器只作用在文章本身，`NotLoaded` 离标题 14 px，文章宽到可聚焦时焦点圈外扩 4 px、离标题仍有 6 px；`current-heading`、`page-layout`、`version-page`、`page-lock`、`edit-lock-note`、`page-panel`、`reading-view`、`page-tasks` 8 个文件 205 格全绿。`19518390`、`286d01b7` 只动故事，第 12 步等的正是决定 900 的行为，断言没有变弱。
- **B**：各项已处理（三种引擎、浅色深色）。锁说明：六种名字（短的、带空格与连字符的、连字符接数字的、像邮箱的、60 个字母的长词、中文的长名字）× 页与版本页 × 1440、375、320，都不横向滚动，说明的右边界都在框里；B 的横幅测试里 Firefox 的溢出 57、112 → 0，Chromium、WebKit 的量一点没变。版本页的作者一行：同样六种名字，1440、820、375、320，中英文，不溢出（分隔点见 F2-1）。读到之前的 14 px：读取中、读取失败、文件夹的提示（合成的拖放事件）、读到之后带与不带属性条，都是 14；读到之后视图自己 `margin-top: -4px`，文章在标题下 10，同设计。`f698d37e`：isolation 的 A/B 与上一轮逐像素相同，两次运行里没有再报 WebKit 的 ResizeObserver 错（只跑了一次那个场景，不算证明）。别处没动（阅读、编辑、横幅与锁的场景逐项比，只有 Firefox 的锁说明变高与读取中、失败下移 4 px，是本意）；新类名都在构建出的 CSS 里，`design-tokens.test.ts` 6 格全绿，`cn` 保留了 `AttachmentDrop` 的变体类。
- **C**：F1-I1 已处理：C 的探针 3 套在新的第 12 步上绿，第二次 Enter 时焦点在开关上；t22 加同一探针红在 `toBeHidden`、焦点在 h2。第 14 步：探针 4 在 Chromium、WebKit 都绿、`scrollY` 0；t23 加探针 4 两种都红。M1 在 `f698d37e` 之后仍成立（格里放 `main > article.nw-reading`，18-ch 2 格红）。新格：起头"PG17 (page): long names break"与另三格互不为前缀；`threeBrowsers` 按文件，`check.mjs` 报 chromium 430、firefox 102、webkit 102、webkit-narrow 3，共 637；t17、t18 只在 Firefox 红（页 `:390`、`scrollWidth` 658；版本页 `:392`、660），vitest 的 EL（9 格）、VP（28 格）都不抓：这一格是这两处唯一的守卫，而且只靠 Firefox，`REQUIRED` 却没有它的行（F2-M1）。变异重跑（`mut/batch-3.out`）：t17、t18、t21、t22 加探针 3、t23 加探针 4、18-ch 都照说法红，不带变异的探针 3、4 绿。门禁（负载 15.5 → 57.9）637 格 632 / 4 / 1、3.0 min：基线 SE2、LW8 与造数的 500 两格（LW6、LW9，重跑 12 过）；PG17 九格都过。8.2 第 1–11 条重跑 `apply82.py` 仍都恰好一次。提交说明的作用域都与改到的目录一致。

新的发现：

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S1C-F2-M1 | Minor | `tools/e2e-projects/check.mjs` 的 `REQUIRED` 缺第 4 格的 firefox、webkit 两行；决定 879 与 00 号文档 4.8 要每格两行。今天这个文件已被第 1、3 格的行钉在 `threeBrowsers` 里，眼下没漏跑；但这一格哪天挪到不在正则里的文件，就只在 Chromium 跑、永远绿，没有检查会报 | 加两行 `["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): long names break"]` 与 webkit 的一行，注释写 Firefox 的 HY×NU（`e2fd05b0`，作用域 `web, tools`）；`check.mjs` 在 `e2fd05b0` 的门禁里过（列 chromium 430、firefox 102、webkit 102、webkit-narrow 3） |
| P4S1C-F2-N1 | Nit | 格数的写法过时：00 号文档 4.8"两格，S1、S2 各一"，决定 879"两格"，8.2 第 10 条只写第 1、3 格 | 四格（S1 第 1、3、4 格，S2 第 2 格）：00 号文档 4.8 与第 7 节 S1 一行、决定 879、5.1、5.4、5.5、8.2 第 10 条照此改，8.2 第 10 条加第 4 格一句；S1–S3 的替换整串再换一次（本文档提交） |
| P4S1A-F2-N1 | Nit | `current-heading.ts:88-92` 的注释说视图旁边变高的是"a lock's note above it, the backlinks below"：反链在右栏里，宽屏上右栏粘住、高同窗口，它变高从不改变页高；视图之后真正会变高的是子页与附件两节（附件是异步读的） | 注释改成"the subpages and the attachments below"（`e2fd05b0`）；A 量的两种过时照实记进 861、4.1.7、10.1（本文档提交）；A 提的在内容列上挂 `MutationObserver`（`childList`、`subtree`，回调同样只排一帧）的补法，主持者定不做：代价只在页顶、页底一阵，下次滚动就对 |
| P4S1B-F2-1 | Nit | 版本页一行里名字放不下时"·"单独占一行：`<span>·{" "}<Link>…</Link></span>`，浏览器先在"· "处换行，`wrap-anywhere` 在那之后才起作用（Firefox 连字符接数字的名字在 820、375、320 宽，三种引擎像邮箱的名字在 320 宽、长词在 375、320 宽）；可选：种类前的"· "同样，免得最后一行只剩"Edit" | 两处都换成不断行空格（`e2fd05b0`）：`version-page.tsx` 的"·"之后，`versionBy` 的种类之前（它只用在版本页的这个链接上，右栏的 `VersionBy` 照旧，hi1、hi2 不受影响）；读这段文字的 `version-page.test.tsx` 十处随之写 ` `，t24、t25 下 5 格、1 格红；B 的静态探针三种引擎里点都跟着名字走、不溢出；整条 `make e2e` 重跑过（下文门禁） |
| B 顺带 | — | 版本页的标题在 320 宽被"Restore this version"挤成"Version / 1"两行（旧的） | 记进 04-P4 第 9 节交给 P5（本文档提交） |
| C 未核实 | — | 页顶时视图之上出现锁说明，读到的一节暂时可能判错 | A 已量（04-P4 10.1），照实写进 861、4.1.7、7.1 |

修复核对到此：三位都没有未了的 Important；C-F2-M1 与三条 Nit 由 `e2fd05b0` 与本文档提交处理，由主持者核对（02-workflow 第 1 节第 7 步）：`tools/e2e-projects/check.mjs` 的 `REQUIRED` 加了第 4 格的 firefox、webkit 两行（标题以"PG17 (page): long names break"起头，与 spec `:363` 的标题对得上；`e2fd05b0` 的 `make lint-web` 列出 chromium 430、firefox 102、webkit 102、webkit-narrow 3）；`current-heading.ts` 的注释改成视图之后的子页与附件两节；版本页"·"之后与 `versionBy` 种类之前换成不断行空格，`version-page.test.tsx` 读这段文字的 10 处随之改，t24、t25 各红；三条文档的 Nit 照本记录写进 04-P4、00 号文档与总体设计（脚本断言每段旧文字恰好一次，S1–S3 的替换整串重放过；`check_links.py` 0 个新坏链接）。

## 主持者的决定（负责人可以改判）

全文在 [04-P4](../04-P4-reading.md) 2.3。

- **897 读者在右栏里动过之后不再跟，到换页、或进出编辑之后大纲重新出现为止**（审查 A 的 P4S1A-Q1、C 的 P4S1C-M2）：照代码，读者的记录跟着大纲的列表走；不把"动过"挪到 `PageShell` 按页记。886、4.1.7、8.2 第 2、7 条照它改，第 2 条里 S3 第 22 条的旧文字原样保住。
- **898 决定 892 那一支的横幅在设置的列里**（审查 B 的 P4S1B-Q2）：`NotebookSettingsLayout` 的 `shown === undefined` 一支把 `ColumnTop` 包进与 `SettingsPage` 同样的列，进设置的索引地址时横幅不在全宽闪一下。
- **899 "编辑过"一句名字长时只截名字、保住时间**（审查 B 的 P4S1B-Q3）：名字的 `bdi` `min-w-0 truncate`，其余不缩，中英文都核对；PG17 第 14 步照旧。
- **900 带锚点打开的页，视图第一次到时焦点给锚点、不看读者其间的动作**（审查 C 的 P4S1C-F1-Q1）：照现状，记给 v0.2。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份，跑完从备份复原，以 `git diff --quiet HEAD -- <文件>` 核对（02-workflow 3.1）；改坏期间不跑 `make gen*`、不用 `git checkout --`。实施者以 `p4s1/mut.py`（清单 `mutations.py`）在 `717f5334` 上跑 vitest 的各行，e2e 的各行（编号后加 `e`）经 `heavy.sh` 重建前端、在 Chromium 里跑那个故事，在 `d29b68b0` 上全部重跑过；修复之后以 `mut2.py`（`mutations_f2.py`）在 `addcfa69`、`286d01b7` 上、以 `mut3.py`（`mutations_f3.py`）在 `e2fd05b0` 上跑改到的各行与新的 t 行（第 4 格的 t17、t18 跑三种浏览器）。脚本与日志在会话的临时目录 `p4s1/`，不进仓库。

设计 7.2 的 S1 各条（1–36、70–81、102–106、104b、112–115）除下表都被 7.2 写的格抓住（7.2 照实改过的行写的是现在的抓手）：

| # | 改坏 | 结果 |
|---|---|---|
| 12、12e | 线在视口的顶 | `current-heading.test.ts` 11 格红；PG17 在 `717f5334` 上抓不到（第 4 步点过的标题被 `pin` 着，第 5 步把标题滚到 0，线在 0 时也读到它）；`d29b68b0` 之后红在第 5 步 `:149`（审查 C 重跑同） |
| 16、16-plain | 读到的一节变了时调 `scrollIntoView` | 照 `{ block: "nearest" }` 写：`page-panel.test.tsx` 2 格红，PG17 绿（右栏整个在视口里，窗口不动）；照默认的 `scrollIntoView()` 写：PG17 红在第 8 步 `:183`（项的底），窗口也不动（审查 C 的 P4S1C-N2） |
| 18、18-ch | 去掉 `typeof ResizeObserver` 的守卫 | 第一轮：`page-panel.test.tsx` 15 格红，`current-heading.test.ts` 14 格全绿（P4S1C-M1）；`ed95b2cf` 之后 17 格红，只跑 `current-heading` 时 2 格红；`f698d37e`、`286d01b7` 上重跑同 |
| 20 | 大纲的链接写回 `block` | 无格抓；写在 `line-clamp-2` 之后长标题四行（审查 B 的截图），写回原来的位置等价（`cn` 合并掉它）；修复之后照 B 的写法重做：71.5 px 对 35.75 px，只有截图看得出 |
| 23 | `columned` 时 `NotebookLayout` 另外也画 | `upgrade-banner.test.tsx` 13 格红，UP1 红在 `:240`（23e）；`general-page.test.tsx:665` 照设计说的不红 |
| 26 | `threeBrowsers` 不含 PG17 | `node tools/e2e-projects/check.mjs` 退出 1，列出 PG17 的格（审查 C 重跑同） |
| 31 | 属性的值不 `overflow-wrap: anywhere` | 等价：Chromium（实施者）、Firefox 与 WebKit（审查 C 的 31-ffwk）都绿；t10 改成 `normal` 时第 15 步红 |
| 35 | 状态的盒子去掉 `leading-4` | 无格抓：审查 B 三种引擎里量出两行的失败说明的盒子与组 37.5 px、标题下移 5.5 px |
| 79、113 | 右边一组不定宽；去掉 `min-h-8` | 79 红在第 1 格第 14 步 `:255` 与第 3 格 `:288`；113 红在第 3 格 `:288`；第 3 格先跑 375 宽，红在那里；审查 C 改成只跑 820 宽（对照绿），79-820、113-820 都红在 `:288`（P4S1C-N1） |
| 104、104b | `plainPress` 只看主键；只排除 Cmd、Ctrl | 分别在 Control、Shift 那一次变成第 9 项 |
| 106 | 索引那一支不画 `ColumnTop` | 第一轮无格抓（审查 B 以截图看到它成立：索引那一段与带尾随斜杠的地址没有横幅）；`addcfa69` 之后 `upgrade-banner.test.tsx` 决定 898 的一格红 |
| 112 | `sameView` 不比 `active` | 一格（"shown again…"）红，7.1 的两种情形都在这一格里 |

实施者另写的（04-P4 7.2 表末的 t1–t25；行号是当时的）：

| # | 改坏 | 结果 |
|---|---|---|
| t1 | `EditButton` 的微任务去掉 `edit.current === null` | 等价：四个文件 103 格全绿（依据见 P4S1C-N7） |
| t2 | 右栏跟着时不取整 | PG17 第 8 步红 |
| t3、t4、t5 | 不写 `data-saved`；失败时不用 `text-destructive`；"保存"进了 after 的落点 | `page-edit.test.tsx` 各红 |
| t6 | `propertyRoles` 不注册 | `717f5334` 上没有格抓；`d29b68b0` 之后 PG17 `:270` 红 |
| t7 | 去掉 `aria-controls` | `page-panel.test.tsx` 红 |
| t8 | 窄屏时也把焦点交给开关 | 等价：`page-panel` 17 格全绿（依据见 P4S1C-N7） |
| t9 | `active` 不看 `useBeside()` | `page-panel.test.tsx` 红 |
| t10 | 值 `overflow-wrap: normal` | PG17 第 15 步红 |
| t11 | A-N1 退回 | `current-heading.test.ts` 1 格红 |
| t12 | 读者动过按页记 | `page-panel.test.tsx` 2 格红（897 一格、视图晚到一格） |
| t13 | 898 那一层去掉 | `upgrade-banner.test.tsx` 898 一格红 |
| t14 | 回到整句 `truncate` | `page-layout.test.tsx` 2 格红；PG17 第 2 步 `:134` 红 |
| t15 | 去掉 `whitespace-pre` | `page-layout.test.tsx` 2 格红 |
| t16 | `bdi` 只留 `truncate` | `page-layout.test.tsx` 2 格红；PG17 三种引擎都绿（行为等价，P4S1C-F1-N1） |
| t17 | 锁说明的 `output` 去掉 `min-w-0 wrap-anywhere` | PG17 第 4 格只在 Firefox 红（页的宽）；Chromium、WebKit 绿；`edit-lock-note.test.tsx` 不抓 |
| t18 | 版本页一行去掉 `min-w-0 wrap-anywhere` | 同 t17，只在 Firefox 红（版本页的宽）；`version-page.test.tsx` 不抓 |
| t19 | `justify-end-safe` 改回 `justify-end` | `page-layout.test.tsx` 2 格红 |
| t20 | B-F2 退回（收回 4 px 作用在 `AttachmentDrop` 上） | 无格抓；临时故事量出三种引擎读取中、读取失败回到 10 px |
| t21 | 改回观察 `main` | `current-heading.test.ts` 观察者一格红（`[[main]]` 不等于 `[[article]]`） |
| t22 | 第 12 步不等锚点的标题拿到焦点 | 探针 3 下红（焦点在 h2，`toBeHidden`） |
| t23 | 第 14 步回到带锚点的刷新 | 探针 4 下红（22462、22461 px） |
| t24 | `versionBy` 种类前的不断行空格改回普通空格 | `version-page.test.tsx` 5 格红（找不到名为"Ada · Edit"的链接） |
| t25 | 版本页一行"·"之后的不断行空格改回普通空格 | `version-page.test.tsx` 1 格红（那一行的 `textContent`） |

审查者另写的（B 没写代码的变异，变异 20、35、106 以截图对照；A 的十三种 CSS 对照见 P4S1A-M1）：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| 108s1 | C | S3 的 108 在 S1 的代码上：没有修改时卸下"保存" | 第 3 格 `:316`（"375: the status's right"）红；S3 照旧要跑 108 |
| 31-ffwk | C | 31 在 Firefox、WebKit | 都绿：三种引擎里等价 |
| 18-ch | C | 18 只跑 `current-heading.test.ts` | 第一轮 14 格全绿（P4S1C-M1）；修复之后 2 格红 |
| 79-820、113-820 | C | 79、113，第 3 格只跑 820 | 都红在 `:288`；对照 ctl-820 绿 |
| 16-plain | C | 16 照默认的写法 | PG17 `:183` 红 |
| t16-ffwk | C | t16 在 Firefox、WebKit | 都绿 |
| 抽样重跑 | C | 实施者的 12、13、17、24、25、33、74、76、79、80、104、104b、112、113、114、16、t1、t8、26（第一轮），18-ch、t11–t14、t16、106（第 1 轮），t17、t18、t21、t22 加探针 3、t23 加探针 4、18-ch（第 2 轮） | 除 P4S1C-M1、N1、N2 之外都与实施者的结果同格；等价的照说法活下来 |

对照（只改测试，或测试加变异）：C 的 `probe-follow`（读者滚过右栏、进出编辑之后右栏又跟，`scrollTop` 708：P4S1C-M2，成了 897 的格）、`probe-unfollowed`（去掉 `unfollowed` 时"Not implemented"四次、格照样绿：P4S1C-N8）、探针 3 与 4（扣住视图，不带变异时绿）；实施者读锁说明的探针（去掉锁的释放事件，三格 `toBeNull` 红）。

没有格守的：20、35、t20（截图与实量看）；版本页一行"·"跟着名字走（B 的静态探针看；不断行空格本身由 t24、t25 守）。等价的：31、t1、t8、t16（e2e 里）。

## 门禁与 CI

| 提交 | 实施者 | 审查 C |
|---|---|---|
| `f7eb1100` | 提交之前第一次 `heavy.sh make lint-web knip test-web` 7 格红（实施中的发现），之后绿（223 个文件、5365 格，1:02.5）；第一次 `CI=1 make e2e` 623 过、7 败、1 跳（实施中的发现），之后 627 / 3 / 1，2.7 min，整条 2:44；全部 vitest 另跑三遍，5365 格全绿，"not configured to support act"0 次 | 只看了日志与提交的时间，没有逐个重跑 |
| `717f5334` | 绿（224 个文件、5374 格，1:06.9）；`make e2e` 631 / 2 / 1（pg15 过了），2.7 min | 同上 |
| `d29b68b0` | vitest 不变；PG17 三种浏览器都绿（`fix-green.log`）；e2e 由合并之前的整条命令跑（下表） | 合并之前的整条命令（下表） |
| `ed95b2cf` | 第一次 oxlint 报 `no-shadow`，提交之前改了；绿（5376 格，1:05.6）；`make e2e` 630 / 3 / 1，2.7 min | — |
| `2ec9ce68` | 绿（5376 格，1:12.7）；630 / 3 / 1，2.6 min | — |
| `addcfa69` | 前两次没过（实施中的发现）；绿（5379 格，1:08.9）；630 / 3 / 1，2.7 min | 合并之前的整条命令（下表） |
| `19518390` | 绿（5379 格，1:15）；631 / 2 / 1，3.6 min | — |
| `c6297488` | 绿（1:11）；632 / 4 / 1，3.1 min：基线两格与 lw8:443（建页）、lw9:60（建笔记本）[chromium]，造数的 API 在负载约 33 下答 500 `internal_error`，单独重跑都绿 | — |
| `f698d37e` | 绿（1:15）；632 / 4 / 1，2.9 min：基线两格与 PG17 `:278` [webkit]（第 14 步的竞态，`286d01b7`）、W6:128 [chromium]（接受邀请答 500，单独重跑绿） | — |
| `286d01b7` | 绿（1:12）；633 / 3 / 1，2.7 min（基线三格） | 合并之前的整条命令（下表） |
| `e2fd05b0` | 绿（224 个文件、5379 格，vitest 68.6 s，整条 1:15.9；`check.mjs` 列 chromium 430、firefox 102、webkit 102、webkit-narrow 3）；`CI=1 make e2e` 634 / 2 / 1（基线两格），3.2 min，整条 3:16.5，负载 40.3 → 33.1；版本页一行的文字变了，这一次就是为它跑的 | — |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `d29b68b0`（实施者；C） | 实施者：lint、knip、gen-check、build 过；224 个文件、5374 格；e2e 631 过、2 败（se2:231、lw8:836，同行同消息；pg15 过了）、1 跳，3.1 min，整条 4 分 40 秒，负载 20.8 → 24.7；受保护路径与 `git diff --check` 0。C（16:09–16:13，负载 18.1 → 38.7）：224 个文件、5374 格（73.0 s）；e2e 630 / 3 / 1（基线三格），3.1 min，整条 4 分 39 秒；受保护路径与 `git diff --check m13-ui-refresh...d29b68b0` 0，跑完树干净 |
| `addcfa69`（实施者；C） | 实施者：5379 格；e2e 631 / 2 / 1（pg15 过了），2.6 min，整条 3 分 45 秒，负载 10.0 → 31.2；受保护路径与 `git diff --check` 0。C（负载 19.2 → 62.5）：5379 格；e2e 629 / 4 / 1，2.7 min：基线三格与 PG17 `:242` [chromium]（P4S1C-F1-I1），不算绿 |
| `286d01b7`（实施者；C） | 实施者：5379 格，"act"0 次；e2e 634 / 2 / 1（se2:231、lw8:836），2.6 min，整条 3 分 47 秒，负载 11.8 → 39.2；受保护路径、`git diff --check` 与分支的 `--check` 0，`git status` 为空。C（18:56 起，负载 15.5 → 57.9）：5379 格；e2e 632 / 4 / 1，3.0 min：基线两格与 LW6 `lw6:151`、LW9 `fixtures/pages.ts:169` [chromium]（服务端日志 `commit transaction: timeout: context deadline exceeded`，造数的 500），经 `heavy.sh` 重跑这两个文件 12 过；受保护路径与 `git diff --check` 0 |
| `e2fd05b0`（实施者；合并的门禁） | lint（`check.mjs` 同上）、oxfmt、knip、gen-check、build 过；224 个文件、5379 格（82.5 s），"act"0 次；e2e 633 过、3 败（se2:231 [chromium] `toHaveValue`、pg15:113 [webkit] `toEqual`、lw8:836 [webkit] `toBeFocused`，基线三格，同行同消息）、1 跳，3.8 min，整条 5 分 24 秒，负载 33.1 → 41.4；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 0；`git diff --check` 0，分支的 `git diff --check m13-ui-refresh...HEAD` 0；跑完 `git status` 为空 |
| `009a27ea`（主持者；文档提交，加上它的头即合并 `715923dd` 的树，`m13-ui-refresh` 在这期间没有动过） | 与 `e2fd05b0` 只差 `docs/`（`git diff e2fd05b0..009a27ea -- . ':(exclude)docs'` 为空），合并之前的整条命令已在 `e2fd05b0` 上跑过，不重跑；`git diff --check` 与 `check_links.py m13-ui-refresh`（6 个文件，0 个新的坏链接）干净 |

- 每次都只败基线里同行同消息的格（上面列出的另几格都查清了，见"基线"一节）。
- Go 一侧：受保护路径的 `git diff --quiet` 每次以 0 退出；`m13-ui-refresh` 一直是 `92cbffd9`，分支是快进。
- 先写、先红：见 04-P4 10.1"测试"（提交 1 的 22 格与两个新文件、PG17 第 1 格与 UP1 三种浏览器都红；提交 2 的 6 格与 `property-roles.test.ts`、PG17 第 15 步与第 3 格三种浏览器都红；修复的新格与第 4 格也都先红；不会先红的五格各有变异看到它红）。
- vitest：5325（221 个文件，S1 之前）→ 5365（223，提交 1）→ 5374（224，提交 2）→ 5376（`ed95b2cf`）→ 5379（`addcfa69`；之后不变）。E2E：628 → 631 → 634 → 637（PG17 第 1、3、4 格 × 三种浏览器）。
- PG17 的时长（第 1 / 3 / 4 格）：

| 跑 | Chromium | Firefox | WebKit |
|---|---|---|---|
| 实施者，`d29b68b0` 合并之前 | 2.1 / 7.3 s | 5.4 / 7.5 s | 3.4 / 6.9 s |
| 审查 C，`d29b68b0` 合并之前 | 2.2 / 6.9 s | 5.6 / 7.4 s | 3.6 / 7.7 s |
| 实施者，`addcfa69` 合并之前 | 1.9 / 7.1 s | 4.2 / 6.9 s | 3.0 / 7.2 s |
| 审查 C，`addcfa69` 合并之前 | 11.8（红）/ 7.1 s | 4.7 / 6.8 s | 2.9 / 6.9 s |
| 实施者，`286d01b7` 合并之前 | 2.3 / 6.5 / 0.53 s | 4.5 / 6.6 / 0.84 s | 2.9 / 7.1 / 0.78 s |
| 审查 C，`286d01b7` 合并之前 | 2.5 / 6.7 / 0.5 s | 4.8 / 7.0 / 1.0 s | 3.5 / 6.8 / 0.9 s |
| 实施者，`e2fd05b0` 的 `make e2e` | 2.3 / 7.1 / 0.52 s | 5.4 / 7.3 / 1.3 s | 3.5 / 6.9 / 0.98 s |
| 实施者，`e2fd05b0` 合并之前 | 2.1 / 7.6 / 1.3 s | 7.1 / 7.9 / 1.4 s | 3.7 / 7.7 / 3.0 s |

  S1 的三格合计约 32–42 test-seconds；7.5 推断 PG17 三格 45–72（第 2 格在 S2），第 1 格推断 6–10 s、实测 2–7 s，第 3 格推断 7–10 s、实测 6.5–7.9 s。本机 E2E 多了 9 格，Playwright 2.6–3.8 min（负载 10–62），7.5 推断 2.8–3.0。UP1 的页面格 1.2–3.3 s。
- **只靠 CI 的**：没有新的（PG17 不按 `Control+K`）。Linux 的字体与滚动条下 PG17 的 `expectFits`、右栏的量与 0.5 px 的容差，Firefox 在 Linux 上的断行（第 4 格只在 Firefox 红，CI 的 Firefox 照样跑它），由负责人推送之后的 CI 看。
- **没有重现的 CI 项**：照 [01-P1](../01-P1-foundation.md) 7.3 的表（Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别）。
- **CI**：本机没有推送的凭据（决定 746、761）：负责人推送之后在最后一个头上补跑，看上面只靠 CI 的几项。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库。

- **实施者**（7.4 的 S1）：Chromium 53 张与 copy、l6 的 JSON（04-P4 10.1 列了截了什么、量了什么）；修复之后三种引擎的 `shots-fix-2`（标题到属性条、小块 375 与 320、锁说明页与版本页 1440、375、320、强制色下的"当前"、设置索引的横幅与它的 `settings-banner.json`、名字长的"编辑过"中英文 1440、820、375、大纲的长标题，量值 `measures.json`、`edited.json`）；修复核对第 1 轮之后改前改后的 `shots-fix3-before`、`shots-fix3-after`（锁说明 `lock-{fixed,email}-{page,version}-{375,320}`，"编辑过" `edited-{en,zh-CN}-{30min,3h}-{320,340,375}`，标题到视图 `gap-{strip,plain}-{loading,read,failed}-{1440,375,320}`）。临时故事的源在 `p4s1/visual/`，跑时复制进工作树、跑完即删。l6 的重量在 `p4s1/perf/perf-{before,after-1,after-2}.json`。
- **实施者的 ResizeObserver 探针**（`pg17-zz-p4s1-ro.spec.ts`、`pg17-zz-p4s1-rob.spec.ts`，不进仓库）：页上每个 `ResizeObserver` 包一层，按帧记下 observe、回调、目标的深度、DOM 变动与还在下载的样式表；B 的临时故事原样加上记录，WebKit 里 6 次中 1 次、10 次中 1 次报错；稳定复现是一张带公式与图的页在 WebKit 打开 40 次跑 3 遍，带一个路由（Playwright 有路由时关掉 HTTP 缓存，B 的故事与 PG17 都有）：改前 23/120、拦掉 `main` 的观察 0/120、改后 0/120、开着缓存 0/120（`ro-5`、`ro-6-before`、`ro-6-after`）。
- **实施者的竞态探针**：`probe-c3.sh`（第 12 步，扣住刷新之后视图的 `GET`，第一次 Enter 之后或 3 秒后才放）、`probe-c4.sh`（第 14 步，扣住视图到"Edit"的盒子读完）。
- **审查 B**：三轮临时故事（`pg17-zz-revb`），三种引擎、亮暗、中英，宽 1440、1304、1303、1280、1279、1024、820、375、320，强制色（Chromium、Firefox；WebKit 不模拟）；`measures.json` 逐项量位置、大小与颜色；放大的裁切；静态探针 `probe/`（小块 12 px、锁说明的两种改法、Firefox 的断行、版本一行的不断行空格）；`classcheck.mjs` 核对新类名、`cn-probe.mjs` 核对合并；isolation 开与关的逐像素 A/B；变异 20、35、106 与未改的构建对比。
- **审查 A**：l6 的测帧探针（`zz-rev-a-perf.spec.ts`、`zz-rev-a-perf2.spec.ts`，`perf-dpr1.json`、`perf-dpr2.json`）；偏离 2 的 `mut-edit.py`；修复核对第 1 轮的窄宽度探针（`zz-rev-a-f1.spec.ts`，三种引擎）与静态页 `f1-static/`；第 2 轮视图外的变化（`zz-rev-a-f2.spec.ts`、`f2-outside-{chromium,firefox,webkit}.json`）与 `safe` 对齐的静态页。
- **审查 C**：`mut_c.py`、`mut/own_mutations.py`、`probe-follow`、`probe-unfollowed`、`probe3.sh` 与探针 4、`apply82.py`（结果 `v01-after-s1.md`）。

## 接受与推后

- l6 大页开着右栏的重画：`isolation: isolate` 之后 60 fps；剩下的约 11 ms/帧的 Paint 读数（帧率已是 60 fps）没再拆（P4S1A-M1）。
- 视图之外变高时读到的一节在页顶、页底过时一阵（A 量的两种），下一次滚动或改窗口就对；不补 `MutationObserver`（主持者，P4S1A-F2-N1）。
- "编辑过"的三行 `innerText`、320 宽某些时间下名字看不见：决定 899 的代价，照记（P4S1A-F1-N1、N2）。
- 320 宽英文三四行的失败说明让标题下移 16、32 px，不截断（P4S1B-N3）；长面包屑多折几行（P4S1B-Q4）：照现状。
- 交给 P5：`sectionAt` 的尾随斜杠（P4S1B-Q1）、版本页的标题在 320 宽的两行（B 修复核对第 2 轮）。
- 交给 v0.2：带锚点打开的页的首次焦点（决定 900）、"编辑于现在"的措辞（P4S1B-F3）。
- 交给 S3：读者的记录随大纲的列表（决定 897），给编辑时加大纲时再核对；变异 108 照旧要跑（108s1 已在 S1 的第 3 格里红）。
- 20、35、t20 只由截图与实量看；31、t1、t8、t16（e2e）等价。

## 未核实的怀疑

- **剩下的 Paint 读数**（A）：`isolation` 之后 l6 开着时 Paint 约 11 ms/帧、单次最多 19 ms，帧率却是 60 fps；推测是嵌套的 Paint 事件被重复计了（实施者合并了嵌套的区间，仍是约 11 ms），没有拆 trace 的层级核对。Firefox、WebKit 里 l6 的帧没有量。
- **真的 Safari**（A）：点开关不给它焦点、焦点在右栏里时交给开关，只有 jsdom 的格（`fireEvent.click`）守着；Playwright 的 WebKit 点按钮会给焦点，造不出来。`justify-end-safe` 在 Safari 17.6 之前退成靠左，没在旧的 Safari 上看。
- **读屏**（A）："编辑过"一句三个弹性项，读屏按行读时会不会拆成三段，没核实（记进 04-P4 第 9 节"收尾"的读屏清单）。
- **WebKit 的 ResizeObserver 报错**（B）：改看视图之后 B 的那个场景只又跑了两次，没再报，不算证明；实施者的 120 次载入是证据。滚动区自己的观察者在回调里同步读布局是 M6/M12 的写法，没改；拦掉 `main` 的观察之后那 120 次里它的目标没被判过没送到。
- **"Not implemented: navigation to another Document"**（C）：全部 vitest 每次都打出一次，P3 的门禁日志里也有；是哪一格让 jsdom 去导航，没查。
- **Linux 上的量**（C）：PG17 的 `expectFits`、右栏的量、0.5 px 的容差在 Linux 的字体与滚动条下，等分支的 CI。
- **提交 1、2 单独为绿**（C）：只看了实施者的日志与提交的时间，没有逐个重跑。
- **强制色**（B）：只在 Chromium 与 Firefox 的模拟下看过（WebKit 不模拟）；编辑时"已保存。"前的 ✓ 在强制色下只读了 CSS。
- **B 的其余几条**：版本页的锁说明在 375 宽只截了名字长的；工作区设置、个人设置是否也有尾随斜杠的问题没跑（交给 P5 先核对）；变更集页的面包屑没截到（造数读错了答复的形状），回收站的预览截了、没看出问题。
