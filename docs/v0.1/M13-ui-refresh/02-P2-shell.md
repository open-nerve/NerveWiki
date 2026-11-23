# M13/P2 新外框：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M13/P2 新外框（三个 Step：S1 外框与账户行；S2 笔记本与页面树；S3 搜索、⌘K、快速切换与窄屏） |
| 日期 | 2026-11-22（初稿）；2026-11-23（照三份设计审查修订；照修订核对第 1、2 轮再修订；主持者补上最后的 Nit，设计定稿；同日照 S1 的实施与审查修订，[S1 的审查记录](reviews/P2-S1-shell-account-review.md)；同日照 S2 的实施与审查修订，[S2 的审查记录](reviews/P2-S2-notebooks-tree-review.md)） |
| 状态 | 设计定稿（修订核对第 3 轮之后三位都没有 Important、Minor；主持者补上最后的 Nit；[审查记录](reviews/P2-design-review.md)，主持者的决定 769–800）；S1 实施与审查完成（[S1 的审查记录](reviews/P2-S1-shell-account-review.md)，主持者的决定 801–803），已合并 `fdad6b9e`；S2 实施与审查完成（[S2 的审查记录](reviews/P2-S2-notebooks-tree-review.md)，主持者的决定 804），合并见 00 号文档第 12 节；S3 待开 |
| 上级文档 | [M13 总设计](00-M13-design.md) 4.2（全部）、4.3 的"按钮""菜单"两条、4.7、4.8（UI1、PG16 与变异的最低清单）、第 7 节 P2 三行与"移交的落实""总体设计的修订"第 5、6、7 项、9.1 的"去掉顶栏""账户菜单""搜索框""工作区切换""页面树进笔记本列表""目录、健康、历史、回收站""375 px 与窄屏""窗口滚动与视口"八行、第 10 节"粘住的侧栏"与"窄屏抽屉"两行、第 11 节决定 717、719、720、727、730–733、737、745；[01-P1](01-P1-foundation.md) 第 9 节"P2"与"P2–P5"两条；[v0.1 总体设计](../v0.1-design.md) 9.2、9.3、13.2 第 16、17、20、26 条、13.4 第 3 条；移交 [v0.2/handoffs/M12-polish.md](../../v0.2/handoffs/M12-polish.md) 第 14 项、第 1 项里的 26.3、35.7、37、38.5 与第 8 项里的 `onMac` 一条 |
| 视觉参照 | [mockups/palette.html](mockups/palette.html)（外框：`.side`、`.ws`、`.search`、`.row`、`.tree`、`.group`、`.me`，`:62-91`、`:229-250`） |
| 代码位置的写法 | 同 00 号文档：`W/` 是 `web/apps/web/src/`；e2e 写作 `e2e/stories/…:行`。行号是 `m13-ui-refresh` 的头 `4c3f10b9` 的 |
| 工作笔记 | 设计只读，没有建应用、没有跑 e2e。设计者跑过两个探针（会话的临时目录，不进仓库）：Tailwind 4.3.3 的 `compile`（第 4 节的全部类名，结论在决定 785）与 Playwright 1.63 三种引擎里一个静态页（粘住的侧栏、滚动锚定、窄屏的位移、`inert` 与挪动拿着焦点的节点，结论在第 1 节末与决定 774、776、781）。对比度的预期值由 4.1 的十六进制按 WCAG 2 算出，实量在 S1 做。修订时与修订核对第 1 轮之后，设计者复用三位审查者的探针重跑了改过的说法（第 1 节末"修订时的探针""修订核对第 1 轮之后的探针""修订核对第 2 轮之后的探针"）；第 4 节的 hook 照写进探针文件、以仓库的 oxlint 配置零警告跑过（4.4） |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P2 分三个 Step，串行，各自一个分支（`m13-p2-s<m>`，从 `m13-ui-refresh` 拉出、合并回它）、一次审查与修复核对、一次合并。每个提交单独为绿（7.3）。

- **S1 外框与账户行**（`m13-p2-s1`）：去掉顶栏；`Layout` 在没有侧栏的地方放右上角，放什么按会话定（决定 769、792、793）；侧栏换成三段（顶部、自己滚动的中段、底部的账户行），宽屏粘住（`md:` 起），底色 `--sidebar`；工作区切换挪出 `nav`、放在顶部，带方形头像；导航行与图标；账户行与头像菜单（个人设置、外观子菜单、退出登录、版本行），未登录时的"外观"按钮；离开工作区外框的两个菜单项带 `arrived`，个人设置页的 `h1` 取到达的焦点（决定 771）；按钮的 30 px 与胶囊形、菜单的子菜单（决定 773）；头像的两个变量；a8 改从头像菜单看主题；se1 在侧栏上的三项下限重量。**单独为绿**：账户按钮挪到侧栏与右上角、两个顶栏菜单删掉、改它们的 vitest 与 a8，都在同一个提交里；树照旧画在 `NotebookColumn` 里（S2 换），搜索框照旧是今天的样子（S3 换），窄屏照旧把侧栏排在页之上（S3 换成抽屉）；新的导出（`W/app/corner.tsx` 的 `CornerBack`、`useCornerBack`，`AppearanceItems`、`AppearanceMenu`、`initialOf`、`ShellSidebar`、菜单的三个子菜单部件）都在 S1 里有调用者，knip 不报；`useSession` 从 `guards.tsx` 挪进 `W/app/session-state.ts`，三个调用者在同一个提交里改引入（决定 793）。S1 开工之前先在没改的头上跑一次 `make e2e` 的基线（决定 795）。
- **S2 笔记本与页面树**（`m13-p2-s2`，S1 合并之后开）：当前笔记本在列表里就地展开（笔记本行、树、树下的图标链接），树的容器由外框按笔记本 id 持有、当前那一项收养，换组时还焦点、通知树的菜单重量（决定 776、798、800）；别的笔记本的链接带 `arrived`（决定 777）；笔记本行的齿轮与＋（决定 778）；树的行换装、普通页的图标、"目录""历史"两行对齐并标当前（决定 780、796）；侧栏把当前行滚进可见范围（决定 779、791）；`notebookGroups` 每项只取第一个链接；新故事 PG16（两格，三种浏览器）。**单独为绿**：树挪进列表项与 `notebookGroups` 的改法在同一个提交；`NotebookColumn` 与 `useNotebookColumn` 随它的唯一用处一起换掉；PG16 进 `threeBrowsers` 与 `check.mjs` 的必需格在加 PG16 的那个提交。
- **S3 搜索、⌘K、快速切换与窄屏**（`m13-p2-s3`，S2 合并之后开）：胶囊搜索框、⌘K 与它的让路、两个快捷键的监听挪到外框、`onMac` 挪进 `lib/`（决定 783、785、794、797）；快速切换换装（决定 786）；窄屏的抽屉、细栏与遮罩（决定 781、782、784、790、797、799）；新故事 UI1（三格，三种浏览器）。**单独为绿**：监听挪走与搜索框的测试在同一个提交；抽屉、`dialogOpen({ besideDrawer })` 与 UI1 在同一个提交；`@narrow` 的必需格不变（第 5 节）。

为什么是这样分：S1 只动"哪里有账户、侧栏长什么样"，S2 只动"笔记本与树在侧栏里的哪里"，S3 只动"搜索与窄屏"。三块各自有一批故事与测试要改（S1：a8、se1 两个故事（用账户菜单的另外五个不改）与十个 vitest 文件；S2：`notebookGroups` 的 17 处调用、树的测试、`general-page.test.tsx`；S3：搜索框的测试、375 px 的格），彼此不重叠，审查各看一块。S2 的稳定容器与 S3 的抽屉是 P2 里最难的两处，各自单独审查。

## 1. 基线

- **外框**（`W/app/layout.tsx:32-58`）：`Layout` 是根路由的组件（`W/app/routes.tsx:37`），画顶栏（`:38-47`：产品名的链接、`LanguageMenu`、`ThemeMenu`、`UserMenu`）与其下的一行：`display: contents` 的侧栏位置（`:49`，`ShellColumn` 经上下文给出，`:11-21`）与唯一的 `main`（`:50`，`p-6`）。两个错误边界：根路由的（`routes.tsx:38`，`Layout` 自己抛错时）与其下的（`:41`，`Layout` 之内）。
- **工作区外框**（`W/pages/workspace/workspace-layout.tsx:74-138`）：读取中与读取失败时 `NotLoaded`（`:102-104`），找不到时 404 或去首页（`:105-107`）；找到之后把侧栏 `left`（`:110-129`）门户进 `ShellColumn`：`data-shell` 的根（`:111-113`，`space-y-4 border-b p-3 md:w-60 md:border-r md:[overflow-anchor:none]`，注释 `:108-109` 写了 `overflow-anchor` 的理由），里面依次是搜索框（`:115`）、工作区的 `nav`（`:116-126`：切换器 `:117`、首页与设置两行 `:118-124`、`NotebookNav` `:125`）、笔记本的位置（`:127`，经 `NotebookColumn` 上下文给出，`:36-46`）。侧栏不粘住，随窗口滚走。
- **账户菜单**（`W/app/user-menu.tsx:28-75`）：幽灵按钮（`CircleUser` 图标与显示名），菜单是"设置"（`Link` 到 `/settings/profile`，不带路由状态）、"退出登录"、分隔线与版本行（`:58-65`）。没有登录时不画（`:36-38`）。主题与语言各是顶栏上的一个图标菜单（`W/app/theme-menu.tsx:18-46`、`W/app/language-menu.tsx:16-40`）。
- **工作区切换**（`W/pages/workspace/workspace-switcher.tsx:33-66`）：`justify-between` 的幽灵按钮，名字与 ⇕，`aria-describedby` 是"切换工作区"；菜单里"新建工作区"是 `Link to="/create-workspace"`，不带路由状态（`:55-60`），新建页的 `h1` 却有到达的焦点（`W/pages/create-workspace.tsx:27,32`）。
- **导航行**（`W/components/nav-item.tsx:11-23`）：`rounded-md px-3 py-2 text-sm`，当前 `bg-active font-medium`；工作区的两行、三个设置布局与"更多"都用它。
- **笔记本**（`W/pages/workspace/notebook-nav.tsx:19-49`、`notebook-groups.tsx:29-75`）：两组，每组一个 `h2` 与它命名的 `ul`（`:63-74`），每本一个 `li`（`:52-56`）里一个 `NavItem`；"新建笔记本"是幽灵按钮（`notebook-nav.tsx:35-46`）。工作区首页用同一个 `NotebookGroups`（`W/pages/workspace/workspace-home.tsx:38-39`）。
- **树**（`W/pages/notebook/notebook-layout.tsx:70-77`）：笔记本外框把 `<div key={notebook.id} className="space-y-4">`（`PageTree` 与 `NotebookPlaces`）门户进 `NotebookColumn`。`PageTree`（`W/pages/notebook/page-tree.tsx:156-215`）的 `nav` 里先是一行：灰色小字的 `h2`（`:159-165`，`tabIndex={-1}`，移动、删除之后焦点回到它，`:98`）与常驻的＋（`:166-177`），再是 LLM Wiki 的"目录""历史"两行（`:179-199`，`state={opened}`，不标当前，`px-3` 与页的行不对齐），失败的提示（`:200-209`）与树。树的行（`W/pages/notebook/page-tree-item.tsx:259-350`）：缩进每层 0.75 rem（`:268`），展开按钮或 1.5 rem 的空位（`:270-283`），链接（`:284-305`，当前 `bg-active`，只有来源根、来源与 `AGENTS` 有图标），菜单按钮（`:306-334`，悬停时显出），落点线（`:336-347`）。列表在点击时自己导航，带 `opened`（`:149-155`）。"更多"是四个文字的 `NavItem`（`W/pages/notebook/notebook-places.tsx:15-35`）。
- **搜索框**（`W/pages/workspace/search-box.tsx:66-178`）：`role=search` 的表单，输入框与描边的"搜索"按钮并排（`:145-166`），其下是看得见的范围说明（`:173-175`，`aria-describedby` 指它）；`Mod+Shift+F` 的监听在搜索框里（`:93-106`）；`aria-keyshortcuts` 只有一个（`:155`）；`composing` 是模块内的函数（`:35-38`）。
- **快捷键**（`W/app/shortcuts.ts`）：`isMod`、`isModShift`、`onMac`；`dialogOpen()` 认 `[role="dialog"], [role="alertdialog"]`（`:38-41`）。
- **快速切换**（`W/pages/notebook/quick-switch.tsx:50-72,136-193`）：对话框里一个组合框与列表；选项是两个 `span`（标题、上级位置，`:176-182`），当前项 `bg-active ring-2 ring-ring ring-inset`（`:171-174`）；`quick-switch.test.tsx:18-22,164` 按 `span` 读它们。
- **组件**：按钮只有 `default`（36 px）与 `icon` 两个尺寸（`W/components/ui/button.tsx:17-20`）；菜单没有子菜单（`W/components/ui/dropdown-menu.tsx:7-71`）。`W/styles.css` 没有头像的颜色。
- **测试**：vitest 5118 格（P1/S2 之后）；E2E 本机 2.4–2.8 min（P1/S2 合并之前，随负载）；本机的已知失败三格（P1 决定 764；S1 开工之前的基线记下的失败所在的行与消息的第一行，决定 795(a)，[S1 的审查记录](reviews/P2-S1-shell-account-review.md)）：SE2 [chromium] `se2:231`（`Error: expect(locator).toHaveValue(expected) failed`）、LW8 [webkit] `lw8:836`（`Error: expect(locator).toBeFocused() failed`）、PG15 [webkit] `pg15:113`（`Error: expect(received).toEqual(expected) // deep equality`）（格的开头行是 se2:188、lw8:822、pg15:75），偶发 AS6 `as6:153`、LW8 `lw8:771` 与负载过 100 时 `markdown.test.ts` 的"not slower"。
- **设计者的探针**（Playwright 1.63，macOS，三种引擎，一个模仿外框的静态页）：
  - 窗口滚到 3000 px，在读者之上插进 500 px：三种引擎的窗口都到 3500（滚动锚定留住读者），侧栏带不带 `overflow-anchor: none` 都一样；
  - 粘住的侧栏里的列表一次长 2000 px、侧栏自己滚到底：窗口的 `scrollY` 不变，侧栏的顶一直是 0；
  - 375 px 下 `position: fixed` 加 `translateX(-100%)` 的侧栏，关着、开着时 `scrollWidth` 都是 375；
  - 拿着焦点的元素的祖先加上 `inert`：三种引擎都把焦点交给 `body`；
  - 把拿着焦点的节点挪到另一个父元素下：三种引擎都丢了焦点（只有 Chromium 发 `blur`），挪完再 `focus()` 都拿回来。
- **修订时的探针**（会话的临时目录 `p2-design/`，复用审查者的；React 19.3、React Router 8.4、`radix-ui` 1.6.7，Playwright 1.63 的三种引擎与 jsdom 30）：
  - 抽屉（审查 A 的最小外框加决定 790 的变体，`probe-a/run-rev.mjs`、`jsdom-rev.mjs`）：A 的 T1–T7 三种引擎与 jsdom 都过（树的链接到 `h1`，首页、Esc 到菜单按钮，账户菜单里的 Esc 只关菜单，Shift+Tab 回绕，宽屏点空白处焦点在 `body`，右上角的菜单项 15 次都到 `h1`）；当前的地址在抽屉里带 `arrived` 时焦点落到 `body`，照旧 `opened` 时到菜单按钮；后退到带 `opened` 的一条时，没有先 `blur` 的 Chromium 里标题取不到（焦点还在已 `inert` 的切换器上，Chromium 到下一次渲染才修正），先 `blur` 之后三种都到 `h1`；用程序把焦点放到抽屉外，困住时三种与 jsdom 都拉回来，变异 35（只留 `loop`）时不拉回；变异 39（总给菜单按钮）三种都落在菜单按钮上（抓得住）；有字的搜索框里按 Esc，Chromium、Firefox 先清空、抽屉留着，Playwright 的 WebKit 不清空。
  - 审查 C 的两个 jsdom 探针按决定 790 改写（`probe-c/zz-focus-790.test.tsx`，vitest）：四格都过，变异 39 的格里 `WorkspaceLayout` 那一层的 passive effect 在去向页的之后跑，菜单按钮赢。
  - 右上角（审查 C 的 `zz-route-error-probe` 加一个照决定 792 写的右上角，`probe-c/zz-corner-792.test.tsx`）：首次载入就出错时右上角读到"Ada"，`GET /me` 只一次；平常的页上 2.5 秒里不多读；未登录与 `GET /me` 答 500 时是"外观"。
  - 当前行（审查 B 的 `reveal-order` 照决定 791 改写，`probe-b/reveal-791.cjs`）：先收养、再五次导航（其间在主区按键、点树的行、在侧栏里按键），有行的五次都滚；晚到的行在读者于侧栏里滚轮之后不滚，在主区滚轮之后照滚。
  - 收养（`probe-b/slot-order.cjs`）：`:slug` 懒加载、`Layout` 先有侧栏的位置时，`SlotHolder` 的 layout effect 在同一次提交里先于树的；只有工作区与 `Layout` 在同一次提交里第一次画出（不懒加载）时，树的 effect 才先跑。
  - 外观（`probe-b/row-probe.mjs`、`se1-pill.mjs`，Tailwind 4.3.3 编译 `W/styles.css`，Chromium）：第 4 节改过的类名的实量与计算值，结论在决定 778、785、794 与 781。
- **修订核对第 1 轮之后的探针**（同一目录，复用审查者的）：
  - 抽屉里被拦下的导航（审查 A 的 `p2-rev-a/r1/` 加决定 799 的上下文，`probe-a-r1/run.mjs`）：三种引擎里，"留下"之后不关抽屉（A 的复现）时抽屉开着、焦点在被遮住的编辑器上，打的字进了它；安静地关上时抽屉关上、焦点在编辑器里。同一个外框里，监听只注册一次：`open`、`closed` 经 ref 读时，在抽屉里点"首页"之后 Ctrl+K 打开抽屉、焦点在框里；读渲染时的值（A 的 P2A1-M2）时抽屉不开，焦点留在菜单按钮。
  - 当前行（`probe-b/reveal-r1.cjs`，照审查 B 的 `reveal-791-hash.cjs` 改）：同一页换 hash、回到不带 hash、`replace` 到当前的地址都算同一次导航，侧栏不动；树还没读到时换了 hash，行到了照滚（其间在侧栏里滚过轮时不滚）；之后开别的页照滚；原来的五次导航与晚到的行照旧。
  - 换组时树的菜单（`probe-b/slot-move-r1.cjs`，jsdom，`TreeMenu` 是 `observer`）：六种情形，结论在决定 800。
  - 笔记本行（`probe-b/row-probe-r1.mjs`，Chromium）：指针在＋上时行的底是 `--hover`、字是 `--foreground`；行的两条 `group-has-[…:hover]` 编译在 `@media (hover:hover)` 之下（决定 778）。
  - 导入的环（`cycle/`）：oxlint 1.86.0 照仓库的 `.oxlintrc.json`（类别只有 correctness、suspicious、perf）不报一个两个文件的环，加 `-D import/no-cycle` 才报（决定 793）。
- **修订核对第 2 轮之后的探针**（`p2-design/probe-r2/`、`lint-r2/`）：
  - `useInsertionEffect`（`insertion.cjs`，React 19.3、jsdom）：父的 insertion effect 在同一次提交里先于子的 layout effect 跑，新挂上的行、更新的行、门户里的都读到这次的 key；只有一行自己重画的提交里读到上一次提交的；挂起的过渡（Suspense 挡住、没有提交）里一行自己重画时，insertion effect 写的读到提交了的 key，渲染时写的读到没提交的（审查 B 的 P2B2-N1 的情形二）。结论在决定 791。
  - 当前行（审查 B 的 `reveal-r2.cjs`、`reveal-transition.cjs` 加一个变体：总调 `reveal()`、key 在 insertion effect 里更新；`reveal-r1-r3.cjs` 是第 1 轮的 `reveal-r1.cjs` 照同样改）：B 的 A 格（换 hash 与晚到的行在同一次提交里）滚，B–D 格与原来的写法相同；过渡里那一块行提交时就滚，不等换 hash 的提交；第 1 轮的各格照旧（滚的仍是 k1、k2、k3、k4、k6、h5、g1、w2、x1）。
  - lint（`lint-r2/`）：第 4 节写到的 hook 与 JSX 照写进 15 个文件，oxlint 1.86.0 以仓库的配置零警告（4.4）。

## 2. 目标与范围

做 00 号文档 4.2（外框的全部）、4.3 的按钮 30 px 与胶囊形、菜单的子菜单（决定 773 改：没有快捷键的部件），以及"总体设计的修订"第 5 项里 9.2 左栏一段与 9.3 快捷键一句、第 6 项里 13.2 第 16、17、20、26 条中属于 P2 的部分、第 7 项里 13.4 第 3 条中属于 P2 的部分（8.2）。设置模式（4.4）、页面的头与正文（4.5）、首页与其余页面（4.6）不在 P2；P2 留给它们的在第 9 节。

**移交**：认领 [M12-polish](../../v0.2/handoffs/M12-polish.md) 第 14 项"页面树的菜单进三种浏览器"（P2/S2，PG16，00 号文档已认领），与第 1 项里四个 M5-polish 编号：37"页面树的图标再多一组"（S2：普通页一个文件图标）、38.5"树里'目录'与'历史'两行不标当前，也不与页的行对齐"（S2）、26.3"`Ctrl+Shift+F` 没有看得见的提示……可以另给一个键"（S3：框里的 `⌘K` 提示与 ⌘K）、35.7"375 宽下打开文件夹里的页时，树展开把已取得焦点的标题推到视口之下"（S3：窄屏的树在抽屉里，不在页之上；UI1 第三格核对）；第 8 项里的"`onMac` 从 `app/shortcuts.ts` 挪进 `lib/`"（S3：它正改 `shortcuts.ts`，又给 `onMac` 加一个调用者，决定 797）。第 1 项里同在搜索框上的 26.8（列表未到时的路径与提示）与 26.10（框里的 Esc 是浏览器的清空，没有什么把焦点还回 `Ctrl+Shift+F` 拿走它的地方）不认领，照旧留给 v0.2（决定 797）。各 Step 的文档提交在那份移交的"处理进展"写一行（写明 Step）。

### 2.1 对 00 号文档的修订

由一个带 `--check` 与 `--path` 的脚本做（02-workflow 2.3），每处一个带标签的替换，与本文同一个提交；任何一处的旧文字不是恰好出现一次时什么也不写。下面的旧文字在 `4c3f10b9` 的 00 号文档里各出现一次（修订时在 `git archive` 的副本上以 `--check` 与实跑核对过）。

1. **4.2.3 右上角**（决定 769、792）：
   - 开头一句括号里的"外框放进侧栏时设一个标记，`Layout` 看标记"改为"命中带 `handle: { sidebar: true }` 的路由（`:slug`）时不放，工作区外框没有画出侧栏（读取失败、404）、会话不可用的页与错误边界经 `useCornerBack()` 交还；放什么按会话定，会话或账户还在读时什么也不放，[02-P2](02-P2-shell.md) 决定 769、792"；
   - "已登录"一条的"工作区读取中与读取失败（`W/pages/workspace/workspace-layout.tsx:102-104`）"改为"工作区读取失败（`W/pages/workspace/workspace-layout.tsx:102-104` 里带 `error` 的 `NotLoaded`；读取中不放：账户按钮紧接着要挪进侧栏，c6:113 一打开页就点它）"；
   - "**未登录**（登录、注册、未登录的邀请页）："改为"**未登录**（登录、注册、未登录的邀请页）与会话不可用（`W/app/guards.tsx:51,73` 的 `SessionUnavailablePage`）："；
   - 个人设置页一条的"（去最近的工作区，没有时去首页）"改为"（去最近的工作区，没有时去首页；带 `arrived`，`LandingPage` 转交给它的 `<Navigate>`）"；
   - "已登录"一条的"工作区之外与工作区之内的出错页与 404（唯一的错误边界在 `W/app/routes.tsx:40-41`）"改为"工作区之外与找不到工作区的 404、所有出错页（笔记本里的 404 有侧栏；唯一的错误边界在 `W/app/routes.tsx:40-41`）"（审查 C 的 P2C1-N1）。
2. **4.2.2 第 3 条**（审查 C-N10）："（图标加文字；"工作区设置"带 `arrived` 进入设置，4.4.1）"改为"（图标加文字；"工作区设置"P3 起带 `arrived` 进入设置，4.4.1：P2 里设置还在工作区外框之内，侧栏不卸下）"。
3. **4.2.2 第 4 条**（决定 777、778）：
   - "展开的箭头在标题之外，"改为"展开的箭头只是装饰（当前笔记本总是展开的），与书本图标一样 `aria-hidden`、在链接里（链接盖满整行），"；
   - "只用 CSS 把它摆到笔记本那一行的右端"之后加"，齿轮在它左边（与 Tab 的次序相同：齿轮在笔记本行里，＋在其后的树的 `nav` 里；样稿是＋在左）"；
   - "笔记本设置"那一句的"（齿轮，直接链到常规页、带 `arrived`，4.4.1）"改为"（齿轮，直接链到常规页，名为"X 的设置"（`notebook.settingsOf`，与笔记本首页上的"笔记本设置"链接不同名，`members-page.test.tsx:460` 等以名称找后者）；P3 起带 `arrived`：P2 里笔记本设置还在工作区外框之内，齿轮不随之卸下，4.4.1）"；
   - "悬停或键盘聚焦时显出；"改为"悬停笔记本那一行或它们自己、键盘聚焦时显出（同样稿的 `.row:hover`：悬停树的行不显出）；"；
   - "**别的笔记本**只是一行链接，不显示按钮（它们的树没有读，主持者的决定 730）。"的句号之前加"，带 `arrived`：点开之后那一项换成展开的笔记本行，链接随之离开（[02-P2](02-P2-shell.md) 决定 777）"。
4. **4.2.2 第 6 条**（决定 770；审查 C-I5）：整句"个人设置（带 `arrived`）、外观（主题三选一、语言二选一，子菜单里就地切换）、版本行（照旧直接显示在菜单里，s2 一打开菜单就读它，`e2e/stories/smoke/s2-web-app.spec.ts:60`）、退出登录。"改为"个人设置（带 `arrived`）、外观（子菜单：主题三选一、语言二选一，子菜单里就地切换（选了之后菜单不关））、分隔线、退出登录、分隔线、版本行（照旧直接显示在菜单里、在最后，s2 一打开菜单就读它，`e2e/stories/smoke/s2-web-app.spec.ts:60`）。"
5. **4.2.5 第 4 条**（决定 782、790、797、799）：整条"- 在抽屉里点了链接或提交了搜索：抽屉关上；路由状态是 `arrived` 或 `opened` 时，不把焦点还给菜单按钮（照 13.2 第 17 条，焦点归去向页）。"改为"- 在抽屉里点了链接或提交了搜索：抽屉随地址的变化关上，焦点在这次提交里离开抽屉。树里打开别的页的链接（平时 `opened`，连同"目录""历史"两行）在抽屉开着时带 `arrived`，去向页的 `h1` 取焦点；点的是当前的地址时照旧 `opened`（同一页不重新挂载：标题取不取焦点要看原来那一条的路由状态，给菜单按钮才一致）。去向是 `arrived`、带附件的定位（`Located`），或后退、前进到带 `opened` 的一条时，焦点由去向页的标题或附件行取（13.2 第 17 条）；其余的导航（首页、设置、"更多"、切换工作区、当前的地址、在结果页上再提交）与 Esc、点遮罩关上时，焦点给菜单按钮。被"未保存"拦下、选了"留下"时抽屉关上，焦点留在确认框还给的地方（编辑器、横幅或冲突的标题）。Esc 让给输入法的组字与有字的搜索框（浏览器先清空它；不清空的浏览器里照常关上）（[02-P2](02-P2-shell.md) 决定 782、790、797、799）。"
6. **4.3 菜单**（决定 773）："子菜单与快捷键的部件在 P2/S1 随头像菜单加（[01-P1](01-P1-foundation.md) 2.1 第 2 条）。"改为"子菜单的部件在 P2/S1 随头像菜单加（[01-P1](01-P1-foundation.md) 2.1 第 2 条）；快捷键的部件等第一个带快捷键的菜单项（P2 没有，[02-P2](02-P2-shell.md) 决定 773）。""每项可带图标与快捷键"不改（是样子，不是部件）。
7. **第 7 节 P2/S1 一行与 4.4.1**（决定 771、773；审查 C-N17 的锚点）：P2/S1 一行的"；按钮的 30 px 与胶囊形、菜单的子菜单与快捷键（4.3，[01-P1](01-P1-foundation.md) 2.1 第 2 条） | a6、a7、a8、s2、c6、c8 的账户菜单；a8 改从头像菜单切主题；`route-error`、`layout`、`routes` 的 vitest 改写 |"（"菜单的子菜单与快捷键"在 00 号文档里出现三次，P1/S2 一行与第 13 节的是历史、不改，带上前后的字只剩这一处）改为"；按钮的 30 px 与胶囊形、菜单的子菜单（4.3，[01-P1](01-P1-foundation.md) 2.1 第 2 条）；离开工作区外框的菜单项（账户菜单的"个人设置"、切换器的"新建工作区"）带 `arrived`，个人设置页的 `h1` 取到达的焦点（[02-P2](02-P2-shell.md) 决定 771：4.4.1 的"焦点"一条里个人设置的部分由此提前） | a6、a7、a8、s2、c6、c8 的账户菜单；a8 改从头像菜单切主题；`route-error`、`layout-error`、`layout`、`routes`、`profile-page` 的 vitest 改写；se1 在侧栏上的三项下限照 4.7 重量 |"；4.4.1"焦点"一条的"设置页的 `h1` 今天不取到达的焦点，加上 `useArrivalFocus` 与 `tabIndex={-1}`（主持者的决定 745）。"之后加"账户菜单的"个人设置"与个人设置页的 `h1` 在 P2/S1 做（[02-P2](02-P2-shell.md) 决定 771）。"
8. **4.1 的颜色表**（决定 772）：`--accent-soft` 一行（"| `--accent-soft` | 琥珀 12% | 琥珀 14% | 当前页那一行的底、编辑器的选区 |"）之后加一行"| `--avatar` / `--avatar-foreground` | #3f8f74 / #ffffff | 同 | 账户行的圆形头像（样稿 palette.html:90） |"。
9. **4.8 的 PG16**（决定 789；审查 C-N14）："新的、只有页面版本的一格，在三种浏览器里打开树的行菜单、改名、取消、焦点回到按钮（M12-polish 第 14 项）；"改为"新的、只有页面版本的两格，一格在三种浏览器里打开树的行菜单、改名、取消、焦点回到按钮（M12-polish 第 14 项），另一格核对当前行滚进侧栏而窗口不动（[02-P2](02-P2-shell.md) 决定 789、791）；"。
10. **"总体设计的修订"第 5、6 项**（审查 C-N17 的锚点）：第 5 项第二小条的"照 4.5 改写：属性条、可收起的右栏、编辑时的大纲；"之后加"P2 的三个 Step 各改各的那几句见 [02-P2](02-P2-shell.md) 8.2；"；第 6 项的"第 17 条（树的标题即笔记本行的标题；设置的进出与跨布局的链接带 `arrived`；抽屉关上时不还焦点）：P2/S2、P2/S3、P3/S1 各改各的；"改为"第 17 条（树的标题即笔记本行的标题；设置的进出与跨布局的链接带 `arrived`；抽屉关上时焦点的去处）：P2/S1（离开外框的菜单项）、P2/S2、P2/S3、P3/S1 各改各的；"；"第 16 条（"左栏常驻"在设置模式下不成立；树的稳定容器）：P2/S2、P3/S1；"一行之后加一行"第 26 条（侧栏只滚自己把当前行带进可见范围）：P2/S2；"；"写到"顶栏""左栏"的其他条目，由 P2、P3 的设计逐条列出、各自的 Step 改写。"之后加"P2 的逐条列在 [02-P2](02-P2-shell.md) 8.2 第 11 条：第 15、21、30、31 条不改，第 20 条"看得见的搜索框"一句由 P2/S3 改。"
11. **第 10 节**（审查 A-Q1）："| WebKit 没有 CSS 的滚动锚定，属性条（服务端 HTML 的一部分，不是后到的）不受影响；右栏的高度变化不在正文之上 | 无需另做；P4/S1 在 WebKit 上核对锚点跳转 |"改为"| 滚动锚定与属性条、右栏：Playwright 的 WebKit 26 与 Chromium、Firefox 一样做 CSS 的滚动锚定（[02-P2](02-P2-shell.md) 第 1 节与它的审查 A 的探针：窗口滚到 3000 px、读者之上插进 500 px，三种都到 3500），初稿的"WebKit 没有"不成立；属性条是服务端 HTML 的一部分、不是后到的，右栏的高度变化不在正文之上 | P4/S1 在它自己的页上于三种引擎里再核对一次锚定与锚点跳转 |"。
12. **第 12、13 节**（审查 C-N5）：P2 一行改为"| P2 新外框（S1–S3） | [02-P2](02-P2-shell.md) | 设计定稿；S1 实施中 | [设计审查](reviews/P2-design-review.md) | — |"（P1 的先例）；第 13 节末加一行"- 2026-11-23：照 P2 的设计修订（[02-P2](02-P2-shell.md) 2.1；[设计审查](reviews/P2-design-review.md)，主持者的决定 769–800）：右上角由路由与会话定，笔记本行与别的笔记本的链接，账户菜单的次序，抽屉里导航之后焦点的去处，菜单的部件，头像的变量，离开外框的菜单项带 `arrived`，PG16 两格，第 10 节滚动锚定一行，第 11 节决定 731、733 注明由 P2 改。"
13. **第 11 节决定 731、733**（审查 C 的 P2C1-M4：两条的说法已由上面第 5、1 条改过，P3 的设计者按决定读会读错）：731 的"细栏不粘住、不是标题（4.2.5）。RA-I5、N3，RB-Q2。"之后加"（[02-P2](02-P2-shell.md) 决定 790 改"在抽屉里导航之后不还焦点"：焦点的去处见 4.2.5 第 4 条。）"；733 的"P3 之前个人设置页另有"回到工作区"（4.2.3）。RA-I8、M1，RC-I2、Q3。"之后加"（[02-P2](02-P2-shell.md) 决定 769 改"含工作区读取中"：读取中不放；会话不可用时放"外观"，决定 792；见 4.2.3。）"。

### 2.2 需要负责人确认

（无。）

### 2.3 本文的决定（主持者的决定，负责人可以改判）

- **769 右上角由路由定，读取中不放**：`:slug` 路由（`W/app/routes.tsx:129-306`）加 `handle: { sidebar: true }`；`Layout` 以 `useMatches()` 看有没有命中它，命中时不放右上角，除非有人经 `useCornerBack()` 交还（工作区外框读取失败时的 `NotLoaded`、找不到时的 `NotFoundPage`、`SessionUnavailablePage` 与 `RouteError`；上下文在 `W/app/corner.tsx`，决定 793）。右上角里放什么按会话定（决定 792）。工作区读取中不放（2.1 第 1 条改 00 号文档）：账户按钮要么在右上角、要么在侧栏，读完之后它从右上角卸下、在侧栏里挂上，c6:113 与 c8:229 一 `goto` 工作区就点账户菜单，会点在正要卸下的那个上；读取中没有账户按钮，Playwright 等侧栏里那个出现。不用 00 号文档写的"外框放进侧栏时设一个标记"：标记在 effect 里设，第一次绘制时右上角已经画了，会闪一下，账户按钮也照样挂两次。`handle` 在渲染时就知道。根路由的错误边界（`routes.tsx:38`）画的时候没有 `Layout`，也就没有右上角（今天它也没有顶栏），照旧：`layout-error.test.tsx` 守着它（7.1）。00 号文档决定 733 的"没有侧栏的地方（含工作区读取中……）"由此改为不含读取中（2.1 第 1、13 条；审查 C 的 P2C1-M4）。
- **770 账户行与右上角是同一个 `UserMenu`**：`UserMenu({ place })`，`place` 是 `"row"`（侧栏底部，满宽，菜单向上开）或 `"corner"`（胶囊，菜单靠右）；按钮里的头像是显示名的第一个字素（`initialOf`，大写，`aria-hidden`），按钮名照旧只是显示名（`accountMenu` 按名称精确匹配，`e2e/fixtures/auth-pages.ts:46-48`）。菜单：个人设置、外观（子菜单：主题三选一、语言二选一）、分隔线、退出登录、分隔线、版本行。外观里选了之后菜单不关（`onSelect` 里 `preventDefault`）：00 号文档 4.2.2 要的是"就地切换"，主题与语言可以一次换完。"个人设置"这种会离开的项照 `page-tree-item.tsx:450` 的 `chosen` 做法：选了它就在 `onCloseAutoFocus` 里 `preventDefault`，Radix 不在 `setTimeout(0)` 里把焦点还给还留着的触发按钮（右上角从 `/settings/security` 去个人设置时），与 `h1` 取到达的焦点不比先后（审查 A 的怀疑；探针 T7 三种引擎各 15 次，做不做都到 `h1`，是预防）。去处就是当前的地址时（在 `/settings/profile` 上从右上角再选"个人设置"）不设：同一路径的 `replace`，标题的 effect 不重跑，再挡住 Radix 还焦点，焦点就落到 `body`；照树的"同一地址照旧"，让 Radix 还给按钮（审查 A 的 P2A1-N1）。"退出登录"的"留下"照旧把焦点还给这个按钮（`useSignOut`，13.2 第 21 条）。与样稿的两处有意的偏离（审查 B-N6）：账户行上方的 `border-t`（中段会滚，分隔线标出底部不随之滚；样稿没有，00 号文档的示意图有）；头像只取一个字素（"U"，样稿是"UI"：与工作区的方形头像同一个 `initialOf`）。
- **771 离开工作区外框的菜单项带 `arrived`，从 P3/S1 提前**：账户菜单挪进侧栏之后，"个人设置"去的 `/settings/profile` 在工作区外框之外，侧栏连同触发按钮一起卸下，Radix 还焦点时按钮已不在，焦点落到 `body`（13.2 第 17 条："链接随它所在的页一起离开的……带 `arrived`"）。所以 S1 就让它带 `arrived`，`SettingsLayout` 的 `h1`（`W/pages/settings/settings-layout.tsx:27`）加 `useArrivalFocus` 与 `tabIndex={-1}`。切换器的"新建工作区"同理（今天就落到 `body`，新建页的 `h1` 已有到达的焦点），一并带上。右上角的"回到工作区"去 `/`，带 `arrived`，`LandingPage` 转交给它的 `<Navigate>`，最近的工作区首页的 `h1` 取焦点（`/` 本来就落到最近的工作区：00 号文档"去最近的工作区，没有时去首页"）。
- **772 头像的颜色是两个新变量**：`--avatar: #3f8f74`、`--avatar-foreground: #ffffff`，亮暗同值（样稿 `.me .av`，palette.html:90），经 `@theme inline` 映射成 `bg-avatar`、`text-avatar-foreground`。不用 `--accent`（琥珀只标状态，13.2 第 32 条）也不用 `--primary`（工作区的方形头像是墨色，账户的是圆形绿色，两种头像分得开）。白字对它 3.89:1，首字是装饰（`aria-hidden`），不量。
- **773 只加有人用的部件**（P1 决定 747 的延续）：按钮加 `size: "sm"`（`h-7.5 px-3.25 text-compact`）、`"icon-sm"`（`size-7.5`）与 `shape: { default: "", pill: "rounded-full" }`，第一批用处是右上角的账户按钮与外观按钮（S1）、细栏的菜单按钮与搜索框里的提交按钮（S3）；菜单加 `DropdownMenuSub`、`DropdownMenuSubTrigger`（带右箭头）、`DropdownMenuSubContent`，用处是外观。快捷键的部件（`DropdownMenuShortcut`）不加：P2 的菜单里没有带快捷键的项，加了 knip 报出没人用的导出（2.1 第 6 条）。
- **774 侧栏分三段**（`W/app/shell-sidebar.tsx` 的 `ShellSidebar`，P3 的设置目录也用它）：顶部（切换器、搜索）不滚；中段（工作区的 `nav`）是 `data-sidebar-scroll` 的滚动区，`overflow-y-auto`；底部是账户行。宽屏上根 `md:sticky md:top-0 md:h-svh md:self-start`，窗口照旧整页滚动（决定 717）。`overflow-anchor: none` 照旧在根上（探针：粘住之后三种引擎带不带它都一样，留着无害，`:108-109` 的理由在侧栏与页一样高时成立）。中段滚到头的滚轮带着窗口走，同 S1 之前（决定 802 改：初稿的 `overscroll-contain` 在 Chromium 里让没有溢出的中段吃掉滚轮）。S1、S2 里窄屏照旧把侧栏排在页之上（`max-md:border-b`），S3 换成抽屉。
- **775 工作区切换挪出 `nav`**：顶部依次是切换器与搜索，再下是 `nav`（00 号文档 4.2.1 的地标与顺序）；切换器照旧在 `[data-shell]` 里（e2e 以它找切换器，`e2e/fixtures/workspace-pages.ts:50-52`），`workspace-layout.test.tsx:84-98` 改写。
- **776 树的容器由工作区外框按笔记本 id 建，当前那一项按地址收养**（00 号文档决定 730、745 的做法）：`W/pages/workspace/notebook-slot.tsx` 的 `NotebookSlot` 上下文由 `WorkspaceLayout` 给出，`containerFor(id)` 按 id 返回同一个 `document.createElement("div")`（各 id 的容器都留着，决定 798）；`NotebookLayout` 把树门户进 `containerFor(id)`（地址里的 id，照原样）；`NotebookNav` 以地址里的笔记本 id（`useMatch("/:slug/notebooks/:id/*")`，照原样，不转小写：`NotebookLayout` 以 `notebooks.byId(id)` 精确比较，`W/stores/notebook.store.ts:79-80`；审查 B-M3）定"当前"：列表里 id 与它相同的那一项画 `SlotHolder`；列表里没有它时，只有在 `store.unsavedEdit({ notebookId: id })` 时（`NotebookLayout` 留住它的同一条件，`notebook-layout.tsx:63`）才把 `SlotHolder` 放在两组之后，大写的、不存在的 id 什么也不放。`SlotHolder` 在 layout effect 里把容器 `append` 进自己，清理时记下容器里拿着焦点的元素，下一个 `SlotHolder` 挪完之后，焦点空着（`body`）就还给它，并以 `reveal` 的做法（只改 `[data-sidebar-scroll]` 的 `scrollTop`，`intoColumn`，4.2.5）把它带进侧栏的可见范围（审查 B 的怀疑 3：从"我的"挪到"团队"时位置下移）（探针：挪动拿着焦点的节点在三种引擎里都丢焦点，`focus()` 拿得回来；清理在 React 移走那一项的 DOM 之前运行，那时焦点还在容器里）。换组时树开着的那一个菜单跟着按钮走，不另关它（审查 B 的怀疑 2）：`SlotHolder` 挪了容器之后通知，`TreeMenu` 随之重量（决定 800）。"当前"按地址定，不由 `NotebookLayout` 在 effect 里登记：登记要多一次提交，那一次里当前那一项画成链接、紧接着又换掉。收养总在树的 layout effect 之前：`WorkspaceLayout` 先画侧栏的门户、后画 `Outlet`，`:slug` 是懒加载的（`routes.tsx:131`），`Layout` 早一次提交就有了侧栏的位置，同一次提交里 `SlotHolder` 的 layout effect 先跑（探针 `slot-order.cjs`）；所以初稿的"收养之后通知树"（`slot.adopted`、`onAdopted`）在产品里走不到，不要了（审查 C 的过度设计），代码里留一句注释写明这个次序。
- **777 别的笔记本的链接带 `arrived`**：点开别的笔记本，那一项从链接换成展开的笔记本行，点下去的链接卸下，焦点会落到 `body`；带 `arrived`，笔记本首页的 `h1` 取焦点（今天新建笔记本之后就是这样，n1:101）。当前笔记本那一行的链接（去首页）不卸下，照旧不带状态。
- **778 笔记本行**：一行 30 px：`h2`（`tabIndex={-1}`）里是去首页的 `NavLink end`，盖满整行的高度与左边距（审查 B-M5：只有 20 px 能点、⌄ 点了没反应），里面依次是装饰的 ⌄（`aria-hidden`；当前笔记本总是展开的）、书本图标与名称；行尾是齿轮与＋。⌄ 带 `-ml-0.5 -mr-1`、两种行的间距都是 `gap-2.25`，当前笔记本那一行与别的笔记本的 ⌄、书本、名字都在 14 / 33 / 58 px，同样稿（palette.html:76 的 `.row .tw`；审查 B-M1，`row-probe.mjs` 实量）。齿轮在笔记本行里、＋在树的 `nav` 里，DOM 次序是齿轮在前，所以画面上齿轮在左（样稿是"＋⚙"，00 号文档的示意图同；改成与 Tab 的次序一致，2.1 第 3 条）。两个都用 `absolute` 摆在容器（`relative`）的右上角，都是 16 px 的图标（`[&_svg]:size-4`；审查 B-N1），平时 `opacity-40`；悬停笔记本那一行、悬停它们自己、键盘聚焦、没有悬停的设备与窄屏下 100%：同样稿的 `.row:hover .end`（palette.html:79-80），悬停树的行、"更多"时不显出（审查 B-M2）。＋不在行里，所以"悬停那一行"写成容器上的 `group-has-[[data-notebook-row]:hover]/notebook:` 与 `group-has-[[data-notebook-new]:hover]/notebook:`（悬停＋时齿轮也显出，同样稿），行另带 `[@media(hover:hover)]:not-data-home:group-has-[[data-notebook-new]:hover]/notebook:bg-hover` 与同样前缀的 `…:text-foreground`，指针在＋上时行的底与字同悬停那一行（审查 B 的 P2B1-N2），在笔记本首页上（`data-home`）照旧是当前的样子。Tailwind 4.3.3 的 `hover:`、`group-hover:` 在 `@media (hover: hover)` 之下，`group-has-[…:hover]` 不在：行的这两条另套 `[@media(hover:hover)]:`，不然触屏上点过＋之后行的底留到下一次点别处（审查 B 的 P2B1-N3）；齿轮与＋的透明度不用套（没有悬停的设备与窄屏下本来是 100%：`[@media(hover:none)]:opacity-100`、`max-md:opacity-100`）。`row-probe.mjs`、`row-probe-r1.mjs` 在 Chromium 里量过：静止 0.4、悬停行、＋、齿轮都是 1，悬停树的行仍是 0.4，指针在＋上时行的底是 `--hover`、字是 `--foreground`，首页上是 `--active`；行的两条编译在 `@media (hover:hover)` 之下。两个都是 `transition-[opacity,color,background-color] duration-150`（tailwind-merge 以它替换 `Button` 的 `transition-colors`；审查 B-N2）。＋发送中禁用时 `disabled:opacity-25!`（`Button` 的 `disabled:opacity-50` 排在 `opacity-40` 之后、`max-md:` 与 `[@media(hover:none)]:` 又在它之后，不加 `!` 禁用的＋反而更亮、窄屏上看不出禁用；审查 B-N3）。齿轮链到 `…/settings/general`（P2 里笔记本设置还在工作区外框之内，侧栏不卸下，不带状态；P3 带 `arrived`），名为"X 的设置"。只读的读者没有＋，齿轮挪到最右，行的右边距由 `pr-14` 减为 `pr-8`（审查 B 的 P2B1-N5：不减时长名字早约 26 px 截断）。
- **779 当前行滚进可见范围，只滚侧栏自己，每次导航至多一次**：行在 `[data-sidebar-scroll]` 的可见范围之外时只改那个滚动区的 `scrollTop`（上下留 8 px），不调 `scrollIntoView`（它也滚窗口，13.2 第 26 条）；同一次导航（`location.key`）只做一次，在范围里的也算做过；读者自这次导航起在侧栏里有过动作就不做：分批画到的当前行可能晚到，读者那时可能正在滚侧栏。谁定、何时定、看守怎样建，见决定 791；只在地址的路径变了（换了一页）时定，同一页换 hash 或查询参数、`replace` 到当前的地址算同一次导航（审查 B 的 P2B1-M1）。PG16 加一格在三种浏览器里核对（决定 789）。
- **780 树的行照样稿**：每层缩进 1 rem（`paddingLeft: 0.5 + depth` rem：深度 0 的图标在 30 px，深度 1 在 46 px，同 palette.html:83-84）；展开按钮 18 px；普通页一个文件图标（`File`，M5-polish 37），来源根、来源与 `AGENTS` 照旧用各自的图标，图标与名字之间 `mr-2.25`（名字在 55 px，与"目录""历史"两行和样稿相同；审查 B-N5）；树的 `ul` 去掉 `gap-0.5`，行紧挨着（样稿的 `.tree`）；当前页一行 `--accent-soft` 底、图标琥珀色（00 号文档 4.2.2）；悬停与当前的底在行上（链接、展开按钮、菜单按钮同在一个底上），链接仍是行的直接子元素（`treeRow` 以链接的父元素为行，`e2e/fixtures/wiki-pages.ts:28-30`）。"目录""历史"两行换成深度 0 的行，`NavLink end` 标当前（M5-polish 38.5）。"更多"是一排 28 px 的图标链接，图标 16 px（链接带 `[&_svg]:size-4`；审查 B-I2：不带时是 lucide 默认的 24 px），名字在 `sr-only` 里（文字照旧，`notebook-layout.test.tsx:84-93` 读 `textContent`），`title` 给悬停提示（00 号文档决定 727）。两处同名的去处用同一个图标、同一种"当前"的样子（决定 796）。树里失败的提示（`page-tree.tsx:200-209` 的 `Alert`）加 `my-1`：`nav` 没有了间距（今天是 `space-y-1`），`Alert` 自己没有外边距（`W/components/ui/alert.tsx:11`），不加时贴着上下的行（审查 B 的 P2B1-N4）。
- **781 抽屉是同一个元素**（00 号文档决定 731）：`ShellSidebar` 的根在所有宽度下都是它；窄屏关着时 `inert`、`invisible`、`-translate-x-full`，开着时滑入（只有开的时候有 150 ms 的 `translate` 过渡，关上没有动画），盖一层 `--overlay` 的遮罩（淡入）。滑动写成 `max-md:data-[drawer=open]:motion-safe:transition-[translate]`（时长用它自带的 150 ms，不另写 `duration-150`）：`prefers-reduced-motion` 时不滑动（审查 A-M2：初稿的 `motion-reduce:transition-none` 的特异度低、被压过；只把属性放进 `motion-safe:` 也不够，另写的 `duration-150` 留在外面，减少动效时成了 `transition: all 150ms`，`row-probe.mjs` 在 Chromium 里量过：现在的写法减少动效时 `transition-duration` 是 `0s`）。焦点由 Radix 的 `FocusScope`（`radix-ui/internal`）困住：它总是包着根（`asChild`），只切 `trapped` 与 `loop`，不按开关挂上卸下（卸下再挂上会让侧栏的整棵树重新挂载）；`onMountAutoFocus`、`onUnmountAutoFocus` 都 `preventDefault`（不然每次载入页面它都把焦点交给切换器）；`tabIndex={trapping ? -1 : undefined}`（它总给根 `tabIndex=-1`，宽屏上点侧栏的空白处会让根取得焦点，之后"只从无人处取"的标题不再取；`scopeProps` 展开在后，覆盖得了；审查 A-M1，探针 T6）。困住焦点时根是 `role="dialog"`、`aria-modal="true"`、名为"导航"，`dialogOpen()` 由此认得它；平时不是。`trapping` 怎样算见决定 790。不用 Radix 的 Dialog：它关着时卸下内容（00 号文档 4.2.5）。不用 `<dialog>`：`showModal()` 把它放进顶层，宽屏上它不是对话框。
- **782 抽屉里的导航**（2.1 第 5 条）：抽屉随地址的变化关上（决定 790）；树里打开别的页的链接与"目录""历史"两行在抽屉开着时带 `arrived` 而不是 `opened`（两种都在点按时定，不在渲染时：4.2.3、4.3.4；审查 A 的 P2A2-M1），去向页的 `h1` 取焦点；点的是当前的地址时照旧 `opened`（`replace`，同一页不重新挂载）：带上 `arrived` 时标题取不取焦点要看原来那一条的路由状态，原来带 `arrived` 的，`useArrivalFocus` 的 effect 不重跑，焦点落到 `body`（探针 T8 的最小外框就是这样）；原来不带的，`arrival` 由假变真、effect 重跑，同一路径的 `replace` 沿用"已给过焦点"的标记、只从无人处取，导航那次提交已 `blur`，取到 `h1`（审查 C 的 P2C1-N4）；照旧 `opened`、给菜单按钮，两种都一样。焦点的去处见决定 790，被"未保存"拦下时见决定 799。Esc 是根上的原生 `keydown`（抽屉里开着的菜单与对话框门户在 `body` 上，它们的 Esc 不经过根；不看 `defaultPrevented`：根之下没有不走门户、又会 `preventDefault` Esc 的东西，审查 A 的过度设计），让给组字与有字的搜索框（决定 797）；Esc 与点遮罩关上，焦点给菜单按钮。窗口变宽时抽屉关上、不再困住焦点，不动焦点。
- **783 ⌘K 的监听在 `WorkspaceLayout`**（00 号文档决定 732）：新模块 `W/pages/workspace/search-shortcuts.ts` 的 `useSearchShortcuts({ field, actions })`（`actions` 是 `useDrawer` 一生不变的那一份，决定 790），`Mod+Shift+F` 从 `search-box.tsx:93-106` 挪过来，规则不变；`Mod+K` 经 `isMod`，同样不管组合中的键（`composing` 挪进 `shortcuts.ts` 导出）、按住的重复与对话框开着时只 `preventDefault`，另在 `event.defaultPrevented` 或焦点在 `.cm-editor` 里时完全不管（编辑器的 `Mod-K` 是插入链接，`W/editor/source-editor.tsx:238`）。没有搜索框时（工作区读取中、读取失败与 404：`workspace-layout.tsx:102-107` 在 hook 之后提前返回）两个键都完全不管，不 `preventDefault`，也不开抽屉（审查 A-M3：不然读取中按 Mod+K 会让侧栏以开着、困住焦点而焦点不在里面的样子挂上）。`dialogOpen({ besideDrawer: true })` 不把抽屉算作对话框：抽屉开着时两个键照旧聚焦抽屉里的框；窄屏抽屉关着时先打开、再聚焦。
- **784 细栏**：窄屏（`md:hidden`）页面顶部 44 px，不粘住：菜单按钮（幽灵、`icon-sm`，名为"导航"，`aria-expanded`、`aria-controls` 指侧栏的根）与当前位置的文字（地址里的笔记本的名字，不在笔记本里时工作区的名字；一个 `span`，不是标题）。细栏在 `ShellSidebar` 里、`[data-shell]` 之外（w3 数 `[data-shell]`，`w3-shell-switch.spec.ts:18-22`）。
- **785 搜索框换成胶囊**：32 px、`rounded-full`、`bg-surface`（暗色 `bg-surface-2`），照 palette.html:71-74；放大镜就是"搜索"按钮（幽灵、`icon-sm`、胶囊，摆在框里的左端，图标 16 px：`Button` 的 `[&_svg]:size-4` 压过写在图标上的大小，不另写；DOM 上在输入框之后，Tab 次序照旧是框、按钮），按钮放在一个与框同色的垫底里（决定 794）；右端是 `aria-hidden` 的 `⌘K`（其他系统 `Ctrl K`）提示，`font-sans`（Tailwind 的 preflight 给 `kbd` 等宽字体；审查 B-N4），窄屏不显示；占位"搜索"；范围说明改成 `sr-only`（`aria-describedby` 照旧，se2:209,225,274 以 `toHaveAccessibleDescription` 核对它）；`aria-keyshortcuts` 列两个键（M5-polish 26.3）。图标在 19 px、占位字在 43 px，同样稿（按钮 `left-1.75`、框 `pl-8.75`；审查 B-N5）。Tailwind 4.3.3 的探针：第 4 节写的全部类名（`h-7.5`、`w-63`、`size-5.5`、`gap-2.25`、`px-3.25`、`leading-4.25`、`max-md:data-[drawer=closed]:-translate-x-full`、`max-md:data-[drawer=open]:motion-safe:transition-[translate]`、`group-has-[[data-notebook-row]:hover]/notebook:opacity-100`、`not-data-home:group-has-[[data-notebook-new]:hover]/notebook:bg-hover`、`disabled:opacity-25!`、`[@media(hover:none)]:opacity-100`、`dark:peer-focus-visible:bg-surface`、`[overflow-anchor:none]` 等）都产出 CSS；`max-md:` 是 `@media (width < 48rem)`，与 `useNarrow` 的查询相同。
- **786 快速切换换装**：选项照旧是两个 `span`（标题、上级位置；`quick-switch.test.tsx:18-22,164` 与 e2e 的 `quickSwitchFor`（`e2e/fixtures/wiki-pages.ts:181-190`）按 `span` 读），文件图标是裸的 `svg`，两段放在一个 `div` 里；当前项照旧带内嵌的环（P1 决定 760）；底部一行键盘提示（"↑↓ 选择 · ↵ 打开 · Esc 关闭"）`aria-hidden`：组合框的键读屏已经知道，读出来是噪音。
- **787 产品名在 P2 与 P5 之间只在账户菜单的版本行与标签页标题里**：顶栏去掉之后，登录、注册、引导页暂时没有产品名，P5/S1 把它放到卡片上（00 号文档 4.2.1）。两个 Phase 之间的中间状态，不另加。
- **788 本机门禁的写法**（P1 决定 761 照旧，由决定 795 细化）：每个提交经 `heavy.sh` 跑 `make lint-web knip test-web`（06-environment 第 3 节：逐个提交的核对也经它，`test-web` 里有计时的断言；审查 C 的 P2C1-M2），改了行为、夹具或故事的另经它跑完整的 `make e2e`；合并之前在并入 `m13-ui-refresh` 的树上带 `CI=1` 经 `heavy.sh` 跑网页一侧的全部与 `make e2e`；Go 一侧不重跑（7.3 的断言）；本机允许的失败要与基线同行同消息，别的红了先查清。
- **789 新故事的格**：PG16 两格（树的行菜单；当前行滚进侧栏，三次导航），UI1 三格（抽屉的开关与焦点；⌘K 与在抽屉里搜索；在抽屉里开页），都在三种浏览器里，只有页面版本。PG16 的第二格放在 PG16：它核对的也是树在三种引擎里的行为（侧栏的滚动区与窗口），单开一个故事只为一格不值。
- **790 抽屉的困住在渲染时推出，焦点在 passive effect 里给**（审查 A-I1 = C-I1；取代初稿 4.3.4 的 layout effect）：打开时记下 `openedAt = location.key`（`useDrawer` 的状态是 `{ at, to } | null`），`trapping = narrow && drawer !== null && drawer.at === location.key`；`inert`、`data-drawer`、`role="dialog"`、`aria-modal`、名字、`trapped`、`loop`、`tabIndex`、遮罩与细栏按钮的 `aria-expanded` 都按 `trapping` 算。导航的那一次渲染里 `trapping` 已经为假，`FocusScope` 拆掉困住的 passive 清理排在去向页 `h1` 取到达焦点的 passive effect 之前（React 先跑完全部 passive 的清理，再跑挂载）；初稿在地址变了的 layout effect 里 `setOpen(false)`，React 处理这个同步更新之前先冲掉上一次提交的 passive effect，`h1.focus()` 在困住还在时就跑了、被拉回抽屉，接着抽屉 `inert`，焦点落到 `body`。导航那次提交的 layout effect 只做一件事：焦点还在抽屉里时 `blur()`，交给 `body`（不 `setState`；Radix 不理去向为空的 `focusout`）：Chromium 到下一次渲染才修正 `inert` 子树里的焦点，不先交出去，后退到带 `opened` 的一条时标题"只从无人处取"取不到（探针 T9）。之后在 `useDrawer` 的 passive effect 里（`useDrawer` 由 `WorkspaceLayout` 调用，是去向页的祖先，它的 passive effect 在页的之后跑）把状态清掉（`setDrawer(null)`，后退回到打开时的那一条不会再打开），并在去向页自己不取焦点时把焦点给菜单按钮。自己取的（`W/app/arrival.ts` 导出的 `headingTakes(state, type)`）是三种：带 `arrived` 的；后退、前进（`POP`）到带 `opened` 的一条（这两种同 `useArrivalFocus` 的 `arrival` 与 `fromNoOne`）；带附件定位（`Located`）的一条，不论导航的类型（附件行或"找不到"的标题取，`W/pages/page/attachments-section.tsx:531-537`、`W/pages/not-found.tsx:22`；从抽屉里只有后退、前进走得到它：初稿算了它，修订以为没有用处去掉了，审查 A 的 P2A1-N2 指出后退到它时菜单按钮先拿走焦点，附件行就不取了）。审查 C 担心的"passive 的给菜单按钮与 `h1` 抢"在这条规则下不会发生：去向会取焦点的导航，侧栏不动焦点；不会取的，没有标题来抢；把规则改坏成"总给菜单按钮"（变异 39）时，`WorkspaceLayout` 的 effect 在 `h1` 的之后跑，菜单按钮赢，测得出（`probe-c/zz-focus-790.test.tsx` 第 4 格，`run-rev.mjs` 的 T12）。Esc 与点遮罩（没有导航）：`close()` 记下"给菜单"、`setDrawer(null)`，`[trapping]` 的 passive effect 在困住拆掉之后 `menu.current.focus()`。`useDrawer` 返回一份一生不变的 `actions`（`closed`、`open`、`close`、`quietClose`、`inDrawer`，`useMemo` 没有依赖），它们经一个 ref 读当前的 `location.key`、`trapping` 与 `narrow`（ref 在每次提交的 layout effect 里更新）；`useSearchShortcuts` 只拿 `actions`，依赖 `[mac, field, actions]` 一生不变，监听只注册一次。读渲染时的值的话，应用内导航之后 `closed()` 照旧为真、`open` 记下旧的 key，`trapping` 恒为假，按键被 `preventDefault` 而抽屉不开（审查 A 的 P2A1-M2，探针 `probe-a-r1`）。不把整个返回值交给它：那个对象带着 `narrow`、`trapping`，每次开关都新建，写进依赖（零警告的 lint 要它写进去）之后监听随之重新注册，读渲染时的值也不会过时，变异 64 就成了等价的（审查 C 的 P2C2-M1）。打开时的聚焦同样在 `[trapping]` 的 passive effect 里（困住的监听挂上之后，`FocusScope` 也记下了这个焦点，审查 A-N3 随之消失）。不用"在根上捕获点击、先 `flushSync` 关上"的做法：链接的导航在点击之后、在路由的过渡里提交，捕获阶段关上之后还有一次导航的提交。探针（审查 A 的 `app.js` 加 `variant=rev`）：T1–T7 与新加的 T8–T14 在三种引擎与 jsdom 里都照上面说的走（第 1 节末）。00 号文档决定 731 的"在抽屉里导航之后不还焦点"由此改为上面的规则（2.1 第 5、13 条；审查 C 的 P2C1-M4）。
- **791 当前行的"滚不滚"由 `PageTree` 自己的 layout effect 定，看守按导航懒建、只看侧栏里的输入**（审查 B-I1 = C-M12）：初稿让变成当前的行在自己的 layout effect 里调 `reveal`，它先于父的 `PageTree` 跑，读到的是上一次导航的看守，`end()` 的也是它，`PageTree` 随后建的新看守没人结束；`watchReader()` 又算整页的任何按键与点击。从第三次导航起，读者在上一页按过键、点过鼠标，就不再滚（审查 B 的两个探针都是"4 次里滚 2 次"）。改为：行只登记自己的链接（当前时在 layout effect 里 `context.show(link)`，清理时撤下）；`PageTree` 的 layout effect（父的在子的之后跑）在 `location.key` 变了时调 `reveal()`；在这次导航里已经调过之后才挂上的行（分批画到的、树后读到的）由登记直接调 `reveal()`。`reveal()` 先看看守：要定的那一次导航（`at`）与看守的不同就结束旧的、按它建新的（`watchReader({ within: "[data-sidebar-scroll]" })`：只算落在侧栏滚动区里的滚轮、按键、按下与触摸的移动，`W/pages/page/readers-input.ts` 的 `watchReader` 加这个选项）；再看有没有登记的行、看守有没有动过、行在不在滚动区里（容器总已收养，决定 776），滚完记下它并结束看守。`location.key` 的 ref（`key`）在 `PageTree` 的 `useInsertionEffect` 里更新，不在渲染时（审查 C 的 P2C2-M1：oxlint 的 `react/refs` 不许渲染时写 ref）：insertion effect 在这次提交的全部 layout effect 之前运行，行的 layout effect 读得到这次的 key；没有提交的渲染（挂起的过渡）不动它，渲染时写的反而会让先提交的一块行读到没提交的 key（探针 `probe-r2/insertion.cjs`）。只在路径变了时定（审查 B 的 P2B1-M1）：`PageTree` 的 layout effect 在 `location.pathname` 与上次的不同时，才把这次导航记成要定的那一次（`at`），之后总调 `reveal()`（对已定过的 `at` 什么也不做，看守也不重建；同一路径时提前返回的话，行恰好画在换 hash 的那次提交里、或换 hash 的渲染被挂起时先提交的一块行里，之后没人再调，审查 B 的 P2B2-N1）；同一页换 hash（点右栏的大纲，`W/pages/page/page-outline.tsx:45`，或后退、前进到同一页的另一个 hash）、换查询参数、`replace` 到当前的地址，都算同一次导航，看守与"至多一次"跟着它：不然读者把侧栏滚开、再点大纲里的一个标题，新的 key 新建一个没动过的看守，侧栏被拉回当前行（审查 B 的 `reveal-791-hash.cjs`）；树还没读到时换了 hash，行到了照旧由 `show` 调 `reveal()`。探针 `reveal-791.cjs`（照审查 B 的 `reveal-order` 改）：收养之后五次导航（其间在主区按键、点树的行、在侧栏里按 ↓），有行的五次都滚；在侧栏里滚轮之后晚到的行不滚，在主区滚轮之后照滚。探针 `reveal-r1.cjs`：同一页换 hash、回到不带 hash、`replace` 到当前的地址都不动侧栏；树没读到时换 hash，行到了照滚；之后开别的页照滚。
- **792 右上角按会话定**（审查 A-M4、C-M1、C-Q1）：`Corner` 读 `useSession()`（挪进 `W/app/session-state.ts`，决定 793）：`signed-out` 与 `unavailable` 放"外观"；`signed-in` 时账户读到了（`account.me`）放账户按钮（在 `/settings/*` 上前面另有"回到工作区"），账户读取失败也放"外观"；`starting` 与账户读取中什么也不放。账户还没读到时 `Corner` 自己以守卫的同一个 SWR 键读它（`useSWR(loading ? "me" : null, () => account.load())`，`W/app/guards.tsx:65`）：错误边界在守卫之上（`routes.tsx:41`），首次载入就出错时守卫没画、`me` 从没读过，按 `me` 判断会让已登录的人只看到"外观"（审查 C 的探针）；同一个键在 SWR 里只发一次，读到之后键是 `null`，平常的页上不多读（探针 `zz-corner-792.test.tsx`：首次载入就出错时右上角是"Ada"、`GET /me` 一次，平常的页上 2.5 秒里不多读，`GET /me` 答 500 时是"外观"）。`SessionUnavailablePage`（`W/app/session-unavailable.tsx:39`）调 `useCornerBack()`：`:slug` 的地址上会话不可用时，今天的顶栏有主题与语言，交还之后右上角照旧有"外观"。
- **793 右上角的上下文在 `W/app/corner.tsx`**（审查 C-I3）：`CornerBack`（上下文）与 `useCornerBack` 放进自己的模块，`RouteError`、`SessionUnavailablePage`、工作区外框从那里引；`Layout` 给出它、画 `Corner`。`layout-error.test.tsx:7-11` 把 `./layout` 整个换成只有 `Layout` 的工厂，`RouteError` 若从 `./layout` 引 `useCornerBack`，vitest 抛"No "useCornerBack" export is defined on the "./layout" mock"（审查 C 的原型里实测如此）；放进自己的模块之后那个 mock 不用改。`layout-error.test.tsx:18` 的 `queryByRole("banner")` 在 S1 之后恒为空，改成"没有名为 Appearance 的按钮，也没有账户按钮"：根路由的错误边界没有右上角（决定 769；审查 A-M6）。`useSession` 从 `guards.tsx:19-23` 挪进新的 `W/app/session-state.ts`（`guards.tsx`、`W/app/event-stream.tsx:6`、`W/pages/invitation.tsx:9` 与 `corner.tsx` 从那里引）：`corner.tsx` 若从 `guards.tsx` 引它，`guards.tsx` → `session-unavailable.tsx` → `corner.tsx` → `guards.tsx` 成了导入的环（审查 A 的未核实）。运行时不出错（用到的都是调用时才取的函数），oxlint 也不报（启用的类别里没有 `import/no-cycle`：设计者以一个两个文件的环核对过，加 `-D import/no-cycle` 才报），但环让模块的初始化次序取决于谁先被引，不留。
- **794 搜索按钮的底是一个与框同色的垫底**（审查 B-M4 = C-M9）：`e2e/fixtures/contrast.ts` 顺着祖先找第一个不透明的底（`under(element.parentElement)`），初稿的按钮与输入框是兄弟，se1 的"button"一项量到的是 `--sidebar`（亮 2.84、暗 8.41），画出来的却是在框的底上（亮 3.10、暗 6.95）。主持者给的做法是把胶囊的底放到包着框与按钮的那层上、框透明：按钮量对了，可框与无效时的环画在框外、落在侧栏上，`under(input.parentElement)` 却读到那一层，"box"与"invalid"两项反而量错（亮 3.10、4.38，暗 7.31、6.33；实际是 2.84、4.02、8.41、7.28）。所以改为：框照旧自己带底，按钮放进一个垫底的 `span`（`absolute top-1 left-1.75 grid size-6 rounded-full bg-surface transition-[background-color] duration-150 dark:bg-surface-2 dark:peer-focus-visible:bg-surface`，框带 `peer`），垫底与框同色、跟着框的聚焦换色（框聚焦时自己的底是 `--surface`，`Input` 的 `focus-visible:bg-surface`），看不出来。三项都量的是画出来的那一对：`se1-pill.mjs` 在 Chromium 里照 `contrast.ts` 的 `measure` 量了三种结构，垫底的这种亮色 box 2.84、button 3.10、invalid 4.02，暗色 8.41、6.95、7.28。S3 照 00 号文档 4.7 的规则重量"button"一项（亮下限由 S1 的 2.74 升到 2.99（3.0977 − 0.1 = 2.9977，向下取到 0.01；审查 B 的 P2B1-N1），暗由 8.30（8.4057 − 0.1 向下取到 0.01；审查 C 的 P2C2-N3）降到 6.85：被量元素的底换了，00 号文档 4.7 的"改了被量元素的底的 Step 重量"），"box"与"invalid"不变；记录写明 S3 起"button"量的是垫底。
- **795 P2 的本机门禁**（细化决定 761、788；审查 C-M6、M7、M8、M10）：(a) S1 开工之前，在没改的 `m13-ui-refresh` 头上经 `heavy.sh` 跑一次 `CI=1 make e2e` 的基线，记下每一格失败的文件与行（Playwright 报告里错误所在的代码位置，例如 `se2-….spec.ts:230`，不是 `test(…)` 的开头行：同一格里更早的一步坏在同一种断言上时，开头行与消息都不变，分不出来；审查 C 的 P2C1-M3）与消息的第一行，写进 S1 的记录，第 1 节的三格随之改写；之后允许的失败只能是与基线同一行、同一消息的那几格，换了行或消息就算新的红，照 02-workflow 3.1 先停、查清。(b) 每个提交的 `make lint-web knip test-web` 经 `heavy.sh`（决定 788）；改了行为、夹具或故事的每个提交另经它跑完整的 `make e2e`（本机约 5 分钟：`make build` 与 Playwright 各约 2.5 分钟，P1/S2 的记录；初稿只算了 Playwright），不再手挑"改到的故事"（审查 C 列出了初稿漏掉的二十多个）；只改文档的提交不跑。(c) 合并之前的整条命令经 `heavy.sh`（`test-web` 里的 `markdown.test.ts`"not slower"对负载敏感，06-environment 第 3 节）。(d) se2 的 page 格在 macOS 上于 se2:230 的方向键那一步失败，之后的步骤（含 :274 的访客说明）本机从来不跑，P2 里只靠 Linux 的 CI；`Control+K` 在 e2e 里只有 Linux 的 CI 走（本机 macOS 走 `Meta+K`），本机由 vitest 走（jsdom 的 `navigator.platform` 不是 Mac，`onMac()` 为假，Mod+K 的格按 Ctrl+K；Cmd+K 另有一格照 `search-box.test.tsx:212-214` 把平台打桩成 Mac）。两件都写进 S3 的记录，负责人推送之后看 CI。
- **796 LLM Wiki 的"目录""历史"两处都留**（审查 B-Q1）：树里的两行（LLM Wiki 的读者从树上直接到它们，M12/P2/S1）与"更多"里的图标都留着；同一个去处在两处用同一个 lucide 图标（目录 `List`、历史 `History`：初稿"更多"里的 `Clock` 换成 `History`），"当前"只有一种样子，照树的当前行：`--accent-soft` 底、琥珀图标（"更多"的当前项由 `bg-active text-accent` 改为 `bg-accent-soft [&_svg]:text-accent`，健康、回收站同）。
- **797 搜索框上的移交与抽屉的 Esc**（审查 C-Q2、C-Q3）：M12-polish 第 1 项里的 26.8 与 26.10 不认领，照旧留给 v0.2（第 2 节"移交"；S3 的"处理进展"写明）。抽屉的 Esc 让给输入法的组字（`composing(event)`：`isComposing` 或 229）与有字的搜索框：焦点在 `type="search"` 的框里、框里有字时，抽屉这次不关，浏览器先清空它；浏览器没清空时（`setTimeout` 里框还是有字）照常关上：Playwright 的 WebKit 不清空（探针 T10），只是不关的话，Esc 在那里就失了效。M12-polish 第 8 项里的"`onMac` 从 `app/shortcuts.ts` 挪进 `lib/`"由 S3 认领：它正改 `shortcuts.ts`，`search-shortcuts.ts` 又给 `onMac` 加一个调用者；挪到 `W/lib/platform.ts`，五个调用者改引入（`search-box.tsx:72`、`quick-switch.tsx:35`、`page-edit.tsx:393`、`page-layout.tsx:322`、`native-select.tsx:42`），新的 `search-shortcuts.ts` 也从那里引；`shortcuts.ts` 自己不调用它（`isMod`、`isModShift` 收的是 `mac` 参数，`shortcuts.ts:15-27`），挪走之后不再引它（审查 C 的 P2C1-N2：照初稿的写法会留下一个没用的导入）；`shortcuts.test.ts` 里 `onMac` 的格挪到 `W/lib/platform.test.ts`。
- **798 容器按笔记本 id 各留一个，渲染时不淘汰**（审查 B 的怀疑 1）：`containerFor` 在一个 `Map` 里按 id 留着各自的 `div`，不在渲染时去掉别的 id 的（初稿"只留当前的一个"）：去 B 的过渡渲染被打断、放弃，`NotebookLayout(A)` 又以更高的优先级重画时，"只留一个"会给 A 新建一个容器，A 的树门户进一个脱离文档的 `div`，侧栏里的树不见、重新挂载。空的 `div` 很便宜，一个工作区外框的一生里至多每本笔记本一个。`keepFocus`、`takeFocus` 照旧按 id（审查 B 说"同时只留一个容器，一个字段就够"，各 id 都留着之后按 id 存取正合用）。
- **799 被"未保存"拦下、选了"留下"时抽屉安静地关上**（审查 A 的 P2A1-M1）：窄屏、编辑中有未保存的改动，在抽屉里点树的另一页，`UnsavedGuard` 的确认框拦下导航（`W/pages/page/unsaved-guard.tsx:55-69`）；它的 `FocusScope` 暂停抽屉的那一层，"留下"时 `onCloseAutoFocus` 调 `stay()`，把焦点给 `main` 里的编辑器、横幅或冲突的标题（`page-edit.tsx:207-213` 的 `back()`），之后 Radix 才恢复抽屉那一层（`@radix-ui/react-focus-scope` 的 `index.mjs:94-102`），困住只对之后的焦点事件起作用：抽屉仍开着、遮罩还在，焦点却在它后面，打的字进了被遮住的编辑器。初稿就有，修订核对时才看到。所以 `useDrawer` 的 `actions` 加 `quietClose`（`setDrawer(null)`，不设 `toMenu`，焦点不动），经 `W/app/shell-sidebar.tsx` 的 `Drawer` 上下文（`WorkspaceLayout` 给出 `drawer.actions`；默认值什么也不做：不在工作区外框之内时）与 `useDrawerActions()` 交给外框之下；`UnsavedGuard` 在"留下"那一支里先调它、再 `stay()`（Esc 关上确认框也是"留下"，同一支）。抽屉关着时调它什么也不变。不选 A 的另一个建议"在抽屉里点链接时，若有拦截在，先关上抽屉"：抽屉不知道有没有拦截（`useBlocker` 在页里），选了"离开"时也不该先关（之后焦点由去向页取，决定 790）。探针 `probe-a-r1/run.mjs`（A 的 `r1` 加这个上下文）：三种引擎里"留下"之后抽屉关上、焦点在编辑器；不加时抽屉开着、焦点在被遮住的编辑器上（A 的复现）。`shell-drawer.test.tsx` (7) 一格，变异 63。
- **800 换组时树的菜单由 `SlotHolder` 通知重量**（审查 C 的 P2C1-M1；取代决定 776 原来靠的"`close` 每次渲染新建"）：`TreeMenu` 每次自己提交之后重量按钮（`page-tree-item.tsx:389-406`），换组时它跟着重画只是顺带的：列表换新给 `PageTree` 新的 `notebook`，`close` 与 `create`（`page-tree.tsx:133-136,222`）每次渲染新建，`observer` 自带的 `memo` 才放它过去。初稿的变异 51 只稳住 `close`，`create` 照样新建，(2c) 抓不到，是等价的变异。改为明说：`Slot` 加 `moved()`、`moves()`、`subscribe(listener)`（整个工作区外框一个计数）；`SlotHolder` 的 layout effect 在容器挪进来之前已有父元素（上一个持有者的、已离开文档的 `div`）时，挪完调 `moved()`，第一次收养不调；`notebook-slot.tsx` 导出 `useSlotMoves()`（`useSyncExternalStore(slot.subscribe, slot.moves)`），`TreeMenu` 调它：计数变了，它在同一次提交里、绘制之前同步重画（layout effect 里触发的外部存储更新），没有依赖的那个重量的 layout effect 随之量到新位置。上下文的默认值是一个从不挪动的 slot（`moves()` 恒为 0）。`PageTree` 的回调照旧每次渲染新建，不去稳住（主持者写了"之后回调可以稳住"）：列表整个重读时（`notebook.store.ts:96`，全部换成新的对象）同一组里上面多了或少了一本，容器不挪、`SlotHolder` 不知道，按钮却移了，今天（M12）靠的就是这次顺带的重画，稳住回调会丢掉这一条。经 `put`、`gone`（`:314-321`：新建、改别本的名、删除、离开、接管、升级之后的那一次）时别的笔记本留着原对象，当前那本只是下标变了，`PageTree` 不重画，树的菜单开着时同一组里的这种位移今天就不重量，P2 照旧：不是回归，接受（审查 B 的 P2B2-N3 与它的 `put-identity.cjs`；B 另提的"在列表的组件里随列表变化调 `slot.moved()`"不做，机制照本条）。所以 (2c) 不经列表换新：在 `act` 里以 `runInAction` 就地改那本笔记本的 `workspace_access`，同一个对象，`PageTree` 不读这个字段（`writesNotebook` 读的是 `role` 与 `upgrade`，`W/app/effective-role.ts:51-53`）、不重画，只有两组的列表读它（`W/stores/notebook.store.ts:22-24`），`TreeMenu` 只能由通知重画；变异 51 改为"`SlotHolder` 挪了容器之后不通知"，(2c) 抓得住。探针 `probe-b/slot-move-r1.cjs`（jsdom，`TreeMenu` 是 `observer`，`NotebookLayout` 是 `memo`）六种情形：列表换新、回调每次新建时，通知与否都量到新位置（不通知时靠重画）；列表换新、回调稳住时，不通知不量（审查 C 担心的将来）、通知量到；就地改时，通知量到，不通知不量（变异 51 被抓）。
- **801 窄屏的账户菜单把外观直接列在菜单里**（改 770；实施者 375 宽的截图）：md 以下（`(width < 48rem)`，与 Tailwind 的 `max-md:`、S3 的抽屉同一个查询）账户菜单是：个人设置、分隔线、"主题"标签与三个选项、分隔线、"语言"标签与两个选项、分隔线、退出登录、分隔线、版本行；选项照旧 `preventDefault`，选了菜单不关；md 起照旧是子菜单。375 宽时账户行的菜单与行同宽（359 px），子菜单至少 200 px，左右都放不下，Radix 照样把它向右开（`@radix-ui/react-menu` 的 `index.mjs:741`），开到屏幕之外，指针与触摸都选不了主题与语言。两套按条件渲染，不靠 CSS 藏起其中一套。`useNarrow()`（`W/app/narrow.ts`：`useSyncExternalStore` 加 `matchMedia("(width < 48rem)")`，没有 `matchMedia`（jsdom）时为假）与 `narrow.test.ts` 由此在 S1 建好，S3 的抽屉只加调用者（4.3.1）。
- **802 中段不再 `overscroll-contain`**（改 774；审查 A 的 S1A-M2）：中段没有溢出时（多数工作区首页与小笔记本），Chromium 把停在中段上的滚轮吃掉，窗口不滚（A 的探针 E1；Firefox、WebKit 照滚）：Blink 把没有溢出的 `overflow-y: auto` 也算进滚动链，`contain` 在那里截断了它；S1 之前侧栏不是滚动区，三种都滚窗口。去掉之后三种引擎在中段上都滚窗口；中段溢出、滚到头的滚轮带着窗口走，同 S1 之前（Chromium、WebKit 在同一次滚动里就带，Firefox 另起一次滚动时才带）。第 0 步 (b) 的"侧栏滚到底、窗口不动"由本条取代（8.1 第 4 条、10.1）；A 的 S1A-N1（Firefox 另起一次才带）随之了结。审查 A 给的两种做法（只在中段溢出时 `contain`；去掉）里取后一种。
- **803 新建工作区页也有"回到工作区"**（审查 A 的 S1A-Q1）：账户有工作区时，`/create-workspace` 的右上角在账户按钮之前也放"回到工作区"（同一个链接：`to="/"`、`arrived`，`LandingPage` 转交给它的 `<Navigate>`）：从切换器点了"新建工作区"又不想建的人有路回去（旧顶栏的产品名链接去 `/`）。`Corner` 在这一页以 `"workspaces"` 键读列表（与落地页、工作区外框同一个键，多半已经读过）；没有工作区时不放：`/` 会又落回这一页。P3 给设置页侧栏之后，个人设置页的那一个去掉，这一个留着（第 9 节）。已登录的邀请页与出错页回去的路留给 P5（产品名与标志的卡片，决定 787）。
- **804 树里父页的展开按钮伸进缩进 4 px**（改 780 的摆放；审查 B 的 S2B-N2）：展开按钮另带 `-ml-1 mr-1`，› 落在 14–28 px，与笔记本行的 ⌄ 同列，深一层右移 16 px；`mr-1` 补回左移的 4 px，链接、页的图标（深度 0 在 38 px）、叶子的空位、落点线与更深的缩进都不动。样稿的树把 › 放在 36 px，并不与笔记本行对齐；这 4 px 之差来自 780 把展开按钮放进缩进的空位。B 说可不改、看图定，主持者取对齐。

## 3. 服务端

无。

## 4. 网页

### 4.1 S1 外框与账户行

#### 4.1.1 文件

| 文件 | 改什么 |
|---|---|
| `W/app/layout.tsx` | 去掉顶栏（`:38-47`）；给出 `CornerBack`、画 `Corner`（4.1.2） |
| `W/app/corner.tsx`（新） | `CornerBack`、`useCornerBack`、`Corner`（决定 792、793） |
| `W/app/session-state.ts`（新） | `useSession` 从 `guards.tsx:19-23` 挪来；`guards.tsx`、`W/app/event-stream.tsx:6`、`W/pages/invitation.tsx:9` 改引入（决定 793：不然 `corner.tsx` 与 `guards.tsx`、`session-unavailable.tsx` 成环） |
| `W/app/routes.tsx:129-130` | `:slug` 路由加 `handle: { sidebar: true }` |
| `W/app/route-error.tsx`、`W/app/session-unavailable.tsx:39-43` | 调 `useCornerBack()`（从 `./corner` 引） |
| `W/app/user-menu.tsx` | `place`、头像、外观子菜单、"个人设置"带 `arrived` 与 `chosen`（4.1.3） |
| `W/app/appearance-menu.tsx`（新） | `AppearanceItems`、`AppearanceMenu`（4.1.3） |
| `W/app/theme-menu.tsx`、`W/app/language-menu.tsx` | 删掉 |
| `W/app/initial.ts`、`W/app/initial.test.ts`（新） | `initialOf` |
| `W/app/shell-sidebar.tsx`（新） | `ShellSidebar` 三段（4.1.4） |
| `W/pages/workspace/workspace-layout.tsx:108-129` | 用 `ShellSidebar`；导航行加图标；读取失败与 404 时 `useCornerBack()`（4.1.4） |
| `W/pages/workspace/workspace-switcher.tsx:36-39,56` | 头像与样子；"新建工作区"带 `arrived`（4.1.5） |
| `W/components/nav-item.tsx:17` | 新的样子（4.1.5） |
| `W/pages/workspace/notebook-nav.tsx:23-45`、`notebook-groups.tsx:12-20,66` | 分组标题、列表与"新建笔记本"的样子；`NotebookGroups` 加 `groupClassName`（4.1.5） |
| `W/pages/workspace/workspace-home.tsx:36-39` | 传 `groupClassName="space-y-2"`（照旧的样子） |
| `W/pages/settings/settings-layout.tsx:27` | `h1` 取到达的焦点 |
| `W/components/ui/button.tsx:17-22` | `sm`、`icon-sm`、`shape`（决定 773） |
| `W/components/ui/dropdown-menu.tsx` | 子菜单的三个部件（4.1.6） |
| `W/styles.css` | `--avatar`、`--avatar-foreground` 与映射（决定 772） |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | `appearance.label`（Appearance / 外观）、`shell.backToWorkspace`（Back to the workspace / 回到工作区） |
| `W/app/narrow.ts`、`W/app/narrow.test.ts`（新） | `useNarrow()`：md 以下账户菜单把外观直接列在菜单里，S3 的抽屉也用它（决定 801） |
| `W/pages/landing.tsx` | 读工作区列表失败时交还右上角（index 路由带 `handle`，审查 A 的 S1A-M1） |
| vitest | 7.1 的 S1 各行 |
| `e2e/stories/identity/a8-profile.spec.ts:81-89`、`e2e/stories/search/se1-search.spec.ts:281-288` | 第 5 节 |
| `docs/v0.1/v0.1-design.md` | 8.2（Step 的文档提交里改） |

#### 4.1.2 `Layout` 与右上角

```tsx
export function Layout() {
  useArrivalWatch();
  const [column, setColumn] = useState<HTMLElement | null>(null);
  const [given, setGiven] = useState(0);
  const giveBack = useCallback(() => {
    setGiven((n) => n + 1);
    return () => setGiven((n) => n - 1);
  }, []);
  const sidebar = useMatches().some((match) => (match.handle as Shell | undefined)?.sidebar === true);
  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <FileDropGuard />
      <div ref={setColumn} className="contents" />
      <div className="flex min-w-0 flex-1 flex-col">
        {(!sidebar || given > 0) && <Corner />}
        <main className="flex min-w-0 flex-1 flex-col p-6">
          <CornerBack value={giveBack}>
            <ShellColumn value={column}>
              <Outlet />
            </ShellColumn>
          </CornerBack>
        </main>
      </div>
    </div>
  );
}
```

- `W/app/corner.tsx`（决定 793）：`CornerBack`（上下文，默认值是什么也不做的函数：根路由的错误边界在 `Layout` 之外用到它）、`useCornerBack(on = true)`（`useLayoutEffect(() => (on ? giveBack() : undefined), [on, giveBack])`：layout effect 里的 `setState` 在绘制之前重新渲染，右上角不闪）与 `Corner`。`type Shell = { sidebar?: boolean }` 留在 `layout.tsx`。
- `Corner`（决定 792）：`observer`，`useSession` 从 `./session-state` 引（决定 793）；`const { status } = useSession(); const { account } = useStore(); const loading = status === "signed-in" && account !== undefined && account.me === undefined; const { error } = useSWR(loading ? "me" : null, () => account!.load());`（守卫的同一个键，`guards.tsx:65`）。外层 `<div className="flex h-12 shrink-0 items-center justify-end gap-1 px-3 md:px-4">`；`signed-in` 且 `account.me` 在：在 `/settings/*` 上先放"回到工作区"（`Link to="/" state={arrived}`，`inline-flex h-7.5 items-center gap-1.5 rounded-full px-3 text-compact font-medium text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground`，前面一个 `ArrowLeft`），再放 `<UserMenu place="corner" />`；`signed-out`、`unavailable` 或 `error !== undefined`（`SessionChangedError` 除外，同 `guards.tsx:70`）：`<AppearanceMenu />`；其余（`starting`、账户读取中）：空的 `div`（高度留着，页不跳）。
- `WorkspaceLayout`：`NotLoaded` 带着 `error` 时与 `NotFoundPage` 时各经一个小组件调 `useCornerBack()`（hook 不能条件调用：`function GiveCornerBack() { useCornerBack(); return null; }`，与它们并列渲染）；读取中（`error` 为空的 `NotLoaded`）不交还（决定 769）。
- `RouteError`、`SessionUnavailablePage`：开头调 `useCornerBack()`。

#### 4.1.3 账户行、右上角与外观

`UserMenu({ place }: { place: "row" | "corner" })`：

- 触发按钮，`row`：`<Button variant="ghost" className="h-10 w-full justify-start gap-2.25 rounded-row px-2 text-row font-normal text-foreground">`（`gap-2.25`：名字在样稿的位置，审查 B-N5），里面 `<span aria-hidden className="grid size-6 shrink-0 place-items-center rounded-full bg-avatar text-[11px] font-semibold text-avatar-foreground">{initialOf(name)}</span>`、`<span className="min-w-0 truncate">{name}</span>`、`<Ellipsis className="ml-auto text-faint" />`。名字的 `span` 不用 `flex-1 text-left`：`text-left` 不在 `design-tokens.test.ts` 测试 5 的"不是颜色"名单里（`:298-316`），用 `ml-auto` 把 ⋯ 推到右端。
- `corner`：`<Button variant="ghost" size="sm" shape="pill" className="gap-2 pl-1.5 text-foreground">`，头像 `size-5.5`，名字 `max-w-32 truncate`，没有 ⋯。
- 内容：`row` 是 `side="top" align="start" className="w-(--radix-dropdown-menu-trigger-width)"`，`corner` 是 `align="end"`。项：
  1. `<DropdownMenuItem asChild onSelect={() => (leaving.current = !atProfile)}><Link to="/settings/profile" state={arrived}><Settings />{t("userMenu.settings")}</Link></DropdownMenuItem>`；内容的 `onCloseAutoFocus={(event) => { if (leaving.current) event.preventDefault(); leaving.current = false; }}`（`page-tree-item.tsx:450` 的 `chosen`，决定 770）；`atProfile` 是 `useMatch("/settings/profile") !== null`：已在个人设置页上时是同一路径的 `replace`，标题的 effect 不重跑，不挡 Radix 把焦点还给按钮（审查 A 的 P2A1-N1）；
  2. `<DropdownMenuSub><DropdownMenuSubTrigger><SunMoon />{t("appearance.label")}</DropdownMenuSubTrigger><DropdownMenuSubContent><AppearanceItems /></DropdownMenuSubContent></DropdownMenuSub>`；
  3. 分隔线、"退出登录"（照旧）、分隔线、版本行（照旧，`info !== undefined` 时）。
- `AppearanceItems`（`W/app/appearance-menu.tsx`）：`DropdownMenuLabel`"主题"，`DropdownMenuRadioGroup aria-label={t("theme.label")}` 的三项（`Monitor`、`Sun`、`Moon`，照 `theme-menu.tsx:16-41`），分隔线，`DropdownMenuLabel`"语言"，`DropdownMenuRadioGroup aria-label={t("language.label")}` 的两项（`lang` 照 `language-menu.tsx:32`）。每项 `onSelect={(event) => event.preventDefault()}`（决定 770）。
- `AppearanceMenu`：`<Button variant="ghost" size="icon-sm" shape="pill" aria-label={t("appearance.label")}><SunMoon /></Button>` 触发，内容 `align="end"`，里面是 `AppearanceItems`。
- `initialOf(name)`：`name.trim()` 的第一个字素（`Intl.Segmenter` 的 `grapheme`），`toLocaleUpperCase()`；空串得空串。

#### 4.1.4 `ShellSidebar` 与工作区外框

`W/app/shell-sidebar.tsx`（S1 的样子；S3 加抽屉，4.3.4）：

```tsx
export function ShellSidebar({ top, children }: { top: ReactNode; children: ReactNode }) {
  return (
    <div data-shell className="flex flex-col bg-sidebar [overflow-anchor:none] max-md:border-b max-md:border-border md:sticky md:top-0 md:h-svh md:w-63 md:shrink-0 md:self-start md:border-r md:border-border">
      <div className="space-y-2 px-2 pt-2.5 pb-1.5">{top}</div>
      <div data-sidebar-scroll className="min-h-0 flex-1 overflow-y-auto px-2 pt-1 pb-3">{children}</div>
      <div className="shrink-0 border-t border-border p-2"><UserMenu place="row" /></div>
    </div>
  );
}
```

- 中段 `pt-1`：滚动区裁掉越界的部分，第一行链接 2 px 偏移的焦点框要留出位置。顶部 `space-y-2 … pb-1.5`、中段 `pt-1`：切换器到框 8 px、框到"首页" 10 px，同样稿（审查 B-N5）。
- `WorkspaceLayout` 的 `left`（`:110-129`）换成 `<ShellSidebar top={<><WorkspaceSwitcher current={workspace} /><SearchBox key={workspace.id} workspace={workspace} /></>}>`，中段是 `<nav aria-label={workspace.name} className="flex flex-col gap-0.5">`（两个 `NavItem`、`NotebookNav`）与 S1 里照旧的 `<div ref={setNotebookColumn} className="mt-3" />`（S2 去掉）。`:108-109` 的注释改写为"粘住、与视口一样高之后，侧栏照旧不做窗口的滚动锚点（`overflow-anchor: none`）：探针在三种引擎里看过，留着无害（M13/P2/S1）"。
- `sections`（`:49-52`）加图标：首页 `House`，设置 `SlidersHorizontal`；`NavItem` 的子元素是图标与文字。

#### 4.1.5 切换器、导航行与笔记本列表

- 切换器（`workspace-switcher.tsx:36-39`）：`<Button variant="ghost" className="h-9 w-full justify-start gap-2.25 rounded-row px-2 text-sm font-semibold text-foreground [&_svg]:size-3.5" aria-describedby={description}>`，里面 `<span aria-hidden className="grid size-5.5 shrink-0 place-items-center rounded-md bg-primary text-xs font-semibold text-primary-foreground">{initialOf(current.name)}</span>`、`<span className="min-w-0 truncate">{current.name}</span>`、`<ChevronsUpDown className="ml-auto text-faint" />`（样稿 `.ws`，palette.html:67-70）。`:56` 的 `Link` 加 `state={arrived}`。
- `NavItem`（`nav-item.tsx:17`）：`cn("flex h-7.5 items-center gap-2.25 rounded-row px-2 text-row transition-colors duration-150 [&_svg]:size-4 [&_svg]:shrink-0", isActive ? "bg-active font-medium text-foreground" : "text-muted-foreground hover:bg-hover hover:text-foreground")`。当前项不加悬停（P1 4.1.3 的理由）。三个设置布局里的 `NavItem` 随之变样，P3 把它们挪进设置目录。
- `NotebookNav`（`notebook-nav.tsx:23-45`）：根 `flex flex-col gap-0.5`；`headingClassName="px-2 pt-3.5 pb-1 text-xs font-normal text-faint"`（样稿 `.group`，palette.html:81）；`listClassName="flex flex-col gap-0.5"`；`groupClassName=""`；空的说明 `px-2 py-1 text-row text-faint`；"新建笔记本"：`<Button variant="ghost" className="h-7.5 w-full justify-start gap-2.25 rounded-row px-2 text-row font-normal">`。S1 里每本笔记本照旧是一个 `NavItem`（加 `Book` 图标），S2 换成 4.2.2 的两种。
- `NotebookGroups` 加 `groupClassName: string`，`Group` 的根（`notebook-groups.tsx:66`）用它；工作区首页传 `"space-y-2"`，照旧。

#### 4.1.6 子菜单的部件

`W/components/ui/dropdown-menu.tsx` 加：

```tsx
export const DropdownMenuSub = Primitive.Sub;

export function DropdownMenuSubTrigger({ className, children, ...props }: ComponentProps<typeof Primitive.SubTrigger>) {
  return (
    <Primitive.SubTrigger className={cn(item, "px-2.5 data-[state=open]:bg-active [&>svg:last-child]:size-3.5 [&>svg:last-child]:text-faint", className)} {...props}>
      {children}
      <ChevronRight className="ml-auto" />
    </Primitive.SubTrigger>
  );
}

export function DropdownMenuSubContent({ className, ...props }: ComponentProps<typeof Primitive.SubContent>) {
  return (
    <Primitive.Portal>
      <Primitive.SubContent
        sideOffset={6}
        className={cn("z-50 min-w-[200px] origin-(--radix-dropdown-menu-content-transform-origin) overflow-hidden rounded-popover border border-border bg-popover p-1.25 text-popover-foreground shadow-popover data-[state=open]:animate-pop-in", className)}
        {...props}
      />
    </Primitive.Portal>
  );
}
```

右箭头是 `SubTrigger` 的直接子元素，`item` 的 `[&>svg]:size-4`、`[&>svg]:text-muted-foreground`（选择器 `.x > svg`，(0,1,1)）也落在它上面，写在箭头自己身上的 `text-faint`（(0,1,0)）压不过；所以由 `SubTrigger` 的 `[&>svg:last-child]:size-3.5`、`[&>svg:last-child]:text-faint`（(0,2,1)）定它的大小与颜色，tailwind-merge 把它们与 `[&>svg]:` 的当作不同的变体，都留着。Radix 的子菜单：`SubTrigger` 带 `aria-haspopup="menu"`，点按只打开、不挪焦点（`SubContent` 的 `onOpenAutoFocus` 只在键盘操作时聚焦，`@radix-ui/react-menu` 的 `index.mjs:745-750`；审查 A-N1），Enter、Space、→ 打开并把焦点给第一项，← 与 Esc 关上（Esc 关上整个菜单，`@radix-ui/react-menu` 的 `SubContent` 调根的 `onClose`）。

#### 4.1.7 头像的变量

`W/styles.css`：`@theme inline`（`:14-40`）里 `--color-overlay` 之后加 `--color-avatar: var(--avatar);`、`--color-avatar-foreground: var(--avatar-foreground);`；`:root`（`:91-`）与 `.dark`（`:121-`）的 `--sidebar` 之后各加 `--avatar: #3f8f74;`、`--avatar-foreground: #ffffff;`。`design-tokens.test.ts` 测试 1 要两套都有，测试 5 要类名有映射，都满足。

**照实**（S1 的实施与审查，[S1 的审查记录](reviews/P2-S1-shell-account-review.md)）：

- 4.1.1 的表已照实加 `W/app/narrow.ts` 与 `narrow.test.ts`（决定 801）、`W/pages/landing.tsx`（读列表失败时交还右上角）。
- **4.1.2**：
  - `useCornerBack()` 不带参数（没有调用者传它）；`GiveCornerBack`（`function GiveCornerBack() { useCornerBack(); return null; }`）在 `corner.tsx` 里导出，工作区外框与 `LandingPage` 都用它（审查 A 的 S1A-M1）。
  - index 路由也带 `handle: { sidebar: true }`：它的去处是有侧栏的工作区，或没有 `handle` 的新建工作区页；落地页读列表失败时交还右上角（S1A-M1）。
  - 工作区外框与落地页交还右上角的条件是"有 `error` 且不是 `SessionChangedError`"（`NotLoaded` 把后者当作读取中，与 `guards.tsx` 相同）；这两处与 `Corner` 自己的一处有意不由测试守（y03、y04、y04b，主持者的决定）。
  - `Corner` 里写的是 `account?.load()`（上面写的是 `!`）；在 `/create-workspace` 上另以 `"workspaces"` 键读列表，账户有工作区时放"回到工作区"（决定 803）。
  - "回到工作区"另带 `shrink-0 whitespace-nowrap [&_svg]:size-4`：375 宽时它折成两行，箭头是 24 px（7.4）。
- **4.1.3**：
  - `corner` 的按钮另带 `min-w-0 shrink`，名字是 `max-w-32 min-w-0 truncate`（375 宽时与"回到工作区"挤在一行）。
  - 内容：`corner` 另有 `collisionPadding={12}` 与 `max-w-[min(16rem,var(--radix-dropdown-menu-content-available-width))]`：版本行里 40 位的提交号在 375 宽时把菜单撑出屏幕，宽屏时撑到 444 px（审查 B 的 S1B-N3）；版本行带 `wrap-anywhere`（账户行的菜单与行同宽，提交号原先被裁掉）。
  - "个人设置"的 `leaving` 是 `!atProfile && pendingSteps(me).length === 0`：还有引导步骤时设置页把它送回引导页，没有标题取焦点（S1A-N2）。
  - 第 2 项的子菜单只在 md 起；md 以下是分隔线加 `AppearanceItems`（决定 801）。
  - `AppearanceItems` 的语言项前有一个 `size-4` 的 `aria-hidden` 占位，语言名与主题名同线（S1B-N1）。
- **4.1.4**：上面的代码已照实去掉中段的 `overscroll-contain`（决定 802）。
- **4.1.5**：分组标题另带 `leading-4.5`，`NotebookNav` 的 `groupClassName` 是 `"flex flex-col gap-0.5"`（S1B-N2）；切换器的菜单是 `w-(--radix-dropdown-menu-trigger-width)`（S1B-N4）。
- **4.1.6**：`DropdownMenuSubContent` 默认 `collisionPadding={8}`（S1B-M1）；`DropdownMenuSeparator` 是 `h-0 border-t border-border`，强制色画得出（S1B-N5；P1/S2 起是 `h-px bg-border`）。
- **7.1**：jsdom 没有布局，Radix 把指针离开子菜单的触发项当作去了别处，`user.click` 子菜单里的项之前子菜单已经关上，所以 `layout`、`routes`、`user-menu` 在子菜单里的选择用 `fireEvent.click`（点"外观"本身照旧 `user.click`）；"New workspace"实际名为"Create workspace"（`workspace.create`）；`layout-error` 一格断言"没有"，不是先红；审查之后加的各格见 7.1 的新行。
- **e2e**：a8 用指针在子菜单里选时分十步移过去：右上角的子菜单在窗口右侧放不下、翻到主菜单左边，Radix 靠指针在主菜单上的横向移动判断它是否正往子菜单去，Playwright 一步跳到项上时没有这样的移动，子菜单就关（三种引擎；人的移动经过主菜单，不受影响；审查 A 的探针 K、L）。L1 的粘住守卫容 1 px：页高带小数、窗口的滚动是整数，滚到最底时粘住的侧栏被容器底边顶上去不到 1 px。
- **第 0 步**：经 perf 的 `setup.ts`（`flat3k`）与临时的三引擎配置；探针在提交 2 改完、提交之前的构建上跑。(b) 由决定 802 取代（8.1 第 4 条）。
- **第 5 节**：se1 无效框亮色下限 3.91（实量 4.018，预期写的 4.02）。
- **决定 795(d)**：se2 失败所在的行是 :231（第二下方向键之后的 `toHaveValue`），:230 是那下方向键本身；"se2:230 之后的步骤只在 Linux 的 CI 上走"照旧成立。
- **6.1**：多了几个提交：`web: the appearance inline in a narrow account menu (M13/P2/S1)`（决定 801）；审查的修复 `web: the shell's review fixes (M13/P2/S1)`（作用域只写 `web`：它不改 e2e）与 `web, e2e: the shell's tests held tighter, as the review found (M13/P2/S1)`；修复核对第 1 轮的 `web: the onboarding cell waits for the bounce itself (M13/P2/S1 fix check 1)`（主持者）。提交 1 在 e2e 之后本地改过两次（加测试一格、三处类名），决定 801 的提交与第一个修复提交各改过一次（右上角菜单的宽度；`notebook-step.test.tsx`），改后的都重跑了检查（审查记录）。
- **随决定 801–803 改的设计文字**（审查 C 的 S1C1-M1）：决定 774 的两句、4.1.4 的代码、4.3.1 的 `narrow.ts` 一行、6.3 提交 2、7.1"窄屏的查询"一行（S3 → S1）、8.1 第 4 条、8.2 第 1、2 条的新文字、第 9 节的 P3 一条。

### 4.2 S2 笔记本与页面树

#### 4.2.1 文件

| 文件 | 改什么 |
|---|---|
| `W/pages/workspace/notebook-slot.tsx`（新） | `makeSlot`、`NotebookSlot`、`useNotebookSlot`、`SlotHolder`、`useSlotMoves`（决定 776、798、800） |
| `W/app/shell-sidebar.tsx` | 导出 `intoColumn(element)`：只改 `[data-sidebar-scroll]` 的 `scrollTop` 把元素带进可见范围（`reveal` 与 `SlotHolder` 共用，4.2.5） |
| `W/pages/workspace/workspace-layout.tsx:36-46,81,127,133-135` | 去掉 `NotebookColumn`、`useNotebookColumn`，给出 `NotebookSlot` |
| `W/pages/workspace/notebook-nav.tsx` | 当前那一项画 `SlotHolder`，别的画 `NotebookLink`；备用位置只给编辑中留住的（4.2.2） |
| `W/pages/notebook/notebook-layout.tsx:53,70-77` | 门户进 `containerFor(id)`，门户的 key 是 `` `tree-${id}` ``（4.2.2） |
| `W/pages/notebook/page-tree.tsx:156-209,222` | 笔记本行、＋的位置、"目录""历史"两行、失败的提示加 `my-1`；`reveal`、`show`（4.2.3、4.2.5）；`close`、`create` 的注释（决定 800） |
| `W/pages/notebook/page-tree-item.tsx:160,259-347,371` | 树的 `ul` 去掉 `gap-0.5`；行的样子、文件图标、当前行登记链接（4.2.4）；`TreeMenu` 调 `useSlotMoves()`（决定 800） |
| `W/pages/page/readers-input.ts:16-41` | `watchReader` 加 `within`（选择器：只算落在它的元素里的输入，决定 791） |
| `W/pages/notebook/notebook-places.tsx:20-33` | 图标链接，图标 16 px，当前项同树的当前行（4.2.3，决定 796） |
| `W/pages/notebook/move-page-dialog.tsx:159,172` | 手写的"标签加控件"组 `space-y-2` → `space-y-1.5`（01-P1 第 9 节"P2–P5"，树的对话框） |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | `notebook.settingsOf`（Settings of {notebook} / {notebook}的设置） |
| `W/pages/workspace/notebook-slot.test.tsx`、`W/pages/notebook/page-tree-reveal.test.tsx`、`W/pages/page/readers-input.test.ts`（新） | 7.1 的"稳定的容器""笔记本行、齿轮与＋""当前行滚进可见范围""看守只算侧栏里的输入"各行 |
| `W/pages/page/reading-view.test.tsx` | 7.1 的"阅读视图的等待"一行：树的看守也在 `window` 上听 |
| vitest | 7.1 的 S2 各行 |
| `e2e/fixtures/notebook-pages.ts:17-26`；`e2e/stories/page/pg16-tree-menu.spec.ts`（新）；`e2e/playwright.config.ts:3-4`；`tools/e2e-projects/check.mjs:22-29` | 第 5 节 |
| `docs/v0.1/v0.1-design.md` | 8.2 |

#### 4.2.2 稳定的容器与列表项

`W/pages/workspace/notebook-slot.tsx`：

```tsx
type Slot = {
  /** The container of the notebook id's row, tree and places: the same element for the id, each id's kept (798). */
  containerFor: (id: string) => HTMLElement;
  /** Where the focus was in the container as its holder went, to give back once the next one holds it. */
  keepFocus: (id: string, element: HTMLElement | undefined) => void;
  takeFocus: (id: string) => HTMLElement | undefined;
  /** A holder moved a container (to the other group, or out of the fallback place): the tree menu measures again (800). */
  moved: () => void;
  moves: () => number;
  subscribe: (listener: () => void) => () => void;
};
```

- `WorkspaceLayout` 以 `useState(() => makeSlot())` 建一个（随工作区外框的一生），经 `NotebookSlot` 给出；`containerFor` 在一个 `Map` 里按 id 留着各自的容器，第一次要时新建（`element.className = "group/notebook relative"`），从不在渲染里去掉别的 id 的（决定 798）；`moved` 给计数加一并通知订阅者（决定 800）。
- `SlotHolder({ id })`：`<div ref={holder} />`；`useLayoutEffect`（依赖 `[slot, id]`）：`const at = holder.current; if (at === null) return undefined; const element = slot.containerFor(id); const focused = slot.takeFocus(id); const moving = element.parentElement !== null; at.append(element); if (moving) slot.moved(); if (focused?.isConnected && (document.activeElement === null || document.activeElement === document.body)) { focused.focus({ preventScroll: true }); intoColumn(focused); }`，清理：`const active = document.activeElement; slot.keepFocus(id, active instanceof HTMLElement && element.contains(active) ? active : undefined);`。注释写明：`WorkspaceLayout` 先画侧栏的门户、后画 `Outlet`，`:slug` 懒加载、`Layout` 早一次提交就有侧栏的位置，所以同一次提交里这个 layout effect 先于树的，树的 effect 运行时容器已在文档里（决定 776）。`moving`：容器之前已在一个持有者里（上一个的 `div`，已离开文档）；第一次收养不通知（决定 800）。
- `useSlotMoves()`（`TreeMenu` 用）：`useSyncExternalStore(slot.subscribe, slot.moves)`；上下文的默认值是一个从不挪动的 slot（`moves()` 恒为 0，`subscribe` 什么也不做）。
- `NotebookNav`：`const current = useMatch("/:slug/notebooks/:id/*")?.params.id;`（照原样）；`renderItem={(notebook) => notebook.id === current ? <SlotHolder id={notebook.id} /> : <NotebookLink workspace={workspace} notebook={notebook} />}`；两组之后：`{current !== undefined && notebooks.list !== undefined && notebooks.byId(current) === undefined && store.unsavedEdit({ notebookId: current }) && <SlotHolder id={current} />}`（编辑中留住的，`NotebookLayout:63` 的同一条件；地址里大写的、不存在的 id 不放，审查 B-M3）。
- `NotebookLink`：`<Link to={…} state={arrived} className="flex h-7.5 items-center gap-2.25 rounded-row px-2 text-row text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground">`，里面 `<ChevronRight aria-hidden className="-mr-1 -ml-0.5 size-3.5 shrink-0 text-faint" />`、`<Book aria-hidden className="size-4 shrink-0" />`、`<span className="min-w-0 truncate">{notebook.name}</span>`（⌄、书本、名字在 14 / 33 / 58 px，与笔记本行相同，决定 778）。链接的 `textContent` 是名字（图标没有文字），`notebookGroups` 读它。
- `NotebookLayout`：`const container = useNotebookSlot().containerFor(id);`（在开头无条件调用，`id` 是地址里的，照原样），门户 `createPortal(<><PageTree notebook={notebook} /><NotebookPlaces notebook={notebook} /></>, container, `tree-${notebook.id}`)`：key 带前缀，不与同级的 `<Outlet key={notebook.id} />`（`notebook-layout.tsx:81`）撞（审查 C-M2：同 key 时 React 报"Encountered two children with the same key"，`test/setup.ts` 把 `console.error` 当失败）；`:72` 的 `div` 不要了（容器就是它）。

#### 4.2.3 笔记本行、＋、齿轮与"更多"

`PageTree` 返回一个片段（`shown` 是齿轮与＋共用的类：`opacity-40 transition-[opacity,color,background-color] duration-150 hover:opacity-100 focus-visible:opacity-100 group-has-[[data-notebook-row]:hover]/notebook:opacity-100 group-has-[[data-notebook-new]:hover]/notebook:opacity-100 [@media(hover:none)]:opacity-100 max-md:opacity-100 forced-colors:opacity-100`，决定 778；强制色一项照审查 B 的 S2B-N3）：

```tsx
<>
  <div data-notebook-row data-home={atHome || undefined} className={cn("flex h-7.5 items-stretch rounded-row text-row text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground [@media(hover:hover)]:not-data-home:group-has-[[data-notebook-new]:hover]/notebook:bg-hover [@media(hover:hover)]:not-data-home:group-has-[[data-notebook-new]:hover]/notebook:text-foreground data-home:bg-active data-home:font-medium data-home:text-foreground", writer ? "pr-14" : "pr-8")}>
    <h2 ref={heading} tabIndex={-1} className="flex min-w-0 flex-1 outline-none">
      <NavLink to={home} end className="flex min-w-0 flex-1 items-center gap-2.25 rounded-row pl-2">
        <ChevronDown aria-hidden className="-mr-1 -ml-0.5 size-3.5 shrink-0 text-faint" />
        <Book aria-hidden className="size-4 shrink-0" />
        <span className="min-w-0 truncate">{notebook.name}</span>
      </NavLink>
    </h2>
    <Link to={`${home}/settings/general`} aria-label={t("notebook.settingsOf", { notebook: notebook.name })} className={cn("absolute top-0.75 grid size-6 place-items-center rounded-row text-faint hover:bg-hover hover:text-foreground [&_svg]:size-4", shown, writer ? "right-7.5" : "right-1")}>
      <Settings2 aria-hidden />
    </Link>
  </div>
  <nav ref={nav} aria-label={t("page.tree", { notebook: notebook.name })} className="flex flex-col">
    {writer && <Button data-notebook-new variant="ghost" size="icon-sm" className={cn("absolute top-0.75 right-1 size-6 rounded-row text-faint", shown, "disabled:opacity-25!")} aria-label={t("page.new")} disabled={newPage.sending} onClick={() => create(null)}><Plus /></Button>}
    {/* 目录、历史两行；失败的提示（Alert 加 my-1，决定 780）；树；TreeMenu：照旧，样子见下 */}
  </nav>
</>
```

- 链接盖满行的高度与左边距，⌄ 在链接里（审查 B-M5）；齿轮在行的 `div` 里、`h2` 之后（它不是标题的一部分，标题的名字照旧只是笔记本名，`page-tree-writes.test.tsx:427,437`；⌄ 与书本 `aria-hidden`，链接的名字与 `textContent` 照旧是笔记本名）；它与＋都相对于容器（`group/notebook relative`）定位。`atHome`：地址就是 `home`（`useMatch`）。
- `nav` 不能带 `relative`（＋要相对于容器）。`h2` 在 `nav` 之前，`treeTitles`（`e2e/fixtures/wiki-pages.ts:24-26`）、`page-tree.test.tsx:17-20` 只读 `nav` 里的链接，不变。
- 换组时开着的菜单：`TreeMenu` 照旧在每次提交之后重量按钮的位置，另调 `useSlotMoves()`（一句调用、不取值：计数变了它就重画，之后没有依赖的那个 effect 重量；lint 照旧只有 `:394` 那一条豁免，4.4），`SlotHolder` 挪了容器时它重画、重量（决定 800）；`close`、`create` 照旧每次渲染新建（列表重读时同一组里的移动靠这次重画），注释写明换组靠的是通知、不是它们。
- "目录""历史"两行（`page-tree.tsx:179-199`）：`NavLink end`，S2 里照旧 `state={opened}`；S3 去掉 `state`，改在点按时定，同树的行：`onClick={follow(to)}`，`follow` 先以 `linkFollowed`（`W/lib/link-click.ts`）判断是不是该自己导航的点按（修饰键、中键照浏览器），是就 `preventDefault`，`navigate(to, { state: context.inDrawer() && !here ? arrived : opened, replace: here })`，`here` 是 `createPath(location) === to`（React Router 的 `Link` 先调 `onClick`，`defaultPrevented` 时不再自己导航，`lib/dom/lib.js:312-315`）。不在渲染时定：`inDrawer()` 经 ref 读，抽屉开关时 `PageTree` 不重画，渲染时带的总是 `opened`，抽屉里点它焦点去了菜单按钮（审查 A 的 P2A2-M1；决定 782）。`cn("flex h-7.5 items-center gap-2.25 rounded-row pl-7.5 pr-2 text-row transition-colors duration-150", isActive ? "bg-accent-soft font-medium text-foreground [&_svg]:text-accent" : "text-muted-foreground hover:bg-hover hover:text-foreground")`，图标 `List`、`History`：左边对齐深度 0 的页的图标（0.5 rem 缩进 + 1.125 rem 展开按钮 + 0.25 rem 间距 = 30 px），当前时同树的当前行。
- `NotebookPlaces`（`notebook-places.tsx:20-33`）：`<nav aria-label={…} className="flex gap-0.5 py-1 pl-6">`，四个 `NavLink end`：`cn("grid size-7 place-items-center rounded-row transition-colors duration-150 [&_svg]:size-4", isActive ? "bg-accent-soft [&_svg]:text-accent" : "text-faint hover:bg-hover hover:text-foreground")`，里面图标（`List`、`HeartPulse`、`History`、`Trash2`，`aria-hidden`，16 px：审查 B-I2）与 `<span className="sr-only">{label}</span>`，`title={label}`。"目录""历史"与树里两行同图标、同一种当前的样子（决定 796）。

#### 4.2.4 树的行

`page-tree-item.tsx:259-347`：

- 列表（`:160`）：`flex flex-col`（去掉 `gap-0.5`，行紧挨着，样稿的 `.tree`；审查 B-N5）。
- 行（`:261-269`）：`cn("group relative flex h-7.5 items-center gap-1 rounded-row pr-1 text-row text-muted-foreground transition-colors duration-150 hover:bg-hover hover:text-foreground data-active:bg-accent-soft data-active:text-foreground", 合并的环照旧)`，`data-active={active || undefined}`，`style={{ paddingLeft: \`${0.5 + depth}rem\` }}`。
- 展开按钮（`:271-280`）：`className="-ml-1 mr-1 grid size-4.5 shrink-0 place-items-center rounded-sm text-faint hover:bg-hover"`（`-ml-1 mr-1`：› 与笔记本行的 ⌄ 同列，链接不动，决定 804），`<ChevronRight className={cn("size-3.5 transition-transform", open && "rotate-90")} />`；空位（`:282`）`w-4.5 shrink-0`。
- 链接（`:284-305`）照旧是行的直接子元素：`cn("relative min-w-0 flex-1 truncate py-1", active && "font-medium")`；普通页（不是 `agents`、`sourceRoot`、`source`）在名字前加 `<File aria-hidden className={cn("mr-2.25 inline-block size-4 align-text-bottom", active && "text-accent")} />`，三种的图标照旧，`mr-1` 改 `mr-2.25`（名字在 55 px，同"目录""历史"两行与样稿；审查 B-N5），当前时同样 `text-accent`。`sr-only` 的种类后缀照旧。
- 菜单按钮（`:331`）：`"grid size-6 shrink-0 place-items-center rounded-row text-faint opacity-0 group-hover:opacity-100 hover:bg-hover hover:text-foreground focus-visible:opacity-100 aria-expanded:opacity-100 any-pointer-coarse:opacity-100"`，`<Ellipsis className="size-4" />`。
- 落点线（`:345`）：`left: \`${0.5 + depth + 1.375}rem\``（缩进加展开按钮与间距，标题开始的地方）。
- `PageRow` 拿一个链接的 ref，`useLayoutEffect(() => { const element = link.current; return active && element !== null ? context.show(element) : undefined; }, [active, context])`：当前时登记、清理时撤下（4.2.5）。

#### 4.2.5 当前行滚进可见范围

`W/app/shell-sidebar.tsx` 导出 `intoColumn(element)`：`const area = element.closest<HTMLElement>("[data-sidebar-scroll]"); if (area === null) return false;` 量两者的 `getBoundingClientRect`，`row.top < view.top` 时 `area.scrollTop -= view.top - row.top + 8`，`row.bottom > view.bottom` 时 `area.scrollTop += row.bottom - view.bottom + 8`；返回 `true`。不调 `scrollIntoView`（它也滚窗口，13.2 第 26 条）。

`PageTree` 的 `context` 加 `show(link)` 与 `reveal()`（`useMemo` 里，经 ref 读；决定 791）：

```ts
// key: location.key of the commit, set in an insertion effect (before the commit's layout effects, the rows' too; a
// render not committed leaves it); settled: the key PageTree's layout effect last ran for; at: the key of the
// navigation that last changed the pathname, the one reveal decides for (a hash, a query or a replace of the
// address shown is the same navigation).
const reveal = () => {
  const nav = at.current;
  if (revealed.current === nav) return;
  const watching = watch.current; // the same object for the tree's life, its fields changed
  if (watching.key !== nav || watching.reader === undefined) {
    watching.reader?.end();
    watching.key = nav;
    watching.reader = watchReader({ within: "[data-sidebar-scroll]" });
  }
  const link = shown.current;
  if (link === null || watching.reader.acted() || !intoColumn(link)) return;
  revealed.current = nav;
  watching.reader.end();
};
const show = (link) => {
  shown.current = link;
  if (settled.current === key.current) reveal(); // a row drawn later in this navigation
  return () => { if (shown.current === link) shown.current = null; };
};
```

`PageTree` 的 layout effect：

```ts
useInsertionEffect(() => {
  key.current = location.key;
}, [location.key]);
useLayoutEffect(() => {
  settled.current = location.key;
  if (location.pathname !== path.current) {
    path.current = location.pathname;
    at.current = location.key;
  }
  context.reveal(); // nothing for a navigation already settled: a hash, a query, a replace of the address shown
}, [location.key, location.pathname, context]);
useEffect(() => {
  const watching = watch.current;
  return () => {
    watching.reader?.end();
    watching.reader = undefined; // StrictMode's unmount and mount again: reveal makes a new one (as useArrivalWatch)
  };
}, []);
```

同一次提交里行的 layout effect 先跑、只登记，父的这个后跑、定滚不滚；之后才画到的行（分批画、树后读到）由 `show` 直接调。看守按要定的那一次导航（`at`）懒建：这次导航之前的按键、点击（快速切换里的 Mod+O 与 Enter、点树的行）都不算；只算落在侧栏滚动区里的。卸下时 `end()`。同一页换 hash（右栏的大纲、正文里指向本页某一节的链接、后退前进到同一页的另一个 hash）、换查询参数与 `replace` 到当前的地址都算同一次导航：不新建看守，"至多一次"照旧；树还没读到时换了 hash，晚到的行照旧由 `show` 调 `reveal()`（决定 791；审查 B 的 P2B1-M1）。卸下时结束看守并把它清空（审查 C 的 P2C3-N1：StrictMode 模拟的卸下再挂上之后，`reveal()` 见 key 相同会接着用一个已结束的读者，读者在侧栏里的动作就不算了；`W/app/arrival.ts` 的 `useArrivalWatch` 也这样做）。layout effect 总调 `reveal()`、路径变了才更新 `path` 与 `at`（审查 B 的 P2B2-N1：行恰好画在换 hash 的那次提交里时，`show` 看到 `settled` 还是旧的 key、不调，同一路径提前返回的话就没人再调了）。宽屏上滚动区是侧栏的中段；S3 之后窄屏的抽屉里同样（关着时也照样有布局，打开时已在位置上）。

**照实**（S2 的实施与审查，[S2 的审查记录](reviews/P2-S2-notebooks-tree-review.md)）：

- 4.2.1 的表已照实加三个新的测试文件与 `reading-view.test.tsx`；`notebook-slot.tsx` 也导出 `makeSlot`（`WorkspaceLayout` 用它），`Slot` 类型不导出（只在本文件用）。
- **4.2.2**：离开一本笔记本再回来时，容器的父元素是上一个已离开文档的持有者，`moving` 为真，再收养它时也调 `moved()`：同决定 800 的条件，只多一次重量，无害。
- **4.2.3**：上面的类已照实：＋ `right-1`，写者的齿轮 `right-7.5`、读者的 `right-1`，行的 `pr-14`、`pr-8` 不变，两者的图标与树行的 ⋯、样稿的 `.end` 同在 219–235（审查 B 的 S2B-N1；原来的 `right-1.5`、`right-8` 偏左 2 px）；`shown` 加 `forced-colors:opacity-100`（S2B-N3：强制色下系统色加 0.4，对 Canvas 约 2.8:1）。
- **4.2.4**：展开按钮另带 `-ml-1 mr-1`（决定 804）；拖放的 `onDrop` 把 `location` 解构成 `dropped`，不与路由的 `location` 同名（oxlint 的 `no-shadow`）。
- **4.2.5**：
  - `intoColumn` 的两个 `if` 写成 `if … else if`：第二次判断用的是滚动之前量的矩形，只有行比滚动区还高时两者才不同，30 px 的行碰不到。它在 `shell-sidebar.tsx` 里放在 `ShellSidebar` 的注释之前（审查 C 的 S2C-N2）。
  - `reveal` 开头是 `if (navigation === undefined || revealed.current === navigation) return;`：`at` 起初是 `undefined`，判断用来收窄类型；上面的写法里 `revealed` 同为 `undefined` 时一样返回，行为相同。
  - ref `shown` 叫 `shownLink`（`shown` 已是决定 778 的类），`nav` 叫 `navigation`。
- **第 5 节**：PG16 两格已照实改写：改名对话框只有 Save，第一格以 Esc 取消；第二格末尾加了 375 宽一段（变异 24 只有它抓得住，10.2）。
- **7.1**：
  - `readers-input.test.ts` 原来没有，是新文件（`test.each` 按键、按下、滚轮、触摸，加"不带 `within` 时哪里都算"一格）；`reading-view.test.tsx` 一格改为按监听函数计。
  - "首页、按键、开页"一格的按键改在滚动区里（审查 C 的 S2C-M1：原写法按在 `document.body` 上，看守有 `within` 之后不算它，这一格不管看守怎样建都过）；"树读到之前在滚动区里滚轮"一格加第二次导航（S2A-N1）；加 StrictMode 一格（S2C-N1）。7.1 的行已照实改。
  - 多出的格："更多"的图标、名字与 `title`（s2）；大写 id 格另断言两组之后紧接"New notebook"（s5）；三种图标的 `test.each`（s15）；页 → 首页（s11）。
  - 不是先红的格与守它们的：大写 id 格由 m49、s5 守；`notebook-nav.test.tsx` 的改写是随结构改的旧格；滚动区里的滚轮格由 m25 守；PG16 的菜单格由 m26 守；"在范围里的行不动""不带 `within` 时哪里都算"、来源根与 `AGENTS` 的图标只断言不变的事；普通页的图标格写在代码之后，经 s15 看过它红；审查修复的三格写在代码之后、没改的树上就绿，由 b1（两格）、b2（一格）守着。
- **7.2**：第 24 行的抓手已照实改（宽屏那几步抓不到，只有 375 宽一段）；第 52 行照设计原样（修复之后"首页、按键、开页"一格也抓 b1）；表末加 b1、b2，表后一句。s17（`key` 在 layout effect 里设）在 jsdom 里等价，不列。
- **7.5**：PG16 的时间已照实记。
- **6.2**：
  - 多了两个审查修复的提交：`web: the tree's watch held per navigation and across StrictMode, as the review found (M13/P2/S2)` 与 `web: the tree's right-hand icons and chevrons aligned, the cog and ＋ whole in forced colours (M13/P2/S2)`。
  - 提交 1 在 e2e 之后本地 amend 过一次（大写 id 一格加断言，s5），提交 2 amend 过一次（PG16 第二格加 375 宽一段；提交说明没写这一段，审查 C 的 S2C-N3），amend 之后都重跑了检查。外观的修复提交就是分支头，它的 `make e2e` 就是合并之前整条命令里的那一次。
- **8.2 第 10 条**：S2 的文字照 PG16 实际做的改了（以 Esc 取消改名；宽屏与 375 px 下都看窗口）。

### 4.3 S3 搜索、⌘K、快速切换与窄屏

#### 4.3.1 文件

| 文件 | 改什么 |
|---|---|
| `W/pages/workspace/search-box.tsx:35-38,93-106,133-176` | 胶囊；`field` 由外框给；去掉监听（4.3.2） |
| `W/pages/workspace/search-shortcuts.ts`（新） | `useSearchShortcuts`（决定 783） |
| `W/app/shortcuts.ts:4-8,35-41` | 导出 `composing`；`dialogOpen({ besideDrawer })`；`onMac` 挪走（4.3.3） |
| `W/lib/platform.ts`、`platform.test.ts`（新） | `onMac`（M12-polish 第 8 项，决定 797）；`search-box.tsx`、`quick-switch.tsx`、`W/pages/page/page-edit.tsx`、`page-layout.tsx`、`W/components/ui/native-select.tsx` 改引入（五个调用者；`shortcuts.ts` 不调用它，不再引），`shortcuts.test.ts` 里 `onMac` 的格挪过来 |
| `W/app/arrival.ts:15-28` | 导出 `headingTakes(state, type)`：`isArrival`、`POP` 加 `isOpened`、`Located`（不导出的 `isLocated`，照 `locatedOf`，`W/pages/page/attachment-row.tsx:42-46`：不从那里引，`arrival.ts` 在主包里、`attachment-row` 在懒加载的包里，引了会把它连同下拉菜单的部件拉进主包）；抽屉用（决定 790），`arrival.test.tsx` 一格 |
| `W/app/narrow.ts` | S1 已建（决定 801，`useSyncExternalStore`）：`useNarrow()` 照 `matchMedia("(width < 48rem)")`，没有 `matchMedia`（jsdom）时为假；S3 加调用者（抽屉） |
| `W/app/shell-sidebar.tsx` | `useDrawer`、抽屉、细栏、遮罩；`Drawer`（上下文）、`useDrawerActions`（4.3.4，决定 790、799） |
| `W/pages/page/unsaved-guard.tsx:63-69` | "留下"那一支先调 `useDrawerActions().quietClose()`，再 `stay()`（决定 799） |
| `W/pages/workspace/workspace-layout.tsx` | 调 `useDrawer`（交给 `ShellSidebar` 与 `useSearchShortcuts`）、`Drawer` 上下文的 provider（`drawer.actions`，决定 799）、搜索框的 ref、细栏的位置文字（审查 C 的 P2C2-N5） |
| `W/pages/notebook/page-tree-item.tsx:149-155`、`page-tree.tsx`（两行改在点按时定 `state`；`context` 加 `useDrawerActions().inDrawer`） | 抽屉开着、点的不是当前的地址时带 `arrived`（决定 782） |
| `W/pages/notebook/quick-switch.tsx:136-193` | 换装（4.3.5） |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | `shell.menu`（Navigation / 导航）、`search.placeholder`（Search / 搜索）、`page.quickSwitchKeys`（↑↓ to choose · ↵ to open · Esc to close / ↑↓ 选择 · ↵ 打开 · Esc 关闭） |
| vitest | 7.1 的 S3 各行 |
| `e2e/stories/shell/ui1-narrow-drawer.spec.ts`、`e2e/fixtures/shell-pages.ts`（新）；`e2e/playwright.config.ts:3-4`；`tools/e2e-projects/check.mjs:22-29`；`e2e/stories/search/se1-search.spec.ts:281-288`（"button"一项重量，决定 794） | 第 5 节 |
| `docs/v0.1/v0.1-design.md` | 8.2 |

#### 4.3.2 胶囊搜索框

`search-box.tsx:133-176`：

```tsx
<form ref={form} role="search" aria-label={…} noValidate onSubmit={onSubmit} className="space-y-1">
  <Label htmlFor={ids.field} className="sr-only">{t("search.field")}</Label>
  <div className="relative">
    <Input ref={field} id={ids.field} type="search" placeholder={t("search.placeholder")} …
      aria-keyshortcuts={mac ? "Meta+K Meta+Shift+F" : "Control+K Control+Shift+F"}
      className="peer h-8 rounded-full border-border bg-surface pr-12 pl-8.75 md:text-compact dark:bg-surface-2" />
    <span className="absolute top-1 left-1.75 grid size-6 rounded-full bg-surface transition-[background-color] duration-150 dark:bg-surface-2 dark:peer-focus-visible:bg-surface">
      <Button type="submit" variant="ghost" size="icon-sm" shape="pill" aria-label={t("search.submit")} className="size-6 text-faint">
        <Search />
      </Button>
    </span>
    <kbd aria-hidden className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 rounded-[5px] border border-border px-1.25 font-sans text-[11px] leading-4.25 text-faint max-md:hidden">
      {mac ? "⌘K" : "Ctrl K"}
    </kbd>
  </div>
  {problem !== undefined && /* 照旧 */}
  <p id={ids.hint} className="sr-only">{/* 照旧 */}</p>
</form>
```

- `field` 不再是 `SearchBox` 自己的 `useRef`，由 `WorkspaceLayout` 经 prop 给（`field: RefObject<HTMLInputElement | null>`），`useSearchShortcuts` 用同一个。
- 按钮的垫底（决定 794）：`span` 是按钮的父元素，与框同色（亮 `--surface`；暗平时 `--surface-2`，框聚焦时 `--surface`，经框上的 `peer`），`contrast.ts` 顺祖先找底时找到它，量的是环画在上面的那一对；框与无效时的环在框外，照旧量到侧栏。
- `type="search"` 的清除按钮（WebKit、Chromium 的 `::-webkit-search-cancel-button`）在 `pr-12` 里，在 `⌘K` 提示的左边，不重叠（审查 B 的截图，Firefox 没有清除按钮）。
- 无效时的提示（`Alert`）照旧在框下。

#### 4.3.3 快捷键

`W/app/shortcuts.ts`：

```ts
/** composing tells whether a key press ends or goes into an IME's composition: Safari's Enter after it has 229. */
export function composing(event: Pick<KeyboardEvent, "isComposing" | "keyCode">): boolean { … }

/**
 * dialogOpen tells whether a dialog is open: the page's own shortcuts then leave the keys to it. The drawer of a
 * phone's width is one (M13/P2/S3), but not to the search's shortcuts, which focus the box inside it.
 */
export function dialogOpen({ besideDrawer = false }: { besideDrawer?: boolean } = {}): boolean {
  const dialogs = besideDrawer ? '[role="dialog"]:not([data-shell]), [role="alertdialog"]' : '[role="dialog"], [role="alertdialog"]';
  return document.querySelector(dialogs) !== null;
}
```

`onMac` 挪进 `W/lib/platform.ts`（决定 797）；`shortcuts.ts` 不再引它（`isMod`、`isModShift` 收的是 `mac` 参数），五个调用者与新的 `search-shortcuts.ts` 从 `lib/platform` 引（审查 C 的 P2C1-N2）。

`useSearchShortcuts({ field, actions })`（`window` 上的 `keydown`，`const [mac] = useState(onMac)`；`useEffect(() => { … }, [mac, field, actions])`）：

```ts
const k = isMod(event, "k", mac);
if ((!k && !isModShift(event, "f", mac)) || composing(event) || field.current === null) return;
if (k && (event.defaultPrevented || document.activeElement?.closest(".cm-editor"))) return;
event.preventDefault();
if (event.repeat || dialogOpen({ besideDrawer: true })) return;
if (actions.closed()) actions.open("search");
else { field.current.focus(); field.current.select(); }
```

`field.current === null`：工作区读取中、读取失败与 404 时没有框，两个键都不管（决定 783）。`actions.closed()` 只在窄屏为真；`actions` 一生不变、经 ref 读当前的 key 与 `trapping`，依赖 `[mac, field, actions]` 也就不变，监听只注册一次（决定 790）；`open("search")` 打开之后在 passive effect 里聚焦并选中框（`inert` 去掉、困住挂上之后，决定 790）。

#### 4.3.4 抽屉、细栏与遮罩

`W/app/shell-sidebar.tsx` 加 `useDrawer()`（`WorkspaceLayout` 调用，把它交给 `ShellSidebar` 与 `useSearchShortcuts`；它的 effect 因此是去向页的祖先的，在页的 passive effect 之后跑，决定 790），返回 `{ narrow, trapping, menu, root, actions }`。`actions`（`closed`、`open`、`close`、`quietClose`、`inDrawer`）在 `useDrawer` 的一生里不变，经 `now`（`{ key, trapping, narrow }` 的 ref，在每次提交的 layout effect 里更新）读当前的值：树的行是 memo、点按时读，`useSearchShortcuts` 的依赖 `[mac, field, actions]` 不变、监听只注册一次（决定 790；审查 A 的 P2A1-M2，审查 C 的 P2C2-M1）。同一个模块导出 `Drawer`（上下文，默认值是什么也不做、`closed()` 与 `inDrawer()` 为假的一份）与 `useDrawerActions()`，`WorkspaceLayout` 以 `drawer.actions` 给出：树的两种链接读 `inDrawer`（决定 782），`UnsavedGuard` 调 `quietClose`（决定 799）。下面的 hook 照写进了探针文件，以仓库的 oxlint 配置零警告（4.4）：

```ts
const narrow = useNarrow();
const location = useLocation();
const type = useNavigationType();
const [drawer, setDrawer] = useState<{ at: string; to: "first" | "search" } | null>(null);
// A wider window closes it, not to open again as the window narrows: the width of the last render, as
// search-box.tsx:83-88 keeps the q it took (no effect: react/set-state-in-effect).
const [wide, setWide] = useState(!narrow);
if (wide === narrow) {
  setWide(!narrow);
  if (!narrow) setDrawer(null);
}
const trapping = narrow && drawer !== null && drawer.at === location.key;
const menu = useRef<HTMLButtonElement>(null);
const root = useRef<HTMLDivElement>(null);
const toMenu = useRef(false);
const now = useRef({ key: location.key, trapping, narrow });
useLayoutEffect(() => {
  now.current = { key: location.key, trapping, narrow };
}, [location.key, trapping, narrow]);
const actions = useMemo<DrawerActions>(() => ({
  closed: () => now.current.narrow && !now.current.trapping,
  open: (to) => setDrawer({ at: now.current.key, to }),
  close: () => { toMenu.current = true; setDrawer(null); },
  quietClose: () => setDrawer(null), // 799: the focus stays where it is
  inDrawer: () => now.current.trapping,
}), []);
// The navigation's commit: the focus leaves the drawer now, no setState (790).
useLayoutEffect(() => {
  const active = document.activeElement;
  if (drawer !== null && drawer.at !== location.key && active instanceof HTMLElement && root.current?.contains(active)) active.blur();
}, [drawer, location.key]);
// After the page's passive effects: the entry left, its state cleared; the focus to the menu unless the page takes it.
useEffect(() => {
  if (drawer !== null && drawer.at !== location.key) {
    // oxlint-disable-next-line react/set-state-in-effect -- the reason in 02-P2 4.4
    setDrawer((d) => (d !== null && d.at !== location.key ? null : d));
    if (!headingTakes(location.state, type)) menu.current?.focus();
  }
}, [drawer, location.key, location.state, type]);
const to = drawer?.to;
useEffect(() => {
  if (trapping) {
    // "search": the box, selected; "first": the root's first focusable element (the switcher)
  } else if (toMenu.current) {
    toMenu.current = false;
    menu.current?.focus();
  }
}, [trapping, to]);
useEffect(() => {
  const element = root.current;
  if (!trapping || element === null) return undefined;
  const onKey = (event: KeyboardEvent) => { /* Escape, below */ };
  element.addEventListener("keydown", onKey);
  return () => element.removeEventListener("keydown", onKey);
}, [trapping, actions]);
return { narrow, trapping, menu, root, actions };
```

`ShellSidebar({ drawer, place, top, children })`：

```tsx
<>
  <div className="flex h-11 shrink-0 items-center gap-1.5 border-b border-border bg-background px-2 md:hidden">
    <Button ref={menu} variant="ghost" size="icon-sm" aria-label={t("shell.menu")} aria-expanded={trapping} aria-controls={id} onClick={() => actions.open("first")}>
      <PanelLeft />
    </Button>
    <span className="min-w-0 truncate text-compact font-medium text-foreground">{place}</span>
  </div>
  <FocusScope.Root asChild trapped={trapping} loop={trapping} tabIndex={trapping ? -1 : undefined} onMountAutoFocus={prevent} onUnmountAutoFocus={prevent}>
    <div ref={root} id={id} data-shell data-drawer={narrow ? (trapping ? "open" : "closed") : undefined}
      inert={narrow && !trapping}
      role={trapping ? "dialog" : undefined} aria-modal={trapping || undefined} aria-label={trapping ? t("shell.menu") : undefined}
      className="flex flex-col bg-sidebar outline-none [overflow-anchor:none] max-md:fixed max-md:inset-y-0 max-md:left-0 max-md:z-40 max-md:w-63 max-md:max-w-[85vw] max-md:shadow-dialog max-md:data-[drawer=closed]:invisible max-md:data-[drawer=closed]:-translate-x-full max-md:data-[drawer=open]:motion-safe:transition-[translate] md:sticky md:top-0 md:h-svh md:w-63 md:shrink-0 md:self-start md:border-r md:border-border">
      {/* 三段照 4.1.4 */}
    </div>
  </FocusScope.Root>
  {trapping && <div aria-hidden className="fixed inset-0 z-30 animate-fade-in bg-overlay md:hidden" onClick={() => actions.close()} />}
</>
```

- 依赖都写全（审查 C 的 P2C2-M1）：导航的两个 effect 依赖 `drawer` 本身（初稿读的是它的 ref `was`），打开时 `drawer.at` 就是这次的 key、Esc 与遮罩关上之后是 `null`，多跑的那几次条件不成立、什么也不做；`location.state`、`type` 与 `location.key` 一起变（每次导航新的 key）。清掉写成函数式的 `setDrawer((d) => …)`：提交与这个 passive effect 之间按下的"打开"（`open` 记下的已是新的 key）不被闭包里旧的 `drawer` 吞掉（审查 A 的 P2A3-N1；只是时机，不设测试格）。打开时聚焦的 effect 依赖 `[trapping, to]`：`to` 只在打开时变，关上时与 `trapping` 同一次变。
- `FocusScope.Root` 的 `tabIndex` 只在困住时是 `-1`（决定 781），根有 `outline-none`。不要 jsx-a11y 的豁免（4.4）：`role` 是条件表达式，`prefer-tag-over-role` 只看字面的；遮罩带 `aria-hidden`，`click-events-have-key-events` 与 `no-static-element-interactions` 都不看它（键盘关它用 Esc）。
- 导航：`headingTakes(location.state, type)` 从 `W/app/arrival.ts` 引（`type` 是 `useNavigationType()`；审查 A 的 P2A2-N1：各处都照这个签名写）：`arrived`、`POP` 到带 `opened` 的一条（同 `:190-210` 的 `arrival` 与 `fromNoOne`）与带 `Located` 的一条（决定 790）。
- Esc 的 `onKey`：`if (event.key !== "Escape" || composing(event)) return; const at = event.target; if (at instanceof HTMLInputElement && at.type === "search" && at.value !== "") { setTimeout(() => { if (at.value !== "") actions.close(); }); return; } event.preventDefault(); actions.close();`（决定 782、797）。
- 变宽：渲染时按上一次的宽度判断（上面的 `wide`），`narrow` 变假的那一次渲染里清掉状态，不动焦点（`toMenu` 不设）。
- 被拦下的导航（决定 799）：`UnsavedGuard` 以 `useDrawerActions()` 取 `actions`，`onCloseAutoFocus` 在"留下"那一支里 `actions.quietClose(); stay();`。确认框开着时抽屉的 `FocusScope` 被它那一层暂停，"留下"把焦点给 `main` 里的编辑器之后 Radix 才恢复抽屉那一层；这次关上之后 `trapped` 为假、根 `inert`，焦点留在编辑器。
- 树：`page-tree-item.tsx:153` 的 `state: opened` 改为 `state: context.inDrawer() && createPath(location) !== href ? arrived : opened`（`context` 加 `inDrawer`，即 `useDrawerActions().inDrawer`；同一地址照旧 `opened` 与 `replace`）；"目录""历史"两行在点按时同样定（4.2.3：不在渲染时，审查 A 的 P2A2-M1）。
- 细栏的 `place`：`WorkspaceLayout` 给（地址里的笔记本在列表里时它的名字，否则工作区名）。

#### 4.3.5 快速切换

`quick-switch.tsx`：

- 列表（`:154-160`）：`className="max-h-80 overflow-y-auto rounded-control border border-border p-1"`。
- 选项（`:163-183`）：`cn("flex cursor-pointer items-center gap-2.5 rounded-row px-2.5 py-1.5 text-row", index === current && "bg-active ring-2 ring-ring ring-inset")`，里面 `<File aria-hidden className="size-4 shrink-0 text-faint" />` 与 `<div className="min-w-0 flex-1"><span className="block truncate">{page.name}</span>{" "}<span className="block truncate text-xs text-faint">{…}</span></div>`。
- 底部（`:186-192`）：`<div className="flex items-center justify-between gap-3">`，`output` 照旧在左，右边 `<p aria-hidden className="shrink-0 text-xs text-faint max-md:hidden">{t("page.quickSwitchKeys")}</p>`。

### 4.4 lint（三个 Step）

`make lint-web` 的 oxlint 是零警告（`package.json:8` 的 `oxlint --max-warnings=0 tools web e2e`）。设计者把第 4 节写到的 hook 与 JSX 照写进 15 个探针文件（`p2-design/lint-r2/web/apps/web/src/`：`app/` 的 `arrival.ts`、`corner.tsx`、`layout.tsx`、`narrow.ts`、`session-state.ts`、`shell-sidebar.tsx`、`shortcuts.ts`、`user-menu.tsx`，`lib/platform.ts`，`pages/notebook/` 的 `page-tree.tsx`、`page-tree-item.tsx`，`pages/page/unsaved-guard.tsx`，`pages/workspace/` 的 `notebook-slot.tsx`、`search-shortcuts.ts`、`workspace-layout.tsx`；目录照 `W/` 的布局、配置是复制来的仓库 `.oxlintrc.json`，`web/**/src/**` 的覆盖也生效），oxlint 1.86.0 的 `oxlint -c .oxlintrc.json --max-warnings=0 web` 以 0 退出。审查 C 的探针报出的三类（`react/refs`、`react-hooks/exhaustive-deps`、`react/exhaustive-effect-dependencies`）照下面改掉了；设计者的探针另报出 `react/set-state-in-effect`（导航之后与变宽时的 `setDrawer(null)`），变宽的改在渲染时，导航之后的一处豁免。

P2 要的豁免只有一处是新的，连同改到的代码里沿用的列在这里；实施时不另加，要加的先改本节：

| 位置 | 规则 | 理由 |
|---|---|---|
| `W/app/shell-sidebar.tsx` 的 `useDrawer`：导航之后的 passive effect 里清掉状态的 `setDrawer((d) => …)`（新） | `react/set-state-in-effect` | 离开的那一条的状态要在提交之后清掉，不清的话后退回到打开时的那一条，抽屉又开；给菜单按钮的焦点要等去向页的 passive effect（决定 790），与清掉是同一件事、在同一个 effect 里。放进 layout effect 是初稿的 I1（`setState` 先冲掉上一次提交的 passive effect）；放进渲染里，passive effect 就不知道"这次导航关上了开着的抽屉"，给不给菜单按钮无从判断 |
| `W/pages/notebook/page-tree-item.tsx:394`：`TreeMenu` 每次提交之后的重量（沿用） | `react-hooks/exhaustive-deps` | 照旧；`useSlotMoves()` 只让它重画，不改这个 effect（决定 800） |
| `page-tree-item.tsx:157`：`PageList` 的 `ul` 上委托的点按（沿用） | `jsx-a11y/click-events-have-key-events`、`jsx-a11y/no-noninteractive-element-interactions` | 照旧（链接上的 Enter 也是一次点按）；"目录""历史"两行的点按在链接自己的 `onClick` 上，不要豁免 |

不要的豁免：初稿 4.3.4 写的抽屉根的 `jsx-a11y/prefer-tag-over-role` 与遮罩的 `click-events-have-key-events`、`no-static-element-interactions`。`role` 是条件表达式、遮罩带 `aria-hidden`，这三条都不报；设计者另以字面的 `role="dialog"` 与不带 `aria-hidden` 的遮罩核对过，那样才报。

为了零警告，下面几处与之前的文字不同，各节已照改：
- `PageTree` 的 `key`（`location.key` 的 ref）在 `useInsertionEffect` 里更新，不在渲染时（决定 791，4.2.5）。
- `useDrawer` 返回一生不变的 `actions`，`useSearchShortcuts` 只拿它（4.3.3、4.3.4）。
- `useDrawer` 里导航的两个 effect 依赖 `drawer` 本身，不用读它的 ref；打开时聚焦的 effect 依赖 `[trapping, to]`；变宽在渲染时按上一次的宽度清掉（4.3.4）。
- `reveal` 的看守是一个一生不变的对象、只改字段，卸下时的清理在 effect 开头取下它（4.2.5）。
- `PageRow`、`SlotHolder` 的 layout effect 先取下 `ref.current`、判空（4.2.2、4.2.4）。

## 5. e2e

**S1**

- `e2e/stories/identity/a8-profile.spec.ts:87-89`："主题"按钮没有了，改为 `await accountMenu(page, "Ada Lovelace").click(); await page.getByRole("menuitem", { name: "外观", exact: true }).click(); await expect(page.getByRole("menuitemradio", { name: "深色" })).toBeChecked(); await page.keyboard.press("Escape");`（一次 Esc 关上整个菜单，4.1.6）；`:81-82` 的注释"as the top bar's theme menu shows too"改为"as the account menu's Appearance shows too"。
- `e2e/stories/search/se1-search.spec.ts:286-288`：搜索框、"搜索"按钮与无效的框的焦点量的是元素外的底，S1 起是 `--sidebar`。预期：琥珀对 `#f5f5f6` 2.84、对 `#141517` 8.41；红对 `#f5f5f6` 4.02、对 `#141517` 7.28。S1 在 Chromium 上实量，照 00 号文档 4.7 的规则（实量 − 0.1，向下取到 0.01）改 `box`、`button`、`invalid` 两种主题的下限，`:281-285` 的注释加"the box and its button on the sidebar re-measured at M13/P2/S1"。`select` 与 `link` 在结果页上，底不变。
- 不改，理由：
  - `accountMenu`、`signOutThroughMenu`（`e2e/fixtures/auth-pages.ts:46-54`）按名称精确匹配，头像的首字 `aria-hidden`，名称照旧；同时只有一个账户按钮（侧栏的或右上角的，决定 769）。a6（`a6-sign-out.spec.ts:31-38`，`/acme` 是 404：右上角在 404 显示之后出现，`openSignedIn` 先等它）、a7（`a7-change-password.spec.ts:94,118`，个人设置：右上角）、a8（`:70,91`：右上角）、s2（`s2-web-app.spec.ts:57-60`，新建工作区页：右上角；版本行照旧在菜单里，一打开就读到）、c6（`c6-expired.spec.ts:112-113`）与 c8（`c8-stream-life.spec.ts:225-229`）一 `goto` 工作区就点：读取中没有账户按钮，点的是侧栏里的。
  - `switcher`（`e2e/fixtures/workspace-pages.ts:50-52`）在 `[data-shell]` 里按名称找按钮：切换器照旧在根里，名字照旧（首字 `aria-hidden`）；w3（`w3-shell-switch.spec.ts:18-22`）数 `[data-shell]`：照旧一个、在 `main` 之外。
  - 侧栏里的"首页""设置""新建笔记本"（`createNotebookWith`）按名称找，名称不变（图标 `aria-hidden`）。

**S2**

- `e2e/fixtures/notebook-pages.ts:17-26`：`notebookGroups` 的 `list.getByRole("link").allTextContents()` 改为 `list.evaluate((ul) => [...ul.children].map((li) => li.querySelector("a")?.textContent ?? ""))`（每一项只取第一个链接：当前那一项是笔记本行的链接，后面还有树与"更多"）；注释写明；17 处调用不动。n1:103 在新建的笔记本首页上读它（树空，"更多"四个链接在那一项里），守住这一改法（7.2 第 16 条）。
- 新故事 `e2e/stories/page/pg16-tree-menu.spec.ts`（照 pg2 的写法：`newTeam`、`joinOnboarded`、`createNotebook`、`createPage`、`goto`、`pageTree`、`renameDialog`）：
  - **PG16 (page): a row's menu in the tree opens at its button, by the pointer and by the keyboard; Escape closes it, Rename opens its dialog, which Escape cancels; the focus goes back to the row's button each time**：编辑者，笔记本"Plans"里"Roadmap""Notes"两页，在"Roadmap"上；点"Actions for Notes"，名为"Actions for Notes"的菜单出现、有"New subpage""Rename""Move to…""Delete"四项；Esc，菜单不在、按钮有焦点；按钮上按 Enter，菜单出现、"New subpage"有焦点（jsdom 里 `page-tree-writes.test.tsx:798-811` 守着同样的事，这里在三种引擎里看）；选"Rename"，对话框"Rename Notes"出现、"Title"有焦点且是"Notes"；按 Esc（对话框只有 Save，没有 Cancel：`W/app/rename-form.tsx`），对话框不在、按钮有焦点。不写数据库（只核对菜单的行为，接口与 MCP 版本豁免，8.2 第 10 条）。
  - **PG16 (page): the page shown has its row brought into the left column, which scrolls itself; the window does not scroll**：60 个根下的页（"Page 01"–"Page 60"，"Page 60"的正文 200 行，页比视口高）；`goto` "Page 60"：树里它的链接 `toBeInViewport()`，`[data-sidebar-scroll]` 的 `scrollTop` 大于 0，`window.scrollY` 是 0；以 `quickSwitchFor(page, "Page 01")`（`e2e/fixtures/wiki-pages.ts:181-190`）打开快速切换、按 Enter 到"Page 01"：它的链接 `toBeInViewport()`；再以快速切换回到"Page 60"：它的链接 `toBeInViewport()`、`window.scrollY` 仍是 0（第三次导航，中间都有按键：审查 B-I1 的情形）；再到 375 宽（`phoneWidth`；md 以下侧栏在页之前、自己不滚，S3 的抽屉之前）：先 `goto` 笔记本首页，再 `goto` "Page 60"：它的链接在，`window.scrollY` 是 0。`scrollIntoView` 只在这一段把窗口滚到那一行：宽屏时整列粘在视口里，当前行在正文画出之前就带进来，文档那时只有视口高，`scrollIntoView` 也滚不动窗口（变异 24，10.2）。先经首页：同一地址再 `goto` 时 Chromium、WebKit 留在原来的历史项里、`arrived` 还在，标题取焦点、窗口滚到它（M3、M12 既有，S2 的审查记录）。
- `e2e/playwright.config.ts:3-4`：`threeBrowsers` 加 `page\/pg16-`（不与 `pg1-` 重叠：`pg1-` 后面是连字符）；`tools/e2e-projects/check.mjs:22-29` 的 `REQUIRED` 加 `["firefox", "page/pg16-tree-menu.spec.ts", "PG16 (page): a row's menu in the tree"]` 与 webkit 一行。
- 不改，理由：`pageTree`、`treeTitles`、`newPageWith`、`choosePageAction`、`renamePageWith`（`e2e/fixtures/wiki-pages.ts:19-90`）：树的 `nav` 名照旧，笔记本行在 `nav` 之外，＋照旧在 `nav` 里、看得见（`opacity-40`），树的菜单的项照旧；`treeRow`（`:28-30`）以链接的父元素为行：链接照旧是行的直接子元素；"更多"的 18 处（`healthLink` 等）按名称找，名字在 `sr-only` 里；lw12（`lw12-downgrade.spec.ts:160-191,262`）按名称看"目录""历史"两行；pg1:180、pg12:256、lw10:337 读 `treeTitles`。

**S3**

- 新故事 `e2e/stories/shell/ui1-narrow-drawer.spec.ts`（`newOnboardedTeam`，笔记本"Plans"里"Roadmap""Notes"与"Roadmap"的子页"Q1"，375×800；新夹具 `e2e/fixtures/shell-pages.ts`：`shell(page)` 是 `[data-shell]`，`drawerButton(page)` 是名为"Navigation"的按钮，`openDrawer(page)` 点它并等名为"Navigation"的 `dialog`；`inert` 都用会重试的 `expect(shell(page)).toHaveJSProperty("inert", …)`，"关上之后"都等焦点落到某个元素，不等"没有某某"，审查 C-N7）：
  - **UI1 (page): at 375 wide the left column is a drawer: closed it is inert and off the screen; the menu opens it with the focus in it, which Tab keeps there; Escape, the overlay and a switch of workspace close it, the focus back on the menu**：另以 `createWorkspace`（`e2e/fixtures/workspaces.ts:20`，照 w3；`pat` 是 `newOnboardedTeam` 给的）建第二个工作区"Beta"；在"Roadmap"上；`shell` 的 `inert` 为真、`toBeHidden()`；`expectFits(page, page.getByRole("main"))`；`openDrawer`：`drawerButton` 是 `aria-expanded="true"`，切换器有焦点；Shift+Tab 到账户按钮，Tab 回切换器；以程序把焦点放到抽屉外（`page.evaluate` 里 `main h1` 的 `.focus()`：遮罩挡着点击，只能用程序），焦点回到抽屉里（切换器；审查 A-M5 = C-I2：只有 `loop` 时 Tab 也回绕，这一步才分得出 `trapped`；探针 T11 三种引擎都拉回）；`expectFits(page, shell(page))`；Esc：`shell` 的 `inert` 为真、`drawerButton` 有焦点；再开，在搜索框里打"x"再按 Esc：Chromium、Firefox 里框空了、抽屉仍开着，再按 Esc 才关上，WebKit 不清空、一次关上（`browserName` 分开写，决定 797，探针 T10）；再开，点遮罩（`page.mouse.click(360, 400)`）：同上；`page.emulateMedia({ reducedMotion: "reduce" })` 之后再开：`shell` 的 `transitionDuration` 是 `0s`（审查 A-M2，决定 781）；`page.emulateMedia({ reducedMotion: null })`，再开，以切换器换到"Beta"（`switchWorkspace`，`e2e/fixtures/workspace-pages.ts`）：`shell` 的 `inert` 为真、`drawerButton` 有焦点（审查 C 的未核实：Radix 的下拉在 `setTimeout(0)` 里把焦点还给切换器，真浏览器里那时切换器已 `inert`、还不了；jsdom 没有 `inert`，这一步只在这里核对，7.1）。
  - **UI1 (page): Ctrl+K or Cmd+K opens the drawer at the search box; a search from it closes it, the results' title takes the focus**：`ControlOrMeta+k`：抽屉开着、`searchBox` 有焦点；`searchFor(page, "Roadmap")`：结果页的 `h1` 有焦点、`shell` 的 `inert` 为真。本机 macOS 走 `Meta+K`，`Control+K` 只有 Linux 的 CI 走（决定 795）。
  - **UI1 (page): a page opened from the drawer's tree closes it, the page's title takes the focus; one in a folder, opened from the search, has its title in view**：`openDrawer`，点树里的"Notes"：`pageHeading(page, "Notes")` 有焦点、`shell` 的 `inert` 为真；`openDrawer`，在框里搜"Q1"、在结果页点它：`pageHeading(page, "Q1")` 有焦点且 `toBeInViewport()`（M5-polish 35.7：展开的树在抽屉里，不在页之上把标题推下去；审查 C-M11）。
- `e2e/playwright.config.ts:3-4`：`threeBrowsers` 加 `shell\/ui1-`；`check.mjs` 的 `REQUIRED` 加 `["firefox", "shell/ui1-narrow-drawer.spec.ts", "UI1 (page): at 375 wide the left column is a drawer"]` 与 webkit 一行。UI1 不带 `@narrow`：它在 `threeBrowsers` 里，`webkit-narrow` 的 `testIgnore` 本来就不跑它（`check.mjs` 也不许）。
- se1（决定 794）：S3 起"button"一项的环画在垫底上，量到的是框的底（预期亮 3.10、暗 6.95），照 00 号文档 4.7 的规则（实量 − 0.1，向下取到 0.01）改它两种主题的下限（照预期是 2.99、6.85）；"box"与"invalid"照旧量到侧栏（预期与 S1 相同：2.84、8.41、4.02、7.28），与 S1 的读数差超过 0.01 时同样改，没变不改；`:281-285` 的注释写明"button"量的是垫底；读数与下限写进记录。
- 不改，理由（375 px 下都不碰侧栏；抽屉关着时 `fixed` 且在画面之外，探针表明不撑宽页面）：pg2（`pg2-rename-page.spec.ts:183,210`，`@narrow`：对话框在 1280 宽时打开，窄了只量对话框；Esc 关对话框，抽屉关着不理）、mc3（`mc3-write-edit.spec.ts:193`，只看 `main`）、se1（`se1-search.spec.ts:412`，在 1280 宽时搜索、之后才窄）、lw3（`lw3-source-duplicates.spec.ts:310-380`：同 pg2，行菜单都在 1280 宽时点）、as3（`as3-render.spec.ts:386`，只量文章）、lw8（`lw8-health.spec.ts:648`，只量页宽）、lw13（`lw13-index.spec.ts:306`，只看目录）；`webkit-narrow` 的必需格（`check.mjs:26-28`）不变。`searchBox`、`searchFor`（`e2e/fixtures/wiki-search.ts:8-22`）：照旧是 `role=search` 里名为"Search terms"的 `searchbox`；se2:209,225,274 的描述在 `sr-only` 里照旧；`quickSwitchFor`（`e2e/fixtures/wiki-pages.ts:181-190`，pg11、pg12）读选项的两个 `span`：照旧两个。lw8:710-722 点框的正中：正中是输入框（"搜索"按钮在左端，提示 `pointer-events-none`），侧栏粘住之后框总在视口里，`toBeInViewport()` 照旧成立。

## 6. 实施步骤

三个 Step 各在自己的 worktree 里，分支从 `m13-ui-refresh` 拉出。每个提交单独为绿：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`（决定 788：逐个提交的核对也经 `heavy.sh`，06-environment 第 3 节）；改了行为、夹具或故事的提交另经它跑完整的 `CI=1 make e2e`（决定 795(b)：本机约 5 分钟，`make build` 与三种浏览器加 `webkit-narrow` 的 Playwright 各约一半），只败与基线同行同消息的那几格。新的测试先写、在没改的树上看到它红，再改代码。动了 `tools/e2e-projects/check.mjs` 的提交，说明的作用域写上 `tools`（审查 C-N12）。

### 6.1 S1

- [ ] **基线**（决定 795(a)）：S1 的 worktree 建好、还没改任何文件时，在 `m13-ui-refresh` 的头上跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make e2e`，把每一格失败的文件:行（Playwright 报告里错误所在的那一行，不是格的开头行，决定 795(a)）与消息的第一行记下，写进 S1 的记录，第 1 节与 7.3 的三格照它改写；第 1 节的三格之外还有红的，先停下汇报。
- [ ] **第 0 步**：粘住的侧栏的探查（00 号文档第 10 节要它是 P2/S1 的第一个动作）：设计者已在静态页上做过（第 1 节末），应用里的一次要等侧栏粘住之后才有东西可看，所以在 S1 的提交 2 改完之后、提交之前做：经 `heavy.sh` 起应用，在一本 3000 页的笔记本里（`e2e/perf` 的造数，或 API 循环建页）于三种引擎里看：树分批画行时与侧栏自己滚到底时窗口的 `scrollY` 不变；窗口滚到页中间，页里在读者之上长高（展开一个 callout）时读者留在原处。脚本不提交，结果写进 S1 的记录。不成立时停下汇报。
- [ ] **提交 1 `web, e2e: the account in the column's foot and at the top right, the top bar gone (M13/P2/S1)`**：先写 7.1 的 S1 各格（`layout`、`layout-error`、`routes`、`route-error`、`profile-page`、`user-menu`、`initial`、`button`、`dropdown-menu`），看到它们红；改 4.1.2（`corner.tsx`；`useSession` 挪进 `session-state.ts`）、4.1.3、4.1.6、4.1.7、`settings-layout.tsx:27`、`session-unavailable.tsx`、`routes.tsx` 的 `handle`、切换器的"新建工作区"；`UserMenu place="row"` 先放进今天的侧栏的末尾（`workspace-layout.tsx:127` 之后）；a8:81-89；`make e2e`；绿。
- [ ] **提交 2 `web, e2e: the column on the sidebar's colour, stuck at wide, its switcher at the top (M13/P2/S1)`**：`workspace-layout.test.tsx:84-98` 先改、先红；`ShellSidebar`（4.1.4）、切换器、`NavItem`、笔记本列表（4.1.5）；第 0 步的探查；se1 的三项下限照第 5 节重量；`make e2e`；绿。
- [ ] 7.2 的 S1 各条变异；B 审查者截图之前，实施者先自己截 7.4 的 S1 几张看一遍。
- [ ] **提交 3 `docs: …(M13/P2/S1)`**：8.2 第 1、2 条与第 12 条的 S1 一行；S1 的审查记录（含基线）；本文 10.1 与照实改的各节；00 号文档第 12、13 节。

### 6.2 S2

- [ ] **提交 1 `web, e2e: the open notebook in its item of the list, its row, tree and places in a holder that keeps them (M13/P2/S2)`**：先写 `notebook-slot.test.tsx`、`notebook-nav.test.tsx:207-208` 与 `general-page.test.tsx:36-40` 的改法，看到红；4.2.2（含 `useSlotMoves` 与 `TreeMenu` 的通知，决定 800）、4.2.3（笔记本行、＋、齿轮、"更多"）、`intoColumn`；`notebookGroups`（第 5 节）；`make e2e`；绿。
- [ ] **提交 2 `web, e2e, tools: the tree's rows restyled, the row shown brought into the column, and its menu in the three browsers (M13/P2/S2)`**：先写 reveal 的 vitest、`readers-input` 的 `within` 一格与 PG16，看到红；4.2.4、4.2.5、"目录""历史"两行、`move-page-dialog.tsx` 的间距；`threeBrowsers` 与 `REQUIRED`；`make e2e`；绿。
- [ ] 7.2 的 S2 各条变异；7.4 的 S2 截图。
- [ ] **提交 3 `docs: …(M13/P2/S2)`**：8.2 第 3 条的 S2 部分、第 4、5、6 条、第 10 条的 S2 部分与第 12 条的 S2 一行；审查记录；本文 10.2；M12-polish 的"处理进展"一行（第 14 项、37、38.5）；00 号文档第 12、13 节。

### 6.3 S3

- [ ] **提交 1 `web, e2e: the pill search box, Mod+K beside Mod+Shift+F from the shell, the quick switch restyled (M13/P2/S3)`**：先改 `search-box.test.tsx:34,44,221` 并写 ⌘K 的格，看到红；4.3.2、4.3.3（`composing` 导出，`dialogOpen` 先不加参数，`onMac` 挪进 `W/lib/platform.ts`）、`useSearchShortcuts`、4.3.5；se1 的"button"照第 5 节重量；在三种引擎里探一次主区开着 Radix 的模态菜单（页的"⋯"菜单、树的行菜单）时按 Mod+K、Mod+Shift+F（审查 A 的未核实）：预期菜单的困住把焦点拉回菜单、按键只被 `preventDefault`，同对话框开着时；不是这样（框拿到焦点、菜单留着）时 `dialogOpen` 另认 `[role="menu"]`、`search-box.test` 加一格，写进记录；`make e2e`；绿。
- [ ] **提交 2 `web, e2e, tools: the drawer and its strip at a phone's width (M13/P2/S3)`**：先写 `shell-drawer.test.tsx`、`arrival.test.tsx` 的 `headingTakes` 格与 UI1，看到红（`narrow.test.ts` 在 S1 已写，决定 801）（UI1 的 ⌘K 一格先在 Firefox、WebKit 里各跑一次，确认浏览器不吞掉 `Cmd+K`；吞掉时这一格在那个引擎里改按 `Mod+Shift+F`，写进记录）；4.3.4、`dialogOpen({ besideDrawer })`、树的 `inDrawer`、`arrival.ts` 的 `headingTakes`、`unsaved-guard.tsx` 的"留下"关抽屉（决定 799）；PG16 第二格的 375 宽一段改在开着的抽屉里核对（树关在抽屉里时它照样成立，却不再抓变异 24；10.2）；`threeBrowsers` 与 `REQUIRED`；`make e2e`；绿。
- [ ] 7.2 的 S3 各条变异；7.4 的 S3 截图与强制色模式一张。
- [ ] **提交 3 `docs: …(M13/P2/S3)`**：8.2 第 3 条的 S3 部分、第 7、8、9 条、第 10 条的 S3 部分与第 12 条的 S3 一行；审查记录（含 se2 的 Linux 那几步与 `Control+K` 只靠 CI 的说明，决定 795(d)）；本文 10.3；M12-polish 的"处理进展"一行（26.3、35.7、第 8 项的 `onMac`；26.8、26.10 留给 v0.2）；00 号文档第 12、13 节；Phase 完成的收尾项（8.1 第 6 条）。

## 7. 测试与验证

### 7.1 每项的测试

菜单开着时 Radix 的 `hideOthers` 把菜单之外的部分标成 `aria-hidden`，`getByRole` 找不到它们（审查 C-M3）：下面"菜单仍开着"之后的断言都只找菜单里的项，要找页上的东西先按 Esc 关上；设计者逐格查过，只有 `routes.test.tsx:27` 要改。

| 项 | 测试 |
|---|---|
| 主题在账户菜单的外观里切换 | `W/app/layout.test.tsx:8-19` 改：在 `/lab` 点"Ada"、"Appearance"、"Dark"，`<html>` 带 `dark`；菜单仍开着（`menuitem`"Appearance"还在）（S1） |
| 右上角 | `layout.test.tsx` 新格（S1）：未登录（`/sign-in`）时只有"Appearance"按钮，菜单里主题与语言两组；工作区的页上只有一个"Ada"，在 `[data-shell]` 里、在 `nav` 之后；工作区的列表读取中没有"Ada"（扣住 `GET workspaces`，等这个请求发出之后再断言：那时 `me` 已到、只差工作区，不会在守卫的 Loading 时就空过；审查 C-M4），读完是 404（`/nowhere`）时右上角有"Ada"、不在 `[data-shell]` 里；列表答 500 时同样；应用内从 `/nowhere` 点"Go to the home page"回到 `/lab`：恰好一个"Ada"（审查 C-M5）；`/settings/profile` 上点"Back to the workspace"，到 `/lab`、`h1`"Lab"有焦点；已登录的人载入 `/settings/profile`、扣住 `POST /auth/refresh`（会话启动中）：没有"Appearance"、也没有"Ada"，放开之后有"Ada"（审查 A-M4）；`GET /me` 答 500 时在 `/lab`：会话不可用的页，右上角有"Appearance"（决定 792）；简体中文时按钮名是"外观"、"回到工作区"（审查 C-N1） |
| 语言在外观里切换 | `W/app/routes.test.tsx:18-30` 改：点"Ada"、"Appearance"、"简体中文"；`:28` 的"主题"按钮改为 `menuitem`"外观"（菜单仍开着，名字已是中文），然后按 Esc 关上整个菜单，才断言 `:27` 的"首页"链接（审查 C-M3）（S1） |
| 出错页在 `Layout` 之内、交还右上角 | `W/app/route-error.test.tsx:25` 的 `banner` 改为 `button`"Appearance"（未登录）；新格：首次载入就是工作区的一页、它的模块载不进来（`vi.mock` 工作区首页）：错误页里右上角有"Ada"（错误边界在守卫之上、`me` 没读过，右上角自己读它，决定 792；审查 C-M1）（S1） |
| 根路由的错误边界没有右上角 | `W/app/layout-error.test.tsx:18` 的 `queryByRole("banner")` 改为没有名为"Appearance"的按钮、也没有名为"Ada"的按钮（决定 769、793；审查 A-M6）；`:7-11` 的 mock 不改（`useCornerBack` 在 `corner.tsx`）（S1） |
| 个人设置带 `arrived`、`h1` 取焦点 | `W/pages/settings/profile-page.test.tsx:47-56` 改：从 `/lab` 开始（等"Lab"），点侧栏的"Ada"、"Settings"，`h1`"Settings"有焦点、地址是 `/settings/profile`；再从右上角的"Ada"选一次"Settings"：焦点回到"Ada"（同一地址不设 `leaving`，审查 A 的 P2A1-N1）（S1） |
| 个人资料页的偏好与外观相同 | `profile-page.test.tsx:146-166` 改：`:161-165` 的"主题"按钮改为右上角的"Ada"、"外观"，"深色"是 `aria-checked="true"`；标题改为"the preferences are the account menu's"（S1） |
| 账户行 | `W/app/user-menu.test.tsx` 加格（S1）：按钮名恰好是"Ada"，头像的"A"在 `aria-hidden` 里；菜单的次序（Settings、Appearance、Sign out、版本行）；外观里选了"Dark"之后"Light"所在的子菜单仍开着；键盘：Enter 打开、↓ 到"Appearance"、→ 打开子菜单、第一项有焦点、Esc 关上整个菜单、焦点回按钮；简体中文时子菜单的触发项是"外观"（审查 C-N1）。现有五格不动 |
| 账户菜单的位置与宽度 | `user-menu.test.tsx`（S1，实施者加；变异 x3、x5、x9）：账户行的菜单从左端向上开、右上角的从右端向下开（Radix 的 `data-side`、`data-align`）；右上角的菜单带 `max-w-[min(16rem,…)]` 的上限 |
| 窄屏的账户菜单 | `user-menu.test.tsx`（S1，决定 801）：`matchMedia` 打桩成窄屏，`/lab` 与 `/settings/profile` 各一次（`test.each`，审查 C 的 S1C-N2）：没有"Appearance"项，菜单的次序是 Settings、—、Theme、—、Language、—、Sign out、—、版本行，主题组里是 System、Light、Dark；选"Dark"之后菜单仍开着、它是 `aria-checked` |
| `/` 与新建工作区页的右上角 | `layout.test.tsx`（S1，审查之后）：在 `/` 扣住 `GET workspaces` 时没有"Ada"，放开之后恰好一个、在 `[data-shell]` 里；`/` 上列表答 500 时"Ada"在右上角（审查 A 的 S1A-M1）；新建工作区页上有"回到工作区"、点了到 `/lab`、`h1` 有焦点；没有工作区的账户从 `/` 落到新建页时没有它（决定 803） |
| 会话不可用与读一次 | `layout.test.tsx`（S1，审查之后）：refresh 答 503 时在 `/settings/profile` 与 `/lab` 上有"Appearance"、没有"Ada"（审查 C 的 S1C-M1）；会话启动中那一格数 `GET /me` 为 1（守卫与右上角同一个键）；404 上没有"回到工作区"；"回到工作区"那一格从 `/settings/security` 开始（S1C-N3、审查 A 的 S1A-N4）；`route-error.test.tsx` 首次载入那一格数 `GET /me` 为 1 |
| 焦点留在按钮上的两处 | `profile-page.test.tsx`（S1，审查之后）：还有引导步骤的账户在 `/onboarding` 从右上角选 Settings，等它被送回（`?next=%2Fsettings%2Fprofile`），焦点在"Ada"上（审查 A 的 S1A-N2；审查 C 的 S1C1-N2）；从 `/settings/security` 在右上角选 Settings、`h1` 取焦点之后再打开菜单按 Esc，焦点回到"Ada"（S1A-N4） |
| 粘住的侧栏与子菜单里的指针 | L1 (page) 一格的末尾（S1，审查之后，Chromium 1280×720）：长页滚到底，`scrollY` 大于 1000，账户按钮在视口里，`[data-shell]` 的 y 在 1 px 之内为 0（审查 C 的 S1C-M2）；a8 用指针在子菜单里选"浅色"、`html` 去掉 `dark`、子菜单仍开着，再选"深色"（S1C-N6） |
| 首字 | `W/app/initial.test.ts`（S1）："ada"→"A"，"  bob"→"B"，"产品研发"→"产"，"\u{1F469}‍\u{1F4BB} Dev"（一个字素）→"\u{1F469}‍\u{1F4BB}"，`"e\u0301"`（e 加组合重音，一个字素）→`"E\u0301"`（期望值照写分解的形式：`toLocaleUpperCase` 不把它合成预组合的 `"\u00C9"`，审查 C 的 P2C1-N3），""→"" |
| 按钮的 30 px 与胶囊 | `W/components/ui/button.test.tsx` 加格（S1）：`size="sm"` 含 `h-7.5`、`text-compact`；`icon-sm` 含 `size-7.5`；`shape="pill"` 含 `rounded-full`、不含 `rounded-lg`（tailwind-merge） |
| 子菜单 | `W/components/ui/dropdown-menu.test.tsx` 加格（S1）："a submenu opens from its item, by the keyboard too, and animates in, never out"：`SubTrigger` 带 `aria-haspopup="menu"`；→ 打开，子菜单的第一项有焦点；子菜单内容的 `className` 含 `data-[state=open]:animate-pop-in`、不含 `data-[state=closed]:`；Esc 关上两层 |
| 切换器在 `nav` 之外、在搜索之前；"新建工作区"带 `arrived` | `W/pages/workspace/workspace-layout.test.tsx:84-98` 改：`nav` 里的链接只有"Home""Settings"；切换器是 `[data-shell]` 里第一个按钮、在 `role=search` 之前、不在 `nav` 里；新格：点切换器的"New workspace"，新建页的 `h1` 有焦点（S1） |
| 当前笔记本在列表里 | `W/pages/workspace/notebook-nav.test.tsx:207-208` 改：取"Team notebooks"列表的第一项的第一个链接；`:232-235` 不改（名为"研究笔记"的链接只有笔记本行的那个）（S2） |
| 笔记本设置页上的笔记本行 | `W/pages/notebook/general-page.test.tsx:36-40` 改：在 `…/settings/general` 上，侧栏里名为"Plans"的链接（笔记本行，`NavLink end`）不带 `aria-current`（其余留给 P3 的设置模式；审查 C-I4）（S2） |
| 稳定的容器 | `W/pages/workspace/notebook-slot.test.tsx`（新，S2）：(1) 在一本笔记本的页上，它那一项里依次是名为它的 `h2`、"Pages of X"、"More of X"，别的项只有一个链接；(2a) 树里一行的菜单按钮有焦点时，这本笔记本从"我的"挪到"团队"（store 里改 `workspace_access`）：树的 `nav` 是同一个节点、在"Team notebooks"里，焦点仍在那个按钮上，滚动区的 `scrollTop` 让它露出（`getBoundingClientRect` 打桩）；(2b) 开着改名的对话框并打了字时换组：对话框与打的字都在，焦点在对话框里（审查 B-N7：模态的对话框拿着焦点，(2a) 与它分开）；(2c) 开着树的行菜单时换组，这一格在 `act` 里以 `runInAction` 就地改那本笔记本的 `workspace_access`（同一个对象：`PageTree` 不重画，`TreeMenu` 只能由 `SlotHolder` 的通知重画；经列表重读的话 `PageTree` 的回调新建、`TreeMenu` 顺带重画，抓不到变异 51）：菜单仍开着，那一次提交里量过那一行的按钮（`getBoundingClientRect` 的 spy；决定 800）；(3) 编辑中留住的、已不在列表里的笔记本（以 `vi.spyOn` 让根 store 的 `unsavedEdit` 答真，`W/stores/root.store.ts`，再让笔记本的列表重读成没有它）：它的行与树在两组之后、不在任何一组的列表里；(4) 点别的笔记本的链接，它首页的 `h1` 有焦点；(5) 地址里的笔记本 id 是大写的：404 页上这本笔记本照旧是列表里的一个链接，两组之后没有别的（审查 B-M3） |
| 笔记本行、齿轮与＋ | `notebook-slot.test.tsx` 或 `page-tree.test.tsx` 加格（S2）：行的链接指向首页、在首页上 `aria-current="page"`、在一页上没有；齿轮名为"Settings of Plans"、指向 `…/settings/general`；简体中文时是"Plans的设置"（审查 C-N1）；"New page"在树的 `nav` 里；读者没有"New page" |
| 当前行滚进可见范围 | `W/pages/notebook/page-tree-reveal.test.tsx`（新，S2）：`[data-sidebar-scroll]` 与链接的 `getBoundingClientRect` 用 `vi.spyOn` 给出（jsdom 没有布局；链接的 `top` 按"行的偏移 − 滚动区的 `scrollTop`"算，不然读者滚开之后行在桩里仍在范围内，同一页换 hash 那一格抓不到变异 62，审查 C 的 P2C2-N2）；`Element.prototype.scrollIntoView` 先像 `quick-switch.test.tsx:163-166` 那样赋上一个再 spy（jsdom 没有它，`vi.spyOn` 会抛；审查 C-N3），`window.scrollTo` 也是 spy：到一页时滚动区的 `scrollTop` 变到让行露出、两个 spy 都没被调用；行已在范围里时不变；同一次导航里再挂上不再滚；三次以上的导航、每两次之间在主区按一个键并点一下树的行，每一次都滚（审查 B-I1）；先到笔记本首页、在 `[data-sidebar-scroll]` 里按一个键、再开一页：滚（审查 C-M12；按在别处的键看守本来不算，S2 审查的 S2C-M1）；树还没读到时在 `[data-sidebar-scroll]` 上发 `wheel`，读到之后不滚，再到一页、它的行已画出且在视野外：滚（S2 审查的 S2A-N1）；StrictMode 下（`configure({ reactStrictMode: true })`）同样不滚（S2C-N1）；在 `main` 上发 `wheel`，读到之后照滚；在一页上于 `[data-sidebar-scroll]` 发 `wheel`、改 `scrollTop`，再点右栏大纲里的一个标题（同一页的 `#…`；页的正文带一个 `## ` 标题），以及 `replace` 到当前的地址：`scrollTop` 不变；树还没读到时换 hash、读到之后照滚（审查 B 的 P2B1-M1）；树还没读到、看守开着时离开这本笔记本：`window.removeEventListener` 对 `readersInput` 的四种事件各调一次（审查 C 的 P2C3-N1） |
| 看守只算侧栏里的输入 | `W/pages/page/readers-input.test.ts`（新，S2：原来没有这个文件）：`watchReader({ within: "[data-sidebar-scroll]" })` 不算落在它之外的按键、按下、滚轮、触摸，算落在它里面的；不带 `within` 时哪里都算 |
| 阅读视图的等待 | `W/pages/page/reading-view.test.tsx`（S2）："the wait's listeners go once its read settles"改为按监听函数计还在听的：树的看守也在 `window` 的捕获阶段听读者的四种事件，按事件名数到 16 个而不是 4 个 |
| "目录""历史"两行标当前 | `page-tree.test.tsx` 加格（S2，LLM Wiki 的笔记本）：在目录页上"Index"`aria-current="page"`、"History"没有；在一个变更集的页（`…/history/:id`）上"History"不带 `aria-current`（`end`；审查 C-N2）；两行与"更多"里同名的链接图标相同 |
| 树的其余 | 照旧：`page-tree.test.tsx`、`page-tree-writes.test.tsx`（标题的名字、`nav` 里的链接与"New page"）、`page-tree-scale.test.tsx`（`li` 与 `li a` 的数、第一个 `ul` 里没有 `data-discover`、首页的"Notebook settings"链接）、`notebook-layout.test.tsx:84-93`（"更多"的 `textContent`）、`upgrade-read-only.test.tsx:141-147`、`quick-switch.test.tsx` 的两个 `span` |
| 搜索框的两个键 | `W/pages/workspace/search-box.test.tsx:44` 改为 `"Control+K Control+Shift+F"`，`:221` 改为 `"Meta+K Meta+Shift+F"`；`:34` 一格的标题改为切换器之下、两个快捷键（审查 C-N13），描述与标签照旧；占位在简体中文时是"搜索"（审查 C-N1）（S3） |
| ⌘K 与让路 | `search-box.test.tsx` 加格（S3，照 `:30-32` 的 `modShiftF` 写 `modK`）：Mod+K（jsdom 不是 Mac，按 Ctrl+K）聚焦并选中框、`defaultPrevented`；组合中（`isComposing`、229）什么也不做；焦点在一个 `.cm-editor` 里时不 `preventDefault`、框没有焦点；已被 `preventDefault` 的不管；对话框开着时只 `preventDefault`；按住的重复只 `preventDefault`；Ctrl+Shift+K、Ctrl+Alt+K 不是它；macOS 上 Cmd+K（照 `:212-214` 打桩平台）；工作区列表答 500 时（没有框）Mod+K 与 Mod+Shift+F 都不 `preventDefault`（审查 A-M3）。`:176-211` 的 Mod+Shift+F 一格照旧（监听挪到外框之后仍在 `renderApp` 的整个应用里） |
| `dialogOpen` 认得抽屉 | `W/app/shortcuts.test.ts` 加格（S3）：只有 `[data-shell][role=dialog]` 时 `dialogOpen()` 为真、`dialogOpen({ besideDrawer: true })` 为假；另有一个对话框时两者都为真；`composing` 的三种 |
| `onMac` | `W/lib/platform.test.ts`（S3）：`shortcuts.test.ts` 里 `onMac` 的格挪过来，不改 |
| `headingTakes` | `W/app/arrival.test.tsx` 加一格（S3，`test.each`）：带 `arrived` 的与带 `Located` 的（`PUSH`、`REPLACE`、`POP` 三种）、`POP` 到带 `opened` 的为真；`PUSH`、`REPLACE` 到带 `opened` 的与没有状态的为假（审查 A 的 P2A1-N2） |
| 窄屏的查询 | `W/app/narrow.test.ts`（S1，决定 801）：没有 `matchMedia` 时为假；有时照 `matches`、只问 `(width < 48rem)`，`change` 之后跟上，卸下之后不再听（审查 C 的 S1C-N2） |
| 抽屉 | `W/pages/workspace/shell-drawer.test.tsx`（新，S3，`matchMedia` 打桩成窄屏）：(1) 关着时根带 `inert` 属性（jsdom 没有 `inert` 的 IDL 属性，React 只写 HTML 属性：这里看 `hasAttribute("inert")`，不像 UI1 看 JS 属性；审查 C 的 P2C1-N5）、不是 `dialog`、没有 `tabindex`；点"Navigation"，根是名为"Navigation"的 `dialog`、切换器有焦点；以程序把焦点放到 `main` 的 `h1` 上，焦点回到抽屉里（审查 C-I2）；Esc 关上、"Navigation"有焦点；简体中文时名为"导航"（审查 C-N1）；(2) 载入页面之后焦点在 `body`（`FocusScope` 不自动聚焦）；(3) 关着时 Mod+K 打开它、框有焦点；开着时 Mod+K、Mod+Shift+F 聚焦框，Mod+O 不开快速切换；在抽屉里点"Home"（抽屉关上、"Navigation"有焦点）之后，Mod+K 仍打开抽屉、框有焦点（审查 A 的 P2A1-M2）；(4) 在抽屉里点树的一页：抽屉关上、`h1` 有焦点；点"Home"：抽屉关上、"Navigation"有焦点；在抽屉里提交搜索：结果页的 `h1` 有焦点；在抽屉里点当前页的那一行：抽屉关上、"Navigation"有焦点（探针 T8）；LLM Wiki 的笔记本里开抽屉、点"Index"：抽屉关上、目录页的 `h1` 有焦点（审查 A 的 P2A2-M1）；抽屉开着时后退到树打开过的一页（`opened`）：`h1` 有焦点（探针 T9；带 `opened` 的那一条这样造：`matchMedia` 先答宽屏、点树里的"Notes"（宽屏上树的链接带 `opened`），再答窄屏、开抽屉点"Roadmap"，再开抽屉（切换器有焦点），然后后退：不再开的话抽屉已随点"Roadmap"关上，后退时 `useDrawer` 什么也不做，变异 54 抓不到；审查 C 的 P2C1-N5、P2C2-N1）；(5) 抽屉里打开账户菜单再按 Esc：菜单关上，抽屉仍开着；组字中的 Esc（`isComposing`）：抽屉仍开着；框里有字时按 Esc：框上有一个替浏览器在按键分发完之后清空框的监听（`keydown` 里 `queueMicrotask(() => (field.value = ""))`，浏览器的清空也是分发之后的默认动作）时抽屉仍开着、再按才关，没有它时（jsdom 不清空）下一个任务里抽屉关上（决定 797）；(6) `matchMedia` 变成宽屏：根不再 `inert`、不是 `dialog`、没有 `tabindex`，点侧栏的空白处焦点仍在 `body`（审查 A-M1）；`matchMedia` 再答窄屏：根带 `inert`、不是 `dialog`，焦点没有动（审查 C 的 P2C3-N2）；(7) 编辑中有未保存的改动（照 `page-edit.test.tsx` 的未保存的格），在抽屉里点树的另一页：确认框出现；选"Stay"：抽屉关上（根带 `inert`、不是 `dialog`），焦点在编辑器里，地址不变（决定 799；审查 A 的 P2A1-M1）。写这个文件时留意：jsdom 没有 `inert`，抽屉里切换器的下拉选了一个工作区之后，Radix 在 `setTimeout(0)` 里把焦点还给切换器，会抢走菜单按钮的焦点（浏览器里切换器那时已 `inert`）；"在抽屉里切换工作区"只在 UI1 第一格里核对（审查 A、C 的未核实） |
| 快速切换的提示 | `quick-switch.test.tsx` 加格（S3）：键盘提示在 `aria-hidden` 里；简体中文时是"↑↓ 选择 · ↵ 打开 · Esc 关闭"（审查 C-N1）；选项照旧两个 `span` |
| 外观与布局 | 不在 jsdom 里测：7.4 的截图与第 0 步的探查 |
| 抽屉、搜索、树在真浏览器里 | UI1、PG16（三种）；se1、se2、lw8（Chromium）；每个改了行为的提交跑完整的 `make e2e`（决定 795） |

### 7.2 变异

每条在本机改坏、跑它旁边的测试、看到失败，再从备份复原（02-workflow 3.1；先在没改的树上跑一遍同样的命令）。

| # | Step | 改坏 | 抓住它的测试 |
|---|---|---|---|
| 1 | S1 | `Layout` 不看 `handle`，右上角总在 | `layout.test.tsx`"只有一个 Ada"；`user-menu.test.tsx` 的 `findByRole`（两个） |
| 2 | S1 | 工作区读取中也交还右上角 | `layout.test.tsx`"读取中没有 Ada"（扣住 `GET workspaces`，在它发出之后断言） |
| 3 | S1 | `NotFoundPage` 时不交还 | `layout.test.tsx` 的 404 格；a6（`openSignedIn` 等不到账户按钮） |
| 4 | S1 | `RouteError` 不交还 | `route-error.test.tsx` 新格 |
| 5 | S1 | 未登录时右上角不放"外观" | `layout.test.tsx` 未登录格；`route-error.test.tsx:25` |
| 6 | S1 | 外观的选项不 `preventDefault` | `user-menu.test.tsx` 的"仍开着"；`routes.test.tsx:28`；`layout.test.tsx:8-19` |
| 7 | S1 | "个人设置"不带 `arrived` | `profile-page.test.tsx:47-56` |
| 8 | S1 | 设置的 `h1` 不取到达的焦点 | 同上 |
| 9 | S1 | 头像的首字不 `aria-hidden` | `user-menu.test.tsx` 的按钮名；a6、a8、s2、c6（`accountMenu` 精确匹配） |
| 10 | S1 | 切换器放回 `nav` 里 | `workspace-layout.test.tsx:84-98` |
| 11 | S1 | `shape: pill` 写成 `rounded-[999px]`（tailwind-merge 照样去掉 `rounded-lg`，画出来也一样；审查 C-N4） | `button.test.tsx` 的胶囊格（只靠它断言了 `rounded-full`） |
| 12 | S1 | 子菜单内容加 `data-[state=closed]:animate-fade-in` | `dropdown-menu.test.tsx` 子菜单格 |
| 13 | S1 | 亮色 `--sidebar` 调深成 `#e9e9eb` | se1 的 `box`、`invalid`（亮） |
| 14 | S1 | 切换器的"新建工作区"不带 `arrived` | `workspace-layout.test.tsx` 新格 |
| 15 | S1 | "回到工作区"不带 `arrived` | `layout.test.tsx` 的设置格 |
| 16 | S2 | `notebookGroups` 改回数列表里全部链接 | n1:103（00 号文档 4.8 的那条） |
| 17 | S2 | `NotebookLayout` 直接门户进列表项（不经稳定的容器） | `notebook-slot.test.tsx` (2a) |
| 18 | S2 | `SlotHolder` 不记、不还焦点 | `notebook-slot.test.tsx` (2a) 的焦点（jsdom 挪动拿着焦点的节点时也把焦点交给 `body`，审查 B 的 `jsdom-move.cjs`） |
| 19 | S2 | 去掉两组之后的备用位置 | `notebook-slot.test.tsx` (3) |
| 20 | S2 | 别的笔记本的链接不带 `arrived` | `notebook-slot.test.tsx` (4) |
| 21 | S2 | 笔记本行的 `h2` 挪进树的 `nav` | `page-tree.test.tsx:17-20` 的标题表（多出笔记本名）；e2e 的 `treeTitles`（pg1:180 等） |
| 22 | S2 | ＋挪出树的 `nav` | `page-tree-writes.test.tsx` 在 `nav` 里找"New page"；pg1:182 |
| 23 | S2 | ＋平时 `invisible` | pg1:182、`newPageWith`（Playwright 只点看得见的） |
| 24 | S2 | `reveal` 改成 `link.scrollIntoView()` | `page-tree-reveal.test.tsx`；PG16 第二格的 375 宽一段（`window.scrollY`；宽屏那几步抓不到，10.2） |
| 25 | S2 | `reveal` 不看看守 | `page-tree-reveal.test.tsx` 在 `[data-sidebar-scroll]` 上发 `wheel` 的格 |
| 26 | S2 | 树的菜单的 `onCloseAutoFocus` 不还焦点 | PG16 第一格（三种） |
| 27 | S2 | "目录""历史"两行不用 `NavLink end` | `page-tree.test.tsx` 的 LLM Wiki 格（`…/history/:id` 上"History"不带 `aria-current`） |
| 28 | S2 | `threeBrowsers` 里写成 `page\/pg16_` | `check.mjs`（`make lint-web`）：`REQUIRED` 的 PG16 两行不在列表里 |
| 29 | S3 | Mod+K 不看焦点在不在编辑器里 | `search-box.test.tsx` 的 `.cm-editor` 格（00 号文档 4.8） |
| 30 | S3 | Mod+K 在对话框开着时也聚焦 | `search-box.test.tsx` 的对话框格（00 号文档 4.8） |
| 31 | S3 | Mod+K 不管组合中 | 组合中格 |
| 32 | S3 | Mod+K 不看 `defaultPrevented` | `defaultPrevented` 格 |
| 33 | S3 | `aria-keyshortcuts` 只列 Mod+Shift+F | `search-box.test.tsx:44,221` |
| 34 | S3 | 抽屉关着时不带 `inert` | UI1 第一格（00 号文档 4.8）；`shell-drawer.test.tsx` (1) |
| 35 | S3 | `FocusScope` 不 `trapped`（`loop` 留着，Tab 照样回绕） | UI1 第一格的"以程序把焦点放到抽屉外"（三种，探针 T11）；`shell-drawer.test.tsx` (1) 的同一步 |
| 36 | S3 | `onMountAutoFocus` 不 `preventDefault` | `shell-drawer.test.tsx` (2)；到达焦点的许多格 |
| 37 | S3 | `trapping` 不看 `drawer.at === location.key`（导航之后还困着，初稿的写法） | UI1 第二、三格；`shell-drawer.test.tsx` (4) 的树与搜索 |
| 38 | S3 | 抽屉里树的链接照旧 `opened` | UI1 第三格；`shell-drawer.test.tsx` (4) |
| 39 | S3 | 导航关上时总把焦点给菜单按钮（不看 `arrived`） | UI1 第二、三格；(4)（`WorkspaceLayout` 的 effect 在 `h1` 的之后，菜单按钮赢：探针 T12、`zz-focus-790` 第 4 格） |
| 40 | S3 | 根上的 Esc 改用 React 的 `onKeyDown`（门户里的菜单的 Esc 冒泡上来） | `shell-drawer.test.tsx` (5) |
| 41 | S3 | `dialogOpen()` 不认抽屉 | `shell-drawer.test.tsx` (3) 的 Mod+O；`shortcuts.test.ts` |
| 42 | S3 | `besideDrawer` 不排除抽屉 | `shell-drawer.test.tsx` (3) 的 Mod+K（UI1 第二格按键时抽屉关着，走不到；审查 A-N2 = C-N16） |
| 43 | S3 | 窄屏关着时 Mod+K 不先开抽屉 | `shell-drawer.test.tsx` (3)；UI1 第二格 |
| 44 | S3 | 快速切换的选项多一个 `span` | `quick-switch.test.tsx:18-22` |
| 45 | S3 | `threeBrowsers` 漏写 `shell\/ui1-` | `check.mjs` 的 UI1 两行 |
| 46 | S1 | `useCornerBack` 的清理不减数 | `layout.test.tsx` 的应用内 404 格（两个"Ada"） |
| 47 | S1 | 右上角按 `account?.me` 判断（不读会话、不自己读 `me`） | `route-error.test.tsx` 的新格（首次载入就出错时只有"Appearance"）；`layout.test.tsx` 的会话启动中格（先出现"Appearance"） |
| 48 | S1 | `SessionUnavailablePage` 不交还右上角 | `layout.test.tsx` 的 `GET /me` 答 500 格 |
| 49 | S2 | `NotebookNav` 把地址里的 id 转小写 | `notebook-slot.test.tsx` (5) |
| 50 | S2 | `SlotHolder` 还焦点之后不带进侧栏的可见范围 | `notebook-slot.test.tsx` (2a) 的 `scrollTop` |
| 51 | S2 | `SlotHolder` 挪了容器之后不通知（不调 `slot.moved()`） | `notebook-slot.test.tsx` (2c)（就地换组，`TreeMenu` 不重画、不重量；决定 800） |
| 52 | S2 | 看守在 `PageTree` 的 effect 里建、行在自己的 layout effect 里调 `reveal`（初稿的写法） | `page-tree-reveal.test.tsx` 的三次以上导航格与"首页、按键、开页"格；PG16 第二格的第三次导航 |
| 53 | S2 | 看守不带 `within`（算整页的输入） | `page-tree-reveal.test.tsx` 在 `main` 上发 `wheel` 的格；`readers-input.test.ts` 的 `within` 格 |
| 54 | S3 | 导航那次提交不 `blur` 抽屉里的焦点 | `shell-drawer.test.tsx` (4) 的后退格（jsdom 没有 `inert`，焦点留在抽屉里，标题不从无人处取；探针 `jsdom-rev.mjs` 的 T9） |
| 55 | S3 | 抽屉里点当前的地址也带 `arrived` | `shell-drawer.test.tsx` (4) 的当前页格（焦点到 `h1` 或 `body`，看原来那一条的路由状态，不是"Navigation"；审查 C 的 P2C1-N4） |
| 56 | S3 | Esc 不让组字 | `shell-drawer.test.tsx` (5) 的组字格 |
| 57 | S3 | Esc 不让有字的搜索框 | `shell-drawer.test.tsx` (5) 的替浏览器清空格；UI1 第一格（Chromium、Firefox） |
| 58 | S3 | `FocusScope` 的根一直 `tabIndex=-1` | `shell-drawer.test.tsx` (1)、(6) 的 `tabindex` 与点空白处 |
| 59 | S3 | 抽屉的滑动只把属性放进 `motion-safe:`（`duration-150` 留在外面） | UI1 第一格的减少动效一步（`transitionDuration` 是 `150ms`，不是 `0s`） |
| 60 | S3 | 没有框时快捷键照样 `preventDefault` | `search-box.test.tsx` 的工作区读取失败格 |
| 61 | S3 | 搜索按钮去掉垫底，直接放在相对定位的 `div` 里 | se1 的 `button`（亮：读数回到 2.84，低于新下限 2.99） |
| 62 | S2 | `PageTree` 不比路径，每个 `location.key` 都定（同一页换 hash 也滚） | `page-tree-reveal.test.tsx` 的同一页换 hash 格 |
| 63 | S3 | `UnsavedGuard` 的"留下"不关抽屉 | `shell-drawer.test.tsx` (7)（抽屉仍是 `dialog`） |
| 64 | S3 | `actions` 的函数不经 `now`，读建它时（第一次渲染）的 `trapping` 与 `location.key`（`useSearchShortcuts` 的依赖照旧不变，监听只注册一次） | `shell-drawer.test.tsx` (3) 的"Home 之后 Mod+K"（抽屉不开） |
| 65 | S1 | 在个人设置页上再选"个人设置"也设 `leaving` | `profile-page.test.tsx:47-56` 的第二步（焦点落到 `body`，不是"Ada"） |
| 66 | S3 | `headingTakes` 不算 `Located` | `arrival.test.tsx` 的 `headingTakes` 格 |
| 67 | S3 | "目录""历史"两行在渲染时定 `state`（`state={context.inDrawer() && !here ? arrived : opened}`，不在点按时） | `shell-drawer.test.tsx` (4) 的 LLM Wiki 格（焦点到"Navigation"，不是 `h1`） |
| 68 | S2 | `PageTree` 卸下时不结束看守 | `page-tree-reveal.test.tsx` 的"树没读到时离开"格（`removeEventListener` 没被调） |
| 69 | S3 | 变宽时不清掉 `drawer` | `shell-drawer.test.tsx` (6) 的"再答窄屏"（抽屉又开着、是 `dialog`） |
| x1 | S1 | 右上角不看账户读取失败 | `layout.test.tsx` 的 `GET /me` 答 500 格 |
| x2 | S1 | 切换器的首字不 `aria-hidden` | `workspace-layout.test.tsx` 六格 |
| x3 | S1 | 账户行的菜单不向上开 | `user-menu.test.tsx` 的位置格（加这一格之前活下来） |
| x4 | S1 | 账户行挪到侧栏顶部 | `layout.test.tsx`、`workspace-layout.test.tsx` |
| x5 | S1 | 右上角的菜单不靠右 | 同 x3 |
| m801 | S1 | 窄屏仍用子菜单 | `user-menu.test.tsx` 的窄屏两格 |
| x6 | S1 | `useNarrow` 的查询写成 `(width < 40rem)` | `narrow.test.ts`；窄屏两格 |
| x7 | S1 | `useNarrow` 不听 `change` | `narrow.test.ts` |
| x8 | S1 | 外观的 `keepOpen` 什么也不做 | `user-menu.test.tsx` 的"仍开着"与窄屏两格 |
| x9 | S1 | 右上角的菜单不设宽度上限 | `user-menu.test.tsx` 的位置格 |
| n01 | S1 | index 路由不带 `handle` | `layout.test.tsx` 的"`/` 扣住列表"格 |
| n02 | S1 | `LandingPage` 读取失败时不交还 | `layout.test.tsx` 的"`/` 列表答 500"格 |
| n03 | S1 | `leaving` 不看引导步骤 | `profile-page.test.tsx` 的引导页格 |
| n04 | S1 | 新建工作区页不放"回到工作区" | `layout.test.tsx` 的新建工作区页格 |
| n05 | S1 | 新建工作区页总放"回到工作区" | `layout.test.tsx` 的"没有工作区"格 |
| b1 | S2 | `reveal` 只在没有看守时建（一棵树一生一个看守，不按导航换新） | `page-tree-reveal.test.tsx` 的"首页、在滚动区里按键、开页"格与"树读到之前在滚动区里滚轮"格的第二次导航（审查 C 的 S2C-M1，A 的 q24） |
| b2 | S2 | 卸下时不清空看守（去掉 `watching.reader = undefined`） | `page-tree-reveal.test.tsx` 的 StrictMode 格（审查 C 的 S2C-N1，A 的 q05） |

x1–x9 是 S1 的实施者另写的，m801 随决定 801，n01–n05 随审查的修复。审查者另写的与它们的结果在 [S1 的审查记录](reviews/P2-S1-shell-account-review.md)：有意不抓的是 y03、y04、y04b（`SessionChangedError` 的三处排除，主持者的决定）；只关布局与样子、由截图与实量看的是 y08、c03、c04；z3（只去掉 `md:h-svh`）交给 S2 的 PG16 第二格。b1、b2 随 S2 审查的修复。S2 的实施者另写的 s1–s17 与审查者另写的（A 的 q 各条，C 的 a7、b6、b9、r1 与对照）和它们的结果在 [S2 的审查记录](reviews/P2-S2-notebooks-tree-review.md)：s17、q27 等价；z3 由 PG16 第二格在三种浏览器里抓住。

00 号文档 4.8 要求的、属于 P2 的四条是第 13（调深侧栏，se1 变红：它的意思是"改了被量元素的底，守卫还在"）、16、29 与 30、34 条。初稿的第 46 条（收养之后不通知树）随那条路一起去掉（决定 776）。第 51 条原来是"`close` 记成稳定的回调"：`create` 照样每次新建、`TreeMenu` 照样重画，是等价的变异（审查 C 的 P2C1-M1），改为去掉决定 800 的通知。第 64 条有意义：`actions` 一生不变，`useSearchShortcuts` 的依赖 `[mac, field, actions]` 也就不变，监听只注册一次，`actions` 的函数若读建它时的值就是过时的，"Home 之后 Mod+K"抽屉不开；把 `actions` 改成随这些值新建（`useMemo` 带上它们）不是这条变异，是另一种对的写法（监听随之重新注册），零警告的 lint 也过（审查 C 的 P2C2-M1）。笔记本行的 `[@media(hover:hover)]:`、读者的 `pr-8` 与树里提示的 `my-1`（审查 B 的 P2B1-N3、N5、N4）是样子，jsdom 里量不出，由 7.4 的截图看，不列。右上角"个人设置"的 `chosen`（决定 770）是预防：探针 T7 三种引擎各 15 次，做不做都到 `h1`，没有能抓它的格，不列。容器各 id 都留（决定 798）防的是并发渲染被打断，jsdom 里造不出来，也不列。

### 7.3 门禁

- **基线**（决定 795(a)）：S1 开工之前在没改的 `m13-ui-refresh` 头上 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make e2e`，失败的格记下文件:行（错误所在的那一行）与消息的第一行（决定 795(a)）。
- **每个提交**：`docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web`（`lint-web` 是四个 `check.mjs`、零警告的 oxlint（豁免只有 4.4 列的那些）、oxfmt、各包的 tsc；经 `heavy.sh`：06-environment 第 3 节，`test-web` 有计时的断言，决定 788）；改了行为、夹具或故事的另经它跑完整的 `CI=1 make e2e`（决定 795(b)）。
- **合并之前**（决定 788、795(c)）：在 Step 分支并入 `m13-ui-refresh` 的树上跑 `CI=1 docs/v0.1/claude_to_codex/tools/heavy.sh make lint-web knip test-web gen-check-web build-web e2e`（三种浏览器与 `webkit-narrow`；整条命令经 `heavy.sh`：`test-web` 里 `markdown.test.ts` 的"not slower"对负载敏感）；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 以 0 退出；结果写进 Step 的记录。本机允许的失败只有基线里的那几格，且失败所在的行与消息的第一行要与基线相同（第 1 节的三格：SE2 `se2:231` [chromium]、LW8 `lw8:836` [webkit] 是 P1 之前就在的环境问题，PG15 `pg15:113` [webkit] 是 P1 里出现的，决定 764；审查 C-N15；行号是 S1 的基线记下的失败所在的行，消息的第一行在第 1 节）；AS6 `as6:153`、LW8 `lw8:771` 与负载过 100 时 `markdown.test.ts` 的"not slower"偶发，重跑全过即可，记下。别的红了照 02-workflow 3.1 先停、查清。
- **只靠 CI 的**（决定 795(d)）：se2 的 page 格在 se2:230 之后的步骤（含 :274 的访客说明）与 UI1 的 `Control+K` 只在 Linux 的 CI 上走；S3 的记录写明，负责人推送之后看 CI。
- **没有重现的 CI 项**：照 [01-P1](01-P1-foundation.md) 7.3 的表（Go 一侧、`lint-actions`、部署与镜像、Linux 与 macOS 的差别）；负责人推送之后 CI 在最后一个头上补跑。
- **文档**：`python3 docs/v0.1/claude_to_codex/tools/check_links.py origin/m13-ui-refresh`（远端的分支不在时用 `m13-ui-refresh`）、`git diff --check`。

### 7.4 视觉的核对

B 审查者（00 号文档 4.8）用 Playwright 截取下面这些，亮暗、1440 与 375 各一张，对照 palette.html 看，记录写明看了哪些：

- **S1**：工作区首页（侧栏、切换器、导航行、笔记本列表、账户行）；账户菜单与外观子菜单开着；登录页（右上角的"外观"）；个人设置页（"回到工作区"与账户按钮）；工作区的 404；一页长文滚到中间（侧栏粘住）。有意的偏离：账户行上方的 `border-t`、头像一个字素（决定 770）。
- **S2**：一本笔记本的一页（笔记本行、悬停行与悬停＋时的齿轮与＋（悬停＋时行的底与字同悬停行）、读者的笔记本行（`pr-8`）、树里失败的提示（上下留空）、当前页的行、"更多"一排：四个图标都是 16 px，审查 B-I2），一本 LLM Wiki（"目录""历史"两行与"更多"里同样的图标、同一种当前的样子，决定 796），树的行菜单，一棵 60 页的树滚到底；⌄、书本、名字的横坐标（14 / 33 / 58）。
- **S3**：搜索框（静止、聚焦、无效、有字时的清除按钮与提示；按钮的垫底看不出来，亮暗、框聚焦与不聚焦各一张）；快速切换；375 宽下抽屉关着、开着（遮罩）与细栏；Chromium 强制色模式下开着的抽屉一张（控件与焦点看得见）；减少动效时抽屉不滑动。

### 7.5 CI 的余量

新故事两个：PG16 两格、UI1 三格，各在三种浏览器里，共 15 次，每次推断 4–6 秒（一个团队、一本笔记本）；PG16 的第二格按位置依次建 60 页、走三次导航，比别的格多几秒，S2 之后照实记一次（审查 C-N8；S2 的实测，审查 C 在 `831e50a6` 上合并之前的整条命令：第一格 0.65 / 1.9 / 0.89 s，第二格连同 375 宽一段 1.8 / 2.3 / 2.2 s（Chromium / Firefox / WebKit），六次合计约 9.7 test-seconds，比推断的少；负载高时第二格到 3.5 s，S2 的审查记录）；合计约 75–90 test-seconds。`e2e/playwright.config.ts` 没设 `workers`，Playwright 默认用一半的 CPU 核：GitHub 的 `ubuntu-latest` 是 4 vCPU，约 2 个 worker，约合 40–45 秒的墙钟（审查 C-N9）；本机 E2E 2.4–2.8 min；CI 的 E2E 一步今天约 6 分 21 秒，时限 18 分钟（00 号文档第 9 节）。其余故事只换定位与断言，次数不变。S3 之后照实记一次时间。

**本机的总时长**（粗算，只在本机、经 `heavy.sh` 串行，不进 CI；审查 C 的未核实）：基线一次完整的 `make e2e` 约 5 分钟；九个提交各一次 `make lint-web knip test-web`（P1/S2 的记录约 70 秒）约 11 分钟；六个改行为的提交各一次 `make e2e` 约 30 分钟；三次合并之前的整条命令（网页一侧约 75 秒加 `make e2e`）约 19 分钟，审查 C 各再跑一次，约 19 分钟；变异约 50 条只跑 vitest（一个文件 15–40 秒）约 25 分钟，约 15 条要重建前端再跑一个故事（每条约 3 分钟）约 45 分钟；修复核对每个 Step 一两轮，每轮一次网页一侧加 `make e2e`，约 40 分钟。合计约 3 小时的锁时间；别的 agent 同时排队时墙钟更长。

## 8. 完成标准

### 8.1 条件

1. 4.1–4.3 的每一项做到；7.1 的测试通过；7.2 的变异都被抓住。
2. 每个 Step 的合并之前，7.3 的门禁为绿（只败基线里同行同消息的那几格，决定 795）；没有服务端与契约的改动。
3. se1 的下限照第 5 节重量，实量值与下限写进 S1 与 S3 的记录（S3 起"button"量的是垫底，决定 794）。
4. 第 0 步的探查成立，写进 S1 的记录（S1：三条在三种引擎里都成立；(b) 的"侧栏滚到底、窗口不动"后由决定 802 取代，中段滚到头带着窗口走）。
5. 7.4 的截图看过，与样稿不同之处写进记录（是有意的，写理由；不是的，修）。
6. Phase 完成（02-workflow 3.7）：00 号文档第 12 节 P2 一行"完成"、头的状态、第 13 节一行；本文头的状态；M12-polish 的"处理进展"两行（S2；S3，含第 8 项的 `onMac` 与留给 v0.2 的 26.8、26.10）。S2 的一行在 S2 的文档提交里写了。

### 8.2 总体设计的新文字

每条由 Step 的文档提交里的脚本替换，断言旧文字恰好出现一次（同一段后来的 Step 还要改，所以各条的锚点写的是那时的文字：第 1、2、4、5、6、8、9 条与第 10 条 S2 部分的锚点在 `4c3f10b9` 上各出现一次，设计者已核对；第 3 条、第 7 条与第 10 条 S3 部分的锚点是前一个 Step 写进去的文字，各出现一次）。S1 改第 1、2 条与第 12 条的 S1 一行；S2 改第 3 条的 S2 部分、第 4、5、6 条、第 10 条的 S2 部分与第 12 条的 S2 一行；S3 改第 3 条的 S3 部分、第 7、8、9 条、第 10 条的 S3 部分与第 12 条的 S3 一行。

1. **9.2 左栏一段**（S1），把整段"- **左栏**：工作区切换、笔记本列表（"我的笔记本"与"团队笔记本"两组）、当前笔记本的页面树（支持拖拽移动与排序）；页面树之外一个单独命名的导航里有这本笔记本的"历史"与"回收站"（M8），"健康"与"目录"（健康度面板与目录视图，两种模式都有，M10）。左栏最上面是工作区的搜索框（M8）：在一本笔记本里时默认只搜这一本，框下写明在哪里找。"换成：

   > - **左栏**（M13 起没有顶栏）：最上面是工作区切换（方形头像与名称），其下是工作区的搜索框（M8）：在一本笔记本里时默认只搜这一本，框下写明在哪里找；再下是工作区的导航：首页、设置、笔记本列表（"我的笔记本"与"团队笔记本"两组）、新建笔记本，当前笔记本的页面树（支持拖拽移动与排序）在列表之后；页面树之外一个单独命名的导航里有这本笔记本的"历史"与"回收站"（M8），"健康"与"目录"（健康度面板与目录视图，两种模式都有，M10）。最下面是账户行（圆形头像与显示名），菜单里是个人设置、外观（主题与语言，就地切换：宽屏是子菜单，窄屏直接列在菜单里）、退出登录与版本。宽屏上左栏底色深一档（`--sidebar`），粘在窗口的左边、中段自己滚动，窗口照旧整页滚动；没有左栏的时候（登录、注册、引导、新建工作区、个人设置、出错页、工作区读取失败，以及工作区之外与找不到工作区的 404；笔记本里的 404 有左栏）右上角放同一个账户按钮，未登录或会话不可用时放"外观"（主题与语言），会话或账户还在读时什么也不放（M13/P2/S1）。

2. **13.2 第 17 条的 S1 部分**，"链接随它所在的页一起离开的（子页列表、面包屑、工作区首页的笔记本卡片、右栏与阅读视图里的链接）带 `arrived`，否则焦点落到 `body`（M12/P2/S1）。"之后加：

   > 左栏里通往工作区外框之外的菜单项（账户菜单的"个人设置"、工作区切换的"新建工作区"）与个人设置页、新建工作区页（账户已有工作区时）右上角的"回到工作区"同样带 `arrived`，个人设置页的 `h1` 取到达的焦点（M13/P2/S1）。

3. **9.2 左栏一段的 S2、S3 部分**：
   - S2，把第 1 条写的"新建笔记本，当前笔记本的页面树（支持拖拽移动与排序）在列表之后；页面树之外一个单独命名的导航里有这本笔记本的"历史"与"回收站"（M8），"健康"与"目录"（健康度面板与目录视图，两种模式都有，M10）。"换成"新建笔记本。当前笔记本在列表里就地展开：笔记本那一行（树的标题，行尾是笔记本设置与新建页面），其下是页面树（支持拖拽移动与排序），再下一排图标链接，是一个单独命名的导航：这本笔记本的"目录"与"健康"（目录视图与健康度面板，两种模式都有，M10），"历史"与"回收站"（M8）（M13/P2/S2）。"
   - S3，把第 1 条写的"其下是工作区的搜索框（M8）：在一本笔记本里时默认只搜这一本，框下写明在哪里找；"换成"其下是工作区的胶囊形搜索框（M8；M13/P2/S3）：在一本笔记本里时默认只搜这一本，在哪里找是框的描述（只给读屏）；"，并在第 1 条写的"会话或账户还在读时什么也不放（M13/P2/S1）。"之后加"窄屏（< 768 px）上左栏是抽屉：页面顶部一条随页滚走的细栏上的菜单按钮打开它，关着时 `inert`，开着时困住焦点，Esc 或点遮罩关上（M13/P2/S3）。"

4. **13.2 第 16 条**（S2），"外壳里在 `Outlet` 之外、属于这个工作区的部分（左栏的 `NotebookNav` 与它的新建对话框）同样按工作区 id 重新挂载。"之后加：

   > 笔记本的外壳把笔记本行、树与"更多"画进工作区外框按笔记本 id 给的一个稳定的容器（`pages/workspace/notebook-slot.tsx`），列表里当前那一项（按地址里的笔记本）收养它，不在列表里时（编辑中留住的）放在两组之后：笔记本在"我的""团队"两组之间挪动时树不重新挂载，开着的对话框与画到的行都在，拿着的焦点挪完之后还回去（M13/P2/S2）。

5. **13.2 第 17 条的 S2 部分**，"左栏的树打开的页带 `opened`（连同"目录""历史"两行）：点开时焦点留在树上，后退、前进回到它时标题只从无人处取（M12/P2/S1）。"之后加：

   > 树的标题是笔记本那一行（`h2`，`tabIndex={-1}`，里面是去笔记本首页的链接），移动、删除之后焦点照旧回到它；列表里别的笔记本的链接带 `arrived`：点开之后那一项换成展开的笔记本行，链接随之离开（M13/P2/S2）。

6. **13.2 第 26 条**（S2），"接管成功或答 `notebook.not_found` 之后标题才取得焦点（`ownerless-page.tsx`，同回收站的恢复）。"之后加：

   > 左栏把当前页的行带进可见范围（M13/P2/S2，`pages/notebook/page-tree.tsx` 的 `reveal`）：只改左栏滚动区（`[data-sidebar-scroll]`）的 `scrollTop`，不调 `scrollIntoView`（它也滚窗口）；每次导航至多一次，读者自这次导航起在左栏的滚动区里滚过、按过键或按下过就不滚；同一页换 hash 或查询参数、`replace` 到当前的地址在这里算同一次导航（与上面到达的看守不同：那里同一路径换了查询或片段的 `replace` 是新的导航）。

7. **13.2 第 17 条的 S3 部分**，在第 5 条加的"链接随之离开（M13/P2/S2）。"之后加：

   > 窄屏的抽屉里，树打开别的页的链接（连同"目录""历史"两行）带 `arrived` 而不是 `opened`，当前的地址照旧 `opened`：抽屉随地址的变化关上、带上 `inert`，焦点在这次提交里离开抽屉；去向是 `arrived`、带附件的定位（`Located`），或后退、前进到带 `opened` 的一条时，焦点由去向页的标题或附件行取（`app/arrival.ts` 的 `headingTakes`），别的导航与 Esc、点遮罩关上时焦点回到菜单按钮；被"未保存"拦下、选了"留下"时抽屉关上，焦点留在确认框还给的地方：编辑器、横幅或冲突的标题（M13/P2/S3，`app/shell-sidebar.tsx` 的 `useDrawer`）。

8. **13.2 第 20 条**（S3）：
   - "现有的：`Mod+O` 快速切换，`Mod+E` 进出编辑，`Mod+S` 保存（9.3），`Mod+Shift+F` 到搜索框（M8）。"改为"现有的：`Mod+O` 快速切换，`Mod+E` 进出编辑，`Mod+S` 保存（9.3），`Mod+Shift+F` 与 `Mod+K` 到搜索框（M8；`Mod+K` 是 M13/P2/S3）。"；
   - "工作区的搜索框在 `window` 上听它，焦点到框并选中文字；"改为"工作区的外框（`WorkspaceLayout`，M13/P2/S3 起；搜索框在窄屏的抽屉里）在 `window` 上听它，焦点到框并选中文字，窄屏时先打开抽屉；"；
   - "它在微软拼音、搜狗、fcitx 里是简繁切换，看得见的搜索框是主要入口（输入法清单第 21 步）。"改为"它在微软拼音、搜狗、fcitx 里是简繁切换，看得见的搜索框是主要入口（输入法清单第 21 步；窄屏上框在关着的抽屉里，细栏的菜单按钮打开它，M13/P2/S3）。"（审查 C-N11）；
   - "搜索框里组合中的回车（同样的判断）`preventDefault`，不提交。"之后加"`Mod+K`（M13/P2/S3，`pages/workspace/search-shortcuts.ts`）经 `isMod`，组合中、按住的重复与对话框开着时同 `Mod+Shift+F`；`event.defaultPrevented` 或焦点在编辑器（`.cm-editor`）里时完全不管：编辑器的 `Mod+K` 是插入链接（9.3），它先拿到按键。`dialogOpen()` 认得窄屏开着的抽屉（`[data-shell]` 的 `role=dialog`）：别的快捷键让路，这两个照旧聚焦抽屉里的框（`dialogOpen({ besideDrawer: true })`）。`aria-keyshortcuts` 列两个键，框里看得见 `⌘K`（`Ctrl K`）的提示。"

9. **9.2 与 9.3 的快捷键**（S3）：9.2 的"`Ctrl+Shift+F`（macOS 上 `Cmd+Shift+F`）到左栏的搜索框（M8）；"改为"`Ctrl+K` 与 `Ctrl+Shift+F`（macOS 上 `Cmd+K`、`Cmd+Shift+F`）到左栏的搜索框（M8；`Ctrl+K` 是 M13/P2/S3，焦点在编辑器里时让给编辑器的插入链接）；"；9.3 的"`Ctrl+K` 插入链接，"改为"`Ctrl+K` 插入链接（编辑器之外它聚焦搜索框，9.2；M13/P2/S3），"。

10. **13.4 第 3 条**：
    - S2，"se1 另断言控件的环是 `ring 2px`。"之后加" M13/P2/S2：`notebookGroups` 每一项只取它的第一个链接（当前笔记本的那一项里还有树与"更多"）；PG16 在三种浏览器里核对树的行菜单（打开、Esc、改名、以 Esc 取消改名（对话框没有取消按钮）、焦点回到按钮）与当前行滚进左栏而窗口不动（宽屏与 375 px 下都看窗口），只有页面版本：它只核对界面的行为、不写，接口与 MCP 版本豁免（PG2 照旧调 `expectRenamed`）。"
    - S3，在 S2 加的"（PG2 照旧调 `expectRenamed`）。"之后加" M13/P2/S3：新分组 `stories/shell/`；UI1 在三种浏览器里核对 375 px 的抽屉（关着时 `inert`、打开与困住焦点、Esc 与遮罩、在里面搜索与开页之后焦点的去处），只有页面版本：只有界面的行为，接口与 MCP 版本豁免；375 px 下要碰左栏的故事先开抽屉（`fixtures/shell-pages.ts` 的 `openDrawer`），今天的 `@narrow` 各格与手写的 375 px 检查都不碰左栏。"

11. **写到"左栏"的其他条目**（00 号文档"总体设计的修订"第 6 项最后一小条；13.2 里没有写到"顶栏"的；第 20 条"看得见的搜索框"一句在第 8 条）：第 15 条（同一份列表在左栏、首页等处显示）、第 21 条（"用户菜单"的退出登录：账户行与右上角是同一个 `UserMenu`，"留下"照旧还焦点给它）、第 30 条（左栏的树列出全部）、第 31 条（375 px 的检查；se1 在侧栏上的重量照它已写的"之后改了被量元素的底或颜色的 Step 照同一规则重量"）不改；第 16 条的"左栏常驻"与 9.2 的设置模式由 P3/S1 改。第 1 节的范围表（"左栏只列页面"）与第 12 节的里程碑表是 M2–M4 的记录，不改。

12. **第 15 节**变更记录，每个 Step 在"## 15. 变更记录"之后的空行后加一行（照邻行的写法，新的在上，日期照合并的那天）：
    - S1：`- 2026-11-2x：M13/P2/S1（依据 [P2 设计](M13-ui-refresh/02-P2-shell.md)）：9.2 的左栏一段改写（没有顶栏、账户行、右上角、粘住的左栏）；13.2 第 17 条加离开工作区外框的菜单项带 `arrived`。`
    - S2：`- 2026-11-2x：M13/P2/S2（依据 [P2 设计](M13-ui-refresh/02-P2-shell.md)）：9.2 的左栏一段（当前笔记本就地展开、树下的图标链接）；13.2 第 16 条（树的稳定容器）、第 17 条（树的标题即笔记本行、别的笔记本的链接带 `arrived`）、第 26 条（左栏只滚自己）；13.4 第 3 条（`notebookGroups`、PG16）。`
    - S3：`- 2026-11-2x：M13/P2/S3（依据 [P2 设计](M13-ui-refresh/02-P2-shell.md)）：9.2 的胶囊搜索框、⌘K 与窄屏抽屉；9.3 的 `Ctrl+K`；13.2 第 17 条（抽屉里的导航）、第 20 条（`Mod+K` 与它的让路、`dialogOpen` 认得抽屉）；13.4 第 3 条（`shell/`、UI1）。`

## 9. 交给后面的 Phase

- **P3**：设置模式用 `ShellSidebar`（三段、抽屉、细栏、账户行都现成：中段换成设置目录，顶部换成"返回"与标题）；`/settings` 路由加 `handle: { sidebar: true }`，个人设置页右上角的"回到工作区"随之去掉（`W/app/corner.tsx` 的 `back` 里 `settings` 那一项，以及 `layout.test.tsx` 的设置那一格）；新建工作区页的"回到工作区"、`shell.backToWorkspace` 与 `layout.test.tsx` 新建工作区页的两格留着（决定 803）；侧栏的"设置"与笔记本行的齿轮带 `arrived`（P2 里它们不离开外框，决定 778）；三个设置的 `index` 重定向转交路由状态；设置模式里没有搜索框，`useSearchShortcuts` 不挂（00 号文档 4.2.4"设置模式里……两个快捷键都不做事"）；13.2 第 16 条的"左栏常驻"在设置模式下不成立、树随之卸下（`SlotHolder` 卸下，容器随 `NotebookLayout` 留着或卸下，回来时重画）；`NavItem` 在 P2/S1 已换样子。
- **P4**：页面的头（面包屑、操作）在 `main` 的顶上，P2 的 `main` 照旧 `p-6`，宽屏上的上下留白由 P4 定；窄屏细栏上的位置文字可以换成页的标题（P2 写笔记本或工作区名）；00 号文档第 10 节的滚动锚定一行由 2.1 第 11 条改正（Playwright 的 WebKit 26 也锚定），P4/S1 在它自己的页上于三种引擎里再核对一次。
- **P5**：产品名放到登录、注册、引导的卡片上（决定 787）；笔记本首页的"笔记本设置"链接去掉之后，`page-tree-scale.test.tsx:243` 改找笔记本行的齿轮（名为"X 的设置"）；工作区首页的笔记本卡片用 `NotebookGroups` 的 `groupClassName`。
- **P2–P5**：01-P1 第 9 节的手写"标签加控件"组，P2 只改了树的移动对话框的两处（`move-page-dialog.tsx:159,172`），其余照旧各在所在页的 Phase 改。
- **收尾**：读屏清单"H. M13"一节加：账户行与外观子菜单、右上角、窄屏抽屉的焦点与 Esc、嵌在工作区 `nav` 的列表项里的树的 `nav`（笔记本行的 `h2` 在它之前）。

## 10. 结果

### 10.1 S1：外框与账户行（2026-11-23，合并 `fdad6b9e`）

- **提交**：`0611c586`（右上角、账户行与头像菜单，去掉顶栏，照 6.1 提交 1）、`0986356e`（侧栏三段、`--sidebar`、宽屏粘住、切换器在顶部，照 6.1 提交 2）、`a9d7c030`（窄屏的账户菜单把外观直接列在菜单里，决定 801）；审查的修复 `50e67f1c`（`/` 上账户按钮等侧栏，中段不 `overscroll-contain`（决定 802），子菜单离窗口边 8 px，引导页上选"个人设置"不挡焦点，新建工作区页的"回到工作区"（决定 803），五处外观）、`1d8414a9`（审查发现的测试缺口：会话不可用的右上角、粘住侧栏的守卫、子菜单里的真指针等）；修复核对第 1 轮的 `63810dff`（主持者）；加本文档提交。[S1 的审查记录](reviews/P2-S1-shell-account-review.md)。
- **基线**（决定 795(a)）：开工之前在没改的 `58c5fa92` 上经 `heavy.sh` 跑 `CI=1 make e2e`：594 过、3 败、1 跳，Playwright 3.1 min；三格失败所在的行与消息的第一行在审查记录，第 1 节与 7.3 照它改写（se2:231、pg15:113、lw8:836）。
- **做完**：4.1.2（`W/app/corner.tsx`，`GiveCornerBack` 也在那里；`:slug` 与 `/` 的 `handle`；`RouteError`、`SessionUnavailablePage`、工作区外框的读取失败与 404、落地页的读取失败交还右上角；`useSession` 挪进 `W/app/session-state.ts`）；4.1.3（`UserMenu({ place })`、`AppearanceItems`、`AppearanceMenu`、`initialOf`；"个人设置"与切换器的"新建工作区"带 `arrived`，个人设置页的 `h1` 取到达的焦点；新建工作区页也有"回到工作区"，决定 803）；窄屏的账户菜单与 `W/app/narrow.ts`（决定 801）；4.1.4–4.1.7；`ThemeMenu`、`LanguageMenu` 删掉；a8 改从头像菜单看主题、用指针在子菜单里选一次；se1 在侧栏上的三项下限重量；L1 加粘住侧栏的守卫。
- **测试**：7.1 的 S1 各格与审查之后的新行；vitest 5120 → 5157 格（211 个文件）。新格先写、在没改的树上红；不是先红后绿的五格由变异守着（`layout-error` 一格；`layout.test` 的会话启动中与列表读取失败两格，m47、y34；"`/` 上列表答 500""没有工作区的账户"两格，n02、n05）。`notebook-step.test.tsx` 的一格改为等步骤的文字：测试里潜在的竞态，与引入无关（审查 C 的 S1C1-N1）。
- **第 0 步**（00 号文档第 10 节）：perf 的造数（`flat3k`）起生产配置的应用，三种引擎里各跑一次（探针不提交）：树分批画行时窗口与侧栏的顶不动；侧栏自己滚到底、窗口不动；读者之上长高约 824 px，读者移动不到 1 px。三条都成立；(b) 后由决定 802 取代：中段不再 `overscroll-contain`，滚到头带着窗口走（Chromium、WebKit 在同一次滚动里，Firefox 另起一次时），没有溢出的中段上滚轮三种都滚窗口。
- **实量**（Chromium，macOS；下限 = 实量 − 0.1，向下取到 0.01）：se1 的框与"搜索"按钮的焦点（量到 `--sidebar`）亮 2.843 → 2.74、暗 8.406 → 8.30；无效的框亮 4.018 → 3.91、暗 7.283 → 7.18。范围下拉与命中链接不变。
- **变异**：7.2 的 S1 各条（1–15、46–48、65）都被抓住，3、9、13 另在 e2e 里；实施者的 x1–x9、m801 与修复的 n01–n05 都被抓住；审查者另写的在修复之后都被抓住，除了：y03、y04、y04b（`SessionChangedError` 的三处排除，有意不抓：短暂，守卫接手）与 A 的 r02（同样的后果）；y08（账户行挪进滚动区）、c03（子菜单的 `collisionPadding`）、c04（分隔线的画法）只关布局与样子，由截图与实量看；z3（只去掉 `md:h-svh`）交给 S2 的 PG16 第二格；y06、r25、r29 等价或近乎等价。逐条在审查记录（实施者跑，审查 C 独立重跑）。
- **视觉的核对**（7.4）：实施者截了 7.4 的 S1 几页（Chromium，亮暗 × 1440、375）、决定 801 的 375 宽菜单与修复之后的各处；审查 B 两轮（Chromium 亮暗 × 1440、375 与强制色；Firefox、WebKit 在 1440 下的侧栏、账户行与菜单）；审查 A 用 Playwright 探针在三种引擎里核对行为。截图在会话的临时目录，不进仓库。与样稿不同之处：账户行上方的 `border-t`、头像一个字素（有意，决定 770）；其余（版本行被裁、"回到工作区"折行、窄屏的子菜单、右上角菜单的宽度、子菜单贴底、语言名的左边线、分组标题的高、切换器菜单的宽、强制色下的分隔线）都已修。
- **门禁**：每个提交的 `make lint-web knip test-web` 都绿；改了行为的提交的 `make e2e` 与合并之前的整条命令（`0986356e`、`a9d7c030`、`1d8414a9` 上，审查 C 在后两个上另跑；主持者在加上文档提交的头 `2456d6ed`、即合并的树上再跑一次）只败基线里同行同消息的格（C 在 `1d8414a9` 上那次 PG15 过了）；E2E 2.2–2.5 min（基线 3.1 min，随负载）；Go 一侧的断言与 `git diff --check` 以 0 退出。逐项在审查记录。
- **与设计不同之处**：4.1 的"照实"；决定 801–803。
- **交给后面**：P3 去掉个人设置页的"回到工作区"，留着新建工作区页的（第 9 节）；S2 的 PG16 第二格守 `md:h-svh`（z3）；S3 的抽屉复用 `useNarrow`，抽屉里的账户菜单同样是行内的外观；已登录的邀请页与出错页回去的路留给 P5（决定 803）。

### 10.2 S2：笔记本与页面树（2026-11-23，合并见 00 号文档第 12 节）

- **提交**：`485e6be3`（当前笔记本在列表里就地展开：`notebook-slot.tsx` 的稳定容器与 `SlotHolder`，换组时还焦点、带进侧栏、通知树的菜单重量；别的笔记本的链接带 `arrived`；笔记本行、齿轮、＋与"更多"一排图标；`intoColumn`；`notebookGroups` 每项只取第一个链接；照 6.2 提交 1）、`831e50a6`（树的行照样稿、普通页的文件图标、"目录""历史"两行对齐并标当前、当前行滚进侧栏（`reveal`、`show`、`watchReader` 的 `within`）、移动对话框的间距、PG16 两格进三种浏览器与 `check.mjs`；照 6.2 提交 2）；审查的修复 `3ca04574`（看守按导航换新、跨过 StrictMode 的卸下与重挂，由测试守住；`intoColumn` 挪到 `ShellSidebar` 的注释之前）、`75cac02a`（＋与齿轮的图标与树行的 ⋯ 同列，父页的 › 与笔记本行的 ⌄ 同列（决定 804），强制色下齿轮与＋不透明）；加本文档提交。提交 1 在 e2e 之后本地 amend 过一次（大写 id 一格加断言，s5），提交 2 amend 过一次（PG16 第二格加 375 宽一段，变异 24；提交说明没写这一段，审查 C 的 S2C-N3），amend 之后都重跑了检查。[S2 的审查记录](reviews/P2-S2-notebooks-tree-review.md)。
- **做完**：4.2.2–4.2.5；决定 776–780、789（PG16 两格）、791、796、798、800、804；`NotebookColumn`、`useNotebookColumn` 随唯一的用处去掉；M12-polish 第 14 项（PG16）、M5-polish 37（普通页的文件图标）、38.5（"目录""历史"两行标当前、与页的行对齐）。
- **测试**：7.1 的 S2 各格。新文件 `notebook-slot.test.tsx`（11 格）、`page-tree-reveal.test.tsx`（修复之后 12 格）、`readers-input.test.ts`（5 格）；vitest 5157 → 5169（提交 1，212 个文件）→ 5189（提交 2，214 个文件）→ 5190（修复）。新格先写、在没改的树上红；不是先红的格由变异守着，或只断言不变的事（4.2 的"照实"）。`reading-view.test.tsx` 一格改为按监听函数计（树的看守也在 `window` 上听）。审查之后："首页、按键、开页"一格改在滚动区里按键（S2C-M1：7.1 原来的写法在看守有 `within` 之后什么也不抓）；"树读到之前在滚动区里滚轮"一格加第二次导航，行已画出、在视野外，被滚进来（S2A-N1）；加 StrictMode 一格（S2C-N1 = S2A-N2）。
- **变异**：7.2 的 S2 各条（16–28、49–53、62、68）都被抓住：16、23 在 e2e（n1:103、pg1:182），26 在 PG16 第一格（三种），28 在 `check.mjs`；24 由 vitest 与 PG16 第二格的 375 宽一段抓住，宽屏那几步抓不到（默认与 `nearest` 两种在三种里都过：宽屏时整列粘在视口里，当前行在正文画出之前就带进来，文档那时只有视口高），补了 375 宽一段之后三种都在 :125 抓住（窗口滚到 1830、1239）；52 由三次以上导航格（每次之间点树）、"首页、按键、开页"格与主区里的滚轮格抓住，另在 PG16 第二格（Chromium，第三次导航）；z3（只去掉 `md:h-svh`）由 PG16 第二格在三种里抓住。实施者另写的 s1–s16 都被抓住（s5 在补格之后），s17（`key` 在 layout effect 里设）在 jsdom 里等价。审查另写的：A 的 q17、q35 与 C 的 a7、b6、b9、r1 被抓住，A 的 q27 等价；b1（= A 的 q24，一棵树一生一个看守）与 b2（= A 的 q05，卸下时不清空看守）第一轮活下来，修复之后分别由两格、一格抓住（7.2 新的两行）。逐条在审查记录（实施者跑，审查 C 独立重跑）。
- **视觉的核对**（7.4）：实施者在 Chromium 里截了 7.4 的 S2 各页（亮暗 × 1440、375）；审查 B 在三种引擎里截（另有读者、触屏、强制色）；修复之后实施者照 B 的 spec 在三种引擎里重截，另看了深度 0–2 与强制色下静止时；主持者看了修复之后的两张。截图在会话的临时目录，不进仓库。实量：当前笔记本与别的笔记本的 ⌄、书本、名字都在 14 / 33 / 58 px；父页的 › 在 14–28，与笔记本行的 ⌄ 同列（决定 804），深一层右移 16 px；页的图标与名字在深度 0 / 1 / 2 是 38 / 63、54 / 79、70 / 95（深度 0 在行内是 30 / 55，同样稿），叶子与父页相同，行高 30 px；"目录""历史"两行、来源根与 `AGENTS` 同是 38 / 63；＋的图标在 219–235，与树行的 ⋯、样稿的 `.end` 同列，写者的齿轮在 193–209，读者的齿轮与＋同位；"更多"四个图标 16 px、格 28 px，当前项 `--accent-soft` 底、琥珀色图标，同树的当前行；齿轮与＋平时 0.4，悬停行、悬停它们、375 宽、触屏与强制色下是 1，悬停树的行时仍是 0.4；树里失败的提示上下各 4 px；落点线从 38 起。与样稿的差别修复之后没有了，改过的就是审查 B 的三处（S2B-N1–N3）。
- **门禁**：每个提交的 `make lint-web knip test-web` 都绿；改了行为的提交跑了 `make e2e`（提交 1：594 过、3 败、1 跳，2.2 min；提交 2 amend 之前：600 / 3 / 1，2.3 min；审查 C 在两个提交上另跑）；合并之前的整条命令在 `831e50a6` 上由实施者与审查 C 各跑一次，修复之后在头 `75cac02a` 上又各跑一次：都是 600 过、3 败、1 跳，Playwright 2.3–2.4 min（负载 8–73，随机器上别的运行），只败基线里同行同消息的格（se2:231、pg15:113、lw8:836）；Go 一侧的断言与 `git diff --check` 以 0 退出。逐项在审查记录。
- **与设计不同之处**：4.2 的"照实"；决定 804。
- **交给后面**：S3 的抽屉：PG16 第二格的 375 宽一段到了抽屉里仍成立，但树关在抽屉里时它不再抓变异 24，S3 的提交 2 改在开着的抽屉里核对（6.3）；375 宽时长树把页推到很下，由抽屉解决；从带 `arrived` 的链接到达之后重载，Chromium、WebKit 里标题重新取焦点（M3、M12 既有），S2 的窄屏里侧栏在页之上才看得出窗口跟着动，抽屉之后不再如此（审查 A、C）。

## 附录：核对过的代码

`4c3f10b9` 上读过：`W/app/layout.tsx`、`routes.tsx:1-310`、`route-error.tsx`、`user-menu.tsx`、`theme-menu.tsx`、`language-menu.tsx`、`shortcuts.ts`、`arrival.ts`（导出与注释）；`W/pages/workspace/workspace-layout.tsx`、`search-box.tsx`、`notebook-nav.tsx`、`notebook-groups.tsx`、`workspace-switcher.tsx:25-66`；`W/pages/notebook/notebook-layout.tsx:40-90`、`page-tree.tsx:1-215`、`page-tree-item.tsx:126-160,215-352`、`notebook-places.tsx`、`quick-switch.tsx:50-72,136-195`、`new-page.ts:47,83`；`W/pages/settings/settings-layout.tsx`；`W/pages/create-workspace.tsx:27-32`；`W/pages/page/readers-input.ts`；`W/components/nav-item.tsx`、`W/components/ui/button.tsx`、`dropdown-menu.tsx`、`input.tsx`、`dialog.tsx:12-15`；`W/styles.css:14-129`；`W/design-tokens.test.ts:249-390`；`W/i18n/messages/en.ts`、`zh-CN.ts` 的有关键；vitest：`W/app/layout.test.tsx`、`routes.test.tsx:1-30`、`route-error.test.tsx`、`user-menu.test.tsx`（格名）、`W/pages/settings/profile-page.test.tsx:40-60,140-170`、`W/pages/workspace/workspace-layout.test.tsx:80-98`、`notebook-nav.test.tsx:195-240`、`search-box.test.tsx`（格名与 `:30-44,176-223`）、`W/components/ui/button.test.tsx`、`dropdown-menu.test.tsx`（格名）、`W/test/render.tsx:20-50`、`W/test/fakes.ts:209-229`；e2e：`e2e/playwright.config.ts`、`tools/e2e-projects/check.mjs:1-60`、`e2e/fixtures/auth-pages.ts:40-55`、`notebook-pages.ts:1-40`、`wiki-pages.ts:1-110`、`wiki-search.ts:1-30`、`workspace-pages.ts:44-60`、`narrow.ts`、`e2e/stories/identity/a6-sign-out.spec.ts:25-140`、`a8-profile.spec.ts:60-95`、`e2e/stories/smoke/s2-web-app.spec.ts:40-64`、`e2e/stories/collab/c6-expired.spec.ts:100-116`、`e2e/stories/page/pg2-rename-page.spec.ts:1-240`、`e2e/stories/notebook/n1-create-notebook.spec.ts:90-104`、`n2-private-notebook.spec.ts:60-90`、`e2e/stories/wiki/lw3-source-duplicates.spec.ts:305-380`、`lw8-health.spec.ts:640-660,706-726`、`lw13-index.spec.ts:300-320`、`e2e/stories/asset/as3-render.spec.ts:383-392`、`e2e/stories/mcp/mc3-write-edit.spec.ts:188-200`、`e2e/stories/search/se1-search.spec.ts:281-290,400-425`；`radix-ui` 1.6.7 的 `internal`（`FocusScope` 是 `@radix-ui/react-focus-scope` 的命名空间：`Root`、`trapped`、`loop`、`onMountAutoFocus`、`onUnmountAutoFocus`）、`@radix-ui/react-menu` 2.1.24 的 `SubTrigger`（`aria-haspopup="menu"`）与 `SubContent` 的 Esc（`rootContext.onClose()`）；`lucide-react` 1.48 的图标在没有可访问的属性时自带 `aria-hidden="true"`；`docs/v0.1/v0.1-design.md` 9.2、9.3、13.2 第 15–17、20、21、26、30、31 条、13.4 第 3 条与第 15 节的头；修订时另读：`W/app/guards.tsx`、`session-unavailable.tsx`、`layout-error.test.tsx`、`providers.tsx`（SWR 的设置）、`arrival.ts:1-60,170-212`、`W/pages/notebook/general-page.test.tsx:25-45`、`W/pages/page/readers-input.ts`、`W/stores/notebook.store.ts:18-24`、`W/components/ui/input.tsx`、`W/pages/notebook/page-tree-item.tsx:380-415`（`TreeMenu` 的量位）、`notebook-places.tsx`、`e2e/fixtures/contrast.ts`、React Router 8.4 的 `lib/router/history.js:199-230`（每次导航新的 `key`，同一地址的 `replace` 也是）、[M12-polish](../../v0.2/handoffs/M12-polish.md) 第 8 项；修订核对第 1 轮之后另读：`W/pages/page/unsaved-guard.tsx`、`page-edit.tsx:200-215`、`page-outline.tsx:40-50`、`attachment-row.tsx:39-46`、`attachments-section.tsx:520-540`、`W/pages/not-found.tsx`、`W/app/event-stream.tsx`、`W/pages/invitation.tsx`（`useSession` 的调用者）、`sign-out.tsx`、`user-menu.tsx` 的导入、`W/app/effective-role.ts:51-53`、`W/stores/notebook.store.ts:18-60`、`page-tree.tsx:125-140,212-230`、`page-tree-item.tsx:360-412`、`arrival.ts:1-30,180-212`、`routes.tsx` 的懒加载、`.oxlintrc.json`、`Makefile` 的 `build`、`e2e`、`e2e/stories/workspace/w3-shell-switch.spec.ts:1-40`；[M12-polish](../../v0.2/handoffs/M12-polish.md) 第 1、14 项与"处理进展"；[M5-polish](../M12-release/handoffs/M5-polish.md) 26、35、37、38；palette.html 的 `.side` 一组（`:62-91`）与外框的标记（`:229-250`）。`grep` 过的：`accountMenu`、`signOutThroughMenu`、`notebookGroups(`、`data-shell`、`setViewportSize`、`phoneWidth`、`ControlOrMeta`、`useArrivalFocus`、`radix-ui`、`headingClassName`。
