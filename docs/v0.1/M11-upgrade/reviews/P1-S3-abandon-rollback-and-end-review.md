# M11/P1 S3（放弃、回滚与结束（服务端））：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p1-s3`（实施的 5 个提交：回滚的变更集与任务的种类（迁移 00058、00059、sqlc、两个模块的种类与 `Kinds()`、River 的种类与 worker、Rescue 的表、契约与网页的新种类、回滚报告的三个数与任务的标题）`19b2ae6f`，page 模块一遍一遍的回滚（`PlanRollback`、先删与 `PackFreeSubtrees`、合成的批、回滚的读、三个成本守卫）`cc015aec`，回滚的任务、它的结束与谁能取消它（`Rollbacks.Start`、按遍的循环、`rollbackSucceeded` 与 `rollbackStopped`、取消只给管理员、wiki 的 `RollbackEnd`、notebook 的 `SetState` 与 `Clear`）`776f957f`，`abandonUpgrade` 与 `endUpgradeRollback`（`getUpgrade` 现读的 `rollback_left`、`wiki.upgrade_state`、第七份的 `TestUpgradeRollback`、交错、流、冻结的批、矩阵四行）`47518c39`，e2e 的 UP3 `9b572167`；推送之前改写过三次 4、5 号：一次给夹具加一个链接，两次补变异第 37、41、43、72 条组合根的一半抓不到的测试；从 `main` 的 `e5059e55` 开始，审查时在 `9b572167` 上，125 个文件、加 8,947 行、减 287 行。修复一轮五个：S3A-M1 `63cd2572`，S3B-I1、S3B-M1 = S3A-N2 与 S3B-Q1 `923ad055`，S3A-M2 与 S3A-N1 `9605c0e4`，S3C-M1、S3C-M2、S3C-N4、S3B-N1 与 S3C-N2 `6d8ed474`，S3C-M3 `9c0a0f42`（31 个文件、加 794 行、减 357 行）；修复第二轮一个：S3A-N3 = S3B-N3 `de554eda`（只改两处注释与一个单元测试）；一共 11 个；合并 `（待填）`），对照 [01-P1-freeze-start-rollback.md](../01-P1-freeze-start-rollback.md) 第 0 节的 S3、2.1、3.16–3.24、4.3、第 5 节、6.3、6.5、第 7 节的 S3 的各行（7.3 的第 13 条的 S3 一半、37–53、60、61、69–73）与第 8 节，[S1](P1-S1-freeze-guardrails-review.md) 与 [S2 的审查记录](P1-S2-start-and-read-review.md)，[00-M11-design.md](../00-M11-design.md) 4.3、4.10、4.12、4.14、4.16–4.18、第 9 节与第 11 节（第 212、217、218、225–231、233、234、236、240 条），[v0.1 总体设计](../../v0.1-design.md) 第 13 节，实施者的报告（27 条偏离、逐个提交的核对、门禁、变异的表、实测、CI、留下的五项） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（S1、S2 时的三个工作树挪到 `9b572167`），用到 Docker 的都经 `heavy.sh`：服务端的正确性（A：page 的回滚 `rollback.go` 全文与领域的 `PackFreeSubtrees`、`RollbackState`，transfer 的回滚任务、Rescue 与取消，wiki 的放弃、结束回滚、`RollbackEnd` 与 `getUpgrade`，三个守卫与它们的计划核对；44 个包级的变异；三个临时探针：两次放弃同时到与删除、放弃各先一次，单元持着行时管理员的写答什么，回滚的结束落在 `getUpgrade` 的几次读之间；把守卫的计划核对原样抄出、喂合成的计划）；数据、迁移、契约与矩阵，计划与成本（B：两个迁移与设计 3.17 逐字比对，迁移的 11 个变异；读的七条语句在三种统计 × 自定义与通用计划下的计划，8 个 SQL 变体与一道更严的核对；收着者多时的 `RollbackFreeForest`；先删的单元为什么慢，三轮对照、轮询 autovacuum；矩阵的两个变异）；测试、变异、e2e、网页、逐个提交、CI 与量的数（C：先读设计与代码、形成判断之后才读实施报告；7.3 的 S3 各行 28 种与自己想的 21 种变异，三个探针，5 个提交各自的门禁，头上的 e2e，第一份逐个测试的时长，以 PostgreSQL 18.6 的临时容器演示行锁的插队）。修复核对一轮三位各在 `9c0a0f42` 上 |
| 日期 | 2026-11-09（实施与审查）至 2026-11-10（两轮修复与修复核对） |
| 结论 | Important 一条（S3B-I1）：`RollbackFreeForest` 的 `NOT IN (SELECT id FROM holders)` 在统计分析过之后是普通的 SubPlan，每个活节点都把 CTE 从头扫一遍，代价是活节点数 × 收着者数；守卫的种子扇出极大（每个文件夹一千页），收着者只有约 200 个，看不出；同为 10 万页、1 万个收着者时这一条读要 26.8 秒，改成 `NOT EXISTS` 是 317 毫秒。修复一轮改成 `NOT EXISTS`，守卫换成父节点多的形状，计划核对加两道（决定 244）。Minor 七条（S3A-N2 与 S3B-M1 是同一件事，按 Minor 处置）：先删的单元持着笔记本行 12–43 秒，长过 15 秒的请求期限（S3A-M1，决定 243）；`getUpgrade` 的三次读不在一个快照里（S3A-M2）；读的守卫不要求在内侧逐行重复的扫描经外行的键（S3B-M1 = S3A-N2）；收着者的递归只测了一层（S3C-M1）；结束时剩下的差只测了两类（S3C-M2）；第一份贴着 450 秒的线（S3C-M3，决定 245）；M9 的撤销测试被新来的事务插队（S3C-M4，不属于 S3）。Nit 七条，Q 一条（S3B-Q1，随 S3B-I1）；修复核对一轮另一条 Nit（S3A-N3 = S3B-N3），修复第二轮改。代码的次序与码三位都没有查出错：实施报告第 3 条（锁 → `FinishJob` → `End`）、第 7 条（逐行经键）、第 14 条 A 与 B 都核对过，照实施的；问题在两处成本、一处快照、一处日志的时机与"测试钉住了什么"。修复核对两轮：第一轮三位判所有要改代码的发现都已关闭，新的 S3A-N3 = S3B-N3 由第二轮处理；第二轮 B 判 S3B-N3 关、没有新发现 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：S3A-M1、M2、S3A-N1、N2，S3B-I1、S3B-M1、S3B-N1、N2、S3B-Q1，S3C-M1–M4、S3C-N1–N4；修复核对一轮的 S3A-N3、S3B-N3。修复一轮照主持者的修复说明（`s3-review/fix-1.md`）分五个提交，第二轮照 `fix-2.md` 一个提交。同一件事合为一个修复：S3A-N2 与 S3B-M1（按 Minor）；S3B-Q1 随 S3B-I1；S3A-N3 与 S3B-N3。下面的代码位置是 `9c0a0f42` 的，路径相对 `server/internal/`，迁移的测试相对 `server/`，网页写作 `W/`） | — |
| S3B-I1 | 重要（成本） | `RollbackFreeForest` 的 `n.id NOT IN (SELECT id FROM holders)`（`modules/page/adapter/postgres/queries/upgrade.sql:79`，照抄设计 3.18）：递归 CTE 的行数估计是"非递归项 + 10 × 递归项"，分析过统计之后 `holders` 估成 1,111 万行（实际 1 万），估的哈希表放不进 `hash_mem`，PostgreSQL 改用普通的 SubPlan，压在 `nodes` 的扫描上，这本笔记本的每个活节点都从头扫一遍 CTE。守卫的形状（100 个文件夹 × 1,000 页，收着者约 200）里普通 SubPlan 已出现在三格（分析过的两种计划、"只有别的升级"的通用），合成读最慢的正是这三格（B 的一轮 1,317–1,333 ms，另三格 819–846 ms）；同一份种子上这一句分析过时 735 ms，`NOT EXISTS` 245 ms。同为 10 万页、换成 1 万个文件夹各 10 页（1 万个收着者）、加 100 个新页、分析过：经 store 一次 26.8 s，`EXPLAIN ANALYZE` 69.6 s（CTE Scan 跑了 110,100 次、平均每次 9,546 行），`NOT EXISTS` 是 Hash Right Anti Join、317 ms；是合成读的界 4.6 s 的约 6 倍。`PlanRollback` 每一遍都读它，守卫过了却抓不到这个代价 | 已改（`923ad055`，提交 2）：改成 `AND NOT EXISTS (SELECT 1 FROM holders h WHERE h.id = n.id)`（`upgrade.sql:82`，注释 `:63-66`：`holders` 的两支都滤掉 `parent_id IS NULL`，语义相同）；回滚的别的读里没有 `NOT IN`。守卫计时的笔记本换成父节点多的形状：1 万个文件夹各是一页、下面 9 页，另 1 万个新文件夹收着三分之一的页（10 万行基线、1 万个收着者）；原来的"100 个文件夹各 999 页"作第二本 `ops`，六格都核对计划、不计时；种子改成一条语句同时建文件夹与页（照原写法按名字连接，1 万 × 1 万要 23 s），整个种子 14.7–14.8 s，计时的那本 2.4 s；计划核对另加两道（S3B-M1 一行）；界照决定 236 重定：`RollbackLeft` 1.2 s（3 × 376 ms）、合成的读 3.1 s（3 × 1,011 ms）。变异：改回 `NOT IN` 在"分析过"的计划核对就红，两种形状 × 两种计划都报 `a CTE Scan holders read whole for each row`（25.3 s）。修复核对 B：代码侧关；在守卫的种子上看了两种形状 × 三种统计 × 两种计划的 12 个计划，都是哈希反连接、CTE 只扫一次，`RollbackFreeForest` 每次 207–310 ms（父节点多的形状 254–310 ms）；g4（改回 `NOT IN`）24.9 s 在计划核对红；设计 3.18 的 SQL 在本提交改 |
| S3B-Q1 | 问题 | CI（7763）上合成读最大 2,968 ms，界 4.6 s，约 1.55 倍的余量，接受还是调；最慢的三格正是 S3B-I1 的普通 SubPlan，B 估修了之后本机最大约 0.85 s | 随 S3B-I1：本机合成读最慢一格 1.01 s（种子的形状也换了，收着者从约 200 个到 2 万个，树多了 1 万个节点），界照决定 236 重定为 3.1 s；CI（9V45）1,472–1,785 ms，约 1.74 倍；7763 上照约 2 倍估约 1.5 倍，还没有数。修复核对 B：关（问题已答）：余量回到"3 ÷ CI 与本机之比"是第 236 条的规则使然，合并之后若 7763 上这一格红，照负责人 2026-11-06 的话调界 |
| S3A-M1 | 低（成本与码） | 先删的单元持着笔记本行 `FOR NO KEY UPDATE`、工作区行 `FOR SHARE` 12–43 秒，长过 15 秒的 `server.request_timeout`；守卫只判中位数，界 39 秒（`modules/page/rollback_cost_test.go`）。一个单元删 1 万棵小子树、每棵一次 `Unit.Delete`，比恢复与撤销同样大小的单元慢 2–3 倍（决定 153 记的那两种在 7763 上约 11–12 秒）：本机 `units_ms` `[15681,28170,36203,28525,11858,…]`，CI（run 42635761979）`[26695,43014,39605,21296,21319,21413,21430,21408,21399,21220]`，十个都过 15 秒，43 秒的那个已过 39 秒、守卫照过。单元在跑时，这本笔记本上的管理写（继续、结束回滚、改名、访问、成员、删除笔记本）、页面写，同一工作区的成员与邀请的改动都要等，等过请求期限答 500 `internal_error`（`platform/httpserver/apierrors.go:199-201`、`:278`）。A 的探针 `TestRevAS3LongUnit`：另开事务持着工作区行与笔记本行的 `FOR SHARE`（比单元的弱），过测试配置的 4 秒期限，继续、结束回滚、`PATCH` 笔记本都在 4.00 秒答 500；放开之后继续与结束回滚答设计的 409 `transfer.busy`。决定 153 把恢复与撤销的单元线定在 14 秒，理由正是上面压着 15 秒的请求期限 | 已改（`63cd2572`，提交 1）：新常量 `MaxDeleteFirstNodes = 2_500`（`modules/page/domain/trash.go:82`，与 `MaxUnitNodes` 放在一起，注释写明为什么小）；`PackFreeSubtrees(forest, whole, pack)`（`modules/page/domain/rollback.go:70`）收两个上限，单独一棵大于 `pack`、不大于 `whole` 的子树独占一步，更大的照设计往下拆；`JobWrites.maxDeleteNodes`（`modules/page/app/jobs.go:116`）默认取它、不超过 `maxUnitNodes`；`PlanRollback` 装步（`modules/page/app/rollback.go:123`）与 `deleteBatch` 判多步的批（`:224`）用它；`TestPackFreeSubtrees` 加"a subtree past the pack, within the whole, a step alone"，`TestARollbackPassAtATime` 加"free subtrees past MaxDeleteFirstNodes"。守卫判每个单元即最慢的那个不过 14 秒（`page/rollback_cost_test.go:47`、`:177`），中位数只记进日志；一遍 330 s（3 × 109.3 s）、2,300 MiB（3 × 737 MiB）；种子在升级开始之前经 HTTP 删掉一个 5,000 页的文件夹（`:86`、`:93`），关掉六张表的 autovacuum 之后再 `ANALYZE`（`:113-116`），测稳态（选 B 的第一种做法：测的是"删过东西的笔记本"的常态，不在被测的一遍中间插手）。本机两轮 40 个单元 2,670–3,221 ms。必须做到的两条：上限改回 1 万时本机单元 10.8–11.1 s、守卫是绿的（CI 上照 1:2 估约 22 s 会红，没有为此改界）；去掉"统计见过已删的行"时单元一个比一个慢，3.0 s 到 28.1 s，一遍 645 s，守卫红。修复核对 A：关（稳态下 9V45 上最慢的单元 3.08 s，离请求期限约 5 倍）；X1、X1b 存活，成了 S3A-N3 |
| S3A-M2 | 低（读） | `getUpgrade` 依次 `Rows.Current`、`Writing.Writing`、`Rollback.Left`，各自一条语句、不在一个快照里（`modules/wiki/app/get_upgrade.go:44-61`）。回滚的结束（`rollbackSucceeded` 或管理员的结束回滚）在读到 `Current` 之后、读 `Left` 之前提交时，`Writing` 为假、基线行已删，`created` 等于笔记本里全部的活节点：答 200 `rolling_back`、`rollback_left: {nodes: 0, pages: 0, created: N}`，可这次升级已经回滚完了，下一次读答 404。A 的探针 `TestRevAS3GetUpgradeAcrossTheEnd` 用 `wiring.wiki` 包住 `Writing`、让读停在 `Current` 之后，放开回滚任务到 `succeeded` 再放开读：`created: 4`，正是全部的活节点 | 已改（`9605c0e4`，提交 3）：三次读放进一个 `Snapshots.WithinSnapshot`（`get_upgrade.go:48`，注释 `:23-27`），`UpgradeDeps` 加 `Snapshots`（`modules/wiki/app/start_upgrade.go`、`modules/wiki/lint.go:157`）；单元的替身记下"snapshot / end snapshot"，`TestGetUpgradeTellsWhatAStoppedRollbackLeft` 带上它们、另加 404 一格；组合根照 A 的探针加 "the read of an upgrade across its rollback's end"（`bootstrap/upgrade_rollback_interleavings_test.go:302`）：答 200 `rolling_back`、它的任务、`rollback_left` 为 null（快照里任务还在跑），之后 404。变异：去掉快照被两处都杀死，组合根杀的是真的行为。修复核对 A：关；快照是 `REPEATABLE READ READ ONLY`（`platform/postgres/tx.go:137-144`），三个适配器都经 `postgres.DB(ctx, pool)` 取 ctx 里的事务、都是不加锁的 `SELECT`；唯一的调用者是 HTTP 的 handler，碰不到 `ErrNestedSnapshot`；代价是多占一个连接到 `Left` 读完；G1、G1b 都抓到（G1b 下答的正是审查时的错答） |
| S3B-M1 = S3A-N2 | 低（测试） | 读的守卫的计划核对（`modules/page/adapter/postgres/rollback_cost_test.go:331-366` 的 `rollbackScanProblems`）对在内侧逐行重复的扫描只要求"带键"（`upgrade_id = `、`notebook_id = `，或 `nodes_pkey` 上的 `id = `），不要求比较外行的列：B 把三处 `LEFT JOIN LATERAL` 探针的 `b.node_id = n.id` 写成 `b.node_id::text = n.id::text`，`FreeForest`、`Left`、`Holders` 三个变体在"从未分析"的两种计划与"只有别的升级"的自定义计划下都过核对（内侧经 `upgrade_nodes_pkey` 的 `(b.upgrade_id = $1)`、`node_id` 掉进 Filter，每一行读这个升级的全部行），只在"分析过"变成 Seq Scan 才被抓；守卫先核对"从未分析"、再跑计时，变异会先被执行，等到 go test 的 10 分钟超时才红，报的是超时。A 以合成的计划核实同一处：内侧以 `nodes_notebook_id_*` 读这本笔记本、以 `upgrade_nodes_pkey` 读这个升级、同样经位图，三格都被接受。S2B-M2、S2A-N4 是同一类，这里没跟上 | 已改（`923ad055`，提交 2）：两道新核对：逐行重复的扫描（嵌套循环的内侧，或非 hashed 的 SubPlan 之下）索引条件不匹配 `rollbackJoined` 就报 "for each row, by no column of it"（`:425-426`）；处在逐行重复位置的 `CTE Scan`、`WorkTable Scan`、`Materialize` 一律报（`:358-360`）；PostgreSQL 17 起 hashed 与否只写在父节点的表达式里，`rollbackHashed`（`:383`）比 `(hashed SubPlan N)` 与 `(SubPlan N)`；`planNode` 加 `Subplan Name`、`CTE Name`（`modules/page/adapter/postgres/search_plans_test.go:172-173`）；一格计划有问题就在这一状态的计时之前停下（`:146`）；读"种子留下了什么"（会执行 `RollbackLeft`）挪到第一种状态的计划核对之后（`:151`）。`TestTheRollbackGuardJudgesThePlans` 加七格（`:503-516`，共 17 格）：A 的三格、B 的一格、非 hashed 的 SubPlan 逐行扫 CTE、内侧的 Materialize 应拒，hashed 的 SubPlan 应收。变异：三处丢键在"从未分析"的计划核对就红（15.9–17.2 s）；n5–n8 各被判据测试的格抓到。修复核对 A：关（头上的核对原样抄出，三格合成计划都被拒；判据测试 17 格全过）；B：关（g1–g3 16.0、15.9、15.7 s，S3 时要等 10 分钟；在一次性的 PostgreSQL 18.6 上看了 EXPLAIN 的写法，SubPlan 子节点不论 hashed 都叫 "SubPlan N"、挂在引用它的节点之下，`rollbackHashed` 只会误报、不会放过） |
| S3C-M1 | 低（测试） | 收着基线节点的新节点要一路往上算到根（设计 3.18"holders climb as many levels"），可 `TestARollbacksReads` 只有一层（Install 被移进新建的 Topics），组合根的 `Made/Guide`、UP3 的 `Hub/Guide`、守卫的 `h<k>` 也都只有一层，领域与 `page/app` 的测试不经这条 SQL：递归的那一步没有测试走到。C 的 c10（递归一步的 `WHERE` 改成 `false`）在 `TestARollbacksReads` 与 `TestUpgradeRollback` 下都照过；探针 `TestZZReviewDeepHolders`（Guide 在新建的 `A/B` 之下）在 c10 下把收着 Guide 的 A 当成自由的子树；组合根的探针在 c10 下第一遍就 `failed`、`forbidden`、`done 0`。后果：数据没丢，但这样的回滚每次继续都失败，网页对 `forbidden` 的文字（"开始回滚的人已不是管理员"）会误导；LLM Wiki 的升级通常正是把页往下挪几层 | 已改（`6d8ed474`，提交 4）：`TestARollbacksReads` 在 Topics 下加 `Sub`、Install 移到 `Topics/Sub`（`modules/page/adapter/postgres/rollback_test.go:58`），holders 断言是 {Sub, Topics}、`Folder` 仍自由（`:99`）；组合根选不加时长的做法：`rollbackNotebook` 写 `Made/Deep/Inner.md`、把 Guide 移到 `Made/Deep/Guide.md`（`bootstrap/upgrade_rollback_test.go:214-215`），各格的期望照实改（"停下后结束"与交错第 11 条剩 2 2 2，Deep 也算新的）。变异：c10 单元与组合根各杀一次。修复核对 C：关；另加 c10b（`RollbackHolders` 的递归只上一层），单元与组合根也各杀一次；`Made/Deep/` 之后各格的期望逐一推过，都对 |
| S3C-M2 | 低（测试） | `End` 在锁笔记本行之后再读一次差，三个数里有一个不为零就不结束（`modules/wiki/app/rollback_end.go:52-57`），单元测试只测了 `Pages: 1` 与 `Created: 1`：c11（不看节点）`TestTheRollbackEnd` 照过。实施报告说第 49 条组合根的一半"冻结之下程序里造不出"：对程序成立，对测试不成立，`rollbackStops.armDone` 本来就能停在计划答 Done 之后、结束之前，以 SQL 写一笔就造出来。C 的探针"left at the end"：头上任务 `failed`、`internal`、剩 `1 0 0`、继续之后复原；c11 与第 49 条（`if left && false`）之下任务成功、升级结束，改过名的页留在原处 | 已改（`6d8ed474`，提交 4）：`TestTheRollbackEnd` 加 `{"a node out of place left", Nodes: 1}`（`rollback_end_test.go:156`）；`TestUpgradeRollback` 加 "something left at the end"（`bootstrap/upgrade_rollback_test.go:510`，本机 0.47 s）：`armDone` 停住，以 SQL 把 Notes 改名，放行；任务 `failed`、`internal`，恰一行 ERROR 的 "rollback ended"，升级仍 `rolling_back`、剩 1 0 0，继续之后成功、笔记本与开始前相同。变异：c11 与第 49 条的组合根一半都被杀；第 49 条原来那一格 `a_pass_that_leaves_as_much` 照旧碰不到 `End` 的剩余。修复核对 C：关 |
| S3C-M3 | 低（CI 的余量） | 第一份的余量是拿包的时间比 450 秒的线：规则（06-environment 第 50 行）看的是测试一步。分支的 run 42635761979 里第一、二份的测试一步 446、443 秒，离线只差 4、7 秒；`main` 上第一份落在 7763 上是 442 秒，报告自己的估算（"约多 8–10 秒，仍在线下"）按它的算术已经到线或过线。分支这次在 9V74 上第一至三份比 `main` 的 9V74 慢 17–29%，同一个 run 的 race、short 与 `main` 一样快，本机第一份只多约 2%：C 推断是那几台 VM 慢，没有在 CI 上重跑对照 | 已改（`9c0a0f42`，提交 5）：`BOOTSTRAP_SEVENTH` 加 `ABackground` 一支（`Makefile:282`），第一份的目标加 `-skip '$(BOOTSTRAP_SEVENTH)'`，分份的注释与计数行照实改（`:265`）；`go test -race -list` 542 个顶层，七份 145、51、121、124、62、11、28，各在恰好一份，`^TestABackground` 13 个；本机第一份 231.0、230.3 s，第七份 168.9、171.1 s；CI 的 run 42671683550 第三次 attempt 里第一份在 7763 上测试一步 371 s（挪之前 `main` 上 442 s）、第七份在 9V45 上 211 s。决定 245。修复核对 C：关；本机一轮第一份 237.4 s、第七份 167.0 s；第七份在 7763 上照两种比例估约 287–300 s；第二份三次落在 7763 上都是 431–437 s，两次 `main` 都落在 7763 上时平均约 435 s，照决定 245 合并之后看 |
| S3C-M4 | 低（M9 的测试） | M9 的 `TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches/a_place's_parent_deleted_between_batches`（第四份）在满负载的 `make test` 里失败，是测试自身的缺陷：测试持住 O、任务的第一批锁着笔记本行等 O，网页的删除排在笔记本行上，之后放开 O；第一批提交时 PostgreSQL 唤醒排着的删除，但删除的后端还没重新取锁时，任务的第二批作为新来的事务直接拿到行（xmax 是已提交的锁者，不经元组锁），先写。FIFO 只在排着的等锁者之间成立。C 以一次性的 PostgreSQL 18.6 容器、三个会话与 `kill -STOP` 演示了机制。报告说的"放开之前核对等锁的是哪个后端"钉不住：等锁的本来就是删除。同样的写法还在同一个文件的另三处 | 不属于 S3（S3 在撤销的路径上只加了 `RunBatch` 前的一个分支与 `revertBatch` 里的两处判断）：由旁支修，见旁支 `m11-p1-m9-revert-flake`（`aebf78c3`，从 `main` 开，`(M9 story, found in M11)`）：新的 `batchGate` 包住 transfer 的 `Tree`、扣住下一批，直到网页的写答了才放，不再靠锁的次序，改了七处；它单独合并，合并号与它的记录另写，不挡 S3 的合并。修复一轮第一次 `make test` 又出一次同一机制：同一张表的 `…/a_place's_parent_moved_under_its_node_between_batches`（`revert_job_test.go:358: the web's write = 409 page.cycle`：第二批先把 x 放回 P 之下，网页的移动成了环），修复核对 C 核对过，说明不止一格会撞上 |
| S3A-N1 | 细节（日志） | `RollbackEnd.End` 在 `rollbackSucceeded` 的事务里、提交之前记"upgrade rolled back"（`modules/wiki/app/rollback_end.go:83-85`）：提交失败时（`commit_timeout` 2 秒、连接断了）这一行已经记了，升级却仍在 `rolling_back`；手动的结束回滚是在 `WithinTx` 返回之后才记（`end_upgrade_rollback.go:99`），两处不一致。设计只写"由 `RollbackEnd` 记"。C 在第 4 节记了同一处 | 已改（`9605c0e4`，提交 3）：`End` 交回 `committed func()`（接口 `modules/transfer/app/ports.go:67`，实现 `rollback_end.go:35-89`），只有真的结束了升级时不为 nil；`rollbackSucceeded` 在 `WithinTx` 返回、没有错之后调它（`modules/transfer/app/background_rollback.go:111-112`），再记 "rollback ended"，级别、字段、恰一行照旧。测试：`TestARollbacksEndIsLoggedOnceCommitted`（`background_rollback_test.go:376`：替身事务的提交答错，两行都没记，任务仍在跑）；`TestTheRollbackEnd` 断言调 `committed()` 之前一行也没记（`rollback_end_test.go:176`）。修复核对 A：关；提交失败时任务行随事务回滚、仍是 `running`，River 重试时计划答 Done、再走一次结束；C1、C2、C3 都抓到 |
| S3B-N1 | 细节（测试） | `TestTheRollbackKindsGoDown`（`server/migrations/upgrade_test.go:152-160`）不核对 Down 复原的检查约束不再收 `rollback`：00059 的 Down 复原的 `transfer_jobs_kind_check`、`transfer_jobs_root_check` 与 00058 的 Down 复原的 `changesets_kind_check` 留着 `rollback`（m7、m8、m10）都照过。只是比旧版宽，程序不出错 | 已改（`6d8ed474`，提交 4）：`shape` 加一列 `checks`（`upgrade_test.go:154`），数三个检查里含 `rollback` 的：三个阶段 3、1、0，再 Up 之后又是 3。修复核对 B：关；m7、m8、m10 报出 `checks:2`、`checks:2`、`checks:1`，其余 8 个照 S3 时被抓 |
| S3B-N2 | 细节（文字） | 实施报告第 5 节说读的守卫的七条语句里有 `DeleteUpgradeNodes`，代码里是 `ListRevertTree`（`rollback_cost_test.go:225`）；第 9 节草稿 10.3 的偏离 2 写"去掉一行 `-timeout 20m`"，实际是给先删的一行加上 `-timeout 20m`（报告第 7 节是对的） | 本提交：P1 设计 7.1 与 10.3 照实写 |
| S3C-N1 | 细节（提交说明） | `cc015aec` 改了 `.github/workflows/ci.yml`（costs-3 的时限）与 `Makefile`，范围只写 `server`、只在正文末段提了；`47518c39` 把 `Makefile` 写进了范围（docs/README 列的部件是 `server`、`web`、`api`、`e2e`、`deploy`、`build`、`docs` 等，先例 `2beef88c` 写 `server, ci`）。这是第三次（S1 的 `a28064e9`、S2 的 S2C-N5） | 只记下：已推的历史不改写，写进本记录、P1 设计 10.3 与合并的说明。修复一轮的提交照它写：`63cd2572`、`923ad055`、`9c0a0f42` 写 `server, build`，`6d8ed474` 写 `server, web`。修复核对 C：`9c0a0f42` 只改 `Makefile`，严格说只该是 `build`；改的是服务端测试的分份，不再算一条 |
| S3C-N2 | 细节（测试） | 变更集种类的文字"升级的回滚"没有 vitest（`W/pages/notebook/changeset-text.ts:26`）：`kindText` 的 `switch` 由 tsc 强制有这一支，写错键没人抓；w4（答成 `history.kind.revert`）整个 `src/` 的 vitest 照过 | 已改（`6d8ed474`，提交 4）：`W/pages/notebook/transfer-names.test.ts:21` 加一行：撤销一个 `rollback` 变更集的任务，标题是 "Undo: Upgrade rollback"，经 `jobTitle` 走到 `kindText`。变异：w4 被杀（`job 7's title`）。修复核对 C：关 |
| S3C-N3 | 细节（报告） | 实施报告第 23 条说 7.3 第 41 条写的抓法（写回撞上 `ErrRevisionMismatch`、任务 `internal`）"在这个实现里不会发生"，建议改成"笔记本与开始前不同"。头上并不是这样：变异 41 被 `TestUpgradeRollback` 的六格抓到，失败都在任务那一行（`failed`、`internal`、`done 8/14`）；C 的探针在变异 41 之下打出 error 行 `"a background revert's content of node …: The page was changed since it was read."`，正是设计写的抓法；可见合成的一遍里改名在 Guide 的写回之前，与第 23 条推的次序不合。第一轮为什么没抓到，C 没有在当时的树上重现 | 本提交：7.3 第 41 条照头上的结果写"写回撞上修订号，任务以 `internal` 停下"，不改成"笔记本与开始前不同"；第 23 条的推断记在这里与 10.3。"第一轮没抓到、补了 `[[Kept 2]]` 之后抓到"照实 |
| S3C-N4 | 细节（测试） | `TestUpgradeStates` 的 `reviewing` 一格没断言 `"rollback_left":null`（`bootstrap/upgrade_start_test.go:347-352`）：c4（`getUpgrade` 在任何状态都读差）组合根照过，单元的 `TestGetUpgradeTellsWhatAStoppedRollbackLeft` 抓到 | 已改（`6d8ed474`，提交 4）：`upgrade_start_test.go:355` 加这一断言。变异：c4 的组合根一半被杀。修复核对 C：关 |
| S3A-N3 = S3B-N3 | 细节（注释） | 修复核对一轮的新发现，A、B 各自报出：`MaxDeleteFirstNodes` 的注释（`modules/page/domain/trash.go:76-79`）与先删守卫的注释（`page/rollback_cost_test.go:39-41`）把两种种子、实测与估算混在一起：21–43 秒是 S3 头上旧种子（统计没见过已删的行、autovacuum 开着）在 7763 上的数，拿它比本机稳态的 10.8–11.1 秒说"about twice as slow"不成立；"about 6 to 7 s there"是估算，读来像实测；"about twice as many on CI, past 14 s"只对 7763 一类的机器成立：本轮 CI 在 9V45 上，稳态的单元中位数 2,872 ms 对本机 2,724 ms，一遍 115.5 s 对 109.3 s，之比约 1.05，照线性 1 万个节点约 11.5 s、过不了 14 s。B 另指出 2,500 只有这个守卫钉着：把常量改成 10,000，`page/domain` 与 `page/app` 的单元测试全过（cap，A 的 X1 同），守卫自己的"单元数 = 10 万 ÷ 上限"用的是常量本身，也拦不住；A 的 X1b（本机跑守卫）单元 10.6–11.1 s、守卫是绿的 | 已改（修复第二轮 `de554eda`；下面的位置是它的）：两处注释（`modules/page/domain/trash.go:67-88`、`page/rollback_cost_test.go:30-48`）分三件事写：旧种子下 1 万个节点的单元在 7763 上 21–43 s（run 42635761979）；稳态下 2,500 个节点本机 2.7–3.2 s、CI 的 9V45 上 2.9–3.1 s（run 42671683550），1 万个节点本机 10.6–11.1 s；7763 的稳态没有实测，照 S3 时同一守卫在 7763 与本机之比（约 1.4–1.7）估，2,500 个节点约 4–6 s、1 万个节点约 15–19 s。守卫那一句改为只在 7763 一类的机器上会过 14 s，上限由单元测试钉住、不靠守卫。新的 `TestTheFirstDeletesBound`（`modules/page/domain/trash_test.go:147`）断言 `MaxDeleteFirstNodes == 2_500` 且不大于 `MaxUnitNodes`，注释写明这是决定 243 的数、改它要先改决定。变异：x1（`MaxDeleteFirstNodes = 10_000`）、x2（`MaxUnitNodes = 2_000`，上限比单元大）都被 `TestTheFirstDeletesBound` 抓到。门禁照决定 242：提交单独为绿，`make lint gen-check knip build` 通过，`page/domain` 的包测试 60.8 s；分支 CI 的 run 42688339794 第二次 attempt 18 项全绿（第一次红在一条无关的 M7 守卫，见"修复第二轮的说明"）。修复核对 B：见"核对"第 2 轮 |
| 偏离 | — | 实施者的 27 条偏离（[01-P1](../01-P1-freeze-start-rollback.md) 10.3、3.27）：1 矩阵照 S2 偏离 5（`rollingBackIn`、预定的回滚任务 id）；2 三个守卫进 costs-3，作业时限 35 分钟，先删的一行加上 `-timeout 20m`；3 做完的结束：锁 → `FinishJob` → `End`；4 `RollbackPass.Items`，`wiki.NewRollbackEnd(pool, notebooks, states, pages, clock, logger)`；5 回滚的 worker 在 1 号登记；6 `PlanRollback` 不收 `nominal`、先判定再读；7 回滚的读逐行经键（`LATERAL … LIMIT 1`），新的 `RollbackNodesNow`、`RollbackContentsNow`；8 合成的批复用 `revertPart`；9 三个守卫的组织；10 `Rollbacks.Start` 的客户端；11 `TestARollbackStartsInItsUpgradeAlone`，时刻与 `elapsed_ms`；12 `:execrows`，写完经 `Current` 读回；13 `ErrRollbackWriting`、`UpgradeDeps.locked`；14 `rollback_left` 何时现读；15 `TestUpgradeRollback` 的组织；16、17 交错第 11、12 条；18 流；19 冻结的"任务的批"两本；20 `awaitBackground`；21 UP3 两个测试；22 第 41 条的夹具；23 第 41 条的抓法；24 第 71 条等价、改 71b；25 第 45、49、69 条的组合根一半；26 只差次序值的节点、第一遍一步；27 River 丢掉的回滚由真的 rescue 记为中断 | 1–4 主持者开工前同意。第 3、7、14 条 A 与 B 都核对过，照实施的：次序比设计的稳（`FinishJob` 答 !ok 时 `End` 不跑；答 left 时整个事务回滚；锁的次序始终笔记本行在前、任务行在后，与取消、心跳、Rescue、删除笔记本都不成环），逐行经键的代价数与 B 的实测相符。第 2 条在草稿里抄错（S3B-N2）；第 23 条的推断不对（S3C-N3）；第 24、25 条照实改 7.3，第 49 条审查之后另有组合根的一格（S3C-M2）。其余三位各在自己的范围里判过，都对；已改在设计各节（3.27 列出） |
| 留下的事 | — | 实施报告第 8 节留下的五项：1 Step 的文档提交；2 先删的第 2–4 个单元慢 2–3 倍，原因未核实，CI 上也一样；3 读的守卫在 CI（7763）上余量最小，合成的读 2,968 ms 对 4.6 s；4 M9 的撤销测试在满负载下失败过一次，可在放开之前核对等锁的是哪个后端；5 S4 照设计 | 1 是本提交；2 B 查清是 `nodes`、`page_contents` 的统计过时（"接受与推后的"），守卫改测稳态，统计过时时的线性变慢进 M12 的性能移交（决定 243）；3 即 S3B-Q1；4 即 S3C-M4，C 指出那个钉法钉不住；5 照设计 |
| 修复一轮的说明 | — | 实施者在修复一轮的说明（`fix-1-report.md` 第 1、7 节）：1 提交 1、2 的范围写成 `server, build`（各改了 `Makefile` 里自己守卫的注释），说明里是 `server`；2 linking 的 `lint.sql`、`lint_rules.sql` 有七处 `NOT IN (SELECT id FROM excluded…)`，不是回滚的读，没动；3 必须做到的第一条（上限改回 1 万时守卫要红）本机做不到，最慢 11.1 s；4 `RollbackLeft` 丢键的变体第一次跑时卡在读种子上（测试在核对之前先执行了 `RollbackLeft`），按进程号停掉那次运行，把读种子挪到核对之后；5 CI 前两次 attempt 有作业没建出来（GitHub 同一时刻开了 "Incident with Actions and Checks API"），两次 `gh run rerun` 整个运行，第三次全绿；6 第二份在 7763 上 437 s；7 头上两个与本轮无关的偶发：`make test` 的 M9 撤销插队、`make e2e` 的 L1 新标签页超时 | 1 照 S3C-N1，对；2 B 核对过：是 `unnest` 的集合，20 万个之内都是 hashed，lint 的守卫已钉住（"接受与推后的"）；3 即 S3A-N3 = S3B-N3，第二轮以单元测试钉住；4 A 判合理：计划有问题时在计时之前停下正是决定 244"计划为主"的意思，按进程号停测试进程不违反"不用 `pkill`、`killall`"；5 C 从三次 attempt 的作业 JSON 核对过，三次都在同一个 sha；6 照决定 245 合并之后看；7 见 S3C-M4 与"接受与推后的" |
| 修复第二轮的说明 | — | 实施者在第二轮的报告（`fix-2-report.md`）：1 两处注释照说明分三件事写，守卫那一句改为只在 7763 一类的机器上会过 14 秒、上限由单元测试钉住；2 本机门禁照决定 242，其余由分支 CI 担保；3 分支 CI 的 run 42688339794 第一次 attempt 17 项过，guards-core 红在 M7 的 markdown 时长比守卫 `TestTheAppsMarkdownCostsAboutItsSize/to_a_page`（`platform/markdown/markdowntest/costs.go:187`，"a block formula left open"：512 KB 12.72 ms，是四分之一的 1.45 ms 的 8.76 倍，线是 8 倍；EPYC 9V74），e2e、image 因此被跳过；`gh run rerun --failed` 之后 guards-core 过（11:07），e2e、image 也过，18 项全绿 | 1 照 S3A-N3 = S3B-N3；2 照决定 242；3 实施者判为与 S3 无关：本提交只改注释与 `page/domain` 的一个单元测试，碰不到 markdown 的解析 |

## 主持者的决定

负责人可以改判（接着 S2 的第 240–242 条编号；243–245 在第一轮的修复说明 `fix-1.md` 里定下）：

243. **回滚先删的单元另有自己的节点上限 `MaxDeleteFirstNodes = 2,500`**（S3A-M1，2026-11-10）：比 `MaxUnitNodes` 小，装在一起的子树合计不过它；单独一棵在 2,500 与 `MaxUnitNodes` 之间的子树独占一个单元，更大的照设计往下拆。理由：先删一个单元删上万棵小子树，比恢复与撤销同样大小的单元慢 2–3 倍，单元持着笔记本行，期间这本笔记本上的管理写要等，过了 15 秒的请求期限就答 500。照决定 153，守卫判每个单元不过 14 秒、不取三倍的规则。守卫的种子先删 5,000 页再 `ANALYZE`，让统计见过已删的行，测的是稳态。统计过时（大量删除之后、autoanalyze 之前）时单元会线性变慢，是哪条语句没有核实，交给 M12 的性能移交，不在这里优化。修复第二轮：上限由 `page/domain` 的单元测试钉住（守卫只在 7763 一类的机器上抓得到改回 1 万，S3A-N3 = S3B-N3）。
244. **回滚的读的守卫照第 236 条"计划为主"再收紧两处，并换种子的形状**（S3B-I1、S3B-M1 = S3A-N2、S3B-Q1，2026-11-10）：在内侧逐行重复的扫描必须经外行的键；不许逐行重读整个集合（非 hashed 的 SubPlan 之下、嵌套循环内侧的 `CTE Scan`、`WorkTable Scan`、`Materialize`）。计时的笔记本是 1 万个文件夹、各是一页下面 9 页，另 1 万个新文件夹收着三分之一的页（10 万行基线、1 万个收着者）；原来的 100 个文件夹各 999 页作第二本只核对计划；九本别的笔记本照旧（决定 240）。`RollbackFreeForest` 用 `NOT EXISTS`。理由：原来的 `NOT IN` 在统计分析过之后是普通的 SubPlan，代价是活节点数 × 收着者数；守卫原来的种子扇出极大、收着者只有约 200 个，看不出；1 万个收着者时同样 10 万页要 26.8 秒，`NOT EXISTS` 是 0.32 秒。
245. **第一份里 `^TestABackground` 的 13 个顶层挪到第七份**（S3C-M3，2026-11-10）：`BOOTSTRAP_SEVENTH = ^Test(Upgrade|IngestBefore$$|ABackground)`，第一份的目标加 `-skip '$(BOOTSTRAP_SEVENTH)'`。理由：分支的 CI 上第一份的测试一步 446 秒、第二份 443 秒，离 450 秒的线只差 4 与 7 秒；`main` 上第一份落在 7763 上时是 442 秒；照 M10/P5/S1 的先例，贴线就先挪，第七份离线最远（211 秒）。第二份暂不动，合并之后看两次 `main`。

修复一轮的处置照 `fix-1.md`：三份审查全部接受，一组一个提交、各自绿、接在 `9b572167` 之后、不改写已推的历史；成本守卫的事只调单元的大小、种子的形状或界，外加一句 SQL 的写法，不做别的性能工作（负责人 2026-11-06："调测试用例量级或者时限即可"）；只改文档的（S3B-N2、S3C-N1、S3C-N3，7.3 第 41、45、49、69、71 条，A 的未核实的怀疑第 1 条，M12 的性能移交）留给 Step 的文档提交。第二轮照 `fix-2.md`：一个提交，只改两处注释、加一个单元测试，本机门禁照决定 242。

## 接受与推后的

- **接受**：
  - 照预料存活的变异：A 的 T11（`ErrJobGone` 走 `stopped`，即第 71 条，等价：`stopped` 归类时保留错误链，`finish` 先判 `errors.Is(err, ErrJobGone)`；71b 抓到）、A4（去掉 `rollbackContents` 的 `len(refs) != len(written)`：基线的修订在开始之后不会被合并、purge 碰不到冻结之下删不掉的基线节点，这一判是防线）；第 45、69 条的组合根一半不盖（7.2 第 12 条两个方向都走 `rollbackSucceeded`；删除笔记本在同一个事务里把任务行一并软删除，`FinishJob` 答 !ok，`End` 照样不调；变异 45 在组合根要造"rescue 先把任务记失败、worker 随后失败、升级仍在回滚"才看得出，后果只是多一帧 `mode`），由 `TestARollbacksEnd` 钉住；C 的 c1（做完的结束里 `End` 挪到 `FinishJob` 之前）组合根照过、单元抓到，组合根的差别只在 `End` 已记的那一行，这一行修复一轮改到提交之后（S3A-N1）。
  - A 的未核实的怀疑第 1 条：`import_workers` > 1 时，Rescue 把心跳过期、其实还活着的回滚任务记为失败，管理员随即继续，开出第二个回滚任务，两个都带同一个 `UpgradeOwn`。它们只靠笔记本行串行、每一遍开头都重算，A 推不出会写坏的次序，只会多做几遍或撞 `ErrRevisionMismatch` 后以 `internal` 停。属决定 225 的僵尸任务那一类，默认一个 worker 时不会出现；没有复现。
  - 先删的单元第 2–4 个为什么慢（实施报告留下的第 2 项，B 查清）：`nodes`、`page_contents` 的统计过时，种子的 `ANALYZE` 是在一页都没删的时候做的；慢到 autovacuum 第一轮的 autoanalyze 为止，与死元组、与 autovacuum 争 I/O 都无关。B 在本机三轮对照：照原样十个单元中位数 11.6 s，autovacuum 两轮在 42.5 s 与 102.6 s，慢的第 2 个单元（23.9 s）整个落在第一轮之前；关掉 autovacuum、第 1 个单元之后只分析 `changeset_items`、`page_search_queue`，单元线性变慢到 70.6 s，中位数 50.7 s；同样关掉、另分析 `nodes`、`page_contents`，死元组照样积到 10 万，单元一直平（中位数 11.5 s）。是哪一条语句换了计划没有逐条核实（推测统计里 `deleted_at` 全为 NULL、`trash_root` 全为假时部分索引看起来是空的）。真实实例里统计通常含已删的行，碰上同样的窗口也只是慢一阵、不出错；守卫改测稳态（决定 243）。
  - linking 的七处 `NOT IN (SELECT id FROM excluded…)`（修复一轮实施者的说明第 2 条，B 核对）：集合都是 `WITH excluded AS MATERIALIZED (SELECT unnest($array::uuid[]) AS id)`，不是递归 CTE，行数估计没有膨胀（自定义计划里是数组的长度，通用计划里固定 10 行）；lint 的守卫 `TestLintPhysicalRangesCompleteSources`（`modules/linking/adapter/postgres/lint_plan_test.go:846-890`）在 0、1 万、10 万三种大小 × 两种统计 × 两种计划下断言成员判定是真跑出来的 hashed SubPlan、CTE Scan 只跑一次；B 在一次性的 PostgreSQL 18.6 上（默认 `work_mem` 4MB、`hash_mem_multiplier` 2，项目与部署都没改）量到 20 万个 uuid 仍是 hashed、25 万起是普通 SubPlan。在 lint 的 10 万规模之内没有问题，越界的一句进 M12 的性能移交。
  - C 的未核实的怀疑：c10 之下的 `forbidden` 是哪一层答的没有追到（推断是收着基线节点的那棵被删时，权限或冻结的判断以 forbidden 一类拒了），不影响 S3C-M1 的结论；分支这次的 9V74 慢了 17–29% 是 VM 的缘故，是推断（S3C-M3 的挪份不靠它）。
  - 偶发的失败，都与 S3 无关、各自重跑为绿：M9 撤销测试的插队（S3C-M4）出现了两次，建成时 `…parent_deleted…` 一格、修复一轮头上 `make test` 第一次的 `…parent_moved_under_its_node…` 一格；修复一轮头上 `make e2e` 第一次的 L1（`l1-links.spec.ts:178`，带修饰键点链接、等新标签页 30 s 超时，负载约 10），是已知的旧偶发：同一行的同一症状见 M10/P3/S1 的审查记录（CI 41188823890），底层原因没查明，本轮网页只改了一个 vitest 文件；修复一轮头上的这两次，修复核对 C 判都与 S3 无关。修复第二轮的分支 CI 第一次 attempt 红在 M7 的 markdown 时长比守卫 `TestTheAppsMarkdownCostsAboutItsSize/to_a_page`（`platform/markdown/markdowntest/costs.go:187`，"a block formula left open"：512 KB 12.72 ms，是四分之一的 1.45 ms 的 8.76 倍，线是 8 倍；EPYC 9V74），只重跑那一项变绿，实施者与 B（修复核对第二轮）都判为与 S3 无关：S3 的提交没有碰 `platform/markdown`，这是共享 runner 上毫秒级的比，只超出 0.04 ms 的余地。
  - 修复一轮的 CI 前两次 attempt 有作业没有建出来（第一次只建出 3 个、第二次缺第四份与 costs-3，e2e、image 因此被跳过），是 GitHub Actions 的事故，工作流本轮没改、矩阵是静态的；第三次 18 项全绿，数都取自它。
  - 流程（照实写下）：
    - 实施者推送之前改写过三次 4、5 号（都还没推过；1–3 号没动）：`43d4dda2`/`d979dad5` → `c2bac045`/`9ca5d122`（给 Guide 加 `[[Kept]]`，三处"剩下的差"从 `2 1 x` 变为 `2 2 x`，竞态构建的顶层数改为 542、第七份 15）→ `08457375`/`a651fd76`（第 41、43、72 条组合根的一半补测试）→ `47518c39`/`9b572167`（第 37 条补测试）；后两次只改 `bootstrap/upgrade_rollback_test.go`，5 号是原样重放；第二次改写之后开始的整串门禁因第三次改写停下重来（以 TaskStop 停下，没有留下进程与 `heavy.sh` 的锁）。
    - 实施者变异第一轮的规格有三处命令错了（第 60 条跑错了文件，第 50、51 条的矩阵没连 `prepare` 一起选），改命令重跑都真的抓到；6 条改坏之后编不过，改写规格重跑。
- **推后**：
  - M12 的性能：先删在统计过时之时单元线性变慢（关掉 autovacuum 时 2,500 个节点的单元从 3.0 s 涨到第 40 个的 28.1 s；B 那一轮 1 万个节点的单元中位数 50.7 s），是哪条语句没有核实；lint 的排除集超过约 20 万个时 `NOT IN` 变成逐行的 SubPlan。写在 00 号文档"写给后面的 M"，M11 收尾时追加到 M12 的性能移交。
  - S3C-M4：M9 的撤销测试的插队，由旁支 `m11-p1-m9-revert-flake`（`aebf78c3`）修，单独合并，合并号与记录另写。
  - 合并之后看：第二份在 7763 上 431–437 秒，两次 `main` 的平均（决定 245）；第一份在 7763 上（修复一轮 371 秒）；第七份第一次落在 7763 上的数（估约 290–300 秒）；costs-3 第一次落在 7763 上时两个守卫的数（合成的读估约 2.0 秒对 3.1 秒的界、约 1.5 倍；2,500 个节点的单元估约 4–6 秒）。
  - 没有交给 v0.1 之后的打磨的。

## 反向对照

建成时以 `mut.py` 跑（`NW_SERVER` 指实施者的工作树，网页的三条指 `web/apps/web`，经 `heavy.sh`，每个之后由工具从备份复原，改坏时不跑 `make gen*`，改生成的 SQL 时改生成的常量），在 `9b572167` 之前的几个头上分六轮，每一轮先在没改的树上跑同样的命令（`m11/s3/mut/run.out`–`run6.out`）。设计列两个测试的，两半各跑一次。修复一轮在 `9c0a0f42` 上整批经 `heavy.sh` 跑（`m11/s3-fix1/mut/`，组合根的不带 `-race`）。审查者的在 `s3-review/` 下（A 的 `a/`、B 的 `b/`、C 的 `c/`；修复核对一轮在各自的 `check1/` 下）。

**7.3 的 S3 各行**（25 行、28 种改坏：第 13 条的 S3 一半，第 37–53、60、61、69–73 条，第 38、69 条各两种写法，第 71 条另加 71b；每一种都有点名的测试抓到，除第 71 条）：

| # | 变异（实际的改法） | 跑的测试 | 结果 |
|---|---|---|---|
| 13 | `FreezeAdmits` 不核对过期的 `UpgradeOwn`（`case false:`） | `TestFreezeAdmits`；7.2 第 16 条 | 两个都抓到 |
| 37 | Rescue 的种类表漏 `rollback`（`Kinds()`） | `TestTheRescue`；`TestUpgradeRollback` 的"River 丢掉的排队中的回滚" | 单元抓到；组合根第五轮没抓到（那一格以 `FailQueued` 的 SQL 代 rescue 写），改为真的跑 rescue 之后抓到 |
| 38a | 写入键的部分索引不含 `rollback`（迁移 00059） | `TestChecksRejectCounterexamples` | 抓到 |
| 38b | `Writing` 的查询不含 `rollback`（生成的常量） | `TestARollbackIsOneWritingJobOfItsNotebook`；"在跑时继续与结束回滚答 `transfer.busy`" | 两个都抓到 |
| 39 | 不先删新建的子树 | `TestARollbackPassAtATime`；"放弃与做完" | 两个都抓到 |
| 40 | 合成的条目带 `Revision` | `TestRollbackState`；"放弃与做完" | 两个都抓到 |
| 41 | 回滚的单元 `UpdateLinks = true` | `TestUpgradeRollback` | 第一轮没抓到；夹具里 Guide 的基线另链向 `[[Kept 2]]` 之后抓到（六格，任务 `internal`，S3C-N3） |
| 42 | `RollbackLeft` 的"不在原位"只比父节点（生成的常量） | `TestARollbacksReads`；"结束时什么也不剩或只剩一次改名" | 两个都抓到 |
| 43 | 先删的一步每棵子树一个单元 | `TestPackFreeSubtrees`；"放弃与做完"；交错第 11 条 | 单元抓到；"放弃与做完"第二轮没抓到（一批装得下六步，分不出），断言第一遍 `steps` 为 1 之后抓到（`steps:6`）；第 11 条抓到 |
| 44 | 差不缩小时不停 | `TestARollbackRunsPassByPass` | 抓到 |
| 45 | `rollbackStopped` 不看 `FinishJob` 的 `ok` | `TestARollbacksEnd`；7.2 第 12 条 | 单元抓到；组合根照过（两个方向都走 `rollbackSucceeded`） |
| 46 | 回滚停下不调 `Stopped` | `TestARollbacksEnd`；`TestUpgradeStatesReachTheStreams` | 两个都抓到 |
| 47 | `RollbackEnd.End` 不清笔记本的列 | `TestTheRollbackEnd`；"放弃与做完" | 两个都抓到 |
| 48 | 结束回滚不删基线行 | `TestEndUpgradeRollback`；"停下、结束、差不空" | 两个都抓到 |
| 49 | 结束时差不空照写成功（`if left && false`） | `TestARollbacksEnd`；"一遍之后差不减" | 单元抓到；那一格照过（它的 `internal` 是 `rollback()` 里差不减的一支）；修复一轮之后"something left at the end"抓到 |
| 50 | 回滚的任务允许写者取消 | `TestOnlyAdminsCancelARollback`；矩阵的"its own rollback, ended" | 两个都抓到（矩阵改命令连 `prepare` 之后） |
| 51 | 取消的回滚规则放在 `sees` 之前 | 矩阵的"another's rollback, queued" | 抓到 |
| 52 | 放弃先记任务的 id、后写状态 | `TestAnUpgradesRowRollsBack` | 抓到（`notebook_upgrades_rollback_job_check`） |
| 53 | 每一遍不核对升级（`case false && (…)`） | `TestARollbackPassAtATime`；"两遍之间以 SQL 结束升级" | 两个都抓到（组合根等到 60 秒） |
| 60 | `shownCounts` 不含 `rollback` | `transfer-background.test.tsx` | 抓到（第一轮的命令跑错了文件） |
| 61 | `jobs-server.ts` 的 `writingKinds` 不含 `rollback` | `src/pages/notebook/` 的 vitest | 抓到 |
| 69a | `rollbackSucceeded` 锁不到行照写 | `TestARollbacksEnd`；7.2 第 12 条 | 单元抓到；组合根照过，与原样一样 |
| 69b | `rollbackStopped` 锁不到行照写 | 同上 | 单元抓到；组合根照过，同第 45 条 |
| 70 | "rollback ended"降成 debug | `TestARollbacksEnd` | 抓到 |
| 71 | `PlanRollback` 的 `ErrJobGone` 经 `stopped` | `TestARollbacksEnd`；"两遍之间" | 两个都照过：等价的变异 |
| 71b | `finish` 不看 `err` 里的 `ErrJobGone`、只看 cause | 同上 | 两个都抓到（组合根等到 60 秒） |
| 72 | 合成的条目漏只差次序值的基线节点 | `TestRollbackState`、`TestRollbackPlansPutTheBaselineBack`；"放弃与做完" | 单元抓到；"放弃与做完"第二轮没抓到（夹具里没有只差次序值的节点），agent 另把 Notes 移进 Folder 再移回之后抓到 |
| 73 | `jobTitle` 漏 `rollback` | `transfer-names.test.ts` | 抓到 |

实施者自己加的 13 行、16 种（o8 四种写法）都抓到：o1 放弃不 `SetState(rolling_back)`、o2 继续不判 `Writing`、o3 结束回滚不判 `Writing`、o4 结束回滚总记完整、o5 在跑时也现读 `rollback_left`、o6 `ContinueUpgradeRollback` 不限 `rolling_back`、o7 `RollBackUpgrade` 不限进行中的两种状态、o8a–d 读的守卫的核对放水（不认连接的索引条件、`upgrade_nodes` 任意索引都算、子计划的位图也认、没有扫描节点不报，`TestTheRollbackGuardJudgesThePlans`）、o9 `Stopped` 不 `Publish`、o10 "rollback pass"降成 debug、o11 结束回滚不清笔记本的列、o12 继续不写行的新任务、o13 继续记成 "upgrade abandoned"。

**修复一轮的变异**（`m11/s3-fix1/mut/`，在 `9c0a0f42` 上；连同审查者的写成 [01-P1](../01-P1-freeze-start-rollback.md) 7.3 的第 111–121 条，第 37、41、43、45、49、69、71、72 条照实改）：

| 来源 | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| 39、42、43、44、47 | 实施者的旧变异重跑 | 同建成时 | 都抓到；43 由 `TestPackFreeSubtrees` 的三格（含新格）抓到 |
| 49 | `if left && false` | `TestARollbacksEnd/something_left`；`TestUpgradeRollback/something_left_at_the_end` | 都抓到；旧的一格 `a_pass_that_leaves_as_much` 照旧存活 |
| o5 | 写着时也读剩余（原来的 `if !writing` 已不在，改成去掉判断里的 `writing` 一项、留 `_ = writing`） | `TestGetUpgradeTellsWhatAStoppedRollbackLeft`；`…/busy_while_it_runs` | 都抓到（第一次没留那一行，编不过） |
| o8a–d | 守卫的核对放水 | `TestTheRollbackGuardJudgesThePlans` | 都抓到 |
| n1（121） | 装步用 `whole` 不用 `pack` | `TestPackFreeSubtrees/random_forests`；`TestARollbackPassAtATime/free_subtrees_past_MaxDeleteFirstNodes` | 抓到 |
| n2（121） | `deleteBatch` 以 `maxUnitNodes` 判 | `TestARollbackPassAtATime/free_subtrees_past_MaxDeleteFirstNodes` | 抓到 |
| n3（121） | `maxDeleteNodes` 不受 `maxUnitNodes` 限 | `TestARollbackPassAtATime/free_subtrees_past_MaxUnitNodes` | 抓到 |
| n5（116） | 去掉"逐行重复须经外行的键" | 判据测试 | 抓到：A 的三格与 B 的一格都红 |
| n6（117） | 去掉"不许逐行重读整个集合" | 同上 | 抓到：`a_CTE_read_whole_for_each_row,_a_SubPlan_not_hashed`、`a_set_materialized,_read_whole_for_each_row` |
| n7（117） | SubPlan 之下不算逐行重复 | 同上 | 抓到：`a_CTE_read_whole_for_each_row,_a_SubPlan_not_hashed` |
| n8（118） | hashed 的 SubPlan 也算逐行重复 | 同上 | 抓到：`a_CTE_read_once,_a_hashed_SubPlan` |
| A（119） | `getUpgrade` 不在快照里（三次读各自一条语句） | `TestGetUpgradeTellsWhatAStoppedRollbackLeft`；`TestUpgradeInterleavings/the_read_of_an_upgrade_across_its_rollback's_end` | 都抓到，组合根是真的行为 |
| A（120） | 提交前记 rolled back：transfer 一侧在事务里就调 `committed`；wiki 一侧 `End` 里直接记 | `TestARollbacksEnd/done`、`TestARollbacksEndIsLoggedOnceCommitted`；`TestTheRollbackEnd` | 都抓到 |
| B 的 m7、m8、m10（115） | Down 复原的检查留着 `rollback` | `TestTheRollbackKindsGoDown` | 都抓到（之前存活） |
| B 的三处丢键（116） | `FreeForest`、`Holders`、`Left` 的 `created` 探针写成 `b.node_id::text = n.id::text` | 读的守卫 | 都在"从未分析"的计划核对红：17.2、16.9、15.9 s（之前要等超时） |
| 原来的 `NOT IN`（117） | `NOT EXISTS` 改回 `NOT IN` | 读的守卫；判据测试 | "分析过"的计划核对红（25.3 s；"从未分析"那格是 hashed，照样收）；判据测试的 SubPlan 一格 |
| C 的 c4（113） | `getUpgrade` 在任何状态读差 | `TestUpgradeStates` | 组合根抓到（之前存活） |
| C 的 c10（111） | `RollbackFreeForest` 的递归一步 `WHERE false` | `TestARollbacksReads`；`TestUpgradeRollback` | 都抓到（之前存活） |
| C 的 c11（112） | `End` 只看 `Pages`、`Created` | `TestTheRollbackEnd` | 抓到（之前存活） |
| C 的 w4（114） | `kindText` 的 `rollback` 答成 `revert` 的键 | `transfer-names.test.ts` | 抓到（之前存活） |
| A 的 T11、A4 | 等价 | — | 照旧存活，没有重跑 |

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，等价的除外）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | T1–T10（差不减不停、`>=` 改 `>`、做完与停下时不理 !ok、left、没锁到，回滚不设 `rollbackStopped`，取消不要管理员，`rollsBack` 不看状态、不看升级 id） | 抓到 |
| A | T11、A4 | 存活：等价 |
| A | K1–K10、K2b（wiki 的结束回滚、`RollbackEnd`、放弃、`getUpgrade`；K5、K8 第一次编不过，重写之后抓到） | 抓到 |
| A | A1–A3、A6，D1–D6，P1–P5（生成的常量），WA1、WA2，N1、N2 | 抓到 |
| A | 三格合成计划喂原样的 `rollbackScanProblems` | 都被接受：S3A-N2 |
| B | 迁移的 m1–m6、m9、m11 | 抓到 |
| B | m7、m8、m10 | 存活：S3B-N1 |
| B | 读的 8 个 SQL 变体：三处探针丢 `node_id` | 几格过核对：S3B-M1 |
| B | `NodesNow`、`ContentsNow` 丢键；两条去掉 `LIMIT 1`；`FreeForest` 改用 `NOT EXISTS` | 丢键的六格都被抓；去掉 `LIMIT 1` 的在计划变了的格都被抓、没变的格仍逐行经主键；`NOT EXISTS` 六格都过核对、分析过时 735 → 245 ms |
| B | 矩阵的 mx1（判定挪到 `sees` 之前）、mx2（去掉判定） | 抓到 |
| C | 7.3 的 S3 各行 28 种（重跑） | 第 71 条等价、71b 抓到；45、69a、69b 的组合根一半照报告说的不盖；49 的组合根一半探针造得出：S3C-M2；41 抓到，抓法与报告说的不同：S3C-N3 |
| C | c1–c9、c12–c16（c14 两种）、w1–w3 | 单元或组合根之一抓到；c1、c8 只有一半抓到（c1 组合根照过，c8 单元的替身不分） |
| C | c4 | 单元抓到、组合根照过：S3C-N4 |
| C | c10、c11 | 只有探针抓到：S3C-M1、S3C-M2 |
| C | w4 | 照过：S3C-N2 |

A 44 个（42 个抓到、2 个等价），另以合成的计划核对守卫；B 迁移 11 个、读的 8 个变体与一道更严的核对、矩阵 2 个；C 重跑 7.3 的 S3 各行 28 种、自己想的 21 种（18 种由单元或组合根之一抓到）。

**修复核对一轮**（在 `9c0a0f42` 上）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | 审查时的 42 个（K3、K4、K8、K9、A3、D1、D2、P3 照头上的写法重写） | T11、A4 照旧等价，其余都抓到；A3 现在由两格抓到，D1 由新格抓到，P3 是去掉 `NOT EXISTS` 那一句 |
| A | G1（单元）、G1b（组合根）：`getUpgrade` 不用快照 | 都抓到；G1b 下答 200、`created: 4`，正是审查时探针看到的错答 |
| A | C1、C2、C3：在事务里就调 `committed`；`End` 自己先记；永远不调 `committed` | 都抓到 |
| A | X1（`MaxDeleteFirstNodes = 10_000`，跑 `page/domain` 与 `page/app`）、X1b（跑先删的守卫） | 存活：两个包都按常量的名字取值；守卫本机 10.6–11.1 s、一遍 108 s，是绿的（S3A-N3） |
| B | 迁移的 11 个 | 都抓到（m7、m8、m10 现在由 `checks` 一列抓到） |
| B | g1–g3 三处丢键、g4 改回 `NOT IN` | 都在计划核对一步红：16.0、15.9、15.7、24.9 s |
| B | cap（`MaxDeleteFirstNodes = 10_000`） | 存活：S3B-N3 |
| C | c10、c10b（新）、c11、c4 各两半，49 的组合根一半，w4 | 都抓到；c10 的组合根 9 格失败（六个任务 `forbidden`，三格没停到停处），c10b 的组合根多一遍 |

**修复第二轮**（在 `de554eda` 上，`m11/s3-fix1/mut/spec-fix2.json`、`run-fix2.out`）：x1（`MaxDeleteFirstNodes = 10_000`）与 x2（`MaxUnitNodes = 2_000`，上限比单元大）都被 `TestTheFirstDeletesBound` 抓到（写成 7.3 第 122 条）；复原之后没有 `*.mutbak`。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `9c0a0f42`（修复一轮 `63cd2572`–`9c0a0f42`，5 个提交），三位各在自己的工作树里 | A：S3A-M1、M2、N1、N2 都关了；稳态下单元离请求期限远（9V45 上约 5 倍），一棵 2,500–10,000 个节点的子树独占一步，与用户经 HTTP 删同样大的子树走同一条路；快照真的盖住三次读、没有新的次序问题；`committed` 的成功、提交失败与事务体出错三条路都对；重跑 42 个变异与新的 G1、G1b、C1–C3；新的一条 Nit（S3A-N3），另提可选的做法：单元界照第 236 条取本机最慢的三倍约 10 s，余量只有约 1 s，由主持者定。B：S3B-I1 代码侧关（设计 3.18 的 SQL 待改）、M1、N1 关，Q1 关（问题已答），N2 待文档；12 个计划都是哈希反连接；`rollbackHashed` 稳；新种子与新界都合第 236 条；linking 的七处 `NOT IN` 没有问题；新的一条 Nit（S3B-N3）。C：S3C-M1、M2、M3、N2、N4 关，N1、N3 归文档没有问题，M4 留给旁支；重跑 10 次变异都抓到；归类 145/51/121/124/62/11/28 与 CI 的数照实；两次偶发都与本轮无关；新发现 0 | 所有要改代码的发现都已关闭（A、B、C 都判了）；S3A-N3 = S3B-N3 在修复第二轮改（`fix-2.md`，`de554eda`）；A 的"单元界取 10 s"不取：照修复说明不为此改界，以单元测试钉住常量；其余照文档提交写 |
| 2 | `de554eda`（修复第二轮 1 个提交，只改两处注释与一个单元测试），B 在自己的工作树里 | B：S3B-N3 关：两处注释的数与估算核过（本机 2.7–3.2 秒、10.6–11.1 秒乘 1.4–1.7，约 4–6 秒、15–19 秒）；头上 `page/domain` 的包测试通过；常量改成 10,000 时 `TestTheFirstDeletesBound` 红（修复核对一轮这一变异存活）；分支 CI 第一次 attempt 的 markdown 时长比（8.77 倍对"8 倍加 1 ms"，只超出 0.04 ms，EPYC 9V74）与 S3 无关：S3 的提交没有碰 `platform/markdown`；没有新发现 | 没有要改的；S3 的审查到此结束 |

## 实测

- **新的测试在竞态检测下**（建成时，`go test -race -count=1 -v`，逐包，经 `heavy.sh`，头 `9b572167`，共 146 秒）：组合根的包 43.7 秒，第七份的 `TestUpgradeRollback` 13.30（含另起应用的超时一格）、`TestUpgradeFreezeReachesEveryPath` 7.61、`TestUpgradeInterleavings` 6.78、`TestUpgradeStatesReachTheStreams` 4.58、`TestUpgradePermissionMatrix` 1.54、`TestUpgradeStates` 0.86 秒；`TestThePermissionMatrixCoversEveryOperation` 0.65、`TestLintMatrixTargets` 0.70 秒。`wiki/adapter/http` 的 `TestTheUpgradesCodesAreAnswered` 13.95 秒（S2 时 7.87 秒，多了两个操作的码）。`migrations`：`TestChecksRejectCounterexamples` 5.63、`TestConstraintAndIndexNames` 1.38、`TestTheRollbackKindsGoDown` 0.06 秒。`page/domain`：`TestRollbackPlansPutTheBaselineBack` 3.10、`TestPackFreeSubtrees` 0.38 秒；`page/app`：`TestTheRollbacksStepsReplayToTheBaseline` 1.98 秒；其余各在 1.4 秒之内；`transfer/adapter/http` 整包 49.9 秒。修复一轮的"something left at the end" 0.47 秒。
- **回滚的成本守卫，建成时**（三个都 `!race`、`-short` 跳过，各一行进 `test-go-costs-3`；本机两轮，经 `heavy.sh`，`m11/s3/guards-r{1,2}-*.log`）：
  - 计划（`TestARollbacksPlanOfAHundredThousandItemsTakesItsTime`，`page/app`；10 万个条目：100 个文件夹各 1,000 页的基线，每页移到下一个文件夹、改名或改次序各三分之一；五次的中位数与 `TotalAlloc`）：639、644 ms，527 MiB；界 2 s、1,600 MiB。
  - 读（`TestARollbacksReadsTakeTheirTimeAtAnyStatistics`，`page/adapter/postgres`；10 万节点、10 万页的笔记本，100 个文件夹各 999 页、100 个收着者，另九本各 5 万页的笔记本与它们的升级的基线行；三种统计 × 两种计划；时长在 `MaxConns 1` 的池上、`SET plan_cache_mode`）：`RollbackLeft` 六个中位数最大 413、429 ms，一遍的合成读最大 1,449、1,514 ms；种子 13.5、13.8 s（这本 2.4 s）；界 1.3 s、4.6 s。第一次核对报 21 处违反（设计的连接与按十万个 id 的数组读），改成逐行经键（偏离 7）：`RollbackLeft` 从 127–462 ms 变成 368–429 ms（分析过的状态里整表扫描反而快），合成的读从 534–1,242 ms 变成 885–1,514 ms，随这本笔记本而不随实例。B 一轮：`RollbackLeft` 344–358 ms，合成读 819–1,333 ms，种子 12.4 s。
  - 先删（`TestARollbackDeletesAHundredThousandScatteredPagesFirst`，`page` 模块根；1,000 个文件夹下 10 万个零散的新页，10 个单元，真的 `PlanRollback` 与 `RunBatch`）：一遍 180.0、182.3 s，单元中位数 12.0、13.0 s，746 MiB；种子 2.5、2.6 s；界 550 s、39 s、2,300 MiB；第 2–4 个单元 27–38 s，其余 12–16 s。
  - `make test` 里（第二次，3,311 秒）：计划中位数 515 ms；`RollbackLeft` 326–361 ms、合成的读 795–1,318 ms；先删一遍 131.3 s、单元中位数 11.4 s。
- **修复一轮之后**（`m11/s3-fix1/`，本机两轮）：
  - 先删（40 个单元，各 2,500 页）：单元 2,689–2,758 与 2,670–3,221 ms，中位数 2,724、2,713 ms，一遍 109,282、109,249 ms，计划 371、163 ms，分配 737 MiB，种子 2,445、2,492 ms；界：每个单元 14 s，一遍 330 s，2,300 MiB。两条观察：上限改回 1 万时 10 个单元 10.8–11.1 s（中位数 10.9 s），一遍 109.1 s，守卫是绿的；不删那 5,000 页时 40 个单元单调上升，3.0、3.6、4.4 … 27.6、28.1 s，一遍 645 s，最慢的单元过 14 s、守卫红（另三条种子的计数断言失败，是实验去掉了删除）。
  - 读（父节点多的形状计时，原来的形状只核对计划）：

    | 状态与计划 | 第一轮 Left / 合成（ms） | 第二轮 Left / 合成（ms） |
    |---|---|---|
    | 从未分析，自定义 | 370 / 963 | 362 / 968 |
    | 从未分析，通用 | 365 / 964 | 358 / 961 |
    | 分析过，自定义 | 361 / 1,011 | 365 / 978 |
    | 分析过，通用 | 369 / 987 | 376 / 992 |
    | 只有别的升级，自定义 | 338 / 984 | 330 / 973 |
    | 只有别的升级，通用 | 368 / 989 | 369 / 975 |
    | 种子（其中计时的那本） | 14,818（2,443） | 14,690（2,461） |

    界：`RollbackLeft` 1.2 s（3 × 376 ms），合成的读 3.1 s（3 × 1,011 ms）。合成的读最慢一格 1.01 s（B 估约 0.85 s；换形状之前同一份 `NOT EXISTS` 在旧形状上没有单独量）。第三轮在把读种子挪到核对之后跑，Left 349–369、合成 957–1,009 ms，不进界的计算。守卫全程本机 43.9 s（原来 40.5 s，B）。
  - `make test` 重跑里：先删单元最大 2,700 ms、中位数 2,634 ms，一遍 105.7 s，737 MiB；读 Left 329–373 ms，合成 958–1,004 ms，种子 14.9 s。
- **审查者 B 的实测**：
  - 收着者多时的 `RollbackFreeForest`（`b/probe-holders.txt`）：1 万个文件夹各 10 页、100 个新页、分析过，经 store 一次 26.8 s，`EXPLAIN ANALYZE` 69.6 s；`NOT EXISTS` 317 ms。守卫原样的种子上：分析过的状态里 735 → 245 ms，另两种 224 → 210、237 → 227 ms。
  - 先删的单元（本机，先删守卫原样的种子，十个单元，秒）：照原样 14.2, 23.9, 14.8, 11.6, 11.4, 11.6, 11.5, 11.5, 11.4, 11.5（一遍 133.5 s）；关掉所有表的 autovacuum、第 1 个单元之后只分析 `changeset_items`、`page_search_queue`：14.4 … 70.6（一遍 456.5 s，中位数 50.7 s）；同样关掉、另分析 `nodes`、`page_contents`：17.9, 12.1, 11.0, 9.6, 10.7, 11.5, 11.5, 11.5, 11.4, 11.5（一遍 119.0 s）。
- **e2e**（不带竞态）：建成时 `make e2e` 509 个（2.3 分钟，138 秒；S2 时 507 个，多的是 UP3 的两个）；审查 C 在头上 509 个（132 秒），UP3 两个 0.24、0.72 秒；修复一轮重跑 509 个，132 秒。

## 组合根的各份

照 3.8 以竞态构建列出（`go test -race -list`，各份照 `Makefile` 的 `-run`、`-skip`）：建成时 542 个顶层测试，七份 158、51、121、124、62、11、15，每个恰在一份；比 S2 之后的 541 个多 `TestUpgradeRollback` 一个，在第七份；三个成本守卫带 `//go:build !race`，不进竞态构建。修复一轮照决定 245 挪份之后仍是 542 个，七份 145、51、121、124、62、11、28，第七份里 13 个 `TestABackground…`、14 个 `TestUpgrade…` 与 `TestIngestBefore`；第四份跳过第一份的模式，照旧跳过它们，第二、三、五、六份的模式不收 `TestA…`。

本机各份单独、竞态下，经 `heavy.sh`（`make -j1 test-go-bootstrap-N` 依次两轮，包的时间，秒）：

| 份 | 建成时（`9b572167`）第一次 | 第二次 | S2 时（`b2c65a37`） | 修复一轮之后（`9c0a0f42`） |
|---|---|---|---|---|
| 第一 | 295.0 | 292.0 | 288.3、286.4 | 231.0、230.3 |
| 第二 | 262.3 | 244.8 | 262.7、243.2 | — |
| 第三 | 264.4 | 249.2 | 267.3、249.8 | — |
| 第四 | 225.2 | 202.1 | 223.9、208.5 | — |
| 第五 | 262.4 | 248.3 | 263.0、245.8 | — |
| 第六 | 194.3 | 182.7 | 184.5、187.0 | — |
| 第七 | 116.4 | 120.6 | 95.6、89.6 | 168.9、171.1 |

- 建成时 14 次都通过（负载 0.8–2.3）。第七份多了约 26 秒（`TestUpgradeRollback` 与加长的交错、流）；第一份多 4–6 秒，S3 在这一份只改了 `awaitBackground` 多等一个 River 种类。审查 C 在头上量第一份 292.7 秒。修复一轮之后第一份少约 63 秒、第七份多约 52 秒（C 量的那 13 个约 56 秒）；C 一轮 237.4、167.0 秒（负载 1.4–1.8）。
- CI 上七份的测试一步：建成（run 42635761979）446、443、409、300、396、354、211 秒（第一至三份 EPYC 9V74，第四份 9V45，第五至七份 7763；包 414.0、409.4、378.3、278.5、363.2、323.1、180.1 秒），第一、二份离线 4、7 秒（S3C-M3）；`main` 上第一份的测试一步 442（7763，42593177104）、367（8573C）、363（9V74）、288 秒（9V45），第二份在 7763 上 436、431 秒。修复一轮（run 42671683550 第三次 attempt）371、437、423、377、401、325、211 秒（第一、二、三、五、六份 7763，第四份 9V74，第七份 9V45；包 339.0、404.2、390.6、348.5、368.4、292.3、187.2 秒）；第二次 attempt 里建出的六份 241、378、417、—、276、208、267 秒。第七份在 7763 上没有实数，照两种比例估约 287–300 秒。修复第二轮（run 42688339794）356、348、419、296、282、279、199 秒。都在 450 秒（7:30）之下，第二份在 7763 上 437 秒离线 13 秒，照决定 245 合并之后看两次 `main`。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**（建成时，实施者的 `m11/tools/percommit-s3.sh`，一个干净的临时工作树、`pnpm install --frozen-lockfile`，经 `heavy.sh`）：5 个提交各自 `go build ./...`、`go vet ./...`、`make lint-go`、`make gen-check`、`make test-go-short`；碰 `api/`、`web/` 或 `e2e/` 的 1、3、4、5 号另跑 `make lint-web`、`make knip`、`make test-web`。改写之前的五个 545 秒，第二次改写之后的 4、5 号 222 秒，第三次之后的 232 秒，每一项 rc=0、每一步之后工作树干净；vitest 189 个文件、4,121 个测试。审查 C 另在临时工作树里对 5 个提交各跑一遍（1–4 号另在竞态下跑各自碰到的包，2–4 号另跑组合根的相关测试），都通过。
- **建成时的头**（`9b572167`）：`make lint gen-check knip build` 9 秒通过；`make test` 第一次 420 秒失败（第四份 M9 的 `…/a_place's_parent_deleted_between_batches`，负载约 8.7，make 随之停下；S3C-M4），之后单独 4 次（`-race -count=4`，34 秒）与第四份单独两轮都通过；`make test-go-short` 44 秒；`make e2e` 509 个 138 秒；`make test` 第二次（单独跑）3,311 秒通过：竞态、组合根七份、成本三份（回滚的三个守卫在界内）、lint 的成本、守卫两份、vitest。df：开始前 /tmp 11G、/ 37G 可用，七份之后 11G、36G。
- **修复一轮**（`9c0a0f42`，`m11/s3-fix1/gates/`）：5 个提交各自在干净的工作树里 build、vet、`make lint-go`、`make gen-check` 与 short 都通过，提交 4 碰了网页、另过 `make lint-web`、`make knip`、`make test-web`，每个之后工作树干净。头上 `make lint gen-check knip build` 通过；经 `heavy.sh`：`make test` 第一次 378 秒在第四份的 `…/a_place's_parent_moved_under_its_node_between_batches` 失败（S3C-M4 的同一机制；`-j6` 在它失败后没再起别的份），重跑完整的一次 3,293 秒通过（Go 全部 ok，组合根七份都 ok，网页 189 个文件、4,122 个测试，costs 各行都过）；`make test-go-short` 40 秒；`make e2e` 第一次 508 个过、1 个败（`l1-links.spec.ts:178`，负载约 10），重跑 509 个 132 秒。工作树干净。审查 C 的修复核对开跑前 `df -h /` 可用 30–31G，A 的 29G。
- **修复第二轮**（`de554eda`，决定 242；`m11/s3-fix1/fix2-chain.sh`，经 `heavy.sh`）：提交单独在干净的工作树里 build、vet、`make lint-go`、`make gen-check`、`make test-go-short` 都是 rc=0，之后工作树没有改动；头上 `make lint gen-check knip build` 通过；`page/domain` 的包测试 60.8 秒通过；其余由分支 CI 担保。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 作业 | 42635761979（建成，`9b572167`） | 42671683550（修复一轮，`9c0a0f42`，第三次 attempt） | 42688339794（修复第二轮，`de554eda`，第二次 attempt） |
|---|---|---|---|
| server (race) | 10:38（10:13） | 13:47（13:25） | 13:56（13:31） |
| server (bootstrap-1) | 7:48（7:26） | 6:30（6:11） | 6:16（5:56） |
| server (bootstrap-2) | 7:44（7:23） | 7:35（7:17） | 6:05（5:48） |
| server (bootstrap-3) | 7:11（6:49） | 7:28（7:03） | 7:18（6:59） |
| server (bootstrap-4) | 5:18（5:00） | 6:36（6:17） | 5:15（4:56） |
| server (bootstrap-5) | 6:55（6:36） | 7:07（6:41） | 5:00（4:42） |
| server (bootstrap-6) | 6:14（5:54） | 5:50（5:25） | 4:58（4:39） |
| server (bootstrap-7) | 3:49（3:31） | 3:53（3:31） | 3:39（3:19） |
| server (costs-1) | 17:17（13:43） | 17:11（13:40） | 12:54（9:56） |
| server (costs-2) | 16:44（16:18） | 16:51（16:11） | 16:38（16:15） |
| server (costs-3) | 18:08（17:48） | 11:06（10:47） | 13:28（13:02） |
| server (costs-lint) | 10:39（10:16） | 7:50（7:31） | 13:06（12:44） |
| server (guards-core) | 13:05（12:42） | 17:31（17:06） | 11:29（11:07）（第一次 3:43 红） |
| server (guards-sources) | 16:47（16:24） | 17:07（16:44） | 17:00（16:38） |
| server (short) | 3:22（2:57） | 4:05（3:44） | 3:21（2:58） |
| web | 4:57（4:20） | 4:56（4:19） | 3:14（2:47） |
| e2e | 8:19（6:53） | 8:16（7:05） | 7:39（6:03） |
| image | 1:27 | 1:20 | 1:21 |

三次都全绿，18 项，都是普通的推送（第二次是 `9b572167..9c0a0f42`，第三次是 `9c0a0f42..de554eda`）。42671683550 的前两次 attempt 有作业没有建出来（第一次只建出 short、costs-1、guards-sources 三个，都绿；第二次建出 14 个 server 作业与 web，缺 bootstrap-4 与 costs-3，e2e、image 被跳过），两次都判 failure，是 GitHub Actions 的事故，`gh run rerun` 之后第三次全绿。42688339794 的第一次 attempt 17 项过，guards-core 红在 M7 的 markdown 时长比守卫 `TestTheAppsMarkdownCostsAboutItsSize/to_a_page`（`platform/markdown/markdowntest/costs.go:187`，"a block formula left open"：512 KB 12.72 ms，是四分之一的 1.45 ms 的 8.76 倍，线是 8 倍；EPYC 9V74），e2e、image 被跳过；`gh run rerun --failed` 只重跑那一项，guards-core 过，e2e、image 也过，18 项全绿，实施者与 B（修复核对第二轮）都判为与 S3 无关：S3 的提交没有碰 `platform/markdown`，这是共享 runner 上毫秒级的比，只超出 0.04 ms 的余地。

- 建成时 costs-3 在 EPYC 7763 上 18:08（`ci.yml` 的时限 35 分钟，约 1.9 倍），回滚的守卫约是本机的 2 倍：计划中位数 1,069 ms（界 2 s）、527 MiB；读 `RollbackLeft` 645–688 ms（界 1.3 s），合成的读 1,747–2,968 ms（界 4.6 s，约 1.55 倍），种子 28.1 s（这本 5.2 s）；先删一遍 259.2 s（界 550 s）、单元中位数 21.4 s（界 39 s）、746 MiB，单元 26,695–43,014 ms，第 2–3 个 43.0、39.6 s。
- 修复一轮 costs-3 在 EPYC 9V45 上 11:06（测试一步 647 秒）：读 `RollbackLeft` 471–652 ms（界 1.2 s，1.84 倍），合成的读 1,472–1,785 ms（界 3.1 s，1.74 倍），种子 19.9 s（计时那本 3.3 s）；先删 40 个单元 2,807–3,075 ms（中位数 2,872，界 14 s，4.6 倍），一遍 115.5 s、737 MiB（界 330 s、2,300 MiB，2.9 倍）；计划 794 ms、527 MiB。包的时间：计划 25.8 s，读 41.0 s，先删 106.4 s。这次不在 7763 上，7763 上的数合并之后看。
- 修复第二轮的 costs-3 13:28（测试一步 13:02）；这一轮只改注释与 `page/domain` 的一个单元测试，实施报告没有另读这一次守卫的数。
