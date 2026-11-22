# M13/P1 设计审查记录

| 项 | 内容 |
|---|---|
| 对象 | [01-P1-foundation.md](../01-P1-foundation.md) 的初稿（2026-11-22，未提交；`m13-ui-refresh` 的头 `65dbfcbb`，代码与 `3c315037` 相同） |
| 方法 | 三位审查者并行，只读，各自在会话的临时目录里跑探针（不进仓库）：A 管 S1（token、`accent` 的分开改、焦点环、危险按钮、守约定的 vitest、对比度的守卫、8.2 的新文字）；B 管 S2 与外观（组件、动画、下拉、标签页，对照样稿，本机缓存里旧版的 Chromium 截图与实量）；C 管测试、变异、e2e、完成度与约定（每个提交单独为绿、决定 746 的本机门禁、2.1 的修订）。审查者报经过核实的发现，另列未核实的怀疑、核对过没有问题的与过度设计（02-workflow 第 0 节） |
| 结论 | A：2 Important、7 Minor、7 Nit；B：0 Important、8 Minor、9 Nit、3 Q；C：2 Important、11 Minor、10 Nit、2 Q。四条 Important 是两对同一个问题：`--ring` 与 `--accent` 各写一个值，调淡琥珀没有测试变红（A-I1、C-I2）；vitest 里 `styles.css?raw` 是空串，守约定的测试比的是两个空集合（A-I2、C-I1）。主持者照下表定处置，修订进本文（主持者的决定 747–762，在 01-P1 2.3） |

## 审查 A（S1）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| RA-I1 | Important | `--ring` 与 `--accent` 各写十六进制，三个故事量的都是 `--ring`；调淡 `--accent` 没有测试变红，00-M13 4.8 要求的变异落空；`--faint` 在 S1 里也没人量 | 接受：`--ring: var(--accent)`，两种主题都写（决定 755）；7.2 第 1 条随之咬得到；第 31 条的新文字写明 `--faint` 不量 |
| RA-I2 | Important | vitest 5.0.2 的 `css-empty-post` 把 `styles.css?raw` 换成空串，测试 1 比两个空集合，测试 2 的 CSS 一半没读，变异 7 抓不到 | 接受：`readFileSync`，先断言读到了东西（长度、已知的名字、文件数与 `button.tsx`），提交之前先做变异 7（决定 751，4.1.7） |
| RA-M1 | Minor | S2 在 `@media` 里加的 `:root` 会让测试 1 变红 | 接受：只取顶层的两块（4.1.7）；S2 的文件表写明测试不改 |
| RA-M2 | Minor | 当前项悬停时 `hover:bg-hover` 压过 `bg-active`，底反而变淡 | 接受：当前项不加悬停（4.1.3） |
| RA-M3 | Minor | P2 把搜索框挪到侧栏上，se1 的下限必红 | 接受：之后改了被量元素的底的 Step 重量（决定 748，2.1 第 5 条改 00-M13 4.7，第 9 节 P2） |
| RA-M4 | Minor | 8.2 的新文字有几处在 S1 合并时不成立（动效、当前页的 `--accent-soft`、编辑器的链接色与匹配色、"同源"的注释） | 接受：照 S1 的实际改写，S2 的改动另列 8.2 第 6 条 |
| RA-M5 | Minor | 字号不做变量，偏离 00-M13 | 接受：两个常用字号 `--text-compact`、`--text-row` 进 `@theme`，标题的由用到的 Step 定（决定 752，2.1 第 4 条） |
| RA-M6 | Minor | 三处 `bg-muted` 从不产出 CSS | 接受：改成 `bg-surface-2`，测试 5 守住颜色类都有映射 |
| RA-M7 | Minor | 测试 3 抓不到 TS 样式对象里的 `var(--accent)` 底 | 接受：测试 3 加一条正则 |
| RA-N1 | Nit | 变异 6 的 l5 抓不到（阅读视图的链接有自己的外框） | 接受：7.2 第 6 条改写 |
| RA-N2 | Nit | 暗色悬停是 5.40 | 接受 |
| RA-N3 | Nit | `bg-background` 的行号是 `dialog.tsx:15`、`alert-dialog.tsx:19` | 接受 |
| RA-N4 | Nit | `grep` 的剩余项漏了新测试 | 接受 |
| RA-N5 | Nit | tailwind-merge 不认新的圆角与阴影 | 接受，并入 B-M7、C-M7：S1 登记（决定 756） |
| RA-N6 | Nit | S1 之后、S2 之前，暗色对话框里的描边按钮比面暗 | 接受：描边按钮的底挪进 S1（决定 748、757） |
| RA-N7 | Nit | 两处子串匹配不改也照绿 | 接受：三处都按词认 |

## 审查 B（S2 与外观）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| RB-M1 | Minor | 暗色下描边按钮悬停反而变暗（半透明的 `--hover` 顶替了不透明的底） | 接受：`hover-layer` 叠一层（决定 757），进 S1 |
| RB-M2 | Minor | 分段式标签页暗色下当前项比轨道还暗 | 接受：改回下划线式（决定 754 改写） |
| RB-M3 | Minor | 英文 375 px 下标签折成两行，胶囊轨道包成大胶囊 | 接受：同上 |
| RB-M4 | Minor | 输入框与下拉的底与 00-M13、样稿不同 | 接受：`--surface-2`，聚焦时 `--surface`（决定 759） |
| RB-M5 | Minor | 箭头是实心三角，样稿是描边的折线 | 接受：四停点的两道渐变，`--faint`（决定 753） |
| RB-M6 | Minor | 读者看到的任务复选框是禁用的，Chromium 不给它琥珀 | 接受：改正 4.2.5 的说法，已勾的禁用项交给 P4/S2 |
| RB-M7 | Minor | tailwind-merge 不认新名字 | 接受：S1 登记（决定 756） |
| RB-M8 | Minor | 令牌期限漏了 `aria-invalid` 与 `w-full`，测试的出处写错；`NativeSelect` 没有无效的样子 | 接受：三处补上，`native-select.test.tsx` 加格 |
| RB-N1 | Nit | 菜单项、`Label` 的类名只写了改动的部分 | 接受：4.2 写完整的类名 |
| RB-N2 | Nit | `tone`、`shape` 要从 props 里解构掉 | 接受：`tone` 解构、测试断言不落到 DOM；`shape` 挪到 P2/S1 |
| RB-N3 | Nit | `[&_svg]` 把单选项的勾压成灰 | 接受：`[&>svg]` |
| RB-N4 | Nit | 菜单内边距、分隔线、宽度、按钮间距与样稿的细差 | 接受：照样稿（`p-1.25`、`mx-1.5`、`min-w-[220px]`、`gap-1.5`）；30 px 按钮的图标随 P2/S1 |
| RB-N5 | Nit | 菜单从自己的中心缩放 | 接受：`origin-(--radix-dropdown-menu-content-transform-origin)` |
| RB-N6 | Nit | 强制色模式下右边距为 0 | 接受：写明 `0.5rem` |
| RB-N7 | Nit | 头表的 settings.html"按钮的 `sm`"引错 | 接受：头表改写（30 px 挪到 P2/S1） |
| RB-N8 | Nit | 暗色遮罩比原来重 | 接受：45%（决定 762） |
| RB-N9 | Nit | 截图清单要补几样 | 接受：7.4 |
| RB-Q1 | Q | 本机没有 Playwright 1.63 的三个浏览器，三种引擎的核对与 `make e2e` 由谁、在哪里做 | 第 6 节第 0 步：开工之前问负责人能否下载；不能时停下汇报（决定 761） |
| RB-Q2 | Q | 鼠标移过菜单项时也出琥珀的环，负责人是否接受 | 主持者定：只在键盘操作时出现（决定 760，`app/input-modality.ts`），合 00-M13 4.7 与样稿；2.1 第 6 条改 00-M13 4.3 |
| RB-Q3 | Q | 暗色菜单的底照样稿还是照 00-M13 | 照样稿 `#25262a`（决定 758，2.1 第 4 条改 00-M13 4.1 的表） |
| 过度设计 | — | `shape`、`icon-sm`（与 `sm`）在 S2 没有调用处 | 接受：挪到 P2/S1（决定 747，2.1 第 2 条） |

## 审查 C（测试、e2e、完成度与约定）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| RC-I1 | Important | 同 RA-I2 | 同 RA-I2 |
| RC-I2 | Important | 同 RA-I1 | 同 RA-I1 |
| RC-M1 | Minor | 每个提交的门禁漏了 knip，`lint-web` 的描述不对 | 接受：第 6 节与 7.3 |
| RC-M2 | Minor | 本机没有 `CI`，留着的 `test.only` 照绿；漏了 `gen-check` | 接受：`CI=1`、`gen-check-web`；没有重现的 CI 项列表（决定 761，7.3） |
| RC-M3 | Minor | 本机跑不了门禁，没有基线这一步 | 接受：第 6 节第 0 步（浏览器、`make build`、基线） |
| RC-M4 | Minor | 同 RA-M1 | 同 RA-M1 |
| RC-M5 | Minor | `aria-disabled` 一格只用 `focus()` 判，抓不到变异 15 | 接受：`user.tab()` 与类名（7.1） |
| RC-M6 | Minor | 第 31 条的新文字说多了（`--faint` 不量；菜单项的"只在键盘"不成立） | 接受：照实改写；菜单项由决定 760 做到，S2 改那句（8.2 第 6 条） |
| RC-M7 | Minor | 同 RB-M7 | 同 RB-M7 |
| RC-M8 | Minor | 同 RA-M6 | 同 RA-M6 |
| RC-M9 | Minor | 没有处理 M12-polish 的移交 | 接受：认领第 5 项的 WebKit 下拉一小项，第 11 项（真的 Safari）不认领（第 2 节"移交"） |
| RC-M10 | Minor | "减少动效时只淡入"没有人核对 | 接受：S2 的探针（7.4） |
| RC-M11 | Minor | 只有四个组件有 vitest | 接受：其余五个由现有的测试覆盖，逐个写出（7.1） |
| RC-N1 | Nit | 同 RA-N1 | 同 RA-N1 |
| RC-N2 | Nit | l5 那一帧的检查要放在 Escape 与 `toBeHidden` 之间；写明进哪个提交 | 接受：第 5 节、6.2 提交 2（`web, e2e`） |
| RC-N3 | Nit | 同 RA-N3 | 同 RA-N3 |
| RC-N4 | Nit | se1:279 的"3:1 at least"也要改 | 接受：4.1.8 |
| RC-N5 | Nit | 8.2 的几处不准（动效、`--radius-md` 的位置、第 15 节的写法、S2 的一行） | 接受 |
| RC-N6 | Nit | 2.1 的几处不一致或不全（菜单的面、阴影、锚点与新文字、第 12、13 节、P2/S1 一格） | 接受：2.1 改写 |
| RC-N7 | Nit | 缺"CI 的余量" | 接受：7.5 |
| RC-N8 | Nit | "两个 `check.mjs`"重复 | 接受 |
| RC-N9 | Nit | 测试 4 抓不到任意的色值 | 接受：测试 4 加任意值与样式对象的色值，写明例外 |
| RC-N10 | Nit | 没有东西守住 3 px → 2 px | 接受：se1 断言 `ring 2px`（变异 19） |
| RC-Q1 | Q | 负责人推送时只有最后一个头有 CI | 主持者定：接受只有最后推上去的头有 CI，每个合并的头另有本机门禁的记录（决定 761） |
| RC-Q2 | Q | WebKit 的结论来自 macOS 的 Playwright，CI 是 Linux | 主持者定：句子与记录注明平台，CI 补跑之后再核（4.2.10） |
| 过度设计 | — | 每次合并都跑完整的 `make check`（含全部 Go 测试） | 接受：Go 一侧不重跑，断言 Go 的树没变（决定 761） |

## 未核实的怀疑（审查者列出，主持者的去向）

- Chromium 的画布读 `color-mix` 的计算值（A）：S1 的实量会直接碰到，读不出时 `contrast.ts` 抛错，不会猜。
- WebKit 在 `appearance: none` 之后是否画环、是否仍把显示的文字算进页面宽度，Firefox 的箭头与强制色（B、C）：4.2.10。
- 下拉的 iOS 缩放（B）：照输入框加 `text-base md:text-sm`（4.2.4）。
- macOS 与 Linux 的 Chromium 量出同样的值（C）：CI 补跑时核。
- 危险按钮描边的透明度 30% 与 35%（A）：取 settings.html 与本文的 35%。

## 主持者的决定

747–762，全文在 [01-P1](../01-P1-foundation.md) 2.3。

## 修订核对第 1 轮

三位核对各自的发现，并看修订有没有带进新的问题（同样只读，探针在会话的临时目录）。上一轮的发现都已改到（RA-I2、RC-I1 部分、RB-N1、RC-M6 部分，余下的进下表）。新的：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| RC1-I1（= RA1-M2、RB1-M1） | Important | 测试 5 把 `text-compact`、`text-row` 当成颜色，S2 的提交 1、2 照写必红，而 S2 写明不改这个测试 | 接受：`text-NAME` 另接受 `@theme` 里的 `--text-NAME`，S1 就写好（4.1.7） |
| RA1-M1（= RC1-M1） | Minor | `new URL(…, import.meta.url)` 在 vitest 里被改写成 http 地址（jsdom 的 `URL`），`readFileSync` 抛错 | 接受：照先例 `import { URL as FileURL } from "node:url"` |
| RA1-M3（= RC1-M2） | Minor | 测试 1 的前置断言在没改的树上就红，与"通过"不符；变异 7 在旧树上无从做起 | 接受：写明旧树上它因前置断言而红；变异 7 挪到改完 `styles.css` 之后 |
| RA1-M4 | Minor | "多于 400 个文件"在去掉测试之后不成立（299） | 接受：去掉之后多于 250 |
| RB1-M2 | Minor | 用鼠标时单按修饰键，鼠标停着的菜单项出琥珀环 | 接受：照 WICG focus-visible 的规则，带 Meta、Ctrl、Alt 的按键与单独的修饰键不算键盘；测试与变异 23 |
| RC1-M3 | Minor | 变异 21 在 S1 里没有测试守，截图也看不出 | 接受：S1 新建 `button.test.tsx` 的第一格（描边按钮的类名） |
| RA1-N1（= RC1-N2） | Nit | 测试 5 的取词方法没写，会取到模块路径与任意值 | 接受：4.1.7 写明取法 |
| RA1-N2 | Nit | S1 里菜单项的环照旧还是照 4.2.6，有歧义 | 接受：S1 照旧，S2 改 |
| RA1-N3 | Nit | 8.2 第 6 条的锚点在 S1 之后出现两次 | 接受：锚点带上下文 |
| RA1-N4 | Nit | 00-M13"总体设计的修订"第 5 项的原文没随 8.2 改 | 接受：2.1 第 8 条与脚本加一处注明 |
| RA1-N5 | Nit | 第 13 节的一行不在脚本里 | 接受：脚本加 |
| RB1-N1 | Nit | 下划线式标签没有左右内边距，焦点环贴着字 | 接受：`px-1`、`gap-x-3` |
| RB1-N2 | Nit | 单选菜单项的勾没有尺寸 | 接受：`<Check className="size-3.5" />` |
| RB1-N3 | Nit | 表单字段的三种小字字号没写全 | 接受：都写 `text-compact` 与完整的类名 |
| RC1-N1 | Nit | "Go 的树没变"的断言范围太窄，`--stat` 判不出 | 接受：`git diff --quiet` 加 `deploy`、`.github`、`testdata`、`e2e/fixtures/db.ts` |
| RC1-N3 | Nit | "不含 `focus:ring-2`"要按类名判 | 接受：`classList.contains` |
| RC1-N4 | Nit | 第 31 条的新文字仍把编辑器的聚焦框算进"只在键盘" | 接受：编辑器另写一句 |
| RC1-N5 | Nit | 脚本里阴影一行的暗色写"同" | 接受：两格都写"见 01-P1 4.1.2" |

核对过、没有问题的（摘要）：`--ring: var(--accent)` 在 Chromium 里两种主题各取到自己的琥珀，传进 Portal 里也一样；`hover-layer` 编译出、只动 `background-image`，暗色在页面、侧栏、对话框里悬停都比静止亮；tailwind-merge 3.7.0 的 `radius`、`shadow`、`text` 登记管用；`in-data-[modality=keyboard]:focus:ring-2` 编译成 `:where([data-modality="keyboard"]) .…:focus`，键盘时 `contrast.ts` 读出 `ring 2px inset`；Chromium 在鼠标静止、元素出现在它下面时不发 `pointermove`；`gen-check-web` 只要 Node；修订脚本在副本上实写全部恰好一处。

## 修订核对第 2 轮

三位核对第 1 轮的修订：都已改到，没有 Important、Minor。B 与 A 各报了 Nit，由主持者补上（02-workflow 3.4 与 M12 起的"只剩 Nit 不再开一轮核对"）：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| RB2-N1 | Nit | 表单字段的后缀（`aria-hidden` 的扩展名）跟着改成 13 px，比它接着的名字小一号 | 接受：后缀与输入框同大 `text-base md:text-sm` |
| RA2-N1 | Nit | 4.2.1 仍写 `button.test.tsx` 是 S2 新建的 | 接受：改成 S1 建、S2 加格 |
| RA2-N2 | Nit | 测试 5 的非颜色表举的 `align-text-bottom` 照"从词头匹配"本来就取不到 | 接受：删掉这个例子 |

另外核对过：C 照新写法对现有源码模拟测试 5，要排除的非颜色值只剩十几个，模块路径与界面文字不误报，三处 `bg-muted` 与 `bg-black`、`text-white` 在旧树上红；23 行变异表照写都会变红（l5 按 `ControlOrMeta+e` 带修饰键，不算键盘；之后用 Enter 打开菜单，算键盘，环在）。B 用新装的 Playwright 1.63 浏览器补跑了指针事件：元素出现在静止的鼠标下面时三种引擎都不发 `pointermove`；滚动时只有 WebKit 发，菜单打开时 Radix 锁住滚动，任何按键又拨回键盘，无害。

设计定稿。
