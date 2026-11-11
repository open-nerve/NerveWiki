# M11/P3 S1（审阅，服务端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p3-s1`（实施的 7 个提交：第四份的四个测试挪到第六份、审阅的测试进第八份 `f86b7a6d`；审阅的判定（`ReviewOf`、`JudgeRoots`、`ApprovalNotReady`）与 notebook 的批准的写 `9432d60a`；批准的来源哈希的重建（`RebuildSources`、`SourceRebuildHeads`、`page.NewSourceRebuild`）`8abb2a1d`；批准、驳回、退回（两条 SQL、三个用例、契约、HTTP、接线、矩阵、组合根的三个新测试、流的九步与两处交错）`34d77c98`；第 181 条（撤销不把节点放回根下保留的标题）`63cc0b84`；批准的成本守卫与判计划的短测试、costs-1 的时限 `bba17208`；e2e 的 UP6–UP8 `d7b85bae`；从 `main` 的 `75f5d077` 开始，审查时在 `d7b85bae`，68 个文件、加 5,866 行、减 65 行）；修复一轮在它之上加 4 个提交（守卫与它的短测试 `9c637540`；只改测试 `cc751dca`；UP7 `9dac9244`；`Makefile` 的注释 `a44602b0`），头 `a44602b0`（6 个文件、加 210 行、减 108 行） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（前几个 Step 的三个工作树挪到 `d7b85bae`），重的都经 `heavy.sh`，先读设计与代码、自己形成判断之后才看实施者的报告（报告里的数、变异表、16 条偏离与决定 295–297 都当要核实的说法）：服务端行为的正确性（A：判定、批准的次序与期限、驳回、退回、两条 SQL 与约束、重建、第 181 条、交错与事件、组合根的投影；单元与真库的测试，组合根四个新测试经 `heavy.sh` 跑一轮；`mut.py` 十条，a1、a2 存活）；成本、计划、分份与变异的抓手（B：`-list` 照 `Makefile` 自己归份；判计划的规则 `approveCostScans` 喂 5 个手写的计划、对规则的 6 个子条件各做一条变异；决定 296 的探针（每条被判的语句第一次真跑之前在同一连接上 `EXPLAIN`，与守卫判的逐条比）；建议的改法试跑整个守卫；重跑 o1、16b，另加 16c；从存下的 25 次 costs-2 日志取发起守卫每格的中位数与 runner 的 CPU）；契约、HTTP、MCP、矩阵、e2e 与文档稿（C：`gen-check`、`lint-web`、`knip`、`test-web`；三个模块的 `adapter/http` 与两个 `domain` 整包；在改写之后的 2–6 号上逐个提交重跑一轮；经 `heavy.sh` 跑组合根的契约测试、矩阵的三行与 UP6–UP8 `--repeat-each=4`；三条变异；判实施报告第 7 节各条进不进 3.23） |
| 日期 | 2026-11-12（实施、审查、一轮修复与一轮修复核对） |
| 结论 | Important 0。Minor 2：判计划的规则对 `nodes_pkey` 不看条件，按键序整读 `nodes` 在任何一条被判的语句里都放过，短测试有六个子条件各自去掉照过（P3S1B-M1，P1/S2 的 S2A-N4 在新守卫里重现）；决定 296 在删基线之后判计划，`SetSourceHashes` 已写完 47,500 行，判的不是生产那一刻的状态，探针 84 条里 2 条与真跑时不同（P3S1B-M2）。Nit 12（去重之后；三份合计 14）：测试有几处没钉住（批准交给 `JudgeRoots` 的来源根、退回的"别的升级的行"、批准一格的 `checkLinks`、`checkSearch`、驳回的基线复原是空的、退回的答复不是整值、`ApproveTimeout` 的 `* 3 / 4` 只在 costs-1 看得见；P3S1A-N1–N6，A-N3 = C-N3）、`EXPLAIN` 出错时报的是"批准答 500"（P3S1B-N1）、UP7 的说法比断言强（P3S1C-N4）、`Makefile` 的计数注释停在 1 号提交时（P3S1C-N5）、文档稿四处（`From`/`To` 记在流的测试名下，P3S1A-N7 = C-N2；47,500 与 5 万，C-N1；13.1 的两句措辞，C-N6）。Q 1：3.9 s 的界在 7763 上会不会偶发地红（P3S1B-Q1）→ 主持者的决定 298。三位都没有查出用户或数据会碰到的问题；实施者的 16 条偏离都判对。修复一轮（4 个提交，头 `a44602b0`）照 `fix-1.md` 改了代码与测试的全部发现，12 条变异改前活、改后死，分支 CI 一次全绿；新的偏离 7 条，主持者都接受。修复核对一轮：三位把各自的发现都关上，文档稿里的四条落对；新的 Nit 两条，P3S1B-N2（守卫失败时的诊断）不改，P3S1C-N7（文档稿漏了 A-N4 的"先退回"）在本提交；没有 Important、Minor 留下，不再有修复轮 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：P3S1A-N1–N7，P3S1B-M1、M2、N1、Q1，P3S1C-N1–N6。同一件事合为一行：A-N3 = C-N3；A-N7 = C-N2。修复核对一轮的新发现 P3S1B-N2、P3S1C-N7 列在后面。修复一轮照主持者的修复说明（`p3s1/review/fix-1.md`，提交 1 是守卫、2 是测试、3 是 e2e、4 是注释）在 `d7b85bae` 之上加新提交，普通推送、不改写已推的历史，每个单独为绿。审查时的代码位置是 `d7b85bae` 的；服务端的模块在 `server/internal/modules/` 之下，`bootstrap/` 在 `server/internal/` 之下） | — |
| P3S1B-M1 | 中（测试与守卫） | `bootstrap/upgrade_approve_cost_test.go:616-619`：`nodes` 的扫描只要索引是 `nodes_pkey` 就 `ok` 且 `keyed`，不论 `Index Cond`。没有条件的 `Index Scan using nodes_pkey`（Merge Join 或按 `id` 排序时按键序读整张 `nodes`，全部笔记本的行）在任何一条被判的语句里都放过，嵌套循环内侧逐行整读也放过，`SourceRebuildHeads` 的"读头只经键"一条也放过（`keyed` 为真）；只有 `SetSourceHashes` 因 `once > 1` 偶尔挡住。B 的探针喂 5 个计划（`SourceRebuildHeads` 的 Merge Join 与内侧逐行、`LintNodeMetadata` 外侧无条件、`RootNodesByKey` 单独一个、`SetSourceHashes` 内侧逐行 `id = ANY($1)`），全部放过。短测试 `TestTheApprovalGuardJudgesItsReads`（`:658-705`）的注释说"规则写错要在短测试里先看见"，而六个子条件各自去掉九格照过（r1 `nodes_notebook_id_*` 不要求 `notebook_id = `，r2 `page_contents_pkey` 不要求 `node_id = `，r3 `asset_blobs_node_id_key` 不要求 `node_id = `，r4 `upgrade_nodes` 怎么读都 `ok`，r5 `upgrade_nodes` 内侧不要求 `node_id = `，r6 去掉通用的"内侧逐行要按外行的键"）。真的计划今天都不撞这些（守卫四轮绿），是"能过却抓不到一类回归"；P1/S2 的 S2A-N4 对 `page_contents` 补过同一种，同一教训在新守卫里重现，定 Minor | 修复一轮改（提交 1，`9c637540`）：`nodes_pkey` 只在条件里有 `(id = ` 时才认（B 试过：短测试九格照过，前四个探针被拒，整个守卫照样绿）；内侧逐行的 `nodes` 读条件含 `= ANY` 的拒绝，前提是真的计划照样绿，否则不加、在报告里写明（加了：守卫在真的计划上照样绿）；短测试照 S2A-N4 的做法为每个子条件补一格反例，r1–r6 与新的 `nodes_pkey` 条件各自去掉时都有一格失败（短测试 9 → 19 格，另一格正例"一次按键读一组节点"放行、钉住 `= ANY` 那一条只管内侧；r1–r6 改前活、改后死，r7（`nodes_pkey` 不看条件）、r8（去掉"内侧不许 `= ANY`"）死）；设计 7.1 与第 297 条随之写（本提交）。修复核对一轮 B 关上：第一轮的 5 个探针都被拒，另加的 r9（`= ANY` 连只读一次的也拒）被正例抓到 |
| P3S1B-M2 | 中（守卫） | `upgrade_approve_cost_test.go:518-543`、`:548-588`：决定 296 在第一次批准的事务里、`DeleteBaseline` 之后 `EXPLAIN` 记下的语句；那时 `SetSourceHashes` 已写完 47,500 行（非 HOT，`nodes` 的堆与每个索引都长了，部分索引 `nodes_notebook_id_kind_source_hash_idx` 多了这一本的 47,500 条），被判的九条里有八条在写之前或写之中真跑。没有统计时规划器按当前的页数估行数，统计过时时也按当前页数放大 `reltuples`，判的是生产那一刻不在的状态。B 的探针（`b/probe296.log.gz`）在每条第一次真跑之前、同一连接上 `EXPLAIN`，两个种子 × 三种统计 × 两种方式 84 条：82 条形状相同，2 条不同（计时种子"从未分析"的 `ForgetSources`，真跑时 `nodes_notebook_id_name_key_idx`、守卫判的是 `nodes_notebook_id_kind_source_hash_idx`），都合规则，这一次没有判错 | 修复一轮改（提交 1，`9c637540`）：`EXPLAIN` 挪到 tracer 的 `TraceQueryStart`，每条被判的语句第一次真跑之前、同一连接上判（自定义带它的参数，通用经 simple protocol），用时照旧从 `elapsed_ms` 里减；`SHOW plan_cache_mode` 留在包住 `DeleteBaseline` 的那一层；第 296 条改写为"每条被判的语句在它第一次真跑之前、同一连接上判"（00 号文档第 11 节、设计 7.1，本提交）。B 的那两条 `ForgetSources` 现在判的是真跑时的 `nodes_notebook_id_name_key_idx`；修复核对一轮 B 关上 |
| P3S1A-N1 | 细节（测试） | `wiki/app/approve_upgrade.go:147`：闸门 2 的第二个合取项"来源根不是根下的 `AGENTS`"在批准里靠 `JudgeRoots(roots, up.SourceRootID, reserved)` 的第二个参数；单元的闸门 2 一格只置 `RootPage = false`，组合根第 4 格只清 `AGENTS` 的正文。a1 把它换成 `uuid.UUID{}`，`TestApproveUpgrade` 照过。只有缺陷才到得了（审阅中冻结） | 修复一轮改（提交 2，`cc751dca`）：`TestApproveUpgrade` 的闸门表加"来源根是根下的 `AGENTS`"一格（A 给的写法），a1 改前活、改后死；修复核对一轮 A 关上 |
| P3S1A-N2 | 细节（测试） | `wiki/app/send_back_upgrade.go:89` 的 `case !ok \|\| up.ID != state.Upgrade.ID \|\| up.State != state.Upgrade.State`：批准有"a row of another upgrade"一格，退回的拒绝表没有；a2 改成 `case !ok:` 照过。第二道防线还在（`SetState(…, up.ID, …)` 改不到一行，照样答缺陷），只是两处同写的判断只在一处测 | 修复一轮改（提交 2）：`TestSendBackUpgrade` 的拒绝表照批准加同一格，a2 改前活、改后死；修复核对一轮 A 关上 |
| P3S1A-N3 = P3S1C-N3 | 细节（测试） | `bootstrap/upgrade_approve_test.go:217-218`：设计 7.1 第 2 格列了 `assertSourceHashesMatchRebuild`、`checkPages`、`checkLinks`、`checkSearch`，实施只调了前两个，报告第 7 节也没列这处不同；整个 `TestUpgradeApproval`、`TestUpgradeTrashReservedTitles` 都没调后两个。`TestUpgradeReviewInterleavings` 在全部交错之后对整个库调了一次（`upgrade_review_test.go:349-352`），库里有"先批准"的几本，覆盖还在；设计 7.2 写"每个交错结束时"。链接的索引不随模式存、`tm.reviewed` 没有链接，看不出实际的缺口 | 修复一轮改（提交 2）：那一格末尾补 `checkLinks`、`checkSearch`；7.2 的"每个交错结束时"照实写成"全部交错之后"、记进 3.23（本提交）；修复核对一轮 A、C 关上 |
| P3S1A-N4 | 细节（测试） | `bootstrap/upgrade_review_test.go:208`：`tm.reviewed` 的升级里没有改过任何原有页，发起只写了 `AGENTS`；"the note as it was"（正文等于 `cleanNote`）不靠回滚也成立，真正证明回滚做了事的只有"`AGENTS` 不在"。设计 7.1 第 4 格的"基线复原（发起时的 frontmatter）"要有一处被改过、再被放回；实施报告第 7 节第 3 条把它改成了现在的写法。驳回与放弃共用 `rollBack`，正文的写回由 P1 的 `TestUpgradeRollback` 钉着 | 修复一轮改（提交 2）：审阅中冻结 agent 的写，驳回之前先由 dana 退回，alice 的 agent 再经 MCP 改 `Note` 的 frontmatter（仍干净，照"sent back"那一格的写法），erin 再交，然后驳回，之后断言正文回到 `cleanNote`（修复说明写"改了再交、再驳回"，没写先退回，是实施者的新偏离第 3 条；文档稿的初稿同样漏了，P3S1C-N7）；n4（回滚的 `RollbackState` 不出正文的版本）改前活、改后死，失败在三处：任务以 `internal` 停、行留在 `rolling_back`、正文仍是改过的；修复核对一轮 A 关上 |
| P3S1A-N5 | 细节（测试） | `bootstrap/upgrade_review_test.go:151-156`：设计 7.1 第 2 格写"答复整值断言"，退回一格只看 `state`、`id`、提交的两项、`report`、`ending`、`rollback_job_id`；批准一格是拿批准之前的读改 `state` 之后整个比较的（`upgrade_approve_test.go:174-180`）。投影由 `TestTheUpgradeIsAnswered` 整值钉着，`view` 由单元整值钉着。附带：码的次序前五格之后没有核"行与笔记本不变"，事务保证了它，可不改 | 修复一轮改（提交 2）：照批准的做法与 `getUpgrade` 之后的读整值比较；附带的一条不改。实施者保留逐项断言、整值比较加在后面（新偏离第 4 条）；修复核对一轮 A 关上 |
| P3S1A-N6 | 细节（测试） | `bootstrap/wire.go:294` 的 `ApproveTimeout = RequestTimeout * 3 / 4`：期限三格经 `wiring.wiki` 把它换成 2 s（`upgrade_approve_test.go:296`），竞态的八份里没有一格看它的原值，只有守卫的 `approveCostApp` 断言 11.25 s（o6）。发起的 `StartTimeout` 也只在它的守卫里核（设计照的先例），`SubmitTimeout` 则在竞态份里核 | 修复一轮改（提交 2）：换期限的钩子里先断言 `d.ApproveTimeout == 3*time.Second`（测试应用的请求 4 s 的四分之三），再换成 2 s；o6 在第八份里改前活、改后死（"the approval's deadline is 4s, want 3s"）；修复核对一轮 A 关上 |
| P3S1A-N7 = P3S1C-N2 | 细节（文档） | 实施报告第 11 节 M5-events 一行与设计 7.1 的 `TestUpgradeStatesReachTheStreams` 一段都说流的批准一步看到"`From standard`、`To llm_wiki`"。`EventMode` 只有 `workspace_id`、`notebook_id`（`api/modules/events.yaml:223-234`），`notebookEvents.ModeChanged` 发 `{}`（`bootstrap/events_registrants.go:63-66`，M10-mode 移交第 3 条"不把前后模式另放进帧"），流的测试只数帧（`upgrade_start_test.go:481-512`）；`From`、`To` 只在单元 `TestApproveUpgrade` 里对 `published` 整值断言（`review_upgrade_test.go:123-125`）。没有行为上的缺口 | 本提交：移交写"批准一帧（帧里没有前后模式；`Publish` 的 `From standard`、`To llm_wiki` 由 `TestApproveUpgrade` 整值断言）"，设计 7.1 那一处照实改、记进 3.23；修复核对一轮 C 核过，落对 |
| P3S1B-N1 | 细节（守卫） | `upgrade_approve_cost_test.go:410-418`、`:542`：`EXPLAIN` 是 SQL 错误时事务进入失败状态，`Rows.Approve` 随之失败，批准答 500；`approveCostApprove` 先判 `status != 200` 就 `Fatalf`，`PlanErr` 里真正的原因没有打出来 | 修复一轮改（提交 1）：先判 `plans && PlanErr != nil`，再判 `status != 200`；修复核对一轮 B 关上（新的 P3S1B-N2 见下） |
| P3S1C-N1 | 细节（文档） | 实施报告第 11 节 13.4 第 8 条的改稿写"47,500 个来源"：守卫的这本笔记本 `raw` 下有 500 × (1 + 90 + 5 + 4) = 50,000 个来源，带哈希的是 47,500 个（空页不带）；设计 2.1 第 2 条、第 280 条、`Makefile:389` 的注释、6 号提交的说明、守卫里笔记本的名字 `"Fifty thousand sources"` 都写 5 万，只有守卫的常量 `approveCostSources`（`:44-46`）把要写的哈希叫作来源 | 本提交：13.4 第 8 条写"5 万个来源（47,500 个要写的哈希……）"，"约占四分之一"照稿改为 22%；修复核对一轮 C 核过，落对 |
| P3S1C-N4 | 细节（e2e） | `e2e/stories/upgrade/up7-approve.spec.ts:89-97`、`:112-115`："hashed as their bytes"比的是 `n.source_hash = coalesce(c.content_hash, b.sha256)`，库里存的正文哈希与附件哈希；漏写一页、漏写附件、互换两行都会失败，那两列本身算错看不见。真正的不变式在组合根由 `assertSourceHashesMatchRebuild` 守着 | 修复一轮改（提交 3，`9dac9244`）：以 `node:crypto` 对 `"The paper.\n"` 与 `pngBytes` 各算 SHA-256，与 `source_hash` 比；测试名照实；修复核对一轮 C 以变异核过（上传时的 SHA-256 每块前多喂一个字节，库里两列一起错：旧的 UP7 照过、新的失败），关上 |
| P3S1C-N5 | 细节（注释） | `Makefile:265`（"当前 547 个顶层（…M11/P3/S1 的第一个提交时的 …）145/50/120/120/61/15/27/9"）、`:321`（"开这一行时这两个名字还没有测试"）：写明了是 1 号提交时的数，不算写错；头上是 551 个、…/15/28/12 | 修复一轮改（提交 4，`a44602b0`）：两处改为头上的数（另写第七、八份本机两轮的数，新偏离第 5 条）；修复核对一轮 C 重跑 `-list` 核过，关上 |
| P3S1C-N6 | 细节（文档） | 设计 8.6 与实施报告第 11 节 13.1 的两句："审阅的三个动作与批准在笔记本行的锁下写"读起来像四个动作；"回收站与后台的恢复照旧加序号"会被读成后台撤销（`startRevert`），而它恰是被拒的那一个（`CheckContract` 只对 `ChangesetRevert` 的 `OpRestore` 判，`page/domain/contract.go:140-141`；回收站的同步与后台恢复都是 `ChangesetTrashRestore`） | 本提交：13.1 第 1 条写"审阅的三个动作（批准、驳回、退回）在笔记本行的锁下写；批准在同一个事务里……"，第 8 条写"回收站的恢复（同步与后台）照旧加序号"；修复核对一轮 C 核过，落对 |
| P3S1B-Q1 | 问题（给主持者） | 3.9 s 的界在 EPYC 7763 上会不会偶发地红。B 的依据：同一条定界规则的发起守卫（3.6 s，本机两轮较大的中位数的 3.08 倍）在存下的 25 次 costs-2 里，7763 八次每次六格最大的中位数 2.26–2.64 s（本机的 1.93–2.26 倍，最紧时余量 1.36 倍），从没红过；8370C 两次、9V74 十一次、9V45 四次、8573C 一次都在其内。批准守卫在分支 CI 的 8370C 上 2.25–2.51 s，是本机的约 1.95 倍，与发起守卫在 8370C 上的倍数相当；照发起守卫 7763 对 8370C 的比，批准守卫在 7763 上最慢的一格约 2.3–3.0 s，余量约 1.3–1.7 倍。要红须一格三次批准中两次过 3.9 s，只会来自整台 runner 持续地慢。B 判不会经常红，建议不改、写下改界的触发条件 | 主持者的决定 298（下面）：不改，写下条件；守卫里界的注释写明（提交 1）；修复核对一轮 B 关上 |
| P3S1B-N2 | 细节（守卫，修复核对一轮） | `upgrade_approve_cost_test.go:416-417`：批准在跑到被判的语句之前就失败时（例如 503 或别的 500），`end()` 给出"the approval ran no …"，`PlanErr` 先报，消息里只有状态码、没有答复的正文，真正的原因看不到；改前这种情形报的是状态与正文 | 不改（主持者）：只影响守卫失败时的诊断，不影响判定 |
| P3S1C-N7 | 细节（文档，修复核对一轮） | 文档稿的 3.23、7.1 `TestUpgradeReview` 第 4 格、7.3 加的一行与本记录 A-N4 一行写"驳回之前 alice 的 agent 改 frontmatter、再交、再驳回"；`tm.reviewed` 的升级已在审阅中，审阅中冻结 agent 的写，`cc751dca` 实际是先退回、agent 再改、erin 再交、再驳回（`upgrade_review_test.go:204-220`），读的人会以为审阅中 agent 还能写 | 本提交：几处都补上"先退回" |

## 主持者的决定

负责人可以改判（295–297 在 S1 的实施中定下，审查时照实施者的做法审，做法不对是发现；298 在第一轮修复说明里定下）：

295. **守卫的另几本是 LLM Wiki，来源在各自根下的 `raw` 里**（实施报告第 7 节第 9 条）：每本 `raw` 收 45,000 页与 2,000 个附件、带哈希，另 4,999 页在根下；基线行用一个没有行的升级 id。B 判合 7.1 的意图，没有改。
296. **守卫在每条被判的语句第一次真跑之前、同一连接上判它的计划**（实施报告第 7 节第 10 条；照 P3S1B-M2 改写）：实施时定的是删基线之后判，审查之后改为在 `TraceQueryStart` 里判。
297. **判计划时内侧逐行的 `nodes` 认条件里的 `(id = `**（实施报告第 7 节第 11 条）：B 判这一条本身不会放过丢了笔记本键的读，洞在 `nodes_pkey` 不看条件（P3S1B-M1），修复一轮收紧。
298. **批准守卫的界 3.9 s 不改**（P3S1B-Q1，2026-11-12）：合并之后看两次 `main` 的 costs-1；任一次在 EPYC 7763 或更慢的 runner 上任一格的中位数过 3.25 s（余量不到 1.2 倍），就照负责人 2026-11-06 的话把界调到 4.5 s（本机较大中位数的约 3.5 倍，仍不到 `ApproveTimeout` 11.25 s 的四成）；不调量级、不做优化。守卫里界的注释写明这个条件。依据：B 的推算（上表 P3S1B-Q1）。

修复一轮的处置照 `fix-1.md`：三份审查去重之后，代码与测试的发现都改，在 `d7b85bae` 之上加四个提交（守卫与它的短测试；只改测试；e2e；只改注释），每个单独为绿，普通推送；变异先在 `d7b85bae` 上跑、要活（B 的 r1–r6 与"`nodes_pkey` 不看条件"、A 的 a1、a2、A-N4 的"回滚不写回正文"、o6 在第八份），改后要死；逐个提交跑 `make lint-go`、`make gen-check`、`make test-go-short`，提交 1 另跑守卫一轮，提交 2 另跑 `wiki/...` 的单元与 `TestUpgradeApproval`、`TestUpgradeReview`，提交 3 另跑 `make lint-web` 与 UP7；头上 `make lint gen-check knip build` 与经 `heavy.sh` 的 `make test-go-short`，`make e2e` 只跑 UP7 或整轮，只改了测试与注释、不必整轮 `make test`；e2e 卡在 apt 镜像或红在 L1 时照第 277 条只重跑 e2e。文档的发现（A-N7 = C-N2、C-N1、C-N6）与 C 对第 7 节进不进 3.23 的判断在本提交。

修复核对一轮之后的处置：修复一轮的 7 条新偏离都接受（3.23）；P3S1B-N2 不改（只影响守卫失败时的诊断，不影响判定）；P3S1C-N7 在本提交。没有 Important、Minor 留下，不再有修复轮。

## 接受与推后的

- **接受**：
  - 等价的变异：B 的 b-16c（保留 `nodes` 的 `LATERAL … LIMIT 1`，`page_contents` 改为普通的 `LEFT JOIN`）：24 个计划与原样相同，只少 `Memoize` 下的一个 `Limit`，在这两个种子上等价；第二个 `LIMIT 1` 是防守，不是计划的成因。7.3 第 2 行设计点名的"只对调两支"是等价的，实施者改的是把报告一支挪到期限之后，抓到。
  - 未核实的怀疑：A："7 之退回（二）"在更慢的 runner 上：从收到 `held` 到持住笔记本行只有一次 `Begin`、一条 `SELECT … FOR NO KEY UPDATE` 与一次读，要在 `Runs.Do` 放弃（约 1.5 s）之前完成，判断余量很大；第二个事务的后端一直排在锁上，依据设计审查 A 的核对，没有另外复测；决定 295–297 的判法是 B 的范围。B：生产的通用计划是缓存的（`SourceRebuildHeads`、`ForgetSources` 不经 `Planned`，同一连接上第六次起可以换通用计划，那份建于更早的请求；`SourceRebuildHeads` 的计划不随统计，风险小）；写重的段在 7763 上的倍数没有样本（批准的大头是约 4.75 万行非 HOT 的 `UPDATE`）；第四份在 7763 上的数要等两次 `main`；"内侧逐行 `id = ANY($1)`"规划器会不会真出，没有逼出。C：`submitUpgrade` 新说明里的"an upgrade sent back … meanwhile"在单个进程里大概到不了（只在多个实例的时钟不一致时），P2 原来那句同样，是设计（P3C-N4）要的说法，不改；矩阵的 `left` 按行循环，某格的升级行不存在时什么也不判，活的笔记本由 `upgradedBy` 一定建出一行，没有为它写变异。
  - 核对过、没有问题的（三位各自的第 4、5 节）：`ReviewOf` 的次序与边界、`JudgeRoots` 与原 `JudgeUpgrade` 逐字等价、批准的次序与期限（`errors.Is` 认 503，回滚失败时 `tx.go` 以 `errors.Join` 包住仍认得出）、决定 293 的 error 一行、三种 info 的时机、驳回的 `ClientOf` 与结局、退回的写与 `upgrade_by_id` 不变、两条 SQL 满足的约束与 `TestTheReviewsRows` 的每个合取项、重建的次序与零字节的规则、`SourceRebuildHeads` 的整值、并发（写入单元取笔记本行 `FOR SHARE`）、第 181 条的范围、交错 7 格与事件的帧数（A）；种子与四个模板、三种统计、`plan_cache_mode` 与 `SHOW`、被判的语句与"没出现就失败"、决定 292、`EXPLAIN` 的用时、启动时的作业、分份（551 个，新模式下每个恰在一份）、挪前挪后与 7.5 的算术、costs-1 的 `-timeout 15m` 与 45 分钟、o6 由守卫看见（B）；契约三个操作的说明与码序、`api/openapi.yaml` 的三个路径、第 181 条的契约与两条撤销的路、生成物、HTTP 处理函数与码的测试（`apitest.Main` 的"声明的码都有人答"三包都过）、MCP 不改、矩阵三行的变体与 `left`、e2e 的夹具与三个故事（UP6 确定地看到回滚在排队）、提交的范围与说明、文档稿与代码（C）。
  - 过度设计：三位都没有看到。A：`RootGates.Failed` 借 `UpgradeGates.Failed` 拼一个其余全过的结构，比另写一个列表绕一点，但次序与键只有一处定义，可取；`reviewFake` 近 400 行，要它才能断言"什么也没写"。B：22% 的只核对计划的种子在"读头只经键"加上之后对 `SourceRebuildHeads` 与计时种子重复，但它另判重建的其余四条在两成以上的占比下的计划，是决定 292 要的，本机约 30 秒。C：`post*` 三个帮手各配一处 409，是 `knip` 与 P3C-N8 共同要求的。
  - 流程（照实写下）：推送之前改写过一次 2–7 号（变异第一轮的 o1、16a 补测试：2 号 `260356f2` → `9432d60a`，6 号 `232cd613` → `bba17208`，逐个 `cherry-pick`，不用交互的 rebase，旧头 `4b852247` 与新头只差两个测试文件），旧的都没推过；2–5 号是先把全部改动写完之后按提交拆的（整份备份在 `final/`，5 号之后与备份逐字节相同）；实施者在改写之后只重跑了 2、6 号与头，3–5 号由审查 C 补跑一轮（"门禁与 CI"）。说明写进文件再 `git commit -F`；范围照实际的部件写；每次提交之前看过 `git status`；没有推 `main`，没有 force-push。
  - 修复一轮与 `fix-1.md` 不同之处 7 条（实施者修复报告第 6 节）："内侧不许 `= ANY`"加了；短测试多一格正例；A-N4 先退回再写；A-N5 保留逐项断言；注释多写第七、八份的数；"ran no %s"挪进 `end()`；第 296 条的说法。
  - P3S1B-N2：不改。
- **推后**：
  - 守卫在慢的 runner 上的余量：第 298 条，合并之后两次 `main` 的 costs-1。
  - 第四、六、八份在 7763 上的数：分支 CI 上都落在 9V74，合并之后两次 `main` 为准（设计第 8 节第 1 条）。
  - S2、S3 的部分（playbook 与 prompt `upgrade`；网页的审阅一节、批准之后的跳转、UP7 的页面版本）与 7.3 里它们的各行。

## 偶发

实施、审查与分支 CI 里都没有。分支 CI 的 e2e 一次过，"Install the Playwright browsers" 2:16 就装好，L1 没有出。S1 开工之前 P2 完成那次 `main` 的 e2e 四次被 apt 镜像取消，是外部故障（"门禁与 CI"末段）。

## 反向对照

建成时以 `mut.py` 跑（`NW_SERVER` 指实施者的工作树的 `server`；规格由 `m11/p3s1/mut/gen.py` 生成 `spec.json` 53 条：7.3 的 S1 36 行（服务端的各行，到"去掉第 181 条的改动"为止）46 条，设计列几个测试的各跑一条，记作 a/b；实施者自己的 7 条 o1–o7；`mut/buildcheck.py` 先核对每条的 `old` 恰好出现一次、改后能编译；`mut/run.sh` 先在没改的树上把 24 条不同的命令各跑一次，都 rc=0，再跑 `mut.py`；整轮经 `heavy.sh`，头 `4b852247`，约 6 分钟，输出 `mut/results.txt`）。sqlc 生成的 SQL 改的是生成的 Go 常量本身（10、16、23、32、o3、o4），改坏时没有跑 `make gen*`；组合根的格子以 `-run '^TestX$/^<格>$'` 单独跑。第一轮 51 条抓到，o1、16a 没抓到，都是真的缺口，补了测试（推送之前改写 2、6 号）。第二轮在头 `d7b85bae`（`mut2/`，4 条，在 `verify-rw.sh` 里经 `heavy.sh`，约 4 分钟；先跑三条命令的 baseline，守卫的 baseline 是同一轮的 `guard-r4`，都 rc=0）：o1、16a、16b、o8 都抓到。每轮之后 `git status` 干净、没有 `*.mutbak`。

**7.3 的 S1 各行与实施者自己的**：

| # | 变异（怎样改坏） | 跑的测试 | 结果 |
|---|---|---|---|
| 1 | `ReviewOf` 以 `!at.After` 判期限（边界错一微秒） | `TestReviewOf` | 抓到 |
| 2 | `ReviewOf` 先判提交在不在与期限、再判报告 | `TestReviewOf` | 抓到 |
| 3a | 批准接受 `ReviewStale`（`review == ReviewChecking` 才拒；`Passed` 一支容忍没有报告） | `TestUpgradeApproval/the codes in their order` | 抓到 |
| 3b | 同上 | `TestApproveUpgrade` | 抓到 |
| 4 | 批准不看报告的 `Passed` | `TestApproveUpgrade` | 抓到 |
| 5a | 批准不再核闸门 1–3（`false && len(failed) > 0`） | `TestUpgradeApproval/the gates checked again under the lock` | 抓到 |
| 5b | 同上 | `TestApproveUpgrade` | 抓到 |
| 6a | 闸门不过不记 error（决定 293） | `TestUpgradeApproval/the gates checked again under the lock` | 抓到 |
| 6b | 同上 | `TestApproveUpgrade` | 抓到 |
| 7 | 以"外面的变量非空"认闸门不过（`refused.failed != nil`） | `TestApproveUpgrade` | 抓到 |
| 8 | `JudgeRoots` 的闸门 2 不比 `AGENTS` | `TestJudgeRoots`、`TestJudgeUpgrade` | 抓到（两个都失败；7.3 的第二个 `upgrade_check_test.go:87` 是组合根的，没另跑） |
| 9a | 批准用 `UpgradeNotReady` | `TestUpgradeApproval/the gates checked again under the lock` | 抓到 |
| 9b | 同上 | `TestApproveUpgrade` | 抓到 |
| 10 | notebook 的 `ApproveUpgrade` 不核 `upgrade_state`（生成的常量） | `TestAnUpgradeIsApproved` | 抓到 |
| 11 | 批准不重建哈希 | `TestUpgradeApproval/approved, another tab told` | 抓到 |
| 12 | 重建不先 `ForgetSources` | `TestTheSourceRebuildIsTheInvariant` | 抓到 |
| 13a | 重建跳过缺文件行的附件 | `TestTheSourceRebuildIsTheInvariant` | 抓到 |
| 13b | 同上 | `TestRebuildSources` | 抓到 |
| 14 | 重建不核每块写的行数 | `TestRebuildSources` | 抓到 |
| 15 | 重建的调用换回 `SourceHeads`（决定 292） | 守卫 | 抓到（第一个计时格"没跑过 `SourceRebuildHeads`"） |
| 16a | `SourceRebuildHeads` 的 SQL 改成 `unnest` 连 `nodes`、左连 `page_contents` | 守卫 | 第一轮没抓到；补规则之后抓到（第二轮） |
| 16b | `SourceRebuildHeads` 的 SQL 换成 `SourceHeads` 的形状 | 守卫 | 抓到（第二轮） |
| 17 | 重建以 `%v` 包 store 的错误 | `TestRebuildSources` | 抓到 |
| 18 | 重建给零字节的页写哈希 | `TestTheSourceRebuildIsTheInvariant` | 抓到 |
| 19 | 重建不给零字节的附件写哈希 | `TestTheSourceRebuildIsTheInvariant` | 抓到 |
| 20 | 批准不删基线 | `TestUpgradeApproval/approved, another tab told` | 抓到 |
| 21 | 批准在重建之前 `Publish` | `TestApproveUpgrade`（调用的次序） | 抓到 |
| 22a | 批准不 `Publish` | `TestUpgradeStatesReachTheStreams` | 抓到 |
| 22b | 同上 | `TestUpgradeApproval/approved, another tab told` | 抓到 |
| 23 | 批准的 SQL 不核 `submission_id`（生成的常量） | `TestTheReviewsRows/Approve` | 抓到 |
| 24 | 批准用请求的 ctx、不另设期限 | `TestUpgradeApproval/its own deadline` | 抓到 |
| 25 | 期限的判断不看 `ctx.Err() == nil` | `TestApproveUpgrade` | 抓到 |
| 26 | 批准的 info 在提交之前记 | `TestApproveUpgrade` | 抓到 |
| 27 | 驳回的 info 在提交之前记 | `TestRejectUpgrade` | 抓到 |
| 28 | 退回的 info 在提交之前记 | `TestSendBackUpgrade` | 抓到 |
| 29 | 驳回接受 `upgrading` | `TestUpgradeReview/the rejection's codes` | 抓到 |
| 30a | 驳回记 `abandoned` | `TestUpgradeReview/rejected` | 抓到 |
| 30b | 同上 | `TestRejectUpgrade` | 抓到 |
| 31a | 退回不判 `ReviewChecking` | `TestUpgradeReview/the send-back's codes` | 抓到 |
| 31b | 同上 | `TestUpgradeSubmitInterleavings/7 a send-back while the submission is checked` | 抓到 |
| 32 | 退回的 SQL 不清报告（生成的常量） | `TestTheReviewsRows/SendBack` | 抓到 |
| 33 | 退回的 `taken_over` 用 `at`、不减 `SubmitTimeout` | `TestSendBackUpgrade` | 抓到 |
| 34 | 退回不 `SetState` | `TestUpgradeReview/sent back` | 抓到 |
| 35a | 第 181 条不限 `ChangesetRevert` | `TestUpgradeTrashReservedTitles` | 抓到 |
| 35b | 同上 | `TestCheckContract` | 抓到 |
| 36a | 去掉第 181 条的改动 | `TestUpgradeTrashReservedTitles` | 抓到 |
| 36b | 同上 | `TestCheckContract` | 抓到 |
| o1 | `UpgradeStates.Approve` 不判事务 | `TestAnUpgradeIsApproved` | 第一轮没抓到；补一格之后抓到（第二轮） |
| o2 | `SourceRebuild.Rebuild` 不判事务 | `TestTheSourceRebuildIsTheInvariant` | 抓到 |
| o3 | `SourceRebuildHeads` 不判笔记本（生成的常量） | `TestSourceRebuildHeadsReadTheirKeys` | 抓到 |
| o4 | `SourceRebuildHeads` 不判删除（生成的常量） | `TestSourceRebuildHeadsReadTheirKeys` | 抓到 |
| o5 | 退回的 info 的 `reported` 恒为假 | `TestUpgradeReview/sent back` | 抓到 |
| o6 | 组合根的 `ApproveTimeout` 取整个请求 | 守卫（应用的断言，第一个种子） | 抓到 |
| o7 | 驳回的处理函数调退回的用例 | `TestTheUpgradeIsAnswered` | 抓到 |
| o8 | 守卫去掉"读头只经键"一条（第二轮的对照） | `TestTheApprovalGuardJudgesItsReads` | 抓到（第二轮） |

**审查者的变异与探针**（照各自的命名；"存活"的都成了一条发现，等价的除外；规格与输出在 `p3s1/review/a/`、`b/`、`c/`；每条都先在没改的树上跑过同样的命令）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | a1（批准交给 `JudgeRoots` 的来源根换成零值） | 存活：P3S1A-N1 |
| A | a2（退回的"行不是这次升级的"判断只剩 `!ok`） | 存活：P3S1A-N2 |
| A | a3（退回的 SQL `submitted_at <= taken_over` 改 `<`）、a4（退回的 SQL 去掉状态）、a5（批准的 SQL 去掉状态） | 抓到（`TestTheReviewsRows`） |
| A | a6、a7（提交的第一个事务不拒 `ReviewReported`、`ReviewChecking`） | 抓到（`TestSubmitUpgrade*`） |
| A | a8（批准的重建交笔记本的 id 作来源根） | 抓到（调用的整列） |
| A | a9（去掉第 181 条）、a10（批准时闸门不过不记 error），组合根经 `heavy.sh` | 抓到 |
| B | b-o1（`UpgradeStates.Approve` 不判事务） | 抓到 |
| B | b-16b（`SourceRebuildHeads` 的生成常量换成 `SourceHeads` 的形状） | 抓到：计时种子六格"not by its key"；22% 种子"发起留下的"两格 `page_contents` 的 `Seq Scan`，其余四格"not by its key" |
| B | b-16c（`page_contents` 改为普通的 `LEFT JOIN`） | 存活，等价（"接受与推后的"） |
| B | r1–r6（判计划的规则的六个子条件各自去掉） | 存活：P3S1B-M1 |
| B | 探针 h1–h5（无条件的 `nodes_pkey` 一次、逐行、合并；内侧逐行 `id = ANY`） | 全部放过：P3S1B-M1 |
| B | 改法试跑（`nodes_pkey` 要求条件里有 `(id = `） | 短测试九格过；h1–h4 被拒、h5 仍放过；整个守卫绿（中位数 1.07–1.18 s） |
| B | 探针 296（每条被判的语句第一次真跑之前在同一连接上 `EXPLAIN`，与守卫判的比） | 84 条里 82 条相同，2 条不同、都合规则：P3S1B-M2 |
| C | c1（退回的处理函数调批准的用例） | 抓到（`TestTheUpgradeIsAnswered`） |
| C | c2（驳回把结局记成 `abandoned`）、c3（批准对 403 不理会、照常往下） | 抓到（矩阵：`rejectUpgrade, reviewing/notebook_admin`；非管理员各格答 409 而不是 403） |

**修复一轮的**（`m11/p3s1/fix1/`：`gen.py` 生成，`run.sh` 先在树上把每条不同的命令跑一次、baseline 都 rc=0，再跑 `mut.py`，整轮经 `heavy.sh`；r1–r6 用 B 的 `b/mut/spec-rules.json`（r1 的 `old` 照新的一行改写），a1、a2 用 A 的 `a/spec1.json`；之后 `git status` 干净、没有 `*.mutbak`；组合根的两条不带 `-race` 跑，竞态的第八份里同样跑）：

| # | 改法 | 跑的 | 改前（`d7b85bae`） | 改后（`a44602b0`） |
|---|---|---|---|---|
| r1 | `nodes_notebook_id_*` 不要求 `notebook_id = ` | `TestTheApprovalGuardJudgesItsReads` | 活 | 死（"nodes by the notebook's index with the id alone"） |
| r2 | `page_contents_pkey` 不要求 `node_id = ` | 同上 | 活 | 死（"the whole of page_contents by its key for each node"） |
| r3 | `asset_blobs_node_id_key` 不要求 `node_id = ` | 同上 | 活 | 死（"asset_blobs by its node's key without a node"） |
| r4 | `upgrade_nodes` 怎么读都 `ok` | 同上 | 活 | 死（"the baseline by the notebook's index"） |
| r5 | `upgrade_nodes` 内侧不要求 `node_id = ` | 同上 | 活 | 死（"the baseline by its key's prefix again for each node"） |
| r6 | 去掉通用的"内侧逐行要按键" | 同上 | 活 | 死（"the notebook's nodes again for each row of another statement"等） |
| r7 | `nodes_pkey` 不看条件 | 同上 | —（就是 `d7b85bae` 的代码） | 死（"the whole of nodes by their key once"与"for each id"） |
| r8 | 去掉"内侧逐行不许 `= ANY`" | 同上 | —（改前没有这一条） | 死（"a set of nodes by their key again for each id"） |
| a1 | 批准交给 `JudgeRoots` 的来源根换成零 | `TestApproveUpgrade` | 活 | 死 |
| a2 | 退回的"行不是这次升级的"只剩 `!ok` | `TestSendBackUpgrade` | 活 | 死 |
| n4 | 回滚不写回正文（`RollbackState` 不出版本：`if true \|\| !ok \|\| …`） | `TestUpgradeReview/rejected` | 活 | 死 |
| o6 | 组合根的 `ApproveTimeout` 取整个请求 | `TestUpgradeApproval/its own deadline`（第八份的那一格） | 活 | 死 |

**修复核对一轮的**（规格与结果在 `p3s1/review/a/check1/`、`b/check1/`、`c/check1/`）：A 在头上重跑 a1、a2、n4、o6，都死（n4 下失败的是 `upgrade_review_test.go:226`、`:232`、`:238` 三行，o6 下是 `upgrade_approve_test.go:300`）。B 在头上重跑 r1–r8，各有一格失败；另加 r9（`= ANY` 连只读一次的也拒），被正例"a set of nodes by their key once"抓到；第一轮的探针 h1–h5 都被拒。C 加一条：上传时的 SHA-256 每块前多喂一个字节（`asset/app/blobs.go:65`），库里的 `asset_blobs.sha256` 与 `source_hash` 一起错，`d7b85bae` 的 UP7 照过、`a44602b0` 的失败。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `a44602b0`（修复一轮 4 个提交），三位各在自己的工作树里；C 另核文档稿（`try.diff`） | A：P3S1A-N1–N6 关；a1、a2、n4、o6 在 `d7b85bae` 上活、在头上死；经 `heavy.sh` 竞态的 `TestUpgradeApproval`、`TestUpgradeReview` 包 14.5 秒；没有新发现。B：P3S1B-M1、M2、N1、Q1 关；短测试 19 格，r1–r8 各有一格失败，r9 被正例抓到，第一轮的 5 个探针都被拒；守卫 24 次判计划，"内侧不许 `= ANY`"没拒掉真的计划；新的 P3S1B-N2（Nit）。C：P3S1C-N3–N5 关（N4 以上传哈希的变异核过，N5 重跑 `-list` 核过）；文档稿里 N1、N2、N6 与 A-N7 落对，3.23 的取舍与 C 的判断一致，10.1 与 00 号文档第 13 节的 CI 数经 `gh api` 逐项核过；新的 P3S1C-N7（Nit，文档稿） | P3S1B-N2 不改（只影响守卫失败时的诊断，不影响判定）；P3S1C-N7 在本提交；没有 Important、Minor 留下，核对到此为止 |

## 实测

- **`go test -race -list`**（`m11/tools/shards8-p3.py`，在 P2 的 `shards8-p2.py` 上改：新模式与 `--old` 的旧模式，各份的命令照 `Makefile` 的 `-run` 减 `-skip`）：基点 `75f5d077` 547 个顶层测试，新模式 145、50、120、120、61、15、27、9（设计写的数），旧模式 145、50、120、124、61、11、27、9，两种模式之间换份的恰是第 278 条的四个；头 `d7b85bae` 551 个，145、50、120、120、61、15、28、12，各在恰好一份，多的四个是 `TestUpgradeApproval`、`TestUpgradeReview`、`TestUpgradeReviewInterleavings`（第八份）与 `TestUpgradeTrashReservedTitles`（第七份）；守卫与 `TestTheApprovalGuardJudgesItsReads` 带 `//go:build !race`，不在其中。审查 B 以自己的 `classify.py` 复核，相同。
- **第四、六份挪前挪后**（`measure.sh`，`make -j1 test-go-bootstrap-<n> GO_TEST_RACE='go test -race -count=1 -timeout 15m -v'`，两份交替各两轮，包的时间；挪前在 `75f5d077`，挪后在它加 1 号的 `Makefile`）：

  | 份 | 挪前（秒） | 挪后（秒） | 7763 上的包 / 本机挪前 | 折到 7763 的 Test 一步 |
  |---|---|---|---|---|
  | 第四份 | 215.4、221.4（均 218.4） | 193.9、191.8（均 192.9） | 410.1 / 218.4 = 1.878 | 192.9 × 1.878 + 32 ≈ 394 |
  | 第六份 | 211.5、210.8（均 211.1） | 242.2、242.2（均 242.2） | 294.8 / 211.1 = 1.396 | 242.2 × 1.396 + 32 ≈ 370 |

  四个在第四份里合计 29.47、29.67 秒，在第六份里 29.33、29.11 秒（设计 7.5：29.70）；第六份第一个跑的仍是 `TestIngestCleanup`。比用的是本机挪前的两轮（设计用设计时的 224.4、213.2，比 1.828、1.383；本机这次稍快，比就稍大），折出的 394、370 落在设计预计的 394–407、374–387 之内，与设计审查 B 的 384、364 相近。第四、六份在头上没有再量（2–7 号给它们加的只有契约里的三个操作，设计估约 1.5%）。
- **第七、八份在头上**（临时工作树检出 `4b852247`，与 `d7b85bae` 的竞态测试二进制相同；两份交替各两轮）：第七份 182.8、185.9 秒（设计时 181.9、182.1），折到 7763 约 321 秒（设计预计约 345）；第八份 187.2、185.8 秒（设计时 167.3、166.2），包对本机之比 1.533，约 318 秒（设计预计 350–375）。第八份的新测试 `TestUpgradeApproval` 10.0 秒（两本 `tm.reviewed`、期限三格共用一个 2 s 的应用）、`TestUpgradeReviewInterleavings` 2.7 秒、`TestUpgradeReview` 1.9 秒；第七份 `TestUpgradeTrashReservedTitles` 1.1 秒，`TestUpgradeStatesReachTheStreams` 多九步之后 5.0 秒。审查 A 经 `heavy.sh` 跑四个新测试一轮：包 19.0 秒（11.7、2.1、2.9、1.1 秒）。
- **批准的成本守卫**（四轮，`logs/guard-r1.txt`–`r4.txt`：r1、r2 定界，r3 在 `4b852247`，r4 在 `d7b85bae`，加了"读头只经键"）。四个模板：计时的两个（旁边九本，这一本约占 `nodes` 的 11%：60,003 / 528,003），只核对计划的两个（旁边四本，约 22%：60,003 / 268,003）。每格三次批准的服务端 `elapsed_ms`（减去 `EXPLAIN` 的用时）的中位数（ms）：

  | 计划 × 统计 | r1 | r2 | r3 | r4 |
  |---|---|---|---|---|
  | 自定义，从未分析 | 1,148 | 1,143 | 1,125 | 1,137 |
  | 自定义，发起留下的 | 1,167 | 1,149 | 1,120 | 1,135 |
  | 自定义，这一本写在统计之后 | 1,238 | 1,236 | 1,218 | 1,192 |
  | 通用，从未分析 | 1,285 | 1,143 | 1,138 | 1,141 |
  | 通用，发起留下的 | 1,113 | 1,118 | 1,077 | 1,089 |
  | 通用，这一本写在统计之后 | 1,105 | 1,115 | 1,114 | 1,080 |

  界 3.9 s（r1、r2 各格中位数里最大的 1.285 s × 3 = 3.86 s，向上取整到 100 ms），远在 `ApproveTimeout` 11.25 s 之内。72 次计时的批准里：`SetSourceHashes`（五块合计）0.74–0.94 s，`SourceRebuildHeads` 0.14–0.18 s，`LintNodeMetadata` 0.11–0.15 s，`DeleteUpgradeNodes` 16–32 ms，`ForgetSources` 5–16 ms，`AttachmentInfos` 2–4 ms。种子：旁边九本 19.1–19.4 秒、写在统计之后 19.1–27.6 秒，旁边四本 9.1–9.3 秒、17.3–17.7 秒；整个测试 121.4、120.6、119.7、111.4 秒（设计估 4–5 分钟）。22% 的种子的计划（六格 × 两种，四轮完全相同）：`SourceRebuildHeads` 十二个都是 `Function Scan` → 逐行 `nodes_pkey` → `Memoize` → 逐行 `page_contents_pkey`；`SetSourceHashes` 每块至多读这本笔记本一遍（自定义计划与"从未分析"的通用计划哈希连接，另两种通用计划逐行经 `nodes_notebook_id_id_key`，决定 297）；`LintNodeMetadata` 读这本笔记本一遍、`page_contents` 逐行经主键；`ForgetSources` 按这本笔记本的索引；`AttachmentInfos` 经 `asset_blobs_node_id_key`。计时的两个种子的计划在 r1–r3 相同；r4 里"写在统计之后"的两格 `FindNodeIn` 的自定义计划换了一个按笔记本的索引，都合规。审查 B 四次整跑守卫，六格中位数 1.22–1.36、1.24–1.41（探针）、1.24–1.41、1.07–1.18 s，界在本机的余量 2.8–3.6 倍。
- **e2e**：`make e2e` 528 个（UP6–UP8 五个：UP6 API 341 ms、MCP 355 ms，UP7 API 134 ms、MCP 108 ms，UP8 51 ms；Playwright 2.3 分钟）。审查 C 在 Chromium 上以 `--repeat-each=4` 跑 UP6、UP7、UP8，20 个都过、共 6.1 秒，UP6 的两个版本各 4 次都确定地看到回滚在排队。
- **vitest**：193 个文件、4,447 个测试（4 号提交加了 `schema.gen.ts` 的三个操作；C 在头上复跑相同）。
- **修复一轮**（头 `a44602b0`）：守卫一轮 117.8 秒，六格中位数 1,130、1,108、1,174、1,119、1,086、1,065 ms（界 3.9 s），每格第一次批准的 `EXPLAIN` 约 12–13 ms，六个计时格（各判九条）与六个只核对计划的格（各五条）每条的自定义与通用计划都合规；短测试 19 格；竞态的 `TestUpgradeApproval` 11.2 秒、`TestUpgradeReview` 2.0 秒（驳回一格多一次退回与一次提交）；`make e2e` 528 个、133 秒。核对时 B 的守卫一轮六格中位数 1.09–1.19 s，`EXPLAIN` 11.7–14.3 ms；A 的两个组合根测试包 14.5 秒。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**（实施者拆提交时在工作树里逐个核对，每个提交的树就是当时的工作树，提交之后 `git status` 干净；表里是改写之前的 2–7 号 `260356f2`…`4b852247`）：

  | # | build、vet、`lint-go` | `gen-check` | `test-go-short`（heavy） | 它自己的（heavy） |
  |---|---|---|---|---|
  | 1 | 检出 1 号的临时工作树里 `make lint-go` rc=0；`make -n test-go` rc=0；`oxfmt --check ci.yml` | — | — | `-list` 归类；第四、六份挪后两轮 |
  | 2 | rc=0 | rc=0 | rc=0 | `notebook/...`、`wiki/...` 全部包（真库）；组合根 `TestUpgradeSubmit`、`…Interleavings`、`TestUpgradeCheck`、`TestUpgradeGates`、`TestUpgradeStatesReachTheStreams`、`TestThePermissionMatrixCoversEveryOperation` |
  | 3 | rc=0 | rc=0 | rc=0 | `page`、`page/app/...`、`page/adapter/postgres/...`（真库，后者 317 秒） |
  | 4 | rc=0，`lint-web`、`knip` rc=0 | rc=0 | rc=0 | `make test-web`（193 个文件、4,447 个测试）；`wiki/...`、`notebook/...`（真库）；组合根新加的四个与改的四个、契约的八个 |
  | 5 | rc=0，`lint-web`、`knip` rc=0 | rc=0 | rc=0 | `make test-web`；组合根 `TestUpgradeTrashReservedTitles` 与碰保留标题、模式守卫、来源恢复的十一个、契约两个 |
  | 6 | rc=0（一处 `errorlint` 改了再提交） | — | rc=0（守卫在 `-short` 下跳过，判计划的短测试在） | 守卫两轮；`make -n test-go-costs-1` |
  | 7 | `lint-web`（含 e2e 的类型检查）、`knip` rc=0 | — | — | `make build` 之后 UP6、UP7、UP8 五个 |

  改写之后（`verify-rw.sh`，整轮经 `heavy.sh`，日志 `logs/rw/`）：检出 2 号 `9432d60a`，`make lint-go`、`make gen-check` rc=0，`notebook/...`（真库）rc=0；检出 6 号 `bba17208`，`make lint-go`、`make gen-check`、`make test-go-short`（42 秒）rc=0，短测试九格都过；头 `d7b85bae` 上 `make lint-go` rc=0、守卫一轮 rc=0（`guard-r4`）。3–5 号与 7 号原样重放，实施者没有另跑；审查 C 以 `percommit.sh` 在改写之后的 2–6 号上逐个检出、跑 `make lint-go`、`make gen-check`、`git status`、经 `heavy.sh` 的 `make test-go-short`，4、5 号另跑 `lint-web`、`knip`、`test-web`，每一项都是 rc=0、每个提交的 gen-check 之后 `status` 都是 0 行，最后回到头（`review/c/percommit/summary.txt`）。拆之前在完整的树上已跑过全部新测试，那时发现并改了三处：退回之后 agent 写了正文（改成只写 frontmatter，照设计）、`TestAnUpgradeIsApproved` 的整行串里 `concat_ws` 把布尔写成 `t`、契约的三个路径没进 `api/openapi.yaml`。
- **建成时的头**（`d7b85bae`，改写之后；`gates.sh`：每一项各经 `heavy.sh`、一次一个，每一项之前判 `df`，整轮 52G 可用；日志 `logs/gates/`，`make test` 的日志 233 MB 压成 `test.log.gz`；开始与结束时工作树干净），一轮全绿：

  | 门禁 | 结果 | 用时 |
  |---|---|---|
  | `make lint gen-check knip build` | rc=0 | 8 秒 |
  | `heavy.sh make test` | rc=0（竞态、组合根八份、成本各行与守卫、vitest 193 个文件 4,447 个测试；批准的守卫 112.6 秒，六格中位数 1,137、1,114、1,219、1,202、1,117、1,086 ms，计划都合规） | 3,549 秒 |
  | `heavy.sh make test-go-short` | rc=0（含判计划的短测试九格） | 41 秒 |
  | `heavy.sh make e2e` | rc=0，528 个 | 138 秒 |

  审查 C 在头上另跑：`make gen-check`、`lint-web`、`knip`、`test-web` 都 rc=0；经 `heavy.sh` 的组合根契约测试八个与矩阵的三行（3 × 13 格）都过（第一次没带 `prepare`，测试照设计报"prepare did not run"，补上之后重跑）。
- **修复一轮**（`fix1/verify.sh`，各检出、各经 `heavy.sh`，日志 `fix1/logs/summary.txt`）：四个提交各自 `make lint-go`、`make gen-check`、`make test-go-short`（40–41 秒）都 rc=0；1 号另跑守卫一轮（rc=0，117.8 秒，短测试 19 格过）；2 号另跑 `go test ./internal/modules/wiki/...` 与竞态的 `TestUpgradeApproval`、`TestUpgradeReview`（rc=0）；3 号另跑 `make lint-web`、`make knip` 与 `make build` 之后 UP7 的两个（chromium）；4 号只改注释，`make -n test-go` rc=0。头 `a44602b0` 的门禁：`make lint gen-check knip build` rc=0（9 秒）；经 `heavy.sh`：`make test-go-short` rc=0（40 秒），`make e2e` rc=0、528 个（133 秒）；照修复说明不跑整轮 `make test`（只改了测试与注释）。整轮之前之后 `git status` 干净。

CI（作业，括号里是 Test 一步；e2e 是 E2E 一步）：

| 作业 | 43088294571（建成，`d7b85bae`） | 43099927925（修复一轮，`a44602b0`） |
|---|---|---|
| server (race) | 14:58（14:38） | 14:04 |
| server (bootstrap-1) | 6:36（6:17），Xeon 8573C | 6:55（6:33） |
| server (bootstrap-2) | 4:17（3:58），EPYC 9V45 | 5:56（5:37） |
| server (bootstrap-3) | 7:16（6:51），EPYC 7763 | 7:01（6:41） |
| server (bootstrap-4) | 6:09（5:45），EPYC 9V74 | 5:31（5:08），Xeon 6973P-C |
| server (bootstrap-5) | 5:28（5:08），Xeon 8573C | 7:16（6:55） |
| server (bootstrap-6) | 5:56（5:33），EPYC 9V74 | 6:37（6:10），EPYC 7763 |
| server (bootstrap-7) | 5:47（5:22），EPYC 7763 | 4:49（4:28），EPYC 9V74 |
| server (bootstrap-8) | 5:26（5:07），EPYC 9V74 | 3:55（3:33），EPYC 9V45 |
| server (costs-1) | 20:05（16:39），Xeon 8370C | 18:46（15:36），Xeon 8573C |
| server (costs-2) | 24:33（24:12） | 25:00 |
| server (costs-3) | 16:21（15:58） | 17:29 |
| server (costs-lint) | 8:22（8:03） | 11:47 |
| server (guards-core) | 13:39（13:17） | 19:45 |
| server (guards-sources) | 17:04（16:46） | 16:56 |
| server (short) | 3:43（3:20） | 3:08 |
| web | 4:08（3:35） | 4:53 |
| e2e | 10:43（7:43） | 9:06（7:20） |
| image | 1:23 | 1:17 |

- 建成时一次全绿，19 项，普通推送（新分支，`d7b85baef5e311cb92579f623e1352d91de49049`，rc=0），`docs/v0.1/claude_to_codex/tools/ci-wait.sh` 等到第 1 次 attempt（中途一次 GitHub 的 API 答空、脚本打了一段 traceback，下一轮照常，结果不受影响）。八份的 Test 一步 377、238、411、345、308、333、322、307 秒，包 345.8、216.3、377.3、318.4、282.5、306.0、288.7、275.2 秒，都在 450 秒之下。第七份在 7763 上 322 秒，与本机折的 321 秒相合；第四、六、八份都在 9V74 上，照设计 7.5 用的第八份的 9V74 → 7763 之比 1.189 折约 411、396、359 秒，比本机折的高 10–40 秒（那个比只取自第八份一次），都在 450 之下，最贴近的是第四份，以合并之后两次 `main` 为准。costs-1 在 Xeon 8370C 上 20:05（设计估 23–26 分钟，时限 45 分钟）：批准的守卫那一行在作业的第 15.7 分钟开始，包 253.0 秒（种子 40.9、57.8、19.7、36.7 秒），六格中位数 2,346、2,349、2,508、2,364、2,269、2,249 ms，界 3.9 s，余量约 1.55 倍（P3S1B-Q1、第 298 条），`SetSourceHashes` 1.54–1.71 s，计划都合规；15 分钟的 Go 超时到第 30.7 分钟，早于 45 分钟（审查 B 核过）。e2e 一次过，"Install the Playwright browsers" 2:16 装好，528 个。
- 修复一轮：普通推送（`d7b85bae..a44602b0`，rc=0；`git ls-remote` 是 `a44602b0fe6d0954a26e70611b5f5d47f07e3877`），`ci-wait.sh a44602b0fe6d0954a26e70611b5f5d47f07e3877`：run 43099927925 的第 1 次 attempt 19 项一次全绿（修复一轮的一栏只记下了组合根、costs-1 与 e2e 的 Test 一步与 runner）。八份的 Test 一步 393、337、401、308、415、370、268、213 秒，都在 450 秒之下；第六份在 EPYC 7763 上包 337.4 秒、Test 一步 370 秒，与本机折的约 370 秒相合；第四份在 Xeon 6973P-C 上包 285.4 秒，第七份在 9V74 上 241.4 秒，第八份在 9V45 上 192.3 秒。costs-1 在 Xeon 8573C 上 18:46：批准的守卫 222.6 秒，六格中位数 2,075、2,081、2,227、2,096、2,007、2,092 ms（界 3.9 s，余量约 1.75 倍），计划在每条语句真跑之前判、都合规；短测试 19 格在 short 一行过。e2e 一次过，"Install the Playwright browsers" 1:04 装好，528 个，L1 没有出。

S1 开工之前 `main` 的几次 CI（详见 [P3 设计](../03-P3-review-approve-playbook.md) 10.1 的"P2 完成之后的 `main`"）：`docs: M11/P2 done` 的 `955fa579`（run 43033276772）e2e 四次（attempt 1–4）都卡在"Install the Playwright browsers"（apt 镜像 azure.archive.ubuntu.com 每个包都超时），到 20 分钟被取消，其余 18 项第一次就绿；同代码、只多 P3 设计文档的 `75f5d077`（run 43060437171）19 项一次全绿，e2e 8:41，作为这份代码的 e2e 证据。S3 之后两次 `main` 的八份平均都在 450 秒之下，第四份 377.5 秒（445、310）。S1 从 `75f5d077` 开出。

## 环境

实施者开工时 `df` 28G，照 06-environment 经 `heavy.sh` 删了 go-build 缓存里 12 小时没用过的条目（可用 28G → 56G，缓存 32G → 4.6G）；之后整轮在 51–56G 之间，门禁每一项之前判 `df`。量用的临时工作树已 `git worktree remove`，`GOTMPDIR` 的目录（`~/.cache/nw-gotmp-p3s1`）已删；这一 Step 没有手动起容器（库都由测试经 testcontainers 起、收），机器上 3 天前留下的已退出的 `postgres` 容器不是这一轮的，没有动。三位审查者的重的运行都经 `heavy.sh`，每次之前看 `df`（50–51G 可用）；A 的 `GOTMPDIR` 设在磁盘上、用完删了；B 的临时探针文件（`b/zz_reviewb_test.go.txt`）已从工作树删掉，守卫文件从备份复原（sha256 核对）；C 的逐个提交最后回到头。三个工作树最后都回到 `d7b85bae`、`git status` 干净、没有 `*.mutbak`。修复一轮：每项之前判 `df`（48–49G），`GOTMPDIR`（`~/.cache/nw-gotmp-p3s1-fix1`）已删，没有手动起容器，工作树干净、没有 `*.mutbak`，分支头与远端相同、`main` 没有动。修复核对一轮：三个工作树最后都在 `a44602b0`、`git status` 干净、没有 `*.mutbak`；A 的 `GOTMPDIR` 用完删了，B 的临时探针文件跑完删掉，C 的临时旧 spec 已删、`bin/` 由干净的树重建。

合并之后 `main` 的第一次 CI：run 43112344486（`daf14dd1`）19 项第一次 attempt 全绿；八份的 Test 一步 392（EPYC 7763）、367（7763）、393（7763）、284（EPYC 9V45）、279（Xeon 6973P-C）、373（7763）、317（7763）、263（Xeon 8573C）秒，都在 450 之下，第四份挪走四个之后不再贴线（第 278 条）；costs-1 在 EPYC 9V74 上 16:16（Test 一步 13:19），批准的守卫六格中位数 1.79–1.91 s（界 3.9 s；第 298 条的条件看 7763 上的数，这一次不在 7763 上）；e2e 7:55（E2E 6:14）。
