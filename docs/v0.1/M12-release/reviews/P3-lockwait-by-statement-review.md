# M12/P3 旁支 `m12-lockwait-by-statement`（日志的一个时刻的测试等它自己的读，不再等到启动时的清理）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-lockwait-by-statement`，从 `main` 的 `0918ad73`（把 L1 的旁支的合并记进文档的提交）开始（主持者的决定 548）。实施的 2 个提交，审查时在 `3d61fc8e`：新加 `pgtest.WaitForTableLockWaitsBy`，只数在跑 sqlc 的某条语句、在等这张表的锁的后端，它与 `WaitForTableLockWaits` 共用一条查询；P3/S1 的 `TestTheLogReadsOneMomentAcrossARevert` 的五格各等它的读停下的那一条语句 `7b3ac149`；M10 的 `TestVirtualRead/rename_snapshot` 等 `ChangesetFacts` `3d61fc8e`。<br>修复一轮在它之上 2 个提交，头 `974c7fa2`：空的语句名立即失败，注释写明批量发出的语句不说明读走到哪里，`WaitForTableLockWaits` 的注释点名 `awaitJob` 的做法，日志的测试的两处注释 `aee229af`（LWA-N1、N2，LWB-Q1，LWC-N1、N2）；virtual 那一格的注释照实写，`PathsOf` 改为 `LinkTargetsByIDs` `974c7fa2`（LWB-M1，LWC-N3）。<br>修复核对之后：只改注释的 2 个，`5fb535a8`（LWA-N3、LWC-N6）、`9f322c1d`（LWB-N2 = LWC-N5），头 `9f322c1d`。<br>到 `9f322c1d` 4 个文件（`pgtest/lockwait.go`、`pgtest/lockwait_test.go`、`bootstrap/history_snapshot_test.go`、`bootstrap/interleavings_virtual_test.go`），加 140 行、减 18 行；只改测试与测试的帮手，不改产品 |
| 审查方式 | 查因者（之后也是实施者）在 `main`（`7ce00077`，与 `0918ad73` 只差文档）的临时工作树里复现，用 `go test -overlay` 加探针、试改法（phase 1、phase 2），之后在 `nerve-wiki-m12-lockwait` 里实施。三位审查者并行，各在自己的分离的工作树里（`nerve-wiki-m12-p2-rev-a`、`nerve-wiki-m12-p2-rev-c`、`nerve-wiki-m12-p4s5-rev-b`，挪到 `3d61fc8e`），重的都经 `heavy.sh`；不碰实施者的工作树：<br>A 帮手 `pgtest`：sqlc 的文本在 pgx 的各条路上是否以 `-- name: X` 开头，有没有包装器改它；正则；`pg_locks` 与 `pg_stat_activity` 两次快照之间会不会错认；共用一条查询之后原帮手的计数与失败的信息；帮手自己的测试与变异（探针 `TestReviewAModes`、`TestReviewARegex`、`TestReviewASkew`、`TestReviewABatches`，给两个测试临时插桩）。<br>B 原因与同类：只读模块缓存里 River 与 rivershared v0.47.0 的代码；`RunOnStart` 的周期任务碰哪些表；在 `0918ad73` 与 `3d61fc8e` 上加先等上锁的外来者各 20 次；服务刚起来时谁在等 `page_revisions` 的探针 10 次；逐处核对同类的清单与别的等锁帮手；virtual 那一格。<br>C 测试、提交与文档：每一格等的语句是不是读停下的那一条，七条名字的变异，READ COMMITTED 的产品变异，`7b3ac149` 单独的门禁，两个组合根测试各 30 次，提交说明、后缀与文档的去处 |
| 日期 | 2026-11-17–18（查因、实施、审查、修复一轮、修复核对一轮） |
| 结论 | Important 0。Minor 1：virtual 那一格的注释把没发生的事写成必然，说去掉过滤时改名会在读的快照之前提交，与去掉过滤 20/20 绿相反（LWB-M1）。Nit 7（LWA-N1、N2，LWB-N1，LWC-N1–N4），Q 2（LWB-Q1、LWC-Q1）；分开是 A 0I 0M 2N，B 0I 1M 1N 1Q，C 0I 0M 4N 1Q。三位都确认：原因的核实部分成立（服务一起来，清理就去等 `page_revisions`，被原帮手数到）；改法有效（同一个外来者在 `main` 上让 100 格里 98 格红，改后 100 格全绿）；每一格等的正是读停下的那一条；产品改坏（READ COMMITTED）时两个测试照样红。代码一侧由修复一轮 2 个提交改，LWB-Q1、LWC-Q1 由主持者定，文档一侧在本提交。修复核对一轮：A 0I 0M 1N，B 0I 0M 1N，C 0I 0M 2N（LWB-N2 = LWC-N5，去重 3 条）；第一轮的都关上，LWB-Q1 的 05-lessons 一半与 LWC-N4 的带负载一半交本提交；新的 Nit 由只改注释的 `5fb535a8`、`9f322c1d` 改，提交说明的不准在"更正" |

## 起因

- **旁支 `m12-submit-deadline-log` 的分支 CI**：提交 `ad68ca6a`，run 44078438315 第一次 attempt，`server (bootstrap-7)` 一份红（job 134022409608）：

  ```
  --- FAIL: TestTheLogReadsOneMomentAcrossARevert/MCP's_log (0.06s)
      history_snapshot_test.go:112: MCP's log across R = C reverted by [01a1e95e-…], R listed true; want the moment of its first read: neither
  ```

  那个提交只改 `wiki/app` 的测试，bootstrap 包编译不到它。只重跑失败的作业，第二次 attempt 全绿。
- **来历**：这个测试是 M12/P3/S1 加的（[P3 设计](../03-P3-server-correctness.md) 3.8.3，2026-11-16 合并 `f3e5a6de`），此后在 `main` 的 CI 上没见它红过。
- **早先的怀疑**：P3/S1 的审查 B 当时已疑到这一点：`pgtest.WaitForTableLockWaits` 数这个库里任何在等 `page_revisions` 的后端，有后台读也在等时，测试会在读开始之前写 R；"只会是假红、不会是假绿；四次运行都稳定，没有复现"，记为接受（[S1 的审查记录](P3-S1-small-correctness-review.md) `:60`，本提交给它加后注）。
- **这一次**：主持者另开查因（设计工作目录的 `m12/revert-snapshot/brief.md`）。查清是测试的问题之后，照 P1 与 L1 的旁支的先例开旁支修（决定 548）。

## 定因

### 核实的

- **测试怎样定先后**：持锁的事务 `LOCK TABLE page_revisions IN ACCESS EXCLUSIVE MODE`，在 goroutine 里发读，等到"有一个后端在等这张表"，再以 SQL 写撤销 C 的 R、提交，放开锁。读的快照在它的第一条读开始时冻结（P3 设计 3.8.2），只要等到的是读停在快照里第二条或更后的读上，R 就在快照之后。原帮手 `pgtest.WaitForTableLockWaits` 数的是这个库里任何在等这张表的关系锁的后端，不管它是谁。
- **外来的等锁者是服务启动时的清理**（审查 B 读代码）：
  - River 的周期任务的入队者拿到 leader 之后，先 `StaggerStart` 睡 0–1 秒（rivershared v0.47.0 `riversharedmaintenance/river_shared_maintenance.go:128-134`，river v0.47.0 `internal/maintenance/periodic_job_enqueuer.go:313-319` 调它），再一次插入 `RunOnStart` 的任务。只有 `TestOnly` 关得掉它，应用的 `river.Config`（`platform/jobs/jobs.go:115-123`）没设。
  - `RunOnStart` 的 `platform.purge_soft_deleted` 共 18 条语句（审查 B 按模块数），碰 `page_revisions` 的 5 条都在 `page/adapter/postgres/queries/purge.sql`：`PurgePageRevisions`、`PurgeNodeLeaves`、`PurgeChangesets`、`PurgeTrimmedChangesets`、`PurgeChangesetSources`。没有可删的行也照跑，语句里引用了这张表就要它的关系锁。别的启动任务不碰这张表；审查 C 另核外键（`page_revisions` 只引用 `changesets` 与 `nodes`，都是 `ON DELETE CASCADE`，这两张表对 `notebooks` 都是 `RESTRICT`），会等它的恰是这 5 条。
  - 测试配置里清理的间隔是 1 小时（`bootstrap/app_test.go:88`），测试里只在启动时跑这一次。
- **服务刚起来时谁在等 `page_revisions`**（审查 B 的探针：`newAcmeTeam` 之后立刻持锁，每 5 ms 看一次；`3d61fc8e` 上 10 次，核对时 `974c7fa2` 上又 5 次）：15 次都只有清理的 `PurgePageRevisions`，出现在 team 建好之后 0.44–3.39 秒，当时 `platform.purge_soft_deleted` 在 `running`。这个测试的五格就落在这段时间里。
- **次数**（查因者，都是 `-race`，经 `heavy.sh`；改前在 `main` 的 `7ce00077` 上。"带探针"是在 R 的 INSERT 里记下那一刻在等 `page_revisions` 的后端，探针在等锁之后，不改等锁之前的先后；审查 C 的 LWC-N4、核对 B）：

  | 树 | 负载 | 次数 | 红 |
  |---|---|---|---|
  | `main` | 没有 | 450（phase 1 的 150，phase 2 base 三轮 300） | 2（`MCP's log of a path`、`REST's getChangeset`） |
  | 带探针的 `main` | 没有 | 300（phase 2 diag 三轮，1,500 格） | 1（`MCP's changeset`） |
  | `main` | 8 个 `yes` | 150（phase 1） | 0 |
  | 带探针的 `main` | 8 个 `yes` | 150（phase 1，750 格） | 0 |
  | 改后（phase 2 的 overlay） | 没有 | 300（fix 三轮） | 0 |
  | 改后 | 有 | 150（fix-load） | 0 |

  改前没有负载 750 次红 3 次，有负载 300 次红 0 次；改后 450 次红 0 次。
- **唯一带诊断的那一次红**（diag，`MCP's changeset`，测试开始之后 1.08 秒）：写 R 的那一刻，等 `page_revisions` 的只有清理的 `PurgeTrimmedChangesets`；读还在 `FindNotebook`，在它的快照之前。带探针的 2,250 格里，凡是绿的格，写 R 时读都已在它自己的语句上等锁；其中 6 格旁边另有清理的一条语句也在等。
- **确定地造出来**：外来者是一个普通的 `SELECT count(*) FROM page_revisions`，在持锁之后、发读之前发出，先用原帮手确认它已在等锁，holder 放开之后再收它。
  - 查因者（phase 2，20 次）：去掉语句的过滤（即改前）每一格都红，100/100；带过滤 20 次全绿。实施的门禁里在分支上又跑一遍，结果相同。
  - 审查 B（overlay，`-race -count=20`）：`main`（`0918ad73`）上 20 次全红，100 格红 98 格（`MCP's log`、`MCP's log of a path`、`MCP's changeset` 各 20/20，REST 的两格各 19/20），红的信息与 CI 那一次同形；`3d61fc8e` 上 100/100 格绿，核对时 `974c7fa2` 上同样。B 的 98 格与查因者的 100 格不矛盾：有两格是读赶在写 R 之前开始了快照，外来者在场时原来的先后只靠快慢。
- **virtual 那一格**（`TestVirtualRead/rename_snapshot`，M10）：它的 team（`virtualReadTeam`）也没等完清理，原帮手同样会数到外来者（审查 B 读代码）。可它没红过：同样的外来者、去掉过滤，`main` 上 20/20 绿（查因者与审查 B 各一次）。查因者的探针说明为什么：测试在发出读之后 1.3–1.6 ms 就往下走，那时等锁的只有外来者；改名走 MCP，发出读之后 12–18 ms 才返回，那时读已在 `ChangesetFacts` 等锁，快照早已开始。所以没有过滤时这一格的先后靠"改名比读慢"，不是靠等待。审查 B 读代码同意：`ChangesetFacts` 是这次读在快照里第一条碰 `page_revisions` 的语句（`page/app/read_path.go:153-167`、`page/virtual_files.go:104-121`），名字在全部 SQL 里只有一处。
- **坏的产品上照样红**（审查 C）：把 `WithinSnapshot` 的 `pgx.RepeatableRead` 改成 `pgx.ReadCommitted`（`postgres/tx.go:143`），语句等完锁之后才取快照：历史的五格全红（`C reverted by [R], R listed false`，正是 H5 说的不一致），virtual 一格红（`one read mixed paths across the rename`）。改后的等待没有把产品的问题一起等掉。

### 推断

- CI 那一次与本机另两次没有诊断的红（`MCP's log of a path`、`REST's getChangeset`），也是清理先等上、被数到。核实的只是机制，以及时间窗与那几格相合（审查 B）；那几次没有诊断。`7b3ac149` 的说明已照实写成推断。

### 结论

答复确实在一个快照里，是测试的问题，不是产品的问题：原帮手数到启动时的清理，测试就在读走到快照之前写下 R，读看到的是 R 之后的一刻。

## 同类

实施者逐处查了在跑着的应用里用 `pgtest.WaitForTableLockWaits` 的地方（行号是 `0918ad73` 的），审查 B 逐处读代码核实：

| 处 | 会不会碰到 | 改不改 |
|---|---|---|
| `history_snapshot_test.go:94`（P3/S1） | 会，核实（上面的次数与外来者） | 改，`7b3ac149` |
| `interleavings_virtual_test.go:147`（M10） | 会数到外来者，核实；没红，原因见上 | 改，`3d61fc8e`（决定 549） |
| `asset_blobs` 的 10 处：`interleavings_transfer_test.go:203`、`:227`、`:267`，`interleavings_import_test.go:116`、`:155`、`:255`、`:302`、`:350`，`interleavings_search_test.go:312`，`interleavings_history_test.go:281` | 不会：10 处都经 `transferTeam`，它持锁之前先 `awaitJob` 等完 `asset.sweep_orphan_files` 与 `platform.purge_soft_deleted`（`interleavings_transfer_test.go:44-45`，等到 `completed`）；之后清理 1 小时、sweep 24 小时才再跑，别的启动任务不碰这张表；带 `betweenUnits` 的四个测试每一轮先等完自己的导入与请求 | 不改 |
| `ingest_virtual_error_test.go:224`（`changeset_sources`） | 不会：`:175` 已等完清理，之后按 pid 断言 | 不改 |
| 迁移代价的测试（`ingest_migration_cost_test.go:611`、`:648`，`lint_index_migration_cost_test.go:596`），linking 适配器的两处（`lint_test.go:300`、`lint_body_mode_cleanup_test.go:110`） | 不会：没有 River 或应用（`:596` 在播种的应用停了之后才交错） | 不改 |

**别的帮手**（审查 B 读代码）：都没有同样的问题。

- `WaitForLockWaits`（任何锁）20 处，都在不引 River、不起应用的适配器或模块测试里。
- `WaitForLockWaitsOn`（某张表的行锁）：启动任务里锁行的语句都带 `SKIP LOCKED`，不等行；外键的动作只在真删到行时才有，新库里没有过了保留期的行；把行做老的测试，要么等完清理，要么让行在启动时仍在保留期内。
- `WaitForKeyWaitOn` 两处（`page_search_queue`、`workspace_invitations`）：测试里没有注册搜索的任务，也没有启动任务写这两张表。
- `WaitForAdvisoryLockWaits` 按键数，搜索的任务只用 try 的锁。
- 手写的三处探针：`wiki_lint_runtime_test.go:343-348` 按 pid、并以 `a.query LIKE '-- name: AttachmentInfos%'` 认语句，是从 `pg_stat_activity.query` 读 sqlc 的名字的早先先例；`revert_job_test.go:77` 看谁被自己持着的 `nodes` 行挡住，清理碰 `nodes` 的语句带 `SKIP LOCKED`；`ingest_cleanup_test.go:311` 是反向的检查。

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `3d61fc8e` 的。修复一轮照主持者的修复说明（`m12/revert-snapshot/fix-1.md`）在 `3d61fc8e` 之上加 2 个提交 `aee229af`、`974c7fa2`，普通推送、不改写已推的历史，每个单独为绿；修复核对之后的 Nit 由只改注释的 `5fb535a8`、`9f322c1d` 改；文档的发现在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| LWA-N1 | 细节（注释） | `lockwait.go:111-116` 说测试等到读自己的语句，就知道读已走到那里：对 pgx 批量发出的语句不成立。探针（事务里一个批量发 `[First, Second]`）：没缓存时 pgx 先把批里的语句一起 Parse，Second 已在等锁、First 还没跑；简单协议下整串显示成第一条的名字。两个测试等的读都不是批量 | `aee229af`：注释补一句，批量发出的语句不说明读走到哪里。核对 A：关上；写出的原因比实情宽，另记 LWA-N3 |
| LWA-N2 | 细节（帮手） | `WaitForTableLockWaitsBy` 收到空的语句名就是"任何语句都算"，不失败，悄悄退回这条旁支要修的行为；今天两处调用都不会漏填 | `aee229af`：空名先 `t.Fatal("pgtest: no statement")`，自测加 `unnamed` 一格（这时有五条语句在等这张表）。核对 A：by-empty-allowed、by-empty-returns 红，关上 |
| LWB-M1 | 中（注释） | `interleavings_virtual_test.go:147-149` 说数到别的等待时"the rename would commit before the read's snapshot began"：去掉过滤、外来者在场，查因者与 B 各 20/20 绿，改名一次也没有赶在读的快照之前；这一格靠的是改名比读慢。推断写成了必然，且与数据相反 | `974c7fa2`：写成数了别的等待，测试会在读走到之前往下走，改名落在读的快照之后只因为它比读慢；现在等读自己的语句。核对 B：关上；新文字里的 "would" 另记 LWB-N2 |
| LWB-N1 | 细节（提交说明） | `7b3ac149` 的说明说清理 "reads page_revisions in most of its statements"：18 条里 5 条，"most" 只对 page 模块的 7 条成立 | 不改写已推的说明，见"更正"。核对 B、C：主持者摘的写法（"约 18 条里只有 5 条"，五条的名字）照实 |
| LWB-Q1 | 问 | 持锁之前先 `awaitJob` 等完启动的任务是另一种做法，`transferTeam` 自 M7/P5A（`55ba1074`）起就这样做：05-lessons 与帮手的注释要不要都提，免得读的人以为 `asset_blobs` 的十处也该改 | 采纳（决定 550）：`aee229af` 的 `WaitForTableLockWaits` 注释点名它；05-lessons 那一条在本提交。核对 B：注释一半关上，注释说的与 `transferTeam` 一致 |
| LWC-N1 | 细节（注释） | `history_snapshot_test.go:19-22` 的 "that statement" 最近的先行词是括号里的第一条读，照字面读说反了 | `aee229af`：写成 "the test waits for the read's wait at that later statement, by its name (waits)"。核对 C：关上 |
| LWC-N2 | 细节（注释） | `:98-100` 的 "counted, R would be written before the read's first read"：只有外来者先等上时才这样；带探针时有 6 格清理在读旁边等着，照样绿 | `aee229af`：would 改 could。核对 C：关上 |
| LWC-N3 | 细节（注释，M10 留下的） | `interleavings_virtual_test.go:151-152` 的 `PathsOf`：这次读取路径用的是 `LinkTargetsByIDs`（`page/virtual_files.go:157`），sqlc 里没有叫 `PathsOf` 的语句 | `974c7fa2`：改名。核对 C：关上 |
| LWC-N4 | 细节（提交说明） | `7b3ac149` 的说明 "On main (7ce00077), without load, it failed 3 times in 750 local runs …, none in 300 under load"：750 与 300 里都含带探针的运行 | 不改写，本记录"定因"的表分开列，见"更正"。核对 B、C：主持者摘的只分了没有负载的一半，带负载的也要分；本记录两半都分 |
| LWC-Q1 | 问 | 后缀：第 7 节"旁支"一条写"它所属的 Step 或 Phase"；实施者援的先例 `m12-p1-navigation-settle` 写的是 Phase（`4fe6e523`）；主持者说明里建议的 `(M12/P3/S1 story, found in M12)` 不在规则里。合并与文档提交写哪一种（C 倾向 `(M12/P3)`） | 主持者定 `(M12/P3/S1)`，改 virtual 那一格的写 `(M10 story, found in M12)`（决定 551）。核对 C：关上 |
| LWA-N3 | 细节（注释，修复核对） | `aee229af` 的 "pgx parses the batch's statements before the first runs" 只对这条连接上还没准备过的语句成立：缓存过的、或 `exec` 模式的批量按次序走（探针各模式一张表）。结论"批量的语句不说明读走到哪里"仍对 | `5fb535a8`：写成 "pgx may parse a batch's statements before the first runs (those not yet prepared on its connection)" |
| LWB-N2 = LWC-N5 | 细节（注释，修复核对） | `974c7fa2` 的 virtual 注释 "counted, the test would go on before the read had begun"：只有清理先等上、被数到时测试才提前（同 LWC-N2 的 could）；查因者的探针只说明读还没走到它的等待，没说读还没开始 | `9f322c1d`：写成 "could go on before the read reached its wait"；说明里的同一句见"更正" |
| LWC-N6 | 细节（提交说明、注释，修复核对） | `aee229af` 的标题 "refuses no statement's name" 与测试注释 "No statement's name fails at once"，照英语的字面读作"什么名字也不拒绝" | 测试注释由 `5fb535a8` 改成 "An empty statement's name fails at once"；标题见"更正" |

## 主持者的决定

负责人可以改判；正文在 [00-M12](../00-M12-design.md) 第 11 节"2026-11-18：等锁帮手的旁支的决定"。

- **548 是测试的问题，开旁支修，照常审查与合并；测试按 sqlc 的语句名认出读停下的那一条，不加长等待**（主持者的查因说明与实施说明）。开旁支同 P1 与 L1 的旁支（决定 356、359、541）。
- **549 M10 的 `rename_snapshot` 随同一条旁支改**，同类的别处不改（审查 B 核实）。
- **550 采纳 LWB-Q1**：帮手的注释与 05-lessons 写另一种做法，持锁之前 `awaitJob` 等完启动的任务（`transferTeam`，M7/P5A `55ba1074`）。
- **551 后缀**（LWC-Q1）：实施的第一个提交、改 pgtest 与日志的测试的修复与注释提交、文档的提交与合并写 `(M12/P3/S1)`；改 virtual 那一格的写 `(M10 story, found in M12)`。
- **552 第 12 节的链接与合并号放在 P3 一行**。
- **553 文档随本提交**：05-lessons 第 3 节一条；总体设计 13.4 第 4 条一句与第 15 节一行；P3 设计 3.8.3 第 3 步与 P3/S1 的记录 `:60` 照决定 481 的做法加后注。
- **554 修复核对之后不开第二轮**：新的 Nit 由 `5fb535a8`、`9f322c1d` 改，与本提交一起推送；已推的说明不改写，不准处写在"更正"。

## 实施中定的

查因与实施的报告没有落成文件（工具不让写），主持者摘在设计工作目录的 `m12/revert-snapshot/host-facts.md`、`host-facts-fix1.md`。

1. **按语句名认，不改用 `awaitJob`**：等的是读本身，别的等锁者（今天的清理，以后加的启动任务）都不算，不必知道服务启动时跑哪些任务、它们碰哪张表；`awaitJob` 作为另一种做法写进注释与 05-lessons（决定 550）。
2. **两个帮手共用一条查询**（`waitForTableLock`）：`WaitForTableLockWaits` 是语句名为空的情形，原帮手的查询因此多连一张 `pg_stat_activity`。审查 A 核过它的计数与失败的信息不变（见"反向对照与变异"末尾 A 的探针）。
3. **语句名逐格写在表里**：`history_snapshot_test.go` 的表多一个字段 `waits`，五格按位置写，漏一项编译不过（审查 A 的 LWA-N2 里核过）。
4. **推送之前用 `commit-tree` 只换说明，树相同**：`95a3989b` → `7b3ac149`，`19012a29` → `3d61fc8e`（树 `383b1c5c…`、`33add7c3…`，审查 C 核过）；门禁跑在原来的两个提交上。门禁里的对照显示 virtual 那一格去掉过滤照绿，说明随之照实改写。
5. **后缀**：实施者写 `(M12/P3/S1)` 与 `(M10 story, found in M12)`，没用主持者说明里的 `(M12/P3/S1 story, found in M12)`，照 00-M12 第 7 节"旁支"一条；合并与文档的由主持者在 LWC-Q1 之后定（决定 551）。

## 接受与推后的

- **接受**：
  - 原帮手 `WaitForTableLockWaits` 留着，`asset_blobs` 的十处与另几处照用它：持锁之前已等完启动的任务，或没有 River（"同类"）。
  - 批量发出的语句不说明读走到哪里，帮手的注释写明；今天两个测试等的读都不是批量（`postgres/planned.go:41-43` 写明批量只用来写行；sqlc 的批量只有 `WriteSearchRow :batchexec`）（LWA-N1、N3）。
  - by-granted-too、by-no-anchor 两条变异活下来，在用法上等价（审查 A）：调用者持着这张表的 ACCESS EXCLUSIVE，别的后端拿不到它的锁，持锁者自己的语句没有名字；sqlc 的文本总以名字开头。整个 `NOT l.granted` 由原帮手的测试守住（沿代码走出，没有跑）。
  - by-empty-check-after 活下来，无害（核对 A）：守卫挪到计数之后，空名的调用照样过不去；只是 `unnamed` 一格的 `n=1` 小于正在等的五条，没钉住"立即"。改成大于 5 可以钉住，不改。
  - virtual 那一格：过滤之后靠等待；改名本身不碰 `page_revisions`，若碰，持锁一方不放手，测试就会挂住（审查 B、C）。
- **未核实**：
  - CI 那一次与本机另两次没有诊断的红是清理造成的（"定因"的推断）。
  - 审查 A：今天的计划先读 `pg_lock_status`、后读活动视图，一个正在等锁的后端读到的 `query` 必是它等着的那一条（探针 24,808 次 0 次错认）；计划若改成先读活动视图，空闲连接的错认在原理上可能，那种次序没有造出来。真要防，加一句 `a.wait_event_type = 'Lock'` 就够，不改。
  - 审查 B：探针里同时有六个任务在 `running`，默认队列的 `maxWorkers` 是 2，没有追查，不影响结论。
- **不改的文档**：
  - P3 设计 10.1 是 S1 当时的结果，不改；3.8.3 第 3 步加后注（决定 553）。
  - M10 的文档：virtual 那一格只改了测试的等待；M10 故事的旁支（`642f9196`、`8635667a`）也都没改 M10 的文档（审查 C）。
  - 移交 `M8-history.md:27` 只说这个交错有、不说怎样等；`06-environment.md` 与此无关（审查 C）。
  - 00-M12 第 10 节没有这一类的风险。

## 更正

已推送的说明不改写（先例 `642f9196` 的"更正"），在这里写对：

- `7b3ac149`：
  - "reads page_revisions in most of its statements"：清理共 18 条语句，碰 `page_revisions` 的 5 条，`PurgePageRevisions`、`PurgeNodeLeaves`、`PurgeChangesets`、`PurgeTrimmedChangesets`、`PurgeChangesetSources`（LWB-N1）。同一句的 "(0 to 1 s after its leader is elected)" 是插入任务的时刻；清理等上这张表，审查 B 量到是 team 建好之后 0.44–3.39 秒。
  - "On main (7ce00077), without load, it failed 3 times in 750 local runs …, none in 300 under load"：750 次是 `main` 450 次（红 2 次）加带探针的 `main` 300 次（红 1 次）；300 次是 `main` 150 次加带探针的 150 次，都没红。探针写在 R 的 INSERT 里，在等锁之后，不改等锁之前的先后，所以数仍是同一种竞态的数（LWC-N4）。
  - "Counted, R is written before the read's first statement"：只有清理先于读等上、被数到时才这样，6 格里清理在读旁边等着照样绿；注释由 `aee229af` 写成 could（LWC-N2）。
- `3d61fc8e` 的 "Counted, the test goes on before the read has begun" 与 `974c7fa2` 的 "It now says the test would go on before the read had begun"：只有清理先等上时测试才提前；查因者的探针只说明读还没走到它的等待，没有说明读还没开始（快照的第一条是 `decideNotebook`）。注释由 `9f322c1d` 写成 "could go on before the read reached its wait"（LWB-N2 = LWC-N5）。
- `aee229af` 的标题 "refuses no statement's name" 读反了：意思是"空的语句名立即失败"（LWC-N6）。同一个提交说明里的 "pgx parses the batch's statements before the first runs"，只对这条连接上还没准备过的语句成立（LWA-N3）。

## 反向对照与变异

帮手的变异用 `mut.py`，命令 `go test -count=1 -run 'TestWaitForTableLockWaits' ./internal/platform/postgres/pgtest/`；组合根的经 `heavy.sh`。都先在没改的树上跑过同样的命令。

**实施时**（实施者）：

| # | 变异 | 结果 |
|---|---|---|
| wrongname | 带路径的日志一格写成 `ListChangesetsOfPages` | 红在 10 秒的时限上：`0 statement(s) waited for the lock of the table page_revisions in ListChangesetsOfPages within 10s, want at least 1` |
| by-filter-dropped | 去掉语句的过滤 | FAILED(good) |
| by-prefix | 只比前缀（`LIKE 'Mine%'`） | FAILED(good) |
| by-any-table | By 时不看表 | FAILED(good) |
| by-message | 信息不带语句名 | FAILED(good) |
| provoke-nofix | 外来者，去掉过滤 | 20 次，100/100 格红 |
| provoke-fix | 外来者，带过滤 | 20 次全绿 |
| virtual，去掉过滤 | 外来者在场 | 20/20 绿（原因见"定因"） |
| virtual，带过滤 | 外来者在场；不带外来者 | 20/20 绿；50/50 绿 |

修复一轮加 by-empty-allowed（去掉空名的守卫），与另四条一起 FAILED(good)。

**审查时**（`3d61fc8e`）：

| # | 谁 | 变异 | 结果 |
|---|---|---|---|
| 上面四条 | A | 自己写的同样四条 | 都 FAILED(good)，独立复现了实施者的 |
| by-ignores-statement | A | By 传 `""` | FAILED(good) |
| any-empty-broken | A | `''` 改成 `'x'`，原帮手什么也数不到 | FAILED(good)，两个测试都红 |
| by-granted-too | A | By 时也数已拿到的锁 | 活，用法上等价（"接受与推后的"） |
| by-no-anchor | A | 去掉 `^` | 活，等价 |
| 名字 1 | C | 带路径的日志写成 `ListChangesetsOfPages`（实施者的 wrongname） | 红，时限，只有这一格 |
| 名字 2 | C | 带路径的日志写成 `ListChangesets`（一条真语句，是 `ListChangesetsOfPage` 的前缀） | 红，时限 |
| 名字 3 | C | 同上，再加帮手的 by-prefix | 绿（演示：帮手只比前缀时，写错了也照过；自测 `MineToo` 一格因此有用） |
| 名字 4 | C | `changeset` 写成 `FindNotebook`（快照之前那条） | 红，时限 |
| 名字 5 | C | REST 的 `listChangesets` 写成 `FirstChanges`（同一个读里后面一条也碰这张表的） | 红，时限 |
| 名字 6 | C | virtual 写成 `FindNotebook` | 红，时限 |
| 名字 7 | C | virtual 写成 `ListChangesets`（读停下之前刚跑的那条） | 红，时限 |
| READ COMMITTED | C | `WithinSnapshot` 用 `pgx.ReadCommitted` | 历史五格全红（`C reverted by [R], R listed false`），virtual 红（`one read mixed paths across the rename`） |
| 外来者 | B | 见"定因" | `0918ad73` 上 98/100 格红，`3d61fc8e` 上 100/100 格绿；virtual 两边都 20/20 绿 |

名字写成快照之前的语句只会在时限上红、走不到快照的断言：读在快照之前不碰 `page_revisions`。"等到的是快照之前的等锁者"只有外来者造得出（B 的外来者）。

审查 A 的探针（"核对过、没有问题的"）：

- **服务器看到的文本**：缓存模式没缓存（在 Parse 上等）、已跑过 7 次（在 Bind 上等），CacheDescribe、DescribeExec（即 `Planned`）、Exec，简单协议带参数，`pgxpool` 的事务：等锁时的 `query` 都以 `-- name: Second` 开头，`By(Second)` 数到、`By(First)` 为 0。给两个测试临时插桩：`TestTheLogReadsOneMomentAcrossARevert` 3 次 15 格、`rename_snapshot` 3 次，等着的都是 `client backend`、`active`、`Lock/relation`，`query` 以 `-- name: ChangesetFacts :many` 或 `-- name: ListChangesetsOfPage :many` 开头。没有包装器改 SQL 的文本。
- **正则** `^-- name: (\w+)`：开头有换行、空格、注释，两个空格，`Name` 大写，都认不出（只会到时限红、不会多数）；`Mine_2 :one` 取 `Mine_2`，没有 `:one` 取 `Mine`；`\w+` 是贪婪的，`MineToo` 不等于 `Mine`。与 `pgtest.CachedStatements` 同一个式子（`statements.go:18`）。
- **两次快照之间的错认**：8 个连接循环先跑 `-- name: Mine`、再跑没有名字的等锁语句，27,901 圈；帮手的查询 24,808 次里 0 次数到 Mine，帮手本身试 10 次 0 次返回。
- **原帮手不变**：`TestWaitForTableLockWaits*` 在没改的树上为绿；linking 的两个使用者为绿；`""` 时失败的信息与改前一字不差。

**修复核对**（`974c7fa2`）：A 重跑第一轮的六条都 FAILED(good)，新加 by-empty-allowed、by-empty-returns（`t.Fatal` 换成 `return`）FAILED(good)，by-empty-check-after（守卫挪到计数之后）活、无害；C 重跑实施者的四条加 by-empty-allowed 都 FAILED(good)，自己的九条结果同第一轮，另把 REST 的 `listChangesets` 一格写成 `""`：0.06 秒红在 `history_snapshot_test.go:102: pgtest: no statement`，不等满时限；B 的外来者 100/100 格绿、virtual 20/20 绿。

## 核对

**修复核对一轮**（2026-11-18）：三位把工作树挪到 `974c7fa2`，只看 `3d61fc8e..974c7fa2`。

- **A**：LWA-N1、N2 关上（变异见上）；自测 `-race -count=30` ok（26.3 秒），新的一格在所有等待者都排上之后才调用，不改先前的先后；`WaitForTableLockWaits` 补的 `transferTeam` 一句与 `interleavings_transfer_test.go:34-46` 一致。能核的那部分 LWB-N1、LWC-N4 照实（`page/adapter/postgres/gen` 里碰 `page_revisions` 的 Purge 语句正是那 5 条；"约 18 条"的总数 A 没核）。新的 Nit LWA-N3。
- **B**：LWB-M1、N1 关上；Q1 的注释一半关上，05-lessons 一半交本提交。外来者在 `974c7fa2` 上同第一轮；启动探针又 5 次，等着的仍只有 `PurgePageRevisions`（0.64–3.39 秒）；清理的语句共 18 条（asset 2、transfer 2、page 7、wiki 1、notebook 3、workspace 3）。LWC-N4 带负载的一半也要分开；05-lessons 那一条的时间写"几秒之内"，不写 0–1 秒（那是 River 睡的时间，不是测试看到的时刻）。新的 Nit LWB-N2。
- **C**：LWC-N1、N2、N3 关上，Q1 由主持者定、关上，N4 部分（带负载的一半交本记录）。两个提交各自单独为绿；变异照预期；外键核对，会等这张表的恰是那 5 条；两个提交的说明照实，只有 LWC-N5 那一句。新的 Nit LWC-N5、N6。

修复核对之后：`5fb535a8`（LWA-N3、LWC-N6）、`9f322c1d`（LWB-N2 = LWC-N5）改注释；不开第二轮（决定 554）。

## 门禁与 CI

- **门禁**（本机 arm64，经 `heavy.sh`）：

  | 头 | 结果 |
  |---|---|
  | `95a3989b`、`19012a29`（实施者；`commit-tree` 换说明之前，树同 `7b3ac149`、`3d61fc8e`） | 每个提交 `go build ./...`、`go vet ./...`、`make lint-go`、`make test-go-short` rc=0；两个组合根测试 `-race -count=50` 100/100；`make test-go` 3,651 秒，185 个包 ok；provoke 前后与 wrongname 见上 |
  | `7b3ac149`（审查 C） | 单独 `go build`、`go vet` rc=0，`make lint-go` rc=0（两个模块 0 issues，`go mod tidy -diff` 干净），`make test-go-short` rc=0（94 个 ok） |
  | `3d61fc8e`（审查） | C：`TestTheLogReadsOneMomentAcrossARevert` `-race -count=30` 30/30（150 格，36.6 秒），`TestVirtualRead/rename_snapshot` 30/30（30.8 秒）；A：pgtest 的自测 `-race -count=30` ok（26.2 秒） |
  | `aee229af`、`974c7fa2`（实施者） | 每个提交四项 rc=0；pgtest `-race -count=30` 60/60；两个组合根测试 `-race -count=30` 60/60 |
  | `aee229af`、`974c7fa2`（核对 C） | 每个提交四项 rc=0（94 个 ok）；pgtest 两个测试各 30/30；两个组合根测试 `-race -count=10` 10/10 + 10/10 |
  | `5fb535a8`、`9f322c1d` | 只改注释；头 `9f322c1d` 上 `go build`、`go vet`、`make lint-go` rc=0，`5fb535a8` 没有单独跑 |

- **分支的 CI**（普通推送，没有改写已推的历史；主持者核实）：

  | 推送 | 头 | `ci.yml` | 结果 |
  |---|---|---|---|
  | 1 | `3d61fc8e` | 44163333532 | 19 项第一次 attempt 全绿 |
  | 2 | `974c7fa2` | 44176852719 | 19 项第一次 attempt 全绿 |

- `5fb535a8`、`9f322c1d` 没有单独推送，分支 CI 在本提交上跑。
- **本提交自己的分支 CI**：`735d306e` 的 run 44199995744，19 项第一次 attempt 全绿（bootstrap-7 4:29，e2e 8:43）。旁支合并为 `5d782ff8`，之后第一次 `main` 的 run 44204397259 同样全绿（bootstrap-7 4:26，race 9:25，e2e 9:42）。

## 环境

查因在 `~/.cache/m12-revert-snapshot/` 下的临时工作树（`main` 的 `7ce00077`）里，改法与探针经 `go test -overlay` 加进去；实施在 `nerve-wiki-m12-lockwait`；审查与核对各在 `nerve-wiki-m12-p2-rev-a`（A）、`nerve-wiki-m12-p2-rev-c`（B）、`nerve-wiki-m12-p4s5-rev-b`（C），挪到 `3d61fc8e`，核对时 `974c7fa2`。重的都经 `heavy.sh`，一个脚本跑完连着的几项，每项之前看 `df`；`TMPDIR`、`GOTMPDIR` 与日志放在 `~/.cache/m12-lockwait*` 下，用完删掉。B 的外来者与探针经 overlay 加进去，不改工作树的文件；A、C 的变异由 `mut.py` 改文件、从 `.mutbak` 复原，探针与插桩删掉；审完各工作树干净。C 用 `GOPROXY=off` 与主仓库 `bin/` 里的 golangci-lint 2.14.0。本机 aarch64（Parallels 虚机），`pgtest` 的容器是 `postgres:18.7-trixie`，Go 1.27.4，River v0.47.0，pgx v5.11.0。B 只读模块缓存里 River 的代码，没有上网；CI 的结果经 `gh` 只读。要留的在设计工作目录的 `m12/revert-snapshot/`（`host-facts.md`、`host-facts-fix1.md`、`review-*.md`、`check1-*.md`）。
