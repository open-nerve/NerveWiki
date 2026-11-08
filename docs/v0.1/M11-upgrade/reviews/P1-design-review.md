# M11/P1 冻结、发起与回滚的设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [01-P1-freeze-start-rollback.md](../01-P1-freeze-start-rollback.md) 的初稿（2026-11-08，1,476 行，工作区里未提交；快照是设计审查工作目录里的 `p1-draft.md`），对照 `main` `9184f67b` 的代码（与 `7ff74a67`、`d9169baa` 相同，其后只改了文档）、[00 号文档](../00-M11-design.md)（`d9169baa` 被接受的版本）、[M11 设计审查](M11-design-review.md)、四份移交（M10-mode、M5-events、M8-P4-background、M9-mcp-upgrade）与 `v0.1-design.md` |
| 审查方式 | 三位审查者并行，按领域分：<br>A：S1 的冻结与 S3 的回滚（第 0、1 节，2.1 第 3、5、7、10、12 条，3.2 的 `changesets.upgrade_id`，3.4–3.6，3.16–3.20，3.22，3.24，6.1、6.3 与第 7 节里属于它们的行）。只读，逐条对着代码走查，没有跑测试。<br>B：S2 与 P1 的迁移（00055–00059 的 SQL、wiki 的表、清理与删除、`UpgradeStates`、基线与会话、`computeIn`、`TryHold`、按页的待建数、发起与读、`checkPages`、日志）。读代码之外，在一次性的 PostgreSQL 18.6 容器（`postgres:18.6-trixie`，经 `docs/v0.1/claude_to_codex/tools/heavy.sh`）里实测了第二遍的读与 `SetBaselineWanted` 的计划，脚本与输出（`second_pass.sql`、`second_pass2.sql`、`setwanted.sql` 与 `.out`）在设计审查的工作目录里，未入库。<br>C：S4 的网页、e2e、测试与完整性（第 4、5 节，第 7 节的网页与 e2e 的行，第 8、9 节，附录）。只读，没有跑测试 |
| 日期 | 2026-11-08 |
| 结论 | A：Important 0、Minor 3、Nit 6。B：Important 1、Minor 1、Nit 3、未核实的怀疑 1。C：Important 0、Minor 2、Nit 9、未核实的怀疑 1，过度设计无。全部照审查者的建议改（主持者的修订说明），修订直接写进设计（初稿留作快照 `p1-draft.md`），修订之后 1,548 行 |

## 发现与处置

节号是修订后的。"第 n 条变异"是修订后的 7.3。

### A：S1 的冻结、S3 的回滚

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-M1 | Minor | 交错第 16 条照写不确定：整个程序里心跳固定每秒一次（`transfer/module.go:185-188`）；任务被 SQL 记为中断之后，下一次 `BeatJob` 读不到 `running` 的行，`beat` 调 `stop(errGone)`（`job_run.go:164-166`），放开之后 `running` 已取消，不跑下一批、没有 `ErrStaleUpgradeWrite`。00 号文档要钉的是"心跳写不进、worker 照跑"（`:148-160`）。第 11 条（取消）正相反，要心跳照常读到取消 | 照改：第 16 条在 `wiring.transfer` 包住的 `Tree` 的第二批的调用里停住，测试在自己的库里装一个只认这个任务 id 的触发器，`BeatJob` 从此交回错误、只记 warn（第 234 条）；以 Rescue 的 `InterruptJobs` 那条 SQL 记为失败、结束回滚、放开；断言 warn 出现过、那一批什么也没写。第 11 条照 `TestCancellingABackgroundJob` 的 `stopAtFirst` 与 `cancelSeen`，不装触发器；两条的接线分开写（7.2）。3.7 的"走单元的错误日志（error 级）"改正：那时 `FinishJob` 是 0 行，只记"stopped" |
| A-M2 | Minor | 第 53 条变异（每一遍不核对升级）与第 38 条的 `Writing` 一半没有能杀它们的测试：同一遍里的一批由守卫的过期核对拒，有没有每遍的核对都一样；`TestEndUpgradeRollback` 用假的端口，碰不到 SQL，整个程序里也没有"回滚在跑时继续、结束答 `transfer.busy`" | 照改：`TestUpgradeRollback` 在第二次 `PlanRollback` 之前停住（`Tree` 的包装），两遍之间以 SQL 结束升级，断言两者看得出的差别（原样只记"stopped"、任务行照旧 `running`、没有单元跑；变异下全部活节点成了先删的子树，单元被拒，任务 `internal`、error 级的一行）；`TestOneWritingJobAtOnceInANotebook`（`transfer/adapter/postgres/store_test.go:304`）加排队的回滚任务的行；`TestUpgradeRollback` 加"回滚在跑时继续与结束回滚答 409 `transfer.busy`"（7.1、7.3 第 38、53 条） |
| A-M3 | Minor | 2.1 第 12 条的前提与代码不符：撤销与回滚的 `place` 把次序值直接设成计划给的（`unit_revert.go:149-157`），`moveSubtree` 不重新编号（`unit_move.go:75-107`），放到一旁的一步保持次序值（`revert_schedule.go:520`），删除不动兄弟；重新编号只在新建、移动、恢复、改位置里。合成的一遍做完就到基线 | 照改：理由改正，判据"差没有变小就 `internal`"留作终止的保证，00 号文档的那一句用改正之后的理由；`TestUpgradeRollback` 断言先删的几遍之后只有一遍合成、之后差为 0，性质测试同时断言"一遍"（2.1 第 12 条、3.18、7.1；第 72 条变异） |
| A-N1 | Nit | `errGone` 不导出（`job_run.go:33`），组合根的 `treeError` 交不回它；照 `errGone` 不写结束时，测试里以 SQL 造出的那一行留在 `running` | 照改：导出为 `ErrJobGone`，`treeError` 把 `ErrRollbackGone` 交回它；取"不写结束"（第 233 条），写明测试自己收拾那一行（2.3、3.18、3.19） |
| A-N2 | Nit | 两个结束的钩子没写笔记本行已不在的情形：`LockNotebook` 不收已删的行（`notebooks.sql:16-24`），交错第 12 条的"之后 `FinishJob` 交回 `!ok`"不准 | 照改：锁不到行就什么也不写、不调 `End` 或 `Stopped`、记"stopped"（照 `export.go:403-407`）；第 12 条改写，`rollbackStopped` 的同一情形由 `TestARollbacksEnd` 的假端口盖住（3.19、7.2；第 69 条变异） |
| A-N3 | Nit | `finish` 在 `succeed` 不为空时直接交回，后面"ended"那一行不记（`job_run.go:189-191`） | 照改：`rollbackSucceeded` 自己记"rollback ended"，同导出（`export.go:427-428`），日志的测试核对（3.19、3.23；第 70 条变异） |
| A-N4 | Nit | `page.Deps` 的字段是 `EditSessionVetoers`（`page/module.go:190`），不是 `SessionVetoers` | 照改（2.1 第 7 条） |
| A-N5 | Nit | `transfer.New` 今天对缺的依赖不 panic，`:176-180` 只是字段的说明，先例不存在 | 照改：写成新加的检查，列出三处调用（`bootstrap/wire.go:212`、`transfer/contributors_test.go:85`、`transfer/adapter/http/module_background_test.go:37`），第 6 节的理由照改（3.19、第 6 节） |
| A-N6 | Nit | 00 号文档第 3 节第 3 条要求导入、恢复、撤销的每一批上都有冻结的整个程序测试，P1 只测了它们的开始 | 照改（主持者取"加测试"）：`TestUpgradeFreezeReachesEveryPath` 加"任务的批"：三种任务各停在两批之间，以 SQL 设上升级的列，下一批以 `refused`、`notebook.upgrading` 停下、什么也没写；写明是纵深防御的一层（7.1；第 68 条变异） |

### B：S2 与 P1 的迁移

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | 第二遍的分页读在来源区页多时是平方级的：整组页一次传入 `MATERIALIZED` 的 CTE，连接之后按（页、位置）的游标排序再取 512 行。实测 10 万页里 3 万页在区里、各 10 条待建（30 万条边）：强制自定义计划整条流 18.1 s（游标越往后越慢），通用计划每批 256 ms（约 150 s），自动选择 66.2 s；照 `SourceValues` 每 512 页一段读，0.17 s。发起的期限是 3 s 到 11.25 s，每次发起都会答 503；成本守卫没写区里的页数，抓不到 | 照改：第二遍照 `SourceValues` 把排好的页每 512 个一段读，段内由页驱动 `page_links` 的主键、按（页、位置）的游标分批（第 235 条），两种计划都有计划的测试；发起的成本守卫写明 3 万页在区里、各 10 条；第 62 条变异（整区一次传入）由它杀；B 的数写进 2.1 第 13 条、7.6 与附录（3.13） |
| B-M1 | Minor | `wiki.Deps` 没有 `Seeds`（只交给 `wiki.NewTemplate`，`bootstrap/wire.go:196`）；端口的类型是 `Seed`、页要带 `Parsed`；`wiki.TemplatePages` 不给 facts；组合根的 `wikiSeeds` 要转 `Upgrade` 而不在文件表里。照文实现第 12 步编译不过 | 照改：复用启动时解析好的模板，`*app.Template` 加 `SeedUpgrade`，`Seed` 加 `Upgrade`，`wiki.Deps` 加 `Template`，`bootstrap/wiki.go` 的 `wikiSeeds` 进文件表；删掉"`Seeds` 已有"（3.9、3.14、3.15） |
| B-N1 | Nit | 没有测试抓"发起不写保护行" | 照改：`TestUpgradeStart` 断言补写的 `raw` 恰有一行 `baseline = false`、已有的 `raw` 没有（7.1；第 63 条变异） |
| B-N2 | Nit | 写成 `=` 的合取只有一个方向的反例：保护行带名称、次序有而名称空、只有发起人 | 照改：3.2 的反例表各补一行（第 64–66 条变异） |
| B-N3 | Nit | 发起算了整份摘要，只为用 `stale_pages` 与待建数；摘要单独在 10 万页上就要 1.7–2.0 s | 照改（主持者取"只算用得到的"）：`computeIn` 再拆出 `prepareIn`，发起只准备 `wanted` 一条规则；2.1 第 11 条照改，第 232 条的成本因此更宽，7.6 照实量（3.13、3.14；第 67 条变异） |
| B 的未核实的怀疑 | — | 10 万页的发起各段加起来可能超过 3 s | 7.6 写明 S2 的记录分段记下各段的耗时；放不下照第 232 条 |

### C：S4 的网页、e2e、测试与完整性

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-M1 | Minor | 矩阵的种子是共用的（`matrixJobs()`，`bootstrap/permission_matrix_transfer_test.go:41-50`）；"another's rollback, queued"占着写入键，种在哪本，那本的发起、放弃、结束回滚就答 `transfer.busy`；设计没写种在哪里 | 照改（主持者取 `reviewing` 的五本）：那里只有只读的 `getUpgrade` 一行；"its own rollback, ended"已结束、不占写入键，种在 `rolling_back` 的五本上；写明理由（3.22） |
| C-M2 | Minor | 4.4 漏了 00 号文档第 9 节要的 vitest：回滚停下之后不刷新就出现两个按钮与三个数、升级页只给管理员与接入一节的两种写法、对话框的焦点、`pages-locked` 认新码；发起对话框的各种情形；7.3 没有对应的变异 | 照改：`general-page.test.tsx` 与 `upgrade-page.test.tsx` 的各条写进 4.4；第 74–79 条变异各配一条 vitest |
| C-N1 | Nit | "插在第一个守卫的码之前"对照今天的列表不成立（M10 把来源的码排在最前），照字面插会排到 404 之前；第 6.1 节"十个单元的操作"数多了一个 | 照改：照 M10/P1 写出 14 个操作改后的 `x-problem-codes`（3.6 的表）；6.1 改为"九个经单元的操作与 `openEditSession`" |
| C-N2 | Nit | `writesPages` 不导出会让 `W/app/effective-role.test.ts` 编不过 | 照改：保留导出（4.4） |
| C-N3 | Nit | `jobTitle` 有兜底，漏了 `rollback` 一支 tsc 不报 | 照改：`transfer-names.test.ts` 加一条（4.3；第 73 条变异） |
| C-N4 | Nit | 第 61 条变异配错了测试：按钮由 store 的 `writingKinds` 决定，假服务端的那份只决定答不答 `transfer.busy` | 照改：杀它的测试改为"假服务端有一个页面还没读到的回滚任务，开始导入，页面显示 `transfer.writingBusy`"（4.3、7.3） |
| C-N5 | Nit | 刻意的 409 写在了 UP1 上，UP1 的页面版本不引起 409；UP2 的页面版本才引起；"已是 LLM Wiki"在 P1 的页面上到不了 | 照改：`expectConsole` 移到 UP2，页面版本只走编辑会话与写入任务两种，另两种只在 API 版本（第 5 节） |
| C-N6 | Nit | 第 8 节 13.4 的几条没写出文字；13.1 第 8 条的 S1 文字说 wiki 也定义 `notebook.upgrading`，那时还没有 | 照改：13.4 第 1、3、4、5、6、8 条逐条写出新文字；13.1 第 8 条 S1 写 page 与 transfer，S2 补 wiki |
| C-N7 | Nit | 第 9 节交给 P2/S2 漏了两个工具的行进 `upgradeToolCodes()` 与 `TestUpgradeToolsMatrix` | 照改（第 9 节） |
| C-N8 | Nit | `coverage_test.go:155` 应是 `permission_matrix_coverage_test.go:155`；取消按钮在 `transfer-job-row.tsx:168`；`schema.gen.ts:3678` 是别名 | 照改（3.22、4.1、4.4、附录） |
| C-N9 | Nit | 控件表的新测试文件写在"`effective-role.test.ts` 旁"，两者不在同一目录 | 照改：写明放在 `pages/notebook/`，经各自的组件渲染三处的控件（4.4） |
| C 的未核实的怀疑 | — | 第七份在 P1 末的 150–250 秒没有量 | 照旧由 7.6 在 S2、S3 量 |

## 重叠

没有两位审查者报同一条。A-M2 的第 53 条与 A-N1 相连：不写结束的结局让第 53 条有可观察的差别（第 233 条）。B-I1 与 B 的未核实的怀疑都落在发起的成本上，一起由 7.6 的分段量收口。

## 主持者的决定

主持者在修订说明里定下的处置（作者照做；负责人可以改判）：

- **B-I1**：第二遍照 `SourceValues` 的做法，排好的页每 512 个一段读，段内由页驱动；两种计划都量；发起的成本守卫写明来源区的规模；变异表加"整区一次传入、按（页、位置）游标分页"，由成本守卫杀；B 的脚本与数写进 7.6 与附录。
- **B-M1**：复用启动时解析好的模板，组合根的 `wikiSeeds` 进文件表，`wiki.Deps` 照实写，第 12 步的类型照端口写。
- **B-N3**：接受，发起只算它用得到的；第 232 条的成本因此更宽，7.6 照实写。
- **A-M1**：交错第 16 条用一个返回错误的 `BeatJob`（测试的钩子）逼出"心跳写不进、旧的 worker 照跑"；第 11 条照常的心跳；两者分开接线，写明钩子在哪里、怎样不进生产的路径。
- **A-M2**：第 53 条在两遍之间停下、断言不同的结果；第 38 条的 SQL 一半加进 `Writing` 的适配器测试，另加一条整个程序的"回滚在跑时继续或结束答 `transfer.busy`"。
- **A-M3**：改正第 12 条修订的理由，规则留作终止的保证；加"一遍合成的计划到达基线"的断言；写进 00 号文档时用改正之后的理由。
- **A-N6**：P1/S1 的整个程序测试加上"每一批"的冻结，写明是纵深防御的一层。
- **C-M1**："another's rollback, queued"种在 `reviewing` 的五本上，写明理由。
- **C-M2**：4.4 补上 00 号文档第 9 节要的那几项 vitest，7.3 各配一条变异。
- **其余的 Nit** 照审查者的建议改；C-N1 照 M10/P1 写出每个操作改后的列表。

### 设计者在这一轮的取舍 233–235（主持者的决定，负责人可以改判）

照修订说明，从 233 起编；本文的 2.3 写着同样的三条，接受本文的文档提交把它们写进 00 号文档第 11 节。

| # | 决定 | 依据 |
|---|---|---|
| 233 | 升级已不是任务的那一次时，回滚的任务不写结束：`errGone` 改名导出为 `ErrJobGone`，组合根把 page 的 `ErrRollbackGone` 交回它，`finish` 照今天的 `errGone` 只记"stopped"。生产上到不了那一行还在 `running` 的情形，照常写结束又要一个现有失败里没有的状态；测试里以 SQL 造出的那一行由测试收拾 | A-N1（两种做法里取前一种）、A-M2 |
| 234 | 交错第 16 条的"心跳写不进"由测试在自己的库里装的触发器逼出（只认那个任务 id、新旧状态都是 `running` 的 `BEFORE UPDATE`），`BeatJob` 交回不是 `ErrNoRow` 的错误；不在 `transfer.Deps` 留只给测试的口子 | A-M1（审查者给的"`Deps` 的口子或别的同等写法"里取后者） |
| 235 | 第二遍的一段之内照（页、位置）的游标分批，由主键的数组扫描按键序交回、每批至多 512 行，不一次读一段的全部的头：一页至多 10,000 条链接，一段的头最坏 512 万行。S2 的计划的测试核对两种计划都没有 `Sort`；做不到时退回审查者实测的形状，并把段缩到最坏也放得下 | B-I1 |

## 作者的更正

1. **3.7 的日志**：初稿说 `ErrStaleUpgradeWrite` 走 error 级的日志、交错第 16 条读它。后台的批把它照"别的错误"归为 `internal`（`transfer/app/background.go:262-297`），而那时任务行已被记为失败，`FinishJob` 是 0 行，只记"stopped"；没有 error 级的一行。改正，第 16 条断言心跳写不进的 warn 与"什么也没写"。
2. **交错第 16 条的"记为 `interrupted`"**：Rescue 的 `InterruptJobs`（`jobs.sql:120-132`）把行记为 `failed`、失败 `interrupted`，不是一个 `interrupted` 的状态。改正。
3. **`checkPages` 的规则名**：是"pages without exactly one content"（`bootstrap/interleavings_page_test.go:91-92`）。改正（3.12）。
4. **第二遍的读不再判当前**：范围在同一个事务里已读到当前的页（`s.current`），Go 只把区里当前的页交给查询，SQL 不再连 `indexed_pages`（3.13）。
5. **几处行号**：`SourceValues` 是 `linking/app/lint_reads.go:271-310`（排好的核对 `:274-279`）；`*app.Template` 与 `NewTemplate` 是 `wiki/app/template.go:23-53`。

## 审查者核对过、没有问题的

**A**：`FreezeAdmits` 的次序与 4.5（含第 225 条）一致；`refusalOf`、`domain.Refuse`、`Grants.Ask` 的行为与文中相符；删除都经 `DeleteSubtree` 的 `OpDelete`，`batch` 的拒绝落在那个删除的操作上；`ensureChangeset` 在守卫之后，冻结之下并进已有变更集的只有回滚的任务；四处预检各有只有它答的情形；transfer 开任务的锁序；00058、00059 与最新的定义一致、Down 的次序成立；`RollbackFreeForest` 与 `RollbackLeft` 的 SQL；先删的切分与 `DeleteNodes` 的规则、批的界；取消与结束的锁序没有环；矩阵的两个取消的变体；第七份的模式。

**B**：00055–00059 与 00 号文档 4.2 逐项一致，NULL 与优先级都对；待建数的上下界与 `CapCount` 一致；`CaptureBaseline` 的列都在、插入时检查一定成立；`SetBaselineWanted` 在三种统计状态下的计划不是平方级的（实测）；跨模块的外键与 wiki 的 sqlc 一项；`ANALYZE` 作为 sqlc 查询有先例；授权的写法；清理器的次序、删除的三条路都经订阅者、笔记本不能恢复、用户不被清理；按笔记本的两条会话的查询、与心跳没有死锁；修订 2 的加锁次序；`computeIn` 能在写事务里跑；补写的页不会触发 `stale_pages`；根下的 `AGENTS` 在第一遍里；`TryHold` 与 `Runs` 的内部相合；S2 的每个提交单独为绿；第 22–36 条变异都有对得上的测试（B-N1 除外）；发起的三行日志的字段。

**C**：只读收成一处的 19 处调用与控件表没有漏写的控件；`EditLost`、横幅、发起、路由的引用；各服务端 Step 的网页部分在加它们的那个提交里；e2e 的 `threeBrowsers`、`expectConsole` 的写法、LW12 占住 worker 的写法对 UP3 适用、夹具的引用；契约的 `Notebook`、14 个冻结操作、新码的去向、其余的引用；矩阵的两个取消的变体与 `matrixRows`；第七份的各处行号；第 8 节的锚点；第 0 节说每个 Step 单独合并都绿；2.1 第 6、8、9 条。

## 未采纳

- **A-N1 的第二种**（照常写结束）：没有取，理由在第 233 条。
- **A-M1 的"在 `transfer.Deps` 留只给测试的口子"**：没有取，用触发器（第 234 条）。
- **B-I1 的"一次读一段的全部的头"**：作为退路留着，首选段内按游标分批（第 235 条）。
- **A-N6 的第一种**（在 2.1 写明为什么不测每一批）：主持者取了"加测试"。
- 没有一条审查者的建议被判为错的。

## 新的问题

没有要负责人现在定的。实施时要看的：第 235 条的计划在 PostgreSQL 18 上能不能不排序（S2 的计划的测试与记录）；发起在 10 万页、3 万页在区里时各段的耗时与 `ctxStart` 的余量（第 232 条）。

## 修订核对第一轮

| 项 | 内容 |
|---|---|
| 核对对象 | 修订后的 [01-P1-freeze-start-rollback.md](../01-P1-freeze-start-rollback.md)（1,548 行，工作区里未提交；快照 `p1-rev1.md`）与本记录的上面各节，对照初稿 `p1-draft.md` 与主持者的修订说明 |
| 核对方式 | 原来的三位各核对自己的发现，并在自己的范围里找修订带进的问题：<br>A：读 `diff` 里 2.1 第 7、12 条，2.3 的第 233、234 条，3.18、3.19、7.1、7.2 第 11、12、16 条与 7.3 第 13、38、53、68–72 条；新依赖的代码逐条核对（`jobs.sql` 里每条改 `transfer_jobs` 的语句、`background.go`、`job_run.go`、`export.go`、几个后台任务的整个程序测试、`transfer.New` 的调用处）。只读。<br>B：逐条核对 B-I1、B-M1、B-N1–N3；在一次性的 PostgreSQL 18.6 容器（经 `heavy.sh`）里照第 235 条的写法重跑第二遍的读，脚本与输出 `second_pass3.sql` 与 `.out` 在设计审查的工作目录里，未入库。<br>C：只读；14 个操作改后的 `x-problem-codes` 用脚本与今天的契约逐个比对；新引的 `file:line` 都核过 |
| 日期 | 2026-11-08 |
| 结论 | 上一轮的发现全部关闭（A-M1 的一个断言、B-N2 与 C-N6 的修订各带出一条新的）。新发现：Important 0，Minor 1，Nit 4。全部照审查者的建议改；这一轮没有新的取舍，决定照旧到 235。修订之后 1,558 行。改过之后 A 核对 A2-M1 关闭；设计审查到此没有 Important 与 Minor |

### 三位的结论（上一轮的发现）

- **A**：A-M1–M3、A-N1–N6 全部关闭；A-M1 的主体关闭，"warn 出现过"的断言另见 A2-M1。核对了第 234 条的触发器不碍别的写（保持 `running` 的只有 `BeatJob` 与请求取消，`InterruptJobs`、`FinishJob`、`CancelQueued` 都改状态）；第 53 条变异的两种结局与代码相符；"任务的批"停在 `RunBatch` 的调用里时不在单元的事务中，以 SQL 设上升级不等锁；第 233 条在生产上到不了那一行还在 `running` 的情形。
- **B**：B-I1、B-M1、B-N1、B-N3 关闭，B-N2 关闭而带出 B2-N1。第 235 条的形状实测：10 万页里区中 30,001 页、46 万条待建的边（一页 10,000 条），四种单批情形都是 `Index Scan using page_links_pkey`，`Index Cond` 含 `source_id = ANY(…)` 与行比较，没有 `Sort`，单批 0.20–2.2 ms；整条流（59 段、956 批）自动选择 0.36 s、强制自定义 0.37 s、强制通用 0.31 s，没有统计的新表 0.31–0.38 s，成本是线性的。`prepareIn` 只准备 `wanted` 一条规则在代码上可行（`linking/app/lint_rules.go:41-66`、`:96`、`lint_results.go:120-132`）。
- **C**：C-M1、C-M2、C-N1–N9 全部关闭，C-N6 关闭而带出 C2-N1。3.6 的 14 个列表去掉 `notebook.upgrading` 之后与今天的契约逐项相同、次序不变；第 74–79 条变异各有杀它的一格；每个提交单独为绿；本记录的格式与 M10 的同类记录相同。

### 主持者的裁定

照审查者的建议改，五条都改：A2-M1 写明等那一行 warn 的办法与断言的次序；A2-N1 改正引用；B2-N1 把列出的每个 `=` 合取的两个方向补全；C2-N1 把第 8 节 13.4 第 3、6 条拆成 S2 与 S3 两段；C2-N2 在 3.22 写明种子种完之后跑 `checkPages`、任务 id 登记进 `workspaceOfRow`。

### 新发现与处置

| 编号 | 级别 | 一句话 | 处置 | 改在哪里 |
|---|---|---|---|---|
| A2-M1 | Minor | 交错第 16 条的"warn 出现过"不等就时过时不过：这行 warn 只在触发器装上之后、行还是 `running` 时落了一次心跳才记（`BeatJob` 只更新 `running` 的行，`jobs.sql:75-81`），记为中断之后心跳交回无行、走 `stop(errGone)`、不记它（`job_run.go:153-166`）；心跳每秒一次 | 照改：应用由 `newAcmeTeamOn` 建、交一个记进共用缓冲的 logger；装上触发器之后轮询缓冲，等到这一行（期限 `interleavingWait`）再记为中断；断言 warn 在"记为中断"之前，之后断言那一批什么也没写、任务行的状态与"stopped"。写明"stopped"可能来自 `errGone` 一支，也可能来自 `rollbackStopped` 的 `!ok`，两者都满足断言 | 7.2 第 16 条 |
| A2-N1 | Nit | 7.2 导言的引用错了：`stopAtFirst` 在 `bootstrap/interleavings_background_stops_test.go:73`，`:689` 起是 `stopAfter`；`cancelSeen` 在 `bootstrap/background_test.go:236` | 照改 | 7.2 导言、第 11 条 |
| B2-N1 | Nit | "每个写成 `=` 的合取两个方向各一行"说得比表里的多：00055 缺"只有时刻"；`upgrade_nodes_page_check` 的四个 `(revision IS NULL) = (x IS NULL)` 缺另一个方向；"基线页缺版本"没写另四列也空，一行会同时违反五个合取；00057 的提交检查"缺时刻"同时违反两个合取，每个合取的另一方向没有单独的一行 | 照改：加"只有时刻"；保护行只带 `content_hash`、`byte_size`、`wanted`、`accepted_hash` 各一行；"基线页缺版本"写明版本五列都空；00057 加"只有 `submission_id`""只有 `submitted_by_id`"；总括句改为"每行只违反一个合取"；第 80–82 条变异 | 3.2、3.10、7.3 |
| C2-N1 | Nit | 第 8 节 13.4 第 3、6 条的 S2 一句写了 S3 才有的东西（UP3 的豁免、`rolling_back` 的五本与占写入键的种子） | 照改：两条各拆成 S2 与 S3 两段 | 第 8 节 |
| C2-N2 | Nit | 3.22 没写升级的变体的种子要过的两项核对：种完之后跑 `checkPages`（同 M10 的变体，`bootstrap/permission_matrix_wiki_test.go:657`），以及 `targetViolation` 要路径里的每个 `{…_id}` 是这一列的工作区里种下的行（`bootstrap/permission_matrix_coverage_test.go:142-147`） | 照改：3.22 加一段，写明种完之后跑 `checkPages` 与它要哪些行成立；`cancelTransferJob` 的两个变体的任务 id 登记进 `workspaceOfRow`（`bootstrap/permission_matrix_seeded_test.go:476-477`），S3 加 | 3.22、第 8 节 13.4 第 6 条 |

### 审查者核对过、没有问题的（这一轮）

- **A**：第 234 条的触发器的条件只碰 `BeatJob` 与请求取消，第 16 条不取消、第 11 条不装它；第 53 条的原样与变异两种结局；"任务的批"的去处（`bootstrap/transfer.go:253-257`）；第 233 条的取舍。
- **B**：`bootstrap/wiki.go:24-31` 的 `wikiSeeds`、`wiki/module.go:19` 的 `Template` 别名、`wire.go:196` 在 `wiki.New` 之前建好；补写的 `raw` 用新的 id、写在根下的末尾；计划的测试的两种数据与"做不到时退回"的写法与实测相合；第 62–66 条变异各有对得上的测试。
- **C**：本记录的格式与内容；第 9 节的交接与 00 号文档第 7 节的分工相合；S3 第 1 个提交与 S4 各提交单独为绿；3.6 的 MCP 部分；新加的引用（`page/module.go:190`、`v0.1-design.md:1262`、`00-M11-design.md:767`、`store_test.go:304`、两个网页的测试文件）都在。
