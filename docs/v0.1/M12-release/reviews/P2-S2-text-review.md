# M12/P2 S2（文字）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p2-s2`。实施的 10 个提交，从 `main` 的 `f08dca69`（P4/S1 的合并）开始，审查时在 `4c9f1212`：<br>英文的单复数，10 个 `*One` 键 `8c144f7d`；根下一律"笔记本的顶层"，根下的同名页 `page.nameAtTop` `32ce4d15`；英文的撤销一律 undo，被清理说"部分或全部"，别处已撤销之后再撤销的冲突说明 `becb67e1`；回收站、预览与历史一件事说一遍、照实说，H4 的两句 `7e90e1b2`；勾了会让任务项消失、附件写法的未建链接、附件的长名、单页的导出、失锁的编辑器 `a5936faf`；比较只差末尾换行、连成一块的并排 `ff87ae77`；搜索的提示、"继续查找"、索引在建照最新一页 `f00970d4`；KiB 与 MiB、来源规则的说明 `ebc1512e`；两份 `AGENTS` 模板 `98e0387d`；变异里活下来的五句中文补格 `4c9f1212`。<br>修复第 1 轮 6 个提交，头 `62e40b25`：替身的任务项照服务端的解析 `a1138dd7`；树没读到时导出仍说子页的一格 `e3671b19`；23.11 先等这次改动的重读 `6b58eea4`；三句改得直白 `b6215ab4`；`page.nameAtTop` 不带笔记本名 `1580e7ca`；Go 测试改名 `62e40b25`。<br>合并 `main` 一次：`260d299d`（P4/S1 与提交期限的旁支的记录、导入守卫的旁支，没有冲突）。<br>分支共 16 个提交（不计合并），相对 `main` 80 个文件，加 1,457 行、减 286 行：网页 74 个文件（`web/apps/web/src/` 下的 `pages/`、`editor/`、`i18n/`、`reading/`、`app/`，两份翻译文件，替身 `test/page-server.ts` 与它的新自测）；三个 e2e 故事（C4、TB1、LW6，只改断言）；`wiki/domain` 的两份 `AGENTS` 模板与它们的测试。契约、迁移与组合根不动，`docs/` 在本提交。i18n：新加 23 个键、删 2 个，改文字的 en 22 个、zh-CN 27 个 |
| 规格 | [P2 设计](../02-P2-web-polish.md) 4.2（4.2.0–4.2.6）、3.1、5.2 的 S2、6.2、7.1–7.4、8.2 的 S2；2.3 的决定 385–391、425–428（P2-8 至 P2-14、P2-48 至 P2-51）；[00 号文档](../00-M12-design.md) 4.5，第 7 节"总体设计的修订"第 20 条；移交 [M5-polish](../handoffs/M5-polish.md) 的 S2 各项与 [M8-history](../handoffs/M8-history.md) 第 4 条 |
| 审查方式 | 三位审查者并行，各在自己分离的工作树里（A `nerve-wiki-m12-p5-rev-a`、B `nerve-wiki-m12-p4s5-rev-b`、C `nerve-wiki-m12-p4s5-rev-c`，都在 `4c9f1212`），重的经 `heavy.sh`，不碰实施者的工作树；`TMPDIR` 在 `~/.cache` 下各自的目录，用完删了：<br>A 文字本身（提交 1–4、10 与 9 的模板）：从两棵树的翻译文件与设计的表里各抽出键逐个对照（`cmp.py`）；逐对读 23 个新键与 31 个改键的两种语言；单复数的十处；"顶层"、撤销的用词、回收站与历史；`AGENTS` 模板对照 `wiki/adapter/mcp/prompts.go` 登记的 prompt；按旧文字的片段与旧词 grep 全部测试与故事；23.11 的探针两格；头上四个文件 121 格。<br>B 行为、替身、比较、搜索与单位（提交 5–8）：服务端任务解析的探针（组合根的 `tasks`、`obsidian` 两个扩展，17 种写法，每种报解析出的任务项与 `ToggleTask` 在 offset 3 上会答什么），对照替身 `tasksOf` 拷到 node 里的同一组写法；19.1 的一条变异；9 个 vitest 文件 211 格；e2e 的断言有没有变松。<br>C 提交、门禁、变异与文档（全部）：挑五个提交（2、4、5、8、9）单独跑 `make lint-web`、`make knip`、`make test-web`（9 号是 `wiki` 模块的 go test 与 `make lint-go`）；逐个提交比较两种语言的键（`keys.py`）；六组的代码文件换回 `f08dca69` 的先红；自加四条变异；`TestLintBusyReturnsBeforeCleanup` 的探针；用 `gh` 只读取 e2e 的逐格用时；文档草稿对照 8.2、13.2 与 00 号文档 |
| 日期 | 2026-11-18（实施、审查、修复第 1 轮、核对第 1 轮、合并 `main`） |
| 结论 | 没有 Important。Minor 2：替身的 `tasksOf` 对"`]` 后紧跟冒号"的行与服务端在一半写法上不一致，说明把推断写成了通则（P2S2B-M1）；19.1"树没读到时照旧说及其子页面"没有测试守（P2S2C-M1 = P2S2B-N1）。Nit 13（A 5、B 2、C 6），Q 3（A 2、C 1）。三位都确认：文字逐字照设计（A 从两棵树抽出逐键比）；挑的提交单独为绿，格数与实施报告相同；翻译文件的三条规则守住；先红后绿与报告相同；实施者说"红的格与设计预期不同"的三条站得住；e2e 的断言没有变松。修复第 1 轮 6 个提交；核对第 1 轮三位的旧发现都关上，新的 0 Important、0 Minor、4 条 Nit（A 1、B 1、C 2，A 的一条与 C 的一条是同一件事），都在本提交写对，不开第二轮 |

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `4c9f1212` 的。修复照主持者的说明（`p2s2/fix-1.md`）在分支上加提交，普通推送、不改写已推的历史，每个单独为绿；文档的部分在实施者的草稿里改，落在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2S2A-N1 | 细节（文字） | `diff.lastBreakRemoved` 的英文 "The only change is the line break at the end, removed." 像漏了词（设计 4.2.1.1 原文） | `b6215ab4`：两句同形，"The only change is that a line break was added at the end."、"…that the line break at the end was removed."；中文不变，`text-diff.test.tsx` 的英文格随之改。核对 A：关上 |
| P2S2A-N2 | 细节（文字） | `transfer.restoreLeftUndone` 两种语言都绕："被撤销"与"由那次撤销删除"说的是同一件事，"那次撤销"没有所指；中文"各自成项"与同组的"分成几项"不同 | `b6215ab4`：照 A 的建议改两种语言（"已恢复的部分被一次撤销删除，又回到了回收站；没处理到的节点也在回收站里，分成几项。"），`transfer-background.test.tsx` 的两格随之改。核对 A：关上 |
| P2S2A-N3 | 细节（文字） | 中文还有一处"变更集"：`problem.page.virtual_file` 的"日志就是变更集"，英文是 "the notebook's changes"；决定 425 要的是同一个用词 | `b6215ab4`：改成"日志就是这本笔记本的改动"，网页的中文里不再有"变更集"；`problem-messages.test.ts` 加两种语言各一格（以前没有格读它）。核对 A：关上 |
| P2S2A-N4 | 细节（行为） | 23.11 的冲突说明不等这次改动的重读（`reread(true)` 是不等的 `mutate`）：重读回来之前、重读失败时一直说 "rename it back"（A 的探针） | `6b58eea4`：`refused` 里先 `await` 这次改动的重读，再 `reread(false)` 重读两份列表、不等，然后 `setConflict`；重读失败时照旧给旧的建议。新格扣住重读，断言扣着时没有 "rename it back"，放开之后是 `history.revertAgain`。核对 A、B：关上 |
| P2S2A-N5 | 细节（测试名） | `TestTheAGENTSTemplatesNameThePromptsAsServed` 只比对模板里写死的一行，不读服务端实际发的 prompt | `62e40b25`：改名为 `TestTheAGENTSTemplatesNameTheUpgradePrompt`，注释写明只查那一行；与登记的 prompt 对照进 v0.2（决定 587）。核对 A、C：关上 |
| P2S2A-Q1 | 问 | `page.nameAtTop` 写 "Notes (at the top level of Plans)"，别处都说 "the notebook's top level"；9.5 的情形里它可以读成 Plans 那一页的顶层 | 决定 584（`1580e7ca`）：不带笔记本名；`distinctName` 去掉 `notebook` 参数，十六处调用与六个测试文件 14 处断言随之改。核对 A：关上 |
| P2S2A-Q2 | 问 | 撤销被拒为 trimmed 时，alert 与页上的 `history.trimmedNote` 两句近乎重复（S2 之前也近似） | 进 v0.2（决定 587）。核对 A：关上 |
| P2S2B-M1 | 中（替身） | 替身的 `tasksOf` 对"`]` 后紧跟冒号"的行与服务端在一半写法上不一致：服务端只在冒号之后是单个目的地、其后只有空白时才成定义；`- [ ]: a b c`、`- [ ]:` 等勾上照写，替身答 422；`- [x]: a b c` 等是任务项，替身不给复选框。说明把只对定义成立的推断写成了通则 | 决定 585（`a1138dd7`）：替身先认定义形（`definitionShape`），没勾的是任务项且 `goes`，勾着的不是任务项，其余照普通任务项；说明照实写，写明不模拟带标题的定义与 `- [ ]x`；新的自测 `test/page-server-tasks.test.ts` 十五行，与 B 在服务端上探出的表逐行相同。核对 B、C：关上 |
| P2S2B-N1 | 细节（测试） | 19.1"树没读到时照旧说'及其子页面'"没有格钉住：把读不到当作没有子页，46 格全过 | 同 C-M1（`e3671b19`）。B 原来写的情形不成立，见"更正"。核对 B：关上 |
| P2S2B-N2 | 细节（报告） | 26.14 的中文一格是"30 条。"，设计与实施报告写"约 30 条。"：读到底，计数是找到的 30，测试对、设计错 | 写成与设计不同之处（P2 设计 10.2，26.14 注"实际见 10.2"）。核对 B：关上 |
| P2S2C-M1 | 中（测试） | 同 B-N1：变异 `m-export-unread`（树没读到也当作没有子页）在 `page-menu.test.tsx` 的 9 格下照过；设计列的两条变异碰不到这一支 | `e3671b19`：页的菜单要等树读到才出现，新格经探针路由（`ExportProbe`）在 `nodesDown` 下打开 Guide 的导出，标题是"…and its subpages?"，读失败之后仍是；把读不到当作没有子页的变异红在这一格。代码不改。核对 B、C：关上 |
| P2S2C-N1 | 细节（报告） | 4 号提交 amend 了两次，报告与草稿写的是一次 | 照实写（"更正"、P2 设计 10.2）。核对 C：关上 |
| P2S2C-N2 | 细节（报告） | 第 9 节读屏清单那一条引的按钮名不对：`history.removeFilterLabel` 是 "Remove the filter “{filter}”"，不是 "Remove filter: …" | 照实写（"读屏与人工清单"），另列树没读到时的 "Remove the filter “…”"。核对 C：关上 |
| P2S2C-N3 | 细节（文档稿） | 第 15 节那一行的依据少了 8.2 | 写"4.2 的 9.5、38.6、8.2、10.2"。核对 C：关上 |
| P2S2C-N4 | 细节（报告） | 第 1 组 `index-page` 的先红没有日志 | 照实写：证据是四条"恒假"变异（"反向对照"）。核对 C：关上 |
| P2S2C-N5 | 细节（文档稿） | 23.1 的 "…reverted whole" 在移交那一行没点名，写成了只进 v0.2 | `M5-polish.md` 那一行点名 `page/domain/errors.go:89`，P3 或 v0.2。核对 C：关上 |
| P2S2C-N6 | 细节（替身） | 替身认 `- [x]: text` 不是任务项的那一支没有格，前提也没核：去掉它，166 格全过 | 随 B-M1（`a1138dd7`）：`- [x]: /u`、`- [X]: /u`、`- [x]:/u`、`- [x]: <>` 四行在自测里，勾着的定义形当任务项的变异红在这四行。核对 C：关上 |
| P2S2C-Q1 | 问 | `TestLintBusyReturnsBeforeCleanup` 那一次红是 M10 测试自己的先后假设，探针确定地复现，不是随机的偶发 | 决定 586：不是 S2 的，不记成已知偶发，另开旁支 `m12-lintbusy-await-expiry` 修。核对 C：关上 |
| P2S2C（可选） | 细节（文档） | 总体设计 3.6 写"单篇正文不超过 5 MB"，界面（38.6 之后）、README 与测试都是 MiB | 本提交改成 5 MiB（决定 588） |
| P2S2A-C1-N1 = P2S2C-C1-N2 | 细节（文档稿，核对第 1 轮） | P2 设计正文里 `nameAtTop` 的旧写法、Go 测试的旧名还在（2.3 的 428、3.1、4.2.1.1、4.2.5、9.5、8.2、第 40 条），草稿没有列 | 逐处注"实际见 10.2"，10.2 列全（决定 588） |
| P2S2B-C1-N1 | 细节（替身，核对第 1 轮） | 替身把 `- [ ]: a)b`（目的地里有落单的 `)`）当成定义，答 422；服务端照写 | 接受，代码不改（决定 585）：只是替身，今天没有测试用到 |
| P2S2C-C1-N1 | 细节（文档稿，核对第 1 轮） | 草稿里 zh-CN 改文字的键按最终状态是 27 个，不是 26 | 照 27 写，多的是 `problem.page.virtual_file`（本记录的表头、P2 设计 10.2） |

## 主持者的决定

负责人可以改判；写进 [00 号文档](../00-M12-design.md)第 11 节"2026-11-18：P2/S2 的决定 584–588"：

- **584 `page.nameAtTop` 不带笔记本名："{name} (at the notebook's top level)"、"{name}（笔记本的顶层）"**（审查 A 的 Q1；P2 设计 9.5、8.2 的 S2）：
  - 理由：与回收站、转移、索引、历史、移动对话框是同一个说法（决定 386）；同一本笔记本里笔记本名不区分什么。带上它，9.5 的情形（笔记本 Plans 的根下有一页也叫 Plans）里 "Notes (at the top level of Plans)" 可以读成 Plans 那一页的顶层，与它旁边的 "Notes (in Plans)" 只差"in"与"top level of"。根下的两页不会同名，不带名字也唯一。
  - 取代决定 428 的前一半（名字里带笔记本名，"(at the top level of Plans)"）；后一半（不用笔记本名的所有格）随之没有对象：新写法不写哪个名字的所有格。
  - `distinctName` 去掉不再用的 `notebook` 参数，十六处调用跟着改（`1580e7ca`）；总体设计 13.2 第 17 条的示例写"Install（笔记本的顶层）"。
- **585 替身的任务项照服务端的解析**（审查 B 的 M1、C 的 N6；P2 设计 6.1）：
  - 只有冒号之后是单个目的地（`<>` 也算）、其后只有空白的行是定义形：没勾的是任务项，勾上会成链接引用定义，答 422 `not_allowed`；勾着的本来就是定义，不是任务项。别的 `]:` 行照普通任务项，勾选照写（`a1138dd7`）。
  - 实施者"`- [x]: /u` 不是任务项"的推断由审查 B 在服务端的解析上核实，只在冒号后是单个目的地时成立；设计"任何 `- [ ]:` 勾上都会消失"、实施"任何 `- [x]:` 都不是任务项"都不成立。
  - 带标题的定义、`- [ ]x` 不模拟，替身的说明写明。
  - `- [ ]: a)b`（目的地里有落单的 `)`）替身答 422、服务端照写，接受，代码不改（核对第 1 轮 B 的 C1-N1）：只是替身，今天没有测试用到。
- **586 实施门禁里 `TestLintBusyReturnsBeforeCleanup` 那一次红不是 S2 的，也不记成已知偶发；另开旁支 `m12-lintbusy-await-expiry`（M10 的故事）修，合并见 00 号文档第 12 节**（审查 C 的 Q1）：
  - M10 的交错测试（`bootstrap/interleavings_lint_test.go:416-427`）等到 alice 的 `server_busy` 与 `pg_sleep` 被取消，就断言 bob 新建一次运行；这一次运行的过期由 `context.AfterFunc` 在另一个 goroutine 里做（`wiki/app/lint_runs.go:98`、`:186-190`），两件事之间没有先后。
  - 审查 C 的探针在 `expire` 开头睡 300 ms，确定地红（"new same-key caller joined an expired worker: {Kind:2 FlightID:1 …}"，与实施时那一次一字不差）；测试里再等 `RunExpired` 就过。
  - S2 在服务端只改两份模板与一个测试，不碰 lint；同一个头上单跑十次与整份重跑都过。
  - 对产品是窗口在亚毫秒级的一次多余的 503，与 `Runs` 的注释"A flight's expiry removes only its permission to be joined"相符，是测试断言得比代码保证的严，不是竞态。（后注：亚毫秒是平常的情形，`expire` 平常在 0.1 ms 之内；测试从取消到新的调用者被收下要 1.3–5 ms，照推断，那一次在负载下到了毫秒级。见[旁支的审查记录](M12-lintbusy-await-expiry-review.md)"更正"与 00 号文档决定 595。）
- **587 推后进 v0.2 的**（审查 A 的 Q2、N5；P2 设计 4.2）：
  - 撤销被拒为 trimmed 时 alert 与页上的说明两句近乎重复（S2 之前也近似）；
  - `AGENTS` 模板写的 prompt 与 MCP 实际登记的对照：测试改名为 `TestTheAGENTSTemplatesNameTheUpgradePrompt`，只查模板里写的那一行（`62e40b25`）；
  - 设计已定的照旧：15.3 从解析不到的附件链接直接上传（M7-transfer T8）；22.16 重开失锁的会话、23.4 链到上级那一项（决定 390）；23.1 服务端的英文（决定 387），其中 `page/domain/errors.go:89` 的 "…it can no longer be reverted whole" 在全部被清理时同样不准，P3 或 v0.2 改；H4 永久删除的功能（`M12-history.md`）。
- **588 文档的去处与收口**（核对第 1 轮 A 的 C1-N1 = C 的 C1-N2，审查 C 可选的一条）：
  - P2 设计正文里仍写旧形式的各处照实改，注"实际见 10.2"，10.2 再列全：读 4.2 各项的人先读到正文（02-workflow 3.5，同 S1）。2.3 的决定 428、388 与 00 号文档的 428 只加注，原文不改。
  - 记录叫 `reviews/P2-S2-text-review.md`；读屏清单写在记录里，P5 设计 3.19 加它的链接（同 S1）。
  - 05-lessons 加两条：替身也不能拒真东西收下的（第 2 节）；改了文字，找全断言旧文字的地方，每种改了的语言都要有格读它（第 6 节）。"一个 ctx 结束触发的几件事之间没有先后"那一条随 586 的旁支写。
  - 总体设计 3.6 的"单篇正文不超过 5 MB"改成 5 MiB：38.6 之后界面、README 与测试都写 MiB，上限是 5,242,880 字节。
  - 核对第 1 轮三位都没有 Important、Minor（四条 Nit 都在本提交写对），不开第二轮；已推的提交说明不改写，不准处写在本记录的"更正"（同决定 547、554、571、579、583）。
  - P2/S2 先于 P2/S3 合并；S3 照决定 378 后合并，先 `git merge main`，解 `history-filters.tsx` 与两份翻译文件的冲突。

## 实施中定的

实施报告第 7、8 节、修复报告第 5 节与 P2 设计 10.2 的"与设计不同之处"（三位审查者各在自己的范围里判过；实施者没有要编号的产品决定）：

- 提交的标题：6.2 只给了每组的名字，照 docs/README.md 的 Git 一节写。
- 替身认勾着的 `- [x]: text` 不是任务项（6.1）：设计的正则同时认出它，取消勾选会答 422，而服务端那里没有任务项。实施时是照 CommonMark 读出的推断，没在服务端上核；审查 B 核实它只在冒号后是单个目的地时成立，修复第 1 轮照服务端改（决定 585）。
- `source-actions.test.tsx` 的 `readerWiki` 改成整句断言；`upgrade-page.test.tsx` 加 zh 的 `approveBody` 整句；22.11 的 `transfer-background` 另写一张 zh 的 `test.each`（那张表没有 locale 的口子）；38.8 的 `probe` 加 `text` 参数，按 `span.block` 比整句。
- 变异在设计之外加的：键逐个改回 `main` 的文字（en 22、zh 26），四个新键的 zh 改成近于旧说法的一句，`clearText` 的三处与 `staleText` 的两处各自不用帮手，`reread(false)`，23.13 的旧说法；38.8 的 e2e 一半另跑一次 LW6。
- 第 10 个提交：活下来的五句中文各补一格，不改代码。
- 修复第 1 轮：`distinctName` 去掉不再用的 `notebook` 参数，没有留一个空着的参数；19.1 那一格照 `fix-1.md` 写的"`nodesDown` 时"，但页的菜单要等树读到才出现，改用探针路由打开对话框；23.11 的 `await` 吞掉重读的失败（`.catch`），失败时照旧给旧的建议，注释写明。

## 接受与推后的

- **接受**：
  - 替身对 `- [ ]: a)b` 与服务端不同（核对第 1 轮 B 的 C1-N1，决定 585）；服务端也把 `- [ ]x`（`]` 后没有空格）读成任务项，替身不认：S2 之前就这样，说明里写明不模拟（审查 B）。
  - 23.11：重读失败时冲突说明照旧建议"改回原名"，与 S2 之前相同，不更坏；网络卡住时对话框一直是发送中，与原来已等的 `transfers.load()` 同一类（核对第 1 轮 B）。
  - 19.1 的新格用有子页的 Guide：树读到与否标题一样，这一格有效全靠 `nodesDown` 真的让树读不到。换成没有子页的 Notes，格自己就能证明；审查 C 说改不改都行，没改（`f2-unread-alone` 与 C 的 `k1-m-export-unread` 都红在它上面）。
  - run 44242250765 第一次 attempt 的 `costs-2` 用了 26:34：S2 不碰成本的测试，推断与机器有关，没查。
- **推后进 v0.2**（决定 587）：
  - 撤销被拒为 trimmed 时两句近乎重复（审查 A 的 Q2）；
  - `AGENTS` 模板写的 prompt 与 MCP 实际登记的对照（审查 A 的 N5）；
  - 设计已定的：15.3 从解析不到的附件链接直接上传（M7-transfer T8）；22.16 重开失锁的会话、23.4 链到上级那一项（决定 390）；23.1 服务端的英文（决定 387），其中 "…it can no longer be reverted whole" 在全部被清理时同样不准，P3 或 v0.2 改；H4 永久删除的功能（`M12-history.md`），部署文档与发布说明在 P5/S3。
- **未核实的怀疑**（审查者的，没有造出来）：
  - `distinct-name.ts`：一页的上级不在树里时 `placeOfTitle` 答 `""`，会被说成在顶层；活着的页的上级一定活着、`pages.tree` 是整棵树，推断走不到（审查 A）。
  - 23.11 的条件 `reverted_by.length > 0` 不分"撤销它的那次已被撤销"：与 `changeset-page.tsx:238` 已有的写法相同，没在服务端上核对这时撤销会不会冲突（审查 A）。
  - `unresolved.reader`、`readerWiki` 的第二句 "…can create it from this link" 接在 "No page or attachment has this name" 之后，"it" 可以读成附件，而从链接只能新建页面；设计 15.3 有意不动第二句（`l2-create.spec.ts:118`），T8 进 v0.2（审查 A）。
  - 19.1：树在对话框开着时才到，`alertdialog` 的名字随之变，读屏会不会再念一遍（审查 B）。
  - 6.1：拒绝之后复选框是否在 alert 出现的同一刻已复原，没有逼出时序（审查 B）。
  - 从笔记本层打开导出（`transfer-page.tsx:141`，没有 `page`）时，`ExportDialog` 的 `usePageTree` 可能多发一次树的读，没有量（审查 C）。

## 更正

已推送的提交说明不改写（决定 588）；下面照审查与修复改准。

**实施报告**：

- "4 号与 8 号各 `--amend` 过一次"：4 号两次（`780d3f46 → db764166`：`tb1-trash.spec.ts` 的格式，提交时没判 lint 的 rc；`db764166 → 7e90e1b2`：`trash-background.test.tsx` 预览恢复、过了期限的一格），8 号一次（`bf95bd2a → ebc1512e`：`upgrade-page.test.tsx:1138-1142` 的四个 `KB`）（审查 C 的 N1，与 reflog 相符）。
- 第 1 节第 7 条照设计写了 26.14 的中文"约 30 条。"：测试断言的是"30 条。"（审查 B 的 N2）。
- 第 2 节的先红表把第 1 组 `index-page` 的 P4/one en 列为红过：没有那一次运行的日志，证据是四条"恒假"变异（审查 C 的 N4）。
- 第 3 节把 `TestLintBusyReturnsBeforeCleanup` 那一次红判为机器忙时的偶发：是 M10 测试的先后假设，探针能确定地复现（审查 C 的 Q1，决定 586）。
- 第 8 节第 1 条把"替身认 `- [x]: text` 不是任务项"写成了通则：只在冒号后是单个目的地时成立（审查 B 的 M1）。
- 第 9 节读屏清单的按钮名写成 "Remove filter: …"：是 "Remove the filter “…”"（审查 C 的 N2）。

**审查**：B 第一轮 N1 的情形"页面先于左栏的树显示，用户在树回来之前打开导出"是没核的推断，不成立：`page-layout.tsx:59-60` 在 `pages.tree === undefined` 时答"未加载"，页的菜单在树读到之前根本不出现（核对第 1 轮 B 自己更正）。C 第一轮 M1 的情形前一半"树还没读完时打开导出"同样经菜单走不到（核对第 1 轮 C）。这一格守的是组件里设计定下的那一支，防以后别的调用者，不是今天用户会碰到的错。

**已推的提交说明**：

- `32ce4d15`："distinctName names it page.nameAtTop, "Notes (at the top level of Plans)""：修复第 1 轮改成 "Notes (at the notebook's top level)"（`1580e7ca`，决定 584）。
- `becb67e1`："An undo refused with conflicts reads its changeset again: undone in another tab meanwhile, the dialog says it was undone before"：重读不等，回来之前（失败时一直）仍说 "rename it back"；`6b58eea4` 起先等重读（审查 A 的 N4）。
- `a5936faf`："The fake server's task items take "- [ ]: text", and a tick of one is refused as the server refuses it"：服务端只在冒号后是单个目的地时拒；`a1138dd7` 起照服务端（审查 B 的 M1）。
- `98e0387d`："TestTheAGENTSTemplatesNameThePromptsAsServed checks the line in each language"：它查的是模板里写的那一行，不读发出的 prompt；`62e40b25` 改名（审查 A 的 N5）。
- `a1138dd7`："reads a task line with a colon after "]" as the server does"、"a colon and a destination alone"：目的地里有落单的 `)` 的（`- [ ]: a)b`），替身当成定义、服务端照写（核对第 1 轮 B 的 C1-N1，接受）。

## 反向对照

变异都经 `heavy.sh`，在实施者的工作树上，`tools/mutx.py`（`docs/v0.1/claude_to_codex/tools/mut.py` 的拷贝）改一处、跑命令、从 `.mutbak` 复原；之后工作树 0 处改动、0 个 `*.mutbak`。命令只跑那几个测试文件；没有先在没改的树上逐条跑，同一棵树的 `make test-web` 与 `wiki` 模块的 go test 全绿（审查 C 在没改的树上跑过七组 vitest 与两条 Go 命令的基线，都绿）。

**实施：114 条**（vitest 111、Go 3）：

| 组 | 条数 | 第一次 | 变异 → 红在哪 |
|---|---|---|---|
| 1 单复数 | 23 | 都抓到 | 每个 `=== 1` 恒假 → 那个 1 的格；两支对调 → 2 的格；`clearText` 的三处、`staleText` 的两处各自不用帮手 → 那一处的 1 格 |
| 2 顶层 | 2 | 都抓到 | 去掉 `place === ""` 的分支、`nameAtTop` 的 name 与 notebook 对调 → `page-tree` 4 格、`page-tree-writes` 2 格，`tag-page`、`upgrade-page`、`page-tree-drop`、`page-backlinks` 各 1 格 |
| 3 撤销 | 4 | 都抓到 | 去掉 `reread(true)`、改成 `reread(false)`、条件恒假 → 23.11 的 en、zh；条件恒真 → "conflicts stay in the dialog…"（`:187-245`，含 `:233`） |
| 4 回收站与历史 | 10 | 都抓到 | 过期一句恒传、恒不传；中性一句恒不用、恒用；去掉附件的分支；按钮名传 `node.name`；不在树里仍说 "a deleted page"；H4 两句去掉，回收站一句只在不空时显示 |
| 5 页面与导出 | 7 | 都抓到 | `refusalText` 恒用通用的、条件换码、替身不答 422 → `page-tasks` en、zh；`assetNameTexts` 的两半；导出恒用旧标题、条件取反 |
| 6 比较 | 6 | 都抓到 | 去掉 `note`、两向对调、只看加的一向、照 `split` 算（`busy` 那格）；21.11 恒用 `tooLargeLines`、恒用 `tooLargeBlock`（两格） |
| 7 搜索 | 4 | 都抓到 | `lastEmpty` 改回原始的、`every` 改 `some`；`building` 改回 `some`、改成 `pages[0]` |
| 8 单位与来源 | 3 | 都抓到 | 单位改回 `MB` → `format` 6 格；进位 1000 → `1,023 B`、`5 MiB`、`5 GiB`、`2,048 TiB`（`1 KiB`、`1.5 KiB` 照过）；`content` 恒真 → `source_admin_only` 一格 |
| 键的文字 | 52 | 47 抓到，5 活下来 | en 22 个、zh 26 个逐个改回 `main` 的文字，四个新键（`page.nameAtTop`、`trash.notInTrashUnknown`、`history.nodeDeleted`、`history.attachment`）的 zh 改成近于旧说法的一句。活下来的是 `unresolved.readOnly`、`upgrading`、`readerWiki`、`image` 与 `transfer.failure.changeset_trimmed` 的 zh：没有格读那句中文，第 10 个提交补格之后都抓到 |
| 9 模板（Go） | 3 | 都抓到 | en、zh 改回旧句 → 新测试与 `TestTheTemplatesCorrespond`；zh 少写 `upgrade` 的反引号 → `TestTheTemplatesCorrespond` |

38.8 的 e2e 一半（`mut-e2e.sh`，一次 `heavy.sh`）：`HistoryAdvice` 的 `content` 恒真、`make build`，LW6 (page) 的"an admin's rename is an admin's to undo"红在 `lw6-source-history.spec.ts:439` 的否定断言；复原、再 `make build`，同一格过。

**审查**：

- A：23.11 的探针两格（扣住这次改动的重读、重读失败），读数见 P2S2A-N4。
- B：19.1 的条件改成"树没读到也算只有这一页"，`page-menu`、`transfer-page` 46 格全过（B-N1）。
- C：在头上把代码文件换回 `f08dca69` 的版本跑新测试：23.11 两格、21.8 与 21.11 五格、38.6 六格、6.1 两格、26.13 与 26.14 两格、40 的新测试红（26.11 是文字，照过，预期）。自加四条：`m-hist-ellipsis`（树没读到时说"已删除"）、`m-search-slice`（`earlier` 含最后一页）抓到；`m-export-unread`（C-M1）、`m-fake-xcolon`（C-N6）活下来，修复第 1 轮补格。

**修复第 1 轮：15 条**（`mut-fix1.json`，与六个提交的核对同一次 `heavy.sh`，最后一步），都抓到：

| 变异 | 对应 | 红在 |
|---|---|---|
| `f1-xcolon-task`：勾着的定义形也当任务项 | C-N6 | `page-server-tasks` 的 `- [x]: /u`、`- [X]: /u`、`- [x]:/u`、`- [x]: <>` |
| `f1-any-colon`：任何 `]:` 都是定义形（替身原来的样子） | B-M1 | `page-server-tasks` 的八种非定义写法 |
| `f1-trailing`：目的地之后有东西也算定义 | B-M1 | `page-server-tasks` 的 `- [ ]: /u x`、`- [x]: /u x`、`- [ ]: a b c`、`- [x]: a b c` 一类 |
| `f1-no422`：替身不答 422 | 6.1 | `page-server-tasks` 两行，`page-tasks` en、zh |
| `f2-unread-alone`：树没读到当作没有子页 | C-M1、B-N1 | `page-menu` 的新格 |
| `f3-not-awaited`：改回不等重读 | A-N4 | `revert-dialog` 的新格 |
| `f3-no-reread`：不重读这次改动 | 23.11 | `revert-dialog` 的新格与 23.11 的 en、zh |
| `f4-*`（5 条）：四个键改回旧文字（en 三处、zh 两处） | A-N1–N3 | `text-diff` en，`transfer-background` en 与 zh，`problem-messages` zh |
| `f5-nameAtTop-en`、`-zh`：名字里又带上笔记本名 | A-Q1 | `page-tree`、`tag-page`、`upgrade-page` |
| `f5-branch-gone`：去掉 `place === ""` 的分支 | 9.5 | 同上 |

核对第 1 轮：C 在头上重跑两条（勾着的定义形当任务项，红在那四行；树没读到当作没有子页，红在新格）；B 把替身的新逻辑拷到 node 里，跑 10 种边上的写法，与服务端在 9 种上一致，不一致的 `- [ ]: a)b` 接受。

## 核对

**核对第 1 轮**（2026-11-18，对象 `62e40b25`）：三位的旧发现都关上，都没有 Important、Minor；新的 Nit A 1、B 1、C 2（A 的 C1-N1 与 C 的 C1-N2 是同一件事），都在本提交写对，不开第二轮（决定 588）。

- **A**：N1–N5、Q1、Q2 关上。`distinctName` 去掉 `notebook` 之后四种情形不变，十六处调用逐处只删了这一个实参；页面树、标签页、反向链接、锁的说明、升级页与回收站、转移、索引、历史、移动、未建链接，两种语言是同一个说法；`web/`、`e2e/` 里不再有 "top level of" 与"Plans 的顶层"；头上 6 个文件 279 格过。新的：P2S2A-C1-N1。
- **B**：M1、N1、N2 关上，另更正自己 N1 的情形。服务端解析的探针：第一轮的 17 种写法加 10 种边上的写法；23.11 的 `await` 等的确实是对话框读的那一份（与 `changeset-page.tsx:94` 同一个 SWR 键），等待期间对话框仍是发送中；替身 `toggle-task` 的码的次序没变；4 个 vitest 文件 87 格过。新的：P2S2B-C1-N1。
- **C**：M1、N1–N6、Q1 关上。f1、f5 两个提交单独检出，`make lint-web`、`make knip`、`make test-web` 都是 0（4,788、4,792 格）；两条变异都抓到；改了名的 Go 测试在头上跑到、PASS；逐个提交比较翻译文件，同一个键的两种语言在同一个提交，改了的 en、zh 文字都有格读到。新的：P2S2C-C1-N1、C1-N2。

## 门禁与 CI

- **逐个提交**（`make lint-web`、`make knip`、`make test-web`，经 `heavy.sh`，在干净的 worktree 里 `checkout --detach`；改了故事的另 `make build` 与那几个故事，9 号与 f6 另 `wiki` 模块的 go test 与 `make lint-go`；之后已跟踪的改动都是 0）：

  | 提交 | test-web 的格数 | 另跑的 |
  |---|---|---|
  | `8c144f7d` | 4,716 | — |
  | `32ce4d15` | 4,727 | TB1 3 格过 |
  | `becb67e1` | 4,735 | — |
  | `7e90e1b2` | 4,743 | TB1 3 格过 |
  | `a5936faf` | 4,754 | C4 4 格过 |
  | `ff87ae77` | 4,762 | — |
  | `f00970d4` | 4,765 | — |
  | `ebc1512e` | 4,769 | LW6 8 格过 |
  | `98e0387d` | 4,769 | `wiki` 的 go test 0，`make lint-go` 0 issues |
  | `4c9f1212` | 4,773 | — |
  | `a1138dd7` | 4,788 | — |
  | `e3671b19` | 4,789 | — |
  | `6b58eea4` | 4,790 | — |
  | `b6215ab4` | 4,792 | — |
  | `1580e7ca` | 4,792 | — |
  | `62e40b25` | 4,792 | `wiki` 的 go test 0；`make lint-go` 2：新建的 worktree 下载 golangci-lint 时 TLS 断了，同一个头上随后的 `make lint` 两处 0 issues |

  lint-web 与 knip 每个都是 0。4 号、8 号第一次在干净的 worktree 里红过，改了再 `--amend`（"更正"），表里是改过的。审查 C 在自己的工作树里重跑了 2、4、5、8、9 号与 f1、f5，都是 0、格数相同。

- **合并之前的门禁**（每一步经 `heavy.sh`，之前 `df -h /`，可用 32–38G）：

  | 步 | 实施 `4c9f1212` | 修复第 1 轮 `62e40b25` | 合并 `main` 的 `260d299d` |
  |---|---|---|---|
  | `make lint knip test-go-short build-web gen-check`（`make check` 除去 `test`） | 0（61 秒；golangci-lint 两处 0 issues，`go test -short` 94 个包 `ok`） | 0（73 秒） | 只跑 `make lint-web`、`make knip`、`make lint-go`：0（主持者的说明） |
  | `make test-web` | 0（4,773，含在 `make test` 里） | 0（4,792） | 0（196 个文件、4,792 格） |
  | `make test` | 第一次 2（M10 的 `TestLintBusyReturnsBeforeCleanup`，决定 586；它单跑 `-race -count=10` 十次都过）；第二次 0（3,799 秒，Go 191 个包 `ok`） | 不全跑（Go 只改了一个测试名，主持者的说明）；`wiki` 模块的 go test 0 | 不全跑（`main` 进来的只有组合根的测试，已过 `main` 的 CI；主持者的说明）；`wiki` 模块的 go test 0 |
  | `make e2e` | 0（576 个通过、1 个照旧跳过：WebKit 的 A15 第 3 格；166 秒） | 0（576、1，167 秒） | 不跑（同上） |

  `make test` 第二次的日志里缩进的 `--- FAIL: TestWikiCostSamplesWaitForFinished/sample-failure` 是那个测试自己起的子进程里预期的失败，顶层 `--- PASS`。

- **分支的 CI**（`ci.yml` 每次 19 项）：

  | 头 | run | 结果 | web（Test） | vitest | e2e（E2E 一步） |
  |---|---|---|---|---|---|
  | `4c9f1212` | 44242250765 | 第一次 attempt 18 项绿，e2e 卡在装 Playwright 的浏览器 20 分钟、被时限取消；第 2–4 次同样卡住（apt 的镜像约 147 kB/s，同一时段 P2/S3 的 run 也卡），只重跑 e2e；第 5 次绿，19 项全绿 | 3:49（3:11） | 195 个文件、4,773 格 | 10:15（8:04，装浏览器 1:32），576、1 |
  | `62e40b25` | 44266671504 | 第一次全绿 | 3:56（3:14） | 196 个文件、4,792 格 | 9:22（8:05），576、1 |
  | `260d299d`（合并 `main`） | 44279300477 | 第一次全绿 | 3:49（3:19） | 196 个文件、4,792 格 | 9:35（8:11），576、1 |

  `f08dca69` 的 `main`（run 44224495963）：web 3:51（3:23），195 个文件、4,694 格；e2e 10:02（8:15）。离 e2e 作业 20 分钟、web 15 分钟的时限都远（P2 设计 7.4）。
- **本提交的分支 CI**：`ebb94daf` 的 run 44284479965，19 项第一次 attempt 全绿；S2 合并为 `6505c3bf`，之后第一次 `main` 的 run 44289945428 同样第一次 attempt 19 项全绿（`web` 3:58，Test 3:16；e2e 9:37，E2E 一步 8:09）。

## e2e 的用时

S2 改到断言的五格，都只在 Chromium，照 CI 的 list 报告逐格加（审查 C 用 `gh` 只读核对，相同）：

| 格 | `f08dca69` 的 `main`（run 44224495963） | `4c9f1212`（run 44242250765 第 5 次 attempt） |
|---|---|---|
| C4 (page) `c4-guard.spec.ts:150` | 7.0 | 6.7 |
| C4 (page, notebook) `:253`（分支上 `:255`） | 3.5 | 3.6 |
| TB1 (page) `tb1-trash.spec.ts:224` | 6.2 | 6.3 |
| TB1 (preview) `:346` | 0.896 | 0.969 |
| LW6 (page) `lw6-source-history.spec.ts:395` | 1.7 | 1.7 |
| 合计 | 19.30 | 19.27 |

差 −0.03 test-seconds，对 7.4 的推断（S2：0）。e2e 作业 10:15（E2E 8:04），基准 10:02（8:15）；577 个，576 个通过、1 个跳过，两边相同。本机：TB1 3 格 9.9–10.1 秒，C4 4 格 11.4 秒，LW6 8 格 11.3–12.0 秒（Playwright 报的整份时间，含启动）；`make e2e` 2.7 分钟（4 个 worker）。修复第 1 轮与合并 `main` 不改故事。

## 读屏与人工清单（交 P5/S3）

P2 设计第 9 节要各 Step 的记录列出，P5/S3 照抄进 `manual/screen-reader-checklist.md`。第 9 节"只从 ARIA 推断的八项"没有 S2 的；S2 改了念出来的两处：

- **22.12 从预览恢复、过了期限被拒**：`h1` 经 `aria-describedby` 念拒绝（`trash.expired`），下面不再有同一件事的第二句（`trash.expiredItem`）。核对：真实读屏上这件事只念一遍。
- **23.13 历史的筛选**："移除"按钮的名字是 `history.removeFilterLabel`："Remove the filter “Attachment: a.png”"（附件）、"Remove the filter “A deleted page or attachment”"（树里没有的节点）；树还没读到时是 "Remove the filter “…”"（`history-page.test.tsx:1023`、`:616`、`:783`）。核对：念出来是什么，省略号怎样念。

## 环境

实施、审查与核对各在自己的工作树里（实施者 `nerve-wiki-m12-p2-s2`，审查者见表头），`TMPDIR`、`GOTMPDIR` 与原始日志在 `~/.cache` 下各自的目录，锁固定在 `/tmp`，用完删掉；探针（审查 A 的 `zz-probe-a.test.tsx`、审查 B 放在 `server/internal/platform/markdown/tasks/` 下的 Go 探针、审查 C 的 lint 探针）从没提交。

- 实施者在干净的 worktree `~/.cache/m12-p2-s2/wt-verify` 里逐个提交核对，写完报告删了。
- 修复第 1 轮，f3 起锁前排着 P2/S3 与旁支的长批：实施者撤了自己排着、还没拿到锁的那一次 `make lint-web`，改为直接跑（几秒的轻活）；每个提交的正式核对另在一次 `heavy.sh` 里（决定 508）。
- P2/S3 的审查与修复、几条旁支同时在跑，用时含在锁上排队的时间。要留的汇总在设计工作目录的 `m12/p2s2/`（`logs/`、`tools/`、`review/`、`docs/`），实施报告 `impl-report.md`、修复报告 `fix-1-report.md`；本记录与文档的改动由脚本（`p2s2/docs/apply_p2s2_docs.py`）写出。
