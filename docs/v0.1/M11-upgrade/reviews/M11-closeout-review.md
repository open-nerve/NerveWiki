# M11 升级流程：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `9e787bab`（`docs: M11/P3 done`）：M11 的三个 Phase 全部合并之后，P1 完成于 `f002672b`，P2 于 `955fa579`，P3 于 `9e787bab`。M11 的改动看 `d9169baa..9e787bab` |
| 方式 | 三位独立审查者并行，各在自己的 worktree 里（detached 在 `9e787bab`），重的都经 `heavy.sh`，审完 worktree 干净、头仍是 `9e787bab`。A 服务端：跨 Phase 的一致性与扩展点，契约、REST、MCP 与矩阵的码与码序，日志，交错；组合根的临时测试经 REST 与 MCP 把整个流程走一遍（`-race`，6.3 秒），三个缝的探针，扩展点的三个"换成空"的变异。B 网页与 e2e：本机 `stories/upgrade` 加 LW11 共 36 个，升级的五个 vitest 文件 222 个；Playwright 探针在三种浏览器上各走一遍、中文界面一遍、四处交错；假服务端对着真服务端。C 完成标准、移交与文档：第 3 节逐条找出守着它的测试与它在哪一份；四份移交逐项对着代码找落实；13.x 里写着 M11 的 80 句由一个只读的子 agent 逐句对着代码核对，C 复核它报出的每一条；README 与人工清单 K 对着契约、`tools.json`、`prompts.json` 与网页；M11 期间 `main` 的每次 CI |
| 日期 | 2026-11-13（三份审查在 2026-11-12） |
| 结论 | 没有行为上的 Important。A：Minor 1（回滚的变更集在一种常见的形状下撤销答冲突）、Nit 2，走一遍没有出错。B：Nit 5、Q 1，三种浏览器走通。C：Important 4，全在收尾手续与文档（四份移交仍是 `open`；"写给后面的 M"没写进 M12；状态停在进行中、没有收尾记录；13.1 第 2 条与代码、决定 177 相反），Minor 3、Nit 6、Q 1。主持者的决定 302、303。修复在分支 `m11-closeout` 上，两轮；原席核对见"核实修复" |

## 完成标准核对

证据是 C 在 `9e787bab` 上读代码与 CI 得的，行号都是 `9e787bab` 的；A、B 的走一遍另核了第 1–10 条。整个程序的测试在 `bootstrap/`，照 `Makefile` 的 `BOOTSTRAP_SEVENTH`、`BOOTSTRAP_EIGHTH` 分在第七、八份；成本守卫是 `!race`，各在 `test-go-costs-*` 的一行，不会被 `-short` 或分份漏掉。

| 第 3 节 | 结论 | 测试 |
|---|---|---|
| 1 状态机 | 满足 | `TestUpgradeStates`；`TestUpgradeStatesReachTheStreams`（`bootstrap/upgrade_start_test.go:389`，S3、P2、P3 共 20 多步，每步 `[n, n, 0]`，订阅者交空时失败）；`TestUpgradeReview`；`TestUpgradeSubmit`、`TestUpgradeApproval` 的码序；wiki/app 的 `TestAbandonUpgrade`、`TestEndUpgradeRollback`；`TestPageViolationsCatchEachUpgradeGap` |
| 2 发起 | 满足 | `TestUpgradeStart`；`TestUpgradeInterleavings` 的 1–4、14、17；`TestUpgradeStartTakesItsTimeAtAnyStatistics`（5 万页，costs-2）。10 万页在 M12（[性能移交](../../M12-release/handoffs/M4-performance.md)第 21 条） |
| 3 冻结到每条路径 | 满足 | `TestUpgradeFreezeReachesEveryPath`（三种状态 × 14 个 REST 操作与 5 个工具、开启、任务的批）、`TestUpgradeFreezeLeavesTheRest`、`TestUpgradeFreezeAnswersFirst`、`TestUpgradeFreezeFollowsTheInitiatorsRole`、`TestUpgradeFreezeLetsOnlyItsOwnWrites`、`TestFreezeAdmits`。13.1 第 8 条的"14 个"今天是 15 个声明（`startUpgrade` 也声明），下一句写明了，不改 |
| 4 护栏 | 满足 | `TestUpgradeGuardrails`（g1–g27）、`TestTheUpgradeBodyShapes`、`TestTheFreezeGuard`、`TestUpgradeToolsErrorsCarryTheirCodes`，UP4 的三个 MCP 版本 |
| 5 闸门 | 满足 | `TestJudgeUpgrade`、`TestJudgeUpgradesCounts`、`TestUpgradeGates`、`TestUpgradeCheck`（含交错 8）、`TestUpgradeSubmitInterleavings`、`TestUpgradeCheckTakesItsTimeAtAnyStatistics`（5 万页）。10 万页在 M12（同上） |
| 6 批准 | 修复之后满足（决定 303） | `TestUpgradeApproval`（哈希等于从头算、`checkPages`、锁下再核闸门、自己的期限三格什么也不写）、`TestUpgradeReviewInterleavings`、`TestAnUpgradeIsApproved`、`TestTheSourceRebuildIsTheInvariant`，UP7。审查时哈希的重建只在固定的形状上核对，随机步的 `TestTheSourceRebuildMatchesSuccessfulRandomSteps` 不走批准（M11CC-Q1）；收尾在 page 模块补了随机形状的性质测试 `TestTheSourceRebuildOnRandomTrees`（`page/source_rebuild_test.go`；第二轮补上 `raw` 下的旧哈希，核对 M11CA-M2） |
| 7 回滚 | 满足 | `TestUpgradeRollback` 的 11 个子测试、`TestRollbackPlansPutTheBaselineBack`（性质）、`TestARollbacksPlanOfAHundredThousandItemsTakesItsTime`、`TestARollbackDeletesAHundredThousandScatteredPagesFirst`、`TestARollbacksReadsTakeTheirTimeAtAnyStatistics`，UP3。回滚的变更集撤销起来有一处已知的冲突（决定 302），由 `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages`（`bootstrap/upgrade_rollback_revert_test.go`）钉住 |
| 8 回收站里的保留标题 | 满足 | `TestUpgradeTrashReservedTitles`；契约里 `revertChangeset`、`startRevert` 声明 `page.virtual_file`，`restoreNode`、`startTrashRestore` 不声明；`page/domain/contract.go:141`；UP7 |
| 9 MCP | 满足 | `tools.json` 两个入口各两个工具；`TestTheToolListsAreTheContract`、`TestUpgradeToolsMatrix`、`TestThePermissionMatrixCoversEveryTool`、`TestUpgradeToolsErrorsCarryTheirCodes`、`TestTheBuiltInInstructionsAreShort`（剩不足 300 字节就失败）、`TestThePromptListsAreTheContract`、`TestUpgradePromptMatrix`、`TestUpgradePrompt`；UP1、UP4–UP7、UP9 的 MCP 版本 |
| 10 网页 | 满足 | UP1–UP3、UP5–UP8 的页面版本（UP1、UP7 在三种浏览器）；`upgrade-read-only.test.tsx`（控件表 c1–c19）、`upgrade-page.test.tsx`、`upgrade-check.test.tsx`、`upgrade-banner.test.tsx`；`problem-messages.test.ts`（两种语言） |
| 11 人工 | 清单在，执行留到 v0.1 的最后 | 人工清单 K1–K5（[P5-mcp-clients-checklist.md](../../M9-mcp/manual/P5-mcp-clients-checklist.md)"K：升级（M11）"）；输入法清单不加步（00 号文档 4.15）。结果写进本记录"人工验收" |
| 12.5 没有 `open` 的 handoff、`main` 绿 | 本提交之后满足 | 四份移交这次改为 `done`（见"handoff"）；`9e787bab` 的 run 43205028299 第一次 attempt 19 项全绿 |

## 发现与处置

### A（服务端）

走一遍覆盖第 3 节第 1–9 条的服务端部分：每一步核对答复、库里的行、另一条流上 `mode` 的帧数、恰好一行 info 日志与它的键；在没有升级、升级中、审阅中、已回滚、LLM Wiki、回滚中六个状态 × 三种身份里，REST 与 MCP 的码逐个比较，没有一处不同；结尾 `checkPages`、`checkBackground` 通过，全部日志行里没有标题与路径。契约十个操作的 `x-problem-codes` 与 00 号文档第 5 节逐个相同，次序也相同；4.18 的日志一行不差；4.16 的十七个交错都有确定的测试；第 8 节的扩展点都登记了，三个"换成空"的变异都被抓到。

| 发现 | 处置 |
|---|---|
| M1 回滚的变更集在常见的形状下撤销不了：新建的文件夹既收着移进去的原有页、又收着新建页时，同步与后台的撤销都答 409 `page.revert_conflict`。先删的一段在自己的单元、自己的时刻把新页删成回收站根，下一遍才删那个文件夹；M8 的撤销计划在结构上把新页的顶定为文件夹，回收站里文件夹的组却不含它（`page/domain/revert.go:502` 的 `leftDeleted`）。00 号文档 4.10 写"之后可以在网页上撤销"，没有测试撤销过回滚的变更集 | 决定 302，v0.1 不修。00 号文档 4.10、4.15 改成实情（本提交）。整个程序的测试 `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages` 钉住两种形状（`e96c7bc0`）：新建的文件夹下只有移进去的基线页时，撤销答 200、笔记本回到 agent 整理之后的样子；又有新建页时，两种撤销都答 409、`conflicts` 恰是那两个节点、笔记本不变。真正修好写进 [M12 的历史移交](../../M12-release/handoffs/M8-history.md)第 6 条 |
| N1 第 8 节写 `SetNotebookMode` 同时清升级的列，与第 281 条和代码相反 | 改为"批准是 notebook 自己的 `ApproveUpgrade`，`SetNotebookMode` 不动"（本提交） |
| N2 `bootstrap/wire.go:213` 另调一次 `notebookRegistrants`，升级的订阅链不是交给降级的那一份 | `wireApp` 取一次 `notebookRegistrants(pool)`，交给 `notebookDeps` 与 `NewUpgradeStates` 共用，`notebook/upgrade_states.go:27-28` 的注释成真（`7c422c0a`）；`TestUpgradeStatesReachTheStreams` 照过，变异 `upgrade-states-no-subscribers` 照死 |
| 存疑：M8 的后台恢复、后台撤销同样一个任务一个变更集、分批删，形状对上时可能有同样的冲突；结束回滚之后从新到旧逐个撤销 agent 的变更集，前四个答 409、接着三个 200、最后两个 409，没有 500 | 前者没有造，写进 M12 历史移交第 6 条一并看。后者的冲突来自回滚已做的部分，本身合理，记在"没能验证的风险" |

### B（网页与 e2e）

Playwright 探针走发起、冻结、agent 整理、检查与提交、退回再交、驳回、再发起与批准、批准之后、另一本的放弃停下继续结束；walk A、B 在 Chromium、Firefox、WebKit 各一遍，walk C 是中文界面，walk D 是四处交错（批准的答复挡 1.5 秒、发起人在升级页上被降级、开着批准对话框时被驳回、正在编辑时发起）。只读收成一处、横幅、焦点、两种语言、假服务端的码与码序都核对过。

| 发现 | 处置 |
|---|---|
| N1 批准时 `mode` 先于答复到达（服务端在提交之后发事件，答复在记日志之后才写），重读升级答 404，"正在批准…"的对话框后面写"没有进行中的升级"；三种浏览器每次都是事件先到 | 批准在飞时记一个 ref，`wiki.upgrade_not_found` 照旧显示 SWR 留着的上一份，答复到了照旧跳转，被拒时清掉、照旧重读；vitest 两格（批准、被拒），去掉 ref 就红；成功之后 ref 留到页面卸下，不在答复时清（偏离 1）（`62dba5e4`）。[P3 设计](../03-P3-review-approve-playbook.md)第 5 节与 4.3 照改（本提交） |
| N2 英文的单复数："1 pages lead to …"（`upgrade.gateLinksWorse`）与"(1 nodes under it now)"（`upgrade.startRaw`） | 并进 [M12 的打磨移交](../../M12-release/handoffs/M5-polish.md)第 41 条（本提交） |
| N3 同一页上来源根有两个名字：状态一节写"raw"，批准对话框写"raw.md"（MCP 的路径写法） | 对话框照状态一节用 `distinctName`，取不到时照旧写"提议的来源根"；vitest 与 UP7 的断言随之改，另加同名的页与中文一格（`f3b65b87`）。P3 设计 4.3 照改（本提交） |
| N4 升级的八个对话框被拒之后焦点落在 `<body>`，根在 `ConfirmDialog` | 写进打磨移交第 38 条（本提交） |
| N5 假服务端的发起、放弃（继续）、结束回滚不发 `mode`，真服务端每个转移都发 | 三条路由加 `sendMode()`，文件头写明每个转移都发（`fae85831`）；没有格依赖发或不发，不设变异 |
| Q1 00 号文档第 7 节写给后面的网页几项还不在 M12 的移交里 | 由本提交写进（C-I2） |

### C（完成标准与文档）

| 发现 | 处置 |
|---|---|
| I1 四份移交仍是 `open` | 改为 `done`，证据见"handoff"（本提交） |
| I2 "写给后面的 M"没写进 M12：性能移交没有 M11 的第 21 条，第 18 条后面没有第 271 条的数；打磨移交没有第 7 节的十项；P2、P3 第 9 节交给 M12 的几项连第 7 节都没列全；第 7 节的"第 40 条起"已过时 | [性能移交](../../M12-release/handoffs/M4-performance.md)第 18 条加第 270、271 条的代价，加第 21 条（升级）、第 22 条（组合根各份的余量与测试设施）；[打磨移交](../../M12-release/handoffs/M5-polish.md)第 42–50 条，"1 nodes"并进第 41 条；00 号文档第 7 节照改编号、补 P2、P3 第 9 节的几项（本提交） |
| I3 状态停在进行中：总体设计 12.6、00 号文档的头与第 12 节；第 13、15 节没有收尾一行；收尾记录没有 | 改为"实现完成，待人工验收"，加收尾的行，写本记录（含 K 的表）；人工清单的头指向本记录（本提交） |
| I4 13.1 第 2 条：变更集的类型少 `rollback`；"M11 的冻结与护栏据此认出还原、恢复与撤销"与代码、决定 177 相反（`page/domain/freeze.go:20-26` 的 `FreezeAdmits` 不看 `Kind`） | 照改，另写回滚的变更集撤销起来的已知冲突（本提交） |
| M1 README 的四处：日志的类型、"任务"一段（同时一个写入任务、谁能取消回滚）、事件 `mode`、笔记本的答复的 `upgrade` | 四处各补半句（本提交） |
| M2 13.1 第 8 条与两处测试注释还说那两个测试在第五、第七份，实际都在第八份 | 文档（本提交）；`bootstrap/upgrade_tool_codes_test.go`、`bootstrap/mcp_reads_test.go` 的注释（`93af4308`） |
| M3 第 11 节"需要负责人确认"只写了第 212 条 | 加第 291、277、302 条各一句（本提交） |
| N1 第 13 节"P1 完成"一行写两条旁支，实际三条（漏了 `m11-p1-shards`，合并 `5795229c`） | 改（本提交） |
| N2 设计阶段两次 `main` 的第一次 attempt 红，文档里没记 | 记在"CI 的旧红" |
| N3 13.4 第 8 条两处说得比测试严 | 照测试改（本提交） |
| N4 13.1 第 10 条结束回滚的 info 行漏了 `complete` | 补上（本提交） |
| N5 README 发起一步没写会被拒的几种情形 | 补一句（本提交） |
| N6 打磨移交开头那句的条号只到 M9 | 补上 M10、M11 的（本提交） |
| Q1 第 3 节第 6 条的"性质测试"指什么 | 决定 303，见上表第 6 条（`9b9bd09f`；第二轮 `1bd91a41`） |

## 主持者的决定

写在 [00 号文档](../00-M11-design.md)第 11 节"收尾的取舍 302–303"，负责人可以改判。

- **302 回滚的变更集在一种常见的形状下撤销答冲突，v0.1 之内不修**（A-M1）：4.10、4.15 改成实情；一个整个程序的测试钉住两种形状；真正修好进 M12。理由：先删的一段是第 243 条有意分开的、有界的几步，回滚的大小靠它；修好要么拆成两个变更集（改"一个任务一个变更集"与任务答复的 `changeset_id`），要么改 M8 的撤销计划（要另做性质测试），都不是收尾的量。放弃照旧不要求输入名称：放弃丢掉的只是 agent 这一次的整理，发起之前的内容照原样回来。
- **303 第 3 节第 6 条的"性质测试"指随机形状、与代码无关地算出期望的测试**（C-Q1）：现有的只在固定的形状上核对，算一个缺口，收尾补上，不改第 3 节的说法。

## 负责人可以改判的决定

第 11 节的 172–303 都写着"负责人可以改判"。下面只列改了负责人原话的、留给负责人定的、收尾的两条、改了负责人认可过的总体设计说法的，以及负责人可能想自己定的。清单是 C 列的；212、291、277 C 对着代码核过（`wiki/app/end_upgrade_rollback.go` 与结束对话框；`upgrade-page.tsx:460-461` 只在 `reviewing` 给批准、退回、驳回，`abandonUpgrade` 的契约仍收 `reviewing`；`e2e/stories/links/l1-links.spec.ts:178` 没改）。

**改了负责人原话的**

| 决定 | 理由 | 出处 |
|---|---|---|
| **212** 回滚停下之后可以"结束回滚、保留现在的样子" | 放宽负责人第 1 条的"放弃时整体回滚"：默认仍整体回滚，停下之后多一个出口；结束之后原有页里被系统改写的链接可能指不到，结束对话框写剩下的三个数。M11 的汇报里第一条 | 00 号文档第 11 节第 212、217、230 条；4.10 |
| **291** 审阅中网页收起"放弃"，横幅写"可以驳回" | 碰到负责人第 1 条"任何笔记本管理员都能随时放弃"：审阅中网页只能经"驳回"，后果相同，库里 `ending` 记为 `rejected` 而不是 `abandoned`；API 照旧接受放弃 | 第 291 条；P3 设计 4.3、第 9 节 |
| **175** 冻结只管内容的写，发起人仍须是管理员 | 负责人的问题里说"整本笔记本只读，只有发起人的 agent 能写"：设置、成员、导出、删除不冻结；发起人被降级之后他的 agent 也不能写 | 第 175 条；4.5 |

**留给负责人定的**

| 决定 | 理由 | 出处 |
|---|---|---|
| **277** e2e 的 L1 照现状，CI 再红只重跑 e2e | 主持者明说"向 Playwright 上游报问题（对外）与改测法（削弱测试）"留给负责人；根在 Chromium，本机约九次一次。P3 之后的五次 `main` 第一次 attempt 都绿，没有再出 | 第 277 条；P2-S3 的审查记录"偶发" |

**收尾的**

| 决定 | 理由 | 出处 |
|---|---|---|
| **302** 回滚的变更集在一种常见的形状下撤销答冲突，v0.1 不修 | 见上节；负责人若要放弃也输入名称，在 M12 改 | 第 302 条；4.10、4.15；[M12 的历史移交](../../M12-release/handoffs/M8-history.md)第 6 条 |
| **303** 批准的哈希重建补一个随机形状的性质测试 | 见上节 | 第 303 条 |

**改了负责人认可过的总体设计说法的**

| 决定 | 理由 | 出处 |
|---|---|---|
| 172 已有的根下 `AGENTS` 正文不能改 | 5.8 第 3 步原写"定制 `AGENTS` 页面"，现在只能改名保留、另建一页 | 第 172 条，修订第 10 条 |
| 178（与 177、193） 回滚按基线整体复原 | 5.8 第 6 步原写"按倒序撤销本次升级的全部变更集" | 第 178 条，修订第 13 条 |
| 180、188 闸门 6 逐页、新建页不算 | 5.8 原写"解析不到的链接数不多于升级前"；逐页更严，新建页发出的不算更宽 | 第 180、188 条，修订第 15 条 |
| 197 报告不含类型分布与逐页的类型、摘要 | 5.8 的报告原列着它们，现在在目录视图里看 | 第 197 条，修订第 16 条 |
| 191 报告写"核对"不写"证明" | 5.8 原写"内容保全的证明" | 第 191 条 |
| 173、214 发起时补写 `AGENTS`、`raw`，根下已有的 `raw` 照用作来源根，不给选择器 | 根下放着普通笔记的 `raw` 会整个变成来源区（对话框写明节点数） | 第 173、214 条 |

**负责人可能想自己定的**

| 决定 | 理由 | 出处 |
|---|---|---|
| 213 新建页让原有页的短链接改指新页 | 字节不变，但原有页的链接意思变了，碰到 1.3 的"严格保全"的精神；只写进 playbook，报告不列 | 第 213 条 |
| 271（与 268、270） 升级中 `batch` 的计划约翻倍 | M9 已接受的最坏情形在慢机上约 18.6 秒，过 15 秒的期限；照负责人 11-06 的话不优化，过期限什么也不写 | 第 270、271 条 |
| 292（与 240、244） 为批准另写按键读头的语句，另有 `NOT EXISTS`、`LEFT JOIN LATERAL` 两处改写 | 主持者说"不是调优"，但它们是为了守卫的计划改产品的 SQL；负责人 11-06 说过"不要无限优化" | 第 240、244、292 条 |
| 232、262、280 发起、检查、批准的守卫从设计的 10 万降到 5 万，10 万进 M12 | 照负责人 11-06 的话；列出来让负责人知道这三处的规模 | 各条；总体设计第 14 节 |
| 249、267、269、275、294、298 调时限；186、245、261、272、276、278、290 分份 | 照负责人 11-06 的话调时限、不优化；CI 从 18 项到 19 项，组合根八份 | 各条 |
| 299 升级中 agent 握手读到"有三个 prompt：ingest、query、lint" | 中文按字面不实，留到 M12 的打磨第 40 条；负责人可能要发布前改 | 第 299 条 |
| 204 批准时不查来源区里的重复 | 批准之后的 LLM Wiki 里可能有逐字节相同的来源，与 M10 的 `page.source_duplicate` 不一致 | 第 204 条 |
| 183 审阅中对所有人冻结，退回不附说明 | 退回附说明进 M12 的打磨第 43 条 | 第 183 条 |

## handoff

四份都改为 `done`。C 逐项对着代码找到了落实的位置，或照 00 号文档第 7 节"移交的落实"写明不适用；"处理进展"里没有未做的项。

[`M5-events.md`](../handoffs/M5-events.md)：

| 项 | 证据 |
|---|---|
| 模式变化的事件类型 | 升级的每个状态变化经 `notebook.NewUpgradeStates` 的 `Publish` 发 `mode`（`notebook/upgrade_states.go:133`），`EventMode` 不加字段；最后一跳 `TestUpgradeStatesReachTheStreams`；网页一半由 UP1、UP7 的页面版本与 `handlers.test.ts` 核对 |

[`M8-P4-background.md`](../handoffs/M8-P4-background.md)：

| 项 | 证据 |
|---|---|
| 1、2 | 由按基线整体复原取代（第 178、193、212 条）：`rollback` 任务，先删新建的子树，计划由基线合成，继续就是重算；`TestUpgradeRollback`、`TestRollbackPlansPutTheBaselineBack`。第 2 条的"之后撤销 R"对回滚的变更集在一种形状下答冲突（第 302 条） |
| 3 | 不适用：回滚中冻结，回滚的任务是唯一的写者 |
| 4 | 由 `UpgradeOwn` 取代：`page/domain/freeze.go:20-35`、`TestUpgradeFreezeLetsOnlyItsOwnWrites`、`TestTheFreezeLetsItsOwnUnits` |
| 5 | 放弃、驳回经 REST，任务的客户端照 `shared.ClientOf` |

[`M9-mcp-upgrade.md`](../handoffs/M9-mcp-upgrade.md)：

| 项 | 证据 |
|---|---|
| 1 | `tools.json` 两个入口各两个工具；内置说明的余量由 `page/adapter/mcp/instructions_test.go:69` 守着；`TestUpgradeToolsMatrix`、`TestUpgradeToolsErrorsCarryTheirCodes`；prompt 在 wiki 的 `Prompts()`，`TestUpgradePromptMatrix`、UP9 |
| 2 | 冻结：`FreezeAdmits` 的 `FreezeAsk`；护栏：`TestUpgradeGuardrails`；`batch` 整个回滚 |
| 3 | 回滚照用撤销的计划与批（`page/app/jobs.go` 的 `rollbackPlan`、`revertPart`） |
| 4 | 不适用：MCP 不放弃、不开任务 |

[`M10-mode.md`](../handoffs/M10-mode.md)：

| 项 | 证据 |
|---|---|
| 1 模板 | `wiki/app/template.go:112` |
| 2、3 订阅链与 `mode` | `notebook/upgrade_states.go:29`、`:133`；收尾时与降级共用同一组订阅者（A-N2） |
| 4 一条 `UPDATE` | `notebook/adapter/postgres/queries/notebooks.sql:125-133`（`ApproveUpgrade`） |
| 5 冻结沿用单元的边界 | `bootstrap/registrants.go:266`、`:273`、`:275`，`[freeze, mode, lock]` |
| P2 补充 | `page.NewSourceRebuild`；护栏不比 `Rewrite` 的步 |
| P3/S1 补充 | 显式的契约与根，根从不为零；`TryHold`、`Runs.Do`；交付前授权；`stale_pages` 是闸门 `index`（`wiki/domain/upgrade_check.go:89`）；"不在已有事务里调"由第 187 条写明偏离 |
| P4 补充 | 旧格式先 reindex（闸门 `index`）；闸门不读 ingest 的覆盖，批准之后照 M10 推导（第 202 条）；根下的 `index`、`log` 由闸门 3 处理 |
| 收尾补充 | 已有的笔记库：README"升级为 LLM Wiki"；根下的保留标题：闸门 3；回收站里的保留标题：第 181 条，`page/domain/contract.go:141`；prompt：`UpgradePlaybook`；闸门 6 逐页；内容保全：护栏、删除的护栏与账本（第 191 条）；守卫的次序；"upgrade"一词反过来处理，`tools.json` 改称 server update；降级答 `notebook.not_llm_wiki`；`notebook.change_mode`（第 195 条） |

写给 M12 的（00 号文档第 7 节"写给后面的 M"与 P2、P3 第 9 节，C-I2）：

- [性能移交](../../M12-release/handoffs/M4-performance.md)：第 18 条"MCP"的"许多次改名之间读带链接的页"加第 270、271 条的代价；第 21 条"M11 的升级"；第 22 条"M11 收尾时组合根各份的余量与测试设施"。
- [打磨移交](../../M12-release/handoffs/M5-polish.md)：开头那句补上 M10、M11 的条号；第 38 条加升级的八个对话框（B-N4）；第 41 条并进"1 pages""1 nodes"（B-N2）；第 42–50 条是第 7 节的打磨各项，"已结束的升级的报告"（第 46 条）带上 B-Q1 看到的情形。
- [历史移交](../../M12-release/handoffs/M8-history.md)：第 6 条，撤销升级的回滚的变更集（第 302 条）。
- 第 277 条的 L1 不进移交：它是给负责人的选择，不是打磨。

## 守卫与分份的数

450 秒的线照组合根各份的 Test 一步（P3 设计第 8 节第 1 条）。

| 运行 | 提交 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
|---|---|---|---|---|---|---|---|---|---|
| 43199957538（main，S3 的合并） | `07abd0f7` | 336（9V74） | 367（7763） | 380（8573C） | 419（7763） | 302（9V74） | 273（9V45） | 238（9V45） | 315（7763） |
| 43205028299（main，记合并号与 P3 完成） | `9e787bab` | 332（9V74） | 290（9V74） | 404（9V74） | 408（7763） | 250（9V45） | 389（7763） | 315（7763） | 318（7763） |

两次的平均 334、328.5、392、413.5、276、331、276.5、316.5 秒，都低于 450。第四份 413.5 秒最贴近，两次都在 EPYC 7763 上，余量 36.5 秒；作业整体另多约 20 秒（`9e787bab` 的第四份 428 秒）。

`9e787bab` 那一次的其余各项：costs-1 在 Xeon 8573C 上 18:55（Test 一步 15:34），批准的守卫六格中位数 2.08–2.30 秒，第 298 条不触发（它要 EPYC 7763 或更慢的 runner 上任一格过 3.25 秒）；guards-core 19:33（Test 一步 19:12）；e2e 9:23（E2E 7:16）。

收尾加的两个测试：

- 决定 302 的整个程序的测试 `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages` 在第八份。名字以 `TestR` 起，原来落在补集的第四份：`BOOTSTRAP_SEVENTH`、`BOOTSTRAP_EIGHTH` 都加上它，第四份的 `-skip` 加了第七份的模式（第七份原有的名字都已被别的份收下，对它们是空操作；偏离 2）。`go test -race -list` 的 554 个顶层改后分作 145/50/120/120/61/15/30/13，每个恰在一份。本机 `-race -v` 两轮 1.39、1.53 秒；第八份包的时间 186.2 → 188.8 秒（两轮均值，约多 2.5 秒、1.4%），顶层 12 → 13；分支 CI 的第八份 Test 一步 273 秒。
- 决定 303 的性质测试 `TestTheSourceRebuildOnRandomTrees` 在 page 模块，跑在 CI 的 `race` 一份（`make test-go-race`，组合根之外的包）：本机竞态下 0.45 秒（page 整包 4.97 秒），分支 CI 里 page 包 10.4 秒。

## CI 的旧红

C 查了 M11 期间 `main` 的 31 次 CI，非第一次 attempt 就绿的都看过：

- 第 13 节已记的：`e42558f2` 红（决定 249）；`955fa579` 的 e2e 卡在 apt 的镜像、被取消；`9c7c6821` 的 attempt 2、`1b0eb605` 的 attempt 2。
- 没记的两次（C-N2），记在这里：设计阶段 `d9169baa` 的 run 42473326168 第一、二次 attempt 红，第三次绿；`9184f67b` 的 run 42476765370 第一次红，第二次绿。都红在 costs-1 的 Lint 一步：`golangci-lint crit unable to find 'v2.14.0'`，安装脚本取不到 tag，是基础设施的问题，与代码无关（同 M10 收尾 I-1 记下以前的红）。
- P3 之后的五次都是第一次 attempt 全绿；`9e787bab`（run 43205028299）第一次 attempt 19 项全绿。

## 核实修复

**第一轮**（`e96c7bc0`、`7c422c0a`、`93af4308`、`9b9bd09f`、`62dba5e4`、`f3b65b87`、`fae85831`，七个提交各自为绿，头 `fae85831`）。

- **A**：M1 的代码一侧关上：两格与它探针的两格对上，成因的注释与 `page/domain/revert.go:502`、`:506`、`:536` 一致，分份自己算过；N2 关上：`wire.go` 取一次 `notebookRegistrants`，走一遍探针里每个转移的 `mode` 帧数照旧。N1 在本提交。新的两条进第二轮：
  - **M11CA-M2（Minor）**：性质测试不给 `raw` 下的节点旧哈希，"区里的非来源要被清掉"没人核对：让 `forgetSources` 不清 `raw` 的直接子节点，`TestTheSourceRebuildOnRandomTrees` 与 `TestTheSourceRebuildIsTheInvariant` 都照过。今天普通笔记本里没有残留的哈希（降级时全清，单元在普通笔记本里不写），够不着用户，是测试强度的缺口；但决定 303 要的"等于从头重建"有一半没核对。
  - **M11CA-N3（Nit）**：新测试的注释说 flat 的第一遍"只删 `Fresh`"，实际还删发起时补写的 `raw` 与 `AGENTS`。
- **B**：N1、N3、N5 关上，没有新发现。批准的答复挡 1.5 秒，POST 之后 22 毫秒读到 404，此后到答复一直是审阅一节，答复到了去首页、焦点在标题；UP7 在三种浏览器 9 个、升级的五个 vitest 文件 225 个、探针的 walk A、D 对真服务端都过。N4 照旧（焦点仍落 `<body>`，进 M12 打磨第 38 条）。
- **C**：原发现全部关上，A-N1、B-N2、B-N4、B-Q1 也由本提交关上；文档稿在 `fae85831` 上与在 `9e787bab` 上写出的一样。新的 Nit 三条都在文档稿，本提交改了：N7，P3 设计 4.3 那句写"批准在飞时（发出到答复回来）"，与偏离 1 不符，B-N1 一行写"vitest 一格"，实际两格；N8，本记录的行号没说是哪个提交的；N9，收尾的几行日期。
- **偏离**（修复报告"与说明不同之处"八条）：1 成功之后不清 ref（B、C 判对，照说明在答复时清会在跳转之前闪出那句，变异为证）；2 第四份 `-skip` 第七份的模式（A、C 判对）；3 用 `newAcmeTeam`、另加 `revert` 变更集的数（A 判对）；4 旧哈希只写在 `raw` 之外（A 判"照字面对、覆盖不够"，缺口出在主持者的说明，第二轮修好）；5 两个生成文件的变异另写 `mutmulti.py`、6 另加同名与中文一格、7 另加一个变异、8 门禁的 `TMPDIR` 放在根分区（都判对）。

**变异**：修复报告的 14 条，没改坏的树上都过，改坏之后都红；条 3 的两格在 `9e787bab` 上红（即缺陷本身）；条 5 没有变异（没有测试依赖假服务端发不发 `mode`）。另加的三条（`notebook-deps-no-mode-subscribers`、`root-named-by-its-title-alone`、"删掉的节点也写哈希"）都在 14 条里。B 另跑网页的六条，五条红；另加的 `meanwhile-any-error`（ref 为真时任何错误都显示旧的 `data`）活着：没有一格让批准在飞时的重读答 404 以外的错，实现只认 404、比变异更窄，B 判可以接受。A 另跑七次：重建也给 `raw` 自己写哈希、只写两层、`ForgetSources` 清掉每一本、零字节的附件不写与 `upgrade-states-no-subscribers` 都红；`forgetSources` 不清 `raw` 的直接子节点对新测试与旧的不变式测试都活着，即 M11CA-M2。

**第二轮**（`1bd91a41`）：M11CA-M2，`raw` 下的节点约一半也带旧哈希（来源被覆写成正确的哈希，空页与只是文件夹的页被清掉，经单元删除的照旧由删除清掉），下限加一种"`raw` 下带旧哈希的非来源"，A 的那条变异在 `fae85831` 上活、改后死，原有的四条照样死；M11CA-N3，注释照实写第一遍还删补写的 `raw` 与 `AGENTS`、它们都不在 `Made` 下。另加一种下限"`raw` 的直接子节点里带旧哈希的非来源"（A 的变异只在那里显出来），`raw` 的直接子节点至多 10 个；四组种子每种至少 13/40；固定形状的 `TestTheSourceRebuildIsTheInvariant` 仍放过这条变异，没改（说明只要新测试抓到）；新测试竞态下 0.56 秒。核对：A 的 M2、N3 关上，那条变异在种子 1 就红，另加的两条与第一轮的四条都红；与说明不同的三处判对，没有新发现（`1bd91a41`）。

## 门禁

**逐个提交**（都经 `heavy.sh`）：服务端的四个 `make lint`、`make gen-check`、`make test-go-short` 都过，条 1 另跑第八份，条 2 跑 `TestUpgradeStatesReachTheStreams`、`TestSourceDowngradeClearsHashesAtomically`，条 7 跑 page 整包（竞态）；网页的三个 `make build`、`make lint-web`、`make knip`、`make test-web` 都过（193 个文件，4,536 → 4,537 个格），碰 `e2e/` 的条 4 另跑 UP7 的页面版本，三种浏览器都过。

**推送之前**（头 `fae85831`）：`make lint`、`make gen-check`、`make knip`、`make build` rc 0；`make test` rc 0，3,545 秒；`make e2e` 538 个全过。前两次门禁的 `make test`、`make e2e` 是红的，原因不在代码：`/tmp`（16G 的 tmpfs）到了这个用户的配额（`EDQUOT`，`df` 还显示 3.2G 空），竞态构建的链接写不进 `/tmp/go-build…`，e2e 有 8 个与升级无关的故事在浏览器写 `/tmp` 时失败；第三次把 `TMPDIR`、`GOTMPDIR` 与日志放到根分区才过（同 M10 收尾记下负载造成的红）。占着 `/tmp` 的主要是各 agent 的 vitest 留下的约 1,400 个缓存目录（约 7G）。

**第二轮**：`make lint`、`make gen-check`、`make test-go-short` rc 0；page 整包（竞态）与另外三组种子各一次都过（经 `heavy.sh`，`TMPDIR`、`GOTMPDIR` 在根分区）。

分支 CI：第一轮 run 43324605097（`fae85831`）attempt 1，19 项全绿，八份的 Test 一步最高是第四份 396 秒，第八份 273 秒；第二轮 run 43330442321（`1bd91a41`）attempt 1，19 项全绿，`race` 一份的 Test 一步 11:54，八份最高是第六份 416 秒。合并 `（待填：合并号）`，合并之后 `main` 的 CI `（待填：合并之后 main 的 CI）`。

## 没能验证的风险

- **第四份的余量**：两次 `main` 平均 413.5 秒，两次都在 EPYC 7763 上，余量 36.5 秒；`ubuntu-24.04` 的 CPU 由不得我们，再往第四份加测试之前先挪。见 [M12 的性能移交](../../M12-release/handoffs/M4-performance.md)第 22 条。
- **第 271 条在慢机上过期限**：升级中 `batch` 的计划约翻倍，M9 已接受的最坏形状（50 次改名一个 10 万个节点的文件夹、每次之后写一页带链接的页）照 EPYC 7763 的比估约 18.6 秒，过 15 秒的请求期限；过期限时什么也不写，拆开就能过。是估的，没有在 7763 上量。
- **第 302 条的缺口**：回滚的变更集在"新建的文件夹收着基线页又收着新建页"时撤销答冲突，v0.1 只钉住、不修。同一形状在 M8 的后台恢复、后台撤销里会不会出现，没有造过。结束回滚之后逐个撤销 agent 的变更集会碰到回滚已做的部分，5.8 第 6 步说的"历史里每个变更集都可以逐个撤销"在这种情形下只是"可以试"。网页的历史对类型 `rollback` 的变更集给不给"撤销"、点了显示什么，A、B 都没有看。
- **e2e 的 L1**：决定 277，照现状；根在 Chromium，本机约九次一次，P3 之后的 `main` 没有再出。向上游报问题与改测法由负责人定。
- 十万页、30 万页的发起、检查、批准与回滚，在 [M12 的性能移交](../../M12-release/handoffs/M4-performance.md)第 21 条。
- 13.4 第 6 条里"`toPageState` 或 `transferNotebooks` 不转换升级时同一个测试失败"一类的变异说法，收尾没有跑变异核对。
- 被降为编辑者的管理员停在任务页的回滚行、检查一节的"更多"上时各自何时发现，B 只走了升级页的"检查"一条，别处照 403 重读的通则。
- 第 212 条"主持者 2026-11-08 已在对话里告诉负责人"：对话不在仓库里，核对不了。

## 人工验收

这些由负责人在 v0.1 的最后执行，通过之后 M11 改为已完成：

1. [真实客户端的清单](../../M9-mcp/manual/P5-mcp-clients-checklist.md) K1–K5，Claude Code 与 Codex 各一遍，与 M9 的 A–I、M10 的 J 同一次跑。准备照清单 K 节：接入用的 PAT 必须是发起人的；一个客户端做完 K4 之后它那本进入审阅，另一个客户端用另一本。
2. 输入法清单 M11 不加步：升级的网页没有新的文字输入框（00 号文档 4.15，退回不附说明）。
3. 真实的读屏软件：升级页的横幅（`role="status"`）与对话框的拒绝（`role="alert"`）只查过角色，同 [M12 的打磨移交](../../M12-release/handoffs/M5-polish.md)第 39 条。

结果写进这一节，playbook 照记下的改；之后改 00 号文档的头与第 12 节、总体设计 12.6 与第 15 节。

| 部分 | Claude Code | Codex |
|---|---|---|
| K1 prompt 的列出 | 待执行 | 待执行 |
| K2 取 `upgrade` | 待执行 | 待执行 |
| K3 `upgrade_verify` 与 `page.upgrade_immutable` | 待执行 | 待执行 |
| K4 `upgrade_submit` | 待执行 | 待执行 |
| K5 普通笔记本取 `upgrade` | 待执行 | 待执行 |
