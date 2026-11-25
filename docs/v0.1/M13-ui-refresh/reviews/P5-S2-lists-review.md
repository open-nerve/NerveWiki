# M13/P5/S2 列表与筛选：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p5-s2`（从 `m13-ui-refresh` 的 `af33b81e` 拉出，可快进），十一个提交：`1a7a735e`（照 6.2 提交 1：搜索、历史与变更集页是列表类的页，标题、筛选条与卡片里同一种行，"只看此人"悬停或聚焦时显出，变更集页的头同页的头、"撤销这次改动"在头一行，通往别处的链接带 `arrived`）、`dbf2c2cf`（照 6.2 提交 2：回收站与预览、目录、标签同样排，空列表是空状态，预览标题与"在后台恢复"里的长名字折行，链接带 `arrived`）、`75578e08`（照实加的：变异 t3、t7、t25、t36、t50–t53、t56、t57 的类名断言，只改测试）、`c238d24d`（照实加的：40b 的次序断言，只改测试）；第一轮审查之后的 `74b22728`、`48c0eaa1`、`446d2463`、`9c97fc79`、`df0437d2`；修复核对第 1 轮之后的 `4730ddd3`、`2c0af01a`。另有本记录所在的文档提交。 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的分离工作树里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为与正确性（读 diff 与设计；13 条变异，vitest 与 e2e 探针，StrictMode 下的到达；修复核对时量 320 宽的警示对话框与 953 的两种设备），B 管外观（自己的探针，37 个视图 × 13 种条件 × 三种引擎，三种长名字；952 另开一棵树改前改后对照；修复核对时整套探针重跑，以 `--blink-settings` 起"触屏笔记本"看 953），C 管测试、e2e、变异、门禁与文档的就绪（逐格读，重放实施者的变异，自己的变异与探针，合并之前的整条命令，8.2 的替换在总体设计的副本上重放）。修复核对一轮，三位；C 在那一轮的两条 Nit 由主持者核对（02-workflow 3.4）。 |
| 结论 | 实施中：变异 t3、t7、t25、t36、t50–t53、t56、t57 在 `dbf2c2cf` 上活着，照实加 `75578e08`；40 的原写法与设计写的抓手等价，重做成 40b 又活着，照实加 `c238d24d`（同 S1 `9975478d` 的先例）。实施者另交出一个发现：422 的提示（共用的 `Alert`）不折没有断点的名字（偏离 g）。第一轮：A 1 Minor、2 Nit、1 Q；B 2 Minor、4 Nit、1 Q；C 3 Important、3 Minor、6 Nit、2 Q。同一件的：A-M1 = C-I2（按 Important 处置）；A-N1 = B-N4 = C-N1。全部接受（B-N1 照处置先试、撤回），主持者的决定 952–954，五个修复提交。修复核对第 1 轮：A、B 没有新的发现（A 更正了第一轮 N2 的一处说法）；C 2 Nit，全部接受，两个只改测试的提交，由主持者核对。三位都没有未了的 Important、Minor。合并 `<待填>`。 |

## 基线

S2 不另跑基线（05-P5 7.3，同 P4 的决定 883）：`m13-ui-refresh` 是 `af33b81e`（P5/S1 合并 `326034d3` 之后记合并的文档提交），提交 1 的 `make e2e` 兼作核对。对照的数是 S1 之后的：vitest 231 个文件、5504 格；E2E 646 格（642 过、3 败、1 跳）；`check.mjs` chromium 433、firefox 105、webkit 105、webkit-narrow 3。已知的本机失败同 [P5/S1 的记录](P5-S1-homes-entry-review.md)：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S2 的每次 E2E 里 SE2、LW8 都红，同行同消息；PG15 在提交 1、2 的 `make e2e` 里过了，之后实施者的七次与审查 C 的两次都红。SE2 那一格的"在哪里找"在 S2 换了样子（修复之后带 `max-w-64`），行号与消息照旧（期望 shared.id、得 ""）。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 变异 t3、t7、t25、t36、t50–t53、t56、t57（都是只改样子的类名：回收站行的图标的灰、目录一行的摘要与类型、回收站页的"在后台恢复"、"Undone before"、变更集的消息、改动行的次要链接、历史行的计数与来源）在 `dbf2c2cf` 上的全部相关测试下活着 | 照实加 `75578e08`：各补一句类名断言，十条在新提交上都红 |
| 变异 40（"恢复"排在链接之前）初写时挪按钮丢了 `ms-auto`，红的是类名；保留类名的 40b 下，设计写的抓手 `trash-page.test.tsx:446-447`（今天 `:457-458`）的两次 Tab 等价，活着 | 照实加 `c238d24d`：`list-rows.test.tsx` 回收站一行补"链接在恢复之前"（`compareDocumentPosition`），40b 红在 `:90` |
| 变异 42 在 TB1 上不红：Chromium 在"`Notes-2026-11-25-…`"的连字符之后断，名字作为匿名弹性项也不溢出；把名字换成只有字母的，没变异的代码也红（422 的提示不折没有断点的名字，375 宽页宽 552） | 名字照设计留着，42 由 vitest 守（42v），提示交主持者（偏离 g）；第一轮之后成决定 952 与 C-I1，`74b22728` 换名 |
| 预览没有面包屑的两支：4.2.8 的字面（不带 `docShellClass`、没有头一行）写在决定 946 之前，照字面画横幅会上移一个头一行 | 照 946 画 `EmptyHead`（偏离 a），从 `page-layout.tsx` 导出；pc6 守 |
| `trash-preview.test.tsx:554`（M8/P2B 的"从列表打开的预览不取焦点"）与决定 940 相反，7.1 没列 | 原地改写成到达（偏离 c）；它另守的一半由 `48c0eaa1` 补（A-M1、C-I2） |
| 预览的新格加在 `trash-preview.test.tsx` 末尾要多三行 import，`:587` 会下移 | 放进新文件 `trash-preview-head.test.tsx`（偏离 d） |
| TB1 新格先红在第二个 `expectFits`（页宽 724），不在设计说的第一个 | 照实（偏离 e） |
| UP1 (page) 变更集那一步：面包屑末尾是作者与时刻，e2e 的显示名约 50 个字符，768、375 宽折成几行，读到之后的头一行更高 | 断言写成 `read.y − max(0, trail − 36) = loading.y`，x 与宽严格相等（偏离 h，决定 946 的照实）；审查 C 的 cR 证它是守卫 |
| 提交之前没过的门禁：`dbf2c2cf` 的 oxlint 两条 warning（`trash-preview-head` 的不安全可选链、`upgrade-banner` 的 `no-map-spread`）；`74b22728` 的 oxlint 两条 `jsx-key`（`dialog.test.tsx` 的 `test.each` 里直接放了 JSX，改成函数） | 改了才提交 |
| 处置的 B-M2 只加 `py-1.5`：探针量出一行的小块 44 px（30 px 的"移除"加 12 加 2） | "移除"另加 `-my-1.5`，一行仍 36 px（修复之后主持者定，接受；变异 m2b） |
| 处置的 B-N1 先试 `max-md:-ms-3.25`：不折时按钮盒子压进链接 1 px，焦点环压在最后一个字上 | 撤回（处置允许；修复之后主持者定） |
| A-N2 的先红：旧的 `bannerPlaces` 遇到"Page not found"不是照过，而是量面包屑时等到 30 s 超时 | 照实；A 修复核对时更正 |
| 起草文档提交时在总体设计的副本上接着重放 S3 的第 11–16 条：第 11、15 条的旧文字锚在 S2 初写的两处句尾（"……"撤销这次改动""恢复"在它的右边（M13/P5/S2）。""mc3 另核对 375 宽时它静止也显着。"），C 的文字（P5S2C-M2、M3）改了这两处，各 0 次 | 本文档提交：两条的旧文字换成 S2 新文字的句尾，新文字照旧接在其后；脚本照样重放第 11–16 条，各恰好一次（不写进总体设计） |

## 偶发

没有。实施者与审查 C 的各次 E2E 都只败基线的格（PG15 两次过了，见"基线"）；AS6、`lw8:771`、"not slower"、`trash-preview.test.tsx:587`、造数的 500、WebKit 的"mermaid could not be loaded"、smoke S1 一次也没有遇到。

## 第一轮的发现与处置

在 `c238d24d` 上（对照 `af33b81e`）。

### 审查 A（行为与正确性）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P5S2A-M1 | Minor | 改写的 `trash-preview.test.tsx:551-561`（偏离 c）不再守"预览自己不取焦点"：`Preview` 的 effect 只在并入或交接时取焦点，变异 r7（条件恒真）在回收站四个测试文件下活着；按地址打开的预览会抢走焦点 | 按 Important 处置（= C-I2），`48c0eaa1`：`trash-preview-head.test.tsx` 补"按地址打开、不带状态的预览不取焦点"一格，等文档以物件的名字命名之后断言焦点在 `body`；r7、pvA 只红这一格 |
| P5S2A-N1 | Nit | 变更集页给能写的人传的 `head={headActionsClass}`（`headOf`）对排版没有作用：空的一组是 0×0，升级中没有能写的人；那一格只守类名 | 决定 954，`446d2463`：去掉 `headOf`，写者那一格与 7.2 的 pc2、pc3 去掉 |
| P5S2A-N2 | Nit | UP1 的 `bannerPlaces` 量"读到之后"时等的是任一个 `h1`，读到"Page not found"时也照过 | `446d2463`：`bannerPlaces` 收所等的标题，`exact`；A 修复核对时更正"照过"：旧写法在量面包屑时 30 s 超时 |
| P5S2A-Q1 | Q | `revealClass` 只看 `hover: none`：触屏笔记本一类能悬停、又有手指的设备上，"只看此人"静止时看不见；同一棵树每行的"⋯"用 `any-pointer-coarse` | 决定 953，`9c97fc79` |
| A 另记 | — | 偏离 a–i 逐条核对：a、b、d、e、f、h、i 接受，c 部分接受（即 M1），g 赞成 952（375 宽量过：S2 原样页宽 539，加 `break-words` 之后 375）；r1–r6（各去掉一处早退的 `PageColumn`）都红在表的对应行，r8–r13（各去掉一处 `arrived`）都红；StrictMode 下三处到达之后焦点照旧在标题上；真浏览器里回收站行 → 预览、改动行 → 历史的 `h1` 拿到焦点 | — |

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P5S2B-M1 | Minor | 回收站的行与搜索的命中里，名字折成两行时 16 px 的图标比第一行的中线低约 10 px，三行时约 20 px（`lineTitleClass` 是 `items-center`）；预览标题对着第一行，同一个 Step 两种对法 | `df0437d2`：行里的图标加 `mt-0.5 self-start`（`ItemIcon` 的默认串、给 `AssetIcon` 的、搜索命中的三种），预览的 `mt-1.5` 经 `cn` 换掉它；`lineTitleClass` 不改；m1t、m1s 红 |
| P5S2B-M2 | Minor | 历史的筛选小块没有上下内边距，名字折行时字贴着上下边，第一个字落在 18 px 圆角的弧里 | `df0437d2`：`filterChipClass` 加 `py-1.5`，"移除"另加 `-my-1.5`（一行仍 36 px）；m2、m2b 红 |
| P5S2B-N1 | Nit | 窄屏上折到下一行的"只看此人"，字比行里别的字缩进 13 px | 照处置先试 `max-md:-ms-3.25`：不折时按钮压进链接 1 px、焦点环压在最后一个字上，撤回，照实（修复之后主持者定） |
| P5S2B-N2 | Nit | "在哪里找"与目录的"类型"随最长的选项变宽（Chromium、WebKit 634、627 px），"谁"却封顶 256 px | `df0437d2`：两处 `max-w-64`；n2s、n2i 红 |
| P5S2B-N3 | Nit | 预览标题下的事实离标题 10 px、行间 2 px，变更集页 14 与 4 px | 归在 929 之下，`df0437d2`：`cn(docFactsClass, "mt-3.5 space-y-1")`；n3 红 |
| P5S2B-N4 | Nit | 变更集页给能写的人的空头一行传 `headActionsClass`，这块是 0×0，什么也没定形（过度设计） | 同 A-N1，决定 954 |
| P5S2B-Q1 | Q | 952 的 `break-words` 管不到警示对话框里的 `Alert`：`AlertDialogContent` 是没有列模板的 `grid`，删除别人正在编辑的页时提示宽 533 px，伸出 448 px 的对话框，"Delete"被推出框外，改前改后一样 | 决定 952 连同 `AlertDialogContent` 的 `grid-cols-[minmax(0,1fr)]`，`74b22728`；952a、952b 红 |
| B 另记 | — | 横向溢出：37 个视图 × 13 种条件 × 3 种引擎页宽都等于窗口宽，唯一的例外是 952 要修的 422 提示；768 宽约 22 px 的空带（透明的"只看此人"占的一行）是决定 927 的取舍，不另提；偏离 a 接受；变更集页面包屑折行时横幅差折出的那一截是 946 的照实；`cn` 的合并与类名的优先级逐个核对过 | — |

### 审查 C（测试、e2e、变异、门禁与文档的就绪）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P5S2C-I1 | Important | TB1 新格的标题与"在后台恢复"的 `wrap-anywhere` 在今天的名字下守不住：Chromium 在连字符之后断，变异 42 与 bw 下 TB1 照绿 | `74b22728`（与 952 同一个提交）：名字换成 `` `Notes${"quarterlyroadmap".repeat(4)}` ``，格名加 "the refusal"，第二个 `expectFits` 也量提示；42 红在 `tb1:428`（页宽 1180），bw、952a 红在 `:442`；79、80 换名之后重跑都红 |
| P5S2C-I2 | Important | `:554` 改写之后"预览按地址打开不抢焦点"没有格守：变异 pvA 下整套 vitest 233 个文件、5559 格全绿 | 同 A-M1，`48c0eaa1` |
| P5S2C-I3 | Important | UP1 两步"由哪些变异证"的说法不对：39b、45b、82、pc2、pc3、pc5、pc6 都到不了这两步；两步本身是真的守卫（C 的 cR 红在变更集一步，pc4 在预览一步） | 本文档提交：10.2 与本记录写明预览一步由 pc4 证、变更集一步由 cR 证，39b、45b、82 只由表抓；7.2 加 cR |
| P5S2C-M1 | Minor | 变异 33 红在 `se1:305` 的暗色 `box`（5.859 < 8.3），不在"在哪里找"；新下限守不守得住琥珀要另一条变异证 | 本文档提交：7.2 第 33 行照实，加 33L（红在 `se1:322`，`link` 5.096 < 7.21） |
| P5S2C-M2 | Minor | 8.2 第 6 条的新文字把预览的"恢复"写进了头一行 | 本文档提交：照 C 的文字 |
| P5S2C-M3 | Minor | 8.2 第 9、10 条漏了 UP1 的两步与 hi3 的悬停 | 本文档提交：照 C 的文字（换名之后的写法） |
| P5S2C-N1 | Nit | `headOf` 给能写的人的空头一行在排版上什么也不做，那一格只守类名（cW-v 只红在它） | 同 A-N1，决定 954 |
| P5S2C-N2 | Nit | `:554` 改写之后还留着反面格的 50 ms 等待 | `48c0eaa1`：换成 `waitFor` 焦点到标题，行数不变、`:587` 不挪 |
| P5S2C-N3 | Nit | mc3 新加的一句没有先把指针移开 | `9c97fc79`：先 `page.mouse.move(0, 0)`（`:203`），那一句到 `:204` |
| P5S2C-N4 | Nit | 文档里会过时或本来就不对的行号（UP1 的 `:172`、`:268`；草稿 10.2 的 `tb1:429`、`:443`；40 今天的行号） | 本文档提交：行号照最后的树，S1 写的另写"S2 之后" |
| P5S2C-N5 | Nit | 8.2 第 7 条"键盘的焦点进了这一行"比实现窄（`group-focus-within`） | 本文档提交："焦点进了这一行或它自己" |
| P5S2C-N6 | Nit | 草稿 10.2 漏了 931 里 S2 的部分、952、hi3 的悬停、`:554` 改写的后果与句末的"。" | 本文档提交 |
| P5S2C-Q1 | Q | 6.2 提交 3 的清单没有 04-P4 第 9 节，要不要加一行 P5/S2 | 主持者定：加，用 C 的草稿（954 之后的写法） |
| P5S2C-Q2 | Q | 决定 952 要不要进总体设计 13.2 第 31 条 | 主持者定：进，写 `Alert` 与 `AlertDialogContent` 两处（8.2 第 7 条的第二处），第 10 条的变更记录相应加一处 |

C 另核实：`c238d24d` 上合并之前的整条命令（"门禁与 CI"）；前两个提交说明与 6.2 逐字相同（脚本比对），两个照实多的提交写 `web: … (M13/P5/S2)`，四个都以 Co-Authored-By 结尾；8.2 第 6–10 条的旧文字在 `af33b81e` 的总体设计上依次各恰好一次（`replay.py`）；`revealClass` 与设计的串逐字相同（决定 953 之前）；"路由 × 支"表的 20 行每行都走到它名字说的那一支，提交里先红；受保护的路径没有改动，`git diff --check` 干净；导出的新名字都有用处，没有孤儿的文字键。

## 过程的事故

- 修复轮里一次门禁（`446d2463` 那一次）的 e2e 跑着时，实施者对变异的锚点做了一次干跑（`mutfix.py` 的核对：改了立刻还原，毫秒级）；那时构建已经完成，e2e 跑的是构建好的程序，结果不受影响，之后没有再这样。审查 C 修复核对时同样做过一次，照实写了。
- 审查 C 修复核对时整批变异在 `heavy.sh` 里等锁 35 分钟（另一位审查者的探针占着）；C 核对了命令行，只停了自己那个还在等锁的进程，之后 vitest 的变异在锁外跑（不用 Docker 与 PostgreSQL），故事的照旧经 `heavy.sh`。
- 审查 B 修复核对的 `sample` 一段第一次卡住半小时（探针点按钮没有设超时，新建的页与另一页同名，怀疑是这个）；B 只停了自己那一次 Playwright，换名、每步加 20 s 超时再跑，改前改后两棵树用的是改过的同一份探针。
- 主持者要 C 的两条 Nit 一个提交，实施者是两个：`4730ddd3` 的注释里有一行 150 列，`2c0af01a` 重折，顺带折了 S2 原来两处 121 列的注释；都只改测试。
- 别的没有。实施者与三位审查者都只停自己起的进程；临时的探针都在跑时拷进 `e2e/stories/shell/`、跑完即删，几棵树最后的 `git status` 都是空的。

## 实施者交来的（主持者定）

| 实施者报告 | 处置 |
|---|---|
| 第 7 节 a：预览没有面包屑的两支画 `EmptyHead` | 接受（A、B 核对），4.2.8 照实 |
| b：带名字的"在后台恢复"的类名是导出的 `backgroundClass` | 接受，4.2.8 照实 |
| c：`trash-preview.test.tsx:554` 原地改写成到达 | 改写接受；它另守的一半由 A-M1、C-I2 补格（`48c0eaa1`） |
| d：预览的格放进新文件 | 接受 |
| e：TB1 先红在第二个 `expectFits` | 接受；换名之后照实（C-I1） |
| f：UP1 的两步在旧树上是绿的 | 接受；"由哪些变异证"照 C-I3 改写（pc4、cR） |
| g：422 的提示不折没有断点的名字 | 决定 952 |
| h：UP1 变更集一步扣掉面包屑折出的高 | 接受（A、C 核对：升级中没有能写的人，头一行只有面包屑，公式是守卫，cR） |
| i：两个只改测试的照实提交 | 接受（S1 `9975478d` 的先例） |
| 说明 1：`PageColumn` 的选项叫 `head`，变更集页对能写的人传 `headActionsClass`（`headOf`） | 决定 954 去掉 `headOf` |
| 说明 2：768 宽历史行第一行很长时透明的"只看此人"留出约 22 px 的空 | 接受：决定 927"透明不是隐藏"的取舍（B 核对：带高 26 px，只在 768–约 1000 宽且名字约 35 个拉丁字符以上时） |
| 说明 3：名字折成两行时图标对着两行的中线 | B-M1，`df0437d2` |
| 说明 4：回收站页的标题与两句说明照旧包在 `div.space-y-1` 里 | 接受 |
| 说明 6：8.2 第 6 条的第二处留给文档提交 | 本文档提交 |
| 修复报告第 6 节：B-N1 撤回；B-M2 多一个 `-my-1.5`；954 没有先红；A-N2 的先红是 30 s 超时；`dialog.test.tsx` 的格名在 `df0437d2` 里改了文法；953 只改 `revealClass`、齿轮留给收尾 | 前两条与 A-N2 是"修复之后主持者定的"；其余接受 |

## 修复核对第 1 轮

在 `df0437d2` 上（对照 `c238d24d`），三位各在自己的分离工作树里（经 `heavy.sh`）。

- **A**：新的发现：无。四条都修好。pvA 那一格的信号是确定的：同一次提交的被动 effect 在一次同步的冲刷里跑完，`waitFor` 看到文档以物件命名时，取焦点的 effect 必然已经跑过，与声明的次序无关（A 的 f1-r7-after-title：把命名挪到取焦点之前、同时恒取焦点，新格照样红）；新格连跑 10 次全绿，原样的 r7 在 `df0437d2` 上只红这一格。`headOf` 去掉，排版没有差别。`bannerPlaces` 的 `.first()` 不放宽什么：名字精确匹配，页与预览自己的标题总在正文的同名 `h1` 之前。953 的规则在构建出的 CSS 里排在 `.opacity-0` 之后；Chromium 的触屏模拟同时报 `hover: none`，分不出是不是 953 起的作用。952 在 320 宽的"撤销"对话框里核对：对话框 288 宽、不溢出，Cancel 在 Undo 之前、都在框里，打开时焦点在 Cancel，Escape 关上之后焦点回到"Undo change"，页宽 320；`AlertDialogContent` 的五处用处里都只有文字、表单与按钮；用到这两个组件的 21 个 vitest 文件、569 格全绿。**更正**第一轮的 N2：旧写法遇到"Page not found"不是照过，而是在量面包屑的 `boundingBox()` 时等到 30 s 超时（实施者的先红日志），Nit 的级别不变，修复让它直接、清楚地失败。
- **B**：新的发现：无。六条（M1、M2、N2、N3、N4、Q1）都照处置修好，三种引擎量过：行首图标的中线对名字第一行的中线 Chromium +0.5、Firefox −0.5 到 0、WebKit 0（折行的约 75 行 × 三种引擎），一行的名字也在 ±0.5 之内；小块一行 36 px、折行时字离边 6.8–9.5 px，"移除"在小块里竖向居中，悬停的底与焦点环不碰小块的边；"在哪里找"256 px（Firefox 的下拉不随选项变宽，照旧）；预览与变更集页的事实同是 14 / 4 / 12.5 px；`headOf` 去掉之后横幅一节的 35 个量值与第一轮逐项相同；警示对话框里的提示 1440 宽 398 × 106、375 宽 293 × 146，"Delete"的右缘在框里（改前 375 宽在屏外、点不到）。952 改前改后量了别处 4 种 `AlertDialog`、4 种 `Alert`，逐项相同。953：以 Chromium 的 `--blink-settings` 起"触屏笔记本"（`pointer: fine`、`hover: hover`、`any-pointer: coarse`），历史页 1440 与 1024 宽 16 个"只看此人"静止都是 1；三种引擎的触屏模拟里只留 953 一个类时改前 0、改后 1。同意 N1 撤回。第一轮的整套探针在 `df0437d2` 上重跑，没有一张横滚、没有元素越出视口，变了的量值都对得上修复或数据。
- **C**：代码的四条（I1、I2、N2、N3）修好，文档的七条与两个 Q 都在处置里（确切的文字在 C 的 `fix-check-1.md` 第 6 节，本文档提交照用）；变异样本 21 条（另有 4 次没有变异的基线）都红在报告写的那一格、那一行，数值相同，审查者另加的 cRnf 活着（F1-N2），m1s-asset、m1t-asset 红，m1s-file、m1t-file 绿（`File` 一支没有行，照实）；`df0437d2` 上合并之前的整条命令 260 秒，只败三格基线；五个提交说明的写法与作用域对。新的发现两条 Nit：

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P5S2C-F1-N1 | Nit | `trash-preview-head.test.tsx:106` 的注释引了 `trash-preview.test.tsx:554`，是设计基线的行号（今天那一格从 `:551` 起），整个 `web/apps/web/src` 的注释里唯一带行号的引用 | `4730ddd3`：改引格名"a preview opened from the trash's list is arrived at"；`2c0af01a` 把它折到 120 列以内 |
| P5S2C-F1-N2 | Nit | 954 之后变更集页两支"找不到"的空头一行没有格守，那一格的注释说的却是三支：cRnf（两支"找不到"传页的 26rem 一组）下 5 个文件 165 格全绿 | `4730ddd3`：写成三行的 `test.each`（读取中、404、地址不是 id），每一行断言空头一行只有面包屑一块；cRnf 红 |

两条只改测试与注释，由主持者读 diff、看变异核对（02-workflow 3.4）：

- `changeset-page.test.tsx:575` 是三行的 `test.each`（"on its way"、"not found"、"at an address of no changeset's id"），都是读者：读取中一行扣住变更集的读，404 一行的地址是没有的 id（`changesetId(9997)`，答 `page.changeset_not_found`），另一行的地址是 "rework"。三行在 `:605` 断言空头一行 `aria-hidden`、`headRowClass`、没有文字、只有 `cn(headCrumbsClass, "h-[1lh] text-compact")` 一块；读取中一行另放开读，看读到之后的头一行只有面包屑一块。注释写明每一支是各自的 `PageColumn`、各由这里守。
- 变异 cRnf 在 `4730ddd3` 上红在 404 与地址不是 id 两行（`:605`，`changeset-page.test.tsx` 与 `upgrade-banner.test.tsx` 2 败 / 75 过）；cR 照旧只红读取中一行（`:605`）。
- `trash-preview-head.test.tsx:106-110` 的注释引格名，不引行号。
- `2c0af01a`：`4730ddd3` 的那段注释里一行 150 列，重折；顺带把 `list-rows.test.tsx:11-14` 与 `trash-page.test.tsx:1101-1102` 两处 121 列的注释（S2 原来的提交里写的）折到 120 以内，只改注释。
- 两个提交各跑 `heavy.sh make lint-web knip test-web`，绿，233 个文件、5564 格（61.4 s、61.6 s）；只改 vitest 的格与注释，不跑 e2e。

修复核对到此：三位都没有 Important、Minor，不再开一轮。

## 主持者的决定（负责人可以改判）

全文在 [05-P5](../05-P5-pages.md) 2.3。

- **952 提示框与警示对话框里的长名字**（实施者的偏离 g，审查 B 的 P5S2B-Q1）：共用的 `Alert` 加 `break-words`；`AlertDialogContent` 照 `DialogContent` 加 `grid-cols-[minmax(0,1fr)]`；TB1 换成只有字母的名字、量提示（C-I1）；写进总体设计 13.2 第 31 条（C-Q2）。
- **953 可能用手指点的设备上，"只看此人"静止也显着**（审查 A 的 P5S2A-Q1）：`revealClass` 另加 `any-pointer-coarse:opacity-100`，同左栏树每行的"⋯"；左栏的齿轮（P2，`W/pages/notebook/page-tree.tsx:40`）只看 `hover:none`，记给收尾的审查（05-P5 第 9 节）。
- **954 变更集页的空头一行对谁都只有面包屑**（审查 A 的 P5S2A-N1、B 的 P5S2B-N4、C 的 P5S2C-N1）：去掉 `headOf`；写者那一格与 7.2 的 pc2、pc3 去掉；`PageColumn` 的 `head` 照旧。

修复之后主持者定的：

- P5S2B-N1 试了、撤回（处置允许）：加 `max-md:-ms-3.25` 之后不折时按钮压进链接 1 px，悬停的底与焦点环压在最后一个字上（`p5s2/fix1/shots-after2/chromium/only-375-Al-focus.png`）；窄屏折行时的 13 px 缩进照旧。B 修复核对时同意。
- P5S2B-M2：只加 `py-1.5` 时一行的小块是 44 px（处置没算进 30 px 的"移除"），"移除"另加 `-my-1.5`（同"只看此人"的 `-my-1`），一行仍 36 px，变异 m2b 守着：接受。
- A-N2 的旧写法在"Page not found"时并不照过，而是在量面包屑时等到 30 s 超时：新写法直接挡、报得清楚，照实记（A 修复核对时更正）。

处置表里另由主持者定的：C-Q1（04-P4 第 9 节"P5"补 S2 一行，用 C 的草稿）、C-Q2（952 写进 13.2 第 31 条）；A 的偏离 a 照实写进 4.2.8；M12-polish 一行与 01-P1 第 9 节一句照 C 的草稿（C 修复核对时照实核过，文字不变，删去"若 I1 不换名字"的备注）。修复核对第 1 轮 C 的两条 Nit：F1-N1 改引格名，F1-N2 写成三行的 `test.each`、加 cRnf。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份，跑完从备份复原，以 `git diff --quiet HEAD -- <文件>` 核对（02-workflow 3.1）；改坏期间不跑 `make gen*`、不用 `git checkout --`。实施者以 `p5s2/mut/mut.py` 跑（结果 `mut/results*.tsv`，日志 `mut/logs/`），修复轮以 `p5s2/fix1/mutfix.py`（`fix1/results.tsv`、`fix1/mutlogs/`）；故事的一条先 `make build`，再跑那一个故事（Chromium，除非写明）。审查 A 以 `p5s2-rev-a/mut.py`（`spec1.json`、`mutlogs/`；修复核对 `mut-f1.py`、`mutlogs-f1/`），审查 C 以 `p5s2-rev-c/mut/revmut.py`（第一轮 27 条）与 `mut2/revmut2.py`（修复核对 21 条）。

- **第一轮，`dbf2c2cf` 上**（实施者；之后的 `75578e08`、`c238d24d` 只改测试，代码相同）：7.2 的 S2 各条都被抓住（下表）；t1–t58 里十条活着，由 `75578e08` 补格，在新提交上重跑都红；40 重做成 40b，在 `75578e08` 上活着，`c238d24d` 补格之后红。
- **审查者，`c238d24d` 上**：A 的 r1–r13，r7 活着（M1）；C 重放实施者的 18 条（17 条加 42 的故事一次，pc1、pc4 的 vitest 与故事各一次）都与实施者同格，另写 9 条（33L、cR、cW-v、pvA、bw、L952 一族四条）。
- **修复之后，`df0437d2` 上**（实施者 20 条，下表的修复各行；C 重放其中的与自己的，共 21 条）：都红在说好的格，C 的 cRnf 活着。
- **修复核对第 1 轮之后，`4730ddd3` 上**（实施者）：cRnf 红；cR 重跑红。

7.2 的 S2 各条（实施者；"红在"的行号照 `c238d24d`，修复轮重跑的照 `df0437d2`）：

| # | 改坏 | 红在 |
|---|---|---|
| 30 | `revealClass` 去掉 `group-focus-within/row:opacity-100` | hi3:340（链接聚焦时得 0）；C 重放同 |
| 31 | 去掉 `forced-colors:opacity-100` | hi3:344 |
| 32 | 历史行的 `li` 去掉 `group/row` | `history-page.test.tsx` 只看此人一格；hi3:336（悬停时得 0） |
| 33 | 暗色 `--accent` `#d9a84e` → `#b88a3a` | `se1:305`，暗色 `box` 5.859 < 8.3（`#b88a3a` 对左栏的 `#141517`）；实施者初报"在哪里找"，C-M1 更正，C 重放同 |
| 34 | se1 暗色 `link` 的下限留 7.82 | `se1:322`，7.311 < 7.82；C 重放同 |
| 35 | 命中的标题换成 `div` | `se1:343`（设计写的 `:340`，注释多了三行） |
| 36 | 筛选条里"在哪里找"之前多一个按钮 | `se1:314`（Tab 之后焦点不在"在哪里找"） |
| 37 | `h1` 与事实之间插一行 | `changeset-page.test.tsx` 5 格（读 `h1.nextElementSibling` 的） |
| 38 | "撤销"留在事实之后 | `changeset-page.test.tsx` 头一行一格与当时写者的空头一行一格 |
| 39a、39b | 历史、变更集页的路由不加 `handle` | 表的历史 2 行、变更集页 5 行（第 2 句，`upgrade-banner.test.tsx:274`） |
| 40、40b | "恢复"排在链接之前 | 初写挪按钮时丢了 `ms-auto`，不算；40b 在 `dbf2c2cf`、`75578e08` 上活着（两次 Tab 等价），`c238d24d` 起红在 `list-rows.test.tsx:90`；C 重放同 |
| 41 | 预览的"恢复"挪出 `shown` | `trash-preview.test.tsx`、`trash-background.test.tsx` 7 格（预览被换掉时的焦点交接，含 `:372`） |
| 42 | 预览的名字不进 `span.wrap-anywhere` | 第一轮 TB1 绿（连字符后断，实施者与 C 都报），vitest 42v 红在 `trash-preview-head.test.tsx:43`；换名之后 TB1 红在 `tb1:428`，页宽 1180 |
| 43 | 面包屑挪到提示之后 | `trash-preview.test.tsx:928` |
| 43b | 预览的"See the jobs"不带 `arrived` | `trash-background.test.tsx` 预览那一格加的一句 |
| 44 | 目录的分组换成 `div` | lw13:176（`section` 的 `nth(0)` 找不到） |
| 45a–d | 回收站、预览、目录、标签的路由不加 `handle` | 表的 2、8、1、2 行（C 重放 45b 红在 `:274`） |
| 46 | `SettingsCard` 写回局部的串、`cardClass` 改 `rounded-md` | `settings-page.test.tsx` 的 `cardClass` 一格；`list-rows.test.tsx` 目录一行 |
| 63 | `revealClass` 去掉 `max-md:opacity-100` | mc3:202（`9c97fc79` 之后 `:204`），Chromium 与 `webkit-narrow` 都得 "0"；修复之后重跑同，C 重放同 |
| 70 | 历史行主链接不带 `arrived` | `history-page.test.tsx` 到达一格 |
| 71 | 回收站行链接不带 `arrived` | `trash-page.test.tsx` 新格（今天 `:1151`）与改写的 `:554` 那一格；修复之后红在 `trash-preview.test.tsx:560` 的 `waitFor` |
| 72–75 | "Reverts …"、"In the trash"、变更集页与回收站页的"See the jobs"不带 `arrived` | `changeset-page.test.tsx` 到达的对应两次；`revert-background.test.tsx`、`trash-background.test.tsx` 加的一句 |
| 76、77 | 回收站行文字一块不带 `listTextClass`；链接照旧 `underline` | `list-rows.test.tsx` 回收站一行（77 另有 `trash-page.test.tsx` 列表页一格） |
| 78 | 历史行第一行不带 `lineTitleClass` | `list-rows.test.tsx` 历史一行 |
| 79 | 带名字的"在后台恢复"不带 `shrink` | `tb1:442`，左边 −289.5；换名之后 −336.4 |
| 80 | 不带 `whitespace-normal` | `tb1:442`，页宽；换名之后 711 |
| 81 | "不在回收站里"另画 `ColumnTop` | 表的"不在回收站里""已过期"两行（第 1 句：两个横幅） |
| 82 | 变更集页地址不是 id 不包 `PageColumn` | 表的"地址不是 id"一行（`upgrade-banner.test.tsx:269`） |
| 90 | 预览按钮一组不带 `ms-auto` | `trash-preview-head.test.tsx` 长名字一格（设计说只由截图看） |
| 91 | "谁"不带 `max-w-64` | `history-page.test.tsx` 筛选条一格（同上） |
| 92 | 去掉 `[@media(hover:none)]:opacity-100` | `history-page.test.tsx` 只看此人一格的类名 |

另写的与修复的（行号照最后的树，写明"当时"的除外）：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| pc1 | 实施者；C 重放 | 页的两支不传 `head` | `page-layout.test.tsx:560`；UP1 `up1:273`，768 宽读取中 78 对读到之后 99.5，三种浏览器 |
| pc2、pc3 | 实施者 | 变更集页对能写的人传页的 26rem 一组；对读者也传 `headActionsClass` | 当时红在 `changeset-page.test.tsx` 写者、读者两格（只守类名，到不了 UP1，C-I3）；决定 954 之后去掉 |
| pc4 | 实施者；C 重放 | 预览的 `PageColumn` 传页的一组 | `trash-preview-head.test.tsx:88`（第一轮 `:87`）；UP1 `up1:309`（当时 `:306`），768 宽 99.5 对 78，三种浏览器 |
| pc5 | 实施者 | `EmptyHead` 的面包屑一块不带 `h-[1lh] text-compact` | vitest 4 格（`page-layout`、`changeset-page` 两格、`trash-preview-head`）；UP1 `up1:273`，768 宽 80 对 99.5，三种浏览器 |
| pc6 | 实施者；C 重放 | 预览没有面包屑时不画空头一行（4.2.8 的字面） | `trash-preview-head.test.tsx:101`（第一轮 `:100`） |
| t1–t58 的其余 | 实施者 | 设计的表没有的类名、结构与 `ColumnTop`（逐条在 `p5s2/report.txt` 第 3 节） | 都红 |
| t3、t7、t25、t36、t50–t53、t56、t57 | 实施者 | 只改样子的类名（"实施中的发现"） | 在 `dbf2c2cf` 上活着；`75578e08` 补格之后重跑都红（C 重放 t3、t50、t57：`trash-page.test.tsx:1121`、`changeset-page.test.tsx:270`） |
| r1–r6 | A | 各去掉一处早退的 `PageColumn`：预览的地址不是 id、别的笔记本的项、`page.not_found`、读取中与读不到；变更集的找不到、读取中与读不到 | 都红在"路由 × 支"表的对应行（r2 只红"别的笔记本的项"一行，没有被 `page.not_found` 截走） |
| r7 | A | 预览的 effect 恒取焦点（`(true \|\| merged \|\| focus)`） | 第一轮活（A-M1）：回收站四个测试文件全绿；A 的探针格（r7b）与原来那一格（配旧的链接）都红；修复之后只红 `trash-preview-head.test.tsx` 的新格 |
| f1-r7-after-title | A | 把文档的命名挪到取焦点的 effect 之前，同时恒取焦点 | 新格照样红：信号不靠声明的次序 |
| r8–r13 | A | 各去掉一处 `arrived`：历史的 Undone、Undid，变更集页的 See the undo、Undone by 之后的 ID、History of this node、内容的版本 | 都红在对应的到达一格 |
| cR | C；实施者 | 变更集页读取中的 `PageColumn` 传页的 26rem 一组 | 第一轮（`headOf` 恒返回页的组）：UP1 三种浏览器红在 `up1:295`（768 宽读到之后扣掉 22.5 是 78，读取中 99.5）；954 之后：`changeset-page.test.tsx:595`（今天 `:605`，读取中一行）与 UP1 `up1:298`，三种浏览器 |
| cRnf | C；实施者 | 两支"找不到"传页的 26rem 一组，读取中一支不动 | `df0437d2` 上 5 个文件 165 格全绿（F1-N2）；`4730ddd3` 上红在 `changeset-page.test.tsx:605` 的 404 与地址不是 id 两行（2 败 / 75 过） |
| cW-v | C | 能写的人也不传 `head`（当时的 `headOf`） | 只红在 `changeset-page.test.tsx:599`（写者一格）：那一格只守类名（C-N1）；954 之后这一支不在了 |
| pvA | C；实施者 | 预览的 effect 去掉 `(merged \|\| focus) &&` | 第一轮整套 vitest 233 / 5559 全绿（C-I2）；`48c0eaa1` 起只红新格（今天 `trash-preview-head.test.tsx:119`） |
| bw | C；实施者 | 带名字的"在后台恢复"去掉 `wrap-anywhere` | 第一轮 TB1 照绿（连字符的名字，C-I1）；换名之后 `tb1:442`，页宽 545 |
| 33L | C | 同 33 调淡琥珀，另放开暗色的 `box`、`button`、`select` 三个下限 | `se1:322`，暗色 `link` 5.096 < 7.21：新下限守得住琥珀 |
| L952 | C | TB1 换成只有字母的名字，`Alert` 加 `break-words`（即 952，不是变异） | TB1 绿 |
| L952-42 | C | L952 上再做 42 | `tb1:428`，页宽 1180 |
| L952-bw | C | L952 上去掉"在后台恢复"的 `wrap-anywhere` | `tb1:442`，页宽 545 |
| L952-noalert | C | 只换名字、不改 `Alert` | `tb1:442`，页宽 552：952 本身也由 TB1 守 |
| 952a | 实施者；C 重放 | `Alert` 去掉 `break-words` | `alert.test.tsx:22`；TB1 `tb1:442`，页宽 552 |
| 952b | 实施者；C 重放 | `AlertDialogContent` 去掉 `grid-cols-[minmax(0,1fr)]` | `dialog.test.tsx:77`（alertdialog 一行） |
| 953 | 实施者 | `revealClass` 去掉 `any-pointer-coarse:opacity-100` | `history-page.test.tsx:1122`（类名；e2e 的三个项目都不是粗指针） |
| m1t、m1s | 实施者 | 回收站行（`ItemIcon` 与给 `AssetIcon` 的）、搜索命中的图标回到居中 | `trash-page.test.tsx:1122`；`search-page.test.tsx:937` |
| m1t-asset、m1s-asset、m1t-file、m1s-file | C | 只去掉附件一支、只去掉 `File` 一支的 `mt-0.5 self-start` | 附件一支两边都红（`:1122`、`:937`）；`File` 一支（附件没有 `asset` 时的回退）两边都绿：没有格 |
| m2、m2b | 实施者；C 重放 m2b | `filterChipClass` 去掉 `py-1.5`；"移除"去掉 `-my-1.5` | `history-page.test.tsx:1160`；`:1161` |
| n2s、n2i | 实施者 | "在哪里找"、目录的"类型"回到 `max-w-full` | `search-page.test.tsx:928`；`index-page.test.tsx:612` |
| n3 | 实施者；C 重放 | 预览的事实回到 `mt-2.5 space-y-0.5` | `trash-preview-head.test.tsx:54` |

没有格守的：`File` 一支的行首图标（m1t-file、m1s-file，照实）。没有等价的变异留下（40 的原写法不算，40b 取代它）。

## 门禁与 CI

| 提交 | `heavy.sh make lint-web knip test-web` | `CI=1 heavy.sh make e2e`（过 / 败 / 跳） |
|---|---|---|
| `1a7a735e` | 绿，232 个文件、5533 格（60.5 s） | 643 / 2 / 1（pg15 过了），2.9 min |
| `dbf2c2cf` | 第一次 oxlint 两条 warning；绿，233 / 5559（60.8 s） | 644 / 2 / 1（pg15 过了），2.9 min |
| `75578e08` | 绿，233 / 5559（62.7 s） | 不跑（只改测试） |
| `c238d24d` | 绿，233 / 5559（62.0 s） | 由合并之前的整条命令覆盖 |
| `74b22728` | 第一次 oxlint 两条 `jsx-key`；绿，233 / 5562（60.8 s） | 643 / 3 / 1，2.7 min；TB1 过 |
| `48c0eaa1` | 绿，233 / 5563（60.3 s） | 不跑（只改 vitest 的格） |
| `446d2463` | 绿，233 / 5562（60.8 s） | 643 / 3 / 1，2.8 min；UP1 (page) 三种浏览器都过 |
| `9c97fc79` | 绿，233 / 5562（61.5 s） | 643 / 3 / 1，2.7 min；hi3、mc3（Chromium 与 `webkit-narrow`）都过 |
| `df0437d2` | 绿，233 / 5562（60.7 s） | 643 / 3 / 1，2.7 min |
| `4730ddd3` | 绿，233 / 5564（61.4 s） | 不跑（只改 vitest 的格与注释） |
| `2c0af01a` | 绿，233 / 5564（61.6 s） | 不跑（只改注释） |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `dbf2c2cf`（实施者） | 以 2 退出，只因基线三格；vitest 233 / 5559（63.8 s）；`gen-check-web` 过；E2E 643 / 3 / 1，2.9 min；整条 249 秒 |
| `c238d24d`（实施者；C） | 实施者：vitest 233 / 5559（63.75 s）；`gen-check-web`、`build-web` 过；E2E 643 / 3 / 1，2.8 min；整条 242 秒。C（2026-11-25 21:40–21:44 JST）：vitest 5559（61.6 s）；E2E 643 / 3 / 1，2.8 min；整条 250 秒；TB1 新格 0.53 s，UP1 (page) 3.2 / 5.3 / 5.3 s，hi3 4.5 s，mc3 0.98 s、`webkit-narrow` 1.9 s，se1 (page) 1.5 s |
| `df0437d2`（实施者；C） | 实施者：vitest 233 / 5562（58.8 s）；`gen-check-web`、`build-web` 过；E2E 643 / 3 / 1，2.7 min；整条 231 秒；TB1 0.43 s，UP1 (page) 3.0 / 4.5 / 4.5 s，hi3 4.5 s，mc3 0.97 s、`webkit-narrow` 2.1 s，se1 (page) 1.4 s。C（22:52–22:56 JST）：vitest 5562（64.0 s）；E2E 643 / 3 / 1，2.9 min；整条 260 秒 |

- 每次都只败基线里同行同消息的格；每次"not configured to support act"0 次，"Not implemented: navigation to another Document"照旧一次；`check.mjs` 提交 1 是 433 / 105 / 105 / 3，提交 2 起 434 / 105 / 105 / 3。
- 受保护路径的 `git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`git diff --check m13-ui-refresh...HEAD` 0；`m13-ui-refresh` 一直是 `af33b81e`，分支可快进；跑完 `git status` 为空。
- 合并之前的整条命令最后一次在 `df0437d2`；之后的 `4730ddd3`、`2c0af01a` 只改 vitest 的格与注释，各跑 `make lint-web knip test-web`。
- **先写、先红**：见 05-P5 10.2"测试"。日志在会话的临时目录：`p5s2/red/c1-vitest.log`、`c1-e2e.log`、`c2-vitest.log`、`c2-vitest-head.log`、`c2-e2e.log`；修复轮 `p5s2/fix1/red-a-*`、`red-b-*`、`red-c-*`、`red-d.log`、`red-e.log`。
- vitest：5504（231）→ 5533（232）→ 5559（233）→ 5559 → 5559 → 5562 → 5563 → 5562 → 5562 → 5562 → 5564 → 5564。E2E 646 → 646 → 647；`check.mjs` 433、105、105、3 → 434、105、105、3，修复之后不变。
- **CI**：负责人推送 `af33b81e`（P5/S1 合并之后）之后分支的 CI（run 45631867718）全绿，E2E 一步 8 分 5 秒；release 的演练（run 45631867695）也绿：S1 记下的只靠 CI 的几样（UI1 两格与 UP1 (page) 的新步在 Linux 的字体与滚动条下）成立。S2 的头还没推送（决定 746：没有推送的凭据时以本机门禁替代分支 CI，负责人推送之后 CI 在最后一个头上补跑）。
- **只靠 CI 的**（S2）：TB1 新的一格（只有字母的名字靠 `wrap-anywhere` 与 `break-words` 断行，与字体无关，推断照样成立）与 UP1 (page) 变更集一步的面包屑在 Linux 的字体下折几行（公式只宽免折出的高，x 与宽照样严格相等）；hi3 静止的那一步在 Linux 的无头 Chromium 上：决定 953 之后它另依赖 `any-pointer` 不是 coarse（审查 A 修复核对第 1 轮）。
- **对比度**（05-P5 4.4；se1 自己的量法，Chromium、1440 宽，临时加日志跑一次，文件从备份还原）：亮色 `marked`、`named` 14.642 / 7，`box` 2.843 / 2.74，`button`、`select`、`link` 3.098 / 2.99，`invalid` 4.018 / 3.91；暗色 `marked`、`named` 7.418 / 7（卡片上，推算 7.28），`box` 8.406 / 8.3，`button` 6.954 / 6.85，`select` 7.923 / 7.82（页的 `#1a1b1e` 上），`link` 7.311 / 7.21（卡片 `#212226` 上，outline solid 2px；推算约 7.31，下限照"实量减 0.1"），`invalid` 7.283 / 7.18。变的只有暗色 `link`（7.82 → 7.21，卡片的面比页深）与暗色的高亮字（在卡片上）。审查 C 以 WCAG 公式复算 `#d9a84e` 对 `#212226` 正是 7.311。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库；量出的值在 05-P5 10.2"视觉的核对"。

- **实施者**：`p5s2/shots/{chromium,firefox,webkit}/`，每种约 300 张与 `measures.json`、`measures-extras.json`（21 个视图 × 英文亮暗 1440、1024、768、375、320 + 中文亮色 1440、375、320；只看此人的静止、悬停、键盘聚焦在链接与在按钮；搜索与历史一行的第一行的高；命中的焦点环；Chromium 的强制色；422 之后带长名字的"在后台恢复"；读者的变更集页与回收站；升级中三种页在五种宽度读取中与读到之后的横幅），样稿的截图 `p5s2/shots/mockup-*.png`，局部 `p5s2/crops/`；探针 `p5s2/probe/ui1-zz-p5s2-visual.spec.ts`。修复之后 `p5s2/fix1/shots-final/`（最后的代码树 `df0437d2`：952 的提示与警示对话框、行首图标、小块、下拉的宽、事实的间距，1440、768、375、320）与 `shots-after2/`（带 B-N1 时，撤回的证据），探针 `p5s2/fix1/probe/ui1-zz-p5s2-fix1.spec.ts`。修复之后的实量（三种引擎）：提示框 1440 / 768 / 375 / 320 宽是 720 × 86 / 468 × 106 / 327 × 146 / 272 × 166，`scrollWidth − clientWidth` 0（改前 768、375、320 宽页宽 791、539、539）；警示对话框里的提示 398（1440、768）、293（375）、238（320）宽，不溢出，两个按钮在框里；行首图标的中线对名字第一行的中线 Chromium +0.5、Firefox −0.5 到 0、WebKit 0（改前两行低 9.5–10.5，三行低 19.5–20.5）；小块一行 36 px，两、三、四行 53、72.5、92 px，字离上下边 6.8–10.8 px；"在哪里找"与"类型"Chromium、WebKit 256；预览与变更集页的事实离标题 14、行间 4、12.5 px。
- **审查 A**：`p5s2-rev-a/zz-rev-a-probe.test.tsx`（vitest 探针：按地址打开的预览、五支早退各一个 `h1` 与空头一行、StrictMode 下的到达），`zz-rev-a-probe.spec.ts` 与 `e2e-out/`（写者与读者的空头一行逐 px 相同、952 在 375 宽的实量、"只看此人"的键盘与平板）；修复核对 `zz-rev-a-f1.spec.ts` 与 `e2e-f1/`（320 宽的"撤销"对话框、953 的两种设备）。
- **审查 B**：第一轮探针 `p5s2-rev-b/probe/ui1-zz-p5s2-rev-b.spec.ts`，`shots-orig/`（视图、只看此人、强制色、头一行）、`shots-rest/`（横幅）、`shots-touch/`、`shots-alert-{orig,fix}/`、`shots-dialog-{orig,fix,grid}/`、`crops/`，试过的补丁 `q1-alert-and-dialog.patch`；修复核对 `probe/ui1-zz-p5s2-rev-b-fix.spec.ts`，`f1-fix/`、`f0-sample/`、`f1-sample2/`、`f1-all/`、`f1-coarse/`，分析 `an-fix.py`、`analyze.py`。对照样稿：列表类的页的标题 26 px / 650 / −0.01em，列 720 宽居中，卡片 14 px 圆角，行 18 × 14 px 的内边距，第一行 14 px / 500，分组标题 13 px / 600 灰，与 settings.html 的 `.card`、`.set`、`.sec` 一致；事实一行 13 px 灰，比样稿的 `.set .d`（12.5 px、text-3）深一级，是设计定的；变更集页与预览的头一行与页的同位置（1440 宽 y 12、高 36，面包屑 13 px），`h1` 32 px / 650 在 (486, 78)，变更集的事实 12.5 px `text-faint`、离标题 14 px，同 reading.html 的 `.meta` 与 `.doc h1`。
- **审查 C**：`p5s2-rev-c/probe/`（`zz-preview-nofocus.test.tsx`，没改的树上绿、pvA 下红），`mut/`、`mut2/`（重放与自己的变异），`premerge.log`、`premerge-f1.log`（两次合并之前的整条命令），`replay.py` 与 `f1docs/`（8.2 第 6–10 条在总体设计副本上的重放，7 步）。

## 接受与推后

- **B-N1 撤回**：窄屏上折到下一行的"只看此人"，字比行里别的字缩进 13 px，照旧：`max-md:-ms-3.25` 在不折时压进链接 1 px、焦点环压在最后一个字上，两害取轻（B 同意）。
- **决定 927 的 768 宽空带**：768 到约 1000 宽、历史行第一行放不下"Edit · 名字"加按钮时（名字约 35 个拉丁字符以上），透明的"只看此人"占着下一行，留出约 22 px（带高 26 px）的空；悬停、聚焦时按钮就出现在那里，是"透明不是隐藏"的取舍（B 核对），不改。
- **左栏的齿轮**（P2，`W/pages/notebook/page-tree.tsx:40`）只看 `hover:none`，触屏笔记本上静止时看不见：要不要同 953 加 `any-pointer-coarse`，记给收尾的审查（05-P5 第 9 节）。
- **953 在真机上**：Chromium 的 CDP（`Emulation.setEmulatedMedia`）不认 `any-pointer`，模拟不了"能悬停、又有粗指针"的设备（A 试过）；B 以 `--blink-settings` 起的浏览器量过，真的触屏笔记本、接了触控板的 iPad 没有看过，留给收尾。
- **长面包屑**：变更集页的面包屑折行时，读取中与读到之后横幅差折出的那一截（决定 946 的照实；UP1 扣掉它比）。
- **`File` 一支的行首图标**没有格（C 修复核对：m1s-file、m1t-file）。
- **M5-polish 38.13**（"下拉框把长路径截在约 39 个字符"）：M12-polish 的 S2 一行没有写，看不出是哪一个下拉（C 的草稿照此），照决定 937 留给 v0.2。
- **B 未核实的**与别的未核实的，照实列在下一节。

## 未核实的怀疑

- **953 单独的效果**（A、B）：`hover: hover` 加 `any-pointer: coarse` 的设备，A 用 CDP 模拟不出来，B 以 `--blink-settings` 量过；真机没有看。
- **hi3 在 Linux 上**（A 修复核对）：hi3:334（静止为 0）在 Linux 的无头 Chromium 上另依赖 `any-pointer` 不是 coarse，等负责人推送之后的 CI。
- **"只看此人"的键盘显出**（A、B、实施者）：只在 Chromium（与 B 的 Firefox）量过；WebKit 的 Tab 照旧跳过链接，实施者的 WebKit 聚焦截图以 `focus()` 进去。
- **预览正文里的 `# 标题`**（A）：渲染成第二个 `h1`（`<h1 id="nw-gone">`，在 `article` 里），S2 之前就这样，S2 没有改正文的渲染；UP1 的 `bannerPlaces` 用 `.first()` 取页的标题。
- **暗色 se1 那一页的焦点环在卡片上**（B）：B 没有自己截，看了实施者的 `p5s2/shots/chromium/search-focus-dark.png`；比值是 se1 自己量的。
- **读者的变更集页**（B）：B 的数据里没有真的读者，看了实施者的 `p5s2/shots/webkit/changeset-reader-light-375-en.png`。
- **`AlertDialogDescription` 带没有断点的名字**（B）：会同样撑宽对话框（952 只管 `Alert` 与一列的网格）；没有单独造。
- **目录的"类型"封顶**（B 修复核对）：B 的数据里没有长类型，用的是代码与实施者的截图（长类型时 256，"标签"留在同一行）。
- **另三个警示对话框**（B 修复核对）：`unsaved-guard`、`unresolved-link`、`version-restore` 没有截；`grid-cols-[minmax(0,1fr)]` 只在内容的最小宽大于对话框时起作用（A 读过五处用处的代码）。
- **Linux 的字体**（A、C）：TB1 换名之后照绿、42 照红，UP1 变更集一步的 `max(0, trail − 36)`，都只在 macOS 上跑过，等负责人推送之后的 CI。
- **`bannerPlaces` 的 `page.unroute(held)`**（C）：05-lessons 第 6 节说去掉最后一个路由会关掉拦截、在途的请求可能挂住；S1 留下的写法，S2 用到三种地址，C 跑了 UP1 12 次（各三种浏览器）没有遇到。
- **M5-polish 38.13 落在哪一页**（C）：见"接受与推后"。
