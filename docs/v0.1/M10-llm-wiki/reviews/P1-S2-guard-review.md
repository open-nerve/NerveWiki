# M10/P1 S2（守卫的骨架）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m10-p1-s2`（实施的 4 个提交：单元在锁下读笔记本的状态（`ShareNotebook` 的两列，notebook 的根的 `ShareState`、`LockState`，page 的端口 `ShareByID`、`LockByID` 交回状态，`pageNotebooks` 的 `toPageState`，`Write.Notebook`、`Write.Grants` 与 `Grants`）`70b0ccdd`，`CheckContract`（`StepFacts`、`NodeFacts`、`NodePlace`、`FactsOf`、`Verdict`、`ReservedKeys()`、R1–R4，三个错误与 `Refused`/`Refuse`，`page.manage_source` 与它的规则）`ee64833c`，模式的守卫（`ModeGuard`、`NewModeGuard`、`page.Refusal`，`Unit.apply` 的标出与 `batch` 的 `failed`，`pageRegistrants(pool, authorizer)` 的 `[mode, lock]` 与 `pageSessionSubscribers()`，六个操作的 `x-problem-codes` 与说明、page 与 asset 的适配器的表的行、`mcpToolCodes()` 与 `wikiCodeCalls`、矩阵的变体的六行与 `adminsOnlyOr`、三个码的中英文字，3.24 的整个程序的测试与 3.23 的交错，e2e 的 LW2 (API)、LW2 (MCP)）`99ec6abe`，`batch` 的计划判契约（`PlanBatch` 的状态、`judge`、`Needs`、`Refuse`，`planned` 的 `StateOf` 与问、`decidedNotebook`，`TestTheGuardRefusesWhatThePlanLetBy` 与 `newAcmeTeamWiring`）`cfa1c608`；第 4 个提交在推送之前修订过一次，加"AGENTS edited and held"一行（修订之前是 `7fa387da`，审查 C-N2）；从 `main` 的 `45d086a1`（S1 的合并）开始，审查时在 `cfa1c608` 上，57 个文件、加 2,507 行、删 181 行。修复一轮四个：A-M1 = C-M2 `a5ea0d68`、C-M1 `8b38a823`、C-N1 `a5990de5`、A-N1 `7834bd70`（4 个文件、加 117 行、删 15 行：三个测试与一处注释）；一共 8 个，合并见 00 号文档第 12 节），对照 [01-P1-contract-frame.md](../01-P1-contract-frame.md) 第 0–2 节、3.17–3.24 与 3.29 的 S2 的各行、4.2、第 5 节的 S2 的部分、6.2、6.5、第 7 节的 S2 的各行（7.3 的第 23–41 条）与第 8 节，[P1 设计审查](P1-design-review.md)（主持者的决定与两轮复核），[S1 的审查记录](P1-S1-mode-template-review.md)，[00-M10-design.md](../00-M10-design.md) 4.1、4.6、4.7、4.9、4.17 与第 5、7、8、9 节，[v0.1 总体设计](../../v0.1-design.md) 12.4 与第 13 节，实施者的报告（15 条偏离、门禁、实测、CI、变异与反向对照的表、留下的三项） |
| 审查方式 | 两位审查者（Opus）并行、只读（主持者的决定 27：负责人的额度紧，三位改为两位，A 的范围兼了 B 平常的那一份服务端：守卫的事实与每条写的路径），各在自己的分离的工作树里做实验，用到 Docker 的都经 `heavy.sh`，不下载任何东西：守卫、规则、每条写的路径、码与 `batch` 的计划（A：快的包（`page/domain` 的契约与计划的测试，page 与 asset 的 HTTP 适配器，`archtest`）；经 `heavy.sh` 在竞态下跑 S2 的七个整个程序的测试与 `TestPageNotebooksConvertTheState`（包 13.3 秒）；只在自己的工作树里的探针：`TestZZReviewAProbes`（每条写的路径在一本 LLM Wiki 与一本普通的孪生里，约 45 个请求）、`TestZZReviewARevertOfAWindowsIndex`（A-M2）、`TestZZReviewARevertLockBeforeMode`（A-M3），纯函数的 `TestZZReviewARefuseAtABound`（A-M1）；自己的变异 RA1–RA6，抽查实施者的第 24、32、35、36、37 条）；测试、变异、e2e、网页、提交、门禁与 CI 的失败（C：4 个提交各在一份 `git archive` 的副本里、`GOPROXY=off GOFLAGS=-mod=readonly`：`go build`、`go vet`、整个服务端模块（加 `config verify`）与 `server/tools` 的 golangci-lint、`go mod tidy -diff`、经 `heavy.sh` 的整个 `go test -short`；第 1、3 个提交上 `make gen-go` 与提交里的生成物相同；第 3 个提交上离线的 `pnpm install` 之后 `make lint-web`、`make knip`、`make gen-web`（bundle 与 `schema.gen.ts` 逐字节相同）与 vitest（166 个文件、3,349 个测试）；以 `-race` 的 `go test -list` 核对头与起点的分份；竞态下量六个新测试（包 9.4 秒，没有 `DATA RACE`）；`playwright test stories/wiki/`（4 个都通过）；读一次 CI 的作业与 main 最近的运行；在头与起点上交替各两次量 grep 的成本守卫；自己的变异 c01–c08、h1–h7，重跑实施者的第 23、26、31、33、35–38、40、41a、41b 条与最后一跳、次序）。修复一轮不另开审查者的修复核对，由主持者自己核对（主持者的决定 31） |
| 日期 | 2026-10-25 |
| 结论 | 没有 Important。Minor 五条，不同的四条（A-M1 = C-M2）：两条在修复一轮改，两条是 S2 与 S3 之间的窗口、记进文档。记为中低的一条：没有测试核对守卫以哪个节点标出拒绝（C-M1）：删除或移动 `raw` 的祖先时被拒的节点是 `raw`，不是步的第一个节点，守卫不标出或以第一个节点标出的两个变异过 page 的每个测试；S2 里生产的代码还不读这个节点，读它的是 S3 的 `treeError` 与报告的 `refusal.node_id`，S3 设计的测试也看不出它。其余三条是低：`BatchPlan.Refuse` 的两种行为（偏离 10：取代同一项上批量自己的界线、清掉 `final`）没有测试，两个变异存活（A-M1 = C-M2）；设计没有列出的两个 S2→S3 的窗口：恢复或导入放到根下的 `index` 改名之后，撤销这次改名答 `revertChangeset` 没有声明的 `page.virtual_file`（A-M2），同步的撤销在前一步碰到编辑锁时答 `page.locked`、不答后一步的 `page.source_admin_only`（A-M3），都由 S3 关上（主持者的决定 29）。细节三条：`ApplyBatch.Execute` 的次序的注释没写契约（A-N1，改）；MCP 的 `write` 覆盖已有的页在 LLM Wiki 里没有整个程序的调用（C-N1，改）；实施者的报告的提交列表写的是第 4 个提交修订之前的 sha（C-N2，记录改）。两位都没有找到答错的结果：A 在真的库上走过每条写的路径（LLM Wiki 与普通的孪生，约 45 个请求），守卫、规则与计划都照设计答；C 的每个提交单独都绿，守卫的最后一跳在五个整个程序的测试与两个 e2e 的故事上都失败得了，交错都是逼出来的。修复一轮四个提交，生产的代码只改一处注释，主持者自己核对（决定 31）：审查者存活的四个变异现在都失败（7.3 的第 81–84 条），实施者在修好的头上重跑第 23–41 条与九个要改代码的反向对照（36 次），都被抓到。建成的 CI 运行红在 M9 的 grep 的成本守卫（中位数 388 毫秒，它的界 270 毫秒），不是 S2 的（C 的诊断，主持者的决定 30）；修复一轮的运行全绿，组合根三份的测试一步的两次的平均都在 7:30 之下。实施者的 15 条偏离由两位判过，都对（第 10 条没有测试，即 A-M1 = C-M2；第 7 条的 `revertChangeset` 的一句说的次序 S2 不守，即 A-M3）。主持者的 5 个决定见下（第 27–31 条，接着 S1 的审查记录的第 23–26 条）。合并见 00 号文档第 12 节 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：A-M1–A-M3、A-N1，C-M1、C-M2、C-N1、C-N2；没有审查者的修复核对，主持者的核对见"核对"。修复一轮的条目照主持者的修复说明编号 1–4。同一个发现合为一个修复：A-M1 与 C-M2） | — |
| C-M1 | 中低（测试） | 没有测试核对守卫以哪个节点标出拒绝，在那个节点不是步的第一个节点的情形（`page/app/mode_guard.go:37` 的 `domain.Refuse(v.Err, v.NodeID)`、`:41` 的 `domain.Refuse(domain.ErrSourceAdminOnly, v.NodeID)`）：R2 的祖先的删除（删 `Notes`，`raw` 在它之下）与 R4 的祖先移到别的父节点下，`v.NodeID` 是 `raw`，步的第一个节点是 `Notes`。C 的 c01（`return v.Err`，不标出：`Unit.apply` 的 `refusalOf` 于是以第一个节点 `Notes` 标出）与 c02（以 `s.Changes[0].NodeID` 标出 `page.source_admin_only`）过 `page/app` 与 `page/domain` 的每个测试：`TestAUnitMarksItsGuardsRefusals` 用替身守卫，`TestTheModeGuardAsksOnceAUnit` 拒绝的步的第一个节点就是 `raw`（`raw` 移它自己），计划的两个测试核对的是计划的判定、不是守卫的。S2 里生产的代码不读这个节点（`page.Refusal` 到 S3 的 `treeError` 才有调用者，REST、MCP 与 `batch` 的 `OpFailed` 不答节点），所以整个程序的测试看不出它。3.20 的注释说"Its refusals are marked (Refused) with the node they are about"，读它的是 S3 的 `report.refusal.node_id`；S3 设计的 `TestTheSourceRootsPutBackIsRefusedInALaterBatch` 拒绝的是 `raw` 自己，也抓不到这两个变异。C 以一个草稿测试复现（两步 LLM Wiki 的步，直接调 `GuardWrite`，核对 `r.NodeID` 是 `raw`：分支上通过，c01、c02 之下失败；`rC-scratch-guard-node_test.go.txt`） | 已改（`8b38a823`，修复一轮第 2 项）：`TestTheModeGuardMarksTheNodeItRefuses`（在 `TestTheModeGuardLetsAStandardNotebookBy` 旁边，C 的草稿测试，`page.manage_source` 一律被拒）：删除 `Notes`（`raw` 在它之下）答以 `raw` 标出的 `page.protected`；把 `Notes` 移到 `Other` 之下（`raw` 在它之下）答以 `raw` 标出的 `page.source_admin_only`。变异：c01、c02 都失败（`logs/fix1-mut-f2.log`；7.3 的第 83、84 条） |
| A-M1 = C-M2 | 低（测试） | 偏离 10 给 `BatchPlan.Refuse`（`page/domain/batch_plan.go:871-877`）的两种行为没有测试：取代同一项上的失败（批量自己的界线）、清掉 `final`。A 的 RA1 = C 的 c06（`p.Failed.Op <= at`：保留同一项上的失败）与 RA2 = c05（不清 `final`）过 `page/domain` 与 `page/app` 的每个测试：`TestThePlanChecksTheContract` 的两行 `Refuse` 一行在更晚的失败之前、一行在更早的失败之后，`TestABatchsPlanRefusesBeforeItsLock` 没有一行的界线落在要问的那一项上。情形（写者的 MCP 的 `batch`；笔记本里有 `raw`、`Notes` 与下面 9,999 页的 `Big`）：`[delete Big（递归）, move raw.md → Notes/raw.md]` 的项数在第 1 项过 `MaxUnitNodes`（`final`），`Needs` 也在第 1 项：建成的答第 1 项的 `page.source_admin_only`，RA1 之下答 `page.batch_too_large`（分开的调用没有这条界线）；`[edit Notes "zzz"→"y"（不匹配）, delete Big, move raw]`：建成的答第 0 项的 422（分开调用时第一个被拒的），RA2 之下正文的一遍不跑、答第 2 项的 403。C 的 c06 的形状是移动带着的节点的界线（`[move Big（6,000 页）→ X/Big, move Notes（raw 与 5,000 页）→ Other/Notes]`）。两种形状都要过一万个节点的界线，窄；但 13.4 第 1 条与 3.29 要每个新检查有反例，`Refuse` 只有这几行没有钉住。A、C 各以一个纯函数的草稿测试复现（`TestZZReviewARefuseAtABound`、`rC-scratch-refuse-bound_test.go.txt`） | 已改（`a5ea0d68`，第 1 项）：`TestThePlanChecksTheContract` 加审查者的三行：项数的界线（A 的：`Big` 下 `MaxUnitNodes - 1` 页，`[delete Big, move raw.md → Notes/raw.md]`：`Refuse` 之前是第 1 项的 `page.batch_too_large`、`Needs` 在第 1 项，`Refuse(1)` 之后是第 1 项的 `page.source_admin_only`）；移动带着的节点（C 的：5,000 页的 `Large` 移到 `X/Large`、带着 `raw` 与 5,000 页的 `Notes` 移到 `Other/Notes`，同样在第 1 项）；清掉 `final`（A 的：`[edit Notes "zzz"→"y", delete Big, move raw]`：之前是第 2 项的界线、什么也不读，`Refuse(2)` 之后 `Reads` 列出 `Notes`，`Contents` 答第 0 项的 `validation_failed ops[0].old_string`）。测试多 0.04 秒。变异：RA1、RA2 都失败（`logs/fix1-mut-f1.log`；第 81、82 条） |
| A-M2 | 低（窗口） | S2 与 S3 之间，`revertChangeset` 答一个它没有声明的码 `page.virtual_file`（`api/modules/page.yaml` 的 `x-problem-codes`，与 `page/domain/contract.go:118-124` 的 R3）。3.21 的表说撤销的放回到不了 R3（"根下从来没有保留的键"）；S2 里到得了：设计自己的 S2→S3 的窗口许恢复（与导入）把 `index` 放到 LLM Wiki 的根下，S3 才加序号。真的库上的探针 `TestZZReviewARevertOfAWindowsIndex`：bob 建 `Other/index`、删 `index`、再删 `Other`；恢复 `index`，父节点不在，落到根下仍叫 `index`（窗口许的）；改名为 `Foo`（R3 只看改名之后的名字，许）；撤销这次改名，放回在根下改名为 `index`：409 `page.virtual_file`，契约的检查让请求失败（`problem code "page.virtual_file" is not declared`，`rA-probe3.log`）。只在 S2 与 S3 的两次合并之间到得了：S3 之后 LLM Wiki 的根下没有保留的键，表的那一句重新成立；第 0 节说其间不合并别的、不发布 | 文档（主持者的决定 29）：列进第 0 节的 S2→S3 的窗口，3.21 的表的那一格写明 S2 与 S3 之间到得了；不在 `revertChangeset` 上声明这个码（S3 之后到不了，A 的建议）；代码不改 |
| A-M3 | 低（窗口） | S2 里同步的撤销在前一步碰到锁时答编辑锁的 `page.locked`，不答后一步的模式的 `page.source_admin_only`（`page/app/unit_revert.go:97-124` 与 `unit.go:346-350`）：守卫 `[mode, lock]` 逐步判，第一个被任何守卫拒绝的步定答复；S2 的同步撤销没有契约的预检（`revertChecks.check` 与 `Unit.Revert` 的锁下的一遍到 S3 才判，3.25）。真的库上的探针 `TestZZReviewARevertLockBeforeMode`：alice 的一个 MCP 的 `batch` 把 `raw` 改名为 `sources`、在根下写一页新的 `raw`；carol 在新的 `raw` 上开编辑会话；bob（写者）撤销 alice 的变更集：第一轮删除新的 `raw`、碰到 carol 的锁，第二轮会把 `sources` 放回为 `raw`、要 `page.manage_source`。答 409 `page.locked`（carol 的），S3 的预检会答 403 `page.source_admin_only`（`rA-probe4.log`）。13.1 第 4 条照 S2 改的"模式在锁之前"与 `revertChangeset` 的新的说明（"…page.source_admin_only. Then a page whose lock an edit session holds … is page.locked"）在 S2 对这个形状都不成立；REST 与 MCP 的一步的路径与 `batch` 没有问题：一步由 `[mode, lock]` 判，`batch` 的计划在它的编辑锁的预检之前判 | 文档（主持者的决定 29）：列进第 0 节的窗口，S3 的撤销的两遍预检关上；S3 加一个钉住这个形状的测试（前一步碰到锁、后一步被模式拒绝：没有预检时答 `page.locked`，只有 S3 的预检过得了），写进 [01-P1](../01-P1-contract-frame.md) 3.28 与第 9 节；代码不改 |
| A-N1 | 细节（注释） | `ApplyBatch.Execute` 的注释列出 `batch` 的次序（`page/app/apply_batch.go:105-120`），没写契约：树的一遍现在判契约（`page.protected`、`page.virtual_file` 在它们的项上），`page.source_admin_only` 在读的上限之前问、答；`planned` 的注释改了，`Execute` 的（用例写明的次序）没改 | 已改（`7834bd70`，第 4 项，这一轮唯一碰生产的文件的一处，只是注释）：树的一遍那一句写出它自己的检查在先、再在不加锁读到的状态上判契约，`page.protected`、`page.virtual_file` 在它们的项上；项数的界线之后，`page.manage_source` 问要它的第一项，被拒时在那一项上答 `page.source_admin_only`、取代更晚的失败，在读与编辑锁的预检之前 |
| C-N1 | 细节（测试） | 3.23 把"MCP 的 `edit`、`write` 覆盖已有的页"列为 P1 里没有可拒的路径，"由 3.24 的整个程序的测试核对它在 LLM Wiki 里照常成功"；`TestTheModeGuardReachesEveryMCPWrite` 与 LW2 (MCP) 只以 `edit` 写 `AGENTS.md`，没有一个调用以 `write {"path":"AGENTS.md", …}` 覆盖它，也没有一个写它的 `batch`（C 搜过 S2 的每个测试）。风险低：两者都是 `OpContent`，`CheckContract` 不论路径都放过，REST 的 PUT 在第 8 步钉住 | 已改（`a5990de5`，第 3 项）：`TestTheModeGuardReachesEveryMCPWrite` 在 `edit AGENTS.md`（读作 `mcpEdited`）之后，以它答的版本 `write` 覆盖 `AGENTS.md`：写下、什么也没建，`AGENTS` 的正文是这次写的，agent 的变更集是 `["edit mcp:whole-program", "edit mcp:whole-program"]`。测试单独在竞态下 2.29 秒（包里的第一个测试，带着应用的启动） |
| C-N2 | 细节（记录） | 实施者的报告的"Commits"一节把第 4 个提交写作 `7fa387da`，那是修订之前的 sha：reflog 里 `7fa387da` 修订成 `cfa1c608`，修订只加了"AGENTS edited and held"一行（钉住第 38 条变异的那一行）；报告的 10.2 的草稿与每个提交的核对写的是 `cfa1c608`，对 | 文档：10.2 与本记录写 `cfa1c608`，注明修订之前的 sha；修复一轮里实施者的报告的提交列表随之改正 |
| 存疑 | 细节（存疑） | A：（1）S2 里的后台撤销（设计写明的窗口）：被拒的一步让 `startRevert` 的任务以 `internal` 停下（`revertDefect`），设计写着；没有核对的是同一个任务也遇到 A-M3 的形状：前一步碰到锁的一批以 `page_locked` 停下，而 S3 的开始时的预检会先拒绝整个任务。（2）`ErrSourceAdminOnly` 的文字（"…rename, move or delete in an LLM Wiki's source area, its source root included"）照字面是管理员可以删除来源根，而 `page.protected` 不许；网页的文字（4.2）为此改写过（P1 设计审查 B-N9），服务端给 MCP 的 agent 读的没有；它照 3.22 写，是设计的文字，不是代码的缺陷。（3）CI 的守卫一份红（C 的范围）：grep 的路径上 S2 只把 `decideNotebook` 改成包着 `decidedNotebook` 的一行（读的一样，`authorize.go:34-59`），A 看不出它怎么能多花 100 毫秒。C：没有留下的存疑，怀疑的都追过或跑过 | 记下、不改："接受与推后的"：（1）S3 的开始时的预检盖住它；（2）P2/S1 加来源区的规则时看这句话（[01-P1](../01-P1-contract-frame.md) 第 9 节）；（3）C 诊断（下面"门禁与 CI"），主持者的决定 30 |
| 偏离 | — | 实施者的 15 条偏离（[01-P1](../01-P1-contract-frame.md) 10.2、3.31）：1 notebook 的存储的 `ShareNotebook` 答 `(domain.Mode, uuid.UUID, error)`，根建 `State`；2 事实的位置叫 `NodePlace`；3 `ReservedKeys()` 是函数；4 `page.manage_source` 在第 2 个提交，第 1 个提交的 `TestAGrantIsAskedOnceAUnit` 问 `node.move`、`node.rename`；5 `pageSessionSubscribers()` 不收连接池；6 MCP 的 `move` 发 `from`，`wikiCodeCalls` 的写者是经 SQL 加进 acme 的 erin；7 `uploadAsset` 与 `revertChangeset` 的说明的新句的位置与写法；8 `decidedNotebook`；9 树的一遍在每一项自己的方法里判（`judge`）；10 `Refuse` 取代同一项上的失败、清掉 `final`；11 没有钉住整个调用列表的 `batch` 测试，替身记 `"StateOf"`；12 `TestTheModeGuardAsksOnceAUnit` 的拒绝的一行改为计划放行、锁下拒绝，替身授权者默认答 `ErrNotVisible`、加 `failing`；13 B-Q1 的一行写成同一父节点下的改名与移到原处；14 `TestABatchsPlanRefusesBeforeItsLock` 加"写者的 `[move raw, edit AGENTS]`，AGENTS 被持着"；15 第 23、29、32、40 条变异的写法。另有报告里写了、没进编号的一处：第 3 个提交的 MCP 的变更集按 `client LIKE 'mcp%'` 找（`acmeTeam` 的 REST 请求带登录会话，它们的变更集的客户端是 `web`） | 两位在各自的范围里判过，都对：A 判全部 15 条（第 7 条对 S2 是对的，`revertChangeset` 的一句的"Then … page.locked"说的次序 S2 不守，即 A-M3；第 10 条的行为对、没有测试，即 A-M1；第 12 条照旧分得出指针与值，A 抽查第 32 条被抓到；第 14 条是唯一钉住计划的问在编辑锁的预检之前的一行；第 15 条的第 29 条照字面写不出：祖先不在事实里）；C 判第 4、6、9–15 条与 `client LIKE` 的一处（第 12 条比设计的写法好：分得出单元的问与计划的问，抓到 c04 与第 40 条；第 14 条是要的，它钉住第 38 条；`client LIKE` 之后要的恰是 `["edit mcp:whole-program"]`，多一个被拒的变更集就失败；第 23、40 条照 C 的写法重跑也被抓到） |
| 流程 | — | 实施者在第 3 个提交之前两次在 `heavy.sh` 之外以 `go test -short` 跑 page、notebook 与 `bootstrap` 的包（`-short` 下 `pgtest` 在起容器之前跳过，`pgtest.go:88`），之后每次都经 `heavy.sh`。修复一轮的第一次 `make test` 244 秒以 2 退出，不是我们的代码：组合根的一份起不来它的 PostgreSQL 容器（对 Docker 的套接字 `context deadline exceeded`，那时另一个会话的 `postgres:18.6` 容器正在共用的 Docker 上起，负载 8–10），那个二进制的每个测试都在准备时失败，另两份与其余的都过；再跑一次 890 秒通过（第一次的日志留在 `logs/fix1-gate-test-1st.log`）。C 一次以 shell 的 `&` 起一个经 `heavy.sh` 的竞态的计时，不是工具的后台选项：照样在 `heavy.sh` 下跑完、放开了锁，没有再犯；C 的 h1v 与 h7 的输出重定向到文件，helper 看不到测试名、记作 FAILED(unnamed)，C 读过两份输出，是下面点名的失败。A 的第一次探针（`rA-probe1.log`）有两处是探针自己的错（排序到 `raw` 已在的位置是什么也不写的 200，链接改写碰到会话的 `linking.pages_locked`）。实施者在建成的 CI 红了之后照说明停下，没有再推、没有重跑（没有权限） | 无害（C 判第一处；第二处是共用的 Docker，重跑通过；其余照实写下） |

## 主持者的决定

负责人可以改判（接着 [S1 的审查记录](P1-S1-mode-template-review.md)的"主持者的决定"第 23–26 条编号，它接着 [P1 设计审查](P1-design-review.md)的第 1–22 条）：

27. **精简的审查**：两位 Opus 审查者 A、C，不是三位：负责人的额度紧（2026-10-25）。A 的范围兼了 B 平常的那一份服务端（守卫的事实与每条写的路径）。
28. **修复一轮的处置**（`fix-brief-1.md`，2026-10-25）：A-M1 = C-M2 在 `TestThePlanChecksTheContract` 加 `Refuse` 的两种行为（审查者的形状），两个变异照此失败；C-M1 加两行，删除 `Notes`（`raw` 在它之下）与把 `Notes` 移到另一个父节点下各以 `NodeID == raw` 拒绝，C 的两个变异照此失败；C-N1 在 `TestTheModeGuardReachesEveryMCPWrite` 加 MCP 的 `write` 覆盖一页已有的页（成功，3.23）；A-N1 改 `ApplyBatch.Execute` 的次序的注释。新的变异照修复报告的建议作 7.3 的第 81–84 条。这一轮除了一处注释没有改生产的代码。A-M2、A-M3 与 C-N2 是主持者的，在文档里，代码不为它们改。
29. **A-M2 与 A-M3 是 S2 与 S3 的两次合并之间的窗口**，由 S3 关上，与第 0 节的窗口列在一起（[01-P1](../01-P1-contract-frame.md) 第 0 节）：A-M2：恢复把 `index` 放到 LLM Wiki 的根下之后（S2→S3 的窗口许的），撤销它的一次改名答 `revertChangeset` 没有声明的 `page.virtual_file`；A-M3：同步的撤销能在 `page.source_admin_only` 之前答 `page.locked`，因为守卫逐步判，S3 的撤销的预检关上它。S3 加一个钉住 A-M3 的确切形状的测试：前一步碰到锁、后一步被模式拒绝（3.28 与第 9 节）。
30. **建成的 CI 的红不是 S2 的**（C 的诊断）：S2 不碰 grep 的守卫跑的任何东西，本机上守卫在 S2 与它的起点上量得相同（中位数 75–87 毫秒）；main 的守卫一份在没有改的代码上从 7:57 到 12:35；修复一轮的运行过了它。它再失败时，守卫照 M9 给它的兄弟的界的改法（量过的 CI 的中位数，或五次的中位数），不在 M10 的哪个 Step 里改；记作 M10 的收尾的一项（[00-M10](../00-M10-design.md) 第 7 节"写给后面的 M"）。
31. **主持者自己核对修复一轮**（代替审查者的修复核对，为了额度）：读过 `cfa1c608..7834bd70` 的差异：四个提交，三个测试、一处注释；实施者重跑了第 23–41 条与要改代码的反向对照（36 次，都被抓到），审查者存活的四个变异现在都让它们的测试失败（第 81–84 条）；CI 全绿。

## 接受与推后的

- **接受**：
  - 两位审查者与主持者的核对（决定 27、31）：B 平常的那一份由 A 盖住（每条写的路径在真的库上、LLM Wiki 与普通的孪生各走一遍）；修复一轮只加测试与一处注释，所以不另开修复核对。
  - S2→S3 的四个窗口（第 0 节的两个与 A-M2、A-M3，决定 29）：只在两次合并之间的 `main` 上，不发布。
  - 建成的运行的红（决定 30）：S2 不碰 `GrepCandidates`，这一次运行整个慢（没有 S2 的测试的第一份 4:59 → 7:18，竞态 6:00 → 7:41，守卫 7:57 → 12:10，对 S1 的合并的运行 40116706995），修复一轮的运行过了它。
  - 守卫与计划问授权者的目标的工作区：A 的 RA3、RA4（不带工作区问）过模块的测试（替身授权者不看目标），被真的授权者（`access/app/authorizer.go:38` 要目标的工作区）与 `TestTheModeGuardReachesEveryPath`、`TestTheGuardRefusesWhatThePlanLetBy` 抓到。
  - `batch` 的计划对每个 `batch` 都读一次 `StateOf`（7.6 收下）；计划在 `CheckContract` 看模式之前就建事实，只花 CPU（10,000 个名字的 `TitleKey` 量得 1.2 毫秒，A）。
  - CI 的作业日志要认证：每个测试在 CI 上的用时没有记下，只有每一步的时间。
- **推后**：
  - S3：关上两个新的窗口与 A-M3 的形状的测试（[01-P1](../01-P1-contract-frame.md) 3.28、第 9 节）；A 的第一条存疑（后台撤销也遇到 A-M3 的形状）由 S3 的开始时的预检盖住。
  - P2/S1：`ErrSourceAdminOnly` 的文字（A 的第二条存疑，第 9 节）。
  - M10 的收尾：grep 的守卫再失败时的改法（决定 30）。
  - 6.5 的两次的平均在合并之后读（第 10 节记下；10.2 记下分支的两次）。
  - 没有交给 v0.1 之后的打磨的。

## 反向对照

建成时以 `mut2.py`（`mut.py` 的多文件的写法：第 34、38、39、40 条与最后一跳改两三个文件）跑，库的经 `heavy.sh`，每个之后由 helper 复原、`git status` 照旧干净：7.3 的与九个反向对照的 spec 在 `spec-s2.json`（36 次，输出 `logs/mut-s2.log`），契约与 MCP 的码的四个反向对照在 `spec-s2b.json`（`logs/mut-s2b.log`），第 1 个提交的转换的四个在 `spec-s2c.json`（`logs/mut-s2c.log`）；修复一轮在 `7834bd70` 上重跑 `spec-s2.json` 的 36 次（`logs/fix1-mut-s2.log`），审查者的四个在 `mut-fix1.json`、`mut-fix2.json`（`logs/fix1-mut-f1.log`、`fix1-mut-f2.log`）。这些文件都在 `scratchpad/m10/p1s2/` 下；审查者的（`rA-mut.py`、`rA-work/`、`rC-mut.py`、`rC-spec-*.json`、`rC-mut-*.out` 之类）在 `scratchpad/m10/p1s2-review/` 下。没有一个是编译不过、命令没跑或不点名的失败（C 的 h1v、h7 的 FAILED(unnamed) 见上表"流程"）。

**7.3 的第 23–41 条**（建成时都被点名的测试抓到，27 次：第 25、26、27、30、31 条各对 `TestCheckContract` 与 `TestTheModeGuardReachesEveryPath` 跑一次，第 34 条对三个测试，第 41 条分 a、b；修复一轮在 `7834bd70` 上重跑，都被抓到）：

| # | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| 23 | `ShareNotebook` 的存储答 `standard` 与零的来源根（查询照选两列、不读：变异之下不重新生成 sqlc，偏离 15） | `TestNotebooksFindAndLock/ShareState reads the state` | 抓到：`ShareState(...) = {standard, 0}, want {llm_wiki, raw}`；C 重跑同 |
| 24 | 守卫 `{lock, mode}` | `TestModeComesBeforeTheLock` | 抓到：降级之前答 409 `page.locked`；A 抽查同，C 的 h2 同 |
| 25 | R1 漏了 `OpMove` | `TestCheckContract`；`TestTheModeGuardReachesEveryPath` | 两者都抓到："AGENTS moved under Notes"、"AGENTS reordered"；REST 的"alice puts AGENTS first"与"moves AGENTS under Notes"答 200 |
| 26 | R1、R2 只看 `Nodes[0]` | 同上 | 两者都抓到："Notes, raw under it, deleted"；REST 的"alice deletes Notes"答 204；C 重跑同 |
| 27 | R3 不只看根下（`After != nil`） | 同上 | 两者都抓到："a page created under Notes as index"；REST 的"bob creates index under Notes"答 409 |
| 28 | R3 也拒导入的新建 | `TestCheckContract` 的导入的一行 | 抓到 |
| 29 | 每个改名都要 `page.manage_source`（改名的一步只有那个节点，祖先的改名只能这样数，偏离 15） | `TestTheModeGuardReachesEveryPath` | 抓到："bob renames Notes"答 403 |
| 30 | R4 对 `OpMove` 只看第一个节点 | `TestCheckContract`；`TestTheModeGuardReachesEveryPath` | 两者都抓到："Notes, raw under it, moved under Other"；REST 的"bob moves Notes under Other"答 200 |
| 31 | R4 不看第一个节点换没换父节点 | 同上 | 两者都抓到："Notes, raw under it, reordered"；REST 的"bob puts Notes first"答 403；C 重跑同 |
| 32 | `Write.Grants` 是一个值（每一步一份单元的副本，同 `Step{Write: u.write}` 复制一个值，偏离 15） | `TestTheModeGuardAsksOnceAUnit` | 抓到：单元里问了 2 次；A 抽查同 |
| 33 | `Grants` 记下数据库的错误（当作拒绝） | `TestAGrantIsAskedOnceAUnit/the database's error` | 抓到；C 重跑同 |
| 34 | 守卫与 `CheckContract` 都不看模式 | `TestTheModeGuardLetsAStandardNotebookBy`；`TestCheckContract`；`TestTheModeGuardReachesEveryPath` | 三者都抓到（普通的笔记本的各行） |
| 35 | `apply` 原样交回守卫的错误、不标出 | `TestAUnitMarksItsGuardsRefusals` | 抓到；A 抽查、C 重跑同 |
| 36 | `failed` 先看 `replans` | `TestABatchAnswersARefusalAtItsOperation` | 抓到：再计划一次、答成第 0 项的；A 抽查同（要第二种能编译的写法），C 重跑同 |
| 37 | 计划不问 `Needs` | `TestABatchsPlanRefusesBeforeItsLock` | 抓到：写者的 `batch` 答 `1 page.protected`；A 抽查、C 重跑同 |
| 38 | 计划在编辑锁的预检之后问 | 同上 | 抓到（写者的几行）；C 以单文件的写法重跑同 |
| 39 | 计划在 `a.read` 之后问 | 同上 | 抓到：`Big` 的一行答 `1 page.batch_too_large`；被编辑并持着的一行读了正文 |
| 40 | 计划的放行交给单元的 `Grants`（单元的 `Grants` 从 ctx 的一个值开始，偏离 15） | `TestTheGuardRefusesWhatThePlanLetBy` | 抓到：bob 被降为编辑者之后他的移动写下；C 的单文件写法（单元的 `do` 先记下放行）被 `TestTheModeGuardAsksOnceAUnit` 与 `TestABatchsPlanRefusesBeforeItsLock` 抓到，作 h4 被整个程序的测试抓到 |
| 41a | 计划把授权者的 `ErrNotVisible` 当拒绝 | `TestABatchsPlanRefusesBeforeItsLock/one who lost the notebook` | 抓到：`0 page.source_admin_only`；C 重跑同 |
| 41b | 计划把数据库的错误标成拒绝 | `…/the authorizer's database error` | 抓到：`0 page.source_admin_only`；C 重跑同 |

**3.29 的 S2 的反向对照**（要改代码才看得到的 13 个；别的几行是测试自己的行：普通的笔记本的几行、没标出的 `page.title_taken` 照旧重新计划、`guards: []` 的程序、`TestModeComesBeforeTheLock` 的两半、handler 的表以标出的错误答出同样的码）：

| 反向对照 | 去掉的检查 | 跑的测试 | 结果 |
|---|---|---|---|
| R-lasthop-rest | serve 不登记模式的守卫（`{lock}`）：REST 的最后一跳 | `TestTheModeGuardReachesEveryPath` | 抓到：模板的撤销答 200，`raw` 随之不见；C 的 h1 同 |
| R-lasthop-mcp | 同上，MCP | `TestTheModeGuardReachesEveryMCPWrite` | 抓到：`write index/x.md` 写下；C 的 h1 同 |
| R-lasthop-lock-order | 同上，次序 | `TestModeComesBeforeTheLock` | 抓到；C 的 h1 同 |
| R-plan-ignores-state | 计划不看笔记本的状态 | `TestThePlanChecksTheContract` | 抓到（LLM Wiki 的每一行） |
| R-plan-move-no-descendants | 计划的移动不列后代 | 同上 | 抓到："the source root's ancestor moved to another parent" |
| R-plan-delete-root-only | 计划的删除只判子树的根 | 同上 | 抓到："the source root's ancestor deleted" |
| R-plan-created-parent-unjudged | 计划只判写的最后一页、不判补建的父页 | 同上 | 抓到："index created at the root as a parent" |
| R-plan-refuse-keeps-reads | `Refuse` 不截 `Reads` | 同上 | 抓到 |
| R-plan-refuse-replaces-earlier | `Refuse` 取代更早的失败 | 同上 | 抓到 |
| R-contract-undeclared | 契约的 bundle 的 `deleteNode` 去掉 `page.protected` | page 的 HTTP 适配器（`apitest`） | 抓到：答出 409 `page.protected`、没有声明 |
| R-contract-unanswered | handler 的表的 `deleteNode` 的 `page.protected` 一行去掉 | page 的 HTTP 适配器，整个包（`apitest.Main`） | 抓到："deleteNode: page.protected"声明了、没有答出 |
| R-mcp-code-not-in-row | `mcpToolCodes()` 的 `write` 一行去掉 `page.virtual_file` | `TestTheMCPToolsErrorsCarryTheirCodes` | 抓到："write's error has the code page.virtual_file, which its row … does not list" |
| R-mcp-code-unanswered | 不调 `wikiCodeCalls` | 同上 | 抓到：行里的新码没有一个调用答出 |

**第 1 个提交的转换**（`spec-s2c.json`，门禁之后；`TestPageNotebooksConvertTheState` 改为三种读法各记下它调的读）：S1 的第 80 条（`toPageState` 对找不到的答 `ok`）在它上面再跑一次照样抓到；转换丢掉 `SourceRootID`、`LockByID` 读 `ShareState`、`ShareByID` 读不加锁的 `StateOf`，都被它抓到（替身记下的读）。

**审查与修复一轮的变异**（写成 [01-P1](../01-P1-contract-frame.md) 7.3 的第 81–84 条，S3、S4 照旧是第 42–64 条）：

| # | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| RA1 = c06（81） | `Refuse` 保留同一项上的失败（`p.Failed.Op <= at`） | `TestThePlanChecksTheContract` | 抓到："the items' bound"与"the moves' carry"答第 1 项的 `page.batch_too_large`（之前没有；第 1 项） |
| RA2 = c05（82） | `Refuse` 不清 `final` | 同上 | 抓到：拒绝之后计划什么也不读，答第 2 项的 403、不答第 0 项的正文的失败（之前没有） |
| c01（83） | 守卫原样交回它的规则的拒绝（`return v.Err`） | `TestTheModeGuardMarksTheNodeItRefuses` | 抓到："Notes deleted, raw under it"没有以 `raw` 标出（之前没有；第 2 项） |
| c02（84） | 守卫以步的第一个节点标出 `page.source_admin_only` | 同上 | 抓到："Notes moved under Other, raw under it"标出的是 `Notes`（之前没有） |

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，修复一轮之后都被抓到）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | RA1、RA2（`Refuse` 保留同一项上的失败、不清 `final`） | 模块的测试下存活：A-M1；A 的 `TestZZReviewARefuseAtABound` 抓到 |
| A | RA3、RA4（守卫、计划问授权者时不带工作区） | 模块的测试下存活（替身授权者不看目标）；整个程序的测试（经 `heavy.sh`）抓到：真的授权者要目标的工作区 |
| A | RA5（守卫不管 `Needs`）、RA6（R3 不看 `OpMove`） | 模块的测试抓到 |
| A | 实施者的第 32、35、36、37 条（模块的测试）、第 24 条（整个程序） | 都抓到（第 36 条要第二种能编译的写法） |
| C | c01、c02（守卫不标出、以第一个节点标出） | 存活：C-M1；C 的草稿测试抓到 |
| C | c03（所有单元一份备忘）、c04（守卫在单元的事务之外问） | 抓到：`TestTheModeGuardAsksOnceAUnit`（c04 另被 `TestABatchsPlanRefusesBeforeItsLock`）；c03 作 h5 也让 `TestTheModeGuardReachesEveryPath` 失败（bob 的撤销照 alice 留下的放行过了） |
| C | c05、c06（`Refuse` 不清 `final`、保留同一项上的失败） | 存活：C-M2；C 的草稿测试抓到 |
| C | c07（树的一遍留下最后一个 `Needs`）、c08（R3 不看节点之前的位置） | 抓到：`TestThePlanChecksTheContract/two needs: the first`、`TestCheckContract` |
| C | h1（不登记模式的守卫，`{lock}`） | `TestTheModeGuardReachesEveryPath`、`TestTheModeGuardReachesEveryMCPWrite`、`TestModeComesBeforeTheLock`、`TestTheProtectionAndADowngrade`（"the deletion first"）、`TestTheGuardRefusesWhatThePlanLetBy`（"the change first"）都失败；普通的孪生的子测试也失败：它的第一次 `createPage` 跑 `checkPages`，看到没有守卫的子测试弄坏的 LLM Wiki，是预期的 |
| C | h7（h1 对 e2e） | LW2 (API)、LW2 (MCP) 都失败（"2 failed"，输出手读：`[409 page.protected]×3` 遇到 200，`errorOf` 遇到写下的删除） |
| C | h2（`{lock, mode}`） | `TestModeComesBeforeTheLock` 失败 |
| C | h3（单元的状态在它的事务之前不加锁读，写进 `Write.Notebook`） | `TestTheProtectionAndADowngrade` 的"the downgrade first"失败：删除在降级提交之前读到 `llm_wiki`；"the deletion first"单独分不出状态在哪里读 |
| C | h4（= 第 40 条）、h5（= c03）、h6（`page.manage_source` 给写者） | 抓到：`TestTheGuardRefusesWhatThePlanLetBy`（"the change first"）、`TestTheModeGuardReachesEveryPath`、`TestPermissionMatrix` 的变体的行（`renameNode`/`moveNode` 来源根的写者的格） |
| C | 实施者的第 23、26、31、33、35–38、40、41a、41b 条、最后一跳与次序 | 都照实施者记下的答（第 38、40 条用 C 的单文件的写法，都被抓到） |

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `7834bd70`（修复一轮 `a5ea0d68`–`7834bd70`，4 个提交），主持者自己（决定 31，代替审查者的修复核对） | 读过 `cfa1c608..7834bd70` 的差异：4 个文件，`batch_plan_test.go`、`mode_guard_test.go`、`mcp_wiki_guard_test.go` 三个测试与 `apply_batch.go` 的一处注释，没有生产的代码的改动；实施者在修好的头上重跑 `spec-s2.json` 的 36 次（7.3 的第 23–41 条与九个要改代码的反向对照），都被抓到，审查者存活的四个变异各让它的测试失败（第 81–84 条）；四个提交各自 build、vet、page 与 `bootstrap` 的静态检查（0 个问题）、`page/app` 与 `page/domain` 的 short 测试，第 3、4 个另经 `heavy.sh` 跑 `bootstrap` 与 page 的 `-short`；本机的门禁全绿（`make test` 的第二次）；CI 运行 40131529634 全绿，守卫一份过了 grep 的守卫 | 合并（见 00 号文档第 12 节） |

## 实测

- **六个新的整个程序的测试**（建成时，`go test -race -count=1 -run '^(…)$' ./internal/bootstrap` 经 `heavy.sh`，开始时负载 4.6）：包 9.1 秒。第三份的四个共 6.1 秒（`TestTheProtectionAndADowngrade` 2.42（1.88 与 0.54）、`TestTheGuardRefusesWhatThePlanLetBy` 2.03（0.91、没有守卫的 0.55、0.57）、`TestTheModeGuardReachesEveryPath` 1.13（两本各 0.32、0.28，另有准备）、`TestTheModeGuardReachesEveryMCPWrite` 0.56），乘 1.73 约 10.6 秒；第二份的两个共 1.1 秒（`TestTheDowngradeEndsTheContract` 0.58、`TestModeComesBeforeTheLock` 0.53），乘 1.73 约 1.9 秒。都在 7.5 的估计之内。审查 C 在负载 8–9 时量的：包 9.4 秒，第三份约 6.5 秒（2.65（2.11 与 0.54）、2.09（0.93、0.57、0.60）、1.16、0.57），第二份 0.57 与 0.57，在实施者的数的几个百分点之内；A 跑七个整个程序的测试与 `TestPageNotebooksConvertTheState`，包 13.3 秒。
- **改过的测试**：`TestTheMCPToolsErrorsCarryTheirCodes`（第三份）5.59 秒（C 5.57）；它与 `TestPermissionMatrix` 一起的包 30.3 秒（C 27.1；S1 记下矩阵单独 21.0 秒）。S2 的矩阵的六行各 13 格、共 78 格（7.5 估的是约 30 格），格的时间之和 43.5 秒，并行跑。修复一轮改的 `TestTheModeGuardReachesEveryMCPWrite` 单独在竞态下 2.29 秒（包里的第一个测试，带着应用的启动）；`TestThePlanChecksTheContract` 多 0.04 秒。
- **e2e**（不带竞态）：建成时 `make e2e` 286 个 37.0 秒（整个目标 39 秒）：LW2 (API) 80 毫秒、LW2 (MCP) 65 毫秒、LW1 (API) 58 毫秒、LW1 (MCP) 37 毫秒、MC1 205 毫秒；审查 C 的 `playwright test stories/wiki/` 4 个（LW1 (API) 82、LW1 (MCP) 32、LW2 (API) 118、LW2 (MCP) 50 毫秒）；修复一轮 286 个 36.9 秒（整个目标 38 秒）。
- **grep 的成本守卫**（审查 C，经 `heavy.sh`，头与起点交替各两次，负载 4.5–9.0，`rC-grep-guard.log`；三次调用的中位数，毫秒）：

  | 阶段 | 头 `cfa1c608` | 起点 `45d086a1` |
  |---|---|---|
  | 没有统计 | 82.0、79.0 | 87.3、84.6 |
  | 分析过 | 78.6、75.0 | 80.7、79.2 |
  | 通用的计划 | 80.6、87.3 | 83.9、85.7 |
  | 一次调用 | 153.7、134.3 | 145.0、133.0 |

  没有差别（三个 `GrepCandidates` 的阶段都在 75–87 毫秒，主持者的决定 30）。

## 组合根的各份

照 6.5 以 `-race` 列出（`BOOTSTRAP_FIRST`、`BOOTSTRAP_THIRD` 同 [S1 的记录](P1-S1-mode-template-review.md)）：头上 426 个测试，三份 168、119、139；起点与 S2 的组合根测试之前（`ee64833c`）都是 420 个、168、117、135。六个新的照 7.5 的表：第二份 `TestModeComesBeforeTheLock`、`TestTheDowngradeEndsTheContract`，第三份 `TestTheGuardRefusesWhatThePlanLetBy`、`TestTheModeGuardReachesEveryMCPWrite`、`TestTheModeGuardReachesEveryPath`、`TestTheProtectionAndADowngrade`；第一份没有新的（第一份的 `TestEveryMCPToolHasItsCodes` 只多几个码）；改过的 `TestPermissionMatrix` 在第二份、`TestTheMCPToolsErrorsCarryTheirCodes` 在第三份（审查 C 核对）。修复一轮没有加组合根的测试。

本机各份单独、竞态下，经 `heavy.sh`，测试二进制的时间（秒；`make test-go-bootstrap-N`，分支与起点交替；各次开始时负载 2.0–6.1，`logs/measure.log`）：

| 份 | 起点 `45d086a1` | 建成 `cfa1c608` |
|---|---|---|
| 第一 | — | 271.1、252.9，平均 262.0 |
| 第二 | 148.3、155.5，平均 151.9 | 164.6、156.0，平均 160.3（+8.4） |
| 第三 | 189.2、195.8，平均 192.5 | 192.3、191.5，平均 191.9（−0.6，在波动之内） |

- 第一份不在起点上量：那里没有 S2 的测试。修复一轮不重量：它只加了三个测试的几行与一处注释，没有组合根的新测试。
- CI 与本机之比（建成的运行的测试一步对建成时的平均）：1.67、2.48、2.03。
- CI 的测试一步（下面"门禁与 CI"）：建成 7:18、6:37、6:30，修复一轮 6:15、5:46、6:14，都在 7:30 之下；两次的平均 6:46.5、6:11.5、6:22，第二、三份在 7.5 到 P1 结束的估计（约 7:00–7:15、6:45–7:00）之下。第一份没有 S2 的测试，是离线最近的一份：同样或更早的代码上它跑过 4:59、5:04、6:49、7:12，7:18 是运行机（审查 C）。照 6.5 的规则什么也不挪：没有分份，没有提高 `-timeout`，CI 的文件与 `Makefile` 没改。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`，机器同时有别的会话在跑测试）：

- 建成时（`cfa1c608`）：`make lint gen-check knip build` 通过（golangci-lint 两次 0 个问题，`gen-check` 没有差异，knip 干净，二进制建成）；`make test` 900 秒（竞态的一份与组合根的三份同时：三份的包 211.5、269.1、358.4 秒；之后成本与守卫的两行；`test-web` 166 个文件、3,349 个测试）；`make test-go-short` 54 秒（91 个包）；`make e2e` 286 个 37.0 秒。4 个提交各自在一个检查的工作树里 `go build ./...`、`go vet ./...`、碰到的包的 golangci-lint（0 个问题）、`page/app` 与 `page/domain` 的 short 测试（第 2 个另有 `access/domain`，第 3 个另有 page 与 asset 的 HTTP 适配器）；碰到的要库的包的 `-short` 经 `heavy.sh`（第 1 个：notebook、page、page 的 postgres 适配器与 `bootstrap`；第 3、4 个：page 与 `bootstrap`）；第 3 个另过网页的离线的 `pnpm install`、三个包的 `check:types`、oxlint（0 个警告）、oxfmt 与 vitest 的 `problem-messages.test.ts` 与 `src/i18n`（3 个文件、1,093 个测试）。第 3 个提交的库上的测试（经 `heavy.sh`，竞态）：`bootstrap` 的 20 个点名的测试 42.1 秒，page 5.0、`page/app` 30.7、`page/domain` 132.2、page 的 HTTP 适配器 92.8、asset 的 HTTP 适配器 36.2、notebook 7.5 秒，LW1 与 LW2 的四个 3.0 秒。审查 C 在每个提交上另跑整个模块的 lint、tidy 与整个 short 测试（上面"审查方式"）：都通过。
- 修复一轮（`7834bd70`）：`make lint gen-check knip build` 通过（golangci-lint 两次 0 个问题，`gen-check` 没有差异，knip 干净）；`make test` 第一次 244 秒以 2 退出（上表"流程"：一份的容器没起来，不是我们的代码），再跑一次 890 秒通过（三份的包 192.4、242.4、323.3 秒；`test-web` 166 个文件、3,349 个测试）；`make test-go-short` 53 秒；`make e2e` 286 个 36.9 秒。4 个提交各自 build、vet、page 与 `bootstrap` 的 golangci-lint（0 个问题）、`page/app` 与 `page/domain` 的 short 测试，第 3、4 个另经 `heavy.sh` 跑 `bootstrap` 与 page 的 `-short`。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 运行 | 头 | 竞态 | 组合根一 | 组合根二 | 组合根三 | 守卫 | 成本 | short | web | e2e | 镜像 | 共 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 40123373818（建成） | `cfa1c608` | 8:01（7:41） | 7:41（7:18） | 6:56（6:37） | 6:51（6:30） | 12:28（12:10，失败） | 11:48（9:00） | 2:32（2:13） | 3:36（3:03） | 跳过 | 跳过 | 12:31 |
| 40131529634（修复一轮） | `7834bd70` | 7:58（7:37） | 6:44（6:15） | 6:04（5:46） | 6:32（6:14） | 8:03（7:45） | 12:20（9:20） | 3:18（2:56） | 4:04（3:31） | 4:28（3:36） | 1:18 | 16:53 |

- **建成的运行红在守卫那一份**：M9 的 grep 的成本守卫 `TestGrepOfAHundredThousandCandidatesTakesItsTime`，"GrepCandidates of 100,000, without statistics: [195.8 ms 388.3 ms 393.1 ms] … a median of 388.3 ms, over 270 ms"；同一个测试的别的阶段：分析过的 183–190 毫秒、通用的 189–417 毫秒，"a call over them" 393–422 毫秒（它的 1 秒的界照过）；环境的注记里作业期间有六个 postgres 的容器。e2e 与镜像要服务端的各份，因此跳过。本机 `make test` 里同一个守卫照过（测试 5.38 秒）。
- **不是 S2 的**（审查 C 的诊断，主持者的决定 30）：S2 不碰这个测试计时的 `store.GrepCandidates`（page 的 postgres 适配器的一条语句，S2 不改它，种子是普通的 SQL）；grep 的路径上 S2 只把 `decideNotebook` 改成包着 `decidedNotebook`（同样两次读，`WorkspaceOf`、`Authorize`，没有 `StateOf`、单元与计划）；`startGrepGuardApp` 的接线多了模式的守卫，读的运行碰不到它；守卫那一份的 `-run` 不调 `prepareMatrix`；测试二进制多六个测试，碰不到一条语句的时间；本机上头与起点量得相同（上面"实测"）。main 的守卫一份的测试一步在没改的代码上从 7:57（`45d086a1`）、8:48（`78aab3cf`，只有文档）到 12:35（`f92ecac7`，通过），S1 的修复一轮的运行 12:27 也通过，分支的 12:10 在这之间；组合根的各份在没改的代码上一样起伏（`45d086a1` 4:59、6:15、5:09，`78aab3cf` 5:04、4:30、4:18），整个运行慢。这一阶段的 CI 与本机之比约 388/80 ≈ 4.8，下一阶段约 2.3：同 M9 因运行机慢把调用的界提到 1 秒（`0f0ccbf9`）的样子。修复一轮的推送重跑了它，全绿。
- **它再失败时**（另一个分支或 main 上 `GrepCandidates` 的中位数又过了 270 毫秒；决定 30，[00-M10](../00-M10-design.md)"写给后面的 M"）：照 M9 给它的兄弟的界的改法，以量过的 CI 的中位数为据，不是重跑：把 CI 的中位数写进界的注释、放宽 `grepCandidatesBound` 到比最慢的 CI 的中位数还有余量（同 `grepCandidatesCallBound` 的注释），或计时五次取中位数，两次慢的调用不再定结果；不在 M10 的哪个 Step 里改。

组合根三份的测试一步两次都在 6.5 的 7:30 之下；守卫那一份的 12:10 与 7:45 在它的 30 分钟之内。分支的两次的测试一步的平均见上面"组合根的各份"。
