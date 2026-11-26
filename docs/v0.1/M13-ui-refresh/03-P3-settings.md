# M13/P3 设置界面：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M13/P3 设置界面（两个 Step：S1 设置模式与个人设置；S2 工作区与笔记本设置） |
| 日期 | 2026-11-23（初稿；同日照设计审查修订，[审查记录](reviews/P3-design-review.md)，主持者的决定 833–847；同日照修订核对第 1、2 轮再修订，决定 848；主持者补上最后的 Nit，设计定稿）；2026-11-24（照 S1 的实施与审查修订，[S1 的审查记录](reviews/P3-S1-settings-mode-review.md)，主持者的决定 849–851；同日照 S2 的实施与审查修订，[S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)，主持者的决定 852–853） |
| 状态 | 完成（2026-11-24）。设计定稿（修订核对第 3 轮之后三位都没有 Important、Minor；主持者补上最后的 Nit；[审查记录](reviews/P3-design-review.md)，主持者的决定 810–848）；S1 实施与审查完成（[S1 的审查记录](reviews/P3-S1-settings-mode-review.md)，主持者的决定 849–851），合并 `f32b0762`；S2 实施与审查完成（[S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)，主持者的决定 852–853），合并 `12b0b734` |
| 上级文档 | [M13 总设计](00-M13-design.md) 4.4（全部）、4.2.2 第 3、4 项里设置模式的两句、4.2.3、4.2.4 末两项、4.2.5"设置模式"一项、4.3 的"单选与复选""下拉""按钮"三条、4.7、4.8（UI2、UI3 与变异的最低清单）、第 7 节 P3 两行、9.1"设置模式""即时生效""原生控件"三行、第 10 节"侧栏换成设置目录""a15"两行；[02-P2](02-P2-shell.md) 第 9 节"P3""P2–P5""P3–P5 的 375 px 故事""收尾"四条；[01-P1](01-P1-foundation.md) 第 9 节"P3"与"P2–P5"两条；[v0.1 总体设计](../v0.1-design.md) 9.2、13.2 第 12、16、17、20、31 条、13.4 第 3 条；移交 [M12-polish](../../v0.2/handoffs/M12-polish.md) 第 1 项（编号沿用 [M5-polish](../M12-release/handoffs/M5-polish.md) 的条目，19.2 的现象与改法写在那里） |
| 视觉参照 | [mockups/settings.html](mockups/settings.html) 方案 A（目录：`.side`、`.row.back`、`.stitle`、`.group`；页：`.page`、`.sec`、`.card`、`.set`；控件：`.btn`、`.input`、`.select`、`.saved`、`.badge`） |
| 代码位置的写法 | 同 00 号文档：`W/` 是 `web/apps/web/src/`；e2e 写作 `e2e/stories/…:行`。行号是 `m13-ui-refresh` 的头 `ba66557c` 的 |
| 工作笔记 | 设计只改了本文。设计者跑过四个探针（会话的临时目录与一个用完即删的分离工作树 `nw-p3-design`，都不进仓库）：(a) Tailwind 4.3.3 的 `compile` 编第 4 节的全部类名；(b) 照抄 `design-tokens.test.ts` 的三条规则核对这些类名与新的 CSS；(c) Playwright 1.63 三种引擎里一个静态页：卡片行在 1280 与 375 px 的排布、"已保存。"的 ✓、淡出与重来、强制色；(d) 在分离工作树里照第 4 节写出 S1 的大部分与 S2 的开放程度一行（23 个文件），`tsc`、零警告的 oxlint 与 knip 都过，全部 vitest 跑过（第 1 节末）。修订时另在一个用完即删的分离工作树 `nw-p3-rev` 里照修订写了改动的那几处，跑了 12 格 vitest 探针、三条变异、全量 vitest 与一个三种引擎的静态页（第 1 节末"修订的探针"），复用了审查者的探针。没有跑 e2e |

---

## 0. 分两个 Step

照 00 号文档第 7 节，P3 分两个 Step，串行，各自一个分支（`m13-p3-s<m>`，从 `m13-ui-refresh` 拉出、合并回它）、一次审查与修复核对、一次合并。每个提交单独为绿（7.3）。

- **S1 设置模式与个人设置**（`m13-p3-s1`）：三个设置路由带 `handle: { settings: true }`，`/settings` 另带 `sidebar: true`（决定 811）；`Layout` 在渲染时记下进入设置之前的那一页（来处，决定 812、833、834），有待办的引导步骤时不认 `sidebar`（决定 832）；设置目录（返回、标题、个人 / 工作区 / 笔记本三组）放进工作区外框的那一个左栏（工作区与笔记本的设置）与个人设置自己的左栏（决定 810、813、816–818、837、843）；笔记本设置里树留在一个 `hidden` 的容器里（决定 814、836），工作区左栏的滚动位置进出时留着（4.1.5）；设置页的 `h1` 按页挂载、取到达的焦点（决定 815），"返回"可能回到的变更集页、回收站的预览与邀请页的 `h1` 也取（决定 835）；卡片组件、文字输入的一行、"已保存。"的 ✓ 与淡出（决定 820、822、823、841、844、845）；个人设置的四页（个人资料、新的外观与语言、安全、访问令牌）换成卡片（决定 821、824）；工作区与笔记本设置只换外框，页的内容照旧（决定 831）；8 个在设置页上读侧栏的故事、a8、a15 与 12 个 vitest 文件照第 5、7 节改；新故事 UI2（三格，三种浏览器）。两个代码提交（决定 842，6.1）：
  - **提交 1** 设置模式、目录、来处、`IndexTo`、按页挂载的 `h1`（个人设置的 `h1` 是页名，页的内容照旧：个人资料页的 `h2`"个人资料"与 `h1` 同名，到提交 2 才去掉）、三页的到达焦点（决定 835）、令牌页撤回之后焦点到 `h1`（与页名同名的 `h2`"访问令牌"随之去掉）、目录当前一行在强制色下的边框（决定 845），连同读左栏与标题的 8 个故事、a8 与 a15 的标题、12 个 vitest 文件、UI2 与 `threeBrowsers`、`REQUIRED`（UI2 只核对这一提交的进出、目录、来处与焦点，不读卡片）；
  - **提交 2** 卡片、`NameRow`、外观与语言页、`.nw-saved`、主按钮的透明边框（决定 845），连同 a8 的保存与外观、a15 的五处 `h2`、`profile-page.test.tsx` 的三格与新的测试文件。

  **单独为绿**：设置模式一来，三处的 `h1`、内容区的 `nav` 去掉与读它们的测试必须同在提交 1；提交 1 里 `settings-page.tsx` 只有 `SettingsPage` 与 `useSettingsHeading`（三个布局与令牌页用），目录的个人一组只有三项（外观与语言随它的路由在提交 2 加），新导出（`settings-origin.ts`、`settings-directory.tsx` 的各项）都有调用者，knip 不报（分离工作树里核对过）；提交 2 的新导出（卡片组件、`name-row.tsx`）由个人设置的四页用；`workspaceSections`、`notebookSections` 由两个布局取文档标题用；UI2、`threeBrowsers` 与 `check.mjs` 的两行在提交 1，同一个提交。
- **S2 工作区与笔记本设置**（`m13-p3-s2`，S1 合并之后开）：工作区的常规、成员（含邀请）、无主笔记本，笔记本的常规、成员、导入导出换成卡片；两个常规页的第一组是"基本信息"（决定 844）；`h1` 换成页名、去掉与它重复的第一个 `h2`，列表删除之后焦点到 `h1`（决定 815）；名称换成文字输入的一行；列表行内不可逆的操作是描边按钮，红只在"危险操作"组（决定 826、838）；随页离开设置的链接带 `arrived`（决定 839）；开放程度换成即生效的下拉，有待用的选择时 Esc 退回（决定 825、840）；认领 M12-polish 第 1 项里的 19.2 与 01-P1 第 9 节的标签组、文件按钮的悬停（决定 830、846）；`changeAccessWith` 改选下拉，n4、UI2 的标题文字；新故事 UI3（两格，三种浏览器）。**单独为绿**：提交 1 里开放程度照旧是单选加"保存"，读它的格按开放程度的那个 `form` 找"保存"（名称的"保存"没编辑时不在，7.1）；页名作 `h1` 与读旧标题的 vitest、n4:164、UI2 在同一个提交；提交 2 的开放程度下拉、`useChoice` 的 Esc 与 `changeAccessWith`、n3、UI3、`threeBrowsers`、`REQUIRED` 在同一个提交。

为什么是这样分：S1 只动"设置在哪个外框里、怎么进出、焦点到哪"，外加样子只涉及个人设置的四页（它们的测试与 a8、a15 随之改）；S2 只动"工作区与笔记本设置的页面长什么样、怎样保存"。两块各自有一批故事与测试（S1：读侧栏的 8 个故事、a8、a15、12 个 vitest 文件；S2：n4、`changeAccessWith`（n3 经它）、`general-page.test.tsx` 的保存与开放程度、成员与导入导出的测试），彼此不重叠（n3 在 S1 改读左栏的几步，在 S2 只经改了的夹具）。S1 的目录与来处、S2 的即时生效与被拒退回是 P3 里最难的两处，各自单独审查；S1 的提交 1 只有外框、目录与来处，可以先单独看（决定 842）。

## 1. 基线

- **路由**（`W/app/routes.tsx`）：个人设置 `:100-130`，没有 `handle`，`index` 是 `<Navigate replace to="/settings/profile" />`（`:107`），三页 profile、security、tokens；`/` 的 `index` 带 `handle: { sidebar: true }`（`:84-92`），`:slug` 同（`:131-135`，`:134`）；笔记本设置 `:237-267`，`index` 是 `<Navigate replace to="general" />`（`:244`），三页 general、members、transfer；工作区设置 `:278-308`，`index` 同（`:285`），三页 general、members、ownerless。三个 `index` 都不转交路由状态。
- **外框**（`W/app/layout.tsx:41-66`）：`sidebar` 读命中的路由的 `handle`（`:49`），没有时、或有人交还时放右上角（`:55`）；唯一的 `main` 是 `p-6`（`:56`）。右上角（`W/app/corner.tsx:52-87`）在 `/settings/*` 上（`:56`）放"回到工作区"（`:65,70-79`，去 `/`，带 `arrived`）与账户按钮（`UserMenu place="corner"`）。
- **三个设置布局**：个人（`W/pages/settings/settings-layout.tsx:23-48`）画 `h1`"设置"（`:31-33`，`useArrivalFocus`，P2/S1 加的）与内容区里名为"设置"的 `nav`（`:35-41`）；工作区（`W/pages/workspace/settings-layout.tsx:22-47`）画 `h1`"工作区设置"（`:30`，不取焦点）与 `nav`（`:32-40`，"无主笔记本"只给管理员，`:34`）；笔记本（`W/pages/notebook/settings-layout.tsx:22-46`）画 `h1`"X 设置"（`:31`，`notebookSettings.heading`）与 `nav`（`:33-39`）。三处都是 `max-w-4xl`、宽屏上 `nav` 在左。
- **工作区外框**（`W/pages/workspace/workspace-layout.tsx:92-179`）：导航的"设置"指 `/${slug}/settings`、不带路由状态（`:43-51`）；`useDrawer()` 与 `useSearchShortcuts`（`:103-104`）；左栏 `left`（`:147-168`）是 `ShellSidebar`，顶部是切换器与搜索框，中段是工作区的 `nav`（首页、设置、`NotebookNav`），门户进 `Layout` 的列（`:173`）。工作区与笔记本的设置都在这个外框之内，P2 里它们的左栏照旧是工作区的。
- **树的容器**：笔记本外框把笔记本行、树与"更多"门户进外框按笔记本 id 给的容器（`W/pages/notebook/notebook-layout.tsx:58,76-83`），列表里那一项的 `SlotHolder` 收养它（`W/pages/workspace/notebook-slot.tsx:108-137`）。笔记本行的齿轮指 `${home}/settings/general`、不带路由状态（`W/pages/notebook/page-tree.tsx:291-301`）。
- **`ShellSidebar`**（`W/app/shell-sidebar.tsx:305-373`）：细栏（`:321`，`md:hidden`：菜单按钮与位置文字）、`FocusScope` 包着的根（`data-shell`、抽屉的 `inert` 与 `role=dialog`）、顶部（`space-y-2 px-2 pt-2.5 pb-1.5`）、自己滚动的中段（`:356`，`data-sidebar-scroll`）、底部的账户行（`:360`）。`useFollow`（`:85-97`）：左栏里不是树的行的链接，抽屉开着时带 `arrived`，否则 `opened`，点的是当前地址时 `replace`。
- **`NavItem`**（`W/components/nav-item.tsx:13-30`）：只有 `to`、`end`、`children`，30 px 的行，当前一行 `bg-active`。
- **页面**：个人资料（`W/pages/settings/profile-page.tsx`）是 `DisplayNameForm`（`:20-40`，按钮一直在）与主题、语言两组单选（`:47-89`，选了即生效、只存本机）；安全是改密码的表单与停用账户；访问令牌（`W/pages/settings/tokens-page.tsx`）撤回之后焦点回到 `h2`"访问令牌"（`:19-21`），列表是 `ul.divide-y.rounded-md.border`（`:42-46`）。工作区的常规是 `RenameForm` 与删除，成员是成员、邀请与离开三节，无主笔记本带审计日志；笔记本的常规（`W/pages/notebook/general-page.tsx:32-77`）是 `RenameForm`、开放程度的单选与"保存"（`AccessForm`，`:89-135`）、模式（`ModeSection`，`h2`"模式"是降级之后的焦点目标）与删除，成员是成员、添加与离开，导入导出是导入、导出与最近的任务三节。
- **表单**：`RenameForm`（`W/app/rename-form.tsx:44-111`）与 `DisplayNameForm`（`W/app/display-name-form.tsx:33-86`，引导的个人资料一步也用）的"保存"一直在、发送中 `disabled`（`rename-form.tsx:104`）；`FormField`（`W/components/form-field.tsx:23-81`）的 `aria-describedby` 在有后缀又有问题时是两个 id（`:30`）；`useChoice`（`W/components/ui/native-select.tsx:59-107`）；开放程度的三项（`W/pages/notebook/access-options.tsx:8-12`，没导出）与单选（`:25-55`，新建笔记本的对话框也用）。
- **样式**：`W/styles.css` 的 `@layer components`（`:175-232`）里 `.nw-search` 的清除按钮是先例：`color` 给颜色、`background-color: currentColor`、`mask` 画形状，强制色下 `forced-color-adjust: none; color: CanvasText`（`:212-231`）。
- **到达的焦点与锚点**：`useArrivalFocus`（`W/app/arrival.ts:208-230`）在 passive effect 里取焦点，为一次导航挂载的第一个标题不论焦点在哪都取走（`takeArrivalFocus`，`:65-94`）；页面的 `useViewAnchors`（`W/pages/page/view-anchors.ts:55-101`）在视图读到时的 layout effect 里聚焦锚点的元素：视图在缓存里时锚点先、`h1` 后（`h1` 取走），不在时 `h1` 先、锚点后（决定 834）。不用 `useArrivalFocus` 的有左栏或右上角的页只有三处：变更集页（`W/pages/notebook/changeset-page.tsx:99` 的 `useRef`）、回收站的预览（`W/pages/notebook/trash-preview.tsx:406,579`，两处 `useRef`，各有一个"焦点无人时取"的 effect，`:410-415`、`:583-588`）与邀请页（`W/pages/invitation.tsx:39`，`h1` 没有 ref 与 `tabIndex`）。邀请的令牌在地址的片段里（`W/pages/invitation.tsx:33-36`）。
- **左栏的滚动**：`SlotHolder` 还焦点之后调 `intoColumn`（`W/pages/workspace/notebook-slot.tsx:127-130`），不看焦点还上了没有；`intoColumn` 只改 `[data-sidebar-scroll]` 的 `scrollTop`（`W/app/shell-sidebar.tsx:31-44`）；树把当前页的行带进视野在 `PageTree` 自己的 layout effect 里（`W/pages/notebook/page-tree.tsx:255-263`），它在 `WorkspaceLayout` 的 `Outlet` 之下，左栏的门户在 `Outlet` 之前（同 `notebook-slot.tsx:111-113` 的注释）。
- **下拉与 Esc**：`useChoice` 的用者是搜索的范围（`W/pages/search/search-page.tsx:184`）、目录与历史的筛选（`W/pages/notebook/index-filters.tsx:31`、`history-filters.tsx:115-117`），都不在对话框或抽屉里；Radix 对话框的 Esc 是 document 上的捕获监听，抽屉的 Esc 是它的根上的原生监听（`W/app/shell-sidebar.tsx:243-268`）。
- **按钮与导航行**：`Button` 的 `default` 变体没有边框（`W/components/ui/button.tsx:11`），`NavItem` 的当前一行只有 `bg-active`（`W/components/nav-item.tsx:22`）：强制色下底色去掉，两处都看不出（审查 B 的 N5）。
- **测试**：vitest 216 个文件、5237 格（`ba66557c`；审查 C 经 `heavy.sh make test-web` 实跑，与 P2/S3 记录里 `bb00d33f` 的数相同；初稿的 5244 不对）。E2E：P2 最后一次门禁（S3 合并之前，`bb00d33f`）609 格过、3 格失败、1 格跳过，本机 2.3–2.5 min；`git diff --stat bb00d33f ba66557c -- web e2e tools Makefile package.json pnpm-lock.yaml server api` 是空的：代码与那次门禁相同（决定 827）。已知的本机失败（P2 的基线，决定 795(a)）：SE2 [chromium] `se2:231`（`Error: expect(locator).toHaveValue(expected) failed`）、LW8 [webkit] `lw8:836`（`Error: expect(locator).toBeFocused() failed`）、PG15 [webkit] `pg15:113`（`Error: expect(received).toEqual(expected) // deep equality`，有时过）；偶发 AS6 `as6:153`（一次是 `as6:167` 的 "Response has been disposed"）、LW8 `lw8:771`、负载过 100 时 `markdown.test.ts` 的"not slower"与负载高时的 `trash-preview.test.tsx:587`（P2/S3 记录的 S3C1-N1）。
- **设计者的探针**：
  - (a) 第 4 节的全部类名（89 个词）在 `W/styles.css` 上编出来都生成了，`break-words`、`font-[650]`、`text-[12.5px]`、`file:hover:hover-layer` 都在。
  - (b) `design-tokens.test.ts` 的"每个颜色类都是主题的颜色"（`:345` 起，`noColour` 在 `:298`）、"琥珀不是填充"（`amberBackground`，`:250`）与"不写调色板、不写色值"三条照抄过来核对：没有一个词、一行 CSS 被标出。✓ 的 CSS 写 `color: var(--accent); background-color: currentColor`，同 `.nw-search`：它是图标，不是填充，`amberBackground` 也不认 `currentColor`（决定 823）。分离工作树的全量 vitest 里这个测试本身也是绿的。
  - (c) 静态页（第 4 节的类名、`cn` 合并之后的 `Input`、`NativeSelect`、`Button` 类名）在 Chromium、Firefox、WebKit 里结果相同：1280 宽时输入框 240×32，控件一组 375 宽，问题提示（`w-0 min-w-full basis-full`）落在输入框下面、不撑宽控件；375 宽时控件落到文字之下、宽 289，`scrollWidth` 375 不溢出；"已保存。"有字时 `::before` 是 13 px 的琥珀（亮色 `rgb(194, 135, 31)`），没字时 `content: none`；0.5 秒时透明度 1，2.2 秒时 0；先清空、隔一帧再写"已保存。"，动画重来（0.3 秒时 1，2.2 秒时 0）；同一帧里清空又写回不重来（透明度停在 0）。审查 B、C 另量到隔不到一帧（快的答复）时 Chromium、Firefox 时而重来时而不重来，初稿的"发出时先清空"不够，改为交替的 keyframes（决定 841，"修订的探针"）；强制色下 `::before` 是 `CanvasText`（`rgb(0, 0, 0)`），三种引擎都画出来了。
  - (d) 分离工作树（`ba66557c`，用完删掉）里照第 4 节写了 `settings-origin.ts`、`settings-directory.tsx`、`settings-page.tsx`、`name-row.tsx`、外观与语言页、改写的个人资料页与令牌页、三个布局、`Layout`、`routes.tsx`、`NavItem`、`WorkspaceLayout`、齿轮与 S2 的开放程度一行：`tsc --noEmit`、`oxlint --max-warnings=0`、`knip --treat-config-hints-as-errors` 都过；全量 vitest 里 28 格变红，26 格在第 7 节列出 S1 要改的 12 个文件里，另 2 格是 S2 的开放程度（`general-page.test.tsx` 找单选），没有一格在别处；另写的 8 个探针格全绿：齿轮进入、`h1` 取焦点、没有搜索框、三个 `nav` 依次是"设置""工作区设置""笔记本设置"；同外框的目录链接点了焦点留在链接上；跨到个人资料、`h1` 取焦点、"返回 Plans"回到笔记本首页、`h1` 取焦点、树回到左栏；直接打开地址时"返回 Lab"去工作区首页；个人设置的工作区组是最近的工作区；文字输入的一行改了才有"保存"，保存之后焦点回到输入框，`aria-describedby` 只有一个 id；开放程度选了即发、"已保存。"、焦点留在下拉上；被拒时退回原值、显示原因、没有"已保存。"；设置模式里 Mod+K 不被 `preventDefault`；`/settings` 落到个人资料、链接当前、右上角没有"回到工作区"、账户按钮在左栏里。
  - 探针找到的两件事：一是 `/settings` 带上 `sidebar: true` 之后，引导中的账户在右上角的菜单里选"个人设置"、被送回引导时，`Layout` 在弹回的那一次渲染里把右上角卸下，账户按钮随之卸下、焦点落到 `body`（`W/pages/settings/profile-page.test.tsx:70-88` 变红）：决定 832 让 `Layout` 对有待办步骤的账户不认 `sidebar`，那一格又绿了。二是容器不在文档里时，13 格 `transfer-background.test.tsx` 与 5 格 `general-page.test.tsx` 的升级格等不到树的 `nav`（`findByRole` 只找文档里的）：决定 814 把容器放进一个 `hidden` 的 `div`，那 18 格加上 `{ hidden: true }` 就都绿了（这只是 jsdom 里的表现；理由是决定 836 的那一条）。
- **修订的探针**（设计审查之后；分离工作树 `nw-p3-rev`，`ba66557c` 加初稿的 23 个文件，再照修订改了 `settings-origin.ts`、`settings-directory.tsx`、`settings-page.tsx`、`name-row.tsx`、`styles.css`、`native-select.tsx`、`button.tsx`、`nav-item.tsx`、`notebook-slot.tsx`、`workspace-layout.tsx`、开放程度的一行与三页的 `h1`；用完删掉；脚本在会话的临时目录 `p3-revise/`，不进仓库）：
  - vitest 12 格（经 `heavy.sh`），全过：未登录打开 `/settings/security`、登录之后"返回 Lab"指 `/lab`（不是登录页，决定 833）；引导的 `next` 是设置页，做完之后"返回"不指引导；来处是 `/invitations/inv-1#tok`："返回"指它、带片段，点了之后邀请页照旧显示邀请、`h1` 拿着焦点（决定 834、835）；来处是找不到的工作区（列表里没有它）："返回"指 `/`（A-N3）；离开设置再从 `/lab/search?q=a` 进入，来处是它；从 Lab 进入个人设置、本设备最近的工作区已是别的标签页里的 Acme：工作区一组是 Lab（决定 837）；直接打开个人设置、没有最近的工作区：`GET /workspaces` 0 次（决定 843）；开放程度按 `ArrowDown` 走到下一项：说明换成那一项的、没有发出，Esc 退回原值与原说明、`preventDefault`，失焦之后也不发（决定 840）；Radix 对话框里同样的下拉按 Esc：对话框先关（它在 document 上捕获），没有发出；`SavedOutput` 的轮次每次写出时换（`0 S1 -1 S0 -0 -0 S1 S1`，决定 841）；`NameRow` 出了问题之后改回原名：没有 `aria-invalid`、没有问题、没有"保存"（A-M1）；`scrollTop` 与 `getBoundingClientRect` 打桩：工作区左栏滚到 300、点齿轮，设置里是 0，"返回"之后先还成 300、树再把当前行带进来（`[300, 738]`，4.1.5）。
  - 十二条变异照这些格跑，都红了（7.2 写了探针号的那几条）：进入设置时不看 `REPLACE`（R1、R2 两格）；不在设置里时不调 `setSeen`（`/lab/search?q=a` 那一格：来处停在 `/lab`）；列表的键不看要找的工作区（读了 1 次）；邀请页的 `h1` 照旧是 `useRef`、工作区之外的来处不带片段（都是 R3）；来处的工作区不在已读的列表里时回来处（R4）；先取最近的工作区（R6）；`useChoice` 不处理 Esc、说明取已生效的值（都是 R8）；轮次不换（R10）；问题不看 `changed`（R11）；`ScrollKept` 不还原（R12）。每条改坏一处、只跑它的那一格、跑完复原（脚本 `p3-revise/mut2.py`）。
- **修订核对第 1 轮之后的探针**（分离工作树 `nw-p3-rev2`：`ba66557c` 加上面的草稿，再加 `user-menu.tsx` 的 `atProfile`、开放程度的 `aria-busy` 取草稿、横幅的 `arrived`；用完删掉）：
  - vitest 6 格，全过：Q1 窄屏、模拟 `inert`，在个人资料页上从抽屉里的账户菜单再选"Settings"，焦点在菜单按钮，路由状态是 `null`；Q2 开放程度走一步之后下拉没有 `aria-busy`，20 ms 后也没有发出；Q3 连选两次、各自扣住，`aria-busy` 依次是 `"true"`、`"true"`、没有；Q4 有待用的选择时父元素收不到 Esc，没有时收得到；Q5 以 `arrived` 打开已不在回收站的一项，`Gone` 的 `h1` 拿着焦点；Q6 升级进行中，笔记本成员设置页顶上横幅里的"View the upgrade"点了之后升级页的 `h1` 拿着焦点（改之前是 `body`）。
  - 六条变异各红一格："个人设置"在个人资料页上也带 `arrived`（Q1：焦点在 `h1`）；绕过 `useChoice`（Q2：`aria-busy` 立即是 `"true"`）；`aria-busy` 取 `sending`（Q3：`["true", null, null]`）；Esc 不 `stopPropagation`（Q4）；`Gone` 的 `h1` 照旧是 `useRef`（Q5：`body`）；横幅的链接不带 `arrived`（Q6）。脚本 `p3-revise/mut3.py`。
  - 静态页（审查 B 的 `gen2.mjs` 与 `rows.mjs`，只换成 4.1.7、4.1.8 修订后的类名，Tailwind 4.3.3，三种引擎）：1440 宽时没编辑、出了问题、已保存三态的输入框都在 867–1107，出了问题时"保存"离卡片右边 19 px、问题在 867–1175；375 宽时没编辑的控件区与输入框同高（行高 134），"已保存。"在 43、与输入框左对齐，有问题时依次是输入框、问题、"保存"。
- **修订核对第 2 轮之后的探针**（审查 B 的 `gen3.mjs`、`band-fix2.mjs` 与 `cost.mjs`，加设计者的 `rows.mjs`，只换成 4.1.7 写的类名，不用内联样式模拟；Tailwind 4.3.3，三种引擎；静态页，不碰应用；脚本在 `p3-revise/r2/`）：
  - 768–1200 每 2 px，中英文："已保存。"出现、消失时，开放程度与名称两行的行高、下拉与输入框的位置都不变（决定 848；不预留时 B 量到 768–1010 与 910–990 两段会跳）。1440、1024、900 宽时下拉离卡片右边 19 px，"已保存。"的字离下拉 8 px。
  - 名称一行四态：1440 宽输入框都在 867–1107，900 宽都在 541–781，"保存"离卡片右边 19 px；问题的字从输入框的左边起（Range 实量 867、541）。只预留宽度、问题不加 `md:ps-19`（当时写作 `md:pl-19`，从左到右的文字里相同）时，问题的字从 791 起，比输入框靠左 76 px。375 宽时没编辑的行高 134，"已保存。"在 43、与输入框左对齐，次序是输入框、问题、"保存"。
  - 代价：名称一行的说明列 250 px（原来 326），1024–1440 宽时行高 92 不变；开放程度一行的说明列 408 px，英文说明两行（行高 92），中文一行（73）。
  - 全量 vitest（经 `heavy.sh`）：红的 28 格与初稿的分离工作树完全相同（逐格比过），修订加的改动没有让别的格变红；`tsc`、零警告的 oxlint（`trash-preview.tsx` 两个 effect 的依赖要加上 `heading`：`useArrivalFocus` 的 ref 不被认作稳定，同 `page-layout.tsx:299`）、`make knip` 都过。
  - 静态页（Tailwind 4.3.3 编译工作树的 `styles.css`，三种引擎，1440、900、375 宽）：页的卡片宽 696（决定 844）；"已保存。"出现时开放程度的下拉与名称的输入框都不动，"保存"出现、消失时输入框不动；控件、"保存"、危险按钮、列表行的按钮右边离卡片 19 px（1 px 边框加 18 px 留白）；375 宽时依次是输入框、问题、"保存"；70 个汉字的只读值右对齐、折行、不溢出，`scrollWidth` 等于窗口宽；组标题"笔记本"带 21 个汉字的名字时高 16（一行）；`h1` 行高 39 px；强制色（Chromium、Firefox 的模拟）下主按钮的边框是系统色，目录的当前一行有 1 px 的系统色边框、别的行没有（WebKit 只认媒体查询、不改颜色，同 P1、P2）；淡出交替两个 keyframes：清空之后同一个任务里、隔 0、2、5、10、20 ms 写回，三种引擎都是 20/20 重来，2.2 秒时透明度 0。

## 2. 目标与范围

做 00 号文档 4.4 的设置界面：设置模式的外框与目录、返回与来处、焦点；卡片行；外观与语言的新页；工作区与笔记本设置的卡片；开放程度的即时生效与草稿；危险操作组；UI2、UI3。不做：别处通往设置的内容链接（工作区首页的提醒、笔记本首页的"笔记本设置"、变更集页与回收站预览里的"See its progress"等，P5，决定 828；设置页里随页离开的链接 S2 做，决定 839）；`main` 的留白与细栏的位置文字换成页的标题（P4）；引导、新建工作区页的样子（P5）。

### 2.1 对 00 号文档的修订

下面各条随本文一起提交（决定 847，P1、P2 的先例 `0ea1ac97`、`58c5fa92`），已照这里的文字落进工作区里的 [00 号文档](00-M13-design.md) 与 [02-P2](02-P2-shell.md)：脚本依次做了这些替换，每段旧文字在做它之前恰好出现一次（脚本在会话的临时目录，不进仓库）。"换成"是旧文字换成新文字，"之后加"是新文字接在旧文字之后。00 号文档的行号照它的头是 `3c315037` 的，不改：`W/app/routes.tsx:105,240,281` 今天是 `:107,244,285`（第 3 条整句换掉，"焦点"一条里的那一处照旧），`useChoice` 今天是 `native-select.tsx:59-107`，`AccessForm` 是 `general-page.tsx:89-135`，`changeAccessWith` 在 `e2e/fixtures/notebook-pages.ts:96-102`。头的状态、第 12 节 P3 一行与第 13 节的一行（第 21–23 条）是设计提交的记录，同 P2。

1. **4.4.1"一个设置目录组件"**（决定 810）：旧文字

   > - **一个设置目录组件**，三个设置布局（`W/pages/settings/settings-layout.tsx`、`W/pages/workspace/settings-layout.tsx`、`W/pages/notebook/settings-layout.tsx`）都用它，把它放进侧栏的位置。

   换成：

   > - **一个设置目录组件**（`W/app/settings-directory.tsx` 的 `SettingsTop` 与 `SettingsDirectory`），放它的外框两个：工作区与笔记本的设置由 `WorkspaceLayout` 把自己的那一个侧栏换成目录（同一个 `ShellSidebar` 元素，抽屉、菜单按钮与细栏都不重新挂载），个人设置由它的 `SettingsLayout` 自己画一个侧栏；工作区与笔记本的两个设置布局不再画 `nav`，也不画侧栏：笔记本读取中、读取失败或找不到时设置布局不渲染，侧栏却要在（[03-P3](03-P3-settings.md) 决定 810）。

2. **4.4.1"工作区"一组**（决定 818、837）：旧文字

   > 在个人设置里时是最近去过的工作区（`W/stores/preferences.store.ts` 已记着它）

   换成：

   > 在个人设置里时是来处所在的工作区，没有来处或来处不在工作区里时是最近去过的工作区（`W/stores/preferences.store.ts` 已记着它；[03-P3](03-P3-settings.md) 决定 818、837）

3. **4.4.1"返回"**（决定 812、813、833、834）：旧文字

   > 进入设置时把来处记在本标签页的 store 里（不放在路由状态：设置的三个 `index` 重定向 `<Navigate replace>` 会丢掉它，`W/app/routes.tsx:105,240,281`）；没有记录（直接打开设置的地址）时回到组里的工作区首页，再没有就回首页。

   换成：

   > 进入设置时把来处（地址连同片段，与它所在的工作区）记在 `Layout` 的状态里（渲染时取；不放在路由状态：设置的三个 `index` 重定向 `<Navigate replace>` 会丢掉它，`W/app/routes.tsx:105,240,281`），以 `replace` 进入（守卫与引导的 `next` 那样的重定向）时不记；没有记录（直接打开设置的地址、经重定向进入）时回到组里的工作区首页，再没有就回首页；来处的笔记本已不在列表里时回到它的工作区首页，来处的工作区已不在列表里时回首页；来处不在工作区里（新建工作区页、邀请页）时只写"返回"，连片段回到来处（邀请的令牌在片段里）；工作区里的来处不带片段：页面的锚点与到达的焦点谁先谁后，取决于视图在不在缓存里（[03-P3](03-P3-settings.md) 决定 812、813、833、834）。

4. **4.4.1"焦点"末句**（决定 815、817、835）：旧文字

   > 设置页的 `h1` 今天不取到达的焦点，加上 `useArrivalFocus` 与 `tabIndex={-1}`（主持者的决定 745）。

   换成：

   > 设置页的 `h1` 今天不取到达的焦点，加上 `useArrivalFocus` 与 `tabIndex={-1}`（主持者的决定 745）：`h1` 是页名（"个人资料""成员"……，不再是"设置""工作区设置""X 设置"；个人设置在 P3/S1，工作区与笔记本的设置在 P3/S2），由布局按页挂载（`key` 是页的路径）；目录的标题"设置"不是标题元素；"返回"可能回到的变更集页、回收站的预览与邀请页的 `h1` 今天也不取，同样加上；在个人资料页上再选账户菜单的"个人设置"不带 `arrived`（地址相同，没有到达），焦点回到菜单按钮（[03-P3](03-P3-settings.md) 决定 815、817、835）。

5. **4.2.2 第 4 项末句**（决定 814、836）：旧文字

   > 设置模式下侧栏不在，树随之卸下（树的 store 与 `TreeWatch` 照旧在笔记本外框里，`W/pages/notebook/notebook-layout.tsx`），返回时重画；13.2 第 16 条照此改写（P3/S1）。

   换成：

   > 设置模式下侧栏换成设置目录，列表里没有一项收养这个容器：笔记本的设置里，目录之后一个 `hidden` 的 `div` 里的 `SlotHolder` 收养它，树照旧挂载、读取与监视（`NotebookLayout` 在笔记本设置里一直挂着，一直把树门户进容器），不显示，也不在可访问性树里；返回时列表里那一项把容器收养回去，不重画。容器要在文档里：直接打开笔记本设置的地址时，"树的 effect 运行时容器已在文档里"（[02-P2](02-P2-shell.md) 决定 776）与"`reveal` 时容器总已收养"（决定 791）才成立。工作区设置与个人设置没有笔记本外框，没有树。13.2 第 16 条照此改写（P3/S1；[03-P3](03-P3-settings.md) 决定 814、836）。

6. **4.2.3 开头**（决定 832）：旧文字

   > （命中带 `handle: { sidebar: true }` 的路由（`:slug` 与去向是它的 `/`）时不放，

   换成：

   > （命中带 `handle: { sidebar: true }` 的路由（`:slug`、去向是它的 `/` 与 P3 起的 `/settings`）时不放，账户还有待办的引导步骤时除外（`Onboarded` 会把它送回引导，弹回的那一次渲染里右上角不卸下，[03-P3](03-P3-settings.md) 决定 832），

7. **4.2.3 末项**（决定 811）：旧文字

   > （去最近的工作区，没有时去首页；带 `arrived`，`LandingPage` 转交给它的 `<Navigate>`）。新建工作区页在账户已有工作区时也放它，P3 之后照旧

   换成：

   > （去最近的工作区，没有时去首页；带 `arrived`，`LandingPage` 转交给它的 `<Navigate>`）；P3/S1 起个人设置有了侧栏，这一项随之删掉（`W/app/corner.tsx` 的 `settings`，[03-P3](03-P3-settings.md) 决定 811）。新建工作区页在账户已有工作区时也放它，P3 之后照旧

8. **4.2.4 的子项（行首两个空格）**（决定 819）：旧文字

   >   - 设置模式里没有搜索框，两个快捷键都不做事（有意如此）。

   换成：

   >   - 设置模式里没有搜索框，两个快捷键都不做事（有意如此）：`useSearchShortcuts` 照旧挂在工作区外框，框不在时直接返回、不 `preventDefault`（[03-P3](03-P3-settings.md) 决定 819）。

9. **4.4.1 的 e2e 清单**：旧文字

   > 看树里的目录与历史两行随降级消失，再从树里删 AGENTS）；

   换成：

   > 看树里的目录与历史两行随降级消失，再从树里删 AGENTS）、hi5（`e2e/stories/history/hi5-revert-import.spec.ts:213`，在导入导出页上读树）；

10. **4.4.1 的 vitest 清单**：旧文字

   > 、`W/pages/notebook/transfer-background.test.tsx:482,611,642,690`（在导入导出页上等树）。

   换成：

   > 、`W/pages/notebook/transfer-background.test.tsx:482,611,642,690`（在导入导出页上等树）；读设置页的标题与右上角的另有 `W/pages/settings/profile-page.test.tsx`、`W/pages/create-workspace.test.tsx`、`W/pages/invitation.test.tsx`、`W/pages/workspace/workspace-home.test.tsx`（"离开之后答复"的三格跳到个人设置等 `h1`"设置"）、`W/app/layout.test.tsx`、`W/app/user-menu.test.tsx`、`W/pages/workspace/workspace-layout.test.tsx`，共 12 个文件，改法见 [03-P3](03-P3-settings.md) 7.1。

11. **4.4.1"处置"**：旧文字

   > ownerless 的那一段改往页面里别的输入框打字，覆盖不减；members 的最后一段改从目录的"笔记本"组进去。

   换成：

   > ownerless 的那一段往 `body` 上挂的一个输入框里打字（页面没有别的输入框），覆盖不减；members 的最后一段是"返回"、点 Atlas、点它的齿轮、再点目录"笔记本设置"一组的"成员"（目录的笔记本组只有当前那一本，[03-P3](03-P3-settings.md) 7.1）。

12. **4.4.3"下拉即生效"**（决定 840）：旧文字

   > 被拒时退回原值；发送中不禁用下拉（禁用会丢焦点）。

   换成：

   > 被拒时退回原值；发送中不禁用下拉（禁用会丢焦点）；键盘走到的一项还没发出时，说明照显示的那一项，Esc 退回已生效的值（[03-P3](03-P3-settings.md) 决定 840）。

13. **4.8 UI3**：旧文字

   > 把本人降为成员之后再选

   换成：

   > 把本人降为编辑者之后再选（笔记本的角色是管理员、编辑者、读者）

14. **第 7 节 P3/S1 一行**（决定 812）：旧文字

   > | 设置目录组件、返回与来处的 store、焦点；卡片行组件；

   换成：

   > | 设置目录组件、返回与来处（记在 `Layout`，[03-P3](03-P3-settings.md) 决定 812）、焦点；卡片行组件；

15. **第 7 节 P3/S2 一行**（决定 826、840）：旧文字

   > 4.4.3 的即时生效与草稿 |

   换成：

   > 4.4.3 的即时生效与草稿（说明随显示的项，Esc 退回待用的选择）；"危险操作"组 |

16. **第 7 节"总体设计的修订"第 6 项，第 31 条**（决定 845）：旧文字

   >    - 第 31 条（对比度与焦点的几句）与新条目"设计基础的变量"（含 4.1 末的例外）：P1/S1；

   换成：

   >    - 第 31 条（对比度与焦点的几句）与新条目"设计基础的变量"（含 4.1 末的例外）：P1/S1；第 31 条强制色下的透明边框加上主按钮与左栏导航的当前一行：P3/S1；

17. **第 7 节"总体设计的修订"第 6 项，第 20 条**：旧文字

   >    - 第 20 条（快捷键：⌘K 与它的让路规则）：P2/S3；

   换成：

   >    - 第 20 条（快捷键：⌘K 与它的让路规则）：P2/S3；设置模式里两个键都不管：P3/S1；

18. **第 7 节"总体设计的修订"第 6 项，第 12 条**：旧文字

   >    - 第 12 条（设置页的下拉即生效、持草稿；单选组仍经保存）：P3/S2；

   换成：

   >    - 第 12 条（"已保存。"的勾与改名称的一行：P3/S1；设置页的下拉即生效、持草稿，单选组仍经保存：P3/S2）；

19. **9.1"设置模式"一行**：旧文字

   > 在设置页上读侧栏的 7 个故事（4.4.1）；设置页的 `h1` 2 个文件（a15、a8） | 4.4.1 列出的 5 个文件 |

   换成：

   > 在设置页上读侧栏的 8 个故事（4.4.1）；设置页的 `h1` 2 个文件（a15、a8） | 4.4.1 列出的 12 个文件（[03-P3](03-P3-settings.md) 7.1） |

20. **第 10 节 a15 一行**（决定 829、843）：旧文字

   > | P3/S1 跑 a15 三遍；列表在缓存里时不另读 |

   换成：

   > | 个人设置只在有工作区可找（来处在某个工作区里，或本设备记着最近的工作区）且列表没读过时读它（[03-P3](03-P3-settings.md) 决定 843）：a15 的页一次也不多读，照常跑 |

21. **头的状态（设计提交）**：旧文字

   > | 状态 | 设计定稿，负责人 2026-11-22 批准；P1 完成；P2 完成，P3 待开 |

   换成：

   > | 状态 | 设计定稿，负责人 2026-11-22 批准；P1 完成；P2 完成；P3 设计定稿，S1 实施中 |

22. **第 12 节 P3 一行（设计提交）**：旧文字

   > | P3 设置界面（S1、S2） | — | 未开始 | — | — |

   换成：

   > | P3 设置界面（S1、S2） | [03-P3](03-P3-settings.md) | 设计定稿；S1 实施中 | [设计审查](reviews/P3-design-review.md) | — |

23. **第 13 节末（设计提交）**：旧文字

   > 合并 `82838aac`。
   > - 2026-11-23：P2 完成（S1 `fdad6b9e`、S2 `70338fc4`、S3 `82838aac`；主持者的决定 769–809；本机门禁替代分支 CI，负责人推送之后 CI 在最后一个头上补跑，决定 746、761；se2 在 se2:230 之后的步骤与 UI1 的 `Control+K` 只在 Linux 的 CI 上走，决定 795(d)）。

   之后加：

   > - 2026-11-23：照 P3 的设计修订（[03-P3](03-P3-settings.md) 2.1；[设计审查](reviews/P3-design-review.md)，主持者的决定 810–847）：设置目录的两个宿主；来处记在 `Layout`（连同片段与工作区，以 `replace` 进入时不记）与"返回"的去处；个人设置的工作区一组；设置模式里树留在 `hidden` 的容器里；页名作 `h1`，三页的到达焦点；右上角对有待办引导步骤的账户、个人设置的"回到工作区"删掉；快捷键照旧挂着；读侧栏的故事 8 个、vitest 12 个文件；下拉的说明与 Esc；UI3 降为编辑者；第 7 节 P3 两行与"总体设计的修订"第 6 项；第 10 节 a15 一行。

24. **[02-P2](02-P2-shell.md) 第 9 节"P3"一条，树**（决定 814、836）：旧文字

   > （`SlotHolder` 卸下，容器随 `NotebookLayout` 留着或卸下，回来时重画）

   之后加：

   > （P3 改判：树留在一个 `hidden` 的容器里，[03-P3](03-P3-settings.md) 决定 814、836）

25. **[02-P2](02-P2-shell.md) 第 9 节"P3"一条，快捷键**（决定 819）：旧文字

   > `useSearchShortcuts` 不挂（00 号文档 4.2.4"设置模式里……两个快捷键都不做事"）

   之后加：

   > （P3 改判：照旧挂着，框不在时不 `preventDefault`，[03-P3](03-P3-settings.md) 决定 819）

### 2.2 需要负责人确认

（无）

### 2.3 本文的决定（主持者的决定，负责人可以改判）

810–832 是初稿的，设计审查之后照审查记录的处置改了 812–818、820、822、823、825、828–830、832 的说法（改的地方注明新的决定号）；833–847 是审查之后的（[审查记录](reviews/P3-design-review.md)"主持者的决定"，括注是发现的编号）。修订核对第 1 轮之后又照处置改了 822、825、839、840 的说法（发现编号带 R1）；第 2 轮之后加了 848，改了 822、823 的说法（R2）。 849–851 是 S1 的实施审查之后的（[S1 的审查记录](reviews/P3-S1-settings-mode-review.md)"主持者的决定"，括注是发现的编号）。 852–853 是 S2 的实施审查之后的（[S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)"主持者的决定"，括注是发现的编号）。

- **810 目录的宿主**：工作区与笔记本的设置由 `WorkspaceLayout` 把自己的左栏换成目录：仍是同一个 `ShellSidebar`（同一个位置、同一种组件，React 保留它与它的抽屉、菜单按钮、细栏），`top` 换成 `SettingsTop`，中段换成 `SettingsDirectory`，`place` 换成"设置"；个人设置的 `SettingsLayout` 自己 `useDrawer()`、画 `ShellSidebar`、给 `Drawer` 上下文，门户进 `Layout` 的列。工作区与笔记本的两个设置布局只剩文档标题与按页挂载的 `SettingsPage`。理由见 2.1 第 1 条；另一个做法（三个布局各画目录）在笔记本读取中、失败与 404 时没有左栏，在笔记本与工作区设置之间跳转时整个左栏重新挂载。
- **811 设置模式的判定**：三个设置路由带 `handle: { settings: true }`，`/settings` 另带 `sidebar: true`；`useSettingsMode()` 读 `useMatches()`，同 `Layout` 的 `sidebar`，渲染时就知道，不用 effect（一个 effect 会先画一帧工作区的左栏）。`/settings` 有了 `sidebar`，个人设置不再有右上角，`corner.tsx` 的 `settings`（`:56`）与它在 `back` 里的一项（`:65`）删掉，"回到工作区"只剩新建工作区页的（[02-P2](02-P2-shell.md) 决定 803）。
- **812 来处**：`Layout` 用 `useOriginRecord(settings)` 在渲染时比较这一次与上一次的 `location`（同 `useDrawer` 取窗口宽度的写法）：从非设置页进入设置时记下上一页的 `pathname`、`search`、`hash` 与它所在的工作区（上一次渲染时 `useParams()` 的 `slug`），设置页之间（个人、工作区、笔记本互相跳转，目录随之重新挂载）保留；以 `replace` 进入时不记（决定 833）；离开设置时不清（没有可见的效果：目录只在设置模式里读它，再进入时重取；审查 C 的 M1），经 `SettingsOrigin` 上下文给目录。在 `Layout` 的状态里，标签页的会话期间都在，刷新就没有了。不放路由状态：三个 `index` 重定向是 `replace`，浏览器的后退与前进也会把状态带乱。来处是进入这一次设置之前渲染的那一页：一次后退、前进跨过几条回到设置时，是刚离开的那一页（审查 A 的 N2）。
- **813 返回**：目录顶部的"返回"是 `Link`（不是 `history.back()`：直接打开的地址没有可回的一页，后退还可能离开应用），带 `arrived`，去处与文字：(1) 来处在一本列出的笔记本里，"返回 {笔记本}"，回来处；(2) 来处在一个列出的工作区里、它的笔记本列表已读而没有那一本（删了、离开了），"返回 {工作区}"，回工作区首页；(3) 来处在一个列出的工作区里的别处（首页、搜索），或笔记本列表还没读，"返回 {工作区}"，回来处；(4) 来处不在任何工作区里（新建工作区页、邀请页），"返回"，连片段回来处（决定 834）；(5) 来处在一个工作区里、工作区列表已读而没有它（离开了、删了，或来处就是"找不到工作区"），"返回"，回 `/`（审查 A 的 N3），列表还没读时"返回"、回来处；没有来处时，目录有工作区就"返回 {工作区}"、回它的首页，没有就"返回"、回 `/`。(1)–(3) 不带片段（决定 834）。名字随列表改名即时变，放在 `bdi` 里（`isolate`/`Isolated`，M12/P2/S3 的写法）。新键 `settings.back`"Back"/"返回"、`settings.backTo`"Back to {name}"/"返回 {name}"（中文的空格同"移动 {name}"）。
- **814 笔记本设置里的树**：见 2.1 第 5 条。`WorkspaceLayout` 在设置模式、地址里有笔记本 id 时，在目录之后画 `<div hidden><SlotHolder id={…} /></div>`。树、它的读取、`TreeWatch`、导入导出与模式一节依赖的树的 store 都与今天一样；看不见，也不在可访问性树里（Playwright 的 `getByRole` 与 testing-library 的 `findByRole` 默认都不找）。进入时列表项的 `SlotHolder` 卸下、记下拿着的焦点（齿轮），`hidden` 里的那个收养容器、试着把焦点还给齿轮（浏览器里 `display: none` 的元素取不到焦点；jsdom 里取得到），还不上时不调 `intoColumn`（审查 A 的 M3），去向页的 `h1` 随后取到达的焦点（`takeArrivalFocus` 让为一次导航挂载的第一个标题不论焦点在哪都取走它，`W/app/arrival.ts:65-94`）；返回时列表项收养回去（`moving` 为真，树的菜单重量，决定 800）。理由是决定 836。
- **815 页的 `h1`**：设置页的 `h1` 是页名，由布局画（`SettingsPage`，`key` 是页的路径，`useArrivalFocus`，`tabIndex={-1}`）：同一个布局里换页时 `h1` 重新挂载，到达时（抽屉里、跨外框、进入）它取焦点，`useArrivalFocus` 的 effect 只在挂载与 `[arrival, fromNoOne]` 变时跑（它的依赖），所以同外框连着两次到达也靠 `key` 才取（审查 C 的 I1）。S1：个人设置是页名（提交 1 起；提交 1 里个人资料页的 `h2`"个人资料"还在，提交 2 去掉）；工作区与笔记本设置照旧"工作区设置""X 设置"（按页挂载、取到达的焦点），页里的 `h2` 不动（决定 831）。S2：换成页名，去掉与它重复的第一个 `h2`（"常规""成员"），列表删除之后焦点到 `h1`（`useSettingsHeading()`；S1 起令牌页已这样）。留着的 `h2` 是分组标题：基本信息、模式、邀请、添加成员、最近的任务、导入、导出、审计日志、密码、危险操作。索引重定向的那一次渲染没有页，只有 `Outlet`。`h1` 的行高照样稿 1.5（`leading-normal`，审查 B 的 N2）。
- **816 链接与路由状态**：目录里同一个外框的页（工作区设置与笔记本设置之间、个人设置四页之间）经 `useFollow`：平时 `opened`、焦点留在链接上，抽屉开着时 `arrived`，点当前地址时 `replace`（决定 782、808 的规则）；跨外框的（个人与工作区、笔记本之间）带 `arrived`；"返回"带 `arrived`；进入设置的链接（工作区导航的"设置"、笔记本行的齿轮、账户菜单的"个人设置"）带 `arrived`：它们随左栏换成目录而离开。账户菜单的"个人设置"在个人资料页上不带（`state={atProfile ? undefined : arrived}`：地址相同，`Link` 自动 `replace`，不算到达，宽屏时菜单把焦点还给按钮，375 px 时关上的抽屉把焦点给菜单按钮；审查 A 的 M2）。工作区导航的"设置"直接指 `/${slug}/settings/general`（少一次重定向；`workspace-layout.test.tsx:113` 随之改）。三个 `index` 重定向换成 `IndexTo`，照 `LandingPage` 把收到的状态转交下去。`NavItem` 加 `state` 与 `onClick` 两个可选的属性。
- **817 组标题与名称**：目录自上而下：返回、标题"设置"（`p`，不是标题元素，页的 `h1` 才是）、三组、底部的账户行。每组是一个 `nav`，名称照旧"设置""工作区设置""笔记本设置"（tr1、tr2、n9、`members-page.test.tsx` 等以它们找链接），组的第一个子元素是一行纯文字的组标题（不是标题元素：一页只有一个 `h1`，左栏里的 `h2` 只有笔记本行），工作区与笔记本组的标题带名字，名字在 `bdi` 里，与标题之间有一个空格（`{" "}`，读屏与 `textContent` 读作"工作区 Lab"；审查 A 的 N1），标题包在 `shrink-0` 的 `span` 里，名字长时只省略名字（审查 B 的 I4）；组里的链接文字照旧（"成员"不改名，n9:188 读它），个人一组多一项"外观与语言"（S1 的提交 2）。细栏的位置文字是"设置"。新键 `settings.groupPersonal`"Personal"/"个人"、`settings.groupWorkspace`"Workspace"/"工作区"、`settings.groupNotebook`"Notebook"/"笔记本"、`settings.appearance`"Appearance and language"/"外观与语言"。
- **818 个人设置的工作区组**（照决定 837、843 改）：来处所在的工作区（`Origin` 的 `workspace`），按当前账户的列表找；没有来处、来处不在工作区里、或列表里没有它时，用本设备最近去过的工作区（`preferences.lastWorkspace()`，挂载时读一次：它不是可观察的，`W/stores/preferences.store.ts:89-96`）；列表没读过、又有要找的工作区时读一次（`useSWR(wanted !== undefined && list === undefined ? "workspaces" : null)`，同键，读过就不再读）；读取中、失败、都不在列表里时不显示这一组，"返回"没有来处时回 `/`。
- **819 快捷键**：`useSearchShortcuts` 照旧挂在 `WorkspaceLayout`；设置模式里没有搜索框，`field.current` 是 `null`，Mod+K 与 Mod+Shift+F 都直接返回、不 `preventDefault`（浏览器自己的 Ctrl+K 照常）；个人设置没有这个监听。见 2.1 第 8、25 条。
- **820 卡片组件**：在 `W/app/settings-page.tsx`（它用 `useArrivalFocus`，`W/components/` 不依赖 `W/app/`）：`SettingsPage`、`SettingsSection`、`SettingsCard`、`SettingsRow`、`SavedOutput`、`useSettingsHeading` 与一行的几段类名常量（4.1.7；S1 的提交 1 只有 `SettingsPage` 与 `useSettingsHeading`，决定 842）。页的内容最宽 696 px（`max-w-178`，决定 844）。卡片的底：亮暗都是 `--surface`（样稿暗色写死 `#1f2023`，与 `--surface` 的 `#212226` 差两级，不另加变量）。卡片里的按钮 30 px、胶囊形（`size="sm" shape="pill"`，00 号文档 4.3"工具栏与卡片里用胶囊形"），令牌页的"新建令牌"同样（审查 B 的 N6）；样稿的"保存"是 26 px 的 `.btn.sm`，取 30 px 与别的按钮一致、点按面积够。
- **821 提交型的表单**：改密码、新建令牌、邀请、添加成员照旧是表单，放在卡片里的一块（`px-4.5 py-4`），字段竖排，按钮照旧 36 px、8 px 圆角（00 号文档 4.3"表单底部用 8 px 圆角"）。
- **822 文字输入的一行**：`W/app/name-row.tsx` 的 `NameRow`：标题是输入框的 `label`，说明在标题下；"保存"只在输入框里的字不同于当前的名字时出现，它的位置一直留着（宽屏 `md:min-w-15` 的槽，"保存"靠右；375 px 时空的槽不占位，审查 B 的 R1-N1），输入框不随它出现、消失而挪（审查 B 的 M6）；回车也保存，失焦不保存；发送中"保存"是 `aria-disabled` 与 `aria-busy`、点了不再发（`disabled` 会丢焦点，13.2 第 12 条）；保存成功、"保存"随之消失时焦点交给输入框（`useFocusLeaving`，"保存"自成一个组件，挂载时取到元素）；问题显示在输入框下面（宽屏上在调用处加 `md:-me-21 md:ps-19`：负的外边距抵掉它前面的间距与它自己的内边距，输入框与"保存"不随它出现而挪，内边距让它的字与输入框左对齐，不落到"已保存。"预留的位置下，审查 B 的 R1-M1、决定 848；375 px 时以 `order` 排在输入框之后、"保存"之前），`aria-describedby` 只指一个 id：有问题时是问题，否则是说明（e2e 的 `noteOf` 只读一个 id，`e2e/fixtures/browser.ts:83-88`）；问题、`aria-invalid` 与横幅只在字段与当前的名字不同时显示：改回原名时它们说的那份草稿已不在，随之不显示（审查 A 的 M1）；没有编辑时回车什么也不发。草稿与编辑次数的规则同 `RenameForm`。`RenameForm`（对话框里的改名）与 `DisplayNameForm`（引导的一步）照旧。
- **823 "已保存。"**（照决定 841 改）：`SavedOutput` 是一直挂载的 `<output>`，文字只是"已保存。"或空（13.2 第 12 条；n3:78、n6:79、w4:115 以精确的"Saved."读它），在一行的控件之前（375 px 时之后）；宽屏上一直占 68 px（`md:min-w-17 md:justify-end`，字靠右贴着控件，决定 848），375 px 时空的时候以 `max-md:empty:-me-2` 抵掉它后面的间距（审查 B 的 I2）；✓ 是 `styles.css` 的 `.nw-saved:not(:empty)::before`：13 px 的遮罩，路径同样稿（`m5 12 5 5 9-10`，审查 B 的 N3），`color: var(--accent)`、`background-color: currentColor`，强制色下 `forced-color-adjust: none; color: CanvasText`；整个输出 1.5 秒后 300 ms 淡到透明度 0（文字留着，读屏照旧读得到）；每次写出时轮次（`data-round`）在 0、1 之间换，两个轮次取两份同样的 `@keyframes`，淡出必定从头来，不论答复多快（决定 841）；每次发出时照旧先把它清空（发送中不显示上一次的"已保存。"）。减少动效时照旧淡出：只是透明度，不是运动。
- **824 外观与语言**：新页 `/settings/appearance`（路由在 profile 之后），一张卡片两行：主题（说明"保存在这个浏览器中。"）与语言，各是一组单选（`role="radiogroup"`、`aria-labelledby` 指行的标题），选了即生效、只存本机，同账户菜单的"外观"；个人资料页去掉偏好一节，`preferences.title` 一键随之删掉。
- **825 开放程度**（S2）：笔记本常规页的开放程度换成一行里的下拉：`useChoice`（鼠标选了即发，键盘逐项走时等失焦或回车，有待用的选择时 Esc 退回，决定 840）加一份草稿（发出之后下拉显示选的那一项，直到最后一次的答复回来）；`sent` 计数，只有最后一次发出的答复算：之前的成功不清草稿，之前的拒绝不显示；最后一次被拒时草稿清掉、下拉退回列表里的值，原因经 `useForm` 的横幅显示在这一行下面，403 说"Only the notebook's admins can change its workspace access."（`texts: { forbidden: "notebookSettings.accessAdminsOnly" }`）；发送中不禁用下拉，最后一次的答复回来之前带 `aria-busy`（取草稿在不在，不取 `useForm` 的 `sending`；审查 B 的 N7、R1-N2）；说明是下拉显示的那一项的（`choice.value`，键盘走到、还没发出的也是，决定 840）。`accesses` 从 `access-options.tsx` 导出给它用；新建笔记本的对话框照旧是单选。
- **826 危险操作组**（S2；个人设置在 S1）：每页最后一组"危险操作"（`settings.danger`"Danger zone"/"危险操作"），里面是删除工作区、删除笔记本、离开工作区、离开笔记本、停用账户，各一行：标题、说明、右边红字描边的胶囊按钮，按钮文字照旧（e2e 以它们找按钮），点了照旧确认。"模式"（降级、升级）自成一组，不进危险操作：升级不是不可逆的删除，降级有自己的确认。列表行内的不可逆操作不用红（决定 838）。
- **827 基线**：P2 最后一次门禁的代码与 `ba66557c` 相同（第 1 节），不另跑基线；S1 第一个改行为的提交的 `make e2e` 兼作核对：只败第 1 节那几格（同行同消息）。
- **828 内容里通往设置的链接**（照决定 839 改）：别处通往设置的内容链接（工作区首页无主笔记本提醒里的"Review them"，`W/pages/workspace/workspace-home.tsx:87`；笔记本首页的"笔记本设置"，`W/pages/notebook/notebook-home.tsx:69`，P5 去掉；变更集页的"See its progress"，`changeset-page.tsx:253`；回收站预览的"See the jobs"，`trash-preview.tsx:361`）留给 P5；设置页里随页离开的链接 S2 都带 `arrived`（决定 839）。
- **829 a15**（照决定 843 改）：个人设置页上只在有工作区可找时读列表（决定 843），a15 的页一次也不多读；a15 随每次 `make e2e` 照常跑，不另跑三遍。只改 `h1` 的名字与五处 `h2`（第 5 节）。
- **830 移交的认领**（照决定 846 改）：M12-polish 第 1 项里的 19.2（英文的任务行"Started by Bob · Started …"说两次 started：`transfer.startedBy` 改成"By {name}"，中文照旧，S2；现象与改法写在 M5-polish 第 19 条）；01-P1 第 9 节的手写标签组（`create-token-dialog.tsx:184` 在 S1；`invitations-section.tsx:130`、`add-member-section.tsx:102,132`、`import-dialog.tsx:318,361` 在 S2；`space-y-2` 改 `space-y-1.5`）；01-P1 第 9 节"P3"的文件按钮悬停（`Input` 加 `file:hover:hover-layer`，S2，三种引擎截图核对）与 30 px 的下拉（`h-7.5`）。M12-polish 第 1 项的 19.5、19.6、20.4、31.1 不认领，照旧留给 v0.2（19.1、19.3、19.4、20.1–20.3 在 M12 已做完）。
- **831 S1 的边界**：S1 不动工作区与笔记本设置的页面内容（只去掉布局里的 `nav`、换 `h1` 的挂载），S2 换。S1 的个人设置已是卡片，所以 S1 与 S2 之间两种页面并存，这是过渡，不是缺陷。
- **832 引导中的账户**：`Layout` 读账户（`observer`），`me` 已读且还有待办的引导步骤时不认 `sidebar`：`Onboarded` 会把任何带左栏的路由送回引导，弹回的那一次渲染里右上角与它的账户按钮不卸下，菜单把焦点还给它（`profile-page.test.tsx:70-88` 守它；第 1 节末）。`/`、`:slug` 的弹回同样留住右上角（今天那一次渲染没有右上角；引导页上没有通往它们的入口，审查 A 的 N4）。
- **833 以 `replace` 进入设置不记来处**（审查 A 的 I1）：只有重定向这样进入设置（守卫登录之后的 `next`、引导做完的 `next`），被换掉的那一页（登录页、引导页）不是回去的地方：它们会立即把人送回设置。没有来处时走 813 的默认去处。设置页之间的 `IndexTo` 是设置里的 `replace`，来处照旧保留。
- **834 来处带片段**（审查 A 的 I3）：`Origin` 有 `hash` 与 `workspace`；工作区之外的来处连片段回去（邀请的令牌在片段里，`W/pages/invitation.tsx:33-36`）；工作区里的来处不带：页面的 `h1` 在 passive effect 里取到达的焦点、锚点的元素在视图读到时的 layout effect 里取，视图在缓存里时 `h1` 后到、取走（回到页首），不在时锚点后到，两种结果不同；不带片段，回到页首、`h1` 拿着焦点，是确定的那一种（第 1 节"到达的焦点与锚点"）。
- **835 三页的 `h1` 取到达的焦点**（审查 A 的 I2）：变更集页（`changeset-page.tsx:99`）、回收站的预览（`trash-preview.tsx:406,579`）的 `useRef` 换成 `useArrivalFocus`（`tabIndex={-1}` 已有，`left={() => heading.current?.focus()}` 照旧用这个 ref；`trash-preview.tsx` 两个 effect 的依赖加上 `heading`）；邀请页的 `h1` 加 `useArrivalFocus`、`tabIndex={-1}` 与 `outline-none`。S1 的提交 1。
- **836 设置模式里树留在 `hidden` 的容器里**（审查 A 的 Q1）：`NotebookLayout` 在笔记本设置里一直挂着、一直把树门户进容器，树在 React 里从不卸下，差别只在容器在不在文档里；直接打开笔记本设置的地址时，容器不在文档里，[02-P2](02-P2-shell.md) 决定 776（树的 effect 运行时容器已在文档里）与 791（`reveal` 时容器总已收养）的前提就不成立；`hidden` 保住它们。02-P2 第 9 节的"卸下"由此改判（2.1 第 24 条加括注）。
- **837 个人设置的工作区一组先取来处的**（审查 A 的 Q2；改 818）：两个标签页分别在 A、B 时，从 A 进入个人设置，组里是 A，与"返回 A"一致；没有来处或来处不在工作区里时才用 `lastWorkspace()`。
- **838 列表行内不可逆的操作一律 `outline`**（审查 B 的 Q1）：成员的"移出"、令牌与邀请的"撤回"、无主笔记本的"删除"是描边的 30 px 胶囊，确认框照旧；红只留给"危险操作"组（同样稿的克制）。
- **839 设置页里随页离开的链接带 `arrived`**（审查 B 的 Q2；改 828）：笔记本成员页 `NoAdmin` 的链接（`W/pages/notebook/members-page.tsx:148`）、无主笔记本页接管之后的"Open it"（`W/pages/workspace/ownerless-page.tsx:137`）、模式一行升级进行中的"View the upgrade"（`upgrade.view`，`W/pages/notebook/general-page.tsx:192`）、任务行的"See in the history"与"The change undone"两个链接（`W/pages/notebook/transfer-job-row.tsx:155,164`）、升级进行中每个笔记本设置页顶上横幅里的"View the upgrade"（`W/pages/notebook/upgrade-banner.tsx:47`），共六处，S2 改这几页时一并带上（13.2 第 17 条）。任务行的第二个链接是设计者核对时加的，横幅里的是审查 B 在修订核对里找到的（R1-N3），同样随页离开。
- **840 `useChoice` 的 Esc；开放程度的说明随显示的值**（审查 B 的 Q3、M5）：`useChoice` 有待用的选择时 Esc 退回已生效的值、`preventDefault` 与 `stopPropagation`，这一键到此为止；没有待用的选择时不处理 Esc。今天的用者（搜索的范围、目录与历史的筛选）都不在对话框与抽屉里；Radix 对话框的 Esc 是 document 上的捕获监听，先于它（探针：对话框先关，没有发出）；375 px 的抽屉碰不到它：抽屉的 Esc 只听自己的根，开着时焦点困在抽屉里、遮罩盖着 `main`，打开抽屉时下拉先失焦、待用的选择随之用上（P2 决定 807；审查 A 的 R1-N1，初稿说的"`main` 是 `inert`"不对，4.2.5）。说明取 `choice.value`。S2 的提交 2。
- **841 淡出交替两份 `@keyframes`**（审查 B 的 I3、审查 C 的 I3；改 823）：清空与写回落在同一帧的两次样式计算之间时，Chromium、Firefox 不重来同一个动画（审查 B、C 的探针）；每次写出时换动画名，三种引擎、任何间隔都重来（修订的探针 20/20）。轮次由 `SavedOutput` 自己在 `saved` 由假变真时换（渲染时 `setState`，同 `useChoice` 的 `was`），不按发出数：两次发出之间只显示一次时，按发出数可能两次同名。
- **842 S1 拆成两个代码提交**（审查 C 的 Q1）：见第 0 节与 6.1；UI2 随它核对的设置模式在提交 1。
- **843 `useLastWorkspace` 只在有工作区可找时读列表**（审查 C 的 M8；改 818、829）：键是 `wanted !== undefined && list === undefined`（`wanted` 是来处的工作区或最近的工作区）；没有来处、本设备也没记着工作区时这份列表用不上。函数改名 `useDirectoryWorkspace`（它不只取最近的工作区了）。
- **844 页宽与"基本信息"**（审查 B 的 M1、M2）：`SettingsPage` 是 `max-w-178`（712 = 696 + 两边 8 px，样稿的 760 含两边 32 px，内容 696）；两个常规页的第一组带标题"基本信息"（新键 `settings.basics`"Basics"/"基本信息"，S2）。
- **845 强制色下的两处**（审查 B 的 N5）：`Button` 的 `default` 变体加 `border border-transparent`（强制色下由系统色画出；宽各多 2 px）；`NavItem` 的当前一行加 `border border-transparent`、`px-1.75`（字不挪，强制色下是 1 px 的系统色边框，别的行没有）。前者 S1 的提交 2，后者提交 1。
- **846 移交的进展写进 M12-polish**（审查 B 的 M3、审查 C 的 I4；改 830）：M13 的认领写进 `docs/v0.2/handoffs/M12-polish.md` 的"处理进展"（S2 的文档提交，M5-polish 已是 `done`，不动）；第 9 节交给 P4、P5 的标签组补上 `history-filters.tsx:132,143`、`index-filters.tsx:48`；标签组与文件按钮的完成写进本文 10.2 与 01-P1 第 9 节的交接处。
- **847 2.1 随设计一起提交**（审查 C 的 M7）：见 2.1 的开头；那条 `routes.tsx` 的替换整句带上下文，恰好一次；9.1 改成 8 个故事。
- **848 宽屏上"已保存。"预留宽度**（审查 B 的 R2-M1；改 823）：`SavedOutput` 带 `md:min-w-17 md:justify-end`，宽屏上不论空不空都占 68 px、字靠右，它的出现与消失在任何宽度下都不让控件挪动或折行；窄屏照旧 `max-md:order-last max-md:empty:-me-2`。代价照记（名称一行的说明列 326 → 250 px，英文开放程度说明宽屏两行，4.1.7）；`NameRow` 的问题提示随之在调用处加 `md:-me-21 md:ps-19`（盖过 `rowProblemClass` 的 `md:-me-2`），字与输入框左对齐（4.1.7；审查 B 的 R3-N2 把它从通用的常量挪到调用处；这一处是设计者核对预留宽度时加的：不加时问题的字从预留位置的左边起，比输入框靠左 76 px）。
- **849 `SettingsRow` 是 `observer`**（审查 A 的 P3S1A-Q1）：一行的控件由 `children(ids)` 在 `SettingsRow` 自己的渲染里画，不在页的 `observer` 之内；不是 `observer` 时，控件读到的 store 变了它不重画。实施时外观页点"Dark"之后主题换了、单选不跟着变（a8 在 Chromium 里抓到；vitest 那一格只在换语言、整页重画之后才看单选），先以"页在自己的渲染里先读主题与语言"绕开；A 指出 S2 的行（开放程度）会再踩同一个坑，给了两种做法：`SettingsRow` 包成 `observer`，或 `children` 改为直接传元素。取前者：`children` 的写法不变，规则由组件担，不靠每页记得先读。外观页改为在 `children` 里读（变异 67：`settings-page.test.tsx` 跟着 store 一格、`appearance-page.test.tsx` 选了即生效一格）。
- **850 设置卡片里列表的名字是 `text-sm`**（审查 B 的 P3S1B-N2）：令牌行的名字照样稿 `.set .t` 是 14 px（`text-sm`、`font-medium`），不照 4.1.7 的"左边的文字块照旧"（`member-summary.tsx` 的名字是 16 px）；S2 的列表随之，设置里只有一种列表名字的字号。经 `MemberSummary` 的（成员行、无主行、审计、添加成员）改它一处；邀请行与任务行不经它，S2 各自改（修复核对 B 的说明）。
- **851 第一组的标题只会重复页名时不写**（审查 B 的 P3S1B-Q1；照修复核对 B 的说法改写）：个人资料、外观与语言、访问令牌三页的第一张卡片上没有分组标题（4.1.10 的 `SettingsSection` 不带标题，卡片在 `h1` 下 28 px），S2 的工作区成员页、笔记本成员页与无主笔记本页的第一组同样（4.2.2 去掉的就是与 `h1` 重复的那一个）：标题只会重复页名。别的照写：两个常规页的"基本信息"（决定 844）、安全页的"密码""危险操作"、各页其后的分组。是有意的偏离，记进 7.4。
- **852 卡片之上的组说明同宽**（审查 B 的 P3S2B-N5；主持者的决定，负责人可以改判）：组标题与卡片之间的说明（邀请、添加成员、无主笔记本、审计日志、最近的任务）都是 `max-w-2xl`；邀请与添加成员原来是 `max-w-xl`，英文的邀请说明在 1440 宽折成"no e- / mail."。
- **853 移动页面对话框的两个下拉在标签之下**（审查 B 的 P3S2B-Q1；主持者的决定，负责人可以改判）：`W/pages/notebook/move-page-dialog.tsx:161,174`（`397241c7` 的行号）的两个下拉加 `block`，同 S2 给邀请、添加成员与导入对话框的下拉（决定 821 的字段竖排）；文件是 P2 的，并进 S2（`1e6231e9`）。历史与目录筛选的标签组照旧随 P4、P5 剩下的标签组（第 9 节）。

## 3. 服务端

无。不改接口与契约（7.3 的 `git diff --quiet`）。

## 4. 网页

### 4.1 S1 设置模式与个人设置

#### 4.1.1 文件

"提交"一列是 S1 的哪个提交（决定 842，6.1）。

| 文件 | 改动 | 提交 |
|---|---|---|
| `W/app/routes.tsx` | `/settings` 带 `handle: { sidebar: true, settings: true }`；笔记本设置（`:237`）与工作区设置（`:278`）带 `handle: { settings: true }`；三个 `index` 换成 `<IndexTo to=… />`（4.1.2）；`/settings/appearance`（在 profile 之后，懒加载 `AppearancePage`）在提交 2 | 1、2 |
| `W/app/layout.tsx` | 成为 `observer`；`bounced`（决定 832）；`useOriginRecord(useSettingsMode())`，`<SettingsOrigin value={origin}>` 包住 `Outlet`；注释照改 | 1 |
| `W/app/settings-origin.ts`（新） | `Origin`、`useSettingsMode`、`useOriginRecord`、`SettingsOrigin`、`useSettingsOrigin`（4.1.3） | 1 |
| `W/app/settings-directory.tsx`（新） | `personalSections`、`workspaceSections`、`notebookSections`、`sectionAt`、`useDirectoryWorkspace`、`useBack`（模块内）、`SettingsTop`、`GroupTitle`（模块内）、`SettingsDirectory`（4.1.4）；`personalSections` 的"外观与语言"一项在提交 2 | 1、2 |
| `W/app/settings-page.tsx`（新） | 提交 1：`SettingsPage`、`useSettingsHeading`；提交 2：`SettingsSection`、`SettingsCard`、`SettingsRow`、`SavedOutput`、一行的类名常量（4.1.7） | 1、2 |
| `W/app/name-row.tsx`（新） | `NameRow` 与模块内的 `SaveButton`（4.1.8） | 2 |
| `W/app/corner.tsx` | 删去 `settings`（`:56`）与 `back` 里的那一项（`:65`）；`Corner` 的注释（`:43-51`）去掉"after Back to the workspace on the settings" | 1 |
| `W/app/user-menu.tsx` | "设置"一项 `state={atProfile ? undefined : arrived}`（决定 816；审查 A 的 M2）；注释（`:50-53`）："which stays at the top right of the settings"改为"which stays in the column's foot on the settings"，加一句：on the profile itself no arrival, the link replacing the address shown；`leaving` 的逻辑不变（4.1.12） | 1 |
| `W/components/nav-item.tsx` | 加可选的 `state` 与 `onClick`，原样交给 `NavLink`；当前一行加 `border border-transparent px-1.75`（决定 845） | 1 |
| `W/components/ui/button.tsx`、`button.test.tsx` | `default` 变体加 `border border-transparent`（决定 845），`:56` 的类名表随之加上 | 2 |
| `W/pages/workspace/workspace-layout.tsx` | 设置模式（4.1.5）与 `ScrollKept`；`sections` 的"设置"指 `/settings/general`、带 `leaves`（渲染时给 `state={arrived}`） | 1 |
| `W/pages/workspace/notebook-slot.tsx` | 焦点还上了才 `intoColumn`（`:127-130`，4.1.5） | 1 |
| `W/pages/notebook/page-tree.tsx` | 齿轮（`:291`）加 `state={arrived}`，引入 `arrived` | 1 |
| `W/pages/notebook/changeset-page.tsx`、`trash-preview.tsx`、`W/pages/invitation.tsx` | `h1` 取到达的焦点（决定 835；变更集页的 `h1` 在读到之后才挂载的 `ChangesetRead` 里，审查 A 的 P3S1A-M1，4.1 的照实） | 1 |
| `W/pages/settings/settings-layout.tsx` | 改写（4.1.6） | 1 |
| `W/pages/workspace/settings-layout.tsx`、`W/pages/notebook/settings-layout.tsx` | 改写：只剩 `useDocumentTitle`（标题照旧）与按页挂载的 `SettingsPage`（`h1` 文字照旧），`sections` 改用 `workspaceSections`、`notebookSections`（4.1.11） | 1 |
| `W/pages/settings/profile-page.tsx` | 改写（4.1.10） | 2 |
| `W/pages/settings/appearance-page.tsx`（新） | 4.1.10 | 2 |
| `W/pages/settings/security-page.tsx`、`deactivate-dialog.tsx` | 卡片；停用进"危险操作"（4.1.10）；`DeactivateDialog` 的按钮 `size="sm" shape="pill"` | 2 |
| `W/pages/settings/tokens-page.tsx`、`token-row.tsx`、`create-token-dialog.tsx` | 撤回之后焦点到 `h1`、去掉与页名同名的 `h2`（提交 1）；卡片，行与"已过期"的样子，"新建令牌"是胶囊，`create-token-dialog.tsx:184` 的 `space-y-2` 改 `space-y-1.5`（决定 830）（提交 2） | 1、2 |
| `W/styles.css` | `@layer components` 末尾加 `.nw-saved`（4.1.9） | 2 |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | 提交 1：`settings.back`、`settings.backTo`、`settings.groupPersonal`、`settings.groupWorkspace`、`settings.groupNotebook`；提交 2：`settings.appearance`、`settings.danger`（安全页就用），删 `preferences.title` | 1、2 |
| 测试 | 第 7 节：新的 `W/app/settings-origin.test.tsx`、`W/app/settings-directory.test.tsx`、`W/app/settings-page.test.tsx`（提交 1），`W/app/name-row.test.tsx`、`W/pages/settings/appearance-page.test.tsx`（提交 2）；改 12 个文件，另在 `W/pages/notebook/trash-preview.test.tsx` 加两格（决定 835）（7.1） | 1、2 |
| e2e | 第 5 节：`e2e/fixtures/settings-pages.ts` 加四个帮手；9 个故事（tb2 是照实加的）、a8、a15（提交 1、2 各改一部分，5.2）；新故事 `e2e/stories/shell/ui2-settings-mode.spec.ts`，`e2e/playwright.config.ts:3-4` 与 `tools/e2e-projects/check.mjs`（提交 1） | 1、2 |

#### 4.1.2 路由、`Layout` 与右上角

`routes.tsx` 顶上加（`useLocation` 从 `react-router` 引入）：

```tsx
/**
 * IndexTo is a settings index's redirect to its first page, the state it was given passed on, as LandingPage passes
 * its own: arrived at, the first page's h1 takes the focus (M13/P3/S1, decision 816).
 */
function IndexTo({ to }: { to: string }) {
  const { state } = useLocation();
  return <Navigate replace to={to} state={state} />;
}
```

三个 `index` 分别是 `<IndexTo to="/settings/profile" />`、`<IndexTo to="general" />`（两处）。`/settings` 的 `handle` 带注释"The settings' mode, its column the directory with the account in its foot (M13/P3/S1, decision 811)"。

`Layout`（`W/app/layout.tsx`）：

```tsx
export const Layout = observer(function Layout() {
  useArrivalWatch();
  // …column, given, giveBack as today…
  const { account } = useStore();
  const me = account?.me;
  // An account with a step of onboarding left is sent there (Onboarded) from every route with a column: through the
  // bounce the top right stays, its account's button holding the focus its menu gives back (decision 832).
  const bounced = me !== undefined && pendingSteps(me).length > 0;
  const routed = useMatches().some((match) => (match.handle as Shell | undefined)?.sidebar === true);
  const sidebar = routed && !bounced;
  const origin = useOriginRecord(useSettingsMode());
  // …the same JSX, the Outlet inside <SettingsOrigin value={origin}>…
});
```

`useMatches()` 照旧无条件调用（不能写在 `&&` 的右边）。`Layout` 的注释加一句：设置模式（`handle: { settings: true }`）由它记下来处（`useOriginRecord`），给目录的"返回"。`corner.tsx` 删去 `settings` 之后，`back` 只剩 `creating && …`。

#### 4.1.3 来处（`W/app/settings-origin.ts`）

```ts
export type Origin = { pathname: string; search: string; hash: string; workspace: string | undefined };
type SettingsHandle = { settings?: boolean };

export function useSettingsMode(): boolean {
  return useMatches().some((match) => (match.handle as SettingsHandle | undefined)?.settings === true);
}

type Seen = { location: Location; workspace: string | undefined; settings: boolean; origin: Origin | undefined };

export function useOriginRecord(settings: boolean): Origin | undefined {
  const location = useLocation();
  const type = useNavigationType();
  const { slug } = useParams();
  const [seen, setSeen] = useState<Seen>({ location, workspace: slug, settings, origin: undefined });
  if (seen.location !== location) {
    let origin = seen.origin;
    if (settings && !seen.settings) {
      const { pathname, search, hash } = seen.location;
      origin = type === "REPLACE" ? undefined : { pathname, search, hash, workspace: seen.workspace };
    }
    setSeen({ location, workspace: slug, settings, origin });
  }
  return seen.origin;
}

export const SettingsOrigin = createContext<Origin | undefined>(undefined);
export function useSettingsOrigin(): Origin | undefined {
  return use(SettingsOrigin);
}
```

（每项的注释照分离工作树里的写法：为什么在渲染时取、为什么不放路由状态、以 `replace` 进入为什么不记、离开设置为什么不清、刷新就没有。）React Router 的 `location` 每次导航是一个新对象，同一次导航的重渲染是同一个（`W/app/arrival.ts:56-62` 的注释），所以比较对象就是比较导航；`useNavigationType()` 是这一次导航的类型。渲染时 `setSeen` 之后 React 立即用新状态重渲染 `Layout`、再渲染它的子树，目录第一次渲染就拿到来处。`Layout` 是根路由的组件，`useParams()` 给的是整个匹配的参数，`slug` 就是这一页所在的工作区（工作区之外的页没有它）；记下的是上一次渲染时的 `slug`，即来处的工作区。`seen` 只在地址变了的那一次渲染里换，离开设置时来处留着、没有人读（目录只在设置模式里渲染），再进入时重取（审查 C 的 M1：初稿的"离开时清掉"没有可见的效果，去掉）。几种情形：

| 上一个地址 | 这一个地址 | 来处 |
|---|---|---|
| 非设置页 A（`push` 或后退、前进进入） | 设置页 | A（地址、查询、片段与它所在的工作区） |
| 非设置页 A | 设置页，以 `replace` 进入（守卫在登录之后去 `next`，`W/app/guards.tsx:28`；引导做完去 `next`，`W/pages/onboarding.tsx:25`） | 没有（决定 833） |
| 设置页 | 设置页（同一个或跨外框；`IndexTo` 的 `replace`；只换查询） | 不变 |
| 设置页 | 非设置页 | 留着，没人读；再进入设置时重取 |
| （标签页的第一个地址就是设置页） | — | 没有 |
| 非设置页 | 设置页，`Onboarded` 随即弹回引导 | 记下；做完引导以 `replace` 回到设置时没有（833） |

来处是进入这一次设置之前渲染的那一页：设置 → 别处 → 一次后退两条回到设置时，来处是刚离开的"别处"，不是第一次进入之前的那一页（审查 A 的 N2，探针 4；照这里写明，不另记）。

#### 4.1.4 设置目录（`W/app/settings-directory.tsx`）

**三组**：

```ts
type Section = { path: string; label: PlainKey; icon: LucideIcon; admins: boolean };
export const personalSections = [
  { path: "profile", label: "settings.profile", icon: UserRound, admins: false },
  { path: "appearance", label: "settings.appearance", icon: SunMoon, admins: false },
  { path: "security", label: "settings.security", icon: LockKeyhole, admins: false },
  { path: "tokens", label: "settings.tokens", icon: KeyRound, admins: false },
] as const satisfies readonly Section[];
export const workspaceSections = [ /* general SlidersHorizontal, members Users, ownerless Box (admins: true) */ ];
export const notebookSections = [ /* general SlidersHorizontal, members Users, transfer ArrowLeftRight */ ];
export function sectionAt<S extends Section>(sections: readonly S[], base: string, pathname: string): S | undefined {
  return sections.find(({ path }) => pathname === `${base}/${path}`);
}
```

`label` 是 `PlainKey`（`W/i18n/i18n.tsx:20`：不带占位符的键；写 `MessageKey` 时 `t(label)` 编不过，分离工作树里碰到过）。"外观与语言"一项随它的路由在 S1 的提交 2 加（决定 842）。图标同样稿（`user`、`sun`、`lock`、`key`、`sliders`、`users`、`box`、`swap`），lucide 1.48 都有。

**`useDirectoryWorkspace()`**（决定 818、837、843；初稿叫 `useLastWorkspace`）：

```ts
export function useDirectoryWorkspace(): Workspace | undefined {
  const workspaces = useWorkspaces();
  const { preferences } = useStore();
  const origin = useSettingsOrigin();
  const [last] = useState(() => preferences.lastWorkspace());
  const wanted = origin?.workspace ?? last;
  useSWR(wanted !== undefined && workspaces.list === undefined ? "workspaces" : null, () => workspaces.load());
  const from = origin?.workspace === undefined ? undefined : workspaces.bySlug(origin.workspace);
  return from ?? (last === undefined ? undefined : workspaces.bySlug(last));
}
```

只在个人设置的布局里用（工作区与笔记本的设置用外框的工作区）。没有来处、本设备也没记着工作区时不读列表（a15 的页就是这样，决定 843）；来处在一个工作区里时那个外框多半已读过列表，SWR 的同键不再读。

**`useBack(fallback)`**（模块内，决定 813、834）：

```ts
function useBack(fallback: Workspace | undefined): Back {
  const origin = useSettingsOrigin();
  const workspaces = useWorkspaces();
  const store = useStore();
  if (origin === undefined) {
    return fallback === undefined ? { to: "/", name: undefined } : { to: `/${fallback.slug}`, name: fallback.name };
  }
  if (origin.workspace === undefined) {
    // Out of the workspaces (the invitation's page, its token in the fragment; a new workspace's): all of it.
    return { to: `${origin.pathname}${origin.search}${origin.hash}`, name: undefined };
  }
  // In a workspace, no fragment: the page's h1 takes the arrival focus, its anchor's element the reading view's,
  // which of the two last depending on whether the view was cached (decision 834).
  const to = `${origin.pathname}${origin.search}`;
  const workspace = workspaces.bySlug(origin.workspace);
  if (workspace === undefined) {
    return workspaces.list === undefined ? { to, name: undefined } : { to: "/", name: undefined };
  }
  const [, , part, id] = origin.pathname.split("/");
  const notebooks = part === "notebooks" && id !== undefined ? store.notebooksOf(workspace) : undefined;
  if (id !== undefined && notebooks?.list !== undefined) {
    const notebook = notebooks.byId(id);
    return notebook === undefined ? { to: `/${workspace.slug}`, name: workspace.name } : { to, name: notebook.name };
  }
  return { to, name: workspace.name };
}
```

来处在不在工作区里看 `Origin.workspace`（来处那一页的路由参数），不看地址的第一段：工作区已不在列表里时（离开了、删了、来处就是"找不到工作区"），`bySlug` 也找不到，与保留的顶层名字（`create-workspace`、`invitations`）分不开，前者要回 `/`（审查 A 的 N3），后者要回来处。`notebooksOf` 在渲染里调用，同 `ShellPlace`（`workspace-layout.tsx:57-62`）；`SettingsTop` 与 `SettingsDirectory` 都是 `observer`，改名、列表变化即时跟上。

**`SettingsTop({ fallback })`**（`ShellSidebar` 的 `top`）：

```tsx
<>
  <Link to={to} state={arrived} className="flex h-7.5 items-center gap-2.25 rounded-row px-2 text-row text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground [&_svg]:size-4 [&_svg]:shrink-0">
    <ChevronLeft aria-hidden />
    <span className="min-w-0 truncate">
      {name === undefined ? t("settings.back") : <Isolated {...isolate((at) => t("settings.backTo", { name: at(name) }))} />}
    </span>
  </Link>
  <p className="px-2 pt-1.5 text-lg font-[650]">{t("settings.title")}</p>
</>
```

"返回"一行与样稿 `.row.back` 逐像素相同（审查 B 实量）。样稿的 `.side` 是 flex 列、`gap: 2px`，`.row.back` 的 `margin-bottom: 6px` 与 `.stitle` 的 `padding-top: 8px` 不折叠，"返回"到"设置"的字是 16 px；这里是顶部的 `space-y-2`（8 px）加 `pt-1.5`（6 px），14 px。审查 B 量到的差（第一个组标题低 3 px、第三组高 5 px、组标题与标题之间多 4 px）都在 5 px 之内，照这些类名（初稿说"折叠"不对，审查 B 的 N1）。`.stitle` 18 px、650 是 `text-lg font-[650]`。"返回"不是 `NavItem`：它不标当前，不要 `aria-current`。

**`SettingsDirectory({ here, workspace })`**（中段）：

```tsx
const follow = useFollow();
const id = useMatch("/:slug/notebooks/:id/settings/*")?.params.id;
const notebook = workspace === undefined || id === undefined ? undefined : store.notebooksOf(workspace)?.byId(id);
const item = (to: string, same: boolean, { path, label, icon: Icon }: Section) => (
  <NavItem key={path} to={to} {...(same ? { onClick: follow(to) } : { state: arrived })}>
    <Icon aria-hidden />
    {t(label)}
  </NavItem>
);
return (
  <>
    <nav aria-label={t("settings.title")} className="flex flex-col gap-0.5">
      <GroupTitle title={t("settings.groupPersonal")} />
      {personalSections.map((section) => item(`/settings/${section.path}`, here === "personal", section))}
    </nav>
    {workspace !== undefined && (
      <nav aria-label={t("workspaceSettings.title")} className="flex flex-col gap-0.5">
        <GroupTitle title={t("settings.groupWorkspace")} name={workspace.name} />
        {workspaceSections
          .filter((section) => !section.admins || workspace.role === "admin")
          .map((section) => item(`/${workspace.slug}/settings/${section.path}`, here === "workspace", section))}
      </nav>
    )}
    {workspace !== undefined && notebook !== undefined && (
      <nav aria-label={t("notebookSettings.title")} className="flex flex-col gap-0.5">
        <GroupTitle title={t("settings.groupNotebook")} name={notebook.name} />
        {notebookSections.map((section) =>
          item(`/${workspace.slug}/notebooks/${notebook.id}/settings/${section.path}`, here === "workspace", section)
        )}
      </nav>
    )}
  </>
);
```

`GroupTitle`：

```tsx
<p className="flex min-w-0 gap-1.5 px-2 pt-3.5 pb-1 text-xs text-faint">
  <span className="shrink-0">{title}</span>
  {name !== undefined && (
    <>
      {" "}
      <bdi className="min-w-0 truncate font-medium text-muted-foreground">{name}</bdi>
    </>
  )}
</p>
```

标题与名字之间的空格写在 `bdi` 之外（05-lessons 第 6 节）：`textContent` 是"工作区 Lab"；flex 里只有空白的匿名项不画，看得见的空隙照旧是 `gap`（审查 A 的 N1）。标题包在 `shrink-0` 的 `span` 里：裸文字是会收缩的匿名 flex 项，中文的最小内容只有一个字宽，名字一长"笔记本"就被挤成竖排（审查 B 的 I4；修订的探针：21 个汉字的名字时标题一行、高 16）。样稿 `.group` 的 12 px、`--text-3`、`padding: 14px 8px 4px`、`gap: 6px`，名字 `--text-2`、500、省略号；`--text-3` 是 `--faint`，`--text-2` 是 `--muted-foreground`。行是 `NavItem` 本身（30 px，`text-row`，当前一行 `bg-active`，另有强制色下才看得见的透明边框，决定 845），不另写。笔记本组只在地址是那本笔记本的设置、它在工作区的列表里时出现（找不到的笔记本的设置页是 404，目录照旧有个人与工作区两组）。

**地标与名称**：左栏的根照旧是 `data-shell`、不是地标（`ShellSidebar` 的注释），里面三个 `nav`："设置""工作区设置""笔记本设置"；没有搜索的 `role=search`；页的 `main` 照旧。窄屏抽屉开着时根是名为"导航"的 `aria-modal` 对话框（同工作区的）。

#### 4.1.5 工作区外框的设置模式（`W/pages/workspace/workspace-layout.tsx`）

```tsx
const settings = useSettingsMode();
// Where the workspace's column was scrolled, through the settings (ScrollKept).
const scrolled = useRef(0);
// Not useParams: the column is the workspace's, above the routes that name a notebook.
const notebookId = useMatch("/:slug/notebooks/:id/*")?.params.id;
// One column whatever the mode: the drawer, its menu and its strip stay the same elements as the settings come and
// go (decision 810). In the settings' mode, their directory, the notebook's row, tree and places held out of sight
// (decisions 814, 836).
const left = settings ? (
  <ShellSidebar drawer={drawer} place={t("settings.title")} top={<SettingsTop fallback={workspace} />}>
    <SettingsDirectory here="workspace" workspace={workspace} />
    {notebookId !== undefined && (
      <div hidden>
        <SlotHolder id={notebookId} />
      </div>
    )}
    <ScrollKept at={scrolled} settings />
  </ShellSidebar>
) : (
  /* today's ShellSidebar: switcher, search box, the workspace's nav with Home, Settings (leaves), NotebookNav, then */
  /* <ScrollKept at={scrolled} settings={false} /> last in its middle */
);
```

两个分支都是同一位置上的 `ShellSidebar`，React 当作同一个组件实例更新它：抽屉的状态（`useDrawer` 在外框里）、菜单按钮（`drawer.menu`）、`FocusScope` 的根都不重建。进入设置时搜索框卸下，`field.current` 变回 `null`（决定 819）。`ShellPlace` 只在工作区模式里用。读取中、读取失败与工作区找不到时照旧（`:125-144`，没有左栏，交还右上角）：设置模式不改这几支。

`useMatch` 用的是带 `*` 的模式：在 `/:slug/notebooks/:id/settings/…` 上取到 id（与 `NotebookLayout` 的 `useParams` 写法相同的 id，容器按地址里的 id 给，`notebook-layout.tsx:57-58`）。`SlotHolder` 在 `hidden` 里收养容器之后，`NotebookLayout` 照旧把树门户进去。

**左栏的滚动**（审查 A 的 M3）：设置模式与工作区模式共用 `ShellSidebar` 的同一个 `[data-sidebar-scroll]`，内容换成短的目录时浏览器把 `scrollTop` 夹到新的最大值；`hidden` 里的 `SlotHolder` 还焦点不成时初稿照样调 `intoColumn`，齿轮的矩形全是 0，`scrollTop` 又减一截（审查 A 的探针 3：三种引擎 300 → 202；初稿说"不滚任何东西"不对）。改成两处：

- `SlotHolder` 只在 `document.activeElement === focused`（焦点真的还上了）时调 `intoColumn`（`notebook-slot.tsx:127-130`）。
- `ScrollKept`（模块内）记下工作区左栏滚到哪里，进出设置时还回去：

  ```tsx
  function ScrollKept({ at, settings }: { at: RefObject<number>; settings: boolean }) {
    const marker = useRef<HTMLSpanElement>(null);
    useLayoutEffect(() => {
      const area = marker.current?.closest<HTMLElement>("[data-sidebar-scroll]");
      if (area === null || area === undefined) {
        return undefined;
      }
      if (settings) {
        area.scrollTop = 0;
        return undefined;
      }
      area.scrollTop = at.current;
      const keep = () => {
        at.current = area.scrollTop;
      };
      area.addEventListener("scroll", keep, { passive: true });
      return () => area.removeEventListener("scroll", keep);
    }, [at, settings]);
    return <span ref={marker} hidden />;
  }
  ```

  工作区模式里它是中段的最后一个子元素：layout effect 子先于父、兄弟按次序，所以它在列表项的 `SlotHolder` 把容器收养回来之后（树的行已在，`scrollTop` 不被夹）、`PageTree` 把当前页的行带进视野之前（`page-tree.tsx:255-263`，在 `Outlet` 之下，左栏的门户在 `Outlet` 之前）还原；之后的滚动由 `scroll` 事件记下（事件是异步的，离开工作区模式的那次提交里监听已拆，被夹的那一下记不到）。设置模式里它把目录放回顶上。修订的探针（jsdom 里给 `scrollTop` 与 `getBoundingClientRect` 打桩）：滚到 300、点齿轮，设置里是 0；"返回"之后先还成 300，树再把当前行带进来（`[300, 738]`）。`SlotHolder` 的那一句在 `ScrollKept` 之后的同一次提交里没有可见的效果（`ScrollKept` 随后置 0），照主持者的处置留着：它不该依赖兄弟的次序（变异表里注为等价，7.2）。

#### 4.1.6 个人设置的外框（`W/pages/settings/settings-layout.tsx`）

```tsx
export const SettingsLayout = observer(function SettingsLayout() {
  const t = useT();
  const { pathname } = useLocation();
  const shown = sectionAt(personalSections, "/settings", pathname);
  useDocumentTitle(shown && t(shown.label), t("settings.title"));
  const column = useShellColumn();
  const drawer = useDrawer();
  const workspace = useDirectoryWorkspace();
  const left = (
    <ShellSidebar drawer={drawer} place={t("settings.title")} top={<SettingsTop fallback={workspace} />}>
      <SettingsDirectory here="personal" workspace={workspace} />
    </ShellSidebar>
  );
  return (
    <Drawer value={drawer.actions}>
      {column !== null && createPortal(left, column)}
      {/* The index's redirect has no page of its own: its Outlet alone, for a commit. */}
      {shown === undefined ? <Outlet /> : (
        <SettingsPage key={shown.path} title={t(shown.label)}>
          <Outlet />
        </SettingsPage>
      )}
    </Drawer>
  );
});
```

`useDrawer` 在这个外框里，同 `WorkspaceLayout`：它的 effect 在页的 effect 之后跑（02-P2 决定 790 要它是页的祖先）。个人设置没有 `useSearchShortcuts`。文档标题照旧（13.2 第 22 条）。`shown === undefined` 只有 `/settings` 本身（`index` 重定向的那一次渲染）。

#### 4.1.7 卡片组件（`W/app/settings-page.tsx`）

S1 的提交 1 只有 `SettingsPage` 与 `useSettingsHeading`，其余在提交 2（决定 842）。

- **`SettingsPage({ title, children })`**：`<div className="mx-auto w-full max-w-178 md:px-2 md:pt-5 md:pb-4">`，里面 `<h1 ref={heading} tabIndex={-1} className="mb-7 text-[26px] leading-normal font-[650] tracking-[-0.01em] break-words outline-none">`，再是 `<SettingsHeading value={heading}><div className="space-y-6.5">{children}</div></SettingsHeading>`。`heading` 是 `useArrivalFocus<HTMLHeadingElement>()`。样稿 `.page` 是 `max-width: 760px; padding: 44px 32px 40px`（`box-sizing: border-box`，内容 696 px），`main` 照旧 `p-6`（P4 再定），所以宽屏上加 `pt-5`（24 + 20 = 44）、`px-2`（24 + 8 = 32）、`pb-4`（24 + 16 = 40），窄屏上只有 `main` 的 24 px；最宽 `max-w-178`（712 = 696 + 两边的 8 px，决定 844；初稿的 `max-w-190` 让内容 744 宽，审查 B 的 M1）；`h1` 26 px、650、`-0.01em`、行高 1.5（39 px，同样稿，审查 B 的 N2）、下边 28 px（`mb-7`）。
- **`useSettingsHeading()`**：`SettingsHeading` 上下文里的那个 ref；不在 `SettingsPage` 里时抛错。
- **`SettingsSection({ title?, heading?, children })`**：`<section className="space-y-2.5">`，有标题时先是 `<h2 ref={heading} tabIndex={heading === undefined ? undefined : -1} className="px-0.5 text-compact font-semibold text-muted-foreground outline-none">`。样稿 `.sec` 是 13 px、600、`--text-2`、`margin: 26px 2px 10px`：组间 26 px 是 `SettingsPage` 的 `space-y-6.5`，标题与卡片之间 10 px 是 `space-y-2.5`。
- **`SettingsCard({ list?, children })`**：`div`（或 `list` 时 `ul`），`divide-y divide-border rounded-card border border-border bg-surface`（14 px 圆角，行间 1 px，没有阴影，01-P1 第 9 节"P3"）。
- **读取中、失败与空**（审查 B 的 N4，各页一样）：读取中（`NotLoaded`、`Loading`）与读取失败（`Alert` 加"重试"）放在卡片的位置上，不进卡片；读到了、列表是空的时，是一张卡片里的一行文字（`<p className="px-4.5 py-3.5 text-sm text-muted-foreground">`，如"No tokens yet."）。
- **一行**（`export const`，`NameRow`、开放程度、`SettingsRow` 都用）：`rowClass = "flex min-h-16 flex-wrap items-center gap-x-6 gap-y-2.5 px-4.5 py-3.5"`、`rowTextClass = "min-w-0 flex-1 basis-56"`、`rowTitleClass = "text-sm font-medium"`、`rowDescriptionClass = "mt-0.5 max-w-105 text-[12.5px] leading-normal text-faint"`、`rowControlClass = "flex min-w-0 flex-wrap items-center gap-2 max-md:w-full md:ml-auto md:shrink-0"`、`rowValueClass = "min-w-0 text-sm break-words text-muted-foreground max-md:w-full md:ml-auto md:max-w-[60%] md:text-right"`、`rowProblemClass = "w-0 min-w-full basis-full text-compact text-destructive md:-me-2"`（宽屏上它在控件区里自成一行，宽是 `min-w-full`；它的宽算作 0，它前面的 `gap-2` 照算，`md:-me-2` 把这个间距抵掉，控件区不因它变宽、整体不左移，审查 B 的 R1-M1）。跟在"已保存。"之后的问题（今天只有 `NameRow` 的）在调用处另加 `md:-me-21 md:ps-19`：`md:ps-19` 是"已保存。"预留的 68 px 加 8 px 间距，让它的字与输入框左对齐（决定 848）；它的宽因此算作内边距的 76 px，`md:-me-21` 的 84 px 抵掉内边距与它前面的间距，tailwind-merge 让它盖过常量里的 `md:-me-2`。缩进不进通用的常量：没有"已保存。"的一行用了它，字会比输入框靠右 76 px（审查 B 的 R3-N2）。样稿 `.set` 是 `gap: 24px; padding: 14px 18px; min-height: 64px`，`.t` 14 px、500，`.d` 12.5 px、`--text-3`、`max-width: 420px`，`.c` 靠右、`gap: 8px`；`flex-wrap` 与 `basis-56` 让窄屏上控件落到文字之下（探针 (c)）。控件区里的东西不收缩（`md:shrink-0`）；只显示一个值的一行不用控件区，用 `rowValueClass`：最多占这一行的 60%、可以收缩、右对齐、长了折行（审查 B 的 M4：70 个汉字的工作区名在控件区里溢出卡片 269 px、页面横向滚动；修订的探针里 1440、900、375 宽都不溢出）。`text-right` 在 `design-tokens.test.ts` 的 `noColour` 里，`text-end` 不在（它会被当成颜色），所以写 `md:text-right`。
- **`SettingsRow({ title?, description?, value?, children? })`**：只显示或只有按钮的一行：标题是 `p`（`id` 是 `${useId()}-title`），说明是 `p`（`-description`）；`value` 是只读的值，放进 `<p className={rowValueClass}>`；`children` 是一个函数，收到这两个 id（`role="radiogroup"` 的 `aria-labelledby`、`aria-describedby` 用），控件区是 `rowControlClass`。没有标题的一行（导入、导出，组标题就是它的名字）只有说明，说明用 `text-sm text-muted-foreground`。有输入框的一行自己用 `label`（`NameRow`、开放程度）。
- **`SavedOutput({ saved, label })`**：

  ```tsx
  export function SavedOutput({ saved, label }: { saved: boolean; label: string }) {
    const [round, setRound] = useState(0);
    const [was, setWas] = useState(saved);
    if (was !== saved) {
      setWas(saved);
      if (saved) {
        setRound((n) => 1 - n);
      }
    }
    return (
      <output
        data-round={round}
        className="nw-saved inline-flex items-center gap-1 text-xs whitespace-nowrap text-faint max-md:order-last max-md:empty:-me-2 md:min-w-17 md:justify-end"
      >
        {saved ? label : ""}
      </output>
    );
  }
  ```

  样稿 `.saved` 是 12 px、`--text-3`、`gap: 4px`。它是控件区的第一个子元素：控件区靠右，它右边的下拉或输入框不动（同样稿 `settings.html:200`，审查 B 的 I2）。宽屏上它不论空不空都占 68 px（`md:min-w-17`，中文"已保存。"连 ✓ 实量 65 px），字靠右（`md:justify-end`），离控件 8 px。不预留时，它一出现控件区就宽出约 61–73 px：768–1010 宽的开放程度一行、910–990 宽的名称一行，控件区整个折到文字之下，卡片变高；文字淡出之后也留着，直到下一次发出清空才缩回（审查 B 的 R2-M1，决定 848）。375 px 时控件区靠左，它以 `max-md:order-last` 排到最后，同样不推动控件；空的时候 `max-md:empty:-me-2` 抵掉它后面的 8 px 间距，不多出缩进与空行。同一张卡片里各行的控件右边对齐（修订的探针：下拉、"保存"、危险按钮、列表行的按钮右边都离卡片 19 px）。代价（决定 848）：宽屏上控件区一直宽出 76 px。名称一行的说明列从 326 窄到 250 px，1024–1440 宽时行高不变（92）；开放程度一行的说明列从 420 窄到 408 px，英文说明在宽屏上是两行（行高 92，原来 73），中文照旧一行；768–912 宽时开放程度的下拉一直在说明之下（原来没有"已保存。"时在右边）。这些都是常态，不再随"已保存。"出现、消失而变。修订核对第 2 轮之后的探针（第 1 节末）：768–1200 每 2 px，中英文、三种引擎都不再跳。`data-round` 每次写出时换（决定 841，4.1.9）。
- **列表的一行**（令牌、成员、邀请、无主笔记本、任务，S1 起令牌）分两种（审查 B 的 I1）：
  - **简单的行**（令牌、成员、无主笔记本）：`li` 是 `flex min-h-14 flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4.5 py-3`，左边的文字块照旧（`min-w-0`），控件在右；
  - **叠的行**（邀请、任务：主行之下还有复制的链接、拒绝的原因或展开的报告）：`li` 只是 `space-y-3 px-4.5 py-3`，里面的主行照旧是 `flex flex-wrap items-center justify-between gap-x-4 gap-y-2`，叠在下面的照旧在它之下。

  行内的按钮 `size="sm" shape="pill" variant="outline"`，移出、撤回、删除这些不可逆的也是（确认框照旧），红只在"危险操作"组（决定 838）；"已过期""你"之类的标记 `rounded-full border border-border bg-surface-2 px-1.5 text-[11px] font-normal text-muted-foreground`（样稿的 `.badge`；S2 把成员行的"你"（`W/app/member-summary.tsx:34`，`rounded border text-xs`）也换成它，审查 B 的 N4）。

#### 4.1.8 文字输入的一行（`W/app/name-row.tsx`）

```tsx
export function NameRow<Field extends string>({ current, field, label, description, autoComplete, check, fieldTexts,
  save, explain, saveLabel, savedLabel }: NameRowProps<Field>) {
  const id = useId();
  const [draft, setDraft] = useState<string>();
  const name = draft ?? current;
  const [saved, setSaved] = useState(false);
  const { ref, sending, banner, problemOf, submit } = useForm([field], { fieldTexts, explain });
  const edits = useRef(0);
  const input = useRef<HTMLInputElement>(null);
  const changed = draft !== undefined && draft !== current;
  // A problem and a refusal say what was wrong with a draft: with the name typed back as it is, gone too.
  const problem = changed ? problemOf(field) : undefined;
  const described =
    problem !== undefined ? `${id}-problem` : description !== undefined ? `${id}-description` : undefined;

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || !changed) {
      return;
    }
    const found = check(name);
    const trimmed = name.trim();
    const edit = edits.current;
    const problems: LocalProblems<Field> = {};
    if (found !== undefined) {
      problems[field] = found;
    }
    setSaved(false);
    void submit(problems, async () => {
      if (trimmed !== current) {
        await save(trimmed);
      }
      if (edits.current === edit) {
        setDraft(undefined);
        setSaved(true);
      }
    });
  }
  return (
    <form ref={ref} noValidate onSubmit={onSubmit}>
      <div className={rowClass}>
        <div className={rowTextClass}>
          <label htmlFor={id} className={rowTitleClass}>{label}</label>
          {description !== undefined && <p id={`${id}-description`} className={rowDescriptionClass}>{description}</p>}
        </div>
        <div className={rowControlClass}>
          <SavedOutput saved={saved} label={savedLabel} />
          <Input ref={input} id={id} name={field} autoComplete={autoComplete} value={name}
            aria-invalid={problem !== undefined || undefined} aria-describedby={described}
            className="h-8 md:w-60 md:text-row"
            onChange={(event) => { edits.current++; setDraft(event.target.value); setSaved(false); }} />
          {/* Save's place, kept: the field does not move as it comes and goes. */}
          <div className="flex max-md:order-2 max-md:empty:hidden md:min-w-15 md:justify-end">
            {changed && <SaveButton label={saveLabel} busy={sending} onLeave={() => input.current?.focus()} />}
          </div>
          {problem !== undefined && <p id={`${id}-problem`} className={cn(rowProblemClass, "max-md:order-1 md:-me-21 md:ps-19")}>{problem}</p>}
        </div>
      </div>
      {changed && banner !== undefined && <div className="px-4.5 pb-3.5"><Alert>{banner}</Alert></div>}
    </form>
  );
}

function SaveButton({ label, busy, onLeave }: { label: string; busy: boolean; onLeave: () => void }) {
  const button = useRef<HTMLButtonElement>(null);
  useFocusLeaving(button, onLeave);
  return (
    <Button ref={button} type="submit" size="sm" shape="pill"
      aria-disabled={busy || undefined} aria-busy={busy || undefined}>{label}</Button>
  );
}
```

- **`SaveButton` 自成一个组件**：`useFocusLeaving` 在挂载时的 layout effect 里取元素（`W/lib/focus-leaving.ts:15-22`）；写在 `NameRow` 里时它在 `NameRow` 挂载时取到的是 `null`（那时没有"保存"），焦点就落到 `body`。它照旧随 `changed` 挂载、卸下，挂在一个一直在的槽里。
- **样子**：控件区里依次是"已保存。"、输入框、"保存"的槽、问题。输入框 32 px（`h-8`；样稿 `.input` 32 px、240 宽、9 px 圆角、`--surface-2`、13.5 px：`Input` 本身就是 `rounded-control bg-surface-2`，宽屏 `w-60`、`text-row`，窄屏照旧 `w-full`、`text-base`，iOS 不放大）；"保存"是 30 px 的主按钮（决定 820），英文 54 px 宽。槽在宽屏上至少 60 px（`md:min-w-15`）、"保存"靠右，所以输入框不随"保存"出现、消失而左右跳（审查 B 的 M6：初稿第一次按键时左跳 60 px）；"保存"的右边与同一张卡片里别的控件对齐（离卡片 19 px）；没编辑时输入框右边空着 68 px，这是不跳的代价（样稿画的是编辑中的样子）。出了问题时也不跳：问题自成一行，调用处的 `md:-me-21 md:ps-19` 抵掉它占的宽与它前面的间距，字与输入框左对齐（4.1.7）。宽屏上"已保存。"一直占 68 px（决定 848），名称一行的说明列因此是 250 px（原来 326）。375 px 时控件区靠左、输入框占满一行，以 `order` 排成输入框、问题、"保存"、"已保存。"（初稿的问题落在"保存"之下，审查 B 的 M6）；空的槽不占位（`max-md:empty:hidden`：不然它与 `SavedOutput` 落到第二行，行间距照算，这一行底下多 8 px、"已保存。"缩进 8 px，审查 B 的 R1-N1）。修订的探针：三种引擎，同一宽度下没编辑、编辑中、已保存、出了问题四态的输入框位置相同（1440 宽都在 867–1107，900 宽都在 541–781，审查 B 的 R2-N1），"保存"离卡片右边 19 px（出了问题时初稿是 27 px），问题的字从输入框的左边起；375 宽时依次是输入框、问题、"保存"，没编辑时控件区与输入框同高，"已保存。"与输入框左对齐（审查 B 的 `rows.mjs`，第 1 节末）。
- **规则**（决定 822）：没编辑、或编辑回与当前相同时没有"保存"，回车什么也不发；只差首尾空白时有"保存"，点了不发请求（`trimmed === current`）、显示"已保存。"、字段回到当前的名字（同 `RenameForm`）；发出之后又编辑的保留草稿、不显示"已保存。"；本地检查或服务端的字段问题显示在输入框下（`useForm` 失败之后焦点到 `aria-invalid` 的字段，`W/components/form-field.tsx:92-111`）；别的拒绝（403、网络）在这一行下面的横幅里，焦点没丢时留在原处。问题、`aria-invalid` 与横幅只在 `changed` 时显示：改回原名之后 `onSubmit` 早退、"保存"也不在，`useForm` 只在下一次 `submit` 时清它们，初稿里它们就一直留着（审查 A 的 M1）；改回原名再改成别的时，上一次的问题照旧显示，直到下一次保存（同今天的表单）。
- **用处**：S1 个人资料的显示名（`field="display_name"`，`check` 只查空，同 `DisplayNameForm`，`save` 是 `account.update`）；S2 工作区与笔记本的名称（`field="name"`，`check`、`fieldTexts`、`explain` 照今天的 `RenameForm` 用法：`workspace/general-page.tsx`、`notebook/general-page.tsx:44-54`）。

#### 4.1.9 "已保存。"的 ✓ 与淡出（`W/styles.css`）

`@layer components` 的末尾（`.nw-search` 之后）：

```css
  /* "Saved."'s check, drawn before its text, never in it: the output's text is what a screen reader and the tests read
     (M13/P3/S1, decisions 823, 841). Amber marks the state, a 13px mask over the text's colour, as the search box's
     clear button is drawn; under forced colours the system's text colour, which the mask is drawn in still. 1.5s
     after the text shows the output fades out, the text kept. Each time the text is written its round turns
     (SavedOutput), which names the other of two like keyframes: the fade starts again however soon after the
     output was emptied (emptied and written between two style computations, Chromium and Firefox go on with the
     animation as it was). */
  .nw-saved:not(:empty) {
    animation: nw-saved-out 300ms ease-in 1.5s forwards;
  }
  .nw-saved[data-round="1"]:not(:empty) {
    animation-name: nw-saved-out-again;
  }
  .nw-saved:not(:empty)::before {
    content: "";
    flex: none;
    width: 13px;
    height: 13px;
    color: var(--accent);
    background-color: currentColor;
    mask: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%23000' stroke-width='2.5' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m5 12 5 5 9-10'/%3E%3C/svg%3E")
      center / contain no-repeat;
  }
  @media (forced-colors: active) {
    .nw-saved:not(:empty)::before {
      forced-color-adjust: none;
      color: CanvasText;
    }
  }
  @keyframes nw-saved-out {
    to {
      opacity: 0;
    }
  }
  @keyframes nw-saved-out-again {
    to {
      opacity: 0;
    }
  }
```

✓ 的路径同样稿 `.saved .i`（`settings.html:114` 的 `m5 12 5 5 9-10`、`stroke-width: 2.5`；初稿用的是 lucide 的 `check`，审查 B 的 N3）。`%23000` 是 URL 编码的，`design-tokens.test.ts` 的 `colourValue` 不认它（探针 (b)）。React 把空串渲染成没有文本节点，`:empty` 成立。

**为什么交替**（决定 841）：CSS 动画只在一次样式计算看到 `:empty` 时撤掉，再写字才从头来；选择是离散事件，React 当场提交空文字，答复在往返之后写回。两者落在同一帧的两次样式计算之间时，浏览器从没算过空的那一态，动画接着上一次 `forwards` 的结尾，透明度停在 0：第二次改开放程度时看不到"✓ 已保存。"。审查 B（`fade2.mjs`，各 10 次）：Chromium 隔 1 ms 0/10、4 ms 5/10、30 ms 10/10，Firefox 1–4 ms 0/10，WebKit 总重来；审查 C（`fade-probe.mjs`，各 20 次）同样，并试了按轮次换动画名：三种引擎、各种间隔都是 20/20。修订的探针照 `SavedOutput` 的换法（每次写出时换）在工作树编出的 CSS 上再跑：清空之后同一个任务里、隔 0、2、5、10、20 ms 写回，三种引擎 20/20 重来，2.2 秒时透明度 0。另一种做法（审查 B 的 `flush.mjs`：清空时读一次计算样式，强制算到空的一态）也成立，但它依赖读的时机（一个 layout effect），换名字与时机无关。

#### 4.1.10 个人设置的四页

四页的卡片都在 S1 的提交 2；提交 1 只把令牌页撤回之后的焦点换到 `h1`、去掉与页名同名的 `h2`"访问令牌"，别的照旧（决定 842；留着它时 `tokens-page.test.tsx:259` 不分级找"Access tokens"，撞上两个标题，审查 A 的 R1-M1、C 的 R1-M1）。

- **个人资料**（`W/pages/settings/profile-page.tsx`，`observer`）：一个 `SettingsSection`（无标题）、一张卡片两行：`NameRow`（显示名，说明"别人看到的名字。"）；`SettingsRow`（标题"E-mail address"（`form.email`），说明"只有服务器管理员能修改邮箱。"，`value={me.email}`：只读的值，`rowValueClass`，长的邮箱折行、不撑破卡片，审查 B 的 M4）。不再有 `h2`"个人资料"（页的 `h1` 就是它）与偏好一节。
- **外观与语言**（新，`W/pages/settings/appearance-page.tsx`，`observer`）：一张卡片两行（决定 824）：`SettingsRow`（`theme.label`"Theme"，说明 `preferences.hint`），控件是 `<div role="radiogroup" aria-labelledby={title} aria-describedby={description} className="flex flex-wrap items-center gap-x-4 gap-y-2">`，三个 `<label className="flex items-center gap-2 text-sm"><input type="radio" name="theme" …/>…</label>`，`onChange` 同今天（`profile-page.tsx:57-71`）；语言一行同样，`label` 带 `lang`（`:72-86`）。单选是原生的（`accent-color` 琥珀，`W/styles.css:169-172`），强制色下照旧可见。
- **安全**（`W/pages/settings/security-page.tsx`）：`SettingsSection title="Password"`（`security.passwordTitle`），卡片里是 `<div className="px-4.5 py-4">` 包着今天的 `ChangePasswordForm`（`max-w-md`，字段竖排，按钮照旧 36 px，决定 821）；`SettingsSection title="Danger zone"`（`settings.danger`），卡片一行：`SettingsRow`（标题"Deactivate account"（`deactivate.title`），说明 `deactivate.body`），控件是 `DeactivateDialog`（按钮 `variant="destructive" size="sm" shape="pill"`，文字照旧"Deactivate account"，a11:93、w10:124 以它找）。`h2`"Password"留着（a15 的 :356 等七处读它）。
- **访问令牌**（`W/pages/settings/tokens-page.tsx`、`token-row.tsx`）：`SettingsSection`（无标题）里先是一行说明与"新建令牌"：`<div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 px-0.5">`（`tokens.body` 的 `p` 是 `max-w-xl text-sm text-muted-foreground`；`CreateTokenDialog` 的触发按钮换成 `size="sm" shape="pill"`，同工具栏里的胶囊，审查 B 的 N6；对话框里的表单照旧）；再是列表（读取中、失败与空照 4.1.7）：有令牌时 `<SettingsCard list>`，每行是 4.1.7 的简单的行（名字、"已过期"标记、创建与到期、最近使用，"撤回"是 `outline` 的 30 px 胶囊，`aria-label` 照旧）。撤回之后焦点到页的 `h1`（`useSettingsHeading()` 代替今天的 `h2` ref，`tokens-page.tsx:19-21`；今天的 `h2`"访问令牌"在提交 1 就去掉，`tokens-page.test.tsx:259` 以名字"Access tokens"找标题、不分级，找到的是 `h1`，照旧过：审查 C 在提交 1 的样子上跑过这个文件的 13 格）。

#### 4.1.11 工作区与笔记本设置的布局（S1 的过渡）

```tsx
export const WorkspaceSettingsLayout = observer(function WorkspaceSettingsLayout() {
  const { slug, name } = useWorkspace();
  const t = useT();
  const { pathname } = useLocation();
  const shown = sectionAt(workspaceSections, `/${slug}/settings`, pathname);
  useDocumentTitle(shown && t(shown.label), t("workspaceSettings.title"), name);
  return shown === undefined ? (
    <Outlet />
  ) : (
    <SettingsPage key={shown.path} title={t("workspaceSettings.title")}>
      <Outlet />
    </SettingsPage>
  );
});
```

笔记本的同样，`title` 是 `t("notebookSettings.heading", { name: notebook.name })`。内容区的 `nav` 去掉（目录里有同名的，两个同名会让 `getByRole("navigation", { name })` 撞）。页本身（`general-page.tsx` 等）S1 不动：它们的 `h2` 照旧，`h1` 之下是今天的样子（决定 831）。S2 把 `title` 换成 `t(shown.label)`。

#### 4.1.12 进出与焦点的走查

| 从 | 经 | 到 | 路由状态 | 焦点 |
|---|---|---|---|---|
| 工作区的任何页 | 导航的"设置" | `/:slug/settings/general` | `arrived` | "工作区设置"（S2："常规"）`h1` |
| 笔记本的任何页 | 笔记本行的齿轮 | `…/notebooks/:id/settings/general` | `arrived` | "X 设置"（S2："常规"）`h1` |
| 任何有左栏的页、设置页 | 账户菜单的"个人设置" | `/settings/profile` | `arrived` | "个人资料" `h1` |
| 个人资料页 | 账户菜单的"个人设置" | 同一个地址（`Link` 自动 `replace`） | 没有（`atProfile`，决定 816） | 宽屏：菜单把焦点还给按钮（`user-menu.tsx` 的 `leaving`）；375 px：抽屉随新的 `key` 关上，不是到达，焦点给细栏的菜单按钮（初稿带 `arrived` 时：个人资料页本身是到达进来的，`h1` 不重挂、`useArrivalFocus` 的依赖不变，菜单按钮也不取，落到 `body`；直接打开的，`h1` 取走；审查 A 的 M2、R1-M2）。jsdom 没有 `inert`，Radix 在 `setTimeout(0)` 里把焦点还给关上的抽屉里的账户按钮，vitest 那一格因此模拟 `inert`（7.1），浏览器里由 UI2 第 3 格核对 |
| 设置页 | 目录里同外框的链接 | 同外框的另一页 | `opened`（抽屉开着时 `arrived`；当前地址 `replace`） | 留在链接上（抽屉里：去向的 `h1`） |
| 设置页 | 目录里跨外框的链接 | 另一个外框的设置页 | `arrived` | 去向的 `h1` |
| 设置页 | "返回" | 来处（或 4.1.4 的去处） | `arrived` | 去向页的 `h1`：来处能是任何有左栏或右上角的页，用 `useArrivalFocus` 的有工作区首页、笔记本首页、页面、版本、目录、健康、历史、标签、回收站、升级、搜索、新建工作区与 404，变更集页、回收站的预览与邀请页在 S1 加上（决定 835；初稿只看了前几种，审查 A 的 I2） |
| 工作区或笔记本设置 | 删除、离开（S2 照旧） | 工作区首页或 `/` | `arrived`（今天的） | 去向的 `h1` |
| 直接打开 `/:slug/settings` | `IndexTo` | `…/general` | 收到的（没有） | 不取 |
| 引导中的账户 | 右上角菜单的"个人设置" | 弹回引导 | — | 菜单还给右上角的按钮（决定 832） |

窄屏：进入设置的三个链接都在抽屉里时，导航让抽屉关上（`useDrawer` 的"离开"），`headingTakes(arrived)` 为真，菜单按钮不取焦点，去向的 `h1` 取；设置页里打开抽屉，焦点到第一个可聚焦的元素，即"返回"（`useDrawer` 的 `focusable`，`shell-sidebar.tsx:112`）；目录的链接在抽屉里时带 `arrived`（`useFollow` 在点按时读 `inDrawer()`）。`hidden` 里的树在 DOM 顺序上在目录之后，不会是第一个可聚焦的元素。

**照实**（S1 的实施与审查，[S1 的审查记录](reviews/P3-S1-settings-mode-review.md)）：

- **4.1.1**：
  - e2e 一行照实加 tb2（`e2e/stories/trash/tb2-background-restore.spec.ts`，5.2 漏列）：它从回收站的状态行"See its progress"进入导入导出页，然后在那里读树；提交 1 的 `make e2e` 在 tb2:274 红，照 hi5 改（5.2 的表）。
  - 变更集页的 `h1` 取到达的焦点（决定 835），`h1` 与 `useArrivalFocus` 在读到变更集之后才挂载的 `ChangesetRead` 里；外层 `ChangesetShell` 只读：标签页标题、找不到、读取中（审查 A 的 P3S1A-M1：从别处到达尚未读取的变更集时，挂载时 `h1` 还没画，`useArrivalFocus` 的 effect 不会再跑，焦点落到 `body`）。
- **4.1.5**：`ScrollKept` 必须排在中段最后，经实施者的探针核实（Chromium 1280×720，60 页的树，左栏滚到 1445，以键盘按齿轮进入设置再返回）：在头上返回之后是 1445；挪到 `nav` 之前，放回的 1445 时笔记本的容器还没回到左栏，被夹成 0，树的 reveal 再停在 1372。`workspace-layout.test.tsx` 的滚动一格模拟浏览器的两件事：看不见树时滚不动（守先后，变异 68），`scrollTop` 变了之后异步发 `scroll`（守监听的生命周期：清理掉了、设置模式里也监听，变异 69、69b；修复核对 C 的 P3S1C1-M1）。
- **4.1.7**：`SettingsRow` 是 `observer`（决定 849），`children` 在它的渲染里调用，读到的 store 受它跟踪；外观页在 `children` 里读主题与语言。实施时先以"页在自己的渲染里先读 store"绕开 a8 抓到的问题，审查之后改由组件担。`rowValueClass` 不导出（S1 只有 `SettingsRow` 用，knip 不放过；S2 要用时再导出）。
- **4.1.8**：出了问题时控件区变高，行是 `items-center`，输入框与"保存"随居中上移约 6 px（行高 73 → 88），横向不动（审查 B 的 P3S1B-N1，主持者接受）。
- **4.1.10**：
  - 安全页原来在 `section` 上的 `max-w-md` 挪到 `ChangePasswordForm` 的 `form` 上（`section` 换成了 `SettingsSection`）。
  - 令牌页说明的 `p` 是 `max-w-xl min-w-0 flex-1 basis-56 text-sm text-muted-foreground`（审查 B 的 P3S1B-M1：原样的类在 696 宽时放不下"新建令牌"，按钮折到说明之下）：768 宽以上与"新建令牌"同一行，375、320 宽折到它下面。
  - 令牌行的名字是 `text-sm`（14 px，同样稿 `.set .t`），S2 的列表随之（决定 850）；第二行的创建与到期各在一个 `whitespace-nowrap` 的 `span` 里，窄的时候只在 `·` 之后折行，不折在日期中间（修复核对 B 的 P3S1B1-N1，M13 之前就有）。
- **第 5 节**：
  - 5.2 的表照实加 tb2 一行，表末"8 个"改为"9 个"（审查 C 的 P3S1C-N2）。a8 的注释改写为"子菜单朝右开，指针分步移过去；Radix 依指针的移动判断它是在朝子菜单去，保持子菜单开着"。
  - 5.3：UI2 第 1、2 格的同外框链接先 `focus()` 再按 Enter：WebKit 点按链接不给它焦点（同 Safari），焦点留在 `body`。第 2 格开始计数之前先 `holdStream`、计数之后放开：事件流连上时的刷新会在目录那一次读还在途时再读一次列表，计到 2；现在计的是目录自己那一次，为 1。
- **7.1**：
  - `settings-directory.test.tsx`"笔记本不在列表里"一格的来处用一个 404 的笔记本地址（`AppStores` 拿不到 `RootStore`，没法从列表里删掉一本）；"改了名的笔记本"一格经常规页的表单改名；`tokens-page.test.tsx:259` 写成 `level: 1`。
  - `workspace-layout.test.tsx` 的 Mod+K 一格也核对 Mod+Shift+F：进入设置之前在 `/acme` 上拦下，设置里不管，返回之后再按一次，拦下、焦点到搜索框（重挂的搜索框的 ref；审查 C 的 P3S1C-N1）。
  - 审查之后的新格：`changeset-page.test.tsx` 以 `arrived` 进入尚未读取的变更集，`h1` 拿着焦点（P3S1A-M1）；`settings-origin.test.tsx` 工作区列表答 503，从右上角的账户菜单进入，"返回"是不带名字的"Back"，回 `/lab`（P3S1A-N2）；`settings-page.test.tsx` 的 `SettingsRow` 的 `children` 读一个 observable，变了即重画（决定 849）；`name-row.test.tsx` 扣住一格在放开之后 `settle` 再数，仍只发了一次（P3S1A-N1、P3S1C-M1）；`workspace-layout.test.tsx` 滚动一格的桩看不见树时夹断、值变了时异步发 `scroll`（4.1.5）。
  - 不是先红的格与守它们的：守 `DisplayNameForm` 原有行为的三格（`name-row.test.tsx` 两格、`profile-page.test.tsx` 改写的一格）；`name-row` 放开之后再数、列表读不到时的"返回"、滚动一格的夹断与 `scroll` 在头上本来就绿，由变异 64、66、68、69、69b 守着（7.2）。
- **7.2**：第 8 行的抓手去掉 `user-menu.test.tsx`（7.1 已把那两格的地址换成 `/create-workspace`，抓不住；审查 C 的 P3S1C-N4）；表末加 S1 的实施者另写的 t1–t14b 与审查之后的 64–69b，表后一句。
- **7.4**：有意的偏离（决定 851）：第一组的标题只会重复页名时不写（个人资料、外观与语言、访问令牌；S2 的成员页与无主笔记本页），否则照写（两个常规页的"基本信息"，决定 844；安全页的"密码"）。令牌页在 `2979f1c2` 之后重截（说明与"新建令牌"一行，1440、768、375、320 宽），在 `6987baa9` 之后重截 375、320 宽（日期，中英文、亮暗）。

### 4.2 S2 工作区与笔记本设置

#### 4.2.1 文件

"提交"一列是 S2 的哪个提交（6.2）。

| 文件 | 改动 | 提交 |
|---|---|---|
| `W/pages/workspace/settings-layout.tsx`、`W/pages/notebook/settings-layout.tsx` | `SettingsPage` 的 `title` 换成 `t(shown.label)`（决定 815） | 1 |
| `W/pages/workspace/general-page.tsx` | "基本信息"一组：名称（管理员 `NameRow`，别人只读的一行）、地址（只读的一行）；"危险操作"里删除工作区（4.2.3） | 1 |
| `W/pages/workspace/members-page.tsx`、`invitations-section.tsx`、`W/app/member-row.tsx`、`W/app/member-summary.tsx`、`W/app/role-menu.tsx` | 成员卡片（去掉 `h2`"成员"，删除之后焦点到 `h1`；"移出"是描边的胶囊，决定 838；"你"换成 4.1.7 的标记）、邀请（`h2` 照旧，"撤回"描边）、"危险操作"里离开工作区；`invitations-section.tsx:130` 的 `space-y-1.5` | 1 |
| `W/pages/workspace/ownerless-page.tsx`、`ownerless-row.tsx`、`audit-section.tsx` | 去掉 `h2`"无主笔记本"、接管之后焦点到 `h1`；列表（行的类与"接管""删除"两个描边的胶囊在 `ownerless-row.tsx:75`）与审计日志进卡片；"Open it"带 `arrived`（决定 839） | 1 |
| `W/pages/notebook/general-page.tsx`、`access-options.tsx` | "基本信息"一组：名称（`NameRow`）与开放程度（提交 1 照旧是单选加"保存"，放进卡片里的一块；提交 2 换成 `AccessRow`，4.2.5）；去掉 `h2`"常规"；模式一组（"View the upgrade"带 `arrived`，决定 839）；"危险操作"里删除笔记本；`accesses` 导出（提交 2） | 1、2 |
| `W/components/ui/native-select.tsx` | `useChoice` 的 Esc（决定 840） | 2 |
| `W/pages/notebook/upgrade-banner.tsx` | "View the upgrade"的 `Link` 带 `state={arrived}`（决定 839，4.2.4） | 1 |
| `W/pages/notebook/members-page.tsx`、`add-member-section.tsx` | 去掉 `h2`"成员"，删除之后焦点到 `h1`；添加成员的表单进卡片（`:102,132` 的 `space-y-1.5`）；"危险操作"里离开笔记本；`NoAdmin` 的链接带 `arrived`（决定 839） | 1 |
| `W/pages/notebook/transfer-page.tsx`、`transfer-job-row.tsx`、`import-dialog.tsx` | 导入、导出各一行，最近的任务进卡片（`h2` 照旧）；任务行的两个链接带 `arrived`（决定 839）；`import-dialog.tsx:318,361` 的 `space-y-1.5` | 1 |
| `W/components/ui/input.tsx` | `file:hover:hover-layer`（决定 830） | 1 |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | `transfer.startedBy`："Started by {name}"改为"By {name}"（M12-polish 第 1 项里的 19.2）；中文照旧"由 {name} 发起"；新键 `settings.basics`"Basics"/"基本信息"（决定 844） | 1 |
| 测试、e2e | 第 5、7 节 | 1、2 |

#### 4.2.2 页名作 `h1`、去掉重复的 `h2`

两个布局的 `title` 换成页名之后，下面这些 `h2` 与 `h1` 同名、去掉，原来以它为焦点目标的改用 `useSettingsHeading()`：工作区成员页的"成员"（`members-page.tsx:43,75-76`）、无主笔记本页的"无主笔记本"（`ownerless-page.tsx:66,125-127`）、笔记本成员页的"成员"（`members-page.tsx:36,103-104`，离开对话框的 `focusAfter` `:191` 同改）；工作区与笔记本常规页的"常规"（`general-page.tsx:24`、`:41`，不是焦点目标）。留着的 `h2` 是分组标题（`SettingsSection` 的 `title`）：基本信息（两个常规页的第一组，决定 844：样稿的每张卡片上都有分组标题，没有它时第一张卡片贴着 `h1`，比样稿高 36 px，审查 B 的 M2）、邀请（焦点目标，`invitations-section.tsx:33,63`）、审计日志、添加成员、模式（焦点目标，`general-page.tsx:159,179`）、导入、导出、最近的任务（焦点目标，`transfer-page.tsx:67,206`）、危险操作。读 `h2` 的格（"Mode""Import"等）不受"基本信息"影响。

#### 4.2.3 工作区的三页

- **常规**："基本信息"一组（`SettingsSection title={t("settings.basics")}`），一张卡片两行：名称（管理员是 `NameRow`：`field="name"`、`label` 是 `createWorkspace.name`、`check={workspaceNameProblem}`、`save={(name) => workspaces.rename(workspace.slug, name)}`，"保存""已保存。"的键照旧；不是管理员时只读的一行：标题"名称"、说明 `workspaceSettings.nameAdminsOnly`、`value` 是名字）；地址（只读：标题 `createWorkspace.slug`、说明 `workspaceSettings.slugFixed`、`value` 是 slug）。只读的值用 `rowValueClass`（4.1.7；名字最长 80 字，`W/app/slug.ts:7`）。管理员另有"危险操作"一组：一行，标题与说明是今天删除一节的 `workspaceSettings.deleteTitle`、`deleteBody`，按钮"Delete workspace"是 `variant="destructive" size="sm" shape="pill"`，打开今天的输入 slug 的确认框（e2e 的 `deleteWorkspaceWith` 以按钮名找它）。
- **成员**：`SettingsSection`（无标题）：成员的卡片 `<SettingsCard list>`，`ul` 的 `aria-label` 照旧"成员"（`members-page.tsx:83`），每行是 4.1.7 的简单的行：名字与邮箱（"你"是 4.1.7 的标记）、角色菜单（`W/app/role-menu.tsx` 的按钮换成 30 px 胶囊）、移出（`outline` 的 30 px 胶囊，`aria-label` 照旧，决定 838）；邀请一组（`SettingsSection title="Invitations"`，`heading` 照旧是它的 ref）：一张 `div` 的卡片，里面先是一块邀请的表单（`px-4.5 py-4`，决定 821），再是邀请的列表：一个自带 `divide-y divide-border` 的 `ul`（名称照旧；卡片的 `divide-y` 在表单与列表之间画一条线），行是 4.1.7 的叠的行（`invitations-section.tsx:193`：主行是邮箱、角色与日期，行内的按钮是描边的胶囊；复制不成时显示的 `LinkField` 叠在主行之下），没有邀请时在列表的位置上是一行 `invitations.empty`（4.1.7 的空）；复制链接的 `<output>` 照旧在卡片之下；"危险操作"一组：离开工作区一行（标题 `members.leaveTitle`，说明 `members.leaveBody`，按钮"Leave workspace"`destructive`；今天是 `outline`，进了危险组换成红字描边，00 号文档 4.4.2）。
- **无主笔记本**（管理员）：说明一段（今天 `h2` 下的那段）在 `SettingsSection` 的顶上，列表进 `<SettingsCard list>`（`ul` 的名称照旧），每行（`ownerless-row.tsx:75`）是简单的行：名字、前任与开放程度、成员数，"接管"与"删除"两个描边的 30 px 胶囊（决定 838）；接管之后的状态 `<output>` 照旧，里面的"Open it"带 `arrived`（决定 839）；审计日志一组（`SettingsSection title="Audit log"`，`ol` 进卡片，名称照旧）。

#### 4.2.4 笔记本的三页

- **常规**："基本信息"一组，一张卡片：名称（管理员 `NameRow`，`check={notebookNameProblem}`、`fieldTexts={notebookNameTexts}`、`save` 同今天 `:51`；别人只读的一行，说明 `notebookSettings.nameAdminsOnly`）、开放程度（管理员 `AccessRow`，4.2.5；别人只读的一行：标题"Workspace access"、说明是当前值的说明，其后 `accessAdminsOnly` 那一句自成一个元素（`<span className="block">`：`general-page.test.tsx:156` 以整句精确地找它，拼进一个字符串就找不到，审查 C 的 R1-N4）、`value` 是当前值）；模式一组（`SettingsSection title="Mode"`，`heading` 照旧，卡片里一行：当前模式与说明，右边是降级或升级的按钮（`outline` 的 30 px 胶囊，文字照旧"Downgrade to a standard notebook…"等，`downgradeWith` 与 `upgrade-pages.ts:36` 以它们找），升级进行中的状态照旧在这一行下，里面的"View the upgrade"带 `arrived`（决定 839））；升级进行中，笔记本的每个设置页顶上还有 `NotebookLayout` 画的升级横幅（`W/pages/notebook/upgrade-banner.tsx:47`，管理员看得到其中的"View the upgrade"），它的 `Link` 同样带 `arrived`：去的升级页上横幅不画这个链接，它随之卸下（决定 839，审查 B 的 R1-N3；在别的笔记本页上它也总是离开本页，带上无害）。横幅在 `main` 的全宽、`SettingsPage` 之外，比 696 px 的内容列宽：S2 截一张（7.4），对齐留给 P4（第 9 节）；样稿模式一行的标题带"当前"徽记，这里不加：这一组只有一行，说的就是这本笔记本现在的模式，没有并列的另一种要区分（有意的偏离，审查 B 的 N3，7.4）；"危险操作"一组：删除笔记本（按钮"Delete notebook"，确认框照旧要输入名称）。
- **成员**：成员的卡片（同工作区的成员页，`ul` 的名称照旧，角色菜单与描边的"移出"）；`NoAdmin` 的提示在卡片之上，它的链接带 `arrived`（决定 839）；添加成员一组（`SettingsSection title="Add a member"`，卡片里是今天的 `AddForm`，决定 821）；"危险操作"一组：离开笔记本。
- **导入、导出与恢复**：导入一组（`SettingsSection title="Import"`，卡片一行：`SettingsRow` 没有标题，组标题就是它的名字，说明 `transfer.importBody` 作这一行的文字，右边"Import a zip…"按钮（`outline` 30 px 胶囊）；不能写时那一行的右边是今天的说明）、导出一组（同样）、最近的任务一组（`SettingsSection title="Recent jobs" heading={heading}`，`ol` 进 `<SettingsCard list>`，名称照旧；任务行 `transfer-job-row.tsx` 的外层换成 4.1.7 的叠的行：主行照旧 `flex flex-wrap items-center justify-between`，`role=alert` 的拒绝与展开的报告叠在它之下；"See in the history"与"The change undone"两个链接带 `arrived`，决定 839）。读取中、失败与空照 4.1.7。`transfer-import.test.tsx:46-53` 读最后三个 `h2` 是"Import""Export""Recent jobs"：次序不变。

#### 4.2.5 开放程度（`AccessRow`，决定 825、840）

替换 `AccessForm`（`general-page.tsx:81-135`）；S2 的提交 2：

```tsx
const AccessRow = observer(function AccessRow({ workspace, notebook }: NotebookProps) {
  const notebooks = useNotebooks(workspace);
  const t = useT();
  const id = useId();
  /** The access chosen, shown until the server answers the latest choice; none, the list's. */
  const [draft, setDraft] = useState<WorkspaceAccess>();
  const shown = draft ?? notebook.workspace_access;
  const [saved, setSaved] = useState(false);
  const { ref, banner, submit } = useForm<never>([], {
    texts: { forbidden: "notebookSettings.accessAdminsOnly" },
  });
  /** How many choices went out: only the latest's answer shows. */
  const sent = useRef(0);
  const choice = useChoice(shown, (value) => {
    const chosen = accesses.find((access) => access.value === value)?.value;
    if (chosen === undefined) {
      return;
    }
    sent.current += 1;
    const mine = sent.current;
    setDraft(chosen);
    setSaved(false);
    void submit({}, async () => {
      try {
        await notebooks.update(notebook.id, { workspace_access: chosen });
      } catch (error) {
        if (sent.current !== mine) {
          return;
        }
        setDraft(undefined);
        throw error;
      }
      if (sent.current === mine) {
        setDraft(undefined);
        setSaved(true);
      }
    });
  });
  // What the select shows: a step of the keys too, waiting (decision 840).
  const hint = accesses.find((access) => access.value === choice.value)?.hint;
  return (
    <form ref={ref} noValidate onSubmit={(event) => event.preventDefault()}>
      <div className={rowClass}>
        <div className={rowTextClass}>
          <label htmlFor={id} className={rowTitleClass}>{t("access.legend")}</label>
          <p id={`${id}-hint`} className={rowDescriptionClass}>{hint === undefined ? "" : t(hint)}</p>
        </div>
        <div className={rowControlClass}>
          <SavedOutput saved={saved} label={t("notebookSettings.saved")} />
          <NativeSelect id={id} aria-describedby={`${id}-hint`} aria-busy={draft !== undefined || undefined}
            className="h-7.5 md:text-compact" {...choice}>
            {accesses.map((access) => (
              <option key={access.value} value={access.value}>{t(access.label)}</option>
            ))}
          </NativeSelect>
        </div>
      </div>
      {banner !== undefined && <div className="px-4.5 pb-3.5"><Alert>{banner}</Alert></div>}
    </form>
  );
});
```

- **为什么这样**：`useChoice` 的 `applied` 是 `shown`：选了之后草稿就是它，`useChoice` 的待定值随之清掉（`native-select.tsx:65-68`）；被拒时草稿清掉，`shown` 回到列表里的值，下拉随之退回。`NotebookStore.update` 不是乐观的（`W/stores/notebook.store.ts:108-114`），没有草稿时下拉会在答复之前跳回旧值。
- **只有最后一次算**：`useForm` 的 `submit` 每次开头清掉上一次的失败（`W/app/form.ts:55-57`），所以第二次发出时第一次的横幅就没了；第一次之后才回来的拒绝在 `catch` 里被吞掉（`submit` 当它成功，不设失败、不移焦点）；第一次之后才回来的成功不清草稿、不显示"已保存。"。`NotebookStore.update` 按笔记本排队（`inTurn`，`W/stores/notebook.store.ts:108-114`）：连续两次选择的请求依次发出，后一次在前一次答复之后才发，最后的答复就是最后一次选择的，列表的值最后是它；前一次的成功先把列表改成前一次的值，草稿还在，下拉照旧显示后一次的选择。
- **焦点**：下拉不禁用，焦点一直在它上面；被拒时焦点没丢，`useForm` 不移焦点（横幅是 `role=alert`，读屏会读）。"已保存。"在每次发出时清空，每次写出时换轮次（决定 823、841）。发送中下拉带 `aria-busy`（审查 B 的 N7：初稿发送中界面上看不出在发），取草稿在不在：草稿在发出时设、只有最后一次的成功或拒绝清它，正好是"最后一次的答复还没回来"。不取 `useForm` 的 `sending`：`submit` 每次开头置真、在自己的 `finally` 里置假，不计数，连选两次时第一次答复回来就置假，第二次的请求这时才发出（`inTurn`），整个第二次在途时没有 `aria-busy`（审查 B 的 R1-N2；修订核对之后的探针 Q3：取 `sending` 时依次是 `["true", null, null]`，取草稿时 `["true", "true", null]`）。草稿同步设上，`aria-busy` 也是"发出了没有"的同步见证（7.1）。
- **说明**：`p` 的文字是下拉显示的那一项的说明（`choice.value`，即 `pending ?? applied`，`native-select.tsx:76`）：键盘逐项走到、还没发出的那一项也是（审查 B 的 M5：初稿取 `shown`，走到"Workspace can edit"时说明还是原来那一项的），随草稿与退回变；它是下拉的 `aria-describedby`。
- **Esc**（决定 840，`W/components/ui/native-select.tsx` 的 `useChoice`）：`onKeyDown` 里 `event.key === "Escape" && pending !== undefined` 时 `preventDefault()`、`stopPropagation()`、`stepped.current = false`、`setPending(undefined)`，不再往下；没有待用的选择时不碰 Esc。走到"Workspace can edit"之后按 Esc，下拉与说明回到已生效的那一项，失焦也不发（审查 B 的 Q3：初稿只能失焦即发）。先后：今天用 `useChoice` 的下拉（搜索的范围、目录与历史的筛选、开放程度）都不在对话框与抽屉里；Radix 对话框的 Esc 是 document 上的捕获监听（`useEscapeKeydown`），先于下拉自己的 `onKeyDown`：若将来有这种下拉放进对话框，Esc 先关对话框，待用的选择随下拉卸下、不发出（修订的探针）。375 px 的抽屉碰不到它，理由有三（审查 A 的 R1-N1；初稿写的"抽屉开着时 `main` 是 `inert`"不对，只有关着的抽屉根带 `inert`，`shell-sidebar.tsx:348`）：(1) 抽屉的 Esc 是它的根上的原生监听（`shell-sidebar.tsx:243-268`），`main` 里的下拉发出的 `keydown` 不经过它，`stopPropagation` 与否都一样；(2) 抽屉开着时焦点困在抽屉里（`FocusScope` 的 `trapped`，`:336-341`），遮罩盖着 `main`，下拉拿不到焦点；(3) 打开抽屉（菜单按钮或 Mod+K）时下拉先失焦，待用的选择随之用上（P2 决定 807，`:124-125`）。审查 A 在 375 px 的搜索页上三种引擎跑过这三条。`stopPropagation` 今天没有一处看得出（没有祖先在冒泡阶段听 Esc），由 `native-select.test.tsx` 的一格守（父元素收不到，7.1）。筛选页一并有了这一键。macOS 上打一个字母找选项也算待用的选择（`stepping`），Esc 同样退回。
- **样子**：控件区里"已保存。"在下拉之前（同样稿 `settings.html:200`；375 px 时在下拉之后，4.1.7），下拉不随它出现而挪，也不折行：宽屏上它一直占 68 px（决定 848；修订核对第 2 轮之后的探针：768–1200 每 2 px，中英文、三种引擎，下拉的位置与行高都不变，离卡片右边 19 px，"已保存。"的字离下拉 8 px）。代价：说明列 408 px，英文说明在宽屏上两行；768–912 宽时下拉一直在说明之下（4.1.7）。下拉 30 px（`h-7.5`，样稿 `.select` 30 px、8 px 圆角、13 px；圆角照 `NativeSelect` 的 `rounded-control` 9 px，不另改），宽屏 13 px（`md:text-compact`）。
- **不做的**：键盘逐项走等失焦的规则不在这里另写（`useChoice` 的，`native-select.test.tsx` 守）；新建笔记本对话框里的单选不动（那是提交型的表单）。

#### 4.2.6 危险操作组（决定 826、838）

每页最后一组，`<SettingsSection title={t("settings.danger")}>`，卡片里每项一行：`SettingsRow`（标题、说明是今天那一节的 `h2` 与说明的键），控件是今天的 `ConfirmDialog` 的触发按钮，`variant="destructive" size="sm" shape="pill"`，按钮文字照旧。确认框、`texts`、`focusAfter`、到达的去处都不变。工作区常规（删除）、工作区成员（离开）、笔记本常规（删除）、笔记本成员（离开），个人设置的安全页（停用）在 S1 已是这样。只给管理员的照旧只给管理员（13.2 第 18 条）；组里一项也没有时（不是管理员的常规页）不画这一组。红字描边只在这一组：列表行内的移出、撤回、删除是 `outline`（决定 838），成员多时不是一列红按钮。

#### 4.2.7 移交的认领（决定 830、846）

- **M12-polish 第 1 项里的 19.2**（现象与改法写在 M5-polish 第 19 条）：英文的任务行今天是"Started by Bob · Started Oct 5, …"（`transfer-job-row.tsx:72-73`），`transfer.startedBy` 改成"By {name}"，读作"By Bob · Started Oct 5, …"；中文"由 Bob 发起 · 开始于 …"不改。`transfer-page.test.tsx:91` 随之改。进展写进 `docs/v0.2/handoffs/M12-polish.md` 的"处理进展"（S2 的文档提交；M5-polish 已是 `done`，不动）。
- **01-P1 第 9 节的标签组**：S1 的 `create-token-dialog.tsx:184`；S2 的五处：P1/S2 审查记录 S2B-N3 列表里的四处（`invitations-section.tsx:130`、`import-dialog.tsx:318,361`、`add-member-section.tsx:132`）与列表外的 `add-member-section.tsx:102`；`space-y-2` 改 `space-y-1.5`，与 `FormField` 同（`add-member-section.tsx:102` 的 `min-w-56 flex-1` 留着）。列表里的 `move-page-dialog.tsx:159,172` 在 P2/S2（`831e50a6`）就已是 `space-y-1.5`。列表里剩下五处（`asset-dialogs.tsx:221`、`search-page.tsx:221`、`index-filters.tsx:60`、`history-filters.tsx:154`、`source-add-dialog.tsx:261`），连同决定 846 补上的 `history-filters.tsx:132,143` 与 `index-filters.tsx:48`，共八处，在 P4、P5（第 9 节；审查 C 的 P3S2C-N1）。完成写进本文 10.2 与 01-P1 第 9 节的交接处。
- **01-P1 第 9 节"P3"的文件按钮**：`Input` 加 `file:hover:hover-layer`（`hover-layer` 叠在按钮的底上，亮色 `--surface`、暗色 `--active` 之上叠 `--hover`）。`::file-selector-button:hover` 在三种引擎里都有，S2 截图核对（7.4）；暗色的底照旧 `--active`。
- **30 px 的下拉**：开放程度的 `h-7.5`（01-P1 第 9 节"P3"）。

**照实**（S2 的实施与审查，[S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)）：

- **4.1.7、4.2.1**：
  - `SettingsCard` 加 `ordered`（画 `ol`）与 `label`（列表的 `aria-label`）；导出列表行的类名常量 `listRowClass`、`stackedRowClass`、`listLineClass`、`badgeClass`、`cardTextClass`，令牌行与令牌页也改用它们（字符串没变，`b0768903`）；`access-options.tsx` 导出 `accesses`（`5cb1de3f`）。
  - 审查之后另导出 `listTextClass`（`min-w-0 flex-1 basis-56`，同 `rowTextClass`：列表行的文字块伸缩，按钮留在右边）与 `WholeParts`（明细一段一个 `whitespace-nowrap` 的 `span`，段间" · "），`badgeClass` 前面加 `ms-1.5 inline-block`，名字的 `p` 不再是 `flex`（`1e6231e9`，审查 B 的 P3S2B-M1、N1、N2）。
  - `rowValueClass` 照旧不导出：导入一行不能写时的那句经 `SettingsRow` 的 `value` 用它（审查 B 的 P3S2B-N4），导出了没有别的用者，knip 不放过。
  - S1 的 `settings-origin.test.tsx`、`settings-directory.test.tsx`、`workspace-layout.test.tsx` 读旧的 `h1`（"Workspace settings""Plans settings"），随提交 1 改读页名；同外框链接的焦点由比 `textContent` 改为比元素：`h1` 与目录里的链接同名（"General""Security"）。
- **4.1.10**：安全页密码卡片写出的那句（"Password changed. …"）与"Change password"在可折的一行（`flex flex-wrap items-center gap-x-3 gap-y-2`），同邀请与添加成员：空的时候不占宽，按钮到卡片底 17 px，原来 32 px（审查 B 的 P3S2B-N3，S1 的页，`1e6231e9`）；写出之后那句落到按钮之下、占满表单宽。初版并排、不折，长句挤成窄的一列（1440 宽 284 px、三行，375 宽 125 px、七行；B 修复核对的 P3S2B1-M1，`8d98f64f`）。
- **4.2.3**：
  - 列表行的文字块是 `listTextClass`：1440、768 宽按钮在右、离卡片 19 px，375 宽在文字之下；控件宽的一行（工作区成员页里带角色菜单与"移出"的）在 768 宽也落到文字之下，是 `basis-56` 的规则（审查 B 的 P3S2B-M1）。
  - 邀请组的说明 `p` 在 `h2` 与卡片之间；卡片里先是表单那一块，再是行；读取中、失败与空都在卡片里（空的是 `cardTextClass` 的一行）；复制链接的输出在卡片之下。无主笔记本页与审计组的说明在卡片之上，空列表是卡片里 `cardTextClass` 的一行。组说明都是 `max-w-2xl`（决定 852）。
  - 无主笔记本页说明之后与邀请卡片之后的 `<output>` 是 `empty:absolute`：Chromium 里空的 `output` 挡住外边距折叠（20 / 36 px，应为 10 / 26）；它照旧挂着、在无障碍树里（审查 B 的 P3S2B-M2，`1e6231e9`）。
  - 邀请与添加成员的"Role"下拉 `block`，在标签之下：`Label` 是行内的，原来与下拉同行，main 上就这样（决定 821 的字段竖排，`397241c7`）。
  - 邀请写出的那句（"Invited {email}. …"）与"Invite"在可折的一行，长了落到按钮之下；output 是 `min-w-0 break-words`，长地址里没有可断处的一段在表单边上断开，320 宽不横向滚动（B 修复核对的 P3S2B1-M1，`8d98f64f`）。
  - `MemberSummary`："You"跟在名字最后一个字之后；地址 `break-words`，"Joined …"在 `whitespace-nowrap` 里（`397241c7`、`1e6231e9`）。无主行的原所有者一行 `break-words`，明细经 `WholeParts`（审查 B 的 P3S2B-N1、N2）。
- **4.2.4**：
  - 升级进行中的那一句在模式卡片里、行之下（`-mt-1 px-4.5 pb-3.5`）。
  - 任务行的主行照设计是 `items-center`（原来 `items-start`）；"Report"是 ghost 的 30 px 胶囊，"Cancel"是 outline 的；明细除"By {name}"（名字长度不定，375 宽会横向滚动）外经 `WholeParts`（审查 B 的 P3S2B-N1）。
  - 导入一行不能写时（读者、升级中），那句是 `SettingsRow` 的 `value`：灰、靠右、最多 60%；初版是控件区里 `max-w-xs` 的 `p`（审查 B 的 P3S2B-N4，`1e6231e9`）。
  - 添加成员写出的那句（"{name} added."）与"Add"在同一行，卡片底下不再多 16 px（`397241c7`）；这一行可折，名字长了那句落到按钮之下，output 是 `min-w-0 break-words`（B 修复核对的 P3S2B1-M1，`8d98f64f`）。
  - 导入对话框的"Import under"下拉 `block`（`397241c7`）；移动页面对话框的"Parent page""Position"同样（决定 853，`1e6231e9`）。
- **4.2.5**：
  - `native-select.test.tsx` 父元素一格的包装是 `role="toolbar"` 加 `tabIndex={-1}`：静态元素上的 `onKeyDown` 过不了 jsx-a11y，这样不加豁免。
  - 名称的扣住格在发出期间点按、回车，放开之后 `settle` 再数；开放程度的扣住格（指针一格）在发出期间不点也不按，只在放开之后再数（审查 C 的 P3S2C-N2）。工作区名称扣住一格打字途经当前名"Acme"，"保存"卸下再挂上，打完再取一次。
  - `useChoice` 的注释照审查 A 的 P3S2A-N2 改：`stopPropagation` 只挡冒泡的监听；Radix 对话框的 Esc 是 document 上的捕获，照旧先到（`dd6ef368`）。
- **5.4**：UI2 第 2 格也读"Plans settings"，同改为"General"（原来只列了第 1、3 格）。
- **5.5**：UI3 第 1 格另在第一次"Saved."之后读 `animationName`，第二次断言不同（审查 C 的 P3S2C-M1，`dd6ef368`）：透明度抓不住轮次，两次写出之间隔着一次网络往返，空的一态总会画出，淡出不靠轮次也重来。
- **7.1**：
  - 成员页（工作区、笔记本）与无主笔记本页加分组的格：列表无标题，其后依次是"Invitations"或"Add a member"（只给管理员）、"Danger zone"里的离开；无主笔记本页是"Audit log"，带 `ol`（`397241c7`，变异 t19 活下来之后补）。
  - "See in the history""The change undone"带 `arrived`，到达尚未读取的变更集，`h1` 取焦点（`transfer-page.test.tsx`、`transfer-background.test.tsx` 各一格）。
  - 审查之后的新格：`settings-directory.test.tsx` 两格，375 宽"到达之后再到达"（工作区：首页抽屉的"Settings"进 General、再点"Members"；个人：Profile 再点"Security"，换了布局，抽屉按钮每次重取），断言第二个 `h1` 是新元素、拿着焦点（审查 A 的 P3S2A-N1）；`transfer-import.test.tsx` 一格，编辑者在页上变成读者、再变回编辑者，导入一行跟着变（审查 B 的 P3S2B-N4）。
  - 不是先红的：上面分组的四格与审查之后的三格守已有的行为，在头上本来就绿，由变异 t19、t21–t23、71–73 看到红。
- **7.2**：第 56 行的 S2 抓手照审查 C 的 P3S2C-M1 改写；表末加 S2 的实施者另写的 t1–t23 与审查之后的 70–74，表后一句。
- **7.4**：
  - 800 宽"选前后各一张，行高与下拉的位置相同"要在说明一样的两个值之间核（read → edit → read）：说明随选择变，viewer 的说明在 800 宽两行，none 与 editor 的一行（行高 131.5 对 112.75）。
  - 文件按钮的悬停三种引擎、亮暗都看过：亮色 255 → 245，暗色 54 → 63；亮色下悬停的按钮与输入框的底（244）接近，靠边框分开，是决定 757 的叠层在灰底上的通例，不改（审查 B）。
  - 审查之后重截（`1e6231e9`、`dd6ef368` 之后，Chromium 亮暗 × 1440、768、375；B 修复核对的 `8d98f64f` 之后，安全页、邀请与添加成员写出之后，亮暗 × 1440、375、320）：见 10.2。

### 4.3 lint

不加豁免。分离工作树里 S1 的大部分与 `AccessRow` 在仓库的 `.oxlintrc.json` 下 `oxlint --max-warnings=0` 零警告：`useOriginRecord` 的渲染时 `setState` 没有触发规则（同 `useDrawer` 的写法）；`Layout` 里 `useMatches()` 必须无条件调用：写在 `&&` 的右边违反 hooks 的规则，而 tsc 与仓库的 oxlint 配置都不报（探针里先写错过一次；把它写回 `&&` 的右边再跑 `oxlint --max-warnings=0`，没有警告），审查 A 要看。`oxfmt` 会把 `NameRow` 的 `described` 折成两行（4.1.8 已照折）。修订的工作树里另有一处：`trash-preview.tsx` 的两个 `useRef` 换成 `useArrivalFocus` 之后，`react-hooks(exhaustive-deps)` 不再把它认作稳定的 ref，要求两个 effect 的依赖加上 `heading`（同 `page-layout.tsx:299`、`health-load-more.ts:45` 的写法），加上之后零警告（决定 835）。`SavedOutput` 的渲染时 `setState` 同 `useChoice` 的 `was`，没有触发规则。

## 5. e2e

### 5.1 S1：夹具

`e2e/fixtures/settings-pages.ts` 加四个帮手（定位集中在这里，00 号文档第 10 节"外框改动波及的 e2e 故事多"的应对）：

```ts
/** A group of the settings' directory, by its navigation's name (M13/P3/S1, decision 817). */
export function settingsGroup(page: Page, name: "Settings" | "Workspace settings" | "Notebook settings"): Locator {
  return page.getByRole("navigation", { name, exact: true });
}

/** The directory's Back: "Back to name", or "Back" alone (decision 813). */
export function settingsBack(page: Page, name?: string): Locator {
  return page.getByRole("link", { name: name === undefined ? "Back" : `Back to ${name}`, exact: true });
}

/** The settings page's title, its h1: the page's name (decision 815). */
export function settingsTitle(page: Page, name: string): Locator {
  return page.getByRole("heading", { level: 1, name, exact: true });
}

/** Opens the settings of the notebook shown by the cog of its row in the left column (M13/P2/S2's "Settings of …"). */
export async function openNotebookSettings(page: Page, notebook: string): Promise<void> {
  await page.getByRole("link", { name: `Settings of ${notebook}`, exact: true }).click();
}
```

`tools/e2e-navigation/check.mjs`（故事只经帮手换文档）不受影响：这些都是点链接，不是 `page.goto`。

### 5.2 S1：在设置页上读左栏的故事（逐个核对过的改法）

| 故事 | 今天 | 改法 |
|---|---|---|
| w4 `e2e/stories/workspace/w4-rename-delete.spec.ts:111-116` | 在常规页上改名之后看切换器（`switcher(page, "Acme Labs")`，`:116`） | `:116` 改为 `settingsGroup(page, "Workspace settings")` 含"Acme Labs"，且 `settingsBack(page, "Acme Labs")` 可见（直接打开的地址，没有来处，回组里的工作区）。`:112` 的 `toHaveURL(…/general)` 照旧（`IndexTo`） |
| n8 `e2e/stories/notebook/n8-remove-deactivate.spec.ts:133,147` | 在成员页上移出之后点左栏的"首页"（`workspaceNav(…).getByRole("link", { name: "Home" })`，`:147`） | `:147` 改为 `settingsBack(page, "Acme").click()`（`:133` 直接打开，回工作区首页）；`:148-149` 照旧 |
| n9 `e2e/stories/notebook/n9-take-over.spec.ts:168` | 在无主笔记本页上接管之后看左栏的分组 | `:168` 挪到 `:176-177`（"Open it"之后，笔记本首页）之后；`:186-189` 不改（目录的"工作区设置"一组同名，成员看不到"无主笔记本"） |
| n3 `e2e/stories/notebook/n3-open-notebook.spec.ts:66-67,76-80,84` | 直接打开常规页，改开放程度前后看分组 | `:66` 改为打开笔记本首页（`notebookPath(slug, id)`），`:67` 在那里看"我的"；接着 `openNotebookSettings(page, "Handbook")`；`:76-78` 照旧（S1 的 `changeAccessWith` 照旧是单选加"保存"）；`:80` 之前 `settingsBack(page, "Handbook").click()`，在首页看"团队"；`:84` 之前再 `openNotebookSettings(page, "Handbook")` |
| n6 `e2e/stories/notebook/n6-rename-delete.spec.ts:80` | 改名之后看分组 | `:80` 改为 `settingsGroup(page, "Notebook settings")` 含"Plans 2027"；`:84-88` 照旧（删除之后到工作区首页） |
| n4 `e2e/stories/notebook/n4-notebook-members.spec.ts:156` | 加成员之后看分组 | `:156` 挪到这一格的末尾（`:166` 之后）：`settingsBack(page, "Acme").click()`（`:144` 直接打开），在首页看"团队"；`:164` 在 S1 照旧（S2 改，5.5） |
| lw12 `e2e/stories/wiki/lw12-downgrade.spec.ts:160-161,187-190,216-220,262` | 两个标签页停在常规页上，看各自树里的"目录""历史"两行随降级消失，再从 `page` 的树里删 AGENTS | `:160-161` 两个标签页都先打开笔记本首页、再 `openNotebookSettings`（"返回"才回到有树的首页）；`:187-190` 的 `virtualLinks` 只取 `sourceTab` 的两行（`writer` 的树在 `hidden` 里）："不经导航、不开新流就消失"的要点留在 `sourceTab` 身上；`:220` 之后 `writer` 点 `settingsBack(writer, notebook.name)`，先等 `pageTree(writer, notebook.name)` 可见（正面的等待），再断言它的两行是 0；`:262` 之前 `page` 点"返回"、等笔记本首页的标题。`:257-260` 的 `lw12Document`、流的计数照旧成立（"返回"是应用内的导航，不换文档，不重连）。lw12 在 `threeBrowsers` 里 |
| hi5 `e2e/stories/history/hi5-revert-import.spec.ts:213` | 在导入导出页上（`:189` 从变更集页的"See its progress"进来）读树 | `:213` 之前 `settingsBack(page, "Notes").click()`（来处是变更集页，那里有树），等变更集页的标题拿着焦点（`toBeFocused`，决定 835：初稿只等它可见，焦点落到 `body` 也过，审查 A 的 I2）；`:213-217` 照旧 |
| tb2 `e2e/stories/trash/tb2-background-restore.spec.ts:274`（照实加，S1 的审查记录） | 从回收站的状态行"See its progress"进入导入导出页，在那里读树 | `:274` 之前 `settingsBack(page, "Plans").click()`，等回收站的 `h1` 拿着焦点，再读树（同 hi5） |

这 9 个故事（tb2 是照实加的）都在 S1 的提交 1（设置模式与目录一来就要改）。

**a8**（`e2e/stories/identity/a8-profile.spec.ts`，只在 Chromium 跑）：
- 提交 1：`:78`、`:84` 的 `h1`"设置"改为 `settingsTitle(page, "个人资料")`（主题与语言的单选还在个人资料页上）；`:87-104` 的账户菜单现在是左栏底部的账户行（按钮名相同，`accountMenu` 照旧找得到），子菜单朝右开，`:90-92` 的注释照改，指针的走法不变（审查 A 在修订核对里以临时故事在 Chromium 里照 a8 的步子走过，英文与中文各一遍，子菜单一直开着、主题即生效；新位置上的路径不经过父菜单的别的项，注释里"跳过去会关上"的理由随之改写；改好的 a8 本身由 S1 提交 1 的 `make e2e` 核对）。
- 提交 2：`:52` 照旧打开 `/settings/profile`；`:55-59` 的 `save` 照旧（`fill` 之后才有"保存"）；`:68` 之后加 `await expect(name).toBeFocused()`（保存成功、"保存"消失，焦点回到输入框，决定 822）；`:73-79` 之前点 `settingsGroup(page, "Settings")` 里的"Appearance and language"，`:78` 的标题改为 `settingsTitle(page, "外观与语言")`；`:84` 同；`:106-107` 之间先点名为"设置"的 `nav` 里的"个人资料"（`settingsGroup` 只收英文的名字，这里直接写 `getByRole`），再看"显示名"。`:69` 的 `getByRole("status")` 在个人资料页上仍只有一个（`NameRow` 的）。

**a15**（`e2e/stories/identity/a15-leave-mid-refresh.spec.ts`，三种浏览器）：
- 提交 1：`expectSignedInOn`（`:189-203`）的 `:197-198` 两句合成一句 `await expect(page.getByRole("heading", { level: 1, name: section, exact: true })).toBeVisible()`；`:311` 的 `"Password"` 改为 `"Security"`（安全页的 `h1`）；`:530`、`:596`、`:607`、`:696`、`:724` 的 `level: 2, name: "Profile"` 照旧（提交 1 里个人资料页的 `h2`"Profile"还在）。
- 提交 2：那五处改为 `level: 1`；读 `h2`"Password"的七处（`:356,392,453,501,555,636,683`）照旧（安全页的分组标题）。
- 随每次 `make e2e` 照常跑一次，不另跑三遍（决定 829、843：个人设置页上没有来处、没有最近的工作区时不读列表，a15 的页一次也不多读）。

**核对过、不改的**：tr1（`e2e/stories/transfer/tr1-export.spec.ts:214-217`）、tr2（`tr2-import.spec.ts:201-204`）以"笔记本设置"找目录里的链接，同名；a7（`a7-change-password.spec.ts:94,118`）在安全页上找账户按钮，左栏底部同名；n5（`:66-73`，离开之后到工作区首页再读左栏）、pg13（`:103-111`，删除之后到首页；`:136` 是已删工作区的 404）、up8（`:88` 起在常规页开始升级，`:135` 读树时已在笔记本首页）、lw10（`:369` 起 `writer` 在导入导出页只点导出）、n11（`:138` 在邀请之后的工作区页上读）、n7、n10、n13、w5、w8、w9、w10、a5、a10、a11、as1、as4、tr3、tr4、up1、up2、lw6、lw7、lw8、pg14 在设置页上不读左栏（`grep` 这些文件里的 `workspaceNav`、`notebookGroups`、`pageTree`、`treeTitles`、`switcher`、`searchBox`、`searchFor`、`openDrawer` 都是 0 处或在离开设置之后）。

### 5.3 S1：UI2（`e2e/stories/shell/ui2-settings-mode.spec.ts`，三格，三种浏览器）

只有页面版本：它核对界面的进出、名字与焦点；两处改名照 w4、n6 调同一组数据库断言，接口与 MCP 版本由 W4、N6 覆盖（13.4 第 3 条的豁免，8.2）。造数经 API：一个团队（管理员，工作区"Acme"）、一本笔记本"Plans"、其中一页"Notes"。S1 的提交 1（决定 842）。

1. **"UI2 (page): the settings are entered from the column's Settings, a notebook's cog and the account's menu, each arrived at; Back goes back where they were entered from"**：
   - 打开"Notes"那一页；点笔记本行的齿轮：地址是 `…/settings/general`，`settingsTitle(page, "Plans settings")` 拿着焦点（S2 起"General"）；没有 `searchbox`，没有切换器，没有名为"Pages of Plans"的可见 `nav`（树在 `hidden` 里，决定 814；审查 C 的 M9：变异 16 要 UI2 也抓）；`settingsGroup` 三组都在，"Workspace settings"含"Acme"，"Notebook settings"含"Plans"；
   - 点"笔记本设置"组的"Members"：地址变了，焦点留在那个链接上（`opened`）；
   - 点"设置"组的"Profile"：`settingsTitle(page, "Profile")` 拿着焦点（跨外框，`arrived`）；`settingsBack(page, "Plans")` 在（来处跨外框留着）；点它：回到"Notes"那一页，页的标题拿着焦点，树回到左栏（"Pages of Plans"可见）；
   - 在工作区首页点左栏的"Settings"：`settingsTitle(page, "Workspace settings")` 拿着焦点（S2 起"General"）；"返回 Acme"回首页，首页的标题拿着焦点；
   - 在笔记本首页打开账户菜单、选"Settings"：`settingsTitle(page, "Profile")` 拿着焦点，"返回 Plans"回笔记本首页。
2. **"UI2 (page): opened at their address, the directory names its workspace and notebook as renamed, and Back goes to the workspace"**：
   - 直接打开 `…/notebooks/:id/settings/general`：`settingsBack(page, "Acme")`；`renameNotebookWith(page, id, "Plans 2027")`：200，"Notebook settings"组含"Plans 2027"，`expectNotebook`（同 n6）；
   - 点"工作区设置"组的"General"（同外框，焦点留在链接上）；`renameWorkspaceWith(page, slug, "Acme Labs")`：200，"Workspace settings"组含"Acme Labs"，"返回 Acme Labs"（同 w4 的断言）；点它：地址是 `/${slug}`，首页标题拿着焦点；
   - 再直接打开 `/settings/profile`（新的文档，没有来处）：工作区组是"Acme Labs"（最近的工作区，读了一次列表），"返回 Acme Labs"。
3. **"UI2 (page): at a phone's width the directory is the drawer, each link in it closing it with the page's title focused"**：`page.setViewportSize({ width: phoneWidth, height: 800 })`（`e2e/fixtures/narrow.ts:4`，同 UI1:40；审查 C 的 N9），打开"Notes"那一页；`openDrawer(page)`，点笔记本行的齿轮：抽屉关上，`settingsTitle(page, "Plans settings")` 拿着焦点（经到达进入）；细栏的位置文字是"Settings"；`openDrawer`：抽屉里第一个拿焦点的是"Back to Plans"；点"Notebook settings"组的"Members"：抽屉关上（`inert`），`settingsTitle(page, "Plans settings")` 拿着焦点（抽屉里 `arrived`）；再 `openDrawer`，点同一组的"Import, export and restore"：抽屉关上，`h1` 拿着焦点（到达之后再到达：没有按页的 `key` 时同一个 `h1` 不重挂、`useArrivalFocus` 的依赖不变，焦点落到 `body`；初稿这一格直接打开地址、arrival 由假变真，`key` 去掉也过，审查 C 的 I1）；`expectFits(page)`（`e2e/fixtures/narrow.ts:11`）；再 `openDrawer`，点"Back to Plans"："Notes"那一页的标题拿着焦点；再 `openDrawer`，打开抽屉底部的账户菜单（`accountMenu`）、选"Settings"：抽屉关上，`settingsTitle(page, "Profile")` 拿着焦点（到达）；再 `openDrawer`，账户菜单里再选"Settings"：地址不变，抽屉关上，细栏的菜单按钮（"Navigation"）拿着焦点（决定 816：同一个地址不算到达；带 `arrived` 时 `h1` 不重挂，焦点落到 `body`，审查 A 的 R1-M2 在 Chromium 里跑过）。不带 `@narrow`：文件进了 `threeBrowsers`，`webkit-narrow` 不收它（`check.mjs:63`），`webkit-narrow` 的必需格不变。

`e2e/playwright.config.ts:3-4` 的 `threeBrowsers` 加 `shell\/ui2-`；`tools/e2e-projects/check.mjs` 的 `REQUIRED` 加两行 `["firefox", "shell/ui2-settings-mode.spec.ts", "UI2 (page): the settings are entered"]`、`["webkit", …]`（`check.mjs:1-3`：正则写错不会让任何一格变红）。

### 5.4 S2：夹具与故事

- **`changeAccessWith`**（`e2e/fixtures/notebook-pages.ts:96-102`）改为：

  ```ts
  /** Chooses the access of the notebook id on its general page, as the page words it, which goes out as it is chosen (M13/P3/S2); resolves the answer. */
  export async function changeAccessWith(page: Page, id: string, access: string): Promise<Response> {
    const answer = answerTo(page, "PATCH", `/api/v0/notebooks/${id}`);
    await page.getByLabel("Workspace access", { exact: true }).selectOption({ label: access });
    return answer;
  }
  ```

  先挂 `answerTo` 再选（选了即发）。`selectOption` 发 `input` 与 `change`，没有按键，`useChoice` 当它是指针的选择、立即用（`native-select.tsx:92-100`）；它不聚焦下拉（审查 C 的三种引擎探针；vitest 里 user-event 的 `selectOptions` 会聚焦，所以初稿的探针看不出），要看焦点的格先 `focus()`。n3 的两处调用（`:76`、`:84`）不动；`:78` 的 `getByText("Saved.", { exact: true })` 照旧（精确匹配，✓ 不在文字里；常规页上名称一行的输出是空的，只有开放程度那一行写了它，照旧唯一）。S2 的提交 2。
- **n4**（`n4-notebook-members.spec.ts:164`）：`level: 2` 改为 `level: 1`（成员页的 `h2`"Members"去掉，焦点到 `h1`"Members"）。S2 的提交 1。
- **UI2**：第 1 格的"Plans settings""Workspace settings"改为"General"，第 2 格的"Plans settings"也改为"General"（照实：原来漏列），第 3 格的三个"Plans settings"依次改为"General""Members""Import, export and restore"（页名作 `h1`）。S2 的提交 1。
- **renameWorkspaceWith、renameNotebookWith**（`workspace-pages.ts:73-83`、`notebook-pages.ts:83-93`）不改：`fill` 之后才点"保存"，`renameForm` 找的是有"Name"标签的 `form`，`NameRow` 就是这样一个 `form`。
- **核对过、不改的**：lw12:162-163,210 与 `upgrade-pages.ts:36` 以 `section` 里的 `h2`"Mode"找模式一组，S2 的 `SettingsSection` 照旧是 `section` 与 `h2`，`:210` 的焦点目标照旧；w5:135（成员看不到"Invitations"）、n4:188（不是管理员看不到"Add a member"）照旧；`deleteWorkspaceWith`、`deleteNotebookWith`、`leaveNotebookWith`、`removeMemberWith`、`takeOverWith`、`changeNotebookRoleWith`、`addNotebookMemberWith` 以按钮与对话框的名字找，文字都不变；`ownerlessListed`、`membersListed`、`auditLog` 以列表的名字找，名字不变（4.2.3、4.2.4）。

### 5.5 S2：UI3（`e2e/stories/shell/ui3-instant-save.spec.ts`，两格，三种浏览器）

只有页面版本：写开放程度，照 n3 调 `expectNotebook`，接口与 MCP 版本由 N3 覆盖（00 号文档 4.8）。只用 `selectOption`（Linux 上的 WebKit 在关着的下拉上按方向键会立即生效，M12-polish 第 8 项；键盘逐项走与 Esc 的规则由 vitest 守）。造数：一个团队，笔记本"Plans"由 A 建（A 是管理员），B 是笔记本的另一个管理员（`addedNotebookMember`）。两格里 `access = page.getByLabel("Workspace access", { exact: true })`，`saved = page.locator("form", { has: access }).getByRole("status")`：只找开放程度那一行的输出（常规页上 `NameRow` 也有一个一直挂载的 `<output>`，`page.getByRole("status")` 不唯一，严格模式报错；审查 C 的 M4）。S2 的提交 2。

1. **"UI3 (page): the workspace access goes out as it is chosen, Saved. said with its check, which fades"**：A 打开常规页；`await access.focus()`（`selectOption` 不聚焦下拉，`goto` 之后焦点在 `body`；审查 C 的 I2）；`changeAccessWith(page, id, "Workspace can read")`：200，答复之后 `await expect(access).toBeFocused()`（`toBeFocused` 只判一次，写在答复之后：发送中禁用了下拉时，Chromium 在下一次渲染把焦点交给 `body`，变异 32 就红）；值是 `viewer`；说明是"The workspace's admins and members can read it; a guest, once added."；`saved` 里精确的"Saved."；`::before` 的 `content` 不是 `none`（`saved.evaluate` 读 `getComputedStyle(el, "::before")`）；`expect.poll` 透明度到 `"0"`（超时 5 秒），文字照旧"Saved."；`expectNotebook(db, …)`；再选"Workspace can edit"：200，先 `await expect(saved).toHaveText("Saved.")`（发出时已清空，等到的一定是这一次写回的；不先等，第一次 poll 可能读到空的那一态，它本来就是 1，审查 C 的 R1-N1），再 `expect.poll` 透明度回到 `"1"`（轮次换了，不论答复多快都重来，决定 841），之后又到 `"0"`。
2. **"UI3 (page): chosen after another admin made the account an editor, the access is refused: the list's again, and why"**：A 的页先 `holdStream(page)`（`e2e/fixtures/events.ts:214-222`，A 收不到降级的事件，常规页照旧是管理员的样子），打开常规页；B 在 `anotherPage` 里打开成员页，`changeNotebookRoleWith(other, A, "Editor")`：200；A `access.focus()`，选"Workspace can edit"：403；答复之后下拉的值回到 `none`、拿着焦点，`role=alert` 里是"Only the notebook's admins can change its workspace access."，`saved` 是空的；`pageWatch.expectConsole({ errors: [failedToLoad(403)] })`；数据库里的开放程度没变（`expectNotebook` 照建时的）；最后放开流。服务端对看得到笔记本、没有权限的成员答 403 `forbidden`（n6:42-49 断言过同样的答复）。

`threeBrowsers` 加 `shell\/ui3-`，`REQUIRED` 加 `["firefox", "shell/ui3-instant-save.spec.ts", "UI3 (page): the workspace access goes out"]` 与 webkit 一行。

### 5.6 375 px

UI2 第 3 格的 `expectFits` 是设置页在 375 px 的守卫（卡片行在窄屏上控件落到文字之下，探针 (c)；S1 里量的是旧样子的笔记本设置页，S2 之后是卡片）。个人资料页上从抽屉里的账户菜单再选"设置"的焦点（审查 A 的 M2）由 UI2 第 3 格在三种浏览器里核对（先经到达进入个人资料页，再选一次，菜单按钮拿着焦点），vitest 的窄屏格模拟 `inert` 守焦点与路由状态（7.1；jsdom 不认 `inert`，审查 A 的 R1-M2、C 的 R1-I1）。今天带 `@narrow` 的格（pg2、mc3、se1）与 `webkit-narrow` 的必需格不变；没有别的故事在 375 px 下碰设置。

## 6. 实施步骤

两个 Step 各在自己的 worktree 里，分支从 `m13-ui-refresh` 拉出。每个提交单独为绿：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`（决定 788）；改了行为、夹具或故事的提交另经它跑完整的 `CI=1 make e2e`（决定 795(b)），只败第 1 节那几格（同行同消息）。新的测试先写、在没改的树上看到它红，再改代码。动了 `tools/e2e-projects/check.mjs` 的提交，说明的作用域写上 `tools`。

### 6.1 S1

（决定 842：两个代码提交与一个文档提交。）

- [ ] **开工**：不另跑基线（决定 827）；提交 1 的 `make e2e` 兼作核对，第 1 节之外的格红了先停下汇报。
- [ ] **提交 1 `web, e2e, tools: the settings' mode, their directory in the column, with Back to where they were entered from (M13/P3/S1)`**：先写 7.1 的提交 1 各格（新的 `settings-origin.test.tsx`、`settings-directory.test.tsx`、`settings-page.test.tsx` 的 `SettingsPage` 几格，`workspace-layout.test.tsx` 的设置模式与左栏滚动几格，`trash-preview.test.tsx` 两格、`profile-page.test.tsx` 模拟 `inert` 的窄屏一格）、12 个文件里属于提交 1 的改法（7.1 的表）与 UI2，看到它们红（UI2 三格都红：没有目录）；4.1.1 表里"提交"一列有 1 的；5.2 的 8 个故事、a8 与 a15 的提交 1 部分、夹具；`threeBrowsers` 与 `REQUIRED`，`node tools/e2e-projects/check.mjs`；`make e2e`；绿。审查时按 4.1.2–4.1.6、4.1.11、4.1.12 的块看。
- [ ] **提交 2 `web, e2e: the account's settings in cards, the name saved as it is changed, appearance and language a page of their own (M13/P3/S1)`**：先写 7.1 的提交 2 各格（新的 `name-row.test.tsx`、`appearance-page.test.tsx`，`settings-page.test.tsx` 的 `SavedOutput` 几格，`profile-page.test.tsx` 的三格，`button.test.tsx:56` 的类名表），看到红；4.1.1 表里"提交"一列有 2 的（4.1.7–4.1.10、`Button` 的透明边框）；a8 与 a15 的提交 2 部分；`make e2e`；绿。
- [ ] 7.2 的 S1 各条变异；B 审查者截图之前，实施者先截 7.4 的 S1 几张看一遍。
- [ ] **提交 3 `docs: …(M13/P3/S1)`**：8.2 第 1–11 条；S1 的审查记录；本文 10.1 与照实改的各节；00 号文档第 12、13 节（2.1 的修订已随设计提交，决定 847）；01-P1 第 9 节交接处记下 `create-token-dialog.tsx:184` 的标签组已改（决定 846）；M12-polish 的"处理进展"S1 不写（S1 不认领它的条目）。

### 6.2 S2

- [ ] **提交 1 `web, e2e: the workspace's and the notebook's settings in cards, each page's name its title, the danger zone last (M13/P3/S2)`**：先改 7.1 的 S2 提交 1 各格（标题的级别与名字、`NameRow` 的"保存"、开放程度的"保存"按它的 `form` 找、等 `aria-busy` 消失、随页离开的链接、"基本信息"、`transfer-page.test.tsx:91,865`、`invitation.test.tsx:262`），看到红；4.2.1 表里"提交"一列有 1 的（4.2.2–4.2.4、4.2.6、4.2.7；开放程度这一提交里照旧是单选加"保存"，放进"基本信息"卡片里的一块）；n4:164、UI2 第 1、3 格的标题；`make e2e`；绿。单独为绿：名称换成 `NameRow` 之后没编辑时没有名称的"保存"，`notebook/general-page.test.tsx` 里以 `getAllByRole("button", { name: "Save" })[1]` 取开放程度的"保存"的几格（`:100`、`:185`、`:197`、`:221`，所在的格 `:88-104`、`:167-204`、`:206-223`）在这一提交里改成在开放程度的那个 `form` 里找（审查 C 的 M6）。
- [ ] **提交 2 `web, e2e, tools: the workspace access goes out as it is chosen, refused it shows the list's again, Escape takes a step back (M13/P3/S2)`**：先写 `general-page.test.tsx` 的开放程度各格、`native-select.test.tsx` 的 Esc 几格与 UI3，看到红；4.2.5（`AccessRow`、`useChoice` 的 Esc）；`accesses` 导出；`changeAccessWith`；`threeBrowsers` 与 `REQUIRED`；`make e2e`；绿。
- [ ] 7.2 的 S2 各条变异；7.4 的 S2 截图（含文件按钮的悬停、强制色下的 ✓ 与"保存"、长的只读值）。
- [ ] **提交 3 `docs: …(M13/P3/S2)`**：8.2 第 12–20 条；审查记录；本文 10.2；`docs/v0.2/handoffs/M12-polish.md` 的"处理进展"一行（第 1 项里的 19.2 做了，19.5、19.6、20.4、31.1 照旧留给 v0.2；决定 846，M5-polish 不动）；01-P1 第 9 节交接处记下 S2 的四处标签组与文件按钮的悬停已改；00 号文档第 12、13 节；Phase 完成的收尾项（8.1 第 4 条）。

## 7. 测试与验证

### 7.1 每项的测试

"（1）""（2）"是 S1 的哪个提交（决定 842）。经 `renderApp` 的格以 `act(() => router.navigate(…))` 或点链接导航；读来处的格读目录的"返回"与它的 `href`。修订的探针（第 1 节末）已照这些写法跑过其中十二格。

**S1，新文件**：

- `W/app/settings-origin.test.tsx`（1）：
  - 从 `/lab` 进入工作区设置，"返回 Lab"指 `/lab`；工作区设置 → 个人设置 → 笔记本设置，来处一直是 `/lab`；
  - 离开设置再从 `/lab/search?q=a` 进入，"返回 Lab"指 `/lab/search?q=a`（变异 3 的抓手）；
  - 标签页的第一个地址就是设置页时没有来处（回工作区首页）；`IndexTo` 的 `replace` 不改来处；
  - 以 `replace` 进入（决定 833）：未登录打开 `/settings/security`、登录（本设备最近的工作区是 Lab）之后"返回 Lab"指 `/lab`，不是登录页；引导的 `next` 是 `/settings/profile`，做完之后"返回"不指引导页；
  - 片段（决定 834）：来处是已登录的邀请页 `/invitations/inv-1#tok`："返回"（只是"Back"）指 `/invitations/inv-1#tok`，点了之后邀请页显示邀请（不是"找不到邀请"）、`h1` 拿着焦点（决定 835）；来处是 `…/pages/:id#h`：`href` 不带 `#h`；
  - 来处的工作区不在已读的列表里（打开 `/lab`、列表里没有 Lab，"找不到工作区"）："返回"指 `/`（审查 A 的 N3）；
  - 设置 → 别处 → 后退两条回到设置：来处是刚离开的那一页（照 4.1.3 写明的规则，审查 A 的 N2）。
- `W/app/settings-directory.test.tsx`：
  - 工作区设置（1）：两个 `nav`，依次是"Settings""Workspace settings"，组标题的名字在 `bdi` 里、组标题的 `textContent` 是"Workspace Lab"（有空格，审查 A 的 N1），管理员才有"Ownerless notebooks"；没有 `searchbox`、没有切换器；细栏（`matchMedia` 桩成窄屏）的位置文字是"Settings"；
  - 笔记本设置（1）：三组；笔记本改名（store 更新）之后组标题即时变；不在列表里的笔记本的设置（404）只有两组；树在：`findByRole("navigation", { name: "Pages of Plans", hidden: true })` 找得到，不带 `hidden: true` 找不到；
  - 个人设置（1）：从 Lab 进入、本设备最近的工作区已是别的标签页里的 Acme（列表有两个）：组是 Lab、"返回 Lab"（决定 837）；没有来处：最近的工作区在列表里时是它的组、"返回 Lab"，不在时没有这一组、"返回"指 `/`；没有来处、也没有最近的工作区：`GET /api/v0/workspaces` 一次也不读（决定 843）；从 `/lab`（列表已读）进入，先 `vi.advanceTimersByTimeAsync(2_000)` 越过 SWR 的去重（同 `workspace-layout.test.tsx:133-146`），再进个人设置，读的次数不变（审查 C 的 M2）；
  - "返回"的去处（决定 813、834、835）（1）：来处是列出的笔记本的页；来处的笔记本已从列表里删掉（回工作区首页、"返回 Lab"）；来处是 `/create-workspace`（"返回"、回它）；来处是变更集页（`W/test/history-server.ts` 的假服务端）："返回 Plans"之后变更集页的 `h1` 拿着焦点；没有来处；
  - 焦点（1）：同外框的链接点了焦点留在链接上；跨外框的链接、"返回"之后去向的 `h1` 拿着焦点；窄屏：从笔记本首页经抽屉里的齿轮进入（到达），再在抽屉里点"Members"，抽屉关上、`h1` 拿着焦点，再打开抽屉点"Import, export and restore"，`h1` 拿着焦点（到达之后再到达，变异 11 的抓手；审查 C 的 I1）；
  - 中文（1）：组标题"个人""工作区 Lab""笔记本 Plans"、"返回 Lab"、没有来处与工作区时单独的"返回"（审查 C 的 N6）、细栏"设置"（13.2 第 4 条）；（2）`h1`"外观与语言"；
  - `IndexTo`（1）：以 `arrived` 打开 `/lab/settings`，常规页的 `h1` 拿着焦点；不带状态时不拿。
- `W/app/settings-page.test.tsx`：（1）`SettingsPage` 的 `h1` 到达时拿焦点、不是到达时不拿；`useSettingsHeading` 在 `SettingsPage` 之外抛错。（2）`SavedOutput` 的文字精确是"Saved."或空、带 `nw-saved`；`saved` 依次是假、真、假、真、假、假、真、真时 `data-round` 是 `0 1 1 0 0 0 1 1`：每次写出时换，清空与一直显示时不换（决定 841，修订的探针 R10）。
- `W/app/name-row.test.tsx`（2，经个人资料页，假服务端答 `PATCH /api/v0/me`，可扣住）：没编辑时没有"保存"、回车不发；编辑回原样时"保存"消失；输入框之后那个放"保存"的槽有没有"保存"都在（审查 B 的 M6）；"保存"发出去空白的名字；扣住时"保存"是 `aria-disabled="true"` 与 `aria-busy="true"`、没有 `disabled`，再点、再回车都不发（变异 22 的抓手）；答复之后"保存"消失、焦点在输入框、"Saved."；扣住时又编辑的保留草稿、不显示"Saved."；422 的问题在输入框下、`aria-describedby` 只有问题的 id、焦点在输入框，改好之后指说明；清空、回车出了"必填"之后把原名打回去：没有 `aria-invalid`、`aria-describedby` 不指问题、没有"保存"、没有横幅（审查 A 的 M1）；只差空白的不发请求、"Saved."、字段回到当前的名字；没编辑时别处改了名字，字段跟上。
- `W/pages/settings/appearance-page.test.tsx`（2）：`profile-page.test.tsx:197-217` 挪过来（选"Dark"即生效、选"简体中文"之后 `h1` 是"外观与语言"、账户菜单的"外观"显示同样的选择）；两个 `radiogroup` 名为"Theme""Language"，主题一组的描述是"Kept in this browser."。

**S1，另加的格**：

- `W/pages/notebook/trash-preview.test.tsx`（1）：以 `arrived` 打开一项的预览，它的 `h1` 拿着焦点（决定 835）；以 `arrived` 打开一项已不在回收站的预览（`restoreElsewhere`，同 `:218-233`），`Gone` 的 `h1`"Trash"拿着焦点（`trash-preview.tsx:579` 的那一处，审查 C 的 R1-N2）。
- `W/pages/settings/profile-page.test.tsx`（1）：窄屏（`matchMedia` 桩），模拟 `inert`：以 `vi.spyOn(HTMLElement.prototype, "focus").mockImplementation(…)` 打桩，`[inert]` 之下的元素不取焦点（`vite.config.ts` 的 `restoreMocks: true` 在下一格之前复原它，`unstubGlobals: true` 复原 `matchMedia`；直接改原型会漏到后面的格，审查 C 的 R2-N4）（jsdom 不认 `inert`，Radix 在 `setTimeout(0)` 里把焦点还给关上的抽屉里的账户按钮，`shell-drawer.test.tsx:10-14` 说的就是它；审查 A 的 R1-M2、C 的 R1-I1）。在 `/settings/profile` 打开抽屉、账户菜单、选"Settings"：抽屉关上（`[data-shell]` 带 `inert`），50 ms 之后焦点在细栏的菜单按钮，`router.state.location.state` 是 `null`（没有 `arrived`，决定 816）。
- `W/components/ui/button.test.tsx`（2）：`:56` 的"each variant has its look"表里 `default` 加 `border`、`border-transparent`（决定 845，审查 C 的 R1-N3）。
- `W/pages/workspace/workspace-layout.test.tsx`（1）：进入设置前后 `[data-shell]` 是同一个元素、没有 `searchbox`；设置模式里 Mod+K 不被 `preventDefault`；`scrollTop` 与 `getBoundingClientRect` 打桩（修订的探针 R12 的写法）：页上左栏滚到 300、点齿轮，设置里是 0；"返回"之后先还成 300，树再把当前行带进来（`scrollTop` 依次被设成 300 与 738）。
- `W/pages/settings/security-page.test.tsx`（2）：危险操作一组名为"Danger zone"（中文"危险操作"），停用账户的按钮在里面（审查 C 的 N6）。

**S1，改的 12 个文件**（初稿的分离工作树里全量 vitest 变红的 28 格里 26 格在这里，另 2 格是 S2 的开放程度；修订的工作树里同样这 28 格）：

| 文件 | 格（行） | 提交 | 改法 |
|---|---|---|---|
| `W/app/layout.test.tsx` | `:164-176`（设置页的"回到工作区"） | 1 | 改写成目录的"返回"：`/settings/security` 上没有右上角、账户按钮在 `[data-shell]` 里；"返回 Lab"回 `/lab`、`h1` 拿着焦点 |
| | `:249-260`（中文的"外观"与"回到工作区"） | 1 | 后半的地址换成 `/create-workspace`（账户有工作区），找"回到工作区" |
| | `:52-56`（"没有外框的页"） | 1 | 地址换成 `/create-workspace`（现在照旧过，但设置页已有外框，格名不再对） |
| | `:201-226`（会话启动中右上角什么也没有；守卫与右上角同一个键、只读一次）、`:237-247`（没有左栏的路由上会话不可用） | 1 | 地址换成 `/create-workspace`：`/settings` 有了 `sidebar` 之后，会话启动中右上角在个人设置上根本不挂载，两格照绿却不再测它们说的事（审查 C 的 I5） |
| `W/app/user-menu.test.tsx` | `:160-178` 的后半、`:180`（`test.each`） | 1 | 右上角的那一半换成 `/create-workspace`：设置页的账户在左栏里，不换就少了右上角的覆盖 |
| `W/pages/settings/profile-page.test.tsx` | `:28-35` | 1 | `h1`"Profile"；目录"Settings"组里"Profile"当前 |
| | `:47-68` | 1 | 标题是 `h1`"Profile"；"在个人资料页上再选"的按钮是左栏底部的 |
| | `:70-88` | — | 不改（决定 832 让它照旧绿） |
| | `:90-107` | 1 | 改写：安全页上左栏底部的账户菜单选"Settings"，个人资料的 `h1` 拿着焦点；再开菜单按 Esc，焦点回到那个按钮 |
| | `:130-162`、`:164-173` | 2 | `NameRow`：编辑之后才找"保存"；`:155` 等"保存"的 `aria-busy` 消失，不等 `disabled` 为假（换成 `aria-disabled` 之后那一句立刻过，接着的点击会撞上发送中的守卫；审查 C 的 M5）；"没改的名字"一格改成"没编辑时没有保存，回车不发" |
| | `:197-217` | 1 | `:208` 的 `h1`"设置"改为 `h1`"个人资料"（主题与语言的单选还在个人资料页上；同 a8 提交 1 的 `:78`；审查 C 的 R1-M1） |
| | `:197-217` | 2 | 挪到 `appearance-page.test.tsx` |
| `W/pages/create-workspace.test.tsx` | `:148` | 1 | `"Settings"` 改 `"Profile"` |
| `W/pages/invitation.test.tsx` | `:246` | 1 | 同上 |
| `W/pages/workspace/workspace-home.test.tsx` | `:131` | 1 | 同上 |
| `W/pages/workspace/workspace-layout.test.tsx` | `:111-116` | 1 | `["Settings", "/acme/settings/general", null]`（新格见上） |
| `W/pages/workspace/general-page.test.tsx` | `:52-60` | 1 | `h2`"General"照旧（S1），`h1`"Workspace settings"；目录"Workspace settings"组的"General"当前，没有名为"Acme"的 `nav` |
| | `:62-83` | 1 | 切换器改读目录：组标题含"Acme Labs"，"返回 Acme Labs" |
| `W/pages/workspace/ownerless-page.test.tsx` | `:216-219` | 1 | 挪到"Open it"之后（`:222-224`），在笔记本首页读名为"Lab"的 `nav` |
| | `:228-246` | 1 | 往 `document.body` 上挂的一个 `input` 里打字（页面没有别的输入框，2.1 第 11 条），断言同今天；这一格结束时自己移掉它（testing-library 的清理只卸下 React 的容器，审查 C 的 N8） |
| `W/pages/notebook/general-page.test.tsx` | `:23`（`nav()`）与 `:25-42` | 1 | 从笔记本首页点齿轮"Settings of Plans"进入：`h1` 拿着焦点、目录"Notebook settings"组的"General"当前、没有名为"Lab"的 `nav` |
| | `:44-62` | 1 | 改名之后读目录的笔记本组："Roadmap" |
| | `:88-104` | 1 | 从首页经齿轮进入；改开放程度、"保存"（S1 照旧）；"返回 Plans"之后在首页读"团队"组 |
| | `:131-145` | 1 | 删除被拒之后目录的笔记本组照旧"Plans" |
| | `:385-388` | 1 | `findByRole("navigation", { name: "Pages of Plans", hidden: true })` 与 `findByRole("link", { name: "Notes", hidden: true })` |
| `W/pages/notebook/members-page.test.tsx` | `:14`（`nav()`）、`:203-216`、`:224-246` | 1 | 从首页经齿轮、目录的"Members"进入；之后"返回 Plans"在首页读分组 |
| | `:399` | 1 | 目录的笔记本组照旧"Plans" |
| | `:459-461` | 1 | "返回 Lab" → 名为"Lab"的 `nav` 里的"Atlas" → 齿轮"Settings of Atlas" → 目录"Notebook settings"一组里的"Members"（目录的工作区组也有"Members"，不限定在组里 `findByRole` 会因不唯一报错；审查 C 的 N7） |
| `W/pages/notebook/transfer-background.test.tsx` | `:482,611,642,690` | 1 | `{ hidden: true }`（探针里这 13 格就此全绿） |

**S2**：

- `W/pages/notebook/general-page.test.tsx`：
  - 提交 1：`:31,33,145` 的标题（`h1`"General"，没有 `h2`"General"）；第一组的 `h2` 是"Basics"（中文"基本信息"）；改名各格照 `NameRow`；开放程度的"保存"在它的 `form` 里找（`:100`、`:185`、`:197`、`:221`，审查 C 的 M6）；`:188` 等"保存"的 `aria-busy` 消失，不等 `disabled`（审查 C 的 M5）；升级进行中模式一行的"View the upgrade"带 `arrived`：点了之后升级页的 `h1` 拿着焦点；同一页顶上升级横幅里的"View the upgrade"（`within` 横幅的 `status` 找）同样（决定 839，审查 B 的 R1-N3）。
  - 提交 2：`:167-204`"保存发出期间选的"拆成名称一格与开放程度一格（扣住答复时下拉显示选的那一项）；`:206-223`"没动的表单跟上别人的改动"：名称照旧，开放程度是下拉的值跟上、没有"保存"；开放程度新格：指针选了即发一次；非 macOS 下（`onMac` 桩成假），照 `native-select.test.tsx:37-38` 以 `fireEvent.keyDown`（`ArrowDown`）加 `fireEvent.change` 走一步（user-event 的 `{ArrowDown}` 在 jsdom 里不改 `<select>` 的值，审查 C 的 M3）：先断言显示的值与说明都换成了新的一项、下拉没有 `aria-busy`（同步的见证：发出时草稿同步设上，`aria-busy` 随之出现；假服务端要到 `await request.clone().json()` 之后才记下 `PATCH`，紧接着数是空的，审查 C 的 R1-M2），再等 20 ms（`await act(() => new Promise((r) => setTimeout(r, 20)))`）断言没有发出；之后失焦发一次、回车发一次；走一步之后按 Esc：值与说明回到原来的、`keydown` 被 `preventDefault`，失焦也不发；没有待用的选择时按 Esc 不被 `preventDefault`（决定 840）；扣住时下拉显示新值、说明随之变、带 `aria-busy`；答复之后"Saved."，下一次发出时先变空，再写出时 `data-round` 换了；被拒（403）退回原值、`role=alert` 是"Only the notebook's admins can change its workspace access."、焦点在下拉上、没有"Saved."；连选两次、第一次被拒在第二次发出之后才回来：没有横幅、显示第二次的值；第一次的成功在第二次之后回来：草稿不清、不显示"Saved."，第一次答复之后、第二次答复之前下拉仍带 `aria-busy`，第二次答复之后没有（`inTurn` 让两次依次发出，假服务端分别扣住；审查 B 的 R1-N2）；发送中下拉没有 `disabled`；中文的拒绝文字。
- `W/components/ui/native-select.test.tsx`（提交 2）：`useChoice` 走一步之后 Esc 退回、`apply` 不调用、`preventDefault`；没有待用的选择时 Esc 不处理（决定 840）；下拉包在一个带 `onKeyDown` 的父元素里：有待用的选择时父元素收不到 Esc，没有时收得到（`stopPropagation`，审查 C 的 R1-N3）。
- `W/pages/workspace/general-page.test.tsx`（提交 1）：`:55` `h1`"General"；第一组的 `h2`"Basics"；`:62-83`、`:85-118`、`:120-169`、`:235` 起照 `NameRow`（"保存它已有的名字不发请求"改成"没有保存"）；`:111` 等 `aria-busy` 消失（审查 C 的 M5）；删除各格只换了按钮所在的组，名字不变；危险操作一组中文是"危险操作"。
- `W/pages/invitation.test.tsx:262`（提交 1）：找"保存"按钮改为找值是"Lab"的"Name"输入框（常规页的名称一行）。
- `W/pages/workspace/ownerless-page.test.tsx`（提交 1）：`:109` `level: 2` 改 `level: 1`；接管之后的"Open it"带 `arrived`：点了之后笔记本首页的 `h1` 拿着焦点（决定 839）。`W/pages/workspace/workspace-home.test.tsx:153` 同改级别。
- `W/pages/workspace/members-page.test.tsx:257`、`W/pages/notebook/members-page.test.tsx:213,376`（提交 1）：按名字找的"Members"现在是 `h1`，加 `level: 1` 写明；笔记本成员页 `NoAdmin` 的链接带 `arrived`：点了之后无主笔记本页的 `h1` 拿着焦点（决定 839）。
- `W/pages/notebook/upgrade-banner.test.tsx`：不改（`:52` 只读链接的 `href`）。
- `W/pages/notebook/transfer-page.test.tsx`（提交 1）：`:91`："Started by Bob"改"By Bob"；`:865`：改读目录的笔记本组含"Atlas"、`h1`"Import, export and restore"；任务行的"See in the history"带 `arrived`：点了之后变更集页的 `h1` 拿着焦点（决定 835、839）。`W/pages/notebook/transfer-background.test.tsx:137-147`（提交 1）之后：点撤销任务一行的"The change undone"，变更集页的 `h1` 拿着焦点（决定 839 的第五个链接）。
- 不改的：`notebook/general-page.test.tsx:148-165`（成员看到的开放程度一行：`accessAdminsOnly` 那一句自成一个元素，`getByText` 的精确匹配照旧，4.2.4；审查 C 的 R1-N4）、`transfer-page.test.tsx:63`、`transfer-import.test.tsx:46-53,101`、`upgrade-read-only.test.tsx:506`、`ownerless-page.test.tsx:511`、`invitations-section.test.tsx:110-112,297`、`general-page.test.tsx:625,658`（升级的"Started by"是另一个键）。

### 7.2 变异

每条在本机改坏、跑它旁边的测试、看到失败，再从备份复原（02-workflow 3.1；先在没改的树上跑一遍同样的命令）。原编号不变（第 5、7 节以编号引用），审查之后改写的注明，新加的从 45 起。修订时在草稿上改坏、以探针（第 1 节末的 R1–R12、Q1–Q6）看到失败的，在"抓住它的测试"里写了探针号：3、33、46、47、49、50（邀请页与回收站的 `Gone`）、51–56、58（横幅）、59–61、63。

| # | Step | 改坏 | 抓住它的测试 |
|---|---|---|---|
| 1 | S1 | `useOriginRecord` 不记来处（总是 `undefined`） | `settings-origin.test.tsx`；`settings-directory.test.tsx` 的"返回"各格；UI2 第 1 格（"返回 Plans"） |
| 2 | S1 | 设置页之间不保留来处（`seen.settings ? seen.origin : …` 换成总记上一页） | `settings-origin.test.tsx` 跨外框一格；UI2 第 1 格（个人资料上的"返回 Plans"） |
| 3 | S1 | 不在设置里时不调 `setSeen`（来处停在第一次进入时的那一页；改写，审查 C 的 M1） | `settings-origin.test.tsx` 从 `/lab/search?q=a` 再进入一格（R5） |
| 4 | S1 | "返回"不带 `arrived` | `settings-directory.test.tsx` 焦点格；UI2 第 1、2 格 |
| 5 | S1 | 目录里跨外框的链接不带 `arrived` | `settings-directory.test.tsx`；UI2 第 1 格（个人资料的 `h1`） |
| 6 | S1 | 目录里同外框的链接不经 `useFollow`、总带 `arrived` | `settings-directory.test.tsx`（焦点留在链接上）；UI2 第 1 格 |
| 7 | S1 | 抽屉里同外框的链接不带 `arrived`（`useFollow` 换成固定的 `opened`） | `settings-directory.test.tsx` 窄屏格；UI2 第 3 格 |
| 8 | S1 | `/settings` 不带 `sidebar` | `layout.test.tsx` 改写的那一格（右上角在） |
| 9 | S1 | `WorkspaceLayout` 不看设置模式 | `workspace-layout.test.tsx` 新格；`general-page.test.tsx`（两处）；w4、n6、UI2 |
| 10 | S1 | `IndexTo` 不转交状态 | `settings-directory.test.tsx` 的 `IndexTo` 格 |
| 11 | S1 | `SettingsPage` 不按页挂载（去掉 `key`） | `settings-directory.test.tsx` 窄屏格（经齿轮到达之后，抽屉里第二个链接的 `h1` 不取焦点）；UI2 第 3 格（同样的走法；审查 C 的 I1） |
| 12 | S1 | 目录的"Workspace settings"一组改名（"Workspace"；审查 C 的 M9 分成两行，笔记本一组见 45） | n9:187；`workspace/members-page.test.tsx:144`、`ownerless-page.test.tsx:138,190` |
| 13 | S1 | 最近的工作区不对照列表（拿 slug 直接画组） | `settings-directory.test.tsx`"不在列表里" |
| 14 | S1 | 列表读过也再读（`useSWR` 的键不看 `list`） | `settings-directory.test.tsx` 的读取计数：先 `vi.advanceTimersByTimeAsync(2_000)` 越过 SWR 的去重（审查 C 的 M2） |
| 15 | S1 | 设置模式里不画 `hidden` 的 `SlotHolder` | `transfer-background.test.tsx` 13 格、`general-page.test.tsx:385` 5 格（等不到树） |
| 16 | S1 | `hidden` 去掉（树显示在目录之后） | `settings-directory.test.tsx`（设置模式里没有名为"Pages of Plans"的可见 `nav`）；UI2 第 1 格（同一句，审查 C 的 M9；lw12 在设置页上已不读树） |
| 17 | S1 | 设置模式里 Mod+K 照旧 `preventDefault`（`field` 指到别处） | `workspace-layout.test.tsx` 新格 |
| 18 | S1 | `Layout` 不认待办的引导步骤（`bounced` 总是假） | `profile-page.test.tsx:70-88` |
| 19 | S1 | ✓ 写进文字（"✓ Saved."） | `settings-page.test.tsx`；`name-row.test.tsx`；n6:79、w4:115、a8:69 的精确匹配 |
| 20 | S1 | "保存"一直显示 | `name-row.test.tsx`（没编辑时没有"保存"） |
| 21 | S1 | 保存之后焦点不回输入框（去掉 `useFocusLeaving`） | `name-row.test.tsx`；a8 的 `toBeFocused` |
| 22 | S1 | 发送中用 `disabled` 代替 `aria-disabled` | `name-row.test.tsx` 扣住答复的那一格（有 `aria-disabled`、没有 `disabled`；a8 的 `toBeFocused` 看时序，不算；审查 C 的 M9） |
| 23 | S1 | `aria-describedby` 写成说明与问题两个 id | a8 的 `noteOf`（`:64`）；`name-row.test.tsx` |
| 24 | S1 | 外观页的单选不即生效（`onChange` 去掉） | `appearance-page.test.tsx`；a8 |
| 25 | S1 | 组标题的名字不放 `bdi` | `settings-directory.test.tsx` |
| 26 | S1 | 齿轮不带 `arrived` | `general-page.test.tsx:25-42` 的焦点；UI2 第 1 格 |
| 27 | S1 | 工作区导航的"设置"不带 `arrived` | `workspace-layout.test.tsx` 新格；UI2 第 1 格 |
| 28 | S1 | 撤回令牌之后焦点不到 `h1`（`useSettingsHeading` 换回一个不挂的 ref） | `tokens-page.test.tsx:259` |
| 29 | S1 | `threeBrowsers` 漏写 `ui2-` | `node tools/e2e-projects/check.mjs` |
| 30 | S1 | ✓ 的淡出去掉 | `settings-page.test.tsx` 只核对类名，抓不住；S2 的 UI3 第 1 格抓（S1 里由 7.4 的截图看） |
| 31 | S2 | 开放程度被拒不退回（`catch` 里不清草稿） | `general-page.test.tsx` 被拒格；UI3 第 2 格（00 号文档 4.8 的最低清单） |
| 32 | S2 | 发送中禁用下拉 | `general-page.test.tsx`（没有 `disabled`）；UI3 第 1 格（先 `focus()`，答复之后下拉拿着焦点；审查 C 的 I2） |
| 33 | S2 | 绕过 `useChoice`（`onChange` 直接发） | `general-page.test.tsx` 走一步的格：`fireEvent.keyDown` 加 `fireEvent.change`，先断言显示的值变了（审查 C 的 M3）、下拉没有 `aria-busy`（同步的见证；紧接着数 `PATCH` 是空检查，假服务端在 `await request.clone().json()` 之后才记下，审查 C 的 R1-M2），再等 20 ms 断言没有发出（Q2） |
| 34 | S2 | 不持草稿（`shown` 只读列表） | `general-page.test.tsx` 扣住格 |
| 35 | S2 | 前一次的成功清掉草稿（不比 `sent`） | `general-page.test.tsx` 连选两次格 |
| 36 | S2 | 前一次的拒绝显示横幅 | `general-page.test.tsx` 连选两次格 |
| 37 | S2 | 发出时不先清空"Saved." | `general-page.test.tsx`（下一次发出时先变空）；UI3 第 1 格（`saved` 一直真、轮次不换，第二次选时透明度不回到 1） |
| 38 | S2 | ✓ 的淡出去掉 | UI3 第 1 格（透明度到 0） |
| 39 | S2 | 列表删除之后焦点不到 `h1` | `notebook/members-page.test.tsx:213,376`；n4:164 |
| 40 | S2 | `changeAccessWith` 留着单选 | n3（找不到"Workspace access"那个下拉）、UI3 |
| 41 | S2 | `transfer.startedBy` 照旧 | `transfer-page.test.tsx:91` |
| 42 | S2 | "模式"的 `h2` 去掉 `tabIndex` | lw12:210；`general-page.test.tsx` 降级之后的焦点格 |
| 43 | S2 | 403 不经 `texts` 说成通用的"forbidden" | `general-page.test.tsx` 被拒格；UI3 第 2 格 |
| 44 | S2 | `threeBrowsers` 漏写 `ui3-` | `check.mjs` |
| 45 | S1 | 目录的"Notebook settings"一组改名（"Notebook"；从 12 分出，审查 C 的 M9） | tr1:215、tr2:202；`transfer-page.test.tsx:55` |
| 46 | S1 | 以 `replace` 进入也记来处（决定 833） | `settings-origin.test.tsx` 登录与引导两格（R1、R2） |
| 47 | S1 | 工作区之外的来处不带片段（决定 834） | `settings-origin.test.tsx` 邀请一格（R3：`href` 少了 `#tok`，邀请页说"找不到邀请"） |
| 48 | S1 | 工作区里的来处也带片段（决定 834） | `settings-origin.test.tsx` `#h` 一格 |
| 49 | S1 | 来处的工作区不在已读的列表里时也回来处（审查 A 的 N3） | `settings-origin.test.tsx` 404 一格（R4） |
| 50 | S1 | 三页的 `h1` 照旧是 `useRef`（决定 835；邀请页、变更集页、回收站的预览两处各改一次） | `settings-origin.test.tsx` 邀请一格（R3）；`settings-directory.test.tsx` 变更集一格；`trash-preview.test.tsx` 的两个新格（`Preview` 的与 `Gone` 的，Q5；审查 C 的 R1-N2）；hi5 的 `toBeFocused` |
| 51 | S1 | 个人设置的工作区组先取最近的工作区（决定 837） | `settings-directory.test.tsx` Lab 与 Acme 一格（R6） |
| 52 | S1 | 读列表的键不看 `wanted`（决定 843） | `settings-directory.test.tsx` 一次也不读一格（R7） |
| 53 | S1 | 问题与 `aria-invalid` 不看 `changed`（审查 A 的 M1） | `name-row.test.tsx` 把原名打回去一格（R11） |
| 54 | S1 | `ScrollKept` 不还原（审查 A 的 M3） | `workspace-layout.test.tsx` 滚动一格（R12：返回之后 `scrollTop` 没有先被设成 300） |
| 55 | S1 | 账户菜单的"个人设置"在个人资料页上也带 `arrived`（审查 A 的 M2） | `profile-page.test.tsx` 窄屏一格（模拟 `inert`：焦点在 `h1` 不在菜单按钮，路由状态有 `arrived`，Q1；不模拟时两种都是账户按钮，分不出，审查 A 的 R1-M2、C 的 R1-I1）；UI2 第 3 格（经到达进入之后再选一次，焦点落到 `body`） |
| 56 | S1 | `SavedOutput` 的轮次不换（决定 841） | `settings-page.test.tsx` 轮次一格（R10）；S2 的 UI3 第 1 格比较两次写出的 `animationName`（`dd6ef368`；原写的透明度抓不住：两次写出之间隔着一次网络往返，空的一态总会画出，淡出不靠轮次也重来，审查 C 的 P3S2C-M1） |
| 57 | S1 | 组标题与名字之间不留空格（审查 A 的 N1） | `settings-directory.test.tsx` 的 `textContent`"Workspace Lab" |
| 58 | S2 | 随页离开的链接不带 `arrived`（决定 839；六处各改一次） | `general-page.test.tsx`"View the upgrade"（模式一行的与横幅里的，后者 Q6）；`ownerless-page.test.tsx`"Open it"；`notebook/members-page.test.tsx` 的 `NoAdmin`；`transfer-page.test.tsx`"See in the history"；`transfer-background.test.tsx`"The change undone" |
| 59 | S2 | `useChoice` 不处理 Esc（决定 840） | `native-select.test.tsx` 的 Esc 格；`general-page.test.tsx` 的 Esc 格（R8） |
| 60 | S2 | 说明取列表的值，不取 `choice.value`（决定 840，审查 B 的 M5） | `general-page.test.tsx` 走一步的格（R8） |
| 61 | S2 | `useChoice` 的 Esc 不 `stopPropagation`（决定 840） | `native-select.test.tsx` 父元素一格（Q4；审查 C 的 R1-N3） |
| 62 | S1 | `Button` 的 `default` 变体去掉透明边框（决定 845） | `button.test.tsx:56` 的类名表（审查 C 的 R1-N3）；强制色下的样子由 7.4 的截图看 |
| 63 | S2 | 开放程度的 `aria-busy` 取 `useForm` 的 `sending`（审查 B 的 R1-N2） | `general-page.test.tsx` 连选两次一格（第二次在途时没有 `aria-busy`，Q3） |
| t1 | S1 | `NameRow` 的横幅不看 `changed` | `name-row.test.tsx` 拒绝一格 |
| t2 | S1 | 不数编辑次数 | `name-row.test.tsx` 一格、`profile-page.test.tsx` 一格 |
| t3 | S1 | 不 `trim` 就比较 | `name-row.test.tsx` 只差空白一格 |
| t4 | S1 | 保存之后不清草稿 | `name-row.test.tsx` 两格 |
| t5 | S1 | `SettingsRow` 不画 `value` | `profile-page.test.tsx` 一格 |
| t6 | S1 | 目录里没有"外观与语言" | `settings-directory.test.tsx` 两格、`appearance-page.test.tsx` 一格 |
| t7 | S1 | "危险操作"一组没有标题 | `security-page.test.tsx` 一格 |
| t8 | S1 | 笔记本不在了也回来处 | `settings-directory.test.tsx` 一格 |
| t9 | S1 | `SettingsPage` 的 `h1` 去掉 ref | `settings-page.test.tsx` 两格 |
| t10 | S1 | `NavItem` 当前一行去掉透明边框（决定 845） | 没有格抓，由强制色的截图看（7.4） |
| t11 | S1 | `notebook-slot.tsx` 去掉焦点守卫 | 等价（表后"不列的"） |
| t12 | S1 | `Layout` 给 `SettingsOrigin` 的是 `undefined` | `settings-origin.test.tsx` 五格、`settings-directory.test.tsx` 五格 |
| t13 | S1 | 令牌为空的文字去掉 | `tokens-page.test.tsx` 三格 |
| t14b | S1 | 外观页撤回 a8 的修复（`276c888a` 上：页不先读主题与语言） | `appearance-page.test.tsx` 选了即生效一格；`2979f1c2` 起由 67 守（t14 在渲染函数里重读 store，写法等价，作废） |
| 64 | S1 | `NameRow` 发送中不拦提交（守卫只看 `changed`） | `name-row.test.tsx` 扣住一格（放开之后再数：发了 3 次；审查 A 的 P3S1A-N1、C 的 P3S1C-M1） |
| 65 | S1 | 变更集页的 `useArrivalFocus` 回到读数据的外层（`heading` 经 props 交给 `ChangesetRead`） | `changeset-page.test.tsx` 到达尚未读取的变更集一格（审查 A 的 P3S1A-M1） |
| 66 | S1 | 工作区列表没读到时"返回"也回 `/` | `settings-origin.test.tsx` 列表答 503 一格（审查 A 的 P3S1A-N2） |
| 67 | S1 | `SettingsRow` 不是 `observer`（决定 849） | `settings-page.test.tsx` 跟着 store 一格；`appearance-page.test.tsx` 选了即生效一格 |
| 68 | S1 | `ScrollKept` 挪到 `nav` 之前 | `workspace-layout.test.tsx` 滚动一格（看不见树时滚不动：438，应为 738） |
| 69 | S1 | `ScrollKept` 不移掉监听 | `workspace-layout.test.tsx` 滚动一格（值变了时异步发 `scroll`：0，应为 300；修复核对 C 的 P3S1C1-M1） |
| 69b | S1 | 设置模式里也监听 | 同上 |
| t1 | S2 | 卡片的 `ul` 去掉 `aria-label` | `ownerless-page.test.tsx` 四格 |
| t2 | S2 | 卡片的 `ol` 去掉 `aria-label` | `ownerless-page.test.tsx` 审计各格 |
| t3 | S2 | `ordered` 也画成 `ul` | 同上 |
| t4 | S2 | 工作区成员页移出之后焦点不到 `h1` | `workspace/members-page.test.tsx` 两格 |
| t5 | S2 | 无主笔记本页删除之后焦点不到 `h1` | `ownerless-page.test.tsx` 两格 |
| t6 | S2 | 邀请组没有 `heading` | `invitations-section.test.tsx` 撤回两格 |
| t7 | S2 | 最近的任务一组没有 `heading` | `transfer-page.test.tsx` 三格 |
| t8 | S2 | 工作区常规页没有"基本信息" | `workspace/general-page.test.tsx` 成员、访客与分组各格 |
| t9 | S2 | 工作区"危险操作"一组没有标题 | `workspace/general-page.test.tsx` 分组两格 |
| t10 | S2 | 笔记本"危险操作"一组的标题错 | `notebook/general-page.test.tsx` 分组两格 |
| t11 | S2 | 中文的"基本信息"写成英文 | 两个常规页的中文分组格 |
| t12 | S2 | 只读的开放程度没有"只有管理员能改"一句 | `notebook/general-page.test.tsx` 编辑者、读者两格 |
| t13 | S2 | 只读的开放程度没有值 | 同上 |
| t14 | S2 | Esc 不看有没有待用的选择 | `native-select.test.tsx` 两格 |
| t15 | S2 | 笔记本设置布局的标题换回"X settings" | `notebook/members-page.test.tsx` 两格 |
| t16 | S2 | 工作区设置布局的标题换回"Workspace settings" | `workspace/members-page.test.tsx` 等三格 |
| t17 | S2 | 工作区名称不做本地检查 | `workspace/general-page.test.tsx` 问题一格 |
| t18 | S2 | 笔记本名称没有 `fieldTexts` | `notebook/general-page.test.tsx` 问题一格 |
| t19 | S2 | 工作区的离开不在"危险操作"组里 | `workspace/members-page.test.tsx` 分组两格（`397241c7` 起；之前没有格抓） |
| t20 | S2 | `sent` 不计数 | `notebook/general-page.test.tsx` 连选两次两格 |
| t21 | S2 | 笔记本的离开不在"危险操作"组里 | `notebook/members-page.test.tsx` 分组两格 |
| t22 | S2 | 审计日志一组没有标题 | `ownerless-page.test.tsx` 两格 |
| t23 | S2 | 添加成员一组没有标题 | `notebook/members-page.test.tsx` 两格 |
| 70 | S2 | 去掉第二轮的 `.nw-saved[data-round="1"]:not(:empty)` 规则（`styles.css`） | UI3 第 1 格比较两次写出的 `animationName`，三种浏览器（审查 C 的 P3S2C-M1，`dd6ef368`） |
| 71 | S2 | 工作区设置布局的 `SettingsPage` 去掉 `key` | `settings-directory.test.tsx` 工作区"到达之后再到达"一格（审查 A 的 P3S2A-N1，`dd6ef368`） |
| 72 | S2 | 个人设置布局的 `SettingsPage` 去掉 `key` | `settings-directory.test.tsx` 个人"到达之后再到达"一格（同上） |
| 73 | S2 | 导入一行的"能不能写"只在挂载时取 | `transfer-import.test.tsx` 编辑者变读者再变回一格（审查 B 的 P3S2B-N4，`1e6231e9`） |
| 74 | S2 | 能写时导入一行也给 `value` | 同上一格；`upgrade-read-only.test.tsx` 管理员一格 |

不列的：`SlotHolder` 只在焦点还在原处时调 `intoColumn`（审查 A 的 M3，4.1.5）在有了 `ScrollKept` 之后是等价变异：去掉守卫，`intoColumn` 量到的是 `hidden` 里的 0，滚回 0，`ScrollKept` 在设置模式里本也把左栏放到 0，返回时再还原；留着它，是"焦点还不上时不碰左栏"的写法清楚。强制色下的两处边框（决定 845）的样子由 7.4 的截图看，主按钮的类名另由第 62 条守。"已保存。"的预留宽度（决定 848）没有 vitest 或故事抓得住（jsdom 不排版，故事不在 768–1010 宽下读这两行），由 7.4 的 800 与 940 宽的截图看。

t1–t14b 是 S1 的实施者另写的（t11 即上面不列的 `SlotHolder` 守卫，等价；t10 没有格守着）；64–69b 随审查与修复核对的修复，69、69b 在滚动一格的桩发 `scroll` 之前活下来。审查者另写的（A 的十条，C 的 c1–c13，c3 等价）和它们的结果在 [S1 的审查记录](reviews/P3-S1-settings-mode-review.md)。

S2 的 t1–t23 是 S2 的实施者另写的（与 S1 的 t 各自编号，以 Step 一列分开）；t19 在 `397241c7` 补分组的格之前活下来，t21–t23 随那几格加。70–74 随审查的修复；56 在 S2 的 e2e 一份（56e）第一轮没抓住，`dd6ef368` 之后抓住。审查者另写的（A 的十二条，esc-stepped 等价；C 的 s1–s10，s4 等价）和它们的结果在 [S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)。

### 7.3 门禁

- **基线**：不另跑（决定 827）。对照的数是第 1 节的：vitest 216 个文件、5237 格；E2E 609 格过、3 格失败、1 格跳过（审查 C 的 N1）。
- **每个提交**：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`；改了行为、夹具或故事的另跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make e2e`。a15 随它照常跑一次（决定 829、843）。
- **合并之前**：在 Step 分支并入 `m13-ui-refresh` 的树上跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web gen-check-web build-web e2e`；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 以 0 退出；结果写进 Step 的记录。本机允许的失败只有第 1 节那三格（同行同消息）；AS6、LW8:771、"not slower"与 `trash-preview.test.tsx:587` 偶发（审查 C 的 N2），重跑全过即可，记下；别的红了先停、查清。
- **只靠 CI 的**：没有新的（UI2、UI3 不按 `Control+K`）。
- **文档**：`python3 docs/v0.1/claude_to_codex/tools/check_links.py origin/m13-ui-refresh`（远端的分支不在时用 `m13-ui-refresh`，同 02-P2 7.3）、`git diff --check`。

### 7.4 视觉的核对

B 审查者用 Playwright 截取下面这些，亮暗、1440 与 375 各一张，对照 settings.html 看，记录写明看了哪些：

- **S1**：个人资料（显示名没改、改了出现"保存"、问题提示、"已保存。"与 ✓；输入框在"保存"出现、消失与问题出现时都不动，"保存"与别的控件离卡片右边 19 px，375 宽时依次是输入框、问题、"保存"，没编辑时这一行底下不多出空行、"已保存。"与输入框左对齐；审查 B 的 R1-M1、R1-N1；940 宽时保存前后各一张，前后行高与输入框的位置相同；这个宽度下输入框一直在说明之下，是决定 848 的代价，审查 B 的 R3-N1）；外观与语言；安全（密码的卡片、危险操作）；访问令牌（有、没有、已过期；"新建令牌"是 30 px 胶囊）；工作区设置与笔记本设置的目录（三组、组标题的名字长到省略，包括 15 个以上的汉字：组标题"笔记本"不折行，"返回 某个很长的笔记本名"）；页的内容宽 696 px（`max-w-178`，决定 844）；`h1` 的行高；375 宽下抽屉里的目录；Chromium 强制色模式下的 ✓、"返回"、名称一行的"保存"（系统色的边框）与目录的当前一行（1 px 的系统色边框，决定 845）。有意的偏离：卡片里的"保存"30 px（决定 820）；暗色卡片 `--surface`（决定 820）；S1 期间工作区与笔记本设置的内容还是旧样子（决定 831）。
- **S2**：工作区的常规（管理员与成员；长的只读值右对齐、折行）、成员（含邀请与离开；列表行的按钮在右边，"移出""撤回"是描边的胶囊，决定 838）、无主笔记本（含审计日志）；笔记本的常规（"基本信息"、名称、开放程度的下拉与说明，"已保存。"出现时下拉不动、在它左边，375 宽时在它之后；800 宽时选前后各一张，前后行高与下拉的位置相同；这个宽度下下拉一直在说明之下，是决定 848 的代价，审查 B 的 R3-N1；英文说明宽屏两行，是 848 的代价；模式、危险操作）、成员（含添加）、导入导出（含展开的报告）；开放程度被拒时的横幅；升级进行中的笔记本设置页（顶上的升级横幅在 `main` 的全宽、比 696 px 的内容列宽，记下它与内容列怎样不齐，对齐留给 P4，审查 B 的 R1 未核实一条）；文件按钮的悬停（三种引擎，亮暗）；强制色下开放程度的 ✓。有意的偏离：模式一行不带样稿的"当前"徽记（审查 B 的 N3，4.2.4）。

### 7.5 CI 的余量

新故事两个：UI2 三格、UI3 两格，各在三种浏览器里，共 15 次。UI1 在 S3 的实测是每次 0.3–3.8 s（02-P2 7.5）；UI2 的格走的导航多（第 1 格约十次），推断每次 3–6 s；UI3 第 1 格等两次淡出（各约 1.8 s），推断每次 5–7 s，第 2 格开两个上下文，推断 3–5 s。合计约 60–80 test-seconds；CI 的 `ubuntu-latest` 约 2 个 worker，约合 30–40 秒的墙钟；本机 E2E 2.3–2.5 min 推断到 2.4–2.7 min。CI 的 E2E 一步今天约 6 分 21 秒，时限 18 分钟（00 号文档第 9 节），余量足够。其余故事只换定位与断言，次数不变（a15 不再另跑三遍，决定 829）。S1、S2 之后照实各记一次。

**本机的总时长**（粗算，经 `heavy.sh` 串行）：四个代码提交（S1、S2 各两个，决定 842）各一次 `make lint-web knip test-web`（约 70 秒）约 5 分钟；四个改行为的提交各一次 `make e2e`（约 5 分钟）约 20 分钟；两次合并之前的整条命令约 13 分钟，审查 C 各再跑一次约 13 分钟；变异 63 条，约 51 条只跑 vitest（一个文件 15–40 秒）约 26 分钟，约 12 条要重建前端再跑一个故事（每条约 3 分钟）约 36 分钟；修复核对每个 Step 一两轮约 30 分钟。合计约 2.4 小时的锁时间。

## 8. 完成标准

### 8.1 条件

1. 4.1–4.3 的每一项做到；7.1 的测试通过；7.2 的变异都被抓住（第 30 条在 S1 由截图看，S2 的 UI3 抓）。
2. 每个 Step 的合并之前，7.3 的门禁为绿（只败第 1 节同行同消息的那几格）；没有服务端与契约的改动。
3. 7.4 的截图看过，与样稿不同之处写进记录（是有意的，写理由；不是的，修）。
4. Phase 完成（02-workflow 3.7）：00 号文档第 12 节 P3 一行"完成"、头的状态、第 13 节一行；本文头的状态；`docs/v0.2/handoffs/M12-polish.md` 的"处理进展"一行（S2 的文档提交里写，决定 846）。

### 8.2 总体设计的新文字

每条由 Step 的文档提交里的脚本替换（"换成"是旧文字换成新文字，"之后加"是新文字接在旧文字之后），断言旧文字恰好出现一次：S1 的第 1–11 条的旧文字在 `ba66557c` 的 `docs/v0.1/v0.1-design.md` 里各出现一次；S2 的第 12–20 条的旧文字是 S1 写进去的（或 S1 之后仍在的），在 S1 改完的文字里各出现一次。设计者修订时用一个脚本从本节读出这些文字（引用块是旧文字与新文字），依次做了 S1、S2 的替换，每个旧文字都恰好一次（脚本在会话的临时目录，不进仓库）。中文里的引号照总体设计的写法是 ASCII 的 `"`。第 11、20 条里的链接在这里写在反引号里（它相对于 `docs/v0.1/`，不是本文的链接，免得 `check_links.py` 把它当成本文坏掉的链接），写进总体设计时不带反引号。S1 只改个人设置的页与卡片（决定 831），所以第 2、5、7 条在 S1 里只说个人设置，S2 的第 12、15–17 条再放宽到所有设置页（审查 C 的 N3）。长期的文字只写行为，effect 的先后留在 4.1.5 与代码的注释里（审查 A 的 R1）。

1. **9.2 左栏一段**（S1）：去掉"没有左栏的时候"里的"个人设置"。

   旧文字：

   > 没有左栏的时候（登录、注册、引导、新建工作区、个人设置、出错页、工作区读取失败，以及工作区之外与找不到工作区的 404；笔记本里的 404 有左栏）

   换成：

   > 没有左栏的时候（登录、注册、引导、新建工作区、出错页、工作区读取失败，以及工作区之外与找不到工作区的 404；笔记本里的 404 有左栏）

2. **9.2 左栏一段**（S1）：段末加设置模式与个人设置的页。

   旧文字：

   > Esc 或点遮罩关上（M13/P2/S3）。

   之后加：

   > 设置（个人、工作区与笔记本的）是左栏的另一种样子：顶部是"返回"（回到进入设置之前的那一页，带那本笔记本或那个工作区的名字）与标题"设置"，其下是"个人""工作区""笔记本"三组目录（各是一个导航，名为"设置""工作区设置""笔记本设置"），底部照旧是账户行，没有搜索框；工作区与笔记本的设置用工作区外框的那一个左栏，个人设置自己画一个。个人设置的页居中、内容最宽 696 px：页名作标题，其下是分组标题与卡片，每项一行（左边标题与说明，右边控件），不可逆的操作在最后一组"危险操作"；名称改了才出现"保存"，"已保存。"前有一个琥珀的勾，片刻之后淡去（M13/P3/S1）。

3. **13.2 第 16 条**（S1）：树的容器在设置模式里的去处；左栏的滚动。

   旧文字：

   > 拿着的焦点挪完之后还回去（M13/P2/S2）。

   之后加：

   > 设置模式（M13/P3/S1）下左栏换成设置目录，列表里没有一项收养这个容器：笔记本的设置里，目录之后一个 `hidden` 的 `div` 收养它，树照旧挂载、读取与监视，不显示，也不在可访问性树里；返回之后列表里那一项把它收养回去，不重画。左栏的滚动位置进入设置时放到顶上、返回时还原（`pages/workspace/workspace-layout.tsx` 的 `ScrollKept`）；焦点没还回原处时，`SlotHolder` 不滚左栏。

4. **13.2 第 16 条**（S1）："左栏常驻"改为"树常驻"。

   旧文字：

   > 是 `useMounted` 的变体：左栏常驻，发起的按钮

   换成：

   > 是 `useMounted` 的变体：树常驻（设置模式下也挂载在 `hidden` 的容器里），发起的按钮

5. **13.2 第 17 条**（S1）：个人设置不再有右上角；进出设置、目录、`IndexTo`、页的 `h1`、来处；三页的 `h1` 取到达的焦点。

   旧文字：

   > 左栏里通往工作区外框之外的菜单项（账户菜单的"个人设置"、工作区切换的"新建工作区"）与个人设置页、新建工作区页（账户已有工作区时）右上角的"回到工作区"同样带 `arrived`，个人设置页的 `h1` 取到达的焦点（M13/P2/S1）。

   换成：

   > 左栏里通往工作区外框之外的菜单项（账户菜单的"个人设置"、工作区切换的"新建工作区"）与新建工作区页（账户已有工作区时）右上角的"回到工作区"同样带 `arrived`（M13/P2/S1）。进出设置的链接（工作区导航的"设置"、笔记本行的齿轮、账户菜单的"个人设置"、设置目录的"返回"）与目录里跨外框的链接（个人设置与工作区、笔记本的设置之间）随左栏离开，带 `arrived`（账户菜单的"个人设置"在个人资料页上不带：同一个地址，`Link` 自动 `replace`，不算到达）；目录里同一个外框的链接同树的"目录""历史"两行，经 `useFollow`；三个设置的 `index` 重定向（`app/routes.tsx` 的 `IndexTo`）转交收到的路由状态；设置页的 `h1` 按页挂载（`app/settings-page.tsx` 的 `SettingsPage`），取到达的焦点，个人设置的 `h1` 是页名；进入设置之前的那一页由 `Layout` 在渲染时记下（`app/settings-origin.ts`；以 `replace` 进入的不记：守卫与引导的 `next` 换掉的那一页会立即把人送回设置），"返回"回到那里：工作区之外的来处连片段（邀请的令牌在片段里），工作区里的不带（页的 `h1` 与锚点谁后取焦点要看视图在不在缓存里，不带片段是确定的那一种）；变更集页、回收站的预览与邀请页的 `h1` 也取到达的焦点（M13/P3/S1）。

6. **13.2 第 17 条**（S1）：删除之后的焦点在设置页上到 `h1`。

   旧文字：

   > 焦点经 `ConfirmDialog` 的 `focusAfter` 移到这一节的标题（`tabIndex={-1}`）；

   换成：

   > 焦点经 `ConfirmDialog` 的 `focusAfter` 移到这一节的标题（`tabIndex={-1}`；设置页上与页名相同的那一节不另有标题，焦点到页的 `h1`，`useSettingsHeading`，M13/P3/S1）；

7. **13.2 第 12 条**（S1）："已保存。"的勾与改名称的一行；`NameRow` 是"发送中按钮禁用"的例外（审查 C 的 N4）。

   旧文字：

   > 成功的提示放在一直挂载的 `<output>` 里，只换文字。

   之后加：

   > 个人设置页的"已保存。"前的勾只用 CSS 画（`styles.css` 的 `.nw-saved`），不进文字，1.5 秒后淡去；下一次发出时先清空，每次写出时在两份同样的 `@keyframes` 之间换（`app/settings-page.tsx` 的 `SavedOutput`，`data-round`），淡出不论答复多快都从头来。个人设置里改名称的一行（`app/name-row.tsx` 的 `NameRow`）改了才出现"保存"，回车也保存、失焦不保存；它是本条开头"发送中按钮禁用"的例外：发送中"保存"是 `aria-disabled` 与 `aria-busy`、不禁用（禁用会丢焦点）；保存成功、它随之消失时焦点交给输入框；问题与 `aria-invalid` 只在字段与当前的名字不同时显示；草稿与编辑次数的规则同 `RenameForm`（M13/P3/S1）。

8. **13.2 第 20 条**（S1）：设置模式里两个键都不管。

   旧文字：

   > `aria-keyshortcuts` 列两个键，框里看得见 `⌘K`（`Ctrl K`）的提示。

   之后加：

   > 设置模式的左栏没有搜索框，两个键都不管、不 `preventDefault`（M13/P3/S1）。

9. **13.2 第 31 条**（S1）：强制色下的主按钮与目录的当前一行（决定 845）。

   旧文字：

   > 由系统色画出，M13/P1/S2）

   之后加：

   > ；`Button` 的 `default` 变体与设置目录的当前一行另有一圈透明的 1 px 边框，强制色下同样由系统色画出（M13/P3/S1）

10. **13.4 第 3 条**（S1）：UI2、`settingsBack`、`settingsGroup`。

   旧文字：

   > （抽屉是 `fixed`，`scrollIntoView` 不滚窗口）。

   之后加（开头一个空格，同这一条里 M13/P2 各段的接法）：

   >  M13/P3/S1：UI2 在三种浏览器里核对设置模式（从工作区导航、笔记本行的齿轮与账户菜单进入，页的 `h1` 取焦点，组名随改名变，"返回"回到来处，直接打开地址时回工作区，375 px 的抽屉里经齿轮到达之后连点两页、各页的 `h1` 取焦点），只有页面版本：改名的接口与 MCP 版本由 W4、N6 覆盖；在设置页上要读左栏的步骤先点"返回"（`fixtures/settings-pages.ts` 的 `settingsBack`），目录的三组以 `settingsGroup` 找。

11. **第 15 节**（S1）：变更记录一行（`YYYY-MM-DD` 换成提交的日期）。

   旧文字：`## 15. 变更记录` 与其后的空行。

   之后加：

   > - YYYY-MM-DD：M13/P3/S1（依据 `[P3 设计](M13-ui-refresh/03-P3-settings.md)`）：9.2 的左栏一段（个人设置不再算没有左栏，设置模式，个人设置的页）；13.2 第 12 条（"已保存。"的勾与它的淡出、改名称的一行）、第 16 条（设置模式里树的容器、左栏的滚动、"树常驻"）、第 17 条（进出设置与目录里的链接、页的 `h1`、来处、三页的 `h1` 取到达的焦点、删除之后焦点到 `h1`）、第 20 条（设置模式里两个键都不管）、第 31 条（强制色下的主按钮与目录的当前一行）；13.4 第 3 条（UI2）。

12. **9.2 左栏一段**（S2）："个人设置的页"改为"设置页"（工作区与笔记本的设置也是卡片了）。

   旧文字：

   > 个人设置的页居中、内容最宽 696 px

   换成：

   > 设置页居中、内容最宽 696 px

13. **9.2 左栏一段**（S2）：开放程度。

   旧文字：

   > 片刻之后淡去（M13/P3/S1）。

   之后加：

   > 笔记本的开放程度在下拉里选了即生效，被拒时退回原值、说明原因（M13/P3/S2）。

14. **13.2 第 12 条**（S2）：单选组一句改写，加开放程度的下拉、Esc 与说明。

   旧文字：

   > 单选组在"保存"时发出，不在选中时发出（方向键移动就会选中）。

   换成：

   > 写到服务端的单选组在"保存"时发出，不在选中时发出（方向键移动就会选中）；笔记本的开放程度是下拉，选了即发（`useChoice`：指针选了即发，键盘逐项走时等失焦或回车，走了一步按 Esc 退回已生效的值、这一键不再往外传），另持草稿：下拉与它的说明显示选的那一项（键盘走到、还没发出的也是）直到最后一次发出的答复回来，之前的答复不改它，被拒时退回列表里的值、原因显示在这一行下面，发送中不禁用，最后一次的答复回来之前带 `aria-busy`（M13/P3/S2）。

15. **13.2 第 12 条**（S2）：勾放宽到所有设置页。

   旧文字：

   > 个人设置页的"已保存。"前的勾

   换成：

   > 设置页的"已保存。"前的勾

16. **13.2 第 12 条**（S2）：改名称的一行放宽到工作区与笔记本。

   旧文字：

   > 个人设置里改名称的一行（`app/name-row.tsx` 的 `NameRow`）

   换成：

   > 设置页里改名称的一行（显示名、工作区与笔记本的名称，`app/name-row.tsx` 的 `NameRow`）

17. **13.2 第 17 条**（S2）：页名作 `h1` 放宽到所有设置页。

   旧文字：

   > 取到达的焦点，个人设置的 `h1` 是页名；

   换成：

   > 取到达的焦点，是页名；

18. **13.2 第 17 条**（S2）：设置页里随页离开的链接（决定 839）。

   旧文字：

   > 邀请页的 `h1` 也取到达的焦点（M13/P3/S1）。

   之后加：

   > 设置页里随页离开的链接（笔记本没有管理员时成员页通往无主笔记本的链接、接管之后的"打开它"、升级进行中模式一行的"查看升级"、任务行的"在历史里查看"与"被撤销的改动"）同样带 `arrived`；升级进行中笔记本各页顶上横幅里的"查看升级"（去的升级页上它不画）也带（M13/P3/S2）。

19. **13.4 第 3 条**（S2）：UI3。

   旧文字：

   > 目录的三组以 `settingsGroup` 找。

   之后加（开头一个空格，同这一条里 M13/P2 各段的接法）：

   >  M13/P3/S2：UI3 在三种浏览器里核对开放程度的即时生效（"已保存。"与它的淡去，第二次选时淡出重来）与被拒退回（另一位管理员在 `anotherPage` 里把本人降为编辑者，本人的事件流扣着），只有页面版本：接口与 MCP 版本由 N3 覆盖；`changeAccessWith` 选下拉。

20. **第 15 节**（S2）：变更记录一行，加在 S1 那一行之上（`YYYY-MM-DD` 同上）。

   旧文字：`## 15. 变更记录` 与其后的空行。

   之后加：

   > - YYYY-MM-DD：M13/P3/S2（依据 `[P3 设计](M13-ui-refresh/03-P3-settings.md)`）：9.2 的设置页一句与开放程度；13.2 第 12 条（开放程度的下拉即生效、Esc 与草稿，勾与改名称的一行放宽到所有设置页）、第 17 条（页名作 `h1` 放宽到所有设置页、设置页里随页离开的链接、升级横幅里的链接）；13.4 第 3 条（UI3）。

不改的：9.2 的账户行一句（"菜单里是个人设置……"）、9.3、13.2 第 15、18、21、22、26、30、32 条（写到"左栏"的几条在设置模式里照旧成立：第 26 条的"侧栏只滚自己"对目录同样；第 18 条按角色隐藏，目录照旧只给管理员"无主笔记本"；第 22 条文档标题照旧）。13.2 第 12 条开头"发送中按钮禁用"对 `NameRow` 与开放程度的下拉不成立，由第 7、14 条加的句子写明。

## 9. 交给后面的 Phase

- **P4**：`main` 照旧 `p-6`，设置页在宽屏上另加 `md:px-2 md:pt-5 md:pb-4` 凑出样稿的 44 / 32 / 40，内容最宽 696 px（`max-w-178`，决定 844；4.1.7）：P4 定了 `main` 的留白之后，`SettingsPage` 的这几个类随之改；细栏的位置文字在设置里是"设置"，P4 把它换成页的标题时，设置页同换（换成页名）；升级进行中，笔记本设置页顶上的升级横幅（`NotebookLayout` 画在 `Outlet` 之前，`W/pages/notebook/notebook-layout.tsx:86`）占 `main` 的全宽，比 `SettingsPage` 的 696 px 内容列宽、左边也不齐，并把 `h1` 往下推：P4 定 `main` 的版式时一并让它与内容列对齐（7.4 S2 有一张截图）。
- **P5**：别处通往设置的内容链接要不要带 `arrived`，照 13.2 第 17 条逐个看（决定 828；设置页里随页离开的六个 S2 已带，决定 839）：工作区首页无主笔记本提醒里的"Review them"（`W/pages/workspace/workspace-home.tsx:87`）；笔记本首页的"笔记本设置"（`W/pages/notebook/notebook-home.tsx:69`，去掉之后 `page-tree-scale.test.tsx:243` 改找齿轮"Settings of …"）；变更集页的"See its progress"（`W/pages/notebook/changeset-page.tsx:253` 交给 `SaidText` 的 `jobs`）；回收站预览的"See the jobs"（`W/pages/notebook/trash-preview.tsx:361`）。设置里的空列表（"No tokens yet."、没有邀请、没有无主笔记本、没有任务）换成 P5 的空状态组件。01-P1 第 9 节剩下的手写标签组八处（`space-y-2` 改 `space-y-1.5`）：S2B-N3 列表里剩下的五处 `W/pages/page/asset-dialogs.tsx:221`、`W/pages/search/search-page.tsx:221`、`W/pages/notebook/index-filters.tsx:60`、`W/pages/notebook/history-filters.tsx:154`、`W/pages/notebook/source-add-dialog.tsx:261`，与决定 846 补上的 `history-filters.tsx:132,143`、`index-filters.tsx:48`（审查 B 的 M3）。列表里的 `move-page-dialog.tsx:159,172` P2/S2 已改；P3/S1 改了 `create-token-dialog.tsx:184`，P3/S2 改了列表里的四处与列表外的 `add-member-section.tsx:102`（S2 的审查 C 的 P3S2C-N1）。历史与目录的筛选里标签与下拉在 flex 项里，可能并排（S2 的审查 B 未核实），随这几处一起看（决定 853）。P5/S1：提醒的"Review them"带了 `arrived`，无主笔记本的设置页的 `h1` 取焦点（`workspace-home.test.tsx` 核对）；笔记本首页的"笔记本设置"去掉了（[05-P5](05-P5-pages.md) 决定 919），`page-tree-scale.test.tsx:243` 改找齿轮"Settings of …"；变更集页、回收站与它的预览里的三个"See the jobs"在 S2 带 `arrived`，两个"See its progress"照旧经 `focusJob`（决定 924）；设置里的五个空列表（令牌、邀请、无主笔记本、审计、任务）换成卡片大小的空状态，`cardTextClass` 随之删掉（决定 938）；P4 交来的 `sectionAt` 用路由的 `matchPath` 比较（`end: true`、不分大小写），比较之前像路由一样逐段解码：带一个或几个尾随斜杠、大小写不同或带编码的地址都是那一页（决定 923，S1 的审查 C 的 P5S1C-M1、A 的 P5S1A-N2；04-P4 第 9 节"P5"）；其余七处标签组六处在 S2、`source-add-dialog.tsx:261` 在 S3，筛选里的标签由 `flex-col` 总在下拉之上（决定 926）（[P5/S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）。
- **P4、P5 的卡片**：要卡片的页（例如健康、升级）用 `W/app/settings-page.tsx` 的 `SettingsCard` 与一行的类名，不另写一套；`SettingsPage` 只给设置的布局用（它的 `h1` 按页挂载、取到达的焦点，别的页有自己的标题）。
- **收尾**：读屏清单"H. M13"一节加：设置模式的地标（三个 `nav` 的名称与它们的组标题是纯文字、名字与"工作区""笔记本"之间读出空格、没有搜索）；"返回"的名字（带笔记本或工作区名，或只是"返回"）与回到来处之后的焦点（变更集页、回收站的预览、邀请页的 `h1` 也取）；页名作 `h1`、进入与跨外框之后读到它；"已保存。"的 `<output>`（读屏读文字、不读勾，淡去之后仍读得到）；名称一行发送中的"保存"读作忙；开放程度：键盘走一步时说明随之换、Esc 退回，发送中读作忙，被拒时的 `alert` 与下拉退回原值；`hidden` 里的树不被读到；强制色模式下的勾、主按钮的边框与目录的当前一行。 M13 收尾：读屏清单 H 一节第 6–9 项是这些，第 7 项另有 Esc 退回草稿（决定 965）。
- **v0.2**：`docs/v0.2/handoffs/M12-polish.md` 第 1 项里的 19.5、19.6、20.4、31.1 照旧留着（决定 830、846）。

## 10. 结果

（S1、S2 合并之后各写一节：提交、做完的、与设计不同之处、实测、审查记录。）

### 10.1 S1：设置模式与个人设置（2026-11-24，合并 `f32b0762`）

- **提交**：`7c5b7517`（设置模式、目录与"返回"，照 6.1 提交 1）、`276c888a`（个人设置的卡片、`NameRow`、外观与语言一页，照 6.1 提交 2）；审查的修复 `2979f1c2`（变更集页读到之后 `h1` 取到达的焦点，`SettingsRow` 是 `observer`（决定 849），令牌页说明与"新建令牌"那一行，审查指出的测试缺口）；修复核对的 `6987baa9`（滚动一格的桩像浏览器那样发 `scroll`，令牌行的两个日期各自不折行）；加本文档提交。[S1 的审查记录](reviews/P3-S1-settings-mode-review.md)。
- **基线**：不另跑（决定 827）。提交 1 的 `make e2e` 兼作核对：最终只败第 1 节那三格（se2:231、pg15:113、lw8:836，同行同消息）。
- **做完**：4.1.1–4.1.12；5.1–5.3、5.6；决定 810–848 里 S1 的部分与 849–851。tb2 也照 hi5 改了（5.2 漏列，照实加）。外观页的单选跟着主题与语言（a8 抓到，决定 849）。01-P1 第 9 节的标签组里 `create-token-dialog.tsx:184` 一处（决定 830、846；其余五处在 S2）。
- **测试**：7.1 的 S1 各格。vitest 5237（216 个文件）→ 5280（219 个，提交 1）→ 5293（221 个，提交 2）→ 5296（221 个，`2979f1c2`；`6987baa9` 不变）。新格先写，在没改的树上红：提交 1 的 vitest 54 格、UI2 三格 × 三种浏览器；提交 2 的 15 格；审查的修复里未读到时到达的变更集、`SettingsRow` 跟着 store 两格。不是先红后绿的：守 `DisplayNameForm` 原有行为的三格（`name-row.test.tsx` 两格、`profile-page.test.tsx` 改写的一格）；外观页"选了即显示"两句是 a8 抓到缺口之后补的（先红后绿）；`name-row` 放开之后再数、列表读不到时的"返回"、滚动一格的夹断在头上本来就绿，由变异 64、66、68 看到红；滚动一格的桩发 `scroll`（修复核对 C 的 P3S1C1-M1）由 69、69b 看到红。
- **变异**：7.2 的 S1 各条（1–30、45–57、62）除 30 都被抓住；11、16、55 另在 UI2 第 3、1、3 格（Chromium，重建前端）；29 由 `check.mjs` 抓。30（✓ 的淡出）在 S1 没有格抓，由截图看。第 8 行只有 `layout.test.tsx` 抓住（审查 C 的 P3S1C-N4）。实施者的 t1–t14b：t10（目录当前一行的透明边框）没有格抓，由强制色截图看；t11 等价（7.2 表后）；t14 写法等价，换成 t14b 被抓住。审查之后加的 64–69b 都被抓住（64 `NameRow` 发送中的守卫，65 变更集页的到达焦点在读到之前，66 列表没读到时的"返回"，67 `SettingsRow` 不是 `observer`，68 `ScrollKept` 不在中段最后，69 `ScrollKept` 不移掉监听，69b 设置模式里也监听；旧的桩下 69、69b 是绿的，即审查 C 指出的缺口）。审查者另写的（A 的十条，C 的 c1–c13，c3 等价）与逐条的结果在审查记录（实施者跑，审查 C 独立重跑）。
- **视觉的核对**（7.4）：实施者在 Chromium 里截了亮暗 × 1440、375 的个人设置四页（个人资料四态）、940 宽保存前后、目录（长名字，中英文）、375 宽的抽屉、强制色、减少动效。实量：内容 696 px；`h1` 26/39 px、650、−0.26 px、下边 28 px；1440 宽名称一行四态输入框不动，"保存"离卡片右边 19 px，问题的字与输入框左对齐；940 宽三态行高都是 115、输入框不动；375 宽依次是输入框、问题、"保存"；组标题不折行；强制色下当前一行有 1 px 系统色边框。令牌页的说明与"新建令牌"在 768 宽以上同一行，375、320 宽折行（审查 B 的 P3S1B-M1，`2979f1c2` 之后重截）；令牌行的日期在 375、320 宽只在 `·` 之后折行（修复核对 B 的 P3S1B1-N1，`6987baa9` 之后重截，主持者看过）。审查 B 两轮。与样稿的差别只有设计里写明的有意偏离与 7.4 的照实（决定 851）。截图在会话的临时目录，不进仓库。
- **门禁**：每个提交的 `make lint-web knip test-web` 都绿；改了行为的提交跑了 `make e2e`（提交 1、2：618 过、3 败、1 跳，2.8 min；`2979f1c2`：618 / 3 / 1，2.6 min）。合并之前的整条命令：`276c888a` 上 vitest 221 个文件、5293 格，E2E 618 / 3 / 1，2.5 min；`2979f1c2` 上 5296 格，618 / 3 / 1，2.7 min；`6987baa9` 上 5296 格，619 过、2 败（se2:231、lw8:836；pg15:113 这次过了）、1 跳，3.2 min（load 27.47）。审查 C 在 `2979f1c2` 上第一次另败两格 WebKit（UI2:95、LW10:23：造数的 API 在负载下答 500，`commit transaction: timeout: context deadline exceeded`），重跑只败基线。UI2 九次合计约 17–20 s。受保护路径的 `git diff --quiet` 与 `git diff --check` 都以 0 退出。逐项在审查记录。
- **与设计不同之处**：4.1 的"照实"（4.1.1、4.1.5、4.1.7、4.1.8、4.1.10、第 5 节、7.1、7.2、7.4）；决定 849–851。
- **交给后面**：`rowValueClass` 要用时再导出；S2 的列表名字随令牌行用 `text-sm`（决定 850；邀请行与任务行不经 `MemberSummary`，各自改）；第一组的分组标题照决定 851；S2 的工作区与笔记本名称经各自的 store，写"扣住时再提交"的格时同样在放开之后再数（审查 C 的 P3S1C-M1）；决定 839 的任务行链接会到达尚未读取的变更集，`ChangesetRead` 已经能处理。

### 10.2 S2：工作区与笔记本设置（2026-11-24，合并 `12b0b734`）

- **提交**：`b0768903`（卡片、页名作 `h1`、危险操作在最后，照 6.2 提交 1）、`5cb1de3f`（开放程度即生效、被拒退回、Esc 退回一步，照 6.2 提交 2）；照实加的 `397241c7`（成员页与无主笔记本页分组的格，变异 t19 活下来之后补；实施者截图看出的三处：三个下拉在标签之下、添加成员写出的那句在按钮旁边、成员的日期不折行）；审查的修复 `1e6231e9`（列表行的文字块伸缩、空的 `<output>` 不占流、明细与日期不折、徽记跟在名字之后、安全页写出的那句在按钮旁边、导入一行不能写时那句是行的值，决定 852、853）与 `dd6ef368`（UI3 比较两次写出的 `animationName`，工作区与个人设置的 `h1` 重挂各一格，`useChoice` 的注释）；B 修复核对的 `8d98f64f`（安全页、邀请与添加成员写出的那句可折到按钮之下，邀请与添加成员的长词在表单边上断开）；加本文档提交。[S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)。
- **基线**：不另跑（决定 827）。提交 1 的 `make e2e` 兼作核对：只败第 1 节那三格（se2:231、pg15:113、lw8:836，同行同消息）。
- **做完**：4.2.1–4.2.7；5.4、5.5；决定 810–851 里 S2 的部分与 852、853。M12-polish 第 1 项里的 19.2（"By {name}"）。01-P1 第 9 节的标签组 S2 改了五处：S2B-N3 列表里的 `invitations-section.tsx:130`、`import-dialog.tsx:318,361`、`add-member-section.tsx:132` 与列表外的 `add-member-section.tsx:102`（`move-page-dialog.tsx:159,172` P2/S2 已改；列表里剩下五处，连同决定 846 补上的三处在 P4、P5，第 9 节；审查 C 的 P3S2C-N1）；`Input` 的 `file:hover:hover-layer`（三种引擎截图看过）；开放程度的下拉 30 px（`h-7.5`）。
- **测试**：vitest 5296 → 5306（提交 1）→ 5316（提交 2）→ 5322（`397241c7`）→ 5323（`1e6231e9`）→ 5325（`dd6ef368`；`8d98f64f` 只改类名），都是 221 个文件；E2E 618 → 624 过（UI3 两格 × 三种浏览器）。新格先写，在没改的树上红：提交 1 相关的 12 个文件里 11 个、51 格；提交 2 的 vitest 12 格，UI3 两格 × 三种浏览器与 n3 的 page 格。不是先红的：`397241c7` 的分组四格（变异 t19、t21–t23 看到红），审查之后的三格（71、72、73 看到红），UI3 的 `animationName`（56e、70 看到红）；它们守已有的行为。
- **变异**：7.2 的 S2 各条（31–44 与它们的 e2e 一份、39b、58a–f、59–61、63）都被抓住；44 由 `check.mjs` 抓。56 的 S2 抓手原写的 UI3 透明度抓不住（审查 C 的 P3S2C-M1，9 次全过），改为比较两次写出的 `animationName`，56e 在三种浏览器里都红；另加 70（第二轮的 CSS 规则去掉）。实施者的 t1–t23 都被抓住，t19 在补格之前活下来。审查之后的 70–74 都被抓住。没有格守的：文件按钮的悬停、危险操作组的 `destructive` 与审查修复里的类名（`listTextClass`、`empty:absolute`、`WholeParts`、徽记、并排、`max-w-2xl`、四个下拉的 `block`、`8d98f64f` 的可折与 `min-w-0 break-words`），由截图看。审查者另写的（A 的十二条，C 的 s1–s10）与逐条的结果在审查记录；等价的：A 的 esc-stepped 与 C 的 s4（同一条）、导入一组的 `observer`（父组件随笔记本重画）。
- **视觉的核对**（7.4）：实施者在 Chromium 里截了亮暗 × 1440、375 的六页，800、1440、375 宽开放程度的前后，长的只读值，被拒的横幅，名称编辑中，升级横幅，强制色，导入对话框，文件按钮的悬停（三种引擎、亮暗）。实量：内容列 x=498、宽 696，控件离卡片右边 19 px；开放程度"Saved."宽屏占 68 px、在下拉左边 8 px，说明一样的两个值之间来回选，800、1440、375 宽前后的行高与下拉位置相同；70 个汉字的工作区名在 1440 宽右对齐、三行，两种宽度都不横向滚动；升级横幅 x=276、宽 1140，`h1` 下移 74 px（留给 P4）；强制色下 ✓ 是 CanvasText。审查之后重截（亮暗 × 1440、768、375）：成员、无主、任务、令牌各行的按钮 1440 宽都在右边、离卡片 19 px；无主页说明到卡片 10 px，邀请卡片到"Danger zone"26 px；"You"在名字最后一行；安全页按钮到卡片底 17 px；升级中导入一行那句灰、靠右；移动对话框的两个下拉在标签之下。B 修复核对之后重截（`8d98f64f`，亮暗 × 1440、375、320）：安全页、邀请与添加成员写出的那句长了都在按钮之下 8 px、不超过表单宽，空的时候按钮到那一块的底照旧 16–17 px，都不横向滚动。审查 B 截了 194 张（三种引擎）。截图在会话的临时目录，不进仓库。
- **门禁**：每个提交的 `make lint-web knip test-web` 都绿；改了行为的提交跑了 `make e2e`（提交 1：618 / 3 / 1，3.0 min；提交 2：624 / 3 / 1，2.8 min；`1e6231e9`：624 / 3 / 1，2.5 min，审查 C）。合并之前的整条命令：`397241c7` 上实施者 5322 格，E2E 623 过、4 败（基线三格与 AS6 `as6:167` 的抖动，单独重跑过）、1 跳，2.5 min，审查 C 625 / 2 / 1（pg15 过了），2.8 min；`dd6ef368` 上 5325 格，实施者 624 / 3 / 1，2.5 min（负载 11.7 → 34.0），审查 C 624 / 3 / 1，2.6 min；`8d98f64f` 上 实施者 5325 格，624 / 3 / 1，2.5 min（负载 15.5 → 28.9）。UI3 七次合计约 16–17 s：第 1 格 4.0–4.9 s、第 2 格 0.77–1.8 s，都低于 7.5 的推断（5–7 s、3–5 s）。受保护路径的 `git diff --quiet` 与 `git diff --check` 都以 0 退出。逐项在审查记录。
- **与设计不同之处**：照实多一个代码提交 `397241c7`；4.2 末的"照实"（4.1.7、4.2.1、4.1.10、4.2.3、4.2.4、4.2.5、5.4、5.5、7.1、7.2、7.4）；决定 852、853。
- **交给后面**：升级横幅与内容列对齐（P4，第 9 节）；01-P1 第 9 节剩下的标签组与历史、目录筛选里可能并排的标签（P4、P5，决定 853）；设置里的空列表换成 P5 的空状态组件（第 9 节）；Linux 上关着的下拉逐项走只由 vitest 守，在浏览器里靠负责人推送之后的 CI（审查 A 未核实的怀疑）。

## 附录：核对过的代码

在 `ba66557c` 上读过（行号以此为准）：

- 外框与路由：`W/app/routes.tsx:1-130,131-135,228-330`；`W/app/layout.tsx`（全文）；`W/app/corner.tsx`（全文）；`W/app/shell-sidebar.tsx`（全文）；`W/app/arrival.ts:1-60,95-260`；`W/app/guards.tsx:74-81`；`W/app/user-menu.tsx:40-130`；`W/app/form.ts:1-80`；`W/lib/focus-leaving.ts`（全文）；`W/app/rename-form.tsx`、`W/app/display-name-form.tsx`（全文）。
- 工作区与笔记本的外框：`W/pages/workspace/workspace-layout.tsx`（全文）；`W/pages/workspace/notebook-slot.tsx`（全文）；`W/pages/notebook/notebook-layout.tsx`（全文）；`W/pages/notebook/page-tree.tsx:1-30,284-301`；`W/stores/context.tsx:70-95`；`W/stores/root.store.ts:305-330`；`W/stores/notebook.store.ts:78-114`；`W/stores/preferences.store.ts:20-100`。
- 设置的布局与页：`W/pages/settings/settings-layout.tsx`、`profile-page.tsx`、`security-page.tsx`、`deactivate-dialog.tsx`、`tokens-page.tsx`、`token-row.tsx:25-71`；`W/pages/workspace/settings-layout.tsx`、`general-page.tsx`、`members-page.tsx`、`invitations-section.tsx:30-100`、`ownerless-page.tsx`（结构）、`audit-section.tsx`（结构）；`W/pages/notebook/settings-layout.tsx`、`general-page.tsx:1-180,292-307`、`members-page.tsx:30-199`、`add-member-section.tsx:30-60`、`transfer-page.tsx:60-220`、`transfer-job-row.tsx:60-80`、`access-options.tsx`（全文）；`W/pages/workspace/workspace-home.tsx:80-92`；`W/pages/notebook/notebook-home.tsx:69`。
- 组件与样式：`W/components/nav-item.tsx`、`form-field.tsx`、`ui/button.tsx`、`ui/input.tsx`、`ui/native-select.tsx:1-110`（全文或所列）；`W/styles.css`（全文）；`W/lib/cn.ts`；`W/i18n/isolated.tsx`；`W/i18n/i18n.tsx:20`；`W/i18n/messages/en.ts`、`zh-CN.ts` 的 `settings.*`、`profile.*`、`preferences.*`、`access.*`、`notebookSettings.*`、`workspaceSettings.*`、`transfer.started*`、`form.email`。
- 测试：`W/design-tokens.test.ts`（全文）；`W/app/layout.test.tsx:48-60,160-260`；`W/app/user-menu.test.tsx:150-200`；`W/pages/settings/profile-page.test.tsx`（全文）；`W/pages/workspace/workspace-layout.test.tsx:95-116`、`general-page.test.tsx:50-80`、`ownerless-page.test.tsx:100-246`、`members-page.test.tsx:135-146`；`W/pages/notebook/general-page.test.tsx:1-60,375-400`、`members-page.test.tsx:10-16,200-250,440-465`、`transfer-background.test.tsx:476-490`、`transfer-page.test.tsx:51-70,860-866`、`transfer-import.test.tsx:40-60`；`W/pages/create-workspace.test.tsx:142-150`、`W/pages/invitation.test.tsx:240-262`、`W/pages/workspace/workspace-home.test.tsx:125-133`；全量 vitest 在分离工作树里跑过两次（第 1 节末）。
- e2e：`e2e/playwright.config.ts:1-12`；`tools/e2e-projects/check.mjs:1-70`；`e2e/fixtures/browser.ts:75-95`、`events.ts:205-225`、`notebook-pages.ts:75-110`、`workspace-pages.ts:60-85`、`settings-pages.ts`、`notebooks.ts`、`notebook-members.ts`（导出）、`shell-pages.ts`、`narrow.ts`；故事 w4:105-117、n8:128-150、n9:145-190、n3:55-87、n6:65-90、n4:135-168、lw12:150-275、hi5:150-222、a8（全文）、a15:180-205 与读标题的各行、a7:80-120、n5:60-92、pg13:95-140、up8:73-135、lw10:320-395、n11:130-160；按文件 `grep` 了访问设置的 39 个故事里读左栏的帮手。
- 服务端（只读，为 UI3 的 403）：`server/internal/modules/notebook/app/update_notebook.go:42-75`、`manage.go:26-51`；n6:40-49 的 403 `forbidden`。
- 文档：[00-M13](00-M13-design.md) 4.2–4.4、4.8、第 7、9、10、12 节；[02-P2](02-P2-shell.md) 第 0、1、6–9 节；[01-P1](01-P1-foundation.md) 第 9 节与 [P1/S2 的审查记录](reviews/P1-S2-components-review.md) S2B-N3；[mockups/settings.html](mockups/settings.html) 的样式表；[M5-polish](../M12-release/handoffs/M5-polish.md) 第 19 条；[M12-polish](../../v0.2/handoffs/M12-polish.md) 第 1 项与"处理进展"；[v0.1 总体设计](../v0.1-design.md) 9.2、13.2 第 12、16、17、20 条、13.4 第 3 条、第 15 节的开头。

- 修订时另读（同在 `ba66557c` 上）：`W/app/arrival.ts:60-94,200-232`；`W/pages/page/view-anchors.ts:50-105`；`W/app/guards.tsx`（全文）、`W/pages/onboarding.tsx` 的 `next`；`W/pages/invitation.tsx:25-60`；`W/pages/notebook/changeset-page.tsx:90-110,245-275`；`W/pages/notebook/trash-preview.tsx:355-365,400-420,575-590`；`W/pages/workspace/notebook-slot.tsx:100-140`；`W/pages/notebook/page-tree.tsx:250-265`；`W/app/shell-sidebar.tsx:25-45,240-270`；`W/pages/notebook/transfer-job-row.tsx:140-170`；`W/pages/workspace/ownerless-page.tsx:125-140`；`W/pages/notebook/members-page.tsx:135-152`；`W/pages/notebook/general-page.tsx:185-195`；`useChoice` 的三个用者（`search-page.tsx:184`、`index-filters.tsx:31`、`history-filters.tsx:115-117`）与第 9 节的八处标签组；`W/components/ui/button.tsx:1-40`；`W/pages/notebook/transfer-background.test.tsx:120-150`；`W/pages/workspace/workspace-layout.test.tsx:125-150`；`W/pages/settings/profile-page.test.tsx:150-160`、`W/pages/workspace/general-page.test.tsx:105-115`、`W/pages/notebook/general-page.test.tsx:85-225`；故事 tr1:210-220、tr2:198-206、n9:185-190；`e2e/fixtures/narrow.ts`；三位审查者的探针（审查记录列出的临时目录）。
