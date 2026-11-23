# M13/P2/S2 笔记本与页面树：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p2-s2`（从 `m13-ui-refresh` 的 `7b22f50d` 拉出，S1 已并入）：`485e6be3`（当前笔记本在列表里就地展开：稳定的容器 `notebook-slot.tsx`，笔记本行、齿轮与＋，"更多"一排图标，`intoColumn`，`notebookGroups` 每项只取第一个链接）、`831e50a6`（树的行换装、普通页的文件图标、"目录""历史"两行、当前行滚进侧栏、移动对话框的间距、PG16 两格进三种浏览器）、`3ca04574`（审查的修复：看守按导航换新、跨过 StrictMode，由测试守住；`intoColumn` 挪到 `ShellSidebar` 的注释之前）、`75cac02a`（审查的修复：＋与齿轮的右边、父页的 ›（决定 804）、强制色下齿轮与＋不透明）；设计 [02-P2](../02-P2-shell.md) 4.2、6.2 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的 detached worktree 里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（换组时的容器、焦点、开着的菜单与对话框，别的笔记本的链接，大写与未知的 id，"目录""历史"两行，当前行滚进侧栏与看守，Playwright 探针在三种引擎里，另写 vitest 变异）；B 管外观（Chromium 亮暗 × 1440、375，Firefox、WebKit 亮暗 × 1440，另有读者、触屏与强制色，逐项量位置、大小与颜色，对照 palette.html，样稿也在 1440 下量）；C 管测试、变异、门禁与完成度（独立重跑每个提交的检查与合并之前的整条命令、7.2 的 S2 各条与实施者的变异、自写变异找漏洞、核对 10.2 与"照实"的草稿）。重的运行都经 `heavy.sh` |
| 结论 | 实施中：PG16 第二格照设计的写法抓不到变异 24，实施者加了 375 宽一段（amend 进 `831e50a6`）；自写变异 s5 活下来，大写 id 一格加了断言（amend 进 `485e6be3`）。第一轮：A 2 Nit；B 3 Nit；C 1 Minor、3 Nit、1 Q；没有 Important。Minor 是"首页、按键、开页"一格按在 `document.body` 上，看守有 `within` 之后它什么也不抓，"一次导航一个看守"只在第一次导航上有测试守着（C；A 的同一条是 Nit）。B 的 S2B-N2 由主持者的决定 804 答。修复在 `3ca04574`、`75cac02a`。修复核对第 1 轮：C 在代码与测试上没有 Important、Minor，另有 1 Minor、3 Nit 都在文档草稿里，由本文档提交改；A、B 只有 Nit，照 M12 的规则由主持者核对 |

## 基线

S2 不另跑基线，沿用 S1 开工之前的那一次（[S1 的审查记录](P2-S1-shell-account-review.md)"基线"：没改的 `58c5fa92` 上 `CI=1 make e2e`，594 过、3 败、1 跳过）：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S2 的每一次 `make e2e`（实施者与审查 C 的）都只败这三格，同行同消息；偶发的 AS6 `as6:153`、LW8 `lw8:771` 与 `markdown.test.ts` 的"not slower"都没有出现。PG16 两格在三种浏览器里共 6 次，`make e2e` 由 594 过变成 600 过。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 变异 24（`reveal` 改用 `link.scrollIntoView()`，默认与 `block: "nearest"` 两种）在照设计写的 PG16 第二格里三种引擎都过。Chromium 的探针：每次 `scrollIntoView` 时文档的 `scrollHeight` 是 720，等于 `innerHeight`。宽屏时整列粘在视口里，当前行在树到达时就带进来，那时页的正文还没画、文档只有视口高，窗口滚不动；之后 `scrollY` 一直是 0。只有 md 以下（S3 的抽屉之前侧栏在页之前，自己不滚）`scrollIntoView` 才把窗口滚到那一行（375 宽的探针：没改的树 0，两种变异 1830、1239） | PG16 第二格末尾加 375 宽一段：先 `goto` 笔记本首页，再 `goto` "Page 60"，它的链接在，`window.scrollY` 是 0。没改的树三种都过，两种变异三种都在 :125 红（1830、1239）（amend 进 `831e50a6`；提交说明没写这一段，审查 C 的 S2C-N3）。先经首页：直接对同一地址再 `goto` 时，Chromium、WebKit 里没改的树也滚到 1830、标题有焦点；那两种引擎把同一地址的载入留在原来的历史项里，`history.state.usr` 的 `arrived` 还在，标题照到达取焦点、窗口跟着滚（审查 A 的探针 D、审查 C 的探针证实；Firefox 丢掉 `history.state`）。这是 M3、M12 既有的行为，不是 reveal 的问题（"接受与推后"） |
| 自写变异 s5（两组之后的备用位置不看 `unsavedEdit`）活下来 | `notebook-slot.test.tsx` 大写 id 那一格另断言两组之后紧接"New notebook"；s5 红（amend 进 `485e6be3`） |
| 改名对话框只有 Save，没有 Cancel（`W/app/rename-form.tsx`、`rename-page-dialog.tsx`），设计写的"点 Cancel"做不到 | PG16 第一格以 Esc 取消，格名照实写"which Escape cancels"（`831e50a6`；审查 A 核实那三个文件 S2 没有改） |
| 树的看守也在 `window` 的捕获阶段听读者的四种事件，`reading-view.test.tsx` 的"the wait's listeners go once its read settles"按事件名数到 16 个，不是 4 个 | 那一格改为按监听函数计还在听的；s16 与 C 的 r1、b6 都让它红（`831e50a6`） |

## 第一轮的发现与处置

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S2A-N1 | Nit | 每次导航换新看守没有测试：变异 q24（只看 `watching.reader === undefined`，一棵树一生一个看守）vitest 与 e2e 全过。第一次导航时树还没读到、读者在侧栏的滚动区里滚了一下，`acted()` 为真、`revealed` 不设；之后的导航沿用那个看守，这棵树再也不把当前行滚进来，违反决定 779、791 的"每次导航至多一次"（至多，不是再也没有） | 接受：`page-tree-reveal.test.tsx` 的"树读到之前在滚动区里滚轮"格加第二次导航，到"Page 59"，它的行已画出、在视野外：滚进来；q24（= C 的 b1）在 `:194:44` 红（`3ca04574`） |
| S2A-N2 | Nit | StrictMode 下看守的复位没有测试（只影响开发）：q05 删掉卸下时的 `watching.reader = undefined` 全过；模拟的卸下已 `end()`，重挂之后 `reveal` 沿用结束了的看守，`acted()` 永远为假，读者在侧栏里的滚动不再算 | 接受：加 StrictMode 一格（`configure({ reactStrictMode: true })`，`onTestFinished` 里还原，照 `transfer-import.test.tsx:19`）：树读到之前在滚动区里滚轮，读到之后行不被滚进来；q05（= C 的 b2）在 `:213:24` 红（`3ca04574`） |

A 另核实了实施者报告里的两件事：PG16 第一格用 Esc 属实（`rename-form.tsx:104` 只有一个提交按钮，`dialog.tsx` 不自带关闭按钮）；375 宽下同一地址再 `goto` 滚到标题是 S2 之前就有的（`app/arrival.ts` 在 `7b22f50d..831e50a6` 之间没改），先经首页的绕法合理。

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S2B-N1 | Nit | ＋与读者的齿轮比树行的 ⋯ 与样稿的 `.end` 偏左 2 px：＋的盒子 213–237、图标约 217–233；树行 ⋯ 的盒子 215–239、图标 219–235；样稿笔记本行 `.end` 的图标 219–235（1440，三种引擎相同）。触屏、窄屏或悬停时两列同时出现，看得出 | 接受：＋ `right-1`，写者的齿轮 `right-7.5`，读者的 `right-1`；行的 `pr-14`、`pr-8` 不变，名字到齿轮仍不少于 2 px。＋的盒子 215–239、图标 219–235，写者的齿轮 189–213（图标 193–209），读者的齿轮 215–239（`75cac02a`） |
| S2B-N2 | Nit | 树里父页的 › 比笔记本行的 ⌄ 偏右 4 px：笔记本行的箭头 14–28，"API conventions"的 › 18–32（按钮 16–34）。样稿的树把 › 放在 36，也不与笔记本行对齐；这里的差别来自决定 780 把展开按钮放进缩进的空位。B：可不改，负责人看图定 | 决定 804：展开按钮另带 `-ml-1 mr-1`，› 在 14–28（按钮 12–30），深一层 30–44；链接、页的图标（38 / 54 / 70）、叶子的空位、落点线都不动（`75cac02a`） |
| S2B-N3 | Nit | 强制色模式下齿轮与＋平时仍是 `opacity-40`（S2 之前＋一直 100%）：＋是 ButtonText 加 0.4，对 Canvas 约 2.8:1，齿轮是 LinkText 加 0.4；悬停或聚焦时 100% | 接受：`shown` 加 `forced-colors:opacity-100`；强制色下静止时两者 1，平时的 0.4 不变；发送中禁用的＋ `disabled:opacity-25!` 照旧压过它（`75cac02a`） |

### 审查 C（测试、变异、门禁）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S2C-M1 | Minor | `page-tree-reveal.test.tsx:160-170` 的"首页、按键、开页"一格 `fireEvent.keyDown(document.body, …)`；看守 `within: "[data-sidebar-scroll]"`，区域外的按键本来不算，这一格不管看守怎么建都过。b1（= A 的 q24）在 reveal、tree、writes、readers-input、reading-view、slot 六个文件里全过；三次导航那一格每次都在新看守建好之前点树，也分不出。b1 下第一次导航之后都沿用那个看守，树分批画、行晚到时读者滚开了左栏也照样被拉回去 | 接受：按键改在滚动区里（`fireEvent.keyDown(area(), …)`），格名照实；没改的树过（C 的 h0），b1 下红（h1；实施者 `:170:44`），原写法按在 body 上时 b1 下不红（h2）。连同 S2A-N1 的第二次导航，7.2 加一行 b1（`3ca04574`） |
| S2C-N1 | Nit | = S2A-N2：`page-tree.tsx` StrictMode 下的清空没有测试（设计审查的 P2C3-N1 要的；第 68 条只核对 `end()`） | 同 S2A-N2；C 试写的同一格（p0 过、b2p 红）。7.2 加一行 b2（`3ca04574`） |
| S2C-N2 | Nit | `W/app/shell-sidebar.tsx` 的 `intoColumn` 插在 `ShellSidebar` 的 JSDoc 与函数之间，那段注释不再贴着 `ShellSidebar` | 接受：`intoColumn` 挪到那段注释之前（`3ca04574`） |
| S2C-N3 | Nit | `831e50a6` 的说明 amend 之后没写 375 宽那一段 | 不再 amend：写进 02-P2 的 10.2 与"照实"、本记录 |
| S2C-Q1 | Q | 实施者的报告（report-1）里没有 10.2 的草稿；"照实"要补：S2C-M1 那一格的改法、4.2.1 的新测试文件与改了的 `reading-view.test.tsx`、`makeSlot` 导出、`reveal` 多了 `navigation === undefined`、7.2 第 24 行的抓手、两次 amend、7.5 的实测时间 | 草稿在实施者的交回里，存下时漏了；各项并进本文档提交，准确性见修复核对的 S2C1-M1–N3 |

C 另核实："先经笔记本首页"那一步只避开了同一历史项上留下的到达状态，没有掩盖 reveal 的问题（375 宽直接同址 `goto`：Chromium、WebKit 的 `history.state.usr` 是 `{"arrived":true}`，`h1` 有焦点、`scrollY` 1830；Firefox 的为 `null`、停在 0）；`vi.spyOn` 造的布局与浏览器里只有中段滚动一致（jsdom 的 `scrollTop` 不截断，只影响最后一行的 8 px 余量，真浏览器由 PG16 补上）；knip 干净，`useNotebookColumn` 没有引用；代码与 4.2.2–4.2.5 相符。

## 修复核对第 1 轮

- **C**（在 `3ca04574`、`75cac02a` 上，分离的 worktree，经 `heavy.sh`）：S2C-M1、N1、N2 修好，N3 与 Q1 交本文档提交；两个新提交各自单独为绿，头上合并之前的整条命令只败基线三格（门禁表）。
  - S2C-M1：没改的树六个文件 175 格全过；b1 两格红（`:170:44`、`:194:44`，`expected false to be true`）；h2（换回按在 body 上，再加 b1）首页那一格过；h3（按在 body 上，树不改）过。按键在 `await row("Page 60")` 之后，`waitFor(inView)` 等状态落定。
  - S2C-N1：StrictMode 一格 b2 在 `:213:24` 红（`expected true to be false`）；c1（去掉 `configure` 再加 b2）过：咬住 b2 的确是 StrictMode 的卸下与重挂。
  - 滚轮那一格去掉第二次导航再加 b1（c3）过：抓 b1 的是新加的第二次导航；第一次导航之后"不滚进来"、`scrollTop` 为 0 的断言没动。
  - `reading-view.test.tsx` 原写法放回去报"expected [ 'wheel', 'touchmove', …(14) ] to have a length of 4 but got 16"。
  - 外观的提交没有弄坏故事用到的东西：PG16 第一格（三种）的行菜单、`newPageWith`、pg1:182 的＋都过；展开按钮故事里没人点（只有 `e2e/perf/tree.perf.ts` 按名字点，不在 `make e2e` 里），深度 0 时 `-ml-1` 之后仍在行里；`dragPage` 拖行的中点；齿轮（距右 30–54 px）与＋（4–28 px）不重叠。
- **A、B**：只有 Nit，照 M12 的规则由主持者核对：两个修复提交的改动与各条相符；实施者重截的 `depths-light.png` 与 `chromium-04-tree-hover-side-1440-dark.png` 上父页的 › 在笔记本行 ⌄ 那一列、＋在树行 ⋯ 那一列；b1、b2 的红绿（实施者：没改的树 175 格全过，b1 两格红，b2 一格红）。

新的发现（都在文档草稿里）：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| S2C1-M1 | Minor | 草稿的"照实 7.2"说第 52 行的抓手不含"首页、按键、开页"一格，不对：m52（`PageTree` 的 effect 里建不带 `within` 的看守）下那一格两种写法都红（按在 body 上时，不带 `within` 的看守连 body 上的按键也算）；修复之后 m52 让 reveal 文件五格红 | 本文档提交：第 52 行的抓手照设计原样，删掉那一句；10.2 的变异一条加上这一格 |
| S2C1-N1 | Nit | 10.2"做完"漏了决定 789（PG16 两格是 S2 做的那一半）；决定 804 不在 2.3，头的状态还是 801–803 | 本文档提交：10.2 加 789；2.3 加 804；头的状态写 S2 与决定 804 |
| S2C1-N2 | Nit | "不是先红的格"的单子没有审查修复的三格（都写在代码之后，没改的树上就绿） | 本文档提交：照实 7.1 加一句，由 b1（两格）、b2（一格）守着 |
| S2C1-N3 | Nit | 7.2 表后的说明只写到 S1；10.2 的门禁只写了实施者在头上的那一次，负载"11–37"不含 C 的几次 | 本文档提交：7.2 表后加一句；10.2 写实施者与 C 各一次，负载照实写宽 |

修复核对到此为止。

## 主持者的决定（负责人可以改判）

全文在 [02-P2](../02-P2-shell.md) 2.3。

- **804 树里父页的展开按钮伸进缩进 4 px**（改 780 的摆放；审查 B 的 S2B-N2）：展开按钮另带 `-ml-1 mr-1`，› 落在 14–28 px，与笔记本行的 ⌄ 同列，深一层右移 16 px；`mr-1` 补回左移的 4 px，链接、页的图标（深度 0 在 38 px）、叶子的空位、落点线与更深的缩进都不动。样稿的树把 › 放在 36 px，并不与笔记本行对齐；B 说可不改、看图定，主持者取对齐。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份到临时目录，跑完从备份复原（02-workflow 3.1）。实施者在各自提交的树上写一条跑一条（vitest 的跑相关的几个文件；e2e 的经 `heavy.sh` 重建前端，再跑那个故事，PG16 在三种浏览器里）；审查 C 在 `831e50a6` 上独立重跑实施者的全部（vitest 34 条、e2e 各条），另写自己的，修复核对时在 `75cac02a` 上重跑 b1、b2 与对照。

设计 7.2 的 S2 各条与 S1 交来的 z3：

| # | 改坏 | 抓住它的 |
|---|---|---|
| 16 | `notebookGroups` 改回数列表里全部链接 | e2e n1:103（N1，Chromium） |
| 17 | `NotebookLayout` 直接门户进列表项（不经稳定的容器） | `notebook-slot.test.tsx` (2a)(2b)(2c)(5) |
| 18 | `SlotHolder` 不记、不还焦点 | (2a) |
| 19 | 去掉两组之后的备用位置 | (5) |
| 20 | 别的笔记本的链接不带 `arrived` | (4) |
| 21 | 笔记本行的 `h2` 挪进树的 `nav` | (1)、(2a)、(4)、笔记本行格；`page-tree.test.tsx` 的树与种类等多格 |
| 22 | ＋挪出树的 `nav` | 笔记本行格；`page-tree-writes.test.tsx` 的新建各格 |
| 23 | ＋平时 `invisible` | e2e pg1:182（PG1，Chromium） |
| 24 | `reveal` 改成 `link.scrollIntoView()` | `page-tree-reveal.test.tsx` 的首格（另有 jsdom 没有 `scrollIntoView` 而抛错的多格）；PG16 第二格只靠 375 宽一段（:125，三种，`scrollY` 1830；`block: "nearest"` 1239），宽屏那几步抓不到（实施中的发现） |
| 25 | `reveal` 不看看守 | 滚动区里的滚轮格 |
| 26 | 树的菜单的 `onCloseAutoFocus` 不还焦点 | PG16 第一格（三种，`toBeFocused`） |
| 27 | "目录""历史"两行不用 `NavLink end` | `page-tree.test.tsx` 的 LLM Wiki 两行格 |
| 28 | `threeBrowsers` 里写成 `page\/pg16_` | `check.mjs`（`make lint-web`）：firefox、webkit 两行"lists no cell" |
| 49 | `NotebookNav` 把地址里的 id 转小写 | 大写 id 格 |
| 50 | `SlotHolder` 还焦点之后不带进侧栏的可见范围 | (2a) 的 `scrollTop`（232） |
| 51 | `SlotHolder` 挪了容器之后不通知 | (2c) |
| 52 | 看守在 `PageTree` 的 effect 里建、行在自己的 layout effect 里调 `reveal`（初稿的写法） | 三次以上导航格、"首页、按键、开页"格、主区里的滚轮格（修复之后 C 重跑：reveal 文件五格红）；PG16 第二格（Chromium，第三次导航） |
| 53 | 看守不带 `within` | 主区里的滚轮格；`readers-input.test.ts` 的 `within` 格 |
| 62 | `PageTree` 不比路径，每个 `location.key` 都定 | 同一页换 hash 格 |
| 68 | `PageTree` 卸下时不结束看守 | 树没读到时离开格（`removeEventListener` 各一次） |
| z3 | 只去掉 `md:h-svh`（S1 交来） | PG16 第二格（三种，`toBeInViewport`） |

实施者另写的：

| # | 改坏 | 抓住它的 |
|---|---|---|
| s1 | `containerFor` 每次新建（不按 id 留着） | 多格（`notebook-layout`、`notebook-slot`、`general-page` 等） |
| s2 | "更多"的链接不带 `title` | "更多"的图标格 |
| s3 | 齿轮挪进 `h2` | 多格（笔记本行格、`page-tree-writes.test.tsx` 的焦点各格） |
| s4 | `useMatch` 不带 `/*` | 多格 |
| s5 | 备用位置不看 `unsavedEdit` | 起初活下来；大写 id 格加断言之后红 |
| s6 | `TreeMenu` 不调 `useSlotMoves` | (2c) |
| s7 | `show` 不调 `reveal`（晚到的行） | reveal 六格 |
| s8 | 不记 `revealed` | 同一次导航再挂上格、同一页换 hash 格 |
| s9 | `watchReader` 不看 `within` | `readers-input.test.ts` 四格、主区里的滚轮格 |
| s10 | 每一行都登记（不看 `active`） | reveal 七格 |
| s11 | `show` 的清理不撤下 | 页 → 首页格 |
| s12 | 上滚的符号反了 | 三次以上导航格 |
| s13 | 下面不留 8 px | 首格（`scrollTop` 1628） |
| s14 | "目录""历史"两行不带 `opened` | `arrival.test.tsx` 两格 |
| s15 | 普通页没有 `File` 图标 | 种类图标格 |
| s16 | 阅读视图锚点的等待不结束（`view-anchors.ts`） | `reading-view.test.tsx` 改写的那一格 |
| s17 | `key` 在 layout effect 里设（不在 insertion effect 里） | 活下来：jsdom 里等价，造不出被打断的渲染（同决定 798 的理由） |

审查者另写的：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| q17 | A | 只按 pathname 判定导航（layout effect 的依赖去掉 `location.key`） | 抓住（树读到之前换 hash 格） |
| q24 | A | = b1 | 第一轮活下来；`3ca04574` 之后两格红 |
| q27 | A | `SlotHolder` 还焦点时不看 `activeElement === body` | 活下来，等价：`append` 移动节点时焦点先落到 `body`，同一次提交里守卫恒为真；只有容器在另一次提交才被收养、读者其间把焦点放到别处时才起作用，今天没有这样的路径 |
| q35 | A | 行登记之前就设 `revealed` | 抓住（reveal 六格） |
| q05 | A | = b2 | 第一轮活下来；`3ca04574` 之后 StrictMode 格红 |
| a7 | C | 备用位置不看 `byId(current) === undefined` | 抓住（`page-edit.test.tsx` 两格） |
| b1 | C | `if (watching.reader === undefined)`：一棵树一生一个看守，不按导航换新 | 第一轮活下来；`3ca04574` 之后 `:170:44`、`:194:44` 红 |
| b2 | C | 卸下时不清空看守（去掉 `watching.reader = undefined`） | 第一轮活下来；`3ca04574` 之后 `:213:24` 红 |
| b6 | C | 滚进来之后看守不结束 | 抓住（`reading-view.test.tsx` 改写的那一格） |
| b9 | C | 行在 `useEffect` 里登记 | 抓住（三次以上导航格、页 → 首页格） |
| r1 | C | 锚点的等待什么也不听（`view-anchors.ts`） | 抓住（同 b6 那一格） |

对照（只改测试，或测试加变异）：

| # | 谁 | 改的 | 结果 |
|---|---|---|---|
| h0 | C | "首页、按键、开页"格的按键改在滚动区里 | 过 |
| h1 | C | h0 加 b1 | 红 |
| h2 | C | 按键在 body 上，加 b1 | 过（第一轮，S2C-M1；修复核对时在修复之后的树上再跑，同样） |
| h3 | C | 修复之后按回 body，树不改 | 过 |
| p0、b2p | C | 试写的 StrictMode 格；它加 b2 | 过；红 |
| c1 | C | 修复之后的 StrictMode 格去掉 `configure`，加 b2 | 过：咬住 b2 靠的是 StrictMode |
| c3 | C | 修复之后的滚轮格去掉第二次导航，加 b1 | 过：抓 b1 的是第二次导航 |

## 实量

都从侧栏的左缘量起；1440 宽下三种引擎、亮暗相同。

| 项 | 修复之前（实施者与 B，`831e50a6`） | 修复之后（实施者，`75cac02a`；B 的 spec 加深度与强制色的探针） |
|---|---|---|
| 笔记本行（当前与别的）的 ⌄、书本、名字 | 14 / 33 / 58（同样稿） | 不变 |
| 写者的齿轮 | 盒子 187–211 | 盒子 189–213，图标 193–209 |
| ＋ | 盒子 213–237，图标约 217–233 | 盒子 215–239，图标 219–235 |
| 读者的齿轮（行 `pr-8`） | 盒子 213–237 | 盒子 215–239 |
| 树行的 ⋯ | 盒子 215–239，图标 219–235（同样稿 `.end`） | 不变 |
| 父页的 › | 深度 0：盒子 16–34，› 18–32 | 深度 0：盒子 12–30，› 14–28；深度 1：› 30–44 |
| 页的图标 / 名字 | 深度 0：38 / 63（行内 30 / 55，同样稿） | 深度 0 / 1 / 2：38 / 63、54 / 79、70 / 95，叶子与父页相同 |
| "目录""历史"两行、来源根、`AGENTS` | 38 / 63 | 不变 |
| "更多" | 链接 28 px，在 32 / 62 / 92 / 122；图标 16 px，第一个在 38 | 不变 |
| 落点线 | 从 38 起，2 px | 不变 |
| 齿轮与＋的不透明度 | 静止 0.4；悬停行、＋、齿轮时 1；悬停树的行时 0.4；375 宽与触屏时 1；强制色下静止 0.4 | 同左，只有强制色下静止时 1 |
| 树里失败的提示 | 与上下各隔 4 px | 不变 |
| 60 页的树 | 当前行离滚动区底边 7 px（量的是链接，比行矮 1.75 px），窗口不滚 | 不变 |

当前行的底：亮 rgba(197,138,34,.12)、暗 rgba(217,168,78,.14)，图标 `--accent`（亮 #c2871f、暗 #d9a84e），同样稿；悬停＋时行的底是 `--hover`、字是前景色，在笔记本首页上照旧是 `--active`。

## PG16 的时间（7.5）

| 跑 | 第一格（Chromium / Firefox / WebKit） | 第二格 | 负载 |
|---|---|---|---|
| 实施者，提交 2 的第一版 `e89e5b9d`（第二格还没有 375 宽一段） | 0.65 / 2.1 / 0.98 s | 1.1 / 1.9 / 1.6 s | 17 → 30 |
| 实施者，`831e50a6` 合并之前的整条命令 | 0.72 / 1.7 / 0.96 s | 1.8 / 2.3 / 2.2 s | 12.6 → 37.5 |
| 审查 C，`831e50a6` 合并之前的整条命令 | 0.65 / 1.9 / 0.89 s | 1.8 / 2.3 / 2.2 s | 8–73（C 这一轮的范围） |
| 实施者，`75cac02a` 合并之前的整条命令 | 0.66 / 2.2 / 1.0 s | 1.6 / 3.5 / 2.7 s | 11.1 → 36.0 |
| 审查 C，`75cac02a` 合并之前的整条命令 | 0.66 / 2.1 / 1.0 s | 1.5 / 2.5 / 2.0 s | 从 48 起 |

六次合计约 9.7 test-seconds（C 在 `831e50a6` 上那一次），远低于 7.5 每次 4–6 秒的推断；E2E 多了 6 格，墙钟没变（2.2–2.4 min）。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库。

- **实施者**（7.4 的 S2，Chromium，亮暗 × 1440、375）：一本笔记本的一页（笔记本行、悬停行、悬停＋、悬停树的行、行菜单、树里失败的提示），一本 LLM Wiki 的目录与历史，读者的笔记本行，60 页的树到最后一页；实量见上表。与样稿没有看出不同。修复之后照 B 的 spec 在三种引擎里重截（1440 亮暗；Chromium 另有 375、读者、触屏与强制色），另写一个探针看深度 0–2 的父页与叶子、强制色下静止时的齿轮与＋。
- **审查 B**：Chromium 亮暗 × 1440、375，Firefox、WebKit 亮暗 × 1440，读者、触屏（CDP 模拟，宽 1024）、强制色；样稿在 1440 下同样量。另核对：焦点环（齿轮、＋、⋯、展开按钮、行与树的链接、"更多"、别的笔记本），侧栏上琥珀环亮 2.84、暗 8.41；行菜单在按钮下、右端对齐；`design-tokens.test.ts` 与 `cn.test.ts` 经 `heavy.sh` 全过，新类名都在 Tailwind 的标准刻度上。
- **审查 A**（行为，Playwright 探针，三种引擎）：换组（焦点在行按钮上、行菜单开着、改名的对话框开着，各跨过一次重读与两次换组）、别的笔记本的链接（决定 777）、地址里大写或未知的 id、换工作区之后后退、打开时在别处被删、"目录""历史"两行（决定 796）、当前行滚进侧栏（决定 779、791：每次导航至多一次，换 hash、后退、点视野里的行都不再滚，只改滚动区的 `scrollTop`，上下留 8 px）。
- **主持者**：修复核对时看了 `depths-light.png` 与 `chromium-04-tree-hover-side-1440-dark.png`。

## 门禁与 CI

| 提交 | 实施者 | 审查 C |
|---|---|---|
| `485e6be3` | `heavy.sh make lint-web knip test-web` 绿（212 个文件、5169 格）；`CI=1 make e2e` 在第一版 `c402ac5b` 的树上 594 过、3 败、1 跳，2.2 min，负载 11 → 37；amend 只给大写 id 一格加断言（s5），之后 `lint-web knip test-web` 再跑、绿 | 绿，212 个文件、5169 格；e2e 594 / 3 / 1，2.3 min |
| `831e50a6` | 第一版 `e89e5b9d`：绿（214 个文件、5189 格），e2e 600 / 3 / 1，2.3 min，负载 17 → 30；amend（PG16 第二格加 375 宽一段）之后 PG16 在三种浏览器里单独跑过，整套由合并之前的整条命令跑（下表） | 绿，214 个文件、5189 格；合并之前的整条命令（下表） |
| `3ca04574` | 绿（214 个文件、5190 格） | 绿：四个 `check.mjs`、零警告的 oxlint、oxfmt、tsc、knip，214 个文件、5190 格 |
| `75cac02a` | 绿（5190 格）；它是头，e2e 由合并之前的整条命令跑 | 合并之前的整条命令（下表） |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `831e50a6`（实施者；C） | 实施者：214 个文件、5189 格；e2e 600 过、3 败、1 跳（基线三格，同行同消息），Playwright 2.3 min，负载 12.6 → 37.5。C：全过，e2e 600 / 3 / 1，2.3 min，整条 3 分 23 秒，跑完树干净 |
| `75cac02a`（实施者；C） | 实施者：214 个文件、5190 格；e2e 600 / 3 / 1（基线三格，同行同消息），2.4 min，负载 11.1 → 36.0。C：静态检查、`gen-check-web`、`build-web` 过，5190 格；e2e 600 / 3 / 1，没有 flaky，2.3 min，开始时负载 48，跑完树干净 |

- 每次都只败基线里同行同消息的格（SE2 `se2:231:23`、PG15 `pg15:113:5`、LW8 `lw8:836:29`）。
- Go 一侧：`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`git diff --check` 干净（C 另对 `831e50a6..75cac02a`）；分支是快进。
- vitest：5157（S1 之后）→ 5169（提交 1）→ 5189（提交 2）→ 5190（修复）。
- **没有重现的 CI 项**：照 [01-P1](../01-P1-foundation.md) 7.3 的表（Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别）。
- **CI**：本机没有推送的凭据（决定 746、761）：负责人推送之后在最后一个头上补跑。

## 接受与推后

- 从带 `arrived` 的链接到达某页之后重载，或对同一地址再 `goto`：Chromium、WebKit 留着历史项里的 `arrived`，标题重新取焦点（Firefox 丢掉 `history.state`）。M3、M12 既有（`app/arrival.ts` S2 没改）；宽屏下无害（`scrollY` 都是 0，`h1` 在顶上）；S2 的窄屏里侧栏在页之上，才看得出窗口跟着滚到标题，S3 的抽屉之后不再如此（审查 A、C）。不改。
- 二合一设备（`any-pointer: coarse`、主指针能悬停）：树行的 ⋯ 100%（`any-pointer-coarse`），齿轮与＋仍 0.4（按 `hover: none`、`max-md`），看得见、点得到。只有 B 的推理，没有实测。
- s17、q27：等价。
- S3：PG16 第二格的 375 宽一段到了抽屉里仍成立，但树关在抽屉里时它不再抓变异 24，S3 的提交 2 改在开着的抽屉里核对（02-P2 6.3）；375 宽时长树把页推到很下，由抽屉解决。
- 没有实测的：未保存编辑时的备用位置只有 vitest（A 读了代码）；晚到的行（分批读到的树）由 `show` 补调 `reveal` 只有 vitest；StrictMode 只在 vitest 里（浏览器里没跑开发构建）；侧栏里拖拽与滚动区一起用时的 reveal、3000 页分批画的树里后来的导航上看守的效果（C 只从 b1 推断）；Linux 上的引擎（B）。
