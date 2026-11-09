# M11/P1 冻结、发起与回滚：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M11/P1 冻结、发起与放弃（四个 Step：S1 数据、冻结与删除的护栏（服务端）；S2 发起与读（服务端）；S3 放弃、回滚与结束（服务端）；S4 网页） |
| 日期 | 2026-11-08（初稿；同日照设计审查第一轮修订；照 S1、S2 修订 2026-11-09） |
| 状态 | 进行中（S1 已合并 `27b30c7f`，S2 已合并 `（待填）`；S3 待开工）；设计已审查，[审查记录](reviews/P1-design-review.md)，修订核对第一轮在它的末节；S1 的审查记录 [P1-S1-freeze-guardrails-review.md](reviews/P1-S1-freeze-guardrails-review.md)，S2 的审查记录 [P1-S2-start-and-read-review.md](reviews/P1-S2-start-and-read-review.md)；实施与审查之后与代码不同之处照实改在各节（3.25、3.26 列出），结果在 10.1、10.2 |
| 上级文档 | [M11 总设计](00-M11-design.md) 4.1–4.5、4.6 的"删除"、4.10、4.12、4.14–4.18、第 5、7、8、9 节与"移交的落实"；[M11 设计审查](reviews/M11-design-review.md)；[移交](handoffs/)：M10-mode、M5-events、M8-P4-background、M9-mcp-upgrade；[v0.1 总体设计](../v0.1-design.md) 5.8、7.2、12.4、13.1–13.4；[M10/P1](../M10-llm-wiki/01-P1-contract-frame.md)（写法） |
| 代码位置的写法 | 同 00 号文档：模块写作 `page/app/unit.go:368`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00048_notebook_notebooks_mode.sql:9`（`server/migrations/sql/`），迁移的测试写作 `server/migrations/schema_test.go:163`；前端写作 `W/stores/notebook.store.ts:45`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/fixtures/notebooks.ts:25`；契约写作 `api/modules/notebook.yaml:462`。代码的行号是 `main` 的 `9184f67b` 的（代码与 `7ff74a67`、`d9169baa` 相同，其后只改了文档），00 号文档的行号也是它的 |
| 工作笔记 | 代码的核对分六份（notebook、page 的单元与守卫、transfer、撤销的计划、wiki 与 lint 与组合根、网页与 e2e 与 CI），本文引的每个行号都在其中核对过、关键的几处又逐行复读过（附录）。P1 的设计不跑重的东西；要量的在 7.6 |

---

## 0. 分四个 Step

照 00 号文档第 7 节，P1 分四个 Step，各自一个分支（`m11-p1-s<m>`）、一次审查与修复核对、一次合并；后一个从前一个合并之后的 `main` 开始。每个 Step 里的每个提交都绿（服务端的测试、网页的五步、e2e），每个服务端 Step 带上它改的契约在网页与 e2e 上要的改动（生成的类型、假服务端的字段、码与种类的两种语言的文字、钉住旧行为的测试），不留到 S4（第 4 节）。

- **S1 数据、冻结与删除的护栏（服务端）**（`m11-p1-s1`）：
  - 迁移 00055（`notebooks` 的升级四列与 `notebooks_upgrade_check`）、00056（`upgrade_nodes`、`changesets.upgrade_id`）、两个 sqlc 项、`schema_test` 的名称与每个析取项的反例、两个 Down 的测试、`runtime-grants.sql` 的两行（3.2）；
  - notebook 的 `Upgrade`、六条读笔记本的查询多选四列、根的 `State.Upgrade`、`Notebook.upgrade`（契约）、模块根导出 `ActionChangeMode`（3.3）；
  - page 的 `NotebookState.Upgrade`、`Write.UpgradeOwn`、`UnitSpec.UpgradeOwn`、`domain.FreezeAdmits`、`page.NewFreezeGuard`（守卫与否决者）登记在最前、删除的护栏与 `page.upgrade_protected`、`ErrStaleUpgradeWrite`、`ensureChangeset` 记 `upgrade_id`、三处 page 的预检（3.4）；
  - transfer 的笔记本端口交回升级、导入的预检、导入建任务与 `startBackground` 的判断（3.5）；
  - `notebook.upgrading` 进 14 个 REST 操作与 5 个 MCP 写工具、`upgradeToolCodes()`、`mcpOwnCodes()`（3.6）；`events.yaml` 的两处说明、日志（3.7）；第七份（3.8）；
  - 网页：生成的类型、两处 `Notebook` 字面量的 `upgrade: null`、`notebook.upgrading` 的文字（4.1；`page.upgrade_protected` 只在 MCP，文字是服务端的 `detail`）。
  - 单独为绿：没有发起的接口也测得到冻结。整个程序的测试以 SQL 设上升级的列与基线的行（同 M10/P1/S2 先建守卫的骨架），`checkPages` 的升级规则在 S2 才加（wiki 的表那时才有），所以 S1 的夹具没有 wiki 的行也不违反任何已有的规则。合并之后 `main` 上的笔记本永远没有升级（没有写它的路），冻结的守卫对它们在第 1 步就放过，行为不变。
- **S2 发起与读（服务端）**（`m11-p1-s2`）：
  - 迁移 00057（wiki 的第一张表）、wiki 的 sqlc 一项与 `wiki/adapter/postgres`、授权、清理器、笔记本删除的订阅者（3.10、3.15）；
  - `notebook.NewUpgradeStates` 的 `Lock`、`Start`、`Publish`（3.11）；
  - page 的基线（`Capture`、`Protect`、`SetWanted`、`Analyze`）、按笔记本的会话的两条查询、根下的键、`SeedSpec.Upgrade`（3.12）；
  - wiki 的 lint 拆出 `computeIn` 与只准备不投影的 `prepareIn`、`Runs.TryHold`、linking 的按页的待建数与来源区里基线页的分段读的第二遍（3.13）；
  - `startUpgrade`、`getUpgrade`、五个码、`common.yaml` 的 `locks` 的说明（3.14）；`checkPages` 的升级规则，S1 的夹具补上 wiki 的行（3.15）；
  - 网页：五个码的文字（4.2）；e2e：UP1（API、MCP）、UP2（API）、UP4 的删除一步（MCP）。
  - 单独为绿：合并之后能发起而还不能放弃（00 号文档第 7 节"大小与绿"、第 207 条：只在开发中的 `main` 上，网页还没有入口）。`getUpgrade` 答的 `Upgrade` 在 S2 已有回滚的几格，都为 `null`（3.14）。
- **S3 放弃、回滚与结束（服务端）**（`m11-p1-s3`）：
  - 迁移 00058（`changesets` 的 `rollback`）、00059（`transfer_jobs` 的 `rollback`、`upgrade_id`、写入键）（3.17）；
  - page 的回滚：先删的划分、合成的计划、`RunBatch` 的回滚一支、剩下的差（3.18）；
  - transfer 的 `rollback`：种类、River、Rescue 的种类表、按遍的运行、`finish` 的两个钩子、`Rollbacks()`、`Cancel.Run` 的规则（3.19）；
  - wiki 的 `abandonUpgrade`（含继续）、`endUpgradeRollback`、`wiki.NewRollbackEnd`、`wiki.upgrade_state`；`UpgradeStates` 的 `SetState`、`Clear`（3.20）；契约（3.21）；
  - 网页：新种类与码的文字、两份 `writingKinds`、回滚报告的计数与失败文字（4.3）；e2e：UP3（API）。
  - 单独为绿：从这里起发起的升级都能出去（放弃、继续、结束回滚），网页的入口在 S4。
- **S4 网页**（`m11-p1-s4`）：只读收成一处与控件表、编辑器失去编辑、横幅、发起的对话框、升级页的状态与接入的说明、放弃、回滚的进度与继续、结束、事件与重连、任务页的"取消"（4.4）；e2e：UP1–UP3 的页面版本，`threeBrowsers` 加 `upgrade\/up1-`（第 5 节）。

## 1. 基线

从 `main` 的 `d9169baa` 开始（M11 总设计被接受、总体设计照它改过的文档提交；移交的处理进展在其后的 `9184f67b`，只改文档）。

- **笔记本的读**：读笔记本的列的查询有六条：`FindNotebook`、`LockNotebook`（`notebook/adapter/postgres/queries/notebooks.sql:9-24`）、`ShareNotebook`（`:26-34`，只选 `id, mode, source_root_id`）、`ListNotebooks`（`:55-68`，自己列 `n.mode, n.source_root_id`）、`LockOwnerlessOf`（`cascade.sql:37-45`）与 `ListOwnerless`（`ownerless.sql:1-9`）。`LockOwnerlessOf` 与 `LockNotebook` 的行以 `gen.FindNotebookRow(r)` 转换（`notebook/adapter/postgres/cascade.go:57`、`store.go:77`），所以列必须相同；`ListOwnerless` 逐字段造 `FindNotebookRow`（`ownerless.go:20-24`）；`ListNotebooks` 逐字段造 `domain.Notebook`（`store.go:146-151`）。根的 `State{Mode, SourceRootID}`（`notebook/notebooks.go:30-33`）由 `ShareState`（`:111-121`）与 `LockState`（`:123-133`）给出；`domain.Notebook`（`notebook/domain/notebook.go:32-47`）要保持可比较（`Apply` 以 `!=` 比较，`:135`）。答 `Notebook` 的六个操作都经 `notebookOf(v app.View)`（`notebook/adapter/http/handler.go:161-172`）。
- **组合根交给各模块的笔记本端口**：page 经 `pageNotebooks`（`bootstrap/page_notebooks.go:15-38`，`toPageState` 在 `:33-38`）；transfer 直接拿 `notebook.NewNotebooks(pool)`（`bootstrap/deps.go:243`），它的端口只有 `WorkspaceOf`、`ShareByID(bool)`、`NameOf`（`transfer/app/ports.go:30-40`）。
- **写入单元**：`Writer.run`（`page/app/unit.go:220-287`）在事务里先工作区行 `ShareByID`（`:246-248`），再笔记本行（`:249-256`，`Tree` 时 `LockByID`），判定（`:257-260`），读一次时钟（`:261-263`），造 `Write`（`:264-268`）。`applyStep`（`:357-404`）里守卫的循环在 `:368-372`，在领域的检查之后；`refusalOf`（`:333-339`）默认标 `s.Changes[0]`。`ensureChangeset`（`:452-475`）在第一个写的步上建变更集（`:467-470`）。`Write`（`page/app/extension.go:27-47`）带 `Notebook domain.NotebookState`（`:39`）与 `Grants`（`:43`）；`NotebookState`（`page/domain/mode.go:17-20`）只有模式与来源根。
- **守卫的登记**：`pageRegistrants(pool, authorizer, logger)`（`bootstrap/registrants.go:260-273`）的 `guards` 与 `sessionVetoers` 都是 `[mode, lock]`（`:268`、`:270`）；注释（`:253-255`）已写着"M11's freeze vetoes an opening"。`Grants.Ask`（`page/app/grants.go:29-49`）每个单元每个动作至多问一次，注释（`:15-16`）只列模式的守卫与单元自己的检查。
- **删除**：`DeleteSubtree`（`page/app/unit_delete.go:42-66`）的步列出整棵子树（`:46-53`）；`Store.DeleteNodes`（`page/adapter/postgres/store.go:447-457`）要求从上往下删（漏掉的中间节点是缺陷）。删除没有大小的界（`MaxUnitNodes = 10,000` 只管恢复，`page/domain/trash.go:57`、`page/app/jobs.go:714`）。
- **编辑会话**：`sessions.sql` 只有按页的 `DeleteExpiredSessionsOf`（`:32-36`）与按页的 `AliveSessionsOf`（`:23-30`），没有按笔记本的；索引 `edit_sessions_notebook_id_idx`（`00017_page_edit_sessions.sql:28`）在。
- **预检**：`AssetWrites.Check`（`page/app/asset_writes.go:49`，`Allowed` `:50`，`StateOf` `:57`）、`JobWrites.CheckRestore`（`page/app/jobs.go:118`，`Allowed` `:119`，`StateOf` `:140-146`）、`CheckRevert`（`:168-170`，`Allowed` `:180`，状态在共用的 `revertChecks.check` 里读，`page/app/revert_changeset.go:248-254`）、`StartImport.Check`（`transfer/app/start_import.go:67-99`，判定在 `:75`）。
- **transfer 开任务**：导入建任务的事务（`transfer/app/start_import.go:248-272`：`lock` `:249`、`LockQueue` `:255`、`room` `:258`、`writing` `:261`、`CreateJob` `:268`）；`startBackground`（`transfer/app/start_background.go:154-198`：`lock` `:169`、`LockQueue` `:172`、`writing` `:178`、`CreateJob` `:184`、"queued" 的日志 `:192`）；共用的 `StartDeps.lock`（`transfer/app/start.go:105-113`）导出也用。
- **后台任务**：`Background.Run`（`transfer/app/background.go:54-74`）以 `nil` 的 `succeed` 调 `finish`（`:73`）；`backgroundRun.run`（`:86-128`）按种类分派（`:93-102`）；`jobRun.finish`（`transfer/app/job_run.go:181-227`：`succeed` `:189-191`、`errGone` `:196-198`、`FinishJob` `:211`、`!ok` `:215-218`）。恢复与撤销进导入的队列（`transfer/adapter/river/jobs.go:202` 的 `QueueImport`），`riverKind`（`:206-217`）。`import_workers: 1`（`server/configs/config.yaml:121`）。心跳每秒一次（`transfer/module.go:185-188` 的 `beat`，`Deps` 改不了它）；`BeatJob` 只更新 `running` 的行（`jobs.sql:75-81`），写失败只记一行 warn、照跑（`job_run.go:148-160`），读到不再运行就 `stop(errGone)`（`:164-166`）；`errGone` 不导出（`:32-33`）。Rescue 的 `InterruptJobs`（`jobs.sql:120-132`）把行记为 `failed`、失败 `interrupted`。`batch` 的 `retry` 每次调 `Tree.RunBatch` 之前先看 `running`（`background.go:146-152`），批的单元本身不看它。`transfer.New`（`transfer/module.go:200-238`）对缺的依赖不 panic；它的调用处是 `bootstrap/wire.go:212`、`transfer/contributors_test.go:85` 与 `transfer/adapter/http/module_background_test.go:37`。
- **lint**：`(*Lint).compute`（`wiki/app/lint.go:198-272`）在 `WithinSnapshot`（`:201`）里读状态、读时钟（`:209`）、算；`Runs`（`wiki/app/lint_runs.go:17-29`）的 `Do`（`:62-124`）、名额的 `slots`（`:139-145`）、`finish`（`:168-204`，`active--` `:198`）、`Stop`（`:221-257`）。linking 的 `prepareWanted`（`linking/app/lint_edges.go:23-87`）按发出页走，`seen` 是一页的不同目标（`:63`），流只有区外的页（`:29`、`lint.sql:10-24`）。
- **测试的设施**：`TestTheGuardRefusesWhatThePlanLetBy`（`bootstrap/interleavings_wiki_test.go:304`）以 `wiring{page: func(d *page.Deps) { d.Guards = nil }}` 换空守卫（`:344-345`）；`TestTheModeGuardReachesEveryPath`（`bootstrap/wiki_guard_test.go:104`）与 `TestModeComesBeforeTheLock`（`:203`）不换列表（只在注释里说，`:102-103`、`:201-202`）。`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:89`，写的五行 `:95-106`），`mcpOwnCodes()`（`bootstrap/mcp_client_test.go:338-340`）。组合根分六份（`Makefile:247-266`），`test-go` 列出六份（`:190`）。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P1 一行（`:514`）：冻结的数据与守卫、删除的护栏、发起与读、放弃、回滚与结束回滚、P1 的网页。

不做（P2、P3）：删除之外的护栏（不改写链接的移动、正文的比较、`batch` 的折进、账本的维护）、闸门、检查、提交与两个 MCP 工具（P2）；批准、驳回、退回、来源哈希的重建、第 181 条、prompt 与 README（P3）。P1 的 `notebook_upgrades` 已有提交与报告的列（00057 一次建全，3.10），P1 不写它们。

**移交的落实**（00 号文档"移交的落实"里 P1 的部分）：

| 移交的项 | P1 的落实 |
|---|---|
| [M10-mode](handoffs/M10-mode.md) 1 契约表与模板 | S2：发起补写的页取 `wiki.TemplatePages`（3.14 第 12 步） |
| 同上 2 先写行、在同一事务里通知 | S2：`UpgradeStates` 的写之后 `Publish`（3.11）；S3 的放弃、结束、停下与做完同样 |
| 同上 3 沿用 `mode` | S1：`events.yaml` 的说明（3.7）；S2：`TestUpgradeStatesReachTheStreams` |
| 同上 5 冻结沿用单元的边界 | S1：`Grants.Ask`、`refusalOf`、`batch` 与 transfer 的去处照旧（3.4） |
| 同上 P3/S1 补充：显式范围、零 UUID、事务、名额与期限、`stale_pages` | S2：`computeIn` 的显式 LLM Wiki 契约与提议的根（根从不为零）、`TryHold`、`ctxStart`、`wiki.upgrade_stale_index`（3.13、3.14） |
| 同上 收尾补充：守卫的次序、`Write.Notebook` | S1：`[freeze, mode, lock]`（3.4） |
| [M5-events](handoffs/M5-events.md) 模式变化的事件 | S2 起每个转移发 `mode`；`TestUpgradeStatesReachTheStreams` |
| [M8-P4-background](handoffs/M8-P4-background.md) 1、2、4 | S3：一个回滚任务、先删、合成的计划、`MaxRollbackItems`、继续就是重算（3.18、3.19）；S1：`UpgradeOwn`（3.4） |
| [M9-mcp-upgrade](handoffs/M9-mcp-upgrade.md) 2、3 | S1：发起人的 MCP 写与 `UpgradeOwn`；S3：合成的条目照用 `PlanRevert` 与 `schedule` |

### 2.1 对 00 号文档的修订

照代码，00 号文档有下面几处要改（第 8 节在接受本文的文档提交里一并改；行号是 `9184f67b` 的）：

1. **六条查询，不是四条**（第 2 节 notebook 一条 `:29`、4.5"输入" `:156`、第 7 节 P1/S1 `:514`、第 8 节 `:648`）：`LockOwnerlessOf` 的行以 `gen.FindNotebookRow(r)` 转换（`notebook/adapter/postgres/cascade.go:57`），列不同就编译不过；`ListOwnerless` 逐字段造同一个行（`ownerless.go:20-24`），不选四列它造出的 `domain.Notebook` 的升级就是零，与同一行别的读不一致。"四条查询多选这四列（`ListNotebooks` 在内）"改为"六条读笔记本的查询多选这四列（`ListNotebooks`、`LockOwnerlessOf`、`ListOwnerless` 在内）"（3.3）。
2. **`UpgradeStates.Lock` 只锁笔记本行**（4.1 的表 `:76`、4.4 第 3 步 `:131`、第 8 节 `:648`）：notebook 的 SQL 读不到 `workspaces`（sqlc 的一项只列自己的迁移，`archtest/sqlc_test.go:260-271`），notebook 的管理写也是经端口先锁工作区（`notebook/app/manage.go:26-41`）。所以工作区行由 wiki 经自己的 `Workspaces` 端口先锁（组合根交 `workspace.NewWorkspaces(pool)`，同 transfer 的 `lock`，`transfer/app/start.go:105-113`），再调 `Lock`；`NewUpgradeStates(pool, subscribers)` 的签名不变。"`Lock`（工作区行 `FOR SHARE`、笔记本行 `FOR NO KEY UPDATE`，交回带升级的状态）"改为"`Lock`（笔记本行 `FOR NO KEY UPDATE`，交回带升级的状态；工作区行 `FOR SHARE` 由调用方先经它的工作区端口锁）"（3.11）。方法随它们的第一个用处加：`Lock`、`Start`、`Publish` 在 S2，`SetState`、`Clear` 在 S3，`Approve` 在 P3/S1。
3. **transfer 的笔记本端口**（4.5"transfer 开任务时" `:173`、第 8 节 transfer 一条 `:650`）："组合根从 `notebook.ShareState` 转换"今天无处可转：组合根直接交 `notebook.NewNotebooks(pool)`（`bootstrap/deps.go:243`）。S1 加组合根的 `transferNotebooks`，端口的 `ShareByID` 交回 `NotebookState`，另加不加锁的 `StateOf`（导入的预检：`StartImport.Check` 不在事务里，`transfer/app/start_import.go:67-99`）；S3 再加 `LockByID`（回滚结束与停下的事务先锁笔记本行，3.19）（3.5）。
4. **发起时的会话**（4.4 第 7 步 `:135`）：引的 `sessions.sql:32-36` 是按页的（`node_id = …`），page 没有按笔记本删过期的会话或列活着的会话的查询。S2 加两条按笔记本的（`DeleteExpiredSessionsOfNotebook`、`LiveSessionsOfNotebook`，走 `edit_sessions_notebook_id_idx`，`00017_page_edit_sessions.sql:28`）（3.12）。"同开启的第一步（`page/adapter/postgres/queries/sessions.sql:32-36`……）"改为"同开启的第一步（那里按页，`page/adapter/postgres/queries/sessions.sql:32-36`；这里按笔记本，一条新的查询……）"。
5. **回滚的一遍**（4.10"每一遍两段" `:282-284`）：一遍只做两段中的一段。先删：这一遍有不含基线节点的新建子树时，这一遍的计划只有删的步（各步的根两两不相干：`DeleteNodes` 要求从上往下删、一次删整棵，`page/adapter/postgres/store.go:447-457`）；一棵超过 `MaxUnitNodes` 的在这一遍先删它下面不超过界线的极大子树，它自己在下一遍（那时它小了）。没有这样的子树时才合成计划，跑完这一遍的批。下一遍重算差，所以"先删、再计划"由遍的循环保证，不在一遍里分两种步（3.18）。"每一遍两段"改为"每一遍先看有没有要先删的子树：有就只删，没有才合成计划"。
6. **`Upgrade.report` 在 P2/S2 加**（第 5 节 `Upgrade` 的 schema `:452`）：它的 schema `UpgradeReport` 是 P2/S2 的（第 7 节 P2 一行 `:515`），契约里每个必填成员都要有 schema。P1/S2 的 `Upgrade` 不列 `report`，P2/S2 加为必填、可为 `null`（网页的类型与假服务端随之）；其余成员（提交、回滚的各格）P1/S2 就列，S2 时都为 `null`（3.14）。
7. **整个程序的测试"组合根的守卫列表换回 `[mode, lock]` 时失败"**（第 9 节 `TestUpgradeFreezeReachesEveryPath` `:666`）：已有的守卫测试不换列表（`bootstrap/wiki_guard_test.go:102-103`、`:201-202` 只是注释）；换空的写法是 `wiring{page: func(d *page.Deps) { …d.Guards… }}`（`bootstrap/interleavings_wiki_test.go:344-345`）。P1 照它：`TestUpgradeFreezeReachesEveryPath` 带一个子测试把 `d.Guards` 与 `d.EditSessionVetoers`（`page/module.go:190`）里的冻结一项去掉，断言别人的写成功（3.4、7.1）；变异表另有"登记里没有冻结"一条（7.3）。
8. **页面版本里刻意的 409**（第 9 节 e2e 的末段 `:692`）：写法是 `pageWatch.expectConsole({ errors: [failedToLoad(409)] })`（`e2e/stories/workspace/w9-leave-remove.spec.ts:88`），不是 `expectConsole(failedToLoad(409))`。
9. **`threeBrowsers`**（第 9 节 `:692`）：P1 只加 `upgrade\/up1-`（UP7 是 P3/S3 的），正则照 `e2e/playwright.config.ts:3` 的写法（`stories\/(wiki\/lw(…)-|…)`）加一支 `upgrade\/up1-`。
10. **page 的回滚要的动作**（4.10"单元" `:289`）：`UnitSpec.Action` 是 `notebook.change_mode`，page 的 `domain` 里没有这个值（`page/domain` 的动作是 page 自己的，`page/tree_writes.go:56` 的 `ActionRestore` 是先例）。S3 给 page 的 `Deps` 加 `UpgradeAction`，组合根交 `notebook.ActionChangeMode`（同冻结的守卫拿到它的方式），`JobWrites` 经它建回滚的单元（3.18）。第 8 节 page 一条加"`Deps.UpgradeAction`"。
11. **发起的 lint 不经 `Do`，只算它用得到的**（4.4 第 13 步 `:141`"种类是'升级的基线'"、4.7 `:213`"`logFinished`……给新的种类（检查、条目、提交、发起）各一个名字"；P1 审查 B-N3）：发起经 `TryHold` 持名额、在自己的事务里算，没有 flight，`logFinished`（`wiki/app/lint_runs.go:265-322`）不记它，`RunKey` 也用不到。它只用 `stale_pages`、`wanted` 的总数与按页的数：`stale` 来自范围、与规则无关（`linking/app/lint_scope.go:184-187`），`RuleCount` 可以单取一条规则的数（`wiki/app/lint_ports.go:100-105`），而摘要单独在 10 万页上就要 1.7–2.0 s（`bootstrap/wiki_lint_cost_test.go:27-29`，决定 160）。所以 `computeIn` 再拆出 `prepareIn`（读状态、范围与 `PrepareRules`，不投影），发起只调它、只准备 `wanted` 一条规则（3.13）。P1 不加 `RunKey` 的种类，检查、条目、提交三种与 `Scope` 在 P2/S2。4.4 第 13 步的"在这个事务里、笔记本行的锁下调 lint 的 `computeIn`（4.7）一次，种类是'升级的基线'"改为"在这个事务里、笔记本行的锁下调 lint 的 `prepareIn`（4.7）一次，只准备 `wanted` 一条规则、要按页的数"，4.7 的"（检查、条目、提交、发起）"改为"（检查、条目、提交；发起不经 `Do`，不记在这里）"。第 232 条的成本因此比按整份摘要估的宽，7.6 照实量。
12. **回滚不前进时停下**（4.10"分批" `:291`"一遍什么也没写而差不空是缺陷（任务以 `internal` 停下）"；P1 审查 A-M3 改正了初稿的理由）：合成的一遍做完就到基线。撤销与回滚的 `place` 把节点的次序值直接设成计划给的（`page/app/unit_revert.go:149-157`），`moveSubtree` 不重新编号兄弟（`page/app/unit_move.go:75-107`），放到一旁的一步保持它现在的次序值（`page/domain/revert_schedule.go:520`），删除不动兄弟（`page/app/unit_delete.go:34-41`）；兄弟的重新编号（`placeAmong`、`domain.Place`）只在新建、移动、恢复与改位置里（`unit_create.go:66`、`unit_create_asset.go:50`、`unit_move.go:64`、`unit_restore.go:81`、`:404`、`unit_relocate.go:76`），它们造成的"只差次序值"正是合成的条目要放回的。所以今天的代码下"写了而差没变小"到不了；判据仍放宽为"一遍之后剩下的三个数之和不比这一遍之前小（含什么也没写）是缺陷，任务以 `internal` 停下"，作为遍的循环一定结束的保证（以后合成的计划漏了一类差，任务停下、可以结束回滚，而不是一直跑）；测试另断言"先删的几遍加一遍合成"就做完（3.18、7.1），让不收敛先在测试里暴露。00 号文档那一句改为"一遍之后剩下的三个数之和不比这一遍之前小（含什么也没写）是缺陷，任务以 `internal` 停下（终止的保证：合成的一遍本就到达基线，`place` 照计划写次序值、不重新编号兄弟）"（3.18、3.19）。
13. **第二遍的读**（4.7 闸门 6 `:224`"页的集合照 `excluded` 用 `MATERIALIZED` 的 CTE 传入（核对 B3-N3）"；P1 审查 B-I1）：整组页一次传入、在连接之后按（页、位置）的游标排序分页是平方级的：审查者 B 在 PostgreSQL 18 上实测，10 万页里 3 万页在区里、各带 10 条待建（30 万条边），强制自定义计划整条流 18.1 s，通用计划每批 256 ms（约 150 s），自动选择 66.2 s；照 `SourceValues`（`linking/app/lint_reads.go:271-310`）把排好的页每 512 个一段读，同样的边 0.17 s。改为：页照 `LintReadRows = 512`（`:15`）一段，段内由页驱动、走主键（3.13）。那一句改为"页按 id 排好，每 512 个一段读，段内由给定的页驱动 `page_links` 的主键（核对 B3-N3；P1 审查 B-I1 实测整组一次传入是平方级的）"。

### 2.2 需要负责人确认

无。第 11 节的负责人与主持者的决定在本文里都照做；2.1 的十三条是照代码改说法、位置、做法或分到哪个 Step，第 12 条把"什么也没写"放宽为"差没有变小"，都不改那些决定。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

设计审查第一轮里设计者在审查者给的几种做法之间取的（[审查记录](reviews/P1-design-review.md)"主持者的决定"；接受本文的文档提交把它们写进 00 号文档第 11 节）：

- **233　升级已不是任务的那一次时，回滚的任务不写结束**（P1 审查 A-N1、A-M2）：page 的 `ErrRollbackGone` 由组合根的 `treeError` 交回 transfer 导出的 `ErrJobGone`（今天不导出的 `errGone` 改名导出，`transfer/app/job_run.go:32-33`，模块根另有同名的别名），`finish` 照今天的 `errGone` 只记一行"stopped"、不写结束（`:196-198`）。生产上到不了这一行还在 `running` 的情形（结束回滚要求任务不在排队或运行；Rescue 记为中断之后 `FinishJob` 本就是 0 行）；照常写结束要给它一个现有的失败里没有的状态。测试里以 SQL 造出的那一行留在 `running`，测试自己断言并收拾（7.1）。
- **234　交错第 16 条的"心跳写不进"由测试在自己的库里装的触发器逼出**（P1 审查 A-M1）：一个只认这个任务 id、`OLD.state` 与 `NEW.state` 都是 `running` 的 `BEFORE UPDATE` 触发器抛错，`BeatJob` 交回一个不是 `ErrNoRow` 的错误（`job_run.go:148-160` 的那一支），Rescue 的那条 SQL 与 `FinishJob` 改状态、不受它影响。不在 `transfer.Deps` 留只给测试的口子：生产的代码与接线一行不改（7.2）。
- **235　第二遍的一段之内照（页、位置）的游标分批**（P1 审查 B-I1）：一段至多 512 页，每批 `source_id = ANY(这一段里游标之后的页)` 加行比较、`ORDER BY l.source_id, l.range_start LIMIT 512`，由主键的数组扫描按键序交回、不对一段的边排序；不取"一次读一段的全部头"（审查者实测的形状）：一页至多 `MaxLinks = 10,000` 条链接，一段的头最坏是 512 万行，过了 `LintReadRows` 管一批的本意（`lint_reads.go:12-13`）。S2 的计划的测试核对两种计划都没有 `Sort`、走 `page_links_pkey`；做不到时退回审查者实测的形状，并把段缩到最坏也放得下（写进 S2 的记录）。

## 3. 服务端

### 3.1 S1 的文件

| 文件 | 改动 |
|---|---|
| `server/migrations/sql/00055_notebook_notebooks_upgrade.sql`、`00056_page_upgrade_nodes.sql` | 新（3.2） |
| `server/sqlc.yaml` | notebook 一项（`:57-60`）加 00055，page 一项（`:79-123`）加 00056（`archtest/sqlc_test.go:260-271` 要一项列出它的模块的每个迁移） |
| `server/migrations/schema_test.go`；新的 `server/migrations/upgrade_test.go` | 名称、反例、Down（3.2） |
| `deploy/runtime-grants.sql`；`bootstrap/runtime_role_test.go` | `upgrade_nodes` 进数据授权（`:17-20`）与 `MAINTAIN`（`:29-30`）；`analyzed` 表（`:389`）加它（3.2） |
| `notebook/domain/notebook.go`、新的 `notebook/domain/upgrade.go`；`notebook/notebooks.go`、`notebook/module.go` | `Upgrade`、`UpgradeState`；`State.Upgrade`；根的别名与 `ActionChangeMode`（3.3） |
| `notebook/adapter/postgres/queries/{notebooks,cascade,ownerless}.sql`、`store.go`、`ownerless.go` | 六条查询多选四列、`upgradeOf`（3.3） |
| `notebook/adapter/http/handler.go`；`api/modules/notebook.yaml` | `Notebook.upgrade`（3.3） |
| `page/domain/mode.go`、新的 `page/domain/freeze.go`、`page/domain/errors.go` | `NotebookUpgrade`、`FreezeAdmits`、三个错误（3.4） |
| `page/app/extension.go`、`unit.go`、`grants.go`、新的 `page/app/freeze_guard.go`、`ports.go`、`asset_writes.go`、`jobs.go` | `Write.UpgradeOwn`、`UnitSpec.UpgradeOwn`、冻结的守卫与否决者、删除的护栏、`ensureChangeset`、预检（3.4） |
| 新的 `page/freeze_guard.go`；`page/adapter/postgres/queries/upgrade.sql`（新）、`changesets.sql`、`page/adapter/postgres/upgrade.go`（新）、`changesets.go` | 模块根的构造、`Protected` 的读、变更集的 `upgrade_id`（3.4） |
| `bootstrap/page_notebooks.go`、`registrants.go`；新的 `bootstrap/transfer_notebooks.go`、`deps.go` | `toPageState`、`[freeze, mode, lock]`、transfer 的笔记本适配器（3.4、3.5） |
| `transfer/app/ports.go`、`start.go`、`start_import.go`、`start_background.go`、`transfer/domain/errors.go` | 端口交回升级、预检与开任务的判断（3.5） |
| `api/modules/{page,asset,transfer,events}.yaml` | `notebook.upgrading` 的声明；`mode` 的说明（3.6、3.7） |
| `bootstrap/permission_matrix_mcp_test.go`、`mcp_client_test.go`、`mcp_reads_test.go`；新的 `bootstrap/upgrade_*_test.go` | 码的表与整个程序的测试（3.6、7.1） |
| `Makefile`、`.github/workflows/ci.yml`、`docs/v0.1/claude_to_codex/06-environment.md` | 第七份（3.8） |
| `web/packages/api-client/src/schema.gen.ts`、`W/test/fakes.ts`、`W/stores/notebook.store.test.ts`、`W/app/problem-messages.ts`、`W/i18n/messages/{en,zh-CN}.ts` | 4.1 |

### 3.2 S1：迁移 00055 与 00056

```sql
-- 00055_notebook_notebooks_upgrade.sql
-- notebooks: an upgrade to an LLM Wiki under way (M11 design 4.1, 4.2): its id, its state, who started it and when,
-- set together or not at all, and only on a standard notebook. No foreign key on upgrade_id: the upgrades are the
-- wiki module's, which its purger purges leaf to root, and a notebook's row must not hold them (as 00048's
-- source_root_id). A program before this one knows no upgrade: an upgrading notebook reads as a standard one there.

-- +goose Up
ALTER TABLE notebooks ADD COLUMN upgrade_id uuid,
    ADD COLUMN upgrade_state text,
    ADD COLUMN upgrade_by_id uuid REFERENCES users,
    ADD COLUMN upgrade_at timestamptz,
    ADD CONSTRAINT notebooks_upgrade_check CHECK ((upgrade_state IS NULL) = (upgrade_id IS NULL)
        AND (upgrade_id IS NULL) = (upgrade_by_id IS NULL) AND (upgrade_id IS NULL) = (upgrade_at IS NULL)
        AND (upgrade_state IS NULL
            OR upgrade_state IN ('upgrading', 'reviewing', 'rolling_back') AND mode = 'standard'));

-- +goose Down
ALTER TABLE notebooks DROP CONSTRAINT notebooks_upgrade_check, DROP COLUMN upgrade_at, DROP COLUMN upgrade_by_id,
    DROP COLUMN upgrade_state, DROP COLUMN upgrade_id;
```

```sql
-- 00056_page_upgrade_nodes.sql
-- upgrade_nodes: an upgrade's baseline (M11 design 4.2): a row for each node alive as the upgrade started, with its
-- kind, parent, name and order then, and a page's revision, content hash and size, the distinct wanted targets it
-- linked to, and the hash the guardrails last accepted. A row not of the baseline protects the source root the start
-- created: a page at the root, nothing else. No deleted_at and no foreign keys, as edit_sessions and the link
-- index's tables: the rows go as the upgrade ends and as the notebook is deleted, so no purger. changesets.upgrade_id
-- is the upgrade a changeset was written in (no key: the upgrades are the wiki module's).

-- +goose Up
CREATE TABLE upgrade_nodes (
    upgrade_id uuid NOT NULL,
    node_id uuid NOT NULL,
    notebook_id uuid NOT NULL,
    baseline boolean NOT NULL,
    kind text NOT NULL,
    parent_id uuid,
    name text,
    sort_order double precision,
    revision integer,
    content_hash bytea,
    byte_size integer,
    wanted integer,
    accepted_hash bytea,
    CONSTRAINT upgrade_nodes_pkey PRIMARY KEY (upgrade_id, node_id),
    CONSTRAINT upgrade_nodes_kind_check CHECK (kind IN ('page', 'asset')),
    CONSTRAINT upgrade_nodes_baseline_check CHECK (baseline = (name IS NOT NULL) AND (name IS NULL) = (sort_order IS NULL)
        AND (baseline OR kind = 'page' AND parent_id IS NULL)),
    CONSTRAINT upgrade_nodes_page_check CHECK ((baseline AND kind = 'page') = (revision IS NOT NULL)
        AND (revision IS NULL) = (content_hash IS NULL) AND (revision IS NULL) = (byte_size IS NULL)
        AND (revision IS NULL) = (wanted IS NULL) AND (revision IS NULL) = (accepted_hash IS NULL)),
    CONSTRAINT upgrade_nodes_wanted_check CHECK (wanted IS NULL OR wanted BETWEEN 0 AND 10000)
);
CREATE INDEX upgrade_nodes_notebook_id_idx ON upgrade_nodes (notebook_id);
ALTER TABLE changesets ADD COLUMN upgrade_id uuid;
CREATE INDEX changesets_upgrade_id_idx ON changesets (upgrade_id) WHERE upgrade_id IS NOT NULL;

-- +goose Down
DROP INDEX changesets_upgrade_id_idx;
ALTER TABLE changesets DROP COLUMN upgrade_id;
DROP TABLE upgrade_nodes;
```

各列的类型照它们抄的那一列：`sort_order` 同 `nodes.sort_order`（`00012_page_nodes.sql:18-19`），`revision`、`content_hash`、`byte_size` 同 `page_contents`（`00013_page_page_contents.sql:10-15`）。每个检查的每个合取都不因 NULL 放过：`baseline`、`kind` 非空，其余都是 `IS NULL` 的比较；`upgrade_nodes_baseline_check` 的最后一个析取在 `baseline` 为真时就是真，为假时要 `kind = 'page' AND parent_id IS NULL`，两者非空。

**`TestConstraintAndIndexNames`**（`server/migrations/schema_test.go:163`，`want` 的表从 `:191` 起，`notebooks` 的项在 `:314-325`）加：`"notebooks_upgrade_check c"`、`"notebooks_upgrade_by_id_fkey f a"`（照 `:314` 的 `"notebooks_created_by_id_fkey f a"`：`REFERENCES users` 不写动作），`"upgrade_nodes_pkey iu"` 与 `"upgrade_nodes_pkey p"`（主键的索引与约束各一项，S1 偏离 4）、`"upgrade_nodes_kind_check c"`、`"upgrade_nodes_baseline_check c"`、`"upgrade_nodes_page_check c"`、`"upgrade_nodes_wanted_check c"`、`"upgrade_nodes_notebook_id_idx i"`、`"changesets_upgrade_id_idx iw"`（部分索引；每项的写法照表里同类的项，实施时对着 `:191` 起的已有项抄）。

**反例**（`TestChecksRejectCounterexamples`，`schema_test.go:544`，表 `{name, stmt, constraint}` 在 `:553` 起，00048 的三行在 `:603-605`；循环 `:755-763` 要 SQLSTATE 23514）。`checkedRows`（`:463`）原来只种下一本 LLM Wiki（`:478-483`），没有普通的笔记本：加一本升级中的普通笔记本（四列齐全）与它的三行 `upgrade_nodes`（根下的基线页、它下面的基线附件、保护行），另给已有的变更集设上 `upgrade_id`（S1 偏离 1；基线行没有外键，不另种节点行）。每行一个反例，改这本笔记本或这三行之一，每个只钉住一种形状（05-lessons 第 1 节）：

| 名称 | 语句的形状 | 约束 |
|---|---|---|
| 状态有、另三列空 | `UPDATE notebooks SET upgrade_state = 'upgrading'` | `notebooks_upgrade_check` |
| 缺状态 | `SET upgrade_id, upgrade_by_id, upgrade_at`（第一个合取的另一个方向；初稿的"id 有、另三列空"同时违反三个合取，S1 偏离 3） | 同上 |
| 缺发起人 | `SET upgrade_id, upgrade_state, upgrade_at` | 同上 |
| 只有发起人 | `SET upgrade_by_id = …`（另三列空：等式的另一个方向） | 同上 |
| 只有时刻 | `SET upgrade_at = …`（另三列空） | 同上 |
| 缺时刻 | `SET upgrade_id, upgrade_state, upgrade_by_id` | 同上 |
| 不认识的状态 | 四列都有，`upgrade_state = 'approved'` | 同上 |
| LLM Wiki 带升级 | 在 LLM Wiki 的行上四列都有，`'upgrading'` | 同上 |
| 种类 | `upgrade_nodes` 一行 `kind = 'folder'` | `upgrade_nodes_kind_check` |
| 基线没有名称 | `baseline = true`、`name` 与 `sort_order` 都空（只空 `name` 时另违反"名称与次序同空"） | `upgrade_nodes_baseline_check` |
| 保护行带名称 | `baseline = false`、`kind = 'page'`、`parent_id IS NULL`、`name`、`sort_order` 都有 | 同上 |
| 名称有、次序空 | `baseline = true`、`sort_order IS NULL` | 同上 |
| 次序有、名称空 | `baseline = false`、`kind = 'page'`、`parent_id IS NULL`、`name IS NULL`、`sort_order` 有 | 同上 |
| 保护行是附件 | `baseline = false`、`kind = 'asset'` | 同上 |
| 保护行不在根下 | `baseline = false`、`parent_id` 有 | 同上 |
| 基线页缺版本 | `baseline = true`、`kind = 'page'`，版本五列（`revision`、`content_hash`、`byte_size`、`wanted`、`accepted_hash`）都空：只违反第一个合取 `(baseline AND kind = 'page') = (revision IS NOT NULL)`，另四列有值时一行会同时违反五个合取、钉不住它 | `upgrade_nodes_page_check` |
| 附件带版本 | `baseline = true`、`kind = 'asset'`、版本五列都有 | 同上 |
| 缺哈希 | 基线页、`content_hash IS NULL` | 同上 |
| 缺字节数 | 基线页、`byte_size IS NULL` | 同上 |
| 缺待建数 | 基线页、`wanted IS NULL` | 同上 |
| 缺认可的哈希 | 基线页、`accepted_hash IS NULL` | 同上 |
| 保护行带哈希 | 保护行（`baseline = false`），版本五列只有 `content_hash`（另一个方向：`revision` 空而它有） | 同上 |
| 保护行带字节数 | 保护行，只有 `byte_size` | 同上 |
| 保护行带待建数 | 保护行，只有 `wanted` | 同上 |
| 保护行带认可的哈希 | 保护行，只有 `accepted_hash` | 同上 |
| 待建数过界 | 基线页、`wanted = 10001` | `upgrade_nodes_wanted_check` |
| 待建数为负 | 基线页、`wanted = -1` | 同上 |

每个写成 `=` 的合取两个方向各一行，每行只违反一个合取（P1 审查 B-N2、修订核对 B2-N1：把 `=` 改成单向的蕴含，只有一个方向的反例抓不住；00057 的同一条规则见 3.10）。正例由 `checkedRows` 自己的插入给出（四列齐全的普通笔记本、基线页、基线附件、保护行，插入不报错）。共 27 个：`notebooks_upgrade_check` 8 个、`upgrade_nodes_*` 19 个；S1 审查 B 在真库上把每个反例对每个合取求值，各恰好违反一个。

**Down**：新的 `server/migrations/upgrade_test.go` 的 `TestTheUpgradeColumnsGoDown`，照 `TestTheNotebooksModeGoesDown`（`server/migrations/notebooks_mode_test.go:17-65`）与 `downTo`（`background_jobs_test.go:15-26`）：升到最新，种一本升级中的笔记本、它的基线行与一个带 `upgrade_id` 的变更集；`downTo(t, m, 56)` 之后 `changesets.upgrade_id` 与 `upgrade_nodes` 不在、变更集的行在；`downTo(t, m, 55)` 之后 `notebooks` 没有四列、笔记本的行在、模式不变（`downTo` 退掉给定的那一版，S1 偏离 2）；再升回最新成功（`TestMigrationsGoUpDownAndUpAgain`，`schema_test.go:68`，另管全程）。

**授权**：`deploy/runtime-grants.sql` 的数据授权（`:17-20`）的表名加 `upgrade_nodes`；`MAINTAIN`（`:29-30`）加 `upgrade_nodes`，注释加一句"an upgrade's start analyzes its baseline (M11 design 4.4 step 16)"。`TestTheGrantsFileCoversEveryRelationAndFunction`（`bootstrap/runtime_role_test.go:362`）对每张表要恰好的权限集，`analyzed` 表（`:389`）照 `page_search` 的写法加 `"upgrade_nodes": true`：两样少一样它都失败。以运行时角色发起一次、断言 `last_analyze` 变了的那一段在 S2（第一个 `ANALYZE` 在那里，3.15）。

### 3.3 S1：notebook 的升级、查询与契约

```go
// notebook/domain/upgrade.go

// UpgradeState is the state of a notebook's upgrade to an LLM Wiki under
// way (M11 design 4.3): no mode, the notebook standard throughout.
type UpgradeState string

const (
	UpgradeUpgrading   UpgradeState = "upgrading"
	UpgradeReviewing   UpgradeState = "reviewing"
	UpgradeRollingBack UpgradeState = "rolling_back"
)

// Upgrade is a notebook's upgrade under way, the zero value for none:
// its id, state, initiator and start (notebooks_upgrade_check sets the
// four together). Comparable, as Notebook and State are compared whole.
type Upgrade struct {
	ID    uuid.UUID
	State UpgradeState
	By    uuid.UUID
	At    time.Time
}
```

- `domain.Notebook`（`notebook/domain/notebook.go:32-47`）加 `Upgrade Upgrade`；`Apply`（`:135`）的 `!=` 照旧成立（`Upgrade` 只有可比较的字段）。根的 `State`（`notebook/notebooks.go:30-33`）加 `Upgrade Upgrade`；`ShareState`（`:111-121`）、`LockState`（`:123-133`）、`StateOf`（`:93`）填它。`notebook/notebooks.go` 加 `Upgrade`、`UpgradeState` 与三个值的别名，挨着 `State`、`Mode`（S1 偏离 5；初稿写在 `notebook/module.go` 的别名里），`notebook/module.go` 另导出 `const ActionChangeMode = domain.ActionChangeMode`（`notebook/domain/actions.go:20`；组合根交给 page 的冻结守卫，S3 也交给 page 与 transfer 的回滚，S2 交给 wiki）。
- 查询：`FindNotebook`、`LockNotebook`（`notebooks.sql:9-24`）、`LockOwnerlessOf`（`cascade.sql:37-45`）在 `mode, source_root_id` 之后加 `upgrade_id, upgrade_state, upgrade_by_id, upgrade_at`（三条同列，`gen.FindNotebookRow(r)` 的转换照旧成立）；`ShareNotebook`（`:26-34`）改为 `SELECT id, mode, source_root_id, upgrade_id, upgrade_state, upgrade_by_id, upgrade_at`；`ListNotebooks`（`:55-68`）与 `ListOwnerless`（`ownerless.sql:1-9`）在 `n.mode, n.source_root_id` 之后加 `n.upgrade_id, n.upgrade_state, n.upgrade_by_id, n.upgrade_at`。每条查询的注释加一句"with its upgrade under way (M11 design 4.5)"；`ShareNotebook` 的注释末尾的"a downgrade takes the row FOR NO KEY UPDATE, so a unit's mode does not change while it runs"改为"…so a unit's mode and upgrade do not change while it runs"。
- `store.go`：`upgradeOf(id, state, by, at)` 照 `idOf` 的写法把四个可空的列转成 `domain.Upgrade`；`notebookOf`（`:94-104`）、`ListNotebooks`（`:146-151`）、`ownerless.go:20-24` 填它；`ShareNotebook`（`:84-90`）改为交回 `(domain.Mode, uuid.UUID, domain.Upgrade, error)`。
- 契约（`api/modules/notebook.yaml`）：`NotebookMode` 的说明（`:452-461`）末尾的"an upgrade's states are no mode"之后加"; see the notebook's upgrade"；`Notebook`（`:462-501`）的 `required`（`:469`）加 `upgrade`，属性加：

```yaml
        upgrade:
          description: >-
            The notebook's upgrade to an LLM Wiki under way, null for none.
            Whoever reads the notebook sees it: its content is read-only
            meanwhile, but for its initiator's MCP writes while upgrading.
          oneOf:
            - $ref: "#/components/schemas/NotebookUpgrade"
            - type: "null"
```

  可为 `null` 的对象照 `api/modules/transfer.yaml:689-691` 的写法；生成的 Go 字段与 handler 的填法照 transfer 的同一处。两个新的 schema：

```yaml
    NotebookUpgradeState:
      description: >-
        The state of a notebook's upgrade under way: upgrading while its
        initiator's agent rearranges it, reviewing once submitted,
        rolling_back while it is put back as it was when it started.
      type: string
      enum: [upgrading, reviewing, rolling_back]
    NotebookUpgrade:
      description: >-
        A notebook's upgrade to an LLM Wiki under way, as each reader of the
        notebook sees it; its admins read the rest with getUpgrade.
      type: object
      additionalProperties: false
      required: [id, state, started_by, started_at]
      properties:
        id:
          type: string
          format: uuid
        state:
          $ref: "#/components/schemas/NotebookUpgradeState"
        started_by:
          description: The account that started it.
          type: string
          format: uuid
        started_at:
          type: string
          format: date-time
```

- `notebookOf(v app.View)`（`notebook/adapter/http/handler.go:161-172`）在 `v.Notebook.Upgrade.ID` 不为零时填 `upgrade`，否则 `null`；六个答 `Notebook` 的操作（`handler.go:97`、`:110`、`:126`、`:135`、`:152`，`ownerless.go:53`）因此都带它，`apitest.CheckResponse` 照旧核对它们。
- 测试：`notebook/notebooks_test.go` 加 `TestTheStatesAnswerTheirUpgrade`（以 SQL 设四列，`StateOf`、`ShareState`、`LockState` 都答它，清掉之后都答零）；`notebook/adapter/postgres` 的测试加"六条查询都带升级"（逐条以 SQL 设列再读）；`notebook/adapter/http` 的测试加一本升级中的笔记本的 `getNotebook` 答 `upgrade`。

### 3.4 S1：page 的冻结、删除的护栏、变更集与预检

**状态与写**：

```go
// page/domain/mode.go

// UpgradeState is a notebook's upgrade's state, the notebook module's.
type UpgradeState string

const (
	UpgradeUpgrading   UpgradeState = "upgrading"
	UpgradeReviewing   UpgradeState = "reviewing"
	UpgradeRollingBack UpgradeState = "rolling_back"
)

// NotebookUpgrade is a notebook's upgrade under way, the zero value for
// none: what the freeze reads (M11 design 4.5).
type NotebookUpgrade struct {
	ID    uuid.UUID
	State UpgradeState
	By    uuid.UUID
}

type NotebookState struct {
	Mode         Mode
	SourceRootID uuid.UUID
	Upgrade      NotebookUpgrade
}
```

- `toPageState`（`bootstrap/page_notebooks.go:33-38`）逐字段转换 `Upgrade`（`ID`、`State`、`By`；`At` 不进 page）。
- `Write`（`page/app/extension.go:27-47`）在 `Notebook` 之后加：

```go
	// UpgradeOwn is the upgrade this unit writes for, the zero id for none:
	// the pages its start seeds (SeedSpec.Upgrade) and its rollback's
	// batches (M11 design 4.5). No REST or MCP adapter can set it.
	UpgradeOwn uuid.UUID
```

  `UnitSpec`（`page/app/unit.go:72-124`）加同名的字段，`run` 造 `Write` 时（`:264-268`）照抄。S1 里没有设它的生产代码（S2 的 `Seed`、S3 的回滚设它）；`page/app` 的模块测试以 `UnitSpec` 直接设它（下面的 `TestTheFreezeLetsItsOwnUnits`）。

**规则**（纯函数，`page/domain/freeze.go`）：

```go
// Freeze is what a notebook's upgrade does to a write (M11 design 4.5).
type Freeze int

const (
	// FreezeOpen: no upgrade, or the upgrade's own unit: nothing asked.
	FreezeOpen Freeze = iota
	// FreezeStale: a unit of an upgrade that is not the notebook's now.
	FreezeStale
	// FreezeRefused: notebook.upgrading.
	FreezeRefused
	// FreezeAsk: the initiator's MCP write while upgrading, admitted to
	// the guardrails when notebook.change_mode is granted.
	FreezeAsk
)

// FreezeAdmits is the freeze of a write by by from client, of the unit of
// the upgrade own (the zero id for none), in a notebook in state n.
func FreezeAdmits(n NotebookState, own uuid.UUID, client Client, by uuid.UUID) Freeze {
	switch {
	case own != (uuid.UUID{}) && own != n.Upgrade.ID:
		return FreezeStale
	case n.Upgrade.ID == (uuid.UUID{}), own == n.Upgrade.ID:
		return FreezeOpen
	case n.Upgrade.State != UpgradeUpgrading:
		return FreezeRefused
	case client.IsMCP() && by == n.Upgrade.By:
		return FreezeAsk
	}
	return FreezeRefused
}
```

错误（`page/domain/errors.go`，照 `:48` 的 `ErrLocked`）：

```go
	// ErrUpgrading: the notebook is being upgraded and the write is not
	// its initiator's MCP write while upgrading (M11 design 4.5).
	ErrUpgrading = shared.NewError(shared.KindConflict, "notebook.upgrading",
		"The notebook is being upgraded to an LLM Wiki: its content is read-only until the upgrade ends.")
	// ErrUpgradeProtected: a delete of a node the upgrade started with,
	// or of the source root its start created (M11 design 4.6).
	ErrUpgradeProtected = shared.NewError(shared.KindConflict, "page.upgrade_protected",
		"The node was there when the upgrade started, or is its source root: an upgrade may not delete it.")
	// ErrStaleUpgradeWrite is a unit of an upgrade the notebook no longer
	// has: a rollback's batch after its rollback was ended. No problem: a
	// defect's error, which rolls the unit back (M11 design 4.5, decision 225).
	ErrStaleUpgradeWrite = errors.New("a write of an upgrade the notebook no longer has")
```

**守卫与否决者**（`page/app/freeze_guard.go`）：

```go
// UpgradeNodes are an upgrade's rows the freeze reads, in the transaction
// ctx carries.
type UpgradeNodes interface {
	// Protected are those of ids that have a row of the upgrade upgradeID:
	// its baseline's and its protected source root's.
	Protected(ctx context.Context, upgradeID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

// FreezeGuard keeps an upgrading notebook's content read-only (M11 design
// 4.5): a guard of the writes and a vetoer of the openings, first of both.
type FreezeGuard struct {
	auth   shared.Authorizer
	admin  shared.Action
	nodes  UpgradeNodes
	// admits is domain.FreezeAdmits; a test's, given by WithAdmits
	// (export_test.go), answers a freeze it does not (S1 review S1A-N2).
	admits func(n domain.NotebookState, own uuid.UUID, client domain.Client, by uuid.UUID) domain.Freeze
}

func NewFreezeGuard(auth shared.Authorizer, admin shared.Action, nodes UpgradeNodes) *FreezeGuard

func (g *FreezeGuard) GuardWrite(ctx context.Context, s Step) error {
	switch f := g.admits(s.Notebook, s.UpgradeOwn, s.Client, s.By); f {
	case domain.FreezeOpen:
		return nil
	case domain.FreezeStale:
		return domain.ErrStaleUpgradeWrite
	case domain.FreezeRefused:
		return domain.ErrUpgrading
	case domain.FreezeAsk:
	default:
		// A freeze it does not know is a defect's error: it lets nothing
		// by it was not told to (S1 review S1A-N2).
		return fmt.Errorf("page: freeze %d unknown", f)
	}
	if err := s.Grants.Ask(ctx, g.admin, decideAction(g.auth, s, g.admin)); err != nil {
		if errors.Is(err, shared.Forbidden()) {
			return domain.ErrUpgrading
		}
		return err
	}
	return g.guardrails(ctx, s)
}

// guardrails are the initiator's agent's limits (M11 design 4.6): P1's is
// the delete's; P2/S1 adds the rest.
func (g *FreezeGuard) guardrails(ctx context.Context, s Step) error {
	if s.Operation != domain.OpDelete {
		return nil
	}
	ids := make([]uuid.UUID, len(s.Changes))
	for i, c := range s.Changes {
		ids[i] = c.NodeID
	}
	protected, err := g.nodes.Protected(ctx, s.Notebook.Upgrade.ID, ids)
	if err != nil {
		return err
	}
	for _, c := range s.Changes {
		if protected[c.NodeID] {
			return domain.Refuse(domain.ErrUpgradeProtected, c.NodeID)
		}
	}
	return nil
}

func (g *FreezeGuard) VetoEditSession(ctx context.Context, o SessionOpening) error {
	if o.Notebook.Upgrade.ID != (uuid.UUID{}) {
		return domain.ErrUpgrading
	}
	return nil
}
```

- `decideAction(auth, s, action)` 是 `ModeGuard.decide`（`page/app/mode_guard.go:88-97`）抽出的函数，两个守卫共用（判定的目标是 `s.WorkspaceID`、`s.NotebookID`，执行者取 ctx 的）；撤销的单元也用它（`page/app/unit_revert.go:100`），不再只为判定建一个 `ModeGuard`（S1 偏离 6）。`Grants.Ask` 的注释（`grants.go:15`）"Only the mode guard and the unit's own locked checks ask"改为"Only the freeze and mode guards and the unit's own locked checks ask"；它前面的"a step of a standard notebook, or one that touches no source, never does"改为"a step of a notebook with no upgrade that touches no source never does"：冻结的守卫恰在普通笔记本里问（S1 审查 S1A-N3）。
- 拒绝标在 `s.Changes` 里第一个受保护的节点（`Changes` 从上往下排，`unit_delete.go:46-53`），`refusalOf`（`unit.go:333-339`）见到已标的不再改标（`domain.Refuse` 不重包，`page/domain/refused.go:23-28`）。`ErrUpgrading` 不标节点时由 `refusalOf` 标 `Changes[0]`，`batch` 的失败项与 transfer 的 `refused` 因此有去处（4.5）。被标的节点只在 page 之内看得到：MCP 的工具错误只带 `code`、`message` 与 `op`（`platform/httpserver/mcpserver/errors.go`），`batch` 的失败项是那个删除的操作（`OpFailed` 只带操作的序号与工具，`page/app/apply_batch.go:624-636`）；transfer 的 `refused` 带节点（`bootstrap/transfer.go:257`），但后台任务的单元不是 MCP 的写，到不了护栏。所以只有 `TestTheFreezeGuard` 钉住"标在第一个受保护的节点"（7.3 第 14 条），P1 不为它改 MCP 工具错误的形状（S1 审查 S1A-Q1 = S1C-M1，决定 237）。
- 模块根（`page/freeze_guard.go`，照 `page/mode_guard.go:20-23`）：`type FreezeGuard = app.FreezeGuard`；`func NewFreezeGuard(pool *pgxpool.Pool, auth shared.Authorizer, admin shared.Action) *FreezeGuard`，读者是 `postgresadapter.New(pool)`（它经 `postgres.DB(ctx, pool)` 用单元的事务）。13.1 第 21 条的例外同 `NewModeGuard`。模块根另照 `ModeStandard`、`ModeLLMWiki` 导出升级的三个状态 `UpgradeUpgrading`、`UpgradeReviewing`、`UpgradeRollingBack`（`page/module.go:70-75`；S1 审查 S1C-N3：组合根的测试以它们写期望）。
- 读（`page/adapter/postgres/queries/upgrade.sql`，新）：

```sql
-- name: UpgradeProtected :many
-- Those of node_ids that have a row of the upgrade upgrade_id (M11 design 4.6): a delete's guardrail, by the primary
-- key, in the unit's transaction under the notebook's row lock.
SELECT node_id FROM upgrade_nodes
WHERE upgrade_id = sqlc.arg(upgrade_id) AND node_id = ANY(sqlc.arg(node_ids)::uuid[]);
```

- 登记（`bootstrap/registrants.go:260-273`）：`freeze := page.NewFreezeGuard(pool, authorizer, notebook.ActionChangeMode)`；`guards: []page.WriteGuard{freeze, mode, lock}`、`sessionVetoers: []page.EditSessionVetoer{freeze, mode, lock}`；注释（`:253-255`）的"M11's freeze vetoes an opening"改为"M11's freeze comes first of both, so that a write it and another refuse answers notebook.upgrading (M11 design 4.5)"。签名不变（4.5 的"登记"）。

**变更集的 `upgrade_id`**：`app.Changeset`（`page/app/ports.go:127-137`）加 `UpgradeID uuid.UUID`；`ensureChangeset`（`unit.go:467-470`）填 `u.write.Notebook.Upgrade.ID`。冻结之下能建变更集的只有发起人的 MCP 写与升级自己的单元（别的单元在守卫处整个回滚，守卫在变更集之前，`unit.go:368-372`），所以"笔记本有升级就记它"等于"这次升级的写都记它"。`CreateChangeset`（`changesets.sql:1-4`）的列与值加 `upgrade_id`、`sqlc.narg(upgrade_id)`；`changesets.go:15-28` 把零 id 写成 `NULL`。`TouchChangeset` 不改（并进已有的变更集不改它的标记；冻结中没有会并进旧变更集的写：编辑会话开不了，导入与后台任务开不了）。

**预检**（4.5，各在判定之后、别的检查之前；三处共用 `Writer.notUpgrading`，`page/app/unit.go:165-182`：读一次不加锁的状态，有升级答 `ErrUpgrading`，没有笔记本答 `notebook.not_found`，S1 偏离 8）：
- `AssetWrites.Check`：`:57` 的 `StateOf` 提到 `:50` 的 `Allowed` 之后，`Upgrade.ID` 不为零答 `ErrUpgrading`；之后的模式的核对（`:78-81`）用同一个读到的状态；状态因此恒读一次，以前名称的检查先拒时一次也不读（M10 的 `TestAnUploadsCheckRefusesAReservedTitle` 随之改为恒一次）。
- `JobWrites.CheckRestore`：`:140-146` 的 `StateOf` 提到 `:119` 的 `Allowed` 之后，同样；之后的 `FindAnyNode` 与 `admit` 照旧。
- `JobWrites.CheckRevert`：`:180` 的 `Allowed` 之后另读一次 `StateOf`、同样判；`revertChecks.check`（`revert_changeset.go:248-254`）不改，同步的 `revertChangeset` 不加预检（4.5）。所以后台撤销的开始路径读两次状态（M10 的 `TestARevertsChecksRefuseTheContract` 随之改为两次）。

**测试**：`page/domain` 的 `TestFreezeAdmits`（状态 × 客户端 × 执行者 × `UpgradeOwn` 的表：没有升级；`upgrading` 的发起人 MCP、发起人 web、别人 MCP；`reviewing`、`rolling_back` 的发起人 MCP；`UpgradeOwn` 等于当前、不等于当前、当前已没有升级）；`page/app` 的 `TestTheFreezeGuard`（假的 `UpgradeNodes` 与授权者：`Ask` 只问一次、`forbidden` 答 `notebook.upgrading`、删除的拒绝标在第一个受保护的节点而不是 `Changes[0]`、新建的子树可以删、不是删除的步不读）、`TestTheFreezeLetsItsOwnUnits`（以 `UnitSpec.UpgradeOwn` 写：等于当前放过、过期答 `ErrStaleUpgradeWrite` 且什么也没写）、`TestTheChangesetsCarryTheirUpgrade`、`TestThePrechecksRefuseAnUpgradingNotebook`（上传、后台恢复、后台撤销的预检在判定之后先答 `notebook.upgrading`、之后什么也不读、撤销不 admit；读者仍是 `forbidden`；S1 偏离 13）；`TestTheFreezeGuard` 另有"a freeze it does not know"（不认识的判定答缺陷的错误，什么也不问、不读，单元什么也没写；S1 审查 S1A-N2）；`page/adapter/postgres` 的 `UpgradeProtected` 的测试与 `TestCreateChangesetWritesItsUpgrade`（都在 `upgrade_test.go`，S1 偏离 9）；组合根的 `TestPageNotebooksConvertTheState` 加升级的一行，期望写 page 的常量（S1 偏离 11、S1 审查 S1C-N3）。

### 3.5 S1：transfer 的笔记本端口与开任务的判断

```go
// transfer/app/ports.go

// NotebookState is what a start reads of its notebook: its upgrade under
// way, the zero id for none (M11 design 4.5).
type NotebookState struct {
	UpgradeID   uuid.UUID
	RollingBack bool
}

type Notebooks interface {
	WorkspaceOf(ctx context.Context, id uuid.UUID) (uuid.UUID, bool, error)
	// ShareByID locks the notebook not deleted id FOR SHARE until the
	// transaction ends, and answers its state read under the lock; false
	// for none.
	ShareByID(ctx context.Context, id uuid.UUID) (NotebookState, bool, error)
	// StateOf is the notebook's state, unlocked; false for none.
	StateOf(ctx context.Context, id uuid.UUID) (NotebookState, bool, error)
	NameOf(ctx context.Context, id uuid.UUID) (string, bool, error)
}
```

- 组合根的新文件 `bootstrap/transfer_notebooks.go`：`transferNotebooks{notebook.Notebooks}`，`ShareByID` 经 `ShareState`、`StateOf` 经根的 `StateOf`，转换 `Upgrade.ID` 与 `State == notebook.UpgradeRollingBack`；`bootstrap/deps.go:243` 改交它。
- `StartDeps.lock`（`transfer/app/start.go:105-113`）改为交回 `(NotebookState, error)`；导出（`:73`）不看它（冻结期间导出照常，4.5）。
- `transfer/domain/errors.go` 加 `ErrUpgrading`（同 page 的码与文字，先例 `page/domain/errors.go:21` 的 `notebook.not_found`）。
- 导入的预检：`StartImport.Check`（`start_import.go:76` 的 `authorize` 之后、`:79` 的 `place` 之前）读 `StateOf`，`UpgradeID` 不为零答 `ErrUpgrading`；没有笔记本（判定之后被删）答 `notebook.not_found`（`StartDeps.notUpgrading`，`transfer/app/start.go:126-137`）。
- 导入建任务的事务：`:249` 的 `lock` 交回的状态 `UpgradeID` 不为零答 `ErrUpgrading`（正在上传的导入在这里被拒，文件照导入失败时的处理删掉，`start_import.go` 已有的失败路径）。
- `startBackground`（`start_background.go:169`）：`lock` 之后，`UpgradeID` 不为零而不是"种类 `rollback`、`job.UpgradeID` 等于它、`RollingBack`"时答 `ErrUpgrading`。S1 里还没有回滚的种类，这一条只有"不为零就拒"一支，S3 加放过的一支（3.19）。
- 测试：`transfer/app` 的 `TestTheStartsRefuseAnUpgradingNotebook`（假的端口：导入的 `Check`、建任务、`StartTrashRestore`、`StartRevert` 各答 `notebook.upgrading`；导出照常。导入的 `Check` 用父节点不存在的请求，断言调用恰是 `[Notebooks.WorkspaceOf, Authorize, Notebooks.StateOf]`，升级结束之后同一请求答 `page.not_found`；读状态时笔记本已删答 `notebook.not_found`；恢复与撤销的开始由读者发起时，锁下的判定先答 `forbidden`：S1 审查 S1A-M1、S1A-N1）；导入的 `Check` 的调用序列多了 `Notebooks.StateOf`，`TestAnImportStarts` 随之改（S1 偏离 8）；组合根的 `TestTransferNotebooksConvertTheState`（没有、升级中、回滚中、没有笔记本、错误，两种读；S1 偏离 11）；transfer 的 `adapter/http` 的替身 `notebooks` 带一个原子的 `upgrading`，恢复与撤销的 409 由 transfer 自己在锁下答、导入的由 `StateOf` 答（S1 偏离 14）。

### 3.6 S1：`notebook.upgrading` 的声明与 MCP 的码

**REST**（00 号文档第 5 节的 14 个；每个在它的模块的 `adapter/http` 测试里经 `apitest.CheckResponse` 答出，13.1 第 8 条）。照 4.16 的判定次序写：经单元的九个，冻结的守卫在单元里领域的检查之后、守卫之中最先，所以 `notebook.upgrading` 插在这一操作的第一个守卫或参与者的码（`page.protected`、`page.virtual_file`、`page.source_admin_only`、`page.source_too_large`、`page.locked`、`linking.pages_locked`）之前，没有这样的码就放在领域的码之后；M10 排在列表开头的来源的码（`page.source_duplicate`、`page.source_immutable`）不动，它们在列表里的位置本来就不是判定的次序（P1 审查 C-N1）。`openEditSession` 的冻结是最先的否决者，插在 `page.locked` 之前。有预检的四个插在 `forbidden` 之后。次序没有测试核对（13.4 第 3 条只核对三种入口一致），改后的列表（加的一项用粗体标出）：

| 操作 | `x-problem-codes`（改后） |
|---|---|
| `createPage`（`page.yaml:64`） | `page.source_duplicate, notebook.not_found, forbidden, validation_failed, page.title_taken, page.too_deep,` **`notebook.upgrading`**`, page.virtual_file, server_busy` |
| `putPageContent`（`:175`） | `page.source_duplicate, page.source_immutable, validation_failed, page.not_found, forbidden, page.edit_session_ended, page.edit_session_taken_over, page.edit_session_unlocked, page.revision_mismatch,` **`notebook.upgrading`**`, page.locked, server_busy` |
| `openEditSession`（`:228`，行内） | `[page.source_immutable, page.not_found, forbidden,` **`notebook.upgrading`**`, page.locked]` |
| `restorePageVersion`（`:475`） | `page.source_immutable, page.not_found, page.version_not_found, forbidden, server_busy, page.revision_mismatch,` **`notebook.upgrading`**`, page.locked` |
| `toggleTask`（`:529`） | `page.source_immutable, page.not_found, forbidden, page.revision_mismatch, validation_failed,` **`notebook.upgrading`**`, page.locked, server_busy` |
| `renameNode`（`:635`） | `page.not_found, forbidden, validation_failed, page.title_taken,` **`notebook.upgrading`**`, page.protected, page.virtual_file, page.source_admin_only, linking.pages_locked, server_busy` |
| `deleteNode`（`:682`，行内） | `[page.source_admin_only, page.not_found, forbidden,` **`notebook.upgrading`**`, page.protected, page.locked]` |
| `moveNode`（`:738`） | `page.source_duplicate, page.not_found, forbidden, validation_failed, page.cycle, page.title_taken, page.too_deep,` **`notebook.upgrading`**`, page.protected, page.virtual_file, page.source_admin_only, page.source_too_large, linking.pages_locked, server_busy` |
| `restoreNode`（`:867`） | `page.source_duplicate, page.not_found, forbidden, page.not_in_trash, page.not_trash_root, page.restore_expired, page.restore_too_large, server_busy, page.trash_changed,` **`notebook.upgrading`** |
| `revertChangeset`（`:1083`） | `page.source_duplicate, page.source_immutable, validation_failed, page.changeset_not_found, forbidden, page.changeset_trimmed, page.revert_too_large, page.revert_conflict,` **`notebook.upgrading`**`, page.protected, page.source_admin_only, page.source_too_large, server_busy, page.locked` |
| `uploadAsset`（`asset.yaml:51`） | `page.source_duplicate, notebook.not_found, forbidden,` **`notebook.upgrading`**`, validation_failed, page.title_taken, page.virtual_file, storage_full` |
| `startImport`（`transfer.yaml:105`，行内） | `[notebook.not_found, forbidden,` **`notebook.upgrading`**`, page.not_found, transfer.busy, server_busy, storage_full]` |
| `startTrashRestore`（`:175`） | `page.source_duplicate, notebook.not_found, forbidden,` **`notebook.upgrading`**`, page.not_found, page.not_in_trash, page.not_trash_root, page.restore_expired, transfer.busy, server_busy` |
| `startRevert`（`:281`） | `page.source_duplicate, page.source_immutable, validation_failed, notebook.not_found, forbidden,` **`notebook.upgrading`**`, page.changeset_not_found, page.changeset_trimmed, page.revert_too_large, transfer.busy, page.revert_conflict, page.protected, page.source_admin_only, page.source_too_large, page.locked, server_busy` |

`page.upgrade_protected` 不进任何 REST 操作（REST 的写在冻结里就被拒）。

**MCP**：`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:89`）的 `write`、`edit`、`move`、`delete`、`batch` 五行（`:95-106`）各加 `notebook.upgrading`，`delete`、`batch` 另加 `page.upgrade_protected`，都在行里已有的守卫的码之后。`mcpOwnCodes()`（`bootstrap/mcp_client_test.go:338-340`）加 `page.upgrade_protected`（`notebook.upgrading` 由 REST 声明）。新的 `upgradeToolCodes()`（新文件 `bootstrap/upgrade_tool_codes_test.go`）按（工具，码）的对列出这七对；第五份的 `TestTheMCPToolsErrorsCarryTheirCodes`（`bootstrap/mcp_reads_test.go:427`）核对 `mcpToolCodes()` 减去它，第七份的 `TestUpgradeToolsErrorsCarryTheirCodes` 核对它（00 号文档第 5 节）：后者以 SQL 设一本笔记本升级中（发起人是 PAT 的主人，基线有一个页）与另一本升级中（发起人是别人），分别以 `write`、`edit`、`move`、`delete`、`batch` 答出 `notebook.upgrading`，以 `delete` 与 `batch` 的删除答出 `page.upgrade_protected`。

### 3.7 S1：事件的说明、日志与 `checkPages`

- `api/modules/events.yaml`：`mode` 的说明（`:48-51`，今天"…(and, later, a notebook upgraded)…"）改为"a notebook's mode or its upgrade changed: a downgrade, an upgrade's start, submission, return, approval, rejection, abandonment, and its rollback's stop and end"；`EventMode.description`（`:223`）"The data of mode, a change of a notebook's mode."改为"The data of mode, a change of a notebook's mode or of its upgrade."。S1 里还没有发它的升级的转移（S2 起），说明先写；`streamEvents` 的契约测试照旧。
- 日志：S1 不加用例，不加日志行。冻结的拒绝照单元已有的拒绝日志（一个有类的错误，不记 error）；`ErrStaleUpgradeWrite` 是缺陷：后台的批照"别的错误"以 `internal` 停下（`transfer/app/background.go:262-297` 的 `stopped`）；交错第 16 条里任务行已被 Rescue 的那条 SQL 记为失败，`FinishJob` 是 0 行，所以只记一行"stopped"（`job_run.go:215-218`），没有 error 级的一行（P1 审查 A-M1），测试断言那一批什么也没写与心跳写不进的那行 warn（7.2）。
- `checkPages`（`bootstrap/interleavings_page_test.go:58`）S1 不改：它的规则在 S2 才能写（要 wiki 的行），S1 的夹具也不违反已有的规则（升级列不进任何规则）。

### 3.8 S1：第七份

- `Makefile`：`BOOTSTRAP_SIXTH`（`:266`）之后加一段注释与 `BOOTSTRAP_SEVENTH = ^TestUpgrade`（"M11/P1/S1：升级的整个程序测试单列一份（00-M11 第 186 条），第三份的 `[U-Z]` 收得下它，所以第三份跳过它；第四份是补集，跳过第三份的模式时已跳过它"）；`test-go-bootstrap-7` 照 `-6`（`:294-296`）写，`-run '$(BOOTSTRAP_SEVENTH)'`；`test-go-bootstrap-3`（`:284`）的 `-skip` 改为 `'$(BOOTSTRAP_SECOND)|$(BOOTSTRAP_FIFTH)|$(BOOTSTRAP_SEVENTH)'`，它的说明改为"不匹配 SECOND/FIFTH/SEVENTH 的"；`test-go-bootstrap`（`:272`）的依赖加 `-7`、说明"六份依次"改为"七份依次"；`test-go`（`:190`）的列表加 `test-go-bootstrap-7`；注释 `:183`"组合根按测试名六份"改为"七份"，`:184`"竞态检测的这七个最多六个同时跑"改为"这八个最多六个同时跑"（`-j6` 不变），`:185` 与 `:195` 的"CI 的十三份"改为"十四份"。
- `.github/workflows/ci.yml`：`bootstrap-6`（`:66-67`）之后加 `- leg: bootstrap-7`、`timeout: 25`；注释 `:23` 的"-4、-5 与 -6"改为"-4、-5、-6 与 -7"，`:25` 的"server 共十四份（含 short），连同 web、e2e 与 image 共十七项"改为"十五份……十八项"，`:48` 的"按测试名分成六份（Makefile 的 FIRST、SECOND、THIRD、FIFTH、SIXTH 与第四份互补规则"改为"七份（……FIFTH、SIXTH、SEVENTH 与第四份互补规则"。
- `docs/v0.1/claude_to_codex/06-environment.md`：`:27`"（`-2` 到 `-6` 同理）"改为"到 `-7`"；`:49`"组合根现在分六份（`Makefile` 的 `BOOTSTRAP_FIRST` 到 `BOOTSTRAP_SIXTH`），CI 共 17 项：14 个 server 的 leg（race、bootstrap-1 到 bootstrap-6、"改为"七份（……到 `BOOTSTRAP_SEVENTH`），CI 共 18 项：15 个 server 的 leg（race、bootstrap-1 到 bootstrap-7、"，同一条的"`make -j1 test-go-bootstrap-1` 到 `test-go-bootstrap-6`"改为"到 `-7`"。
- S1 的记录写出每份的 `go test -list`：`TestUpgrade…` 只在第七份，七份合起来等于全部顶层测试（第 223 条）。清单是竞态构建的，同 `Makefile` 的分份注释的口径（`//go:build !race` 的成本与守卫测试不进任何一份，在 costs、guards 的行里跑；S1 审查 S1C-N2）：S1 之后 534 个，七份 158、51、121、123、62、12、7，`Makefile` 的注释写这个数。

### 3.9 S2 的文件

| 文件 | 改动 |
|---|---|
| `server/migrations/sql/00057_wiki_notebook_upgrades.sql`；`server/sqlc.yaml` 新的 wiki 一项；`schema_test.go`、`upgrade_test.go` | 3.10 |
| 新的 `wiki/adapter/postgres/`（`queries/upgrades.sql`、`store.go`、`gen/`）；`deploy/runtime-grants.sql`；`bootstrap/registrants.go`、`runtime_role_test.go` | wiki 的表的读写、授权、清理器与删除的订阅者（3.15） |
| 新的 `notebook/upgrade_states.go`；`notebook/adapter/postgres/queries/notebooks.sql` | `UpgradeStates` 的 `Lock`、`Start`、`Publish`（3.11） |
| 新的 `page/upgrades.go`、`page/adapter/postgres/queries/upgrade.sql`、`sessions.sql`、`page/adapter/postgres/upgrade.go`；`page/app/seed_writes.go`；page 的笔记本删除的订阅者 | 3.12 |
| `wiki/app/lint.go`、`lint_runs.go`、`lint_ports.go`、`lint_run_types.go`；`linking/app/lint_edges.go`、`lint_rules.go`、`lint_reads.go`、`linking/adapter/postgres/queries/lint.sql`；`bootstrap/wiki_lint.go` | 3.13 |
| 新的 `wiki/app/start_upgrade.go`、`get_upgrade.go`、`upgrade_ports.go`、`wiki/domain/upgrade.go`、`upgrade_errors.go`；`wiki/app/template.go`、`wiki/app/ports.go`；`wiki/module.go`、`wiki/lint.go`；`wiki/adapter/http/`；`bootstrap/wiki.go`（`wikiSeeds`）；`api/modules/wiki.yaml`、`api/openapi.yaml`、`api/common.yaml` | 3.14 |
| `bootstrap/wire.go`、`deps.go`；`bootstrap/interleavings_page_test.go` | 接线与 `checkPages`（3.15） |

### 3.10 S2：迁移 00057

```sql
-- 00057_wiki_notebook_upgrades.sql
-- notebook_upgrades: a notebook's upgrades to an LLM Wiki (M11 design 4.2, 4.3), the wiki module's first table: who
-- started it and when, the source root it proposes, the pages its start wrote, its baseline's counts, its submission
-- and the report kept, its rollback's job, its state, ending and whether its rollback was done. A notebook has at
-- most one under way. The ones that ended stay, soft-deleted with their notebook and purged after it.

-- +goose Up
CREATE TABLE notebook_upgrades (
    id uuid NOT NULL,
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT,
    state text NOT NULL,
    ending text,
    source_root_id uuid NOT NULL,
    seeded_root boolean NOT NULL,
    seeded_agents_id uuid,
    locale text NOT NULL,
    baseline_nodes integer NOT NULL,
    baseline_pages integer NOT NULL,
    baseline_wanted integer NOT NULL,
    baseline_wanted_capped boolean NOT NULL,
    submission_id uuid,
    submitted_by_id uuid REFERENCES users,
    submitted_at timestamptz,
    report jsonb,
    rollback_job_id uuid,
    rollback_complete boolean,
    rollback_left_nodes integer,
    rollback_left_pages integer,
    rollback_left_created integer,
    created_by_id uuid NOT NULL REFERENCES users,
    updated_by_id uuid NOT NULL REFERENCES users,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    deleted_at timestamptz,
    CONSTRAINT notebook_upgrades_pkey PRIMARY KEY (id),
    CONSTRAINT notebook_upgrades_state_check
        CHECK (state IN ('upgrading', 'reviewing', 'rolling_back', 'approved', 'rolled_back')),
    CONSTRAINT notebook_upgrades_ending_check CHECK (ending IN ('abandoned', 'rejected')),
    CONSTRAINT notebook_upgrades_locale_check CHECK (locale IN ('zh-CN', 'en')),
    -- baseline_nodes >= 0 follows from 0 <= baseline_pages <= baseline_nodes, kept to be read.
    CONSTRAINT notebook_upgrades_baseline_check CHECK (baseline_nodes >= 0 AND baseline_pages BETWEEN 0 AND baseline_nodes
        AND baseline_wanted BETWEEN 0 AND 10000 AND baseline_wanted_capped = (baseline_wanted = 10000)),
    CONSTRAINT notebook_upgrades_ending_state_check
        CHECK ((state IN ('rolling_back', 'rolled_back')) = (ending IS NOT NULL)),
    CONSTRAINT notebook_upgrades_submission_check CHECK ((submission_id IS NULL) = (submitted_at IS NULL)
        AND (submitted_at IS NULL) = (submitted_by_id IS NULL)),
    -- 'approved' in the second conjunct follows from approved_check with report_check, kept to be read.
    CONSTRAINT notebook_upgrades_submission_state_check CHECK ((state <> 'upgrading' OR submission_id IS NULL)
        AND (state NOT IN ('reviewing', 'approved') OR submission_id IS NOT NULL)),
    -- state <> 'upgrading' follows from submission_id IS NOT NULL with submission_state_check, kept to be read.
    CONSTRAINT notebook_upgrades_report_check CHECK (report IS NULL OR submission_id IS NOT NULL AND state <> 'upgrading'
        AND jsonb_typeof(report) = 'object' AND octet_length(report::text) <= 65536),
    CONSTRAINT notebook_upgrades_approved_check CHECK (state <> 'approved' OR report IS NOT NULL),
    CONSTRAINT notebook_upgrades_rejected_check CHECK (ending IS DISTINCT FROM 'rejected' OR submission_id IS NOT NULL),
    CONSTRAINT notebook_upgrades_rollback_job_check
        CHECK ((rollback_job_id IS NOT NULL) = (state IN ('rolling_back', 'rolled_back'))),
    CONSTRAINT notebook_upgrades_rollback_end_check CHECK ((rollback_complete IS NOT NULL) = (state = 'rolled_back')
        AND (rollback_complete IS DISTINCT FROM false) = (rollback_left_nodes IS NULL)
        AND (rollback_left_nodes IS NULL) = (rollback_left_pages IS NULL)
        AND (rollback_left_nodes IS NULL) = (rollback_left_created IS NULL))
);
-- One upgrade under way a notebook; the foreign key's check as a notebook is purged.
CREATE UNIQUE INDEX notebook_upgrades_notebook_id_key ON notebook_upgrades (notebook_id)
    WHERE state IN ('upgrading', 'reviewing', 'rolling_back') AND deleted_at IS NULL;
CREATE INDEX notebook_upgrades_notebook_id_idx ON notebook_upgrades (notebook_id);

-- +goose Down
DROP TABLE notebook_upgrades;
```

`ending`、`locale` 的检查在 NULL 时放过是有意的（`ending` 可空；`locale` 非空）；其余每个合取都是非空列或 `IS NULL`、`IS DISTINCT FROM` 的比较。名称进 `TestConstraintAndIndexNames`（主键、`notebook_id` 的外键照已有的 `ON DELETE RESTRICT` 项的写法、三个 `REFERENCES users` 的外键照 `"notebooks_created_by_id_fkey f a"`、十二个检查、两个索引）。反例：`checkedRows` 给 S1 种下的升级中的笔记本补一行 `upgrading` 的 wiki 行，再各以一条 `INSERT` 或 `UPDATE` 钉住：不认识的状态、结局、语言；`baseline_nodes < 0`（连带违反 `pages <= nodes`）、`baseline_pages < 0`、`baseline_pages > baseline_nodes`、`baseline_wanted < 0`（同时设 `capped = false`）、`baseline_wanted = 10001`、`capped` 与数不符（两个方向各一）；`rolling_back` 没有结局、`upgrading` 有结局；提交三列各缺一（三条）；只有 `submission_id`（只违反 `(submission_id IS NULL) = (submitted_at IS NULL)`）、只有 `submitted_by_id`（只违反 `(submitted_at IS NULL) = (submitted_by_id IS NULL)`）（两条："缺时刻"同时违反两个合取，每个合取的这一方向要单独的一行；修订核对 B2-N1）；`upgrading` 带提交、`reviewing` 没有提交；报告在 `upgrading`（连带违反 `submission_state_check`，报出的是 `report_check`：PostgreSQL 按名字的次序查检查约束）、报告没有提交、报告是数组、报告超过 65,536 字节；`approved` 没有报告；`rejected` 没有提交；`rolling_back` 没有任务、`upgrading` 有任务；`rolled_back` 没有 `rollback_complete`、`upgrading` 有它、`true` 带三个数、`false` 缺一个数（三条）；这三个 `=` 的另一向各一条（`false` 三个数都空、`true` 只带页数、`true` 只带新节点数：原来只由"`false` 缺节点数"一条同时碰到三个合取，S2 审查 S2B-M1）。唯一索引另有四条：同一本笔记本第二行 `reviewing`、第二行 `rolling_back`（带结局与任务）答 23505，一行 `approved`、一行软删除的 `upgrading` 放过（谓词的两项各有一条，S2B-N1；循环照 `schema_test.go:755-763` 的写法，这几条单独断言 SQLSTATE）。检查的反例共 36 个。被别的约束蕴含的三个合取保留，只为读得清楚，迁移里各一句注释写明由哪条推出（决定 241，S2B-N2）：`baseline_nodes >= 0`（由 `0 <= baseline_pages <= baseline_nodes`）、`submission_state_check` 的 `'approved'`（由 `approved_check` 与 `report_check`）、`report_check` 的 `state <> 'upgrading'`（由 `submission_id IS NOT NULL` 与 `submission_state_check`）；13.4 第 1 条的"每个合取单独一条反例"对它们不适用，"`baseline_nodes < 0`"与"报告在 `upgrading`"两条照实写连带违反的检查。`upgrade_test.go` 的 Down 测试 `downTo(t, m, 57)`（`downTo` 退掉给定的那一版，同 S1 偏离 2）之后只少这张表、其余照旧，再 Up 回来。

### 3.11 S2：`notebook.NewUpgradeStates`

```go
// notebook/upgrade_states.go

// UpgradeStates writes a notebook's upgrade's columns and tells the mode's
// subscribers of each change, in the transaction ctx carries (M11 design
// 4.1): the wiki module's upgrade calls it, under its own transaction.
type UpgradeStates struct {
	store       *postgresadapter.Store
	subscribers []ModeSubscriber
}

// NewUpgradeStates is over pool and the mode's subscribers, the ones
// bootstrap hands the downgrade (v0.1 design 12.1 item 6's exception).
func NewUpgradeStates(pool *pgxpool.Pool, subscribers []ModeSubscriber) *UpgradeStates

// Lock locks the notebook not deleted id FOR NO KEY UPDATE until the
// transaction ends and answers its state; false for none. Its caller has
// locked the workspace's row FOR SHARE first.
func (u *UpgradeStates) Lock(ctx context.Context, id uuid.UUID) (State, bool, error)

// Start sets the notebook's upgrade to up, which is upgrading, by up.By
// at up.At; the notebook is locked, standard, and has none.
func (u *UpgradeStates) Start(ctx context.Context, id uuid.UUID, up Upgrade) error

// Publish tells the mode's subscribers of c in their order, as the
// downgrade does (publishMode): the change of an upgrade's state is one
// with From == To.
func (u *UpgradeStates) Publish(ctx context.Context, c ModeChange) error
```

每个方法照 `ShareState` 先核对 `postgres.InTx(ctx)`（`notebook/notebooks.go:112`），不在事务里是错误。`Lock` 经 `LockNotebook`（3.3 之后带四列）；`Publish` 调 `publishMode`（`notebook/app/extension.go:91-98`）同一段。`Start` 的 SQL（`notebooks.sql`）：

```sql
-- name: StartUpgrade :execrows
-- A notebook's upgrade's start (M11 design 4.4 step 10), under the row's lock: the one write of its four columns
-- that sets them, on a standard notebook with none.
UPDATE notebooks
SET upgrade_id = sqlc.arg(upgrade_id), upgrade_state = 'upgrading', upgrade_by_id = sqlc.arg(by), upgrade_at = sqlc.arg(now),
    updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND upgrade_id IS NULL AND mode = 'standard';
```

改了不是一行是缺陷（调用方已在锁下判过）。测试：`notebook/upgrade_states_test.go` 的 `TestTheUpgradeStatesWriteUnderTheirCaller`（不在事务里答错误；外层回滚之后什么也没写；`Lock` 答四列；`Start` 在 LLM Wiki 或已有升级上改 0 行答错误）与 `TestTheUpgradeStatesPublishInOrder`（替身的订阅者记下次序与 `ModeChange`）。

### 3.12 S2：page 的基线、会话与根下的键

模块根新导出只凭连接池的 `page.NewUpgrades(pool) *Upgrades`（13.1 第 11 条），方法都用 ctx 的事务（`postgres.DB(ctx, pool)`）。S2 的方法：

```go
// Baseline is what an upgrade's start recorded: its nodes and pages.
type Baseline struct{ Nodes, Pages int }

// SessionHolder is an edit session alive in a notebook: its page and editor.
type SessionHolder struct{ PageID, UserID uuid.UUID }

// RootNode is a node at a notebook's root by its title key.
type RootNode struct {
	ID   uuid.UUID
	Kind Kind
	Key  string
}

func (u *Upgrades) Capture(ctx context.Context, upgradeID, notebookID uuid.UUID) (Baseline, error)
func (u *Upgrades) Protect(ctx context.Context, upgradeID, notebookID, rootID uuid.UUID) error
func (u *Upgrades) SetWanted(ctx context.Context, upgradeID uuid.UUID, wanted map[uuid.UUID]int) error
func (u *Upgrades) Analyze(ctx context.Context) error
func (u *Upgrades) ClearExpiredSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) error
func (u *Upgrades) LiveSessions(ctx context.Context, notebookID uuid.UUID, at time.Time) ([]SessionHolder, int, error)
func (u *Upgrades) RootNodes(ctx context.Context, notebookID uuid.UUID, keys []string) ([]RootNode, error)
```

SQL（`page/adapter/postgres/queries/upgrade.sql` 与 `sessions.sql`）：

```sql
-- name: CaptureBaseline :one
-- An upgrade's baseline (M11 design 4.4 step 11): a row for each node alive in the notebook, under the notebook's row
-- lock; a page's latest version, its hash also the one the guardrails accept, its wanted count 0 until step 13. The
-- notebook's nodes drive page_contents' primary key, one node at a time, so that the start reads this notebook's
-- rows and not the instance's (decision 240): the LIMIT 1 keeps the LATERAL from being pulled up into a join, which
-- may hash the whole table, and changes nothing, a node having one row of page_contents at most.
WITH captured AS (
    INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
        content_hash, byte_size, wanted, accepted_hash)
    SELECT sqlc.arg(upgrade_id), n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision,
        c.content_hash, c.byte_size, CASE WHEN n.kind = 'page' THEN 0 END, c.content_hash
    FROM nodes n
    LEFT JOIN LATERAL (
        SELECT c.revision, c.content_hash, c.byte_size FROM page_contents c
        WHERE c.node_id = n.id AND c.deleted_at IS NULL
        LIMIT 1
    ) c ON true
    WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL
    RETURNING kind
)
SELECT count(*)::integer AS nodes, (count(*) FILTER (WHERE kind = 'page'))::integer AS pages FROM captured;

-- name: ProtectSourceRoot :exec
-- The source root an upgrade's start created (M11 design 4.4 step 12): kept from a delete, not of the baseline.
INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind)
VALUES (sqlc.arg(upgrade_id), sqlc.arg(node_id), sqlc.arg(notebook_id), false, 'page');

-- name: SetBaselineWanted :exec
-- The distinct wanted targets each baseline page links to as the upgrade starts (M11 design 4.4 step 13), one
-- unnest a list in the select list as SetSourceHashes (sources.sql:46-54): sqlc parses no multi-argument unnest in FROM.
UPDATE upgrade_nodes u SET wanted = w.wanted
FROM (SELECT unnest(sqlc.arg(node_ids)::uuid[]) AS node_id, unnest(sqlc.arg(wanted)::integer[]) AS wanted) w
WHERE u.upgrade_id = sqlc.arg(upgrade_id) AND u.node_id = w.node_id AND u.baseline AND u.kind = 'page';

-- name: AnalyzeUpgradeNodes :exec
ANALYZE upgrade_nodes;

-- name: RootNodesByKey :many
-- The nodes alive at the notebook's root whose title keys are keys, by nodes_notebook_id_name_key_root_key.
SELECT id, kind, name_key FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND deleted_at IS NULL
    AND name_key = ANY(sqlc.arg(keys)::text[]);
```

```sql
-- name: DeleteExpiredSessionsOfNotebook :execrows
-- An upgrade's start's first look at the sessions (M11 design 4.4 step 7), under the notebook's row lock: the rows
-- expired at now go, tombstones among them, as an opening's first step does for its page (DeleteExpiredSessionsOf),
-- so that a heartbeat that read an earlier time finds no row to keep alive.
DELETE FROM edit_sessions WHERE notebook_id = sqlc.arg(notebook_id) AND expires_at <= sqlc.arg(now);

-- name: LiveSessionsOfNotebook :many
-- The sessions alive at now in the notebook, the first 100 by page, and how many there are.
SELECT node_id, user_id, count(*) OVER () AS total
FROM edit_sessions
WHERE notebook_id = sqlc.arg(notebook_id) AND ended_reason IS NULL AND expires_at > sqlc.arg(now)
ORDER BY node_id, created_at, id
LIMIT 100;
```

- `Capture` 不连 `nodes` 以外的模块的表；正文由这本笔记本的节点逐个经 `page_contents_pkey` 读（`LATERAL … LIMIT 1`：不带 `LIMIT 1` 时子查询被拉平成连接；原来的普通连接在库里有别的笔记本、分析过之后，两种计划都对整张 `page_contents` 顺序扫描），发起的成本随这本笔记本、不随实例（S2 审查 S2B-M2，决定 240；成本守卫核对它的计划，7.1）；页一定有一行活的正文（`checkPages` 的"pages without exactly one content"，`bootstrap/interleavings_page_test.go:91-92`），所以 `upgrade_nodes_page_check` 不会因缺正文被违反。
- `SetWanted` 以两个数组（按 id 排好）调一次；数组为空时不发。
- `SeedSpec`（`page/app/seed_writes.go:28-34`）加 `Upgrade uuid.UUID`（"the upgrade the pages are seeded for, the zero id for none: its unit is the upgrade's own"），`Seed`（`:42-62`）的 `UnitSpec` 加 `UpgradeOwn: spec.Upgrade`。模板的创建（`wiki/app/template.go:86`）照旧不设。
- page 的笔记本删除的订阅者另删这些笔记本的 `upgrade_nodes`（`DELETE FROM upgrade_nodes WHERE notebook_id = ANY(…)`，在它已有的语句之后）。
- 测试：`page/adapter/postgres` 的 `TestTheBaselineIsTheNotebookAsItStarted`（`INSERT … SELECT` 等于逐行读：已删的不在、附件没有版本列、页的哈希等于 `accepted_hash`、数对）、`SetWanted`（不是基线页的 id 改 0 行）、`RootNodes`、两条会话的查询（过期的与墓碑删掉、活着的按页、`LIMIT 100` 时总数照数，第 101 个会话不在列表里）；`page/app` 的 `TestASeedForAnUpgradeIsItsOwn`（升级中的笔记本里 `Seed` 带 `Upgrade` 写得进、变更集带 `upgrade_id`，不带时答 `notebook.upgrading`）；`wiki/app` 的 `TestTheTemplateSeedsAnUpgrade`（假的 `Seeds`：只写缺的键的页、带启动时解析的 `Parsed`、`Upgrade` 是这次的、两种语言）。

### 3.13 S2：lint 的 `computeIn`、`TryHold` 与按页的待建数

**`computeIn`**（`wiki/app/lint.go`）：`compute`（`:198-272`）的 `WithinSnapshot`（`:201`）里那一段搬进

```go
// computeIn computes query in scope, as of at, reading through the
// transaction ctx carries: a snapshot's (compute) or an upgrade's start's
// write transaction (M11 design 4.7).
func (l *Lint) computeIn(ctx context.Context, scope LintScope, query RuleQuery, kind domain.LintQueryKind,
	at time.Time) (RunOutput, error)
```

`computedAt` 用 `at`（今天在快照里读时钟，`:209`）；`compute` 改为 `WithinSnapshot(ctx, func(snapshot) { output, err = l.computeIn(snapshot, scope, query, kind, l.deps.Clock.Now().UTC()) })`，交付之后的 `ctx.Err()` 核对（`:266-268`）照旧在 `compute` 里。各端口本来只用 ctx 的事务（00 号文档 4.7）。`computeIn` 再分两段（2.1 第 11 条，P1 审查 B-N3）：

```go
// prepareIn reads the notebook's state and scope and prepares query's
// rules through the transaction ctx carries, projecting nothing: an
// upgrade's start reads only the stale pages and the wanted counts.
func (l *Lint) prepareIn(ctx context.Context, scope LintScope, query RuleQuery) (PreparedRules, RunStats, error)
```

  `computeIn` 是 `prepareIn` 加今天的投影（`:250-262`）；发起只调 `prepareIn`，`query` 是 `RuleQuery{Rule: &wanted, Limit: 1, WantedByPage: true}`（只准备 `wanted` 一条规则，`Limit` 照规则页的下限、不读条目），之后读 `StalePages()`、`RuleCount(ctx, wanted)` 与 `WantedByPage()`。P2/S2 的检查用 `computeIn`。测试：`wiki/app` 的 `TestComputeInTakesItsMoment`（`computeIn` 用调用方的时刻，`compute` 在快照里读一次时钟；`computeIn` 的调用恰是 `state, nodes, prepare, encode, paths`，`prepareIn` 恰是 `state, nodes, prepare`；S2 偏离 8，S2C-N1）。

**`TryHold`**（`wiki/app/lint_runs.go`）：

```go
// TryHold takes a slot now or not at all, never waiting (M11 design 4.7):
// an upgrade's start holds it under the notebook's row lock. false when
// every slot is taken or Runs has stopped. A holder counts in active, so
// Stop waits for its release, which the start's own deadline bounds.
func (r *Runs) TryHold() (release func(), ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, false
	}
	select {
	case r.slots <- struct{}{}:
	default:
		return nil, false
	}
	r.active++
	var once sync.Once
	return func() {
		once.Do(func() {
			<-r.slots
			r.mu.Lock()
			r.active--
			if r.closed && r.active == 0 {
				close(r.drained)
			}
			r.mu.Unlock()
		})
	}, true
}
```

名额与 `work` 的取法（`:139-145`）同一个 `slots`；放开照 `finish`（`:168-204`）的 `active--` 与 `drained`（`:198-201`）。持有者不登记 flight，`Stop`（`:221-257`）不取消它，只等它放开。发起不经 `Do`、没有 flight，`logFinished`（`:265-322`）不记它，发起自己的日志行有耗时（3.23）；检查、条目、提交的 `RunKey` 种类与 `Scope` 是 P2/S2 的（2.1 第 11 条）。测试（`wiki/app/lint_runs_test.go`）：`TestTryHoldNeverWaits`（名额占满时立即答 `false`，不等；放开之后可取；`release` 调两次只放一次）、`TestStopWaitsForTheHolders`（`Stop` 之后 `TryHold` 答 `false`，`Stop` 等到持有者放开才返回，`active` 回到 0）、`TestAHolderTakesAWorkersSlot`（持有者占着唯一的名额时 `Do` 的 worker 等它）。

**按页的待建数**（linking，`linking/app/lint_edges.go:23-87`）：`RuleQuery`（`wiki/app/lint_ports.go:82-86`）加 `WantedByPage bool`（只与 `wanted` 一条规则同用，别的组合 `lintQuery` 答 `errLintQuery`；S2 偏离 2），经组合根的 `wikiRules`（`bootstrap/wiki_lint.go:120-156`）交给 linking 的计划；`PreparedRules`（`lint_ports.go:100-105`）加 `WantedByPage() map[uuid.UUID]int`。linking 里：

- 第一遍：`prepareWanted` 在换发出页（`:46`）之前与流的末尾把 `len(seen)` 记进 `byPage[source]`（只有带资格的目标，`seen` 本来就是它，`:59-63`）。这条流只有 `generalEmitter` 的页（`:42`：显式的 LLM 契约下是区外的当前页）。
- 第二遍（核对 B2-I1）：范围的模式是 LLM Wiki、要按页的数时（实施只看模式、不看契约是不是显式的；今天只有发起传 `WantedByPage`，它交显式的契约；S2 偏离 2），`prepareAreaWanted` 对 `s.excluded`（`linking/app/lint_scope.go:205`）里当前的页（`s.current`，范围在同一个事务里读到的，所以 SQL 不再判当前）另读一条流 `r.reads.WantedOf(s.notebook, pages)`，同样判 `WantedTarget`、按页数，不经 `generalEmitter`；结果并进 `byPage`，不进规则的条目与数（`wanted` 规则照旧只数区外的页）。
- 读（2.1 第 13 条，P1 审查 B-I1；第 235 条）：不把整组页一次传入、在连接之后排序分页（审查者 B 实测平方级）。`WantedOf` 照 `SourceValues`（`linking/app/lint_reads.go:271-310`）：页去重、按 id 排好（不排好答错误，同 `:274-279`），每 `LintReadRows = 512`（`:15`）个一段；段内照（页、位置）的游标分批，每批只传这一段里游标所在的页及其后的页，由 `page_links` 的主键 `(source_id, range_start)`（`00021_linking_page_links.sql:25`）的数组扫描按键序交回，不对一段的边排序；一段读完换下一段。取目标照 `lintBatches` 的 `maxPayload`（同 `Wanted`）。查询（`linking/adapter/postgres/queries/lint.sql`）：

```sql
-- name: LintAreaWantedEdgeHeads :many
-- The wanted edges' heads of up to LintReadRows given current pages after the cursor (M11 design 4.7 gate 6's
-- second pass): the pages drive page_links' primary key through the array, whose btree scan returns the rows in key
-- order, so that no batch sorts a chunk's edges and LIMIT stops early. The whole set at once, joined and then
-- sorted past a cursor, was quadratic (M11/P1 design review B-I1). Custom and generic plans are both guarded. As in
-- the keyset queries above, the edge's eligibility stays inside a LIMIT 1 LATERAL: on the outer scan, its IS NULL
-- with no statistics estimated one row, and a custom plan then sorted all of a page's edges for each batch.
SELECT l.source_id, l.range_start, octet_length(l.target)::bigint AS target_bytes
FROM page_links l
CROSS JOIN LATERAL (
    SELECT 1 WHERE l.resolved_id IS NULL AND l.target_key IS NOT NULL
    LIMIT 1
) wanted
WHERE l.source_id = ANY(sqlc.arg(page_ids)::uuid[])
    AND (l.source_id, l.range_start) > (sqlc.arg(after_source)::uuid, sqlc.arg(after_start)::integer)
ORDER BY l.source_id, l.range_start
LIMIT (SELECT sqlc.arg(take)::integer);
```

  一段的第一批的游标是零 UUID 与 `-1`；`range_start` 的类型照 00021。`LIMIT` 的标量子查询照 `lint.sql:4-9` 的注释的理由（没有统计时的自定义计划不排整段）。新的计划的测试照 `linking/adapter/postgres/lint_plan_test.go`：一段 512 页、每页 10 条与一页 10,000 条两种数据，自定义与通用两种计划，断言走 `page_links_pkey`、没有 `Sort` 节点；做不到时退回审查者实测的形状（一段一次读全部的头，`second_pass.sql` 的 `chunked_all`），段缩到最坏（一页 `MaxLinks = 10,000` 条，`linking/domain/facts.go:114`）也放得下，写进 S2 的记录（第 235 条）。S2 做到了：`linking/adapter/postgres/lint_area_plan_test.go` 的 `TestLintPlansOfTheAreaPass`；为此待建的过滤写在 `LIMIT 1` 的 `LATERAL` 里，不写在 `WHERE` 里（没有统计时 `IS NULL` 估 0.005，单页数组的自定义计划选了位图扫描加 Sort；S2 偏离 1）。
- 测试：`linking/app` 的 `TestTheWantedCountsByPage`（两页各自的不同目标数、同一目标两次只算一次、读作附件的与虚拟的不算、没有待建链接的页不在表里即 0；区里的页只在第二遍里、不进规则的数；不要按页的数时不读第二遍；区里的页同样带重复的目标、相对链接 `./Alpha`、图片与虚拟的 `index`，两遍判得一样（S2A-M2）；第二遍交回区外的页答 `ErrLintRows`、交回已解析的边不计（S2A-N2））；`linking/adapter/postgres` 的 `TestTheAreaWantedHeadsGoByBatch`（第一批、一页跨两批、跨段、空段、拒收 513 个 id）与计划的测试 `TestLintPlansOfTheAreaPass`（`!race`，由 `test-go-costs-lint` 已有的 `^TestLint(Plans|…)` 一行跑：没有统计与分析过 × 自定义与通用 × 七种位置都只经 `page_links_pkey` 读一次、没有 Sort）；`linking/app` 的 `TestTheAreaPassReadsByChunk`（`WantedOf` 对 1,500 页分三段读、每段内游标前进、没排好的页答错误、替身拒收 512 个以上的 id）。

### 3.14 S2：`startUpgrade` 与 `getUpgrade`

**用例** `StartUpgrade.Execute(ctx, notebookID uuid.UUID, locale string) (UpgradeView, error)`（`wiki/app/start_upgrade.go`），00 号文档 4.4 的次序，各步的码：

1. `locale` 空时取 `en`，不是 `zh-CN`、`en`：422 `validation_failed`（字段 `template_locale`、`invalid_format`，照 `createNotebook`）。
2. `Notebooks.WorkspaceOf` 不加锁：没有答 404 `notebook.not_found`。
3. `ctxStart, cancel := context.WithTimeout(ctx, d.StartTimeout)`（组合根交 `RequestTimeout × 3/4`，第 227、232 条，S2 偏离 16）；`Tx.WithinTx(ctxStart, fn)`，`fn` 里：`Workspaces.ShareByID`（没有：404）→ `States.Lock`（没有：404）→ 判定 `UpgradeAction`（`shared.ErrNotVisible`：404；`forbidden`：403）→ `at := Clock.Now().UTC()`（之后每一步都用它）。
4. `Mode == llm_wiki`：409 `notebook.already_llm_wiki`。
5. `Upgrade.ID` 不为零：409 `notebook.upgrading`。
6. `Writing.Writing(notebookID)` 为真：409 `transfer.busy`（`transfer.NewWritingJobs(pool)`，`transfer/writing_jobs.go:26`，同降级）。
7. `Upgrades.ClearExpiredSessions(notebookID, at)`；`Upgrades.LiveSessions(notebookID, at)` 有：经 `Names` 取显示名，409 `wiki.upgrade_pages_locked`，`Locks` 是至多 100 项（`shared.Error.Locks`，`shared/error.go:119-121`，同 `linking.pages_locked` 的 `domain.PagesLocked`，`linking/app/rewrite.go:775-789`），`detail` 写总数。
8. `Upgrades.RootNodes(notebookID, ["raw", "agents"])`：有一个是附件：409 `wiki.upgrade_title_taken`；`raw` 是页：它是提议的根；`agents` 是页：留着。
9. `release, ok := Runs.TryHold()`；不 `ok`：503 `server_busy`（`shared.ServerBusy(time.Second)`）。`release` 记在 `fn` 之外，`WithinTx` 返回之后才调（"提交；放开名额"）：计算与提交时持着、`ANALYZE` 之前已放开，名额占满时第 4–8 步照答自己的码，由 `wiki/app` 的 `TestUpgradeStart` 钉住（S2 审查 S2A-M1 = S2C-N3）。
10. `id := uuid.NewV7()`；`States.Start(notebookID, Upgrade{ID: id, State: upgrading, By: actor, At: at})`。`Start` 断言 `up.ID` 不为零，零是缺陷：00055 的检查收零 UUID，程序却处处以零当"没有升级"（S1 审查 S1B-N1）。
11. `base := Upgrades.Capture(id, notebookID)`。
12. 缺的页（P1 审查 B-M1：复用启动时解析好的模板，同建 LLM Wiki，13.3 第 3 条的例外只用一次）：`*app.Template`（`wiki/app/template.go:23-53`，`NewTemplate` 在启动时解析每种语言的每一页）加

    ```go
    // SeedUpgrade writes, in one unit of the upgrade upgradeID, the pages of
    // locale's template whose title keys are among keys (raw, agents: the
    // ones the notebook lacks), new ids, parsed at start-up, at the root's
    // end; it answers the written ids by title key.
    func (t *Template) SeedUpgrade(ctx context.Context, notebookID, upgradeID uuid.UUID, locale domain.Locale, client string,
        at time.Time, keys []string) (map[string]uuid.UUID, error)
    ```

    它经 `t.seeds.Seed` 写一个 `Seed`（`wiki/app/ports.go:14-27`，端口的类型是 `Seed`、页是带 `Parsed` 的 `SeedPage`），`Seed` 加 `Upgrade uuid.UUID`，说明是模板的（`domain.TemplateMessage`，`wiki/domain/template.go:50-58`）；组合根的 `wikiSeeds.Seed`（`bootstrap/wiki.go:24-31`）把它转成 `page.SeedSpec.Upgrade`。写了 `raw` 时它是提议的根，`Upgrades.Protect(id, notebookID, root)`。
13. `prepared, stats := Lint.prepareIn(ctx, LintScope{显式的 llm_wiki 契约, 根}, RuleQuery{Rule: &wanted, Limit: 1, WantedByPage: true})`（3.13；`prepareIn` 不投影，用不到时刻）；`prepared.StalePages() > 0`：409 `wiki.upgrade_stale_index`；`Upgrades.SetWanted(id, prepared.WantedByPage())`；`prepared.RuleCount(ctx, wanted)` 是报告用的总数与是否封顶。
14. `Rows.Insert`：wiki 的行（`upgrading`、根、`seeded_root`、`seeded_agents_id`、语言、`base`、总数、审计列）。
15. `States.Publish(ModeChange{From: standard, To: standard, …})`。
16. `WithinTx` 返回：错误包着 `context.DeadlineExceeded`、`ctxStart.Err() != nil`、`ctx.Err() == nil` 三样同时成立时记一行 warn（3.23）、答 503 `server_busy`（`Retry-After: 1`）；别的错误照它自己的答。交回 nil 就是已提交：`Upgrades.Analyze(ctx)`（请求的 ctx），失败记一行 warn、答复不变；记 info；答 200 `Upgrade`。

`StartTimeout` 是 `wiki.Deps` 的新项。组合根原交 `LintTimeout`（`bootstrap/wire.go:249`）；成本守卫（7.1）量出 10 万页放不下（本机 2.2–2.4 s，离守卫配置下 3 s 的期限只余约 20%，照 lint 守卫在 CI 上的倍率折到 2.5–5.6 s），S2 照第 232 条改交 `RequestTimeout × 3/4`（`bootstrap/wire.go:264`；守卫 4.5 s，生产 11.25 s，默认 4 s 的请求下 3 s），守卫的规模减为 5 万页，10 万页进 M12 的性能移交（S2 偏离 16，10.2）。

**`GetUpgrade.Execute(ctx, notebookID) (UpgradeView, error)`**：`WorkspaceOf`（404）→ 判定 `UpgradeAction`（404、403）→ `Rows.Current(notebookID)`（进行中的三种状态、`deleted_at IS NULL`）没有：404 `wiki.upgrade_not_found` → `Names` 取发起人与提交人的显示名 → 视图。S2 的回滚各格都为 `null`（S3 加 `rollback_left` 的现读）。不加锁、不在快照里：各格都来自一行。

**错误**（`wiki/domain/upgrade_errors.go`）：`ErrAlreadyLLMWiki`（409 `notebook.already_llm_wiki`）、`ErrUpgrading`（409 `notebook.upgrading`，同 page 的文字）、`ErrWriting`（409 `transfer.busy`，文字照 `notebook/domain/errors.go:31-32`，末句改为"…start the upgrade once it is done."，S2 偏离 4）、`ErrUpgradePagesLocked`（409 `wiki.upgrade_pages_locked`）、`ErrUpgradeTitleTaken`（409 `wiki.upgrade_title_taken`："A node at the root titled raw or AGENTS is an attachment: rename it, then start the upgrade."）、`ErrUpgradeStaleIndex`（409 `wiki.upgrade_stale_index`："The link index is out of date: a server admin runs nervewiki reindex, then the upgrade can start."）、`ErrUpgradeNotFound`（404 `wiki.upgrade_not_found`）。

**契约**（`api/modules/wiki.yaml`；`api/openapi.yaml` 的路径表（`:164-167` 的 wiki 一段）加 `/api/v0/notebooks/{notebook_id}/upgrade`，wiki 标签的说明（`:31-32`）加"and its upgrade"）：

```yaml
  /api/v0/notebooks/{notebook_id}/upgrade:
    get:
      operationId: getUpgrade
      summary: The notebook's upgrade to an LLM Wiki under way, for its admins.
      x-problem-codes: [notebook.not_found, forbidden, wiki.upgrade_not_found]
      responses:
        "200": {description: The upgrade., content: {application/json: {schema: {$ref: "#/components/schemas/Upgrade"}}}}
    post:
      operationId: startUpgrade
      summary: >-
        Starts upgrading a standard notebook to an LLM Wiki: records its baseline, writes the AGENTS and raw pages it
        lacks, and freezes its content but for its initiator's MCP writes.
      x-problem-codes: [validation_failed, notebook.not_found, forbidden, notebook.already_llm_wiki, notebook.upgrading,
        transfer.busy, wiki.upgrade_pages_locked, wiki.upgrade_title_taken, server_busy, wiki.upgrade_stale_index]
      requestBody:
        required: true
        content: {application/json: {schema: {$ref: "#/components/schemas/UpgradeStart"}}}
      responses:
        "200": {description: The upgrade started., content: {application/json: {schema: {$ref: "#/components/schemas/Upgrade"}}}}
```

（参数、错误答复与块的展开照 `listLintItems`（`wiki.yaml:38-85`）与 `downgradeNotebook`（`notebook.yaml:256-271`）的写法，上面是缩写。）新的 schema：`UpgradeStart {template_locale?: zh-CN | en}`（`additionalProperties: false`）；`UpgradeState`（五种）；`UpgradeEnding`（`abandoned`、`rejected`）；`UpgradeBaseline {nodes, pages, wanted: LintCount}`；`UpgradeRollbackLeft {nodes, pages, created}`；`Upgrade`：`id`、`notebook_id`、`state`、`started_by`（`UserRef`，`api/common.yaml:190-200`）、`started_at`、`source_root_id`、`seeded_page_ids`（0–2 项）、`baseline`、`submitted_by`（`UserRef | null`）、`submitted_at`、`rollback_job_id`、`ending`、`rollback_complete`、`rollback_left`（`UpgradeRollbackLeft | null`，说明写"Where a rollback ended short: the nodes not back in place, the pages not written back, the new nodes still there. In pages not yet written back, links the system rewrote may lead nowhere."），全部必填、`additionalProperties: false`；`report` 在 P2/S2 加（2.1 第 6 条）。`api/common.yaml` 的 `locks`（`:72-80`）的说明"The edit locks linking.pages_locked names, one a page: … No other code carries it."改为"The edit locks linking.pages_locked and wiki.upgrade_pages_locked name, one a page: each page whose links a rename or move would write again, or that an upgrade could not start beside, and who edits it (the caller themself when they edit it); at most 100, the detail saying how many there are. No other code carries it."

**适配器**：wiki 的 `adapter/http`（`handler.go:19-30` 的 `Register` 加两个操作；`oapi-codegen.yaml` 与 `Makefile:116-119` 的生成照旧）；每个声明的码在 `wiki/adapter/http` 的测试里经 `apitest.CheckResponse` 答出（假的用例）。

### 3.15 S2：接线、清理与 `checkPages`

- `wiki.Deps`（`wiki/lint.go:72-86`）加：`Pool`（wiki 自己的表的适配器 `postgresadapter.New(d.Pool)`；S2 偏离 3）、`Tx`（连接池的事务）、`Workspaces`（`workspace.NewWorkspaces(pool)`）、`UpgradeStates`（`notebook.NewUpgradeStates(pool, modeSubscribers)`）、`Upgrades`（`page.NewUpgrades(pool)`）、`Writing`（`transfer.NewWritingJobs(pool)`）、`Names`（`displayNames{identity.NewDirectory(pool)}`，`bootstrap/display_names.go:13`）、`UpgradeAction`（`notebook.ActionChangeMode`）、`StartTimeout`、`Template`（`bootstrap/wire.go:196` 建好的 `*wiki.Template`，模块根的别名；今天 `wiki.Deps` 没有 `Seeds`，它只交给 `wiki.NewTemplate`）。组合根的 `wikiDeps`（`bootstrap/wire.go:239-250`）填它们；`wiki.New`（`:254`）照旧在 transfer 之后。
- `wiki/adapter/postgres`：`queries/upgrades.sql` 的 `InsertUpgrade`、`CurrentUpgrade`（`WHERE notebook_id = $1 AND state IN (…) AND deleted_at IS NULL`，走部分唯一索引）、`SoftDeleteUpgradesOf`（笔记本删除）、`PurgeUpgrades`（照别的模块的清理器）。`server/sqlc.yaml` 加 wiki 的一项（schema 只有 00057，`REFERENCES notebooks` 照 `archtest/sqlc_test.go:113-116` 许跨模块的引用；`TestSQLCSchemaScope`，`:18-36`，要有查询的模块有一项）。
- 授权：`deploy/runtime-grants.sql` 的数据授权（`:17-20`）加 `notebook_upgrades`。
- 清理器：`wiki.Purgers(pool)` 进 `bootstrap/registrants.go:311-314`，排在 notebook 之前（外键 `ON DELETE RESTRICT`）；`bootstrap/purge_test.go`（`:56`、`:71`、`:98`）的"每张带 `deleted_at` 的表都有清理器"照旧过。
- 笔记本删除的订阅者：`wiki.NewNotebookDeletion(pool)`（只凭连接池：命令行的路径上不许建 `wiki.New`）进 `notebookRegistrants`（`registrants.go:189-204`），以删除的那一刻软删除这些笔记本的行；page 的订阅者同时删 `upgrade_nodes`（3.12）。
- 运行时角色：`TestTheRuntimeRoleServesWithTheGrantsFile`（`bootstrap/runtime_role_test.go:128`）照它对导入的写法（`:334-336`）加一段：先以 owner 的 SQL 给那本笔记本加一个管理员（S2 偏离 14），再以运行时角色发起一次升级，断言答 200、`upgrade_nodes` 的 `last_analyze` 变了、日志里没有 `ANALYZE` 失败的那行 warn（核对 B2-M3）。
- `checkPages`（`bootstrap/interleavings_page_test.go:58`，规则的表 `:83-151`）加（只看活着的笔记本；库里有 `notebook_upgrades` 时才加，`to_regclass('notebook_upgrades')`，`:156`：M10 的迁移夹具 `TestIngestMigrationCostsLongHistory` 在退到 53 的库上调 `checkPages`，S2 偏离 6）：有升级列的笔记本恰有一行同 id、同状态、同发起人的进行中的 wiki 行；没有升级列的笔记本没有进行中的 wiki 行；进行中的升级的基线行数等于 `baseline_nodes`、基线页数等于 `baseline_pages`；基线行与保护行只属于进行中的升级；`upgrading`、`reviewing` 中提议的来源根活着、是页、属于这本笔记本（回滚先删会把补写的 `raw` 挪进回收站，所以不管 `rolling_back`）；`seeded_agents_id` 不为空时属于这本笔记本；LLM Wiki 没有升级列（00055 的检查本来就挡住，规则留作对照）。每条规则照 `TestPageViolationsCatchEachLLMWikiGap`（`:173`）的写法各有一个以 SQL 造出的违反，规则被删时它失败。测试名为 `TestPageViolationsCatchEachUpgradeGap`（`bootstrap/page_violations_upgrade_test.go`，第四份：不以 `TestUpgrade` 起名）；每行断言七条升级规则里只有它自己的报，"同状态"、基线页数与源根"是页"三个合取另各有一行（S2B-N3）；"同笔记本"一个合取没有单独的行：可以造（wiki 行指向一本软删除的笔记本时只有它报），有主键与部分唯一索引挡着，省了。S1 的整个程序测试的夹具（以 SQL 设升级列）在这个提交里补上 wiki 的行（00 号文档第 7 节"大小与绿"）。

### 3.16 S3 的文件

| 文件 | 改动 |
|---|---|
| `server/migrations/sql/00058_page_changesets_rollback.sql`、`00059_transfer_rollback_jobs.sql`；`server/sqlc.yaml`；`schema_test.go`、`upgrade_test.go` | 3.17 |
| `page/domain/changeset.go`、新的 `page/domain/rollback.go`、`page/domain/jobs.go`；`page/app/jobs.go`、新的 `page/app/rollback.go`、`unit_revert.go`；`page/upgrades.go`、`page/adapter/postgres/queries/upgrade.sql`；`page/module.go`、`page/tree_writes.go` | 3.18 |
| `transfer/domain/job.go`；`transfer/app/background.go`、`job_run.go`、`maintain.go`、`cancel.go`、`start_background.go`、`ports.go`、新的 `rollbacks.go`；`transfer/adapter/river/jobs.go`、`transfer/adapter/postgres/queries/jobs.sql`；`transfer/module.go`；`bootstrap/transfer.go`、`transfer_notebooks.go` | 3.19 |
| 新的 `wiki/app/abandon_upgrade.go`、`end_upgrade_rollback.go`、`rollback_end.go`；`wiki/rollback_end.go`（模块根）；`notebook/upgrade_states.go`；`bootstrap/wire.go`、`deps.go` | 3.20 |
| `api/modules/{wiki,page,transfer}.yaml` | 3.21 |

### 3.17 S3：迁移 00058 与 00059

```sql
-- 00058_page_changesets_rollback.sql
-- changesets: an upgrade's rollback (M11 design 4.10, decision 192), a kind of its own: it reverts no one changeset,
-- so not revert, whose reverts_id changesets_reverts_check asks for. Its upgrade_id is the upgrade rolled back.

-- +goose Up
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check
        CHECK (kind IN ('edit', 'import', 'version_restore', 'trash_restore', 'revert', 'rollback'));

-- +goose Down
-- A program before it knows no rollback: its changesets read as edits, their writes kept (as 00034's Down).
UPDATE changesets SET kind = 'edit' WHERE kind = 'rollback';
ALTER TABLE changesets DROP CONSTRAINT changesets_kind_check,
    ADD CONSTRAINT changesets_kind_check CHECK (kind IN ('edit', 'import', 'version_restore', 'trash_restore', 'revert'));
```

```sql
-- 00059_transfer_rollback_jobs.sql
-- transfer_jobs: an upgrade's rollback (M11 design 4.10), a job that writes its notebook's tree as a restore and a
-- revert do, one at a time with them; upgrade_id is the upgrade it rolls back (no key: the wiki module's), set on it
-- alone. Its changeset is made by its first unit that writes, as theirs.

-- +goose Up
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export', 'trash_restore', 'revert', 'rollback')),
    ADD COLUMN upgrade_id uuid,
    ADD CONSTRAINT transfer_jobs_upgrade_check CHECK ((upgrade_id IS NOT NULL) = (kind = 'rollback')),
    DROP CONSTRAINT transfer_jobs_root_check,
    ADD CONSTRAINT transfer_jobs_root_check CHECK (CASE kind WHEN 'trash_restore' THEN root_id IS NOT NULL
        WHEN 'revert' THEN root_id IS NULL WHEN 'rollback' THEN root_id IS NULL ELSE true END);
DROP INDEX transfer_jobs_writing_key;
CREATE UNIQUE INDEX transfer_jobs_writing_key ON transfer_jobs (notebook_id)
    WHERE kind IN ('import', 'trash_restore', 'revert', 'rollback') AND state IN ('queued', 'running')
        AND deleted_at IS NULL;

-- +goose Down
DELETE FROM transfer_jobs WHERE kind = 'rollback';
DROP INDEX transfer_jobs_writing_key;
CREATE UNIQUE INDEX transfer_jobs_writing_key ON transfer_jobs (notebook_id)
    WHERE kind IN ('import', 'trash_restore', 'revert') AND state IN ('queued', 'running') AND deleted_at IS NULL;
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_root_check,
    ADD CONSTRAINT transfer_jobs_root_check CHECK (CASE kind WHEN 'trash_restore' THEN root_id IS NOT NULL
        WHEN 'revert' THEN root_id IS NULL ELSE true END),
    DROP CONSTRAINT transfer_jobs_upgrade_check, DROP COLUMN upgrade_id,
    DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export', 'trash_restore', 'revert'));
```

最新的定义照抄自 `00034_page_changesets_history.sql:7`（`changesets_kind_check`）与 `00039_transfer_background_jobs.sql:8-9`、`:18-19`、`:26-27`（另两个检查与写入键）；`transfer_jobs_changeset_check`（`:13-14`，非导出、非导入要变更集）、`transfer_jobs_target_check`（`:15`）、`transfer_jobs_message_check`（`:16-17`）对回滚都照旧成立（有变更集、没有目标与说明），不改。`server/sqlc.yaml` 的 page 一项加 00058、transfer 一项（`:177-200`）加 00059。`transfer/adapter/postgres/queries/jobs.sql:28-35` 的 `Writing` 的种类表加 `'rollback'`，注释的"an import, a restore or a revert"加"or a rollback"；`CreateJob`（`:4-9`）与读任务的各条加 `upgrade_id`。名称：`"transfer_jobs_upgrade_check c"`。反例：回滚没有 `upgrade_id`、撤销带 `upgrade_id`、回滚带 `root_id`、不认识的种类 `'rollback2'`（kind 的检查）、同一本笔记本一个排队的导入与一个排队的回滚（写入键，23505）。Down：`downTo(t, m, 58)` 之后回滚的任务行不在、导入照在；`downTo(t, m, 57)` 之后回滚的变更集成了 `edit`。

### 3.18 S3：page 的回滚

**一遍**（2.1 第 5 条、第 12 条）：

```go
// page/app/rollback.go

// RollbackPass is what one pass of an upgrade's rollback does (M11 design
// 4.10): delete the new subtrees that hold no baseline node, or, with
// none left, put the rest back by a plan made from the baseline; or
// nothing, the notebook being as the upgrade started.
type RollbackPass struct {
	Deletes bool
	Done    bool
	Left    domain.RollbackLeft
}

// PlanRollback plans one pass of the rollback of spec's upgrade, unlocked
// (the notebook is frozen, the rollback its one writer): the upgrade must
// still be the notebook's, rolling back, else ErrRollbackGone.
func (w *JobWrites) PlanRollback(ctx context.Context, spec JobSpec, nominal func([]domain.JobStep) [][2]int) (JobPlan, RollbackPass, error)
```

`JobSpec`（`page/app/jobs.go:24-29`）加 `UpgradeID uuid.UUID`；`JobPlan`（`:34-38`）加 `rollback *rollbackPlan`（先删的子树的包，或合成的撤销计划）。次序：

1. `Notebooks.StateOf`：`Upgrade.ID != spec.UpgradeID` 或不是 `rolling_back`：`ErrRollbackGone`（组合根交回 transfer 的 `ErrJobGone`，任务只记一行"stopped"、不写结束，第 233 条、3.19）。
2. `Upgrades.Left(upgradeID, notebookID)`（下面的 SQL）全是 0：`Done`。
3. 读不含基线节点的新建子树的森林（下面的 `FreeForest`）：不空时 `domain.PackFreeSubtrees(forest, MaxUnitNodes)` 把它们装成步：每一步是若干棵合计至多 `MaxUnitNodes` 个节点的子树的根（根两两不相干，`DeleteNodes` 一次删一整棵，`page/adapter/postgres/store.go:447-457`）；超过界线的一棵在这一遍只删它下面的不超过界线的极大子树，它自己留到下一遍。`Deletes`。
4. 否则合成：`domain.RollbackState(baseline, now)` 造出 `RevertState`（`page/domain/revert.go:110-121`）：`Items` 是位置、名称或次序值任一与基线不同的基线节点（`Before` 基线、`After` 现在）与收着基线节点的新建节点（`Before == nil`），`Revision` 都是 0、`Restored` 为空；`Versions` 是哈希与基线不同的基线页（`Base` 是基线的版本号、`BaseBytes` 基线的字节数、`Rewrite = false`、`Revision`、`Hash` 是现在的）；`Hashes` 里基线版本的哈希取自基线的行；`Nodes`、`Contents`、`Tree`、`Lines`、`Groups`、`At`、`Retention` 照 `revertReads` 的 `state`、`rest`（`page/app/unit_revert.go:384-416`）读（`NodesNow`、`ContentsNow`、`ListRevertTree`，`ports.go:174-175`、`:581-582`）。条目数先数：超过 `MaxRollbackItems = 300,000`（`page/domain/jobs.go` 新的常量，同 `MaxJobItems`，`:29`）答 `page.revert_too_large`；版本超过余下的额度时按页 id 取前面的，其余留到后面的遍。再 `PlanRevert`（`revert.go:373`）与 `revertSteps`（`jobs.go:378`）。`ErrRevertUnordered`（`revert.go:98`）是缺陷，照旧以 500 的类别交回。

**读**（`page/adapter/postgres/queries/upgrade.sql`）：

```sql
-- name: RollbackFreeForest :many
-- The new nodes alive in the notebook that hold no baseline node, with their parents (M11 design 4.10): the
-- holders are the ancestors of the baseline nodes alive; a free node is a new one that is no holder. The roots of
-- the free subtrees are those whose parent is none, a holder or a baseline node. Depth is at most MaxDepth.
WITH RECURSIVE holders AS (
    SELECT n.parent_id AS id FROM upgrade_nodes b JOIN nodes n ON n.id = b.node_id
    WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline AND n.deleted_at IS NULL AND n.parent_id IS NOT NULL
  UNION
    SELECT n.parent_id FROM holders h JOIN nodes n ON n.id = h.id WHERE n.parent_id IS NOT NULL
)
SELECT n.id, n.parent_id FROM nodes n
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL
    AND NOT EXISTS (SELECT 1 FROM upgrade_nodes b WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.node_id = n.id
        AND b.baseline)
    AND n.id NOT IN (SELECT id FROM holders);

-- name: RollbackLeft :one
-- What a rollback has left to do (M11 design 4.10, decision 226): the baseline nodes not in place (parent, name or
-- order not the baseline's, or not alive), the baseline pages whose content is not the baseline's, and the live
-- nodes with no baseline row. getUpgrade, endUpgradeRollback and the rollback's end read it alike.
SELECT
    (SELECT count(*) FROM upgrade_nodes b LEFT JOIN nodes n ON n.id = b.node_id AND n.deleted_at IS NULL
     WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline AND (n.id IS NULL
         OR n.parent_id IS DISTINCT FROM b.parent_id OR n.name <> b.name OR n.sort_order <> b.sort_order))::integer AS nodes,
    (SELECT count(*) FROM upgrade_nodes b JOIN page_contents c ON c.node_id = b.node_id AND c.deleted_at IS NULL
     WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline AND b.kind = 'page'
         AND c.content_hash <> b.content_hash)::integer AS pages,
    (SELECT count(*) FROM nodes n WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL
     AND NOT EXISTS (SELECT 1 FROM upgrade_nodes b WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.node_id = n.id
         AND b.baseline))::integer AS created;

-- name: DeleteUpgradeNodes :execrows
-- An upgrade's rows, as it ends (M11 design 4.10).
DELETE FROM upgrade_nodes WHERE upgrade_id = sqlc.arg(upgrade_id);
```

自由的森林在 Go 里按 `parent_id` 建（父节点不在结果里的就是根），`PackFreeSubtrees` 自下而上数大小再装。基线行的读（合成要的全部基线行，一次按 `upgrade_id` 读，走主键的前缀）与"收着基线节点的新建节点"（`holders` 里没有基线行的）同在 `upgrade.sql`，形状照上面两条。`page.Upgrades`（3.12）加 `Left` 与 `DeleteBaseline`。

**批**：`RunBatch`（`page/app/jobs.go:609`）加回滚的分支：先删的包经新的 `Unit.DeleteFreeSubtrees`，每棵 `Subtree`（`unit_delete.go:26-32`）再 `DeleteSubtree`（`:42-66`）；合成的批经新的 `Unit.RollbackBatch(ctx, part RevertPart, plan *rollbackPlan)`，照 `Unit.RevertBatch`（`unit_revert.go:460`）：同样读 `rest`、`nodesNow`（`:357-368`），条目与版本来自合成的状态、不经 `batchChangeset`（`:634-636`），写回的正文经 `History.VersionContents`（`ports.go:584`）按（页、基线版本号）读、不经 `revertContents`（`jobs.go:872`）；其余（`CheckRevertPart`、`CheckRevertSteps`、`place`、`Delete`、`WriteContent`、`revertDefect` `:935-948`）照用。回滚的单元：

```go
func rollbackUnit(spec JobSpec, action shared.Action) UnitSpec {
	return UnitSpec{NotebookID: spec.NotebookID, Action: action, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Kind: domain.ChangesetRollback, Changeset: spec.Changeset, Create: true,
		Message: &rollbackMessage, Background: true, UpgradeOwn: spec.UpgradeID, Options: Options{UpdateLinks: false}}
}
```

`action` 是 page 的 `Deps.UpgradeAction`（2.1 第 10 条），`NewJobWrites`（`jobs.go:82`）多收它；`rollbackMessage` 是固定的英文 `"Upgrade rollback"`；`domain.ChangesetRollback`（`page/domain/changeset.go:21-26` 的常量加一个）。`UpdateLinks` 为假（`linking/app/rewrite.go:63-65`）。变更集的 `upgrade_id` 由 `ensureChangeset` 从 `Write.Notebook.Upgrade.ID` 记上（3.4）。

**组合根**：`transferTree`（`bootstrap/transfer.go:86-88`）加 `PlanRollback` 与回滚的 `RunBatch` 的转发（照 `PlanRevert` 的 `:133-139`），`treeError`（`:232-261`）认 `ErrRollbackGone`，交回 `transfer.ErrJobGone`（第 233 条：今天的 `errGone` 不导出，`transfer/app/job_run.go:32-33`，组合根交不回它；P1 审查 A-N1）。

**测试**：`page/domain` 的 `TestPackFreeSubtrees`（多棵小的装进一步、合计不超过界线、超过的一棵这一遍只删它下面的极大子树、根两两不相干）、`TestRollbackState`（位置、名称、次序各自不同的基线节点各一条；收着基线的新建节点一条 `Before == nil`；正文不同的基线页一个版本、`Base` 是基线的；净效果为零的不进；`Revision` 都是 0；条目数过界答 `revert_too_large`）与 `TestRollbackPlansPutTheBaselineBack`（性质测试，照 `TestScheduleEndsAndPutsTheNotebookBack`，`page/domain/revert_schedule_property_test.go:51`：大的随机形状，改名环、跨层的移动、新建的文件夹收着基线节点、交换名称；`PlanRevert` 不答 `ErrRevertUnordered`，按步重放之后等于基线，而且是一遍：重放之后由基线行算的剩下的差为 0（2.1 第 12 条：合成的一遍本就到达基线，P1 审查 A-M3）；判据由基线行直接比，不经被测的合成）；`page/app` 的 `TestARollbackPassAtATime`（先删的一遍只有删、之后的一遍是合成的、再一次 `PlanRollback` 就是 `Done`，没有第二遍合成；升级换了答 `ErrRollbackGone`）与 `TestTheRollbacksStepsReplayToTheBaseline`（照 `TestTheJobsStepsReplayToStar`，`page/app/revert_job_property_test.go:64`，`cutsBy` `:1031`：任意切批都到基线）；`page/adapter/postgres` 的 `RollbackFreeForest` 与 `RollbackLeft`（"只差次序值"记为不在原位，核对 A3-N2）。

### 3.19 S3：transfer 的回滚

- **种类**：`transfer/domain/job.go` 的种类（`:14-19`）加 `KindRollback Kind = "rollback"`，`Writes()`（`:23-25`）认它；`Job`（`:63-98`）加 `UpgradeID *uuid.UUID`。River（`transfer/adapter/river/jobs.go`）：`RollbackKind`、`RollbackArgs{JobID}`、`Queue.Rollback(ctx, id)` 照 `Queue.Revert`（`:198-202`，`QueueImport`、`MaxAttempts: 1`），`riverKind`（`:206-217`）加一支，worker 在 `transfer/module.go:229-237` 登记。Rescue 的种类表（`transfer/app/maintain.go:108`）加它；`TestTheRescue`（`transfer/app/requests_test.go:339-371`）改为对 `domain` 的全部种类逐个核对（种类的表由新的 `domain.Kinds()` 给出，加一个种类而 Rescue 不认时失败）。
- **端口**：`Notebooks`（3.5）加 `LockByID(ctx, id) (bool, error)`（`FOR NO KEY UPDATE`，组合根经根的 `LockByID`，`notebook/notebooks.go:106-109`）。新的：

```go
// RollbackEnd ends an upgrade as its rollback's job ends (M11 design 4.10):
// the wiki module's, in the job's end's transaction, the notebook's row
// locked first.
type RollbackEnd interface {
	// End ends the upgrade upgradeID of the notebook when its rollback left
	// nothing, and answers whether it left something; nothing, and false,
	// when the notebook's upgrade is no longer it, rolling back.
	End(ctx context.Context, notebookID, upgradeID uuid.UUID) (left bool, err error)
	// Stopped tells the mode's subscribers that the rollback stopped, when
	// the notebook's upgrade is still it, rolling back.
	Stopped(ctx context.Context, notebookID, upgradeID uuid.UUID) error
}
```

  `Deps`（`transfer/module.go`，`:129-181`）加 `RollbackAction`、`RollbackEnd`；`New`（`:200-238`）在没有它们时 panic，这是新加的检查（今天 `New` 对缺的依赖不 panic，`:176-180` 只是字段的说明；P1 审查 A-N5）；建模块的三处交它们：`bootstrap/wire.go:212`、`transfer/contributors_test.go:85`、`transfer/adapter/http/module_background_test.go:37`（测试交一个什么也不做的 `RollbackEnd` 与 `notebook.change_mode` 的值）。
- **开任务**（`transfer/app/rollbacks.go`）：

```go
// Rollbacks start an upgrade's rollback's job in the caller's transaction
// (M11 design 4.10): the wiki module's abandonment. It logs nothing: its
// caller logs once it commits.
type Rollbacks struct{ d StartDeps; action shared.Action }

func (r Rollbacks) Start(ctx context.Context, notebookID, upgradeID uuid.UUID, client string) (uuid.UUID, error)
```

  `startBackground`（`start_background.go:154-198`）拆成 `queueBackground`（`:160-190` 的部分：`room`、`workspaceOf`、`WithinTx` 里的锁、队列锁、`room`、`writing`、建行、投递，交回任务）与 `startBackground`（`queueBackground` 之后记"queued"、造视图）；`Rollbacks.Start` 只调前者，`Job{Kind: KindRollback, UpgradeID: &upgradeID, Name: "upgrade", Client: client}`，动作是 `RollbackAction`。`WithinTx` 加入调用方的事务（`platform/postgres/tx.go:121-129`），`lock` 的 `ShareByID` 在调用方已持的 `FOR NO KEY UPDATE` 之下重入。冻结的判断（3.5）加放过的一支：`Kind == KindRollback && *job.UpgradeID == state.UpgradeID && state.RollingBack`。模块的 `(*Module).Rollbacks() Rollbacks`。
- **运行**：`backgroundRun.run`（`background.go:86-128`）的分派（`:93-102`）加 `KindRollback` 一支，调 `r.rollback(ctx, running, spec)`：`spec` 带 `UpgradeID`；按遍循环：`Tree.PlanRollback`；`Done` 就返回 nil；否则照现有的循环跑这一遍的批（`:107-126`，`batch`、`count`、`publish` 照用；总数逐遍加）；每遍一行 info（3.23）；这一遍之后 `Left` 的三个数之和不比这一遍之前小时以 `internal` 停下（2.1 第 12 条，终止的保证）。`PlanRollback` 交回的 `ErrJobGone` 原样交回 `finish`，不经 `stopped`（`background.go:262-297` 会把它归为 `internal`）：`finish` 照 `errGone` 只记一行、不写结束（第 233 条）。
- **结束**：`Background.Run`（`:54-74`）对回滚的任务给 `finish` 一个 `succeed` 与一个结束的写法；`jobRun`（`job_run.go`）加字段 `finishJob func(end context.Context, e Ended) (bool, error)`，缺省是 `r.rows.FinishJob`（`:211`），回滚的是 `rollbackStopped`：

```go
// rollbackSucceeded ends the rollback's job and its upgrade in one
// transaction: the notebook's row first, as a deletion takes it before the
// job's (jobs.sql:149-152); the upgrade ended when nothing is left; then
// the job's end, and its "rollback ended" line, as the export's succeed
// logs its own (finish logs nothing past a succeed). Something left rolls
// that back and ends the job failed, internal, as rollbackStopped does. A
// notebook deleted meanwhile leaves no row to lock: nothing is written,
// End is not called, and the "stopped" line is logged (as export.go:403-407).
func (r *backgroundRun) rollbackSucceeded(end context.Context, e Ended) error

// rollbackStopped writes a stopped rollback's end in one transaction: the
// notebook's row, the job's end, then RollbackEnd.Stopped when the end
// was written (M11 design 4.10, decision 228). No notebook row: false,
// nothing written, Stopped not called.
func (r *backgroundRun) rollbackStopped(end context.Context, e Ended) (bool, error)
```

  `finish` 里的 `FinishJob`（`:211`）改调 `r.finishJob`；`!ok`（`:215-218`）与 `errGone`（`:196-198`）照旧只记一行，不调 `Stopped`（核对 B3-N2）。
- **取消**：`Cancel.Run`（`transfer/app/cancel.go:49-92`）在 `sees`（`:63`）之后：任务是回滚的，判定 `RollbackAction`，`forbidden` 或看不到都答 403 `forbidden`（`sees` 已过，这里的"看不到"只会是两步之间被移出，照 403）；`CancelQueued`（`:68-70`）照旧在它之后。`views.sees`、读与列表不改（4.3）。
- **测试**：`transfer/app` 的 `TestARollbackRunsPassByPass`（假的 `Tree`：两遍先删、一遍合成、`Done`；总数逐遍加；差不缩小时 `internal`）、`TestARollbacksEnd`（假的 `RollbackEnd`：做完时一个事务里先锁笔记本、`End`、再 `FinishJob`；`left` 时第二个事务以 `internal` 结束并调 `Stopped`；取消、超时、中断各调一次 `Stopped`；`FinishJob` 交回 `!ok` 时不调；笔记本行锁不到时两个钩子都什么也不写、不调 `End` 与 `Stopped`、记"stopped"（P1 审查 A-N2）；做完时记"rollback ended"一行（A-N3），`ErrJobGone` 只记"stopped"）、`TestOnlyAdminsCancelARollback`、`TestTheStartsRefuseAnUpgradingNotebook` 加回滚放过的一支与"升级换了"不放过的一支。

### 3.20 S3：wiki 的放弃、结束回滚与 `RollbackEnd`

- **`UpgradeStates`**（3.11）加 `SetState(ctx, id, upgradeID uuid.UUID, state UpgradeState, by uuid.UUID, at time.Time) error` 与 `Clear(ctx, id, upgradeID, by uuid.UUID, at time.Time) error`：

```sql
-- name: SetUpgradeState :execrows
UPDATE notebooks SET upgrade_state = sqlc.arg(state), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND upgrade_id = sqlc.arg(upgrade_id);

-- name: ClearUpgrade :execrows
UPDATE notebooks SET upgrade_id = NULL, upgrade_state = NULL, upgrade_by_id = NULL, upgrade_at = NULL,
    updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND upgrade_id = sqlc.arg(upgrade_id);
```

- **`AbandonUpgrade.Execute(ctx, notebookID) (UpgradeView, error)`**：`WorkspaceOf`（404）→ `WithinTx`：工作区行、`States.Lock`（404）、判定（404、403）、`at`；没有升级或状态不在 `upgrading`、`reviewing`、`rolling_back`：409 `wiki.upgrade_state`。`upgrading`、`reviewing`：`States.SetState(rolling_back)` → `Publish` → `Rollbacks.Start(notebookID, upgradeID, client)`（`MaxQueued` 满时 503）→ 一条 `UPDATE` 写 wiki 行的 `state = 'rolling_back'`、`ending = 'abandoned'`、`rollback_job_id`（核对 B2-M2）。`rolling_back`：`Writing` 为真 409 `transfer.busy`；否则 `Rollbacks.Start` → 只改 `rollback_job_id` → `Publish`（`From == To`；继续也是表里的一个转移，4.3）。提交之后一行 info（`job_id`）。答 200 `Upgrade`。驳回（P3/S1）走同一个私有的 `rollBack(ending)`。
- **`EndUpgradeRollback.Execute(ctx, notebookID) (UpgradeView, error)`**：锁与判定同上；状态不是 `rolling_back`：409 `wiki.upgrade_state`；`Writing` 为真：409 `transfer.busy`；`left := Upgrades.Left(upgradeID, notebookID)`；`States.Clear`；`Upgrades.DeleteBaseline(upgradeID)`；wiki 行改为 `rolled_back`、`rollback_complete = left.Zero()`，不为零时记三个数（第 230 条）；`Publish`；提交之后 info（三个数）。答 200 `Upgrade`（`rolled_back`）。
- **`RollbackEnd`**（`wiki/app/rollback_end.go`，模块根 `wiki.NewRollbackEnd(pool, states, upgrades)`：`states` 是 `notebook.NewUpgradeStates(pool, modeSubscribers)`，`upgrades` 是 `page.NewUpgrades(pool)`，都只凭连接池；组合根在 transfer 之前预先建它，`bootstrap/wire.go:188-196` 的 `VirtualFiles`、`Template` 旁）：`End`：`States.Lock`（transfer 已持，重入）；升级不是它或不在 `rolling_back`：`false, nil`；`Upgrades.Left` 不为零：`true, nil`；否则 `Clear`、`DeleteBaseline`、wiki 行 `rolled_back` 与 `rollback_complete = true`、`Publish`，`false, nil`。`Stopped`：`States.Lock`，仍是它、仍在回滚就 `Publish`（`From == To`）。执行者是任务的发起人（ctx 的 actor，`background.go:64`），`updated_by_id` 记他。
- **`getUpgrade`** 在 S3 加 `rollback_left`：`rolling_back` 且 `Writing` 为假时 `Upgrades.Left`（现读）；别的时候 `null`。
- **错误**：`ErrUpgradeState`（409 `wiki.upgrade_state`："The upgrade is not in a state that allows this."）。
- **测试**：`wiki/app` 的 `TestAbandonUpgrade`、`TestEndUpgradeRollback`、`TestTheRollbackEnd`（假的端口：每条码依次；`upgrading` 与 `reviewing` 的写的次序过得了 `notebook_upgrades_rollback_job_check`；继续只改任务的 id；结束时三个数为 0 记 `true`）。

### 3.21 S3：契约

- `api/modules/wiki.yaml`：`abandonUpgrade`（`POST …/upgrade/abandon`，`x-problem-codes: [notebook.not_found, forbidden, wiki.upgrade_state, transfer.busy, server_busy]`，说明写"Abandons the upgrade, or continues its stopped rollback: puts the notebook back as the upgrade started, in a background job"）、`endUpgradeRollback`（`POST …/upgrade/end-rollback`，`[notebook.not_found, forbidden, wiki.upgrade_state, transfer.busy]`，说明写"Ends a stopped rollback, keeping the notebook as it is: the upgrade's original pages and nodes are all there; some may not be back in place, and in pages not yet written back, links the system rewrote may lead nowhere"）。没有请求体，照 `downgradeNotebook`（`notebook.yaml:256-271`）。
- `api/modules/page.yaml` 的 `ChangesetKind`（`:1526-1531`）加 `rollback`，说明加"rollback: an upgrade's rollback, which put the notebook back as the upgrade started"。
- `api/modules/transfer.yaml`：`TransferKind`（`:826-828`）加 `rollback`；`TransferJob.kind` 的说明（`:657-658`）加回滚（名称是固定的 `upgrade`）；`cancelTransferJob`（`:370-395`）的 `x-problem-codes`（`:386`）改为 `[transfer.not_found, forbidden, transfer.not_cancellable]`，说明加"An upgrade's rollback only a notebook admin may cancel: whoever else sees it gets forbidden"；`TransferFailure.description`（`:912-947`）的 `forbidden`（`:916-918`）加"or a rollback's (its starter is no longer the notebook's admin: another admin may continue it)"，"A revert's alone"一段的 `revert_too_large`（`:942-944`）加"or a rollback's pass too large"。
- 每个新声明的码在模块的 `adapter/http` 测试里经 `apitest.CheckResponse` 答出（wiki 的两个操作、transfer 的 `cancelTransferJob` 的 `forbidden`）。

### 3.22 矩阵的行

照 00 号文档 4.14：在单独的工作区（照 `wikiSeeded`，`bootstrap/permission_matrix_wiki_test.go:30-45`）建"升级的变体"，以 SQL 种下：五本没有升级的普通笔记本（发起用）、五本 `upgrading`、五本 `reviewing`（S2）、五本 `rolling_back` 且回滚任务已停下（S3，要 00059）。13 个笔记本的列照 `notebookOf`（`bootstrap/permission_matrix_notebook_test.go:166-178`）落在五本上。行在第七份的 `TestUpgradePermissionMatrix`（`matrixRows` 的写法，`bootstrap/permission_matrix_test.go:229-233`；写操作每格一个库的副本加一个应用），覆盖的检查 `TestThePermissionMatrixCoversEveryOperation`（`bootstrap/permission_matrix_coverage_test.go:155`）读到它们：

| 操作 | 变体 | 管理员 | 看得到而不是管理员 | 看不到 | Step |
|---|---|---|---|---|---|
| `startUpgrade` | 没有升级 | 200 | 403 `forbidden` | 404 `notebook.not_found` | S2 |
| `getUpgrade` | `upgrading`、`reviewing` | 200 | 403 | 404 | S2 |
| `abandonUpgrade` | `upgrading` | 200 | 403 | 404 | S3 |
| `endUpgradeRollback` | `rolling_back`（停下） | 200 | 403 | 404 | S3 |
| `cancelTransferJob` | its own rollback, ended | 409 `transfer.not_cancellable` | 403 `forbidden`（开任务之后被降级的发起人） | 404 `transfer.not_found` | S3 |
| `cancelTransferJob` | another's rollback, queued | 200 | 404 `transfer.not_found` | 404 | S3 |

种在哪几本（P1 审查 C-M1）：矩阵的种子是共用的，写操作每格拿它的一个副本（同 `matrixJobs()`，`bootstrap/permission_matrix_transfer_test.go:41-50`）；排队或运行的回滚任务占着写入键（00059），种在哪本，那本的发起、放弃、结束回滚就答 `transfer.busy`。所以"another's rollback, queued"（别人的、排队中的回滚任务）种在 `reviewing` 的五本上：那里只有只读的 `getUpgrade` 一行，写入键不挡别的格；种子不经状态机，`reviewing` 的 wiki 行没有 `rollback_job_id`，它不违反任何检查（任务行的 `upgrade_id` 是那本的升级）。"its own rollback, ended"：每一列以 SQL 开过自己的回滚任务、已结束，不占写入键（排队的不行：写入键，`00039_transfer_background_jobs.sql:26-27`，13 列落在五本上，修订核对 C4-M1），种在 `rolling_back` 的五本上（那里本来就有停下的回滚任务的行）。把 403 放在 `sees` 之前的实现在"another's"一行失败。

种子要过的两项核对（修订核对 C2-N2）：变体以 SQL 种完之后跑 `checkPages`，同 M10 的 LLM Wiki 的变体（`bootstrap/permission_matrix_wiki_test.go:657`）：S2 起它有升级的规则（3.15），所以每本升级中的笔记本要有同 id、同状态、同发起人的 wiki 行，基线行数等于 `baseline_nodes`、基线页数等于 `baseline_pages`，`upgrading`、`reviewing` 的提议的根活着、是页。矩阵的 `targetViolation` 要求路径里的每个 `{…_id}` 是这一列的工作区里种下的行（`bootstrap/permission_matrix_coverage_test.go:142-147`），所以 `cancelTransferJob` 的两个变体的任务 id 登记进 `workspaceOfRow`（`bootstrap/permission_matrix_seeded_test.go:476-477`），S3 加。

S2 的三行没有另种升级的变体（S2 偏离 5）：用 lab 种好的笔记本，每格在自己的库的副本上以 SQL 准备，`startUpgrade` 的一格先删那本笔记本的编辑会话（lab 的种子里会话是活的），`getUpgrade` 的两格由 `upgradeIn` 以笔记本的创建者设成 `upgrading`、`reviewing`；每格之后只核对 `pageViolations` 里升级的几句；覆盖的两个检查 `TestThePermissionMatrixCoversEveryOperation` 与 `TestLintMatrixTargets` 经 `coveredMatrixRows()` 读到这些行（逐个提交的核对发现后者原来只读 `matrixRows()`）。上面的变体工作区 S2 没有用到。

### 3.23 日志

13.1 第 10 条：只记事实。每条一个测试，只读自己子测试开始之后的行（05-lessons 第 1 节）。

- 发起（S2）：info "upgrade started"：`workspace_id`、`notebook_id`、`upgrade_id`、`user_id`、`baseline_nodes`、`baseline_pages`、`seeded_root`、`seeded_agents`、`wanted`、`elapsed_ms`；warn "upgrade baseline not analyzed"：`notebook_id`、`upgrade_id`、`error`；warn "upgrade start timed out"：`workspace_id`、`notebook_id`、`user_id`、`elapsed_ms`、`timeout_ms`（第 232 条）。
- 放弃（S3）：info "upgrade abandoned"（继续时 "upgrade rollback continued"）：id、`user_id`、`job_id`。结束回滚：info "upgrade rollback ended"：id、`user_id`、`nodes`、`pages`、`created`、`complete`。
- 回滚（S3）：transfer 的任务日志照旧（"rollback started"；停下的"rollback ended"由 `finish` 记；做完的"rollback ended"由 `rollbackSucceeded` 自己记，因为 `finish` 在 `succeed` 不为空时直接交回它的结果、不记那一行，`job_run.go:189-191`，同导出的 `export.go:427-428`；P1 审查 A-N3）；每遍 info "rollback pass"：`job_id`、`upgrade_id`、`pass`、`deleted_first`（这一遍删的节点数）、`items`、`steps`；做完 info "upgrade rolled back"：id、`elapsed_ms`（由 `RollbackEnd` 记）。

### 3.24 每个新检查的反向对照

每个新检查都有一个去掉它就失败的测试（全表在 7.3）：

- `notebooks_upgrade_check`、`upgrade_nodes_*`、`notebook_upgrades_*`、`transfer_jobs_upgrade_check` 与写入键：`TestChecksRejectCounterexamples` 的各行（3.2、3.10、3.17）。
- 冻结的每一步：`TestFreezeAdmits` 的表；登记：`TestUpgradeFreezeReachesEveryPath` 的去掉冻结的子测试与 `TestUpgradeFreezeAnswersFirst`。
- 删除的护栏：`TestUpgradeGuardrails` 的删除（REST 到不了，只经 MCP）；标的节点：`TestTheFreezeGuard`（只在 page 之内看得到，决定 237）；护栏的读走主键：成本守卫的计划的断言（7.1，决定 236）。
- 预检：`TestUpgradeFreezeReachesEveryPath` 里只有预检答的情形，各是预检之后、单元之前另一个码会先答的：上传在存储将满时（`storage_full` 在 page 的检查之后，`asset/app/upload.go:67-76`）、恢复一个过期的回收站项（`page.restore_expired`）、撤销一个超过 `MaxJobItems` 的变更集（`page.revert_too_large`）、导入在存储将满时；预检判冻结时它们答 `notebook.upgrading`，不判时答那个码。建任务时的判断：正在上传的导入（7.2 第 4 条）。
- 后台任务的每一批（纵深防御的一层，P1 审查 A-N6）：`TestUpgradeFreezeReachesEveryPath` 的"任务的批"子测试（7.1）；S1 只在 `upgrading` 里测，S3 加 `rolling_back` 一行：别的任务的一批在回滚中被拒、回滚自己的批放过（S1 审查 S1C-N1）。
- 发起的每个码：`TestUpgradeStart` 依次；保护行：同一个测试断言补写的 `raw` 恰有一行 `baseline = false`、已有的 `raw` 没有（P1 审查 B-N1）。
- 回滚的每遍核对：`TestUpgradeRollback` 的"两遍之间以 SQL 结束升级"（7.1）；`UpgradeOwn` 的过期：7.2 第 16 条；写入键与 `Writing` 认回滚：`TestOneWritingJobAtOnceInANotebook` 的回滚的行与 `TestUpgradeRollback` 的"回滚在跑时继续、结束回滚答 `transfer.busy`"。
- 取消的规则：矩阵的两个变体。

### 3.25 S1 的实施对设计的修正

S1 的审查与修复核对见 [S1 的审查记录](reviews/P1-S1-freeze-guardrails-review.md)，结果在 10.1。与上文最初的设计不同之处已改在各节里；各节因 S1 新写的代码位置是 `68c3c505` 的（头的表说的 `9184f67b` 管其余的）：

- **S1，实施时定下的**（实施者的 24 条偏离，括号里是它的号；S1 审查 A、B、C 各在自己的范围里判过，都对，第 10、20、23、24 条另成了审查的发现）：
  - 3.2：`checkedRows` 原来只有一本 LLM Wiki、没有普通的笔记本，加一本升级中的普通笔记本与它的三行 `upgrade_nodes`（基线页、基线附件、保护行），另给变更集设上 `upgrade_id`（1）；初稿的反例"id 有、另三列空"同时违反三个合取，换成"缺状态"，"基线没有名称"一行的名称与次序都空，每个反例只违反一个合取，共 27 个（3）；名称表的主键是 `upgrade_nodes_pkey iu` 与 `upgrade_nodes_pkey p` 两项（4），部分索引照表里同类的项写作 `iw`；`TestTheUpgradeColumnsGoDown` 依次 `downTo(56)`、`downTo(55)`（`downTo` 退掉给定的那一版，2）。
  - 3.3：notebook 根的 `Upgrade`、`UpgradeState` 与三个值的别名在 `notebook/notebooks.go`，挨着 `State`、`Mode`；`ActionChangeMode` 在 `module.go`（5）。
  - 3.4：`decideAction` 也给撤销的单元用，它不再只为判定建一个 `ModeGuard`（6）；三个 page 的预检共用 `Writer.notUpgrading`，后台撤销的开始路径读两次状态、上传的 `Check` 恒读一次，M10 的两个钉住读的次数的测试随之改（8）；测试另有 `TestThePrechecksRefuseAnUpgradingNotebook`（13）与适配器的 `TestCreateChangesetWritesItsUpgrade`（9）；模块根的构造在第 4 个提交里先不用连接池、第 5 个接上，签名一直是设计的（7，各节不改）。
  - 3.5：导入的 `Check` 的调用序列多一个 `Notebooks.StateOf`，`TestAnImportStarts` 随之改（8）；组合根的 `TestTransferNotebooksConvertTheState`，`TestPageNotebooksConvertTheState` 加升级的一行（11）；transfer 的 `adapter/http` 的替身 `notebooks` 带一个原子的 `upgrading`（14）。
  - 3.7：`events.yaml` 的 `mode` 的说明保留其后的"Its data is the notebook's ids. Read the notebook again…"一句，加"its upgrade"（22）。
  - 6.1：第 6 个提交的范围是 `server, api`（12）；第 2 个提交另改 M10 的 lint 迁移的成本夹具（24，见下面的决定 238）；oxfmt 报出的四处格式以 fixup 并回第 3、4、6 个提交（15，各节不改）。
  - 7.1：`TestUpgradeFreezeReachesEveryPath` 的三种状态在一个应用里、三个子测试各起自己的应用（16）；`TestUpgradeFreezeLeavesTheRest` 另核对读页与字节不变的写（17）；`TestUpgradeFreezeAnswersFirst` 另核对接管的开启（18）；`TestUpgradeFreezeLetsOnlyItsOwnWrites` 跑 `upgrading` 与 `rolling_back`，另含后台撤销的开始与发起人撤销 agent 的变更集（19）；夹具的 `upgradeIn` 与 `endUpgrade`（21）；成本守卫写在适配器一层（20，界见下面的决定 236）。这几条是测试的组织，7.1 的表不改，10.1 记下。
  - 7.3：第 14 条的第二个测试看不到被标的节点（10，决定 237）；第 68 条改在 `unit.go`（23）；第 18 条在 S1 由 `TestTheStartsRefuseAnUpgradingNotebook` 的建任务抓到，7.2 第 4 条的交错在 S2。
  - 7.5：第七份在 S1 有七个测试（`TestUpgradeToolsErrorsCarryTheirCodes` 也以 `TestUpgrade` 起名），不是六个。
- **S1，审查与修复一轮带来的**（编号是 S1 的审查记录的；主持者的决定见审查记录的第 236–239 条）：
  - 删除护栏的读的成本守卫以计划为主、时长为辅：四种统计状态 × 两种计划都走主键、两列都进索引条件、主键的列钉住，界 25 ms（S1B-I1 = S1C-M2，决定 236；7.1、3.24，7.3 第 87–89 条；00 号文档第 9 节的同一行）。
  - 被标的节点只在 page 之内看得到（S1A-Q1 = S1C-M1，决定 237；3.4，7.3 第 14 条；00 号文档 4.6）。
  - M10 的 lint 迁移的成本夹具从嵌入的最新迁移退（S1B-M1、S1C-N5，决定 238）：6.2、6.3 不用改它，6.1 第 2 个提交写明。
  - transfer 的开始的次序与两处新分支有测试（S1A-M1、S1A-N1；3.5，7.3 第 90–92 条）。
  - 冻结的守卫对不认识的判定答缺陷的错误（S1A-N2，修复一轮唯一的生产的改动；3.4 的代码与测试，7.3 第 93 条）；`Grants` 的注释照实（S1A-N3，3.4）；page 的模块根导出三个状态常量（S1C-N3；3.4，7.3 第 94 条）。
  - 分份的数用竞态构建的口径（S1C-N2，3.8）。
  - C 的 o1、o4、o7、o9 补成 7.3 第 83–86 条（S1C-N7）。
  - 交给后面的 Step：S2 的 `States.Start` 断言升级的 id 非零（S1B-N1，3.14 第 10 步）；S3 的"任务的批"加 `rolling_back` 一行（S1C-N1；7.1、3.24、6.3 第 4 个提交）。没有交给 v0.1 之后的打磨的。

### 3.26 S2 的实施对设计的修正

S2 的审查与两轮修复核对见 [S2 的审查记录](reviews/P1-S2-start-and-read-review.md)，结果在 10.2。与上文最初的设计不同之处已改在各节里；各节因 S2 新写的代码位置是 `42529055` 的（头的表说的 `9184f67b` 管其余的，S1 新写的是 `68c3c505` 的）：

- **S2，实施时定下的**（实施者的 20 条偏离，括号里是它的号；三位审查者各在自己的范围里判过，都对，第 7 条连带出审查的 S2A-M1）：
  - 3.13：`LintAreaWantedEdgeHeads` 的待建过滤写在 `LIMIT 1` 的 `LATERAL` 里，不写在 `WHERE` 里：没有统计时 PostgreSQL 18 把 `IS NULL` 估成 0.005，单页数组的自定义计划选了位图扫描加 Sort；`LATERAL` 是 `lint.sql` 里已有的写法，`TestLintPlansOfTheAreaPass` 钉住（1）。`RuleQuery.WantedByPage` 只与 `wanted` 一条规则同用，别的组合 `lintQuery` 答 `errLintQuery`；第二遍在范围的模式是 LLM Wiki 时跑，不看契约是不是显式的，今天只有发起传它（2）。`TestComputeInTakesItsMoment` 钉住 `computeIn` 用调用方的时刻：S2 的发起只调 `prepareIn`（8）。
  - 3.14：发起的期限 `StartTimeout = RequestTimeout × 3/4`，不是 `LintTimeout`（16，第 232 条：10 万页放不进守卫配置下 `LintTimeout` 的 3 秒）；生产上发起的期限因此从 7.5 秒变成 11.25 秒，默认 4 秒的请求下从 2 秒变成 3 秒。`transfer.busy` 的文字照降级的，末句改为"…start the upgrade once it is done."（4）。`GetUpgrade` 直接从行读 S2 还用不到的回滚的几列，都是 null（15）。
  - 3.15：`wiki.Deps` 加 `Pool`，wiki 自己的表的适配器要它（3）。`checkPages` 的升级规则只在库里有 `notebook_upgrades` 时加（`to_regclass`）：M10 的 `TestIngestMigrationCostsLongHistory` 在退到 53 的库上调 `checkPages`，只在不带竞态的 `costs-3` 里跑，实施者第二次 `make test` 才撞上，同 S1 偏离 24；测试名为 `TestPageViolationsCatchEachUpgradeGap`，在第四份（6）。S1 的测试照新规则改 wiki 的行或加 raw（11）；运行时角色的测试先以 owner 的 SQL 加一个笔记本管理员（14）。
  - 3.22：矩阵的三行没有另种变体工作区：用 lab 种好的笔记本，每格在自己的库的副本上以 SQL 准备，覆盖的两个检查经 `coveredMatrixRows()` 读它们（5）。
  - 7.1、7.2：名额占满的 503 只在 7.2 第 17 条与 `wiki/app` 的 `a-full-slot-is-503-at-once` 测（7）；`wiki/app` 的 `TestUpgradeStart` 在 `app_test` 包里（9）；夹具 `upgradeIn` 选根、写一致的 wiki 行，`endUpgrade` 记 `rolled_back`（10）；7.2 第 3 条的心跳答 `page.edit_session_not_found`（12）；`TestUpgradeStatesReachTheStreams` 里补写的页先送 `pages`、再是 `mode`（13）；成本守卫是 5 万页、1.5 万页在 raw 里，名字不带规模（16），在发起的事务里核对 `SetBaselineWanted` 的自定义与通用计划，"本次之后"读作分析过这本笔记本的上一次发起，每次发起之后 `VACUUM`（17）。
  - 第 5 节：e2e 的夹具 `upgrades.ts` 与 `assert/wiki.ts` 的 `upgradesOf`；UP1 的"别人"在 REST 版本是一位编辑者、在 MCP 版本是另一位笔记本管理员；UP2 末尾在编辑者离开、导入做完之后再发起一次答 200；`threeBrowsers` 在 S4 改（18）。
  - 7.3：第 31 条的测试改为 `TestComputeInTakesItsMoment`（8）；第 62 条四种写法，成本守卫只抓得到三者合在一起的那种（19）；第 18 条的 S2 一半由 7.2 第 4 条抓到（20）。
- **S2，审查与修复两轮带来的**（编号是 S2 的审查记录的；主持者的决定见审查记录的第 240–242 条）：
  - 发起的期限落在一条语句之中时答 503 而不是 500，由组合根的 `inside a statement` 钉住（S2C-I1；7.1，7.3 第 26 条）。
  - 领域的拒绝在回滚越过期限时照答自己的码（S2C-M1，7.3 第 95 条）；`COMMIT` 自己超时而 `ctxStart` 还活着时不答 503（S2A-N1，第 96 条）。
  - 名额在计算与提交时持着、`ANALYZE` 之前已放开，名额占满时第 4–8 步照答自己的 409（S2A-M1 = S2C-N3；3.14 第 9 步，7.1，7.3 第 97、98 条）；7.2 第 17 条在另一个 goroutine 里发起、限时 2 秒（S2C-N2；7.2，7.3 第 24 条）。
  - 第二遍对目标的判断与它的两道防御检查有测试（S2A-M2、S2A-N2；3.13，7.3 第 99、100 条）；`CurrentUpgrade` 的 `rolling_back`（S2A-N3，第 101 条）；`TestComputeInTakesItsMoment` 的后一半（S2C-N1，第 102 条）。
  - 发起只读这本笔记本的行（S2B-M2，决定 240）：`CaptureBaseline` 改为 `LEFT JOIN LATERAL (… LIMIT 1)`，修复一轮唯一的生产的改动（3.12）；成本守卫的库另种九本笔记本，核对 page 的四条读的两种计划，界 3.6 秒（7.1，7.3 第 103–105 条；00 号文档第 9 节的同一行）；第二轮把"逐个经主键读"落到核对上（S2A-N4，7.3 第 110 条，门禁照决定 242）。
  - 00057 的反例补五条、部分唯一索引补两条（S2B-M1、S2B-N1；3.10，7.3 第 107、108 条）；被蕴含的三个合取保留并在迁移里注明（S2B-N2，决定 241；3.10）。
  - `checkPages` 的升级规则补三行、每行只报自己的规则（S2B-N3；3.15，7.3 第 109 条）。
  - 第二遍的计划的测试另钉住 `source_id` 的键与 `LATERAL` 的写法（审查 B 的 c3、C 的 62e；7.3 第 106 条）。
  - 7.1 的"种子用 SQL，准备不超过 4 秒"照实写成被测的那份（修复核对 A 提的口径）。
  - 只记下、不改历史的：第 6 个提交 `1c4d35c0` 的标题没写发起在生产上的期限从 7.5 秒变成 11.25 秒，范围只写 `server`；修复一轮的 `0da54746` 同样改了 `ci.yml` 的一行注释、范围只写 `server`（S2C-N5，10.2）。
  - 交给后面的：10 万页的发起进 M12 的性能移交（00 号文档第 232 条与"写给后面的 M"，第 9 节）。没有交给 v0.1 之后的打磨的。

## 4. 网页

网页的门禁（tsc、knip、vitest、`make gen-check-web`）在每个 Step 的合并时都跑，所以服务端的 Step 自带它改的契约在网页上要的东西；service、store 的方法与组件只在 S4 加（knip 报出没人用的导出）。

### 4.1 S1 随服务端的部分

- `make gen-web`（`Makefile:123`）重新生成 `web/packages/api-client/src/schema.gen.ts`（`components['schemas']['Notebook']` 加 `upgrade`，另有 `NotebookUpgrade`、`NotebookUpgradeState`；`:3678` 是别名 `export type Notebook = components['schemas']['Notebook']`，不改）。
- `Notebook` 必填 `upgrade` 之后 tsc 报出的两处字面量：`W/test/fakes.ts:91-102` 的 `notebookJSON` 与 `W/stores/notebook.store.test.ts:7-19`，各加 `upgrade: null`。
- `notebook.upgrading` 的文字：`W/app/problem-messages.ts:11-74` 的表加 `"notebook.upgrading": "problem.notebook.upgrading"`；`W/i18n/messages/en.ts`（`:1093` 旁）"This notebook is being upgraded to an LLM Wiki: it is read-only until the upgrade ends."，`zh-CN.ts`（`:1047` 旁）"这本笔记本正在升级为 LLM Wiki，升级结束之前只读。"。`problem-messages.test.ts`（`:43-62`）读契约，核对每个操作声明的码都有文字。`page.upgrade_protected` 只在 MCP，它的文字是服务端的 `detail`（3.4），网页不加。
- `events.yaml` 的说明改了，生成的类型的注释随之变，`make gen-check-web` 照过。

### 4.2 S2 随服务端的部分

- 生成的类型加 `Upgrade` 等与两个操作；没有 service 用它们（S4 才加 `upgrade.service.ts`），生成的类型经 `export type *` 再导出，knip 看不到（同 M10/P1 4.1）。
- 五个码的两种语言的文字：`notebook.already_llm_wiki`（"这本笔记本已经是 LLM Wiki。"）、`wiki.upgrade_not_found`（"这本笔记本没有进行中的升级。"）、`wiki.upgrade_pages_locked`（"有页面正在被编辑，不能开始升级：等他们退出编辑，或由管理员强制解锁。"）、`wiki.upgrade_title_taken`（"根下名为 raw 或 AGENTS 的是附件：先改名，再开始升级。"）、`wiki.upgrade_stale_index`（"链接的索引过时了：请服务器管理员运行 nervewiki reindex，再开始升级。"），英文照同样的意思。

### 4.3 S3 随服务端的部分

- `wiki.upgrade_state` 的文字（"升级现在的状态不允许这样做。"）。
- 新种类：变更集的 `kindText`（`W/pages/notebook/changeset-text.ts:14-27`，`switch` 由 tsc 强制）加 `rollback`（"升级的回滚"/"Upgrade rollback"，键 `history.kind.rollback`）；`W/pages/notebook/transfer-names.ts` 的 `changesetKinds`（`:9-15`，`Record<Changeset["kind"], true>`，tsc 强制）加 `rollback`，`jobTitle`（`:22-34`）对 `kind === "rollback"` 写"撤销升级"/"Undo the upgrade"（任务的名称是固定的 `upgrade`）；`jobTitle` 的 `switch` 之后有一句兜底、落到导出的标题（`:33`），漏了这一支 tsc 不报，所以 `transfer-names.test.ts` 加"回滚任务的标题是'撤销升级'"一条（P1 审查 C-N3）。两份 `writingKinds`（`W/stores/transfer.store.ts:27`、`W/test/jobs-server.ts:10`）加 `"rollback"`（05-lessons 第 2 节：替身在真东西会失败的地方失败）：store 的那份决定按钮（`transfer.store.ts:101`），vitest 是"回滚在跑时导入的按钮不可用"；假服务端的那份决定它答不答 `transfer.busy`（`jobs-server.ts:204`），vitest 是"假服务端有一个页面还没读到的回滚任务，用户开始导入，页面显示 `transfer.writingBusy`"（P1 审查 C-N4）。
- 回滚的报告（`W/pages/notebook/transfer-report.tsx`）：`shownCounts`（`:122-127`）加 `rollback: ["deleted", "put_back", "written"]`，不然 `:202` 退回导入的计数；`kindFailures`（`:54`）加 `rollback` 的失败文字（`forbidden`"开始回滚的人已不是管理员，另一位管理员可以继续"、`revert_too_large`"这次升级改动太多，回滚停下了：可以结束回滚、保留现在的样子"、`internal`、`interrupted`、`timeout`"可以继续回滚"）。vitest：回滚的报告显示三个计数与每种失败的文字。

### 4.4 S4：网页

00 号文档 4.15 的 P1 部分。

- **只读收成一处**：`W/app/effective-role.ts` 加 `writesNotebook(notebook) = writesPages(notebook.role) && notebook.upgrade === null`（`writesPages` 在 `:40-42`）；今天直接以 `writesPages(notebook.role)` 判的 14 个文件的调用（`app/source-permissions.ts:10`、`:22`、`:42`；`pages/notebook/transfer-page.tsx:101`、`changeset-page.tsx:118`、`page-tree-item.tsx:85`、`:199`、`trash-row.tsx:485`、`page-tree.tsx:65`、`notebook-home.tsx:47`、`trash-page.tsx:178`、`trash-preview.tsx:197`、`:319`；`pages/page/page-layout.tsx:154`、`upload-rows.tsx:50`、`attachments-section.tsx:83`、`:101`、`attachment-drop.tsx:36`、`unresolved-link.tsx:125`）都换成它；`writesPages` 照旧导出（`W/app/effective-role.test.ts:5`、`:28`、`:30` 导入并测它，测试用到的导出 knip 算有人用；P1 审查 C-N2），之后生产代码只由 `writesNotebook` 用；`effective-role.test.ts` 加 `writesNotebook` 的四格（管理员与写者 × 有无升级）。`unresolved-link.tsx` 的 `Why`（`:33`）加 `upgrading` 与它的文字。**控件表**（新的 `W/pages/notebook/upgrade-read-only.test.tsx`，放在 `pages/notebook/`；它经各自的组件渲染 `app/`、`pages/notebook/`、`pages/page/` 三处的控件，P1 审查 C-N9；每项一条，照 M10 4.16 来源页那一格）：笔记本首页的新建；页面树的新建、改名、移动、拖放、排序、删除；页面的编辑与 `Ctrl+E`；阅读视图的任务勾选；附件一节的上传、拖入、改名、删除；附件一节与编辑器上传行的重试（`pages/page/upload-rows.tsx:48-51` 的 `canRetry`）；未建链接的新建；版本页的还原；历史的撤销；回收站的恢复（行与预览）；导入。每项在 `upgrade` 不为空、角色是管理员时不出现或不可用。
- **编辑器失去编辑**：`W/stores/edit-session.ts` 的 `EditLost`（`:26-32`）加 `{ reason: "upgrading" }`；`lostBy`（`:63-83`）认 409 `notebook.upgrading`（保存与开启）；照 `source` 先 `end()`（`:180-181`）；`W/pages/page/edit-lost-banner.tsx` 的 `lostText`（`:46-47` 的 `switch`）加文字（tsc 强制）。vitest：保存答它时停止自动保存、显示文字、不再重试。
- **横幅**：`W/pages/notebook/upgrade-banner.tsx`（新），在 `notebook-layout.tsx` 的 `<Outlet>`（`:76`）之前，`role="status"`，不抢焦点。按状态三种文字（00 号文档 4.15）；管理员另有"查看升级"的链接与"任何笔记本管理员都可以放弃这次升级"；在升级页上不带链接。vitest：三种状态 × 管理员与非管理员 × 在不在升级页。
- **发起**：`W/pages/notebook/general-page.tsx` 的"模式"一节（`ModeSection`，`:129`；降级的对话框 `:156-166`）对普通笔记本、管理员、没有升级时加"升级为 LLM Wiki…"按钮，`ConfirmDialog` 的 `held` 与 `tone="default"`，对话框写后果；根下已有标题为 `raw` 的页而它下面有节点时另写一段，N 从页面树已读到的 `listNodes` 数出（主持者第 214 条）；模板的语言取界面语言。拒绝的文字：`wiki.upgrade_pages_locked` 列出页与编辑者（`W/app/pages-locked.tsx:19` 今天只认 `linking.pages_locked`，放宽到新码，并提示可以强制解锁）；`transfer.busy` 用 `transfer.writingBusy`；其余照 `problemMessages`。成功之后去升级页、焦点给标题。有升级时这一节写状态与"查看升级"。vitest（`general-page.test.tsx`；P1 审查 C-M2，00 号文档第 9 节 `:693`）：按钮只对普通笔记本、管理员、没有升级时出现；根下 `raw` 有节点时对话框有那一段与 N，没有时没有；`wiki.upgrade_pages_locked` 列出页与编辑者（`pages-locked.tsx` 认新码，另有它自己的一条）；`transfer.busy` 显示 `transfer.writingBusy`；成功之后去升级页、焦点在标题上；对话框的 `held` 与 `onClosed` 的焦点（关上之后焦点回到这一节的标题）。
- **service 与 store**：新的 `W/services/upgrade.service.ts`（`startUpgrade`、`getUpgrade`、`abandonUpgrade`、`endUpgradeRollback`；检查、提交、审阅的在 P2/S3、P3/S3 加）；`NotebookStore`（`W/stores/notebook.store.ts`）加 `startUpgrade`、`abandonUpgrade`、`endUpgradeRollback`，经 `inTurn`（`:45`），成功之后重读笔记本列表与 `["upgrade", notebook_id]`，403、404、状态不对时照 `downgrade`（`:114-143`）重读列表。vitest 照降级的七条（成功、403、404、409、并发的两次、卸载、重读）。
- **升级页**：`W/app/routes.tsx` 在 `health`（`:195-200`）旁加 `upgrade`（`/:slug/notebooks/:id/upgrade`），新的 `W/pages/notebook/upgrade-page.tsx`，`useDocumentTitle`；只给管理员的读（13.2 第 18 条）：非管理员不发读、只说明谁能看；403 时重读 `["notebooks", ws]`；404 `wiki.upgrade_not_found`"没有进行中的升级"。各节（P1 的）：状态（谁、何时发起，提议的来源根与补写的页的链接）；接入 agent（发起人：prompt 名 `upgrade` 与"照 README 的'接入 agent（MCP）'一节把 agent 连到这本笔记本"；其他管理员："只有发起人（显示名）的 agent 能写；你可以放弃"）；"放弃这次升级"（`destructive`，`upgrading`、`reviewing` 与停下的 `rolling_back`）；回滚：按 `rollback_job_id` 读任务，排队或运行时照任务页轮询（`W/pages/notebook/transfer-page.tsx:46` 的 `pollInterval`、`:178` 的 `refreshInterval`），转为结束时再读一次升级、取回 `rollback_left`；进度与报告复用 `transfer-report.tsx`；停下时"继续回滚"与"结束回滚，保留现在的样子"（`destructive`，对话框写三个数与链接可能指不到的一句；三个数都是 0 时写"已经复原完，结束即可"）。对话框都用 `held` 与 `onClosed` 的焦点规则（`general-page.tsx:138-140`）。vitest（`upgrade-page.test.tsx`；P1 审查 C-M2，00 号文档第 9 节 `:693`）：非管理员不发读（假服务端没收到请求）、只说明谁能看；403 重读笔记本列表；404 显示"没有进行中的升级"；接入一节对发起人写 prompt 名与 README 的那一句、对其他管理员写"只有发起人（显示名）的 agent 能写"、不写 prompt 名；回滚的任务在跑时轮询，假服务端把任务转为停下之后，不刷新页面就重读升级、出现"继续回滚"与"结束回滚"两个按钮与三个数，之后不再轮询；三个数都为 0 时对话框写"已经复原完，结束即可"；放弃与结束回滚的对话框关上之后焦点的去处。
- **事件与重连**：`W/events/handlers.ts` 的 `modeChanged`（`:277-279`）另外重读 `["upgrade", notebook_id]`；`W/app/event-stream.tsx` 的 `refreshedOnConnect`（`:15-35`）的第三层加 `upgrade`（与 `pages` 同层）。vitest：`mode` 事件与重连各重读升级的键。
- **任务页的"取消"**：对回滚的任务只给笔记本管理员（取消按钮在 `W/pages/notebook/transfer-job-row.tsx:168`，它的 props 有 `notebook`，`:38`；按 `notebook.role`；P1 审查 C-N8）。vitest：写者看不到它。
- **WebKit**：横幅出现时内容往下移一个横幅的高，在 WebKit 里人工看一次并记进记录（第 224 条）。

## 5. e2e

**夹具**：新的 `e2e/fixtures/upgrades.ts`（经 REST 的 `startUpgrade`、`getUpgrade`、`abandonUpgrade`、`endUpgradeRollback`，S2、S3 各加自己的）；新的 `e2e/fixtures/assert/wiki.ts`（`expectUpgrade`：wiki 的行的状态、结局、任务、三个数）；`assert/notebook.ts` 的 `expectNotebook`（`:46`）加升级的四列；`assert/page.ts` 加基线行的数与变更集的 `upgrade_id`（照 `expectChangesetOf`，`:222`）；`assert/transfer.ts` 的 `expectBackgroundJob`（`:179`）认回滚。MCP 照 `e2e/fixtures/mcp.ts` 的 `mcpClient`（`:152`）、`errorOf`（`:227`），每个 agent 一个 PAT（`createToken`，`fixtures/auth.ts:67`）。

| 故事（`e2e/stories/upgrade/`） | 版本与 Step | 内容 |
|---|---|---|
| `up1-start-freeze.spec.ts` UP1 发起与冻结 | API、MCP：S2；页面：S4（三种浏览器） | 一本有几页、一个附件的普通笔记本（经 REST 种下）；发起；补写的 `raw`、`AGENTS` 与基线的行（数据库）；别人的 REST 写答 409、发起人的 REST 写答 409；MCP 版本：发起人的 agent 的 `write` 放过、另一位管理员的 agent 的 `write` 答 `notebook.upgrading`。页面：设置里发起、去升级页；横幅；另一个标签页随 `mode` 看到横幅；发起人的网页只读（编辑按钮不在）。页面版本不引起 409 |
| `up2-start-refused.spec.ts` UP2 发起被拒 | API：S2；页面：S4 | 编辑会话（`locks` 列出页与编辑者）、写入任务（`transfer.busy`）、已是 LLM Wiki、已在升级；页面版本走编辑会话与写入任务两种（对话框里各自的文字），根下已有 `raw` 时对话框写它将成为来源区与节点数；两种 409 是刻意的，以 `pageWatch.expectConsole({ errors: [failedToLoad(409)] })` 声明（13.2 第 9 条）。"已是 LLM Wiki"与"已在升级"只在 API 版本：P1 里普通笔记本变不成 LLM Wiki（批准在 P3），按钮又只给普通的、没有升级的笔记本（P1 审查 C-N5）。MCP 豁免：MCP 不能发起；冻结之外的 MCP 写由 `TestUpgradeFreezeReachesEveryPath` 盖住 |
| `up3-abandon-rollback.spec.ts` UP3 放弃、回滚与结束 | API（MCP 写作准备）：S3；页面：S4 | agent 经 MCP 移动、改名、改 frontmatter、新建之后，经 REST 放弃；任务做完；基线节点的位置、名称与正文复原、新建的在回收站、一个 `rollback` 的变更集带 `upgrade_id`；另一段照 LW12 的做法（`holdNotebookRow(db, other.id)` 持住另一本笔记本、开它的导入占住唯一的导入 worker，`e2e/stories/wiki/lw12-downgrade.spec.ts:60-72`，`import_workers: 1`）让回滚排队、取消，之后"继续"做完；第三本同样取消之后结束回滚，冻结解除、`rollback_complete = false` 与三个数。页面：放弃的对话框、进度、停下之后两个按钮、结束的对话框写三个数。MCP 豁免：MCP 不能放弃或结束回滚；回滚中 MCP 写被拒由 `TestUpgradeFreezeReachesEveryPath` 的 `rolling_back` 一格盖住 |
| `up4-guardrails.spec.ts` UP4 护栏 | MCP：S2 的删除一步（P2/S1 补全） | 发起人的 agent 删一个原有页答 `page.upgrade_protected`、删一个新建的页成功、删补写的 `raw` 答 `page.upgrade_protected`；数据库里原有页在 |

`e2e/playwright.config.ts:3` 的 `threeBrowsers` 在 S4 加 `|upgrade\/up1-`（2.1 第 9 条）。对话框与按钮按确切的名字找（05-lessons 第 6 节）。

## 6. 实施步骤

每个 Step 一个分支，从前一个合并之后的 `main` 开始；分支上的每个提交都绿。提交的写法照 00 号文档第 7 节"提交说明"（`:520-527`）：代码 `<范围>: <说明> (M11/P1/S<m>)`，范围列出碰到的每个部件（改契约的提交重新生成 `schema.gen.ts`，所以带 `web`）；分支上的文档 `docs: … (M11/P1/S<m>)`；合并之前的修复写到 Step，核对写 `fix check <k>`。接受本文的文档提交同时照 2.1 改 00 号文档（第 8 节）。

**每个提交单独是绿的**：golangci-lint 的 `unused`（`server/.golangci.yml:5`，测试文件也算）要每个不导出的函数在它落下的提交里有调用者；契约声明一个码的提交要同时在模块的 `adapter/http` 测试里答出它（`apitest` 的 `unanswered`）、在网页加它的文字；`transfer.New` 在没有 `RollbackEnd` 时 panic 是 S3 新加的检查（3.19），所以 `RollbackEnd`、这个检查与它的三处调用的改动在同一个提交。

### 6.1 S1（`m11-p1-s1`）

1. `server, docs: the composition root's seventh shard (M11/P1/S1)`：3.8（还没有 `TestUpgrade…`，`-run '^TestUpgrade'` 什么也不跑也是绿的）。
2. `server: an upgrade's columns and baseline (M11/P1/S1)`：00055、00056、sqlc、`schema_test` 的名称与反例、`TestTheUpgradeColumnsGoDown`、两行授权与 `analyzed`（3.2）；M10 的 lint 迁移的成本夹具 `lintMigrationDropIngest`（`bootstrap/lint_index_migration_cost_test.go`）把库退过新的迁移（S1 偏离 24；修复一轮改为从嵌入的最新迁移退，以后的迁移不用碰它，决定 238）。
3. `server, api, web: a notebook's reads carry its upgrade (M11/P1/S1)`：3.3；网页的两处字面量（4.1）。
4. `server, api, web: the freeze guards the page writes and vetoes the openings (M11/P1/S1)`：3.4 除删除的护栏与预检；九个经单元的操作与 `openEditSession` 的声明（3.6 的表）、page 的 `adapter/http` 测试答出；`mcpToolCodes()` 的五行加 `notebook.upgrading`、`upgradeToolCodes()` 的五对、`TestUpgradeToolsErrorsCarryTheirCodes` 的这一半；`notebook.upgrading` 的文字（4.1）。
5. `server: the initiator's agent deletes no node the upgrade started with (M11/P1/S1)`：删除的护栏、`UpgradeProtected`、`page.upgrade_protected`、`mcpToolCodes()` 与 `upgradeToolCodes()` 的两对、`mcpOwnCodes()`、`TestUpgradeGuardrails` 的删除。
6. `server, api: the prechecks and the transfer starts refuse an upgrading notebook (M11/P1/S1)`：3.4 的预检、3.5；四个操作的声明与 asset、transfer 的 `adapter/http` 测试。不改网页：`x-problem-codes` 不进 `schema.gen.ts`，`notebook.upgrading` 的文字在第 4 个提交已加（S1 偏离 12）。
7. `server, api, web: the freeze reaches every path and leaves the rest (M11/P1/S1)`：`TestUpgradeFreezeReachesEveryPath`、`TestUpgradeFreezeLeavesTheRest`、`TestUpgradeFreezeAnswersFirst`、`TestUpgradeFreezeFollowsTheInitiatorsRole`、`TestUpgradeFreezeLetsOnlyItsOwnWrites`（含"任务的批"的子测试，P1 审查 A-N6）；`events.yaml` 的两处说明（3.7）；删除护栏的成本守卫。

### 6.2 S2（`m11-p1-s2`）

1. `server: the wiki module's first table, its purge and its deletion (M11/P1/S2)`：00057、wiki 的 sqlc 一项与 `wiki/adapter/postgres`、授权、清理器、两个模块的删除订阅者（3.10、3.15 的前四条）。
2. `server: an upgrade's notebook states (M11/P1/S2)`：3.11。
3. `server: an upgrade's baseline, sessions and root keys (M11/P1/S2)`：3.12。
4. `server: lint computes in its caller's transaction, holds a slot now or not at all, and counts wanted by page (M11/P1/S2)`：3.13。
5. `server, api, web: startUpgrade and getUpgrade (M11/P1/S2)`：3.14、3.15 的其余（接线、运行时角色、`checkPages` 与 S1 夹具的 wiki 行）、矩阵的 S2 行、`TestUpgradeStart`（含保护行）、`TestUpgradeStates`、`TestUpgradeStatesReachTheStreams`、`TestUpgradeDeletion`、交错（7.2 第 1–4、13、14、17 条）、日志；五个码的文字（4.2）。
6. `server: an upgrade's start's cost guard (M11/P1/S2)`：7.1 的发起的守卫。
7. `e2e: UP1 and UP2 over the API, UP1 and UP4's delete over MCP (M11/P1/S2)`。

### 6.3 S3（`m11-p1-s3`）

1. `server, api, web: the rollback's kinds of changeset and job (M11/P1/S3)`：00058、00059、sqlc、`schema_test`、`Writing` 与它的适配器测试的回滚的行（`TestOneWritingJobAtOnceInANotebook`，`transfer/adapter/postgres/store_test.go:304`；P1 审查 A-M2）、两个模块的种类、River、Rescue 的表与 `TestTheRescue`、`ChangesetKind` 与 `TransferKind`；新种类的文字与两份 `writingKinds`、回滚的报告（4.3）。
2. `server: the page module rolls an upgrade back pass by pass (M11/P1/S3)`：3.18，含性质测试与回滚的计划、先删、剩下的差三个成本守卫。
3. `server, api, web: a rollback's job, its end and who cancels it (M11/P1/S3)`：3.19、3.20 的 `RollbackEnd`、`SetState`、`Clear`；`cancelTransferJob` 与 `TransferFailure` 的契约、`transfer` 的 `adapter/http` 测试答出 `forbidden`。
4. `server, api, web: abandonUpgrade and endUpgradeRollback (M11/P1/S3)`：3.20 的其余、`wiki.upgrade_state` 的文字、矩阵的 S3 行、`TestUpgradeRollback`、`TestUpgradeFreezeReachesEveryPath` 的"任务的批"的 `rolling_back` 一行（S1 审查 S1C-N1）、交错（7.2 第 10–12、16 条）、日志。
5. `e2e: UP3 over the API (M11/P1/S3)`。

### 6.4 S4（`m11-p1-s4`）

1. `web: one judgment of whether a notebook takes writes (M11/P1/S4)`：`writesNotebook`、14 个文件、`Why`、控件表。
2. `web: the editor loses its session to an upgrade (M11/P1/S4)`。
3. `web: the upgrade's banner (M11/P1/S4)`。
4. `web: start an upgrade from the notebook's settings (M11/P1/S4)`：`upgrade.service.ts` 的 `startUpgrade`、`NotebookStore.startUpgrade`、对话框、`pages-locked.tsx`；升级页的路由与状态一节（成功之后要去的地方）。
5. `web: the upgrade page's agent, abandonment and rollback (M11/P1/S4)`：其余的 service 与 store 方法、接入、放弃、回滚、事件与重连、任务页的取消。
6. `e2e: UP1, UP2 and UP3 in the browser (M11/P1/S4)`：页面版本、`threeBrowsers`。

### 6.5 合并的门禁

02-workflow 3.1：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`（经 `docs/v0.1/claude_to_codex/tools/heavy.sh`）；7.3 的每个变异；7.5 要量的；分支的 CI 全绿。合并之前的修复提交只改成本或守卫测试本身、且该测试只由一行跑时，本机取提交单独为绿、`make lint gen-check knip build`、经 `heavy.sh` 的那一行与 `make test-go-short`，其余由分支 CI 的全部各项担保（决定 242）。

## 7. 测试与验证

### 7.1 各项与它们的测试

| 项 | 测试 | 在哪里 | Step |
|---|---|---|---|
| 迁移的检查、名称、Down | `TestChecksRejectCounterexamples`、`TestConstraintAndIndexNames`、`TestTheUpgradeColumnsGoDown` | `server/migrations` | S1、S2、S3 |
| 六条查询带升级 | 3.3 的三个测试 | notebook | S1 |
| 冻结的规则 | `TestFreezeAdmits` | `page/domain` | S1 |
| 守卫、否决者、删除的护栏、自己的单元、预检 | `TestTheFreezeGuard`、`TestTheFreezeLetsItsOwnUnits`、`TestTheChangesetsCarryTheirUpgrade`、`TestThePrechecksRefuseAnUpgradingNotebook` | `page/app` | S1 |
| transfer 的开任务；组合根交给 page 与 transfer 的状态 | `TestTheStartsRefuseAnUpgradingNotebook`；`TestPageNotebooksConvertTheState`、`TestTransferNotebooksConvertTheState` | `transfer/app`；组合根的第四、三份 | S1、S3 |
| 冻结到每条路径（14 个 REST 写、5 个 MCP 写、开会话、导入、恢复与撤销的开始；只有预检答的情形；发起人的 MCP 写放过；`upgrading`、`reviewing`、`rolling_back` 三列；去掉冻结一项的子测试失败；"任务的批"：导入、恢复、撤销各一个任务停在两批之间（`wiring.batch = transfer.Batch{Nodes: 1}`，`wiring.transfer` 包住 `Tree`、在第二批的调用里停住，照 `bootstrap/source_background_test.go:96`），以 SQL 设上升级的列与基线的行（S2 起另有 wiki 的行），放开之后那一批以 `refused` 停下、`refusal.code` 是 `notebook.upgrading`、什么也没写（`bootstrap/transfer.go:253-257` 的去处）。发起时有写入任务就答 `transfer.busy`，冻结之下到不了这里，这是纵深防御的一层（00 号文档第 3 节第 3 条，P1 审查 A-N6）；S3 给"任务的批"加 `rolling_back` 一行：别的任务的一批在回滚中被拒、回滚自己的批放过（S1 审查 S1C-N1：S1 只在 `upgrading` 里测，按状态区分后台单元的代码在 S3 才有）） | `TestUpgradeFreezeReachesEveryPath` | 第七份 | S1、S3 |
| 冻结之外照常（导出、改名称与开放程度、加成员、删除笔记本、强制解锁，三种状态） | `TestUpgradeFreezeLeavesTheRest` | 第七份 | S1 |
| 冻结排在最前 | `TestUpgradeFreezeAnswersFirst`（以 SQL 造出冻结与锁都拒的写） | 第七份 | S1 |
| 发起人被降为写者 | `TestUpgradeFreezeFollowsTheInitiatorsRole` | 第七份 | S1 |
| 不按 `Kind` 放行 | `TestUpgradeFreezeLetsOnlyItsOwnWrites`（别人在网页上撤销一个旧变更集被拒） | 第七份 | S1 |
| 删除的护栏经 MCP | `TestUpgradeGuardrails` 的删除（基线页、补写的 `raw`、新建的页；`batch` 的删除标在那个操作） | 第七份 | S1 |
| MCP 的码 | `TestUpgradeToolsErrorsCarryTheirCodes`；第五份的 `TestTheMCPToolsErrorsCarryTheirCodes` 减去 `upgradeToolCodes()` | 第七份、第五份 | S1 |
| lint 的拆分、名额、按页的数 | 3.13 的测试 | `wiki/app`、`linking` | S2 |
| 基线、会话、根下的键、`Seed` | 3.12 的测试 | page | S2 |
| `UpgradeStates` | 3.11 的测试 | notebook | S2 |
| 发起的码与期限 | `TestUpgradeStart`：4.4 的码依次；名额占满立即 503；`fn` 里 `ctxStart` 到期答 503、没有 error 级的日志、整个回滚、有超时的 warn（组合根两格：期限落在一条语句之中，测试持住笔记本行、发起在 `States.Lock` 上到期，S2C-I1；落在两条语句之间）；`COMMIT` 之后才到期答 200（`wiki/app` 里假的事务管理器）；领域的 409 照答，回滚越过期限时也是（S2C-M1）；`COMMIT` 自己超时而 `ctxStart` 还活着时不答 503（S2A-N1）；名额在计算与提交时持着、`ANALYZE` 之前已放开，名额占满时第 4–8 步照答自己的 409、stale 一格答 503（S2A-M1 = S2C-N3）；`ANALYZE` 在请求的 ctx 上；在已有的 `raw` 里带一个解析不到的链接的页的基线 `wanted` 是 1 而不是 0（第二遍）；补写的 `raw` 恰有一行保护行（`baseline = false`）、已有的 `raw` 没有（P1 审查 B-N1）；`wiki/app` 的一半以假的 `Rules` 断言只准备 `wanted` 一条规则（B-N3） | 第七份、`wiki/app` | S2 |
| 状态到流 | `TestUpgradeStatesReachTheStreams`（S2：发起；S3：放弃、继续、停下、做完、结束回滚；订阅者交空时失败；`listNotebooks` 带 `upgrade`） | 第七份 | S2、S3 |
| 删除笔记本 | `TestUpgradeDeletion`（wiki 的行软删除、基线行删去；订阅者交空时失败） | 第七份 | S2 |
| 运行时角色 | `TestTheRuntimeRoleServesWithTheGrantsFile` 的发起一段 | 已有的份 | S2 |
| `checkPages` 的升级规则 | `TestPageViolationsCatchEachUpgradeGap`（照 `TestPageViolationsCatchEachLLMWikiGap` 的写法）：每条一个 SQL 造出的违反，只报它自己的规则；"同状态"、基线页数与源根"是页"三个合取另各一行（S2B-N3） | 第四份 | S2 |
| 回滚的计划与先删 | 3.18 的测试（含性质测试） | `page/domain`、`page/app` | S3 |
| 回滚的运行与结束 | 3.19 的测试 | `transfer/app` | S3 |
| 放弃、继续、结束回滚 | `TestUpgradeRollback`：放弃与做完（基线复原、新建的在回收站、`rollback` 的变更集；先删的几遍之后只有一遍合成，`rollback pass` 的日志行与之后 `RollbackLeft` 为 0，2.1 第 12 条、P1 审查 A-M3）、回滚在跑时（`Tree` 的包装停住一批）继续与结束回滚都答 409 `transfer.busy`（A-M2）、先删的一段（一个单元装多棵子树）、停下继续、超时之后继续、结束回滚（`false` 与三个数；差为空时 `true`；只剩一次改名时 `false`、`nodes = 1`）、`RollbackEnd` 换成空时状态留着、测试失败、差不空时任务以 `internal` 结束、开任务之后被降为写者的发起人取消答 403、两遍之间以 SQL 结束升级（`Tree` 的包装在第二次 `PlanRollback` 之前停住；只记"stopped"、任务行照旧 `running`、没有单元跑、新建的页都在，测试自己以 SQL 收拾那一行，第 233 条；A-M2）、River 丢掉的排队中的回滚被 Rescue 记为失败之后继续与结束都可用 | 第七份 | S3 |
| 矩阵 | `TestUpgradePermissionMatrix`（3.22） | 第七份 | S2、S3 |
| Rescue 认全部种类 | `TestTheRescue` | `transfer/app` | S3 |
| 网页 | 第 4 节的 vitest | `web` | S1–S4 |

**成本守卫**（13.4 第 8 条；`test-go-costs` 各一行、`-short` 跳过；界取本机两轮的中位数的三倍并对照 CI 的竞态；种子用 SQL，被测的那份准备不超过 4 秒（发起的守卫另种九本笔记本作不该读的行，种子共约 10 秒，这本笔记本的 1.4 秒）；统计状态 × 计划照 05-lessons 第 4 节）：

| 守卫 | 规模 | Step | 预算 |
|---|---|---|---|
| 发起 | 5 万节点、5 万页，其中 1.5 万页在提议的来源区（已有的 `raw`）里、各带 10 条待建，区外 3,499 页各 1 条、1,000 个目标（P1 审查 B-I1：夹具不放区里的页抓不到第二遍；另量第 16 步的 `ANALYZE` 与各段：`Capture`、`Seed`、`prepareIn` 的两遍、`SetWanted`）；库里另有九本各 5 万页的笔记本（`nodes`、`page_contents` 各多 45 万行，`edit_sessions` 4.5 万行），作发起不该读的行（S2 审查 S2B-M2，决定 240）；`upgrade_nodes`、`nodes`、`page_contents`、`edit_sessions` 从未分析、分析过这本笔记本的上一次发起、只有另一次进行中的升级的 10 万行的统计 × 自定义与通用计划（`TestUpgradeStartTakesItsTimeAtAnyStatistics`，组合根，`!race`，`test-go-costs-2` 一行）。设计的规模是 10 万页、3 万页在区里：放不进守卫配置下的期限，进 M12 的性能移交（第 232 条，S2 偏离 16） | S2 | 计划为主（决定 236、240）：每种统计 × 计划的第一次发起在发起的事务里取 `SetBaselineWanted`（`upgrade_nodes` 只扫一次，在嵌套循环的内侧时经 `upgrade_nodes_pkey`、两列都在索引条件里）、`CaptureBaseline`、`RootNodesByKey` 与两条会话的语句的自定义与通用计划：`nodes`、`edit_sessions` 只经索引条件含 `notebook_id` 的扫描，`page_contents` 只经 `page_contents_pkey`、索引条件含 `node_id`、在嵌套循环的内侧逐个节点读（后两项是修复第二轮的，S2A-N4；`TestTheStartGuardJudgesThePageReads` 以合成的计划钉住这道核对），不对这几张表顺序扫描。时长为辅：发起实际受的 `ctxStart` 是请求的四分之三（守卫的请求 6 s 照 lint 的成本守卫的配置，`bootstrap/wiki_lint_cost_test.go:27-34`，M10 决定 160，即 4.5 s），界取本机两轮中位数较大的乘 3、向上取整到 100 ms：建成时库里只有这一本，3.5 s；另种九本之后 3.6 s |
| 删除护栏的读 | 一次删 1 万个节点的子树，对着两次升级各 10 万行（`TestUpgradeProtectedReadsInTimeAtTheUnitsBound`，`page/adapter/postgres`，`!race`，`test-go-costs-2` 一行） | S1 | 计划为主：四种统计状态（没有统计、分析过、只有另一次升级的统计、`force_generic_plan`）× 自定义与通用两种计划都只经 `upgrade_nodes_pkey` 读、两列都进索引条件、没有顺序扫描，主键的列钉成 `(upgrade_id, node_id)`；时长为辅，本机两轮中位数较大的乘 3、向上取整到 5 ms，即 25 ms（初稿写"单元 2 s"，与上面的总规则不一致；S1 审查 S1B-I1 = S1C-M2，决定 236） |
| 回滚的计划 | 10 万个条目（纯函数）与合成读的三种统计状态 | S3 | 本机两轮中位数 × 3 的时长与 `TotalAlloc` |
| 回滚先删 | 10 万个零散的新页 | S3 | 同上 |
| 剩下的差的读 | 10 万节点、10 万页 | S3 | 请求 4 s |

### 7.2 交错

00 号文档 4.16 里 P1 的十一个，各一个确定的测试（持住双方都要的行，`pgtest.WaitForLockWaits`（`platform/postgres/pgtest/lockwait.go:25`）或 `WaitForLockWaitsOn`（`:45`）等到另一方在等，再放开；先后记在共用的日志里；结束时 `checkPages`、`checkLinks`、`checkSearch`）。停住后台任务的两种办法：持住它的一批要写的节点的行（`stopAtFirst`，`bootstrap/interleavings_background_stops_test.go:73`；`stopAfter`，`:689`；等心跳读到取消的 `cancelSeen`，`bootstrap/background_test.go:236`），或以 `wiring.transfer` 包住 `Tree`、在某次调用里停住（照 `bootstrap/source_background_test.go:96`；包装在 `retry` 看过 `running` 之后，`background.go:146-152`，所以放开之后那一次调用照样跑）。都在第七份的 `TestUpgradeInterleavings` 里：

| # | 交错 | 怎样逼出 | Step |
|---|---|---|---|
| 1 | 发起与正在跑的写入单元 | 照 `acmeTeam.interleaveOn`（`bootstrap/interleavings_workspace_test.go:185`：持住双方都要的行，先放第一个、等它在锁上，再放第二个）：单元先，发起等笔记本行，基线含它写的页；发起先，别人的 REST 写等行，之后答 `notebook.upgrading` | S2 |
| 2 | 发起与正在开的编辑会话 | 开启先持页的正文行与笔记本行的 `FOR SHARE`，发起等，之后答 `wiki.upgrade_pages_locked`；反过来开启被否决 | S2 |
| 3 | 发起与过期之前读了时钟的心跳 | 心跳在读时钟之后停住（钩子），会话在发起的那一刻已过期，发起删掉它的行，心跳放开之后找不到行可续 | S2 |
| 4 | 发起与正在上传的导入 | 导入的 `Check` 已过、上传停住，发起成功，上传放开之后建任务被拒、文件删掉 | S2 |
| 10 | 放弃与发起人正在跑的 `batch` | `batch` 先持笔记本行，放弃等它，回滚的计划含它的改动，做完之后等于基线 | S3 |
| 11 | 回滚的批与取消 | 照 `TestCancellingABackgroundJob`（`bootstrap/interleavings_background_stops_test.go:335`）：`stopAtFirst`（`interleavings_background_stops_test.go:73`）持住第一批，取消，`cancelSeen`（`bootstrap/background_test.go:236`）等心跳照常读到它（`job_run.go:167-168`），放开：第一批写完，下一批之前停下，任务 `cancelled`、`mode` 发一次；继续之后做完，或结束回滚。心跳要照常，所以这一条不装第 16 条的触发器（P1 审查 A-M1） | S3 |
| 12 | 回滚做完与删除笔记本 | 结束的事务停在锁笔记本行之后，删除等它；反过来删除先持，结束的事务等它，之后锁不到笔记本行（`LockNotebook` 不收已删的，`notebooks.sql:16-24`）：什么也不写、不调 `End`、记"stopped"、不发 `mode`；不死锁（P1 审查 A-N2；`rollbackStopped` 的同一情形由 `TestARollbacksEnd` 的假端口盖住） | S3 |
| 13 | 发起人被降为写者与他正在跑的写 | 成员的写先持笔记本行，发起人的 MCP 写等它，之后 `Grants` 答 `forbidden`、单元答 `notebook.upgrading` | S2 |
| 14 | 两次发起同时 | 第一次停在 `Start` 之后，第二次等笔记本行，之后答 `notebook.upgrading` | S2 |
| 16 | Rescue 记为中断、worker 还在跑、管理员结束回滚 | 00 号文档要钉的是"心跳写不进、worker 照跑"（第 225 条，`job_run.go:148-160`），不是"心跳读到不再运行"（那时 `stop(errGone)`，`:164-166`，放开之后不跑下一批，测试时过时不过；P1 审查 A-M1）。接线：应用由 `newAcmeTeamOn`（`bootstrap/interleavings_workspace_test.go:92`）建，同时交 `wiring.transfer`（包住 `Tree`，在第二批的调用里停住）与一个记进共用缓冲的 logger。次序：(1) 停住之后，测试在自己的库里装第 234 条的触发器；(2) 轮询日志的缓冲，等到这次装上之后的"rollback heartbeat not written"那一行（期限 `interleavingWait`，`:33`）：这行只在触发器装上之后、行还是 `running` 时落了一次心跳才记（`BeatJob` 只更新 `running` 的行，`jobs.sql:75-81`；触发器只对选中的行触发），心跳每秒一次，不等就时过时不过（修订核对 A2-M1）；(3) 以 Rescue 的 `InterruptJobs` 那条 SQL（`jobs.sql:120-132`）把任务记为 `failed`、`interrupted`；之后的心跳交回无行，走 `stop(errGone)`，不再记 warn（`job_run.go:153-166`）；(4) 结束回滚（任务已不在运行，答 200，冻结解除）；(5) 放开：停住的那一批在 `retry` 看过 `running` 之后，照跑，单元答 `ErrStaleUpgradeWrite`、什么也没写、新建的页还在。断言的次序：共用的日志里 warn 在"记为中断"之前；之后断言那一批什么也没写、任务行照旧是 `failed`、`interrupted`、记了"stopped"。"stopped"可能来自 `finish` 的 `errGone` 一支（心跳已在 (3) 之后读到无行），也可能来自 `rollbackStopped` 的 `FinishJob` 交回 `!ok`（心跳还没落），两者都满足断言，测试不区分。触发器只在这个测试的库里、只认这个任务 id，生产的代码与接线不改 | S3 |
| 17 | 发起与占满的 lint 名额 | 以 `Runs.Do` 的假计算占满全部名额，发起立即答 503（`Retry-After: 1`），不等（用时远小于名额的期限）；发起在另一个 goroutine 里，限时 2 秒等答复，`TryHold` 写坏成等名额时几秒内失败、不挂到第七份的期限（S2C-N2） | S2 |

第 5–9、15 条是 P2、P3 的（提交、检查、批准与驳回）。

### 7.3 变异

每个变异是实施时在本机对代码的一处改动，跑它旁边写的测试，要它失败（不提交；改回从备份复原）；第 10 节记下结果。

| # | 变异 | 让它失败的测试 | Step |
|---|---|---|---|
| 1 | 00055 的检查去掉"`mode = 'standard'`" | `TestChecksRejectCounterexamples` 的"LLM Wiki 带升级" | S1 |
| 2 | 00056 的 `upgrade_nodes_page_check` 去掉 `accepted_hash` 一个合取 | 同上的"缺认可的哈希" | S1 |
| 3 | `runtime-grants.sql` 不给 `upgrade_nodes` 的 `MAINTAIN` | `TestTheGrantsFileCoversEveryRelationAndFunction` | S1 |
| 4 | `ListNotebooks` 不选升级的列 | notebook 的六条查询的测试；`TestUpgradeStatesReachTheStreams` 的 `listNotebooks` | S1 |
| 5 | `toPageState` 不转换 `Upgrade` | `TestUpgradeFreezeReachesEveryPath`（别人的写成功） | S1 |
| 6 | 登记里没有冻结（`[mode, lock]`） | `TestUpgradeFreezeReachesEveryPath` | S1 |
| 7 | 冻结排在锁之后 | `TestUpgradeFreezeAnswersFirst` | S1 |
| 8 | 否决者不拒 | `TestUpgradeFreezeReachesEveryPath` 的开会话 | S1 |
| 9 | 放过发起人的非 MCP 写 | `TestFreezeAdmits`；`TestUpgradeFreezeReachesEveryPath` 的发起人 REST 写 | S1 |
| 10 | 去掉 `Grants.Ask` | `TestUpgradeFreezeFollowsTheInitiatorsRole`；`TestTheFreezeGuard` | S1 |
| 11 | 按 `Kind == revert` 放行 | `TestUpgradeFreezeLetsOnlyItsOwnWrites` | S1 |
| 12 | `reviewing` 也放过发起人的 MCP 写 | `TestFreezeAdmits`；`TestUpgradeFreezeReachesEveryPath` 的 `reviewing` 列 | S1 |
| 13 | 去掉过期的 `UpgradeOwn` 的核对 | `TestFreezeAdmits`、`TestTheFreezeLetsItsOwnUnits`；7.2 第 16 条 | S1、S3 |
| 14 | 删除的拒绝标在 `Changes[0]` | `TestTheFreezeGuard`（只有它：被标的节点只在 page 之内看得到，MCP 的工具错误只带 `code`、`message`、`op`，后台任务到不了护栏；`TestUpgradeGuardrails` 的 `batch` 一行钉的是整批在那个删除的操作被拒、什么也没写，对这条变异照过；S1 审查 S1C-M1，决定 237） | S1 |
| 15 | 删除的护栏不读保护行（只看 `baseline`） | `TestUpgradeGuardrails` 的补写的 `raw` | S1 |
| 16 | 上传的预检不判冻结 | `TestUpgradeFreezeReachesEveryPath` 的上传在存储将满时（答 `storage_full` 而不是 `notebook.upgrading`，3.24） | S1 |
| 17 | `CheckRevert` 不判冻结；`CheckRestore` 不判；导入的 `Check` 不判 | 同上的 `startRevert`（超过 `MaxJobItems` 的变更集）、`startTrashRestore`（过期的项）、`startImport`（存储将满），各一次 | S1 |
| 18 | 导入建任务的事务不判冻结 | `TestTheStartsRefuseAnUpgradingNotebook` 的建任务（S1）；7.2 第 4 条（S2） | S1、S2 |
| 19 | 冻结的判断放进 `StartDeps.lock` | `TestUpgradeFreezeLeavesTheRest` 的导出 | S1 |
| 20 | `ensureChangeset` 不记 `upgrade_id` | `TestTheChangesetsCarryTheirUpgrade`；UP3 的变更集 | S1 |
| 21 | 第三份的 `-skip` 不含第七份 | 不列：没有会因它失败的测试（第 223 条），由记录里的 `go test -list` 核对 | S1 |
| 22 | 发起不先删过期的会话 | 7.2 第 3 条 | S2 |
| 23 | 基线在补写之后记 | `TestUpgradeStart`（补写的页不在基线里） | S2 |
| 24 | `TryHold` 改为等名额 | 7.2 第 17 条（发起在另一个 goroutine 里、限时 2 秒，S2C-N2）；`TestTryHoldNeverWaits`（持着锁阻塞的写法在清理时的 `Stop` 上挂到测试二进制的期限，不动夹具） | S2 |
| 25 | `TryHold` 的持有者不计入 `active` | `TestStopWaitsForTheHolders` | S2 |
| 26 | 发起事务里另套一层期限（`WithinTx` 开在请求的 ctx 上、`fn` 里另套同长的期限：审查 C 的 d26b；或更短的：实施的写法） | 组合根 `TestUpgradeStart/its own deadline/inside a statement`（测试持住笔记本行，期限落在 `States.Lock` 的语句之中：变异下 pgx 关掉连接、答 500、有 error 级的日志；S2C-I1）；更短的那种另被 `between statements` 抓到 | S2 |
| 27 | `WithinTx` 交回 nil 时也看 `ctxStart` | `TestUpgradeStart` 的 `COMMIT` 之后到期一行 | S2 |
| 28 | `ANALYZE` 用 `ctxStart` | 同上（`ANALYZE` 失败的 warn 出现） | S2 |
| 29 | 第二遍不读来源区里的基线页 | `TestUpgradeStart` 的已有 `raw` 一行；`TestTheWantedCountsByPage` | S2 |
| 30 | 第一遍记页数时不重置（`seen` 跨页） | `TestTheWantedCountsByPage` | S2 |
| 31 | `computeIn` 自己读时钟 | `TestComputeInTakesItsMoment`（S2 的发起只调 `prepareIn`、不经 `computeIn`，`TestUpgradeStart` 抓不到它；S2 偏离 8） | S2 |
| 32 | 发起不 `Publish` | `TestUpgradeStatesReachTheStreams` | S2 |
| 33 | wiki 的删除订阅者交空 | `TestUpgradeDeletion` | S2 |
| 34 | `checkPages` 去掉"有升级列就有 wiki 行" | 它的 SQL 造出的违反 | S2 |
| 35 | `stale_pages > 0` 不拒 | `TestUpgradeStart` 的 `wiki.upgrade_stale_index` | S2 |
| 36 | 根下的 `raw` 附件不拒 | `TestUpgradeStart` 的 `wiki.upgrade_title_taken` | S2 |
| 37 | Rescue 的表漏掉 `rollback` | `TestTheRescue`；`TestUpgradeRollback` 的 River 丢掉的一行 | S3 |
| 38 | 写入键或 `Writing` 不含 `rollback` | 索引的一半：`TestChecksRejectCounterexamples` 的写入键；查询的一半：`TestOneWritingJobAtOnceInANotebook`（`transfer/adapter/postgres/store_test.go:304`，加一个排队的回滚任务的行）与 `TestUpgradeRollback` 的"回滚在跑时继续与结束回滚答 `transfer.busy`"（`TestEndUpgradeRollback` 用假的端口，碰不到 SQL；P1 审查 A-M2） | S3 |
| 39 | 回滚漏删新建的子树 | `TestARollbackPassAtATime`；`TestUpgradeRollback`（新建的不在回收站） | S3 |
| 40 | 合成的条目带 `Revision` | `TestRollbackState`；`TestUpgradeRollback`（停在 `page.changeset_trimmed`） | S3 |
| 41 | 回滚的单元 `UpdateLinks = true` | `TestUpgradeRollback`（写回撞上 `ErrRevisionMismatch`，任务 `internal`） | S3 |
| 42 | "不在原位"只比父节点 | `RollbackLeft` 的测试；`TestUpgradeRollback` 的只剩一次改名 | S3 |
| 43 | 先删的一步每棵子树一个单元 | `TestPackFreeSubtrees`；回滚先删的成本守卫 | S3 |
| 44 | 差不缩小时不停 | `TestARollbackRunsPassByPass` | S3 |
| 45 | `Stopped` 不看 `FinishJob` 的 `ok` | `TestARollbacksEnd`；7.2 第 12 条 | S3 |
| 46 | 回滚停下不发 `mode` | `TestUpgradeStatesReachTheStreams` 的停下 | S3 |
| 47 | `RollbackEnd` 不清列 | `TestUpgradeRollback`（冻结还在） | S3 |
| 48 | 结束回滚不删基线行 | `TestEndUpgradeRollback`；`checkPages` 的"基线行只属于进行中的升级" | S3 |
| 49 | 差不空时 `succeed` 照写成功 | `TestARollbacksEnd`；`TestUpgradeRollback` 的 `internal` 一行 | S3 |
| 50 | 回滚的任务允许写者取消 | `TestUpgradePermissionMatrix` 的"its own rollback, ended" | S3 |
| 51 | 取消的规则放在 `sees` 之前 | 同上的"another's rollback, queued" | S3 |
| 52 | 放弃先记任务的 id、后写状态（两条 `UPDATE`） | `TestAbandonUpgrade` 的真库一行（`notebook_upgrades_rollback_job_check`） | S3 |
| 53 | 回滚的每一遍不核对升级 | `TestUpgradeRollback` 的"两遍之间以 SQL 结束升级"：原样只记"stopped"、任务行照旧 `running`、没有单元跑；变异下基线行已删，`RollbackFreeForest` 没有 holder，全部活节点成了先删的子树，单元被守卫以 `ErrStaleUpgradeWrite` 拒，任务以 `internal` 结束、记 error 级的"rollback ended"（同一遍里的一批由守卫的过期核对拒，有没有每遍的核对都一样，所以测试停在两遍之间；P1 审查 A-M2） | S3 |
| 54 | `writesNotebook` 不看升级 | 控件表 | S4 |
| 55 | `lostBy` 不认 `notebook.upgrading` | `edit-session` 的 vitest | S4 |
| 56 | `modeChanged` 不重读升级 | `handlers` 的 vitest | S4 |
| 57 | `refreshedOnConnect` 不含 `upgrade` | `event-stream` 的 vitest | S4 |
| 58 | 横幅在升级页上也带链接 | 横幅的 vitest | S4 |
| 59 | 任务页的取消对写者也显示 | 任务页的 vitest | S4 |
| 60 | 回滚的报告退回导入的计数（`shownCounts` 不含 `rollback`） | 回滚报告的 vitest | S3 |
| 61 | `jobs-server.ts` 的 `writingKinds` 不含 `rollback` | 任务页的 vitest：假服务端有一个页面还没读到的回滚任务，用户开始导入，页面显示 `transfer.writingBusy`（按钮由 store 的那一份决定，`transfer.store.ts:101`；P1 审查 C-N4） | S3 |
| 62 | 第二遍把整区的页一次传入、在连接之后按（页、位置）的游标排序分页 | 应用不分段：`TestTheAreaPassReadsByChunk`；适配器不限 512 个 id：`TestTheAreaWantedHeadsGoByBatch/refused`；与 `unnest` 的 id 连接之后排序：计划的测试 `TestLintPlansOfTheAreaPass`；发起的成本守卫只抓得到三者合在一起的那种（审查者 B 实测的形状，自动选择计划 66 s）：5 万页上单独的整区一次传入只慢到界的约 90%（S2 偏离 19） | S2 |
| 63 | 发起不调 `Protect` | `TestUpgradeStart` 的保护行 | S2 |
| 64 | `upgrade_nodes_baseline_check` 的 `baseline = (name IS NOT NULL)` 改成单向（`NOT baseline OR name IS NOT NULL`） | 反例"保护行带名称" | S1 |
| 65 | 同一检查的 `(name IS NULL) = (sort_order IS NULL)` 改成单向 | 反例"次序有、名称空" | S1 |
| 66 | 00055 的 `(upgrade_id IS NULL) = (upgrade_by_id IS NULL)` 改成单向 | 反例"只有发起人" | S1 |
| 67 | 发起准备整份摘要 | `TestUpgradeStart` 的 `wiki/app` 一半（假的 `Rules` 记下查询） | S2 |
| 68 | 后台任务的单元放过冻结：守卫的 `Step` 没有 `Background`，改在 `page/app/unit.go` 造 `Write` 处，`spec.Background` 的单元的 `Write.Notebook` 去掉升级（或取当前升级为 `UpgradeOwn`）（S1 审查 S1C-N7） | `TestUpgradeFreezeReachesEveryPath` 的"任务的批" | S1 |
| 69 | 结束的两个钩子锁不到笔记本行时照调 `End` 或 `Stopped` | `TestARollbacksEnd`；7.2 第 12 条 | S3 |
| 70 | `rollbackSucceeded` 不记"rollback ended" | `TestARollbacksEnd` 的日志 | S3 |
| 71 | `PlanRollback` 的 `ErrJobGone` 经 `stopped`（归为 `internal`） | `TestARollbacksEnd`；`TestUpgradeRollback` 的"两遍之间"（任务行成了 `failed`） | S3 |
| 72 | 合成的条目漏了一类差（例如只差次序值的基线节点） | `TestRollbackState` 的次序一行；`TestRollbackPlansPutTheBaselineBack` 的"一遍"；`TestUpgradeRollback` 的"只有一遍合成" | S3 |
| 73 | `jobTitle` 漏了 `rollback` 一支 | `transfer-names.test.ts` 的回滚任务的标题 | S3 |
| 74 | 升级页的任务转为结束时不重读升级 | `upgrade-page.test.tsx` 的"停下之后出现两个按钮与三个数" | S4 |
| 75 | 升级页的轮询在任务结束之后不停 | 同上（之后不再有请求） | S4 |
| 76 | `pages-locked.tsx` 不认 `wiki.upgrade_pages_locked` | `pages-locked` 的 vitest；`general-page.test.tsx` 的列出页与编辑者 | S4 |
| 77 | 升级页对非管理员也发读 | `upgrade-page.test.tsx` 的非管理员 | S4 |
| 78 | 接入一节对其他管理员也写 prompt 名 | `upgrade-page.test.tsx` 的接入 | S4 |
| 79 | 发起成功之后焦点不给升级页的标题 | `general-page.test.tsx` 的焦点 | S4 |
| 80 | 00055 的 `(upgrade_id IS NULL) = (upgrade_at IS NULL)` 改成单向 | 反例"只有时刻" | S1 |
| 81 | `upgrade_nodes_page_check` 的 `(revision IS NULL) = (x IS NULL)` 四个合取任一改成单向 | 反例"保护行带哈希""保护行带字节数""保护行带待建数""保护行带认可的哈希"各杀一个 | S1 |
| 82 | `notebook_upgrades_submission_check` 的两个合取任一改成单向 | 反例"只有 `submission_id`""只有 `submitted_by_id`"各杀一个 | S2 |
| 83 | `batch` 的单元跳过冻结的守卫（S1 审查 C 的 o1） | `TestUpgradeFreezeReachesEveryPath`、`TestUpgradeToolsErrorsCarryTheirCodes` 的 `batch` | S1 |
| 84 | 删除的护栏只问子树的根（o4） | `TestTheFreezeGuard` 的删除护栏；`TestUpgradeGuardrails`（删掉了收着基线页的新建节点） | S1 |
| 85 | 任何人的 MCP 写都去问 `notebook.change_mode`，不看发起人（o7，同审查 A 的 W1） | `TestUpgradeToolsErrorsCarryTheirCodes`（别人发起的那一本）；`TestFreezeAdmits` | S1 |
| 86 | 组合根的 `transferNotebooks` 不转换升级（o9） | `TestUpgradeFreezeReachesEveryPath` 的 `startImport`（答 202、任务多一行） | S1 |
| 87 | 删除护栏的读用不上主键：只剩 `upgrade_id` 走键、两列都不走键，或写成 `IS TRUE` 的条件（审查 B 的 c1，C 的 c1、c2） | `TestUpgradeProtectedReadsInTimeAtTheUnitsBound` 的计划 | S1 |
| 88 | 00056 去掉 `upgrade_nodes_pkey`（B 的 c2） | 同上（`pg_get_indexdef` 找不到主键） | S1 |
| 89 | 00056 的主键的列序换成 `(node_id, upgrade_id)` | 同上（只有列的钉住抓得到：计划仍走主键、两列都进条件） | S1 |
| 90 | 导入的 `Check` 把冻结的判断挪到 `place` 之后（A 的 A1） | `TestTheStartsRefuseAnUpgradingNotebook` 的"an import's Check"（父节点不存在的请求、调用序列） | S1 |
| 91 | transfer 的 `notUpgrading` 对没有的笔记本放过（A5） | 同上的"the notebook deleted as its state is read" | S1 |
| 92 | transfer 的开任务在判定之前答 `notebook.upgrading`（A7） | 同上的恢复与撤销的"by a reader" | S1 |
| 93 | 冻结的守卫对不认识的判定放行（去掉 `default`） | `TestTheFreezeGuard` 的"a freeze it does not know" | S1 |
| 94 | page 的 `UpgradeRollingBack` 不是 notebook 的那个字符串 | `TestPageNotebooksConvertTheState` | S1 |
| 95 | 503 的判断去掉 `errors.Is(err, context.DeadlineExceeded)`（审查 C 的 o24） | `wiki/app` 的 `TestUpgradeStart/a-refusal-rolled-back-past-its-deadline`（领域的拒绝在回滚越过期限时照答 409；S2C-M1） | S2 |
| 96 | 503 的判断去掉 `ctxStart.Err() != nil`（A 的 W1） | 同上的 `a-commit-out-of-its-own-time`（`COMMIT` 自己超时、`ctxStart` 还活着：答提交的错误；S2A-N1） | S2 |
| 97 | 名额放得早：`TryHold` 之后立刻放开（A 的 W3）、`fn` 里 `defer` 放开、提交之前（C 的 o3）；放得晚：`WithinTx` 之后不放、只靠函数末尾的 `defer`（A 的 W6） | 同上的 `it-writes-what-the-notebook-lacks`（替身在 `Capture`、准备、提交之前与 `Analyze` 四处探名额：前三处被持着，`ANALYZE` 之前已放开；S2A-M1 = S2C-N3） | S2 |
| 98 | `TryHold` 挪前：挪到 `Writing` 之前（A 的 W4）或 `refuseSessions` 之前（C 的 o2b） | 同上的 `each-refusal-in-its-turn` 的 `-a-full-slot` 各格（名额被占满时第 4–8 步照答自己的 409，`a-stale-index` 答 503） | S2 |
| 99 | 第二遍不判资格（A 的 L5）、不去重（L8）、不取父页的键（L10） | `TestTheWantedCountsByPage` 的 `an-llm-contract`（区里的页带重复的目标、`./Alpha`、图片与虚拟的 `index`；S2A-M2） | S2 |
| 100 | 第二遍不核对 `s.source`（A 的 L4）；不跳过已解析的链接（L7） | 同上的 `a-page-outside-the-area-answered`；`a-resolved-link-answered`（S2A-N2） | S2 |
| 101 | `CurrentUpgrade` 去掉 `rolling_back`（A 的 WA2） | `TestTheUpgradesRows`（以 SQL 设成 `rolling_back` 的一行；S2A-N3） | S2 |
| 102 | `prepareIn` 返回之前投影（n1） | `TestComputeInTakesItsMoment`（`prepareIn` 的调用恰是 `state, nodes, prepare`；S2C-N1） | S2 |
| 103 | 发起读 page 的表丢了笔记本的键：生成的 `CaptureBaseline`（审查 B 的 c2）、`RootNodesByKey`（rk）、两条会话的语句（ds、ls）写成 `(notebook_id = $n) IS TRUE` | 发起的成本守卫的计划（库里另有九本笔记本；S2B-M2，决定 240） | S2 |
| 104 | `CaptureBaseline` 的正文不逐个经主键读：改回普通的 `LEFT JOIN`（f240a）、`LATERAL` 不带 `LIMIT 1`（f240b）、与按 `node_id` 排序的子查询连接（C 的 x1） | 同上（"page_contents read by a Seq Scan"） | S2 |
| 105 | 生成的 `SetBaselineWanted` 丢 `upgrade_id`（B 的 c1） | 同上（分析过、通用计划一格：主键只以 `node_id` 走） | S2 |
| 106 | 第二遍的读用不上主键或要排序：生成的 `LintAreaWantedEdgeHeads` 丢 `source_id` 的键（B 的 c3）；待建的过滤挪出 `LIMIT 1` 的 `LATERAL`（C 的 62e，S2 偏离 1 反过来） | `TestLintPlansOfTheAreaPass`（成本守卫对它们只看时长，抓不抓得到看机器） | S2 |
| 107 | 00057 的五个方向：`baseline_pages`、`baseline_wanted` 的下界去掉，`rollback_end_check` 的三个 `=` 各去掉一个方向（`false` 就有剩下的节点数、有页数就有节点数、有新节点数就有节点数）（B 的 g1–g5） | `TestChecksRejectCounterexamples` 的"a baseline of -1 pages""-1 wanted at the start""a short rollback with nothing left""a complete rollback with its pages left""a complete rollback with its new nodes left"各杀一个（S2B-M1） | S2 |
| 108 | 部分唯一索引的谓词去掉 `'rolling_back'`（g6）、去掉 `deleted_at IS NULL`（g7） | 唯一索引的"第二行 `rolling_back` 答 23505""软删除的 `upgrading` 放过"（S2B-N1） | S2 |
| 109 | `checkPages` 的升级规则去掉"同状态"、基线页数、源根"是页"三个合取之一（cp-state、cp-pages、cp-kind），或连同"同笔记本"一起（cp-all） | `TestPageViolationsCatchEachUpgradeGap` 的三行（wiki 行另一个状态、基线页多一、源根是附件），每行只报自己的规则（S2B-N3） | S2 |
| 110 | 成本守卫对 `page_contents` 的核对不要求 `Index Cond` 含 `node_id`（r1）；不要求在嵌套循环的内侧逐个节点读（r2）（S2A-N4） | `TestTheStartGuardJudgesThePageReads`（与守卫同在 `!race` 的 `bootstrap/upgrade_start_cost_test.go`，不连数据库，short 一行）：Nested Loop 内侧没有条件的一格杀 r1，Merge Join 内侧有条件、只读一次的一格杀 r2（修复第二轮 `9b3000c1`） | S2 |

### 7.4 门禁

6.5。另：`make gen-check` 在每个改契约的提交上；`TestTheToolListsAreTheContract` 照旧（P1 不改 `tools.json`：MCP 的码在 `mcpToolCodes()` 与工具的错误里，不在工具的说明里）；`TestTheBuiltInInstructionsAreShort` 照旧（P1 不改内置的说明）。

### 7.5 CI 的余量

新的整个程序的测试都以 `TestUpgrade` 起名，只落进第七份（`BOOTSTRAP_SEVENTH = ^TestUpgrade`）；改的已有测试（运行时角色、`checkPages` 的规则、覆盖的检查、第五份的码的核对）只多几行，留在原份。第七份在 P1 末约有：S1 的七个、S2 的五个（`Start`、`States`、`StatesReachTheStreams`、`Deletion`、`Interleavings` 的 S2 部分）与矩阵、S3 的 `Rollback` 与交错的 S3 部分。每个 Step 的记录写出七份各自的 `go test -list` 与本机在 heavy 锁下各跑两轮的平均（`make -j1 test-go-bootstrap-<n>`）；合并之后两次 `main` 的 CI 里每份 Test 一步的平均要在 450 秒之下（06-environment 第 49 行的规则），第七份在 P1 末估在 150–250 秒（交错与矩阵各约 30–60 秒，照 M10 的同类），P2、P3 再加。S1 合并之后两次 `main` 的第六份在 EPYC 7763 上是 461、459 秒，过了线；照规则把 `TestIngestBefore`（7763 上约 120 秒）挪给第七份（`BOOTSTRAP_SEVENTH = ^Test(Upgrade|IngestBefore$$)`，`2beef88c`），第六份折到 7763 约 386 秒，第七份现约 147 秒，P1 末估约 274–374 秒；`TestIngestUnit`（约 261 秒）不挪，挪了第七份在 P1 末会过线。S2 加七个组合根的测试（第七份六个，第四份的 `TestPageViolationsCatchEachUpgradeGap` 一个），竞态构建 541 个、七份 158、51、121、124、62、11、14；第七份本机两轮约 90 秒，CI 149、151 秒，折到 7763 约 164 秒（10.2）。e2e 加 UP1 的三种浏览器，作业的 20 分钟照 M10 的量（6:36–8:12）有余。

### 7.6 P1 怎样量

- S1：第七份与第三份的 `go test -list`；第七份本机两轮；删除护栏的守卫的本机中位数。
- S2：发起的成本守卫三种统计状态 × 两种计划的本机中位数与它在 `ctxStart` 之内的余量，各段（`Capture`、`Seed`、`prepareIn` 的两遍与 `PreparationTimings` 的分规则耗时、`SetWanted`、`ANALYZE`）分开记（放不下时照第 232 条，写进记录；P1 审查 B 的未核实的怀疑。S2 量下来 10 万页放不下守卫配置下的 3 s，期限改为请求的四分之三、守卫减为 5 万页，审查之后库里另种九本笔记本，决定 240；各项的数在 10.2）；`LintAreaWantedEdgeHeads` 的整条流在审查者 B 的数据上的时长，与 B 实测的对照：整组一次传入时强制自定义计划 18.1 s、通用计划每批 256 ms（约 150 s）、自动选择 66.2 s，一段 512 页一次读 0.17 s（PostgreSQL 18.6，10 万页里 3 万页在区里、各 10 条，30 万条边）；`SetBaselineWanted` 对照 B 实测的三种统计状态（从未分析：自定义 432 ms、通用 671 ms；分析过已结束的另一次升级：386、444 ms；分析过进行中的另一次：400、445 ms）；第七份本机两轮。
- S3：回滚的计划、先删、剩下的差三个守卫；第七份本机两轮。
- S4：e2e 三种浏览器的 UP1 的时长。

## 8. 完成标准

P1 完成，当且仅当：

1. 四个 Step 各自合并进 `main`，合并之前 6.5 的门禁全绿、审查与修复核对结束；每个 Step 合并之后两次 `main` 的 CI 里组合根七份的 Test 一步的平均都在 450 秒之下（7.5），否则照 06-environment 第 49 行的规则处理之后才开下一个 Step。
2. 第 3–5 节的每个测试在，7.3 的每个变异被它旁边的测试抓到（第 10 节记下），3.24 的每个反向对照在测试里。
3. 第 2 节的移交的落实做完；四份移交（`handoffs/M10-mode.md`、`M5-events.md`、`M8-P4-background.md`、`M9-mcp-upgrade.md`）的"处理进展"各加一行（每个 Step 的文档提交写它做了的项），状态仍是 `open`（00 号文档"移交的落实"：M11 收尾时改为 `done`）。
4. **00 号文档**（`docs/v0.1/M11-upgrade/00-M11-design.md`）：
   - **接受本文的文档提交**：2.1 的十三处照各条写的新文字改；2.3 的三条（233–235）写进第 11 节，接在"00-M11 的设计者的取舍……与修订核对之后的 225–232"一节之后，另起一节"2026-11-08：P1 的设计审查之后的取舍 233–235（主持者的决定，负责人可以改判）"，每条一句、带它在本文与审查记录里的位置；头的"状态"（`:7`）改为"进行中（P1 设计已审查，S1 开工；[审查记录](reviews/M11-design-review.md)；总体设计照第 7 节"总体设计的修订"改过）"；第 12 节的 P1 一行 `| P1 冻结、发起与放弃（S1–S4） | — | 未开始 | — | — |` 改为 `| P1 冻结、发起与放弃（S1–S4） | [01-P1](01-P1-freeze-start-rollback.md) | 进行中（设计已审查） | [设计](reviews/P1-design-review.md) | — |`；第 13 节加一行"P1 设计已审查，S1 开工；2.1 的修订落进本文"，依据是 P1 的设计与它的审查记录。
   - **每个 Step 的文档提交**（在 Step 的分支上、合并之前）：第 12 节的 P1 一行的"状态"写到这个 Step、"审查"一格加它的记录 `reviews/P1-S<m>-<slug>-review.md`，"合并"一格写"见本文第 12 节"的占位，合并之后在 `main` 的小文档提交里填入合并的提交号；第 13 节加一行（实施的结果、照实与设计不同之处、随之改的节）。
   - **`docs: M11/P1 done (M11/P1)`**：第 12 节的 P1 一行"完成"；头的"状态"改为"进行中（P1 完成，P2 设计中；…）"；第 13 节加"P1 完成"一行。
5. **v0.1 总体设计**（`docs/v0.1/v0.1-design.md`；00 号文档第 7 节"总体设计的修订"第 30 条里属于 P1 的部分，各在实现它的 Step 的文档提交里改，用脚本断言锚点恰好出现一次；行号是 `9184f67b` 的）：
   - 13.1 第 1 条（`:1049`）末尾加——S1："M11 起单元在笔记本行的锁下也读它的升级（`Write.Notebook.Upgrade`）；冻结的守卫排在守卫与否决者的最前，按升级与 `Write.UpgradeOwn` 判、不按 `Kind`；`UpgradeOwn` 只由发起补写的页与回滚的单元设，REST 与 MCP 的适配器建不出它（M11/P1）。"S2："`TreeWrites().Seed` 的 `SeedSpec.Upgrade` 让发起补写的页成为这次升级自己的写（M11/P1）。"S3："回滚的单元是 `rollback` 的变更集、`UpgradeOwn` 是它的升级、不让参与者改写链接（M11/P1）。"
   - 13.1 第 4 条（`:1071`）末尾加——S1："冻结的码在写入单元的领域的码之后、有预检的路径在判定之后先答它（M11/P1）。"S2："升级的操作的码的次序：取值的 422 → 404 → 403 → 状态的 409 → 资源占用的 409 → 内容的 409 → 503，`startUpgrade` 的 503 在资源与内容之间（M11/P1）。"
   - 13.1 第 5 条（`:1073`）末尾加——S2："wiki 的 `notebook_upgrades` 的行只在笔记本行的 `FOR NO KEY UPDATE` 下改，排在笔记本行之后（M11/P1）。"S3："回滚的任务在升级的事务里开时，transfer 的队列锁在笔记本行之后；回滚结束的事务先锁笔记本行，再写任务行（M11/P1）。"
   - 13.1 第 8 条（`:1100`）末尾加——S1："`notebook.upgrading` 由 page 与 transfer 各自定义（同 `notebook.not_found`），只在 MCP 的 `page.upgrade_protected` 进 `mcpOwnCodes()`（M11/P1）。"S2："wiki 同样定义 `notebook.upgrading`；`wiki.upgrade_pages_locked` 也带 `locks`，至多 100 项，`detail` 写总数（M11/P1）。"（wiki 的 `ErrUpgrading` 在 S2 才有，P1 审查 C-N6。）
   - 13.1 第 9 条（`:1102`）末尾加——S2："升级的发起只读一次时钟：会话的判活、基线、补写的页与 lint 的 `computed_at` 都用这一刻（M11/P1）。"
   - 13.1 第 10 条（`:1103`）末尾加——S2、S3：3.23 的各行，写法"升级的发起、放弃、结束回滚与回滚的每一遍各一行 info，只有 id、数与耗时（M11/P1）"。
   - 13.1 第 11 条（`:1104`）与第 21 条（`:1127`）末尾加——S1："`page.NewFreezeGuard(pool, authorizer, admin)` 同 `page.NewModeGuard` 不只凭连接池（M11/P1）。"S2："`notebook.NewUpgradeStates(pool, subscribers)` 在调用方的事务里写升级的列并调模式变化的订阅者（12.1 第 6 条的例外），`page.NewUpgrades(pool)` 只凭连接池（M11/P1）。"S3："`wiki.NewRollbackEnd` 由组合根在 transfer 之前以只凭连接池的端口建，`(*transfer.Module).Rollbacks()` 在调用方的事务里开回滚的任务、不记日志（M11/P1）。"
   - 13.1 第 23 条（`:1137`）末尾加——S2："`Runs.TryHold` 在笔记本行的锁下只试取一个名额、不等，持有者计入 `active`，`Stop` 等它放开（M11/P1）。"
   - 13.1 第 31 条（`:1164`）末尾加——S2："发起时列出的编辑会话至多 100 项（M11/P1）。"S3："回滚一遍的树的条目至多 `MaxRollbackItems = 300,000`，先删的一个单元至多 `MaxUnitNodes` 个节点（M11/P1）。"
   - 13.2 第 9 条（`:1199`）末尾加——S4："升级的故事里刻意的 409 以 `pageWatch.expectConsole({ errors: [failedToLoad(409)] })` 声明（M11/P1）。"第 18 条（`:1209`）末尾加——S4："升级页（M11）同样只给管理员的读。"
   - 13.4 第 1 条（`:1262`）末尾加——S1："M11/P1：迁移的每个检查的每个合取各一个反例，写成 `=` 的合取两个方向各一个（把 `=` 改成单向的蕴含时失败）。"S2、S3 的文档提交不再加（同一句管 00057–00059）。
   - 13.4 第 3 条（`:1264`）末尾加——S2："M11/P1：升级的故事在 `e2e/stories/upgrade/`；UP2 的 MCP 版本豁免（MCP 不能发起），UP4 只有 MCP 版本（REST 的写在冻结里就被拒），各由写明的整个程序的测试盖住。"S3："UP3 的 MCP 版本豁免（MCP 不能放弃或结束回滚），由 `TestUpgradeFreezeReachesEveryPath` 的 `rolling_back` 一格盖住。"S4："M11/P1：UP1 的页面版本在三种浏览器里跑；UP2 的页面版本只走编辑会话与写入任务两种拒绝。"
   - 13.4 第 4 条（`:1265`）末尾加——S2："M11/P1：发起与写入单元、编辑会话、过期之前读了时钟的心跳、正在上传的导入、两次发起、降级的发起人、占满的 lint 名额各一个确定的交错。"S3："M11/P1：后台任务的心跳写不进而 worker 照跑的情形，由测试在自己的库里装只认那个任务的触发器逼出，生产的代码不留口子（第 234 条）。"
   - 13.4 第 5 条（`:1272`）末尾加——S1："M11/P1：冻结的守卫与否决者的登记由去掉冻结一项的子测试守着（`wiring.page` 改 `Guards` 与 `EditSessionVetoers`）。"
   - 13.4 第 6 条（`:1273`）末尾加——S2："M11/P1：升级的操作在单独的工作区里的升级的变体上（没有升级、`upgrading`、`reviewing` 各五本），以 SQL 种下之后跑 `checkPages`。"S3："`rolling_back` 的五本带停下的回滚任务；占写入键的种子（别人的、排队中的回滚任务）只种在只有只读行的 `reviewing` 的几本上；种下的任务登记进 `workspaceOfRow`。"（S2 的文档提交写不到 S3 才有的东西，修订核对 C2-N1。）
   - 13.4 第 8 条（`:1275`）末尾加——S1："M11/P1：删除护栏的读（一次删 1 万个节点）。"S2："发起在 10 万页上（3 万页在提议的来源区里），以发起实际受的期限为预算；第二遍的读有自定义与通用两种计划的计划的测试。"S3："回滚的计划与先删各 10 万、剩下的差的读在 10 万页上。"
   - 第 15 节：每个 Step 的文档提交加一行（日期、改了上面哪几条、依据是本文与 Step 的审查记录、`M11/P1/S<m>`）。
6. `docs/v0.1/claude_to_codex/06-environment.md` 的分份两条在 S1 的第一个提交里改（3.8）。

## 9. 交给后面的 Phase

- **P2/S1（护栏）**：`FreezeGuard.guardrails`（3.4）今天只判删除，P2/S1 在它里面加不改写链接的移动与改名、正文的比较、`Folded`；账本更新 `upgrade_nodes.accepted_hash`（`Capture` 已写成发起时的哈希）；`TestUpgradeGuardrails` 与 UP4 补全；三个码进 `mcpToolCodes()`、`upgradeToolCodes()`、`mcpOwnCodes()`。
- **P2/S2（闸门与提交）**：`RunKey` 的检查、条目、提交三种与 `Scope`，`Runs.Do` 的结果的泛化，`logFinished` 的名字（2.1 第 11 条）；`computeIn` 已收调用方的时刻、`WantedByPage` 已有（闸门 6 读 `upgrade_nodes.wanted`）；`Upgrade.report` 与 `UpgradeReport` 加为必填（2.1 第 6 条）；提交用 S3 已有的 `UpgradeStates.SetState`；检查与条目用 `computeIn`（发起用的 `prepareIn` 不投影）。两个工具（`upgrade_verify`、`upgrade_submit`）在 `mcpToolCodes()` 的行要整行进 `upgradeToolCodes()`，由第七份的 `TestUpgradeToolsErrorsCarryTheirCodes` 答出（第五份的应用种不出升级，答不了它们）；`TestUpgradeToolsMatrix` 也在 P2/S2（第 222 条；P1 审查 C-N7）。
- **P2/S3、P3/S3（网页）**：`upgrade.service.ts` 与 `NotebookStore` 加检查、提交、审阅的方法；升级页加"检查"与"审阅"两节；P3/S3 的 `threeBrowsers` 加 `upgrade\/up7-`。
- **P3/S1（审阅）**：`UpgradeStates.Approve`、`page.NewSourceRebuild`；`rejectUpgrade` 用 3.20 的 `rollBack(ending)`；批准与退回删基线行用 S3 的 `DeleteBaseline`；第 181 条。
- **P3/S2**：prompt `upgrade`、README 的"升级"一节、MCP 人工清单的 K。
- **M12**（00 号文档"写给后面的 M"已列）：P1 量的发起、回滚的计划、先删与剩下的差在 10 万上的数，写进 M12 的性能移交的那几条（S2：10 万页的发起放不下守卫配置下的 4.5 s，10.2，00 号文档第 232 条）。

## 10. 结果

每个 Step 合并之前在这里写它的实施、审查与验证的结果；合并的提交见 00 号文档第 12 节。

### 10.1 S1：数据、冻结与删除的护栏（服务端）（2026-11-09，合并 `27b30c7f`）

- 提交（从 main 的 `379a2d6d` 开始，照 6.1 的次序，7 个；Step 的文档提交是主持者的）：
  - 组合根的第七份：`BOOTSTRAP_SEVENTH = ^TestUpgrade`、`test-go-bootstrap-7`，第三份的 `-skip` 加它，`test-go` 与 `test-go-bootstrap` 的依赖、CI 的 `bootstrap-7`（25 分钟）、`06-environment.md` 的两处（`8b98ade3`）。
  - 迁移 00055（笔记本的四列与 `notebooks_upgrade_check`）与 00056（`upgrade_nodes`、`changesets.upgrade_id` 与它的部分索引），sqlc 的两项；`schema_test` 的十个名称、一本升级中的普通笔记本与三行、27 个反例（每个只违反一个合取，写成 `=` 的合取两个方向各一个）；`TestTheUpgradeColumnsGoDown`；`runtime-grants.sql` 的两行与 `analyzed`；M10 的 lint 迁移的成本夹具先经真的 Down 退 56、55（`3b23bad0`）。
  - notebook 的读带升级：领域的 `Upgrade`、根的 `State.Upgrade` 与别名、`ActionChangeMode`；六条读笔记本的查询与 `upgradeOf`；契约的 `NotebookUpgrade`、`NotebookUpgradeState` 与必填可空的 `Notebook.upgrade`；`TestTheStatesAnswerTheirUpgrade`、`TestTheNotebooksAnswerTheirUpgrade`、`TestAnUpgradingNotebookIsAnsweredWithItsUpgrade`；网页两处字面量（`2f209937`）。
  - 冻结：page 的 `NotebookState.Upgrade`、`FreezeAdmits`（`TestFreezeAdmits`）、`ErrUpgrading`、`ErrStaleUpgradeWrite`；`Write.UpgradeOwn` 与 `UnitSpec.UpgradeOwn`；`FreezeGuard`（守卫与否决者）、`decideAction`；`ensureChangeset` 记 `upgrade_id`；组合根的 `toPageState` 转换升级、登记 `[freeze, mode, lock]`；九个经单元的操作与 `openEditSession` 声明 `notebook.upgrading`，page 的 `adapter/http` 测试答出；`mcpToolCodes()` 的五行与 `upgradeToolCodes()`、`TestUpgradeToolsErrorsCarryTheirCodes` 的一半；两种语言的文字；`TestTheFreezeGuard`、`TestTheFreezeLetsItsOwnUnits`、`TestTheChangesetsCarryTheirUpgrade`（`1a54bfcd`）。
  - 删除的护栏：`UpgradeNodes.Protected`（`UpgradeProtected`，按主键，在单元的事务里），`page.upgrade_protected` 标在子树里第一个受保护的节点；`mcpOwnCodes()`、`mcpToolCodes()` 与 `upgradeToolCodes()` 的两对；`TestUpgradeProtected`、`TestCreateChangesetWritesItsUpgrade`、`TestUpgradeGuardrails`（`d668fc3f`）。
  - 预检与 transfer 的开始：page 的三个预检在判定之后先读一次状态（`Writer.notUpgrading`）；transfer 的 `NotebookState`、端口的 `ShareByID` 交回它、新的 `StateOf`，组合根的 `transferNotebooks`；导入的 `Check`、建导入的任务、恢复与撤销的开始答 `notebook.upgrading`，导出照常；四个操作的声明与 asset、transfer 的 `adapter/http` 测试；`TestTheStartsRefuseAnUpgradingNotebook`、`TestThePrechecksRefuseAnUpgradingNotebook`、`TestTransferNotebooksConvertTheState`（`54ce782f`）。
  - 整个程序的测试：`TestUpgradeFreezeReachesEveryPath`（三种状态 × 14 个 REST 写 × bob 与发起人；5 个 MCP 写；只有预检答的四种情形与升级结束之后的原码；"任务的批"；去掉冻结）、`TestUpgradeFreezeLeavesTheRest`、`TestUpgradeFreezeAnswersFirst`、`TestUpgradeFreezeFollowsTheInitiatorsRole`、`TestUpgradeFreezeLetsOnlyItsOwnWrites`；`events.yaml` 的两处说明；删除护栏的读的成本守卫 `TestUpgradeProtectedReadsInTimeAtTheUnitsBound`（`test-go-costs-2` 一行）（`a28064e9`）。
  - 每个提交单独在一个干净的工作树里 build、vet、`make lint-go`、`make gen-check` 与 short 测试都通过；碰契约或网页的另过 `make lint-web`、`make knip` 与 vitest（189 个文件，4,087–4,088 个测试）。第 6 个提交的 vitest 第一次在负载约 24 下有两个计时的测试（编辑器的 20 万字符的段落解析）超界，单独重跑通过；第 2 个提交的成本夹具修正之后，它另单独重跑 build、vet、`make lint-go` 与 short。推送之前分支改写过两次：oxfmt 的四处并回第 3、4、6 个提交，M10 的 lint 迁移的成本夹具的修正并回第 2 个。审查 C 另在第 2、4、6 个提交上跑同样的门禁与网页的三步，在第 2–6 个提交上跑竞态下的数据库测试与组合根里 S1 加或改的测试，都通过。
  - 修复一轮另有 4 个（下面），一共 11 个；合并 `27b30c7f`。
- 与设计不同之处（实施时定下的，都是最小的改动；已改在各节，3.25 列出）：
  1. `checkedRows` 原来只有一本 LLM Wiki：加一本升级中的普通笔记本（`…019`，升级 `…020`）与三行 `upgrade_nodes`（基线页 `…021`、基线附件 `…022`、保护行 `…023`），另给变更集设上 `upgrade_id`（3.2）。
  2. `TestTheUpgradeColumnsGoDown` 依次 `downTo(56)`、`downTo(55)`：`downTo` 退掉给定的那一版，设计写的 55、54 差一（3.2；审查 B 核对）。
  3. 设计的反例"id 有、另三列空"同时违反三个合取：换成每个只违反一个合取的反例，写成 `=` 的合取两个方向各一个，`notebooks_upgrade_check` 8 个、`upgrade_nodes_*` 19 个（3.2；第 64–66、80、81 条变异各被点名的那一个杀死）。
  4. 名称表要 `upgrade_nodes_pkey iu` 与 `upgrade_nodes_pkey p` 两项（主键的索引与约束）（3.2）。
  5. notebook 根的 `Upgrade`、`UpgradeState` 与三个值的别名在 `notebook/notebooks.go`，挨着 `State`、`Mode`；`ActionChangeMode` 在 `module.go`（3.3）。
  6. 从 `ModeGuard` 抽出的 `decideAction(auth, s, action)` 由冻结的守卫与 `unit_revert.go` 共用，撤销不再只为判定建一个 `ModeGuard`（3.4；审查 A：与原来等价）。
  7. 第 4 个提交里根的 `page.NewFreezeGuard(pool, auth, admin)` 还不用连接池，第 5 个接上 `UpgradeProtected` 的读者；签名一直是设计的。
  8. page 的三个预检共用 `Writer.notUpgrading`（读一次不加锁的状态，有升级答 `notebook.upgrading`，没有笔记本答 `notebook.not_found`）：后台撤销的开始路径读两次状态（M10 的 `TestARevertsChecksRefuseTheContract` 改为两次）；上传的 `Check` 恒读一次，以前名称的检查先拒时一次也不读（`TestAnUploadsCheckRefusesAReservedTitle` 改为恒一次）；导入的 `Check` 的调用序列加 `Notebooks.StateOf`（`TestAnImportStarts`）（3.4、3.5；审查 A：M10 已有的码之间的相对次序不变，只多一次主键读）。
  9. `CreateChangeset` 写 `upgrade_id` 的适配器测试 `TestCreateChangesetWritesItsUpgrade` 在 `page/adapter/postgres/upgrade_test.go`（3.4）。
  10. 第 14 条变异的第二个测试 `TestUpgradeGuardrails` 的 `batch` 一行看不到拒绝标在哪个节点（MCP 的工具错误只带 `code`、`message`、`op`），只有 `TestTheFreezeGuard` 抓到它（审查 S1C-M1，决定 237；7.3 第 14 条改）。
  11. `TestPageNotebooksConvertTheState` 加升级的一行；另加 `TestTransferNotebooksConvertTheState`（组合根的 `transferNotebooks`：没有、升级中、回滚中、没有笔记本、错误，两种读）（3.5）。
  12. 第 6 个提交的范围写 `server, api`：`x-problem-codes` 不进 `schema.gen.ts`，`notebook.upgrading` 的两种语言的文字在第 4 个提交已加，网页没有文件改动；`problem-messages.test.ts` 读契约，照过（6.1）。
  13. 另加 `page/app` 的 `TestThePrechecksRefuseAnUpgradingNotebook`：上传、后台恢复、后台撤销的预检在判定之后先答 `notebook.upgrading`、之后什么也不读、撤销不 admit，读者仍是 `forbidden`。设计只靠整个程序的测试盖这三处，单元一层另钉一次（3.4；它抓到审查 A 的 A2、A4）。
  14. transfer 的 `adapter/http` 的替身 `notebooks` 带一个原子的 `upgrading`：恢复与撤销的 `notebook.upgrading` 由 transfer 自己在锁下答，导入的由 `StateOf` 答，三条都经真的用例；asset 的那条由 page 的检查的替身答（3.5；它让审查 A 的 A3 被抓到）。
  15. oxfmt 报出第 3、4、6 个提交里四个文件的格式（`notebook.yaml` 的流式列表、`page.yaml` 与 `transfer.yaml` 的行内码表、`en.ts` 的一行）：第 1–6 个提交时只跑了 Go 一侧与 tsc、knip、vitest，没跑 `make lint-web`，第 7 个之前才发现，以 fixup 并回各自的提交（分支还没推），打包的 `api/dist/openapi.yaml` 不变。
  16. `TestUpgradeFreezeReachesEveryPath`：三种状态在一个应用里（每种一本 `frozenNotebook`：一页在第 2 版、一页改过名、一个回收站项），bob 与发起人（经 REST）的 14 个写各答 409 `notebook.upgrading`、行与任务数不变；bob 的 agent 的 5 个 MCP 写被拒；发起人的 agent 只在 `upgrading` 放过。"only the prechecks answer"（存储一点余地也不留、一个已过恢复期、清理器还没取走的回收站项、`MaxJobItems = 1`；升级中四个各答 `notebook.upgrading`，`endUpgrade` 之后各答原来的码）、"the jobs' batches"（`transfer.Batch{Nodes: 1}`，`upgradeBetweenBatches` 在任务的第二次调用之前以 SQL 设上升级；导入、恢复、撤销各以 `failed`、`refused`、`notebook.upgrading`、`done = 1` 结束）、"without the freeze"（从 `Guards` 与 `EditSessionVetoers` 各删去恰好一个 `*page.FreezeGuard`，之后九个经单元的写、五个工具与开会话都成功）三个子测试各起自己的应用。
  17. `TestUpgradeFreezeLeavesTheRest` 另核对读页与"字节不变的写答成功、什么也不写"（4.5 的"不冻结"）；导出的开始不经 `background()`（它要求变更集），另写开始与等结束。
  18. `TestUpgradeFreezeAnswersFirst` 另核对接管的开启也先答 `notebook.upgrading`（否决者里冻结最先）。
  19. `TestUpgradeFreezeLetsOnlyItsOwnWrites` 跑 `upgrading` 与 `rolling_back` 两种状态，另含后台撤销的开始与发起人经 REST 撤销她的 agent 为升级写的变更集。
  20. 删除护栏的读的成本守卫写在适配器一层（`page/adapter/postgres` 的 `TestUpgradeProtectedReadsInTimeAtTheUnitsBound`，`//go:build !race`，`-short` 下随 pgtest 跳过）：1 万个 id（一半是基线的）对着两次升级各 10 万行，一个连接上四种状态；界照 7.1 的表取单元的 2 秒。审查发现它抓不到顺序扫描（S1B-I1 = S1C-M2），修复一轮改为以计划为主、界 25 ms（决定 236；7.1 改）。
  21. 夹具另加 `upgradeIn`（一个事务，交回错误，供任务的 goroutine 用，不调 `t.Fatal`）与 `endUpgrade`（清四列、删行）。
  22. `events.yaml` 的 `mode` 的说明用设计的新句子，其后"Its data is the notebook's ids. Read the notebook again…"一句保留，加"its upgrade"（3.7）。
  23. 第 68 条变异改在 `unit.go`：守卫看不到 `Background`，`spec.Background` 的单元的 `Write.Notebook` 去掉升级（审查 S1C-N7；7.3 第 68 条改）。
  24. 设计没有列 M10 的 lint 迁移的成本夹具（`bootstrap/lint_index_migration_cost_test.go` 的 `lintMigrationDropIngest`，`test-go-costs-lint` 经它把库退到 52）：它认定最新的迁移是 54，S1 加了 55、56，第二次 `make test` 在 `costs-lint` 失败；改为先经真的 Down 退 56、55，并回第 2 个提交。这一处只在不带竞态的 `costs-lint` 里跑，逐个提交的核对与组合根的七份都碰不到它。修复一轮改为从嵌入的最新版本退（审查 S1B-M1，决定 238），以后的迁移不用改它（6.1）。
- 实测（本机，经 `heavy.sh`）：
  - 新的测试在竞态下（`go test -race -count=1 -v`，逐包）：第七份的七个共 13.0 秒（`TestUpgradeFreezeReachesEveryPath` 5.56、`TestUpgradeFreezeLeavesTheRest` 2.53、`TestUpgradeFreezeFollowsTheInitiatorsRole` 1.37、`TestUpgradeFreezeLetsOnlyItsOwnWrites` 0.95、`TestUpgradeGuardrails` 0.91、`TestUpgradeToolsErrorsCarryTheirCodes` 0.90、`TestUpgradeFreezeAnswersFirst` 0.80），包 14.2 秒；`TestChecksRejectCounterexamples` 5.53 秒；其余新的单元与适配器测试各在 1.4 秒之内（含起容器）。
  - 删除护栏的读的成本守卫（不带竞态，五次读的中位数）：建成时没有统计、分析过、只有另一次升级的统计、通用计划 4.0–6.8 ms，准备 0.8–0.9 秒，整个测试 2.5–2.7 秒；修复一轮加了计划的断言之后，本机两轮 7.5、6.1、6.3、5.1 ms 与 6.5、5.8、5.3、6.4 ms，界取较大的 7.5 ms 的三倍、向上取整到 5 ms，即 25 ms；两次 CI 的 costs-2 5.7–8.6 ms，在界内。
  - e2e（不带竞态）：建成时 `make e2e` 503 个 148 秒（Playwright 2.4 分钟）；审查 C 503 个（2.3 分钟）；修复一轮 503 个（2.6 分钟）。S1 不加 e2e。
- 反向对照（每一条的结果见 [S1 的审查记录](reviews/P1-S1-freeze-guardrails-review.md)"反向对照"）：
  - 7.3 的 S1 各行（1–20、64–66、68、80、81a–d，设计列两个测试的各跑一次，共 37 次）：36 次被点名的测试抓到；第 14 条的 `TestUpgradeGuardrails` 一半看不到被标的节点（MCP 的错误不带节点），同一变异由 `TestTheFreezeGuard` 抓到（决定 237，第 14 条改为只列它）。第 21 条照设计以 `go test -list` 核对：第三份的模式单独收下 7 个 `TestUpgrade…`，七份合起来每个恰在一份。
  - 审查者的变异里存活的（A 的 A1、A5、A7，B 的 c1、c2，C 的 c1、c2）各成了一条发现，修复一轮之后都被抓到；连同修复一轮自己的 p1、n2、n3 与 C 的 o1、o4、o7、o9 写成 7.3 的第 83–94 条。A 的 W6（同第 14 条）与 C 的 o2（S1C-N1，S3 补）照预料存活。修复一轮在 `68c3c505` 上重跑 49 条（实施的 37 条与 12 条），46 条抓到，存活的正是这三条照预料的。
- 门禁（本机）：建成时 `make lint gen-check knip build` 通过（16 秒）；经 `heavy.sh`：`make test` 第一次在第五份有两个计时的测试超界（逐个提交的核对与它同时跑，负载 24；单独重跑通过），第二次（单独跑，1,942 秒）竞态、组合根七份与成本三份通过、在 `test-go-costs-lint` 发现 M10 的 lint 迁移的成本夹具认定最新的迁移是 54（上面第 24 条），之后 `make -j1 test-go-costs-lint test-go-guards test-web` 在修正后的头上通过（1,547 秒，vitest 189 个文件、4,088 个测试）；`make test-go-short` 46 秒；`make e2e` 503 个 148 秒。修复一轮（`68c3c505`）：4 个提交各自 build、vet、`make lint-go`、`make gen-check` 与 short 都通过（碰 `Makefile` 的两个另过 `make lint`，夹具的那个另过 `make test-go-costs-lint` 与一个空的 00057）；头上 `make lint`、`gen-check`、`knip`、`build` 通过，`make test-go-short` 通过，`make e2e` 503 个；`make test` 第一次只在 guards-core 的 `TestTheAppsMarkdownCostsAboutItsSize/to_a_page` 以 0.02 ms 超界（`platform/markdown` 最后改在 M10/P4/S2，单独重跑通过），第二次全绿。
- 组合根的各份（3.8，竞态构建的口径）：`go test -race -list` 534 个顶层测试，七份 158、51、121、123、62、12、7，各在恰好一份，`TestUpgrade…` 只在第七份；第三份的 `-run` 模式单独收下全部 7 个 `TestUpgrade…`（7.3 第 21 条）。另有 42 个 `!race` 的成本与守卫测试不进竞态构建（实施报告的 576 个、167/58/124/136/62/22/7 是不带竞态的清单，审查 S1C-N2）。本机单独两轮（包的时间）：第一份 298.8、290.8 秒，第二份 279.5、283.2，第三份 266.2、278.9，第四份 220.2、227.4，第五份 259.1、259.9，第六份 264.9、261.4，第七份 14.5、14.7。
- CI：分支的 run 42491802403（`a28064e9`）全绿（作业，括号里是测试一步）：竞态 13:29（13:09）、组合根一 5:00（4:42）、二 6:11（5:49）、三 5:04（4:45）、四 7:24（7:02）、五 6:48（6:25）、六 4:27（4:10）、七 1:02（0:42）、成本一 17:00（13:38）、二 16:12（15:51）、三 10:23（10:01）、lint 的成本 11:22（11:04）、守卫 core 19:31（19:11）、sources 16:57（16:33）、short 2:37（2:19）、web 4:20（3:48）、e2e 7:30（E2E 一步 5:58）、镜像 1:25。修复一轮的 run 42512020813（`68c3c505`）全绿：竞态 13:23（13:04）、组合根一 7:02（6:45）、二 4:46（4:28）、三 7:18（6:54）、四 7:20（7:01）、五 6:46（6:25）、六 5:04（4:48）、七 1:21（0:57）、成本一 17:12（13:40）、二 12:42（12:23）、三 8:00（7:39）、lint 的成本 13:12（12:49）、守卫 core 12:00（11:41）、sources 17:24（17:02）、short 3:19（2:57）、web 4:51（4:17）、e2e 6:45（E2E 一步 5:34）、镜像 1:23。组合根七份的测试一步都在 450 秒之下，最紧的是第四份（422、421 秒）；第 8 节第 1 条要的合并之后两次 `main` 的平均，合并之后量。
- 设计的文字照建成的与修复一轮改，已改在各节（3.2、3.3、3.4、3.5、3.8、3.14、3.24、6.1、6.3、7.1、7.3、7.5；3.25 列出）；7.3 加第 83–94 条。00 号文档 4.6 与第 9 节的成本守卫一行随之改。
- **修复一轮**（2026-11-09，分支 `m11-p1-s1` 的 `68c3c505`；三份审查与主持者的修复说明 `fix-1.md`，决定 236–239）：从 `a28064e9` 起 4 个提交，一组一个，普通推送，没有改写已推的历史：
  - 重要的与次要的：
    - S1B-I1 = S1C-M2 `f66537d4`：删除护栏的读的成本守卫钉住主键的列，四种统计 × 自定义与通用两种计划都断言走主键、两列都进索引条件、没有顺序扫描；界 25 ms（决定 236）。
    - S1B-M1、S1C-N5 `b7f242e7`：M10 的 lint 迁移的成本夹具从嵌入的最新迁移逐个经真的 Down 退到 54（决定 238）；另以一个空的 00057 验证过。
    - S1A-M1、S1A-N1 `e5aa8a67`：导入的 `Check` 以父节点不存在的请求断言答 `notebook.upgrading` 与完整的调用序列；"读状态时笔记本已删"答 `notebook.not_found`；恢复与撤销的开始由读者发起时答 `forbidden`。
  - 小问题 `68c3c505`：冻结的守卫对不认识的判定答缺陷的错误、整个单元回滚（S1A-N2，这一轮唯一的生产的改动；`FreezeGuard` 加测试的缝 `admits` 与 `WithAdmits`）；`Grants` 与 `TestUpgradeFreezeAnswersFirst` 的注释（S1A-N3、N4）；`Makefile` 的分份的数写竞态的 534 个（S1C-N2）；`TestPageNotebooksConvertTheState` 的期望写 page 的常量，page 的模块根导出三个状态常量（S1C-N3）；`06-environment.md` 的 `-j6` 与"逐个提交的核对也经 `heavy.sh`"一句（S1C-N4、N6）。
  - 只改文档的在本 Step 的文档提交：7.3 第 14 条只列 `TestTheFreezeGuard`，3.4 与 00 号文档 4.6 照实（S1A-Q1 = S1C-M1，决定 237）；3.14 第 10 步记 `Start` 断言升级的 id 非零（S1B-N1，S2 做）；S3 的"任务的批"加 `rolling_back` 一行（S1C-N1）；`NotebookUpgrade` 的说明在 S2 之前引用 `getUpgrade`，接受（S1B-N2）；7.3 第 68 条的写法与第 83–94 条（S1C-N7 与修复一轮的变异）。
  - 实施者在这一轮的一次失误：第一次跑 `costs-lint` 时在同一个工作树里接着改后面的提交，那次编进了半成品、作废，以干净的工作树里的那次为准。
- **审查的结论**：Important 1（S1B-I1 = S1C-M2），Minor 另 3（S1A-M1、S1B-M1、S1C-M1 = S1A-Q1），Nit 13（S1C-N5 并入 S1B-M1），都已改或写明交给 S2、S3。三位都没有找到答错的结果：A 的交错探针在四条路上逼不出"读了没升级、写照样落下"的窗口；B 在真库上核对 27 个反例各恰好违反一个合取、14 个操作的码表与设计一致、读者与看不到的人先答 403 与 404；C 的逐个提交、竞态下的中间提交与头上的 e2e 都通过。实施者的 24 条偏离由三位判过，都对（第 20 条照了设计的表，表本身错了）。
- **修复核对一轮**（2026-11-09，三位各在 `68c3c505` 的自己的工作树里）：三位都没有新的发现（Important 0、Minor 0、Nit 0）。要改代码的旧发现都关了：A 的 S1A-M1、N1–N4、Q2，B 的 S1B-I1、M1，C 的 S1C-M2、N2–N6；只改文档的处置对，文档的部分在本提交（S1A-Q1 = S1C-M1、S1C-N7、S1B-N1、S1B-N2），S1C-N1 交给 S3。A 重跑 A1、A5、A7，新加 F1–F5（`default` 的三种写坏、构造器、`goneAsRead` 的开关），都被抓到；B 的 c1、c2 与新的 c3 被杀，q3、q4（只留位图、套一层 Gather）照过、不误报，q1、q2（关掉位图）让"分析过、自定义计划"一格改选并行的顺序扫描，一个空的 00057 下两个 lint 迁移的测试通过；C 在 4 个提交上各自跑门禁都通过，重跑 32 个变异，31 个抓到、o2 照预料存活，实施者报告的变异表、CI 与中位数与它读到的逐项一致。B 记下一项观察：那一格位图读与并行顺序扫描的估计成本只差约 9%，代价模型一变，测试会以整表扫描失败；主持者接受，这正是决定 236 要的信号，若 CI 上因此不稳再议。不需要第二轮。
- 实施者留下的六项：Step 的文档提交是本提交；第 14 条的 `TestUpgradeGuardrails` 一半见决定 237；"任务的批"里任务由发起人开，不改（第 68 条与 9b 在那里各自失败）；本机计时的守卫在高负载下超界，见审查 S1C-N6；成本守卫的界见决定 236；lint 迁移的夹具见决定 238。

### 10.2 S2：发起与读（服务端）（2026-11-09，合并 `（待填）`）

- 提交（从 main 的 `5795229c` 开始，照 6.2 的次序，7 个；Step 的文档提交是主持者的）：
  - wiki 模块的第一张表：迁移 00057 的 `notebook_upgrades`（14 个约束与"一本笔记本至多一个进行中"的部分唯一索引）、wiki 的 sqlc 一项与 `wiki/adapter/postgres`、授权一行、wiki 的清理器排在 notebook 之前、notebook 与 page 两个模块的笔记本删除订阅者（删笔记本时一并软删除升级的行、删掉 `upgrade_nodes`）（`1423abec`）。
  - notebook 的 `UpgradeStates`：在调用方的事务里锁笔记本行、写四列（断言升级的 id 非零，S1B-N1）、把模式的变化交给降级的订阅者（`26f6762b`）。
  - page 的升级端口：`CaptureBaseline`、`ProtectSourceRoot`、`SetBaselineWanted`、`AnalyzeUpgradeNodes`、`RootNodesByKey`，按笔记本删过期的会话与读活着的会话；`TreeWrites().Seed` 的 `SeedSpec.Upgrade`（`35c290fb`）。
  - lint：`computeIn` 在调用方的事务里算、`prepareIn` 只准备不投影；`Runs.TryHold` 立即持名额或不持；`RuleQuery.WantedByPage` 与第二遍的 `LintAreaWantedEdgeHeads`（按 512 页一段、行比较的游标，待建的过滤在 `LIMIT 1` 的 `LATERAL` 里）（`4488279f`）。
  - `startUpgrade` 与 `getUpgrade`：契约的两个操作、六个 schema 与十个码的声明，`wiki/adapter/http` 答出每个码；组合根的接线；`checkPages` 的七条升级规则（库里有这张表时）与 `TestPageViolationsCatchEachUpgradeGap`；矩阵的三行与 `TestUpgradePermissionMatrix`；`TestUpgradeStart`、`TestUpgradeStates`、`TestUpgradeStatesReachTheStreams`、`TestUpgradeDeletion`、交错（7.2 第 1–4、13、14、17 条）；运行时角色的发起；网页的五个码的两种语言的文字（`0778f5e2`）。
  - 发起的成本守卫 `TestUpgradeStartTakesItsTimeAtAnyStatistics`（`test-go-costs-2` 一行）；发起的期限改为请求的四分之三（第 232 条）（`1c4d35c0`）。这个提交的标题只说成本守卫，没写生产上发起的期限从 `LintTimeout` 的 7.5 秒变成 11.25 秒，范围只写 `server` 而改了 `ci.yml` 的一行注释（审查 S2C-N5；已推的历史不改写，记在这里与合并的说明里）。
  - e2e：UP1 的 API 与 MCP、UP2 的 API、UP4 的删除一步；夹具 `upgrades.ts`、`assert/wiki.ts`，`expectNotebook` 的四列、`expectBaseline` 与变更集的 `upgrade_id`（`c5a81c7a`）。
  - 每个提交单独在一个干净的工作树里 build、vet、`make lint-go`、`make gen-check` 与 short 测试都通过；碰契约、网页或 e2e 的另过 `make lint-web`、`make knip` 与 vitest（189 个文件，4,095 个测试）。推送之前改写过两次：逐个提交的核对发现第 5 个的 `TestLintMatrixTargets` 没读升级的行、第 6、7 个的 errorlint；第二次 `make test` 发现 M10 的迁移夹具在 53 的库上调 `checkPages`（下面第 6 条）。审查 C 另在 7 个提交上跑同样的门禁与竞态下各自碰到的包，都通过。
  - 修复一轮另有 4 个、第二轮 1 个（下面），一共 12 个；合并 `（待填）`。
- 与设计不同之处（实施时定下的，都是最小的改动；已改在各节，3.26 列出）：
  1. `LintAreaWantedEdgeHeads` 的待建过滤写在 `LIMIT 1` 的 `LATERAL` 里：没有统计时 `IS NULL` 估 0.005，自定义计划选了位图扫描加 Sort（3.13）。
  2. `WantedByPage` 只配 `wanted` 一条规则，别的组合答 `errLintQuery`；第二遍在模式是 LLM Wiki 时跑（3.13）。
  3. `wiki.Deps` 加 `Pool`（自己的表的适配器）（3.15）。
  4. `transfer.busy` 的文字末句是"…start the upgrade once it is done."（3.14）。
  5. 矩阵的行用 lab 的笔记本，每格在库的副本上以 SQL 准备：发起的一行先删那本笔记本的编辑会话，`getUpgrade` 的两行以 `upgradeIn` 由笔记本的创建者设成 `upgrading`、`reviewing`；每格之后只核对 `pageViolations` 里升级的几句；两处覆盖的检查经 `coveredMatrixRows()` 读它们（3.22）。
  6. `checkPages` 的违反的测试名为 `TestPageViolationsCatchEachUpgradeGap`（第四份）；升级的规则只在库里有 `notebook_upgrades` 时加（3.15）。
  7. 名额占满的 503 只在 7.2 第 17 条与 `wiki/app` 测（审查之后另有"名额占满时各码照答"的一组，S2A-M1）。
  8. 第 31 条变异的测试是 `TestComputeInTakesItsMoment`：S2 的发起不经 `computeIn`（7.3 第 31 条改）。
  9. `wiki/app` 的 `TestUpgradeStart` 在 `app_test` 包里，替身 `startFake` 盖住每个端口、记下调用的次序。
  10. 夹具 `upgradeIn` 选根（根下的 `raw` 页优先，否则最早的根页）、按状态写一致的 wiki 行，`endUpgrade` 把 wiki 的行记为 `rolled_back`。
  11. S1 的测试照 `checkPages` 的新规则改 wiki 的行或加 raw。
  12. 7.2 第 3 条：发起删了过期的会话，之后的心跳答 `page.edit_session_not_found`。
  13. 流里先是补写的 `pages`，再是 `mode`。
  14. 运行时角色的测试先以 owner 的 SQL 加一个笔记本管理员，断言 `upgrade_nodes` 被分析过、日志里没有"upgrade baseline not analyzed"。
  15. `GetUpgrade` 的回滚几列直接从行读（S2 时都是 null）。
  16. 发起的期限是请求的四分之三（守卫 4.5 秒、生产 11.25 秒、默认 4 秒的请求下 3 秒）；守卫是 5 万页、1.5 万页在 raw 里；10 万页在最慢的 runner 上过 4.5 秒，进 M12 的移交（第 232 条；3.14、7.1 改）。
  17. 守卫在发起的事务里核对 `SetBaselineWanted` 的自定义与通用计划；"本次之后"读作分析过这本笔记本的上一次发起；每次发起之后 `VACUUM`；`ci.yml` 的 costs-2 加一行注释（36 条，时限不变）。
  18. e2e 的夹具名与 `upgradesOf`；UP1 的"别人"是编辑者（REST）与另一位管理员（MCP）；UP2 末尾再发起一次答 200（第 5 节）。
  19. 第 62 条变异四种写法，7.3 第 62 条改列 `TestTheAreaPassReadsByChunk`、`TestTheAreaWantedHeadsGoByBatch/refused` 与计划的测试，成本守卫只对三者合在一起的那种。
  20. 第 18 条变异的 S2 一半由 7.2 第 4 条抓到。
- 实测（本机，经 `heavy.sh`；每一项的出处见 [S2 的审查记录](reviews/P1-S2-start-and-read-review.md)"实测"）：
  - 新的测试在竞态下：第七份的六个共 9.5 秒（`TestUpgradeStart` 2.27、`TestUpgradeStatesReachTheStreams` 2.13、`TestUpgradeInterleavings` 1.91、`TestUpgradePermissionMatrix` 1.48、`TestUpgradeStates` 0.94、`TestUpgradeDeletion` 0.78 秒）；`TestPageViolationsCatchEachUpgradeGap` 2.34 秒；`TestTheUpgradesCodesAreAnswered` 7.87 秒；其余各在 1.7 秒之内。
  - 发起的成本守卫：10 万页本机两轮中位数 2.16–2.41 秒，守卫配置下 3 秒的期限只余约 20%，折到 CI 2.5–5.6 秒，照第 232 条改期限、减规模；5 万页建成时两轮 1.04–1.15 秒、界 3.5 秒，CI（9V74）1.57–1.62 秒；修复一轮另种九本笔记本之后两轮 1.11–1.17 秒、界 3.6 秒，种子 10.2 秒（这本笔记本的 1.4 秒），CI（8370C）2.13–2.22 秒。计划：六种状态与模式都经主键或一次读完，没有顺序扫描；修复一轮之后 `CaptureBaseline` 在六格里都是逐个经 `page_contents_pkey` 读正文的 Nested Loop。第二遍在审查者 B 的数据上 0.34–0.40 秒（B 量的整组一次传入 18.1、约 150、66.2 秒）；`SetBaselineWanted` 10 万页 162–211 毫秒（B 386–671 毫秒）。修复第二轮之后本机 `make test-go-costs-2` 里中位数 1.34–1.42 秒（种子 11.9 秒），CI（9V74）1.69–1.74 秒，界 3.6 秒。
  - e2e：`make e2e` 507 个（建成 2.2–2.4 分钟，修复一轮 130 秒）。
- 反向对照（每一条的结果见 S2 的审查记录"反向对照"）：
  - 7.3 的 S2 各行（第 18 条的 S2 一半、22–36、62、63、67、82）共 25 种改坏，都被点名的测试抓到；第 24 条建成时靠测试二进制的期限（修复一轮之后第 17 条交错 2 秒内失败）；第 62 条成本守卫只抓得到三者合在一起的那种（62c），另三种由分段、适配器与计划的测试抓到。
  - 审查者的变异里存活的（A 的 W1、W3、W4、W6、L4、L5、L7、L8、L10、WA2 与组合根的 B4–B6，B 的 g1–g7、c2、cp-all，C 的 d26b、o24、o2b、o3）各成了一条发现，修复一轮之后都被抓到；连同修复一轮自己的 n1、f240a、f240b，B 的 c1、c3，C 的 62e 写成 7.3 的第 95–109 条，第 24、26、31、62 条照实改。照预料存活的：A 的 N2、P3、B8 与 B 的 g9、g10（等价或被蕴含，决定 241），d26b 的 `wiki/app` 一半，cp-notebook。修复核对一轮另加的 B 的 rk、ds、ls 与 C 的 x1 都被守卫的计划抓到。修复第二轮：`startCostPageScans` 去掉两项新要求之一（r1、r2）各被 `TestTheStartGuardJudgesThePageReads` 的一格抓到（7.3 第 110 条），B 的 c2 与 f240a、f240b 照旧被守卫抓到。
- 门禁（本机）：建成时头 `c5a81c7a` 上 `make lint gen-check knip build` 通过；经 `heavy.sh`：`make test` 3,473 秒通过、`make test-go-short` 47 秒、`make e2e` 507 个 148 秒。此前两次 `make test`（`b2c65a37`）：第一次在竞态六份同时跑时第一份的两个交错的计时测试超时（与 S2 无关，单独两轮通过）；第二次发现 M10 的迁移夹具在 53 的库上调 `checkPages`（上面第 6 条，并回第 5 个）。修复一轮（`42529055`）：4 个提交各自 build、vet、`make lint-go`、`make gen-check` 与 short 都通过；头上 `make lint gen-check knip build` 通过，`make test` 第一次在第一份的 `TestABackgroundJobAndItsNotebooksDeletion/revert` 失败（与 S2 无关，单独三次通过），第二次 3,324 秒通过，`make test-go-short` 40 秒，`make e2e` 507 个 130 秒。修复第二轮照决定 242：提交单独为绿，`make lint gen-check knip build` 通过，`make test-go-costs-2` 第一次因本机根分区满失败（Go 的构建缓存涨到 59 G，修剪之后剩 44 G；`06-environment.md` 第 3 节加一句），第二次 608 秒通过，`make test-go-short` 44 秒。
- 组合根的各份（竞态构建的口径）：`go test -race -list` 541 个顶层，七份 158、51、121、124、62、11、14，各在恰好一份；S2 加的七个里六个 `TestUpgrade…` 在第七份，`TestPageViolationsCatchEachUpgradeGap` 在第四份，`!race` 的成本守卫不进任何一份。本机单独两轮（`b2c65a37`，包的时间）：第一份 288.3、286.4 秒，第二份 262.7、243.2，第三份 267.3、249.8，第四份 223.9、208.5，第五份 263.0、245.8，第六份 184.5、187.0，第七份 95.6、89.6（S2 之前 82.1、84.1）；修复一轮之后第七份 90.3、93.4 秒。分份调整合并之后 `main` 的第一次运行 42530568840（`5795229c`）的七份测试一步是 288、399、348、284、392、228、137 秒（审查 C 读的）；S2 合并之后两次 `main` 的平均：`（待填：两次 main 的七份与平均）`。修复第二轮的 `TestTheStartGuardJudgesThePageReads` 在 `!race` 的守卫文件里，不进任何一份，跑在 short 一行。
- CI：分支的 run 42554470668（`c5a81c7a`）全绿，七份的测试一步 357、431、382、431、395、245、149 秒（第二、四份在 EPYC 7763 上），costs-2 16:13（15:49），守卫在 9V74 上 38.6 秒；修复一轮的 run 42571990070（`42529055`）全绿，七份 288、350、342、410、393、298、151 秒，costs-2 在 8370C 上 20:39（20:06），守卫 70.1 秒（种子 22.1 秒）。都在 450 秒之下。costs-2 在 7763 上：`main` 的 42530568840 已是 22.2 分钟，加上发起的守卫约 23.5 分钟，对 35 分钟的作业时限不到 `ci.yml` 注释说的约两倍（审查 C；S2 只多约 1.2 分钟），合并之后看 `main` 上的数。修复第二轮的 run 42579807531（`9b3000c1`）全绿，七份 444、429、262、422、393、304、119 秒（第一份离线 6 秒，第二轮没有碰任何一份的测试），costs-2 在 9V74 上 16:39（16:12），守卫 55.7 秒。
- **修复一轮**（2026-11-09，分支 `m11-p1-s2` 的 `42529055`；三份审查与主持者的修复说明 `fix-1.md`，决定 240、241）：从 `c5a81c7a` 起 4 个提交，一组一个，普通推送，没有改写已推的历史（提交 3 推送之前为 gochecknoglobals 重写过一次）：
  - `219d6f28`：组合根的 `its own deadline` 分成 `inside a statement`（测试持住笔记本行，发起在 `States.Lock` 的语句之中到期：503、恰一行 warn、没有 error、什么也没写）与 `between statements` 两格（S2C-I1）；`wiki/app` 的替身加 `rollbackLate` 与 `commitErr`：领域的拒绝在回滚越过期限时照答 409，`COMMIT` 自己超时时不答 503（S2C-M1、S2A-N1）；替身在 `Capture`、准备、提交与 `Analyze` 四处探名额，`each-refusal-in-its-turn` 每格另在名额占满时跑一次（S2A-M1 = S2C-N3）；7.2 第 17 条在另一个 goroutine 里发起、限时 2 秒（S2C-N2）。
  - `99bf2d54`：第二遍的夹具带重复的目标、`./Alpha`、图片与虚拟的 `index`，两遍判得一样（S2A-M2）；替身的 `areaStray`、`areaResolved` 两个开关（S2A-N2）；`TestComputeInTakesItsMoment` 用会投影的查询（S2C-N1）。
  - `0da54746`：`CaptureBaseline` 改为 `LEFT JOIN LATERAL (… LIMIT 1)`，这一轮唯一的生产的改动；守卫另种九本笔记本，核对 `CaptureBaseline`、`RootNodesByKey` 与两条会话的语句的两种计划，界 3.6 秒（S2B-M2，决定 240）；守卫的笔记本名与日志前缀、一处注释（S2C-N4）。
  - `42529055`：00057 的五条反例与部分唯一索引的两条（S2B-M1、S2B-N1）；被蕴含的三个合取的注释（S2B-N2，决定 241）；`checkPages` 的升级规则补三行、每行只报自己的（S2B-N3）；`TestTheUpgradesRows` 的 `rolling_back`（S2A-N3）。
  - 只改文档的在本 Step 的文档提交：S2C-N5；设计 3.10、3.12–3.15、3.22、7.1–7.3、7.5、7.6；00 号文档第 232 条、第 9 节与 M12 的移交。
- **修复核对一轮**（2026-11-09，三位各在 `42529055` 的自己的工作树里）：旧发现都关了（只改文档的部分在本提交）。A 重跑留下的 10 个变异都被抓到；B 的 g1–g7、c2、cp-all 都被杀，新加的 rk、ds、ls 被守卫的计划杀，`CaptureBaseline` 在旁边有九本与 200 本笔记本的两种库、两种统计、两种计划下都逐个经主键读；C 在 4 个提交上各自跑门禁都通过，重跑 12 次变异，d26b 的组合根一半、o24、o2b、o3、24a、24b、n1、d31 与新的 x1 都被抓到。新的一条 Nit：S2A-N4（守卫对 `page_contents` 的核对只看索引名，Merge Join 之下没有条件的主键扫描也能过；B、C 把同一处列为未核实的怀疑）。B 指出实施者在 `42529055` 的提交说明里"cp-notebook 没有一行能单独违反"不对（wiki 行指向一本软删除的笔记本就只有 `upgradeRowGap` 报），那一行仍可省；C 记下 costs-2 在 7763 上的时长（上面"CI"）与第一份负载敏感的交错计时测试（S2 第一次 `make test` 的两个、修复一轮的 `TestABackgroundJobAndItsNotebooksDeletion/revert`），都与 S2 无关，不挡合并。
- **修复第二轮**（2026-11-09，`9b3000c1`；`fix-2.md`，决定 242）：一个提交，只改 `bootstrap/upgrade_start_cost_test.go`：`startCostPageScans` 对 `page_contents` 另要求每个经主键的扫描的 `Index Cond` 含 `node_id`、且在嵌套循环的内侧逐个节点重复（S2A-N4，决定 240 的"逐个经主键读"落到核对上）；新的不连数据库的 `TestTheStartGuardJudgesThePageReads` 喂五种合成的计划，其中两格各只违反一项。本机门禁照决定 242（上面"门禁"），分支 CI 全绿（上面"CI"）。
- **修复核对第二轮**（A，在 `9b3000c1` 上）：S2A-N4 关了，没有新的发现；r1、r2 重跑都被抓到，check1 那种 Merge Join 的合成计划现在被拒。修复核对两轮就结束。
- **审查的结论**：Important 1（S2C-I1），Minor 另 5（S2A-M1 = S2C-N3、S2A-M2、S2B-M1、S2B-M2、S2C-M1），Nit 10（S2C-N3 并入 S2A-M1），修复核对一轮另 1 条 Nit（S2A-N4）；都已改或记下（S2C-N5）。代码本身三位都没有查出错，问题都在"测试钉住了什么"与 `CaptureBaseline` 的计划上：A 的探针在发起与删除笔记本两种先后下都对；B 在真库上核对 31 个反例、契约与矩阵与设计一致；C 的逐个提交、竞态下的中间提交与头上的 e2e 都通过。实施者的 20 条偏离由三位判过，都对。
- 实施者留下的五项：Step 的文档提交是本提交；10 万页的发起进 M12 的性能移交（00 号文档第 232 条与"写给后面的 M"）；第 24 条变异靠测试二进制的期限，见审查 S2C-N2；`make test` 里第一份的交错计时测试，记为已知的负载敏感；S3、S4 照设计。

## 附录：核对过的代码

每个决定在 `9184f67b` 上核对过的地方（读法同头的表）。

| 决定（节） | 核对过的 |
|---|---|
| 迁移与授权（3.2、3.10、3.17） | `00009_notebook_notebooks.sql:5-27`；`00012_page_nodes.sql:5-30`；`00013_page_page_contents.sql:5-20`；`00017_page_edit_sessions.sql:28`；`00021_linking_page_links.sql:25`；`00034_page_changesets_history.sql` 全文；`00039_transfer_background_jobs.sql` 全文；`00042_page_nodes_path_keys.sql:14-15`；`00048_notebook_notebooks_mode.sql:1-18`；`server/migrations/schema_test.go:68`、`:163`、`:191`、`:314-325`、`:463`、`:478-483`、`:544`、`:553`、`:603-605`、`:755-763`；`server/migrations/notebooks_mode_test.go:17-65`；`background_jobs_test.go:15-26`；`server/sqlc.yaml:55-200`；`archtest/sqlc_test.go:18-36`、`:106`、`:113-116`、`:230-277`；`deploy/runtime-grants.sql:14-31`；`bootstrap/runtime_role_test.go:128`、`:330-352`、`:362`、`:385-394`、`:416-424` |
| notebook（3.3、3.11） | `notebook/notebooks.go:26-34`、`:93`、`:104-135`；`notebook/domain/notebook.go:28-47`、`:135`；`notebook/domain/actions.go:20`；`notebook/domain/errors.go:6-33`；`notebook/adapter/postgres/queries/notebooks.sql:1-68`；`cascade.sql:37-45`；`ownerless.sql:1-9`；`store.go:75-105`、`:140-155`；`ownerless.go:15-30`；`cascade.go:57`；`notebook/adapter/http/handler.go:50`、`:97-172`；`notebook/app/view.go:10-14`；`notebook/app/manage.go:26-41`；`notebook/app/extension.go:75-98`；`notebook/app/downgrade_notebook.go:56-111`；`notebook/module.go:61-143`；`api/modules/notebook.yaml:12`、`:35`、`:67`、`:86`、`:256-271`、`:310`、`:452-510`；`api/modules/transfer.yaml:689-691` |
| page 的冻结（3.4） | `page/app/extension.go:27-80`、`:170-177`、`:218-240`；`page/domain/mode.go:15-21`；`page/domain/client.go:44`；`page/domain/errors.go:21`、`:44-50`；`page/domain/refused.go:13-28`；`page/refusal.go:12-28`；`page/mode_guard.go:1-23`；`page/app/mode_guard.go:25-97`；`page/app/grants.go:12-50`；`page/app/unit.go:72-124`、`:191-287`、`:328-340`、`:350-404`、`:452-476`；`page/app/unit_delete.go:15-66`；`page/adapter/postgres/store.go:447-458`；`page/app/unit_session.go:25-60`；`page/app/ports.go:54-76`、`:127-137`；`page/adapter/postgres/queries/changesets.sql:1-9`；`page/app/asset_writes.go:49-81`；`page/app/jobs.go:20-45`、`:64-146`、`:168-192`；`page/app/revert_changeset.go:248-254`；`bootstrap/page_notebooks.go:15-38`；`bootstrap/registrants.go:240-282`、`:311-314` |
| transfer 的开任务（3.5） | `transfer/app/ports.go:28-41`；`transfer/app/start.go:73`、`:100-114`；`transfer/app/start_import.go:60-100`、`:244-285`；`transfer/app/start_background.go:100-198`；`transfer/domain/errors.go:12-36`；`bootstrap/deps.go:115`、`:159-163`、`:197`、`:217-223`、`:235-252` |
| 码（3.6） | `api/modules/page.yaml:64`、`:175`、`:228`、`:475`、`:529`、`:635`、`:682`、`:738`、`:867`、`:1083`；`api/modules/asset.yaml:51`；`api/modules/transfer.yaml:105`、`:175-186`、`:281-298`、`:370-395`、`:912-947`；`bootstrap/permission_matrix_mcp_test.go:89-122`；`bootstrap/mcp_client_test.go:338-340`；`bootstrap/mcp_reads_test.go:61`、`:427`、`:432`、`:473-479` |
| 事件（3.7） | `api/modules/events.yaml:48-51`、`:87`、`:222-233`；`bootstrap/events_registrants.go:47-66`；`bootstrap/registrants.go:175`、`:189-204` |
| 分份（3.8、7.5） | `Makefile:179-196`、`:245-300`；`.github/workflows/ci.yml:20-68`；`docs/v0.1/claude_to_codex/06-environment.md:25-28`、`:47-50` |
| page 的发起（3.12） | `page/adapter/postgres/queries/sessions.sql:23-36`、`:66-73`、`:91-108`；`page/app/heartbeat_edit_session.go:45-73`；`page/app/seed_writes.go:24-62`；`page/tree_writes.go:42`、`:56`、`:135`、`:211-224`；`page/adapter/postgres/queries/sources.sql:15-17`、`:46-54`；`linking/adapter/postgres/queries/statistics.sql:4`；`bootstrap/interleavings_page_test.go:58`、`:83-151`、`:173` |
| lint（3.13） | `wiki/app/lint.go:25-126`、`:196-272`；`wiki/app/lint_runs.go:17-60`、`:60-205`、`:218-322`；`wiki/app/lint_run_types.go:13-46`；`wiki/app/lint_ports.go:15-30`、`:82-138`；`linking/app/lint_edges.go:15-87`；`linking/app/lint_scope.go:29`、`:75-242`；`linking/adapter/postgres/queries/lint.sql:10-42`；`linking/app/lint_reads.go:256-261`；`linking/app/lint_rules.go:72-138`；`bootstrap/wiki_lint.go:20-40`、`:120-156`；`platform/postgres/tx.go:36-44`、`:121-203` |
| 发起与读（3.14、3.15） | `wiki/module.go:52-63`；`wiki/lint.go:72-107`；`wiki/app/template.go:14-21`、`:66-100`；`wiki/domain/template.go:29`、`:50-58`；`wiki/app/ports.go:14-27`；`wiki/adapter/http/handler.go:19-30`；`api/modules/wiki.yaml:7-86`；`api/openapi.yaml:17-38`、`:164-167`；`api/common.yaml:66-82`、`:125-140`、`:190-200`；`shared/error.go:115-125`、`:164`、`:181-183`、`:249`；`linking/app/rewrite.go:54`、`:63-65`、`:775-790`；`linking/domain/errors.go:19`；`transfer/writing_jobs.go:17-28`；`bootstrap/wire.go:180-285`；`bootstrap/display_names.go:13-19`；`bootstrap/purge_test.go:56`、`:71`、`:98` |
| 回滚（3.18、3.19、3.20） | `page/domain/revert.go:78-132`、`:152-186`、`:373`、`:463-464`、`:557-564`；`page/domain/revert_schedule.go:304-342`；`page/domain/changeset.go:17-26`、`:192-217`；`page/domain/change.go:9-13`；`page/domain/jobs.go:29`、`:38-42`；`page/domain/trash.go:46`、`:57`；`page/domain/node.go:23`；`page/app/jobs.go:609-730`、`:872`；`page/app/unit_revert.go:100-104`、`:129-133`、`:149`、`:305-416`、`:424-460`、`:634-636`、`:935-948`；`page/app/ports.go:174-175`、`:544`、`:581-584`；`page/domain/revert_schedule_property_test.go:51`；`page/app/revert_job_property_test.go:64`、`:1031`；`bootstrap/transfer.go:86-179`、`:232-261`、`:295-306`；`transfer/domain/job.go:14-98`；`transfer/app/background.go:50-128`、`:140-236`；`transfer/app/batches.go:20-44`、`:88-100`；`transfer/app/job_run.go:25-33`、`:127-242`；`transfer/app/maintain.go:71-114`；`transfer/app/requests_test.go:339-371`；`transfer/app/cancel.go:45-92`；`transfer/app/views.go:73-103`；`transfer/app/reads.go:48-115`；`transfer/adapter/river/jobs.go:26-92`、`:180-240`；`transfer/adapter/postgres/queries/jobs.sql:4-14`、`:26-35`、`:67-100`、`:120-152`；`transfer/module.go:129-239`；`transfer/app/export.go:394-430`；`linking/app/rewrite.go:63-65` |
| 矩阵（3.22） | `bootstrap/permission_matrix_wiki_test.go:30-45`、`:140`；`bootstrap/permission_matrix_notebook_test.go:166-178`；`bootstrap/permission_matrix_test.go:229-248`；`bootstrap/permission_matrix_coverage_test.go:155`；`bootstrap/permission_matrix_transfer_test.go:237-260` |
| 测试的设施（7.1、7.2） | `bootstrap/wiki_guard_test.go:58-85`、`:102-104`、`:116-195`、`:201-203`；`bootstrap/interleavings_wiki_test.go:304`、`:344-345`；`bootstrap/wire.go:62-89`；`bootstrap/interleavings_workspace_test.go:85`、`:185`；`platform/postgres/pgtest/lockwait.go:25`、`:45`；`bootstrap/links_test.go:25`；`bootstrap/search_test.go:175`；`bootstrap/wiki_lint_cost_test.go:27-34`；`platform/httpserver/apitest/apitest.go:55` |
| 网页（第 4 节） | `W/app/effective-role.ts:40-42` 与第 4.4 节列出的 14 个文件的行；`W/stores/edit-session.ts:26-32`、`:63-83`、`:180-181`、`:199`；`W/pages/page/edit-lost-banner.tsx:46-47`；`W/pages/notebook/notebook-layout.tsx:76`；`W/pages/notebook/general-page.tsx:128-169`；`W/app/routes.tsx:195-200`；`W/stores/notebook.store.ts:45-48`、`:114-143`；`W/events/handlers.ts:277-291`；`W/app/event-stream.tsx:15-35`；`W/app/problem-messages.ts:11-74`、`:137`；`W/app/problem-messages.test.ts:43-62`；`W/app/pages-locked.tsx:19`；`W/test/fakes.ts:88-103`；`W/stores/notebook.store.test.ts:7-19`；`W/pages/notebook/changeset-text.ts:13-27`；`W/pages/notebook/transfer-names.ts:9-34`；`W/stores/transfer.store.ts:27`；`W/test/jobs-server.ts:10`；`W/pages/notebook/transfer-report.tsx:54`、`:122-134`、`:202`；`W/pages/notebook/transfer-page.tsx:46`、`:101`、`:178`；`W/i18n/messages/en.ts:1093`；`W/i18n/messages/zh-CN.ts:1047` |
| e2e（第 5 节） | `e2e/playwright.config.ts:3`；`e2e/fixtures/notebooks.ts:16-25`、`:83`；`e2e/fixtures/assert/notebook.ts:46`；`e2e/fixtures/assert/page.ts:70`、`:139`、`:222`、`:296`；`e2e/fixtures/assert/transfer.ts:179`；`e2e/fixtures/transfer.ts:196`、`:206`、`:231`、`:256`；`e2e/fixtures/mcp.ts:152`、`:221`、`:227`、`:240`；`e2e/fixtures/auth.ts:67`；`e2e/stories/wiki/lw12-downgrade.spec.ts:60-72`；`e2e/stories/workspace/w9-leave-remove.spec.ts:88`；`server/configs/config.yaml:17`、`:106`、`:121` |
| 设计审查第一轮之后核对的（2.1 第 11–13 条、2.3、3.13、3.14、3.18、3.19、7.2、7.3） | `page/app/unit_revert.go:149-157`；`page/app/unit_move.go:64`、`:75-107`；`page/domain/revert_schedule.go:520`；`page/app/unit_delete.go:34-41`；`page/app/unit_create.go:66`；`unit_create_asset.go:50`；`unit_restore.go:81`、`:404`；`unit_relocate.go:76`；`page/module.go:188-191`；`linking/app/lint_reads.go:12-16`、`:256-261`、`:271-310`；`linking/app/lint_scope.go:184-187`；`linking/adapter/postgres/queries/lint.sql:1-24`；`linking/adapter/postgres/lint_plan_test.go`；`wiki/app/lint_ports.go:82-110`；`wiki/app/lint.go:220-250`；`wiki/app/template.go:10-64`；`wiki/app/ports.go:10-30`；`bootstrap/wiki.go:20-36`；`bootstrap/wiki_lint_cost_test.go:27-29`；`transfer/module.go:129-238`（`beat` `:185-188`）；`transfer/app/job_run.go:32-33`、`:127-173`、`:181-227`；`transfer/app/background.go:54-74`、`:140-170`、`:258-298`；`transfer/adapter/postgres/queries/jobs.sql:66-90`、`:120-132`；`transfer/adapter/postgres/store_test.go:245-272`、`:304`；`transfer/app/export.go:403-408`、`:425-429`；`transfer/contributors_test.go:85`；`transfer/adapter/http/module_background_test.go:37`；`bootstrap/wire.go:62-89`、`:212`；`bootstrap/source_background_test.go:96`；`bootstrap/interleavings_background_stops_test.go:328-360`、`:684-712`；`bootstrap/permission_matrix_transfer_test.go:41-50`；`bootstrap/permission_matrix_coverage_test.go:155`；`bootstrap/interleavings_page_test.go:91-92`；`api/modules/page.yaml`、`asset.yaml`、`transfer.yaml` 的 14 个 `x-problem-codes`（3.6 的表逐个抄自它们）；`W/app/effective-role.test.ts:5`、`:28`、`:30`；`W/pages/notebook/transfer-job-row.tsx:38`、`:168`；`W/pages/notebook/transfer-names.ts:22-34`、`transfer-names.test.ts`；`W/pages/notebook/general-page.test.tsx`；`W/stores/transfer.store.ts:101`；`W/test/jobs-server.ts:204`。审查者 B 的实测（PostgreSQL 18.6 的一次性容器，经 `heavy.sh`；脚本与输出 `second_pass.sql`、`second_pass2.sql`、`setwanted.sql` 与 `.out` 在设计审查的工作目录里，未入库）：数据与数见 2.1 第 13 条与 7.6 |
| 00 号文档与总体设计的行（2.1、第 8 节） | `00-M11-design.md:7`、`:29`、`:76`、`:131`、`:135`、`:141`、`:156`、`:173`、`:213`、`:282-289`、`:452`、`:514-527`、`:648-650`、`:666`、`:692`、`:826-844`；`v0.1-design.md:1049`、`:1071`、`:1073`、`:1100-1104`、`:1127`、`:1137`、`:1164`、`:1199`、`:1209`、`:1264-1275` |
