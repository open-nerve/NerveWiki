# M13/P5 其余页面：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M13/P5 其余页面（三个 Step：S1 首页与入口；S2 列表与筛选；S3 版本、健康、升级、附件与对话框） |
| 日期 | 2026-11-25（初稿；同日照设计审查修订（审查记录由主持者写进 `reviews/P5-design-review.md`；三份审查全部接受，主持者的决定 940–945），又照修订核对第 1 轮再修订（三份报告全部接受，没有新的决定），照修订核对第 2 轮三修（A、C 的报告全部接受，B 没有新的发现，没有新的决定）；主持者照修订核对第 3 轮改了 A、C 的两条 Minor 与两条 Nit（大半用审查 C 给的文字，没有新的决定），又照第 4 轮改了 C 的一条 Minor 与一条 Nit（用 C 的文字）之后定稿；修订的依据写在各条里）；同日照 S1 的实施与审查修订（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)，主持者的决定 946–951） |
| 状态 | 设计定稿（2026-11-25）：B 在修订核对第 2 轮、A 在第 4 轮没有新的发现；C 第 4 轮的一条 Minor（主持者换了 2.1 第 12 条的锚点，重放的基随之改成 `3e288acb`，四处说法跟着改）与一条 Nit 由主持者用 C 的文字改，C 在第 5 轮核对过（没有 Important、Minor，最后一条 Nit 由主持者补上）、在改后的全文上重放过；[审查记录](reviews/P5-design-review.md)，主持者的决定 912–945；S1 实施与审查完成（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)，主持者的决定 946–951），合并 `<待填>` |
| 上级文档 | [M13 总设计](00-M13-design.md) 4.6（全部）、4.2.3 末句（"已登录的邀请页与出错页回去的路留给 P5"）、4.3"空状态""加载"两条、4.4.2（卡片与行）、4.7、4.8（变异的最低清单里"把 `--faint` 或 `--accent` 调淡"一条）、第 7 节 P5 两行、第 9 节"全部 e2e 在 P5 之后完整跑一次"；[04-P4](04-P4-reading.md) 第 9 节"P5"一条（`main` 的留白、升级横幅与内容列、版本页等的头、`Mark` 的 `current`、右栏的一行灰字、`sectionAt` 的尾随斜杠、版本页标题在 320 宽、右栏版本行的"-"）；[03-P3](03-P3-settings.md) 第 9 节"P5"与"P4、P5 的卡片"两条（通往设置的内容链接、设置里的空列表、八处标签组、卡片用 `SettingsCard`）；[01-P1](01-P1-foundation.md) 第 9 节"P5"（空状态与骨架）与"P2–P5"（标签组）两条；[v0.1 总体设计](../v0.1-design.md) 9.2、12.2、13.2 第 17、31 条、13.4 第 3 条；移交 [M12-polish](../../v0.2/handoffs/M12-polish.md) 第 1、5 项（编号沿用 [M5-polish](../M12-release/handoffs/M5-polish.md) 的条目） |
| 视觉参照 | 没有 P5 自己的样稿。外观照三份样稿里已有的部件：[mockups/settings.html](mockups/settings.html) 的页（26 px 的标题、13 px 灰的分组标题、14 px 圆角的卡片与行、`.badge`）用于列表类的页与首页的卡片；[mockups/reading.html](mockups/reading.html) 的头一行（`.topbar`、`.crumb`）、32 px 的标题与 720 px 的一列用于版本页、变更集页与回收站的预览，`.meta .dot` 用于升级横幅的小圆点；[mockups/palette.html](mockups/palette.html) 的配色与按钮。外观的结论以正文的类名为准（第 1 节 (b)、(h)）：初稿的静态参照页与正文有几处不同（审查 B 的 P5B-N8），修订时不再引它们，另照正文的类名写探针页 |
| 代码位置的写法 | 同 00 号文档：`W/` 是 `web/apps/web/src/`；e2e 写作 `e2e/stories/…:行`。行号是 `f62d0390`（P4 完成时 `m13-ui-refresh` 的头）的；之后的 `9871c7d4`、`3e288acb`（CI 的两处修复，04-P4 10.4）只改了 pg17、`e2e/fixtures/notebook-pages.ts` 与三份文档，本文引的行里只有 pg17:654（今天 `:677`）与 `notebook-pages.ts:17-37,55`（今天 `:17-41,59`）挪了；2.1、8.2 照今天的头 `3e288acb` 做（2.1 第 12 条的旧文字是 `3e288acb` 加的 CI 修复那一行的末句，`f62d0390` 上没有），32 处各恰好一次（审查 C 的 `p5-rev-c/r4b/`） |
| 工作笔记 | 设计只写了本文（2.1 对 00 号文档与总体设计的修订由主持者随设计的提交落进去）。探针都在会话的临时目录 `p5-design/`，不进仓库：初稿的 (a)–(f)（第 1 节）；修订时的 (g)–(k)：右栏版本行的"-"（决定 943）、照正文类名的探针页、头一行三串从 `page-layout.tsx` 导入时的分包、设计基础的守卫、2.1 与 8.2 的替换；修订核对第 1 轮之后再修订时的 (l)：标题行、横幅的小圆点与历史行的第一行（(g) 另扫了第四次）。`f62d0390` 的分离工作树初稿用 `nw-p5-design`、修订用 `nw-p5-rev`、再修订用 `nw-p5-rev2`，都用完即删 |

---

## 0. 分三个 Step

00 号文档第 7 节把 P5 分成两个 Step。S2 那一行列了十一样（搜索、历史与变更集、回收站与预览、目录、标签、健康、升级、导入导出、版本与对比、附件一节、对话框的内容），读它们的测试有一百多处（第 1 节 (f)），一次审查看不完；这里把它拆成两个，各自成块、各自为绿（决定 912）。三个 Step 串行，各自一个分支（`m13-p5-s<m>`，从 `m13-ui-refresh` 拉出、合并回它）、一次审查与修复核对、一次合并。每个提交单独为绿（7.3）。

- **S1 首页与入口**（`m13-p5-s1`）：空状态的组件 `EmptyState`（两种大小，决定 913）；工作区首页的笔记本卡片与虚线的"新建笔记本"（决定 917、918、942）；笔记本首页的标题行、带图标与子页面数的顶层页面列表，去掉"笔记本设置"的链接（决定 919）；设置里五个空列表换成卡片大小的空状态（决定 938）；登录、注册、邀请、引导、新建工作区的居中卡片（决定 920）；出错页、会话不可用与 404 的居中样子与"回到首页"（决定 921、922），已登录的邀请页右上角的"回到工作区"，与它带来的接受邀请时的竞态在 store 里补上（决定 941）；内容链接的 `arrived`（决定 924，回收站与变更集的在 S2）；P4 交来的 `sectionAt` 的尾随斜杠与大小写（决定 923）；升级横幅的"路由 × 支"表（4.3.6，从这里起每个 Step 加自己的路由），页的早退两支随之画在列里。
  - **提交 1** `sectionAt` 一处，连同它的 vitest；
  - **提交 2** 空状态、两个首页、设置的空列表，页的早退两支的列，`expectFits` 收窗口的宽度，连同 UI1 的新一格（两个首页在 375 与 320 宽）与 `REQUIRED` 两行、横幅的表的第一批路由；
  - **提交 3** 入口的卡片、出错与 404 的样子、邀请页的"回到工作区"与 `WorkspaceStore.load` 的再读（审查 A 的 P5A-R1-M1），连同 UI1 的又一格（登录的卡片与 404 在 375 宽）与 `REQUIRED` 两行。
- **S2 列表与筛选**（`m13-p5-s2`，S1 合并之后开）：列表类的页的格局（标题行、说明、筛选条、卡片里的行，决定 925）与它的类名（S1 建的 `W/app/page-classes.ts` 里再加的那些）；同一种行（第一行 14 px、500，链接悬停才有下划线，审查 B 的 P5B-I2）；筛选条与六处标签组（决定 926；第七处在 S3 的对话框里）；行内次要操作的显出（决定 927）；搜索、历史、变更集、回收站、回收站的预览、目录、标签七页；变更集页与回收站预览的头照页的头一行（决定 929），头一行的三串与列、标题的串从 `page-layout.tsx` 原处导出（决定 925）；改写的行与头里的内容链接带到达的状态（决定 940、924）；这些页的空状态；se1 暗色的链接下限照 4.7 重量（决定 936）。
  - **提交 1** `page-classes.ts` 的列表类名、`page-layout.tsx` 原处导出的串与 `PageColumn`、搜索、历史、变更集，连同 hi3 的显出、mc3 的窄屏一句、se1 的下限、横幅的表的两个路由；
  - **提交 2** 回收站与它的预览、目录、标签，连同 TB1 的新一格（认领 M12-polish 第 5 项"回收站预览 `h1` 的长名"，决定 937）与横幅的表的四个路由。
- **S3 版本、健康、升级、附件与对话框**（`m13-p5-s3`，S2 合并之后开）：版本页照页的头一行，"还原这个版本"进头一行、能编辑的人那一格总在（决定 929，P4 交来的"版本页的标题在 320 宽"；审查 A 的 P5A-M4）；`Mark` 的样子（决定 930），历史的"Reverted"同一种形状；右栏版本行的"当前""编辑中"接在"· Edit"之后（决定 943）；健康与升级两页（决定 932）；附件一节（决定 933，认领 16.7 看得见的一半）；对话框里的两个框、粘贴来源的文本框与一处标签组（决定 934）；升级横幅的样子，它从此总在列里，`NotebookLayout` 里"不在列里"的那一支去掉（决定 915、935）。
  - **提交 1** 版本页、`Mark`、右栏的版本行、健康、升级，连同 hi2 的 320 宽一段、hi1 与 hi2 读版本行的两句、横幅的表的三个路由；
  - **提交 2** 附件一节、对话框的内容、升级横幅与去掉的那一支。

**为什么这样分、各自为绿**：S1 只动首页、入口与出错的页（加上设置页里的空列表、`sectionAt` 与页的早退两支），列表类的页一页不碰；S2 只动工作区与笔记本里"一页一个列表"的七页，它们共用新的筛选条与行的类名，S1 的 `EmptyState` 已在；S3 动剩下的几页与跨页的部件（附件一节同时在页的底部与笔记本首页、对话框、升级横幅），最后去掉 `NotebookLayout` 的回退：那时每个笔记本页的每一支都已在自己的列里画 `<ColumnTop />`。读它们的测试按页分开（第 1 节 (f)），只有 `upgrade-banner.test.tsx` 三个 Step 都碰（各加自己那几页在表里的行）。每个 Step 结束时，没排的页照旧是今天的样子、照旧在 `main` 的全宽，横幅照旧画在它们之上（`W/pages/notebook/notebook-layout.tsx:97`），不会有页丢了横幅。

**单独为绿**：S1 提交 1 只改一个函数；提交 2 新的 `EmptyState` 在同一个提交里有两种大小的用处，它的 `iconTileClass` 在这个提交里不导出（只在本文件用，knip 照默认配置报"只在本文件用的导出"，审查 A 的 P5A-I2、C 的 P5C-M1），提交 3 的 `StatusPage` 用它时才导出；`page-layout.tsx` 的 `PageColumn` 与列的串同样先只在本文件用、不导出（S2 提交 1 导出）；`notebook.homeEmpty` 等文字照旧在 `<p>` 里；提交 3 不碰首页。S2 提交 1 给 `page-classes.ts` 加的类名与 `page-layout.tsx` 导出的串在同一个提交里被搜索、历史与变更集用到（`splitRowClass` 等到提交 2 的回收站才有用处，就在提交 2 里加）；提交 2 只加用处与它们。S3 提交 1 不碰附件与横幅；提交 2 去掉回退时，S1–S3 已给每个笔记本的子路由画了 `<ColumnTop />`，每一支都在列里（4.3.6 的表，12 个路由）。

## 1. 基线

- **外框**：唯一的 `main` 是 `flex min-w-0 flex-1 flex-col p-6 xl:px-10`（`W/app/layout.tsx:71`，P4 的决定 855：1280 px 起左右 40 px）。没有左栏时右上角的 `Corner`（`W/app/corner.tsx`）放账户按钮或"外观"；新建工作区页在账户已有工作区时另放"回到工作区"（`:64` 的 `back`：`:56` 的 `creating` 且工作区的列表不空，链接去 `/`、带 `arrived`，`:69-78`；工作区只在这一页读，`:59-61`）。已登录的邀请页右上角只有账户按钮。
- **列与升级横幅**：只有 `pages/:pageId` 带 `handle: { column: true }`（`W/app/routes.tsx:188`），笔记本设置带 `handle: { settings: true }`（`:266`）。`NotebookLayout` 以 `useMatches` 看这两个（`W/pages/notebook/notebook-layout.tsx:23,67-70`）：命中时横幅经 `ColumnTopContext` 交给页的列（`:98-100`），否则画在出口之上、`main` 的全宽里（`:97`）。`columned` 只看路由的 `handle`，不看页的哪一支画不画 `ColumnTop`：加了 `handle` 的路由，凡是没画它的一支就没有横幅（审查 A 的 P5A-I1）。画 `<ColumnTop />` 的只有 `W/app/settings-page.tsx:38`、`W/pages/page/page-layout.tsx:80,90,431`；`:80`、`:90` 是页读取中或失败、页不在的两支，写成片段（`:79-82`、`:89-92`），横幅的父元素是外框里出口的容器，不在 720 的列里：树读到之前横幅在 `main` 的全宽，读到之后进列，宽窄与左边都跳一次。横幅是 `output.mb-6.block.space-y-1.rounded-md.border.border-primary/40.bg-surface-2.px-4.py-3.text-sm`（`W/pages/notebook/upgrade-banner.tsx:41`）。`W/pages/notebook/upgrade-banner.test.tsx` 有五格读它的位置：页（`:141-151`，`banner.parentElement === heading.parentElement`）、页不在（`:153-163`）、树读不到（`:165-177`，`nodesDown`）、笔记本设置的 `index`（`:179-207`，`MutationObserver`）、版本页"不在列里"（`:209-220`）。
- **P3 的卡片与行**（`W/app/settings-page.tsx`）：设置的列 `settingsColumnClass = "mx-auto w-full max-w-178 md:px-2 md:pt-5"`（`:24`）；`SettingsPage` 的 `h1` 是 `mb-7 text-[26px] leading-normal font-[650] tracking-[-0.01em] break-words outline-none`（`:39-45`），类名没有导出；`useSettingsHeading`（`:12-18`）在 `SettingsPage` 之外抛错；`SettingsCard`（`:87-113`）的类名是函数里的局部常量 `"divide-y divide-border rounded-card border border-border bg-surface"`，`list` 时是带 `aria-label` 的 `ul`，`ordered` 时是 `ol`；导出的有 `listRowClass`（`:116`）、`stackedRowClass`（`:121`）、`listLineClass`、`listTextClass = "min-w-0 flex-1 basis-56"`（`:127`）、`badgeClass`（`:132-133`）、`WholeParts`（`:139-146`，各段 `whitespace-nowrap`、之间" · "）、`cardTextClass = "px-4.5 py-3.5 text-sm text-muted-foreground"`（`:148`）。
- **P4 的页的头**（`W/pages/page/page-layout.tsx:388-441`）：外层 `md:-mt-3 md:pb-4 xl:flex xl:items-start`（`:388`）；头一行 `mb-7.5 flex min-h-9 flex-wrap items-center gap-x-3 gap-y-1.5`（`:391`），面包屑 `min-w-0 flex-[1_1_14rem]`（`:393`），右边一组 `ms-auto flex min-h-8 min-w-0 flex-[0_1_26rem] items-center justify-end gap-1.5`（定宽，P4 的决定 894：行数只看宽度）；内容列 `mx-auto w-full max-w-180`（`:430`），先 `<ColumnTop />`，再 `h1`（`:436`，`mb-3.5 min-w-0 text-[32px] leading-tight font-[650] tracking-[-0.012em] break-words outline-none`）；子页面一节的 `h2` 是 `text-sm font-medium text-muted-foreground`（`:507`）。这几串都是 JSX 里的字面量，没有导出；`page-layout.tsx` 今天只有 `version-page.tsx:22` 导入（`PageGone`）。"编辑"是 `size="sm"` 的胶囊（`Button` 的 `shape="pill"`）。`HeadMeta` 与右栏开关只属于页（`W/pages/page/page-head.tsx`）。
- **首页**：工作区首页（`W/pages/workspace/workspace-home.tsx`）是 `section.max-w-4xl.space-y-6`（`:31`）、`h1.text-2xl.font-semibold.outline-none`（`:32`，`useArrivalFocus`）、管理员的无主笔记本提醒（`:85-90`，"Review them"链到 `/<slug>/settings/ownerless`，不带 `arrived`）与 `NotebookGroups`（`:36-65`：分组标题 `text-lg font-medium`、列表 `grid gap-3 sm:grid-cols-2 lg:grid-cols-3`、组 `space-y-2`；卡片是带 `arrived` 的 `Link`，`rounded-lg border p-4 hover:bg-hover`，里面名称（`block truncate`）与开放程度两个 `block` 的 `span`；两组都空时一行"No notebooks yet."与能建的人的"新建笔记本"按钮）。`NotebookGroups` 每项一个 `li.flex.flex-col`（`W/pages/workspace/notebook-groups.tsx:62`），每组一个 `h2` 与以它命名的 `ul`（`:78-89`）。列表的答复里有 `mode` 与 `member_count`（`api/modules/notebook.yaml:505-548`），网页只在无主笔记本的行用成员数（`W/pages/workspace/ownerless-row.tsx:72`）。笔记本首页（`W/pages/notebook/notebook-home.tsx:42-73`）：`section.space-y-4`（`:42`），标题行（`h1.text-2xl` 与能写的人的"新建页面"，`:43-59`），失败的 `Alert`（`:60`），`NotLoaded` 或一行"No pages yet."或 `SubpageList`（`:61-67`），`AttachmentsSection`（`:68`），最后是"笔记本设置"的链接（`:69-71`）；顶层页面是 `pages.childrenOf(null)`（`:40`）。`SubpageList`（`W/pages/page/subpage-list.tsx`）是 `ul.divide-y.rounded-md.border`、在 `ul` 上接管点按（`:45`），每行一个 memo 的 `li > a.block.truncate.px-4.py-2.hover:bg-hover`（`:54-62`），页的子页面一节（`W/pages/page/page-layout.tsx:508`）、标签页（`W/pages/notebook/tag-page.tsx:54`）也用它。`PageTreeStore.tree`（`W/stores/page-tree.store.ts:240-243`）是 `makeAutoObservable` 的 getter，即 computed（`:161-183`）：有 observer 读着它时缓存，首页就是读它的 observer（初稿写成"每次读都重建"，不对，审查 A 的 P5A-N1、C 的 P5C-M3）；`childrenOf(tree, parent)` 在 `W/stores/page-tree.ts:38`。
- **入口**：登录、注册、邀请、引导、新建工作区各是 `section.mx-auto.max-w-sm.space-y-6` 加 `h1.text-2xl.font-semibold`（`W/pages/sign-in.tsx:20-21`、`sign-up.tsx:34-35`、`invitation.tsx:40-41`、`onboarding.tsx:29-33`、`create-workspace.tsx:31-32`）；邀请与新建工作区的 `h1` 取到达的焦点。表单是 `CredentialsForm`（`W/pages/credentials-form.tsx:72-96`，`form.space-y-4`、提交按钮 `w-full`）。邀请页里会话不可用时就地画 `SessionUnavailable`（`invitation.tsx:65`），它自己有一个 `h1`（`W/app/session-unavailable.tsx:24`）：那一刻页上两个 `h1`。产品名只在文档标题里（`W/app/document-title.ts:3` 的 `const app = "Nerve Wiki"`，没有导出）与头像菜单的版本行；`public/favicon.svg` 是 32 px 的墨色圆角方块里一个白的"N"。
- **出错与 404**：`RouteError`（`W/app/route-error.tsx:26-35`）是 `section role=alert`、`h1`、一句说明、错误码与"重新加载"；`SessionUnavailable`（`W/app/session-unavailable.tsx:23-33`）同样是 `role=alert` 的 `section`、`h1.text-xl`、说明、"重试"与"退出登录"；`NotFoundPage`（`W/pages/not-found.tsx:24-32`）是 `section.space-y-2`、取到达焦点的 `h1`（带 `Located` 时也取）、一句说明与不带 `arrived` 的"回到首页"。`Button` 的类名是 `cva` 拼出的（`W/components/ui/button.tsx:6-32`），`Button` 自己再经 `cn` 合并（`:36-38`）；`buttonVariants` 没有导出。
- **列表类的页**（行号与类名逐个列在 4.2、4.3 的表里）：搜索、历史、变更集、回收站、回收站的预览、目录、标签、健康、升级都是 `section` 或 `div` 加 `space-y-4`、`text-2xl` 或 `text-3xl` 的 `h1`，列表是 `ul.divide-y.rounded-md.border`，行是 `p-4`，主链接带下划线；筛选条是 `flex flex-wrap items-end gap-4`，标签与下拉的组是 `space-y-2`。行的第一行今天不是一种样子（审查 B 的 P5B-I2 在 Chromium 里量的）：搜索的命中 14 px、无下划线，历史与目录 16 px（链接在没写字号的 `p`、`li` 里），回收站的行（`W/pages/notebook/trash-row.tsx:500-509`）与变更集的改动（`changeset-change.tsx:106-134`）16 px、有下划线，健康的项（`health-item.tsx:56-74`）14 px、400、有下划线。版本页（`W/pages/page/version-page.tsx:158-201`）、变更集页（`W/pages/notebook/changeset-page.tsx:230-275`）、回收站的预览（`W/pages/notebook/trash-preview.tsx:187-202,336-344,409-460`）的头是面包屑之下一行里的 `text-3xl` 的 `h1` 与它右边的按钮。这三页在头之前另有早退（4.2.6、4.2.8、4.3.1 逐行列出）：版本页 `:76-78`（树读取中或读不到）、`:79-82`（页不在或版本号不合法）；变更集页 `:69`（地址不是 id）、`:92`（别的笔记本的或 404）、`:95`（读取中或读不到）；预览 `:66`（地址不是 id）、`:249`（别的笔记本的项）、`:270`（`page.not_found`）、`:277-288`（不在回收站里）、`:290-301`（已过期）、`:306-308`（读取中或读不到）。`Mark`（`W/pages/page/page-versions.tsx:151-163`）的 `current` 只有右栏用，版本页（`version-page.tsx:193-194`）与历史（`W/pages/notebook/history-ingest.tsx:16`、`history-row.tsx:115`）用普通的那种；历史行的"Reverted"是另写的方角小块（`history-row.tsx:107-113`，`rounded border px-1.5 text-xs underline underline-offset-4`）。右栏的版本行（`page-versions.tsx:109-135`）是"时间 · 名字 · 类型"一块，"当前""编辑中"在行尾自成一列（`ms-auto flex shrink-0`）。
- **列表与文档类的页里通往别处的内容链接**（随页离开，去处的 `h1` 只在 `arrived` 时取焦点）：不带路由状态的有历史行的主链接、"Reverted"与"Reverts …"（`history-row.tsx:64-71,107-113,120`），变更集的改动行的"In the trash""History of this page"与内容的版本（`changeset-change.tsx:120,131,148`），回收站的行（`trash-row.tsx:502-509`），版本页的作者（`version-page.tsx:189`），变更集页的"See it""Reverts …""Reverted by"（`changeset-page.tsx:296,387,398`），升级页通往页面的链接（`upgrade-page.tsx:606`），以及后台的答复丢了、任务读不到时的"See the jobs"（`changeset-page.tsx:344`、`trash-page.tsx:274`、`trash-preview.tsx:361`）。带了的：目录的行（`index-rows.tsx:69-76`）、改动行的主链接与健康的项（`openOf` 的 `state`）、两处"See its progress"（`focusJob`）。
- **附件一节**（`W/pages/page/attachments-section.tsx:211-284`、`attachment-row.tsx:120-161`）：`section.space-y-2.rounded-md`，拖进来时 `ring-2 ring-primary ring-offset-4`（页的阅读视图那一块的拖放区同样，`W/pages/page/attachment-drop.tsx:51`）；`h2.text-sm.font-medium.text-muted-foreground`；"上传"是描边按钮；行 `li.flex.items-center.gap-2.px-3.py-2.text-sm`，类型的图标 `AssetIcon`（`attachment-row.tsx:28-31`）是 `text-muted-foreground`，名称是下载或新标签页打开的链接，行尾"操作"是 `rounded p-1 text-muted-foreground hover:bg-hover` 的按钮。上传中的列表 `UploadRows`（`W/pages/page/upload-rows.tsx:54`）是 `divide-y rounded-md border text-sm`，行 `px-3 py-2`（`:93`）与 `px-3 py-3`（`:176`），附件一节、编辑时的上传（`editor-uploads.tsx`）与粘贴来源的对话框都用它。粘贴来源对话框的文本框（`W/pages/notebook/source-add-dialog.tsx:263-271`）是手写的 `w-full resize-y rounded-md border bg-transparent px-3 py-2 font-mono text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring aria-invalid:border-destructive`，没有 `focus-visible:outline-hidden`（强制色下聚焦时没有指示，审查 B 的 P5B-M10），说明 `text-sm`（`:272-277`）；`Input`（`W/components/ui/input.tsx`）的类名是组件里的字面量。
- **测试**：vitest 228 个文件、5455 格；E2E 640 格：636 过、3 败、1 跳，本机约 2.6–2.8 min（P4 合并之后、`f62d0390` 的树）。本机的三格失败照旧：SE2 [chromium] `se2:231` `Error: expect(locator).toHaveValue(expected) failed`；LW8 [webkit] `lw8:836` `Error: expect(locator).toBeFocused() failed`；PG15 [webkit] `pg15:113` `Error: expect(received).toEqual(expected) // deep equality`（有时过）。偶发的：AS6（`as6:153`、`as6:167` "Response has been disposed"）、LW8 `lw8:771`、负载过 100 时 `markdown.test.ts` 的"not slower"、`trash-preview.test.tsx:587`、造数时接口的 500（`commit transaction: timeout: context deadline exceeded`）、WebKit 刷新之后的"mermaid could not be loaded"。`check.mjs` 列出的格：chromium 431、firefox 103、webkit 103、webkit-narrow 3。`e2e/fixtures/narrow.ts` 的 `expectFits` 拿常量 `phoneWidth`（375）比页宽与盒子的右边（`:16`、`:26`），窗口不是 375 宽时第一句恒红（审查 C 的 P5C-I1）；今天的调用方 17 处都在 375 宽：pg2:184,211、pg17:288,417,654、ui2:169、ui1:54,67、lw3:346,357,364、mc3:202,214,220,226,233、se1:420。
- **设计者的探针**（初稿的 (a)–(f)；修订时另跑的 (g)–(k) 在后面）：
  - (a) **类名能不能生成**（`p5-design/classes.ts` 是初稿第 4 节全部的类名串，`compile.mjs` 以网页自己的 Tailwind 4.3.3 的 `compile` 对每个词单独编一次，与空的输入比）：初稿的 284 个词都生成了；写完初稿之后又从它的引号与反引号里抽出全部像类名的词再编一次（`cands.txt`，251 个），没生成的只有混进来的英文散文、数字、一个以点连写的描述与 `group/row`（只是标记，本来不生成规则）。
  - (b) **初稿的静态参照页**（`page.mjs` 写的 `ref-*.html`，`measure.mjs`、`measure2.mjs` 在 Playwright 1.63 的三种引擎里量）：参照页是手拼的，有几处与正文不同（审查 B 的 P5B-N8：历史行链接多了 `text-sm`、筛选下拉是 `h-7.5`、`revealClass` 没有 `max-md:`、笔记本首页是 720 宽、标题是 `leading-tight`、404 的结构与 `StatusPage` 不同），所以修订时不再以它们为据：下面留下的三条结论在 (h) 里照正文的类名重量过，其余的（版本页 320 宽的一行、回收站预览的长名、行内操作的显出）都由 (h) 取代。参照页留在临时目录，不改，也不再引。
    - 工作区首页的网格只写 `sm:grid-cols-2` 时最窄的那一档是隐式的 `auto` 轨，截短的长名字照样以全长作最小内容撑宽，375 与 320 宽三种引擎都横向溢出；基础的一档写明 `grid-cols-1`（`minmax(0, 1fr)`）之后不溢出（决定 917；审查 B、C 照正文的类名量过，去掉它时页宽 562–567）。
    - Chromium 的强制色下入口卡片的标志隐去（`forced-colors:hidden`）、产品名照旧。
    - 右栏版本一行的"-"：初稿只放了两种名字、两种时间，没有扫宽度，没有复现；这条结论不对，(g) 取代它。
  - (c) **设计基础的守卫**：把第 4 节的类名放进 `nw-p5-design` 的 `src/p5probe.ts` 跑 `design-tokens.test.ts`，红在 `text-center`、`text-balance`、`border-dashed` 三个词（不是颜色类、却以颜色类的前缀起头，`W/design-tokens.test.ts:319-339` 的 `noColour` 里没有）；`fill-primary`、`fill-primary-foreground`、`ring-offset-background`、`ring-accent` 认作主题色、不红。S1 把这三个加进 `noColour`（4.5）。
  - (d) **对比度的推算**（照 00 号文档 4.1 的色值算，没有在应用里量）：暗色的琥珀对 `--background` `#1a1b1e` 7.923、对 `--surface` `#212226` 7.311；暗色的高亮里的字对两者 7.995、7.28；亮色两者都是白，3.098 与 14.642 不变。se1 的命中链接挪进卡片之后，暗色的焦点约 7.31，低于今天的下限 7.82（决定 936）。审查 B 在 Chromium 里实量 7.311。
  - (e) **代码**：`sectionAt`（`W/app/settings-directory.tsx:53-55`）精确比较地址；工作区、个人与笔记本三个设置布局都经它（`W/pages/workspace/settings-layout.tsx:20`、`W/pages/settings/settings-layout.tsx:29`、`W/pages/notebook/settings-layout.tsx:26`）。React Router 8.4 的匹配容得下尾随斜杠，也不分大小写（审查 A 的 P5A-M1），所以带斜杠或大小写不同的地址都走 `shown === undefined` 那一支：在用 `useSettingsHeading` 的四页（令牌、工作区成员、无主笔记本、笔记本成员：`W/pages/settings/tokens-page.tsx`、`W/pages/workspace/members-page.tsx`、`ownerless-page.tsx`、`W/pages/notebook/members-page.tsx`）上是整页报错（"useSettingsHeading is used outside SettingsPage"，落到"Something went wrong"，审查 A 的探针：`/settings/tokens/`、`/lab/settings/members/`、`/settings/Tokens`、`/lab/settings/Members`、`/lab/SETTINGS/members`）；别的页（`profile/`、`general/`、`/settings/Profile`）画出来了，但没有 `h1`、各组之间没有间距。初稿只写了后一种（决定 923）。
  - (f) **测试的清单**（只读，`f62d0390`）：全部 vitest 与 `e2e/stories`、`e2e/fixtures` 里读这些页的文字、角色、结构（`closest`、`parentElement`、兄弟、`:scope`、`locator("section")` 之类）、类名、Tab 与焦点、375 宽与对比度的地方，逐条读过上下文，结果分在第 5 节与 7.1 的表里。读到 `.max-w-178` 的只有 `W/pages/notebook/upgrade-banner.test.tsx:190`；读到 `.rounded-md`、`.divide-y` 的没有。修订时另读的：读右栏版本行文字的 `page-versions.test.tsx:21-29,104,111-141`、`version-restore.test.tsx:157`、hi1:259、hi2:114（"· EditCurrent"连写；`:104` 是中文的"撤销当前编辑中"，修订核对第 1 轮之后补上，审查 A 的 P5A-R1-M2）；读回收站行 `p:last-child` 的 `trash-page.test.tsx:120,147`；读目录行 `p` 的 `index-page.test.tsx:439-440`；读"See the jobs"的 `revert-background.test.tsx:432-435`、`trash-background.test.tsx:388-391,686-689`；`W/app/layout.test.tsx:100-115,184-206`（右上角的"回到工作区"）；`W/test/attachments.ts:21-26` 的 `rows()` 与它的 27 处用处（`attachments-section.test.tsx` 17、`attachment-uploads.test.tsx` 8、`W/pages/notebook/source-uploads.test.tsx` 2）；读 `main` 直接子元素的格（没有：`firstElementChild` 只出现在阅读视图、设置目录与页的编辑的测试里）。
  - 修订时（照审查记录）另跑的：
    - (g) **右栏版本行的"-"**（决定 943；`p5-design/vrow.mjs`，照 B 的 `p5-rev-b/dash6.mjs` 写，`page-versions.tsx:109-135` 的真类名经 (a) 的 `compile.mjs` 编出）：照 `emailFor` 造 40 个名字（`admin|user-<20 位十六进制>-<20 位十六进制>-0-0`）× 中英两种语言的四种时间 × 没有、"当前"、"当前"与"编辑中"三种小块，共 960 行，三种引擎里把右栏的内容宽从 200 到 240 px 逐 1 px 扫（217 px 是 xl 时的宽），数有单独一行"-"的（行, 宽度）对，共 39,360 对。跑了两次（名字随机）：今天的写法（小块在行尾自成一列）Chromium 716、522 对（217 px 时 35、18 行），Firefox 679、391（31、10），WebKit 1632、1605（56、56），全在带小块的行；小块接在"· Edit"之后（两种写法：包在一个 `inline-flex` 的 `span` 里，或每个小块前一个空格、不包），三种引擎两次都是 0 对，也都没有横向溢出。这两次的"编辑中"用的是今天的普通小块；照本文的类名（"编辑中"是 4.3.2 的 `markClass`，审查 B 的 P5B-N8）另跑一次（名字另随机）：今天的写法 Chromium 585 对（217 px 时 47 行）、Firefox 537（23）、WebKit 1444（64），本文的写法三种引擎都是 0 对、没有横向溢出。B 的扫描（只有英文与"当前"）是 Chromium 256、WebKit 920、Firefox 96 对，`wrap-anywhere` 与 `<wbr>` 无效。修订核对第 1 轮之后扫了第四次（`p5-design/vrow2.mjs`，照 B 的 `p5-rev-b/dash7.mjs` 加了只有"编辑中"的一种，共 1280 行、52,480 对，另数断成两半的小块：小块的字落在两行上，审查 B 的 P5B-R1-M2）：今天的写法单独一行的"-" Chromium 1075 对（217 px 时 62 行）、Firefox 647（18）、WebKit 3406（120），小块不断；修订稿的写法（内联，"编辑中"是 `markClass`，不带 `whitespace-nowrap`）"-"三种引擎都是 0，中文的小块却断成两半：Chromium 1740 对（217 px 时 0）、Firefox 2752（28 行）、WebKit 1092（0），英文 0；`Mark` 的两串都加 `whitespace-nowrap` 之后，"-"、断开的小块与横向溢出三种引擎都是 0。B 的扫描是断开的小块 Chromium 1920、Firefox 2756（217 px 时 8 行）、WebKit 1224，加了之后 0。所以改法进 S3（4.3.2），连同 `whitespace-nowrap`。
    - (h) **照正文类名的探针页**（`p5-design/rev.mjs`：类名逐字抄自第 4 节，`Button` 的类名照 `buttonClass` 合并之后的写；`rev/out.css` 由 (a) 的 `compile.mjs` 编出，164 个词只有 `group/row` 不生成规则；三种引擎，宽 320、375、600、640、1024、1440）：
      - 笔记本首页的标题行放 60 个字符、没有空格的名字（初稿写"连字符接数字"，探针里其实是连字符接字母的 `n000-n001-…`，Firefox 在那里也断，量不出变异 62，审查 C 的 P5C-R1-M3；照 UI1 的 `longName` 连字符接数字、`titleTextClass` 的重量在 (l)）：`h1` 在 1024、375、320 宽折成 2、3、4 行，"新建页面"在它之下、右边与行的右边齐（差 0 px），都不溢出；子页面列表里同样长的名字一行、截短（`scrollWidth > clientWidth`），1024 宽放得下时不截。
      - 工作区首页：长名字在卡片里一行、截短（320 到 1440 都是）；卡片在 640 宽起 112 px 高，375、320 宽一列时随内容 97 px，虚线卡 56 px（`sm:min-h-28`，审查 B 的 P5B-M2）；不溢出。
      - 版本页的头一行：能编辑的人那一格（`flex-[0_1_10rem]`、`min-h-7.5`）有没有"还原这个版本"，`h1` 的上边在 320、375、600、1024 宽都一样（110、110、90、78 px），320 宽时"Version 1"一行（审查 A 的 P5A-M4，4.3.1）。
      - 回收站的预览：标题里同样的长名字在 1440、375、320 宽折成 2、4、6 行；"恢复"与"在后台恢复"一组在标题之下时与行的右边齐（`ms-auto`，审查 B 的 P5B-M7、Q2）。带名字的"在后台恢复"先照 B 的写法只加 `h-auto min-h-7.5 py-1 whitespace-normal text-start wrap-anywhere`：375 宽时它仍是一行、高 30 px，`scrollWidth` 不溢出，可是 `Button` 的 `shrink-0` 让它在 `justify-end` 的一行里向左伸出屏幕（页面不横向滚动，左边看不见；探针第一次只量了右边，漏看了左边）；再加 `shrink` 之后在 375、320 宽折成三行、高 69 px，左边 24 px，不溢出（4.2.8）。
      - 行内操作的显出（Chromium、Firefox；`transition-[opacity,color,background-color]`）：静止 0，焦点在这一行的链接上 1，`aria-expanded="true"` 1，767 宽 1。筛选的小块 `rounded-[18px]` 放两倍长的名字，375 宽折成多行（100 px 高）时圆角仍是 18 px，不溢出。横幅的小圆点 6 px、3 px 的琥珀边（亮色 `rgb(194, 135, 31)`）。（这一页的标题行与小圆点是修订时的类名，(l) 照再修订的重量。）
    - (i) **头一行三串从 `page-layout.tsx` 导入时的分包**（决定 925 与过度设计一条）：在 `nw-p5-rev` 里以 `vite build --manifest` 构建两次，一次照 `f62d0390`，一次让 `page-layout.tsx` 导出头一行的串、`changeset-page.tsx` 与 `trash-preview.tsx` 各导入它（`closure.mjs` 数每个路由的块静态导入的闭包）：变更集页的闭包从 44 块 976,309 字节变成 47 块 1,047,564 字节，预览从 980,922 变成 1,044,506（多出 `page-layout` 一块 58,109 字节、gzip 19,509，与 `attachments-section`、`subpage-list`、`export-dialog` 等几块）；版本页本来就导入 `page-layout.tsx`，不变。这是"在原处导出、同一个来源"的代价，照处置接受（决定 925），写进 4.5。
    - (j) **设计基础的守卫，修订之后**：把修订的类名（67 串）放进 `nw-p5-rev` 的 `src/p5probe.ts`，经 `heavy.sh` 跑 `design-tokens.test.ts`：只红在 `border-dashed`、`text-center`、`text-balance`（同 (c)）；横幅小圆点的 `border-3 border-accent`、文档类事实的 `[&_a]:text-muted-foreground`、拖放区的 `ring-accent`、"amber is no fill"都不红。
    - (k) **2.1 与 8.2 的替换**（`p5-design/replay.py`，每次修订之后跑）：在 `3e288acb`（设计提交落在它上面；2.1 第 12 条的旧文字是它加的 CI 修复那一行的末句，`f62d0390` 上没有）的 `00-M13-design.md` 与 `v0.1-design.md` 的副本上先做 2.1 的十四条（十五处），再在 2.1 做完的总体设计上做 8.2 的十六条（十七处）：每段旧文字在做到它时恰好一次，每段新文字在它那一步之后恰好一次，32 处都是（见 2.1、8.2 的头一段）；修订核对第 1 轮之后再修订，又在新的副本上跑了一次，同样 32 处。
    - (l) **再修订的类名**（`p5-design/rev2.mjs`，修订核对第 1 轮之后跑；类名逐字抄自 4.1.2、4.2.3、4.2.8、4.3.6，`rev2/out.css` 由 (a) 的 `compile.mjs` 编出，102 个词只有 `group/row` 不生成规则，`mt-[calc(0.5lh-3px)]` 生成；三种引擎，中英两种文字）：
      - 标题行（`titleTextClass`，审查 B 的 P5B-R1-M1、C 的 P5C-R1-M3）：短名字（"Plans""计划"）的笔记本首页、健康页的"重新检查"、短名字的回收站预览，在 320、375、390、640、1024 宽都与按钮同行；中长的名字（"Research notes for the spring""春季调研的笔记与资料"）在 640 px 以下占满一行、按钮在下一行靠右（与行的右边差 0 px），640 起同行；照 UI1 的 `longName` 连字符接数字的 60 个字符，320、375、390 宽折成 4、3、3 行、按钮在下面靠右，640、1024 宽在按钮旁折两行；都不横向溢出。
      - 横幅的小圆点（`items-start`、`mt-[calc(0.5lh-3px)]`，审查 B 的 P5B-R1-N1）：状态一行在 320、375、1024 宽折成 4、3、2 行（中英同），点的中心离行顶都是 10 px，正是 20 px 行高的一半。
      - 历史行的第一行（"只看此人"加 `-my-1`，审查 B 的 P5B-R1-N5）：1440 宽时 22 px、到第二行 26 px；搜索的行 20、24 px（B 量的不加时是 28 对 23 px）。
  - 没探的：应用里的整套行为（Step 的代码没有写）；(h) 是静态页，不是应用；对比度只推算了，没有在应用里量（S2 的 se1 量）；Linux 的字体下 (g)、(h) 的行数。

## 2. 目标与范围

照 00 号文档 4.6 排工作区首页、笔记本首页、登录、注册、邀请、引导、新建工作区、出错与 404，以及搜索、历史、变更集、回收站与预览、目录、标签、健康、升级、版本、附件一节与对话框的内容；建空状态的组件（4.3）。不改接口与服务端，不改文字的意思（只加四条文字：成员数与子页面数，各有一个与多个的两条，4.1.3、4.1.4），不改路由的地址。

不做：骨架（决定 914）；搜索摘要里的 Markdown 原文（00 号文档第 2 节，v0.2）；导入导出页本身（P3/S2 已换成卡片，P5 只管它的两个对话框里的框，决定 939）；对比视图（`W/editor/text-diff.tsx`）的样子（P4/S3 已换）；右栏里的空的一行灰字（决定 938）。

### 2.1 对 00 号文档的修订

下面各条由主持者随本文的设计提交照这里的文字落进 [00 号文档](00-M13-design.md)（第 1–12 条）与总体设计（第 13、14 条，`docs/v0.1/v0.1-design.md`）；设计者只写了本文。脚本依次做这些替换，断言每段旧文字在做到它时的文件里恰好出现一次、每段新文字在它那一步之后也恰好一次。"换成"是旧文字换成新文字，"之后加"是新文字接在旧文字之后，"之后另起一行加"是旧文字之后先换行再接新文字。引号照文件里的写法是 ASCII 的 `"`。引用块里写在反引号里的链接（审查记录还没有写进仓库，总体设计的链接相对于 `docs/v0.1/`）写进去时去掉反引号，同 8.2。设计者已用这样的脚本（`p5-design/replay.py`，会话的临时目录，不进仓库）在 `3e288acb` 的两个文件的副本上依次做过这十四条的十五处（第 12 条的旧文字是 `3e288acb` 加的 CI 修复那一行的末句，`f62d0390` 上没有；修订核对第 3 轮之前各次在 `f62d0390` 上做），每段旧文字都恰好一次、每段新文字都恰好一次（修订时照审查 C 的 P5C-I3 改成这样的写法；初稿是散文，第 1 条的"五个 Phase、12 个 Step"在 00 号文档里有两处，`:344` 与 `:385`，第 7、9 条引的旧文字用了中文的引号，照写都不成）。00 号文档的行号照它的头是 `3c315037` 的，不改（今天的在第 1 节）。第 10–12 条（头的状态、第 12 节 P5 一行、第 13 节的一行）是设计提交的记录，同 P4；它们在设计定稿之后落进去。

1. **第 7 节的开头**（决定 912；`:344`，`:385` 的那一处由第 3 条加注）：旧文字

   > 五个 Phase、12 个 Step，**串行**

   换成：

   > 五个 Phase、13 个 Step，**串行**

2. **第 7 节的表里 P5 的两行**（决定 912、915）：旧文字

   > | **P5 其余页面** | S1 首页与入口 | 工作区首页、笔记本首页、登录、注册、邀请、引导、新建工作区、出错与 404；空状态的组件（4.3，第一批用处是两个首页） | 引导、登录的故事；375 px |
   > | | S2 列表类页面 | 搜索、历史与变更集、回收站与预览、目录、标签、健康、升级、导入导出、版本与对比、附件一节；各对话框的内容排版；这些页面的空状态；骨架（只在某个列表确实需要时加，4.3） | 这些页面的故事全绿 |

   换成：

   > | **P5 其余页面** | S1 首页与入口 | 工作区首页、笔记本首页、登录、注册、邀请、引导、新建工作区、出错与 404；空状态的组件（4.3，第一批用处是两个首页与设置里的空列表）；P4 交来的 `sectionAt`；升级横幅的"路由 × 支"表（[05-P5](05-P5-pages.md) 4.3.6） | 引导、登录的故事；UI1 两格新的（375 px，三种浏览器） |
   > | | S2 列表与筛选 | 搜索、历史与变更集、回收站与预览、目录、标签；筛选条、同一种行与行内操作的显出；这些页面的空状态 | 这些页面的故事全绿；TB1 一格新的 |
   > | | S3 版本、健康、升级、附件与对话框 | 版本与对比、健康、升级、附件一节；各对话框的内容排版；升级横幅总在列里（[05-P5](05-P5-pages.md) 决定 912、915） | 这些页面的故事全绿 |

3. **"总体设计的修订"第 1 条**（决定 912）：旧文字

   > M12 一行的"发布 v0.1"后注"（发布在 M13 收尾之后，负责人 2026-11-22）"。

   之后加：

   > P5 的设计把 P5/S2 拆成两个之后是 13 个 Step，12.2 那一行随 P5 的设计提交改成"13 个 Step"，记进总体设计第 15 节（[05-P5](05-P5-pages.md) 决定 912）。

4. **第 9 节的两处"P5/S2"**（决定 912）：旧文字

   > 全部 e2e 在 P5/S2 合并之后

   换成：

   > 全部 e2e 在 P5/S3 合并之后

   旧文字

   > P5/S2 之后记一次时间

   换成：

   > P5/S3 之后记一次时间

5. **4.3"加载"一条**（决定 914）：旧文字

   > a15:279 与 9 个 vitest 文件等它。

   之后加：

   > P5 没有加骨架：列表的第一次读取都短，骨架只会多一次闪动（[05-P5](05-P5-pages.md) 决定 914）。

6. **4.2.3 的末句**（决定 921、941）：旧文字

   > 已登录的邀请页与出错页回去的路留给 P5。

   换成：

   > 已登录的邀请页在账户已有工作区、引导已走完时同样放它（[05-P5](05-P5-pages.md) 决定 941）；出错页与 404 回去的路是页里的"回到首页"（决定 921）。

7. **4.6 第一条的虚线卡**（决定 918、942）：旧文字

   > "新建笔记本"是卡片列表最后一张虚线卡（新建照旧经工作区 `nav` 里的按钮，虚线卡只是同一个对话框的另一个入口）。

   换成：

   > "新建笔记本"是两组之后自成一行的一张虚线卡，只给能建笔记本的人，两组都空时不画（它不是哪一组里的笔记本；新建照旧经工作区 `nav` 里的按钮，虚线卡只是同一个对话框的另一个入口；[05-P5](05-P5-pages.md) 决定 918、942）。

8. **4.6 第四条的页**（决定 929、939、945）：旧文字

   > **历史、回收站、目录、标签、健康、升级、导入导出、版本**：统一成"页面标题 + 筛选条 + 卡片里的行"；

   换成：

   > **历史、回收站、目录、标签、健康、升级**：统一成"页面标题 + 筛选条 + 卡片里的行"（版本页、变更集页与回收站的预览照页的头一行排：面包屑与页级的操作在上，32 px 的标题与 720 px 的一列；导入导出是笔记本设置的一页，P3/S2 已换成设置的卡片；[05-P5](05-P5-pages.md) 决定 929、939、945）；

9. **4.6 最后一条**（决定 921）：旧文字

   > 居中的图标、一句说明与"回到首页"

   之后加：

   > （出错页另有"重新加载"，会话不可用的页是"重试"与"退出登录"，[05-P5](05-P5-pages.md) 决定 921）

10. **头的状态**（设计定稿之后）：旧文字

    > P4 完成，P5 待开

    换成：

    > P4 完成；P5 设计定稿

11. **第 12 节 P5 一行**（设计定稿之后；Step 三个，同 P4 的写法）：旧文字

    > | P5 其余页面（S1、S2） | — | 未开始 | — | — |

    换成：

    > | P5 其余页面（S1–S3） | [05-P5](05-P5-pages.md) | 设计定稿；S1 实施中 | `[设计审查](reviews/P5-design-review.md)` | — |

12. **第 13 节的一行**（设计定稿之后；在 CI 的两处修复那一行之后，04-P4 10.4）：旧文字

    > 只改测试，由主持者核对。

    之后另起一行加：

    > - 2026-11-25：照 P5 的设计修订（[05-P5](05-P5-pages.md) 2.1；`[设计审查](reviews/P5-design-review.md)`，主持者的决定 912–945）：P5 分成三个 Step（第 7 节的开头、表与"总体设计的修订"第 1 条的注，第 9 节的两处）；4.2.3 已登录的邀请页的"回到工作区"；4.3"加载"不加骨架；4.6 的虚线卡、文档类的三页与导入导出、出错页的去路；头的状态与第 12 节 P5 一行。

13. **总体设计 12.2 的 M13 一行**（决定 912；`docs/v0.1/v0.1-design.md:963`；00 号文档"总体设计的修订"第 1 条说那一行在设计定稿的提交里改，P5 照此）：旧文字

    > 中（五个 Phase、12 个 Step）

    换成：

    > 中（五个 Phase、13 个 Step）

14. **总体设计第 15 节**（审查 C 的 P5C-M6：历来改 12.2 都记一行，例如"M12 开工：12.2（M12 五个 Phase、17 个 Step，中）"）：旧文字：`## 15. 变更记录` 与其后的空行。

    之后加：

    > - 2026-11-25：M13/P5 的设计（依据 `[P5 设计](M13-ui-refresh/05-P5-pages.md)`）：12.2（M13 五个 Phase、13 个 Step）。

### 2.2 需要负责人确认

无。

### 2.3 本文的决定（主持者的决定，负责人可以改判）

912–939 是初稿的；修订时照设计审查改了其中 913、915–925、927–935、937 的文字（各条里写明依据，审查的编号如 P5A-I1）。940–945 是主持者照设计审查定的（审查记录"主持者的决定"），写法由设计者补在第 4 节与这里。修订核对第 1 轮之后再修订（三份报告全部接受，没有新的决定）：919、925、930、932、935、940、941、943 的文字照它改了（各条里写明依据，报告的编号如 P5A-R1-M1）。修订核对第 2 轮之后再修订（没有新的决定）：941 的文字照它改了（竞态的一格先等邀请页发出的那次读，审查 A 的 P5A-R2-M1、C 的 P5C-R2-M1）。修订核对第 3 轮之后主持者改（没有新的决定）：941 的文字（否定的两格在旧树上红在等列表那一步，审查 A 的 P5A-R3-M1、C 的 P5C-R3-M1）与 944 的文字（M13 在 CI 上跑过一次，run 45551442052，C 的 P5C-R3-M2）。946–951 是 S1 的实施审查之后的（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)"主持者的决定"，括注是发现的编号）；照它们与 S1 的实施又改了 915、922、923、944 的文字（各条里写明"S1 的实施与审查之后"）。

- **912 三个 Step**（第 0 节）：00 号文档的 S2 拆成 S2"列表与筛选"与 S3"版本、健康、升级、附件与对话框"。理由：十一页、一百多处读它们的测试放进一次审查太大；分开之后两个 Step 各有一套共用的东西（S2 的筛选条与行，S3 的跨页部件），读它们的测试不交叉。全部 e2e 的那一次完整跑随之在 P5/S3 之后（2.1 第 1–4、13、14 条）。
- **913 空状态的组件**（4.1.1）：`W/components/empty-state.tsx` 的 `EmptyState`，图标（`aria-hidden` 的方块里一个 lucide 图标）、一句说明（`<p>`，就是今天那一行的文字，不改）、主操作（可选，调用者给的按钮）；没有标题（00 号文档 4.3 只要"图标 + 一句说明 + 主操作"）；两种大小：`page`（页上的列表，居中、上下留白大）与 `card`（设置的卡片里、代替今天的 `cardTextClass` 那一行）。它不是 `status`，不播报：说明今天就不是，播报的照旧是各页自己的 `<output>`。图标方块的类名 `iconTileClass` 在 S1 提交 2 只在本文件用、不导出，提交 3 的 `StatusPage` 用它时才导出（审查 A 的 P5A-I2、C 的 P5C-M1：knip 照默认配置报只在本文件用的导出）。
- **914 不加骨架**：00 号文档 4.3 只许"某个列表确实需要时"加。P5 的列表第一次读取都短（同一个请求，没有分批的长列表），读取中照旧是 `Loading` 的一行"加载中…"（a15:279 与九个 vitest 文件等它）；骨架只会让短的读取多闪一次。
- **915 每个笔记本页的每一支都在列里**（4.3.6）：P5 排到哪一页，就给它的路由加 `handle: { column: true }`，在它的列的最上面画 `<ColumnTop />`，读取中、读不到、找不到的各支同样（P4 的决定 857 的做法）。只有一个根的页（笔记本首页、历史、回收站、目录、标签、健康、升级）各支本来就在根里；有早退的（页、版本页、变更集页、回收站的预览）每一处早退都包进一个列：`page-layout.tsx` 的 `PageColumn`（`mx-auto w-full max-w-180` 的 `div`，第一个子元素是 `<ColumnTop />`）（S1 的实施与审查之后照决定 946、951 排：外层同页的外层，列之前一个空的头一行，页的两支传 `panel`，xl 且右栏开着时留出右栏的地方），4.2.6、4.2.8、4.3.1 逐行列出。P4 的页的两支（`page-layout.tsx:77-93`）今天是片段，横幅的父元素是出口的容器、在 `main` 的全宽里，S1 一并包进 `PageColumn`：树读到之前与之后横幅都在同一列里，不再从全宽跳进 720 的列；"横幅的父元素是那一页的列的根"也才对每一支都断言得了（审查 A 的 P5A-I1、C 的 P5C-M5）。S3 的最后一个提交里，笔记本下的每个子路由的每一支都已在列里，`NotebookLayout` 的回退（`:97` 的横幅与 `:67-70` 的判断）与 `handle: { column: true }` 一起删掉，横幅总经 `ColumnTopContext` 交给页（`settings: true` 留着，`Layout` 的设置模式读它）。理由：回退那一支在 S3 之后不会再被走到，留着它，横幅画在哪就有两条路。守它的是 4.3.6 的"路由 × 支"表：从 S1 起只有一个遍历的格，每个 Step 加自己的路由与各支，加的时候先红。
- **916 列宽**：工作区首页的列最宽 928 px（`max-w-232`，宽屏三列卡片，每张约 300 px）；笔记本首页、列表类的页与版本、变更集、回收站的预览最宽 720 px（`max-w-180`，同页的内容列）；居中。`main` 的留白照旧（P4 的决定 855）。笔记本首页初稿也是 928，修订时改成 720（审查 B 的 P5B-M1）：它没有卡片的网格，只有一列行与附件一节，928 宽时行尾的子页面数离名字约 850 px，进一页时内容列从 928 变 720、左边跳 104 px，附件一节在首页与页的底部也不一样宽。列表类的页的列上方另有 `md:pt-5`（同设置的列，标题与左栏顶部的工作区切换差不多齐）；版本等三页照页的外层（`md:-mt-3 md:pb-4`），头一行与页的头一行同高。
- **917 工作区首页的卡片**（4.1.3）：每张卡片一个书本图标的方块、名称（一行，截短；名字的 `span` 带 `title`，审查 B 的 P5B-N6）、LLM Wiki 的徽记（`badgeClass`，模式是 `llm_wiki` 时）、一行灰字"开放程度 · N 位成员"（`WholeParts`；数照语言区写，`toLocaleString`，审查 A 的 P5A-N4）；640 px 起卡片高至少 112 px（`sm:min-h-28`），同一行的卡片一样高，一列时随内容高（375 宽约 97 px，审查 B 的 P5B-M2）；网格最窄时写明一列（第 1 节 (b)）。不显示页数（答复里没有）与"最近更新"（`updated_at` 不是内容的时刻，00 号文档 4.6）。成员数是新的两条文字（`notebooks.membersOne`、`notebooks.members`）。
- **918 虚线的"新建笔记本"**：两组之后自成一行（同样的网格，在第一列），不进任何一组的列表。理由是语义：它不是一本笔记本，进了"我的""团队"哪一组都不对，也说不清新建的那本会在哪一组（决定 942；初稿另说"进了列表会多一个列表项、`workspace-home.test.tsx:36-37,48,60-61` 数每项一个链接"，不成立：那几格数的是链接，`li > button` 不多一个链接，审查 C 的 P5C-M2）。只给能建笔记本的人（`createsNotebooks`），按下打开左栏那个按钮的同一个对话框（`CreateNotebookDialog`），名字同是"新建笔记本"。两组都空时不画它，空状态自己带那个按钮。它的对话框开着时列表被别处变空（另一位管理员删了最后一本、或本人被移出），它随卡片卸下、对话框连同打了的名字一起没了：与今天反方向的那一种（空状态的对话框开着时别人建了一本）相同，不另处理（审查 A 的 P5A-N7）。
- **919 笔记本首页**（4.1.4）：标题行是 26 px 的 `h1` 与右边 `size="sm"` 胶囊的"新建页面"（同页的头一行里的"编辑"）；名字长时 `h1` 取按钮留下的宽（`titleTextClass`），按钮留在右边，排不下时在标题之下、仍靠右（`ms-auto`，审查 B 的 P5B-M7）；640 px 以下标题只有它的字那么宽，短名字时按钮照旧在它旁边（只用 `basis-56` 时 224 px 加按钮宽过 375 宽时的列，"Plans"也让按钮掉下去，审查 B 的 P5B-R1-M1）；顶层页面的列表每行一个页面图标，有子页面的在行尾一个灰的数（子页面数），数在链接之外（`page-tree-scale.test.tsx:147,239-240` 与 `e2e/fixtures/wiki-pages.ts:174-180` 读链接的文字），看得见的数 `aria-hidden`，读屏读的是链接的描述（`aria-describedby` 指向这一行里一个 `hidden` 的句子"3 个子页面"）：按 Tab 走链接、打开读屏的链接列表也听得到，浏览模式不读两遍（审查 A 的 P5A-N6）；数与句子照语言区写（`toLocaleString`，P5A-N4）。树在渲染里读一次（`pages.tree`），各行的数由它算：一次读，意思清楚（初稿说"经 `pages.childrenOf` 一千个顶层页就重建一千次索引"，不对：`tree` 是首页这个 observer 读着的 computed，两种写法等价，变异 15）。页的子页面一节与标签页用同一个 `SubpageList`，有图标、没有数（00 号文档 4.6 只说首页）。"笔记本设置"的链接去掉（00 号文档 4.6）。没有页面时是空状态（页面图标、"还没有页面。"），不带按钮："新建页面"已在标题行（两个同名会让 `page-tree-writes.test.tsx:660,668,692` 一类的查找撞名）。
- **920 入口的卡片**（4.1.5）：登录、注册、邀请、引导、新建工作区五页各是一张居中的卡片，宽 400 px（`max-w-100`），卡片里最上面是标志与产品名"Nerve Wiki"（标志是 `favicon.svg` 的两条路径，内联的 `svg`，方块 `fill-primary`、字 `fill-primary-foreground`，亮暗随主题；`aria-hidden`；强制色下隐去，产品名的字照旧），其下是各页今天的内容。产品名是普通文字（不是标题：很多测试要页上只有一个 `h1`）；它取自 `W/app/document-title.ts` 导出的 `appName`（今天的局部常量），文档标题与它同源。卡片与出错、404 的标题是同一串 22 px 的类名（`entryTitleClass`，`entry-card.tsx` 导出，`StatusPage` 的用处也取它；初稿两个模块各一份一字不差的串，审查 B 的 P5B-N3）。
- **921 出错页与 404**（4.1.6）：居中的图标方块、22 px 的 `h1`、一句说明与操作。404：`FileQuestionMark`，"回到首页"（描边胶囊样子的链接，带 `arrived`）；出错页：`TriangleAlert`，错误码一行照旧，"重新加载"与"回到首页"两个（"回去的路"是 00 号文档 4.2.3 留给 P5 的）；会话不可用的页：`CloudOff`，"重试"与"退出登录"照旧，不加"回到首页"（首页也要会话）。链接的样子取 `button.tsx` 新导出的 `buttonClass(...)`：`cva` 只把各变体的类名拼起来、不经 tailwind-merge，裸的 `buttonVariants({ variant: "outline", size: "sm", shape: "pill" })` 里同时有 `rounded-lg` 与 `rounded-full`、`text-sm` 与 `text-compact`，样式表里 `rounded-lg` 在后，链接成了 8 px 的圆角（审查 B 的 P5B-I1，三种引擎实量）；`buttonClass` 是 `cn(buttonVariants(...), className)`，`Button` 自己也改用它，`buttonVariants` 照旧不导出。`role=alert` 照旧在包着 `h1` 的 `section` 上（`route-error.test.tsx:24-26,47,70-73`、`layout-error.test.tsx:17`、`guards.test.tsx:63,80,154,236` 读整个 `alert` 的文字，里面要有 `h1`）。已登录的邀请页的回去的路是右上角的"回到工作区"（决定 941；初稿说"它的两个按钮就是去路、右上角有账户菜单"，不成立：两个按钮一个是接受、一个是退出登录，账户菜单里没有回工作区的项，审查 A 的 P5A-Q2）。
- **922 邀请页里的会话不可用是 `h2`**：`SessionUnavailable` 多一个 `level`（`"h1"` 默认、`"h2"`），邀请页就地画它时给 `"h2"`，页上只有邀请的 `h1`；单独成页（`SessionUnavailablePage`）照旧 `h1`。`layout.test.tsx:239` 以名字找这个标题、不看级别，照旧过。 S1 的实施与审查之后（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：`SessionUnavailable` 不收 `level`，只有邀请页用的 `h2` 一支（`h1` 那一支没有调用者，审查 A 的 P5S1A-N3、C 的过度设计）；会话不可用拆成钩子 `useUnavailable` 与两个外壳，单独成页的 `SessionUnavailablePage` 用 `StatusPage`。
- **923 `sectionAt` 照路由的匹配比较**（4.1.7）：比较之前把地址末尾的 `/` 去掉、两边都转小写。理由：React Router 的匹配容得下尾随斜杠、不分大小写（第 1 节 (e)），路由画出了哪一页，设置的目录与 `useSettingsHeading` 就该认得哪一页；今天带斜杠或大小写不同的地址在用 `useSettingsHeading` 的四页上是整页报错，别的页没有 `h1`（审查 A 的 P5A-M1）。转小写不会多认什么：工作区的 slug 与笔记本的 id 大小写不同时，外框在走到设置布局之前已经 404 或照它自己的规矩找。不取"设置的子路由带 `caseSensitive: true`"：那会把今天打得开的地址变成 404，又要在三棵设置的路由上各加一处，改的是路由的行为，不是 P4 交来的这一处。初稿只去斜杠、不改大小写，理由是"slug 与 id 大小写不同时外框找不到"，管不到 `settings` 这一段与页这一段，个人设置的地址里也没有 slug。 S1 的实施与审查之后（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：用路由的 `matchPath` 比较（`path` 是 `${base}/${path}`，`caseSensitive: false`、`end: true`），比较之前像路由一样逐段解码（路由不导出它的 `decodePath`，本地的 `decodedPath` 同样写：逐段 `decodeURIComponent`，段里解出的 `/` 写回 `%2F`，解不了时用原样）：带一个或几个尾随斜杠（路由的匹配给结尾加 `\/*$`）、大小写不同或带编码的地址都是那一页（审查 C 的 P5S1C-M1、A 的 P5S1A-N2）；审查 A 拿路由自己的匹配对照三个设置区各 26 种写法、共 260 个地址，0 处不一致。
- **924 通往设置的内容链接**（03-P3 第 9 节交来）：工作区首页无主笔记本提醒里的"查看"（`workspace-home.tsx:87`）加 `arrived`：它离开工作区的外框进设置，左栏随之换成设置目录，不带时焦点落到 `body`；404 与出错页的"回到首页"同样带（`LandingPage` 把它转交给自己的 `<Navigate>`，工作区首页的 `h1` 取焦点）。后台的答复丢了、说不出是哪个任务时的三个"See the jobs"（`changeset-page.tsx:344` 的 `startUnread`、`trash-page.tsx:274`、`trash-preview.tsx:361` 的 `LostStart`）没有路由状态，同样离开工作区的外框进设置，都加 `arrived`（S2），导入导出页的 `h1` 取焦点（初稿只列了预览的一处，审查 A 的 P5A-M2）。变更集页与回收站的"See its progress"（`changeset-page.tsx:331`、`trash-page.tsx:259-265`）照旧：它们带 `focusJob`，任务的那一行取焦点，比 `h1` 更近。笔记本首页的"笔记本设置"去掉了（决定 919）。
- **925 列表类的页的格局**（4.2.1）：列（决定 916）里依次是 `<ColumnTop />`（笔记本里的页）、标题行（26 px 的 `h1`，页级的操作在右，如健康的"重新检查"）、一两行说明（13 px 灰）、筛选条、状态行（照旧的 `<output>`）、卡片里的行。卡片与行用 P3 的（03-P3 第 9 节"P4、P5 的卡片"）：`SettingsCard` 能用的（一个只带 `aria-label` 的 `ul`）就用它，别的（带 `aria-labelledby`、`onClick` 或别的属性的列表）用 `W/app/settings-page.tsx` 新导出的 `cardClass`（就是 `SettingsCard` 今天的局部常量，`SettingsCard` 改用它）；左右排的行（回收站的项）是新的 `splitRowClass`（P3 的 `listRowClass` 是 `items-center`、至少 56 px 高，回收站的项左边有几行事实，要 `items-start`），它左边的文字一块用 P3 的 `listTextClass`，"恢复"在右边，窄屏时在文字之下、靠右（审查 B 的 P5B-M6、P5B-R1-N7）；上下排的几行是新的 `stackedLinesClass`（P3 的 `stackedRowClass` 的 `space-y-3` 是"一行加一块"的，这里是几行文字）。各页的行是同一种：第一行 `lineTitleClass`（14 px、500，同设置的行），行的主链接 `lineLinkClass`（不加下划线、悬停才加，4.7"列表里的链接不加"），其下的事实 `factsClass`，内容的摘录（搜索的摘录、目录的摘要）`text-sm`；初稿只有搜索这样写，历史与目录 16 px、回收站与变更集的改动 16 px 带下划线、健康的项 400 带下划线（审查 B 的 P5B-I2）。`h1` 的类名是 `settings-page.tsx` 新导出的 `pageTitleClass`（`SettingsPage` 的 `h1` 去掉 `mb-7` 的那一串，`SettingsPage` 改用 `cn("mb-7", pageTitleClass)`）。`cardClass` 与 `pageTitleClass` 在 S1 导出（首页先用）。列、标题行、说明、筛选条、行、文档类的头的右边一组与事实、行内操作的类名放在新的 `W/app/page-classes.ts`（S1 建、S2 加）：它们不是设置的，也不是组件（只是类名，同 P3 的做法）。P4 的页的外层、头一行、面包屑、列与标题的串不搬：在 `page-layout.tsx` 原处写成导出的常量（同一串，生成的类不变），文档类的三页从那里导入，同一个来源（初稿把它们挪进 `page-classes.ts`、再改 `page-layout.tsx` 去用，审查 B 的"过度设计"一条：好处只是同一个来源，却要再动一次 P4 已核对过的文件的结构）。代价是变更集页与回收站的预览的块要静态导入 `page-layout.tsx` 的块（第 1 节 (i)：闭包多约 64–71 KB，gzip 之后 `page-layout` 一块约 19.5 KB）；版本页本来就导入它。`page-classes.ts` 不从 `page-layout.tsx` 再导出：那样列表类的七页都会拉进 `page-layout.tsx` 的块。
- **926 筛选条与标签组**（4.2.2）：筛选条 `flex flex-wrap items-end gap-x-3 gap-y-3`；每个"标签 + 下拉"的组 `flex min-w-0 max-w-full flex-col gap-1.5`，标签总在下拉之上（03-P3 第 9 节说的"可能并排"由 `flex-col` 排除），间距同 `FormField`（01-P1 第 9 节"P2–P5"，P5 的七处：`W/pages/search/search-page.tsx:221`、`W/pages/notebook/index-filters.tsx:48,60`、`W/pages/notebook/history-filters.tsx:132,143,154`、`W/pages/notebook/source-add-dialog.tsx:261`，最后一处在对话框里，照旧 `space-y-1.5`）。下拉照旧 36 px（筛选不是工具栏）；历史的"谁"至多 256 px（`max-w-64`，e2e 式的名字让它宽到 382 px，筛选条参差不齐；`NativeSelect` 的 `contain-paint` 在，截掉的名字不撑宽页，审查 B 的 P5B-N7）。页的筛选小块的圆角是 18 px（它最矮时高的一半），名字长、折成几行时是圆角的方块，不是胶囊（审查 C 顺手看到的：`rounded-full` 折行之后字贴着弧形的边）。
- **927 行内次要操作的显出**（4.2.3）：历史行的"只看此人"与附件行的"操作"平时透明（`opacity-0`），不藏起、不 `inert`：Playwright 只点看得见的元素，透明的照样看得见（hi3:328-331 悬停之后点它）；悬停这一行、键盘的焦点在这一行里（含它自己）、它的菜单开着（`aria-expanded`）时显出；没有悬停的设备、窄屏（< 768 px）与强制色下一直显示（同左栏的齿轮，`W/pages/notebook/page-tree.tsx:39-40`）。过渡写 `transition-[opacity,color,background-color]`（同左栏的齿轮）：`cn` 把 `transition-opacity` 与 `Button` 的 `transition-colors` 当作同一组，留下前者，悬停时底色与字色的过渡就没了（审查 A 的 P5A-N2、B 的 P5B-N1）。别的行内操作（回收站的"恢复"、目录与健康的链接）照旧一直显示：它们是这一行的主操作。
- **928 搜索**（4.2.4）：结果在卡片里，每行照旧标题（`p`，`font-medium`：se1:340 读它的字重）、位置、摘要、时刻；命中的链接不加下划线（4.7：列表里的链接不加），悬停时加。没有命中时照旧只有状态行说"没有找到。"（`W/pages/search/search-status.ts:24`），不另画空状态：两处同一句会让找它的查找撞名；没有提交时的那一行说明（`search.howTo`）换成空状态（放大镜）。
- **929 文档类的头**（4.2.6、4.3.1）：版本页与变更集页照页的头一行：左边面包屑，右边是页级的操作（"还原这个版本"、"撤销这次改动"，`size="sm"` 的描边胶囊）；其下居中的 720 px 一列里是 32 px 的 `h1`、它下面的一行事实与其余。版本页右边那一格对能编辑这一页的人总在、定宽（`flex-[0_1_10rem]`、`min-h-7.5`），"还原这个版本"在其中出现或消失不改头一行的行数（P4 的决定 894 的规矩，P3 的决定 848 的教训）：它要等版本与版本列表都读到才知道显不显，直接打开版本的地址时版本列表常常晚到，窄屏上头一行会从一行变两行、标题与正文下移约 36 px（审查 A 的 P5A-M4；第 1 节 (h) 照正文的类名量过，有没有按钮 `h1` 的上边都一样）；读者没有这一格。代价是能编辑的人看最新的版本时，窄屏上头一行之下空着一行（30 px）。变更集页的"撤销"与 `h1` 同在 `ChangesetRead` 挂载时出现，不跳，不另留。两页的按钮在 `h1` 之前：从页首按 Tab 的次序不变；到达之后焦点在 `h1` 上，从 `h1` 起 Tab 先到事实里的链接、跳过按钮，要 Shift+Tab 回去，与页的"编辑"一样（决定 865、894：主操作统一在头一行右边；挪到 `h1` 之后就要放进列里，与页不一致），可以接受（审查 A 的 P5A-N3），记进读屏清单（第 9 节）。回收站的预览只有面包屑进头一行，"恢复"两个按钮留在标题旁：它们在预览的 `shown` 容器里（`trash-preview.tsx:409-411` 的 `useFocusLeaving`），挪出去之后预览被换掉时它们带着焦点离开、不在那个容器里，`trash-preview.test.tsx:372,395,523,603-614,632` 的焦点交接都要改写，好处只是对齐（主持者照审查 B 的 P5B-Q2 定：留在标题旁）；名字长时标题取按钮留下的宽，按钮在标题之下时仍靠右（`ms-auto`，同 919）。`HeadMeta` 与右栏开关只属于页（P4 第 9 节），这三页没有。三页标题下的事实是同一种样子（`docFactsClass`：12.5 px、`text-faint`，其中的链接 `text-muted-foreground`，同版本页与 P4 的头一行的灰字；初稿是三种字号与颜色，审查 B 的 P5B-M9），变更集页的 `ChangesetFacts` 只换类名、结构照旧（`changeset-page.test.tsx:106,257,495` 读 `h1` 的 `nextElementSibling`）。
- **930 `Mark`**（4.3.2）：普通的那种换成圆角小块（同 `badgeClass` 的形状与字号，`page-versions.tsx` 导出它的类名 `markClass`）；版本页的"当前"用 `current`（琥珀，同右栏，P4 的决定 863）。历史的"Ingest"与"Trimmed"用普通的；历史行的"Reverted"链接同一种形状（`cn(markClass, "hover:border-border-strong hover:text-foreground")`，悬停时边与字变深，同卡片的悬停；不用下划线：它压在小块的下边框上，审查 B 的 P5B-R1-N3），一行里不再有两套小块（审查 B 的 P5B-M8），随 `Mark` 在 S3。
- **931 名字的显示**：卡片里与子页面列表的行里的名字截短一行（卡片排成网格，高要齐；子页面列表与今天一样，行尾给数留了位置），名字的 `span` 带 `title`，全名照旧是链接的可访问名称；其余列表的行（搜索、历史、回收站、目录）与标题里的名字折行（`break-words`，`h1` 里没有空格的长名字用 `wrap-anywhere` 的 `span`）。初稿写成"行里的名字折行"，与 4.1.4 的 `truncate`、UI1 第 4 格的标题与 8.2 第 4 条互相矛盾（审查 A 的 P5A-N5、B 的 P5B-M4、C 的 P5C-M7），四处照这一条改。右栏版本行的单独一行"-"见决定 943。
- **932 健康与升级**（4.3.3、4.3.4）：只换类名，结构不动：健康的每条规则照旧是带 `id` 的 `section`（`e2e/fixtures/wiki-health.ts:24-26`、lw8:625,678 数 `section[id]`），换成卡片的样子，其中的项是同一种行（决定 925）；健康页的标题行同笔记本首页（`h1` 取 `titleTextClass`，"重新检查"靠右）；升级页的根照旧是 `div`（`e2e/fixtures/upgrade-pages.ts:31` 以"含某个 `h2` 的 `section`"找各节，外面多一层 `section` 会撞名），各节照旧是 `section`，标题换样子，两个列表进卡片（各节本身不是卡片）。
- **933 附件一节**（4.3.5）：列表进卡片，行的间距同列表类的页，"操作"照决定 927 显出，拖进来时的环用琥珀（`ring-accent`，琥珀只标状态，"可以放下"是一种状态），页的阅读视图那一块的拖放区（`attachment-drop.tsx:51`）同样换成琥珀，同一页上两个拖放区一种颜色（审查 C 顺手看到的）；不内联的附件（名字是下载的链接）在名字之后、大小之前一个灰的下载图标（`aria-hidden`），认领 M5-polish 16.7 看得见的一半；读屏那一半（名字里说"下载"）要改链接的文字，`W/test/attachments.ts:21-26` 的 `rows()` 与二十七处读它的格都读链接的 `textContent`，不做。类型的图标（`AssetIcon`）与新的下载图标、搜索结果里的图标同是 `text-faint`（审查 B 的 P5B-N2）。上传中的列表（`UploadRows`）同样进卡片、行的内边距同附件行（审查 B 的 P5B-M11），附件一节、编辑时的上传与粘贴来源的对话框一起变。小标题"附件"照旧是 `text-sm font-medium text-muted-foreground`，同页上"子页面"一节的小标题（`page-layout.tsx:507`，P4 的文件，不动；初稿把附件的改成 `text-compact font-semibold`，同一页上两个小节标题就不一样了，审查 C 顺手看到的）。页的底部与笔记本首页是同一个组件，一起变。
- **934 对话框的内容**（4.3.7）：对话框的外框与底部的按钮 P1/S2 已换；P5 换里面手写的三处：导入与粘贴来源两个对话框里 `role=alert` 的框（`W/pages/notebook/import-dialog.tsx:139`、`source-add-dialog.tsx:135` 的 `space-y-3 rounded-md border px-4 py-3 text-sm`）换成只有边、没有底的 14 px 以内的圆角框（`rounded-lg border border-border`：暗色下框若是 `bg-surface-2`，描边按钮也是 `dark:bg-surface-2`，只剩约 1.07:1 的边，看不出是按钮，审查 B 的 P5B-N4）；粘贴来源的文本框（`source-add-dialog.tsx:263-271`）用 `Input` 抽出的共用类名 `fieldClass`（底、边、圆角、聚焦时的边与环、`focus-visible:outline-hidden`：强制色下聚焦时有透明的外框画出，13.2 第 31 条），说明换成 `text-compact`（同 `FormField` 的说明）；一处标签组（决定 926）。初稿说"以 `rounded-md border` 搜过，对话框里只有这两处"，漏了这个文本框（它的类名里就有 `rounded-md border`，审查 B 的 P5B-M10）；修订时以 `<textarea`、`rounded-md border`、`border bg-transparent` 再搜 `W/pages`、`W/components`、`W/app`（测试之外）：对话框里手写的框与字段只有这三处，其余的 `rounded-md border` 是 P5 各页的列表、卡片与横幅（第 4 节各自换），`page-layout.tsx:467` 是 P4 的页里的一个说明框（不动）。
- **935 升级横幅的样子**（4.3.6）：`rounded-card border border-border bg-surface-2 px-4.5 py-3.5 text-sm`，第一行（升级的状态）`font-medium`，前面一个 6 px 的琥珀小圆点（样稿 reading.html 的 `.meta .dot`；3 px 的 `border-accent` 画的圆，不是填色，"琥珀只标状态"，审查 B 的 P5B-N5；对齐第一行，状态折行时不落在两行之间，审查 B 的 P5B-R1-N1）；去掉 `border-primary/40`（墨色的半透明边在暗色里发灰，与卡片的边不是一套）。文字不动，状态那一行的文字照旧是 `output` 的直接子元素的文字（`upgrade-banner.test.tsx:35-36` 的 `bannerOf` 读文字的 `parentElement`）。
- **936 se1 暗色链接的下限重量**（4.4.3）：命中进了卡片，暗色的焦点环量的是卡片的底（`--surface`），推算约 7.31（第 1 节 (d)），今天的下限 7.82 会红；S2 照 4.7 的规则在应用里量、改成实量减 0.1，量出的值与下限写进 se1 的注释与本文 10.2。亮色不变（两种底都是白）；"在哪里找"的下拉照旧在页的底上，不变。
- **937 移交的认领**（第 9 节、4.5）：M12-polish 第 1 项（M5-polish 的条目）与第 5、11 项里落在 P5 的页上的，逐项写明（修订时照审查 C 的 P5C-M8 补全）：
  - 认领的：第 5 项的"回收站预览 `h1` 的长名"（S2：名字在 `wrap-anywhere` 的 `span` 里，TB1 新的一格在 375 宽核对；M12/P2 的决定 414 留下它，因为"没有现成的 375 检查"），同一页与回收站页上带名字的"在后台恢复"按钮在 375、320 宽横向溢出的那一处也在 S2 一并改（按钮可折行，TB1 那一格核对；审查 B 的 P5B-M12，它不是 M12-polish 里的条目，算进这一项的同类）；第 1 项里的 16.7 看得见的一半（S3，决定 933）。
  - 不认领、照旧留给 v0.2 的（第 1 项）：1（笔记本首页上"某某正在编辑"的标记：要在线状态的数据，是功能）；9.4（笔记本首页的"新建页"不清掉树上拖动被拒的说明：行为）；16.2、16.4a、16.5、16.9（附件的拖放、复制、焦点：行为）；21.1、21.4、21.5、21.6、21.9、21.12、21.13（版本：行为与 Worker）；22.1、22.3–22.9（回收站：文字、焦点与竞态）；23.2（"撤销这次改动"的英文措辞：改按钮的名字要改以这个名字找它的 16 个 e2e 与 vitest 文件，含 `e2e/fixtures/wiki-pages.ts:391-401` 的 `revertWith`，不是外观）；23.5、23.6、23.7、23.9、23.15、23.18；24.4（进度的写法带单位、计数分位：改文字）；24.5（回收站页的"在后台恢复"在说明之下、离那一项的行远：放进行里要改拒绝的提示与按钮在哪、行离开时焦点给谁，是行为；"已开始"不区分同名是文字；S2 只改这个按钮的折行，见上）；26.6、26.7、26.8、26.10；27.1（搜索之后退回已删的页：行为）；31.2（历史显示 `clientInfo.title`：要服务端给）；34（目录的父页选择器：功能）；35.4、35.5、35.6、36、38.2、38.4、38.13、42–47（行为、服务端或文字）。
  - 不认领的（第 5、11 项）：同类约二十处的 `bdi`（M12/P2 决定 395 只说"其余约二十处"，没有逐处的清单；P4 第 9 节把"P4 的页之外的"交了出来）：找出它们要逐个看名字后面紧跟的是什么，是文字的方向，不是外观；P5 新加与改写的文字里，名字都在自己的盒子里（卡片的名字、子页面行的名字是弹性项，数与"开放程度 · 成员数"在别的盒子里），不与数或别的名字同一段，P5 不另加；第 5 项的"变更集页面包屑的 `current`"（要让 `BreadcrumbTrail` 收 `ReactNode`，不是外观）。
- **938 设置里的空列表与右栏**（4.1.1）：令牌、邀请、无主笔记本、审计、任务五个空列表（`W/pages/settings/tokens-page.tsx:46`、`W/pages/workspace/invitations-section.tsx:54`、`ownerless-page.tsx:152`、`audit-section.tsx:72`、`W/pages/notebook/transfer-page.tsx:235`）换成卡片大小的空状态（03-P3 第 9 节交来）；令牌、无主笔记本、任务三个的图标与设置目录里那一页的相同（`KeyRound`、`Box`、`ArrowLeftRight`，`W/app/settings-directory.tsx:35,42,49`），邀请是 `Mail`，审计是 `ScrollText`。右栏的"没有反链""没有属性"（`W/pages/page/page-backlinks.tsx:140`、`page-properties.tsx:79`）照旧一行灰字：右栏的小节窄，一个图标方块比它的内容还占地方（P4 第 9 节交来"看要不要换"）。
- **939 导入导出**：00 号文档第 7 节 S2 一行的"导入导出"是笔记本设置里的那一页，P3/S2 已换成卡片（`transfer-page.tsx`）；P5 只管它的两个对话框里的框（决定 934）与它的空列表（决定 938）。00 号文档 4.6 的那一条随 2.1 第 8 条改（决定 945）。
- **940 内容链接的到达**（主持者照审查 A 的 P5A-Q1 定）：P5 改写的行与头里通往别处的内容链接照 13.2 第 17 条补上到达的状态（`arrived`），随改写那一页的 Step：S2 是历史行的主链接、"Reverted"与"Reverts …"（`history-row.tsx:64-71,107-113,120`），变更集的改动行的"In the trash""History of this page"与内容的版本（`changeset-change.tsx:120,131,148`），回收站的行（`trash-row.tsx:502-509`），变更集页的"See it""Reverts …""Reverted by"（`changeset-page.tsx:296,387,398`）；S3 是版本页的作者（`version-page.tsx:189`）与升级页通往页面的链接（`upgrade-page.tsx:606`）。去处的 `h1` 照旧在到达时取焦点（变更集页的在 `ChangesetRead`，预览的在 `Preview`，历史、页、版本页的各在自己的壳里）。不带时焦点落到 `body`（审查 A 的探针：历史页点一行的主链接，变更集页的 `h1` 出来之后 `activeElement` 是 `BODY`）。已带路由状态的照旧（改动行的主链接与健康的项经 `openOf`，目录的行，"See its progress"的 `focusJob`）。不加的：目录的说明里的两个链接（`index-page.tsx:270` 回到本页、`:277` 去健康页的锚点，锚点要取焦点的是那一节）与预览"已过期"一支里回回收站的链接（`trash-preview.tsx:599`，在说明的句子里，不是行或头）。各配一格与一条变异（7.1、7.2），进 8.2 第 8、13 条。（上面的英文是代码里的叫法；界面的英文与测试找它们的名字是"Undone"（链接名"Undone: …"）、"Undid “…”"、"See the undo"、"Undone by:"之后的"Undo change, ID …"与"History of this node"，审查 C 的 P5C-R1-N1。）
- **941 已登录的邀请页的"回到工作区"**（主持者照审查 A 的 P5A-Q2 定，照决定 803）：`Corner` 的"回到工作区"在新建工作区页之外，也给已登录、账户已有工作区、在 `/invitations/:id` 上的页（4.1.6）；去 `/`、带 `arrived`，`LandingPage` 把它转到最近的工作区，那里的 `h1` 取焦点。账户还没有工作区时不放：那时 `/` 落到新建工作区的页，"回到工作区"说的不对（同决定 803 对新建工作区页的规矩：照处置的"没有时去首页"，在这里就是不放）。账户的引导还没走完时也不放：邀请页在 `Onboarded` 之外（`W/app/routes.tsx:78-85`），`/` 会去 `/onboarding`（`W/app/guards.tsx:74-81`），同样说的不对；`inviting` 时另要 `pendingSteps(account.me).length === 0`（`W/onboarding/steps.ts:59`，审查 A 的 P5A-R1-N1）。邀请页因此要读工作区的列表（同新建工作区页，`corner.tsx:59-61` 的读法）。这带来一个竞态：从邮件打开的新标签页还没读到列表，第一次读还在路上时接受邀请，那次读回来会把不含刚加入的工作区的列表写进去，进去是"Page not found"（审查 A 的 P5A-R1-M1 的探针；今天邀请页不读列表，没有这一种）。`WorkspaceStore.load` 在读的过程中有改动被答复、又没有可留的列表时再读一次（4.1.6）。配格（`invitation.test.tsx`，照 `layout.test.tsx:184-206` 的两格，另有引导的一格与竞态的一格；`workspace.store.test.ts` 再读的一格。竞态的一格按"Accept invitation"之前先等邀请页发出的那次读已在路上，不然它分不出"竞态补上了"与"竞态没发生"，在旧树上也绿；没有工作区、引导没走完的两格断言"没有"之前也先等这次读（不然在列表回来之前断言，变异 95 下也过），在旧树上同样红在这一步（今天邀请页不读列表），"没有"本身由变异 68、95 守着，审查 A 的 P5A-R2-M1、C 的 P5C-R2-M1）与变异 68、69、93、95。
- **942 虚线卡自成一行**（主持者照审查 B 的 P5B-Q1、C 的 P5C-M2 定）：虚线的"新建笔记本"在两组之后自成一行（决定 918），算 M 内的修订，00 号文档 4.6 的"卡片列表最后一张"随 2.1 第 7 条改；918 的理由照实写成语义（它不是任何一组里的笔记本），不是测试。
- **943 右栏版本行的"-"**（主持者照审查 B 的 P5B-M5 定）：设计者在探针里试 B 的改法（"当前""编辑中"内联在"· Edit"之后，同版本页的事实一行），三种引擎 200–240 px 逐 1 px 扫，"-"不再单独一行就进 S3，否则照 B 的数据把交 v0.2 的文字改成"可复现"。结果：今天的写法三种引擎都复现，内联的写法三种引擎都是 0（第 1 节 (g)，四次扫描），所以进 S3 提交 1（4.3.2，右栏的版本行是 P4 的文件，S3 改它）。小块前各一个空格、不另包一层：读版本行的 `page-versions.test.tsx:21-29` 以 `span` 数小块，多包一个 `span` 会多数一个；"· EditCurrent"连写的四处（`version-restore.test.tsx:157`、`page-versions.test.tsx:104` 的中文"撤销当前编辑中"、hi1:259、hi2:114）随之改成中间各一个空格（`:104` 审查 A 的 P5A-R1-M2 补上）。小块挪进行内之后，汉字之间可以断，"编辑中"会被劈成两半、各带半个圆角，所以 `Mark` 的两串都加 `whitespace-nowrap`（审查 B 的 P5B-R1-M2）；第四次扫描（第 1 节 (g)）也数了断成两半的小块：不加时中文的小块三种引擎都断，加了之后三种引擎的"-"与断开的小块都是 0。
- **944 CI**（主持者照审查 C 的 P5C-Q1 定）：7.3、7.5 写明 M13 在 CI 上只跑过一次：负责人推送 `f62d0390` 之后的 run 45551442052（web、server 各分片、image 绿；e2e 红三格，PG17 第 2 格的 Chromium、WebKit 与 C8，都是测试的写法，由 `9871c7d4` 修好，04-P4 10.4；修好的头还没推送，决定 746）；初稿的"L4 已在 CI 上用过（P4/S2）"在写的时候不对，L4 到这一次才在 CI 上跑过，没有红；CI 的 E2E 一步在 `f62d0390` 上是 8 分 22 秒，不再拿 M12 收尾的 6 分 21 秒推。只靠 CI 的各步都等负责人推送之后的 CI，18 分钟的余量照本机推断应当够，是推断，不是 CI 的数。 S1 的实施与审查之后（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：负责人推送 `e8809170` 之后分支的 CI（run 45579305353）全绿，E2E 一步 8 分 5 秒，PG17 与 C8 的两处修复在 Linux 上成立（04-P4 10.4）；release 的演练（run 45579305372）也绿。S1 的头还没推送，7.3 列的 S1 的几样等负责人推送之后。
- **945 00 号文档 4.6 的文档类的页**（主持者照审查 C 的 P5C-Q2 定）：4.6 把"版本"列进"页面标题 + 筛选条 + 卡片里的行"，与决定 929 不同；随 2.1 第 8 条修订：版本页、变更集页与回收站的预览照页的头一行排，不算"列表类"；导入导出页已由 P3 换过（决定 939）。

以下是主持者照 S1 的实施审查定的（负责人可以改判）：

- **946 页读取中与页不在时，列照读到之后的页排**（审查 B 的 P5S1B-M1；改 915 里 `PageColumn` 的写法）：`PageColumn`（`W/pages/page/page-layout.tsx`）是读右栏偏好的 observer，照 `PageShell` 排：外层 `md:-mt-3 md:pb-4 xl:flex xl:items-start`；列之前一个 `aria-hidden` 的空头一行，照页的头一行排（`headRowClass`，里面两个空块：面包屑的那一块 `headCrumbsClass`、高一行 13 px 的字，右边一组的那一块 `headGroupClass`；初稿是 `mb-7.5 min-h-9`，窄于约 950 px 时页的头一行折成两行、57.5 px 高，横幅仍下移 21.5 px，审查 B 的 P5S1B-F1-N1）；`panel` 时（决定 951）xl 且右栏开着，左边加 `xl:pe-10`、右边一个 `xl:w-63 xl:-me-10` 的空块。页读取中与页不在两支都这样：树读到之前与之后横幅的 x、y、宽逐 px 相同，右栏开着与收起都是（UP1 在 1440、1280、1024、768、375 宽核对，三种浏览器）。照实：面包屑长、自己折行时，读到之后的头一行更高，读取中与页不在时横幅高出一截（审查 B 修复核对第 2 轮量的约 78 个字符的路径，三种引擎相同，21 组里 18 组不同：1440 宽差 3 px，1280、1024 宽 22.5 px，900 宽 0，768、375 宽 19.5 px，320 宽 39 px；1440 宽右栏开着时面包屑约 428 px，笔记本名四十来个字符加页名就会折）；没有横幅时，页不在的"Page not found"也在页的列里，xl、右栏开着时比"笔记本不存在"偏左 126 px、低一截（审查 B 的 P5S1B-F1-N2，主持者取 B 倾向的 (a)：那正是页的内容列所在处，与读取中、读到之后一致）。
- **947 卡片里的徽记紧跟名字**（审查 B 的 P5S1B-N1）：工作区首页卡片里名字的 `span` 不带 `flex-1`，"LLM Wiki"的徽记紧跟名字（样稿的 `.badge`，`badgeClass` 的注释）；长名字照样截短，徽记在卡片之内。
- **948 入口卡片里的正文一律 `text-sm`**（审查 B 的 P5S1B-N3）：邀请的一句（原是 `text-lg`）、注册关闭、新建工作区关闭时、邀请页里会话不可用的说明；引导的"第 N 步"与 `h1` 包进 `space-y-1.5`。实施者照"一律"另改了引导里的两句（工作区那一步关闭时、笔记本那一步的 noTarget），接受。卡片里短暂出现的共用 `Loading`、`NotLoaded` 一行照旧 16 px：全站共用的组件，只闪一下，不改（审查 B 修复核对第 1 轮同意：要统一时改的是 `Loading` 本身，另作一条决定）。
- **949 居中的操作是小胶囊**（审查 B 的 P5S1B-Q1）：`EmptyState` 与 `StatusPage` 的居中操作一律 `size="sm" shape="pill"`（00 号文档的按钮规则：卡片与居中的页级操作用胶囊，表单底部才用 8 px 圆角）；工作区首页空状态的"新建笔记本"随之。
- **950 8.2 第 1 条在 S1 收窄**（审查 C 的 P5S1C-Q1）：S1 只给两个首页与设置的五个列表用了 `EmptyState`，总体设计 9.2 那一句在 S1 写成"两个首页与设置的卡片里，没有东西的列表是……"，句尾"右栏的……（M13/P5/S1）"照旧（S2 第 6 条的锚点）；S2 的文档提交再放宽到列表类的页（8.2 第 6 条的第二处）。
- **951 `PageColumn` 的 `panel`**（审查 B 的 P5S1B-F1-M1）：`PageColumn` 收 `panel?: boolean`，默认 `false`，不留右栏的地方；只有页的两支早退传它。S2、S3 照 4.2.6、4.2.8、4.3.1 把它用在变更集页、回收站的预览与版本页的早退里时不传：那三页没有右栏，读取中、找不到与读到之后的列同在居中处；S2 导出它时，空的头一行由调用者定形（只有面包屑一块，或带调用者那一页右边一组的类名），选项按"页的头"命名；S2、S3 的表在 1440、768 与 375 宽同时量 x 与 y（审查 B 的 P5S1B-F2-N1，主持者定，归在 951 之下：今天不传 `panel` 的空头一行照页的样子排，面包屑一块加 26rem 的右边一组，那三页的头一行不是这样，900 以下会差约 21.5 px）。

## 3. 服务端

无。不改接口、契约与数据库；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 在每个 Step 合并之前以 0 退出（7.3）。

## 4. 网页

类名逐字写出，探针 (a)、(h) 编过；`[&_svg]:size-5` 一类的选择器与 `group/row` 的标记照 Tailwind 4.3.3 的写法。颜色类只用主题色（`design-tokens.test.ts`，4.5）。`cn(a, b)` 的合并照 `W/lib/cn.ts` 的 tailwind-merge（P1 登记过 `text-compact`、`rounded-card` 等名字）：后面的同组类名换掉前面的，例如 `cn(docTitleClass, "mb-0")` 去掉 `mb-3.5`。

### 4.1 S1 首页与入口

#### 4.1.1 空状态（提交 2）

新文件 `W/components/empty-state.tsx`：

```tsx
type Size = "page" | "card";
const frame: Record<Size, string> = {
  page: "flex flex-col items-center gap-3 px-6 py-14 text-center",
  card: "flex flex-col items-center gap-2.5 px-4.5 py-7 text-center",
};
/** The tile an empty list's or a status page's icon is in (StatusPage shares it, from commit 3: exported then). */
const iconTileClass: Record<Size, string> = {
  page: "grid size-11 shrink-0 place-items-center rounded-card border border-border bg-surface-2 text-muted-foreground [&_svg]:size-5",
  card: "grid size-9 shrink-0 place-items-center rounded-control border border-border bg-surface-2 text-muted-foreground [&_svg]:size-4.5",
};
export function EmptyState({ icon: Icon, size = "page", action, children }: {
  icon: LucideIcon; size?: Size; action?: ReactNode; children: ReactNode;
}) {
  return (
    <div className={frame[size]}>
      <div aria-hidden className={iconTileClass[size]}><Icon /></div>
      <p className="max-w-90 text-sm text-balance text-muted-foreground">{children}</p>
      {action !== undefined && <div className="mt-1 flex flex-wrap justify-center gap-2">{action}</div>}
    </div>
  );
}
```

- `iconTileClass` 在提交 2 不导出（决定 913），提交 3 写成 `export const`。
- 说明是 `<p>`，文字就是今天那一行的（键不改）：`history-page.test.tsx:413,520,528,761,921,983` 以 `{ selector: "p" }` 找"No changes."，`notebook-layout.test.tsx:20`、pg1:179、tb1:283,387 以确切的文字找，都照旧只找到一处。
- 没有 `role`：今天的那一行也没有；`history-page.test.tsx:411` 要页上只有一个 `status`。
- 图标的方块 `aria-hidden`，`svg` 自己 lucide 已带 `aria-hidden`（两层都在，方块里将来放别的也不读）。
- 用处（S1）：工作区首页（`Book`，带"新建笔记本"）、笔记本首页（`FileText`，不带按钮）、设置的五个空列表（`size="card"`，决定 938，替换 `<p className={cardTextClass}>`：`tokens-page.tsx:46`（`KeyRound`）、`invitations-section.tsx:54`（`Mail`）、`ownerless-page.tsx:152`（`Box`）、`audit-section.tsx:72`（`ScrollText`）、`transfer-page.tsx:235`（`ArrowLeftRight`））。这五处是 `cardTextClass` 全部的用处（`git grep cardTextClass`，`f62d0390`），S1 之后它没有用处，删掉（knip 会报）。S2 用在搜索（没提交时）、历史、回收站、目录（两种）、标签。

#### 4.1.2 共用的类名（提交 2）

- `W/app/settings-page.tsx`：
  - 新导出 `cardClass = "divide-y divide-border rounded-card border border-border bg-surface"`，`SettingsCard`（`:98`）改用它；
  - 新导出 `pageTitleClass = "text-[26px] leading-normal font-[650] tracking-[-0.01em] break-words outline-none"`，`SettingsPage` 的 `h1`（`:39-45`）改成 `cn("mb-7", pageTitleClass)`，生成的类不变。
- 新文件 `W/app/page-classes.ts`（只有类名，决定 925），S1 放进五个：

  ```ts
  /** The workspace's home: its cards three abreast from xl (decision 916). */
  export const homeColumnClass = "mx-auto w-full max-w-232 space-y-7 md:pt-5 md:pb-4";
  /** A list's page and a notebook's home: 720px, centred, as a page's column (decision 916). */
  export const listColumnClass = "mx-auto w-full max-w-180 space-y-4 md:pt-5 md:pb-4";
  /**
   * A page's title and, on its right, what the page does: New page, Recheck (decision 925). Its title takes
   * cn(pageTitleClass, titleTextClass), its button ms-auto: the button on the right, below the title too.
   */
  export const titleRowClass = "flex flex-wrap items-center justify-between gap-x-4 gap-y-3";
  /**
   * The title in that row: the width its button leaves, at least 14rem from sm before the button goes below it;
   * under sm as wide as its words, so that a short title keeps its button beside it (review B's P5B-R1-M1).
   */
  export const titleTextClass = "min-w-0 flex-1 basis-56 max-sm:basis-auto";
  /** A group's title above its cards or rows: 13px, grey (the mockup's settings .gt). */
  export const groupTitleClass = "mb-2.5 px-0.5 text-compact font-semibold text-muted-foreground";
  ```

  `listColumnClass` 初稿在 S2，笔记本首页改用它之后在 S1（决定 916）。
- `W/pages/page/page-layout.tsx:76-93`（P4 的页的两支早退，决定 915）：两个片段换成同一个列：

  ```tsx
  /** The page's column, as a page's content column is (decision 915): local in S1, exported in S2 with the strings. */
  const docColumnClass = "mx-auto w-full max-w-180";
  /** PageColumn is a page's column alone, the shell's banner at its top: what shows before its head can. */
  function PageColumn({ children }: { children: ReactNode }) {
    return (
      <div className={docColumnClass}>
        <ColumnTop />
        {children}
      </div>
    );
  }
  ```

  `:79-82` 的 `<><ColumnTop /><NotLoaded …/></>` 换成 `<PageColumn><NotLoaded …/></PageColumn>`，`:89-92` 的同样包 `PageGone`；`:430` 的内容列改用 `docColumnClass`（同一串）。`:76` 的注释照实改（"the banner at the top of a column, the tree read or not, the page found or not"）。读这两支的格（`upgrade-banner.test.tsx:153-177`）只读次序与个数；`page-layout.test.tsx` 等以角色与文字找 `NotLoaded`、`PageGone`，多一层 `div` 不变（第 1 节 (f)：没有格读 `main` 的直接子元素）。 照实（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：`PageColumn` 照决定 946、951 写成读右栏偏好的 observer，收 `panel`，外层、空的头一行与右栏的地方见 2.3；S1 就导出了它（`page-layout.test.tsx` 的格用它）。

#### 4.1.3 工作区首页（提交 2）

`W/pages/workspace/workspace-home.tsx`：

- `:31` `section.max-w-4xl.space-y-6` → `section` 带 `homeColumnClass`；`:32` 的 `h1` → `pageTitleClass`（`ref`、`tabIndex` 照旧）。
- 无主笔记本的提醒（`:85-90`）：`p.text-sm.text-muted-foreground`，链接 `font-medium text-foreground underline underline-offset-4`，加 `state={arrived}`（决定 924）。
- `NotebookGroups` 的三个类名：`headingClassName={groupTitleClass}`、`listClassName="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"`、`groupClassName="min-w-0"`。
- 卡片（`renderItem`，`:44-51`），链接照旧带 `arrived`、去笔记本首页：

  ```tsx
  <Link to={…} state={arrived} className="flex h-full flex-col gap-3 rounded-card border border-border bg-surface p-4 transition-colors duration-150 hover:border-border-strong hover:hover-layer sm:min-h-28">
    <span className="flex min-w-0 items-center gap-2.5">
      <span aria-hidden className="grid size-8 shrink-0 place-items-center rounded-control bg-surface-2 text-muted-foreground [&_svg]:size-4"><Book /></span>
      <span title={notebook.name} className="min-w-0 flex-1 truncate text-sm font-medium">{notebook.name}</span>
      {notebook.mode === "llm_wiki" && <span className={cn(badgeClass, "ms-0 shrink-0")}>{t("notebooks.modeLLMWiki")}</span>}
    </span>
    <span className="mt-auto text-[12.5px] text-faint">
      <WholeParts parts={[t(`access.${notebook.workspace_access}`), members(notebook.member_count)]} />
    </span>
  </Link>
  ```

  焦点照全局的 2 px 琥珀外框（`W/styles.css:160-163`；去掉今天的 `focus-visible:ring-2 … outline-none`：卡片是链接，13.2 第 31 条"链接与别的元素是外框"）。`members(n)` 是 `n === 1 ? t("notebooks.membersOne") : t("notebooks.members", { count: n.toLocaleString(preferences.locale) })`（同 `index-rows.tsx:102-103`，决定 917）。链接的文字是"名称 + 徽记 + 开放程度 · 成员数"（`textContent` 没有空格：`PlansPrivate · 1 member`，今天同样没有）。名字的 `title` 只给看得见的提示，链接的可访问名称照旧来自它的文字。
- 卡片之后、两组之后自成一行（决定 918、942）：`createsNotebooks(workspace)` 且两组不都空时，

  ```tsx
  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
    <CreateNotebookDialog workspace={workspace} here={here} trigger={
      <button type="button" className="flex min-h-14 w-full items-center justify-center gap-2 rounded-card border border-dashed border-border-strong text-sm font-medium text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden sm:min-h-28 [&_svg]:size-4">
        <Plus aria-hidden />{t("notebooks.create")}
      </button>} />
  </div>
  ```

  按钮是控件，焦点是环（同 `Button`）。一列时 56 px 高，640 px 起同卡片 112 px（审查 B 的 P5B-M2）。`NotebookGroups` 两组都空时画 `empty`，虚线卡不画；它要在 `NotebookGroups` 之外知道"都空"：用 `notebooks.list` 的长度（同一个 store，`NotebookGroups` 自己也读它；"列表不空"与"两组不都空"等价，`groupNotebooks` 是一个划分，`W/stores/notebook.store.ts:18-24`），读取中不画。
- 两组都空（`empty`，`:53-64`）：`<EmptyState icon={Book} action={createsNotebooks(workspace) ? <CreateNotebookDialog … trigger={<Button>{t("notebooks.create")}</Button>} /> : undefined}>{t("notebooks.empty")}</EmptyState>`。
- `W/pages/workspace/notebook-groups.tsx:62` 的 `li.flex.flex-col` 照旧（卡片 `h-full` 撑满）。
- 文字（`W/i18n/messages/en.ts`、`zh-CN.ts`，在 `notebooks.modeLLMWikiHint` 之后）：`notebooks.membersOne` "1 member" / "1 位成员"；`notebooks.members` "{count} members" / "{count} 位成员"（同 `index.inboundOne` 与 `index.inbound` 的写法）。

#### 4.1.4 笔记本首页与子页面列表（提交 2）

`W/pages/notebook/notebook-home.tsx`：

- `:42` `section.space-y-4` → `section` 带 `listColumnClass`（720，决定 916）。它的第一个子元素是 `<ColumnTop />`（决定 915；`W/app/routes.tsx` 的笔记本 `index` 路由（`:178-184`）加 `handle: { column: true }`）。`NotLoaded` 那一支也在这个 `section` 里、`ColumnTop` 之后，不另画。
- 标题行（`:43-59`）：`div` 带 `titleRowClass`；`h1` → `cn(pageTitleClass, titleTextClass)`（P3 的 `listTextClass` 加 `max-sm:basis-auto`）；"新建页面"加 `size="sm" shape="pill" className="ms-auto"`（`<Plus />` 照旧）。名字长时标题折行、按钮留在右边，排不下时在标题之下靠右（审查 B 的 P5B-M7）；640 px 以下标题只有它的字那么宽，"Plans""计划"那样的名字在 320–390 宽照旧与按钮同行，长一些的名字占满一行、按钮在下一行靠右（第 1 节 (l)，审查 B 的 P5B-R1-M1）。
- `:63-64` 没有页面：`<EmptyState icon={FileText}>{t("notebook.homeEmpty")}</EmptyState>`（不带按钮，决定 919）。
- `:66` `SubpageList` 多传 `countOf`：在渲染里读一次树（`const tree = pages.tree`，`:61` 的判断随之写成 `tree === undefined`：`countOf` 里的 `tree` 才是 `TreeIndex`，审查 C 的 P5C-R1-N1），`countOf={(page) => childrenOf(tree, page.id).length}`（`W/stores/page-tree.ts:38`）。与每行经 `pages.childrenOf` 读等价（`tree` 是首页这个 observer 读着的 computed，`W/stores/page-tree.store.ts:161-183,240-243`），写成一次读是为了意思清楚（决定 919，变异 15）。
- `:69-71` 的"笔记本设置"链接删掉（`Link` 的 import 若再没有用处一并删）。
- 附件一节照旧（S3 换它的样子）。

`W/pages/page/subpage-list.tsx`：

- `ul`（`:45`）→ `className={cardClass}`（`onClick` 照旧，所以不用 `SettingsCard`）。
- `SubpageList` 收 `countOf?: (page: TreeNode) => number`，在一次渲染里取一次 `t` 与语言区（`useT()`、`useStore().preferences.locale`），给每行算好两串：看得见的数 `count`（`n.toLocaleString(locale)`）与读屏的句子 `said`（`n === 1 ? t("notebook.subpagesOne") : t("notebook.subpages", { count: n.toLocaleString(locale) })`），没有子页面的行两串都不给。行只收字符串，memo 照旧只比字符串。
- `SubpageRow`（`:54-62`，照旧 memo）收 `href`、`name`、`count?`、`said?`：

  ```tsx
  const described = useId();
  return (
    <li className="relative transition-colors duration-150 first:rounded-t-[13px] last:rounded-b-[13px] hover:bg-hover">
      <a href={href} aria-describedby={said === undefined ? undefined : described} className={cn("flex items-center gap-2.5 rounded-[inherit] py-2.5 ps-4.5 text-sm focus-visible:-outline-offset-2", count === undefined ? "pe-4.5" : "pe-16")}>
        <FileText aria-hidden className="size-4 shrink-0 text-faint" />
        <span title={name} className="min-w-0 truncate">{name}</span>
      </a>
      {count !== undefined && (
        <span aria-hidden className="pointer-events-none absolute inset-y-0 end-4.5 flex items-center text-compact text-faint tabular-nums">{count}</span>
      )}
      {said !== undefined && <span id={described} hidden>{said}</span>}
    </li>
  );
  ```

  链接的文字照旧只是名字（`svg` 没有文字）：`page-tree-scale.test.tsx:147`、`tag-page.test.tsx:35-45`、`page-tree.test.tsx:333,510-517`、`e2e/fixtures/wiki-pages.ts:174-180` 不变；数在链接之外、不是 `a`（`page-tree-scale.test.tsx:239-240` 数 `a`）。读屏的数是链接的描述（`aria-describedby` 指向 `hidden` 的句子：描述照样取被直接引用的隐藏元素的文字，浏览模式不读它，看得见的数 `aria-hidden`，决定 919）。`13px` 是卡片 14 px 的圆角减去 1 px 的边，悬停的底不出角；链接 `rounded-[inherit]` 随 `li` 的圆角，键盘焦点的外框向里收 2 px（`focus-visible:-outline-offset-2`：全局的外框偏移 2 px，铺满一行的链接的外框会冲出卡片的圆角与左右边约 3 px，审查 B 的 P5B-M3）。
- 文字：`notebook.subpagesOne` "1 subpage" / "1 个子页面"；`notebook.subpages` "{count} subpages" / "{count} 个子页面"。
- 页的子页面一节（`page-layout.tsx:508`）与标签页（`tag-page.tsx:54`）不传 `countOf`，有图标、没有数。

#### 4.1.5 入口的卡片（提交 3）

- `W/app/document-title.ts:3`：`const app` 改成 `export const appName = "Nerve Wiki"`，文件里的用处随之改名。
- `W/components/ui/button.tsx`：新导出 `buttonClass`，`Button`（`:36-38`）改用它；`buttonVariants`（`:6-32`）照旧不导出（决定 921，审查 B 的 P5B-I1）：

  ```ts
  /** buttonClass is a button's classes merged, as Button has them: a link drawn as a button takes it (cva joins its variants' unmerged). */
  export function buttonClass(variants: VariantProps<typeof buttonVariants>, className?: string): string {
    return cn(buttonVariants(variants), className);
  }
  ```

- `W/components/empty-state.tsx`：`iconTileClass` 导出（`StatusPage` 用它）。
- 新文件 `W/components/entry-card.tsx`：

  ```tsx
  /** The title of a page in an entry card, and of a status page: 22px (decisions 920, 921). */
  export const entryTitleClass = "text-[22px] leading-snug font-[650] tracking-[-0.01em] break-words outline-none";
  export function EntryCard({ children }: { children: ReactNode }) {
    return (
      <div className="flex flex-col items-center pt-[4svh] md:pt-[10svh]">
        <section className="w-full max-w-100 rounded-card border border-border bg-surface px-6 py-7 sm:px-8 sm:py-8">
          <div className="mb-6 flex items-center gap-2.5 text-sm font-semibold">
            <svg aria-hidden viewBox="0 0 32 32" className="size-7 shrink-0 forced-colors:hidden">
              <rect width="32" height="32" rx="7" className="fill-primary" />
              <path d="M9 23V9h2.6l7.8 9.6V9H22v14h-2.6l-7.8-9.6V23z" className="fill-primary-foreground" />
            </svg>
            {appName}
          </div>
          <div className="space-y-6">{children}</div>
        </section>
      </div>
    );
  }
  ```

  路径照 `web/apps/web/public/favicon.svg` 抄（同一个"N"）；内联，不另请求文件（s2 核对静态资源都同源、控制台安静，`e2e/stories/smoke/s2-web-app.spec.ts`）。
- 五页：根的 `section.mx-auto.max-w-sm.space-y-6` 换成 `<EntryCard>`，`h1` 的类名换成 `entryTitleClass`（`ref`、`tabIndex` 照旧）：`W/pages/sign-in.tsx:20-21`、`sign-up.tsx:34-35`（`Loading` 那一支在卡片之外，照旧）、`invitation.tsx:40-41`、`onboarding.tsx:29-33`（"第 N 步，共 M 步"那一行在 `h1` 之前，照旧，`text-compact text-muted-foreground`）、`create-workspace.tsx:31-32`。卡片里的内容照旧（表单、链接、引导的各步）；`CredentialsForm` 的提交按钮照旧 `w-full`。
- `W/app/session-unavailable.tsx`：`SessionUnavailable` 收 `level?: "h1" | "h2"`（默认 `"h1"`）；`invitation.tsx:65` 传 `"h2"`（决定 922），这一支不加图标与居中（它在卡片里）。

#### 4.1.6 出错、会话不可用、404 与邀请页的右上角（提交 3）

新文件 `W/components/status-page.tsx`：

```tsx
export const statusBodyClass = "max-w-100 text-sm text-balance text-muted-foreground";
export const statusActionsClass = "mt-2 flex flex-wrap justify-center gap-2";
/** A page that says why there is nothing: centred, its icon in a tile, then what the caller writes (decision 921). */
export function StatusPage({ icon: Icon, alert = false, children }: { icon: LucideIcon; alert?: boolean; children: ReactNode }) {
  return (
    <section role={alert ? "alert" : undefined} className="flex flex-col items-center gap-3 px-2 pt-[6svh] text-center md:pt-[10svh]">
      <div aria-hidden className={iconTileClass.page}><Icon /></div>
      {children}
    </section>
  );
}
```

- `W/pages/not-found.tsx:24-32`：`<StatusPage icon={FileQuestionMark}>`，`h1`（`ref`、`tabIndex` 照旧）`entryTitleClass`，说明 `p` `statusBodyClass`，"回到首页"是 `<Link to="/" state={arrived} className={buttonClass({ variant: "outline", size: "sm", shape: "pill" })}>`，在 `div` 带 `statusActionsClass` 里。笔记本里的 404（有左栏）同一个组件，照样居中在 `main` 里。
- `W/app/route-error.tsx:26-35`：`<StatusPage icon={TriangleAlert} alert>`，`h1`（`entryTitleClass`）、说明、错误码（`p`，`text-compact text-faint`）、操作一行："重新加载"（`Button` 照旧 `variant="outline"`，加 `size="sm" shape="pill"`）与"回到首页"（同 404 的链接）。
- `W/app/session-unavailable.tsx`：`SessionUnavailablePage`（`:43-47`）画 `<StatusPage icon={CloudOff} alert>` 包着 `h1`（`entryTitleClass`）、说明与两个按钮（`size="sm" shape="pill"`）；`SessionUnavailable` 在 `level="h2"` 时（邀请页）照旧是今天的 `section role=alert` 与左对齐的几行，只换标题的级别与字号（`text-base font-semibold`）。为此 `SessionUnavailable` 拆成"内容"（标题、说明、按钮）与外壳两半：页给 `StatusPage`，邀请给普通的 `section role=alert`。`role=alert` 两种都在包着标题的元素上（决定 921）。
- `W/app/corner.tsx`（决定 941）：`:56` 的 `creating` 之外加 `inviting = useMatch("/invitations/:id") !== null`；`:64` 的 `back` 从 `creating && …` 改成 `(creating || (inviting && account?.me !== undefined && pendingSteps(account.me).length === 0)) && …`（邀请页在 `Onboarded` 之外，引导没走完时 `/` 去引导，审查 A 的 P5A-R1-N1；`pendingSteps` 取自 `W/onboarding/steps.ts`），其余照旧（已登录、账户读到、工作区的列表不空；链接照旧去 `/`、带 `arrived`）；读工作区的 `useSWR`（`:59-61`）同样在这两页上读。`:43-51` 的注释照实改。
- `W/stores/workspace.store.ts`（决定 941，审查 A 的 P5A-R1-M1）：`load`（`:57-66`）在读的过程中有改动被答复时，今天只在已有列表时留着它（`this.list !== undefined`）；还没有列表时那次读照样写进去，而它不含读出去之后才接受的工作区。邀请页从此读列表，从邮件打开的新标签页上第一次读还在路上时接受邀请，`WorkspaceLayout` 的 `useSWR("workspaces")` 与那次读同键、用它的结果，`bySlug` 找不到，画出"Page not found"（`W/pages/workspace/workspace-layout.tsx:123-160`）。改成没有可留的列表时再读一次：

  ```ts
  if (this.changesAnswered !== answeredBefore) {
    return this.list ?? this.load();
  }
  ```

  `:52-56` 的注释加一句（"with no list to keep, it reads again: what it read is older than the change"）。直接打开的新建工作区页上同一种情形（`create` 同样只经 `changed`）一并补上。

#### 4.1.7 `sectionAt`（提交 1）

`W/app/settings-directory.tsx:53-55`：

```ts
export function sectionAt<S extends Section>(sections: readonly S[], base: string, pathname: string): S | undefined {
  // As the router matches: a trailing slash or not, whatever the case (M13/P5/S1, decision 923).
  const at = pathname.replace(/\/+$/, "").toLowerCase();
  return sections.find(({ path }) => at === `${base}/${path}`.toLowerCase());
}
```

三个设置布局不动：`shown` 有了，`SettingsPage` 照旧以 `key={shown.path}` 画，`useSettingsHeading` 在它之下（决定 923）。

#### 4.1.8 焦点、地标与名字的走查（S1）

- **到达的焦点**：工作区首页、笔记本首页、邀请、新建工作区、404 的 `h1` 照旧取（`useArrivalFocus`）；登录、注册、引导的 `h1` 今天不取，照旧（守卫的重定向到达它们，焦点在文档开头）。新带 `arrived` 的："查看"（无主笔记本，去设置页的 `h1`）、404 与出错页的"回到首页"（去工作区首页的 `h1`，经 `LandingPage`）、已登录的邀请页右上角的"回到工作区"（同新建工作区页的那一个）。虚线卡与空状态里的"新建笔记本"同左栏的那个：建好之后去新笔记本的首页、它的 `h1` 取焦点（`CreateNotebookDialog` 照旧）；取消时 Radix 把焦点还给触发它的那个按钮。
- **地标**：首页与入口没有新地标；卡片的列表照旧是"我的笔记本""团队笔记本"两个以分组标题命名的 `list`；子页面列表照旧名为"页面"（`notebook.pages`）；空状态不是地标、没有角色。入口卡片是 `section`，没有名字（不成区域）。
- **名字**：卡片的链接名是"名称 LLM Wiki 开放程度 · N 位成员"（读屏读整个链接的文字）；书本图标、页面图标、标志与图标方块都 `aria-hidden`；子页面的链接名是名字、描述是"3 个子页面"，看得见的数 `aria-hidden`；虚线卡的名字是"新建笔记本"，`Plus` `aria-hidden`；产品名是普通的文字。
- **`useDocumentTitle`** 不变。
- **375 px**：卡片一列（`grid-cols-1`）、随内容高、名字截短；笔记本首页的长名字在标题里折行、"新建页面"在它之下靠右；入口卡片宽 327 px（`main` 左右 24 px）、里面左右 24 px；404 与出错页的操作折行。UI1 的两格新的核对（5.1）。
- **强制色**：卡片、虚线卡与入口卡片的边由系统色画出（是 `border`）；图标方块的底没了、边在；标志隐去、产品名在；虚线卡是按钮，强制色下的透明外框同 `Button`（`focus-visible:outline-hidden`，13.2 第 31 条）。

**照实**（S1 的实施与审查，[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：

- **4.1.1**：`EmptyState` 的注释写明居中的操作是调用者给的小胶囊（决定 949）。
- **4.1.2**：`PageColumn` 照决定 946、951（见 2.3），S1 就导出（不等 S2）；页的头一行的三串写成本地常量 `headRowClass`、`headCrumbsClass`、`headGroupClass`，`PageShell` 与 `PageColumn` 的空头一行共用（`e41fce83`）；面包屑长、自己折行时读取中与读到之后横幅仍差一截（946 末尾的数）。
- **4.1.3**：卡片里名字的 `span` 不带 `flex-1`，徽记紧跟名字（决定 947）；卡片的图标方块带 `border border-transparent`，强制色下方块有边（审查 B 的 P5S1B-N2，同 4.1.8 的说法）；空首页的"新建笔记本"是 `size="sm" shape="pill"`（决定 949）；读取中与读不到时不画虚线卡（代码本来就对，两格守，审查 A 的 P5S1A-M1）。卡片的 `h-full`、`sm:min-h-28` 与虚线卡的 `sm:min-h-28` 由格的类名守（审查 B 的 P5S1B-M2）。
- **4.1.4**：顶层页面由 `childrenOf(tree, null)` 算：`tree` 在渲染里读一次，`countOf` 共用它，不另经 `pages.childrenOf(null)`。
- **4.1.5**：`EntryCard` 的 `section` 带 `break-words`：没有断点的工作区名或地址在卡片里折行，375、320 宽不横向滚动（审查 B 的 P5S1B-I1；不要 `min-w-0`：实量卡片里的盒子都在卡片之内）；卡片里的正文一律 `text-sm`，引导的"第 N 步"与 `h1` 包进 `space-y-1.5`（决定 948）。
- **4.1.6**：`W/app/session-unavailable.tsx` 拆成钩子 `useUnavailable` 与两个外壳：`SessionUnavailable`（邀请页就地画的，只有 `h2` 一支，`text-base font-semibold`，说明 `text-sm`）与 `SessionUnavailablePage`（`StatusPage` 居中，`sm` 的胶囊）；922 的"多一个 `level`"照实改（922 末尾）。出错页的"回到首页"用 `notFound.home` 的文字，不另加键。
- **4.1.7**：`sectionAt` 用路由的 `matchPath`、比较之前逐段解码（923 末尾）。
- **4.1.8**：子页面行读屏的那一句 `hidden`，由格守（审查 C 的 P5S1C-M2）。
- **5.1、5.4、6.1、7.1、7.2、7.3、7.5、8.2 第 1、3、4、5、6、10 条、第 10 节**：见各节；决定 946–951。

### 4.2 S2 列表与筛选

#### 4.2.1 列表类的页的格局与同一种行（提交 1）

`W/app/page-classes.ts` 加：

```ts
/** A line or two under a list page's title: what it holds, how long things stay. */
export const noteClass = "text-compact text-muted-foreground";
/** A list's rows of lines: a changeset, a hit, a directory's page (decision 925; P3's stackedRowClass is a line and a block). */
export const stackedLinesClass = "group/row space-y-1 px-4.5 py-3.5";
/** A row of text on the left and what it does on the right: an item of the trash (added in commit 2, its first use). */
export const splitRowClass = "group/row flex flex-wrap items-start justify-between gap-x-4 gap-y-2 px-4.5 py-3.5";
/** A row's first line, 14px, 500, as a settings row's (review B's P5B-I2); its link, underlined as hovered alone (4.7). */
export const lineTitleClass = "flex items-center gap-2 text-sm font-medium";
export const lineLinkClass = "min-w-0 break-words underline-offset-4 hover:underline";
/** A row's lines of facts. */
export const factsClass = "text-compact break-words text-muted-foreground";
```

列表类的页的根照旧是今天的元素（升级是 `div`，别的是 `section`；健康的 `h1.closest("section")` 是根，`health-page.test.tsx:160`），带 `listColumnClass`；笔记本里的页的第一个子元素是 `<ColumnTop />`（路由各加 `handle: { column: true }`，决定 915），读取中、失败、不在的各支都在根里、`ColumnTop` 之后。标题：`h1` 换成 `pageTitleClass`；标题右边有页级操作的（健康）用 `titleRowClass` 的一行，`h1` 取 `cn(pageTitleClass, listTextClass)`、按钮 `ms-auto`（同笔记本首页）。

同一种行（决定 925）：各页的行的第一行是 `lineTitleClass`（它在 `flex-wrap` 的一行里时写 `cn(lineTitleClass, "flex-wrap gap-x-3 gap-y-1")`），主链接是 `lineLinkClass`，第一行里次要的链接（变更集的改动行的"In the trash""History of this page"）是 `cn(lineLinkClass, "font-normal text-muted-foreground hover:text-foreground")`，事实是 `factsClass`，内容的摘录（搜索的摘录、目录的摘要）是 `text-sm break-words`。各页逐个写在 4.2.4–4.2.9、4.3.3。

#### 4.2.2 筛选条（提交 1、2）

`page-classes.ts` 加：

```ts
export const filterBarClass = "flex flex-wrap items-end gap-x-3 gap-y-3";
/** A filter's label above its control, as FormField spaces them (01-P1 section 9; decision 926). */
export const filterFieldClass = "flex min-w-0 max-w-full flex-col gap-1.5";
/** A filter shown as a chip: the history's page, with its Remove; its corners half its least height, not round when it wraps. */
export const filterChipClass = "flex min-h-9 max-w-full min-w-0 items-center gap-1 rounded-[18px] border border-border bg-surface-2 ps-3 pe-1 text-compact";
```

- 历史（提交 1，`W/pages/notebook/history-filters.tsx`）：`:131` → `filterBarClass`；`:132`、`:143`、`:154` → `filterFieldClass`；"谁"的下拉（`:134`）加 `className="max-w-64"`（`cn` 换掉 `NativeSelect` 的 `max-w-full`，决定 926）；`:175` 的 `p`（页的筛选）→ `filterChipClass`（照旧是 `p`：`history-page.test.tsx:783` 读它的 `closest("p")` 的文字"…Remove"）；里面的名字照旧 `span.min-w-0.break-words`，"移除"（`:177-185`，`variant="ghost"`、`h-7 px-2 text-xs`）换成 `variant="ghost" size="sm" shape="pill"`，文字与名字照旧。
- 搜索（提交 1，`W/pages/search/search-page.tsx:221`）：`div.space-y-2` → `filterFieldClass`，外面包一个 `filterBarClass` 的 `div`；`NativeSelect` 的 `block max-w-full` 照旧。
- 目录（提交 2，`W/pages/notebook/index-filters.tsx`）：`:42` 的 `form` → `filterBarClass`；`:48`、`:60` → `filterFieldClass`；"应用""清除"两个按钮照旧。

#### 4.2.3 行内次要操作的显出（提交 1）

`page-classes.ts` 加：

```ts
/**
 * What a row offers besides its own link, seen as the row is hovered or holds the keyboard's focus, or as its menu is
 * open; always where nothing hovers, on a narrow screen, and in forced colours (decision 927). Transparent, not
 * hidden: Playwright clicks what it sees, and a transparent control is seen. Its transition names opacity and the
 * button's colours both, as the tree's cog does: cn would let transition-opacity take transition-colors' place.
 */
export const revealClass =
  "opacity-0 transition-[opacity,color,background-color] duration-150 group-hover/row:opacity-100 group-focus-within/row:opacity-100 focus-visible:opacity-100 aria-expanded:opacity-100 [@media(hover:none)]:opacity-100 max-md:opacity-100 forced-colors:opacity-100";
```

用处：历史行的"只看此人"（提交 1，`W/pages/notebook/history-row.tsx:73-77`：`Button variant="ghost"` 的 `h-7 px-2 text-xs` 换成 `size="sm" shape="pill"` 加 `-my-1` 与 `revealClass`；`-my-1` 让 30 px 高的按钮不把第一行撑高：历史行的第一行 22 px、到第二行 26 px，同搜索的行的 20、24 px 只差 2 px，第 1 节 (l)，审查 B 的 P5B-R1-N5）；附件行的"操作"（S3，4.3.5）。行（`li`）带 `group/row`（`stackedLinesClass` 已有，附件行另加）。

#### 4.2.4 搜索（提交 1）

`W/pages/search/search-page.tsx`：`:211` `section.space-y-4` → `listColumnClass`（工作区的页，不在笔记本里，没有 `ColumnTop`）；`:212-219` 的 `h1` → `pageTitleClass`（`aria-describedby` 照旧）；`:221` 见 4.2.2；`:239-241` 的状态行 `output.block.text-sm.empty:sr-only` 照旧（换成 `text-compact text-muted-foreground`）；`:242` 的 `sr-only` 输出照旧在它之后（`search-page.test.tsx:79-80,543` 数 `status` 的次序与第一个 `output`）；`:244` 没提交时的说明 → `<EmptyState icon={Search}>{t("search.howTo")}</EmptyState>`（`search-page.test.tsx:653,663` 以文字找它）；`:253` 的 `ul` → `<SettingsCard list label={t("search.results")}>`（只带 `aria-label`）。没有命中时照旧不画列表（决定 928）。

`W/pages/search/search-hit.tsx`：`:87` `li.space-y-1.p-4` → `stackedLinesClass`；`:88` 的 `p` → `lineTitleClass`（照旧 `p`、500：se1:340）；`:90,92` 的图标照旧是行里第一个 `svg`（`search-page.test.tsx:105,113`），`text-muted-foreground` → `text-faint`；`:94` 的 `AssetIcon`（`W/pages/page/attachment-row.tsx:27-31`）同样 → `text-faint`，另收 `className?: string`，类名写成 `cn("size-4 shrink-0 text-faint", className)`（它在这个提交里改：搜索、回收站与附件一节同用它，同一种灰；初稿写在 S3，审查 B 的 P5B-N2、P5B-R1-N2）；`:96-106` 的链接 `underline underline-offset-4` → `lineLinkClass`；`:108`、`:118`、`:126` 的三行 → 位置与时刻 `factsClass`，摘要 `text-sm break-words`（照旧 `p`：`search-page.test.tsx:125,191` 读 `p bdi`）。名字照旧是 `li` 里的第一段文字（`:348`）。

#### 4.2.5 历史（提交 1）

`W/pages/notebook/history-page.tsx`：`:171` → `listColumnClass`，`ColumnTop` 在 `h1` 之前（路由 `:239-246` 加 `handle`）；`:172` 的 `h1` → `pageTitleClass`；筛选见 4.2.2；`:184` 的 `sr-only` 输出照旧；`:194` 没有改动 → `<EmptyState icon={History}>{t("history.empty")}</EmptyState>`；`:196` 的 `ul` → `<SettingsCard list label={t("history.list")}>`（只带 `aria-label`；`li` 照旧是它的直接子元素：`history-ingest.test.tsx:43`、lw9:492 的 `:scope > li`）；`:232` 的"更多"一块照旧。

`W/pages/notebook/history-row.tsx`：

- `:62` `li.space-y-1.p-4` → `stackedLinesClass`；`:63` 的第一行 `p` → `cn(lineTitleClass, "flex-wrap gap-x-3 gap-y-1")`（今天它没写字号，链接是 16 px）；`:64-71` 的主链接 `min-w-0 font-medium break-words underline underline-offset-4` → `lineLinkClass`，加 `state={arrived}`（决定 940；链接里第一个 `bdi` 照旧是消息：`history-page.test.tsx:98`）；"只看此人"见 4.2.3。
- 消息那一行之后紧接摘要那一行（`:280,311` 读 `li` 的文字 `Change 1Wrote`），两行之间不插东西；`:79` 的来处与时刻、`:102` 的计数 → `factsClass`；`:92-101` 的消息与摘要照旧 `text-sm break-words`。
- `:103` 的 `IngestSources` 给 `className={factsClass}`（见下）。
- `:105` 小块的一行 `text-sm` → `text-compact`；"Reverted"（`:107-113`）与"Reverts …"（`:120`）加 `state={arrived}`（决定 940），"Reverts …"的类名 → `lineLinkClass`；"Reverted"的形状随 `Mark` 在 S3 改（4.3.2），S2 里照旧。`Mark` 见 4.3.2（S3；S2 里照旧）。

`W/pages/notebook/history-ingest.tsx` 的 `IngestSources`（审查 B 的 P5B-R1-M4：摄取的变更在历史行与变更集页里有一块 14 px、带下划线的链接，LLM Wiki 的每一次摄取都有）：收 `className?: string`，根 `space-y-1 text-sm`（`:37`）→ `cn("space-y-1", className)`，字号与颜色随所在的事实；说明（`:38`）去掉 `text-muted-foreground`；来源的链接（`:49`）`underline underline-offset-4` → `lineLinkClass`（`open.state` 照旧）；"已删除""不可用"的 `span` 照旧 `text-muted-foreground`。历史行给 `factsClass`；变更集页的那一个（`changeset-page.tsx:406`）在 `docFactsClass` 的根里，不另给（随根，链接经 `[&_a]:text-muted-foreground`）。`history-ingest.test.tsx` 以角色、名字与文字找它，照旧。

#### 4.2.6 变更集页（提交 1）

`W/pages/notebook/changeset-page.tsx:230-275` 照页的头一行（决定 929）。

`W/pages/page/page-layout.tsx` 把 P4 的头一行几串在原处写成导出的常量（决定 925：不搬、同一来源；S1 已写成本地常量的 `docColumnClass` 这里一并导出；照实：`PageColumn` S1 已导出，S1 另把头一行的三串写成了本地常量 `headRowClass`、`headCrumbsClass`、`headGroupClass`（`PageShell` 与 `PageColumn` 的空头一行共用，审查 B 的 P5S1B-F1-N1），前两串这里导出，`headGroupClass` 照旧本地），`:388`、`:391`、`:393`、`:430`、`:436` 改用它们，生成的类不变；右边一组照旧自己的定宽串（P4 的决定 894）：

```ts
/** A page's outer, and a document's page's: the head's row a line above the column (P4's PageShell; decision 916). */
export const docShellClass = "md:-mt-3 md:pb-4";
export const headRowClass = "mb-7.5 flex min-h-9 flex-wrap items-center gap-x-3 gap-y-1.5";
export const headCrumbsClass = "min-w-0 flex-[1_1_14rem]";
export const docColumnClass = "mx-auto w-full max-w-180";
export const docTitleClass = "mb-3.5 min-w-0 text-[32px] leading-tight font-[650] tracking-[-0.012em] break-words outline-none";
```

`:388` 写成 `cn(docShellClass, "xl:flex xl:items-start")`（P4 的页的外层要排右栏；文档类的三页没有右栏，只取 `docShellClass`：带着 `xl:flex` 的外层，唯一的子元素会缩成内容宽）。`PageColumn`（4.1.2）S1 已导出。文档类的三页的早退用它时不传 `panel`（决定 951）：它们没有右栏，读取中、找不到与读到之后的列同在居中处。S2 导出它时，空的头一行由调用者定形：只有面包屑一块，或带调用者那一页右边一组的类名（变更集页的 `headActionsClass` 一组、版本页的那一格；回收站的预览只有面包屑），选项按"页的头"命名（今天的 `panel`）；今天不传 `panel` 的空头一行照页的样子排（面包屑一块加 26rem 的右边一组），这三页在 900 以下会差约 21.5 px（审查 B 的 P5S1B-F2-N1，决定 951 之下）。4.3.6 的表里这几行在 1440、768 与 375 宽同时量 x 与 y，与读到之后相同。

`page-classes.ts` 加文档类的头的右边一组与标题下的事实（它们不是 P4 的串）：

```ts
/** The head's right on a document's page: what the page does (Restore this version, Undo change); the page's own is P4's. */
export const headActionsClass = "ms-auto flex min-w-0 flex-wrap items-center justify-end gap-1.5";
/** The facts under a document's title, one look on the three pages (review B's P5B-M9): grey, its links a shade darker. */
export const docFactsClass = "text-[12.5px] text-faint [&_a]:text-muted-foreground";
```

变更集页的新结构：

```tsx
<div className={docShellClass}>
  <div className={headRowClass}>
    <div className={headCrumbsClass}><BreadcrumbTrail … /></div>
    {!data.trimmed && writer && <div className={headActionsClass}><RevertDialog … /></div>}
  </div>
  <div className={cn(docColumnClass, "space-y-4")}>
    <ColumnTop />
    <div><h1 … className={docTitleClass}>…</h1><ChangesetFacts … /></div>
    {/* revertAgain or the readers' note, noteClass; the status output; the Alert; the changes */}
  </div>
</div>
```

- `h1` 与 `ChangesetFacts` 照旧紧挨着（`changeset-page.test.tsx:106,257,495` 读 `heading.nextElementSibling`）；`h1` 照旧是页上唯一的（lw9:528）。`ChangesetFacts` 的根 `space-y-1 text-sm` → `cn(docFactsClass, "space-y-1")`，结构照旧（决定 929）；来处、建于与更新于、计数三行（`:365,368,405`）去掉自带的 `text-muted-foreground`，随根是一种灰；消息一行（`:378`）是用户写的内容，给 `text-sm text-foreground`（同搜索的摘录：内容不比元数据淡；处置交设计者定，取正文色，审查 B 的 P5B-R1-M3）。
- "撤销这次改动"（`W/pages/notebook/revert-dialog.tsx:160` 的触发按钮）加 `size="sm" shape="pill"`，挪到头一行：它的 `ref`、`aria-busy` 与对话框关上之后焦点回到它（`revert-dialog.test.tsx:165,247,665,1351`）照旧，按钮不卸下；`RevertDialog` 自己的 `useFocusLeaving(trigger, …)`（`revert-dialog.tsx:98`）跟着按钮走，挪走它不丢交接。"再撤销一次"的说明（`history.revertAgain`）与读者的说明照旧在原来的条件下，挪到事实之后，`noteClass`。
- 改动的列表（`:458-459`）→ `<SettingsCard list label={t("history.changes")}>`。`W/pages/notebook/changeset-change.tsx`：`:105` 的 `li.space-y-1.p-4` → `stackedLinesClass`；`:106` 的第一行 → `cn(lineTitleClass, "flex-wrap gap-x-3 gap-y-1")`；`:108-116` 的主链接 → `lineLinkClass`（`open.state` 照旧）；不在了的节点的名字（`:119`）`min-w-0 font-medium break-words` → `min-w-0 break-words`；"In the trash"（`:120-127`）与"History of this page"（`:131-137`）→ `cn(lineLinkClass, "font-normal text-muted-foreground hover:text-foreground")`，加 `state={arrived}`（决定 940）；同名时的 id（`:130`）`text-sm text-muted-foreground` → `font-normal text-muted-foreground`；`:140` 的第二行 → `cn(factsClass, "flex flex-wrap gap-x-3")`，其中内容的版本（`:148-154`）`underline underline-offset-4` → `cn(lineLinkClass, "text-muted-foreground hover:text-foreground")`（同第一行的次要链接，悬停才有下划线，审查 B 的 P5B-R1-N4），加 `state={arrived}`（决定 940）。行里的 `span` 照旧（`changeset-page.test.tsx:501,515`）。
- 内容链接（决定 940）：`SaidText` 的"See it"（`:296`）、"Reverts …"（`:387`）、"Reverted by"的各个（`:398`）加 `state={arrived}`；"See the jobs"（`:344`，`startUnread`）加 `state={arrived}`（决定 924）。
- 早退（决定 915，第 1 节）：`ChangesetPage` 的 `:69`（地址不是 id）`return <PageColumn><NotFoundPage /></PageColumn>`；`ChangesetShell` 的 `:92`（别的笔记本的，或读到 404 `page.changeset_not_found`）同样包 `NotFoundPage`；`:95`（读取中，或读不到：变更集的读失败）包 `NotLoaded`。 这几处的 `PageColumn` 都不传 `panel`，空的头一行照这一页的头定形（决定 951，审查 B 的 P5S1B-F2-N1）。
- 路由 `:247-253` 加 `handle`。

#### 4.2.7 回收站（提交 2）

`W/pages/notebook/trash-page.tsx:181-232`：根 → `listColumnClass`，`ColumnTop`；`h1` → `pageTitleClass`（`aria-describedby` 照旧，`e2e/fixtures/trash.ts:46`）；两句说明（`trash.hint`、`trash.purge`）→ `noteClass`；状态行与 `SaidText` 照旧，其中"See the jobs"（`:274`，后台恢复的答复丢了、任务读不到）加 `state={arrived}`（决定 924）；带名字的"在后台恢复"（`:201-208` 的 `offered`）见 4.2.8 的同一个按钮；`trash.empty` → `<EmptyState icon={Trash2}>`；`:213` 的 `ul` → `<SettingsCard list label={title}>`。路由 `:202-209` 加 `handle`。

`W/pages/notebook/trash-row.tsx`：

- `TrashRow` 的 `li`（`:498`）→ `splitRowClass`；行里照旧先是链接、再是"恢复"（`trash-page.test.tsx:446-447` 两次 Tab）。
- `:499` 文字的一块 `min-w-0 space-y-1` → `cn(listTextClass, "space-y-1")`：它取"恢复"留下的宽，"恢复"在右上（审查 B 的 P5B-M6：今天长的事实让它折到文字之下、靠左）。窄屏时 `basis-56` 加"恢复"宽过行里的宽（375 宽时 289 px），文字一块占满一行、"恢复"在它之下；行里的 `RestoreButton` 给 `className="ms-auto"`，在下面时靠右，同标题行的按钮（初稿说"总在右上"，375、320 宽时不是，审查 B 的 P5B-R1-N7）。
- `ItemIcon`（`:271-280`）自画的两个图标（`ScrollText`、`File`）`text-muted-foreground` → `text-faint`，同 `AssetIcon`（4.2.4）；它收 `className?: string`（预览用，4.2.8），两个图标的类名写成 `cn("size-4 shrink-0 text-faint", className)`，附件那一支交给 `AssetIcon`（`<AssetIcon mime={…} className={className} />`），预览里附件的图标与页的同大、同位置（审查 B 的 P5B-R1-N2：今天的写法给预览的 `className` 到不了 `AssetIcon`，附件是 16 px、贴顶）。
- `:500` 的第一行 `flex items-center gap-2 font-medium` → `lineTitleClass`；`:502-509` 的链接 `min-w-0 break-words underline underline-offset-4` → `lineLinkClass`，加 `state={arrived}`（决定 940：去预览，它的 `h1` 在到达时取焦点）。
- `TrashFacts`（`:288-319`）收 `className`，几行 `p` 包进一个带它的 `div`，`p` 自己只留 `break-words`（今天每个 `p` 是 `text-sm break-words text-muted-foreground`）：行里给 `cn(factsClass, "space-y-1")`，预览给 `cn(docFactsClass, "mt-2.5 space-y-0.5")`（4.2.8）。"恢复到…"那一行照旧是最后一个 `p`：`trash-page.test.tsx:120,147` 读每行的 `p:last-child`，包了一层之后它仍是那个 `div` 的最后一个子元素，第一行的 `p` 不是（它之后是那个 `div`）。
- `RestoreButton`（`:449-459`）加 `size="sm" shape="pill"`，另收 `className` 交给 `Button`（带名字的"在后台恢复"用，4.2.8）。

#### 4.2.8 回收站的预览（提交 2）

`W/pages/notebook/trash-preview.tsx`：

- `shell`（`:192-202`）：有面包屑时外层是 `docShellClass`（没有时，"不在回收站里"与"已过期"两支，不带它：没有头一行，同页不在时的 `PageColumn`）；`Trail`（`:336-344`）的 `div.mb-2` → `headRowClass`，里面的面包屑包在 `headCrumbsClass` 的 `div` 里（`useFocusLeaving` 照旧在 `Trail` 自己的 `div` 上）；`Alert` 与 `view` 进 `docColumnClass` 的 `div`，它的第一个子元素是 `<ColumnTop />`（面包屑照旧在提示之前：`trash-preview.test.tsx:928`）。
- `Preview`（`:396-460`）：`div ref={shown}`（`:419`）照旧包着标题、"恢复"与其余（决定 929）；标题一行（`:421`）`flex items-start justify-between gap-3` → `flex flex-wrap items-start justify-between gap-x-3 gap-y-2`；`h1`（`:422-430`）→ `cn(docTitleClass, titleTextClass, "mb-0 flex items-start gap-2.5")`（取按钮留下的宽，审查 B 的 P5B-M7），`ItemIcon`（`trash-row.tsx:271-280`）收 `className`、这里给 `mt-1.5 size-6 shrink-0 text-faint`，名字进 `<span className="min-w-0 wrap-anywhere">`（认领的"长名"，决定 937）；按钮的一组（`:432`）`flex flex-wrap justify-end gap-2` → `ms-auto flex flex-wrap justify-end gap-2`（在标题之下时仍靠右，审查 B 的 P5B-Q2）；两个 `RestoreButton` 加 `size="sm" shape="pill"`；带名字的"在后台恢复"（`:439-446`）另给 `className="h-auto min-h-7.5 shrink rounded-[15px] py-1 whitespace-normal text-start wrap-anywhere"`（审查 B 的 P5B-M12：名字没有空格时今天 375、320 宽横向溢出；`shrink` 去掉 `Button` 的 `shrink-0`，不然它在 `justify-end` 的一行里向左伸出屏幕，第 1 节 (h)；圆角 15 px 是一行时的胶囊、折成几行时的圆角方块），回收站页的那一个（`trash-page.tsx:201-208`）同样给；`TrashFacts` 一块（`:450-452` 的 `div.space-y-1` 去掉）→ `TrashFacts` 带 `className={cn(docFactsClass, "mt-2.5 space-y-0.5")}`。
- 不在回收站里、已过期的两支（`:277-301`，`NotInTrash` 与 `Gone`）照旧经 `shell` 画在它的列里，`Gone` 的 `section`（`:591-605`）与 `NotInTrash` 的只把 `h1` 换成 `docTitleClass`，自己不画 `ColumnTop`、不另套列（审查 A 的 P5A-M3：初稿让它们"进同样的列、`ColumnTop` 在前"，与 `shell` 的列重了，会有两个横幅、两层外层）。
- 早退（决定 915，第 1 节）：`TrashPreview` 的 `:66`（地址不是 id）、`PreviewShell` 的 `:249`（读到别的笔记本的项）与 `:270`（`page.not_found`：不在回收站里也不在树上，或笔记本没了）各 `return <PageColumn><NotFoundPage /></PageColumn>`；`:306-308`（读取中，或读不到：回收站项的读失败）包 `NotLoaded`。`:237-246`、`:274` 是 `<Navigate>`，不画东西。 这几处的 `PageColumn` 都不传 `panel`，空的头一行照这一页的头定形（决定 951，审查 B 的 P5S1B-F2-N1）。
- `LostStart`（`:352-366`）里"See the jobs"的 `Link`（`:361`）加 `state={arrived}`（决定 924）。
- 路由 `:210-216` 加 `handle`。

#### 4.2.9 目录（提交 2）

`W/pages/notebook/index-page.tsx`：`:61` → `listColumnClass`，`ColumnTop`；`:62` 的 `h1` → `pageTitleClass`；`:68,71` 的两句 → `noteClass`；`:270,277` 两个链接照旧（链接在句子里，带下划线；`:277` 去健康页的锚点，不加 `arrived`，决定 940）；`:294-296` 的两种空 → `<EmptyState icon={ListTree}>`（文字照旧，`index-page.test.tsx:424-430`）。筛选见 4.2.2。路由 `:217-223` 加 `handle`。

`W/pages/notebook/index-rows.tsx`：`:52` 的区域照旧（`section aria-label`，`space-y-6`）；分组的 `section` 照旧（lw13:176-177 数它们），各组的 `section`（`:64`）去掉 `space-y-2`，`h2`（`:65`，`text-lg font-semibold break-words`）→ `cn(groupTitleClass, "break-words")`（它自带 `mb-2.5`）；`:66` 的 `ul` → `cardClass`（没有名字，照旧）；行 `li.space-y-2.p-4`（`:68`）→ `cn(stackedLinesClass, "space-y-1.5")`；链接（`:69-76`，`arrived` 照旧）包进 `<p className={lineTitleClass}>`，类名 `font-medium break-words underline underline-offset-4` → `lineLinkClass`；类型（`:78`）、"摘要截断"的说明（`:86`）、标签（`:88`）与时刻、反链数（`:96`）的几行 → `factsClass`（标签与时刻的两行加回各自的 `flex flex-wrap gap-…`）；摘要（`:81-84`）`break-words` → `text-sm break-words`，同搜索的摘录（审查 B 的 P5B-I2）。行里的 `p` 多了第一行那一个：`index-page.test.tsx:439-440` 读行里文字是空白的 `p`，不数个数，照旧。

#### 4.2.10 标签（提交 2）

`W/pages/notebook/tag-page.tsx:39-63`：根 → `listColumnClass`，`ColumnTop`；`:40` 的 `h1` → `pageTitleClass`；`:52` → `<EmptyState icon={Tag}>`；`SubpageList` 照旧（S1 已换样子）。路由 `:254-262` 加 `handle`。

#### 4.2.11 焦点、地标与名字的走查（S2）

- 焦点的去向：多出的都是"到达"：改写的行与头里的内容链接带 `arrived`（决定 940），三个"See the jobs"带 `arrived`（决定 924），去处的 `h1` 取焦点，不再落到 `body`。别的不改：各页 `h1` 的到达、恢复与撤销之后的 `h1`、"只看此人"之后到"谁"、行随恢复离开时交给 `h1`（`useFocusLeaving`）、预览被换掉时的交接（`shown` 不变）。Tab 的次序：搜索照旧"标题 → 在哪里找 → 第一个命中"（se1:309-316，筛选条里只有那一个下拉）；变更集页的"撤销这次改动"挪到了 `h1` 之前：从页首 Tab 是面包屑、按钮、事实里的链接，到达之后焦点在 `h1`，从它起 Tab 先到事实里的链接、跳过按钮（同页的"编辑"，决定 929；没有格读这一段的次序，第 1 节 (f)），读屏的阅读次序随之是面包屑、按钮、标题（收尾记进读屏清单）；回收站的预览的"恢复"照旧在标题之后。
- 地标：没有新的；列表的名字照旧（"搜索结果""变更""回收站""变更"……）。
- 显出的操作透明时照样可聚焦、照样在可访问性树里（不是 `hidden`）；`group-focus-within/row` 让焦点进了这一行（主链接）时它先显出，Tab 走到它时已看得见。
- 375 px：筛选条折行，每个组 `max-w-full`；卡片里的行不溢出（`min-w-0`、`break-words`）；回收站的预览与回收站页上带名字的"在后台恢复"折行；mc3:174-233（历史、筛到一页、变更集页的头）、se1:401-421、lw13:300-317 照旧要过，mc3 在 375 宽另核对"只看此人"一直显示（5.2）；TB1 新的一格看预览（5.2）。
- 强制色：显出的操作一直显示；卡片与筛选的小块的边由系统色画出。
### 4.3 S3 版本、健康、升级、附件与对话框

#### 4.3.1 版本页（提交 1）

`W/pages/page/version-page.tsx:158-201`（`docShellClass` 等取自 `page-layout.tsx`，它本来就导入那个文件，`:22`；`headActionsClass`、`docFactsClass` 取自 `page-classes.ts`）：

```tsx
<div className={docShellClass}>
  <div className={headRowClass}>
    <div className={headCrumbsClass}><Breadcrumbs … part={title} /></div>
    {canEditPageBody(pages, page.id) && (
      <div className={cn(headActionsClass, "min-h-7.5 flex-[0_1_10rem]")}>
        {data !== undefined && newest !== undefined && newest.id !== data.id && <VersionRestore … />}
      </div>
    )}
  </div>
  <div className={cn(docColumnClass, "space-y-5")}>
    <ColumnTop />
    <div>
      <h1 ref={heading} tabIndex={-1} className={docTitleClass}>{title}</h1>
      {data !== undefined && (
        <p className={cn(docFactsClass, "flex flex-wrap items-center gap-x-2 gap-y-1")}>
          <time …/>
          <span className="min-w-0 wrap-anywhere">·{" "}<Link to={`${home}/history/${data.changeset.id}`} state={arrived} className="underline underline-offset-4">{versionBy(data, t)}</Link></span>
          {newest?.id === data.id && <Mark current>{t("version.current")}</Mark>}
          {data.editing && <Mark>{t("version.editing")}</Mark>}
        </p>
      )}
      <output className="mt-1 block text-compact text-muted-foreground empty:sr-only">…</output>
    </div>
    <EditLockNote … />
    {/* NotLoaded or the Tabs, as today */}
  </div>
</div>
```

- "还原这个版本"（`W/pages/page/version-restore.tsx:73` 的触发按钮）加 `size="sm" shape="pill"`，进头一行（决定 929）：320 宽时标题不再被它挤成两行（第 1 节 (h)；hi2 核对，5.3）。显示的条件照旧（`canEditPageBody … newest.id !== data.id`），只是它的那一格对能编辑的人总在、定宽、至少 30 px 高（决定 929，审查 A 的 P5A-M4）：版本列表晚到、别人把这个版本还原成最新时按钮出现或消失，头一行的行数与标题的位置都不变。按钮照旧不随状态卸下（`hi2:132`"没有改动"之后焦点在它上面）。
- 作者那一个 `span` 照旧（`version-page.test.tsx:69` 读它的 `parentElement` 的文字"· Ada · Edit"），`wrap-anywhere` 照旧（PG17 的 375 宽一格，P4 的 P4S1B-F1）；它的链接加 `state={arrived}`（决定 940：去变更集页，它的 `h1` 在到达时取焦点），颜色由 `docFactsClass` 的 `[&_a]:text-muted-foreground` 给。
- 早退（决定 915，审查 A 的 P5A-I1）：`VersionPage` 的 `:76-78`（树还没读到或没确认：读取中，或树的读失败）`return <PageColumn><NotLoaded …/></PageColumn>`；`:79-82`（页不在，或版本号不合法）`return <PageColumn><PageGone …/></PageColumn>`。画在 `VersionPage` 里，不画在 `PageGone` 里：页的那一支已在 `PageColumn` 里，画在 `PageGone` 里就有两个。版本本身读不到（`page.version_not_found` 或读失败）的一支在 `VersionShell` 的列里、`h1` 之后（`:203` 的 `NotLoaded`），照旧。 这两处的 `PageColumn` 都不传 `panel`，空的头一行照这一页的头定形（决定 951，审查 B 的 P5S1B-F2-N1）。
- 横幅：路由 `:194-201` 加 `handle`，`ColumnTop` 在列里、标题之前；`upgrade-banner.test.tsx:209-220`"版本页的横幅不在列里"换成表里的版本页各行（4.3.6）。
- `Tabs` 与它的内容、`VersionView` 的阅读视图（P4/S2 的 `.nw-reading`）与对比视图照旧。

#### 4.3.2 `Mark` 与右栏的版本行（提交 1）

`W/pages/page/page-versions.tsx`：

- `Mark`（`:151-163`）：普通的那种 `rounded border px-1.5 text-xs text-muted-foreground` 换成导出的 `markClass = "rounded-full border border-border bg-surface-2 px-1.5 text-[11px] whitespace-nowrap text-muted-foreground"`（同 `badgeClass` 的形状，决定 930）；`current` 的那一串加 `whitespace-nowrap`（`rounded-full border border-transparent bg-accent-soft px-1.5 text-[11px] whitespace-nowrap text-accent`）：小块挪进行内之后汉字之间可以断，不加时"编辑中"会断成"编"与"辑中"两半（审查 B 的 P5B-R1-M2，第 1 节 (g) 的第四次扫描）；版本页的事实与历史行里 `Mark` 是弹性项，加了也不变。版本页的"当前"用 `current`（4.3.1）。注释（`:146-150`）里"历史与版本页用普通的"改成"历史用普通的"。
- 历史行的"Reverted"（`W/pages/notebook/history-row.tsx:107-113`）`rounded border px-1.5 text-xs underline underline-offset-4` → `cn(markClass, "hover:border-border-strong hover:text-foreground")`（决定 930，审查 B 的 P5B-M8；悬停不用下划线，审查 B 的 P5B-R1-N3）。
- 右栏的版本行（`:109-135`，决定 943）："当前""编辑中"从行尾自成一列的 `div.ms-auto.flex.shrink-0`（`:129-134`）挪进时间与作者那一块（`:112` 的 `div.min-w-0.flex-1.break-words`）的末尾，每个前面一个空格，不另包一层：

  ```tsx
  <div className="min-w-0 flex-1 break-words">
    <Link …>…</Link>
    <p className="inline text-muted-foreground before:mx-1 before:content-['·']"><VersionBy version={version} t={t} /></p>
    {current && <>{" "}<Mark current>{t("version.current")}</Mark></>}
    {version.editing && <>{" "}<Mark>{t("version.editing")}</Mark></>}
  </div>
  ```

  文字有了整行宽，十六进制的一段不再被小块挤到恰好占满一行，"-"不再单独一行（第 1 节 (g)：三种引擎、200–240 px 逐 1 px，四次都是 0；第四次另数了断成两半的小块，带 `whitespace-nowrap` 时也是 0）。`li` 的 `flex items-baseline gap-2` 照旧（只剩一个子元素）。`:110-111` 的注释（"the marks at the first line's end"）照实改成"the marks after it, in the same run"。读这一行的：`page-versions.test.tsx:21-29` 的 `listed()` 以 `span` 数小块、取 `p` 的文字，照旧；`:111-141` 那一格断言 `li` 的子元素是两个 `DIV`、第二个里是两个小块，改写成"`li` 一个 `DIV`，它的子元素是 `A`、`P` 与小块的 `SPAN`"；`:104` 的中文那一格期望的 `"5分钟前Ada · 撤销当前编辑中"` 改成 `"5分钟前Ada · 撤销 当前 编辑中"`（审查 A 的 P5A-R1-M2）；`version-restore.test.tsx:157` 的 `/Ada · RestoreCurrent$/`、hi1:259 的 `· EditCurrent`、hi2:114 的 `· RestoreCurrent` 改成中间一个空格（5.3）；`page-versions.test.tsx:110`、hi1:258、hi2:113 的注释（"the marks at its end""then the marks at the line's end"）随之改成"then its marks, after a space (M13/P5/S3, decision 943)"（审查 C 的 P5C-R1-N1）。

#### 4.3.3 健康（提交 1）

- `W/pages/notebook/health-page.tsx`：`:273` `section.min-w-0.space-y-4` → `cn(listColumnClass, "min-w-0")`，`ColumnTop`；`:274` 的一行 → `titleRowClass`；`:275` 的 `h1` → `cn(pageTitleClass, titleTextClass)`；"重新检查"（`:278`）加 `size="sm" shape="pill" className="ms-auto"`；`:292-298` 的几句 → `noteClass`（"检查中"等照旧是 `p`）；`:306-307` 的 `details`/`summary` → `summary` 换成 `cursor-pointer text-compact font-semibold text-muted-foreground`（照旧只有一个 `details`：lw8:673）；`:312` → `noteClass`。路由 `:224-230` 加 `handle`。
- `W/pages/notebook/health-rule.tsx:230-234`：`section` 照旧（`id`、`aria-labelledby`），`min-w-0 space-y-2 rounded-md border p-4` → `min-w-0 space-y-2 rounded-card border border-border bg-surface px-4.5 py-4`；`:235` 的标题加 `text-sm`；`:238` → `factsClass`；`:243` 的 `ul.divide-y` → `divide-y divide-border border-t border-border`（卡片里的列表与说明之间一条线）。不加别的带 `id` 的 `section`（lw8:625,678）、不加 `h2`（`health-page.test.tsx:252-256,313-330` 数它们的次序）。
- `W/pages/notebook/health-item.tsx`（同一种行，决定 925）：`:54` 的 `py-3` → `py-2.5`；`:56` 的 `div` → `lineTitleClass`；`:58-68` 的路径链接 `underline underline-offset-4` → `lineLinkClass`（`openOf` 的 `state` 照旧经 `useNodeNavigate`）；`:70-72` 读取中与"不可用"的 `span` 加 `font-normal`。
- Tab 的次序不变（lw8:834-836）；`main` 里第一个 `time` 照旧是检查的时刻（lw8:983 等）。

#### 4.3.4 升级（提交 1）

只换类名（决定 932）。`W/pages/notebook/upgrade-page.tsx`：`:43` 的根 `div.max-w-2xl.space-y-8` → `cn(listColumnClass, "space-y-8")`（照旧是 `div`），`ColumnTop` 是它的第一个子元素（路由 `:231-238` 加 `handle`）；`h1` → `pageTitleClass`；各节的 `h2`（`:614-615`、`:653-654`、`:698-699`，`W/pages/notebook/upgrade-check.tsx:231-232`、`:367-368`）`text-lg font-semibold` → `text-base font-semibold`；`:765` 的 `ul.rounded-md.border` → `cardClass`；`upgrade-check.tsx:438` 的列表同样；说明的 `p` → `noteClass`；通往页面的链接（`:606`）加 `state={arrived}`（决定 940：去页，它的 `h1` 在到达时取焦点）。各节的 `h2` 照旧是 `section` 的直接子元素（`upgrade-page.test.tsx:92,112,142,509` 读 `parentElement`），`h2` 与按钮的次序不变（`:864`）。

#### 4.3.5 附件一节（提交 2）

`W/pages/page/attachments-section.tsx`：

- `:211-215` 的 `section`：拖进来时的环 `ring-2 ring-primary ring-offset-4` → `ring-2 ring-accent ring-offset-4 ring-offset-background`，`rounded-md` → `rounded-card`（决定 933）。`W/pages/page/attachment-drop.tsx:51`（页的阅读视图那一块的拖放区）的环同样换成 `ring-2 ring-accent ring-offset-4 ring-offset-background`，它的 `rounded-md` 照旧（它包着整块阅读视图，P4 的样子）。
- `:218-226` 的标题行：`h2` 照旧 `text-sm font-medium text-muted-foreground`（同页上"子页面"一节的 `h2`，`page-layout.tsx:507`；决定 933）；"上传"加 `size="sm" shape="pill"`。
- `:258-260` 的拖放说明 → `text-compact text-faint`。
- `:284` 的 `ul` → `cardClass`（照旧 `aria-labelledby`、没有 `aria-label`：`W/test/attachments.ts:21-26` 找第一个没有 `aria-label` 的列表；`[aria-live=polite]` 照旧在 `section` 里）。

`W/pages/page/attachment-row.tsx`：

- `AssetIcon`（`:27-31`）已在 S2 提交 1 换成 `text-faint`、收 `className`（4.2.4）；来源的 `FileText`（`:132`）`text-muted-foreground` → `text-faint`，同它与下载的图标（审查 B 的 P5B-N2）。
- `:120-123` 的 `li` → `group/row flex items-center gap-2.5 px-4 py-2.5 text-sm`（照旧可拖、照旧是链接的父元素：lw5:578-580、`attachments-section.test.tsx:357,366`）；
- 链接（`:134-149`）照旧是 `li` 的直接子元素、文字照旧只是名字（与"来源"的 `sr-only`、`NewTab`）；
- 链接之后、大小之前：`{!inline && <Download aria-hidden className="size-3.5 shrink-0 text-faint" />}`（决定 933）；
- 大小（`:149-151`）`text-xs text-muted-foreground` → `text-compact text-faint`；
- "操作"（`:153-161`）`rounded p-1 text-muted-foreground hover:bg-hover` → `cn("rounded-row p-1 text-muted-foreground hover:bg-hover hover:text-foreground", revealClass)`（菜单开着时 `aria-expanded` 让它显着）。

`W/pages/page/upload-rows.tsx`（决定 933，审查 B 的 P5B-M11）：`:54` 的 `ul` `divide-y rounded-md border text-sm` → `cn(cardClass, "text-sm")`；`:93` 的行 `px-3 py-2` → `px-4 py-2.5`，`:176` 的 `px-3 py-3` → `px-4 py-3`。附件一节、编辑时的上传与粘贴来源的对话框同用它，一起变；`aria-label`（"上传"）照旧。

#### 4.3.6 升级横幅与列（提交 2；表从 S1 起）

- `W/pages/notebook/upgrade-banner.tsx:41`：`mb-6 block space-y-1 rounded-md border border-primary/40 bg-surface-2 px-4 py-3 text-sm` → `mb-6 block space-y-1 rounded-card border border-border bg-surface-2 px-4.5 py-3.5 text-sm`；`:42` 状态那一行 `span.block` → `flex items-start gap-2 font-medium`，文字之前一个 `<span aria-hidden className="mt-[calc(0.5lh-3px)] size-1.5 shrink-0 rounded-full border-3 border-accent" />`（决定 935：3 px 的琥珀边画的 6 px 小圆点，不是填色，"amber is no fill"不红，第 1 节 (j)；强制色下边由系统色画出；`items-start` 加上边距让点对齐第一行的中线，状态一行折行时不落在两行之间：三种引擎、中英、320 到 1024 宽，点的中心都离行顶 10 px，正是 20 px 行高的一半，第 1 节 (l)，审查 B 的 P5B-R1-N1）。状态的文字照旧是这个 `span` 唯一的文字，它的父元素照旧是 `output`（`bannerOf` 读 `queryByText(text)?.parentElement`）。
- `W/pages/notebook/notebook-layout.tsx`：删 `ColumnHandle`（`:23`）、`columned`（`:67-70`）与 `:97` 的横幅；`:98` → `<ColumnTopContext value={<UpgradeBanner notebook={notebook} />}>`；`useMatches` 的 import 若再没有用处一并删；`:94-96` 的注释照实改。`W/app/column-top.tsx:3-6` 的注释里"NotebookLayout gives the routes that draw it there (handle: { column: true }, or a settings page's)"同改成"NotebookLayout gives every page of a notebook"（审查 A 的 P5A-N8、C 的 Nit）。
- `W/app/routes.tsx`：笔记本下各子路由的 `handle: { column: true }` 删掉（`:188` 与 S1、S2、S3 加的十处）；`settings: true`（`:266`）留着。
- **"路由 × 支"表**（决定 915；审查 A 的 P5A-I1 与 C 的 P5C-M5 合并）：`W/pages/notebook/upgrade-banner.test.tsx` 从 S1 提交 2 起只有一个遍历的格 `test.each(columned)`，每一行是一个路由的一支：升级中的笔记本，照"造法"一列设好假服务端，`renderApp` 那一行的地址，等"先等到的"那个元素出来，然后断言：
  1. 页上恰好一个横幅：`screen.getAllByRole("status").filter((s) => s.textContent?.includes(said.upgrading))` 的长度是 1（笔记本设置的常规页另在自己的一行里说模式，不是 `status`；初稿的每页一格只看读到之后，也抓不到两个横幅，审查 A 的 P5A-M3 那一种）；
  2. 横幅的父元素是那一页的列的根：`column = banner.parentElement`，`column.firstElementChild === banner`、`column.classList.contains("mx-auto")`（列都居中；回退画的横幅在出口之上，它的父元素是 `main` 本身，它也是 `main` 的第一个子元素、`main` 也包含 `first`，三项里只有这一项分得出回退与列，审查 C 的 P5C-R1-N1），且 `column.contains(first)`；不比 `h1` 的父元素（笔记本首页、健康页的 `h1` 在标题行里，文档类的三页的 `h1` 在下一层的 `div` 里；初稿照 `:141-148` 写的比法在正确的实现上也红，C 的 P5C-M5）；
  3. 横幅在 `first` 之前，也在 `main` 里每一个 `h1` 之前（`compareDocumentPosition` 是 `FOLLOWING`）。

  表的行（"支"一列写明哪一个读、在哪一处早退；`first` 是那一支先等到的元素；"`h1`"指 `findByRole("heading", { level: 1 })`，"alert"指 `main` 里的 `alert`，"加载中"指横幅出来之后 `main` 里文字是"Loading…"的 `output`：这几行先等横幅（`waitFor` 到 `getAllByRole("status")` 里有含 `said.upgrading` 的那一个），再 `within(main).getByText("Loading…", { selector: "output" })`；直接 `findByText` 先等到的是外框在笔记本列表读到之前画的那一个"加载中"（`notebook-layout.tsx:75-77` 的 `NotLoaded`），它随即卸下，第 2、3 句在对的实现上也红，审查 C 的 P5C-R1-I1）：

  | Step | 路由 | 支 | 造法 | first |
  |---|---|---|---|---|
  | S1 提交 2 | 页 `pages/:pageId` | 读到 | `pageServer()`，`pagePath(guide.id)` | `h1` "Guide" |
  | S1 提交 2 | 页 | 树读取中（`page-layout.tsx:77-84`） | `nodesHeld = true` | 加载中 |
  | S1 提交 2 | 页 | 树读不到（同上） | `nodesDown = true` | alert |
  | S1 提交 2 | 页 | 页不在（`:86-93`） | 不存在的 id | `h1` "Page not found" |
  | S1 提交 2 | 笔记本设置 | 读到 | `…/settings/general` | `h1` "General" |
  | S1 提交 2 | 笔记本首页 `index` | 读到 | `/lab/notebooks/<id>` | `h1` "Plans" |
  | S1 提交 2 | 笔记本首页 | 树读取中（根里的 `NotLoaded`） | `nodesHeld = true` | 加载中 |
  | S1 提交 2 | 笔记本首页 | 树读不到 | `nodesDown = true` | alert |
  | S2 提交 1 | 历史 | 读到 | `…/history` | `h1` "History" |
  | S2 提交 1 | 历史 | 日志读不到（根里） | `historyDown = true` | alert |
  | S2 提交 1 | 变更集页 | 读到 | `server.changesets` 里一个，`…/history/<id>` | `h1` |
  | S2 提交 1 | 变更集页 | 地址不是 id（`changeset-page.tsx:69`） | `…/history/not-an-id` | `h1` "Page not found" |
  | S2 提交 1 | 变更集页 | 找不到（`:92`，404 `page.changeset_not_found`） | 合法但不在的 id | `h1` "Page not found" |
  | S2 提交 1 | 变更集页 | 读取中（`:95`） | `historyWaits.changeset` 一个不兑现的 `Promise` | 加载中 |
  | S2 提交 1 | 变更集页 | 变更集读不到（`:95`） | `historyDown = true` | alert |
  | S2 提交 2 | 回收站 | 读到 | 一项在回收站里，`…/trash` | `h1` "Trash" |
  | S2 提交 2 | 回收站 | 列表读不到（根里） | `trashDown = true` | alert |
  | S2 提交 2 | 回收站的预览 | 读到 | 一项，`trashPath` | `h1`（项的名字） |
  | S2 提交 2 | 回收站的预览 | 地址不是 id（`trash-preview.tsx:66`） | `…/trash/not-an-id` | `h1` "Page not found" |
  | S2 提交 2 | 回收站的预览 | 别的笔记本的项（`:249`） | `server.trash` 里一项，根节点的 `notebook_id` 换成别的 | `h1` "Page not found" |
  | S2 提交 2 | 回收站的预览 | 不在回收站也不在树上（`:270`，`page.not_found`） | 合法但哪里都没有的 id | `h1` "Page not found" |
  | S2 提交 2 | 回收站的预览 | 不在回收站里（`:277-288`，`page.not_in_trash`，`shell` 的列） | 一个还在树上的页的 id | `h1`（`NotInTrash` 的） |
  | S2 提交 2 | 回收站的预览 | 已过期（`:290-301`，`page.restore_expired`，`shell` 的列） | 一项 `expired: true` | `h1`（`Gone` 的） |
  | S2 提交 2 | 回收站的预览 | 读取中（`:306-308`） | `trashWaits.item` 一个不兑现的 `Promise` | 加载中 |
  | S2 提交 2 | 回收站的预览 | 回收站项读不到（`:306-308`） | `trashDown = true` | alert |
  | S2 提交 2 | 目录 | 读到 | `indexServer()`，`fixture.server.notebook` 设升级中 | `h1` |
  | S2 提交 2 | 标签 | 读到 | 带标签的页，`…/tags/<tag>` | `h1` |
  | S2 提交 2 | 标签 | 树读不到（根里） | `nodesDown = true` | alert |
  | S3 提交 1 | 版本页 | 读到 | `withVersions`，`…/versions/1` | `h1` "Version 1" |
  | S3 提交 1 | 版本页 | 树读取中（`version-page.tsx:76-78`） | `nodesHeld = true` | 加载中 |
  | S3 提交 1 | 版本页 | 树读不到（`:76-78`） | `nodesDown = true` | alert |
  | S3 提交 1 | 版本页 | 页不在（`:79-82`） | 不存在的页 id | `h1` "Page not found" |
  | S3 提交 1 | 版本页 | 版本号不合法（`:79-82`） | `…/versions/abc` | `h1` "Page not found" |
  | S3 提交 1 | 版本页 | 版本读不到（`VersionShell` 的列，`:203`，404 `page.version_not_found`） | `…/versions/99` | alert（`h1` "Version 99" 在它之前） |
  | S3 提交 1 | 健康 | 读到 | `healthServer()`，同目录的写法 | `h1` |
  | S3 提交 1 | 升级 | 读到 | `pageServer()`，`upgradePath` | `h1` "Upgrade to an LLM Wiki" |

  除了页与笔记本设置的"读到"两行（P4 已把横幅放进这两处的列，旧树上就绿，是搬进表里、守着不退回的行，审查 C 的 P5C-R1-M1、A 的 P5A-R1-N2），每一行在加它的那个提交里先红（照 7.1：先写测试、在没改的树上看到红；审查 C 的探针在 `f62d0390` 上逐行跑过，其余 34 行都红在第 2 句的 `mx-auto`）：没加 `handle` 时横幅由回退画在出口之上，第 2 句红；页的两支（S1）今天是片段，同样红在第 2 句。S3 提交 2 去掉回退时表不先红（那时每个路由都已 `handle: column`，回退走不到），由变异 57、58、61 证它守得住（审查 C 的 P5C-M5）。这一格取代 `upgrade-banner.test.tsx:141-151`（页）、`:153-163`（页不在）、`:165-177`（树读不到）三格（S1 提交 2）与 `:209-220`（版本页"不在列里"，S3 提交 1）；`:179-207`（笔记本设置的 `index` 一支，`MutationObserver` 看横幅从第一次渲染起就在设置的列里）照旧，它看的是重定向途中的每一刻，表看不到。表的行只写在这一个文件里，各页自己的测试文件不另写横幅的格（初稿每页一格，与 S3 的遍历重复，审查 C 的"过度设计"一条）。
- 那时画 `<ColumnTop />` 的笔记本页（逐个核对，每个都在它的列的最上面、每一支都在列里）：笔记本首页（S1）；页（P4，早退的两支 S1）；版本页（S3）；回收站、回收站的预览、目录、历史、变更集页、标签（S2）；健康、升级（S3）；笔记本设置（P4，`settings-page.tsx:38` 与 `W/pages/notebook/settings-layout.tsx` 的 `index` 一支）。共 12 个路由，都在上面的表里。

#### 4.3.7 对话框的内容（提交 2）

- `W/pages/notebook/import-dialog.tsx:139` 与 `W/pages/notebook/source-add-dialog.tsx:135` 的 `div role=alert`：`space-y-3 rounded-md border px-4 py-3 text-sm` → `space-y-3 rounded-lg border border-border px-4 py-3 text-sm`（只有边，决定 934）。
- `W/components/ui/input.tsx`：`Input` 的类名里除了高度、上下内边距与 `file:` 的那一串，抽成导出的 `fieldClass`（同一串，`Input` 改成 `cn("h-9 py-1", fieldClass, "file:…", className)`，生成的类不变）：

  ```ts
  /** A text field's look, an input's and a textarea's: its ground, border, corners, focus and invalid state (decision 934). */
  export const fieldClass = cn(
    "w-full min-w-0 rounded-control border border-border bg-surface-2 px-3 text-base text-foreground outline-none transition-[color,background-color,border-color,box-shadow] duration-150 md:text-sm",
    "placeholder:text-faint focus-visible:border-ring focus-visible:bg-surface focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden",
    "disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive"
  );
  ```

- `source-add-dialog.tsx:263-271` 的 `textarea`：类名 → `cn(fieldClass, "resize-y py-2 font-mono")`（决定 934，审查 B 的 P5B-M10：底与圆角同旁边的"标题"输入框，强制色下聚焦时有透明的外框）；`:272-277` 的说明 `text-sm` → `text-compact`（错误时照旧 `text-destructive`）。
- `source-add-dialog.tsx:261`：`space-y-2` → `space-y-1.5`（决定 926）。

#### 4.3.8 焦点、地标与名字的走查（S3）

- 版本页：从页首 Tab 的次序是"面包屑 → 还原这个版本 → 作者的链接 → 标签页"，不变（今天"还原"在 `h1` 之后、作者之前）；到达之后焦点在 `h1`，"还原"在它之前，从 `h1` 起 Tab 先到作者的链接，同页的"编辑"（决定 929；初稿写"次序其实相同"，只对从页首 Tab 成立，审查 A 的 P5A-N3）。`h1` 的到达焦点、"没有改动"之后焦点在按钮上、还原之后去页的 `h1` 照旧；作者的链接带 `arrived`（决定 940）。
- 健康：每张卡片照旧是以规则的标题命名的区域（`aria-labelledby`），`h2`/`h3` 的层级不变。
- 附件：下载图标 `aria-hidden`；"操作"透明时照样可聚焦（`aria-label` 照旧"<名字> 的操作"）。
- 横幅：照旧是 `<output>`，在列里、标题之前（每个笔记本页的每一支同一处，读屏从标题往回读得到它）；小圆点 `aria-hidden`。
- 右栏的版本行："当前""编辑中"接在作者之后读（今天在它之后另一块里，读的次序相同）。
- 375 px：版本页的头一行在 375、320 宽折行，能编辑的人的那一格在面包屑之下靠右（`ms-auto`），看最新的版本时空着；pg17:392-421、pg2:193-238、lw3:310-318、lw8:648-656 照旧要过。

### 4.4 对比度的下限

- **se1**（S2，决定 936）：暗色的 `link` 下限照实量减 0.1 改（推算约 7.31 → 约 7.21）；`marked`、`named`（命中的字，7:1）照旧：暗色的高亮字在卡片上推算 7.28，仍在 7 之上，量出来低于 7 时停下查（4.7"正文至少 7:1"）。注释照 P1–P4 的写法加一句"From M13/P5/S2 the hits are in a card, on --surface: the link's ring re-measured there"，写进量出的值。亮色与 `box`、`button`、`select`、`invalid` 不变。
- pg4、l5 不在 P5 的页上，不动。

### 4.5 lint、分包与设计基础的检查

- `W/design-tokens.test.ts:319-339` 的 `noColour` 加 `text-center`、`text-balance`、`border-dashed`（S1 提交 2；探针 (c)、(j)）。
- 新的模块都在已有的分包里（首页、入口与各页照旧按路由懒加载；`page-classes.ts`、`empty-state.tsx`、`entry-card.tsx`、`status-page.tsx` 是普通的共用模块，Vite 放进用到它们的块）。`entry-card.tsx` 与 `status-page.tsx` 用在不按需加载的路由上（登录、出错的边界），进主块；它们只有几十行，不另分。变更集页与回收站的预览从 S2 起静态导入 `page-layout.tsx` 的块（决定 925，第 1 节 (i)：两个路由的闭包各多约 64–71 KB，其中 `page-layout` 一块 gzip 约 19.5 KB）；它是懒加载的块之一，`e2e/perf`、`web/apps/web/build/out-of-main.ts` 只看主块，不受影响。
- knip：每个新的导出在它的提交里有别的文件用到（第 0 节）：S1 提交 2 的 `cardClass`、`pageTitleClass`、`homeColumnClass`、`listColumnClass`、`titleRowClass`、`titleTextClass`、`groupTitleClass`、`EmptyState`（`iconTileClass` 不导出）；提交 3 的 `appName`、`buttonClass`、`iconTileClass`、`EntryCard`、`entryTitleClass`、`StatusPage`、`statusBodyClass`、`statusActionsClass`；S2 提交 1 的列表、筛选、显出与文档头各串、`page-layout.tsx` 的五串与 `PageColumn`（`splitRowClass` 在提交 2）；S3 提交 1 的 `markClass`（历史行用），提交 2 的 `fieldClass`。`cardTextClass` 在 S1 之后没有用处，删掉。
- `make lint-web` 的 `check.mjs`：S1 加四行 `REQUIRED`（5.1）。

## 5. e2e

只列要改的与新加的；没列的照旧（第 1 节 (f) 逐个读过：读这些页的 e2e 大多以角色、名字与文字找，P5 不改名字与文字）。

### 5.1 S1

- **`expectFits` 收窗口的宽度**（提交 2，`e2e/fixtures/narrow.ts:11-29`，审查 C 的 P5C-I1）：今天第一句拿常量 `phoneWidth` 比页宽，320 宽的检查在正确的实现上也必红。改成取窗口的宽度、调用的写法不变：

  ```ts
  /**
   * expectFits checks, at the window's width (phoneWidth, or narrower: 320, M13/P5), that page scrolls not sideways,
   * and that each of boxes keeps to the screen and, one not inline (…as today…), holds its content.
   */
  export async function expectFits(page: Page, ...boxes: Locator[]): Promise<void> {
    const screen = page.viewportSize()?.width ?? phoneWidth;
    // …
    expect(width, "the page's width").toEqual({ viewport: screen, content: screen });
    // …
    expect(placed.right, `${String(box)}'s right`).toBeLessThanOrEqual(screen);
  ```

  今天的调用方 17 处都先设了 375 宽（第 1 节），照旧比 375，不改：pg2:184,211、pg17:288,417,654、ui2:169、ui1:54,67、lw3:346,357,364、mc3:202,214,220,226,233、se1:420。
- **UI1 新的第 4 格**（提交 2，`e2e/stories/shell/ui1-narrow-drawer.spec.ts` 末尾）`UI1 (page): at 375 wide the workspace's home and a notebook's home keep to the screen, the home's card at 320 too, a long name cut on its card and in the list of pages, broken in the title`：
  1. `newOnboardedTeam`；经接口建三本笔记本：`Plans`、一本 LLM Wiki（`createNotebook` 的第六个参数 `{ mode: "llm_wiki" }`），一本名字是连字符接数字、没有空格的 60 个字符（照 pg17:33-40 的 `unbroken` 在本文件里另写一个 `longName`）；在 `Plans` 里建一页同样长名字的页与它的一个子页。
  2. `signedInPage`，`setViewportSize({ width: phoneWidth, height: 800 })`，`goto` 工作区首页：`expectFits(page, 长名字的卡片)`；卡片的链接名含"LLM Wiki"（那一本）与"· 1 member"；长名字的卡片里名字的 `span`（`getByText(longName, { exact: true })`）一行（高除以 `line-height` 小于 1.5）且被截短（`scrollWidth > clientWidth`）。不比卡片的高：卡片带 640 px 起的 `sm:min-h-28`，375 宽时随内容高，截短与折成两行的名字都可能同高（审查 C 的 P5C-I2，变异 6）。
  3. 宽 320：同样 `expectFits(page, 长名字的卡片)`（`expectFits` 取 320，第 1 节 (b) 的溢出在这一档最明显）。
  4. 回到 375，`goto` 长名字那本笔记本的首页：`h1` 是那个名字、多于一行（高除以 `line-height` 不小于 1.5），"New page"在它之下、右边与 `h1` 的右边齐（差不到 1 px：标题取按钮留下的宽，按钮 `ms-auto`）；`expectFits(page, h1, "New page")`。三种引擎都要看：Firefox 不在连字符与数字之间断（P4 的 P4S1B-F1），`h1` 少了 `min-w-0` 就以整个名字作最小宽度撑出屏（变异 62）。
  5. `goto` `Plans` 的首页：`expectFits(page, 子页面列表里长名字的那一行)`；那一行的名字的 `span` 一行、被截短（同第 2 步）；那一行的链接的可访问描述是"1 subpage"（`toHaveAccessibleDescription("1 subpage")`：数在链接之外、`aria-hidden`，读屏经 `aria-describedby` 听到）。标题"Plans"与"New page"在同一行（按钮的上边在 `h1` 的下边之上：640 px 以下标题只有它的字那么宽，`max-sm:basis-auto`，变异 96；比的是两者的位置，不数行：Linux 的字体更宽，行数会变，审查 C 的 P5C-R2-N2）。
  只有页面版本：只有界面的样子，接口与 MCP 版本豁免（5.5）。进 `threeBrowsers`（文件已在）；`REQUIRED` 加 firefox、webkit 两行（`tools/e2e-projects/check.mjs`，前缀 `UI1 (page): at 375 wide the workspace's home`）：卡片网格的隐式轨（第 1 节 (b) 在三种引擎里都溢出）与第 4 步的标题折行（Firefox 的断法与别的两种不同）都是三种引擎可能不一样的排版。
- **UI1 新的第 5 格**（提交 3）`UI1 (page): at 375 wide the sign-in card and a page not found keep to the screen, the product's name above the form; the invitations to a workspace whose name has no break and to an address that has none break in their cards, at 375 and at 320`：
  1. 不登录的 `page`，375 宽，`goto` `/sign-in`：卡片里有"Nerve Wiki"（`getByText("Nerve Wiki", { exact: true })`，不是标题），`h1`"Sign in"；`expectFits(page, 表单)`。
  2. `signedInPage`，`goto` 一个不存在的工作区（`/` 加 `slugFor(testInfo, "nowhere")`）：`h1`"Page not found"，"Go to the home page"是链接；`expectFits(page)`；按下它，工作区首页的 `h1` 拿着焦点（决定 924）。
  同上豁免；`REQUIRED` 加 firefox、webkit 两行（前缀 `UI1 (page): at 375 wide the sign-in card`）。
- **照实**（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：两格在 `ui1-narrow-drawer.spec.ts:180`、`:307`。第 4 格第 5 步另断言名字的右边不超过数的左边（`:234`，`9975478d`，变异 t6）与"1 subpage"那一句藏着（`:238` 的 `toBeHidden`，审查 C 的 P5S1C-M2）。第 5 格另有一步（审查 B 的 P5S1B-I1，`4340e64a`）：未登录开一个工作区名 60 个字符、没有断点的邀请，已登录（`anotherPage`，`registerOnboarded` 注册的地址，本地部分是 `christopheralexanderwellington` 加 `slugFor` 的哈希、截到 40 个字符）开另一个，375 与 320 宽各 `expectFits`，卡片里每个盒子与每段文字的左右边都在卡片之内（`expectInvitationsFit`、`edgesIn`）；造数写在格里，不改夹具。`expectFits` 另断言窗口不宽于 `phoneWidth`（审查 A 的 P5S1A-N4）。UP1 (page)（`e2e/stories/upgrade/up1-start-freeze.spec.ts:172`）末尾加一步（决定 946）：扣住树的读，在 1440、1280、1024、768、375 宽各量读取中与读到之后横幅的 x、y、宽，逐个相等（`:268`）。三格的名字写出新加的步（审查 C 的 P5S1C-F1-N3，`dc59e9e2`），`REQUIRED` 的前缀照旧对得上，`check.mjs` 的数不变。
- 不改的（核对过）：a1–a15、w1–w12、n1–n13、s2、s4 以角色与名字找入口、首页与 404 的元素（第 1 节 (f)）；w5 以名字找邀请页的按钮与链接，右上角多一个"Back to the workspace"不撞（决定 941）；`n1:116` 是访客，没有虚线卡；`createNotebookWith`（`e2e/fixtures/notebook-pages.ts:55`）在左栏的 `nav` 里找"新建笔记本"，主区里多一个同名的不撞；pg16:121-123 只看笔记本首页的标题；`e2e/perf/tree.perf.ts:249`、`perf/probe.ts:157,174,208` 数 `ul[aria-label="Pages"] > li`（照旧每页一个）；ui1:40-54（页上的子页面与附件一节，S1 换了 `SubpageList`）、ui2:143-169（任务的空列表，S1 起是卡片大小的空状态，`expectFits` 照旧要过）；17 处 `expectFits` 的调用（上面）。

### 5.2 S2

- **se1**（提交 1，`e2e/stories/search/se1-search.spec.ts:282-292`）：注释末尾加一句（4.4），`floors.dark.link` 改成实量减 0.1。实施者先在改了样子的树上跑一次 se1、读出暗色 `link` 的比，写进注释与 10.2；量出低于 7.21 很多（推算的 7.31 不对）时停下查。
- **hi3**（提交 1，`e2e/stories/history/hi3-log-revert.spec.ts:328-331`，"只看此人"的点按之前）：

  ```ts
  const row = listed.nth(2);
  const only = row.getByRole("button", { name: `Only this person (${bName})`, exact: true });
  const link = row.getByRole("link").first();
  // Transparent at rest, shown once the row's link holds the keyboard's focus, and in forced colours (decision 927).
  await page.mouse.move(0, 0);
  await expect(only).toHaveCSS("opacity", "0");
  await link.focus();
  await expect(only).toHaveCSS("opacity", "1");
  await link.blur();
  await expect(only).toHaveCSS("opacity", "0");
  await page.emulateMedia({ forcedColors: "active" });
  await expect(only).toHaveCSS("opacity", "1");
  await page.emulateMedia({ forcedColors: "none" });
  await only.click();
  ```

  hi3 只在 Chromium（不在 `threeBrowsers`），1280 宽（`max-md` 不生效）。先把指针移开：之前的点按可能让它停在这一行上。`toHaveCSS` 会等 150 ms 的过渡走完。
- **mc3**（提交 1，`e2e/stories/mcp/mc3-write-edit.spec.ts:200` 之后，`@narrow` 的那一格，375 宽）：`await expect(listed.first().getByRole("button", { name: /^Only this person/ })).toHaveCSS("opacity", "1");`（窄屏上一直显示，`revealClass` 的 `max-md:opacity-100`；mc3 的历史没有按人筛，每行都有它；这一格在 Chromium 与 `webkit-narrow` 上都跑，审查 C 的 P5C-M4，变异 63）。
- **TB1 新的一格**（提交 2，`e2e/stories/trash/tb1-trash.spec.ts` 末尾）`TB1 (preview): at 375 wide an item whose name has no space keeps its preview to the screen: its title broken, its Restore, its restore in the background and its breadcrumb`：`newOnboardedTeam`，建一本笔记本与一页长名字（同 UI1 的 `longName`）的页，经接口删掉（`deleteNode`，`e2e/fixtures/pages.ts:101`），`signedInPage`、375 宽，`goto` `trashPath(slug, notebookId, 页的 id)`（`trashPath` 取的就是节点的 id，tb1:269,361）：`h1` 的名字是那个长名字，`expectFits(page, h1, "Restore"按钮, 面包屑的 nav)`。再以 `page.route` 让 `POST **/api/v0/nodes/*/restore` 答 422 `page.restore_too_large`（`application/problem+json`，写法照 a11:130-136），按"Restore"：带名字的"Restore “<长名字>” in the background"出现，`expectFits(page, 它)`（审查 B 的 P5B-M12：今天它是一行、横向溢出）；控制台的那一条 422 照 tb1:400 的写法以 `pageWatch.expectConsole({ errors: [failedToLoad(422)] })` 记为预期的。只有页面版本：只核对预览的样子，不写（删除是造数，恢复被假的答复拒了，不发生）；接口与 MCP 版本豁免（5.5）。只在 Chromium（tb1 不在 `threeBrowsers`），不带 `@narrow`：`h1` 的 `wrap-anywhere` 与按钮的折行只在 Chromium 里核对（初稿说"WebKit 上的同类由 UI1 第 4 格覆盖"，不成立：UI1 的卡片与行都是截短，没有 `wrap-anywhere` 的元素，审查 C 的 P5C-M7），WebKit 与 Firefox 上的同类写法在第 1 节 (h) 的探针里量过。
- 不改的（核对过）：se1:340（命中的标题照旧是 500 的 `p`）、se1:309-316（Tab 的次序）、se1:401-421（`@narrow`）、`e2e/fixtures/wiki-search.ts:25-36`（结果是 `listitem`，状态行经 `h1` 的 `aria-describedby`）；hi3:321-334（行的文字，`main` 里一个 `status`）、hi3:343,417 与 hi4:165,185（`listed.….getByRole("link").first()` 仍是行的主链接）、hi4:162、lw1:238；lw9:492（`:scope > li`）、lw9:528-529（一个 `h1`，`main` 里一个"Ingest"）；`e2e/fixtures/wiki-pages.ts:391-401`（`revertWith` 以名字找"Undo change"）；mc3:174-233（历史与变更集页在 375 宽，筛选的组外面的 `xpath=..` 照旧是 `filterFieldClass` 的那一层）；`e2e/fixtures/trash.ts:46-66`、tb1、tb2:235（`main` 里一个 `alert`）；lw6:180,198（"Restore … in the background"以名字找）；lw13:172-185（区域里的两个 `section`）、lw13:232（目录第 50 个链接：行里的链接只多包一层 `p`）、lw13:300-317、lw10:346。

### 5.3 S3

- **hi2**（提交 1，`e2e/stories/history/hi2-version-restore.spec.ts:93-95`，点"Restore this version"之前）：

  ```ts
  // 320 wide, the version's title on one line, Restore in the head's row above it (decision 929; P4's handoff).
  await page.setViewportSize({ width: 320, height: 800 });
  const title = main.getByRole("heading", { level: 1, name: "Version 1", exact: true });
  const lines = await title.evaluate((element) => element.getBoundingClientRect().height / parseFloat(getComputedStyle(element).lineHeight));
  expect(lines).toBeLessThan(1.5);
  await expectFits(page, title, main.getByRole("button", { name: "Restore this version", exact: true }));
  await page.setViewportSize({ width: 1280, height: 800 });
  ```

  `h1` 的文字是 `version.title`（`W/pages/page/version-page.tsx:136`），"Version 1"；视图的 `article` 也叫这个名字（hi2:93），所以按 `heading` 找。`expectFits` 取 320（5.1）。只在 Chromium。
- **hi1:259、hi2:114**（提交 1，决定 943）：版本行的"当前"接在作者之后、中间一个空格，`toContainText(\`${admin} · EditCurrent\`)` 改成 `· Edit Current`，`· RestoreCurrent` 改成 `· Restore Current`（`toContainText` 读 `textContent`）；hi1:258、hi2:113 的注释同改（4.3.2，审查 C 的 P5C-R1-N1）。
- **as1**（提交 2，`e2e/stories/asset/as1-upload.spec.ts:183`，"Actions for notes.txt"按下之后、点"Download"之前）：`await page.mouse.move(0, 0); await expect(section.getByRole("button", { name: "Actions for notes.txt", includeHidden: true })).toHaveCSS("opacity", "1");`（菜单开着时焦点在菜单里、指针不在行上，只有 `aria-expanded` 让它显着；Radix 的菜单是模态的，开着时菜单之外的都 `aria-hidden`，附件的 `ul` 也是，按角色找要 `includeHidden`，不带时这一句在对的实现上也等到超时，审查 C 的 P5C-R1-I2；as1 只在 Chromium，1280 宽；审查 A 的 P5A-N8、C 的 P5C-M4，变异 56）。
- 不改的（核对过）：hi1:256-265（除 :259）、hi2:132（"没有改动"之后焦点在按钮上）、lw4:311,394、lw6:476；pg17:392-421（版本页在 375 宽，作者的链接）；`e2e/fixtures/wiki-health.ts:24-26`、lw8:625,673,678,683,707,834-836,983,1059,1081,1108、lw8:648-656；`e2e/fixtures/upgrade-pages.ts:31,48,56`、up1:231、up3:257,292、up5:231,245,307,313-317、up6:286,305、up7:242、up8:100,119；lw5:578-580、as1:118-135,205,214-217、as4:202,212、as6:134-150、pg2:193-238、lw3:310-318,343-346、lw7:181-182、lw6:379、lw8:995、se1:363。

### 5.4 375 px 与 `webkit-narrow`

`webkit-narrow` 的三格（MC3、PG2、SE1 的 `@narrow`）不变：P5 不加 `@narrow` 的格，mc3 那一格多一句。375 宽的新检查在 UI1（三种浏览器）、TB1（Chromium）、mc3 的一句（Chromium 与 `webkit-narrow`）与 hi2 的 320 宽一段（Chromium）。`check.mjs` 预计列出：chromium 434、firefox 105、webkit 105、webkit-narrow 3（UI1 两格在三种浏览器里、TB1 一格在 Chromium；实施者照 `node tools/e2e-projects/check.mjs` 的输出写准）。E2E 预计 647 格。 照实（S1，[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：S1 之后 chromium 433、firefox 105、webkit 105、webkit-narrow 3，E2E 646 格（UI1 两格 × 三种浏览器）；审查之后的修复都在已有的格里加步，数不变；vitest 231 个文件、5504 格。

### 5.5 13.4 第 3 条

UI1 的两格新的只有界面的样子，接口与 MCP 版本豁免（同 UI1 已有的写法）；TB1 新的一格只核对预览的样子，删除是造数，恢复被假的答复拒了，同样豁免。hi1、hi2、hi3、mc3、as1 加或改的是已有的格里的几句。文字见 8.2 第 4、9、15 条。

## 6. 实施步骤

三个 Step 各在自己的 worktree 里，分支从 `m13-ui-refresh` 拉出。每个提交单独为绿：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`；改了行为、夹具或故事的提交另经它跑完整的 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make e2e`，只败第 1 节那三格（同行同消息）。新的测试先写、在没改的树上看到它红，再改代码。提交说明的作用域照实写：动了 `e2e/` 的写 `e2e`，动了 `tools/e2e-projects/check.mjs` 的写 `tools`，只动 `web/` 的只写 `web`（P4 的决定 891）。文档提交写 `docs: … (M13/P5/S<m>)`。

每个 Step 合并进 `m13-ui-refresh` 之后，在它上面补一个文档提交，把合并的提交号填进占位：`docs: S<m>'s merge recorded in P5's design, 00-M13 and its review (M13/P5/S<m>)`（02-workflow 3.6 第 5 步）。S3 合并、记下合并之后，另写 `docs: M13/P5 done (M13/P5)`（02-workflow 3.7，8.1 第 4 条）。

### 6.1 S1

- [x] **开工**：不另跑基线（同 P4 的决定 883）；提交 1 的 `make e2e` 兼作核对，第 1 节之外的格红了先停下汇报。
- [x] **提交 1 `web: a settings page at its address with a trailing slash or in another case has its title and its groups apart, in the personal, workspace and notebook settings alike (M13/P5/S1)`**：先写 7.1 的 `settings-directory.test.tsx` 一格与三个布局、令牌页各一格，看到红（今天 `sectionAt` 对带斜杠或大小写不同的地址答 `undefined`：令牌页那一格红在整页报错，别的红在没有 `h1`），再改 4.1.7。
- [x] **提交 2 `web, e2e, tools: the workspace's home as cards of its notebooks, New notebook a dashed one after them; a notebook's home a title, its root pages with their icons and subpages, its settings link gone; an empty list said with an icon and what to do, in the settings too; a page's banner in its column while it loads or is gone (M13/P5/S1)`**：先写 7.1 的提交 2 各格（新的 `empty-state.test.tsx`、`notebook-home.test.tsx`，`workspace-home.test.tsx` 的卡片、虚线卡与提醒，`upgrade-banner.test.tsx` 的"路由 × 支"表的 S1 各行，`settings-page.test.tsx` 的 `cardClass` 一格，五个设置页的空列表各一格）、`expectFits` 的改动与 UI1 第 4 格，看到红：UI1 第 4 格三种浏览器都红在第 2 步（今天的卡片没有"· 1 member"；第 2 步的 `expectFits` 在今天的网格上也红，375 宽时页宽 578–579（初稿推的是 485，照实；变异 5 之下 563），没有 `grid-cols-1`，变异 5 再看一次）；表的 S1 各行除了页与笔记本设置的"读到"两行（旧树上就绿）都红在第 2 句（笔记本首页没有 `handle`，横幅由回退画在出口之上；页的读取中、读不到、不在三行是片段，横幅的父元素不是列）。`page-tree-scale.test.tsx:243` 改找齿轮的那一句在旧树上就绿（它是为删链接预先改的，不算"看到红"）。再做 4.1.1–4.1.4、4.5 的 `noColour`、`REQUIRED` 两行、`routes.tsx` 一处 `handle`、四条文字（两种语言）。
- [x] **提交 3 `web, e2e, tools: sign-in, sign-up, the invitation, onboarding and a new workspace in a card under the product's name; an error, a lost session and a page not found centred, the way home arrived at; the signed-in invitation's way back to the workspace, the workspace it joins kept from a read sent before (M13/P5/S1)`**：先写 7.1 的提交 3 各格（新的 `entry-card.test.tsx`，`button.test.tsx` 的 `buttonClass` 一格，`route-error.test.tsx` 的"回到首页"，`arrival.test.tsx` 的 404"回到首页"，`invitation.test.tsx` 的一个 `h1`、"回到工作区"三格与接受邀请时的竞态一格，`workspace.store.test.ts` 的再读一格，`guards.test.tsx` 的会话不可用照旧）与 UI1 第 5 格，看到红（"回到工作区"的"没有工作区时没有""引导没走完时没有"两格与竞态的一格都先等邀请页发出的那次读，在旧树上都红在这一步：今天邀请页不读工作区的列表；那两格的"没有"在旧树上本来就成立（今天邀请页上没有这个链接），由变异 68、95 证（C 的探针 `p5-rev-c/probe3/`）；竞态的一格在加了 `inviting` 的读、没改 `load` 的树上红在"Page not found"，由变异 93 证；审查 A 的 P5A-R2-M1、C 的 P5C-R2-M1 与 C 的探针 `p5-rev-c/probe2/`）；4.1.5、4.1.6、`REQUIRED` 两行。
- [x] 7.2 的 S1 各条变异；B 审查者截图之前，实施者先截 7.4 的 S1 几张看一遍；`make e2e` 之后照实记 UI1 两格每次的秒数（7.5）。
- [x] **提交 4 `docs: … (M13/P5/S1)`**：8.2 第 1–5 条；S1 的审查记录；本文 10.1 与照实改的各节；00 号文档第 12、13 节；`docs/v0.2/handoffs/M12-polish.md` 的"处理进展"一行（S1 不认领，写明核对过第 1 项里落在首页与入口上的：1、9.4 不认领，决定 937）；01-P1 第 9 节"P5"一条记下空状态已建、骨架不加（决定 914）；03-P3 第 9 节"P5"一条记下内容链接、空列表与 `sectionAt` 的去处；04-P4 第 9 节"P5"一条记下 `sectionAt` 已改、页的早退两支进了列、横幅与列的去处。

照实（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：提交 1–3 是 `262b7232`、`d65c9132`、`9ef5d2de`，说明与上面逐字相同。另有九个代码提交：`9975478d`（变异 t2、t3、t6、t12 在 `9ef5d2de` 上活下来之后补的格，只改测试，同 P4 `d29b68b0` 的先例）；审查之后的 `b46675d9`（`sectionAt` 用路由的 `matchPath`、逐段解码）、`45f02991`（工作区首页：读取中与读不到时没有"新建笔记本"的两格，卡片与虚线卡的高的类名，徽记紧跟名字，图标方块的边，空首页的胶囊）、`4340e64a`（入口卡片的 `break-words` 与 UI1 第 5 格的邀请一步，卡片里的 `text-sm`，`SessionUnavailable` 只有 `h2`）、`cdddecdd`（`PageColumn` 照读到之后的页排，UP1 的一步）、`de40d1e6`（读屏那一句 `hidden` 的断言，`listAnswered` 等 `load` 的结果，`expectFits` 的窗口守卫）；修复核对第 1 轮之后的 `31e05f59`（`PageColumn` 的 `panel`）、`e41fce83`（空头一行照页的头一行排，UP1 加 768 与 375）、`dc59e9e2`（`EmptyState` 的注释折行，去掉两个不承重的超时，三格的名字）。

### 6.2 S2

- [ ] **提交 1 `web, e2e: search, the history and a changeset as a list's page, a title, a bar of filters and rows in a card, one kind of row; Only this person shown as its row is hovered or focused; a changeset's head as a page's, Undo change in its row; their links to elsewhere arrived at (M13/P5/S2)`**：先写 7.1 的提交 1 各格（表的历史与变更集各行，`list-rows.test.tsx` 的搜索、历史、变更集三行，`history-page.test.tsx` 的"只看此人"与主链接的到达，`changeset-page.test.tsx` 的按钮次序与链接的到达，`revert-background.test.tsx:432-435` 加的一句）与 hi3、mc3 的几句、se1 的下限，看到红（hi3 红在静止的 `opacity` 0：今天它一直是 1；mc3 那一句在旧树上是绿的（今天一直是 1），它守的是 `max-md:` 不被去掉，由变异 63 证；se1 在改样子之前是绿的，改了之后才红在暗色的 `link`，量出新值再改下限：这一条不是"先红"，照实写进 10.2）；4.2.1–4.2.6、`page-layout.tsx` 原处导出的五串与 `PageColumn`、`routes.tsx` 两处 `handle`。
- [ ] **提交 2 `web, e2e: the trash and its preview, the directory and a tag's pages as the lists are, their empty lists said with an icon; a long name in the preview's title broken, its restore in the background too; their links arrived at (M13/P5/S2)`**：先写 7.1 的提交 2 各格（表的回收站、预览、目录、标签各行，`list-rows.test.tsx` 的回收站、目录两行，`trash-page.test.tsx` 的链接的到达，`trash-background.test.tsx:388-391,686-689` 加的两句，`trash-preview.test.tsx` 末尾的一格）与 TB1 新的一格，看到红（TB1 红在第一个 `expectFits`：今天的 `h1` 是 `flex`，名字是匿名的弹性项，最小宽度是整个名字）；4.2.7–4.2.10、`routes.tsx` 四处 `handle`。
- [ ] 7.2 的 S2 各条变异；实施者先截 7.4 的 S2 几张；照实记 TB1 一格的秒数。
- [ ] **提交 3 `docs: … (M13/P5/S2)`**：8.2 第 6–10 条；S2 的审查记录；本文 10.2 与照实改的各节（se1 量出的值）；00 号文档第 12、13 节；M12-polish 的"处理进展"一行（第 5 项的"回收站预览 `h1` 的长名"做了，TB1 核对，同一处的"在后台恢复"按钮一并；第 1 项里落在这几页上的不认领，逐项见决定 937）；01-P1 第 9 节"P2–P5"一条记下 P5 的六处标签组已改（`source-add-dialog.tsx:261` 留给 S3）。

### 6.3 S3

- [ ] **提交 1 `web, e2e: a version's page headed as a page is, Restore this version in its head's row, Current marked as the right column marks it, the right column's marks after the line; the health and upgrade pages in cards (M13/P5/S3)`**：先写 7.1 的提交 1 各格（表的版本页、健康、升级各行，`list-rows.test.tsx` 的健康一行，`page-versions.test.tsx:111-141` 的改写与 `Mark` 一格，`version-page.test.tsx` 的"当前"、那一格与作者链接的到达，`history-page.test.tsx` 的"Reverted"一句，`upgrade-page.test.tsx` 的链接的到达）与 hi2 的一段、hi1:259 与 hi2:114 的两句，看到红（hi2 红在 `lines`：今天 320 宽时标题被按钮挤成两行，P4 的审查 B 看到的；hi1、hi2 的两句红在"· Edit Current"）；4.3.1–4.3.4、`routes.tsx` 三处 `handle`。
- [ ] **提交 2 `web, e2e: the attachments and the uploads as cards of rows, their actions shown as a row is hovered or focused, a download marked; the dialogs' boxes, the pasted source's field and a label group; the upgrade's banner at the top of every notebook page's column, a dot of amber before it, the layout's other place for it gone (M13/P5/S3)`**：先写 7.1 的提交 2 各格（`attachments-section.test.tsx` 的下载图标与"操作"的类名，`source-add-dialog.test.tsx` 的标签组与文本框，`upgrade-banner.test.tsx` 的小圆点一句）与 as1 的一句，看到红（as1 那一句在旧树上是绿的：今天"操作"一直显示，它守的是 `aria-expanded:opacity-100` 不被去掉，由变异 56 证）；表在这个提交里不先红（4.3.6，由变异 57、58、61 证）；4.3.5–4.3.7。
- [ ] 7.2 的 S3 各条变异；实施者先截 7.4 的 S3 几张；全部 e2e 的这一次（00 号文档第 9 节，2.1 第 4 条）就是 S3 合并之前的门禁那一次，照实记时间（7.5）。
- [ ] **提交 3 `docs: … (M13/P5/S3)`**：8.2 第 11–16 条；S3 的审查记录；本文 10.3 与照实改的各节；00 号文档第 9、12、13 节（E2E 一步的时间）；M12-polish 的"处理进展"一行（16.7 看得见的一半做了；右栏版本行的"-"做了，决定 943：它是 P4 交来的，不是 M12-polish 的条目，记在 P4/S1 那一行之后）；01-P1 第 9 节"P2–P5"一条记下最后一处标签组已改、清单做完；04-P4 第 9 节"P5"一条记下右栏版本行的"-"已改。

## 7. 测试与验证

### 7.1 每项的测试

vitest 的格写明文件与要点；改写的写明行号与改法。改了文字的，两种语言都有格（13.2 第 4 条）。横幅的格都在 4.3.6 的表里，各 Step 只加自己的行，下面只写"表的某某各行"。

**S1 提交 1**

| 测试 | 要点 |
|---|---|
| `W/app/settings-directory.test.tsx`（加一格） | `sectionAt(workspaceSections, "/acme/settings", "/acme/settings/members/")` 是"成员"，`…/Members`、`/acme/SETTINGS/members` 也是；没有斜杠的照旧；`/acme/settings/` 与 `/acme/settings//` 不是任何一节（`index` 的重定向那一支）。 |
| 三个布局与一个用 `useSettingsHeading` 的页（加在现有的测试里：个人 `W/pages/settings/profile-page.test.tsx`、工作区 `W/pages/workspace/general-page.test.tsx`、笔记本 `W/pages/notebook/general-page.test.tsx`、令牌 `W/pages/settings/tokens-page.test.tsx`，各一格） | 打开 `…/general/`（个人是 `/settings/profile/`）：`h1` 是页名、取到达的焦点；令牌页打开 `/settings/tokens/` 与 `/settings/Tokens`：页画出来、不是"Something went wrong"，`h1` 是"Access tokens"（今天整页报错，第 1 节 (e)）。 |

**S1 提交 2**

| 测试 | 要点 |
|---|---|
| `W/components/empty-state.test.tsx`（新） | 两种大小：图标的方块 `aria-hidden`；说明是 `p`、文字原样；有 `action` 时它在说明之后，没有时不画那一层；整块没有 `role`（`queryByRole("status")` 为空）。不读 `iconTileClass`（它在这个提交里不导出）。 |
| `W/pages/workspace/workspace-home.test.tsx` | `:38-47` 改成 `["PlansPrivate · 1 member"]`、`["AtlasWorkspace can edit · 1 member"]`；新格：两位成员的写"2 members"、中文"私密 · 2 位成员"，1,200 位写"1,200 members"（`toLocaleString`）；LLM Wiki 的卡片含"LLM Wiki"；卡片里的图标 `aria-hidden`，名字的 `span` 带 `title`。`:83-88` 改写成"有笔记本时，能建的人在两组之后多一个'新建笔记本'，按下打开同一个对话框，Esc 之后焦点回到它"（`within(home).queryAllByRole("button")` 照角色是 `["New notebook"]`）；它不在任何一个 `list` 里（`closest("ul")` 为空，变异 10）。新格：访客、有笔记本时没有虚线卡（`queryAllByRole("button")` 为 `[]`，变异 9：两组不空，才看得出它看不看 `createsNotebooks`）。`:67-80` 照旧（空状态里只有那一个按钮）。新格：管理员的提醒里"Review them"按下之后到无主笔记本的设置页，它的 `h1` 拿着焦点。`:28-29` 照旧（`h1` 最近的 `section` 包着全部）。 |
| `W/pages/notebook/notebook-home.test.tsx`（新） | 顶层页面每行一个 `aria-hidden` 的图标；有两个子页面的一行的链接的可访问描述是"2 subpages"、中文"2 个子页面"，一个的"1 subpage"，没有的没有描述；看得见的数在链接之外、`aria-hidden`；链接的文字只是名字（变异 13）；没有"Notebook settings"链接；树里没有页面时（`pageServer({ nodes: [] })`）"No pages yet."在 `p` 里，`h1` 最近的 `section` 里只有一个"New page"（变异 17）。 |
| `W/pages/notebook/page-tree-scale.test.tsx:243` | 改找齿轮 `getByRole("link", { name: "Settings of …" })`（`notebook.settingsOf`，`W/pages/notebook/page-tree.tsx:293-304`，同是路由的 `Link`）的 `data-discover`；`:150-168`、`:214-215` 的反应次数照旧要过（行是 memo，数在父的渲染里算）。这一句在旧树上就绿。 |
| `W/app/settings-page.test.tsx`（加一格） | `SettingsCard` 的 `ul` 的 `className` 就是 `cardClass`（变异 46）。 |
| `W/pages/notebook/upgrade-banner.test.tsx` | 4.3.6 的表，S1 的八行（页四行、笔记本设置、笔记本首页三行）；删 `:141-177` 的三格。 |
| 五个设置页的测试（`W/pages/settings/tokens-page.test.tsx`、`W/pages/workspace/invitations-section.test.tsx`、`ownerless-page.test.tsx`、`W/pages/notebook/transfer-page.test.tsx`，审计在 `ownerless-page.test.tsx:531` 的那一格里） | 各加一句：空列表的说明在 `p` 里，前面有一个 `aria-hidden` 的图标方块。今天找文字的各格（`tokens-page.test.tsx:109,134,268`、`invitations-section.test.tsx:103`、`members-page.test.tsx:259,306,341,353`、`ownerless-page.test.tsx:183,531`、`transfer-page.test.tsx:69,765`、`W/pages/notebook/transfer-import.test.tsx:458,713,744,781`）照旧。 |
| `W/design-tokens.test.ts` | `noColour` 加三个词（4.5），守卫本身不改。 |
| UI1 第 4 格 | 5.1。 |

**S1 提交 3**

| 测试 | 要点 |
|---|---|
| `W/components/entry-card.test.tsx`（新） | 卡片里有"Nerve Wiki"（不是标题）、标志 `aria-hidden`；孩子在它之后；`appName` 与文档标题的后缀同一个（`useDocumentTitle` 的"· Nerve Wiki"）。 |
| `W/components/ui/button.test.tsx`（加一格） | `buttonClass({ variant: "outline", size: "sm", shape: "pill" })` 里有 `rounded-full`、`text-compact`，没有 `rounded-lg`、`text-sm`；`Button` 的类名与它相同（变异 66）。 |
| `W/pages/sign-in.test.tsx`、`sign-up.test.tsx`、`invitation.test.tsx`、`onboarding.test.tsx`、`create-workspace.test.tsx` | 照旧（以角色与名字找）；`invitation.test.tsx` 加五格：会话不可用时页上只有一个 `h1`（"Invitation"），"Cannot reach the server"是 `h2`、在 `alert` 里；已登录、账户有工作区、引导走完了（`userJSON`）时右上角有"Back to the workspace"，按下之后到 `/acme`（`invitationServer` 的账户只有 Acme，`/` 落到它）、它的 `h1` 拿着焦点（照 `W/app/layout.test.tsx:184-195`）；账户没有工作区时没有它（照 `:197-206`；`invitationServer` 加 `joined` 选项，今天它是函数里的局部变量 `[acme]`）；引导还有步骤时（`me: { ...userJSON, onboarding_steps: [] }`）也没有它（审查 A 的 P5A-R1-N1）。这两格是否定的，"没有"在旧树上本来就成立（今天邀请页上没有这个链接），守的是变异 68、95；断言"没有"之前先等列表答复（账户按钮与"Accept invitation"都出现、`sent` 里有一项以 `/api/v0/workspaces` 结尾的 GET：`invitationServer` 记的是方法加完整地址，例如 `GET http://nervewiki.test/api/v0/workspaces`，比全文的 `toContain("GET /api/v0/workspaces")` 在对的实现上也等不到，照同文件的写法用 `endsWith`；审查 A 的 P5A-R3-N1），不然在列表回来之前断言，变异 95 下也过（审查 A 的 P5A-R2-M1）；旧树上邀请页不读列表，这两格与竞态的一格一样红在这一步（C 的探针 `p5-rev-c/probe3/`）。竞态的一格：工作区列表的读换成照 `W/app/layout.test.tsx:9-21` 的 `held()` 扣住第一次（答的是发问那一刻的列表；只有第一次读交给 `held()`，之后的读照 `joined` 答，那是 `load` 的再读：整条路由都交给 `held()` 时再读也被扣住、永远不答，对的实现停在"加载中"，审查 A 的 P5A-R3-N1），按下"Accept invitation"之前先 `await` 它的 `asked`（那次读是邀请页的 `Corner` 发出的：旧树上邀请页不读列表，这一格红在这一步），按下之后地址到了 `/lab` 再放开那次读，`h1` 是"Lab"（照旧的 `load` 加上 `inviting` 的读，在这里画"Page not found"；`f62d0390` 上邀请页不读列表，不等 `asked` 时这一格是绿的；审查 A 的 P5A-R1-M1、P5A-R2-M1，C 的 P5C-R2-M1 与探针 `p5-rev-c/probe2/`）（决定 941，变异 68、69、93、95）。 |
| `W/stores/workspace.store.test.ts` | `:36-60`（读在写之前发出、写之后答复）照旧：第一次 `load` 已留下列表，`this.list ?? …` 取的是它，不再读；`:93-101,188-196`（写在第一次读之前）照旧：改动在读出去之前就答复了，计数不变，不再读（审查 A 的 P5A-R2-N1）；加一格（审查 A 的 P5A-R1-M1）：第一次读扣住，`accept` 答复之后放开它（答的是不含那个工作区的列表），`load` 再读一次，`list` 里有刚加入的工作区，`service.list` 被叫两次（变异 93）。 |
| `W/app/route-error.test.tsx` | `:24-26,47,70-73` 照旧（`alert` 包着 `h1`、错误码）；加一格："Go to the home page"是链接、去 `/`、路由状态是 `arrived`（变异 65），类名里没有 `rounded-lg`（变异 66）。 |
| `W/app/arrival.test.tsx` | `:89,103,123` 等 404 的几格照旧（`h1` 的 `ref` 与 `tabIndex`）；加一格：404 的"回到首页"按下之后，工作区首页的 `h1` 拿着焦点（`LandingPage` 转交）。 |
| `W/app/guards.test.tsx:63,80,154,236`、`W/app/layout-error.test.tsx:17`、`W/app/layout.test.tsx:58-62,100-115,184-206,239` | 照旧（`:113-114` 的"回到工作区只在设置与新建工作区页"那一句看的是 404，照旧成立；`:113` 的注释改成"…the settings', the creating page's and the signed-in invitation's"，审查 A 的 P5A-R1-N1）。 |
| UI1 第 5 格 | 5.1。 |

**S2 提交 1**

| 测试 | 要点 |
|---|---|
| `W/pages/search/search-page.test.tsx` | `:79-80,543`（两个 `status` 的次序、`main` 里第一个 `output`）、`:105,113`（行里第一个 `svg`）、`:125,191`（`p bdi`）、`:348`（名字是 `li` 的第一段文字）、`:653,663`（说明的文字）照旧；加一格：结果的列表名为"Search results"，它的 `li` 带 `group/row`。 |
| `W/pages/list-rows.test.tsx`（新，B 的 P5B-I2 的守卫） | `test.each` 逐页渲染一行，断言同一种行：行的第一行（主链接的父元素）的类名有 `text-sm` 与 `font-medium`，主链接的类名有 `hover:underline`、没有单独的 `underline`，行在一张卡片里（`row.closest(".rounded-card")` 不为空：别的页是卡片的列表本身，健康的行的列表在规则的卡片 `section` 里，列表自己没有圆角，4.3.3；审查 C 的 P5C-R1-M4、B 的 P5B-R1-N6）。这个提交是搜索、历史（用一条摄取的变更造数，另断言摄取一块的根没有 `text-sm`、来源的链接有 `hover:underline`、没有单独的 `underline`，审查 B 的 P5B-R1-M4）、变更集的改动三行，S2 提交 2 加回收站（另断言文字的一块有 `basis-56`）、目录两行，S3 提交 1 加健康一行；各页用它自己的测试文件里的造数（`pageServer`、`indexServer`、`healthServer`）。jsdom 不算样式，守的是类名还在；样子由 7.4"各列表的一行并排"的截图看（变异 76–78）。 |
| `W/pages/notebook/history-page.test.tsx` | `:98,280,311,783`、`:243,411,413,520,528,761,921,983` 照旧；加一格："只看此人"的类名含 `opacity-0`、`group-focus-within/row:opacity-100`、`max-md:opacity-100` 与 `[@media(hover:none)]:opacity-100`，它的 `li` 带 `group/row`（jsdom 不算样式，只守住类名还在；行为由 hi3、mc3 守，`hover: none` 没有故事守：e2e 的三个项目都不是触屏，审查 C 的 P5C-R1-M2，变异 92）；加一格（决定 940）：`test.each` 逐个按下一行的主链接、"Undone"（链接名"Undone: …"，代码里的"Reverted"）与"Undid “…”"（代码里的"Reverts …"），`router.state.location.state` 带 `arrived`，主链接那一次另看变更集页的 `h1` 拿着焦点（变异 70）。 |
| `W/pages/notebook/history-ingest.test.tsx:43,44,60,160-166,176,180-205` | 照旧。 |
| `W/pages/notebook/changeset-page.test.tsx` | `:106,257,495`（`h1` 的下一个兄弟是事实）、`:125,219,446`（行的文字）、`:501,515`（`span`）照旧；加一格："Undo change"在面包屑之后、`h1` 之前（`compareDocumentPosition`），按下与关上之后焦点照旧回到它；加一格（决定 940）：`test.each` 逐个按下"See the undo"、"Undid “…”"、"Undone by:"之后的一个"Undo change, ID …"、改动行的"In the trash""History of this node"与内容的版本（界面的英文，审查 C 的 P5C-R1-N1），`router.state.location.state` 带 `arrived`（变异 72、73）。 |
| `W/pages/notebook/revert-background.test.tsx:432-435` | 那一格之后加：按下"See the jobs"，导入导出页的 `h1` 拿着焦点（决定 924，变异 74）。 |
| `W/pages/notebook/revert-dialog.test.tsx:165,247,529-535,665,1351` | 照旧。 |
| `W/pages/notebook/upgrade-banner.test.tsx` | 表的历史与变更集各行（7 行）。 |
| `W/pages/page/page-layout.test.tsx` | 照旧（`page-layout.tsx` 只是把串写成了常量）。 |
| hi3、mc3、se1 | 5.2。 |

**S2 提交 2**

| 测试 | 要点 |
|---|---|
| `W/pages/notebook/trash-page.test.tsx` | `:59-86,120,147,160,291,446-447` 照旧（`:120,147` 的 `p:last-child` 在 `TrashFacts` 包的那一层里仍是"恢复到…"那一行）；"The trash is empty."在 `p` 里，前面一个 `aria-hidden` 的图标方块（加一句）；加一格（决定 940）：按下一行的链接，预览的 `h1` 拿着焦点（变异 71）。 |
| `W/pages/notebook/trash-background.test.tsx:388-391,686-689` | 两格之后各加：按下"See the jobs"，导入导出页的 `h1` 拿着焦点（决定 924：回收站页的与预览的，变异 75、43b）。 |
| `W/pages/notebook/trash-preview.test.tsx` | `:72-75,372,395,523,603-614,632,928` 照旧（"恢复"照旧在 `shown` 里，面包屑在提示之前）；文件末尾加一格（不挪 `:587` 那一格偶发的行号）：长名字在 `h1` 的 `span.wrap-anywhere` 里，`h1` 的文字照旧是名字。 |
| `W/pages/notebook/index-page.test.tsx`、`index-page-lifecycle.test.tsx` | `:21-28,46,424-430,439-440` 照旧。 |
| `W/pages/notebook/tag-page.test.tsx` | `:16,35-45,64-65,73-74,91-92` 照旧。 |
| `W/pages/list-rows.test.tsx` | 加回收站、目录两行。 |
| `W/pages/notebook/upgrade-banner.test.tsx` | 表的回收站、预览、目录、标签各行（13 行）。 |
| TB1 新的一格 | 5.2。 |

**S3 提交 1**

| 测试 | 要点 |
|---|---|
| `W/pages/page/version-page.test.tsx` | `:69`（"· Ada · Edit"）、`:231`、`:377` 照旧；加一格："Restore this version"在面包屑之后、`h1` 之前；最新的版本上"Current"与右栏的"当前"同一种 `Mark`（类名含 `text-accent`）；加一格（决定 929）：能编辑的人打开版本、版本列表的读扣住（`deferred`）时，头一行右边那一格已在、类名含 `min-h-7.5`，里面没有按钮，放开之后按钮出现在同一格里；读者没有那一格（变异 83）；加一格（决定 940）：作者的链接按下之后，变更集页的 `h1` 拿着焦点（变异 88）。 |
| `W/pages/page/version-restore.test.tsx`、`page-versions.test.tsx` | `version-restore.test.tsx:157` 改成 `/Ada · Restore Current$/`；`page-versions.test.tsx:21-29` 的 `listed()` 与 `:62,195` 照旧；`:111-141` 改写成"`li` 一个 `DIV`，它的子元素是 `A`、`P` 与小块的 `SPAN`（当前的带 `bg-accent-soft`）"（决定 943，变异 84）；`:104` 的期望改成 `"5分钟前Ada · 撤销 当前 编辑中"`（审查 A 的 P5A-R1-M2）；`:403-409` 照旧；加一格：普通的 `Mark` 的类名是 `markClass`（含 `rounded-full` 与 `whitespace-nowrap`），`current` 的含 `whitespace-nowrap`（变异 94）。 |
| `W/pages/notebook/history-page.test.tsx` | 读"Undone"（代码里的"Reverted"）的那一格加一句：它的类名含 `rounded-full` 与 `hover:border-border-strong`、没有 `underline`（`markClass`，变异 85）。 |
| `W/pages/notebook/upgrade-banner.test.tsx` | 表的版本页、健康、升级各行（8 行）；删 `:209-220`。 |
| `W/pages/notebook/health-page.test.tsx`、`health-rule.test.tsx`、`health-page-lifecycle.test.tsx` | `health-page.test.tsx:160,166,252-256,313-330`、`health-rule.test.tsx:29,41,76,90,117,171,274,288,313,347,648-650`、`health-page-lifecycle.test.tsx:35,391,437,439` 照旧。 |
| `W/pages/notebook/upgrade-page.test.tsx`、`upgrade-check.test.tsx` | 第 1 节 (f) 列的各处（`upgrade-page.test.tsx:92,112,142,509,853,864,1067-1071,1084,1088,1112,1365`，`upgrade-check.test.tsx:38,228-230,296,302,325,375-377,407,441,448,459,470,763-766,919,943,1208`）照旧；`upgrade-page.test.tsx` 加一格（决定 940）：通往种子页的链接按下之后，页的 `h1` 拿着焦点（变异 87）。 |
| `W/pages/list-rows.test.tsx` | 加健康一行。 |
| hi2、hi1:259、hi2:114 | 5.3。 |

**S3 提交 2**

| 测试 | 要点 |
|---|---|
| `W/pages/page/attachments-section.test.tsx` | `W/test/attachments.ts:21-26` 的 `rows()` 与它的二十七处用处（`attachments-section.test.tsx` 17、`attachment-uploads.test.tsx` 8、`W/pages/notebook/source-uploads.test.tsx` 2）、`:39,173,336,357,366` 照旧；加一格：不内联的附件一行在链接之后有 `aria-hidden` 的下载图标、内联的没有；"操作"的类名含 `opacity-0` 与 `aria-expanded:opacity-100`，它的 `li` 带 `group/row`。 |
| `W/pages/page/attachment-uploads.test.tsx`、`W/pages/notebook/source-uploads.test.tsx`、`source-attachments.test.tsx:86` | 照旧（`UploadRows` 以名字"Uploads"找）。 |
| `W/pages/notebook/upgrade-banner.test.tsx` | 表照旧要过（不先红，4.3.6）；`:179-207` 照旧；`:35-36` 照旧（`bannerOf`）；加一句：横幅里状态那一行前有一个 `aria-hidden` 的元素、没有文字。 |
| `W/pages/notebook/transfer-import.test.tsx`（导入的对话框在这里测）、`W/pages/notebook/source-add-dialog.test.tsx` | 照旧（以角色找 `alert`）；`source-add-dialog.test.tsx` 加一格：Markdown 那一个标签的父元素带 `space-y-1.5`（变异 60），文本框的类名含 `focus-visible:outline-hidden` 与 `rounded-control`（`fieldClass`，变异 86），说明带 `text-compact`。 |
| as1 | 5.3。 |

**S1 的照实**（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：

- 上面 S1 的各格都在。写法与上面不同的：`invitation.test.tsx` 的 `invitationServer` 另有 `joined`（接受之后列表里有这个工作区）与 `firstList`（只扣住第一次读列表，之后照 `joined` 答）；否定的两格以 `listAnswered()` 等列表读到：渲染之前 `vi.spyOn(WorkspaceStore.prototype, "load")`，它返回的函数先等账户按钮与"Accept"出现，再等 `load` 被调用，然后在 `act` 里等 `load` 的结果全部落定（`Promise.allSettled`）。实施时初写的是列表的 GET 之后多等 50 ms，不承重（变异 t11），换成这个信号（审查 A 的 P5S1A-N1、C 的 P5S1C-N5）；它承重：审查 C 的一对变异里，第一次读列表晚 400 ms 答、同时做 95，这一格红（rc-slow-95）；再去掉这个等待，假绿（rc-slow-95-nosignal）。列表马上答时不靠它也红（an1-no-signal-95）。vitest 配了 `restoreMocks: true`，spy 每格之后还原。
- `9975478d` 补的格：`notebook-home.test.tsx` 1,200 个子页面一格（数与描述照语言区写，t2），`entry-card.test.tsx` 标志带 `forced-colors:hidden`（t3），`workspace-home.test.tsx` 虚线卡的焦点环三个类（t12），UI1 第 4 格第 5 步名字截在数之前（t6）。
- 审查之后多的：`settings-directory.test.tsx` 的 `members//`、`%6Dembers` 与答 `undefined` 的 `members%2F`、`%E0%A4%A`，令牌页的 `test.each` 加 `/settings/tokens//`、`/settings/%74okens`（`b46675d9`）；`workspace-home.test.tsx` 两格（笔记本的读不答时 `main` 里没有按钮，答 500 时只有"Try again"），卡片一格断言 `h-full`、`sm:min-h-28`、图标方块的边、名字不带 `flex-1` 且下一个兄弟是"LLM Wiki"，虚线卡一格断言 `sm:min-h-28`，空首页新建一格断言 `buttonClass({ size: "sm", shape: "pill" })`（`45f02991`）；`entry-card.test.tsx` 的 `break-words` 一格，邀请、注册、新建工作区、引导两步的 `text-sm` 与引导页的 `space-y-1.5`（`4340e64a`）；`notebook-home.test.tsx` 第一格断言描述的句子 `hidden`（`de40d1e6`）；`page-layout.test.tsx` 三格：不带 `panel` 的列不留右栏的地方，页的两支开着右栏时留、收起时不留（`31e05f59`），读取中的空头一行与读到之后的头一行同类名（`e41fce83`）。

### 7.2 变异

每条在对应的树上做、跑写明的测试、看到红、还原。"故事"一列要重建前端再跑那一个故事（Chromium，除非写明）。1–60 与 43b 是初稿的（修订时照审查改了其中 5、6、9、10、13、15、17、18、19、39、45、46、49、52、56、57、58、60 的抓手，写明依据），61–91 是修订加的；修订核对第 2 轮之后再修订加了 96；修订核对第 1 轮之后再修订加了 92–95，改了 62、63 的抓手的理由，69 的去处（审查的编号如 P5C-R1-M2）。

| # | Step | 变异 | 抓住它的 |
|---|---|---|---|
| 1 | S1 | `sectionAt` 不去尾随的斜杠 | `settings-directory.test.tsx` 新格；三个布局与令牌页的新格 |
| 2 | S1 | `EmptyState` 的图标方块去掉 `aria-hidden` | `empty-state.test.tsx` |
| 3 | S1 | `EmptyState` 的说明换成 `div` | `empty-state.test.tsx`；S2 之后另有 `history-page.test.tsx:413`（`selector: "p"`） |
| 4 | S1 | `EmptyState` 的外层加 `role="status"` | `empty-state.test.tsx`；`history-page.test.tsx:411`（S2 之后） |
| 5 | S1 | 首页的网格去掉 `grid-cols-1` | UI1 第 4 格第 2 步（故事，三种浏览器；375 宽的 `expectFits` 就红，页宽 562–567，审查 B、C 量过；不需要 320 那一步） |
| 6 | S1 | 卡片的名字去掉 `truncate` 换成 `break-words` | UI1 第 4 格第 2 步（375 宽时名字的 `span` 多于一行、不截短；初稿比卡片的高，抓不到：两张都是 112 px，审查 C 的 P5C-I2） |
| 7 | S1 | 成员数总用 `notebooks.members`（一位也写"1 members"） | `workspace-home.test.tsx:38-47` |
| 8 | S1 | 中文的 `notebooks.membersOne` 写成英文 | `workspace-home.test.tsx` 中文一格 |
| 9 | S1 | 虚线卡不看 `createsNotebooks` | `workspace-home.test.tsx` 访客、有笔记本的新格（初稿写 n1:116，那里访客没有笔记本、两组都空，虚线卡本来就不画，审查 C 的 P5C-M2） |
| 10 | S1 | 虚线卡放进最后一组的列表里（多一个 `li`） | `workspace-home.test.tsx` 新格（它不在任何 `list` 里；初稿写 `:36-37,48,60-61`，那几格数的是链接，C 的 P5C-M2）。照实：决定 942 之后虚线卡不在任何一组的列表里，"多一个 `li` 在最后一组"做不了原样，用代理（虚线卡包进 `ul > li`），"in no list"那一格红 |
| 11 | S1 | 两组都空时也画虚线卡 | `workspace-home.test.tsx:67-80`（按钮只有一个） |
| 12 | S1 | 提醒的"Review them"不带 `arrived` | `workspace-home.test.tsx` 新格（焦点在 `body`） |
| 13 | S1 | 子页面数挪进链接里 | `notebook-home.test.tsx`（链接的文字只是名字；初稿写 `page-tree-scale.test.tsx:147`，那是平的树、没有子页面，数不画，C 的 P5C-M2） |
| 14 | S1 | 子页面数不经 `aria-describedby`（句子 `sr-only` 地放在链接之外）、看得见的数不 `aria-hidden` | `notebook-home.test.tsx`（链接的描述是"2 subpages"、看得见的数 `aria-hidden`） |
| 15 | S1 | 数经 `pages.childrenOf` 每行读一次 `tree`，不在渲染里读一次 | 等价，不跑：`tree` 是 `makeAutoObservable` 的 getter，即 computed（`W/stores/page-tree.store.ts:161-183,240-243`），首页这个 observer 渲染时读它、一直观察着它，同一次渲染里非 observer 的 `SubpageList` 每行再读都取缓存（审查 A 的探针：一千个顶层页两种写法都只调 `indexTree` 一次；C 的探针：被观察时在 reaction 之外读一千次只建一次索引）；页上没有 observer 读 `pages.tree` 时才不等价，首页总读它（05-lessons 第 1 节；初稿的"量时间"去掉，A 的 P5A-N1、C 的 P5C-M3） |
| 16 | S1 | "笔记本设置"的链接留着 | `notebook-home.test.tsx` |
| 17 | S1 | 笔记本首页的空状态带"新建页面" | `notebook-home.test.tsx` 的空树一句（初稿另写 `upgrade-read-only.test.tsx:137-138`，它的 `pageServer()` 有 Guide、Notes，不是空状态，C 的 P5C-M2） |
| 18 | S1 | 笔记本首页不画 `<ColumnTop />` | 表的笔记本首页三行（第 1 句：`handle` 在而横幅不见） |
| 19 | S1 | 笔记本 `index` 路由不加 `handle` | 表的笔记本首页三行（第 2 句：横幅照旧画在出口之上，父元素不是列） |
| 20 | S1 | 设置的空列表的说明换成 `span` | 五个设置页的新句 |
| 21 | S1 | `noColour` 不加 `border-dashed`（守卫不改） | `design-tokens.test.ts`（虚线卡的类名红） |
| 22 | S1 | 卡片用 `text-gray-500` 代替 `text-faint` | `design-tokens.test.ts`（调色板） |
| 23 | S1 | 入口卡片的标志去掉 `aria-hidden` | `entry-card.test.tsx` |
| 24 | S1 | 产品名写成 `h2` | `entry-card.test.tsx`（卡片里没有 `h1` 之外的标题） |
| 25 | S1 | 邀请页的会话不可用照旧 `h1` | `invitation.test.tsx` 新格 |
| 26 | S1 | 出错页的 `role="alert"` 挪到说明的 `p` 上 | `route-error.test.tsx:24-26`；`guards.test.tsx:63`（会话不可用同样挪时） |
| 27 | S1 | 404 的"回到首页"不带 `arrived` | `arrival.test.tsx` 新格；UI1 第 5 格（故事） |
| 28 | S1 | 404 的 `h1` 去掉 `tabIndex` | `arrival.test.tsx:89` |
| 29 | S1 | `REQUIRED` 的 UI1 新前缀写错一个字 | `node tools/e2e-projects/check.mjs`（列表里没有那一格） |
| 30 | S2 | `revealClass` 去掉 `group-focus-within/row:opacity-100` | hi3（故事：焦点在链接上时仍是 0） |
| 31 | S2 | `revealClass` 去掉 `forced-colors:opacity-100` | hi3（故事：强制色下 0） |
| 32 | S2 | "只看此人"的 `li` 去掉 `group/row` | `history-page.test.tsx` 新格；hi3（故事） |
| 33 | S2 | 把暗色的 `--accent` 调淡（`#d9a84e` → `#b88a3a`） | se1（故事：暗色的 `link`、`select` 红；00 号文档 4.8 的最低清单） |
| 34 | S2 | se1 的暗色 `link` 下限留在 7.82 | se1（故事：改了样子的树上红；这是"下限跟着改"的反证） |
| 35 | S2 | 命中的标题换成 `div` | se1:340（故事） |
| 36 | S2 | 筛选条里在"在哪里找"之前多一个可聚焦的元素（例如把状态行做成链接） | se1:309-316（故事） |
| 37 | S2 | 变更集页在 `h1` 与事实之间插进"撤销"的那一行 | `changeset-page.test.tsx:106` |
| 38 | S2 | "撤销这次改动"留在事实之后 | `changeset-page.test.tsx` 新格 |
| 39 | S2 | 历史与变更集页的路由不加 `handle` | 表的历史、变更集各行（第 2 句） |
| 40 | S2 | 回收站的行里"恢复"排在链接之前 | `trash-page.test.tsx:446-447` |
| 41 | S2 | 预览的"恢复"挪出 `shown` | `trash-preview.test.tsx:372`（预览被换掉时焦点交接） |
| 42 | S2 | 预览的名字不进 `wrap-anywhere` 的 `span`（照旧是 `flex` 的 `h1` 里的文字） | TB1 新的一格（故事） |
| 43 | S2 | 面包屑挪到提示之后 | `trash-preview.test.tsx:928` |
| 43b | S2 | 预览的"See the jobs"不带 `arrived` | `trash-background.test.tsx:686-689` 加的一句（焦点在 `body`） |
| 44 | S2 | 目录的分组不再是 `section`（换成 `div`） | lw13:176-177（故事） |
| 45 | S2 | 回收站、预览、目录、标签任一路由不加 `handle` | 表的对应各行 |
| 46 | S2 | `SettingsCard` 不用 `cardClass`（写回局部的串，再把 `cardClass` 的圆角改成 `rounded-md`） | `settings-page.test.tsx` 新格（`className` 不再是 `cardClass`；初稿说"没有格读圆角，由截图看"，C 的 P5C-M4 给了这一格） |
| 47 | S3 | "还原这个版本"留在标题旁 | hi2（故事：320 宽时 `lines` ≥ 1.5）；`version-page.test.tsx` 新格 |
| 48 | S3 | 版本页的"当前"用普通的 `Mark` | `version-page.test.tsx` 新格 |
| 49 | S3 | 版本页的路由不加 `handle` | 表的版本页各行 |
| 50 | S3 | 健康的规则卡片换成 `div` | `health-page.test.tsx:166`；lw8:625（故事） |
| 51 | S3 | 健康页加一个带 `id` 的 `section`（例如把标题行做成 `section id="head"`） | lw8:625,678（故事） |
| 52 | S3 | 升级页的根换成 `section` | `e2e/fixtures/upgrade-pages.ts:31` 的 strict mode（up5 等，故事；初稿另写 `upgrade-page.test.tsx:853`，它的 `closest("section")` 取最近的一节，根换成 `section` 不影响它，C 的 P5C-M2） |
| 53 | S3 | 附件的列表加 `aria-label` | `W/test/attachments.ts` 的 `rows()`（`attachments-section.test.tsx:30` 等） |
| 54 | S3 | 下载图标放进链接里且带文字（`sr-only`"下载"） | `attachments-section.test.tsx:30`（`rows()` 的文字） |
| 55 | S3 | 下载图标给内联的附件也画 | `attachments-section.test.tsx` 新格 |
| 56 | S3 | "操作"去掉 `aria-expanded:opacity-100` | as1 加的一句（故事：菜单开着、指针移开时 `opacity` 0）；`attachments-section.test.tsx` 新格（类名）。初稿说"jsdom 与 e2e 都不量它"，不成立：菜单开着时焦点在 portal 里、指针移开之后不悬停，只有 `aria-expanded` 让它显着，Chromium 的 e2e 量得到（A 的 P5A-N8、C 的 P5C-M4） |
| 57 | S3 | `NotebookLayout` 删了回退，某个笔记本页（例如目录）不画 `<ColumnTop />` | 表的目录一行（第 1 句：没有横幅） |
| 58 | S3 | 删了回退，读取失败的一支（变更集页 `:95` 的 `NotLoaded`）不包 `PageColumn` | 表的变更集页"读取中""变更集读不到"两行 |
| 59 | S3 | 横幅的 `bg-surface-2` 换成 `bg-amber-50` | `design-tokens.test.ts` |
| 60 | S3 | `source-add-dialog.tsx:261` 照旧 `space-y-2` | `source-add-dialog.test.tsx` 新格（标签的父元素带 `space-y-1.5`；初稿说"没有格，由截图看"，C 的 P5C-M4） |
| 61 | S3 | 版本页的 `:79-82`（页不在、版本号不合法）不包 `PageColumn`（直接 `return <PageGone …/>`） | 表的版本页"页不在""版本号不合法"两行（S3 提交 1 起；提交 2 删了回退之后同样，审查 A 的 P5A-I1） |
| 62 | S1 | 笔记本首页的 `h1` 不带 `titleTextClass`（只有 `pageTitleClass`） | UI1 第 4 格第 4 步，只在 Firefox 那一次红（照实红在 `:213` 的行数：Firefox 里 `h1` 一行也不折，先被 `lines` 拦下，S1 的审查 C 的 P5S1C-N4；初稿写的是：Firefox 不在连字符与数字之间断，`h1` 以整个名字作最小宽度，375 宽时页宽 863、`expectFits` 红；Chromium 与 WebKit 在连字符之后断，`h1` 不带 `min-w-0` 也缩到一行宽、折成三行，"新建页面"照样与它右边齐，两次都过；审查 C 的 P5C-R1-M3 的探针 `p5-rev-c/probe1/m62.mjs`） |
| 63 | S2 | `revealClass` 去掉 `max-md:opacity-100` | mc3 加的一句（故事：Chromium 与 `webkit-narrow` 两次都在 375 宽静止时 `opacity` 0：`webkit-narrow` 用的是 `Desktop Safari`，不是触屏，三个项目都报 `hover: hover`，审查 C 的 P5C-R1-M2 的探针 `p5-rev-c/probe1/hover.mjs`；C 的 P5C-M4） |
| 64 | S1 | `sectionAt` 不转小写 | `settings-directory.test.tsx` 新格的大写地址；令牌页的新格（`/settings/Tokens` 整页报错） |
| 65 | S1 | 出错页的"回到首页"不带 `arrived` | `route-error.test.tsx` 新格（C 的 P5C-M4） |
| 66 | S1 | `buttonClass` 不经 `cn`（直接返回 `buttonVariants(…)`） | `button.test.tsx` 新格；`route-error.test.tsx` 新格的同一句（类名里有 `rounded-lg`，B 的 P5B-I1） |
| 67 | S1 | 页的早退一支（`page-layout.tsx:79-82`，树读取中）照旧是片段 | 表的页"树读取中""树读不到"两行（第 2 句） |
| 68 | S1 | 邀请页的"回到工作区"不看工作区的列表（没有工作区时也放） | `invitation.test.tsx` 账户没有工作区的那一格（列表空着也放了链接，"没有"红） |
| 69 | S1 | 邀请页的"回到工作区"不带 `arrived` | `invitation.test.tsx` 有"回到工作区"的那一格（`/acme` 的 `h1` 不拿焦点；初稿写 `/lab`，`invitationServer` 的账户只有 Acme，审查 A 的 P5A-R1-N1） |
| 70 | S2 | 历史行的主链接不带 `arrived` | `history-page.test.tsx` 新格（路由状态、变更集页的 `h1` 不拿焦点） |
| 71 | S2 | 回收站行的链接不带 `arrived` | `trash-page.test.tsx` 新格 |
| 72 | S2 | 变更集页的"Reverts …"（`:387`）不带 `arrived` | `changeset-page.test.tsx` 新格的那一次 |
| 73 | S2 | 改动行的"In the trash"不带 `arrived` | `changeset-page.test.tsx` 新格的那一次 |
| 74 | S2 | 变更集页的"See the jobs"（`:344`）不带 `arrived` | `revert-background.test.tsx:432-435` 加的一句 |
| 75 | S2 | 回收站页的"See the jobs"（`:274`）不带 `arrived` | `trash-background.test.tsx:388-391` 加的一句 |
| 76 | S2 | 回收站行文字的一块不带 `listTextClass` | `list-rows.test.tsx` 回收站一行（`basis-56`）；7.4 的截图（长的事实下"恢复"折到左下） |
| 77 | S2 | 回收站行的链接照旧 `underline underline-offset-4` | `list-rows.test.tsx` 回收站一行 |
| 78 | S2 | 历史行的第一行不带 `lineTitleClass`（照旧没写字号） | `list-rows.test.tsx` 历史一行 |
| 79 | S2 | 带名字的"在后台恢复"不带 `shrink` | TB1 新的一格的后一段（故事：`expectFits` 的左边小于 0） |
| 80 | S2 | 带名字的"在后台恢复"不带 `whitespace-normal`（照旧不折行） | TB1 新的一格的后一段（故事：页宽或右边出屏） |
| 81 | S2 | 预览"不在回收站里"一支另画 `ColumnTop`（初稿的字面） | 表的预览"不在回收站里"一行（第 1 句：两个横幅，审查 A 的 P5A-M3） |
| 82 | S2 | 变更集页 `:69`（地址不是 id）不包 `PageColumn` | 表的变更集页"地址不是 id"一行（路由有 `handle`，回退不画；那一支没有 `ColumnTop` 就没有横幅，第 1 句） |
| 83 | S3 | 版本页右边那一格只在按钮显示时画 | `version-page.test.tsx` 新格（版本列表扣住时那一格不在） |
| 84 | S3 | 右栏版本行的小块回到行尾自成一列 | `page-versions.test.tsx:111-141` 改写的那一格；hi1:259（故事："· EditCurrent"） |
| 85 | S3 | 历史行的"Reverted"照旧方角（不用 `markClass`） | `history-page.test.tsx` 加的一句 |
| 86 | S3 | 粘贴来源的文本框照旧手写（没有 `fieldClass`） | `source-add-dialog.test.tsx` 新格（类名里没有 `focus-visible:outline-hidden`）；强制色下的聚焦由 7.4 的截图看 |
| 87 | S3 | 升级页通往页面的链接不带 `arrived` | `upgrade-page.test.tsx` 新格 |
| 88 | S3 | 版本页作者的链接不带 `arrived` | `version-page.test.tsx` 新格 |
| 89 | S3 | 横幅的小圆点写成 `bg-accent` | `design-tokens.test.ts`（"amber is no fill"） |
| 90 | S2 | 预览的按钮一组不带 `ms-auto` | 7.4 的截图（长名字时按钮在标题之下靠左）；没有格量它，照实记"由截图看" |
| 91 | S2 | 历史的"谁"不带 `max-w-64` | 7.4 的截图（长名字的"谁"让筛选条参差）；没有格，照实记 |
| 92 | S2 | `revealClass` 去掉 `[@media(hover:none)]:opacity-100` | `history-page.test.tsx` 新格的类名（e2e 的三个项目都报 `hover: hover`，没有故事量它；审查 C 的 P5C-R1-M2） |
| 93 | S1 | `WorkspaceStore.load` 不再读（照旧 `this.changesAnswered !== answeredBefore && this.list !== undefined`） | `workspace.store.test.ts` 新格（`list` 里没有刚加入的）；`invitation.test.tsx` 竞态的一格（`h1` 是"Page not found"；审查 A 的 P5A-R1-M1） |
| 94 | S3 | `markClass` 去掉 `whitespace-nowrap` | `page-versions.test.tsx` 新格的类名（断开的小块由第 1 节 (g) 的第四次扫描看到，jsdom 不排版；审查 B 的 P5B-R1-M2） |
| 95 | S1 | `Corner` 的 `inviting` 不看引导 | `invitation.test.tsx` 引导还有步骤的那一格（审查 A 的 P5A-R1-N1） |
| 96 | S1 | 标题行的 `titleTextClass` 去掉 `max-sm:basis-auto` | UI1 第 4 格第 5 步（故事，三种浏览器：375 宽时 `basis-56` 的 224 px 加按钮宽过列，"New page"掉到"Plans"之下，第 1 节 (l)，审查 B 的 P5B-R1-M1、C 的 P5C-R2-N2） |
| t2 | S1 | 子页面数不照语言区写 | `notebook-home.test.tsx` 1,200 个子页面一格（`9975478d`） |
| t3 | S1 | 入口卡片的标志去掉 `forced-colors:hidden` | `entry-card.test.tsx`（`9975478d`） |
| t6 | S1 | 子页面行的行尾不给数留位置 | UI1 第 4 格第 5 步 `:234`（三种浏览器，名字右边 332 对数的左边 323.9；`9975478d`） |
| t12 | S1 | 虚线卡去掉焦点环的类 | `workspace-home.test.tsx` 虚线卡一格（`9975478d`） |
| t7 | S1 | 卡片去掉 `sm:min-h-28`；同类的：虚线卡去掉它（t7-dashed），卡片去掉 `h-full`（t7-h-full） | `workspace-home.test.tsx` 卡片与虚线卡两格的类名（`45f02991` 起；之前没有格，审查 B 的 P5S1B-M2、C 的 P5S1C-N4） |
| cx1 | S1 | `sectionAt` 只容一个尾随斜杠 | `settings-directory.test.tsx:456`（`members//`）；令牌页 `/settings/tokens//`（`b46675d9` 起，审查 C 的 P5S1C-M1） |
| 不解码 | S1 | `sectionAt` 比较之前不解码 | `settings-directory.test.tsx:457`（`%6Dembers`）；令牌页 `/settings/%74okens`（审查 A 的 P5S1A-N2） |
| cx16 | S1 | 子页面行读屏的那一句去掉 `hidden` | vitest：`notebook-home.test.tsx:48`；故事（另一行跑，vitest 红时 `&&` 之后不跑）：UI1 第 4 格 `:238` 的 `toBeHidden`，三种浏览器（`de40d1e6` 起，审查 C 的 P5S1C-M2） |
| cx21 | S1 | 工作区首页读取中也画虚线卡 | `workspace-home.test.tsx:130`、`:142` 两格（`45f02991` 起，审查 A 的 P5S1A-M1、C 的 P5S1C-N1） |
| 947 | S1 | 卡片名字加回 `flex-1` | `workspace-home.test.tsx` 卡片一格；同一格也抓方块去掉边（bn2-tile） |
| 949 | S1 | 空首页的"新建笔记本"回到默认大小 | `workspace-home.test.tsx` 从空首页新建一格（`buttonClass`） |
| bi1 | S1 | `EntryCard` 去掉 `break-words` | `entry-card.test.tsx` 新格；故事：UI1 第 5 格 `:335`，375 宽未登录那页页宽 482，三种浏览器（审查 B 的 P5S1B-I1） |
| 948 | S1 | 邀请那一句回到 `text-lg`；步骤与 `h1` 的外层换成 `contents`（948-step） | `invitation.test.tsx:426`；`onboarding.test.tsx:72` |
| 946 | S1 | `PageColumn` 去掉空头一行（946-head）；去掉右栏的地方（946-room）；外层去掉 `md:-mt-3`（946-outer） | UP1 (page) 的那一步，三种浏览器：1440 宽 y 12 对 78；x 466 对 360；y 90 对 78（当时在 `:265`，今天 `:268`） |
| an4 | S1 | UI1 第 5 格去掉开头的 `setViewportSize` | `expectFits` 的守卫"a phone's width or narrower"（1280 > 375），三种浏览器（审查 A 的 P5S1A-N4） |
| 951 | S1 | `panel` 默认 `true`（951-default）；页的一支早退不传 `panel`（951-page-loading、951-page-gone） | `page-layout.test.tsx`：不带 `panel` 的一格；页的两支各一格（决定 951） |
| n1 | S1 | 空头一行回到 `mb-7.5 min-h-9`（n1-plain；故事 n1-plain-story）；面包屑那一块不带一行的高（n1-no-line） | `page-layout.test.tsx` 空头一行一格；故事：UP1 (page) `:268`，768 宽 y 78 对 99.5，三种浏览器（审查 B 的 P5S1B-F1-N1） |

第 33 条照 00 号文档 4.8 的最低清单（"把 `--faint` 或 `--accent` 调淡，se1 或 l5 变红"）取 `--accent`：se1 不量 `--faint`（4.7"说明文字不量"）。第 90、91 条没有格抓，由截图看，写进 10.x；第 15 条等价，不跑；第 76、86 条另有截图。 表末 t2 起的各行是 S1 的实施与审查加的（[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)，逐条的改法、在哪棵树上跑与审查者另写的在那里）：t 是实施者另写的（t1–t12 里活下来的四条由 `9975478d` 补格，t7 由 `45f02991` 补格），cx 是审查者的，以决定号起头的是修复的；第 15 条等价（审查 C 的 `probe15/`：1,000 个顶层页，`indexTree` 改前改后都只调 1 次）。

### 7.3 门禁

- **基线**：不另跑（同 P4 的决定 883）。对照的数是第 1 节的：vitest 228 个文件、5455 格；E2E 636 格过、3 格失败、1 格跳过。
- **每个提交**：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`；改了行为、夹具或故事的另跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make e2e`（S1 的提交 1–3，S2 的提交 1、2，S3 的提交 1、2 都要）。
- **合并之前**：在 Step 分支并入 `m13-ui-refresh` 的树上跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web gen-check-web build-web e2e`；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 以 0 退出；`git diff --check`；结果写进 Step 的记录。本机允许的失败只有第 1 节那三格，同行同消息：P5 不改 `se2-*.spec.ts`、`lw8-health.spec.ts`、`pg15-reading-updates.spec.ts` 三个文件，SE2 的那一格照旧在 `se2:231`（S2 改了搜索页的样子，消息照旧是 `toHaveValue`；行号或消息变了先停、查清），LW8 的照旧在 `lw8:836`（S3 改了健康页的类名，Tab 的次序不变），PG15 的照旧在 `pg15:113`。AS6、`lw8:771`、"not slower"、`trash-preview.test.tsx:587`（S2 的新格加在那个文件的末尾，这一格的行号不挪）、造数的 500 与 WebKit 的"mermaid could not be loaded"偶发，重跑全过即可，记下；别的红了先停、查清。
- **CI**（决定 944）：M13 在 CI 上只跑过一次：负责人推送 `f62d0390` 之后的 run 45551442052，e2e 红的三格（PG17 两格、C8）由 `9871c7d4` 修好（04-P4 10.4），修好的头还没推送（决定 746：没有推送的凭据时以本机门禁替代分支 CI，负责人推送之后 CI 在最后一个头上补跑）。下面这些只有 P5 推送之后的 CI 才看得到，都等负责人推送之后：Linux 的字体下 UI1 两格的截短与折行、TB1 与 hi2 的行数（比的是按行高算的界，推断照样成立）；Linux 的 Chromium 上 hi3 的 `emulateMedia({ forcedColors })`（L4 的同一步在 run 45551442052 里已在 Linux 的 Chromium 上跑过、没有红；hi3 是另一页，照样等 P5 推送之后的 CI。初稿写"L4 已在 CI 上用过，P4/S2"，在写的时候不对，审查 C 的 P5C-Q1）；mc3 那一句在 Linux 的 `webkit-narrow` 上。 照实（S1，[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：负责人推送 `e8809170` 之后分支的 CI（run 45579305353）全绿，E2E 一步 8 分 5 秒，两处修复在 Linux 上成立；release 的演练（run 45579305372）也绿。S1 另有只靠 CI 的：UI1 第 5 格的邀请一步与 UP1 (page) 的新一步在 Linux 的字体与滚动条下（x 与宽由列宽定、与字体无关；会占位的经典滚动条会让居中的列左移，PG17 在 run 45579305353 上断言过 `clientWidth` 是 375，反面的证据，审查 C 修复核对第 1 轮）。
- **文档**：`python3 docs/v0.1/claude_to_codex/tools/check_links.py origin/m13-ui-refresh`（远端的分支不在时用 `m13-ui-refresh`）、`git diff --check`。

### 7.4 视觉的核对

B 审查者用 Playwright 截取下面这些，亮暗、1440 与 375 各一张，对照 settings.html（卡片、行、分组标题、徽记）与 reading.html（头一行、32 px 的标题、720 px 的一列、`.meta .dot`）看，记录写明看了哪些。以应用的截图与本文的类名为准；第 1 节 (h) 的探针页是照本文的类名写的静态页，可以并排看，不是样稿，也不是应用（初稿的参照页与正文有几处不同，不再引，审查 B 的 P5B-N8）：

- **S1**：工作区首页（两组、三列、一本 LLM Wiki、一个长名字、虚线卡；只有一组；访客；没有笔记本的空状态，能建与不能建的人；管理员的提醒；375 宽一列时卡片随内容高）；笔记本首页（顶层页面带数与不带数、长名字的页、长名字的笔记本的标题与"新建页面"、没有页面、升级中的横幅在列里、键盘焦点在第一行与最后一行的子页面上）；页的子页面一节与标签页（图标）；页读取中与页不在时横幅在列里（1440 宽，与读到之后同一处）；登录（开放注册与不开放）、注册（关闭时）、邀请（未登录、已登录有与没有工作区、会话不可用）、引导的三步、新建工作区（有与没有工作区）；404（工作区外、笔记本里）、出错页（带错误码；"回到首页"与"重新加载"同是胶囊）、会话不可用的页；设置里的五个空列表；强制色下的卡片、虚线卡、入口的卡片（标志隐去）；短名字的笔记本首页在 320、375、390 宽"新建页面"在标题旁，中长的名字时在下一行靠右（审查 B 的 P5B-R1-M1）。
- **S2**：搜索（没提交的空状态、有命中、没有命中只有状态行、长路径）；历史（筛选条的三个下拉与页的筛选小块，长名字的"谁"，375 宽折行，筛选小块折成几行时的圆角；"只看此人"静止、悬停、键盘聚焦、强制色）；变更集页（头一行的"撤销"、长标题、标题下的事实、读者的说明、改动的卡片）；回收站（有项、空、长名字的项、长的"来自"一行的项，"恢复"在右上）；预览（页与附件、长名字、"恢复"两个在标题旁与在标题之下靠右、带长名字的"在后台恢复"在 375 宽折行、并入的提示）；目录（筛选、分组、空的两种）；标签（有页、空）；各列表的一行并排（搜索、历史、变更集的改动、回收站、目录，同一种第一行与链接，审查 B 的 P5B-I2）；暗色的 se1 那一页的焦点环在卡片上；历史行的第一行与搜索的行同高（"只看此人"静止时不撑高）；摄取的变更在历史行与变更集页里那一块的字号与链接；变更集页标题下的事实是一种灰、消息是正文色；回收站的行在 375、320 宽"恢复"在文字之下靠右；预览里附件的图标与页的同大、同位置（审查 B 的 P5B-R1-M3、M4、N2、N4、N5、N7）。
- **S3**：版本页（头一行的"还原"、"当前"的琥珀小块、编辑中的小块、锁说明、阅读与两种对比；320 宽；375 宽、版本列表扣住时与读到之后头一行一样高，审查 A 的 P5A-M4；能编辑的人看最新的版本时 375 宽空着的那一行）；右栏的版本行（带"当前""编辑中"、e2e 式的长名字，xl 宽）；三种文档类页标题下的事实同一种样子；健康（各卡片、"重新检查"、`details` 展开）；升级（各节）；附件一节（页的底部与笔记本首页：上传与上传中的列表、拖进来时的琥珀环，页的阅读视图那一块的拖放环同色，下载的图标、"操作"静止、悬停、菜单开着）；导入与粘贴来源两个对话框里的框（暗色下框里的描边按钮看得出），粘贴来源的文本框（聚焦时，强制色下聚焦时）；升级横幅在每类页上（首页、列表、版本、设置），状态前的小圆点（375 宽中文的状态折成几行时对齐第一行）；中文的右栏版本行在 Firefox、xl 宽时"当前""编辑中"不断开；历史行的"Undone"小块悬停（审查 B 的 P5B-R1-M2、N1、N3）。

### 7.5 CI 的余量

新的格：UI1 两格（三种浏览器，共 6 次）、TB1 一格（Chromium）；hi1、hi2、hi3、mc3、as1 各多一两句。UI1 已有的格每次约 2–5 s（P2/S3 的记录）；第 4 格建三本笔记本与两页、开三页、改三次窗口，推断每次 6–9 s；第 5 格两次导航、一次点按，推断 2–4 s；TB1 一格建页、删除、开一页、一次假的拒绝，推断 3–6 s；hi2 多一次改窗口与量，hi3 多两次 `emulateMedia`，mc3、as1 各一句，各不到 1 s。合计约 30–50 test-seconds，CI 的 `ubuntu-latest` 约 2 个 worker，约合 15–25 秒的墙钟。本机 E2E 2.6–2.8 min 推断到 2.7–3.0 min。CI 的 E2E 一步在 `f62d0390` 上是 8 分 22 秒（run 45551442052，决定 944；那一次红了三格，修好之后的头还没在 CI 上量过），时限 18 分钟（00 号文档第 9 节）；照本机的增量（2.6–2.8 → 2.7–3.0 min）推断余量足够，这是推断，等负责人推送之后的 CI 才有数。S1、S2、S3 之后照实各记一次本机的时间，S3 的那一次是 00 号文档第 9 节要的"P5 之后记一次时间"。 照实（S1，[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）：CI 的 E2E 一步在 `e8809170` 上是 8 分 5 秒（run 45579305353，全绿）。S1 本机的 E2E 每次 2.7–2.9 min（646 格）。UI1 两格与 UP1 (page) 远快于推断，合并之前的整条命令里（Chromium / Firefox / WebKit）：`9975478d` 第 4 格 0.50 / 0.91 / 0.91 s、第 5 格 0.35 / 0.60 / 0.50 s；`de40d1e6` 第 4 格 0.50 / 0.99 / 1.1 s、第 5 格 0.68 / 1.5 / 1.3 s、UP1 (page) 1.7 / 2.7 / 3.4 s；`dc59e9e2` 第 4 格 0.55 / 1.0 / 1.4 s、第 5 格 0.68 / 1.8 / 1.6 s、UP1 (page) 2.4 / 3.1 / 3.4 s（UP1 多了五种宽度各一次扣住的读）。

**本机的总时长**（粗算，经 `heavy.sh` 串行）：七个代码提交各一次 `make lint-web knip test-web`（约 70 秒）约 8 分钟；七次 `make e2e`（约 5 分钟，含构建）约 35 分钟；三次合并之前的整条命令约 13 分钟、审查 C 各再跑一次，约 78 分钟；变异 97 条（1–96 与 43b）：72 条只跑 vitest（一个文件 15–40 秒）约 32 分钟，22 条的抓手是故事，要重建前端再跑一个（每条约 3 分钟，UI1 的四条三种浏览器各约 4 分钟）约 70 分钟，第 15 条等价、不跑，第 90、91 条只看截图；修复核对每个 Step 一两轮约 45 分钟。合计约 4.5 小时的锁时间。

## 8. 完成标准

### 8.1 条件

1. 第 4 节的每一项做到；7.1 的测试通过；7.2 的变异都被抓住（第 15 条等价、不跑，理由在它那一行；第 90、91 条没有格，由 7.4 的截图看；第 76、86 条另有截图），结果写进 10.x。
2. 每个 Step 的合并之前，7.3 的门禁为绿（只败第 1 节同行同消息的那三格）；没有服务端与契约的改动。只有 CI 看得到的几样等负责人推送之后（决定 944）。
3. 7.4 的截图看过，与本文的类名或 settings.html、reading.html 不同之处写进记录（是有意的，写理由；不是的，修）。
4. Phase 完成（02-workflow 3.7）：S3 合并、它的合并记下之后，另一个提交 `docs: M13/P5 done (M13/P5)`：00 号文档第 12 节 P5 一行"完成"、头的状态、第 13 节一行；本文头的状态。`docs/v0.2/handoffs/M12-polish.md` 的"处理进展"由每个 Step 的文档提交各写一行，这里不另写。

### 8.2 总体设计的新文字

每条由 Step 的文档提交里的脚本替换（"换成"是旧文字换成新文字，"之后加"是新文字接在旧文字之后），断言旧文字恰好出现一次：S1 的第 1–5 条的旧文字在 2.1 落进去之后的 `docs/v0.1/v0.1-design.md` 里各出现一次（2.1 对总体设计只改 12.2 的一行与第 15 节的头一行，与这里的旧文字不重叠）；S2 的第 6–10 条、S3 的第 11–16 条的旧文字有的是之前的 Step 写进去的，在之前的 Step 改完的文字里各出现一次。设计者用 `p5-design/replay.py` 从本节读出这些文字（引用块是旧文字与新文字），在 `3e288acb` 的副本上先做 2.1，再依次做 S1、S2、S3 的 18 处替换（第 3、6 条各两处），每个旧文字在它那一步做之前都恰好一次，每条新文字在它那一步做完之后也恰好一次（修订之后重跑过；脚本在会话的临时目录，不进仓库，审查 C 的 P5C-I3）。中文里的引号照总体设计的写法是 ASCII 的 `"`。13.2 第 17、31 条里句与句之间不空格，接在其后的新文字不以空格开头；13.4 第 3 条里句与句之间空一格，新文字以一个空格开头（审查 C 的 Nit）。链接在这里写在反引号里（它们相对于 `docs/v0.1/`，不是本文的链接，免得链接检查把它们当成本文的；写进总体设计时去掉反引号，同 P4 的 8.2）。第 5、10、16 条的日期写合并的那一天（`YYYY-MM-DD` 由实施者换掉）。2.1 第 13、14 条（12.2 的一行与它的变更记录）不在这里：它们随设计的提交改。13.2 第 17 条在 S1、S2、S3 都改（决定 924、940、941：通往别处的内容链接随改写那一页的 Step 带上到达的状态；初稿说 S3 不改，那时还没有决定 940）。

1. **9.2 中间一段**（S1）：两个首页、入口的卡片、出错与 404、空状态（审查 C 的 P5C-M9：只有有子页面的行带数；搜索没有命中与右栏是空状态的例外；决定 950：S1 只说两个首页与设置的卡片，列表类的页由第 6 条放宽）。

   旧文字：

   > 笔记本首页列出根下的附件（M7）。

   换成：

   > 工作区首页居中、最宽 928 px，是笔记本的卡片（书本图标、名称、LLM Wiki 的徽记、开放程度与成员数；"我的""团队"两组，宽屏三列），能建笔记本的人在两组之后另有一行虚线的"新建笔记本"；笔记本首页居中、最宽 720 px，是标题与"新建页面"、顶层页面的列表（有子页面的行尾带子页面数）与根下的附件（M7；M13/P5/S1）。登录、注册、邀请、引导与新建工作区是居中、宽 400 px 的卡片，顶上是标志与产品名；出错页、会话不可用与 404 是居中的图标、一句说明与去路（"回到首页""重新加载"或"重试"）；两个首页与设置的卡片里，没有东西的列表是一个图标、一句说明与主操作（如果有），搜索没有命中时照旧只有状态行，右栏的"没有反链""没有属性"照旧一行灰字（M13/P5/S1）。

2. **13.2 第 17 条**（S1）：通往设置、首页与工作区的内容链接（决定 924、941）。

   旧文字：

   > 右上角的"回到工作区"同样带 `arrived`（M13/P2/S1）。

   之后加：

   > 工作区首页无主笔记本提醒里的"查看"（去无主笔记本的设置页）、已登录、账户已有工作区且引导已走完时邀请页右上角的"回到工作区"（去 `/`，`LandingPage` 转到最近的工作区；账户还没有工作区或引导没走完时不放），与 404、出错页的"回到首页"同样带 `arrived`（M13/P5/S1）。

3. **13.2 第 31 条**（S1）：卡片的网格、`expectFits` 量的宽度与卡片里整行链接的焦点外框（审查 C 的 P5C-I1、B 的 P5B-M3；第一处引的那一句延长到"在 Chromium 与 WebKit 上跑"，换上的写明哪些格在哪些浏览器上跑，审查 C 的 P5C-R1-N1）。

   旧文字：

   > 375 px 的检查用 `e2e/fixtures/narrow.ts` 的 `expectFits`，在 Chromium 与 WebKit 上跑（标签 `@narrow`）

   换成：

   > 卡片的网格在最窄的一档也写明一列（`grid-cols-1`，即 `minmax(0, 1fr)`）：只写 `sm:grid-cols-2` 时最窄的那一档是隐式的 `auto` 轨，截短的名字照样以全长撑宽（工作区首页，M13/P5/S1）。375 px 与更窄的检查用 `e2e/fixtures/narrow.ts` 的 `expectFits`（窗口不宽于 375，页宽与各个盒子的右边比的是窗口的宽度，M13/P5/S1）；带 `@narrow` 的格在 Chromium 与 WebKit 上跑（项目 `webkit-narrow`），别的格在它的故事跑的浏览器上跑

   旧文字：

   > 链接与别的元素是 2 px 的实色外框、偏移 2 px，原生的单选与复选同样；

   之后加：

   > 铺满卡片一行的链接（子页面列表的行）偏移 -2 px、圆角随行，外框收在卡片里（M13/P5/S1）；

4. **13.4 第 3 条**（S1）：UI1 的两格与 UP1 的一步（审查 A 的 P5A-N5、B 的 P5B-M4、C 的 P5C-M7：卡片与子页面列表里截短，标题折行；S1 的审查 B 的 P5S1B-I1、P5S1B-M1、P5S1B-F1-N1，决定 946）。

   旧文字：

   > UP1 另核对"编辑"消失时焦点到标题、读的那一段不动。

   之后加：

   >  M13/P5/S1：UP1 另核对树读到之前与之后横幅的位置与宽相同（1440、1280、1024、768、375 宽）；UI1 另两格在三种浏览器里核对：375 宽时工作区首页的卡片里长名字截短一行（320 宽也不横向滚动），笔记本首页长名字的标题折行、"新建页面"在它之下靠右，短名字的标题与"新建页面"同一行，子页面列表里的长名字截短一行、截在行尾的子页面数之前，子页面数由链接的描述读到、那一句本身藏着；没有断点的工作区名的邀请页（未登录）与本地部分 40 字的邮箱的邀请页（已登录）在 375 与 320 宽不横向滚动、卡片里的字与盒子不出卡片，登录的卡片与 404（"回到首页"之后首页的 `h1` 拿着焦点），只有页面版本：只有界面的样子，接口与 MCP 版本豁免。

5. **第 15 节**（S1）：变更记录一行（`YYYY-MM-DD` 换成合并的日期），加在 2.1 第 14 条那一行之上。

   旧文字：`## 15. 变更记录` 与其后的空行。

   之后加：

   > - YYYY-MM-DD：M13/P5/S1（依据 `[P5 设计](M13-ui-refresh/05-P5-pages.md)`）：9.2 的中间一段（两个首页、入口的卡片、出错与 404、两个首页与设置里的空状态）；13.2 第 17 条（提醒的"查看"、邀请页的"回到工作区"与"回到首页"带 `arrived`）、第 31 条（卡片的网格写明一列，`expectFits` 比窗口的宽度，卡片里整行链接的焦点外框，哪些格在哪些浏览器上跑）；13.4 第 3 条（UI1 的两格、UP1 的一步）。

6. **9.2 中间一段**（S2）：列表类的页（决定 925、929、945）与它们的空状态（决定 950）。

   旧文字（S1 写进的）：

   > 右栏的"没有反链""没有属性"照旧一行灰字（M13/P5/S1）。

   之后加：

   > 搜索、历史、回收站、目录与标签的页居中、最宽 720 px：标题、一两句说明、筛选条与卡片里的行，行里的次要操作（历史的"只看此人"）在悬停或键盘聚焦时显出；变更集页与回收站的预览也是 720 px 的一列，头同页的头一行，"撤销这次改动""恢复"在它的右边（M13/P5/S2）。

   旧文字（S1 写进的）：

   > 两个首页与设置的卡片里，没有东西的列表是

   换成：

   > 两个首页、设置的卡片与列表类的页里（搜索没提交时、历史、回收站、目录与标签，M13/P5/S2），没有东西的列表是

7. **13.2 第 31 条**（S2）：行内次要操作的显出（决定 927）。

   旧文字：

   > 靠颜色区分的另有颜色之外的提示（搜索命中加粗到 700）。

   之后加：

   > 列表行里的次要操作（历史的"只看此人"）平时透明、不藏起（Playwright 照样点得到，读屏照样读得到），悬停这一行、键盘的焦点进了这一行或它自己、它的菜单开着（`aria-expanded`）时显出；没有悬停的设备、窄屏（< 768 px）与强制色下一直显示（`app/page-classes.ts` 的 `revealClass`，M13/P5/S2）。

8. **13.2 第 17 条**（S2）：列表类的页与变更集页通往别处的链接（决定 924、940；审查 A 的 P5A-M2、P5A-Q1）。

   旧文字（S1 写进的）：

   > 与 404、出错页的"回到首页"同样带 `arrived`（M13/P5/S1）。

   之后加：

   > 列表类的页与变更集页的行和头里通往别处的内容链接同样带 `arrived`，去处的 `h1` 取焦点：历史行的主链接、"已撤销"与"撤销了「…」"，变更集的改动行的"在回收站里""这个节点的历史"与正文的版本，回收站的行，变更集页的"查看这次撤销""撤销了「…」"与"被撤销："之后的各个；后台的答复丢了、说不出是哪个任务时变更集页、回收站与它的预览里的"查看任务"（去导入导出的设置页）也带（M13/P5/S2；已带路由状态的照旧：改动行的主链接与健康的项经 `openOf`，目录的行，"查看进度"经 `focusJob`）。

9. **13.4 第 3 条**（S2）：TB1 的一格、hi3 与 mc3（决定 927，审查 B 的 P5B-M12）。

   旧文字（S1 写进的）：

   > 登录的卡片与 404（"回到首页"之后首页的 `h1` 拿着焦点），只有页面版本：只有界面的样子，接口与 MCP 版本豁免。

   之后加：

   >  M13/P5/S2：TB1 一格在 375 宽核对回收站的预览里没有空格的长名字（标题折行，"恢复"、带名字的"在后台恢复"与面包屑都不出屏），只有页面版本：只核对样子，删除是造数，恢复被假的答复拒了，接口与 MCP 版本豁免；hi3 另核对"只看此人"静止时透明、键盘的焦点进了那一行与强制色下显出，mc3 另核对 375 宽时它静止也显着。

10. **第 15 节**（S2）：变更记录一行，加在 S1 那一行之上。

    旧文字：`## 15. 变更记录` 与其后的空行。

    之后加：

    > - YYYY-MM-DD：M13/P5/S2（依据 `[P5 设计](M13-ui-refresh/05-P5-pages.md)`）：9.2 的中间一段（列表类的页与它们的空状态、变更集页与回收站的预览的头）；13.2 第 17 条（列表类的页与变更集页通往别处的链接、"查看任务"带 `arrived`）、第 31 条（行内次要操作的显出）；13.4 第 3 条（TB1 的一格、hi3、mc3）。

11. **9.2 中间一段**（S3）：版本、健康、升级与附件（审查 C 的 P5C-M9：升级页只有两个列表在卡片里；决定 943）。

    旧文字（S2 写进的）：

    > 变更集页与回收站的预览也是 720 px 的一列，头同页的头一行，"撤销这次改动""恢复"在它的右边（M13/P5/S2）。

    之后加：

    > 版本页同样，"还原到这个版本"在头一行的右边，"当前"同右栏是琥珀的小块，右栏的版本行里"当前""编辑中"接在"· 编辑"之后；健康与升级的页同样居中、最宽 720 px，健康的每条规则是一张卡片，升级页的两个列表在卡片里（各节照旧）；附件一节是卡片里的行，不内联的附件名字之后一个下载的图标，"操作"同样在悬停或聚焦时显出（M13/P5/S3）。

12. **9.2 中间一段**（S3）：升级横幅总在列里（决定 915、935）。

    旧文字：

    > 升级中的笔记本的每个页面上方有一条常驻的提示（M11）；页面与设置页里它在那一列的最上面、标题之前（M13/P4/S1）。

    换成：

    > 升级中的笔记本的每个页面上方有一条常驻的提示（M11）；它在那一页的列的最上面、标题之前，读取中、读不到与页不在的各支也在那里，状态之前一个琥珀的小圆点（页面与设置页读到时 M13/P4/S1，其余的页与各支 M13/P5）。

13. **13.2 第 17 条**（S3）：版本页与升级页的链接（决定 940）。

    旧文字（S2 写进的）：

    > 目录的行，"查看进度"经 `focusJob`）。

    之后加：

    > 版本页的作者与升级页通往页面的链接同样带（M13/P5/S3）。

14. **13.2 第 31 条**（S3）：附件行的"操作"（决定 927、933）。

    旧文字（S2 写进的）：

    > 列表行里的次要操作（历史的"只看此人"）平时透明

    换成：

    > 列表行里的次要操作（历史的"只看此人"，M13/P5/S3 起还有附件行的"操作"）平时透明

15. **13.4 第 3 条**（S3）：hi2、hi1 与 as1（决定 929、943）。

    旧文字（S2 写进的）：

    > mc3 另核对 375 宽时它静止也显着。

    之后加：

    >  M13/P5/S3：hi2 另在 320 宽核对版本页的标题是一行、"还原到这个版本"在头一行里、都不出屏；hi1、hi2 读右栏的版本行时"当前"接在"· 编辑"之后、隔一个空格；as1 另核对附件的"操作"在菜单开着、指针移开时照样显着。

16. **第 15 节**（S3）：变更记录一行，加在 S2 那一行之上。

    旧文字：`## 15. 变更记录` 与其后的空行。

    之后加：

    > - YYYY-MM-DD：M13/P5/S3（依据 `[P5 设计](M13-ui-refresh/05-P5-pages.md)`）：9.2 的中间一段（版本、健康、升级与附件；升级的提示在每一页每一支的列里）；13.2 第 17 条（版本页与升级页的链接带 `arrived`）、第 31 条（附件行的"操作"）；13.4 第 3 条（hi2、hi1、as1）。

## 9. 交给后面的 Phase

- **收尾**（00 号文档第 7 节"收尾"）：
  - 读屏清单"H. M13"一节加：空状态（图标不读、说明是普通的段落、主操作是按钮，不播报）；入口的卡片（产品名是普通的文字、在 `h1` 之前读到，标志不读）；已登录的邀请页右上角的"回到工作区"（账户已有工作区时才有，决定 941）；工作区首页的卡片（链接名是"名称 LLM Wiki 开放程度 · N 位成员"连读，记下各读屏有没有在几段之间停顿）、虚线的"新建笔记本"（按钮，与左栏的同名）；笔记本首页的子页面数（看得见的数不读，读屏读链接的描述"3 个子页面"：按 Tab 走到链接时与浏览时各听一次，记下各读屏读不读描述，决定 919）；出错页与会话不可用的 `alert`（打开时读出整段）、邀请页里会话不可用的 `h2`；列表行里透明的"只看此人"与"操作"（透明时照样可聚焦，读屏照样读到）；版本页与变更集页的按钮在 `h1` 之前（阅读的次序是面包屑、按钮、标题；到达之后焦点在 `h1`，从它起 Tab 先到作者或下一个链接，主操作要往回找，同 P4 的"编辑"，决定 929，审查 A 的 P5A-N3）；右栏版本行的"当前""编辑中"接在作者之后读；升级横幅在每一页每一支的列里、标题之前，状态前的小圆点不读；下载的图标不读（名字里不说"下载"，16.7 读屏那一半留给 v0.2）。
  - README 与 `CHANGELOG.md` 0.1.0：P5 没有改它们。`CHANGELOG.md` 的"网页的打磨"一条（`CHANGELOG.md:43`）里没有 P5 做掉的已知限制。README 描述界面的句子里与 P5 有关的：`README.md:471` 说笔记本的外壳与设置，没有"笔记本首页有设置链接"的说法，"管理员的工作区首页提醒还有几本无主笔记本"照旧成立，不用改；顺带看到 `:471`"开放程度点'保存'才发出"在 P3/S2 之后已不准（即生效的下拉，03-P3 决定 825），收尾改 README 时一并改。
  - 全部 e2e 的完整跑：P5/S3 合并之前的门禁跑过一次、记了时间（2.1 第 4 条）；收尾照 00 号文档第 7 节在 `main` 上再跑。M13 在 CI 上的第一次跑等负责人推送（决定 944），7.3 列的那几样那时才看得到。
- **v0.2**（写进 `docs/v0.2/handoffs/M12-polish.md` 的"处理进展"，各 Step 的文档提交）：M12-polish 第 1 项里落在 P5 的页上、P5 不认领的（决定 937 的清单，逐项带理由：1、9.4、16.2、16.4a、16.5、16.9、21 与 22 的各条、23.2、23.5–23.18 的各条、24.4、24.5、26.6–26.10 的各条、27.1、31.2、34、35.4–35.6、36、38.2、38.4、38.13、42–47）；16.7 读屏那一半；第 5、11 项里落在 P5 的页上的约二十处 `bdi`（要逐处看名字之后紧跟的是什么，决定 937）；第 5 项的"变更集页面包屑的 `current`"；骨架（决定 914：若以后有分批的长列表再看）。右栏版本行单独一行的"-"不再交出：S3 提交 1 照决定 943 改了（第 1 节 (g) 的扫描：今天的写法三种引擎都复现，内联之后三种引擎都是 0；小块带 `whitespace-nowrap`，中文的小块也不在字间断开），S3 的文档提交在 04-P4 第 9 节记下。
- **负责人**：无新的人工核对。M13 的 CI 等负责人推送（决定 944）。

## 10. 结果

（S1、S2、S3 合并之后各写一节：提交、做完的、与设计不同之处、实测、审查记录。）

### 10.1 S1：首页与入口（2026-11-25，合并 `<待填>`）

- **提交**：照 6.1 的 `262b7232`（提交 1）、`d65c9132`（提交 2）、`9ef5d2de`（提交 3），说明与 6.1 逐字相同；照实加的 `9975478d`（变异 t2、t3、t6、t12 在 `9ef5d2de` 上活下来之后补的格）；第一轮审查之后的 `b46675d9`、`45f02991`、`4340e64a`、`cdddecdd`、`de40d1e6`，修复核对第 1 轮之后的 `31e05f59`、`e41fce83`、`dc59e9e2`（各做了什么见 6.1 之后的照实）；加本文档提交。[S1 的审查记录](reviews/P5-S1-homes-entry-review.md)。
- **基线**：不另跑（7.3）。提交 1 的 `make e2e` 兼作核对，只败第 1 节那三格（se2:231、pg15:113 [webkit]、lw8:836，同行同消息）。之后 se2:231、lw8:836 每次都红，同行同消息；pg15:113 [webkit] 有时过（实施者的十四次里过了 5 次，审查 C 的两次里过了 1 次）。`de40d1e6` 的那一次另红一格 smoke S1（`s1-server-ready.spec.ts:40` [chromium]，关停时 River 的 jobcompleter 超过 `jobs.shutdown_timeout`），单独重跑 18 次全过，记为偶发（新，审查记录）。
- **做完**：4.1.1–4.1.8；决定 913、914、917–923、938、941、942；915 里 S1 的部分（笔记本首页的 `handle` 与 `<ColumnTop />`，页的两支早退包进 `PageColumn`，"路由 × 支"表的八行）；916 里 S1 的部分（工作区首页最宽 928 px、笔记本首页最宽 720 px）；924 里 S1 的部分（提醒的"查看"，404 与出错页的"回到首页"带 `arrived`）；925 里 S1 的部分（`cardClass`、`pageTitleClass` 导出，`W/app/page-classes.ts` 的五串）；931 里 S1 的部分（卡片与子页面列表的行里名字截短一行、带 `title`）；937 里 S1 的部分（第 1 项的 1、9.4 不认领）；4.5 的 `noColour` 三个词与 `REQUIRED` 四行；决定 946–951（950 是本文档提交的一条：8.2 第 1 条收窄；948 之下主持者另定的两处：引导里另改的两句照"一律"，卡片里共用的 `Loading`、`NotLoaded` 一行照旧 16 px；951 之下的 P5S1B-F2-N1 归到 S2、S3）。
- **测试**：
  - vitest：5455（228 个文件）→ 5461（228，提交 1）→ 5484（230，提交 2；19 格，另 `messages.test.ts` 的 `test.each(keys)` 因四个新键多 4 格）→ 5494（231，提交 3）→ 5495（`9975478d`；另三处断言加在已有的格里）→ 5497（`b46675d9`）→ 5499（`45f02991`）→ 5500（`4340e64a`、`cdddecdd`、`de40d1e6`）→ 5503（`31e05f59`）→ 5504（`e41fce83`、`dc59e9e2`），都是 231 个文件。
  - E2E：640 格 → 643（UI1 第 4 格 × 三种浏览器）→ 646（第 5 格 × 三）；修复都在已有的格里加步。`check.mjs` 列 chromium 433、firefox 105、webkit 105、webkit-narrow 3（之前 431、103、103、3）。
  - 先写的新格在没改的树上红：提交 1 的 vitest 6 格（5 个文件；实施者没有留日志，审查 C 复现）；提交 2 的 vitest 21 格（8 个文件，`empty-state.test.tsx` 导入失败），UI1 第 4 格三种浏览器红在"the page's width"（旧树 375 宽时页宽 578–579 px，初稿推的是 485）；提交 3 的 vitest 9 格（5 个文件，`entry-card.test.tsx` 导入失败），UI1 第 5 格三种浏览器红在产品名的 `toBeVisible`；`9975478d` 的新断言在 t2、t3、t12（vitest）与 t6（三种浏览器，`:234`）下红；`b46675d9` 的 2 格（`%6Dembers`、`%74okens`）；`45f02991` 的卡片一格（方块的边、名字的 `flex-1`）与空首页新建一格（`buttonClass`）；`4340e64a` 的 `text-sm` 7 格（6 个文件）与 `break-words` 一格，UI1 第 5 格的邀请一步三种浏览器红在 375 宽未登录那页（页宽 482）；`cdddecdd` 的 UP1 (page) 那一步三种浏览器红（1440 宽读取中 x 486、y 24，读到之后 360、78）；`31e05f59` 的"不带 `panel`"一格；`e41fce83` 的空头一行一格与 UP1 (page) 那一步（三种浏览器红在 768 宽，y 78 对 99.5）。
  - 不是先红的：`design-tokens.test.ts` 的 `noColour` 加的三个词（守卫）；工作区首页读取中与读不到的两格、卡片与虚线卡的高的类名、读屏那一句的 `hidden`、页的两支留右栏的两格（代码本来就对，由 cx21、t7 一族、cx16、951-page 的变异证）；`listAnswered` 与 `SessionUnavailable` 的两处重构。
  - 每次门禁"not configured to support act"0 次；"Not implemented: navigation to another Document"照旧每次一次。
- **变异**（逐条在审查记录，7.2 的表末）：7.2 的 S1 各条除 15（等价，不跑）都被它写的格抓住；照实改了抓手的两条：10（决定 942 之后虚线卡不在任何列表里，用代理：包进 `ul > li`，"in no list"那一格红），62（只在 Firefox，红在第 4 步 `:213` 的行数）。故事抓住的：5（`:198`，页宽 563）、6（`:202`）、27s（`:346`，焦点）、96（`:242`，119 对 < 107），都是三种浏览器（行号照 `dc59e9e2` 的树）。实施者另写的 t1–t12：t2、t3、t6、t12 由 `9975478d` 补格，t7 由 `45f02991` 补格（同类的 t7-dashed、t7-h-full 也由它抓），t11（去掉 50 ms 并做 95）照样红，其余都被抓住。修复与修复核对加的各行（cx1、不解码、cx16 的 vitest 与故事两行、cx21、947、949、bi1、948、946、an4、951、n1）都红在说好的格；审查 C 另有四条：rc-edges-left、rc-mail-nowrap 红，rc-slow-95 红而 rc-slow-95-nosignal 假绿（`load` 的信号承重）。
- **视觉的核对**（7.4）：实施者在三种浏览器各截 308 张（英文亮暗 × 1440、1024、375、320，中文亮色 × 1440、375、320），另有焦点、Chromium 的强制色、引导三步读完之后、三份样稿；修复之后另截 946、947、方块的边、949、B-I1、948 的几张，修复核对第 1 轮之后截横幅在 1440、1280、1024、768、375 宽、右栏开着与收起时读取中、读到之后与页不在的样子（三种浏览器相同，读取中 = 读到之后 = 页不在：右栏开 360 / 78 / 720、292 / 78 / 696、278 / 78 / 720、276 / 99.5 / 468、24 / 155.5 / 327，右栏收起 1440 与 1280 宽 486 / 78 / 720、406 / 78 / 720，其余同开着）。实量（三种浏览器相同）：横向溢出 0；首页卡片 1440 三列（301.3 × 112）、1024 两列、375 与 320 一列（96.8 px 随内容）；虚线卡自成一行，375 宽高 56；访客没有虚线卡；短名字的笔记本在 320 宽"新建页面"在标题旁，中长的名字在 375、320 宽两行、"新建页面"在下一行靠右，长名字在 375 宽 3 行、320 宽 4 行；入口卡片 1440 宽 400 px 居中，375、320 宽左右各留 24 px，长的工作区名与地址在卡片里折行。对照 settings.html 与 palette.html（琥珀）：卡片、分组标题、圆角与暗色的面相符。WebKit 的焦点截图不算（macOS 上它的 Tab 跳过链接与按钮；审查 B 用 Option+Tab 走到了每个目标）。出错页的错误码那一行没有截到（中断分包的错误没有码），由 `route-error.test.tsx` 守。截图在会话的临时目录，不进仓库。
- **对比度**（4.4 没有 S1 的下限；se1、pg4、l5 每次都过；Chromium 1440 宽）：名字与标题亮 17.58、暗 12.99–14.07；muted 亮 6.18、暗 7.02；徽记亮 5.63、暗 6.16；`text-faint`（卡片的一行与子页面数）亮 3.07、暗 3.51，是 00 号文档 4.7 写明的取舍；焦点亮 3.10，暗在页上 7.92、在卡片里 7.31。
- **时长**（7.5）：本机 E2E 每次 2.7–2.9 min；UI1 两格与 UP1 (page) 的秒数见 7.5 的照实（远快于推断）；CI 的 E2E 一步在 `e8809170` 上 8 分 5 秒。
- **门禁**：每个提交的 `heavy.sh make lint-web knip test-web` 都绿（约 63–67 秒）；改了行为、故事或夹具的提交各跑 `CI=1 heavy.sh make e2e`，只败第 1 节的格（`de40d1e6` 另有一次 smoke S1 的偶发）。合并之前的整条命令在 `9975478d`、`de40d1e6`、`dc59e9e2` 上各跑一次（审查 C 另在前两个上各跑一次），每次只败基线那三格（或两格，pg15 过了）；受保护路径的 `git diff --quiet` 与 `git diff --check` 都以 0 退出，跑完 `git status` 为空。`dc59e9e2` 上：vitest 231 个文件、5504 格（57.6 s），E2E 642 / 3 / 1（2.9 min），整条 244 秒，负载 12.8 → 26.5。
- **与设计不同之处**：照实多九个代码提交（6.1 之后）；`roots` 由 `childrenOf(tree, null)` 算；会话不可用拆成 `useUnavailable` 与两个外壳，`SessionUnavailable` 不收 `level`（922）；出错页的"回到首页"复用 `notFound.home`；`sectionAt` 用路由的 `matchPath`、逐段解码（923）；`PageColumn` 照决定 946、951，S1 就导出；7.2 的第 10、62 行；旧树 375 宽的页宽是 578–579 px；`listAnswered` 等 `load` 的结果（7.1 之后）。
- **交给后面**：
  - S2：标签页的列表在 S1 之后铺满 `main`（S1 改了共用的 `SubpageList`，标签页的版式在 S2 换，审查 B 的旁记）；`PageColumn` 导出时空的头一行由调用者定形、选项按"页的头"命名，S2、S3 的表在 1440、768 与 375 宽量 x 与 y（决定 951 之下的 P5S1B-F2-N1，4.2.6）；8.2 第 6 条的第二处把空状态那一句放宽到列表类的页（决定 950）。
  - 负责人推送之后的 CI：UI1 两格在 Linux 的字体下的截短与折行，第 5 格的邀请一步与 UP1 (page) 的新一步在 Linux 的字体与滚动条下（7.3）。

## 附录：核对过的代码

行号是 `f62d0390` 的。

- 外框与列：`W/app/layout.tsx:62,70-71`；`W/app/corner.tsx:43-85`；`W/app/column-top.tsx:9-10`；`W/app/routes.tsx:59-95`（守卫、邀请、引导）、`:99-114`（落地页与新建工作区）、`:155-170`（工作区外框与首页）、`:178-262`（笔记本的子路由，`handle` 在 `:188`、`:266`）、`:298-305`（搜索）、`:341-347`（404）；`W/pages/notebook/notebook-layout.tsx:23,63,67-70,79-81,94-100`；`W/pages/notebook/upgrade-banner.tsx:41-55`。
- P3、P4 的部件：`W/app/settings-page.tsx:24,30-45,84-113,116-148`；`W/app/settings-directory.tsx:28-55`；`W/pages/settings/settings-layout.tsx:26-50`、`W/pages/workspace/settings-layout.tsx:16-30`、`W/pages/notebook/settings-layout.tsx:21-40`；`W/pages/page/page-layout.tsx:76-93,388-441,467,507-508`；`W/pages/page/page-versions.tsx:100-161`；`W/components/ui/button.tsx:6-38`；`W/components/ui/label.tsx:6-10`；`W/styles.css:155-170`；`W/design-tokens.test.ts:302,319-339`；`W/app/document-title.ts:3`；`web/apps/web/public/favicon.svg`。
- 首页：`W/pages/workspace/workspace-home.tsx:24-95`；`W/pages/workspace/notebook-groups.tsx:50-89`；`W/pages/workspace/ownerless-row.tsx:72`；`W/stores/notebook.store.ts:18-24`；`W/pages/notebook/notebook-home.tsx:36-73`；`W/pages/page/subpage-list.tsx:23-67`；`W/stores/page-tree.ts:38`、`W/stores/page-tree.store.ts:161-183,240-243`；`W/pages/notebook/page-tree.tsx:30-40,293-304`；`W/pages/notebook/page-tree-item.tsx:355-365`；`api/modules/notebook.yaml:452-548`。
- 入口与出错：`W/pages/sign-in.tsx:12-38`、`sign-up.tsx:16-57`、`credentials-form.tsx:54-98`、`invitation.tsx:31-180`、`onboarding.tsx:13-35`、`create-workspace.tsx:21-50`、`not-found.tsx:19-34`；`W/app/route-error.tsx:17-36`、`session-unavailable.tsx:18-47`。
- 设置的空列表：`W/pages/settings/tokens-page.tsx:46`、`W/pages/workspace/invitations-section.tsx:54`、`ownerless-page.tsx:152`、`audit-section.tsx:72`、`W/pages/notebook/transfer-page.tsx:235`；`W/pages/page/page-backlinks.tsx:140`、`page-properties.tsx:79`。
- 列表类的页：`W/pages/search/search-page.tsx:205-262`、`search-hit.tsx:36,87-126`、`search-status.ts:24,40`；`W/pages/notebook/history-page.tsx:170-232`、`history-row.tsx:62-115`、`history-filters.tsx:128-185`、`history-ingest.tsx:16`；`W/pages/notebook/changeset-page.tsx:226-275,458-459`、`revert-dialog.tsx:160,541`；`W/pages/notebook/trash-page.tsx:181-232`、`trash-row.tsx:288-319,449-459,498-522`、`trash-preview.tsx:185-202,330-346,396-462,592-605`；`W/pages/notebook/index-page.tsx:61-71,257-296`、`index-filters.tsx:42-60`、`index-rows.tsx:52-106`；`W/pages/notebook/tag-page.tsx:39-63`。
- 版本、健康、升级、附件、对话框：`W/pages/page/version-page.tsx:150-215`、`version-restore.tsx:60-75`；`W/pages/notebook/health-page.tsx:273-312`、`health-rule.tsx:230-259`、`health-item.tsx:54`；`W/pages/notebook/upgrade-page.tsx:43,614-615,653-654,698-699,765,781`、`upgrade-check.tsx:231-232,367-368,438`；`W/pages/page/attachments-section.tsx:211-284`、`attachment-row.tsx:100-175`、`attachment-drop.tsx:51`；`W/pages/notebook/import-dialog.tsx:139`、`source-add-dialog.tsx:135,261`。
- e2e：`e2e/playwright.config.ts:3-4`；`tools/e2e-projects/check.mjs:17-47`；`e2e/fixtures/narrow.ts:4,11-29`、`auth.ts:13-15`、`notebook-pages.ts:17-37,55`、`wiki-search.ts:8-36`、`wiki-pages.ts:174-180,391-401`、`wiki-health.ts:24-26`、`upgrade-pages.ts:31,48,56`、`trash.ts:13-66`、`pages.ts:101`；`e2e/stories/search/se1-search.spec.ts:270-347,401-421`；`e2e/stories/history/hi2-version-restore.spec.ts:72-132`、`hi3-log-revert.spec.ts:295-336`；`e2e/stories/trash/tb1-trash.spec.ts:35,224,346`；`e2e/stories/shell/ui1-narrow-drawer.spec.ts:1-13,46,119,135`；`e2e/stories/page/pg17-page-head.spec.ts:33-40,392-421`；`e2e/stories/notebook/n1-create-notebook.spec.ts:88,107-116`。
- 文档：`docs/v0.1/v0.1-design.md:852-853,963,1212,1238-1239,1271,1332-1334`；`docs/v0.1/M13-ui-refresh/00-M13-design.md:187-196,290-316,342-361,424-426,542-551`；`docs/v0.1/M13-ui-refresh/04-P4-reading.md:2087-2104`；`docs/v0.1/M13-ui-refresh/03-P3-settings.md` 第 9 节；`docs/v0.1/M13-ui-refresh/01-P1-foundation.md` 第 9 节；`docs/v0.2/handoffs/M12-polish.md` 第 1、5 项与"处理进展"；`docs/v0.1/M12-release/handoffs/M5-polish.md` 第 16、21–23、26、35、36、38、42–47 条；`docs/v0.1/M12-release/02-P2-web-polish.md:202`（决定 414）。
- 修订时另读（照设计审查，行号同样是 `f62d0390` 的）：`W/lib/cn.ts`；`W/app/layout.test.tsx:184-206`；`W/app/column-top.tsx:5`；`W/pages/notebook/notebook-layout.tsx:97-100`；`W/pages/page/page-layout.tsx:77-84,79-82,86-93,89-92,430`；`W/pages/notebook/upgrade-banner.test.tsx:35-36,141-220`；`W/pages/notebook/history-row.tsx:92-122`、`changeset-change.tsx:115-150`、`changeset-page.tsx:69,92,95,290-400`、`trash-page.tsx:259-274`、`trash-preview.tsx:66,249,270-308,361,419-432,599`、`index-page.tsx:270,277`、`index-rows.tsx:96`、`health-item.tsx:56-74`；`W/pages/search/search-hit.tsx:96-106`；`W/pages/page/version-page.tsx:76-82,136,186-203`、`page-versions.tsx:109-163`、`page-versions.test.tsx:21-29,111-141`、`version-restore.test.tsx:157`、`attachment-row.tsx:132-149`；`W/pages/notebook/upgrade-page.tsx:603-608`；`W/components/ui/input.tsx`；`W/pages/notebook/source-add-dialog.tsx:261-277`、`source-add-dialog.test.tsx`、`transfer-import.test.tsx`、`source-uploads.test.tsx`；`W/pages/notebook/revert-background.test.tsx:432-435`、`trash-background.test.tsx:388-391,686-689`、`trash-preview.test.tsx:587`；`W/pages/notebook/page-tree-scale.test.tsx:147,239-243`；`W/pages/notebook/page-tree.tsx:39-40`；`W/i18n/messages/en.ts`、`zh-CN.ts`（8.2 引的按钮与链接的中文）；`web/apps/web/build/out-of-main.ts`；`e2e/stories/history/hi1-versions.spec.ts:256-265`、`hi2-version-restore.spec.ts:93-95,114,132`、`hi3-log-revert.spec.ts:328-331`；`e2e/stories/mcp/mc3-write-edit.spec.ts:174-233`；`e2e/stories/asset/as1-upload.spec.ts:183`；`e2e/stories/trash/tb1-trash.spec.ts:269,361`；`README.md:471`、`CHANGELOG.md:43`；`docs/v0.1/M13-ui-refresh/04-P4-reading.md` 的头（"修订（照设计审查）"的写法）；三份审查与处置（会话的临时目录，`p5-rev-a`、`p5-rev-b`、`p5-rev-c` 的 `review.md` 与 `p5-design/dispositions.md`），B 的 `p5-rev-b/dash6.mjs`。
- 修订核对第 1 轮之后另读（行号同样是 `f62d0390` 的）：`W/stores/workspace.store.ts:52-66,85-90,126-131`；`W/pages/invitation.tsx:150-157`、`invitation.test.tsx:25-60,267-277`；`W/pages/workspace/workspace-layout.tsx:123-160`；`W/app/guards.tsx:74-81`；`W/app/routes.tsx:76-86`；`W/onboarding/steps.ts:55-62`；`W/app/layout.test.tsx:108-116`；`W/pages/page/page-versions.test.tsx:100-114`；`W/pages/notebook/changeset-page.tsx:359-408`；`W/pages/notebook/history-ingest.tsx:19-70`；`W/pages/notebook/trash-row.tsx:265-282`；`W/pages/page/attachment-row.tsx:20-31`；`W/pages/search/search-hit.tsx:86-96`；`W/test/fakes.ts:73-77`；`W/i18n/messages/en.ts`（"Undone""Undid “…”""See the undo""Undone by:""History of this node"）；`e2e/stories/history/hi1-versions.spec.ts:256-260`、`hi2-version-restore.spec.ts:111-115`；`e2e/stories/page/pg17-page-head.spec.ts:30-38`；三份修订核对的报告（`p5-rev-a`、`p5-rev-b`、`p5-rev-c` 的 `check-1.md`）、处置的"修订核对第 1 轮"一节，A 的 `probes/`、B 的 `dash7.mjs`、C 的 `probe1/`。
