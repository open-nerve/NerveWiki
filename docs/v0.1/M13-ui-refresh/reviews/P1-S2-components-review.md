# M13/P1/S2 基础组件：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p1-s2`（从 `m13-ui-refresh` 的 `46ff9215` 拉出）：`77c0499d`（按钮、输入框与下拉）、`3ab4e9fc`（菜单、对话框、标签页、进场动画与只在键盘操作时的环）、`f6ef65c8`（亮色琥珀深一档与它挪动的下限）、`463dd983`（审查的修复，测试并在各自的修复里）、`67d1e970`（修复核对第 1 轮的 Nit）；设计 [01-P1](../01-P1-foundation.md) 4.2、6.2 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的 detached worktree 里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（键盘、菜单、对话框、标签页、`useChoice` 的下拉、输入方式的模块，三种引擎里的临时探针，强制色模式）；B 管外观（Chromium 截图对照样稿，亮暗、1440 与 375；下拉在三种引擎与强制色模式，放大图与原型）；C 管测试、变异、门禁与完成度（独立重跑 7.2 的 S2 各条、自己写变异找漏洞、S1 的守卫管不管得住 S2 的代码、每个提交单独为绿、合并之前的本机门禁、WebKit 的像素探针）。重的运行都经 `heavy.sh` |
| 结论 | 第一轮：A 1 Minor、1 Q；B 1 Important、1 Minor、4 Nit、1 Q；C 1 Minor、6 Nit。Important 是 Chromium 亮色下勾选的复选框与单选画成深色（`#c58a22` 对白不到 3:1），由决定 766 修；A 的 Minor 是强制色模式下控件的焦点看不出（M12 起就有，S2 正改这几行），由决定 768 修。修复在 `f6ef65c8`、`463dd983`。修复核对第 1 轮：三位都没有 Important、Minor，上一轮的发现都已修好或照决定处置；剩下 5 条 Nit（A 1、B 1、C 3），由 `67d1e970` 修。修复核对第 2 轮：B 看两处类名，都已修好，另有 1 Nit、1 Q 推后到 P3；测试与注释由主持者读 diff、另改坏两条核对（02-workflow 3.4）；没有 Important、Minor |

## 第一轮的发现与处置

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S2A-M1 | Minor | 强制色模式下大部分控件聚焦时看不出焦点：Firefox 里描边按钮、幽灵图标按钮、搜索框、版本页的标签页、菜单项聚焦与不聚焦逐像素相同；Chromium 里幽灵按钮与菜单项同样，带边框的只有边框换成系统色。这些组件写 `outline-none`、焦点只在 `box-shadow` 的环上，强制色去掉了 `box-shadow`。M12 起就有 | 接受，在 S2 修（决定 768）：`outline-none` 旁加 `focus-visible:outline-hidden`（`463dd983`） |
| S2A-Q1 | Q | S2A-M1 在 S2 修还是移交；强制色的截图只有下拉一张 | 在 S2 修（决定 768）；实施者在 Chromium 的强制色模式下看过描边的图标按钮、幽灵图标按钮、搜索框与菜单项 |

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S2B-I1 | Important | Chromium 亮色下勾选的复选框是深色的勾、单选是深色的心（阅读视图的任务项、资料页、笔记本设置、新建笔记本），样稿是琥珀底白勾；`#c58a22` 对白 2.986:1，Chromium 因此改画深色。Firefox 与 WebKit 是白的；暗色的深勾合样稿 | 接受：亮色 `--accent` 改为 `#c2871f`（3.10:1，Chromium 从这里起画白的），亮色量琥珀的下限重量（决定 766，`f6ef65c8`）。没有照建议单加 `--accent-control`：琥珀仍只有一个来源（取舍 755） |
| S2B-M1 | Minor | 下拉的箭头比样稿粗大：约 11×6 px、线宽 1.6 px；样稿 14 px 的 `cd` 图标约 7×3.5 px、线宽约 1 px。取舍 753 的"与样稿一致"实测不成立 | 接受：`background-size: 5px 5px`、线宽 ±0.6 px、位置 `calc(100% - 16px)` 与 `calc(100% - 12px)`（`463dd983`）。没用建议的 15.5 px：1x 下三种引擎里两条线都在尖端之前交叉，尖下留一个点；16 px 时正好在尖端相交。实施者在三种引擎、亮暗、1x 与 2x 下与样稿并排比过 |
| S2B-N1 | Nit | 标签页当前项的下划线两端往上卷（`rounded-sm` 只有下边框） | 接受：`rounded-t-sm`（`463dd983`） |
| S2B-N2 | Nit | 树与附件的行菜单里"删除"前面没有样稿的分隔线 | 接受：两处在危险项前加 `DropdownMenuSeparator`，前面总有别的项（树里总有一个新建项，附件总有"打开"与"下载"）（`463dd983`） |
| S2B-N3 | Nit | `FormField` 改成 `space-y-1.5` 之后，手写的"Label 加控件"组还有 12 处是 `space-y-2`：`create-token-dialog.tsx:184`、`asset-dialogs.tsx:221`、`invitations-section.tsx:130`、`search-page.tsx:221`、`index-filters.tsx:60`、`history-filters.tsx:154`、`import-dialog.tsx:318,361`、`source-add-dialog.tsx:261`、`add-member-section.tsx:132`、`move-page-dialog.tsx:159,172` | 推后：各交给这些页所在的 Phase（P2–P5），01-P1 第 9 节加一条。S2 只改了与 `FormField` 夹在同一对话框里的令牌期限一块（10.2"与设计不同之处"） |
| S2B-N4 | Nit | 文件输入框的"Choose File"看不出是按钮（preflight 清掉了它的底与边框，S2 换成灰底之后更显眼） | 接受：`Input` 加 `file:` 的样式（`463dd983`） |
| S2B-Q1 | Q | 单选的焦点外框是方的；圆角与 `box-shadow` 都做不圆，只有自画的单选能圆 | 接受方框（决定 767） |

### 审查 C（测试、变异、门禁）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S2C-M1 | Minor | 菜单项的环另写成 `focus-visible:ring-2`、`data-highlighted:ring-2` 或 `in-data-[modality=pointer]:focus:ring-2`，用指针时也会出环，vitest 与 l5 都照绿 | 接受：断言菜单项的环宽类恰好是 `["in-data-[modality=keyboard]:focus:ring-2"]`（`463dd983`） |
| S2C-N1 | Nit | 两处"删除"去掉 `tone="destructive"`，全套照绿 | 接受：`attachments-section.test`、`page-tree-writes.test` 断言它带 `text-destructive`、紧前是分隔线、只有这一条线、"改名"不是红的（`463dd983`） |
| S2C-N2 | Nit | "each variant"只排除不带前缀的 `bg-destructive`，`dark:bg-destructive/60` 照绿 | 接受：含 `bg-destructive` 的类只许在 `hover:` 之下（`463dd983`） |
| S2C-N3 | Nit | 令牌期限改回裸的 `<select>` 照绿 | 接受：`tokens-page.test` 断言它带 `nw-select`（`463dd983`） |
| S2C-N4 | Nit | 菜单项图标的选择器写回 `[&_svg]` 照绿 | 接受：断言 `[&>svg]:text-muted-foreground`（`463dd983`） |
| S2C-N5 | Nit | 新的动画名没有登记进 tailwind-merge，调用处覆盖不了进场动画 | 接受：`cn.ts` 登记 `animate: ["pop-in", "fade-in"]`，读全部名字的那一格也读 `--animate-*`（`463dd983`，取舍 756 的同一理由） |
| S2C-N6 | Nit | 减少动效时的变量改坏照绿（7.1 只靠 7.4 的探针）；`stop()` 去监听不带 `capture` 照绿 | 接受：`design-tokens.test` 新一格读减少动效的块；`input-modality.test` 停下之后发 `keydown`（`463dd983`）。`stop()` 本来就传同一个选项对象，代码不改 |

## 修复核对第 1 轮

上一轮的发现：

- **A**：S2A-M1 已修。强制色模式下，Chromium 与 Firefox 里描边按钮、幽灵图标按钮、搜索框、下拉、版本页的标签与标签内容、对比视图的按钮、页面菜单的菜单项用键盘聚焦时都有 2 px 的实线外框；不聚焦的（页面、打开的菜单、搜索页、版本页上全部写了 `outline-none` 的元素）`outline-style` 都是 `none`；平时三种引擎里聚焦的控件是 `outline-style: none` 加 2 px 琥珀环，pg4、se1、l5 在新的下限下全绿。S2A-Q1 照决定 768。
- **B**：S2B-I1、M1、N1、N2、N4 已修。Chromium 亮色是白勾，单选是琥珀圈、白间隙、琥珀圆心，Firefox、WebKit 照旧，暗色照旧是合样稿的深勾；箭头在 1x 下三种引擎都是一个干净的 ∨，约 9×5 px、1 px 线，2x 下细而匀，与样稿相当；下划线两端是直的（375 宽折行时也是）；分隔线上下 4 px、左右 6 px，`--border`；亮色的文件按钮白底带边框。N3 推后，Q1 照决定 767。加深后的琥珀在焦点环、编辑器的聚焦框与选区（`--accent-soft` 照旧）、搜索框与下拉的焦点上都没有读错；对比视图的按钮与描边按钮实测一致。
- **C**：S2C-M1、N1–N6 已修。54 个 vitest 变异（7.2 的 S2 各条、C 上一轮的、S1 的守卫、实施者的 r1–r9）与 e2e 变异（13、12、`main.tsx` 不调用 `trackInputModality`、亮色琥珀改回 `#c58a22` 时 2.986 < 2.99）在 `463dd983` 上都红，每次复原；`styles.css` 里写死的色值照旧逃过测试 4（它只管 TS 源码，已知的范围）。两个修复提交各自单独为绿。下限全设 99 时 28 条读数都拿到：3.0977 → 2.99、2.6459 → 2.54，与注释相符，其余读数与 S1 相同。非颜色表的 `outline-hidden` 一行按整个词匹配（`outline-red-500`、`outline-muted` 照红，`outline-ring` 照绿）；减少动效那一格在块为空、不存在或注释掉时都红。
- **A 另核**：树与附件的菜单在三种引擎里，方向键跳过分隔线；不会有开头的分隔线（树里 `canDelete` 要先满足 `canManageNode`，新建一类的项总在；附件的"打开""下载"总在；读者没有"删除"）；导入对话框的文件输入在三种引擎里不溢出 36 px 的框，Tab 过去有环。

新的发现：

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S2A1-N1 | Nit | `editor/text-diff.tsx:260` 的比较失败提示仍是 S2 之前 `Alert` 的类名（同一文件的 `buttonClass` 已跟上 S2 的按钮） | 接受：照抄 S2 的 `Alert` 的样子（`67d1e970`） |
| S2B1-N1 | Nit | 暗色下"Choose File"（`file:bg-surface`，`#212226`）比输入框（`#25262a`）暗，像凹进去；与描边按钮"比所在的面亮一档"的约定相反 | 接受：`dark:file:bg-active`（`67d1e970`） |
| S2C1-N1 | Nit | 减少动效那一格：同一个 `:root` 里后写一行 `--animate-pop-in: nw-pop-in`、或删掉 `@keyframes nw-pop-in`，照绿 | 接受：正则不放过后面的声明，另断言 `@keyframes nw-pop-in`（`67d1e970`） |
| S2C1-N2 | Nit | `styles.css` 琥珀的注释写"from 3.1:1 they are white"；C 的探针：`#c48921`（3.023:1）起就是白的，门槛是 3:1 | 接受：改注释（`67d1e970`） |
| S2C1-N3 | Nit | 非颜色表 `outline-hidden` 一行的注释写"平时透明"；构建出的 CSS 平时没有外框，只在强制色模式下是透明的 2 px 外框 | 接受：改注释（`67d1e970`） |

## 修复核对第 2 轮

`67d1e970` 改了两处类名与测试、注释。两处类名交审查 B 看（Chromium，亮暗，1440 与 375）；测试与注释由主持者读 diff，在实施者的 worktree 里另改坏两条（失败提示去掉 `bg-destructive/5` → `alert.test` 的拷贝一格红；减少动效的 `:root` 里后写一行 `nw-pop-in` → 减少动效一格红），复原后 8 格全绿（02-workflow 3.4）。

- **S2B1-N1 已修**：暗色下文件按钮的底是 `rgba(255,255,255,.08)`，像素 `#36373a`，比输入框 `#25262a` 亮一档，读得出是凸起的按钮，比主按钮弱得多，与描边的"取消"分量相近；亮色不变；375 宽两种主题同样成立。
- **S2A1-N1 已修**：比较失败的提示框（拦掉比较的 worker 得到的真失败；"再次失败"加"重新载入"的一版也看了）与登录页密码错误的 `Alert` 计算样式逐项相同，亮暗的边、底与框外的像素都相同。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| S2B2-N1 | Nit | 文件按钮悬停时没有反馈：S2 给了它底与边框，原生按钮的悬停没了；旁边的"取消"悬停会变色。建议 `hover:file:hover-layer` | 推后：P3 重做导入对话框所在的设置界面时一并处理（01-P1 第 9 节） |
| S2B2-Q1 | Q | 暗色按钮对输入框的亮度差（ΔL\* 约 7.3）比亮色的（约 3.9）强；改用 `dark:file:bg-hover` 正好对等。两种 B 都接受 | 照现在的 `--active`；与 S2B2-N1 一起在 P3 再看 |

没有 Important、Minor，核对到此为止。

## 主持者的决定（负责人可以改判）

- **766 亮色琥珀深一档**：亮色 `--accent` 由样稿的 `#c58a22` 改为 `#c2871f`；暗色 `#d9a84e` 与两种主题的 `--accent-soft` 不变。`#c58a22` 对白 2.986:1，Chromium 的 `accent-color` 不到 3:1 时给勾选的复选框画深色的勾、单选画深色的心，与样稿的白勾相反；从 `#c2871f`（3.098:1）起画白的，色相看不出差别（S2B-I1；实施者在 Chromium 1x、2x 下核过：勾的中心像素由 59,59,59 变为 255,255,255）。不单加一个控件用的琥珀（审查 B 的建议）：琥珀仍只有一个来源（取舍 755），焦点圈随之深一点，对比度略高。亮色量琥珀的下限照 4.1.8 重量（下表）。
- **767 单选的焦点框是方的，接受**：原生的单选走全局的 `:focus-visible` 外框（4.2.5），三种引擎都画成方框；审查 B 试过 `border-radius` 与圆形的 `box-shadow`，都做不圆（WebKit 不画 `box-shadow`）。只有 `appearance: none` 自画的单选能圆，与 00-M13 决定 737 选原生控件相违（S2B-Q1）。
- **768 强制色模式下的焦点**：按钮、输入框、下拉、标签页（触发与内容）、菜单项与对比视图的 `buttonClass` 在 `outline-none` 旁加 `focus-visible:outline-hidden`。Tailwind 4.3.3 平时写 `outline-style: none`（`contrast.ts` 照旧读到环，pg4、se1、l5 全绿），`@media (forced-colors: active)` 下写 `outline: 2px solid transparent` 与 2 px 的偏移，系统色把它画出来（Chromium 模拟时算出 `solid 2px`，看得见）。不把 `outline-none` 整个换成 `outline-hidden`：强制色下不聚焦的控件也会带一圈框（审查 A 实测）。在 S2 修而不移交：S2 正改这几行，00-M13 第 4 节"照旧保留"的"强制色模式下控件可见"也该包括焦点（S2A-M1、S2A-Q1）。

## 变异

设计 7.2 的 S2 各条（实施者跑，C 独立重跑）：

| # | 改坏 | 抓住它的 |
|---|---|---|
| 12 | `NativeSelect` 去掉 `contain-paint` | `native-select.test` 的"paint contained"；se1 `@narrow` [webkit-narrow] 的"the page's width"（375 宽时 WebKit 的页宽 762） |
| 13 | 菜单内容加 `data-[state=closed]:animate-fade-in` | `dropdown-menu.test` 的"never out"；l5 的"menus a frame after Escape"（期望 0，得 1） |
| 14 | `tone` 不加 `text-destructive` | `dropdown-menu.test` 的危险项 |
| 15 | 加 `aria-disabled:pointer-events-none` 并给 `aria-disabled` 的按钮 `tabIndex=-1`（另拆成只加类、只加 `tabIndex` 两条） | `button.test` 的 `aria-disabled` 格，三条都红 |
| 20 | 菜单项的环写成不带前缀的 `focus:ring-2` | "keyboard's"一格；修复之后照旧红 |
| 22 | 不听 `pointermove` | `input-modality.test` 两格 |
| 23 | 修饰键也算键盘 | 修饰键一格 |

审查的修复带来的（实施者在 `463dd983` 上跑；要在修复之前就红的，写好时在旧代码上红过）：

| 改坏 | 抓住它的 |
|---|---|
| 按钮、对比视图的 `buttonClass`、菜单项各去掉 `focus-visible:outline-hidden` | `button.test` 的"each variant"、对比视图那一格、菜单"keyboard's" |
| 危险按钮加 `dark:bg-destructive/20` | "each variant" |
| 菜单项另加 `focus-visible:ring-2`、`data-highlighted:ring-2` 或 `in-data-[modality=pointer]:focus:ring-2` | 菜单"keyboard's"，三条都红 |
| 去掉 `[&>svg]:text-muted-foreground` | 同上 |
| 令牌期限换回裸的 `<select>` | `tokens-page.test` 两格 |
| 减少动效时也是 `nw-pop-in` | `design-tokens.test` 新的一格 |
| `stop()` 去掉 `keydown` 时不带 `capture` | `input-modality.test` 停下的一格 |
| 树或附件的菜单去掉分隔线；两处"删除"各去掉 `tone` | `page-tree-writes.test`、`attachments-section.test` |
| `cn.ts` 去掉 `animate` 的登记 | `cn.test` 两格 |
| 亮色 `--accent` 调淡成 `#e0b468`（7.2 第 1 条，换色之后再跑） | se1 与 l5 读到 1.927 < 2.99 |
| 修复核对第 1 轮之后（`67d1e970`）：减少动效的 `:root` 里再写一行 `nw-pop-in`；删掉 `@keyframes nw-pop-in`；对比视图的失败提示换回 S2 之前的类 | `design-tokens.test` 减少动效一格（前两条）；`alert.test` 的拷贝一格（实施者跑；主持者另改坏两条：去掉失败提示的 `bg-destructive/5`、`:root` 里后写 `nw-pop-in`，都红） |

审查者另写的：C 的 `input-modality.ts` 三条（改在冒泡阶段、Shift 也算修饰键、Meta 不算）、菜单与对话框四条（`tone` 落到 DOM、勾不写尺寸、去掉进场动画）、下拉两条（去掉 `aria-invalid:ring-destructive`、去掉 `nw-select`）都被抓住；`main.tsx` 不调用 `trackInputModality` 时 vitest 照绿，由 l5 抛"no focus indicator: none"守住。S1 的守卫管 S2 的代码：遮罩写 `bg-black/40`、菜单标签写 `text-muted`、菜单项写回 `focus:bg-accent`、S1 修复核对里的两个回归都被抓住；S2 新加的类没有一个是守卫本该报而没报的。

## 实量与下限（Chromium，macOS，Playwright 1.63）

决定 766 之后重量了亮色量琥珀的各项（下限 = 实量 − 0.1，向下取到 0.01；注释的实量值同步改）；其余各项与 S1 的记录相同，S2 的其余改动没有动被量元素的底或颜色。

| 故事 / 项 | 亮：实量 → 下限（S1 → S2） |
|---|---|
| se1 搜索框、"搜索"按钮、范围下拉与命中链接的焦点 | 2.986 → 3.098；2.88 → 2.99 |
| l5 编辑器的聚焦框、阅读视图链接的焦点 | 2.986 → 3.098；2.88 → 2.99 |
| l5 补全选中项、菜单当前项（内嵌的环） | 2.551 → 2.646；2.45 → 2.54 |

暗色各项、pg4（4.378 / 6.335）、se1 的无效框（4.378）、补全的说明字（5.281）与命中字（14.642 / 8.027，下限 7）不变。

## 三种引擎里的下拉（4.2.10，macOS，Playwright 1.63）

- `appearance: none` 之后三种引擎都画出 2 px 的 `ring`：边框外 1–2 px 的像素逐一相同，亮 `#c58a22`、暗 `#d9a84e`（换色之前量的），无效时亮 `#d64545`、暗 `#ff7b7b`（实施者；C 另以构建出的 CSS 在 WebKit 上逐像素读过，与 Chromium 相同）。所以 13.2 第 31 条括号里"Playwright 的 WebKit 不画原生下拉的 `box-shadow`……"一句删掉（01-P1 8.2 第 6 条），M12-polish 第 5 项的那一小项写进"处理进展"。Linux 的 WebKit 由负责人推送之后的 CI 再看：se1 的 `webkit-narrow` 格只量宽度，不看环。
- WebKit 不带 `contain-paint` 时仍把显示的那一项的文字算进页宽（375 宽时 762，从头布局与先宽后缩都是），带着时是 375；Chromium 与 Firefox 两种都是 375。所以 7.2 第 12 条由 vitest 与 se1 的 `@narrow` [webkit-narrow] 一起守。
- 强制色模式（Chromium 模拟）：`appearance: auto`，没有背景图，右边距 8 px，原生的箭头回来。

## 视觉的核对与探针（7.4）

- **实施者**：Chromium 亮暗 × 1440、375 共 71 张：首页、阅读视图、编辑、搜索、页面菜单（键盘与鼠标打开）、树的行菜单（悬停删除）、删除确认框（静止、悬停 Cancel、Tab）、个人资料页的单选、新建令牌对话框（含无效）、版本页的标签页；下拉另截 Firefox、WebKit 与 Chromium 的强制色模式。修复之后另截了勾与单选的放大（换色前后）、强制色下四种控件的聚焦、箭头的对照（三种引擎 × 亮暗 × 1x、2x，六种画法并排）、标签页、两处菜单的分隔线与导入对话框的文件按钮。
- **审查 B**：同样的页面与组合，另有树的行菜单（悬停改名、键盘走到项上）、版本页的方向键、新建令牌的下拉无效并聚焦、登录页的提示条、导入对话框；1440 另截登录页空提交、安全页、工作区与笔记本设置。实测与 4.2 一致：菜单宽 220、内边距 5、圆角 12、行高 32、项圆角 7、13.5 px；对话框圆角 16，暗色下弹出层 `#25262a` 比面 `#212226` 亮一档；按钮高 36、圆角 8、间距 6；输入框与下拉高 36、圆角 9、`surface-2` 聚焦换 `surface`，375 宽 16 px、宽屏 14 px；标签与小字 13 px；提示条红 5% 底、35% 边；危险项红字亮 `#d64545`、暗 `#ff7b7b`。与样稿不同、已修的是上面的 S2B-I1、M1、N1、N2、N4。
- **减少动效的探针**（实施者，三种引擎相同；C、A 另核）：不减少时菜单与对话框内容是 `nw-pop-in`（0.15 s），遮罩是 `nw-fade-in`，对话框的 `translate` 照旧 −50% −50%；`reducedMotion: reduce` 时菜单与对话框内容是 `nw-fade-in`；按 Escape 或 Cancel 之后一帧，菜单、对话框与遮罩都是 0 个。
- **只在键盘操作时出现的环**（实施者与 A，三种引擎相同）：鼠标移到菜单项上，它有焦点、底是 `--active`、没有环，`data-modality` 是 `pointer`，单按 Shift 照旧；方向键或用键盘打开时有内嵌的 2 px 琥珀环；鼠标一动，环消失，焦点留在原项；菜单在静止的鼠标下用键盘打开，焦点不被鼠标抢走。
- **审查 A 另核的行为**：菜单的按字查找、Home 与 End、Escape 之后焦点回到触发按钮；"删除"项的 DOM 上没有 `tone`；对话框打开时焦点在"取消"上、焦点困在对话框里、Escape 之后回到触发处；版本页的标签页照旧 `activationMode="manual"`；`useChoice` 的下拉在 `appearance: none` 与强制回 `auto` 时键盘的结果相同；单选的方向键与复选的空格。

## 门禁与 CI

| 项 | 结果 |
|---|---|
| 每个提交（实施者；C 另在 `77c0499d` 上带 `CI=1` 跑过） | `make lint-web`、`make knip`、`make test-web` 都绿（5104 → 5114 → 5114 → 5116 → 5118 格）；`3ab4e9fc` 另跑 l5，`f6ef65c8`、`463dd983` 另跑 se1、l5、pg4 [chromium]（`463dd983` 也跑 se1 [webkit-narrow]）。C 第一次跑 `77c0499d` 时负载 131，`markdown.test.ts` 的"不更慢"一格因负载失败，负载降下之后整个重跑为绿 |
| 合并之前，`3ab4e9fc`（实施者与 C 各跑一次） | `CI=1 make lint-web knip test-web gen-check-web build-web` 绿（68 s、79 s）；`CI=1 make e2e` 594 过、3 败、1 跳，败的正是允许的 SE2 se2:188 [chromium]、PG15 pg15:75 [webkit]、LW8 lw8:822 [webkit]；Playwright 2.4 min、2.6 min |
| 合并之前，`463dd983`（实施者） | `CI=1 make lint-web knip test-web gen-check-web build-web` 绿，71 s；`CI=1 make e2e` 594 过、只败允许的三格、1 跳，Playwright 2.6 min（含 `make build` 163 s，负载 50–58） |
| 合并之前，`67d1e970`（实施者）；修复核对第 1 轮（C，在 `463dd983` 上） | `67d1e970`：`CI=1 make lint-web knip test-web gen-check-web build-web` 绿，74 s，5118 格；`CI=1 make e2e` 594 过、只败允许的三格、1 跳，Playwright 2.5 min（含 `make build` 152 s，负载 46–53）；Go 一侧的断言与 `git diff --check` 以 0 退出；C：网页一侧绿，5116 格，80 s；`make e2e` 594 过、只败允许的三格、1 跳，Playwright 2.8 min（含 build 172 s）；`f6ef65c8` 单独 `CI=1 make lint-web knip test-web` 绿、pg4、se1、l5 [chromium] 11 格全过 |
| E2E 用时（7.5） | 第 0 步的基线 2.4 min，S2 之后 2.4–2.8 min（随负载），在设计推断的"不到一分钟"之内 |
| Go 一侧 | `git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 以 0 退出（取舍 761）；`git diff --check` 干净 |
| 没有重现的 CI 项 | 照 01-P1 7.3 的表：Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别 |
| CI | 本机没有推送的凭据（取舍 746、761）：负责人推送之后在最后一个头上补跑；se1 的 `webkit-narrow` 格在 Linux 上再核 4.2.10 |

## 接受与推后

- S2B-N3（12 处手写的标签组的间距）：各页所在的 Phase（01-P1 第 9 节）。
- S2B2-N1、Q1（文件按钮的悬停，暗色的底取 `--active` 还是 `--hover`）：P3（01-P1 第 9 节）。
- 亮色下搜索框（`surface-2`）与"搜索"按钮（`surface`）不同色（实施者与 B 都看到）：P2 把搜索框挪进侧栏时一并处理。
- 已勾的禁用任务项（灰勾）：P4/S2（4.2.5，原已记下）。
- 审查 A 没有实测的：读屏软件（VoiceOver 的组合键带 Ctrl 与 Option，不算键盘；它的"激活"可能合成指针事件，那样菜单项只有底色）、Windows 上 AltGr 的字符在菜单的按字查找里不算键盘、Playwright 的强制色仿真与真的 Windows 高对比度可能不同、Linux 上的引擎。
- 审查 C 没有实测的：对话框若用 CSS 加退场动画（不走类名），vitest 与 l5 都抓不到（jsdom 只守类名，设计已承认）；`markdown.test.ts` 的"不更慢"两格在负载 100 以上时会红（已有的性能测试，与 S2 无关）。
- 审查 B 在范围之外看到的：键盘在菜单里走过、再用鼠标点"删除"，确认框的 Cancel 带着琥珀环（Chromium 的 `:focus-visible` 沿程序移交的焦点传下去，S1 的记录已写），不改。
