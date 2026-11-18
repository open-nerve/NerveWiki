# M12/P2 网页的打磨：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M12/P2 网页的打磨（六个 Step：S1 焦点与共用组件；S2 文字；S3 375 px、`bdi` 与对比度；S4 阅读视图；S5 编辑器、附件、上传、粘贴与导入导出；S6 版本、回收站、历史、目录、后台任务与搜索的网页） |
| 日期 | 2026-11-16（初稿；同日第一次修订：三份审查之后，照主持者的修订说明改，并删短；同日第二次修订：修订核对第 1 轮之后；同日第三次修订：修订核对第 2 轮之后；同日照 S1 的实施与审查修订；2026-11-18 照 S2 的实施与审查修订） |
| 状态 | 进行中（设计已审查：三份设计审查、三次修订与三轮修订核对之后，[审查记录](reviews/P2-design-review.md)；S1 已合并 `d1ade2ed`，[S1 的审查记录](reviews/P2-S1-focus-and-shared-review.md)；S2 已合并 `6505c3bf`，[S2 的审查记录](reviews/P2-S2-text-review.md)；实施与审查之后与代码不同之处照实改在各节，结果在 10.1、10.2；S3 实施中，S4–S6 未开工） |
| 上级文档 | [M12 总设计](00-M12-design.md) 4.1、4.5、第 3 节第 3、11 条、第 7 节 P2 一行与"次序与并行""移交的落实""总体设计的修订"第 11、17 条、9.1、9.3、9.6、9.7、第 10 节、第 11 节决定 312、331、附录 A；[M12 设计审查](reviews/M12-design-review.md)的 C-M1、C-M7、C-N5；[P1 设计](01-P1-foundation.md) 3.17（e2e 换文档的帮手）；[移交](handoffs/)：M5-polish 第 2–49 条里附录 A 分给 P2 的子项，M8-history 第 4 条（界面的一句），M4-performance 第 13 条（每小时整篇替换）；[v0.1 总体设计](../v0.1-design.md) 13.2 第 1、12、23、25、26 条，13.4 第 1、3 条；[M11/P1](../M11-upgrade/01-P1-freeze-start-rollback.md) 第 4 节（网页 Phase 的写法） |
| 代码位置的写法 | 同 00 号文档：前端写作 `W/pages/page/reading-view.tsx:138`（`W/` 是 `web/apps/web/src/`）；模块写作 `wiki/domain/template-en-agents.md:33`（`server/internal/modules/` 之下）；e2e 写作 `e2e/stories/links/l1-links.spec.ts:219`；其余写仓库根起的路径。代码的行号在 `86565dd6` 上读过；`main` 的 `48bbfc99`（P3 的设计，决定 361–377）与 `86565dd6`、`194ca350` 之间只改了文档，代码相同，初稿照 `194ca350` 读的代码行号照旧。`web/` 与 `wiki/domain/` 从 I3 的基点 `5e8af5b5` 起没有改动（`git diff --stat` 为空），I3 引的网页行号照旧。00 号文档的行号是 `48bbfc99` 的（已含 P3 的修订） |
| 工作笔记 | 设计只读，不改仓库；设计阶段只跑过一个探针（15.2，三种引擎，`m12/p2-probe-15.2/`）；15.2 的改法照修订核对 C 在三种引擎里跑过的探针定（`m12/p2-check1-c/`）。逐项的核对分 Step 做：S1、S2、S3、S5、S6 各由一位只读的助手对着 `194ca350` 逐项读代码、写现状、改法与测试，设计者抽查、统一取舍之后合进本文（底稿不进仓库）；S4 的 spec 由设计者自己写。凡是读代码推出、没有在浏览器里看过的，标"推断"；要在浏览器里先探查的（8.1、33.1、48 在 WebKit 上的滚动锚定、17.8 的无头全屏），写成 Step 的第一个动作 |

---

## 0. 分六个 Step

照 00 号文档第 7 节，P2 分六个 Step，各自一个分支（`m12-p2-s<m>`）、一次审查与修复核对、一次合并。每个 Step 自带它加的文字的两种语言（13.2 第 4 条）、假服务端的字段、钉住旧行为的测试的改动与 knip 的干净；每个提交单独为绿（tsc、knip、oxlint、vitest，改到 e2e 时还有 `make lint-web` 的门禁与那几个故事）。附录 A 的每一项落在哪个 Step、怎样测，总表在 4.0，逐项在 4.1–4.6。

- **S1 焦点与共用组件**（`m12-p2-s1`）：`ConfirmDialog` 发送中改用 `aria-disabled`、被拒之后焦点有去处；`useForm` 只有横幅的拒绝、而焦点已丢时焦点给横幅；`button.tsx` 的 `aria-disabled:` 样式（随 23.16 的 C 类 23.17、28.2）；到达的焦点（12.3、23.14、26.4、28.4、28.5）；其余焦点与可访问名称的项；撤销对话框的两句说明进 `aria-describedby`（24.3、49）；第 7 条"仍要退出吗"。总体设计 13.2 第 12、26、1、21、17 条随之改写。**单独为绿**：只改网页与 e2e；改到焦点的故事 A10、LW3 已在 `threeBrowsers`，不加新的条目。
- **S2 文字**（`m12-p2-s2`）：单复数（每个单数一个 `*One` 键）、根下的叫法、撤销的用词、其余误导的文字；第 40 条两份 `AGENTS` 模板（服务端唯一的改动，第 3 节）；H4 的界面一句。**单独为绿**：改一个英文文字的提交同时改读它的 e2e 与 vitest；模板与钉住它的 Go 测试同一个提交。
- **S3 375 px、`bdi` 与对比度**（`m12-p2-s3`）：flex 项补 `min-w-0` 或 `[overflow-wrap:anywhere]`；从右到左的名字进 `bdi`；焦点环、补全说明、暗色高亮的颜色；MC3 的 375 检查（32.3）；33.1 先在 WebKit 上排查。**单独为绿**：只改样式、标记与 e2e 的断言。
- **S4 阅读视图**（`m12-p2-s4`）：阅读视图的 HTML 就地更新（spec 在 4.4）：只差附件地址时就地换地址，内容变了按顶层块更新，语言变了不动 HTML，可写与否只改勾选框；焦点留在原来的元素或同键的元素；版本页与回收站的预览同一个钩子；失去角色之后重读笔记本；mermaid 的 id；按职责拆开 `reading-view.tsx`。**单独为绿**：网页、e2e，与一个只加 Go 测试与样例的服务端提交（P2-45）；新故事 PG15 进 `threeBrowsers`。
- **S5 编辑器、附件、上传、粘贴与导入导出**（`m12-p2-s5`）：附件一节、阅读视图里的附件、编辑器的上传与粘贴、导入对话框与任务列表的 A、B；上传的超时（16.3，取多长在 4.5.1 定）；15.1 给服务端加一个属性（P2-27）。**单独为绿**：按"附件与上传""编辑器""导入导出与来源""阅读视图里的附件"四组提交，每个提交单独为绿；新故事 AS6 只在 Chromium（P2-30）。
- **S6 版本、回收站、历史、目录、后台任务与搜索的网页**（`m12-p2-s6`）：在途的恢复与撤销进 store（22.14、23.10）；页面树的参数上限（12.12）；过时的树的一闪（27.2）；删除与新建之后的导航（2）；其余 A、B；TB2 偶发的根源（24.8、24.10）与 26.15。在 P4/S2 的页面树结论之后才开，结论要改树时在改法的 Step 之后（P2-1）；哪几项随结论变、开工之前补什么，在 4.6 的开头与 4.6.6。**单独为绿**：提交分组在 6.6。

### 0.1 次序、并行与翻译文件

次序照主持者在审查之后的决定（P2-1，C 的 Q1、Q2）：

- **一次合并一个**：P2 的 Step 不按编号合并。先做完的先合并，后合并的先 `git merge main`，解决冲突，提交 `Merge main into m12-p2-s<m>: <一句说明>`，推送、等分支的 CI 绿，再合并（02-workflow 第 3.6 节）。
- **只守真的硬依赖**：

  | 依赖 | 为什么 |
  |---|---|
  | S4 在 S1 合并之后开 | 两者都改 `W/reading/task-toggle.ts` 与阅读视图的上下文：S1 的 6.3 命名读者的勾选框，35.2 加 `leads`，11.7 加观察者；S4 的 U7 让勾选框跟着可写与否变。S1 的焦点规则也是 S4 的焦点核对的前提（P2-46）。 |
  | S5 的 17.4、17.7 在 S4 合并之后 | 它们建在 S4 的 `assets.ts`（`adopt`）与读最新值的 `assetsExpire` 上（4.5.2）。S5 的别的提交可以先做。 |
  | S6 的整句断言在 S2 之后 | S6 的测试整句比对撤销、回收站与搜索的说法，以 S2 的定稿为准（4.6.8）。 |
  | S6 的 24.2、21.3、22.2 在 S1 之后 | 被拒之后焦点的去处照 S1 的规则（P2-2、P2-3）；21.3、22.2 的 `replace` 靠 S1 的 28.4 定的接续（P2-4）。 |
  | S6 在 P4/S2 的页面树结论之后开 | 结论要改树时，在改法的 Step 之后开（00 号文档 `:796`）。 |
  | P3/S3 的网页部分在 S1 合并之后写 | P3 的硬依赖（P3 设计第 0 节，00 号文档 `:756`）：它的横幅新格照 S1 的写法。排 S1 时算上它：S1 拖久了，P3/S3 的网页部分跟着等。 |

- **推荐的排法**：
  - S1 与 P3 的一个 Step 并行。
  - S1 合并之后，S2 与 S3 并行，先做完的先合并；S3 只有 13 项，文字全不变，多半先完。
  - S4 在 S1 之后；S5 与 S4 并行，17.4、17.7 两个提交等 S4 合并、`git merge main` 之后再写。
  - S6 等结论。
  - 主持者照当时的队列排，本文不硬定。
- **同时在实施的不多于两个**，算上 P3、P4、P5（00 号文档 `:804`：本机 8 核，重的运行串行）。等结论的 S6 在结论之前不开，不占名额。
- **翻译文件**：
  - 每个 Step 新加的键加在它那一组键的末尾，按组排（`page.*`、`history.*`、`trash.*`……），不重排已有的键；改文字的键原地改。
  - 两个 Step 改到同一个键或同一句的，写在各 Step 的"牵连"里（例如 S6 的 35.3 照 S2 改过的 `history.badFilters` 的句式，4.6.8），后合并的照前一个的新文字再改。
  - 同一处代码：S1 的 28.5 让目录与历史的筛选带上路由状态（`index-page.tsx:56`、`history-page.tsx:134`），S6 的 26.5 也改历史的筛选（4.6）：后合并的保留对方的改动。
- **与 P3 同改的文件**（审查 B 的 N6、C 的 M4）：P3 设计 4.1 定了 P3/S1 的网页不改代码与文字。下表是 P3/S3 与 P2 同改的文件，都是文本冲突，语义不冲突，后合并的先 `git merge main`：

  | 文件 | P3/S3 改的 | P2 改的 |
  |---|---|---|
  | `W/i18n/messages/en.ts`、`zh-CN.ts` | 两种语言的 `problem.transfer.too_many_jobs`、`transfer.problem.name_unicode_path`、`transfer.problem.name_gbk`，改 `transfer.importCut`（P3 设计 4.2） | 每个 Step；S5 的 `transfer.importStalled`、`transfer.importUnanswered` 接在 `transfer.importCut` 之后（4.5 的 16.3），相邻的行，冲突是一定的：后合并的照对方的新文字再改 |
  | `W/pages/notebook/revert-dialog.tsx` | 后台一支显示新码 | S1 的 24.3、49；S2 的 23.4、23.11；S6 的 24.2、24.9 |
  | `W/pages/notebook/trash-row.tsx` | `offeredOf`（`:172-177`，决定 377），引用 `refusedText`（P3 设计 4.2） | S2 改 `:102`、`:104` 的说法；S3 的 26.12 改 `:83-106`、`:260-279`；S6 的 22.14 抽 `trashRestoredOf`、改 `RestoreButton` |
  | `W/pages/notebook/transfer-report.tsx` | `problems`（`:92-108`）与它的测试 | S1 的 19.3 改 `:203`、`:246`；S2 的 23.1、24.7 改 `transfer.failure.*`、`transfer.restoreLeft*` 的文字 |
  | `W/pages/notebook/export-dialog.tsx` | 引 `:76-80` | S2 的 19.1 改 `:54-58` |
  | `W/pages/notebook/revert-background.test.tsx` 的 `test.each`（`:138-166`） | 加"后台撤销答 `transfer.too_many_jobs`"一行 | S6 的 24.2 删"too large for the background too"一行 |
  | `transfer-import.test.tsx`、导出对话框的测试、`problem-messages.test.ts` | 各加一格 | S5、S2 改同一个文件的别的格 |

  - 审查 C 的表里还有 `W/test/history-server.ts`：P3 设计（4.2）只改 `W/test/jobs-server.ts`、不改它，P2 不改 `jobs-server.ts`，所以不重叠。
  - 20.2 与新码：导入的 `Check` 先判 `transfer.busy`，后判 `transfer.too_many_jobs`（P3 设计 3.16）。所以这本笔记本有进行中的导入时，答的是 `transfer.busy`，S5 的 `refusedBusy` 只认它；`too_many_jobs` 的横幅不与 `importUnderWay` 同时出现（推断，照服务端的次序；4.5 的 20.2）。

## 1. 基线

从 `main` 的 `48bbfc99` 开始：P1 完成，两个 Step 与三条旁支都已合并；P3 的设计已提交（决定 361–377）。它的代码与 `86565dd6`、`194ca350` 相同，网页代码与 I3 调查时的 `5e8af5b5` 相同。

**共用的网页代码**（逐项的现状在第 4 节）：

- **`ConfirmDialog`**（`W/app/confirm-dialog.tsx`）：`run`（`:92-104`）失败时只 `setFailure`、`setSending(false)`；发送中两个按钮都 `disabled`（`:177-183`），拿着焦点的按钮被禁用，浏览器把焦点交给 `body`，之后不还。17 个文件、25 处 `<ConfirmDialog`（I3 抽查的数，本文重数过）。撤销与还原的确认框不经它，已是 `aria-disabled` 加忙的守卫（`W/pages/notebook/revert-dialog.tsx:342-353`、`W/pages/page/version-restore.tsx:197-210`，13.2 第 12 条）。
- **`useForm`**（`W/app/form.ts`）与 `useFocusOnInvalid`（`W/components/form-field.tsx:89-97`）：失败之后焦点移到第一个 `aria-invalid` 的字段；没有字段错误、只有表单上方的横幅时，焦点没有去处，按钮发送中又是 `disabled`，焦点落到 `body`。网页里 19 个文件、20 次调用：字面的 `useForm(` 18 处，另两处是 `useForm<never>`。
- **`button.tsx`**（`W/components/ui/button.tsx:7`）：只有 `disabled:` 的样式，`aria-disabled` 的按钮看着可按。
- **到达的焦点**（`W/app/arrival.ts`）：链接带 `state={arrived}` 才算到达，标题取焦点；13.2 第 26 条"读者动了就不抢焦点"。
- **阅读视图**：4.4.1。
- **页面树**（`W/stores/page-tree.store.ts`、`W/stores/page-tree.ts`）：整棵读、整棵重读（I2 第 2 节）；P4/S2 量它（00 号文档 4.7.2）。
- **i18n**：`W/i18n/messages/en.ts`、`zh-CN.ts`，键相同（13.2 第 4 条）；没有复数的机制，单数另用 `*One` 键（00 号文档 4.5）。

**测试**：

- vitest 与组件同目录（`*.test.tsx`），`W/test/render.tsx` 的 `renderApp` 起整个应用的路由，`W/test/*-server.ts` 是各模块的假服务端，`W/test/fakes.ts` 是夹具。jsdom 没有布局（`getBoundingClientRect` 全是 0，`scrollY` 恒为 0），不做 HTML 的焦点修正（05-lessons 第 6 节）。
- e2e：换文档只经 `e2e/fixtures/navigation.ts` 的 `goto`、`reload`（门禁 `tools/e2e-navigation/check.mjs`，P1 设计 3.17）；同一个文档里的 `goBack`、`goForward` 照直接调。全量只跑 Chromium，`threeBrowsers`（`e2e/playwright.config.ts:3-4`）列的故事三种都跑：今天是 LW3、LW4、LW8、LW9、LW10、LW12、LW13、A10、A15、W5、UP1、UP7。`e2e/stories` 15 个分组、123 个 `*.spec.ts`、386 处 `test(`；`make e2e` 577 个（P1 设计 3.17）。
- CI 的 e2e 作业（`.github/workflows/ci.yml:265-271`，时限 20 分钟）最近几次 `main`：7:23（E2E 一步 6:12）、7:22（6:07）、9:26、7:04（6:03），以及 `194ca350` 那一次（run 43745380462，attempt 2）9:39（8:10）。同一棵树的同一份可以差三成（05-lessons 第 7 节）；`web` 一项（tsc、knip、oxlint、vitest）时限 15 分钟（`:232`）。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P2 一行与附录 A"进 v0.1 的"表里 P2/S1–S6 的六行：A 28 项、第 48 条、B 79 项，C 类三项（随 23.16 的 23.17、28.2，随第 3 条的 11.6 的失焦部分），D 类 14d、24.8、24.10、26.15、32.3，M 类 33.1 的排查，与 H4 的界面一句；一共 118 项（4.0 的总表逐项对照附录 A）。附录 A 的 A 31 项与 B 81 项里，10.1、10.2、25.1（A）与 30.2、30.4（B）在 P3/S1，不在本文。

**不做**：

- C 类 87 项（附录 A"已知限制，进 v0.2"）；第 3、11.6、12.2 的 C 的部分（S4 顺带改好的照实写在 4.4，不算承诺）。
- 附录 A"发布之后的人工验收"的 M 类 14 项与只从 ARIA 推断的八项（16.8、18.4a、21.15、21.16、22.10、26.1、26.2 的后一半、28.3）：不写自动测试，P5/S3 的读屏清单收它们（00 号文档 4.12）。
- D 类的其余（11.2、14a、14b、14c、17.5、22.15、22.18、23.8、26.16、30.3）。
- 服务端的接口、契约、迁移与组合根。P2 碰服务端的只有三处，都在第 3 节：S2 的两份 `AGENTS` 模板，S4 只加的 Go 测试与样例，S5 的 15.1 一个属性。
- 页面树的改法（分层加载或只渲染看得见的行）：P4/S2 没过时由 P4 的条件 Step 做（00 号文档 4.7.2、P4/S4 起），S6 在它之后开。
- 网页上显示 MCP 地址（决定 331）。

**移交的落实**（00 号文档"移交的落实"里 P2 的部分）：

| 移交的项 | P2 的落实 |
|---|---|
| [M5-polish](handoffs/M5-polish.md) 附录 A 分给 P2/S1–S6 的 A、B 与几项 C、D | 4.1–4.6 逐项 |
| 同上第 48 条 | S4（4.4，U7） |
| 同上第 49 条 | S1，与 24.3 同一个改法（4.1） |
| 同上 33.1 | S3 先排查：复现了在 S3 修，没复现留在发布之后的人工清单（4.3） |
| [M8-history](handoffs/M8-history.md) 第 4 条（永久删除）的界面一句 | S2（4.2）；部署文档与发布说明写明与变通在 P5/S3 |
| [M4-performance](handoffs/M4-performance.md) 第 13 条（每小时整篇替换） | S4 的 U3（4.4）；带宽照旧并入第 11 条只量（P4/S2） |

### 2.1 对 00 号文档的修订

照代码、本文的设计与审查之后主持者的决定，00 号文档有下面几处要改。接受本文的文档提交里一并改，第 8 节；`p2-design-commit/apply_p2_design.py` 按锚点改。行号是 `48bbfc99` 上 `00-M12-design.md` 的（已含 P3 的修订）。

1. **P2/S1–S5 不按编号合并**（"次序与并行" `:795` 的"P2/S1–S5 彼此按序合并（翻译文件）"）：
   - 翻译文件的冲突靠"后合并的先 `git merge main`"解决，与次序无关。
   - 改为"P2 的 Step 一次合并一个，后合并的先 `git merge main`；只守真的硬依赖，见 P2 设计 0.1"。
   - `:804` 的"同时在实施的 Step 不多于两个"之后加"（等页面树结论的 P2/S6 在结论之前不开，不占名额）"。
   - 另三处"按序合并"随之改成"一次合并一个"，免得读成按编号（修订核对 C 的 N14）：第 7 节 P2 表下的翻译文件一句（`:746`）、P3 表下的"翻译文件照上面按序合并"（`:756`，P3 的修订写的）、风险表"并行的 Step 争机器与翻译文件"一行（`:1121`）。
   - 主持者在审查之后定（C 的 Q1、Q2）。
2. **`ConfirmDialog` 被拒之后焦点留在"确认"上；横幅只在焦点已丢时取焦点**（第 7 节 P2/S1 一行 `:739` 的"被拒之后焦点还给确认按钮或说明"、"`useForm` 只有横幅的拒绝时焦点给横幅"；"总体设计的修订"第 11 条 `:948` 的新文字）：
   - 照字面无条件给横幅，会推翻 M8 撤销确认框"被拒之后焦点在 Undo 上"（`W/pages/notebook/revert-background.test.tsx:183`、HI3 的 e2e）。给"说明"，会让键盘的读者再按一次之前要先找回按钮（P2-2、P2-3）。
   - `:739` 的两句改为"被拒之后焦点留在确认按钮上（丢了就还给它）；`useForm` 的拒绝只有横幅、而焦点已丢时，焦点给横幅"。
   - 第 11 条的新文字整段换成 8.2 里 S1 的第 1 条那一段，从"`ConfirmDialog` 在 M12 照此"起，到"（`components/ui/button.tsx`）。"止。
3. **"总体设计的修订"加六条**（第 7 节"总体设计的修订"，接在第 18 条之后；P3 的设计不加新条，审查 C 核对过）：
   - **第 19 条，随 P2/S1 改**：13.2 第 21 条"退出登录不问"、第 17 条"打开、切换进入的页面不抢焦点"、第 26 条到达的那一句。S1 之后旧文字不实，新文字在 8.2。
   - **第 20 条，随 P2/S2 改**：13.2 第 17 条 `page.nameIn` 那一句，加根下的页的写法（9.5）。
   - **第 21 条，随 P2/S3 改**：13.2 加第 30 条（名字、窄屏与对比度，P2-19，8.2）；13.4 第 3 条加 `webkit-narrow`。
   - **第 22 条，随 P2/S4 改**：13.2 第 23 条"HTML 换掉时倒序清理、再重跑"与上下文的字段（4.4）；13.4 第 3 条加 PG15。
   - **第 23 条，随 P2/S5 改**：
     - 13.2 第 27 条加上传的超时，与下载链接在点击时比到期（P2-26、P2-29）；
     - 第 7 条加睡醒之后的重读（17.6）；
     - 第 12 条加粘贴对话框的先问（38.7）；
     - 第 21 条加上传在途时离开先问（18.1）。
   - **第 24 条，随 P2/S6 改**：13.2 第 12 条加"关着的原生选择等读者停下再用"（26.5，P2-34）。
4. **部件**（第 7 节 P2/S4 一行 `:742`、P2/S5 一行 `:743` 的"web、e2e"）：
   - **S5 加 server**："web、e2e、server（`platform/markdown/obsidian` 的一个属性）"。15.1 只改 CSS，只修得了"只写高度"；"宽高都写"要服务端给写下的尺寸一个标记属性（P2-27）。
   - **S4 加 server 的测试**："web、e2e、server（只加 Go 测试与 `testdata/m12` 的样例）"（P2-45）。
   - 两处都不改服务端的接口、契约与组合根。
   - S5 的"可按三组提交"改为"四组"（4.5）。
5. **9.1 的阅读视图一句照 S4 的 spec 写准**（`:1029`）：
   - 原文："更新之后，选区、展开的 callout、焦点、正在播放的媒体都还在"。
   - 改为："更新之后，留着的块里的选区、展开的 callout、焦点与正在播放的媒体都还在（同一个节点；公式与图由增强重写，其中的选区除外）；只差附件地址的重读不换任何节点；换了新的块里的焦点找回同键的元素，或它原来所在的那一块；任务勾选框找不回就不给（P2 设计 4.4）"。
6. **9.1 的超宽一句写明怎样只加 WebKit**（`:1025`"Chromium 与 WebKit 两种"）：后加"：375 的几格带标签 `@narrow`，由新的 Playwright 项目 `webkit-narrow` 在 WebKit 上再跑一遍，不把整份故事交给 `threeBrowsers`（P2 设计 4.3）"。
7. **附录 A 照本文的取舍补三处**：
   - **"已知限制，进 v0.2"一节**（`:1463` 起），"D 类的其余"一条之后另起一条："**P2 只做了一半的与没有并进的**（P2 设计 2.3、4.4.8）：22.16 的重开会话（S2 只改说法）、23.4 的链到那一项（要服务端点名上级）、21.10 的标签页（S6 只改说的话）；S3 没有并进的两处同类（变更集页面包屑的 `current`、回收站预览 `h1` 的长名）；退出登录不为在途的上传问；回收站预览'在后台恢复'的开始也有在途的窗口；S4 之后仍在的限制"。
   - **P2/S3 一行**（`:1452`）的"另外"一格加"菜单与选项的高亮（P2 设计审查 B 的 M4，主持者的决定）"：它不在 I3 的列表里，审查时新发现，在 S3 修（P2-43）。
8. **附录 A 的 8.1、33.1 是有条件的**（附录 A 的 P2/S4 一行 `:1453`、P2/S3 一行 `:1452` 与 M 类 `:1460`）：
   - 8.1 在 S4 先探查，33.1 在 S3 先排查；没复现的，移到"发布之后的人工验收"一节。
   - 这一条在 S3、S4 的文档提交里照结果改（33.1 的条件 00 号文档 4.5 已写），本文只预告，脚本不改。
9. **第 7 节 P2/S4 一行的"e2e 加阅读视图的焦点与滚动检查，三种引擎"**（`:742`）：后加"（新故事 PG15，UP1 (page) 加第 48 条的一段）"。

初稿的 2.1 第 1 条（"S6 可以在结论之前做"）撤回：主持者定了 S6 在结论之后才开（C 的 Q2），00 号文档 `:744`、`:796` 原样成立。

### 2.2 需要负责人确认

无。第 11 节的负责人与主持者的决定在本文里都照做；2.1 的九条是照代码改说法、次序、范围或做法。比移交写的少做的几处各有理由，进 v0.2，2.3 列出。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

正式编号 378–429，主持者在设计审查之后给的（P2-k 是 377 + k）；括号里是临时号，审查记录用它。

- 标"审查之后主持者定"的，是主持者在第一次修订说明里定的；别的是设计者的取舍，照审查与修订核对改过。
- 每条一句，细节在括号里的节。

- **378（P2-1）次序与并行**（审查之后主持者定，C 的 Q1、Q2；0.1）：P2 的 Step 一次合并一个，后合并的先 `git merge main`，不按编号；只守真的硬依赖：S4 在 S1 之后开；S5 的 17.4、17.7 在 S4 合并之后；S6 的整句断言在 S2 之后，24.2 的焦点与 21.3、22.2 的接续在 S1 之后。S2 与 S3 可以并行，先做完的先合并。S6 在 P4/S2 的页面树结论之后才开（结论要改树时，在改法的 Step 之后），结论之前不占"同时在实施的不多于两个"的名额。
- **379（P2-2）`ConfirmDialog` 被拒之后焦点留在"确认"上**，不给拒绝的说明：同 M8 的三个确认框（HI3 钉着），拒绝由 `role=alert` 说出；发送中 Esc 与"取消"不关闭对话框（4.1 的 23.16）。
- **380（P2-3）`useForm` 的横幅只在焦点已丢时取焦点**（在 `body`，或在发送中禁用的元素上）：不改 M8 撤销框"被拒之后焦点在 Undo 上"；横幅经一层 `data-form-banner` 找到，不改各处的调用；取得焦点的横幅用全局的 `:focus-visible`，不加 `outline-none`（4.1 的 9.1）。
- **381（P2-4）到达的焦点的四个新规则**：28.4 从导航提交起 `watchReader`，读者动过就只从无人处取；28.5 树打开的页带路由状态 `opened`，后退、前进回到它时标题从无人处取；`NotFoundPage` 的 `replacing`（23.14、18.6）只从无人处取（4.1）；同一路径与查询、只换 state 的 `replace` 不算新的导航，看守不重新起算，`offered` 接续（S6 的 21.3、22.2 用它，所以它们在 S1 之后合并）。
- **382（P2-5）12.3 的范围加工作区首页的笔记本卡片**（`W/pages/workspace/workspace-home.tsx:41`）：同一种问题，一个属性（4.1 的 12.3）。
- **383（P2-6）第 7 条**：用户菜单、邀请页与会话不可用时的三处"退出登录"共用 `useSignOut`；"有未保存的修改"取 `PageEditing.unsaved`，自动保存的 2 秒窗口里也问；别的标签页经 `nwiki.edits` 一问一答、等 100 毫秒（4.1.2）。
- **384（P2-7）11.7 留在 S1，用 S1 的做法**（不依赖 S4）；S4 的就地更新让多数情形不再发生，S4 的测试照过它的格（4.1 的 11.7、4.4.2）。
- **385（P2-8）单复数**：`en.ts` 里带计数、会显示 1 的十个键各加一个 `*One`，其余写明不改的理由（4.2.2）。
- **386（P2-9）根下统一叫"笔记本的顶层"/"the notebook's top level"**，`index.root` 一并；LLM Wiki 与升级里说库根位置的"根下"不改（4.2 的 22.11）。
- **387（P2-10）撤销的用词**：网页的英文一律 undo；服务端 `page/domain/errors.go` 的文字是 API 的词，网页不显示它，不改（4.2 的 23.1）。
- **388（P2-11）38.6 的单位改成 KiB、MiB**（`W/i18n/format.ts:50`）：与 README、粘贴那一句一致，名副其实；改 10 个测试文件、19 行（4.2 的 38.6）。实际 11 个文件、23 行（10.2）。
- **389（P2-12）15.3 用中性的说法**"没有这个名称的页面或附件"，不按扩展名猜附件：服务端只在有同名附件时才把 `x.png` 读成附件（`linking/domain/target.go:109-126`），按写法猜会与服务端不一致（4.2 的 15.3；移交原话是"按写法一开始就说没有这个附件"）。
- **390（P2-13）22.16 只改说法，23.4 只改英文**：重开失锁的会话、冲突链到上级那一项（要服务端点名上级）进 v0.2（4.2）。
- **391（P2-14）H4 的两句**：回收站"过了可以恢复的期限之后由系统永久删除"，不写天数、不写"立即"；版本列表"版本不能单独删除，系统也不会"，不提时间与清理（4.2 的 H4）。
- **392（P2-15）只加 WebKit 的 375 检查用新项目 `webkit-narrow`**（`grep: /@narrow/`、`testIgnore: threeBrowsers`），不把 SE1、MC3、PG2 整份交给 `threeBrowsers`（4.3.1.2）。
- **393（P2-16）焦点环与次要文字的颜色**：全应用的 `:focus-visible` 是 2 px 实色外框；亮色 `--ring` 改为 `oklch(0.556 0 0)`，八处 `ring-ring/50` 去掉 `/50`，destructive 的环改实色；亮色 `--muted-foreground` 从 0.556 调到 0.53；顺带找到的暗色 destructive 按钮的白字 2.89:1，加 `dark:bg-destructive/60`（4.3 的 12.4）。
- **394（P2-17）26.9 只把命中加粗到 700**，颜色不动：命中的文字对高亮已达标，不达标的是颜色之外的提示；把高亮的底色调到对页 3:1 要改阅读视图的样子（4.3 的 26.9）。
- **395（P2-18）`bdi` 用一个共用件 `W/i18n/isolated.tsx`**，范围是点名的几处加"名字后面紧跟数字、日期、另一个名字或箭头"的高风险几处（含变更集页"Renamed: “A” → “B”"）；其余约二十处进 v0.2（4.3 的 26.12、附录 A）。
- **396（P2-19）13.2 加第 30 条**：名字与别的文字在同一行时进 `bdi`；flex 项里的长文字要 `min-w-0` 或 `[overflow-wrap:anywhere]`；焦点指示对相邻颜色 ≥ 3:1（8.2）。
- **397（P2-20）S4 的 spec 写在本文 4.4**，不另写 `specs/P2-S4-…`：spec 与它的测试、变异同在一处，审查一次。
- **398（P2-21）S4 按文章的顶层块就地更新：尾先比、再头比，中间再按键贪心配一次，不做 LCS**：线性；留着的节点不移动；同名标题插在前面时留住原来那一节；链接的落点变了、本页相隔很远的几处同时变时，其间没变的块也留着（审查 A 的 M4；4.4.2）。
- **399（P2-22）`ReadingContext` 删掉 `role`，加 `writes()` 与 `onWritesChange`，`revision`、`assetsExpire` 读最新的**（4.4.2）。
- **400（P2-23）6.2 的重读也给"编辑"的 `forbidden`**：同一个根因，同一行（4.4.2 末条）。
- **401（P2-24）焦点找不到同一个节点与同键的元素时，给它原来所在的那一块**（经配对映射到的顶层块，`tabindex=-1`，不展开 callout、不滚动），不再落到 `body`；任务勾选框照 M5/P6 的规则，找不回就不给（`page-tasks.test.tsx:103`、`:146`、`:217` 照过）（审查 A 的 I1、M3，修订核对 A 的 M11；4.4.3）。
- **402（P2-25）8.1 先探查、复现了才改**；没复现就移到发布之后的人工清单（4.4.4）。
- **403（P2-26）上传的超时**：请求体在发时 60 秒没有进度、发完之后 180 秒没有答复就放弃，两种说明，离开页面的提醒随之关掉；不设整体的超时。M 比 N 长：中止会让服务端的事务回滚，误判一次就毁掉一个健康的上传，而发完之后发送缓冲还要排空（审查 C 的 M2；4.5.1）。
- **404（P2-27）15.1 在 S5 里给服务端加一个属性**（写下的尺寸 `data-nw-sized`），CSS 照它；只改 CSS 分不开写下的与图片自己的宽高（4.5 的 15.1）。
- **405（P2-28）17.2b 只给提示的 `span` 加 `select-none`，空格留在外面**：空格放进去（移交的写法）在 jsdom 里少一个空格，约 30 处可读名字的断言要改成与真浏览器不符的样子（05-lessons 第 6 节）。三种引擎里复制都不带提示，审查 C 的探针（`m12/p2-review-c/probe1.txt`）证实（4.5 的 17.2b）。
- **406（P2-29）17.4 下载的链接在点击时比到期，过期就再签再下**：只对下载的链接，新标签页的照旧靠到期之前的重读（13.2 第 27 条随之改）；放行的标记在这次派发之后立即清掉，S4 的就地换地址之后第二次过期也能重签（审查 C 的 M1；4.5 的 17.4）。
- **407（P2-30）只有 PG15 进三种浏览器**：PG15 查阅读视图的就地更新（4.4.6）。AS6 只在 Chromium：17.2b、17.4 的三引擎行为审查 C 已探过，相同；15.2 只在 Chromium 复现（4.5、P2-42）。
- **408（P2-31）27.2 在笔记本的外壳卸下时把树标为未确认**（照 P4/S2 的结论写：保留整棵树时照此，改成分层加载时照 4.6.6），重新挂上时页的外壳先读一次树再显示：比移交写的"点搜索结果时标记"宽（后退、前进、任何从笔记本外进来的路都算），不改搜索结果与根 store 的查找（4.6 的 27.2）。
- **409（P2-32）24.8 的 `links` 事件的读排在同一个单元的 `pages` 事件要的那次树的读之后**：移交写的"跳过树里没有的页"不够（事件先于删除的答复到时树里还有那一页，4.6 的 24.8）。
- **410（P2-33）21.10 只改"说的话"，不改标签页**：换版本时留着的标签页对新版本仍对，`version-page.test.tsx:227` 有意钉着它（4.6 的 21.10）。
- **411（P2-34）26.5 连同历史的三个筛选与目录的"类型"**：同一个写法（4.6 的 26.5）。
- **412（P2-35）23.19 连同回收站的预览**：预览同样不转小写，大写地址下恢复过的节点会被说成"页面不存在"（推断，4.6 的 23.19）。
- **413（P2-36）33.1 没复现时套件不变**（照 00 号文档 4.5"排查不进套件"），复现了才把 SE1 (page) 的 375 一段拆成 `@narrow` 的一格（4.3 的 33.1）。
- **414（P2-37）S3 不并进移交没点名的两处同类**：变更集页面包屑的 `current`（显示名加日期，要让 `BreadcrumbTrail` 收 `ReactNode`）、回收站预览 `h1` 的长名（没有现成的 375 检查）；记进 v0.2（4.3 的 32.4、38.10）。
- **415（P2-38）"仍要退出登录吗"不算在途的上传**：S1 只看未保存的文字（P2-6）；上传在途时退出登录，行随会话失效照常说失败，记进 v0.2（4.5 的 18.1）。
- **416（P2-39）26.18a 命中的两端都对齐到字素簇**：开头也往前移到簇的起点（移交只说末尾），命中盖住整个簇（4.6 的 26.18a）。
- **417（P2-40）16.4c 读作"清掉已不成立的那一句"**：没有在途的上传、这一轮也没话说时清空播报区域；仍成立的（"已复制嵌入"）不清，不做定时清空（4.5 的 16.4c）。
- **418（P2-41）S6 不做的两处**：回收站预览"在后台恢复"的开始也有在途的窗口（`trash-preview.tsx:124`），移交没写，进 v0.2；21.3、22.2 的"刷新之后不再说"只用 vitest 断言历史条目的 state，不在 HI2 加一次真的刷新（4.6 的 21.3、22.14）。
- **419（P2-42）15.2 在媒体的 `DOMActivate` 之后的下一个任务里把 `open` 设回去**（主持者定"先探、再定"；修订核对 C 的 M9 改成不留状态）：`DOMActivate` 的目标是 `<summary>` 里的音视频时，取下这个 `<details>` 当时的 `open`，`setTimeout(0)` 里变了就设回；不听 `toggle`、`keydown`、`pointerdown`，不跨事件记东西。三种引擎里探过：Chromium 点控件的空白与时间处会开合（没修时复现），改了之后不开合，只派发一次 `toggle`；读者点 summary 的文字、键盘的 Enter 与 Space、点过控件之后程序里的打开（锚点导航、焦点还原），都照常；Firefox、WebKit 本来就不开合，改与不改相同（`m12/p2-check1-c/probe.log`，`fix=timeout`）。上一版留着记录、等下一次 `toggle` 复原，在 Firefox 与 WebKit 上会撤回点过控件之后程序里的打开；初稿听 `click` 的改法在 Chromium 里从不运行（审查 C 的 I1；4.5 的 15.2）。
- **420（P2-43）菜单与选项的高亮在 S3 修**（审查之后主持者定，B 的 Q1 取甲）：菜单项、快速切换的选项与链接补全的选中项加 2 px、`--ring` 的内嵌环（底色照旧），对相邻的颜色至少 3:1；附录 A 的去处随之记（4.3 的 12.4）。
- **421（P2-44）Playwright 的项目由一个检查钉住**：`make lint-web` 加 `tools/e2e-projects/check.mjs`，以 `playwright test --list` 断言 `firefox` 项目列出 PG15、`webkit-narrow` 只列出带 `@narrow` 的那几格（审查 C 的 M7；5.1）。
- **422（P2-45）附件地址的拼法由 Go 与网页共用的样例钉住**：`testdata/m12/signed-view.json`，Go 一侧断言真的渲染与签名写出的就是它，网页一侧 `patch.test.ts` 读它核对 `addressesOnly` 与 `resign`（审查 A 的 I2；4.4.2）。
- **423（P2-46）S4 开工时对着 S1 合并之后的代码重做 4.4.9 的核对**，记进 S4 的记录（审查之后主持者定，A 的 Q1；6.4 第 0 步）。
- **424（P2-47）`resign` 不跳过已加载的图**（审查之后主持者定，A 的 Q2）：之后"在新标签页打开图片"拿到的仍是新的地址；每小时重下的带宽由 P4/S2 量（4.4.2）。
- **425（P2-48）23.1 连同 `upgrade.approveBody` 的中文**：同一个用词，中文也改"改动"（4.2 的 23.1）。
- **426（P2-49）22.13 用中性的一句，不先显示加载中**：树还没读到或读失败时说"它不在回收站里"，读到之后再照树分；改动最小（4.2 的 22.13）。
- **427（P2-50）23.12 的中文用 `{count}+`**：与 `search.capped` 一致，不改模板的空格（4.2 的 23.12）。
- **428（P2-51）9.5 的英文写"(at the top level of Plans)"**，不用所有格：以 s 结尾的笔记本名的所有格不好看（4.2 的 9.5）。前一半（名字里带笔记本名）由 S2 修复第 1 轮的决定 584 取代："(at the notebook's top level)"，不带笔记本名；后一半随之没有对象（实际见 10.2）。
- **429（P2-52）20.2 留被拒的那一句**（`importBusy`，回答这次按下），压下常驻的 `importUnderWay`（4.5 的 20.2）。

## 3. 服务端

P2 碰服务端的有三处：S2 的两份 `AGENTS` 模板（第 40 条）、S4 只加的 Go 测试与样例（P2-45）、S5 的 15.1 一个属性（P2-27）。三处都不改接口、契约、迁移与组合根。

### 3.1 S2：两份 `AGENTS` 模板（第 40 条）

| 文件 | 改动 |
|---|---|
| `wiki/domain/template-en-agents.md:33` | "Connect an agent to this notebook at `` `/mcp?notebook=<id>` ``. Three prompts are served there:" 改为 "Connect an agent to this notebook at `` `/mcp?notebook=<id>` ``. While the notebook is being upgraded to an LLM Wiki, the prompt `` `upgrade` `` guides the agent there; once it is one, three prompts serve it:"（冒号之后照旧接 `ingest`、`query`、`lint` 三项的列表） |
| `wiki/domain/template-zh-CN-agents.md:33` | "在 `` `/mcp?notebook=<id>` `` 把 agent 连到这本笔记本。那里有三个 prompt：" 改为 "在 `` `/mcp?notebook=<id>` `` 把 agent 连到这本笔记本。笔记本升级为 LLM Wiki 期间，那里的 prompt `` `upgrade` `` 指引 agent；成为 LLM Wiki 之后，有三个 prompt 为它服务：" |
| `wiki/domain/template_test.go` | 加 `TestTheAGENTSTemplatesNameThePromptsAsServed`（下；修复第 1 轮改名为 `TestTheAGENTSTemplatesNameTheUpgradePrompt`，实际见 10.2） |

- **为什么这样写**：发起升级时补写的 `AGENTS` 取同一份模板（`wiki/app/template.go:104-128` 的 `SeedUpgrade`），握手的说明含 `AGENTS` 的正文，升级中的 agent 读到"有三个 prompt"，这时它们都答 `notebook.not_llm_wiki`，`upgrade` 没提（移交第 40 条）。新句先说升级期间的 `upgrade`，再说建成之后的三个；一本一开始就是 LLM Wiki 的笔记本读到前半句无害。MCP 的笔记本入口静态列出四个 prompt（`wiki/adapter/mcp/prompts.go:31-49`：`upgrade` 与那三个），新句与它一致。两份都改，照 00 号文档 4.5（决定 299 原来只说中文那句；移交与 00 号文档说两份）。
- **已有笔记本里的 `AGENTS` 不随之改**：模板只在新建 LLM Wiki（`SeedNotebook`）与发起升级（`SeedUpgrade`）时写，不加迁移。
- **模板的三个守卫照过**：两种语言的块对应、行内代码相同（`TestTheTemplatesCorrespond`，`wiki/domain/template_test.go:111-153`：两句的行内代码都是 `/mcp?notebook=<id>` 与 `upgrade`）；代码之外没有 `[[`、`![`、`](`、`#`（`TestTheTemplatesLinkNothingOutsideCode`）；模板 2,209 与 2,107 字节，加约 120 字节，在 8 KiB 之下（`TestTheTemplatesAreSmall`）。
- **新测试**：`TestTheAGENTSTemplatesNameThePromptsAsServed`（修复第 1 轮改名为 `TestTheAGENTSTemplatesNameTheUpgradePrompt`：它只查模板里写的那一行，不读 MCP 实际登记的 prompt，对照进 v0.2；实际见 10.2）对 `template(LocaleEn)[1].Content` 与 `template(LocaleZhCN)[1].Content`（`AGENTS` 是第二页，同 `:127`）各断言含上表的那一整行（`strings.Contains(content, line+"\n")`），且不含旧句。没修的代码上两种语言都失败。今天没有测试钉着这一句：`wiki/app/template_test.go`、`bootstrap/mcp_instructions_test.go`、`bootstrap/wiki_template_test.go`、`bootstrap/upgrade_start_test.go`、`bootstrap/mcp_wiki_guard_test.go` 与 e2e 的 LW1 都从模板或 REST 动态取正文（S2 的助手逐个读过），不改。
- **变异**：任一语言改回旧句（新测试失败）；中文少了 `upgrade` 的反引号（`TestTheTemplatesCorrespond` 失败）。
- **CI**：只多 wiki 模块的一个短测试，组合根不变，`short` 与 `race` 两份各多不到 1 秒。

### 3.2 S4：附件地址的样例（P2-45，只加测试）

| 文件 | 改动 |
|---|---|
| `testdata/m12/signed-view.json`（新） | 同一页在相隔一小时的两个时刻的两份视图 HTML，带图、音频与下载链接 |
| Go 的样例测试（新，放在同时拿得到渲染器与 `asset/adapter/http` 的 `ContentURL` 的地方；推断是组合根的 Markdown 测试旁，`bootstrap/markdown_app_test.go` 一带） | 用真的渲染器、`ContentURL` 与固定密钥、固定时刻的签名器写出两份 HTML，断言与样例逐字相同 |

- **为什么**：网页的 `maskAddresses` 照服务端的拼法认地址（`asset/adapter/http/content.go:76-86`、`upload.go:175-180`、`asset/adapter/mac/sign.go:31-37`）。今天 Go 里钉着的只有假地址（`platform/markdown/obsidian/asset_test.go:51` 的 `?b=1&e=2&s=3`）。两边任一边改了拼法，`addressesOnly` 永远为假，17.1、17.8 悄悄没修好，vitest 照样绿。共用一份样例，两边各有一个测试红（审查 A 的 I2；网页一侧在 4.4.2）。
- **先例**：`testdata/m10/virtual-targets.json`，由 `W/pages/page/virtual-target.test.ts:17` 读。
- **变异**：网页一侧，正则不认 `&amp;`（`patch.test.ts` 读样例的那一格红）。服务端一侧，改参数的次序，或 `s` 的长度（样例测试红）。

### 3.3 S5：写下的尺寸的标记（15.1，P2-27）

| 文件 | 改动 |
|---|---|
| `platform/markdown/obsidian/asset.go:131-142` 的 `element` 与图片、视频的调用处 | 作者写了尺寸时（`sz != size{}`，在 `imageSize` 补图片自己的宽高之前判），另写一个空值的属性 `data-nw-sized=""` |
| `platform/markdown/obsidian/obsidian.go:105-106` 的 `Markup` | 给 `img`、`video` 登记 `data-nw-sized` |
| `platform/markdown/obsidian/asset_test.go:130-162` | 写了尺寸的几格，期望加 `data-nw-sized=""`；`"of an image of known size"`（`:130`，图片自己的宽高）不加 |

- **为什么**：服务端在没写尺寸时也写 `width`、`height`（`asset.go:157-178`），只改 CSS 分不开写下的与图片自己的（4.5 的 15.1）。
- **不碍别的测试**：服务端每次读都渲染，不缓存 HTML（`page/app/get_page_view.go:39`），不用升版本。整个程序的测试只以 `<img class="nw-asset" src=` 为前缀比对没写尺寸的图（`bootstrap/markdown_app_test.go:58-62`）；属性写在 `src` 之后就不碍（审查 C 核对过）。
- **包**：与 P3/S1 的 10.2 不冲突：10.2 改 `platform/markdown/parse.go`，这里改子包 `platform/markdown/obsidian`。
- **变异**：图片自己的宽高也写 `data-nw-sized`（`:130` 那一格红）。
- **CI**：只多单元测试，`short` 与 `race` 两份各多不到 1 秒。

## 4. 网页（按 Step）

本节逐 Step 写每一项：组件（`file:line`，`194ca350`）、改法、两种语言的文字、假服务端要加的、测试（vitest 或 e2e；在没修的代码上怎样失败）与变异；只能在真浏览器上人看的部分写明理由。4.4 是 S4 的 spec。

- **记法**：`W/` 是 `web/apps/web/src/`，`E/` 是 `e2e/`；服务端写 `x/y.go` 的在 `server/internal/modules/` 之下，`platform/…` 在 `server/internal/` 之下。
- **小节号**：4.k 之内照 4.k.n 排；文中的"第 n 节""n.m"指同一个 4.k 之内的小节，指向别处的写全（"2.3 的 P2-n""4.4.3"）。
- **推断**：标"推断"的是读代码推出、没有跑过的。实施时每个新测试先在没修的树上跑红，记进 Step 的记录（05-lessons 第 1 节）。
- **"拿不准的"**：各 Step 的那一小节留给实施者与审查者。底稿里要设计者定的，已在 2.3 定下，那里写"已定在 2.3"。

### 4.0 总表：对照附录 A

| Step | A | B | 另外 | 项数 | 节 |
|---|---|---|---|---|---|
| S1 焦点与共用组件 | 7 | 6.3、9.1、11.7、12.3、16.1、18.6、19.3、21.14、22.17、23.14、23.16、24.3、26.4、28.4、28.5、35.2、38.1、38.3、49（19 项） | C：23.17、28.2 | 22 | 4.1 |
| S2 文字 | — | 6.1、9.5、15.3、16.6、19.1、21.8、21.11、22.11、22.12、22.13、22.16、23.1、23.3、23.4、23.11、23.12、23.13、24.7、26.11、26.13、26.14、35.1、38.6、38.8、40、41.1、41.2、41.3（28 项） | H4 的界面一句 | 29 | 4.2 |
| S3 375 px、`bdi` 与对比度 | — | 12.4、26.9、26.12、26.17、32.1、32.2、32.4、35.8、38.10、38.11、38.12（11 项） | D：32.3；M：33.1 的排查 | 13 | 4.3 |
| S4 阅读视图 | 6.2、11.1、17.1、17.8；48（C 改判为 A） | 3、8.1、8.2、12.2、29 | C：11.6 的失焦部分；D：14d | 12 | 4.4 |
| S5 编辑器、附件、上传、粘贴与导入导出 | 12.8、15.1、15.2、16.3、16.4b、16.11、17.2b、17.4、17.6、17.7、18.1、18.2、19.4、20.3、38.7（15 项） | 16.4c、16.10、20.1、20.2、28.1、38.9 | — | 21 | 4.5 |
| S6 版本、回收站、历史、目录、后台任务与搜索 | 2、12.12、21.7、22.14、23.10、23.19、24.2、35.3 | 21.2、21.3、21.10、22.2、22.19、24.9、26.2 的前一半、26.5、26.18a、27.2 | D：24.8、24.10、26.15 | 21 | 4.6 |
| 合计 | 28，另有 48 | 79 | C 3、D 5、M 1、H4 | 118 | |

- **核对**：附录 A 的六行（`00-M12-design.md:1450-1455`，`48bbfc99`）逐项对过；审查 C 用自己的脚本又数了一遍，相同。每一个编号在它那个 Step 的一节里都有以它开头的标题（S4 是粗体的段首），一项不漏，也没有多出的项。每一项都写了变异（S4 的在 4.4.7）。两样都是脚本核对的。
- **只能人看的**：没有整项是。几项的一部分要在真浏览器上看，写在各项里：
  - 26.18a 的字形；
  - S1 的三引擎探查（28.5 的后退前进、12.3 的点链接）；
  - S4 的探查（8.1、WebKit 的滚动锚定、无头模式下的全屏）；
  - S3 的 33.1 排查。

### 4.1 S1 焦点与共用组件

S1 改三处共用的东西：`ConfirmDialog`、`useForm` 的横幅、到达的焦点（`W/app/arrival.ts`）。另有第 7 条（退出登录之前先问），其余是零散的焦点与名称。

- **本节的取舍**：P2-2 至 P2-7（2.3）。
- **13.2 的新文字**：第 12、26、1、21、17 条，写在 8.2。
- **与 S4 的分界**：6.3、11.7、35.2 改阅读视图的增强与上下文，在 S1 做，不依赖 S4。S4 在 S1 合并之后开（P2-1），重写 effect 时这几项的测试照过（4.4.5）。
- **可选的 PG11 一行**：加。它是 12.3 在真浏览器里的一处，不到 0.1 秒（5.2）。

#### 4.1.0 总表、键、e2e 与提交

##### 4.1.0.1 总表

| 编号 | 分类（I3） | 组件文件 | 测试的种类 | 测试文件 | 大小 |
|---|---|---|---|---|---|
| 23.16 | B（中） | `W/app/confirm-dialog.tsx`、`W/lib/focus-leaving.ts`（新的 `focusLost`） | vitest + e2e | `W/app/confirm-dialog.test.tsx`、`W/pages/page/source-attachments.test.tsx`、`W/pages/settings/security-page.test.tsx`（实际多改的一格，10.1）；`e2e/stories/identity/a10-api-tokens.spec.ts`（三种） | 中 |
| 23.17、28.2 | C（随 23.16） | `W/components/ui/button.tsx:7` | e2e | `e2e/stories/history/hi3-log-revert.spec.ts`（Chromium） | 小 |
| 9.1、38.3 | B（小、小到中） | `W/app/form.ts`、`W/components/form-field.tsx:83-97`、`W/pages/page/upload-rows.tsx`（修复第 1 轮，决定 506） | vitest + e2e | 新 `W/app/form.test.tsx`；`W/pages/notebook/source-uploads.test.tsx`（决定 506）；`W/pages/notebook/page-tree-writes.test.tsx`；`W/pages/notebook/source-add-dialog.test.tsx`；`e2e/stories/wiki/lw3-source-duplicates.spec.ts`（三种） | 小 |
| 12.3 | B（小） | `W/pages/page/subpage-list.tsx:27`、`W/pages/page/breadcrumbs.tsx:54`、`W/pages/workspace/workspace-home.tsx:41` | vitest | `W/pages/page/page-layout.test.tsx`、`W/pages/workspace/workspace-home.test.tsx` | 小 |
| 26.4 | B（小） | `W/pages/search/search-page.tsx:80,172-180` | vitest | `W/app/arrival.test.tsx` | 小 |
| 28.4 | B（中） | `W/app/arrival.ts`、`W/app/layout.tsx` | vitest | `W/app/arrival.test.tsx` | 中 |
| 28.5 | B（小） | `W/app/arrival.ts`、`W/pages/notebook/page-tree-item.tsx:120`、`W/pages/notebook/page-tree.tsx:150,154`、`index-page.tsx:56`、`history-page.tsx:134` | vitest | `W/app/arrival.test.tsx` | 小 |
| 23.14 | B（小） | `W/app/arrival.ts`、`W/pages/not-found.tsx:17-20`、`W/pages/notebook/changeset-page.tsx:110-111` | vitest | `W/pages/notebook/revert-dialog.test.tsx` | 小 |
| 18.6 | B（小） | `W/pages/page/page-layout.tsx:64,226-243`、`PageGone` | vitest | `W/pages/page/page-lock.test.tsx`、`W/pages/page/page-edit.test.tsx` | 小 |
| 7 | A（中） | `W/stores/edit-closing.ts`、`W/stores/root.store.ts`、新 `W/app/sign-out.tsx`、`W/app/user-menu.tsx:49`、`W/pages/invitation.tsx:167`、`W/app/session-unavailable.tsx:25` | vitest + e2e | `W/stores/edit-closing.test.ts`、`W/stores/root.store.test.ts`、`W/app/user-menu.test.tsx`、`W/pages/page/page-lock.test.tsx`、`W/pages/invitation.test.tsx`、`W/app/guards.test.tsx`（修复第 1 轮）；`e2e/stories/collab/c6-expired.spec.ts`（Chromium） | 中 |
| 6.3 | B（小） | `W/reading/task-toggle.ts:14-46` | vitest | `W/reading/task-toggle.test.ts` | 小 |
| 11.7 | B（小） | `W/pages/page/unresolved-link.tsx:147-148,176-178,199-206`、`W/reading/unresolved-links.ts:46` | vitest | `W/pages/page/unresolved-link.test.tsx`、`W/reading/unresolved-links.test.ts` | 小 |
| 35.2 | B（小） | `W/reading/enhancement.ts`、`W/reading/unresolved-links.ts:21-25`、`W/pages/page/unresolved-link.tsx`、`reading-view.tsx`、`version-view.tsx` | vitest + e2e | `W/reading/unresolved-links.test.ts`、`W/pages/page/unresolved-link.test.tsx`、`W/pages/page/virtual-navigation.test.tsx`；`e2e/stories/wiki/lw10-virtual.spec.ts`（三种；实际多改的，10.1） | 小 |
| 16.1 | B（小） | `W/pages/page/attachment-row.tsx:84-150`、`attachments-section.tsx:263-276` | vitest | `W/pages/page/attachments-section.test.tsx` | 小 |
| 19.3 | B（小） | `W/app/not-loaded.tsx:12-25`、`W/pages/notebook/transfer-report.tsx:203,245-246`、`transfer-job-row.tsx:203` | vitest | `W/pages/notebook/transfer-page.test.tsx` | 小 |
| 21.14 | B（小） | `W/pages/page/conflict-panel.tsx:29-67`、`W/pages/page/comparison.tsx:84-117` | vitest | `W/pages/page/conflict.test.tsx` | 小 |
| 22.17 | B（小） | `W/pages/workspace/ownerless-page.tsx:66-85` | vitest | `W/pages/workspace/ownerless-page.test.tsx` | 小 |
| 24.3、49 | B（小） | `W/pages/notebook/revert-dialog.tsx:326-332,366-382` | vitest | `W/pages/notebook/revert-dialog.test.tsx`、`revert-background.test.tsx` | 小 |
| 38.1 | B（小） | `W/pages/page/page-layout.tsx:312` | vitest | `W/pages/page/source-actions.test.tsx` | 小 |
| 13.2 第 12、26、1 条 | — | `docs/v0.1/v0.1-design.md` | — | — | — |

数目：vitest 推断新加约 48 格、改约 10 格；e2e 改 5 个故事文件（A10、LW3 在 `threeBrowsers` 里，HI3、C6、可选的 PG11 只跑 Chromium），不加故事、不加 `threeBrowsers` 的条目。实际（10.1）：vitest 4,537 → 4,602 格（实施 54 格，修复第 1 轮 11 格）；e2e 改 6 个故事文件（多 LW10），不加故事、不加 `threeBrowsers` 的条目。

##### 4.1.0.2 本 Step 新加或改的 i18n 键

新加（en 与 zh-CN 同时加，13.2 第 4 条）：

| 键 | en | zh-CN | 用处 |
|---|---|---|---|
| `signOut.unsavedTitle` | `Sign out anyway?` | `仍要退出登录吗？` | 第 7 条的对话框标题 |
| `signOut.unsavedHere` | `A page you are editing in this tab has changes that are not saved. Signing out saves what it can first; what it cannot save is lost.` | `这个标签页里正在编辑的页面有还没有保存的修改。退出登录会先尽量保存，保存不了的会丢失。` | 第 7 条，本标签页有 |
| `signOut.unsavedElsewhere` | `A page you are editing in another tab has changes that are not saved. Signing out saves what it can first; what it cannot save is lost.` | `另一个标签页里正在编辑的页面有还没有保存的修改。退出登录会先尽量保存，保存不了的会丢失。` | 第 7 条，别的标签页报告有 |
| `signOut.confirm` | `Sign out` | `退出登录` | 第 7 条的确认按钮 |
| `signOut.signingOut` | `Signing out…` | `正在退出登录…` | 第 7 条发送中的确认按钮 |
| `transfer.reportRetryOf` | `Try again (report on {name})` | `重试：{name} 的报告` | 19.3 报告里"重试"的可读的名字（可见文字"Try again""重试"在前，WCAG 2.5.3；en 的括号照 `transfer.inHistoryOf`，zh 的冒号照 `transfer.cancelOf`） |

沿用（不改文字）：`editor.stay`（`Stay` / `留下`，第 7 条的"留下"，与离开编辑的提醒同一个词）、`status.loading`（21.14 的"加载中…"）、`page.sourceReadonly`（38.1，只换元素）、`history.revertLinks`、`history.backgroundAsk`（24.3、49，只进描述）。

##### 4.1.0.3 本 Step 改到的 e2e 故事

| 故事 | 加什么 | 浏览器 | 推断多跑 |
|---|---|---|---|
| `e2e/stories/identity/a10-api-tokens.spec.ts`，"A10 (page): a token asks for the password, … stops when revoked"（`:130`）的撤销一段（`:184-191`）之前 | 撤销被拒一次：路由扣住第一个 `DELETE /api/v0/api-tokens/<id>`、之后答 503 `server_busy`、再 `route.fallback()`（不 `unroute`，05-lessons 第 6 节）；键盘（`focus()` 再 `Enter`）按"Revoke"；发送中按钮 `aria-disabled="true"` 且 `toBeFocused()`；被拒之后 alert 是 `The server is busy. Try again in a moment.`、"Revoke" `toBeFocused()`；再按一次，204 照旧 | 三种（A10 已在 `threeBrowsers`） | 每种约 1–2 秒，共约 4–5 秒 |
| `e2e/stories/wiki/lw3-source-duplicates.spec.ts`，"LW3 (page): at 375 wide …"（`:286`）粘贴被拒的一段（`:322-326`） | 提交改成键盘（`focus()` "Add source" 再 `Enter`），被拒之后 `dialog.getByRole("alert")` `toBeFocused()`，其余断言照旧 | 三种（LW3 已在 `threeBrowsers`） | 每种不到 0.5 秒 |
| `e2e/stories/history/hi3-log-revert.spec.ts`（`:419-447` 扣住撤销的一段） | 发送中"Cancel"与"Undo…"`toHaveCSS("opacity", "0.5")`（23.17、28.2） | Chromium（HI3 不在 `threeBrowsers`；不透明度不分引擎） | 不到 0.2 秒 |
| `e2e/stories/collab/c6-expired.spec.ts`，"C6 (page, signed out elsewhere)"（`:83`） | `signOutThroughMenu(tab, …)` 之后先等 `alertdialog` "Sign out anyway?"，核对 `signOut.unsavedElsewhere` 的整句，按其中的"Sign out"，再 `page.clock.runFor(200)` | Chromium | 不到 0.5 秒 |
| 可选：`e2e/stories/page/pg11-navigation.spec.ts:144` | 面包屑"Plans"点了之后笔记本首页的 `h1` `toBeFocused()`（12.3 在真浏览器里的一处） | Chromium | 不到 0.1 秒 |

合计推断多约 6–7 秒（三种引擎的 A10、LW3 按三遍算）；没有故事要进 `threeBrowsers`。另要一次性的三引擎探查（不进套件，记进 Step 的记录）：28.5 的后退前进、12.3 的点链接，见 4.1.5。

##### 4.1.0.4 提交的分组

见 6.1。

##### 4.1.0.5 拿不准的、要实施者先核对与审查重点看的

已定在 2.3 的：P2-2（23.16 被拒之后的焦点）、P2-3（9.1 的横幅只在焦点已丢时取，与它的找法）、P2-4（到达的三个新规则）、P2-5（12.3 的范围）、P2-6（第 7 条的范围）、P2-7（11.7 留在 S1）。`app/arrival.ts` 第一次静态导入 `pages/`，oxlint 没有拦它的规则（审查 A 核对过）。实施者先核对的：

1. 16.1：行的 layout 清理先于 `<li>` 与菜单的 portal 被移除（推断，照 React 19 的删除次序，没有跑过）；先用 16.1 的测试证实。实际：次序成立；但菜单开着时在 layout 清理里给的焦点被菜单的 `FocusScope` 拉回，改成 layout 清理记、passive 清理给（10.1）。
2. 28.4：路由的懒加载块在提交之前读，其间的按键不算（推断，只在第一次进一种页时有）；全量 vitest 里有没有格依赖"标题挂上之前有点击，标题照样抢"（推断没有）。实际：全量 vitest 在看守之下全绿，没有格依赖它；懒加载块读的期间按的键没量（vitest 里路由的懒加载在同一次 `act` 里读完，造不出这个窗口；e2e 要扣住块的请求，S1 没做）。
3. 28.5：树里"Directory""History"两行与页行同类（推断）。实际：成立，两行带 `opened`。
4. 11.7：`aria-busy` 照代码读成"问落点的期间"，不含新建页的确认框开着时。实际：照代码就是这样，`finally` 里去掉；`unresolved-link.test.tsx` 两格在"不存在"的对话框开着时断言没有 `aria-busy`。
5. 23.16：WebKit 用鼠标点时焦点在对话框的内容上，被拒之后也在那里（推断）；A10 的三种引擎回答。实际：不成立：A10 用键盘按；修复第 1 轮的鼠标探针里，Playwright 的三种引擎都给点过的"确认"焦点，发送中与被拒之后仍在它上面。Safari 不给点过的按钮焦点，那里焦点留在原处（照代码推），交 P5/S3 的人工清单（决定 509）。

#### 4.1.1 共用的三处

##### 23.16 `ConfirmDialog` 发送中用 `disabled`，焦点落到 `body`

- **现状**：
  - `W/app/confirm-dialog.tsx:176-183`：发送中"取消"`disabled={sending}`（`:177`），"确认"`disabled={sending || !ready}`（`:181`）。
  - 按下"确认"的焦点在它上面；它一禁用，Chromium 在下一次渲染时把焦点交给 `body`（05-lessons 第 6 节）。被拒之后（`:97-100` 只 `setFailure`、`setSending(false)`）按钮回来，焦点不回来，Tab 从页首起（Radix 的 `FocusScope` 只在 `focusin` 时拉回跑出去的焦点，`body` 不算跑出去，推断）。
  - 一次按下一个请求今天靠 `disabled` 与确认词的 `!sending`（`:168`）；`onOpenChange` 用 `sending` 挡关闭（`:120`）。
  - 例：管理员在笔记本的设置里按"降级"，有导入在排队，答 409 `transfer.busy`，焦点在 `body`。
- **先例**：撤销与还原已这样做（`version-restore.tsx:123-131,166-169`，`revert-dialog.tsx:181-182,306-321`）：两个按钮 `aria-disabled`、确认 `aria-busy`、一个 `useRef` 的忙守卫；被拒之后不移焦点。差别：`ConfirmDialog` 的 `onOpenChange` 看忙守卫的 ref，不看 `sending` 状态，状态还没渲染时按 Esc 也挡得住。
- **改法**（`W/app/confirm-dialog.tsx`）：
  - 新 `busy = useRef(false)` 与 `confirmButton = useRef<HTMLButtonElement>(null)`。
  - `run()`（`:92-104`）：
    - `busy.current || !ready` 时什么也不做；
    - 否则置忙、`setSending(true)`、`setFailure(undefined)`，再 `await confirm()`；
    - 被拒：`setFailure(error)`、`setSending(false)`；`focusLost()` 时焦点给 `confirmButton`；不关；
    - 成功：`confirmed.current = true`、`setOpen(false)`；
    - `finally` 清忙。
  - `onOpenChange`（`:119-125`）的条件 `!sending` 改成 `!busy.current`：发送中 Esc、点"取消"都不关（`aria-disabled` 的"取消"仍触发 Radix 的 `onOpenChange(false)`）；点外面本来就不关（`W/components/ui/alert-dialog.tsx:6-7`）。
  - 确认词的 `onKeyDown`（`:167-171`）：Enter 就 `void run()`，`run` 自己看 `ready` 与忙。
  - 按钮：
    - "取消"：`aria-disabled={sending || undefined}`，不再 `disabled`；
    - "确认"：`ref={confirmButton}`、`disabled={!ready}`，发送中 `aria-busy` 与 `aria-disabled`；
    - **`!ready` 仍用 `disabled`**：没输入确认词时没有焦点要保。`general-page.test.tsx:154-156`、`notebook/general-page.test.tsx:118`、`ownerless-page.test.tsx:251` 与 e2e 的 `fixtures/workspace-pages.ts:93` 照旧（Playwright 的 `toBeDisabled` 两者都认）；
    - 边缘：发送中确认词被重读改掉（另一处改了名）时按钮在发送中禁用。罕见，不处理。
  - `focusAfter`、`held.onClosed(confirmed)`、`onOpenAutoFocus`、`onCloseAutoFocus` 都不动（`:129-144`）：成功之后的去处照旧。
  - 文件头的注释（`:56-64`）补一句：按钮发送中 `aria-disabled` 而不禁用，禁用会丢焦点；被拒之后焦点在"确认"上，由 alert 说出（13.2 第 12 条；M12/P2/S1）。
  - 新帮手 `W/lib/focus-leaving.ts` 的 `export function focusLost(): boolean`：`document.activeElement` 为空、为 `body`，或 `matches(":disabled")` 时为真。最后一种是发送中禁用的按钮：Chromium 下一次渲染才把它的焦点交给 `body`，Firefox 与 WebKit 让焦点在禁用的按钮上留一阵（S1 审查 C 的 M3），jsdom 从不。
  - 文字不变。
- **被拒之后焦点留在"确认"上**（已定：2.3 的 P2-2）：焦点根本不走，"再按一次"是最常见的下一步；同 M8 的三个确认框（`revert-background.test.tsx:168`，HI3 的 `:419-447`）。拒绝由 `role=alert` 播报：每次发送先去掉旧的 Alert（`:94`），同一个拒绝第二次也播报；念不念到进发布之后的读屏验收。`focusLost()` 一支是保险（按理没有元素被禁用）。
- **调用处**：17 个文件、25 处 `<ConfirmDialog`，调用处都不用改；38.3 点名的八个在其中。
- **测试**：vitest，新 `W/app/confirm-dialog.test.tsx`（en；文字不变，不另做 zh-CN 格）：
  1. `"while confirm is out, its buttons are aria-disabled, not disabled, the focus staying on confirm; pressed again, Esc and Cancel do nothing, and the dialog stays"`：
     - `confirm` 返回一个测试扣住的 promise（普通函数，不用 `vi.fn`，05-lessons 第 2 节）；点"Delete"；
     - `[confirm.getAttribute("aria-disabled"), confirm.hasAttribute("disabled"), confirm.getAttribute("aria-busy"), cancel.getAttribute("aria-disabled"), cancel.hasAttribute("disabled")]` 等于 `["true", false, "true", "true", false]`，焦点在"Deleting…"；
     - 再点它、按 `{Escape}`、点"Cancel"：对话框仍在，`confirm` 只调用了一次。
     - **没修的代码上**：第一条断言拿到 `[null, true, null, null, true]`。
  2. `"a refusal stays in the dialog, the focus on confirm, which sends again"`：以 `new Error("refused")` 拒绝；alert 的 `textContent` 整句比对 `errorText` 的通用文字；焦点在"Delete"、不再 `aria-disabled`；再点一次，`confirm` 第二次被调用。**没修的代码上**：照过（jsdom 不做焦点修正）。它钉的是"被拒之后仍可再按、焦点不被挪走"；分新旧的是第 1、3 格。
  3. `"a refusal that finds the focus lost gives it back to confirm"`：扣住期间 `act(() => (document.activeElement as HTMLElement).blur())`，模拟 Chromium 的焦点修正；拒绝之后焦点在"Delete"。**没修的代码上**：在 `body`。
  4. `"Enter in the typed word sends once, however often it is pressed; refused, the focus stays in the field"`：`typedConfirmation`，输入确认词，连按两次 `{Enter}`，扣住，`confirm` 一次；拒绝之后焦点在输入框。**没修的代码上**：两次 Enter 之间没渲染时发两次（推断，`sending` 是状态）；焦点那条不分新旧。
  - e2e：A10（三种），见 4.1.0.3。用键盘按（`focus()` 再 `Enter`）：Safari 不给点过的按钮焦点，键盘在各种浏览器里一样（Playwright 的三种引擎用鼠标点也给焦点，S1 的鼠标探针，10.1）。**没修的代码上**：三种都红在发送中的 `aria-disabled` 断言（旧的按钮是 `disabled`），没有走到焦点的断言（实际见 10.1）。
- **已有测试要随之改的**：
  - `W/pages/page/source-attachments.test.tsx:396`（`:341` 那一格的 `entry === "Delete"` 一支，`submit` 是 `ConfirmDialog` 的"Delete"）：发送中 `hasAttribute("disabled") === true` 改成 `getAttribute("aria-disabled") === "true"` 且 `hasAttribute("disabled") === false`。"Rename""Move to…"（`useForm`，仍 `disabled`）与 `:408` 照旧。
  - 核对过照旧的：`app/confirm-dialog.test.tsx:47`（重开之后不禁用）；e2e `fixtures/workspace-pages.ts:93`（输入确认词之前 `toBeDisabled`）、`a11-deactivate.spec.ts:144`（被拒之后 `toBeEnabled`）。
- **变异**：
  - `aria-disabled` 改回 `disabled`（两个按钮）→ 第 1 格红；A10 在 Chromium 红。
  - 去掉忙守卫（`run` 不看 `busy`）→ 第 1 格"再点一次"之后 `confirm` 两次，红；第 4 格同。
  - `onOpenChange` 不看 `busy` → 第 1 格按 Esc 之后对话框没了，红。
  - 去掉被拒之后的还焦点（`focusLost` 那一支）→ 第 3 格红。
  - 被拒之后焦点给 Alert（不取的做法）→ 第 2 格与 A10 被拒之后的 `toBeFocused` 红。实际：Alert 挂上时就取的，vitest 第 2、3、4 格与 A10 三种都红；在 `setTimeout` 里取的，A10 照过（`toBeFocused` 判过一次就不再看），vitest 第 3 格抓（10.1）。
- **牵连**：23.17、28.2 同一提交；第 7 条的对话框也是 `ConfirmDialog`。

##### 23.17、28.2 `aria-disabled` 的按钮看着可按

- **现状**：`W/components/ui/button.tsx:7` 的基础类只有 `disabled:pointer-events-none disabled:opacity-50`。
  - 自己加了 `aria-disabled:opacity-50` 的：`version-restore.tsx:204` 的"还原"、`import-dialog.tsx:392`、`index-filters.tsx:74`、`comparison.tsx:103`；`text-diff.tsx:347` 的类串已有。
  - 没加的：`version-restore.tsx:199`、`revert-dialog.tsx:345` 的"取消"（28.2）、`revert-dialog.tsx:349` 的"撤销"、`load-more.tsx:169-177`、`trash-row.tsx:423-432`、`transfer-job-row.tsx:174-182`、`page-layout.tsx:274-284`、`page-editing-bar.tsx:60-67`、`upload-rows.tsx:112-128`、`page-backlinks.tsx:178-186`；23.16 之后 `ConfirmDialog` 的两个按钮也在内。
- **改法**：`button.tsx:7` 的基础类在 `disabled:opacity-50` 之后加 `aria-disabled:opacity-50`。**不加** `aria-disabled:pointer-events-none`：点了什么都不做由代码管，焦点要能留在上面。四处自己加的类可以去掉（`cn` 用 tailwind-merge，留着也无害）。文字不变。
- **测试**：e2e，HI3（Chromium）扣住撤销的一段（`hi3-log-revert.spec.ts:436-447`）加 `await expect(sessionDialog.getByRole("button", { name: "Cancel", exact: true })).toHaveCSS("opacity", "0.5")` 与 `await expect(confirm).toHaveCSS("opacity", "0.5")`。**没修的代码上**：两者都是 `1`（撤销对话框的两个按钮都没有自己的类）。A10 那一段钉不住它：旧代码的 `ConfirmDialog` 是 `disabled`，同样变暗。
- **变异**：去掉 `button.tsx` 的 `aria-disabled:opacity-50` → HI3 红。
- **牵连**：所有 `aria-disabled` 的 `Button` 在忙时变暗（"编辑"进入中、"完成"等上传、"更多"读着），正是 23.17 要的。

##### 9.1、38.3 只有横幅的拒绝之后焦点落到 `body`

- **现状**：
  - `W/app/form.ts:36-73` 的 `useForm`：失败时 `setFailure`、`setFailures(n+1)`（`:57-60`）；`useFocusOnInvalid(failures)`（`:41`，`W/components/form-field.tsx:89-97`）只找 `[aria-invalid="true"]`，没有就什么也不做。
  - 提交按钮发送中 `disabled`（13.2 第 12 条），焦点在它上面时 Chromium 交给 `body`；拒绝没有字段（只在横幅里说）时没人把焦点放回来。
  - 9.1 点名的：改名 `W/app/rename-form.tsx:104`（I3 写 `:96`）、移动 `W/pages/notebook/move-page-dialog.tsx:194`（"取消"）、`:197`（"移动"），都 `disabled={sending}`。改名被链接锁拒（`linking.pages_locked`，`rename-page-dialog.tsx:68` 的 `pagesLocked`）、移动被拒（409 `page.cycle`、`page.too_deep`）都只在横幅里说。
  - 38.3 点名的粘贴来源：`W/pages/notebook/source-add-dialog.tsx:212` `disabled={sending || !canAddSource(...)}`；重复的拒绝（`duplicate`，带链到那份来源的链接）只在横幅里说。38.3 另点名的降级与升级的八个对话框是 `ConfirmDialog`，归 23.16。
  - 例：在树的菜单里把一页移到它自己的子页下，答 409 `page.cycle`；对话框上方出现原因，焦点在 `body`。
- **经 `useForm` 的表单**：20 次调用、19 个文件（`grep -rn "useForm(\|useForm<" W`，去掉测试与定义）。字面的 `useForm(` 18 处，另两处是 `useForm<never>`（`notebook/general-page.tsx:96`、`onboarding/notebook-step.tsx:95`）。每一处的横幅都是 `{banner !== undefined && <Alert>{banner}</Alert>}`，在表单里（`upload-rows.tsx:240` 是 `note`，含 `banner`），所以只改 `form.ts` 与 `form-field.tsx` 就全改到（实际不成立：上传对话框里来源的重试，`upload-rows.tsx` 的 `SourceTitleForm` 的提示先取上传的失败、不带 `data-form-banner`，"Add source"发送中卸下、焦点被 Radix 交给对话框本身；修复第 1 轮改成横幅在前、按钮发送中禁用而不卸下，决定 506，10.1）。工作区搜索框（`search-box.tsx:91`）直接用 `useFocusOnInvalid`，它的失败总有 `aria-invalid` 的输入框，不受影响。
- **改法**（已定：2.3 的 P2-3）：
  - 规则：失败之后焦点给第一个 `aria-invalid` 的字段；没有时（只有横幅说），**拒绝到达时焦点已丢**（在 `body`，或在发送中禁用的元素上）就给横幅；焦点在别处的不动。加"已丢"是为了不推翻 M8：撤销的确认框用 `useForm`、按钮 `aria-disabled`，被拒之后焦点留在"撤销"（HI3、`revert-background.test.tsx:183`）。
  - "已丢"在**拒绝到达的那一刻**判：`submit` 的 `catch` 里，`finally` 的 `setSending(false)` 渲染之前。那时 Chromium 已把焦点交给 `body`（或答得太快，焦点仍在禁用的按钮上），jsdom 焦点在禁用的按钮上，两种都算"已丢"。到 effect 里再判，jsdom 里按钮已经回来，判不出。
  - `W/app/form.ts`：
    - 新状态 `lost`；`catch` 里 `setLost(focusLost())`，再 `setFailure`、`setFailures`；本地字段的失败 `setLost(false)`；
    - `useFocusOnInvalid(failures, lost)`；
    - 横幅的内容包一层 `createElement("div", { "data-form-banner": "", className: "contents" }, said)`。用 `div` 不用 `span`：内容可以是 `<p>`、`<ul>`（`pagesLocked`）；`contents` 不占盒子；
    - 注释（`:11-20`）补一句：只有横幅说的拒绝，焦点已丢时给横幅（M12/P2/S1）。
  - `W/components/form-field.tsx:83-97` 的 `useFocusOnInvalid(failures: number, lost = false): RefObject<HTMLFormElement | null>`，effect 的依赖是 `[failures, lost]`：
    - `failures === 0` 时什么也不做；
    - 有 `[aria-invalid="true"]` 就给它；
    - 否则 `lost` 时找 `form.current?.querySelector("[data-form-banner]")?.closest('[role="alert"]')`，命令式设 `tabIndex = -1` 再 `focus()`。React 不管 `Alert` 上它没渲染的属性；下一次失败是新元素，再设一次。
  - 取得焦点的横幅用全局的 `:focus-visible`，不加 `outline-none`（P2-3）。不改 `components/ui/alert.tsx`：全局让 Alert 可聚焦，波及面大。
  - 文字不变。
- **测试**：
  - 新 `W/app/form.test.tsx`（单元，一个小表单：字段 `name`、横幅、提交按钮 `disabled={sending}`；另一个变体的按钮 `aria-disabled`，照撤销的确认框）：
    1. `"a refusal only the banner says gives it the focus its disabled button dropped"`：以 `new ApiError(409, { code: "page.cycle" })` 之类拒绝（没有字段）；焦点是 `getByRole("alert")`，`tabIndex === -1`。**没修的代码上**：在提交按钮上（jsdom 不做焦点修正）。
    2. `"… the focus fallen to the body, as Chromium's fixup leaves it"`：扣住期间 `act(() => (document.activeElement as HTMLElement).blur())`；拒绝之后焦点在 alert。**没修的代码上**：`body`。
    3. `"a refusal the reader moved the focus away from meanwhile leaves it where they put it"`：扣住期间 `user.click` 表单外的输入框；拒绝之后焦点仍在那里。
    4. `"a refusal a field says gives the field the focus, not the banner"`：422 带字段错误；焦点在 `aria-invalid` 的字段。
    5. `"a refusal, its button aria-disabled and focused, leaves the focus on it"`：`aria-disabled` 的变体，焦点在按钮上（钉住 M8 的做法）。
  - 页面一级（en；文字不变，不另做 zh-CN 格）：
    - `W/pages/notebook/page-tree-writes.test.tsx:567` 的 `"a move refused stays in its dialog"`：末尾加 `await waitFor(() => expect(document.activeElement).toBe(within(dialog).getByRole("alert")))` 与 alert 的整句（`page.too_deep` 的文字）。**没修的代码上**：焦点在"Move"上。
    - 同文件新加 `"a rename refused for the pages its links lock says why above the field, which takes the focus"`：`answers` 覆盖 `PATCH /api/v0/nodes/*` 答 409 `linking.pages_locked`（带 `locks`）；焦点在 alert，alert 的整句。**没修的代码上**：在"Save"上。
    - `W/pages/notebook/source-add-dialog.test.tsx` 新加 `"pasted Markdown refused as a duplicate says which, the focus on what it says"`：照 `:158`、`:184` 的夹具造一份同字节的来源；焦点在 alert，链接的名字照 LW3（`raw/<…>.md`）。**没修的代码上**：在"Add source"上。
  - e2e：LW3（三种），见 4.1.0.3。**没修的代码上**：alert 不是焦点。
- **已有测试要随之改的**：没找到。核对过照旧的：`revert-background.test.tsx:183`（焦点在 `aria-disabled` 的"Undo in the background"，没丢）；`transfer-import.test.tsx:612`（导入对话框自己把焦点给"Import"，发送中焦点在"停止上传"上、没丢，`import-dialog.tsx:229-238`）；`upgrade-page.test.tsx:1231`（`ConfirmDialog`）；各处 alert 的 `textContent`（包的 `div` 不改文字）。
- **变异**：去掉 `:disabled` 的判断 → 第 1 格与三个页面格红；去掉 `body` 的判断 → 第 2 格红；不看 `lost`（无条件给横幅）→ 第 3、5 格红，HI3、`revert-background.test.tsx:183` 红；去掉 `data-form-banner` 的包 → 第 1、2 格红。
- **牵连**：改写 13.2 第 12 条（8.2）。

##### 到达的焦点（`W/app/arrival.ts`）：12.3、23.14、26.4、28.4、28.5

现有的规则（`arrival.ts:26-50,62-78`，13.2 第 26 条末段）：到达（`arrived` 或 `Located`）时，标题在这次导航第一次挂上时取焦点（不管焦点在哪）；之后再挂上只从无人处取。不是到达的导航，标题从不取。`offered`（`:24`）按 React Router 的 location 对象记"这次导航已挂过标题"。

改后的 `arrival.ts`（已定：2.3 的 P2-4），实施者照下面的规则写：

- `export const opened = { opened: true } as const`：左栏的树打开的页的路由状态（28.5）；`isOpened(state)` 照 `isArrival`。
- `export function useArrivalWatch(): void`，`W/app/layout.tsx:30` 的 `Layout` 开头调用一次（28.4）：
  - `useLayoutEffect`，依赖 `[location]`；到达（`isArrival(location.state)`）的导航提交时开一个 `watchReader()`，记在模块私有的 `watching: WeakMap<object, Reader>` 里，键是 location 对象，到下一次导航结束；
  - `Located` 不进这个看守：附件的行自己经 `useLocated` 看读者（`attachments-section.tsx:420-431`）；
  - 同一路径与查询的 `replace`（只换 state）不算新的导航，见 28.4 的规则；
  - `Layout` 是根路由的组件，每次导航都渲染，它的 layout effect 先于标题的被动 effect，同一次提交里看守已经在了。同一个路由器里 `useLocation()` 拿到同一个对象（`offered` 已经靠这一点）。
- `takeArrivalFocus` 的 `first` 改成 `!offered.has(navigation) && watching.get(navigation)?.acted() !== true`，其余照旧。
- `useArrivalFocus<T>(also = false, replacing = false)`：
  - `fromNoOne = !arrival && (replacing || (useNavigationType() === "POP" && isOpened(location.state)))`；
  - effect 的依赖 `[arrival, fromNoOne]`：到达就 `takeArrivalFocus`；否则 `offered.add(entry.current)`，`fromNoOne` 时再 `takeArrivalFocus`，这时已不是第一次，只从无人处取（WebKit 的滚动照旧补）。
- `arrival.ts` 从 `pages/page/readers-input.ts` 导入 `watchReader`。

###### 12.3 子页列表、面包屑的链接点击之后焦点落到 `body`

- **现状**：`W/pages/page/subpage-list.tsx:27` 与 `W/pages/page/breadcrumbs.tsx:54` 的 `<Link>` 不带 `state`；右栏的链接带 `arrived`（`page-backlinks.tsx:150`、`page-properties.tsx:268`）。点了之后旧页（含这个链接）卸下，焦点落到 `body`；新页不是到达，标题不取。
  - `SubpageList` 用在页面的子页（`page-layout.tsx:341`）、笔记本首页的根页（`notebook-home.tsx:66`）、标签页（`tag-page.tsx:54`）；
  - `BreadcrumbTrail` 用在页面、版本页（`version-page.tsx:129`）、变更集页（`changeset-page.tsx:198`）、回收站预览（`trash-preview.tsx:302`）；
  - 工作区首页的笔记本卡片（`W/pages/workspace/workspace-home.tsx:41`）同一种问题（已定：2.3 的 P2-5）；
  - 去处的标题都用 `useArrivalFocus`。
- **改法**：三处 `<Link>` 加 `state={arrived}`。注释各补一句：链接随它所在的页一起离开，去处是到达，标题取焦点（13.2 第 17 条；M5-polish 12.3）。文字不变。
- **测试**：vitest：
  - `W/pages/page/page-layout.test.tsx` 新加 `"a subpage's link and a breadcrumb's lead to a page arrived at: its title takes the focus the link took with it"`：在 Install 页 `user.click` 子页"Linux"，`await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Linux" })))`；再点面包屑"Guide"，Guide 的标题。**没修的代码上**：`body`（链接随页卸下，jsdom 移除元素时 `activeElement` 回到 `body`，推断）。
  - `W/pages/workspace/workspace-home.test.tsx` 新加 `"a notebook's card leads to its home arrived at: its title takes the focus"`：照 `:31` 的夹具点卡片"Plans"，笔记本首页的 `h1` 取得焦点。**没修的代码上**：`body`。
- **已有测试**：没找到断言"点子页链接之后焦点不动"的格。e2e：`tb1-trash.spec.ts:276` 点面包屑"Trash"之后回收站的 `h1` 取焦点，`:283` 之后断言 `h1` 取焦点，照旧过（推断）；`pg11-navigation.spec.ts:144` 之后开快速切换，照旧过（推断）。
- **变异**：去掉 `subpage-list.tsx` 的 `state` → 第一格第一步红；去掉 `breadcrumbs.tsx` 的 → 第二步红；去掉 `workspace-home.tsx` 的 → 卡片那一格红。

###### 26.4 "在哪里找"替换历史条目，丢掉它的 `arrived`

- **现状**：`W/pages/search/search-page.tsx:172-180` 的 `choose` 以 `setParams(next, { replace: true })` 换地址，没带状态，这一条历史的 `arrived` 没了；之后从别处后退回到它，结果页的标题不取焦点，焦点在 `body`。`:80` 只取了 `key`。
- **改法**：`:80` 改成 `const location = useLocation(); const entry = location.key;`；`:179` 改成 `setParams(next, { replace: true, state: location.state })`。在结果页换范围时焦点照旧留在选择上：`useArrivalFocus` 的 effect 只看 `arrival` 变没变（`true → true` 不重跑），标题也不重挂（`entry` 只进 SWR 的键）。这次 `replace` 换了查询，不是 28.4 说的"只换 state"，照新的导航算：看守重新起算；页没换、标题不重挂，已挂过标题的记录（`offered`）照样接续，不然之后挂上的"页面不存在"被当作第一次、从读者手里拿走焦点（S1 审查 B 的 M1，决定 507，10.1）。文字不变。
- **测试**：vitest，`W/app/arrival.test.tsx` 新加 `"Where to look keeps the arrival of its entry: Back to the results from a page gone to from them gives their title the focus"`：`home()`，搜索框里 `Install{Enter}`，结果页的标题取得焦点；`user.selectOptions` 选 Plans，焦点仍在选择上、`router.state.location.state` 等于 `arrived`；`router.navigate(pagePath(install.id))`；`router.navigate(-1)`；结果页的标题取得焦点。**没修的代码上**：`location.state` 是 `null`，后退之后焦点在 `body`。
- **变异**：去掉 `state: location.state` → 新格红。

###### 28.4 迟挂上的第一个标题从读者手里抢焦点

- **现状**：`W/app/arrival.ts:39-44`：第一次挂上不管焦点在哪都取。到达一页而它的标题迟挂上（`PageLayout` 在树读到之前只显示 `NotLoaded`，`page-layout.tsx:58-60`），读者其间在搜索框里打字，标题挂上时把焦点拿走，之后的按键落到标题上丢了。
- **"迟"的判据**：不用时间，用读者有没有动作（13.2 第 26 条）。
  - `Layout` 在每次到达的导航**提交时**开一个 `watchReader()`（滚轮、触摸的移动、按下指针、按键都算），到下一次导航；
  - 标题第一次挂上时，读者自提交起有过动作，就当作"之后再挂上"，只从无人处取；
  - 引起导航的那一下（点击或回车）在提交之前，不算；
  - 同一次提交里就挂上的标题（树已在缓存里）不可能有动作，照旧不管焦点在哪都取；
  - 读者滚了一下、而焦点已随旧页落到 `body` 的，标题照样从无人处取，这不是抢。
- **同一路径与查询的 `replace`（只换 state）不算新的导航：`useArrivalWatch` 的看守不重新起算，`offered` 接续。**
  - 判据：`useNavigationType() === "REPLACE"`，且 `pathname`、`search`、`hash` 都与上一个 location 相同。
  - 做法：`useArrivalWatch` 用一个 ref 记上一个 location。新的 location 合判据时，上一个的看守（有的话）记到新的 location 下、不结束；`offered` 里有上一个的，新的也加进去。否则结束上一个看守，到达的再开一个。`Layout` 卸下时结束。
  - 为什么：`replace` 产生新的 location 对象。不接续的话，看守从这次 `replace` 重新起算，之前的按键不算了；之后在这次导航里挂上的标题（"页面不存在"）会被当作第一次，从读者手里拿走焦点（K1-1 的回归）。
  - 4.6 的 21.3、22.2 的 `useRouteNotice` 就是这样一次 `replace`，靠这一条。
- **改法**：见上面的 `arrival.ts` 与 `layout.tsx` 的一行。文字不变。
- **测试**：vitest，`W/app/arrival.test.tsx`（用文件里的 `home()`、`shown()`、`deleted()`，`renderApp` 带 `Layout`）：
  1. `"a title that shows late for a page arrived at, the reader typing in the search box meanwhile, takes no focus: the keys stay in the box"`：`pageServer` 的 `answers` 覆盖 `` `GET /api/v0/notebooks/${notebookJSON.id}/nodes` `` 扣住；`router.navigate(pagePath(install.id), { state: arrived })`；`user.click(box)`、`user.keyboard("abc")`；放开；`findByRole("heading", { level: 1, name: "Install" })`；`user.keyboard("d")`；焦点在 `box`，值 `"abcd"`。**没修的代码上**：焦点在 Install 的标题，值是 `"abc"`。
  2. `"a title that shows late takes the focus wherever it is, the reader having done nothing"`（对照）：同样扣住，其间只 `act(() => box.focus())`（不发读者的事件）；放开之后焦点在标题。钉住判据是动作，不是迟。
  3. `"a replace of the entry's state alone is the same navigation: the keys typed before it still keep a late title from the box"`：同第 1 格，状态用 `{ ...arrived, said: 1 }`；打完 `"abc"` 之后 `act(() => router.navigate(pagePath(install.id), { replace: true, state: arrived }))`；放开之后焦点仍在 `box`，值 `"abcd"`。**没修的代码上**：焦点在标题（今天没有看守）。
  4. `"… nor is a heading that mounts after it the navigation's first: Page not found takes the focus only from no one"`：`shown({ ...arrived, said: 1 })`，标题取得焦点；`act(() => box.focus())` 移焦点，不发读者的事件（读屏移焦点正是这种情形）；同上 `replace`；`deleted(page)`；"Page not found"挂上之后焦点仍在 `box`。不用点击与打字：那样看守随 `replace` 接续、`acted()` 为真，`first` 本来就为假，不接续 `offered` 也照过（修订核对 A 的 M14）。**没修的代码上**：没有看守，"Page not found"当作第一次，取走焦点；只不接续 `offered` 时同样。
- **已有测试要随之改的**：没找到。
- **变异**：
  - `first` 不看 `watching` → 第 1 格红；
  - `useArrivalWatch` 不在 `Layout` 里调用 → 第 1 格红；
  - 看守改用 `useEffect` → 第 1 格红（推断：子先于父，标题的 effect 先跑，看守还没开）。实际：第 1 至 4 格照过（一次提交的 layout effect 全都先于 passive effect）；抓它的是 S1 审查 B 加的一格：只换 state、把树打开的页变成到达的 `replace`，焦点留在框里（10.1）；
  - 同一地址只换 state 的 `replace` 也重新起算看守 → 第 3 格红；
  - 不接续 `offered` → 第 4 格红。

###### 28.5 经左栏的树来到的页，后退离开再前进回到它，焦点落在 `body`

- **现状**：`W/pages/notebook/page-tree-item.tsx:120-130` 的 `NavLink` 不带状态。点开时焦点留在树上，这是对的（13.2 第 17 条"打开、切换进入的页面不抢焦点"）；但这一条历史没有标记，后退、前进回到它时标题不取，焦点在 `body`（前一页随焦点卸下）。树里"Directory""History"两行（`page-tree.tsx:150`、`:154`）同样（推断）。
- **改法**：`NavLink` 与那两个 `Link` 加 `state={opened}`；目录与历史的筛选换地址时带上这一条的状态（修订核对 A 的 M16）：`W/pages/notebook/index-page.tsx:56` 与 `history-page.tsx:134` 的 `setParams(…, { replace: true })` 不带 state，换一次筛选 `opened` 就没了，同 26.4，照它加 `state: location.state`（`index-page.tsx:40` 已有 `useLocation`，历史页加一个）。S6 的 26.5 也改历史的筛选（4.1.0.1）。`POP` 且状态是 `opened` 时，标题从无人处取。点开（`PUSH`）不取；后退时焦点在树的链接或搜索框上的，不动。刷新（也是 `POP`）一页树打开的页，标题也取，与 `arrived` 的页刷新时一样（13.2 第 17 条"返回或刷新时再聚焦一次，无害"）。左栏的笔记本列表等别的留下的链接不在内。
- **测试**：vitest，`W/app/arrival.test.tsx`：
  1. `"a page the tree opened is no arrival; Back to it from a page arrived at from it gives its title the focus from no one"`：`shown(arrived)`（Install），点树的"Notes"（焦点留在链接上，照 `:118-132` 断言）；`router.navigate(pagePath(linux.id), { state: arrived })`，Linux 的标题取得焦点；`router.navigate(-1)`；Notes 的标题取得焦点。**没修的代码上**：`body`。
  2. `"Back to a page the tree opened, the focus on the tree's link, leaves it there"`：点树的"Notes"、再点"Guide"（焦点在 Guide 的链接上），`router.navigate(-1)`，焦点仍在 Guide 的链接上。钉住"只从无人处"。
  3. `"the directory the tree opened keeps its mark through a filter: Back to it gives its title the focus"`：不用 `home()`：树里的"Directory""History"两行只在 `llm_wiki` 时才有（`page-tree.tsx:148`），`home()` 的 `pageServer` 缺省 `standard`（`W/test/page-server.ts:166`）；目录读的 `GET …/index` 也不在 `pageServer` 里（修订核对 A 的 N14）。这一格用 `pageServer({ mode: "llm_wiki", answers })`，`answers` 照 `W/test/index-server.ts` 的 `indexServer`（`:72` 起）给目录的路由（或让 `indexServer` 收 `mode`）；历史的路由 `pageServer` 已有。点树的"Directory"，换"类型"筛选（`user.selectOptions`），`router.state.location.state` 仍是 `opened`；`router.navigate(pagePath(linux.id), { state: arrived })`；`router.navigate(-1)`；目录的标题取得焦点。历史页的筛选同一格的 `test.each` 第二行。**没修的代码上**：换筛选之后 `state` 是 `null`，后退之后焦点在 `body`。
- **变异**：
  - 去掉 `isOpened` 的判断（任何 POP 都取）→ 两格照过；`renderApp` 的初始地址也是无状态的 POP，已有的"打开的页不抢"格（如 `trash-preview.test.tsx:209`）抓得到（推断）。实际：照过（那一页没有到达的标题）；抓它的是 S1 加的"先载入的页不取焦点"一格；
  - 去掉 `type === "POP"`（点开也取）→ `arrival.test.tsx:127` 的"焦点留在树的链接上"红。实际：照过（jsdom 里点过的链接拿着焦点）；抓它的是 S1 加的"点击把焦点留在 `body`（Safari 的样子）"一格；
  - 去掉 `page-tree-item.tsx` 的 `state` → 第 1 格红；
  - 筛选的 `setParams` 不带 `state` → 第 3 格红。

###### 23.14 撤销答 404、整页换成"页面不存在"时焦点落到 `body`

- **现状**：
  - `W/pages/notebook/revert-dialog.tsx:279-296` 的 `refused`：404 时重读变更集（`reread(true)`），再 `close(…, !reader.acted())`（`:295`）：读者在撤销的期间没有动作，焦点给变更集页的 `h1`。
  - 重读答 404，`W/pages/notebook/changeset-page.tsx:110-111` 整页换成 `NotFoundPage`，`h1` 随之卸下，焦点落到 `body`。
  - `NotFoundPage` 的 `useArrivalFocus`（`W/pages/not-found.tsx:20`）只在到达时取；从历史列表点进来的变更集页不是到达，不取。原因（"页面不存在"）听不到。
- **改法**：
  - `not-found.tsx:17`：`export function NotFoundPage({ replacing = false }: { replacing?: boolean })`；`:20` 改成 `useArrivalFocus<HTMLHeadingElement>(locatedOf(useLocation().state) !== undefined, replacing)`。注释补：代替这次导航自己的页时（撤销答 404 拿走变更集页），只从无人处取焦点（M12/P2/S1）。
  - `changeset-page.tsx:109-112`：`gone = error instanceof ApiError && error.status === 404`；`(data !== undefined && data.notebook_id !== notebook.id) || gone` 时 `<NotFoundPage replacing={gone && data !== undefined} />`。SWR 重读失败时保留上一次的 `data`：`data` 在，说明这一页显示过。
  - 焦点读得出原因：标题就是"Page not found"。不另把拒绝的说明留在前面。文字不变。
- **测试**：vitest，`W/pages/notebook/revert-dialog.test.tsx`：
  1. `:591` 的 `"an undo refused as not found closes the dialog, and reads the changeset and the notebooks again: gone, it is no page"`：`findByRole("heading", { name: "Page not found" })` 之后加 `await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Page not found" })))`。**没修的代码上**：`body`。
  2. 新加 `"an undo refused as not found, the reader in the search box once the dialog closed, keeps the focus there as the changeset read again is gone"`：
     - 照 `:591` 的夹具（`reworked`、删掉那个变更集）；打开对话框之后设 `server.historyWaits.changeset`（`W/test/history-server.ts:244`），扣住按下 Undo 之后的那次读（`reread(true)` 的重读）；
     - 按 Undo，撤销答 404，对话框关上，等焦点到变更集页的标题（`title()`，`close(…, !reader.acted())` 给的）；
     - `user.click(box)`（左栏的"Search terms"）；放开重读，答 404；
     - `findByRole("heading", { name: "Page not found" })`，稍等（照 `arrival.test.tsx` 的 `settle`）之后焦点仍在 `box`。
     - **没修的代码上**：照过（今天 `NotFoundPage` 不取）；它钉的是"只从无人处取"。
     - 不能扣撤销的答复：其间撤销的对话框是模态的，点搜索框报 `pointer-events: none`，`box.focus()` 被 Radix 的焦点范围拉回"Undoing…"（审查 A 的探针 `m12/p2-review-a/zz-probe-a1.test.tsx`，结果在 `m12/p2-review-a/probe-results.txt`）。
- **变异**：`changeset-page.tsx` 不传 `replacing` → 第 1 格红；`useArrivalFocus` 的 `replacing` 不经"已挂过"（当作第一次，不管焦点在哪都取）→ 第 2 格红。
- **假服务端**：夹具里删掉那个变更集，照 `:601`；第 2 格的扣住用已有的 `historyWaits.changeset`。

#### 4.1.2 第 7 条 退出登录时保存不了的文字随退出丢失（A，中）

- **现状**：
  - `W/app/user-menu.tsx:49`：`<DropdownMenuItem onSelect={() => void auth.signOut()}>`，不问。
  - `W/stores/auth.store.ts:44-47`：`signOut` 先 `beforeSignOut`（`RootStore.endEdits`，`root.store.ts:140`）再登出。
  - `W/stores/root.store.ts:207-216` 的 `endEdits`：本标签页的每个编辑 `close(signOutSave)`（1.5 秒，`:434`）；经 `BroadcastChannel("nwiki.edits")`（`:172`）请别的标签页关闭（`closeOtherTabs`，`W/stores/edit-closing.ts:55-97`）；整体至多 2 秒（`:433`）。
  - `root.store.ts:223-238` 的 `answerSignOuts`（`W/app/sign-out-elsewhere.tsx:13` 挂上）：别的标签页来请时，有编辑就答"closing"、关、答"closed"（`edit-closing.ts:20-48`）；答复是否有编辑只等 100 毫秒（`closingAnswerWait`，`:12`）。
  - `root.store.ts:183-198` 已有 `unsavedEdit({...})`：本标签页有没有未保存的编辑。
  - 另两处退出：`W/pages/invitation.tsx:167`（邀请页）、`W/app/session-unavailable.tsx:25`（会话一时不能用时）。
  - 丢文字的情形（移交第 7 条）：冲突、已失锁、保存没成功或 1.5 秒内没答、主线程被占住 100 毫秒以上的别的标签页。Web Locks 的登记不做（进 v0.2）。
- **改法**（范围已定：2.3 的 P2-6）：
  - **在哪里判断**：退出的按钮按下时、在界面一层（新的 hook），不在 `AuthStore.signOut` 里：`signOut` 照旧由确认之后调用，store 的测试（`root.store.test.ts:432-560`）不变。
  - **别的标签页怎样报告**：同一个频道 `nwiki.edits` 加一问一答，在 `W/stores/edit-closing.ts`：
    - `Message` 的 `kind` 加 `"ask" | "unsaved"`（`:9`），`messageOf`（`:105-113`）认它们；
    - `answerClosings` 加第五个参数 `unsaved: () => boolean`：收到同一登录、别的标签页的 `ask`，且 `unsaved()` 为真时答 `unsaved`（同一个 id）；
    - 新 `askOtherTabs(port: Port, { loginId, tabId }: { loginId: string; tabId: string }, id: string): Promise<boolean>`：发 `ask`；第一个同 id 的 `unsaved` 一到就答 `true`，不等满 100 毫秒（C6 的时钟停着，也靠这一点）；`closingAnswerWait` 到了答 `false`；两种都撤掉监听与定时器。主线程被占住更久的标签页不答，等它们是 Web Locks 的事（v0.2）。
  - `W/stores/root.store.ts`：
    - `answerSignOuts`（`:229-234`）给 `answerClosings` 传 `() => this.unsavedEdit({})`；
    - 新 `unsavedEdits(): Promise<"here" | "elsewhere" | undefined>`：本标签页的 `unsavedEdit({})` 先，有就 `"here"`；没有频道（`this.closing` 为空，测试的 `testApp` 没有 `EventDeps`）就 `undefined`；否则开一个端口，`askOtherTabs(port, tabs, id)`（id 是 `tabId` 加 `this.closings` 递增的序号），答真就 `"elsewhere"`；`finally` 关端口。
  - 新 `W/app/sign-out.tsx` 的 `useSignOut(stayed: () => void): { signOut: () => void; dialog: ReactNode }`：
    - `signOut()`：一个 ref 守着，问别的标签页期间再按什么也不做；`store.unsavedEdits()` 答 `undefined` 就 `store.auth.signOut()`，否则开对话框并记下 `here`/`elsewhere`；已卸下就什么也不做；
    - `dialog`：`ConfirmDialog` 以 `held` 开，`onClosed: (done) => done || stayed()`；标题 `signOut.unsavedTitle`，描述按 `here`/`elsewhere` 取两句之一，确认 `signOut.confirm`，发送中 `signOut.signingOut`，取消 `editor.stay`，`confirm={() => store.auth.signOut()}`；
    - 语气 `destructive`（默认），同离开编辑的提醒的"Leave"（`unsaved-guard.tsx:72`）。退出成功之后这一代卸下，对话框随之没了。
  - `W/app/user-menu.tsx`：`useSignOut(() => trigger.current?.focus())` 放在 `if (me === undefined) return null` 之前（hook 的规则）；`<Button ref={trigger} variant="ghost">`；`:49` 改成 `<DropdownMenuItem onSelect={signOut}>`；返回 `<>{menu}{dialog}</>`。从菜单项开对话框照 `page-menu.tsx:55-71`（导出）。
  - `W/pages/invitation.tsx:167`：`onClick={signOut}`，`{dialog}` 放在 `<form>` 之外；`stayed` 把焦点还给这个按钮（加一个 ref）。`W/app/session-unavailable.tsx:25` 同。
  - 对话框的文字（两种语言）见 4.1.0.2。
  - 焦点：对话框开时 Radix 给"留下"（`AlertDialogCancel`）；留下时回到用户菜单的按钮；退出时整代卸下，去登录页（守卫）。
- **测试**：
  - `W/stores/edit-closing.test.ts`（假定时器、`FakeChannels`，照文件里的 `tab()`，`tab()` 加一个 `unsaved` 选项）：
    1. `"asked whether one has an edit not saved, a tab that has one says so, and the asking tab hears it before the answers' wait is out"`：`askOtherTabs(...)` 在 `advanceTimersByTimeAsync(0)` 之后已答 `true`。
    2. `"a tab without changes not saved says nothing to an ask: it resolves false once closingAnswerWait is out"`：`closingAnswerWait - 1` 时未答、到点答 `false`。
    3. `"a tab answers its own login's other tabs' asks only: not itself, nor another login's"`。
    4. `"a tab unsubscribed answers no ask"`。
  - `W/stores/root.store.test.ts` 新加 `"unsavedEdits tells this tab's edit with changes not saved first, then another tab's of the login, else none"`：照 `:496-560` 的两代 `RootStore`（同一个 `FakeChannels`）。
  - `W/app/user-menu.test.tsx`：
    5. `"signing out while another tab of the login has an edit not saved asks first, saying it is in another tab; Stay keeps the session and gives the focus back to the menu's button"`：`withEvents(signedInApp(...), page, channels)`，另开一个端口以 `answerClosings(..., () => true)` 作"别的标签页"；点"Ada"、点"Sign out"；`findByRole("alertdialog", { name: "Sign out anyway?", description: <signOut.unsavedElsewhere 的整句> })`；点"Stay"；焦点在"Ada"按钮，`sent` 里没有 logout，会话仍是 `signed-in`。**没修的代码上**：没有对话框，`sent` 有 logout，`findByRole` 超时红。
    6. 同上，确认"Sign out"：去登录页，`sent` 只有一次 logout。
    7. zh-CN 一格：`app.preferences.setLocale("zh-CN")`，对话框名"仍要退出登录吗？"、描述整句（`signOut.unsavedElsewhere` 的中文）。
    8. 已有 `"signing out from the user menu ends the session and goes to sign in"`（`:8`）：没有未保存的编辑时不问。实际：退出之后整代卸下，`queryByRole("alertdialog")` 恒为空，是空断言，修复第 1 轮去掉；守它的是走到了登录页（S1 审查 A 的 N1）。
  - `W/pages/page/page-lock.test.tsx` 新加 `"signing out with what was typed not saved asks first, saying it is in this tab: Stay keeps the editor and the session; Sign out then saves it and ends the session before the logout"`：`type(" more")`，点"Ada"、"Sign out"；描述是 `signOut.unsavedHere` 的整句；"Stay"之后编辑器仍可写、`server.sent` 没有 `PUT`；再走一遍点"Sign out"：`PUT`、`END`、登录页。**没修的代码上**：没有对话框。修复第 1 轮（S1 审查 C 的 M2，7.3）：这一格扣住 logout 的答复，断言确认键是"Signing out…"、`aria-busy` 与 `aria-disabled`；另加中文一格：本标签页的描述整句与"正在退出登录…"。
  - 邀请页与会话不可用页（修复第 1 轮，S1 审查 A 的 M2、C 的 M1；原表漏了）：`W/pages/invitation.test.tsx` 两格（另一个标签页有未保存的编辑时先问，"Stay"之后焦点回到"Sign out"；问的期间连按两下只问一次、只退出一次），`W/app/guards.test.tsx` 一格（会话不可用时同样先问）。
  - e2e：C6（Chromium），见 4.1.0.3。
- **已有测试要随之改的**：
  - `W/pages/page/page-lock.test.tsx:127`、`:141`、`:199`：三格都先 `type(" more")` 再退出，现在会先问；点菜单项之后加 `await user.click(within(await screen.findByRole("alertdialog", { name: "Sign out anyway?" })).getByRole("button", { name: "Sign out" }))`。`:199` 用假定时器（`shouldAdvanceTime: true`），本标签页有未保存的编辑，不问别的标签页，照旧推进即可（推断）。
  - 不改的：`:97`、`:170`（没有未保存的，不问；`:170` 正好钉住"开着编辑但没有未保存的不问"）；`W/app/guards.test.tsx:167`、`W/pages/invitation.test.tsx:188,213`、`W/pages/notebook/source-uploads.test.tsx:507`（`testApp` 没有 `EventDeps`，不问别的标签页，不多等，推断）。
  - e2e：`c6-expired.spec.ts:83`（见 4.1.0.3）。`a6-sign-out.spec.ts`、`c8-stream-life.spec.ts:229`、`w6-accept.spec.ts:147`：没有未保存的编辑，不问，只多等至多 100 毫秒。
- **变异**：
  - `askOtherTabs` 恒答 `false` → 第 5、6 格与 C6 红。
  - `answerClosings` 不看 `unsaved()` 就答 → 第 2 格红。
  - `useSignOut` 不问直接退出 → 第 5 格与 `page-lock` 新格红。
  - `unsavedEdits` 改看 `this.edits.size > 0`（开着就算）→ `page-lock.test.tsx:170` 红（它不点确认）。
  - `onClosed` 不还焦点 → 第 5 格的焦点断言红。
  - 修复第 1 轮加：邀请页、会话不可用页去掉 `{dialog}` 或改回立即退出 → 各自的新格红；去掉 `useSignOut` 的 `asking` 守卫 → 邀请页"连按两下"一格红；`signOut.signingOut`、中文 `signOut.unsavedHere` 改字 → `page-lock` 的格红。
- **假服务端**：`W/test/event-server.ts:59` 的 `withEvents(app, page = new FakePage())` 加第三个可选参数 `channels = new FakeChannels()`，测试拿同一个 `FakeChannels` 开"别的标签页"的端口。
- **牵连**：13.2 第 1、21 条的改写（8.2）；用 `ConfirmDialog`（23.16 之后发送中是 `aria-disabled`）。

#### 4.1.3 其余各项

##### 6.3 阅读者禁用的复选框没有名称

- **现状**：`W/reading/task-toggle.ts:14-18`：没有 `toggleTask`（读者、版本页、回收站预览）就直接返回，复选框保持服务端写的 `disabled`、没有名称；有的才在 `:32-38` 加 `aria-label`。服务端（`server/internal/platform/markdown/tasks/tasks.go:116-121`）不写名称。读屏念"复选框，未选中，不可用"，不说是哪一项。
- **改法**：在判断 `toggleTask` 之前给每个 `input[data-task]` 设 `aria-label`（`taskText(box)`，空的不设）。没有框时返回 `undefined`；没有 `toggleTask` 时返回一个只去掉名称的清理；有时照旧启用、监听，清理里另去掉名称。注释（`:3-13`）末句改为"A reader's checkboxes stay disabled, named all the same."。文字不变（名称是项的文字）。
- **测试**：vitest，`W/reading/task-toggle.test.ts:153` 改名为 `"without a toggle, a reader's, the checkboxes stay disabled, named by their items, and a click asks nothing"`：`const undo = taskToggle(...)`；`[a.getAttribute("aria-label"), b.getAttribute("aria-label")]` 等于 `["a", "b"]`；`undo?.()` 之后名称去掉。**没修的代码上**：`[null, null]`，且返回 `undefined`。这一格原来断言 `toBeUndefined()`，随之改。
- **变异**：命名挪回 `toggleTask` 判断之后 → 红；读者的清理不去名称 → 红。
- **牵连**：S4 改写阅读视图的 effect，可能把启用、禁用挪出 `taskToggle`（4.4.2 的 U7）；命名留在这里，S4 在 S1 之后开（2.3 的 P2-1）。

##### 11.7 未建链接：`aria-busy`、关闭后的焦点、按住 Enter

- **现状**：
  - (a) `W/pages/page/unresolved-link.tsx:147-148`：问服务端落点时在**那个元素**上 `aria-busy`，`:176-178` 去掉。其间视图被重读（`innerHTML` 整段换，`reading-view.tsx:138`），新的那条链接没有 `aria-busy`。
  - (b) `:199-206` 的 `giveBack`：元素不在了就找"同一目标的未建链接里同一位置的"（`sameTarget`，`:262-266`，只认 `a.nw-unresolved[data-nw-target]`）；链接因重读已解析（服务端写成 `data-nw-node`，`server/internal/platform/markdown/obsidian/view.go:84-97`），找不到，焦点落在文章上。
  - (c) `W/reading/unresolved-links.ts:46`：`keydown` 的 Enter 每次都 `act`，按住 Enter 的连发反复开对话框。
- **改法**（已定：2.3 的 P2-7，不依赖 S4）：
  - (a) `unresolved()` 问的期间以 `MutationObserver` 看 `article.current`（`childList`、`subtree`）；变了就给 `sameTarget(article.current, link.target)[at]` 加 `aria-busy="true"`；`finally` 里断开，去掉 `link.element` 与 `sameTarget(...)[at]` 两者的 `aria-busy`。
  - (b) `Asked` 记下链接的文字与它在"同样文字的链接"里的位置（`linksOfText(container, text)`：`container` 里 `textContent === text` 的 `a`）；`giveBack` 的去处：元素还在就是它，否则 `sameTarget(container, target)[at] ?? linksOfText(container, text)[textAt] ?? container`。
  - (c) `unresolved-links.ts:46` 加 `!event.repeat`（照 `page-layout.tsx:236` 的 Mod+E）。
  - 文字不变。
- **测试**：vitest：
  - (a) `W/pages/page/unresolved-link.test.tsx:272` 之后新加 `"the link read again while the server answers is busy too, and neither is once it answers"`：扣住 `server.landings` 的 x；照 `:296-305` 的 `readAgain` 重读同样的 HTML；新的 `button("x")` 有 `aria-busy="true"`；放开之后没有。**没修的代码上**：新的那条没有。
  - (b) `:295` 那格加第三段：重读为 `<p><a data-nw-node="${notes.id}">two</a></p>`（已解析），"Cancel"之后焦点在 `getByRole("link", { name: "two" })`。**没修的代码上**：在文章上。
  - (c) `W/reading/unresolved-links.test.ts:114` 附近新加 `"Enter held down acts once: its repeats hand nothing"`：`fireEvent.keyDown(link, { key: "Enter", repeat: true })` 不调用 `unresolved`。**没修的代码上**：调用了。
- **变异**：去掉观察者 → (a) 红；去掉文字的回退 → (b) 红；去掉 `!event.repeat` → (c) 红。
- **牵连**：S4 的就地更新让多数情形不再发生，S4 的测试照过这几格（4.4.2）。

##### 35.2 LLM Wiki 里导向目录或历史的未建链接带 `aria-haspopup="dialog"`

- **现状**：`W/reading/unresolved-links.ts:21-25` 给每个未建链接 `role="button"`、`aria-haspopup="dialog"`；而 `W/pages/page/unresolved-link.tsx:125-131` 在 LLM Wiki 里对 `index`、`log`（`virtualTarget`，`virtual-target.ts:8-39`）是导航，不开对话框。增强不知道模式。
- **改法**：
  - `W/reading/enhancement.ts` 的 `ReadingContext` 加可选字段 `leads?: (target: string) => boolean`：对这个目标的未建链接动作是去笔记本的一页（LLM Wiki 的目录或历史，M10 设计 4.9），而不是开对话框；这样的链接是链接，不是开对话框的按钮。
  - `unresolved-links.ts:21-25`：普通链接（`kindOf(link) === "link"`）且 `leads?.(target) === true` 的 `role="link"`、不设 `aria-haspopup`，空格对它不起作用（用一个 `Set` 记哪些是按钮）；嵌入与图片照旧是按钮（M10 的 `virtual-navigation` 守格要求，实际见 10.1）；其余照旧。清理照旧去掉三个属性。
  - `unresolved-link.tsx`：`useUnresolvedLinks` 多返回 `leads: (target) => notebook.mode === "llm_wiki" && virtualTarget(target, pages.nodeAncestors(page)?.length) !== undefined`，`unresolved()` 的 `:125-131` 也用它（一处判断）。
  - `reading-view.tsx:173` 与 `version-view.tsx:101` 的上下文加 `leads: (target) => latestLeads.current(target)`（照 `latestUnresolved` 用 ref，不进 effect 的依赖）。
  - 文字不变。
- **测试**：vitest：
  - `W/reading/unresolved-links.test.ts` 新加 `"a link the view goes to a page by, an LLM Wiki's directory, is a link: no dialog pops up from it, and Space is the page's"`：上下文 `leads: (target) => target === "index"`；`index` 的链接 `role="link"`、没有 `aria-haspopup`；`x` 的照旧。**没修的代码上**：`index` 也是 `button` + `dialog`。
  - `W/pages/page/unresolved-link.test.tsx` 新加 `"in an LLM Wiki, a link to its directory is a link that goes there"`：`pageServer({ mode: "llm_wiki", … })`，`[[index]]`：`getByRole("link", { name: "index" })`，没有 `aria-haspopup`，点了去目录。**没修的代码上**：`getByRole("link")` 找不到。
  - 实际多改的（10.1）：`unresolved-links.test.ts` 加"嵌入与图片照旧是按钮"一格；`W/pages/page/virtual-navigation.test.tsx` 四格与 e2e LW10 (page)（三种）原先按按钮找这些链接，改按链接找，LW10 另断言没有 `aria-haspopup`。
- **变异**：`leads` 恒 `false` → 两格红；`role` 仍写 `button` → 第一格红。
- **牵连**：S4 改写同一个上下文所在的 effect。模式在显示中改变（升级批准）时：S4 会订阅模式，模式变了就撤掉增强再重跑、不动 HTML（4.4.2 的"笔记本的模式变了"一条）。

##### 16.1 菜单开着时它的行被重读拿走，焦点丢失

- **现状**：`W/pages/page/attachment-row.tsx:84-150`：行没有 `useFocusLeaving`；菜单的内容是 portal（`components/ui/dropdown-menu.tsx:11-24`），不在 `<li>` 里。附件在别处被删、移走，列表重读（`attachments-section.tsx:263-276`）拿走这一行：焦点在菜单项上时随内容卸下，落到 `body`；Radix 在 `setTimeout(0)` 里想把焦点还给已卸下的触发按钮，什么也不做（推断）。焦点在行的"Actions"按钮上时同样。
- **改法**：
  - `AttachmentRow` 加 prop `left: () => void`：行离开文档、焦点在它或它的菜单里时，焦点去的地方。`attachments-section.tsx:265` 传 `left={() => heading.current?.focus()}`（13.2 第 17 条"交给行或这一节的标题"；与 `held` 的 `onClosed` 的回退一致）。
  - 行里：`row = useRef<HTMLLIElement>`；`menu` 由 `DropdownMenuContent` 的回调 ref 记下（只记非空，已关的内容 `isConnected` 为假）；`left` 经 ref 取最新的。
  - 一个依赖为空的 `useLayoutEffect` 的清理记下焦点在不在行或菜单里（`row` 含 `document.activeElement`，或 `menu` 仍在文档里且含它）；一个依赖为空的 `useEffect` 的清理（节点已离开文档，菜单的 `FocusScope` 已拆）在记下为真、焦点已丢（`focusLost()`）时调 `left()`。只在 layout 清理里给，菜单开着时焦点被 `FocusScope` 拉回将卸下的菜单项、随后落到 `body`（实际见 10.1）。
  - 不用 `useFocusLeaving`：它在挂上时取 `ref.current`，那时菜单没开（`lib/focus-leaving.ts:15-22`）。
  - **推断**（审查 A 的未核实的怀疑第 2 条）：照 React 19 的删除次序，删除一个函数组件时先跑它的 layout effect 的清理，再往下移除宿主节点；portal 在它的子树里，也在之后移除。所以清理时 `<li>` 与菜单都还在文档里，焦点也还在里面。没有跑过；实施时先用下面的测试证实（在没改的树上先看它红在 `body`，加清理之后看它绿）。不成立就改由这一节记下最后取得焦点的行（`focusin`），列表重读之后焦点在 `body`、那一行已不在时给标题。实际：清理次序成立；在 layout 清理里给不成立，见上一条。
  - 文字不变。
- **测试**：vitest，`W/pages/page/attachments-section.test.tsx` 新加：
  - `test.each` `"a row taken away by the list read again %s gives the focus to the section's title"`，`["its menu open", "on its Actions"]`：打开菜单（焦点在菜单项上）或只聚焦"Actions for photo.png"；`server.nodes` 去掉它；`readAgain()`（`W/test/page-panel.ts:34`）；`waitFor(() => expect(document.activeElement).toBe(within(section).getByRole("heading", { name: "Attachments" })))`。**没修的代码上**：`body`。
  - `"… the focus elsewhere stays there"`：焦点在搜索框，行被拿走，焦点仍在框里。
- **变异**：只看行、不看菜单 → "menu open"格红；去掉整段 → 两格红；不看 `contains`（总是给标题）→ 第三格红。实施另加：在 layout 清理里就给 → "menu open"格红。

##### 18.6 没有"编辑"可接焦点时焦点落在 `body`；`back` 留着

- **现状**：`W/pages/page/page-layout.tsx:226-243` 的 effect：`if (!editable || !reading) return`（`:227-229`）在处理 `back`（`:230-236`）之前。
  - 编辑中被降为读者，"完成"或"回到阅读"之后 `editable` 为假，直接返回：焦点（编辑器随之卸下）在 `body`，`back` 留着；之后升回写者，`editable` 变真，effect 跑，`back` 仍为 `true`，"编辑"把焦点从读者手里拿走。
  - 页面被删之后（失锁的横幅、"回到阅读"并确认）整页换成 `PageGone` → `NotFoundPage`（`:63-65`、`:76-81`），不是到达的导航不取焦点，落在 `body`。
- **改法**：
  - `page-layout.tsx:225-243` 拆成两个 effect：
    - `back` 的，依赖 `[reading, editable]`：`reading` 且 `back.current !== false` 时，`back.current === true` 或焦点为空、为 `body`，就把焦点给"编辑"（`editable` 时）或页标题（不然）；然后 `back.current = false`。`back` 在阅读视图显示时就处理掉，不留给之后才出现的"编辑"。
    - Mod+E 的监听：条件与依赖照旧（`[editable, reading, pages, page.id]`）。
  - 页面被删的一半：`:64` 改成 `<PageGone notebook={notebook} page={pageId} replacing={last?.id === pageId} />`；`PageGone`（`:76-86`）加 `replacing?: boolean`，转给 `<NotFoundPage replacing={replacing} />`（4.1.1 的 `arrival.ts`）。`version-page.tsx:61` 不传（没有 `last`）。
  - 文字不变。
- **测试**：vitest：
  - `W/pages/page/page-lock.test.tsx:496`（"a page deleted while this tab edits it unsaved stays…"）末尾加 `await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("heading", { level: 1, name: "Page not found" })))`；`:520`（"… with nothing unsaved is no page at once"）同。**没修的代码上**：`body`。
  - `W/pages/page/page-edit.test.tsx` 新加 `"Done, the account made a reader meanwhile, gives the focus to the page's title; made a writer again, Edit does not take it"`：照 `source-attachments.test.tsx:21-40` 的 `renderAttachments`（拿到 store、`refreshNotebook`）；编辑中把 `server.notebook.role` 改成 `reader` 再重读笔记本列表；点"Done"；焦点在页标题；改回 `editor`、重读；焦点仍在页标题。**没修的代码上**：第一处是 `body`，第二处是"Edit"。
- **已有测试**："Done 之后焦点在 Edit"的格照旧（如 `page-edit.test.tsx:45`、`page-autosave.test.tsx:152`）。
- **变异**：`back` 的处理挪回 `editable` 判断之后 → `page-edit` 新格红；不传 `replacing` → `page-lock` 两格红；没有 Edit 时不给标题 → `page-edit` 新格第一处红。
- **牵连**：依赖 4.1.1 的 `replacing`（6.1 的第 5 个提交在第 3 个之后）。空闲退出照旧，见 4.1.4。

##### 19.3 读不到的报告各有同名的"重试"

- **现状**：`W/pages/notebook/transfer-report.tsx:245-246` 的 `<NotLoaded error={error} retry={...} />`；`W/app/not-loaded.tsx:21-23` 的按钮只叫"Try again"。几份展开的报告都读不到时各行一个同名的按钮（13.2 第 17 条：行内控件要带上对象）。`TransferReport`（`:203`）只拿 `job`，不拿行的名字。
- **改法**：
  - `not-loaded.tsx:12`：`NotLoaded` 加可选 prop `retryLabel?: string`（几个同时显示时按钮的名字，以它的可见文字开头），按钮加 `aria-label={retryLabel}`。
  - `transfer-report.tsx:203` 加 prop `name: string`（行的名字，`transfer-names` 给的）；`:246` 传 `retryLabel={t("transfer.reportRetryOf", { name })}`。
  - `transfer-job-row.tsx:203` 传 `name={name}`。
  - 新键 `transfer.reportRetryOf`（4.1.0.2）。
- **测试**：vitest，`W/pages/notebook/transfer-page.test.tsx:352`（"a report whose problems cannot be read says why, its counts shown; Try again reads it"）：
  - `:374` 改按新名字找：`within(job).getByRole("button", { name: "Try again (report on <行名>)" })`（行名照 `jobJSON(1)` 的整句）；
  - 另加 `test.each` 两种语言：两个结束的任务都展开、报告都读不到，两个按钮的名字整句比对（en："Try again (report on …)"；zh-CN："重试：… 的报告"）。
  - **没修的代码上**：名字都是"Try again"/"重试"。
- **已有测试要随之改的**：`transfer-page.test.tsx:374`（按旧名字找，会找不到）。不受影响的：`:392`、`:404`（列表的 `NotLoaded`）、`transfer-import.test.tsx:512,659`、`trash-page.test.tsx:215`、`trash-preview.test.tsx:169`。
- **变异**：不传 `retryLabel` → 新格与 `:374` 红；`NotLoaded` 不用 `retryLabel` → 同。
- **假服务端**：`jobsServer` 的 `answers` 覆盖，照 `:355-363`。
- **牵连**：P3/S3 改同一个文件 `transfer-report.tsx:92-108` 的 `problems` 与它的测试（P3 设计 4.2）；文本冲突、语义不冲突，后合并的先 `git merge main`。

##### 21.14 冲突面板的附件块"重试"把焦点掉到 `body`

- **现状**：`W/pages/page/conflict-panel.tsx:50-63`："Try again"先 `setFailed(false)`，失败的一块换成 `<Loading/>`，按钮随之卸下，焦点落到 `body`。版本页、还原的比较已照 `comparison.tsx:84-117` 的 `ChunkNotLoaded` 做对（加载中 `aria-disabled`、留着；加载好了焦点给比较）。
- **改法**：
  - `W/pages/page/comparison.tsx:91`：导出 `ChunkNotLoaded`，加两个可选 prop：`text?: string`（失败时的说明，默认 `t("version.diffNotLoaded")`）、`reload = true`（是否给"Reload"）。
  - `conflict-panel.tsx`：`failed` 改成 `"failed" | "loading" | undefined`（照 `comparison.tsx:38`）；失败一支用 `<ChunkNotLoaded loading={failed === "loading"} retry={() => { setFailed("loading"); void load(); }} left={() => heading.current?.focus()} text={t("editor.conflictDiffNotLoaded")} reload={false} />`；`load` 的失败设 `"failed"`。`heading` 的类型由 `Ref<HTMLHeadingElement>` 改成 `RefObject<HTMLHeadingElement | null>`（`page-edit.tsx:100` 给的就是 `useRef`）。
  - 不给"Reload"：冲突时编辑器里有未保存的文字，重新加载会丢。
  - 文字不变（加载中用 `status.loading`）。
- **测试**：vitest，`W/pages/page/conflict.test.tsx:110` 的 `"differences that cannot load say so; the buttons still work, and Try again shows them"` 扩写：照 `version-page.test.tsx:320-355` 用 `vi.doMock` 让第二次加载先扣住再失败。点"Try again"，alert 是"Loading…"，按钮 `aria-disabled="true"`、焦点仍在它上面；放开（失败），焦点仍在"Try again"；再解开 mock，点"Try again"，比较出来之后焦点在冲突面板的标题。**没修的代码上**：点了之后焦点是 `body`。
- **已有测试**：`version-page.test.tsx:320-355`、`version-restore.test.tsx:252,277` 的 `ChunkNotLoaded` 行为不变（默认 `text`、`reload`）。
- **变异**：`retry` 里改回 `setFailed(undefined)` → 红；去掉 `left` → 最后一步红。

##### 22.17 接管无主笔记本之后焦点不看读者

- **现状**：`W/pages/workspace/ownerless-page.tsx:76-85` 的 `takeOver`：成功后 `heading.current?.focus()`（`:82`）；`notebook.not_found` 时 `refused` 里同样（`:71`）。等答复期间读者在别处（搜索框）打字，焦点被拿走。
- **改法**：照回收站的恢复（`W/pages/notebook/trash-page.tsx:114-170` 的 `toTitle`）：
  - `takeOver` 开头记下 `from = document.activeElement`，开 `watchReader({ on: from === document.body ? null : from })`，`finally` 里结束；
  - 成功之后与答 `notebook.not_found` 之后调 `toHeading(from, reader)`：读者没有动作，且焦点在 `from`、为空、在 `body` 或在标题上，才把焦点给标题；
  - `refused`（`:66-74`）去掉 `heading.current?.focus()`（删除的那条路由对话框的 `focusAfter={removed}` 管，`:137`）。
  - "接管"仍 `disabled={sending}`（`ownerless-row.tsx:87`）：Chromium 里焦点在 `body`，jsdom 里在 `from`，两种都合条件。
  - 文字不变。
- **测试**：vitest，`W/pages/workspace/ownerless-page.test.tsx` 新加 `"a take-over answered as the reader types in the search box leaves the focus and the keys there"`：扣住接管的答复（`ownerlessServer` 的 `answers` 覆盖）；点"Take over…"；`user.click(searchBox)`（工作区设置页左栏的搜索框，`workspace-layout.tsx:115`）、`user.keyboard("ab")`；放开；`user.keyboard("c")`；焦点在框里、值 `"abc"`；状态行仍说接管了。**没修的代码上**：焦点在标题，值 `"ab"`。已有 `:204`（不动时标题取焦点）是对照，与 `:314` 照旧过。
- **变异**：去掉 `watchReader` → 新格红；`toHeading` 不看焦点在哪 → 新格红（推断：读者点框时焦点已在框里）。实际：去掉 `watchReader` 时新格照过（读者在框里时，焦点的判断本身就够）；抓它的是 S1 加的"读者滚动过、焦点仍在 Take over"一格（10.1）。
- **牵连**：13.2 第 26 条的改写（8.2）。

##### 24.3、49 撤销对话框里两句说明不在可访问描述里

- **现状**：`W/pages/notebook/revert-dialog.tsx`：
  - 49：`:327-329` 的 `AlertDialogDescription` 只有 `history.revertBody`；`:331` 的 `history.revertLinks`（改名、移动之后新写的链接不会跟着改回）是普通 `<p>`，确认框打开时读屏不念。
  - 24.3：后台模式的 `TooLarge`（`:366-382`）把 `h3` 当描述（`AlertDialogDescription asChild`，`:374-378`），问的那一句（`history.backgroundAsk`，`:379`）不在描述里；`h3` 取得焦点时只念它。
- **改法**：
  - 表单里（`:323-331`）`<AlertDialogDescription asChild>` 包一个 `div.space-y-4`，里面是 `TooLarge`（后台时）或 `revertBody` 的 `<p>`，再加有改名、移动时的 `revertLinks` 的 `<p>`（照 `ConfirmDialog` 的 `more`，`confirm-dialog.tsx:150-156`）。
  - `TooLarge` 不再自己是描述：`h3` 保留 `ref={title}`、`tabIndex={-1}`，加 `aria-describedby={askId}`（`useId()`），问的那句是 `<p id={askId}>`。对话框的描述含两句；取得焦点的 `h3` 另以问的那句为描述，因为焦点移进已开的对话框时读屏不重念对话框的描述。
  - 文字不变。
- **测试**：vitest：
  - 49：`revert-dialog.test.tsx:54`（"Undo change asks first…"）的 `:61-64` 改成 `expect(screen.getByRole("alertdialog", { name: "Undo this change?", description: "Nodes: 1, contents: 1. They go back … and nothing changes. Links written since to the new names or places are not changed back." })).toBeTruthy()`（整句）。**没修的代码上**：描述只有前一句。
  - 24.3：`revert-background.test.tsx` 的 too-large 一格（`:66`）的 `:78` 改成对话框的描述是"This change is too large … Nodes it touched: 1,000+. Undo it in the background? … what it undid stays. Links written since …"（夹具改名过，`large()` 的 `renamed(notes, …)`）；`h3` 以 `{ description: "Undo it in the background? An undo in the background goes in batches: stopped on the way, what it undid stays." }` 找得到。**没修的代码上**：`aria-describedby` 指 `h3`，描述只有它的文字，`h3` 没有描述。
  - 文字不变，不另做 zh-CN 格。
- **已有测试要随之改的**：`revert-dialog.test.tsx:61-63`（按 id 取描述元素的 `textContent` 只比 `revertBody`）、`revert-background.test.tsx:78`（`aria-describedby` 等于 `heading.id`）。
- **变异**：`revertLinks` 挪回描述之外 → 49 的格红；问的那句挪回描述之外 → 24.3 的格红；去掉 `h3` 的 `aria-describedby` → 24.3 的 `h3` 一步红。

##### 38.1 来源页的只读说明是红色的 `role="alert"`

- **现状**：`W/pages/page/page-layout.tsx:312` `{source && <Alert>{t("page.sourceReadonly")}</Alert>}`；`Alert`（`components/ui/alert.tsx:6-12`）是红色、`role="alert"`。读者、写者每次打开来源页都"出错"地念一遍。
- **改法**：换成 `<p className="rounded-md border px-4 py-3 text-sm text-muted-foreground">`：平常的说明，不播报。文字不变。
- **测试**：vitest，`W/pages/page/source-actions.test.tsx` 新加 `"a source says how it is changed as a note: no alert, not in red"`：来源页；`screen.getByText("Sources keep their original content for the wiki to use. To change one, an admin can move it out of the source area first.").closest('[role="alert"]')` 为 `null`；`screen.queryByRole("alert")` 为 `null`。**没修的代码上**：是 alert。
- **已有测试**：来源页上找 alert 的格都限在对话框或树里（`source-attachments.test.tsx`、`source-moves.test.tsx`），照旧；e2e `lw4-source-readonly.spec.ts:302`（`toContainText`）照旧。
- **变异**：改回 `<Alert>` → 红。

#### 4.1.4 一处改了焦点的规则会波及别处：核对过的结论（05-lessons 第 6 节）

- **空闲退出**（M5/P5，`page-edit.tsx` 的 `leaveIdle` → `done({ idle: true })` → `PageShell` 的 `back`）：
  - 18.6 把 `back` 的处理挪到 `editable` 判断之前，有"编辑"时仍给"编辑"（`back === true`），`"fallen"` 仍只在焦点空着时移：去处不变（`page-autosave.test.tsx:152`、`:219` 照旧）。
  - 空闲退出期间 `ConfirmDialog` 的"离开"（`page-edit.tsx:425`）发送中改为 `aria-disabled`，焦点留在它上面；关闭之后 `onClosed(left)` 什么也不做，`back` 照旧给"编辑"。
  - 退出登录的提问（第 7 条）在空闲退出存着未保存的文字时会问，确认之后照旧先保存（`endEdits`）。
- **未读的内容**（还没读到的页、视图、树）：
  - 28.4：页面的标题在树读到之前不挂上（`page-layout.tsx:58-60`），看守从提交起；`NotLoaded`、`Loading` 本身不取焦点。
  - 阅读视图为锚点的重读（`reading-view.tsx:267` 的 `watchReader`）：到达的标题先取焦点是程序的移动，不是读者的动作，锚点的元素回来时照旧取；读者动了，两者都不移。
  - 附件一节的 `useLocated`（`attachments-section.tsx:420-431`）、任务列表的 `useArriving`（`transfer-page.tsx:275-280`）各有自己的看守，`Located` 不进 `useArrivalWatch`，不变。
  - 23.14、18.6 的 `replacing`：只在这次导航已显示过的页被"页面不存在"代替时，从无人处取；初次打开一个不存在的地址（不是到达）照旧不取。不是到达的导航（树打开的页）在别处被删、焦点在页里时，焦点从 `body` 改到"页面不存在"的标题（推断是改进）。
  - 只换 state 的 `replace`（28.4 的规则）：看守与 `offered` 接续；26.4 与 28.5 的筛选的 `replace` 换了查询：看守重新起算，`offered` 照样接续（原写"不受影响"不成立，决定 507）。
- **portal 里的对话框**：
  - `ConfirmDialog`（`AlertDialog`，portal）改 `aria-disabled` 之后焦点一直在对话框里，Radix 的 `FocusScope` 不用拉回；被拒之后焦点丢了才交还给"确认"（同一个 portal 里）。
  - `useForm` 的横幅在对话框（portal）里时，取焦点仍在 `FocusScope` 之内；横幅在页面上的表单（设置页）同样。
  - 退出登录的对话框从下拉菜单（另一个 portal）开：菜单先关，Radix 在 `setTimeout(0)` 里把焦点还给"Ada"；至多 100 毫秒之后对话框开、焦点给"留下"；留下时 `onClosed(false)` 把焦点还给"Ada"（同页面菜单开导出对话框）。
  - 16.1：菜单的内容是 portal，行卸下时看它，焦点在里面就交给这一节的标题；之后 Radix 对已卸下的触发按钮 `focus()`，什么也不做（推断），焦点留在标题。
  - 28.4 的看守：对话框里的按键（快速切换里按 Enter 开始导航）在提交之前，不算；导航之后对话框关、焦点回到触发处是 Radix 的移动，不是读者的事件，不算。
- **别的**：
  - 撤销的确认框（`useForm` + `aria-disabled`）：9.1 的横幅规则在焦点没丢时不动，HI3 与 `revert-background.test.tsx:183` 照旧。
  - 导入对话框（`import-dialog.tsx:229-238` 自己移焦点）：发送中焦点在"停止上传"上，没丢，横幅不取；它自己的规则照旧。
  - 工作区搜索框（直接用 `useFocusOnInvalid`）：总有 `aria-invalid` 的输入框，横幅一支走不到。

#### 4.1.5 vitest 与 e2e 的分工（00 号文档 9.1）

| 项 | 只用 vitest 够不够 | 理由 | e2e |
|---|---|---|---|
| 23.16 | 不够 | 旧的毛病是 Chromium 下一次渲染的焦点修正，jsdom 不做：jsdom 里只能钉"不是 `disabled`"与"焦点丢了就交还" | A10，三种（已在 `threeBrowsers`）；键盘按 |
| 23.17、28.2 | 不够 | jsdom 不算 Tailwind 的样式 | HI3，Chromium（不透明度不分引擎） |
| 9.1、38.3 | 逻辑够（`:disabled` 与 `body` 两格） | 真浏览器的修正只能真跑，第 2 格只是模拟；Firefox、WebKit 禁用之后焦点各在哪没有核实 | LW3，三种（已在 `threeBrowsers`）；键盘提交 |
| 12.3 | 够 | 链接随页卸下，`activeElement` 回到 `body`；不靠布局与焦点修正 | 不进；三引擎探查一次（Safari 不给点过的链接焦点，点之前就在 `body`，结果应相同）；可选 PG11 一行 |
| 26.4 | 够 | 路由状态与 `replace` 在 memory router 一样 | 不进 |
| 28.4 | 够 | `watchReader` 的事件 jsdom 有 | 不进 |
| 28.5 | 够 | `POP` 与状态在 memory router 一样 | 不进；三引擎探查一次后退前进（WebKit 不滚到靠近顶部的标题已由 `takeArrivalFocus` 补） |
| 23.14、18.6 | 够 | 元素移除 | 不进 |
| 16.1 | 够 | Radix 的 portal 卸下在 jsdom 一样 | 不进 |
| 21.14、22.17 | 够 | | 不进 |
| 6.3、19.3、24.3、49、35.2、38.1、11.7 | 够 | 名字、描述、角色 jsdom 算得出（`aria-label`、`aria-describedby`，没有 `sr-only` 的空格问题） | 不进 |
| 7 | 逻辑够 | 真的 `BroadcastChannel` 跨标签页只有真浏览器（vitest 用 `FakeChannels`）；C6 本来就在测这条路 | C6，Chromium |

不进 `threeBrowsers` 的新条目：A10、LW3 已在；HI3、C6 的断言不分引擎。"三引擎探查一次"：实施者在 Step 里写一个临时的故事，经 `docs/v0.1/claude_to_codex/tools/heavy.sh` 以三个 project 跑一次，结果写进 Step 的记录，不提交那个故事。

### 4.2 S2 文字

S2 只改文字与少量行为（23.11 冲突时重读，6.1、22.12、22.13 选哪一句），另有服务端的两份 `AGENTS` 模板（第 40 条，全文在 3.1）。

- **本节的取舍**：P2-8 至 P2-14（2.3）。
- **i18n 的规则**：同一个键的 en 与 zh-CN 在同一个提交里；删键与它的用处在同一个提交里；改了英文的 e2e 断言与那个键在同一个提交里。

#### 4.2.0 总表

| 编号 | 分类（I3；00 号） | 组件 | 测试文件 |
|---|---|---|---|
| 6.1 | B；B | `W/pages/page/page-layout.tsx`；替身 `W/test/page-server.ts` | `W/pages/page/page-tasks.test.tsx` |
| 9.5 | B；B | `W/pages/notebook/distinct-name.ts` | `W/pages/notebook/tag-page.test.tsx` 等 7 个文件改断言，加两格 |
| 15.3 | B；B | 文字 | `W/pages/page/unresolved-link.test.tsx` 等 4 个 |
| 16.6 | B；B | `W/pages/page/asset-dialogs.tsx` | `W/pages/page/attachments-section.test.tsx` |
| 19.1 | B；B | `W/pages/notebook/export-dialog.tsx` | `W/pages/page/page-menu.test.tsx` |
| 21.8 | B；B | `W/editor/text-diff.tsx` | `W/editor/text-diff.test.tsx` |
| 21.11 | B；B | `W/editor/text-diff.tsx` | `W/editor/text-diff.test.tsx` |
| 22.11 | B；B | 文字 | `W/pages/notebook/trash-page.test.tsx`、`changeset-page.test.tsx` 等；e2e `tb1-trash.spec.ts:316` |
| 22.12 | B；B | `W/pages/notebook/trash-preview.tsx` | `W/pages/notebook/trash-preview.test.tsx` |
| 22.13 | B；B | `W/pages/notebook/trash-preview.tsx` | `trash-preview.test.tsx`；e2e `tb1-trash.spec.ts:369-376` 的观察者 |
| 22.16 | B（审查时记 C）；B | 文字 | `W/pages/page/page-lock.test.tsx`；e2e `c4-guard.spec.ts:236,273` |
| 23.1 | B；B | 文字 | `revert-dialog`、`revert-background`、`changeset-page`、`transfer-import`、`transfer-background`、`upgrade-page` 的 `.test.tsx`，`transfer-report.test.ts`，`W/app/problem-messages.test.ts` |
| 23.3 | B；B | 同 22.11 | 同 22.11 |
| 23.4 | B；B | 文字（en） | `W/pages/notebook/revert-dialog.test.tsx` |
| 23.11 | B；B | `W/pages/notebook/revert-dialog.tsx` | `revert-dialog.test.tsx` |
| 23.12 | B；B | 文字（zh） | `W/pages/notebook/history-page.test.tsx`、`revert-dialog.test.tsx` |
| 23.13 | B；B | `W/pages/notebook/history-filters.tsx` | `history-page.test.tsx` |
| 24.7 | B；B | 文字 | `W/pages/notebook/transfer-background.test.tsx` |
| 26.11 | B；B | 文字 | `W/pages/search/search-page.test.tsx` |
| 26.13 | B；B | `W/pages/search/search-page.tsx` | `search-page.test.tsx` |
| 26.14 | B（可降 C）；B | `W/pages/search/search-status.ts` | `W/pages/search/search-status.test.ts` |
| 35.1 | B；B | `index-page.tsx`、`index-rows.tsx`、`health-rule.tsx`、`health-item.tsx`、`health-page.tsx`、`upgrade-check.tsx` | `index-page`、`health-rule`、`health-page`、`upgrade-check` 的测试 |
| 38.6 | B；B | `W/i18n/format.ts` | `W/i18n/format.test.ts` 与另 9 个文件（共 10 个文件 19 行；实际 11 个文件 23 行，见 10.2） |
| 38.8 | B；B | 文字 | `W/pages/notebook/duplicate-text.test.tsx`；e2e `lw6-source-history.spec.ts:439` |
| 40 | B；B | `wiki/domain/template-en-agents.md:33`、`template-zh-CN-agents.md:33` | go test：`wiki/domain/template_test.go` |
| 41.1 | B；B | `health-page.tsx:306`、`upgrade-check.tsx:252,383` | `upgrade-page`、`health-page`、`upgrade-check` 的测试 |
| 41.2 | B；B | `upgrade-check.tsx:446` | `W/pages/notebook/upgrade-check.test.tsx` |
| 41.3 | B；B | `general-page.tsx:230` | `W/pages/notebook/general-page.test.tsx` |
| H4 界面一句 | C 加文档（I3）；随 S2 | `W/pages/notebook/trash-page.tsx:191`、`W/pages/page/page-versions.tsx:95` | `trash-page.test.tsx`、`page-versions.test.tsx` |

- 都是小。"文字"指只改 `W/i18n/messages/{en,zh-CN}.ts`，组件不变。
- 测试都是 vitest，另注 e2e 与 go test。没有 e2e 新故事；e2e 只改四个文件里已有的断言（4.2.3.1）。
- 路径：`W/` 是 `web/apps/web/src/`；服务端的 `x/y.go` 在 `server/internal/modules/` 之下。行号在 `194ca350` 上读过，与 `86565dd6` 的代码相同。

#### 4.2.1 本 Step 新加、改、删的 i18n 键

##### 4.2.1.1 新加（23 个）

| 键 | en | zh-CN | 项 |
|---|---|---|---|
| `index.sourcesOne` | `{path} · 1 source` | `{path} · 1个来源` | 35.1 |
| `index.longTypesOne` | `1 page has a type name too long to filter by.` | `有1页的类型名称过长，无法按该类型筛选。` | 35.1 |
| `index.loadedOne` | `1 page shown.` | `已显示1页。` | 35.1 |
| `index.inboundOne` | `1 incoming link` | `1个入链` | 35.1 |
| `health.addedOne` | `1 more issue shown.` | `新增显示 1 项问题。` | 35.1 |
| `health.clearOne` | `No issues (1 check)` | `没有问题（1 项检查）` | 41.1 |
| `health.referringPagesOne` | `Referenced by 1 page` | `被 1 篇页面引用` | 35.1 |
| `health.staleOne` | `1 page has an outdated index. Ask the server administrator to run nervewiki reindex.` | `1 篇页面的索引已过时，请服务器管理员运行 nervewiki reindex。` | 35.1 |
| `upgrade.startRawOne` | `The page raw at the root becomes the source area (1 node under it now): once approved, everything under it is a read-only source. Your agent can move out what is not a source.` | `根下的 raw 将成为来源区（现有 1 个节点），批准之后它下面的都是只读的来源；不是来源的可以由 agent 移出。` | 41.3 |
| `upgrade.gateLinksWorseOne` | `1 page leads to more pages that do not exist than when the upgrade started.` | `1 页指向的待建页面比升级开始时多。` | 41.2 |
| `page.nameAtTop` | `{name} (at the top level of {notebook})`；实际 `{name} (at the notebook's top level)`（决定 584，10.2） | `{name}（{notebook} 的顶层）`；实际 `{name}（笔记本的顶层）` | 9.5 |
| `page.taskWouldGo` | `This task cannot be ticked or cleared here: its line would no longer be a task. Edit the page to change it.` | `这一项不能在这里勾选或取消勾选：改了之后它那一行就不再是任务项。请编辑页面来修改。` | 6.1 |
| `field.asset_name.too_long` | `At most 255 bytes, the extension included: 255 Latin letters, or about 85 Chinese characters.` | `连同扩展名最多 255 字节：255 个拉丁字母，或约 85 个汉字。` | 16.6 |
| `transfer.exportPageAloneTitle` | `Export the page {name}?` | `导出页面「{name}」？` | 19.1 |
| `diff.lastBreakAdded` | `The only change is a line break added at the end.`；实际 `The only change is that a line break was added at the end.`（审查 A 的 N1，10.2） | `唯一的改动是在末尾加了一个换行。` | 21.8 |
| `diff.lastBreakRemoved` | `The only change is the line break at the end, removed.`；实际 `The only change is that the line break at the end was removed.`（10.2） | `唯一的改动是去掉了末尾的换行。` | 21.8 |
| `diff.tooLargeBlock` | `Lines: +{added} −{removed}, but the changes join into a block too large to show line by line.` | `+{added} −{removed} 行，但改动连成的一块太大，不能逐行显示。` | 21.11 |
| `trash.notInTrashUnknown` | `This is not in the trash.` | `它不在回收站里。` | 22.13 |
| `history.attachment` | `Attachment: {name}` | `附件：{name}` | 23.13 |
| `history.nodeDeleted` | `A deleted page or attachment` | `已删除的页面或附件` | 23.13 |
| `history.removeFilterLabel` | `Remove the filter “{filter}”` | `去掉筛选「{filter}」` | 23.13 |
| `trash.purge` | `Nothing here can be deleted for good by hand: after an item can no longer be restored, the system deletes it for good.` | `这里的项不能手动永久删除：一项过了可以恢复的期限之后，由系统永久删除。` | H4 |
| `version.kept` | `No version can be deleted on its own, by hand or by the system: the page keeps every one.` | `版本不能单独删除，手动不能，系统也不会：这一页的每个版本都留着。` | H4 |

占位符两种语言相同（`W/i18n/messages.test.ts` 的 `placeholders` 核对）。`*One` 键都不带 `{count}`，照 `health.oneIssue`、`upgrade.checkFailedOne` 把 1 写死。

##### 4.2.1.2 改文字的键

| 键 | 改哪种语言 | 新的 en | 新的 zh-CN | 项 |
|---|---|---|---|---|
| `unresolved.reader` | 两种 | `No page or attachment has this name. The notebook's editors can create it from this link.` | `没有这个名称的页面或附件。笔记本的编辑者可以从这条链接新建它。` | 15.3 |
| `unresolved.readOnly` | 两种 | `No page or attachment has this name. This view is read only: no page is created from its links.` | `没有这个名称的页面或附件。这是只读的视图，不从它的链接新建页面。` | 15.3 |
| `unresolved.upgrading` | 两种 | `No page or attachment has this name. The notebook is being upgraded to an LLM Wiki: no page is created until the upgrade ends.` | `没有这个名称的页面或附件。这本笔记本正在升级为 LLM Wiki，升级结束之前不新建页面。` | 15.3 |
| `unresolved.readerWiki` | 两种 | `No page or attachment has this name. The notebook's editors may be able to create it from this link.` | `没有这个名称的页面或附件。笔记本的编辑者也许可以从这条链接新建它。` | 15.3 |
| `unresolved.embed` | 两种 | `No page or attachment has this name. An embed shows a page or an attachment that is there; it does not create one.` | `没有这个名称的页面或附件。嵌入只显示已有的页面或附件，不新建。` | 15.3 |
| `unresolved.image` | 两种 | `No page or attachment has this name. An image shows what is there; it does not create anything.` | `没有这个名称的页面或附件。图片只显示已有的内容，不新建任何东西。` | 15.3 |
| `trash.fromTop` | zh | 不变 | `原位置：笔记本的顶层` | 22.11 |
| `trash.toTop` | zh | 不变 | `恢复到笔记本的顶层` | 22.11 |
| `trash.toTopParentInTrash` | 两种 | `Restores at the notebook's top level: the page it was under is in the trash` | `恢复到笔记本的顶层：原上级页面在回收站里` | 22.11 |
| `trash.toTopTooDeep` | 两种 | `Restores at the notebook's top level: where it was, it would nest more than 10 levels deep` | `恢复到笔记本的顶层：放回原处会超过 10 层` | 22.11 |
| `trash.restoredTop` | zh | 不变 | `已恢复「{name}」到笔记本的顶层。` | 22.11 |
| `trash.restoredAsTop` | zh | 不变 | `已恢复为「{name}」，在笔记本的顶层：那里已有同名的页面或附件。` | 22.11 |
| `history.detail.top` | zh | 不变 | `笔记本的顶层` | 23.3 |
| `transfer.placedTop` | zh | 不变 | `放回了原处：笔记本的顶层。` | 23.3 |
| `transfer.placedParentInTrash` | zh | 不变 | `放在了笔记本的顶层：原上级页面在回收站里。` | 23.3 |
| `transfer.placedTooDeep` | zh | 不变 | `放在了笔记本的顶层：放回原处会超过 10 层。` | 23.3 |
| `index.root` | 两种 | `The notebook's top level` | `笔记本的顶层` | 22.11 |
| `editor.lostGone` | 两种 | `This page was deleted, or you lost access to it, while you edited it: this editor saves no more.` | `编辑期间这一页被删除了，或你已无权访问它：这里不再保存。` | 22.16 |
| `problem.page.changeset_trimmed` | 两种 | `Some or all of this change has been purged from the trash: it can no longer be undone.` | `这次改动已有部分或全部从回收站清理，无法再撤销。` | 23.1 |
| `problem.page.revert_too_large` | en | `This change is too large to undo at once.` | 不变 | 23.1 |
| `problem.page.revert_conflict` | en | `What this change touched has changed since: it cannot be undone as it is.` | 不变 | 23.1 |
| `upgrade.approveBody` | en（zh 建议同改） | 把 `the upgrade's changesets can still be reverted one by one in the history` 换成 `the upgrade's changes can still be undone one by one in the history`，其余不变 | 建议把 `升级的变更集仍可…` 换成 `升级的改动仍可…`（4.2.5） | 23.1 |
| `transfer.failure.changeset_reverted` | en | `What the import had written was undone while it ran.` | 不变 | 23.1 |
| `transfer.failure.changeset_trimmed` | 两种 | `The purge took some or all of the change: it can no longer be undone.` | `这次改动已有部分或全部被清理，不能再撤销。` | 23.1 |
| `history.trimmedNote` | 两种 | `Some or all of what this change touched has been purged from the trash: it can no longer be undone.` | `这次改动碰过的内容已有部分或全部被清理，不能再撤销。` | 23.1 |
| `history.conflict.parent_gone` | en | `The page that “{name}” was under is gone: it is in the trash, or purged.` | 不变 | 23.4 |
| `history.conflictTrashLabel` | en | `Trash: the page that “{name}” was under may be there` | 不变 | 23.4 |
| `history.atLeast` | zh | 不变（`{count}+`） | `{count}+` | 23.12 |
| `transfer.restoreLeftUndone` | 两种 | `What it restored was undone and is in the trash again, deleted by that undo; the nodes it had not come to are there too, in items of their own.` | `已恢复的部分被撤销，由那次撤销删除，又回到了回收站；没处理到的节点也在回收站里，各自成项。`；实际 `已恢复的部分被一次撤销删除，又回到了回收站；没处理到的节点也在回收站里，分成几项。`，英文实际 `An undo deleted what it restored, which is in the trash again; the nodes it had not come to are there too, in items of their own.`（审查 A 的 N2，10.2） | 24.7 |
| `search.howTo` | 两种 | `Type what to find in the search box at the top, and press Enter.` | `在顶部的搜索框里输入要找的文字，按回车。` | 26.11 |
| `source.historyRestore` | 两种（只换第一句） | `A restore or an undo follows the source rules at every step, not only in what it leaves. To restore content, first move the page outside the source area, restore it, then move it back. Resolve any conflicting source before restoring or moving back; moving back checks for duplicates again. Administrators cannot edit source content either.` | `还原与撤销的每一步都要遵守来源规则，不只是最后的结果。需要还原正文时，先将页面移到来源区外，还原后再移回。恢复或移回前先处理冲突来源；移回时仍会检查重复。管理员也不能修改来源正文。` | 38.8 |

修复第 1 轮另改了 `problem.page.virtual_file` 的中文：把"日志就是变更集"改成"日志就是这本笔记本的改动"，与决定 425 同一个用词（审查 A 的 N3）。改文字的键实际 en 22 个、zh-CN 27 个（10.2）。

38.6（已定：2.3 的 P2-11）不改 i18n 文件：`W/i18n/format.ts:50` 的单位改成 `B`、`KiB`、`MiB`、`GiB`、`TiB`，两种语言同。

##### 4.2.1.3 删掉的键

- `history.pageDeleted`、`history.removePageLabel`（23.13，由 `history.nodeDeleted`、`history.removeFilterLabel` 取代；与 `history-filters.tsx` 的用处同一个提交）。
- 没有"每个键都有人用"的门禁（knip 不看对象的键，`messages.test.ts` 只核对占位符与非空），删键靠实施者 grep。

#### 4.2.2 英文单复数的全面清点（35.1、41.x）

##### 4.2.2.1 已有的写法

`health.oneIssue`（`W/pages/notebook/health-rule.tsx:59-61`）与 `upgrade.checkFailedOne`（`upgrade-check.tsx:188-191`）按 `=== 1` 选键，把 1 写死、不带占位符。新键照此，一律后缀 `One`（00 号文档 4.5）。CodeMirror 的 `"$ page"`/`"$ pages"`（`W/editor/phrases.ts:35-36`）由 CodeMirror 选键，不在此列。

##### 4.2.2.2 `en.ts` 里带计数的键，逐个判断

改的十个，各加一个 `*One`（已定：2.3 的 P2-8）：

| 键（`en.ts` 行） | 什么时候显示 1（组件） | 项 |
|---|---|---|
| `index.sources`（322） | 来源区只有一个来源（`index-page.tsx:270-273`） | 35.1 |
| `index.longTypes`（316） | `unfilterable_type_pages` 为 1（`index-page.tsx:68-71`，`> 0` 才显示） | 35.1 |
| `index.loaded`（319） | 目录只有一页（`index-page.tsx:256`） | 35.1 |
| `index.inbound`（333） | 一个入链（`index-rows.tsx:99-101`，每行都显示） | 35.1 |
| `health.added`（340） | "更多"只加了一项（`health-rule.tsx:170`） | 35.1 |
| `health.clear`（346） | 只有一条规则计数为 0（`health-page.tsx:306`、`upgrade-check.tsx:252`、`:383`） | 41.1 |
| `health.referringPages`（353） | 待建页只被一页链到，最常见（`health-item.tsx:84`） | 35.1 |
| `health.stale`（357） | `health-page.tsx:295`、`upgrade-check.tsx:458` | 35.1 |
| `upgrade.startRaw`（707-708） | `raw` 下只有一个节点（`general-page.tsx:228-231`） | 41.3 |
| `upgrade.gateLinksWorse`（778） | 只有一页变坏（`upgrade-check.tsx:445-447`） | 41.2 |

不改的：

| 键（`en.ts` 行） | 理由 |
|---|---|
| `index.inboundCapped`（334） | 只在上限 1000 时显示（`linking/domain/index.go:24`、`linking/adapter/http/index.go:45`） |
| `health.completed`（344） | 4 或 8 条（`health-page.tsx:229-231`） |
| `page.backlinkCount`（228） | 只在 `count > 1` 时显示（`page-backlinks.tsx:155-160`） |
| `page.quickSwitchFirst`（204） | 恒为 50（`quick-switch.tsx:19,187`） |
| `history.conflictsFirst`（475） | 服务端的上限 100，只在 `count > conflicts.length` 时显示（`revert-dialog.tsx:437-439`、`transfer-report.tsx:401`） |
| `editor.phrase.unchangedLines`（656） | `collapseUnchanged` 的 `minSize: 4`（`text-diff.tsx:144`） |
| `health.count`、`upgrade.checkFailed` | 已有 `health.oneIssue`、`upgrade.checkFailedOne` |
| `editor.phrase.replacedMatches`（650） | 会显示 1，但键由 CodeMirror 选、只有这一个短语；`W/editor/phrases.test.ts:9-11` 钉着 en 等于 CodeMirror 的短语；只影响读屏的一句 |
| `page.moreHeadings`、`search.moreFound`、`search.soFar`、`index.typeCount`、`history.atLeast`、`onboarding.progress`、`page.untitledN`、`transfer.importSent`、`problem.rate_limitedFor` | 没有要变形的名词 |
| `page.deleteSubpages`、`page.deleteAttachments`、`trash.counts`、`trash.tooLarge`、`history.counts`、`history.revertBody`、`history.revertTooLarge`、`history.conflictsTitle`、`history.ingestCount`、`history.listed(More)`、`search.count/about/atLeast/capped`、`asset.uploading`、`diff.changes`、`diff.lines`、`diff.tooLargeLines`、`upgrade.gatePreserved`、`ownerless.members`、`ownerless.reminder`、`transfer.progress`、`transfer.conflictsTitle`、`transfer.restoreLeft*`、`transfer.revertLeft*` | 标签式（`Nodes: 1`），1 也通顺 |

`formatPeriod`、`formatDuration`、`formatRelative`（`W/i18n/format.ts`）经 `Intl`，单复数由浏览器处理。中文没有这个问题，只为新键写对应的中文。

#### 4.2.3 e2e 与 vitest 里断言旧文字的地方（汇总）

##### 4.2.3.1 e2e（只改已有断言，不加故事，不动 `threeBrowsers`）

| 文件:行 | 现在断言的 | 改成 | 项 |
|---|---|---|---|
| `e2e/stories/collab/c4-guard.spec.ts:236`、`:273`（删笔记本） | `toContainText("This page no longer exists: this editor saves no more.")` | `"This page was deleted, or you lost access to it, while you edited it: this editor saves no more."` | 22.16 |
| `e2e/stories/trash/tb1-trash.spec.ts:315-317` | `"Restores at the top level: the page it was under is in the trash"` | `"Restores at the notebook's top level: the page it was under is in the trash"` | 22.11 |
| `e2e/stories/trash/tb1-trash.spec.ts:369-376` | `MutationObserver` 只找 `"This is no longer in the trash."`（否定断言：恢复在途时不说"已不在"） | 同时找 `"This is not in the trash."`；否则新代码下它读不到中性的那一句，什么也不查 | 22.13 |
| `e2e/stories/wiki/lw6-source-history.spec.ts:439` | `not.toContainText("Temporary steps")`（改名被拒不给"移出再移回"的建议） | `not.toContainText("outside the source area")`；第一句换掉之后旧词永远不出现，断言变空 | 38.8 |

- 四个文件都只跑 Chromium。推断全量 e2e 的用时不变。
- 核对过不受影响的：`l2-create.spec.ts:118`（读者一句的第二句）、`:83`；`tr1-export.spec.ts:261`（Café 下有 One）；`hi3-log-revert.spec.ts:338`（`Page: Alpha`）；`tb2-background-restore.spec.ts:205,239`、`hi5-revert-import.spec.ts:217`、`tb1-trash.spec.ts:321`、`c4-guard.spec.ts:219`（英文的 `the notebook's top level` 不变）；`lw8-health.spec.ts:674`、`e2e/fixtures/wiki-health.ts:37`；`up2-start-refused.spec.ts:136`（2 nodes）；`hi5-revert-import.spec.ts:175`；`as1-upload.spec.ts:136`、`as3-render.spec.ts:346`（单位 `B`）；`lw1-create.spec.ts:126-143`（AGENTS 的正文从服务端读）；`lw6-source-history.spec.ts:485-486`。
- e2e 没有断言同名区分的 `(in …)`、`No page has this name`、`Page: a deleted page`、`was under is gone`、diff 的文字。

##### 4.2.3.2 vitest 里断言旧文字、要随之改的（逐项也写在各节）

逐项写在各节（4.2.6 各项的"随之改"）。

#### 4.2.4 提交的分组

见 6.2。

#### 4.2.5 拿不准的、要实施者先核对与审查重点看的

取舍已定在 2.3 的 P2-8 至 P2-14。下面是读代码的推断，实施者先核对：

1. 6.1：替身 `tasksOf` 的正则改了之后，先跑 `page-tasks.test.tsx` 与用 `withTasks` 的测试（推断不受影响：它们都写成 `- [ ] x`）。实际：成立，全量照过；替身的正则后来照服务端的解析改（10.2）。
2. 22.11：`transfer-background.test.tsx:176-200` 的表有没有 locale 的口子；没有就加一格 `setLocale`。实际：没有，另写一张 zh 的 `test.each`。
3. 22.13：`trash-preview.test.tsx` 的 `:203`、`:361`、`:427`、`:534`、`:572` 推断不变（节点都在树里）。实际：不变；`:303` 加一条否定断言（22.13）。
4. 23.1：`revert_too_large`、`revert_conflict` 的英文今天推断没有测试断言；23.4：`transfer-report.tsx` 的冲突列表推断也用 `reasonText`。实施者 grep 核对。实际：两条推断都成立；英文两行加在 `problem-messages.test.ts`。
5. 35.1：`indexFacts` 夹具在哪；`health-page.test.tsx` 的中文格里有没有被 2 页引用的项。实际：`indexFacts` 在 `W/test/index-server.ts`；`health-page` 的夹具里待建页的引用数都不是 1，新格改一项。

另有四处小的取舍不在 2.3，各项里写了推荐的做法，已定在 2.3要不要记成主持者的决定：23.1 的 `upgrade.approveBody` 中文同改"改动"；22.13 用中性的一句、不先显示加载中；23.12 用 `{count}+`；9.5 写"(at the top level of Plans)"、不用所有格（前一半由决定 584 取代，实际见 10.2）。

#### 4.2.6 逐项

##### 6.1 勾了会让任务项消失时只说通用的话

- 现状：
  - 服务端：`page/domain/errors.go:358-364` 的 `TaskWouldGo()` 答 422 `validation_failed`，字段错误 `{field: "offset", code: "not_allowed"}`（契约 `api/modules/page.yaml:510-523`）。REST 里只有 `toggleTask` 用 `offset`（`page.yaml:2252-2259`）。
  - 网页：阅读视图的勾选失败经 `report` 到 `W/pages/page/page-layout.tsx:204-217` 的 `toggleRefused`，不是 `page.locked` 的都 `setRefusal(error)`（`:206`）；`:298` 显示 `<Alert>{errorText(refusal, t)}</Alert>`，`errorText`（`W/app/problem-messages.ts:109-137`）对 `validation_failed` 只给 `problem.validation_failed`。
  - 用户：正文里写了 `- [ ]: /u`，点它的复选框，页上方写"Some values are not valid."/"有些内容不符合要求。"（移交写的"有些值无效"不对，I3 已记）。
- 改法：
  - `page-layout.tsx` 加不导出的 `refusalText(error: unknown, t: Translate): string | undefined`：`error` 是 `ApiError`、`code === "validation_failed"`、`problem.errors` 里有 `field === "offset" && code === "not_allowed"` 时给 `t("page.taskWouldGo")`，否则 `errorText(error, t)`。`:298` 改用它。`Translate` 从 `../../i18n/i18n` 导入。
  - "编辑"被拒也走 `refusal`，开会话没有 `offset` 字段，不受影响。
  - 新键 `page.taskWouldGo`（4.2.1.1）。
  - 不取 `field.offset.not_allowed` 加 `fieldErrors`：`out_of_range` 会显示更差的"Out of range."，13.2 第 11 条的字段文案又是全局的。
- 假服务端：
  - `W/test/page-server.ts:1110-1121` 的 `tasksOf` 认出 `]` 之后紧跟 `:` 的项（goldmark 认它是任务项）：正则改为 `/^- \[([ xX])\]( |(?=:))(.*)$/` 或等价，每项另记 `goes`。实际不成立：只有冒号之后是单个目的地、其后只有空白的行，勾上之后才成链接引用定义；勾着的 `- [x]: /u` 本来就是定义、不是任务项，`- [ ]: a b c`、`- [ ]:` 等照普通任务项（审查 B 的 M1，决定 585；`W/test/page-server-tasks.test.ts` 十五行对照，实际见 10.2）。
  - `toggle-task`（`:1135-1172`）在 `out_of_range` 之后、"同一状态什么也不写"之前：`task.goes && task.checked !== toggle.checked` 时答 422 `validation_failed`、`errors: [{ field: "offset", code: "not_allowed" }]`（次序照 `page.yaml:519-523`）。替身的说明（`:108-112`）加一句。
  - 可选：在 `W/test/page-server-source.test.ts` 旁的替身自测里加一格（05-lessons 第 2 节，推断有用）。
- 测试（`W/pages/page/page-tasks.test.tsx`，`test.each(["en", "zh-CN"])`）：
  - 名：`a tick that would leave no task item says so above the view, in %s; the checkbox as it was, nothing written`。
  - 逼出：`opened("editor", "- [ ]: /u\n- [ ] b\n")`（zh 先 `setLocale("zh-CN")`，`opened` 加一个参数或在 `pageServer` 上设），点第一个复选框。
  - 断言：`(await screen.findByRole("alert")).textContent` 整句等于新键的那种语言；`boxes()[0]?.checked` 为 `false`；`server.contents.get(install.id)` 不变；`server.sent` 里有一条 `TOGGLE`。
  - 没修的代码上：alert 是通用的那句，整句比对失败。
- 变异：`refusalText` 恒用 `errorText`（两格失败）；条件里 `not_allowed` 改成 `out_of_range`（两格失败）；替身不答 422（`checked` 断言失败）。
- 牵连：S4 的 6.2 改 `reading-view.tsx` 的 `toggleTask`，不碰 `page-layout.tsx:298`，冲突小（推断）。

##### 9.5 `distinctName` 在一处仍重名

- 现状：`W/pages/notebook/distinct-name.ts:22`：`place === undefined ? node.name : t("page.nameIn", { name, place: place || notebook.name })`。`placeOfTitle`（`W/stores/page-tree.ts:86-95`）对根下的页答 `""`，于是根下的 Notes 写作"Notes (in Plans)"，Plans 根下页的子页 Notes 也是"Notes (in Plans)"。
  - 用它的 13 处：`tag-page.tsx:58`、`general-page.tsx:245`、`revert-dialog.tsx:200`、`page-tree-item.tsx:84,180,371`、`rename-page-dialog.tsx:68`、`page-tree.tsx:169`、`asset-dialogs.tsx:74,173`、`upgrade-page.tsx:496,604`、`move-page-dialog.tsx:111`、`transfer-report.tsx:326`、`version-restore.tsx:175`、`page-backlinks.tsx:60`。
- 改法：
  - `:22` 改为：`place === undefined` 答 `node.name`；`place === ""` 答 `t("page.nameAtTop", { name: node.name, notebook: notebook.name })`；否则 `t("page.nameIn", { name: node.name, place })`。实际：`page.nameAtTop` 不带笔记本名，`distinctName` 去掉 `notebook` 参数，十六处调用随之改（决定 584，10.2）。
  - 文档注释（`:5-9`）的"or the notebook at the root"改成"or, at the root, that it is at the notebook's top level"。
  - 新键 `page.nameAtTop`（4.2.1.1），用词随 22.11。
  - 总体设计 13.2 第 17 条随之改写（8.2）。
- 测试：
  - `W/pages/notebook/tag-page.test.tsx:33`：`"Install (in Plans)"` 改成 `"Install (at the top level of Plans)"`（这一格本来就有根下与页下各一项）。实际是 `"Install (at the notebook's top level)"`（10.2）。
  - 加一格（同文件或 `page-tree.test.tsx`）`a page at the root and a page under a root page titled as the notebook are told apart`：`plans = pageNode(5, "Plans")`、`under = pageNode(6, "Notes", plans)` 与根下的 `notes`；断言树的按钮名分别是 `Actions for Notes (in Plans)` 与 `Actions for Notes (at the top level of Plans)`（实际后者是 `Actions for Notes (at the notebook's top level)`，10.2）。没修的代码上两个都是"(in Plans)"，`getByRole` 找到两个而抛出。
  - zh 一格：同上，`Notes（Plans 下）` 与 `Notes（Plans 的顶层）`（实际后者是 `Notes（笔记本的顶层）`）。
- 随之改（都是笔记本 Plans 根下的页）：`page-backlinks.test.tsx:71`；`page-tree.test.tsx:104`、`:181`；`page-tree-drop.test.tsx:72`；`page-tree-writes.test.tsx:237`、`:250`、`:251`、`:260`；`upgrade-page.test.tsx:1464`、`:1467`、`:1472`（`raw（Plans 下）` → `raw（Plans 的顶层）`；实际 `raw（笔记本的顶层）`，10.2）。页下的 `page-tree.test.tsx:107,117,180-185`、`page-tree-writes.test.tsx:369` 不变。
- 变异：删掉 `place === ""` 的分支（新格与改过的断言失败）；`nameAtTop` 的 name、notebook 对调（tag-page 的断言失败）。实际：修复第 1 轮之后键里没有 `{notebook}`，对调一条不再有；换成键的两种语言各改回带名字的写法（`f5-nameAtTop-en`、`-zh`，S2 的记录）。
- 取舍：英文比原来长；"(at Plans's top level)"对以 s 结尾的笔记本名不好看，不取（2.3 的 P2-51）。实际：修复第 1 轮改成不带笔记本名的 "(at the notebook's top level)"（审查 A 的 Q1，决定 584）：与回收站、转移、索引、历史、移动对话框同一个说法；同一本笔记本里笔记本名不区分什么（10.2）。
- 牵连：S3 的 `bdi`（26.12、32.4）推断不受影响：这些是 `t()` 拼出的字符串，用在 `aria-label` 与纯文字里。

##### 15.3 附件写法的未建链接说成"页"

- 现状：`W/pages/page/unresolved-link.tsx:126-140`：嵌入、图片（`link.kind !== "link"`）、只读的视图、读者、升级中都不问服务端落点，直接 `show({ why })`；`whyText`（`:298-326`）的六个键第一句都是"No page has this name."。只有写者点普通链接才问落点，答 `target_is_asset` 才说附件（`:152-157`）。读者点 `[[x.png]]`、任何人点未解析的 `![[x.png]]`，都说"没有这个名称的页面"。
- 已定：2.3 的 P2-12（中性的说法；服务端只在有同名附件时才把 `x.png` 读成附件，`linking/domain/target.go:109-126`）。
- 改法：只改六个键（4.2.1.2）：第一句改成"页面或附件"；嵌入与图片的第二句不说"页"。读者、只读、升级中、`readerWiki` 的第二句不变（`l2-create.spec.ts:118` 用 `toContainText` 读者那句的第二句）。`unresolved.sourceArea` 只在服务端答了页的落点之后出现，不改。
- 测试：
  - `W/pages/page/unresolved-link.test.tsx:140-163` 那一格的 `texts` 改成新文字，并加一条读者的、带扩展名的普通链接（`linkTo("q.png")`）：也说 `unresolved.reader` 的新句。没修的代码上旧文字不等于新文字，`getByText` 抛出。
  - 加 zh 一格 `to a reader in Chinese, a link to a name with an extension says neither a page nor an attachment has it`：读者点 `q.png`，对话框描述整句等于 zh 的 `unresolved.reader`；嵌入一条同。
- 随之改：`unresolved-link.test.tsx:143-145`；`W/pages/notebook/trash-preview.test.tsx:85`、`W/pages/page/version-page.test.tsx:103`（`readOnly`）；`W/pages/notebook/upgrade-read-only.test.tsx:384`（正则 `/^No page has this name\./u` 改成 `/^No page or attachment has this name\./u`）、`:386-387`。
- 变异：任一键改回旧文字，对应一格失败。实际：中文的 `readOnly`、`upgrading`、`readerWiki`、`image` 改回照过（没有格读那句中文），第 10 个提交各补一格之后抓到（10.2）。
- 牵连：移交 M7-transfer T8（从解析不到的附件链接直接上传）是 C、进 v0.2。

##### 16.6 改名的 `too_long` 没说扩展名也算

- 现状：`W/pages/page/asset-dialogs.tsx:63-75` 的 `RenameForm`：`check={(name) => assetNameProblem(name, extension)}`，`fieldTexts={notebookNameTexts}`（`:71`）。`assetNameProblem`（`:93-100`）对 `stem + extension` 调 `notebookNameProblem`，超过 255 字节答 `field.notebook_name.too_long`。字段里只填词干（扩展名在字段之后），词干不到 255 也被拒。
- 改法：
  - 模块里加 `const assetNameTexts = { ...notebookNameTexts, "name.too_long": "field.asset_name.too_long" } as const satisfies FieldTexts;`，`:71` 改用它（`FieldTexts` 从 `../../app/problem-messages` 导入）。
  - `assetNameProblem` 的最后一行：先判 `isPageName(name)` 答 `field.asset_name.page`；`notebookNameProblem` 答 `field.notebook_name.too_long` 时换成 `field.asset_name.too_long`；其余照答。
  - 新键 `field.asset_name.too_long`（4.2.1.1）。没有扩展名的附件也用它。
- 测试（`W/pages/page/attachments-section.test.tsx`，`:291` 那一格之后）：
  - `test.each(["en", "zh-CN"])("a stem too long with its extension says the extension counts, in %s, and sends nothing; the server's too_long says the same")`：打开 photo.png 的改名，清空字段，`user.paste("a".repeat(252))`（加 `.png` 共 256 字节），回车；字段下的文字整句等于新键；`patches === 0`（照 `:291` 的计数法）。
  - 再改成短的词干，替身答 `problem(422, "validation_failed", { errors: [{ field: "name", code: "too_long" }] })`，断言同一句。
  - 没修的代码上显示笔记本名的那句，整句比对失败。审查 B 实跑：新格在没修的代码上失败，照此改了之后本地与服务端两半都过，tsc 过。
- 变异：去掉 `assetNameTexts` 的覆盖（服务端那一半失败）；去掉本地的换键（本地那一半失败）。
- 假服务端：`answers` 覆盖 `PATCH /api/v0/nodes/*`（照 `:293-301`）。
- 牵连：S5 改同一个文件（16.x），冲突小。

##### 19.1 没有子页的页也说"及其子页面"

- 现状：`W/pages/notebook/export-dialog.tsx:54-58` 有 `page` 就用 `transfer.exportPageTitle`（"…and its subpages?"）。带 `page` 的调用只有 `W/pages/page/page-menu.tsx:55-71`；`transfer-page.tsx:141` 是整个笔记本。
- 改法：
  - `const pages = usePageTree(notebook)`（`../../stores/context`）；`alone = page !== undefined && pages.tree !== undefined && childrenOf(pages.tree, page.id).length === 0`（`childrenOf` 从 `../../stores/page-tree`；`pages.tree` 只含页，`W/stores/page-tree.store.ts:179-182`）。
  - 标题：`alone` 时 `transfer.exportPageAloneTitle`（4.2.1.1），否则照旧。树没读到时照旧说"及其子页面"。组件已是 `observer`，树变了随之重渲染。
  - 附件照样随页导出，描述里已说"its pages and attachments"。
- 测试（`W/pages/page/page-menu.test.tsx`）：
  - `test.each([["en", "Export the page Notes?"], ["zh-CN", "导出页面「Notes」？"]])("a page without subpages is exported without saying its subpages, in %s")`：`renderApp(pagePath(notes.id), server.app)`（`notes` 在根下、没有子页），点菜单的"Export this page"，`findByRole("alertdialog", { name })`。没修的代码上名字带"and its subpages"，超时失败。
  - 已有的 `exporting()`（`:18-27`，Guide 有子页 Install）照旧断言"…and its subpages?"，是对照。
  - 替身不改：`W/test/jobs-server.ts:123,432` 建在 `pageServer` 上，默认的 `notes` 在根下、没有子页。
- 变异：恒用 `exportPageTitle`（新格失败）；条件取反（Guide 那一格失败）。实际另有一条"树没读到当作没有子页"，起初活下来（审查 B 的 N1、C 的 M1）：页的菜单要等树读到才出现，修复第 1 轮的一格经探针路由打开对话框，`nodesDown` 时标题仍是"…and its subpages?"（10.2）。
- 牵连：e2e `tr1-export.spec.ts:261`（Café 下有 One）不变。P3/S3 引同一个文件的 `:76-80`（`texts` 不覆盖 `transfer.too_many_jobs`）并在 `page-menu.test.tsx` 加一格；本项在 `:54-58` 加几行，P3 引的行号随之后移。文本冲突、语义不冲突，后合并的先 `git merge main`。

##### 21.8 只差末尾换行显示 −x/+x

- 现状：`W/editor/text-diff.tsx:86-90` 的 `breaksOnly` 只认"换行写法或 BOM 不同、拆开之后相同"。`a\nb` 对 `a\nb\n` 拆开之后仍不同，`lineDiff`（`W/lib/line-diff.ts:45-48`：没有换行的最后一行与带换行的不同）画出 `−b`、`+b`。版本比较、还原的确认框、冲突面板都用 `TextDiff`。
  - 移交说"HI1 的方向断言钉住了这种画法"：`hi1-versions.spec.ts:278-287` 断言的是加了一行"First"，不是"只差末尾换行"，不受影响；本改法也保留画法。
- 改法：
  - 说明照**显示着的比较**算，不照 `split`：`shows()` 的最后（`:190-197`）渲染 `UnifiedDiff` 之前，`lastBreak = compared.a + "\n" === compared.b ? "added" : compared.b + "\n" === compared.a ? "removed" : undefined`。
  - 理由（审查 B 的 N5）：大文本在 worker 里比较、`busy` 时，显示的是上一次的比较（`compared` 是 `later`），`split` 已是新文本的；照 `split` 算，说明会与看到的比较不符。
  - `UnifiedDiffProps`（`:285`）加 `note?: string`，`UnifiedDiff`（`:299`）在计数那一行之下、视图之上渲染 `<p className="text-sm text-muted-foreground">`；`lastBreak` 选 `diff.lastBreakAdded` 或 `diff.lastBreakRemoved`（4.2.1.1）。视图、计数、上一处下一处照旧。
- 测试（`W/editor/text-diff.test.tsx`；`diff()` 帮手（`:14-25`）加一个 `locale` 参数，默认 `zh-CN`）：
  - `says when the only change is the line break at the end, either way, in either language`：`diff("a\nb", "a\nb\n")` 有 zh 的 `diff.lastBreakAdded`；`rerender` 成 `diff("a\nb\n", "a\nb")` 有 `diff.lastBreakRemoved`；en 两格同（"The only change is a line break added at the end."、"The only change is the line break at the end, removed."）。视图还在（一个 `.cm-editor`）。
  - 对照：`diff("a\nb\n", "a\nc\n")` 没有这两句（`queryByText(/末尾/)` 为空）。
  - `says the line break at the end only of the comparison it shows, while that of newer texts runs`：照 `:145-171` 用 `controlled()` 与 `big = "x\n".repeat(20_000)`。先 `` diff(`${big}y`, `${big}y\nz\n`, compare) ``，答 `asked[0]`，没有说明；`rerender` 成 `` diff(`${big}y`, `${big}y\n`, compare) ``，`busy` 时 `queryByText(/末尾/)` 为空；答 `asked[1]` 之后有 zh 的 `diff.lastBreakAdded`。
  - 没修的代码上找不到这句，`getByText` 抛出（审查 B 实跑：显示"+1 −1"，新句不在）。
- 变异：去掉 `note`（第一格失败）；`added`、`removed` 对调（方向两格失败）；只看 `compared.a + "\n" === compared.b`（removed 那格失败）；改照 `split` 算（`busy` 那一格失败）。
- 牵连：原文同时差换行写法（CRLF）与末尾换行时，只说末尾换行（推断，罕见，不另处理）。

##### 21.11 并排写"+0 −5000 行，太大"而行数在上限内

- 现状：`text-diff.tsx:172-185`：`result.kind === "tooLarge"`，或比较有结果但视图连成的一块删去的行多于 5000（`removedLines(state) > lineDiffLimits.maxChangedLines`，`:249-263`），都并排显示并说 `diff.tooLargeLines`；后一种的 `added + removed` 在上限之内，句子与数字矛盾。
- 改法：`:178-182` 三选一：`result.kind === "tooLarge"` 照旧（`added`、`removed` 为 `null` 时 `diff.tooLarge`，否则 `diff.tooLargeLines`）；`result.kind === "changes"` 时 `t("diff.tooLargeBlock", { added: result.added, removed: result.removed })`（4.2.1.1）。
- 测试（`text-diff.test.tsx`）：`:352-358` 那一格的期望改成 `"+0 −2501 行，但改动连成的一块太大，不能逐行显示。"`，加 en 一格 `"Lines: +0 −2501, but the changes join into a block too large to show line by line."`。没修的代码上是"+0 −2501 行，太大，不能逐行比较。"，失败。`:126`（`tooLarge` 带计数）不变，是对照。
- 随之改：`text-diff.test.tsx:357`。
- 变异：恒用 `tooLargeLines`（改过的格失败）；恒用 `tooLargeBlock`（`:126` 失败）。实际后一条红两格：`:126` 所在的一格与 "keeps texts too far apart…"（10.2）。

##### 22.11 根下的说法不统一（与 23.3 同改）

- 现状（两种语言）：
  - zh "笔记本首页"：`trash.fromTop`（`zh-CN.ts:252`）、`trash.toTop`（257）、`trash.restoredTop`（269）、`trash.restoredAsTop`（271）、`history.detail.top`（447）、`transfer.placedTop`（946）、`transfer.placedParentInTrash`（948）、`transfer.placedTooDeep`（949）；显示在 `trash-row.tsx:85,99,112`、`changeset-change.tsx:17`、`transfer-report.tsx:304,310,312`。
  - "根下"/"the top level"：`trash.toTopParentInTrash`、`trash.toTopTooDeep`（en 270-271，漏了"the notebook's"；zh 258-259）；显示在 `W/pages/notebook/trash-row.tsx:102`、`:104`。
  - "笔记本的顶层"/"the notebook's top level"：英文其余各处都是它；zh 的 `unresolved.atRoot`（93）、`page.moveRoot`（174）；显示在 `unresolved-link.tsx:280`、`parent-options.tsx:25`（移动与导入的"上级页面"）。
  - 第四种（I3 没列）："笔记本根目录"/"Notebook root"：`index.root`（en 328、zh 314），`index-rows.tsx:56`（"Without a type · Notebook root"）。
  - LLM Wiki 的"根下"/"at the root"（`unresolved.notResolvable`、`upgrade.startBody`、`upgrade.startRaw`、`upgrade.gate.agents`、`upgrade.gate.source_root`、`upgrade.gate.reserved_titles`、`upgrade.gate.frontmatter`、`problem.wiki.upgrade_title_taken`、`problem.page.virtual_file`）不改：它们讲的是导出的库的根上的文件，与 `AGENTS` 模板、MCP 的 prompt、README 同一组词。
  - 用户：中文的回收站里一项写"原位置：笔记本首页"，另一项写"恢复到根下"；移动对话框说"笔记本的顶层"。"笔记本首页"是一页真的页面（`notebook-home.tsx`），会读成"放进首页"。
- 已定：2.3 的 P2-9（统一成"笔记本的顶层"/"the notebook's top level"，`index.root` 一并）。
- 改法：只改 4.2.1.2 里 22.11、23.3 的十一个键：zh 十个（其中 `trash.toTopParentInTrash`、`trash.toTopTooDeep` 两种语言都改），加 `index.root` 两种语言。`index.root` 的 en 写 `The notebook's top level`（与 `page.moveRoot` 同，大写开头）。组件不变。
- 测试：
  - `W/pages/notebook/trash-page.test.tsx:96-118` 的期望改成"Restores at the notebook's top level: …"两句（没修的代码上是"Restores at the top level: …"，失败）。
  - 加 zh 一格 `in Chinese, the trash says the notebook's top level for where an item was and where it goes back`：复用 `:96` 的布置，三行分别是 `trash.toTopTooDeep`、"恢复到「Guide」下"、`trash.toTopParentInTrash` 的新中文；再用 `:61` 那种布置断言根下一项的 `trash.fromTop`、`trash.toTop`。没修的代码上是"根下""笔记本首页"，失败。
  - `trash.restoredTop`、`restoredAsTop` 的 zh：在恢复根下一项的那一格（`:495` 是 en 的 `restoredAsTop`）加 zh 版，断言状态行"已恢复为「Notes 2」，在笔记本的顶层：那里已有同名的页面或附件。打开"。
  - `transfer.placed*` 的 zh：`W/pages/notebook/transfer-background.test.tsx:176-200` 的表加 zh 三行（推断有 locale 的口子，4.2.5）。
  - `index.root`：`W/pages/notebook/index-page.test.tsx` 加一格，父页为 `null`、无类型的项，分组标题"Without a type · The notebook's top level"，zh"无类型 · 笔记本的顶层"（今天没有测试覆盖这个键）。
- 随之改：`trash-page.test.tsx:115`、`:117`；`W/pages/notebook/changeset-page.test.tsx:164`（"从（已清理）移到笔记本首页" → "…笔记本的顶层"）；e2e `tb1-trash.spec.ts:315-317`。
- 变异：任一 zh 键改回"首页"或"根下"，对应格失败；en 的两个改回"the top level"，`trash-page.test.tsx:96` 那格失败。
- 牵连：
  - 9.5 的 `page.nameAtTop` 用同一个词，同一个提交。
  - 本项改 `trash-row.tsx:102`、`:104` 显示的说法（组件不改）；P3/S3 改同一个文件的 `offeredOf`（P3 设计 4.2，决定 377），并引 `refusedText`。文本冲突（翻译文件）、语义不冲突，后合并的先 `git merge main`。

##### 22.12 `restore_expired` 说两遍

- 现状：`W/pages/notebook/trash-preview.tsx:212-215`：从预览恢复被拒为 `page.restore_expired`，`shell(told, <Gone text={t("trash.expiredItem")} …/>)`：上面的 `Alert` 是 `trash.expired`（`trash-row.tsx:131-132`），下面的正文是 `trash.expiredItem`，同一件事两遍；`h1` 经 `aria-describedby` 读 alert，读屏再读正文。
- 改法：`Gone`（`:440-476`）的 `text` 改成可选，`{text !== undefined && <p>{text}</p>}`；`:215` 改为 `text={told === undefined ? t("trash.expiredItem") : undefined}`。直接打开一个过期的项（没有本页的拒绝）时照旧说正文。文字不变。
- 测试（`trash-preview.test.tsx:351-381` 的 `test.each`）：
  - `past its time` 一行的 `text` 改成 `undefined`；测试体先 `await screen.findByRole("alert")`，再在 `text !== undefined` 时 `findByText(text)`，否则断言 `trash.expiredItem` 的英文句 `queryByText` 为空；其余断言（alert 整句、`h1` 取得焦点、`aria-describedby`）不变。没修的代码上否定断言失败（正文在）。
  - 直接打开一个过期的项，正文仍在、没有 alert（推断已有的 `:225-231` 那格就是，不变）。
  - 否定断言之前先等 alert 出现（05-lessons 第 6 节）。
- 随之改：`trash-preview.test.tsx:364-370` 那一行与 `:376`。
- 变异：`text` 恒传 `expiredItem`（否定断言失败）；恒不传（直接打开的那格失败）。
- 牵连：S6 的 22.14 改同一个文件的 `restoring` 一段；本项只碰 `:215` 与 `Gone`，冲突小。

##### 22.13 别的笔记本的 `not_in_trash` 先说"已不在"

- 现状：`trash-preview.tsx:208-211` 对 `page.not_in_trash` 渲染 `NotInTrash`（`:384-431`）：它读树（`:399-415`），读到且没有这个节点时换成 404（`:417-419`）；读到之前、读失败时一直是 `trash.notInTrash`"This is no longer in the trash."。手写别的笔记本的节点的预览地址，先看到"已不在"，再变成 404；树读不到时一直说"已不在"。
- 改法：`:423` 按树里有没有这个节点选：`node === undefined` 时 `trash.notInTrashUnknown`（4.2.1.1，中性：对"本笔记本里恢复了"与"不是本笔记本的"都对），否则 `trash.notInTrash`。读到且没有照旧 404。
- 取舍（2.3 的 P2-49）：另一种是读到之前显示"加载中"、读失败才说中性的一句，改动略大，不取。
- 测试（`trash-preview.test.tsx`）：
  - `:431-441`（树读不到）改成 `test.each(["en", "zh-CN"])`，断言 `findByText` 新键的句子，且 `queryByText("This is no longer in the trash.")` 为空。没修的代码上 `findByText` 超时失败。
  - `:187-195`（别的笔记本）加一个中间断言：树的读扣住时说新键的句子、不说"no longer"；放开之后 404。没修的代码上中间断言失败。扣住树的读照 `:271-305` 的写法：`` trashed("admin", { [`GET /api/v0/notebooks/${notebookJSON.id}/nodes`]: async () => { await tree.waits; return json({ data: … }); } }) ``。
  - `:197-212`（本笔记本里恢复了、树里有）不变，是对照。
- 随之改：
  - `trash-preview.test.tsx:437`。
  - `:303` 的否定断言（恢复在途、树的读扣住时不说"已不在"）同时断言 `queryByText("This is not in the trash.")` 为空：那一刻树里还没有这个节点，修过的代码若显示它，说的正是新句。
  - `:203`、`:361`、`:427`、`:534`、`:572` 推断不变（4.2.5）。
  - e2e `tb1-trash.spec.ts:369-376` 的观察者（4.2.3.1）。
- 变异：恒用 `trash.notInTrash`（改过的格失败）；恒用新键（`:197` 那格失败）。
- 牵连：S6 改同一个文件。

##### 22.16 页面从别处恢复后失锁的编辑器仍说"已不在"

- 现状：`W/stores/edit-session.ts:83-85`：保存、心跳答 404 `page.not_found` 记 `{ reason: "gone" }`；`W/pages/page/edit-lost-banner.tsx:56-57` 显示 `editor.lostGone`。页面从回收站恢复之后横幅仍说"已不在"。404 还包括笔记本被删、失去角色（`page.yaml:515-517`）。移交引的中文与实际的不同（I3 已记）。
- 已定：2.3 的 P2-13（只改说法，重开会话进 v0.2）。
- 改法：只改 `editor.lostGone`（4.2.1.2）：写成编辑期间发生的事，之后恢复仍然对，并包含"无权访问"。组件不变。
- 测试：`W/pages/page/page-lock.test.tsx:333-341` 一行的期望改成新句（en 照审查 B 的 N4）；加 zh 一格（`setLocale("zh-CN")`，`lapsed, the page gone`，横幅含 zh 的新句）。没修的代码上 `toContain(why)` 失败。
- 随之改：`page-lock.test.tsx:340`；e2e `c4-guard.spec.ts:236`、`:273`（4.2.3.1）。
- 变异：改回旧句（两格失败）。

##### 23.1 英文 revert/undo 两种说法；"一部分"在全被清理时也说

- 现状：
  - 网页英文的"revert"在 `en.ts:800`（`upgrade.approveBody`）、`:957`（`transfer.failure.changeset_reverted`）、`:1267`（`problem.page.changeset_trimmed`）、`:1268`（`revert_too_large`）、`:1269`（`revert_conflict`）；别处都是"undo"（按钮、`history.*`、`transfer.revert*`、`transfer.restoreFailure.changeset_reverted`）。中文一律"撤销"。
  - "一部分"：`problem.page.changeset_trimmed`（en 1267、zh 1202）、`transfer.failure.changeset_trimmed`（en 962、zh 923）、`history.trimmedNote`（en 442-443、zh 428）；清理拿走全部时服务端也答 `page.changeset_trimmed`（`page.yaml:1052`、`:1904-1907`）。`trimmedNote` 还说"不能整组撤销"，而 `trimmed` 的变更集根本不能撤销（`changeset-page.tsx:221` 起不给按钮）。
- 已定：2.3 的 P2-10。服务端 `page/domain/errors.go:85-97` 的文字不改：它们是 API 的词（`revertChangeset`、`page.revert_*`、MCP 的历史工具），网页从不显示服务端的 `title`/`detail`（`problemMessages`，13.2 第 11 条）；"…reverted whole"在全部被清理时同样不准，记进移交，P3 或 v0.2 改。
- 改法：只改 4.2.1.2 里 23.1 的七个键：英文一律"undo"，"一部分"改成"部分或全部"，`trimmedNote` 去掉"整组"。`upgrade.approveBody` 的英文把"changesets"改成"changes"；中文的"变更集"同改为"改动"（2.3 的 P2-48）。
- 测试：
  - `W/pages/notebook/revert-dialog.test.tsx:366-386`（`trimmed`）与 `:388-401`：断言改成新句；加 zh 一格（同一布置），alert 是 zh 的 `problem.page.changeset_trimmed`，页上是 zh 的 `history.trimmedNote`。
  - `revert_too_large`、`revert_conflict` 的英文：今天推断没有测试断言（`revert_too_large` 在对话框里被 `history.backgroundTooLarge` 顶替，`revert_conflict` 由 `Conflicts` 接手）。`W/app/problem-messages.test.ts:225-233` 那种按语言的表加两行：`errorText(new ApiError(…, "page.revert_too_large"), en)` 等于新句，`revert_conflict` 同；zh 两行不变，作对照。
  - `transfer.failure.changeset_reverted`：`W/pages/notebook/transfer-report.test.ts:25` 改成新句。
  - 没修的代码上各处整句比对失败。
- 随之改：`revert-dialog.test.tsx:377`、`:381`、`:400`；`W/pages/notebook/revert-background.test.tsx:227`；`changeset-page.test.tsx:280`；`transfer-report.test.ts:25`、`:43`；`W/pages/notebook/transfer-import.test.tsx:450`；`transfer-background.test.tsx:374`、`:384`；`W/pages/notebook/upgrade-page.test.tsx:1357`（`approveBody` 帮手；zh 若改，`:1472` 的 `toContain` 只取前半句，不受影响）。
- 变异：任一键改回"revert"或"Part of"，对应格失败。实际：中文的 `transfer.failure.changeset_trimmed` 改回照过，第 10 个提交补格（10.2）。
- 牵连：
  - 23.4、24.7、23.11 同一组文件。
  - 与 P3/S3：本项改 `transfer.failure.*` 的文字；P3/S3 给 `W/pages/notebook/transfer-report.tsx:92-108` 的 `problems` 加键（两种语言的 `transfer.problem.*`），并在 `transfer-report.test.ts`、`revert-background.test.tsx`、`transfer-import.test.tsx`、`problem-messages.test.ts` 各加一格。文本冲突、语义不冲突，后合并的先 `git merge main`。

##### 23.3 根下：首页对顶层

见 22.11（同一个改法、同一组测试）。I3 列的 `zh:447,252,257,269,271,946-949` 与 `zh:93,174` 都在 22.11 里。

##### 23.4 `parent_gone` 链到列表；英文含糊

- 现状：`W/pages/notebook/changeset-text.ts:144-145` 的 `history.conflict.parent_gone`："The page “{name}” was under is gone: …"，读者先读到"The page “X”"、以为 X 不在了；`revert-dialog.tsx:466-474` 的链接名 `history.conflictTrashLabel` 同样含糊；链接去 `${home}/trash`（列表）：冲突只给节点、名称与原因（`RevertConflict`），不给上级。中文不含糊。
- 已定：2.3 的 P2-13（只改英文两句；链到那一项要服务端点名上级，进 v0.2）。
- 改法：只改英文两句（4.2.1.2）。
- 测试（`revert-dialog.test.tsx:187-245`）：`:206` 改成 `"The page that “Linux” was under is gone: it is in the trash, or purged."`，`:215` 的链接名改成 `"Trash: the page that “Linux” was under may be there"`。没修的代码上整句比对失败。
- 随之改：`revert-dialog.test.tsx:206`、`:215`。`transfer-report.tsx` 的冲突列表推断也用 `reasonText`；实施者 grep `was under is gone`（今天只有 `:206` 一处）。
- 变异：改回旧句。

##### 23.11 别处已撤销后再撤销，提示改回原名

- 现状：`revert-dialog.tsx:279-285`：撤销答 `page.revert_conflict` 时只 `transfers.load()` 后 `setConflict(refusal)`，不重读这次改动；`Conflicts`（`:396-513`）在 `changeset.renames` 时显示 `history.conflictRenameBack`（`:509`），不看 `reverted_by`。别的标签页已把这次改名撤销，这里再按"撤销"：冲突说"“Beta” changed since."，又建议"rename it back"，而它已经回去了。
- 改法：
  - `refused`（`:279-285`）在 `setConflict(refusal)` 之前加 `reread(true)`：重读这次改动（`["changeset", notebook.id, id]`）、它的节点与历史的列表。页上的 `ChangesetPage`（`changeset-page.tsx:94`）拿到新的 `reverted_by`，经 `RevertDialog`、`RevertForm` 传给 `Conflicts`。审查 B 实跑证实这条数据的路（加 `reread(true)` 与条件之后新格过，文件 44 格全过；去掉 `reread(true)` 新格失败）。实际：重读不等就 `setConflict`，重读回来之前（失败时一直）仍说 "rename it back"（审查 A 的 N4）；修复第 1 轮先 `await` 这次改动的重读，再 `reread(false)` 重读两份列表、不等，然后才显示冲突（10.2）。
  - `:509`：`changeset.reverted_by.length > 0` 时显示 `history.revertAgain`（已在 `changeset-page.tsx:235` 用，不加新键），否则照旧在 `renames` 时显示 `history.conflictRenameBack`。
  - 焦点不变（冲突的标题照旧取得，`:410-412`；13.2 第 26 条：重读不移焦点）。
- 测试（`revert-dialog.test.tsx`）：
  - `test.each(["en", "zh-CN"])("a rename undone elsewhere meanwhile, undone again, says it was undone before, not to rename it back, in %s")`：`server.log("rename", { changes: [renamed(...)] })`（`renames` 为真，`W/test/history-server.ts:403`），`conflicts = [conflict(id, "Beta", "changed")]`；页面读完之后 `changeset.revertedBy.push(otherId)`（`:317`，模拟别的标签页）；打开对话框按"Undo"。断言有 `history.revertAgain` 的整句，`queryByText(/rename it back/)`（zh：`/改回原名/`）为空。
  - 没修的代码上对话框里是"rename it back"，没有 `revertAgain`，失败。
  - 对照：`:187-245` 那一格（`reverted_by` 空）照旧有"rename it back"；`:233`、`:269`、`:289` 照旧。
- 变异：去掉 `reread(true)`（新格失败）；条件恒假（新格失败）；条件恒真（`:233` 失败）。实际：条件恒真红的是 `:187-245` 那一格（含 `:233`）；修复第 1 轮另加"不等重读"一条（10.2）。
- 牵连：S1 改这个文件的说明（24.3、49）与 `ConfirmDialog`；本项只碰 `:279-285` 与 `:509`。

##### 23.12 "1,000 多 个节点"多一个空格

- 现状：`changeset-text.ts:86-90` 的 `countText` 在上限时用 `history.atLeast`，中文 `"{count} 多"`（`zh-CN.ts:390`），代入"共 {nodes} 个节点、…"（`:389`）得"共 1,000 多 个节点"；`history.revertBody`（`:451-452`）与 `history.revertTooLarge`（`:457-458`）同。比 I3 列的多两处。
- 改法：zh 的 `history.atLeast` 改成 `{count}+`（与 en 及 zh 的 `search.capped` 一致）。组件不变。
- 取舍（2.3 的 P2-50）：另一种是改模板去掉"个"前的空格，会波及非上限时的写法，不取。
- 测试：
  - `W/pages/notebook/history-page.test.tsx:221`（`:202-221` 那一格）改成"共 1,000+ 个节点、999 篇正文"。
  - `revert-dialog.test.tsx` 加 zh 一格：`node_count` 为 1000 的改动（`W/test/history-server.ts` 的计数要能给到 1000，照 `history-page.test.tsx:202-221`），对话框描述以"1,000+ 个节点与"开头。这个文件今天没有上限的格（查过）。
  - 没修的代码上是"1,000 多 个节点"，失败。
- 随之改：`history-page.test.tsx:221`。
- 变异：改回"{count} 多"。

##### 23.13 附件也写"页面："

- 现状：`W/pages/notebook/history-filters.tsx:116-117`：`page = node?.name ?? (pages.nodes === undefined ? "…" : t("history.pageDeleted"))`；`PageFilter`（`:166-189`）写 `history.page`（`:177`），去掉的按钮名 `history.removePageLabel`（`:182`）。筛选按节点（契约 `page_id` 包括附件，README:177），"这个节点的历史"从附件也来（`changeset-change.tsx:122`）。附件写成"Page: a.png"，已删除的写成"Page: a deleted page"。
- 改法：
  - `:116-117` 改成算出 `label`：`node` 在时按 `node.kind === "asset"` 选 `history.attachment` 或 `history.page`（参数 `name: node.name`）；`pages.nodes === undefined` 时 `"…"`；否则 `history.nodeDeleted`。
  - `PageFilter` 的 prop 由 `name` 改成 `label`，`:177` 直接写 `label`，`:182` 写 `t("history.removeFilterLabel", { filter: label })`。`pages.nodes` 含附件（`page-tree.store.ts:78`）。
  - 新键与删键见 4.2.1。`history.page` 不变，"Remove the filter “Page: Guide”"整句不变。
- 测试（`history-page.test.tsx`）：
  - `:1010-1019` 改成 `getByText("Attachment: a.png")` 与按钮名"Remove the filter “Attachment: a.png”"；加 zh 一格（"附件：a.png"、"去掉筛选「附件：a.png」"）。没修的代码上是"Page: a.png"，失败。
  - `:615-623` 改成 `history.nodeDeleted` 的英文，加 zh 一格。
  - `:766-774`（树没读到）断言筛选那一段以"…"开头（例如 `getByRole("button", { name: "Remove the filter “…”" }).closest("p")?.textContent`），不再是"Page: …"。
- 随之改：`history-page.test.tsx:622`、`:773`、`:1018`。页面的 `:603-604`、`:744`、`:831`、`W/pages/page/page-menu.test.tsx:114`、e2e `hi3-log-revert.spec.ts:338` 不变。
- 变异：去掉 `asset` 的分支（附件格失败）；`removeFilterLabel` 的 `filter` 传 `node.name`（按钮名的格失败）。
- 牵连：S6（历史与目录）、S3（历史的筛选在 375 px 上超宽，32.x）改同一个文件的布局，冲突小。

##### 24.7 `changeset_reverted` 一句读来像同一项

- 现状：`W/pages/notebook/transfer-report.tsx:173-175`：后台恢复因 `changeset_reverted` 失败时说 `transfer.restoreLeftUndone`，读来像已恢复的与没处理到的在同一项；实际再删除的部分是撤销时新的一项（或几项），没处理到的各自成项（移交第 24 条）。
- 改法：只改文字（4.2.1.2）：分开说两样，各自在哪。
- 测试：`transfer-background.test.tsx:314-323` 那一行改成新句；加 zh 一格（推断那张表经 `renderApp` 用 en），断言报告里 zh 的新句整句。没修的代码上失败。
- 随之改：`transfer-background.test.tsx:322`。
- 变异：改回旧句。
- 牵连：`transfer.restoreLeft*` 与 P3/S3 的 `transfer-report.tsx:92-108` 同一组文件，见 23.1 的牵连。

##### 26.11 "在左栏顶部的搜索框里……"在窄屏上不对

- 现状：`W/pages/search/search-page.tsx:241-242` 在没有提交时显示 `search.howTo`（"…at the top of the left column…"）。搜索框在 `W/pages/workspace/workspace-layout.tsx:108-113` 的左栏里，小于 `md` 时左栏在正文之上（`border-b`）。
- 改法：只改文字（4.2.1.2）："在顶部的搜索框里"：宽屏时在左栏的顶上、也在窗口的顶上，窄屏时在页首，两种都对。
- 测试：`W/pages/search/search-page.test.tsx:626-631` 改成新的 en；加 zh 一格（`setLocale("zh-CN")`）。没修的代码上失败。文字不随宽度变，不需要 e2e。
- 随之改：`search-page.test.tsx:630`。
- 变异：改回旧句。

##### 26.13 "可以继续查找"而按钮仍是"更多结果"

- 现状：`search-page.tsx:201`：`lastEmpty = data?.at(-1)?.data.length === 0` 看原始的最后一页。`:148-158`："更多"读到的一页只有已列出的（改名在两页之间），`added` 为空，播报 `search.noneHere`"…you can keep looking."，而那一页原始不空，按钮仍是 `search.more`"More results"（`:267`）。
- 改法：`:201` 改为"最后一页没有新的"：

  ```ts
  const earlier = new Set((data ?? []).slice(0, -1).flatMap((read) => read.data.map((hit) => hit.node.id)));
  const lastEmpty = (data?.at(-1)?.data ?? []).every((hit) => earlier.has(hit.node.id)) && data !== undefined && data.length > 0;
  ```

  空的最后一页照旧为真；第一页不空时为假。文字不变。
- 测试（`search-page.test.tsx`，照 `:323-363` 的布置）`after a stretch that listed only hits listed before, More is Keep looking, as what it said`：第一页 `[guide, notes]`、游标 `second`；第二页 `[notes]`、游标 `third`；第三页 `[linux]`、游标 `null`。按"More results"之后断言 `told()` 是 `search.noneHere` 的英文，`getByRole("button", { name: "Keep looking" })` 在、`document.activeElement` 是它。没修的代码上按钮名是"More results"，`getByRole` 抛出（审查 B 实跑：新格在没修的代码上失败，修了之后文件 30 格都过）。
- 变异：
  - 改回原始的 `length === 0`：新格失败。
  - `every` 改 `some`：已有的 `:366` 那一格失败。空的最后一页 `[].some()` 为假，按钮不再是"Keep looking"（审查 B 实跑：`:366` 与对照格同时失败）。不必为它另加格。
- 牵连：S6（搜索）改同一个文件。

##### 26.14 任何一页说 `building` 就一直说

- 现状：`W/pages/search/search-status.ts:30`：`pages.some((page) => page.building)`，读过的任何一页说索引在建，状态行就一直加"The index is being built; …"，索引追上之后也不变。
- 改法：`:30` 改成 `pages.at(-1)?.building === true`（最新读到的一页定）；`statusText` 的注释（`:9-18`）随之改。文字不变。
- 测试（`W/pages/search/search-status.test.ts:65-75`）：加两格：`statusText([page(20, true, { value: 30, kind: "about" }, true), page(10, false, null, false)], 30, en, "en")` 等于"Results: 30."（没修的代码上带"The index is being built…"，失败）；zh 同一组等于"约 30 条。"（实际是"30 条。"：最后一页读到底，计数是找到的 30；设计的预期错，审查 B 的 N2，10.2）。`:71-73`（最后一页在建）照旧，是对照。
- 变异：改回 `some`（新格失败）；改成 `pages[0]`（新格与 `:71` 失败）。

##### 35.1 英文单复数（"1 sources" 等）

- 现状与清点见 4.2.2。本项改 `index.sources`、`index.longTypes`、`index.loaded`、`index.inbound`、`health.added`、`health.referringPages`、`health.stale`。
- 改法（每处一个三元，照 `health-rule.tsx:59-61`；新键见 4.2.1.1）：
  - `W/pages/notebook/index-page.tsx:68-71`：`current.unfilterable_type_pages === 1`。
  - `index-page.tsx:256`：`items.length === 1`。
  - `index-page.tsx:270-273`：`area.sources === 1` 时 `t("index.sourcesOne", { path })`（`path` 先算出来）。
  - `W/pages/notebook/index-rows.tsx:99-101`：`item.inbound_capped` 先判，再 `item.inbound === 1`。
  - `W/pages/notebook/health-rule.tsx:170`：`added.length === 1`。
  - `W/pages/notebook/health-item.tsx:84`：`item.pages === 1`。
  - `health.stale` 两处（`health-page.tsx:295`、`upgrade-check.tsx:458`）：`health-rule.tsx` 导出 `staleText(t: Translate, locale: Locale, count: number): string`（`count === 1` 时 `health.staleOne`，否则 `health.stale` 带 `Intl.NumberFormat(locale)` 格式化的数），两处改用它。
- 测试（每个键 1 与 2（或 0）两格，en 与 zh）：
  - `index.sources`：`W/pages/notebook/index-page.test.tsx:115-118` 的 `test.each` 加 `{ sources: 1, label: "Guide/raw.md · 1 source" }`（没修：`· 1 sources`，`findByRole` 超时）；已有 0、2 两行。zh 加一格，链接名"Guide/raw.md · 1个来源"。
  - `index.loaded`：`:31-46` 是 3；加只有一页的目录一格，`status` 是新键的句子，zh 同。
  - `index.longTypes`：`:218`（7）是 2 的那格；加 `unfilterable_type_pages: 1` 一格，zh 同。
  - `index.inbound`：`:141` 的夹具（`indexFacts`，推断在同文件或 `W/test/`）加 `inbound: 1` 的一项，断言"1 incoming link"，zh"1个入链"；`:141`"2 incoming links"是 2 的那格。
  - `health.added`：`W/pages/notebook/health-rule.test.tsx` 加"更多"只加一项的一格；`:179`（0）是另一格；zh 同。
  - `health.referringPages`：`health-rule.test.tsx:556`、`:565` 改成"Missing 00nReferenced by 1 page"（这两格就是 1）；`health-page.test.tsx:268-269`（3、2）是另一格；zh 加到 `health-page.test.tsx:361-373` 的中文格："被 1 篇页面引用"与"被 2 篇页面引用"（推断夹具里有 2，没有就加一项）。
  - `health.stale`：`health-page.test.tsx:291`（7）、`:372`（zh 7）是 2 的那格；加 en、zh 各一格 `stale_pages: 1`。`upgrade-check` 的一处随 41.2 的格测。
  - 没修的代码上每个 1 的 en 格是复数的旧句，整句比对失败；zh 的 1 格在没修的代码上也会过，它测的是新键的中文，不是变异的证据。
- 随之改：`health-rule.test.tsx:556`、`:565`。
- 变异：每处去掉 `=== 1` 的分支（对应的 1 格失败）；两支对调（2 的那格失败）；`staleText` 只在一个文件用（41.2 的 1 格失败）。
- 假服务端：夹具要能给出 1：`indexServer` 的 `value`（`index-page.test.tsx:130-134` 已这样改 `source_area`）、健康度的 `summary`/`pageOf`、升级的 `checkOf`（`upgrade-check.test.tsx:201-216` 已带 `worse`、`stale` 参数）。
- 牵连：41.x。

##### 38.6 5 MB 对 5 MiB

- 现状：`W/i18n/format.ts:50-71` 的 `formatBytes` 按 1024 进位（`:59`），单位写 `KB`、`MB`（`:50`），注释写"such as 1.5 MB"。上传超过上限的 `.md` 来源（`W/pages/page/upload-rows.tsx:262-277`）说"…: 5 MB at most."；粘贴超过时 `field.content.too_long` 说"over 5 MiB"（`en.ts:1309`、`zh-CN.ts:1240`）；README 一律 MiB（`:131`、`:140`、`:145`）。同一个上限，两种说法。
- 已定：2.3 的 P2-11（甲）。不取乙（只把 `field.content.too_long` 改成"5 MB"）：仍是按 1024 算却标 MB，且与 README 不一致。
- 改法：`format.ts:50` 的单位改成 `["B", "KiB", "MiB", "GiB", "TiB"]`，`:54` 与 `formatBytes` 的注释改成"such as 1.5 MiB"。两种语言同。附件、导出包、导入的上限、无主笔记本、搜索与回收站里的大小都随之写 KiB、MiB。
- 测试：`W/i18n/format.test.ts:20-27` 的表改成 `1 KiB`、`1.5 KiB`、`5 GiB`、`2,048 TiB`、zh `1.5 KiB`（没修的代码上失败）；加 `5 * 1024 ** 2` 为 `5 MiB` 一格。`source-uploads.test.tsx:142` 断言"…: 5 MiB at most."，与 `W/pages/page/page-edit.test.tsx:374` 的"over 5 MiB"一致。
- 随之改（10 个测试文件 19 行，都是 `KB`、`MB` 换成 `KiB`、`MiB`；实际 11 个文件 23 行，另有 `upgrade-page.test.tsx:1138-1142`，10.2）：`format.test.ts:22-26`；`trash-preview.test.tsx:101`；`transfer-import.test.tsx:57`；`ownerless-page.test.tsx:135`；`attachment-uploads.test.tsx:296`、`:312`、`:412`；`search-page.test.tsx:116`、`:140`；`reading/assets.test.ts:161`；`transfer-page.test.tsx:92`、`:96`、`:189`；`attachments-section.test.tsx:49`；`source-uploads.test.tsx:142`。e2e 只断言了 `B`，不改。
- 变异：单位改回 `MB`（`format.test.ts` 失败）；进位改成 1000（`1 KiB` 那格失败）。实际 `1 KiB`、`1.5 KiB` 照过，红的是 `1,023 B`、`5 MiB`、`5 GiB`、`2,048 TiB`（10.2）。

##### 38.8 `source.historyRestore` 的行话"临时步骤"

- 现状：`en.ts:165-166`、`zh-CN.ts:158-159` 的第一句"Temporary steps must also follow the source rules."；LLM Wiki 里撤销、还原被来源的规则拒绝（`page.source_immutable`）时由 `W/pages/notebook/duplicate-text.tsx:256-264` 的 `HistoryAdvice` 显示。"临时步骤"指撤销分轮时把节点暂时改名、移到根下（README:179），用户不知道。
- 改法：只换第一句（4.2.1.2），后三句不变（`lw6-source-history.spec.ts:485-486` 断言"outside the source area"与"Administrators"）。
- 测试（`W/pages/notebook/duplicate-text.test.tsx`，今天没有测试读这段话）：`test.each(["en", "zh-CN"])("a history write the source rules refused says each step follows them, in plain words, in %s")`：照 `:40-60` 的 `probe` 调 `sourceHistoryText(new ApiError(409, { code: "page.source_immutable" }), pages, navigate)`，整句比对 `HistoryAdvice` 的 `span`（四句）。另一格 `page.source_admin_only`：不含这段（`content` 为假）。没修的代码上第一句是"Temporary steps…"，失败。
- 随之改：e2e `lw6-source-history.spec.ts:439`（4.2.3.1）。
- 变异：改回旧句（新格失败）；`HistoryAdvice` 的 `content` 恒真（`source_admin_only` 那格与 e2e 的否定断言失败）。

##### 40 升级中的 `AGENTS` 写"三个 prompt"

- 组件：`wiki/domain/template-en-agents.md:33`、`wiki/domain/template-zh-CN-agents.md:33`，各改一行。新句、为什么这样写、三个守卫与不改的测试，全文见 3.1。
- 测试：`wiki/domain/template_test.go` 加 `TestTheAGENTSTemplatesNameThePromptsAsServed`（修复第 1 轮改名为 `TestTheAGENTSTemplatesNameTheUpgradePrompt`，10.2），两种语言各断言含新的整行、不含旧句。没修的代码上两种语言都失败。
- 变异：任一语言改回旧句（新测试失败）；中文少写 `upgrade` 的反引号（`TestTheTemplatesCorrespond` 失败）。

##### 41.1 "No issues (1 checks)"

- 现状：`health.clear`（`en.ts:346`）三处：`W/pages/notebook/health-page.tsx:306`、`upgrade-check.tsx:252`（检查一节）、`:383`（审阅的报告）。普通笔记本 4 条规则、3 条有问题时写"No issues (1 checks)"。`upgrade-page.test.tsx:1113` 把它写进了断言。
- 改法：`health-rule.tsx` 导出 `clearText(t: Translate, count: number): string`（`count === 1` 时 `health.clearOne`），三处改用。
- 测试：`upgrade-page.test.tsx:1113` 改成"No issues (1 check)"（审阅报告的 1 格；没修的代码上失败）。`health-page.test.tsx` 加 en 一格（只有一条规则计数为 0）与 zh 一格；`upgrade-check.test.tsx` 加一格 1（检查一节）。复数的格：`health-page.test.tsx:210`（3）、`:303`（6）、`:340`（8）、`upgrade-check.test.tsx:376`（4）。
- 随之改：`upgrade-page.test.tsx:1113`。
- 变异：三处里任一处不用帮手，那处的 1 格失败（所以三处各有一个 1 的格）。

##### 41.2 "1 pages lead to …"

- 现状：`upgrade-check.tsx:445-447`：闸门 6 不过时 `t("upgrade.gateLinksWorse", { count: … pages_worse })`；只有一页变坏时"1 pages lead to …"。
- 改法：`pages_worse === 1` 时 `upgrade.gateLinksWorseOne`（4.2.1.1）。
- 测试（`W/pages/notebook/upgrade-check.test.tsx`）：`test.each(["en", "zh-CN"])("a gate failed by one page says page, singular; so does an index stale on one page, in %s")`：`checkOf(first, { worse: 1, stale: 1 })`，闸门 6 下是 `upgrade.gateLinksWorseOne`、闸门 8 下是 `health.staleOne` 的整句。`:135-137` 等行（3、7、4、12）是复数的格。没修的代码上 en 是"1 pages lead…""1 pages have…"，失败。`:254`、`:266` 的模板行里没有 1，不改。
- 变异：去掉分支（新格失败）。

##### 41.3 "(1 nodes under it now)"

- 现状：`W/pages/notebook/general-page.tsx:228-231`：`underRaw > 0` 时 `t("upgrade.startRaw", { count })`；`raw` 下只有一个节点时"(1 nodes under it now)"。
- 改法：`underRaw === 1` 时 `upgrade.startRawOne`（4.2.1.1）。
- 测试（`W/pages/notebook/general-page.test.tsx:415-450`）：`test.each` 加一行 `["a page raw at the root with one node under it", [raw, rawPage], 1]`，期望的模板（`:443-447`）按 `count === 1` 选"(1 node under it now)"；加 zh 一格（照 `:452-460`，整句是 zh 的 `upgrade.startRawOne`）。没修的代码上 en 是"(1 nodes under it now)"，失败。e2e `up2-start-refused.spec.ts:136`（2 个）不变。
- 随之改：`general-page.test.tsx:443-447` 的期望式。
- 变异：去掉分支（新行失败）。

##### H4 的界面一句（移交 M8-history 第 4 条）

- 现状：网页上没有一句说永久删除不能做（README:181 已写）。回收站页 `W/pages/notebook/trash-page.tsx:181-192` 的标题下只有 `trash.hint`；版本列表 `W/pages/page/page-versions.tsx:93-95` 只有 `version.listHint`。
- 期限的事实：一项可以恢复到"删除的时刻加 `jobs.purge_retention` 减 1 小时"（README:45、:180，`trash_retention_seconds`），每行写着"Can be restored until {time}"（`trash-row.tsx:275`）；过了它列表不再列、恢复答 409 `page.restore_expired`；清理在删除之后满 `jobs.purge_retention` 才物理删除，每 `jobs.purge_interval`（默认 1 小时）跑一次（README:115）。所以"过了期限之后由系统删除"对，"到期立即删除"不对。活着的页的版本一直留着（README:180-181）。
- 已定：2.3 的 P2-14（不写天数、不写"立即"；版本一句不提时间与清理）。
- 改法：
  - 回收站：`trash-page.tsx:191` 之后、同一个 `div` 里加 `<p className="text-sm text-muted-foreground">{t("trash.purge")}</p>`，回收站为空时也显示。
  - 版本列表：`page-versions.tsx:95` 之后加 `<p className="text-xs text-muted-foreground">{t("version.kept")}</p>`（`NotLoaded` 分支 `:80-88` 不显示）。
  - 新键 `trash.purge`、`version.kept`（4.2.1.1）。不写版本号（没有"v0.1""目前"）。
- 测试：
  - `W/pages/page/page-versions.test.tsx:52-53` 之后断言 `within(section("Versions"))` 里有 `version.kept` 的英文整句，`:90` 之后断言中文那句。没修的代码上 `getByText` 抛出。
  - `W/pages/notebook/trash-page.test.tsx:61` 那一格断言有 `trash.purge` 的英文整句；22.11 加的 zh 格里断言中文那句；空的回收站（`:124`，"The trash is empty."）也断言它在。
- 变异：去掉任一 `<p>`（对应格失败）；回收站一句只在不空时显示（空的那格失败）。
- 牵连：P5/S3 写部署文档与发布说明（永久删除不能做、运维的变通）；功能进 v0.2 的 `M12-history.md`。移交 M8-history 的"处理进展"由 S2 加一行。界面不写变通。

### 4.3 S3 375 px、`bdi` 与对比度

S3 改三样：

- 布局的类：让 375 px 不超宽；
- 名字的方向隔离：新的共用件 `W/i18n/isolated.tsx`；
- 焦点环与次要文字的颜色。

文字一个不改。

- **本节的取舍**：P2-15 至 P2-19、P2-36、P2-37、P2-43（2.3）。P2-43 是审查之后加的：菜单与选项的高亮也在 S3 修。
- **e2e 的新东西**：一个只跑 `@narrow` 的 Playwright 项目 `webkit-narrow`（4.3.1.2），钉住它的检查 `tools/e2e-projects/check.mjs`（P2-44，5.1），与两个夹具：`E/fixtures/narrow.ts`、`E/fixtures/contrast.ts`（4.3.1.3）。
- **13.2 的新条**：第 30 条，写在 8.2。

#### 4.3.0 总表、键、e2e 与提交

##### 4.3.0.1 总表

| 编号 | 分类（I3 → 本 Step） | 测试 |
|---|---|---|
| 12.4 | B → B（含子项：菜单与选项的当前项，2.3 的 P2-43） | e2e，Chromium，亮暗两种：SE1 (page)、L5 (page) 加段；新夹具 `E/fixtures/contrast.ts` |
| 26.9 | B（部分）→ B | e2e：SE1 (page) `:272-275` 改、加 |
| 26.12 | B → B | vitest：新 `W/i18n/isolated.test.tsx`；`search-page.test.tsx`、`trash-page.test.tsx`、`history-page.test.tsx` |
| 26.17、32.1、32.2、35.8、38.12 | B（26.17 部分）→ B | e2e：MC3 新的一格（Chromium、WebKit） |
| 32.3 | D → D（随 32.1–32.2） | 就是 MC3 那一格 |
| 32.4 | B（I3 说可降 C，不降）→ B | vitest：`history-page.test.tsx`、`changeset-page.test.tsx` |
| 33.1 | M → S3 里排查 | 排查不进套件；复现了才把 SE1 的 375 一段拆成 `@narrow` 的一格 |
| 38.10、38.11 | B → B | e2e：PG2 新的一格（Chromium、WebKit）；LW3 (page) 加一段（三种） |

##### 4.3.0.2 i18n 键

- 不加、不改、不删。本 Step 只改样式、颜色与 DOM 结构（`<bdi>`），文字全部不变，13.2 第 4 条没有新的要求。
- `bdi` 的单元测试仍放一格 zh-CN，证明名字跟着译文的语序走（26.12、32.4）。

##### 4.3.0.3 改到的 e2e 故事、浏览器、用时

`E/playwright.config.ts` 的 `projects` 加一项（2.3 的 P2-15，4.3.1.2）：

```ts
  // The checks at a phone's width (WCAG 1.4.10) run on Safari's engine too, whose widths differ from Chromium's
  // (M12/P2/S3): those tagged @narrow, of the stories the three browsers do not run already.
  { name: "webkit-narrow", grep: /@narrow/, testIgnore: threeBrowsers, use: { ...devices["Desktop Safari"] } },
```

| 故事 | 加了什么 | 浏览器 | 每遍 → 合计 |
|---|---|---|---|
| MC3 新的一格 `MC3 (page): at 375 wide, a client's name of 128 characters and an author's of 100, neither with a space, keep the history, its filters and a changeset's page to the screen`（`@narrow`，32.3） | 改显示名；MCP 写两页、REST 撤销后一页、最后改名一次；历史、带 `page` 筛选的历史、三个变更集页 | Chromium、WebKit | 改名之前的部分实测 0.98 s、1.6 s（审查 B 的 `m12/p2-review-b/e2e-run4.txt`）；改名一步推断各 +0.5 s → 约 3.5 s |
| PG2 新的一格 `PG2 (page): at 375 wide, a title of 120 characters without a space keeps the page, its breadcrumbs and the dialogs that name it to the screen: rename, move, delete, and its attachment's rename and move`（`@narrow`） | 长标题的父页与子页、长名的附件；页面与五个对话框 | Chromium、WebKit | 推断 6 s、8 s → 14 s |
| LW3 (page) 加一段（已在 `threeBrowsers`） | 无空格标题的来源页；长名父页下的粘贴、上传两个对话框 | 三种 | 推断 2.5 s × 3 → 7.5 s |
| SE1 (page) 加段 | 两种主题下四个焦点环、mark 的对比度与字重；无效的搜索框的环；`:275` 的 `"600"` 改 `"700"` | Chromium | 推断 2.5 s |
| L5 (page) 加段 | 两种主题下补全选中项的说明与它的环、编辑器外框、阅读视图链接的外框、页面菜单的当前项 | Chromium | 推断 2 s |
| `webkit-narrow` 的 worker 启动 | 建库、起 nervewiki（`E/fixtures/test.ts:112-135` 是 worker 级的）、起 WebKit | WebKit | 推断 2–4 s × 1–2 个 worker |
| 只在 33.1 复现时：SE1 拆出的 `SE1 (page): at 375 wide …`（`@narrow`） | 种数据、搜一次、375 下的检查 | Chromium、WebKit | 推断约 +6 s |

- 全量多跑 test-seconds 约 32–38（33.1 复现时再加约 6）；测试数加 4 个（MC3、PG2 各两种引擎），复现时再加 2 个。墙钟多约 15–20 s，区间 10–40 s（算法与偏差见 4.3.5）。实现之后照实量（`--reporter=list` 的每格用时），写进记录。

##### 4.3.0.4 提交分组（每个提交单独为绿：tsc、knip、vitest、oxlint；e2e 由合并前的全量盖住）

见 6.3。

##### 4.3.0.5 拿不准的、要实施者先核对与审查重点看的

1. 全局外框的 `outline-offset: 2px` 在页面树的滚动容器里可能被裁掉左边（推断，没跑）：375 与 1280 下各看一眼树的行（12.4）。
2. 内嵌的环在计算样式里怎样写：推断 Chromium 把 `inset` 写在末尾，夹具照实解析（4.3.1.3、12.4 的子项）。
3. SE1 里 Tab 的次序（搜索框 → 搜索按钮；标题 → "Where to look" → 第一个命中）是推断（12.4）。
4. 38.11 剩下的两个对话框是哪两个、为什么，移交没写，读代码也定不了；`AlertDialogContent` 要不要加 `grid-cols-[minmax(0,1fr)]`，量完再定（38.11）。
5. LW3 能不能经 API 在 `raw` 下建一个无内容的父页、在它下面粘贴（38.11）。
6. MC3 最后的改名一步审查 B 没有跑：推断它是 `renamed` 一类的变更，那一行是 "Renamed: “A” → “B”"（32.3）。

#### 4.3.1 共同的做法

##### 4.3.1.1 375 px 为什么会超宽，改哪一个类

- **`break-words`（`overflow-wrap: break-word`）不降 min-content。** 它只在"放不下"时断词，不改元素报给父布局的最小宽度。CSS Text 3 规定只有 `anywhere` 才把断点算进 min-content。
- **flex 项的 `min-width: auto`** 取内容的 min-content，一个没有空格的长名字就是它的最小宽度，flex 项缩不下去，撑宽整页。父链里每一层 flex 都有这个问题：`W/app/layout.tsx:45-47`（窄屏时 `div.flex.flex-col`，`main.flex.min-w-0.flex-col`）、页里的 `div.flex`、`p.flex.flex-wrap`。两种改法：
  - **`min-w-0`** 加在那个 flex 项上，它能缩到行宽，`break-words` 再断词。先例：`search-hit.tsx:100`、`trash-row.tsx:478`。
  - **`[overflow-wrap:anywhere]`** 加在文字所在的元素或它的祖先上。它会继承，min-content 降到一个字。flex 容器里的文字是匿名 flex 项（写不上类）时，或多个子项都要管时，用它。先例：`W/pages/notebook/duplicate-text.tsx:249`、`history-ingest.tsx:47`、`index-rows.tsx:90`。子元素自己写了 `break-words` 会盖掉继承下来的 `anywhere`。
- **grid 的 `auto` 轨**同样按 min-content 定下限。`DialogContent` 在 `692d2482` 改成 `grid-cols-[minmax(0,1fr)]`，`AlertDialogContent`（`alert-dialog.tsx:19`）没有改。
- **下拉 `select`** 的固有宽度取最长的选项。`W/components/ui/native-select.tsx:10` 已有 `max-w-full`（`692d2482`），但它只在父元素的宽度定下之后才管用：父元素是没有 `min-w-0` 的 flex 项时，它的 min-content 就是 `select` 的全宽，两者一起超出去（移交第 38 条）。改外层 `div`：`max-w-full min-w-0`，照 `index-filters.tsx:46`；`select` 不加 `block`（审查 B 的 run4 在两种引擎里证实）。
- **对话框是 `position: fixed`**，超出视口不会让页面变宽，只能量它自己（LW3 的 `:330-333` 先例）。断言的写法见 4.3.1.3 的 `expectFits`。
- **jsdom 没有布局**，这些只能进 e2e（05-lessons 第 6 节、00 号文档 9.1）。

##### 4.3.1.2 浏览器：Chromium 与 WebKit，怎样只加 WebKit

- 00 号文档 9.1 写"Chromium 与 WebKit 两种"。今天 `E/playwright.config.ts:3-4,24-28` 的 `threeBrowsers` 一个正则同时交给 Firefox 与 WebKit；全量只有 Chromium。
- 做法（2.3 的 P2-15）：0.3 的 `webkit-narrow` 项目；375 的几格写成 `test(title, { tag: "@narrow" }, async (...) => ...)`。
  - Playwright 1.63 的项目级 `grep` 匹配标签：`--list --project webkit-narrow` 只列带 `@narrow` 的格，`chromium` 照跑全部；`make e2e` 与 CI 不带 `--project`，新项目会跑（审查 B 核过）。
  - `testIgnore: threeBrowsers` 让 LW3、LW13 不跑第二遍。
  - 夹具按 `browserType().name()` 判浏览器（`E/fixtures/browser.ts:105,134`），不看项目名；换文档的门禁 `tools/e2e-navigation/check.mjs` 不读配置。都不受影响。
- 不取：把 SE1、MC3、PG2 整份交给 `threeBrowsers`（整份多跑两遍，SE1 在 WebKit 上本就因手法失败，移交第 33 条）；`grep` 标题里的"at 375 wide"（靠措辞）。

##### 4.3.1.3 两个新的 e2e 夹具（要点，S3 照着写）

**`E/fixtures/narrow.ts`**：

- `phoneWidth = 375`；`expectFits(page: Page, ...boxes: Locator[])`。
- 页：`document.documentElement` 的 `clientWidth`、`scrollWidth` 都是 375。
- 每个框：`getBoundingClientRect()` 的 `left >= 0`、`right <= 375`；计算样式的 `display` 不是 `inline` 的，再断言 `scrollWidth - clientWidth <= 0`（行内元素的 `clientWidth` 是 0）。断言的消息带上是哪个框。
- 对话框是固定定位，当框量（4.3.1.1）。
- 照这些要点写的版本在两种引擎里可用（审查 B 的 `m12/p2-review-b/narrow.ts`、run2、run4）。

**`E/fixtures/contrast.ts`**（12.4、26.9 用）：

- 导出 `textContrast`、`focusContrast`、`indicatorContrast`，都是 `(locator) => Promise<Contrast>`，`Contrast` 是 `{ ratio, front, back, indicator? }`：两个颜色是 8 位的 `#rrggbb`，`indicator` 如 `"outline solid 2px"`、`"ring 3px"`。故事写 `expect(c.ratio, JSON.stringify(c)).toBeGreaterThanOrEqual(3)`，失败时报出两个颜色。
- 三个都调一个经 `locator.evaluate` 序列化进页面的 `measure(element, what)`，它不能引用外面的东西。实参写 `"text" as const`（或参数收 `string` 再收窄），否则 e2e 的 tsc 报 TS2345，只有 `make lint-web` 的门禁看得到（审查 B 的 N2）。
- **先结束元素自己的过渡**：`element.getAnimations().forEach((a) => a.finish())`，再读计算样式。
  - `input.tsx:9`、`native-select.tsx:10` 带 `transition-[color,box-shadow]`：刚取得焦点时读到过渡的起点 `oklab(0 0 0 / 0) 0px 0px 0px 0px`，晚一点读到中间值，时红时绿。
  - 证据（审查 B 的 M2）：不结束时夹具在搜索框上抛 "no focus indicator"（`m12/p2-review-b/e2e-run1.txt`）；结束之后，没修的代码亮 1.54、暗 1.88（run3），修了之后 4.74、4.18（run4）。
- **合成**：在 1×1 的 canvas 上从下往上画背后的颜色，再画前景，读回 8 位 sRGB，算 WCAG 2 的比值（4.3.1.4）。
  - 背后：`"text"` 从元素自己、外框与环从父元素往上取 `backgroundColor`，到第一个不透明的为止；带 `inset` 的环从元素自己取（环画在元素里面，里面的底比外面的更接近环色，4.3.4）。
  - canvas 读 `oklch(...)`、`oklab(...)` 与带 alpha 的颜色，按浏览器的 source-over 合成（审查 B 在 Chromium 里核过，得数与 4.3.4 一致）。
- **指示的解析**：`outline-style` 不是 `none` 且宽度大于 0 取外框；否则在 `box-shadow` 的列表里（按不在括号里的逗号分）找两个偏移与模糊都是 0、扩展大于 0 的一项，即 Tailwind 的环，颜色在前；认得末尾的 `inset`（推断 Chromium 这样写）。
- **`"focus"`** 先断言 `element === document.activeElement && element.matches(":focus-visible")`；**`"indicator"`** 不断言，给编辑器的外框（`.cm-editor.cm-focused`）与菜单、列表的当前项用。
- **量不到就抛错**，不猜颜色：canvas 读不懂的颜色（`fillStyle` 先设一个哨兵色再设目标色，没变就是没读懂）、往上找不到不透明的底、既没有外框也没有环。
- 切主题：`await page.emulateMedia({ colorScheme: "dark" })`，先例 `E/stories/links/l4-tags-math.spec.ts:189`；等 `html` 的 class 含 `dark`，回亮色时等它不含。量完回亮色。
- 让 `:focus-visible` 成立：文字输入框直接 `focus()`；按钮、链接、`select` 从一个已知元素按 `Tab`，或在一次键盘操作之后 `focus()`（推断 Chromium 的启发式这时匹配，夹具会核对）。
- SE1、L5 两个对比度故事只跑 Chromium。

##### 4.3.1.4 对比度怎样算

- **WCAG 2 的相对亮度**：每个 8 位通道 c/255，c ≤ 0.04045 取 c/12.92，否则 ((c+0.055)/1.055)^2.4，Y = 0.2126R + 0.7152G + 0.0722B。对比度 = (Y亮 + 0.05)/(Y暗 + 0.05)。
- **这个应用的灰都是 `oklch(L 0 0)`**：a = b = 0，OKLab 到线性 sRGB 的三行矩阵每行之和是 1，所以 Y = L³。例：`--muted-foreground` 亮色 0.556，Y = 0.1719，8 位是 #737373，对白 4.74。有彩色的（mark、destructive）照 OKLab 的完整矩阵换算再编码。
- **半透明的颜色**（`ring-ring/50` = `color-mix(in oklab, var(--ring) 50%, transparent)`）按浏览器的合成算：8 位 sRGB 上 source-over。例：#a1a1a1 半透明叠在 #ffffff 上得 #d0d0d0。
- 全部数值见 4.3.4（8 位取整之后的值，e2e 读回来的也是 8 位）。审查 B 用完整的矩阵另算了一遍（`m12/p2-review-b/contrast.py`），全部一致。

##### 4.3.1.5 `bdi` 的共用件：`W/i18n/isolated.tsx`（新）

- **为什么要共用件**：名字都在译文的句子里，en 与 zh-CN 的语序不同（`trash.deletedBy`：en "Deleted by {name}"，zh "{name} 删除"）；`Translate` 只出字符串（`W/i18n/i18n.tsx:22-32`）。先例 `search-page.tsx:273-285` 的 `Heading` 用 `split("{q}")` 插一个元素，一处一写，不能放多个名字。
- **签名与决定行为的几行**：

```tsx
/** Isolating is a translation whose names stand in it as slots, and the names, by their slots' indexes. */
export type Isolating = { text: string; names: readonly string[] };

// Private-use characters, which no translation has: a slot passes through Translate as any other value.
const slotted = /(\d+)/;

/**
 * isolate composes one Isolating: compose writes its text, each name in it given by name(value), which keeps the
 * value and answers the next slot. Every part the text is composed of takes this one name: no two names share a slot.
 */
export function isolate(compose: (name: (value: string) => string) => string): Isolating {
  const names: string[] = [];
  const text = compose((value) => `${(names.push(value) - 1).toString()}`);
  return { text, names };
}

/** Isolated writes text with each slot's name in a bdi, where the translation puts it (UAX #9's isolates; M5-polish 26.12, 32.4). */
export function Isolated({ text, names }: Isolating): ReactNode {
  return text.split(slotted).map((part, i) => (i % 2 === 0 ? part : <bdi key={i}>{names[Number(part)]}</bdi>));
}
```

  - `split` 带捕获组时，奇数位就是 slot 的序号。名字本身不进 `text`，名字里有私用区字符也不会混。`key={i}` 有先例（`page-backlinks.tsx:166`）。
- **拼接的规则**（审查 B 的 M5）：
  - 一个 `Isolating` 一个收集器：只有最外层调 `isolate`，句子的每一段都经同一个 `name` 取 slot，序号不会重。
  - 拼一段的函数多一个可选参数 `name: (value: string) => string`，缺省原样（`(value) => value`，下面记作 `same`），返回串：缺省时出今天的字符串，已有的使用者（选项、`aria-label`、面包屑）不变；传 `isolate` 的 `name` 时出带 slot 的串。所以不要 `plain` 一类的还原函数。
  - 不许把一个 `Isolating` 的 `text` 嵌进另一句、再沿用它的 `names`：两段各自从 0 起，"Moved from {from} to {to}" 的两个名字都是 slot 0，显示成 "Moved from “A” to “A”"。
  - 不取：`join(parts…)` 把后一段的序号整体平移。它要改写已经拼好的 `text`，平移错了还是同一种错。
- **用法**：
  - 一个名字：`<Isolated {...isolate((name) => t("trash.deletedBy", { name: name(item.deleted_by.display_name) }))} />`；
  - 嵌套：`isolate((name) => t("history.clientOf", { client: clientText(changeset.client, t, name) }))`；
  - 两段：`isolate((name) => t("history.detail.moved", { from: placeText(before, t, name), to: placeText(after, t, name) }))`；
  - 路径：每段一个 `name(...)`，用 `" / "` 连起来，分隔符在 `bdi` 外面，段的次序总是从左到右，与面包屑一致。
- **`<option>` 里放不了元素**：历史的"客户端""谁"两个下拉、搜索的"Where to look"仍用缺省 `name` 出的字符串。选项各自成一段，名字在句尾，风险低（4.3.3）。
- **`changeset-text.ts` 怎样改**（文件保持 `.ts`）：
  - `clientText(client, t, name = same)`：`mcp:<name>` 为 `t("history.client.mcp", { name: name(...) })`，不认识的客户端为 `name(client)`；`web`、`api`、`cli`、`mcp` 不出名字。
  - `changesetName(changeset, twin, t, locale, name = same)`：`display_name` 经 `name`。
  - `changesText(changeset, t, locale, name = same)` 与 `changeText(change, t, name)`：每个名字经 `name`。句首大写照旧作用在串上：en 的句首是动词，zh 的是汉字，不会碰到 slot。
  - `byText`、`countsText`、`kindText`、`reasonText`、`twinsIn` 不变（理由见 32.4）。

#### 4.3.2 逐项

##### 12.4 焦点环在白底上淡（全局）；补全选中项的说明文字 4.34:1；菜单与选项的当前项只靠底色

- **现状（`194ca350`）**：
  - 焦点环的颜色：`W/styles.css:41` 亮色 `--ring: oklch(0.708 0 0)`（#a1a1a1），`:57` 暗色 `oklch(0.556 0 0)`（#737373）。
  - 控件的环都是 `focus-visible:ring-[3px] focus-visible:ring-ring/50`，3 px 的 box-shadow、半透明，共 8 处：`W/components/ui/button.tsx:7`、`input.tsx:10`、`native-select.tsx:11`、`tabs.tsx:21,32`；`W/pages/workspace/workspace-home.tsx:43`、`W/pages/notebook/transfer-job-row.tsx:116`、`W/editor/text-diff.tsx:347`。`button.tsx:14` 的 destructive 是 `focus-visible:ring-destructive/40`。
  - 无效字段：`input.tsx:11` 的 `aria-invalid:ring-destructive/20` 只设 `--tw-ring-color`，宽度只来自 `focus-visible:ring-[3px]`，所以它就是无效字段的焦点环，不是装饰；生成的 CSS 里它排在 `focus-visible:ring-ring/50` 之后、特异性相同，盖掉后者（审查 B 的 M3）。表单被拒时 `useFocusOnInvalid` 把焦点放到第一个无效字段上，搜索框超过 256 字节被拒时正是这样。
  - 没有 `outline-none` 的元素（页面树的行 `page-tree-item.tsx:121-131` 的 `NavLink`、面包屑、搜索命中、所有 `Link`）用浏览器的 `outline: auto`，颜色被 `W/styles.css:62` 的 `* { @apply border-border outline-ring/50; }` 改成半透明的环色。Chromium 的计算样式是 `outline auto 1px`。
  - 实色外框：阅读视图 `W/reading/reading.css:123-127`、`:241-244`，编辑器 `W/editor/theme.ts:28`，都是 `2px solid var(--ring)`。
  - 补全：`W/editor/loaded/link-completion.ts:128-131` 的选中项底是 `--accent`，`:132` 的说明是 `--muted-foreground`；快速切换 `quick-switch.tsx:171,174`、页面树的悬停是同一组颜色。
  - 数（全表见 4.3.4）：控件的环亮 1.54、暗 1.88（Chromium 实测，`m12/p2-review-b/e2e-run3.txt`），链接同色；无效字段的环亮 1.44、暗 1.29（同上）；实色外框亮 2.58；destructive 的环 2.12、1.95；补全选中项的说明 4.35（移交写 4.34，是浮点的 4.339）。都不达标。
  - **用户怎样碰到**：按 Tab 到"搜索"按钮，白底上一圈 #d0d0d0 的浅灰；补全选中项后面的"→ Target"是 4.35:1 的灰字；表单被拒，焦点落在无效的字段上，环是 #faccce 对白。
  - **与 I3、移交**：一致，范围更大：I3 只列了 `styles.css:41,62`、`button.tsx:7`、`link-completion.ts:128-132`。
- **改法**（文字不变；2.3 的 P2-16）：
  1. `W/styles.css:36` 亮色 `--muted-foreground` 从 `oklch(0.556 0 0)` 改为 `oklch(0.53 0 0)`（#6c6c6c）：对白 5.25，对 accent 4.82。暗色 `:52` 不变。不取刚好达标的 0.547（4.48）或 0.54（4.61），给 8 位取整与抗锯齿留余量。
  2. `:41` 亮色 `--ring` 改为 `oklch(0.556 0 0)`（#737373）：对白 4.74，对 accent 4.35，对 primary 按钮 3.78。暗色 `:57` 不变：对页 4.18，对 popover 3.78，对 accent 3.19，对暗色 primary 3.76。
  3. `:60-63` 的 base：`*` 只留 `@apply border-border`，去掉 `outline-ring/50`；加

     ```css
     /* What takes the focus shows it, 3:1 at least against the page in either theme (WCAG 1.4.11; M12/P2/S3). Controls
        that draw their own ring (outline-none) keep theirs; the reading view's and the editor's are their own. */
     :focus-visible {
       outline: 2px solid var(--ring);
       outline-offset: 2px;
     }
     ```

     层的次序：utilities 的 `outline-none`（Tailwind 4.3 是 `outline-style: none`）比 base 后，压过它；`reading.css` 在 components 层，也压过它；CodeMirror 的 `.cm-content { outline: none }` 不在任何层（`node_modules/@codemirror/view/dist/index.js:6876`），压过层里的，编辑器不会多一圈。审查 B 核过这三条。
  4. 8 处 `focus-visible:ring-ring/50` 改为 `focus-visible:ring-ring`。
  5. `button.tsx:14`：`focus-visible:ring-destructive/40` 改为 `focus-visible:ring-destructive`（对页亮 4.77、暗 6.85）；加 `dark:bg-destructive/60`：暗色 destructive 按钮的白字今天是 #ffffff 对 #ff6467 2.89:1，叠成 #9d4042 之后 6.47，按钮对页 3.06。
  6. `input.tsx:11`：`aria-invalid:ring-destructive/20` 改为 `aria-invalid:ring-destructive`（亮 4.77、暗 6.85）。它只设颜色，环仍只在 `:focus-visible` 时有；规则的次序不变，照旧盖过 `ring-ring`。不取 `aria-invalid:focus-visible:ring-destructive`：效果相同，多一层变体。边框 `aria-invalid:border-destructive` 不变。
  7. 菜单与选项的当前项：见下面的子项。
- **S3 之后的焦点指示**（13.2 第 30 条照此写，8.2）：
  - 链接与其余没写 `outline-none` 的元素：2 px 实色的 `:focus-visible` 外框，`--ring`，外移 2 px；
  - 控件（按钮、输入框、下拉、标签页）：3 px 的 `ring-ring` 环；destructive 按钮是 `ring-destructive`；
  - 无效字段：3 px 的实色 `ring-destructive` 环；
  - 菜单项与列表框的当前项：`bg-accent` 加 2 px 的内嵌 `ring-ring` 环；
  - 阅读视图与编辑器：各自的 2 px 实色外框，`--ring`。

  每一种对相邻的颜色至少 3:1（4.3.4）。
- **子项：菜单与选项的当前项（2.3 的 P2-43；审查 B 的 M4，Q1 取甲）**
  - 现状：`W/components/ui/dropdown-menu.tsx:30`（`DropdownMenuItem`）与 `:42`（`DropdownMenuRadioItem`）是 `outline-none … focus:bg-accent`；`W/pages/notebook/quick-switch.tsx:171` 是 `index === current && "bg-accent"`；补全的 `li[aria-selected]`（`link-completion.ts:128-131`）只换底色。accent 对 popover 亮 1.09、暗 1.18，哪一项是当前项几乎看不出。页面树的菜单、页面菜单、用户菜单、主题与语言菜单都用 `dropdown-menu.tsx`。
  - 改法：当前项加一个 2 px、`--ring` 的内嵌环，底色照旧。
    - `dropdown-menu.tsx:30`、`:42`：加 `focus:ring-2 focus:ring-ring focus:ring-inset`。Radix 把焦点放在当前项上，键盘与指针都是，所以用 `focus:`。
    - `quick-switch.tsx:171`：`"bg-accent"` 改为 `"bg-accent ring-2 ring-ring ring-inset"`。焦点留在输入框里（`aria-activedescendant`），所以按 `current` 加。
    - `link-completion.ts:128-131`：加 `boxShadow: "inset 0 0 0 2px var(--ring)"`。
    - 内嵌的环不被列表的 `overflow` 裁掉，也不挪动内容。环对里面的 accent 亮 4.35、暗 3.19，对外面的 popover 4.74、3.78；快速切换在对话框里，对页 4.74、4.18。
    - 不取：左边 2 px 的边框（每一项都要留透明的边，夹具也要另认一种画法）；`data-[highlighted]:outline`（与同一个类里的 `outline-none` 抢一个属性）。
  - 测试：L5 (page)，只 Chromium，两种主题各一遍：
    - `:125` 之后（"#proj" 的补全开着，唯一的选项被选中）：`indicatorContrast(completion(page).locator("li[aria-selected]"))` ≥ 3；
    - `:138` 回到阅读视图之后：聚焦 "Page actions" 按钮，按 Enter，等第一个 `menuitem` 取得焦点，`indicatorContrast` ≥ 3，再 Escape。
  - 在没修的代码上：两处都没有环，夹具抛 "no focus indicator"，格红。
  - 变异：去掉 `:30` 的环，L5 菜单那一步失败；去掉补全主题的 `boxShadow`，补全那一步失败。抓不到的：`:42`（主题、语言菜单）与 `quick-switch.tsx:171`，没有 e2e 走到，照实写进记录。
  - 去处：S3 修，不进 v0.2。
- **测试**（其余；e2e，Chromium。jsdom 不载样式表，取不到计算之后的颜色；夹具见 4.3.1.3）：
  - **SE1 (page)**（`E/stories/search/se1-search.spec.ts`），`:275` 之后（"检索"的结果页），两种主题各一遍：
    - 搜索框 `focus()`，`focusContrast` ≥ 3；
    - `Tab`，断言 `getByRole("search").getByRole("button", { name: "Search" })` 有焦点，≥ 3；
    - 标题 `focus()`、`Tab`，"Where to look" 有焦点，≥ 3；
    - `Tab`，第一个命中的链接有焦点，≥ 3，且 `indicator` 以 `"outline solid"` 开头；
    - 26.9 的 mark 也在这个循环里量。

    Tab 的次序是推断（`search-page.tsx:205-262`），实现时照实核对。
  - **SE1 (page)**，`:314` 之后、375 那一段之前（审查 B 的 M3）：搜索框填 300 个字符、回车，断言 `aria-invalid="true"`、焦点在它上面；两种主题各一遍 `focusContrast` ≥ 3。
  - **L5 (page)**（`E/stories/links/l5-completion.spec.ts`），两种主题各一遍：
    - `:125` 之后：`textContrast(completion(page).locator("[aria-selected] .cm-completionDetail"))` ≥ 4.5；`indicatorContrast(page.locator(".cm-editor.cm-focused"))` ≥ 3；
    - `:138` 按 `ControlOrMeta+e` 回到阅读视图之后：`article.getByRole("link", { name: "Target", exact: true }).focus()`（紧跟一次键盘操作，推断 `:focus-visible` 成立，夹具会核对），`focusContrast` ≥ 3；
    - 子项的两步也在这里。
- **在没修的代码上怎样失败**：
  - SE1 的搜索框与按钮亮 1.54、暗 1.88（`m12/p2-review-b/e2e-run3.txt`）；"Where to look" 推断同；命中的链接是 `outline auto 1px`、半透明，推断 1.54；
  - 无效的搜索框 1.44、1.29（run3）；
  - L5 的说明 4.35 < 4.5；编辑器外框与阅读视图的链接亮 2.58。
  - 加上第 1–5 条之后（run4）：搜索框与按钮 4.74、4.18，无效的搜索框仍是 1.44、1.29。所以第 6 条不改，SE1 就红。
- **变异**：
  - `button.tsx:7` 改回 `ring-ring/50`：SE1 的搜索按钮 1.54、1.88，失败。
  - `--ring` 亮色改回 0.708：SE1 四处与 L5 的外框、链接亮色 2.58，失败。
  - 删掉 base 的 `:focus-visible` 规则：命中的链接回到 `outline: auto`，颜色是浏览器的默认值，比值可能反而够；靠 `indicator` 以 `"outline solid"` 开头那一句抓。
  - `--muted-foreground` 改回 0.556：L5 的说明 4.35，失败。
  - `input.tsx:11` 改回 `/20`：SE1 的无效搜索框 1.44，失败。
  - 抓不到的：`tabs.tsx`、`workspace-home.tsx`、`transfer-job-row.tsx`、`text-diff.tsx` 各自改回 `/50`；destructive 改回 `/40`、去掉 `dark:bg-destructive/60`。没有 e2e 走到。不在 PG2 顺带量 Delete 的环：PG2 也在 WebKit 上跑，夹具只在 Chromium 上核过。照实写进记录。
- **牵连**：
  - S1 也改 `button.tsx:7`（`aria-disabled:` 的样式），同一行会冲突；S5 的 12.8 改 `link-completion.ts` 的表格判断（`:151` 起）；S6 的 27.2 在 SE1 的 `:304-306` 之后加一段。后合并的先 `git merge main`、手解。
  - 全局的 `--muted-foreground`、`--ring` 与 base 规则改变全应用的样子。阅读视图的 `a.nw-unresolved`（`reading.css:109-112`）随之变深，对白 5.25，无害。

##### 26.9 暗色高亮的对比度；名称里 600 对 500，颜色之外的提示弱

- **现状（`194ca350`）**：
  - `W/pages/search/search-hit.tsx:35` 的 `mark`：`nw-hit rounded-xs bg-(--nw-mark-background) font-semibold text-inherit`。名称一行 `:87` 是 `font-medium`（500），摘要 `:117` 是常规（400）。
  - 颜色：`W/reading/reading.css:13` 亮色 `oklch(0.93 0.11 95)`（#ffe891），`:17` 暗色 `oklch(0.45 0.09 95)`（#655406）。
  - 文字对高亮：亮 16.22、暗 7.12，都达标；移交要求"暗色在结果页上核对"，结论是达标。高亮对页面：亮 1.22、暗 2.66（I3 估 2.7）。颜色之外的提示只有字重：名称里 600 对 500，只差 100。
  - **用户怎样碰到**：色觉弱的用户在结果的名称里认不出命中的字：名称是 500 的链接，600 的命中几乎看不出差别。
  - **与 I3**：I3 引的 `search-hit.tsx:142` 是 `pathOf`，相关的是 `:35` 与 `:87`。"部分"成立。
- **改法**（2.3 的 P2-17）：`search-hit.tsx:35` 的 `font-semibold` 改为 `font-bold`（700）：名称里 700 对 500，摘要里 700 对 400。颜色不动，文字不变。`Marked` 的注释（`:24-28`）已写 "bold, not by their colour alone"，不用改。
- **测试**：e2e，SE1 (page)，Chromium，用 `textContrast`。jsdom 不载 Tailwind，类名断言只是复述实现。
  - `:275` 的 `toBe("600")` 改为 `toBe("700")`；两种主题各一遍 `textContrast(mark)` ≥ 4.5，与 12.4 的循环合在一起。
  - 名称里的 mark：`:277-286` 的循环最后一次是"季度报告"，命中在附件的名称里。在 `:289` 的 `page.goBack()` 之前加：`searchResults(page).first().getByRole("link").locator("mark").first()` 的 `fontWeight` 为 `"700"`，它所在的 `p` 为 `"500"`；两种主题各一遍 `textContrast` ≥ 4.5。
  - **在没修的代码上怎样失败**：`:275` 与名称里的 mark 都拿到 `"600"`。对比度两格在没修的代码上就过（16.22、7.12），它们是守卫。
- **已有测试要随之改的**：`se1-search.spec.ts:275` 的 `"600"`。vitest 的 `search-page.test.tsx:103` 只看 `nw-hit` 这个类，不受影响。
- **变异**：`font-bold` 改回 `font-semibold`：两处字重都失败。暗色 `--nw-mark-background` 改成 `oklch(0.75 0.09 95)`：文字 2.12:1（`m12/p2-review-b/contrast.py`），失败。
- **牵连**：S6 改搜索的网页（26.2、26.5、26.18a、27.2），可能碰 `search-hit.tsx`，后合并的先 `git merge main`。阅读视图的 `mark`（`reading.css:155-160`）不加粗：那是作者自己的高亮。

##### 26.12 路径与名称不在 `bdi` 里

- **现状（`194ca350`）**：
  - **搜索**：`W/pages/search/search-hit.tsx:107-108` 的路径一行 `{path}` 是 `pathOf`（`:141-143`）拼成的 `[notebook, ...ancestors].join(" / ")`，双胞胎时接 `" · ID 0000e1"`（`:109-114`）。名称与摘要已在 `bdi` 里（`:102`、`:118`）。I3 写的 `:162-163` 不对：文件只有 143 行。
  - **回收站**：`W/pages/notebook/trash-row.tsx:260` 是 "From: A / B"（`fromText`，`:83-87`）；`:262` 是 "Deleted by {name} · 2 hours ago"；`:279` 是 "Restores under “{parent}”"（`restoreToText`，`:94-106`）。`TrashFacts`（`:255-282`）也用在预览页。
  - **历史的列表**：这里指变更里的名称（I3 已指出）。`history-row.tsx:96` 的 `changesText`（`changeset-text.ts:100-134`）拼成 "Renamed “A” to “B”, wrote “C”…"。
  - **用户怎样碰到**：页名都是希伯来文时，路径 "תוכניות / מדריך" 在从左到右的行里整段成了从右到左，次序倒过来；"Deleted by דוד · 2 hours ago" 里名字后面的 " · 2" 被拉进从右到左的一段，乱序（UAX #9 的 N1：中性符号夹在 R 与数字之间取 R）。
- **改法**（文字不变；规则见 4.3.1.5）：
  - `search-hit.tsx`：`pathOf(hit, notebook, name = same)` 让每段经 `name`，导出不变，`search-page.tsx:188` 照旧调用；`:108` 的 `{path}` 改为 `<Isolated {...isolate((name) => pathOf(hit, notebook ?? t("search.notebook"), name))} />`。`:77` 的 `path` 只留给 `aria-label`。
  - `trash-row.tsx`：`fromText(item, t, name = same)`、`restoreToText(item, t, name = same)` 让每个名字经 `name`；`namesOf`（`:212`）的标签照旧用缺省。`:260`、`:279` 改为 `<Isolated {...isolate((name) => fromText(item, t, name))} />` 一类；`:262` 改为 4.3.1.5 "一个名字" 的写法。
  - `history-row.tsx:59,96`：`const summary = isolate((name) => changesText(changeset, t, locale, name))`；`summary.text === ""` 时不渲染，否则 `<p className="text-sm break-words"><Isolated {...summary} /></p>`。
  - 不在这一项里的同类，见 4.3.3。
- **测试**：vitest。jsdom 有 DOM，`<bdi>` 在不在一查就知道，不必进 e2e。
  - 新 `W/i18n/isolated.test.tsx`：
    - `"Isolated writes each name in a bdi where the translation puts it, the text around it as it is, in English and in Chinese"`：用 `translator("en")` 与 `translator("zh-CN")`（`W/i18n/i18n.tsx:25`）各渲染一次 `isolate((name) => t("history.change.renamed", { from: name("מדריך"), to: name("התקנה") }))`。断言 `bdi` 的文字依次是 `["מדריך", "התקנה"]`；`container.textContent` 是 `"renamed “מדריך” to “התקנה”"` 与 `"改名「מדריך」为「התקנה」"`，整句比对。
    - `"the parts a text is composed of give their names slots of their own"`：`isolate((name) => [name("א"), name("ב")].join(" / "))` 的 `names` 是 `["א", "ב"]`，渲染出的 `bdi` 依次是这两个。
  - `W/pages/search/search-page.test.tsx` 新格 `"a hit's path writes its notebook and each page above it in a bdi of its own, the separators between them outside"`：一页在两层希伯来文父页之下，搜到它；路径那个 `p`（`row.querySelectorAll("p")[1]`）的 `bdi` 依次是 `["Plans", "מדריך", "התקנה"]`，`textContent` 是 `"Plans / מדריך / התקנה"`。
  - `W/pages/notebook/trash-page.test.tsx` 新格 `"an item's facts write where it was, who deleted it and where a restore puts it, each name in a bdi"`：`trashed(nodes, gone, nth, by)`（`W/test/trash-server.ts:63`），祖先是希伯来文名，`by.display_name` 是 `"דוד"`；行里 `bdi` 依次是 `["מדריך", "התקנה", "דוד", "התקנה"]`（来处两段、删除者、恢复到的父页）。再切 zh-CN（`act(() => server.app.preferences.setLocale("zh-CN"))`）：删除者那一行以 `"דוד 删除 · "` 开头，`bdi` 仍是 `"דוד"`。
  - `history-page.test.tsx` 的新格见 32.4，它也覆盖 `changesText`。
  - **在没修的代码上怎样失败**：上面各格的 `bdi` 都是 `[]`。
- **已有测试不受影响**（逐个核对过）：`search-page.test.tsx:125,191` 取最后一个 `p bdi`，摘要在路径之后；回收站的测试不查 `bdi`；按可访问名找的（`trash-preview.test.tsx:500`、`trash-page.test.tsx:956`）：`bdi` 是行内元素，可访问名不加空格。
- **变异**：`Isolated` 把 `<bdi>` 换成 `<>`：上面各格全部失败。`pathOf` 不经 `name`：搜索那一格失败。`trash.deletedBy` 改回 `{ name: display_name }`：回收站那一格失败。`isolate` 的 `name` 总答 slot 0：`isolated.test.tsx` 的第二格失败。
- **牵连**：
  - P3/S3 改同一个 `trash-row.tsx` 的 `:125-177`（`refusedText`、`refusalText`、`offeredOf`，P3 设计 4.2 与决定 377）和 `trash-background.test.tsx`；S3 改 `:83-106`、`:255-282` 与 `trash-page.test.tsx`。行不重叠，文本冲突时后合并的先 `git merge main`。
  - S6 改回收站与搜索（22.x、26.x），S2 改回收站与历史的文字（22.x、23.x）：同上。S2 改了键的，S3 这里跟着改。

##### 26.17 历史筛选的选择比手机宽

- **现状（`194ca350`）**：
  - `W/pages/notebook/history-filters.tsx:119` 是 `div.flex.flex-wrap.items-end.gap-4`，三个 flex 项 `div.space-y-2`（`:120` 谁、`:136` 客户端、`:151` Ingestion）都没有 `min-w-0`。`NativeSelect` 基类已有 `max-w-full`。原理见 4.3.1.1：flex 项的 min-content 等于下拉的全宽。
  - 范围比移交大一点：`:162`、`:171-189` 的"页面"筛选 `PageFilter` 是 `p.flex.h-9.items-center…`，也是这一行的 flex 项，没有 `min-w-0`；里面的 `span.break-words`（`:177`，"Page: {name}"）也是 flex 项。页名是无空格的长串时同样撑宽；`h-9` 定高，文字折了行也会溢出框外。
  - **用户怎样碰到**：有人的显示名很长且没有空格，"谁"的下拉在手机上把整页撑宽。
  - **与 I3、移交**："部分"对：基类已有 `max-w-full`，缺的是外层。
- **改法**（文字不变）：
  - `history-filters.tsx:120`、`:136`：`className="space-y-2"` 改为 `"max-w-full min-w-0 space-y-2"`，照 `index-filters.tsx:46`。`NativeSelect` 不加类，不加 `block`。
  - 不改 `:151`：Ingestion 的选项都短，没有一格抓得到（05-lessons 第 1 节）。
  - `:176` 的 `PageFilter`：`flex h-9 items-center …` 改为 `flex min-h-9 max-w-full min-w-0 items-center …`；`:177` 的 `span`：`break-words` 改为 `min-w-0 break-words`。
- **测试**：e2e，MC3 的新格（32.3 的第 1、2 步），Chromium 与 WebKit。jsdom 没有布局。
  - **在没修的代码上怎样失败**（`m12/p2-review-b/e2e-run2.txt`）：历史页宽 Chromium 966、WebKit 1010，"谁"的外层右边 847、883，"客户端"的 966、1010；带筛选的页 966，`PageFilter` 右边 820。加上改法之后（run4）都是 375。
- **变异**：去掉 `:120` 的 `min-w-0` 或 `:136` 的：第 1 步失败。去掉 `PageFilter` 的 `min-w-0`：第 2 步失败。
- **牵连**：S6 的 27.2 与 21.x、23.x 若改 `history-filters.tsx` 会冲突，后合并的先 `git merge main`。35.5 是 C，不改。

##### 32.1 历史的"客户端""谁"下拉在 375 px 超宽

- **现状**：同 26.17。`history-filters.tsx:136-149` 是"客户端"，选项是 `clientText`（`:146`）；`mcps`（`:114`）来自已读到的变更集，`mcp:<名>` 写成 "MCP (<名>)"。名字最长 128 个字符，可以没有空格（`server/internal/shared/client_name.go:13,29`）。
  - 移交量到的 1,027 是 `692d2482`（基类的 `max-w-full`）之前的数；外层仍缺 `min-w-0`，仍超宽（审查 B 的 run2：966）。移交第 32 条试过的改法里，下拉上的 `block max-w-full` 今天都不要（run4）。
  - **用户怎样碰到**：一个 agent 报了 128 个字符的 `clientInfo.name`，管理员在手机上看历史，页面横向可滚。
- **改法**：同 26.17（`:120`、`:136`）。文字不变。
- **测试、变异**：同 26.17，MC3 的第 1 步；去掉 `:136` 的 `min-w-0`，客户端的外层撑宽，失败。
- **牵连**：P3/S1 的 30.2 改的是只由不可见字符组成的名字（服务端），与宽度无关。

##### 32.2 变更集页"Client: MCP (…)"不折行

- **现状**：`W/pages/notebook/changeset-page.tsx:331` 是 `<p className="text-muted-foreground">{t("history.clientOf", …)}</p>`，没有 `break-words`。父链都是块，宽度确定，只缺断词。移交量到 712。
  - **用户怎样碰到**：打开那个 agent 的一次写，要横向滚动。
- **改法**：`className` 改为 `"break-words text-muted-foreground"`。`p` 是块，`break-words` 就够。这一行的内容同时换成 `Isolated`（32.4）。文字不变。
- **测试**：MC3 的第 3 步，`expectFits` 这一行。Playwright 的文字匹配按元素的整段文字算，`bdi` 不影响。
  - **在没修的代码上怎样失败**：这一行溢出 543，页宽 894（run2）。
- **已有测试不受影响**：`changeset-page.test.tsx:92` 是 `about.textContent` 包含 "Client: CLI"。
- **变异**：去掉 `break-words`，第 3 步失败。
- **牵连**：同一页的面包屑（`:198-204`）的 `current` 含长显示名，要 38.10 的面包屑改法，否则第 3 步在面包屑上失败（run2：面包屑溢出 458）。所以面包屑的改法放在提交 2 里（4.3.0.4）。

##### 32.3 MC3 加一个 128 个字符的客户端名在 375 px 上的检查

- **现状**：`E/stories/mcp/mc3-write-edit.spec.ts` 只有一格（`:46-164`），客户端名是 `"e2e-agent"`，没有 375 的检查。
- **改法**：加一格（标题见 4.3.0.3，`{ tag: "@narrow" }`）。不改 `:46` 那格，以免动它 `:149` 的 `"MCP (e2e-agent) · "`。
  - **布置**：
    - `newOnboardedTeam`；显示名 `author = ada_${"a".repeat(96)}`（100 个字符，上限 `identity/domain/user.go:33`），`PATCH /api/v0/me`（`bearer`，`E/fixtures/auth.ts:58`；先例 `a8-profile.spec.ts:15`），断言 200；
    - 笔记本 "Handbook"；`mcpClient(nervewiki.baseURL, pat, { notebook, client: agentName })`，`agentName = agent.${"c".repeat(122)}`（128 个字符，上限 `shared/client_name.go:13`；`client` 就是 `clientInfo.name`，`E/fixtures/mcp.ts:152-170`）；
    - agent `write` `${title}.md`，`title = report_${"r".repeat(113)}`（120 个字符），记 `wrote`；
    - agent 再 `write` `${other}.md`，`other = other_${"o".repeat(114)}`，记 `second`；
    - REST 撤销 `second`（`revertChangeset`，`E/fixtures/history.ts:55`），断言 200：`other` 进回收站，长标题那一页还活着；
    - 视口 375 × 800。
  - **不撤销 `wrote`**：撤销一个新建会把它新建的页放进回收站（README:179），筛选那一格就成了 "Page: a deleted page"，变更集页上也没有那条链接；初稿这样写，两种引擎都在等筛选时失败（审查 B 的 M1，`m12/p2-review-b/e2e-run1.txt`）。
  - **断言**，依次：
    1. 历史（`historyPath`）有三行（`historyListed`）；`expectFits` 谁、客户端、Ingestion 的外层 `div`（`main.getByLabel("Who", { exact: true }).locator("xpath=..")` 一类）与三行。
    2. `historyPath(…, { page: wrote.node_id })`：等 `` main.getByText(`Page: ${title}`, { exact: true }) ``，`expectFits` 它的父元素 `p`。
    3. `wrote` 的变更集页（`changesetPath`）：等 `` main.getByText(`Client: MCP (${agentName})`, { exact: true }) ``；`expectFits` 这一行、`navigation` "Breadcrumb"、列表 "Nodes of this change" 里名为 `title` 的链接（`changeset-change.tsx:103`）。
    4. `second` 的变更集页：`expectFits` 列表里文字为 `other` 的 `span`（`:109`，已删的节点）与面包屑。
    5. agent `move` `${title}.md` 到 `${renamed}.md`（`renamed = renamed_${"n".repeat(112)}`），打开这次的变更集页：`` expectFits(page, main.getByText(`Renamed: “${title}” → “${renamed}”`, { exact: true })) ``（`:132` 的 `span`）。
  - 夹具都已有：`Written` 在本文件 `:18-27`；`historyPath`、`changesetPath`、`historyListed` 在 `E/fixtures/wiki-pages.ts:357-370`；`phoneWidth`、`expectFits` 在新的 `E/fixtures/narrow.ts`。
- **这一格守着** 26.17、32.1、32.2、35.8、38.12，以及 38.10 的面包屑。
- **浏览器**：Chromium，加上 `webkit-narrow`。
- **在没修的代码上怎样失败**：第 1–4 步的五处在两种引擎里都失败（`m12/p2-review-b/e2e-run2.txt`：历史页宽 966、1010，行溢出 588；带筛选的页 966；`wrote` 的变更集页 894；`second` 的 1193）。加上本 Step 的类之后（run4）都是 375/375，两种引擎都过。第 5 步 B 没有跑，推断没修时那一行撑宽。
- **变异**：见 26.17、32.1、32.2、35.8、38.12。
- **牵连**：WebKit 上 run4 没有多出的控制台消息；`pageWatch` 已放行 WebKit 的"预加载没用上"警告（`E/fixtures/browser.ts:210,223`）。

##### 32.4 客户端名与显示名不在 `bdi` 里

- **现状（`194ca350`）**：
  - `history-row.tsx:79` 的 `{clientText(changeset.client, t)}{" · "}<time>…` 写成 "MCP (עוזר) · 5 minutes ago · ID 0000e1"。
  - `changeset-page.tsx:331` 是 "Client: MCP (עוזר)"。
  - 撤销的链接 `history-row.tsx:116`、`changeset-page.tsx:352` 是 `changesetName`：en "Undid “Edit · {name} · Oct 5, 2026, 8:02 AM”"，zh "撤销了「编辑 · {name} · 2026年10月5日 08:02」"。
  - 变更集页上节点的说明 `changeset-change.tsx:14-54`（`placeText`、`treeTexts`，显示在 `:131-133`）："Renamed: “A” → “B”"、"Moved from “A” to “B”"。两个从右到左的名字之间只有中性符号与箭头，整段成了从右到左，"从哪到哪"读反了，"→" 不镜像。它就在同一页，属于"变更里的名称"，一并做（2.3 的 P2-18）。
  - 消息已在 `bdi` 里（`:93`、`:343`）。服务端只去掉 bidi 的控制字符（移交第 32 条），强的从右到左的字母照留。
  - **用户怎样碰到**：en 的 "MCP (עוזר) · 5 minutes ago" 里，名字后面的 ") · 5" 被拉进从右到左的一段（N1），括号与时刻乱序；zh-CN 的撤销链接里名字后面紧跟 " · 2026"，同样乱。
  - **不改的**：`byText`（`history-row.tsx:69` 的链接、`changeset-page.tsx:211` 的 `h1`）。链接是 flex 项、名字在段尾；`h1` 里名字之后是 "Ingest"，中性符号取 L。风险低。而且已有测试用 `row.querySelector("bdi")` 取第一个 `bdi` 当消息（`history-ingest.test.tsx:44,60`、`history-page.test.tsx:98,891,954`、`page-menu.test.tsx:112`）：在 `byText` 里加 `bdi`，`:954` 永远为空，`page-menu.test.tsx:112` 直接失败。
  - **I3 说可降为 C**：保留 B。zh-CN 的日期与 en 的相对时刻都紧跟在名字之后，乱序会读错时刻。
- **改法**（文字不变；4.3.1.5 的 `isolate` 与收 `name` 的 `clientText`、`changesetName`）：
  - `history-row.tsx:79`：`<Isolated {...isolate((name) => clientText(changeset.client, t, name))} />`。
  - `history-row.tsx:116`（`Link` 的子元素）与 `changeset-page.tsx:352`（`twin` 为 `false`）：`<Isolated {...isolate((name) => t("history.reverts", { name: changesetName(reverts, revertsTwin, t, locale, name) }))} />`。
  - `changeset-page.tsx:331`：`<p className="break-words text-muted-foreground"><Isolated {...isolate((name) => t("history.clientOf", { client: clientText(changeset.client, t, name) }))} /></p>`。
  - `changeset-change.tsx`：`placeText(state, t, name)` 让父页的名字经 `name`；`treeTexts` 返回 `Isolating[]`，每一句一个 `isolate`，moved 的 `from`、`to` 两段共用它的 `name`，renamed 的两个名字同样；`:131-133` 改为 `<span key={parts.text} className="min-w-0"><Isolated {...parts} /></span>`（`min-w-0` 见 35.8；一行至多 moved 与 renamed 两句，`text` 不重）。
- **测试**：vitest。
  - `W/pages/notebook/history-page.test.tsx` 新格 `"a name of another direction does not turn a row around it: an MCP client's, the names of its changes and of the author of what it undid, each in a bdi"`：
    - `server.log("rtl", { client: "mcp:עוזר", by: { user_id: …, display_name: "דוד" }, changes: [renamed(notes, "מדריך")] })`，再记一次 `kind: "revert"`、`reverts` 指向它（照 `:40-58` 的 `logged()`）。
    - 断言：时刻所在的 `p`（`row.querySelector("time")!.parentElement`）的 `bdi` 是 `["עוזר"]`，`textContent` 以 `"MCP (עוזר) · "` 开头；变更一行的 `bdi` 是 `["מדריך", "Notes"]`（次序已对着 `history-server.ts:164-172` 核过，审查 B）；撤销那一行 "Undid" 链接里的 `bdi` 是 `["דוד"]`。
    - 再 `setLocale("zh-CN")`：客户端一行以 `"MCP（עוזר） · "` 开头，`bdi` 仍是 `["עוזר"]`。
  - `W/pages/notebook/changeset-page.test.tsx` 新格 `"a changeset's page writes its MCP client's name, what it undid and its nodes' names where they were renamed each in a bdi"`：客户端行 `bdi` 为 `["עוזר"]`，`textContent` 为 `"Client: MCP (עוזר)"`；改名的节点（from `"מדריך"`、to `"התקנה"`）那一行的 `span` 里 `bdi` 为 `["מדריך", "התקנה"]`，`textContent` 为 `"Renamed: “מדריך” → “התקנה”"`。整句比对。
  - 同一文件新格 `"a node moved between two pages of another direction names where it was and where it is in that order, each in a bdi"`：`moved(node, from)`（`W/test/history-server.ts:175`），节点现在在希伯来文名的父页 "התקנה" 下，`from` 是另一个希伯来文名的父页 "מדריך" 的 id；那一行的 `span` 里 `bdi` 依次是 `["מדריך", "התקנה"]`，`textContent` 是 `"Moved from “מדריך” to “התקנה”"`，整句比对（审查 B 的 M5）。
  - **在没修的代码上怎样失败**：这些 `bdi` 都是 `[]`。
- **已有测试不受影响**（逐个核对过）：`changeset-page.test.tsx:95` 的客户端是 `cli`，不出 `bdi`；撤销链接按可访问名找（`history-page.test.tsx:110,144,180`、`changeset-page.test.tsx:260`）；"取第一个 `bdi`" 的几格，客户端都是 `web`、`api`，变更名在消息之后。
- **变异**：
  - `clientText` 不经 `name`：两个页面的客户端断言失败。
  - `Isolated` 不写 `bdi`：全部失败。
  - `treeTexts` 改回字符串：变更集页的两格失败。
  - 两段都用 slot 0（`placeText` 自己调 `isolate`，或 `name` 总答 slot 0）：moved 那一格的 `bdi` 成了 `["מדריך", "מדריך"]`，失败。
- **牵连**：
  - S2 的 23.x 改撤销的用词（`history.reverts`、`history.kind.revert` 等）：按整句比对的几格要跟着改，后合并的 Step 改测试。
  - 面包屑的 `current`（`changeset-page.tsx:203`）也含显示名加日期，zh-CN 里同样有风险；改它要让 `BreadcrumbTrail` 的 `current` 收 `ReactNode`，`changeset-page.test.tsx:91` 随之改。不并进本 Step，进 v0.2（2.3 的 P2-37）。

##### 33.1 Safari 窄屏：搜索页宽到 704

- **现状（`194ca350`）**：
  - SE1 (page)（`E/stories/search/se1-search.spec.ts:250-328`）只在 Chromium 上跑，最后一段（`:316-327`）在 375 下断言页宽与 "Where to look"。
  - 2026-11-07 全量三浏览器跑过一次，WebKit 上页宽 704（`docs/v0.1/M10-llm-wiki/reviews/P4-S3-web-review.md:151`）。之后相关的只有 `692d2482`（下拉基类的 `max-w-full`）与 `bafbd418`（e2e 的帮手）；搜索页的下拉在 `8cf63252` 已是 `block max-w-full`。
  - 读代码找不到会在 WebKit 上到 704 的东西。704 − 24（`main` 的 `p-6`）= 680，约等于 98 个字符的笔记本名在 14px 下的下拉固有宽度：推断最可疑的是 "Where to look" 的 `select`；其次是左栏在窄屏时的列（`workspace-layout.tsx:110-128`）。
- **排查的步骤**（S3 开工时先做，在 `main` 的代码上，任何改动之前；不进套件）：
  1. 看 `df -h / /tmp`（`06-environment.md` 第 3 节），`docs/v0.1/claude_to_codex/tools/heavy.sh make build`。
  2. 让 SE1 (page) 只在 WebKit 上跑：一个不提交的临时配置，`projects` 只留一个没有 `testMatch` 的 WebKit 项目（命令行没有覆盖 `testMatch` 的开关）；或者临时把 `search\/se1-` 加进 `threeBrowsers`，跑完 `git checkout`。
  3. 在 `:325` 之前临时插一段探针：页宽大于视口时，列出右边超过 `clientWidth + 0.5` 的元素（标签、类名、左右）、WebKit 的版本，存整页截图。
  4. 经 `heavy.sh` 跑这一格三遍：`-g "SE1 \(page\)" --repeat-each=3 --workers=1 --reporter=list`，`NWIKI_E2E_VERSION` 取 `Makefile:13` 的默认 `VERSION`（`E/fixtures/test.ts:82-89` 要它）。
  5. 对照：同一构建、默认配置，`--project=chromium` 跑一遍同一格，应得 375/375。
  6. **看什么数**：`width.window`（应为 375，不是就先查视口）；`width.page`（大于 375 即复现，11-07 那次是 704）；`placed`（下拉框的右边）；探针列出的元素，最深的那个是根；WebKit 的版本；截图。
  7. 跑完删掉临时配置与探针，`git status` 干净。
  8. SE1 (page) 若在 375 之前就在 WebKit 上失败（多半是手法，05-lessons 第 6 节）：记下是哪一条断言，在临时文件里只留"种数据、搜一次、375"几步（`written()`、`searchFor()`），重跑 4–6 步。
- **复现了**（三遍里任意一遍 `width.page > 375`）：
  - 按 B 处理（WCAG 1.4.10），在 S3 里修：照探针指出的元素用 4.3.1.1 的办法。
  - 回归测试：把 `:316-327` 拆成一格 `SE1 (page): at 375 wide the results keep to the screen, Where to look too, which lists the notebook's long name`（`@narrow`），在 `webkit-narrow` 里跑；长的那格的标题去掉 "; on a phone's width the page fits"。这一格在没修的代码上、在 WebKit 里失败（第 4 步已证）。
  - 记录写下 WebKit 的版本与量到的数；主持者把 00 号文档附录 A 的 33.1 从"发布之后的人工"移到 S3，记成主持者的决定。
- **没复现**（三遍都是 375）：33.1 留在发布之后的人工清单（真的 Safari，iPhone 或 macOS 的窄窗口，00 号文档 4.12）；记录写下 WebKit 的版本、三遍的数与对照，说明 Playwright 的 WebKit 不是 Safari。套件不变（2.3 的 P2-36）。
- **测试、已有测试**：只在复现时有，见上。
- **变异**：复现时，去掉所修的那一处，拆出的那一格在 WebKit 上失败。
- **牵连**：拆出的格要等 4.3.0.3 的 `webkit-narrow` 项目，排在提交 2 之后。

##### 35.8 375 宽：谁的下拉、`changeset-change.tsx` 的链接超宽

- **现状（`194ca350`）**：
  - 谁的下拉：同 26.17，`history-filters.tsx:120`。
  - 节点的链接：`W/pages/notebook/changeset-change.tsx:96` 是 `p.flex.flex-wrap.items-center.gap-x-3.gap-y-1`：`:98-106` 的 `Link`（类在 `:103`）、`:109` 已删节点的 `span`，都是 `font-medium break-words` 的 flex 项，没有 `min-w-0`；`:130` 的 `p.flex.flex-wrap … break-words` 里，`:132` 的 `span`（"Renamed: …""Moved from …"）也是 flex 项，`break-words` 是继承来的，不降最小宽度。移交量到 1596。
  - 同一类、移交没写的：`history-row.tsx:62` 的 `p.flex.flex-wrap` 里 `:63-70` 的 `Link`（类在 `:67`，"Edit · <显示名>"），`:100` 的 `p.flex.flex-wrap` 里 `:115` 的撤销链接，都没有 `min-w-0`。移交第 32 条说"历史的行会折行（有 `break-words`）"，只对 `:78` 那个块级的 `p` 成立。
  - **用户怎样碰到**：LLM Wiki 里文件名样子的来源名（`attention_is_all_you_need_…`），变更集页的那一行把页撑到 1596。
- **改法**（文字不变）：
  - `changeset-change.tsx:103`、`:109`：类前加 `min-w-0`；`:132` 的 `span` 加 `className="min-w-0"`（32.4 改成 `Isolated` 之后同一处）。
  - `history-row.tsx:67`、`:115`：类前加 `min-w-0`。
  - 谁的下拉：同 26.17。
- **测试**：MC3 的新格（32.3）：第 1 步的三行，第 3 步的节点链接，第 4 步已删节点的 `span`，第 5 步改名的那一句。
  - **在没修的代码上怎样失败**（run2）：三行各溢出 588；第 3 步的链接右边 842；第 4 步的 `span` 右边 1193。第 5 步推断。
- **变异**：
  - 去掉 `changeset-change.tsx:103` 的 `min-w-0`：第 3 步失败。
  - 去掉 `:109` 的：第 4 步失败。
  - 去掉 `:132` 的：第 5 步失败（推断；没有这一步就抓不到，所以 MC3 加一次改名）。
  - 去掉 `history-row.tsx:67` 的：第 1 步三行都失败（显示名 100 个字符）。
  - 去掉 `:115` 的：第 1 步撤销那一行失败。
- **牵连**：26.12、32.4 改同一个文件的同几行，同一个 Step 里先后改即可。

##### 38.10 很长的连续标题撑宽页面

- **现状（`194ca350`）**：
  - `W/pages/page/page-layout.tsx:254` 是 `div.flex.items-start.justify-between.gap-3`，`:255-263` 的 `h1`（类在 `:259`）是 `text-3xl font-semibold break-words outline-none`，flex 项，没有 `min-w-0`；旁边是 `div.flex.shrink-0`（菜单与 "Edit"）。
  - 面包屑：`W/pages/page/breadcrumbs.tsx:50` 是 `ol.flex.flex-wrap.items-center.gap-x-1…`，每个 `li`（`:52`、`:59`）是 flex 项，没有 `min-w-0`，也没有断词。
  - 移交量到：375 宽下页宽 1416；1280 宽下 1656（普通笔记本 1483、1723）。`h1` 加 `min-w-0` 清掉 1280 的那一处，375 宽下面包屑还剩 630。
  - `BreadcrumbTrail` 也用在版本页、变更集页、回收站的预览（`version-page.tsx:129`、`changeset-page.tsx:198`、`trash-preview.tsx:302`），一处改法盖住四处。
  - 同类、移交没写：回收站预览的 `h1`（`trash-preview.tsx:310-318`）是 `flex min-w-0 … break-words`，名字是匿名 flex 项，长名同样撑宽（推断）。不并进本 Step，进 v0.2（2.3 的 P2-37）。
  - **用户怎样碰到**：在 LLM Wiki 里打开一个文件名样子的来源（写者改不了它的名），手机上整页横向滚动。
- **改法**（文字不变）：
  - `page-layout.tsx:259`：类前加 `min-w-0`。
  - `breadcrumbs.tsx:50`：`ol` 的类后加 `[overflow-wrap:anywhere]`。它继承到每个 `li` 与里面的链接，`li` 的 min-content 降到一个字；`li` 里没有写 `break-words`，不会盖掉它。
- **测试**：e2e。jsdom 没有布局。
  - **PG2 新格**（`E/stories/page/pg2-rename-page.spec.ts`，`@narrow`，Chromium 与 WebKit）：父页 `plans_${"p".repeat(114)}`，其下子页 `roadmap_${"x".repeat(112)}`（都是 120 个字符；`shared.MaxTitleBytes` 是 255 字节，`server/internal/shared/title.go:13`），子页下上传一个附件 `scan_${"s".repeat(110)}.txt`（`E/fixtures/assets.ts:39` 的 `uploadAsset`）。375 下 `goto` 子页，等 `pageHeading`，`expectFits(page, heading, breadcrumbNav)`。1280 下先量一次页宽 ≤ 1280，盖住移交的 1280 那一处。
  - **LW3 (page)**（`threeBrowsers`）：`:325` 的 `createPage(…, unbroken, raw, …)` 留下返回值；`:338` 之后在 375 下 `goto` 这个来源页，等标题，`expectFits(page, heading)`。`unbroken` 是 46 个字符，在 30px 的 `h1` 里推断约 740px。
  - **在没修的代码上怎样失败**：页宽大于 375（移交量到 1416）。
- **已有测试要随之改的**：LW3 (page) 的标题加上新的说法，例如 "…and a source's name without a space keeps its page and the dialogs that name it to the screen"。
- **变异**：去掉 `h1` 的 `min-w-0`：PG2、LW3 失败。去掉 `ol` 的 `anywhere`：PG2 失败，MC3 的第 3、4 步也失败（run2：面包屑溢出 458）。
- **牵连**：S4 改阅读视图（`reading-view.tsx`），推断不碰 `page-layout.tsx:254-263`。S1 的到达焦点用 `h1`，只加类，不受影响。

##### 38.11 长标题跑出对话框的标题行

- **现状（`194ca350`）**：
  - `W/components/ui/dialog.tsx:24-26` 的 `DialogTitle` 只有 `text-lg font-semibold`；`alert-dialog.tsx:28-30` 的 `AlertDialogTitle` 同样。
  - `DialogContent`（`:15`）的列是 `grid-cols-[minmax(0,1fr)]`，列宽确定（375 下约 295），标题是块，只缺断词。`AlertDialogContent`（`alert-dialog.tsx:19`）没有这一列，是 `auto` 轨，按 min-content 撑开。
  - 名字进标题的对话框：改名 `rename-page-dialog.tsx:56`、移动 `move-page-dialog.tsx:68`；附件的改名与移动 `W/pages/page/asset-dialogs.tsx:63,127`；粘贴、上传 `source-add-dialog.tsx:80`（名字是父页的）；删除的确认 `ConfirmDialog`（`page.deleteTitle`、`asset.deleteTitle`，走 `AlertDialogTitle`）。
  - 移交：375 宽下这些对话框量到 847–878；`DialogTitle` 加 `[overflow-wrap:anywhere]` 在 Chromium 上清掉其中三个，另两个是哪两个、为什么，没写。
  - **用户怎样碰到**：在手机上给一个长名的页改名，对话框的标题跑出屏幕，"保存"按钮可能也跟着出去。
- **改法**（文字不变）：
  - `dialog.tsx:25`、`alert-dialog.tsx:29`：`cn("text-lg font-semibold", className)` 改为 `cn("text-lg font-semibold [overflow-wrap:anywhere]", className)`。
  - 量完再定：剩下的两个对话框照 e2e 指出的元素补，用 4.3.1.1 的办法；`alert-dialog.tsx:19` 要不要加 `grid-cols-[minmax(0,1fr)]`，看删除确认在标题加了 `anywhere` 之后是否仍超出，没有一格抓得到就不加（05-lessons 第 1 节）。
- **测试**：e2e。对话框是固定定位，必须量对话框自己（4.3.1.1）。
  - **PG2 新格**（同 38.10）：在 375 下依次打开下面这些对话框，每个 `expectFits(page, dialog, dialog.getByRole("heading"))`，再 Escape 或"取消"：
    - 树的行 "Actions for <子页>" 里的 "Rename"、"Move to…"、"Delete"（`choosePageAction`，`E/fixtures/wiki-pages.ts:54`）；
    - 附件一节 "Actions for <附件>" 里的 "Rename"、"Move to…"（先例 `E/stories/asset/as1-upload.spec.ts:183-193`）。

    照 LW3 的做法（`:321-341`），在 1280 宽下经树打开，再设回 375（推断 375 下树的动作按钮不便点）。
  - **LW3 (page)**（三种浏览器）：在 `raw` 下建一个名字没有空格的父页（如 `folder_${"f".repeat(40)}`），对它打开 "Paste Markdown" 与上传（推断是 "Upload files"，照菜单核对），各 `expectFits(page, dialog, title)`。能不能经 API 这样建父页（决定 171 的"空的粘贴留作文件夹"）要先核对，不行就换一种让标题里出现长名的办法。
  - **在没修的代码上怎样失败**：`dialog` 的 `right` 或 `scrollWidth - clientWidth` 大于 0（移交量到 847–878）。
- **变异**：去掉 `DialogTitle` 的 `anywhere`：PG2 的改名、移动、附件两个，LW3 的两个，都失败。去掉 `AlertDialogTitle` 的：删除确认失败。
- **牵连**：S1 改 `W/app/confirm-dialog.tsx`，S3 只改 `components/ui/alert-dialog.tsx` 的标题类，不冲突。S5 改附件与粘贴、导入的对话框，可能碰 `asset-dialogs.tsx`、`source-add-dialog.tsx`，后合并的先 `git merge main`。

##### 38.12 375 宽下历史的第一个下拉仍超宽

- **现状**：同 26.17、32.1。"第一个"就是"谁"（`history-filters.tsx:120-135`）。移交量到 Chromium 75、WebKit 122，是加了 `max-w-full` 之后的数：`max-w-full` 管不到，下拉与它的 `div.space-y-2` 一起超宽。I3 写 `native-select.tsx:9`，`194ca350` 上 `max-w-full` 在 `:10`。
- **改法**：同 26.17 的 `:120`。文字不变。
- **测试**：MC3 的第 1 步，"谁"的外层 `div`，WebKit 那一份要有。
  - **在没修的代码上怎样失败**：外层右边 Chromium 847、WebKit 883（run2）。
- **变异**：去掉 `:120` 的 `min-w-0`，Chromium 与 WebKit 都失败。

#### 4.3.3 附：全应用里"名字和别的文字在同一行"的地方

- **风险的判据**：名字后面紧跟中性符号再接数字（日期、相对时刻、计数、编号），或者两个名字之间只有中性符号（" / "、" · "、箭头、引号），名字是从右到左的字母时就乱序（UAX #9 的 W、N 规则）。名字在段尾，或者后面接拉丁字母、汉字（强 L），风险低。
- **flex 项自成一段**：面包屑的每个 `li`、行里的链接各自是一个块，彼此不相互重排。
- `<option>` 里放不了元素，只能用字符串。要隔离可以用 U+2068/U+2069（FSI/PDI），不建议。

| # | 位置（`194ca350`） | 显示（en / zh-CN） | 名字之后 | 风险 | 处理 |
|---|---|---|---|---|---|
| 1 | `search-hit.tsx:108` | "Plans / A / B · ID x" | 另一个名字、" · ID" | 高 | S3（26.12） |
| 2 | `trash-row.tsx:260` | "From: A / B" / "原位置：A / B" | 另一个名字 | 高 | S3（26.12） |
| 3 | `trash-row.tsx:262` | "Deleted by {name} · 2 hours ago" / "{name} 删除 · 2 小时前" | en：" · 2" | 高 | S3（26.12） |
| 4 | `trash-row.tsx:279` | "Restores under “{parent}”" / "恢复到「{parent}」下" | 引号、句尾 | 低 | S3（26.12，同一组件、同一个改法） |
| 5 | `history-row.tsx:79` | "MCP ({name}) · 5 minutes ago" / "MCP（{name}） · 5 分钟前" | ") · 5" | 高 | S3（32.4） |
| 6 | `history-row.tsx:96` | "Renamed “A” to “B”, wrote “C”…" / "改名「A」为「B」、写入「C」…" | 引号、另一个名字 | 中 | S3（26.12） |
| 7 | `history-row.tsx:116`、`changeset-page.tsx:352` | "Undid “Edit · {name} · Oct 5, 2026…”" / "撤销了「编辑 · {name} · 2026年10月5日…」" | zh：" · 2026" | 高（zh） | S3（32.4） |
| 8 | `changeset-page.tsx:331` | "Client: MCP ({name})" / "客户端：MCP（{name}）" | 括号、句尾 | 中 | S3（32.4） |
| 9 | `changeset-change.tsx:34,46`（显示在 `:132`） | "Renamed: “A” → “B”" / "改名：「A」→「B」"；"Moved from “A” to “B”" | 箭头、另一个名字 | 高（次序与箭头反过来） | S3（32.4，2.3 的 P2-18） |
| 10 | `changeset-page.tsx:203`（面包屑的 `current`）；`history-row` 的 `aria-label` 不显示 | "Edit · {name} · Oct 5, 2026" / "编辑 · {name} · 2026年…" | zh：数字 | 高（zh） | v0.2（2.3 的 P2-37） |

其余的不在本 Step 里：

- **不做**：`byText`（`history-row.tsx:69`、`changeset-page.tsx:211`，段尾，理由见 32.4）；`history-filters.tsx:177`（"Page: {name}"，段尾）；`<option>` 里的（`history-filters.tsx:131,146`、`search-page.tsx:226`，放不了元素）。
- **进 v0.2**（风险低，只有 `unresolved.under` 为中）：`changeset-page.tsx:282`；`trash-page.tsx:246`、`:256-271`；`revert-dialog.tsx:451`、`transfer-report.tsx:405`（`reasonText`）；`page-versions.tsx:137`；`transfer-job-row.tsx:72`；`member-summary.tsx:18`、`invitation.tsx:161`、`ownerless-row.tsx:80`、`audit-section.tsx:27`；`edit-lock-note.tsx:58`、`pages-locked.tsx:47`、`edit-lost-banner.tsx:55`、`transfer-report.tsx:330-339`、`upgrade-page.tsx:663`；`unresolved-link.tsx:286`（M6 的对话框，不在 26.12 的话里）。
- **已在 `bdi` 里**：`search-page.tsx:280` 的 `Heading`、`history-row.tsx:93`、`changeset-page.tsx:343`、`history-ingest.tsx:50`、`search-hit.tsx:102,118`。

#### 4.3.4 附：对比度全表（8 位 sRGB，WCAG 2）

前景、背景都是渲染出来的 8 位值；半透明的已按 source-over 叠在背景上。12.4 与 26.9 只引这张表。

| 主题 | 用途 | 前景 | 背景 | 今天 | 改后 | 要求 |
|---|---|---|---|---|---|---|
| 亮 | 控件的环 `ring/50` 改实色 `ring`（0.708 改 0.556） | #d0d0d0 → #737373 | #ffffff | 1.54 | 4.74 | ≥ 3 |
| 亮 | 同上，对悬停的 accent 底 | → #737373 | #f5f5f5 | 1.49 | 4.35 | ≥ 3 |
| 亮 | 同上，对 primary 按钮本身 | #737373 | #171717 | — | 3.78 | ≥ 3 |
| 亮 | 链接（`auto` 半透明，改实色 2 px） | #d0d0d0 → #737373 | #ffffff | 1.54 | 4.74 | ≥ 3 |
| 亮 | 阅读视图、编辑器的实色外框 | #a1a1a1 → #737373 | #ffffff | 2.58 | 4.74 | ≥ 3 |
| 亮 | destructive 的环 `/40` 改实色 | #f5999d → #e7000b | #ffffff | 2.12 | 4.77 | ≥ 3 |
| 亮 | 无效字段的环 `ring-destructive/20` 改实色 | #faccce → #e7000b | #ffffff | 1.44 | 4.77 | ≥ 3 |
| 亮 | 菜单与选项的当前项：内嵌 2 px 的 `ring` | #737373 | #f5f5f5（accent）、#ffffff（popover、页） | 只有底色：accent 对 popover 1.09 | 4.35、4.74 | ≥ 3 |
| 亮 | 补全选中项的说明（muted 0.556 改 0.53） | #737373 → #6c6c6c | #f5f5f5 | 4.35 | 4.82 | ≥ 4.5 |
| 亮 | muted 文字在白底 | #737373 → #6c6c6c | #ffffff | 4.74 | 5.25 | ≥ 4.5 |
| 亮 | 命中的文字在高亮上 | #0a0a0a | #ffe891 | 16.22 | 不变 | ≥ 4.5 |
| 亮 | 高亮对页面（参考，靠字重） | #ffe891 | #ffffff | 1.22 | 不变 | （2.3 的 P2-17） |
| 暗 | 控件的环 `ring/50` 改实色（0.556 不变） | #3f3f3f → #737373 | #0a0a0a | 1.88（Chromium 实测；按公式是 #3e3e3e、1.85） | 4.18 | ≥ 3 |
| 暗 | 同上，对 popover、accent、primary 按钮 | #737373 | #171717、#262626、#e5e5e5 | — | 3.78、3.19、3.76 | ≥ 3 |
| 暗 | 阅读视图、编辑器的外框 | #737373 | #0a0a0a | 4.18 | 不变 | ≥ 3 |
| 暗 | destructive 的环 `/40` 改实色 | #6c2e2f → #ff6467 | #0a0a0a | 1.95 | 6.85 | ≥ 3 |
| 暗 | 无效字段的环 `ring-destructive/20` 改实色 | #3b1c1d → #ff6467 | #0a0a0a | 1.29（Chromium 实测） | 6.85 | ≥ 3 |
| 暗 | 菜单与选项的当前项：内嵌 2 px 的 `ring` | #737373 | #262626（accent）、#171717（popover）、#0a0a0a（页） | 只有底色：accent 对 popover 1.18 | 3.19、3.78、4.18 | ≥ 3 |
| 暗 | 补全选中项的说明 | #a1a1a1 | #262626 | 5.86 | 不变 | ≥ 4.5 |
| 暗 | 命中的文字在高亮上 | #fafafa | #655406 | 7.12 | 不变 | ≥ 4.5 |
| 暗 | 高亮对页面（参考） | #655406 | #0a0a0a | 2.66 | 不变 | （2.3 的 P2-17） |
| 暗 | destructive 按钮的白字 | #ffffff | #ff6467 | 2.89 | 加 `dark:bg-destructive/60`，叠成 #9d4042：6.47；按钮对页 3.06 | ≥ 4.5 |

- 取值来源：`W/styles.css:30-57`（background 1 / 0.145，foreground 0.145 / 0.985，accent 0.97 / 0.269，popover 1 / 0.205，primary 0.205 / 0.922，muted 0.556 / 0.708，ring 0.708 / 0.556，destructive `oklch(0.577 0.245 27.325)` / `oklch(0.704 0.191 22.216)`）；`W/reading/reading.css:13,17`；`W/editor/loaded/link-completion.ts:128-132`。
- 有彩色的照 OKLab 的完整矩阵换算到线性 sRGB 再编码；灰色的 Y = L³（4.3.1.4）。审查 B 的 `m12/p2-review-b/contrast.py` 另算一遍，全部一致；暗色 destructive 对页按浮点是 6.855，记 6.85。
- Chromium 实测的几行出自 `m12/p2-review-b/e2e-run3.txt`（没修）、`e2e-run4.txt`（加上第 1–5 条之后）。

#### 4.3.5 附：e2e 用时推断的算法

- 每格按"API 种数据约 0.5–1.5 s，每次 `goto` 约 1 s，每个对话框开关约 0.5 s"估，WebKit 按 1.3–1.5 倍；新项目的 worker 启动每个 2–4 s，1–2 个。墙钟按 2 个 worker 除以 2（CI 的 runner 推断 4 核，Playwright 默认取一半）。
- MC3 的实测（0.98 s、1.6 s）只有这样估出的 5 s、7 s 的五分之一，4.3.0.3 的合计偏保守。

### 4.4 S4 阅读视图（spec）

00 号文档第 7 节要 S4 先写 spec：阅读视图那个 layout effect 的重写，新的数据流与每种更新的处理，并对照 M6 以来依赖 `editable` 的增强逐一核对（移交第 48 条）。本节就是这份 spec；S4 不另写 `specs/` 文件（P2-20）。

#### 4.4.1 今天的数据流（`194ca350`）

- **读**：`usePageView`（`W/pages/page/page-view.ts:18-22`）以 SWR 的 `["page-view", 笔记本 id, 页 id]` 读视图，`eachRead`（`compare: Object.is`，`assets-expiry.ts:70`）让每次读都是新的对象；`useAssetsExpiry`（`assets-expiry.ts:98-125`）在第一个附件地址到期前一分钟重读，最多一小时（`rereadIn` `:44-46`）。服务端把地址签到"下一个整点之后的那个整点"（`asset/adapter/mac/sign.go:31-37`），所以同一小时里重读的 HTML 一字不差，跨了整点就只差地址的 `e` 与 `s`。事件（`pages`、`links`）经 `refresher` 重读同一个键（13.2 第 23 条）。
- **写进文章**：一个 `useLayoutEffect`（`W/pages/page/reading-view.tsx:133-232`），依赖 15 样（`:216-232`）：`html`、`revision`、`assetsExpire`、`enhancements`、`slug`、`notebookId`、`role`、`editable`、`t`、`preferences`、`page.id`、`mutate`、`pages`、`assets`、`navigate`。任何一样变了：清理（`:202-215`）先记下拿着焦点的任务勾选框（位置、文字、状态）与带 id 的元素（id、是否在视口里），再倒序撤掉增强；然后 `container.innerHTML = html`（`:138`），按序重跑增强（`:139-176`），把焦点还给"同一位置、同一文字与状态（或自己的勾选要的状态）"的勾选框（`:177-194`），或同 id 的元素（`:195-201`，原来在视口里而现在不在时滚到 `nearest`）。别的元素的焦点落到 `body`。
- **锚点**：第二个 `useLayoutEffect`（`:233-277`）按 `location.key + hash` 判断这次导航去过没有，去锚点的元素、或为缓存里的视图再读一次（`awaited`，`watchReader`）。
- **增强的上下文**（`W/reading/enhancement.ts:32-79`）：`role` 没有一个增强读它（`grep` 过 `W/reading/*.ts`：只有 `setAttribute("role", …)`）；`editable` 只决定给不给 `toggleTask`（`reading-view.tsx:152`），`taskToggle` 没拿到它就让勾选框照服务端写的 `disabled`（`W/reading/task-toggle.ts:14-18`）；`t` 由滚动区（`scroll-regions.ts:30`）与附件（`assets.ts:55`、`:138-146`）读；`locale` 由附件的大小读；主题经 `theme()`、`onThemeChange` 跟着变，不重跑（`diagrams.ts:140`、`:202`）。
- **增强撤掉之后的 DOM**：任务勾选框回到 `disabled`、去掉 `aria-label`（`task-toggle.ts:40-46`）；附件去掉提示与大小、`target`、`rel`（`assets.ts:85-102`），开始了的音视频记进 `kept`，下一次增强时由 `adopt` 以 `element.replaceWith(old)` 接回新 HTML 里同一附件的位置（`:156-189`）；图把原来的 `pre` 放回（`diagrams.ts:212`）；公式写回 TeX（`math.ts:298`）；滚动区与未建的链接去掉 `tabindex`、`role`、`aria-label`。代码高亮的清理只停下 Worker，不还原着色（`highlight.ts:59-62`）：撤掉之后的代码块仍是着色的 `span`。
- **版本页与回收站的预览**：`VersionView`（`W/pages/page/version-view.tsx:82-105`）同一种写法，只是什么焦点也不还（第 29 条）；用在 `version-page.tsx:175` 与 `W/pages/notebook/trash-preview.tsx:356`。

**于是**（逐项的现状在 4.4.4）：角色或升级一变（`editable`、`role`），整篇重写，浏览器的滚动锚点被移除，正文跳 94 px（第 48 条）；每小时的重读整篇重写，选区丢、展开的 callout 合上、正在全屏的视频被移出文档而退出全屏、图片重下（17.1、17.8）；别人的写、语言的切换，焦点只回到勾选框与带 id 的元素，链接、`summary`、滚动区的焦点落到 `body`（3、8.2、12.2、11.6）；同名标题因前面新加了一个而换了 id 时，焦点落到拿到原来 id 的那个（8.2）。

#### 4.4.2 新的数据流

**原则**：文章的 DOM 一旦写进去，只在内容真的变了的地方改；增强的上下文里会变而增强又不必重跑的东西，经读最新值的函数或订阅交给增强，同主题的先例（13.2 第 23 条）。

**一个钩子管文章的 HTML**：新文件 `W/pages/page/server-html.ts` 的 `useServerHTML(article, view, context)`，`ReadingView` 与 `VersionView` 都用它（第 29 条），它记着"文章里现在是哪一份服务端的 HTML"（`shown`：HTML 字符串、它解析成的 `<template>`、撤掉增强的函数、写进去时的 `t` 与 `locale`），按下表处理每一种更新。纯 DOM 的部分在新文件 `W/reading/patch.ts`，单独测（4.4.5）。

| # | 更新 | 怎样认出 | 今天 | S4 之后 |
|---|---|---|---|---|
| U1 | 第一次：挂上、换页、视图从没有变成有（`NotLoaded` 换回文章） | `shown` 为空，或文章元素不是 `shown` 记的那一个 | `innerHTML`、增强 | 同：`innerHTML`、增强 |
| U2 | 再读到同一份 HTML（版本号可能变了：只改了空白的保存） | 字符串相等 | HTML 与版本号都相等时不动；版本号变了整篇重写 | 不动 DOM，不重跑增强；上下文的 `revision` 与勾选发的 `base_revision` 读最新的（经 ref） |
| U3 | 只差附件地址（每小时的重读） | 两份 HTML 把附件地址的 `e`、`s` 换成占位之后相等（`addressesOnly`） | 整篇重写 | 就地换地址（`resign`）：不移除、不新建任何节点，不重跑增强；上下文的 `assetsExpire` 读最新的 |
| U4 | 内容变了（别人的写、自己的保存与勾选、链接的落点或别名变了） | 不是 U2、U3 | 整篇重写 | 记下焦点 → 撤掉增强 → 按块就地更新（`patchBlocks`）→ 重跑增强 → 还焦点（4.4.3） |
| U5 | 语言变了（`t`、`locale`） | 与 `shown` 记的不同 | 整篇重写 | 记下焦点 → 撤掉增强 → **不动 HTML** → 重跑增强 → 还焦点 |
| U6 | 主题变了 | `onThemeChange` | 不重跑 | 同 |
| U7 | 可写与否变了（角色、升级开始或结束、来源） | `canEditPageBody` 的值 | 整篇重写 | 不重跑；`taskToggle` 经新的 `onWritesChange` 只改勾选框的 `disabled` |
| U8 | 视图没了（缓存过期显示 `NotLoaded`）、换页、卸下 | `html` 为空、`page.id` 变、卸下 | effect 的清理撤掉增强 | 同：撤掉增强、清空 `shown` |

- **依赖里另外七样**（`enhancements`、`slug`、`notebookId`、`mutate`、`pages`、`assets`、`navigate`）都稳定，不触发上表的任何一种：slug 不可改（`W/pages/workspace/general-page.tsx:44`），`PageShell` 按页作 `key`（`page-layout.tsx:66`），`mutate` 按键稳定，store 是同一个对象。换页归 U1 与 U8。
- **U2、U3 不重跑增强**：`ReadingContext` 的 `revision` 与 `assetsExpire` 改成读最新值的 getter（`get revision() { return latest.current.revision }`），类型不变（仍是 `number`、`string | null`），增强照旧读字段。`toggleTask` 发的 `base_revision` 同样读 ref。版本号变而 HTML 不变时文章里的东西正是新版本的（服务端照字节渲染），所以用新的版本号是对的。
- **U3 `resign` 的做法**：在 `shown` 的模板与新的模板里各按文档次序取出所有"属性值是附件内容地址"的元素（`[src^="/api/v0/assets/"], [href^="/api/v0/assets/"]`），两边一一对应（`addressesOnly` 为真保证个数、标签、附件 id、`b`、`d` 都相同）；在文章里用同一个选择器取出活的元素，个数与每个的附件 id 对不上就退回 U4（增强不增删这样的元素：附件的提示与大小没有地址，`adopt` 按附件 id 与次序原位换；对不上说明有意外，宁可走 U4）。对得上就把新的值写到活的元素上：链接改 `href`；图片改 `src`（旧图显示到新图加载完，HTML 规范的 "update the image data"：节点不换，没写尺寸的也不跳）；还没开始的音视频改 `src`；**开始了的音视频不改**，照旧在失败时就地重签（`assets.ts` 的 `signAnew`，移交第 17 条的改法）。什么节点也不移除，所以选区、展开的 callout、焦点、全屏都还在（17.1、17.8）。 已加载的图也换 `src`，每张图每小时照今天重下一次；带宽由 P4/S2 量（M4-performance 第 11 条；主持者的决定 P2-47）。
- **地址的形状**：服务端写 `/api/v0/assets/<节点 id>/content?b=<blob id>&e=<十进制>&s=<22 个 base64url>`，下载的另有 `&d=1`（`asset/adapter/http/content.go:76-86` 的 `addressOf`、`upload.go:175-180` 的 `ContentURL`）；HTML 的属性里 `&` 写成 `&amp;`。`maskAddresses` 只作用于元素的 `src`、`href` 的值，不作用于整串 HTML（正文或代码块里照抄的一条地址仍是正文），只把 `e=` 之后的数字与 `s=` 之后的 22 个字符换成占位；`b`、`d` 与路径不动：换了文件（`b` 变）、换了显示与下载（`d`）都是内容变了（U4）。
- **Go 与网页共用的样例**（P2-45，照 `testdata/m10/virtual-targets.json` 与 `W/pages/page/virtual-target.test.ts:17` 的先例）：同一个小时里的两次重读地址逐字相同，PG15 这类 e2e 走不到正则对真地址的匹配，正则与服务端的拼法对不上时 `addressesOnly` 永远为假、17.1 与 17.8 悄悄没修好，而 vitest 用的是设计者理解的拼法。所以加一份 `testdata/m12/signed-view.json`：同一页在相隔一小时的两个时刻，由真的渲染器、`ContentURL` 与固定密钥的签名器写出的两份视图 HTML，带图、音频与下载链接。Go 一侧的测试（放在同时拿得到渲染器与 `ContentURL` 的组合根的 Markdown 测试旁，推断是 `bootstrap/markdown_app_test.go` 一带）断言渲染结果与样例逐字相同；网页一侧 `patch.test.ts` 读同一份样例，断言两份之间 `addressesOnly` 为真、`resign` 之后与第二份相同，把其中一个 `b` 或 `d` 改掉时为假。服务端的拼法或网页的正则任一边变了，都有一边红。S4 因此多一个只加测试与样例的服务端提交（第 3 节）。
- **U4 `patchBlocks` 的做法**（只看文章的顶层节点，不递归，P2-21）：
  1. 撤掉增强之后，文章的顶层节点应当与 `shown` 的模板的顶层节点一一对应（同个数、同 `nodeName`）。增强只在块里改（13.2 第 23 条"不改 HTML 的结构"；图把 `pre` 换成包装、撤掉时放回）。对不上就整篇 `innerHTML` 写新的，焦点照 4.4.3 的键去找（安全的退路，有一格测试）。
  2. 每个顶层节点的比较键：元素是 `outerHTML`，文字是 `data`；先 `maskAddresses`，再把 `id` 属性的值换成占位（同名标题的重新编号不算内容变了，8.2）。
  3. **先从后往前、再从前往后**比出相同的尾与头（尾先比：头与尾重叠时归给尾）。尾先比，是为了"在一个标题之前插进一节同名的"时，原来那一节留在原来的节点上：旧 `[A, H(#notes), p1]`、新 `[A, H'(#notes), p0, H(#notes-1), p1]`，尾比出 `p1`、`H`，头比出 `A`，中间新插 `H'`、`p0`；拿着焦点的旧 `H` 还是那个节点，只是 `id` 改成 `notes-1`。这只在原来那一节没被同一次保存改动时成立：`p1` 也变了时尾为空，头比把旧 `H` 配给新插入的 `H'`，焦点留在新插入的那个上（写进 4.4.8）。只在末尾追加时尾为空，头比出全部旧的，同样对。
  3a. **中间再按键贪心配一次**（P2-21）：链接的落点变了（别处新建、改名、删除一页）时，本页相隔很远的几处链接同时变，头尾之间的块很多。所以在旧的中间与新的中间之间再配一次：一张从键到旧序号队列（递增）的表，沿新的中间往后走，每个新块取大于上一个配上的旧序号的、最小的同键旧序号（队列里不大于它的弹掉）。配上的序号递增，留着的节点之间的次序不变，不必移动任何节点；仍是线性的（每个旧序号进出队列一次），不保证留下最多的块。空白的文字节点不作锚：不参与配对，随它两边的块删插。
  4. 相同的头与尾里的节点留着（同一个 DOM 节点），只把旧模板到新模板之间的属性差别（`id`、附件地址）照 U3 的办法写到活的节点上；**用户改的属性不碰**：`details` 的 `open`、锚点给的 `tabindex=-1`、活的媒体的 `src`（只按"旧模板 → 新模板"的差别改，不按"活的 → 新"）。块里活的结构与旧模板对不上（着色的代码块）而这一块又有属性差别时，这一块整个换新的（退路；代码块里没有附件地址与 id，实际碰不到）。
  5. 中间：没配上的活节点移除；没配上的新块（`document.importNode` 的副本）按新的次序插在它之后第一个留着的节点之前，没有就插在末尾。**留着的节点从不移动**：移动一个节点会先把它移出文档，正在全屏的视频随之退出、拿着焦点的元素失焦（HTML 的 focus fixup），所以只删、只插。
  6. 代价：一次比较要把两份模板的顶层节点各序列化一次，线性于 HTML 的大小；旧模板记在 `shown` 里，不再解析。不做 LCS：同一键在中间重复出现、又被挪了位置时，贪心可能少留几块，其中的选区与 callout 的开合丢失，焦点经 4.4.3 的键找回（写进 4.4.8；今天整篇都丢）。
- **`adopt` 不搬留着的媒体**（17.8）：U4 之后再跑 `assets` 时，`adopt`（`assets.ts:159-194`，`:189` 的 `replaceWith`）先把仍在文档里的 `kept` 元素从配对里去掉：它们就在原处，不占"附件 id + 第几个"的键（`keyed`，`:218-231`）。所以所在的块留着的媒体不被 `replaceWith`（它会先把元素移出文档，全屏就退出）；别人在它前面插入同一附件的嵌入时，新元素拿到"第 0 个"，也不会把留着的、在播的旧元素搬过去。块换了新的，照旧接回：这时它已离开过文档，全屏照旧退出，只在那一块被别人改了时。
- **U5 语言**：不动 HTML，只撤掉、重跑增强。文章的顶层节点与块里的节点都还在，焦点、选区、展开的 callout、宽内容的横向滚动都还在（11.6 的失焦与滚动归零；I3 把 11.6 的其余列为 C，滚动是顺带好的）。例外是增强自己重写的节点：公式（撤掉时写回 TeX）与图的包装（撤掉时放回 `pre`，重跑时新建，`diagrams.ts:157-163`、`:212`）里的选区与焦点不保留；图的包装是滚动区，拿着焦点时按 4.4.3 的键找回。代码块由下面的规则留住。
- **U7 可写与否**（第 48 条的改法）：`ReadingContext` 加两个可选字段，同 `theme()`、`onThemeChange` 的写法：`writes?: () => boolean`（现在能不能勾）与 `onWritesChange?: (listener: () => void) => () => void`（可写与否变了时调 `listener`，返回的函数停止）。移交第 48 条审查 C 给的是"由另一个 effect 改勾选框"，这里用上下文里的订阅，是等价的变体。阅读视图给 `writes: () => canEditPageBody(pages, page.id)`、`onWritesChange: (l) => reaction(() => canEditPageBody(pages, page.id), l)`，**`toggleTask` 总是给**（它在点下之后与发出之前已两次查 `canEditPageBody`，`reading-view.tsx:154`、`:156`，照旧）。`taskToggle`：没有 `toggleTask` 时同今天（版本页与回收站的预览）；有 `toggleTask` 而没有 `writes` 时当作可写（同今天，测试里的上下文多是这样）；两者都有时，按 `writes()` 设每个勾选框的 `disabled`，`onWritesChange` 时再设一次，清理时停止监听、全部 `disabled`。`aria-label` 的名称照 S1 的 6.3（读者的勾选框也按文字命名），不随可写与否变。`editable` 与 `role` 都不再进任何 effect 的依赖；`ReadingContext.role` 删掉（没有增强读它，P2-22）。
- **笔记本的模式变了**（`NotebookMode` 只有 `standard` 与 `llm_wiki`，`api/modules/notebook.yaml:454-459`：升级被批准时由 `standard` 变 `llm_wiki`、`upgrade` 随之清空，降级变回；升级的开始与驳回不改模式。S1 的 35.2 让未建链接按模式写 `leads`，`W/reading/unresolved-links.ts`）：`useServerHTML` 同样订阅 `notebook.mode`，变了就走 U5 的路：撤掉、重跑增强，不动 HTML。不换节点，不引起第 48 条的跳动；可写与否变了仍只改勾选框（U7）。
- **焦点与选区之外，增强自己的状态**：U2、U3、U7 不重跑增强，`math` 的 `kept`、`diagrams` 的 `drawings`、`assets` 的 `kept`、`reloadedFor`、`signing` 都不受影响；U4、U5 照今天撤掉再重跑，公式与图照今天"放回留着的排版、立即放回同主题同源码的图"（不闪）。代码高亮的撤掉不还原着色（`highlight.ts:59-62`），重跑时 Worker 答复就 `code.replaceChildren(markup)`（`:69-71`），留着的块里的代码选区会丢。所以 `codeHighlight` 记下每个代码块写进去时的语言与文字（`WeakMap<HTMLElement, string>`），重跑时跳过语言与文字都没变、已经着色的块：留着的块里的代码不闪、选区不丢（第 3 条的 C 部分，顺带）。
- **锚点**：第二个 effect 不变，搬进 `W/pages/page/view-anchors.ts` 的 `useViewAnchors`（14d）；它依赖 `html` 判断"为锚点再读的那一次到了"，U3 也让它跑一次，`anchored.current === navigation` 时只看 `awaited`，与今天相同。
- **第 6.2 条**（失去角色后勾选框仍可点）：`page-layout.tsx:198-215` 的 `toggleRefused` 与 `enter` 的拒绝（`:186-190`）遇到 `forbidden` 时重读这个工作区的笔记本列表（`void mutate(["notebooks", workspace.id]).catch(() => undefined)`，同 `revert-dialog.tsx:286-294`、`trash-row.tsx:321-323` 的先例）：角色变了，`canEditPageBody` 随之变，U7 把勾选框设为 `disabled`，"编辑"按钮随 `editable` 消失。文字照旧是拒绝的那一句（`setRefusal`）。改"编辑"的拒绝是 P2-23。

#### 4.4.3 焦点：记下与还回

U4 与 U5 之前，`useServerHTML` 记下 `document.activeElement`（在文章里时）、它是否在视口里（`shows`，`reading-view.tsx:307-310`）、它的**键**、它所在的顶层块；任务勾选框另记今天的那三样（位置、文字、状态）与自己的勾选要的状态（`toggled`，`reading-view.tsx:110`）。

**块经配对映射**（审查 A 的 M3、修订核对 A 的 M11）：块的序号不能原样用，焦点之上有插入或删除时同一序号已是别的块。

- 记下的是撤掉增强之后的那个活的顶层节点：增强一对一地换顶层节点（图把 `pre` 换成包装），撤掉之后同一序号就是它。
- `patchBlocks` 之后求它的新序号：它留着（头、尾或中间配上的），就是它的新位置；它被移除了，就在它原来那一段空隙（两个留着的节点之间）里，取同一次序插入的新块（这段里第 k 个被移除的对第 k 个插入的，插入的少就夹到最后一个）；这段没有插入，取空隙之后第一个留着的节点，在末尾就取最后一个。整篇退回 `innerHTML` 时取同一序号，越界夹到两端。U5 不动 HTML，序号不变。
- 重跑增强之后按这个新序号取顶层块：增强一对一地换，序号不变。
- 例：旧的是 `[p0, 图(焦点), p2]`，Bob 在页首插入一段，新的是 `[p_new, p0, pre, p2]`。图的 `pre` 在中间配上、留着，新序号 2，重跑之后那里是新的包装；用旧序号 1 会落到 `p0`。

之后按次序找要给焦点的元素，找到第一个就停：

1. **同一个节点**还在文档里：它（U5 与 U4 里留着的块多是这种；增强自己重写的节点，如图的包装，不是：它们走第 3 步）。
2. **任务勾选框**：今天的规则照旧（`:177-194`）：同一位置的勾选框，文字相同、状态相同或是自己的勾选要的状态。不是就不给焦点：不找别的，也不走第 4 步。`page-tasks.test.tsx:103`、`:146`、`:217` 三格钉着"别人的写挪来的项不给焦点，callout 照视图的样子"（M5/P6），照过（P2-24）。
3. **同键的元素**（在新的 DOM 里，按文档次序数第几个）：
   - 标题（`h1`–`h6`）：标签与可见文字（空白折叠）、同键里的第几个。不按 id：同名标题会重新编号（8.2）。
   - 链接（`a`）：`href`（附件地址先 `maskAddresses`）、可见文字、同键里的第几个。未建的链接（没有 `href`）按 `data-nw-target` 与文字。
   - `summary`（callout 的标题，12.2）：可见文字、同键里的第几个。
   - 音视频：附件 id 与同一附件里的第几个（同 `assets.ts` 的 `keyed`；`adopt` 已经给回焦点的，不再动）。
   - 别的带 id 的元素（脚注等）：id。
   - 滚动区（`.nw-scroll`：图的包装、宽的表格与代码块，`scroll-regions.ts:10`、`:40-43`）：所在顶层块（经上面的映射）与块里的第几个滚动区。图的包装在 U4、U5 之后是新节点，靠这个键找回。
4. **它原来所在的位置**（只用于不是任务勾选框的元素）：取上面映射到的那个顶层块；没有块就是文章本身。给它焦点用 `tabindex=-1` 与 `preventScroll`，**不展开 callout、不滚动**（不用 `focusOn`）：今天焦点落 `body`、视口不动，这一步只补焦点，不把读者看的地方拉走（`reading-view.tsx` 的文件头："a view read again stays where it is"）。

给焦点一律 `preventScroll`；第 1–3 步里原来在视口里、现在不在的，`scrollIntoView({ block: "nearest" })`（今天对带 id 的元素的规则，推广到这三步；第 4 步不滚动）。读者其间动过（`watchReader`）不在这里考虑：这一切在同一个同步的 layout effect 里，读者没有机会动（13.2 第 26 条说的是"等异步的答复之后"）。

**第 4 步是本文的取舍**（P2-24）：今天找不到同 id 的元素时焦点落到 `body`（`reading-view.test.tsx:500-503` 断言了它）；B 类的要求是不失焦。给它原来所在的那一块，读屏念出那一块，Tab 从那里接着走；任务勾选框不走这一步，视口不动。那一格的断言随之改（4.4.5）。

**丢掉的一条**（修订核对 A 的 M15）：标题按文字找，不按 id。带 id 的标题换了文字（同 id）时第 3 步找不到，走第 4 步：焦点仍落在映射到的那一块（多半就是新的标题），但原来看得见、现在看不见时不再滚出来（M6/P3 的"shown again if it showed and no longer does"，`reading-view.tsx:59-62`，对改了文字的标题不再成立）。`reading-view.test.tsx:492-494` 随之改（4.4.5），`reading-view.tsx` 的文件头照改，写进 4.4.8。别的带 id 的元素（脚注等）按 id 找，规则照旧。

#### 4.4.4 逐项

**3 阅读视图更新时闪动、失焦**（B 的失焦部分；高亮闪是 C）
- 现状：`W/pages/page/reading-view.tsx:138` 每次整篇写 `innerHTML`；焦点只回到勾选框与带 id 的元素（`:177-201`），链接、`summary`、滚动区落到 `body`；代码高亮重新着色前闪一下（`W/reading/highlight.ts:41-78` 没有缓存）。公式与图已不闪（I3：部分修了）。
- 改法：U4（4.4.2）与 4.4.3。
- 测试：vitest `reading-view.test.tsx` 新格 "another's write elsewhere in the page keeps the focus on a link without an id, the selection in a paragraph and a callout opened": 视图 `<p><a href="/x">x</a></p><details><summary>S</summary><p>in</p></details><p id="last">tail</p>`，焦点给链接，读者打开 callout，选中 `tail` 的一段；假服务端改第三块（`<p id="last">tail, edited</p>`）、版本号加一，`reload`。断言：`document.activeElement` 是同一个链接节点（`toBe` 之前取的引用）、`details.open` 为真、`document.getSelection().anchorNode` 仍在文档里且是原来那个文本节点。没修时：链接是新的节点，焦点在 `body`；`details` 是新的、合着。e2e PG15 第 1 格（4.4.6）在三种浏览器里核对同样的三样。
- 测试（P2-21 的中间贪心配对）：`reading-view.test.tsx` 新格 "a write that changes two links far apart keeps the blocks between, a callout opened and the focus in them"：视图 `<p>a [[X]]</p><details><summary>Tip</summary><p>in</p></details><p>mid</p><p>b [[X]]</p>`，读者打开 callout、焦点给 `summary`；新 HTML 只把首尾两段里的链接从未建换成已建。断言 `details` 与 `summary` 是原来的节点、`open` 仍为真、焦点在原来的 `summary` 上。只有头尾比较时中间整段换新，`open` 为假。

**8.1 后退、前进到带锚点的地址时浏览器恢复的滚动盖过视图的滚动**（B，中，推断）
- 现状：锚点的 effect 在 `reading-view.tsx:233-277`，去锚点时 `focusOn(target, {})` 滚过去（`:255`）。应用没有设 `history.scrollRestoration`，也没有 React Router 的 `<ScrollRestoration>`（`W/main.tsx:38` 只是 `createBrowserRouter(routes)`，全库 `grep` 不到这两个名字）。同一个文档里的历史条目，浏览器在 `popstate` 时恢复滚动，应用之后才渲染、滚到锚点，按理视图的滚动在后；移交与 I3 都是读代码推出的，没有在浏览器里复现过。
- 改法：**先探查、复现了才改**（P2-25）。S4 的第一个提交之前，用 L1 的形状（长页、远处的标题、`[[#Part A]]`）在三种浏览器里各跑：去锚点 → 去别的页 → `page.goBack()`，断言标题拿着焦点并在视口里，各 10 次。没有一次复现：8.1 移进发布之后的人工清单（与第 13.3 条同列，写探查的记录），本 Step 不改代码。复现了：锚点的 effect 在 POP 导航（`useNavigationType() === "POP"`）去锚点之后，下一帧（`requestAnimationFrame`）再看一次：焦点仍在目标上、读者没动过（`watchReader`）、目标不在视口里，就再 `scrollIntoView`；测试是探查用的那一格，进 PG15，三种浏览器。
- 拿不准：浏览器的恢复可能晚于一帧（Chromium 对跨文档的恢复会等内容长高）；探查时记下恢复发生在 `popstate` 之后多久。

**8.2 重读之后没有 id 的元素失焦；同名标题换了 id 时落到拿到原来 id 的那个**（B）
- 现状：`reading-view.tsx:195-215` 只按 id 还焦点。
- 改法：U4 的"尾先比"留住原来的节点（原来那一节没被同一次保存改动时；改动了的情形写进 4.4.8），4.4.3 的标题键按文字与次序，不按 id。
- 测试：vitest 新格 "a heading of the same name put before the focused one leaves the focus on the one the reader had": 视图 `<h2 id="nw-notes">Notes</h2><p>one</p>`，焦点在标题；新 HTML `<h2 id="nw-notes">Notes</h2><p>zero</p><h2 id="nw-notes-1">Notes</h2><p>one</p>`。断言焦点是原来那个节点，它的 `id` 已是 `nw-notes-1`，其后是 `one`。没修时焦点落到第一个（新插入的）标题。`patch.test.ts` 另有纯函数的一格。

**12.2 callout 的 `summary` 拿着焦点时重读，焦点落到 body**（B；右栏与 callout 的开合是 C）
- 现状：`summary` 没有 id，`reading-view.tsx:203-215` 不记它。
- 改法：U4（块没变就是同一个节点）；块变了按 4.4.3 的 `summary` 键。callout 的开合在留着的块里自然保留（C 的一半，顺带）；换掉的块里的不还原（C，进 v0.2）。
- 测试：vitest 新格 "a callout's summary that had the focus has it back, its callout written again by another": `<details><summary>Tip</summary><p>a</p></details>`，焦点给 `summary`；新 HTML 只改 `<p>b</p>`。断言焦点在新的 `summary` 上（文字 Tip）。没修时在 `body`。

**29 版本页与回收站的预览重读时整篇换 `innerHTML`，焦点落到 body**（B）
- 现状：`W/pages/page/version-view.tsx:82-105`，什么焦点也不还；`version-page.tsx:175`、`W/pages/notebook/trash-preview.tsx:356` 用它。重读来自树的变化与 `links` 带 `aliases` 的事件（13.2 第 23 条）。
- 改法：`VersionView` 改用 `useServerHTML`（同一套 U1–U8，没有 U7：不给 `toggleTask`）。
- 测试：vitest `W/pages/page/version-page.test.tsx` 新格 "a version's view read again keeps the focus on its link": 版本视图里一个链接拿着焦点，触发一次 `links` 事件（带 `aliases`）让它重读、新 HTML 只改另一段。断言焦点仍是那个链接节点。没修时在 `body`。回收站的预览走同一个组件，不另写。

**6.2 失去角色的编辑者在笔记本列表重读之前勾选框仍可点**（A，小，推断）
- 现状：`reading-view.tsx:152-156`；`page-layout.tsx:198-215` 的 `toggleRefused` 对 403 只 `setRefusal`，不重读笔记本列表，角色要等别的重读才变。I3 备注"服务端拒得对、文字也对，同'编辑'按钮；可算 C"。
- 改法：4.4.2 末条。
- 测试：vitest `W/pages/page/page-tasks.test.tsx` 新格 "a toggle refused as forbidden reads the notebooks again, and with the role gone the checkboxes are disabled, the view not written again": 写者勾选，假服务端答 403 `forbidden`，笔记本列表此后给 `reader`。断言：发出了 `GET …/notebooks`；勾选框 `disabled`；文章里的节点是原来那些（U7）；拒绝的文字在。没修时没有那个请求，勾选框仍可点。"编辑"另一格：进入编辑答 403 之后"编辑"按钮消失。

**11.1 mermaid 的几个 id 不以图的 id 开头**（A，小）
- 现状：`W/reading/diagrams.ts:285-287` 的 `withOwnIds` 只把以这张图的 id（`nw_mermaid_N`）开头的换掉；时序图、桑基图的 `actor0`、`root-0`、`linearGradient-N` 不换，同一页两张这样的图 id 重复，渐变可能指到另一张。
- 改法：`withOwnIds` 先收集 SVG 里所有的 `id="…"`，**不包含**图的 id 的每个改成 `<新 id>-<原 id>`。包含图的 id 的不再改名：mermaid 11.17.2 为 `accTitle`、`accDescr` 写的 `chart-title-<图的 id>`、`chart-desc-<图的 id>`（`mermaid.core.mjs:891-905`）今天的替换已让它们各不相同。再把对改了名的 id 的引用一起改：`url(#原 id)`、`href="#原 id"`、`xlink:href="#原 id"`、`<style>` 里的 `#原 id`（后面不是标识符的字符），以及 IDREF 属性 `aria-labelledby`、`aria-describedby` 里的那一项。原 id 先按正则转义。只改 mermaid 写的标记（13.2 第 23 条的例外），写者的文字经 mermaid 的 `dompurifyConfig` 已去掉 `id`。
- 测试：vitest `W/reading/diagrams.test.ts` 新格 "two drawings whose elements' ids are not the drawing's own have no id in common, and each names its own"：假的 `Drawer` 交回带 `id="actor0"`、`<use href="#actor0"/>`、`fill="url(#linearGradient-1)"`、`<style>#actor0{…}</style>` 的 SVG，同一页两张。断言文章里没有重复的 id，每张的引用都指向自己里面的元素。没修时两张都有 `actor0`。 假的 SVG 另带 `<title id="chart-title-ID">` 与 `aria-labelledby="chart-title-ID"`：断言图仍有这个可读的名字（审查 A 的 M9）；再带 `<text id="actor0-label">` 与另一个元素的 `aria-labelledby="actor0-label"`：断言改名之后这个 `aria-labelledby` 指向同一张图里改了名的那个元素（修订核对 A 的 M13）。
- 等价的一处：包含图的 id 的 id 也改名时，IDREF 的改写把引用跟上，结果相同；不当变异（证据：上面两种 IDREF 都由同一个改写处理，第一种在两种写法下都照过）。

**11.6 换语言时整个视图重新增强，没有 id 的元素失焦**（C，失焦的部分随第 3 条）
- 现状：依赖里的 `t`、`preferences`（`reading-view.tsx:225-226`）一变就整篇重写。
- 改法：U5。
- 测试：改 `reading-view.test.tsx:90` 那一格：换语言之后 `log` 仍是撤掉一次、再跑一次（今天的断言照旧），另断言文章的第一个子节点是同一个节点、拿着焦点的链接仍拿着焦点。没修时节点换了、焦点在 `body`。
- 测试（增强自己重写的节点，审查 A 的 M2）：`W/reading/highlight.test.ts` 新格 "an unchanged block already coloured is not written again"：跑一次、等着色、撤掉、再跑，`MutationObserver` 在代码元素里记不到 `childList` 的变化，选区仍在；改了文字的块照旧着色。`reading-view.test.tsx` 新格 "a drawing in a callout that had the focus has it back after the language changes"：图放在打开的 callout 里（`<details open><summary>S</summary><pre><code class="language-mermaid">…</code></pre></details>`，假的 `Drawer`），不放在顶层：顶层的图，第 4 步取到的那一块正是新包装，去掉滚动区的键也照过（修订核对 A 的 M12）。jsdom 里包装要能拿焦点（`scroll-regions.ts:33` 只给宽的滚动区 `tabindex`）：stub `ResizeObserver`，宽度定在原型上、按类判断：`HTMLElement.prototype` 的 `scrollWidth` getter 对 `.nw-diagram` 答大于 `clientWidth` 的值。不照 `scroll-regions.test.ts:15-17` 定在元素实例上：换语言或 U4 之后的包装是重跑时新建的 `div`（`diagrams.ts:157`），`diagrams` 排在 `scrollRegions` 之前（`enhancement.ts:101-102`），新包装在同一次运行里就被判宽度，实例上的定义还没有，它拿不到 `tabindex`，照设计实现的代码也红（修订核对 A 的 N13）。焦点在包装上，换语言之后焦点在同一个 callout 里的新包装上。没修时前一格记到一次 `replaceChildren`，后一格焦点在 `body`；去掉滚动区的键时第 4 步把焦点给 callout，后一格红。
- 测试（块经配对映射，修订核对 A 的 M11）：`reading-view.test.tsx` 新格 "a drawing that had the focus has it back as a paragraph is put before its block"：同上的 callout 与图（宽度同样定在原型上）放在两段之间（`[p0, callout, p2]`），焦点在包装上；新 HTML 在页首插入一段。断言焦点在同一个 callout 里的新包装上，callout 是原来的节点。没修时焦点在 `body`；用记下的旧序号（不映射）时，第 3 步在 `p0` 里找滚动区、找不到，第 4 步把焦点给 `p0`，红。

**17.1 每小时整篇替换文章**（A，中）
- 现状：`reading-view.tsx:138`；依赖 `html`、`assetsExpire`（`:217-219`）；`assets-expiry.ts:98-125` 的计时器按时重读；跨整点之后地址的 `e`、`s` 变了，HTML 就不同。选区丢、callout 合上、图片重下、没写尺寸的图片跳。
- 改法：U3。
- 测试：
  - `W/reading/patch.test.ts`（新）："addressesOnly is true when only the addresses' expiry and signature differ, false for another blob, a download, or any other change"（五格各一个反例）；"resign gives the links, the images and the media not begun their new addresses, and keeps every node: a begun one keeps its own"。
  - `reading-view.test.tsx` 新格 "a view read again whose attachments' addresses alone changed keeps every node, the selection and the callouts, runs no enhancement again, and the links and images have the new addresses"：视图带一张图、一个链接、一个 callout（读者打开了），选区在一段里；用假定时器推到到期，假服务端交回只改了 `e`、`s` 的 HTML。断言：`MutationObserver` 没有记到 `childList` 的变化；`img.src`、`a.href` 是新的；`log` 没有新的撤掉与运行；`details.open`；选区的节点仍在。没修时节点全换、`log` 多一次。
  - 上下文的 `assetsExpire` 读最新的：`reading-view.test.tsx:820` 那一格加一步：U3 之后，增强读到的 `context.assetsExpire` 是新的值。

**17.8 替换时原生全屏退出**（A，中，随 17.1）
- 现状：`innerHTML` 把正在全屏的视频移出文档，全屏随之退出；`assets.ts:189` 的 `adopt` 之后把它接回，那时已退出。
- 改法：U3 不移除任何节点；U4 不移动留着的节点，`adopt` 不让仍在文档里的媒体占配对的键（4.4.2）。剩下的：视频所在的那一块被别人改了，块换新、视频被接回，全屏照旧退出（写进 4.4.8 的限制）。
- 测试：vitest `W/reading/assets.test.ts` 新格 "a started video still in its place as the enhancement runs again is not moved"：在 `container` 里放一个开始了的视频，撤掉、再跑（不动 HTML），`MutationObserver` 没有记到它被移除。没修时 `replaceWith` 记到一次移除与一次加入。e2e：PG15 第 3 格（Chromium；Firefox 与 WebKit 的无头模式给不给全屏，S4 先试：给就三种都断言 `document.fullscreenElement` 仍是那个视频，不给就只断言它没有离开过文档，`MutationObserver`）。视频的字节：AS3 的 `recordWebm` 只在 Chromium（WebKit 没有 `MediaRecorder`，05-lessons 第 2 节之外的已知），WebKit 一格用 WAV 音频只核对"没有离开过文档"。
- 测试（审查 A 的 M6）：`assets.test.ts` 新格 "a started clip whose block stays is not moved when the same clip is embedded before it"：在播的 `clip.mp4` 所在的块留着，前面插入同一附件的嵌入，再跑 `assets`：在播的元素仍在原来的块里、没有离开过文档，新的嵌入是新元素。只跳过 `old === element` 时它被搬到新的位置。在播的元素照这个文件的 `play()`（`assets.test.ts:73-95`）造，`error` 是 `null`：jsdom 的 `HTMLMediaElement.error` 是 `undefined`，`adopt` 会走"失败"一支、不搬，没修的代码上也绿（修订核对 A 的 N10，探针证实）。

**48 横幅出现时正文跳动**（C 改判为 A，中）
- 现状：`reading-view.tsx:131-132` 读 `role` 与 `editable`，都在依赖里（`:223-224`）；升级开始，`writesNotebook`（`W/app/effective-role.ts:51-53`）变了，整篇重写，浏览器的滚动锚点被移除，这一帧不补偿，正在读的段落往下跳 94 px（横幅 70 加间距 24，`W/pages/notebook/notebook-layout.tsx:79` 的 `UpgradeBanner`）；升级结束、角色变化同样。
- 改法：U7。`ReadingContext.role` 删掉（P2-22）。M6 以来依赖 `editable` 的增强逐一核对的结果在 4.4.9。
- 测试：
  - `reading-view.test.tsx` 新格 "the notebook's mode changing runs the enhancements again without writing HTML"：LLM Wiki 的升级被批准（`mode` 由 `standard` 变 `llm_wiki`，`upgrade` 清空），`log` 多一次撤掉与运行，`MutationObserver` 在文章的顶层记不到变化，未建的 `index` 链接的 `leads` 随之更新（S1 的 35.2）。没订阅模式时 `log` 不变，`leads` 停在旧的。
  - vitest `reading-view.test.tsx` 新格 "the notebook's upgrade starting writes no HTML: the checkboxes become disabled, a link keeps the focus"：写者打开页，焦点在一个链接上；笔记本的 `upgrade` 变成非空（`NotebookStore` 的 `put`，或 `mode` 事件重读的列表）。断言：`MutationObserver` 没有 `childList` 的变化；`log` 没有新的撤掉与运行；勾选框都 `disabled`；焦点在原来的链接。再结束升级：勾选框又可勾、有名称。没修时 `log` 多一次撤掉与运行、节点全换。
  - `W/reading/task-toggle.test.ts` 新格 "the checkboxes follow whether the context writes, without running again"：`writes` 先真后假再真，`onWritesChange` 调监听，断言 `disabled` 跟着变、监听在清理之后停止。
  - e2e：UP1 (page)（`e2e/stories/upgrade/up1-start-freeze.spec.ts:171`，已在 `threeBrowsers`）的 `tab` 先打开 Plan：Plan 的正文加长到要滚动，`tab` 滚到一段在视口中部，记下它的 `getBoundingClientRect().top` 与文章第一个子节点的身份（`window` 上的一个引用）；横幅出现之后，断言那个子节点还是同一个（三种浏览器），那一段的 `top` 差不过 1 px（Chromium 与 Firefox：它们有 CSS 的滚动锚定）。WebKit 有没有滚动锚定，S4 先探查：有就三种都断言 `top`；没有就 WebKit 只断言节点没换，写进 4.4.8（横幅本身推下正文，不是阅读视图的事）。

**14d `reading-view.tsx` 按职责拆开**（D，随 S4）
- 现状：325 行，`:133-277` 管四件事：HTML 与增强、任务勾选的焦点与状态、锚点、未建链接的对话框（后者已在 `unresolved-link.tsx`）。
- 改法：`W/reading/patch.ts`（纯 DOM：`maskAddresses`、`addressesOnly`、`resign`、`patchBlocks`、`focusKeyOf`、`findByKey`）；`W/pages/page/server-html.ts`（`useServerHTML`：U1–U8 与 4.4.3）；`W/pages/page/view-anchors.ts`（`useViewAnchors`：今天的第二个 effect，照搬）；`reading-view.tsx` 只组合它们、给上下文、管 `toggled`。`version-view.tsx` 用 `useServerHTML`。knip：每个新导出都有人用。
- 测试：搬动的部分由原有的测试照过（`reading-view.test.tsx` 37 格、`page-tasks.test.tsx` 11 格、`task-toggle.test.ts` 8 格，除 4.4.5 列的几处不改）。

#### 4.4.5 已有的测试要随之改的

- `reading-view.test.tsx:456-504` 末一步（`:500-503`）：新 HTML 里没有同 id 的元素时，焦点今天落到 `body`；S4 之后给它原来所在的那一块（4.4.3 第 4 步：映射到的顶层块，这里就是同一位置的新元素，`tabindex=-1`，不滚动）。断言改为焦点在那一块上，并断言没有调用 `scrollIntoView`。
- `reading-view.test.tsx:492-494`（同一格的"It showed and no longer does"）：标题从 "A, still" 改成 "A, moved"，第 3 步按文字找不到，走第 4 步，不再滚动（4.4.3 的"丢掉的一条"）。`activeElement` 照旧是新的标题（它就是映射到的那一块）；`scroll` 的断言改为没有调用。`:497-499` 照过。
- `reading-view.test.tsx:90-117`：换语言之后的断言照旧（撤掉一次、再跑一次），加"同一个节点"一句（11.6）。
- 测试里自己构造 `ReadingContext` 的九个文件去掉 `role`：`W/reading/enhancement.test.ts`、`highlight.test.ts`、`diagrams.test.ts`、`assets.test.ts`、`unresolved-links.test.ts`、`app-links.test.ts`、`scroll-regions.test.ts`、`math.test.ts`、`task-toggle.test.ts`（`:31`；tsc 报出）。
- `reading-view.test.tsx:20-25` 的 `recording` 解构并记下 `context.role`，`:39`、`:46-47` 的期望串带 "editor"：删掉 `role` 之后改成不记它，期望串随之（tsc 报出）。
- `W/pages/page/source-actions.test.tsx:249-271`（"retained task callback follows source/role"，M10 的来源只读守卫）：`:265` 的 `expect(latest?.toggleTask).toBeUndefined()` 在 S4 之后必红（U7 不重跑增强，`toggleTask` 总是给）。删掉这一行，改为断言文章的节点没换、勾选框 `disabled`、点了不发正文的写。守卫换成了 `writes()`（来源页为假）与 `reading-view.tsx:154`、`:156` 的两次 `canEditPageBody`；后半段的"没有正文的写"照旧有效。
- `page-tasks.test.tsx` 的 11 格照过：勾选让任务所在的列表块变了，块整个换新，焦点经 4.4.3 第 2 步（今天的规则）找回；找不回的照旧不给焦点（`:103`、`:146`、`:217`）。

#### 4.4.6 e2e

- **新故事 PG15**（`e2e/stories/page/pg15-reading-updates.spec.ts`，三种浏览器，进 `threeBrowsers`）："PG15 (page): the reading view updated in place: another's write elsewhere in the page keeps the focus on a link, a callout opened, the selection and an audio playing; a heading of the same name put before the focused one leaves the focus on it"。理由：L1 第 4 格（`l1-links.spec.ts:219`）已核对"重读之后带 id 的标题拿着焦点、视口不动"，但 L1 整个文件进 `threeBrowsers` 要多跑它的 API 与 MCP 格；阅读视图的焦点与滚动要三种引擎（05-lessons 第 6 节），单独一个故事最省。接口版本与 MCP 版本豁免：只是网页的行为。
  - 第 1 格：Ada 打开页（长页：一个链接、一个折叠的 callout、一段文字、一个 WAV 音频，远处一个 `## Notes`），焦点给链接、打开 callout、选中一段、播放音频；Bob 经接口改页尾的一段。等那一次重读答了（`waitForResponse` 那一次 `GET …/view`，05-lessons 第 6 节：在同一个文档里等那次本会触发它的事答了），断言：链接拿着焦点（`toBeFocused`）、callout 开着、`getSelection().toString()` 是原来的文字、音频没停（`paused` 为假）且它与文章的第一个子节点都是同一个节点（`window` 上的引用）。 音频用 WAV（`e2e/fixtures/assets.ts` 的 `wav`，S4 与 S5 先合并的那个加，6.4 第 6 个提交、6.5 第 15 个提交都写了）：本机 Playwright 的 WebKit 放不出 `oggOpus`，`readyState` 10 秒内一直是 0（修订核对 C 的 N8）；修订核对 C 的探针在三种引擎里放 WAV 都不停（`currentTime > 0.3`、`paused` 为假，审查记录修订核对第 3 轮的 N19）。
  - 第 2 格：焦点给 `## Notes`；Bob 在它前面插一节同名的 `## Notes`。断言拿着焦点的标题之后是原来的那段文字，它在视口里。
  - 第 3 格（17.8，Chromium；另两种看 4.4.4 的探查）：视频全屏，Bob 改别处，断言 `document.fullscreenElement` 仍是它。
  - 8.1 复现了才加的一格（4.4.4）。
- **UP1 (page)** 加 48 的断言（4.4.4），已在 `threeBrowsers`。
- 推断用时：PG15 三格 × 三种浏览器约 9 个测试，每个 6–10 秒，两个 worker 摊开约 +35–45 秒；UP1 每种浏览器多约 1 秒的滚动与断言。

#### 4.4.7 变异

| # | 变异 | 让它失败的测试 |
|---|---|---|
| M4-1 | U4 退回整篇 `innerHTML` | `reading-view.test.tsx` 的"another's write elsewhere…"（焦点、callout、选区）；PG15 第 1 格 |
| M4-2 | `patchBlocks` 移动留着的节点（先全移除、再按新次序插回） | 同上（焦点落到 `body`、callout 合上）；PG15 第 1、3 格 |
| M4-3 | 头先比、再比尾 | "a heading of the same name put before…"；`patch.test.ts` 的同形一格；PG15 第 2 格 |
| M4-4 | 焦点的键里去掉 `summary` | "a callout's summary that had the focus…" |
| M4-5 | `VersionView` 仍写 `innerHTML` | `version-page.test.tsx` 的 "a version's view read again keeps the focus on its link" |
| M4-6 | `toggleRefused` 不重读笔记本 | `page-tasks.test.tsx` 的 "a toggle refused as forbidden…" |
| M4-7 | `withOwnIds` 只换图的 id 开头的 | `diagrams.test.ts` 的 "two drawings whose elements' ids…" |
| M4-8 | U5 也写 `innerHTML` | `reading-view.test.tsx:90` 那一格的"同一个节点" |
| M4-9 | `addressesOnly` 恒为假 | "a view read again whose attachments' addresses alone changed…"；`patch.test.ts` 的正例 |
| M4-10 | `resign` 也改开始了的音视频的 `src` | `patch.test.ts` 的 "…a begun one keeps its own" |
| M4-11 | 上下文的 `assetsExpire` 是写进去那一刻的值 | `reading-view.test.tsx:820` 那一格加的一步 |
| M4-12 | `adopt` 让仍在文档里的媒体照样占配对的键（只跳过 `old === element`） | `assets.test.ts` 的 "a started clip whose block stays is not moved when the same clip is embedded before it"；"a started video still in its place…"；PG15 第 3 格 |
| M4-13 | `editable` 回到依赖里 | "the notebook's upgrade starting writes no HTML…"；UP1 (page) 的节点身份 |
| M4-14 | `taskToggle` 不听 `onWritesChange` | `task-toggle.test.ts` 的 "the checkboxes follow…"；"the notebook's upgrade starting…" 的 `disabled` |
| M4-15 | `maskAddresses` 也把 `b` 换成占位 | `patch.test.ts` 的"another blob"反例 |
| M4-16 | 焦点第 4 步去掉（找不到就不给）；或第 4 步滚动、展开 callout | `reading-view.test.tsx:500` 改过的那一步（焦点在那一块上、没有调用 `scrollIntoView`） |
| M4-17 | 顶层对不上时不退回整篇写（直接按序号比） | `patch.test.ts` 的"结构对不上"一格（撤掉增强之后多出一个顶层节点） |
| M4-18 | 第 4 步也用于任务勾选框 | `page-tasks.test.tsx:103`、`:146`、`:217`（焦点在 `body`、callout 照视图的样子） |
| M4-19 | 去掉中间的贪心配对（只比头尾） | "a write that changes two links far apart keeps the blocks between…" |
| M4-20 | `codeHighlight` 照旧重写没变的、已着色的块 | `highlight.test.ts` 的 "an unchanged block already coloured is not written again" |
| M4-21 | 焦点的键里去掉滚动区 | "a drawing in a callout that had the focus has it back after the language changes"（图在 callout 里，第 4 步落在 callout 上） |
| M4-22 | IDREF（`aria-labelledby`、`aria-describedby`）不随改名改 | `diagrams.test.ts` 的那一格：`aria-labelledby="actor0-label"` 指不到元素 |
| M4-23 | `maskAddresses` 作用于整串 HTML | `patch.test.ts` 新格：正文里照抄一条只改了 `e`、`s` 的地址，`addressesOnly` 为假 |
| M4-24 | 不订阅笔记本的模式 | "the notebook's mode changing runs the enhancements again without writing HTML" |
| M4-25 | `maskAddresses` 的正则与服务端的拼法不合（例如不认 `&amp;`） | `patch.test.ts` 读 `testdata/m12/signed-view.json` 的那一格；服务端一侧改拼法时 Go 的样例测试红 |
| M4-26 | 块不经配对映射（第 3、4 步用记下的旧序号） | "a drawing that had the focus has it back as a paragraph is put before its block" |

#### 4.4.8 S4 之后仍在的限制（写进 v0.2 的 `M12-polish.md` 或人工清单）

- 同一键在中间重复出现、又被挪了位置时，贪心的配对可能少留几块：其中的选区丢失、callout 的开合不还原（C）。
- 同一次保存既在一个标题之前插进同名的一节、又改了原来那一节：头比把旧标题配给新插入的那个，焦点留在新插入的标题上（8.2 的剩余）。
- 公式与图的包装由增强重写：U4、U5 之后其中的选区不保留；图的包装拿着的焦点按键找回到新的包装。
- 带 id 的标题改了文字时，焦点落在映射到的那一块，原来看得见、现在看不见时不再滚出来（4.4.3 的"丢掉的一条"）。
- 视频所在的那一块被别人改了：视频被接回，全屏退出（17.8 的剩余，同移交原来记的"已知的限制"）。
- WebKit 没有 CSS 滚动锚定时，横幅本身仍把正文推下 94 px（看 4.4.4 的探查，没有就写进人工清单与已知限制）。
- 8.1 没复现：进人工清单。

#### 4.4.9 M6 以来依赖 `editable` 的增强，逐一核对（移交第 48 条）

在 `194ca350` 上 `grep` 了 `W/reading/*.ts` 与 `reading-view.tsx`、`version-view.tsx` 的每一处 `editable`、`toggleTask`、`role`： S1 合并之后，35.2 的 `leads`、6.3 的命名与 11.7 的观察者都改了增强与上下文：S4 开工时，实施者对着 S1 合并之后的代码重做一遍这张表的核对，记进 S4 的记录（主持者的决定 P2-46；6.4 第 0 步）。

| 增强或代码 | 读的 | S4 之后 | 核对的测试 |
|---|---|---|---|
| `taskToggle`（M5，`task-toggle.ts:14-47`） | `toggleTask` 有没有（今天由 `editable` 决定） | `toggleTask` 总是给；`disabled` 按 `writes()`，跟 `onWritesChange` | `task-toggle.test.ts` 新格；`page-tasks.test.tsx` 照过 |
| 任务勾选的焦点还回（M5/P6，`reading-view.tsx:177-194`） | 清理时记、写之后找 | 4.4.3 第 2 步，规则不变；U7 不重写，勾选框是同一个节点 | `page-tasks.test.tsx:63`、`:103`、`:146`、`:217`、`:289`、`:345` 照过 |
| 勾选的拒绝与锁的说明（M5/P6，`page-layout.tsx:198-215`） | 拒绝的码 | 加 `forbidden` 时重读笔记本（6.2） | `page-tasks.test.tsx:163`、`:229`、`:260` 照过；6.2 新格 |
| `toggleTask` 发出之前的两次检查（`reading-view.tsx:154`、`:156`） | `canEditPageBody` | 不变（不再由 `editable` 决定给不给，这两次检查是唯一的闸） | "a reader's checkboxes stay disabled"（`page-tasks.test.tsx:208`）照过；新格：U7 把可写变成不可写之后点勾选框不发请求 |
| `appLinks`、`unresolvedLinks`（M6） | 不读 `editable`、`role`（`app-links.ts:22`、`unresolved-links.ts:14`） | 不变；未建链接的对话框打开时自己按 `writesNotebook` 判断写者（`W/pages/page/unresolved-link.tsx:133`） | `unresolved-link.test.tsx` 照过 |
| `math`、`diagrams`、`scrollRegions`（M4、M6） | 不读 | 不变；图的包装在 U4、U5 之后是新节点，焦点按滚动区的键找回 | 各自的测试照过；"a drawing that had the focus…" |
| `codeHighlight`（M4） | 不读 | 重跑时跳过语言与文字都没变、已着色的块 | `highlight.test.ts` 新格 |
| 未建链接的 `leads`（S1 的 35.2，按 `notebook.mode`） | 模式 | 模式变了走 U5 的路（撤掉、重跑、不动 HTML） | "the notebook's mode changing…" |
| `assets`（M7） | 不读 `editable`；读 `assetsExpire`（`assets.ts:58`） | `assetsExpire` 读最新的（getter） | `reading-view.test.tsx:820` 加的一步 |
| `ReadingContext.role`（M4 起） | 没有增强读 | 删掉 | tsc |
| 来源页（M10，`canEditPageBody` 含 `!pages.isSource(id)`） | 来源页的勾选框不可勾 | 同上：`writes()` 对来源页为假；页变成来源、或来源恢复成普通页（`mode` 事件）时跟着变，不重写 | 新格：来源与否经 `mode` 事件变了，勾选框跟着变、节点不换 |

### 4.5 S5 编辑器、附件、上传、粘贴与导入导出

S5 照 00 号文档第 7 节分组：附件与上传、编辑器、导入导出与来源。另加"阅读视图里的附件"一组：与 S4 有交叉，4.5.2 是交叉的核对。

- **本节的取舍**：P2-26 至 P2-30、P2-38、P2-40、P2-42（2.3）。AS6 只在 Chromium（P2-30）；15.2 在媒体的 `DOMActivate` 之后的下一个任务里设回 `open`、不留状态（P2-42），是在三种引擎里探过的（修订核对 C 的探针）。
- **16.3 的超时**：在 4.5.1 定下：60 秒没有进度、发完之后 180 秒没有答复。
- **部件**：S5 多了 server，是 15.1 的一个属性（P2-27、2.1 第 4 条）。
- **13.2 第 27 条的新文字**：写在 8.2。

#### 4.5.0 总表、键、e2e 与提交

##### 4.5.0.1 总表

| 编号 | I3 分类 | 组件文件（主要） | 测试 | 大小 |
|---|---|---|---|---|
| 12.8 | A | `W/editor/loaded/link-completion.ts`、新 `gfm-table.ts` | vitest：`link-completion.test.ts`（新 `gfm-table.test.ts` 可选） | 小 |
| 15.1 | A | `W/reading/reading.css`；服务端 `server/internal/platform/markdown/obsidian/{asset,obsidian}.go`（P2-27） | e2e（Chromium）：`e2e/stories/asset/as3-render.spec.ts` 的 AS3 (page)；Go：`obsidian/asset_test.go` | 小到中 |
| 15.2 | A | 新 `W/reading/title-media.ts`，在 `W/reading/enhancement.ts` 登记（P2-42） | vitest：新 `title-media.test.ts`；e2e（Chromium）：AS6 的一格 | 小 |
| 16.3 | A | `W/services/upload-fetch.ts`、`W/pages/page/upload-rows.tsx`、`W/pages/notebook/import-dialog.tsx` | vitest：`upload-fetch.test.ts`、`attachment-uploads.test.tsx`、`transfer-import.test.tsx`、`page-asset-upload.test.tsx` | 小到中 |
| 16.4b | A | `W/pages/page/attachments-section.tsx` | vitest：`attachments-section.test.tsx` | 小 |
| 16.4c | B | 同上（`useNotice`） | vitest：`attachment-uploads.test.tsx` | 小（与 16.10、38.9 一个改法） |
| 16.10 | B | 同上 | 同上 | 小 |
| 16.11 | A | `W/stores/asset.store.ts`、`upload-rows.tsx` | vitest：`attachment-uploads.test.tsx` | 小 |
| 17.2b | A | `W/reading/assets.ts`、`W/pages/page/attachment-row.tsx`、`page-properties.tsx` | e2e（Chromium）：新 AS6；vitest（结构）：`assets.test.ts`、`attachments-section.test.tsx`、`page-properties.test.tsx` | 小 |
| 17.4 | A | 新 `W/reading/expired-downloads.ts`（`assets.ts` 导出 `assetOf`） | vitest：新 `expired-downloads.test.ts`；e2e（Chromium）：AS6 | 小 |
| 17.6 | A | `W/pages/page/assets-expiry.ts`、`attachments-section.tsx`（`useExpiry`） | vitest：`reading-view.test.tsx`、`attachments-section.test.tsx` | 小 |
| 17.7 | A | `W/reading/assets.ts` | vitest：`assets.test.ts` | 中 |
| 18.1 | A | `W/pages/page/page-edit.tsx`、`unsaved-guard.tsx` | vitest：`page-asset-upload.test.tsx` | 小 |
| 18.2 | A | `W/editor/loaded/asset-upload.ts`、`W/editor/registry.ts`、`page-edit.tsx`、`asset.store.ts` | vitest：`asset-upload.test.ts`、`page-asset-upload.test.tsx` | 小 |
| 19.4 | A | `W/stores/transfer.store.ts` | vitest：`transfer.store.test.ts` | 小 |
| 20.1 | B | `W/pages/notebook/import-dialog.tsx` | vitest：`transfer-import.test.tsx` | 小 |
| 20.2 | B | 同上 | 同上 | 小 |
| 20.3 | A | 同上 | 同上 | 小 |
| 28.1 | B | `attachments-section.tsx`（`useLocated`） | vitest：`attachments-section.test.tsx` | 小 |
| 38.7 | A | `W/pages/notebook/source-add-dialog.tsx` | vitest：`source-add-dialog.test.tsx` | 小 |
| 38.9 | B | `attachments-section.tsx`（`useNotice`） | vitest：`source-attachments.test.tsx` | 小（同 16.4c） |

- 17.2b、17.4 的 e2e 只在 Chromium（P2-30 改）：审查 C 的探针在三种引擎（无头）里核对过，两样的行为相同（`m12/p2-review-c/probe1.txt`）；结果记进 S5 的记录。
- 只能人工核对的只有一样：17.4 在真浏览器里会不会弹下载许可，进 P5/S3 的人工清单（见 17.4）。
- 16.3 不加 e2e：三种引擎"请求体发完"的信号派发的先后，探针已量过（`probe2.txt`，见 4.5.1.3）；两种信号 vitest 都钉住。

##### 4.5.0.2 本 Step 新加的 i18n 键（两种语言都加，13.2 第 4 条；不删键）

| 键 | en | zh-CN | 项 |
|---|---|---|---|
| `asset.stalled` | `The upload stopped: nothing went out for a minute. Upload the file again.` | `上传停住了：一分钟没有传出任何数据。请再上传一次。` | 16.3 |
| `asset.unanswered` | `The server did not answer within three minutes of getting the file, so the upload was stopped. If it is not in the list, upload it again.` | `服务器收到文件之后三分钟没有答复，上传已停止。列表里没有它的话，再上传一次。` | 16.3 |
| `transfer.importStalled` | `The upload stopped: nothing went out for a minute. Try again, on a faster connection if you can.` | `上传停住了：一分钟没有传出任何数据。请重试，最好换一个更快的网络。` | 16.3 |
| `transfer.importUnanswered` | `The server did not answer within three minutes of getting the zip, so the upload was stopped. If the import is not among the recent jobs, import again.` | `服务器收到 zip 之后三分钟没有答复，上传已停止。最近的任务里没有这次导入的话，再导入一次。` | 16.3 |
| `asset.cancelledLate` | `Cancelled too late: the server already had all of {name}, and kept it. Delete it from the attachments if you do not want it.` | `取消晚了：服务器已经收到了整个「{name}」并留下了它。不要的话，在附件里删除它。` | 16.11 |
| `editor.leaveUploadsTitle` | `Leave before the files are inserted?` | `在文件插入之前离开？` | 18.1 |
| `editor.leaveUploadsDescription` | `Files you pasted or dropped are still uploading. They will be added to this page's attachments, but not inserted into it.` | `粘贴或拖入的文件还在上传。它们会照样加进这一页的附件，但不会插入页面。` | 18.1 |
| `source.discardAsk` | `Close, and lose the title and Markdown entered here?` | `关闭并丢掉这里填写的标题与 Markdown？` | 38.7 |
| `source.keepEditing` | `Keep editing` | `继续编辑` | 38.7 |
| `source.discardClose` | `Discard and close` | `丢掉并关闭` | 38.7 |

- "最近的任务"照 `transfer.jobsTitle` 的 zh（`zh-CN.ts:871`），"recent jobs"照 en 的"Recent jobs"（`en.ts:907`）。
- 放的位置：`asset.*` 接在 `asset.failed`（`en.ts:559`、`zh-CN.ts:540`）之后；`transfer.import*` 接在 `transfer.importCut` 之后；`editor.leave*` 接在 `editor.leave` 之后；`source.*` 接在 `source.creating` 之后。
- 与 S2 的键不重（S2 改的是 15.3、16.6、19.1、38.6、38.8 等的已有文字）。两份翻译文件文本冲突，后合并的先 `git merge main`，只是相邻插行。

##### 4.5.0.3 e2e 故事

- **新 AS6**（`e2e/stories/asset/as6-reading-links.spec.ts`），只在 Chromium，不进 `threeBrowsers`（P2-30 改）：
  - 17.2b：复制正文不带"在新标签页打开"的提示，名字照旧；
  - 17.4：过期的下载链接点了再签、下载到；
  - 15.2：折叠的 callout 标题里的音频，点控件的空白与时间处不开合，点标题的文字照常开合（P2-42）。
  - jsdom 做不到的：复制的序列化与 `user-select`，程序里 `click()` 一个 `download` 链接、真的下载。
  - 自成一个故事：两格各要自己的页与附件，不动 AS3 由 API、MCP 两格共用的 `content`（`as3-render.spec.ts:37-39`）。
- **AS3 (page)**（`as3-render.spec.ts:297`，只 Chromium，照现在）：15.1 另建一页 `Sizes`，量盒子，再在 375 宽量一次。jsdom 没有布局。
- 推断的用时：AS6 每格建团队、上传两三个附件、建页、打开、做一件事，约 5–8 秒，三格约 15–24 秒的 worker 时间；AS3 多约 1–2 秒。CI 推断 2 个 worker（审查 C 的"核对过"第 11 条），E2E 一步的墙钟多约 5–10 秒。

##### 4.5.0.4 提交的分组（每个提交单独为绿：tsc、knip、vitest、oxlint）

见 6.5。

##### 4.5.0.5 拿不准的、要实施者先核对与审查重点看的

1. **15.1**：Obsidian 对 `|300x200` 是拉伸还是保持比例，没有核实；推荐的 `object-fit: contain` 不变形（15.1）。
2. **15.2**：设回在 `setTimeout(0)` 里，Chromium 只派发一次 `toggle`；中间会不会画出一帧展开的样子没有量（15.2）。
3. **18.1**：退出登录时 `UnsavedGuard` 不问，靠旧一代整个卸下、blocker 随之注销（推断），第三格核对（18.1）。
4. **12.8**：lezer 的 `Paragraph` 与 goldmark 的段落行一致是推断；实施时对着 goldmark 做一次差分探针，同一组样例比服务端渲染出 `<table>` 与编辑器判为表格（12.8）。
5. **20.2**：留被拒的那句（`importBusy`），压下常驻的 `importUnderWay`，已定：2.3 的 P2-52（20.2）。

#### 4.5.1 16.3 上传的超时取多长（本 Phase 要定的）

##### 4.5.1.1 事实

- **网页**：`W/services/upload-fetch.ts:40-86` 的 `uploadFetch` 在 XHR 上发表单；`:78` 听 `timeout`，但没有设 `xhr.timeout`（默认不限），永远不触发；此外没有计时。附件（`W/services/asset.service.ts:56-71`）与导入（`W/services/transfer.service.ts:50-68`）的上传都走它；`.md` 来源走 JSON 的 `createWithContent`（`W/stores/asset.store.ts:405`），不在此列。
- **挂住时**：`W/pages/page/upload-rows.tsx:89` 字节发完就是不可按的"正在完成…"（`:121`）；离开页面的提醒（`W/stores/asset.store.ts:178`；导入是 `W/stores/transfer.store.ts:286-297` 的 `warn`）与编辑器"完成"的等待（`W/pages/page/page-edit.tsx:217-232` 的 `uploadsIn`）一直开着。
- **大小**：附件至多 50 MiB、导入 512 MiB，最低速率 64 KiB/s（`server/configs/config.yaml:131-137`、`:149-151`）：按它 50 MiB 约 13 分钟，512 MiB 约 2 小时 17 分钟。
- **服务端的期限**：
  - `ReadHeaderTimeout` 5 秒、`ReadTimeout` 30 秒、`WriteTimeout` 60 秒（`server/internal/platform/httpserver/server.go:40-48`）。上传与导入的路由是 `api.Stream`（`asset/adapter/http/handler.go:67-68`、`transfer/adapter/http/handler.go:71`），按连接改读写截止（`stream.go:235-252`）。
  - 读截止是累计的：`start + read_timeout + 已读字节/最低速率`（`stream.go:245`），写截止在它之后 30 秒（`:251`）。前面传得快的上传，之后可以停很久，服务端照收。
  - 处理器之前的一段在 `request_timeout` 15 秒之内（`longlived.go:83-89` 的 `opening`）；处理器的 context 随原请求的 context 结束（`longlived.go:93-104` 的 `opened`）。`Check`、`Create` 经 `bounded`，各至多 15 秒（`asset/adapter/http/upload.go:63`、`:79-83`）；读请求体与写文件只受读截止；之后的 fsync（`server/internal/platform/storage/local.go:469-475`）与读图片的宽高（`asset/app/blobs.go:78-85`）没有期限。导入同形（`transfer/adapter/http/start_import.go:60-88`、`:77-81`）。
  - 中止不是中性的：浏览器 `abort()` 关掉连接，net/http 取消请求的 context，`Create` 的事务回滚（审查 C 的 M2）。
  - 一个健康的服务端收完最后一个字节之后，约 fsync 加至多 15 秒就答复，成功或 5xx。
- **README 的部署**（`README.md:619-621`）：nginx 的 `client_body_timeout`、`proxy_send_timeout`、`proxy_read_timeout` 是两次读写之间的间隔，默认 60 秒（样例 `README.md:567`）；Caddy 的样例没有超时。
- **浏览器的信号**（审查 C 的探针 `m12/p2-review-c/probe2.txt`，回环，无头）：
  - 服务端读完之后扣住答复 20 秒：Chromium、WebKit 的 100% 进度与 `upload` 的 `load` 都在读完之后约 0.1 秒内；Firefox 的 100% 进度也是，`load` 却推到 20 秒、与答复头同时。
  - 服务端以 256 KiB/s 读 8 MiB：Chromium 的 `upload` `load` 比服务端读完早 15.4 秒，WebKit 早 14.8 秒，其间约 3.8 MiB 在缓冲里；两次进度之间最长约 6 秒（Firefox 3.9 秒）。本机 `tcp_wmem` 上限 4 MiB。
- **在哪里碰到**（推断）：直接暴露或经 Caddy 时服务端挂住；连接无声断了（换网、NAT 过期，TCP 重传约 15 分钟才放弃）。经 README 的 nginx，上游 60 秒不答给 504。

##### 4.5.1.2 两种做法

- **不用整体的超时**（`xhr.timeout`，从发出起 T 秒没有答完就放弃）：要容得下最慢的合法传输（50 MiB 约 13 分钟、导入约 2 小时 17 分钟），挂住时就要等这么久；取小了误杀慢网的大文件。
- **用空闲 N ＋ 发完之后 M**：请求体在发时 N 秒没有进度就放弃，发完之后 M 秒没有答复就放弃。两者都与文件大小无关，挂住时 N 或 M 秒之内放弃。

##### 4.5.1.3 推荐：N = 60 秒，M = 180 秒

已定：2.3 的 P2-26，M 由 60 秒改为 180 秒（主持者与设计者的决定，照审查 C 的 M2）。

- **N = 60 秒**：请求体在发时，最后一次上传进度之后 60 秒没有新的进度就放弃。
  - 字节在流动时进度不会停一分钟：探针里两次进度之间最长约 6 秒（256 KiB/s，见 4.5.1.1）；按最低速率 64 KiB/s 推断约四倍，仍在 60 秒之内。
  - 取舍：服务端的读截止是累计的（`stream.go:245`），直接暴露或经 Caddy 时前面传得快的上传可以停很久，服务端照收（例如 50 MiB 以 10 MiB/s 传了一半，余量约 6 分钟）。N = 60 会中止这类服务端本来收得下的上传（漫游、VPN 重连），今天它们会接着传完。换来的是连接无声断了、服务端挂住时一分钟之内告诉用户；经 README 的 nginx 时，停过 60 秒本来就被代理切断。
- **M = 180 秒**：请求体发完之后 180 秒没有答复就放弃。
  - 发完之后要等的：发送缓冲的排空，加服务端的 `Create`（至多 15 秒），加 fsync 与读宽高。
  - 缓冲：回环上约 3.8 MiB、15 秒（4.5.1.1）。回环夸大了接收端的缓冲，真实网络上只剩发送端的；`tcp_wmem` 上限 4 MiB，按最低速率 64 KiB/s 排约 64 秒（推断，没有在真实链路上量）。
  - 误判的代价不对称：中止取消请求的 context，`Create` 的事务回滚（4.5.1.1）。M 误判一次就毁掉一个健康的上传，慢链路上的大文件每次都会；M 取长，只是在服务端真的挂住时多等一会儿。
- 实现是一个计时：每次进度重来，发完之后换成 M。阶段分开记，界面说的话不同：停住了，服务端多半什么也没收到；没答复，多半正是这次中止让它没有建成，只有提交恰好早于中止时才建成了。
- "发完"认两个信号：`xhr.upload` 的 `load`，与 `loaded >= total` 的进度。Firefox 的 `load` 推到答复到了才派发（4.5.1.1），只认 `load` 的话，Firefox 上发完之后仍按 N 计，60 秒就误报"停住了"。
- 附件、编辑器里的粘贴与拖入、来源区的非 `.md` 文件、导入都经 `uploadFetch`，一起得到这两个计时。服务端不改。

##### 4.5.1.4 超时之后

- `uploadFetch` 自己 `xhr.abort()`，以 `UploadTimeout`（`stage: "stalled" | "unanswered"`）拒绝，**不是** `AbortError`：`asset.store.ts:456-458` 把 `AbortError` 当作用户取消、移走这一行。
- **附件的行**（`asset.store.ts:455-476` 的 `catch` 照旧，不改）：先重读树与列表（`:460`），再记 `failure`；`going` 不再含它（`:187-189`），没有别的在途时离开页面的提醒关掉。进度条换成 `role="alert"` 的一句，同一个按钮从"正在完成…"变成"移除"（`upload-rows.tsx:112-131`，元素不换，焦点留着）。
- **编辑器**：`assets.uploaded()`（`asset.store.ts:317-333`）以这个失败拒绝，嵌入不插入，"完成"的等待结束；编辑器旁的行（`W/pages/page/editor-uploads.tsx`）说同样的话。
- **导入**：`transfer.store.ts:291-296` 的 `finally` 关提醒；对话框（`import-dialog.tsx:264-293`）照旧重读任务列表，横幅说 `transfer.importStalled` 或 `transfer.importUnanswered`；文件还在，可以再按"导入"。
- **不加"重试"**：附件重新选文件。`unanswered` 的那句叫人先看列表：提交早于中止时它已建成，再传会成为 `名字 2.扩展名`。
- 不取：`unanswered` 之后看树里有没有新的同名附件、换一句话（16.11 的办法）；v0.1 不值得。

##### 4.5.1.5 13.2 要随之写进的一句

13.2 第 27 条末尾加："附件与导入的上传不设整体的超时：请求体在发时 60 秒没有进度、发完之后 180 秒没有答复就放弃，行上（或导入的对话框里）分两种说明，离开页面的提醒随之关掉（M12/P2/S5，移交第 16 条）。"

#### 4.5.2 与 S4 的重叠

S4 重写 `W/pages/page/reading-view.tsx:133-232` 的 layout effect，做 17.1（HTML 只在附件地址上不同时就地换地址，不重跑增强）与 17.8（全屏）。S5 在阅读视图一侧的六项：

- **不交叉，不依赖 S4**：
  - 15.1：CSS 与 HTML 的一个属性，就地换 `src` 不碰宽高；
  - 17.2b：就地换 `href` 时链接节点不换，提示留着；
  - 17.6：不经阅读视图，只决定何时 `mutate()`。
- **交叉，在 S4 之后**（硬依赖）：
  - 17.4：点击时读 `context.assetsExpire`。已定：2.3 的 P2-22，S4 把它改成读最新值的 getter；增强不重跑、闭包与链接留着，所以 `passing` 只在一次派发期间为真（17.4）。
  - 17.7：S4 的 17.1 改开始了的媒体怎样跨替换留下（`kept`、`adopt`），17.7 建在它上面。
- 15.2：新的增强自己一个文件，只在 `readingEnhancements` 末尾登记，不留状态，不交叉 S4 的 effect。
- 不取：17.4、17.7 交给 S4 一起做。它们留作 S5 的最后两个提交，S4 只需守 P2-22。

#### 4.5.3 逐项

##### 附件与上传

##### 16.3 服务端挂住时一直"正在完成…"，没有超时

- 现状：见 4.5.1.1。用户：传完一个视频之后服务端的磁盘卡住（或换了 Wi-Fi），行一直"正在完成…"，关标签页时一直问，编辑器的"完成"一直等。与 I3 的不同：无声断开的连接也一样。
- 改法（已定：2.3 的 P2-26，M 为 180 秒；理由见 4.5.1）：
  - `W/services/upload-fetch.ts`：
    - 常量 `stallLimit = 60_000`、`answerLimit = 180_000`，文档注释各写一句理由（N：停住一分钟就放弃；M：发送缓冲的排空、服务端的 15 秒与 fsync，中止会让服务端回滚）。
    - `export class UploadTimeout extends Error { constructor(readonly stage: "stalled" | "unanswered") }`，`name` 是 `"UploadTimeout"`。
    - 一个计时 `wait(stage)`：清掉旧的，按 `stage` 取 N 或 M 重设；到点时记下 `timedOut = new UploadTimeout(stage)`，再 `xhr.abort()`。
    - 谁重设：`xhr.send(form)`（`:84`）之后 `wait("stalled")`；上传进度的监听**总是**挂上（今天只在 `progress !== undefined` 时，`:60-62`），每次 `wait(event.lengthComputable && event.loaded >= event.total ? "unanswered" : "stalled")`；`xhr.upload` 的 `load` 时 `wait("unanswered")`。两个监听在 `send` 之前挂上。
    - `settle`（`:53`）先 `clearTimeout`；`abort` 的监听（`:79-82`）改成 `reject(timedOut ?? aborted())`。
    - 删掉 `:78` 的 `timeout` 监听，不设 `xhr.timeout`。文档注释（`:29-39`）补两个计时。
  - `W/pages/page/upload-rows.tsx:262-282` 的 `failureText`：`isRefusal` 之后，`UploadTimeout` 按 `stage` 给 `asset.stalled` 或 `asset.unanswered`。
  - `W/pages/notebook/import-dialog.tsx:224` 的 `explain`：`TypeError` 照旧 `transfer.importCut`；`UploadTimeout` 按 `stage` 给 `transfer.importStalled` 或 `transfer.importUnanswered`。
  - 四个键见 4.5.0.2。
- 测试（vitest）：
  - `W/services/upload-fetch.test.ts` 新加（`vi.useFakeTimers()`，`afterEach` 还原；结果接到 `let settled: unknown`，不直接 `await`，免得没修的代码挂住）：
    - `"a body that goes nowhere for a minute is given up: the transfer is aborted, and it rejects with an UploadTimeout, stalled"`：`sending(form)`；走 59,999 毫秒，`sent().aborted === false`、`settled === undefined`；再走 1 毫秒，`aborted === true`，`settled` 是 `stage === "stalled"` 的 `UploadTimeout`。没修的代码上：`aborted` 是 `false`，`settled` 是 `undefined`。
    - `"progress puts the minute off: a body that goes on slowly is not given up"`：第 50 秒 `progress(10, 100)`，再走 50 秒没有中止，再走 10 秒才中止、`stalled`。
    - `"a body all sent waits three minutes for its answer: none comes, it is given up, unanswered"`：`progress(100, 100)` 之后走 60 秒没有中止，走到 180 秒中止、`stage === "unanswered"`；另一份只派发 `upload` 的 `load`（不报进度），同样。
    - `"an answer in time is the answer: nothing is aborted after"`：`progress(100, 100)`，第 179 秒 `answer(201)`，解析为 201；再走两分钟，`aborted === false`。
    - `"the signal stops it as a cancel still: an AbortError, not an UploadTimeout"`：计时进行中 `stop.abort()`，拒绝的 `name === "AbortError"`。
  - `W/pages/page/attachment-uploads.test.tsx` 新加：
    - `test.each` 两种语言 `"an upload the server does not answer within three minutes of its last byte fails, saying the upload was stopped; the button that had the focus dismisses it, and the page no longer asks before it is left (%s)"`：`vi.useFakeTimers({ shouldAdvanceTime: true })`；`server.uploadsHeld = true`；选 `a.png`；`progress(4, 4)`；"Finishing the upload of a.png"按钮 `focus()`；走 180 秒；`role="alert"` 的整句（en / zh）；"Dismiss the upload of a.png"（zh：`移除a.png的上传`）是同一个元素且有焦点（同 `:240` 那一格的"取消→移除"）；`transfers(server.app)[0]?.aborted === true`；`beforeunload` 的 `defaultPrevented === false`（照 `transfer-import.test.tsx:30-34`）。没修的代码上：仍是"Finishing…"，`findByRole("alert")` 超时。
    - `"an upload that makes no progress for a minute fails, saying it stopped"`：不报进度，走 60 秒，`asset.stalled` 整句（en）。
  - `W/pages/notebook/transfer-import.test.tsx` 新加 `test.each` 两种语言 `"an import's upload the server does not answer within three minutes of the zip says the upload was stopped, the file kept to try again (%s)"`：导入的答复挂住（`imports: () => new Promise(() => undefined)`），`progress(n, n)`，走 180 秒；横幅整句；"导入"不再 `disabled`；`asksBeforeLeaving() === false`；任务列表多读了一次。
  - `W/pages/page/page-asset-upload.test.tsx` 新加 `"a file pasted whose upload has no answer within three minutes inserts nothing; Done waiting for it leaves"`：粘贴、按"完成"、进度报满、走 180 秒，编辑结束（"Edit"出现），`PUT` 里没有嵌入。
- 已有测试要随之改的：
  - `page-asset-upload.test.tsx:272-289`（"an edit whose upload goes is not idle…"）扣住上传走 31 分钟，新代码 60 秒就判停住：`rest` 拆成每 30 秒一步，每步 `progress(k, total)`（k 递增、小于 total）。
  - `upload-fetch.test.ts:98-106`（"a network failure, or a timeout, rejects with a TypeError"）：删掉 `late` 那一半，名字改成 `"a network failure rejects with a TypeError, as fetch's"`。
  - 别的扣住上传的格用真的计时、几秒之内结束；e2e 里扣住上传的（`tr2-import.spec.ts:243`、`as1-upload.spec.ts:241`）都在 60 秒之内。
- 变异：不起停住的计时（第一格）；进度不重新计时（第二格）；发完不换成 `unanswered`（第三格：60 秒时以 `stalled` 中止）；只认 `load`、不认 100% 的进度（第三格的 `progress(100, 100)` 那一份，Firefox 靠它，4.5.1.3）；只认 100% 的进度、不认 `load`（第三格只派发 `load` 的那一份）；以 `AbortError` 拒绝（附件那格的 `alert` 找不到：store 当作取消、移走行）；答复之后不清计时（第四格，`aborted` 变 `true`）；`failureText` 不认 `UploadTimeout`（整句落到 `problem.other`）。
- 假服务端：`W/test/transfer.ts` 的 `FakeTransfer` 已有 `progress()`，`upload` 是 `EventTarget`；可加 `sentAll()`（派发 `upload` 的 `load`）给"只派发 `load`"的那一份。`transferTo` 不改：它在答复之前报满进度，正好让计时换成 M，再答复清掉。
- 牵连：S2 的 38.6 也改 `upload-rows.tsx` 的 `failureText` 附近；P3/S3 在 `transfer-import.test.tsx` 加一格（导入答 `transfer.too_many_jobs`），并改 `transfer.importCut` 的文字（P3 设计 4.2），两个新键接在它之后，冲突是一定的。文本冲突，后合并的先 `git merge main`。

##### 16.4b 附件在别处改名之后，复制的字段仍是旧的嵌入

- 现状：`W/pages/page/attachments-section.tsx:91` 的 `copied` 存 `{ embed, name, done }`，点"复制嵌入"时定下（`:141-152`）；字段 `CopyField` 以 `key={copied.embed}` 挂（`:249`）。本节菜单里的改名先 `setCopied(undefined)`（`:138`），所以只有别处改名才碰到。用户：不经 HTTPS 时（剪贴板不可用，字段才出现，`:146-150`）复制 `data.zip` 的嵌入，同事把它改名为 `archive.zip`，字段仍给 `![[data.zip]]`。
- 改法（`attachments-section.tsx`）：
  - `copied` 改存 `{ id: string; done: boolean } | undefined`（`:91`）；`:149`、`:152` 各存 `{ id: asset.id, done }`。
  - 由当前列表推出：`copiedAsset = copied !== undefined && !copied.done ? list?.assets.find((asset) => asset.id === copied.id) : undefined`，`copiedLink = copiedAsset?.link ?? null`。
  - `:245-250`：`copied.done` 时照旧那句；否则只在 `copiedAsset !== undefined && copiedLink !== null` 时渲染 `<CopyField key={copied.id} embed={embedOf(copiedLink)} name={copiedAsset.name} field={copyField} left={() => heading.current?.focus()} />`（附件离开了列表或改成没有扩展名，字段随之消失）。
  - `CopyField`（`:318-334`）加 `left` 参数与 `useFocusLeaving(field, left)`（`W/lib/focus-leaving.ts:10`）：字段带着焦点离开时焦点交给这一节的标题（13.2 第 17 条）。`key` 用 id，嵌入换了不重挂，挂载时的取焦点（`:321-323`）不再来一次。
- 测试（vitest，`W/pages/page/attachments-section.test.tsx`）：
  - `"an embed shown to copy by hand follows its attachment renamed elsewhere: the field holds its new embed and name, the focus where it was"`：照 `:193-232` 那格去掉剪贴板；复制 `data.zip`，字段 `Embed of data.zip` 值 `![[data.zip]]`；焦点放到 `Upload` 按钮；`server.nodes` 里改名为 `archive.zip`，`readAgain()`（`W/test/page-panel.ts:34-37`）；字段（同一个元素）的名字是 `Embed of archive.zip`、值 `![[archive.zip]]`，焦点仍在 `Upload`。没修的代码上：值仍是 `![[data.zip]]`。
  - `"an embed shown to copy by hand goes as its attachment goes elsewhere, the focus in it going to the section's title"`：字段有焦点时别处删掉它、`readAgain()`；字段不在了，`document.activeElement` 是 `Attachments` 标题。没修的代码上：字段还在。
- 变异：`key` 改回 `embed`（重挂、取走焦点：第一格的焦点断言）；嵌入仍取点击时的（值断言）；去掉 `useFocusLeaving`（第二格焦点在 `body`）。
- 牵连：S1 的 16.1 也改 `attachments-section.tsx`、`attachment-row.tsx`，文本冲突，后合并的先 `git merge main`。16.4a（关闭字段）是 C，不做。

##### 16.4c 播报区域里留着已不成立的上一句

- 现状：`W/pages/page/attachments-section.tsx:238-240` 是一直挂着的 `aria-live="polite"` 段；`useNotice`（`:345-393`）只在 `assets.uploads` 的成员变化时（`:356-358`）合成"已上传 / 正在上传"，从不清空。上传被取消或失败之后，区域仍写着"Uploading files: 1."（`attachment-uploads.test.tsx:181` 正断言它），读屏的虚拟光标读到一句不成立的话。失败不改数组，连这一轮也不触发。
- 改法（已定：2.3 的 P2-40；`useNotice` 的 effect，与 16.10、38.9 一起）：
  - 两个 MobX reaction 把这一轮记下，`setTimeout(flush, 0)` 合成（`turn ??= setTimeout(flush, 0)`，一轮一次）：
    1. 数据函数 `assets.uploads.filter((upload) => upload.parent === parent)`：前一份里有、这一份没有、带 `result` 的进 `left`（名字）；这一份新出现、没有 `failure` 的进 `begun`（`Set<Upload>`，16.10）。
    2. 数据函数 `assets.going.some((upload) => upload.parent === parent)` 变成 `false` 时 `ended = true`。`assets.going`（`asset.store.ts:187-189`）读每个上传的 `failure`，所以失败也触发它。
  - `flush`：
    - `going` 数 `begun` 里仍在 `assets.uploads`、没有 `failure`、`!upload.signal.aborted` 的（`signal` 不可观察，合成时读当时的值）；
    - 有"已上传"或"正在上传"的话（两句都有时用 `page.sentences` 连起来）就 `setSaid(saying(text))`；
    - 没话说而 `ended` 时 `setSaid({ text: "", again: false })`；
    - 然后清空 `left`、`begun`、`ended`、`turn`。
  - 不变量：最后一个上传成功离开的那一轮照说"已上传"，不清；清空用 `{ text: "", again: false }`，下一次同一句从空变过来，读屏照读（`saying`，`:391-393`）；清理停掉两个 reaction、`clearTimeout(turn)`。文件头的注释（`:336-344`）补一句。
- 测试（vitest，`W/pages/page/attachment-uploads.test.tsx`）：
  - `:181` 的 `toBe("Uploading files: 1.")` 改成 `toBe("")`，即是本项的回归格。`:44-74`、`:106-160` 的播报断言不变（上传都以成功离开）。
  - 新加 `"an upload that fails leaves nothing said of uploads going: the region says nothing"`：照 `:240-265` 扣住、答 507，等 `alert` 出现，再等一轮（20 毫秒），`notice(section) === ""`。没修的代码上：`"Uploading files: 1."`。
- 变异：去掉第二个 reaction（失败那格仍是旧句）；`ended` 时不清（同上）；成功离开的那一轮也清（`:44-74` 那格的"Uploaded: …"断言抓）。
- 不取：说过的一律几秒后清空（"已复制嵌入"也清）：多一个计时，与 P2-40 的读法不合。

##### 16.10 一轮里开始又离开的上传仍被数进"正在上传"

- 现状：`attachments-section.tsx:364` 在 reaction 里就把这一轮新出现、没有失败的上传数进 `begun`；同一轮里它又被取消或失败（例如换代时 `generation` 中止，`asset.store.ts:179-183`），合成时（`:371-379`）仍数着它，读屏听到"正在上传 1 个文件"而什么也没在传。
- 改法：见 16.4c：`begun` 改成 `Set<Upload>`，合成时只数仍在 `assets.uploads` 里、没有失败、没有被中止（`upload.signal.aborted`，`asset.store.ts:91-93`）的。
- 测试（vitest，`attachment-uploads.test.tsx`）新加 `"an upload cancelled in the turn it began is not said to go"`：照 `:122-144` 在 `renderApp` 之前 `vi.spyOn(AssetStore.prototype, "upload")`（MobX 在构造时把原型的方法包成实例上的 action），包装里调原来的 `upload` 之后对每个返回的 `Upload` 调 `cancel()`；选 `a.png`；等一轮；`notice(section) === ""`。没修的代码上：`"Uploading files: 1."`。
- 变异：合成时不看 `signal.aborted`（这一格）；不看 `assets.uploads.includes`（取消之后已移走的那种：同一格在 dismiss 先于合成时抓，推断）。

##### 38.9 上传到 `raw` 被拒之后，附件下留着 "Uploading files: 1."

- 现状：来源区（LLM Wiki 的 `raw`）的附件一节，上传经 `uploadSources`（`attachments-section.tsx:100-107` → `asset.store.ts:272-292`）；`.md` 文件被拒（409 重名、403、无效的 UTF-8、超过 5 MiB）只设 `failure`（`asset.store.ts:416-424`），播报区一直"Uploading files: 1."。原因同 16.4c（I3 标推断，读代码确认是同一条路）。
- 改法：同 16.4c（第二个 reaction 看 `going`）。
- 测试（vitest，`W/pages/page/source-attachments.test.tsx`）新加 `"a source refused as it is uploaded leaves nothing said of uploads going"`：`pageServer` 用 LLM Wiki、来源根，`answers` 让建页答 409 `page.title_taken`；在来源页的附件一节选一个 `notes.md`；等行上的拒绝出现，再等一轮；`[aria-live=polite]` 的文字是空的。没修的代码上：`"Uploading files: 1."`。`source-uploads.test.tsx` 各格不断言播报区，不受影响。
- 变异：同 16.4c。

##### 16.11 临近结尾的取消晚于服务端，不告知

- 现状：`W/stores/asset.store.ts:455-464`：取消之后先移走行（`:456-458`），重读树与列表（`:460`），再移走一次、返回。服务端其实已收完时，附件照样建成、列表里多了它，行却随取消消失了，什么也不说。字节发完的按钮已不可按（P4A B7），窗口只剩"浏览器的进度还没到 100%、服务端已收完"那一段。
- 改法：
  - `asset.store.ts` 导出 `class CancelledLate extends Error { constructor(readonly file: string) }`，`name` 是 `"CancelledLate"`（字段不叫 `name`：那是 `Error.name`）。
  - `send`（`:434-485`）开头记 `before`：`upload.parent` 下现有兄弟节点的 id 集合（`this.pages.siblingsOf`）。
  - `catch` 里重读之后（`:461-464`），`upload.signal.aborted` 时：先 `dismiss(upload)`；在兄弟节点里找不在 `before`、`titleKey(node.name) === titleKey(name)` 的；找到就 `runInAction` 设 `upload.failure = new CancelledLate(made.name)` 并放回 `this.uploads`；然后返回。树的重读失败时 `siblingsOf` 是旧的，找不到，照今天移走（不知道就不说）。
  - `upload-rows.tsx` 的 `failureText`：`CancelledLate` 给 `asset.cancelledLate`（`name` 是 `failure.file`）。行带着"移除"，不移焦点（焦点已随第一次移走交给"上传"按钮，13.2 第 26 条）。
  - 编辑器的上传：`uploaded()` 已在第一次移走时以 `AbortError` 拒绝，不插入嵌入；重新出现的行说明它已在附件里。
- 测试（vitest，`attachment-uploads.test.tsx`）：
  - `test.each` 两种语言 `"a cancel that comes once the server has the whole file says so: the attachment is listed, its row says the cancel was late (%s)"`：`answers` 让上传的路由把 `a.png` 加进 `server.nodes`、答复永远不来（服务端已建、答复丢了）；选 `a.png`；按"Cancel the upload of a.png"；`aborted` 为真；`alert` 整句（en / zh）；`rows(section)` 含 `a.png (opens in a new tab)`；`document.activeElement` 是"Upload"。没修的代码上：行消失、找不到 `alert`。
  - 控制：已有的 `:164-182`（服务端 `abortedFor` 时什么也不建）。
- 变异：`push` 漏掉（行不出现）；`failureText` 不认它（整句）；不记 `before`、只比名字：在 vitest 造得出的情形里等价，不进 vitest。名字由 `freeName` 避开发送前已有的（`asset.store.ts:435`），只有上传途中别人在同一处建了同名的附件才不同；实施时用探针核对，在 Step 的记录里写明等价及其证据。

##### 28.1 "要打开的附件已不在这里"不随它在别处恢复而撤下

- 现状：`W/pages/page/attachments-section.tsx:426-460` 的 `useLocated` 一旦判定"不在"（`setGone`，`:448`）就一直为真（`:439-442`），说明（`:232-236`）与标题的 `aria-describedby`（`:205`）一直留着；它在别处恢复、行回来了，说明还在。
- 改法：`useLocated` 的返回改成 `gone !== undefined && !(list?.assets.some((asset) => asset.id === located) ?? false)`（`:460`）：一次性的只是"焦点给谁"，说明随当前列表显示与撤下；列表又没有它时（又被删），说明再出现。
- 测试（vitest，`attachments-section.test.tsx`）新加 `"an attachment said gone that comes back, restored elsewhere, is said gone no more: its row shows, the title described by nothing"`：照 `:666-707` 造出"不在"的说明；`server.nodes.push(gone)`，`readAgain()`；`gone.png (opens in a new tab)` 的链接出现，说明不在，标题没有 `aria-describedby`。没修的代码上：说明还在。
- 变异：返回值不看列表（这一格）；看列表但不看 `gone`（`:666` 那格之前列表还没读完时说明就出现：推断会抓）。
- 牵连：28.3（这条说明是 `<output>` 又描述标题，读屏可能念两次）只从 ARIA 推断，移到发布后的读屏验收（00 号 4.5），不在本 Step。

##### 阅读视图里的附件

##### 15.1 写下的高度不起作用

- 现状：`W/reading/reading.css:385-391` 是 `.nw-reading :is(img, video).nw-asset { max-width: 100%; height: auto; }`。服务端写 `width`、`height`（`server/internal/platform/markdown/obsidian/asset.go:157-178`）：作者写了尺寸只写写下的（`|0x200` 只写高，`size.go:32-55`）；没写时写图片自己的宽高（防排版跳动）。`height: auto` 盖掉了属性：`![[chart.png|0x200]]` 显示成原图的高，`|300x200` 显示成 300 宽、按比例的高。只改 CSS 分不开"宽高都写"是作者写的还是图片自己的。
- 改法（已定：2.3 的 P2-27，服务端 ＋ CSS）：
  - 服务端，三个文件：
    - `server/internal/platform/markdown/obsidian/asset.go`：作者写了尺寸（`sz != size{}`，在 `imageSize`（`:160-165`）补图片自己的宽高之前判）时，`element`（`:137-145`）在 `src` 之后多写一个空值的 `data-nw-sized=""`。图片的调用在 `:116`，视频在 `:122-127`（视频只写写下的尺寸，同样带）。
    - `obsidian.go` 的允许名单 `Markup.Elements`：`img`（`:105`）与 `video`（`:107`）加 `data-nw-sized`。
    - `asset_test.go` 的期望（见测试）。
  - 与 P3/S1 不冲突：P3/S1 的 10.2 改 `platform/markdown/parse.go`（包 `markdown`）；`asset.go` 在子包 `platform/markdown/obsidian`，不是同一个包。
  - CSS（`reading.css:385-391` 换成三条）：
    ```css
    .nw-reading :is(img, video).nw-asset { max-width: 100%; }
    .nw-reading :is(img, video).nw-asset:not([data-nw-sized][height]) { height: auto; }
    .nw-reading :is(img, video).nw-asset[data-nw-sized][height] { object-fit: contain; }
    ```
    `|0x200` 高 200、宽由比例；窄屏被 `max-width` 截时盒子 100% × 200，图片在里面保持比例。`|300x200` 盒子 300 × 200，图片保持比例、不拉伸。`|300` 与没写尺寸的照旧。
  - 不取（只改网页）：`:not([height]:not([width]))` 才 `height: auto`；修不了宽高都写的。
- 测试：
  - Go 单元（`server/internal/platform/markdown/obsidian/asset_test.go:130-162`）：`"a width and a height"`（`:132`）、`"captions of '|' and a size"`（`:146`）、`"sizes out of range"`（`:149-151` 的 `|0x200`）、`"the largest and the least size"`（`:153`）、`"its caption and size"`（`:157`）、`"video"`（`:162`）的期望加 `data-nw-sized=""`；`"of an image of known size"`（`:130`，图片自己的宽高）**不加**。没修：属性不在，期望不符。
  - e2e（Chromium，`e2e/stories/asset/as3-render.spec.ts` 的 AS3 (page)，`:297` 那一格末尾）：建页 `Sizes`，内容 `![[cover.png|0x200]] ![[cover.png|300x200]]`，`goto` 它；两张图 `boundingBox()`：第一张高 200、宽约 133（`pngBytes` 是 2 × 3，`e2e/fixtures/assets.ts:88-94`），第二张 300 × 200，`objectFit === "contain"`；视口改成 375 宽之后第二张宽不超过文章、高仍 200。没修的代码上：第一张 2 × 3、第二张 300 × 450。
- 已有测试要随之改的：上面的 Go 期望。别的不碍（审查 C 的"核对过"第 10 条）：服务端每次读都渲染、不缓存 HTML（`page/app/get_page_view.go:39`）；整个程序的测试只以 `<img class="nw-asset" src=` 为前缀比对（`bootstrap/markdown_app_test.go:58-62`）；AS3 (API)、(MCP) 只取 `alt`、`width`、`src`。
- 变异：CSS 的 `:not(...)` 写反（尺寸断言）；服务端对图片自己的宽高也写 `data-nw-sized`（Go 的 `:130` 那格）；去掉 `object-fit`（计算样式断言）。
- 牵连：`version-view.tsx`、回收站的预览用同一份 CSS，一并生效。

##### 15.2 折叠的 callout 标题里的音视频控件，点了也开合 callout

- **现状**：
  - 服务端把折叠的 callout 写成 `<details class="nw-callout">`，标题是 `<summary>`（`obsidian/render.go:169-215`）。标题里的音视频嵌入照样是带 `controls` 的元素（`asset.go:117-121`）；只有"链接里"的才换成文字。
  - `W/reading/*.ts` 没有拦截（`reading.css:313-315` 只是 `cursor: pointer`）。
- **只在 Chromium 复现**。三种引擎、无头，在控件上从左到右点（审查 C 的 `m12/p2-review-c/probe3.txt`–`probe7.txt`，设计者的 `m12/p2-probe-15.2/run1.txt`，修订核对 C 的 `m12/p2-check1-c/probe.log`）：
  - **Chromium**：点控件左边的空白（`x=6`）与时间的显示（`x=60`），开合 `details`；播放键与进度条不开合。
  - **Firefox、WebKit**：哪里都不开合。
  - 用户碰到的样子：`> [!note]- ![[talk.mp3]] 讲座` 折叠着，在播放键旁边点了一下，callout 跟着展开。
- **文档上看得到的事件**：Chromium 的媒体控件在 UA 的 shadow DOM 里截下了 `pointerdown`、`mousedown`、`mouseup`、`click`（初稿听 `click` 的改法因此从不运行，审查 C 的 I1，`probe7.txt`）。点空白与时间处时，文档上只到 `DOMActivate`（目标是媒体）与随后的 `toggle`；点播放键没有媒体的 `DOMActivate`。Firefox 点时间处、WebKit 点控件的哪里，都派发媒体的 `DOMActivate`，却不开合。
- **改法**（P2-42，修订核对 C 的 M9）：新的增强 `W/reading/title-media.ts`，在 `W/reading/enhancement.ts:98-107` 的 `readingEnhancements` 末尾登记。只听一种事件，不留状态：
  - 容器上以捕获阶段听 `DOMActivate`：目标是某个 `<details>` 自己的 `<summary>` 里的 `audio`、`video` 时，取下这个 `<details>` 当时的 `open`，`setTimeout(0)` 里它还在文档里、`open` 变了，就设回去。
  - 不听 `toggle`、`keydown`、`pointerdown`：没有跨事件记下的东西，之后程序里的打开（锚点导航的 `unfold`，`reading-view.tsx:317`；焦点还原，`assets.ts:198`）不会被当作控件引起的而撤回。上一版留着记录、等下一次 `toggle` 复原，在 Firefox 与 WebKit 上会撤回点过控件之后的这种打开（修订核对 C 的 M9，`probe.log` 的 I 格）。
  - 不能只把"忘掉"挪进 `setTimeout(0)` 而仍在 `toggle` 里复原：Chromium 里计时先于 `toggle` 的任务，那样就不复原了（同一份日志）。
  - Chromium 里设回在 `toggle` 的任务之前，只派发一次 `toggle`，报的是复原之后的状态（日志 `"put back","toggle d open=false"`）。中间会不会画出一帧展开的样子，没有量（推断：计时与 `toggle` 都在下一次渲染之前）。
  - 读者点标题的文字、键盘的 Enter 与 Space，`DOMActivate` 的目标是 `<summary>`，不处理。
  - 不 `preventDefault`：控件自己的处理照常。清理时移除监听；已排上的计时照跑，它只看 `isConnected` 与 `open`，无害。
- **探针**（修订核对 C 的 `m12/p2-check1-c/probe.spec.ts`，`fix=timeout`，与改法同一套监听；三种引擎）：

  | 情形 | 没修 | 改了之后 |
  |---|---|---|
  | 折叠时点控件的空白、点时间（A，AS6 的前两步） | Chromium 展开 | 不展开 |
  | 展开时点控件的空白（G） | Chromium 合上 | 不合上 |
  | 点标题的文字（C） | 三种都开合 | 照常开合 |
  | 点播放键之后点文字（D） | 三种都开合 | 照常开合 |
  | 键盘 Enter、Space（E） | 三种都开合 | 照常开合 |
  | 正文里的媒体（K） | 不开合 | 照常 |
  | 点过控件，300 毫秒后脚本设 `open`（I） | 三种都打开 | 照常打开 |
  | 点过控件，工具栏"前进"到 callout 里的锚点（J） | 三种都打开 | 照常打开 |

  Firefox、WebKit 的结果改与不改相同。
- **测试**：
  - **vitest**（新 `W/reading/title-media.test.ts`，假计时）：jsdom 不画控件，所以直接派发事件、设 `open`，造出"引擎开合了"的次序。五格：
    1. "a toggle that a control of a medium in the title caused is undone"：媒体上派发 `DOMActivate`，再把 `open` 设为真；推过计时，断言 `open` 复原为假。没修时留在真。
    2. "the reader's toggle stays: the title's text"：`DOMActivate` 的目标是 `<summary>`，`open` 设为真，推过计时，断言留在真。
    3. "a medium out of a callout's title is not the callout's"：callout 合着，正文里的媒体派发 `DOMActivate`，再把 `open` 设为真，推过计时，断言留在真。要这样造：展开着、什么也不变时，"正文里的媒体也算"那个变异只在 `open` 变了时才设回，照过（修订核对 C 的 N15，`m12/p2-check2-c/vitest-matrix.txt` 的 `3` 与 `3'` 行）。
    4. "a callout opened by the app after a click on the control stays open"：媒体的 `DOMActivate` 之后没有开合，推过计时；之后程序设 `open` 为真，再推过计时，断言留在真（修订核对 C 的 M9；上一版的写法在这一格红）。
    5. "a control's click that toggled nothing leaves the callout alone"：callout 展开着，媒体的 `DOMActivate`，`open` 不变，推过计时，断言 `MutationObserver`（`attributes`）在 `<details>` 上什么也没记到。记录在观察者的回调里收集（或同步推进计时之后再 `takeRecords()`）：用 `advanceTimersByTimeAsync` 推进时回调在中途就收走了记录，之后的 `takeRecords()` 是空的，"一律设回"也照过（修订核对 C 的 N16，矩阵的 `5''` 行）。一律设回时 `open = true` 会再写一次属性，记到一条（jsdom 30.1.1 把布尔属性设为真即 `setAttributeNS`，`Element-impl.js:537`）；callout 合着时设回假不写属性，所以这一格要展开着的。
  - **e2e**（AS6，Chromium）"AS6 (page): a control of an audio in a folded callout's title leaves the callout as it was"：
    - 页的内容是 `> [!note]- ![[sound.wav]] Talk\n> body\n`，WAV 由夹具现做（三种引擎都放得出、播放不停，修订核对 C 的探针核对过）；不用 `oggOpus`：本机 Playwright 的 WebKit 上它的 `readyState` 10 秒内一直是 0（修订核对 C 的 N8）。
    - `goto` 之后，等音频可见、它的 `src` 带签名的查询串（肯定的东西）。不等 `readyState >= 1`：服务端写 `preload="none"`（`asset.go:122`），增强只在重签之后改成 `metadata`（`assets.ts:275`），第一次打开页时三种引擎的 `readyState` 10 秒内都是 0，这一步永远等不到（修订核对 C 的 M10，`m12/p2-check2-c/wav-probe.log` 的 `load preload=none`）。也不为了凑这个等待调 `load()` 或改成 `metadata`：那样测的不是读者看到的样子。
    - 在控件的 `x: 6` 与 `x: 60` 各点一次，每次之后等两帧，断言 `details.open === false`、正文不可见。
    - 再点标题的文字，断言展开（对照）。
    - 没修的代码上，第一次点之后 `open` 是真，这一格红在第一步的断言：Chromium 在 `readyState` 为 0 时点 `x: 6` 也展开；改了之后是假，点文字展开（`wav-probe.log` 的 `AS6 preload=none` 两行）。
    - Firefox、WebKit 不复现，这一格不进三种浏览器（P2-30）；S5 实施时在两者上各跑一次，记进记录。
- **变异**：
  - 不复原：vitest 第 1 格红，AS6 红。
  - 目标不是媒体的 `DOMActivate` 也处理：vitest 第 2 格红，AS6 的对照红。
  - 正文里的媒体也算：vitest 第 3 格红。
  - 记下 `details`、留到下一次 `toggle` 才复原（上一版）：vitest 第 4 格红。
  - 不比较、一律设回：vitest 第 5 格红。
- **牵连**：版本页、回收站的预览也跑这些增强（`W/pages/page/version-view.tsx:88`），一并生效。增强不留状态，S4 的 U4、U5 撤掉再重跑与它不交叉。

##### 17.2b 复制正文带上"在新标签页打开"的提示

- 现状：阅读视图的增强 `W/reading/assets.ts:120-133` 给浏览器能显示的附件链接加 `target`，在链接里追加一个空格与 `<span class="sr-only">（在新标签页打开）</span>`；附件一节的行（`W/pages/page/attachment-row.tsx:155-164` 的 `NewTab`）与右栏的属性链接（`W/pages/page/page-properties.tsx:257`）同样。全仓没有 `user-select`。复制一段带 PDF 链接的正文，粘出来是"doc.pdf (opens in a new tab) (12 KB)"。
- 改法（已定：2.3 的 P2-28）：只给提示的 span 加 Tailwind 的 `select-none`，**空格留在外面**：
  - `assets.ts:125-126`：`hint.className = "sr-only select-none"`（Tailwind 4.3 自动扫 `.ts`；它出 `-webkit-user-select` 与 `user-select`，实施时看一眼构建出的 CSS）。
  - `attachment-row.tsx:161` 的 span，同文件 `:108` 的来源提示 `page.sourceKind`（同一种复制问题，顺手）；`page-properties.tsx:257` 的 span。
  - 注释（`assets.ts:116-119`、`attachment-row.tsx:155`）补"不随正文复制"。
  - 证据：审查 C 的探针（`m12/p2-review-c/probe1.txt`）在同样的结构上（链接里"名字、空格、加了 `user-select: none` 的 `sr-only` 提示"，链接之后" (12 KB)"）选中整段 `Ctrl+C`，三种引擎复制出来都是 `doc.pdf (12 KB)`，没有提示，也没有连续的两个空格；没加的对照链接三者都带提示。
  - 不取移交的写法（空格进 span）：jsdom 里名字少一个空格（dom-accessibility-api 0.5.16 对每个子元素的名字 `trim()`，`accessible-name-and-description.js:262`），约 30 处 `name:` 断言要改成与真浏览器不符的样子。
- 测试：
  - e2e（新 AS6，Chromium）`"AS6 (page): copying a page's text leaves out the hints of links that open in a tab of their own, which their names keep: in the reading view, the properties and the attachments' rows"`：
    - 准备：上传 `doc.pdf` 与 `data.zip`（`e2e/fixtures/assets.ts`），页 `Guide` 的内容 `---\ncover: "[[doc.pdf]]"\n---\n[[doc.pdf]] [[data.zip|the data]] end\n`，附件放在 `Guide` 下；`goto`；`grantClipboardPermissions`（`e2e/fixtures/clipboard.ts:6-14`）。
    - 名字：文章、右栏、附件一节里 `getByRole("link", { name: "doc.pdf (opens in a new tab)", exact: true })` 都可见。
    - 复制：选中文章里那一段（`selectNodeContents`），`ControlOrMeta+c`（照 `e2e/stories/wiki/lw4-source-readonly.spec.ts:484-487`），`readClipboardText`；整句等于 `doc.pdf (N B) the data (M B) end`（N、M 是字节数）。右栏的值、附件一节的列表各复制一次，`not.toContain("(opens in a new tab)")`。
    - 没修的代码上：复制的文字含"(opens in a new tab)"。
  - vitest（结构）：`W/reading/assets.test.ts:132` 那格加"`.sr-only` 带 `select-none`"的断言；`attachments-section.test.tsx` 与 `page-properties.test.tsx` 各加一句。
- 变异：去掉任一处的 `select-none`（AS6 对应的复制断言与 vitest 的结构断言抓）；把空格放进 span（vitest 的名字断言全红）。
- 牵连：S4 的 17.1 就地换 `href` 不换链接节点，提示留着；17.2a（空格带下划线）是 C，不做。

##### 17.4 地址过期之后点下载链接静默失败

- 现状：`W/reading/assets.ts:86` 只在容器上听 `error`，链接的点击没有处理；地址由 `useAssetsExpiry` 在到期前一分钟重读（`W/pages/page/assets-expiry.ts:81-96`），睡眠、断网、重读失败时页面上留着过期的地址。点过期的下载链接（`a.nw-asset[download]`，`obsidian/asset.go:53-61`），浏览器下载到一个 403，只在下载栏里显示失败。新标签页的链接打开一个显示错误的标签页，不算静默；13.2 第 27 条为它定了"不在点击时再签"。
- 改法（已定：2.3 的 P2-29，只管下载链接）：新增强 `export const expiredDownloads: Enhancement = (container, context) => …`（`W/reading/expired-downloads.ts`），在 `readingEnhancements` 里登记在 `assets()` 之后；`W/reading/assets.ts:233-236` 的 `assetOf` 改成导出。容器上听冒泡的 `click`：
  1. 只管 `a.nw-asset[download][href]`；跳过 `event.defaultPrevented`、非左键、带修饰键的点击，与正是自己发出的那一下（`passing === link`）。
  2. `context.assetsExpire` 为 `null`、`assetOf(href)` 认不出、或还没到期：照常走。
  3. 过期：`preventDefault()`；异步 `context.assetAddress(id)` 拿新地址写回 `href`（签不到就留着旧的，浏览器说下载失败）；链接已不在文档里就停；然后 `passing = link; try { link.click(); } finally { passing = undefined; }`；每个到期至多 `context.reload()` 一次（`reloaded !== expires`）。
  - `passing` 只在这一次派发期间为真（`click()` 同步派发）。S4 之后（P2-22）就地换地址不重跑增强，闭包与链接都留着；记下不清的话，链接再次过期时直接走过期的地址，又静默失败（审查 C 的 M1）。文档注释照此写：程序里的那一下放行一次，之后的点击重新比到期。
  - `context.assetAddress` 答 `content_url`（`W/stores/asset.store.ts:341-343`）：对浏览器不显示的类型服务端照样以附件下载（`asset/adapter/http/content.go:139`）。
  - 13.2 第 27 条随之改（句子在 8.2）。
- 测试：
  - vitest（新 `W/reading/expired-downloads.test.ts`，照 `W/reading/assets.test.ts:31-64` 的 `setUp` 造容器与 `context`）：
    - `"a download clicked once the addresses expired is signed anew, goes from the new address, and the view is read again"`：容器里 `<a class="nw-asset" href="/api/v0/assets/f1/content?…" download>f.zip</a>`，`expires` 已过；文档上一个冒泡的监听记下每次点击的 `{ href, prevented }` 再 `preventDefault()`（挡住 jsdom 的导航）；`link.click()`；`await` 一轮；第一次 `prevented: true`，`signed` 是 `["f1"]`，第二次 `href` 是新地址、`prevented: false`，`reloads` 是 `["p"]`。没修的代码上：第一次 `prevented: false`，`signed` 空。
    - `"a download signed anew, its address changed in place since, is signed anew again as it expires again"`（审查 C 的 M1）：`assetsExpire` 是读一个变量的 getter（照 S4，P2-22）；过期时点一次，`signed` 是 `["f1"]`；不重跑增强，换 `href`、到期改到一小时之后，点一次照常走；`vi.setSystemTime` 过了新的到期，再点：`prevented: true`，`signed` 是 `["f1", "f1"]`。在 `passing` 只加不删的写法上：`signed` 仍是 `["f1"]`。
    - `"one clicked before they expire, or one that opens in a tab of its own, goes as it is"`（控制）。
    - `"one whose address cannot be signed goes as it is: the browser says it failed"`：`id` 为 `gone`，第二次点击的 `href` 是旧的。
    - `"a click with a key held goes as the browser has it"`（`ctrlKey`）。
  - e2e（AS6，Chromium）`"AS6 (page): a download whose address expired is signed anew as it is clicked, and downloads"`：`page.route` 改写本页阅读视图的答复，把 `assets_expire_at` 改成一分钟前（`route.fetch()` 再 `route.fulfill`，路由一直留着，05-lessons 第 6 节）；`goto`；同时等 `page.waitForRequest`（路径 `/api/v0/assets/${data.id}`）与 `page.waitForEvent("download")`，点"the data"；断言再签的请求来了、下载的字节等于 `zipBytes`；再等视图的重读请求。新地址多半与旧的相同（服务端按小时签），所以断言再签的请求。没修的代码上：`waitForRequest` 超时。
  - 只在 Chromium 的证据：审查 C 的探针（`m12/p2-review-c/probe1.txt`）拦下真实的点击，延后 0、300、1500、6000 毫秒再程序里 `click()` 一个 `<a download>`，三种引擎（无头）都下载了改过的地址。
  - 人工：真的 Chrome 的"自动下载多个文件"与 Safari 的"允许这个网站下载吗"会不会在程序里再点时弹出，无头的探针看不到；进 P5/S3 的人工清单（审查 C 未核实的第 2 条）。
- 变异：不比到期（控制格）；不记 `passing`（程序里的 `click()` 又被拦：`signed` 两次、循环）；`passing` 只加不清（第二格）；不 `reload`（第一格的 `reloads`）；签不到时不放行（签不到的那格）。
- 牵连：**S4**（硬依赖，P2-22）。右栏的属性链接与附件一节的下载链接有同样的窗口，由各自的到期重读（与 17.6）兜住，不在本项。

##### 17.6 系统休眠之后定时器晚到

- 现状：`W/pages/page/assets-expiry.ts:81-96` 与附件一节的 `useExpiry`（`W/pages/page/attachments-section.tsx:467-497`）各只设一个 `setTimeout`。机器睡着时计时不走（推断：按单调时钟），醒来之后地址已过期，计时还要等很久。SWR 在 `focus`、可见与 `online` 时重读（`W/app/providers.tsx:17-20` 没关），兜住了大部分；**兜不住**的是从往返缓存回来（`pageshow`，SWR 不听）与醒来之后焦点、可见性、网络都没变的情形。移交说的三个事件里两个 SWR 已在听，再听会各读一次。
- 改法（`assets-expiry.ts`，导出给附件一节用）：
  - `export function dueWatch(at: number, due: () => void): () => void`，常量 `lookAgainIn = 60_000`。
  - 不变量：每次至多隔 `lookAgainIn` 看一次时钟（`setTimeout(look, Math.min(at - Date.now(), lookAgainIn))`），`window` 的 `pageshow` 时也看；`Date.now() >= at` 时先停、再调一次 `due`；返回的 `stop` 清计时、移除 `pageshow` 的监听，并记 `over`，之后的 `look` 什么也不做。
  - `useAssetsExpiry`（`:81-96`）：`return dueWatch(due, () => { if (!document.hidden && !readAgain.has(data)) { readAgain.add(data); latest.current(); } });`。
  - `attachments-section.tsx` 的 `useExpiry`（`:487-495`）：`return dueWatch(Date.now() + rereadIn(earliest, Date.now()), () => { if (!document.hidden) latest.current(); });`。
  - 13.2 第 7 条随之补一句（计时至多隔一分钟看一次时钟，从往返缓存回到页面时立即比较；句子在 8.2）。
- 测试（vitest）：
  - `W/pages/page/reading-view.test.tsx`（`:627-650` 那一组之后）：
    - `"a view whose timer falls behind, the machine asleep, is read again within a minute of waking"`：`vi.useFakeTimers({ shouldAdvanceTime: true })`；视图 `assets_expire_at: inMinutes(10)`；读一次；`vi.setSystemTime(Date.now() + 3 * 60 * 60_000)`（只动钟、不跑计时，如睡眠）；`await act(() => vi.advanceTimersByTimeAsync(60_000))`；`reads() === 2`。没修的代码上：原计时还差约 9 分钟，`reads() === 1`。
    - `"…and at once as the page is shown again from the back-forward cache"`：同上 `setSystemTime` 之后派发 `pageshow`，`await waitFor(() => expect(reads()).toBe(2))`。没修：1。
  - `W/pages/page/attachments-section.test.tsx`（`:559-642` 那一组之后）：同样两格，数 `GET assets Guide`。
  - 已有的 `:559-642`、`reading-view.test.tsx:627-735` 在到期之前不读、到期才读，多出的每分钟看钟不改变读的次数；隐藏的标签页那格（`attachments-section.test.tsx:622-642`）照旧不读。
- 变异：计时不封顶（第一格）；不听 `pageshow`（第二格）；到期时不看 `document.hidden`（隐藏那格）；`stop` 不移除 `pageshow` 的监听：等价，`over` 挡住之后的 `look`，只多留一个监听，不抓。
- 牵连：不经阅读视图，与 S4 不交叉；`version-view.tsx:57`、`page-properties.tsx:65` 用同一个 hook，一并生效。

##### 17.7 重签被拒之后元素坏到下一次替换

- 现状：`W/reading/assets.ts:244-281` 的 `signAnew`：`assetAddress` 被拒时（`:257-263`）只在视图已过期时重读一次视图（`readAgain`，`:57-63`）；视图没过期（只是开始了的媒体还拿着旧地址，`adopt` 留下的，`:179-189`）就什么也不做。Firefox、Safari 的原生控件出错之后多半不重新加载（移交，推断），元素坏到下一次整篇替换（至多一小时）；Chromium 的控件重试会重新加载，回到 0 秒，`begun()`（`:214-216`）成了假，下一次 `error` 走 `readAgain`（`:83`），什么也不做。用户：听 40 分钟的录音，第 35 分钟断网十秒，网回来之后它一直坏着。
- 改法（`assets.ts`）：
  - `Signing`（`:111`）加 `refused: Map<HTMLMediaElement, { place: Place; retry: ReturnType<typeof setTimeout> | undefined; after: number }>`：重签被拒时失败的位置与下一次再签。
  - `signAnew` 加参数 `from?: Place`：`failing.place` 取 `from ?? placeOf(element)`。`catch`（`:259-263`）里，元素仍在文档中时：在 `signing.refused` 记下失败那一刻的位置（速率、音量、是否在播；已有记录的保留旧的位置），安排再签（`after` 从 30 秒起每次翻倍、至多 10 分钟），再照旧 `refused()`；成功（`:267` 之后）时删掉这一项并清计时。
  - 再签的三个时机，都从记下的位置签：`window` 的 `online`（对 `refused` 里仍在这个容器里的各签一次）；计时到点（只那一个）；`onError`（`:64-84`）时元素在 `refused` 里（不看 `begun` 与 `signedAgainAfter`，Chromium 的控件重试之后再出错走这里）。
  - `adopt`（`:168-178`）：旧元素出过错时，`putAt(element, signing.refused.get(old)?.place ?? goingOn(old, failing))`，新元素放到失败的位置。
  - 增强的撤销（`:87-103`）移除 `online` 的监听、清掉各计时；`refused` 按元素记，元素随 `adopt` 留到下一次运行的，表也留着（在 `assets()` 的闭包里，与 `signing` 同）。
  - 文档注释（`:17-45`）补一条。
- 测试（vitest，`W/reading/assets.test.ts`，`setUp` 的 `assetAddress` 加一个开关：`offline` 时拒绝）：
  - `"one whose signing anew is refused, the view not expired, is signed again as the browser is online again, from where it failed"`：`expires = later`；`play(audio, 42)`；`offline = true`；派发 `error`；`signed` 是 `["a1"]`，`reloads` 空；`loadAgain(audio)`（Chromium 的控件重新加载）；`offline = false`；派发 `online`；`signed` 是 `["a1", "a1"]`，`src` 是新地址，`currentTime` 是 42。没修的代码上：第二次签不会发生。
  - `"one refused is signed again half a minute later, then a minute, while refused"`：`vi.useFakeTimers({ now, toFake: ["Date", "setTimeout", "clearTimeout"] })`；一直 `offline`；走 30 秒签第二次，再走 60 秒签第三次。
  - `"one refused that fails again, its controls having loaded it anew, is signed anew from where it first failed"`：被拒之后 `loadAgain`（`begun` 为假）、再派发 `error`，签了并放到 42。没修：`begun` 为假走 `readAgain`，不签。
  - `"the HTML replaced after a refusal puts the new one where the old one failed"`：被拒、`loadAgain` 之后，撤销增强、换一份同一附件的 HTML、再运行；新元素的 `currentTime` 是 42。
- 已有测试要随之改的：`assets.test.ts:482-525` 两格（"one whose address is not given…"）要核对：被拒之后现在会安排再签，那两格只假了 `Date`（推断不受影响）。
- 变异：不听 `online`（第一格）；不记位置、用 `placeOf`（第一、三格的 42）；不安排计时（第二格）；`adopt` 不用记下的位置（第四格）；成功之后不清这一项（第一格末尾再派发 `online`，断言 `signed` 不再增加）。
- 假服务端：`setUp` 的 `context.assetAddress` 加 `offline` 开关。
- 牵连：**S4**（硬依赖，17.1 改 `kept`、`adopt`）。

##### 编辑器

##### 18.1 站内跳转离开时上传在途：附件照样落下，嵌入不插入，没人看到告知

- 现状：`W/pages/page/page-edit.tsx:485` 的 `<UnsavedGuard unsaved={editing.unsaved} stay={back} />` 只看未保存；`W/pages/page/unsaved-guard.tsx:31-33` 的 `useBlocker` 只在 `unsaved` 时拦。粘贴一张图（空选区）不改正文；有选区时删选区之后约 2 秒自动保存。点树上的另一页：编辑器卸下，上传照样完成，嵌入不插入，"未插入"的话没人看到。关标签页由 `AssetStore` 的提醒（`W/stores/asset.store.ts:178`）管，不缺。
- 改法：
  - `page-edit.tsx`：`const uploading = assets.uploads.some((upload) => upload.parent === page.id && upload.fromEditor && upload.failure === undefined && upload.result === undefined);`（`PageEdit` 是 observer），`<UnsavedGuard unsaved={editing.unsaved} uploading={uploading} stay={back} />`。
  - `unsaved-guard.tsx`：
    - props 加 `uploading?: boolean`（默认 `false`）。
    - `asking = unsaved || uploading`；`useBlocker` 的条件与"问的时候不再需要就照常离开"的 effect（`:34-38`）都用 `asking`。
    - `beforeunload` 的 effect（`:39-46`）仍只看 `unsaved`（上传由 `AssetStore` 的提醒管）。
    - 标题：`unsaved` 时 `editor.leaveTitle`，否则 `editor.leaveUploadsTitle`；描述：两者都在时用 `page.sentences` 连起 `editor.leaveDescription` 与 `editor.leaveUploadsDescription`，只有一样时各自那句。按钮"留下 / 离开"不变。
  - 两个键（4.5.0.2）。13.2 第 21 条随之补一句（句子在 8.2）。
  - 已定：2.3 的 P2-38，"仍要退出登录吗"不算在途的上传。
- 测试（vitest，`W/pages/page/page-asset-upload.test.tsx`）：
  - `test.each` 两种语言 `"a file pasted uploading, going to another page asks first: Stay keeps the edit; Leave goes, the upload goes on, its embed not inserted (%s)"`：`server.uploadsHeld = true`；粘贴一张图，等 `UPLOAD`；点树上的"Notes"（照 `W/pages/page/page-edit.test.tsx:152-176` 的 `tree()`）；`alertdialog` 的名字是 `Leave before the files are inserted?`（zh：`在文件插入之前离开？`），描述整句；点"Stay"，仍在编辑，焦点回到编辑器；再点"Notes"、"Leave"，到 Notes；`server.release()`；`server.nodes` 里有那张图，`PUT` 一次也没有。没修的代码上：没有对话框，直接到 Notes。
  - `"unsaved and uploading, leaving says both"`：先打字再粘贴，标题是"Leave without saving?"，描述是两句。
  - `"signing out as a pasted file uploads asks nothing"`：照 `page-edit.test.tsx` 里退出登录的格，粘贴后退出，没有对话框（4.5.0.5 第 3 条）。
- 变异：
  - `uploading` 不进 `useBlocker`：第一格。
  - `result !== undefined` 不排除：在第一格里先 `release`、等嵌入插入，再导航，断言不问；抓不到就在 Step 的记录里写明等价及其证据。
  - `beforeunload` 也看 `uploading`：等价，不抓。`AssetStore` 在有在途的上传时本来就挂着 `beforeunload`（`asset.store.ts:178`），多挂一个，行为相同。
- 牵连：S1 的 18.6（离开之后焦点）；S1 也可能加 `editor.*` 的键，文本冲突，后合并的先 `git merge main`。答复之后、组合中等插入的那一瞬（`result` 已有、嵌入未插入）不问（推断：对话框开着时编辑器失焦、组合已结束）。

##### 18.2 同步被拒（`.md`、超过上限）时选区已删；frontmatter 里选中带引号的整个值再粘贴，值成了空的

- 现状：`W/editor/loaded/asset-upload.ts:137-154` 的 `paste`：有选区且有文件就先删选区（`:149-151`），再 `take`；文件是否被拒在之后（`W/pages/page/page-edit.tsx:456-462` → `W/stores/asset.store.ts:247-265` 同步判 `.md` 与超过上限）。所以粘贴一个 `.md` 或 60 MB 的文件覆盖选中的文字，文字没了，什么也没插入。frontmatter：选中 `image: 'old.png'` 里的 `'old.png'` 粘贴一张图，引号随选区删掉，`embedAt`（`:222-235`）拒绝插入，属性成了空值（Codex M7 评审修复核对 A2-Q1）。
- 改法：
  - `W/stores/asset.store.ts`：把 `:253-257` 的两个同步判断提成导出的 `refusalOf(file: File, untitled: string, maxBytes: number | undefined): UploadRefusal | undefined`（`isPageName(fixedName(file.name, untitled))` 给 `{ refused: "page-file" }`；`maxBytes !== undefined && file.size > maxBytes` 给 `{ refused: "too-large", max: maxBytes }`）。`upload` 用它（`upload.failure = refusalOf(…)`，有值就不发）。
  - `W/editor/registry.ts:12-29` 的 `EditorContext` 加必填的 `refuses(file: File): boolean`（文档注释：uploadAsset 发送之前就会拒绝它时为真；粘贴留着没有东西替代的选区，M12 polish 18.2）。
  - `page-edit.tsx` 的 `context` 加 `refuses: (file) => refusalOf(file, t("asset.untitled"), instance.info?.asset_max_bytes) !== undefined`。
  - `asset-upload.ts` 的 `paste`（`:148-152`）：只在 `from !== to && named.some((file) => !context.refuses(file)) && takesEmbed(view.state, from, to)` 时删选区（`userEvent: "delete.cut"`），然后照旧 `take(view, from, named, folders)`。
  - `takesEmbed(state, from, to)` = `"insert" in embedAt(state.update({ changes: { from, to } }).state, from, "x.png", undefined)`：在删掉之后的状态上判。
  - 选区留着时文件照样上传：被拒的行说为什么；落点不收的，结束时编辑器说"未插入"，与今天"空选区落在不收的位置"一致。文档注释（`:95-98`）的"which it replaces"补条件。
- 测试：
  - `W/editor/loaded/asset-upload.test.ts`（`editing()` 的 `context` 加 `refuses: (file) => file.name.endsWith(".md")`）：
    - `"a paste of files all refused before they go keeps the selection: nothing would take its place"`：`editing("See it here.", { anchor: 7, head: 11 })`；粘贴 `notes.md`；`text() === "See it here."`；`going` 里有 `notes.md`。没修的代码上：`"See it ."`。
    - `"a quoted value selected in a frontmatter, whose quotes would go with it, is kept: the file goes, and is said not inserted (Codex M7 review fix check A2-Q1)"`：`doc = "---\nimage: 'old.png'\n---\nBody"`，选区是 `'old.png'`（含引号）；粘贴 `b.png`；`text() === doc`；答复之后 `told` 末句是 `notInFrontmatter` 的英文。没修的代码上：`"---\nimage: \n---\nBody"`。
    - 控制 `"a value selected inside its quotes is replaced by the embed"`：选区是 `old.png`（不含引号），结果 `image: '![[b.png]]'`。
  - `W/pages/page/page-asset-upload.test.tsx`：`"a Markdown file pasted over a selection keeps it: its row by the editor says why"`（整个程序：`page-edit` 真的把 `refuses` 接到 store）。没修：选区没了。
- 已有测试要随之改的（`refuses` 必填，tsc 要求每个字面量都有）：`W/editor/idle-exit.test.ts:20`、`W/editor/extensions.test.ts:20`、`W/editor/autosave.test.ts:21`、`W/editor/loaded/link-completion.test.ts:69`、`W/editor/source-editor.test.tsx:27`，各加 `refuses: () => false`；`W/editor/loaded/asset-upload.test.ts:54` 用上面的规则。`asset-upload.test.ts:146-167`、`:211-221`、`:288-314` 的期望不变。
- 变异：不看 `refuses`（第一格）；不看 `takesEmbed`（第二格）；`page-edit` 的 `refuses` 恒为 `false`（整个程序那格）；`takesEmbed` 改在删之前的状态上判：现有三格抓不住（删前删后结论相同），要另造一格，例如选区是 `'old.png`（含开引号、不含闭引号），形状照 `embedAt` 对未闭合引号的判断定，实施时核对。
- 牵连：18.3（撤销的粒度）是 C，不做。`refusalOf` 由 `upload` 与编辑器共用，knip 不报。

##### 12.8 补全的"表格"与服务端不对齐：表格里选别名写出不转义的 `|`

- 现状：`W/editor/loaded/link-completion.ts:151` 按 lezer 的语法树判"在表格里"（`at.includes("Table")`）；在表格里别名写成 `\|`（`:231`），`#` 补全按单元格判（`:170-188`）。lezer 的 GFM 表格要表头与分隔行格数相等；服务端的 goldmark v1.8.6 宽一些：
  - 表头少于分隔行也算（`extension/table.go:253-255`、`:167-170`）；
  - 表头里 `\` 之后的 `|` 不分格（`:225-227` 只看前一个字符），所以 `\\|` 在 goldmark 里不分格，lezer 里分；
  - 段落里第一条分隔行之前的那一行是表头，表格从那一行起到段落末尾（`:162-187`）；表头多于分隔行就不是表格，也不再往下找（`:168-170`）。
  - 这时 lezer 看到的是段落，补全写出 `[[Plans|Roadmap]]`，服务端把它切成两格。与 I3 的不同：还有表头里的 `\\|`（M6/P7 第 15 节）。
- 改法：新文件 `W/editor/loaded/gfm-table.ts`，`export function inTable(state: EditorState, pos: number): boolean`（文档注释注明 M6/P7 fix check fc3-c1-2）：
  - 从 `syntaxTree(state).resolveInner(pos, -1)` 往上找；有 `Table` 即真。
  - 找到 `Paragraph` 时取它的首行到末行，从第二行起找第一条分隔行（照 `isTableDelim` 与四个正则，`table.go:124-143`：行首缩进至多 3、只含空白 `-` `|` `:`、不全是 `-`；按 `|` 切、去掉首尾的空白格，每格匹配 `^\s*:?-+:?\s*$`）。
  - 找到第 n 行：表头（第 n−1 行）的格数（照 `parseRow`：去首尾空白、去首尾的 `|`，按前一个字符不是 `\` 的 `|` 切）大于分隔行的格数就返回假，否则返回"光标所在行 ≥ n−1"；没有分隔行返回假。
  - `link-completion.ts:151` 改成 `table: frontmatter === undefined && inTable(state, pos)`。`inLinkWritten` 的 `cellStart`（`:180-188`）已与 goldmark 同样只看前一个字符，不改。
- 测试（vitest，`W/editor/loaded/link-completion.test.ts`，照 `:307-333` 的写法）：
  - `"in a table the server finds and lezer does not, a header of fewer cells than its delimiter row, an alias is written with \\| (M6/P7 fix check fc3-c1-2)"`：`head = "| a |\n| - | - |\n| x | "`；`type(view, "[[Road")`、选 Roadmap；`${head}[[Plans\\|Roadmap]]`。没修的代码上：`[[Plans|Roadmap]]`。
  - `"… a header whose \\\\| the server does not split at"`：`head = "| a \\\\| b |\n| - |\n| "`（TS 字符串里写出 `a \\| b`），同样 `\\|`。没修：`|`。
  - 控制 `"a header of more cells than its delimiter row is no table, nor the paragraph's lines before its header: an alias is written with |"`：`"| a | b | c |\n| - | - |\n| "` 与 `"[[‸\n| a |\n| - |"`（`editingAt`）都写 `|`。
  - 可选新文件 `W/editor/loaded/gfm-table.test.ts` 对 `inTable` 逐形状列表（分隔行全是 `-`、缩进 4、冒号两端、没有前后 `|` 的行）。
  - 已有的 `:307-333`、`:347-380`、`:738-748` 的表格都是格数相等，不受影响。
- 变异：只认 lezer 的 `Table`（第一、二格）；表头按 lezer 的规则切（第二格）；表头多于分隔行也算（控制格）；从段落首行起算表格（控制格的第二种）。
- 牵连：0.5 第 4 条的差分探针。

##### 导入导出与来源

##### 19.4 重读改了游标时，那一下"加载更多"不起作用

- 现状：`W/stores/transfer.store.ts:181-195` 的 `addPage`：答复回来时 `cursor !== this.nextCursor`（期间的重读改了游标：新任务插在最前，页的分界移动，`server/internal/modules/transfer/app/reads.go:99-110`）就返回 `[]`；`W/pages/notebook/transfer-page.tsx:184-195` 的 `useLoadMore` 收到空，要再点一次。有任务在跑时列表每秒重读（`transfer-page.tsx:46-49`），窗口不小。`W/stores/transfer.store.test.ts:133-143` 正断言了这个"不加"。
- 改法：`addPage` 的 `:183-185` 改成
  ```ts
  if (cursor !== this.nextCursor) {
    // A read again moved where the jobs held end: the page after them now is the one asked for.
    return this.nextCursor === null ? [] : this.more();
  }
  ```
  `more()`（`:160-175`）以新游标去重：期间再按一次"加载更多"拿到同一个承诺；外层承诺的 `finally`（`:168-172`）只在 `this.next.added` 还是它时才清，不会清掉跟进的那次。注释（`:177-180`）改写。
- 测试（vitest，`W/stores/transfer.store.test.ts`）：
  - 把 `:133-143` 改成 `"a page answered once a read again moved the cursor is read again from where the jobs held end now, and added"`：`load` 得 `[4, 3]` 游标 `c2`；`more()`（请求 1 问 `c2`，扣住）；`load()` 答 `[5, 4]`、`c4`；答请求 1 `[3]`；跟进的请求 2 问 `c4`，答 `[3]`；`more` 解析为 `[job(3)]`；`ids(store)` 是 `[5, 4, 3]`，`nextCursor` 是 `null`；`lists.map((l) => l.ask)` 是 `[undefined, "c2", undefined, "c4"]`。没修的代码上：`more` 解析为 `[]`，没有问 `c4`。
  - 另加 `"a read again that reads the jobs to their end leaves More nothing to read"`：重读答 `next_cursor: null` 时 `more` 解析为 `[]`、不再请求。
- 变异：返回 `[]`（第一格）；跟进时不判 `null`（第二格多一次请求、或请求 `null`）。

##### 20.1 自己的导入成功、对话框关闭之前，"已有进行中的导入"闪一下

- 现状：`W/pages/notebook/import-dialog.tsx:301` 只要 `transfers.writing` 就显示 `importUnderWay`（`role="alert"`），"导入"随之 `aria-disabled`（`:391`）。推断：成功之后的那一下（`W/stores/transfer.store.ts:290` 的 `first()` 与 `import-dialog.tsx:167-171` 的 `close()` 在同一轮微任务里，React 合批）多半画不出来；**真正会闪**的是上传在途时任务列表被重读（别的任务在跑时每秒轮询，或 SWR 的焦点重读），而服务端已建好这个导入（提交之后、202 到达之前），"已有进行中的导入"出现在正在上传的对话框里。与 I3、移交的不同：成因多半是这条。
- 改法：`:301` 改成 `{transfers.writing && !sending && !refusedBusy && <Alert>…</Alert>}`（`refusedBusy` 见 20.2）。照移交"按 `importing && !sending` 显示"。
- 测试（vitest，`W/pages/notebook/transfer-import.test.tsx`）新加 `"an import uploading that the jobs read again already show does not say an import is under way"`：`jobsServer` 里有一个导出在跑（列表每秒读），导入的答复扣住；打开、选文件、导入；等请求到了，`server.jobs` 加一个进行中的 `Vault.zip` 导入；走 1 秒，等列表多读一次；对话框里没有 `alert`。没修的代码上：出现 `importUnderWay` 那句。`:246-267`（打开时就有进行中的写）照旧显示。
- 变异：去掉 `!sending`（这一格）。
- 牵连：P3/S3 在 `transfer-import.test.tsx` 加一格，文本冲突，后合并的先 `git merge main`。

##### 20.2 被拒、重读显示进行中的导入时，两条几乎相同的提示同时播报

- 现状：`import-dialog.tsx:301-302`：409 `transfer.busy` 的横幅（`importBusy`，`:218`）与重读之后的 `importUnderWay` 同时是 `role="alert"`。`W/pages/notebook/transfer-import.test.tsx:616-643` 正是这个情形（只断言了焦点与 `aria-disabled`）。
- 改法：`ImportForm` 加 `const [refusedBusy, setRefusedBusy] = useState(false)`；`onSubmit` 开头 `setRefusedBusy(false)`；`submit` 的发送函数里 `catch`（`:277-280`）在重新抛出之前，`error instanceof ApiError && error.code === "transfer.busy"` 时 `setRefusedBusy(true)`（`ApiError` 从 `../../services/api` 引入）。`:301` 的条件加 `!refusedBusy`。只留横幅：它回答这次按下，先到先播；常驻的那句压下；下一次提交清掉。
- P3/S3 的新码 `transfer.too_many_jobs`：`refusedBusy` 只认 `transfer.busy`。导入的 `Check` 先 `transfer.busy`、后 `transfer.too_many_jobs`、再 `server_busy`（P3 设计 3.16，决定 373），所以这本笔记本里有进行中的导入（或恢复、撤销）时答的是 `transfer.busy`；`too_many_jobs` 只在没有时才答出，横幅只说它自己的那句，不会与 `importUnderWay` 同时出现（推断，照服务端的次序）。P3/S3 在 `transfer-import.test.tsx` 加的那一格照 P2 合并之后的写法。
- 测试（vitest）：在 `transfer-import.test.tsx:616-643` 那格末尾加 `expect(within(shown).getAllByRole("alert").map((alert) => alert.textContent)).toEqual(["An import, a restore or an undo is under way in this notebook, perhaps someone else's. Wait for it to end, then try again."])`。没修的代码上：两条。可选另加 `"refused for another's import, then pressed again, says the standing one once the answer is not a refusal"`。
- 变异：不设 `refusedBusy`（这一格）；重新提交不清（可选格）。
- 牵连：P3/S3 同改 `transfer-import.test.tsx`，文本冲突，后合并的先 `git merge main`。

##### 20.3 鼠标双击"取消上传"，第二下落在随后出现的"取消"上，关掉对话框、丢了选的文件

- 现状：`import-dialog.tsx:378-386`：发送中是"Stop the upload"，停下之后同一位置换成"Cancel"（关闭对话框，`:189`、`:383`）。中止是同步的，几毫秒后换按钮，双击的第二下落在"Cancel"上（推断：它右对齐，原按钮中点多半落在它上面）。
- 改法：`:383` 的"Cancel"：`onClick={(event) => { if (event.detail > 1) return; cancel(); }}`，注释："The second click of a double one on Stop the upload, which this takes the place of, closes nothing (P6B review)."。键盘激活的 `detail` 是 0，不受影响。
- 测试（vitest，`transfer-import.test.tsx`）新加 `"a double click on Stop the upload stops it, and the dialog stays with the file chosen"`：导入挂住；`fireEvent.click(stop, { detail: 1 })`；等"Cancel"出现；`fireEvent.click(cancel, { detail: 2 })`；对话框仍在，文件仍是 `Vault.zip`；再 `detail: 1`，对话框关闭。没修的代码上：`detail: 2` 就关了。`detail` 由引擎按平台的点击次数给（推断三引擎一致），不另加 e2e。已有的格 `user.click` 的 `detail` 是 1，不受影响。
- 变异：去掉 `detail` 判断（这一格）；判成 `>= 1`（再点一次的那步抓：键盘与单击都关不掉）。
- 牵连：P3/S3 同改 `transfer-import.test.tsx`，文本冲突，后合并的先 `git merge main`。

##### 38.7 Esc 或点外面关掉粘贴对话框时，不问就丢掉已粘贴的文字

- 现状：`W/pages/notebook/source-add-dialog.tsx:59-62,69`：`Dialog` 的 `onOpenChange` 直接关；`DialogContent`（`:71-79`）没有 `onEscapeKeyDown`、`onInteractOutside`。标题与正文是 `PasteSource` 的状态（`:132-133`），一关就没了。对照：导入对话框在上传时 Esc 改问（`W/pages/notebook/import-dialog.tsx:120-125`），令牌对话框挡住 Esc 与点外面（`W/pages/settings/create-token-dialog.tsx:48-55`）。"取消"（`:209-211`）是明确的动作，不问。用户：粘了一大段 Markdown、打了标题，手一滑按了 Esc，全没了。
- 改法（`source-add-dialog.tsx`）：
  - `PasteSource` 加 prop `report: (state: { draft: boolean; sending: boolean }) => void`，在 effect 里报 `{ draft: title !== "" || content !== "", sending }`；`SourceAddDialog` 记在 ref 里，另有 `asking` 状态与问之前拿着焦点的元素。
  - `DialogContent` 的 `onEscapeKeyDown`：`asking` 时 `preventDefault()` 并"继续编辑"；否则 `holding(event)`。`onInteractOutside`：`asking` 时 `preventDefault()`；否则 `holding(event)`。
  - `holding` 的次序：不是粘贴的模式就照常关；发送中 `preventDefault()`，留着（关了也停不下已发的，同 `ConfirmDialog`，13.2 第 12 条）；有草稿时 `preventDefault()`，记下 `document.activeElement`，`setAsking(true)`；空的照常关。
  - "继续编辑"：`setAsking(false)`，焦点回到记下的元素（仍在文档里时）。
  - 问的一块照导入对话框（`import-dialog.tsx:136-159`）放在 `DialogDescription` 之后：`role="alert"`，一句 `source.discardAsk`，"继续编辑"（出现时取焦点，同 `:73-77`）与"丢掉并关闭"（`destructive`）。关闭之后 `asking` 复原。
  - 三个键（4.5.0.2）。13.2 第 12 条随之补一句（句子在 8.2）。
- 测试（vitest，`W/pages/notebook/source-add-dialog.test.tsx`）：
  - `test.each` 两种语言 `"a draft is not lost to Escape or a click outside: the dialog asks, Keep editing keeps it and the focus where it was, Discard and close closes (%s)"`：`pasteDialog(user)`；`draft(dialog)`；焦点放到 Markdown 文本框；按 Esc；对话框仍在，`alert` 整句（en / zh），焦点在"Keep editing"（zh：继续编辑）；点它，问的一块消失，两个字段的值不变，焦点回到 Markdown 文本框；`user.click(document.body)`（推断由此触发 Radix 的 `onInteractOutside`；不行就 `fireEvent.pointerDown(document.body)`），又问；点"Discard and close"，对话框关闭，`creates(server)` 为空。没修的代码上：Esc 直接关，找不到 `alert`。
  - 控制 `"an empty dialog closes at once on Escape"`。
  - `"a paste being sent stays on Escape, asking nothing"`：照 `:250-` 那组扣住 `POST`，Esc 之后对话框仍在、没有 `alert`。
  - 已有的 `:204-249`、`:250-` 都用"Cancel"关，不受影响。
- 变异：不挡 `onInteractOutside`（第一格的后半）；空的也问（控制格）；问的时候焦点不给"继续编辑"（焦点断言）；发送中照样关（第三格）。
- 假服务端：`sourceServer` 的 `answers`。
- 牵连：S1 的 38.3（粘贴对话框走 `useForm`，改在共用的 `form-field.tsx`）与 S3 的 38.11（对话框标题行超宽，`dialog.tsx`）都碰这个对话框的周边，文本冲突，后合并的先 `git merge main`。38.2（降级后"添加来源"悄悄禁用）是 C，不做。

#### 4.5.4 补记

- 05-lessons 第 6 节"jsdom 会去掉 `sr-only` 里开头的空格"更准确的原因是 dom-accessibility-api 对每个子元素的名字 `trim()`（`accessible-name-and-description.js:262`，不只 `sr-only`）；S5 收尾时可以照实补一句。

### 4.6 S6 版本、回收站、历史、目录、后台任务与搜索

- **本节的取舍**：P2-31 至 P2-35、P2-39、P2-41（2.3）。
- **与 P4/S2 的关系**：S6 在 P4/S2 的页面树结论之后才开，结论要改树时在改法的 Step 之后（P2-1）。只有 27.2 的改法取决于结论；12.12、22.14、23.10、24.8 的代码要随改法改，见 4.6.6。
- **开工之前补**：结论出来、S6 开工之前，照 4.6.6 的表补下面几样：
  1. 27.2 的改法：保留整棵树时照 4.6.7 的 27.2；分层加载时改成按页读节点与祖先，vitest 的准备随之换。
  2. 12.12 的 `canHold` 在分层加载时怎样判深度：服务端给高度，或只靠 422。
  3. 22.14、23.10 的包装与 `restore`、`revert` 重写之后的合并。
  4. 24.8 的"树的读"换成"受影响的层的读"。
  5. 4.6.6 末的附带观察交给 P4/S2 量：移动对话框的 `canHold`、树行的 `subtreeOf`。

#### 4.6.1 总表

| 编号 | 分类 | 组件文件（主要） | 测试 | 测试文件 | 大小 |
|---|---|---|---|---|---|
| 2 | A | `W/pages/page/page-layout.tsx`（`PageGone`）、`W/pages/notebook/new-page.ts` | vitest | `W/pages/notebook/page-tree-writes.test.tsx` | 小到中 |
| 12.12 | A | `W/stores/page-tree.ts` | vitest | `W/stores/page-tree.test.ts` | 小 |
| 21.2 | B | `W/pages/page/version-page.tsx` | vitest | `W/pages/page/version-restore.test.tsx` | 小 |
| 21.3 | B | `W/pages/page/page-layout.tsx`、`W/app/arrival.ts`（新帮手） | vitest | `W/pages/page/version-restore.test.tsx`、`W/app/arrival.test.tsx` | 小 |
| 21.7 | A | `W/pages/page/comparison.tsx`、`W/editor/text-diff.tsx` | vitest | `W/pages/page/version-page.test.tsx` | 小 |
| 21.10 | B | `W/pages/page/version-page.tsx` | vitest | `W/pages/page/version-page.test.tsx` | 小 |
| 22.2 | B | `W/pages/notebook/trash-page.tsx`、`trash-preview.tsx`（用 21.3 的帮手） | vitest | `W/pages/notebook/trash-preview.test.tsx` | 小 |
| 22.14 | A | `W/stores/page-tree.store.ts`、`W/pages/notebook/trash-preview.tsx`、`trash-row.tsx` | vitest | `trash-preview.test.tsx`、`trash-page.test.tsx`、`W/stores/page-tree.store.test.ts` | 中 |
| 22.19 | B | `W/pages/notebook/trash-preview.tsx` | vitest | `trash-preview.test.tsx` | 小 |
| 23.10 | A | `W/stores/page-tree.store.ts`、`W/pages/notebook/revert-dialog.tsx`、`changeset-page.tsx` | vitest | `W/pages/notebook/revert-dialog.test.tsx`、`page-tree.store.test.ts` | 中 |
| 23.19 | A | `W/pages/notebook/changeset-page.tsx`、`trash-preview.tsx` | vitest | `revert-dialog.test.tsx`、`trash-preview.test.tsx` | 小 |
| 24.2 | A | `W/pages/notebook/revert-dialog.tsx` | vitest | `W/pages/notebook/revert-background.test.tsx` | 小 |
| 24.8 | D | `W/events/handlers.ts`、`W/app/event-stream.tsx` | vitest + e2e（TB2） | `W/events/handlers.test.ts`、`W/pages/page/page-backlinks.test.tsx`；`e2e/stories/trash/tb2-background-restore.spec.ts` | 小到中 |
| 24.9 | B | `W/pages/notebook/changeset-page.tsx` | vitest | `W/pages/notebook/revert-dialog.test.tsx` | 小 |
| 24.10 | D | `e2e/stories/trash/tb2-background-restore.spec.ts` | e2e | 同左 | 小 |
| 26.2 前一半 | B | `W/pages/workspace/search-box.tsx` | vitest | `W/pages/workspace/search-box.test.tsx` | 小 |
| 26.5 | B | `W/components/ui/native-select.tsx`（新帮手）、`W/pages/search/search-page.tsx`、`history-filters.tsx`、`index-filters.tsx` | vitest + e2e（SE2 一段） | `W/pages/search/search-page.test.tsx`、`history-page.test.tsx`、`index-page.test.tsx`；`e2e/stories/search/se2-search-visibility.spec.ts` | 中 |
| 26.15 | D | （只加测试）`W/pages/workspace/workspace-layout.tsx:115` | vitest | `search-box.test.tsx` | 小 |
| 26.18a | B | `W/lib/text-ranges.ts` | vitest；字形人工 | `W/lib/text-ranges.test.ts` | 小 |
| 27.2 | B | `W/stores/page-tree.store.ts`、`W/pages/notebook/notebook-layout.tsx`、`W/pages/page/page-layout.tsx`、`version-page.tsx` | vitest + e2e（SE1 一段） | `W/pages/page/page-layout.test.tsx`；`e2e/stories/search/se1-search.spec.ts` | 中 |
| 35.3 | A | `W/pages/notebook/index-page.tsx` | vitest | `W/pages/notebook/index-page.test.tsx` | 小 |

#### 4.6.2 本 Step 新加或改的 i18n 键

只有一个新键；其余各项文字不变（23.10 的忙按钮借用已有的 `history.reverting`）。

| 键 | en | zh-CN | 项 |
|---|---|---|---|
| `index.badFilters`（新） | These filters cannot be read: the address has one this directory does not take. Clear the filters to see every page. | 读不出这些筛选：地址里有这个目录不认的筛选。清除筛选即可看到全部页面。 | 35.3 |

- 句式照 `history.badFilters`（en `en.ts:433`、zh `zh-CN.ts:419`），后半句指向已有的"清除筛选"（`index.clearFilters`，`en.ts:314`）。S2 若改 `history.badFilters`，这一句随之改。

#### 4.6.3 改到的 e2e 故事

- 三处，见 5.2 的 S6 三行：TB2 (page)（24.8、24.10）、SE1 (page)（27.2）、SE2 (page)（26.5）。
- 都只在 Chromium，不进 `threeBrowsers`（`e2e/playwright.config.ts:3-4`）：查的是事件的次序、树的读与选择的事件，不是焦点修正或滚动。
- 21.3、22.2 的"刷新之后不再说"只用 vitest 断言历史条目的 state，不在 HI2 加真的刷新（2.3 的 P2-41）。

#### 4.6.4 提交的分组

见 6.6。

#### 4.6.5 拿不准的、要实施者先核对与审查重点看的

1. TB2 (page) 在没修的代码上每次都红，靠"links 帧与 pages 帧同一批到、早于树的读的答复"（推断）：在撤掉 24.8 修复的分支上经 `heavy.sh` 跑一次，看它红 → 24.8。
2. 24.2 的最终焦点（推断在"取消"）取决于 effect 的次序：先在 S1 合并之后的树上探出，再写断言 → 24.2。
3. Windows 上关着的选择的事件次序没有核实（探针只在 Linux）→ 26.5。
4. `useNavigation()` 在路由级 `lazy` 的分包加载时是 `"loading"`（推断；测试用带 `loader` 的路由造出这个状态）→ 2。
5. 数 `GET nodes` 的已有 vitest 要不要随之改，以全量 vitest 找出 → 27.2。

#### 4.6.6 与 P4/S2 的页面树结论的关系

S6 在 P4/S2 的页面树结论出来之后才开；结论要改树时，在改法的 Step 之后开（主持者的决定）。P4/S2 在 1 万页上量三条界线（00 号文档 4.7.2：T1 冷打开 ≤ 1 秒、T2 别人改一次树 ≤ 1.5 秒且长任务 ≤ 200 ms、T3 展开 1,000 个孩子 ≤ 500 ms）。都过，保留整棵树；没过，数据一侧多半是分层加载，只有"平"的形状渲染没过多半是只渲染看得见的行。

只有 27.2 的改法取决于结论。12.12、22.14、23.10、24.8 的改法不取决于它，但结论要改树时，它们的代码要随改法改（下表）。00 号文档点名的是 12.12、27.2、22.14、23.10。

树今天怎样读、怎样重读（I2 第 2.2 节，`194ca350` 核对过）：一本笔记本每代一个 `PageTreeStore`（`W/stores/root.store.ts:334-347`），`nodes` 整份替换（`W/stores/page-tree.store.ts:78`、`:283-308`）；SWR 的键 `["pages", 笔记本 id]` 有 12 处挂着（`page-tree.tsx:63`、`page-layout.tsx:53`、`version-page.tsx:55` 等）；自己的每个树的写之后直接 `this.load()`（不经 SWR，`:389`）；别人的改树经 `pages` 事件的 refresher，至多 500 ms 一次，而且只有挂着的钩子才真的读（SWR 对没挂的键的 `mutate` 只丢去重的记录）。

| 项 | 保留整棵树时 | 改成分层加载时 | 改成只渲染看得见的行时 | 结论之后怎样落 |
|---|---|---|---|---|
| 12.12 | 4.6.7 的两个纯函数（`page-tree.ts:116`、`:124`） | 只在已读的层上算：`removed` 只含已读的页（没读的也没画，无害）；`canHold` 看不到没读的子孙，深度要服务端给高度或只靠 422（推断）。测试照样有效 | 不受影响 | 改法照本文；分层加载时 `canHold` 的深度随改法改 |
| 27.2 | 4.6.7 的甲' | 页的外壳多半改成按页读自己的节点与祖先，过时的整树随之没了；`watch`、`confirmed` 不要，闸门换成"这一页的节点的读已答"。SE1 的一段照用，vitest 的准备要换（按页的读答 404） | 不受影响 | **改法取决于结论**：保留整棵树照本文；分层加载照左格另写 |
| 22.14、23.10 | 4.6.7 的两张表 | 表、读法与测试不变；`restore()`、`revert()` 的后半（写之后读什么、`revert` 比前后两棵树，`page-tree.store.ts:593-620`）随改法重写，包装照旧包住整个方法的 promise | 不受影响 | 改法照本文；分层加载时包装加在重写之后的方法上 |
| 24.8（00 号文档没点名） | 4.6.7：links 的读等树的读 | "树的读"换成"受影响的层的读"，机制照旧 | 不受影响 | 改法照本文；分层加载时换读的对象；P4/S2 若改 `pagesChanged` 的树的读（`handlers.ts:65-78`），同一处 |
| 2（附带） | 照旧 | `removedTo` 的语义不变 | 不受影响 | 照本文 |

其余各项不碰 `PageTreeStore`，与结论无关。

**附带的观察，给 P4/S2**（推断，没有量）：`W/pages/notebook/move-page-dialog.tsx:120` 对树里每一页调 `canHold`，每次都重算被移的页的子树与高度（`page-tree.ts:132-137`），"深"的形状里移一个 1,000 页的文件夹约 10⁷ 量级的步数；`W/pages/notebook/page-tree-item.tsx:274` 每一行渲染都算一次 `subtreeOf`。不在三条界线里，量的时候可以顺手看"移到……"对话框打开的时间。

#### 4.6.7 逐项

##### 2 删除与新建之后的导航被旧位置覆盖

- **现状**：
  - 删除：删掉正在显示的页，答复时 `PageTreeStore.remove` 把子树记进 `removed`（`W/stores/page-tree.store.ts:367-373`），`PageLayout` 渲染 `PageGone`（`W/pages/page/page-layout.tsx:63-65`），它无条件渲染 `<Navigate replace … state={arrived}>`（`:75-86`）。
  - 新建：`useNewPage.create` 在答复之后，只要发起的组件还挂着、已提交的地址的 `key` 没变就跳到新页（`W/pages/notebook/new-page.ts:36-44`、`:70-77`）。
  - 写在途时用户点了一个第一次去的路由（`W/app/routes.tsx` 全是 `lazy`，分包还在加载）；答复一到，旧页的 `<Navigate replace>`（或新建的跳转）打断用户的导航，落到父页（或新页）。不丢数据。I3 说可降 C；不改判：落到别处是错的行为，改法小。
- **改法**：
  - `PageGone`（`:75-86`）：在 `to === undefined` 的判断之后、`<Navigate>` 之前，`useNavigation().state !== "idle"`（`react-router` 8.4.0）时渲染 `<Loading />`：读者发起的导航在途，让它去读者要去的地方；它结束或被取消，`PageLayout` 都卸下。
  - `new-page.ts`：每次提交之后的 effect 把 `useNavigation().state !== "idle"` 记进 ref `navigating`；`create` 的条件（`:75`）加 `&& !navigating.current`。
- **测试**（vitest，`W/pages/notebook/page-tree-writes.test.tsx`，两格）：
  - 准备：`renderApp(pagePath(linux.id), server.app, { routes: [...routes, { path: "/elsewhere", loader: () => gate.promise, Component: … }] })`：导航到它时一直是 `"loading"`，直到 `gate.open()`。
  - `"a page deleted while the reader goes elsewhere leaves them going there: its shell does not send them to its parent"`：`server.deletesHeld = true`；树的菜单删除 Install 并确认；等 `"DELETE Install"`；`void act(() => router.navigate("/elsewhere"))`；`server.release()`；等删除之后的 `GET nodes`；`gate.open()`；地址是 `/elsewhere`。没修的代码上：`<Navigate replace>` 取消了挂起的导航，地址停在 `pagePath(guide.id)`，`waitFor` 超时。
  - `"a page created while the reader goes elsewhere leaves them going there"`：`server.createsHeld = true`；按树上的"New page"；导航到 `/elsewhere`；放开创建；`gate.open()`；地址是 `/elsewhere`。没修的代码上：挂起的导航还没提交，`at.current === from`，跳到新页，同一个断言失败。
  - 对照：`:269`、`:61` 两格照旧通过。
- **变异**：
  - `PageGone` 去掉 `navigation.state` 的判断：删除那一格失败；`useNewPage` 去掉 `!navigating.current`：新建那一格失败。
  - `navigating` 在渲染里赋值：等价，不列（推断：`create` 只在答复之后读它，那时最后一次渲染已提交，两处的值相同；写在 effect 里只为不在渲染里改 ref）。
- **假服务端**：`W/test/page-server.ts` 加 `deletesHeld: false`（同 `restoresHeld`，`:212-213`）；`"DELETE /api/v0/nodes/*"`（`:648`）改成 `async`，`server.sent.push` 之后 `if (server.deletesHeld) await new Promise<void>((resolve) => server.held.push(resolve))`；`release()` 的注释（`:248`）加上 deletions。
- **牵连**：推断 `useNavigation().state` 在路由级 `lazy` 的分包加载时是 `"loading"`（审查 C 读代码核对过 `createBrowserRouter`，`main.tsx:38`，与路由级 `lazy`，`app/routes.tsx:49` 起）；同一种路由第二次去时没有窗口。`PageGone` 也给版本页用（`version-page.tsx:61`）。与 27.2 同改 `page-layout.tsx`，无语义冲突。

##### 12.12 页面树的 `Math.max(...)` 与 `push(...)` 的参数上限

- **现状**：
  - `W/stores/page-tree.ts:115-117`：`subtreeOf` 每一层 `pages.push(...childrenOf(tree, …))`；`:122-125`：`heightOf` 是 `Math.max(0, ...subtreeOf(tree, id).map(...))`。
  - 参数超过引擎的上限时抛 `RangeError`。Node 24.21 里 `Math.max(...)` 的上限是 125,154 个参数，`push(...)` 是 125,146（审查 C 的 `m12/p2-review-c/spread.mjs`）；浏览器里随栈的大小，没有量。
  - 怎样碰到：树的每一行的菜单渲染调 `subtreeOf`（`W/pages/notebook/page-tree-item.tsx:274`），一个父页下孩子超过上限时左栏落到错误边界；删除时 `remove` 调它（`page-tree.store.ts:370`），抛错让写被当作失败，外壳显示 404 而不是去父页；`canHold`（`page-drag.ts:28`、`move-page-dialog.tsx:120`）对超过上限的子树抛错。
  - 一个导入至多 6 万个节点，经 API、MCP 可以更多；M10 起按一本 10 万页设计（I2 第 2.3 节）。
- **改法**（`page-tree.ts`）：
  - `subtreeOf`：外层照旧按下标走 `pages`，环的守卫 `pages.length <= tree.byId.size` 照旧每个父页判一次；孩子用 `for … of` 一个个 `push`。
  - `heightOf`：`subtreeOf(tree, id).reduce((height, page) => Math.max(height, depthOf(tree, page.id) - top + 1), 0)`，`top = depthOf(tree, id)`。
  - 注释照 `W/pages/page/page-outline.tsx:37` 的先例："One at a time, not spread into push: a call takes so many arguments only (v0.1 design 13.2, item 25)."
- **测试**（vitest，`W/stores/page-tree.test.ts`）`"a page with more children than a call takes arguments has them all in its subtree, and its height: none is spread into a call"`：
  - 规模：测试里 `spreadLimit()` 从 2^16 起翻倍，直到 `Math.max(...new Array(n).fill(0))` 抛 `RangeError`，答这个 `n`；造 `many = spreadLimit() * 2` 个孩子。Node 24.21 上探得 131,072，测试用 262,144，两处的上限都过了（`spread.mjs`）。
  - 造法：不经 `indexTree`（每个标题算两次 NFC，几十万次太慢），直接造 `TreeIndex`：`byId` 是 Guide 与 `many` 个孩子（`{ ...linux, id: `c${i}`, parent_id: guide.id }`），`children` 是 `[[null, [guide]], [guide.id, children]]`，`titles` 空。
  - 断言 `subtreeOf(tree, guide.id).length === many + 1`、`heightOf(tree, guide.id) === 2`。没修的代码上 `subtreeOf` 抛 `RangeError`。代价推断约 0.2–0.5 秒，没量。
- **变异**：只还原 `push(...)`：`subtreeOf` 抛；只还原 `Math.max(0, ...)`：`heightOf` 抛（审查 C 在 Node 24.21 上核对过两处各自抛）。探出上限这一步本身证明规模过了上限（05-lessons 第 1 节）。
- **牵连**：分层加载与性能的附带观察见 4.6.6。

##### 21.2 同一句"没有改动"再说不播报

- **现状**：版本页的 `<output>{said}</output>`（`W/pages/page/version-page.tsx:162`），`said` 是字符串状态（`:103`），还原的确认框 `close(t("version.unchanged"))`（`W/pages/page/version-restore.tsx:138`）经它写进去。第二次还原同一个无差别的版本，文字相同，React 不改 DOM，播报区什么也不说；`version.gone`（`:158`）同样。附件一节的 `saying`（`W/pages/page/attachments-section.tsx:387-393`：同一句每隔一次加一个不换行空格）没用在这里。
- **改法**（与 21.10 一起，`version-page.tsx`）：
  - `type Said = { text: string; of: string; again: boolean }`：说的话、说时显示的版本 id、是不是重说。
  - `say(text)`：`setSaid((before) => ({ text, of: data.id, again: before?.of === data.id && before.text === text && !before.again }))`；`VersionRestore` 的 `said={setSaid}`（`:139`）改为 `said={say}`。
  - `<output>` 显示 `said?.of === data?.id ? said.text + (said.again ? 不换行空格 : "") : ""`。
  - 不把 `saying` 挪进 `lib/`：S5 在改那个文件。两份同样的三行记进 v0.2 的收拢（C）。
- **测试**（vitest，`W/pages/page/version-restore.test.tsx`，en、zh-CN 各一格）`"a version the page holds already, restored again, says so again: what the line shows changes, which a screen reader hears"`：用 `restoring()`（`:93`）开对话框，内容设成与版本相同（`{ content: "Install\nthree\n", revision: 6 }`），按"Restore"，等那一句；对 `<output>` 挂 `MutationObserver`（`childList`、`characterData`、`subtree`）；再开、再按；变化次数 > 0，`textContent` 是原句加一个不换行空格；第三次回到原句。zh-CN 比"与当前相同，没有改动。"。
  - 没修的代码上：第二次之后没有一条记录，`toBeGreaterThan(0)` 失败。`getByText` 的默认规整去掉首尾的不换行空格，`:159` 照过。
- **变异**：`again` 恒为 `false`：本格失败。
- **牵连**：S2 若改 `version.unchanged`，整句随之改。

##### 21.3 "已还原到版本 N"留在路由状态

- **现状**：页的外壳每次渲染从 `useLocation().state.restored` 读（`W/pages/page/page-layout.tsx:91-95`、`:133`），显示在 `<output>` 里并描述 `h1`（`:258`、`:288-292`）。路由状态在这一条历史条目里：刷新、后退再前进都再说；进入编辑再回来也还在。22.2 同一个根因。
- **改法**：
  - `W/app/arrival.ts` 加 `export function useRouteNotice<T>(of: (state: unknown) => T | undefined, keys: readonly string[]): T | undefined`：答挂上时路由状态说的（这次挂载用），并把历史条目换成去掉 `keys` 的状态，刷新、后退再前进到它不再说（M5-polish 21.3、22.2）。
    - `const [notice] = useState(() => of(location.state))`；挂上时只跑一次的 effect（`useRef` 守住 StrictMode 的第二次）在 `notice !== undefined` 时 `navigate({ pathname, search, hash }, { replace: true, state: 去掉 keys 的 state })`。`arrived` 留在 state 里，`useArrivalFocus` 的 effect 不重跑。
    - 到达的焦点靠 S1 定的接续（4.1 的 28.4，2.3 的 P2-4）："同一路径与查询、只换 state 的 `replace` 不算新的导航，`useArrivalWatch` 的看守不重新起算，`offered` 接续"。这次 `replace` 正是这一种，所以这一页之后被删、"页面不存在"挂上时只从无人处取焦点（不回到 K1-1 那一类）。S6 不自己接续 `offered`。
  - `PageShell`：`const [restored, setRestored] = useState(useRouteNotice(restoredOf, ["restored"]))`；`enter()`（`:163-199`）里与 `setRefusal(undefined)` 一起 `setRestored(undefined)`：进入编辑之后不再说。
- **测试**（vitest）：
  - `W/pages/page/version-restore.test.tsx` `"the status of a version restored is said as the page shows, once: back and forward say it no more, nor does the entry, which a reload reads; an edit entered takes it away"`：照 `:106` 还原到页；`"Restored version 4."` 在、`router.state.location.state` 等于 `{ arrived: true }`；后退、前进，先等 `h1` "Install"，再断言那一句不在。另一分支：按"Edit"进入编辑再离开，那一句不在。没修的代码上：state 是 `{ arrived: true, restored: 4 }`，先失败；后退再前进之后那一句又在。
  - `W/app/arrival.test.tsx` `"a page whose route notice was taken, deleted elsewhere later, gives way to Page not found, which takes the focus only from no one"`：到达带 `restored` 的页，焦点放到树的一个链接上，页被删（`server.trashNode` 加事件或重读），"Page not found"挂上，焦点仍在那个链接上。它钉住 28.4 的规则对这次 `replace` 成立。
- **变异**：不换掉条目：state 的断言失败；`enter` 里不清：编辑那一分支失败；28.4 的同路径 `replace` 规则去掉（4.1 的变异）：arrival 那一格也失败。
- **牵连**：
  - 与 S1 同改 `W/app/arrival.ts`（S1 的第 3 个提交）。21.3、22.2 靠 28.4 的接续，因此在 S1 之后合并：这是 0.1 的硬依赖。
  - `replace` 换了 `location.key`：`useNewPage`（`new-page.ts:36-44`）在挂上那一刻恰有新建在途时不再跳（推断，几乎碰不到）；阅读视图的锚点 effect（`reading-view.tsx:233-276`）在没有锚点的地址上什么也不做。

##### 21.7 "显示未改动的行"按了没用时仍显示

- **现状**：`Comparison` 在比较的分包载入之后就画出"Show unchanged lines"（`W/pages/page/comparison.tsx:59-61`），它只作用于统一视图的折叠（`W/editor/text-diff.tsx` 的 `collapseUnchanged`，`:137`）。四种情形按了不变：两边相同或只差换行（`:160-162`）、太大而并排（`:174-189`）、比较失败（`:170-172`）、还在比较（`:164-169`）。
- **改法**：
  - `text-diff.tsx`：`TextDiffProps` 加 `foldable?: (yes: boolean) => void`；在 `:151-153` 之后算 `unified = split.a !== split.b && compared?.result.kind === "changes" && state !== undefined && removedLines(state) <= lineDiffLimits.maxChangedLines`，`useEffect(() => foldable?.(unified), [foldable, unified])`；`shows()` 里 `:174` 的判断改用 `!unified`。
  - `comparison.tsx`：`const [foldable, setFoldable] = useState(false)`，传给 `TextDiff`；按钮只在 `foldable` 时画，加 `useFocusLeaving(button, () => root.current?.focus({ preventScroll: true }))`：比较换成"没有改动"时按钮带着焦点离开，焦点给这一组。
  - 冲突面板的同一个按钮（`W/pages/page/conflict-panel.tsx:45-47`）只差换行时同样没用：`ConflictDiff`（`W/editor/conflict-view.tsx`）透传 `foldable`。
  - 不做：统一视图里没有够长的未改动段（CodeMirror 只折叠至少 4 行的段，推断）时也不画。要读 `.cm-collapsedLines`（`conflict.test.tsx:150`），展开之后它就没了；记进 v0.2（C）。
- **测试**（vitest，`W/pages/page/version-page.test.tsx`，`test.each` 三行）`"Show unchanged lines is offered where the comparison folds: not for texts the same, nor those of their line breaks alone, nor side by side"`：内容与版本 4 相同（"No changes."）；换成 CRLF（"Only the line breaks or the byte order mark differ."）；并排：照 `version-restore.test.tsx:12-19` mock `../../lib/line-diff-client`，`compareInWorker` 答 `{ kind: "tooLarge", added: null, removed: null }`，内容大到 `compareNow` 不在主线程比（"Too large to compare line by line."）。各自选"Compare with the page now"，等那一句话，没有"Show unchanged lines"按钮。
  - 对照：`installed()` 的版本 4 与当前不同，按钮在，按下 `aria-pressed="true"`。
  - 没修的代码上：三种都找到按钮。
  - 冲突面板：`W/pages/page/conflict.test.tsx` 加只差换行的一格；`:144`（有折叠）照过。
  - 第二格 `"Show unchanged lines leaving with the focus hands it to its comparison"`：比较有改动、按钮取得焦点；页的当前内容在假服务端改成与版本 4 相同，发 `pages` 事件让它重读（推断：比较随当前内容重算）；"No changes." 出现，按钮不在，焦点在这一组。没修的代码上：焦点落到 `body`。
- **变异**：`unified` 恒为 `true`：第一格失败；去掉 `useFocusLeaving`：第二格失败。

##### 21.10 同一页的两个版本之间后退、前进留着说的话

- **现状**：`VersionShell` 按页挂载（`W/pages/page/version-page.tsx:63` 的 `key={page.id}`），`tab` 与 `said`（`:101-103`）在两个版本之间留着。在版本 4 上还原得到"没有改动"，后退到版本 6，那一句仍在，读来像在说版本 6。I3 另说标签页留着、"与上一版"的基版本被 `useLeftRevision`（`:257-275`）去掉再读、闪一下"加载中"（闪的一半是 C）。
- **改法**：21.2 的 `Said` 带 `of`，只在 `said.of === data?.id` 时显示。地址被换成版本自己的修订号（`:118-124`）或编辑会话的保存推进修订号时，版本 id 不变，照旧显示（推断：`newest.id !== data.id` 的用法如此）。标签页不改（2.3 的 P2-33，`version-page.test.tsx:227` 钉着它）。
- **测试**（vitest，`version-page.test.tsx`）`"back and forward between two versions of a page say nothing of what a restore of the other said"`：`renderApp([versionPath(install.id, 6), versionPath(install.id, 4)], server.app)`；内容设成与版本 4 相同；还原、等那一句；后退，先等 `h1` "Version 6"，再断言那一句不在；前进回到 4，同样不在。没修的代码上：后退到 6 之后那一句仍在。
- **变异**：`said` 不比 `of`：本格失败。

##### 22.2 回收站的状态行与"已并入"留在路由状态

- **现状**：回收站页挂上时从路由状态读"已恢复""已开始"（`W/pages/notebook/trash-page.tsx:93-94`，`saidOf`），由预览带过去（`W/pages/notebook/trash-preview.tsx:170-177`）；预览上"已并入这一项"由 `<Navigate replace … state={{ merged: true }}>`（`:205`）带来，`Preview` 每次渲染从 `useLocation().state` 读（`:286`、`:341`）。刷新、后退再前进都再说。同 21.3。
- **改法**（21.3 的 `useRouteNotice`）：
  - `trash-page.tsx:93-94`：`const [said, setSaid] = useState(useRouteNotice(saidOf, ["restored", "started"]))`。
  - `PreviewShell`（按节点 id 挂载，`:67`）里 `const merged = useRouteNotice(mergedOf, ["merged"]) ?? false`（`mergedOf` 答 `true | undefined`），作为 prop 传给 `Preview`；`Preview` 不再读 `useLocation()`。放在外壳：`Preview` 会在"已不在回收站"与本身之间切换时重新挂上，第二次就读不到了。
  - 到达的焦点同 21.3，靠 S1 定的接续（4.1 的 28.4，2.3 的 P2-4），S6 不自己接续 `offered`；所以 22.2 也在 S1 之后合并（0.1 的硬依赖）。
- **测试**（vitest，`W/pages/notebook/trash-preview.test.tsx`，两格）：
  - `"the trash arrived at from a restore in the preview says where it went once: forward to it again, it says nothing"`：照 `:247` 恢复，到回收站，状态行在、`router.state.location.state` 等于 `{ arrived: true }`；后退、前进，先等 `h1` "Trash"，状态行为空。没修的代码上又说一遍。
  - `"an item's preview gone to from a node deleted with it since says so once: back to it, it says nothing"`：照 `:148`、`:392` 准备合并，落到 Guide 的预览、那一句在；点面包屑去回收站，后退，先等 `h1` "Guide"，没有"What you opened was deleted again…"。没修的代码上又出现。
  - `:471` 两个 alert 的次序不变。
- **变异**：`useRouteNotice` 不换掉条目：两格都失败。`merged` 放回 `Preview` 里读：两格都照过，等价（推断：两者只在同一次访问里 `Preview` 重新挂上时不同，例如拒绝之后单项回来；没有格造这个情形）。
- **牵连**：22.19、22.14 也改 `PreviewShell`，次序见 6.6。

##### 22.14 与 23.10 的共同改法：在途的写与它的结果放进 store

两项是同一个改法（移交第 22、23 条，I3 第 9 节）。

- **store**（`W/stores/page-tree.store.ts`，在 `restores`（`:106`）旁）：
  ```ts
  private readonly restoring = new Map<string, Promise<NodeRestored>>();
  private readonly reverting = new Map<string, { answer: Promise<ChangesetReverted>; before: string | undefined }>();
  restoreOut(id: string): Promise<NodeRestored> | undefined
  revertOut(id: string): { answer: Promise<ChangesetReverted>; before: string | undefined } | undefined
  ```
  - 键：回收站项的根节点 id、变更集 id，都是服务端给的小写 id（23.19 先转）。值：`restore()`、`revert()` 返回的那个 promise；撤销另记发出时知道的最新的撤销 `before`。
  - 何时清：答复落定（成功或失败）时，在 `runInAction` 里只删自己那一项（`if (this.restoring.get(id) === answer) this.restoring.delete(id)`）；同一项排队的第二次覆盖成自己的，前一次落定时不删它。
  - 可观察：私有名单（`:142-155`）加 `"restoring" | "reverting"`，覆盖写 `observableShallow`（mobx 7）：表可观察，promise 不被转换。
  - 写法：`restore`（`:553-559`）与 `revert`（`:592-622`）的方法体改名为私有的 `restoreNow`、`revertNow`；公开的两个同步取得 promise、记进表、挂上 `answer.then(clear, clear)`（不留未处理的拒绝），再返回它。`revert(id, message?, before?)` 多一个可选参数，`RevertForm` 传 `changeset.reverted_by[0]`（`W/pages/notebook/revert-dialog.tsx:222` 已算出）。
- **表只在在途时有项**：结果只对答复到来时正显示着的页有意义；之后才挂上的页读到服务端的现状，这是对的。
- **记 `before`**：不记的话，重新挂上的页在答复丢失（500、断网）时只能用挂上时缓存里的 `reverted_by[0]`，可能把自己的撤销说成"没有撤销"。
- **原来那次挂载的后续照旧**：卸下的 `RevertForm.revert()` 与 `useTrashRestore` 的闭包在答复之后仍做它们的重读（SWR 的 `mutate` 不随组件卸下）；跟随的页不必再重读。

##### 22.14 后退再前进的预览不知道在途的恢复

- **现状**：预览的恢复状态只在组件里（`W/pages/notebook/trash-preview.tsx:122` 的 `restoring`；"恢复"的忙是 `RestoreButton` 自己的 `sending`，`W/pages/notebook/trash-row.tsx:401-402`）；store 的 `restores`（`page-tree.store.ts:106`）不可观察，也不给出在途的答复。
  - 在预览上按"恢复"，答复未到时后退再前进，新挂上的预览什么也不知道：200 之后单项重读答 409 `page.not_in_trash`，显示"它已不在回收站里"，没有状态行，焦点在 `body`；其间"恢复"不忙，再按排上第二个请求、被拒为"在这期间变了"。后退落在回收站的列表时，那一行的"恢复"同样不忙（移交没写，一起改）。
- **改法**：
  - `trash-row.tsx`：从 `useTrashRestore`（`:298-330`）抽出与预览共用的纯函数 `trashRestoredOf(restored: NodeRestored, pages: PageTreeStore, item: TrashItem | undefined): TrashRestored`；`RestoreButton`（`:386-434`）加 `busy?: boolean`，`aria-busy`、`aria-disabled` 取 `sending || busy`，`busy` 时不发；`TrashRow`（`:485`，observer）传 `busy={pages.restoreOut(item.id) !== undefined}`。
  - `PreviewShell`：
    - `useState<Restoring>(() => (pages.restoreOut(id) === undefined ? undefined : { out: true }))`：挂上时有在途的恢复，第一次渲染起就是"在途"（单项读成"已不在"也照旧显示它，`:196-198`）。
    - 只依赖 `[pages, id]` 的 effect 跟随 `pages.restoreOut(id)`：成功 `setRestoring({ restored: trashRestoredOf(answer, pages, latest.current) })`，失败 `setRestoring({ refused, item: latest.current })`（`latest` 记最近一次 `data`）；cleanup 置 `showing = false`。原来那次挂载挂上时表里还没有这一项，不会处理两次。
    - 预览的"恢复"传 `busy={restoring !== undefined && "out" in restoring}`。
  - 之后与没卸下时一样：成功 `<Navigate to=回收站 state={{ ...arrived, restored }}>`（`:170-174`）；被拒则警示在上（22.19 之后在面包屑之下）。
  - 不做：列表在答复到来时也说"已恢复……"（列表没发出它）；预览"在后台恢复"的开始有同样的窗口（`trash-preview.tsx:124`），进 v0.2（2.3 的 P2-41）。
- **测试**（vitest）：
  - `trash-preview.test.tsx`：
    1. `"a restore from the preview out as the reader goes back and forward is followed by the preview shown again: its Restore busy, pressed it sends nothing; answered, the trash says where the item went, its title taking the focus"`：`trashed()`，`server.restoresHeld = true`，`renderApp([trashPath, previewPath(install.id)], server.app)`；点"Restore “Install” (From: Guide)"；等 `"RESTORE Install"`；后退、等列表、前进；按钮 `aria-busy`、`aria-disabled` 为 `"true"`；再点它；`server.release()`；`h1` "Trash" 出现、地址是 `trashPath`、焦点在它上；状态行是 `"Restored “Install” under “Guide”. Open"`；`RESTORE Install` 恰一次。没修的代码上：先失败的是 `aria-busy` 为 `null`；其后地址停在预览、没有状态行、两个 RESTORE。
    2. `"a restore from the preview refused as the reader went back and forward is said on the preview shown again"`：后退之前 `item.history = 400_001`；放开之后 alert 是"“Install” is too large to restore here: …"，出现"在后台恢复"。
    3. `"a preview shown after its restore answered follows nothing: it says the item is no longer in the trash"`：扣住、恢复、后退、放开、等列表重读，再前进：说"This is no longer in the trash."，地址仍是预览。
  - `trash-page.test.tsx`：`"back on the trash's list as a restore from the preview is out, the item's Restore is busy, pressed it sends nothing; answered, the item leaves the list"`。没修的代码上 `aria-busy` 为 `null`。
  - `W/stores/page-tree.store.test.ts`：`"a restore out is the node's until it settles either way: restoreOut answers its answer, then none; a second one out after it is the node's"`（照 `:778`，`held()` 扣住）。
- **变异**：`restore()` 不记进表：预览第 1、2 格失败；不删表项：预览第 3 格失败（跟随已落定的 promise，立刻跳到回收站）；跟随只处理成功：预览第 2 格失败；列表的行不读表：列表那一格失败；清理不比 `=== answer`：store 那一格的"第二个"失败。
- **假服务端**：已有 `restoresHeld`（`W/test/page-server.ts:212-213`、`W/test/trash-server.ts:260-262`）。
- **牵连**：`trash-row.tsx` 另有三处在改：P3/S3 的 `offeredOf`（`:172-177`，决定 377）与 `refusedText`，S2 的 `:102`、`:104`，S3 的 26.12（`:83-106`、`:260-279`）。文本冲突，语义不冲突（决定 377 的按钮是"在后台恢复"，本项的是"恢复"）；后合并的先 `git merge main`。

##### 22.19 `not_trash_root` 说三遍、焦点掉 body、alert 在面包屑之上

- **现状**（`W/pages/notebook/trash-preview.tsx`）：
  1. **说三遍**：预览上恢复被拒为 `page.not_trash_root`（这一项随它上面的页又被删了）：先显示拒绝的 alert"“X” was deleted again with a page above it: …"（`trash-row.tsx:133-134`，经外壳的 `Alert`，`:185-187`）；`useTrashRestore` 的重读又答 `not_trash_root`，外壳 `<Navigate replace>` 到根的预览（`:202-208`）；那里 `Alert` 说"What you opened was deleted again…"（`:341`），`h1` 取得焦点、经 `aria-describedby` 再读一次（`:313`）。推断（I3 也是推断）：第一句的 alert 至少出现一帧，读屏多半念出。
  2. **焦点掉 body**：说明页 `Gone`（`:440-476`）没有 `useFocusLeaving`：它的 `h1` 持有焦点时单项又回到回收站，焦点落到 `body`；`Preview` 只在 `merged` 时取焦点（`:290-295`）。
  3. **alert 在面包屑之上**：外壳把拒绝的 `Alert` 放在视图之前（`:130-135`），面包屑在 `Preview` 里（`:302-308`）。
- **改法**：
  1. `PreviewShell.restoreItem` 的 `catch`（`:145-149`）：拒绝是 `page.not_trash_root` 且带 `root_id` 时，设 `{ merged: root }`（`Restoring` 加这一种）而不设 `{ refused }`，渲染 `<Navigate replace to={`${home}/trash/${root}`} state={{ merged: true }}>`（同 `:205`）。剩下"活的一次、取得焦点的标题经描述一次"（移交第 22 条要两者）。
  2. `Gone` 加 `left` prop 与 `useFocusLeaving(section, left)`；`PreviewShell` 给 `NotInTrash`、`Gone` 传 `left={() => setHanded(true)}`；`Preview` 加 `focus` prop，effect 改为 `(merged || focus) && (at === null || at === document.body)` 时 `h1` 取得焦点（同 `Gone`，`:456-461`）；`PreviewShell` 传 `focus={handed}`。`handed` 一直为真也无妨：只从无人处取（13.2 第 26 条）。
  3. 面包屑提到外壳：`shell(told, view, trail?)` 依次渲染 `{trail}`、`{told !== undefined && <Alert id={note}>…}`、`{view}` 三个固定的槽；`Preview` 不再画 `BreadcrumbTrail`。`Alert` 一直在第二个槽里，`Preview` 与说明页互换时不重新挂上（`:129` 要的"同一个 alert"保住，N2 的修复不回退）。
- **测试**（vitest，`trash-preview.test.tsx`）：
  - `"a restore from the preview refused as deleted again with a page above it goes to that item's preview, which says why once"`：照 `:392` 准备（`restoreElsewhere(install.id)`、`trashNode(guide.id)`），用 `:508` 的 `MutationObserver` 记下插入的 `role=alert`；`server.trashWaits.item` 扣住单项的重读；点"恢复"；等 `h1` "Guide"；放开；`inserted` 恰是 `["What you opened was deleted again with a page above it: it is part of this item now, and is restored with it."]`。没修的代码上第一项是"“Install” was deleted again with a page above it: …"。
  - `"an item back in the trash, its notice holding the focus, takes it to its title"`：照 `:543`，别处恢复让说明页出现并取得焦点，再 `trashNode(install.id)` 让单项回来；焦点是 `h1` "Install"。没修的代码上是 `body`。
  - `"a refusal on the preview is said below its breadcrumb"`：照 `:313`（太大被拒），`crumbs.compareDocumentPosition(alert) & Node.DOCUMENT_POSITION_FOLLOWING` 不为 0。没修的代码上为 0。
  - 已有的 `:392`、`:508` 照过；`:471` 两个 alert 的次序照旧。
- **变异**：拒绝 `not_trash_root` 照旧先显示：第一格失败；`Gone` 不加 `useFocusLeaving`：第二格失败；面包屑留在 `Preview` 里：第三格失败；`Alert` 不在固定的槽（例如在 `trail` 之前条件插入）：`:508` 失败（重新挂上、插入两次）。

##### 23.10 后退再前进的变更集页不知道在途的撤销

- **现状**：撤销的发送状态在确认框里（`W/pages/notebook/revert-dialog.tsx:87-88` 的 `open`、`sending`），页的 `said`、`refused`、`attempt` 在 `ChangesetShell` 里（`W/pages/notebook/changeset-page.tsx:97-103`）；store 的 `revert`（`page-tree.store.ts:592-602`）只是按 id 排队。确认框里按"撤销"，答复未到时后退再前进，重新挂上的页没有状态行，"撤销这次改动"不忙；再按、再确认，排在第一个之后，得到冲突。
- **改法**：
  - store：见共同改法。`RevertForm.revert`（`:226`）改为 `pages.revert(changeset.id, text, before)`。
  - 跟随在 `RevertDialog`（`:73-143`，它有触发按钮、`heading`、`sent`、`done`），包成 `observer`：
    - `out = pages.revertOut(changeset.id)` 在时，触发按钮显示 `t("history.reverting")`，`aria-busy`、`aria-disabled`；`onOpenChange` 在它忙时不打开（`if (!sending && !(next && out !== undefined)) setOpen(next)`）。
    - 依赖 `[pages, changeset.id]` 的 effect：挂上时有 `out`、且这次挂载自己没在发时跟随，次序：
      1. `sent()`：页的 `attempt` 加一、清掉之前说的；
      2. `reader = watchReader({ on: trigger.current })`：再按忙的按钮不算动作；
      3. 答复：`changed` 且有 `changeset_id` 时 `reverted`，否则 `unchanged`，给焦点与否看 `changed`；失败：`lost(error)` 时 `{ kind: "unknown", before: out.before }`，否则 `refused`，都给焦点；
      4. 这次挂载仍在时 `done(结果)`；要给焦点、`!reader.acted()`、焦点在 `null`、`body`、触发按钮或 `heading` 上时 `heading` 取得焦点；`reader.end()`。cleanup 置 `showing = false`、`reader.end()`。
    - 框里会留在框内的拒绝（冲突、转后台、有人在编辑、忙、说明的 422）在重新挂上的页上一律当作 `refused`，页的 alert 以 `sourceHistoryText ?? errorText` 说（`changeset-page.tsx:246-248`）；再按"撤销这次改动"就看到框里的细节。
- **测试**（vitest，`W/pages/notebook/revert-dialog.test.tsx`）：
  1. `"an undo out as the reader goes back and forward is followed by the changeset's page shown again: Undo change busy, opening nothing; answered, the line says it was undone, the title taking the focus"`：`reworked()`，`server.revertsHeld = true`，`renderApp([historyPath, changesetPath(rework.id)], server.app)`；`opened(user)`、按"Undo"；等 `["REVERT rework"]`；后退、等历史列表、前进；按钮"Undoing…"`aria-busy`、`aria-disabled` 为 `"true"`；点它，没有 `alertdialog`；`server.release()`；状态行是 `"Undone. See the undo"`，焦点在 `title()`，它由状态行描述；`REVERT rework` 恰一次；按钮回到"Undo change"。zh-CN 一格比"正在撤销…"与"已撤销。查看这次撤销"（照 S2 的定稿）。没修的代码上：找"Undoing…"超时。
  2. `"an undo refused as the reader went back and forward says why on the changeset's page shown again"`：扣住时 `rework.trimmed = true`；放开之后 alert 是 trimmed 的说明、焦点在标题。
  3. `"an undo whose answer was lost as the reader went back and forward finds out by the newest undo known as it was sent"`：撤销的路由用 `answers` 覆盖，等一个 gate、写下撤销、答 500；前进之后放开；答复之前一个 `pages` 事件让单个变更集重读；状态行是"Undone. See the undo"。
  4. `"a changeset's page shown after its undo answered follows nothing: Undo change opens the dialog"`。
  5. `"… the reader scrolling meanwhile keeps the focus where it is"`：跟随期间滚动，答复之后焦点不动（照 `:551`）。
  - `page-tree.store.test.ts`：`"a revert out is the changeset's until it settles either way, with the newest revert its sender knew"`（照 `:837`）。
  - `:521`（"Undo pressed twice sends one undo"）只在框内找按钮，照过（推断）。
- **变异**：不记表：第 1 格失败；不删表项：第 4 格失败；跟随不判 `lost`、`before` 用挂上时的缓存：第 3 格失败；焦点不看 `watchReader`：第 5 格失败。
- **假服务端**：已有 `revertsHeld`（`W/test/history-server.ts:434-436`）；第 3 格要 `revertsHeldAfterWrite`：写下撤销之后再扣住答复。
- **牵连**：23.19 先合并；24.9 改同一个 `done()`；后台的撤销（`transfers.startRevert`）有同样的窗口，同 22.14 不做的那一处。

##### 23.19 地址里大写的变更集 id

- **现状**：`ChangesetPage` 只核对形状（`idPattern` 带 `i`，`W/lib/ids.ts:7`），把原样的 id 交给外壳（`W/pages/notebook/changeset-page.tsx:65-69`），SWR 的键是大写的（`:94`、`:390`）。撤销之后确认框按服务端的小写 id 重读（`W/pages/notebook/revert-dialog.tsx:211-213`），碰不到挂着的大写键，"被谁撤销"与明细要等撤销自己的 `pages` 事件稍后重读。历史页与搜索页已经转小写（`history-page.tsx:32-34`、`search-page.tsx:84`）。
  - 回收站预览同样不转（`W/pages/notebook/trash-preview.tsx:63-67`），`NotInTrash` 按原样找节点（`:416`），大写地址下已恢复的节点读完树之后被说成"页面不存在"（推断）。一起改（2.3 的 P2-35）；22.14 的表按小写 id 记。
- **改法**：`changeset-page.tsx:65-69`：`const id = changesetId.toLowerCase();`（注释同 `search-page.tsx:83`），`<ChangesetShell key={id} … id={id}>`；`trash-preview.tsx:63-67` 同样转 `nodeId`。
- **测试**（vitest）：
  - `revert-dialog.test.tsx`：`"an undo on a changeset's page whose address has its id in capitals reads the changeset and its nodes again as the dialog closes"`：`renderApp(changesetPath(rework.id.toUpperCase()), server.app)`（没有事件流）；撤销；等"Undone. See the undo"；`server.sent` 里有 `"GET changeset rework"` 与 `"GET changes rework"`，页上出现"Undone by"那一行。没修的代码上没有这两次读，那一行不出现。
  - `trash-preview.test.tsx`：`"a node's preview whose address has its id in capitals, restored elsewhere, leads to it"`：`server.restoreElsewhere(install.id)`；`renderApp(previewPath(install.id.toUpperCase()))`；"This is no longer in the trash."与"Open it"在，没有"Page not found"。没修的代码上读完树之后换成"Page not found"。
- **变异**：去掉 `toLowerCase()`：两格各自失败。
- **假服务端**：`W/test/history-server.ts:272-274` 的 `changesetOf`、`labelOf` 改用已有的 `sameId`（`:225-227`），像真的服务端一样不分大小写（推断：它按 UUID 解析）；`W/test/trash-server.ts` 的 `found`（`:206-214`）与 `nameOf` 同样。

##### 24.2 后台的 422 之后"在后台撤销"仍可按

- **现状**：确认框转到后台之后，开始被拒为 422 `page.revert_too_large`（超过 `MaxJobItems`），横幅说"This change is too large to undo, even in the background."（`W/pages/notebook/revert-dialog.tsx:192-193`，经 `useForm` 的 `explain`），"在后台撤销"照旧可按（`:349-353`），再按得到同样的答复（M8/P4B 审查的偏离 5）。I3 说可算 C；不改判：永远不会成功的按钮是误导。
- **改法**（`RevertForm`）：
  - `const [hopeless, setHopeless] = useState(false)`；`start()` 的 `catch`（`:263-269`）里，`refused` 之后、`throw error` 之前：`ApiError` 且 `code === "page.revert_too_large"` 时 `setHopeless(true)`（横幅照旧由 `useForm` 显示）。
  - `hopeless` 时不画确认按钮，只留"取消"。确认按钮加 `useFocusLeaving(confirm, () => cancel.current?.focus())`（"取消"的 `Button` 加 `ref={cancel}`）。
- **焦点的去处**（推断）：`setHopeless` 与 `useForm` 的 `setFailures` 落在同一次提交里；确认按钮卸下时，`useFocusLeaving` 在布局 effect 的清理里把焦点给"取消"（`W/lib/focus-leaving.ts:15-22`），早于 `useFocusOnInvalid` 的被动 effect（`W/components/form-field.tsx:89-97`）。S1 的规则是"拒绝到达时焦点已丢才给横幅"（4.1 的 9.1），那时焦点在"取消"上，没丢，不动。所以最终在"取消"。两者哪个先发生取决于 effect 的次序：实施时先在 S1 合并之后的树上探出，再写下面的焦点断言。
- **测试**（vitest，`W/pages/notebook/revert-background.test.tsx`）`"an undo in the background refused as too large for the background too says so in the dialog, which offers only Cancel"`：照 `:138-196` 准备（`rework.jobTooLarge = true`）；按"Undo in the background"；alert 的整句、框里没有"Undo in the background"、焦点在"Cancel"（照上一段探出的去处）、`server.jobs` 为空。zh-CN 一格比"这次改动太大，后台也撤销不了。"。没修的代码上：按钮还在。
- **已有测试要随之改的**：`test.each`（`:138-166`）删去"a changeset too large for the background too"一行（`:153-159`：它断言焦点留在按钮上、再按就开始，与新行为相反）。
- **变异**：不设 `hopeless`：新格失败；不加 `useFocusLeaving`：焦点随按钮落到 `body`，S1 的规则把它给横幅，"取消"的断言失败（推断）。
- **牵连**：撤销的确认框不是 `ConfirmDialog`，S1 对它的改动不涉及这里。P3/S3 在同一张 `test.each` 里加"后台撤销答 `transfer.too_many_jobs`"一行：文本冲突，语义不冲突（那一行照旧留着按钮）；后合并的先 `git merge main`。

##### 24.8 `links` 事件重读已删页的反链

- **现状**（TB2 偶发的根源）：
  - `linksChanged`（`W/events/handlers.ts:130-148`）收到事件就经 refresher 重读反链（`:133`）、属性（`:134`）、版本的视图（`:135`）、阅读视图（`:136-147`）；目标多于 20 个（`MaxEventPages`，`server/internal/modules/linking/app/ports.go:317`）时 `targets: null`，重读这本笔记本的每一个反链的键（`readEach`，`:157-168`）。只有挂着的钩子真的读，所以问题只在"被删的页的面板还挂着时 links 事件已到"。
  - 删除的单元先发 `pages` 事件（`tree: true`，`server/internal/modules/page/domain/change.go:59-61`；被删的页不在 `pages` 里，`events/app/publisher.go:52-63`），再发 `links`：观察者的次序是 `pageEvents`、`links`、`assetObserver`（`server/internal/bootstrap/registrants.go:273`，审查 C 核对过），同一个事务里发布；依次送达是推断。`pagesChanged` 只请求一次树的读（至多每 500 ms，`:65-78`）；links 的读不等它。
  - 事件先于删除的答复到达时（负载重的 runner 上会），被删的页的外壳还挂着，它的反链、属性、阅读视图各读一次，答 404，Chromium 记"Failed to load resource"，安静页检查失败。TB2 (page) 15 次失败 5 次就是这个（M8/P4B 审查 F-1），当时改成在 Index 的页上删 Big 绕开（`e2e/stories/trash/tb2-background-restore.spec.ts:182-185`）。
  - 对用户无害；别人删掉你正在看的页时同样多出这几次读与 `NotLoaded` 的一闪（推断）。
- **改法**（2.3 的 P2-32）：
  - 不取移交的"跳过树里已没有的页的键"：处理函数手里只有 SWR 的缓存（`EventContext`，`:21`），store 自己的写之后的重读不经 SWR（`page-tree.store.ts:389`），缓存不反映 `removed`；事件先于答复时树里还有那一页（推断）。
  - links 事件的读排在同一个单元的 pages 事件要的那次树的读之后、React 显示之后：被删的页随树的读离开，面板卸下，之后的 `mutate` 碰不到它们。与 `refreshedOnConnect`（`W/app/event-stream.tsx:9-35`）、`pagesChanged` 读附件（`:69-75`）同一个原则。
  - `handlers.ts`：`EventContext` 加 `trees: Map<string, TreeRead>`，`type TreeRead = { done: Promise<void>; settle: () => void }`（按笔记本）。
    - `pagesChanged` 的 `tree` 分支：请求之前，这本笔记本没有项就建一个 deferred；refresher 的读函数**开始时**取出并删掉这一项（之后来的 pages 事件建新的一项、等新的读），读完 `await mutate(["pages", notebook]); await shown();` 之后在 `finally` 里 `settle()`，再照旧读附件。
    - `linksChanged` 的方法体包进 `afterTree(notebook, context, read)`：有项就 `void due.done.then(read)`，没有就立即 `read()`。
    - 这一项总会落定：`Refresher.request` 每次都把这个键的读换成最新的闭包，并且终会调一次（`W/events/refresher.ts:41-86`，审查 C 核对过），取出这一项的总是那一次真的读。两种例外都无害：藏起的标签页，树的读等可见时，links 的读随之等；流停了（`stop()` 丢掉待读的），这一代已结束。
  - `W/app/event-stream.tsx:63`：`context` 加 `trees: new Map()`（每代一个）。
  - 代价：同一单元的 links 的读晚至多 500 ms 加一次树的读，只在树变了的单元。
- **测试**：
  - 单元，`W/events/handlers.test.ts`：
    1. `"a links event after a pages event that changed the tree asks for what it names only once that tree is read and shown: a page the unit deleted has left first (M5-polish 24.8)"`：照 `:91`（refresher 的替身、假定时器）；先 pages（`tree: true`），再 links（`targets: ["a"]`，`["backlinks", "n1", "a"]` 在缓存里）；`requested` 里没有反链的键；跑树的读函数，三次 `await Promise.resolve()` 之后仍没有；`await vi.advanceTimersByTimeAsync(0)` 之后有。
    2. `"a pages event during the tree read has the links event after it wait for its own read"`。
  - 整合，`W/pages/page/page-backlinks.test.tsx`：`"a page deleted elsewhere as it shows reads none of its panels again as the deletion's links event names them: the tree, read first, takes it away"`：`eventServer()`，树的读按需扣住（照 `trash-preview.test.tsx:271-312`）；`renderApp(pagePath(install.id), withEvents(server.app), { eventHandlers })`，等"No page links here."、流连上、`hello()`、等 100 ms；`before = server.sent.length`；`server.trashNode(install.id)`；扣住下一次 `GET nodes`；发 `pages`（`tree: true, pages: []`）与 `links`（`pages: null, targets: null, aliases: false`）；`server.sent.slice(before)` 里没有以 `GET backlinks`、`GET properties`、`GET view` 开头的；放开；等"Page not found"；再断言一次。没修的代码上：links 一到，refresher 就发出 `GET backlinks <Install 的 id>` 等，第一次断言失败；树的读被扣住，这个次序是确定的。
  - e2e：TB2 (page) 改回在 Big 自己的页上删除，见 24.10。
  - 怎样证明 TB2 的偶发没了（不靠重复跑，05-lessons 第 6 节"把答复延后"）：
    1. 整合格确定地造出"事件先于答复、links 的读早于树的答复"，没修的代码上必红；
    2. TB2 (page) 把删除的答复扣到事件引起的树的读答了才放，没修的代码上每次都有 404。推断：links 帧与 pages 帧同一批刷出、同一个 JS 任务里依次处理，早于树的读的答复；没有核实服务端是否每帧单独 flush，即使分开也只隔微秒；
    3. 实施时在撤掉 24.8 修复的分支上经 `heavy.sh` 跑一次 TB2 (page)，看它红（这是变异，不是重复跑）。
- **已有测试要随之改的**：`handlers.test.ts:13-34` 的 `context()` 加 `trees: new Map()`。
- **变异**：去掉 `afterTree`：单元第 1 格与整合格失败；读开始前就 `settle`：整合格失败；读完才删表项：单元第 2 格失败。
- **牵连**：P4/S2 若改 `pagesChanged` 的树的读，同一处（4.6.6）。

##### 24.9 查到的一行与后一次的拒绝并存

- **现状**：第一次撤销的答复丢了，页说"不知道"并去查（`findOut`，`W/pages/notebook/changeset-page.tsx:130-159`）；查的读在途时用户又撤销一次（`sent()` 清掉状态行与拒绝，`:228-232`），查的结果在第二次的答复之前到，照 `saying()`（`:121-128`）仍说出；第二次被拒（trimmed、403、404），`done()` 只设 `refused`（`:182-184`），不清状态行。两句同时在页上，`h1` 由拒绝描述（`:208`）。I3 标推断，说几乎碰不到、可算 C。
- **改法**：`done()` 的 `case "refused"`：`setSaid(undefined); setRefused(reverted.error); return;`。
- **测试**（vitest，`revert-dialog.test.tsx`）`"a refusal of an undo clears what was found out of the one before it"`：撤销的路由用 `answers` 覆盖，第一次答 500，第二次等一个 gate 再答 409 `page.changeset_trimmed`；`server.historyWaits.changeset` 扣住 `findOut` 的读。撤销一次（页说"不知道"）；再撤销（第二次扣住）；放开 `findOut` 的读，等状态行变成它的结论；放开第二次；状态行为空、只有一个 alert（trimmed 的说明）、`h1` 由它描述。没修的代码上：状态行仍是第一次的结论。`revert-background.test.tsx:347`、`:379`（后台的那一路）照过。
- **变异**：不清 `said`：本格失败。

##### 24.10 TB2 的 `importBig` 断言与注释

- **现状**：注释已有（`e2e/stories/trash/tb2-background-restore.spec.ts:115`，`9b65c5c3`）。断言没有：`importBig`（`:68-80`）只取 `Big` 与 `Big/` 开头的路径，`expect(ids.size).toBe(nodes)` 只数 Big 自己的节点，库里多出别的节点也照过（M8/P4B 修复核对第二轮）。
- **改法**（与 24.8 同一个提交）：
  - `importBig(api, db, pat, notebookId, others: readonly string[])` 加 `expect([...(await idsOf(db, notebookId)).keys()].filter((path) => path !== "Big" && !path.startsWith("Big/"))).toEqual(others)`，注释"The notebook is Big's nodes and others: an import of more than Big/ would add some (P4B fix check 2)."。接口版本传 `[]`（`:130`），页面版本传 `["Index"]`（`:179`）。
  - TB2 (page) 改回"Big 自己的页开着时删除"，并扣住删除的答复：
    - `goto` Big 的页，等 `h1` "Big" 与"No page links here."（面板已读）；
    - `page.route(`**/api/v0/nodes/${big.id}`)`：不是 `DELETE` 就 `fallback`；是就 `route.fetch()`，`await treeRead`，再 `fulfill`。不 `unroute`（05-lessons 第 6 节）。
    - `treeRead` 在确认框打开之后、按"Delete"之前登记：`page.waitForResponse` 等 `GET /api/v0/notebooks/${notebook.id}/nodes`。答复扣住时 store 自己的写之后的重读不会发生，之后的第一次树的读只能来自事件。
    - 删除之后 `treeTitles` 为 `["Index"]`；外壳去笔记本首页（`removedTo` 为 `null`），首页的 `h1`（笔记本名"Plans"，推断）取得焦点，换掉原来的"Index 的标题可见"（`:196`）。
    - `:182-185` 的注释换成："Big deleted from the tree as its own page shows, its panels read: the deletion's links event names no page (past MaxEventPages). Its answer is held until the tree read its pages event asks for has answered, the events coming first as they may on a loaded runner: the panels of Big, gone with the tree, are not read again (M5-polish 24.8)."
    - 故事末尾的 `pageWatch.expectConsole({ errors: [failedToLoad(422)] })` 不变：没修的 24.8 会多出 404，在这里红。
  - 只 Chromium，`test.slow()` 照旧。
- **在没修的代码上怎样失败**：`importBig` 的新断言今天照过，它守的是以后；做变异：页面版本里多建一页（或传 `[]`），新断言失败。TB2 (page) 在撤掉 24.8 时红（见 24.8）。
- **变异**：`others` 不比：上面的变异照过，说明断言有用；`treeRead` 不等：没修的 24.8 上 TB2 (page) 不一定红（次序回到偶然），这是对"扣住"本身的对照。

##### 26.2 的前一半：框的本地问题在编辑之后仍在

- **范围**（移交第 26 条第二项）：前一半是框的本地问题（`Alert`）与 `aria-invalid` 在编辑之后留到下一次提交、空的回车在清掉它们之前就返回。后一半（以点击"搜索"提交时问题可能读两遍）只从 ARIA 推断，照负责人第 6 问移到发布之后的读屏验收。
- **现状**：`W/pages/workspace/search-box.tsx`：问题的状态 `:89-91`；`onSubmit` 空的就返回（`:111-113`），在 `setProblem(found)`（`:115`）之前；`onChange` 只改草稿（`:156`）；描述（`:131`）、`aria-invalid`（`:153`）与 `Alert`（`:163-168`）一直跟着 `problem`。输入过长的查询、回车，框下说"至多 256 字节……"；删短之后仍说、仍无效。
- **改法**：`onChange`（`:156`）改为 `(event) => { setDraft(event.target.value); setProblem(undefined); }`。空的回车那一路不另改：另改是等价的变异（推断：要清空框必先编辑，编辑时已清；没有测试分得开，05-lessons 第 1 节）。
- **测试**（vitest，`W/pages/workspace/search-box.test.tsx`）`"a problem under the box goes as the box is edited: the field no longer invalid, nor described by it"`：输入 86 个"数"、按"Search"，问题在；`fireEvent.change(field, { target: { value: "数" } })`；`aria-invalid` 为 `null`、`hintOf(field)` 为 `"Searches the whole workspace."`、表单里没有 alert。没修的代码上 `aria-invalid` 仍是 `"true"`。`:112` 照过（每次都先改再提交）。
- **变异**：去掉 `setProblem(undefined)`：本格失败。

##### 26.5 Windows 上关着的选择按方向键逐项触发 `change`

- **现状**：搜索页的"在哪里找"（`W/pages/search/search-page.tsx:216-233`）`onChange` 直接 `choose`（`:219`），`choose` 以 `replace` 换地址（`:172-180`），SWR 的键带 `location.key`，每换一次就读一次、状态行播一次。
  - 平台行为：Linux 上三引擎关着的 `<select>` 按 `ArrowDown` 每一步都派发 `input` 与 `change`，`Tab` 离开时只多一个 `blur`；Firefox 的 `change` 也先于 `blur`（审查 C 的探针 `m12/p2-review-c/probe8.txt`）。Windows 上没有核实（推断同样；只有 macOS 用方向键弹出列表）。
  - 键盘用户在"在哪里找"上按向下键看有哪些笔记本，每按一次就换一次地址、搜一次、播一次结果数。
  - 历史的三个筛选（`W/pages/notebook/history-filters.tsx:122-160`，每次都重读日志）与目录的"类型"（`index-filters.tsx:52`）同一个写法，一起改（2.3 的 P2-34）。
- **改法**：键盘在关着的选择上一项项换时，等读者离开它或按回车再用。
  - `W/components/ui/native-select.tsx` 加 `useChoice(applied: string, apply: (value: string) => void): { value, onKeyDown, onPointerDown, onChange, onBlur }`（M5-polish 26.5）：
    - 状态：`pending`（显示用，`value = pending ?? applied`：受控的选择不改状态的话 React 会把它改回去）与 `stepped`（ref）。
    - `onKeyDown`：不带 Alt 的 ArrowUp、ArrowDown、ArrowLeft、ArrowRight、Home、End、PageUp、PageDown，或可打印的单个字符（类型搜索）：`stepped = true`；Enter 且有 `pending`：用它。Alt+方向键、F4、空格（打开列表）不算。
    - `onPointerDown`：`stepped = false`。
    - `onChange`：`stepped` 时记进 `pending`，不用；否则（指针、打开的列表、脚本）清 `pending`、立即用。最后 `stepped = false`。
    - `onBlur`：有 `pending` 就用。`applied` 变了（别处换了地址）时清 `pending`。
    - 初稿为"`change` 晚于 `blur` 的浏览器"另加的兜底（焦点已离开就立即用）删掉：三引擎的 `change` 都先于 `blur`（`probe8.txt`）。
  - `search-page.tsx:216-233`：`const scope = useChoice(notebook ?? "", choose)`，`<NativeSelect id=… {...scope} className=…>`；历史的三个与目录的"类型"同样。
  - 不取：去抖（慢慢按照样逐项搜，指针选择也要等）；显式的"搜索"按钮（要新文字、改 SE2 与 `search-page.test.tsx:530-556`，并推翻 M8/P5 设计的"选择即换地址"）。
- **测试**：
  - vitest，`W/pages/search/search-page.test.tsx`：`"Where to look stepped through with the keys on the closed select searches once the reader leaves it, or presses Enter; nothing meanwhile"`：在 `:530` 的准备上，`fireEvent.keyDown(scope, { key: "ArrowDown" })`、`fireEvent.change(scope, { target: { value: ideas.id } })`（`probe8.txt` 的次序）；地址仍是 `?q=…`、没有新的搜索、`scope.value` 是 `ideas.id`；`fireEvent.keyDown(scope, { key: "Enter" })`（或 `fireEvent.blur(scope)`）；命中一条，地址带 `notebook_id`，搜索读了一次。没修的代码上：`change` 一到就换了地址。
  - vitest，历史与目录各一格（`W/pages/notebook/history-page.test.tsx` 的一个筛选、`index-page.test.tsx` 的"类型"）：同样的次序，`change` 之后不重读，Enter 之后读一次。没修的代码上 `change` 一到就重读。
  - e2e，`e2e/stories/search/se2-search-visibility.spec.ts` 在 `:217-224`（"Chosen as a reader of the page would"）之后（只 Chromium）：`scope.focus()`；`ArrowDown` 两次；`toHaveValue(shared.id)`；地址仍是 `/${workspace.slug}/search?q=${word}`；登记 `answerTo(page, "GET", 搜索的路径)`；`Tab`；搜索答 200；地址带 `notebook_id=${shared.id}`。注释："Stepped through with the keys on the closed select, as Chromium does off macOS: nothing is searched until the reader leaves it (M5-polish 26.5)."。变量名照该故事，搜索的路径照 `fixtures/wiki-search.ts`。没修的代码上：`toHaveValue` 过，只带 `q` 的地址那一步等到超时失败（`probe8.txt`）。
  - `user.selectOptions`（vitest）与 `selectOption`（Playwright，SE2 `:220`、`:252`）不发键盘事件，走"立即用"那一路（推断：user-event 14 的 `selectOptions` 不派发 keydown）。
- **变异**：`stepped` 恒为 `false`：vitest 的各格与 e2e 都失败；`onBlur` 不用 `pending`：e2e 的 Tab 那一步失败；历史或目录的某一处没换成 `useChoice`：那一处的格失败。
- **牵连**：S1 的 26.4 改同一个 `choose`（保留到达），以 S1 的为准；S3 的 32.x 在 `history-filters.tsx` 的同几个 `NativeSelect` 上加 `className` 与外层宽度：文本冲突。

##### 26.15 换工作区清框没有测试

- **现状**：`<SearchBox key={workspace.id} …>`（`W/pages/workspace/workspace-layout.tsx:115`）让换工作区时搜索框重新挂上，草稿与问题随之清掉；`search-box.test.tsx` 没有测试核对它（P5B 审查 C 的 C21）。只加测试。
- **测试**（vitest，`search-box.test.tsx`）`"another workspace's box is its own: what was typed in the last, and its problem, are not there"`：`renderApp("/lab", signedInApp({ "GET /api/v0/workspaces": () => json({ data: [acme, workspaceJSON] }) }))`（`acme` 照 `workspace-layout.test.tsx:10`）；输入 86 个"数"、按"Search"；`router.navigate("/acme")`；等表单换成"Search “Acme”"；框是空的、`aria-invalid` 为 `null`、没有 alert。今天的代码上它通过（D：补测试）。
- **变异**：去掉 `key={workspace.id}`：草稿仍在，本格失败；这是这一格存在的理由。

##### 26.18a 组合记号被命中切开

- **现状**：`partsOf`（`W/lib/text-ranges.ts:12-31`）照服务端的 UTF-16 区间切，不看字素。区间的一端落在一个字母与它的组合记号之间时，记号被分进下一段；命中的 `<mark>` 是 `font-semibold`（`W/pages/search/search-hit.tsx:33-35`），两段不在一起排版，孤立的记号多半画成带虚线圈的样子（推断，没在真的浏览器里看过）。例：印地文"किताब"里搜"कित"，区间止于 `त` 与 `ा` 之间。
- **改法**（两端都对齐，2.3 的 P2-39）：模块级 `const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" })`；`partsOf` 先求一次 `text` 的边界（各段的 `index` 加 `text.length`）；每个区间：`end` 移到不小于它的下一个边界，`start` 移到不大于它的上一个边界，再取 `Math.max(start, at)`（前一段已占到的位置），`end <= start` 的照旧跳过。
- **测试**（vitest，`W/lib/text-ranges.test.ts`）`"a hit that ends inside a cluster of a letter and its marks takes the whole cluster; one that begins in one, from its start; side by side, they do not overlap"`：
  - `hits("किताब", [{ start: 0, end: 3 }])` 为 `["[किता]", "ब"]`（`कि`、`ता` 都是"字母 + SpacingMark"，GB9a，各版本的 ICU 一致）；
  - `hits("e\u0301te", [{ start: 1, end: 3 }])` 为 `["[e\u0301t]", "e"]`；
  - 相邻：`hits("किताब", [{ start: 0, end: 1 }, { start: 1, end: 3 }])` 为 `["[कि]", "[ता]", "ब"]`。
  - 不用"स्ते"这类辅音 + virama + 辅音（GB9c，随 ICU 的版本不同，推断）。
  - 没修的代码上：第一句是 `["[कित]", "ाब"]`。
  - 组合记号写成 `\u0301` 转义，防工具把转义变成字符（05-lessons 第 8 节），或写字面的字符并加注释。
- **变异**：只移 `start` 不移 `end`：第一句失败；不取 `Math.max(start, at)`：相邻那一句重叠，失败。
- **人工**：在真的浏览器里看一次印地文、泰文的命中（发布之后的人工清单，或 S3 的对比度核对时顺手）。
- **牵连**：`Intl.Segmenter` 要 Firefox 125 以上（推断；项目已用 `toReversed`，要 Firefox 115 以上；没有写明支持的版本）；TS 的 `lib: ES2023`（`web/tsconfig.base.json`）有它的类型。

##### 27.2 过时的树先显示外壳，五个 404、四个 alert 一闪

- **现状**：
  - 读过一本笔记本的树之后离开它（去搜索的结果、工作区首页），笔记本的外壳卸下，`["pages", id]` 没有挂着的钩子，`pages` 事件不读（4.6.6），`PageTreeStore.nodes` 停在离开时。
  - 那一页在别处被删，再从搜索的结果（或后退、前进）回到它：`PageLayout` 用过时的树找到了页，画出外壳（`W/pages/page/page-layout.tsx:53-66`）；五个读（阅读视图、属性、反链、附件、编辑锁）答 404，控制台五条错误；四个面板的 `NotLoaded`（`W/app/not-loaded.tsx:12-25`）各插一个 `role="alert"`；约 20 ms 之后树的读（SWR 挂上时的读，有缓存时以 `rAF` 推迟，`swr@2.5.1` 的 `use-swr-*.mjs:584-592`）答复，换成"页面不存在"。读屏可能先念到四次 alert。
- **改法**：照 P4/S2 的结论（4.6.6）。保留整棵树时用甲'（2.3 的 P2-31）；改成分层加载时照 4.6.6 那一格另写。
  - **甲'**：笔记本的外壳卸下（从此事件不再读树）时把树标为未确认；重新挂上时，页的外壳先等一次挂上之后开始的树的读。后退、前进、任何从笔记本外进来的路都盖住；笔记本之内的导航不受影响。
  - 不取甲（移交的主推，点搜索结果时标记）：要动搜索的结果与根 store 的查找，后退、前进进来没有点击可挂。不取乙（面板对 `page.not_found` 不警示）：要改四个面板，五个 404 照样发出，且与 S4、S5 冲突。
  - `PageTreeStore`（`W/stores/page-tree.store.ts`）：
    - `watching`（私有：显示这棵树的笔记本外壳有几个）；`confirmFrom`（可观察：没人看着时 `Infinity`，再挂上时是第一次算数的读的序号，已确认时 `0`）；`get confirmed()`（`confirmFrom === 0`）；`confirming()`（`confirmFrom !== 0 && readsStarted >= confirmFrom`）；`watch(): () => void`。
    - `watch()`：`watching` 从 0 到 1 且 `confirmFrom === Infinity` 时 `confirmFrom = readsStarted + 1`。返回的函数：减到 0 且 `nodes !== undefined` 时 `confirmFrom = Infinity`。
    - `read()`（`:283-308`）：`confirms = confirmFrom !== 0 && read >= confirmFrom`；`runInAction` 的条件加 `|| confirms`，里面置 `confirmFrom = 0`。
    - 私有名单加 `"watching"`；改动都在 action 里（`enforceActions: "observed"` 的警告在 vitest 里算失败，13.2 第 9 条）。
    - 卸下时标、挂上时定起点：其间也可能有读（上传的 `wrote()`，`:402-430`），那些读之后的事件仍没人看。`Infinity` 让 `PageLayout` 从第一次渲染起就挡住。
  - `W/pages/notebook/notebook-layout.tsx`：`<NotebookContext>` 里（`:68`）、`<Outlet>` 之前加 `<TreeWatch notebook={notebook} />`，以 `useLayoutEffect(() => pages.watch(), [pages])` 登记、渲染 `null`：layout effect 先于 `PageLayout` 的被动 effect，起点先定下。
  - `PageLayout`（`:48-67`）与 `VersionPage`（`version-page.tsx:51-64`）：
    - effect `[pages, mutate]`：`!pages.confirmed && !pages.confirming()` 时 `void mutate()`。绑定的 `mutate()` 越过 SWR 的去重强制一次读；SWR 以 `rAF` 推迟的挂上时的读随之被去重（推断，读 `swr` 的源码）。
    - `pages.tree === undefined || !pages.confirmed` 时 `<NotLoaded error={error} retry={() => void mutate()} />`：读失败时说明并给"重试"，不停在加载中。放在 `found` 之前；钩子都在提前返回之前。
- **测试**：
  - vitest，`W/pages/page/page-layout.test.tsx`：
    1. `"a page deleted elsewhere while its notebook was left shows none of itself when gone back to: the tree is read first, and the page is not found"`：`pageServer()`；`renderApp(pagePath(notes.id))`，等 `h1` "Notes"；`router.navigate("/lab")`，等工作区首页；`server.trashNode(install.id)`（没有事件）；`before = server.sent.length`；`MutationObserver` 记插入的 `role=alert`；`router.navigate(pagePath(install.id), { state: arrived })`；`h1` "Page not found" 出现并取得焦点；`server.sent.slice(before)` 里没有匹配 `/^GET (view|properties|backlinks|assets)/` 的、插入的 alert 为 `[]`、恰一次 `GET nodes`。没修的代码上：有 `GET view undefined`、`GET properties <id>`、`GET backlinks <id>`、`GET assets …`，第一个断言失败；视图的 404 插入一个 alert。
    2. `"a page gone to within its notebook shows at once: its tree is not read again for it"`：在笔记本里从 Notes 点到 Guide，没有新的 `GET nodes`。
    3. `"a page gone back to after its notebook was left shows once its tree is read: a read that fails says why, with Try again"`：离开、`server.nodesDown = true`、回到页，`NotLoaded` 的 alert 与"Try again"；`nodesDown = false`、按"Try again"，页出现。
    4. `"a tree read sent before the notebook was left and answered after it was gone back to confirms nothing: the page waits for a read of its own"`：在 Install 上设 `server.nodesHeld = true`，发 `pages` 事件让树重读（扣住，答的是这时的树）；离开；`server.trashNode(install.id)`；回到 Install（新的读也扣住）；`server.held.shift()!()` 只放开旧的读；没有 `GET view Install`，页仍在等；`server.release()`，`h1` "Page not found"。没修的代码上（`read()` 不比起点）：旧的读确认了树，发出 `GET view Install`。
  - e2e，SE1 (page)（`e2e/stories/search/se1-search.spec.ts`），在 `:304-306`（再搜"检索"得 1 条、经接口删掉 Storage）之后、再搜之前：点还列着的"Storage"；`main` 里 `h1` "Page not found" 取得焦点；`page.goBack()`；`h1` "Search “检索”" 可见。注释："Its hit, still listed, opened: the notebook's tree, read before it was left, is read again first; the page is not found, none of its own reads made (M5-polish 27.2)."。没修的代码上：五条"Failed to load resource: 404"，故事末尾的安静页检查失败（故事没有 `pageWatch.expectConsole`）。
    - 要 e2e 的理由：jsdom 的假服务端对不存在的页的反链、属性、附件、锁仍答 200（`W/test/page-server.ts:338`、`:366-378`），"Failed to load resource"也只有真的浏览器记。
- **已有测试要随之改的**（推断，以全量 vitest 找出）：2 秒之内回到笔记本的格，今天 SWR 去重、不读树，改后多一次 `GET nodes`；数 `GET nodes` 的断言随之改。
- **变异**：
  - 卸下时不标、`PageLayout` 不挡：第 1 格失败；
  - 挂上时不定起点（`confirmFrom` 一直是 `Infinity`）：第 3 格按"Try again"之后页仍不出现；
  - 每次挂上都标未确认：第 2 格失败；
  - 挡了但不强制读：第 1 格停在加载中（推断：vitest 里离开又回来在 2 秒之内，SWR 去重挂上时的读）；
  - `read()` 让开始早于起点的读也确认：第 4 格失败。
- **假服务端**：`W/test/page-server.ts` 的 `GET nodes`（`:320-323`）加 `nodesHeld`：收到时取下当时的 `server.nodes`，扣住（同 `viewsHeld`，`:327-329`），放开后答取下的那份。别的不改：让反链、属性、附件、锁对没有的页答 404 会影响别的测试（推断），不在 S6 做。
- **牵连**：版本页同样挡；笔记本首页、目录、回收站、历史不挡（不读单页，过时的列表只一闪）；与 2 同改 `page-layout.tsx`；24.8 管"正看着时被删"，这里管"回来时"。

##### 35.3 `type` 超过 1024 字节时"重试"永不成功

- **现状**：目录的第一页读不到时一律 `NotLoaded`（`W/pages/notebook/index-page.tsx:277-278`）：地址里的 `type` 长于 1024 字节或含 NUL 时服务端答 422 `validation_failed`（`server/internal/modules/linking/app/list_index.go:172-181`），`NotLoaded` 说"有些值无效"并给"重试"，重试永远同样地失败；只有筛选里的"清除筛选"有用。历史页对这种情形有专门的说明（`history-page.tsx:160-166`、`:225-237`）。手写或截断的地址会碰到。I3 说可算 C；不改判（误导的"重试"）。
- **改法**（`IndexResults`）：`const refused = first === undefined && error instanceof ApiError && error.code === "validation_failed"`（注释"A filter the server does not take is read no better again: it says so, the filters' Clear filters above (35.3)."）；`:277` 改为 `first === undefined ? (refused ? <Alert>{t("index.badFilters")}</Alert> : <NotLoaded … />)`。不另放"清除筛选"：`IndexFilters` 已有。新键见 4.6.2。
- **测试**（vitest，`W/pages/notebook/index-page.test.tsx`，en 与 zh-CN 各一）`"a type the server does not take says the filters cannot be read, without Try again; Clear filters lists every page"`：`fixture.answer` 对带 `type` 的请求答 `problem(422, "validation_failed", { errors: [{ field: "type", code: "too_long", message: "" }] })`，否则 `json(fixture.value)`；`renderApp(`${indexPath}?type=${"x".repeat(1025)}`, fixture.server.app)`；alert 的整句、没有"Try again"；按"Clear filters"，列表出现，地址没有 `type`。没修的代码上：alert 是"Some values are not valid."且有"Try again"。`:181`（1024 字节，答 200）照过。
- **变异**：`refused` 恒为 `false`：本格失败。
- **牵连**：P3/S1 改 MCP 的 `index` 对空筛选的处理（30.4、30.5），网页从不发空的筛选，不受影响。

#### 4.6.8 与别的 Step 的牵连（汇总）

- **S1 焦点与共用组件**（S6 在它之后合并）：
  - `W/app/arrival.ts` 与 S1 同改（S1 的第 3 个提交）。21.3、22.2 靠 S1 定的接续（4.1 的 28.4，2.3 的 P2-4："同一路径与查询、只换 state 的 `replace` 不算新的导航，`useArrivalWatch` 的看守不重新起算，`offered` 接续"），S6 不自己接续 `offered`；所以 21.3、22.2 在 S1 之后合并，这是 0.1 的硬依赖。
  - 26.4 改搜索页的 `choose`（保留到达），28.5 让目录与历史的筛选带上路由状态（`index-page.tsx:56`、`history-page.tsx:134`），26.5 改它们的触发方式：S6 保留 S1 加的 `state: location.state`；
  - 9.1 的横幅焦点规则与 24.2 的 `useFocusLeaving` 一起决定 24.2 的最终焦点（24.2 先探）；
  - 23.14、24.3/49 与 23.10、23.19、24.2、24.9 改同两个文件（`changeset-page.tsx`、`revert-dialog.tsx`）；
  - `button.tsx` 的 `aria-disabled:` 样式让 22.14、23.10 的忙按钮看着不可按；
  - 2 的删除走 `ConfirmDialog`（S1 改成 `aria-disabled`），测试只点"Delete"，不受影响。
- **S2 文字**（S6 的整句断言在 S2 之后）：撤销的用词（23.1、23.4、23.11、24.7）与回收站、历史的说法，S6 的测试整句比对以 S2 的定稿为准；`version.unchanged`、`field.q.*`、`history.badFilters`（35.3 的新句照它）若改，随之；S2 改 `trash-row.tsx:102`、`:104`；26.11、26.13、26.14 与 S6 不重叠。
- **S3 375 px、`bdi` 与对比度**：
  - 26.12 改 `search-hit.tsx` 与回收站、历史的行（`trash-row.tsx:83-106`、`:260-279`）：26.18a 只改 `text-ranges.ts`，22.14 改 `trash-row.tsx` 的别处，22.19 不改文字的结构；26.9 改 `Marked` 的样式；
  - 26.17、32.x（`history-filters.tsx` 的 `NativeSelect` 与外层宽度、`changeset-page.tsx` 的"Client"一行）与 26.5、23.19、24.9 改同几个文件：文本冲突，语义不冲突；
  - 22.19 把面包屑提到预览的外壳：S3 若给预览的标题或面包屑加 `min-w-0`，以 S6 的结构为准。
- **S4 阅读视图**：27.2 的甲'不碰 `reading-view.tsx`。
- **S5 附件**：21.2 不挪 `attachments-section.tsx` 的 `saying`（S5 在改那个文件）；27.2 不碰附件一节。
- **P3**：与 P3/S3 同改的文件（P3 设计 4.2）：

  | 文件 | P3/S3 改的 | P2 改的 |
  |---|---|---|
  | `W/pages/notebook/trash-row.tsx` | `offeredOf`（`:172-177`，决定 377）、`refusedText` | S2 改 `:102`、`:104` 的说法；S3 的 26.12 改 `:83-106`、`:260-279`；S6 的 22.14 抽出 `trashRestoredOf`、改 `RestoreButton`、`TrashRow` |
  | `W/pages/notebook/transfer-report.tsx` | `problems` 的 `Record`（`:92-108`）与它的测试 | S1 的 19.3 改 `:203`、`:246` |
  | `W/pages/notebook/revert-background.test.tsx` 的 `test.each`（`:138-166`） | 加"后台撤销答 `transfer.too_many_jobs`"一行 | S6 的 24.2 删其中"too large for the background too"一行 |

  - P3/S3：上表几处，文本冲突，语义不冲突（决定 377 的按钮是后台恢复的那一个，22.14 的是"恢复"）；后合并的先 `git merge main`。
  - P3/S1、S2 无实质牵连：P3/S1 的变更集详情进快照不改答复的形状；P3/S2 的 302 只改回滚的变更集撤销时的冲突，网页照常显示。
- **P4**：S6 在 P4/S2 的结论之后开，结论要改树时在改法的 Step 之后开（4.6.6）；P4/S2 若改 `pagesChanged` 的树的读，与 24.8 同一处。

## 5. e2e

P2 的 e2e 改动：

- 两个新故事：PG15、AS6。
- 一个新的 Playwright 项目：`webkit-narrow`。
- 钉住项目配置的检查：`tools/e2e-projects/check.mjs`。
- 两个夹具：`E/fixtures/narrow.ts`、`E/fixtures/contrast.ts`。
- 改十几个已有的故事。

约定照旧：

- 换文档一律经 `goto`、`reload`（门禁 `tools/e2e-navigation/check.mjs`）。
- 新故事只有页面版本：查的是网页的行为，接口与 MCP 版本豁免（00 号文档 9.3）。
- 各故事的细节在它那一项里（4.x），本节是汇总。

### 5.1 配置

`e2e/playwright.config.ts` 在 P2 之后的样子：

- S3 加项目 `webkit-narrow`；
- S4 给 `threeBrowsers` 加 PG15；
- AS6 只在 Chromium，不进 `threeBrowsers`（P2-30）。

```ts
const threeBrowsers =
  /stories\/(wiki\/lw(3|4|8|9|10|12|13)-|identity\/a10-|identity\/a15-|workspace\/w5-|upgrade\/up1-|upgrade\/up7-|page\/pg15-)/;

  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", testMatch: threeBrowsers, use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", testMatch: threeBrowsers, use: { ...devices["Desktop Safari"] } },
    // The checks at a phone's width (WCAG 1.4.10) run on Safari's engine too, whose widths differ from Chromium's
    // (M12/P2/S3): those tagged @narrow, of the stories the three browsers do not run already.
    { name: "webkit-narrow", grep: /@narrow/, testIgnore: threeBrowsers, use: { ...devices["Desktop Safari"] } },
  ],
```

- **`webkit-narrow` 跑哪几格**：带 `@narrow` 标签、又不在 `threeBrowsers` 里的格，即 MC3、PG2 各一格；33.1 复现时 SE1 再加一格。LW3 (page) 已在三种浏览器里跑，它加的 375 一段不打标签。审查 B 在 Playwright 1.63 上核对过：`--list --project webkit-narrow` 只列带标签的格，`chromium` 照跑全部，`make e2e` 与 CI 不带 `--project`，新项目会跑。
- **钉住配置的检查**（P2-44，审查 C 的 M7）：配置写错时没有一格会红，所以 S3 加 `tools/e2e-projects/check.mjs`，由 `make lint-web` 跑。它进 S3 的第 3 个提交（PG2 的 `@narrow` 格在那里才有），同一个提交给 `knip.jsonc` 的 `entry` 加它、给 `Makefile` 的 `lint-web` 加一行（6.3；修订核对 B 的 M6）；S4 的第 6 个提交加 PG15 那一条。`web` 作业装着 e2e 包的依赖（`pnpm -r run check:types` 已在用），`--list` 不要浏览器。
  - 它以 `pnpm --dir e2e exec playwright test --list --reporter=json` 列出 `firefox`、`webkit-narrow` 两个项目的格，断言：
    - `firefox` 列出的文件含 `page/pg15-`（S4 起；S3 时这一条先不加）；
    - `webkit-narrow` 列出的每一格都带 `@narrow`、都不在 `threeBrowsers` 里，并且至少有 MC3、PG2 那两格。
  - 反向对照写进 Step 的记录：S3 把 `grep` 改坏、S4 从正则里删掉 `page\/pg15-`，各看它失败一次。

### 5.2 逐个故事

"多跑"是推断的 test-seconds，即每格多用的秒数之和，三种浏览器的按三遍算。它怎样折成墙钟，在 7.4。

| Step | 故事 | 新的或改的 | 查什么 | 浏览器 | 多跑 |
|---|---|---|---|---|---|
| S1 | A10 (page)（`identity/a10-api-tokens.spec.ts:130`） | 撤销一段之前加"被拒一次" | 23.16：键盘按"Revoke"，发送中 `aria-disabled` 且焦点在它上面，被拒之后仍在它上面 | 三种（已在） | 4–5 s |
| S1 | LW3 (page)（`wiki/lw3-source-duplicates.spec.ts:286`） | 粘贴被拒的一段改成键盘提交 | 9.1：被拒之后横幅取得焦点 | 三种（已在） | < 1.5 s |
| S1 | HI3（`history/hi3-log-revert.spec.ts:419-447`） | 加两条断言 | 23.17、28.2：发送中的按钮半透明 | Chromium | < 0.2 s |
| S1 | C6 (page, signed out elsewhere)（`collab/c6-expired.spec.ts:83`） | 先过"仍要退出登录吗" | 第 7 条：别的标签页报告有未保存的修改 | Chromium | < 0.5 s |
| S1 | PG11（`page/pg11-navigation.spec.ts:144`） | 加一行 | 12.3：点面包屑之后，笔记本首页的 `h1` 取得焦点 | Chromium | < 0.1 s |
| S1 | LW10 (page)（`wiki/lw10-virtual.spec.ts:293`；实际多改的，10.1） | 目录、历史的未建链接改按链接找，断言没有 `aria-haspopup` | 35.2 | 三种（已在） | 约 0.4 s（实测） |
| S2 | C4（`collab/c4-guard.spec.ts:236`、`:273`） | 改断言的文字 | 22.16 | Chromium | 0 |
| S2 | TB1（`trash/tb1-trash.spec.ts:316`、`:369-376`） | 改断言的文字，观察者加新句 | 22.11、22.13 | Chromium | 0 |
| S2 | LW6（`wiki/lw6-source-history.spec.ts:439`） | 否定断言换成不变的半句 | 38.8 | Chromium | 0 |
| S3 | MC3（`mcp/mc3-write-edit.spec.ts`） | 新的一格（`@narrow`）：agent 另写一页、只撤销它（审查 B 的 M1） | 26.17、32.1–32.3、35.8、38.12：128 个字符的客户端名在 375 下 | Chromium、WebKit | 约 14 s |
| S3 | PG2（`page/pg2-rename-page.spec.ts`） | 新的一格（`@narrow`） | 38.10、38.11：120 个字符的标题，页面与六个对话框 | Chromium、WebKit | 约 14 s |
| S3 | LW3 (page) | 加一段 | 38.10、38.11：来源页与两个对话框 | 三种（已在） | 约 7.5 s |
| S3 | SE1 (page)（`search/se1-search.spec.ts`） | 加一段，`:275` 改 | 12.4、26.9：两种主题下的焦点环、无效时的环、mark 的对比度与字重 | Chromium | 约 3 s |
| S3 | L5 (page)（`links/l5-completion.spec.ts`） | 加一段 | 12.4、P2-43：补全的说明与选中项的指示、编辑器与阅读视图的外框、一个菜单项的焦点指示 | Chromium | 约 2 s |
| S3 | `webkit-narrow` 的 worker | — | 建库、起服务与浏览器 | WebKit | 2–8 s |
| S3 | SE1 (page) 的 375 一段 | 只在 33.1 复现时拆出（`@narrow`） | 33.1 | Chromium、WebKit | 约 6 s |
| S4 | **PG15**（新，`page/pg15-reading-updates.spec.ts`） | 三格（8.1 复现时四格） | 4.4.6：就地更新留住焦点、callout、选区、播放；同名标题；全屏 | 三种（新加） | 54–90 s |
| S4 | UP1 (page)（`upgrade/up1-start-freeze.spec.ts:171`） | 加一段 | 48：横幅出现时文章的节点不换、段落不跳 | 三种（已在） | 约 3 s |
| S5 | **AS6**（新，`asset/as6-reading-links.spec.ts`） | 三格 | 17.2b 复制、17.4 过期的下载、15.2 callout 标题里的音频 | Chromium | 15–24 s |
| S5 | AS3 (page)（`asset/as3-render.spec.ts:297`） | 加一页、量盒子 | 15.1 | Chromium | 1–2 s |
| S6 | TB2 (page)（`trash/tb2-background-restore.spec.ts`） | 改回"在 Big 自己的页上删"，扣住删除的答复 | 24.8、24.10 | Chromium | 约 0.5 s |
| S6 | SE1 (page) | 加一段 | 27.2：过时的结果打开已删的页 | Chromium | 1–2 s |
| S6 | SE2 (page)（`search/se2-search-visibility.spec.ts`） | 加一段 | 26.5：方向键不搜，Tab 离开时搜 | Chromium | 约 1 s |

- **测试数**：577 个之后多 16 个。
  - MC3、PG2 各两个：Chromium 与 `webkit-narrow` 各一个。
  - PG15 三格，三种浏览器，共 9 个。
  - AS6 三格，只在 Chromium。
  - 另有条件的：33.1 复现时多 2 个，8.1 复现时多 3 个。
- **不进 e2e 的真浏览器行为**：几处 jsdom 断言不了、又不值得常驻套件的，各 Step 用临时的故事经 `heavy.sh` 跑一次，结果记进 Step 的记录，不提交。
  - S1：28.5 的后退前进、12.3 的点链接。
  - S4：8.1、WebKit 的滚动锚定、无头模式下的全屏。
  - S3：33.1 的排查。
  - S5：AS6 的三格在 Firefox 与 WebKit 上各跑一次，确认与审查 C、设计者的探针相同。

## 6. 实施步骤

### 6.0 每个 Step 共同的

1. **开分支**：从 `main` 开 `m12-p2-s<m>`。次序与并行照 0.1：同时实施的至多两个 Step，连同 P3–P5 一起算；只守 0.1 表里的硬依赖；S6 在 P4/S2 的结论之后开。
2. **先写测试，先看它红**：动手改代码之前，先写本 Step 的新测试，在没修的树上跑红，记进 Step 的记录；标"推断"的尤其要这样（05-lessons 第 1 节）。没红的测试不算数：先改测试，让它在没修的代码上失败，或照实写进记录。
3. **每个提交单独为绿**：
   - 网页：`make lint-web`（Markdown 样例、e2e 换文档的门禁、oxlint、格式、各包的 tsc）、`make knip` 与 vitest（`make test-web`）。
   - 服务端（S2 的模板、S4 的样例测试、S5 的 15.1）：另跑那个模块的 `go test` 与 `make lint-go`。
   - 这些都经 `docs/v0.1/claude_to_codex/tools/heavy.sh` 跑（06-environment 第 3 节）。
   - 改了故事的提交，跑那几个故事。
4. **哪些东西必须在同一个提交里**：
   - 同一个键的 en 与 zh-CN：`Messages` 要求两种语言的键相同，否则 tsc 红。
   - 新导出与它的第一个使用者：否则 knip 红。
   - 改了文字的 e2e 断言与那个键。
   - 新故事与 `threeBrowsers` 的正则。
5. **实施完**：
   - 经 `heavy.sh` 跑 `make check` 与 `make e2e`；S2、S4、S5 另跑 `make test`。
   - 照 7.2 做变异。
   - 照 02-workflow 交给三位审查者，修复。
   - 写文档提交（第 8 节）。
   - `git merge main`，再以 `--no-ff` 合并：P2 的 Step 一次合并一个，先做完的先合并（P2-1）。`main` 上 CI 的 19 个作业全绿之后，记下合并号。
6. **Step 的记录写什么**：
   - 与本文不同之处；
   - 探查的结果；
   - 变异的结果；
   - e2e 的用时：CI 的 `list` 报告里，这个 Step 改到、新加的那几格的秒数之和，与 5.2 的推断对照（7.4）；
   - 读屏清单要收的（第 9 节）；
   - 推后进 v0.2 的。

下面各 Step 的提交分组取自各自的逐项核对。提交说明照 docs/README.md 的 Git 一节，末尾写 `(M12/P2/S<m>)`。

### 6.1 S1

每个提交单独为绿（tsc、knip、vitest、oxlint；改了故事就跑那几个故事）。knip：新的导出都有人用（`focusLost`、`opened`、`useArrivalWatch`、`askOtherTabs`、`useSignOut`、导出的 `ChunkNotLoaded`）。

1. **`web, e2e: ConfirmDialog keeps the focus on confirm while it is out and after a refusal, and aria-disabled buttons look disabled (M12/P2/S1)`**：23.16 + 23.17 + 28.2（**必须同一个提交**：00 号文档 4.5，不然发送中的按钮看着能按）；`lib/focus-leaving.ts` 的 `focusLost`；`confirm-dialog.test.tsx`；`source-attachments.test.tsx:396` 的改动（实际另有 `security-page.test.tsx:185`，发送中的"撤销"原断言 `disabled`）；A10、HI3。
2. **`web, e2e: a refusal only the banner says takes the focus the form lost (M12/P2/S1)`**：9.1 + 38.3；用第 1 个提交的 `focusLost`（第 2 个先合时把 `focusLost` 放进它）；新 `form.test.tsx`；`page-tree-writes.test.tsx`、`source-add-dialog.test.tsx`；LW3。
3. **`web: arrival focus: a late title yields to the reader, Back and Forward to a page the tree opened, Not found in place of a page (M12/P2/S1)`**：28.4（连同同一地址只换 state 的 `replace`）、28.5、23.14（`arrival.ts`、`layout.tsx`、`page-tree-item.tsx`、`page-tree.tsx`、`not-found.tsx`、`changeset-page.tsx`）；`arrival.test.tsx`、`revert-dialog.test.tsx`。
4. **`web, e2e: links that leave with their page arrive, and Where to look keeps the arrival (M12/P2/S1)`**：12.3（连同工作区首页的笔记本卡片）、26.4；PG11 的一行。可与 3 合并。
5. **`web: an edit left with no Edit gives the focus to the title, or to Page not found (M12/P2/S1)`**：18.6；**在 3 之后**（"页面不存在"一半要 3 的 `replacing`）。
6. **`web, e2e: ask before signing out with changes not saved, this tab's or another's (M12/P2/S1)`**：7；新的 i18n 键；`page-lock.test.tsx` 的三格改动与 C6 **必须在这个提交里**（不然它们红）。
7. **`web: the undo dialog's description says all it asks (M12/P2/S1)`**：24.3 + 49（同一个改法，**同一个提交**）。
8. **`web, e2e: the reading view's task boxes are named for readers, and links to pages not there keep their state (M12/P2/S1)`**：6.3、11.7、35.2（可拆成三个）；实际另改 `virtual-navigation.test.tsx` 四格与 LW10 (page)（35.2 的嵌入与图片，10.1）。
9. **`web: a row gone with its menu, the conflict's Try again and a take-over leave the focus somewhere; a report's Try again names its job (M12/P2/S1)`**：16.1、21.14、22.17、19.3（可拆）。
10. **`web: a source's read-only note is no alert (M12/P2/S1)`**：38.1。
11. **`docs: … (M12/P2/S1)`**：总体设计 13.2 的改写（8.2）、Step 的记录。

实际（10.1）：第 1–10 个照上面的次序；之后修复第 1 轮四个（`52295438`、`1ebe1b16`、`793c062c`、`137356d6`）、第 2 轮一个（`2d707feb`）与一次合并 `main`（`01283f3f`），文档提交在最后。

`en.ts`、`zh-CN.ts` 只有第 6、9 个提交改。

### 6.2 S2

每个提交自己为绿（tsc、knip、vitest、oxlint；第 9 个是 wiki 模块的 go test）。同一个键的 en 与 zh-CN 在同一个提交（`Messages` 类型要求两种语言的键相同）；删键与它的用处同一个提交；改了英文文字的 e2e 断言与那个键同一个提交（分支的 CI 跑 e2e）。

1. **单复数**（35.1、41.1、41.2、41.3）：10 个 `*One` 键；`health-rule.tsx` 导出 `clearText`、`staleText`（各在两个文件里用，knip 不报），其余就地三元。
2. **"顶层"**（22.11、23.3、9.5）：9.5 的新键用 22.11 的词，同一个提交或在它之后；e2e `tb1-trash.spec.ts:316`。总体设计 13.2 第 17 条随之改（8.2），文档另一个提交亦可。
3. **撤销的用词**（23.1、23.4、24.7、23.11）：23.11 是行为，放这里因为它与 23.4 同改 `revert-dialog.tsx` 的冲突一段与 `revert-dialog.test.tsx`。
4. **回收站与历史**（22.12、22.13、23.12、23.13、H4）：e2e `tb1-trash.spec.ts:369-376` 与 22.13 同一个提交。
5. **页面、附件、导出与编辑器**（6.1、15.3、16.6、19.1、22.16）：含 `W/test/page-server.ts` 的替身；e2e `c4-guard.spec.ts:236,273` 与 22.16 同一个提交。
6. **比较**（21.8、21.11）。
7. **搜索**（26.11、26.13、26.14）。
8. **大小的单位与来源的说明**（38.6、38.8）：e2e `lw6-source-history.spec.ts:439` 与 38.8 同一个提交；38.6 改 10 个测试文件，可以单独一个提交（实际 11 个，与 38.8 同在第 8 个提交，10.2）。
9. **服务端**：两份 `AGENTS` 模板（40）与 `wiki/domain/template_test.go` 的新测试。组合根不变。

与别的 Step、Phase 的冲突（`en.ts`、`zh-CN.ts` 每个 Step 都改，后合并的先 `git merge main`）：

- P2 之内：`revert-dialog.tsx`（S1 的 24.3、49 改说明的 `aria-describedby`）；`trash-preview.tsx`、`history-filters.tsx`、`search-page.tsx`、`search-status.ts`（S6 的 22.14、26.x，历史的超宽在 S3）；`asset-dialogs.tsx`（S5）；`page-layout.tsx`、`reading-view.tsx`（S4 的 6.2、48）；`export-dialog.tsx` 用 `ConfirmDialog`（S1 改它的内部，不改 props）。
- P3/S3：`transfer-report.tsx` 与它的测试（23.1、24.7）、`export-dialog.tsx` 与 `page-menu.test.tsx`（19.1）、`trash-row.tsx` 显示的文字（22.11）、`revert-background.test.tsx`、`transfer-import.test.tsx`、`problem-messages.test.ts`（23.1）：文本冲突、语义不冲突，各见那一项的牵连。

### 6.3 S3

1. **`web: a name in a line of other text is written in a bdi (26.12, 32.4)`**：
   - `W/i18n/isolated.tsx` 与 `isolated.test.tsx`；
   - `changeset-text.ts`、`changeset-change.tsx`、`trash-row.tsx`、`search-hit.tsx` 的拼串函数加 `name`，`history-row.tsx`、`changeset-page.tsx` 与这几个文件里用 `Isolated` 渲染；
   - 四个页面测试文件的新格。

   `isolate`、`Isolated` 要与第一个使用者一起进，否则 knip 报没人用的导出。
2. **`web, e2e: the history and a changeset's page keep to a phone's width (26.17, 32.1, 32.2, 32.3, 35.8, 38.12)`**：
   - `history-filters.tsx`、`history-row.tsx`、`changeset-page.tsx`、`changeset-change.tsx` 的类；
   - `breadcrumbs.tsx:50` 的 `[overflow-wrap:anywhere]`：MC3 的变更集页面包屑里有长显示名，要它；
   - `playwright.config.ts` 的新项目；新夹具 `E/fixtures/narrow.ts`；MC3 的新格。

   夹具要与第一个使用者同进，否则 knip 报。
3. **`web, e2e: a long title keeps to a phone's width, on its page and in the dialogs that name it (38.10, 38.11)`**：`page-layout.tsx:259`、`dialog.tsx`、`alert-dialog.tsx`，PG2 的新格，LW3 (page) 加的一段。依赖 2 的夹具与项目，排在 2 之后。

   同一个提交加钉住项目配置的检查（P2-44，5.1；修订核对 B 的 M6）：`tools/e2e-projects/check.mjs`；`knip.jsonc` 的 `entry` 加它（照 `tools/e2e-navigation/check.mjs` 的先例，`knip.jsonc:9`；它只由 `Makefile` 调用，不加就报 "Unused files (1) tools/e2e-projects/check.mjs"，`m12/p2-check1-b/knip-new-tool.txt`）；`Makefile` 的 `lint-web`（`:158-161`）在 `tools/e2e-navigation/check.mjs` 之后加一行 `node tools/e2e-projects/check.mjs`。两处逐项列出 `lint-web` 跑什么的注释各补半句"Playwright 项目配置的检查"：`knip.jsonc:6` 的注释与 `Makefile:159` 的 `##` 帮助（修订核对 B 的 N8）。它断言 `webkit-narrow` 至少有 MC3、PG2 那两格，PG2 的格在这个提交才有，所以不能更早。
4. **`web, e2e: focus rings, the current item of a menu or a list, and muted text read against the page, light and dark (12.4)`**：
   - `styles.css`；8 处 `ring-ring/50`；`button.tsx:14`；`input.tsx:11`；
   - `dropdown-menu.tsx:30,42`、`quick-switch.tsx:171`、`link-completion.ts:128-131`（P2-43）；
   - 新夹具 `E/fixtures/contrast.ts`；SE1、L5 加的段。

   夹具的三个导出在这个提交里都要有使用者（L5 用了三个），否则 knip 报（`m12/p2-review-b/knip-with-draft-fixtures.txt`）。
5. **`web, e2e: a hit's terms are bold, not by their colour alone (26.9)`**：`search-hit.tsx:35`；SE1 的 `:275` 改与 mark 的对比度。用 4 的夹具，排在 4 之后，或与 4 并成一个提交。
6. （只在 33.1 复现时）**`web, e2e: the search results keep to a phone's width on WebKit (33.1)`**：修排查找到的那一处，SE1 拆出 `@narrow` 的一格。
7. S3 的文档提交（记录）里，05-lessons 第 6 节加一句：量计算之后的样式之前，先结束元素自己的过渡（`element.getAnimations().forEach((a) => a.finish())`）；带 `transition-[color,box-shadow]` 的控件刚取得焦点时，读到的是过渡的起点（M12/P2 设计审查 B 的 M2）。

1 与 2–5 互不依赖。顺序建议 1 → 2 → 3 → 4 → 5（→ 6）。

### 6.4 S4

S4 在 S1 合并之后开（P2-1）。

0. **开工时，不提交**：
   - **重做 4.4.9 的核对**（P2-46）：对着 S1 合并之后的代码，S1 的 35.2 `leads`、6.3 的命名与 11.7 的观察者都改了增强与上下文。结果记进 S4 的记录；与 4.4.9 不同的，先改 spec 再写代码。
   - **探查**：经 `heavy.sh` 以三个 project 跑临时的故事，探三样，结果写进 Step 的记录，临时文件删掉。
     - 8.1：各跑 10 次（4.4.4）。
     - WebKit 有没有 CSS 的滚动锚定：决定 UP1 (page) 断言哪一样（48）。
     - Firefox、WebKit 的无头模式给不给全屏：决定 PG15 第 3 格断言哪一样（17.8）。
1. **`web: the reading view in parts: the server's HTML, the anchors and the view (M12/P2/S4)`**（14d）：
   - `W/pages/page/server-html.ts` 的 `useServerHTML` 先照今天的行为：U1 与整篇重写。
   - `W/pages/page/view-anchors.ts` 照搬第二个 effect。
   - 行为不变：已有的 `reading-view.test.tsx` 37 格、`page-tasks.test.tsx` 11 格、`task-toggle.test.ts` 8 格照过。
2. **`web: the reading context reads the revision and the addresses' expiry as they are, has no role, and tells the checkboxes whether the page can be written (M12/P2/S4)`**（U7、48）：
   - 改动：
     - `revision`、`assetsExpire` 改成 getter；
     - 加 `writes` 与 `onWritesChange`；
     - 删 `role`；
     - `taskToggle` 跟着可写与否走；
     - `editable` 与 `role` 移出依赖。
   - 同一个提交里改的已有测试（4.4.5，审查 A 的 M1）：
     - 九个自己构造上下文的测试文件；
     - `reading-view.test.tsx:20-25` 的 `recording`；
     - `source-actions.test.tsx:265`。
   - 新格：
     - `reading-view.test.tsx` 与 `task-toggle.test.ts` 的新格（模式的那一格不在这里，见第 7 个提交）；
     - UP1 (page) 加的一段，照探查的结果写。
3. **`web: a toggle or Edit refused as forbidden reads the notebooks again (M12/P2/S4)`**（6.2，P2-23）。
4. **`server, web: a sample of a signed view, the same for Go and the web (M12/P2/S4)`**（P2-45）：
   - `testdata/m12/signed-view.json` 与 Go 的样例测试（3.2）；
   - `W/reading/patch.ts` 的 `maskAddresses`、`addressesOnly`，与 `patch.test.ts` 读样例的那几格。
   - 两侧同一个提交：样例与读它的两个测试同进。
5. **`web: the same HTML read again is left alone, and addresses alone are signed anew in place (M12/P2/S4)`**（U2、U3、17.1）：
   - `resign` 与它的测试；
   - `reading-view.test.tsx` 的 U3 一格，与 `:820` 那一格加的一步。
6. **`web, e2e: a change is written block by block, and the focus stays or is found again (M12/P2/S4)`**（U4、3、8.2、12.2、17.8、29）：
   - 改动：
     - `patchBlocks`：头尾，与中间的贪心配对（P2-21）；
     - 焦点的键与找回的四步（4.4.3，P2-24）；
     - `adopt` 不让仍在文档里的媒体占键；
     - `VersionView` 改用 `useServerHTML`。
   - 已有测试：`reading-view.test.tsx:500-503` 与 `:492-494` 随之改（4.4.5）：两处都是这个提交的焦点规则改的（第 4 步、标题按文字），放在更早的提交里会红。
   - e2e：新故事 PG15，与 `threeBrowsers` 加的 `page\/pg15-`、`tools/e2e-projects/check.mjs` 加的 PG15 一条，在同一个提交里：不然这个提交的 PG15 不在三种浏览器里跑。`e2e/fixtures/assets.ts` 加 `wav(seconds)`（4.4.6）；S5 已先合并、加了它的，用 S5 的（修订核对 C 的 N17）。
7. **`web: the language and the notebook's mode changing leave the HTML, and code already coloured is not coloured again (M12/P2/S4)`**（U5、11.6、48）：
   - U5；
   - 订阅笔记本的模式，走 U5 的路，与它的那一格 "the notebook's mode changing runs the enhancements again without writing HTML"：这一格断言顶层没有变化，要 U5 在才绿，所以不放在第 2 个提交（修订核对 A 的 M10）；
   - `codeHighlight` 跳过没变、已着色的块；
   - 滚动区的键，与两格图的焦点（4.4.4 的 11.6）。
8. **`web: every id in a drawing is its own, its accessible name kept (M12/P2/S4)`**（11.1）：独立，可以排在最前。
9. （只在 8.1 复现时）**`web, e2e: Back to an anchor keeps the view's scroll over the browser's own (M12/P2/S4)`**：PG15 加那一格。
10. **`docs: … (M12/P2/S4)`**：
    - 13.2 第 23 条与 13.4 第 3 条（8.2）；
    - Step 的记录；
    - 8.1 没复现时，附录 A 的 8.1 移到"发布之后的人工验收"。

### 6.5 S5

照 00 号第 7 节分附件、编辑器、导入导出三组，另加"阅读视图里的附件"一组（与 S4 交叉，见 4.5.2）：

- **附件与上传**
  1. 16.3：`upload-fetch.ts` 的两个计时与 `UploadTimeout`、`upload-rows.tsx` 的 `failureText`、`import-dialog.tsx` 的 `explain`、四个键、测试，**同一个提交**（`UploadTimeout` 导出就要有人用，否则 knip 报；键与用它的代码同提交）。`page-asset-upload.test.tsx:272` 那一格（扣住上传、走 31 分钟）同一个提交里改，否则这个提交红。
  2. 16.11：`asset.store.ts` 的迟到的取消与 `CancelledLate`、`failureText`、一个键。
  3. 16.4c、16.10、38.9：`useNotice` 一处改法，**同一个提交**；`attachment-uploads.test.tsx:181` 的旧断言同时改。
  4. 16.4b（复制的字段跟着附件）。
  5. 28.1（"已不在这里"跟着列表）。
- **编辑器**
  6. 18.2：`EditorContext.refuses`、`asset.store.ts` 导出 `refusalOf`、`asset-upload.ts` 的粘贴、`page-edit.tsx` 接上，**同一个提交**，连同六个测试文件里 `EditorContext` 的字面量（否则 tsc 红，见 18.2）。
  7. 18.1：`UnsavedGuard` 看编辑器在途的上传，两个键。
  8. 12.8：补全的"表格"照 goldmark。
- **导入导出与来源**
  9. 19.4。
  10. 20.1、20.2：同一处（`import-dialog.tsx:301-302`），**同一个提交**。
  11. 20.3。
  12. 38.7：三个键。
- **阅读视图里的附件**
  13. 17.6（不依赖 S4）。
  14. 15.1：先一个服务端的提交（`data-nw-sized`，Go 单元），再一个网页的提交（CSS 与 AS3 的 e2e）；两个各自为绿（没有这个属性时 CSS 与今天相同）。
  15. 15.2 ＋ 新建 AS6（它的第一格）：`title-media.ts`、登记、`title-media.test.ts`；`e2e/fixtures/assets.ts` 加 `wav(seconds)`（静音的 PCM，照设计者探针的写法，服务端按 `audio/wave` 收，`asset/domain/types.go:36`）；S4 已先合并、加了它的，用 S4 的（修订核对 C 的 N17）。
  16. 17.2b ＋ AS6 的一格。AS6 只在 Chromium，不改 `playwright.config.ts`。
  17. 17.4 ＋ AS6 的一格（**S4 合并之后**）。
  18. 17.7（**S4 合并之后**）。

硬依赖只有 17.4、17.7 在 S4 合并之后（主持者对审查 C 的 Q1 的决定）：这两个提交建在 S4 合并之后的 `main` 上。别的 Step 与 S5 同改的文件只是文本冲突，后合并的先 `git merge main`。

### 6.6 S6

每个提交单独为绿（`tsc`、`knip`、vitest、`oxlint`）。knip 报没人用的导出：新导出的帮手要在同一个提交里有调用者。

1. **12.12**：`page-tree.ts` 的两处与一格。
2. **24.8 与 24.10**（必须同一个提交）：`handlers.ts`、`event-stream.tsx`、`handlers.test.ts`（`context()` 加 `trees`）、整合格、TB2。TB2 改回"在 Big 自己的页上删除"之后，没有 24.8 它会红。
3. **23.19**（含回收站预览）与假服务端按 `sameId` 找：排在 23.10 之前，store 的键从此一律小写。
4. **22.14 与 23.10**：store 的两张表与读法、`trash-row.tsx` 的 `trashRestoredOf` 与 `RestoreButton` 的 `busy`、预览与列表、撤销对话框的跟随，与它们的测试。可拆成三个，store 与它的测试先（`restoreOut`、`revertOut` 是类的方法，knip 不报）。
5. **22.19**：预览的外壳，排在 22.14 之后。
6. **21.3 与 22.2**：`useRouteNotice` 由两处用；或 21.3 先、22.2 后各一个（帮手随 21.3 进来）。
7. **21.2 与 21.10**：都改 `said` 的形状。
8. **21.7**：`comparison.tsx`、`text-diff.tsx`、`conflict-panel.tsx`、`conflict-view.tsx`。
9. **24.2**、**24.9**：各一个或合一个。
10. **2**：`PageGone`、`useNewPage`、假服务端的 `deletesHeld`。
11. **26.2 前一半与 26.15**：搜索框。
12. **26.5**：`native-select.tsx` 的帮手、搜索页、历史与目录的筛选、SE2 的一段。
13. **26.18a**：`text-ranges.ts`。
14. **35.3**：目录与新键（en 与 zh-CN 同一个提交）。
15. **27.2**：照 P4/S2 的结论写（4.6.6）。保留整棵树时：store、`notebook-layout.tsx`、`page-layout.tsx`、`version-page.tsx`、测试与 SE1 的一段，一个提交。

## 7. 测试与验证

### 7.1 每项的测试

每一项的测试写在它那一节（4.1–4.6），包括：

- 测试文件与新格；
- 它在没修的代码上怎样失败；
- 已有的测试里要随之改的。

分工照 00 号文档 9.1：

- **vitest**：盖住逻辑、名称、描述、角色、焦点的去处（元素移除之后）与路由状态。
- **e2e**：只放 jsdom 做不到的：布局与宽度、计算之后的颜色、浏览器自己的焦点修正与滚动、复制与下载、跨标签页的频道、真的事件次序。
- **三种浏览器**：只给各引擎不同、又常驻套件的焦点与滚动：新的 PG15，已在的 A10、LW3、UP1。复制、下载与 15.2 在三引擎里各探过（审查 C、设计者与修订核对 C），行为相同或只在 Chromium 复现，所以 AS6 只在 Chromium（P2-30）。15.2 的改法在 Firefox、WebKit 上与不改相同，点过控件之后程序里的打开照常（修订核对 C 的探针）。
- **375 px**：Chromium 与 WebKit（`@narrow`）。

几种难造的情形，各节都照下面的办法确定地造出，不靠重复跑：

| 情形 | 怎样造出 | 用在 |
|---|---|---|
| 发送途中、被拒之后 | 假服务端或 `page.route` 扣住第一个请求，答 503 之后 `route.fallback()`；不 `unroute`（05-lessons 第 6 节） | 23.16、9.1、22.14、23.10、24.2 |
| 整点之后的重签、休眠之后的计时 | vitest 的假定时器推进，不真的等 | 17.1、17.6、16.3 |
| 节点没换、没有闪 | `MutationObserver` 记 `childList`；在 `window` 上留一个节点的引用，事后比 `===` | S4 的 U2–U7、48、17.8 |
| 事件先于答复 | 扣住删除的答复，直到事件引起的树的读答了才放 | 24.8（TB2 与 vitest 的整合格） |
| 参数上限 | 测试里先翻倍探出 `RangeError` 的上限，再造两倍于它的子树 | 12.12 |
| 别的标签页 | C6 用两个 tab，走真的 `BroadcastChannel`；vitest 用 `W/events/testing/fake-channels.ts` 的 `FakeChannels`（同 `edit-closing.test.ts`） | 第 7 条 |
| 375 px 与对比度 | `expectFits`（页面、对话框的右边 ≤ 视口）；`contrast.ts` 先结束元素的过渡，再从计算之后的颜色算比值（审查 B 的 M2） | S3 |
| 复制与下载 | Playwright 读剪贴板、`waitForEvent("download")` | 17.2b、17.4 |
| 媒体控件引起的开合 | e2e 在控件不是按钮的地方点（左边的空白、时间的显示），只有 Chromium 复现；vitest 派发媒体的 `DOMActivate`、设 `open`，用假计时推过设回的那个任务（4.5 的 15.2） | 15.2 |
| 服务端的拼法 | Go 与网页共用 `testdata/m12/signed-view.json` | 17.1（P2-45） |

### 7.2 变异

- **逐项的变异**：写在它那一节的"变异"里（4.1–4.3、4.5、4.6），S4 的表在 4.4.7。
- **跨项的变异**：下表只放守着 2.3 的取舍、牵动几项或几个 Step 的那几条，每个 Step 的审查必做。
- **记录**：每一条变异的结果（红在哪一格）写进 Step 的记录。
- **抓不到的**：照实写，并判断它是不是等价变异（05-lessons 第 1 节）。逐项里已写明"等价"或"只能探针核对"的，记录写出等价的证据或探针的结果。

| # | Step | 变异 | 让它失败的测试 |
|---|---|---|---|
| X1 | S1 | `ConfirmDialog` 发送中改回 `disabled` | `confirm-dialog.test.tsx` 的焦点一格；A10 (page) 的撤销被拒一段（三种浏览器） |
| X2 | S1 | 去掉"焦点丢了就交还给确认"（`focusLost`） | `confirm-dialog.test.tsx` 焦点丢了的那一格 |
| X3 | S1 | `useForm` 无条件把焦点给横幅（P2-3 反过来） | `form.test.tsx` 的那两格；`revert-background.test.tsx:183`；HI3 |
| X4 | S1 | 去掉 `button.tsx` 的 `aria-disabled:opacity-50` | HI3 |
| X5 | S3 | 删掉全局的 `:focus-visible` 规则 | SE1 (page) 断言焦点指示以 `outline solid` 开头的那一步 |
| X6 | S3、S4 | `webkit-narrow` 的 `grep` 写错；`threeBrowsers` 漏掉 PG15 | `tools/e2e-projects/check.mjs`（`make lint-web`；P2-44） |
| X7 | S4 | 网页的地址正则与服务端的拼法不合 | `patch.test.ts` 读样例的那一格；服务端一侧改拼法时 Go 的样例测试（P2-45） |

### 7.3 门禁

- **每个提交**：见 6.0 第 3 条。
- **合并之前**：
  - 经 `heavy.sh` 跑 `make check` 与 `make e2e`；S2、S4、S5 另跑 `make test`。
  - 先看一眼 `df -h /`（06-environment 第 3 节）。
- **合并之后**：`main` 上 CI 的 19 个作业全绿。
- **e2e 的门禁**：
  - `tools/e2e-navigation/check.mjs` 核对新故事只经 `goto`、`reload` 换文档，并按表核对逃生口。PG15、AS6 不用逃生口。
  - `tools/e2e-projects/check.mjs` 核对项目的配置（5.1）。
- **翻译**：`Messages` 的类型保证两种语言的键相同；新键的两种语言都有一格测试（13.2 第 4 条）。

### 7.4 CI 的余量

**今天**：

- e2e 作业（`.github/workflows/ci.yml:265-271`，时限 20 分钟）最近几次是 7:04–9:39，E2E 一步 6:03–8:10。9:39 与 8:10 是 `194ca350` 那一次（run 43745380462，attempt 2），记在 00 号文档第 13 节。
- 同一棵树的同一份，可以差三成（05-lessons 第 7 节）。
- `web` 作业时限 15 分钟。vitest 今天约 1,922 格、192 个文件。

**e2e 多跑多少**（5.2 的 test-seconds 折成墙钟）：

- CI 的 e2e 没写 `workers`，默认取一半的核。`ubuntu-24.04` 推断 2 个 worker，墙钟约是 test-seconds 的一半到三分之二。
- 逐 Step 推断：

| Step | 多跑的 test-seconds | 多用的墙钟 |
|---|---|---|
| S1 | 6–7 | 约 4 秒 |
| S2 | 0 | 0 |
| S3 | 42–50（33.1 复现时再加约 6） | 22–30 秒，区间 15–45 |
| S4 | 57–93 | 35–45 秒 |
| S5 | 16–26 | 8–15 秒 |
| S6 | 3–4 | 约 2 秒 |
| 合计 | 约 125–180 | 约 70–95 秒 |

**结论**：

- P2 加的整个量，比同一棵树两次运行之间的抖动还小。e2e 作业的上沿约从 9:39 到 11:15（推断），离 20 分钟的时限仍有 8 分钟以上。P3–P5 加的另算。
- **`web` 作业**：P2 加的 vitest 推断在 200 格上下，约一成；按今天的量级多几十秒，离 15 分钟远。
- **服务端**：S2、S4、S5 加的都是单元测试，`short` 与 `race` 两份各多不到 1 秒。

**怎样照实量**（审查 C 的 M6）：

- 每个 Step 记下 CI 的 `list` 报告里，它改到、新加的那几格的秒数之和，与 5.2 的推断对照：这一个数不受整份运行的抖动影响。
- e2e 作业的总时间按合并之后两次 `main` 的平均判，同 06-environment 第 4 节判分份的口径。平均超过 13 分钟时，在下一个 Step 之前告诉主持者。

## 8. 完成标准

### 8.1 条件

P2 完成，当且仅当下面七条都成立。

1. **合并与 CI**：
   - 六个 Step 各自合并进 `main`，一次一个，守 0.1 的硬依赖；S6 在 P4/S2 的结论之后开（P2-1）。
   - 合并之前，7.3 的门禁全绿，审查与修复核对结束。
   - 每次合并之后，`main` 上 CI 的 19 个作业全绿，e2e 作业的时间记下（7.4）。
2. **测试与变异**：
   - 4.x 的每个测试都在，而且在没修的代码上红过（Step 的记录）。
   - 7.2 与各项的每个变异被抓到，或在 Step 的记录里写明等价及其证据（探针的结果）；第 10 节记下。
   - 探查与排查都有记录：S1 的三引擎探查、S4 开工时的核对与三项探查（6.4 第 0 步）、S3 的 33.1 排查、S5 的 AS6 在 Firefox 与 WebKit 上的一次。
3. **去处**：附录 A 里 P2 的 118 项，每一项都有去处，是下面三种之一：
   - 修好，记下合并号；
   - 照本文推后进 v0.2，即 2.3 里写明推后的那几部分；
   - 移到发布之后的人工验收：8.1、33.1 没复现时。
4. **00 号文档**（`docs/v0.1/M12-release/00-M12-design.md`）：
   - **接受本文的文档提交**：
     - 2.1 的第 1–7、9 条照写的改，"总体设计的修订"加第 19–24 条。
     - 2.3 的取舍 P2-1–P2-52 写进第 11 节：主持者给正式编号，接着当时最大的号（`48bbfc99` 上是 377）；另起一节"P2 的设计审查之后的取舍（主持者的决定，负责人可以改判）"，每条一句，带它在本文与审查记录里的位置。
     - 第 12 节的 P2 一行：Phase 文档链到本文，状态写"进行中（P2 设计已审查）"。
     - 第 13 节加一行。
   - **每个 Step 的文档提交**（在 Step 的分支上、合并之前）：
     - 第 12 节的 P2 一行：状态写到这个 Step；"审查"一格加它的记录 `reviews/P2-S<m>-<slug>-review.md`；"合并"一格先写占位，合并之后在 `main` 的小文档提交里填入合并号。
     - 第 13 节加一行。
     - S3、S4 另照排查与探查的结果改附录 A 的 33.1、8.1（2.1 第 8 条）。
   - **`docs: M12/P2 done (M12/P2)`**：第 12 节的 P2 一行写"完成"；头的状态随之改；第 13 节加"P2 完成"一行。
5. **v0.1 总体设计**：照 8.2，各在实现它的那个 Step 的文档提交里改。
6. **移交的"处理进展"**：各加一行，状态仍是 `open`，收尾时关。
   - 每个 Step，`handoffs/M5-polish.md`：
     > `- M12/P2/S<m>（日期，合并见 [00-M12](../00-M12-design.md) 第 12 节）：做完的条目（编号）；推后进 v0.2 的（编号与理由）；改法与这里写的不同的（编号，见 P2 设计 2.3 的正式编号）。`
   - S2，`handoffs/M8-history.md`：第 4 条的界面一句做完。回收站与版本列表各一句，原文见 4.2 的 H4。部署文档与发布说明在 P5/S3。
   - S4，`handoffs/M4-performance.md`：第 13 条做完。每小时的重读只就地换地址，不再换节点（P2 设计 4.4 的 U3）。图片仍按新地址重新下载，带宽照旧并入第 11 条，由 P4/S2 量。
7. **其余的文档**：
   - README 不改：38.6 是让界面与 README 的 MiB 一致。
   - 审查发现的新问题，照 02-workflow 加进 05-lessons。

**S1 之后**（2026-11-16）：第 1 条，S1 已合并（`d1ade2ed`），合并之前 7.3 的门禁全绿、审查与修复核对结束。第 2 条，S1 的测试在没修的代码上红过，变异都抓到或写明（10.1 与 [S1 的记录](reviews/P2-S1-focus-and-shared-review.md)），三引擎的探查与鼠标的探针记下。第 3 条，附录 A 里 S1 的 22 项修好，第 7 条里来不及答的标签页推后进 v0.2。第 4–7 条，S1 的文档提交照写的改了（00 号文档第 12、13 节，总体设计 13.2 第 1、12、17、21、26 条与第 15 节，`M5-polish.md` 的一行，05-lessons）。

**S2 之后**（2026-11-18）：第 1 条，S2 合并见 00 号文档第 12 节；合并之前的门禁照 7.3 与主持者的说明跑过、全绿（实施时 `make check` 的各项、`make e2e` 与 `make test`；修复第 1 轮与合并 `main` 之后 Go 只跑 `wiki` 模块），审查与修复核对结束（核对第 1 轮之后不开第二轮，决定 588）。第 2 条，S2 的测试在没修的代码上红过（35.1 的 `index-page` 一组以四条"恒假"变异为证），变异都抓到，活下来的五句中文补格之后抓到（10.2 与 [S2 的记录](reviews/P2-S2-text-review.md)）。第 3 条，附录 A 里 S2 的 28 项与 H4 的界面一句修好，推后进 v0.2 的照 2.3 与决定 587。第 4–7 条，S2 的文档提交照写的改了（00 号文档第 11、12、13 节与第 7 节第 20 条，总体设计 13.2 第 17 条、3.6 与第 15 节，`M5-polish.md`、`M8-history.md` 各一行，05-lessons 两条）；README 不改。

### 8.2 总体设计的新文字

改的是 `docs/v0.1/v0.1-design.md`。

- **锚点**：每段旧文字都在 `194ca350` 上核对过恰好出现一次（`python3`，`str.count`）。替换时，脚本再断言一次。
- **第 15 节**：每个 Step 的文档提交各加一行，写日期、改了下面哪几条、依据（本文与 Step 的审查记录），末尾是 `M12/P2/S<m>`。

#### S1：13.2 第 12、26、1、21、17 条（第 11、17、19 条修订）

1. **第 12 条**（00 号文档第 7 节第 11 条）。旧文字，起：`` `ConfirmDialog` 发送中仍用 `disabled` ``，止：它之后第一个 `第 23 条）。`；整段是：
   > `` `ConfirmDialog` 发送中仍用 `disabled`，留到 v0.1 之后的打磨（[M12 的打磨移交](M12-release/handoffs/M5-polish.md)第 23 条）。 ``

   新文字（取代 00 号文档写的"…被拒之后焦点还给确认按钮或说明；只有横幅的拒绝时焦点给横幅…"，理由见 2.3 的 P2-2、P2-3）：
   > `ConfirmDialog` 在 M12 照此（M12/P2/S1）：发送中"确认"与"取消"用 `aria-disabled`（"确认"另带 `aria-busy`），忙的守卫让一次按下至多一个请求，Esc 与"取消"在发送中不关闭对话框；焦点不动：按下"确认"时它拿着焦点的（键盘，Playwright 的三种引擎用鼠标也是），发送中与被拒之后仍在它上面（丢了就还给它）；浏览器不给点过的按钮焦点的（Safari），焦点留在原处；拒绝由对话框里的 `role=alert` 说出。`useForm` 的表单发送中仍禁用按钮：失败而没有 `aria-invalid` 的字段（只有横幅说的拒绝），且拒绝到达时焦点已丢（在 `body`，或在发送中禁用的元素上）时，焦点给横幅；焦点没丢的（撤销的确认框）、读者放到别处的不动（第 26 条）。`aria-disabled` 的按钮与禁用的一样变暗（`components/ui/button.tsx`）。

2. **第 26 条**（第 17 条的前一半）。旧文字恰好一句：
   > `无主笔记本接管之后的焦点（M3）还没有照此，留到 v0.1 之后的打磨（[M12 的打磨移交](M12-release/handoffs/M5-polish.md)第 22 条）。`

   新文字：
   > 无主笔记本的接管（M3）在 M12 照此（M12/P2/S1）：从按下时看，焦点仍在"接管"、`body` 或这一节的标题上、读者其间没有动作时，接管成功或答 `notebook.not_found` 之后标题才取得焦点（`ownerless-page.tsx`，同回收站的恢复）。

3. **第 1 条**（第 17 条的后一半）。旧文字恰好一处：`见 M12 的体验移交第 7 项`，所在的括号是"（`stores/edit-closing.ts`，答复是否有编辑等 100 毫秒：收到请求时主线程被占住更久的标签页当作没有编辑，见 M12 的体验移交第 7 项；退订之后不再答复，端口可能已关闭）"。新文字（只换那几个字）：
   > M12/P2/S1 起退出之前先问"有没有未保存的编辑"（第 21 条），同样只等 100 毫秒，这样的标签页也不算在内；要等它们得让有编辑的标签页事先登记（Web Locks），进 v0.2

下面三条原不在 00 号文档第 7 节的修订表里，S1 之后旧文字不实，随 S1 改（2.1 第 3 条的第 19 条修订）：

4. **第 21 条**。旧文字恰好一句：`退出登录不问：先经编辑器悄悄保存，再结束编辑（第 1 条）。` 新文字：
   > 退出登录先问（M12/P2/S1，`app/sign-out.tsx` 的 `useSignOut`；用户菜单、邀请页与会话不可用时的"退出登录"）：这个标签页的编辑，或同一登录的别的标签页经频道 `nwiki.edits` 在 100 毫秒内说有的，有未保存的修改时，先问"仍要退出登录吗"（留下，或退出）；退出之后照旧先经编辑器悄悄保存，再结束编辑（第 1 条），保存不了的随退出丢失。

5. **第 17 条**。旧文字恰好一句的末尾：`打开、切换进入的页面不抢焦点。` 新文字：
   > 打开、切换进入的页面不抢焦点；链接随它所在的页一起离开的（子页列表、面包屑、工作区首页的笔记本卡片、右栏与阅读视图里的链接）带 `arrived`，否则焦点落到 `body`（M12/P2/S1）。左栏的树打开的页带 `opened`（连同"目录""历史"两行）：点开时焦点留在树上，后退、前进回到它时标题只从无人处取（M12/P2/S1）。

6. **第 26 条到达的那一句**。旧文字恰好一句：`标题在一次导航的第一次挂上时取到达的焦点（`arrived` 或 `Located`）；同一次导航之后再挂上时，只在焦点空着（`document.activeElement` 为空或 `body`）时取。` 新文字：
   > 标题在一次导航的第一次挂上时取到达的焦点（`arrived` 或 `Located`）。`arrived` 的导航，读者自提交起有过动作时（`Layout` 的 `useArrivalWatch`，从提交起的 `watchReader`）只从无人处取：迟挂上的标题（笔记本的树还在读）不抢读者正在打字的框；`Located` 不进这个看守，附件的行自己看读者（`useLocated`）。同一路径、查询与片段、只换 state 的 `replace` 不算新的导航：看守不重新起算，已挂过标题的记录接续；同一路径、换了查询或片段的 `replace`（目录、历史的筛选，"在哪里找"）是新的导航，看守重新起算，但页没换、标题不重挂，已挂过标题的记录照样接续（M12/P2/S1）。同一次导航之后再挂上时，只在焦点空着（`document.activeElement` 为空或 `body`）时取；代替这次导航已显示的页的"页面不存在"（`NotFoundPage` 的 `replacing`：撤销答 404、编辑中被删的页）同样只从无人处取（M12/P2/S1）。

另外，第 12 条里的 `失败之后焦点移到第一个 `aria-invalid` 的字段；`（恰好一处）可以不动：横幅的规则写进了第 1 条的新文字。第 26 条"现有的几处"可以不加 22.17：第 2 条的新文字已在同一条里。

#### S2：13.2 第 17 条（第 20 条修订）

旧文字恰好一处：

> `根下的页面带笔记本名（"Install（Guide / Setup 下）"；`

新文字：

> 根下的页面写作笔记本的顶层（"Install（笔记本的顶层）"，文案 `page.nameAtTop`，M12/P2/S2），别的写上级的位置（"Install（Guide / Setup 下）"；

落进总体设计的是这一句：修复第 1 轮之后照决定 584 不带笔记本名，原来的示例是"Install（Plans 的顶层）"（实际见 10.2）。

后面的 `` `stores/page-tree.ts` 的 `placeOfTitle`…文案 `page.nameIn`；M4 收尾审查 B-M4） `` 不动。

#### S3：13.2 加第 30 条（第 21 条修订，P2-19）

插在第 29 条末句 `读者动过就不动（第 26 条）。` 之后（恰好一处），另起一行：

> 30. **名字、窄屏与对比度**（M12/P2/S3）：名字（页面、笔记本、附件、人、客户端）与别的文字在同一行时进 `bdi`（`i18n/isolated.tsx` 的 `isolate`、`Isolated`：名字照译文的语序落位；一句由几段拼成时，各段经同一个收集器取名字的位置；只要字符串的地方，拼句的函数不传收集器，照旧出字符串）。flex 项与 grid 的 `auto` 轨以最小内容为下限：里面放长文字的项加 `min-w-0`，或让文字 `[overflow-wrap:anywhere]`；原生的选择放在 `max-w-full min-w-0` 的外层里（`pages/notebook/index-filters.tsx` 的写法）。375 px 的检查用 `e2e/fixtures/narrow.ts` 的 `expectFits`，在 Chromium 与 WebKit 上跑（标签 `@narrow`）。焦点指示对相邻的颜色至少 3:1：链接与别的元素是 `:focus-visible` 的 2 px 实色外框，控件（按钮、输入框、下拉、标签页）是 3 px 的 `ring-ring` 环，无效的字段是实色的 `ring-destructive`，菜单项、快速切换的选项与补全的选中项另有看得见的指示（内嵌的 2 px `ring-ring` 环），不只靠底色。次要文字（`--muted-foreground`）对它的底色至少 4.5:1；靠颜色区分的另有颜色之外的提示（搜索命中加粗）。对比度在 e2e 里从计算之后的颜色算，先结束元素的过渡（`e2e/fixtures/contrast.ts`）。

#### S4：13.2 第 23 条（第 22 条修订）

第一处，旧文字恰好一处：

> `HTML 换掉时倒序清理、再重跑；`

新文字：

> HTML 的内容变了时倒序清理，只把变了的顶层块换新（头尾与中间按键配上的块留着，留着的节点不移动），再重跑；同一份 HTML 再读到、只差附件地址（每小时的重签，地址就地换）、可写与否变了（`ReadingContext` 的 `writes` 与 `onWritesChange` 告诉增强）时，不清理也不重跑；语言或笔记本的模式变了时清理、再重跑，不动 HTML；上下文的 `revision` 与 `assetsExpire` 读最新的；代码高亮跳过没变、已着色的块。留着的块里的节点都还在，焦点、选区与展开的 callout 随之还在（公式与图由增强重写，其中的选区除外）；焦点所在的块换了新的，按键找回同样的元素，找不到就给它原来所在的那一块（块经头尾与中间的配对映射到新的位置），不展开、不滚动；任务勾选框照第 17 条的规则，找不回不给（`pages/page/server-html.ts`、`reading/patch.ts`，M12/P2/S4）；

第二处，旧文字恰好一处：

> `开始了的音视频在 HTML 换掉时接回到新 HTML 里同一附件的位置，清理不还原`

新文字：

> 开始了的音视频在它所在的块换新时接回到新块里同一附件的位置；块留着时它就在原处，不占配对的键、不搬动（M12/P2/S4），清理不还原

#### S5：13.2 第 27 条（第 23 条修订）

第一处，旧文字恰好一处：

> `所以靠到期之前的重读，不在点击时再签；`

新文字：

> 所以靠到期之前的重读，不在点击时再签；下载的链接例外：点击时比到期，过期了先再签、再下载（`reading/expired-downloads.ts`，M12/P2/S5）；

第二处，在第 27 条末句 `` （`attachments-section.tsx` 的 `useNotice`）。 `` 之后（恰好一处）加一句：

> 附件与导入的上传不设整体的超时：请求体在发时 60 秒没有进度、发完之后 180 秒没有答复就放弃（中止会让服务端的事务回滚，所以发完之后等得更久），行上（或导入的对话框里）分两种说明，离开页面的提醒随之关掉（`services/upload-fetch.ts` 的 `UploadTimeout`，M12/P2/S5，移交第 16 条）。

#### S5：13.2 第 7、12、21 条（第 23 条修订）

第 7 条（17.6），在 `至迟在读之后的一小时之内）` 之后（恰好一处）加：

> ；计时每次至多隔一分钟看一次时钟：机器睡醒之后至迟一分钟重读，从往返缓存回到页面时立即比较（M12/P2/S5，移交第 17 条）

第 12 条（38.7），在末句 `它们是读取控制，不增加自动提交表单。` 之后（恰好一处）加：

> 粘贴 Markdown 的对话框（来源区）有填写的内容时，Esc 与点外面先问；发送中不关；"取消"照旧直接关（M12/P2/S5，移交第 38 条）。

第 21 条（18.1），在 `它的"回到阅读"在有未保存的修改时先问。` 之后（恰好一处）加：

> 编辑器里粘贴、拖入的文件还在上传时，应用内离开同样先问，说它们不会插入页面；离开之后上传照样完成，附件照样加进来（M12/P2/S5，移交第 18 条）。退出登录不为在途的上传问。

#### S6：13.2 第 12 条（第 24 条修订）

在 `不在选中时发出（方向键移动就会选中）。` 之后（恰好一处）加：

> 选了就换地址的原生选择（搜索的"在哪里找"、历史与目录的筛选）同理：在关着的选择上用键盘一项项换时，等读者离开它或按回车再用，指针与打开的列表里的选择立即用（`components/ui/native-select.tsx` 的 `useChoice`，M12/P2/S6）。

#### S3、S4：13.4 第 3 条

末句 `（WebKit 在请求下一个文档之后旧文档发不出请求）（M12/P1）。` 恰好一处。两个 Step 各在它之后加自己的一句，按合并的次序：

- S3：" M12/P2/S3：375 px 的检查带标签 `@narrow`：不在 `threeBrowsers` 里的故事，由 Playwright 项目 `webkit-narrow` 在 WebKit 上再跑一遍，夹具 `e2e/fixtures/narrow.ts`；项目的配置由 `tools/e2e-projects/check.mjs`（`make lint-web`）核对；对比度由 `e2e/fixtures/contrast.ts` 从计算之后的颜色算，量之前先结束元素的过渡（M12/P2）。"
- S4：" M12/P2/S4：PG15 在三种浏览器里核对阅读视图的就地更新：别人写了别处之后，焦点、展开的 callout、选区与播放都还在；在拿着焦点的标题之前插进同名的一节，焦点留在原来那一节；全屏照 S4 的探查，在给全屏的引擎里核对（M12/P2）。"

## 9. 交给后面的 Phase

- **P3（并行）**：
  - P3/S1 的网页不改代码与文字（P3 设计 4.1）。
  - P3/S3 与 P2 同改的文件列在 0.1 的表里，都是文本冲突，后合并的先 `git merge main`。
    - P3/S3 的网页部分在 P2/S1 合并之后写（P3 的硬依赖，0.1）；之后与 P2 的 S2–S6 谁后合并，谁照对方已合并的样子改。
    - 关键的三处：一是 `revert-background.test.tsx` 的同一张 `test.each` 表，P3 加一行、P2 的 24.2 删一行；二是 20.2 的 `refusedBusy` 只认 `transfer.busy`（0.1 末条）；三是 `transfer.importCut` 的文字由 P3/S3 改，S5 的两个新键接在它之后。
  - P3/S1 的 10.2（标题 id 带上自动链接与原始 HTML 的文字）改已有的锚点 id。S4 的焦点键对标题按文字、不按 id（4.4.3），不受它影响；PG15 与 L1 的锚点断言用的标题没有自动链接，也不受影响。
- **P4/S2（页面树）**：
  - S6 在它的结论之后开；结论要改树时，在改法的 Step 之后开（P2-1）。
    - 改法的 Step 的设计照 4.6.6 写明会碰到的 S6 的项；
    - S6 开工之前照 4.6 开头的"开工之前补"补齐。
  - M4-performance 第 11 条（阅读视图里的大图与带宽）在 S4 之后量：U3 之后每小时的重读不再换节点，但图片仍改 `src`、重新下载（4.4.2，P2-47），量的是这一种。
  - 移动对话框的 `canHold` 与树行的 `subtreeOf` 的开销，可以顺手量（4.6.6 末）。
  - `e2e/perf/` 的脚本照 P1 的门禁经 `goto`。
- **P5/S3（发布说明与人工清单）**：
  - **`CHANGELOG.md` 的已知限制从 P2 取**：附录 A 的 C 类（按主题）；4.4.8 的 S4 之后仍在的限制；8.1、33.1 若没复现，写"待人工确认"。
  - **读屏清单**（`manual/screen-reader-checklist.md`）收：
    - 只从 ARIA 推断的八项；
    - S1 改的焦点去处在真实读屏上念什么：`ConfirmDialog` 被拒之后、横幅的拒绝之后、"仍要退出吗"；
    - S4 的第 4 步（4.4.3：给它原来所在的那一块）念出的是哪一块。

    P2 的各 Step 记录列出它们，P5/S3 照抄。
  - **真浏览器的下载许可**：17.4 在程序里再点下载，真的 Chrome 的"自动下载多个文件"、Safari 的"允许这个网站下载吗"会不会拦，进人工清单（审查 C 的未核实第 2 条）。
  - **汇总的人工入口**（`manual/README.md`）：8.1、33.1 没复现时进这里。
- **收尾**：v0.2 的 `M12-polish.md` 收 C 类 87 项、D 类其余、4.4.8 的限制、各 Step 记录里"推后的"，与 2.1 第 7 条列的几处。

## 10. 结果

（实施后补。每个 Step 一节：合并号、与设计不同之处、变异的结果、门禁与 CI、e2e 的用时。）

### 10.1 S1：焦点与共用组件（2026-11-16，合并见 00 号文档第 12 节）

- **提交**：十五个代码提交与一次合并 `main`，加本文档提交；[审查记录](reviews/P2-S1-focus-and-shared-review.md)。
  - 第 1–10 个照 6.1，头 `14cc9769`；
  - 修复第 1 轮四个（`52295438`、`1ebe1b16`、`793c062c`、`137356d6`），第 2 轮一个（`2d707feb`，只改注释）；
  - 合并 `main` 的 `01283f3f`（P3/S1、S2 与 P4/S4 的设计），没有冲突；
  - 三份审查 A 0I2M2N、B 0I1M1N、C 0I3M8N1Q；核对第 1 轮只有 C 的一条 Nit，修复第 2 轮由主持者核对。
- **做完**：A：7；B：6.3、9.1、11.7、12.3、16.1、18.6、19.3、21.14、22.17、23.14、23.16、24.3、26.4、28.4、28.5、35.2、38.1、38.3、49；C：23.17、28.2。推后进 v0.2：第 7 条里来不及答的标签页（主线程被占住超过 100 毫秒；要事先登记，Web Locks）。
- **与设计不同之处**（正文照实改了，各处注"实际见 10.1"）：
  - **16.1** 改成两段清理：layout 清理记下焦点在不在行或菜单里，passive 清理在焦点已丢时给标题。只在 layout 清理里给，菜单开着时被菜单的 `FocusScope` 拉回将卸下的菜单项、随后落到 `body`。
  - **35.2**：只有普通链接看 `leads`，嵌入与图片照旧是按钮（M10 的 `virtual-navigation` 守格）；`virtual-navigation.test.tsx` 四格与 LW10 (page)（三种）改按链接找。
  - **`security-page.test.tsx:185`** 随 1 号改成 `aria-disabled`。
  - **9.1**："只改 `form.ts` 就全改到"对上传对话框里来源的重试不成立：提示先取上传的失败、不带 `data-form-banner`，"Add source"发送中卸下、焦点被交给对话框本身。改成横幅在前、按钮发送中禁用而不卸下（决定 506）。
  - **26.4 与 4.1.4**：筛选与"在哪里找"的 `replace`"不受影响"不成立：新地址不在 `offered` 里，之后挂上的"页面不存在"当作第一次。同一路径的 `replace` 也接续 `offered`（决定 507）。
  - **4.1.2 的测试表**漏了邀请页与会话不可用页的"退出登录"，7.3 要的新键两种语言也没列全，设计与实现同漏，修复第 1 轮补上；表里第 8 格加的"没问"是空断言，去掉。
  - **13.2 第 12 条的新文字**：探针之后改成"焦点不动"的写法，Safari 用鼠标的情形写明（决定 509）。
  - **23.16 的 e2e 说明**"WebKit 鼠标点按钮不给焦点"在 Playwright 里不成立（三种引擎都给）；A10、LW3 的注释改成 Safari。
  - **推断错了的变异**：28.4 的 `useEffect` 起初照过，补审查 B 的一格之后红；28.5 的两条与 22.17 的一条各补一格。
- **4.1.0.5 先核对的五条**：
  1. 16.1：清理次序成立；在 layout 清理里给不成立，见上。
  2. 28.4：全量 vitest 在看守之下全绿（4,602 格），没有格依赖"标题挂上之前有点击，标题照样抢"。懒加载块读的期间按的键没量：vitest 里路由的懒加载在同一次 `act` 里读完，造不出这个窗口；e2e 要扣住块的请求，S1 没做。
  3. 28.5：树里"目录""历史"两行与页行同类，带 `opened`。
  4. 11.7：照代码读，`aria-busy` 从问落点起，到落点的答复处理完（去那一页、读父页或开对话框）为止，在 `finally` 里去掉，对话框开着时不忙；`unresolved-link.test.tsx` 两格在"不存在"的对话框开着时断言没有 `aria-busy`，新建页的确认框走同一个 `show()`，没有另一格。
  5. 23.16：推断不成立。Playwright 的三种引擎用鼠标点"确认"，焦点都在它上面（打开时在"取消"），发送中（"Revoking…"）与被拒之后仍在"确认"上（各两次）。Safari 不给点过的按钮焦点，那里焦点会留在"取消"上（照代码推，没核实），交 P5/S3 的人工清单。
- **变异**：vitest 57 条，加修复第 1 轮 12 条，都抓到（28.4 的 `useEffect` 在补格之后）。e2e 的 X1、X3、X4、横幅不取、第 7 条、12.3、焦点给 Alert（挂上时取）都抓到；在 `setTimeout` 里给 Alert 的 A10 照过（`toBeFocused` 判过一次就不再看），vitest 抓。"去掉 `:disabled`"vitest 抓，LW3 在 WebKit 上也抓（审查 C 两次共 5/5 红，实施者一次过：拒绝到达时 WebKit 的焦点多半仍在禁用的按钮上）；审查 C 的 X1b 三种都红。表在 S1 的记录。
- **门禁与 CI**：实施时 `make gen-check`、`make check`（`5de7f0d0`）与 `make e2e`（`14cc9769`）全绿；修复第 1 轮、合并 `main` 之后 `make gen-check`、`make lint-web`、`make knip`、`make test-web`、`make e2e` 全绿（Go 不改）。分支 CI 每次 19 项全绿：`14cc9769` 的 run 43848588621，`137356d6` 的 43869362051，`2d707feb` 的 43877723619，合并 `main` 的 43881669843；`793c062c` 的 run 由 `concurrency` 取消。文档提交 `27ee9e58` 的分支 CI：run 43887344640，19 项第一次 attempt 全绿；合并 `d1ade2ed` 之后第一次 `main` 的 run 43892544486 同样全绿（E2E 一步 6:22）。
- **e2e 的用时**：S1 改到的十二格（设计列的九格加 LW10 (page) ×3）33.0 → 38.3 秒，多 5.3 test-seconds；九格多 4.9（推断 6–7）。`14cc9769` 的 e2e 作业 7:29（E2E 6:04），web 3:49（Test 3:07），vitest 4,591 格；修复之后 4,602 格。
- **三引擎探查**：12.3 与 28.5 在三种引擎里都是标题取得焦点；树打开的页之间后退，焦点留在树上；鼠标按"确认"，三种引擎都给它焦点。
- **照实改的各节**（02-workflow 3.5）：4.1.0.1 的表与数目、4.1.0.5、23.16（`focusLost` 一句、e2e 一句、给 Alert 的变异）、26.4、28.4 与 28.5 的变异、4.1.2 的测试与变异、9.1（"经 `useForm` 的表单"）、35.2、16.1、22.17、4.1.4、5.2 的 S1 一行、6.1、8.1、8.2 第 1、5、6 条。

### 10.2 S2：文字（2026-11-18，合并见 00 号文档第 12 节）

- **提交**：十六个代码提交与一次合并 `main`，加本文档提交；[审查记录](reviews/P2-S2-text-review.md)。
  - 第 1–9 个照 6.2 的九组；第 10 个（`4c9f1212`）只加测试，是变异里活下来的五句中文补的格；
  - 推送之前 4 号 `--amend` 两次（`780d3f46 → db764166`：`tb1-trash.spec.ts` 的格式，提交时没判 lint 的 rc；`db764166 → 7e90e1b2`：22.12 在后台恢复的那一格），8 号一次（`bf95bd2a → ebc1512e`：38.6 漏列的 `upgrade-page.test.tsx:1138-1142`）；
  - 修复第 1 轮六个（头 `62e40b25`）：替身的任务项照服务端（`a1138dd7`）、树没读到时导出仍说子页的一格（`e3671b19`）、23.11 先等这次改动的重读（`6b58eea4`）、三句改得直白（`b6215ab4`）、`page.nameAtTop` 不带笔记本名（`1580e7ca`）、Go 测试改名（`62e40b25`）；
  - 合并 `main` 的 `260d299d`（P4/S1 与提交期限的旁支的记录、导入守卫的旁支），没有冲突；
  - 三份审查 A 0I0M5N2Q、B 0I1M2N、C 0I1M6N1Q；核对第 1 轮 A、B 各一条 Nit，C 两条，都在本提交写对，不开第二轮（决定 588）。
- **做完**：B：6.1、9.5、15.3、16.6、19.1、21.8、21.11、22.11、22.12、22.13、22.16、23.1、23.3、23.4、23.11、23.12、23.13、24.7、26.11、26.13、26.14、35.1、38.6、38.8、40、41.1、41.2、41.3（28 项）；H4 的界面一句（移交 M8-history 第 4 条）。i18n 新加 23 个键、删 2 个，改文字的 en 22 个、zh-CN 27 个（修复第 1 轮多改了 `problem.page.virtual_file` 的中文；它改的另几个键是 S2 新加的，或已算过）。推后进 v0.2 的（决定 587）：设计已定的（15.3 的直接上传，M7-transfer T8；22.16 的重开会话与 23.4 的链到上级，390；23.1 的服务端英文，387，"…reverted whole"在全部被清理时同样不准，P3 或 v0.2；H4 的永久删除），与审查提出的两条：撤销被拒为 trimmed 时两句近乎重复（审查 A 的 Q2）；`AGENTS` 写的 prompt 与 MCP 实际登记的对照（审查 A 的 N5）。
- **主持者的决定**（584–588，00 号文档第 11 节）：9.5 的名字不带笔记本名，取代决定 428 的前一半；替身的任务项照服务端的解析，`- [ ]: a)b` 的不同接受；`TestLintBusyReturnsBeforeCleanup` 那一次红是 M10 测试的先后假设，另开旁支修；推后进 v0.2 的；文档的去处与收口（含总体设计 3.6 的 5 MiB）。
- **与设计不同之处**（正文照实改了，各处注"实际见 10.2"）：
  - **9.5**：`page.nameAtTop` 是 "{name} (at the notebook's top level)"、"{name}（笔记本的顶层）"，不带笔记本名（修复第 1 轮，决定 584）；`distinctName` 去掉 `notebook` 参数，十六处调用随之改。9.5 那一格照旧分开 "Notes (in Plans)" 与 "Notes (at the notebook's top level)"：根下的两页不会同名，这个名字仍然唯一。8.2 的示例随之是"Install（笔记本的顶层）"；决定 428 的前一半由它取代。
  - **40**：测试改名为 `TestTheAGENTSTemplatesNameTheUpgradePrompt`（审查 A 的 N5）：它只查模板里写的那一行，不读 MCP 实际登记的 prompt。
  - **6.1 的替身**：设计的正则把任何 `- [ ]:` 当作勾上会消失的，实施时又把任何 `- [x]:` 当作不是任务项，两样都只在"冒号之后只有一个目的地"时成立。审查 B 在服务端的解析上逐个探过（`- [ ]: a b c`、`- [ ]:`、`- [x]: /u x` 等照普通任务项，勾选照写），修复第 1 轮照它改（`a1138dd7`，决定 585），`W/test/page-server-tasks.test.ts` 十五行对照；带标题的定义与 `- [ ]x` 不模拟，说明里写明。
  - **19.1 的"树没读到时照旧说及其子页面"**：起初没有格守（审查 B 的 N1、C 的 M1）。页的菜单只在树读到之后才出现（`PageLayout` 在那之前答"未加载"），从菜单造不出这一支；修复第 1 轮的一格从探针路由打开对话框，`nodesDown` 时标题仍是"…and its subpages?"。
  - **23.11**：冲突说明原来不等这次改动的重读，重读回来之前（失败时一直）仍说 "rename it back"（审查 A 的 N4）；修复第 1 轮先等它的重读再显示冲突，另两份列表照旧不等，重读失败时照旧给旧的建议。
  - **修复第 1 轮改的文字**（审查 A 的 N1–N3）：`diff.lastBreakAdded`、`lastBreakRemoved` 的英文改成 "The only change is that …"；`transfer.restoreLeftUndone` 两种语言改得不重复（中文"分成几项"同它的两个邻居）；`problem.page.virtual_file` 的中文"变更集"改成"这本笔记本的改动"（决定 425 的同一个用词，网页的中文里不再有"变更集"），加两种语言各一格。
  - **38.6**：随之改的是 11 个测试文件 23 行，设计（与决定 388）写 10 个文件 19 行，漏了 `upgrade-page.test.tsx:1138-1142`（升级报告"未消化的来源"的大小，`2 KBCreated` 连着写，按词边界的 grep 找不到）。
  - **26.14 的 zh 一格**是"30 条。"，设计写"约 30 条。"：最后一页 `next_cursor` 为空，已读到底，计数是找到的 30，不是 about（en 同是 "Results: 30."）；测试对，设计的预期错（审查 B 的 N2）。
  - **推断错了的变异**：38.6 的"进位改成 1000，`1 KiB` 那格失败"不成立：1,024 与 1,536 按 1,000 进位，一位小数写出来仍是 `1 KiB`、`1.5 KiB`；红的是 `1,023 B`、`5 MiB`、`5 GiB`、`2,048 TiB`。15.3、23.1 的"任一键改回旧文字，对应一格失败"对中文不全成立：`unresolved.readOnly`、`upgrading`、`readerWiki`、`image` 与 `transfer.failure.changeset_trimmed` 的 zh 没有格读它，改回旧文字照过；第 10 个提交各补一格。23.11 的"条件恒真（`:233` 失败）"红在 `:187-245` 那一格，`:233` 在其中；21.11 的"恒用 `tooLargeBlock`（`:126` 失败）"红两格（`:126` 所在的一格与 "keeps texts too far apart…"）。
  - **4.2.5 第 2 条**：`transfer-background.test.tsx:176-200` 的表没有 locale 的口子，另写 zh 的一张表。
  - **38.8 的测试**：`probe` 加 `text` 参数（默认 `duplicateText`）调 `sourceHistoryText`；整句比对的是 `span.block` 里有那一句（`source_immutable` 的拒绝另有两个 `span.block`）。
  - **随之改的 vitest 多于各项列的**：9.5 的 `page-tree.test.tsx` 另两处；22.12 的 `trash-background.test.tsx` 一格；22.13 的 `trash-preview.test.tsx:303` 加一条否定断言；23.1 的 `revert-dialog.test.tsx:315`、`:320`；22.16 的 `page-lock.test.tsx` 另三处前缀。
- **4.2.5 先核对的五条**：
  1. 6.1：用 `withTasks` 的测试都写成 `- [ ] x`，替身改了之后全量照过；替身与服务端的对照见上。
  2. 22.11：没有 locale 的口子，见上。
  3. 22.13：`:203`、`:361`、`:427`、`:534`、`:572` 不变；`:303` 加否定断言。
  4. 23.1：`revert_too_large`、`revert_conflict` 的英文没有格，加在 `problem-messages.test.ts`；23.4：`transfer-report.tsx` 的冲突列表也经 `reasonText`。
  5. 35.1：`indexFacts` 在 `W/test/index-server.ts`；`health-page` 的夹具里待建页的引用数都不是 1，新格改一项。
- **先红**：各项的新测试在没修的代码上红过，`index-page` 一组（35.1）除外：它的先红没有单独一次运行的记录，证据是四条"恒假"变异（`c1-longTypes-gone`、`loaded-gone`、`inbound-gone` 红在 P4/one en，`sources-gone` 红在 P4/details），每条都等于没修的那一支（审查 C 的 N4）。审查 C 把六组的代码文件换回 `f08dca69` 的版本另跑，红的格与实施者的相同。
- **变异**：实施 114 条：vitest 111 条（代码 59、键的文字 52），106 条第一次抓到，活下来的 5 条（上面）补格之后抓到；Go 3 条都抓到；38.8 的 e2e 一半（`content` 恒真）红在 LW6 (page) `:439` 的否定断言。4.2 各项写的都在其中；另加键逐个改回 `main` 的文字（en 22、zh 26）、新键的 zh 四条、帮手各处各自不用。审查 C 加的两条（19.1 的树没读到、替身的 `- [x]:` 那一支）活下来，修复第 1 轮补格；修复第 1 轮的 15 条都抓到。表在 S2 的记录。
- **门禁与 CI**：
  - 实施时在头 `4c9f1212` 上 `make lint knip test-go-short build-web gen-check`、`make e2e`（576 过、1 跳过）与 `make test` 全绿；`make test` 第一次红在 M10 的 `TestLintBusyReturnsBeforeCleanup`（决定 586），单跑十次与整份重跑都过。
  - 修复第 1 轮：六个提交各自在干净的 worktree 里 `make lint-web`、`make knip`、`make test-web` 绿（4,788 → 4,792 格），`62e40b25` 另 `wiki` 模块的 go test；头上 `make lint knip test-go-short build-web gen-check` 与 `make e2e`（576、1）绿，`make test` 照主持者的说明不全跑（Go 只改了一个测试名）。
  - 合并 `main` 之后：`make lint-web`、`make knip`、`make test-web`（196 个文件、4,792 格）、`wiki` 模块的 go test 与 `make lint-go` 绿。
  - 分支 CI：`4c9f1212` 的 run 44242250765 第一次 attempt 18 项绿，e2e 四次卡在装 Playwright 的浏览器（apt 的镜像慢，到 20 分钟时限被取消），只重跑 e2e，第五次绿；`62e40b25` 的 44266671504 与合并 `main` 的 44279300477 第一次 attempt 19 项全绿。本提交的分支 CI：`ebb94daf` 的 run 44284479965，19 项第一次 attempt 全绿；S2 合并为 `6505c3bf`，之后第一次 `main` 的 run 44289945428 同样第一次 attempt 19 项全绿（`web` 3:58，Test 3:16；e2e 9:37，E2E 一步 8:09）。
- **e2e 的用时**：改到的五格（C4 (page)、C4 (page, notebook)、TB1 (page)、TB1 (preview)、LW6 (page)，都只在 Chromium）19.30 → 19.27 秒，差 −0.03 test-seconds（推断 0）；`4c9f1212` 的 e2e 作业 10:15（E2E 8:04），基准 10:02（8:15）；web 3:49（Test 3:11），vitest 4,694 → 4,773 格，修复之后 4,792 格。
- **读屏清单**：第 9 节的八项没有 S2 的；S2 改了念出来的两处（22.12、23.13）写在 [S2 的记录](reviews/P2-S2-text-review.md)"读屏与人工清单"，交 P5/S3。
- **照实改的各节**（02-workflow 3.5）：头；2.3 的 388 与 428（只加注）；3.1 的测试名两处；4.2.0 的 38.6 一行；4.2.1.1 的 `page.nameAtTop`、`diff.lastBreakAdded`、`lastBreakRemoved`；4.2.1.2 的 `transfer.restoreLeftUndone` 与表后一段（`problem.page.virtual_file`）；4.2.5 的五条与末段；6.1 的替身；9.5 的改法、测试、随之改、变异与取舍；15.3、19.1、21.11、23.1、23.11 的变异与 23.11 的改法；26.14 的测试；38.6 的随之改与变异；40 的测试；6.2 第 8 条；8.1 的"S2 之后"；8.2 的 S2。

## 附录：核对过的代码

各项引的代码位置都写在它那一项里，在 `86565dd6` 上读过；它的代码与 `194ca350`、`48bbfc99` 相同，`web/` 与 I3 的 `5e8af5b5` 相同。这里只列跨 Step 的、各项没写全的几样。

- **e2e 的配置**：`e2e/playwright.config.ts:3-4` 是 `threeBrowsers` 的正则，`:24-28` 是项目。`e2e/stories` 有 15 个分组、123 个 `*.spec.ts`、386 处 `test(`；`make e2e` 577 个。
- **CI**：`.github/workflows/ci.yml:265-271` 是 e2e 作业（20 分钟），`:232` 是 `web` 作业（15 分钟）。最近的用时见 7.4。
- **`make lint-web`**（`Makefile:159-164`）：
  - 已跑 `tools/md-fixtures/check.mjs`、`tools/e2e-navigation/check.mjs`、oxlint、格式与各包的 tsc（含 e2e 包）；
  - P2-44 的检查加在这里。
- **vitest**：192 个文件、约 1,922 处 `test(` 或 `it(`。
- **跨文件的共用件**：
  - `W/app/confirm-dialog.tsx`：17 个文件、25 处用它。
  - `useForm`：19 个文件、20 次调用。
  - `W/app/arrival.ts`：S1 改它，S6 的 21.3 用它（0.1、4.6.8）。
  - `app/` 今天只经 `routes.tsx` 动态导入 `pages/`。S1 的 28.4 第一次静态导入 `W/pages/page/readers-input.ts`，oxlint 没有拦这一条的规则（审查 A 核对过）。
- **设计阶段的探针**：15.2，三种引擎（Playwright 1.63.0，无头，Desktop Chrome/Firefox/Safari）。设计者的 `m12/p2-probe-15.2/probe-toggle.spec.ts` 与 `run1.txt`（上一版听 `toggle` 的改法）；修订核对 C 的 `m12/p2-check1-c/probe.spec.ts` 与 `probe.log`（`fix=timeout` 是现在的改法，另有 I、J 两种程序里的打开）。不进仓库；结论写在 2.3 的 P2-42 与 4.5 的 15.2。
