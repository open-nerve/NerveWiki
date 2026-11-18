# M12/P2 S3（375 px、`bdi` 与对比度）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p2-s3`。实施的 6 个提交，从 `main` 的 `f08dca69`（P4/S1 的合并）开始，审查时在 `72b1e679`：<br>名字进 `bdi`，共用件 `W/i18n/isolated.tsx` 与五处拼句 `9ca1c6a1`；历史与变更集页在 375 px 不超宽，新 Playwright 项目 `webkit-narrow` 与夹具 `E/fixtures/narrow.ts`，MC3 的 `@narrow` 一格 `76be71df`；长标题与它的对话框，PG2 的 `@narrow` 一格、LW3 (page) 加一段，钉住项目配置的 `tools/e2e-projects/check.mjs` `ceb0482b`；焦点环、菜单与选项的当前项、次要文字的对比度，夹具 `E/fixtures/contrast.ts` `468635fe`；命中加粗到 700 `f5131dd3`；33.1 复现之后 `NativeSelect` 加 `contain-paint`、SE1 拆出 `@narrow` 的一格 `72b1e679`。<br>修复第 1 轮 4 个提交，头 `f5d1220a`：`check.mjs` 的 `REQUIRED` 加 SE1 `1d8a2fd9`；`check.mjs` 经 `pnpm --filter` 跑 Playwright、头注释改准 `63004369`；暗色 destructive 按钮悬停的底 `3c4d5710`；`Isolated` 的 `key` 按位置 `f5d1220a`。<br>合并 `main` 一次：`e19172e3`（P2/S2 的合并 `6505c3bf` 与导入守卫的旁支；`history-filters.tsx` 一处冲突，见"实施中定的"）。<br>分支共 10 个提交（不计合并），相对 `main` 40 个文件，加 998 行、减 99 行：网页 28 个文件（`W/i18n/`、`W/pages/`、`W/components/ui/`、`W/editor/text-diff.tsx`、`W/styles.css`，其中 5 个 vitest 文件）；e2e 9 个文件（两个新夹具、`playwright.config.ts`、MC3、PG2、PG4、LW3、SE1、L5 六个故事）；新的 `tools/e2e-projects/check.mjs`、`Makefile` 与 `knip.jsonc` 各两行。翻译文件、契约、迁移与服务端都不动，`docs/` 在本提交 |
| 规格 | [P2 设计](../02-P2-web-polish.md) 4.3（4.3.0–4.3.5）、5.1、5.2 的 S3、6.3、7.1–7.4、8.2 的 S3 与 13.4、第 9 节；2.3 的决定 393–396、413、414、420、421（P2-16 至 P2-19、P2-36、P2-37、P2-43、P2-44）；[00 号文档](../00-M12-design.md) 4.5，第 7 节"总体设计的修订"第 21 条，附录 A 的 P2/S3 一行；移交 [M5-polish](../handoffs/M5-polish.md) 的 S3 各项 |
| 审查方式 | 三位审查者并行，各在自己分离的工作树里（A `nerve-wiki-m12-p2-rev-a`、B `nerve-wiki-m12-p2-rev-b`、C `nerve-wiki-m12-p2-rev-c`，都在 `72b1e679`），重的经 `heavy.sh`，不碰实施者的工作树；`TMPDIR` 在 `~/.cache` 下各自的目录，用完删了：<br>A `bdi` 与拼句（提交 1，修复的 F4）：vitest 探针（名字是 `"0"`、空、两位数的 slot、译文漏掉一个名字、整句与 `same` 逐个比）；`bidi-probe.mjs` 在 Chromium、Firefox、WebKit 里比有没有 `bdi` 时各段的位置、复制的文字、`innerText` 与 Chromium 可访问性树的名称；按出现先后取名字的变异；对 S2 的 `4c9f1212` 试合并、跑九个 vitest 文件，`merge --abort`。<br>B 宽度、对比度与 33.1（提交 2–6）：在同一个构建上从 `DOMContentLoaded` 注入样式，比去掉 `contain-paint`、换别的改法与外层 `overflow: hidden`；逐像素读截图；Chromium 与 WebKit 里整页逐个 Tab（亮暗两种主题的 1280、亮色的 375，页面、历史与搜索，每种 30 多个停点）；destructive 按钮与快速切换的探针；SE1、L5 的格各 `--repeat-each=3`。<br>C 提交、门禁、检查工具与文档（全部）：四个提交逐个检出跑 `make lint-web`、`make knip`、`make test-web`；knip 的两种改法；`--list` 的 JSON；`check.mjs` 的六个变异与两个 e2e 变异；用 `gh` 只读取逐格的用时；文档草稿对照 `main` |
| 日期 | 2026-11-18（实施、审查、修复第 1 轮、核对第 1 轮、合并 `main`） |
| 结论 | 没有 Important。Minor 3：`check.mjs` 不管 SE1 的 `@narrow` 那一格，去掉它的标签照过（P2S3C-M1）；草稿的 13.2 新条与 `main` 上 P4/S4 的第 30 条相撞（P2S3C-M2）；暗色下 destructive 按钮悬停时白字 3.45:1（P2S3B-M1，改前就有）。Nit 6（A 2、B 1、C 3），Q 2（A 1、B 1）。三位都确认：每个提交单独为绿，格数与实施报告相同；变异与反向对照照报告红，"等价"与"预写的抓不到"站得住；33.1 改前 704、改后 375，`contain-paint` 没有可见的副作用，外层 `overflow: hidden` 确实裁掉焦点环；`bdi` 修好了乱序，整句的文字、复制与可访问名称不变。修复第 1 轮 4 个提交；核对第 1 轮三位的旧发现都关上，新的 0 Important、0 Minor、1 条 Nit（C），在本提交写对，不开第二轮 |

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `72b1e679` 的。修复照主持者的说明（`p2s3/fix-1.md`）在分支上加提交，普通推送、不改写已推的历史，每个单独为绿；文档的部分在实施者的草稿里改，落在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2S3C-M1 | 中（检查工具） | `check.mjs` 的 `REQUIRED` 只列 MC3、PG2。去掉 SE1 那一格的 `{ tag: "@narrow" }`，检查照过、列 `webkit-narrow 2`：完整性那一条只数带标签的格，看不见丢了标签的。SE1 那一格是 `contain-paint` 唯一的守卫（去掉它只在 `webkit-narrow` 红）。实施报告说"完整性那一条已管它"不实 | `1d8a2fd9`：`REQUIRED` 加 `["webkit-narrow", "search/se1-search.spec.ts", "SE1 (page): at 375 wide"]`，注释写明为什么；自检样例加"必需的格丢了标签"一种（决定 592）。去掉标签之后红在 `lists no cell "SE1 (page): at 375 wide…"`。核对 C：关上 |
| P2S3C-M2 | 中（文档稿） | 草稿把 S3 写成 13.2 第 30 条，`main` 上第 30 条已是 P4/S4 的（`35501dec`）；照草稿插在第 29 条末句之后，会有两个"30."、S3 的排在前面，与决定 505、536 不合。第 30 条的末句在文件里出现三次，不能当锚点 | 13.2 加第 31 条，插在第 30 条末句之后，锚点用更长的一句（`4.1。1 万页根下的一本，…约 2.4 秒全部行进 DOM。`，恰好一处）；P2 设计与 00 号文档里指它的号随之改，决定 396 只加注（决定 593）。核对 C：关上 |
| P2S3B-M1 | 中（对比度） | 暗色下 destructive 按钮静止时白字 6.43，指针放上去底回到 destructive 的 90%（`hover:` 的特异性高于 `dark:`），白字 3.45（WCAG 1.4.3）。S3 之前也是这样；4.3.4 的表只有静止一行 | `3c4d5710`：加 `dark:hover:bg-destructive/50`，暗色悬停 3.45 → 8.07，比静止深，悬停仍看得出；亮色 4.77、4.54 不变。PG4 (page) 在删除之前量确认框的 "Delete"，两种主题、静止与悬停都 ≥ 4.5（决定 591）。核对 B：关上，读数一致 |
| P2S3A-N1 | 细节（代码） | `Isolated` 用 `key={part}`（slot 的序号），设计 4.3.1.5 写 `key={i}`；一句译文写两次同一个占位符时有两个同 `key` 的 `bdi`，React 报错（今天没有这种译文）；实施报告没记成偏离 | `f5d1220a`：`key={i}`，lint 的例外照 `page-backlinks.tsx:165` 的先例；`isolated.test.tsx` 加"a name a translation writes twice is in a bdi each time"，`key={part}` 时红在 React 的重复 key。核对 A：关上 |
| P2S3A-N2 | 细节（说明） | 设计 32.4 举的两个"会乱序"的例子，en 的 "Moved from “A” to “B”" 与 en 的 "MCP (עוזר) · 5 minutes ago"，三种引擎里没有 `bdi` 也不乱（两个名字之间有强 L 的 "to"；UBA 的 N0 把成对的括号定为 L）；`9ca1c6a1` 的说明照搬了前一个 | 照实写（P2 设计 10.3，本记录"更正"）；仍加 `bdi`，与同类一致。确实乱序、`bdi` 修好的有四处（删除者一行、搜索的路径、改名、zh 的撤销链接）。已推的说明不改写（决定 594）。核对 A：关上 |
| P2S3A-Q1 | 问 | S2 与 S3 合并时，文本冲突只在 `history-filters.tsx`；S3 的整句断言一格也不用改；丢了 S3 的四处类 vitest 看不出来，要另跑 MC3 的 `@narrow` | 写成合并的检查单，后合并的 S3 照做（`e19172e3`：两边都留，另跑 MC3 与 TB1、SE1，见"门禁与 CI"）。核对 A：关上 |
| P2S3B-N1 | 细节（报告） | "WebKit 里选择看不到焦点环"写重了：Playwright 的 WebKit 不画原生下拉的 `box-shadow`，但聚焦时 1 px 的边框变成 `#737373`（`focus-visible:border-ring`），对页 4.74、对下拉里面 4.36；截图里那条深灰的边框就是它 | 报告与草稿照实改；13.2 第 31 条写明 WebKit 的下拉只有 1 px 的 `ring` 色边框。核对 B：关上 |
| P2S3B-Q1 | 问 | WebKit 原生下拉没有 3 px 的环，改前就有；今天满足 AA，按 4.5 是 C；改法核过（选择的 2 px 外框，同时去掉 `ring-[3px]`，否则 Chromium 与 Firefox 画两层） | 进 v0.2；真的 Safari 进 P5/S3 的人工清单（决定 590）。核对 B：关上 |
| P2S3C-N1 | 细节（检查工具） | `knip.jsonc` 的 `ignoreBinaries: ["playwright"]` 对根工作区整个生效；改用 `pnpm --filter @nervewiki/e2e exec playwright …`，knip 不报、检查的输出不变 | `63004369`：改用 `--filter`，去掉 `ignoreBinaries` 与它的注释（决定 592）。核对 C：关上；包名写错时 pnpm 以 0 退出、`JSON.parse` 抛错，检查照样失败 |
| P2S3C-N2 | 细节（检查工具） | `check.mjs` 的头注释说"在 WebKit 上跑"；把 `webkit-narrow` 换成 Desktop Chrome，检查照过：`--list` 的 JSON 里没有 `use`，看不见引擎 | `63004369`：头注释写明它看的是项目、文件与格，引擎由配置那一行定；"…but not run on WebKit"的报错随之改成"…but not listed"（实施中定）。核对 C：关上 |
| P2S3C-N3 | 细节（文档稿） | 草稿缺 `v0.1-design.md` 第 15 节一行、00 号文档第 21 条的"落了"、状态行与 `:893`；草稿 A、B 讲过渡的说法不一 | 本提交都写了；两处都写"先结束元素与它祖先的过渡"。核对 C：关上 |
| P2S3C-C1-N1 | 细节（文档稿，核对第 1 轮） | 草稿 A 把"按钮上的字静止与悬停都是"接在讲次要文字的那一句里，读来像按钮上的字也是次要文字；量过的只有 destructive（审查 B 也提了，不算发现） | 单列一句："destructive 按钮的白字对它的底也至少 4.5:1，静止与悬停都是" |

## 主持者的决定

负责人可以改判；写进 [00 号文档](../00-M12-design.md)第 11 节"2026-11-18：P2/S3 的决定 589–594"：

- **589 33.1 复现了，按 B 在 S3 修：`NativeSelect` 的基类加 `contain-paint`，不用 4.3.1.1 的办法**（P2 设计 4.3 的 33.1；实施报告第 5.1 节、第 7 节第 1 条；审查 B）：
  - 在没改的 `f08dca69` 上，Playwright 1.63 的 WebKit 26.6 里 SE1 (page) 的 375 一段页宽 704，三遍都是；只种数据、搜一次的短格也是。Chromium 153 同一构建 375。没有一个元素的盒子超出屏幕；逐个隐藏元素，只有"在哪里找"的 `select` 与它的祖先能让页宽回到 375：WebKit 把选择显示的那一项的文字整个算进页面的宽度，选择裁掉了也一样。
  - 4.3.1.1 的办法都不管用：选择的 `overflow` 在 WebKit 里不生效（计算值仍是 `visible`），外层是块里的块，`min-w-0` 从头布局照样 704。`contain: paint` 管用。外层 `overflow: hidden` 也管用，但裁掉选择 3 px 的焦点环（审查 B 核实）；`main` 的 `overflow-x: clip` 会把以后真的超宽一并藏起来。
  - 放在基类上：原因在组件（WebKit 画选择的方式），基类已有同为窄屏的 `max-w-full`（`692d2482`）。它不裁选择自己的环与边框（Chromium、Firefox 截图逐字节相同）。历史的"客户端"选中 128 个字的客户端时，改前改后都是 375：别的选择今天没有复现。
  - SE1 拆出 `@narrow` 的一格（决定 413）。附录 A 的 33.1 仍在 M 类、注明 S3 复现与修好：真的 Safari 上是否同样，发布之后看（决定 590）。
- **590 WebKit 原生下拉没有 3 px 的环，按 C 进 v0.2；真的 Safari 上的两项进 P5/S3 的人工清单**（审查 B 的 N1、Q1）：
  - Playwright 的 WebKit 不画原生下拉的 `box-shadow`，聚焦时只有 1 px 的边框变成 `#737373`，对页 4.74、对里面 4.36，满足 2.4.7 与 1.4.11；改前就是（那时边框 `#a1a1a1`，对白 2.58，由 4.3.4 推得），S3 的 `--ring` 顺带让它达标。
  - v0.2 的改法已核：`select:focus-visible` 的 2 px 外框，同时去掉 `NativeSelect` 的 `ring-[3px]`。
  - 人工清单（发布之后，真的 Safari，iPhone 或 macOS 的窄窗口）：窄屏的搜索页（33.1）；原生下拉的焦点。
- **591 暗色 destructive 按钮悬停的对比度在 S3 修**（审查 B 的 M1）：改前就有，属于 12.4 的对比度，小、可修。`dark:hover:bg-destructive/50`，3.45 → 8.07；PG4 (page) 钉住两种主题的静止与悬停；4.3.4 的表加两行。别的按钮变体静止与悬停按 4.3.4 推都远高于 4.5，不另量。
- **592 钉住 Playwright 项目的检查**（审查 C 的 M1、N1、N2；P2 设计 5.1、决定 421）：`REQUIRED` 列 MC3、PG2 之外再列 SE1；经 `pnpm --filter @nervewiki/e2e exec playwright` 列格，不用 knip 的 `ignoreBinaries`；检查看的是各项目列出的文件与格，看不见引擎，注释与报错照此写。S4 的 PG15 一条照旧在 `REQUIRED` 加一行。
- **593 13.2 的新条是第 31 条**（审查 C 的 M2）：照决定 505、536，P4/S4 先合并、取了第 30 条，P2/S3 接在它之后；P2 设计 2.1 第 3 条的第 21 条、决定 396、4.3.0 与 8.2 的号随之改，决定 396 与 00 号文档第 7 节第 21 条只改号、注明。
- **594 文档的去处与收口**（核对第 1 轮 C 的 C1-N1，审查 A 的 N2、Q1）：
  - 记录叫 `reviews/P2-S3-narrow-bdi-contrast-review.md`；S3 没有要读屏清单新收的，真的 Safari 上的两项写在本记录"读屏与人工清单"，P5 设计 3.19 的人工入口加它的链接。
  - P2 设计正文里与实施不同的各处照实改、注"实际…（10.3）"，决定 396、413 只加注。
  - 设计 32.4 举的两个乱序的例子三种引擎里都不乱：照实写进 10.3 与本记录；已推的提交说明不改写，不准处写在"更正"（同决定 547、554、571、579、583、588）。
  - 05-lessons 第 6 节加两条：量计算之后的样式之前先结束过渡（设计 6.3 第 7 条，连同祖先）；WebKit 把原生选择显示的文字算进页宽。
  - P2/S2 先合并；S3 照决定 378 合并 `main`，`history-filters.tsx` 的冲突照合并检查单解，两边都留。
  - 核对第 1 轮三位都没有 Important、Minor，一条 Nit 在本提交写对，不开第二轮。

## 实施中定的

实施报告第 7、8 节、修复报告第 7 节与 P2 设计 10.3 的"与设计不同之处"（三位审查者各在自己的范围里判过；实施者没有要编号的产品决定）：

- 提交的标题照 6.3，括号里的条目号挪进正文（S1 的写法）。
- `isolated.tsx` 导出 `Namer` 类型与缺省的 `same`，五个文件共用；`changesetName` 的收集器参数叫 `by`，`changeText`、`restoreToText` 与 `history-row.tsx` 的回调叫 `named`（同一作用域里已有 `name`，oxlint 的 `no-shadow`）。`isolated.test.tsx` 另有"没有名字的一句照原样"一格。
- `check.mjs` 比 5.1 多查：一次列四个项目；"在 `threeBrowsers` 里"按 `webkit` 列出的文件判；带 `@narrow`、不在 `threeBrowsers` 里的每一格都要在 `webkit-narrow` 里；先在样例上自检（照 `tools/e2e-navigation/check.mjs` 的先例）。
- `E/fixtures/contrast.ts` 多一个导出 `setColorScheme(page, scheme)`（切主题并等 `html` 的 class）；结束的是元素与它祖先的 `CSSTransition`（底色也可能在过渡里），无限的动画不碰。
- SE1 拆出的那一格用 `expectFits` 量"在哪里找"的外层与第一个命中，先断言它显示的是那本笔记本；不直接量选择（WebKit 的选择 `scrollWidth` 比 `clientWidth` 大 1，与超宽无关）。
- PG2、LW3 关对话框用 Escape；PG2 的 1280 一步先把视口设成 1280 × 800。
- `AlertDialogContent` 不加 `grid-cols-[minmax(0,1fr)]`：加了标题的 `anywhere` 之后七个对话框都不超出，没有一格抓得到（照设计）。
- 修复第 1 轮：F1 的自检样例加一种破坏；F2 连报错一起改（"…but not listed"）；F3 在 PG4 (page) 加量的一段；F4 加一格 vitest 给它先红。第一次的 F1、F3 在逐个提交的检查里 `make lint-web` 红（样例里 `map` 中展开对象，PG4 在循环里 `await`），都没推，从 `72b1e679` 重做；新旧两份只差这两处。
- 合并 `main`（`e19172e3`）：`history-filters.tsx` 里 `PageFilter` 的一行，S2 把它改成收节点的 `label`（页或附件，`Remove` 的可访问名由它来），S3 给 `p`、`span` 加了窄屏的类；两边都留：`p` 是 `flex min-h-9 max-w-full min-w-0 …`，`span` 是 `min-w-0 break-words`，里面是 `{label}`。"谁""客户端"两个外层的类自动合进。合并之后的树与 `main` 只差 S3 的改动（40 个文件，与 `f08dca69..f5d1220a` 相同）。S2 没改 S3 整句断言用到的键。

## 接受与推后的

- **接受**：
  - `contrast.ts` 从计算之后的样式取外框与环，被祖先的 `overflow` 或 `contain` 裁掉时照样过；外框落在父元素之外、底色不同时取的底不对。今天量到的底都是页面色，是设计选的量法的边界（审查 B）。
  - `check.mjs` 的键只用文件与格的标题，不带 `describe` 的路径：同一个文件里两个 `describe` 下同名、带标签的格会合成一个键（审查 C，没有造出来）。
  - 亮色 destructive 悬停 4.54 只比 4.5 高一点，有 PG4 钉着，不改（审查 B）。
- **推后进 v0.2**：WebKit 原生下拉的焦点指示（决定 590）；26.12、32.4 的同类约二十处（决定 395）；变更集页面包屑的 `current` 与回收站预览 `h1` 的长名（决定 414）。
- **e2e 走不到、照实记下的**（12.4 与决定 420 预先写明）：`DropdownMenuRadioItem` 的环（主题、语言菜单）、快速切换的当前项、`tabs.tsx` 两处、工作区首页的笔记本卡片、任务行、`text-diff.tsx` 的按钮、destructive 按钮的环与暗色底。改了，没有故事读它们的颜色；审查 B 的探针补了快速切换当前项的读数（亮 4.35、暗 3.19）。
- **未核实的怀疑**（审查者的，没有造出来）：
  - 真的 Safari 上 `contain-paint` 是否同样让 33.1 回到 375（iOS 用系统的选择器），是否也丢下拉的 `box-shadow`（审查 B；交 P5/S3）。
  - 真的读屏软件念到 `bdi` 时会不会停顿；Firefox、WebKit 的可访问名称没有经它们自己的 API 读（审查 A）。
  - zh 的 "MCP（עוזר） · 5 分钟前"、"从「A」移到「B」"没有放进 bidi 探针，按 N0 与"汉字是强 L"推断不乱（审查 A）。

## 更正

已推送的提交说明不改写（决定 594）；下面照审查与修复改准。

**实施报告**：

- 第 7 节第 3 条"`REQUIRED` 没把 SE1 列进去……完整性那一条已管它"：不实，完整性那一条看不见丢了标签的格（审查 C 的 M1）。
- 第 5.1、9 节"WebKit 里原生选择看不到焦点环"、"截图里只有原生的灰边框"：没有 3 px 的环，聚焦时 1 px 的边框变成 `#737373`，截图里那条深灰的边框就是它（审查 B 的 N1）。
- 第 7 节第 1 条"外层 `overflow: hidden` 会裁掉焦点环（推断，没有试）"：审查 B 核实，Chromium、Firefox 左边 3 px 的环整个没了。
- 第 9 节"S2 若改……整句断言要跟着改"：对 S2 的 `4c9f1212` 与合并进来的 `6505c3bf`，一格也不用改（审查 A 的 Q1，合并时照实核过）。
- 第 7 节第 2 条（`knip.jsonc` 的 `ignoreBinaries`）：修复第 1 轮改用 `pnpm --filter` 之后已不是偏离（审查 C 的 N1）。

**设计**：32.4 举的两个乱序的例子三种引擎里都不乱（审查 A 的 N2，P2 设计 10.3）；4.3.4 的暗色 destructive 静止写 6.47（`#9d4042`），Chromium 读回 `#9e4042`、6.43，8 位取整差一位（审查 B）。

**已推的提交说明**：

- `9ca1c6a1`："Moved from “A” to “B” between two Hebrew names read from where to where backwards"：三种引擎里没有 `bdi` 也不乱（两个名字之间有强 L 的 "to"）；确实乱的是删除者一行、搜索的路径、改名与 zh 的撤销链接（审查 A 的 N2）。
- `ceb0482b`："webkit-narrow runs every cell tagged @narrow …; it first checks itself on samples of each breach"、"knip takes … playwright as e2e's binary"：检查看的是列出的格，看不见引擎；丢了标签的格要 `REQUIRED` 列出才看得见，那一种样例与 SE1 那一格修复第 1 轮才加（`1d8a2fd9`）；knip 的忽略随 `pnpm --filter` 去掉（`63004369`）（审查 C 的 M1、N1、N2）。
- `72b1e679`："NativeSelect's paint is contained (contain-paint): every native select has it, its own ring and border not being its content"：Chromium、Firefox 里环与边框照画；WebKit 本来就不画选择的 `box-shadow`（审查 B 的 N1）。

## 反向对照

变异都经 `heavy.sh`，在实施者的工作树上，`mut.py`（`docs/v0.1/claude_to_codex/tools/mut.py` 的思路，改一处或几处、跑命令、从备份复原，不用 `git checkout --`）；先在没改的树上跑同一组命令的基线（构建 0；vitest 五个文件 144 过；e2e 五个故事的相关格 11 过；SE1 (page) `--repeat-each=5` 15 过；`check.mjs` 0）；改 `web/` 的 e2e 变异各自先构建；之后工作树 0 处改动。

**实施：52 条**（vitest 15、e2e 37），在 `72b1e679` 上：

| 组 | 条数 | 结果 | 变异 → 红在哪 |
|---|---|---|---|
| `bdi`（vitest） | 15 | 都抓到 | `Isolated` 用 `span` 代 `bdi` → 五个文件 7 格；每个名字都记成 0 号 → 95 格；`pathOf`、删除者、来处、恢复到、客户端名、作者名、改名的旧名、改动的节点名、两处"撤销了"、节点改名两端、移动的父页名各不经收集器，移动一句的第二个名字自己 `isolate` → 各红在它那一页的 `bdi` 格 |
| 宽度（e2e） | 17 | 都抓到 | 历史两个外层、`PageFilter` 的 `p` 与名字、两个链接、客户端一行、节点链接、已删节点、改动的那一句、面包屑各去掉自己的类 → MC3 的 `@narrow` 格在 chromium 与 `webkit-narrow` 都红（`mc3:202`、`:214`、`:220`、`:226`、`:233`）；面包屑、页标题、`DialogTitle`、`AlertDialogTitle` → PG2 两个项目（`pg2:209`、`:214`、`:216`），页标题与 `DialogTitle` 另 → LW3 (page) 三种浏览器（`lw3:346`、`:357`） |
| 对比度（e2e） | 10 | 都抓到 | 去掉 base 的 `:focus-visible`（X5）→ `se1:305`（`outline auto`）；按钮、输入框、选择的环回到 `/50` → `se1:294`、`:290`、`:299`；亮色 `--ring` 回到 0.708 → SE1、L5 各一条；`--muted-foreground` 回到 0.556 → `l5:133`（4.35）；无效的环回到 `/20` → `se1:371`；菜单项、补全选中项去掉内嵌的环 → `l5:177`、`:136` |
| 命中（e2e） | 2 | 都抓到 | `mark` 回到 `font-semibold` → `se1:277`；暗色高亮调亮到 0.75 → `se1:287` |
| 33.1（e2e） | 1 | 抓到 | `NativeSelect` 去掉 `contain-paint` → 只在 `webkit-narrow` 红（`se1:399` 页宽），chromium 照过 |
| 项目的检查 | 5 | 4 抓到，1 等价 | `grep` 写错（X6）、写成 `/\(page\)/`、删掉 `webkit-narrow`、PG2 的格不带标签 → `check.mjs` 红；去掉 `testIgnore` 照过、等价：今天 `threeBrowsers` 的故事里没有一格带 `@narrow`，跑的格不变；以后有人给那里的一格加标签，"webkit runs its story already"会红（审查 C 的 `cx-no-testIgnore-lw3-tagged` 证实） |
| 夹具 | 1 | 抓到 | `contrast.ts` 不结束过渡 → SE1 (page) 在 chromium 五遍都红（环读成 `ring 0.098px`、1.03，或"no focus indicator"）；第一次跑在上一条的构建上，改了 `mut.py` 在原树的构建上重跑 |
| 走不到的七处（合成一条） | 1 | 照过 | 快速切换当前项、`tabs.tsx` 两处、工作区首页卡片、任务行、`text-diff.tsx`、destructive 的环与暗色底、`DropdownMenuRadioItem` → 全量 582 过、1 跳过（12.4 与决定 420 预先写明没有 e2e 走到） |

**审查**：

- A：按出现先后取名字（`names[(i - 1) / 2]`）→ `history-page` 4 格红（`changeText` 的改名先取新名的 slot，译文里 slot 倒着出现），探针的 `PROBE-reverse` 也红：语序不同不会错位，今天有测试钉住。
- B：同一构建上注入样式去掉 `contain-paint` → WebKit 704、Chromium 与 Firefox 375；注入 `*:has(> select){overflow:hidden}` → Chromium、Firefox 的焦点环左边 3 px 没了。
- C：`cx-se1-untagged` 照过（→ M1）；`cx-narrow-on-chrome` 照过（→ N2）；`cx-grep-wrong` 红；`cx-no-testIgnore` 照过、`cx-no-testIgnore-lw3-tagged` 红、`cx-lw3-tagged-control` 照过（等价站得住）；`cx-X5-no-focus-visible` 红在 `se1:305`；`cx-w-page-filter-span` 两个项目红在 `mc3:214`；knip 去掉 `ignoreBinaries` 报 `Unlisted binaries`，改 `--filter` 之后 0。

**修复第 1 轮**（在头 `f5d1220a` 上，各从副本改、跑、复原，`fix1/tools/rc.py`；先红见修复报告第 1 节）：

| 对照 | 对应 | 结果 |
|---|---|---|
| 去掉 SE1 那一格的 `@narrow` | C-M1 | `check.mjs` 红：`webkit-narrow search/se1-search.spec.ts: lists no cell "SE1 (page): at 375 wide…"`（修之前照过） |
| 去掉 `dark:hover:bg-destructive/50`，构建 | B-M1 | PG4 (page) 红在 `dark under the pointer`，3.45；先红时四个读数都在消息里 |
| `key={part}` | A-N1 | 新格红在 React 的 "Encountered two children with the same key, `0`" |
| `--dir e2e`、不带 `ignoreBinaries` | C-N1 | knip 报 `Unlisted binaries`（审查 C 已做）；`--filter` 之后 knip 0，`check.mjs` 的输出逐字相同 |

核对第 1 轮：A 在头上把 `key={i}` 改回 `key={part}`，新格红；B 用第一轮的探针在 `f5d1220a` 的构建上重量 destructive，读数与修复报告相同，PG4 (page) `--repeat-each=2` 都过；C 在头上删 SE1 的标签，`check.mjs` 红，把 `--filter` 的包名写错（`c1-filter-matches-nothing`），pnpm 以 0 退出、`JSON.parse` 抛错，检查照样失败。

## 核对

**核对第 1 轮**（2026-11-18，对象 `f5d1220a`）：三位的旧发现都关上，都没有 Important、Minor；新的只有 C 的一条 Nit（C1-N1），在本提交写对，不开第二轮（决定 594）。

- **A**：N1、N2、Q1 关上。`key={i}` 与设计相同、带 lint 的例外；S3 的五个 vitest 文件 145 格过；语言切换时按下标复用 `bdi`，`bdi` 没有状态，不受影响；修复的另三个提交不碰拼句的文件。
- **B**：M1、N1、Q1 关上。用到 destructive 的 8 处都在页面或对话框的 `bg-background` 上，没有一处另传底色；`disabled`、`aria-disabled` 与 `ring-destructive` 不受影响；修复没有碰 `NativeSelect`、宽度的类与 `styles.css`。
- **C**：M1、M2、N1、N2、N3 关上。头上 `make lint-web`、`make knip` 都是 0，输出与改前逐字相同；修复报告里 F1–F4 的逐个提交都绿。新的：C1-N1。

## 门禁与 CI

- **逐个提交**（`make lint-web`、`make knip`、`make test-web`，经 `heavy.sh`，在干净的 worktree 里 `checkout --detach`，先后断言没有改动；改了故事的另 `make build` 与那几个故事）：

  | 提交 | test-web 的格数 | 另跑的 |
  |---|---|---|
  | `9ca1c6a1` | 4,702 | — |
  | `76be71df` | 4,702 | MC3 3 格过 |
  | `ceb0482b` | 4,702 | PG2、LW3 32 格过；`check.mjs`：chromium 409、firefox 85、webkit 85、webkit-narrow 2 |
  | `468635fe` | 4,702 | SE1、L5 7 格过 |
  | `f5131dd3` | 4,702 | SE1 3 格过 |
  | `72b1e679` | 4,702 | SE1 5 格过；`check.mjs` 的 webkit-narrow 3 |
  | `1d8a2fd9` | 4,702 | 两个 e2e 的检查 0 |
  | `63004369` | 4,702 | 同上，输出与 F1 逐字相同；knip 0 |
  | `3c4d5710` | 4,702 | PG4 3 格过 |
  | `f5d1220a` | 4,703 | — |

  lint-web 与 knip 每个都是 0（第一次的 F1、F3 红过，见"实施中定的"）。先红：每个提交的新测试在没修的代码上红过（实施报告第 2 节、修复报告第 1 节）；审查 C 在自己的工作树里重跑了前四个提交，都是 0、格数相同。

- **合并之前的门禁**（经 `heavy.sh`，之前判 `df`，可用 31–35G）：

  | 步 | 实施 `72b1e679` | 修复第 1 轮 `f5d1220a` | 合并 `main` 的 `e19172e3` |
  |---|---|---|---|
  | `make check` | 0（约 64 分钟；golangci-lint 0 issues，Go 285 个包 `ok`，vitest 196 个文件 4,702 格） | 0（3,974 秒；Go 285 个包，vitest 196 个文件 4,703 格） | 只跑 `make lint-web`、`make knip`、`make test-web`：0（197 个文件、4,801 格；主持者的说明：Go 已过 `main` 的 CI） |
  | S2、S3 都改过的故事 | — | — | TB1、MC3、SE1 全部项目 11 格过，含 `webkit-narrow` 的 MC3、SE1 |
  | `tools/e2e-navigation/check.mjs`、`tools/e2e-projects/check.mjs` | 0、0 | 0、0 | 0、0（chromium 410、firefox 85、webkit 85、webkit-narrow 3） |
  | `make e2e` | 0（582 个通过、1 个照旧跳过：WebKit 的 A15 第 3 格；166 秒） | 0（582、1，173 秒） | 0（582、1，167 秒） |

- **分支的 CI**（`ci.yml` 每次 19 项）：

  | 头 | run | 结果 | web（Test） | e2e（E2E 一步） |
  |---|---|---|---|---|
  | `72b1e679` | 44252268978 | 第一次 attempt 18 项绿，e2e 卡在装 Playwright 的浏览器（apt 的镜像慢），20 分钟被时限取消；只重跑 e2e，第 2 次绿，19 项全绿 | 3:58（3:13） | 9:40（8:02，装浏览器 59 秒），582、1 |
  | `f5d1220a` | 44277971443 | 第一次 attempt 18 项绿，e2e 同样卡住（装浏览器 17 分钟，E2E 跑了 2:15、177 格过）被取消；主持者只重跑 e2e，第 2 次绿 | 3:56（3:18） | 9:47（8:30，装浏览器 44 秒），582、1 |
  | `e19172e3`（合并 `main`） | 44291042215 | 第一次 attempt 18 项绿；e2e 整 20 分钟都在装 Playwright 的浏览器（apt 的镜像慢），被时限取消，E2E 一步没有开始；只重跑 e2e，第 2 次绿，19 项全绿 | 4:18（3:41） | 第一次被取消（装浏览器 19:39）；第 2 次 9:32（E2E 8:07），582、1 |

  `f08dca69` 的 `main`（run 44224495963）：web 3:51（3:23）；e2e 10:02（8:15），576、1。离 e2e 作业 20 分钟、web 15 分钟的时限都远（P2 设计 7.4）。
- **本提交的分支 CI**：`496f2043` 的 run 44298535841：第一次 attempt 18 项绿，e2e 在装 Playwright 的浏览器时被 20 分钟的时限取消，只重跑 e2e，第二次绿（装浏览器 0:46，E2E 一步 7:23）。S3 合并为 `e639b9f7`，之后第一次 `main` 的 run 44305876925 第一次 attempt 19 项全绿（`web` 4:42，Test 4:03；e2e 9:06，E2E 一步 7:36）。

## e2e 的用时

S3 新加与改到的格，照 CI 的 `list` 报告逐格读（秒；审查 C 用 `gh` 只读核对过实施那一次）：

| 格 | 项目 | `f08dca69` 的 `main`（44224495963） | `72b1e679`（44252268978 第 2 次） | `f5d1220a`（44277971443 第 2 次） | 4.3.0.3 的推断 |
|---|---|---|---|---|---|
| MC3 (page): at 375（新） | chromium / **webkit-narrow** | — | 2.2 / 4.3 | 2.3 / 4.5 | 合约 3.5 |
| PG2 (page): at 375（新） | chromium / **webkit-narrow** | — | 1.9 / 4.6 | 2.1 / 4.9 | 6 / 8 |
| SE1 (page): at 375（新，33.1） | chromium / **webkit-narrow** | — | 0.9 / 2.0 | 1.1 / 2.0 | 合约 6 |
| LW3 (page): at 375 wide a long source（改） | chromium / firefox / webkit | 2.3 / 3.9 / 5.6 | 3.3 / 5.4 / 7.0 | 3.5 / 6.4 / 8.3 | 7.5（合） |
| SE1 (page): searched …（改；375 一段移出） | chromium | 2.6 | 3.3 | 3.6 | 2.5 |
| L5 (page): [[ completes …（改） | chromium | 3.2 | 4.3 | 4.1 | 2 |
| PG4 (page)（修复第 1 轮改） | chromium | 1.5 | 1.3 | 1.9 | — |

- 实施：新加的六格 15.9 秒，其中**新的窄屏项目 `webkit-narrow` 三格 10.9 秒**、chromium 三格 5.0 秒；改的三个故事比 `main` 多 5.7 秒；合计多约 21.6 test-seconds，是 4.3.0.3 推断（32–38，33.1 复现再加约 6）的一半上下，PG2 的五个对话框比估的快得多。`webkit-narrow` 的 worker 启动不在格里，没有单独量到。
- 修复第 1 轮：PG4 (page) 多约 0.4 秒（推断 0；比 `main` 的 1.5）。同一些格在两次运行之间差一两成，是抖动。
- e2e 作业 9:40、9:47（E2E 8:02、8:30），与 `main` 的 10:02（8:15）相当；折成墙钟 S3 多约 11–14 秒，在 7.4 写的 15–45 秒之下。本机 `make e2e` 166–173 秒。

## 读屏与人工清单（交 P5/S3）

- **读屏**：S3 不改可读的名字与描述：加了 `bdi` 的句子整句的 `textContent` 不变（vitest 按整句断言），复制的文字、`innerText` 与 Chromium 可访问性树的名称不变（审查 A 的探针）；`contain-paint` 只是样式。没有要读屏清单新收的；真的读屏软件念到 `bdi` 时会不会停顿没有核对（审查 A），可以随第 9 节的八项顺带听一遍。
- **人工（真的 Safari，iPhone 或 macOS 的窄窗口，发布之后）**（决定 590）：
  - 搜索页在 375 宽左右、"在哪里找"选着一本名字很长的笔记本时，页面不横向滚动（33.1，Playwright 的 WebKit 上已修）。
  - 原生下拉（搜索的"在哪里找"、历史的筛选）用键盘取得焦点时，焦点看得出来（Playwright 的 WebKit 只有 1 px 的 `ring` 色边框）。

## 环境

实施、审查与核对各在自己的工作树里（实施者 `nerve-wiki-m12-p2-s3`，审查者见表头），`TMPDIR`、`GOTMPDIR` 与原始日志在 `~/.cache` 下各自的目录，锁固定在 `/tmp`，用完删掉；探针（33.1 的 `probe-33.1/`、页面树外框的 `probe-tree/`、选择的 `probe-select/`、destructive 的 `fix1/probe/`，审查 A 的 `zz-review-a.test.tsx` 与 `bidi-probe.mjs`，审查 B 的 `b-probe.spec.ts`）从没提交。

- 实施者在干净的 worktree `~/.cache/m12-p2-s3/wt-verify` 里逐个提交核对，用完删了；修复第 1 轮的逐个提交、反向对照、`make check` 与 `make e2e` 在一次 `heavy.sh` 里（里面不再取锁）。
- P2/S2 的修复、几条旁支与审查同时在跑，用时含在锁上排队的时间。要留的汇总在设计工作目录的 `m12/p2s3/`（`logs/`、`tools/`、`fix1/`、`finish/`、`review/`、`docs/`），实施报告 `impl-report.md`、修复报告 `fix-1-report.md`；本记录与文档的改动由脚本（`p2s3/docs/apply_p2s3_docs.py`）写出。
