# M13/P5/S3 版本、健康、升级、附件与对话框：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p5-s3`（从 `m13-ui-refresh` 的 `293d016b` 拉出，可快进），九个提交：`51125487`（照 6.3 提交 1：版本页照页排头一行，"还原这个版本"在头一行，"当前"同右栏的标，右栏的标接在作者之后；健康与升级页是卡片）、`066c84b1`（照 6.3 提交 2：附件与上传是卡片里的行，"操作"悬停或聚焦时显出，下载有标；对话框里问的框、粘贴来源的文本框与标签组；升级横幅在每个笔记本页的列的最上面、前面一个琥珀的小圆点，布局里的另一处去掉）、`22c693f8`（照实加的：变异 t2、t8、t13、t21、t23a、t23b、t42、t51 活下来之后补的类名断言，只改测试）；第一轮审查之后的 `9de4aed9`（修复：问的框的按钮放不下时各占一行，还原那一格不窄于按钮，健康卡片的 sr-only `output` 与"更多"，版本行的时间与种类各在一行里，升级页两节的字号）、`9e6594a5`（只改测试：cxa、cxb、cxc、cxx 的断言，957 的来源页一行，hi2 在 320 宽按位置量的三句）、`e8aa3b36`（决定 958 扩到别的对话框里放不下的三行）；修复核对第 1 轮之后的 `ff3e3788`（"Stop the upload"不理双击的第二下，`output` 在标题之后，`expectButtonsInside` 等弹入结束再严格比较）；第 2 轮之后的 `35bcc7bc`（决定 960，按住的 Enter、空格，tr2 的计数为 2 的点按，0.01 px 的浮点余量）；第 3 轮之后的 `8a89e184`（按住的键在整个导入表单上只按一下，格 B 从上传开始算，tr2 记下每一下点按落在哪里）。另有本记录所在的文档提交。 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的分离工作树里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为与正确性（读 diff 与设计；vitest 探针 10 格与 StrictMode 下 5 格，浏览器探针 A1–A5，三种引擎；修复核对时 F1–F5、G1–G4 两套探针，第 3 轮量 960 与按住的键、另做 4 条变异，第 4 轮量按住的键与点击记录、另做 3 条变异），B 管外观（自己的探针，三种引擎 × 亮暗 × 五种宽度 × 强制色 × 中英，三种长名字与 e2e 式的长作者名；修复的试验另开一棵树，`fix-experiment.patch`；修复核对第 1 轮整套重跑，另量 958 扩到的三行与 45 / 44 对按钮文字），C 管测试、e2e、变异、门禁与文档的就绪（逐格读，重放实施者的变异，自己的变异与探针，先红的复现，合并之前的整条命令，8.2 的替换在 `293d016b` 的副本上重放；每一轮修复核对都再跑门禁、重放变异、连跑，并定文档的最后文字）。修复核对四轮：第 1 轮三位；第 2–4 轮只请 A、C（B 那边没有看得见的改动：第 2 轮改的 `output` 是 sr-only、只动 DOM 次序，之后是行为与测试）。 |
| 结论 | 实施中：变异 t2、t8、t13、t21、t23a、t23b、t42、t51 在 `066c84b1` 上活着，照实加 `22c693f8`（同 S2 `75578e08` 的先例）；49 在最后一个提交上是空操作，用代理；52 红法与设计写的不同。第一轮：A 1 Nit、2 Q；B 1 Important、1 Minor、3 Nit、2 Q；C 2 Minor、5 Nit、3 Q。同一件的：A-Q1 = B 的偏离 5 = C-Q1；A-Q2 = B-Q1；A-N1 = B-Q2。全部接受（B-N3 交给收尾），主持者的决定 955–959，主持者另把 A 的一条未核实的怀疑加成 Minor；修复 `9de4aed9`、只改测试的 `9e6594a5`；修复之后主持者把 958 扩到别的对话框里放不下的三行，`e8aa3b36`。修复核对第 1 轮：A 1 Minor、2 Nit；B 没有新的发现（两处观察）；C 1 Important、3 Nit、1 Q；全部接受，`ff3e3788`。第 2 轮（A、C）：A 1 Q、2 Nit，并撤回实施者焦点核实的结论；C 3 Nit（C-F2-N2 = A-F2-N1）；全部接受，决定 960，`35bcc7bc`。第 3 轮（A、C）：A 1 Minor、1 Nit；C 1 Minor；全部接受，`8a89e184`。第 4 轮（A、C）：没有新的发现（A 顺带看到两件 S3 之前就有的，交给收尾）。三位都没有未了的 Important、Minor。合并 `75f020a4`。 |

## 基线

S3 不另跑基线（05-P5 7.3，同 P4 的决定 883）：`m13-ui-refresh` 是 `293d016b`（P5/S2 合并 `d2c1c8ee` 之后记合并的文档提交），提交 1 的 `make e2e` 兼作核对。对照的数是 S2 之后的：vitest 233 个文件、5564 格；E2E 647 格（643 过、3 败、1 跳）；`check.mjs` chromium 434、firefox 105、webkit 105、webkit-narrow 3。已知的本机失败同 [P5/S2 的记录](P5-S2-lists-review.md)：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S3 的每次 E2E 里 SE2、LW8 都红，同行同消息（S3 改了健康页的类名，Tab 的次序不变，lw8:836 的行号与消息没有变）。PG15 在提交 1、2 的 `make e2e` 与 `ff3e3788` 的两次（`make e2e` 与合并之前的整条命令，644 / 2 / 1）里过了，实施者其余的十次与审查 C 的五次都红。没保留的 `8280a72a` 那次门禁另红两格 LW3，见"偶发"与"主持者的决定"。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 变异 t2、t8、t13、t21、t23a、t23b、t42、t51（都是只改样子的类名：健康卡片标题的字号、路径缺时的 `span`、"Rechecking health…"、"Being checked…"、检查与报告两处"状态变了"、右栏时间与作者那一块的 `break-words`、拖放说明）在 `066c84b1` 上的全部相关测试下活着 | 照实加 `22c693f8`：6 个文件，都加在已有的格里，没有新格；8 条在新提交上重跑都红（`p5s3/mut/results-again.tsv`） |
| 变异 49（版本页的路由不加 `handle`）：提交 2 删了各路由的 `handle: { column: true }`，在最后一个提交上是空操作 | 实施者用代理（`VersionShell` 不画 `ColumnTop`），红在表的两行；第一轮之后 C-Q2：两种都记（"变异"一节） |
| 变异 52 的红法与设计写的不同：不是 `upgrade-pages.ts:31` 的 strict mode，而是 `up5:252` 的 `toHaveText` 多读到根里的 19 项 | 照实；vitest 另跑 52v；C-N4 照实测改 7.2 |
| UP1 (page) 的版本页一步在旧树上是绿的：旧树的回退把横幅画在出口之上，读取中与读到之后同一处 | 同 S2 的预览、变更集两步，由 t32、t33 证（`up1:319`，三种浏览器）；实施者的偏离 4（测试名照实加 "and a version's page"） |
| as1 加的那一句在旧树上是绿的（旧树"操作"一直显示）；表在提交 2 里不先红 | 由 56 证（`as1:187`）；表由 57、58、61、fb1、fb2 证（设计 4.3.6 说的） |
| 设计按 `f62d0390` 写的 `hi2:114`，在 `22c693f8` 上是 `hi2:124`；84 在 hi1 上红，hi2 那一句没有抓手 | 实施者另跑 84h2（同一变异跑 hi2），红在 `hi2:124`；C-N4 之后行号照最后的树（`hi2:146`） |
| t49（"操作"不带 `revealClass`）：vitest 红，as1 那一句照样是 1 | 对故事等价，照实不算它的抓手（as1 只在菜单开着时量，去掉 `revealClass` 之后恒为 1；C 核对） |
| 实施者的视觉探针里"reader"那个上下文其实是有编辑权的成员（工作区成员加开放程度 editor 的笔记本），`version-reader-*` 的截图是第二个写者的 | 照实；读者的头一行由 `version-page.test.tsx` 的 `test.each` 与 UP1 守；A 的 A1 以真读者核对 |
| 修复轮：处置要的 e2e 里没有现成的故事会打开粘贴来源的问的框 | 加进 lw3:287 已有的一段（先填草稿、Esc 出问的框、量完点"Discard and close"），三种浏览器；导入的加进 tr2 现成的离开时问的那一段（选择 1） |
| 修复轮：958 的 grep 找到 10 行同样写法的按钮行，S3 都没碰过；英文 320 宽放不下的有 3 行（`import-dialog.tsx:395` 在 S3 碰过的文件里，这一行本身没碰） | 实施者照处置不改、列出、请主持者判断；主持者把 958 扩到这三行，`e8aa3b36` |
| 修复轮：变异 rc 在 hi2 里活着：本机字体下按钮 150 px，放得进 160 px 的格 | 由 vitest 的类名与 letter-spacing 的探针证；C 的 rcw、w（修复核对第 1 轮）证 hi2 的位置断言是活的；等 CI 的 Linux 字体 |
| 修复轮 3：因 960，三格在上传刚开始就用指针点 Stop，那一下不再算 | 三格点 Stop 之前把假的 `performance` 推进 500 ms，其余断言不动；先在新代码上红 3 格（日志被下一次覆盖） |
| 修复轮 4：oxlint 的 `jsx-a11y/no-noninteractive-element-interactions` 对 `form` 上的 `onKeyDown` 报警告，lint-web 要零警告；表单里没有文本框 | 一行 `oxlint-disable-next-line` 带理由；"按住空格照旧打字"改为"文件框与 select 上重复的空格不被拦"（"主持者的决定"） |
| 提交之前没过的门禁：修复轮第 1 个提交在本地两次没过（lint 先后抓到 `page-versions.test.tsx` 的 `by` 与 `health-rule.test.tsx` 的 `ends`；LW3 的 0.07 px） | 改了才提交（"主持者的决定"里重做本地提交的那一句） |

## 偶发

- **lw8:903 [webkit]**（新的一种）：提交 1 的 `make e2e` 里一次 `browser.newContext: Target page, context or browser has been closed`（113 ms，负载约 26），紧接在 :822 那一格失败之后；单独重跑 LW8 [webkit]，:903 过，只剩 :822 那一格在 lw8:836 红（同行同消息）。之后实施者的十三次与审查 C 的五次都没有再出现。原因没有查（C：像是 WebKit 的 worker 在前一格失败之后重启时撞上的）。记为偶发（7.3）。
- **up3:281 的时红时过**不是程序的偶发，是帮手在对话框弹入时量（P5S3C-F1-I1）：`e8aa3b36` 的 `expectButtonsInside`（不等弹入，带 0.5 px 的容差）在 up3:281 上时红时过：审查 C 的探针（只在同一次 `evaluate` 里多记 `scale`，量法不变）一次 8 次红 2 次，两次都量在弹入的第 0 ms（`scale` 0.97，最右的按钮超出内边 0.75 px）；照原样，实施者 8 次、C 在修复核对第 2 轮 16 次都过；只严格、不等弹入时 8 次全红。可见每一次都量在弹入之中，红不红只看落在哪一帧（只有头约 12 ms 超过 0.5 px），由机器的忙闲定，不是哪一次能复现的。`ff3e3788` 先等弹入结束、再严格比较之后，up3（实施者 8 次、C 16 次）与 lw3（三种引擎，实施者各 2 次、C 各 4 次）都过，量到的内边是整数（41、58）。
- 没保留的 `8280a72a` 上 LW3 [chromium]、[webkit] 的 0.07 px 是同一个原因（C 第 1 轮的探针：lw3 量的时候弹入还在跑，Chromium 50 ms 时 `scale` 0.995748，超出 0.0723 px）。
- 别的没有：AS6、`lw8:771`、"not slower"、`trash-preview.test.tsx:587`、造数的 500、WebKit 的"mermaid could not be loaded"一次也没有遇到。

## 第一轮的发现与处置

在 `22c693f8` 上（对照 `293d016b`）。

### 审查 A（行为与正确性）

| # | 级别 | 发现 | 处置与修复提交 |
|---|---|---|---|
| P5S3A-N1 | Nit | 写者直接打开来源页的版本（粘贴地址、重新载入、从别的笔记本来）：早退两支的头一行按 `writesNotebook` 有"还原"那一格，读到之后按 `canEditPageBody` 没有（来源页对谁都是 `false`，当时 `version-page.tsx:89-93`、`:205`）；375、320 宽空头一行折成两行、读到之后一行，列的上边 153.5 → 134，三种引擎逐个相同（A2）；写者看不到横幅（升级中 `writesNotebook` 是 `false`），挪动的只有"加载中…" | = B-Q2；决定 957：代码不改，`9e6594a5` 在早退的 `test.each` 加来源页一行（选择 4），钉住"早退有那一格、读到之后没有"；变异 957 红在 `version-page.test.tsx:865` |
| P5S3A-Q1 | Q | 偏离 5：`clearSummaryClass` 也用在升级的检查与报告（`upgrade-check.tsx:258`、`:389`）：行为不变（同一个 `details` 与 `summary`，照旧 `cursor-pointer`、`clearText(...)`、下面同一个 `HealthRule`），只有样子 16 px、500 → 13 px、600、灰，点击的高约 24 → 19.5 px | = B 的偏离 5 = C-Q1；决定 955，`9e6594a5` 补 cxc 那一句（`upgrade-page.test.tsx:1120`） |
| P5S3A-Q2 | Q | 偏离 6：升级页的说明两种大小并存（`noteClass` 13 px 与正文 16 px）；实施者"说明与内容"的分法讲得通 | = B-Q1；决定 956（两节 `text-sm`），`9de4aed9`；956a、956b |
| 主持者加（A 的未核实的怀疑） | Minor | "还原这个版本"在 Linux 的字体下可能比那一格宽：那一格是 `flex-[0_1_10rem]`（160 px）带 `min-w-0`、`justify-end`，按钮超过 160 px 时从格的左边溢出；macOS 的 Chromium 上按钮 150.5 px；hi2 的 `expectFits` 只看出不出屏 | `9de4aed9`：`restoreCellClass` 改为 `cn(headActionsClass, "min-h-7.5 min-w-fit flex-[0_1_10rem]")`（`version-page.tsx:119`），`cn` 替掉 `min-w-0`，10rem 的基宽与 `min-h-7.5` 照旧（决定 929，P5A-M4）；`9e6594a5`：hi2 在 320 宽按位置量还原在它那一格之内、不压面包屑（`hi2:118-119`、`:124`）；letter-spacing 的探针；变异 rc、rcw、w |
| A 另记 | — | 偏离 1–9 逐条核对：都接受，3、5 带条件（3 补 N1 的半句，5 在 4.3.4 照实写明）。读者两处都没有那一格（vitest 探针与 A1 的 375、320 宽，读者是工作区成员、开放程度 viewer 的笔记本）；升级中谁都没有那一格，有横幅时空头一行与读到之后一样（UP1 的版本页一步与 A5）；头一行的 Tab 次序 Plans → Guide → Restore this version → 作者 → Read（A3，375 宽，三种引擎）；还原对话框取消之后焦点回到头一行里的"还原"（A3、StrictMode）；作者的链接、右栏的版本、树读取中经 `arrived` 到达，`h1` 拿到焦点并留住（StrictMode；A1 点过之后再等 500 ms 仍在标题上）；附件"操作"静止 0、焦点在本行的链接上 1、在它自己上 1、离开 0（A4，三种引擎，WebKit 以 `focus()` 补上实施者没截到的一格）；去掉回退之后笔记本下每个路由、每一支恰好一个横幅、在列的最前（A5 与 vitest 探针）。另一条未核实：上传中的行比附件的行高约 11 px（归 B，没量） | — |

### 审查 B（外观）

| # | 级别 | 发现 | 处置与修复提交 |
|---|---|---|---|
| P5S3B-I1 | Important | 粘贴来源、导入两个对话框里"问的框"的两个按钮在 320 宽溢出、被裁（当时 `source-add-dialog.tsx:140`、`import-dialog.tsx:142` 的 `flex justify-end gap-2`）：框的内容宽 204，两对都要 276（117 + 151 + 8；136 + 132 + 8），左边的按钮 left −14，正是框出现时取焦点的那个；375 宽内容宽 259，按钮压过内边距与左边框；中文放得下；这一行的布局类 S3 之前就是这样，S3 只换了框（决定 934），框是 7.4 点名要看的 | 决定 958，`9de4aed9`：两处 `flex flex-wrap justify-end gap-2`（今天 `source-add-dialog.tsx:141`、`import-dialog.tsx:143`）；vitest 两句类名；e2e 在 320 宽 `expectButtonsInside`（lw3:366 三种浏览器；tr2，今天 `:311`）；958a、958b；修复之后扩到三行（`e8aa3b36`）；帮手先等弹入（`ff3e3788`，C-F1-I1）、留 0.01 px（`35bcc7bc`，C-F2-N1） |
| P5S3B-M1 | Minor | 健康的卡片没有"更多"时下边 25、上边 17：sr-only 的 `output` 是 `section` 的最后一个子元素（当时 `health-rule.tsx:272`），`space-y-2` 给看得见的最后一块多 8 px；升级页的检查与报告用同一个组件 | `9de4aed9`：`output` 挪成第一个子元素，17 / 17；vitest 新的一格；m1；修复核对第 1 轮 A-F1-N1 之后放在标题之后（`ff3e3788`，今天 `health-rule.tsx:254`） |
| P5S3B-N1 | Nit | 健康卡片里的"更多"/"重试"是 36 px、8 px 圆角，不是胶囊（00 号文档 4.3：卡片里的按钮是胶囊）；右栏版本的"更多"带 `small` | `9de4aed9`：`LoadMoreButton` 带 `small`（今天 `health-rule.tsx:279`），30 px 胶囊；n1 |
| P5S3B-N2 | Nit（S3 之前就有） | 中文右栏版本行：时间或种类的一个字单独占一行（Chromium 在 xl 的 217 px 上"现 / 在…"与单独的"辑"，196–244 逐 1 px 每个宽度都有；Firefox 209–211、WebKit 200–202）；英文 0；实施者的扫描只数单独的"-"与断开的小块 | 决定 959，`9de4aed9`：时间的 Link 加 `whitespace-nowrap`（今天 `page-versions.tsx:121`）；`VersionBy` 的名字照 B 的 f1 写成 `Isolated`（带" · "），种类包进 `whitespace-nowrap` 的 `span`（`:199`）；n2a、n2b |
| P5S3B-N3 | Nit（P4 的） | Firefox 320 宽、连字符加数字的长名字：版本页头一行的面包屑折成几行时有一行只有"/"（`breadcrumbs.tsx:48` 起的 `BreadcrumbTrail`，页与变更集同一个组件） | 交给收尾（05-P5 第 9 节） |
| P5S3B-Q1 | Q | 偏离 6：升级页"Where it is""Connect an agent"两节的正文 16 px，与节标题同号；同页别的都是 13–14 px，settings.html 的行 14 px | = A-Q2；决定 956，`9de4aed9`（`upgrade-page.tsx:634`、`:673` 带 `text-sm`） |
| P5S3B-Q2 | Q | 偏离 3 的后果：写者打开 LLM Wiki 来源页的版本，树扣住时头一行是一行占位（19.5）加 160×30 的那一格，320、375 宽 55.5，读到之后那一格没有了；1440 宽前后都是 36；面包屑只有一行时推算 `h1` 上移约 36 px，没有量 | = A-N1；决定 957，`9e6594a5` |
| B 另记 | — | 偏离 2、5 接受，6 一半（说明用 `noteClass` 接受；两节正文见 Q1；"Check"的 36 px、8 px 圆角同页的 Submit、Abandon，是表单底部的动作，接受；健康页的"Recheck"在标题行，30 px 胶囊，两者不同是对的）。版本页的头一行三种浏览器 320、375 宽 55.5、`h1` y 153.5，768、1440 宽 36、y 78，树扣住、版本列表扣住、读到、写者看最新版本都相同；320 宽"还原"x 145.5、y 93.5、150.5×30。右栏英文 196–244 逐 1 px 没有单独的"-"、断开的小块、溢出。横幅读取中与读到之后 x、y、宽严格相等（变更集 768、375 宽例外，S2 已接受的面包屑折行）；小圆点 6×6、3 px 边，中心与第一行字形中心差 ≤ 0.5。所有视图没有页的横向溢出，只有阅读视图 `div.space-y-4` 里 4 px 的 `scrollWidth`（P4 的，看不见、不出滚动条）。对比度与实施者同、不低于样稿自己的（"当前"2.757 对样稿 `.ver .tag` 2.658；小圆点 2.818 对 `.meta .dot` 2.717）。`cn` 合并的结果逐个核对（`cn.json`）。I1、M1、N2、Q1 的修复在另一棵树上试过，只跑了探针 | — |

### 审查 C（测试、e2e、变异、门禁与文档的就绪）

| # | 级别 | 发现 | 处置与修复提交 |
|---|---|---|---|
| P5S3C-M1 | Minor | hi2 的 320 宽一段没有核对"还原在头一行、标题之上"，注释（"Restore in the head's row above it"）与 8.2 第 15 条的新文字却这样说：断言只有 `:102` 的 `lines` 与 `:103` 的 `expectFits`；C 的 cxr（还原挪到标题之下）下 vitest 红 4 格，hi2 照绿 | `9e6594a5`：hi2 补按位置量的断言（今天 `hi2:117`，还原的下边不超过标题的上边；与主持者加的两句一起）；cxr 红在 `hi2:117`（237.5 对 ≤ 153.5）；8.2 用全句 |
| P5S3C-M2 | Minor | 8.2 第 15、16 条漏了 UP1 (page) 的版本页一步（`up1-start-freeze.spec.ts:311-320`，偏离 4；t32、t33 都红在 `up1:319`）；S2 的同一种情形是 P5S2C-M3 | 本文档提交：C 的文字，5.5 补 UP1；958 的 e2e 一段照实加进同一条与 13.4 第 3 条（8.2 的编号照 C 第 4 轮定的第 11–17 条） |
| P5S3C-N1 | Nit | 设计 4.3 的四处只改样子的类名没有格守，各自整套 233 / 5592 全绿：cxa（`health-page.tsx:319`"模式变了"）、cxb（`:333` 分页说明；这一句的出现本身 S3 之前就没有格读过）、cxc（`upgrade-check.tsx:389` 报告一节的 summary）、cxx（`source-add-dialog.tsx:279` 出错的说明） | `9e6594a5`：都加在已有的格里（cxx 那一格扩成"标题与正文都被拒"、改了格名，选择 5）；四条都红（`health-rule.test.tsx:331`、`health-page.test.tsx:540`、`upgrade-page.test.tsx:1120`、`source-add-dialog.test.tsx:228`） |
| P5S3C-N2 | Nit | 报告与 10.3 草稿里几个先红的数不准：表的 8 行都红在第 2 句的 `mx-auto`，不是"第 1、2 句"；version-page 是 10 格不是 9 格；提交 2 的"6 个文件 7 格"那次跑的树不是没改的，真正的先红是 7 个文件 10 格 | 本文档提交：照 C 的更正写进 10.3 与本记录（"门禁与 CI"的先红一表） |
| P5S3C-N3 | Nit | 10.3 草稿的"做完"缺 916、925、937 里 S3 的部分；"变异"缺 49 照设计写法在 `51125487` 上的结果与 C 的 cxa、cxb、cxc、cxx、cxv、cxr | 本文档提交 |
| P5S3C-N4 | Nit | 文档里会过时的行号（`hi2:114` 在 `22c693f8` 上是 `:124`；as1 的 `:183` 是 `:186-190`、红在 `:187`；`upgrade-banner.test.tsx:209-220` 已删；表的 S3 各行在 `:246-260`）与 7.2 要照实改的 49、50、51、52、56、84 | 本文档提交：行号照最后的树（hi2 那一句今天 `:146`），7.2 照 C 的实测 |
| P5S3C-N5 | Nit | 8.2 第 11 条的"各节照旧"读起来像升级页别的都没变：检查与报告里的各条规则同健康是卡片 | 本文档提交：照 C 的写法（另加 956 的"字号同设置页的行"） |
| P5S3C-Q1 | Q | 偏离 5 留不留 | 决定 955：留，补 cxc |
| P5S3C-Q2 | Q | 7.2 第 49 行记哪一种 | 主持者定：两种都记（"变异"一节） |
| P5S3C-Q3 | Q | 6.3 提交 3 的"记在 P4/S1 那一行之后"指 04-P4 第 9 节"P5"一条，还是 M12-polish 里 P4/S1 那一行之后 | 主持者定：照 C 的读法，指 04-P4 第 9 节的"P5"一条 |

C 另核实：`22c693f8` 上合并之前的整条命令（"门禁与 CI"）；`51125487`、`066c84b1` 的说明与 6.3 逐字相同（脚本比对），`22c693f8` 写作 `web: … (M13/P5/S3)`、只动 6 个测试文件，三个都以 Co-Authored-By 结尾；8.2 第 11–16 条的旧文字在 `293d016b` 的总体设计上依次各恰好一次（`replay.py`），第 11–14 条的新文字与实现相符；横幅表新加的 8 行各读对的那一支，删掉的"版本页的横幅不在列里"那一格守的正是回退的样子，净加 7 格，笔记本下的 12 个路由每一个 `return` 都在列里画 `ColumnTop`；`version-page.test.tsx` 的 10 格否定都在肯定之后断言，读者那一行靠 `row.children` 的长度抓那一格；`22c693f8` 补的 8 条都红在说好的格；"由谁证"的说法都核过（UP1 的版本页一步由 t32、t33，as1 那一句由 56，提交 2 的表由 57、58、61、fb1、fb2，都红在说好的那一步）；knip 与孤儿：`clearSummaryClass` 由 `health-page.tsx:26`、`upgrade-check.tsx:28` 导入，`sectionTitleClass` 由 `upgrade-page.tsx:31` 导入，代码里已没有 `column: true`、`ColumnHandle`，`NotebookLayout` 不再用 `useMatches`；受保护的路径没有改动，`git diff --check` 干净，并入是快进。

## 过程的事故

- 实施者的视觉探针第一次在附件一节卡住：Radix 的模态菜单开着时行被 `aria-hidden`，按角色找的 locator 等到超时；停掉之后只补跑后半（四次运行，`measures.json` 之外另有 `extras*.json`）。
- 修复轮的第 1 个提交在本机重做过两次，见"主持者的决定"里的那一句；之后的四个修复提交都没有在本地重做，门禁一次就过。
- 实施者核实焦点时，探针把自己工作树的 HEAD 临时 detach 到要测的提交（`e8aa3b36`、`22c693f8`、`293d016b`），跑完回到 `m13-p5-s3` 并确认工作树干净；A 修复核对第 2 轮把自己的工作树临时切到 `293d016b` 对照，C 第 2 轮把跑变异的工作树临时 detach 到 `e8aa3b36`，都切回并核对干净。
- 修复轮 3：因 960 改的三格第一次在新代码上红（46 格 3 红）的那次日志被下一次覆盖，`p5s3/fix4-vitest.log` 是 46 格全过的那一次；实施者在报告里照实写了。
- A 第一轮：第一次浏览器探针里 A1、A2 的"读取中"量早了（量到外框在笔记本列表读到之前的"加载中…"，得 −1，`run1.log`），第二次改成等列里的"加载中…"之后只跑 A1、A2。A 修复核对第 2 轮：第一次探针因探针自己的写法卡住，A 核对命令行之后停的是自己的进程。
- 别的没有。C 的五次门禁都没有等锁；实施者与三位审查者都只停自己起的进程；临时的探针都在跑时拷进 `e2e/stories/shell/`、跑完即删，几棵树最后的 `git status` 都是空的，审查者的工作树用完都已删。

## 实施者交来的（主持者定）

| 实施者报告 | 处置 |
|---|---|
| 第 7 节偏离 1：多一个只改测试的提交 `22c693f8` | 接受（A、C；S2 `75578e08` 的先例） |
| 2：`Breadcrumbs` 收 `className={headCrumbsClass}`，少一层 `div` | 接受（A、B：`page-layout.tsx:473`、`changeset-page.tsx:264` 同样写法，Tab 的次序照旧），4.3.1 照实 |
| 3：早退两支的 `head` 按 `writesNotebook(notebook)` 定那一格 | 决定 957 |
| 4：UP1 (page) 加版本页一步（1440、768、375），测试名照实加 | 接受（A：决定 951 的版本页那一半在浏览器里唯一的守卫，t32、t33 只在它上面红；`check.mjs` 的数不变）；8.2 与 5.5 补上（C-M2） |
| 5：`clearSummaryClass`（`health-rule.tsx`）与 `sectionTitleClass`（`upgrade-check.tsx`）导出，前者另用在升级的检查与报告 | 决定 955 |
| 6：`noteClass` 取检查的说明、两处"状态变了"、报告的两句，"Where it is""Connect an agent"的正文照旧 | 说明的分法接受；两节正文决定 956 |
| 7：门槛的 `li` 用 `stackedLinesClass` | 接受（A：带的 `group/row` 无害，门槛的行里没有 `revealClass`；B 核对与列表页的行一致） |
| 8：7.1 之外多的类名格 | 接受（A） |
| 9：49 用代理，52 的红法不同 | 接受；49 两种都记（C-Q2），52 照实（C-N4） |
| 给审查者的：lw8:903 [webkit] 一次新的偶发 | 记为偶发（7.3） |
| 给审查者的：能编辑的人看最新的版本时 375 宽空着的那一行与有按钮时同高（55.5） | 接受（A、B 核对） |
| 给审查者的：探针里的"reader"是有编辑权的成员；WebKit 里键盘聚焦时"操作"没截到 | 照实；A 的 A1 以真读者核对，A4 以 `focus()` 量到 1 |
| 10.3 草稿"交给后面"：升级页两节与检查说明的字号要不要统一；"Check"要不要同 Recheck 用小胶囊 | 字号由决定 956 定；"Check"照 B 的判断照旧；都不再交出 |
| 10.3 草稿"交给后面"：Linux 字体下 hi2 的 320 宽一段、右栏版本行、横幅小圆点，UP1 (page) 版本页一步 | 等负责人推送之后的 CI（"门禁与 CI"） |
| 修复轮的报告（`p5s3/report-fix1.txt` 的五个部分）：选择、958 的 grep、rc、重做本地提交、焦点的核实、因 960 改的三格、表单上的 oxlint 一行、字段的重复 | 见"主持者的决定"里"修复轮之后主持者定的"与"撤回的焦点核实" |

## 修复核对第 1 轮

在 `e8aa3b36` 上（对照 `22c693f8`；修复 `9de4aed9`、只改测试的 `9e6594a5`、958 扩到三行的 `e8aa3b36`），三位各在自己的分离工作树里（经 `heavy.sh`）。

- **A**：自己的三条都修好。957：早退的 `test.each` 加了来源页一行（`version-page.test.tsx:831`，`mode: "llm_wiki"`、`sourceRootId: guide.id`），浏览器 F3 里来源页 375、320 宽读取中 153.5、读到之后 134，1440、768 宽不动。955：`upgrade-page.test.tsx:1120` 断言报告一节的 summary，cxc 红。956：`upgrade-page.tsx:634`、`:673` 是 `space-y-2 text-sm`，`h2` 照旧 `sectionTitleClass`，"Connect an agent"里的 `code` 本来就带 `text-sm`。958 的五行（两个问的框、`ConfirmDialog`、导入的按钮行、撤销的按钮行）DOM 次序不变，Tab、默认焦点、Escape、`aria-*` 都不变；F5（升级页 Abandon 的确认框，320 宽英文）三种引擎打开时焦点在 Cancel，Tab 到"Abandon the upgrade"，Cancel 在上 `[200..279]`、Abandon 在下 `[102..279]`；发送中两行变短、排回一行（"Abandoning…""Starting…"），框矮 44 px，原来那一点下面是 Cancel，无害：发送中两处的关闭都挡着（`confirm-dialog.tsx:136` 的 `busy.current`、`revert-dialog.tsx:154` 的 `!sending`）。`min-w-fit` 前后 F3 的表逐个相同（本机按钮 150.5 px，小于 160）；`output` 挪到最前之后 `role=status` 与播报不变、17 / 17；`VersionBy` 拆开之后三种引擎的 `ariaSnapshot` 不变。新的三条见下表。另记一条未核实（Chromium 里 Abandon 的确认框按 Escape 之后读到 `body`），见"撤回的焦点核实"。
- **B**：五条（I1、M1、N1、N2、Q1）都修好，三种引擎、亮暗、各宽度、中英都看过：问的框 35 种情形 × 三种引擎都在框内，英文 375、320 各占一行、靠右（Keep 在上），左边那个按钮从 −14 变为 145.5（Keep editing）与 125.8（Keep uploading），中文三种宽度都是一行；健康页 4 张卡片与升级页"检查"之后 64 张卡片都是 17 / 17，其中有条目没有"更多"的三张（最后一块是 `UL`）也是，第一轮那条未核实的核实了；"更多"30 px 胶囊；版本行 196–244 逐 1 px 单独一行的一个字（含"·"）、单独的"-"、断开的小块、溢出都是 0；两节正文 14 px、正文色。958 扩到的三行：真实打开的 Abandon、Approve、Reject 的确认框，导入上传中，后台撤销（把撤销的 POST 答成 422 `page.revert_too_large`），英文 320 两行、靠右、在框内，宽时一行；`ConfirmDialog` 的别的用法（附件的删除、撤销令牌、页的删除）照旧一行；标签对的检查英文 45 对、中文 44 对（另补了实施者没列的 `invitations-section.tsx` 的 Withdraw），英文 320 Chromium、WebKit 6 对两行、Firefox 7 对，都在框内。`min-w-fit` 前后与第一轮的 `head.json` 逐项相同（四种宽度 × 七种状态 × 三种引擎），letter-spacing 撑长到 0.5em 都在格内、不压面包屑、在标题之上。新的问题没有；两处观察：Firefox 320 宽"开始升级"的确认（Cancel + Start the upgrade，238.1 对内宽 238）折成两行，修之前是看不见的 0.1 px 溢出；折成两行时五处都是"安全的在上、要做的在下"。另量了 768 宽的来源页（决定 957）：读到之后 `h1` 从 78 到 81。
- **C**：门禁 247 s，只败三格基线；实施者的变异重放都同格同数（958a、958c、958d、958e、rc、cxr、m1、n1、n2a、n2b、956a、956b、957、cxa、cxb、cxc、cxx、cxv），另加 958b2、rcw、w；新格逐格读过（`expectButtonsInside` 按盒子量、查框里所有的 `button`；lw3 改了的走法里"空的框按 Esc 就关"由 vitest 守；`listed()` 收窄之后没有失去实际的守护）；三个提交的写法 `web, e2e: … (M13/P5/S3)` 加 Co-Authored-By（README:104 字面上说审查修复只写到 Phase，M13 的 S1、S2 的修复都写到 Step，同先例，不算发现）；两个重做过的提交父都是 `22c693f8`、没推送，与 `9de4aed9` 只差测试；文档在 `293d016b` 的副本上重放（`replay-f1.py`）。新的五条见下表。

| # | 级别 | 发现 | 处置与修复提交 |
|---|---|---|---|
| P5S3A-F1-M1 | Minor | 958 扩到导入的那一行之后，320 宽英文下双击"Import"，第二下停掉刚开始的上传：发送时这一行变成"Stop the upload"+"Uploading…"（约 260 px，可用 238），折成两行，Stop 正好挪到原来 Import 的下半截之下（F1：Import 在 y 650..685，Stop 在 667..703）；`:408-409` 防的是反方向（Cancel 取代 Stop） | `ff3e3788`：Stop 的 `onClick` 遇到 `event.detail > 1` 返回，同 Cancel；vitest 一格（`detail` 2 不停，`detail` 0 停、Import 回到可用）；tr2 先切到 320 再在 Import 上 `dblclick`，核对仍在传（返回时照样问）；f1m1 红在 `transfer-import.test.tsx:538` 与 `tr2:261` |
| P5S3A-F1-N1 | Nit | `output` 挪到最前之后，读屏浏览时"已多显示 N 项"读在卡片标题之前 | `ff3e3788`：放成标题之后的第二个子元素（今天 `health-rule.tsx:254`），17 / 17 不变（sr-only 是绝对定位）；断言改成"第一个是 H2、第二个是 OUTPUT、最后一个是 FIELDSET 或 P"；f1n1 红在 `health-rule.test.tsx:318` |
| P5S3A-F1-N2 | Nit | `min-w-fit` 在字宽时的副作用：letter-spacing 0.12em 把按钮撑到 181.6 px，头一行宽 396–417.6 时空着的格按 160 排、有按钮时按 181.6 排，版本列表晚到时头一行从一行变两行，标题从 134 下移到 153.5（F2：406 宽 36 对 55.5）；不加 `min-w-fit` 时不折，但按钮溢出 21.6 px、压到面包屑 | 接受：决定 929 的照实记一句（本文档提交）；本机字体下没有这一段 |
| P5S3C-F1-I1 | Important | `expectButtonsInside` 在对话框弹入时量（`styles.css:63-69` 的 `animate-pop-in`，150 ms 里 `scale` 0.97 → 1）：内边按 `getBoundingClientRect()`（缩放之后）加 `getComputedStyle` 的边框与内边距（不缩放）算，靠右的按钮会超出 `25·(1−s)` px；up3:281 在不改代码的树上 8 次红 2 次；narrow.ts 注释里"Chromium and WebKit put a button 0.07 px past the padding"的说法不对 | `ff3e3788`：先等框与它的祖先上有限的动画结束（同 `shell-pages.ts:23` 的 `openDrawer`），每个 `finished` 接 `.catch()`，再严格比较，去掉 0.5 px 的容差，注释照实；up3 ×8、lw3 三种引擎各 ×2 全过，958a、958c 重跑仍红在那一句；只严格、不等动画时 up3 ×8 全红，等动画是必要的；之后留 0.01 px 的浮点余量（`35bcc7bc`，C-F2-N1） |
| P5S3C-F1-N1 | Nit | 958b 在最后的树上不再唯一（`import-dialog.tsx` 里两处同样的串，`mut3.py check 958b` 报 2 处）；处置与报告里的行号是改之前的 | 本文档提交：7.2 记 958b2（问的框那一行，带上下文）；行号照最后的树 |
| P5S3C-F1-N2 | Nit | 文档里还要照实改的：5.3 的 hi2 代码块，lw3、tr2、up3、hi5 在 320 宽的各段；5.4 补 S3 一句；13.2 第 31 条还没有 958 的一句；`9e6594a5` 改了一格的名字 | 本文档提交（958 那一句作 8.2 里 13.2 第 31 条那一条的第二处） |
| P5S3C-F1-N3 | Nit | 别的量对话框的地方同样可能在弹入时量：`expectFits` 只比屏幕的左右边与 `scrollWidth`，缩放只让盒子变小，只会放过、不会误红，288 宽的对话框最多放过约 4 px | 交给收尾（05-P5 第 9 节）：量对话框的帮手统一先等弹入结束 |
| P5S3C-F1-Q1 | Q | rc 在 hi2 不红，可以接受；补一对变异证那三句断言是活的 | 7.2 记 rcw（hi2 红在 `:118`）与对照 w（hi2 过） |

修复轮 2：`ff3e3788`（6 个文件，+74 / −20），门禁一次过；合并之前的整条命令 240 s，只败 se2:231、lw8:836（PG15 这次过了）；up3 ×8、lw3 三种引擎各两次全过；f1m1、f1n1 与重跑的 m1、958a、958c 都红在说好的那一句。

## 修复核对第 2 轮

在 `ff3e3788` 上（对照 `e8aa3b36`），只请 A、C（B 那边没有看得见的改动）。

- **A**：F1-M1 修好：320 宽英文，三种引擎里鼠标双击、三击 Import，上传都照旧在传（点击记录 `[Import,1] [Stop,2]`、`[Import,1] [Stop,2] [Stop,3]`）；单击（0.7 s 后单击 Stop）、Enter、空格、触屏单点 Stop 都照旧能停；Import 的中心在 Stop 上沿之下 9 px（WebKit 8.8 px），`293d016b` 上那一行不折、同一个中心在 Stop 上沿之上 13 px。F1-N1 修好：卡片的无障碍树依次是标题、`status`、说明、"7 issues"、列表，More 之后 `status: 2 more issues shown.` 照常播报，17 / 17。焦点核实的结论要撤回（G4，见"撤回的焦点核实"）。新的三条见下表。
- **C**：F1-I1 修好：up3 ×16、lw3 三种引擎各 ×4、tr2 ×5、hi5 ×3 全过；958a、958c 仍红在那一句，量到的内边是整数（58、41）；框中途关掉时等待照样结束、不会挂着（`mut/anim-cancel.cjs`，三种引擎：框留着时 1484–1520 ms 之后结束、`scale` 是 none；200 ms 时移出文档，204–205 ms 结束）；严格比较在这五处不受负载与字体影响（框宽由窗口定，贴边的一侧落在整数上）；带 0.5 px、不等弹入的旧版（`e8aa3b36`）这一次 up3 ×16 全过。门禁 246 s，只败三格基线。f1m1、f1n1、m1、958a、958c 重放同格，f1m1 的 tr2 连跑 5 次都红在 `:261`；另加 f1m1b。拟了 10.3 讲 up3 的那一句（"偶发"里照用）。文档在 `293d016b` 的副本上重放（`replay-f2.py`）。新的三条见下表。

| # | 级别 | 发现 | 处置与修复提交 |
|---|---|---|---|
| P5S3A-F2-Q1 | Q | 320 宽触屏连点两下 Import：WebKit、Firefox 的触屏模拟里第二下的 click 是 `detail` 1、落在折上来的 Stop 上，把上传停了（各 2/2）；Chromium 是 `detail` 2，守卫挡住；`293d016b` 上那一行不折，落不到 Stop，所以这是 958 扩到这一行之后才有的；真机的 `detail` 没有核实 | 决定 960，`35bcc7bc` |
| P5S3A-F2-N1 | Nit | tr2 的 `dblclick` 一段要靠版面才抓得住：320 宽时两个按钮加间距 260 px、可用约 240 px，只多出约 8%，CI 的字体窄 8% 以上时这一行不折，f1m1 下 tr2 也会过 | = C-F2-N2；`35bcc7bc`：`dblclick` 保留，另用 `elementFromPoint` 断言 Import 原来的中心落在"Stop the upload"上，再对 Stop 发一次 `mouse.down`、`mouse.up` 的 `clickCount: 2`（A 量过：`293d016b` 上会停）；第 3 轮 A-F3-M1 之后换成点击记录（`8a89e184`） |
| P5S3A-F2-N2 | Nit（S3 之前就有） | 按住 Enter 按 Import：上传开始时焦点移到 Stop，自动重复的 Enter 把它按了，上传停了（三种引擎，`293d016b` 上也一样） | 同在这个按钮上，一起改：`35bcc7bc` 给 Stop 加 `onKeyDown`，`event.repeat` 的 Enter 或空格 `preventDefault()`；格 C；f2n2；第 3 轮 A-F3-N1 之后挪到整个表单（`8a89e184`） |
| P5S3C-F2-N1 | Nit | Firefox 的 DOMRect 带单精度的噪声（958a 下 Keep editing 的左边是 −14.333328247070312，不是 1/60 px 的倍数）：宽由内容定的框上严格比较可能差 1e-5 px 而误红 | `35bcc7bc`：`floatNoise = 0.01`（`e2e/fixtures/narrow.ts:41`），两侧对称，注释写明是浮点数的噪声、不是动画（真的超出至少几 px，958 的几行是 21–41 px）；照处置不做变异 |
| P5S3C-F2-N2 | Nit | tr2 不核对双击的第二下真的落在 Stop 上 | 同 A-F2-N1 |
| P5S3C-F2-N3 | Nit | 新的 vitest 格名说"a click or a key's does"，格里只有 `detail` 2 与 0；单击能停由别的格守（f1m1b 红在三格） | `35bcc7bc`：这一格（格 A）补 500 ms 之后 `detail` 1 停的一句，格名照实 |

修复轮 3：`35bcc7bc`（4 个文件，+140 / −21），门禁一次过；合并之前的整条命令 237 s，只败三格基线；tr2 ×5、up3 ×4 全过；960t、960c、both、f2n2、f2n1 都红在各自的格。

## 修复核对第 3 轮

在 `35bcc7bc` 上（对照 `ff3e3788`），A 与 C。

- **A**：960 做到了：320 宽三种引擎里，鼠标双击 Import 仍在传；触屏在 Import 中心连点两下（间隔 5–13 ms 与 250 ms）仍在传，Firefox、WebKit 的第二下 `detail` 1 由时刻挡住；Stop 一出现就单击或轻点（15–50 ms）那一下不算，600 ms 后再按就停；Enter、空格随时能停，最快的一次在单击 Import 之后 7 ms。500 ms 不至于让人觉得 Stop 不灵：双击、双点两下之间通常 100–300 ms，系统的双击判定上限大多在 400–500 ms；有意要停得先看到上传开始，反应时间约 200–250 ms，要想一下的判断通常 0.5–1 s 以上；万一 500 ms 内有意点了，再按一下就停；焦点在上传开始时已在 Stop 上，键盘随时停。vitest 的格 A（`:550`）、格 B（`:569`）、格 C（`:593`）与因 960 改的三格都对，假的只有 `performance`，`setTimeout` 照常；A 另做 4 条变异（去掉时刻守卫只红格 B，去掉计数守卫只红格 A，`<` 改成 `<=` 红 5 格，`onKeyDown` 不拦只红格 C），与实施者的变异表一致。tr2 的 600 ms 是页面时钟上的下界，960c 红在 `:289`。N2 只做到一半，见 F3-N1；`elementFromPoint` 的问题见 F3-M1。顺带看到 WebKit 里双击 Import 之后焦点从 Stop 落到对话框本身，是 macOS 的 WebKit 鼠标点按钮不给它焦点的惯例，不是这一轮带来的。
- **C**：`fakePerformance()` 只假 `performance`（`vi.useFakeTimers({ toFake: ["performance"] })`，`onTestFinished` 还原），各格里 `now − sentAt` 的值是确定的；因 960 改的三格原来守的东西没有丢，960b 在这三格都红，可见它们确实在 500 ms 之后点；tr2 的 600 ms（`startedAt` 在请求发出之后才读，`waitForFunction` 用页面自己的 `performance`）是下界，不会因 CI 慢而时红时过；`elementFromPoint` 靠版面不靠时间，f2n1 下直接红；时刻那道守卫在 e2e 里没有自己的格（只有两道都去掉时 dblclick 才红），由 vitest 的格 B 守；`floatNoise` 远小于真溢出、远大于 Firefox 的 1e-5 px。实施者的变异重放同格，另加 960a、960b、960k。门禁 251 s，只败三格基线；tr2 ×8（6 个 worker 并跑）、up3 ×8、lw3 三种引擎各 ×2、`transfer-import.test.tsx` 单独连跑 5 次全过。判断 13.2 宜加 960 的一句。文档在 `293d016b` 的副本上重放（`replay-f3.py`）。

| # | 级别 | 发现 | 处置与修复提交 |
|---|---|---|---|
| P5S3A-F3-M1 | Minor | tr2 量 Import 的中心时对话框还在弹入（Chromium 在弹入的 `@0` 或 `@17`），量到的点比停稳之后高 8.28 px，只在 Stop 里面 0.72 px（量到 y 667.60，Stop 上沿 666.875），而 `dblclick` 真正点在 y 675.88；CI 的字体让对话框变高时，`:281` 会为错的理由红 | `8a89e184`，取 A 的第二种：去掉在弹入中量的 `centerOf`/`elementFromPoint`，在 `dblclick` 之前往 `document` 装一个捕获阶段的 `click` 监听，每一下记成按钮的文字或节点名与 `event.detail`，断言 `[["Import", 1], ["Stop the upload", 2]]`（今天 `tr2:290`），600 ms 之后那一下再加一条（`tr2:302`）；与量的时机、字体都无关；f2n1 红在 `:290` |
| P5S3A-F3-N1 | Nit | 按住 Enter 按 Stop：第一下停了上传，焦点回到 Import，56–63 ms 后的那次重复按到 Import，上传又开始了（三种引擎，最后在传、2 个请求）；N2 只拦了 Import → Stop 一个方向 | `8a89e184`：重复的守卫从 Stop 挪到整个 `form` 的 `onKeyDown`，只对目标是按钮的 `event.repeat` 的 Enter 与空格 `preventDefault()`（今天 `import-dialog.tsx:331`），Stop 自己的 `onKeyDown` 删了；vitest 格 D（今天 `transfer-import.test.tsx:613`）；f3n1、f3n1s、f3n1t |
| P5S3C-F3-M1 | Minor | 960 那道时刻守卫的起点没有被钉住：删去 `sentAt.current = performance.now()`（960a）46 格全过，因为 `sentAt` 的初值是 0、假的 `performance.now()` 也从 0 起，格 B 在 0 时刻开始上传；真浏览器里 `performance.now()` 从页面打开时算起，没有这一句时窗口只是"页面打开后的头 500 ms" | `8a89e184`：格 B 在 `renderApp(...)` 之后、开始上传之前 `vi.advanceTimersByTime(10_000)`（今天 `transfer-import.test.tsx:575`），注释写明从上传开始算；960a 红在格 B（`:581`）（C 的 `probe960a.py` 试过） |
| C 的判断 | — | 13.2 宜加 960 的一句：一个看起来可用的按钮在一段时间里不理指针的点按，以后改导入对话框或 `Button` 的人容易把它当毛病"修掉" | 接受：13.2 第 12 条之后加一句；8.2 新加第 13 条，原第 13–16 条顺延，文字由 C 在第 4 轮照最后的代码定（本文档提交） |

修复轮 4：`8a89e184`（3 个文件，+70 / −20），门禁一次过；合并之前的整条命令 241 s，只败三格基线；tr2 ×8 全过；960a、960t、960c、f3n1、f3n1s、f3n1t、f2n1 都红在各自的格。

## 修复核对第 4 轮

在 `8a89e184` 上（对照 `35bcc7bc`），A 与 C。

- **A**：按住 Enter、空格（按下、再发 6 次重复、间隔 50 ms、松开），三种引擎完全一样：在 Import 上、在 Stop 上按住，上传都不来回、不重开，松开之后再按照旧能开始、能停；文件框与"Import under"的 select 上重复不被拦、照旧起作用（文件框上按住 Enter 选择框开了 7 次，是文件框自己的行为；Chromium、WebKit 里 select 上的 Enter 是原生的隐式提交，守卫接住了随后落到 Stop 上的重复）。tr2 的点击记录确实证明第二下落在 Stop 上：记录在捕获阶段、比守卫早，浏览器只把 click 派给 mousedown 与 mouseup 两个目标的共同祖先，只有按下、松开都在 Stop 上才会记成 `["Stop the upload", 2]`。f2n1 时记成 `["FORM", 2]` 是对的，原因照 A 的表（"变异"一节的 f2n1）。A 另做 3 条变异（去掉"目标是按钮"只红格 D，只拦 Enter 红格 C、D，表单守卫关掉红格 C、D），与 f3n1、f3n1t 一致。表单上那一行 `oxlint-disable-next-line` 写明了理由、有先例，行为上没有影响。没有要本轮改的；顺带看到的两件 S3 之前就有的事交给收尾（"接受与推后"）。
- **C**：0 I、0 M、0 N。格 B 的 960a 红在 `:581`，960t 也红在 `:581`，960c 只红格 A（`:559`），两道守卫各有只由自己抓住的一格；格 D 测的是在 Stop 上用键盘的 Enter 停掉之后，Import 上重复的 Enter 与空格被拦、两个字段上重复的空格不拦、之后不重复的 Enter 开始第二次上传；`form` 上的 `oxlint-disable-next-line jsx-a11y/no-noninteractive-element-interactions` 带着理由，同 `attachment-row.tsx:125` 的先例；tr2 的点击记录不量位置，与量的时机和弹入无关，`centerOf(stop)`（`:297`）量在 `expectButtonsInside` 等完动画之后，落点由第三条记录核对，不是死代码。实施者的变异重放同格，960c、both 的 e2e 在这一轮的树上重跑（`tr2:301`、`:287`），另加 f3n1k、f3n1r。门禁 250 s，只败三格基线；tr2 ×8 全过，`transfer-import.test.tsx` 单独跑 3 次全过。文档的最后文字已定（`p5s3-rev-c/docs/replay-f4.py`、`8.2-S3-block-f4.md`：8.2 第 11–17 条，新的第 13 条是 960，第 15 条的第二处是 958）；另提 6.3 那一行的"8.2 第 11–16 条"要改成"第 11–17 条"。

修复核对到此：A、B、C 都没有 Important、Minor，不再开一轮。

## 撤回的焦点核实

- **起因**：A 修复核对第 1 轮的"未核实的怀疑"里记下，Chromium 里升级页 Abandon 的确认框按 Escape 之后 `activeElement` 读到的是 `body`（Firefox、WebKit 是触发按钮）；A 猜是自己读得太早，没有在 `22c693f8` 上对照。
- **处置**：实施者在 `e8aa3b36` 与 `22c693f8` 上各量一次，等框从 DOM 里去掉之后再读焦点（当时以为这是确定的信号、不用时间）；两边回到触发按钮则是 A 读得太早，两边都是 `body` 则 S3 之前就有、列给收尾，只有 `e8aa3b36` 是 `body` 则停下来报告。
- **实施者的核实**（修复轮 2 报告第六节；探针 `p5s3/probe1/ui1-zz-p5s3-focus.spec.ts`，三种引擎，1280 与 320 宽各开关 3 次，先等弹入结束再按 Escape，`toHaveCount(0)` 之后读一次 `activeElement`）：Chromium 在 `e8aa3b36` 上 4 次 `body`、2 次回到"Abandon this upgrade…"，`22c693f8` 上 5 次 `body`、1 次回到触发按钮，另加的对照 `293d016b` 上 6 次 `body`；Firefox、WebKit 每次都回到触发按钮。实施者读了这一次就下结论："Chromium 多半落在 `body`，S3 之前就是这样"，照处置的第二种列给收尾；另记 Chromium 按 Escape 当时就读到 `body`，原因没有追。
- **这个结论不对**（A 修复核对第 2 轮的 G4）：焦点由调用方移回（`upgrade-page.tsx:413` 调 `buttons.abandon.current.focus()`，经 `confirm-dialog.tsx` 的 `onCloseAutoFocus`），而 `onCloseAutoFocus` 由 Radix FocusScope 卸载时在 `setTimeout(0)` 里派发：焦点是在框去掉之后由一个定时器移回的，去掉之后立刻读一次就和这个定时器赛跑。G4 在框打开前拿住触发按钮，包过 `HTMLElement.prototype.focus()`，记下带时间的 `focusin`/`focusout`，在四个时点读 `activeElement`，最后用会轮询的 `toBeFocused()` 收尾：Chromium 在 4–12 ms 有 Cancel 的 `focusout`（焦点先落到 `body`），14–16 ms 对"Abandon this upgrade…"调 `focus()`；Firefox、WebKit 在 7–18 ms 调 `focus()`。100 ms 与 1.1 s 时三种引擎 6/6 都在触发按钮上、是同一个元素，轮询的 `toBeFocused()` 6/6 过，`293d016b` 上也一样。Chromium 只是晚约 10 ms，一次性的读数正好落在这段空档里；观察到的值没错，错的是由此得出的结论。A 第 1 轮的那一条错在同一处（也是在 `toHaveCount(0)` 之后读一次），一并更正。
- **撤回**：收尾里没有这件事，不再列给收尾（实施者在修复轮 3 的报告里照实撤回）。以后核对焦点的去向，用会轮询的 `toBeFocused()`，或者等一个 `focusin`，不只读一次。

## 主持者的决定（负责人可以改判）

全文在 [05-P5](../05-P5-pages.md) 2.3。

- **955 升级的检查与报告里"都没问题"的 summary 用 `clearSummaryClass`**（实施者的偏离 5；审查 A 的 P5S3A-Q1、B 的偏离 5、C 的 P5S3C-Q1）：与健康页同一个 `details`、同一个 `HealthRule` 卡片，同一种东西一个样子（`upgrade-check.tsx:258`、`:389`）；补 C 的 cxc 那一句（报告一节的 summary，`upgrade-page.test.tsx:1120`）；4.3.4 照实。
- **956 升级页"Where it is""Connect an agent"两节的正文 `text-sm`**（偏离 6；B 的 P5S3B-Q1、A 的 P5S3A-Q2）：颜色仍是正文色，同 settings.html 的行（`upgrade-page.tsx:634`、`:673`）；检查的说明等照旧 `noteClass`；"Check"按钮的 36 px、8 px 圆角同页的 Submit、Abandon，不改（B 的判断）；956a、956b。
- **957 早退两支的头一行按 `writesNotebook` 定那一格**（偏离 3；A 的 P5S3A-N1、B 的 P5S3B-Q2）：能写的人打开来源页的版本时，树读到之前有那一格、读到之后没有，375 宽列的上边约移 20–36 px，1440、768 宽不动（B 修复核对第 1 轮另量到 768 宽读到之后 `h1` 下移 3 px，见"接受与推后"）；接受的理由：树读到之前无从知道是来源页，反过来做，更常见的普通页会反方向挪；早退的 `test.each` 加来源页一行，钉住"早退有那一格、读到之后没有"（A 的建议）；变异 957。
- **958 对话框里问的框的按钮一行折得下**（B 的 P5S3B-I1）：`source-add-dialog.tsx:141` 与 `import-dialog.tsx:143` 改成 `flex flex-wrap justify-end gap-2`，放不下时各占一行、靠右，宽时照旧一行（B 的 f2）；vitest 两句类名，各一条变异；e2e 在已有的、打开问的框的故事里加 320 宽一段，按位置核对按钮都在框内（05-lessons 第 6 节）；另 grep 别的对话框里同样写法的按钮行在 320 宽量一次。修复之后扩到三行（下面）。
- **959 中文的右栏版本行里，时间与种类不再有单独一行的一个字**（B 的 P5S3B-N2，接着 943）：时间的 Link 加 `whitespace-nowrap`（`page-versions.tsx:121`）；`VersionBy` 改成名字（`Isolated`，带" · "）之后接 `whitespace-nowrap` 的 `span` 包住的种类（`:199`）；196–244 px 逐 1 px 扫，三种引擎、中英都是 0；n2a、n2b。
- **960 上传开始之后 500 ms 内，"Stop the upload"不理会指针的点按**（A 修复核对第 2 轮的 P5S3A-F2-Q1）：320 宽触屏连点两下 Import，WebKit、Firefox 的模拟里第二下是 `detail` 1、落在折上来的 Stop 上，这是 958 扩到这一行之后才有的；做法 (a)：`event.detail > 1`，或 `detail > 0` 且离上传开始不到 500 ms 时不停（`import-dialog.tsx:187` 的 `stopTakesNoPointerMs`、`:291` 记下 `sentAt`、`:431`），键盘（`detail` 0）不受影响；不取 (b)：(b) 要换 958 的样子；vitest 用假的 `performance`，500 ms 内 `detail` 1 不停、之后停、`detail` 0 随时停；两道守卫各有一格只由它抓住（960t、960c）；同一按钮的 P5S3A-F2-N2 一起改；13.2 第 12 条之后加一句（C 修复核对第 3 轮的判断）。

处置表里另由主持者定的：A 的未核实的怀疑加成一条 Minor（还原那一格不窄于按钮，`9de4aed9`）；C-Q2（49 两种都记）；C-Q3（04-P4 第 9 节的"P5"一条）；B-N3 交给收尾；实施者的"交给后面"（升级页的字号由 956 定、"Check"照 B 的判断照旧，都不再交出；lw8:903 [webkit] 记为偶发；Linux 字体下 hi2 的 320 宽一段与 UP1 的版本页一步等推送之后的 CI）；C 的交接草稿（M12-polish 一行、01-P1 第 9 节、04-P4 第 9 节、00-M13 第 9、12、13 节）照用，修复之后照实核一次；审查记录的文件名是 `reviews/P5-S3-versions-review.md`；10.3 的 up3 一句用 C 拟的；10.3"交给后面"的 tr2 一句用 C 拟的意思，照 A-F3-M1 改成捕获的点击记录；6.3 那一行改成"8.2 第 11–17 条"（C 第 4 轮）。

修复轮之后主持者定的：

- **重做本地提交**（接受：不 amend 守的是审查过的提交，这两个没进过审查、没推送）：第 1 个修复提交在本机重做过两次（`git reset --soft HEAD~1`；`8280a72a`、`9a5e0b36`，父都是 `22c693f8`，门禁未过，没推送、没人审查过），与 `9de4aed9` 只差测试：`8280a72a` 的 `expectButtonsInside` 是严格比较，lw3 在 Chromium、WebKit 上超出 0.07 / 0.006 px 而败，`9a5e0b36` 为此加了 0.5 px 的容差（修复核对第 1 轮查明那是对话框弹入的余量，P5S3C-F1-I1：`ff3e3788` 改成先等弹入的动画结束、再严格比较，去掉了这个容差，之后 `35bcc7bc` 只留 0.01 px 的浮点余量，P5S3C-F2-N1），它与 `9de4aed9` 先后把两个局部函数提到模块级，S3 原有的三个提交没动。
- **958 扩到三行**：958 的 grep 找到 10 行都是 `flex justify-end gap-2`；实施者在 320 宽打开的对话框里把每一对按钮文字写进同一行量了 69 对（`ConfirmDialog` 的每一处用法连同发送中的文字、每个对话框的每种状态，中英，三种引擎，内宽 238），英文放不下的只有三行：`confirm-dialog.tsx`（升级页的四个确认：Cancel + Abandon the upgrade 264.7、Continue the rollback 262.5、Approve the upgrade 260.7、Reject the upgrade 247.0；实际打开的 Abandon 框里 Cancel 的左边在对话框之外）、`import-dialog.tsx` 的上传中（Stop the upload + Uploading… 260.1）、`revert-dialog.tsx` 的在后台撤销（Cancel + Undo in the background 280.0）；中文最大 199.8，375 宽内宽 293 都放得下。主持者定：同一种毛病，改法一样，用户会碰到，不留给收尾。`e8aa3b36`（9 个文件，+39 / −5）：三行改成 `flex flex-wrap justify-end gap-2`、各加一句注释（今天 `confirm-dialog.tsx:193`、`import-dialog.tsx:420`、`revert-dialog.tsx:422`）；vitest 在已有的格里各加一句类名断言（`confirm-dialog.test.tsx:101`、`transfer-import.test.tsx:471`、`revert-background.test.tsx:103`）；三行都有现成的故事，各在 320 宽加 `expectButtonsInside`：up3（放弃升级的确认，`up3:281`）、tr2（上传中的一行，今天 `tr2:294`）、hi5（在后台撤销，`hi5:184`），都只在 Chromium；958c、958d、958e 在 vitest 与 e2e 都红；放得下的其余七行不改（"接受与推后"）。
- **rc 在 hi2 不红**（本机字体下按钮 150 px，放得进 160 px 的格）：接受，由 vitest 的类名一句（`version-page.test.tsx:748` 与早退表 `:855`）与 letter-spacing 的探针证（修好的树三种引擎撑到 0.5em 都在格内、不压面包屑、在标题之上；对照 rc 从 0.15em 起溢出 29–120 px、1440 宽压到面包屑）；hi2 的三句按位置的断言在 Linux 字体下若按钮超过 160 px 就会红，等 CI；7.2 照实记，另记 C 的 rcw 与 w（P5S3C-F1-Q1）。
- **实施者的选择 1–6**（处置的编号），都接受，照实记进 7.1、7.2 与 10.3：
  - 1：粘贴的问的框加进 lw3:287 已有的一段（三种浏览器），导入的加进 tr2 现成的离开时问的那一段（只跑 Chromium）。
  - 2：`expectButtonsInside`（`e2e/fixtures/narrow.ts`）允许 0.5 px 的误差，lw3 与 tr2 在同一处另加 `expectFits(dialog, asking)`；这个容差之后由 P5S3C-F1-I1 去掉（上面那一句）。
  - 3：956 与 N2 的变异各拆两条（956a、956b，n2a、n2b），每条只改一行。
  - 4：957 那一行写成 `test.each` 的 branch `"source"`，不加第五列，格的签名不变；早退两支的判断从 `=== "loading"` 改成 `!== "gone"`；`installed()` 加第三个参数（`mode`、`sourceRootId`）。
  - 5：cxx 加进已有的"服务端拒绝"一格：拒绝同时含标题与正文，格名改为"pasted Markdown shows a server refusal of the title and the body without clearing the draft"，断言说明的文字与 `text-compact text-destructive`。
  - 6：`page-versions.test.tsx` 的 `listed()` 只把直接子 `span` 算作小块（`:scope > div > span`）：种类的 `span` 是新加的，不该算进小块（A、C 核对：标记本来就是那个 `div` 的直接子 `span`，收窄之后更严，没有失去实际的守护）。
- **修复轮 2（`ff3e3788`）的照实**：只严格比较、不等动画时 up3 ×8 全红，等动画是必要的；带 0.5 px 容差的旧版这一次 up3 ×8 全过，没能复现 C 的 2/8（"偶发"里 C 的那一句）；焦点的核实之后撤回（"撤回的焦点核实"）。
- **修复轮 3（`35bcc7bc`）的照实**：因 960 改的三格（"Stop the upload stops it, the dialog kept; …"、"a double click on Stop the upload stops it, …"、`test.each` 里的"an upload stopped reads the jobs again, …"）点 Stop 之前把假的 `performance` 推进 500 ms，其余断言不动；接受实施者说明的第 1 条：计数为 2 的点按接在 `dblclick` 之后、等过 600 ms，在同一次上传里（`dblclick` 的第一下就是那次单击，少开一次上传；500 ms 之内的点按反正都不算，那时去测计数守卫就测不到它）；tr2:281（Import 的中心落在 Stop 上）靠版面，Linux 字体下不成立会响亮地红，等 CI（第 3 轮之后换成点击记录，`tr2:290`）。
- **修复轮 4（`8a89e184`）**，两处接受：
  - 实施者选的表单上的 `oxlint-disable-next-line jsx-a11y/no-noninteractive-element-interactions`（`import-dialog.tsx:322`，一行、不超过 120 列，带理由"a held key's repeats on its buttons"，同 `attachment-row.tsx:125` 的先例）：lint-web 要零警告；另两种做法（在 `useEffect` 里用 ref 装原生的 keydown 监听、给三个按钮各装一个同样的 `onKeyDown`）没取，因为处置写的是放在 `form` 上。
  - 字段的重复：表单里没有文本框，只有 zip 的文件框与"Import under"的 select，"按住空格照旧能打字"没法照字面断言，改为格 D 断言这两个字段上重复的空格不被拦（`fireEvent` 返回 true），格的注释照实写"The form has no text box"；f3n1t 红在这一句（`:626`）。

## 变异

每条都先从备份还原，再对改过的文件以 `git diff --quiet HEAD --` 核对（02-workflow 3.1），没有一次没回到 HEAD；改坏期间不跑 `make gen*`、不用 `git checkout --`；修复轮的各条先跑一遍"只改不跑"的 check，都只改到一处。实施者以 `p5s3/mut/mut.py` 跑（`results.tsv`、`results-again.tsv`、`results-e2e.tsv`，日志 `mut/logs/`），修复轮以 `mut3.py`–`mut7.py`（`results-fix1.tsv`–`results-fix5.tsv`）；故事的一条先 `make build`，再跑那一个故事（Chromium，除非写明）。审查 C 以 `p5s3-rev-c/mut/mut.py`（实施者那一份，只改 `ROOT`）与 `mut2.py`（自己的），修复核对以 `mut3c.py`–`mut12c.py`（实施者的原样，只换结果文件，另加自己的），结果在 `results-*.tsv`、`results-c-fix*.tsv`。审查 A 在自己的工作树里另做 7 条（第 3 轮 4 条、第 4 轮 3 条，"修复核对"两节），各红在与实施者、C 相同的格。

- **第一轮**（实施者）：vitest 的一轮在 `066c84b1` 上，活下来的 8 条补格之后在 `22c693f8` 上重跑都红；故事的各条都在 `22c693f8` 上。7.2 的 S3 各条都被抓住（49 照实，下表）；t1–t67 里八条活着，由 `22c693f8` 补格。
- **审查者，`22c693f8` 上**：C 重放实施者的 47、50、52、52v、56、57、58、61、84、84h2、fb1、fb2、t2、t8、t13、t21、t23a、t51、t32、t33、t41、t49 与 49 的两种，都与实施者同格；另写 cxa、cxb、cxc、cxx（整套全绿，C-N1）、cxv、cxr（hi2 照绿，C-M1）。
- **修复之后，`9e6594a5` 上**（实施者 16 条）：vitest 16 条全红，e2e 4 条里 3 条红，rc 活着；`e8aa3b36` 上 958c、958d、958e 在 vitest 与 e2e 都红。C 在 `e8aa3b36` 上重放并另写 958b2、rcw、w。
- **`ff3e3788` 上**：实施者 f1m1、f1n1 与重跑的 m1、958a、958c；C 重放并另写 f1m1b。
- **`35bcc7bc` 上**：实施者 960t、960c、both、f2n2、f2n1；C 重放并另写 960a（活着，C-F3-M1）、960b、960k。
- **`8a89e184` 上**：实施者 960a、960t、960c、f3n1、f3n1s、f3n1t、f2n1；C 重放（960c、both 的 e2e 也在这棵树上重跑）并另写 f3n1k、f3n1r。

7.2 的 S3 各条（实施者；"红在"的行号照 `22c693f8`，之后没有挪的除外）：

| # | 改坏 | 红在 |
|---|---|---|
| 47 | "还原这个版本"挪出头一行（那一格留着、空），放在 `h1` 旁边一个 `flex justify-between` 里（S3 之前的样子） | `version-page.test.tsx` 4 格（头一行一格、"a reader restores none"、admin 与 editor 的"版本列表读到之前"两行）；`hi2:102`（320 宽 `lines` 得 2，期望 < 1.5）；C 重放同 |
| 48 | 版本页的"当前"用普通的 `Mark` | `version-page.test.tsx`"the newest version's Current…"一格 |
| 49 | 版本页的路由不加 `handle` | 两种都记（P5S3C-Q2）：照设计的写法，在它有意义的 `51125487` 上，表的版本页 6 行都红在 `upgrade-banner.test.tsx:290` 的 `mx-auto`（C，`49orig.log`，6 failed / 45 passed）；在最后一个提交上已是空操作（提交 2 删了路由的 `handle`，读 `handle` 的只剩 `settings-origin.ts:18` 读 `settings` 与 `layout.tsx:62` 读 `sidebar`）。代理：`VersionShell` 的列不画 `ColumnTop`，表的"a version's page, read""its version not found"2 行红（实施者；C 在 `22c693f8` 上复现）；早退两支另由 61 与 cxv 守 |
| 50 | 健康的规则卡片 `section` 换成 `div` | vitest 48 格（`list-rows` 1、`health-page` 7，含 `rule()` 的 `closest("section")`，`health-page-lifecycle` 8、`health-rule` 32）；lw8 在 Chromium 里 24 格红 13 格：`:625` 两格，其余在 `:678`、`:786`、`:832`、`:860`、`:915`、`:955`、`:982`、`:1017`、`:1058`、`:1136`、`:1197`（C-N4；设计写的是 `health-page.test.tsx:166` 与 lw8:625）；C 重放同 |
| 51 | 健康页的标题行做成 `section id="head"` | 只红在 `lw8:625`，不在 `:678`：`:678` 数的是 `details` 里的 `section[id]`，标题行不在那里（设计写的 `:625,678`，C-N4） |
| 52 | 升级页的根 `div` 换成 `section` | `up5:252` 的 `toHaveText`：`upgradeSection` 的 `locator("section").filter(...)` 同时匹配根与"检查"一节，"Missing type"那张卡片后面的 `getByRole("listitem")` 多读到根里的 19 项，不是设计写的 strict mode；vitest 由 52v 抓（下表）；C 重放同 |
| 53 | 附件的 `ul` 加 `aria-label` | `rows()` 找不到列表：`attachments-section` 9、`attachment-uploads` 8、`source-uploads` 1 格 |
| 54 | 下载图标挪进链接、带 `sr-only`"Download" | `rows()` 的文字：`attachments-section` 6、`attachment-uploads` 1、`source-actions` 1 格 |
| 55 | 内联的附件也画下载图标 | `attachments-section.test.tsx` 新格 |
| 56 | "操作"去掉 `aria-expanded:opacity-100` | `attachments-section.test.tsx` 新格；`as1:187`（期望 "1"、得 "0"）；C 重放同 |
| 57 | 目录页不画 `ColumnTop`（回退已删） | 表的目录一行；C 重放同 |
| 58 | 变更集页读取中、读不到那一支不包 `PageColumn` | 表的"on its way""not read"两行；`changeset-page.test.tsx` 读取中空头一行一格；C 重放同 |
| 59 | 横幅的 `bg-surface-2` 换成 `bg-amber-50` | `design-tokens.test.ts` 2 格；横幅一格 |
| 60 | 标签组照旧 `space-y-2` | `source-add-dialog.test.tsx` 新格 |
| 61 | 版本页 `PageGone` 那一支不包 `PageColumn` | 表的"page not found""revision no number"两行；C 重放同 |
| 83 | 那一格只在按钮显示时画 | `version-page.test.tsx` admin、editor 两行（版本列表扣住时那一格不在） |
| 84 | 右栏的小块回到行尾自成一列（`div.ms-auto.flex.shrink-0`） | `page-versions.test.tsx` 2 格（改写的那一格、中文那一格）、`version-restore.test.tsx:157`；`hi1:259`（"· EditCurrent"）；hi2 那一句由 84h2 抓；C 重放同 |
| 85 | 历史行的"Reverted"照旧方角 | `history-page.test.tsx` 加的那一句 |
| 86 | 粘贴来源的文本框照旧手写 | `source-add-dialog.test.tsx` 新格 |
| 87 | 升级页通往页面的链接不带 `arrived` | `upgrade-page.test.tsx` 新格 |
| 88 | 版本页作者的链接不带 `arrived` | `version-page.test.tsx` 新格 |
| 89 | 小圆点 `border-3 border-accent` 换成 `bg-accent` | `design-tokens.test.ts`（"amber is no fill"）；横幅一格 |
| 94 | `markClass` 去掉 `whitespace-nowrap` | `page-versions.test.tsx` 新格：`toBe(markClass)` 抓不到改了 `markClass` 本身，靠 `arrayContaining(["rounded-full", "whitespace-nowrap"])` 那一句（C 核对） |

实施者另写的（`066c84b1` 上 vitest，活下来的 8 条在 `22c693f8` 上重跑；逐条在 `p5s3/report.txt` 第三节）：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| fb1 | 实施者；C 重放 | `NotebookLayout` 照旧在出口之上画横幅（没有路由带 `handle`，与原来的只在不分列时画等价），列里的也在 | `upgrade-banner.test.tsx` 51 格（表的 36 行全红在第 1 句"恰好一个"） |
| fb2 | 实施者；C 重放 | 原来的整套：出口之上画横幅，`ColumnTopContext` 给 null | `upgrade-banner.test.tsx` 37 格（表的 36 行红在第 2 句 `mx-auto`，加设置 index 那一格） |
| t1、t3–t7 | 实施者 | 健康一行：卡片的类名、说明的 `factsClass`、列表的线、行的 `py`、`lineTitleClass`、`lineLinkClass` | `list-rows.test.tsx` 健康一行 |
| t2 | 实施者；C 重放 | 健康卡片的标题 `text-sm` → `text-lg` | `066c84b1` 上活；`22c693f8` 补格之后红在 `list-rows.test.tsx` 健康一行 |
| t8 | 实施者；C 重放 | 路径缺时的 `span` 去掉 `font-normal` | 活；补格之后 `health-rule.test.tsx` 的 `test.each` 3 行 |
| t9–t12 | 实施者 | 健康页的标题行、标题、Recheck 小胶囊、计算时刻的说明 | `health-page.test.tsx` 列表页一格 |
| t13 | 实施者；C 重放 | "Rechecking health…"不带 `noteClass` | 活；补格之后 `health-page.test.tsx` 列表页一格 |
| t14、t15 | 实施者 | 健康页列的类名；clear 的 summary | `health-page.test.tsx` 列表页一格；t14 另红在表的健康一行，t15 另红在 `upgrade-check.test.tsx` 门槛一格 |
| t16 | 实施者 | 健康页不画 `ColumnTop` | 表的健康一行 |
| t17 | 实施者 | 升级页根的类名 | 表的升级一行；`upgrade-page.test.tsx` 列表页一格 |
| t18、t20 | 实施者 | 升级页的标题；检查的说明 | `upgrade-page.test.tsx` 列表页一格 |
| t19、t22、t26 | 实施者 | `sectionTitleClass`；`reportPassed`；回滚作业的卡片 | `upgrade-page.test.tsx`"a rollback's job is a card's row…"一格（t19 另一格） |
| t21 | 实施者；C 重放 | "Being checked…"不带 `noteClass` | 活；补格之后 `upgrade-check.test.tsx`"in review without a report…" |
| t23a、t23b | 实施者；C 重放 t23a | 检查、报告两处"状态变了"不带 `noteClass` | 活；补格之后 `upgrade-check.test.tsx`"a card's More refused…""the report's More refused…"两格 |
| t24、t25、t28 | 实施者 | 门槛的卡片、门槛的行、检查一节的 summary | `upgrade-check.test.tsx` 门槛一格 |
| t27 | 实施者 | 升级页不画 `ColumnTop` | 表 4 格 |
| t29、t30、t35、t39 | 实施者 | 版本页事实的类名、`output` 的类名、标题、Restore 不是小胶囊 | `version-page.test.tsx` 头一行一格 |
| t31 | 实施者 | 那一格的 `min-h` 与宽 | `version-page.test.tsx` 5 格 |
| t32 | 实施者；C 重放 | 早退传页的 26rem 一组 | `version-page.test.tsx` 4 格；UP1 `up1:319` 三种浏览器，768 宽读取中 y 99.5 对读到之后 78 |
| t33 | 实施者；C 重放 | 早退恒传那一格 | `version-page.test.tsx` 2 格（读者两行）；UP1 `up1:319` 三种浏览器，375 宽 153.5 对 134 |
| t34 | 实施者 | 早退不传那一格 | `version-page.test.tsx` 写者两行 |
| t36 | 实施者 | 版本页列的类名 | 表的版本页两行；`version-page.test.tsx` 头一行一格 |
| t37 | 实施者 | 面包屑不带 `headCrumbsClass` | `version-page.test.tsx` 6 格 |
| t38 | 实施者 | "当前"的底 | `page-versions.test.tsx` 2 格；`version-page.test.tsx` 1 格 |
| t40 | 实施者 | Undone 不带悬停的类 | `history-page.test.tsx` 那一句 |
| t41 | 实施者；C 重放 | 小块之前不带空格 | `page-versions.test.tsx` 2 格、`version-restore.test.tsx`；`hi1:259` |
| t42 | 实施者 | 右栏时间与作者那一块去掉 `break-words` | 活；补格之后 `page-versions.test.tsx` 改写的那一格 |
| t43、t44a、t44b、t46–t48、t50 | 实施者 | 附件：卡片、行不带 `group/row`、行的间距、下载图标的大小与颜色、大小的类名、"操作"的底色、Upload | `attachments-section.test.tsx` 新格 |
| t45 | 实施者 | 来源的图标 | `source-attachments.test.tsx` 2 格 |
| t49 | 实施者；C 重放 | "操作"不带 `revealClass` | `attachments-section.test.tsx` 新格；as1 照绿（4 passed）：对故事等价，as1 只在菜单开着时量 1，去掉 `revealClass` 之后恒为 1 |
| t51 | 实施者；C 重放 | 拖放说明回到 `text-sm text-muted-foreground` | 活；补格之后 `attachments-section.test.tsx`"without attachments…" |
| t52、t53 | 实施者 | 附件一节的环；阅读视图的环 | `attachment-uploads.test.tsx` 新格 |
| t54 | 实施者 | 附件一节的 `rounded-card` | `attachment-uploads.test.tsx`、`attachments-section.test.tsx` 新格 |
| t55、t56 | 实施者 | 上传的卡片；上传的行 | `source-uploads.test.tsx` 3 格；`attachment-uploads.test.tsx` 新格 |
| t57 | 实施者 | 粘贴来源的上传行 | `source-uploads.test.tsx` 3 格 |
| t58–t61 | 实施者 | 横幅的边与圆角、小圆点的上边距、状态一行的字重、小圆点不带 `aria-hidden` | `upgrade-banner.test.tsx` 横幅一格 |
| t62、t64–t67 | 实施者 | 粘贴来源问的框、说明 `text-compact`、文本框的 `py-2`、`Input` 的 `py-1`、`fieldClass` 去掉 `focus-visible:outline-hidden` | `source-add-dialog.test.tsx` 新格（它比 `Input` 与文本框的类名） |
| t63 | 实施者 | 导入问的框 | `transfer-import.test.tsx` 问的框那一句 |

修复轮的与审查者的（行号照各自的提交，写明的除外）：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| 52v | 实施者；C 重放 | 同 52，跑 vitest | `upgrade-page.test.tsx` 列表页一格（根是 `DIV`、根的类名） |
| 84h2 | 实施者；C 重放 | 同 84，跑 hi2 | 320 宽那一段过了之后红在 `hi2:124`（今天 `:146`），得"· RestoreCurrent" |
| cxa | C；实施者 | `health-page.tsx:319`"模式变了"回到 `text-sm` | `22c693f8` 上整套 233 / 5592 全绿（C-N1）；`9e6594a5` 起红在 `health-rule.test.tsx:331` |
| cxb | C；实施者 | `health-page.tsx:333` 分页说明回到 `text-sm text-muted-foreground` | 全绿；`9e6594a5` 起红在 `health-page.test.tsx:540` |
| cxc | C；实施者 | `upgrade-check.tsx:389` 报告一节的 summary 回到 `cursor-pointer font-medium` | 全绿；`9e6594a5` 起红在 `upgrade-page.test.tsx:1120` |
| cxx | C；实施者 | `source-add-dialog.tsx:279` 出错的说明回到 `text-sm text-destructive` | 全绿；`9e6594a5` 起红在 `source-add-dialog.test.tsx:228` |
| cxv | C；实施者 | 版本页"树读取中或读不到"一支不包 `PageColumn` | `22c693f8` 上表两行、`version-page.test.tsx` 早退空头一行两行；`9e6594a5` 上横幅 2 格、`version-page` 3 格 |
| cxr | C；实施者 | "还原"挪到标题之下自成一行，那一格留着、空着 | `version-page.test.tsx` 4 格；`22c693f8` 上 hi2 照绿（C-M1）；`9e6594a5` 起红在 `hi2:117`（"Restore's bottom, at most the title's top" 237.5，要求 ≤ 153.5） |
| 958a | 实施者；C 重放 | 粘贴问的框的按钮行去掉 `flex-wrap` | `source-add-dialog.test.tsx:463`；`lw3:366` 三种引擎（`9e6594a5` 上 Chromium −13.15；`ff3e3788` 上 −13.89 / −14.33 / −13.89，要求 ≥ 58，停稳之后量） |
| 958b | 实施者 | 导入问的框去掉 `flex-wrap` | `transfer-import.test.tsx` 离开时问的那一格；`tr2:264`（"Keep uploading's left" −13.47，要求 ≥ 57.5）；`e8aa3b36` 之后这个文件里有两处同样的串，在最后的树上用不了（C-F1-N1） |
| 958b2 | C | 958b 按问的框那一行写（带上下文） | `e8aa3b36` 上 `transfer-import.test.tsx:490`；`tr2:266`（−13.47，要求 ≥ 57.5） |
| 958c | 实施者；C 重放 | `confirm-dialog.tsx` 的按钮一行去掉 `flex-wrap` | `confirm-dialog.test.tsx:101`；`up3:281`（"Cancel's left" 15.70，要求 ≥ 41.69；C 第 1 轮 16.96）；`ff3e3788` 上 14.5，要求 ≥ 41 |
| 958d | 实施者；C 重放 | `import-dialog.tsx` 上传中那一行去掉 `flex-wrap` | `transfer-import.test.tsx:471`；`tr2:261`（"Stop the upload's left" 19.17，要求 ≥ 40.5） |
| 958e | 实施者；C 重放 | `revert-dialog.tsx` 的一行去掉 `flex-wrap` | `revert-background.test.tsx:103`；`hi5:184`（"Cancel's left" −0.59，要求 ≥ 40.5） |
| rc | 实施者；C 重放 | 那一格回到 `min-w-0` | `version-page.test.tsx` 4 格（头一行 `:748`、早退表 `:855`）；hi2 活：本机字体下按钮 150 px，放得进 160 px 的格 |
| rcw | C | rc，再给"还原"加 `tracking-[0.2em]`，代替更宽的字体 | `hi2:118`（"Restore's left, in its cell" 93.55，要求 ≥ 136） |
| w | C | 对照：保留修复，按钮同样加宽 | hi2 过：那一格跟着按钮变宽，面包屑让开 |
| m1 | 实施者；C 重放 | `output` 回到 `ul` 之后 | `health-rule.test.tsx:316`；`ff3e3788` 上重跑红在 `:318` |
| n1 | 实施者；C 重放 | 去掉 `small` | `health-rule.test.tsx:319` |
| n2a | 实施者；C 重放 | 时间的 Link 去掉 `whitespace-nowrap` | `page-versions.test.tsx:155` |
| n2b | 实施者；C 重放 | 种类放回名字那一串 | `page-versions.test.tsx:156` |
| 956a、956b | 实施者；C 重放 | "Where it is"、"Connect an agent"去掉 `text-sm` | 各红在 `upgrade-page.test.tsx:2157` |
| 957 | 实施者；C 重放 | 读到之后那一格按 `writesNotebook` 定，不按 `canEditPageBody` | `version-page.test.tsx:865`（来源页那一行读到之后仍有那一格） |
| f1m1 | 实施者；C 重放 | Stop 的 `onClick` 回到直接 abort | `transfer-import.test.tsx:538`；`tr2:261`（"Stop the upload"不可见）；C 连跑 tr2 ×5 都红在 `:261` |
| f1m1b | C | `event.detail > 1` 改成 `> 0`（只有键盘能停） | `ff3e3788` 上红 3 格（`transfer-import.test.tsx:476`、`:515`、`:756`），新的一格本身过（C-F2-N3） |
| f1n1 | 实施者；C 重放 | `output` 回到标题之前 | `health-rule.test.tsx:318` |
| 960t | 实施者；C 重放 | 去掉时刻那道守卫，只留 `detail > 1` | `35bcc7bc` 上只红格 B（`:579`）；`8a89e184` 上只红格 B（`:581`） |
| 960c | 实施者；C 重放 | 去掉计数那道守卫，只留时刻 | 只红格 A（`:559`）；`tr2:289`（`35bcc7bc`：600 ms 之后计数为 2 的点按停了上传，`ERR_ABORTED`，Stop 不在了）；`8a89e184` 上 C 重跑红在 `tr2:301` |
| both | 实施者；C 重放 | 两道守卫都去掉（同以前的 f1m1） | 格 A `:559`、格 B `:579`；`tr2:275`（双击的第二下停了上传）；`8a89e184` 上 C 重跑红在 `tr2:287` |
| 960a | C（第 3 轮）；实施者 | 删去 `sentAt.current = performance.now()` | `35bcc7bc` 上 46 格全过（C-F3-M1：假的 `performance.now()` 从 0 起，正好等于 `sentAt` 的初值）；`8a89e184` 起只红格 B（`:581`） |
| 960b | C | 边界 `<` 改成 `<=` | `35bcc7bc` 上红 5 格（`:487`、`:528`、`:562`、`:582`、`:821`）：因 960 改的三格确实在 500 ms 之后点 |
| 960k | C | 只拦 Enter 的重复 | `35bcc7bc` 上格 C `:601`（空格那一句） |
| f2n2 | 实施者；C 重放 | 去掉 Stop 的 `onKeyDown` | `35bcc7bc` 上只红格 C（`:600`） |
| f2n1 | 实施者；C 重放 | 导入底部那一行去掉 `flex-wrap`（版面变了） | `35bcc7bc` 上 `tr2:281`（Import 原来的中心那里取不到按钮，`closest` 得 undefined，直接红，不再悄悄地过）；`8a89e184` 上 `tr2:290`，第二下记成 `["FORM", 2]`，不是 `["Stop the upload", 2]` |
| f3n1 | 实施者；C 重放 | 表单守卫的条件恒为假（`false && …`） | 格 C `:602`、格 D `:624` |
| f3n1s | 实施者；C 重放 | 守卫挪回只在 Stop 上（同 `35bcc7bc`） | 只红格 D `:624`（Import 上的重复） |
| f3n1t | 实施者；C 重放 | 去掉"目标是按钮"这个条件 | 只红格 D `:626`（文件框上重复的空格被拦了） |
| f3n1k | C | 只拦 Enter 的重复 | 格 C `:603`、格 D `:625`（都是空格那一句） |
| f3n1r | C | 不看 `repeat`，Enter 与空格一律拦 | 格 C `:604`、格 D `:620`（"不重复的那一下照样按得动"那一句） |

f2n1 记成 `["FORM", 2]` 的原因照 A 修复核对第 4 轮的表（照 tr2 的次序，三种引擎；Chromium 的数，Firefox 相同，WebKit 差 0.1 px）：照现在的写法记成 `[Import,1]`、`[Stop the upload,2]`，Import 的中心 y 675.9，Stop 在 666.9–702.9；那一行去掉 `flex-wrap` 时进度那一块在 626.9–672.9、按钮行在 688.9–724.9，Import 的中心正落在两者之间、表单 `space-y-4` 的空隙里，谁也没点中，记成 `["FORM", 2]`，`elementFromPoint` 取那一点也是 `FORM`；同样不折、直接点禁用的"Uploading…"的中心，点击落到按钮行的 `DIV` 上，记成 `["DIV", 1]`。所以 `FORM` 是因为那一点落在空隙里，实施者与 C 先前写的推断不对；变异照样红在 `:290`，测试的结论不受影响。

没有格守的：决定 960 的时刻那道守卫在 e2e 里没有只属于它的格（桌面上的 Playwright 没有触屏的连点，只有 both 时 `dblclick` 才红），由 vitest 的格 B 守；rc 在本机字体下没有故事抓（vitest 两句类名守着，rcw 证 hi2 的位置断言是活的）；hi2 的"不与面包屑相交"一句没有单独红过（rcw 先红在 `:118`，同一种毛病的两种量法，C 不另要求）；0.01 px 的余量照处置不做变异。等价的：t49 对 as1；49 照设计的写法在最后一个提交上（在 `51125487` 上红）。

## 门禁与 CI

| 提交 | `heavy.sh make lint-web knip test-web` | `CI=1 heavy.sh make e2e`（过 / 败 / 跳） |
|---|---|---|
| `51125487` | 绿，233 个文件、5588 格（提交之前一次，提交之后注释折行重跑，71 s） | 643 / 3 / 1，2.9 min（181 秒，含构建）；PG15 过；另 lw8:903 [webkit] 一次偶发，单独重跑 LW8 [webkit] 只剩 lw8:836 |
| `066c84b1` | 绿，233 / 5592（69 s） | 644 / 2 / 1，2.9 min（177 秒）；PG15 过 |
| `22c693f8` | 绿，233 / 5592（70 s） | 不跑（只改 vitest 的格），由合并之前的整条命令覆盖 |
| `9de4aed9` | 绿，233 / 5593（64 s） | 643 / 3 / 1，166 秒 |
| `9e6594a5` | 绿，233 / 5594（64 s） | 643 / 3 / 1，168 秒 |
| `e8aa3b36` | 绿，233 / 5594（63 s） | 643 / 3 / 1，168 秒 |
| `ff3e3788` | 绿，233 / 5595（67 s） | 644 / 2 / 1，171 秒；PG15 过 |
| `35bcc7bc` | 绿，233 / 5597（64 s） | 643 / 3 / 1，171 秒 |
| `8a89e184` | 绿，233 / 5598（64 s），oxlint 零警告 | 643 / 3 / 1，166 秒 |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `22c693f8`（实施者；C） | 实施者：lint-web（oxlint 0 警告、oxfmt、`check.mjs`）、knip、vitest 233 / 5592、`gen-check-web`、`build-web` 都绿；E2E 643 / 3 / 1，2.9 min；整条 246 秒（负载 5.3 → 23.7）；UP1 (page) Chromium / Firefox / WebKit 4.3 / 5.8 / 6.2 s，hi2 (page) 2.9 s，hi1 (page) 1.8 s，as1 (page) 3.1 s。C（锁当即拿到，01:54:49–01:59:01）：vitest 5592（63.88 s）；E2E 643 / 3 / 1，2.8 min，lw8:903 也过了；整条 252 秒（负载 15.85 → 59.20）；UP1 (page) 3.6 / 5.4 / 6.7 s，hi2 3.0 s，hi1 2.0 s，as1 3.3 s |
| `9e6594a5`（实施者） | vitest 233 / 5594；`gen-check-web`、`build-web` 过；E2E 643 / 3 / 1；整条 234 秒（负载 24 → 32） |
| `e8aa3b36`（实施者；C） | 实施者：vitest 5594；E2E 643 / 3 / 1；整条 234 秒（负载 25 → 32）。C：vitest 5594；E2E 643 / 3 / 1；整条 247 秒（负载 7.30 → 43.70） |
| `ff3e3788`（实施者；C） | 实施者：vitest 5595；E2E 644 / 2 / 1（PG15 过了）；整条 240 秒（负载 22 → 24）。C：vitest 5595；E2E 643 / 3 / 1，up3、lw3（三种引擎）、tr2、hi5 都过；整条 246 秒（负载 13.89 → 82.15） |
| `35bcc7bc`（实施者；C） | 实施者：vitest 5597；E2E 643 / 3 / 1；整条 237 秒（负载 24.74 → 25.36）。C：vitest 5597；E2E 643 / 3 / 1；整条 251 秒（负载 9.49 → 58.49） |
| `8a89e184`（实施者；C） | 实施者：vitest 233 个文件、5598 格；E2E 643 / 3 / 1；整条 241 秒（负载 27.08 → 18.14）。C：vitest 233 / 5598，oxlint 零警告；E2E 647 格，643 过、3 败、1 跳，tr2、up3、lw3（三种引擎）、hi5 都过；整条 250 秒（负载 7.02 → 54.40） |
| `32045048`（主持者；文档提交，加上它的头即合并 `75f020a4` 的树，`m13-ui-refresh` 在这期间没有动过） | 与 `8a89e184` 只差 `docs/`（`git diff 8a89e184..32045048 -- . ':(exclude)docs'` 为空），合并之前的整条命令已在 `8a89e184` 上跑过（实施者 241 秒、C 250 秒），不重跑；主持者在文档脚本之后另改了 8.2 引言里过时的三处（第 11–17 条、21 处替换、第 17 条的日期）；`git diff --check` 与 `check_links.py m13-ui-refresh`（7 个文件，0 个新的坏链接）干净 |

- 每次都只败基线里同行同消息的格：SE2 [chromium] se2:231、PG15 [webkit] pg15:113（时红时过，过的四次见"基线"）、LW8 [webkit] lw8:836；`make` 只因这几格以 2 退出。`check.mjs` 的四个数每次相同：md-fixtures 78 cases、47 rename、27 resolution、65 render；e2e-navigation 199 files，a15 ×3；e2e-projects chromium 434、firefox 105、webkit 105、webkit-narrow 3；licenses 10 cells。"not configured to support act"在 S3 的三个提交每次 0 次；C 在 `22c693f8` 上 0 次，"Not implemented: navigation to another Document"照旧一次。
- 受保护路径的 `git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`git diff --check m13-ui-refresh...HEAD` 0（C 另核对各轮的增量）；`m13-ui-refresh` 一直是 `293d016b`，分支可快进；跑完 `git status` 为空。
- 合并之前的整条命令最后一次在 `8a89e184`（实施者 241 秒、C 250 秒）；之后只有本文档提交。
- vitest：5564 → 5588 → 5592 → 5592 → 5593 → 5594 → 5594 → 5595 → 5597 → 5598（一直 233 个文件；S3 的三个提交之后 5592，修复多 6 格）。E2E 一直 647 格（没有新的故事，只在已有的格里加步）；`check.mjs` 不变。
- **时长**（7.5，00 号文档第 9 节的"P5 之后记一次时间"）：S3 的三个提交与 C 第一轮的 E2E 2.8–2.9 min（647 格）；实施者每次的 `make e2e` 166–181 秒（含构建）；合并之前的整条命令实施者 234–246 秒、C 247–252 秒；UP1 (page) 多了版本页三种宽度的扣住的读，比 S2 的 3.0–3.1 / 4.5–5.3 / 4.5–5.6 s 慢一些。照这几次推断余量照旧足够，CI 的 E2E 一步的数要等负责人推送之后。
- **CI**：最近的一次是负责人推送 `af33b81e`（P5/S1 合并之后）之后分支的 CI（run 45631867718），全绿，E2E 一步 8 分 5 秒；release 的演练（run 45631867695）也绿。S3 还没有在 CI 上跑过，等负责人推送（决定 746：没有推送的凭据时以本机门禁替代分支 CI，负责人推送之后 CI 在最后一个头上补跑）。
- **只靠 CI 的**（S3）：hi2 320 宽一段的三句按位置的断言（`hi2:117-119`、`:124`）在 Linux 的字体下，rc 也只在那里由它抓（按钮超过 160 px 时）；UP1 (page) 的版本页一步（`up1:319`）在 Linux 的字体下；tr2:290 的点击记录：320 宽时双击"Import"的第二下落在折上来的"Stop the upload"上靠版面，不成立时这一句直接红（第二下记成别的按钮或 `FORM`），不会悄悄地过，那时照实改这一句，计数那一道仍由之后计数为 2 的那一下（`tr2:299-302`）与 vitest 的格 A 守；lw3、up3、hi5 的 `expectButtonsInside`（框宽由窗口定、贴边的一侧落在整数上，严格比较只留 0.01 px，C 第 2 轮推断在 Linux 下照样成立，没有跑过）。另有实施者 10.3 草稿列的右栏版本行与横幅小圆点在 Linux 的字体下，与 B 量到贴着边的 Cancel + Submit for review（237.2–237.5 对内宽 238，折行也无害）。
- **先写、先红**（C 的 P5S3C-N2 更正之后；日志在会话的临时目录）：

| 提交 | vitest | e2e |
|---|---|---|
| `51125487`（提交 1） | 新格与改的格在没改的实现（`293d016b` 的 10 个实现文件）上 9 个文件、29 格红：表的 8 行都红在第 2 句的 `mx-auto`（`upgrade-banner.test.tsx:290`），`version-page` 10 格，`page-versions` 3、`upgrade-page` 3，`list-rows`、`health-page`、`history-page`、`upgrade-check`、`version-restore` 各 1（实施者 `p5s3/red/c1-vitest.log`；C 用提交里的测试复现，数相同，`p5s3-rev-c/mut/red-c1.log`） | hi2 红在 `hi2:102`（320 宽 `lines` 得 2），之后那一句没走到；hi1 红在 `hi1:259`（"· EditCurrent"）；UP1 (page) 的版本页一步在旧树上绿，由 t32、t33 证（`p5s3/red/c1-e2e-red.log`、`c1-e2e-red-up1.log`） |
| `066c84b1`（提交 2） | 真正的先红：`066c84b1` 的测试、`51125487` 的 11 个实现文件，7 个文件、10 格红：先写的 8 格（附件、上传、来源附件两格、横幅、粘贴来源、导入，与 `source-uploads`"关上之后"一格），另 `source-uploads` 的"generation change""logout"两格因 `:462` 那一格中途红、留下离开的提示，红在 `:538` 的 `warnsBeforeLeaving()`，是连带红（C 的 `red-c2.log`）；实施者报的"6 个文件 7 格"（`p5s3/red/c2-vitest.log`）那次跑的树不是没改的，`source-uploads` 改的那一格读的 `cardClass` 是绿的；同一次跑里表的 36 行都绿 | as1 那一句在旧树上绿，由 56 证；表在提交 2 里不先红，由 57、58、61、fb1、fb2 证 |
| `22c693f8` | 补的断言由那 8 条变异证 | — |
| `9de4aed9`、`9e6594a5` | `22c693f8` 加上新测试、不改代码：7 个文件 256 格中 9 格红（`p5s3/red1-vitest.log`）：`page-versions` `:147`（时间 Link 的类名）、`version-page` 头一行 `:748` 与早退表 3 行 `:855`（期望 `min-w-fit`）、`health-rule` 新格 `:319`（第一个子元素是 H2，期望 OUTPUT）、`source-add-dialog` `:463` 与 `transfer-import` `:485`（按钮一行是 `flex justify-end gap-2`）、`upgrade-page` `:2157`（两节不带 `text-sm`）；排在后面的两句（"更多"的 `small`、种类的 nowrap）没轮到，由 n1、n2b 证；cxa、cxb、cxc、cxx 与 957 的来源页一行是守卫，由变异证 | `make build` 之后跑 hi2、tr2、lw3，27 过 4 红（`p5s3/red1-spec.log`）：tr2 [chromium] 红在 `tr2:264`（"Keep uploading's left" −13.47，要求 ≥ 58）；lw3:287 三种引擎红在 `lw3:366`（"Keep editing's left" Chromium −13.15、Firefox −14.33、WebKit −13.74，要求 ≥ 58.5、58、58.1）；同一刻的 `expectFits` 过（从左边溢出不加 `scrollWidth`，05-lessons 第 6 节）；hi2 是守卫，由 cxr 证 |
| `e8aa3b36` | `9e6594a5` 加新断言、代码不改，3 个文件 78 格中 3 格红，实际都是 "flex justify-end gap-2"：`confirm-dialog.test.tsx:101`、`revert-background.test.tsx:103`、`transfer-import.test.tsx:471`（`p5s3/red2-vitest.log`） | 三个 page 故事（Chromium，17 s）都红在 `expectButtonsInside`：`up3:281`（"Cancel's left" 15.71，要求 ≥ 41.69）、`tr2:261`（"Stop the upload's left" 19.17，要求 ≥ 40.5）、`hi5:184`（"Cancel's left" −0.59，要求 ≥ 40.5）（`p5s3/red2-spec.log`） |
| `ff3e3788` | `e8aa3b36` 的代码上 80 格中 2 格红：`health-rule.test.tsx:318`（前两个子元素是 OUTPUT、H2）、`transfer-import.test.tsx:538`（`detail` 2 的点按之后 `aborted` 为 true）（`p5s3/red3-vitest.log`） | tr2 红在 `tr2:261`（双击的第二下停了上传，"Stop the upload"不在，`red3a-spec.log`）；I1 两步对照：只严格、不等弹入时 up3 ×8 全红（"Cancel's right" 275.43，要求 ≤ 274.68，`red3c-spec.log`），lw3 三种引擎各 2 次红 4 次（Chromium 2、WebKit 1、Firefox 1，`red3d-spec.log`）；带 0.5 px 的旧版这一次 up3 ×8 全过（`red3b-spec.log`） |
| `35bcc7bc` | `ff3e3788` 的代码上 46 格中 2 格红：格 B（当时 `:568`，0 ms 的 `detail` 1 停了，`aborted` 为 true）、格 C（`:589`，重复的 Enter 没有被 `preventDefault`）（`p5s3/red4-vitest.log`）；格 A 守的是已有的计数守卫，在旧代码上本来就过，由 960c 证 | tr2 新加的计数为 2 的点按在旧代码上过（计数守卫已在），由 960c 证；A 量过 `293d016b`（没有守卫）上会停 |
| `8a89e184` | 格 D 放在 `35bcc7bc` 的代码上，47 格中 1 格红（`:624`，Import 上重复的 Enter 没被拦，`p5s3/red5-vitest.log`）；改过的格 B 放在 `35bcc7bc` 加 960a 上红在 `:581`（格 D 也红，那时 N1 的代码还没改，`red5-960a-vitest.log`），改之前的格 B 在 960a 下过（C 的 `probe960a.py`） | tr2 的点击记录在 `35bcc7bc` 的版面上过，由 f2n1 证 |

- **对比度**（05-P5 4.4 没有给 S3 的下限，S3 也没有改 se1、pg4、l5 量的元素，三个故事每次都过，se1 的值与 S2 记的相同；实施者以 `e2e/fixtures/contrast.ts` 的量法照实量，Chromium、1440 宽，探针的第二格，数在 `p5s3/shots/contrast.json`；亮 / 暗）：版本页的"当前"（琥珀在 `accent-soft` 上）2.757 / 6.101，右栏的同值；普通的小块（`markClass`：编辑中、历史的 Undone）5.625 / 6.159；版本页标题下的事实（faint）3.065 / 3.801，作者的链接（muted）6.183 / 7.017；附件的名字 17.578 / 12.987，大小与下载图标 3.065 / 3.508（卡片 `#212226` 上），拖放说明 3.065 / 3.801，"操作"显出时 6.183 / 6.475；健康的 `noteClass` 6.183 / 7.017，卡片标题与行的路径 17.578 / 12.987，卡片里的 `factsClass` 6.183 / 6.475，clear 的 summary 6.183 / 7.017；升级的节标题 17.578 / 14.074，检查的说明 6.183 / 7.017；横幅状态的字 15.992 / 12.353，小圆点（边色当字色量）2.818 / 6.954；粘贴来源的文本框键盘聚焦时 2 px 琥珀的 ring 3.098 / 7.311（与 se1 的 button、select、暗色 link 同值），说明 6.183 / 6.475。低于 4.5 的只有琥珀与 faint 的几项，都在 00 号文档 4.7"可以为美观让步"的那几类（琥珀标状态、说明文字、焦点圈），P4 起的同一串类名、同一些颜色；正文都在 7 之上（12.3–17.6）；没有设新的下限。审查 B 量的各值与实施者相同，且不低于样稿自己的；`markClass` 那一项 B 的会话过期没量到，沿用实施者的。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库；量出的值在 05-P5 10.3"视觉的核对"。

- **实施者**：`p5s3/shots/{chromium,firefox,webkit}/`，每种约 260 张与 `measures.json`、`extras*.json`（15 个视图：旧版本、最新版本带"当前""编辑中"与锁说明、三种长名字的版本、页与右栏、笔记本首页的附件、健康、升级、横幅在首页、历史、版本、设置，变更集、预览；英文亮暗 1440、1024、768、375、320 + 中文亮色 1440、375、320；另有右栏版本行、版本页 375 与 320、横幅的小圆点、附件的静止、悬停、聚焦、菜单开着、触屏，上传中与拖进来的环，Undone 悬停，两个对话框，Chromium 的强制色，健康展开；长名字三种：连字符加数字、没有断点、长中文），样稿的截图 `p5s3/shots/mockup-{settings,reading}-{light,dark}.png`；探针 `p5s3/probe/ui1-zz-p5s3-visual.spec.ts`。实量：横向溢出 0（只有页 320 宽时链接里一个 sr-only 的 `span` 的框在视口外，被裁、不占位）；版本页 320 宽标题一行，Restore 在头一行、面包屑之下靠右（x 145.5、y 93.5、150.5×30）；375 宽版本列表扣住、读到之后、写者看最新版本头一行都是 55.5（P5A-M4）；右栏版本行 217 px 与 200–240 px 逐 1 px 的扫描里单独一行"-"、断开的小块、溢出都是 0；变更集、版本、预览标题下的事实同 12.5 px、`#91939b`、行高 18.75、离标题 14 px；横幅在每类页的列里第一个，小圆点 6 px、3 px 琥珀边，中心与状态一行的行框中心相同、与第一行字形中心差 ≤ 0.5（中文 375、320 两行，英文 320 三行）；"操作"静止 0、悬停与键盘聚焦 1（Chromium、Firefox）、菜单开着指针移开 1、触屏 1；两处拖放的环同为 4 px 页底色间隔加 2 px 琥珀。对照 settings.html 与 reading.html：卡片、小徽记、头一行、32 px 标题、720 px 一列、标题下的事实一致；右栏的"当前"照决定 943 不在行尾（样稿在行尾）。修复轮：`p5s3/shots1/{chromium,firefox,webkit}/`（`fix1.json`、`fix1-health.json`；问的框 `fix1-asking-*`、健康卡片、版本行、升级页两节、还原那一格撑长字 `fix1-restore-*`、别的对话框 `fix1-rows-abandon-320.png` 与 `fix1-rows-restore-320.png`），对照组 `p5s3/shots1-rc/`、`shots1-n2/`、`shots1-m1/`，探针 `p5s3/probe1/ui1-zz-p5s3-fix1.spec.ts`；958 扩到的三行 `p5s3/shots2/{chromium,firefox,webkit}/`（`fix2-fix2.json`、`fix2-abandon-*`、`fix2-import-sending-*`、`fix2-revert-background-*`），探针 `ui1-zz-p5s3-fix2.spec.ts`；焦点 `p5s3/shots3/{chromium,firefox,webkit}/focus-*.json`，探针 `ui1-zz-p5s3-focus.spec.ts`。修复之后的实量（三种引擎）：问的框 1440、375、320 宽两个按钮都在框的内边之内，英文 375、320 各占一行、靠右（右边 317、262，内宽 259、204），中文一行（要 201–202，内宽 204）；健康卡片与升级页的检查卡片 17 / 17（对照组没有"更多"的 17 / 25）；"更多"30 px 胶囊；版本行 196–244 px 单独一个字（含"·"）、单独的"-"、断开的小块、溢出都是 0（对照组 n2a、n2b 一起改回，中文 Chromium 四个页 62 / 202 / 13 / 62，Firefox 42 / 7 / 7 / 7，WebKit 35 / 5 / 5 / 5）；升级页两节 14 px、正文色，检查的说明仍 13 px 灰；还原撑到 0.5em 都在格内、不压面包屑、在标题之上、页不加宽，只有 320 宽 0.5em 时按钮 280 px 比头一行的 272 宽、伸出 8.45 px，页仍是 320；958 的三行英文 320 内宽 238、各占一行、右边都在 279（Abandon the upgrade 从 101.9、Undo in the background 从 86.4–86.8、Stop the upload 从 140 开始），375 一行，中文一行。
- **审查 A**：`p5s3-rev-a/vitest/zz-rev-a-probe.test.tsx`（10 格：表外的横幅分支 8 行、写者打开来源页的版本、真读者打开旧版本）、`zz-rev-a-strict.test.tsx`（StrictMode 下 5 格），浏览器探针 `p5s3-rev-a/probe/ui1-zz-rev-a.spec.ts` 与 `probe/out/`（A1 真读者，A2 写者与来源页，A3 还原对话框与 Tab 次序，A4 附件"操作"的显出，A5 升级中每个笔记本页的横幅）；修复核对 `f1/ui1-zz-rev-a-f1.spec.ts` 与 `f1/out/`（F1 320 宽双击 Import，F2 还原撑长字，F3 各宽度的头一行，F4 `output` 的位置，F5 Abandon 的确认框）、`f2/`（G1–G3 Stop 的守卫与 `output`，G4 焦点的时间线，`out-run3-base-measures.txt` 是 `293d016b` 的对照）、`f3/`（960 与按住的键）、`f4/`（按住的键与点击记录）。
- **审查 B**：第一轮探针 `p5s3-rev-b/probe/ui1-zz-p5s3-rev-b.spec.ts`，`r1/`–`r4/`（`22c693f8` 原样），`f1/`、`f2/`（另一棵树上试修复，补丁 `fix-experiment.patch`），`cn.json`；修复核对 `p5s3-rev-b/fc1/ui1-zz-p5s3-rev-b-fc1.spec.ts` 与 `c1/{chromium,firefox,webkit}/`，汇总 `fc1/analyze.py`。对照样稿：版本页的头一行、32 px 标题、720 px 一列同 reading.html；健康与升级的卡片 14 px 圆角、1 px 边、底亮 `#ffffff`、暗 `#212226`；问的框 8 px 圆角、没有底，暗色下框里的描边按钮看得出；粘贴来源的文本框聚焦时 2 px 琥珀 ring，强制色下 2 px 实线外框；历史的"Undone"`rounded-full`、11 px，悬停时边变 `border-strong`、字变正文色、没有下划线。
- **审查 C**：`p5s3-rev-c/gate.log` 与 `fix1/`–`fix4/gate.log`（五次门禁），`mut/`（重放与自己的变异、`red-c1.log`、`red-c2.log`、`49orig.log`，弹入的探针 `probe-scale.py`、`probe2.py`，动画取消的 `anim-cancel.cjs`，连跑 `rep.py`，`probe960a.py`），`docs/`（`replay.py`、`replay-proposed.py`、`replay-f1.py`–`replay-f4.py` 与各次的输出，8.2 那一段 `8.2-S3-block-f4.md`）。

## 接受与推后

- **B-N3**：Firefox 320 宽、连字符加数字的长名字，版本页（与页、变更集）头一行的长面包屑有一行只有"/"（`breadcrumbs.tsx:48` 起的 `BreadcrumbTrail`，P4 的）：交给收尾（05-P5 第 9 节），例如让分隔符不与前一段分开。
- **958 不改的七行**：`create-token-dialog.tsx:146`、`unsaved-guard.tsx:88`、`asset-dialogs.tsx:234`、`version-restore.tsx:199`、`create-notebook-dialog.tsx:132`、`source-add-dialog.tsx:285`、`move-page-dialog.tsx:193`（最后的树上行号相同）在 320 宽中英都放得下（实施者 69 对的量法；B 没有另量），不改。共用的 `ConfirmDialog` 改了之后，Firefox 320 宽"开始升级"的确认（Cancel + Start the upgrade，238.1 对内宽 238；Chromium、WebKit 237.9 一行）折成两行：修之前是看不见的 0.1 px 溢出，现在照 958 折行、在框内，是想要的样子（B）；实施者"英文其余用法都本来就放得下"在 Firefox 差 0.1 px，照实记。
- **C-F1-N3**：`expectFits` 量对话框时同样可能在弹入中：缩放只让盒子变小，只会放过、不会误红，量程是 `(1−s)` 乘以半宽，288 宽的对话框最多放过约 4 px；不是 S3 的新代码，交给收尾：量对话框的帮手统一先等弹入结束。
- **A 修复核对第 4 轮顺带看到的两件**（S3 之前就有），交给收尾（第 9 节）：按住 Enter 按 Cancel，对话框关掉后焦点回到"Import a zip…"，下一次重复又把它打开（之后的重复开了选择框），这是全站对话框关闭时焦点回触发按钮的通则，要管得在对话框那一层；没选文件时按住 Enter 按 Import，校验不过、焦点移到文件框，重复打开选择框，真浏览器里选择框是模态的、只开一次，可以接受（要管可以改成"重复的目标与按下时的不同就拦"）。
- **决定 957 的位移**：写者打开来源页的版本，375、320 宽列的上边 153.5 → 134（A），768 宽读到之后长的来源根面包屑折了一点（头一行 36 → 39），`h1` 从 78 到 81，下移 3 px（B 修复核对第 1 轮新量），1440 不动；只发生在写者身上，写者看不到横幅，挪动的只有"加载中…"。
- **A-F1-N2**：字宽到按钮比 160 px 宽时，有一段头一行的宽度上（约按钮宽减 160 px）"还原"的到来多出一行；本机字体下没有。决定 929 的照实记一句；彻底去掉要让空的格也按按钮的宽排（例如一个 `invisible`、`aria-hidden` 的同文字占位），不值得（A）。
- **960 的代价**：上传开始之后 500 ms 内有意的指针点按不停，再按一下就停（A 第 3 轮：不显得不灵）；时刻那道守卫在 e2e 里没有自己的格，只由 vitest 的格 B 守（C 第 3 轮，照实）。
- **B 未核实的**与别的未核实的，照实列在下一节。

## 未核实的怀疑

- **Linux 的字体**（A、B、C、实施者）：hi2 的三句位置断言（与 rc）、UP1 (page) 的版本页一步、tr2:290 的点击记录、lw3、up3、hi5 的 `expectButtonsInside`，与右栏版本行、横幅小圆点、贴着边的按钮对，都只在 macOS 上跑过，等负责人推送之后的 CI。C 推算 320 宽"Version 1"约 175 px、375 宽面包屑约 190–220 px，都不该变，没有量。
- **触屏连点的真机**（A 修复核对第 2 轮）：iOS Safari、Android Chrome、Firefox for Android 上连点第二下的 `detail` 没有核实；决定 960 不论哪种都挡住，只在 Playwright 的触屏模拟里量过。
- **真键盘按住**（A 修复核对第 2–4 轮）：重复的首次延迟与间隔由系统设置定，只用 Playwright 发了重复的 keydown；结论只依赖"重复的 keydown 会按到按钮、会被拦"，三种引擎都验证了。
- **WebKit 里键盘聚焦"操作"**（B、C、实施者）：WebKit 的 Tab 默认跳过链接，实施者的探针在 WebKit 里没有让焦点进那一行；B 用 Alt+Tab 量到焦点在本行的链接上时是 1，焦点直接落在"操作"上的那一次按的是普通 Tab、焦点离开了这一行，量到的 0 是探针的问题；A 的 A4 以 `focus()` 量到 1；用键盘直接落在"操作"上的那一种没有量，类名由 vitest 守。
- **lw8:903 [webkit] 那次的原因**（C）：`browser.newContext` 的错误紧接在 :822 失败之后，像是 WebKit 的 worker 在前一格失败之后重启时撞上的，没有查。
- **957 面包屑只有一行时的位移**（B）：B 推算 375 宽头一行从 55.5 变成 19.5、`h1` 上移约 36 px，没有量；A 在自己的数据里量到列的上边 153.5 → 134（头一行 55.5 → 36），处置写"约 20–36 px"。
- **上传中的行比附件的行高约 11 px**（A 第一轮，归 B）：实施者的 `shots/chromium/uploads-light.png` 上上传行带一个 36 px 的"Cancel"，约 57 px 高，附件行约 45 px；B 没有量、没有提。
- **B 未核实的**：拖进来时的琥珀环 B 没有渲染（两处类名相同，`attachment-drop.tsx:55` 与 `attachments-section.tsx:216`，实施者截了）；"Continue the rollback"与"有未保存的改动时退出登录"两个确认框没有真打开，只用标签对的检查；958 没改的七行 B 没有另量；B 没有跑 vitest 与 e2e。B 第一轮另两条（中文导入的问的框在 320 宽、有条目没有"更多"的卡片）在修复核对第 1 轮核实了（一行；17 / 17）。
- **两次 `evaluate` 之间框被换掉**（C 修复核对第 2 轮）：`expectButtonsInside` 等的是第一次 `evaluate` 那一刻的元素，框若在两次之间换成新元素，新元素的弹入不会被等；现在的五处都在框出现、停稳之后量，中间没有会重画框的动作，不算发现。
