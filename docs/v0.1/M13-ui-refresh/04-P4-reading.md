# M13/P4 阅读与编辑：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M13/P4 阅读与编辑（三个 Step：S1 页头、属性条与右栏；S2 正文的样式；S3 编辑） |
| 日期 | 2026-11-24（初稿；同日照[设计审查](reviews/P4-design-review.md)修订，又照它的修订核对第 1 轮再修订、第 2 轮三修；主持者照第 3 轮改了最后一条 Minor 与 Nit，第 4 轮核对之后定稿）；同日照 S1 的实施与审查修订（[S1 的审查记录](reviews/P4-S1-head-strip-panel-review.md)，主持者的决定 897–900）；2026-11-25 照 S2 的实施与审查修订（[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)，主持者的决定 901–906）；2026-11-25 照 S3 的实施与审查修订（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)，主持者的决定 907–911） |
| 状态 | 设计定稿（修订核对之后三位都没有 Important、Minor：B 在第 2 轮、A 在第 3 轮、C 在第 4 轮；主持者补上最后的 Nit；[审查记录](reviews/P4-design-review.md)，主持者的决定 854–896）；S1 实施与审查完成（[S1 的审查记录](reviews/P4-S1-head-strip-panel-review.md)，主持者的决定 897–900），合并 `715923dd`；S2 实施与审查完成（[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)，主持者的决定 901–906），合并 `cea66d69`；S3 实施与审查完成（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)，主持者的决定 907–911），合并 `4f2fde3b` |
| 上级文档 | [M13 总设计](00-M13-design.md) 4.5（全部）、4.1 末段"阅读视图的变量"与"语义名的例外"、4.3"单选与复选""按钮"两条、4.7、4.8（大纲的当前节、编辑时的大纲、复制按钮的 vitest；变异的最低清单）、第 7 节 P4 三行、第 8 节（注册"代码块的复制按钮"）、9.1"阅读视图的样式断言""右栏""编辑的'保存'""编辑器的字体与主题"四行、第 10 节"编辑器混排""滚动锚定"两行、"移交的落实"（"编辑时没有大纲"由 P4/S3 认领）；[03-P3](03-P3-settings.md) 第 9 节"P4"一条（`main` 的留白、细栏的位置文字、升级横幅与内容列）；[02-P2](02-P2-shell.md) 第 9 节"P2–P5"与"收尾"两条；[01-P1](01-P1-foundation.md) 第 9 节"P4"（任务列表的复选框）与"P2–P5"（标签组）两条；[v0.1 总体设计](../v0.1-design.md) 9.2、9.3、13.2 第 17、23–26、31、32 条、13.4 第 3 条；移交 [M12-polish](../../v0.2/handoffs/M12-polish.md) 第 1、5、6、11 项（编号沿用 [M5-polish](../M12-release/handoffs/M5-polish.md) 的条目） |
| 视觉参照 | [mockups/reading.html](mockups/reading.html)（头一行：`.topbar`、`.crumb`、`.meta`、`.btn.icon`、`.btn.primary`；正文：`.doc`、`.props`/`.chip`、`.callout`、`.diagram`、`.tasks`、`.code`/`.ch`、`table`、`code.ic`；右栏：`.panel`、`.ph`、`.outline`、`.bl`、`.kv`、`.ver`；编辑：`.editbar`、`.editor`、`.saved`） |
| 代码位置的写法 | 同 00 号文档：`W/` 是 `web/apps/web/src/`；e2e 写作 `e2e/stories/…:行`。行号是 `m13-ui-refresh` 的头 `072dcec2` 的 |
| 工作笔记 | 设计只写了本文（2.1 对 00 号文档的修订由主持者随设计的提交落进去）。设计者跑过的探针都在会话的临时目录 `p4-design/`，不进仓库：初稿的 (a)–(c)（第 1 节末）；修订时的 (d)–(i)：mermaid 11.17.2 的 28 张图、CodeMirror 的压淡与主题次序（三种引擎与 jsdom）、按钮的焦点、右栏的 `watchReader`、头一行的预留；再修订时的 (j)–(m)：头一行的定宽（决定 894）、`useCurrentHeading` 与 vitest 的 `act`、大纲晚到时的 `watchReader`、PG17 的造数；三修时的 (n)：右边一组的最小高。(k)、(l) 在 `072dcec2` 的分离工作树里跑了 vitest（用完即删）；没有跑应用与 e2e，Step 的代码没有写 |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P4 分三个 Step，串行，各自一个分支（`m13-p4-s<m>`，从 `m13-ui-refresh` 拉出、合并回它）、一次审查与修复核对、一次合并。每个提交单独为绿（7.3）。三个 Step 的分法照 00 号文档，只把 00 号文档没有分到的几件放进 S1（决定 854）。

- **S1 页头、属性条与右栏**（`m13-p4-s1`）：`main` 的留白（宽屏左右 40 px，决定 855）；页的两栏：内容区与宽屏上高同窗口、粘住的右栏（决定 860）；头一行：面包屑、"谁、何时编辑过"（决定 858）、右栏的开关（决定 859）、页面菜单、主按钮"编辑"（决定 865）；居中、最宽 720 px 的内容列与 32 px 的标题；升级横幅进内容列（决定 857）；右栏的小节、反链的小卡片、属性、版本换样子（决定 863）；大纲读到的一节（决定 861、886、887、893）与长标题的截断（决定 862）；属性条（CSS）与补 `role` 的增强 `propertyRoles`（决定 864）；编辑时头一行的状态、"保存"与"完成"（决定 866）、标题下的编辑说明（决定 867）；阅读时的锁说明换样子（决定 868，版本页同一个组件）；新故事 PG17 的第 1 格（决定 879）；up1 加"编辑"拿着焦点的一段（决定 865）。两个代码提交（6.1）：
  - **提交 1** 留白、两栏、头一行（阅读时的那几项）、内容列与标题、升级横幅、右栏与它的开关、小节、大纲读到的一节，连同 PG17 第 1 格的宽屏部分、up1、`threeBrowsers` 与 `REQUIRED`；
  - **提交 2** 属性条与 `propertyRoles`，编辑时的头一行（状态进"编辑过"的盒子，决定 894）与说明，锁说明，标签组一处（决定 882），连同 PG17 第 1 格的 375 px 一段、第 3 格（头一行的行数，`REQUIRED` 两行）与 pg5 的标题。

  **单独为绿**：提交 1 不碰阅读视图的 HTML 与编辑：编辑时 `PageEditingBar` 照旧在原处（提交 1 的头一行里还没有那两个槽，也没有导出给 `PageEdit` 用的东西，knip 不报），属性表照旧是表；读到属性表的格（pg5:128-129、`page-properties.test.tsx`）不受影响。PG17 第 1 格在提交 1 里只核对宽屏与 480 px 的"编辑过"，375 px 一段（属性条折行）随提交 2 加。提交 2 的 `propertyRoles` 进注册表，它的新格与 13.2 第 23、24 条、12.4 的新文字同 Step；锁说明的名字进 `bdi` 之后以整句文字找它的九处 vitest 同提交改（7.1）。
- **S2 正文的样式**（`m13-p4-s2`，S1 合并之后开）：正文的字号、行高与间距，链接、标签、高亮，表格、行内代码，callout，任务项（决定 872、873）；hljs 的九色与 callout 的语义色（决定 870）；mermaid 的 `base` 主题、系列色与库节点（决定 871、888）；代码块的头：语言名与"复制"（增强 `codeBlocks`，决定 869、890）；PG17 第 2 格；pg5 加一段 Chromium 里手选复制不带头的字。两个代码提交（6.2）：**提交 1** CSS、mermaid、`diagrams.test.ts` 与 `design-tokens.test.ts` 的亮暗成对一格；**提交 2** `codeBlocks`、`patch.ts` 的焦点键、PG17 第 2 格、`REQUIRED` 两行、pg5。**单独为绿**：提交 1 只换样子与 mermaid 的选项，读它们的只有 `diagrams.test.ts`（同提交改）与 pg5:158-163（关键字与代码颜色不同，照旧成立）；代码块的头的 CSS 随提交 2 的增强一起加（提交 1 里没有 `.nw-code-head` 的元素，写了也是死规则）。
- **S3 编辑**（`m13-p4-s3`，S2 合并之后开）：编辑器的比例字体，代码块、表格与 frontmatter 的等宽行（`proseLines`，决定 874）；压淡的符号与标题的字号、wiki 链接与 `==` 的压淡（`markupMarks`，决定 889）；编辑器的链接色与搜索匹配色改用阅读视图的变量，`editor/theme.ts` 不再写色值（决定 875）；编辑时的大纲（决定 876）；"保存"只在需要时显出，显着时焦点在头一行右边一组里就留着，否则 `invisible`、照旧占位（决定 877、885、894、895、896）；冲突、失锁的横幅与补全的弹层换样子（决定 878）；l6 改编辑时的大纲；l5 照旧重量、记下（决定 880、891）。两个代码提交（6.3）：**提交 1** 编辑器的样子与颜色、`proseLines`、`markupMarks`、`design-tokens.test.ts` 的白名单、补全；**提交 2** 编辑时的大纲、"保存"的显出、冲突与失锁的横幅、l6。**单独为绿**：提交 1 不动编辑时的右栏与"保存"；提交 2 的大纲经 `SourceEditor` 的新属性，没有它时编辑器照旧（版本页、对比视图不传）。

为什么是这样分：S1 只动"页的骨架"（两栏、头一行、内容列、右栏）和属性条（00 号文档 4.5.2 本就归 S1），正文的 HTML 照旧；S2 只动阅读视图的 CSS 与两个增强，页的骨架不动；S3 只动编辑器与编辑时的右栏。三块读它们的测试彼此不重叠：S1 是 l6、hi1–hi3、pg5 的标题、`page-panel`、`page-outline`、`page-layout` 与 `upgrade-banner` 的 vitest；S2 是 pg5 的复制、l4、`diagrams.test.ts`、阅读视图增强的 vitest；S3 是 l5、l6 的编辑一段、编辑器与 `page-edit` 的 vitest。编辑时头一行的状态与按钮放在 S1：它们和阅读时的"编辑"在同一行、同一个位置，分开做的话 S1 合并之后编辑时会有两套头。

## 1. 基线

- **外框**：唯一的 `main` 是 `flex min-w-0 flex-1 flex-col p-6`（`W/app/layout.tsx:71`）。细栏（375 px 时）的位置文字是地址里的笔记本名，否则是工作区名（`W/pages/workspace/workspace-layout.tsx:64-69` 的 `ShellPlace`，`W/app/shell-sidebar.tsx:333`），设置模式里是"设置"。升级横幅由 `NotebookLayout` 画在 `Outlet` 之前（`W/pages/notebook/notebook-layout.tsx:86`），是一个 `<output>`（`W/pages/notebook/upgrade-banner.tsx:41`，没有升级时不渲染），占 `main` 的全宽；设置页的内容列是 `mx-auto w-full max-w-178 md:px-2 md:pt-5 md:pb-4`（`W/app/settings-page.tsx:28`）。页的路由 `pages/:pageId` 没有 `handle`（`W/app/routes.tsx:186`）；设置的三个路由带 `handle: { settings: true }`（P3）。
- **页的外框**（`W/pages/page/page-layout.tsx:336-439`，`PageShell`）：`space-y-6` 里先是头（`space-y-2`）：面包屑（`:339`），一行里的 `h1`（`:341-348`，`min-w-0 text-3xl font-semibold break-words outline-none`，`useArrivalFocus`，`tabIndex={-1}`，恢复之后 `aria-describedby` 指恢复的说明）与只在阅读时的按钮（`:349-372`：没删时的 `PageMenu`，能编辑时的"编辑"，`variant="outline"`、进入中 `aria-busy`/`aria-disabled`、闲置退出时 `aria-describedby`）；恢复的 `<output>`（`:374-378`，阅读与编辑时都在）。其下是 `xl:grid xl:grid-cols-[minmax(0,1fr)_15rem]`（`:380`）的两栏：左栏里阅读时依次是拒绝的 `Alert`、闲置退出的 `<output>`、锁说明（`:390-397`，一个 `tabIndex={-1}` 的 `div` 包着 `EditLockNote`）、来源的说明、`AttachmentDrop` 里的 `ReadingView`，编辑时是 `PageEdit`（`:414-426`）；然后子页面与附件一节；右栏 `PagePanel`（`:436`）。`enter()` 先把焦点给 `h1` 再 `setEditing`；回到阅读时焦点给"编辑"或 `h1`（`:309-318`）；Mod+E 的监听（`:319-333`）；角色被拒之后重读、"编辑"没了时焦点给 `h1`（`:289-298`，只在 `roleRead` 的那条路上）。
- **面包屑**（`W/pages/page/breadcrumbs.tsx`）：`BreadcrumbTrail` 是 `nav`（`page.breadcrumb`）里的 `ol.flex.flex-wrap.items-center.gap-x-1.text-sm.text-muted-foreground.[overflow-wrap:anywhere]`，每项一个 `li`，`Separator` 是 `aria-hidden` 的"/"，当前一项 `aria-current="page"`；回收站的预览与变更集页也用 `BreadcrumbTrail`，版本页用 `Breadcrumbs`（带 `part`）。
- **右栏**（`W/pages/page/page-panel.tsx:33-41`）：`<aside aria-label={t("page.panel")} className="min-w-0 space-y-4 xl:sticky xl:top-5 xl:-m-1 xl:max-h-[calc(100svh-2.5rem)] xl:overflow-y-auto xl:p-1 xl:[overflow-anchor:none]">`，大纲只在阅读时（`!editing`），然后反链、属性、版本。小节（`W/pages/page/panel-section.tsx`）是 `<details open className="space-y-2">`，`summary` 是 `cursor-pointer text-sm text-muted-foreground`，里面 `h2.inline.font-medium`；`summaryRef` 是反链与版本的焦点目标。大纲（`W/pages/page/page-outline.tsx`）：`listedUpTo = 1000`（`:13`），`memo` 按笔记本 id 与页比较（`sameView`），标题从视图的 HTML 在 `template` 里读，`li` 以内联的 `paddingLeft` 按级缩进（`:44`），链接是 `Link to={{ hash }}`、`block break-words hover:underline`（`:45`），多出的说"…and N more"（`page.moreHeadings`，`:52`）。反链（`W/pages/page/page-backlinks.tsx:143-187`）是 `ul > li`，名字是带下划线的链接，摘要是 `ul.text-xs` 的每行一项；版本（`W/pages/page/page-versions.tsx`）的 `useVersions`（`:27-35`，键 `["page-versions", 笔记本 id, 页]`）、`Mark`（`rounded border px-1.5 text-xs text-muted-foreground`，历史页也用）、`versionBy`（`:137`，"Ada · Edit"）；属性（`W/pages/page/page-properties.tsx:81-98`）是 `dl aria-label="Properties"` 里每个键值一个 `div > dt + dd`，链接带下划线。
- **阅读视图**：`<article ref={article} aria-label={page.name} className="nw-reading min-w-0" />`（`W/pages/page/reading-view.tsx:187`）。增强的注册表（`W/reading/enhancement.ts:129-140`）依次是 `codeHighlight`、`math`、`diagrams`、`scrollRegions`、`taskToggle`、`appLinks`、`unresolvedLinks`、`assets`、`expiredDownloads`、`titleMedia`。就地更新（`W/pages/page/server-html.ts:90-135`）先记下焦点（`noted`，`:218`，在清理之前）、倒序清理（`:107`）、再按块比对换新（`patchBlocks`）、重跑增强、还焦点（`giveBack`）；焦点的键（`W/reading/patch.ts:353` 的 `keyed`、`:366` 的 `keyOf`）认标题、链接、`summary`、音视频与带 id 的元素。滚动区（`W/reading/scroll-regions.ts`）认 `.nw-scroll, pre, span.nw-math-block`，名字是"表格""属性""公式""图""代码""宽的内容"，不宽时去掉 `role` 与 `aria-label`。高亮（`W/reading/highlight.ts`）认 `pre > code[class^="language-"]:not(.language-mermaid)`，着过色的块记在 `WeakMap`。图（`W/reading/diagrams.ts`）：`options(theme)`（`:75-98`）的 `theme: theme === "dark" ? "dark" : "default"`（`:97`），缓存按主题与源码（`keyOf`，`:219`），包装是 `nw-scroll nw-diagram`（`:158`）。
- **`reading.css`**（504 行）：`:root`/`.dark` 只有 `--nw-mark-background`（`:12-18`）；`.nw-reading` 里有代码块的底、hljs 的十一个变量与标签的两个（`:20-34`，暗色 `:55-70`），行高 1.7、`> * + *` 1rem；h2 有底线（`:87-91`）；任务的复选框（`:180-187`）；`pre`（`:206-213`）；表格（`:248-260`）；属性表（`:263-275`，项目自己的注释说改了 `display` 的表在 WebKit 加 VoiceOver 下会丢语义，`:246-247`）；callout 以 rgb 三元组写每类的颜色（`:279-375`）；hljs（`:434-492`）。
- **编辑**（`W/pages/page/page-edit.tsx`）：视图的 `div`（`:420`）里依次是 `PageEditingBar`（`:422`）或失锁的 `EditLostBanner`（`:429`）、`EditorUploads`（`:431`）、确认框、冲突时的 `ConflictPanel`（`:443`）、`Suspense` 里的 `SourceEditor`（`:451`）；`leave()` 的 `stayed()`（`:257-260`）认 `view.current?.contains(at)`；正文没读到时的分支（`:407-417`）有自己的"完成"；Mod+S、Mod+E 的监听（`:393-405`）。`PageEditingBar`（`W/pages/page/page-editing-bar.tsx`）是一个 `<output>`（`text-sm text-muted-foreground`，失败时 `text-destructive`）、一直在的"保存"（`outline`）与"完成"（等上传时 `aria-busy`）。`PageEditing.unsaved` 是 `version !== savedVersion`（`W/stores/page-editing.ts:135`）。锁说明（`W/pages/page/edit-lock-note.tsx`）只在有人拿着锁时显示（阅读时），是一个 `<output>`，名字不在 `bdi` 里（`:58`）；**编辑时标题下没有任何锁说明**：00 号文档 4.5.1 说"就是现有的锁说明换到这个位置"不对（2.1 第 1 条）。
- **编辑器**：`editorTheme`（`W/editor/theme.ts:10-41`）写死六个 oklch 色值（链接、搜索匹配与当前的匹配，亮暗各三，`:15-17,24-26`），`.cm-focused` 是 2 px 的琥珀外框，`.cm-content` `minHeight: 20rem`、`padding: 1rem 0`、`lineHeight: 1.6`，`.cm-line` `padding: 0 1rem`；正文继承 CodeMirror 默认的等宽。`markdownHighlight`（`W/editor/markdown.ts:13-26`）：标题 1.6/1.4/1.25/1.1em、700，`monospace` 用 `--nw-editor-mono`，链接 `--nw-editor-link`，符号 `--muted-foreground`；`markdownEditing()` 由编辑器与对比视图共用（`W/editor/text-diff.tsx:141,396`）。`SourceEditor` 的扩展（`W/editor/source-editor.tsx:229-266`）、句柄（`:23-42`：`text`、`version`、`focus`、`whenComposed`、`load`、`hold`、`working`、`settled`）、内容的名字"Page content"（`:179`）。`frontmatterAt`、`fenceAt`（`W/editor/loaded/yaml-quotes.ts:231,259`，导出；`opening`、`closing` 不导出）。补全的样子（`W/editor/loaded/link-completion.ts:120-139`）。分包的检查（`web/apps/web/build/out-of-main.ts`）：主包里不能有 `@codemirror`、`@lezer`。`design-tokens.test.ts:283` 的白名单是 `editor/theme.ts` 与 `reading/diagrams.ts`。
- **CodeMirror 的解析**（`@codemirror/language` 6.12.4）：后台解析停在可见范围之后 100,000 个字符（`Work.MaxParseAhead`，`dist/index.js:612`），之后不再自己往下解析；`ensureSyntaxTree(state, upto, timeout)` 在给的时间里接着解析，进度留在解析的上下文里。
- **偏好**（`W/stores/preferences.store.ts`）：`nwiki.theme`、`nwiki.locale`、`nwiki.workspace` 三个键，`read`/`write` 吃掉存储的异常。
- **测试**：vitest 221 个文件、5325 格（P3/S2 合并之前的门禁，`8d98f64f`）。E2E 628 格：624 过、3 败、1 跳，本机约 2.5 min；`git diff --stat 12b0b734 072dcec2 -- web e2e tools Makefile package.json pnpm-lock.yaml server api` 与 `git diff --stat 8d98f64f 12b0b734 -- …` 都是空的：代码与那次门禁相同（决定 883）。已知的本机失败（P2 的基线）：SE2 [chromium] `se2:231`（`Error: expect(locator).toHaveValue(expected) failed`）、LW8 [webkit] `lw8:836`（`Error: expect(locator).toBeFocused() failed`）、PG15 [webkit] `pg15:113`（`Error: expect(received).toEqual(expected) // deep equality`，有时过）；偶发 AS6 `as6:153`（或 `as6:167` 的 "Response has been disposed"）、LW8 `lw8:771`、负载过 100 时 `markdown.test.ts` 的"not slower"、`trash-preview.test.tsx:587`、造数的 API 在负载下答 500（`commit transaction: timeout: context deadline exceeded`）。P4 不改 PG15 与 LW8 的文件，这三格的行号照旧。
- **设计者的探针**：
  - (a) **比例字体加等宽行的长文档**（`p4-design/cm/`：`entry.ts` 照 4.3.3 写一个只看 `visibleRanges` 的 ViewPlugin，给 `FencedCode`、`CodeBlock`、`Table` 的行加类；用网页的 `markdownEditing()` 与 `editorTheme`；`build.mjs` 经网页自己的 Vite 编，`run.mjs` 在 Playwright 1.63 的 Chromium、Firefox、WebKit 里跑）：文档 932,359 个字符、13,200 行（每节一个标题、三段会折行的正文、14 行的 Go 代码块、6 行的表），宿主最宽 720 px，窗口 1280×800，窗口滚动。四种写法：`prose`（全是比例字体，15.5 px、行高 1.85）、`mono-relative`（等宽行 13.5 px、行高随之是 1.85 倍）、`mono-absolute`（等宽行的行高写死成比例字体的那一行高）、`mono-only`（全是等宽，今天的编辑器）。结果：先以 `ensureSyntaxTree` 50 ms 一片把整篇解析完，三种引擎 44–81 ms；滚到 10%、30%、50%、70%、90% 等七处（像拖滚动条），视口中间那一行在之后 8 帧里都没动（三种引擎、四种写法都是 0 px）；`scrollIntoView` 远处的三行，落在 5–6 px（同上）；没量过的行的高度估计比量过之后的总高度：`prose` 少 17–21%，`mono-relative` 少 10.5–11%，`mono-absolute` 少 24%，`mono-only` 少 10.5–12.4%。所以取 `mono-relative`：估计的偏差与今天的编辑器相同，跳动没有出现，00 号文档第 10 节的回退（等宽行只给代码块）不需要（决定 874）。
  - (b) **静态页**（`p4-design/static/`：`page.html` 照第 4 节的类名写页的两栏、头一行、内容列、属性条、正文、代码块与它的头、右栏；`reading.css` 是草拟的新样式；Tailwind 4.3.3 的 `compile` 编出 `out.css`；`measure.mjs` 在三种引擎里量）：1440 宽时头一行 x=292、宽 856、高 36，标题 x=360、宽 720；右栏 x=1188、宽 252、高等于窗口、顶在 0，窗口滚到 1200 与到底时仍顶在 0；1280 宽时右栏 x=1028，标题宽 696（内容区不够 720）；1024 宽时右栏在正文之后、宽 720，开关不在；375 宽时头一行折成两行（高 75），`scrollWidth` 375 不溢出；代码块横向滚动 200 px 之后头的左边仍是 `pre` 的左边加 1（三种引擎）；属性条带一个 97 个字符不断开的值时 `scrollWidth` 等于 `clientWidth`；已完成的任务项划线、变淡，嵌套的子项是 `inline-block`、正文色；正文行高 28.8 px，h2 21 px。不可用的已勾选复选框：Chromium 与 Firefox 画成灰的，WebKit 画成淡的琥珀（决定 872）。Chromium 的强制色模拟下，大纲的当前项以下划线标出，大纲左边的线可见，没有透明的竖条。两处要留意的写法：`line-clamp-2` 与 `block` 同写时 `block` 盖掉它的 `display`，大纲的链接不写 `block`；开关的 `hidden xl:inline-flex` 经 `cn` 合并，`Button` 自己的 `inline-flex` 被它盖掉（静态页上两者同写时窄屏也显示，`cn` 不会）。
  - (c) **类名**：静态页的 152 个词经 (b) 的编译都生成了（`language-go` 是服务端写的类，不是 Tailwind 的）；写完本文之后又从本文的 `className` 与类名串里抽出全部的词再编一次，也都生成了；`group-not-open:-rotate-90`、`xl:data-[open=false]:hidden`、`forced-colors:border-l-0`、`max-xl:max-w-180`、`text-[12.5px]`、`font-[650]`、`tracking-[-0.012em]` 都在。
  - 修订时（照审查记录）另跑的：
  - (d) **mermaid**（`p4-design/mermaid/`：`all.html` 是审查 B 的 14 张图加 14 张：流程图里各种圆柱、`style` 与 `classDef` 着色的圆柱、ER 带属性、类图带注、状态图带选择与注、需求、看板、架构、块图、C4、桑基、包、雷达、九块的饼、五条分支的 git 图；另有 `shapes.html` 一张流程图排了 40 个节点、37 种形状，`dump.mjs` 列出每个节点的元素与类；`draw2.js` 照 `diagrams.ts` 的选项加 `vars.js` 的变量与 `themeCSS` 画，`shootall.mjs` 在三种引擎里亮暗各画一遍、逐张截图并列出每张图里两个选择器命中的节点，`sheet.mjs` 拼成对照图看）：`base` 主题的 `Theme.calculate` 先赋值、再派生、再赋一次给的值，给的 `pie*`、`git*`、`cScale*` 原样用；对象形的 `xyChart`、`radar` 给了就整个代替，只给 `plotColorPalette` 时其余的颜色丢了（暗色画成白底）；`cScaleLabel*` 还是时间线主线的颜色、`cScaleInv*` 是它的竖线，甘特的网格线带 `stroke="currentColor"` 的属性、随页面的字色（亮色画成黑线），ER 的圈是写死的 `fill="white"`，`rowOdd`/`rowEven` 在 `darkMode` 下是近黑：这几处由 `themeVariables` 与一段按主题的 `themeCSS` 补上（决定 888，4.2.6）。选择器 `.node > path.basic.label-container.outer-path` 在 28 张图里只命中圆柱：流程图的 `[(…)]`、`cyl`、`h-cyl`、`lin-cyl` 与块图的 `[("…")]`，`shapes.html` 里别的形状与 28 张里别的 24 张图一个不中；`style`、`classDef` 着色的圆柱照旧是作者的颜色（内联样式盖过 `themeCSS`）。`.node > rect.basic.label-container` 的 9 px 圆角落在流程图的矩形与圆角矩形、状态图的状态、没有属性的 ER 实体、看板的卡片、块图的块上，C4 照旧。`themeCSS` 经 mermaid 以 SVG 的 id 限定（`#nw_mermaid_N …`），特指度高过它自己的 `#… .node path`，不用 `!important`。WebKit 认 CSS 的 `rx`。看过的对照图：饼、git、甘特、时间线、思维导图、xychart、象限、旅程、雷达、架构、ER、类、状态、时序、流程（含样稿的四个节点），亮暗都不再有黑块、白块或同色的两块。
  - (e) **编辑器的压淡**（`p4-design/cm2/`：网页的 `markdown.ts` 一份副本改了 `HighlightStyle`，`entry.ts` 是 4.3.3 的 `markupMarks` 草稿，经网页自己的 Vite 编，`run.mjs` 在三种引擎里把每一行的每个字画成"下划线、淡、都不是"）：lezer 把 `[[页]]` 读成 `[`、链接 `[页]`、`]`，高亮的 `span` 是扁平的（一个字一个 `span`，类叠在一起），装饰的 `span` 与它们互相嵌套；所以下划线不放在 `tags.link` 上，只由装饰给页名与 `[文字](地址)` 的文字，淡的装饰连它里面的高亮一起压淡（`.nw-md-faint, .nw-md-faint *`）。三种引擎画出的完全一样：`[[部署指南]]`、`[[页面#小节|别名]]`、`[[#小节]]`、`![[image.png|100]]` 的括号与 `#`、`|`、`!` 淡，页名（没有页名时 `#` 后的标题）带下划线；`[文字](https://…)` 的括号与地址淡、文字带下划线；`[ref]` 只有括号淡；`==…==` 两端淡、`a == b` 不动；行内代码、围栏代码块与缩进代码块里的一个不动；frontmatter 整块是次要色、不画下划线。8,000 行的文档里装饰只在可见的 441 个字符里（66 个）。
  - (f) **主题的次序**（同上，另有 `jsdom-order.mjs` 在 jsdom 30.1.1 里挂 CodeMirror）：`proseTheme` 在 `editorTheme` 之后时 `.cm-content` 行高 24.8 px、上内边距 16 px、`.cm-line` 左内边距 16 px；在它之前或 `Prec.high(proseTheme)` 时 28.675 px（Firefox 28.683）、22、26，三种引擎一样；jsdom 的 `getComputedStyle` 也照次序答 `1.6`/`1.85`、`16px`/`26px`（CodeMirror 在 jsdom 里写 `<style>`，jsdom 按样式表的先后叠），所以 vitest 能红。
  - (g) **按钮的焦点**（`p4-design/focus/`）：Chromium、Firefox 里点按钮，按钮拿到焦点；WebKit 里点按钮不给它焦点，原先在可编辑区里的焦点落到 `body`，Tab 也不走到按钮。三种引擎里焦点从按钮移走时，`focusout` 那一刻的 `document.activeElement` 都是 `body`（窗口失去焦点时照规范它仍是那个按钮，无头的引擎里模拟不了）。
  - (h) **右栏的读者**（`p4-design/rail/`，P2 的 `watchReader` 原样抄进页面）：`watchReader({ on: 大纲的列表, within: "[data-page-panel]" })` 在三种引擎里：在右栏上滚轮、焦点在右栏里按方向键算读者动过；点大纲的项、在它上面按 Enter 不算；在正文上滚轮不算。无头的 macOS 引擎的滚动条是浮动的（宽 0），按右栏滚动条那一种没探到，照 P2 的说法（Chromium、WebKit 里算，Firefox 里不发事件）。
  - (i) **头一行的预留**（`p4-design/headrow/`，照审查 A 的 `headrow.mjs` 改）：头一行宽 327、480、552、696 px，三个长短不同的名字：今天的写法（读到之前没有那一句、右边一组折行）在 327 px 的长名字与 480、552 px 的每个名字下，"编辑"随那一句到来下移 19–36 px；"编辑过"的盒子一开始就在（`w-64`、可缩、省略、字靠右）、右边一组不折行时，三种引擎里"编辑"在到来前后一个像素不动（决定 858）。
  - 再修订时（照修订核对第 1 轮）另跑的：
  - (j) **头一行的定宽**（`p4-design/head894/head.mjs`，照决定 894 的写法以纯 CSS 抄 4.1.3：右边一组 `flex: 0 1 26rem`，第一个盒子 `flex: 1 1 0; min-width: 0`，"保存"不需要时 `visibility: hidden`）：行宽 327（375 宽的窗口）、519（820 宽，左栏 253 px）、600、651、653、724（1024 宽）、900（1440 宽，带开关），五种状态（阅读、"编辑过"到来之后、刚进编辑、打了一个字、自动保存之后），名字取 PG17 造数的那种约 50 个字符的：三种引擎里每个行宽下五种状态的标题 `y` 一样（≤ 651 px 两行、≥ 653 px 一行），编辑时状态的右边与"完成"的 `x` 一样，行不溢出，藏起的"保存"`focus()` 拿不到焦点。对照：修订稿的写法（审查 A 的 `headmodes.mjs`）在 519–610 px 阅读两行、编辑一行，327 px 时打第一个字变两行。
  - (k) **`useCurrentHeading` 与 vitest 的 `act`**（`072dcec2` 的分离工作树，用完即删）：把修订稿 4.1.7 的钩子原样放进 `page-outline.tsx`，全部 vitest（222 个文件）跑三遍，`src/app/arrival.test.tsx` 的"the first title shown for a navigation takes the focus on the tree's link"两遍红在 `console.error`："The current testing environment is not configured to support act(...)"，单独跑那个文件四遍都过：时有时无。原因：这个项目的 vitest 不开全局，Testing Library 不设 `IS_REACT_ACT_ENVIRONMENT`，平时 `act` 之外的更新不报；但那一格 `await act(async () => (await treeLink("Notes")).focus())` 在 `act` 里等 `findBy`，`waitFor` 那时把环境设成假，钩子第一帧的 `setCurrent` 正好落在这个窗口里，React 就报这一句，`setup.ts` 把它当失败。改成 4.1.7 的写法（第一次量在 `useLayoutEffect` 里同步做，之后只在值变了时设，点大纲的项时在点按里设）之后，全部 vitest 跑三遍都绿，挂上之后立刻读得到当前项；假计时下推进计时也不报。别的有标题的页的格（`src/pages/page` 的 25 个文件、574 格）两种写法下都过。
  - (l) **大纲晚到时的 `watchReader`**（同一个工作树）：扣住视图的 `GET`，页的外框先挂上、大纲是 `null`，放行之后大纲出现；给右栏与项、标题的盒子打桩，点大纲的"H1"（`user.click`：`pointerdown` 加 `click`），再让读到的一节变到第 32 项：修订稿的写法（`[page]` 的 effect 里建，`on` 是 `null`）右栏的 `scrollTop` 停在 0（点那一下被算成读者动过，审查 A 的 R1-I2）；列表的回调 ref 里建时它滚到 624（刚好露出第 32 项）。点的那一项在它进视口之后才算"量到过"，出了视口才解除（审查 A 的 R1-M1）也照写跑过。
  - (m) **PG17 的造数**（`p4-design/pg17geo/geo.mjs`：40 节，`Part 01`–`Part 38` 各 12 段、`Part 39`、`Part 40` 各 2 段，`Part 20` 末尾一个以标题开头的折叠 callout；正文之后 184 px 代替附件一节与页底；S1 的样式是 `072dcec2` 的 `reading.css`，S2 的是审查 B 照 4.2.3–4.2.7 抄的那份；1440×900）：一节 579–580 px（S1）、624 px（S2）；"Hidden"的顶在线之上 5 px 时（295），`Part 21` 在 327（S1）、342（S2），都在线下；`Part 38` 的顶能滚到 0（它到文档末尾 1,058、1,110 px）；`Part 39` 滚到 16 px 时窗口已到底，它停在 414–421 px、`Part 40` 在 568–570 px，都在视口里（`Part 39` 到末尾 479–486 px）。三种引擎相差不超过 1 px。
  - 三修时（照修订核对第 2 轮）另跑的：
  - (n) **右边一组的最小高**（`p4-design/head896/minh.mjs`，照 4.1.3 以纯 CSS 抄，右边一组加 `min-height: 32px`，与不加的对照）：行宽 327、519、651、653、724、900 px，十九种状态（阅读、"编辑过"到来之后、组里一个按钮也没有——内容读到之前、失锁或读取失败，三者在头一行里一样——、刚进编辑、未保存、已保存，中英文的冲突、繁忙、等上传、连不上、太慢，英文的网络与 NUL）：加了之后，三种引擎里每个行宽下标题的 `y` 都一样（≤ 651 px 是 84，≥ 653 px 是 66），两行的状态（327 px 的中文"网络太慢……""暂时连不上服务器……"，327–651 px 的英文长说明）也一样；只有 327 px（375 宽的窗口）的英文"too slow""NUL""Cannot reach the server for now…"是三行，标题在 100。不加时组里没有按钮的那几种是 66（≤ 651 px，阅读与编辑是 82），两行的状态是 84：与审查 A 的 `head894b.mjs`、`head894c.mjs` 和审查 B 的 R2-N1 相同。
  - 没探的：应用里的整套行为（Step 的代码没有写）；(j)、(n) 是纯 CSS 的抄写，不是应用；三修改的 `useCurrentHeading`（`live`、收起时结束 `pin`）与 `held` 的写法没有在 vitest 里跑（(k)、(l) 跑的是再修订稿的那一份）。

## 2. 目标与范围

做 00 号文档 4.5 的阅读与编辑：页面的头一行、属性条、可收起的右栏与大纲读到的一节；正文的样式、代码块的头、hljs 与 mermaid 的配色、任务项；编辑器的字体、等宽行、编辑时的大纲与只在需要时显出的"保存"。另做 P3 交过来的三件：`main` 的留白、细栏的位置文字（决定不换，856）、升级横幅与内容列对齐。不做：版本页、回收站的预览、变更集页的头（P5；它们的面包屑随 `BreadcrumbTrail` 换了样子，第 9 节）；页的头一行之外的页（首页、列表类页面，P5）；空状态（P5）；M12-polish 里没认领的项（决定 881）。

### 2.1 对 00 号文档的修订

下面各条由主持者随本文的设计提交照这里的文字落进 [00 号文档](00-M13-design.md)（设计者只写了本文）：脚本依次做这些替换，断言每段旧文字在做到它时的 00 号文档里恰好出现一次、每段新文字在它那一步之后也恰好一次（设计者已用同样的脚本在 `072dcec2` 上依次做过这十五处，每段都是一次；脚本在会话的临时目录，不进仓库）。"换成"是旧文字换成新文字，"之后加"是新文字接在旧文字之后。00 号文档的行号照它的头是 `3c315037` 的，不改（今天的在第 1 节）。头的状态、第 12 节 P4 一行与第 13 节的一行是设计提交的记录，同 P3。

1. **4.5.1 编辑时标题下的一句**（决定 867、868）：旧文字

   > 编辑时标题下是一行灰字的锁说明（"你正在编辑，别人此时只能阅读这一页"，就是现有的锁说明换到这个位置）。

   换成：

   > 编辑时标题下是一行灰字的说明（"你正在编辑，别人此时只能阅读这一页"）：这是新的一句（`page.editingNote`），不是挪来的；现有的锁说明（`W/pages/page/edit-lock-note.tsx`）只在阅读时、有人拿着锁时显示，留在原处，只换样子（[04-P4](04-P4-reading.md) 决定 867、868）。

2. **4.5.4 大纲的当前节"只观察"**（决定 861、886、887、893）：旧文字

   > 只观察大纲列出的那些标题（至多 `listedUpTo` = 1000 个，`page-outline.tsx:13`；l6 的大页有 14 万个标题）；

   换成：

   > 只量大纲列出的那些标题（至多 `listedUpTo` = 1000 个，`page-outline.tsx:13`；l6 的大页有 14 万个标题）：窗口滚动时（被动的监听，一帧至多一次）与正文的尺寸变了时（`ResizeObserver`）量它们的位置，取视口上三分之一处之上的最后一个；关着的 `details`（折叠的 callout）里的标题不算，大纲照旧列出它们；文档能滚、又已滚到底时，线是视口的底；点了大纲的一项、它还在视口里时先标它，直到读者再动；右栏收起或不在正文旁边（窄于 1280 px）时不量；不用 `IntersectionObserver`（一下跳过一段时没有标题穿过观察的带，当前项不变；[04-P4](04-P4-reading.md) 决定 861、886、887、893）；

3. **4.5.5 编辑时的大纲"语法树解析完成时再刷新一次"**（决定 876）：旧文字

   > 停手之后（去抖）再算，语法树解析完成时再刷新一次（长文档的树起初只覆盖解析过的部分；主持者的决定 745）。

   换成：

   > 进编辑时先算一次，之后停手（去抖）再算。CodeMirror 自己的后台解析停在可见范围之后 10 万个字符（`@codemirror/language` 的 `MaxParseAhead`），不会自己"完成"，所以大纲以 20 ms 一片补完解析（`ensureSyntaxTree`），补完再算；补的途中文档又变了才重来，只换了选区或别的状态时接着补（主持者的决定 745；[04-P4](04-P4-reading.md) 决定 876）。

4. **第 7 节 P4 的三行**（决定 854、879）：旧文字

   > | **P4 阅读与编辑** | S1 页头、属性条与右栏 | 4.5.1、4.5.2、4.5.4 | 属性条、右栏收起、大纲当前节的 vitest；l6、hi1–hi3、pg5 |

   换成：

   > | **P4 阅读与编辑** | S1 页头、属性条与右栏 | 4.5.1、4.5.2、4.5.4；`main` 的留白、升级横幅进内容列、编辑时头一行的状态与按钮、标题下的编辑说明（[04-P4](04-P4-reading.md) 决定 854） | 属性条、右栏收起、大纲当前节的 vitest；l6、hi1–hi3、pg5；新故事 PG17 第 1 格 |

   旧文字

   > | | S2 正文的样式 | 4.5.3（含复制按钮、hljs、mermaid 主题、任务项） | 阅读视图增强的测试；PG15 等就地更新的故事；l4 |

   换成：

   > | | S2 正文的样式 | 4.5.3（含复制按钮、hljs、mermaid 主题、任务项） | 阅读视图增强的测试；PG15 等就地更新的故事；l4；PG17 第 2 格 |

   旧文字

   > | | S3 编辑 | 4.5.5 | 编辑器的 vitest；编辑、冲突、锁、补全（l5）的故事 |

   换成：

   > | | S3 编辑 | 4.5.5 | 编辑器的 vitest；编辑、冲突、锁、补全（l5）的故事；l6 的编辑时的大纲 |

5. **4.8 的新故事**（决定 879）：旧文字

   > PG16 只核对菜单的行为（取消，不写），接口与 MCP 版本豁免。

   之后加：

   > P4 另加 PG17（`e2e/stories/page/pg17-page-head.spec.ts`，两格，S1、S2 各一，三种浏览器）：页的头一行（"谁、何时编辑过"晚到时"编辑"不动）、宽屏的右栏（大纲读到的一节、折叠的 callout 里的标题不算、右栏只滚到刚好露出、读者在右栏里滚过之后不再跟、收起与记住、锚定）、375 px 的属性条，与代码块的头（语言名、"复制"、横向滚动时头不动）；进 `threeBrowsers` 与 `check.mjs` 的必需格（每格 firefox、webkit 两行）；只核对界面的行为，接口与 MCP 版本豁免。"编辑"拿着焦点、读者读到页中间时页变得不能编辑，焦点到标题而窗口不动，由 UP1 核对（三种浏览器）（[04-P4](04-P4-reading.md) 决定 865、879）。

6. **第 10 节"编辑器混排"一行的应对**（决定 874）：旧文字

   > P4/S3 用 M12/P4/S5 的大文档探查；不行时等宽行只给代码块

   换成：

   > P4/S3 用 M12/P4/S5 的大文档探查；不行时等宽行只给代码块。P4 的设计已在三种引擎里探过（[04-P4](04-P4-reading.md) 第 1 节 (a)：93 万个字符，等宽行的行高随字号，跳动 0 px，高度估计的偏差与今天的等宽编辑器相同），回退不需要

7. **第 10 节"滚动锚定"一行的应对**（决定 879）：旧文字

   > P4/S1 在它自己的页上于三种引擎里再核对一次锚定与锚点跳转

   换成：

   > P4/S1 在它自己的页上于三种引擎里再核对一次锚定与锚点跳转（PG17 第 1 格：大纲点到的标题落在 16 px、读者之上就地更新之后读的那一段不动，[04-P4](04-P4-reading.md) 5.1）

8. **4.5.5 "保存"拿着焦点时**（决定 885）：旧文字

   > 它拿着焦点时消失（按了它、或停在它上面时自动保存完成），焦点交给编辑器，不落到 `body`（`useFocusLeaving`，主持者的决定 745）。

   换成：

   > 它拿着焦点时不消失：有未保存的修改时显出，显着的时候焦点在头一行右边那一组里（状态、"保存"、开关、"完成"），它就一直留着，焦点离开那一组、又没有未保存的修改时才隐去；没有修改时焦点走进那一组，藏着的它不显出；不显出时它照旧占着位置、不可聚焦，头一行的行数不随它变（状态因此离开关约 70 px，样稿是 8 px，记成偏离）；拿着焦点、没东西可存时按它不做事（保存失败一定伴随未保存的修改）；窗口失焦（换窗口、换应用）不算焦点离开。原来"它拿着焦点时消失、焦点交给编辑器"（主持者的决定 745）让键盘用户刚到它上面就赶上自动保存完成，接下来按的 Enter 或空格打进了正文（[04-P4](04-P4-reading.md) 决定 885、894、895、896 改判）。

9. **第 11 节决定 745 的"保存"一层**（决定 885）：旧文字

   > 编辑的"保存"拿着焦点消失时焦点给编辑器（RB3）

   换成：

   > 编辑的"保存"拿着焦点消失时焦点给编辑器（RB3；[04-P4](04-P4-reading.md) 决定 885 改为它拿着焦点时不消失）

10. **4.1 `--accent` 的用处**（决定 888）：旧文字

    > 焦点圈、"当前"徽记的字。没有琥珀实心底上的字

    换成：

    > 焦点圈、"当前"徽记的字、编辑器的光标、图里的库节点与注（mermaid 流程图与块图的圆柱、时序图等的 `note`：琥珀边、淡琥珀底）、甘特图的今天线与标记线（[04-P4](04-P4-reading.md) 决定 871、874、888）。没有琥珀实心底上的字

11. **第 8 节的注册**（决定 864）：旧文字

    > - **注册**：阅读视图的交互增强（12.4，建立于 M4）注册"代码块的复制按钮"。

    换成：

    > - **注册**：阅读视图的交互增强（12.4，建立于 M4）注册"属性条的角色"（`propertyRoles`，P4/S1）与"代码块的复制按钮"（`codeBlocks`，P4/S2；[04-P4](04-P4-reading.md) 决定 864、869）。

12. **4.5.4 右栏只滚到刚好露出**（决定 886）：旧文字

    > 要让当前项可见时只动右栏自己的 `scrollTop`（`W/pages/page/page-panel.tsx:35`），从不调 `scrollIntoView`（13.2 第 26 条）。

    换成：

    > 要让当前项可见时只动右栏自己的 `scrollTop`（`W/pages/page/page-panel.tsx:35`），只滚到它刚好露出；读者在右栏里动过（滚轮、触摸、按下、按键）之后不再跟，到下一次换页为止（`watchReader({ within })`，同 [02-P2](02-P2-shell.md) 决定 779、791 对左栏的规则；[04-P4](04-P4-reading.md) 决定 886）；从不调 `scrollIntoView`（13.2 第 26 条）。

13. **第 7 节"总体设计的修订"第 3 项**（审查 C 的 R1-N3；8.2 第 9 条）：旧文字

    > 3. 12.4 扩展点表"阅读视图的交互增强"一行的注册者加"M13（代码块的复制按钮）"。

    换成：

    > 3. 12.4 扩展点表"阅读视图的交互增强"一行的注册者加"M13（属性条的读屏角色、代码块的复制按钮）"，同时补上漏记的"M12（过期的地址重签之后下载、标题里的音视频不开合 callout）"（[04-P4](04-P4-reading.md) 8.2 第 9 条）。

### 2.2 需要负责人确认

（无）

### 2.3 本文的决定（主持者的决定，负责人可以改判）

854–884 是初稿的；修订时照设计审查改了其中 857–861、863、865、866、868–877、880、881 的文字（各条里写明依据）。885–893 是主持者照设计审查定的（[审查记录](reviews/P4-design-review.md)"主持者的决定"），894、895 是照修订核对第 1 轮定的（同上，"主持者的决定（第 1 轮）"），写法由设计者补在 4.x。再修订时照第 1 轮又改了 858、859、861、863、866、871、874、876、877、881、885、888 的文字。896 与改过的 895 是照修订核对第 2 轮定的（同上，"主持者的决定（第 2 轮）"）；三修时照第 2 轮又改了 861、863、866、871、877、885、894 的文字。897–900 是 S1 的实施审查之后的（[S1 的审查记录](reviews/P4-S1-head-strip-panel-review.md)"主持者的决定"，括注是发现的编号）；照 S1 的实施与审查又改了 861、879、886 的文字。901–906 是 S2 的实施审查之后的（[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)"主持者的决定"，括注是发现的编号）；照 S2 的实施与审查又改了 869、871、872 的文字。907–911 是 S3 的实施审查之后的（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)"主持者的决定"，括注是发现的编号）；照 S3 的实施与审查又改了 874–876、878、889 的文字。

- **854 Step 的分法**：照 00 号文档第 7 节的三个 Step，00 号文档没分到的四件放进 S1：`main` 的留白（P3 交来，855）、升级横幅进内容列（P3 交来，857）、编辑时头一行的状态与按钮（866，它和"编辑"同一行）、标题下的编辑说明（867）。每个 Step 两个代码提交加一个文档提交（第 6 节）；理由与"单独为绿"见第 0 节。
- **855 `main` 的留白**：`W/app/layout.tsx:71` 的 `p-6` 换成 `p-6 xl:px-10`（样稿 `.content` 左右 40 px）；页的外框另加 `md:-mt-3 md:pb-4`，凑出样稿头一行离顶 12 px、页底 40 px（`main` 的 24 px 加减）。所有页在 1280 px 起左右多 16 px 的留白，是有意的（P5 的页照这个留白排）；设置页的内容列居中、最宽 696 px，1280 px 起照样居中，`SettingsPage` 的类不改（`md:px-2` 在 768–1279 px 之间照旧凑出 32 px），只改它的注释。
- **856 细栏的位置文字不换**：P3 交来"换成页的标题"一问，取不换：375 宽时细栏之下紧接着就是面包屑与 32 px 的标题，细栏再写页名是重复；笔记本名说的是"在哪里"，面包屑的第一项也是它，与 P2 的决定 784 一致。设置模式里照旧是"设置"。
- **857 升级横幅进内容列**：新文件 `W/app/column-top.tsx`：`ColumnTopContext`（`createContext<ReactNode>(null)`）与 `ColumnTop`（渲染上下文里的元素）。`NotebookLayout` 读 `useMatches()`：命中的路由带 `handle.column` 或 `handle.settings` 时不在 `Outlet` 之前画横幅，而是以 `<ColumnTopContext value={<UpgradeBanner notebook={notebook} />}>` 包住 `Outlet`；页的路由 `pages/:pageId` 加 `handle: { column: true }`。`PageShell` 在内容列的最上面（标题之前）、`PageLayout` 的另外两个分支（`NotLoaded`：读取中与读取失败；`PageGone`：找不到）的最上面、`SettingsPage` 的标题之前、`NotebookSettingsLayout` 不画 `SettingsPage` 的那一支（`shown === undefined`，决定 892）的 `Outlet` 之前画 `<ColumnTop />`。在渲染时判定，同 P3 的设置模式（决定 811），不闪；`ColumnTop` 是单独的组件，上下文变了只有它重渲染，阅读视图不动（`upgrade-banner.test.tsx:97` 的"页照旧"成立）。别的笔记本页（首页、版本页、回收站、历史、目录……）照旧画在 `main` 的全宽，P5 照它们的布局再定。横幅自己的样子不改。
- **858 头的"谁、何时编辑过"**：`useVersions(notebook, page.id)` 的第一页的第一项（与右栏的版本同键，SWR 只读一次），没读到或没有版本时不显示。文字新键 `page.editedBy`：英文"Edited by {name} {when}"（"Edited by Ada yesterday"），中文"{name} 编辑于{when}"（"Ada 编辑于昨天"，同样稿）；`name` 是 `changeset.created_by.display_name`，经 `isolate`/`Isolated` 进 `bdi`；`when` 是 `formatRelative(updated_at)`；整句的 `title` 是 `formatDateTime(updated_at)`。是 `<p>`，不是 `<output>`：读屏不在每次读到时播报它，编辑时头一行的状态与 `editStatus`（`e2e/fixtures/wiki-pages.ts:233-235`，`main` 里唯一的 `status`）不受打扰。只在阅读时显示，在头一行右边一组的第一个盒子里（决定 894）：那个盒子一直在、`min-w-0 flex-1`、字靠右，阅读时是这一句（`truncate`，名字长时省略），编辑时是保存的状态；版本读到之前、没有版本时它是空的。右边一组定宽（`flex-[0_1_26rem]`，窄时缩到行宽），它到来时头一行的行数与"编辑"的位置都不变（初稿说"它出现时别的不动"，在 375–696 px 宽时不成立：右边一组随它折行，"编辑"下移 19–36 px，页在顶部时没有滚动锚定可依，审查 A 的 M4；修订稿的 `w-64` 预留只管阅读，进出编辑时右边一组的宽仍随模式变，审查 A 的 R1-I1）。相对时间不随时间刷新（M5-polish 的已知限制，照旧）。
- **859 右栏的开关**：`Button variant="ghost" size="icon-sm" shape="pill"`，图标 `PanelRight`，`aria-label={t("page.panel")}`（"About this page"/"页面信息"，与右栏同名），`aria-expanded`，`aria-controls` 指右栏的 `id`（`useId`），`className="hidden xl:inline-flex"`：只在并排（1280 px 起）时出现。状态存在 `PreferencesStore` 的 `panelOpen`（键 `nwiki.panel`，值 `"open"`/`"closed"`，没存过或读不到时展开），`setPanelOpen(open)` 写进去；读写照旧经 `read`/`write` 吃掉异常。右栏带 `data-open`，收起时 `xl:data-[open=false]:hidden`（更窄时不认它：右栏照旧在正文之后）；收起时内容区不再留右边的 40 px（`xl:pe-10` 只在展开时），正文那一列在内容区里居中，最宽仍是 720 px（窗口不到 1304 px 时展开的右栏让它窄于 720 px，收起之后放宽到 720 px；审查 A 的 N3）。点了之后焦点留在开关上；右栏藏起时（收起，或窗口变宽跨过 1280 px、偏好是收起）焦点在右栏里的，交给开关（`focus({ preventScroll: true })`，WebKit 里点开关不给它焦点、焦点照旧在右栏里；审查 A 的 N1）。窗口变宽那一支照这里的写法（layout effect 里看焦点在不在右栏）在 Chromium 里落空：它在重算样式时就把藏起的元素里的焦点修到 `body`，早于 `matchMedia` 的 `change` 与之后的 layout effect，那时焦点留在 `body`；Firefox、WebKit 里焦点到开关。在右栏的 `focusout` 里看 `relatedTarget === null && !aside.checkVisibility()`，三种引擎都做得到（审查 A 的 R3-N1 的探针）；焦点在开关上时窗口变窄到 1280 px 以下，开关 `display: none`，三种引擎都把焦点修到 `body`，同样的写法也能把它交给标题。两处都要在焦点停在那里时改窗口的宽，P4 不另修，照实记下（审查 A 的 R1-N4、R3-N1；主持者定）。`PanelToggle` 是 `observer`，自己读 `preferences.panelOpen`（审查 A 的 N2）。阅读与编辑时都有。名字不是"Edit"：lw5 数整页名为"Edit"的按钮。
- **860 右栏的样子与位置**：页的外框是 `md:-mt-3 md:pb-4 xl:flex xl:items-start`，两个子元素：内容区 `min-w-0 xl:flex-1`（展开时加 `xl:pe-10`）与右栏 `<aside id aria-label data-open data-page-panel className="min-w-0 space-y-6 max-xl:mx-auto max-xl:mt-10 max-xl:max-w-180 xl:sticky xl:top-0 xl:-me-10 xl:-mt-3 xl:-mb-10 xl:h-svh xl:w-63 xl:shrink-0 xl:overflow-y-auto xl:border-l xl:border-border xl:pt-16 xl:pr-4 xl:pb-5 xl:pl-4.5 xl:[overflow-anchor:none] xl:data-[open=false]:hidden">`：负的外边距抵掉 `main` 的右边与上下的留白，宽屏上右栏贴着窗口的右边、从顶到底（样稿 `.panel`：左边一条线，内边距 64/16/20/18 px，宽 252 px），粘住、自己滚动，照旧不作窗口滚动的锚点。更窄时排在正文之后，与正文同宽、居中。静态页的探针（第 1 节 (b)）量过 1440、1280、1024 三种宽度。`data-page-panel` 是 `watchReader` 的 `within`（决定 886；`useId` 的 id 带冒号，不宜写进选择器）。
- **861 大纲读到的一节**：新文件 `W/pages/page/current-heading.ts` 的 `useCurrentHeading(ids, active)`，`PageOutline` 用它（4.1.7）。`ids` 只是大纲列出的那些（至多 `listedUpTo`；审查 C 的 M10）；`active` 是右栏展开、又在正文旁边（`useBeside()`，`(width >= 80rem)`；决定 886），不是时、或大纲没有列出标题时（审查 A 的 P4S1A-N1），不监听、不量。窗口的 `scroll`（被动）与 `resize` 的监听、正文的 `ResizeObserver`（`typeof ResizeObserver === "undefined"` 时不用：jsdom 没有它），都只排一个 `requestAnimationFrame`，一帧至多算一次；第一次（挂上、`ids` 或 `active` 变了）在 `useLayoutEffect` 里同步量，之后的每次只在答的值变了时才设状态（第 1 节 (k)：照修订稿在第一帧才设，vitest 里会落进别的格的 `act` 里、让 `setup.ts` 判红；在 layout effect 里量，第一次就在那次提交里，之后在 jsdom 里值不再变，不再有更新）；`active` 变假时清掉当前项、结束点了的那一项，再展开的那一次渲染不答收起之前的那一项；不活跃时点大纲不 `pin`（窄于 1280 px 时大纲在正文之后、点得到；审查 A 的 R1-N5、R2-N2）。`PageOutline` 的 `memo` 比较（`sameView`）加上 `active`：只比笔记本与页时 `active` 的变化传不进来，存着收起时打开页、展开之后读到的一节永远不标、右栏也不跟，收起之后又照旧每次滚动都量（审查 A 的 R2-I1）。算法：线是视口高度的三分之一；文档能滚（`scrollHeight > innerHeight + 1`）、又已滚到底时（`scrollY + innerHeight >= scrollHeight - 1`）线是视口的底（决定 893）；从列出的最后一个标题往前找，第一个不在关着的 `details` 里（`closest("details:not([open])")`，决定 887：关着的 `<details>` 里的标题在三种引擎里都答一个不是 0 的盒子，初稿的"盒子是 0"不成立）且顶不低于线的，就是读到的一节；一个也没有时没有当前项。点了大纲的一项（不带修饰键的主键点按，同 React Router 的 `isModifiedEvent`：带 Cmd、Ctrl、Shift、Alt 的点按开新标签页、新窗口或下载，视图不滚，不算；审查 A 的 R2-N1），在点按里就标它（`pin(id)` 直接设当前项），视图把它滚进视口之前一直标它，进过视口之后到它出了视口为止，读者一动（`watchReader({ on: 那个链接 })` 答 `acted`）立刻照线算（决定 893；路由的那次提交在 `startTransition` 里、可能晚于下一帧，修订稿在下一帧量到它还不在视口里就解除了，审查 A 的 R1-M1）。元素每次按 id 在文档里找（`document.getElementById`），就地更新换了标题的元素也找得到；`ResizeObserver` 看的是视图（`main article.nw-reading`：正文写进去、长高、图与公式画出来，它都随之变高；右栏粘住、自己滚动，不改页的尺寸）。照实：设计原写看应用唯一的 `main`，S1 的实施照此写，WebKit 在 KaTeX 的样式表还在途时把它的通知报成没送到（"ResizeObserver loop completed with undelivered notifications"，e2e 的夹具算成未捕获的错误），改看视图（`f698d37e`，4.1.7 与 4.2 之前的"照实"）；视图之外变高的东西（视图之上的锁说明，视图之后的子页与附件两节）要等下一次滚动或改窗口才重量，审查 A 量的两种过时见 10.1。当前项的链接带 `aria-current="location"`；不移焦点；每项是 `memo` 的 `OutlineItem`，当前项变了只重画两项（审查 A 的 N6）。右栏跟着当前项（决定 886）：当前项变了、它的 `li` 不全在右栏的可见范围里、右栏自己滚动时（`scrollHeight > clientHeight`），只改右栏的 `scrollTop`，滚到它刚好露出（在上面时顶对齐，在下面时底对齐）；读者在右栏里动过之后（`watchReader({ on: 大纲的列表, within: "[data-page-panel]" })`，在列表的回调 ref 里随列表挂上而建、卸下而止：直接打开一页时视图的 HTML 晚到、大纲先是 `null`，修订稿在 `[page]` 的 effect 里建，`on` 永远是 `null`，点一下大纲的项就被算成读者动过，审查 A 的 R1-I2，第 1 节 (l)；换页时 `PageShell` 以 `key={page.id}` 重挂，进出编辑之后大纲重新挂上，它都随之重起，决定 897）不再跟；没变时不动右栏（l6 的"更多"一格核对窗口与右栏都没滚）；从不调 `scrollIntoView`（13.2 第 26 条）。不用 `IntersectionObserver`：一下跳过一段（点大纲、按 End）时没有标题穿过观察的带，当前项不变（2.1 第 2 条）。版本页与回收站的预览没有大纲。（初稿把当前项放到右栏的中间、每过一节都跟，审查 A 的 I3。）
- **862 大纲的长标题**：链接 `line-clamp-2`（不写 `block`，第 1 节 (b)），全文照旧是它的名字。认领 M5-polish 12.10 的"截断"一半（"大纲的长标题撑高右栏"）；"千分位"一半照旧留给 v0.2（l6 读"…and 139000 more"）。
- **863 小节的标题**：`PanelSection` 加一个 `icon` 属性（大纲 `List`、反链 `Link2`、属性 `SlidersHorizontal`、版本 `Clock`），`details` 是 `group space-y-2`，`summary` 是 `flex cursor-pointer list-none items-center gap-1.5 rounded-row text-xs text-faint [&::-webkit-details-marker]:hidden`，里面依次是 13 px 的图标（`aria-hidden`）、`h2.font-normal`、右端的 `ChevronDown`（`aria-hidden`，`ms-auto size-3.5 transition-transform duration-150 group-not-open:-rotate-90`）。不加样稿 `.ph .n` 的计数（00 号文档 4.5.4 说"右端一个箭头"；计数要另读）。`details` 的子元素照旧直接放在它里面（`W/test/page-panel.ts:27-30` 与 `:scope > ul > li` 的读法），`h2` 照旧在 `summary` 里（l6:319 以级别 2 的标题找"Properties"）。反链是小卡片（`rounded-control border border-border bg-surface px-2.5 py-2 dark:bg-surface-2`，名字 13 px、500，摘要每行 `line-clamp-2`、12 px、`text-faint`），属性是两列的 `grid`（键 64 px、`text-faint`，值 `text-muted-foreground`，12.5 px），版本照样稿 `.ver` 每项一行："时间 · 谁写的"排成一段字，同是 13 px 的 `--muted-foreground`（样稿的 `--text-2`），名字长时接着时间折行，标记一直在第一行的最右（审查 B 的 N4、R2-N2；一行里的元素与次序见 4.1.6，审查 B 的 R1-N1、C 的 R1-M7；"当前"是 `Mark` 的新写法 `current`：`rounded-full bg-accent-soft px-1.75 text-[11px] text-accent`，历史页与版本页的 `Mark` 不变）；右栏里的链接不加下划线（4.7）；解析不到的属性链接照旧是带虚线下划线的文字（`page-properties.tsx:266`：它不是链接，是"解析不到"的标记，三格断言 `decoration-dashed`；审查 C 的 M15）。
- **864 属性条**：只用 CSS（`reading.css` 的新一节，4.1.8），加一个增强 `propertyRoles`（`W/reading/property-roles.ts`，注册在 `diagrams` 之后、`scrollRegions` 之前；S2 起它之前是 `codeBlocks`）：给 `table.nw-props`（含嵌套的）补 `role="table"`，它的 `tbody` `rowgroup`、`tr` `row`、`th` `rowheader`、`td` `cell`，值里的 `ul` `list`、`li` `listitem`；清理时去掉它加的（原来就有的 `role` 不碰：服务端不写，记下自己加过的元素）。键 `text-faint`、值正文色（00 号文档 4.5.2"键灰字、值正文色"，样稿的值是次要色，从 00 号文档）。不做"+N"（00 号文档）。
- **865 "编辑"与页面菜单的按钮**：页面菜单的触发换成 `variant="outline" size="icon-sm" shape="pill"`；"编辑"是主按钮 `size="sm" shape="pill"`、前面一个 `Pencil`（`aria-hidden`，名字照旧"Edit"）。"编辑"自成一个组件 `EditButton`（同 P3 的"保存"，挂载时取到元素），经 `useFocusLeaving` 在它拿着焦点消失时把焦点交给 `h1`，以 `focus({ preventScroll: true })`：事件让页变得不能编辑（角色降为读者、升级开始、变成来源）时不再落到 `body`，也不把读到页中间的人拉回页顶（初稿没写 `preventScroll`，三种引擎里 `scrollY` 从 2000 变成 0，审查 A 的 I2；UP1 核对）。本 Phase 新加的把焦点交给标题或开关的地方都这样写（4.1.11）；编辑时大纲的 `reveal` 有意滚到那一行，不在此列。认领 M12-polish 第 6 项里"可写与否因事件变了时拿着焦点的……'编辑'，焦点落 `body`"的"编辑"一半；勾选框一半照旧留给 v0.2。`enter()` 先把焦点给 `h1`，进入编辑时"编辑"不拿着焦点，钩子不动。
- **866 编辑时的头一行**：头一行右边一组里有三个落点，经回调 ref 记进 `PageShell` 的状态、由新的 `EditHeadSlots` 上下文连同右边一组本身（`group`，决定 895 听它的焦点）给 `PageEdit`：第一个盒子（`meta`，阅读时放"谁、何时编辑过"，决定 894）、开关之前的 `<span className="contents">`（`before`）、开关之后的（`after`）。`PageEditingBar` 不再是一行，而是三段门户：状态的 `<output>` 进 `meta`，"保存"进 `before`，"完成"进 `after`；次序是［状态］［保存］［开关］［完成］（样稿的［状态］［开关］［⋯］［完成］，页面菜单在编辑时照旧不显示）。按钮是 `size="sm" shape="pill"`（"保存"`outline`，"完成"主按钮）。状态的 `<output>` 是 `nw-edit-status block [overflow-wrap:anywhere]`，字号、行高与颜色取盒子的（`text-[12.5px] leading-4 text-faint`，样稿 `.meta` 的 12.5 px 淡色，审查 B 的 N6，对比度照 4.7、不进 l5 的量项；失败时 `text-destructive`；空的时候盒子照旧占着它的宽，不多也不少一个间距，修订稿的 `empty:absolute` 不再要；长的说明——失败、繁忙、冲突——在盒子里折行：两行是 32 px，正好是右边一组的最小高（`min-h-8`），头一行不变；375 宽时英文的三种失败说明是三行，头一行高 16 px，照实记下、不截断，4.1.9；决定 894，审查 A 的 R2-I2、R2-N3，B 的 R2-N1），文字是"已保存。"时带 `data-saved`，`styles.css` 在它前面画 13 px 的琥珀 ✓（同 `.nw-saved` 的遮罩，强制色下 `CanvasText`），文字与键不变（00 号文档 4.5.1）。`leave()` 的 `stayed()` 另认两个槽里的元素：离开时焦点在头一行的"保存"或"完成"上（点了"完成"，或等上传时挪过去）也算"还在编辑里"，于是离开的保存照常不安静（撞上冲突时冲突的标题取焦点，`page-edit.tsx:279`）、离开失败时焦点回到编辑器（`back()`，`:289`）；成功离开时焦点照旧给"编辑"（审查 A 的 M5：初稿说的"离开之后给编辑"不由 `stayed()` 决定）。失锁时三个落点都空着（失锁的横幅照旧在内容列里）；正文没读到的分支照旧在内容列里画自己的"完成"。
- **867 编辑说明**：新键 `page.editingNote`：英文"You are editing; others can only read this page meanwhile."，中文"你正在编辑，别人此时只能阅读这一页"。`PageEdit` 在视图的最上面画（失锁时不画）：`<p className="-mt-3 flex items-center gap-1.5 text-[12.5px] text-faint"><Lock aria-hidden className="size-3.25 shrink-0" />…</p>`（样稿 `.editbar`：标题下 2 px）。普通的段落，不播报。
- **868 阅读时的锁说明**：位置不变（内容列里、正文之上），换样子：`flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-control border border-border bg-surface-2 px-3.5 py-2 text-[13px] text-muted-foreground`，前面一个 `Lock`，按钮 `variant="outline" size="sm" shape="pill"`；拿着锁的人的名字经 `isolate` 进 `bdi`。版本页（`version-page.tsx:198`）用同一个 `EditLockNote`，随之换样子、名字进 `bdi`（审查 C 的 M14）。名字进了 `bdi` 之后，以整句找它的 vitest（`findByText("Bob is editing this page.")`：Testing Library 的文字只算元素自己的文字节点）找不到它，其中三处 `queryByText(…)).toBeNull()` 会空过，九处同提交改成按 `status` 的整段文字找（7.1）。认领 M12-polish 第 5、11 项"26.12、32.4 的同类约二十处 `bdi`"里在 P4 的页上的几处：锁说明（S1，页与版本页）、右栏版本列表里的"谁写的"（S1：`versionBy` 另有一个返回 `Isolating` 的写法给右栏；字符串的写法照旧只有版本页用，审查 C 的 N2）、失锁横幅里的名字（S3）；其余照旧留给 v0.2。
- **869 代码块的头**（S2）：新增强 `codeBlocks`（`W/reading/code-blocks.ts`，注册在 `diagrams` 之后、`propertyRoles` 之前）：每个 `pre > code`（`language-mermaid` 的除外：图画出来之前它还是代码块）的 `pre`，以 `prepend` 加上 `div.nw-code-head`：语言名（`code` 的 `language-` 类去掉前缀；没有时不写）、一个 `span.nw-code-said`（`aria-live="polite"`，平时空着）、有剪贴板时（`navigator.clipboard?.writeText` 是函数，同 `create-token-dialog.tsx:194`）一个 `button.nw-code-copy`（`type="button"`，名字"复制"一直不变）。按下：先清空 `said`，`writeText(code.textContent)`（只是代码，不带头），成功时按钮 `data-state="copied"`（它的图标换成琥珀的 ✓，字照旧"复制"：看得见的反馈只有这一处，决定 890），`said` 写"已复制"、带 `data-state="copied"`（只给读屏：在行内、1 px 宽、`clip-path` 藏起，不用 `sr-only` 的绝对定位，免得"复制"失去它左边那个 `margin-inline-start: auto` 的元素而左移）；失败时 `said` 写"没能复制"、带 `data-state="failed"`，看得见；1.5 秒后都清掉。连按几次只认最后一次：每次按下记一个序号，前一次的 `writeText` 晚到时它的结果与计时都不算（审查 B 列的未核实一条，设计者核对：初稿的写法里前一次的 `finally` 会清掉后一次的"已复制"）。没有语言名又没有剪贴板时不加头。`said` 不是 `<output>`：`status` 的角色会让读页里 `getByRole("status")` 的读法多一个（`aria-live` 的 `span` 没有角色，同附件一节的 `p[aria-live]`，`W/pages/page/attachments-section.tsx:255`）；按钮的名字不变，读屏不会听到两次。头 `position: sticky; left: 0`、`user-select: none`，放在 `pre` 里面：顶层块不变，代码块的区域名称照旧"代码"（00 号文档 4.5.3）；打印时不印（`@media print`，应用今天没有别的打印样式，整页照旧印；页内查找照旧找得到头里的语言名与"复制"，记下）。清理时移除头、清掉计时。焦点：`patch.ts` 的 `keyed` 加 `button.nw-code-copy`，`keyOf` 给它 `copy`（按它在所在块里的第几个找回）：就地更新时拿着焦点的"复制"回到新的那一个。新键 `reading.copy`"Copy"/"复制"、`reading.copied`"Copied"/"已复制"、`reading.copyFailed`"Couldn't copy"/"没能复制"。按钮的宽度不预留：它在最右，`said` 在它左边、空着时宽 0，什么都不动。 S2 的实施与审查照实改了三处（[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)）：只认 `pre` 里只有一个 `code`、此外只有空白文字的块（作者手写的 HTML 里几个 `code` 的 `pre` 原先各加一个头、倒序堆在 `pre` 的开头，文字夹着 `code` 的 `pre` 原先也加头、"复制"只复制那个 `code`，审查 A 的 P4S2A-N2、P4S2A-F1-N2）；清理之后才到的 `writeText` 答复既不写字、也不再挂计时（各按钮共用的 `undone`，清理时先置真，审查 A 的 P4S2A-N1）；"复制"的图标 16 px、`gap` 6 px、字重 500，照样稿（审查 B 的 P4S2B-N2，主持者定：样子为主）。
- **870 hljs 与 callout 的颜色**（S2）：变量从 `.nw-reading` 挪到 `reading.css` 的 `:root`/`.dark`（编辑器借用其中的链接线与匹配色，决定 875）。hljs：关键字 `#a0661a`/`#e0b061`、字符串 `#3e7d5a`/`#8fcf9f`、注释 `#9a9ca3`/`#6f7178`、函数名与标题 `#3d5fb5`/`#92aaf0`（样稿的四色），另推五个：常量 `#7a52b3`/`#c3a6f2`、内置 `#2f8f6a`/`#5cc49a`、名称 `#b15c2e`/`#eb9a6c`、增 `#3e7d5a` 底 `rgba(62,125,90,.1)`/`#8fcf9f` 底 `rgba(143,207,159,.12)`、删 `#d64545` 底 `rgba(214,69,69,.08)`/`#ff7b7b` 底 `rgba(255,123,123,.12)`（删与 `--destructive` 同色）。callout 七族（今天的 rgb 三元组按族归并）：蓝 `#3d6fb8`/`#8fb0ec`（note、info、todo）、青 `#2f8f6a`/`#5cc49a`（abstract、tip，样稿的 tip）、绿 `#3e8a4f`/`#7fcf8f`（success）、橙 `#b26a12`/`#e0a656`（question、warning）、红 `#d64545`/`#ff7b7b`（failure、danger、bug）、紫 `#7a52b3`/`#c3a6f2`（example）、灰 `#6b6e76`/`#9a9ca3`（quote）；底是 `color-mix(in srgb, var(--nw-callout) 7%, transparent)`，边是 22%，圆角 10 px，标题是 `var(--nw-callout)`（暗色的值本就是亮的那一档，不再另混白）。注释写明出处（00-M13 4.1 与 reading.html，五个推出的写明"P4/S2 推出"）。`design-tokens.test.ts` 的亮暗成对一格扩到 `reading.css` 的 `:root`/`.dark`（审查 C 的 N4，设计者定：S2 起那里有二十多个变量，漏写一个暗色的值只有截图看得出；同提交加一格，4.5）。
- **871 mermaid**（S2）：`options(theme)` 的 `theme` 换成 `"base"`，加 `themeVariables`：亮 `{ background: "#ffffff", primaryColor: "#f4f4f5", primaryBorderColor: "#d4d4d9", primaryTextColor: "#18191c", secondaryColor: "#f4f4f5", tertiaryColor: "#ffffff", lineColor: "#91939b", textColor: "#18191c", noteBkgColor: "#f8f1e4", noteBorderColor: "#c2871f", noteTextColor: "#18191c", fontFamily: "inherit", fontSize: "13.5px" }`，暗 `{ background: "#1f2023", primaryColor: "#25262a", primaryBorderColor: "#3a3b41", primaryTextColor: "#e8e8ea", secondaryColor: "#25262a", tertiaryColor: "#1f2023", lineColor: "#74767e", textColor: "#e8e8ea", noteBkgColor: "#39332a", noteBorderColor: "#d9a84e", noteTextColor: "#e8e8ea", fontFamily: "inherit", fontSize: "13.5px" }`（`inherit` 取正文的系统字体；灰阶的节点与线，强调用琥珀；`diagrams.ts` 是 13.2 第 32 条的例外，十六进制）。只给这十三个不够：`base` 由它们转色相推出饼、git、思维导图、时间线的系列色，灰推不出颜色，暗色还要 `darkMode`（审查 B 的 I1）；系列色、甘特、看板、时间线、xychart、边标签与库节点另给，见决定 888 与 4.2.6。`themeVariables` 与 `themeCSS` 不进 `secure`（图自己的 `%%{init}%%` 与 frontmatter 照旧能改颜色；只是颜色，不是信任边界；mermaid 对指令给的 `themeCSS` 自己净化。初稿写"与今天能改 `theme` 一样"，照实不一样：mermaid 把这里给的 `themeVariables` 交给作者指定的主题去推导，应用的灰阶盖在作者的主题上、字读不清，S2 照决定 904、906 改：图自己指定了不是 `base` 的主题、或给了自己的 `themeVariables` 时只照它自己的配置画，只给变量、没写主题的主题按应用的亮暗取 `dark` 或 `default`；没有指令也没有 frontmatter 的图不另解析，审查 A 的 P4S2A-I2、P4S2A-F1-M1、F1-N1）。缓存、换主题重画照旧（13.2 第 23 条）。图的容器：`border`、12 px 圆角、`padding: 26px 20px`，亮 `--surface`、暗 `--nw-code-background`（样稿 `.diagram`）。强制色下不强制它（`forced-color-adjust: none`）：图连同它的底照应用的主题画，同图片；不然底被换成 `Canvas`，而 mermaid 的 SVG 的 `fill` 不被强制，高对比的明暗与应用的主题相反时直接画在底上的字看不见（今天就这样，审查 B 的 R2-N3）。 流程图的节点照样稿：`flowchart: { padding: 9 }`（mermaid 默认 15；"浏览器"一个词的节点 Chromium 76.49×38.24 px，样稿 74.5×40.25；审查 B 的 P4S2B-N3，主持者定），状态图的注也用它、变小一点，标签都不被裁；图框高 129.3 px，样稿 94.25（图框的内边距与 mermaid 自己围着图的边距照旧），照实记成偏离（10.2）。Chromium 里强制色下图里的 `foreignObject` 另要 `forced-color-adjust: none`（`cdedff02`，4.2.5）。
- **872 任务项**（S2）：复选框照 4.3 用原生的 `accent-color`（P1 已给 `input[type=checkbox]`），不画样稿的自制方块；不可用的已勾选复选框照浏览器的样子（Chromium、Firefox 灰，WebKit 淡琥珀，第 1 节 (b)），不另写。已完成的项照 00 号文档 4.5.3：它自己的文字划线、颜色 `--faint`，里面的链接与标签跟着变淡（`color: inherit`，审查 B 的 M4），嵌套的列表 `display: inline-block; width: 100%` 且颜色设回正文色。服务端写的任务项文字以空格开头，划线从复选框后的那个空格起；`text-decoration-skip-spaces` 三种引擎都不支持，照实记成与样稿的偏离（审查 B 的 N1）。01-P1 第 9 节交给 P4 的"任务列表的复选框"一条到此结束。 S2 的实施与审查之后（[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)）：`text-decoration` 也传进项里流内的块，S2 的第一稿里已完成项下面的代码块（代码、头里的语言名与"复制"）、引用、callout、图与表格都被划线、变淡；照嵌套列表的办法，它的 `pre`、`blockquote`、`div`（callout、图与表格的外层）、`details`（折叠的 callout）也是 `inline-block; width: 100%`，另加 `vertical-align: top`（滚动的 `inline-block` 以底边为基线，不加时下面多一行的空），颜色设回正文色（引用保留它自己的次要色），里面的链接与标签照它们自己的颜色（`color: inherit` 那一条的 `:not()` 同样排除这些块）；项自己的文字、松散项的各段照旧划线（`9685e300`，主持者定）。三种引擎、亮暗、1440 与 375 宽，已完成项与同样内容的没完成项高度、各块的位置与大小都相同，粘住的头、横滚、"复制"与折叠的开合照旧（10.2）。
- **873 正文**（S2）：字号 16 px、行高 1.8，块间 1.125rem（18 px）；h1 1.5rem、h2 21 px（630，前 38 px、后 14 px，不再有底线）、h3 17 px（620，前 26 px、后 10 px）、h4–h6 16 px；"后"只给接着的不是标题的块，标题接标题时用后一个标题自己的"前"（h2 接 h3 26 px、h2 接 h2 38 px，同样稿；审查 B 的 M2）；标题的 `scroll-margin-top: 1rem` 照旧。链接：正文色、1 px 的下划线 `--nw-link-line`（亮 `rgba(197,138,34,.6)`、暗 `rgba(217,168,78,.55)`）、偏移 3.5 px，悬停时下划线是字色；没建的页：`--muted-foreground`、虚线、`--faint`；标签：`--surface-2` 底、6 px 圆角、0.84375em、`--muted-foreground`；高亮 `--nw-mark-background` 加 3 px 圆角；行内代码：1 px 边、5 px 圆角、`--surface-2`、`--font-mono`、0.84em；表格（属性表之外）只留横线，表头 13 px、500、`--muted-foreground`，单元 `10px 12px`、行高 1.5（样稿每行 41.75 px，审查 B 的 N3）；callout 的字号与行高写在 `.nw-callout` 上（15 px、1.7），标题自己定 14 px：里面的 `pre` 照旧 13 px（审查 B 的 M3：写在子元素上的那条特指度盖过 `.nw-reading pre`）；折叠的 callout 的标题右端一个 chevron，关着时转 −90°，同右栏的小节（审查 B 的 M8）；强制色下 callout 标题的图标与 chevron 是 `CanvasText`（审查 B 的 N2）；`pre`：1 px 边、12 px 圆角、`--nw-code-background`（亮 `#f6f6f7`、暗 `#1f2023`，挪到 `:root`/`.dark`）、13 px、行高 1.7，内边距在 `code` 上（`14px 16px`，`width: max-content; min-width: 100%`）。版本页与回收站的预览也是 `.nw-reading`，随之换样子。
- **874 编辑器的字体**（S3）：只给 `SourceEditor`（对比与冲突视图照旧等宽，00 号文档 4.5.5）：`proseTheme`（`W/editor/prose-lines.ts`）：底亮 `--surface`、暗 `--nw-code-background`，12 px 圆角；`.cm-scroller` `font-family: var(--font-sans)`；`.cm-content` 15.5 px、行高 1.85、`padding: 22px 0`；`.cm-line` `padding: 0 26px`；光标 `--accent`，写在 `.cm-content` 的 `caretColor` 上（`SourceEditor` 不加 `drawSelection()`，光标是浏览器自己的，`.cm-cursor` 不存在；修订稿那一条在它下面是死的，`editorTheme` 里的同一条今天就是死的，审查 B 的 R1-M1）。它在扩展里以 `Prec.high(proseTheme)` 加：CodeMirror 把样式模块倒过来挂（`@codemirror/view` 6.43.13 `dist/index.js:8351`），排在前面、优先级高的主题冲突时赢；初稿把它排在 `editorTheme` 之后，行高与内边距被盖过（审查 B 的 I2，第 1 节 (f)）。`proseLines`（同文件）：只看 `visibleRanges` 的 ViewPlugin，语法树里的 `FencedCode`、`CodeBlock`、`Table` 的行与 frontmatter 的行加 `nw-mono-line`（`var(--font-mono)`、13.5 px，行高随字号，第 1 节 (a) 的 `mono-relative`），frontmatter 的行另加 `nw-frontmatter-line`（整块 `--muted-foreground`，里面的高亮 `color`、`font-size`、`font-weight` 都 `inherit`：解析成 setext 标题的那一行不放大，lezer 读出的链接、水平线不画样子；决定 889。样稿的 `---` 是淡色、键是次要色、值是正文色；这里整块一种次要色，免得为键另加一层装饰，记成偏离）；frontmatter 的范围由新导出的 `frontmatterRange(state)`（`yaml-quotes.ts`，用它的 `opening`、`closing`：闭合的到闭合的那一行，没闭合的到第一个空行之前），按 `state.doc` 记在 `WeakMap` 里，同一份文档只算一次（审查 A 的 N4：没闭合时每滚一下都扫整篇）；起点在 frontmatter 里的节点不算。文档、可见范围或语法树变了时重建。`markdownHighlight`（共用）：标题 1.29em/650、1.065em/630、1em/620、其余 600（样稿 `.editor .h1` 20 px、`.h2` 16.5 px 对 15.5 px），符号 `--faint`（样稿 `.f`），引用 `--muted-foreground`。 S3 的实施与审查之后（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）：行内代码（`tags.monospace`）的字号是 `calc(var(--nw-editor-code, 1em) * var(--nw-heading, 1))`：`.cm-content` 设 `--nw-editor-code: 0.871em`（13.5 px），`.nw-mono-line` 设 `1em`，h1、h2 的规则另设 `--nw-heading`（1.29、1.065），标题里的行内代码随标题放大，17.4、14.4 px（审查 B 的 P4S3B-M2、A 的 P4S3A-F1-M1：高亮的 `span` 是扁平的，标题与 monospace 的类在同一个 `span` 上）；引用的规则排在 `url`、`processingInstruction` 之前，引用里的标记与地址也淡（B 的 P4S3B-N1）；对比与冲突视图的等宽是编辑器的等宽字体、13 px（决定 909）。
- **875 编辑器的链接与匹配色**（S3）：`markdownHighlight` 的 `tags.link` 换成正文色、不画下划线，`tags.url` 换成 `--faint`；下划线由 `markupMarks` 画（决定 889：lezer 的链接节点包着 wiki 链接的内层括号与 `[文字](地址)` 的括号与地址，画在 `tags.link` 上就都带下划线，审查 B 的 I3）；对比与冲突视图里链接只是正文色。`editorTheme` 的搜索匹配用 `--nw-mark-background`，当前的匹配用新变量 `--nw-match-selected`（亮 `rgba(245,196,81,.72)`、暗 `rgba(217,168,78,.5)`，`reading.css` 的 `:root`/`.dark`）；`--nw-editor-link`、`--nw-editor-match*` 三个变量删掉，`--nw-editor-mono` 改为 `var(--font-mono)`。`editor/theme.ts` 不再写色值，`design-tokens.test.ts` 的白名单只剩 `reading/diagrams.ts`，注释随之改。 S3 的实施与审查之后：CodeMirror 自带的查找面板、它的输入框与按钮、对比视图的 gutter 原是亮色基底的样子，也只用变量另写（决定 910）。
- **876 编辑时的大纲**（S3）：`W/editor/edit-outline.ts`：从语法树读标题（`ATXHeading1`–`6`、`SetextHeading1`–`2`，只进 `Document`、`Blockquote`、列表与列表项，不进代码与表），起点在 frontmatter 里的不算，文字去掉 `HeaderMark` 与 setext 的下划线行、空白归一，空的不列；至多列 `listedUpTo`（从 `page-outline.tsx` 导出）个，其余只数。构造时立刻算一次（审查 A 的 M3：进编辑时右栏的大纲不空 300 ms），之后每次改动停手 300 ms 再算；树没覆盖到文档末尾时（`syntaxTreeAvailable` 为假）以 `ensureSyntaxTree(state, doc.length, 20)` 一片一片补完（每片之间 `setTimeout(0)`）：下一片前 `view.state.doc` 变了才放弃这一轮、重新排 300 ms 的计时，只换了选区、补全或解析工作者自己的 `Language.setState` 换了 `view.state` 时用 `view.state` 接着补（审查 A 的 I4：初稿遇到不改文档的事务就放弃、不再重排）；补完就用它返回的树算。插件记着上次报出的各标题的位置，之后的改动经 `changes.mapPos` 映射过去，点按时按序号取映射过的位置（审查 A 的 M2：初稿用报出时的 `from`，打字之后 300 ms 之内点会跳错行）；`destroy` 清掉计时与补的那一片（审查 A 的 N5）。经 `SourceEditor` 的新属性 `outline?: OutlineSink`（`{ upTo: number; told(listed, more): void }`）报出去；句柄加 `reveal(index)`：光标到第 `index` 个标题那一行的开头（`view.state.doc.lineAt(映射过的位置).from`：在标题那一行的最前面打的字排在映射的位置之前，引用、列表里的标题的节点在 `> `、`- ` 之后，审查 A 的 R1-N3），`EditorView.scrollIntoView(行首, { y: "start", yMargin: 16 })`，焦点给编辑器（窗口滚动；不是左栏或右栏的滚动，13.2 第 26 条说的是那两处）。页这一边（`W/pages/page/edit-outline.tsx`）：`OutlineBox`（MobX 的可观察：`listed`、`more`、`reveal`），`PageShell` 建一个，交给 `PageEdit`（转给 `SourceEditor`，`reveal` 接到句柄）与 `PagePanel`；`EditOutline`（`observer`）在编辑时画在右栏的最上面：同一个 `PanelSection`（"大纲"）与 `nav`（名字"Outline"），每项一个 `button`（缩进同阅读时，`key` 是序号：位置变了拿着焦点的按钮不换），"…and N more"同阅读时；不标读到的一节（00 号文档）。页这一边只 `import type` 编辑器的类型，主包里不进 CodeMirror（`out-of-main.ts`）。认领 00 号文档已认领的"编辑时没有大纲"（M5-polish 13.1）。 S3 的实施与审查之后（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）：项的文字同阅读时的大纲（决定 907），只排除闭合的 frontmatter（决定 908）；`same()` 只比级别、文字与 `more`，位置留在插件里（审查 A 的 P4S3A-N3）；`OutlineBox` 另有可观察的 `ready`（`told()` 设真、`clear()` 设假），编辑器第一次报出之前右栏照旧画阅读时的 `PageOutline`（不量读到的一节，那一节连开关 `inert`；A 的 P4S3A-M1、F1-N1、F2-N1，C 的 P4S3C-N1）；按钮 `block`、手形，项与阅读时同高（B 的 P4S3B-M1、N2）。
- **877 "保存"**（S3）：显出的条件是 `editing.unsaved`，或它显着的时候焦点就在头一行右边一组里、之后一直没离开（`held`，决定 885、895；初稿的"或有显示出来的保存失败"是多余的：显示出来的失败一定伴随未保存的修改，`page-editing.ts:226,257-259,302,314,349,385`，审查 C 的 M5）。不显出时 `invisible`，占位、不可聚焦、不在可访问性树里，不卸下（决定 894）。`held` 是 `PageEditingBar` 的状态：组上的 `focusin` 在有未保存的修改时记下，焦点已在组里时才有了修改（等上传时插进的链接）也记下；`focusout` 时 `relatedTarget` 不在组里、`document.activeElement` 也不在组里才清掉（窗口失焦时它仍在组里，第 1 节 (g)；在组里从"完成"移到开关、"保存"时不清，决定 895）；没有修改时焦点进组（点开关、点"完成"、键盘走进来），藏着的"保存"不显出（审查 A 的 R2-M1）；拿着焦点、没东西可存时按它，`save()` 本来就在 `version === savedVersion` 时返回，不做事。不再经 `useFocusLeaving`：它拿着焦点时不会消失。WebKit 里点它不给它焦点（第 1 节 (g)），焦点不在组里，保存完成就隐去；焦点在点的那一下已从编辑器落到 `body`，与今天点一直在的"保存"相同，P4 不另管。等上传时规则不变，Mod+S 照旧随时保存。
- **878 冲突、失锁与补全**（S3）：冲突的面板与失锁的横幅换成 `rounded-control border border-destructive/35 bg-surface px-4 py-3`，按钮 `size="sm" shape="pill"`，失锁横幅里的名字进 `bdi`（决定 868）；补全的弹层：圆角 `var(--radius-popover)`、阴影 `var(--elevation-popover)`、4 px 的内边距，选项 `var(--radius-row)` 圆角、`var(--text-row)` 字号，颜色与选中项的环不变（l5 量它们）。 S3 的实施与审查之后：强制色下补全的选中项另有一圈内嵌的透明 2 px 外框（`outline-offset: -2px`，同应用的菜单项，只在 `@media (forced-colors: active)` 里，审查 B 的 P4S3B-M4）；冲突面板里"上一处、下一处"那一组 `flex-wrap justify-end`，320 宽英文时两个按钮上下排、不越出面板（B 的 P4S3B-M3）；补全的密度照旧（决定 911）。
- **879 PG17**：新故事 `e2e/stories/page/pg17-page-head.spec.ts`，四格（S1 第 1、3、4 格，S2 第 2 格；第 3 格是设计修订时加的，第 4 格是 S1 的审查加的，审查 B 的 P4S1B-F1、C 的 P4S1C-F2-N1），进 `threeBrowsers`（`page\/pg17-`）与 `check.mjs` 的 `REQUIRED`（每格 firefox、webkit 两行；第 4 格的两行是审查 C 的 P4S1C-F2-M1 补的）；只有页面版本，接口与 MCP 版本豁免（13.4 第 3 条）。格的步骤见 5.1、5.2。
- **880 对比度的下限**：P4 改了被量元素的底或颜色的只有两处：S2 的阅读视图链接（字色与下划线变了，焦点环与底没变）与 S3 的编辑器（暗色的底由 `#1a1b1e` 变成 `#1f2023`）。S2、S3 各重量 l5 的五项、实量值写进 Step 的记录，量出的值不低于下限就不改下限（决定 891）。初稿推算暗色的编辑器外框降到约 7.49、S3 必须改，不成立：l5 量外框时取的是父元素链上第一个不透明的底（`e2e/fixtures/contrast.ts:112-116`），不是编辑器自己的底，两种次序下实量暗 7.923、亮 3.098（审查 B 的 M6、C 的 M1）；量法改不改不在 P4。pg4、se1 不涉及（P4 不碰确认框与搜索页）。
- **881 移交的认领**：M12-polish 第 1 项里 M5-polish 的 12.10 的"截断"一半（S1，决定 862）与 13.1"编辑时的大纲"（S3，00 号文档已认领）；第 6 项里"编辑"的焦点一半（S1，决定 865）；第 5、11 项的 `bdi` 里 P4 的页上的几处（S1、S3，决定 868）。不认领第 1 项里与阅读、编辑、右栏有关的这些（逐项核对过 M5-polish，审查 C 的 M17 补上初稿漏的 12.6、12.9、13.2、17.3、18.3、18.7；12.10 的"千分位"一半，审查 C 的 R1-N6）：阅读视图的行为，不是外观：3（高亮的闪动：S2 只核对代码块的头不添闪动，清理与重加在同一个任务里）、6.4、8.3、10.3、11.3、11.4、11.5、11.6 的其余、11.9；补全与右栏的数据与功能，不是外观：12.1、12.2 的开合、12.5、12.6（右栏属性链接的文字与序号）、12.7、12.9（反链与补全的代价）、13.2（标签的总览、属性里的标签链接）；附件与粘贴：17.2a、17.3（右栏的附件链接不显示大小）、18.3（粘贴的撤销粒度）、18.4b、18.4c、18.7（等上传时从编辑之外按 Mod+E 的焦点）；别处：4、20.4、21.1；第 7 项"Firefox 复制右栏的值多一个尾随空格"（右栏的属性换成 `grid` 之后由 S1 的实施者在 Firefox 里看一次、照实记进本文 10.1，不改；审查 C 的 N8）。每个 Step 的文档提交在 M12-polish 的"处理进展"写一行。
- **882 标签组一处**（S1）：01-P1 第 9 节的标签组清单里在 P4 页上的一处，`W/pages/page/asset-dialogs.tsx:221` 的 `space-y-2` 换成 `space-y-1.5`；其余在 P5。
- **883 基线**：P3/S2 合并之前最后一次门禁的代码与 `072dcec2` 相同（第 1 节），不另跑基线；S1 提交 1 的 `make e2e` 兼作核对：只败第 1 节那三格（同行同消息）。
- **884 新键与两种语言**：新键 `page.editedBy`、`page.editingNote`（S1），`reading.copy`、`reading.copied`、`reading.copyFailed`（S2），都在 `en.ts` 与 `zh-CN.ts` 里，改了文字的格两种语言各一（13.2 第 4 条）；没有删掉的键。

以下是主持者照设计审查定的（负责人可以改判），写法由设计者补在 4.x：

- **885 "保存"拿着焦点时不消失**：显出的条件是 `editing.unsaved`，显着时焦点在头一行右边那一组里就留着（决定 895 补的：不只是它自己，又不让焦点进组本身让藏着的它显出）；不显出时 `invisible`、占位、不可聚焦（决定 894）；焦点离开那一组且没有未保存的修改时才隐去；拿着焦点、没东西可存时按它不做事。不再需要"它拿着焦点消失时交给编辑器"的那一步（编辑结束、失锁时照各自的规则）。改 00 号文档 4.5.5 与决定 745 的这一层意思（2.1 第 8、9 条）、本文的 877（审查 A 的 Q1、C 的 M5；写法见 4.3.5）。
- **886 右栏跟着读到的一节只滚到刚好露出**；读者在右栏里滚过之后不再跟，到下一次换页、或进出编辑之后大纲重新出现为止（P2 的 `watchReader({ within })`，同 P2 决定 779、791；后一半是 S1 照实定的，决定 897）；右栏收起或不在正文旁边时不量（审查 A 的 I3、C 的 I1、A 的 N6；写法见 861、4.1.7；2.1 第 2、12 条）。
- **887 关着的 `details` 里的标题不当读到的一节**（`closest("details:not([open])")`），大纲照旧列出它们（审查 A 的 I1；861、4.1.7；PG17 第 1 格）。
- **888 mermaid**：暗色 `darkMode: true`；系列色写死成 callout 的七族；甘特、xychart、暗色的边标签另给；流程图节点 9 px 圆角，圆柱（库）节点琥珀边、淡底，照样稿（00 号文档 4.1 的琥珀用处由 2.1 第 10 条补一句"图里的库节点"；选择器命中不止圆柱时改写成偏离）（审查 B 的 I1、M7，C 的 M4）。设计者照第 1 节 (d) 的探针补齐：七族的次序是蓝、橙、青、紫、红、绿、灰（相邻的色相隔得开），轮着给 `pie1–12`、`git0–7`、`cScale0–11`；系列上的字亮色白、暗色近黑；时间线的线、git 与思维导图的标签、ER 的行与圈、甘特的网格线、雷达与架构图另给；选择器只命中圆柱，不是偏离（写法见 4.2.6）。再修订时照审查 B 的第 1 轮补：甘特亮色"进行中"的底与边（`#e7e7ea`、`#91939b`，原先白底浅边看不见），暗色"关键"的底压暗到 `#b33b3b`（浅字 4.76:1；`#ff7b7b` 配浅字只有 2.05:1，配近黑的字要在 `themeCSS` 里另分条外与条内的字，压暗的底一处就够，B 画过）；暗色看板的列名近黑（6.35:1，原先 2.26–2.74:1）；时间线的事件框去掉 mermaid 自己的 `brightness(120%)`（亮色白字由 2.85–3.70 到 3.99–5.70）。旅程的节用 `fillType0–7`，不给，照旧是灰的（看得清），角色的点是 mermaid 配置里写死的颜色；饼多于七块时颜色重复（第 8 块又是蓝），记成偏离（审查 B 的 R1-M2、M3、N3、N4）。
- **889 编辑器里 `[[`、`]]`、`==` 压淡**，wiki 链接只有页名带下划线，`[文字](地址)` 的括号与地址压淡、不带下划线，frontmatter 里不画链接的样子，照样稿在 S3 做；不在行内代码与代码块里压淡；只处理可见范围，测试数装饰（审查 B 的 I3、Q1、N7）。写法由设计者照探针定（第 1 节 (e)）：只看可见范围的装饰 `markupMarks`，不加 lezer 的行内语法。理由：一、lezer 的链接节点已经包着 wiki 链接的内层括号，加 `WikiLink` 的行内解析要排在 `Link` 之前、自己找 `]]`，每个 `[[` 往后扫到 `]]` 或行尾，正是 M12/P4/S5 以 `linearScans` 去掉的那种按长度平方增长的扫法（成串的 `[[a` 是服务端 Markdown 的病态输入之一，`server/internal/platform/markdown/markdowntest/inputs.go:225`），要线性就得照 `closersOf` 另做一份预先算好的闭合表；二、frontmatter 不在 lezer 的语法里（它读成水平线加 setext 标题），无论哪种写法都要按 `frontmatterRange` 另判，装饰本来就有它；三、装饰只碰可见的几十行，不改语法树：编辑时的大纲、补全读到的树照旧，M12/P4/S5 的差分与上限不受影响。代价：`==` 与 wiki 链接按行内的文字认（不跨行），行内长过 `inlineLimit` 的一段里 lezer 不认行内代码，那里的 `==` 也压淡（写法见 4.3.3）。 S3 的实施与审查之后（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）：wiki 链接照服务端的读法认（`W/editor/wiki-links.ts` 的 `wikiLinks`，编辑时的大纲也用它）：前面有奇数个 `\` 的、中间另有方括号的、页名与小节去掉空白之后都空的不算，链接的标题（`LinkTitle`）里不认；下划线不连两边的空白，表格里的 `\|` 一起淡（审查 A 的 P4S3A-N2）。`[see [[A]]](u)` 里的 wiki 链接照样标（lezer 不把外层读成链接，服务端把它当链接的文字），照实。
- **890 复制的反馈看得见的只有一处**：按钮自己变成 ✓（加"已复制"或只 ✓，照样稿）；`said` 的成功一句只给读屏（`sr-only`）；失败的一句看得见（审查 B 的 Q2）。设计者取"只 ✓"：样稿只画了没按时的"复制"，按钮的字与名字一直是"复制"（869），图标换成琥珀的 ✓；成功的一句在行内藏起（不用 `sr-only` 的绝对定位，869）。
- **891 l5 的编辑器外框照今天的量法不降**：S2、S3 照旧重量、记实量值，量出的值不低于下限就不改下限；S3 提交 1 没有 e2e 改动就用 `web:`；量法是否改为编辑器自己的底不在 P4 做（审查 B 的 M6、C 的 M1、Q1；880、4.4、6.3）。
- **892 `NotebookSettingsLayout` 的 `shown === undefined` 那一支也画 `<ColumnTop />`**（审查 C 的 M14；857）。
- **893 读到的一节的"到底"规则只在文档能滚时用**；点大纲之后，目标还在视口里时优先标它，直到读者再滚（审查 A 的 M1；861、4.1.7）。
- **894 头一行的行数只看宽度、不看模式**：右边一组在阅读与编辑时同宽，编辑时的状态占"谁、何时编辑过"的那个盒子，"保存"不需要时 `invisible`（占位、不可聚焦），进出编辑与"保存"出现、隐去时标题不动（审查 A 的 R1-I1，00 号文档 4.5"进出编辑时页面不跳"）。设计者照第 1 节 (j)、(n) 的探针定写法（4.1.3）：右边一组 `flex-[0_1_26rem]`（不长、可缩到行宽）、`min-h-8`（组里一个按钮也没有时——进编辑、内容读到之前，失锁，读取失败，窄于 1280 px 时开关又不显示——它照旧 32 px 高，两行的状态也正好 32 px：不然那一行塌成 0 高、标题上跳 16 px，两行的状态让标题下移 2 px，审查 A 的 R2-I2、B 的 R2-N1），一行放得下面包屑的 `14rem` 加它的 `26rem` 与 12 px 的间距（行宽 652 px，约 950 px 宽的窗口）时一行，放不下时它折到第二行、靠右，行数只看行宽；它的第一个盒子 `me-1 min-w-0 flex-1 text-end text-[12.5px] leading-4 text-faint`，阅读时放 `HeadMeta`、编辑时放状态，按钮都是 `flex: none` 的 30 px。375 与 820 宽时阅读、编辑都是两行，1024 宽起都是一行。PG17 第 3 格在 375 与 820 宽时核对（含内容读到之前的那一段）。
- **895 "保存"的 `held`**（第 2 轮改）：焦点在右边一组里、而且"保存"已经显着（有未保存的修改，或焦点就在它上面）时才保住它，直到焦点离开那一组；焦点进组本身不让藏着的"保存"显出（审查 A 的 R1-M2、R2-M1；补决定 885）。第 1 轮的写法是"焦点在右边一组里的任何地方"：没有修改时点开关、点"完成"（离开要等一次往返）、键盘走进组，一个"保存"都会在已经走过的位置上冒出来。写法见 877、4.3.5：组上的 `focusin` 在有未保存的修改时记下，焦点已在组里时才有了修改也记下（藏着的"保存"拿不到焦点，焦点在它上面时它已经显着，不另判）；`focusout` 看 `relatedTarget` 与 `document.activeElement`，照旧。
- **896 藏起的"保存"照旧占位，与样稿不同**（S3）：编辑时没东西可存，"保存"`invisible`、照旧占着它的 54 px，状态的右边离开关（窄于 1280 px 时是"完成"）约 70 px，样稿是 8 px（那个位置在样稿里是"⋯"）：记成与样稿的偏离（4.1.9、4.3.5、7.4）。理由：打字、自动保存时头一行什么都不动（00 号文档 4.5"进出编辑时页面不跳"，决定 894）；不需要时卸下，静止时同样稿，但每次自动保存状态都左右滑一个按钮宽；一直显出要改 00 号文档 4.5.5 与决定 745 的"只在需要时显出"（审查 B 的 R2-Q1）。

以下是主持者照 S1 的实施审查定的（负责人可以改判）：

- **897 读者在右栏里动过之后不再跟，到换页、或进出编辑之后大纲重新出现为止**（审查 A 的 P4S1A-Q1、C 的 P4S1C-M2）：读者的记录跟着大纲的列表走（`watchReader` 在列表的回调 ref 里随它挂上而建、卸下而止，4.1.7）。阅读时才有大纲（S3 之前编辑时没有），进编辑再回到阅读，列表重新挂上，右栏又跟着读到的一节滚；视图晚到时，大纲挂上之前读者在右栏里的动作也不算。照实写，不把"动过"挪到 `PageShell` 按页记（那一种写法是变异 t12：`page-panel.test.tsx` 的 897 一格与视图晚到一格红）。886 的说法、4.1.7、8.2 第 2、7 条随之改；S3 给编辑时加大纲时照它的写法再核对一次。
- **898 决定 892 那一支的横幅在设置的列里**（审查 B 的 P4S1B-Q2）：`NotebookSettingsLayout` 的 `shown === undefined` 一支（索引地址换到"基本信息"之前的那次渲染）把 `<ColumnTop />` 包进与 `SettingsPage` 同样的列（`settingsColumnClass`，从 `W/app/settings-page.tsx` 导出），没有横幅时那一层 `empty:hidden`：从索引地址进设置时横幅不在全宽闪一下（改前三种引擎都是先在全宽，x=292、宽 1108，再进列，x=498、宽 696；改后两次都在列里）。`upgrade-banner.test.tsx` 以 `MutationObserver` 核对横幅从出现起一直在列里（变异 106、t13）。
- **899 "编辑过"一句名字长时只截名字、保住时间**（审查 B 的 P4S1B-Q3）：句子是 `flex justify-end-safe overflow-hidden whitespace-pre` 的一行，名字的 `bdi` 是 `min-w-0 truncate`（`Isolated` 加 `nameClass`），其余不缩；中文"{name} 编辑于{when}"名字也在前，同样。代价：`innerText` 与选中复制成了三行（审查 A 的 P4S1A-F1-N1）；320 宽、能编辑的账户、"10–59 minutes ago"时名字缩到 0、看不见（`justify-end-safe` 之后放不下时裁的是结尾，不到 1 px，审查 A 的 P4S1A-F1-N2）。PG17 第 2 步核对句子不被截、名字被截（变异 t14）。
- **900 带锚点打开的页，视图第一次到时焦点给锚点，不看读者其间的动作**（审查 C 的 P4S1C-F1-Q1）：照现状（`W/pages/page/view-anchors.ts:72-80`，M6/M12 的写法，不是 S1 带来的；S1 的开关在头一行，读者在视图到之前按了它，焦点也照样被拿走；13.2 第 26 条要异步的答复之后移动焦点时看读者动没动），记给 v0.2（第 9 节，`docs/v0.2/handoffs/M12-polish.md`）。PG17 第 12 步刷新带锚点的地址之后先等锚点的标题拿到焦点再按开关（审查 C 的 P4S1C-F1-I1），第 14 步进不带锚点的地址（5.1）。

以下是主持者照 S2 的实施审查定的（负责人可以改判）：

- **901 callout 的标题是一行文字**（审查 A 的 P4S2A-I1、B 的 P4S2B-I1）：标题回到块（`display: block; line-height: 1.5`），16 px 的图标是行内的 `::before`（`inline-block`，离字 7 px，`vertical-align: -0.15em`），折叠的 `summary` 是 `position: relative`、留出 22 px 的右内边距、`list-style: none`，WebKit 另以 `::-webkit-details-marker { display: none }` 藏起自带的三角，chevron 的 `::after` 绝对定位在第一行的右端（`top: calc((1.5em - 14px) / 2)`）。设计 4.2.5 的 `display: flex` 把标题里的每段文字、每个加粗、链接与行内代码都拆成弹性项：逗号前多 7 px，375 宽时排成并排的几列各自折行，长地址溢出 callout、阅读视图横滚，`innerText` 与选中复制都断成几行（A、B 在三种引擎里量过）。单行标题高 21 px，同样稿 `.ct`（原先继承 callout 的 1.7，23.8 px）；Chromium 里带行内代码的标题高 21.64 px（等宽字体撑高行框）。PG17 第 2 格核对标题的 `innerText` 是连续的文字、1440 宽时行数的上限与 375 宽时至多四行、不横滚（5.2；变异 t14）。4.2.5 与第 9 节读屏清单里"`display: flex` 的 `summary`"一条随之改。
- **902 callout 里只有标题之后 4 px，块与块之间 0.5rem**（审查 B 的 P4S2B-Q1）：`.nw-callout > * + *` 是 0.5rem，`.nw-callout > :is(.nw-callout-title, summary) + *` 是 0.25rem（样稿 `.ct` 的 4 px 只是标题到正文；两段正文 4 px 读起来像一段多了行距）。
- **903 列表项里的块之间 0.5rem**（审查 B 的 P4S2B-Q2，S2 之前就没有间距）：松散列表的第二段起（`li > p:not(:first-child)`）与列表项里紧接项的那一行的代码块、引用、callout、图与表格的外层、折叠的 callout、分隔线与标题（`li > :is(pre, blockquote, div, details, hr, h1, h2, h3, h4, h5, h6)`）上方 0.5rem，同 callout；嵌套列表的 0.25rem 不动。紧凑列表里项的那一行不是段落，CSS 看不出块之前有没有文字，所以以代码块开头的列表项在代码块上方也多 8 px（照实，10.2）。`:is()` 里原有 `table`，修复核对第 1 轮去掉（审查 B 的 P4S2B-F1-M1）：属性条里列表值是映射的那种（`td > ul > li > table.nw-props`）也被选中，那一块小块高出 8 px、字下沉；正文里的表格在列表项里是 `div.nw-scroll`，照旧在 `div` 里。PG17 第 2 格核对间距（第 5、8 步）与那一块小块（第 6 步）。
- **904 图自己给了颜色时只照它自己的配置画**（审查 A 的 P4S2A-I2；修复之后照实施者第 7 节第 3 条改）：图自己的指令或 frontmatter 指定了不是 `base` 的主题，或给了自己的 `themeVariables`（写不写 `theme: base` 都算）（只给变量、没写主题的，主题怎么取见 906），`initialize` 只给它的主题，不给应用的 `themeVariables` 与 `themeCSS`（M13 之前的样子）；只写 `base`、没给变量，或什么都没写时，照应用的颜色（871）。mermaid 把 `initialize` 给的 `themeVariables` 交给作者指定的主题去推导，应用的灰阶会盖在作者的主题上：亮色应用里作者的 `dark` 节点名对底 1.08:1，暗色应用里 `forest`、`neutral` 1.13、1.05:1；作者的 `base` 加 `primaryColor: #ffcc00` 在暗色应用里是应用的浅字画在黄底上，1.24:1。改后作者的主题读得清（`dark` 10.17，`forest` 15.15，`base` 加变量 8.36）。`render()` 在 `tooLarge` 之后以 `mermaid.parse(source, { suppressErrors: true })` 读出图自己的配置（`Drawer` 加 `parse`，`ownColours`）；`theme` 照旧不进 `secure`（作者的 `base` 加变量照旧有效）。
- **905 焦点按"复制"在所在块里的第几个找回，照实**（审查 A 的 P4S2A-Q1）：同一个顶层块（例如一个有序列表）里有几段代码，别人在拿着焦点的那段之前插入或删去一段时，就地更新之后焦点落到别一段的"复制"上（A 在三种引擎里看到）；读者拿着焦点时另一人在同一个块里增删代码块，窄，照实（决定 869 的"按它在所在块里的第几个找回"）。记进 10.2 与第 9 节。
- **906 图只给了自己的 `themeVariables`、没写主题时，主题按应用的亮暗取 mermaid 的 `dark` 或 `default`，再加作者的变量**（审查 A 的 P4S2A-F1-M1；改 904）：改过的 904 把这一种也算作者自己的颜色，却不给主题，mermaid 取它的 `default`（浅色）：最常见的这种指令只是改字号（`%%{init: {"themeVariables": {"fontSize": "20px"}}}%%`），暗色应用里节点读得清，边与箭头是 `#333`，画在暗色的图框（`#1f2023`）上 1.29:1。M13 之前这种图在暗色应用里是 mermaid 的 `dark`。改后（`5bbf897f`）在暗色应用里是 `dark`：字号 20 px，节点 `#ccc` 在 `#1f2020` 上 10.17:1，边与箭头 lightgrey 对图框 10.88:1；作者给的浅色 `primaryColor`（`#fff4dd`）一样，mermaid 的 `dark`、`default` 的节点底取它们自己的 `mainBkg`、不取 `primaryColor`；亮色应用里两种都是 `default`，`#333` 在 `#ececff` 上 10.83:1，边对白底 12.63:1，三种引擎相同、控制台没有输出。写了 `base` 加变量、或指定了别的主题的，照 904；作者的 `base` 加变量在暗色应用里边是 `#0b0b0b`、对图框 1.21:1，同第 9 节记给 v0.2 的"作者的浅色主题画在暗色图框上"一类。

以下是主持者照 S3 的实施审查定的（负责人可以改判）：

- **907 编辑时大纲的项的文字同阅读时的大纲**（审查 A 的 P4S3A-Q1、B 的 P4S3B-Q1；改 876 的"只去掉 `HeaderMark`"）：`textOf` 沿标题的行内节点读：去掉 `HeaderMark`、`QuoteMark`，强调、行内代码与删除线的标记，链接的括号、地址与标题（`LinkTitle`），引用式链接的标签，HTML 标签与注释（lezer 的 `Comment`），转义的反斜杠（留被转义的字），`==` 与脚注引用 `[^n]`，代码之外的 Obsidian 注释 `%%…%%`（连同里面的字）；字符实体照它显示的字（一次读里同一个实体只解码一次，记在这一次读的表里，读完就丢；只建一个 `textarea`。模块级、不清的表让写者的内容能让编辑者的标签页一直多占内存：1,000 个标题各 300 个不同的数字实体留下约 19 MB，审查 C 修复核对第 2 轮），图片取替代文字；wiki 链接照服务端的 `shown()`（新文件 `W/editor/wiki-links.ts`，照 `server/internal/platform/markdown/obsidian/wikilink.go` 的 split 与 shown、`asset.go` 的说明）：有显示文字取它，否则页名，只有小节时是小节名，页名加小节写作"页名 > 小节"；附件（目标带不是 `.md`、不含空白的扩展名）的嵌入取说明（去掉末尾的尺寸），页的嵌入显示目标；行内代码里的照原样。列出的至多 `upTo` 个之后的标题只判有没有字（`HeaderMark`、`QuoteMark` 之外有没有非空白），不读文字（审查 A 的 P4S3A-F1-M2：20,000 个带行内标记的标题，每次停手重读原是一个 148–168 ms 的长任务）。注释、页的嵌入与 `upTo` 之后的两条是修复核对第 1 轮加的（A 的 P4S3A-F1-M2、F1-N2，`d7fb332c`），扩展名不含空白与每次读新建的表是第 2 轮之后的（B 的 P4S3B-F2-N1、C 的 P4S3C-F2-M1，`bcd1575b`）。L6 的页面格加一个带行内标记的标题，阅读与编辑时项的文字相同。已知的差别照实（4.4 之前的"照实"）：`[see [[A]]](u)`、`[![[img.png]]](u)`（lezer 不把外层读成链接）、行内数学 `$…$`（lezer 没有它的语法）、有定义的简写引用 `[r]`（编辑时 `[r]`，阅读时 r）、带点的页名加显示文字（`![[Node.js|JS]]`、`![[Release 2.0|Rel]]`，当作附件、取了显示文字）与不存在的附件带说明（取了说明；服务端都显示目标，编辑器不知道目标找没找到）、图片替代文字里的 `%%…%%`（服务端当文字，编辑时剪掉）、之前的段落开的块注释里的标题（服务端隐去，编辑时照列）。
- **908 编辑时的大纲只排除闭合的 frontmatter，与服务端一致**（审查 A 的 P4S3A-Q2）：起点在闭合的 frontmatter（`frontmatterRange` 的 `closed`）里的标题不列；还没闭合的照正文读，同服务端与阅读时的大纲（页开头是一条水平线、之后没有闭合的 `---` 时，服务端把整篇读成正文，初稿的编辑时大纲漏掉其中的标题）。`proseLines`、`markupMarks` 不变：正在写的 frontmatter 照 874 整块等宽、次要色。
- **909 对比与冲突视图的等宽是编辑器的等宽字体、13 px**（审查 B 的 P4S3B-Q2）：`editorTheme` 的 `.cm-scroller` 写 `font-family: var(--nw-editor-mono)`、`font-size: 13px`（行高照旧 1.6），不再用 CodeMirror 基底的通用 `monospace`（Chromium 画成 Courier，Firefox 中文是 16 px）；源编辑器的 `Prec.high(proseTheme)` 照旧给它比例字体。
- **910 查找面板（Mod+F）的样子放进 S3，只用变量**（审查 B 的 P4S3B-I1，S3 之前就有）：编辑器从不标 dark，CodeMirror 亮色的基底一直生效，暗色下查找框是白底配 `#e8e8ea` 的字（约 1.2:1），按钮是浅灰的渐变。`editorTheme` 另写：`.cm-panels` 的底随编辑器、上两角随编辑器的 12 px 圆角；输入框照应用的输入框（30 px 高、13 px 字、9 px 圆角，没聚焦时 `--surface-2`、聚焦时 `--surface` 加 2 px 的环，宽 10.5rem：各引擎输入框的默认宽不同，Firefox 中文 1440 宽时面板原成三行，审查 B 的 P4S3B-F1-N1），按钮照小号 outline 按钮（没有渐变），`label` 次要色，关闭按钮另画；强制色下聚焦的输入框与按钮另有透明的 2 px 外框；对比视图的 gutter（同一个亮色基底的 `#f5f5f5`）透明、无边、次要色。L5 (page) 加查找面板一步：亮暗的查找框（聚焦的与没聚焦的）、按钮的字与 `label` 对底，下限取实量 − 0.1，按钮没有渐变，面板的底同编辑器的底。
- **911 补全照 878 比菜单密，不改**（审查 B 的 P4S3B-Q3）：补全项 26.2 px 高、弹层 4 px 的内边距，应用的菜单 32 px、5 px；补全跟着打字、列得多，密一些合适。

## 3. 服务端

无。P4 只改网页与 e2e；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 是合并之前的门禁（7.3）。

## 4. 网页

### 4.1 S1 页头、属性条与右栏

#### 4.1.1 文件

| 文件 | 改什么 | 提交 |
|---|---|---|
| `W/app/layout.tsx` | `main` 加 `xl:px-10`（决定 855） | 1 |
| `W/app/column-top.tsx`（新） | `ColumnTopContext`、`ColumnTop`（决定 857） | 1 |
| `W/app/routes.tsx` | `pages/:pageId` 加 `handle: { column: true }` | 1 |
| `W/app/settings-page.tsx` | 标题之前 `<ColumnTop />`；注释里"P4 decides the main's own"改成 `main` 的实际留白 | 1 |
| `W/pages/notebook/notebook-layout.tsx` | 命中的路由带 `column` 或 `settings` 时，横幅经 `ColumnTopContext` 交给页，否则照旧画在 `Outlet` 之前 | 1 |
| `W/pages/notebook/settings-layout.tsx` | `shown === undefined` 那一支在 `Outlet` 之前 `<ColumnTop />`（决定 892） | 1 |
| `W/app/narrow.ts` | 加 `useBeside()`（`(width >= 80rem)`，没有 `matchMedia` 时答"是"，同文件的 `useNarrow` 的写法；决定 886） | 1 |
| `W/pages/page/page-layout.tsx` | `PageShell` 的两栏、头一行、内容列与 32 px 的标题（4.1.2–4.1.4）；`EditButton`（决定 865）；右栏藏起时把右栏里的焦点交给开关（决定 859）；`PageLayout` 的另两个分支（`NotLoaded`、`PageGone`）之前 `<ColumnTop />`（提交 1）；右边一组定宽、`min-h-8`、它的第一个盒子 `meta`（决定 894，提交 1）；`meta`、两个 `span` 与右边一组的回调 ref、`EditHeadSlots`（4.1.9，提交 2：提交 1 里 `meta` 盒子在、只放 `HeadMeta`，没有 ref，也没有两个 `span`） | 1、2 |
| `W/pages/page/page-head.tsx`（新） | `HeadMeta`（决定 858）、`PanelToggle`（决定 859）；提交 2 加 `EditHeadSlots` 上下文与 `useEditHeadSlots` | 1、2 |
| `W/design-tokens.test.ts:298-316` | `noColour` 加 `"text-end"`（头一行 `meta` 盒子的字靠右；守卫把 `text-` 开头的类当颜色读，同审查 B 的 I4），注释写明是对齐 | 1 |
| `W/pages/page/breadcrumbs.tsx` | `Breadcrumbs`、`BreadcrumbTrail` 加可选的 `className`（给 `nav`）；字号、颜色（4.1.3） | 1 |
| `W/pages/page/page-menu.tsx:46` | 触发 `size="icon-sm" shape="pill"` | 1 |
| `W/pages/page/page-panel.tsx` | `id`、`open` 两个属性，`aside` 的类与 `data-page-panel`（决定 860），小节的图标；交给大纲 `active`（展开且 `useBeside()`） | 1 |
| `W/pages/page/panel-section.tsx` | `icon`；`summary` 的样子与箭头（决定 863） | 1 |
| `W/pages/page/page-outline.tsx` | 读到的一节、竖线与 `aria-current`、`memo` 的 `OutlineItem`、`sameView` 加比 `active`、点按（不带修饰键）时 `pin`、右栏只滚到刚好露出、读者在右栏里动过就不跟（`watchReader`）、`line-clamp-2`（决定 861、862、886、893）；导出 `listedUpTo`（S3 用，S1 不导出） | 1 |
| `W/pages/page/current-heading.ts`（新） | `useCurrentHeading`（决定 861、887、893） | 1 |
| `W/pages/page/page-backlinks.tsx`、`page-properties.tsx`、`page-versions.tsx` | 小卡片、两列的属性、版本一行"时间 · 谁写的"与右端的标记（4.1.6 写明元素与次序）；链接不加下划线（解析不到的虚线照旧）；`Mark` 的 `current`；右栏的"谁写的"进 `bdi`（决定 863、868）；随之改的两格：`page-versions.test.tsx:103`、`version-restore.test.tsx:156`（7.1） | 1 |
| `W/stores/preferences.store.ts` | `panelOpen`、`setPanelOpen`、键 `nwiki.panel`（决定 859） | 1 |
| `W/reading/property-roles.ts`（新）、`W/reading/enhancement.ts` | `propertyRoles` 与它在注册表里的位置；`readingEnhancements` 的注释加一句"M13 the properties' roles (propertyRoles)"（决定 864，审查 C 的 M11） | 2 |
| `W/reading/reading.css` | 属性条一节（4.1.8），替换 `:263-275` | 2 |
| `W/pages/page/page-edit.tsx`、`page-editing-bar.tsx` | 门户进三个落点（状态进 `meta`）、`stayed()`、编辑说明（决定 866、867、894） | 2 |
| `W/styles.css` | `.nw-edit-status[data-saved]::before` 的 ✓（决定 866） | 2 |
| `W/pages/page/edit-lock-note.tsx` | 样子与 `bdi`（决定 868）；版本页（`version-page.tsx:198`）用同一个组件，文件不改 | 2 |
| `W/pages/page/asset-dialogs.tsx:221` | `space-y-2` → `space-y-1.5`（决定 882） | 2 |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | `page.editedBy`（提交 1）、`page.editingNote`（提交 2） | 1、2 |

#### 4.1.2 页的两栏与留白

`PageShell` 的根：

```tsx
<div className="md:-mt-3 md:pb-4 xl:flex xl:items-start">
  <div className={cn("min-w-0 xl:flex-1", open && "xl:pe-10")}>
    {/* 头一行，4.1.3 */}
    <div className="mx-auto w-full max-w-180">
      <ColumnTop />
      <h1 ref={heading} tabIndex={-1} aria-describedby={…}
          className="mb-3.5 min-w-0 text-[32px] leading-tight font-[650] tracking-[-0.012em] break-words outline-none">
        {page.name}
      </h1>
      {restored !== undefined && (
        <output id={restoredNote} className="-mt-2 mb-3.5 block text-[13px] text-muted-foreground">…</output>
      )}
      <div className="space-y-6">
        {/* 阅读：拒绝、闲置退出、锁说明、来源的说明、AttachmentDrop > ReadingView；编辑：PageEdit。然后子页面、附件一节，同今天 */}
      </div>
    </div>
  </div>
  <PagePanel id={rail} open={open} notebook={notebook} page={page} editing={!reading} href={href} />
</div>
```

`open` 是 `preferences.panelOpen`（`PageShell` 已是 `observer`），`rail` 是 `useId()`。右栏藏起时（`!open && useBeside()`）一个 `useLayoutEffect` 看右栏里有没有焦点（`aside.contains(document.activeElement)`），有就 `toggle.focus({ preventScroll: true })`（决定 859；开关的 ref 由 `PanelToggle` 转出）。内容列 720 px（样稿 `.doc`），1280 px 宽时内容区只有 696 px，列随之窄（第 1 节 (b)）。`main` 的 `xl:px-10` 与外框的 `md:-mt-3 md:pb-4`：头一行离 `main` 的顶 12 px，页底 40 px；375 宽时照旧 `p-6`（细栏在上面）。右栏的负外边距（决定 860）与它们配套：宽屏上右栏的盒子正好从窗口顶到 `main` 的底，内容比窗口短时页也不会多出滚动（12 + 100svh − 52 + 16 + 24 = 100svh）。

#### 4.1.3 头一行

```tsx
<div className="mb-7.5 flex min-h-9 flex-wrap items-center gap-x-3 gap-y-1.5">
  <Breadcrumbs className="min-w-0 flex-[1_1_14rem]" notebook={notebook} ancestors={…} page={page} href={href} />
  {/* The same width whatever the mode, 32px high with no button in it (decision 894): the row's lines are its width's alone. */}
  <div ref={setGroup} className="ms-auto flex min-h-8 min-w-0 flex-[0_1_26rem] items-center justify-end gap-1.5">
    <div ref={setMeta} className="me-1 min-w-0 flex-1 text-end text-[12.5px] leading-4 text-faint">
      {reading && <HeadMeta notebook={notebook} page={page.id} />}
    </div>
    <span ref={setBefore} className="contents" />   {/* 提交 2 */}
    <PanelToggle ref={toggle} controls={rail} />
    <span ref={setAfter} className="contents" />    {/* 提交 2 */}
    {reading && !gone && <PageMenu … />}
    {reading && editable && <EditButton edit={edit} left={() => heading.current?.focus({ preventScroll: true })} … />}
  </div>
</div>
```

- **面包屑**：`ol` 换成 `flex flex-wrap items-center gap-x-1 text-compact text-faint [overflow-wrap:anywhere]`，每一步的链接 `hover:text-foreground`（不加下划线，4.7），当前一项 `font-medium text-muted-foreground`，`Separator` 是 `pr-1 font-normal text-faint`（照旧 `aria-hidden`；`e2e/fixtures/wiki-pages.ts:162-171` 的 `breadcrumbs` 读每项里不带 `aria-hidden` 的子元素，结构不变）。`flex-[1_1_14rem]`：宽屏上面包屑占左边、右边一组靠右；窄到放不下时右边一组折到下一行、仍靠右。右边一组不折行（初稿是 `flex-wrap`），定宽 `flex-[0_1_26rem]`、最小高 `min-h-8`（决定 894）：它的宽不随模式与内容变、高不随组里有没有按钮变，于是头一行在行宽 652 px 起一行、更窄时两行，进出编辑、"编辑过"到来、"保存"出现与隐去都不改行数（第 1 节 (j)；修订稿只给"编辑过"预留 `w-64`，右边一组阅读时约 380 px、编辑时约 130–190 px，行宽 470–610 px 时阅读两行、编辑一行，375 px 时打第一个字变两行，审查 A 的 R1-I1）。第一个盒子（`meta`）`flex-1`，宽是组里按钮之外的全部；阅读时放"编辑过"，编辑时放状态（4.1.9）。回收站的预览与变更集页的面包屑随 `BreadcrumbTrail` 同换（第 9 节）。
- **`HeadMeta`**（决定 858）：`<p title={读到时的全时间} className="truncate">{读到时 <Isolated … />}</p>`，在 `meta` 盒子里：阅读时一直在，读到之前与没有版本时是空的；名字长时省略。盒子的 `text-end` 照旧要进 `design-tokens.test.ts` 的 `noColour`（守卫把 `text-` 开头的类当颜色读，4.5）。
- **`PanelToggle`**（决定 859）：`observer`，转出 ref；`<Button ref variant="ghost" size="icon-sm" shape="pill" className="hidden xl:inline-flex" aria-label={t("page.panel")} aria-expanded={preferences.panelOpen} aria-controls={controls} onClick={() => preferences.setPanelOpen(!preferences.panelOpen)}><PanelRight /></Button>`。
- **页面菜单**：触发 `<Button ref={trigger} variant="outline" size="icon-sm" shape="pill" aria-label={t("page.menu")}><Ellipsis /></Button>`。
- **`EditButton`**（决定 865）：`<Button ref={edit} size="sm" shape="pill" aria-busy=… aria-describedby=… aria-disabled=… onClick=…><Pencil aria-hidden />{t("page.edit")}</Button>`，同一个组件里 `useFocusLeaving(edit, left)`，`left` 以 `preventScroll` 给标题焦点；属性与今天的"编辑"相同（`page-layout.tsx:358-369`）。
- 按钮都是 30 px，头一行 `min-h-9`（36 px），右边一组 `min-h-8`（32 px）：按钮出现、消失（"编辑"随角色、开关随宽度、S3 的"保存"随需要）时行高不变，行数也不变（右边一组定宽）。组里一个按钮也没有的三种时候——点了"编辑"、内容的 `GET` 答回之前（`begin` 不等内容，`page-edit.tsx:407-417` 那时画 `NotLoaded`，没有 `PageEditingBar`），失锁（`:421-429`），读取失败——窄于 1280 px 时开关又不显示，组里只剩空的 `meta` 盒子：没有最小高时，头一行是两行的宽度下它那一行塌成 0 高，标题上跳 16 px（审查 A 的 R2-I2）；两行的状态（32 px）在那一行里把按钮的 30 px 撑到 32 px，标题下移 2 px（审查 B 的 R2-N1）。有了 `min-h-8`，两者都不动（第 1 节 (n)：行宽 327–651 px 时标题的 `y` 在阅读、编辑、组里没有按钮、两行的状态下都是 84，653 px 起都是 66）。

#### 4.1.4 内容列与升级横幅

`W/app/column-top.tsx`：

```tsx
/** What a shell puts at the top of its page's column: a notebook's upgrade banner (M13/P4/S1, decision 857). */
export const ColumnTopContext = createContext<ReactNode>(null);
export function ColumnTop() {
  return use(ColumnTopContext);
}
```

`NotebookLayout`：`const columned = useMatches().some((match) => { const handle = match.handle as ColumnHandle | undefined; return handle?.column === true || handle?.settings === true; })`（`type ColumnHandle = { column?: boolean; settings?: boolean }`，同 `settings-origin.ts:18` 读 `handle` 的写法；初稿的 `isObject` 在代码里没有，审查 C 的 N5）；`columned` 时 `<ColumnTopContext value={<UpgradeBanner notebook={notebook} />}><Outlet key={notebook.id} /></ColumnTopContext>`，否则照旧 `<UpgradeBanner … /><Outlet … />`。横幅自己的 `mb-6` 照旧，它在内容列里、标题之前；设置页里同样在 696 px 的列里、标题之前；笔记本设置的地址没对上一节时（`NotebookSettingsLayout` 的 `shown === undefined`，如 `settings` 的 index 重定向的那一次渲染）在 `Outlet` 之前（决定 892）。`upgrade-banner.test.tsx` 的格经 `findByRole("article")` 与 `bannerOf` 找横幅，不看位置；新加一格核对它在页的内容列里（7.1）。

#### 4.1.5 右栏与开关

`PagePanel({ id, open, … })`：`<aside id={id} aria-label={t("page.panel")} data-open={open} data-page-panel className="…决定 860 的类…">`，子元素与次序照旧（大纲只在阅读时，S3 起编辑时另有大纲），小节之间 `space-y-6`（样稿 24 px）；`<PageOutline … active={open && beside} />`（`beside = useBeside()`，决定 886；`memo` 的 `sameView` 比它，4.1.7）。收起的右栏照旧挂着（读取照旧、事件照旧重读），只是 `display: none`：再展开时不重读、不闪，读到的一节在展开的那一帧量一次。更窄时 `data-open` 不起作用，开关不显示，右栏在正文之后、读者看不到它与正文并排，读到的一节不量。注释照新的样子改（"From xl up it is beside the page's content, the window's height, held at the top…"）。

`PreferencesStore`：

```ts
const panelKey = "nwiki.panel";
panelOpen: boolean;                       // constructor: read(storage, panelKey) !== "closed"
setPanelOpen(open: boolean): void {       // this.panelOpen = open; write(storage, panelKey, open ? "open" : "closed")
```

#### 4.1.6 小节

```tsx
export function PanelSection({ title, icon: Icon, summaryRef, children }: { title: string; icon: LucideIcon; … }) {
  return (
    <details open className="group space-y-2">
      <summary ref={summaryRef}
        className="flex cursor-pointer list-none items-center gap-1.5 rounded-row text-xs text-faint [&::-webkit-details-marker]:hidden">
        <Icon aria-hidden className="size-3.25 shrink-0" />
        <h2 className="font-normal">{title}</h2>
        <ChevronDown aria-hidden className="ms-auto size-3.5 shrink-0 transition-transform duration-150 group-not-open:-rotate-90" />
      </summary>
      {children}
    </details>
  );
}
```

`summary` 的焦点圈照 `styles.css` 的全局 `:focus-visible`（2 px 的琥珀外框，`rounded-row` 给它圆角）。各节的内容：

- **反链**：`ul.space-y-2`，每项 `li.rounded-control.border.border-border.bg-surface.px-2.5.py-2.dark:bg-surface-2`；名字 `div.break-words` 里的 `Link className="text-[13px] font-medium hover:underline"`，次数 `text-faint`；摘要 `ul.mt-0.5.space-y-0.5.text-xs.leading-normal.text-faint`，每行 `li.line-clamp-2.break-words`；"更多反向链接"是 `variant="outline" size="sm" shape="pill"`；"没有反链"是 `text-[13px] text-faint`。
- **属性**：`dl.grid.grid-cols-[4rem_minmax(0,1fr)].gap-x-2.5.gap-y-1.5.text-[12.5px]`，每个键值的 `div` 加 `contents`，`dt.break-words.text-faint`，`dd.break-words.text-muted-foreground`，值里的链接 `hover:underline`；说明文字（无效、没有）`text-[13px] text-faint`。
- **版本**：两句说明 `text-xs text-faint`；`ul.space-y-1.5.text-[13px]`，每项一行（样稿 `.ver`，决定 863；审查 B 的 R1-N1、C 的 R1-M7）：

  ```tsx
  <li key={version.id} className="flex items-baseline gap-2">
    {/* The time and who wrote it one run of text, wrapping as one; the marks at the first line's end (review B's R2-N2). */}
    <div className="min-w-0 flex-1 break-words">
      <Link … className="text-muted-foreground hover:text-foreground"><time …>{when}</time></Link>
      {/* The dot drawn before it, in no text: what the tests read of the writer stays "Ada · Edit". */}
      <p className="inline text-muted-foreground before:mx-1 before:content-['·']"><VersionBy version={version} /></p>
    </div>
    {(newest || version.editing) && (
      <div className="ms-auto flex shrink-0 items-center gap-1">
        {newest && <Mark current>{t("version.current")}</Mark>}
        {version.editing && <Mark>{t("version.editing")}</Mark>}
      </div>
    )}
  </li>
  ```

  次序：时间链接与谁写的（`VersionBy` 是 `versionBy` 的 `Isolating` 写法，名字在 `bdi` 里，决定 868）在一个 `div` 里排成一段字（`p` 是 `inline`），然后是标记。名字长时谁写的接着时间往下折，不再整个掉到下一行、以"·"开头；标记的一组 `shrink-0`、`ms-auto`，一直在第一行的最右（审查 B 的 R2-N2，B 照这一写法在三种引擎里画过；第 1 轮的 `flex-wrap` 写法下，1440 宽时右栏 217 px 里约 40 个字符的名字就让谁写的掉到第二行，最新版本的"当前"再掉到第三行）。"·"两边各 4 px（`before:mx-1`），与同一行里"Ada · 编辑"的空格相近（第 1 轮是 6 px）。字号与颜色照样稿 `.ver`：谁写的与时间同是 13 px（`ul` 的 `text-[13px]`）、`--muted-foreground`（样稿的 `--text-2`），不再是 12 px 的 `--faint`。设计者定，理由：一、样稿整行一种字，"时间 · 谁"读作一句；二、排成一段字之后，一行里两种字号的"·"间距与基线对不齐；三、时间是链接，悬停时变正文色，不靠颜色与谁写的分开（右栏的链接都不加下划线，4.7）。B 画的那一份就是 13 px（继承 `ul`），这里只换了颜色，排版不变。标记包在 `div` 里而不是 `span` 里：`page-versions.test.tsx` 的 `listed()`（`:20-27`）把 `li` 里的 `span` 读成标记、第一个 `p` 读成谁写的，`writers()`（`:179-182`）读 `p`；这一写法里 `li` 之下的 `span` 只有 `Mark`（`time` 不是 `span`，`Isolated` 写的是 `bdi`，`i18n/isolated.tsx:31-35`），`p` 只有一个，两者照旧（`:61-66`、`:159`、`:217` 起的期望不变）。随之改的只有读整项 `textContent` 的两格：`page-versions.test.tsx:103` 的 `"5分钟前当前编辑中Ada · 撤销"` 换成 `"5分钟前Ada · 撤销当前编辑中"`，`version-restore.test.tsx:156` 的 `/CurrentAda · Restore$/` 换成 `/Ada · RestoreCurrent$/`（都在 S1 提交 1）。"更多版本"同反链的按钮。

#### 4.1.7 大纲与读到的一节

`W/pages/page/current-heading.ts`（第 1 节 (k)、(l) 在分离工作树里照再修订稿的这一份跑过；三修加的 `live` 与收起时结束 `pin` 没有跑）：

```ts
/** The heading read (decisions 861, 887, 893): the id of the last of ids (the outline's, in order) above the line. */
export function useCurrentHeading(ids: readonly string[], active: boolean): { current: string | undefined; pin: (id: string, link: Element) => void } {
  const [current, setCurrent] = useState<string>();
  // What was set last: a frame that reads the same sets nothing, so that no update comes from a frame where nothing moved.
  const shown = useRef<string | undefined>(undefined);
  const show = useCallback((id: string | undefined) => { if (shown.current !== id) { shown.current = id; setCurrent(id); } }, []);
  const pinned = useRef<{ id: string; reader: ReturnType<typeof watchReader>; seen: boolean }>(undefined);
  const ask = useRef<() => void>(() => {});
  const live = useRef(false);
  useLayoutEffect(() => {
    // Folded away or not beside the page, or no heading listed (nothing to read, review A's P4S1A-N1): none and no pin,
    // so that shown again it answers what it measures (review A's R1-N5, R2-N2).
    const reading = active && ids.length > 0;
    live.current = reading;
    if (!reading) { pinned.current?.reader.end(); pinned.current = undefined; show(undefined); return; }
    let frame = 0;
    const read = (): string | undefined => {
      const { innerHeight, scrollY } = window;
      const { scrollHeight } = document.documentElement;
      // The end's rule where the page scrolls at all (decision 893): a page shorter than the window is never at its end.
      const atEnd = scrollHeight > innerHeight + 1 && scrollY + innerHeight >= scrollHeight - 1;
      const line = atEnd ? innerHeight : innerHeight / 3;
      const pin = pinned.current;
      if (pin !== undefined && !pin.reader.acted()) {
        const top = document.getElementById(pin.id)?.getBoundingClientRect().top;
        const inView = top !== undefined && top >= 0 && top < innerHeight;
        pin.seen ||= inView;
        // Until the view has scrolled it in (the router's commit may come a frame late: review A's R1-M1), and while in.
        if (inView || !pin.seen) return pin.id;
      }
      if (pin !== undefined) { pin.reader.end(); pinned.current = undefined; }
      for (let i = ids.length - 1; i >= 0; i--) {
        const heading = document.getElementById(ids[i]);
        // A heading in a closed details (a folded callout) answers a box all the same (review A's I1): not read.
        if (heading === null || heading.closest("details:not([open])") !== null) continue;
        if (heading.getBoundingClientRect().top <= line) return ids[i];
      }
      return undefined;
    };
    const measured = () => { frame = 0; show(read()); };
    const asked = () => { frame ||= requestAnimationFrame(measured); };
    ask.current = asked;
    // The first in the commit, before the paint: no frame later sets it (section 1 (k)).
    show(read());
    addEventListener("scroll", asked, { passive: true });
    addEventListener("resize", asked);
    // The view, not the main (WebKit's undelivered resize while KaTeX's stylesheet loads, f698d37e): what resizes beside
    // it alone (a lock's note above it, the subpages and the attachments below) is measured as the window next scrolls.
    const view = document.querySelector("main article.nw-reading");
    const resized = typeof ResizeObserver === "undefined" || view === null ? undefined : new ResizeObserver(asked);
    if (view !== null) resized?.observe(view);
    return () => { cancelAnimationFrame(frame); removeEventListener("scroll", asked); removeEventListener("resize", asked); resized?.disconnect(); ask.current = () => {}; };
  }, [ids, active, show]);
  useEffect(() => () => pinned.current?.reader.end(), []);
  const pin = useCallback((id: string, link: Element) => {
    // Not measured (folded away, or below the page): nothing marked, nothing watched (review A's R2-N2).
    if (!live.current) return;
    pinned.current?.reader.end();
    // The press on the link itself is no move of the reader's (watchReader's on); marked in the press's own handler.
    pinned.current = { id, reader: watchReader({ on: link }), seen: false };
    show(id);
    ask.current();
  }, [show]);
  return { current: active && ids.length > 0 ? current : undefined, pin };
}
```

（`ids` 是 `useMemo` 出来的数组，只含大纲列出的那些（至多 `listedUpTo`），视图的 HTML 变了才换。视图在同一次提交的 layout effect 里把 HTML 写进 `article`（`server-html.ts:82-171`），它在大纲之前（右栏在内容之后），所以第一次量时标题已在；之后正文长高、图与公式画出来，视图的 `ResizeObserver` 再排一次（照实，原写 `main` 的，见 4.2 之前的"照实"）。设状态只在三处：layout effect 里的第一次（在那次提交里，测试的 `act` 之内）、点按里的 `pin`、之后某一帧量到的值变了（jsdom 里不排版，什么也没动，值不变）；第 1 节 (k)：照修订稿在第一帧才设，vitest 里它会落进别的格 `act` 里等 `findBy` 的窗口，React 报"not configured to support act"，`setup.ts` 判红，`src/app/arrival.test.tsx` 的焦点一格时有时无地红。点了一项之后先标它：视图滚过去之前、进了视口之后一直是它；读者再滚、再按键或按下（`watchReader`），或它进过视口之后又出了，就照线来算。不活跃时 `pin` 什么也不做，`active` 变假时连点了的那一项一起结束：窄于 1280 px 时大纲在正文之后、点得到，之后窗口放宽，变真的那次量不答那一项（审查 A 的 R2-N2）。）`PageOutline`（新的属性 `active`；`memo` 的 `sameView` 加比它，见下：只比笔记本与页时 `PagePanel` 的 `open`、`beside` 变了传不进来，存着收起时打开页、展开之后读到的一节永远不标、右栏也不跟，反过来收起之后照旧每次滚动都量，审查 A 的 R2-I1）：

```tsx
const { current, pin } = useCurrentHeading(ids, active);
const list = useRef<HTMLUListElement>(null);
// The reader moved in the right column (decision 886): from then on, while this list is up, it follows no more.
const reader = useRef<ReturnType<typeof watchReader>>(undefined);
// Watched from the list's own mounting: the page's view, and the outline with it, may come after the page (review A's R1-I2).
const watched = useCallback((element: HTMLUListElement | null) => {
  list.current = element;
  if (element === null) return;
  const watching = watchReader({ on: element, within: "[data-page-panel]" });
  reader.current = watching;
  return () => { watching.end(); reader.current = undefined; };
}, []);
useLayoutEffect(() => { /* 见下：只滚到刚好露出 */ }, [current]);
…
<ul ref={watched} className="border-l border-border text-[13px]">
  {listed.map(({ id, level, text }) => (
    <OutlineItem key={id} id={id} indent={level - top} text={text} current={id === current} pin={pin} />
  ))}
</ul>
{more > 0 && <p className="mt-1.5 ps-3 text-xs text-faint">{t("page.moreHeadings", …)}</p>}

/** OutlineItem is a heading of the outline, drawn again only as it becomes or stops being the current one. */
const OutlineItem = memo(function OutlineItem({ id, indent, text, current, pin }: …) {
  return (
    <li data-current={current || undefined} style={{ paddingLeft: `${indent * 0.75}rem` }}
        className={cn("-ms-px border-l-2 py-1 forced-colors:border-l-0", current ? "border-accent" : "border-transparent")}>
      <Link to={{ hash: encodeURIComponent(id) }} aria-current={current ? "location" : undefined}
            onClick={(event) => { if (plainPress(event)) pin(id, event.currentTarget); }}
            className={cn("ps-3 pe-1 leading-snug break-words line-clamp-2",
              current ? "font-medium text-foreground forced-colors:underline" : "text-muted-foreground hover:text-foreground")}>
        {text}
      </Link>
    </li>
  );
});
/** plainPress is a press of the main button with no modifier, as the router's isModifiedEvent: with one it opens a tab, a window
    or a download, and the view does not scroll to it (review A's R2-N1). MouseEvent is React's (`type MouseEvent` from "react",
    as subpage-list.tsx has it; review A's R3-N2). */
const plainPress = (event: MouseEvent) =>
  event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;

/** sameView tells whether the outline's props are those of the same page's view, measured or not (review A's R2-I1). */
function sameView(before: OutlineProps, after: OutlineProps): boolean {
  return before.notebook.id === after.notebook.id && before.page === after.page && before.active === after.active;
}
```

（React 19 的回调 ref 可以返回清理，`watched` 是稳定的 `useCallback`，列表元素不换时它不重跑。列表在大纲有项时才挂上，大纲变空又有项时读者的记录重起，与换页同；进编辑时大纲卸下、回到阅读时重新挂上，也重起（照实，决定 897）。）

竖线：`ul` 左边 1 px 的 `--border`，当前项的 `li` 左边 2 px 的琥珀压在它上面（`-ms-px`）；按级的缩进在 `li` 的内边距里，竖线一直在 `ul` 的左边（样稿 `.outline li.sub`）。上下的 4 px 在 `li` 上，不在带 `line-clamp-2` 的链接上（审查 B 的 M1：同在链接上时 Chromium 把第三行的字头画进下内边距）。强制色下边框的颜色被换掉，当前项去掉粗线、以下划线标出（第 1 节 (b)）。

右栏只滚到刚好露出（决定 886）：`useLayoutEffect(…, [current])` 里，`current` 有值、`reader.current?.acted()` 为假、右栏（`list.current.closest("aside")`）自己滚动（`scrollHeight > clientHeight`）时，取当前项的 `li`（`[data-current]`）与右栏的盒子：项的顶在右栏的顶之上时 `aside.scrollTop -= 右栏的顶 − 项的顶`，项的底在右栏的底之下时 `aside.scrollTop += 项的底 − 右栏的底`，都不是时不动。第一次量出当前项（打开带锚点的地址）也是"变了"。读者在右栏里滚轮、触摸、按下（大纲的项除外）、焦点在右栏里按键之后不再跟（第 1 节 (h)）；Firefox 里拖右栏的滚动条不发事件，照旧跟（同 P2 对左栏的说法）。

#### 4.1.8 属性条（提交 2）

`reading.css` 的属性表一节（替换 `:263-275`）：

```css
/* The properties as a strip (00-M13 4.5.2): each key and value a chip, a list's items a chip each, wrapping; a nested
   table a run of its own. The table and its list keep their roles by propertyRoles: changed in display, WebKit with
   VoiceOver drops them (M13/P4/S1, decision 864). */
.nw-reading .nw-scroll:has(> table.nw-props) { margin: -2px; padding: 2px; }
.nw-reading > .nw-scroll:first-child:has(> table.nw-props) + * { margin-top: 1.875rem; }
.nw-reading table.nw-props { display: block; font-size: 0.78125rem; line-height: 1.5; }
.nw-reading table.nw-props > tbody { display: flex; flex-wrap: wrap; gap: 6px; }
.nw-reading table.nw-props tr,
.nw-reading table.nw-props td > ul > li {
  display: inline-flex; align-items: baseline; gap: 5px; max-width: 100%; min-height: 24px; padding: 2px 9px;
  border: 1px solid var(--border); border-radius: 9999px; background: var(--surface-2);
}
.nw-reading table.nw-props :is(th, td) { padding: 0; border: 0; }
.nw-reading table.nw-props th { color: var(--faint); font-weight: 400; text-align: start; white-space: nowrap; }
.nw-reading table.nw-props td { min-width: 0; color: var(--foreground); overflow-wrap: anywhere; }
.nw-reading table.nw-props tr:has(> td > ul) { align-items: center; padding: 0; border: 0; background: none; }
.nw-reading table.nw-props td > ul { display: flex; flex-wrap: wrap; gap: 6px; padding: 0; list-style: none; }
.nw-reading table.nw-props td > ul > li { margin: 0; }
.nw-reading table.nw-props table.nw-props { display: inline-block; }
.nw-reading table.nw-props table.nw-props > tbody { display: inline-flex; gap: 0 10px; }
.nw-reading table.nw-props table.nw-props tr { min-height: 0; padding: 0; border: 0; background: none; }
```

（`.nw-scroll` 的 2 px 内边距留住小块的焦点圈；属性条在正文最前时与之后的块隔 30 px，样稿 `.props`。）列表值的那一行不画小块，键照旧在前面（灰字），每项一个小块。嵌套的表（`table.nw-props table.nw-props`）排成一段不折的行；宽过显示时外层属性表的那一层 `.nw-scroll`（服务端只包外层，`platform/markdown/render.go:72`）照旧横向滚动，是名为"属性"的区域（13.2 第 24 条）。

`W/reading/property-roles.ts`：

```ts
/** The roles a properties table keeps as the strip sets it otherwise (decision 864). */
const roles: [selector: string, role: string][] = [
  ["table.nw-props", "table"], ["table.nw-props > tbody", "rowgroup"], ["table.nw-props tr", "row"],
  ["table.nw-props th", "rowheader"], ["table.nw-props td", "cell"],
  ["table.nw-props td > ul", "list"], ["table.nw-props td > ul > li", "listitem"],
];
export const propertyRoles: Enhancement = (container) => {
  const given: Element[] = [];
  for (const [selector, role] of roles) {
    for (const element of container.querySelectorAll(selector)) {
      if (!element.hasAttribute("role")) { element.setAttribute("role", role); given.push(element); }
    }
  }
  return () => { for (const element of given) element.removeAttribute("role"); };
};
```

（`tr`、`th`、`td` 的选择器也选到嵌套表里的，那是想要的；值里不是 `nw-props` 的表——写者自己的 HTML 表——服务端不放进属性值。）就地更新时清理在比对之前，比对的模板里没有这些 `role`（`patch.ts` 比的是读到的 HTML），换上的新块由重跑补上。

#### 4.1.9 编辑时的头一行与说明（提交 2）

- `page-head.tsx`：`EditHeadSlots = createContext<{ group: HTMLElement | null; meta: HTMLElement | null; before: HTMLElement | null; after: HTMLElement | null }>(…全是 null…)`；`PageShell` 以 `useState` 的四个回调 ref 记下 4.1.3 的右边一组、`meta` 盒子与两个 `span`，`<EditHeadSlots value={{ group, meta, before, after }}>` 包住内容区。
- `PageEditingBar`（同名，仍是 `observer`）：

  ```tsx
  const { meta, before, after } = useEditHeadSlots();
  return (
    <>
      {/* In the box of "Edited by", its type and colour the box's (decision 894). */}
      {meta && createPortal(
        <output data-saved={status === t("page.saved") || undefined}
          className={cn("nw-edit-status block [overflow-wrap:anywhere]", failed !== undefined && "text-destructive")}>{status}</output>,
        meta)}
      {before && createPortal(
        <Button variant="outline" size="sm" shape="pill" onClick={save}>{t("page.save")}</Button>, before)}
      {after && createPortal(
        <Button size="sm" shape="pill" aria-busy=… aria-disabled=… onClick={leave}>{t("editor.done")}</Button>, after)}
    </>
  );
  ```

  （S3 起"保存"换成 `SaveButton`、不需要时 `invisible`，决定 877、885、894、895，4.3.5。）状态在 `meta` 盒子里，盒子的宽在阅读与编辑时由右边一组的定宽定下，状态的有无、长短都不改头一行的行数；两行以内的说明在 `leading-4` 下是 32 px，正好是右边一组的 `min-h-8`，头一行不变（第 1 节 (j)、(n)）。375 宽时英文的三种失败说明——"The network was too slow to send the page in time. Save again."、"The page has a NUL character, which cannot be saved: take it out to save it."、"Cannot reach the server for now. You are still signed in; try again in a moment."——在 193 px 的盒子里是三行，失败出现时头一行高 16 px；中文都在两行以内，820 宽起英文也是（第 1 节 (n)）。这是失败带来的变化，不是进出编辑，照实记下，7.4 截一张；不截断：窄屏的触屏读不到 `title`，NUL 那一句要读全才知道怎么办（审查 A 的 R2-N3）。S3 起"保存"不需要时 `invisible`、照旧占着位置，状态的右边离开关（窄于 1280 px 时是"完成"）约 70 px，样稿是 8 px：记成与样稿的偏离（决定 896）。`data-saved` 以状态的种类判断，不比文字（实现里由 `status` 的计算一并给出"是不是已保存"）。
- `styles.css` 的 `@layer components` 加：

  ```css
  /* The edit's "Saved." in the page's head, its check drawn before it, never in its text (00-M13 4.5.1; M13/P4/S1). */
  .nw-edit-status[data-saved]::before {
    content: ""; display: inline-block; width: 13px; height: 13px; margin-inline-end: 5px; vertical-align: -2px;
    color: var(--accent); background-color: currentColor;
    mask: url("…同 .nw-saved 的 m5 12 5 5 9-10…") center / contain no-repeat;
  }
  @media (forced-colors: active) {
    .nw-edit-status[data-saved]::before { forced-color-adjust: none; color: CanvasText; }
  }
  ```

- `PageEdit`：视图的 `div` 换成 `space-y-3.5`；没失锁时最上面是编辑说明（决定 867），然后 `PageEditingBar`（只剩门户，不占位）；`stayed()` 加 `|| slots.before?.contains(at) === true || slots.after?.contains(at) === true`：它决定离开的保存安不安静（`page-edit.tsx:279`）与离开失败时调不调 `back()`（`:289`），不决定成功离开之后焦点去哪（决定 866）。失锁的横幅、上传、冲突、编辑器照旧在视图里。
- 375 宽时编辑的头一行：面包屑一行，状态、"保存"、"完成"一行（开关在 375 宽时不显示），靠右，与阅读时同样两行；状态长（字段的错误）时它在盒子里折行，两行时头一行不变，三行时见上。

#### 4.1.10 阅读时的锁说明（提交 2）

`EditLockNote` 的外层换成决定 868 的类，`<output>` 之前一个 `<Lock aria-hidden className="size-3.5 shrink-0" />`，文字 `isolate((name) => t("page.lockedBy", { name: name(holder.display_name) }))` 经 `Isolated` 写出；"在这里编辑"与"解除锁定"是 `variant="outline" size="sm" shape="pill"`；外层的 `div ref={lockNote} tabIndex={-1}`（焦点目标）不变。版本页的锁说明是同一个组件，随之换（它在版本页旧的头之下，P5 排版本页时再看）。e2e 以 `getByText("… is editing this page.", { exact: true })` 找它的几处（c1、c3、c6、c7、c10、hi2:137）不改：Playwright 的文字含子元素的文字。

#### 4.1.11 焦点、地标与名字的走查

| 时机 | 焦点 |
|---|---|
| 点开关（鼠标或键盘） | 留在开关上（WebKit 在 macOS 上点按钮不给它焦点：那时焦点照旧在之前的地方） |
| 右栏藏起（收起，或窗口变宽跨过 1280 px、偏好是收起）时焦点在右栏里 | 开关（`preventScroll`，决定 859）；窗口变宽那一支在 Chromium 里焦点落在 `body`（它重算样式时就把焦点修走，早于 layout effect；审查 A 的 R1-N4，照实记下） |
| 焦点在开关上时窗口变窄到 1280 px 以下 | `body`（开关 `display: none`，三种引擎都把焦点修走；只在改窗口宽度时有，照实记下，审查 A 的 R3-N1） |
| 点大纲的项 | 同今天：视图把那个标题滚到 16 px、给它焦点（`view-anchors.ts`）；读到的一节在点按里就标它（决定 893） |
| 读到的一节变了 | 不动；右栏只改自己的 `scrollTop`，读者在右栏里动过之后也不改（决定 886） |
| "编辑"拿着焦点时页变得不能编辑 | `h1`，窗口不动（`EditButton` 的 `useFocusLeaving`，`preventScroll`，决定 865） |
| 进入编辑、离开编辑 | 同今天（`enter()` 先给 `h1`；回到阅读给"编辑"或 `h1`） |
| 离开时焦点在头一行的"保存"或"完成"上（点了"完成"，或等上传时挪过去） | `stayed()` 认两个槽：离开的保存撞上冲突时冲突的标题，离开失败时编辑器（`back()`）；成功离开照旧给"编辑"（审查 A 的 M5） |
| 有未保存的修改时焦点进了头一行右边一组（"保存"、开关或"完成"），之后自动保存完成 | 不动，"保存"不隐去（决定 885、895），Shift+Tab 照旧走得到它；焦点离开那一组、又没有未保存的修改时它才 `invisible`（占位，不可聚焦，决定 894） |
| 没有修改时焦点进头一行右边一组（点开关、点"完成"、键盘走进来） | 不动；"保存"照旧 `invisible`，不在走过的位置上冒出来（决定 895 改，审查 A 的 R2-M1） |
| 解除锁定之后 | 同今天：`released` 给"编辑"或 `h1` |

地标与名字：头一行不是地标（它在 `main` 里，与今天的头一样）；右栏照旧是名为"About this page"的 `complementary`，开关是同名的按钮、带 `aria-expanded` 与 `aria-controls`；大纲照旧是名为"Outline"的 `nav`，当前项 `aria-current="location"`；面包屑照旧是名为"Breadcrumb"的 `nav`；头的"编辑过"是普通文字；属性条照旧是 `table`（经 `propertyRoles`）。

#### 4.1.12 375 px

头一行折成两行（面包屑；右边一组靠右，"编辑过"缩到放得下、省略）；内容列就是 `main` 的内容宽；属性条折行，长的值在小块里折，不成区域；右栏在正文之后，没有开关。PG17 第 1 格的 375 px 一段以 `expectFits` 核对（5.1）。

**照实**（S1 的实施与审查，[S1 的审查记录](reviews/P4-S1-head-strip-panel-review.md)）：

- **4.1.1**：表外另改了 `W/pages/load-more.tsx`（`LoadMoreButton` 加 `small`：`size="sm" shape="pill"`，"更多版本""更多反向链接"用它，默认不变）、`W/pages/page/attachment-drop.tsx`（加可选的 `className`，页给它收回视图的 4 px）、`W/i18n/isolated.tsx`（`Isolated` 加 `nameClass`，给名字的 `bdi`，决定 899；别的调用不带它，输出不变）、`W/pages/page/version-page.tsx`（标题下"时间 · 作者"一行，4.1.10 的照实）。`W/app/settings-page.tsx` 另导出 `settingsColumnClass`（决定 898），`md:text-right` 旁那半句过时的注释删掉（审查 A 的 P4S1A-N2）。`tools/e2e-projects/check.mjs` 多第 4 格的两行（5.1）；测试的帮手 `W/test/lock-note.ts`（7.1）。
- **4.1.3**：`HeadMeta` 是 `<p title className="flex justify-end-safe overflow-hidden whitespace-pre">`，名字的 `bdi` 是 `min-w-0 truncate`（决定 899）：名字长时只截名字、时间完整；`justify-end-safe` 是 `justify-content: safe flex-end`，放不下时裁结尾、不裁开头（审查 A 的 P4S1A-F1-N2；Safari 17.6 之前不认 `safe`，整条声明被丢掉，退成靠左）。`EditButton` 不照设计在同一个组件里 `useFocusLeaving(edit, left)`：它卸下时在微任务里交出焦点，条件是 `edit.current === null && focusLost()`。照设计写，`page-edit`（3 格）、`page-autosave`（2 格）、`page-lock`（1 格）红在"离开编辑之后焦点在'编辑'上"：这三个文件开着 `reactStrictMode`，StrictMode 在"回到阅读，焦点给'编辑'"那个被动效果之后重演新挂的组件的 layout 清理，那时"编辑"拿着焦点，设计的写法立刻把焦点给了标题；生产里没有这次重演，两种写法的结局相同，差别只是焦点先落 `body` 一个微任务（审查 A 核实，偏离 2）。前半个条件多余（变异 t1 等价，7.2）。
- **4.1.4**：`NotebookLayout` 的 `Outlet` 一直包在 `ColumnTopContext` 里，不带列时值是 `null`（与"没有提供者"答的一样）：照设计只在带列时包，列与非列的路由之间换时 `Outlet` 那一格的元素类型变了，React 卸下重挂（审查 A 的偏离 1）。决定 892 那一支照决定 898 包进 `settingsColumnClass` 的列。
- **4.1.6**：`Mark` 的 `current` 是 `rounded-full border border-transparent bg-accent-soft px-1.5 text-[11px] text-accent`：透明的边框在强制色下画成框（审查 B 的 P4S1B-N2），`px-1.75` 降成 `px-1.5` 保住宽度，高 18.5 px（同样稿的 `.tag`）。"更多版本""更多反向链接"经 `LoadMoreButton` 的 `small`（审查 A 的偏离 5）。
- **4.1.7**：右栏跟着时 `scrollTop` 的增减取 `Math.ceil`：项高 25.875 px，边落在小数上，DPR 1 时 Chromium 把 `scrollTop` 取整，差 0.375 px 就切掉项的边，PG17 第 8 步红过；多滚不到 1 px，下一帧当前项不变、不再动（变异 t2，审查 A 的偏离 7）。没有列出的标题时不监听、不量、答 `undefined`（审查 A 的 P4S1A-N1，上面的代码）。`ResizeObserver` 看视图：S1 起初照设计看 `main`，审查 B 在修复核对第 1 轮跑临时故事时 WebKit 报了一次"ResizeObserver loop completed with undelivered notifications"。实施者给页上每个 `ResizeObserver` 包一层，按帧记下观察、回调、目标的深度与还在下载的样式表：每一次报错时 `katex-*.css` 都还在途；那一轮先交出滚动区的观察者看的视图（深度 11）与区块（12），再交出 `main`（5），下一帧 `main` 又交一次。滚动区的回调同步读布局（`scrollWidth`、`clientWidth`），WebKit 这时才把 React 刚写进的东西排进布局，`main` 在它的通知之后又变了，而它不比这一轮最浅的深，规范判为没送到。一页带公式与图、在 WebKit 里打开 40 次跑 3 遍（有 Playwright 的路由，HTTP 缓存关着；开着时 0 次）：改前 120 次 23 次报错（`addcfa69` 上 7、4、12 次，`c6297488` 上 10、6、7 次），只在记录层里拦掉 `main` 的观察 0 次，改看视图之后 0 次（`f698d37e`；视图与滚动区自己的目标一样深，S1 不再多出一个会被跳过的深度）。代价：视图之外变高的东西要等下一次滚动或改窗口才重量（审查 A 在三种引擎里量了两种过时，10.1）；审查 A 提的在内容列上挂 `MutationObserver` 的补法不做（主持者定：代价只在页顶、页底一阵，下次滚动就对）。读者的记录随大纲的列表（决定 897）。
- **4.1.8**：`.nw-reading` 加 `isolation: isolate`（审查 A 的 P4S1A-M1）：l6 的大页上右栏开着滚动约 20 fps，右栏的每次改动（`aria-current`、类、`font-medium`、`scrollTop`）都让根绘制层重画、走遍正文的每一块；正文成了自己的绘制层，60 fps（10.1）。不用 `position: relative`：它改正文里 `.nw-math-measure`（`position: absolute`）的包含块。小块的圆角是 12 px，不是 9999 px：长的值折成多行时胶囊的弧把键与值压到边框上（审查 B 的 P4S1B-M1；单行时看不出差别）。嵌套表与它的 `tbody` 是 `display: inline-flex`，不是 `inline-block`：Firefox、WebKit 里那一块原比同一行的高约 1.5 px（审查 B 的 P4S1B-N1）。
- **4.1.2、4.1.10**：没人拿锁时锁说明的外层（`tabIndex={-1}`）是 `empty:mb-0`，不是审查 B 提的 `empty:hidden`：`enter()` 与 `toggleRefused` 读到锁之后立刻聚焦它，那时说明可能还没画出，`display: none` 会让这一下落空。视图的 4 px 内边距在标题下由视图自己收回：`AttachmentDrop` 带 `[:first-child:empty+&>.nw-reading:first-child]:-mt-1`，只在它紧跟着空的、第一个锁说明外层，视图又是它的第一个子元素时生效；标题到属性条（或第一段）14 px，同样稿（审查 B 的 P4S1B-M2）；读取中、读取失败与文件夹的提示也是 14 px（第一回写在 `AttachmentDrop` 自己身上，`[:first-child:empty+&]:-mt-1`，那三种离标题 10 px，审查 B 的 P4S1B-F2）；有锁说明时标题到说明 14 px、说明到属性条 28 px，照旧。锁说明里锁的图标与句子包进 `div.flex.min-w-0.items-center.gap-x-3`，一起换行（审查 B 的 P4S1B-M3：句子比图标右边的空间宽时原是图标单独占一行）；句子的 `output` 是 `min-w-0 wrap-anywhere`（`overflow-wrap: anywhere`）：Firefox 照 UAX #14 的 HY×NU 不在连字符接数字处断行，那样的名字冲出说明、页横向滚动（审查 B 的 P4S1B-F1）。版本页标题下"时间 · 作者"一行是 M8 起的，同样会冲出（实施者找到），同加 `min-w-0 wrap-anywhere`；"·"后与 `versionBy` 的种类前是不断行空格，名字放不下时整个从"·"起换行，"·"不单独留在上一行（审查 B 的 P4S1B-F2-1；右栏的 `VersionBy` 照旧）。
- **5.1、5.4、5.5、7.1、7.2、7.4**：见各节的照实；决定 897–900。

### 4.2 S2 正文的样式

#### 4.2.1 文件

| 文件 | 改什么 | 提交 |
|---|---|---|
| `W/reading/reading.css` | `:root`/`.dark` 的变量（4.2.2）；正文、标题、链接、标签、高亮、列表与任务项、引用、行内代码、`pre`、表格、callout、图的容器、hljs（4.2.3–4.2.6）；提交 2 加代码块的头（4.2.7） | 1、2 |
| `W/reading/diagrams.ts` | `base`、两份 `themeVariables` 与两段 `themeCSS`（决定 871、888，4.2.6） | 1 |
| `W/design-tokens.test.ts` | 亮暗成对的检查另核对 `reading.css` 的 `:root`/`.dark`（决定 870） | 1 |
| `W/reading/code-blocks.ts`（新）、`W/reading/enhancement.ts` | `codeBlocks` 与它在注册表里的位置、注释里的一句（决定 869） | 2 |
| `W/reading/patch.ts` | `keyed` 加 `button.nw-code-copy`，`keyOf` 给它 `copy`（决定 869） | 2 |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | `reading.copy`、`reading.copied`、`reading.copyFailed` | 2 |

#### 4.2.2 变量

`reading.css` 开头的 `:root`/`.dark` 收进阅读视图与编辑器都要的颜色（`.nw-reading` 里的同名变量删掉），两块挪出 `@layer components`，放在文件的最外层（同 `styles.css`；今天它们在层里，`:11-18`）。都是自定义属性、名字只在这里声明，出不出层不改任何值；挪出来是为了 `design-tokens.test.ts` 读得到：它的 `blocks()` 只读样式表最外层的规则（审查 B 的 R1-I1、C 的 R1-M5）：

```css
:root {
  --nw-mark-background: rgba(245, 196, 81, 0.38);
  /* reading.html .light: --code, --link-line (00-M13 4.1); the current match's, P4/S3's, a deeper --mark. */
  --nw-code-background: #f6f6f7;
  --nw-link-line: rgba(197, 138, 34, 0.6);
  --nw-match-selected: rgba(245, 196, 81, 0.72);                  /* S3 */
  /* hljs: reading.html's four (--kw, --str, --cm, --fn), the other five drawn from the same set (M13/P4/S2). */
  --nw-hl-keyword: #a0661a;  --nw-hl-string: #3e7d5a;  --nw-hl-comment: #9a9ca3;  --nw-hl-title: #3d5fb5;
  --nw-hl-constant: #7a52b3; --nw-hl-built-in: #2f8f6a; --nw-hl-name: #b15c2e;
  --nw-hl-addition: #3e7d5a; --nw-hl-addition-background: rgba(62, 125, 90, 0.1);
  --nw-hl-deletion: #d64545; --nw-hl-deletion-background: rgba(214, 69, 69, 0.08);
  /* The callouts' families: reading.html's --tip, the others its like (M13/P4/S2). */
  --nw-callout-blue: #3d6fb8; --nw-callout-teal: #2f8f6a; --nw-callout-green: #3e8a4f; --nw-callout-orange: #b26a12;
  --nw-callout-red: #d64545;  --nw-callout-purple: #7a52b3; --nw-callout-grey: #6b6e76;
}
.dark {
  --nw-mark-background: rgba(217, 168, 78, 0.28);
  --nw-code-background: #1f2023;
  --nw-link-line: rgba(217, 168, 78, 0.55);
  --nw-match-selected: rgba(217, 168, 78, 0.5);                   /* S3 */
  --nw-hl-keyword: #e0b061;  --nw-hl-string: #8fcf9f;  --nw-hl-comment: #6f7178;  --nw-hl-title: #92aaf0;
  --nw-hl-constant: #c3a6f2; --nw-hl-built-in: #5cc49a; --nw-hl-name: #eb9a6c;
  --nw-hl-addition: #8fcf9f; --nw-hl-addition-background: rgba(143, 207, 159, 0.12);
  --nw-hl-deletion: #ff7b7b; --nw-hl-deletion-background: rgba(255, 123, 123, 0.12);
  --nw-callout-blue: #8fb0ec; --nw-callout-teal: #5cc49a; --nw-callout-green: #7fcf8f; --nw-callout-orange: #e0a656;
  --nw-callout-red: #ff7b7b;  --nw-callout-purple: #c3a6f2; --nw-callout-grey: #9a9ca3;
}
```

（`--nw-match-selected` 是 S3 的，S2 不写。）标签的两个变量（`--nw-tag`、`--nw-tag-background`）删掉：标签改用 `--surface-2` 与 `--muted-foreground`。`design-tokens.test.ts` 今天只在 `styles.css` 里核对亮暗齐全（`:215`）；S2 提交 1 加一格，`reading.css` 的 `:root` 与 `.dark` 声明的变量名两边相同、各不少于 20 个（它已读 `reading.css`，`declared(reading, ":root")` 与 `".dark"`；决定 870）。今天的 `reading.css` 里这两块在 `@layer components` 里，`declared` 读出 0 个，这一格红；照上面挪到最外层、写全之后读出各 21 个（S3 加 `--nw-match-selected` 之后各 22 个），绿。

#### 4.2.3 正文、标题、链接

```css
.nw-reading { font-size: 1rem; line-height: 1.8; /* overflow、margin、padding 照旧 */ }
.nw-reading > * + * { margin-top: 1.125rem; }
.nw-reading :is(h1, h2, h3, h4, h5, h6) { margin-top: 1.625rem; font-weight: 620; line-height: 1.35; scroll-margin-top: 1rem; }
.nw-reading :is(h1, h2) { margin-top: 2.375rem; font-weight: 630; letter-spacing: -0.005em; }
.nw-reading h1 { font-size: 1.5rem; }
.nw-reading h2 { font-size: 1.3125rem; }        /* 21px，不再有底线 */
.nw-reading h3 { font-size: 1.0625rem; }        /* 17px */
.nw-reading :is(h4, h5, h6) { font-size: 1rem; }
/* After a heading, a block not a heading comes close; a heading after a heading keeps its own space (review B's M2). */
.nw-reading > :is(h1, h2) + :not(h1, h2, h3, h4, h5, h6) { margin-top: 0.875rem; }
.nw-reading > :is(h3, h4, h5, h6) + :not(h1, h2, h3, h4, h5, h6) { margin-top: 0.625rem; }
.nw-reading a { color: var(--foreground); text-decoration: underline; text-decoration-thickness: 1px;
  text-decoration-color: var(--nw-link-line); text-underline-offset: 3.5px; }
.nw-reading a:hover { text-decoration-color: currentColor; }
.nw-reading a.nw-unresolved { color: var(--muted-foreground); text-decoration-style: dashed; text-decoration-color: var(--faint); }
.nw-reading .nw-tag { padding: 1px 6px; border-radius: 6px; background: var(--surface-2); color: var(--muted-foreground);
  font-size: 0.84375em; text-decoration: none; }
.nw-reading a.nw-tag:hover { color: var(--foreground); }
.nw-reading a.nw-tag:focus-visible { border-radius: 6px; }
.nw-reading mark { padding: 0 2px; border-radius: 3px; background: var(--nw-mark-background); color: inherit; }
.nw-reading blockquote { padding-left: 1rem; border-left: 3px solid var(--border); color: var(--muted-foreground); }
```

链接的焦点照旧（2 px 的琥珀外框、偏移 2 px，`:is(a, [role="button"]):focus-visible`）；没建的页的 `role="button"` 与 `aria-busy` 的规则照旧；嵌入（`a.nw-embed`）照旧加 6 px 圆角的边。`h1` 在正文里是 24 px：页的标题（32 px）在内容列里，正文的 `#` 标题小一档，同样稿的层次。

#### 4.2.4 列表、任务项、行内代码、`pre`、表格

```css
.nw-reading li + li, .nw-reading li > :is(ul, ol) { margin-top: 0.25rem; }
/* A list item's blocks 0.5rem apart, as a callout's (decision 903): a loose list's paragraphs, and a code block, a quote,
   a callout or a table's wrapper under the item's line. Not a table: the properties' list of maps is li > table.nw-props. */
.nw-reading li > p:not(:first-child),
.nw-reading li > :is(pre, blockquote, div, details, hr, h1, h2, h3, h4, h5, h6) { margin-top: 0.5rem; }
.nw-reading li:has(> input[type="checkbox"], > p:first-child > input[type="checkbox"]) { list-style: none; }
.nw-reading :is(li, li > p:first-child) > input[type="checkbox"]:first-child {
  width: 1rem; height: 1rem; margin: 0 0.55rem 0 -1.55rem; vertical-align: -0.15em; }
/* A done item's own text struck and faint, not its nested items' (00-M13 4.5.3): a line-through goes into no
   inline-block, and a colour set back is a colour, where an opacity is not undone. */
.nw-reading li:has(> input[type="checkbox"]:checked, > p:first-child > input[type="checkbox"]:checked) {
  color: var(--faint); text-decoration: line-through; text-decoration-color: var(--faint); }
/* Its nested items and its blocks under its text each an inline block the item's width, their colours their own (S2's
   fix check); atop their line, as a block stands: an inline block that scrolls has its baseline at its bottom. */
.nw-reading li:has(> input[type="checkbox"]:checked, > p:first-child > input[type="checkbox"]:checked)
  > :is(ul, ol, pre, blockquote, div, details) { display: inline-block; width: 100%; vertical-align: top; }
.nw-reading li:has(> input[type="checkbox"]:checked, > p:first-child > input[type="checkbox"]:checked)
  > :is(ul, ol, pre, div, details) { color: var(--foreground); }
/* Its own links and tags faint with it (review B's M4), not those of the items nested in it (review B's R1-N2). */
.nw-reading li:has(> input[type="checkbox"]:checked, > p:first-child > input[type="checkbox"]:checked) > :is(a, .nw-tag),
.nw-reading li:has(> input[type="checkbox"]:checked, > p:first-child > input[type="checkbox"]:checked) > :not(ul, ol, pre, blockquote, div, details) :is(a, .nw-tag) {
  color: inherit; }
.nw-reading :not(pre) > code { padding: 1px 5px; border: 1px solid var(--border); border-radius: 5px;
  background: var(--surface-2); font-family: var(--font-mono); font-size: 0.84em; }
.nw-reading pre { overflow-x: auto; border: 1px solid var(--border); border-radius: 12px; background: var(--nw-code-background);
  font-family: var(--font-mono); font-size: 0.8125rem; line-height: 1.7; }
.nw-reading pre > code { display: block; width: max-content; min-width: 100%; padding: 14px 16px; }
.nw-reading pre:not(:has(> code)) { padding: 14px 16px; }
.nw-reading table:not(.nw-props) { width: 100%; border-collapse: collapse; font-size: 0.90625rem; }
.nw-reading table:not(.nw-props) :is(th, td) { padding: 10px 12px; border-bottom: 1px solid var(--border); line-height: 1.5; }
.nw-reading table:not(.nw-props) th { padding-block: 8px; border-bottom-color: var(--border-strong);
  color: var(--muted-foreground); font-size: 0.8125rem; font-weight: 500; }
.nw-reading table:not(.nw-props) :is(th, td):first-child { padding-left: 2px; }
```

（`th:not([align]) { text-align: start }` 照旧，pg5 读 `th[align="left"]`。）旧的竖线、表头的底色与 h2 的底线去掉。修订时逐条核对了 4.1.8、4.2.3–4.2.7 各规则的特指度（审查 B 的 M3）：callout 那一处改了（4.2.5）；已完成的项的 `color: inherit` 只给它自己的链接与标签（直接的子元素，或它的 `p` 等非列表子元素里的），不落进嵌套的子项（审查 B 的 R1-N2：修订稿的一条后代选择器落进嵌套的、没完成的子项，那里的标签与没建的链接成了正文色；B 照这两条在三种引擎里量过，自己的、`strong` 里的、松散项第二段里的都淡，嵌套的已完成项淡，没完成的回到次要色）；它高过 `a.nw-tag:hover`，已完成项里的标签悬停不变色，无关紧要；别的规则同一元素同一属性上没有被盖过的。任务项的复选框照 4.3：原生、`accent-color`（决定 872）；已完成的项里嵌套的子项照旧可以单独勾。

#### 4.2.5 callout 与图的容器

```css
.nw-reading .nw-callout {
  --nw-callout: var(--nw-callout-blue);
  --nw-callout-icon: url(…照旧…);
  padding: 12px 16px; border: 1px solid color-mix(in srgb, var(--nw-callout) 22%, transparent); border-radius: 10px;
  background: color-mix(in srgb, var(--nw-callout) 7%, transparent);
  /* On the callout, not its children: a rule on its children outranks .nw-reading pre (review B's M3). */
  font-size: 0.9375rem; line-height: 1.7;
}
/* Its blocks 0.5rem apart, its title 4px above the first (decision 902). */
.nw-reading .nw-callout > * + * { margin-top: 0.5rem; }
.nw-reading .nw-callout > :is(.nw-callout-title, summary) + * { margin-top: 0.25rem; }
/* A line of text, its bold, links and code in it as in any line, its 16px icon first in the line (decision 901). */
.nw-reading .nw-callout > :is(.nw-callout-title, summary) { display: block;
  color: var(--nw-callout); font-size: 0.875rem; font-weight: 600; line-height: 1.5; }
.nw-reading .nw-callout > :is(.nw-callout-title, summary)::before { content: ""; display: inline-block; width: 16px;
  height: 16px; margin-inline-end: 7px; vertical-align: -0.15em; /* 照旧的遮罩 */ }
.nw-reading details.nw-callout > summary { position: relative; padding-inline-end: 22px; list-style: none; cursor: pointer; }
.nw-reading details.nw-callout > summary::-webkit-details-marker { display: none; }
/* A folded callout says it folds: a chevron at its title's first line's end, turned while closed, as the right
   column's sections. */
.nw-reading details.nw-callout > summary::after {
  content: ""; position: absolute; top: calc((1.5em - 14px) / 2); inset-inline-end: 0; width: 14px; height: 14px;
  background-color: currentColor; mask: url("…lucide 的 chevron-down…") center / contain no-repeat;
  transition: transform 150ms;
}
.nw-reading details.nw-callout:not([open]) > summary::after { transform: rotate(-90deg); }
@media (forced-colors: active) {
  .nw-reading .nw-callout > :is(.nw-callout-title, summary)::before,
  .nw-reading details.nw-callout > summary::after { forced-color-adjust: none; background-color: CanvasText; }
}
.nw-reading .nw-callout:is([data-callout="abstract"], …) { --nw-callout: var(--nw-callout-teal); …icon 照旧… }
/* 其余各类同此：info、todo 蓝；tip、hint、important 青；success、check、done 绿；question、help、faq、warning、caution、
   attention 橙；failure、fail、missing、danger、error、bug 红；example 紫；quote、cite 灰。 */
/* Drawn in the app's colours under forced colours, as an image is: mermaid's fills are not forced (review B's R2-N3). */
.nw-reading .nw-diagram { padding: 26px 20px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface);
  forced-color-adjust: none; }
.dark .nw-reading .nw-diagram { background: var(--nw-code-background); }
.nw-reading .nw-diagram > svg { display: block; margin-inline: auto; }
/* A timeline's arrowheads the colour of its line, not the text's (review B's P4S2B-N1). */
.nw-reading .nw-diagram svg[aria-roledescription="timeline"] marker > path { fill: var(--faint); }
/* Its labels too: Chromium's own sheet forces a foreignObject's HTML under forced colours (cdedff02). */
.nw-reading .nw-diagram foreignObject { forced-color-adjust: none; }
```

暗色下标题不再 `color-mix(… 65%, white)`：暗色的族色本就是亮的一档。折叠的 callout 的 `summary` 照旧 `cursor: pointer`，`list-style: none` 与 WebKit 的 `::-webkit-details-marker { display: none }` 去掉浏览器自带的三角（三种引擎都没有，审查 B 的 M8）：`::before` 的图标在行首，`::after` 的 chevron 绝对定位在第一行的右端、关着时转 −90°，看得出里面还有内容；标题里的音视频控件照旧（`titleMedia`；在标题的文字里）。设计原写标题 `display: flex`，S2 照决定 901 改回块：弹性项把标题里的标记拆开（审查 A 的 P4S2A-I1、B 的 P4S2B-I1），`::before` 原为长标题加的 `flex: none` 随之去掉。强制色下标题的图标与 chevron 的遮罩以 `CanvasText` 画（审查 B 的 N2：不然图标消失、留下空位）。WebKit 加 VoiceOver 读不读 `summary` 的展开状态，进读屏清单（第 9 节"收尾"；决定 901 之后 `summary` 是块，不再是 `display: flex`）。图在强制色下不被强制（`forced-color-adjust: none`）：不然它的底被换成 `Canvas`，mermaid 的 SVG 的 `fill` 却不被强制（Chromium 是 `preserve-parent-color`，Firefox 不改），系统高对比的明暗与应用的主题相反时，直接画在底上的字（标题、轴、节名、条外的标签、边上的字）几乎看不见，Firefox 里看板的列名另被套上白底；不强制之后图连同它的底照应用的主题画，同图片（审查 B 的 R2-N3，B 在两种引擎里画过；今天的 `default`/`dark` 也这样，不是 P4 带来的）。

#### 4.2.6 hljs 与 mermaid

hljs 的规则（`reading.css:434-492`）一条不改，只换变量的值（4.2.2）：关键字色给 `keyword`、`doctype`、`template-tag`、`template-variable`、`type`；标题色给 `title`（含 `class_`、`function_`，样稿的函数名）；常量色给 `attr`、`attribute`、`literal`、`meta`、`number`、`operator`、`variable`、`selector-attr`、`selector-class`、`selector-id`、`section`；字符串色给 `regexp`、`string`；内置色给 `built_in`、`symbol`、`bullet`；注释色给 `comment`、`code`、`formula`；名称色给 `name`、`quote`、`selector-tag`、`selector-pseudo`；增删两组给 `addition`、`deletion`。pg5:158-163（关键字与代码的颜色不同）照旧成立。hljs 的 Go 语法不给方法调用标 `title`、把 `nil` 归常量色，样稿的 Go 片段里方法名是函数色、`nil` 是关键字色：这是 hljs 的分词，不改，照实记成与样稿的偏离（审查 B 的 N5）。

`diagrams.ts`（决定 871、888；值与写法照第 1 节 (d) 的探针，`p4-design/mermaid/vars.js` 是它的草稿）：

```ts
/** The series' colours, the callouts' seven families (00-M13 4.1): blue, orange, teal, purple, red, green, grey. */
const series = {
  light: ["#3d6fb8", "#b26a12", "#2f8f6a", "#7a52b3", "#d64545", "#3e8a4f", "#6b6e76"],
  dark: ["#8fb0ec", "#e0a656", "#5cc49a", "#c3a6f2", "#ff7b7b", "#7fcf8f", "#9a9ca3"],
} as const;
/** The diagrams' colours, mermaid's base theme in the app's (00-M13 4.1; v0.1 design 13.2 item 32): hexadecimal alone. */
function themeVariables(theme: "light" | "dark"): Record<string, unknown> { … }
/** What the variables cannot say: a node's corners, a store's amber, two lines mermaid draws in the page's colour. */
function themeCSS(theme: "light" | "dark"): string { … }
// options(theme): …, theme: "base", themeVariables: themeVariables(theme), themeCSS: themeCSS(theme),
```

`themeVariables(theme)` 依次是：

| 什么 | 亮 | 暗 | 为什么 |
|---|---|---|---|
| 决定 871 的十三个 | 同 871 | 同 871 | 灰阶的节点与线、琥珀的注 |
| `darkMode` | 不给 | `true` | 派生色照暗色算（审查 B 的 I1） |
| `pie1`–`pie12`、`git0`–`git7`、`gitInv0`–`gitInv7`、`cScale0`–`cScale11`、`cScalePeer0`–`cScalePeer11` | 七族轮着给 | 同，暗的一档 | 饼、git 图、思维导图、时间线、看板、雷达的系列（`base` 由灰推不出颜色）；`gitInv*` 是高亮的提交。饼多于七块时颜色重复（第 8 块又是蓝），记成偏离（审查 B 的 R1-N3）；旅程的节读 `fillType0–7`（不给，由灰推出，照旧是灰的、看得清），角色的点是 mermaid 配置里写死的 `journey.actorColours`，都不是这里的系列 |
| `cScaleLabel0`–`11`、`gitBranchLabel0`–`7` | `#ffffff` | `#18191c` | 系列色上的字：亮色族色上白字，暗色淡的一档上近黑的字 |
| `cScaleInv0`–`11` | `#91939b` | `#74767e` | 时间线的竖线（不给时是族色的反色，亮色看不见） |
| `pieStrokeColor`、`pieOuterStrokeColor`、`pieSectionTextColor`、`pieOpacity` | `#ffffff`、`#d4d4d9`、`#ffffff`、`"1"` | `#1f2023`、`#3a3b41`、`#18191c`、`"1"` | 饼块之间的缝是底色，块上的字照族色定 |
| 甘特：`altSectionBkgColor`、`gridColor`、`doneTaskBkgColor`、`critBkgColor`、`todayLineColor`、`vertLineColor` | `#ffffff`、`#e7e7ea`、`#d4d4d9`、`#d64545`、`#c2871f`、`#c2871f` | `#1f2023`、`#2b2c31`、`#3a3b41`、`#b33b3b`、`#d9a84e`、`#d9a84e` | 隔行、网格、完成、关键（亮色 `--destructive`；暗色压暗一档：任务的字随 `textColor` 是浅的，`#ff7b7b` 上只有 2.05:1，`#b33b3b` 上 4.76:1，审查 B 的 R1-M2）、今天（琥珀） |
| 甘特：`activeTaskBkgColor`、`activeTaskBorderColor` | `#e7e7ea`、`#91939b` | 不给 | 亮色"进行中"的任务（`base` 由灰推出白底、`#f4f4f5` 的边，白底的图上看不见；字 14.2:1，审查 B 的 R1-M2）；暗色派生的看得见 |
| `excludeBkgColor` | 不给 | `#25262a` | 甘特排除的日子 |
| `edgeLabelBackground`、`rowOdd`、`rowEven` | 不给 | `#1f2023`、`#1f2023`、`#25262a` | 边上的字的底（不给时近黑）；ER 带属性的行（`darkMode` 下派生成近黑） |
| `archEdgeColor`、`archEdgeArrowColor`、`archGroupBorderColor` | `#91939b` | `#74767e` | 架构图的线与组框（默认黑） |
| `xyChart` | 整个对象：底 `#ffffff`，字 `#18191c`，轴线与刻度 `#91939b`，`plotColorPalette` 七族 | 底 `#1f2023`，字 `#e8e8ea`，轴 `#74767e`，七族 | 给了就整个代替，只给调色板时其余的丢了（暗色画成白底） |
| `radar` | 整个对象：轴 `#91939b`，网格 `#e7e7ea`、透明度 0.3，其余照 mermaid 的默认（线宽、字号、`curveOpacity` 0.5） | 轴 `#74767e`，网格 `#3a3b41`、0.5 | 同上；默认的网格 `#DEDEDE` 在暗色上是一片灰 |

`themeCSS(theme)`（mermaid 把它放进图自己的样式，选择器都以图的 id 限定，特指度高过 mermaid 自己的 `#… .node path`；作者用 `style`、`classDef` 给的内联样式仍盖过它）：

```css
.node > rect.basic.label-container { rx: 9px; ry: 9px; }                       /* 样稿 .node 的 9 px 圆角 */
.node > path.basic.label-container.outer-path { fill: #f8f1e4; stroke: #c2871f; } /* 圆柱（库）：淡琥珀底、琥珀边；暗 #39332a、#d9a84e */
.grid .tick line { stroke: #e7e7ea; }                                           /* 甘特的网格线（带 stroke="currentColor"）；暗 #2b2c31 */
.lineWrapper line { stroke: #91939b; }                                          /* 时间线的主线（默认是 cScaleLabel）；暗 #74767e */
.eventWrapper { filter: none; }                                                 /* 时间线的事件框：去掉 mermaid 的 brightness(120%) */
/* 只在暗色：ER 的圈写死了 fill="white"；看板的列名取 textColor（浅），底是压暗的系列色 */
marker circle[fill="white"] { fill: #1f2023; }
.sections > .cluster > .cluster-label { color: #18191c; fill: #18191c; }
```

（时间线的事件框不去掉提亮时，亮色白字对提亮的青、橙、蓝只有 2.85–3.70:1，去掉之后 3.99–5.70，审查 B 的 R1-N4；暗色看板的列名原先 2.26–2.74:1，近黑之后 6.35:1，`g.sections` 只有看板画，流程图的子图在 `g.clusters` 里，审查 B 的 R1-M3。这两处与甘特的两处 B 照这些值画过，`diagrams-fix.html`。暗色下作者以 `style`/`classDef` 给的浅底配浅字读不清，今天也这样，不是回归，记下。）

库节点的淡底就是 871 的 `noteBkgColor`（琥珀 12%、14% 叠在图的底上的颜色），边是 `--accent`。选择器的核对（第 1 节 (d)）：`.node > path.basic.label-container.outer-path` 只命中流程图的四种圆柱与块图的圆柱，不是偏离；`.node > rect.basic.label-container` 的圆角另落在状态图的状态、没有属性的 ER 实体、看板的卡片与块图的块上（同一种节点，照样稿一并圆角，记下）。`themeVariables` 与 `themeCSS` 每次渲染照旧重建（`options(theme)`），缓存照旧按主题分（`keyOf(theme, source)`），换主题照旧重画（l4:185-194）。375 px 宽时图的字约 7 px（今天就这样，不是回归；7.4 的截图记下）。

照实（S2 的实施与审查，[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)）：`options(theme, own)` 另给 `flowchart: { padding: 9 }`（871）；没有图自己的颜色时给 `base`、两份 `themeVariables` 与 `themeCSS`，有时只给它自己的主题，没写主题的取 `theme === "dark" ? "dark" : "default"`（决定 904、906）。`render()` 先照旧 `initialize(options(theme))`、`tooLarge`，之后 `ownColours(mermaid, source)`：源码里没有 `%%{`、去掉开头的空白也不以 `---` 起头时不解析（审查 A 的 P4S2A-F1-N1：作者的配置只能写在指令或 frontmatter 里；A 量的大时序图多解析一遍约多三成的主线程时间），否则 `mermaid.parse(source, { suppressErrors: true })` 读出 `config.theme` 与 `config.themeVariables`，指定了不是 `base` 的主题、或有非空的变量时再 `initialize(options(theme, { theme }))`；`parse` 答 `false`（语法错、不认识的类型）时照应用的颜色，画不出照旧显示源码。每张图都重新 `initialize`，画经 `drawingsOut` 串行，作者的配置不漏到下一张图（审查 C 核对）。375 px 宽时图的字实量见 10.2（甘特约 2 px、时间线 3.9 px、旅程 4.3 px，比"约 7 px"小，今天就这样）。

#### 4.2.7 代码块的头（提交 2）

`W/reading/code-blocks.ts`：

```ts
/** How long a copy's word shows: Copied, or Couldn't copy. */
const shownFor = 1500;
/**
 * codeBlocks gives each code block (a pre whose code is not mermaid's) its head (00-M13 4.5.3; decisions 869, 890): the
 * language, and Copy where the page may use the clipboard, which copies the code's text alone. In the pre, its first
 * child: the top-level node is the same, the block's region its name still; sticky at its left, unselectable.
 */
export const codeBlocks: Enhancement = (container, { t }) => {
  const heads: HTMLElement[] = [];
  const timers = new Set<ReturnType<typeof setTimeout>>();
  const clipboard = typeof navigator.clipboard?.writeText === "function";
  for (const code of container.querySelectorAll<HTMLElement>("pre > code:not(.language-mermaid)")) {
    const language = [...code.classList].find((name) => name.startsWith("language-"))?.slice("language-".length);
    if (language === undefined && !clipboard) continue;
    const head = document.createElement("div");
    head.className = "nw-code-head";
    if (language !== undefined) head.append(language);
    if (clipboard) {
      const said = document.createElement("span");
      said.className = "nw-code-said";
      said.setAttribute("aria-live", "polite");
      const copy = document.createElement("button");
      copy.type = "button";
      copy.className = "nw-code-copy";
      copy.textContent = t("reading.copy");
      let timer: ReturnType<typeof setTimeout> | undefined;
      // Only the last press tells: an earlier one's answer, come later, neither says nor clears (review B's doubt).
      let presses = 0;
      const clear = () => { said.textContent = ""; delete said.dataset.state; delete copy.dataset.state; };
      copy.addEventListener("click", () => {
        const press = ++presses;
        clear();
        clearTimeout(timer);
        const told = (copied: boolean) => {
          if (press !== presses) return;
          if (copied) copy.dataset.state = "copied";
          said.dataset.state = copied ? "copied" : "failed";
          said.textContent = t(copied ? "reading.copied" : "reading.copyFailed");
          timer = setTimeout(() => { clear(); timers.delete(timer!); }, shownFor);
          timers.add(timer);
        };
        navigator.clipboard.writeText(code.textContent ?? "").then(() => told(true), () => told(false));
      });
      head.append(said, copy);
    }
    code.parentElement?.prepend(head);
    heads.push(head);
  }
  return () => { for (const timer of timers) clearTimeout(timer); for (const head of heads) head.remove(); };
};
```

（实现时照项目的写法拆小、写注释；`t` 是 `ReadingContext` 已有的；清理之后才回来的 `writeText` 写的是已不在文档里的元素，不碍事。）注册表：`codeHighlight, math, diagrams, codeBlocks, propertyRoles, scrollRegions, …`：在图之后（图替换掉 mermaid 的 `pre` 之前它已被排除），在滚动区之前（头在、`pre` 的高度已定）。`enhancement.ts` 的注释在 S1 加的"M13 the properties' roles (propertyRoles)"之前加"a code block's head (codeBlocks) and"。

CSS（`reading.css`，`pre` 的规则之后）：

```css
.nw-reading .nw-code-head {
  position: sticky; left: 0; display: flex; align-items: center; height: 34px; padding: 0 8px 0 14px;
  border-bottom: 1px solid var(--border); color: var(--faint); font-family: var(--font-sans); font-size: 0.75rem;
  line-height: 1; white-space: nowrap; user-select: none; -webkit-user-select: none;
}
/* What a copy said: in the row, pushing Copy to the end however wide it is; a success for screen readers alone, kept
   in the row (an absolute one would leave Copy without its push), a failure shown (decision 890). */
.nw-reading .nw-code-said { margin-inline-start: auto; }
.nw-reading .nw-code-said[data-state="copied"] { width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
.nw-reading .nw-code-said[data-state="failed"] { color: var(--destructive); }
.nw-reading .nw-code-copy {
  display: inline-flex; align-items: center; gap: 6px; height: 24px; margin-inline-start: 6px; padding: 0 8px;
  border-radius: 9999px; color: var(--faint); font: inherit; font-weight: 500; cursor: pointer;
}
.nw-reading .nw-code-copy:hover { background: var(--hover); color: var(--foreground); }
.nw-reading .nw-code-copy::before { content: ""; width: 16px; height: 16px; background-color: currentColor;
  mask: url("…lucide 的 copy…") center / contain no-repeat; }
.nw-reading .nw-code-copy[data-state="copied"]::before { color: var(--accent); mask-image: url("…m5 12 5 5 9-10…"); }
@media (forced-colors: active) {
  /* As specific as the copied one, or it keeps its amber (review B's M5). */
  .nw-reading .nw-code-copy::before, .nw-reading .nw-code-copy[data-state="copied"]::before {
    forced-color-adjust: none; color: ButtonText; }
}
@media print { .nw-reading .nw-code-head { display: none; } }
```

（`said` 空着、或藏起的成功一句只有 1 px 时，它的 `margin-inline-start: auto` 把"复制"推到最右；失败的一句在按钮左边 6 px，按钮不动。初稿另写的 `.nw-code-said:empty + .nw-code-copy` 与 `.nw-code-head > .nw-code-copy:first-child` 两条多余，后一条永远匹配不到，去掉。）按钮的焦点照 `styles.css` 的全局 `:focus-visible`（2 px 的琥珀外框；`pre` 是 `overflow-x: auto`，外框贴着头的下边线，头的 34 px 里有空，画得下）。`pre` 自己是滚动区时（宽），Tab 先到 `pre`（区域），再到"复制"。

`patch.ts`：`keyed` 加 `", button.nw-code-copy"`；`keyOf` 在 `audio`/`video` 之前加 `if (element.matches("button.nw-code-copy")) return "copy";`：同一块里第几个"复制"（`ofKey` 照旧按块数）。就地更新先倒序清理全部增强再重跑，留着的块里的头也是新建的（审查 C 的 M9）：拿着焦点的"复制"不论它的块换没换，都由 `giveBack` 按这个键找回新的那一个。

**照实**（S2 的实施与审查，[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)）：

- **4.2.1**：表外另改了 `W/pages/page/reading-view.test.tsx` 的 `afterEach`（拆掉 user-event 留在 `navigator` 上的剪贴板替身：这个文件没开 vitest 的 globals，`userEvent.setup()` 装的替身留到文件结束，之后用 `readingEnhancements` 的格的代码块会有"复制"；今天没有格受影响，临时的探针格看到这个次序依赖，审查 C 的未核实一条，`0e35d0dc`）。`W/reading/diagrams.ts` 的 `Drawer` 加 `parse`，测试里的四个替身随之答它。L4 的页面格加强制色一步（`cdedff02`，5.2）。
- **4.2.4**：决定 903 的两条与已完成项下面的块（上面的代码；872）。
- **4.2.5**：标题照决定 901 是块（上面的代码），块间照决定 902；图里的 `foreignObject` 另 `forced-color-adjust: none`（`cdedff02`）：照设计只给 `.nw-diagram` 时，Chromium 的 UA 样式在强制色下给 `svg` `preserve-parent-color`、给 `foreignObject` `auto`，mermaid 的 HTML 标签（流程图、类图、状态图、思维导图等的节点名）照旧被强制成系统色，暗色高对比加亮色应用时白字画在亮的节点上；Firefox 改前就看得见，WebKit 模拟不了强制色。时间线的箭头头 `fill: var(--faint)`，与线同色（审查 B 的 P4S2B-N1：写在 `themeCSS` 里不生效）；旅程图的主线与箭头是 mermaid 写死的 `stroke="black"`，照旧。
- **4.2.6**：上面的"照实"一段；决定 904–906。
- **4.2.7**：选择器是 `pre > code:not(.language-mermaid)` 加 `alone(code)`（`pre` 的子节点里除了这个 `code` 只有空白文字）；`undone` 由清理置真，`told` 开头与 `press !== presses` 一起看它；"复制"16 px、`gap` 6 px、字重 500（上面的代码；869）。
- **5.2、6.2、7.1、7.2、7.3、7.5、8.2 第 13、16、17、18 条**：见各节；决定 901–906。

### 4.3 S3 编辑

#### 4.3.1 文件

| 文件 | 改什么 | 提交 |
|---|---|---|
| `W/editor/theme.ts` | 搜索匹配用 `--nw-mark-background`/`--nw-match-selected`，删掉三个写死色值的变量，`--nw-editor-mono` 是 `var(--font-mono)`，圆角 12 px（决定 875） | 1 |
| `W/editor/markdown.ts` | `markdownHighlight` 的标题字号、符号 `--faint`、链接正文色不画下划线、地址 `--faint`（决定 874、875） | 1 |
| `W/editor/prose-lines.ts`（新） | `proseTheme`、`proseLines` 与 `markupMarks`（决定 874、889） | 1 |
| `W/editor/loaded/yaml-quotes.ts` | 导出 `frontmatterRange(state)`（用文件里的 `opening`、`closing`，按 `state.doc` 记在 `WeakMap` 里） | 1 |
| `W/editor/source-editor.tsx` | 扩展里加 `Prec.high(proseTheme)`、`proseLines`、`markupMarks`（决定 874、889）；提交 2 加 `outline` 属性、`editOutline` 与句柄的 `reveal` | 1、2 |
| `W/editor/loaded/link-completion.ts:120-139` | `completionTheme` 的圆角、阴影、内边距、选项的圆角与字号（决定 878） | 1 |
| `W/reading/reading.css` | `:root`/`.dark` 加 `--nw-match-selected` | 1 |
| `W/design-tokens.test.ts:281-283` | 白名单只剩 `./reading/diagrams.ts`，注释改 | 1 |
| `W/design-tokens.test.ts:298-316` | `noColour` 加 `"text-start"`（编辑时大纲的按钮；审查 B 的 I4），注释写明是对齐 | 2 |
| `W/editor/edit-outline.ts`（新） | `editOutline(sink)`：构造时读、之后去抖、补完解析、读标题、映射位置（决定 876） | 2 |
| `W/pages/page/edit-outline.tsx`（新） | `OutlineBox`、`EditOutline` | 2 |
| `W/pages/page/page-outline.tsx` | 导出 `listedUpTo` | 2 |
| `W/pages/page/page-layout.tsx`、`page-panel.tsx`、`page-edit.tsx` | `OutlineBox` 的建立与传递；编辑时右栏的大纲 | 2 |
| `W/pages/page/page-editing-bar.tsx` | `SaveButton`，有未保存的修改时显出，显着时焦点在头一行右边一组里就留着，否则 `invisible`、照旧占位（决定 877、885、894、895、896） | 2 |
| `W/pages/page/conflict-panel.tsx`、`edit-lost-banner.tsx` | 样子；失锁横幅里的名字进 `bdi`（决定 878） | 2 |

#### 4.3.2 编辑器的样子

`W/editor/prose-lines.ts`：

```ts
/** proseTheme is the source editor's look as the page reads (00-M13 4.5.5): its prose in the reading's font. */
export const proseTheme = EditorView.theme({
  "&": { backgroundColor: "var(--surface)", borderRadius: "12px" },
  ".dark &": { backgroundColor: "var(--nw-code-background)" },
  ".cm-scroller": { fontFamily: "var(--font-sans)" },
  // The caret is the browser's (no drawSelection()): its colour is the content's caret-color (review B's R1-M1).
  ".cm-content": { fontSize: "15.5px", lineHeight: "1.85", padding: "22px 0", caretColor: "var(--accent)" },
  ".cm-line": { padding: "0 26px" },
  ".nw-mono-line": { fontFamily: "var(--font-mono)", fontSize: "13.5px" },
  // The frontmatter is no Markdown: lezer's heading, rule and links in it drawn as plain text (decision 889).
  ".nw-frontmatter-line": { color: "var(--muted-foreground)" },
  ".nw-frontmatter-line *": { color: "inherit", fontSize: "inherit", fontWeight: "inherit" },
  // markupMarks' (decision 889): a faint mark fades the highlighting's spans inside it too.
  ".nw-md-faint, .nw-md-faint *": { color: "var(--faint)" },
  ".nw-md-link": { textDecoration: "underline", textDecorationColor: "var(--nw-link-line)", textUnderlineOffset: "3px" },
});
```

`SourceEditor` 的扩展里写 `Prec.high(proseTheme)`（决定 874：CodeMirror 倒过来挂样式模块，同优先级时排在前面的主题赢；初稿排在 `editorTheme` 之后被它盖过，审查 B 的 I2）。`editorTheme` 的 `minHeight: 20rem`、边、`.cm-focused` 的外框照旧（l5 量外框；对比视图照旧用 `editorTheme` 加等宽）。编辑器的盒子就是内容列（720 px），进出编辑时列不动；字比阅读时右移 27 px（样稿 `.editor` 的内边距）。

`markdownHighlight`：

```ts
{ tag: tags.heading1, fontWeight: "650", fontSize: "1.29em" },   // 样稿 .editor .h1：20px / 15.5px
{ tag: tags.heading2, fontWeight: "630", fontSize: "1.065em" },  // .h2：16.5px
{ tag: tags.heading3, fontWeight: "620" },
{ tag: [tags.heading4, tags.heading5, tags.heading6], fontWeight: "600" },
{ tag: tags.link, color: "var(--foreground)" },   // 下划线由 markupMarks 给（决定 889）
{ tag: tags.url, color: "var(--faint)" },
{ tag: [tags.processingInstruction, tags.contentSeparator, tags.labelName], color: "var(--faint)" },
// strong、emphasis、strikethrough、monospace、quote 照旧
```

（共用，对比视图同换：标题小一些、符号与地址淡一些、链接是正文色，都是等宽；对比视图没有 `markupMarks`，wiki 链接的外层括号与 `==` 照旧是正文色。）高亮的 `span` 是扁平的：一段字只包一层，它所在的各节点（连同带 `/...` 的祖先）的类叠在这一层上（第 1 节 (e)），所以下划线放在 `tags.link` 上就落在链接节点里的括号与地址上；装饰的 `span` 又与它嵌套，套在外面撤不掉里面的下划线。`--nw-link-line` 等在 `reading.css` 的 `:root`/`.dark` 里（S2），编辑器不在 `.nw-reading` 里也读得到。

#### 4.3.3 等宽的行（`proseLines`）与压淡的标记（`markupMarks`）

```ts
const mono = Decoration.line({ class: "nw-mono-line" });
const front = Decoration.line({ class: "nw-mono-line nw-frontmatter-line" });
const monoNodes = new Set(["FencedCode", "CodeBlock", "Table"]);
/** proseLines has the lines of code, of tables and of the frontmatter monospaced, those in view alone (decision 874). */
export const proseLines = ViewPlugin.fromClass(class {
  decorations: DecorationSet;
  constructor(view: EditorView) { this.decorations = linesOf(view); }
  update(update: ViewUpdate) {
    if (update.docChanged || update.viewportChanged || syntaxTree(update.startState) !== syntaxTree(update.state)) {
      this.decorations = linesOf(update.view);
    }
  }
}, { decorations: (plugin) => plugin.decorations });
```

`linesOf(view)`：`frontmatterRange(view.state)` 一次（按 `state.doc` 记下的，同一份文档不再扫：没闭合的 frontmatter 要扫到第一个空行，初稿每滚一下都扫，审查 A 的 N4）；对每个 `visibleRanges` 的 `{ from, to }`：先给落在 frontmatter 里的行加 `front`，再 `syntaxTree(state).iterate({ from, to, enter })`：`enter` 遇到起点在 frontmatter 里的节点返回 `false`；`monoNodes` 里的节点把它与可见范围相交的每一行加 `mono`、返回 `false`；只进 `Document`、`Blockquote`、`BulletList`、`OrderedList`、`ListItem`（别的返回 `false`，不往行内走）。每行至多一个装饰（记下上一个加过的行首），`RangeSetBuilder` 按位置加。`frontmatterRange` 与 `link-completion` 判断光标在不在 frontmatter 里同一份规则（`frontmatterAt` 改为用它）。M12/P4/S5 的上限照旧：不多解析，只读已有的树。

**`markupMarks`**（同文件，决定 889；第 1 节 (e) 的探针在三种引擎里画出的就是这样）：

```ts
const faint = Decoration.mark({ class: "nw-md-faint" });
const link = Decoration.mark({ class: "nw-md-link" });
/** The nodes whose text is code or an address: no mark in them. */
const verbatim = new Set(["InlineCode", "CodeText", "FencedCode", "CodeBlock", "URL", "Autolink", "HTMLTag", "HTMLBlock",
  "Comment", "CommentBlock", "ProcessingInstructionBlock"]);
const wiki = /!?\[\[([^[\]\n]*)\]\]/g;
const highlight = /==(?=[^=\s])([^=\n]*?[^=\s])==/g;
/** markupMarks fades a wiki link's and a highlight's marks and underlines a link's text, in the lines in view alone. */
export const markupMarks = ViewPlugin.fromClass(class {
  decorations: DecorationSet;
  constructor(view: EditorView) { this.decorations = marksOf(view); }
  update(update: ViewUpdate) {
    if (update.docChanged || update.viewportChanged || syntaxTree(update.startState) !== syntaxTree(update.state)) {
      this.decorations = marksOf(update.view);
    }
  }
}, { decorations: (plugin) => plugin.decorations });
```

`marksOf(view)`：`frontmatterRange` 之后的部分，对每个 `visibleRanges` 的 `{ from, to }`：

- **`[文字](地址)`**：`syntaxTree(state).iterate({ from, to, enter })` 进到 `Link` 节点（`FencedCode`、`CodeBlock`、`InlineCode` 与 frontmatter 里的返回 `false`）：它有 `URL` 子节点、前两个 `LinkMark` 之间有字时，那一段加 `link`；括号与地址由高亮压淡（`LinkMark` 是 `processingInstruction`，`URL` 是 `url`）。图片（`Image`）不加。只有 `[文字]` 的（引用式的简写，lezer 也读成 `Link`）不加：它的括号已淡。
- **wiki 链接与高亮**：可见范围里的每一行（frontmatter 里的除外）以 `wiki`、`highlight` 两个正则找；起点（高亮另看终点）在 `verbatim` 的节点里的不算（`syntaxTree(state).resolveInner(pos, 1)` 往上找）。wiki 链接：`[[`（嵌入连同 `!`）、`]]` 加 `faint`；页名（到第一个 `#` 或 `|` 之前）加 `link`，没有页名时（`[[#小节]]`）`#` 之后到 `|` 之前的标题加 `link`；之后的 `#`、`|` 各加 `faint`，小节名与别名照正文。高亮：两端的 `==` 加 `faint`，中间照正文（样稿 `.f`）。
- 装饰按位置排好再交给 `RangeSetBuilder`。只碰可见的几十行；文档、可见范围或语法树变了时重建（同 `proseLines`）。

代价与边界：正则按行认，`[[`、`==` 跨行的不认（服务端的写法里它们也不跨段，跨行的少见）；一段行内内容长过 `inlineLimit` 时 lezer 不认其中的行内代码，那里的 `==` 也压淡；对比与冲突视图不加 `markupMarks`（它们只换配色）。

#### 4.3.4 编辑时的大纲

`W/editor/edit-outline.ts`：

```ts
/** A heading the edit's outline lists: where its line started as it was read, its level, its text. */
export type OutlineHeading = { from: number; level: number; text: string };
/** OutlineSink is the page's side of the outline: how many it lists at most, and what it is told. */
export type OutlineSink = { upTo: number; told(listed: readonly OutlineHeading[], more: number): void };
/** How long after the last change the outline is read again. */
const settles = 300;
/** How long a slice of the parse the outline completes takes at most. */
const slice = 20;
export function editOutline(sink: () => OutlineSink | undefined): Extension { … }
/** where is the line's start of the outline's index-th heading as the editor's text is now: its place told, mapped since. */
export function where(view: EditorView, index: number): number | undefined { … }
```

一个 ViewPlugin（决定 876）：

- **构造时立刻读一次**（不等计时：进编辑时右栏的大纲不空 300 ms，审查 A 的 M3）；之后每次 `docChanged` 排一个 300 ms 的计时（重新计时）。
- **读**（`read`）：`syntaxTreeAvailable(view.state, doc.length)` 为假时 `ensureSyntaxTree(view.state, doc.length, slice)`，返回 `null` 就 `setTimeout(0)` 再来一片；下一片之前 `view.state.doc` 与这一轮开始时的不是同一个（文档改了）才放弃这一轮、重新排 300 ms 的计时，`view.state` 换了而文档没变（移动光标、补全、解析工作者自己的 `Language.setState`，`@codemirror/language` `dist/index.js:622`）时用新的 `view.state` 接着补（审查 A 的 I4：初稿遇到这些事务就放弃、又不重排，文档不变就永远没有大纲）；返回树时用它。`headingsOf(state, tree, sink.upTo)` 依次 `enter`：`ATXHeading1`–`6`、`SetextHeading1`–`2` 记下（起点在 `frontmatterRange` 里的不算；文字是节点的文字去掉 `HeaderMark` 子节点与 setext 的下划线行、`/\s+/g` 归一、`trim`，空的不列），够 `upTo` 个之后只数；只进 `Document`、`Blockquote`、列表与列表项。`sink().told(listed, more)`；结果与上次相同（逐项比较）时不报。
- **位置**：插件记着上次报出的各项的 `from`（一个数组），之后每次 `docChanged` 以 `update.changes.mapPos(from, 1)` 就地映射（至多 1,000 个数，不报出去、不重画右栏）；`where(view, index)` 答映射过的位置（审查 A 的 M2：打字之后 300 ms 之内点一项，初稿用报出时的 `from` 会跳错行）。
- **`destroy`**：清掉 300 ms 的计时与补的那一片的 `setTimeout(0)`（审查 A 的 N5：不清，剩下的一片会写进已 `clear()` 的 `OutlineBox`）。

`SourceEditor`：`Live` 加 `outline`，扩展里 `editOutline(() => this.live.current.outline)`；句柄加：

```ts
/** reveal puts the cursor at the start of the outline's index-th heading's line, scrolled near the window's top, the editor focused. */
reveal(index: number): void;
// const pos = where(view, index); if (pos === undefined) return;
// The line's start: what is typed at its very start comes before the mapped position, and a heading in a quote or a
// list item starts after its "> " or "- " (review A's R1-N3).
// const start = view.state.doc.lineAt(pos).from;
// view.dispatch({ selection: { anchor: start }, effects: EditorView.scrollIntoView(start, { y: "start", yMargin: 16 }) }); view.focus();
```

页这一边，`W/pages/page/edit-outline.tsx`：

```tsx
/** OutlineBox is the edit's outline as its editor last told it, and the way to a heading's line (decision 876). */
export class OutlineBox implements OutlineSink {
  readonly upTo = listedUpTo;
  listed: readonly OutlineHeading[] = [];
  more = 0;
  reveal: ((index: number) => void) | undefined = undefined;
  constructor() { makeAutoObservable(this, { upTo: false, reveal: false }); }
  told(listed: readonly OutlineHeading[], more: number) { this.listed = listed; this.more = more; }
  clear() { this.listed = []; this.more = 0; }
}
export const EditOutline = observer(function EditOutline({ box }: { box: OutlineBox }) { … });
```

`EditOutline`：`listed` 空时不画；否则 `<PanelSection title={t("page.outline")} icon={List}><nav aria-label={t("page.outline")}><ul className="border-l border-border text-[13px]">`，每项 `li.-ms-px.border-l-2.border-transparent.py-1.forced-colors:border-l-0`（缩进同阅读时，从列出的最高级算起；上下的 4 px 在 `li` 上，审查 B 的 M1），里面 `<button type="button" onClick={() => box.reveal?.(index)} className="w-full ps-3 pe-1 text-start leading-snug break-words line-clamp-2 text-muted-foreground hover:text-foreground">`；多出的同阅读时。`key` 是序号（审查 A 的 M2：`key={from}` 时一次异步的文档改动换掉拿着焦点的按钮，焦点落到 `body`）。`text-start` 要进 `design-tokens.test.ts` 的 `noColour`（同提交，审查 B 的 I4）。`PageShell`：`const [outline] = useState(() => new OutlineBox())`，交给 `PageEdit`（`<SourceEditor outline={outline} …>`，挂上时 `outline.reveal = (index) => editor.current?.reveal(index)`，卸下时 `outline.clear()`、`reveal = undefined`）与 `PagePanel`（编辑时在最上面画 `<EditOutline box={outline} />`）。页这一边只 `import type { OutlineHeading, OutlineSink }`，主包里不进 CodeMirror（`out-of-main.ts`）。编辑器的分包第一次加载期间（`Suspense`）右栏的大纲照旧空着，加载过之后不再有这一段。

#### 4.3.5 "保存"

`PageEditingBar`（决定 877、885、894、895）：

```tsx
const { group, before } = useEditHeadSlots();
// Whether a Save showing is kept (decision 895): from the focus in the head's right group with something to save until it
// leaves the group, so that the keys on their way to Save find it there however the autosave falls (review A's R1-M2);
// the focus coming into the group shows no Save that was hidden (review A's R2-M1).
const [held, setHeld] = useState(false);
useEffect(() => {
  if (group === null) return;
  const into = () => { if (editing.unsaved) setHeld(true); };
  const out = (event: FocusEvent) => {
    const to = event.relatedTarget;
    // A window losing the focus keeps the active element where it was: not left (section 1 (g)).
    if (!(to instanceof Node && group.contains(to)) && !group.contains(document.activeElement)) setHeld(false);
  };
  group.addEventListener("focusin", into);
  group.addEventListener("focusout", out);
  return () => { group.removeEventListener("focusin", into); group.removeEventListener("focusout", out); };
}, [group, editing]);
// Something to save coming as the focus is in the group already (a link an upload puts in as Done waits): kept as well.
useEffect(() => {
  if (editing.unsaved && group?.contains(document.activeElement) === true) setHeld(true);
}, [group, editing.unsaved]);
…
{before && createPortal(<SaveButton save={save} shown={editing.unsaved || held} />, before)}
```

`SaveButton` 是 `<Button variant="outline" size="sm" shape="pill" className={cn(!shown && "invisible")} onClick={save}>`：不显出时 `visibility: hidden`，占着位置、不可聚焦、不在可访问性树里（第 1 节 (j) 在三种引擎里核对了 `focus()` 拿不到焦点），不卸下（决定 894：卸下时状态右移一个按钮宽）。保存成功、`unsaved` 变假时，焦点在组里（它是有修改时进的组，或在组里时才有了修改），"保存"留着；按它时 `save()` 在 `version === savedVersion` 时返回，不做事。没有修改时焦点进组（点开关、点"完成"，键盘从面包屑 Tab 进来），`into` 不记，"保存"照旧藏着（审查 A 的 R2-M1）；从编辑器 Shift+Tab 到"完成"时打过字、`unsaved` 为真，照旧记下（R1-M2 的走法）。藏着的"保存"拿不到焦点，焦点在它上面时它已经显着，决定 895 的"或焦点就在它上面"不另判。挂上时焦点在标题或编辑器上（`enter()`；`source-editor.tsx:297`），`held` 从假起：第 1 轮挂上时的 `setHeld(group.contains(document.activeElement))` 去掉，它不看有没有修改。藏起时"保存"照旧占着 54 px，状态的右边离开关（窄于 1280 px 时是"完成"）约 70 px，样稿 8 px（决定 896 记的偏离）。焦点移走时三种引擎里 `focusout` 那一刻 `document.activeElement` 是 `body`（jsdom 也是，审查 A 第 1 轮的 `jsdomblur.cjs`），`relatedTarget` 是去处。没有 `useFocusLeaving`、不再需要 `focusEditor`。Mod+S 照旧。等上传（`waiting`）时规则不变。离开编辑、失锁时整条门户卸下，照各自的规则（`stayed()`、失锁横幅取焦点）。

#### 4.3.6 冲突、失锁与补全

- `ConflictPanel`：`section` 换成 `space-y-3 rounded-control border border-destructive/35 bg-surface px-4 py-3`，按钮 `size="sm" shape="pill"`，标题的焦点照旧。
- `EditLostBanner`：同样的外框；里面的文字由 `lostText` 拼，名字（拿走锁的人）经 `isolate` 进 `bdi`，横幅的 `role` 与焦点照旧。
- `completionTheme`：`borderRadius: "var(--radius-popover)"`、`boxShadow: "var(--elevation-popover)"`、`> ul` 的 `padding: "4px"`、`> ul > li` 的 `padding: "0.3125rem 0.5rem"`、`borderRadius: "var(--radius-row)"`、`fontSize: "var(--text-row)"`；选中项的底、字与内嵌的环照旧。

#### 4.3.7 编辑时右栏的次序

编辑时右栏：大纲（来自编辑器）、反链、属性、版本；`page-versions.test.tsx:403-409` 读的三节照旧在。

**照实**（S3 的实施与审查，[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）：

- **4.3.1**：表外另有新文件 `W/editor/wiki-links.ts`（`wikiLinks`、`shownOf`、`WikiLink`：照 `server/internal/platform/markdown/obsidian/wikilink.go` 的 split 与 shown、`asset.go` 的说明认 wiki 链接，编辑时的大纲与 `markupMarks` 共用；`15372fcf`、`d7fb332c`）与 `wiki-links.test.ts`；`yaml-quotes.ts` 的 `frontmatterRange` 返回 `Frontmatter`（`{ from, to, closed, opening, end, span }`），`frontmatterAt`、`fenceAt` 改读它，一份文档只扫一次；`edit-lost-banner.tsx` 的 `lostText` 一律返回 `Isolating`（没有名字的经 `plain()`）；`W/pages/page/edit-outline.tsx` 的 `OutlineBox` 另有 `ready`（不叫 `told`：`told()` 已是方法），按钮 `block`、`cursor-pointer`，`break-words line-clamp-2` 在按钮里文字的 `span` 上（`3c75954d`）；`page-panel.tsx` 的 `ColumnOutline`（编辑中、编辑器报出之前画阅读时的 `PageOutline`，`active` 为假、`inert`）；`page-outline.tsx` 加 `inert` 属性，传给 `panel-section.tsx` 的 `PanelSection`，加在它自己的 `details` 上（`d7fb332c`、`bcd1575b`）；`theme.ts` 加查找面板与 gutter 的样子（决定 910），`.cm-scroller` 用 `--nw-editor-mono`、13 px（决定 909），查找框宽 10.5rem（`e6558722`）；`markdown.ts` 的次序是 link、quote、url、processingInstruction，行内代码的字号是变量乘标题的倍数；`prose-lines.ts` 设 `--nw-editor-code`，wiki 链接经 `wikiLinks`，`LinkTitle` 当原样；`link-completion.ts` 强制色下选中项的外框；`text-diff.tsx` 那一组按钮 `flex-wrap justify-end`（`d54a9d00`）。e2e 另改了 `pg17-page-head.spec.ts`（第 3 格一步）与 `l5-completion.spec.ts`（查找面板与强制色两步），5.3 之后的照实。
- **4.3.2**：874、875 的"S3 的实施与审查之后"；`markdownHighlight` 的 h1、h2 另设 `--nw-heading`，monospace 的字号是 `calc(var(--nw-editor-code, 1em) * var(--nw-heading, 1))`（只写 `var(--nw-editor-code, 1em)` 时，标题里的行内代码只有一个 `span`，后挂的 monospace 定字号，0.871em 又是对 `.cm-line` 算的：h1 里的代码从约 20 px 变成 13.5 px，两边的反引号仍是 20 px，审查 A 的 P4S3A-F1-M1）。
- **4.3.3**：`markupMarks` 经 `wikiLinks` 认 wiki 链接（889 的"S3 的实施与审查之后"）。代价与边界另有：frontmatter 的 YAML 里只有半个围栏时（块标量里的一行 ```` ``` ````），lezer 的 `FencedCode` 从那里一直到全文末，编辑时的大纲与压淡都停在它的起点，`proseLines` 照旧（`Body` 不等宽）；很少见，照实（审查 A 的 P4S3A-N1）。
- **4.3.4**：`textOf` 照决定 907（改掉"文字是节点的文字去掉 `HeaderMark` 子节点与 setext 的下划线行"），另去掉 `QuoteMark`，setext 截在下划线那一行的行首；`upTo` 之后的标题只判有没有字（`hasText`：只有脚注引用、注释、`<br>`、`&nbsp;`、空的链接或图片的标题也计数，"…and N more"可能多算、不会少算，审查 A 修复核对第 2 轮）；字符实体在一次读里只解码一次，表是每次读新建的、读完就丢（审查 C 的 P4S3C-F2-M1），只建一个 `textarea`。决定 908。`same()` 只比级别、文字与 `more`。`ready` 由 `told()` 设真、`clear()` 设假：每个新的插件实例第一次读必报，`load` 之后、零个标题时也设真；StrictMode 的二次 effect 里最后一次是编辑器的 layout 挂上，晚于 `PageEdit` 的清理（审查 A 核对）。两片之间文档改了时这一轮直接结束、不另排 300 ms（`update()` 每次 `docChanged` 本就重排）；sink 经 `Facet` 传进插件，答 `undefined` 时不读；`listed` 是 `observableRef`（MobX 7 的名字）。`EditOutline` 的按钮 `block`（`inline-block` 时坐在 `li` 的行框里，项比阅读时高 1.6–5.5 px，审查 B 的 P4S3B-M1）、`cursor-pointer`（B 的 P4S3B-N2），截断在文字的 `span` 上（WebKit 不把 `<button>` 排成 `-webkit-box`，`3c75954d`）；`block` 与 `line-clamp-2` 在同一个按钮上时三种引擎都不截：构建出的 CSS 里 `.line-clamp-2` 排在 `.block` 之前，`display: block` 赢（变异 t29e，审查 C 修复核对第 1 轮查到）。已知的差别（决定 907）：`[see [[A]]](u)`、`[![[img.png]]](u)` 的外层 lezer 不读成链接，编辑时是 `[see A](u)`、`[img.png](u)`；行内数学 `$…$` 照原样；有定义的简写引用 `[r]` 编辑时照原样；带点的页名加显示文字（`![[Node.js|JS]]`、`![[Release 2.0|Rel]]`）当作附件、显示了显示文字，不存在的附件带说明（`![[missing.png|Gone caption]]`）显示了说明，服务端都显示目标（编辑器不知道目标找没找到，只凭扩展名猜，扩展名不含空白，审查 B 的 P4S3B-F2-N1）；图片替代文字里的 `%%…%%` 编辑时剪掉、服务端当文字，之前的段落开的块注释里的标题服务端隐去、编辑时照列（审查 A 修复核对第 2 轮）。
- **4.3.6**：878 的"S3 的实施与审查之后"；对比与冲突视图照决定 909。
- **5.3、6.3、7.1、7.2、7.3、7.5、8.1 第 1 条、8.2 第 19、22、24–28 条、第 9 节**：见各节；决定 907–911。

### 4.4 对比度的下限

照 4.7 与 01-P1 2.1 第 5 条：

- **S2**：l5 的"阅读视图链接的焦点"（`l5:176-180`）与"菜单的当前项"照旧量；链接的字色与下划线变了，焦点环与它周围的颜色没变。实施者照旧跑 l5，实量值写进记录；量出的值不低于下限就不改下限（决定 891）。
- **S3**：l5 的编辑器外框（`l5:142-143`）照今天的量法不降：`e2e/fixtures/contrast.ts:112-116` 量外框时取父元素链上第一个不透明的底，不是编辑器自己的底；编辑器的底由 `--background` 变成 `--surface`/`--nw-code-background` 不改它量到的数（审查 B 量过两种次序：暗 7.923、亮 3.098；B 的 M6、C 的 M1）。初稿"暗色约 7.49、l5 会红、提交 1 改下限"不成立，S3 提交 1 没有 e2e 改动（前缀 `web:`，决定 891）。实施者照旧跑 l5、记实量值。变异 59（`--accent` 的暗色调淡到 `#b88a3c`）对父元素的底 `#1a1b1e` 约 5.54，照旧被 l5 抓住。补全的说明字与选中项的环颜色不变，照旧量。量法是否改成编辑器自己的底不在 P4。
- pg4、se1：P4 不碰确认框、搜索页与侧栏，不重量。

### 4.5 lint、分包与设计基础的检查

- `design-tokens.test.ts`：S1、S2 不写新的色值进 TS（`code-blocks.ts` 只写类名；mermaid 的色值与 `themeCSS` 在白名单里的 `diagrams.ts`）；`reading.css` 的 `.nw-code-copy[data-state=copied]::before` 是 `color: var(--accent)` 加 `background-color: currentColor`，同 `.nw-saved`，"琥珀不是填充"不认它（P3 决定 823）；`bg-accent-soft` 是当前一项的底色，允许。"每个颜色类都是主题的颜色"把 `text-` 开头的类都当颜色读，`noColour` 里只有 `text-right`（`:298-316`）：S1 提交 1 的 `text-end`（头一行的 `meta` 盒子）与 S3 提交 2 的 `text-start`（编辑时大纲的按钮）各在同一提交加进去，注释写"alignments"（审查 B 的 I4；设计者照抄守卫跑了修订之后的全部类名，只有这两个）。S2 提交 1 起亮暗成对的一格另核对 `reading.css`（决定 870）。S3 的白名单去掉 `editor/theme.ts`，之后它与 `markdown.ts`、`prose-lines.ts` 写色值都会红。
- `out-of-main.ts`：`prose-lines.ts`、`edit-outline.ts` 只由 `source-editor.tsx` 导入（编辑器的分包）；`yaml-quotes.ts` 原本就在编辑器的分包里；页这一边的 `edit-outline.tsx` 只有类型的导入。
- knip：每个新导出都有调用者（`ColumnTop`、`ColumnTopContext`、`HeadMeta`、`PanelToggle`、`useCurrentHeading`、`useBeside`、`EditHeadSlots`、`useEditHeadSlots`、`propertyRoles`、`codeBlocks`、`frontmatterRange`、`proseTheme`、`proseLines`、`markupMarks`、`editOutline`、`where`、`OutlineHeading`、`OutlineSink`、`OutlineBox`、`EditOutline`、`listedUpTo`；S3 照实另有 `Frontmatter`、`wikiLinks`、`shownOf`、`WikiLink`）；`Mark` 的 `current` 有调用者。
- 文字：13.2 第 4 条，新键两种语言（决定 884）。

## 5. e2e

逐个核对过的范围：按文件 `grep` 了 `complementary`、`About this page`、`Outline`、`Breadcrumb`、`getByRole("status")`、`name: "Save"`、`name: "Edit"`、`nw-props`、`article`、三个反引号（只有 l4 与 pg5 写代码块，l4 的都是 mermaid）、`boundingBox`/`getBoundingClientRect`、`scrollY`/`scrollTop`、`expectFits`。默认的窗口是 1280×720（`Desktop Chrome` 等，`e2e/playwright.config.ts:24-30`），在 `xl` 之上：所有在页上读右栏的故事都在并排的布局里跑。

### 5.1 S1

**PG17 第 1 格**（新文件 `e2e/stories/page/pg17-page-head.spec.ts`，三种浏览器；标题以"PG17 (page): the page's head says"起头，`check.mjs` 的两行以它为准：`check.mjs` 以 `startsWith` 认格，起头写到"says"，免得第 3 格的"the page's head keeps"也对上）：

```
test("PG17 (page): the page's head says who edited the page last, Edit kept where it is as that comes; beside it the right column, the window's height, outlines the page and marks the section read, a folded callout's heading never, following it into view until the reader moves in it, the heading clicked marked at the page's end; keeps the line read where it is as the page is written above it, and folds away, the page's column as wide and centred, folded still once reloaded; narrower it follows the page, and at 375 wide the properties wrap", …)
```

1. 造数：`newOnboardedTeam`、`createNotebook(…, "Plans")`、`createPage(…, "Doc", null, content)`，`content` 是 frontmatter（`status: draft`、`owner: "[[Hub]]"`、`tags: [alpha, beta]`、一个 97 个字符不断开的值）加 `## Part 01`…`## Part 40` 四十节，`Part 01`–`Part 38` 每节 `"filler\n\n".repeat(12)`，`Part 39`、`Part 40` 每节 `"filler\n\n".repeat(2)`；`Part 20` 一节的末尾是一个折叠的 callout，第一行就是标题（`> [!note]- Folded\n> ## Hidden\n> inside`）。大纲 41 项，比右栏高。第 1 节 (m) 在 S1、S2 的样式下量过：一节约 580、624 px；`Part 38` 的顶到文档末尾 1,058、1,110 px（第 9 步滚得到 0）；`Part 39` 的顶到末尾 479–486 px，矮于窗口（第 10 步点它之后窗口在底、它停在 414–421 px，`Part 40` 在 568–570 px，都在视口里；初稿与修订稿每节都是 12 段，`Part 39` 到末尾 1,281 px，窗口不到底、`Part 40` 在线下，`pin` 不起作用，审查 C 的 R1-I1）。`page.setViewportSize({ width: 1440, height: 900 })`；`holdStream`，进页之后再放（照 PG15：事件流连上之后的写才是事件）。
2. 头一行：`main` 里以 `/^Edited by /` 起头的文字可见，它的 `bdi` 是造数的管理员的显示名；它不是 `status`（`main.getByRole("status").filter({ hasText: "Edited by" })` 0 个：右栏读取中的"加载中…"也是 `status`，不数全部）。名字长时只截名字（决定 899）：那一句的 `scrollWidth − clientWidth` 不大于 0，它的 `bdi` 的大于 0（造数的管理员的显示名约 50 个字符，1440 宽时放不下；照实加，`addcfa69`，变异 t14）。
3. 右栏：`complementary` "About this page" 的 `x` 不小于 `article` 的右边加 39，`y` 是 0，高是窗口的高；窗口滚到底时 `y` 仍是 0（照实：都容 0.5 px，WebKit 在页底差 −0.078 px，审查 A 的偏离 8）。
4. 大纲：点"Part 05"，标题 `toBeFocused`，它的 `y` 在 16 ± 2；"Part 05"的链接 `aria-current="location"`，别的项没有。
5. 窗口滚到"Part 10"的顶在窗口顶下 100 px（`window.scrollBy`）：`expect.poll` 到"Part 10"的链接 `aria-current="location"`（照实：设计写的顶在 0 时线在视口顶的变异 12 也读到"Part 10"，抓不到；100 px 时它读到"Part 09"，12e 红，`d29b68b0`）。
6. 折叠的 callout（决定 887）：窗口滚到"Hidden"自己的顶在线（视口高的三分之一）之上 5 px（它在关着的 `details` 里，以 `locator.evaluate((h) => h.getBoundingClientRect().top)` 读它的盒子，`boundingBox()` 对看不见的元素答 `null`）：`expect.poll` 到"Part 20"带 `aria-current`；"Hidden"的项在大纲里（`outline.getByRole("link", { name: "Hidden" })` 可见），没有 `aria-current`。这时"Part 21"在线下 27 px（S1）、42 px（S2，第 1 节 (m)），去掉 `closest` 时标的是"Hidden"（修订稿以 callout 的顶定位，"Hidden"在线下约 42 px，两种写法都标"Part 20"，抓不到变异 13，审查 A 的 R1-M4、C 的 R1-M1）。
7. 之上就地更新：窗口滚到"Part 21"的标题在 200 px；记下"Part 21"一节第一段的 `top`；经接口在"Part 01"一节里加一段（`writeContent`，`base_revision` 照读到的）；等文章里出现那一段；隔两帧（`afterFrames`，PG17 自己写一份，同 up1:252-255 的局部函数）那一段离原处不超过 1 px，`scrollY` 变大了（三种引擎的滚动锚定，00 号文档第 10 节的那一行）。
8. 右栏跟着（决定 886，审查 C 的 M10）：记下右栏的 `scrollTop`（此时 0 或很小）；窗口滚到"Part 36"的顶在 0；`expect.poll` 到"Part 36"带 `aria-current`；隔两帧：右栏的 `scrollTop` 变大了，"Part 36"的 `li` 的底在右栏的底之上、相差不超过 2 px（只滚到刚好露出，不是放到中间）；`scrollY` 仍是滚到的那个值（窗口没跟着动）。
9. 读者在右栏里滚过（决定 886）：指针移到右栏上，`mouse.wheel(0, -4000)`，`expect.poll` 到右栏的 `scrollTop` 是 0（Firefox 的滚轮可能是平滑的）；窗口滚到"Part 38"的顶在 0；`expect.poll` 到"Part 38"带 `aria-current`；隔两帧右栏的 `scrollTop` 仍是 0。
10. 到底（决定 893）：点大纲的"Part 39"（Playwright 先把它滚进右栏的可见范围）：先 `expect.poll` 到窗口在底（`scrollY + innerHeight >= scrollHeight - 1`）、"Part 40"的标题在视口里（`toBeInViewport`），再看"Part 39"的标题拿着焦点、"Part 39"带 `aria-current`、"Part 40"没有（照线算到底时标的是"Part 40"：去掉点按的 `pin`，这一步红，变异 76；初稿到底时标最后一个标题）。
11. 开关：`toggle.focus()`、`Enter`：`aria-expanded="false"`，右栏不可见（`toBeHidden`），`article` 的宽度与收起之前相同，它的中心与 `main` 的内容区（`main` 的盒子去掉左右各 40 px）的中心相差不超过 1 px（收起之前它在内容区左边那一块的中间），开关拿着焦点；`localStorage` 不读（只看页面的表现）。
12. `await reload(page)`（`e2e/fixtures/navigation.ts:108`；直接的 `page.reload()` 过不了 `tools/e2e-navigation/check.mjs`，审查 C 的 M2）：开关 `aria-expanded="false"`，右栏不可见；地址带着第 10 步点的 `#nw-part-39`，视图到时把焦点给锚点的标题（决定 900）：先等"Part 39"的标题拿到焦点，再 `toggle.focus()`、`Enter`：展开，可见。再 `Enter` 收起（照实：设计写的"刷新之后焦点在 `body`"对带锚点的地址不成立，视图答得慢时焦点在两次 `Enter` 之间被拿走，第二次打在 h2 上、右栏不收起，审查 C 的门禁在负载 62 时红过，审查 C 的 P4S1C-F1-I1；`19518390`，扣住视图的 `GET` 的探针下改前红、改后绿，变异 t22）。
13. `setViewportSize({ width: 1024, height: 800 })`：开关不可见，右栏可见（偏好是收起也一样），它的 `y` 大于 `article` 的底。
14. "编辑过"晚到（决定 858，审查 A 的 M4）：`setViewportSize({ width: 480, height: 800 })`（造数的管理员的显示名是 `admin-<testId>-0-0`，约 50 个字符，长得要省略；480 宽时头一行的行宽 432 px，右边一组一开始就在第二行）；以 `page.route` 扣住版本列表的 `GET`（`/api/v0/pages/{id}/versions`，带查询，谓词匹配；写法同 `holdAnswer`，`e2e/fixtures/settings-pages.ts:59`）；`goto` 页的地址（照实：不带锚点；原写 `reload(page)`，地址带着 `#nw-part-39`，视图在"Edit"的盒子记下之后才到时把窗口滚到"Part 39"，"Edit"跟着走，`f698d37e` 的门禁里 WebKit 红过一次（"how far Edit moved down"），`286d01b7`；扣住视图到"Edit"的盒子读完的探针下改前红、改后绿，变异 t23）；"Edit"可见、"Edited by"还没有，记下它的盒子；放行；"Edited by"出现之后隔两帧，"Edit"的 `x`、`y` 与之前相差不超过 0.5 px（右边一组不定宽时，它在"Edited by"到来时从一行变两行，第 1 节 (j)，变异 79）。
15. （提交 2）`setViewportSize({ width: 375, height: 800 })`：`expectFits(page)`（`e2e/fixtures/narrow.ts:11`）；属性条折了行：`table.nw-props > tbody` 的高大于它的第一个小块（第一个 `tr`）的高，第一个与最后一个 `tr` 的 `top` 不同；之后隔两帧（滚动区的标记由 `ResizeObserver` 的回调给，立刻读会先通过；审查 C 的 M3），`article.getByRole("region")` 0 个（长的值在小块里折，不比显示的宽）。属性表带 `role="table"`、它的 `tbody` 带 `role="rowgroup"`（`propertyRoles`；照实加，`d29b68b0`：没有这一句时变异 t6 活下来）。

**PG17 第 3 格**（提交 2，同一文件，三种浏览器，标题以"PG17 (page): the page's head keeps its lines"起头；决定 894，审查 A 的 R1-I1）：

```
test("PG17 (page): the page's head keeps its lines whatever its mode, the title where it is as the page is read, its content awaited, edited, typed in and saved, at 375 and 820 wide", …)
```

1. 造数：一页"Doc"，内容"One\n\nTwo"（没有 frontmatter）；管理员的显示名照造数（约 50 个字符）。
2. 375×800 与 820×800 各一遍（820 宽时左栏在，头一行的行宽约 519 px，正是修订稿阅读两行、编辑一行的那一段）：`setViewportSize`；进页（第一遍照 PG16 进页，第二遍 `reload(page)`）；等"Edited by"可见；记下标题（`pageHeading(page)`）的 `y`（窗口在页顶）。
3. 扣住内容的 `GET`（`holdAnswer(page, "GET", "/api/v0/pages/<id>/content")`，`e2e/fixtures/settings-pages.ts:59`：只扣 `GET`，自动保存的 `PUT` 照常；每一遍各扣一次）；`edit.focus()`、`Enter`；"Edit"不见了（进了编辑、内容还没到：右边一组里一个按钮也没有，375 与 820 宽时开关不显示）之后隔两帧，标题的 `y` 与第 2 步相差不超过 0.5 px（去掉 `min-h-8` 时上跳 16 px，变异 113，审查 A 的 R2-I2）；放行：编辑器出现（`editorContent(page)` 可见），`editStatus(page)` 是空的；标题的 `y` 同上。
4. 在编辑器里 `End`、打一个字：`editStatus(page)` 是"Unsaved changes"；标题的 `y` 同上；记下状态（`editStatus(page)`，`block`，宽同它的盒子）的右边。
5. 等自动保存（`editStatus(page)` 是"Saved."，至多 10 秒），焦点一直在编辑器里：标题的 `y` 同上；状态的右边与第 4 步相差不超过 0.5 px（S3 起"保存"这时 `invisible`、照旧占位；卸下时状态右移一个按钮宽，变异 108）。
6. `ControlOrMeta+e` 回到阅读：标题的 `y` 同上。

**PG17 第 4 格**（照实加，`c6297488`；同一文件，三种浏览器，标题以"PG17 (page): long names break"起头；审查 B 的 P4S1B-F1，审查 C 的 P4S1C-F2-M1、N1）：

```
test("PG17 (page): long names break, the lock's holder's in its note, a version's author's in its line: the page and its version fit at 375 wide", …)
```

1. 造数：`newOnboardedTeam`；管理员的显示名经 `PATCH /api/v0/me` 改成 `unbroken("admin")`（文件里的函数：名字、七个 `2026-11-24-`、再一个 `0`，连字符都接数字，Firefox 照 UAX #14 的 HY×NU 不在那里断行，像从地址取的名字）；`createNotebook(…, "Plans", "editor")`，一页"Doc"经接口再写一次（有版本 1 与它之后的一版）；一个成员经 `joinAs` 加入，显示名改成 `unbroken("editor")`，`openSession` 拿着这一页的锁。
2. `setViewportSize({ width: 375, height: 800 })`；`goto` 页：锁说明（`main` 里文字是"<名字> is editing this page."的 `status`）可见；`expectFits(page, 说明)`（页不横向滚动，说明在窗口里、不溢出）。
3. `goto` 版本页（`…/versions/1`）：同一个锁说明可见；`expectFits(page, 说明, 作者的链接)`（标题下"时间 · 作者"一行里名为 `unbroken("admin")` 的链接）。

先红只在 Firefox：先红在页（锁说明），锁说明修好之后再红在版本页（`scrollWidth` 660），两处都修好之后三种浏览器都绿；Chromium、WebKit 在连字符处断行，本来就绿。t17（锁说明去掉 `min-w-0 wrap-anywhere`）、t18（版本页那一行去掉）都只在 Firefox 红，vitest 的 `edit-lock-note.test.tsx`、`version-page.test.tsx` 抓不到（jsdom 不排版）：这一格是这两处唯一的守卫，所以它在 `REQUIRED` 里有两行（下面"改的"）。只核对 375 宽；320 宽与 B 那种像邮箱的名字由实量看（10.1）。

**改的**：

- `e2e/playwright.config.ts:3-4`：`threeBrowsers` 加 `page\/pg17-`（与 `pg1-`…`pg16-` 都不是前缀重叠：正则以 `-` 结尾）。
- `tools/e2e-projects/check.mjs` 的 `REQUIRED`：在 PG16 两行之后加 `// The page's head, its right column and their focus are each engine's (M13/P4/S1, decision 879).` 与 `["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): the page's head says"]`、`["webkit", …同…]`（提交 1）；提交 2 再加第 3 格的两行 `["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): the page's head keeps its lines"]`、`["webkit", …同…]`（`check.mjs:78` 以 `startsWith` 认格，两格的起头互不为前缀）。照实：`e2fd05b0` 再加第 4 格的两行 `["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): long names break"]`、`["webkit", …同…]`，注释写 Firefox 的 HY×NU 与这一格只在那里红（审查 C 的 P4S1C-F2-M1：这一格哪天挪出 `threeBrowsers` 的文件，只在 Chromium 跑就永远绿）；四格的起头（S2 的"a code block's head"在内）互不为前缀，`check.mjs` 列 chromium 430、firefox 102、webkit 102、webkit-narrow 3。
- UP1（`e2e/stories/upgrade/up1-start-freeze.spec.ts`，提交 1，已在 `threeBrowsers` 里；决定 865，审查 A 的 I2）：另一个标签页读到页中间之前先把焦点给"编辑"（`await edit.focus()`，在 `paragraph.scrollIntoView` 之前；之后的滚动不动焦点）；升级开始、"编辑"没了之后加 `await expect(tab.getByRole("heading", { level: 1, name: "Plan" })).toBeFocused()`；`:248` 的"读的那一段移动不超过 1 px"照旧，现在它同时守着 `h1.focus()` 的 `preventScroll`（初稿的写法下三种引擎的 `scrollY` 从页中间回到 0）。标题里"…loses Edit, without a reload,"换成"…loses Edit, its focus going to the title without a scroll, without a reload,"（`REQUIRED` 里没有 UP1 的行）。照实：那一句找标题写 `tab.getByRole("heading", { level: 1, name: "Plan" }).first()`：页的正文自己也有一个 `# Plan`，壳的 `h1` 在它之前；焦点若落到文章的那个上照样红（审查 A 的偏离 6）。
- pg5（`e2e/stories/page/pg5-reading-view.spec.ts:86`，提交 2）：标题里"shows the page's properties first"换成"shows the page's properties first, as a strip"；断言不改（`:128-129` 读 `:scope > :first-child > table` 的类与 `textContent`，`propertyRoles` 只加 `role`）。

**不改的**（理由）：

- 夹具：`pageHeading`（`e2e/fixtures/wiki-pages.ts:44-46`，`main` 里唯一的 `h1`）；`breadcrumbs`（`:162-171`，每个 `li` 里不带 `aria-hidden` 的子元素，结构照旧）；`editStatus`（`:233-235`，编辑时 `main` 里唯一的 `status`：头一行的状态是它，`HeadMeta` 是 `<p>`，提交 2 之后它在头一行里、仍在 `main` 里）；`saveEdit`（`:238-241`，Mod+S）；`versionsListed`（`:335-340`，右栏里名为"Versions"的列表）。
- l6（`e2e/stories/links/l6-panel.spec.ts`）：`:215` 右栏的 `x` 大于正文的右边（内容区展开时右边留 40 px）；`:218-228` 大纲的链接、点了之后标题拿焦点、在视口里，`:229` 右栏在视口里（粘住、高同窗口）；`:234-240` 编辑时没有大纲（S3 改）；`:286-293` 按"更多"之后窗口与右栏都没滚（Hub 没有标题，读到的一节没有、右栏不动）；`:319-320` 级别 2 的"Properties"与 `dl dt` 一个（`h2` 在 `summary` 里、`dl` 照旧）；`:332-333` 1,000 个链接与"…and 139000 more"。
- hi1:256-265、hi2:111-112（版本三项、点第三项：版本一行的结构变了，项与链接照旧；照实：hi1:258、hi2:113 读整项的文字，4.1.6 把标记挪到谁写的之后，整项随之读作"…· Edit"接"Current"，两处照新次序改了，`f7eb1100`，审查 A 的偏离 4）；hi3:358（以 `details` 与文字找小节）；as3:323、as6:133,150（右栏里的属性链接，`dd` 照旧）：结构与文字不变。
- 读锁说明的故事：c1:66、c3:86、c6:60、c7:243、c10:211、hi2:137 以 `getByText("… is editing this page.", { exact: true })` 找它，Playwright 的文字含 `bdi` 里的名字，照旧找得到（vitest 的不同，7.1）。
- ui1:65（`main h1`）；ui1:54、pg2:211、lw3:345 的 `expectFits`：头一行在 375 宽时折行、不溢出（第 1 节 (b)）。
- up1:240-244：横幅在内容列里、标题之前，不在 `article` 里，文章的第一个节点照旧；之上的变化由滚动锚定吸收（≤ 1 px），同今天。
- lw5:546 数整页名为"Edit"的按钮（0 个）：开关名为"About this page"。
- c7:225、c8:149、c9:59 读 `article` 的整段文字：那几页没有代码块与属性（S2 的头不进它们的文字）。
- pg15（就地更新，三种浏览器）：它的页没有代码块与属性；`main` 的留白与两栏不改相对的位置。PG15 与 LW8 的文件不改，第 1 节的基线行照旧。
- 读页面菜单的故事（`Page actions`，l5:175 等）：名字不变，按钮小了一号。

### 5.2 S2

**PG17 第 2 格**（同一文件，三种浏览器，标题以"PG17 (page): a code block's head"起头）：

```
test("PG17 (page): a code block's head names its language and copies its code alone, its button turning to a check a moment, held at the block's left as the code scrolls sideways; a done task's own text is struck and faint, not its nested items' nor its blocks'; a property's map is a chip as tall as a word's; a callout's title is a line of text, its markup in it, at 375 wide too, its blocks apart as a list item's are", …)
```

1. 造数：一页，frontmatter 里一个列表值是映射的属性（`people` 两项）与一个列表值是词的（`stages`），一个 `go` 代码块（一行长过 720 px），一个已完成的任务项：它下面两个 `sh` 代码块、一个带标签的 callout、一段引用、一个没完成的子项与一个折叠的 callout（引用与 callout 之间隔着代码块或子项，不然合成一段引用）；两个带标记的 callout 标题（一个带加粗、`[[Doc]]`、行内代码与一个长地址，三段正文；一个折叠的带加粗、链接与代码），一个松散列表与一个紧凑列表（项的那一行下一个代码块与一个嵌套项）。1440×900；`grantClipboardPermissions(page.context(), browserName)`（`e2e/fixtures/clipboard.ts:6`）。
2. 头：`pre` 里的 `.nw-code-head` 以"go"起头；`pre.getByRole("button", { name: "Copy", exact: true })` 可见。
3. 点"Copy"：按钮带 `data-state="copied"`；`.nw-code-said` 是"Copied"、带 `data-state="copied"`，它的盒子宽不超过 1 px（只给读屏，决定 890）；`readClipboardText(page, browserName)`（`:20`）等于 `pre > code` 的 `textContent`；之后（1.5 秒）按钮不再带 `data-state`、`.nw-code-said` 变空，按钮的名字一直是"Copy"。
4. `pre` 是名为"Code"的区域；`scrollLeft = 200` 之后头的左边等于 `pre` 的左边加 1（边框）± 1。
5. 已完成的项（审查 C 的 M12；S2 的修复）：它的 `li`（`:scope > ul > li` 的第一个，属性条里也有 `li`）的 `text-decoration-line` 含 `line-through`、`color` 是 `--faint` 的值（读 `:root` 的变量算出的颜色）；它下面的块依次是 `pre`、`div`（callout）、`pre`、`blockquote`、`ul`、`details`，各是 `inline-block`、与项同宽，颜色是 `article` 的（引用是 `--muted-foreground`），离上面的那一行或块 8、8、8、8、4、8 px；callout 里的标签是 `--muted-foreground`（`9685e300`、`02cb019e`、`ade3abd1`）。
6. 属性条（S2 的修复核对第 1 轮，审查 B 的 P4S2B-F1-M1）：`people` 的第一个小块与 `stages` 的第一个小块同高（取到 0.25 px），映射的嵌套表 `margin-top` 是 0。
7. callout 的标题（决定 901）：两个标题的 `innerText` 是连续的一行文字（`.nw-callout` 取 `:scope >` 的，已完成项里的不算）；1440 宽时第一个不超过 2 行、第二个 1 行（上限，不是等于：Linux 的字体更宽时不至于红，审查 C 修复核对第 1 轮的未核实一条），375 宽时各不超过 4 行，`expectFits` 核对视图、callout 与两个标题不横滚、不出屏。
8. 块的间距（决定 902、903）：第一个 callout 的子元素的 `margin-top` 是 0、4、8、8 px，松散项的两段 0、8 px，紧凑项里的 `pre` 8 px、嵌套的 `ul` 4 px。

**改的**：`check.mjs` 的 `REQUIRED` 再加 `["firefox", "page/pg17-page-head.spec.ts", "PG17 (page): a code block's head"]`、`["webkit", …同…]`（注释同上，Step 写 S2）。pg5（只在 Chromium）加一段：选中整个 `pre`（`selectNodeContents`）、`ControlOrMeta+c`，剪贴板里是代码的字，不带"go"与"Copy"（as6:97-110 的写法：先写一句别的进剪贴板，再读）；标题加"…, a code block selected copying its code without its head"。L4 (page) 第一格（`cdedff02`，`e2e/stories/links/l4-tags-math.spec.ts:196-206`，只在 Chromium）：暗色重画之后 `emulateMedia({ forcedColors: "active" })`，先断言 `matchMedia("(forced-colors: active)")` 为真（不空过；WebKit 模拟不了，在那里会红、不会空过），再断言 `.nw-diagram` 的底与"Start"节点 `.nodeLabel` 的颜色同强制之前，最后还原成 `forcedColors: "none"`；L4 经 `emulateMedia({ colorScheme: "dark" })` 进暗色，模拟的是暗色高对比加暗色应用（主持者定：比的是强制前后不变，够了，审查 C 的 P4S2C-Q1）。

**不改的**：pg5:158-163（关键字与 `code` 的颜色不同：新的关键字色 `#a0661a` 不是正文色）；pg5:156（`article` 不横向滚动）与 `:151`（区域一个：Go 代码块不宽）；l4:175（KaTeX 的字体）、l4:185-194（换主题重画：缓存键照旧有主题；它只看 `svg` 的 `id` 变了，两种主题取同一份颜色时照样变，守颜色的是 `diagrams.test.ts` 与 7.4 的截图，审查 C 的 M4）、l4:361（不画的 mermaid 显示源码：`language-mermaid` 不加头）（L4 的行号是 `cdedff02` 起的）；se1:276-277,324-325（搜索页的命中高亮：不在 `.nw-reading` 里，`--nw-mark-background` 的值不变）；l5:171（`article` 里唯一的 `code` 是行内代码：头是 `div`）、l5:176-186（4.4）。

### 5.3 S3

**改的**：

- l6（`e2e/stories/links/l6-panel.spec.ts:181`）：标题"…and is not shown while the page is edited…"换成"…and, while the page is edited, comes from the editor, leading to a heading's line…"；`:236` 的 `await expect(outline).toHaveCount(0);` 换成：`outline.getByRole("button")` 是 `["Intro", "Setup", "Usage", "Hidden"]`（frontmatter 的 `---`/`up:`/`see:`/`---` 在 lezer 里是水平线加二级 setext 标题，不列，决定 876；callout 里的 `## Hidden` 列出）；点"Usage"：编辑器拿着焦点（`editorContent(page)` 的 `toBeFocused`），`## Usage` 那一行在视口里。`:239-240` 回到阅读之后的链接照旧。

**不改的**：l5（决定 891：外框照今天的量法不降，S3 照旧跑、记实量值）；编辑、冲突、锁、自动保存的故事（pg7–pg10、c1–c6、lw4、lw12、up1）经 `saveEdit`（Mod+S）、`editStatus` 与按钮的名字，"保存"不被点（e2e 里点"Save"的 7 处都在对话框与设置里：a8:58、as1:196、lw5:597,613、`wiki-pages.ts:100`、`workspace-pages.ts:80`、`notebook-pages.ts:90`）；as2:199-204（编辑器的内容填满它，点下方落在内容里）：内边距在 `.cm-content`/`.cm-line` 里，`minHeight` 照旧；补全的故事（l5、lw 的几个）读选项的名字与 `aria-selected`，样子换了、结构不变；读编辑器文字的故事（`editorContent(page)` 的 `toHaveText`、`fill`）不受 `markupMarks` 影响（装饰不改文字）。

**照实**（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）：

- l6 的这几步在 `45a477bd` 上是 `:245-248`（编辑时大纲的项 `:245`，点按之后的焦点 `:247`，`## Usage` 那一行在视口里 `:248`），回到阅读之后的链接 `:251-252`。L6 另加一个带行内标记的标题 `` ## Run **it** with `make`, [[#Setup|the setup]] and [docs](https://x.test) ``，阅读与编辑时项的文字都是"Run it with make, the setup and docs"（决定 907，`:199`、`:245`）；用同页的 `[[#Setup|…]]`：指向别页的 wiki 链接多一条索引的链接，`fixtures/assert/links.ts:43` 的 `expectLinked` 由 API 两格与页面一格共用，不能只为页面这一格加一行。
- PG17 第 3 格加一步（`:343-357`，三种浏览器，`REQUIRED` 的起头不变，`check.mjs` 的数不变）：编辑时大纲的项与阅读时同高（0.5 px 内）、手形；一个长标题的按钮高不超过两倍行高加 0.5 px、又大于 1.5 倍（审查 B 的 P4S3B-M1、N2，C 的 P4S3C-M1）。
- L5 (page) 从"不改的"挪到改的（`:135-212`）：强制色下补全的选中项是实线的外框（`:174-178`，Chromium，审查 B 的 P4S3B-M4）；查找面板一步（`:193-212`，决定 910）。

### 5.4 375 px 与 `webkit-narrow`

PG17 在三种浏览器里跑，不带 `@narrow`（`webkit-narrow` 只跑带 `@narrow`、又不在 `threeBrowsers` 里的格，`playwright.config.ts:30`）：它的 375 px 两段（第 1 格第 15 步的属性条、第 4 格的长名字）在三种引擎里以 `expectFits` 核对，比 `@narrow` 多一种；第 3 格 375 宽那一遍只量标题与状态。`webkit-narrow` 的必需格（MC3、PG2、SE1）不变。

### 5.5 13.4 第 3 条

PG17 只有页面版本：它核对的是界面的行为（头一行、右栏、开关的记住、长名字的折行、代码块的头），写的只有造数（第 4 格的改名、加入与开会话也是造数）与一次经接口的写（为了就地更新），没有页面写、接口读的数据；接口版本与 MCP 版本豁免，同 PG16（8.2 第 10、17 条）。UP1 只加两句断言（标题拿着焦点、读的那一段不动），它照旧是升级的界面一侧，13.4 第 3 条里它的说法不改。

## 6. 实施步骤

三个 Step 各在自己的 worktree 里，分支从 `m13-ui-refresh` 拉出。每个提交单独为绿：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`；改了行为、夹具或故事的提交另经它跑完整的 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make e2e`，只败第 1 节那几格（同行同消息）。新的测试先写、在没改的树上看到它红，再改代码。提交说明的作用域照实写：动了 `e2e/` 的写 `e2e`，动了 `tools/e2e-projects/check.mjs` 的写 `tools`，只动 `web/` 的只写 `web`（决定 891）。文档提交写 `docs: … (M13/P4/S<m>)`，括号前一个空格（审查 C 的 N5）。

每个 Step 合并进 `m13-ui-refresh` 之后，在它上面补一个文档提交，把合并的提交号填进占位：`docs: S<m>'s merge recorded in P4's design, 00-M13 and its review (M13/P4/S<m>)`（02-workflow 3.6 第 5 步，同 P3）。S3 合并、记下合并之后，另写 `docs: M13/P4 done (M13/P4)`（02-workflow 3.7，8.1 第 4 条；审查 C 的 M13）。

### 6.1 S1

- [ ] **开工**：不另跑基线（决定 883）；提交 1 的 `make e2e` 兼作核对，第 1 节之外的格红了先停下汇报。
- [ ] **提交 1 `web, e2e, tools: the page's head in one row with who edited it last, the right column the window's height beside it and folding away, its outline marking the section read (M13/P4/S1)`**：先写 7.1 的提交 1 各格（新的 `current-heading.test.ts`、`column-top.test.tsx`，`page-panel.test.tsx` 的开关、读到的一节、展开与收起时的量、右栏的滚动、视图晚到时点大纲的项与点按的 `pin`（带修饰键的四种不 `pin`），`page-layout.test.tsx` 的头一行与"编辑"的焦点，`preferences.store.test.ts` 的偏好，`upgrade-banner.test.tsx` 的横幅（含页树读取失败一格）与 `general-page.test.tsx:665-688`（照旧），`page-versions.test.tsx` 的版本一行与随之改的 `:103`、`version-restore.test.tsx:156`）、PG17 第 1 格的第 1–14 步与 UP1 的两句，看到它们红（PG17 三种浏览器都红：没有"Edited by"；UP1 三种浏览器都红在新加的 `toBeFocused`：今天"编辑"卸下时焦点落到 `body`，这条路上还没有 `h1.focus()`；"不滚动"那一句在变异 70 下才红，审查 C 的 R1-N1）；4.1.1 表里"提交"一列有 1 的（含 `design-tokens.test.ts` 的 `"text-end"`）；`threeBrowsers` 与 `REQUIRED`，`node tools/e2e-projects/check.mjs`；`make e2e`；绿；全部 vitest 另跑三遍（`docs/v0.1/claude_to_codex/tools/heavy.sh make test-web` 三次），一次也没有"not configured to support act"，记进 10.1（第 1 节 (k)：新写法下第一次设状态跟着别的更新同一次提交、不另添窗口，审查 A、C 第 2 轮只推了理、没跑全部）。审查时按 4.1.2–4.1.7 的块看。
- [ ] **提交 2 `web, e2e, tools: the properties as a strip, their roles told; the edit's status and buttons in the page's head, its lines the same in every mode, a note under its title (M13/P4/S1)`**：先写 7.1 的提交 2 各格（新的 `property-roles.test.ts`，`page-edit.test.tsx` 的头一行、状态在 `meta` 盒子里、编辑说明与 `stayed()`，`edit-lock-note.test.tsx` 的 `bdi`，读锁说明的九处 `findByText` 改写）、PG17 第 15 步与第 3 格，看到红（第 3 格在提交 1 之后的树上红在第 5 步：状态还在内容列里那一条的左端，`page-editing-bar.tsx:53-54` 的 `justify-between`，"Saved."比"Unsaved changes"短，右边左移；这是照代码推的，没跑。标题的 `y` 那几步那时已绿：右边一组的定宽与 `min-h-8` 都在提交 1，编辑时它照旧 32 px 高、宽同阅读时；它们另由变异 79（820 宽，去掉定宽）与 113（第 3 步，去掉 `min-h-8`）看红。修订稿说第 3 格在那时"照样绿"不对：没有 `min-h-8` 时编辑时头一行在 375 与 820 宽塌掉，审查 A 的 R2-I2）；4.1.1 表里有 2 的（4.1.8–4.1.10、标签组一处）；pg5 的标题；`REQUIRED` 加第 3 格的两行，`node tools/e2e-projects/check.mjs`；`make e2e`；绿。
- [ ] 7.2 的 S1 各条变异；B 审查者截图之前，实施者先截 7.4 的 S1 几张看一遍；Firefox 里从右栏复制一个属性的值，看尾随的空格还在不在，照实记进本文 10.1（决定 881）；l6 的大页（1,000 个链接、标题至多 1,000 个）上开着右栏滚一遍，以 Performance 面板量每帧 `measure` 的时长，记进 10.1（审查记录"未核实的怀疑"）。
- [ ] **提交 3 `docs: … (M13/P4/S1)`**：8.2 第 1–11 条；S1 的审查记录；本文 10.1 与照实改的各节；00 号文档第 12、13 节；`docs/v0.2/handoffs/M12-polish.md` 的"处理进展"一行（12.10 的截断一半、第 6 项的"编辑"一半、第 5、11 项的 `bdi` 里锁说明、版本页的锁说明与右栏版本的几处）；01-P1 第 9 节交接处记下 `asset-dialogs.tsx:221` 的标签组已改（决定 882），P4 那一条的四项留给 S2、S3（审查 C 的 N6）。

### 6.2 S2

- [ ] **提交 1 `web: the reading view's body as the mockup has it: its type, links, tables, callouts and task items, the highlighting's and the diagrams' colours (M13/P4/S2)`**：先改 `diagrams.test.ts` 的主题断言、加 `design-tokens.test.ts` 的成对一格（7.1），看到红；4.2.2–4.2.6；`make e2e`（pg5、l4 照旧绿；l5 照旧绿，记下量出的值，决定 891）；绿。
- [ ] **提交 2 `web, e2e, tools: a code block's head names its language and copies its code (M13/P4/S2)`**：先写 `code-blocks.test.ts`、`patch.test.ts` 与 `reading-view.test.tsx` 的焦点格、PG17 第 2 格、pg5 的复制一段，看到红；4.2.7；`REQUIRED` 两行；`make e2e`；绿。
- [ ] 7.2 的 S2 各条变异；7.4 的 S2 截图（含 callout 七族与折叠的箭头、mermaid 各种图的亮暗、hljs 亮暗、强制色下的"复制"与 callout 的图标）；看一遍换主题时浏览器的控制台有没有 mermaid 的警告，照实记进 10.2（审查记录"未核实的怀疑"）。
- [ ] **提交 3 `docs: … (M13/P4/S2)`**：8.2 第 12–18 条；审查记录；本文 10.2；00 号文档第 12、13 节；01-P1 第 9 节交接处记下 P4 那一条的三项做完：阅读视图的其余变量（决定 870）、已勾的禁用任务项（决定 872）、`reading.css` 写死的色值的"同源"注释（决定 870）；M12-polish 的"处理进展"一行（S2 不认领，第 1 项的 3 照旧，写明核对过代码块的头不添闪动）。

照实（[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)）：提交 1、2 是 `96346ed3`、`1c170cba`，说明与上面逐字相同。另有十一个代码提交：`cdedff02`（7.4 的强制色下的一张图找到的：图里的 `foreignObject`，L4 一步）；审查之后的 `b24c98df`（决定 901）、`8dfb516b`（决定 902、903）、`36eb0c5d`（决定 904）、`bbd5b46e`（时间线的箭头头，"复制"照样稿的大小）、`0e35d0dc`（`undone`、`only-child`，`reading-view.test.tsx` 的 `afterEach`）；修复之后实施者交来、主持者定的 `d43b3e6f`（`flowchart.padding`，改过的 904）、`9685e300`（已完成项下面的块）、`02cb019e`（它的标签一步）；修复核对第 1 轮之后的 `5bbf897f`（决定 906，没有指令与 frontmatter 时不解析，文字夹着 `code` 的 `pre` 不加头）、`ade3abd1`（903 的 `:is()` 去掉 `table` 与属性条一步，已完成项下面的引用与折叠的 callout，第 7 步的行数取上限）。

### 6.3 S3

- [ ] **提交 1 `web: the editor's prose in the reading's font, its code, tables and frontmatter in lines of their own, its marks faint, its links and matches in the reading's colours (M13/P4/S3)`**：先写 `prose-lines.test.ts`（`proseLines` 与 `markupMarks` 两组）、`source-editor.test.tsx` 的主题、等宽行与压淡几格、`text-diff.test.tsx` 的行高一格、`design-tokens.test.ts` 的白名单（改了之后 `editor/theme.ts` 的色值让它红），看到红；4.3.2、4.3.3、4.3.6 的补全一项；`make e2e`（l5 照旧绿，记下量出的值，决定 891）；绿。没有 e2e 的改动，作用域只写 `web`（审查 C 的 Q1）。
- [ ] **提交 2 `web, e2e: an outline while the page is edited, leading to a heading's line; Save shown only when it is needed (M13/P4/S3)`**：先写 `edit-outline.test.ts`、`page-panel.test.tsx:186-199` 的改写、`page-edit.test.tsx` 的"保存"各格（点"保存"的几格照旧：它一直挂着，决定 894，7.1），看到红；4.3.4、4.3.5、4.3.6 的冲突与失锁两项、4.3.7、`noColour` 的 `"text-start"`（4.5）；l6:181,236；`make e2e`；绿。
- [ ] 7.2 的 S3 各条变异；7.4 的 S3 截图（编辑器亮暗、frontmatter、代码块、表格、压淡的标记、长文档滚动、编辑时的大纲、"保存"出现与消失、冲突与失锁、补全）；在 Safari 里人工看一次：焦点在显着的"保存"上时切到别的窗口再回来，它不隐去（`relatedTarget` 为 `null`、`activeElement` 照旧，无头引擎模拟不了，第 1 节 (g)；审查 A 第 2 轮的未核实一条），记进 10.3（照实：没有做，交负责人人工核对，第 9 节，审查 C 的 P4S3C-Q1）；在 Chromium 里用第 1 节 (a) 的那种大文档（经 API 建页）进编辑、拖滚动条，看一遍跳动；在一篇几 MB 的文档上进编辑，以 Performance 面板量编辑时的大纲补完解析的总时长，记进 10.3（审查记录"未核实的怀疑"）。
- [ ] **提交 3 `docs: … (M13/P4/S3)`**：8.2 第 19–28 条；审查记录；本文 10.3；00 号文档第 12、13 节（P4 一行仍是"进行中"）；M12-polish 的"处理进展"一行（13.1"编辑时没有大纲"做了，第 5、11 项的失锁横幅一处）；01-P1 第 9 节交接处记下 P4 那一条的最后一项做完：编辑器的链接色与搜索匹配色（决定 875）。Phase 完成的收尾项不在这里（8.1 第 4 条，S3 合并之后的 `docs: M13/P4 done (M13/P4)`）。

照实（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）：提交 1、2 是 `ab6f4353`、`68a7e4be`，说明与上面逐字相同。另有九个代码提交：`3c75954d`（7.4 的截图找到的：WebKit 不截断编辑时大纲的长标题，截断放在按钮里文字的 `span` 上）、`9a52ddb4`（变异 t8 活下来之后补的一格：frontmatter 里 lezer 读成标题的那一行不放大、不加粗）；审查之后的 `15372fcf`（决定 907、908，`ready`，`same()`，"…and N more"与 `clear()` 两格，按钮 `block` 与手形，PG17 第 3 格一步，L6 带行内标记的标题）、`4ecc3e10`（决定 910，行内代码 13.5 px，引用里的标记淡，`markupMarks` 照服务端认 wiki 链接，强制色下补全的选中项，L5 两步）、`d54a9d00`（决定 909，冲突面板的按钮折行）、`45a477bd`（L5 另量没聚焦的查找框）；修复核对第 1 轮之后的 `d7fb332c`（`upTo` 之后只判有没有字、实体解码记下，注释不进项，页的嵌入显示目标，编辑中先画的阅读大纲 `inert`）、`e6558722`（标题里的行内代码随标题放大，查找框 10.5rem 宽，查找面板强制色的焦点外框一格）；修复核对第 2 轮之后的 `bcd1575b`（实体解码的表每次读新建，阅读大纲那一节连开关 `inert`，附件的扩展名不含空白）。第 3 项：Safari 的人工核对没有做（实施者手里没有 Safari；jsdom 里由 page-edit 的 `relatedTarget` 为 `null` 一格守，变异 68），交负责人（第 9 节）；大文档（980,777 个字符、412 节）进编辑、远跳与拖动：第一次远跳 Chromium 里一帧移 157.2 px（S3 之前 58.9 px），之后的远跳 0 px，是 CodeMirror 对没量过的行的高度估计在校正（审查 A 量过，S3 的校正幅度在对照的范围里，记给 v0.2）；几 MB 的文档（3,581,086 字节、1,501 个标题）进编辑，编辑器出现之后大纲补完解析、显出"…and 501 more"用 290 ms，这一段主线程脚本 158–160 ms，没有 ≥50 ms 的长任务（10.3）。

## 7. 测试与验证

### 7.1 每项的测试

"（1）""（2）"是 Step 的哪个提交。经 `renderApp` 的格以 `router.navigate` 或点链接导航。改了文字的格两种语言各一（决定 884）。`vi.spyOn` 的桩由 `restoreMocks` 还原，`vi.stubGlobal` 的由 `unstubGlobals` 还原（`vite.config.ts:47-48`）；别的桩格后自己还原。

**jsdom 的桩**（jsdom 不排版，审查 C 的 M7）：

- 标题与项的盒子：`vi.spyOn(element, "getBoundingClientRect")` 返回整个 `DOMRect`（`new DOMRect(0, top, 600, 28)`），算法只读 `top`、`bottom`；折叠里的标题不靠盒子判定（决定 887，`closest`），不必桩成 0。
- 窗口：`window.innerHeight` 以 `vi.stubGlobal("innerHeight", 900)`；`document.documentElement.scrollHeight`（jsdom 恒为 0，`atEnd` 的判定读它，0 时页"不能滚"）以 `Object.defineProperty(document.documentElement, "scrollHeight", { configurable: true, get: () => height })`，格后 `delete`（它在原型上，`delete` 去掉的是格里加的那一个）；`scrollY` 是 `window` 自己的可配置访问器，`delete` 会把它整个删掉，以 `vi.spyOn(window, "scrollY", "get")` 打桩、由 `restoreMocks` 还原（审查 C 的 R1-N5）。
- 右栏：`aside` 的 `clientHeight`、`scrollHeight` 以 `defineProperty` 给在那个元素上（jsdom 里都是 0，跟着的效果什么都不做，审查 C 的 I1）；`scrollTop` 是 jsdom 真的可写的属性；项的盒子随 `aside.scrollTop` 算（`li` 的 `top` = 右栏的顶 + 序号 × 28 − `scrollTop`）。
- `navigator.clipboard`：jsdom 没有，`vi.spyOn` 给不了不存在的属性：`Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } })`，格后 `delete`。
- `matchMedia`：jsdom 没有，`useBeside()` 答"是"（同文件 `useNarrow` 的写法反过来：没有时答宽屏）；"不在正文旁边"的格以 `vi.stubGlobal("matchMedia", …)` 答 `matches: false`（同 `shell-drawer.test.tsx:21-27`）。
- `ResizeObserver`：jsdom 没有；要它回调的格以 `vi.stubGlobal` 给一个记下回调的类。
- `act`：这个项目的 vitest 不开全局，Testing Library 不设 `IS_REACT_ACT_ENVIRONMENT`，`act` 之外的更新平时不报；但在别的格的 `act` 里等 `findBy` 的那一段，一次帧里的设状态会让 React 报"not configured to support act"（第 1 节 (k)）。量位置的新格把"发 `scroll`、等一帧"包进 `await act(async () => { fireEvent.scroll(window); await frame(); })`（`frame` 是等一次 `requestAnimationFrame` 的小函数）。用假计时的格里 `requestAnimationFrame` 也是假的，`await frame()` 等不到，要以 `vi.advanceTimersToNextFrame()` 推进（同 `reading/title-media.test.ts:57`；审查 C 第 2 轮的未核实一条）。

**S1，新文件**：

- `W/pages/page/current-heading.test.ts`（1）：四个标题在 −400、100、500、700（视口高 900，线在 300，`scrollHeight` 3000）：挂上之后不等一帧就是第二个（第一次量在 layout effect 里，第 1 节 (k)）；都在线下：没有当前项；第二个放在一个真的关着的 `<details>` 里（`document.body` 里拼出，盒子照旧桩成 100）：当前是第一个（决定 887），`open` 之后是第二个；页能滚且窗口在底（`scrollY + innerHeight = scrollHeight`）：当前是第四个（线是视口的底）；页不能滚（`scrollHeight` 等于 `innerHeight`）时线照旧在三分之一：当前是第二个（决定 893）；`pin("h3", link)` 之后，第三个在视口里（500）：当前是第三个，`link` 上的 `pointerdown` 不解除它，`window` 上一次 `wheel` 之后照线算（决定 893）；`pin` 时它在视口外（盒子桩成 2000）：`pin` 一返回就是它，下一帧仍是它（视图还没滚过去），盒子移到 16 之后仍是它，再一次 `wheel` 之后照线算（审查 A 的 R1-M1）；一个记下每次渲染答的值的小组件：先 `active` 答第二个，`active` 变假再变真，变真的那次渲染答 `undefined`、之后答第二个，不答收起之前的那一项（审查 A 的 R1-N5）；`active` 为假时 `pin("h3", link)`：照旧答 `undefined`，之后 `active` 变真：照线是第二个，不是第三个（审查 A 的 R2-N2，变异 114）；`active` 时 `pin` 第三个（盒子桩成 2000：在视口外、没量到过），`active` 变假再变真：照线是第二个（点了的那一项随 `active` 变假结束，变异 115）；量到的值不变的一帧不再渲染（渲染的次数不变）；同一帧里两次 `scroll` 只量一次（`requestAnimationFrame` 打桩，数 `getBoundingClientRect` 的调用）；`active` 为假时一次也不量、答 `undefined`；没有 `ResizeObserver` 时不抛错（照实：格里先放一个 `main > article.nw-reading`，才走得到 `typeof ResizeObserver` 的守卫：原先页上没有 `main`，`=== null` 先短路，去掉守卫的变异 18 在这个文件里活下来，审查 C 的 P4S1C-M1）；给了桩时观察的是视图（`main article.nw-reading`），不是 `main`（照实，`f698d37e`，变异 t21），它回调时再量；大纲没有列出标题（`ids` 为空）时不挂 `scroll`、`resize` 的监听、不观察、答 `undefined`（审查 A 的 P4S1A-N1，变异 t11）；卸下时监听都去掉。
- `W/reading/property-roles.test.ts`（2）：一张带列表值与嵌套表的 `table.nw-props`（照服务端写的，没有 `tbody`，经 `innerHTML` 由解析器补上）：`table`、`rowgroup`、`row`、`rowheader`、`cell`、`list`、`listitem` 都给了，嵌套的表也是；属性表之外的表不碰；已有 `role` 的元素不碰；清理之后它加的 `role` 都去掉、原有的留着。
- `W/app/column-top.test.tsx`（1）：没有提供者时 `ColumnTop` 什么也不画；有时画给的元素。
- `W/test/lock-note.ts`（2，测试的帮手）：`lockNote(text)` 是 Testing Library 的匹配函数，`(_, element) => element?.tagName === "OUTPUT" && element.textContent === text`（整段文字，含 `bdi` 里的名字）。

**S1，改的文件**：

- `W/pages/page/page-layout.test.tsx`（1）：头一行的"Edited by Ada yesterday"（版本列表的最新一项，名字在 `bdi` 里，`title` 是全时间），中文"Ada 编辑于昨天"；它不是 `status`；编辑时不在；没有版本时没有这句话（`meta` 盒子在，`HeadMeta` 的 `<p>` 是空的，决定 858、894）；`GET` 版本只一次（与右栏共键）。"编辑"拿着焦点、事件把角色降为读者：焦点到 `h1`，`HTMLElement.prototype.focus` 的桩记下的调用带 `{ preventScroll: true }`（决定 865，审查 A 的 I2；变异 21、70 的抓手）。头一行在 `h1` 之前、面包屑在右边一组之前（DOM 的次序）。`:24-32` 的面包屑照旧。
- `W/pages/page/page-panel.test.tsx`（1）：
  - 开关：`aria-expanded`、`aria-controls` 等于右栏的 `id`；点了 `data-open` 变 `"false"`、`localStorage` 的 `nwiki.panel` 是 `"closed"`、焦点在开关上；存着 `"closed"` 时打开页是收起的；中文的名字"页面信息"；焦点在右栏的一个链接上时收起：焦点到开关，`focus` 带 `preventScroll`（决定 859）。
  - 读到的一节：给标题的盒子打桩，读到的一节的链接 `aria-current="location"`、别的没有；一页 1,001 个标题、第 1,001 个也在线上：当前项是第 1,000 个（列出的最后一个），大纲照旧 1,000 项加"还有 1 个"（审查 C 的 M10）；收起，或 `matchMedia` 答不在正文旁边：标题的 `getBoundingClientRect` 一次也没被调（决定 886）。
  - 展开与收起时的量（审查 A 的 R2-I1，变异 112）：存着 `"closed"` 时打开页（挂上时 `active` 为假），点开关展开：读到的那一节的链接带 `aria-current`，右栏的 `scrollTop` 跟着变（桩同下一条）；展开时挂上（量过），点开关收起之后再 `fireEvent.scroll(window)`、等一帧：标题的 `getBoundingClientRect` 不再被调。别的格都是从挂上起就收起或展开的，`sameView` 不比 `active` 时照样过。
  - 右栏跟着（决定 886；审查 C 的 I1 的写法，jsdom 的桩见上）：右栏高 300、`scrollHeight` 2000，项高 28；当前项变到第 30 项（在右栏的可见范围之下）：`scrollTop` 是 `30 × 28 − 300 + 28 = 568`（项的底与右栏的底齐，不是放到中间），`window.scrollTo` 与 `Element.prototype.scrollIntoView` 的桩都没被调；当前项变到第 25 项（还在可见范围里）：`scrollTop` 不变。读者在右栏里滚开：`fireEvent.wheel(aside)`、`aside.scrollTop = 0`；再让当前项变到第 31 项：`scrollTop` 仍是 0。（换页时 `PageShell` 以 `key={page.id}` 重挂，读者的记录随之重起，不另写格，审查 A 的 R1-N1。）照实加一格（决定 897，取自审查 C 的探针）：读者滚过右栏、不再跟之后进编辑，以 Mod+E 回到阅读，大纲重新挂上，当前项变到第 35 项时右栏又跟（`scrollTop` 708）；它描述现有的行为，不会先红，变异 t12（按页记"动过"）看到它红。
  - 视图晚到（审查 A 的 R1-I2；第 1 节 (l) 照这一格跑过）：`server.viewsHeld = true`，页的外框挂上、大纲没有；放行，大纲出现；打桩（右栏、项、标题的盒子，40 个标题、每个 100 px）；`user.click` 大纲的"H1"（`pointerdown` 加 `click`）：它带 `aria-current`（"H1"的盒子在 16，视口里：量到过）；把它的盒子移出视口（桩成 −200）、让照线算的一节是第 32 项，`fireEvent.scroll(window)`、等一帧：`pin` 结束（量到过、又出了视口），第 32 项带 `aria-current`，右栏的 `scrollTop` 是 624（照旧跟；在 `[page]` 的 effect 里建 `watchReader` 时是 0，变异 74）。不先把"H1"移出视口，`pin` 一直标着它，读到的一节不变（审查 C 第 2 轮的未核实一条）。
  - 点按（审查 C 的 R1-I1）：标题的盒子桩成照线算是第 5 项；先按住修饰键点按大纲的第 9 项的链接，Control、Meta、Shift、Alt 各一次（`user.keyboard("{Control>}")`、`user.click`、`user.keyboard("{/Control}")`，其余同），每次之后照线仍是第 5 项带 `aria-current`，不 `pin`（审查 A 的 R1-M1、R2-N1，变异 104、104b）；最后不带修饰键点第 9 项：它立刻带 `aria-current`（`onClick` 设 `pin`；去掉时照线还是第 5 项，变异 76）。次序不能反过来：普通的点按 `pin` 了之后，修饰键的 `keydown` 与 `pointerdown` 都打在 `on` 上、不算读者动过，`pin` 不结束，之后的修饰键点按无论 `pin` 不 `pin` 都答第 9 项（审查 C 的 R3-M1）。这一格在 `document` 上挂一个捕获阶段的 `click` 监听，带修饰键时 `preventDefault()`，格后去掉：React Router 对带修饰键的点按不 `preventDefault`（`Link` 不导航），jsdom 就执行 `<a>` 的默认动作，经虚拟控制台报"Not implemented: navigation to another Document"（照实：不挂这个监听时那一格打出四次，格照样绿：这句话不经被桩住的 `console.error`，`setup.ts` 看不见它，设计照审查 C 的 R2-M1 写的"格后判红"不成立，审查 C 的 P4S1C-N8；监听照旧挂着，让输出安静、不让 jsdom 去导航，变异 104、104b、76 的抓法不受影响）；`Link` 照样先调我们的 `onClick`，`plainPress` 不看 `defaultPrevented`，变异 104 照样抓得住。
  - `:16-22` 的 `outlined` 读 `li` 的 `textContent` 与内联的 `paddingLeft`：照旧（缩进仍在 `li` 上）。`:182-183`（右栏在正文与子页之后）照旧。
- `W/stores/preferences.store.test.ts`（1）：`panelOpen` 默认 `true`；存着 `"closed"` 时 `false`；`setPanelOpen(false)` 写 `"closed"`；存储抛错时照样改、不抛。
- `W/pages/notebook/upgrade-banner.test.tsx`（1）：新一格：升级中的笔记本的页上，横幅与 `h1` 同在一个父元素里、横幅在前，整页只有一个横幅；找不到的页（`PageGone`）上横幅在列的最上面；页树的 `GET` 答 500（`server.nodesDown`，`NotLoaded` 的 `Alert` 与"重试"一直留着，`app/not-loaded.tsx:17-27`）：横幅在 `Alert` 之上（审查 C 的 R1-M4，变异 80）；`:97` 的"来去、页照旧"照旧（`article` 是同一个元素）。版本页上横幅照旧在笔记本的外框里（`Outlet` 之前）。
- 决定 892 那一支（`settings-layout.tsx:24-25`，`shown === undefined`）只在笔记本设置的索引地址上、`IndexTo` 换到"基本信息"之前的一瞬画出。照实：决定 898 把它包进设置的列之后，`upgrade-banner.test.tsx` 加一格（"a notebook's settings index has the banner in the settings' column…"）：从索引地址进设置，`MutationObserver` 记下每次变动时横幅（`output`）在不在 `.max-w-178` 的列里、有几个、`main h1` 在不在，横幅从出现起一直在列里、只有一个（改前第一次见到它时是 `{column:false,title:false}`，先红；变异 106、t13）。
- `W/pages/notebook/general-page.test.tsx:665-688`："the banner above the page"一格照旧（横幅在设置页的列里，`getAllByRole("status")` 照样找到）。
- `W/pages/page/page-versions.test.tsx`（1）：右栏版本一行照 4.1.6：`li` 的子元素依次是放时间链接与谁写的（`p`，名字在 `bdi` 里）的 `div`、标记的 `div`（决定 863）；`Mark` 的 `current`；`:scope > ul > li` 与 `listed()`、`writers()` 的读法照旧；`:103` 的整项文字换成 `"5分钟前Ada · 撤销当前编辑中"`。`W/pages/page/version-restore.test.tsx:156` 的 `/CurrentAda · Restore$/` 换成 `/Ada · RestoreCurrent$/`（审查 C 的 R1-M7）。
- `W/pages/page/page-edit.test.tsx`（2）：编辑时状态的 `<output>` 在头一行右边一组的第一个子元素（`meta` 盒子）里，"保存"在开关之前、"完成"在开关之后（`compareDocumentPosition`）；盒子的类有 `text-[12.5px] text-faint`，失败时 `<output>` 带 `text-destructive`；阅读时同一个盒子里是"Edited by"（决定 894）；编辑说明（中英文）在、失锁时不在；"已保存。"时 `<output>` 带 `data-saved`；按头一行的"完成"离开、离开的保存撞上冲突（`PUT` 答 409）：冲突面板的标题拿着焦点（守的是"完成"进了头一行之后离开的焦点照旧：按"完成"时 `from` 就是它，`stayed()` 认不认槽都为真，抓不到变异 33，审查 A 的 R1-M3、C 的 R1-M2）。`:27` 等读唯一的 `status` 的格照旧（头一行的状态是它）。
- `W/pages/page/page-asset-upload.test.tsx:769`："a leave that fails once the uploads are in, Done pressed as it waited after Mod+E, gives the focus back to the editor"照旧，它是变异 33 唯一的抓手（`stayed()` 不认槽时离开失败不调 `back()`：焦点在离开开始之后、等上传时才挪进槽里，只有这一种走法认槽才有区别）。
- `W/pages/page/edit-lock-note.test.tsx`（2）：拿着锁的人的名字在 `bdi` 里；`status` 仍是一个。
- 读锁说明的九处（2）：`page-autosave.test.tsx:170`、`page-tasks.test.tsx:411`、`page-versions.test.tsx:383,391`、`version-page.test.tsx:417,428,471,481`、`version-restore.test.tsx:377` 的 `findByText("Bob is editing this page.")`、`queryByText(…)` 改成 `findByText(lockNote("Bob is editing this page."))`、`queryByText(lockNote(…))`。其中三处 `toBeNull()`（`page-versions.test.tsx:391`、`version-page.test.tsx:428,481`）照原样不改会空过（名字进 `bdi` 之后整句本来就找不到）：改了之后，实施者把锁说明的释放事件去掉一次，看这三格红。

**S1 的照实**（[S1 的审查记录](reviews/P4-S1-head-strip-panel-review.md)）：

- `page-layout.test.tsx`：决定 899 的中英两格（`test.each`）读类名：`p` 不带 `truncate`、带 `flex justify-end-safe overflow-hidden whitespace-pre`，`bdi` 是 `min-w-0 truncate`（jsdom 不排版；浏览器里的行为由 PG17 第 2 步核对；变异 t14、t15、t16、t19）。
- `page-panel.test.tsx`：开关的格读应用的内存存储（`PreferencesStore` 读写的就是 `nwiki.panel` 这个键），不读 `localStorage`；"重新载入之后照旧收起"由 PG17 第 12 步在真浏览器里守（审查 A 的偏离 9）。
- `reading-view.test.tsx` 的"the wait's listeners go once its read settles"按监听函数认出等待的那几个：大纲的 `watchReader` 在列表挂上时加 4 个捕获阶段的监听，按函数分开之后，断言等待的全拆、大纲的 4 个在、大纲可见（照设计数个数，这一格红；审查 A 的偏离 3）。
- `page-edit.test.tsx`：中文那一半的打开读中文那次的 `sent`（原读英文那次的，空断言，审查 C 的 P4S1C-N9）。
- `version-page.test.tsx`：版本页标题下"时间 · 作者"一行里"·"之后与种类之前是不断行空格（`versionBy`，审查 B 的 P4S1B-F2-1）：找作者链接的名字写 `"Ada · Edit"`，整行的 `textContent` 是 `"· Ada · Edit"`（`e2fd05b0`；变异 t24、t25）。长名字在那一行与锁说明里折行由 PG17 第 4 格守（5.1）。
- `upgrade-banner.test.tsx`：没改的树上，找不到的页、页树读取失败两格本来就绿（改前横幅画在 `Outlet` 之前，"在它之前、只有一个"成立；变异 25、80 看到它们红），"a version's page has the banner where it had it"守照旧的那一支，构造上就绿（变异 22 不会让它红）；`page-panel.test.tsx` 的"folded away, or not beside the page, the right column measures none"在旧树上什么也不量，本来就绿（变异 73 看到它红）；`general-page.test.tsx:665-688` 原有、没改（变异 24 看到它红）（审查 C 的 P4S1C-N3）。

**S2**：

- `W/reading/code-blocks.test.ts`（新，2）：`pre > code.language-go`：头是 `pre` 的第一个子元素，语言名"go"，"Copy"按钮；`language-mermaid` 不加；没有语言名的有按钮、没有名字；没有 `navigator.clipboard` 时只有语言名、没有按钮；两者都没有时没有头。按下（`writeText` 打桩）：写进去的是 `code` 的 `textContent`（不带"go"与"Copy"），`said` 是"Copied"、`said` 与按钮都带 `data-state="copied"`，1.5 秒之后（假计时）都清掉；按两次，第一次的 `writeText` 晚于第二次按下才兑现："已复制"从第二次按下起满 1.5 秒才清（按的次数只认最后一次，审查记录"未核实的怀疑"）；`writeText` 拒绝时 `said` 是"Couldn't copy"、带 `data-state="failed"`，按钮不带；中文的三个词；按钮的名字一直是"Copy"；`said` 不是 `status`（`queryByRole("status")` 为空）；清理之后头不在、计时清掉。
- `W/reading/patch.test.ts`（2）：`focusKeyOf` 给"复制"`copy` 与它在块里的第几个，`findByKey` 找回新块里的那一个。
- `W/pages/page/reading-view.test.tsx`（2，经 `readingEnhancements`）：一页有两个代码块，第二个的"复制"拿着焦点，就地更新改了它所在的块：焦点在新的那一块的"复制"上；改了另一块：焦点在第二块的（新建的）"复制"上，按位置找（审查 C 的 M9：清理与重跑之后头都是新建的，不是同一个元素）。
- `W/reading/diagrams.test.ts`（1）：`:68-71`、`:124-147`、`:326`、`:331`、`:338`、`:342`、`:364`、`:366`、`:369` 以 `config.theme` 区分两种画法的断言，改成 `theme` 都是 `"base"`、`themeVariables.background` 亮 `#ffffff`、暗 `#1f2023`（以它区分）；新断言（决定 888）：暗色 `darkMode` 是 `true`、亮色没有；两种主题的 `pie1`、`git0`、`cScale0` 是七族的蓝（亮 `#3d6fb8`、暗 `#8fb0ec`，同 `--nw-callout-blue`，4.2.6）；`xyChart.backgroundColor` 与 `background` 相同、`xyChart.plotColorPalette` 以七族的蓝起头（整个对象，不是只给几项：mermaid 整个替换它）；`themeCSS` 含 `outer-path` 与 `rx: 9px`；亮色 `activeTaskBkgColor` 是 `#e7e7ea`、暗色 `critBkgColor` 是 `#b33b3b`，两种主题的 `themeCSS` 都含 `.eventWrapper { filter: none; }`，暗色的含 `.cluster-label`（审查 B 的 R1-M2、M3、N4）。缓存按主题分的格照旧。
- `W/design-tokens.test.ts`（1）：新一格，`reading.css` 的 `:root` 与 `.dark` 声明的变量名两边相同、各不少于 20 个（4.2.2；决定 870，审查 C 的 N4；`read()` 已读它，用 `declared(css, prelude)`）：今天的 `reading.css` 里这两块在 `@layer components` 里，`declared()` 只读最外层的规则，读不到 20 个，它红（修订稿说"没改的树上没有 `:root`/`.dark`"不对：有，各一个变量，在层里，审查 B 的 R1-I1、C 的 R1-M5）；挪到最外层、只写了 `:root` 时也红；两块写全之后绿，实施时看一次。

**S2 的照实**（[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)）：

- `code-blocks.test.ts` 比上面多四格：第 8 格"pressed again while its word shows, the word goes at once, and the last press's shows 1.5 s from its answer"（再按时上一句立即清掉、它的计时也清掉，守 t1、t2；没改的树上导入失败，放空的 `codeBlocks` 桩时它必红，`said` 为 `null`，推得，没有那一次的日志，审查 C 的 P4S2C-N2）；"a pre of more than one code, as an author's HTML may write it, has no head"（审查 A 的 P4S2A-N2）；"a copy answered once its view is cleaned up says nothing and sets no timer"（A 的 P4S2A-N1，`vi.getTimerCount()` 为 0）；"a pre of a code and text about it, as an author's HTML may write it, has no head; of a code and blank text, its head"（A 的 P4S2A-F1-N2）。都先红。
- `diagrams.test.ts` 比上面多四格、改一格：决定 904 的"a diagram naming a theme of its own, in a directive or its frontmatter, is drawn in that theme alone, not in the app's colours"（指令的 `dark`、frontmatter 的 `forest`，没有 `themeVariables` 与 `themeCSS`，`securityLevel`、`secure`、`maxEdges` 照旧）；"a diagram naming base with variables of its own is drawn from its config alone; base alone, or nothing, in the app's colours; a mindmap too large is not read, its frontmatter's theme neither"（太大的思维导图带着 frontmatter，`tooLarge` 之前解析就会看到）；决定 906 的"a diagram giving variables of its own without a theme is drawn from its config alone, in mermaid's default theme in the light app and its dark one in the dark"；A 的 F1-N1 的"a diagram of neither a directive nor frontmatter is not read before it is drawn; one of frontmatter after blank lines is"（`parse` 没被调）；第一格的选项多了 `flowchart: { padding: 9 }`。替身的 `parse` 读指令的整段 JSON 与 frontmatter 的 `theme:`，`detectType` 读过 frontmatter 与指令认类型；别的四个替身答 `{ config: {} }`。都先红。
- `reading-view.test.tsx` 的 `afterEach` 拆掉剪贴板替身（4.2 之后的照实；临时的探针格先红、改后绿，探针没有提交，变异 t26 没有格抓）。
- L4 (page) 第一格的强制色一步（5.2；先红在 `l4:205`）。
- PG17 第 2 格的第 5–8 步（5.2）：第 7、8 步与已完成项的块、属性条一步都先红（三种浏览器）。

**S3**：

- `W/editor/prose-lines.test.ts`（新，1）：
  - `proseLines`：围栏代码块、缩进代码块、表格、闭合的与没闭合的 frontmatter 的行带 `nw-mono-line`，frontmatter 的另带 `nw-frontmatter-line`；正文的行不带；引用与列表里的代码块也带；文档改了（加一行围栏）之后重建。一篇 8,000 行、整篇是一个代码块的文档：`view.plugin(proseLines)!.decorations.size` 不超过 `view.visibleRanges` 覆盖的行数（数的是装饰，不是 DOM：jsdom 里 CodeMirror 只画几十行，给每行都加的变异下 DOM 里带类的行照样只有几十，审查 C 的 I2）。
  - `markupMarks`（决定 889）：`[[Deploy guide#Steps|guide]]`：`[[`、`#`、`|`、`]]` 各在 `nw-md-faint` 里，"Deploy guide"在 `nw-md-link` 里，"Steps"与"guide"两者都不在；`![[a.png]]` 的 `![[` 淡；`[[#Steps]]`："Steps"带 `nw-md-link`；`[text](https://x)`："text"带 `nw-md-link`，括号与地址不带（它们由高亮压淡）；`==mark==`：两端的 `==` 淡、"mark"不淡；行内代码、围栏代码块与 frontmatter（`up: "[[Hub]]"`、`tags: [a, b]`）里的都不加；同一篇 8,000 行、每行一个 wiki 链接的文档：`view.plugin(markupMarks)!.decorations.size` 不超过可见行数的 4 倍（每个链接至多四段）。
- `W/editor/edit-outline.test.ts`（新，2）：
  - ATX 与 setext 的标题、引用里的、列表里的都列出，代码块与表里的不列；frontmatter 里的（l6 那种 `---`/`up:`/`---`）不列；文字去掉 `#` 与下划线行；空的不列；`upTo` 之后只数。
  - 构造时就报一次，不等计时（审查 A 的 M3：读的那一刻不前进假计时）；之后连续改 5 次（假计时，每次 100 ms）只再算一次，在最后一次之后 300 ms。
  - 分片（审查 C 的 M8：`Parse.work` 以 `Date.now()` 计时，假计时下一片永远不超时，真计时看机器）：`vi.mock("@codemirror/language", async (actual) => ({ ...(await actual()), ensureSyntaxTree: vi.fn() }))`，`beforeEach` 里把桩设回真的实现（`vi.mock` 对整个文件生效，同文件别的格默认拿到答 `undefined` 的桩）；分片的几格桩前 3 次返回 `null`、之后调真的。文档远长于初始的解析（`InitViewport` 是 3,000 个字符，`dist/index.js:541`；文档短时 `syntaxTreeAvailable` 一开始就为真，桩不被调，审查 A 的 R1-N2、C 的 R1-M6）：约 20 万个字符的正文、末尾一个标题，断言它列出、`ensureSyntaxTree` 至少被调了 4 次。两片之间发一个只改选区的事务：只推进 0 ms 的计时（`vi.advanceTimersByTime(0)` 几次）、不到 300 ms 就报出（审查 A 的 I4；放弃之后重排 300 ms 的变异 97 这时还没报）；另一格两片之间发一个只带 effect 的事务：同样；两片之间改了文档：这一轮不报，300 ms 之后重排的一轮报出改过的。
  - 位置（审查 A 的 M2）：报出之后在第一个标题之前打 5 个字、不等 300 ms：`where(view, 1)` 是第二个标题那一行映射过的开头；序号超出时答 `undefined`。
  - `destroy` 之后，前进假计时，`told` 不再被调；另一格在两片之间（桩正返回 `null`、那一片的 `setTimeout(0)` 已排）`destroy`，再前进计时，`told` 不被调（只清 300 ms 计时的变异 100 这时报出，审查 A 的 N5、C 的 R1-N4）。
- `W/editor/source-editor.test.tsx`（1、2）：`SourceEditor` 里 `getComputedStyle(".cm-content")` 的 `lineHeight` 是 `1.85`、`.cm-line` 的 `paddingLeft` 是 `26px`（`proseTheme` 赢过 `editorTheme`，决定 874；jsdom 照挂载的次序算样式，初稿的次序下答 `1.6` 与 `16px`，第 1 节 (f)）；围栏代码块那一行带 `nw-mono-line`、`[[Hub]]` 的 `[[` 在 `.nw-md-faint` 里（`proseLines`、`markupMarks` 在 `SourceEditor` 的扩展里，审查 C 的 M6；`prose-lines.test.ts` 自己拼扩展，抓不到它们被去掉）（1）；`.cm-content` 的 `caretColor` 是字面的 `"var(--accent)"`（jsdom 不解析 `var()`，答的就是写的那一串，审查 C 的 R2-N1；光标是浏览器自己的，`.cm-cursor` 不存在，审查 B 的 R1-M1）（1）；`reveal(1)` 之后光标在第二个标题那一行的开头、编辑器拿着焦点、发出了 `scrollIntoView` 的效果；第二个标题在引用里（`> ## Quoted`）时光标在那一行的 `>` 之前，不在 `#` 之前（`lineAt(pos).from`，审查 A 的 R1-N3）（2）。
- `W/editor/text-diff.test.tsx`（1）：对比视图的 `.cm-content` 的 `lineHeight` 照旧是 `1.6`，`.cm-scroller` 的字体照旧是等宽（`proseTheme` 只在 `SourceEditor` 里）。
- `W/editor/loaded/yaml-quotes.test.ts`（1）：`frontmatterRange` 的闭合、没闭合（到空行）、没有、CRLF、只有开头一行几种，与 `frontmatterAt` 的判断一致；同一个 `state.doc` 第二次不再扫（数 `doc.line` 的调用）。
- `W/pages/page/page-panel.test.tsx:186-199`（2）：改写成"编辑时的大纲来自编辑器：标题的按钮，点了光标到那一行；回到阅读，照旧是链接"；进编辑时大纲在第一次渲染之后就在（不前进计时，审查 A 的 M3）；焦点在第三项的按钮上，一次异步的文档改动在第一个标题之前插进一行（拿着焦点的那个标题的位置随之变；插在它之后时位置不变，`key={from}` 照样是同一个元素，审查 C 的 R1-N4），报出之后：拿着焦点的还是同一个按钮元素（`key` 是序号，审查 A 的 M2）。
- `W/pages/page/page-edit.test.tsx`（2）：
  - "保存"一直在 DOM 里（决定 894）：没有修改时它带 `invisible`（jsdom 不读 Tailwind 的 CSS，`getByRole` 照样找得到它，断言读类）；打字之后不带；保存失败时不带，再按是重试；
  - 自动保存完成时焦点在"保存"上：它不带 `invisible`、焦点还在它上面，再按它不发 `PUT`；焦点移到编辑器：它带 `invisible`（决定 885）；
  - 打了字之后焦点到"完成"，自动保存完成："保存"不带 `invisible`；`user.tab({ shift: true })` 两次（经开关：jsdom 不读 CSS，`hidden xl:inline-flex` 的开关也在 Tab 的次序里），焦点到"保存"（决定 895，审查 A 的 R1-M2）；
  - 没有修改时（刚进编辑；另一遍在自动保存完成、焦点回到编辑器之后）焦点到"完成"、再到开关（`focus()`）："保存"一直带 `invisible`（决定 895 改，审查 A 的 R2-M1，变异 116）；
  - 没有修改、焦点在"完成"上时以 `view.dispatch` 改正文（等上传时插进链接的样子，焦点不动）："保存"不带 `invisible`；自动保存完成之后仍不带；焦点移到编辑器之后带（变异 117）；
  - 打了字、焦点到"保存"、自动保存完成之后（这时没有未保存的修改：有的话"保存"本来就显出，变异 68 照样绿，审查 C 的 R2-N2），发一个 `relatedTarget` 为 `null` 的 `focusout` 而 `document.activeElement` 仍是它（窗口失焦的样子）：它不带 `invisible`；
  - `:286`（打字之后点"保存"）照旧（"保存"一直挂着，不用改成 `findByRole`）。
- `W/pages/page/page-asset-upload.test.tsx:365,387,434,449,660,691`（2）：打字或插入之后点"保存"的六处照旧："保存"一直挂着（决定 894），修订稿要把它们改成 `findByRole`，是因为那时它要等渲染才挂上。
- `W/pages/page/conflict.test.tsx:79`、`page-autosave.test.tsx:327-333`（2）：有未保存的修改或失败时点的，"保存"在；`:327-333` 是同一个按钮经过重试，照旧。
- `W/pages/page/page-versions.test.tsx:403-409`：编辑时右栏的三节照旧在（S3 另有大纲在它们之前）。
- `W/pages/page/page-lock.test.tsx`（2）：新一格：失锁（"unlocked" 与 "taken"）的横幅里，名字在 `bdi` 里，`role=alert` 与焦点照旧。
- `W/editor/loaded/link-completion.test.ts:636-645`（计时）、`W/editor/markdown.test.ts` 的"不更慢"：只换样子，照旧；负载高时"not slower"偶发（第 1 节），重跑。
- `W/design-tokens.test.ts:281-283`（1）：白名单只剩 `./reading/diagrams.ts`，注释改成"mermaid's themeVariables take hexadecimal colours alone"；`noColour` 的 `"text-start"`（2，4.5）。

不另写的：`W/test/page-panel.ts:27-30` 的小节读法（`details` 里的 `h2`）照旧；`page-menu.test.tsx:160-169`（编辑时没有菜单）照旧；`page-properties.test.tsx:14-18`（一个名为"Properties"的 `dl`）照旧：属性条的区域名只在宽时有，jsdom 里不宽；`page-properties.test.tsx` 断言解析不到的链接 `decoration-dashed` 的三格照旧（虚线是"解析不到"的标记，不是链接的下划线，决定 863，审查 C 的 M15）。

**S3 的照实**（[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）：

- 上面各格都在（`ab6f4353` 13 格、`68a7e4be` 19 格、`9a52ddb4` 1 格）。写法与上面不同的：分片的几格把假计时推 1 ms × 5（运行中排下的 0 ms 计时 1 ms 之后才到；变异 97 照样抓得住）；page-edit"一直在 DOM 里"一格在自己的答复函数里数 `PUT`（自定义答复的请求不进 `server.sent`）；`source-editor.test.tsx` 另有 frontmatter 一格（`9a52ddb4`：frontmatter 里 lezer 读成 setext 标题的那一行 13.5 px、400，正文的 setext 标题 16.5075 px、630 作对照，变异 t8 之下红）。不先红的：page-edit"打了字之后焦点在'完成'上"与"`focusout` 的 `relatedTarget` 为 `null`"两格（改前"保存"一直显着，分别守 109、68），`text-diff.test.tsx` 的行高一格（守 57）。
- 审查之后多 14 格：`15372fcf` 的 `edit-outline.test.ts` 3 格（决定 907 的项文字逐种、决定 908 的没闭合的 frontmatter、所有标题之前打字不重报），`page-panel.test.tsx` 3 格（扣住正文的读取时右栏照旧是阅读时的大纲；离开编辑、再进时不画上一回的标题；1,003 个标题的"…and 3 more"），新文件 `wiki-links.test.ts` 3 格（显示的文字与说明；转义、空的与内有方括号的；位置、去空白与表格里的 `\|`）；`4ecc3e10` 的 `prose-lines.test.ts` 1 格（`markupMarks` 照服务端认 wiki 链接），`source-editor.test.tsx` 2 格（行内代码 13.5 px；引用里的标记淡），`text-diff.test.tsx` 1 格（gutter），`link-completion.test.ts` 1 格（强制色下选中项的外框：先断言计算出的 `outline` 是空的，再从样式表里读 `(forced-colors: active)` 里选中它的规则）。改写的：`source-editor.test.tsx` 的主题一格多一句 `.cm-scroller` 是 `--font-sans`，`text-diff.test.tsx` 的等宽一格改成 `--nw-editor-mono`、13 px（`d54a9d00`，决定 909）；L6 的页面格、PG17 第 3 格与 L5 的页面格的步（5.3 之后的照实）。都先红，除了"…and 3 more"一格（守已有的行为，t12 下红）与 `--font-sans` 一句（守已有的先后，t49 下红）。
- 修复核对第 1 轮之后多 5 格：`d7fb332c` 的 `edit-outline.test.ts` 2 格（注释不进项、页的嵌入显示目标；`upTo` 之后不读文字：`vi.mock` 包住 `wikiLinks` 数它被调的次数，2 次，改前 6 次），`page-panel.test.tsx` 1 格（等正文时保留的阅读大纲 `inert`，读到、编辑器报出之后右栏是编辑时的大纲）；`e6558722` 的 `source-editor.test.tsx` 2 格（h1、h2 里的行内代码的字号带 `--nw-heading` 1.29、1.065；查找框宽 10.5rem（jsdom 读作 `168px`），强制色下聚焦的查找框与按钮有透明的 2 px 外框，照 `link-completion.test.ts` 那一格从样式表里读）。改写的：`wiki-links.test.ts` 的显示一格加页的嵌入（`![[Page|alias]]` 显示 Page，`![[Doc.PDF|Spec]]` 显示 Spec），行内代码一格的期望随之是 `calc(…)`。都先红；查找面板一格的强制色一半守已有的规则（t63 下红，即审查 C 的 c-fc-search）。
- 修复核对第 2 轮之后多 1 格（`bcd1575b`）：`wiki-links.test.ts` 一格（`![[v1.2 notes|Alias]]` 显示 v1.2 notes：扩展名不含空白）。改写的：`page-panel.test.tsx` 的 `inert` 一格改成断言大纲那一节的 `details` 带 `inert`（开关在内）。两格都先红。实体的表每次读新建与模块级的表行为相同，没有格分得出（t67），由堆的量看（10.3）。

### 7.2 变异

每条在本机改坏、跑它旁边的测试、看到失败，再从备份复原（02-workflow 3.1；先在没改的树上跑一遍同样的命令）。编号从 1 起，P4 自己的；1–69 是初稿的（修订时照审查改了其中几条的改法或抓手，写明依据），70–101 是修订加的，102–111 是再修订加的（修订核对第 1 轮；同时又改了 13、33、35、66、68、74、76、79、80、97、99、100 的改法或抓手），112 起是三修加的（修订核对第 2 轮；同时又改了 68、104、109 的抓手）；实施者另写的以 `t` 起头、各 Step 各自编号，e2e 的一份在编号后加 `e`（P3 的写法）；字母后缀（104b）是同一条的另一种改法，单占一行（P3 的 69b）。"守卫"一列标出它守的是 4.8 的最低清单、焦点的规则、本文的决定还是一处上限。

| # | Step | 改坏 | 抓住它的测试 | 守卫 |
|---|---|---|---|---|
| 1 | S1 | `HeadMeta` 画成 `<output>` | `page-layout.test.tsx`"不是 status"；PG17 第 2 步（1e） | `editStatus` 的唯一 |
| 2 | S1 | `HeadMeta` 取版本列表的第二项 | `page-layout.test.tsx` 的"Edited by Ada" | |
| 3 | S1 | 名字不进 `bdi`（`same` 代替 `isolate`） | `page-layout.test.tsx`；PG17 第 2 步 | `bdi` |
| 4 | S1 | `HeadMeta` 另起一个键读版本（多一次 `GET`） | `page-layout.test.tsx` 的读取计数 | 不加请求 |
| 5 | S1 | 开关不写偏好（只改组件的状态） | `page-panel.test.tsx` 的 `nwiki.panel`；PG17 第 12 步（刷新之后展开，5e） | 右栏收起与偏好 |
| 6 | S1 | 偏好没存过时收起（默认反了） | `preferences.store.test.ts`；PG17 第 3 步 | 同上 |
| 7 | S1 | 开关的 `aria-expanded` 一直是 `true` | `page-panel.test.tsx`；PG17 第 11 步 | 名字与状态 |
| 8 | S1 | 收起时内容区照旧留 `xl:pe-10` | PG17 第 11 步（正文的中心比 `main` 的内容区的中心偏左 20 px，8e） | |
| 9 | S1 | 右栏去掉 `xl:data-[open=false]:hidden` | PG17 第 11 步（9e） | |
| 10 | S1 | 开关去掉 `hidden`（窄时也显示） | PG17 第 13 步（10e） | |
| 11 | S1 | 右栏去掉 `xl:h-svh`（只高到内容） | PG17 第 3 步（11e） | |
| 12 | S1 | 读到的一节的线是视口的顶（0） | `current-heading.test.ts`；PG17 第 5 步（"Part 10"的标题在窗口顶下 100 px，线在顶时算成"Part 09"，12e；照实：设计写的第 4 步抓不到，点到的标题被 `pin` 着，线在 0 时也读到它，`d29b68b0` 把第 5 步改到 100 px） | 大纲的当前节 |
| 13 | S1 | 不跳过关着的 `details` 里的标题（去掉 `closest`；初稿的"盒子是 0"在浏览器里不成立，审查 A 的 I1） | `current-heading.test.ts` 真的关着的 `details` 一格；PG17 第 6 步（照"Hidden"自己的盒子定位，它带 `aria-current`，13e；修订稿以 callout 的顶定位，这一步两种写法都过，审查 A 的 R1-M4、C 的 R1-M1） | 决定 887 |
| 14 | S1 | 到底时线不放到视口的底 | `current-heading.test.ts` 到底一格 | 大纲的当前节 |
| 15 | S1 | `aria-current` 写成 `"true"` | `page-panel.test.tsx`；PG17 第 4 步 | 同上 |
| 16 | S1 | 读到的一节变了时调 `scrollIntoView` | `page-panel.test.tsx` 右栏跟着一格的 `scrollIntoView` 桩（2 格；照 `scrollIntoView({ block: "nearest" })` 写时只有它抓：右栏整个在视口里，窗口不动，PG17 照过）；照默认的 `scrollIntoView()` 写，PG17 第 8 步红在"项的底离右栏的底不超过 2 px"（16e；窗口也不动：两种写法都不是"窗口跟着动"，照实，审查 C 的 P4S1C-N2） | 13.2 第 26 条 |
| 17 | S1 | 读者在右栏里滚过之后照旧跟（去掉 `acted()` 的判断；初稿的"去掉依赖"是等价的：`PageOutline` 只在当前项变了时重渲染，审查 C 的 I1） | `page-panel.test.tsx` 右栏跟着的读者一格（`scrollTop` 不再是 0）；PG17 第 9 步（17e） | 决定 886 |
| 18 | S1 | 去掉 `typeof ResizeObserver` 的守卫 | `page-panel.test.tsx` 15 格（jsdom 里抛错）；`current-heading.test.ts` 的"without a ResizeObserver"一格与连带的"unmounted"一格（照实：那一格先放 `main > article.nw-reading` 才走得到守卫，审查 C 的 P4S1C-M1，`ed95b2cf`；之前这个文件 14 格全绿） | |
| 19 | S1 | 一次 `scroll` 量一次（不经 `requestAnimationFrame`） | `current-heading.test.ts` 的调用计数 | 每帧至多一次 |
| 20 | S1 | 大纲的链接写回 `block`，写在 `line-clamp-2` 之后（照实：写回原来的位置、类名最前是等价的，`cn` 的 tailwind-merge 把两者当作 `display` 的一组、合并掉前面的 `block`，审查 B） | 无格抓（`line-clamp-2` 失效，长标题由两行 35.75 px 变成四行 71.5 px，只在截图里看得出，7.4） | |
| 21 | S1 | `EditButton` 不经 `useFocusLeaving` | `page-layout.test.tsx` 的角色降级一格 | 焦点的去向 |
| 22 | S1 | 横幅照旧画在 `Outlet` 之前（`NotebookLayout` 不看 `handle`） | `upgrade-banner.test.tsx` 新的一格（不在列里） | |
| 23 | S1 | `columned` 时 `NotebookLayout` 另外也画（两个横幅） | `upgrade-banner.test.tsx` 新的一格（一个）；`general-page.test.tsx:665`（`find` 仍找得到，不抓）；up1:237（`upgradeBanner` 的严格模式，23e） | |
| 24 | S1 | `SettingsPage` 不画 `ColumnTop` | `general-page.test.tsx:665-688`（找不到横幅） | |
| 25 | S1 | `PageGone` 不画 `ColumnTop` | `upgrade-banner.test.tsx` 找不到的页一格 | |
| 26 | S1 | `threeBrowsers` 不含 `page\/pg17-` | `node tools/e2e-projects/check.mjs` | 必需格 |
| 27 | S1 | `propertyRoles` 不给 `tbody` | `property-roles.test.ts` | 读屏的语义 |
| 28 | S1 | `propertyRoles` 的清理不去掉 `role` | `property-roles.test.ts` 清理一格 | |
| 29 | S1 | `propertyRoles` 盖掉已有的 `role` | `property-roles.test.ts` | |
| 30 | S1 | 属性条的 `tbody` 不 `flex-wrap` | PG17 第 15 步：`tbody` 的高不大于第一个 `tr` 的高、首尾 `tr` 的 `top` 相同（小块挤在一行，不宽、不出区域，审查 C 的 M3；30e） | 375 px |
| 31 | S1 | 属性的值不 `overflow-wrap: anywhere` | 等价（照实；Chromium 是实施者跑的，Firefox、WebKit 是审查 C 的 31-ffwk）：`.nw-reading` 的 `overflow-wrap: break-word` 继承下来，`td` 的 `min-width: 0` 与 `tr` 的 `max-width: 100%` 已够折行；改成 `normal` 时 PG17 第 15 步红（隔两帧之后区域一个，t10） | 375 px |
| 32 | S1 | 编辑时的状态与按钮照旧画在原处（不经门户） | `page-edit.test.tsx` 的次序一格 | |
| 33 | S1 | `stayed()` 不认两个槽 | `page-asset-upload.test.tsx:769`（离开失败不调 `back()`，焦点留在"完成"）。修订稿另记的 `page-edit.test.tsx` 冲突一格抓不到它：按"完成"时 `from` 就是它，`stayed()` 本来就为真（审查 A 的 R1-M3、C 的 R1-M2）；初稿记的"等上传一格（焦点落 `body`）"认不认槽都一样（审查 A 的 M5） | 焦点的去向 |
| 34 | S1 | 失锁时照旧画编辑说明 | `page-edit.test.tsx` | |
| 35 | S1 | 状态的 `<output>` 所在的盒子去掉 `leading-4`（两行的说明把头一行撑高；修订稿这一条是"去掉 `empty:absolute`"，决定 894 之后状态在定宽的盒子里，空着不影响别的，那一条不再有） | 无格抓（7.4 看一张两行的失败说明） | 决定 894 |
| 36 | S1 | 锁说明的名字不进 `bdi` | `edit-lock-note.test.tsx` | `bdi` |
| 37 | S2 | `codeBlocks` 不看剪贴板（总有"复制"） | `code-blocks.test.ts` 没有剪贴板一格 | 复制按钮（有无剪贴板） |
| 38 | S2 | 复制 `pre` 的 `textContent`（带上头） | `code-blocks.test.ts`；PG17 第 2 格第 3 步（38e） | |
| 39 | S2 | 头去掉 `user-select: none` | pg5 的复制一段（39e，Chromium） | |
| 40 | S2 | 头去掉 `position: sticky` | PG17 第 2 格第 4 步（40e） | |
| 41 | S2 | "已复制"不清掉（不设计时） | `code-blocks.test.ts`；PG17 第 2 格第 3 步 | |
| 42 | S2 | 失败也说"已复制" | `code-blocks.test.ts` | |
| 43 | S2 | `said` 写成 `<output>` | `code-blocks.test.ts` 的 `status` 一格 | `getByRole("status")` |
| 44 | S2 | 清理不移除头 | `code-blocks.test.ts`；`reading-view.test.tsx` 就地更新一格（两个头） | 增强的清理 |
| 45 | S2 | `keyed` 不含"复制" | `patch.test.ts`；`reading-view.test.tsx` 焦点两格 | 焦点的去向 |
| 46 | S2 | mermaid 的 `pre` 也加头 | `code-blocks.test.ts` 第一格（`ade3abd1` 上选择器是 `pre > code:not(.language-mermaid)`，改坏成 `pre > code`）；L4 整个文件 6 格照旧绿（`l4:361` 只读 `code.language-mermaid` 的字，会画的图由 diagrams 换成包装、头随 `pre` 一起没了：设计说的不抓，审查 C 重跑同） | |
| 47 | S2 | 语言名不去掉 `language-` | `code-blocks.test.ts`；PG17 第 2 格第 2 步 | |
| 48 | S2 | mermaid 照旧 `default`/`dark` | `diagrams.test.ts` | |
| 49 | S2 | 两种主题取同一份 `themeVariables` | `diagrams.test.ts`（初稿的 49e 去掉：l4:185-194 只看 `svg` 的 `id` 变了，这时照样变，审查 C 的 M4） | 换主题重画 |
| 50 | S2 | 已完成的项里嵌套的列表不 `inline-block` | PG17 第 2 格第 5 步（50e） | |
| 51 | S2 | hljs 关键字色取 `--foreground` | pg5:158-163（51e） | |
| 52 | S2 | `codeBlocks` 排在 `scrollRegions` 之后 | 等价（`reading-view` 与 `scroll-regions` 75 格、PG17 第 2 格三种浏览器都绿，实施者与审查 C 各跑过）：`scrollRegions` 的 `MutationObserver`（`childList`、`subtree`）在头 `prepend` 进来时重看那个 `pre`，`ResizeObserver` 也看到 `pre` 高了 35 px；头 `nowrap`、比列窄得多，自己不让 `pre` 变宽；`nameOf(pre)` 不看头，恒为"代码"；倒序清理时 `codeBlocks` 只拿掉头与计时。有东西在 `codeBlocks` 之后、两个观察者回调之前读区域的状态，或头会让 `pre` 变宽时，就不再等价（审查 C 的 P4S2C-N3） | |
| 53 | S2 | PG17 第 2 格的标题改一个字，与 `REQUIRED` 的起头对不上 | `node tools/e2e-projects/check.mjs` | 必需格 |
| 54 | S3 | `proseLines` 不加（`SourceEditor` 的扩展里去掉） | `source-editor.test.tsx` 围栏一行（`prose-lines.test.ts` 自己拼扩展，不抓；审查 C 的 M6） | |
| 55 | S3 | frontmatter 的行不等宽 | `prose-lines.test.ts` | |
| 56 | S3 | `proseLines` 遍历整篇（不看 `visibleRanges`） | `prose-lines.test.ts` 8,000 行一格（数 `decorations.size`；初稿数 DOM，jsdom 只画几十行，抓不到，审查 C 的 I2） | 上限（M12/P4/S5） |
| 57 | S3 | `proseTheme` 进了 `editorTheme`（对比视图也用比例字体与 1.85 的行高） | `text-diff.test.tsx` 的行高与字体（初稿记的 `.cm-scroller` 的类在两种编辑器上都不成立，审查 C 的 M6） | 对比视图等宽 |
| 58 | S3 | `theme.ts` 写回一个 oklch | `design-tokens.test.ts` | 写死的色值 |
| 59 | S3 | 把 `--accent` 的暗色调淡（`#d9a84e` → `#b88a3c`） | l5 的 `frame`（对父元素的底 `#1a1b1e` 约 5.54，低于下限；59e，Chromium） | 4.8：`--accent` 调淡，l5 红 |
| 60 | S3 | 编辑时的大纲不去掉 frontmatter | `edit-outline.test.ts`；l6:245（多一项 `up: … see: …`，60e；行号是 `45a477bd` 起的） | 4.8：不含 frontmatter |
| 61 | S3 | 大纲不设上限 | `edit-outline.test.ts` 的 `upTo` | 13.2 第 25 条 |
| 62 | S3 | 大纲不补完解析（只用 `syntaxTree`） | `edit-outline.test.ts` 分片一格（`ensureSyntaxTree` 的桩） | 2.1 第 3 条 |
| 63 | S3 | 大纲每次改都算（不去抖） | `edit-outline.test.ts` 的次数 | 13.2 第 25 条 |
| 64 | S3 | `reveal` 不给编辑器焦点 | `source-editor.test.tsx`；l6:247 的点按（64e，`toBeFocused`） | 焦点的去向 |
| 65 | S3 | 编辑时右栏照旧不画大纲 | `page-panel.test.tsx:186-199` 的改写；l6:245（65e） | |
| 66 | S3 | "保存"一直显出（不需要时也不加 `invisible`） | `page-edit.test.tsx` 没有修改一格（读类） | |
| 67 | S3 | "保存"拿着焦点时自动保存完成就隐去（条件去掉 `held`，初稿的写法；初稿的"失败时不显出'保存'"是等价的：显出的失败一定伴随未保存的修改，审查 C 的 M5） | `page-edit.test.tsx` 自动保存一格 | 决定 885 |
| 68 | S3 | 组的 `focusout` 不看 `document.activeElement`（窗口失焦也隐去；初稿的"不经 `useFocusLeaving`"随 885 不再有） | `page-edit.test.tsx` 的 `focusout` 一格（在自动保存完成之后发：有未保存的修改时"保存"本来就显出，审查 C 的 R2-N2） | 决定 885、895 |
| 69 | S3 | 失锁横幅的名字不进 `bdi`（`lostText` 照旧出字符串） | `page-lock.test.tsx` 新的一格（"unlocked"、"taken" 两种的名字在 `bdi` 里） | `bdi` |
| 70 | S1 | "编辑"消失时 `h1.focus()` 不带 `preventScroll` | `page-layout.test.tsx` 的 `focus` 桩；UP1（`scrollY` 回到 0，读的那一段移出视口，70e） | 窗口不动（审查 A 的 I2） |
| 71 | S1 | 右栏藏起时不把右栏里的焦点交给开关 | `page-panel.test.tsx` 收起一格 | 焦点的去向 |
| 72 | S1 | 跟着时把当前项放到右栏的中间（初稿的写法） | `page-panel.test.tsx` 右栏跟着一格（568）；PG17 第 8 步（底差不超过 2 px，72e） | 决定 886 |
| 73 | S1 | 收起或不在正文旁边时照旧量（`active` 恒真） | `current-heading.test.ts` 的 `active` 一格；`page-panel.test.tsx` 收起一格 | 决定 886 |
| 74 | S1 | 右栏的 `watchReader` 照修订稿在 `[page]` 的 effect 里建（视图晚到时 `on` 是 `null`，点大纲的项被算成读者动过；修订稿记的"只建一次、不随页"是等价的：`PageShell` 以 `key={page.id}` 挂载，换页时整个重挂，审查 A 的 R1-N1、C 的 R1-M3） | `page-panel.test.tsx` 视图晚到一格（`scrollTop` 停在 0，审查 A 的 R1-I2；第 1 节 (l)） | 决定 886 |
| 75 | S1 | `atEnd` 不看页能不能滚 | `current-heading.test.ts` 不能滚一格 | 决定 893 |
| 76 | S1 | 点大纲的项不 `pin`（`onClick` 去掉） | `page-panel.test.tsx` 点按一格（照线还是第 5 项）；PG17 第 10 步（"Part 40"带 `aria-current`，76e；修订稿的造数下窗口不到底，这一步两种写法都过，审查 C 的 R1-I1） | 决定 893 |
| 77 | S1 | `pin` 不随读者的动作解除（不看 `acted()`） | `current-heading.test.ts` pin 一格的 `wheel` | 决定 893 |
| 78 | S1 | 量 `article` 里的全部标题，不只列出的 | `page-panel.test.tsx` 1,001 个标题一格（审查 C 的 M10） | 13.2 第 25 条 |
| 79 | S1 | 右边一组不定宽（去掉 `flex-[0_1_26rem]`；修订稿只给"编辑过"预留 `w-64`，变异是去掉它） | PG17 第 1 格第 14 步（"编辑过"到来时头一行由一行变两行，"编辑"下移，79e）；PG17 第 3 格 820 宽（阅读两行、编辑一行；375 宽那一遍也红，两遍都红在第 3 步的 `:288`：先跑的是 375 宽，820 宽那一遍由审查 C 单独跑，审查 C 的 P4S1C-N1）；第 1 节 (j) 量过这两处都动 | 决定 858、894 |
| 80 | S1 | `NotLoaded` 不画 `ColumnTop` | `upgrade-banner.test.tsx` 页树读取失败一格（审查 C 的 R1-M4：读取失败时 `Alert` 与"重试"一直留着，不是"一瞬"） | |
| 81 | S1、S3 | `noColour` 去掉 `"text-end"`（S1）或 `"text-start"`（S3） | `design-tokens.test.ts`（守卫把它们当颜色读，审查 B 的 I4） | 守卫的白名单 |
| 82 | S2 | `reading.css` 的 `.dark` 少写一个变量 | `design-tokens.test.ts` 的成对一格（审查 C 的 N4） | 亮暗成对 |
| 83 | S2 | 暗色不给 `darkMode` | `diagrams.test.ts` | 决定 888 |
| 84 | S2 | 系列色不给（去掉七族） | `diagrams.test.ts` 的 `pie1`、`git0`、`cScale0` | 决定 888 |
| 85 | S2 | `xyChart` 只给 `plotColorPalette` | `diagrams.test.ts` 的 `xyChart.backgroundColor` | 决定 888 |
| 86 | S2 | `themeCSS` 去掉圆柱一条 | `diagrams.test.ts` 的 `outer-path`；7.4 的截图 | 决定 888 |
| 87 | S2 | 已完成的项自己的文字不划掉（去掉 `line-through`） | PG17 第 2 格第 5 步（87e，审查 C 的 M12） | |
| 88 | S2 | `said` 的成功一句看得见（去掉 `[data-state="copied"]` 的藏起） | PG17 第 2 格第 3 步（盒子宽，88e） | 决定 890 |
| 89 | S2 | 不认按的次数（去掉 `press !== presses`） | `code-blocks.test.ts` 按两次一格 | |
| 90 | S2 | 复制成功时按钮不变 ✓（不设 `copy.dataset.state`） | `code-blocks.test.ts`；PG17 第 2 格第 3 步 | 决定 890 |
| 91 | S3 | `markupMarks` 不加（`SourceEditor` 的扩展里去掉） | `source-editor.test.tsx` 的 `[[` 一格 | 决定 889 |
| 92 | S3 | `markupMarks` 不看 `verbatim`（行内代码与代码块里也压淡） | `prose-lines.test.ts` 的行内代码与围栏一格 | 决定 889 |
| 93 | S3 | `markupMarks` 遍历整篇（不看 `visibleRanges`） | `prose-lines.test.ts` 8,000 行一格（数 `decorations.size`） | 上限（M12/P4/S5） |
| 94 | S3 | `markupMarks` 不跳过 frontmatter | `prose-lines.test.ts` 的 frontmatter 一格 | 决定 889 |
| 95 | S3 | `proseTheme` 不加 `Prec.high`（排在 `editorTheme` 之后，初稿的写法） | `source-editor.test.tsx` 的 `lineHeight` 与 `paddingLeft`（审查 B 的 I2） | 决定 874 |
| 96 | S3 | 编辑时的大纲构造时不读（等 300 ms） | `edit-outline.test.ts` 构造一格；`page-panel.test.tsx:186-199` 进编辑一格（审查 A 的 M3） | |
| 97 | S3 | `view.state` 换了就放弃这一轮（初稿的写法），放弃之后照修订稿重排 300 ms | `edit-outline.test.ts` 只改选区、只带 effect 两格（只推 0 ms 的计时、300 ms 之前就要报出；把计时推过 300 ms 的写法下这条变异会从重排的那一轮报出、漏过，审查 C 的 R1-M6；审查 A 的 I4） | |
| 98 | S3 | `where` 不映射（答报出时的 `from`） | `edit-outline.test.ts` 的位置一格（审查 A 的 M2） | |
| 99 | S3 | 编辑时大纲的 `key` 用 `from` | `page-panel.test.tsx:186-199` 同一个元素一格（异步的改动在拿着焦点的那个标题之前，审查 C 的 R1-N4；审查 A 的 M2） | 焦点的去向 |
| 100 | S3 | `destroy` 只清 300 ms 的计时，不清那一片的 `setTimeout(0)` | `edit-outline.test.ts` 在两片之间 `destroy` 的一格（审查 C 的 R1-N4；审查 A 的 N5） | |
| 101 | S3 | `frontmatterRange` 不按 `state.doc` 记下 | `yaml-quotes.test.ts` 第二次不扫一格（审查 A 的 N4） | |
| 102 | S1 | 第一次量排到下一帧（不在 layout effect 里同步做，初稿与修订稿的写法） | `current-heading.test.ts` 挂上之后不等一帧一格；第 1 节 (k)：照这写法 `arrival.test.tsx` 的焦点一格时有时无地红 | vitest 的 `act` |
| 103 | S1 | `pin` 不记"量到过"（第一次量时不在视口就解除，修订稿的写法） | `current-heading.test.ts` 的 `pin` 时在视口外一格（审查 A 的 R1-M1） | 决定 893 |
| 104 | S1 | 带修饰键的点按也 `pin`（`plainPress` 只看主键） | `page-panel.test.tsx` 按住修饰键点按一格（四个键各一次；第一次 Control 点按之后就是第 9 项） | 决定 893 |
| 104b | S1 | `plainPress` 只排除 Cmd、Ctrl（Shift、Alt 加点按也 `pin`，审查 A 的 R2-N1） | `page-panel.test.tsx` 按住修饰键点按一格（到 Shift 那一次变成第 9 项，审查 C 的 R3-N1） | 决定 893 |
| 105 | S1 | `active` 变假时不清当前项 | `current-heading.test.ts` 记下每次渲染的一格（变真的那次答收起之前的那一项，审查 A 的 R1-N5） | |
| 106 | S1 | `NotebookSettingsLayout` 的 `shown === undefined` 一支不画 `ColumnTop` | `upgrade-banner.test.tsx` 决定 898 的一格（`addcfa69`：横幅到"基本信息"的标题出来才有，不是从出现起一直在列里；照实，原写"无格抓"，那时它只在索引地址换到"基本信息"之前的一次渲染里，审查 B 以截图看到它成立） | 决定 892、898 |
| 107 | S2 | 甘特亮色"进行中"的两项、暗色"关键"的底、暗色看板的列名、时间线的 `filter: none` 去掉任一 | `diagrams.test.ts` 的各一句断言；7.4 的截图 | 决定 888 |
| 108 | S3 | "保存"不需要时卸下（不是 `invisible`，修订稿的写法） | PG17 第 3 格第 5 步（状态的右边移动，108e）；`page-edit.test.tsx` 没有修改一格（它在 DOM 里） | 决定 894 |
| 109 | S3 | `held` 只看"保存"自己（885 的写法，不看右边一组） | `page-edit.test.tsx` 打了字之后焦点在"完成"上一格（审查 A 的 R1-M2） | 决定 895 |
| 110 | S3 | `reveal` 用映射过的位置当光标，不取行首 | `source-editor.test.tsx` 引用里的标题一格（审查 A 的 R1-N3） | |
| 111 | S3 | 光标色照修订稿写在 `.cm-cursor` 上（`.cm-content` 不给 `caretColor`） | `source-editor.test.tsx` 的 `caretColor` 一格（审查 B 的 R1-M1） | |
| 112 | S1 | `sameView` 不比 `active`（再修订稿的"照旧"） | `page-panel.test.tsx` 展开与收起时的量两格（存着收起时打开、展开之后不带 `aria-current`；展开时挂上、收起之后照旧量；审查 A 的 R2-I1） | 决定 886 |
| 113 | S1 | 右边一组去掉 `min-h-8` | PG17 第 3 格第 3 步（内容读到之前标题上跳 16 px，375 与 820 宽两遍都红，113e：先跑的 375 宽红在 `:288`，820 宽那一遍由审查 C 单独跑，也红在 `:288`；第 1 节 (n)，审查 A 的 R2-I2） | 决定 894 |
| 114 | S1 | 不活跃时 `pin` 照样设当前项、建 `watchReader`（去掉 `live` 的判断） | `current-heading.test.ts` 不活跃时 `pin` 一格（变真时答点了的那一项，审查 A 的 R2-N2） | 决定 886 |
| 115 | S1 | `active` 变假时不结束点了的那一项 | `current-heading.test.ts` 活跃时 `pin`、收起再展开一格（展开时答点了的那一项） | |
| 116 | S3 | 焦点进组就保住"保存"（`into` 不看 `unsaved`，第 1 轮的 895） | `page-edit.test.tsx` 没有修改时焦点到"完成"、开关一格（审查 A 的 R2-M1） | 决定 895 |
| 117 | S3 | 焦点已在组里时才有了修改不保住（去掉看 `unsaved` 的第二个 effect） | `page-edit.test.tsx` 焦点在"完成"上时改正文一格（自动保存之后"保存"藏起） | 决定 895 |
| t1 | S1 | `EditButton` 的微任务去掉 `edit.current === null` 这一半 | 等价（`page-layout`、`page-autosave`、`page-edit`、`page-lock` 103 格全绿，审查 C 重跑同）：微任务里 `edit.current !== null` 只发生在 StrictMode 模拟卸下的那一次，那时"编辑"还拿着焦点，`focusLost()` 已为假；"编辑"改成 `disabled`（不是 `aria-disabled`）拿着焦点时就不再等价（审查 C 的 P4S1C-N7） | |
| t2 | S1 | 右栏跟着时不取整（去掉 `Math.ceil`） | PG17 第 8 步（项的底差 0.375 px） | 决定 886 |
| t3 | S1 | "已保存。"时不写 `data-saved` | `page-edit.test.tsx` | 决定 866 |
| t4 | S1 | 保存失败时 `<output>` 不带 `text-destructive` | `page-edit.test.tsx` | 决定 866 |
| t5 | S1 | "保存"进了 after 的落点 | `page-edit.test.tsx` 的次序一格 | 决定 866 |
| t6 | S1 | `propertyRoles` 不注册 | `717f5334` 上没有格抓；`d29b68b0` 之后 PG17 第 15 步（`role="table"`） | 决定 864 |
| t7 | S1 | 开关去掉 `aria-controls` | `page-panel.test.tsx` 开关一格 | 名字与状态 |
| t8 | S1 | 不在正文旁边时也把右栏里的焦点交给开关 | 等价（`page-panel.test.tsx` 17 格全绿，审查 C 重跑同）：开关是 `hidden xl:inline-flex`，`useBeside` 是 `(width >= 80rem)`，与 Tailwind 的 `xl` 同界，不在旁边时开关 `display: none`，`focus()` 什么也不做；两处的界不同了就不再等价，jsdom 不读 CSS，没有格分得出（审查 C 的 P4S1C-N7） | |
| t9 | S1 | `active` 不看 `useBeside()` | `page-panel.test.tsx` 不在正文旁边一格 | 决定 886 |
| t10 | S1 | 属性的值 `overflow-wrap: normal` | PG17 第 15 步（区域一个） | 375 px |
| t11 | S1 | 没有列出的标题时照样量（`reading = active`） | `current-heading.test.ts`"with no heading listed"一格 | |
| t12 | S1 | 读者动过按页记（大纲重新挂上也不再跟） | `page-panel.test.tsx` 897 一格与视图晚到一格 | 决定 897 |
| t13 | S1 | 决定 898 那一层去掉（索引那一支的 `ColumnTop` 照旧在全宽） | `upgrade-banner.test.tsx` 898 一格 | 决定 898 |
| t14 | S1 | "编辑过"一句回到整句 `truncate` | `page-layout.test.tsx` 中英两格；PG17 第 2 步（"how much of the sentence is cut"） | 决定 899 |
| t15 | S1 | "编辑过"一句去掉 `whitespace-pre` | `page-layout.test.tsx` 两格（读类名） | 决定 899 |
| t16 | S1 | 名字的 `bdi` 只留 `truncate`（去掉 `min-w-0`） | `page-layout.test.tsx` 两格（读类名）；PG17 第 2 步在三种引擎里都过，行为等价：`truncate` 带 `overflow: hidden`，这样的弹性项的自动最小宽度是 0，`min-w-0` 多余；换成不带 `overflow: hidden` 的写法（只 `text-ellipsis whitespace-nowrap`）就不再等价（审查 C 的 P4S1C-F1-N1） | 决定 899 |
| t17 | S1 | 锁说明的 `output` 去掉 `min-w-0 wrap-anywhere` | PG17 第 4 格，只在 Firefox 红（页的 `expectFits`，`scrollWidth` 658）；Chromium、WebKit 在连字符处断行，绿；`edit-lock-note.test.tsx` 抓不到 | 375 px |
| t18 | S1 | 版本页"时间 · 作者"一行去掉 `min-w-0 wrap-anywhere` | PG17 第 4 格，只在 Firefox 红（版本页的 `expectFits`，660）；`version-page.test.tsx` 抓不到 | 375 px |
| t19 | S1 | `justify-end-safe` 改回 `justify-end` | `page-layout.test.tsx` 两格（读类名） | |
| t20 | S1 | 视图收回 4 px 的选择器改回 `[:first-child:empty+&]:-mt-1`（作用在 `AttachmentDrop` 自己身上） | 无格抓：临时故事量出三种引擎读取中、读取失败回到离标题 10 px（7.4） | |
| t21 | S1 | 读到的一节改回观察 `main` | `current-heading.test.ts` 观察者一格（`[[main]]` 不等于 `[[article]]`）；WebKit 的报错由探针看（4.2 之前的"照实"） | |
| t22 | S1 | PG17 第 12 步刷新之后不等锚点的标题拿到焦点 | 只在扣住视图的探针下红（`toBeHidden`，焦点在"Part 39"的 h2）：守的是竞态，平常绿 | 决定 900 |
| t23 | S1 | PG17 第 14 步改回刷新带锚点的地址 | 只在扣住视图到"Edit"的盒子读完的探针下红（Chromium、WebKit，"how far Edit moved down"，22462、22461 px）：守的是竞态 | 决定 858 |
| t24 | S1 | `versionBy` 种类前的不断行空格改回普通空格 | `version-page.test.tsx` 5 格（找不到名为"Ada · Edit"的链接） | |
| t25 | S1 | 版本页一行"·"之后的不断行空格改回普通空格 | `version-page.test.tsx` 1 格（那一行的 `textContent`） | |
| t1 | S2 | 按下时不清上一句的计时 | `code-blocks.test.ts` 第 8 格（旧计时清掉新的一句） | |
| t2 | S2 | 按下时不先清掉上一句 | `code-blocks.test.ts` 第 8 格 | |
| t3 | S2 | `said` 去掉 `aria-live` | `code-blocks.test.ts` 复制一格 | |
| t4 | S2 | "复制"去掉 `type="button"` | `code-blocks.test.ts` 第一格（`submit`） | |
| t5 | S2 | 头 `append`、不 `prepend` | `code-blocks.test.ts` 11 格（`parts()` 取 `pre` 的第一个子元素；S2 之初是 8 格，审查 C 的 P4S2C-N2；`0e35d0dc` 上 10 格）；PG17 第 2 格绿（只有 vitest 抓） | |
| t6 | S2 | 语言名以 `className` 切 | `code-blocks.test.ts` 的 rust 一句（`'uage-rust'`） | |
| t7 | S2 | 清理不清计时 | `code-blocks.test.ts` 清理一格（计时数 1） | |
| t8 | S2 | `keyOf` 不给 `copy` | `patch.test.ts` 1 格、`reading-view.test.tsx` 2 格 | |
| t9 | S2 | 不注册 `codeBlocks` | `reading-view.test.tsx` 2 格 | |
| t10 | S2 | 没有语言也没有剪贴板时照样加头 | `code-blocks.test.ts` 没有剪贴板一格（空的头） | |
| t11 | S2 | 去掉图里 `foreignObject` 的一条（`cdedff02`） | L4 (page) 第一格 `l4:205`（255 对 232） | |
| t12 | S2 | `.nw-diagram` 去掉 `forced-color-adjust: none` | L4 (page) 第一格 `l4:204`（底 `rgb(0, 0, 0)`） | |
| t13 | S2 | 删掉最外层的 `.dark` 块（只有 `:root`） | `design-tokens.test.ts` 成对一格（7.1 要看的一次） | |
| t14 | S2 | callout 的标题回到 `display: flex; align-items: center; gap: 7px` | PG17 第 2 格第 7 步，三种浏览器（`innerText` 拆行；`ade3abd1` 上 1440 宽的行数是上限，照红） | 决定 901 |
| t15 | S2 | 去掉"标题之后 4 px"一条 | PG17 第 2 格第 8 步 | 决定 902 |
| t16 | S2 | callout 块间回到 0.25rem | PG17 第 2 格第 8 步 | 决定 902 |
| t17 | S2 | 去掉 `li > p:not(:first-child)` | PG17 第 2 格第 8 步（松散项的第二段 0 px） | 决定 903 |
| t18 | S2 | `li > :is(pre, …)` 加 `:not(:first-child)` | PG17 第 2 格第 8 步（紧凑项的 `pre` 0 px） | 决定 903 |
| t19 | S2 | 指定的主题不算（只看变量） | `diagrams.test.ts` 自己的主题一格 | 决定 904 |
| t20 | S2 | `base` 也算作者的主题 | `diagrams.test.ts` 的 `base` 一格 | 决定 904 |
| t21 | S2 | 作者的主题照旧加应用的 `themeVariables` 与 `themeCSS` | `diagrams.test.ts` 3 格（自己的主题、`base` 加变量、只有变量） | 决定 904、906 |
| t22 | S2 | 在 `tooLarge` 之前解析 | `diagrams.test.ts` 的 `base` 一格（太大的思维导图带 frontmatter） | |
| t23 | S2 | 不看 `alone`（`only-child` 之前的选择器） | `code-blocks.test.ts` 几个 `code` 一格与文字夹着 `code` 一格 | |
| t24 | S2 | `told` 不看 `undone` | `code-blocks.test.ts` 清理之后才答的一格 | |
| t25 | S2 | 清理不设 `undone` | 同上一格 | |
| t26 | S2 | `reading-view.test.tsx` 的 `afterEach` 不拆剪贴板 | 无格抓（66 格绿）：今天没有格依赖它，次序依赖由临时探针看到 | |
| t27 | S2 | chevron 去掉 `position: absolute` | 无格抓（PG17 绿）：chevron 的位置只由截图看 | |
| t28 | S2 | 去掉 `flowchart: { padding: 9 }` | `diagrams.test.ts` 第一格 | |
| t29 | S2 | 只看主题、不看变量 | `diagrams.test.ts` 2 格（`base` 加变量、只有变量） | 决定 904、906 |
| t30 | S2 | 只给变量时主题取 `base` | `diagrams.test.ts` 只有变量一格 | 决定 906 |
| t31 | S2 | 已完成项下面的块不 `inline-block`（只剩 `ul, ol`） | PG17 第 2 格第 5 步，三种浏览器 | |
| t32 | S2 | 已完成项下面的块的颜色一条只剩 `ul, ol` | PG17 第 2 格第 5 步（颜色） | |
| t33 | S2 | 去掉 `vertical-align: top` | PG17 第 2 格第 5 步，三种浏览器（间距 17 对 8） | |
| t34 | S2 | 去掉 `width: 100%` | PG17 第 2 格第 5 步（不满宽，间距也变） | |
| t35 | S2 | 链接与标签那条的 `:not()` 回到 `(ul, ol)` | PG17 第 2 格第 5 步（callout 的标签是正文色，`02cb019e` 补的一句） | |
| t36 | S2 | 只给变量时主题总取 `default` | `diagrams.test.ts` 只有变量一格（暗色应用） | 决定 906 |
| t37 | S2 | 没有指令与 frontmatter 也解析 | `diagrams.test.ts` 没有指令与 frontmatter 一格 | |
| t38 | S2 | 只看 `%%{`、不看开头的 `---` | `diagrams.test.ts` 自己的主题一格（frontmatter 的 `forest`）与没有指令一格（空白之后的 frontmatter） | |
| t39 | S2 | `alone` 不看文字节点 | `code-blocks.test.ts` 文字夹着 `code` 一格 | |
| t40 | S2 | 903 的 `:is()` 放回 `table` | PG17 第 2 格第 6 步，三种浏览器（32.75 对 24.75 px） | 决定 903 |
| t41 | S2 | 已完成项下面的块去掉 `blockquote, details`（审查 C 的 c1） | PG17 第 2 格第 5 步，三种浏览器（引用与 `details` 是 `block`） | |
| t42 | S2 | 已完成项下面的引用也设回正文色 | PG17 第 2 格第 5 步（引用是正文色，应是次要色） | |
| t43 | S2 | callout 的标题 `white-space: nowrap` | PG17 第 2 格第 7 步的 `expectFits`（375 宽时阅读视图横滚），三种浏览器 | 决定 901 |
| t1 | S3 | `textOf` 留着 `QuoteMark` | `edit-outline.test.ts` 列出一格 | |
| t2 | S3 | setext 不在下划线那一行的行首截断 | 同上 | |
| t3 | S3 | `same()` 永远答不同 | `edit-outline.test.ts` 次数一格 | |
| t4 | S3 | 两片之间改了文档也接着补 | `edit-outline.test.ts` 改文档一格 | |
| t5 | S3 | 没闭合的 frontmatter 不在空行停 | `yaml-quotes.test.ts` 2 格、`prose-lines.test.ts` 1 格 | |
| t6 | S3 | `listed` 用深的 observable（不是 `observableRef`） | 等价（`page-panel.test.tsx` 绿，审查 C 重跑同）：写 `listed` 的只有 `told`（`headingsOf` 每读一次新建一个数组）与 `clear`（新的 `[]`），读它的只有 `EditOutline`，只 `reduce`、`map` 取 `level` 与 `text`，没有人按身份比较 `box.listed`、也没有人改其中的项；插件留着的是自己的原数组（`same()` 比的是它），深的 observable 是另一份拷贝；MobX 没有设 `observableRequiresReaction`。只多每次报出时把至多 1,000 个对象包成 observable 的开销。有人按身份比较 `box.listed`、插件就地改已经报出的数组、或页这一边改其中的项时，就不再等价（审查 C 的 P4S3C-N3） | |
| t7 | S3 | 截断挪回 `button`（`3c75954d` 之前） | PG17 第 3 格 `:354`，三种浏览器（`block` 加进来之后三种都红，同 t29e；之前只有 WebKit 红，审查 C 的 t7e） | |
| t8 | S3 | 去掉 `.nw-frontmatter-line *` 的 `inherit` | `source-editor.test.tsx` frontmatter 一格（`9a52ddb4`；之前在全部 vitest 下活着） | 决定 874 |
| t9 | S3 | 扩展里不加 `editOutline` | `source-editor.test.tsx` reveal 2 格、`page-panel.test.tsx` 2 格 | |
| t10 | S3 | 编辑时右栏不等 `ready`（一编辑就画 `EditOutline`） | `page-panel.test.tsx` 2 格（扣住正文时、再进编辑时） | |
| t11 | S3 | `clear()` 不复位 `ready` | `page-panel.test.tsx` 再进编辑一格 | |
| t12 | S3 | 不画"…and N more" | `page-panel.test.tsx` 1,003 个标题一格 | 13.2 第 25 条 |
| t13 | S3 | `same()` 也比位置 | `edit-outline.test.ts` 所有标题之前打字一格 | |
| t14 | S3 | 正在写的 frontmatter 里的也不列 | `edit-outline.test.ts` 没闭合的 frontmatter 一格 | 决定 908 |
| t15 | S3 | 不去掉强调的标记 | `edit-outline.test.ts` 项文字一格 | 决定 907 |
| t16 | S3 | wiki 链接照原文（不经 `shownOf`） | 同上 | 决定 907 |
| t17 | S3 | 脚注引用不去掉 | 同上 | 决定 907 |
| t18 | S3 | 链接的 `](地址 "标题")` 不去掉 | 同上 | 决定 907 |
| t19 | S3 | 转义的反斜杠不去掉 | 同上 | 决定 907 |
| t20 | S3 | 字符实体不解码 | 同上 | 决定 907 |
| t21 | S3 | `==` 不去掉 | 同上 | 决定 907 |
| t22 | S3 | 行内代码里的也照规则剪 | 同上 | 决定 907 |
| t23 | S3 | 转义的 `[[` 也认作 wiki 链接 | `wiki-links.test.ts` 不认的一格 | |
| t24 | S3 | 没有页名也没有小节的也认 | 同上 | |
| t25 | S3 | 嵌入的说明不去掉末尾的尺寸 | `wiki-links.test.ts` 显示一格、`edit-outline.test.ts` 项文字一格 | 决定 907 |
| t26 | S3 | 页名加小节写作 `页名#小节` | 同上两格 | 决定 907 |
| t27e | S3 | 编辑时大纲的按钮去掉 `block` | PG17 第 3 格 `:347`，三种浏览器（项高差 5.5 / 4.75 / 1.625 px） | |
| t28e | S3 | 去掉 `cursor-pointer` | PG17 第 3 格 `:348`，三种浏览器 | |
| t29e | S3 | 截断挪回按钮（同 t7） | PG17 第 3 格 `:354`，三种浏览器（178.75 / 178.83 > 36.25 / 36.27 px） | |
| t30e | S3 | `textOf` 只去掉 `HeaderMark`（初稿） | L6 (page) `:245`（Chromium） | 决定 907 |
| t31 | S3 | 引用的规则排回最后 | `source-editor.test.tsx` 引用里的标记一格 | |
| t32 | S3 | 行内代码不设字号（`4ecc3e10` 的写法上；`e6558722` 起由 t61 代替） | `source-editor.test.tsx` 行内代码一格 | |
| t33 | S3 | `.cm-content` 不设 `--nw-editor-code` | 同上 | |
| t34 | S3 | `.nw-mono-line` 不设 `1em` | 同上 | |
| t35 | S3 | `LinkTitle` 不当原样 | `prose-lines.test.ts` wiki 链接一格 | |
| t36 | S3 | 竖线的淡只给最后一个字（表格里转义的竖线只淡竖线） | 同上 | |
| t37 | S3 | 页名的范围不去空白 | `wiki-links.test.ts` 位置一格、`prose-lines.test.ts` wiki 链接一格 | |
| t38 | S3 | `markupMarks` 也认转义的 | `prose-lines.test.ts` wiki 链接一格 | |
| t39 | S3 | `markupMarks` 也认空的 | 同上 | |
| t40 | S3 | 补全选中项的外框挪进 `@media print` | `link-completion.test.ts` 强制色一格 | |
| t41 | S3 | 去掉 gutter 的一条 | `text-diff.test.tsx` gutter 一格 | 决定 910 |
| t42e | S3 | 查找框去掉自己的底（`--surface-2`） | `4ecc3e10` 上无步抓（L5 只量聚焦的那个框，聚焦时底换成 `--surface`）：`45a477bd` 加没聚焦的框一项，之后是 t50e | 决定 910 |
| t43e | S3 | 按钮去掉 `background-image: none` | L5 (page) 查找面板一步（按钮的 `background-image`） | 决定 910 |
| t44e | S3 | 面板的底写回 `--background` | L5 (page) 查找面板一步（面板的底同编辑器的底） | 决定 910 |
| t45e | S3 | 补全选中项的外框挪进 `@media print` | L5 (page) 强制色一步（`outline-style`，Chromium） | |
| t46e | S3 | `label` 写成 `--faint` | L5 (page) 查找面板一步（`label` 的下限） | 决定 910 |
| t47 | S3 | 对比视图的 `.cm-scroller` 一条去掉 | `text-diff.test.tsx` 等宽一格 | 决定 909 |
| t48 | S3 | 对比视图不写 13 px | 同上 | 决定 909 |
| t49 | S3 | `proseTheme` 的 `.cm-scroller` 不写 `--font-sans` | `source-editor.test.tsx` 主题一格的 `--font-sans` 一句 | 决定 874 |
| t50e | S3 | 同 t42e（`45a477bd` 上） | L5 (page) `:198`，暗色没聚焦的框 1.224 | 决定 910 |
| t51 | S3 | `upTo` 之后也读文字 | `edit-outline.test.ts` `upTo` 之后一格（数 `wikiLinks` 被调的次数） | 13.2 第 25 条 |
| t52 | S3 | `hasText` 不跳过 `HeaderMark`、`QuoteMark` | 同上（只有标记的标题也计数） | |
| t53 | S3 | 实体的解码不记下 | 无格抓：只省重读的代价。A 的 20,000 个标题只有一种实体，量不出它；1,000 个各带 20 个命名实体的标题，记下时重读没有 ≥50 ms 的任务，不记下时每轮一个 60–75 ms 的（审查 C 修复核对第 2 轮，A 的 harness）；`bcd1575b` 起表是每次读新建的，同一次读里照样记下：1,000 个各带 20 个命名实体的标题重读 312–323 ms 报出、没有 ≥50 ms 的任务，A 的 20,000 个 328–334 ms、也没有（实施者，`fix/g-heap.log`） | |
| t54 | S3 | `dropped` 去掉 `Comment` | `edit-outline.test.ts` 注释与嵌入一格 | 决定 907 |
| t55 | S3 | 不剪 `%%…%%` | 同上 | 决定 907 |
| t56 | S3 | 页的嵌入也取说明 | `wiki-links.test.ts` 显示一格、`edit-outline.test.ts` 注释与嵌入一格 | 决定 907 |
| t57 | S3 | `.md` 也算附件的扩展名 | 同上两格 | 决定 907 |
| t58 | S3 | 阅读大纲的 `nav` 不带 `inert`（`d7fb332c` 的写法；`bcd1575b` 起 `inert` 在那一节的 `details` 上，由 t65、t66 代替） | `page-panel.test.tsx` `inert` 一格 | |
| t59 | S3 | 阅读时也 `inert` | 同上 | |
| t60 | S3 | h1 不设 `--nw-heading` | `source-editor.test.tsx` 标题里的行内代码一格 | |
| t61 | S3 | 行内代码的字号不乘 `--nw-heading` | `source-editor.test.tsx` 行内代码两格 | |
| t62 | S3 | 查找框不写宽 | `source-editor.test.tsx` 查找面板一格 | 决定 910 |
| t63 | S3 | 查找面板强制色的焦点外框挪进 `@media print`（同审查 C 的 c-fc-search） | `source-editor.test.tsx` 查找面板一格（`text-diff`、`link-completion`、`prose-lines` 照绿） | 决定 910 |
| t64 | S3 | 附件的扩展名可以带空白（`d7fb332c` 的写法） | `wiki-links.test.ts` 扩展名一格（`v1.2 notes`） | 决定 907 |
| t65 | S3 | `PanelSection` 不把 `inert` 加在 `details` 上 | `page-panel.test.tsx` `inert` 一格 | |
| t66 | S3 | `PageOutline` 把 `inert` 加在 `nav` 上、不传给 `PanelSection`（`d7fb332c` 的写法） | 同上 | |
| t67 | S3 | 实体的表放回模块级（每次读共用一份、不清） | 无格分得出：行为相同，由堆的量看（`bcd1575b` 编辑器换成空文档之后 2.6 MB，`e6558722` 21.9 MB，t53 2.5 MB；t67 在 `bcd1575b` 上 21.9 MB，审查 C 修复核对第 3 轮） | 13.2 第 25 条 |
| c-indent | S3 | 编辑时的大纲不按级缩进（审查 C） | `page-panel.test.tsx` 进编辑一格的 `paddingLeft` | |
| c-codeblock | S3 | `verbatim` 去掉 `CodeText`、`CodeBlock`（审查 C） | `prose-lines.test.ts` 行内代码一格（缩进代码块那一行） | |
| c-noscroll | S3 | `reveal` 不发 `scrollIntoView`（审查 C） | `source-editor.test.tsx` reveal 2 格；L6 的视口一句（noscroll-e，Chromium） | |
| c-more | S3 | 不画"…and N more"（审查 C） | 第一轮活着（P4S3C-M3）；`15372fcf` 起 `page-panel.test.tsx` 1,003 个标题一格 | 13.2 第 25 条 |
| c-clear | S3 | 离开编辑不 `clear()`（审查 C） | 第一轮活着（P4S3C-N1）；`15372fcf` 起再进编辑一格 | |
| c-ready-never | S3 | `told()` 不设 `ready`（审查 C） | `page-panel.test.tsx` 6 格 | |
| c-upto | S3 | `upTo` 少一（审查 C） | `page-panel.test.tsx` 1,003 个标题一格 | |
| c-clamp3e | S3 | 截断写成三行（审查 C） | PG17 第 3 格 `:354`，三种浏览器（53.6 > 36.25 px） | |
| c-active | S3 | 编辑中的阅读大纲 `active={active}`（审查 C） | 等价：等正文的那一段里阅读视图已卸下，`article h2` 是 0 个，没有可量的标题，至多多挂一个量不到东西的滚动监听（C 的探针）；编辑时阅读视图仍留在 DOM 里（例如改成藏起而不是卸下）时就不再等价 | |
| c-fc-search | S3 | 查找面板强制色的焦点外框挪进 `@media print`（审查 C） | 修复核对第 1 轮活着（P4S3C-F1-N2）；`e6558722` 起是 t63 | 决定 910 |
| c-h2 | S3 | h2 不设 `--nw-heading`（审查 C） | `source-editor.test.tsx` 标题里的行内代码一格 | |
| c-comment-code | S3 | 行内代码里的 `%%…%%` 也剪（审查 C） | `edit-outline.test.ts` 注释与嵌入一格 | 决定 907 |
| c-decode-fresh | S3 | 实体每次新建 `textarea`、不记下（审查 C；`d7fb332c` 之前的写法） | 等价，同 t53：只省代价 | |
| c-read-past | S3 | `upTo` 之后照文字计数（审查 C；`d7fb332c` 之前的写法） | `edit-outline.test.ts` `upTo` 之后一格（数 `wikiLinks` 的调用） | 13.2 第 25 条 |

不列的：4.8 最低清单里不在 P4 的几条（⌘K、即时生效、设置目录、`notebookGroups`、抽屉）由 P2、P3 守着；`se1` 的那一半（`--faint` 调淡，se1 红）P4 不碰搜索页。20、35 没有格抓，由 7.4 的截图看（106 原也没有，决定 898 之后由 `upgrade-banner.test.tsx` 的一格抓；31 等价，见它那一行）；S1 的 t1–t25 是 S1 的实施者另写的（t22、t23 改的是故事，只在扣住视图的探针下红；t1、t8 等价，t16 在 e2e 里等价，依据都在行里；t20 没有格抓；t17、t18 只有 Firefox 的 PG17 第 4 格抓），审查者另写的与逐条的结果在 S1 的审查记录；52 等价（依据在它那一行）。S2 的 t1–t43 是 S2 的实施者另写的（t26、t27 没有格抓：t26 由临时探针、t27 由截图看；t5 只有 vitest 抓），审查者另写的与逐条的结果在 S2 的审查记录；格数随 S2 的修复变了的：44 是 2 格（`alone` 之后，没被清掉的旧头让 `code` 不再独自在 `pre` 里，不再叠一个头）、48 是 5 格、49 是 5 格、t5 是 11 格（`ade3abd1` 上）。 S3 的 t1–t67 是 S3 的实施者另写的（t6 等价，依据在它那一行；t53 只省代价、没有格抓，由重量看；t67 没有格分得出，由堆的量看；t42e 的洞由 `45a477bd` 补，之后是 t50e；t32 是 `4ecc3e10` 的写法，`e6558722` 起由 t61 代替；t58 是 `d7fb332c` 的写法，`bcd1575b` 起由 t65、t66 代替），`c-` 开头的是审查 C 另写的（c-active 等价，c-decode-fresh 同 t53 只省代价），逐条的结果在 S3 的审查记录。`useCurrentHeading` 的"值不变就不设"（`shown` 的比较）不另列：去掉它时 React 自己多半也不排更新（同值的 `setState` 提前退出），它是让"不排"成为确定的一层，第 1 节 (k) 的三遍全绿是连同它跑的。决定 885 的"拿着焦点、没东西可存时按它不做事"是 `save()` 原有的判断（`version === savedVersion` 时返回），不另列变异，`page-edit.test.tsx` 的"再按它不发 `PUT`"看着它。决定 889 里下划线落在哪些字上（`tags.link` 不再画下划线）由 7.4 的截图看：下划线画给里面的行内元素而不继承，每个元素自己的 `getComputedStyle` 读不出它落在哪些字上。

### 7.3 门禁

- **基线**：不另跑（决定 883）。对照的数是第 1 节的：vitest 221 个文件、5325 格；E2E 624 格过、3 格失败、1 格跳过。
- **每个提交**：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`；改了行为、夹具或故事的另跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make e2e`（S1、S2、S3 的提交 1、2 都要）。
- **合并之前**：在 Step 分支并入 `m13-ui-refresh` 的树上跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web gen-check-web build-web e2e`；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 以 0 退出；`git diff --check`；结果写进 Step 的记录。本机允许的失败只有第 1 节那三格（同行同消息）；AS6、LW8:771、"not slower"、`trash-preview.test.tsx:587` 与造数的 500 偶发，重跑全过即可，记下；别的红了先停、查清。PG17 不碰 PG15、LW8 的文件，那三格的行号不变。
- **只靠 CI 的**：没有新的（PG17 不按 `Control+K`，剪贴板经 `fixtures/clipboard.ts` 在三种引擎里都能读）。Linux 的 WebKit 与 Firefox 上剪贴板的读法、Linux 的字体与滚动条下 PG17 的 `expectFits` 与右栏的量，由负责人推送之后的 CI 看（w5、a10 已在三种浏览器里用同一个剪贴板的夹具；本机的门禁照旧）。 S2 起另有：Linux 的 Chromium 第一次跑 `emulateMedia({ forcedColors })`（L4 的强制色一步，审查 C 的 P4S2C-M1）；PG17 第 2 格第 7 步在 Linux 的字体下的行数（取上限）。 S3 起另有：PG17 第 3 格的项高与长标题的行数、L5 的查找面板与强制色两步在 Linux 的字体下（比的是同一页里的差、按行高算的界与计算后的颜色，推断照样成立，审查 C 修复核对第 1 轮的未核实一条）。
- **文档**：`python3 docs/v0.1/claude_to_codex/tools/check_links.py origin/m13-ui-refresh`（远端的分支不在时用 `m13-ui-refresh`，同 02-P2 7.3）、`git diff --check`。

### 7.4 视觉的核对

B 审查者用 Playwright 截取下面这些，亮暗、1440 与 375 各一张，对照 reading.html 看，记录写明看了哪些：

- **S1**：页的阅读（头一行、标题、属性条、正文、右栏的四节）；1280 宽（内容列 696）与 1024 宽（右栏在正文之后）；右栏收起（1280–1303 px 之间列宽由 696 变 720，8.2 第 2 条）；长的面包屑与长的标题（折行）；名字长的"编辑过"（只截名字、时间完整，决定 899；480 与 375 宽时"编辑"不随它的到来移动）；375 与 820 宽时头一行在阅读、刚进编辑、打了字、自动保存之后都是两行，1024 宽起都是一行（决定 894）；一个两行的失败说明（820 宽的"The network was too slow…"、375 宽的"网络太慢……"）在头一行里折行、行高不变（变异 35；`min-h-8`，审查 B 的 R2-N1）；375 宽时英文三行的失败说明（"too slow"、NUL、"Cannot reach the server for now…"）让头一行高 16 px（照实记下、不截断，审查 A 的 R2-N3）；大纲读到的一节、长标题两行截断（变异 20）、强制色下的当前项（下划线）、一个以标题开头的折叠 callout（大纲里有它的项、从不标为当前）；反链的小卡片（摘要两行）；属性条（列表值、嵌套的表、长值、解析不到的链接的虚线照旧）；版本一行（"时间 · 谁写的"一种字，"当前"在最右；名字长时接着时间折行，"当前"仍在第一行的最右，审查 B 的 R2-N2）；编辑时的头一行（状态四态：空、未保存、保存中、已保存带 ✓；状态是 12.5 px 的淡色）、编辑说明；阅读时的锁说明，页与版本页各一张（同一个组件，审查 C 的 M14）；升级中的页（横幅在列里、标题之前）、升级中的设置页（横幅在 696 px 的列里）、升级中不存在的页；回收站的预览与变更集页的面包屑（随之换了，记下）。有意的偏离：属性的值是正文色（00 号文档，样稿是次要色）；没有小节的计数（决定 863）；编辑时没有页面菜单（00 号文档）。
- **S2**：正文的标题（标题接标题的间距，审查 B 的 M2）、段落、链接（悬停）、没建的页、标签、高亮、行内代码、表格（行高 1.5）、引用、列表、任务项（勾与不勾、可用与不可用、嵌套；已完成项里的链接与标签随之变淡；划线从复选框后的空格起，审查 B 的 N1，记成偏离）、七族的 callout（亮暗、折叠的与它的箭头、callout 里的代码块 13 px、强制色下的图标）、代码块（几种语言、长行横向滚动、头、按下之后的 ✓ 与"没能复制"、强制色下的图标、打印预览里没有头）、mermaid 亮暗各一：流程图（9 px 圆角，带圆柱的与 `style` 着色的圆柱）、时序图（注）、饼图（多于七块时颜色重复，记成的偏离）、甘特图（网格线、完成、进行中、关键、关键加完成的条里的字、今天线与 `vert` 标记线，审查 B 第 2 轮的未核实一条）、git 图、思维导图、时间线、xychart、ER（带属性、圈）、状态图、看板（暗色的列名）、块图、时间线（事件框不提亮）、旅程（灰的节）；375 宽时图的字约 7 px（今天就这样，记下）；强制色下的一张图（Chromium 的暗色高对比加亮色应用：图照应用的主题画、字看得见，审查 B 的 R2-N3）；公式；版本页与回收站的预览（同是 `.nw-reading`，记下）。Go 的 hljs 不给方法调用标 title、`nil` 归常量色（审查 B 的 N5，记成偏离）。
- **S3**：编辑器（亮暗；frontmatter（整块次要色，链接与标签不画链接的样子；样稿的 `---` 淡、值是正文色，决定 874 记的偏离）、围栏代码、表格、引用里的代码、长的正文行、标题、链接（`[[页名]]` 只有页名带下划线、括号淡；`[文字](地址)` 的括号与地址淡；`==高亮==` 两端淡）、选区、琥珀的光标）；长文档拖滚动条；编辑时的大纲（长标题、frontmatter 不在里面）；"保存"的出现、拿着焦点时留着与消失、没有修改时焦点进组它不冒出来；有意的偏离：藏起的"保存"照旧占位，状态离开关约 70 px（样稿 8 px，决定 896）；冲突的面板、失锁的横幅；补全的弹层（亮暗）；对比视图（等宽、只换配色，`[[ ]]` 与 `==` 照旧正文色）。

### 7.5 CI 的余量

新故事一个：PG17 三格，三种浏览器，共 9 次。PG16 在 P2/S2 的实测是每次 1–4 s；PG17 第 1 格走的多（一次就地更新、两次刷新、四次改窗口大小、十来次滚动与 `expect.poll`），推断每次 6–10 s；第 2 格一次复制、等 1.5 秒，推断 2–4 s；第 3 格两种宽度各进一次编辑、等一次自动保存（停手 2 秒之后），推断 7–10 s。合计约 45–72 test-seconds，CI 的 `ubuntu-latest` 约 2 个 worker，约合 23–36 秒的墙钟；UP1 多两句断言，不计。本机 E2E 2.5 min 推断到 2.8–3.0 min。CI 的 E2E 一步今天约 6 分 21 秒，时限 18 分钟（00 号文档第 9 节），余量足够。其余故事只换定位与断言，次数不变。S1、S2 之后照实各记一次（S1 的在 10.1：第 1、3、4 格合计约 32–42 test-seconds，本机 E2E 2.6–3.8 min，负载 10–62；S2 的在 10.2：第 2 格每次 2.2–3.2 s，本机 E2E 2.6–2.8 min，负载 8–68；S3 的在 10.3：第 3 格每次 6.7–7.6 s，本机 E2E 2.6–3.0 min，负载 4–45）。

**本机的总时长**（粗算，经 `heavy.sh` 串行）：六个代码提交各一次 `make lint-web knip test-web`（约 70 秒）约 7 分钟；六次 `make e2e`（约 5 分钟，含构建）约 30 分钟；三次合并之前的整条命令约 13 分钟、审查 C 各再跑一次，约 78 分钟；变异 118 条（含 104b），约 84 条只跑 vitest（一个文件 15–40 秒）约 45 分钟，31 条要重建前端再跑一个故事（每条约 3 分钟）约 93 分钟（审查 C 的 R3-N2），3 条没有格抓；修复核对每个 Step 一两轮约 45 分钟；S1 的全部 vitest 另跑三遍约 4 分钟。合计约 5.0 小时的锁时间（审查 C 的 R4-N1）。

## 8. 完成标准

### 8.1 条件

1. 4.1–4.5 的每一项做到；7.1 的测试通过；7.2 的变异都被抓住（20、35 由截图看，52 等价；31 与 S1 的 t1、t8 等价；S3 的 t6 与审查 C 的 c-active 等价，t53 与 c-decode-fresh 只省重读的代价、由重量看，t67 由堆的量看；S3 的 t7 由 PG17 第 3 格守）。
2. 每个 Step 的合并之前，7.3 的门禁为绿（只败第 1 节同行同消息的那几格）；没有服务端与契约的改动。
3. 7.4 的截图看过，与样稿不同之处写进记录（是有意的，写理由；不是的，修）。
4. Phase 完成（02-workflow 3.7）：S3 合并、它的合并记下之后，另一个提交 `docs: M13/P4 done (M13/P4)`（不在 S3 的文档提交里，审查 C 的 M13）：00 号文档第 12 节 P4 一行"完成"、头的状态、第 13 节一行；本文头的状态。`docs/v0.2/handoffs/M12-polish.md` 的"处理进展"由每个 Step 的文档提交各写一行（决定 881），这里不另写。

### 8.2 总体设计的新文字

每条由 Step 的文档提交里的脚本替换（"换成"是旧文字换成新文字，"之后加"是新文字接在旧文字之后），断言旧文字恰好出现一次：S1 的第 1–11 条的旧文字在 `072dcec2` 的 `docs/v0.1/v0.1-design.md` 里各出现一次；S2 的第 12–18 条、S3 的第 19–28 条的旧文字是之前的 Step 写进去的（或那之后仍在的），在之前的 Step 改完的文字里各出现一次。设计者用一个脚本从本节读出这些文字（引用块是旧文字与新文字），依次做了 S1、S2、S3 的替换，每个旧文字在它那一步做之前都恰好一次，每条新文字在它那一步做完之后也恰好一次（脚本在会话的临时目录，不进仓库）。中文里的引号照总体设计的写法是 ASCII 的 `"`。链接在这里写在反引号里（它们相对于 `docs/v0.1/`，不是本文的链接，免得 `check_links.py` 把它们当成本文坏掉的链接），写进总体设计时不带反引号。13.4 第 3 条的新段以一个空格开头（同这一条里 M13/P2、P3 各段的接法）。长期的文字只写行为，类名与 effect 的先后留在第 4 节与代码的注释里。

1. **9.2 中间一段**（S1）：头一行、720 px 的一列、属性条、编辑说明、升级提示的位置。

   旧文字：

   > - **中间**：面包屑、页面标题（即节点名）、阅读 / 编辑切换、正文、子页面列表、附件面板。笔记本首页列出根下的附件（M7）。升级中的笔记本的每个页面上方有一条常驻的提示（M11）。

   换成：

   > - **中间**：最上面一行，左边是面包屑，右边阅读时是"谁、何时编辑过"（版本列表的最新一项，晚到时别的不动）、右栏的开关、页面菜单与"编辑"，编辑时是保存的状态、"保存"、右栏的开关与"完成"；右边那一组定宽，头一行的行数只看宽度，进出编辑与"编辑过"到来时标题不动；其下居中、最宽 720 px 的一列：页面标题（即节点名），阅读时标题下是属性条（服务端的属性表经 CSS 排成一条小块，自动折行）与正文，编辑时标题下是一句"你正在编辑，别人此时只能阅读这一页"与编辑器；再下是子页面列表、附件面板（M13/P4/S1）。笔记本首页列出根下的附件（M7）。升级中的笔记本的每个页面上方有一条常驻的提示（M11）；页面与设置页里它在那一列的最上面、标题之前（M13/P4/S1）。

2. **9.2 右栏一段**（S1）：可收起的右栏、大纲读到的一节。

   旧文字：

   > - **右栏**（"页面信息"）：大纲、反链、属性、版本（M8：最近的版本，点开是版本页：阅读、与上一版或当前比较、还原）；锁的状态在中栏（M5）。

   换成：

   > - **右栏**（"页面信息"）：大纲、反链、属性、版本（M8：最近的版本，点开是版本页：阅读、与上一版或当前比较、还原）；锁的状态在中栏（M5）。宽屏（1280 px 起）上右栏在中间的右边，高同窗口、粘住、自己滚动，头一行的开关收起它（记在这个浏览器里，没记过时展开），收起时中间那一列仍居中、最宽 720 px（展开时 1280–1303 px 之间它窄于 720 px，收起之后变宽到 720 px）；更窄时排在中间之后，没有开关。大纲以琥珀竖线与 `aria-current` 标出读到的那一节：视口上三分之一处之上的最后一个标题，页能滚且到了底时是视口里的最后一个；关着的折叠 callout 里的标题照旧列出，不当读到的一节；点了一项之后先标它，直到读者再动或它出了视口。读到的那一节的项不在右栏的可见范围里时右栏滚到刚好露出它，读者在右栏里动过之后不再跟，到换页、或进出编辑之后大纲重新出现为止。长标题至多两行；编辑时没有大纲（M13/P4/S1）。

3. **13.2 第 17 条**（S1）："编辑"消失时的焦点。

   旧文字：

   > 焦点不在离开的控件里时不动它（第 26 条）。

   之后加：

   > 页头的"编辑"在页变得不能编辑（角色、来源或升级随事件变了）时拿着焦点消失，焦点交给页的标题，不滚动窗口（读者可能正读在页的中间；M13/P4/S1）。

4. **13.2 第 23 条的注册表**（S1）：补上 M12 的两个与 `propertyRoles`。

   旧文字：

   > M7 的附件 `assets`（最后一个）；

   换成：

   > M7 的附件 `assets`，M12 的过期地址重签之后的下载 `expiredDownloads` 与标题里的音视频不开合 callout 的 `titleMedia`（最后一个），M13 的属性条的角色 `propertyRoles`（在图之后、滚动区之前；只加 `role`，同滚动区）；

5. **13.2 第 24 条**（S1）：属性条。

   旧文字：

   > 这些都不纵向滚动（`[M6/P6](M6-links/06-P6-reading-view.md)`）。

   之后加：

   > 属性表经 CSS 排成属性条（M13/P4/S1，`reading.css`）：每个键值一个小块，列表的每项一个，自动折行，长的值在小块里折，平时不比显示的宽，不是区域；嵌套的表排成一段不折的行，宽过显示时外层属性表的滚动区（服务端只给外层的表包一层）照旧横向滚动，是名为"属性"的区域。表与列表改了 `display`，增强 `propertyRoles` 给它们补上 `table`、`rowgroup`、`row`、`rowheader`、`cell`、`list`、`listitem`（WebKit 加 VoiceOver 会丢掉改了 `display` 的表与列表的语义），清理时去掉。

6. **13.2 第 25 条的右栏**（S1）：读到的一节只量列出的标题。

   旧文字：

   > （`page-outline.tsx` 的 `listedUpTo`）；

   之后加：

   > 读到的一节只量列出的那些标题，每帧至多一次，右栏收起或不在正文旁边时不量（M13/P4/S1，`pages/page/current-heading.ts`）；

7. **13.2 第 26 条**（S1）：右栏只滚自己、让给读者。

   旧文字：

   > （与上面到达的看守不同：那里同一路径换了查询或片段的 `replace` 是新的导航）。

   之后加：

   > 右栏同样（M13/P4/S1，`pages/page/page-outline.tsx`）：大纲读到的那一节变了、它的项不在右栏的可见范围里时，只改右栏的 `scrollTop`，滚到刚好露出，不调 `scrollIntoView`，不移焦点；读者自大纲出现起在右栏里滚过、按过键或按下过（大纲的项除外）就不再跟，到换页、或进出编辑之后大纲重新出现为止；右栏不自己滚动时（更窄，排在正文之后）不动。

8. **头的表**（S1）：浏览器本地存储键。

   旧文字：

   > `nwiki.workspace`（最后访问的工作区，M2）；

   换成：

   > `nwiki.workspace`（最后访问的工作区，M2）、`nwiki.panel`（页的右栏收起与否，M13/P4/S1）；

9. **12.4 的扩展点**（S1）：阅读视图的交互增强一行补上 M12 的两个与 `propertyRoles`。

   旧文字：

   > 、M13（代码块的复制按钮）

   换成：

   > 、M12（过期的地址重签之后下载、标题里的音视频不开合 callout）、M13（属性条的读屏角色、代码块的复制按钮）

10. **13.4 第 3 条**（S1）：PG17 与 UP1。

    旧文字：

    > 接口与 MCP 版本由 N3 覆盖；`changeAccessWith` 选下拉。

    之后加（开头一个空格）：

    >  M13/P4/S1：PG17 在三种浏览器里核对页的头一行（谁、何时编辑过，名字在 `bdi` 里，不是 `status`；它晚到时"编辑"不动）、宽屏的右栏（与正文并排、高同窗口；点大纲的项与读到的一节，关着的折叠 callout 里的标题从不标为读到的一节，页末点的一项照旧标它；读者之上就地更新时读的那一行不动；右栏只滚到当前项刚好露出、窗口不动，读者在右栏里滚过之后不再跟；收起之后正文宽度不变、仍居中，刷新之后仍收起；1024 宽时没有开关、右栏在正文之后）与 375 宽时的属性条（折行、不成区域、`expectFits`），一格在 375 宽时核对名字长而无处可断（连字符接数字，Firefox 不在那里断行）时锁说明与版本页"时间 · 作者"一行照样折行、页不横向滚动，另一格在 375 与 820 宽时核对阅读、刚进编辑、打了字与自动保存之后标题都不动，只有页面版本：它只核对界面的行为，写的只有造数与一次经接口的写，接口与 MCP 版本豁免。UP1 另核对"编辑"消失时焦点到标题、读的那一段不动。

11. **第 15 节**（S1）：变更记录一行（`YYYY-MM-DD` 换成提交的日期）。

    旧文字：`## 15. 变更记录` 与其后的空行。

    之后加：

    > - YYYY-MM-DD：M13/P4/S1（依据 `[P4 设计](M13-ui-refresh/04-P4-reading.md)`）：头的表（`nwiki.panel`）；9.2 的中间与右栏两段（头一行、720 px 的一列、属性条、编辑说明、升级提示的位置、可收起的右栏、大纲读到的一节）；12.4（阅读视图的增强补上 M12 的两个与属性条的角色）；13.2 第 17 条（"编辑"消失时焦点到标题、不滚动窗口）、第 23 条（注册表补上 M12 的两个与 `propertyRoles`）、第 24 条（属性条）、第 25 条（读到的一节只量列出的标题）、第 26 条（右栏只滚自己、让给读者）；13.4 第 3 条（PG17、UP1）。

12. **13.2 第 23 条的注册表**（S2）：`codeBlocks`。

    旧文字：

    > M13 的属性条的角色 `propertyRoles`（在图之后、滚动区之前；只加 `role`，同滚动区）；

    换成：

    > M13 的代码块的头 `codeBlocks` 与属性条的角色 `propertyRoles`（都在图之后、滚动区之前；后者只加 `role`，同滚动区）；

13. **13.2 第 23 条的例外**（S2）：代码块的头。

    旧文字：

    > 清理不还原（接回的是旧元素，地址是它自己的，失败时就地重签）（`[M7/P4](M7-assets-transfer/04-P4-assets-web.md)` 第 4 节）。

    之后加：

    > M13 的代码块的头（`reading/code-blocks.ts` 的 `codeBlocks`，M13/P4/S2）是第三个例外：给代码块（`pre` 里只有一个 `code`、此外只有空白，mermaid 的除外；作者手写的几个 `code`、或文字夹着 `code` 的 `pre` 不加）的 `pre` 加上第一个子元素 `div.nw-code-head`：语言名（`code` 的 `language-` 类），有剪贴板时另有一个 `aria-live` 的说明与"复制"（只复制 `code` 的 `textContent`；成功时按钮的图标片刻换成 ✓、说明的"已复制"只给读屏，失败时"没能复制"看得见；按钮的名字不变；连按时只认最后一次，清理之后才到的答复不说）；两者都没有时不加；`position: sticky; left: 0`、`user-select: none`，打印时不印；放在 `pre` 里而不包一层，顶层块不变，代码块的区域名称照旧是"代码"；清理时移除。

14. **13.2 第 23 条的焦点键**（S2）。

    旧文字：

    > 音视频按附件，别的按 id；滚动区按它所在的块里的第几个）

    换成：

    > 音视频按附件，代码块的"复制"按它在块里的第几个（M13/P4/S2），别的按 id；滚动区按它所在的块里的第几个）

15. **13.2 第 23 条留着的块**（S2）：头随清理与重跑重建（审查 C 的 M9）。

    旧文字：

    > （公式与图由增强重写，其中的选区除外）

    换成：

    > （公式与图由增强重写，其中的选区除外；代码块的头随清理与重跑重建，拿着焦点的"复制"按键找回新的那一个，M13/P4/S2）

16. **13.2 第 32 条的例外**（S2）：mermaid 的 `base` 与 `reading.css` 的变量（审查 C 的 N4：870 在 S2 就把变量挪进 `:root`/`.dark`）。

    旧文字：

    > 例外：mermaid 的 `themeVariables` 只收十六进制，在 `reading/diagrams.ts` 里另写一份亮暗的色值；`editor/theme.ts` 的链接色、搜索匹配色与语法色，`reading.css` 的 hljs 与 callout 各色可以写色值（P4 起注释写明与 00-M13 4.1 的表同源）。

    换成：

    > 例外：mermaid 的 `themeVariables` 只收十六进制，在 `reading/diagrams.ts` 里另写一份亮暗的色值（M13/P4/S2 起是 `base` 主题加这两份与几条 `themeCSS`：灰阶的节点与线，系列用 callout 的七族，强调与库节点用琥珀；图自己的指令或 frontmatter 指定了不是 `base` 的主题、或给了自己的 `themeVariables` 时，只照它自己的配置画，不加这些；只给变量、没写主题的，主题按应用的亮暗取 mermaid 的 `dark` 或 `default`）；`editor/theme.ts` 的链接色、搜索匹配色与语法色可以写色值；`reading.css` 的 hljs 与 callout 各色、它在 `:root` 与 `.dark` 里给阅读视图（P4/S3 起也给编辑器）的链接下划线、代码块的底与高亮色可以写色值，注释写明与 00-M13 4.1 的表同源，两边声明的变量相同（`design-tokens.test.ts`，M13/P4/S2）。

17. **13.4 第 3 条**（S2）：PG17 的第二格。

    旧文字：

    > 另一格在 375 与 820 宽时核对阅读、刚进编辑、打了字与自动保存之后标题都不动，只有页面版本

    换成：

    > 另一格在 375 与 820 宽时核对阅读、刚进编辑、打了字与自动保存之后标题都不动，M13/P4/S2 起再一格核对代码块的头（语言名、"复制"进剪贴板而不带头的字、按钮片刻变成 ✓ 而说明只给读屏、代码横向滚动时头不动）、已完成的任务项（自己的字划掉、变淡，嵌套的列表与它下面的代码块、引用、callout、折叠的 callout 不划、不淡）、属性条里列表值是映射的小块（与列表的小块同高）、callout 的标题（加粗、链接、行内代码与长地址在一行文字里，375 宽时也不横滚）与 callout、列表项里块的间距，只有页面版本

18. **第 15 节**（S2）：变更记录一行，加在 S1 那一行之上。

    旧文字：`## 15. 变更记录` 与其后的空行。

    之后加：

    > - YYYY-MM-DD：M13/P4/S2（依据 `[P4 设计](M13-ui-refresh/04-P4-reading.md)`）：13.2 第 23 条（`codeBlocks`：注册表、第三个例外、"复制"的焦点键、留着的块里的头重建）、第 32 条（mermaid 的 `base`、图自己的主题与变量、`reading.css` 的 `:root`/`.dark` 与它们的成对）；13.4 第 3 条（PG17 的第二格）。

19. **9.3 第一项**（S3）：比例字体、等宽行与压淡的符号。新文字里的 `` `[文字](地址)` `` 是 Markdown 的写法，不是链接，写进总体设计时留着反引号（脚本只给目标是 `.md` 的链接去掉反引号）。

    旧文字：

    > - Markdown 语法高亮（标题加粗加大，符号保留原样）；

    换成：

    > - Markdown 语法高亮（标题加粗加大，符号保留原样、压淡：wiki 链接的 `[[`、`]]`、`#`、`|`（表格里是 `\|`；嵌入连同 `!`）与高亮的 `==` 也淡，引用里的标记同样淡；wiki 链接照服务端的读法认：前面有奇数个 `\` 的、中间另有方括号的、页名与小节去掉空白之后都空的不算，链接的标题里不认；只有页名（没有页名时是小节名）带下划线，不连两边的空白；`[文字](地址)` 只有文字带下划线；行内代码、代码块与 frontmatter 里不压淡）；正文用比例字体，代码块、表格与 frontmatter 的行用等宽，行内代码与代码行一样大（13.5 px），标题里的随标题放大，frontmatter 整块次要色、里面的行不当标题放大；等宽的行与压淡的标记只给可见范围加（M13/P4/S3，`editor/prose-lines.ts`、`editor/wiki-links.ts`）；对比与冲突视图用编辑器的等宽字体、13 px，不另压淡 wiki 链接与高亮；

20. **9.3 语法树的用处**（S3）。

    旧文字：

    > - lezer 的语法树只用于编辑辅助（高亮、补全时判断光标是否在代码里、列表续行），不做链接、标签的语义判断。

    换成：

    > - lezer 的语法树只用于编辑辅助（高亮、等宽的行、压淡标记时跳过代码、编辑时的大纲、补全时判断光标是否在代码里、列表续行），不做链接、标签的语义判断。

21. **9.3 快捷键一项**（S3）：之后一项"保存"。

    旧文字：

    > - 快捷键：`Ctrl+B` 包上 `**`，`Ctrl+K` 插入链接（编辑器之外它聚焦搜索框，9.2；M13/P2/S3），`Ctrl+S` 立即保存，`Ctrl+E` 切换阅读与编辑；

    换成：

    > - 快捷键：`Ctrl+B` 包上 `**`，`Ctrl+K` 插入链接（编辑器之外它聚焦搜索框，9.2；M13/P2/S3），`Ctrl+S` 立即保存，`Ctrl+E` 切换阅读与编辑；
    > - 已经自动保存（M5），"保存"只在有未保存的修改或保存失败时显出（失败时它就是重试）；它显着时焦点在头一行右边那一组里，它就不隐去（自动保存完成了也不，这时按它不做事），焦点离开那一组、又没有未保存的修改时才隐去；焦点走进那一组不让藏着的它显出；隐去时照旧占着位置、不可聚焦，头一行不变（M13/P4/S3）；

22. **9.2 右栏一段**（S3）：编辑时的大纲。

    旧文字：

    > 长标题至多两行；编辑时没有大纲（M13/P4/S1）。

    换成：

    > 长标题至多两行；编辑时的大纲来自编辑器的语法树（只去掉闭合的 frontmatter，还没闭合的照正文读，同服务端），项的文字同阅读时的大纲（行内的标记、地址、注释与脚注引用去掉，字符实体照它显示的字，图片取替代文字，wiki 链接取阅读视图显示的文字，行内代码里的照原样），项与阅读时一样高、指上去同样是手形；进编辑之后、编辑器第一次列出标题之前，右栏照旧是阅读时的大纲（不可点、不可聚焦）；点按跳到那一行，不标读到的一节（M13/P4/S1、S3）。

23. **13.2 第 17 条**（S3）："保存"不拿着焦点消失。

    旧文字：

    > 焦点交给页的标题，不滚动窗口（读者可能正读在页的中间；M13/P4/S1）。

    之后加：

    > 编辑时显着的"保存"在焦点在头一行右边那一组里时不隐去，焦点离开那一组、又没有未保存的修改时才隐去（占着位置、不可聚焦），所以它从不拿着焦点消失（M13/P4/S3）。

24. **13.2 第 25 条的右栏**（S3）：编辑时的大纲的上限。

    旧文字：

    > 右栏收起或不在正文旁边时不量（M13/P4/S1，`pages/page/current-heading.ts`）；

    之后加：

    > 编辑时的大纲同样至多 1,000 项、另说还有几项（之后的标题只数，不读它们的文字），进编辑时算一次，之后停手 300 毫秒才从语法树重算（只有标题的级别、文字或个数变了才重画），CodeMirror 没解析到的部分以 20 毫秒一片补完（`editor/edit-outline.ts`，M13/P4/S3）；

25. **13.2 第 32 条的例外**（S3）：编辑器不再写色值。

    旧文字：

    > ；`editor/theme.ts` 的链接色、搜索匹配色与语法色可以写色值；

    换成：

    > ；`editor/` 下的样子不写色值，只用 `styles.css` 与 `reading.css` 的变量：编辑器搜索的当前匹配是 `reading.css` 的 `--nw-match-selected`；CodeMirror 自带的查找面板、它的输入框与按钮、对比视图的 gutter 原是亮色的样子，编辑器的主题照应用的输入框与小号 outline 按钮另写，暗色下也读得清；编辑器的等宽字体是它自己的变量 `--nw-editor-mono`，行内代码的字号是 `--nw-editor-code`（正文里 0.871em、等宽的行里 1em，都是 13.5 px；标题里另乘标题的倍数 `--nw-heading`；对比与冲突视图不设，随它们的 13 px；M13/P4/S3）；

26. **第 15 节**（S3）：变更记录一行，加在 S2 那一行之上。

    旧文字：`## 15. 变更记录` 与其后的空行。

    之后加：

    > - 2026-11-25：M13/P4/S3（依据 `[P4 设计](M13-ui-refresh/04-P4-reading.md)`）：9.2 的右栏（编辑时的大纲：项的文字同阅读时，只去掉闭合的 frontmatter，编辑器列出标题之前照旧是阅读时的大纲）；9.3（比例字体与等宽行、压淡的符号与照服务端认的 wiki 链接、行内代码的字号、对比与冲突视图的等宽字体、"保存"只在需要时显出、语法树的用处、文档内查找的样子）；13.2 第 17 条（"保存"不拿着焦点消失）、第 25 条（编辑时的大纲的上限）、第 31 条（强制色下补全的选中项与查找面板的焦点）、第 32 条（编辑器不再写色值，查找面板与对比视图的 gutter 只用变量，编辑器自己的变量）。

27. **9.3 文档内查找一项**（S3）：查找面板的样子（决定 910）。

    旧文字：

    > - 文档内查找与替换。

    换成：

    > - 文档内查找与替换（CodeMirror 的查找面板：在编辑器的底上，输入框与按钮照应用的，亮暗两种主题都读得清；M13/P4/S3）。

28. **13.2 第 31 条的指示**（S3）：强制色下补全的选中项与查找面板的焦点（P4S3B-M4、决定 910）。

    旧文字：

    > 快速切换的选项与补全的选中项另有看得见的指示（内嵌的 2 px `ring-ring` 环），不只靠底色；

    换成：

    > 快速切换的选项与补全的选中项另有看得见的指示（内嵌的 2 px `ring-ring` 环），不只靠底色，强制色下补全的选中项同菜单项另有一圈内嵌的透明 2 px 外框；编辑器查找面板的输入框与按钮聚焦时同控件是 2 px 的环，强制色下另有透明的 2 px 外框（M13/P4/S3）；

不改的：9.1（技术栈照旧）；9.2 的左栏一段（P4 不碰左栏；细栏的位置文字照旧，决定 856）；13.2 第 12 条（编辑的状态照旧是一直挂载的 `<output>`；它前面的 ✓ 只在 9.2 的长期文字之外，写在 4.1.9 与代码的注释里）、第 16、18、20–22 条；第 31 条第 28 条之外的部分（编辑器聚焦时的 2 px 琥珀外框照旧；P4 加的控件都是 `Button` 或照全局的 `:focus-visible`，焦点照它）。

## 9. 交给后面的 Phase

- **P5**：
  - `main` 在 1280 px 起左右 40 px（决定 855）：P5 的页照这个留白排；
  - 升级横幅在页与设置页里进了内容列（决定 857、892），别的笔记本页（首页、版本页、回收站与预览、历史与变更集、目录、标签、健康、升级、导入导出）照旧画在 `main` 的全宽；P5 定了它们的布局之后，要进列的路由加 `handle: { column: true }` 并在列的最上面画 `<ColumnTop />`；横幅自己的样子（`rounded-md border-primary/40 bg-surface-2`）P5 看着改；
  - 版本页、回收站的预览与变更集页的面包屑随 `BreadcrumbTrail` 换了字号与颜色，它们的头（标题、按钮）还是旧的：P5 照 PG 页的头一行排（`HeadMeta` 与开关只属于页）；版本页的锁说明随 S1 换了样子（决定 868），在版本页旧的头之下；版本页与回收站的预览的正文随 `.nw-reading` 换了样子（S2）；
  - `Mark` 只有页的右栏用了 `current`（决定 863），版本页与历史页的 `Mark` 照旧；
  - 01-P1 第 9 节的标签组清单里剩下的（`asset-dialogs.tsx:221` 由 P4 改了，决定 882）；
  - 空状态（右栏的"没有反链""没有属性"照旧是一行灰字，P5 的空状态组件出来之后看要不要换）；
  - P3 的 `sectionAt`（`W/app/settings-directory.tsx:53`）精确比较路径：笔记本设置的地址带尾随斜杠（`…/settings/general/`）时路由匹配到"基本信息"，`sectionAt` 认不出，走 `shown === undefined` 那一支，页没有 `h1`、各组之间没有间距（S1 起那一支的横幅在设置的列里，决定 898）；P5 做其余页面时先核对工作区与个人设置是否同样，再定改法（审查 B 的 P4S1B-Q1）；
  - 版本页的标题在 320 宽时被"Restore this version"挤成"Version / 1"两行（M8 起的，不是 S1 改出来的；审查 B 修复核对第 2 轮）：P5 照页的头一行排版本页的头时一起看。
  - 右栏版本一行里自动生成的长名字（`admin-<hex>-<hex>-0-0`）在 Chromium、WebKit 1440 宽时拆出单独一行"-"（S1 的版本行，S3 的实施者看到）：UAX#14 不在"-"与其后的数字之间断，整串连成一个过长的词，只能紧急断，断点落在"-"之前；一行的改法（`break-all`、`line-break: anywhere`）会把普通名字从词中间切开，比现在差：P5 看名字怎样显示（截短或换行的规则）时一起看（审查 B 修复核对第 1 轮同意）。
- **各 Step 的实施者**（审查记录"未核实的怀疑"的去向）：S1 在 l6 的大页上量一次读到的一节每帧的代价，记进 10.1；S1 跑全部 vitest 三遍，看有没有"not configured to support act"（第 1 节 (k)；三修的写法下第一次设状态跟着别的更新同一次提交，审查 A、C 第 2 轮推了理、没跑），记进 10.1（6.1）；S1 在 Firefox 里看一次右栏复制属性的值的尾随空格，记进 10.1（决定 881）；S2 看换主题时控制台有没有 mermaid 的警告，记进 10.2；S3 在一篇几 MB 的文档上量一次编辑时的大纲补完解析的总时长，记进 10.3（第 6 节）；S2 在 7.4 的截图里看甘特的 `vert` 标记线与暗色"关键加完成"的条里的字（审查 B 第 2 轮）；S3 在 Safari 里人工看一次窗口失焦时显着的"保存"不隐去，记进 10.3（审查 A 第 2 轮，6.3）；各 Step 的 `make e2e` 里看 PG15 [webkit] 那一格是否仍"同行同消息"（P4 改了 `main` 的留白与两栏，PG15 的文件不改，审查 C 第 1 轮的怀疑），不是就停下查。S1 的四件都做了，记在 10.1（PG15 [webkit] 在 S1 的门禁里照旧同行同消息、有时过）。S3 的几 MB 文档量了，大文档的跳动看了，记在 10.3；Safari 那一件没有做，交负责人（下一条）。01-P1 第 9 节 P4 那一条的四项：三项在 S2、一项在 S3，各在它的文档提交里注明（第 6 节，审查 C 的 N6）。
- **负责人**：Safari 的人工核对（6.3 第 3 项，审查 A 第 2 轮的未核实一条，审查 C 的 P4S3C-Q1；实施者与审查者手里都没有 Safari，jsdom 里由 page-edit 的 `relatedTarget` 为 `null` 一格守）：在 Safari 里编辑一页，打了字，焦点在"保存"上，等自动保存完成，切到别的窗口再回来，"保存"仍显着、焦点仍在它上面。放进 M13 汇报的"还欠负责人的人工验收"。
- **收尾**：读屏清单"H. M13"一节加：右栏的开关（名字"页面信息"、`aria-expanded`，收起之后右栏不在可访问性树里，1280 px 以下没有开关）；大纲读到的一节（`aria-current="location"` 读作"当前位置"，随滚动变、不播报）；页头的"谁、何时编辑过"是普通文字（不播报），名字在 `bdi` 里；右栏版本一行的"·"是 CSS 生成的（`before:content`），各读屏读不读出它（审查 B 第 2 轮的未核实一条）；编辑时头一行的状态（`<output>`，读屏读文字、不读 ✓）、"保存"出现与隐去（显着时焦点在头一行右边那一组里就不隐去，自动保存完成之后按它不做事；焦点进组不让藏着的它显出；隐去时不在可访问性树里，决定 885、894、895）、"完成"；标题下的编辑说明；属性条在 VoiceOver 下照旧读作表（`propertyRoles`）；折叠 callout 的 `summary`（决定 901 之后是块，图标在行内、chevron 绝对定位）在 WebKit 加 VoiceOver 下认不认它的展开状态；代码块的"复制"（名字不变，按下之后按钮变成 ✓，经 `aria-live` 读到"已复制"或"没能复制"一次，"已复制"只给读屏，决定 890；记下各读屏是否另读了名字）；编辑时的大纲（按钮，按下之后焦点到编辑器、光标在那一行）；页头的"编辑过"一句是三个弹性项（决定 899），`innerText` 与选中复制都是三行，读屏按行读时会不会拆成三段（审查 A 的 P4S1A-F1-N1，没有核实）。README 与 `CHANGELOG.md` 0.1.0 的"网页的打磨"一条里"编辑时没有大纲"的已知限制删掉（00 号文档第 7 节"收尾"第 2 项）。
- **v0.2**：M12-polish 第 1 项里 P4 不认领的（决定 881，逐项核对过 M5-polish）：3（高亮的闪动）、4、6.4、8.3、10.3、11.3–11.5、11.6 的其余、11.9、12.1、12.2 的开合、12.5、12.6、12.7、12.9、12.10 的千分位、13.2、17.2a、17.3、18.3、18.4b、18.4c、18.7、20.4、21.1；第 6 项的勾选框一半；第 5、11 项里 P4 的页之外的 `bdi`；第 7 项的 Firefox 尾随空格（照实记进 10.1，不改）：照旧留着。另有：l5 量编辑器外框时取父元素链上的底（`e2e/fixtures/contrast.ts:112-116`），改为量元素自己的底不在 P4（决定 891）；页内查找会找到代码块头里的语言名与"复制"，复制之后的 1.5 秒里也找得到藏起的"已复制"（打印时不印，决定 869；审查 B 第 1 轮核对过选区与剪贴板都不含它们）；mermaid 的 `themeVariables` 与 `themeCSS` 照旧可被图自己的指令改（决定 871；指令给的 `themeCSS` 经 mermaid 的 `sanitizeCss`）。 S2 留下的两件：作者自己指定浅色主题（`forest`、`neutral`、`default`，或 `base` 加浅色的变量）的图在暗色应用里，节点自带底与字、读得清，直接画在图框底上的边、箭头与时序图的消息字是深色，画在暗色的图框上看不清（`base` 加 `primaryColor` 的边 1.21:1；M13 之前图框透明、页是暗的，也这样）：图框的底跟着作者的主题（浅色主题给白底），P4 不做（实施者 S2 修复报告第 7 节第 4 条，主持者定）；同一个顶层块里有几段代码时，在拿着焦点的那段之前增删一段，就地更新之后焦点落到别一段的"复制"上（决定 905，照实；键先认代码的文字、再认第几个是一种改法，审查 A 的 P4S2A-Q1）。S1 留下的两件：带锚点打开的页，视图第一次到时焦点给锚点、不看读者其间的动作（决定 900，`W/pages/page/view-anchors.ts:72-80`；13.2 第 26 条，异步的答复之后移动焦点要看读者动没动）；刚编辑过的页，头一行读作"<名字> 编辑于现在""Edited by <名字> now"（`page.editedBy` 加相对时间一直有的措辞，"刚刚编辑""just now"更顺，审查 B 的 P4S1B-F3），两件都记进 `docs/v0.2/handoffs/M12-polish.md`。 S3 留下的两件：Firefox 编辑时滚长文档报"scroll-linked positioning effect"（CodeMirror 在 scroll 事件里同步量、以 `window.scrollBy` 校正锚点；S3 之前就有，阅读时没有）；长文档进编辑之后第一次远跳，Chromium 里一帧移约 157 px（S3 之前约 59 px），是 CodeMirror 对没量过的行的高度估计在校正：它没有给单行估计高度的公开接口，S3 的校正幅度在对照的范围里（审查 A 在三种引擎里量过远跳与小步滚动）；两件都记进 M12-polish 的 S3 一行。

## 10. 结果

（S1、S2、S3 合并之后各写一节：提交、做完的、与设计不同之处、实测、审查记录。）

### 10.1 S1：页头、属性条与右栏（2026-11-24，合并 `715923dd`）

- **提交**：`f7eb1100`（头一行与"编辑过"、右栏粘住并可收起、大纲读到的一节，照 6.1 提交 1）、`717f5334`（属性条与 `propertyRoles`、编辑时头一行的状态与按钮、标题下的说明、锁说明，照 6.1 提交 2）；照实加的 `d29b68b0`（PG17 第 5 步把标题滚到窗口顶下 100 px、第 15 步读属性表的角色：变异 12e、t6 在 `717f5334` 上活下来之后补）；审查的修复 `ed95b2cf`（没有列出的标题时不量；`main` 的观察者一格、897 一格与中文那次的打开照它们说的写）、`2ec9ce68`（`.nw-reading` 自成绘制层；小块 12 px 的圆角、视图离标题 14 px、锁与句子一项、嵌套表的小块同高、强制色下"当前"的框）、`addcfa69`（决定 899 的"编辑过"、决定 898 的设置索引）；修复核对第 1 轮之后的 `19518390`（PG17 第 12 步先等锚点的标题拿到焦点）、`c6297488`（长名字在锁说明与版本页一行里折，`justify-end-safe`，读取中与读取失败时离标题 14 px；PG17 第 4 格）、`f698d37e`（读到的一节看视图、不看 `main`）、`286d01b7`（PG17 第 14 步进不带锚点的地址）；修复核对第 2 轮之后的 `e2fd05b0`（第 4 格的 `REQUIRED` 两行、版本页一行的不断行空格、注释里视图旁边变高的是子页与附件）；加本文档提交。[S1 的审查记录](reviews/P4-S1-head-strip-panel-review.md)。
- **基线**：不另跑（决定 883）。提交 1 的 `make e2e` 兼作核对：只败第 1 节那三格（se2:231、pg15:113、lw8:836，同行同消息）。之后 se2:231、lw8:836 每次都红，同行同消息；pg15:113 [webkit] 照旧有时过：实施者十次 `make e2e` 与四次合并之前的整条命令里红 6 次，审查 C 的三次门禁里红 2 次，红时都同行同消息（审查 C 的 P4S1C-N4）。
- **做完**：4.1.1–4.1.12（照实见 4.2 之前的一段）；决定 854–868、879（PG17 第 1、3、4 格；第 4 格是审查加的）、882、883、884 里 S1 的两个键、886、887、892、893、894 的 S1 一半（右边一组定宽与 `min-h-8`；"保存"不需要时 `invisible` 在 S3，审查 C 的 P4S1C-N6）、897–900；881 里 S1 的部分：M5-polish 12.10 的截断、第 6 项"编辑"的焦点、P4 的页上的 `bdi`（锁说明、右栏版本一行、新的"编辑过"）；01-P1 第 9 节的标签组一处（`asset-dialogs.tsx:221`，`717f5334`）。S3 的变异 108 的抓手在 S1 的第 3 格里：审查 C 在 S1 的代码上照它的意思做了一条（没有修改时卸下"保存"），红在第 3 格的 `:316`（"375: the status's right"）；S3 照旧要跑 108。
- **测试**：
  - vitest：5325（221 个文件）→ 5365（223，提交 1）→ 5374（224，提交 2；`d29b68b0` 不变）→ 5376（`ed95b2cf`：A-N1、897 各一格）→ 5379（`addcfa69`：899 中英两格、898 一格；之后不变，新的断言加在已有的格里）。
  - E2E：628 → 631（PG17 第 1 格 × 三种浏览器）→ 634（第 3 格 × 三）→ 637（第 4 格 × 三，`c6297488`）；`check.mjs` 列 chromium 430、firefox 102、webkit 102、webkit-narrow 3。
  - 先写的新格在没改的树上红：提交 1 的 vitest 22 格与两个新文件导入失败，PG17 第 1 格（三种浏览器红在没有"Edited by"）与 UP1 新加的 `toBeFocused`（三种）；提交 2 的 vitest 6 格与 `property-roles.test.ts`，PG17 第 15 步（区域一个）与第 3 格（"375: the status's right"右移约 71 px）（三种）；读锁说明三处 `toBeNull` 的探针（去掉锁的释放事件，三格红）。修复的：A-N1 一格（没改时挂了监听、观察了 `main`）、898 一格（第一次见到横幅时 `{column:false,title:false}`；e2e 里改前三种引擎都是先在全宽、再进列）、899 中英两格与 PG17 第 2 步（三种浏览器红在"how much of the sentence is cut"）、`justify-end-safe` 的断言、观察视图的一格（`[[main]]` 不等于 `[[article]]`）、PG17 第 4 格（只在 Firefox 红：先在页上，锁说明修好之后在版本页）、PG17 第 12、14 步（扣住视图的探针下改前红、改后绿）、版本页一行的不断行空格（t24、t25 下 5 格、1 格红）。
  - 不是先红的（审查 C 的 P4S1C-N3，7.1 的照实）：`upgrade-banner.test.tsx` 找不到的页、页树读取失败两格与"a version's page has the banner where it had it"，`page-panel.test.tsx` 的"folded away, or not beside the page, the right column measures none"，897 一格（现有的行为，t12 看到它红），原有的 `general-page.test.tsx:665-688`；hi1:258、hi2:113 是实现之后照新的次序改的。
  - 提交 1 之后全部 vitest 跑三遍（`heavy.sh make test-web`），都是 5365 格全绿，"not configured to support act"零次，约 60.5、55.7、56.3 s（第 1 节 (k)）；之后每次门禁也是零次。全部 vitest 照旧每次打出一次"Not implemented: navigation to another Document"：原有的某一格让 jsdom 去导航，P3 的门禁日志里也有，`setup.ts` 的控制台守卫看不见它（审查 C 的 P4S1C-N8 与未核实的怀疑，没查是哪一格）。
- **变异**（逐条在审查记录）：
  - 7.2 的 S1 各条除 20、35 都被它写的格抓住（1–19、21–30、32–34、36、70–81、102–106、104b、112–115），26 由 `check.mjs` 抓（列出 PG17 的格）；照实改了抓手的：12（第 5 步，12e 在 `d29b68b0` 之后红在 `:149`；之前第 4 步点过的标题被 `pin` 着，线在 0 时也读到它）、16（`scrollIntoView({ block: "nearest" })` 只有 vitest 的桩抓，PG17 只抓默认的写法，红在第 8 步，审查 C 的 P4S1C-N2）、18（`page-panel` 15 格；`current-heading` 的那一格先放视图才走到守卫，审查 C 的 P4S1C-M1，之后 2 格红）、31（等价，三种引擎；t10 改成 `normal` 时第 15 步红）、79 与 113（第 3 格先跑的 375 宽红在 `:288`；820 宽那一遍由审查 C 单独跑，也红在 `:288`，审查 C 的 P4S1C-N1；79 另红在第 1 格第 14 步）、106（决定 898 之后由 `upgrade-banner.test.tsx` 的一格抓）。
  - 20、35 没有格抓，由截图看：20 写在 `line-clamp-2` 之后时长标题由两行（35.75 px）变四行（71.5 px），写回原来的位置（类名最前）是等价的（`cn` 合并掉它）；35 让两行的失败说明把头一行撑高，标题下移 5.5 px（审查 B 在三种引擎里量过）。
  - 实施者另写的 t1–t25（7.2 表末）：t1、t8 等价，t16 在 e2e 里三种引擎等价、vitest 读类名的两格红（依据都在行里）；t6 由 `d29b68b0` 补格；t17、t18 只有 Firefox 的 PG17 第 4 格抓；t20 没有格抓（临时故事量出读取中、读取失败回到 10 px）；t22、t23 改的是故事，只在扣住视图的探针下红（守的是竞态）；其余都被抓住。
  - 审查者另写的：审查 C 的 108s1、31-ffwk、16-plain、18-ch、79-820、113-820、t16-ffwk，探针 3、4 配 t22、t23；审查 B 的 20、35、106 的截图对照；审查 A 的 l6 的十三种 CSS 对照。
- **视觉的核对**（7.4）：
  - 实施者先在 Chromium 截了 53 张（亮暗 × 1440、1280、1024、820、375 与整页；滚动之后大纲的当前项；1280、1440 收起；强制色；头一行四态（阅读、刚进编辑、打了字、已保存）；820、375 宽两行的失败说明；长面包屑、长标题；样稿的阅读亮暗、编辑、收起），对照样稿：头一行、32 px 的标题、属性条、右栏四节的图标与箭头、反链卡片、属性两列、版本一行"时间 · 谁 · 种类"与最右的"Current"、编辑时的"✓ Saved."、"Save"、开关、"Done"与带锁的说明都相符；不同的都是设计写明的（没有 +N，值是正文色，小节没有计数，编辑时没有菜单）。修复之后在三种引擎里重截：标题到属性条、小块、锁说明（页与版本页，1440、375、320）、强制色下的"当前"、设置索引的横幅、名字长的"编辑过"（中英文，1440、820、375）、大纲的长标题；修复核对第 1 轮之后改前改后的锁说明（固定的长名与像邮箱的名字，375、320）、320、340、375 宽的"编辑过"（30 分钟与 3 小时前，中英文）、三种状态离标题的距离（带属性条与不带，1440、375、320）。
  - 实量：820 宽两行的失败说明 `output` 32 px、组 32 px，头一行不变高；375 宽三行时头一行高出 16 px（照实，不截断）；320 宽英文 NUL 的失败四行、标题下移 32 px，"too slow"三行下移 16 px，三种引擎相同（审查 B 的 P4S1B-N3，不截断）；820、375 宽阅读与编辑都是两行，1024 起一行；1280 宽收起时列 720、居中；强制色下当前项是下划线；折叠 callout 里的标题列出、从不当前；长标题两行截断；375 宽时长值在小块里折；长面包屑在 1440 宽折 4 行（头一行 78 px）、1024 宽折 6 行（117 px），右边一组照旧 416 px，里面只有一句被截的"编辑过"：决定 894 为"行数只看宽度"付的代价，照现状（审查 B 的 P4S1B-Q4）。
  - 修复之后：标题到属性条（或第一段）在三种引擎、1440、1024、820、375、320、中英文都是 14 px，读取中、读取失败与文件夹的提示也是 14 px（改前 10 px）；单行的小块 24.75 px 高，折行的 43.5、62.25 px，字都在框里；嵌套表的小块与同一行的同高；"当前"18.5 px 高，Chromium、Firefox 的强制色下有框；"编辑过"在 1440 到 320 都不溢出，名字的 `bdi` 宽 174、210、121、66 px（英文），时间完整，但 `innerText` 与选中复制成了三行（"Edited by \n<名字>\n yesterday"，三种引擎都这样，审查 A 的 P4S1A-F1-N1，决定 899 的代价）；320 宽、能编辑的账户、"10–59 minutes ago"时名字是 0 宽、看不见（省略号也画不出），改前"Edited by"的开头被裁 0.7 px（Chromium、WebKit）、0.92 px（Firefox），`justify-end-safe` 之后开头不裁、裁的是结尾的同样的量；340 宽以上与"N hours ago""yesterday"时名字至少留 4–33 px，中文不受影响（审查 A 的 P4S1A-F1-N2）；锁说明的长名字在 Firefox 里改前冲出 283、338 px（固定的长名，375、320 宽）与 56、111 px（像邮箱的名字），改后 0，Chromium、WebKit 改前改后都是 0；审查 B 试了六种名字（短的、带空格与连字符的、连字符接数字的、像邮箱的、60 个字母的长词、中文的长名字），页与版本页，1440、375、320 宽，浅色深色，三种引擎都不横向滚动；版本页那一行在 1440、820、375、320 宽、中英文都不溢出，改成不断行空格之后"·"跟着名字走（B 的静态探针三种引擎都对）。
  - 截图在会话的临时目录，不进仓库。
- **Firefox 复制**（决定 881）：照 AS6 的 `copied` 复制右栏属性的值，Firefox 得 `{"status":"draft","owner":"Hub","owner (link)":"Hub"}`，与 Chromium 相同；属性换成两列的网格之后，M12 记下的尾随空格复现不出（没改代码）。
- **l6 的大页**（14 万个标题，列出 1,000 个；Chromium，1440×900，每帧滚 40 px、120 帧）：钩子量一次 1,000 个标题中位 0.4 ms、最大 1 ms，脚本每帧 1.3–2.1 ms：慢的不是量位置。右栏开着从页顶滚时帧间隔中位 50 ms（约 20 fps）：审查 A 找到原因，右栏的每次改动都让根绘制层重画、走遍正文的每一块（每次约 36 ms），当前项每帧都变（P4S1A-M1）。`.nw-reading` 加 `isolation: isolate` 之后（`2ec9ce68`），实施者在同一会话里依次量改后、改前（只去掉它）、再改后（Paint 取渲染主线程的 Paint 事件、合并嵌套的区间）：l6 开着，改前帧间隔中位 50.0 ms、p90 50.1 ms、超过 20 ms 的帧 73、每帧 Paint 22.4 ms，改后 16.7 ms、16.7 ms、2–3 帧、11.0–11.3 ms；l6 收起，改前 16.7 ms、16.8 ms、9 帧、2.6 ms，改后 16.7 ms、16.7 ms、0 帧、1.2 ms；普通的页（40 节、每节 8 段）开着与收起，改前改后都是 16.7 ms、0 帧，Paint 0.01–0.03 ms。再展开的那一次与开着相同。改后 l6 开着时 Paint 仍约 11 ms/帧（74 次重画），帧率已是 60 fps，与审查 A 用注入的 CSS 量的一致，原因没再拆。审查 B 以 isolation 开与关逐像素比七个场景（KaTeX 与 mermaid、视图左边的焦点圈、375 宽公式区域的焦点圈、页面菜单、粘住的右栏、375 宽的视图与抽屉）：Firefox、WebKit 相同，Chromium 最大差 3/255，只是栅格化。
- **读到的一节的观察者**（`f698d37e`，4.2 之前的"照实"）：WebKit 的"undelivered notifications"在改前 120 次载入里报 23 次，只拦掉 `main` 的观察 0 次，改看视图之后 0 次；审查 B 之后两次运行里没有再报。代价，审查 A 在三种引擎里量了（1440×900，40 节的页，在视图外插 200 或 900 px 的块）：页顶（`scrollY` 0）视图之上插 200 px，过时：仍标"Part 01"，按线该是没有，0 次 `scroll`（页顶不做滚动锚定），窗口再滚 1 px 就对；页中视图之上插 200 px，对（滚动锚定把窗口挪 200 px、发一次 `scroll`）；页底视图之后追加 900 px（不再到底），过时：仍标"Part 40"，按线该是"Part 38"，直到下一次滚动或改窗口；视图里变高（就地更新），对。三种引擎相同。只在页顶或页底、视图外的东西变高时过时一阵，下一次滚动或改窗口就纠正，照实记下（主持者定，不补 `MutationObserver`）。
- **PG17 的时长**（7.5，第 1 / 3 / 4 格）：实施者合并之前的整条命令，`d29b68b0` 上 Chromium 2.1 / 7.3 s、Firefox 5.4 / 7.5 s、WebKit 3.4 / 6.9 s；`addcfa69` 上 1.9 / 7.1、4.2 / 6.9、3.0 / 7.2 s；`286d01b7` 上 2.3 / 6.5 / 0.53、4.5 / 6.6 / 0.84、2.9 / 7.1 / 0.78 s；`e2fd05b0` 上 2.1 / 7.6 / 1.3、7.1 / 7.9 / 1.4、3.7 / 7.7 / 3.0 s。审查 C：`d29b68b0` 上 2.2 / 6.9、5.6 / 7.4、3.6 / 7.7 s；`addcfa69` 上 11.8（红，F1-I1）/ 7.1、4.7 / 6.8、2.9 / 6.9 s；`286d01b7` 上 2.5 / 6.7 / 0.5、4.8 / 7.0 / 1.0、3.5 / 6.8 / 0.9 s。S1 的三格合计约 32–42 test-seconds（7.5 推的是 PG17 三格 45–72，第 2 格在 S2；第 1 格多半比推的 6–10 s 快）；本机 E2E 2.6–3.8 min（负载 10–62），7.5 推的是 2.8–3.0。UP1 的页面格 1.2–3.3 s。
- **门禁**：
  - 每个提交的 `heavy.sh make lint-web knip test-web` 都绿（提交之前没过的，改了才提交：提交 1 第一次 7 格红，6 格是 StrictMode 下"离开编辑之后焦点在'编辑'上"、1 格是"the wait's listeners"，4.2 之前的照实与 7.1；`ed95b2cf` 第一次 oxlint 报 `no-shadow`；`addcfa69` 前两次 oxlint 报 `no-array-index-key`、新格的 `MutationObserver` 以 `queryByText` 找横幅、通用页的行里也有那句话；`c6297488` 的第 4 格 oxlint 报 `consistent-function-scoping`、`no-shadow` 与 `no-await-in-loop`，改成模块级的 `unbroken()` 与 `fitsAt`）。
  - `CI=1 heavy.sh make e2e`：提交 1 第一次 623 过、7 败（基线三格外，hi1:258、hi2:113 的文字次序，PG17 第 1 格在 Chromium 差 −0.375 px、在 WebKit 差 −0.078 px：照实改 hi1、hi2，加 `Math.ceil` 与 0.5 px 的容差），之后 627 / 3 / 1，2.7 min；`717f5334` 631 / 2 / 1，2.7 min；`ed95b2cf`、`2ec9ce68`、`addcfa69` 630 / 3 / 1，2.6–2.7 min；`19518390` 631 / 2 / 1，3.6 min；`c6297488` 632 / 4 / 1，3.1 min（基线之外 lw8:443、lw9:60 [chromium]）；`f698d37e` 632 / 4 / 1，2.9 min（基线之外 PG17 第 1 格 `:278` [webkit]，即 `286d01b7` 修的竞态，与 W6:128 [chromium]）；`286d01b7` 633 / 3 / 1，2.7 min；`e2fd05b0` 634 / 2 / 1，3.2 min（负载 40.3 → 33.1）。lw8:443（建页）、lw9:60（建笔记本）、W6:128（接受邀请）都是造数的 API 在负载（约 33）下答 500 `internal_error`，单独重跑都绿。
  - 合并之前的整条命令（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`，`m13-ui-refresh` 一直是 `92cbffd9`）：`d29b68b0` 上 vitest 224 个文件、5374 格，E2E 631 / 2 / 1，3.1 min，整条 4 分 40 秒（负载 20.8 → 24.7）；`addcfa69` 上 5379 格，631 / 2 / 1，2.6 min，3 分 45 秒（10.0 → 31.2）；`286d01b7` 上 5379 格，634 / 2 / 1，2.6 min，3 分 47 秒（11.8 → 39.2）；`e2fd05b0` 上 5379 格（82.5 s），633 过、3 败（基线三格，同行同消息）、1 跳，3.8 min，5 分 24 秒（33.1 → 41.4）。每次"not configured to support act"0 次，受保护路径的 `git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 与 `git diff --check` 都以 0 退出，跑完 `git status` 为空。
  - 审查 C 的门禁：`d29b68b0` 上 630 / 3 / 1，3.1 min；`addcfa69` 上 629 / 4 / 1，2.7 min（基线三格与 PG17 `:242`，即 F1-I1，负载 19 → 62）；`286d01b7` 上 632 / 4 / 1，3.0 min（基线两格与 LW6 `lw6:151`、LW9 `fixtures/pages.ts:169`，服务端日志都是 `commit transaction: timeout: context deadline exceeded`，属造数的 500；重跑这两个文件 12 过）。
- **与设计不同之处**：照实多九个代码提交（`d29b68b0` 与八个修复提交）；4.2 之前的"照实"（4.1.1、4.1.2、4.1.3、4.1.4、4.1.6、4.1.7、4.1.8、4.1.10）；861、879、886 的文字；5.1（第 2、3、5、12、14、15 步，第 4 格，`REQUIRED`，UP1 的 `.first()`，hi1、hi2）、5.4、5.5；7.1（`current-heading` 的守卫、观察者与没有标题时，点按一格的"Not implemented"，决定 892 那一支的格，897 的格，S1 末的照实一段）；7.2（第 12、16、18、20、31、79、106、113 行，t1–t25，表后的"不列的"）；7.4 的"编辑过"；7.5、8.1 第 1 条；8.2 第 2、7、10 条；第 9 节；决定 897–900。
- **交给后面**：第 9 节（B 的尾随斜杠与版本页标题在 320 宽的两行给 P5；决定 900 与"编辑于现在"的措辞给 v0.2；读屏清单加"编辑过"的三段）；S3：右栏的读者记录随大纲的列表（决定 897），给编辑时加大纲时再核对；变异 108 照旧要跑；"保存"的显出与编辑时的大纲接在 `EditHeadSlots` 的 before、after 上。

### 10.2 S2：正文的样式、代码块的头与图（2026-11-24 至 25，合并 `cea66d69`）

- **提交**：`96346ed3`（正文的字、标题、链接、表格、callout、任务项，hljs 与 mermaid 的颜色，照 6.2 提交 1）、`1c170cba`（代码块的头与"复制"，照 6.2 提交 2）；照实加的 `cdedff02`（7.4 的强制色下的一张图找到的：图里的 `foreignObject` 另不强制，L4 的页面格加一步）；审查的修复 `b24c98df`（决定 901：callout 的标题是一行文字）、`8dfb516b`（决定 902、903：callout 与列表项里块的间距）、`36eb0c5d`（决定 904：图自己指定的主题只照它画）、`bbd5b46e`（时间线的箭头头与线同色，"复制"照样稿的大小）、`0e35d0dc`（清理之后晚到的答复不说，几个 `code` 的 `pre` 不加头，`reading-view.test.tsx` 拆掉剪贴板的替身）；修复之后实施者交来、主持者定的 `d43b3e6f`（流程图节点 9 px 的 padding，改过的 904：图自己给了变量时也只照它画）、`9685e300`（已完成项下面的块不划、不淡）、`02cb019e`（已完成项里 callout 的标签一句）；修复核对第 1 轮之后的 `5bbf897f`（决定 906，没有指令与 frontmatter 时不解析，文字夹着 `code` 的 `pre` 不加头）、`ade3abd1`（903 的 `:is()` 去掉 `table` 与属性条一步，已完成项下面的引用与折叠的 callout，第 7 步的行数取上限）；加本文档提交。[S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)。
- **基线**：不另跑（决定 883）。提交 1、2 的 `make e2e` 只败第 1 节那三格（se2:231、pg15:113、lw8:836，同行同消息）。之后 se2:231、lw8:836 每次都红，同行同消息；pg15:113 [webkit] 照旧有时过（实施者到 `5bbf897f` 为止的十二次 `make e2e` 与三次合并之前的整条命令里过了 3 次，之后的见"门禁"；审查 C 的两次门禁里过了 1 次）。另有的红都查清了，见"门禁"。
- **做完**：4.2.1–4.2.7（照实见 4.3 之前的一段）；决定 869–873、879 的 S2 一半（PG17 第 2 格，`REQUIRED` 里 firefox、webkit 各一行）、880 与 891 的 S2 一半（l5 照旧量，值不变）、884 的三个键、888、890、901–906；01-P1 第 9 节 P4 那一条的三项（阅读视图的其余变量、已勾的禁用任务项、`reading.css` 写死的色值的"同源"注释，决定 870、872）。代码块的头不添闪动（M12-polish 的"处理进展"）：就地更新时 `useServerHTML` 的 layout effect 在同一次同步执行里依次 `was.undo()`、`patchBlocks`、`enhance(...)`（`W/pages/page/server-html.ts:88-130`），`codeBlocks` 是同步的，头移除又建回之间浏览器画不了一帧，`.nw-code-copy` 也没有过渡（审查 C 读代码核对，P4S2C-M2）。
- **测试**：
  - vitest：5379（224 个文件，S2 之前）→ 5381（224，提交 1：`diagrams.test.ts` 新一格，三格以 `config.theme` 区分的断言改成 `base` 与 `themeVariables.background`；`design-tokens.test.ts` 成对一格）→ 5395（225，提交 2：`code-blocks.test.ts` 8 格，`patch.test.ts` 1 格，`reading-view.test.tsx` 2 格，`messages.test.ts` 随三个新键 3 格）→ 5397（`36eb0c5d`：决定 904 两格）→ 5399（`0e35d0dc`：`code-blocks.test.ts` 两格）→ 5400（`d43b3e6f`：只有变量一格）→ 5402（`5bbf897f`：没有指令与 frontmatter 一格，文字夹着 `code` 一格；`ade3abd1` 不变）。
  - E2E：637 → 640（PG17 第 2 格 × 三种浏览器；之后不变，新的步加在这一格里）；`check.mjs` 列 chromium 431、firefox 103、webkit 103、webkit-narrow 3。pg5 的页面格加整块选中复制（Chromium），L4 的页面格加强制色一步（Chromium）。
  - 先写的新格在没改的树上红：提交 1 的 vitest 5 格（diagrams 4、design-tokens 1）；提交 2 的 patch 1 格、reading-view 2 格与 `code-blocks.test.ts` 的导入失败（放一个空的 `codeBlocks` 桩时七格全红；第 8 格在桩上必红，推得，审查 C 的 P4S2C-N2），PG17 第 2 格三种浏览器红在没有头，PG5 (page)（`pg5:88`）红在剪贴板；`cdedff02` 的 L4 红在 `l4:205`（`rgb(255, 255, 255)`，应为 `rgb(232, 232, 234)`）。修复的：决定 901 一步三种浏览器红在 `innerText` 拆成多行；902、903 一步三种红（4、4 px 与 0 px）；决定 904 两格（`expected 'base' to be 'dark'`，没有解析过）；A-N1、A-N2 两格；`d43b3e6f` 的三格；`9685e300` 的一步三种浏览器红在块的 `display: block`、颜色不对；`02cb019e` 的一句在变异 t35 下红；`5bbf897f` 的三格（主题 `undefined`、没有指令的图被解析、头在）；`ade3abd1` 的属性条一步在带 `table` 的规则下三种浏览器红（32.75 对 24.75 px），已完成项的块一步在审查 C 的 c1（t41）之下三种浏览器红在引用与 `details` 的 `display: block`，t14 在行数的上限之下三种浏览器照旧红在文字。7.1 的"挪到最外层、只写了 `:root` 时也红"由 t13 看到（`expected 0 to be greater than or equal to 20`）。`reading-view.test.tsx` 的 `afterEach` 由临时探针格看到它要拆的次序依赖（先红、改后绿，探针没有提交）。
  - 每次门禁"not configured to support act"0 次；全部 vitest 照旧打一次"Not implemented: navigation to another Document"（S1 已记）。
- **变异**（逐条在审查记录）：
  - 7.2 的 S2 各条（37–53、82–90、107 的五项）除 52（等价，依据在它那一行）都被它写的格抓住；46 的 l4 照设计不抓（`code-blocks.test.ts` 抓）；53 由 `check.mjs` 抓（firefox、webkit 两行）。修复之后的格数（`ade3abd1` 上）：44 是 2 格、48 是 5 格、49 是 5 格。
  - 实施者另写的 S2 的 t1–t43（7.2 表末）：t26、t27 没有格抓（t26 由临时探针、t27 由截图看）；t5 只有 vitest 抓（11 格）；t11、t12 由 L4 的强制色一步抓；t31、t33 三种浏览器都红；其余都被抓住。修复核对第 1 轮之后：照新代码改了写法的 10、18、21、23、30、46 与照行数上限重跑的 t14 都红（t14 三种浏览器红在文字）；新的 t36–t43 都被抓住（t40、t41、t43 三种浏览器）；顺带重跑的 37、38、41–44、47–49、83–87、89、90、107 的五项、t1–t7、t15–t17、t19、t20、t22、t24、t25、t28、t29、t31–t35 与 40、50 都照旧红，格数变了的：48 是 5 格（多了没有指令一格）、t5 是 11 格、t19、t22 各 2 格、t21 3 格。
  - 审查者另写的：审查 C 第一轮的 30 条（有几条故意与实施者的改法不同，都照报告；38e 三种浏览器都红），修复核对第 1 轮的 15 条与它自己的 c1（去掉 `blockquote, details`，PG17 照绿，成了 P4S2C-F1-M1，之后是 t41）；审查 A 的探针（真的 mermaid 的 `parse`、`only-child` 与 `undone`、计时）；审查 B 的截图对照（flex 的标题、时间线的箭头、`flowchart.padding`、903 落进属性条）。
- **视觉的核对**（7.4）：
  - 实施者在三种引擎里截了：亮暗；1440、1024、375、320；强制色（Chromium、Firefox）；中英文；正文与标题接标题、链接与悬停、没建的页、标签、高亮、行内代码、表格、引用、列表、任务项（可用的与版本页里不可用的、嵌套的、下面有块的）、七族的 callout 与折叠的箭头、带标记与长地址的标题、callout 里的代码块、代码块（几种语言、长行横滚、头、✓ 与"没能复制"、强制色、打印）、mermaid 十四种亮暗（修复之后各重拍一遍）、作者自己的主题与变量的图（两种应用主题）、375 宽的图、强制色下的图、样稿亮暗。三种引擎量出的值相同（除注明的）。
  - 实量：正文 16 / 28.8 px，块间 18 px；h1、h2 在段落后 38 px，h3 26 px，段落在 h1、h2 后 14 px、在 h3 后 10 px；h3 在 h2 后 26 px，h2 在 h3、h2 后 38 px；标题 24 / 21 / 17 px；表格各行 36 / 42.75 px，与样稿相同；`pre` 13 px、行高 22.1 px；头 34 px，长行横滚 200 px 时头离 `pre` 的左边 1 px；"复制"的图标 16 px、`gap` 6 px、500，英文按钮 67.86 px（样稿中文 64 px，差在字）；callout 正文 15 px、标题 14 px、单行高 21 px（Chromium 带行内代码的 21.64 px），callout 高 76.5 px、标题到正文 25 px，与样稿相同；callout 里标题之后 4 px、块间 8 px；松散项的第二段与紧凑项下的代码块离项的那一行 8 px，嵌套列表 4 px；长地址的标题在 375 宽 Chromium、WebKit 4 行、Firefox 3 行，320 宽都是 4 行，视图不横滚；流程图"浏览器"节点 Chromium 76.49×38.24、Firefox 77.52×39.27、WebKit 76.17×38.24 px（样稿 74.5×40.25），图框高 129.3 px（样稿 94.25，偏离）；375 宽时图的字饼 9.2、git 7.4、思维导图 7、时间线 3.9、xychart 8.1、甘特 2、看板 6.1、流程 7.3（`d43b3e6f` 的 padding 9 之后；之前 6.5）、时序 10.1、旅程 4.3 px，状态、类、ER、块图不缩（13.5–14 px，横滚）（今天就这样，照实）。
  - 已完成项与同样内容的没完成项（代码块、宽代码块、callout、折叠的 callout、图、表格、引用、嵌套的项；松散的一对）：三种引擎、亮暗、1440 与 375 宽，高度、各块的位置与大小都相同；宽代码块横滚 200 px 时头仍贴左边；"复制"复制出代码；折叠的开合照旧；块里的链接与标签是它们自己的颜色。从项的文字选进代码块复制，与没完成的项相同，只有 Chromium 在换行前多一个空格（`"…a link. \nmake ship"` 对 `"…a link.\nmake wait"`；审查 A 的探针同样）；WebKit 不再把项那一行与块之间的 8 px 画成选中色。
  - 作者的颜色（决定 904、906）：节点名对它的底，作者的 `dark` 10.17、`forest` 15.15、frontmatter 的 `default` 10.83、`base` 加 `primaryColor: #ffcc00` 8.36、只有变量（亮色应用）10.83，只写 `base` 与没写的照应用（亮 15.99、暗 12.35）；只改字号的指令（`fontSize: "20px"`）在暗色应用里是 mermaid 的 `dark`：节点 `#1f2020` 上 `#ccc` 的字 10.17、边与箭头 `lightgrey` 对图框 `#1f2023` 10.88:1，字 20 px；给了浅色 `primaryColor`（`#fff4dd`）的相同（mermaid 的 `dark`、`default` 节点的底取它自己的 `mainBkg`，不取 `primaryColor`）；亮色应用里两种都是 `default`，`#ececff` 上 `#333` 10.83、边对白底 12.63:1；`base` 加变量的在暗色应用里边与箭头 `#0b0b0b` 对图框 1.21:1（决定 904，与第 9 节作者的浅色主题同类）；三种引擎相同，两种应用主题里作者的主题都是同一套颜色，控制台没有输出。
  - 看过、照设计：甘特的 `vert` 标记线与今天线是琥珀，暗色"关键加完成"的条里的字看得见（第 9 节的未核实一条）；饼图多于七块时颜色重复、Go 的 hljs（审查 B 的 N5）、划线从复选框后的空格起（N1）、原生复选框（决定 872）是设计记的偏离；版本页里禁用的复选框 Chromium、Firefox 灰，WebKit 淡琥珀；作者以 `style` 着色的圆柱照作者的颜色，暗色下其上的浅色字难读（作者给的，照旧）；时间线的箭头头与线同色（亮 `#91939b`、暗 `#74767e`），旅程图的主线照旧是黑的。
  - 强制色下的图：`cdedff02` 之前 Chromium 的暗色高对比加亮色应用时，图的底与节点照应用画，流程图的节点名却是白的；之后是 `rgb(24, 25, 28)`，14 种图里每个元素的颜色与不强制时相同（审查 B 逐元素比过）。Firefox 改前就看得见；WebKit 模拟不了强制色。
  - 控制台：三种引擎加载与换主题（亮→暗→亮）都没有警告与错误（第 9 节要 S2 记的"换主题时 mermaid 的警告"：没有）。审查 B 的一次 WebKit 运行里有过一条"mermaid could not be loaded… Importing a module script failed"，单独重跑三次都绿：05-lessons 记过的被下一个文档打断的 `import()`（那一格在 mermaid 载入时 `reload` 过），没有逐次排查。
  - 整块选中代码块复制，三种引擎的剪贴板里都只有代码；`window.find("Copy")` 在 Firefox 为真，Chromium、WebKit 为假（真的查找栏没有试）。
  - 截图在会话的临时目录，不进仓库。
- **对比度**（l5，决定 880、891）：提交 1 与 `cdedff02` 的二进制上相同，也与之前相同：亮 补全项说明 5.281、编辑器外框 3.098、选中项 2.646、阅读视图链接的焦点 3.098、菜单的当前项 2.646；暗 4.849、7.923、5.476、7.923、5.476。下限不改；修复没有改 l5 量的东西，没有重量。
- **PG17 的时长**（7.5，第 2 格，Chromium / Firefox / WebKit）：实施者 2.2–2.4 / 2.6–3.2 / 2.5–2.8 s（`ade3abd1` 的 `make e2e` 2.5 / 2.7 / 2.7 s）；审查 C 2.3 / 2.5 / 2.8 s（`cdedff02`）、2.4 / 2.8 / 2.7 s（`02cb019e`）。推的是 2–4 s。L4 页面格 0.6–0.8 s，PG5 页面格 0.5–0.6 s；本机 E2E 2.6–2.8 min（负载 8–68）。
- **门禁**：
  - 每个提交的 `heavy.sh make lint-web knip test-web` 都绿（提交之前没过的，改了才提交：提交 2 的 tsc 报 `ByRoleOptions` 没有 `exact`，oxlint 报 `consistent-function-scoping` 与 PG17 的 `no-shadow`；`8dfb516b` 的 `consistent-function-scoping`；`36eb0c5d` 的 oxfmt；`d43b3e6f` 的 `no-shadow`；`9685e300` 的 `no-shadow`；`5bbf897f` 的 tsc（`textContent` 可能是 `null`）与 `no-await-in-loop`）。
  - `CI=1 heavy.sh make e2e`：提交 1 633 / 3 / 1，2.7 min（负载 19 → 28）；提交 2 636 / 3 / 1，2.7 min（16 → 26）；`cdedff02` 637 / 2 / 1，2.6 min（20.8 → 32.1）；`b24c98df` 637 / 2 / 1；`8dfb516b` 636 / 3 / 1；`36eb0c5d` 634 / 5 / 1（基线之外 pg15:136 [webkit] 与 lw4:592 [firefox] 造数答 500、a15:197 [webkit] 标题 10 s 内没出现，三个文件在 Firefox、WebKit 重跑 54 过，只败基线的 pg15:113）；`bbd5b46e` 635 / 4 / 1（基线之外 as6:167 [chromium] "Response has been disposed"，已知的偶发，重跑 3 过）；`0e35d0dc` 637 / 2 / 1；`d43b3e6f` 635 / 4 / 1（as6:167 同上，重跑 3 过）；`9685e300` 636 / 3 / 1；`02cb019e` 634 / 5 / 1（基线之外 lw13:112、lw12:126 [webkit]，造数与降级的 API 在负载约 33 下答 500，单独重跑都绿）；`5bbf897f` 636 / 3 / 1，2.7 min（负载 58 → 29）；`ade3abd1` 635 / 4 / 1（基线三格与 as6:167 [chromium] "Response has been disposed"，已知的偶发，重跑 3 过），2.6 min，整条 161 s，负载 25 → 26。各次 2.6–2.8 min。
  - 合并之前的整条命令（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`，`m13-ui-refresh` 一直是 `4f5552f2`）：`cdedff02` 上 vitest 225 个文件、5395 格（60.6 s），E2E 636 / 3 / 1，2.8 min，整条 3 分 58 秒（负载 10.4 → 26.0）；`0e35d0dc` 上 5399 格（58.5 s），636 / 3 / 1，2.7 min，3 分 52 秒（10.0 → 26.7）；`02cb019e` 上 5400 格（55.9 s），634 / 5 / 1，2.8 min，3 分 58 秒（16.6 → 31.9；基线之外 lw4:349、lw8:692 [firefox]，单独重跑都绿）；`ade3abd1` 上以 2 退出，只因基线三格（se2:231、pg15:113 [webkit]、lw8:836 [webkit]）：vitest 225 个文件、5402 格（58.3 s），E2E 636 / 3 / 1，2.6 min，PG17 第 2 格 2.3 / 2.8 / 2.7 s，整条 3 分 48 秒（实施者的记录：`premerge-4.log` 没有时间戳；负载 14.8 → 27.5）；每次"not configured to support act"0 次，受保护路径的 `git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 与 `git diff --check` 都以 0 退出，跑完 `git status` 为空。
  - 审查 C 的门禁：`cdedff02` 上合并之前的整条命令 636 / 3 / 1（基线三格），2.7 min，整条 3 分 59 秒（负载 7.8 → 45.1），vitest 5395 格（58.0 s），逐个提交的 `make lint-web knip test-web` `96346ed3` 5381 格、`1c170cba` 5395 格；`02cb019e` 上 637 / 2 / 1（se2:231、lw8:836；pg15 过了），2.7 min，整条 4 分 07 秒（负载 15.5 → 67.5，别的审查者也在跑），vitest 5400 格（63.7 s）。
- **与设计不同之处**：照实多十一个代码提交（`cdedff02` 与十个修复提交）；4.3 之前的"照实"（4.2.1、4.2.4–4.2.7）；869、871、872 的文字；4.2.4、4.2.5、4.2.7 的代码；5.2（PG17 第 2 格的标题、造数与第 5–8 步，L4 挪到"改的"，L4 的行号）；6.2；7.1（S2 的照实）；7.2（第 46、49、52 行，S2 的 t1–t43，表后的一句）；7.3、7.5；8.2 第 13、16、17、18 条；第 9 节；决定 901–906。偏离：流程图的图框高 129.3 px（样稿 94.25）；Chromium 里带行内代码的 callout 标题高 21.64 px；标题里的链接用正文色（同正文的链接，不用族色）；决定 903 让以代码块开头的列表项在代码块上方多 8 px；属性条里列表的列表（`lists: [[a, b], [c]]`）的小块多 4 px，是 S2 之前就有的 `li > :is(ul, ol)` 一条（审查 B 修复核对第 1 轮）；状态图的注也用 `flowchart.padding`，变小一点（框高 428 → 416 px，不裁），照样稿的 9 px 也说得通（同上）。
- **交给后面**：第 9 节（作者的浅色主题在暗色的图框上，决定 905，给 v0.2；读屏清单里 `summary` 的一条照决定 901 改写）；375 宽时甘特、时间线、旅程的字小于 5 px（照旧）；S3：编辑器借用 `reading.css` 的变量（8.2 第 16 条写"P4/S3 起也给编辑器"，`reading.css:12` 与 `design-tokens.test.ts:236` 的注释到 S3 一起看，审查 C 的 P4S2C-N1）；`W/reading/code-blocks.ts:85` 的注释一行 146 列，下次碰这个文件时折行（审查 C 的 P4S2C-F2-N4）。

### 10.3 S3：编辑器、编辑时的大纲与"保存"（2026-11-25，合并 `4f2fde3b`）

- **提交**：`ab6f4353`（编辑器的比例字体、等宽的行、压淡的标记、阅读视图的链接色与匹配色，照 6.3 提交 1）、`68a7e4be`（编辑时的大纲，"保存"只在需要时显出，照 6.3 提交 2）；照实加的 `3c75954d`（7.4 的截图找到的：WebKit 里编辑时大纲的长标题不截断）、`9a52ddb4`（变异 t8 活下来之后补的 frontmatter 一格）；审查的修复 `15372fcf`、`4ecc3e10`、`d54a9d00`、`45a477bd`；修复核对第 1 轮之后的 `d7fb332c`、`e6558722`，第 2 轮之后的 `bcd1575b`（各做了什么见 6.3 之后的"照实"）；加本文档提交。[S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)。
- **基线**：不另跑（决定 883）。提交 1 的 `make e2e` 兼作核对：只败第 1 节那三格里的两格（se2:231、lw8:836，同行同消息；pg15:113 那次过了）。之后 se2:231、lw8:836 每次都红，同行同消息；pg15:113 [webkit] 照旧有时过（实施者的十三次，九次 `make e2e` 与四次合并之前的整条命令，过了 2 次；审查 C 的一次门禁里过了）。另有的红都查清了，见"门禁"。
- **做完**：4.3.1–4.3.7（照实见 4.4 之前的一段）；决定 874–878、880 与 891 的 S3 一半（l5 的五项照旧量，值不变；决定 910 另加查找面板的八项）、881 的 S3 部分（M5-polish 13.1"编辑时的大纲"，第 5、11 项里失锁横幅的 `bdi`）、885、889、894 的 S3 一半（"保存"不需要时 `invisible`、占位）、895、896、907–911；01-P1 第 9 节 P4 那一条的最后一项（编辑器的链接色与搜索匹配色，决定 875）。S1 交来的：变异 108 跑了（PG17 第 3 格三种浏览器红在 `:344`）；右栏的读者记录随大纲的列表（决定 897），编辑时右栏是 `EditOutline`、回到阅读 `PageOutline` 重新挂上、照旧跟，897 的一格照绿。S2 交来的：`reading.css:12-15` 与 `design-tokens.test.ts:236-238` 的注释在 `ab6f4353` 改了（编辑器借用 `--nw-mark-background`、`--nw-match-selected`、`--nw-link-line`、`--nw-code-background`，审查 C 核对）；`W/reading/code-blocks.ts:85` 没碰。
- **测试**：
  - vitest：5402（225 个文件，S3 之前）→ 5415（226，`ab6f4353`：`prose-lines.test.ts` 8 格，`yaml-quotes.test.ts` 2 格，`source-editor.test.tsx` 2 格，`text-diff.test.tsx` 1 格）→ 5434（227，`68a7e4be`：`edit-outline.test.ts` 8 格，`source-editor.test.tsx` 2 格，`page-panel.test.tsx` 一格改写成两格，`page-edit.test.tsx` 6 格，`page-lock.test.tsx` 2 格）→ 5434（`3c75954d`）→ 5435（`9a52ddb4`）→ 5444（228，`15372fcf`：`edit-outline.test.ts` 3 格、`page-panel.test.tsx` 3 格、`wiki-links.test.ts` 3 格）→ 5449（`4ecc3e10`：`prose-lines.test.ts` 1 格、`source-editor.test.tsx` 2 格、`text-diff.test.tsx` 1 格、`link-completion.test.ts` 1 格；`d54a9d00`、`45a477bd` 不变）→ 5452（`d7fb332c`：`edit-outline.test.ts` 2 格、`page-panel.test.tsx` 1 格）→ 5454（`e6558722`：`source-editor.test.tsx` 2 格）→ 5455（`bcd1575b`：`wiki-links.test.ts` 1 格；`page-panel.test.tsx` 的 `inert` 一格改写）。
  - E2E：640，不变（L6、L5 的页面格与 PG17 第 3 格加步，没有新格）；`check.mjs` 照旧列 chromium 431、firefox 103、webkit 103、webkit-narrow 3，`REQUIRED` 不加行。
  - 先写的新格在没改的树上红：提交 1 在 `5c1fbdb2` 上只放测试，`prose-lines.test.ts` 导入失败，`design-tokens.test.ts` 一格（`theme.ts` 的 12 处色值），`source-editor.test.tsx` 两格（`'1.6'` 对 `'1.85'`，等宽的标记全无），`yaml-quotes.test.ts` 两格（`frontmatterRange is not a function`）；提交 2 在提交 1 上，`edit-outline.test.ts` 导入失败，reveal 两格（`expected undefined to be 2`），`page-panel.test.tsx` 两格，`page-lock.test.tsx` 两格（`bdi` 为空），`page-edit.test.tsx` 四格（没有 `invisible`），L6 红在当时的 `l6:239`。修复的：`15372fcf` 的 vitest 5 格与 `wiki-links.test.ts` 的导入失败，PG17 `:347` 三种浏览器（项高差 5.5 / 4.75 / 1.625 px）与 L6 `:244`；`4ecc3e10` 的 5 格（gutter 是 `rgb(245, 245, 245)`）、补全的强制色一格（`[]`）、L5 亮色的按钮有渐变、暗色的查找框 1.224；`d54a9d00` 的等宽一格（`monospace`、`16px`）；`d7fb332c` 的 4 格（`upTo` 那一格之后改成数调用，2 次对 6 次）；`e6558722` 的 2 格（标题里的代码没有 `--nw-heading`；查找框的宽是 `auto`）；`bcd1575b` 的 2 格（`![[v1.2 notes|Alias]]` 显示 Alias；大纲那一节的 `details` 不带 `inert`）。不先红的：page-edit 的两格（守 109、68）、`text-diff` 的行高一格（守 57）、"…and 3 more"一格（t12）、`--font-sans` 一句（t49）、查找面板一格的强制色一半（t63）；`9a52ddb4` 的一格在 t8 之下红。
  - 每次门禁"not configured to support act"0 次；全部 vitest 照旧打一次"Not implemented: navigation to another Document"（S1 已记）。
- **变异**（逐条在审查记录）：
  - 7.2 的 S3 各条（54–69、91–101、108–111、116、117，e2e 的 59e、60e、64e、65e、108e）都被它写的格抓住：59e 红在 `l5:143`（5.526 对下限 7.82）；108e 三种浏览器红在 PG17 `:344`（63.73 / 63.70 / 63.73 px）。
  - 实施者另写的 t1–t67（7.2 表末）：t6 等价（依据在它那一行），t53 只省代价、没有格抓，t67（实体的表放回模块级）没有格分得出、由堆的量看；t42e 活着，`45a477bd` 补了一项之后是 t50e；t7（t29e）、t27e、t28e 三种浏览器都红；其余都被抓住。
  - 审查者另写的（7.2 表末的 `c-` 各条）：审查 C 第一轮的 c-indent、c-codeblock、c-noscroll 与 noscroll-e 红，c-more、c-clear 活着（P4S3C-M3、N1，`15372fcf` 之后红），t7e 只有 WebKit 红（P4S3C-M1）；修复核对第 1 轮的 c-ready-never、c-upto、c-clamp3e 红，c-active 等价，c-fc-search 活着（P4S3C-F1-N2，`e6558722` 之后是 t63）；修复核对第 2 轮的 c-h2、c-comment-code、c-read-past 红，c-decode-fresh 同 t53 活着。审查 A 的一条（标题的容器过滤去掉）等价：标题节点只是 `Document`、`Blockquote`、`ListItem` 的块级孩子，过滤只省代价。
- **视觉的核对**（7.4）：
  - 实施者在三种引擎里截了：编辑器（亮暗；正文、frontmatter、围栏、表格、引用里的代码、标题与标题里的行内代码、链接、压淡的标记、选区、光标）、长文档、编辑时的大纲（1440、1024、375，点一项，阅读与编辑对照）、"保存"的四个状态、冲突与失锁、补全（亮暗、强制色）、查找面板（亮暗、1440 与 375、中英文）、对比视图；修复之后各重拍。三种引擎量出的值相同（除注明的）。
  - 实量：正文 15.5 px 系统无衬线，等宽行 13.5 px（行高 24.97 px），h1 20 px、h2 16.5 px，与样稿 `.ln` 相同；行内代码 13.5 px，h1、h2 里的随标题 17.42、14.38 px（Firefox 17.41、14.38）；编辑时大纲的项与阅读时同高（单行 25.88、两行 43.75 px，Firefox 43.77），长标题两行；"保存"藏着时状态的右边不动（英文 1440 宽 977.89 px、375 宽 216.89 px），状态到"保存" 10 px，藏着时到开关 73.7 px（英）、70 px（中），决定 896 的偏离；查找面板的框与按钮 30 px 高、13 px 字、9 px 圆角，框宽 168 px，1440 宽两行（82.4 px，Firefox 中文也是），375 宽多行（约 173 px），页不横滚；暗色的查找框对底约 13.0、按钮约 12.35（之前约 1.2）；对比与冲突视图 SF Mono 13 px、行高 20.8 px（Firefox 中文原是 16 px）；320 宽英文冲突面板的两个按钮上下排，右边在内容区的右沿 279 px（原 305.3）。
  - 大文档（Chromium 1280×800，经 API：980,777 字符、12,360 行、412 节）：`scrollTo` 跳到七处，每处之后 8 帧视口中间那一行：6 处 0 px；第一次远跳（顶 → 10%）一帧移 157.2 px，下一帧 −0.2 px；对照（去掉 S3 的三个扩展）同处 58.9 px。审查 A 在三种引擎里量了远跳与小步滚动：校正是 CodeMirror 对没量过的行的高度估计，S3 的幅度（14–143 px）在对照（59–204 px）的范围里，记给 v0.2。
  - 几 MB 文档（3,581,086 字节、45,002 行、1,501 个标题）：点"编辑"到编辑器出现 487–494 ms；之后大纲补完解析、显出"…and 501 more"用 290 ms（两次相同），这一段主线程脚本 158–160 ms，只有一个 ≥50 ms 的长任务（53–56 ms），在编辑器出现之前；对照 359 ms。审查 A：打字时每次 `dispatch` 中位 2.6–3.1 ms（不带 S3 的扩展 1.3–1.7 ms）。
  - 重读的代价（审查 A 的 P4S3A-F1-M2，A 的 `reread2.mjs`，Chromium，在第 500 个标题里插一个字，负载约 5）：20,000 个带行内标记与实体的标题，`d7fb332c` 之前每轮一个 148–168 ms 的长任务、报出在 453–470 ms，之后没有 ≥50 ms 的任务、326–336 ms；不带实体的 100–117 ms 与 5,000 个的 56–62 ms 也都没有了；只有文字的 20,000 个前后都没有。审查 C（修复核对第 2 轮，同一个 harness）：1,000 个各带 20 个命名实体的标题，实体不记下（t53）时每轮一个 60–75 ms 的长任务，记下时没有；`bcd1575b`（表每次读新建）上：1,000 个各带 20 个命名实体的标题重读 312–323 ms 报出、没有 ≥50 ms 的任务，A 的 20,000 个 328–334 ms、也没有（实施者，`fix/g-heap.log`）。
  - 实体的表留下的堆（审查 C 的 `heap.mjs`，Chromium 加 `--enable-precise-memory-info`，每次量之前以 CDP 做两次 GC；1,000 个标题各 300 个不同的数字实体，2.71 MB，编辑器建好、再换成空文档）：审查 C 量的 `e6558722` 26.9 / 21.9 MB（建好 / 换掉之后），t53 7.6 / 2.6 MB；实施者在同一个 harness 上重量（`fix/g-heap.log`，Chromium 一次，量之前 1.2 MB）：`e6558722` 27.2 / 21.9 MB，t53 7.6 / 2.5 MB，`bcd1575b` 7.6 / 2.6 MB，表每次读新建之后编辑器换掉就不多留，与不记下时一样。
  - Firefox：编辑时滚长文档报"scroll-linked positioning effect"，阅读时没有；对照同样报，S3 之前就有，记给 v0.2。
  - 截图在会话的临时目录，不进仓库。
- **对比度**（l5，决定 880、891）：`ab6f4353` 与 `9a52ddb4` 上与 S2 相同：亮 补全项说明 5.281、编辑器外框 3.098、选中项 2.646、阅读视图链接的焦点 3.098、菜单的当前项 2.646；暗 4.849、7.923、5.476、7.923、5.476；五项的下限不改。决定 910 另加八项，下限取实量 − 0.1：亮 聚焦的查找框 17.578（下限 17.47）、没聚焦的 15.992（15.89）、按钮的字 17.578（17.47）、`label` 6.183（6.08）；暗 12.987（12.88）、12.353（12.25）、12.353（12.25）、6.637（6.53）。
- **PG17 的时长**（7.5，第 3 格，Chromium / Firefox / WebKit）：实施者 6.7–7.6 s（`e6558722` 的 `make e2e` 7.0 / 7.4 / 7.2 s，合并之前 7.0 / 7.0 / 7.1 s；`bcd1575b` 合并之前 7.0 / 6.9 / 7.1 s）；审查 C 7.0 / 7.4 / 6.9 s（`9a52ddb4`）。推的是 7–10 s。L5 页面格 3.1 s，L6 页面格 1.6–1.7 s；本机 E2E 2.6–3.0 min（负载 4–45）。
- **门禁**：
  - 每个提交的 `heavy.sh make lint-web knip test-web` 都绿（提交之前没过的，改了才提交：`ab6f4353` 的 oxlint `no-unmodified-loop-condition`；`68a7e4be` 的 tsc（MobX 7 没有 `observable.ref`，改用 `observableRef`）与 oxlint `consistent-function-scoping` 两处；`9a52ddb4` 的 `consistent-function-scoping`；`15372fcf` 的 `consistent-function-scoping` 与 oxfmt）。
  - `CI=1 heavy.sh make e2e`：提交 1 637 / 2 / 1，2.8 min（负载 11.3 → 27.8）；提交 2 636 / 3 / 1，2.7 min（14.4 → 21.1）；`3c75954d` 636 / 3 / 1，2.8 min（18.3 → 33.1）；`15372fcf` 第一次 635 / 4 / 1（基线之外 L6 [chromium] 红在 `fixtures/assert/links.ts:43`：新标题里的 `[[Elsewhere#Part|part]]` 多一条索引的链接，改用同页的 `[[#Setup|the setup]]`），之后 636 / 3 / 1，3.0 min；`4ecc3e10` 637 / 2 / 1，2.7 min；`d54a9d00` 636 / 3 / 1，2.7 min；`d7fb332c` 636 / 3 / 1，2.8 min（结束时负载 41.9）；`e6558722` 636 / 3 / 1，2.6 min（24.9）。`9a52ddb4`、`45a477bd`、`bcd1575b` 由合并之前的整条命令覆盖。
  - 合并之前的整条命令（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`，`m13-ui-refresh` 一直是 `5c1fbdb2`）：`9a52ddb4` 上 vitest 227 个文件、5435 格（59.2 s），E2E 636 / 3 / 1，2.8 min，整条 3 分 56 秒（负载 5.2 → 27.5）；`45a477bd` 上 228 / 5449，636 / 3 / 1，2.7 min，3 分 52 秒（7.3 → 28.9）；`e6558722` 上以 2 退出，只因基线三格（se2:231、pg15:113 [webkit]、lw8:836 [webkit]）：228 / 5454（55.7 s），636 / 3 / 1，2.6 min，3 分 41 秒（04:38:35 → 04:42:16，3.7 → 23.3）；`bcd1575b`（合并的门禁）上同样只因这三格：228 / 5455（55.7 s），636 / 3 / 1，2.6 min，3 分 43 秒（05:21:33 → 05:25:16，15.3 → 20.6）；每次"not configured to support act"0 次，`check.mjs` 431 / 103 / 103 / 3，受保护路径的 `git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 与 `git diff --check` 都以 0 退出，跑完 `git status` 为空。
  - 审查 C 的门禁：`9a52ddb4` 上合并之前的整条命令 637 / 2 / 1（se2:231、lw8:836；pg15 过了），2.8 min，整条 4 分 13 秒（负载 12.2 → 44.6，别的审查者在锁外也有进程），vitest 5435 格（63.05 s）；逐个提交的 `make lint-web knip test-web`：`ab6f4353` 226 / 5415、`68a7e4be` 227 / 5434、`3c75954d` 227 / 5434，修复核对第 1 轮 `15372fcf` 228 / 5444、`4ecc3e10` 228 / 5449、`d54a9d00` 228 / 5449，第 2 轮 `d7fb332c` 228 / 5452，都以 0 退出（实施者每个提交的门禁跑在提交之前，日志单独证明不了"每个提交单独为绿"，C 的这一遍补上）。
- **与设计不同之处**：照实多九个代码提交（`3c75954d`、`9a52ddb4` 与七个修复提交）；874–876、878、889 的文字；4.4 之前的"照实"（4.3.1–4.3.4、4.3.6）；4.5 的 knip；5.3（L6 的行号与新标题，PG17 第 3 格一步，L5 挪到改的）；6.3；7.1（S3 的照实）；7.2（第 60、64、65 行，S3 的 t1–t67 与审查 C 的 `c-` 各条，表后的一句）；7.3、7.5；8.1 第 1 条；8.2 第 19、22、24–26 条与新的第 27、28 条，引言与"不改的"一行；第 9 节；决定 907–911。实施者第一轮的偏离 1–9（审查 A 逐条接受）：两个照实加的提交；截断在 `span` 上；`textOf` 另去掉 `QuoteMark`、setext 截在下划线那一行；两片之间改了文档就结束这一轮、sink 经 `Facet`；`observableRef`；假计时推 1 ms × 5；`frontmatterRange` 返回 `Frontmatter`；"一直在 DOM 里"一格自己数 `PUT`；`lostText` 一律 `Isolating`。修复报告的 1–8：`ready` 不叫 `told`；907 另做实体解码、嵌入去尺寸，`[see [[A]]](u)` 与行内数学照实；A-N2 的"`Link` 里跳过"做成 `LinkTitle` 里不认；补全的外框只在强制色下（无条件加会让 L5 的 `indicatorContrast` 量到透明的外框，比值 1）；查找面板的字号 13 px（`text-compact`），不是 13.5；t29e 三种引擎都红；查找面板强制色的焦点外框当时没有格守（`e6558722` 补了）；变异的工具以 `HEAD` 核对复原，每轮在提交之后跑。偏离：藏起的"保存"照旧占位（决定 896）；frontmatter 整块次要色（决定 874）；冲突面板 320 宽的按钮折行量过、没有守（冲突的故事没有窄屏的一步）；907 的已知差别（4.4 之前的"照实"）；e2e 只交叉核对了 907 的显示文字一条路（L6 的 `[[#Setup|the setup]]`），"页名 > 小节"与嵌入的说明只在 vitest 里，对照的是照服务端代码写的期望（审查 C 修复核对第 1 轮）。
- **交给后面**：第 9 节（v0.2：Firefox 编辑时的 scroll-linked 警告、第一次远跳一帧约 157 px、全篇约 2 万个互不相同的数字实体时编辑时的大纲每轮重读 52–60 ms（审查 A 的 P4S3A-F3-N1，照实）；P5：版本行单独一行的"-"；负责人：Safari 的人工核对）；`CHANGELOG.md` 0.1.0"网页的打磨"一条里的"编辑时没有大纲"由 M13 收尾删（00 号文档第 7 节"收尾"第 2 项）；`W/reading/code-blocks.ts:85` 的长注释照旧（10.2）；P4 的 Phase 完成另一个提交（8.1 第 4 条）。

## 附录：核对过的代码

在 `072dcec2` 上读过（行号以此为准）：

- 外框与路由：`W/app/layout.tsx`（全文）；`W/app/routes.tsx:150-280`；`W/app/settings-page.tsx:1-60`；`W/app/shell-sidebar.tsx:300-373`；`W/pages/workspace/workspace-layout.tsx:40-75,160-190`；`W/pages/notebook/notebook-layout.tsx`（全文）；`W/pages/notebook/upgrade-banner.tsx`（全文）；`W/stores/preferences.store.ts`（全文）；`W/lib/focus-leaving.ts`（全文）；`W/i18n/isolated.tsx`（全文）；`W/i18n/format.ts:30-40`；`W/components/ui/button.tsx`（全文）；`W/styles.css`（全文）；`W/lib/cn.ts`。
- 页：`W/pages/page/page-layout.tsx`（全文）；`page-edit.tsx:200-470`；`page-editing-bar.tsx`（全文）；`edit-lock-note.tsx`（全文）；`edit-lost-banner.tsx`（全文）；`conflict-panel.tsx:30-80`；`breadcrumbs.tsx`（全文）；`page-panel.tsx`、`panel-section.tsx`、`page-outline.tsx`（全文）；`page-versions.tsx:1-160`；`page-backlinks.tsx:120-190`；`page-properties.tsx:60-100`；`page-menu.tsx:30-60`；`reading-view.tsx:170-190`；`view-anchors.ts`（全文）；`server-html.ts:60-300`；`asset-dialogs.tsx:210-225`；`W/stores/page-editing.ts:120-140`。
- 阅读视图：`W/reading/reading.css`（全文）；`enhancement.ts`（全文）；`patch.ts:330-410`；`scroll-regions.ts`（全文）；`highlight.ts`（结构）；`diagrams.ts:60-250`；`task-toggle.test.ts:75-90`。
- 编辑器：`W/editor/theme.ts`（全文）；`markdown.ts:1-60,320-340`；`source-editor.tsx:1-300`；`text-diff.tsx:130-150,380-420`；`registry.ts`（全文）；`loaded/yaml-quotes.ts:200-300`；`loaded/link-completion.ts:100-145`；`loaded/gfm-table.ts:1-30`；`web/apps/web/build/out-of-main.ts`；`node_modules/@codemirror/language/dist/index.js:560-700`（`Work`、`ensureSyntaxTree`、`syntaxTreeAvailable`）。
- 测试：`W/design-tokens.test.ts`（全文）；`W/pages/page/page-panel.test.tsx`（全文）；`page-layout.test.tsx:1-60`；`page-edit.test.tsx:1-60,270-300`；`page-asset-upload.test.tsx:30-50,355-460,650-700`；`page-autosave.test.tsx:150-170,320-340`；`conflict.test.tsx:60-90`；`page-versions.test.tsx:395-415`；`page-menu.test.tsx:150-175`；`page-properties.test.tsx:1-30`；`W/test/page-panel.ts`；`W/pages/notebook/upgrade-banner.test.tsx:1-140`；`W/pages/notebook/general-page.test.tsx:655-690`；`W/reading/diagrams.test.ts:60-150,320-370`；`W/pages/page/reading-view.test.tsx:630-650,1000-1020,1200-1210`；`page-tasks.test.tsx:50-60`。
- e2e：`e2e/playwright.config.ts`（全文）；`tools/e2e-projects/check.mjs:1-70`；`e2e/fixtures/wiki-pages.ts:40-50,155-245,325-345`；`clipboard.ts`（全文）；`narrow.ts:1-30`；`workspaces.ts:85-110`；`pages.ts:130-150,200-225`；故事 l6（全文）、l5:110-200、pg5:80-170、l4:30-40,170-200,230-245,340-350、up1:170-260、hi1:250-270、hi2:105-115、hi3:350-365,435-450、as3:315-330,375-390、as6:90-155、lw5:540-575、c7:215-230、ui1:50-70、pg2:205-215、lw3:340-350；按文件 `grep` 了第 5 节开头列的那些词。
- 文档：[00-M13](00-M13-design.md) 4.1、4.3、4.5、4.7、4.8、第 7、9、10 节；[03-P3](03-P3-settings.md)（结构、第 0–2、6–9 节与附录）；[02-P2](02-P2-shell.md) 第 9 节；[01-P1](01-P1-foundation.md) 第 9 节；[mockups/reading.html](mockups/reading.html) 的样式表与标记；[M5-polish](../M12-release/handoffs/M5-polish.md) 第 3、12、13 条；[M12-polish](../../v0.2/handoffs/M12-polish.md) 第 1–15 项与"处理进展"；[v0.1 总体设计](../v0.1-design.md) 9.1–9.3、13.2 第 17、23–26、31、32 条、13.4 第 3 条、第 15 节的开头；[02-workflow](../claude_to_codex/02-workflow.md)、[05-lessons](../claude_to_codex/05-lessons.md)、[06-environment](../claude_to_codex/06-environment.md)。

修订时另读（同在 `072dcec2` 上；探针在会话的临时目录 `p4-design/`，不进仓库）：

- 页与外框：`W/pages/page/version-page.tsx:180-210`；`W/pages/notebook/settings-layout.tsx`（全文）；`W/app/routes.tsx:255-295`；`W/app/narrow.ts`（全文）；`W/pages/page/readers-input.ts`（全文）；`W/pages/notebook/page-tree.tsx` 的 `reveal`；`W/pages/page/server-html.ts` 的 `giveBack`；`W/pages/page/page-edit.tsx:250-295`；`W/pages/page/page-properties.tsx:260-270`；`W/stores/page-editing.ts:220-390`；`W/services/page.service.ts:170-185`；`W/reading/enhancement.ts:115-140`。
- 服务端的写法：`server/internal/platform/markdown/render.go:60-80`；`server/internal/platform/markdown/obsidian/render.go:175-190`；`server/internal/platform/markdown/markdowntest/inputs.go:220-230`。
- 编辑器与库：`W/editor/text-diff.tsx:130-150,395-425`；`node_modules/@codemirror/view` 6.43.13 `dist/index.js:8340-8360`（样式模块的次序）；`@codemirror/language` 6.12.4 `dist/index.js:600-640`；`@lezer/markdown` 的 `Link` 与 `InlineCode` 的解析；mermaid 11.17.2 的 `Theme.calculate`（`base` 主题）、各图读的变量与 `themeCSS` 的处理（`CSSStyleSheet.replaceSync` 与按图的 id 限定、指令里的 `sanitizeCss`）。
- 测试：`W/design-tokens.test.ts:200-320`；`web/apps/web/vite.config.ts:40-50`；`W/pages/workspace/shell-drawer.test.tsx:15-30`；`W/pages/page/page-asset-upload.test.tsx:760-780`；`page-properties.test.tsx:70-130,700-710`；读锁说明的九处（`page-autosave.test.tsx:165-175`、`page-tasks.test.tsx:405-415`、`page-versions.test.tsx:375-395`、`version-page.test.tsx:410-485`、`version-restore.test.tsx:370-380`）。
- e2e：`e2e/fixtures/settings-pages.ts:55-75`（`holdAnswer`）；`navigation.ts:100-115`；`auth.ts:10-20,140-165`；`workspaces.ts:85-110`；up1:230-260。
- 文档：[设计审查记录](reviews/P4-design-review.md) 与三位审查者的报告；[02-workflow](../claude_to_codex/02-workflow.md) 3.5–3.7；[01-P1](01-P1-foundation.md) 第 9 节；[mockups/reading.html](mockups/reading.html) 的 `.editor` 一节；[v0.1 总体设计](../v0.1-design.md) 头的表（本地存储键）、12.4、13.2 第 23、25、26、32 条。
- 再修订时另读：`W/app/layout.tsx:55-80`；`W/app/shell-sidebar.tsx:352`（`md:w-63`）；`W/pages/page/page-editing-bar.tsx`（全文）；`W/i18n/messages/en.ts:632-655`、`zh-CN.ts:604-625`（状态的说明）；`W/pages/page/page-versions.tsx:95-140`；`page-versions.test.tsx:15-30,55-70,98-106,155-160,175-185`；`version-restore.test.tsx:150-160`；`W/pages/page/server-html.ts:82-195`；`W/test/setup.ts`（全文）；`W/test/page-server.ts:160-350`；`W/app/arrival.test.tsx:120-170`；`web/apps/web/vite.config.ts:40-50`；`tools/e2e-projects/check.mjs:1-20,60-80`；`e2e/fixtures/auth.ts:10-20,140-165`；`server/internal/platform/markdown/obsidian/render.go:170-200`；审查 B 第 1 轮的 `vars-fix.js` 与 `reading.r1fix.css`、审查 A 的 `headmodes.mjs`、审查 C 的 `geo.mjs`（都在会话的临时目录）。
- 三修时另读：`W/pages/page/page-editing-bar.tsx:50-70`；`W/pages/page/page-edit.tsx:400-432`；`W/app/not-loaded.tsx`（全文）；`W/pages/page/page-outline.tsx:55-63`（`sameView`）；`W/i18n/isolated.tsx`（全文）；`W/pages/page/page-versions.tsx:110-135`；`page-versions.test.tsx:15-30,98-106,176-184`；`version-restore.test.tsx:150-158`；`W/stores/page-editing.ts:130-140`；`W/services/page.service.ts:150-160`；`W/i18n/messages/en.ts:632-655,1354-1355`、`zh-CN.ts:604-625,1270`；`W/reading/title-media.test.ts:55-70`；`W/pages/page/page-asset-upload.test.tsx:136-190`；`e2e/fixtures/settings-pages.ts:55-75`；样稿 `reading.html:122-123,301-303`；三位审查者第 2 轮的报告与探针（审查 A 的 `head894b.mjs`、`head894c.mjs`，审查 B 的 `mock-ver2.html` 与 `sheet-versions2.png`，审查 C 的 `nav.cjs`、`caret.cjs`）。
