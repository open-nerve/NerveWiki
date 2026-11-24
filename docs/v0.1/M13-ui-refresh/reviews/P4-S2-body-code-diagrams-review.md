# M13/P4/S2 正文的样式、代码块的头与图：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p4-s2`（从 `m13-ui-refresh` 的 `4f5552f2` 拉出）：`96346ed3`（照 6.2 提交 1：`reading.css` 最外层的 `:root`/`.dark` 各 21 个变量，正文的字、标题接标题、链接、没建的页、标签、高亮、任务项、行内代码、`pre`、表格、callout 的七族与 chevron、`.nw-diagram`；mermaid 的 `base` 主题、两份 `themeVariables` 与 `themeCSS`；`diagrams.test.ts`、`design-tokens.test.ts`；4 个文件）、`1c170cba`（照 6.2 提交 2：`codeBlocks` 与它的注册、`patch.ts` 的"复制"键、三个文案键、4.2.7 的头；`code-blocks.test.ts`、`patch.test.ts`、`reading-view.test.tsx`，PG17 第 2 格、pg5 的整块选中复制、`REQUIRED` 两行；12 个文件）、`cdedff02`（照实加：图里的 `foreignObject` 在强制色下也不强制，L4 的页面格加强制色一步；2 个文件）、`b24c98df`（第一轮的修复：决定 901，PG17 第 2 格的标题一步；2 个文件）、`8dfb516b`（决定 902、903，间距一步；2 个文件）、`36eb0c5d`（决定 904：`Drawer.parse`，图自己指定的主题只照它画；3 个文件）、`bbd5b46e`（时间线的箭头头，"复制"与 callout 的图标照样稿的大小；1 个文件）、`0e35d0dc`（`undone`、`only-child`，`reading-view.test.tsx` 的 `afterEach`；3 个文件）、`d43b3e6f`（修复之后实施者交来、主持者定的：`flowchart.padding` 9，改过的 904；2 个文件）、`9685e300`（同上：已完成项下面的块不划、不淡；2 个文件）、`02cb019e`（同上：已完成项里 callout 的标签一句；1 个文件）、`5bbf897f`（修复核对第 1 轮之后：决定 906，没有指令与 frontmatter 时不解析，文字夹着 `code` 的 `pre` 不加头；4 个文件）、`ade3abd1`（同上：903 的 `:is()` 去掉 `table` 与属性条一步，已完成项下面的引用与折叠的 callout，标题一步的行数取上限；2 个文件）；设计 [04-P4](../04-P4-reading.md) 4.2、5.2、6.2、7.1、7.2 的 S2 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的分离工作树里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（读 diff 与设计、`make build`；vitest 探针以真的 mermaid 11.17.2 读指令与 frontmatter 里的主题、看清理之后晚到的答复与几个 `code` 的 `pre`；静态页探针在三种引擎里量 callout 标题的两种写法；对着应用的 Playwright 探针看剪贴板的有无与拒绝、键盘、就地更新时"复制"的焦点、作者自己的主题；修复核对时在三种引擎里看已完成项下面的块的行为，量 `parse` 的代价）；B 管外观（临时故事在三种引擎里截图、量盒子，对照 `mockups/reading.html`：亮暗、中英，宽 1440、1024、375、320，强制色（Chromium、Firefox）；14 种 mermaid 在两种强制色组合下逐元素比强制前后的颜色；12 种语言的 hljs；修复核对时加已完成项与它的孪生项、自带颜色的图、属性条里的列表值）；C 管测试、变异、门禁与完成度（独立跑 `cdedff02`、`02cb019e` 上合并之前的整条命令，提交 1、2 逐个跑 `make lint-web knip test-web`，自写改法重做第一轮 30 条、修复核对 15 条变异，另写 c1；以 `apply82.py`、`apply82-proposed.py` 重跑 8.2 第 12–26 条的替换核对，列出文档提交要带的）。重的运行都经 `heavy.sh` |
| 结论 | 实施中：7.4 的强制色截图看到 Chromium 里图的 HTML 标签照旧被强制成系统色，照实加 `cdedff02`；设计 4.2.5 的 callout 标题 `display: flex` 拆开标题里的标记，没改，交给审查。第一轮：A 2 Important、2 Nit、1 Q；B 1 Important、3 Nit、2 Q；C 3 Minor、3 Nit、1 Q。A 的 I1 与 B 的 I1 是同一件（实施者交来的第 1 条）。主持者的决定 901–905 答了 A 的 I1（B 的 I1）、I2、Q1 与 B 的 Q1、Q2；其余接受，C 的各条都只是文档。修复在 `b24c98df`、`8dfb516b`、`36eb0c5d`、`bbd5b46e`、`0e35d0dc`；修复之后实施者交来七条，主持者定了（`d43b3e6f`、`9685e300`、`02cb019e`，904 随之改写）。修复核对第 1 轮：A 1 Minor（只给变量的图在暗色应用里边与箭头看不见）、2 Nit；B 1 Minor（903 落进属性条）；C 2 Minor（已完成项下面的引用与折叠的 callout 没有格守；8.2 第 13、16、17、18 条）；决定 906；修在 `5bbf897f`、`ade3abd1`，只动文档的随本文档提交。修复核对第 2 轮：A、B 没有新发现；C 5 Nit（三处文档的旧值与漏写、8.2 第 16 条的一处标点，随本文档提交；`code-blocks.ts` 一行注释过长，推后），没有 Important、Minor，可以合并。两轮核对都没有过程的事故。合并 `<待填>` |

## 基线

S2 不另跑基线（决定 883）：S1 合并之后 `m13-ui-refresh` 是 `4f5552f2`，第一个改行为的提交（`96346ed3`）的 `make e2e` 兼作核对。对照的数是 S1 之后的：vitest 224 个文件、5379 格；E2E 637 格。已知的本机失败同 [S1 的记录](P4-S1-head-strip-panel-review.md)：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S2 的每次 E2E 里 SE2、LW8 都红，同行同消息；PG15 有时过（实施者到 `5bbf897f` 为止的十五次里过了 3 次，审查 C 的两次里过了 1 次）。另有的红都查清了，见"实施中的发现"。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 7.4 的强制色截图（Chromium，暗色高对比加亮色应用）：照 4.2.5 只给 `.nw-diagram` `forced-color-adjust: none` 时，图的底与节点照应用画，流程图、类图、状态图、思维导图等的 HTML 节点名却被强制成系统色，白字画在亮的节点上；亮色高对比加暗色应用时黑字在暗的节点上。Chromium 的 UA 样式在强制色下给 `svg` `preserve-parent-color`、给 `foreignObject` `auto`；Firefox 改前就看得见，WebKit 模拟不了 | 照实加 `cdedff02`：图里的 `foreignObject` 也 `none`，L4 (page) 第一格加强制色一步，先红在 `l4:205`（`rgb(255, 255, 255)`，应为 `rgb(232, 232, 234)`）；审查 A 接受（偏离 1），B 逐元素核对 |
| callout 标题照设计 4.2.5 `display: flex`，标题里的加粗、链接、行内代码各成弹性项，窄时排成几列 | 改的是设计写明的布局，没做，交给审查（报告第 7 节第 1 条）；成了 P4S2A-I1、P4S2B-I1，决定 901 |
| flex 写法下长标题把 `::before` 的图标压到 0 宽 | 加 `flex: none`（偏离 2）；决定 901 之后随之去掉 |
| 提交之前没过的门禁：提交 2 的 tsc（`ByRoleOptions` 没有 `exact`）与 oxlint（`consistent-function-scoping`、PG17 的 `no-shadow`）；`8dfb516b` 的 `consistent-function-scoping`；`36eb0c5d` 的 oxfmt；`d43b3e6f`、`9685e300` 的 `no-shadow`；`5bbf897f` 的 tsc（`textContent` 可能是 `null`）与 `no-await-in-loop` | 改了才提交 |
| `36eb0c5d` 的 `make e2e` 基线之外三格：pg15:136 [webkit]、lw4:592 [firefox] 造数答 500，a15:197 [webkit] 标题 10 s 内没出现 | 三个文件在 Firefox、WebKit 重跑，54 过，只败基线的 pg15:113 |
| `bbd5b46e`、`d43b3e6f` 的 `make e2e` 里 as6:167 [chromium]"Response has been disposed" | 已知的偶发，重跑 3 过 |
| `02cb019e` 的 `make e2e` 里 lw13:112、lw12:126 [webkit]：造数与降级的 API 在负载约 33 下答 500；同一提交上合并之前的整条命令里 lw4:349、lw8:692 [firefox] | 单独重跑都绿 |
| `ade3abd1` 的实施 | 造数里相邻的引用与 callout 会并成一段引用：引用与上面的 callout 之间隔一个代码块，与折叠的 callout 之间隔嵌套的列表；属性条里也有 `li`，已完成项改取 `:scope > ul > li` 的第一个。提交之前在工作树上照 c1（t41）与 t14 各改坏一次、三种浏览器看它红，复原之后提交。`make e2e` 另红 as6:167 [chromium]（已知的偶发，重跑 3 过） |

## 第一轮的发现与处置

在 `cdedff02` 上（对照 `4f5552f2`）。

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S2A-I1 | Important | callout 标题的 `display: flex`（`W/reading/reading.css:576-614`，设计 4.2.5 的写法）把标题里每段文字与每个行内元素拆成弹性项：1440 宽"**bold**, a"的逗号前多 7 px；375 宽一个带加粗、链接、行内代码的长标题排成并排的几列、高 167 px（修法 71 px）；`innerText` 与选中复制断成几行（`"Title\nc1"`），三种引擎相同。服务端自己的测试就有标题里一个 wiki 链接的形状 | 决定 901（`b24c98df`）：标题回到块，16 px 的图标在行内，`summary` 留出右内边距、chevron 绝对定位在第一行的右端，行高 1.5；PG17 第 2 格加一步（标题的 `innerText` 连续、行数的上限、375 宽不横滚），三种浏览器先红 |
| P4S2A-I2 | Important | 图自己指定了主题（`%%{init: {"theme": …}}%%` 或 frontmatter 的 `config.theme`）时，mermaid 把 `initialize` 给的应用的 `themeVariables` 交给作者的主题去推导（`updateCurrentConfig`），节点的字读不出：亮色应用里作者的 `dark` 1.08:1，暗色应用里 `forest`、`neutral` 1.13、1.05:1；S2 之前读得清。871 的"与今天能改 `theme` 一样"不成立 | 决定 904（`36eb0c5d`）：`Drawer.parse`，`tooLarge` 之后解析，作者指定的不是 `base` 的主题只照它画；不把 `theme` 放进 `secure`（A 的探针看到那样弄坏作者最常用的 `base` 加变量）；`diagrams.test.ts` 两格先红；871 照实改（本文档提交）。修复之后照实施者第 7 节第 3 条改写（`d43b3e6f`），修复核对第 1 轮之后照决定 906 再改（`5bbf897f`） |
| P4S2A-N1 | Nit | 清理时还在等的 `writeText` 晚到，在脱离文档的节点上写字、再挂一个 1.5 s 的计时（`W/reading/code-blocks.ts:34-48`） | 各按钮共用的 `undone`，清理时置真，`told` 开头看它；一格断言计时数为 0，先红（`0e35d0dc`） |
| P4S2A-N2 | Nit | 作者手写的 `<pre>` 里几个 `<code>` 各加一个头，头倒序堆在 `pre` 的开头，"复制"只复制它那个 `code`（`code-blocks.ts:82-96`） | 选择器 `pre > code:only-child`，一格先红（`0e35d0dc`）；文字夹着 `code` 的一半见 P4S2A-F1-N2 |
| P4S2A-Q1 | Q | 同一个顶层块里有几段代码时，在拿着焦点的那段之前增删一段，就地更新之后焦点落到别一段的"复制"上，三种引擎相同（`patch.ts:381-383` 的 `keyOf` 靠 `nth`） | 决定 905：照实，记进 04-P4 10.2 与第 9 节（本文档提交） |
| 偏离 1–4 | — | `cdedff02`；`::before` 的 `flex: none`；`code-blocks.test.ts` 第 8 格；7.1 的"只写 `:root` 时也红"由 t13 看到 | 接受；`flex: none` 随决定 901 去掉 |
| 给审查者的七条 | — | 实施者报告第 7 节 | A 坐实第 1 条（I1）；第 2、3 条交 B；第 4–6 条照设计；第 7 条归 C |

A 另核实：剪贴板的有无与拒绝（Chromium headless 拒绝时"没能复制"看得见，Firefox、WebKit 凭点击照样复制）；连按、1.5 s、键盘（Space 之后焦点仍在按钮上，焦点圈在 34 px 的头里画得下）；读屏的一句（`said` 一直在、先清空再写，不重复播报）；整个视图选中复制，三种引擎的剪贴板里都没有头的字；`taskText`、`patch.ts` 的 `text()`、`blockKey`、`restamp` 都不看头；就地更新时拿着焦点的"复制"在改它之上、改它自己、改它之下、在它之前插入顶层块时都回到它那一段；注册次序与倒序的清理、StrictMode 不留两个头；每次画都重新 `initialize`，暗→亮不留 `darkMode`；打印不印头。

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S2B-I1 | Important | 同 P4S2A-I1；另：长词与地址溢出 callout，阅读视图因此横滚；320 宽英文标题高 214.2 px、排成七列，中文一列一个字；单行标题高 23.8 px（行高继承 1.7，样稿 21） | 同 P4S2A-I1（决定 901），标题 `line-height: 1.5`，单行 21 px |
| P4S2B-N1 | Nit | 时间线的箭头头是正文色（`marker` 里的 `path` 没有自己的 `fill`），线是 `themeCSS` 给的灰（`W/reading/diagrams.ts:191`） | `reading.css` 加 `marker > path { fill: var(--faint) }`（`bbd5b46e`；B 试过写在 `themeCSS` 里不生效）；旅程图的主线是 mermaid 写死的黑，照旧 |
| P4S2B-N2 | Nit | "复制"的图标 13 px、`gap` 5 px、字重 400，callout 标题的图标 14 px；样稿 16 px、6 px、500、16 px，设计没记成偏离 | 照样稿（主持者定：样子为主；`bbd5b46e`、`b24c98df`），4.2.5、4.2.7 的值照实改（本文档提交） |
| P4S2B-N3 | Nit | 流程图的节点比样稿大一圈（"浏览器"节点 100.5×50.3 px，样稿 74.5×40.25），图框高 136.6 px（样稿 94.25）：mermaid 默认的 `padding` 15 | 实施者试了 8–15，主持者定 `flowchart: { padding: 9 }`（`d43b3e6f`）：节点 76.49×38.24 px，图框 129.3 px 照实记成偏离 |
| P4S2B-Q1 | Q | callout 里块与块之间只有 4 px（`.nw-callout > * + *`，S2 之前 0.5rem），两段正文像一段多了行距 | 决定 902（`8dfb516b`） |
| P4S2B-Q2 | Q | 列表项里的块之间没有间距（S2 之前就这样）；S2 给代码块加了边框与头，贴着上一行更显眼 | 决定 903（`8dfb516b`）；它落进属性条的一处见 P4S2B-F1-M1 |
| 给审查者的七条 | — | 实施者报告第 7 节 | 第 1、2 条并入 I1；其余照设计记下 |

B 另核实：正文对样稿（段落 16 / 28.8 px，段到 h2 38 px、h2 到表格 14 px，表格各行 36 / 42.75 px，与样稿相同；h2 的行高 1.35 比样稿矮 3.2 px，是设计定的）；暗色整体与样稿暗色一致；任务项（已完成的字淡并划线，嵌套的没完成的项回到正文色）；七族 callout 亮暗的标题色对自己的底（亮 3.68–5.19、暗 5.66–8.04）；代码块的头 34 px、横滚时贴左、320 宽"没能复制"放得下、强制色下系统色；12 种语言的 hljs 与对比度；14 种 mermaid 亮暗；`cdedff02` 之后 Chromium 两种强制色组合下 14 种图每个元素的颜色与不强制时相同；新规则的特指度与 `reading.css` 的色值只在最外层的 `:root`/`.dark` 里。

### 审查 C（测试、变异、门禁与完成度）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S2C-M1 | Minor | `cdedff02` 让 4.2.5（`foreignObject` 一条）、5.2（L4 不改与它的三个行号）、6.2、7.1、7.2（46 一行、t 行）照实要改，报告没列；3.5 要的"照实改的各节"不在 10.2 的草稿里 | 照实改，各节列进 04-P4 10.2 与 4.3 之前的"照实"（本文档提交；之后各轮的修复一并） |
| P4S2C-M2 | Minor | M12-polish 与 01-P1 第 9 节的两行没起草，"头不添闪动"没核对。C 读代码核对了：layout effect 在同一次同步执行里 `was.undo()`、`patchBlocks`、`enhance(...)`（`W/pages/page/server-html.ts:88-130`），`codeBlocks` 同步，没有没有头的一帧 | 两行照 C 的草稿（本文档提交）；M12-polish 一行另记决定 905 与作者的浅色主题 |
| P4S2C-M3 | Minor | 8.2 第 13 条的新文字读起来像总有一个 `aria-live` 的说明；说明只在有剪贴板时随"复制"建，两者都没有时没有头 | 照 C 改（本文档提交），修复核对第 1 轮之后再照 P4S2C-F1-M2 |
| P4S2C-N1 | Nit | 8.2 第 16 条、`reading.css:12`、`design-tokens.test.ts:236` 与 `96346ed3` 的说明说"编辑器也借用这些变量"，S2 时还不成立 | 第 16 条写"（P4/S3 起也给编辑器）"；两处注释到 S3 一起看（04-P4 10.2） |
| P4S2C-N2 | Nit | 报告与 10.2 草稿几处数或说法不准：t5 是 8 格，不是 6；"pg5 的第一格"是 PG5 (page)（`pg5:88`）；e2e 的是 41e；应写"S2 的 t1–t13"；第 8 格在桩上红是推得，没有日志；Firefox 2.5–2.7 s | 10.2 照改（本文档提交）；修复之后 t5 是 10 格 |
| P4S2C-N3 | Nit | 52 的"等价"没写依据（05-lessons 第 1 节） | 7.2 的 52 一行写出 C 给的依据与不再等价的条件（本文档提交） |
| P4S2C-Q1 | Q | L4 的强制色一步只跑暗色高对比加暗色应用（L4 经 `emulateMedia({ colorScheme: "dark" })` 进暗色）；改前真正看不见的是暗色高对比加亮色应用 | 主持者定：够了（比的是强制前后不变，t11、t12 照红），照实记进 5.2 |
| C 未核实 | — | `reading-view.test.tsx` 里 `userEvent.setup()` 装的剪贴板替身留到文件结束（没开 vitest 的 globals，user-event 不登记拆除），潜在的次序依赖 | `0e35d0dc` 的 `afterEach` 拆掉它；临时探针改前红、改后绿，探针没有提交；t26 没有格抓 |

C 另核实：`cdedff02` 上合并之前的整条命令（下文门禁一节）与提交 1、2 各自的门禁；与基线比多 16 格（diagrams 1、design-tokens 1、code-blocks 8、patch 1、reading-view 2、messages 3）、E2E 多 3 格；受保护路径与 `git diff --check` 0；`REQUIRED` 两行的起头只对上第 2 格；三个提交的说明（前两个与 6.2 逐字相同，作用域与改到的目录一致）；先红的日志与行号，先红只停在 `:421`、`:176` 的那些步都有变异各自红过；L4 的 `matchMedia` 守卫在 Chromium 上为真；8.2 第 12–18 条在 `4f5552f2` 的总体设计上依次各一次、第 19–26 条在内存里接着也各一次（`apply82.py`，"ALL OK"）；抽的 30 条变异都与报告同格或照说法绿，差的几处是改法不同（41、49）或报告的格数不准（t5，N2）。

## 过程的事故

两轮都没有。实施者与三位审查者都只停自己起的进程，停之前以 `ps -o command= -p PID` 核对命令行。

## 修复之后实施者交来的（主持者定）

| 实施者修复报告第 7 节 | 处置与提交 |
|---|---|
| 1. P4S2B-N3：`flowchart.padding` 9 时节点 76.5×38.2 px（样稿 74.5×40.25），图框高 129.3 px（样稿 94.25：圆柱较高与 mermaid 自己围着图的边距）；8–15 都不裁标签 | 取 9（`d43b3e6f`）；图框的差别照实记成偏离（04-P4 10.2） |
| 2. 已完成的任务项下面的代码块被划线、变淡（`text-decoration` 传进流内的块；嵌套列表因 `inline-block` 不受） | 修（`9685e300`）：它的 `pre`、`blockquote`、`div`、`details` 也 `inline-block`、满宽、`vertical-align: top`，颜色设回自己的；三种引擎核对粘住的头、横滚、选中复制与孪生项相同；PG17 第 2 格加一步，先红；`02cb019e` 补 callout 里的标签一句（t35 之下红） |
| 3. 作者用 `base` 加自己的 `themeVariables` 时，暗色应用里应用的浅字画在作者的黄底上，1.24:1（S2 带来的合并） | 改决定 904（`d43b3e6f`）：图自己给了变量，或指定了不是 `base` 的主题，都只照作者的配置画；`diagrams.test.ts` 的 `base` 一格随之改，先红 |
| 4. 作者指定浅色主题时，暗色应用里直接画在图框底上的边、箭头、时序图的消息字看不清（M13 之前就这样） | 记给 v0.2（04-P4 第 9 节）：图框的底跟着作者的主题；P4 不做 |
| 5. 决定 903 让以代码块开头的列表项在代码块上方多 8 px | 照实记下（10.2） |
| 6. t26、t27 没有格抓 | 照实：t26 由临时探针、t27 由截图看 |
| 7. Chromium 里带行内代码的标题高 21.64 px；标题里的链接用正文色 | 照实记下（10.2） |

## 修复核对第 1 轮

在 `02cb019e` 上（对照 `cdedff02`），三位各在自己的分离工作树里（经 `heavy.sh`）。

- **A**：I1（决定 901）、I2（904 与改过的 904，节点的字都读得清）、N1、N2（几个 `code` 的一半）都已处理，Q1 照决定 905 没有要核的代码。901：三种引擎里聚焦 `summary` 之后 Enter、Space、点击开合照常（已完成项里那个 `inline-block` 的折叠 callout 也是），`innerText` 连续，WebKit 没有原生的三角，焦点圈围住整个 `summary`，chevron 在第一行的中间。904：对着应用量节点名对底，作者的 `dark` 10.17、frontmatter 的 `forest` 15.15，只写 `base` 照应用的颜色（亮 15.99、暗 12.35），只给变量 10.83、`base` 加变量 8.36；写坏的指令照应用的颜色，语法错显示源码，控制台没有输出；真的 mermaid 的 `parse` 认得指令、frontmatter 与 `%%{initialize:…}%%`，语法错与不认识的类型答 `false`、不抛、不写控制台。`undone`：清理之后马上重跑时，前一轮晚到的答复不碰新建的头。已完成项下面的块：复制、区域"代码"、粘住的头、折叠的开合、图的宽、划线与变淡、选区与页内查找，三种引擎与孪生项相同（只有 Chromium 的复制在换行前多一个空格）。四个相关的 vitest 文件 121 格全绿。
- **B**：I1、N1、N2、N3（照实记偏离）、Q1、Q2 都已处理（三种引擎、亮暗、1440 / 375 / 320）：标题连续、单行 21 px，375 宽英文 63 px、320 宽中文 42–43 px，长词与地址折行、视图 `scrollWidth` 等于宽；callout 高 76.5 px、标题 21 px、标题到正文 25 px，与样稿相同；时间线的箭头头亮 `#91939b`、暗 `#74767e`，与线同色；"复制"16 px、6 px、500，中文按钮 62 px（样稿 64，差在字体）；callout 里标题之后 4 px、块间 8 px，列表项里的代码块离上一行 8 px。已完成项与孪生项的位置、宽、高完全相同，块里的字、链接、标签照它们自己的颜色；自带颜色的图都只照作者的配置画，只写 `base` 的与其后不写配置的照应用的颜色、不串色；中文对照页里别的东西没动，14 种图改前改后除流程图与状态图外只有 ±1–2 px 的取整差，强制色下照旧同色；`design-tokens.test.ts` 7 格全绿，各条的特指度核对过。
- **C**：M1–M3、N1–N3 已处理、待文档提交（清单扩到这一轮的各节），Q1 主持者定了，未核实的剪贴板替身已处理（`0e35d0dc`）。新格读过：PG17 第 2 格的标题一步、间距一步与已完成项的块，`:scope > pre` 只取顶层那一块；`diagrams.test.ts` 904 的三格与第一格的 `padding`，替身的 `parse` 读指令的整段 JSON 与 frontmatter 的 `theme:`，太大的思维导图不解析，每张图都重新 `initialize`、画经 `drawingsOut` 串行，作者的配置漏不到下一张图；`code-blocks.test.ts` 两格；各有先红的日志。15 条变异与报告相符，C 自己加的 c1 没被抓住（F1-M1）。门禁见下文。8.2 第 12–18 条照旧干净（`apply82.py`），改过的文字由 `apply82-proposed.py` 核过：第 12–18 条依次各一次，第 19–26 条在内存里接着也各一次。

新的发现：

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P4S2A-F1-M1 | Minor | 改过的 904 把只给变量、没写主题的图也算作者自己的颜色，却不给主题，mermaid 取它的浅色 `default`：最常见的这种指令只改字号，暗色应用里节点读得清，边与箭头 `#333` 画在暗色的图框（`#1f2023`）上 1.29:1；M13 之前这种图在暗色应用里是 mermaid 的 `dark` | 决定 906（`5bbf897f`）：主题按应用的亮暗取 `dark` 或 `default`，再加作者的变量；`diagrams.test.ts` 那一格改成暗色时 `dark`，先红；实施者量了只改字号的与给了浅色 `primaryColor` 的两种（04-P4 2.3 的 906） |
| P4S2A-F1-N1 | Nit | 每张图多解析一遍：600 条消息的时序图 `parse` 35–67 ms、`render` 138–203 ms，约多三成；199 条边的流程图约多一成（负载 36–44 下量） | `5bbf897f`：源码里没有 `%%{`、开头（去掉空白）也不是 `---` 时不调 `parse`；一格断言这时 `parse` 没被调、空行之后的 frontmatter 照样解析，先红 |
| P4S2A-F1-N2 | Nit | `:only-child` 不看文字节点，`<pre>before <code>one</code> after</pre>` 照旧加头，"复制"只复制 `"one"` | `5bbf897f`：`alone(code)`，`pre` 里除了这个 `code` 只能有空白的文字；`code-blocks.test.ts` 一格（文字夹着的不加，空白夹着的加），先红 |
| P4S2B-F1-M1 | Minor | 903 的 `li > :is(…, table, …)` 选中了属性条里列表值是映射的那种（`td > ul > li > table.nw-props`），那一块小块高 32.75 px（同一行别的 24.75），字下沉；同一处列表的列表（`lists: [[a, b], [c]]`）的 4 px 是 S2 之前就有的 `li > :is(ul, ol)` | `ade3abd1`：`:is()` 里去掉 `table`（正文的表格在列表项里是 `div.nw-scroll`，照旧在 `div` 里）；PG17 第 2 格加属性条一步（映射的小块与列表的小块同高、嵌套表 `margin-top` 为 0），三种浏览器在带 `table` 的规则下先红；4 px 照实记一句（04-P4 10.2） |
| B 未核实 | — | WebKit 里一次"mermaid could not be loaded… Importing a module script failed"，单独重跑三次都绿；那一格在 mermaid 载入时 `reload` 过 | 05-lessons 记过的被下一个文档打断的 `import()`，照实记进 10.2，没有逐次排查 |
| P4S2C-F1-M1 | Minor | 已完成项下面的引用与折叠的 callout 没有格守：C 的变异 c1 把 `blockquote, details` 从 `inline-block` 一条去掉，PG17 第 2 格照绿 | `ade3abd1`：造数的 `shipped` 加一段引用与一个折叠的 callout，`blocks` 加上它们，期望 `inline-block`、满宽，引用是它自己的次要色；在 c1 之下先红（7.2 的 t41） |
| P4S2C-F1-M2 | Minor | 8.2 第 13、16、17、18 条是在 901–905、A-N1、A-N2 之前写的 | 照 C 的 `apply82-proposed.py` 的文字（本文档提交）：第 13 条另加文字夹着 `code` 的一种（A-F1-N2），第 16 条另照决定 906，第 17 条另加 C-F1-M1 补的两块与 B-F1-M1 的一步；"；`editor/theme.ts` 的链接色、搜索匹配色与语法色可以写色值；"一句逐字留着（S3 第 25 条的锚点）；13.2 第 23 条不补 |
| C 未核实 | — | PG17 第 6 步（之后的第 7 步）在 1440 宽断言第一个标题正好 2 行，Linux CI 的字体更宽时可能折成 3 行 | `ade3abd1`：改成上限（第一个不超过 2 行、第二个 1 行）；t14 照上限重跑（"变异"一节） |

## 修复核对第 2 轮

在 `ade3abd1` 上（对照 `02cb019e`），只看新的提交（`5bbf897f`、`ade3abd1`）。

- **A**：旧条目都已处理，没有新发现。906：只给变量、没写主题的图暗色应用里是 mermaid 的 `dark`（只改字号的那张：`#1f2020` 上 `#ccc` 10.17:1，边 `lightgrey` 对图框 10.88:1，字 20 px 生效；浅色 `primaryColor` 的同样），亮色应用里是 `default`（10.83:1，边 12.63:1），都不带应用的变量与 `themeCSS`（圆角 `auto`，不是 9 px）。同意 `base` 加作者变量在暗色应用里的边（1.21:1）交给 v0.2，与作者指定浅色主题同因同改法（同一批里作者的 `forest` 1.29:1、缩进的 frontmatter `neutral` 2.84:1 也是）。F1-N1：mermaid 11.17.2 只从指令（`directiveRegex` 要 `%{2}{`）与 frontmatter（`frontMatterRegex` 只许第一行行首的横向空白）读配置，`includes("%%{")` 加 `trimStart().startsWith("---")` 是它的超集，多出的只是开头的空行、代价是多解析一次，不漏；Chromium 里缩进的 frontmatter 照作者的 `neutral`，只有 `title` 的 frontmatter 与 `%% 注释` 照应用的颜色。核对了 C 的未核实一条：`mermaid.parse` 带着指令会改全局配置（下一张图第一步的 `initialize(base)` 之后仍是 `dark`），但 `render` 一开头的 `processAndSetConfigs` 先 `reset()` 再加它自己的指令，`diagrams.ts` 每张图的几步在同一个串行的任务里，中间读旧配置的只有 `tooLarge` 的 `detectType`（不读全局配置）；对着应用排了 11 张交错的图（亮→暗→亮三次载入，再在开着的页上换暗色），紧跟在自带配置的图之后的四张普通图都是应用的颜色（亮 15.99、暗 12.35，圆角 9 px），换主题各自照自己的重画，控制台没有输出。F1-N2：注释节点、文字夹着 `code`、两个 `code`、mermaid 不加头，空白夹着的与普通的加，清理之后重跑相同。PG17 的 `done` 改取 `:scope > ul > li` 是对的。四个 vitest 文件 100 格全绿。
- **B**：B-F1-M1 已处理：三种引擎里属性条里映射的小块 24.75 px，与同一行列表的小块同高，嵌套表 `margin-top` 0、内偏移 3 px、字上下居中；列表项里的 Markdown 表（`div.nw-scroll`）照旧 8 px。906 的图（只给变量、只改字号、浅色 `primaryColor`、时序图只改字号）两种应用主题里与 `4f5552f2`（M13 之前）逐项相同：亮色 `default`，边、箭头与边标签 `#333`；暗色 `dark`，边与箭头 `rgb(211, 211, 211)`、标签 `rgb(204, 204, 204)`，读得清；作者的 `primaryColor` 不生效（mermaid 只在 `base` 下用这些变量，M13 之前也这样）。自带主题与颜色的图与第 1 轮相同，紧接其后的普通图照应用的颜色。已完成项下面的引用与折叠的 callout 与孪生项的盒子逐个相同，引用不划、不淡、是它自己的次要色（亮 `rgb(95, 97, 104)`、暗 `rgb(164, 165, 172)`），折叠的 callout `inline-block`、696×76.5 px、`margin-top` 8 px、标题橙色不划。别处没动：样稿对照、标题与 callout、代码块、长 JSON 的量与 hljs 各色完全相同，图的截图高只差 ±1–4 px 的取整。修复版 24 格、基线 3 格都绿。没有新发现。
- **C**：可以合并。旧条目都已处理（C-M1–M3、N1–N3、Q1、F1-M1、F1-M2、第 1 轮未核实的标题行数）。实施者的先红与 C 自己的 t36–t43 都对得上（C 的 t38 只去掉 `trimStart`，比 7.2 的窄，两种都抓住）。门禁没有重跑：`fix/premerge-4.log` 的数与本记录相同（vitest 225 个文件、5402 格、58.30 s，E2E 636 / 3 / 1、2.6 min，只败基线三格，PG17 第 2 格 2.3 / 2.8 / 2.7 s），`fix/c13-e2e.log` 的 as6 一格与重跑 3 过也对得上；"整条 3 分 48 秒"日志里没有时间戳，照实施者的记录。文档脚本在 `git archive ade3abd1` 的副本上 `--check`、写入、再 `--check`、`check_links.py`（0 个新断链）、`git diff --check` 都过，S3 第 25 条的锚点逐字只一次，第 19–26 条在内存里重放都过；照第 1 轮的清单逐节读过；10.2 的数与 `p4s2/` 的日志、`shots-fix2/all14` 的量抽查相符。五条 Nit，见下表。

新的发现：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P4S2C-F2-N1 | Nit | 10.2 里 375 宽流程图的字"6.5"是 `d43b3e6f` 之前的值，padding 9 之后三种引擎是 7.3 | 本文档提交：换成"流程 7.3（`d43b3e6f` 的 padding 9 之后；之前 6.5）" |
| P4S2C-F2-N2 | Nit | 7.2 的 t21、t29 两行"守的决定"只写 904，它们抓的格里有 906 的"只有变量"一格；904 的条目读起来像只给变量的图也只照自己的配置画 | 本文档提交：两行写"决定 904、906"；904 的条目加"（只给变量、没写主题的，主题怎么取见 906）" |
| P4S2C-F2-N3 | Nit | 7.2 之后"不列的"一段，44 的说明还写着 `only-child` | 本文档提交：照 C 改成 `alone` 的说法 |
| P4S2C-F2-N4 | Nit | `code-blocks.ts:85` 的注释一行 146 列（`5bbf897f` 加的 `alone` 的说明），别的注释都在 120 列之内 | 推后：S2 的文档提交只改文档，下次碰这个文件时折行（04-P4 10.2 的"交给后面"） |
| P4S2C-F2-N5 | Nit | 8.2 第 16 条"不加这些，只给变量"是逗号连句 | 本文档提交：换成"不加这些；只给变量"（不在 S3 第 25 条的锚点里） |
| C 未核实 | — | 合并之前的整条命令"3 分 48 秒"在日志里没有时间戳佐证 | 照实施者的记录，10.2 写明出处 |
| C 未核实 | — | 暗色应用里只给变量的图，节点的底与图框的底几乎相同（对比约 1：mermaid 的 `dark` 不采用作者的 `primaryColor`，节点底取 `mainBkg`），字与边都读得清 | 视觉的判断归 B：B 第 2 轮看过读得清，与 M13 之前逐项相同；照实 |

## 主持者的决定（负责人可以改判）

全文在 [04-P4](../04-P4-reading.md) 2.3。

- **901 callout 的标题是一行文字**（审查 A 的 P4S2A-I1、B 的 P4S2B-I1）：标题回到块，行高 1.5，16 px 的图标在行内，chevron 绝对定位在第一行的右端；改设计 4.2.5 的 flex 写法，第 9 节读屏清单的一条随之改。
- **902 callout 里只有标题之后 4 px，块与块之间 0.5rem**（审查 B 的 P4S2B-Q1）。
- **903 列表项里的块之间 0.5rem**（审查 B 的 P4S2B-Q2）：松散列表的段、列表项里的代码块、引用、callout、图与表格的外层；嵌套列表的 0.25rem 不动；修复核对第 1 轮之后不含 `table`（P4S2B-F1-M1）。
- **904 图自己给了颜色时只照它自己的配置画**（审查 A 的 P4S2A-I2；修复之后照实施者第 7 节第 3 条改写）：指定了不是 `base` 的主题，或给了自己的 `themeVariables`；只写 `base` 或什么都没写时照应用的颜色。871 照实改。
- **905 焦点按"复制"在所在块里的第几个找回，照实**（审查 A 的 P4S2A-Q1）：同一块里在它之前增删代码时可能落到别一段上，记给 v0.2。
- **906 只给了变量、没写主题的图，主题按应用的亮暗取 mermaid 的 `dark` 或 `default`**（审查 A 的 P4S2A-F1-M1；改 904）。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份，跑完从备份复原，以 `git diff --quiet HEAD -- <文件>` 核对（02-workflow 3.1）；改坏期间不跑 `make gen*`、不用 `git checkout --`。实施者以 `p4s2/mut.py`（清单 `mutations-r0.py`、`mutations-r1.py`、`mutations.py`）跑 vitest 的各行，e2e 的各行（编号后加 `e`）经 `heavy.sh` 先 `make build`、再跑那个故事；结果在 `p4s2/mut/results.jsonl` 与逐条的日志。脚本与日志在会话的临时目录，不进仓库。

- **第一轮，`cdedff02` 上**：7.2 的 S2 各条（37–53、82–90、107 的五项）与实施者另写的 t1–t13。除 52（等价，依据在 7.2 它那一行，P4S2C-N3）都被抓住；46 的 l4 照设计不抓（`code-blocks.test.ts` 抓）；53 由 `check.mjs` 抓（firefox、webkit 两行）；t11、t12 由 L4 的强制色一步抓（`l4:205`、`l4:204`）；t5 只有 vitest 抓（8 格）。
- **修复之后，`0e35d0dc` 上**（58 条）：全部重跑；照修复改了写法的 46、48、49、89；新的 t14–t27。t26、t27 没有格抓（实施者第 7 节第 6 条）；t5 成了 10 格，44 是 2 格，48 是 4 格，49 是 5 格。
- **修复之后的第二批，`02cb019e` 上**：改到的 48、49、83–86、107 的五项、t19–t22（照 `ownColours` 改了写法）、50（照已完成项下面的块改了写法）、87、t14–t18、t27、40，与新的 t28–t35。除 t27 都红；t31、t33 三种浏览器都红（t33 的间距 17 对 8 px），t34 不满宽，t35 红在 callout 里的标签。
- **修复核对第 1 轮之后，`ade3abd1` 上**：照新代码改了写法的 10、18、21、23、30、46 与照行数上限重跑的 t14 都红（t14 三种浏览器红在文字）；新的 t36–t43 都被抓住（t40、t41、t43 三种浏览器）；顺带重跑的 37、38、41–44、47–49、83–87、89、90、107 的五项、t1–t7、t15–t17、t19、t20、t22、t24、t25、t28、t29、t31–t35 与 40、50 都照旧红，格数变了的：48 是 5 格（多了没有指令一格）、t5 是 11 格、t19、t22 各 2 格、t21 3 格。 新的 t36–t43 的抓手在 04-P4 7.2 表末。

审查者另写的：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| 38e | C | 复制 `pre` 的 `textContent` | PG17 第 2 格三种浏览器都红在第 3 步（实施者只跑了 Chromium） |
| 41、41e | C | 计时设成 `2 ** 31 - 1`（设了、永远不到） | `code-blocks.test.ts` 4 格红（实施者的改法 5 格，差的一格是改法不同）；PG17 红在第 3 步 |
| 49 | C | 两种主题都取 `themeVariables("dark")` | `diagrams.test.ts` 3 格红（实施者以 `"light"` 4 格："drawn once it shows"一格在暗色里，相符） |
| t5 | C | 头 `append` | `code-blocks.test.ts` 8 格红（报告写 6，P4S2C-N2）；PG17 绿 |
| t13 | C | `.dark` 块放回 `@layer components` | `design-tokens.test.ts` 成对一格红 |
| c1 | C | 已完成项下面的块去掉 `blockquote, details` | PG17 第 2 格绿（P4S2C-F1-M1）；`ade3abd1` 之后是 7.2 的 t41 |
| 抽样重跑 | C | 第一轮 30 条（e2e 14 条）；修复核对第 1 轮的 t14、t15、t17、t19、t20、t21、t23、t24、t29、t30、t31、t33、t35、44、48 | 都与实施者的结果同格，等价的 52、52e 照说法活下来 |
| `secure` | A | 把 `theme` 放进 `secure`（作者的主题一律不认，不建议的"最小"改法） | 作者的 `base` 加 `primaryColor` 不再重新推导，节点的底照旧（P4S2A-I2，没有采用） |
| 标题的写法 | A、B | callout 标题 flex 与块两种写法的静态页与临时故事 | 三种引擎里量出 I1 与修法（`callouts.json`、`titles.json`、`long.json`） |
| `margin: 0` | B | 属性条里映射的嵌套表注入 `margin-top: 0` | 小块回到 24.75 px（P4S2B-F1-M1） |

对照（只改测试，或测试加变异）：实施者的剪贴板替身探针（`reading-view.test.tsx` 里一格依赖替身被拆，改前红、改后绿，`p4s2/fix/red-stub-vitest.log`）；A 的 vitest 探针（真的 mermaid 的 `parse`、`only-child` 与 `undone`）。

没有格守的：t26（临时探针看）、t27（chevron 的位置，截图看）。等价的：52、52e。

## 门禁与 CI

| 提交 | `heavy.sh make lint-web knip test-web` | `CI=1 heavy.sh make e2e`（过 / 败 / 跳） |
|---|---|---|
| `96346ed3` | 绿，224 个文件、5381 格，71.2 s | 633 / 3 / 1（基线三格），2.7 min，负载 19 → 28 |
| `1c170cba` | 第一次 tsc 与 oxlint 没过（实施中的发现）；绿，225 / 5395，65.1 s | 636 / 3 / 1，2.7 min，16 → 26 |
| `cdedff02` | 绿，225 / 5395，62 s | 637 / 2 / 1（pg15 过了），2.6 min，20.8 → 32.1 |
| `b24c98df` | 绿，5395，64 s | 637 / 2 / 1（pg15 过了），2.6 min，41.2 → 29.7 |
| `8dfb516b` | 第一次 oxlint 没过；绿，5395，64 s | 636 / 3 / 1，2.6 min，42.0 → 34.9 |
| `36eb0c5d` | 第一次 oxfmt 没过；绿，5397，63 s | 634 / 5 / 1（另三格见实施中的发现），2.8 min，39.7 → 21.1 |
| `bbd5b46e` | 绿，5397，62 s | 635 / 4 / 1（as6:167），2.6 min，14.5 → 18.8 |
| `0e35d0dc` | 绿，5399，63 s | 637 / 2 / 1（pg15 过了），2.7 min，19.1 → 35.3 |
| `d43b3e6f` | 第一次 `no-shadow`；绿，5400，vitest 55.6 s | 635 / 4 / 1（as6:167），整条 169 s |
| `9685e300` | 第一次 `no-shadow`；绿，5400，64 s（vitest 57.5 s） | 636 / 3 / 1，整条 165 s，16 → 25.7 |
| `02cb019e` | 绿，5400，67 s（vitest 60.25 s） | 634 / 5 / 1（lw13:112、lw12:126），整条 171 s，约 33 → 36 |
| `5bbf897f` | 第一次 tsc 与 `no-await-in-loop`；绿，225 / 5402，67 s（vitest 60.15 s） | 636 / 3 / 1，2.7 min，整条 168 s，58 → 29 |
| `ade3abd1` | 绿，225 / 5402，64 s | 635 / 4 / 1（基线三格与 as6:167 [chromium] "Response has been disposed"，已知的偶发，重跑 3 过），2.6 min，整条 161 s，负载 25 → 26 |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `cdedff02`（实施者；C） | 实施者：以 2 退出（只因基线三格）；vitest 225 个文件、5395 格（60.6 s）；E2E 636 / 3 / 1，2.8 min，整条 3 分 58 秒，负载 10.4 → 26.0。C：636 / 3 / 1（基线三格），2.7 min，整条 3 分 59 秒，负载 7.8 → 45.1；vitest 5395 格（58.0 s）；提交 1、2 各自的门禁 5381、5395 格（`check.mjs` 430 / 102 / 102 / 3 与 431 / 103 / 103 / 3）。两次的受保护路径与 `git diff --check` 0 |
| `0e35d0dc`（实施者） | 5399 格（58.5 s）；E2E 636 / 3 / 1，2.7 min，整条 3 分 52 秒，负载 10.0 → 26.7 |
| `02cb019e`（实施者；C） | 实施者：5400 格（55.9 s）；E2E 634 / 5 / 1，2.8 min，整条 3 分 58 秒，负载 16.6 → 31.9：基线三格与 lw4:349、lw8:692 [firefox]，单独重跑 4 过。C（23:05:04–23:09:11）：5400 格（63.7 s）；E2E 637 / 2 / 1（se2:231、lw8:836；pg15 过了），2.7 min，整条 4 分 07 秒，负载 15.5 → 67.5（别的审查者也在跑）；`check.mjs` 431 / 103 / 103 / 3 |
| `ade3abd1`（实施者；合并的门禁） | `ade3abd1` 上以 2 退出，只因基线三格（se2:231、pg15:113 [webkit]、lw8:836 [webkit]）：vitest 225 个文件、5402 格（58.3 s），E2E 636 / 3 / 1，2.6 min，PG17 第 2 格 2.3 / 2.8 / 2.7 s，整条 3 分 48 秒（实施者的记录：`premerge-4.log` 没有时间戳；负载 14.8 → 27.5） |

- 每次都只败基线里同行同消息的格（上面列出的另几格都查清了，见"实施中的发现"）；每次"not configured to support act"0 次，全部 vitest 照旧打一次"Not implemented: navigation to another Document"（S1 已记）。
- Go 一侧：受保护路径的 `git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`m13-ui-refresh` 一直是 `4f5552f2`，分支是快进。
- 先写、先红：见 04-P4 10.2"测试"。
- vitest：5379（224 个文件，S2 之前）→ 5381（224，提交 1）→ 5395（225，提交 2）→ 5397（`36eb0c5d`）→ 5399（`0e35d0dc`）→ 5400（`d43b3e6f`）→ 5402（`5bbf897f`）。E2E：637 → 640（PG17 第 2 格 × 三种浏览器；之后的步都加在这一格里）。
- PG17 第 2 格的时长（Chromium / Firefox / WebKit）：实施者提交 2 2.2 / 2.6 / 2.5 s，`cdedff02` 2.3 / 2.6 / 2.6 s（合并之前 2.2 / 2.7 / 2.8 s），修复的五个提交 2.2–2.4 / 2.6–2.9 / 2.5–2.7 s（`0e35d0dc` 合并之前 2.2 / 2.9 / 2.7 s），`d43b3e6f` 2.3 / 2.8 / 2.7 s，`9685e300` 2.3 / 2.7 / 2.7 s，`02cb019e` 2.4 / 3.2 / 2.8 s（合并之前 2.3 / 3.2 / 2.7 s），`5bbf897f` 2.3 / 2.6 / 2.6 s，`ade3abd1` 2.5 / 2.7 / 2.7 s；审查 C `cdedff02` 合并之前 2.3 / 2.5 / 2.8 s，`02cb019e` 合并之前 2.4 / 2.8 / 2.7 s。7.5 推的是 2–4 s。L4 页面格 0.6–0.8 s，PG5 页面格 0.5–0.6 s；本机 E2E 2.6–2.8 min（负载 8–68）。
- **只靠 CI 的**：Linux 的 Chromium 第一次跑 `emulateMedia({ forcedColors })`（L4 的强制色一步，P4S2C-M1）；PG17 第 2 格在 Linux 的字体下标题的行数（取了上限）与 `expectFits`；Firefox、WebKit 的剪贴板（a10、lw4 已在三种浏览器里用同一个夹具）与第 3 步 `expect.poll` 读到 1.5 s 里的"已复制"。由负责人推送之后的 CI 看。
- **没有重现的 CI 项**：照 [01-P1](../01-P1-foundation.md) 7.3 的表（Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别）。
- **CI**：本机没有推送的凭据（决定 746、761）：负责人推送之后在最后一个头上补跑，看上面只靠 CI 的几项。
- **对比度**（l5，决定 880、891）：提交 1 与 `cdedff02` 的二进制上与之前相同，下限不改；修复没有改 l5 量的东西，没有重量（04-P4 10.2）。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库；量出的值与偏离在 04-P4 10.2"视觉的核对"。

- **实施者**（7.4 的 S2）：三种引擎的 `p4s2/shots/`（亮暗；1440、1024、375、320；强制色；中英；正文、标题、链接、任务项、七族 callout、代码块与头、mermaid 十四种、作者的 `style`、样稿亮暗）与拼图 `sheet-*.png`；修复之后的 `shots-fix/`（callout 的标题与长地址、间距、"复制"、流程图的 `padding` 8–15、作者自己的主题与变量），`shots-fix3/`（已完成项与孪生项：代码块、宽代码块、callout、折叠的 callout、图、表格、引用、嵌套的项，选区与复制），`shots-fix4/`（决定 906：只改字号的指令、给了浅色 `primaryColor` 的，两种应用主题）。临时故事的源在 `p4s2/visual/`，跑时复制进工作树、跑完即删。
- **审查 A**：vitest 探针 `zz-probe-a.test.ts`、`zz-probe-mermaid-a.test.ts`；静态页 `callouts.mjs` 与 `static/callouts.html`（`callouts.json`）；对着应用的 `pg17-zz-reva.spec.ts`（剪贴板、焦点、就地更新、作者的主题）；修复核对的 `f1/`（`cost.mjs` 量 `parse` 的代价，`zz-f1-c.test.ts`，已完成项下面的块的行为）。
- **审查 B**：临时故事 `visual/pg17-zz-revb.spec.ts`（八格）与修复核对的 `pg17-zz-revbfix.spec.ts`（另加已完成项、自带颜色的图、属性条的列表值）；`scripts/mock.mjs` 量样稿；`shots/`、`shots-fix/` 与拼图 `sheet-*.png`、`sheetfix-*.png`。
- **审查 C**：`revmut.py` 与 `mut/`、`mut2/`；`apply82.py`、`apply82-proposed.py`；门禁日志 `premerge-1.log`、`premerge-2.log`、`c1-gate.log`、`c2-gate.log`。

## 接受与推后

- 交给 v0.2（04-P4 第 9 节）：作者自己指定浅色主题（或 `base` 加浅色的变量）的图在暗色应用里，边、箭头与时序图的消息字画在暗色的图框上看不清（实施者第 7 节第 4 条；`base` 加 `primaryColor` 的边 1.21:1）；决定 905 的焦点；页内查找找到头里的语言名与"复制"（决定 869，Firefox 的 `window.find` 为真）。
- 照实记成偏离（04-P4 10.2）：流程图的图框高 129.3 px（样稿 94.25）；Chromium 里带行内代码的 callout 标题高 21.64 px；标题里的链接用正文色；903 让以代码块开头的列表项在代码块上方多 8 px；属性条里列表的列表的小块多 4 px（S2 之前就有）；状态图的注也用 `flowchart.padding`。
- 照旧：375 宽时甘特约 2 px、时间线 3.9 px、旅程 4.3 px 的字；旅程图写死的黑色主线；作者以 `style` 给的颜色在暗色下难读；Go 的 hljs、饼图多于七块时颜色重复、原生复选框（设计记的偏离）。
- 交给 S3：编辑器借用 `reading.css` 的变量（8.2 第 16 条已写"P4/S3 起也给编辑器"），`reading.css:12` 与 `design-tokens.test.ts:236` 的注释到 S3 一起看（P4S2C-N1）。
- t26、t27 由临时探针与截图看；52、52e 等价。
- 推后（P4S2C-F2-N4）：`code-blocks.ts:85` 的注释 146 列，下次碰这个文件时折行。

## 未核实的怀疑

- **读屏**（A）：WebKit 加 VoiceOver 读不读折叠的 `summary` 的展开状态（决定 901 之后是块）：只能人工，在 04-P4 第 9 节"收尾"的读屏清单里，照 901 改写。
- **真的查找栏**（A、实施者）：Ctrl+F / ⌘F 找不找得到 `user-select: none` 的语言名与"复制"；`window.find` 不等于查找栏，Playwright 里试不了。记给 v0.2（M12-polish 的一行）。
- **几百个宽代码块**（A）：Chromium 是否给每个粘住的头单独合成一层、多占内存，没量。
- **强制色下图里链接的焦点圈**（A）：`cdedff02` 之后是应用的琥珀，推理，没跑；图作为滚动区时强制色下的焦点环（B），375 宽时 B 造的图没有成为区域，没能实看。
- **标题里的音视频**（B）：块布局下折叠标题里带音视频控件（`titleMedia`）的样子没有用附件试。
- **滚动锚定**（C）：`was.undo()` 与 `codeBlocks` 重跑之间若有增强强制了一次布局，滚动锚定会在没有头的布局上调一次、头建回之后调回，推断净为零，没有探查。
- **`mermaid.parse` 与全局配置**（C）：修复核对第 2 轮由 A 核实：解析带着指令时会改全局配置，但 `render` 一开头先 `reset()` 再加它自己的指令，每张图的几步在同一个串行的任务里；11 张交错的图与换主题都照各自的颜色。
- **合并之前的整条命令的时长**（C 第 2 轮）："3 分 48 秒"是实施者的记录，`premerge-4.log` 没有时间戳；分项（vitest 58 s、E2E 2.6 min 加构建）合起来说得过去。
- **WebKit 里 mermaid 的导入失败**（B）：一次，单独重跑三次都绿，没有逐次排查（10.2）。
- **Linux 上的量**（C）：PG17 第 2 格的行数（已取上限）、`expectFits` 与剪贴板，等分支的 CI。
- **提交 1、2 单独为绿**：C 逐个跑过；之后的修复提交只看了实施者的日志与提交的时间，没有逐个重跑。
