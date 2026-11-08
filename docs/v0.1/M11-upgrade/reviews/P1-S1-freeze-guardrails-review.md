# M11/P1 S1（数据、冻结与删除的护栏（服务端））：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p1-s1`（实施的 7 个提交：组合根的第七份（`BOOTSTRAP_SEVENTH = ^TestUpgrade`、`test-go-bootstrap-7`、第三份的 `-skip`、CI 的 `bootstrap-7`、`06-environment.md` 的两处）`8b98ade3`，迁移 00055 与 00056、两个 sqlc 项、`schema_test` 的十个名称与 27 个反例、`TestTheUpgradeColumnsGoDown`、`runtime-grants.sql` 的两行与 `analyzed`、M10 的 lint 迁移的成本夹具先退 56、55 `3b23bad0`，notebook 的 `Upgrade`、六条读笔记本的查询与 `upgradeOf`、根的 `State.Upgrade`、契约的 `Notebook.upgrade` 与两处网页的字面量 `2f209937`，page 的 `FreezeAdmits`、`FreezeGuard`（守卫与否决者）、`decideAction`、`Write.UpgradeOwn`、`ensureChangeset` 记 `upgrade_id`、组合根的 `toPageState` 与 `[freeze, mode, lock]`、十个操作的声明、MCP 的五行与 `upgradeToolCodes()` 的五对 `1a54bfcd`，删除的护栏 `UpgradeProtected` 与 `page.upgrade_protected` `d668fc3f`，page 的三个预检（`Writer.notUpgrading`）、transfer 的笔记本端口与组合根的 `transferNotebooks`、导入的预检与开任务的判断、四个操作的声明 `54ce782f`，整个程序的五个 `TestUpgradeFreeze…`、`events.yaml` 的两处说明、删除护栏的读的成本守卫 `a28064e9`；推送之前改写过两次分支的历史：oxfmt 的四处并回第 3、4、6 个提交，lint 迁移的成本夹具的修正并回第 2 个（偏离 15、24）；从 `main` 的 `379a2d6d` 开始，审查时在 `a28064e9` 上，102 个文件、加 2,954 行、减 218 行。修复一轮四个：S1B-I1 = S1C-M2 `f66537d4`，S1B-M1 与 S1C-N5 `b7f242e7`，S1A-M1 与 S1A-N1 `e5aa8a67`，S1A-N2、S1A-N3、S1A-N4、S1C-N2、S1C-N3、S1C-N4 与 S1C-N6 的一句 `68c3c505`（13 个文件、加 324 行、减 87 行）；一共 11 个；合并见 [00 号文档](../00-M11-design.md)第 12 节），对照 [01-P1-freeze-start-rollback.md](../01-P1-freeze-start-rollback.md) 第 0–2 节、3.1–3.8、3.24、4.1、第 5 节、6.1、6.5、第 7 节的 S1 的各行（7.3 的第 1–21、64–66、68、80、81 条）与第 8 节，[P1 设计审查](P1-design-review.md)，[00-M11-design.md](../00-M11-design.md) 第 3 节、4.1、4.2、4.5、4.6、4.16 与第 11 节，[v0.1 总体设计](../../v0.1-design.md) 第 13 节，实施者的报告（24 条偏离、逐个提交的核对、门禁、实测、CI、变异的表、留下的六项） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验，用到 Docker 的都经 `heavy.sh`，先读设计与代码、自己判断之后才读实施报告：服务端的每条路径、事务与锁、码的次序（A：沿 `Writer.run`、`applyStep` 的守卫循环、开会话与接管、九个经单元的操作、后台恢复与撤销与导入的每一批、三个 page 的预检、transfer 的开任务、组合根的两个转换、六条查询与 `CreateChangeset`、`UpgradeProtected` 逐条走；另查单元之外写内容或树的路（强制解锁、心跳、上传失败时的文件、MCP 的写工具）；快的包与第七份；变异 A1–A7、W1–W6；一个临时的交错探针 `TestRevAFreezeHoldsUnderTheNotebookLock`：测试的事务以 `UPDATE notebooks` 持住笔记本行，bob 的后台恢复与新建页、之后导入与正文写各等在锁上，提交之后四个都答 409 `notebook.upgrading`、什么也没写、没有任务行，把读挪出锁的 P1、P2 与建任务时不判的 P3 都让它失败，审完删去）；数据、迁移、契约与矩阵（B：`make gen-check`；自起的 PostgreSQL 18.6 容器迁移到 56，把 `checkedRows` 的行抄进无约束的临时表，对每个反例的每个合取逐一求值；外键与删除（删发起人、硬删基线行指向的节点、硬删笔记本）；`UpgradeProtected` 在 20 万行与 102 万行、四种统计状态、自定义与通用计划下的 `EXPLAIN ANALYZE`；一个临时的整个程序的测试（读者的 14 个写全是 403、看不到的全是 404，没有一处先答 409），跑完删去；从 `api/dist/openapi.yaml` 逐个读 14 个操作的码表与设计比较，逐个看契约里的 49 个写操作；变异 a1–a17、b1–b7、c1、c2、d1、e1、f1、f2、g1、h1、i1、i2、r1、r2）；测试、变异、e2e、网页、逐个提交、CI 与量的数（C：第 2、4、6 个提交各自 build、vet、`make lint-go`、`make gen-check`、`make test-go-short`，第 4、6 个另 `make lint-web`、`make knip`、`make test-web`；第 2–6 个提交在竞态下跑 S1 碰到的模块的数据库测试与组合根里 S1 加或改的测试；头上的 `make e2e`；`go test -list` 与 `go test -race -list` 照 `Makefile` 分份；读 CI 的运行 42491802403 与最近 6 次 `main` 的组合根各份；变异 23 个（r 是设计 7.3 的行、o 是自己想的）、成本守卫的 c1、c2 与网页的 w1）。A 有一次事故：第二组的整个程序的变异在 `heavy.sh` 里排队时，同一个工作树里前台跑了一组模块级的变异，两个 `mut.py` 同时改 `page/app/unit.go`、互相盖掉备份；A 照那一处改动逆替换复原（没有用 `git checkout --`），`git diff` 为空之后整组 W1–W6 连同基线重跑，记录只用重跑的结果。C 的一次运行第一版不带竞态、太慢、占锁太久，按 PID 停下（没有结果），改成竞态下按名字挑的第二版。修复核对一轮在 `68c3c505` 上，三位各在自己的工作树里，核对自己的发现与自己范围里别人的，重跑自己存活过的变异、另加新的：A 在模块层跑 A1、A5、A7 与新的 F1–F5（不碰组合根，没有经 `heavy.sh`），读四个提交，没有重查 CI；B 经 `heavy.sh` 跑成本守卫的 c1、c2 与新的 c3，以 q1–q4 改规划的设置看计划的断言会不会误报合法的形状，在自己的工作树里临时加一个空的 00057 跑两个 lint 迁移的测试；C 经 `heavy.sh` 在一个临时的分离工作树里跑 4 个提交各自的门禁，跑 32 个变异（自己第一轮的 25 个、实施者表里的 6 个与新的 k5），量三次成本守卫，读 CI 运行 42512020813 与 costs-2 的作业日志。三位核对完工作树都干净、没有 `*.mutbak` |
| 日期 | 2026-11-08 到 2026-11-09（实施 2026-11-08；审查、修复一轮与修复核对一轮 2026-11-09） |
| 结论 | Important 一条，与一条 Minor 是同一个发现（S1B-I1 = S1C-M2，A 的 S1A-Q2 也提了一句）：删除护栏的读的成本守卫抓不到退化成顺序扫描。界取了设计 7.1 的表里这一行的"单元 2 s"，本机中位数 4–7 ms；B 与 C 的四个变异（读用不上主键、去掉主键、只剩 `upgrade_id` 走键、整表顺序扫描）都在界内通过，通用计划下慢 100 倍以上也是绿的。修复一轮以计划的断言为主、时长为辅（本机中位数的三倍，25 ms），四个都被抓到（主持者的决定 236）。Minor 另三条：导入的预检的次序没有测试钉住（S1A-M1）；M10 的 lint 迁移的成本夹具把最新的迁移写死，每加一个迁移就在 `costs-lint` 失败（S1B-M1，S1C-N5 是同一处）；设计 7.3 第 14 条点名的第二个测试 `TestUpgradeGuardrails` 抓不到它（S1C-M1 = S1A-Q1：被标的节点只在 page 之内看得到，决定 237）。Nit 十三条，S1C-N5 并入 S1B-M1，其余十二条：八条在代码、测试、注释、`Makefile` 与 `06-environment.md` 里改（S1A-N1–N4，S1C-N2、N3、N4，S1C-N6 的一句），S1B-N1 交给 S2，S1C-N1 交给 S3，S1B-N2 记下，S1C-N7 改在设计 7.3。修复一轮四个提交，唯一的生产的改动是 S1A-N2：冻结的守卫对不认识的判定答缺陷的错误、整个单元回滚。三位都没有找到答错的结果：A 的交错探针在四条路上逼不出"读了没升级、写照样落下"的窗口；B 在真库上核对 27 个反例各恰好违反一个合取，14 个操作的码表与设计一致；C 的三个提交各自绿、第 2–6 个提交在竞态下的数据库测试都通过，头上 e2e 503 个全过。实施者的 24 条偏离由三位在各自的范围里判过，都对（第 20 条照了设计的表，表本身错了，即 S1B-I1；第 10 条即 S1C-M1）。建成与修复一轮的两次 CI 运行都全绿，组合根七份的测试一步都在 450 秒之下。主持者的 4 个决定见下（第 236–239 条，接着 P1 设计审查的第 233–235 条）。修复核对一轮由三位核对，没有新的发现（Important 0、Minor 0、Nit 0）：要改代码的都关了，只改文档的处置对（文档的部分在本提交，S1C-N1 交给 S3）；B 记下一项观察（"分析过、自定义计划"一格位图读与并行顺序扫描的估计成本只差约 9%，"接受与推后的"）。不需要第二轮。合并见 00 号文档第 12 节 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：S1A-M1、S1A-N1–N4、S1A-Q1、Q2，S1B-I1、S1B-M1、S1B-N1、N2，S1C-M1、M2、S1C-N1–N7。修复一轮照主持者的修复说明（`s1-review/fix-1.md`）分四个提交。同一个发现合为一个修复：S1B-I1 与 S1C-M2（S1A-Q2 只提一句同一处）；S1A-Q1 与 S1C-M1；S1B-M1 与 S1C-N5。下面的代码位置是 `68c3c505` 的） | — |
| S1B-I1 = S1C-M2 | 重要（测试） | 删除护栏的读的成本守卫 `TestUpgradeProtectedReadsInTimeAtTheUnitsBound`（`page/adapter/postgres/upgrade_cost_test.go`）种 20 万行（两次升级各 10 万），读 1 万个 id，界取设计 7.1 的表里这一行的"单元 2 s"；同一节表上一段与 05-lessons 第 1 节的总规则是"界取本机两轮的中位数的三倍"（实施报告的偏离 20 与留下的事第 5 条已提出，A 的 S1A-Q2 同提）。真实的计划在每种统计状态下都走主键，本机 4–7 ms，CI 的 costs-2 6.0–8.6 ms，界在它们的 230–500 倍之外。把读改成用不上索引的样子，守卫照过：B 的 c1（生成的 `upgradeProtected` 写成 `(upgrade_id = $1) IS TRUE AND (node_id = ANY($2::uuid[])) IS TRUE`，四种状态的中位数 18.0、18.7、18.9、558 ms）、c2（00056 去掉 `upgrade_nodes_pkey`），C 的 c1（`(node_id::text)::uuid = ANY(…)`，只剩 `upgrade_id` 走键：44、40、66、576 ms）、c2（两列都转换，整表顺序扫描：41、41、41、581 ms），四个都存活。`upgrade_nodes` 是全实例的表，退化之后一次删除的读随所有进行中的升级的行数线性涨，按 B 的外推十次 10 万节点的升级同时进行就过了单元的 2 秒，守卫在 20 万行上却永远是绿的；名称表（`server/migrations/schema_test.go:421-422`）只钉主键的名字，不钉它的列与次序 | 已改（`f66537d4`，修复一轮提交 1，主持者的决定 236）：同一个测试、同一个 `!race` 文件、同一行 CI（`Makefile:346-347`，costs-2）。先钉住 `pg_get_indexdef('upgrade_nodes_pkey'::regclass)` 是 `(upgrade_id, node_id)` 的唯一 B 树（`upgrade_cost_test.go:55-61`；05-lessons 第 4 节：PG18 的跳跃扫描会用非前导列）。四种状态（没有统计、分析过、只有另一次升级的统计、`force_generic_plan`）各在读的那条连接上以 `PREPARE` 加 `EXPLAIN (FORMAT JSON, VERBOSE, COSTS OFF) EXECUTE` 取自定义与通用两种计划，`pg_prepared_statements` 的计数核实取到的是哪一种；断言没有对 `upgrade_nodes` 的 `Seq Scan`，读它的扫描节点恰一个、在 `upgrade_nodes_pkey` 上，索引条件同时含 `upgrade_id` 与 `node_id`（`protectedPlans`，`:142-203`，照 `paths_test.go` 的 `lookupPlan.check` 与 `search_plans_test.go` 的 `planNode`）。时长为辅：界 `upgradeProtectedBound = 25 ms`（`:35`；本机两轮中位数较大的 7.5 ms 乘 3、向上取整到 5 ms，两次 CI 的 5.7–8.6 ms 在内）。设计 7.1 与 00 号文档第 9 节的这一行随之改。变异：B 的 c1（四种状态、两种计划都是对整张表的并行顺序扫描；时长只在通用计划超界）、c2（`pg_get_indexdef` 找不到主键）与 C 的 c1、c2 都失败；新的 p1（主键的列序换成 `(node_id, upgrade_id)`）只被列的钉住抓到（`fix1/all.out`）。修复核对 B：c1、c2 被杀，新的 c3（只把 `node_id` 的条件包进 `IS TRUE`，`node_id` 落到过滤）也被杀；q3（只留位图）、q4（`debug_parallel_query = on`，套一层 Gather）照过，断言不误报合法的形状；q1、q2（关掉位图）让"分析过、自定义计划"一格改选并行的顺序扫描、测试失败，那是计划真的变了（"接受与推后的"）。修复核对 C：自己的 c1、c2 与 B 的 c1、p1 都被抓到，B 的 c1 前两种状态的时长 18.9、19.0 ms 在界内、只由计划抓到；计划用的是生成的那条 SQL（`statementOf` 取 `gen.UpgradeProtected` 实际发的语句），改生成的常量就改到计划；25 ms 是本机最大的 7.5 ms 的约 3.3 倍，对两次 CI 的余量 2.9–4.4 倍 |
| S1A-M1 | 低（测试） | 导入的预检的"判定之后先答 `notebook.upgrading`"没有测试钉住。设计 3.5 写"`StartImport.Check`（`authorize` 之后、`place` 之前）读 `StateOf`"，契约 `startImport` 的码表把它排在 `page.not_found` 之前，代码对（`transfer/app/start_import.go:80-85`）。A 的 A1（`notUpgrading` 与 `place` 对调：带 `parent_id` 的导入在升级中的笔记本、父节点不是页时答 `page.not_found`）过 `transfer/...` 的全部测试：`TestTheStartsRefuseAnUpgradingNotebook` 的"an import's Check"的父节点是存在的页，`place` 放过，断言只看最后一个调用是 `Notebooks.StateOf`。page 一侧的同类测试用会先答别的码的请求，对应的 A2 被抓到；导入这一条不对称 | 已改（`e5aa8a67`，提交 3）：`transfer/app/start_upgrade_test.go` 的"an import's Check"用父节点不存在的请求（`nowhere`，`:32-38`），断言答 `notebook.upgrading`、调用恰是 `[Notebooks.WorkspaceOf, Authorize, Notebooks.StateOf]`；升级结束之后同一请求答 `page.not_found`（它真走到 `place`），父节点是页的请求通过、笔记本没被占着（`:40-68`）。变异：A1 让两行都失败（答 `page.not_found`，调用序列少了 `StateOf`）。修复核对 A：A1 被抓到（"an import's Check"两行）；修复核对 C 同，C 的 o3b 现在除了 `TestAnImportStarts` 的调用序列，另被这一行的完整调用序列抓到 |
| S1B-M1、S1C-N5 | 低（测试的夹具） | M10 的 lint 迁移的成本夹具 `lintMigrationDropIngest`（`bootstrap/lint_index_migration_cost_test.go`）原来认定最新的迁移是 54。实施把它改成一张 `{56, 55, 54}` 的表（偏离 24）：每一步先核对当前版本、经真的 Down 退一步、核对退掉的版本与文件名，改法对，但又把"最新是 56"写死。S2 的 00057 一合并，`TestLintSourcesMigrationCosts…`、`TestLintRange…` 就以"legacy migration setup version=57, want 56 before Down"失败，S3 的 00058、00059 同样。这个夹具只在 `!race` 的 `test-go-costs-lint` 里跑，逐个提交的核对与组合根各份都碰不到它，实施者自己是在第二次 `make test`（1,942 秒）的末尾才撞上的。仓库里别的"退到某一版"的测试都不依赖最新是几（`downTo`、`search_scope_test.go` 的循环、按文件名的上界截取）；设计 6.2、6.3 没写这一处（C：每个迁移各要加一行） | 已改（`b7f242e7`，提交 2，主持者的决定 238）：新的 `lintEmbeddedMigrations`（`:142-166`）按版本读嵌入的迁移名、版本重复就失败，`lintMigrationFS`（`:103-125`）改用它。`lintMigrationDropIngest`（`:171-223`）先断言库的版本是嵌入的最新，`v > 54` 时每步经真的 Down 退一步，断言退掉的是 `v`、文件是版本为 `v` 的那个，再照旧退 54、53；不再写死 55、56，设计 6.2、6.3 不加一行。验证：`make test-go-costs-lint` 通过；另在临时的工作树里加一个空的 00057（Up、Down 都是 `SELECT 1`），夹具依次退 57、56、55、54、53，`TestLintSourcesMigrationCostsEmptySources` 通过，跑完删去、没有提交（`fix1/percommit/b7f242e7-empty57.log`）。修复核对 B：在自己的工作树里临时加一个空的 00057（跑完删去），`TestLintSourcesMigrationCostsEmptySources`（14.4 秒）与 `TestLintMigrationFixturesKeepVersionsAndLinkFacts` 的两个子测试都通过，夹具从 57 退起、再走 52、51 的原有步骤；修复核对 C：在 `b7f242e7` 上单独跑通过（14.4 秒，依次退 56、55、54、53），比它建议的改法好 |
| S1C-M1 = S1A-Q1 | 低（设计的说法） | 设计 7.3 第 14 条（删除的拒绝标在 `Changes[0]`）点名两个测试：`TestTheFreezeGuard` 与 `TestUpgradeGuardrails` 的 `batch` 一行。后者只断言 `code` 与 `op`（`bootstrap/upgrade_guardrails_test.go:40-45`）：MCP 的工具错误只带 `code`、`message` 与 `op`（`platform/httpserver/mcpserver/errors.go`），`batch` 的 `OpFailed` 只带操作的序号与工具（`page/app/apply_batch.go:624-636`），不带被标的节点。page 之外读到被标的节点的只有后台任务的 `refused`（`bootstrap/transfer.go:257`），而后台任务的单元的客户端不是 MCP，`FreezeAdmits` 不给它 `FreezeAsk`，到不了护栏。所以整个程序里这个标看不到，第二个测试对这条变异永远通过（实施的 14b、A 的 W6，偏离 10）。A 倾向于 7.3 第 14 条只列 `TestTheFreezeGuard`，并在 3.4 与 00 号文档 4.6 写明"MCP 的文字指向真正受保护的那个"今天不成立，不为它改 MCP 的错误形状 | 改在设计（主持者的决定 237）：被标的节点只在 page 之内看得到，S1 不为它改 MCP 工具错误的形状；7.3 第 14 条只列 `TestTheFreezeGuard` 并写明原因，3.4 与 00 号文档 4.6 的那一句照实改；`TestUpgradeGuardrails` 的 `batch` 一行钉的是整批在那个删除的操作被拒、什么也没写。14b 与 W6 在修复一轮照旧存活，照预料（下面"反向对照"）。修复核对 A：代码侧关，文档照决定 237 在本提交改；修复核对 C：处置对，14b、W6 照旧存活，与决定一致 |
| S1A-N1 | 细节（测试） | transfer 的两处新分支没有测试。`StartDeps.notUpgrading` 的"没有笔记本"（`transfer/app/start.go:131-132`：判定之后被删答 `notebook.not_found`）：A5（改成 `return nil`）过 transfer 的全部测试，替身的 `WorkspaceOf` 与 `StateOf` 读同一张表，造不出"判定时在、读状态时不在"；page 一侧同样的分支由 `TestAnUploadsCheckRefusesAReservedTitle/gone_after_placement` 钉住（A4 被抓到）。锁下的升级判断排在判定之后（`start.go:110-120`、`start_background.go:170-176`、`start_import.go:253-262`）：A7（挪到判定之前，读者答 `notebook.upgrading` 而不是 `forbidden`）同样通过。整个程序里预检先答 `forbidden`，这个次序只在"预检之后被降为读者、同时开始升级"的竞态里看得到 | 已改（`e5aa8a67`，提交 3）：替身 `notebooks` 加 `goneAsRead`（读状态时笔记本已删，`WorkspaceOf` 照旧在；`transfer/app/fakes_test.go:153-161`、`:185`），"an import's Check"加一行答 `notebook.not_found`；恢复与撤销的开始各加一行"by a reader"：page 的预检的替身放过，锁下的判定先拒，答 `forbidden`，不写行、不入队（`start_upgrade_test.go:109-122`）。变异：A5、A7 都失败。修复核对 A：A5、A7 被抓到；新的 F5（`goneAsRead` 不起作用）让同一个子测试失败，开关不是摆设；导入建任务的事务没有加读者一行，整个程序里预检先答 `forbidden`，可以接受。修复核对 C：A5、A7 同 |
| S1A-N2 | 细节（生产的代码） | 冻结的守卫对不认识的 `Freeze` 值是放行的方向：`GuardWrite` 的 `switch` 没有 `default`，`case domain.FreezeAsk:` 是空的，不是 Open、Stale、Refused 的值都落到 `Grants.Ask` 与护栏，授权通过就写。今天四个值都列了，不是缺陷；P2、P3 给 `Freeze` 加值而忘了这里，就是静默地放行 | 已改（`68c3c505`，提交 4，这一轮唯一的生产的改动）：`page/app/freeze_guard.go:51-61` 的 `switch` 加 `default: return fmt.Errorf("page: freeze %d unknown", f)`（缺陷的错误，整个单元回滚），注释写明（`:41-49`）。为了让测试造得出不认识的值，`FreezeGuard` 加字段 `admits`（`:30`，生产上恒为 `domain.FreezeAdmits`，`:38`），`page/app/export_test.go:35-41` 加测试用的 `WithAdmits`（同文件已有 `WithLines`、`WithMaxWork` 的做法）。`TestTheFreezeGuard` 加"a freeze it does not know"（`freeze_guard_test.go:165-189`）：动作全部授予，答的是缺陷的错误（不是 `*shared.Error`），什么也不问、不读行；同一守卫下的单元答错误，0 个节点、0 个变更集。变异：n2（去掉 `default` 的拒绝）在这一行失败（答 nil、问了 `notebook.change_mode`，单元写了 1 个节点与 1 个变更集）。修复核对 A：F1（去掉 `default`）、F2（`default` 答 nil）、F3（`default` 答 `ErrUpgrading`，一个 problem）都被这一行抓到，F4（构造器的 `admits` 恒答 Open）被已有的子测试抓到；生产上只有 `NewFreezeGuard` 一处构造它（grep `FreezeGuard{`），`WithAdmits` 只在测试构建里，代价可以忽略、不算过度设计。修复核对 C：n2 同 |
| S1A-N3 | 细节（注释） | `Grants` 的注释（`page/app/grants.go`）说"a step of a standard notebook, or one that touches no source, never does"：冻结的守卫恰在普通笔记本里问（发起人的 MCP 写，升级中的笔记本只能是普通的）；同一段后一句已改为"Only the freeze and mode guards…ask"，前一句没跟着改 | 已改（`68c3c505`，提交 4）：`grants.go:10-21` 改为"a step of a notebook with no upgrade that touches no source never does: the freeze guard asks of the initiator's MCP writes while a notebook is being upgraded, a standard one too, the mode guard of the writes that touch an LLM Wiki's source"。修复核对 A：合乎代码 |
| S1A-N4 | 细节（注释） | `TestUpgradeFreezeAnswersFirst` 的注释说她的开会话"which the lock would take over"：接管只结束开启者自己的会话（契约 `openEditSession`："someone else's lock it does not pass"），bob 的会话留着，没有冻结时锁答 `page.locked`。测试本身对 | 已改（`68c3c505`，提交 4）：`bootstrap/upgrade_freeze_rest_test.go:56-63` 改为"which the lock would refuse too, bob's session not hers to take over"。修复核对 A 核对 |
| S1B-N1 | 细节（数据） | `notebooks_upgrade_check`（`00055_notebook_notebooks_upgrade.sql:11-14`）收零 UUID 作升级的 id：真库上 `upgrade_id` 为零、`upgrading` 的一行插得进，而程序处处以零 id 当"没有升级"（`FreezeAdmits` 放过、`notUpgrading` 放过、handler 答 `upgrade: null`）：库说在升级、程序说没有。生产到不了（S2 的 `Start` 用 UUIDv7） | 交给 S2：P1 设计 3.14 第 10 步记下"`States.Start` 断言升级的 id 非零，零是缺陷"，不加合取。修复核对 B：处置接受 |
| S1B-N2 | 细节（契约的文字） | `NotebookUpgrade` 的说明引用 S2 才有的 `getUpgrade`（`api/modules/notebook.yaml:469-472`，设计 3.3 给的原文）：S1 合并之后到 S2 合并为止，`main` 的契约提到一个还不存在的操作 | 接受、记下（"接受与推后的"）：开发中的 `main` 可以接受；S2 加 `getUpgrade` 时核对这一处的名字。修复核对 B：处置接受 |
| S1C-N1 | 细节（测试） | "任务的批"只在 `upgrading` 里测（`bootstrap/upgrade_freeze_test.go:292-343`，`:299` 写死 `"upgrading"`）。C 的 o2（`page/app/unit.go:289` 造 `Write` 处，`spec.Background` 且状态是 `reviewing` 的单元取当前升级为 `UpgradeOwn`）过全部 7 个 `TestUpgrade…`。今天的守卫看不到 `Background`，任何单元按同一条规则判，这条变异是人造的；但 S3 给回滚的批设 `UpgradeOwn`，正是会出现"按状态区分后台单元"的代码的时候，而 3.24 把这一层（纵深防御）的反向对照只放在这个子测试里 | 交给 S3：P1 设计给"任务的批"加 `rolling_back` 一行（别的任务的一批在回滚中被拒、回滚自己的批放过），写在 7.1、3.24 与 6.3 的第 4 个提交。o2 在修复一轮照旧存活，照预料。修复核对 C：处置对，o2 在这一轮重跑照旧存活 |
| S1C-N2 | 细节（记录的数） | 实施报告 5.2 与第 9 节的草稿写"576 个顶层，167/58/124/136/62/22/7"：576 是不带竞态构建的清单，含 42 个 `//go:build !race` 的成本与守卫测试，它们不在任何一份的竞态构建里、在 costs、guards 的行里跑；`Makefile` 的分份注释一直用竞态构建的口径（"历史 race 编译清单的 499 个"、"当前 521 个"）。竞态构建是 534 个：158/51/121/123/62/12/7，每个恰在一份，`TestUpgrade…` 只在第七份；第 21 条的核对（第三份的 `-run` 单独收下全部 7 个 `TestUpgrade…`）照样成立。`Makefile` 的"当前 521 个…"已过时 | 已改（`68c3c505`，提交 4）：`Makefile:265` 改为"当前 534 个顶层（竞态构建，M11/P1/S1 之后）静态分配为 158/51/121/123/62/12/7"，实施者在头上重数过（`go test -race -list` 534 个，`fix1/race-list.txt`）；本记录与 P1 设计 10.1 用竞态的口径（下面"组合根的各份"）。修复核对 C：与它第一轮在竞态构建上数的一致 |
| S1C-N3 | 细节（测试） | `TestPageNotebooksConvertTheState` 的期望写 `page.UpgradeState(notebook.UpgradeRollingBack)`，与 `toPageState` 的转换是同一个式子，钉不住 page 的常量与 notebook 的字符串相同。今天 page 只比 `UpgradeUpgrading`，它由整个程序的测试钉住 | 已改（`68c3c505`，提交 4）：期望写 `page.UpgradeRollingBack`（`bootstrap/page_notebooks_test.go:90-91`）；page 的模块根原先不导出状态的常量，照 `ModeStandard`、`ModeLLMWiki` 的写法导出三个（`page/module.go:70-75`，修复一轮的偏离 1）。变异：n3（page 的 `UpgradeRollingBack` 指向 `reviewing`）失败。修复核对 C：新的 k5（page 的领域里 `"rolling_back"` 拼成 `"rolling-back"`，`page/domain/mode.go:21`）让 `TestPageNotebooksConvertTheState` 失败，第一轮的写法下它会通过；导出这三个常量照 `ModeStandard`、`ModeLLMWiki` 的先例，合理 |
| S1C-N4 | 细节（文档） | `06-environment.md` 的"不要拿 `make test-go` 内部的 `-j5` 当这些串行的样本"，`Makefile` 是 `-j6`（S1 之前就不符；第 1 个提交改了同一句的其余部分） | 已改（`68c3c505`，提交 4）：`06-environment.md:49` 改为 `-j6`。修复核对 C：与 `Makefile:190` 的 `-j6` 一致 |
| S1C-N6 | 细节（流程） | 实施者第一次 `make test` 里第五份的两个计时的测试（`TestTheMCPBatchIsRefusedWhole/the_plan's_parses_past_10_MiB`、`TestTheMCPCallStopsWhenTheClientGoes`）与第 6 个提交的 vitest 的两个计时的测试（`editor/markdown.test.ts`，20 万字符、界 1,000 ms）超界：都在 `379a2d6d` 上已有，S1 没有碰 MCP 的批的解析、客户端的断开与编辑器，负载 24（8 核的三倍）下超界是计时守卫应有的样子。06-environment 第 3 节列的重的运行没有"逐个提交的核对"，而它含 vitest 的计时断言与全量的 lint | 已改（`68c3c505`，提交 4）：`06-environment.md:41` 的"重的运行一次只跑一个"一条加一句：逐个提交的核对也经 `heavy.sh`，至少不与 `make test` 同时跑（放在第 3 节，S1C-N6 指的那一节）。修复核对 C：放在第 3 节对；实施者修复一轮的逐个提交经 `heavy.sh` 跑；同意 markdown 的平方守卫超界 0.02 ms 的判断（M10 的守卫，这一轮没有碰它） |
| S1C-N7 | 细节（设计的说法） | 设计 7.3 第 68 条"冻结的守卫放过后台任务的单元（按 `Background`）"：`Step` 没有 `Background`，守卫写不出这条，实际的写法改在 `page/app/unit.go` 造 `Write` 处（实施者：`spec.Background` 的单元的 `Write.Notebook` 去掉升级；C：取当前升级为 `UpgradeOwn`；两种都被"任务的批"抓到）。C 的 12 条里被抓到、设计没列、值得补进表的：组合根的 `transferNotebooks` 不转换升级（o9）、`batch` 的单元跳过冻结（o1）、任何 agent 的 MCP 写都去问 `notebook.change_mode`（o7，整个程序里只有 `TestUpgradeToolsErrorsCarryTheirCodes` 抓到）、删除的护栏只问子树的根（o4） | 改在设计：7.3 第 68 条写明改在 `unit.go`；o1、o4、o7、o9 补成第 83–86 条（S1，都已被抓到）。修复核对 C：处置对，o1、o4、o7、o9 在这一轮重跑仍被抓到 |
| S1A-Q2 | 存疑（成本的界） | A 只提一句（C 的范围）：成本守卫的界取了"单元 2 s"，本机中位数约 5 ms，余量约 400 倍，05-lessons 第 1 节的规矩是本机的三倍 | 同 S1B-I1 = S1C-M2（决定 236）。修复核对 A：关 |
| 偏离 | — | 实施者的 24 条偏离（[01-P1](../01-P1-freeze-start-rollback.md) 10.1、3.25）：1 `checkedRows` 加一本升级中的普通笔记本与三行；2 Down 的版本号是 56、55；3 反例每个只违反一个合取；4 名称表的 `upgrade_nodes_pkey` 两项；5 notebook 根的别名在 `notebooks.go`；6 `decideAction` 抽出、撤销也用；7 第 4 个提交里根的构造先不用连接池；8 三个预检共用 `Writer.notUpgrading`，已有的三个钉住读或调用的测试随之改；9 `TestCreateChangesetWritesItsUpgrade` 在 `upgrade_test.go`；10 第 14 条变异的第二个测试看不到被标的节点；11 `TestPageNotebooksConvertTheState` 加升级的一行、另加 `TestTransferNotebooksConvertTheState`；12 第 6 个提交的范围是 `server, api`；13 另加 `TestThePrechecksRefuseAnUpgradingNotebook`；14 transfer 的 `adapter/http` 替身的 `upgrading`；15 oxfmt 的四处以 fixup 并回；16 `TestUpgradeFreezeReachesEveryPath` 的组织；17 `TestUpgradeFreezeLeavesTheRest` 另核对读页与字节不变的写；18 `TestUpgradeFreezeAnswersFirst` 另核对接管；19 `TestUpgradeFreezeLetsOnlyItsOwnWrites` 跑两种状态；20 成本守卫在适配器一层、界取单元的 2 秒；21 夹具的 `upgradeIn` 与 `endUpgrade`；22 `events.yaml` 保留后一句；23 第 68 条变异改在 `unit.go`；24 M10 的 lint 迁移的成本夹具先退 56、55 | 三位在各自的范围里判过：A 判 6、7、8、13、14，都对（8：后台撤销的开始多一次主键读，`revertChecks.check` 不改，正是设计 3.4 的做法，M10 已有的码之间的相对次序不变，唯一的变化是"判定之后笔记本被删"的竞态里 `notebook.not_found` 提前答，更合 00 号文档 4.16；13 有用，抓到 A2、A4；14 让 `TestTheBackgroundStartsRefusals` 抓到 A3）。B 判 1、2、3、4、24，都对（2：`downTo` 退掉给定的那一版，设计的 55、54 错了；3：设计的"id 有、另三列空"同时违反三个合取；24 的改法对，写死的形状见 S1B-M1），20 照了设计的表、表本身错了（S1B-I1）。C 判 10（对，即 S1C-M1）、12、13、16–19、21（都对）、20（S1C-M2）、23（S1C-N7）、24（S1C-N5）。其余（5、9、11、15、22）三位没有异议 |
| 留下的事 | — | 实施报告第 8 节留下的六项：1 Step 的文档提交；2 第 14 条变异的 `TestUpgradeGuardrails` 一半看不到被标的节点；3 "任务的批"里任务是发起人开的（不是 MCP，冻结照样拒，变异 9b 也在那里失败），要它只盖"按后台放行"可以改由 bob 开；4 本机计时的守卫在高负载下超界；5 成本守卫的界；6 S2、S3 每加一个迁移要改 lint 迁移的夹具 | 1 是本提交；2 决定 237；3 不改：两种都抓得到（第 68 条与 9b 各自失败）；4 S1C-N6；5 决定 236；6 决定 238 |
| 修复一轮的偏离与发现 | — | 实施者在修复一轮的六条说明（`fix-1-report.md` 第 7 节）：1 page 的模块根新导出三个状态常量（S1C-N3 要的常量原本不存在）；2 `FreezeGuard` 加测试的缝（字段 `admits` 与 `WithAdmits`），不加就造不出不认识的判定；3 计划在读的那条连接上以 `PREPARE` 与 `EXPLAIN EXECUTE` 取，不用 `planTreeWith`、`genericPlanTree`：带实参的 `EXPLAIN` 不管 `plan_cache_mode`，总是自定义计划；第四种状态的统计同第三种，它的两种计划与第三种相同，照修复说明仍各取一次；4 提交 1 也改了 `Makefile:346` 的说明，范围仍写 `server`（同第 1 个提交把 `Makefile` 算在 server 一侧）；5 06-environment 的那一句放在第 3 节；6 一次失误：第一次跑 `heavy.sh make test-go-costs-lint` 时在同一个工作树里接着改提交 3、4 的文件，后几行编进了半成品、build 失败，那次作废，以干净的工作树里的那次为准。另：头上的第一次 `make test` 在 guards-core 的 `TestTheAppsMarkdownCostsAboutItsSize/to_a_page` 以 0.02 ms 超界（"a block formula left open"，9.06 ms 对 `8 × 1.00 ms + 1 ms = 9.04 ms`）：`platform/markdown` 最后改在 M10/P4/S2 的 `6d1549fc`，S1 与修复都没有碰，单独重跑通过（4.8 倍），第二次 `make test` 全绿 | 修复核对判过：A 认为 `admits` 的缝只为一个测试、代价可以忽略；新导出的 `UpgradeUpgrading`、`UpgradeReviewing` 暂时没有用处，导出的常量 `unused` 不报，留给 S2、S3（A、B 同）；C 核对提交 1 的范围写 `server`、提交 4 写 `server, docs`，与改的文件一致，并同意 markdown 的守卫超界的判断 |

## 主持者的决定

负责人可以改判（接着 [P1 设计审查](P1-design-review.md)的"主持者的决定"第 233–235 条编号；236–238 在修复说明 `fix-1.md` 里定下，236 与 238 已写进代码与 `Makefile` 的注释，所以修复一轮的处置排在最后）：

236. **删除护栏的读的成本守卫以计划为主、时长为辅**（S1B-I1 = S1C-M2，2026-11-09）：四种统计状态 × 自定义与通用两种计划都走主键、两列都进索引条件、没有顺序扫描，主键的列钉成 `(upgrade_id, node_id)`；时长的界取本机两轮中位数较大的乘 3、向上取整到 5 ms，即 25 ms。设计 7.1 的表里这一行的"单元 2 s"与同节"界取本机两轮的中位数的三倍"的总规则不一致，以总规则为准；00 号文档第 9 节的同一行随之改（[01-P1](../01-P1-freeze-start-rollback.md) 7.1，7.3 第 87–89 条）。
237. **被标的节点只在 page 之内看得到**（S1A-Q1 = S1C-M1）：删除的护栏把拒绝标在子树里第一个受保护的节点，由 `TestTheFreezeGuard` 钉住；MCP 的工具错误只带 `code`、`message` 与 `op`，后台任务到不了护栏，整个程序里看不到它。S1 不为它改 MCP 工具错误的形状；7.3 第 14 条只列 `TestTheFreezeGuard`，P1 设计 3.4 与 00 号文档 4.6 照实改。
238. **M10 的 lint 迁移的成本夹具不写死最新的迁移**（S1B-M1、S1C-N5）：从嵌入的最新版本逐个经真的 Down 退到 54，再照旧退 54、53；以后的迁移不用改它，P1 设计 6.2、6.3 不加"改这张表"的一行。
239. **修复一轮的处置**（`fix-1.md`，2026-11-09）：三份审查全部接受，一组一个提交、各自绿、接在 `a28064e9` 之后、不改写已推的历史：S1B-I1 = S1C-M2 加计划的断言与新的界（决定 236）；S1B-M1 与 S1C-N5 改夹具（决定 238）；S1A-M1 与 S1A-N1 钉住 transfer 的开始的次序与两处新分支；S1A-N2 加 `default`（这一轮唯一的生产的改动），S1A-N3、N4 改注释，S1C-N2 改 `Makefile` 的数，S1C-N3 改期望，S1C-N4 与 S1C-N6 改 `06-environment.md`。只改文档的留给 Step 的文档提交：S1A-Q1 = S1C-M1（决定 237）；S1B-N1 交给 S2、S1C-N1 交给 S3，各记在 P1 设计里；S1B-N2 接受；S1C-N2 的记录用竞态的口径；S1C-N7 与成本的变异补进 7.3。修好之后重跑实施的 37 条与审查者留下的变异（A 的 A1、A5、A7，B 与 C 的 c1、c2）；A 的 W6 与实施的 14b 照旧存活（决定 237），C 的 o2 照旧存活（S3 补）。

## 接受与推后的

- **接受**：
  - S1B-N2：`NotebookUpgrade` 的说明在 S2 合并之前引用 `getUpgrade`。
  - 照预料存活的变异：实施的 14b 与 A 的 W6（决定 237）；C 的 o2（S1C-N1，交给 S3）。
  - A 的交错探针不进仓库：它只证明今天没有窗口；P1 在模块层由 `TestARestoreStarts`、`TestARevertStarts` 的调用序列抓到，P2 由 S2 的交错第 1 条盖，P3 是实施的第 18 条变异。
  - `startBackground` 在锁之前先判 `room`（503），所以"预检之后升级才开始、同时队列满"答 503 而不是 409：只在竞态里，不改 M10 的次序（A 核对过）。
  - B 的未核实的怀疑：10 万个 id（一半是基线的）对 20 万行自定义 37 ms、通用 56 ms，都走主键，更大的子树没量；`upgrade_nodes` 没有钉"一次升级至多一行保护行"与各行的 `notebook_id` 一致，设计没有要求，S2 的 `TestUpgradeStart` 断言补写的 `raw` 恰有一行保护行。
  - C 的未核实的怀疑：`upgradeBetweenBatches` 按调用数数，导入的一批若因 `errBusy` 重试也算一次，测试会失败；一批一个小页的导入里没见到，CI 与本机至今没见过。第四份建成时 422 秒离 450 秒 28 秒，最近 6 次 `main` 的第四份 294–414 秒；S2 合并之后的两次 `main` 要看它。
  - B 在修复核对里的观察：在"分析过、自定义计划"一格，位图读与并行的顺序扫描的估计成本只差约 9%（第一轮在同样的 20 万行、1 万个 id 上量到 7203 对 7861）。PostgreSQL 的代价模型一变，这一格可能翻成顺序扫描，测试会以"scans upgrade_nodes whole"失败。主持者接受：这正是决定 236 要的信号（计划为主），不是偶然的失败；若 CI 上因此不稳再议。
  - 计时的守卫在自造的负载下超界（S1C-N6）是已有的，不是 S1 的；修复一轮的 `TestTheAppsMarkdownCostsAboutItsSize/to_a_page`（0.02 ms）同类。
  - 流程（照实写下）：
    - 实施者推送之前改写过两次分支的历史（都还没推过）：oxfmt 的四处并回第 3、4、6 个提交；lint 迁移的成本夹具的修正并回第 2 个。改写之前的头 `cd416624` 与 `a28064e9` 只差那一个 `!race` 的文件，门禁的大部分与变异在 `cd416624` 上跑。
    - 实施者第一次 `make test` 与逐个提交的核对同时跑（负载 24），两个计时的测试超界；之后的重的运行都单独跑。
    - A 的两组变异在同一个工作树里撞车，整组重跑（上表"审查方式"）；C 按 PID 停下自己的一次运行。
    - 修复一轮的一次失误：在跑 `costs-lint` 的工作树里改后面的提交，那次作废。
- **推后**：
  - S2：`States.Start` 断言升级的 id 非零（S1B-N1，P1 设计 3.14 第 10 步）；S2 加 `getUpgrade` 时核对 `NotebookUpgrade` 说明里的名字（S1B-N2）。
  - S3："任务的批"加 `rolling_back` 一行（S1C-N1，P1 设计 7.1、3.24、6.3）。
  - 设计第 8 节第 1 条的"合并之后两次 `main` 的平均"在合并之后读。
  - 删除护栏的读的成本守卫若在 CI 上因"分析过、自定义计划"一格的翻转而不稳，再议（决定 236）。
  - 没有交给 v0.1 之后的打磨的。

## 反向对照

建成时以 `mut.py` 跑（库的经 `heavy.sh`，每个之后由工具从备份复原，变异期间不跑 `make gen*`；改查询的两条改生成的 SQL 常量），在 `cd416624` 上（与头只差那个 `!race` 的文件），先在没改的树上把 15 条不同的命令各跑一次（`mut/baseline.out`），规格与输出在 `mut/spec.json`、`mut/mut.out`；修复一轮在 `68c3c505` 上重跑，先跑 20 条基线命令，规格与输出在 `fix1/all.json`、`fix1/all.out`，日志在 `fix1/mutlogs/`。这些都在 `scratchpad/m11/` 下；审查者的在 `scratchpad/m11/s1-review/` 下（A 的 `rev-a/`、B 的 `b-mut/`、C 的 `c/`）。设计列两个测试的各跑一次（a、b）。

**7.3 的 S1 各行**（建成时 37 次：36 次被点名的测试抓到，14b 没抓到、同一变异由 14a 抓到；修复一轮在 `68c3c505` 上重跑这 37 次，结果相同，第 11 条的旧文字随提交 4 的 `switch f := g.admits(…)` 改写、变异本身不变）：

| # | 变异（实际的改法） | 跑的测试 | 结果 |
|---|---|---|---|
| 1 | 00055 的检查去掉 `AND mode = 'standard'` | `TestChecksRejectCounterexamples`（an LLM Wiki upgrading） | 抓到 |
| 2 | 00056 的 `upgrade_nodes_page_check` 去掉 `accepted_hash` 的合取 | 同上（a baseline page without its accepted hash；另 a protected row with an accepted hash） | 抓到 |
| 3 | `runtime-grants.sql` 的 `MAINTAIN` 去掉 `upgrade_nodes` | `TestTheGrantsFileCoversEveryRelationAndFunction` | 抓到 |
| 4 | 生成的 `listNotebooks` 的四列换成 `NULL` | `TestTheNotebooksAnswerTheirUpgrade` | 抓到 |
| 5a | `toPageState` 不转换 `Upgrade` | `TestUpgradeFreezeReachesEveryPath`（三种状态：别人的写成功） | 抓到 |
| 5b | 同上 | `TestPageNotebooksConvertTheState`（另加） | 抓到 |
| 6 | 登记里没有冻结：守卫与否决者都是 `[mode, lock]` | `TestUpgradeFreezeReachesEveryPath` | 抓到 |
| 7 | 冻结排在锁之后：两处都是 `[mode, lock, freeze]` | `TestUpgradeFreezeAnswersFirst` | 抓到 |
| 8 | 否决者不拒 | `TestUpgradeFreezeReachesEveryPath`（开会话） | 抓到 |
| 9a | `FreezeAdmits` 不看 `IsMCP()` | `TestFreezeAdmits` | 抓到 |
| 9b | 同上 | `TestUpgradeFreezeReachesEveryPath`（发起人的 REST 写；"任务的批"也失败，那里的任务是发起人开的） | 抓到 |
| 10a | 去掉 `Grants.Ask` | `TestUpgradeFreezeFollowsTheInitiatorsRole` | 抓到 |
| 10b | 同上 | `TestTheFreezeGuard`（问一次） | 抓到 |
| 11 | 守卫开头按 `Kind == revert` 放行 | `TestUpgradeFreezeLetsOnlyItsOwnWrites`（两种状态） | 抓到 |
| 12a | `reviewing` 也放过发起人的 MCP 写 | `TestFreezeAdmits` | 抓到 |
| 12b | 同上 | `TestUpgradeFreezeReachesEveryPath`（`reviewing` 一列） | 抓到 |
| 13a | 去掉过期的 `UpgradeOwn` 的核对 | `TestFreezeAdmits` | 抓到 |
| 13b | 同上 | `TestTheFreezeLetsItsOwnUnits` | 抓到 |
| 14a | 删除的拒绝标在 `Changes[0]` | `TestTheFreezeGuard`（the deletion guardrail） | 抓到 |
| 14b | 同上 | `TestUpgradeGuardrails` | 没抓到，照预料（偏离 10、S1C-M1、决定 237）：MCP 的错误不带节点；同一变异由 14a 抓到，7.3 第 14 条改为只列 `TestTheFreezeGuard` |
| 15 | 生成的 `upgradeProtected` 加 `AND baseline`（不读保护行） | `TestUpgradeGuardrails`（补写的 `raw`） | 抓到 |
| 16 | 上传的预检不判冻结 | `TestUpgradeFreezeReachesEveryPath`（only the prechecks answer：答 `storage_full`） | 抓到 |
| 17a | `CheckRevert` 不判冻结 | 同上（答 `page.revert_too_large`） | 抓到 |
| 17b | `CheckRestore` 不判冻结 | 同上（答 `page.restore_expired`） | 抓到 |
| 17c | 导入的 `Check` 不判冻结 | 同上（答 `storage_full`） | 抓到 |
| 18 | 导入建任务的事务不判冻结 | `TestTheStartsRefuseAnUpgradingNotebook`（an import's Create；7.2 第 4 条的交错在 S2） | 抓到 |
| 19 | 冻结的判断放进 `StartDeps.lock` | `TestUpgradeFreezeLeavesTheRest`（导出，三种状态） | 抓到 |
| 20 | `ensureChangeset` 不记 `upgrade_id` | `TestTheChangesetsCarryTheirUpgrade` | 抓到 |
| 21 | 第三份的 `-skip` 不含第七份 | 不跑（设计写明不列），以 `go test -list` 核对：第三份的 `-run` 模式单独收下 7 个 `TestUpgrade…`，`-skip` 里的 SEVENTH 是要的（下面"组合根的各份"） | 核对 |
| 64 | `baseline = (name IS NOT NULL)` 改成单向 | `TestChecksRejectCounterexamples`（a protected row with a name） | 抓到 |
| 65 | `(name IS NULL) = (sort_order IS NULL)` 改成单向 | 同上（an order without its name） | 抓到 |
| 66 | 00055 的 `(upgrade_id IS NULL) = (upgrade_by_id IS NULL)` 改成单向 | 同上（an initiator alone） | 抓到 |
| 68 | 后台任务的单元放过冻结：守卫看不到 `Background`，改在 `unit.go`，`spec.Background` 的单元的 `Write.Notebook` 去掉升级 | `TestUpgradeFreezeReachesEveryPath`（the jobs' batches） | 抓到 |
| 80 | 00055 的 `(upgrade_id IS NULL) = (upgrade_at IS NULL)` 改成单向 | `TestChecksRejectCounterexamples`（a start alone） | 抓到 |
| 81a–d | `upgrade_nodes_page_check` 的 `(revision IS NULL) = (x IS NULL)` 各改成单向 | 同上（a protected row with a hash、a size、a wanted、an accepted hash，各杀一个） | 抓到 |

`TestChecksRejectCounterexamples` 不在每个反例之后回滚：放过的反例改了行，后面的反例随之失败；第 2、64、66、80 条第一个失败的都是点名的那个。

**修复一轮的变异**（`fix1/`；连同审查者的 o1、o4、o7、o9 写成 [01-P1](../01-P1-freeze-start-rollback.md) 7.3 的第 83–94 条。修复一轮一共 49 条：实施的 37 条与下表 12 条，46 条被点名的测试抓到，3 条照预料存活）：

| # | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| B 的 c1（87） | 生成的 `upgradeProtected` 写成 `(upgrade_id = $1) IS TRUE AND (node_id = ANY(…)) IS TRUE` | `TestUpgradeProtectedReadsInTimeAtTheUnitsBound` | 抓到（之前存活）：四种状态、两种计划都是对整张表的并行顺序扫描；时长只在通用计划超界（前三种 19 ms、在 25 ms 之内，通用 564 ms） |
| B 的 c2（88） | 00056 去掉 `upgrade_nodes_pkey` | 同上 | 抓到（之前存活）：`pg_get_indexdef` 找不到主键 |
| C 的 c1（87） | `(node_id::text)::uuid = ANY(…)`（只剩 `upgrade_id` 走键） | 同上 | 抓到（之前存活）：两种计划的索引条件只有 `upgrade_id`，分析过之后是顺序扫描；时长 46、40、73、572 ms 也超界 |
| C 的 c2（87） | 两列都转换 | 同上 | 抓到（之前存活）：四种状态都是顺序扫描；时长 42、41、42、588 ms |
| p1（89） | 00056 的主键列序换成 `(node_id, upgrade_id)` | 同上 | 抓到：只有 `pg_get_indexdef` 的钉住抓得到（计划仍走主键、两列都进条件） |
| A1（90） | 导入的 `Check` 把 `notUpgrading` 挪到 `place` 之后 | `TestTheStartsRefuseAnUpgradingNotebook` | 抓到（之前存活）：两行都答 `page.not_found`，调用序列少了 `StateOf` |
| A5（91） | transfer 的 `notUpgrading` 没有笔记本时答 nil | 同上 | 抓到（之前存活）："…the notebook deleted as its state is read"答 `page.not_found` |
| A7（92） | `lock` 在判定之前答 `notebook.upgrading`（导出除外） | 同上 | 抓到（之前存活）：恢复、撤销的"by a reader"都答 `notebook.upgrading` |
| n2（93） | 去掉 `default` 的拒绝（落到 `Ask`） | `TestTheFreezeGuard` | 抓到："a freeze it does not know"答 nil、问了 `notebook.change_mode`，单元写了 1 个节点、1 个变更集 |
| n3（94） | page 的 `UpgradeRollingBack` 指向 `reviewing` | `TestPageNotebooksConvertTheState` | 抓到 |
| A 的 W6 | 护栏的拒绝不自己标节点（由 `refusalOf` 标 `Changes[0]`） | `^TestUpgrade`（第七份） | 存活，照预料（= 14b，决定 237） |
| C 的 o2 | `reviewing` 放过后台任务的单元 | `^TestUpgrade`（竞态） | 存活，照预料（S1C-N1，S3 补） |

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，照预料的两条除外）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | A1（导入的 `Check` 把 `notUpgrading` 挪到 `place` 之后） | 存活：S1A-M1 |
| A | A2（上传的 `Check` 把 `notUpgrading` 挪到 `assetPlace` 之后）、A4（page 的 `notUpgrading` 没有笔记本时答 nil） | 抓到（`TestThePrechecksRefuseAnUpgradingNotebook`、`TestAnUploadsCheckRefusesAReservedTitle`；`…/gone_after_placement`） |
| A | A3（`startBackground` 不判冻结） | 抓到（`TestTheBackgroundStartsRefusals`） |
| A | A5（transfer 的 `notUpgrading` 没有笔记本时答 nil）、A7（transfer 的 `lock` 在判定之前答 `notebook.upgrading`） | 存活：S1A-N1 |
| A | A6（否决者只在 `upgrading` 拒） | 抓到（`TestTheFreezeGuard/the_vetoer`） |
| A | W1（任何人的 MCP 写都 Ask） | 抓到（第七份里只有 `TestUpgradeToolsErrorsCarryTheirCodes`；`TestFreezeAdmits` 另有一行） |
| A | W2（`ensureChangeset` 记 `UpgradeOwn`）、W3（`toPageState` 不转 `By`）、W4、W5（生成的 `ShareNotebook`、`LockNotebook` 的四列换成 `NULL`） | 抓到 |
| A | W6（护栏的拒绝不自己标节点） | 存活，照预料：S1A-Q1（同 14b） |
| A | P1（`startBackground` 用锁之前读的状态）、P2（单元用事务之前读的状态）、P3（导入建任务时不判） | 交错探针都抓到；P1 在模块层由 `TestARestoreStarts`、`TestARevertStarts` 的调用序列抓到 |
| B | c1、c2（成本守卫：用不上索引的写法、去掉主键） | 存活：S1B-I1 |
| B | a1–a13、a17（00055、00056 的每个合取的另一个方向或另一个合取） | 抓到（各被点名的反例） |
| B | a14（`changesets_upgrade_id_idx` 不是部分索引）、a15（`ON DELETE SET NULL`）、a16（00055 的 Down 漏一列） | 抓到（`TestConstraintAndIndexNames`、`TestTheUpgradeColumnsGoDown`） |
| B | b1–b5（生成的五条查询的四列换成 `NULL`）、b6、b7（`upgradeOf` 的状态、时刻） | 抓到 |
| B | d1（handler 的 `started_by`）、e1（`CreateChangeset` 不写 `upgrade_id`）、f1（`toTransferState` 的 `RollingBack`）、f2（`mcpOwnCodes()` 去掉 `page.upgrade_protected`）、g1、r1（`analyzed` 与数据授权去掉 `upgrade_nodes`）、r2（打包的 `deleteNode` 去掉 `notebook.upgrading`）、h1、i1、i2（`upgradeToolCodes()` 少一对、多一对，`batch` 的行少一个码） | 抓到 |
| C | r5–r7、r9b、r11、r13b、r15、r16、r18、r19、r68（7.3 的行重跑；r68 是 C 的写法：取当前升级为 `UpgradeOwn`） | 抓到 |
| C | o1（`batch` 的单元跳过冻结）、o3a、o3b（预检先判冻结、后判定）、o4a、o4b（护栏只问子树的根）、o5（没有升级的变更集写零 UUID）、o6（否决者只拒 `upgrading`）、o7（任何 MCP 写都 Ask）、o9（`toTransferState` 不转换 `UpgradeID`）、o10（护栏以 `s.UpgradeOwn` 读保护行）、o11（读不认 `upgrade_id`） | 抓到（o3b 只由 `TestAnImportStarts` 的调用序列；o7 只由 `TestUpgradeToolsErrorsCarryTheirCodes`） |
| C | o2（`reviewing` 放过后台任务的单元） | 存活：S1C-N1 |
| C | c1、c2（成本守卫：`node_id` 不走键、两列都不走键） | 存活：S1C-M2 |
| C | w1（`problem-messages.ts` 去掉 `notebook.upgrading`） | 抓到：恰 14 个操作失败 |

A 16 个（A1–A7、W1–W6、P1–P3；模块层的 P2 与 W2 撞车、作废），B 36 个（34 个抓到），C 26 个（23 个抓到）。

**修复核对一轮**（在 `68c3c505` 上；A 的规格与输出 `rev-a/spec5.json`、`mut5.out`，B 的在 `b-check1/`，C 的在 `c1/`）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | A1、A5、A7（存活过的） | 都抓到 |
| A | F1（守卫去掉 `default`）、F2（`default` 答 nil）、F3（`default` 答 `ErrUpgrading`，一个 problem） | 都抓到（"a freeze it does not know"） |
| A | F4（构造器的 `admits` 恒答 Open） | 抓到（"the initiator's web write"等已有的子测试） |
| A | F5（替身的 `goneAsRead` 不起作用） | 抓到 |
| B | c1、c2（存活过的）；c3（只把 `node_id = ANY(…)` 包进 `IS TRUE`） | 都抓到（c2 由 `pg_get_indexdef`） |
| B | q1、q2（取计划时关掉位图；另关 Index Only Scan） | 失败：分析过、自定义计划一格改选并行的顺序扫描，计划真的变了（"接受与推后的"） |
| B | q3（只留位图）、q4（`debug_parallel_query = on`，套一层 Gather） | 通过：断言接受 Bitmap Heap Scan 加 Bitmap Index Scan 与 Gather 下的扫描，不误报 |
| B | 临时的空 00057 | `TestLintSourcesMigrationCostsEmptySources`、`TestLintMigrationFixturesKeepVersionsAndLinkFacts` 通过，夹具从 57 退起 |
| C | c1、c2（存活过的）；B 的 c1、p1 | 都抓到（B 的 c1 前两种状态的时长在界内，只由计划抓到） |
| C | A1、A5、A7、n2 | 都抓到 |
| C | k5（page 的领域里 `"rolling_back"` 拼错） | 抓到（竞态下的 `TestPageNotebooksConvertTheState`） |
| C | r5–r7、r9b、r11、r13b、r15、r16、r18、r19、r68；o1、o3a、o3b、o4a、o4b、o5、o6、o7、o9、o10、o11 | 都抓到，失败的子测试与第一轮相同（o3b 另被"an import's Check"的完整调用序列抓到） |
| C | o2 | 存活，照预料（S3 补） |

A 8 个都抓到；B 3 个被杀、q1–q4 是看形状的核对；C 32 个，31 个抓到。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `68c3c505`（修复一轮 `f66537d4`–`68c3c505`，4 个提交），三位各在自己的工作树里 | A：自己的 S1A-M1、N1–N4、Q2 都关了，S1A-Q1 代码侧关、文档照决定 237 在本提交改；A1、A5、A7 被抓到，新的 F1–F5 都被抓到（F5 证明 `goneAsRead` 不是摆设，F1–F3 钉住 `default` 的三种写坏，F4 钉住构造器）；`goneAsRead` 只在那一个子测试里设、设完清掉；生产上 `FreezeGuard` 只有 `NewFreezeGuard` 一处构造。B：S1B-I1、M1 都关了，N1、N2 照处置；c1–c3 被杀，q3、q4 照过（不误报），q1、q2 是计划真的变了；空的 00057 下两个 lint 迁移的测试都通过；另两个提交没有问题。C：S1C-M2、N2–N6 关了，M1、N1、N7 的处置对、文档的部分在本提交、N1 交给 S3；32 个变异 31 个被点名的测试抓到、o2 照预料存活；4 个提交各自 build、vet、`make lint-go`（0 个问题）、`make gen-check`、short 都通过，提交 1、4 另过 `make lint`，提交 2 另过 `TestLintSourcesMigrationCostsEmptySources`，提交 3 另过竞态下的 `transfer/...`，提交 4 另过竞态下的 `page/app`、`page/domain` 与组合根的两个转换测试和 `^TestUpgrade`，每个之后工作树干净；实施者报告的变异表、CI 与中位数与它读到的逐项一致。三位：Important 0、Minor 0、Nit 0 | 合并（见 00 号文档第 12 节） |

## 实测

- **新的测试在竞态检测下**（建成时，`go test -race -count=1 -v`，逐包，经 `heavy.sh`，单独跑）：第七份的七个共 13.0 秒（`TestUpgradeFreezeReachesEveryPath` 5.56、`TestUpgradeFreezeLeavesTheRest` 2.53、`TestUpgradeFreezeFollowsTheInitiatorsRole` 1.37、`TestUpgradeFreezeLetsOnlyItsOwnWrites` 0.95、`TestUpgradeGuardrails` 0.91、`TestUpgradeToolsErrorsCarryTheirCodes` 0.90、`TestUpgradeFreezeAnswersFirst` 0.80），包 14.2 秒；`TestTransferNotebooksConvertTheState`、`TestPageNotebooksConvertTheState` 各 0.00 秒。`migrations`：`TestChecksRejectCounterexamples` 5.53 秒（改了）、`TestConstraintAndIndexNames` 1.41 秒（改了）、`TestTheUpgradeColumnsGoDown` 0.06 秒。`TestTheStatesAnswerTheirUpgrade`、`TestTheNotebooksAnswerTheirUpgrade`、`TestUpgradeProtected` 各约 1.4 秒（含起容器），`TestAnUpgradingNotebookIsAnsweredWithItsUpgrade` 0.62 秒，`TestCreateChangesetWritesItsUpgrade` 0.04 秒，`TestFreezeAdmits` 与 `page/app`、`transfer/app` 的新测试各 0.00 秒。
- **删除护栏的读的成本守卫**（不带竞态，五次读的中位数：没有统计、分析过、只有另一次升级的统计、通用计划）：
  - 建成时（界 2 秒）：单独跑 6.3、4.1、4.0、4.9 ms，准备 0.80 秒，整个测试 2.49 秒；`make test` 里 6.8、4.9、5.8、5.7 ms，2.70 秒。审查 B 原样 7.3、5.9、6.0、5.7 ms；审查 C 6.6、5.3、4.1、4.6 ms；CI 的 costs-2（运行 42491802403）8.6、6.1、6.0、6.2 ms。
  - B 在真库上的 `EXPLAIN ANALYZE`：每种状态都是 `upgrade_nodes_pkey` 上的 Bitmap Index Scan 或 Index Only Scan，索引条件含两列，4–7 ms；10 万个 id 时 37–56 ms；强制顺序扫描 20 万行 15 ms、102 万行 40 ms。
  - 修复一轮之后（加了计划的断言，界 25 ms）：本机两轮 7.5、6.1、6.3、5.1 ms 与 6.5、5.8、5.3、6.4 ms（负载约 0.3）；两次 `make test` 里 6.8、6.3、6.0、6.0 与 6.6、6.4、6.5、5.3 ms；CI 的 costs-2（运行 42512020813）5.9、6.2、5.7、6.5 ms，整个测试 5.93 秒（种子 1.05 秒）。修复核对 B 原样 5.8、5.6、6.0、4.9 ms；修复核对 C 本机 7.5、6.0、5.8、5.6 ms 与 7.0、6.1、4.5、5.1 ms（负载 3.2、3.0），在 `f66537d4` 上单独 5.8、5.5、5.5、4.9 ms（负载 5.0）。
- **e2e**（不带竞态）：建成时 `make e2e` 503 个通过，148 秒（Playwright 2.4 分钟，`cd416624`）；审查 C 在 `a28064e9` 上 503 个（2.3 分钟）；修复一轮 503 个（2.6 分钟）。S1 不加 e2e（设计第 0 节与第 5 节）。

## 组合根的各份

照 3.8 以竞态构建列出（`go test -race -list`，各份照 `Makefile` 的 `-run`、`-skip`）：534 个顶层测试，七份 158、51、121、123、62、12、7，每个恰在一份；`TestUpgrade…` 的 7 个（`TestUpgradeFreezeAnswersFirst`、`TestUpgradeFreezeFollowsTheInitiatorsRole`、`TestUpgradeFreezeLeavesTheRest`、`TestUpgradeFreezeLetsOnlyItsOwnWrites`、`TestUpgradeFreezeReachesEveryPath`、`TestUpgradeGuardrails`、`TestUpgradeToolsErrorsCarryTheirCodes`）只在第七份；第三份的 `-run` 模式单独收下全部 7 个，所以它的 `-skip` 里的 SEVENTH 是要的（7.3 第 21 条）。另有 42 个 `//go:build !race` 的成本与守卫测试不进竞态构建，在 costs、guards 的行里跑：实施报告的 576 个、167/58/124/136/62/22/7 是不带竞态的清单（审查 C 的 S1C-N2，审查 C 在 `a28064e9` 上以两种清单核对，实施者修复一轮在 `68c3c505` 上重数竞态的 534 个）。S1 碰到的别的组合根的测试：新的 `TestTransferNotebooksConvertTheState` 在第三份；改了的 `TestPageNotebooksConvertTheState` 在第四份、`TestTheMCPToolsErrorsCarryTheirCodes`（减去 `upgradeToolCodes()`）在第五份、`TestTheGrantsFileCoversEveryRelationAndFunction` 在第三份。修复一轮不加顶层测试，改的只有第四份的一个期望、第七份的一处注释与两个 `!race` 的测试，所以各份不重量。

本机各份单独、竞态下，经 `heavy.sh`（建成时，`make -j1 test-go-bootstrap-N` 依次两轮，包的时间，秒）：

| 份 | 测试数（竞态构建） | 第一次 | 第二次 | 平均 | 开始时负载 |
|---|---|---|---|---|---|
| 第一 | 158 | 298.8 | 290.8 | 294.8 | 3.59、1.27 |
| 第二 | 51 | 279.5 | 283.2 | 281.4 | 0.89、1.40 |
| 第三 | 121 | 266.2 | 278.9 | 272.6 | 1.36、1.40 |
| 第四 | 123 | 220.2 | 227.4 | 223.8 | 1.33、1.07 |
| 第五 | 62 | 259.1 | 259.9 | 259.5 | 1.95、1.74 |
| 第六 | 12 | 264.9 | 261.4 | 263.2 | 1.47、1.45 |
| 第七 | 7 | 14.5 | 14.7 | 14.6 | 1.48、1.28 |

- 14 次都通过。第七份单独约 15 秒（七个测试本身 13 秒）。S1 在第三份只加了 `TestTransferNotebooksConvertTheState`（0 秒），它的模式收得下的 7 个 `TestUpgrade…` 由 `-skip` 交给第七份。
- CI 上七份的测试一步（下面"门禁与 CI"）：建成 4:42、5:49、4:45、7:02、6:25、4:10、0:42，修复一轮 6:45、4:28、6:54、7:01、6:25、4:48、0:57，都在 450 秒（7:30）之下；两次的平均 5:43.5、5:08.5、5:49.5、7:01.5、6:25、4:29、0:49.5。最紧的是第四份（422、421 秒），S1 在那里只改了一个 0 秒的测试，最近 6 次 `main` 的第四份是 299、414、294、412、300、355 秒（审查 C）。修复一轮的第一份（4:42 → 6:45）与第三份（4:45 → 6:54）这一轮没有改它们的测试，读作运行机的差别。这是分支的两次运行；设计第 8 节第 1 条要的是合并之后两次 `main` 的平均，合并之后读。没有挪份，没有提高 `-timeout`。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`，机器同时有别的会话在跑测试）：

- **逐个提交**（建成时，实施者的脚本，一个干净的临时工作树、`pnpm install --frozen-lockfile`）：7 个提交各自 `go build ./...`、`go vet ./...`、`make lint-go`（全部 Go 的 golangci-lint、配置校验、`go mod tidy -diff`）、`make gen-check`、`make test-go-short` 都通过，之后工作树干净；碰 `api/` 或 `web/` 的第 3、4、7 个另过 `make lint-web`、`make knip`、`make test-web`（vitest 189 个文件；第 3 个 4,087 个测试，第 4、7 个 4,088 个）。第 6 个的 `make lint-web`、`make knip` 通过，`make test-web` 第一次在负载约 24 下有两个计时的测试超界（20 万字符的段落解析 1,038、1,022 ms，界 1,000 ms；第 6 个没有改 `web/`），改写之后在新的工作树里单独重跑通过（189 个文件、4,088 个测试，负载 7）。第二次改写只改了第 2 个提交的一个 `!race` 的测试文件，第 2 个另在新的工作树里重跑 build、vet、`make lint-go` 与 short。审查 C 另在第 2、4、6 个提交上跑同样的五步（lint 0 个问题，short 93 个包）与第 4、6 个的网页三步，都通过；第 2–6 个提交在竞态下跑 S1 碰到的模块的数据库测试（76–84 秒）与组合根里 S1 加或改的测试（45–46 秒），都通过。
- **建成时的头**（表里注明头：`cd416624` 与 `a28064e9` 只差那个 `!race` 的成本测试文件）：`make lint gen-check knip build`（`cd416624`）16 秒通过；`make test` 第一次（`cd416624`）821 秒、在第五份的两个计时的测试失败（`TestTheMCPBatchIsRefusedWhole/the_plan's_parses_past_10_MiB` 12.08 秒，界 12 秒；`TestTheMCPCallStopsWhenTheClientGoes` 2 秒之内没等到 debug 的一行），当时逐个提交的核对与它同时跑、负载到 24，两个测试单独重跑通过（58 秒，批在 5.07 秒被拒）；`make test` 第二次（单独跑，`cd416624`）1,942 秒，组合根之外的包、组合根七份与成本三份通过（含新的成本守卫），`test-go-costs-lint` 的 `TestLintSourcesMigrationCostsEmptySources` 失败（"legacy migration setup version=56, want 54 before Down"，偏离 24，修正并回第 2 个提交）；之后 `make -j1 test-go-costs-lint test-go-guards test-web`（`a28064e9`）1,547 秒通过（`costs-lint` 14 条、守卫 26 条、vitest 189 个文件 4,088 个测试，开始时负载 0.3）；`make test-go-short`（`cd416624`）46 秒；`make e2e`（`cd416624`）503 个 148 秒。修正之后另在头上跑 `go vet ./internal/bootstrap/` 与 `golangci-lint run ./internal/bootstrap/...`（0 个问题）。
- **修复一轮**（`68c3c505`）：4 个提交各自在干净的临时工作树里（整个经 `heavy.sh`，不与 `make test` 同时跑）build、vet、`make lint-go`、`make gen-check` 与 short 都通过；碰 `Makefile` 的提交 1、4 另过 `make lint`；提交 2 另过 `make test-go-costs-lint` 与加一个空的 00057 之后的 `TestLintSourcesMigrationCostsEmptySources`；每个之后工作树干净。头上 `make lint`、`make gen-check`、`make knip`、`make build` 通过；经 `heavy.sh`：`make test-go-short` 通过，`make e2e` 503 个（2.6 分钟）；`make test` 第一次只在 guards-core 的 `TestTheAppsMarkdownCostsAboutItsSize/to_a_page` 失败（0.02 ms，上表"修复一轮的偏离与发现"），单独重跑通过，第二次（01:50–02:50）全绿，vitest 189 个文件、4,088 个测试；结束时工作树干净。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 作业 | 42491802403（建成，`a28064e9`） | 42512020813（修复一轮，`68c3c505`） |
|---|---|---|
| server (race) | 13:29（13:09） | 13:23（13:04） |
| server (bootstrap-1) | 5:00（4:42） | 7:02（6:45） |
| server (bootstrap-2) | 6:11（5:49） | 4:46（4:28） |
| server (bootstrap-3) | 5:04（4:45） | 7:18（6:54） |
| server (bootstrap-4) | 7:24（7:02） | 7:20（7:01） |
| server (bootstrap-5) | 6:48（6:25） | 6:46（6:25） |
| server (bootstrap-6) | 4:27（4:10） | 5:04（4:48） |
| server (bootstrap-7) | 1:02（0:42） | 1:21（0:57） |
| server (costs-1) | 17:00（13:38） | 17:12（13:40） |
| server (costs-2) | 16:12（15:51） | 12:42（12:23） |
| server (costs-3) | 10:23（10:01） | 8:00（7:39） |
| server (costs-lint) | 11:22（11:04） | 13:12（12:49） |
| server (guards-core) | 19:31（19:11） | 12:00（11:41） |
| server (guards-sources) | 16:57（16:33） | 17:24（17:02） |
| server (short) | 2:37（2:19） | 3:19（2:57） |
| web | 4:20（3:48） | 4:51（4:17） |
| e2e | 7:30（5:58） | 6:45（5:34） |
| image | 1:25 | 1:23 |

两次都全绿，18 项，都没有重跑：建成 14:50:50Z–15:17:56Z，修复一轮 18:50:50Z–19:15:03Z。组合根七份的测试一步都在 7:30 之下（上面"组合根的各份"）。建成时 costs-2 里新的成本守卫的中位数 8.6、6.1、6.0、6.2 ms（6.74 秒，审查 C 读的作业日志）；修复一轮 5.9、6.2、5.7、6.5 ms（5.93 秒）。
