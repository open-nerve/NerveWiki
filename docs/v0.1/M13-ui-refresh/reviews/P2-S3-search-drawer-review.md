# M13/P2/S3 搜索、⌘K、快速切换与窄屏：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p2-s3`（从 `m13-ui-refresh` 的 `bce67a06` 拉出，S1、S2 已并入）：`87d8eecb`（胶囊搜索框，Mod+K 与 Mod+Shift+F 的监听挪到外框 `search-shortcuts.ts`，`composing` 导出，`onMac` 挪进 `lib/platform.ts`，快速切换换装，se1 "button" 的下限重量）、`6f5f31c0`（窄屏的抽屉、细栏与遮罩：`useDrawer`、`Drawer`、`useDrawerActions`，`dialogOpen({ besideDrawer })`，`headingTakes`，树与"目录""历史"两行在抽屉里带 `arrived`，"留下"安静地关上抽屉，UI1 三格进三种浏览器与 `check.mjs`，PG16 第二格的 375 宽一段改在开着的抽屉里核对）、`da648b2c`（审查的修复：开着的菜单让开两个键（决定 806），只改查询参数的 replace 不关抽屉（807），变窄时焦点给菜单按钮，"更多"的图标照两行跟随、健康页的标题取到达的焦点（808），困住焦点时背后不滚（809），C 的两格）、`bf67b11d`（审查的修复：搜索框的间距、占位与提示的位置、窄屏的右内边距、清除按钮按配色画（805），快速切换没有结果时不写键盘提示）、`8f51cf11`（修复核对的四条 Nit：变窄时不滚窗口，锁住时留下滚动条的槽，筛选的后退一格，回收站预览一格等按钮）、`bb00d33f`（修复核对之后：笔记本的"更多"等树读到再点，LW8 的偶发，HI3、HI5、LW9 同样）；设计 [02-P2](../02-P2-shell.md) 4.3、6.3 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的 detached worktree 里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（抽屉的开关与焦点，375 与 1280 两种宽度，Esc 与菜单、对话框的层次，导航之后焦点的去处，快捷键与让路，筛选，未保存的拦截，Playwright 探针在三种引擎里，另写 vitest 变异）；B 管外观（Chromium 亮暗 × 1440、375，另有 320 宽、强制色、减少动效，Firefox、WebKit 在 1440 下的搜索框与快速切换、375 下的细栏与抽屉，逐项量位置、大小与颜色，对照 palette.html，样稿也在 1440 下量）；C 管测试、变异、门禁与完成度（独立重跑每个提交的检查与合并之前的整条命令、7.2 的 S3 各条与实施者的变异、自写变异找漏洞、核对 10.3 与"照实"的草稿）。重的运行都经 `heavy.sh` |
| 结论 | 实施中：Radix 菜单的探针（宽屏）结论是 `dialogOpen` 不改，后由审查 A 推翻（窄屏不成立，决定 806）；375 宽时抽屉是 `fixed`，变异 24 改由开着的抽屉里行下的 8 px 抓；强制色下抽屉的边看不出，根加 `max-md:border-r`。第一轮：A 2 Minor、1 Nit、3 Q；B 1 Minor、4 Nit、1 Q；C 2 Minor、3 Nit、1 Q；没有 Important。主持者的决定 805–809 答了 B 的 N3、A 的 M1、M2、Q1、Q3；A 的 Q2 与 B 的 Q1 照旧不改。修复在 `da648b2c`、`bf67b11d`。修复核对第 1 轮：三位都没有 Important、Minor，A、B 各 1 Nit（A 另 1 Q），C 2 Nit；四条 Nit 修在 `8f51cf11`，照 M12 的规则由主持者核对；合并之前的整条命令里 LW8 的一格偶发（"更多"在树读到之前被点）由 `bb00d33f` 修 |

## 基线

S3 不另跑基线，沿用 S1 开工之前的那一次（[S1 的审查记录](P2-S1-shell-account-review.md)"基线"：没改的 `58c5fa92` 上 `CI=1 make e2e`，594 过、3 败、1 跳过）：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S3 的 `make e2e`（实施者与审查 C 的）除了下面两次都只败这三格或其中两格（PG15 有两次过了），同行同消息：`e8da913e`（`8f51cf11` amend 之前的一版，没有人审过）另败 UI1 的一格，是代码的问题（门禁一节）；`8f51cf11` 上合并之前的整条命令第一次另败两格偶发的，AS6 [chromium]（S1 的记录列过的 `as6:153` 那一格，这次错在 `as6:167` 的 `apiResponse.json: Response has been disposed`）与 LW8 [firefox] `lw8:1028`（门禁一节；它的竞态由 `bb00d33f` 修掉），整条重跑只败基线三格。LW8 `lw8:771` 与 `markdown.test.ts` 的"not slower"没有出现。UI1 三格在三种浏览器里共 9 次，`make e2e` 由 600 过变成 609 过。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 6.3 提交 1 要的探针：主区开着 Radix 的模态菜单（页的"Page actions"、树的行菜单）时按 Mod+K、Mod+Shift+F。1280 与 1440 宽下三种引擎都只被 `preventDefault`，记下的 `focusin` 没有一次落到搜索框：菜单的 `FocusScope` 在 `focusout` 时就把焦点拉回去，同对话框开着时 | 照 6.3，`dialogOpen` 不另认 `[role="menu"]`，不加格（写进 `87d8eecb` 的说明）。只量了宽屏；375 宽下抽屉会开在菜单底下（审查 A 的 S3A-M1），由决定 806 改在 `useSearchShortcuts`（`da648b2c`） |
| UI1 的 ⌘K 一格先在 Firefox、WebKit 里跑（提交 1 的构建上）：`Meta+K` 之后框都拿到了焦点，浏览器没有吞掉 Cmd+K；那一格只因还没有抽屉而在下一步红 | 不改按 Mod+Shift+F |
| 375 宽下的变异 24（提交 2 的构建，三种引擎，在"Page 60"的行上手动调 `scrollIntoView` 与 `{ block: "nearest" }`，抽屉关着与开着各一次）：`window.scrollY` 都是 0（文档高 9196）。抽屉是 `fixed`，`scrollIntoView` 不滚窗口，S2 的 375 宽一段以 `window.scrollY` 再也抓不到它；分得出的是滚动区的 `scrollTop`：reveal 1284（行在区底之上 8 px），默认 1357，`nearest` 1276、1277（贴底） | PG16 第二格的 375 宽一段改为：先经笔记本首页，再到"Page 60"，`openDrawer`，它的链接 `toBeInViewport()`，链接的下边到 `[data-sidebar-scroll]` 的下边 7–9 px，`window.scrollY` 是 0（`6f5f31c0`）。变异 24 在 :133（默认，行下 80.9 px）与 :132（`nearest`，−0.125 / 0.883 px）三种都红 |
| 关着的抽屉 `inert`，Playwright 的 `getByRole` 找不到里面的东西（探针第一次就卡在这里） | 375 宽下要碰侧栏的步骤先 `openDrawer`（`e2e/fixtures/shell-pages.ts`）；交给后面（02-P2 第 9 节） |
| 强制色模式下遮罩与阴影都被去掉，抽屉的右边看不出（7.4 的截图） | 根加 `max-md:border-r max-md:border-border`，与宽屏同一条边线；重截之后看得见（`6f5f31c0`，本地 amend，只改说明） |
| UI1 第一格在 Firefox、WebKit 里 `expectFits` 读到抽屉的左边小于 0（滑入时量）；Chromium 里 `switchWorkspace` 超时（"Beta"在载入页面之后才建，工作区列表只在载入时读一次） | `openDrawer` 另等抽屉的过渡结束（`getAnimations()` 的 `finished`）；"Beta"在载入页面之前建（`6f5f31c0`） |
| Esc 在有字的搜索框里的 `setTimeout`：那个任务之前抽屉若已被遮罩关上，`close()` 会留下 `toMenu`，下一次由导航关上抽屉时把焦点从去向页的标题抢给菜单按钮 | 任务里另看 `now.current.trapping`；自写变异 t2 与"Esc 之后在它的任务之前点遮罩"一格守着（`6f5f31c0`） |
| 提交 2 第一次 `make lint-web` 红在 pg16 的 `no-shadow`（`area` 与外层同名；同一次的 e2e 在改名之前的树上跑） | 改名为 `column` 之后提交；提交 2 是那时的头，它单独为绿的证据是合并之前的整条命令 |

## 第一轮的发现与处置

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S3A-M1 | Minor | 375 宽下主区开着 Radix 菜单时按 Mod+K 或 Mod+Shift+F，抽屉开在菜单底下：菜单的 `FocusScope` 把焦点拉回菜单项；按 Esc，菜单关上，焦点回到遮罩后面的"Page actions"，再按 Esc 不起作用，Tab 在主区里走。1280 宽下什么也不发生，实施者的探针结论只在宽屏成立 | 决定 806：`useSearchShortcuts` 遇到开着的 `[role="menu"]` 只 `preventDefault`，`dialogOpen()` 不改；`shell-drawer.test.tsx` (3) 加"主区开着菜单"一格（`da648b2c`） |
| S3A-M2 | Minor | 筛选框里有一个待用的选择时按 Mod+K（Chromium 里点菜单按钮也一样），抽屉一开就关、焦点落到 `body`：select 失焦，`useChoice` 用掉待用的选择，`setParams(…, { replace: true, state })` 换了 `location.key`，`trapping` 变假；`headingTakes(arrived, REPLACE)` 为真，抽屉不把焦点给菜单按钮，结果页的标题这条路上已给过焦点。历史页、目录页的筛选同样 | 决定 807：A 的修法 (a)，渲染时把 `drawer.at` 挪到新的 key（导航类型 `REPLACE`、同 `pathname`、不同 `search`）；(5) 加"Where to look 有待用的选择"一格与"只有只改 query 的 replace 才留着"一格；三种引擎在结果页、历史页、目录页的筛选上核对（`da648b2c`） |
| S3A-N1 | Nit | 窗口从宽变回窄时，留在侧栏里的焦点落到 `body`，下一次 Tab 从主区开头开始（平板旋转会碰到） | `narrow` 变真、焦点在根里时，一个 layout effect 把焦点给菜单按钮；(6) 加"变窄"一格（`da648b2c`） |
| S3A-Q1 | Q | 从抽屉去"更多"的图标、笔记本行、齿轮、Home 时焦点给菜单按钮，去"目录"行与树的页时给页标题；LLM Wiki 里同一个"目录"从行进去给标题、从图标进去给菜单按钮 | 决定 808："更多"的图标照两行跟随（`useFollow`，两处共用），健康页的标题取到达的焦点；(4) 加"更多"的图标格、`health-page.test.tsx` 加一格（`da648b2c`） |
| S3A-Q2 | Q | 375 宽、结果页上从抽屉再搜一次，焦点给菜单按钮，新结果的标题不读（从结果页搜索时 `state` 是 `undefined`） | 照旧不改（主持者）：直接改成 `arrived` 不会让 `useArrivalFocus` 的 effect 重跑，焦点反而落到 `body`。由 (5)"只有只改 query 的 replace 才留着"一格的第一步守着 |
| S3A-Q3 | Q | 抽屉开着时背后的页面会滚：遮罩上滚轮从 0 滚到 600，树滚到底之后窗口跟着滚到 4941 | 决定 809：`trapping` 时锁住 `<html>` 的 overflow、恢复原值，`[data-sidebar-scroll]` 加 `max-md:overscroll-contain`；(1) 加"背后不滚"一格；三种引擎核对（`da648b2c`） |

A 另核实：Esc 的 `setTimeout` 里另看 `now.current.trapping` 对，A 的 r04 被 (5) 抓住、t2 那一格守住 `trapping` 的判断；A 的变异 r01–r09 全被抓住（变异一节）。

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S3B-M1 | Minor | 搜索框到第一行"首页"14 px，样稿 10 px，整列从"首页"起低 4 px：`space-y-1` 给除最后一个子元素以外的都加 4 px 的下边距，范围说明 `p` 是 `sr-only`（绝对定位）却仍是最后一个，框外那层 `div.relative` 也拿到 4 px | 表单改为 `flex flex-col gap-1`：框底到"首页"10 px（"首页"顶 96，同样稿），无效时提示仍在框下 4 px（`bf67b11d`） |
| S3B-N1 | Nit | 占位字在 44 px、⌘K 的右边在 233 px，样稿与决定 785 是 43 与 232：`pl-8.75` 与 `right-2.5` 都没算框的 1 px 边框 | `pl-8.5`、`right-2.75`：43 / 232（`bf67b11d`） |
| S3B-N2 | Nit | 窄屏下 ⌘K 已藏起，`pr-12` 还在，清除按钮离框的右端约 49 px | `pr-3 md:pr-12`：375 宽时内容右端 230（框宽 243），× 的墨迹离外沿约 19 px（Chromium）、15.5 px（WebKit）（`bf67b11d`） |
| S3B-N3 | Nit | Chromium 自带的清除按钮亮色深蓝、暗色白，都比 `text-faint` 的图标重；WebKit 是灰色圆圈；Firefox 没有 | 决定 805：`styles.css` 的 `.nw-search` 以 mask 画 14 px 的 ×，平时 faint、悬停前景色，强制色下 CanvasText（`bf67b11d`） |
| S3B-N4 | Nit | 快速切换没有结果时，右下仍写键盘提示 | 只在有选项时显示；`quick-switch.test.tsx` 加一格，en、zh 两格改用 `findByText`（`bf67b11d`） |
| S3B-Q1 | Q | 细栏不粘住：手机上滚下长页之后，菜单按钮随细栏滚走 | 照旧不改：00-M13 决定 731（细栏随页滚走，锚点不落到它底下） |

B 另核对过、没有问题的：搜索框（三种引擎相同，235×32，放大镜在 19，⌘K 29.5×19、11 px，与切换器之间 8 px，底色与边框同样稿）、垫底（各状态与框同色）、焦点的对比度（框 2.84 / 8.41、"搜索"按钮 3.10 / 6.95、无效 4.02 / 7.28）、快速切换（图标 16 px faint，标题 13.5 px，当前项内嵌 2 px 琥珀环）、细栏（44 px，菜单按钮 30 px，位置文字截断不溢出）、抽屉（宽 252，右边线，`shadow-dialog`，遮罩亮 rgba(16,17,20,.32)、暗 rgba(0,0,0,.45)，滑入 150 ms 三种约 153–158 ms 到位，减少动效时第一帧就到位，关上时没有动画）、强制色（遮罩与阴影去掉、右边线与各焦点环看得见）、`design-tokens.test.ts` 与 `cn.test.ts`。

### 审查 C（测试、变异、门禁）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S3C-M1 | Minor | 导航之后清掉抽屉状态那一句（4.4 唯一一处新豁免的理由）去掉没有测试红（u40）：不清的话后退回到打开它的那一条，抽屉又开、困住焦点 | (4) 加"后退回到打开它的那一条"一格：没改的树上过（h40），u40 下红（`da648b2c`） |
| S3C-M2 | Minor | "目录""历史"两行在抽屉里点当前的地址照旧 `opened`，没有格守着（u6：`inDrawer() && !here` 改成 `inDrawer()`，全过） | LLM Wiki 那一格末尾在目录页上再开抽屉、点"目录"：`opened`、"Navigation"有焦点；没改的树上过（h6），u6 下红（`da648b2c`；u6 之后作用在共用的 `useFollow` 上，图标格也红） |
| S3C-N1 | Nit | 10.3 的"做完"漏了决定 789（UI1 三格是它的 S3 那一半） | 本文档提交 |
| S3C-N2 | Nit | 照实 7.2 的第 24 行草稿丢了还在的 `page-tree-reveal.test.tsx`；第 59 行写的 `150ms`，浏览器给的是 `0.15s` | 本文档提交：第 24 行写"`page-tree-reveal.test.tsx`；PG16 第二格的 375 宽一段（开着的抽屉里行下 7–9 px：默认 80.9，`nearest` −0.1 / 0.9）"，第 59 行 `0.15s` |
| S3C-N3 | Nit | 照实 4.3.1 的"照实加 `W/lib/platform.test.ts`"多余（表里已有）；7.2 表后的说明没有 S3 的一句 | 本文档提交 |
| S3C-Q1 | Q | 10.3 与照实都没写 8.1 第 6 条与文档提交的其他项 | 本文档提交写其中属于 S3 的各项；第 12 节 P2 一行"完成"由合并之后的 Phase 完成提交写 |

C 另核实：两个提交各自单独为绿；7.2 的 S3 各条、S2 交来的 24 与 24n、t1–t10 都重跑、都被抓住；PG16 的 7–9 px 区间对两种错误写法都够紧（reveal 留 8 px，`scrollTop` 取整的余数小于 1 px）；UI1 的两处等待只会更严；StrictMode 探针（`shell-drawer.test.tsx` 与 `search-box.test.tsx` 整个文件加 `configure({ reactStrictMode: true })`）全过；knip 干净，新的 lint 豁免只有 4.4 表里那一处。

## 修复核对第 1 轮

在 `da648b2c`、`bf67b11d` 上，三位各在自己的 worktree 里（经 `heavy.sh`）。

- **A**：M1、M2、N1 修好；808、809 照决定做到。F1（806）：375 与 1280、两个键、三种一样，菜单仍开着，375 时抽屉关着、`<html>` 不锁，Esc 之后焦点回"Page actions"，之后再按照常起作用。F2（807）：结果页"Where to look"、历史页"Ingestion"、目录页"Type"，各试 Mod+K 与点菜单按钮，抽屉打开且保持开着、选择被用掉。F3：变窄之后焦点给"Navigation"（三种）。F4（808）：四个图标从抽屉去，各自的 `h1` 取焦点；再点当前的：`opened`、replace、菜单按钮；1280 宽 push、`opened`；Back 回到图标打开的那一条与树的规则一致。F5、F8（809）：每一种关法之后锁都解开（Esc、遮罩、树里选页、变宽、去设置时随外框卸下、"留下"；叠开的改名对话框各管各的锁）；先滚到 300 再 Mod+K 打开，遮罩上滚轮与树滚到底都仍 300。F6：807 的规则在别的导航上不变。A 的变异 f05、f11 被抓住，f09、f02 活下来（等价，变异一节）。
- **B**：M1、N1–N4 都修好，数值与样稿一致（实量一节）；375 宽下先滚到 400 再 Mod+K 打开，`scrollY` 仍 400、`clientWidth` 仍 375；健康页的标题到达时取焦点，`outline: none`、不匹配 `:focus-visible`，与别的到达标题一致；`design-tokens.test.ts` 与 `cn.test.ts` 11 格全过。
- **C**：M1、M2 修好（两格在没改的树上过，u40、u6 各让它们红，`:287:3`、`:256:54`）；两个新提交各自单独为绿；头上合并之前的整条命令第一次红在 vitest `trash-preview.test.tsx:587`（门禁一节），整条重跑全绿、只败基线三格。C 的变异 12 条被抓住，c807g、c808c、c807e 活下来（变异一节）。

新的发现：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| S3A1-N1 | Nit | 变窄时把焦点给菜单按钮，`focus()` 把页顶的细栏滚进视口：1280 宽滚到 2000、焦点在树的链接上，变窄之后 `scrollY` 2000 → 0（F7，三种）；焦点不在侧栏里时仍 2000。读者什么也没做，只是窗口变了 | `menu.current?.focus({ preventScroll: true })`；(6)"变窄"一格另断言 `focus` 以 `{ preventScroll: true }` 调用（`8f51cf11`）。A 的 F7 在三种引擎里 `scrollY` 2000 → 2000、焦点在"Navigation" |
| S3A1-Q1 | Q | 在最先载入的那一条上改 hash，抽屉不关：那两条记录的 `history.state` 都没有 key，React Router 给的都是 `"default"`，`drawer.at === location.key` 仍成立；应用内导航之后的条目上改 hash 照常关上。不是 807 造成的（hash 的变化是 POP） | 照旧（"接受与推后"）：抽屉开着时应用里没有会改 hash 的操作，锁不遗留 |
| S3B1-N1 | Nit | 经典滚动条下（Windows、Linux 的桌面或 macOS"总是显示滚动条"，窗口窄于 768）打开抽屉，锁去掉 `<html>` 的滚动条，背后的页面右移约 15 px（`clientWidth` 685 → 700，Edit 按钮右边 661 → 676） | 锁住时滚动条原来占地方（`innerWidth` 大于 `clientWidth`）才另设 `scrollbar-gutter: stable`，关上时恢复原值；(1)"背后不滚"一格另断言槽的设与还，与滚动条不占地方时不设（`8f51cf11`）。只在占地方时设：实施者第一次无条件设，Playwright 的 Chromium（隐藏滚动条，不占地方）里 UI1 第一格的 `expectFits` 读到页宽 360（视口 375），页面放得下时经典滚动条下同样会横移；改为有条件之后 amend（那一版 `e8da913e` 没有人审过）。B 的复现：开着时主区 685、Edit 右边 661，与关着相同；页面放得下时不设槽（700、676 开关前后相同）；隐藏滚动条的三种引擎不设、不横移（实施者的探针） |
| S3C1-N1 | Nit | `trash-preview.test.tsx:580-587`（M8/P2B 写的格，S3 没改这个文件）：`events.last().hello()` 之后立刻 `getByRole` 找 Restore 按钮，没等预览读到；事件流先连上、预览的答复后到时按钮还没画出 | `:587` 改为先 `await screen.findByRole(…)` 再聚焦（`8f51cf11`） |
| S3C1-N2 | Nit | 活下来的 c807g（`type === "REPLACE"` 改成 `type !== "PUSH"`：POP 也挪抽屉，结果页上又搜一次、抽屉开着时按后退，回到同一路径另一个 query 的那一条，抽屉照旧开着）、c808c（`here` 只比 `pathname`）、c807e（挪 `at` 时不更新 `search`） | c807g：(5)"只有只改 query 的 replace 才留着"一格在再搜关上之后再开抽屉、后退、等"Search “Guide”"、断言关上，c807g 在 `:473:3` 红（`8f51cf11`）；c808c、c807e 照实记为不守（变异一节） |

四条 Nit 由主持者核对（照 M12 的规则）：读了 `8f51cf11` 的改动，对照四条；看了实施者的证据：A 的 F7（三种引擎 `scrollY` 2000 → 2000，焦点在"Navigation"），B 的经典滚动条复现（开着时主区 685、Edit 右边 661，同关着；页面放得下与隐藏滚动条的情形不变），c807g 在 `shell-drawer.test.tsx:473:3` 红，新格在 `bf67b11d` 上红（`:123:17`、`:403:17`）；与合并之前的整条命令（门禁一节）。修复核对到此为止。

## 修复核对之后：LW8 的偶发（`bb00d33f`）

`8f51cf11` 上合并之前的整条命令第一次另败 LW8 [firefox] `lw8:1028`（门禁一节）：笔记本的"更多"自 S2 起在树下，`connectedHealth` 在树读到之前点 Health，树的行在按下与松开之间进来，那一排下移，松开时已不在链接上，没有点按。主持者：现在修，S2 把"更多"放到树下才有这个竞态，CI 里的偶发比一个提交更费负责人的事。

- **提交** `bb00d33f`（只改 e2e）：`e2e/fixtures/wiki-pages.ts` 加 `treeRead`（树的列表出现在 Loading 的位置；不到 100 行的树读到时一次画完，`app/shown-rows.ts`）；`e2e/fixtures/wiki-health.ts` 加 `openHealth`，先 `treeRead` 再点 Health，`connectedHealth` 与 LW8 的九处点按都经它，`healthLink` 不再导出。点"更多"的另外几处逐一看过：HI3 的 History 只等了页的标题，改为另等树读到；HI5 的 Trash 在撤销拿走 1,501 行之后点，树缩回只剩 Index 时"更多"上移，`treeTitles` 只剩 Index 的那一步挪到点按之前；LW9 的 History 在以接口写了 Unmarked 之后点，它的行由事件进树、在"更多"之上，改为等它进树再点。TB1、TB2、HI4、LW13 点的时候树已读到、不再变。
- **做出来**（探针，不进仓库，三种引擎）：点按的按下保持 300 ms（`click` 的 `delay`），树的第一次读取扣到按下为止（或 1.5 s）。没改的 `connectedHealth` 三种都红在 `wiki-health.ts:36`（健康页的标题没出现，焦点在 Health 或 `body`，地址没变，同 CI 的那一次）；改了之后三种都过（树读到之前不按，读取由 1.5 s 放行）。
- **跑**：改了的四个故事文件（LW8、HI3、HI5、LW9，`CI=1`，各自的项目）：87 过，只败基线的 LW8 [webkit] `lw8:836`；门禁见"门禁与 CI"。

## 主持者的决定（负责人可以改判）

全文在 [02-P2](../02-P2-shell.md) 2.3。

- **805 搜索框的清除按钮按配色画**（审查 B 的 S3B-N3）：`styles.css` 的 `.nw-search` 去掉 Chromium、WebKit 清除按钮的原生外观，以 mask 画 14 px 的 ×，平时 `--faint`、悬停 `--foreground`、强制色下 `CanvasText`；只换外观，有字时才出现、点了清空，仍是浏览器的；Firefox 没有清除按钮，不变。
- **806 开着的菜单让开 Mod+K 与 Mod+Shift+F，同对话框**（审查 A 的 S3A-M1）：窄屏上抽屉会开在菜单底下，Esc 之后焦点落在遮罩后面（与决定 799 同一类）；`useSearchShortcuts` 只 `preventDefault`，`dialogOpen()` 不改。
- **807 只改查询参数的 replace 不关抽屉**（审查 A 的 S3A-M2）：筛选的待用选择在失焦时用掉是页面自己的更新；渲染时把抽屉挪到新的一条；树上再点当前这一页、搜索框的再次提交、后退照样关上。
- **808 "更多"的图标照两行跟随**（审查 A 的 S3A-Q1）：抽屉开着、点的不是当前的地址时 `arrived`，否则 `opened`，以 `useFollow` 在点按时定；Home、笔记本行、齿轮照旧给菜单按钮。健康页的标题随之取到达的焦点：照字面带 `arrived` 时它的 `h1` 不取，焦点落到 `body`（实施者的探针，三种引擎）；另一种做法是只让 Health 图标给 `opened`，主持者取前者。
- **809 困住焦点时背后不滚**（审查 A 的 S3A-Q3；B 的 S3B1-N1）：`<html>` 的 overflow 锁住、滚动条原来占地方时留下它的槽，关上时恢复原值；`[data-sidebar-scroll]` 在窄屏 `overscroll-contain`（宽屏照决定 802）。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份到临时目录，跑完从备份复原（02-workflow 3.1）。实施者在各自提交的树上写一条跑一条（vitest 跑相关的文件；e2e 的经 `heavy.sh` 重建前端，再跑那个故事，UI1、PG16 在三种浏览器里）；审查 C 在 `6f5f31c0` 上独立重跑实施者的全部，修复核对时在 `bf67b11d` 上跑自己的。行号是各自那个提交的。

设计 7.2 的 S3 各条：

| # | 改坏 | 抓住它的 |
|---|---|---|
| 29 | Mod+K 不看焦点在不在编辑器里 | `search-box.test.tsx` 的编辑器格 |
| 30 | 对话框开着也聚焦 | `search-box.test.tsx` 的 Mod+K 主格 |
| 31 | Mod+K 不管组合中 | 同上 |
| 32 | 不看 `defaultPrevented` | 编辑器格 |
| 33 | `aria-keyshortcuts` 只列 Mod+Shift+F | 第一格；macOS 的 Cmd+Shift+F 格 |
| 34 | 关着不 `inert` | `shell-drawer.test.tsx` (1)(3)–(7)；UI1 第一格 `:52`（三种） |
| 35 | 不 `trapped` | (1)；UI1 第一格 `:66`（以程序把焦点放到外面，三种） |
| 36 | `onMountAutoFocus` 不挡 | (2)；`arrival.test.tsx` 两格 |
| 37 | `trapping` 不看 `at` | (3)(4)(5)；UI1 `:131`、`:143`（三种） |
| 38 | 树的链接照旧 `opened` | (4)(5)；UI1 `:143`（三种） |
| 39 | 导航后总给菜单按钮 | (4) 三格、(5)；UI1 `:131`、`:143`（三种） |
| 40 | Esc 改用 React 的 `onKeyDown` | (5) 账户菜单格 |
| 41 | `dialogOpen()` 不认抽屉 | `shortcuts.test.ts`；(3) 的 Mod+O |
| 42 | `besideDrawer` 不排除抽屉 | (3) |
| 43 | 关着时 Mod+K 不先开 | (3) |
| 44 | 选项多一个 `span` | `quick-switch.test.tsx` 五格 |
| 45 | `threeBrowsers` 漏写 `shell\/ui1-` | `check.mjs` 的 UI1 两行 |
| 54 | 导航那次提交不 `blur` | (4) 的后退格 |
| 55 | 当前的地址也 `arrived` | (4) 的当前页那一步 |
| 56 | Esc 不让组字 | (5) 的组字格 |
| 57 | Esc 不让有字的框 | (5) 两格；UI1 `:82`（Chromium、Firefox；WebKit 不清空，照设计不红） |
| 58 | 根一直 `tabIndex=-1` | (1)(6) |
| 59 | `duration-150` 写在 `motion-safe:` 外 | UI1 第一格 `:103` 的减少动效一步（三种） |
| 60 | 没有框也 `preventDefault` | `search-box.test.tsx` 的列表答 500 格 |
| 61 | 去掉垫底 | se1 `:307`（亮色读数回到 2.843，低于新下限 2.99） |
| 63 | "留下"不关抽屉 | (7) |
| 64 | `actions` 读第一次渲染的值 | (3) 的"Home 之后 Mod+K"与 (4)(5) |
| 66 | `headingTakes` 不算 `Located` | `arrival.test.tsx` 的三行 attachment |
| 67 | 两行在渲染时定 `state` | (4) 的 LLM Wiki 格 |
| 69 | 变宽不清掉 `drawer` | (6) |
| 24（S2 交来的） | `reveal` 改成 `link.scrollIntoView()` | PG16 第二格 `:133`（行下 80.9 px，三种） |
| 24n | 同上，`{ block: "nearest" }` | PG16 第二格 `:132`（行下 −0.125 / 0.883 px，三种） |

实施者另写的：

| # | 改坏 | 抓住它的 |
|---|---|---|
| t1 | 细栏总写工作区名 | `shell-drawer.test.tsx` 的细栏格 |
| t2 | Esc 的 `setTimeout` 不看 `trapping` | (5) 的"Esc 之后在它的任务之前点遮罩"格 |
| t3 | 遮罩点了不关 | (1)；UI1 `:93`（Chromium） |
| t4 | `"first"` 也去聚焦框 | (1)(4)–(7) |
| t5 | 两行 `replace: false` | 起初活下来；`page-tree.test.tsx` 补 LLM Wiki 两行一格之后红 |
| t6 | 两行不看 `linkFollowed` | 同上 |
| t7 | 导航的 effect 不看 `drawer !== null` | (6)(7) |
| t8 | `aria-expanded` 恒假 | (1) |
| t9 | 不写 `aria-modal` | (1) |
| t10 | `data-drawer` 恒为 `closed` | UI1 第一格的 `openDrawer`（`shell-pages.ts:22`，三种） |

修复的各条（实施者；7.2 新的几行）：

| # | 改坏 | 结果 |
|---|---|---|
| u40（C 的） | 导航之后不清掉抽屉的状态 | (4) 的后退格 `:286:3`（`da648b2c`） |
| u6（C 的） | `useFollow` 点当前的地址也 `arrived` | (4) 的 LLM Wiki 格 `:253:54`、图标格 `:271:54` |
| m806 | `useSearchShortcuts` 不看开着的菜单 | (3) 的"主区开着菜单"格 `:195:3`（`6f5f31c0` 上先写，`:171` 红） |
| m807 | 只改 query 的 replace 不挪抽屉 | (5) 的"Where to look"格 `:436:17`（`6f5f31c0` 上 `:412` 红） |
| m807a | 挪的时候不看导航类型（= C 的 c807a） | (5)"只有只改 query 的 replace 才留着"格 `:449:3` |
| m807b | 挪的时候不看路径（= c807b） | 同一格 `:455:3` |
| m807c | 挪的时候不看 query 变没变（= c807c） | (4) 的当前页格、LLM Wiki 格、图标格 |
| m807g | `type !== "PUSH"`（= c807g，POP 也挪） | 同一格的后退一步 `:473:3`（`8f51cf11`） |
| m808 | "更多"的图标不经 `useFollow`（= c808d） | (4) 的图标格 `:265:54`（`6f5f31c0` 上 `:241` 红） |
| m808h | 健康页的标题不接 ref（= C 的 ch2；ch1 去掉 `tabIndex`） | `health-page.test.tsx` `:175:54` |
| m809 | 不锁 overflow | (1)"背后不滚"格 `:117:25` |
| m809b | 关上时 overflow 写回空值（= c809b；c809d 不恢复） | 同一格 `:120:25` |
| m809g | 不设槽 | 同一格 `:123:17`（`8f51cf11`） |
| m809h | 关上时槽写回空值 | `:126:17` |
| m809r | 不看滚动条占不占地方，总设槽 | `:143:17` |
| n1 | 变窄时不给菜单按钮（= C 的 cN1a） | (6)"变窄"格 `:396:34`（`6f5f31c0` 上 `:372` 红） |
| n1b | 变窄时 `focus()` 不带 `preventScroll` | 同一格 `:413:17`（`8f51cf11`） |
| n4 | 快速切换没有结果时照样写提示（= C 的 cQ） | `quick-switch.test.tsx` `:147:18` |
| rN1 | N1 的 effect 不看 `narrow` | 活下来，等价：md 以上菜单按钮是 `display:none`，`focus()` 什么也不做；jsdom 里旧的陷阱在 layout 阶段还挂着，把焦点拉回 |

审查者另写的：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| r01 | A | `closed()` 不看 `narrow` | 抓住 |
| r02 | A | `blur` 改在 passive effect | 抓住 |
| r03 | A | `quietClose` 也设 `toMenu` | 抓住 |
| r04 | A | Esc 的任务不看框里是否还有字 | 抓住（(5)） |
| r05 | A | Esc 改挂在 `document` 上 | 抓住 |
| r06 | A | 不管按住的重复键 | 抓住 |
| r07 | A | `headingTakes` 对任何 `opened` 都为真 | 抓住 |
| r08 | A | 两行在宽屏也带 `arrived` | 抓住 |
| r09 | A | 树的链接在宽屏也带 `arrived` | 抓住 |
| u1 | C | `toMenu` 不复位 | 抓住（(5)"Esc 之后点遮罩"） |
| u2 | C | `closed()` 不看 `narrow` | 抓住（`search-box.test.tsx` 五格） |
| u3 | C | `inDrawer` 恒真 | 抓住（arrival 与 tree 八格） |
| u7 | C | `opened` 不看导航的类型 | 抓住（arrival 两行与 (4)） |
| u11 | C | 遮罩改成安静关上 | 抓住（(1)(5)） |
| u12 | C | Esc 的任务改成安静关上 | 抓住（(5)） |
| u14 | C | 开在框上时不选中文字 | 抓住（(3)） |
| u15 | C | 按住的重复照样聚焦 | 抓住（`search-box.test.tsx` 两格） |
| u18 | C | `quietClose` 把焦点给菜单按钮 | 抓住（(7)） |
| u28 | C | 编辑器的规则也用在 Mod+Shift+F 上 | 抓住（编辑器格） |
| u32 | C | `blur` 挪进 passive effect | 抓住（(4) 的后退格） |
| e-u4 | C | `loop={false}` | 抓住（UI1 `:61` 的 Shift+Tab，三种） |
| e-u5 | C | 关着时去掉 `invisible` | 抓住（UI1 `:53` 的 `toBeHidden`，三种） |
| u40、u6 | C | 见上 | 第一轮活下来（S3C-M1、M2）；`da648b2c` 之后红 |
| f05 | A | 图标永远只带 `opened` | 抓住 |
| f11 | A | 锁按 `narrow` 不按 `trapping` | 抓住 |
| f09 | A | 挪 `at` 时不更新 `search`（= C 的 c807e） | 活下来，不守：只有同一次打开里两次只改 query 的 replace、第二次回到原来的 query 才分得出，实际碰不到 |
| f02 | A | N1 的焦点改在 passive effect 里给 | 活下来，等价：编进构建 F3、F7 三种照样过；layout effect 更稳 |
| c807a、c807b、c807c、c809b、c809d、c808b、c808d、ch1、ch2、c806b、cN1a、cQ | C | 见上表的对应各行；c808b `useFollow` 不 `replace`（`page-tree.test.tsx:259`），c806b 菜单的判断挪到 `preventDefault` 之前（菜单格 `:195`） | 都抓住 |
| c807g | C | `type !== "PUSH"` | 修复核对时活下来（S3C1-N2）；`8f51cf11` 之后 `:473:3` 红 |
| c808c | C | `here` 只比 `pathname` | 活下来，不守：只在当前的地址带 query 时有差别 |

对照（只改测试，或测试加变异）：h40、h6（C 试写的两格，在没改的树上过；加 u40、u6 红）；C 的 StrictMode 探针（两个文件全过）。

没有格守的：`max-md:overscroll-contain`。有锁时桌面的三种引擎里看不出差别；不带锁的对照里，只有它时 Chromium 的树滚到底不再连到窗口（停在 900），Firefox 照样连（4942），两样都没有时 Chromium 到 4941（实施者的探针）。

## 实量

se1（Chromium；下限 = 实量 − 0.1，向下取到 0.01；决定 794）：

| 项 | 亮色 | 暗色 |
|---|---|---|
| button（S3 起量的是垫底） | 3.0977 → 下限 2.99（底 #ffffff） | 6.9539 → 下限 6.85（底 #25262a） |
| box | 2.8431，与 S1 相同，下限 2.74 不改 | 8.4057，不改，下限 8.30 |
| invalid | 4.0185，不改，下限 3.91 | 7.2832，不改，下限 7.18 |

搜索框与它的周围，都从侧栏的左缘量起（1440 宽下三种引擎、亮暗相同）：

| 项 | 修复之前（实施者与 B，`6f5f31c0`） | 修复之后（`bf67b11d`；B 的 spec 与实施者的探针，B 的修复核对） |
|---|---|---|
| 框 | 235×32，在 8 | 不变 |
| 放大镜 | 19–35 | 不变 |
| 占位字 | 44 | 43（样稿 43） |
| `⌘K` | 204–233 | 202.5–232（样稿 232） |
| 框底到第一行（"首页"顶） | 14（100） | 10（96，样稿 10 / 96）；375 的抽屉里也是 10 |
| 无效时提示到框 | 4 | 4 |
| 375 宽的右内边距、内容右端 | 48、194 | 12、230（框宽 243） |
| 清除按钮 | Chromium 原生 ×（亮深蓝、暗白），WebKit 灰色圆圈；375 宽离框的右端约 49 | 14 px 的盒子、墨迹约 8×8；静止同放大镜的 faint（亮 (145,147,155)、暗 (116,118,126)），悬停前景色（亮 (24,25,28)、暗 (232,232,234)），强制色下 CanvasText；Chromium 与 WebKit 相同（WebKit 偏右 2–3 px）；375 宽墨迹在 208–215（Chromium）、211–218.5（WebKit）；长查询停在 × 左边 4 px |
| 快速切换没有结果时 | 右下仍写键盘提示 | 只剩左边那句 |
| 经典滚动条、700 宽、抽屉开着（B 的复现） | 主区 700、Edit 右边 676（关着 685、661） | 685、661，与关着相同（`8f51cf11`） |

垫底与框同色（亮 #ffffff，暗 #25262a；暗色聚焦时两者一起变 #212226）；细栏 44 px、菜单按钮 30 px 在 x 8、位置文字在 x 44；抽屉宽 252（320 宽时也是）。

## UI1 与 PG16 的时间（7.5）

| 跑 | UI1 三格（Chromium / Firefox / WebKit） | PG16 两格 | 负载 |
|---|---|---|---|
| 实施者，`6f5f31c0` 合并之前的整条命令 | 1.9 / 3.7 / 2.3 s、0.31 / 0.82 / 0.62 s、0.86 / 1.4 / 1.1 s | 0.80 / 2.1 / 1.3 s、1.7 / 3.6 / 2.3 s | 6.5 → 27.9 |
| 审查 C，`6f5f31c0` 合并之前的整条命令 | 1.8 / 3.5 / 2.2 s、0.31 / 0.68 / 0.55 s、0.94 / 1.2 / 1.1 s | 0.64 / 2.0 / 1.1 s、2.0 / 3.0 / 2.4 s | 从 48.8 起 |
| 实施者，`bf67b11d` 合并之前的整条命令 | 1.7 / 3.4 / 2.4 s、0.34 / 0.88 / 0.72 s、0.92 / 1.2 / 1.3 s | 0.81 / 2.2 / 1.0 s、2.1 / 3.1 / 2.7 s | 12.9 → 38.3 |
| 审查 C，`bf67b11d` 合并之前的整条命令（第二次） | 1.7 / 3.5 / 2.7 s、0.40 / 0.99 / 0.98 s、0.90 / 1.3 / 1.2 s | 0.66 / 2.1 / 0.93 s、1.8 / 3.6 / 2.7 s | — |
| 实施者，`8f51cf11` 合并之前的整条命令（重跑的那一次） | 2.0 / 3.7 / 2.3 s、0.42 / 0.90 / 0.86 s、0.92 / 1.2 / 1.4 s | 0.69 / 1.7 / 0.98 s、2.0 / 3.6 / 2.7 s | 3.7 → 32.8 |
| 实施者，`bb00d33f` 合并之前的整条命令 | 2.0 / 3.8 / 2.5 s、0.29 / 1.2 / 0.63 s、0.92 / 1.4 / 1.2 s | 0.74 / 2.2 / 0.87 s、1.7 / 3.6 / 3.0 s | 23.3 → 32.8 |

PG16 与 UI1 共 15 次，一次合计约 23–26 test-seconds（C 在 `6f5f31c0` 上那一次约 24 s），远低于 7.5 估计的 75–90 s；E2E 多了 9 格，墙钟 2.3–2.5 min 不变。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库。

- **实施者**（7.4 的 S3，Chromium）：搜索框的静止、聚焦、有字、无效与按钮聚焦（亮暗 × 1440），快速切换，整页，375 的关着与开着（亮暗），强制色（抽屉开着、Tab 到第二个控件、关着时菜单按钮聚焦），减少动效（40 ms 时平时在 −208、减少动效时已在 0）。修复之后用 B 的 spec 在三种引擎里重截（260 张），另写一个探针量占位字、⌘K、第一行、无效的提示、375 的右内边距、清除按钮的颜色（静止、悬停、强制色）与快速切换的提示。
- **审查 B**：213 张（Chromium 亮暗 × 1440 / 375，320 宽、强制色、减少动效；Firefox、WebKit 在 1440 下的搜索框与快速切换、375 下的细栏与抽屉）；修复核对另截 Chromium 4 倍、WebKit 2 倍的特写，与经典滚动条的复现（Chromium 不带 `--hide-scrollbars`，给 html 加 15 px 的 `::-webkit-scrollbar`，700 宽）。
- **审查 A**（行为，Playwright 探针，三种引擎，375 与 1280）：T1–T11（菜单与对话框之上的快捷键、Esc 的层次、导航之后焦点的去处、跨过 md、未保存的拦截、编辑器与不存在的工作区上的快捷键、宽屏没有抽屉、换组、筛选的待用选择、背后的滚动、结果页上再搜）；修复核对 F1–F8。
- **实施者的修复探针**（三种引擎）：806 在 375 与 1280；807 在结果页、历史页、目录页的筛选上各试 Mod+K 与点菜单按钮；N1；808 的四个图标、树的页、笔记本行与 1280 宽；809 先滚到 300（Mod+K 打开），遮罩上滚轮、树滚到底、手动去掉锁、各种关法，与不带锁、只有 overscroll-contain、两样都没有的对照；修复核对之后 A 的 F7、B 的经典滚动条复现，另加页面放得下与 375 宽两种，经典滚动条与隐藏滚动条（三种引擎）各跑。
- **主持者**：修复核对时看了实施者的证据（上面"修复核对第 1 轮"末尾）。

## 门禁与 CI

| 提交 | 实施者 | 审查 C |
|---|---|---|
| `87d8eecb` | `heavy.sh make lint-web knip test-web` 绿（215 个文件、5200 格）；`CI=1 make e2e` 600 过、3 败、1 跳，2.3 min，负载 11.3 → 25.7 | 绿，215 个文件、5200 格；e2e 600 / 3 / 1，2.4 min |
| `6f5f31c0` | 第一次 `lint-web` 红在 pg16 的 `no-shadow`，改名之后绿（216 个文件、5228 格）；同一次的 e2e（改名之前的树）609 / 3 / 1，2.3 min，负载 5.3 → 39.5；本地 amend 一次，只改说明 | 合并之前的整条命令（下表） |
| `da648b2c` | 绿（216 个文件、5236 格，1 分钟）；`CI=1 make e2e` 610 过、2 败（SE2、LW8，PG15 这次过了）、1 跳，2.5 min，负载 17.8 → 26.8 | 绿，216 个文件、5236 格；e2e 609 / 3 / 1，2.5 min（负载从 85.8 起） |
| `bf67b11d` | 绿（216 个文件、5237 格） | 合并之前的整条命令（下表） |
| `8f51cf11` | 绿（216 个文件、5237 格，1 分钟）；e2e 由合并之前的整条命令跑（下表） | — |
| `bb00d33f` | 绿（216 个文件、5237 格，1 分钟）；改了的四个故事先单独跑过（87 过，只败基线的 `lw8:836`）；它是头，e2e 由合并之前的整条命令跑（下表） | — |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `6f5f31c0`（实施者；C） | 实施者：216 个文件、5228 格；e2e 609 过、3 败、1 跳（基线三格，同行同消息），Playwright 2.4 min，整条 3 分 27 秒，负载 6.5 → 27.9。C：全过，e2e 609 / 3 / 1，没有 flaky，2.3 min，整条 3 分 27 秒，跑完树干净 |
| `bf67b11d`（实施者；C） | 实施者：216 个文件、5237 格；e2e 609 / 3 / 1（基线三格），2.4 min，整条 3 分 29 秒，负载 12.9 → 38.3。C 第一次（负载 41 → 68）：test-web 红一格，`trash-preview.test.tsx:587` 找不到"Restore “Install” (From: Guide)"（5237 格里 1 格，M8 起的竞态，S3C1-N1），make 停在 e2e 之前；那个文件单独跑 10 次都绿；第二次全过：216 个文件、5237 格，e2e 609 / 3 / 1，没有 flaky，2.4 min，整条 3 分 33 秒，跑完树干净 |
| `e8da913e`（实施者；`8f51cf11` amend 之前的那一版，没有人审过） | test-web 5237 格绿；e2e 609 过、3 败、1 跳：SE2 与 LW8 是基线（PG15 过了），另一格是 UI1 [chromium] 第一格 `:67` 的 `expectFits`（页宽 360，视口 375）：无条件设的 `scrollbar-gutter: stable` 在 Playwright 的 Chromium（隐藏滚动条）里也留了 15 px 的槽（S3B1-N1 那一行）。改为只在滚动条原来占地方时设，amend 成 `8f51cf11` |
| `8f51cf11`（实施者） | 第一次（负载 14.3 → 27.2）：216 个文件、5237 格；e2e 607 过、5 败、1 跳：基线三格之外另有两格偶发的。AS6 [chromium]（`as6:153` 那一格）错在 `as6:167` 的 `apiResponse.json: Response has been disposed`（S1 的记录列过的偶发）；LW8 [firefox] `lw8:1028` 错在 `e2e/fixtures/wiki-health.ts:36`，健康页的标题 10 秒没出现：trace 里 `connectedHealth` 在 `goto` 之后 18 ms 就点"更多"的 Health，那时树还写着 Loading，"更多"那一排在 y 316–344，按下落在链接上（焦点到了 Health），树读到之后那一排下移约 60 px，松开时已不在链接上，没有点按、没有导航（点按前后相隔约 40 ms 的两帧：前一帧树还写着 Loading、那一排在点按的位置，后一帧树已读到、那一排下移）。两格都不碰 S3 改的代码（宽屏，抽屉的 effect 不跑），S2 起"更多"在树下。LW8 那一格的竞态由 `bb00d33f` 修掉（"修复核对之后"一节），AS6 的照旧是偶发。整条重跑（负载 3.7 → 32.8）：216 个文件、5237 格；e2e 609 过、3 败（基线三格，同行同消息，PG15 也在）、1 跳，Playwright 2.5 min，整条 3 分 32 秒；跑完树干净 |
| `bb00d33f`（实施者） | 216 个文件、5237 格；e2e 609 过、3 败（基线三格，同行同消息）、1 跳：LW8 `lw8:1028`、HI3、HI5、LW9 的页面格都过（LW8、LW9 在三种浏览器里）；Playwright 2.5 min，整条 3 分 33 秒，负载 23.3 → 32.8；跑完 docs 之外的树干净（文档的改动还没提交） |
| `cc5e8898`（主持者；加上文档提交的头，`m13-ui-refresh` 没有动过，即合并的树） | 与 `bb00d33f` 只差 `docs/`（`git diff bb00d33f..cc5e8898 -- . ':(exclude)docs'` 为空），合并之前的整条命令已在 `bb00d33f` 上跑过，不再重跑；Go 一侧的断言 0、`git diff --check m13-ui-refresh...HEAD` 0、快进；合并 `82838aac` |

- 每次都只败基线里同行同消息的格（SE2 `se2:231:23`、PG15 `pg15:113:5`、LW8 `lw8:836:29`），`e8da913e` 那一次另有上表的 UI1，`8f51cf11` 的第一次另有上表的两格偶发。
- Go 一侧：`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`git diff --check m13-ui-refresh...HEAD` 干净；分支是快进。
- vitest：5190（S2 之后）→ 5200（提交 1，215 个文件）→ 5228（提交 2，216 个文件）→ 5236（`da648b2c`）→ 5237（`bf67b11d`、`8f51cf11`、`bb00d33f`）。
- **只靠 CI 的**（决定 795(d)）：se2 的 page 格在 se2:230 之后的步骤（含 :274 的访客说明；本机红在 :231，走不到）与 UI1 的 `Control+K`（本机 macOS 走 `Meta+K`；Firefox 在 Linux 上会不会被浏览器的 Ctrl+K 拿走，本机验证不了），只在 Linux 的 CI 上走。本机由 vitest 走 Ctrl+K（jsdom 不是 Mac），Cmd+K 另有一格把平台打桩成 Mac。
- **没有重现的 CI 项**：照 [01-P1](../01-P1-foundation.md) 7.3 的表（Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别）。
- **CI**：本机没有推送的凭据（决定 746、761）：负责人推送之后在最后一个头上补跑，看上面只靠 CI 的几步。

## 接受与推后

- 结果页上从抽屉再搜一次，焦点给菜单按钮，新结果的标题不读（审查 A 的 S3A-Q2）：照旧。宽屏上焦点留在框里；直接改成 `arrived` 时上一条也带 `arrived`，`useArrivalFocus` 的 effect 不重跑，焦点反而落到 `body`。
- 细栏不粘住（审查 B 的 S3B-Q1）：照 00-M13 决定 731，锚点不落到它底下；手机上滚下长页之后，要打开导航先滚回顶部（或按 Mod+K）。
- 页面滚下去时用 Mod+K 打开抽屉，Esc 或点遮罩关上，焦点回到页顶细栏里的菜单按钮，浏览器把页面滚回顶部（300 → 0，三种；Chromium、Firefox 在 focus 事件之前滚，WebKit 之后）：决定 790 本来的行为，与锁无关（开着时手动去掉锁，位置不变）。照旧（主持者）：窄屏上打开抽屉多半先滚回页顶点菜单按钮，Mod+K 在窄屏上少见。变窄时的那一次已不滚（S3A1-N1）。
- 在最先载入的那一条上改 hash，抽屉不关（审查 A 的 S3A1-Q1）：React Router 给最先载入的条目与它的 hash 条目同一个 key（`"default"`）；抽屉开着时应用里没有会改 hash 的操作，锁不遗留。照旧。
- f09（= c807e）、c808c、`max-md:overscroll-contain` 不守；rN1、f02 等价（变异一节）。
- 没有实测的：iOS Safari 上 `<html>` 的 `overflow: hidden` 是否截住触摸滚动，`overscroll-contain` 在真机上的效果；经典滚动条只在 Chromium 里模拟过（B 的复现与实施者的变体），没在 Windows、Linux 的真浏览器里看；真机触屏与读屏器（VoiceOver、TalkBack 下的 `aria-modal`）；真实的输入法组字（浏览器里用的是合成的 `isComposing`）；真 Safari 的 Esc 会不会清空 `type=search`（Playwright 的 WebKit 不清空，代码两种都处理了）；Linux 上的 `Control+K`（只靠 CI）；WebKit 在 macOS 上 Tab 不进按钮，"搜索"按钮的聚焦只在 Chromium、Firefox 上看了。
