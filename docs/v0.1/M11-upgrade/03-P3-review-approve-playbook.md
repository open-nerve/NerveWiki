# M11/P3 审阅、批准与 playbook：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M11/P3 审阅、批准与 playbook（三个 Step：S1 审阅（服务端）；S2 playbook 与文档（服务端）；S3 网页） |
| 日期 | 2026-11-11（初稿）；2026-11-12（照设计审查第一轮修订；同日照修订核对第一、二轮再修订，加第 294 条） |
| 状态 | 进行中（设计已审查，S1 开工；[审查记录](reviews/P3-design-review.md)，三轮修订核对在它的末三节） |
| 上级文档 | [M11 总设计](00-M11-design.md) 4.3 审阅之后的几条转移、4.8 的退回、4.9、4.10 的驳回、4.11–4.20 里属于 P3 的部分、第 5、7、8、9、10 节、第 11 节（尤其 176、181、183–185、195、196、201、202、204、212、216、222、232、236、240、243、244、258、259、262、264、274、276、277）与"移交的落实""写给后面的 M""总体设计的修订"里属于 P3 的条目；[P1 设计](01-P1-freeze-start-rollback.md)（写法，3.19、3.20 的回滚与结束）；[P2 设计](02-P2-guardrails-gate-submit.md)（3.12 的提交、3.19–3.21 的修正、第 9 节交给 P3 的事）；[审查记录](reviews/)（P1、P2 的设计与各 Step）；[v0.1 总体设计](../v0.1-design.md) 5.8、5.9、6.3、13.1–13.4 |
| 代码位置的写法 | 同 00 号文档与 P1、P2 设计：模块写作 `wiki/app/abandon_upgrade.go:106`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；迁移写作 `00057_wiki_notebook_upgrades.sql:52`（`server/migrations/sql/`）；前端写作 `W/pages/notebook/upgrade-page.tsx:277`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/stories/upgrade/up5-check-submit.spec.ts:312`；契约写作 `api/modules/wiki.yaml:163`。行号都是 P2 完成之后的 `main` `955fa579`（`docs: M11/P2 done`）的 |
| 工作笔记 | 代码的核对分四份（wiki、notebook、page 的升级用例与 SQL；网页的升级页、检查一节与 `HealthRule`；组合根的测试设施、分份与 CI；MCP 的 prompt、README、人工清单与移交），本文引的每个行号都在其中核对过（附录）。实测在临时的 worktree 里、经 `heavy.sh`：批准的哈希重建在 10 万与 5 万个来源上的各段时长与计划（3.4、7.6），组合根第四、六、七、八份的本机两轮（7.5）；修订时另量决定 292 的读头语句在这本笔记本约占一成与约占四分之一的两个种子上的计划（3.4） |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P3 分三个 Step，各自一个分支（`m11-p3-s<m>`）、一次审查与修复核对、一次合并；后一个从前一个合并之后的 `main` 开始。每个提交都绿，服务端的 Step 带上它改的契约在网页与 e2e 上要的东西（生成的类型；三个新操作的码都已有两种语言的文字），网页的 service、store 方法与组件只在 S3 加（knip）。P3 没有迁移：`notebook_upgrades` 的 `approved`、`rejected` 与它们的检查（`00057_wiki_notebook_upgrades.sql:37-55`）、`notebooks` 的模式与来源根的检查（`00048`）在 P1 之前都已建好。

- **S1 审阅（服务端）**（`m11-p3-s1`）：
  - 第一个提交给组合根的第四份减负（第 278 条，7.5）；
  - wiki 的判定：`domain.ReviewOf`（审阅中的三种情形，提交、批准、退回共用，3.2）、`domain.JudgeRoots`（闸门 1–3，检查与批准共用）、批准时闸门不过的 `detail`；
  - notebook 的批准的写：`ApproveUpgrade` 一条 `UPDATE`、`UpgradeStates.Approve`（3.3）；
  - page 的来源的哈希的重建：`page.NewSourceRebuild`，用已有的四条语句与一条按键读头的新语句 `SourceRebuildHeads`（决定 292，3.4）；
  - wiki 的行：`ApproveUpgrade`、`SendBackUpgrade` 两条 SQL（3.5）；
  - `approveUpgrade`（自己的期限，3.6）、`rejectUpgrade`（与放弃共用 `rollBack`，3.7）、`sendBackUpgrade`（3.8）；提交的第一个事务改用 `ReviewOf`（3.9）；
  - 第 181 条：`CheckContract` 对撤销里的恢复判保留的标题，`revertChangeset`、`startRevert` 声明 `page.virtual_file`（3.10）；
  - 契约、组合根、矩阵的行、日志（3.11–3.14，含决定 293 的 error 一行）；交错、事件、成本守卫（7.1、7.2）；
  - 网页：生成的类型（4.1）；e2e：UP6、UP7、UP8 的 API 版本，UP6、UP7 的 MCP 版本（第 5 节）。
  - 单独为绿：合并之后管理员能经 REST 批准、驳回与退回；网页还没有这三个按钮（S3），审阅中仍可在网页上放弃（P1），所以不会卡住，这一段只在开发中的 `main` 上（同 00 号文档第 207 条与 P2/S2 的处理）。
- **S2 playbook 与文档（服务端）**（`m11-p3-s2`）：
  - prompt `upgrade`：新的用例 `UpgradePlaybook`、`wiki/domain/playbook-upgrade.md`、`api/mcp/prompts.json` 的一项（3.16、3.17）；
  - prompt 的测试改写：`TestThePromptReadsLLMWikiMatrix` 只取三个 LLM Wiki 的 prompt，`TestThePromptListsAreTheContract` 改为四，新的 `TestUpgradePromptMatrix` 与 `TestUpgradePrompt`（3.18）；
  - README 的"升级为 LLM Wiki"一节（3.19）；MCP 人工清单的 K（3.20）；
  - e2e：UP9（MCP）。网页：不改。
  - 单独为绿：prompt 只是文字（判 `lint.read`），列在每本笔记本的笔记本入口里，取它在升级中之外答 `wiki.upgrade_state`；别的 prompt 的行为不变。
- **S3 网页**（`m11-p3-s3`）：升级页的审阅一节（存下的报告经 `HealthRule` 渲染，规则的"更多"从第一页读条目，P2 第 9 节的七条）、"批准""退回继续修改""驳回"三个动作、审阅中收起"放弃"与横幅的"驳回"一句（决定 291）、"退回"在提交正在计算时的说明与 20 秒的再读、批准之后去笔记本首页（4.3）；e2e：UP6–UP8 的页面版本，UP7 在三种浏览器里跑（第 5 节）。

## 1. 基线

从 P2 完成之后的 `main` `955fa579` 开始（S3 合并为 `5d3a0f35`，其后两个提交只改文档：`git diff 5d3a0f35 955fa579 -- server api web e2e Makefile .github` 为空）。P1、P2 留下的、P3 要用的：

- **升级的用例**（`wiki/app`）：`UpgradeDeps`（`start_upgrade.go:27-51`，`Timeout` 是发起的期限，`SubmitTimeout` 是提交的接管时刻）与 `locked`（`:58-74`：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定 `notebook.change_mode` → 读一次时钟）；发起的期限的写法（`start_upgrade.go:147-176`：`ctxStart` 到期而请求的 ctx 没到期时记 warn"upgrade start timed out"、答 `shared.ServerBusy(time.Second)`）。放弃（`abandon_upgrade.go:47-98`）的 `rollBack`（`:106-128`：`SetState(rolling_back)` → `Publish` → `Rollbacks.Start` → `Rows.RollBack` 一条语句写状态、结局与任务；注释写着"The abandonment's, and the rejection's (P3)"）、`view`（`:163-172`，读 `Rows.Current`，读不到是缺陷）与 `named`（`:175-189`）。结束回滚（`end_upgrade_rollback.go`）在写之前读好要答的行，写之后改内存里的状态：`Current` 只读进行中的三种状态（`wiki/adapter/postgres/queries/upgrades.sql:10-18`），结束之后读不到。提交的第一个事务（`submit_upgrade.go:164-209`）在锁下判"审阅中"的三种：有报告答 `wiki.upgrade_state`；`SubmittedAt` 为空是缺陷；`at` 在 `submitted_at + SubmitTimeout` 之前答 `ErrUpgradeSubmitting`（同码，`detail` 是"The previous submission is still being checked: check again in a moment."，`wiki/domain/upgrade_errors.go:45-46`）；之后接管。`at` 截到微秒，`Rows.Submit` 以 `at.Add(-SubmitTimeout)` 判接管（`:195`）。
- **端口**（`wiki/app/upgrade_ports.go`）：`UpgradeStates{Lock, Start, SetState, Clear, Publish}`（`:54-60`，没有 `Approve`）；`RollbackPages{Left, DeleteBaseline}`（`:65-68`，`DeleteBaseline` 是 page 的 `DeleteUpgradeNodes`，`page/upgrades.go:116`）；`UpgradePages`（`:88-96`，含 `RootNodes(notebook, keys)`）；`UpgradeRows`（`:118-133`）；`UpgradeGatePages.Roots`（`:155-170`）。
- **wiki 的行**：`notebook_upgrades` 的检查里与 P3 有关的：`approved` 必有报告（`00057_wiki_notebook_upgrades.sql:52`）、`rejected` 必有提交（`:53`）、报告只在有提交且不是 `upgrading` 时（`:49-50`）、`reviewing` 与 `approved` 必有提交（`:46-47`）。SQL 有 `RollBackUpgrade`（`upgrades.sql:51-58`，从 `upgrading` 或 `reviewing`，不清提交与报告）、`SubmitUpgrade`（`:67-74`，带 `taken_over`）、`KeepUpgradeReport`（`:76-81`）、`UnsubmitUpgrade`（`:83-89`，要 `report IS NULL` 与这次的 `submission_id`）；store 的方法在 `wiki/adapter/postgres/store.go`（`RollBack` `:108`、`Submit` `:129`、`KeepReport` `:140`、`Unsubmit` `:157`），测试在 `store_test.go`（`TestAnUpgradesRowRollsBack` `:217`）与 `submission_test.go`（`TestTheSubmissionsRows` `:23`）。
- **闸门 1–3**：`JudgeUpgrade`（`wiki/domain/upgrade_check.go:205-258`）在一处里算：`Agents = AgentsID != nil && AgentsBody`；`SourceRoot = RootPage && (AgentsID == nil || *AgentsID != SourceRootID)`；保留的标题取 lint 读到的活节点里父为空、键在 `Reserved` 里的（`:225-230`）。`UpgradeNotReady(failed)`（`upgrade_errors.go:72-81`）的 `detail` 写"The upgrade is upgrading again: check it, then submit it again."，只对提交成立。
- **notebook**：`UpgradeStates`（`notebook/upgrade_states.go`）的每个写都要求在事务里（`postgres.InTx`），改了不是一行是缺陷；SQL `StartUpgrade`、`SetUpgradeState`、`ClearUpgrade`（`notebook/adapter/postgres/queries/notebooks.sql:104-123`）；降级的 `SetNotebookMode`（`:42-47`）无条件地写模式与来源根。降级的用例（`notebook/app/downgrade_notebook.go:56-113`）是"写模式 → 清哈希 → `PublishMode`"。`notebooks_source_root_check`（`00048`）允许普通笔记本带来源根。组合根的适配器 `wikiUpgradeStates`（`bootstrap/wiki_upgrade.go:24-80`）。
- **page 的来源区**：模块根 `SourceArea{Forget, Nodes, LintData}`（`page/source_area.go`）。语句：`ForgetSources`（`page/adapter/postgres/queries/sources.sql:59-63`）、`SourceHeads`（`:1-7`）、`SetSourceHashes`（`:46-54`，只交回真改了的行）；`SourceNodes`（`page/adapter/postgres/lint_metadata.go:33`，`LintNodeMetadata` 加 Go 里的广度优先，根不是活页时报错）；store 的 `SourceHeads`、`SetSourceHashes` 各拒绝多于 `MaxUnitNodes` 的一批（`source_area.go:19-37`、`:75-100`）。附件的元数据 `AttachmentInfos.Of`（`page/app/ports.go:602-614`），组合根交 asset 的（`bootstrap/deps.go:172`），没有文件行的不在答复里。单元算来源的哈希（`page/app/unit_sources.go:40-90`）：页取 `content_hash`、零字节的页为空；附件取 SHA-256（零字节的也有）；附件缺元数据时报错。M10 的不变式由 `checkPages` 的 `sourceHashGap`（`bootstrap/interleavings_page_test.go:85`）与 `assertSourceHashesMatchRebuild`（`bootstrap/source_hash_invariant_test.go:140`）守着。
- **第 181 条**：`CheckContract`（`page/domain/contract.go:124-156`）只对建（导入除外）、改名、移动判保留的标题（`:137-143`）；撤销里的恢复是 `OpRestore`、`Before == nil`（`page/app/unit_restore.go:113-140` 的 `bringBack`，`page/domain/revert_facts.go:54-63`），种类是 `ChangesetRevert`；回收站与后台的恢复是 `ChangesetTrashRestore`（`page/app/source_restore_precheck.go:52`），在单元里给根下保留的键加序号（`unit_restore.go:83-90`）。`revertChangeset`（`api/modules/page.yaml:1091-1107`）与 `startRevert`（`api/modules/transfer.yaml:283-301`）的码都没有 `page.virtual_file`。MCP 没有撤销的工具（`api/mcp/tools.json`），P3 不改 MCP 的码表。
- **prompt**：wiki 的 `Prompts()`（`wiki/prompts.go:9`）只交三个 playbook 的 prompt（`wiki/adapter/mcp/prompts.go:27-38`），`Playbooks.Execute`（`wiki/app/playbooks.go:28-46`）对不是 LLM Wiki 的笔记本答 `notebook.not_llm_wiki`；`PlaybookText`（`wiki/domain/playbook.go:32-49`）；`MaxPlaybookBytes = 16 KiB`（`:21`）。`api/mcp/prompts.json` 笔记本一项三个（`:3-30`）。测试：`TestThePromptReadsLLMWikiMatrix`（`bootstrap/permission_matrix_mcp_wiki_test.go:153-208`，`:205-206` 断言三个）、`TestThePromptListsAreTheContract`（`bootstrap/mcp_prompts_test.go:101`，`:105-106`）、`TestTheWikiPromptsAreReached`（`:145-259`）。没有升级的 prompt 的码表；`promptArgumentsError` 只判声明了的参数（`platform/httpserver/mcpserver/prompt.go:86-89`）。
- **契约**：`api/modules/wiki.yaml` 的升级的操作（`startUpgrade` `:112`、`abandonUpgrade` `:163-194`、`endUpgradeRollback` `:195-224`、`checkUpgrade` `:229`、`listUpgradeCheckItems` `:262`、`submitUpgrade` `:305-358`）；`UpgradeState` 已有 `approved`、`rolled_back`（`:866`），`UpgradeEnding` 已有 `rejected`（`:874`）。模块的码的测试 `TestTheUpgradesCodesAreAnswered`（`wiki/adapter/http/upgrade_test.go:187`）、`page/adapter/http/history_test.go:182`、`transfer/adapter/http/start_background_test.go:112`。
- **网页**：升级页 `UpgradeRead`（`W/pages/notebook/upgrade-page.tsx:94-356`）：审阅中的报告在 `:277-279`，在这一次进入的 `SWRConfig`（`:300`）之外；按钮一行（`:280-291`，"提交审阅…"只在 `upgrading`，"放弃"在 `upgrading` 与 `reviewing`）；`change`（`:176-204`，`wiki.upgrade_state` 等码重读升级）；`dialogOf`（`:228-234`）；`readingTree`（`:162`，树读失败时恒真）。`UpgradeReportSection`（`W/pages/notebook/upgrade-check.tsx:238-286`）自己渲染规则（`:261-281`），不经 `HealthRule`；检查一节的 `readItems`（`:159-166`，"更多"的 403 重读笔记本列表）。`HealthRule`（`W/pages/notebook/health-rule.tsx:70-245`）：`HealthCard.items: readonly LintItem[]`（`:19`）、`cursor: string | null`、`summary: LintRuleSummary`（`:85`）、`paths`（`:94`）、`read(rule, cursor: string, signal)`（`:96`）；`cursor !== null` 才渲染"更多"（`:238`）、读之前同样判（`:178`）。`UpgradeService.checkItems` 的游标必填（`W/services/upgrade.service.ts:69`）。`NotebookStore` 的升级动作经 `upgraded`（`W/stores/notebook.store.ts:195-207`）。到达的焦点 `arrived`、`useArrivalFocus`（`W/app/arrival.ts:11`、`:62`）；笔记本首页的标题接到达的焦点（`W/pages/notebook/notebook-home.tsx:34`、`:44`）。
- **测试的设施**：`upgradeIn`（`bootstrap/upgrade_fixtures_test.go:42-73`，`reviewing` 时写提交的三列、不写报告，`submitted_at` 是 `now()`）、`tm.upgraded`（`bootstrap/page_violations_upgrade_test.go:19`）、`tm.submitted`（`bootstrap/upgrade_submit_test.go:44`）、`tm.submission`（`:66`）与两个常量 `backToUpgrading`、`reviewedReported`（`:88-91`）、`cleanNote`（`:94`）、`tm.modesOf`（`:98`）；`TestUpgradeSubmit` 的 passed 一格（`:230`）以一本只有一页干净的笔记的笔记本发起、提交通过；`tm.upload`（`bootstrap/assets_test.go:66`）；发起的期限的两格与 `slowCapture`（`bootstrap/upgrade_start_test.go:216-298`）；`TestUpgradeStatesReachTheStreams`（`:388-480`）；`TestUpgradeSubmitInterleavings`（`upgrade_submit_test.go:481-691`）；`TestUpgradeCheck` 的第 8 条（`upgrade_check_test.go:469`）；`heldRules`（`upgrade_interleavings_test.go:46-69`）；`TestUpgradePermissionMatrix`（`upgrade_permission_matrix_test.go:306`，变体都没有存下的报告，`:108-112` 的注释）；`interleaveOn`（`interleavings_workspace_test.go:191`）、`notebookRow`（`interleavings_notebook_members_test.go:21`）；`wiring.wiki`（`bootstrap/wire.go:84-85`）；发起的成本守卫判计划的帮手 `startCostTrace`、`startCostPlans`（`bootstrap/upgrade_start_cost_test.go:311`、`:441-496`）。
- **分份与 CI**：组合根八份（`Makefile:247-310`），547 个顶层测试，145/50/120/124/61/11/27/9；第七份 `^Test(Upgrade|IngestBefore$$|ABackground|…)` 去掉第八份的 `^Test(Upgrade(Check|Gates|Submit|Tools)|…)`；第四份是补集（`Makefile:330-332`，`-skip` 第一、二、三、五、六份）。S3 合并之后两次（分支与 `main`）第四份都在 EPYC 7763 上，440、445 秒；P2 完成那次 `main`（run 43033276772）第四份在 9V45 上 310 秒。costs-1 的 Test 一步 11:18，作业时限 35 分钟。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P3 一行（`00-M11-design.md:517`）：审阅的三个动作（批准、驳回、退回）与批准时来源的哈希的重建；第 181 条；prompt `upgrade` 与它的 playbook、README 的"升级"一节、MCP 人工清单的 K；P3 的网页。

不做：批准之后的整体撤销（负责人第 3 条）；退回附说明（第 183 条，M12 的打磨）；已结束的升级的报告（M12 的打磨）；M11 的收尾（四份移交改为 `done`、M11 的状态、收尾的审查）。

**移交的落实**（00 号文档"移交的落实"与四份移交里 P3 的部分；设计审查 P3C-M10 补上后两份）：

| 移交的项 | P3 的落实 |
|---|---|
| [M10-mode](handoffs/M10-mode.md) 2 先写行、在同一事务里通知订阅者 | S1：批准的事务里写模式与哈希之后 `Publish`（3.6） |
| 同上 4 模式与来源根一次写入 | S1：notebook 的 `ApproveUpgrade` 一条 `UPDATE` 写模式、根并清掉升级的四列（3.3） |
| 同上 P2 补充：批准时重建严格后代的哈希、清区外 | S1：`page.NewSourceRebuild`（3.4） |
| 同上 收尾补充：回收站里的保留标题（A-M1） | S1：第 181 条（3.10） |
| 同上 收尾补充：已有的笔记库 | S2：README 的"升级为 LLM Wiki"一节写"导进普通笔记本、再升级"的路（3.19） |
| 同上 收尾补充：upgrade 的 prompt | S2：`UpgradePlaybook`，不经 `Playbooks.Execute`（3.16） |
| [M9-mcp-upgrade](handoffs/M9-mcp-upgrade.md) 1 的末句"将来的 upgrade prompt 走 P5 已有的按入口注册" | S2：`upgrade` 以 `Entry: NotebookEntry` 登记在 wiki 的 `Prompts()`（3.16） |
| [M5-events](handoffs/M5-events.md) 模式变化的事件 | S1：批准、驳回、退回经模式变化的订阅链发 `mode`，`TestUpgradeStatesReachTheStreams` 加步（7.1）；S3：批准之后跳到笔记本首页、升级页卸下，之后的 `mode` 与重连不再读 `["upgrade", id]`（SWR 只重读装着的键，4.3） |
| [M8-P4-background](handoffs/M8-P4-background.md) 5 客户端 | S1：驳回经 REST，回滚任务的客户端是 `web` 或 `api`（3.7 的 `ClientOf`）；第 1–4 条在 P1 已落实或不适用 |

### 2.1 对 00 号文档的修订

照代码与实测，00 号文档有下面几处要改（行号是 `955fa579` 的 00 号文档的）。接受本文的文档提交以脚本照改（`p3-apply-00.py`，每处一个带标签的替换，一处不匹配就什么也不写），第 8 节与第 9 节在同一个提交里一并改：

1. **批准有自己的期限，到期答 503 `server_busy`**（第 5 节 `approveUpgrade` 一行 `:442`、4.9、4.16 的码的次序 `:372`）：批准在笔记本行的锁下重建全部来源的哈希。实测（7.6）：10 万个来源（9.1 万页、4,000 个附件，库里另有九本各 5 万页的 LLM Wiki 与它们的升级）整个重建本机 2.4–3.7 秒，大头是 `SetSourceHashes` 的约 9.5 万行的 `UPDATE`（1.9–2.6 秒：`nodes` 的每个索引都要改，不是 HOT 的更新），语句换写法（连接、`= ANY`、`LATERAL`，一块一万或全部一次）都在 2.1–3.1 秒，底在行的写本身；EPYC 7763 约是本机的两倍。默认 4 秒的请求（组合根的测试应用）放不下 10 万个来源，生产的 15 秒放得下。照发起的写法（第 219、227、232 条）：批准的事务受 `ApproveTimeout = request_timeout × 3/4` 约束（生产 11.25 秒），到期答 503 `server_busy`、`Retry-After: 1`、记一行 warn（4.18 加这一行），什么也不写；`approveUpgrade` 的码加 `server_busy`。不分批提交：模式与哈希要在同一个事务里成立（M10 的不变式，`bootstrap/interleavings_page_test.go:52-55` 的注释），分开提交会让中间的状态违反它（第 279 条，3.6）。
2. **批准的成本守卫定在 5 万个来源，十万进 M12 的性能移交**（第 9 节成本守卫表"批准"一行 `:701`"10 万个来源 | 同上（基线行的 `DELETE`）| 请求 4 s"）：10 万个来源本机两轮中位数的三倍约 11 秒，等于生产的期限，守卫的界没有意义；照第 232、262 条调量级：5 万个来源，预算是批准实际受的期限（守卫以生产的 15 秒请求配，`ApproveTimeout` 11.25 秒），界取本机两轮较大的中位数 × 3；计划为主（第 280 条，7.1）。10 万的实测写进 M12 的性能移交。
3. **笔记本的批准是自己的一条 `UPDATE`**（4.9 第 3 步 `:266`"`SetNotebookMode` 加列，`notebooks.sql:42-47`"）：`SetNotebookMode` 是降级无条件的写（`WHERE id = $1`，`:exec`），加了升级的四列就让降级也去清它们，而降级时笔记本不会有升级（升级中是普通的笔记本，`downgradeNotebook` 答 `notebook.not_llm_wiki`）。改为 notebook 新的 `ApproveUpgrade`（`:execrows`，只在 `upgrade_id` 是这一次、`upgrade_state = 'reviewing'` 时写（有升级的笔记本一定是普通的，`notebooks_upgrade_check`），改的不是一行是缺陷），`SetNotebookMode` 不动（第 281 条，3.3）。
4. **批准时闸门不过的 `detail` 另写**（4.9 第 2 步 `:265`）：`UpgradeNotReady` 的 `detail` 写"The upgrade is upgrading again: check it, then submit it again."，批准时升级仍在审阅中，这句话不实。同码 `wiki.upgrade_not_ready`，`detail` 是"These gates did not pass: <键>. The upgrade is still in review: send it back, or reject it."（`domain.ApprovalNotReady`，第 282 条，3.2）。这条路只有缺陷才到得了：批准在答 409 的同时记一行 error（`notebook_id`、`upgrade_id`、`failed_gates`），4.9 第 2 步与 4.18 随之各加一句（决定 293，3.6、3.14）。
5. **存下的报告不通过是缺陷**（4.9 第 1 步 `:264`；P2 第 9 节"且报告的 `passed` 为真"）：第二个事务只在检查通过时存报告（`KeepReport`，P2 3.12），存下的报告 `passed` 为假到不了；批准读到它答 500、记 error，不答 409（变异"存下不通过的报告"由提交的测试抓，`TestUpgradeSubmit` 的 not ready 一格断言行回到 `upgrading`、没有报告）。
6. **prompt `upgrade` 没有参数，没有 `validation_failed`**（第 5 节 MCP 表 prompt 一行 `:484`"参数过界的 `validation_failed` 照 `TestTheWikiPromptsAreReached` 的做法"）：`promptArgumentsError` 只判声明了的参数（`platform/httpserver/mcpserver/prompt.go:86-89`），没有参数的 prompt 答不出它。码只有 `notebook.not_found`（看不到的笔记本在 HTTP 的边上就答 404，`platform/httpserver/mcpserver/edge.go:233-250`；用例里判定 `lint.read` 的 `ErrNotVisible` 与删掉的笔记本也是它）与 `wiki.upgrade_state`。
7. **矩阵里批准的管理员一格答 409 `wiki.upgrade_state`**（4.14 `:343`"`reviewing` 且有存下的报告"）：P2 的变体都没有存下的报告（`bootstrap/upgrade_permission_matrix_test.go:108-112`：lab 的笔记本没有 `AGENTS`，提交过不了闸门；以 SQL 写一份报告要在测试里复制适配器私有的 JSON 形状），`reviewing` 的变体是"审阅中、没有报告、刚提交"。照第 260 条的写法：矩阵钉的是谁能批准，409 与 403、404 分得开；批准通过由 `TestUpgradeApproval` 与 UP7、UP8 盖住。退回另用一个变体"审阅中、没有报告、一小时之前提交"（过了接管的时刻，管理员 200），驳回用 `reviewing`（管理员 200）（3.13）。
8. **审阅一节的"更多"从第一页读**（4.15"审阅（P3/S3）" `:359`"'更多'在没有游标时被 `HealthRule` 藏起（`health-rule.tsx:210`），'从第一页读'做成它的参数，P3/S3 写代码时核对"）：核对了，行号是 `:238`（渲染）与 `:178`（读之前）。改为：`HealthCard.cursor` 是 `string | null | undefined`，`undefined` 是"下一次从第一页读"，`null` 是"没有更多"；存下的报告的卡初始为 `undefined`（数比条目多或封顶时）或 `null`（条目已是全部）；读一页的 `read` 的游标可缺（P2 第 9 节第 3、6 条，4.3）。
9. **第 9 节的测试名**（`:675`"`TestUpgradeApproval`（…）、`TestUpgradeRollback`（放弃、驳回、…）"与"`TestUpgradePrompt`"）：驳回的回滚与放弃的是同一个 `rollBack`，`TestUpgradeRollback` 不加驳回的格；驳回、退回与它们的码在新的 `TestUpgradeReview`，批准与驳回、批准与删除笔记本、批准与正在等行锁的写在新的 `TestUpgradeReviewInterleavings`（7.1、7.2）。
10. **审阅中，网页收起"放弃"**（4.15 的"放弃"一项 `:361`"`upgrading`、`reviewing` 都有"；4.15 的横幅 `:351`"任何笔记本管理员都可以放弃这次升级"（后一句只在 `upgrading`、`reviewing`，P1/S4 决定 246）"；4.3 的"发起人离开" `:122`"横幅写清'任何笔记本管理员都可以放弃'"；第 11 节决定 246 的条目 `:842`）：决定 291。4.15 的"放弃"一项改为"`upgrading` 有；审阅中由'驳回'代替（决定 291）"；横幅那一句改为管理员在 `upgrading` 看到"任何笔记本管理员都可以放弃这次升级"、在 `reviewing` 看到"任何笔记本管理员都可以驳回这次升级"，`rolling_back` 都不写（P1/S4 决定 246；决定 291）；4.3 那一句补"（审阅中写'驳回'，决定 291）"；决定 246 的条目末尾加一句"P3 起审阅中的那一句改为'驳回'（决定 291）"。API 不变：`abandonUpgrade` 照旧接受 `reviewing`，4.3 的状态表、契约、MCP 与矩阵都不改。
11. **人工清单的 K 是五步，改原有页的正文在提交之前**（4.20 `:428`"K1 …；K2 取它；K3 让客户端调 `upgrade_verify`、`upgrade_submit`，…并让它改一页原有页的正文…；K4 在普通笔记本取 `upgrade`"）：K3 提交通过之后升级进入审阅、对所有人冻结，再改正文答的是 `notebook.upgrading`，看不到 `page.upgrade_immutable`。照 3.20 改为 K1–K5 的新次序（设计审查 P3C-M9、P3C-N19）。
12. **第 10 节风险表的成本守卫**（`:723`"10 万页的成本守卫；30 万页进 M12"）：发起（第 232 条）、检查（第 262 条）早已是 5 万页，本文把批准调到 5 万个来源（第 2 条），批准另有自己的期限（第 1 条）。改为"发起在行锁下只试取 lint 名额、整个事务受发起的期限，批准受它自己的期限；发起、检查与批准在 5 万页（批准是 5 万个来源）上、回滚的计划、合成与先删在 10 万上有成本守卫；10 万页与 30 万页进 M12；都是一次性的管理员动作"（设计审查 P3C-N19）。

### 2.2 需要负责人确认

无。上面十二条里，第 1–9、11、12 条是照代码与实测改说法、位置或做法，不改负责人与主持者的决定：第 1 条给批准加一个它在大的来源区上本来就会撞上的期限（答 503 而不是平台在请求期限上答 500），第 2 条照第 232、262 条调守卫的量级，第 3 条让降级的写保持原样，第 7 条照第 260 条，第 11、12 条让 00 号文档跟上 3.20 与第 2 条。第 10 条是主持者的决定 291：网页上审阅中由"驳回"代替"放弃"，碰到负责人第 1 条的原话（"任何笔记本管理员都能随时放弃"），效果相同（后台回滚、放回发起时的样子），API 不变；主持者在 M11 的最后报告里单独列出，负责人可以改判。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

00 号文档没有定、本文取的做法。278–290 是初稿的十三条（初稿的临时编号 P3-1 到 P3-13，设计审查之后照次序给了正式编号，P3-k 是 277 + k）；291–293 是审查之后新定的三条，285、288 照它们改写；294 是修订核对第二轮之后新定的。接受本文的文档提交把它们写进 00 号文档第 11 节（`p3-apply-00.py`），另起一节"<接受本文的日期>：P3 的设计审查之后的取舍 278–…（主持者的决定，负责人可以改判）"，接在"2026-11-11：P2/S3 的取舍 277"一节之后：

- **278　S1 的第一个提交把第四份的四个测试挪到第六份**（7.5）：`BOOTSTRAP_SIXTH` 由 `^TestIngest` 改为 `^Test(Ingest|TheBackgroundRevertStopsAtAWriteBetweenItsBatches$$|TheBackgroundRevertOfASegmentPastABatchStops$$|RevertingTheNewestChangesetsPutsTheNotebookBack$$|SoleAdminDeactivatingWhileOneJoins$$)`；第四份的 `-skip` 本就含第六份的模式（`Makefile:332`），自动少了它们。四个本机两轮的均值合计 29.7 秒。比用包对包（7763 上 `ok … bootstrap` 一行的包时间对本机两轮的包；Test 一步另多约 32 秒的竞态构建，挪测试不改它）：第四份 410.1/224.37 = 1.828，第六份 294.8/213.21 = 1.383，四个折到 7763 约 41–54 秒；第四份约 442.5 → 394–407 秒，第六份约 327 → 374–387 秒（都加契约约 1.5%；设计审查 P3B-M5 改正初稿以 Test 一步算的比）。理由：第六份的余量在挪得动的几份里最大，而且 P3 不往它加测试；第七、八份要收下 P3 的测试（第 290 条）；开第九份要多一个 CI 作业，眼下不需要。四个都不是所在份里第一个跑的（第四份第一个是 `TestTheAPIConfigBoundsTheLargeBodies`，第六份是 `TestIngestCleanup`，挪来的文件名都排在 `ingest_*` 之后），不换付冷启动的测试（M11/P1 的教训）。不提高 `-timeout`。
- **279　批准有自己的期限 `ApproveTimeout`**（2.1 第 1 条，3.6）：`wiki.Deps` 与 `UpgradeDeps` 各加一项，组合根填 `cfg.Server.RequestTimeout * 3 / 4`（同发起的 `StartTimeout`，`bootstrap/wire.go:278`）；不与发起共用一项：期限的测试要只缩批准的，名字也说清是谁的。到期答 503、`Retry-After: 1`、warn 一行 `upgrade approval timed out`；用例照发起的写法判"是批准自己的期限"（`ctxApprove.Err() != nil && ctx.Err() == nil`）。
- **280　批准的成本守卫在 5 万个来源，计划为主**（2.1 第 2 条，7.1）：每次批准在库的一份新副本上（同矩阵，`pgtest.NewDatabaseFrom`）：新副本没有前一次回滚留下的死行与膨胀的索引，不让它们改了下一次的计划（设计时的探针在同一个库里反复回滚，第二轮里分析过的统计下 `ForgetSources` 与 `LintNodeMetadata` 换成了整表的顺序扫描，7.6）。被判的语句列出来，小表的点读不判（设计审查 P3B-I1）；另有一个只核对计划的种子（决定 292）。
- **281　笔记本的批准是自己的一条 `UPDATE`**（2.1 第 3 条，3.3）。
- **282　批准时闸门不过另写 `detail`**（2.1 第 4 条，3.2）。
- **283　"审阅中"的三种情形由 `domain.ReviewOf` 一处判定**（3.2）：没有报告且在接管的时刻之前（`ReviewChecking`）、没有报告且已过（`ReviewStale`）、有报告（`ReviewReported`）；提交的第一个事务、批准、退回各自照它分支（驳回不分：审阅中都可以）。SQL 里同一个判断另有一份（`SubmitUpgrade`、`SendBackUpgrade` 的 `taken_over`、`ApproveUpgrade` 的 `report IS NOT NULL`），守着锁下的读与写之间不变的事实；每一处各有自己的测试格（05-lessons"两处复写的同一判断各自有测试"，7.1）。
- **284　闸门 1–3 的判定提成 `domain.JudgeRoots`，检查与批准共用**（3.2）：批准在锁下读 `Gates.Roots(notebook, root)` 与 `Pages.RootNodes(notebook, ReservedKeys)`（两条都是按键的点读，`page/upgrades.go:123-158`），不跑 lint。检查照旧从 lint 的节点里取根下的节点，交给同一个函数。
- **285　哈希的重建用已有的 `ForgetSources`、`SourceNodes`、`AttachmentInfos.Of`、`SetSourceHashes`，读头另写一条按键的语句（决定 292）**（3.4）：先 `ForgetSources`（清掉整本的，含区外的），再读来源区的成员（`SourceNodes`），页分块读 `SourceRebuildHeads`、附件分块读 `AttachmentInfos.Of`，分块 `SetSourceHashes`；每块写的行数要等于块的大小（清过之后每行都从空变为有值，`SetSourceHashes` 只交回真改了的行）。实测换写法省不下时间（2.1 第 1 条），除读头之外不另写 SQL。（初稿写"只用已有的语句"、页读 `SourceHeads`，设计审查 P3B-M6 之后照决定 292 改写。）
- **286　`UpgradePlaybook` 读 wiki 的行判状态，playbook 的文字另一个函数**（3.16）：wiki 的 `Notebooks.StateOf` 不带升级（`bootstrap/wiki_lint.go:26-41`），`Rows.Current` 是这个模块自己的表；文字是 `domain.UpgradePlaybookText()`，不进 `PlaybookText` 的 `switch`（那样 `Playbooks.Execute` 在 LLM Wiki 上会答出升级的文字）。
- **287　审阅一节挪进这一次进入的 `SWRConfig`，规则经 `HealthRule` 渲染**（2.1 第 8 条，4.3）：`HealthCard.items` 放宽为 `readonly (LintItem | UpgradeReportItem)[]`，`summary` 只要 `rule`、`count`、`count_capped`；`readItems` 提成模块里的一个函数，检查与审阅两节共用，各测一格 403。
- **288　"批准"只在有报告时显示；"退回""驳回"在审阅中一直显示；"放弃"只在 `upgrading`（决定 291）**（4.3）：没有报告时批准一定答 409，按钮只会让人碰壁；退回照 00 号文档 4.15 一直显示（期限之内答 409 的说明与 20 秒的再读）；驳回在审阅中都可以（4.3 的状态表）；审阅中由"驳回"代替"放弃"（决定 291）。（初稿让"放弃"在审阅中也一直显示，设计审查 P3C-Q1 之后照决定 291 改写。）
- **289　批准之后去笔记本首页，焦点给标题**（4.3）：照建笔记本的对话框（`W/pages/workspace/create-notebook-dialog.tsx:59`），`navigate(..., { state: arrived })`；笔记本首页已接到达的焦点。
- **290　P3 的组合根测试分在第七、八两份**（7.5）：S1 的第一个提交把第八份的模式扩为 `^Test(Upgrade(Approval|Check|Gates|Review|Submit|Tools)|VirtualExport$$|TheSourceRebuildMatchesSuccessfulRandomSteps$$|TheMCPToolsErrorsCarryTheirCodes$$)`（第七份照旧 `-skip` 它）：较重的 `TestUpgradeApproval`、`TestUpgradeReview`、`TestUpgradeReviewInterleavings` 进第八份；`TestUpgradeTrashReservedTitles`、`TestUpgradePromptMatrix`（第 222 条）、`TestUpgradePrompt` 与已有测试里加的格在第七份。照 P2 第 9 节："第七份放不下时扩第八份的模式"。估计（7.5，包对包的比）：第七份 7763 上约 317 → 345 秒，第八份 289 → 350–375 秒；全放进第七份时约 405–430 秒，余量比两份分摊时小。
- **291　审阅中，网页收起"放弃"**（设计审查 P3C-Q1；2.1 第 10 条，4.3）：`reviewing` 时按钮一行只有"批准…"（只在有报告时）、"退回继续修改…"、"驳回…"，不给"放弃这次升级…"；`upgrading` 照旧有"放弃"。API 不变：`abandonUpgrade` 照旧接受 `reviewing`（契约、MCP、矩阵、P1 的测试都不动）。横幅在 `reviewing` 时管理员看到的那一句改为"任何笔记本管理员都可以驳回这次升级"，`upgrading` 照旧写"放弃"。理由：审阅中的驳回与放弃，用户看得到的后果完全一样（后台回滚、放回发起时的样子），网页从不显示 `ending`，事前事后都分不出；两个相邻的 `destructive` 按钮只会让人问"按哪个"，与第 248 条收起停下时的"放弃"是同一个理由；负责人第 1 条"任何笔记本管理员都能随时放弃，放弃时整体回滚"在审阅中由"驳回"兑现，效果相同。这一条碰到负责人的原话，主持者在 M11 的最后报告里单独列出。
- **292　批准的哈希重建另有一条按键读头的语句**（设计审查 P3B-M6、P3B-Q1；3.4、7.1）：照决定 240 的先例，重建另写 `SourceRebuildHeads`，只给重建用：从 `unnest($ids)` 出发，以 `LATERAL` 按主键读 `nodes`，再以 `LATERAL` 按 `page_contents` 的主键读 `content_hash`、`byte_size`，笔记本与删除的条件在外面判；`SourceHeads` 与它现有的两个调用者（`page/app/unit_sources.go:47`、`page/app/source_precheck.go:65`）不动。守卫另加一个只核对计划的种子（这本笔记本约占实例的四分之一），只判重建的几条语句的计划、不计时：新语句两种计划都只经键，`page_contents` 不顺序扫描。理由：这本笔记本约占 `page_contents` 的两成以上、分析过之后，`SourceHeads` 的自定义计划（经 `Planned`，就是生产的计划）每块顺序读整张 `page_contents`（审查 B 实测 23%、38%，修订时复测 22%）；决定 240 管的是读了谁的行，不是时长；这是一条语句加一个只核对计划的种子，不是调优，与负责人 2026-11-06 的话不冲突。
- **293　批准时锁下再核闸门 1–3 不过，在答 409 的同时记一行 error**（设计审查 P3A-Q1；2.1 第 4 条，3.6、3.14）：这条路只有缺陷才到得了；检查遇到同一类缺陷时记 error、答 500（`wiki/app/check_upgrade.go:229-233`）。批准照旧答 409 `wiki.upgrade_not_ready`（`detail` 照 2.1 第 4 条），另记一行 error：`notebook_id`、`upgrade_id`、`failed_gates`（不过的闸门的键，同提交的那一行），记在事务回滚之后、答复之前（同 13.1 第 10 条）；以 `errors.Is(err, domain.ErrUpgradeNotReady)` 认、答 `err` 本身，回滚失败时照 `WithinTx` 交回的错误答（设计审查 P3A-N11）。
- **294　S1 加批准守卫的那个提交把 costs-1 的作业时限从 35 分钟提到 45 分钟**（修订核对第二轮 P3B-M8；7.5、6.1）：照决定 267（costs-2 35 → 45）的先例与 `ci.yml` 的"时限约为各份在 CI 上实测的两倍"（runner 的 CPU 可差两倍，`ci.yml:38`、`:85`）。理由：今天 costs-1 的作业 14:31–15:41（三次），P3 加这次编译约 1 分钟与守卫的 7–9 分钟之后约 23–26 分钟，两倍是 46–52 分钟，35 分钟只有约 1.35–1.5 倍，比它慢 1.4–1.5 倍的 runner 会在第 35 分钟被整个取消、Go 来不及打出卡住的测试；"最后一行开始的时刻加它的 Go 时限在作业时限之前"之外，作业时长的两倍也要满足。`ci.yml` 那一行的注释写实测的三次与估计。

## 3. 服务端

### 3.1 S1 的文件

| 文件 | 改动 |
|---|---|
| `wiki/domain/upgrade_review.go`（新） | `Review`、`ReviewOf`（3.2） |
| `wiki/domain/upgrade_check.go` | `RootGates`、`JudgeRoots`；`JudgeUpgrade` 的闸门 1–3 改由它算（3.2） |
| `wiki/domain/upgrade_errors.go` | `ApprovalNotReady`（3.2） |
| `wiki/app/upgrade_ports.go` | `UpgradeStates.Approve`；`UpgradeRows.Approve`、`SendBack`；新端口 `SourceHashes`（3.3–3.5） |
| `wiki/app/start_upgrade.go` | `UpgradeDeps` 加 `ApproveTimeout`、`SourceHashes`（3.6） |
| `wiki/app/approve_upgrade.go`、`reject_upgrade.go`、`send_back_upgrade.go`（新） | 三个用例（3.6–3.8） |
| `wiki/app/submit_upgrade.go` | `first` 改用 `ReviewOf`（3.9） |
| `wiki/adapter/postgres/queries/upgrades.sql`、`store.go`、`gen/` | `ApproveUpgrade`、`SendBackUpgrade`（3.5） |
| `wiki/adapter/http/upgrade.go`、`handler.go` | 三个处理函数，`UpgradeUseCases` 加三个方法（3.11） |
| `wiki/lint.go` | `Deps` 加 `SourceHashes`、`ApproveTimeout`；`Module` 加三个用例与方法（3.12） |
| `notebook/adapter/postgres/queries/notebooks.sql`、`store.go`、`gen/`；`notebook/upgrade_states.go` | `ApproveUpgrade`、`UpgradeStates.Approve`（3.3） |
| `page/app/source_rebuild.go`、`page/source_rebuild.go`（新） | `RebuildSources`、`SourceRebuild`、`NewSourceRebuild`（3.4） |
| `page/adapter/postgres/queries/sources.sql`、`source_area.go`、`gen/` | `SourceRebuildHeads`（决定 292，3.4）；`SourceHeads` 不动 |
| `page/domain/contract.go` | 第 181 条（3.10） |
| `api/modules/wiki.yaml`、`page.yaml`、`transfer.yaml`；`api/dist/openapi.yaml`；各模块的 `adapter/http/gen` | 三个操作；两个撤销的 `page.virtual_file`（3.11） |
| `bootstrap/wiki_upgrade.go`、`bootstrap/wire.go` | `wikiUpgradeStates.Approve`；`SourceHashes`、`ApproveTimeout` 的接线（3.12） |
| `Makefile`、`.github/workflows/ci.yml` | 第四份挪四个测试到第六份、第八份的模式扩大（第 1 个提交，第 278、290 条，7.5）；costs-1 加批准的守卫一行（7.1），作业时限 35 → 45 分钟（第 294 条） |
| `web/packages/api-client/src/schema.gen.ts` | 生成（4.1） |
| `e2e/fixtures/upgrades.ts`、`e2e/stories/upgrade/up6-…`、`up7-…`、`up8-…` | UP6–UP8 的 API 与 UP6、UP7 的 MCP 版本（第 5 节） |
| 测试 | 各节写出；组合根的在 7.1 |

### 3.2 S1：wiki 的判定

**审阅中的三种情形**（新文件 `wiki/domain/upgrade_review.go`）。提交的第一个事务今天自己判（`submit_upgrade.go:181-190`），批准与退回又各要一份；提成一个函数，三处照它分支（第 283 条）：

```go
// Review is what an upgrade in review holds as an admin acts on it (M11
// design 4.3, 4.8): its report, kept as its check passed; none, its
// submission still being checked; none, past the moment another
// submission or a send-back may take it over.
type Review int

// An upgrade's reviews.
const (
	ReviewReported Review = iota + 1
	ReviewChecking
	ReviewStale
)

// ReviewOf is up's review at at, timeout the bound of a submission's part
// apart from its request (SubmitTimeout): its report decides first; then
// at before submitted_at plus timeout is still checking, and at or after
// it is stale. up not reviewing, or reviewing without a submission, is a
// defect (00057's checks forbid the latter).
func ReviewOf(up Upgrade, at time.Time, timeout time.Duration) (Review, error) {
	switch {
	case up.State != UpgradeReviewing:
		return 0, fmt.Errorf("wiki: the review of the upgrade %s, %s, a defect", up.ID, up.State)
	case up.Report != nil:
		return ReviewReported, nil
	case up.SubmittedAt == nil:
		return 0, fmt.Errorf("wiki: the upgrade %s reviewing has no submission, a defect", up.ID)
	case at.Before(up.SubmittedAt.Add(timeout)):
		return ReviewChecking, nil
	}
	return ReviewStale, nil
}
```

调用者各自的分支：

| 调用者 | `ReviewReported` | `ReviewChecking` | `ReviewStale` |
|---|---|---|---|
| 提交的第一个事务（3.9） | `ErrUpgradeState` | `ErrUpgradeSubmitting` | 接管，写新的提交 |
| 批准（3.6） | 往下（`Report.Passed` 为假是缺陷） | `ErrUpgradeState` | `ErrUpgradeState` |
| 退回（3.8） | 退回 | `ErrUpgradeSubmitting` | 退回（接管） |
| 驳回（3.7） | 不调：审阅中都可以驳回 | | |

`ErrUpgradeSubmitting` 的 `detail` 照旧（"The previous submission is still being checked: check again in a moment."），退回也用它：同一个事实，网页另有自己的文字（4.3）。批准在没有报告时答普通的 `ErrUpgradeState`，不说"正在计算"：期限已过的那种也不能批准，而且批准不靠等。

单元测试 `wiki/domain/upgrade_review_test.go` 的 `TestReviewOf`：时刻与期限取互不相同的值（`submitted_at` 是 `12:00:00.000001`，期限 17 秒），每格整值断言；"有报告而 `submitted_at` 还在期限之内"答 `ReviewReported`（报告先判）；期限前 1 微秒 `ReviewChecking`、恰在期限 `ReviewStale`、之后 1 微秒 `ReviewStale`（边界与提交的 SQL 的 `submitted_at <= taken_over` 一致，3.5）；`upgrading`、`rolling_back` 各答错误；审阅中而 `SubmittedAt` 为空答错误。

**闸门 1–3**（`wiki/domain/upgrade_check.go`，第 284 条）：

```go
// RootGates are gates 1 to 3 (M11 design 4.7): the root's AGENTS page with
// a body; the proposed source root a live page, not the root's AGENTS; no
// node at the root keyed a reserved title, the ones there by id.
type RootGates struct {
	Agents, SourceRoot bool
	ReservedTitles     NodesGate
}

// JudgeRoots judges gates 1 to 3 from what they read: roots, the proposed
// source root, and the ids of the nodes at the root keyed a reserved
// title, in any order. The check's and the approval's (M11/P3 design 3.2).
func JudgeRoots(roots UpgradeRoots, sourceRootID uuid.UUID, reserved []uuid.UUID) RootGates {
	g := RootGates{
		Agents:     roots.AgentsID != nil && roots.AgentsBody,
		SourceRoot: roots.RootPage && (roots.AgentsID == nil || *roots.AgentsID != sourceRootID),
	}
	g.ReservedTitles.NodeIDs = slices.Clone(reserved)
	slices.SortFunc(g.ReservedTitles.NodeIDs, uuid.UUID.Compare)
	g.ReservedTitles.Passed = len(reserved) == 0
	return g
}

// Failed are the gates of g that did not pass, in their order.
func (g RootGates) Failed() []UpgradeGateKey
```

`JudgeUpgrade` 里 `:221-231` 换成：收集 `r.Nodes` 里父为空、键在 `r.Reserved` 里的 id，调 `JudgeRoots`，把三项抄进 `UpgradeGates`。检查的行为不变：`upgrade_check_test.go` 的现有各格照旧通过（闸门 1–3 的五格在 `:84-93`）。`TestJudgeRoots`（新）：每道闸门一个反例（`AgentsID` 为空；正文空；根是 `AGENTS`；根不是活页；根下一个保留的键），全过一格，`reserved` 乱序交入时交回的 `NodeIDs` 有序、是副本（改交入的切片不影响结果）；`Failed` 的次序是键的次序。

**批准时闸门不过**（`wiki/domain/upgrade_errors.go`，第 282 条）：

```go
// ApprovalNotReady is ErrUpgradeNotReady at an approval: gates 1 to 3,
// checked again under the notebook's row lock, do not pass (M11 design 4.9
// step 2), which only a defect reaches, the upgrade frozen in review; it
// is still in review.
func ApprovalNotReady(failed []UpgradeGateKey) error {
	e := *ErrUpgradeNotReady
	e.Detail = "These gates did not pass: " + keysOf(failed) + ". The upgrade is still in review: send it back, or reject it."
	return &e
}
```

`UpgradeNotReady` 与它共用一个 `keysOf`。单元测试：`detail` 整句断言，码是 `wiki.upgrade_not_ready`、409。

### 3.3 S1：notebook 的批准的写

`notebook/adapter/postgres/queries/notebooks.sql` 加（第 281 条）：

```sql
-- name: ApproveUpgrade :execrows
-- A notebook's upgrade approved (M11 design 4.9 step 3), under the row's lock: an LLM Wiki with the upgrade's source
-- root, its four upgrade columns cleared, in one statement; the notebook's upgrade is still upgrade_id, in review.
-- notebooks_upgrade_check keeps a notebook with an upgrade standard.
UPDATE notebooks
SET mode = 'llm_wiki', source_root_id = sqlc.arg(source_root_id)::uuid, upgrade_id = NULL, upgrade_state = NULL,
    upgrade_by_id = NULL, upgrade_at = NULL, updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND upgrade_id = sqlc.arg(upgrade_id)::uuid AND upgrade_state = 'reviewing';
```

`notebook/upgrade_states.go`：

```go
// Approve ends the notebook's upgrade upgradeID, in review, as an LLM Wiki
// whose source root is rootID, by by at at (M11 design 4.9 step 3): the
// notebook is locked and its upgrade is upgradeID in review, which its
// caller judged under the lock; a row unchanged is a defect, as a zero
// root (00048's check would refuse no root, and the zero id is none).
func (u *UpgradeStates) Approve(ctx context.Context, id, upgradeID, rootID, by uuid.UUID, at time.Time) error
```

照 `SetState`（`:74-88`）：先 `postgres.InTx`，零的 `rootID` 答错误，改的不是一行答缺陷的错误。它不 `Publish`：用例在重建哈希之后再发（4.9 第 7 步，审查 B-N3）。wiki 的端口 `UpgradeStates`（`wiki/app/upgrade_ports.go:54-60`）加同样签名的 `Approve`，组合根的 `wikiUpgradeStates`（`bootstrap/wiki_upgrade.go`）转调。

测试（`notebook/upgrade_states_test.go`，真库）：`TestAnUpgradeIsApproved`：发起、`SetState(reviewing)` 之后批准，整行断言（模式、根、四列为空、审计列是批准人与时刻，名称等别的列不变）；反例各一：别的 `upgrade_id`、`upgrading`、`rolling_back`、零的根，各答错误、行不变；不在事务里答错误。`mode = 'standard'` 不写进 `WHERE`：`notebooks_upgrade_check`（`00055_notebook_notebooks_upgrade.sql:11-14`）让有升级的笔记本一定是普通的，那一项是造不出反例的合取项。

### 3.4 S1：page 的来源的哈希的重建

**做什么**（00 号文档 4.9 第 4 步，第 285、292 条）：在批准的事务里（它持着笔记本行的 `FOR NO KEY UPDATE`，写入单元都在它之后），先清掉这本笔记本全部活节点的来源哈希（含来源区之外的，M10 的移交"清区外"），再为提议的来源根的每个严格活后代写上：正文字节数大于 0 的页取 `page_contents.content_hash`；附件取它的文件的 SHA-256（零字节的也有）；空页与只是文件夹的页不写。这正是 `checkPages` 的 `sourceHashGap` 与 `assertSourceHashesMatchRebuild` 核对的不变式，也是单元维护它的规则（`page/app/unit_sources.go:40-90`）。

`page/app/source_rebuild.go`（新）：

```go
// SourceRebuildStore is what a source area's rebuild reads and writes, in
// its caller's transaction: the source area's statements (sources.sql,
// LintNodeMetadata) and the rebuild's own heads (decision 292).
type SourceRebuildStore interface {
	ForgetSources(ctx context.Context, notebook uuid.UUID) error
	SourceNodes(ctx context.Context, notebook, root uuid.UUID) ([]SourceNode, error)
	SourceRebuildHeads(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]SourceValue, error)
	SetSourceHashes(ctx context.Context, notebook uuid.UUID, values []SourceValue) (map[uuid.UUID]SourceValue, error)
}

// RebuildSources sets notebook's source hashes as an LLM Wiki whose source
// root is root wants them (M10 design 4.8, M11 design 4.9 step 4), in the
// caller's transaction, which holds the notebook's row: every live node's
// cleared, then each strict live descendant's of root, a page with content
// its content's hash, an attachment its file's SHA-256. It answers how
// many it set. A head or an attachment's file missing, or a chunk's write
// that changed another number of rows than it holds, is an error, which
// rolls the caller back (M11 design 4.9: not skipped, review B-N6); the
// store's errors are wrapped with %w, so that a deadline stays one.
func RebuildSources(ctx context.Context, s SourceRebuildStore, attachments AttachmentInfos, notebook, root uuid.UUID) (int, error)
```

步骤：

1. `ForgetSources(notebook)`（`sources.sql:59-63`）。
2. `SourceNodes(notebook, root)`：根不是这本笔记本的活页时它报错（`lint_metadata.go:33`），原样交回（批准的第 2 步已核过闸门 2，这里到不了）。
3. 页：`Kind == page && PageBytes > 0` 的 id，按 id 排序，每 `MaxUnitNodes` 个一块 `SourceRebuildHeads`（决定 292，见下）；每个 id 都要在答复里、`Kind` 是页、`Hash` 32 字节、`Bytes > 0`，否则报错（`source metadata missing for node %s` 的写法，同 `unit_sources.go:56`）。
4. 附件：`Kind == asset` 的 id，按 id 排序，每 `MaxUnitNodes` 个一块 `attachments.Of`；每个都要有、`SHA256` 32 字节，否则报错（`source attachment metadata missing for node %s`）。
5. 页在前、附件在后，每 `MaxUnitNodes` 个一块 `SetSourceHashes`；交回的行数不等于这一块的大小就报错（清过之后每一行都从空变为有值，`SetSourceHashes` 只交回真改了的行，`sources.sql:46-54` 的注释）。
6. 交回写下的个数。

每一步 store 与 `attachments` 的错误都以 `%w` 包住再交回（`fmt.Errorf("rebuild the source hashes: %w", err)` 的写法）：批准以 `errors.Is(err, context.DeadlineExceeded)` 认出自己的期限（3.6），以 `%v` 包会把 503 变成 500（设计审查 P3A-N10）。

**读头另写一条按键的语句**（决定 292；设计审查 P3B-M6）：`SourceHeads`（`sources.sql:1-7`，`nodes` 连 `page_contents`、`notebook_id = $1 AND id = ANY($2)`）经 `Planned`，生产里总是自定义计划；这本笔记本约占 `page_contents` 的两成以上、分析过之后，它的自定义计划把"这一本"与"这一万个 id"两个条件的选择度相乘、估这一块只有一两千行，换成 `Hash Join (c.node_id = n.id)`、以 `Seq Scan on page_contents`（并行）作一边：每块读一遍整个实例的 `page_contents`（审查 B 在约 23%、38% 上实测；修订时在 22.4% 上复现）。守卫的种子里这一本约占 12%，看不出。照决定 240 的先例，重建另写一条只给它用的语句，`SourceHeads` 与它现有的两个调用者（`page/app/unit_sources.go:47`、`page/app/source_precheck.go:65`）不动。`page/adapter/postgres/queries/sources.sql` 加：

```sql
-- name: SourceRebuildHeads :many
-- An upgrade's approval's heads (M11/P3 design 3.4, decision 292): each id
-- read by its own key, nodes by nodes_pkey and its content by
-- page_contents_pkey, whatever the statistics; the notebook and the
-- deletion are judged on the row read. A node of another notebook, deleted
-- or missing is not answered.
SELECT n.id, n.kind, c.content_hash, COALESCE(c.byte_size, 0)::bigint AS byte_size
FROM unnest(sqlc.arg(node_ids)::uuid[]) AS i(id)
CROSS JOIN LATERAL (
    SELECT x.id, x.kind, x.notebook_id, x.deleted_at FROM nodes x WHERE x.id = i.id LIMIT 1
) n
LEFT JOIN LATERAL (
    SELECT y.content_hash, y.byte_size FROM page_contents y
    WHERE y.node_id = n.id AND y.deleted_at IS NULL LIMIT 1
) c ON true
WHERE n.notebook_id = sqlc.arg(notebook_id)::uuid AND n.deleted_at IS NULL;
```

外层对 `LATERAL` 子查询别名（`n`）的比较，参数要显式带类型（`::uuid`）：sqlc 从 `n.notebook_id` 推不出参数的类型，不带就报"table alias "n" does not exist"（审查 A 在 worktree 里以 `sqlc generate` 实测；带上之后生成 `NodeIds []uuid.UUID`、`NotebookID uuid.UUID`，行是 `ID`、`Kind`、`ContentHash []byte`、`ByteSize int64`；设计审查 P3A-M6）。同模块 `search.sql:63` 的 `FROM unnest(…) AS i(id)` 写法 sqlc 认得。

store 的 `SourceRebuildHeads(ctx, notebook, ids) (map[uuid.UUID]app.SourceValue, error)`（`source_area.go`）照 `SourceHeads` 的写法：多于 `MaxUnitNodes` 个拒绝，空的一批不发语句，错误以 `%w` 包住（"read the rebuild's source heads: %w"）。不经 `Planned`：它的计划不随统计与参数（下面十二格同形），同 `ForgetSources`、`AttachmentInfos`，同一连接上第六次起可以是通用计划，守卫两种都判（7.1）。

写法是修订时在临时 worktree 里比出来的（审查 B 的探针为底，经 `heavy.sh`）：这本笔记本 60,003 个节点，另九本（这一本占 `nodes` 11.4%）或另四本（22.4%）各 5 万页、各 2,000 个附件与一次升级的基线；三种统计状态（这一本在 `ANALYZE` 之后才写入；全部分析过；从未分析）× 自定义（带参数的 `EXPLAIN`）与通用（`EXPLAIN (GENERIC_PLAN)`），一万个 id：

| 写法 | 计划 |
|---|---|
| `SourceHeads`（今天的） | 22.4%、分析过、自定义：`Hash Join`，以 `Seq Scan on page_contents`（并行）作一边（复现审查 B） |
| `LATERAL`，节点按 `id = i.id AND notebook_id = $1` 读（审查 B 的建议） | "这一本在分析之后才写入"、自定义，两个种子都是：`Limit` 之下 `Index Scan using nodes_notebook_id_name_key_idx cond (notebook_id = …) filter (id = i.id)`，每个 id 沿这本笔记本的索引走到碰上为止，是决定 244 的"内侧逐行重读"；其余各格经 `nodes_notebook_id_id_key` |
| 普通的连接（不用 `LATERAL`） | 自定义计划把这本笔记本的节点整个读进哈希表，每块一次，不比今天的好 |
| **上面的 `SourceRebuildHeads`**（节点只按主键，笔记本与删除在外面判） | 两个种子 × 三种状态 × 两种计划，十二格都是：`Function Scan` → 逐行 `Index Scan using nodes_pkey cond (id = i.id)`（外面 `Subquery Scan` 判 `notebook_id`、`deleted_at`）→ `Memoize` → 逐行 `Index Scan using page_contents_pkey cond (node_id = n.id)` |

两个 `LIMIT 1` 让子查询不被拉平成普通的连接（拉平了就是上表第三行的形状）。修订时只核对了计划，没有计时：它逐行经两个主键，读的行数等于块的大小，与一成的种子上 `SourceHeads` 的计划同阶。只按主键读，另一本笔记本的 id 也读得到它的行，再在外面滤掉；重建交的 id 都来自 `SourceNodes`，到不了，store 的测试仍钉住它不答（见下）。

模块根 `page/source_rebuild.go`（新）：

```go
// SourceRebuild rebuilds a notebook's source hashes as an upgrade's
// approval makes it an LLM Wiki (M11/P3 design 3.4).
type SourceRebuild struct {
	store       *postgresadapter.Store
	attachments AttachmentInfos
}

// NewSourceRebuild returns the rebuild over pool; attachments are the
// attachments' files' metadata, the asset module's, as the units' (Deps.Attachments).
func NewSourceRebuild(pool *pgxpool.Pool, attachments AttachmentInfos) SourceRebuild

// Rebuild is RebuildSources in the transaction ctx carries: none is an error.
func (s SourceRebuild) Rebuild(ctx context.Context, notebook, root uuid.UUID) (int, error)
```

`attachments` 为 nil 照 `page.New` 对没有 `Attachments` 的处理（`page/extension_test.go:936` 测它）。wiki 的端口：

```go
// SourceHashes are the page module's source hashes, rebuilt as an
// approval makes the notebook an LLM Wiki whose source root is rootID, in
// the transaction ctx carries: how many it set (M11 design 4.9 step 4).
type SourceHashes interface {
	Rebuild(ctx context.Context, notebookID, rootID uuid.UUID) (int, error)
}
```

`page.SourceRebuild` 的方法签名与它相同，组合根直接交 `page.NewSourceRebuild(pool, attachmentInfos{asset.NewAttachmentInfos(pool)})`（`attachmentInfos` 是 `bootstrap/assets.go:136` 的那个适配，单元的同一个）。

**`SetSourceHashes` 不另写**（第 285 条，7.6）：设计时在 10 万个来源上量了重建的每一段与几种写法（本机，库里另有九本各 5 万页的 LLM Wiki 与它们的升级、56 万行 `upgrade_nodes`）：

| 段 | 本机（四种统计与计划状态的中位数的范围） |
|---|---|
| `ForgetSources` | 17–286 ms |
| `SourceNodes` | 235–389 ms |
| `SourceHeads`（十块；重建改用 `SourceRebuildHeads`，见上） | 277–393 ms |
| 附件（4,000 个） | 2 ms |
| `SetSourceHashes`（十块，约 9.5 万行） | 1.86–2.58 s |
| `DeleteUpgradeNodes`（11 万行） | 32–40 ms |
| 合计 | 2.44–3.68 s |

`SetSourceHashes` 的另几种写法（`unnest` 连 `page_contents` 取哈希、`= ANY`、`LATERAL`，一块一万或全部一次）都在 2.14–3.14 秒：底在约 9.5 万行的 `UPDATE` 本身（`nodes` 的每个索引都要改，不是 HOT 的更新）。所以写照用已有的语句；读头另写一条是为了读了谁的行（决定 240、292），不是为了时长。

**界与分块**：每块至多 `MaxUnitNodes = 10,000`（三个 store 方法的界，`source_area.go:20-22`、`:76-78` 与新的 `SourceRebuildHeads`）。`SetSourceHashes` 经 `Planned`，生产里总是自定义计划：每一块以这本笔记本的键读一遍、建一次哈希表（分析之后也可能逐行经 `nodes_pkey`，设计时 10 万个来源的第二轮）；通用计划（守卫的 `force_generic_plan`）逐行经 `nodes_notebook_id_id_key`，或哈希建在这一块上、这本笔记本作外侧读一遍（审查 B 以 `EXPLAIN (GENERIC_PLAN)` 量，设计审查 P3B-M1；初稿把自定义计划的形状写成了通用计划的）。自定义计划下块数 × 笔记本的节点数是它的读量：10 万个来源、11 万个节点约 110 万行的读，已含在上面的时长里；30 万个来源时约 30 块 × 33 万，写进 M12 的性能移交（第 9 节）。这不是"内侧逐行重复的扫描"：每块一遍，不随行重复（决定 244 管的是嵌套循环的内侧逐行重读整个集合，7.1 的计划断言区分这两种）。

**测试**：

- `page/app/source_rebuild_test.go`（单元，替身的 store）：调用的次序（清、成员、各块的 head、各块的附件、各块的写）；`MaxUnitNodes + 1` 个页分两块；每一种报错各一格（head 缺、head 的种类是附件、哈希 31 字节、字节数为 0、附件缺、附件的哈希 31 字节、写交回的行数少一个），各在第一处就停、不再写；交回的数；替身的 store 答一个包着 `context.DeadlineExceeded` 的错误时，交回的错误 `errors.Is` 认得出它（`%w`）。
- `page/adapter/postgres` 的 store 测试（真库）：`SourceRebuildHeads` 只答这本笔记本的活节点：另一本笔记本的 id、已删的节点、不存在的 id 都不在答复里；带正文的页答它的 `content_hash` 与 `byte_size`；附件没有正文行，`content_hash` 为空、`byte_size` 为 0；空页（文件夹）有一行空的正文，答空串的 SHA-256（`e3b0c442…`）与 0（每个活页恰有一行正文，`interleavings_page_test.go:92-93`；重建只读 `PageBytes > 0` 的页，用不到它；设计审查 P3A-M8，审查 A 实测）；空的一批不发语句；`MaxUnitNodes + 1` 个拒绝。
- `page/source_rebuild_test.go`（模块根，真库）：`TestTheSourceRebuildIsTheInvariant`：一本普通笔记本，`raw` 下有带正文的页、空页、带正文又有子节点的页、两层深的页、附件、零字节的附件，另有一页经单元删除（删除时哈希已清为空）；`raw` 之外有带正文的页，另以 SQL 给它写上旧的哈希；另一本笔记本的来源带哈希。在一个事务里 `Rebuild`，再以一条 SQL 读这本笔记本全部节点的 `(id, source_hash)` 与"从 `page_contents`、`asset_blobs` 推出的期望"（同不变式，已删的为空）整值比较（每个哈希互不相同）：`raw` 之外那一页清掉了，已删的那一页仍是空、不被写上；交回的数等于期望的非空数；另一本笔记本的哈希不变。不给已删的节点种哈希：这个形状程序造不出来（删除的两条语句都把它置空，`nodes.sql:213`、`:302`），`ForgetSources` 只管活节点（设计审查 P3A-M2 = P3B-M4）。附件的元数据用测试里另写的一个 `AttachmentInfos`，以 SQL 读 `asset_blobs`（page 不能 import asset；`page/extension_test.go:107-112` 的 `noAttachments` 读不到它，设计审查 P3A-N8）。缺附件的文件行（以 SQL 删掉 `asset_blobs` 的那一行）：报错，事务回滚之后什么也没变。不在事务里：报错。

### 3.5 S1：wiki 的行

`wiki/adapter/postgres/queries/upgrades.sql` 加两条：

```sql
-- name: ApproveUpgrade :execrows
-- An upgrade approved (M11 design 4.9 step 6), under the notebook's row lock: approved, who and when; the submission
-- the approval read, in review with its report (notebook_upgrades_approved_check wants the report too).
UPDATE notebook_upgrades
SET state = 'approved', updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND submission_id = sqlc.arg(submission_id) AND state = 'reviewing' AND report IS NOT NULL
    AND deleted_at IS NULL;

-- name: SendBackUpgrade :execrows
-- An upgrade sent back (M11 design 4.8), under the notebook's row lock: upgrading again, its submission and report
-- cleared; from reviewing with a report, or without one whose submission was taken over (submitted at taken_over or
-- before), as SubmitUpgrade judges it.
UPDATE notebook_upgrades
SET state = 'upgrading', submission_id = NULL, submitted_by_id = NULL, submitted_at = NULL, report = NULL,
    updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND state = 'reviewing' AND deleted_at IS NULL
    AND (report IS NOT NULL OR submitted_at <= sqlc.arg(taken_over)::timestamptz);
```

端口（`wiki/app/upgrade_ports.go` 的 `UpgradeRows`）加：

```go
	// Approve sets the upgrade id approved, by by at at: in review, its
	// submission submissionID with its report. SendBack sets it upgrading
	// again, its submission and report cleared: in review with a report,
	// or without one submitted at takenOver or before (M11 design 4.8).
	// False for no row so.
	Approve(ctx context.Context, id, submissionID, by uuid.UUID, at time.Time) (bool, error)
	SendBack(ctx context.Context, id, by uuid.UUID, at, takenOver time.Time) (bool, error)
```

store 照 `Unsubmit`（`store.go:157`）。测试 `wiki/adapter/postgres/review_test.go` 的 `TestTheReviewsRows`（真库，照 `submission_test.go:23` 的夹具），每个合取项与析取项一个反例（13.4 第 1 条）：

- `Approve`：成功一格整行断言（`approved`，提交的三列与报告原样，审计列是批准人与时刻）；别的 `submission_id`、`upgrading`、审阅中没有报告、已软删除，各答假、行不变。
- `SendBack`：有报告一格（报告在、`submitted_at` 是现在）整行断言（`upgrading`，四列为空）；没有报告、`submitted_at = T`：`taken_over = T - 1µs` 答假，`= T` 答真（与 `ReviewOf` 的边界一致）；`upgrading`、`rolling_back`、已软删除各答假。

### 3.6 S1：批准

`wiki/app/approve_upgrade.go`（新）：

```go
// ApproveUpgrade approves a notebook's upgrade in review: POST
// /api/v0/notebooks/{notebook_id}/upgrade/approve (M11 design 4.9, M11/P3
// design 3.6).
type ApproveUpgrade struct{ d UpgradeDeps }

func NewApproveUpgrade(d UpgradeDeps) *ApproveUpgrade

// Execute approves notebook id's upgrade, in the order of M11/P3 design 3.6.
// It answers the upgrade's row as the transaction left it, approved.
func (a *ApproveUpgrade) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error)
```

次序：

1. `shared.RequireActor`；笔记本的工作区，不加锁（`Notebooks.WorkspaceOf`，没有答 404 `notebook.not_found`）。
2. `ctxApprove, cancel := context.WithTimeout(ctx, a.d.ApproveTimeout)`，记下 `started := time.Now()`（同发起，`start_upgrade.go:147`）。一个事务（`a.d.Tx.WithinTx(ctxApprove, …)`）：
   1. `a.d.locked`：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定 `notebook.change_mode`（404、403）→ 读一次时钟 `at`。
   2. 笔记本的升级不是 `reviewing`（含没有升级）：409 `wiki.upgrade_state`。
   3. `Rows.Current`：读不到、id 或状态与笔记本的不同是缺陷（同提交，`submit_upgrade.go:174-180`）。
   4. `ReviewOf(up, at, SubmitTimeout)`：不是 `ReviewReported` 答 409 `wiki.upgrade_state`；报告的 `Passed` 为假是缺陷（2.1 第 5 条）。
   5. 锁下再核闸门 1–3（第 284 条）：`Gates.Roots(ctx, id, up.SourceRootID)`、`Pages.RootNodes(ctx, id, ReservedKeys)`，`JudgeRoots(roots, up.SourceRootID, ids)`；有不过的，回调把不过的键与 `up.ID` 记在外面的变量里（只给日志取字段），交回 `ApprovalNotReady(failed)`（409）。
   6. 要答的行：`view, err := a.d.named(ctx, up)`，在写之前读（`Current` 之后读不到 `approved` 的行，同结束回滚）。
   7. `States.Approve(ctx, id, up.ID, up.SourceRootID, actor.UserID, at)`（3.3）。
   8. `SourceHashes.Rebuild(ctx, id, up.SourceRootID)`，记下写的个数（3.4）。
   9. `Rollback.DeleteBaseline(ctx, up.ID)`（4.9 第 5 步，`page/upgrades.go:116`）。
   10. `Rows.Approve(ctx, up.ID, *up.SubmissionID, actor.UserID, at)`：答假是缺陷。
   11. `States.Publish(ctx, ModeChange{WorkspaceID, NotebookID: id, From: standard, To: llm_wiki, By, At: at})`，在重建之后（审查 B-N3）。
   12. `view.Upgrade.State = approved`。
3. 出错时：`errors.Is(err, context.DeadlineExceeded) && ctxApprove.Err() != nil && ctx.Err() == nil` 是批准自己的期限：记 warn `upgrade approval timed out`（`workspace_id`、`notebook_id`、`user_id`、`elapsed_ms`、`timeout_ms`），答 `shared.ServerBusy(time.Second)`（503、`Retry-After: 1`）；闸门不过（`errors.Is(err, domain.ErrUpgradeNotReady)`，`shared.Error` 按码认，`shared/error.go:215`）：记 error `upgrade gates failed at approval`（`notebook_id`、`upgrade_id`、`failed_gates`，字段取外面的变量），答 `err` 本身（决定 293：这条路只有缺陷才到得了，同检查遇到同一类缺陷时记 error，`check_upgrade.go:229-233`；记在事务回滚之后、答复之前）。不以"外面的变量非空"来认：回调答了 409 而 ROLLBACK 失败时，`WithinTx` 交回的是回滚的失败（`platform/postgres/tx.go:164-172`，它特意不让领域的错误盖住它），照那个错误答，不答成 409（设计审查 P3A-N11）；别的错误照原样。判的写法与发起相同（`start_upgrade.go:167-176`，含"`WithinTx` 交回 nil 时不看 `ctxApprove`"，核对 A3-M1 的那一条）。
4. 提交之后记 info `upgrade approved`（3.14），答 `view`。

**码的次序**：404 `notebook.not_found` → 403 `forbidden` → 409 `wiki.upgrade_state` → 409 `wiki.upgrade_not_ready` → 503 `server_busy`。期限在哪一步到期都答 503，包括等笔记本行的锁时（那时还没判定），同发起（P1 设计 3.15）；契约的说明写明。

**为什么不分批提交、不脱离请求**：模式与哈希要在同一个事务里成立（2.1 第 1 条）。批准不像提交那样有"断开之后还要存下"的需要：断开时整个回滚，升级留在审阅中，再点一次即可；所以用请求的上下文，不脱离。

**测试**（单元，`wiki/app/approve_upgrade_test.go`，替身的端口，照 `submit_upgrade_test.go` 的写法）：调用的次序整列断言（锁、`Current`、`Roots`、`RootNodes`、名字、`Approve`、`Rebuild`、`DeleteBaseline`、`Rows.Approve`、`Publish`）；`Publish` 的值整值断言（`From`、`To`、`By`、`At` 互不相同）；每个码一格且什么也没写；`ReviewChecking`、`ReviewStale` 各一格答 `wiki.upgrade_state`；报告不通过一格答缺陷；三道闸门各一格不过：答 409 `wiki.upgrade_not_ready`，error 一行整值断言（`notebook_id`、`upgrade_id`、`failed_gates` 恰是那一道的键，没有多余的键），没有 info、没有 warn（决定 293）；回调答 409 而替身的事务回滚失败一格：答回滚的那个错误（不是 409），答复照 `WithinTx` 交回的错误（设计审查 P3A-N11）；`Rows.Approve` 答假是缺陷；提交失败一格：替身的事务在回调成功之后提交失败（`submit_upgrade_test.go:59`、`:100-103` 的 `commit` 钩子），答那个错误，没有 info，也没有 warn，调用的次序里 info 在 `commit` 之后（设计审查 P3A-M3 = P3B-M3 第 1 条）；期限：替身的 `Rebuild` 等到 ctx 到期，答 503 与 warn 一行、没有 info；请求的 ctx 先被取消时不记 warn、不答 503。整个程序的在 7.1。

### 3.7 S1：驳回

`wiki/app/reject_upgrade.go`（新）：`RejectUpgrade.Execute(ctx, id) (UpgradeView, error)`，照放弃（`abandon_upgrade.go:47-98`）去掉"继续回滚"一支：

1. actor、`shared.ClientOf`（回滚任务的客户端，`web` 或 `api`）、不加锁读工作区（404）。
2. 一个事务（请求的上下文，同放弃）：`locked`（404、403）；升级不是 `reviewing` 答 409 `wiki.upgrade_state`（含 `upgrading`：驳回只给审阅中的，00 号文档 4.3）；`a.d.rollBack(ctx, actor, client, workspace, id, up.ID, domain.UpgradeRejected, at)`（`abandon_upgrade.go:106-128`，`MaxQueued` 满时 503 `server_busy`）；`view`（`:163-172`）。
3. 提交之后记 info `upgrade rejected`（3.14），答 `view`（`rolling_back`、`ending = rejected`、带任务）。

审阅中没有报告时也可以驳回（00 号文档 4.3 的状态表）：正在算的那次提交的第二个事务看到 `rolling_back`，什么也不写，答 409 `wiki.upgrade_state`（P2 3.12 的 `superseded`）。`notebook_upgrades_rejected_check`（`00057:53`）要驳回的行有提交，审阅中一定有，它是第二道防线。

`rollBack` 的注释改为"The abandonment's and the rejection's"。码：404、403、409 `wiki.upgrade_state`、503 `server_busy`。不声明 `transfer.busy`：审阅中没有写入的任务（冻结，P1），`startBackground` 的写入键答不出它。

单元测试 `reject_upgrade_test.go`：每个码一格；`upgrading` 一格答 `wiki.upgrade_state`、什么也没写；`rollBack` 的调用带 `UpgradeRejected`（替身记下结局，整值断言）；提交失败一格（同 3.6：答那个错误，没有 info，也没有 warn；调用的次序里 info 在 `commit` 之后）。

### 3.8 S1：退回

`wiki/app/send_back_upgrade.go`（新）：`SendBackUpgrade.Execute(ctx, id) (UpgradeView, error)`：

1. actor、不加锁读工作区（404）。
2. 一个事务（请求的上下文）：`locked`（404、403）；`at` 截到微秒（同提交，`submit_upgrade.go:169`：Go 与 SQL 判同一个时刻）；升级不是 `reviewing` 答 409 `wiki.upgrade_state`；`Rows.Current` 与笔记本的不同是缺陷；`ReviewOf`：`ReviewChecking` 答 409 `ErrUpgradeSubmitting`；`States.SetState(…, upgrading, …)`；`Rows.SendBack(ctx, up.ID, actor.UserID, at, at.Add(-SubmitTimeout))`，答假是缺陷；`Publish(From == To == standard)`；之后照放弃与驳回在写之后读 `view`（`a.d.view(ctx, id, up.ID)`，`abandon_upgrade.go:83`、`:163-172`：行是 `upgrading`，`Rows.Current` 读得到，提交与报告已清，不用手工改成 `nil`）。日志要的 `submission_id` 与 `reported` 取写之前读到的 `up`（设计审查 P3A-N6）。
3. 提交之后记 info `upgrade sent back`（3.14），答 `view`（`upgrading`，提交的三列与报告为空）。

码：404、403、409 `wiki.upgrade_state`。退回之后发起人的 agent 又能写（冻结只放过 `upgrading` 中发起人的 MCP 写，P1），UP6 走这一步。

单元测试 `send_back_upgrade_test.go`：每个码一格；三种 `Review` 各一格（`ReviewChecking` 的 `detail` 是"still being checked"）；`SendBack` 收到的 `takenOver` 等于 `at - SubmitTimeout`（整值断言，`at` 带纳秒、断言截过的值）；答复是写之后读的行（提交与报告为空）；info 的 `submission_id`、`reported` 是写之前的；提交失败一格（同 3.6）。

### 3.9 S1：提交的第一个事务改用 `ReviewOf`

保留 `submit_upgrade.go:181` 的外层 `if up.State == domain.UpgradeReviewing {`，只把里面的 `switch`（`:182-190`）换成 `ReviewOf` 的三支（3.2 的表）：`ReviewOf` 对 `upgrading` 答缺陷，整段换掉会让从 `upgrading` 提交答 500（设计审查 P3A-N7）。行为不变：`TestUpgradeSubmit` 的码与接管的各格（`upgrade_submit_test.go:144-178`、`:327-443`）与 `submit_upgrade_test.go` 照旧通过。这是一处只为共用而改的代码，改动由现有的测试守着，不另加格。

### 3.10 S1：第 181 条

`page/domain/contract.go:137`：

```go
	if s.Operation == OpCreate && s.Kind != ChangesetImport || s.Operation == OpRename || s.Operation == OpMove ||
		s.Operation == OpRestore && s.Kind == ChangesetRevert {
```

注释（`:110-123` 的那一段）加一句："A revert's restore is judged as a creation: it brings a node back under its old name (M11 design 4.11); a restore from the trash numbers a reserved key in its unit instead." 撤销里的恢复的 `Before` 为空（`revert_facts.go:54-63`），`!x.Before.atRoot()` 为真，回到根下的保留键就答 `page.virtual_file`。三条路都经 `CheckContract`：同步撤销的预检（`page/app/revert_contract.go:16-41`）、后台撤销开始时的预检（`page/app/revert_job_contract.go:15-47`）、单元里每一步的判定（`source_facts.go:48-49` 取 `Write.Kind`），所以一处改动覆盖三条。回收站的恢复与后台的恢复是 `ChangesetTrashRestore`（`source_restore_precheck.go:52`），不受影响，照旧在单元里加序号。

撤销里的另两种放回（`OpRename`、`OpMove`，`revert_facts.go:35-50`）回到根下的保留键今天就答 `ErrVirtualFile`（`contract.go:137-143`），不用改；但这条路 P3 之前走不到（LLM Wiki 的根下建不出保留的键），升级之后却是最常见的一种：playbook 要 agent 把根下的 `index` 改名（00 号文档 4.13），批准之后撤销那个变更集，放回原名就被拒。契约的句子因此写"restore, rename or move"，整个程序的测试另有一格（7.1 `TestUpgradeTrashReservedTitles`；设计审查 P3A-M5）。

**契约**：`revertChangeset`（`api/modules/page.yaml:1091-1107`）与 `startRevert`（`api/modules/transfer.yaml:283-301`）的 `x-problem-codes` 在 `page.protected` 之后加 `page.virtual_file`；说明各加一句。`revertChangeset` 在"…or rename or move AGENTS is page.protected;"之后加：

```
one that would restore, rename or move a node back to the notebook's root under a title the LLM Wiki reserves for its generated pages (index or log, index.md or log.md) is page.virtual_file;
```

`startRevert` 的"(page.protected, page.source_immutable, page.source_admin_only, or page.source_too_large)"改为"(page.protected, page.virtual_file, page.source_immutable, page.source_admin_only, or page.source_too_large)"。

**测试**：

- `page/domain/contract_test.go`：`OpRestore`、`ChangesetRevert`、`After` 在根下、键 `index` 答 `ErrVirtualFile`；同样而键是 `log.md` 一格（`Reserved` 的另一种形状）；同样而不在根下放过；同样而 `ChangesetTrashRestore` 放过；同样而笔记本是普通的放过；`OpRestore`、`ChangesetRevert` 回到根下一个不保留的键放过。每格一个反例只钉一种形状（05-lessons）。
- 模块的 `adapter/http`：`page/adapter/http/history_test.go:182` 的表加 `domain.Refuse(domain.ErrVirtualFile, id(13))` 答 409 `page.virtual_file`；`transfer/adapter/http/start_background_test.go:112` 的表加 `refusing("page.virtual_file")` 答 409（13.1 第 8 条）。
- 整个程序 `TestUpgradeTrashReservedTitles`（7.1）。

### 3.11 S1：契约

`api/modules/wiki.yaml`，接在 `submitUpgrade`（`:305-358`）之后：

```yaml
  /api/v0/notebooks/{notebook_id}/upgrade/approve:
    parameters:
      - $ref: "#/components/parameters/NotebookID"
    post:
      operationId: approveUpgrade
      tags: [wiki]
      summary: >-
        Approves the upgrade in review: the notebook becomes an LLM Wiki
        whose source root is the upgrade's, its content no longer frozen.
      description: >-
        An upgrade in review whose submission kept its report is approved: in
        one transaction the notebook becomes an LLM Wiki, its sources' hashes
        are rebuilt and the upgrade's baseline is dropped. There is no undoing
        it as a whole: each of its changesets can still be reverted as an LLM
        Wiki's rules allow; downgrading makes it a standard notebook again.
        Its admins can (forbidden). A notebook that does not exist, is
        deleted, or that the caller cannot see is notebook.not_found; one
        without an upgrade in review, or whose submission is still checked or
        never finished, is wiki.upgrade_state; one whose AGENTS page, source
        root or root titles no longer pass is wiki.upgrade_not_ready, its
        detail naming the gates, the upgrade still in review. These checks
        occur in that order. An approval that does not finish within its time,
        at any step, waiting for the notebook too, is server_busy with
        Retry-After: 1, with nothing written: rebuilding the hashes of tens of
        thousands of sources takes seconds.
      security: [{ bearer: [] }]
      x-problem-codes: [notebook.not_found, forbidden, wiki.upgrade_state, wiki.upgrade_not_ready, server_busy]
      responses:
        "200":
          description: The upgrade, approved.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Upgrade"
        "503":
          $ref: "#/components/responses/Problem"
        default:
          $ref: "#/components/responses/Problem"
  /api/v0/notebooks/{notebook_id}/upgrade/reject:
    parameters:
      - $ref: "#/components/parameters/NotebookID"
    post:
      operationId: rejectUpgrade
      tags: [wiki]
      summary: >-
        Rejects the upgrade in review: puts the notebook back as the
        upgrade started, in a background job.
      description: >-
        An upgrade in review, with its report or not, rolls back,
        rejected, its content frozen but for the rollback's job; a
        submission still checked then stores nothing. Its admins can
        (forbidden). A notebook that does not exist, is deleted, or that
        the caller cannot see is notebook.not_found; one without an
        upgrade in review is wiki.upgrade_state; a full job queue is
        server_busy. These checks occur in that order. The answer is the
        upgrade, rolling back, with its rollback's job.
      security: [{ bearer: [] }]
      x-problem-codes: [notebook.not_found, forbidden, wiki.upgrade_state, server_busy]
      responses:
        "200":
          description: The upgrade, rolling back.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Upgrade"
        "503":
          $ref: "#/components/responses/Problem"
        default:
          $ref: "#/components/responses/Problem"
  /api/v0/notebooks/{notebook_id}/upgrade/send-back:
    parameters:
      - $ref: "#/components/parameters/NotebookID"
    post:
      operationId: sendBackUpgrade
      tags: [wiki]
      summary: >-
        Sends the upgrade in review back: upgrading again, its submission
        and report cleared, its initiator's agent able to write again.
      description: >-
        An upgrade in review that has its report, or has none and whose
        submission may be taken over, goes back to upgrading, its submission
        and report cleared: a submission still checked is wiki.upgrade_state,
        its detail saying so; send it back a little later. Its admins can
        (forbidden). A notebook that does not exist, is deleted, or that the
        caller cannot see is notebook.not_found; one without an upgrade in
        review is wiki.upgrade_state. These checks occur in that order.
      security: [{ bearer: [] }]
      x-problem-codes: [notebook.not_found, forbidden, wiki.upgrade_state]
      responses:
        "200":
          description: The upgrade, upgrading.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Upgrade"
        default:
          $ref: "#/components/responses/Problem"
```

`Upgrade` 与它的枚举不改（`approved`、`rejected` 已有）。`submitUpgrade` 的说明（`wiki.yaml:328-329`）"a submission taken over or an upgrade abandoned meanwhile"改为"a submission taken over, or an upgrade sent back, rejected or abandoned meanwhile"：P3 之后提交算完时多了两种读到别的状态的原因（设计审查 P3C-N4）。`make gen` 重新生成 `api/dist/openapi.yaml`、wiki 的 `adapter/http/gen` 与 `web/packages/api-client/src/schema.gen.ts`。

处理函数（`wiki/adapter/http/upgrade.go`）照 `AbandonUpgrade`（`:43-53`）各一个，答复经 `upgradeOf`（`:83-130`）。`UpgradeUseCases`（`handler.go:22-28`）加 `ApproveUpgrade`、`RejectUpgrade`、`SendBackUpgrade`。`TestTheUpgradesCodesAreAnswered`（`wiki/adapter/http/upgrade_test.go:187`）的表加三个操作的每个声明的码（approve 五个、reject 四个、send-back 三个），替身 `upgradeFake`（`:25`）加三个方法；`TestTheUpgradeIsAnswered`（`:139`）加三个操作各一格答 200 的投影（`approved`、`rolling_back` 带 `ending: rejected`、`upgrading` 没有提交），夹具的每个值互不相同、整值断言；`:138-143` 的"不收请求体"一格加三个路径。

### 3.12 S1：组合根

- `wiki/lint.go` 的 `Deps` 加：

  ```go
  	// An upgrade's approval (M11/P3 design 3.6): the page module's source
  	// hashes' rebuild, and its transaction's deadline.
  	SourceHashes   SourceHashes
  	ApproveTimeout time.Duration
  ```
  `New` 把它们交进 `UpgradeDeps`，`Module` 加 `approve`、`reject`、`sendBack` 与三个方法（`ApproveUpgrade`、`RejectUpgrade`、`SendBackUpgrade`，签名同 `AbandonUpgrade`）；`Deps` 的注释加一句。模块根的类型别名加 `SourceHashes`。
- `bootstrap/wire.go:255-296` 的 `wikiDeps` 加 `SourceHashes: page.NewSourceRebuild(pool, attachmentInfos{asset.NewAttachmentInfos(pool)})` 与 `ApproveTimeout: cfg.Server.RequestTimeout * 3 / 4`（注释照 `StartTimeout` 的那段，第 279 条）。`wiring` 不加钩子：测试经已有的 `wiring.wiki` 换 `SourceHashes` 与 `ApproveTimeout`（7.1 的期限与空的重建两格）。
- `bootstrap/wiki_upgrade.go` 的 `wikiUpgradeStates` 加 `Approve`，转调 `notebook.UpgradeStates.Approve`。
- HTTP 的路由由 wiki 的 `Register` 照生成的代码挂上，不另改组合根。`archtest` 不改：wiki 不 import page，组合根交的是 page 模块根的值。

### 3.13 S1：矩阵的行

`TestUpgradePermissionMatrix`（`bootstrap/upgrade_permission_matrix_test.go:306`，第七份）的 `upgradeMatrixRows()`（`:51-106`）加三行；变体照 `upgradedBy(state)`（`:28-40`），写的每格在库的一份副本上（`:320`）：

| 操作 | 变体 | 管理员 | 看得到的非管理员 | 看不到的 | `left`（写之后的行） |
|---|---|---|---|---|---|
| `approveUpgrade` | `reviewing`（没有报告、刚提交） | 409 `wiki.upgrade_state` | 403 | 404 | 管理员一格之后仍是 `reviewing reviewing`、模式 `standard` |
| `rejectUpgrade` | `reviewing` | 200，在 `check` 里核答复：`rolling_back`、`ending: rejected`、带任务（同 `abandonUpgrade` 的 `rollbackAnswer`，`:217-233`） | 403 | 404 | 被拒的各格：行不变（`reviewing reviewing`，提交还在）。管理员一格只断言 `ending = rejected`、状态是 `rolling_back` 或 `rolled_back`、提交留着：每格起一个完整的应用，回滚任务照常跑，`upgradeIn` 的基线就是现状，差是空的，第一遍就可能做完（`:323`、`:341-343`；同 P2 交错第 7 条，`upgrade_submit_test.go:615-620`；设计审查 P3A-M1） |
| `sendBackUpgrade` | 新的"reviewing, taken over"：`upgradedBy(reviewing)` 之后 `UPDATE notebook_upgrades SET submitted_at = submitted_at - interval '1 hour'` | 200 | 403 | 404 | 管理员：`upgrading upgrading`，提交的三列为空 |

（列的写法照现有的行：`adminsOnly(cell, notFound)`。）管理员批准答 409 的理由在 2.1 第 7 条。`coveredMatrixRows()`（`permission_matrix_coverage_test.go:159-165`）读 `upgradeMatrixRows()`，三个新操作因此被 `TestThePermissionMatrixCoversEveryOperation`（第三份）要求有行；漏一行时它失败。

### 3.14 S1：日志

13.1 第 10 条：只记事实。每行在事务提交之后、答复之前记；被拒、回滚的不记 info。批准时闸门不过的 error 一行（决定 293）在事务回滚之后、答复之前记。

| 消息 | 级别 | 字段 |
|---|---|---|
| `upgrade approved` | info | `workspace_id`、`notebook_id`、`upgrade_id`、`submission_id`、`user_id`、`sources_hashed`、`elapsed_ms` |
| `upgrade approval timed out` | warn | `workspace_id`、`notebook_id`、`user_id`、`elapsed_ms`、`timeout_ms` |
| `upgrade gates failed at approval` | error | `notebook_id`、`upgrade_id`、`failed_gates`（不过的闸门的键，同 `upgrade submitted` 一行的字段名；决定 293：这条路只有缺陷才到得了，同检查的 `upgrade source root is no live page`，`check_upgrade.go:232`） |
| `upgrade rejected` | info | `workspace_id`、`notebook_id`、`upgrade_id`、`submission_id`、`user_id`、`job_id` |
| `upgrade sent back` | info | `workspace_id`、`notebook_id`、`upgrade_id`、`submission_id`（退回的那一次）、`user_id`、`reported`（有没有报告） |

每行一个整个程序的测试格，只读自己的子测试开始之后的行（05-lessons 第 1 节），整行断言字段的集合（没有多余的键，没有标题、路径）；error 一行在 `TestUpgradeApproval` 第 4 格。

### 3.15 S2 的文件

| 文件 | 改动 |
|---|---|
| `wiki/domain/playbook-upgrade.md`（新）、`wiki/domain/playbook.go` | playbook 与 `UpgradePlaybookText`（3.17） |
| `wiki/app/upgrade_playbook.go`（新） | `UpgradePlaybook`（3.16） |
| `wiki/adapter/mcp/prompts.go` | `Prompts(p, u)` 加 `upgrade` |
| `wiki/prompts.go`、`wiki/lint.go` | `Module` 加 `upgradePlaybook`，`Prompts()` 交它 |
| `api/mcp/prompts.json` | 笔记本一项加 `upgrade` |
| `bootstrap/permission_matrix_mcp_wiki_test.go`、`mcp_prompts_test.go`、`upgrade_prompt_test.go`（新） | 3.18 |
| `README.md` | "升级为 LLM Wiki"一节，连同 `:214`、`:234`、`:236`、`:246` 与 MCP 一节的两个工具（3.19） |
| `api/mcp/tools.json` | `upgrade_submit` 的说明加"or sends it back"（两个入口各一项，`:589`、`:1298`；设计审查 P3C-N4） |
| `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md` | K（3.20） |
| `e2e/stories/upgrade/up9-playbook.spec.ts`（新）、`e2e/fixtures/mcp.ts` | UP9（第 5 节） |
| `e2e/stories/wiki/lw11-playbook.spec.ts` | 标题与 `:10`、`:62` 两处注释的"three"（第 5 节末；设计审查 P3C-N5） |

### 3.16 S2：prompt `upgrade` 与它的用例

`wiki/app/upgrade_playbook.go`（新，第 286 条）：

```go
// UpgradePlaybook is the upgrade prompt's text (M11 design 4.13): for a
// notebook upgrading alone, which the module's row tells.
type UpgradePlaybook struct {
	notebooks Notebooks
	auth      shared.Authorizer
	rows      UpgradeRows
}

func NewUpgradePlaybook(notebooks Notebooks, auth shared.Authorizer, rows UpgradeRows) *UpgradePlaybook

// Execute answers the upgrade playbook for notebook: the actor; lint.read
// decided as the other prompts decide it (notebook.not_found for a
// notebook the caller cannot see, or none); the upgrade under way, read
// unlocked; wiki.upgrade_state unless it is upgrading.
func (p *UpgradePlaybook) Execute(ctx context.Context, notebook uuid.UUID) (string, error)
```

次序照 `Playbooks.Execute`（`playbooks.go:28-46`）：`RequireActor` → `authorizeLintRead`（`app/lint.go:137-150`：`WorkspaceOf`、`Authorize(lint.read)`、`ErrNotVisible` 答 `notebook.not_found`）→ `rows.Current(ctx, notebook)`：没有、或状态不是 `upgrading` 答 `ErrUpgradeState`（普通的 `detail`）→ `domain.UpgradePlaybookText()`。不读笔记本的模式：LLM Wiki 没有进行中的升级，答 `wiki.upgrade_state`，同普通笔记本。

`wiki/domain/playbook.go`：

```go
//go:embed playbook-upgrade.md
var upgradePlaybook string

// UpgradePlaybookText is the upgrade playbook, trimmed (M11 design 4.13).
// It is not a Playbook: PlaybookText answers an LLM Wiki's three alone.
func UpgradePlaybookText() string { return strings.TrimSpace(upgradePlaybook) }
```

`wiki/adapter/mcp/prompts.go`：

```go
// UpgradePlaybookUseCase is the upgrade prompt's text.
type UpgradePlaybookUseCase interface {
	Execute(ctx context.Context, notebook uuid.UUID) (string, error)
}

	upgradePlaybookDescription = `Organize this standard notebook into an LLM Wiki with the upgrade tools until every check passes, then submit it for an admin's review. Upgrading notebooks only.`

// Prompts … The upgrade's prompt is listed for every notebook too; a get
// answers wiki.upgrade_state unless the notebook is upgrading.
func Prompts(p PlaybooksUseCase, u UpgradePlaybookUseCase) []mcpserver.Prompt
```

`upgrade` 一项：`Name: "upgrade"`、`Title: "Upgrade to an LLM Wiki"`、上面的说明、`Entry: mcpserver.NotebookEntry`、没有参数，`Get` 交 `u.Execute(ctx, c.Notebook)` 的文字作唯一的消息、说明作 `description`。排在三个之后（`prompts.json` 按名字排）。`api/mcp/prompts.json` 的 `notebook` 一项加：

```json
    {
      "name": "upgrade",
      "title": "Upgrade to an LLM Wiki",
      "description": "Organize this standard notebook into an LLM Wiki with the upgrade tools until every check passes, then submit it for an admin's review. Upgrading notebooks only."
    }
```

单元测试：`wiki/app/upgrade_playbook_test.go`：调用的次序（"workspace""authorize lint.read""current"）；看不到答 `notebook.not_found`；没有升级、`reviewing`、`rolling_back` 各答 `wiki.upgrade_state`；`upgrading` 答文字。`wiki/adapter/mcp/prompts_test.go`：四个 prompt 的名字与次序；`upgrade` 的 `Get` 把笔记本交给用例、错误原样交回（`:67-70` 的拒绝一格改按名字取，不按下标）。`wiki/domain/playbook_test.go`（`playbooks_test.go:129-152` 那几格）：`UpgradePlaybookText` 的第一行、每段要有的句子（3.17 的"必有的句子"）、不过 `MaxPlaybookBytes`、已去掉首尾空白。

### 3.17 S2：playbook 的正文

`wiki/domain/playbook-upgrade.md`，英文（第 176 条），照三份已有的 playbook 的写法：第一行一句祈使的标题，之后平铺的段落，没有小标题，直呼工具与码的名字。初稿（去掉首尾空白 4,827 字节，离 16 KiB 的界很远；设计审查 P3C-N3 改正初稿写的"约 5.6 KB"）：

```
Upgrade this notebook to an LLM Wiki.

Read and follow AGENTS.md and this notebook's instructions. The notebook is standard while it upgrades, and frozen: only the agent of the admin who started the upgrade, through MCP, may write, and only while the upgrade is upgrading. notebook.upgrading means you are not that agent, the upgrade is no longer upgrading, or your user is no longer an admin of this notebook: stop and tell your user. The upgrade proposed a source root, raw at the notebook's root, and AGENTS at the root; it may have written them.

Start with upgrade_verify without a rule. It names eight gates and eight rules: AGENTS has a body; the source root is a live page and not AGENTS; no node at the root is titled index, log, index.md or log.md; every page outside the source root has valid frontmatter with type and summary; every sources reference is a link to a page or attachment under the source root; no original page links to more missing pages than when the upgrade started; every original page and node is still there, its body unchanged; and the link index is up to date. Read one rule's items with upgrade_verify and that rule, and continue next_cursor until it is null. Call upgrade_verify again after each round of changes.

Sort the original pages: a source is evidence you keep as it is; a wiki page is knowledge you maintain. Move each source, with the pages under it, under the source root. Pages already under the source root that are not sources: move them out of it; after the approval, everything under it is a source, read-only. Give each wiki page, folder pages too, frontmatter with type and summary; a page that rests on sources also lists them in sources, quoted links to their paths under the source root, for example sources: ["[[raw/Note.md]]"]. You may change an original page's frontmatter only: its body after the frontmatter, and after a byte order mark, must stay byte for byte, or the write is page.upgrade_immutable. A page that starts with --- and has no closing --- line has no frontmatter: add a valid block before it. A page whose block between two --- lines is not valid YAML: rewrite that block in place, for example keeping its old text in a quoted property, or move the page under the source root, where frontmatter is not checked; a new block before it would make the old one part of the body.

Keep an existing AGENTS page whose body is not empty, or rename it and write a new AGENTS; rename any node at the root titled index, log, index.md or log.md. Never delete an original node or the proposed source root: that is page.upgrade_protected. Pages you created during the upgrade may be deleted.

Moves and renames must let the system rewrite the links that lead to what moved: never pass update_links: false, or the change is page.upgrade_keep_links. A link by name alone resolves after a move and is not rewritten; a link that names a path is rewritten when its target moves, and a rename rewrites both. A new page's title becomes the target of existing short links that end with it: choose new titles knowing that, and read the ambiguous rule's items after creating pages.

In a batch, once a move or a rename has rewritten an original page's links, or would by the links the page had before the batch, writing that page later in the same batch is page.upgrade_batch_rewrite, whatever you write: write the page first and move afterwards, or use two batches. A page changed and changed back within one batch passes, unless a move in between rewrote it or reached it. Splitting the batch does not make a body change acceptable: it is still page.upgrade_immutable. Large original pages that a batch writes whole count toward its read bound; page.batch_too_large means split the batch.

When upgrade_verify reports every gate passed, call upgrade_submit. It checks again and puts the upgrade in review, where nothing can be written. wiki.upgrade_not_ready names the gates that did not pass: the upgrade is upgrading again, so verify, fix and submit again. wiki.upgrade_stale_index means the link index is out of date: tell your user that a server admin runs nervewiki reindex; once upgrade_verify passes the index gate, submit again. wiki.upgrade_state saying the previous submission is still being checked means wait a moment and call upgrade_verify before submitting again; wiki.upgrade_state otherwise means the upgrade is already in review or no longer under way: tell your user. server_busy means wait retry_after_seconds, then submit again.

Once submitted, an admin reviews the upgrade in the web app, then approves it, sends it back for more work, or rejects it, which puts the notebook back as it was. Report what you changed, which gates passed, and what remains for your user to decide. Never claim a gate passed that upgrade_verify did not report as passed.
```

**必有的句子**（单元测试逐句断言，一句一格，防止改写时漏掉一类）：第一行；"only the agent of the admin who started the upgrade"；`notebook.upgrading`；八道闸门的那一段里每道闸门的说法；`next_cursor`；`page.upgrade_immutable`、`page.upgrade_protected`、`page.upgrade_keep_links`、`page.upgrade_batch_rewrite`、`page.batch_too_large`；"has no closing --- line"与"is not valid YAML"两种 frontmatter；"A new page's title becomes the target"；`wiki.upgrade_not_ready`、`wiki.upgrade_stale_index`、"still being checked"、`retry_after_seconds`；最后一句；设计审查 P3C-M6 的三句："is a link to a page or attachment under the source root"、"tell your user that a server admin runs nervewiki reindex"、"Pages already under the source root that are not sources"（决定 214）。设计审查之后改了六处（P3C-M6 的三处、P3C-N18 的三处：每一轮改动之后再验、建页之后读 `ambiguous` 的条目、只留正文非空的原有 `AGENTS`，空正文的改不了正文，只能改名另建，第 172 条）。逐项对着 P2 第 9 节"P3/S2"的四条与 00 号文档 4.13 的清单核对过：护栏的三个码、无效 frontmatter 的两种改法、改了又改回、拆的写法、拆开之后仍是 `immutable`、只写名字的链接、盲写计进读的界、`retry_after_seconds`、"正在计算"时等一会再检查、已在审阅中就告诉用户。

### 3.18 S2：prompt 的测试改写

- **`TestThePromptReadsLLMWikiMatrix`**（`bootstrap/permission_matrix_mcp_wiki_test.go:153-208`，第三份）：循环改为只取 `ingest`、`query`、`lint` 三个（按名字列出，不是 `prompts.json` 的每一项）；`:177` 的"没有行就失败"随之只对这三个；`:205-206` 的断言改为"`prompts.json` 里恰有这三个与 `upgrade`，`checked == 2 * 13 * 3`"，加一个 prompt 而没有在这里或 `TestUpgradePromptMatrix` 里有期望时失败。
- **`TestThePromptListsAreTheContract`**（`bootstrap/mcp_prompts_test.go:101`，第三份）：`:105-106` 的 `!= 3` 改为 `!= 4`，注释（`:97-100`）"lists the three"改为"lists the four"。
- **`TestTheWikiPromptsAreReached`**（`:145-259`）：注释与表（`:140-144`、`:167-173`）照旧是三个 LLM Wiki 的 prompt；`upgrade` 的到达在新的测试里。
- **`TestUpgradePromptMatrix`**（新，`bootstrap/upgrade_prompt_test.go`，第七份，第 222 条）：四个 prompt × 四种状态（`llm_wiki`；没有升级的 `standard`；`upgrading`；`reviewing`）× 13 个笔记本列。期望表：`ingest`、`query`、`lint` 在 `llm_wiki` 照 `promptCellOf`（`permission_matrix_mcp_wiki_test.go:131-147`）答文字，在后三种答 `notebook.not_llm_wiki`；`upgrade` 只在 `upgrading` 答文字，别的三种答 `wiki.upgrade_state`；看不到的列在 HTTP 的边上答 404。升级的两种状态在升级的变体上（`TestUpgradePermissionMatrix` 的同一组建法）：每种升级状态一份库的副本（`upgradedBy` 的建法），矩阵的几本笔记本在副本上以 `upgradeIn` 设好，4 个 prompt × 13 列在同一份上读，不为每格复制（prompt 只读；设计审查 P3C-N10）。计数断言 `checked == 4 * 4 * 13`。变异"prompt 在审阅中也答"由 `reviewing` 一格抓住（00 号文档 4.13）。
- **`TestUpgradePrompt`**（新，同一个文件）：经真的笔记本入口：`upgrading` 的笔记本取 `upgrade`，一条 `user` 的消息、文字等于 `UpgradePlaybookText()`、`description` 等于 `prompts.json` 的；读者（能读、不是管理员）也取得到（`lint.read`）；没有升级的普通笔记本、审阅中、回滚中、LLM Wiki 各答 `-32602`、`data` 是 `wiki.upgrade_state`；`prompts/list` 在 LLM Wiki 与普通笔记本里都列出它；取用的日志一行（`prompt` 是 `upgrade`，不记参数，照 `:240-258`）。

### 3.19 S2：README 的"升级为 LLM Wiki"一节

`README.md` 的"### LLM Wiki 与 playbook"（`:230-246`）之后、"## 接入 agent（MCP）"（`:248`）之前加一节：

```markdown
### 升级为 LLM Wiki

一本普通笔记本可以升级为 LLM Wiki：笔记本的管理员在"设置 → 模式"里点"升级为 LLM Wiki…"。升级中笔记本对所有人只读，只有发起人自己的 agent 经 MCP 能写（提交审阅之前）；冻结没有时限，任何笔记本管理员都可以随时放弃（审阅中网页上由"驳回"代替，效果相同）。REST 的路径都在 `/api/v0/notebooks/{notebook_id}/upgrade` 之下：`POST` 它发起、`GET` 它读，另有 `check`、`submit`、`approve`、`send-back`、`reject`、`abandon`、`end-rollback`（`api/modules/wiki.yaml`）。

1. **发起**：根下有标题为 `raw` 的页时，它成为来源区（批准之后它下面的都是只读的来源）；没有就补写一页 `raw`。根下没有 `AGENTS` 时补写一页（取界面语言的模板）。原有页的正文不改。
2. **接 agent**：照下面"接入 agent（MCP）"一节，用发起人自己的访问令牌，把 agent 连到这本笔记本的入口 `/mcp?notebook=<id>`（id 在网页地址 `/notebooks/` 之后），运行 prompt `upgrade`。agent 照它把来源移进 `raw`、给其余的页补上 frontmatter（`type`、`summary`、`sources`），反复用 `upgrade_verify` 检查。升级中原有的页只能改 frontmatter、不能删，移动与改名必须改写链接；被拒时工具的错误说明怎么改。
3. **提交**：八道闸门都过了，agent 调 `upgrade_submit`，或在升级页上点"提交审阅…"。服务端再检查一次，通过就进入审阅，存下这次的报告；不过就回到升级中。
4. **审阅**：任何笔记本管理员在升级页上看报告，然后"批准"（笔记本成为 LLM Wiki，来源区只读、`AGENTS` 与来源根受保护）、"退回继续修改"（回到升级中，agent 可以再写）或"驳回"（在后台撤销这次升级的全部改动）。批准之后没有整体的撤销：升级的变更集仍可在历史里照 LLM Wiki 的规则逐个撤销；要回到普通笔记本用降级。
5. **放弃与回滚**：放弃与驳回都在后台按发起时记下的样子把笔记本放回去；回滚停下时可以继续，或者"结束回滚，保留现在的样子"。

批准时服务端为来源区里每一页正文非空的页与每个附件算一次来源的哈希，数万个来源要几秒；批准的期限是 `server.request_timeout` 的四分之三，太短时答 503 `server_busy`、什么也不写（`server/configs/config.yaml` 的 15 秒在十万个来源上有余）。

**已有的笔记库**：要把一份按 LLM Wiki 整理过的库（例如从另一个实例导出的 LLM Wiki）搬进来，先导入一本新建的普通笔记本（根下正好是 `AGENTS` 与 `raw`，导入不加序号；根下已有 `raw`、`AGENTS` 的笔记本照样加序号），再升级：发起认出根下的 `raw` 作来源区、留着原有的 `AGENTS`，agent 只需补齐 frontmatter。根下的 `index`、`log`、`index.md`、`log.md` 要由 agent 改名，闸门才会过。
```

同一个提交一并改 README 里 S2 让它不实的几处（设计审查 P3C-M7）：

- `:234`"`prompts/list` 提供英文的 `ingest`、`query`、`lint`"改为"提供英文的 `ingest`、`query`、`lint` 与 `upgrade`（`upgrade` 只在升级中取得到，见'升级为 LLM Wiki'）"；
- `:236`"普通笔记本也列出同样三个名字，取用答 JSON-RPC -32602，码是 `notebook.not_llm_wiki`"改为"普通笔记本也列出同样四个名字，取用前三个答 JSON-RPC -32602、码是 `notebook.not_llm_wiki`；`upgrade` 不在升级中时同样答 -32602、码是 `wiki.upgrade_state`"；
- `:214` 的内置说明的字节数照 S2 实测时 `TestTheBuiltInInstructionsAreShort -v` 的数写（审查 C 在 `955fa579` 上量到 notebook 3225/3276、account 3294/3346，距 4 KiB 余 820、750；S2 不改内置说明，它不列 prompt），"保留M11至少300B余量"改为写实测的余量；
- MCP 一节的工具（`:219` 一条之后）加一句："升级中的笔记本入口另有 `upgrade_verify`（只读，检查闸门与规则）与 `upgrade_submit`（提交审阅），见'升级为 LLM Wiki'"；`:299` 的工具数（"笔记本入口有 14 个工具（账户入口 16 个）"）与 `:319` 的注解列表 S2 照 `api/mcp/tools.json` 一并核对（M10、M11 都加过工具）；
- `:246`"**重新导入的限制**"一段末尾加"要这样的库成为 LLM Wiki：导进新的普通笔记本再升级，见下一节"，新的一节不再重复 `AGENTS 2`、`raw 2` 那一句。

文字照 00 号文档 4.4、4.8–4.10、4.19 与负责人的四个回答写；"导入不加序号"一句照 `bootstrap/virtual_export_import_test.go:33-36` 核对过。S2 的文档提交另核对 `README.md:157`、`:169-172`、`:481-525` 的"升级"指的是服务端的升级（第 4.13 条的"server update"只改了工具的措辞，README 的中文不歧义），不改。

### 3.20 S2：MCP 人工清单的 K

`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`：J 一节（`:288-308`）之后、"## 做完"（`:310`）之前加：

```markdown
### K：升级（M11）

这一节由 M11/P3 加。每步只记客户端实际做了什么、显示了什么，不评判 agent 整理得好不好。

准备：在网页上以一位管理员在一本普通笔记本上发起升级（设置 → 模式 → 升级为 LLM Wiki），笔记本里留一页正文非空的原有页。接入用的 PAT 必须是发起人的，否则所有写都被冻结（`notebook.upgrading`）。另备一本没有升级的普通笔记本。

| 步 | 做什么 | 看什么 |
|---|---|---|
| K1 | 在升级中的笔记本接笔记本入口 | prompt 列表里 `upgrade` 怎样列出（名字、标题、说明） |
| K2 | 取 `upgrade` | 文字是否完整显示；客户端是否把它当作一条用户消息 |
| K3 | 让客户端照它调 `upgrade_verify`，并让它改一页原有页的正文（在提交之前） | `upgrade_verify` 的审批怎样问（只读）；`page.upgrade_immutable` 在客户端里怎样显示，agent 是否读懂、改为只改 frontmatter |
| K4 | 让客户端调 `upgrade_submit` | 审批怎样问（`upgrade_submit` 不是毁坏性的：`destructiveHint: false` 是否让客户端少问一次）；笔记本入口的 `upgrade_submit` 的 `inputSchema` 没有 `properties`，客户端是否接受；提交通过之后再写答 `notebook.upgrading` 怎样显示 |
| K5 | 在没有升级的普通笔记本上取 `upgrade` | `wiki.upgrade_state` 怎样显示 |
```

同时把 `:308` 的"…M11 的 upgrade prompt 不在这一节。"改为"…M11 的 upgrade prompt 在 K。"，`:323`"M11 的 upgrade prompt 由 M11 登记。"改为"M11 的 upgrade prompt 在 K（M11/P3 加）。"；头（`:8`）把 J 的结果交 M10 收尾审查的那一句之后加"K 的结果交 M11 的收尾审查"。K3、K4 落实 P2 第 9 节"P3/S2"的第三条与审查 C 的未核实的怀疑。改正文在提交之前：提交通过之后升级进入审阅、对所有人冻结，再改答的是 `notebook.upgrading`，看不到 `page.upgrade_immutable`（设计审查 P3C-M9；00 号文档 4.20 照改，2.1 第 11 条）。K 与 J 一起在 v0.1 的最后执行（00 号文档 4.20）。

### 3.21 每个操作改后的码

| 操作 | `x-problem-codes`（次序即判定的次序） | Step |
|---|---|---|
| `approveUpgrade`（新） | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`wiki.upgrade_not_ready`、`server_busy` | S1 |
| `rejectUpgrade`（新） | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`server_busy` | S1 |
| `sendBackUpgrade`（新） | `notebook.not_found`、`forbidden`、`wiki.upgrade_state` | S1 |
| `revertChangeset` | 现有的十四个，`page.protected` 之后加 `page.virtual_file` | S1 |
| `startRevert` | 现有的，`page.protected` 之后加 `page.virtual_file` | S1 |
| prompt `upgrade`（新，MCP） | `notebook.not_found`、`wiki.upgrade_state`（没有表，`TestUpgradePrompt` 与矩阵逐个答出） | S2 |

MCP 的工具不改，`mcpToolCodes()`、`upgradeToolCodes()`、`mcpOwnCodes()` 都不改（P3 没有新的工具码；`page.virtual_file` 加在两个 REST 的撤销上，MCP 没有撤销的工具）。三个新操作的码都已有两种语言的文字（`W/app/problem-messages.ts:42-50`），`problem-messages.test.ts` 读契约照样通过。

### 3.22 每个新检查的反向对照

| 检查 | 反向对照（把它去掉或放宽时哪个测试失败） |
|---|---|
| 批准只给审阅中、有报告的 | `TestUpgradeApproval` 的"没有报告、刚提交""没有报告、已过期限""upgrading"三格答 409；单元的 `ReviewChecking`、`ReviewStale` 两格；SQL 的 `report IS NOT NULL` 由 `TestTheReviewsRows` 的反例 |
| 批准的 SQL 核 `submission_id` | `TestTheReviewsRows` 的"别的提交"一格；单元的"`Rows.Approve` 答假是缺陷"一格（锁下读到的提交在同一个事务里不会变，这一项只守缺陷） |
| 锁下再核闸门 1–3 | `TestUpgradeApproval` 的"以 SQL 清空根下 `AGENTS` 的正文之后批准"（同 7.1 第 4 格；以 SQL 软删除会同时违反 `checkPages` 的几条，设计审查 P3B-N4）答 `wiki.upgrade_not_ready`、`detail` 写着 `agents` 与 "still in review"、什么也没写、记一行 error（决定 293）；单元的三格 |
| 批准重建哈希 | `TestUpgradeApproval` 的 `checkPages` 与 `assertSourceHashesMatchRebuild`；`wiring.wiki` 把 `SourceHashes` 换成什么也不做的替身时 `sourceHashRebuildMismatches` 答出这本笔记本的来源（00 号文档第 9 节） |
| 重建先清区外 | `TestTheSourceRebuildIsTheInvariant` 的"区外带旧哈希"一格 |
| 重建的读头只经键（决定 292） | 守卫（7.1）：调用换回 `SourceHeads` 时两个种子都以"没跑过 `SourceRebuildHeads`"失败；SQL 改回连接的形状时只核对计划的种子上分析过的自定义计划顺序读 `page_contents` |
| 批准删掉基线 | `TestUpgradeApproval` 断言 `upgrade_nodes` 没有这次升级的行；`checkPages` 的升级规则（`upgradeRules()`）对已结束的升级的基线报违规 |
| 批准发 `mode` | `TestUpgradeStatesReachTheStreams` 的"approval"一步（1 帧）；`TestUpgradeApproval` 的另一个标签页的流 |
| 批准自己的期限 | `TestUpgradeApproval` 的"its own deadline"三格（语句之中等笔记本行、语句之中等一个来源的行、两条语句之间） |
| 驳回只给审阅中 | `TestUpgradeReview` 的 `upgrading` 一格答 409、什么也没写 |
| 退回的接管时刻 | `TestUpgradeReview` 的"没有报告、刚提交"答 409 `still being checked`、"一小时之前"答 200；`TestTheReviewsRows` 的 `taken_over` 两格；单元的边界三格 |
| 退回清掉报告 | `TestUpgradeReview` 断言行回到 `backToUpgrading`；之后再提交通过并存下新的报告（`submission_id` 不同） |
| 第 181 条 | `TestUpgradeTrashReservedTitles` 的同步与后台两格答 `page.virtual_file`；`contract_test.go` 的各格；两个模块的 `adapter/http` 表；撤销改名的一格守着今天就有的 `OpRename` 一支在批准之后走得到 |
| 第 181 条不碰回收站的恢复 | 同一个测试的 `restoreNode`、`startTrashRestore` 两格答 200、名为 `index 2`、`log 2`；`contract_test.go` 的 `ChangesetTrashRestore` 一格 |
| prompt 只在 `upgrading` | `TestUpgradePromptMatrix` 的 `reviewing`、`standard`、`llm_wiki` 三列；`TestUpgradePrompt` 的各格 |

## 4. 网页

### 4.1 S1 随服务端的部分

`make gen` 重新生成 `web/packages/api-client/src/schema.gen.ts`（三个新操作的路径与类型；第 5 个提交改的 `revertChangeset`、`startRevert` 的说明也进它的 JSDoc，`schema.gen.ts:1269`、`:1843`）。别的不改：三个新操作的码都已有两种语言的文字，`problem-messages.test.ts`（`W/app/problem-messages.test.ts:47-61`）读契约逐个操作核对，照样通过；`revertChangeset`、`startRevert` 加的 `page.virtual_file` 也已有文字（M10）。假服务端（`W/test/upgrade-server.ts`）在 S1 不加路由：S1 没有调用它们的页面（knip 不许没人用的 service 方法）。

### 4.2 S2 随服务端的部分

不改。升级页的接入一节已经写着 prompt 的名字 `upgrade`（`W/pages/notebook/upgrade-page.tsx:411`、`:425`，`W/i18n/messages/en.ts:721`），S2 让它真的存在。

### 4.3 S3：网页

照 P2 第 9 节"P3/S3"的七条与 00 号文档 4.15"审阅（P3/S3）"，以及决定 291（审阅中收起"放弃"）。

**文件**（设计审查 P3C-M3 要的表）：

| 文件 | 改动 |
|---|---|
| `W/pages/notebook/health-rule.tsx` | `HealthRuleItem`、`HealthCard.cursor` 可为 `undefined`、`summary`/`paths`/`read` 的类型；可缺的 `treeFailed`，原样交给 `HealthItem`（下面） |
| `W/pages/notebook/health-page.tsx` | `paths` 的参数放宽；`read` 在没有游标时抛缺陷（下面） |
| `W/pages/notebook/health-rule.test.tsx` | 替身的 `read` 的类型（`:416`）；`undefined` 游标的几格 |
| `W/pages/notebook/health-item.tsx`、`W/pages/notebook/upgrade-check.tsx` | 树读失败时路径一格空着（`treeFailed`）；`UpgradeReportSection` 经 `HealthRule`、它的 `HealthScope`；`itemsReader` |
| `W/pages/notebook/upgrade-page.tsx` | 审阅一节挪进这一次进入的 `SWRConfig`；三个动作与批准自己的函数；审阅中不给"放弃"（决定 291）；退回的说明与 20 秒的再读；退回之后清掉 `checked` |
| `W/pages/notebook/upgrade-banner.tsx` | `reviewing` 时管理员那一句写"驳回"（决定 291） |
| `W/services/upgrade.service.ts`、`W/stores/notebook.store.ts` | 三个动作；`checkItems` 的游标可缺 |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | 三个按钮、三个对话框、`upgrade.approveNotReady`、`upgrade.sendBackChecking`、`upgrade.bannerAnyAdminReject`、`upgrade.approveRootUnread`；`upgrade.startBody` 的"批准或放弃"改为"批准、驳回或放弃"（en"approved, rejected or abandoned"，`en.ts:704-705`、`zh-CN.ts:680`；设计审查 P3C-N23） |
| `W/pages/notebook/general-page.test.tsx` | `:439` 断言发起对话框的正文，随 `upgrade.startBody` 改 |
| `W/test/upgrade-server.ts` | 三条路由；`reportJSON` 加一个形状 |
| `W/pages/notebook/upgrade-page.test.tsx`、`upgrade-check.test.tsx`、`upgrade-banner.test.tsx`、`W/stores/notebook.store.test.ts` | vitest（下面；`upgrade-page.test.tsx:175-196`、`:856-867` 与 `upgrade-banner.test.tsx:49-51`、`:127-129` 随决定 291 改） |
| `e2e/stories/upgrade/up5-check-submit.spec.ts` | 两处断言（本节末） |
| `e2e/stories/upgrade/up6-…`、`up7-…`、`up8-…`、`e2e/playwright.config.ts` | 页面版本；`threeBrowsers` 加 `upgrade\/up7-`（第 5 节） |

决定 291 查过现有的测试：vitest 里在审阅中断言"放弃"的只有上面三处（`upgrade-page.test.tsx:175-196` 的 `reviewing` 一行断言有"放弃"、`:856-867` 的 `sectionsInOrder` 的审阅一行、`upgrade-banner.test.tsx` 的两格断言 `reviewing` 的横幅写"放弃"）；其余按"放弃"的格都在 `upgrading`（`upgradeJSON()` 的默认）。e2e 里按"放弃"的只有 UP3（`up3-abandon-rollback.spec.ts:253`，在 `upgrading`）。

**`HealthRule` 收存下的报告**（`W/pages/notebook/health-rule.tsx`，第 287 条；P2 第 9 节第 1、2、3、6 条）：

```ts
/** An item a rule's card lists: lint's, or a stored report's, which has no cursor. */
export type HealthRuleItem = LintItem | UpgradeReportItem;

export interface HealthCard {
  readonly items: readonly HealthRuleItem[];
  /** The next page's cursor; undefined: the next read starts from the first page; null: there is no more. */
  readonly cursor: string | null | undefined;
  // error, told, again as today (:18-24)
}

// HealthRule's props (:84-101):
//   summary: Pick<LintRuleSummary, "rule" | "count" | "count_capped">;
//   paths: (items: readonly HealthRuleItem[]) => void;
//   read: (rule: LintRule, cursor: string | undefined, signal: AbortSignal) => Promise<LintPage>;
//   treeFailed?: boolean; // passed on to each HealthItem as is (:223-232); the health page passes none
```

"更多"照旧在 `card.cursor !== null` 时渲染（`:238`），读之前同样判（`:178`），把 `card.cursor`（可能是 `undefined`）交给 `read`。从第一页读到的前几项与存下的重复，`HealthRule` 已按 `item_key` 去重（`:155-160`、`:181`、`:194`），不多出行。`HealthRule` 内部记下的读的游标（`operation` 的 `cursor`，`:107`）随之是 `string | undefined`。健康度页随之改两处（审查 C 只改类型、`tsc --noEmit` 报出三处，设计审查 P3C-M3；初稿说"行为不变、tsc 守着"不对）：`paths`（`health-page.tsx:247`）的参数放宽为 `readonly HealthRuleItem[]`（`healthNodeId` 本就收 `UpgradeReportItem`，`health-item.tsx:17`）；`read`（`:248`）在游标为 `undefined` 时抛一个缺陷（`throw new Error("a health card reads on from its cursor")`），不放宽 `WikiService.items`（`W/services/wiki.service.ts:21`）：健康度页的卡的游标只会是 `string | null`，第一页在摘要里，抛缺陷守住 P2 4.3 的这个用意（主持者倾向的那一种）。`health-rule.test.tsx:416` 的替身的 `read` 的类型随之放宽。检查一节的卡的游标是 `string | null`，行为不变。`UpgradeService.checkItems`（`W/services/upgrade.service.ts:69`）的游标改为可缺，缺时不带 `cursor` 这个查询参数（契约里它本就可缺，`api/dist/openapi.yaml` 的 `listUpgradeCheckItems`）。

**审阅一节**（`UpgradeReportSection`，`W/pages/notebook/upgrade-check.tsx:238-286`）：

- 规则改由 `HealthRule` 渲染（`level={3}`、`mode="llm_wiki"`、`moved="wiki.upgrade_state"`、`paths={noPaths}`，同检查一节的 `card()`，`:170-199`），替掉自己渲染的 `h3`、数与 `HealthItem`（`:261-281`）。卡的初值：`items` 是存下的条目；`cursor` 在 `count > items.length` 或 `count_capped` 时是 `undefined`，否则是 `null`；其余成员照检查一节的 `card()`。规则照检查一节分两组（`:167-169`、`:219-224`）：计数大于 0 的照 `healthRuleOrder` 列出，计数为 0 的收进 `<details>`（`health.clear`）。
- 挪进这一次进入的 `<SWRConfig value={checksConfig}>`（`upgrade-page.tsx:300` 那一个；P2 第 9 节第 4 条）：卡的状态与"更多"读到的条目随这一次进入，离开升级页再回来从存下的报告重新开始。
- 审阅一节的 `HealthScope`（`HealthRule` 的键、"更多"的取舍与 403 的重读都靠它，`health-rule.tsx:106-137`；设计审查 P3C-M4）：
  - `key`：`["upgrade-report", store.loginId, workspace.id, notebook.id, upgrade.id, upgrade.submission_id]`。不带这一次进入的 `locationKey`、`nonce`（检查一节带，`upgrade-check.tsx:93-101`）：缓存已是这一次进入的（`checksConfig`），键里不再带它，"审阅一节留在 `SWRConfig` 之外"的变异才会让离开再进入接着旧的条目、被那一格抓到。带 `submission_id`：退回之后再提交存下的新报告不接着旧的条目。
  - `current()`：这一次进入还活着（`entry.control.current.alive`）、登录没变（`store.auth.state.loginId === store.loginId`）、显示的升级的 `submission_id` 等于键里的（每次渲染写入的 ref）。不看检查一节的 `generation`：审阅中没有检查。
  - `controllers`：这一次进入的（`entry.control.current.controllers`，同检查一节）。
- `readItems` 提成模块里的一个函数（P2 第 9 节第 5 条）：

  ```ts
  /** A rule's items read for a section of the upgrade page: forbidden, while the section is current, rereads the notebooks first. */
  function itemsReader(
    upgrades: Pick<UpgradeService, "checkItems">, notebookId: string,
    current: () => boolean, rereadNotebooks: () => void
  ): (rule: LintRule, cursor: string | undefined, signal: AbortSignal) => Promise<LintPage>
  ```

  检查一节与审阅一节各以自己的 `current()` 调它；`UpgradeReportSection` 从 `UpgradeRead` 收 `rereadNotebooks`（外层的 `mutate(["notebooks", ws])`，`upgrade-page.tsx:149`，不是这一次进入的缓存的）。两节各一格 403 的测试（同一个判断两处用，各自测）。
- 树读失败（P2 第 9 节第 7 条）：`UpgradeRead` 把 `tree={{ error, reading, retry }}` 交给检查与审阅两节，照健康度页（`W/pages/notebook/health-page.tsx:51`、`:60`、`:245`、`:292-294`）：`reading` 是 `pages.nodes === undefined && tree.error === undefined || tree.isValidating`，失败时整节的顶上一处显示"加载失败"与"重试"（`NotLoaded`），不再一直写"加载中"；闸门的节点（`GateNode`，`upgrade-check.tsx:384`）与条目的路径（`HealthItem`，`health-item.tsx:64`）收一个可缺的 `treeFailed`，为真时那一格空着，不写"（已不在）"（`upgrade.gateNodeGone`）、"节点已不可用"（`health.nodeUnavailable`）：树没读到，说节点不在是假话（设计审查 P3C-N12）。检查与审阅两节的条目都经 `HealthRule` 渲染，`HealthRule` 给 `HealthItem` 今天只交 `readingTree`（`health-rule.tsx:223-232`），所以 `HealthRule` 的 props 加可缺的 `treeFailed`、原样交给 `HealthItem`（设计审查 P3C-N20）。健康度页不交 `treeFailed`，照旧。
- 没有报告时照旧写"正在核对，或上一次提交没有做完。"（`upgrade.reportPending`）。

**三个动作**（第 288、291 条）：`reviewing` 时按钮一行（`upgrade-page.tsx:280-291`）依次是"批准…"（en"Approve…"，只在有报告时）、"退回继续修改…"（"Send back for more work…"）、"驳回…"（"Reject…"）；不给"放弃这次升级…"（决定 291：今天 `:280-291` 在 `reviewing` 也给它，这一处改为只在 `upgrading`）。`upgrading` 照旧是"提交审阅…"与"放弃这次升级…"。对话框都是 `ConfirmDialog`，`held={dialogOf(which)}`（`:228-234`），`type Change`（`:64`）加 `"approve" | "sendBack" | "reject"`：

| 动作 | 语气 | 标题（en / zh） | 正文（zh） | 确认按钮（en / zh） |
|---|---|---|---|---|
| 批准 | `default` | Approve the upgrade of “{name}”? / 批准「{name}」的升级？ | 笔记本将成为 LLM Wiki：来源区（{root}）下的页与附件只读，`AGENTS` 与来源根受保护，生成的目录与日志出现在页面树里。批准之后没有整体的撤销：升级的变更集仍可在历史里照 LLM Wiki 的规则逐个撤销；要回到普通笔记本用降级。 | Approve the upgrade / 批准升级 |
| 退回 | `default` | Send back the upgrade of “{name}”? / 退回「{name}」的升级？ | 升级回到进行中，发起人的 agent 可以接着改；这次的报告不再保留，之后要重新提交。 | Send it back / 退回 |
| 驳回 | `destructive` | Reject the upgrade of “{name}”? / 驳回「{name}」的升级？ | 照 `upgrade.abandonBody` 的说法（`zh-CN.ts:699-700`、`en.ts:726-727`；决定 291 的理由是两者后果相同，文字也相同）："一个后台任务把笔记本复原到升级开始时的样子：原有的页与附件回到原处，名称与正文复原，之后新建的进回收站。回滚结束之前，笔记本照旧只读。"再接一句"回滚停下时可以继续，或者结束回滚、保留当时的样子。"（设计审查 P3C-N22：初稿写"agent 新建的页进回收站"，漏了发起补写的 `raw`、`AGENTS` 与附件、文件夹） | Reject the upgrade / 驳回升级 |

`{root}` 是提议的来源根的路径（`nodeTarget(pages, upgrade.source_root_id)`）；树还没读到或读失败时写"提议的来源根"（`upgrade.approveRootUnread`，en"the proposed source root"），不等树（设计审查 P3C-N13）。中文标题照升级的另四个对话框（`upgrade.startTitle`、`abandonTitle`、`continueTitle`、`endTitle`，`zh-CN.ts:678`、`:698`、`:711`、`:716`）用「」，英文照旧用 “”（设计审查 P3C-N21）。"生成的目录与日志出现在页面树里"照降级的说法（`zh-CN.ts:654`、`en.ts:678` 的"生成的目录与日志从页面树里消失"）。

拒绝的文字：驳回与退回经 `change`（`:176-204`），它对 `wiki.upgrade_state`、`wiki.upgrade_not_ready` 重读升级再抛出（`:183-192`），对话框照 `problemMessages` 显示。批准有自己的函数（设计审查 P3C-M2：`change` 成功之后在页面还装着时一定 `reread()`、`shows` 时还先把答复写进缓存，`:198-203`，批准经它就在跳转之前多读一次升级、答 404，UP7、UP8 会因为未声明的 404 失败，`holdStream` 挡不住它）：拒绝那一支照 `change`（`wiki.upgrade_state`、`wiki.upgrade_not_ready` 重读升级再抛出）；成功那一支不写缓存、不 `reread`，只在 `mounted()` 时跳转（下面）。批准答 `wiki.upgrade_not_ready` 时对话框显示 `upgrade.approveNotReady`（zh"批准前重新核对时有检查没有通过，升级仍在审阅中：可以退回继续修改，或者驳回。"，en"Checked again before the approval, a check did not pass; the upgrade is still in review: send it back, or reject it."；批准的 `texts` 照提交的 `:343-353` 写）：`upgrade.notReady` 与通用的 `problem.wiki.upgrade_not_ready` 都写"已回到升级中"，批准时不实（同 2.1 第 4 条在服务端改掉的那一句；设计审查 P3C-M1）。`server_busy` 显示通用的忙的文字，不自动重试（同发起）。

**退回在提交正在计算时**（00 号文档 4.15，核对 C3-M2）：退回答 409 `wiki.upgrade_state`、而 `change` 重读之后的升级仍是"审阅中、没有报告"时（经 `ConfirmDialog` 的 `explain` 按重读之后的状态定，不按打开对话框时的：409 也可能来自别的管理员刚驳回或退回；设计审查 P3C-N14），对话框显示自己的文字 `upgrade.sendBackChecking`（zh"上一次提交正在计算，稍后再试。"，en"The previous submission is still being checked: try again in a moment."），不显示服务端英文的 `detail`；`change` 已重读升级；另在 20 秒之后再自动重读一次（`setTimeout`，`UpgradeRead` 卸下、升级的状态变了或下一次退回时清掉；20 秒是 `LintTimeout + 10 s` 的上界，网页拿不到实例的 `LintTimeout`）。有报告时的 409（例如别人已经批准）照通用的文字。

**批准之后**（第 289 条）：`store.approveUpgrade` 成功之后，若页面还装着（`useMounted`，`W/app/mounted.ts:12`），`navigate(\`/${slug}/notebooks/${id}\`, { state: arrived })`，笔记本首页的标题接到达的焦点（`notebook-home.tsx:34`、`:44`）；对话框随页面卸下，不走 `onClosed` 的焦点。不再读 `["upgrade", id]`（批准之后它答 404 `wiki.upgrade_not_found`）。驳回与退回留在升级页，照现有的 `change` 盖住答复再重读，焦点照 `dialogOf` 回到按钮或标题。退回成功之后清掉这一次进入的 `checked`（`upgrade-page.tsx:115`，`setChecked({ generation: 0, upgrade: "" })`），检查一节回到"还没有检查"：不显示提交之前缓存里的结果，那容易让人误读（设计审查 P3C-N15，主持者倾向的那一种）。

**横幅**（决定 291）：`upgrade-banner.tsx:42` 的管理员那一句按状态：`upgrading` 照旧 `upgrade.bannerAnyAdmin`；`reviewing` 用新的 `upgrade.bannerAnyAdminReject`（zh"任何笔记本管理员都可以驳回这次升级。"，en"Any admin of the notebook can reject this upgrade."）；`rolling_back` 照旧不写（决定 246）。

**store 与 service**：

- `UpgradeService`（`W/services/upgrade.service.ts`）加 `approve(notebookId)`、`reject(notebookId)`、`sendBack(notebookId)`，各 `POST` 它的路径、答 `Upgrade`；`checkItems` 的游标可缺（上面）。
- `NotebookStore`（`W/stores/notebook.store.ts`）的构造参数的 `Pick`（`:54`）加 `"approve" | "reject" | "sendBack"`；`approveUpgrade(id)`、`rejectUpgrade(id)`、`sendBackUpgrade(id)` 经 `inTurn` 与 `upgraded`（`:195-207`），拒绝之后要重读列表的码：三个都是 `forbidden`、`notebook.not_found`、`wiki.upgrade_state`（同 `behindRollback`，`:310`）。批准不收 `wiki.upgrade_not_ready`：它只在缺陷时到得了，列表也不变（同 `behindSubmit` 不收它；设计审查 P3C-N11）。成功之后照 `upgraded` 重读列表（批准之后列表里这本笔记本是 LLM Wiki、没有升级，横幅随之消失）。

**假服务端**（`W/test/upgrade-server.ts`，05-lessons 第 2 节：只答真服务端答得出的码，码序照真序）：

- `POST …/upgrade/approve`：不是管理员 403；不是 `reviewing` 或没有报告 409 `wiki.upgrade_state`；可配的结局 `not_ready`（409 `wiki.upgrade_not_ready`，`detail` 照服务端的句子）与 `busy`（503 `server_busy`、`Retry-After: 1`）；否则 200 `approved`，笔记本的 `mode` 改为 `llm_wiki`、`upgrade` 为 `null`，发一次 `mode`。
- `POST …/upgrade/reject`：403；不是 `reviewing` 409；否则 200 `rolling_back`、`ending: "rejected"`、带 `rollbackJobJSON`（`:159`），发一次 `mode`。
- `POST …/upgrade/send-back`：403；不是 `reviewing` 409；没有报告而没有设"已过接管的时刻"时 409 `wiki.upgrade_state`、`detail` 是"The previous submission is still being checked: check again in a moment."；否则 200 `upgrading`、提交与报告清空，发一次 `mode`。
- `GET …/upgrade/check/{rule}`：不带游标时答第一页、`reviewing` 时也答，今天已是如此（`upgrade-server.ts:350-356`、`:470`），不改（设计审查 P3C-N7）。
- `reportJSON`（`:130`）加一个带条目、`count` 比条目多的形状，每个值互不相同。

**测试**（vitest；夹具的标量互不相同、整值断言，05-lessons 的"投影的测试"）：

- `health-rule.test.tsx`：游标为 `undefined` 的卡渲染"更多"、点它以 `undefined` 调 `read`、第一页里与已有条目同 `item_key` 的不重复、之后照答复的 `next_cursor` 继续；游标为 `null` 的不渲染、不读；`UpgradeReportItem` 的条目照样渲染（没有 `cursor` 成员）。
- `upgrade-check.test.tsx`：审阅一节经 `HealthRule` 渲染每条规则（数、至多五项）；"更多"从第一页读、请求不带 `cursor`；读到的追加在存下的之后；条目已是全部的规则没有"更多"；审阅一节的"更多"答 403 时重读笔记本列表（且只在这一代当前时）；检查一节的 403 一格照旧；树读失败时两节显示"加载失败"与"重试"，点重试之后读出路径；离开再进入，审阅一节从存下的报告重新开始（这一次进入的缓存）；退回之后再提交的新报告不带旧的条目（键带 `submission_id`）；计数为 0 的规则在 `<details>` 里；树读失败时闸门的节点与条目的路径一格空着、没有"（已不在）"与"节点已不可用"。
- `upgrade-page.test.tsx`：审阅中有报告时三个按钮依次在、没有"放弃这次升级…"（`:175-196` 的 `reviewing` 一行改为断言没有"放弃"；`:856-867` 的 `sectionsInOrder` 的按钮表加三个名字，审阅一行是 `["Where it is", "Check report", "Approve…", "Send back for more work…", "Reject…"]`）；没有报告时没有"批准"、有另两个；`upgrading` 照旧有"放弃"；每个对话框的语气、标题与正文（树没读到时 `{root}` 是"提议的来源根"）；批准成功之后在笔记本首页、焦点在标题、不再读升级（请求的记录里批准之后没有 `GET upgrade`）；批准答 `wiki.upgrade_not_ready` 时对话框留着、显示 `upgrade.approveNotReady`、升级被重读；退回成功之后检查一节没有提交之前的结果；退回答 409 而重读之后已不在审阅中时显示通用的文字、不是 `sendBackChecking`；批准答 503 的文字；驳回之后页面是回滚一节（任务的整行）；退回之后回到升级中、接入一节与"提交审阅…"出现；没有报告时退回答 409：显示 `upgrade.sendBackChecking`、重读一次、20 秒之后再读一次（假的计时器），卸下之后不再读；有报告时的 409 显示通用的文字；`mode` 事件之后审阅一节随新的报告更新。
- `upgrade-banner.test.tsx`：`:49-51` 与 `:127-129` 两格改为：管理员在 `upgrading` 看到"Any admin of the notebook can abandon this upgrade."，在 `reviewing` 看到"Any admin of the notebook can reject this upgrade."、没有"abandon"那一句，`rolling_back` 两句都没有（决定 291）。
- `notebook.store.test.ts`：三个动作各照降级的七条（`:761`、`:783`、`:837` 那几组）：成功之后重读列表、拒绝之后按各自的码重读、轮到它才发、并发时的次序。
- `upgrade.service` 的测试（若有，`checkItems` 不带游标时的查询串；三个动作的路径与方法）。

**UP5 的页面断言**（P2 第 9 节第 1 条连带）：`e2e/stories/upgrade/up5-check-submit.spec.ts:312` 读每条规则一节的第一个 `<p>`（今天是数），换成 `HealthRule` 之后第一个 `<p>` 是规则的说明（`health-rule.tsx:216-218`），改为按数的那个元素读；计数为 0 的规则收进 `<details>` 之后排在计数大于 0 的之后（节仍在 DOM 里，`evaluateAll` 读得到），期望的次序照"计数大于 0 的在前、为 0 的在后"；`:321` 断言报告里没有按钮，改为"条目已是全部的规则没有'更多'"（按名字找，不数所有按钮：`<details>` 的 `<summary>` 不是"更多"）。随 S3 的第 1 个提交改，不然 e2e 在那个提交上红。

## 5. e2e

新故事都在 `e2e/stories/upgrade/`，帮手加在 `e2e/fixtures/upgrades.ts`，照 `postSubmit`/`submitUpgrade` 的一对（`:102`、`:110`）：`postApprove`、`postReject`、`postSendBack` 答 API 的原样（含拒绝），`approveUpgrade`、`rejectUpgrade`、`sendBackUpgrade` 断言 200、答 `Upgrade`（设计审查 P3C-N8）。准备一律经 API 与 MCP（10.1：REST 的写在冻结里被拒，整理由发起人的 agent 经 MCP 做）。

| 故事 | 做什么 | 断言 | 版本 |
|---|---|---|---|
| UP6 退回与驳回（`up6-send-back-reject.spec.ts`） | 一本普通笔记本，一页原有页，另一本普通笔记本（占 worker 用）；alice 发起（补写 `raw`、`AGENTS`）；她的 agent 经 MCP 给原有页补 frontmatter、`upgrade_submit`；第二位管理员 dana 退回；agent 再写一次 frontmatter（被放行）、再交；以另一本笔记本的导入占住唯一的导入 worker（`holdNotebookRow` 加 `startImport`，`up3-abandon-rollback.spec.ts:145-161` 的做法）；dana 驳回；放开 worker，等回滚的任务做完 | API 版本：退回答 `upgrading`、提交与报告为空；再交存下新的报告（`submission_id` 不同）；驳回答 `rolling_back`、`ending: "rejected"`、带任务，任务在排队；回滚做完之后原有页的 frontmatter 与发起时相同，发起补写的 `raw`、`AGENTS` 在回收站，`notebook_upgrades` 的行是 `rolled_back`、`rejected`。MCP 版本：退回之后 agent 的写成功；驳回之后、放开 worker 之前 agent 经 MCP 写答 409 `notebook.upgrading`；任务做完之后 agent 的写放行（设计审查 P3C-N9、P3C-M5） | API、MCP：S1；页面：S3 |
| UP7 批准（`up7-approve.spec.ts`） | 普通笔记本：根下一页 `index`（发起之前删掉，留下它的变更集）、`raw` 下两个来源（一页、一个附件）、一页笔记；alice 发起、agent 补 frontmatter、交；dana 批准 | 答 `approved`；笔记本 `mode = llm_wiki`、`source_root_id` 是 `raw`；来源的哈希等于正文与附件的（数据库断言）；`upgrade_nodes` 没有这次升级的行；MCP：`list` 列出 `index.md`、`log.md` 两项虚拟的、`delete AGENTS.md` 答 `page.protected`；撤销删掉 `index` 的那个变更集答 409 `page.virtual_file`（第 181 条），同步与后台各一次；另一个标签页随 `mode` 看到 LLM Wiki | API、MCP：S1；页面：S3（三种浏览器） |
| UP8 只在网页上走完（`up8-web-only.spec.ts`） | 空的普通笔记本：发起、检查、提交、批准，不接 agent | 每一步的答复；批准之后是 LLM Wiki，根下有补写的 `raw` 与 `AGENTS` | API：S1；页面：S3。MCP 豁免：故事本身就是不接 agent；批准之后的 MCP 由 UP7 盖住 |
| UP9 upgrade 的 playbook（`up9-playbook.spec.ts`） | 升级中的笔记本 `prompts/list` 与 `prompts/get upgrade`；一本普通笔记本 `prompts/get upgrade` | 列表里有 `upgrade`（名字、标题、说明等于 `prompts.json`）；文字的第一行是"Upgrade this notebook to an LLM Wiki."；普通笔记本答 `-32602`、`data` 是 `wiki.upgrade_state` | MCP：S2（只能由 MCP 完成，不写，不断言数据库，10.1 的例外） |

**UP6 的"驳回之后 MCP 写答 `notebook.upgrading`"**（00 号文档第 9 节 UP6 一行）照 UP3 的 API 版本：以另一本笔记本的导入占住唯一的导入 worker，回滚的任务确定地排队，再断言冻结（`up3-abandon-rollback.spec.ts:145-161`、`:187-193`），不豁免（设计审查 P3C-M5；初稿以"回滚几十毫秒做完、e2e 等不住"豁免它）。

**页面版本**（S3）：

- UP6：dana 在升级页上退回（对话框、答复之后是升级中、接入一节出现）。agent 再交之后，dana 的那一页以 `holdStream` 重新打开（不放开，或离开升级页之后才放开）；审阅中按钮一行恰是"批准…""退回继续修改…""驳回…"，没有"放弃这次升级…"（决定 291）。照 API 版本占住唯一的导入 worker，回滚的任务因此排队。dana 驳回：对话框、回滚一节的任务整行（排队），再放开 worker。之后唯一的 404 是 `RollbackJob` 看到任务结束之后的那一次重读（`upgrade-page.tsx:518-525`），以 `pageWatch.expectConsole({ errors: [failedToLoad(404)] })` 声明（13.2 第 9 条），等"没有进行中的升级"出现。流不通，就没有两条 `mode` 引起的重读（驳回的与回滚做完的）；worker 被占住，`change()` 之后的重读一定是 200；剩下 `RollbackJob` 的那一次恰好一次（设计审查 P3C-I1；初稿照 UP3 写"两个刻意的 404"，在这里先后与次数都不定）。
- UP7（三种浏览器，`playwright.config.ts:3` 的 `threeBrowsers` 加一支 `upgrade\/up7-`）：dana 批准的那一页以 `holdStream`（`e2e/fixtures/events.ts:214`）打开，它的事件流在跳转到笔记本首页之后才放开：`mode` 可能早于批准的答复到达，那时升级页还装着，会重读升级、多出一条先后不定的 404；放开之后的重连只重读装着的键，升级页已卸下。断言跳转之后焦点在笔记本首页的标题、横幅消失、首页是 LLM Wiki 的样子。alice 的另一个标签页停在笔记本首页，先等它的事件流连上（照 UP1 的写法），批准之后不刷新就看到横幅消失。撤销 `index` 的那次删除在历史页上点"撤销"，答 409 `page.virtual_file`，以 `expectConsole({ errors: [failedToLoad(409)] })` 声明，对话框显示它的文字。
- UP8：发起、检查、提交、批准都在网页上；批准的那一页同 UP7 以 `holdStream` 打开；对话框与按钮按确切的名字找（05-lessons 第 6 节）。

UP5 的页面版本随 S3 的第 1 个提交改两处断言（4.3 末）。LW11（`e2e/stories/wiki/lw11-playbook.spec.ts:27-81`）按契约文件比较 prompt 的列表，加了 `upgrade` 照样通过（普通笔记本一格只取 `lint`）；它的标题写着"three playbooks"，`:10` 的注释"takes the three prompts"、`:62` 的"A standard notebook lists the same three"也随 S2 不实：S2 的第 2 个提交改为"the LLM Wiki's playbooks""takes the LLM Wiki's three prompts""lists the same prompts"，不改断言（设计审查 P3C-N5）。

## 6. 实施步骤

每个提交都过 `make lint`、`make test-go-short` 与它碰到的测试；分支的 CI 在合并之前全绿。

### 6.1 S1（`m11-p3-s1`）

1. `build, ci: move four of the fourth shard's tests to the sixth, and the review's tests to the eighth (M11/P3/S1)`：`Makefile` 的 `BOOTSTRAP_SIXTH`（第 278 条）、`BOOTSTRAP_EIGHTH`（第 290 条）、两处的 `##` 与 `#` 说明、计数注释（`Makefile:265-266`）145/50/120/124/61/11/27/9 → 145/50/120/120/61/15/27/9；`ci.yml` 的 bootstrap-4、bootstrap-6、bootstrap-8 一项前的说明。核对：`go test -race -list . ./internal/bootstrap` 的全部名字照 `make -n test-go` 展开的 `-run`/`-skip` 归一次，每个恰在一份（P2 的分份旁支的做法，`tools/shards8-p2.py`），只有这四个换了份；`make -n test-go` rc=0。
2. `server: the review's judgments, and the approval's write of the notebook (M11/P3/S1)`：3.2 的 `ReviewOf`、`JudgeRoots`、`ApprovalNotReady` 与测试；`JudgeUpgrade` 改用 `JudgeRoots`；提交的第一个事务改用 `ReviewOf`（3.9）；3.3 的 notebook 的 SQL、`UpgradeStates.Approve`、wiki 的端口与 `wikiUpgradeStates.Approve`、测试。
3. `server: rebuild an approved notebook's source hashes (M11/P3/S1)`：3.4 的 `RebuildSources`、`SourceRebuild`、`SourceRebuildHeads`（SQL、store、`gen/`，决定 292）与三组测试。
4. `server, api, web: approve, reject and send back an upgrade (M11/P3/S1)`：3.5 的 SQL 与 store；3.6–3.8 的用例与单元测试；3.11 的契约、处理函数、`make gen`（含 `schema.gen.ts`）与模块的码的测试；3.12 的接线；3.13 的矩阵的行；3.14 的日志（含决定 293 的 error 一行）；组合根的 `TestUpgradeApproval`、`TestUpgradeReview`、`TestUpgradeReviewInterleavings`、`TestUpgradeStatesReachTheStreams` 加的步、`TestUpgradeSubmitInterleavings` 第 7 条的退回与 `TestUpgradeCheck` 第 8 条的批准（7.1、7.2）。
5. `server, api, web: a revert does not bring a node back to a reserved root title (M11/P3/S1)`：3.10 的 `CheckContract`、两个操作的契约与 `make gen`（说明进 `schema.gen.ts` 的 JSDoc，所以带 `web`，设计审查 P3C-N2）、模块的码的测试、`TestUpgradeTrashReservedTitles`（含撤销改名的一格）。
6. `server, ci: the approval's cost guard (M11/P3/S1)`：`bootstrap/upgrade_approve_cost_test.go`（7.1）、`Makefile` 的 `test-go-costs-1` 加一行（带 `-v` 与显式的 `-timeout 15m`；说明照 `Makefile:433-436` 回滚先删那一行与 `:471-478` 的写法："本机约 4–5 分钟、7763 上约 7–9 分钟，过了 Go 默认的 10 分钟超时的一半，所以这一行 -timeout 15m：红在守卫自己的界上，不在 Go 默认的超时上"；设计审查 P3B-M7）、`.github/workflows/ci.yml` 的 costs-1：`timeout` 从 35 分钟提到 45 分钟（第 294 条），那一行的注释写实测的三次（14:31、15:06、15:41）与估计（P3 之后约 23–26 分钟，照约两倍的规则取 45；开始约第 15–17 分钟，加 `-timeout 15m` 到第 30–32 分钟，Go 的超时先于作业时限到）。不拆行：只核对计划的种子与计时的在同一个测试里，一个二进制。本机两轮，界照实写进说明与 10.1。
7. `e2e: approve, reject and send back through the API and MCP (M11/P3/S1)`：UP6、UP7、UP8 的 API 版本与 UP6、UP7 的 MCP 版本、`fixtures/upgrades.ts` 的三个帮手。
8. Step 的文档提交（第 8 节第 4、5 条）。

### 6.2 S2（`m11-p3-s2`）

1. `server, api: the upgrade prompt and its playbook (M11/P3/S2)`：3.16 的用例、`playbook-upgrade.md`、`UpgradePlaybookText`、`prompts.go`、`prompts.json` 与单元测试；3.18 的四处组合根测试（第三份的 `TestThePromptReadsLLMWikiMatrix`、`TestThePromptListsAreTheContract`，第七份的 `TestUpgradePromptMatrix`、`TestUpgradePrompt`），计数注释 +2；`upgrade_submit` 的工具说明加"or sends it back"（`api/mcp/tools.json:589` 与账户入口的同一项 `:1298`，设计审查 P3C-N4）。
2. `e2e: the upgrade playbook over MCP (M11/P3/S2)`：UP9；LW11 的标题与 `:10`、`:62` 的注释（第 5 节末；设计审查 P3C-N25）。
3. `docs: README's upgrade section and the MCP checklist's K (M11/P3/S2)`：3.19（新的一节，连同 `README.md:214`、`:234`、`:236`、`:246` 与 MCP 一节的两个工具）、3.20（K1–K5）。
4. Step 的文档提交。

### 6.3 S3（`m11-p3-s3`）

1. `web, e2e: a stored report's rules through HealthRule, More from their first page (M11/P3/S3)`：`HealthRule` 的类型与游标、健康度页随之改的两处（`paths` 的参数、`read` 在没有游标时抛缺陷）、`checkItems` 的可缺的游标、`itemsReader`、审阅一节挪进这一次进入的 `SWRConfig`（它的 `HealthScope`）、树读失败；对应的 vitest；UP5 的页面版本的两处断言。
2. `web: approve, send back and reject an upgrade (M11/P3/S3)`：service、store、按钮与对话框（批准自己的函数）、审阅中收起"放弃"与横幅的"驳回"一句（决定 291）、两种语言的文字、退回的说明与 20 秒的再读、批准之后的跳转、假服务端的三条路由；vitest。
3. `e2e: approve, send back and reject in the web app (M11/P3/S3)`：UP6–UP8 的页面版本、`threeBrowsers` 加 `upgrade\/up7-`。
4. Step 的文档提交。

### 6.4 合并的门禁

照 P2 6.4：Step 的分支 CI 19 项全绿、审查与修复核对结束、`--no-ff` 合并；合并之后两次 `main` 的组合根各份的 Test 一步平均在 450 秒之下（第 8 节第 1 条）。e2e 的 L1 照第 277 条照现状；CI 再红就只重跑 e2e，记下 run 与 attempt。

## 7. 测试与验证

### 7.1 各项与它们的测试

| 项 | 测试 | 份 / 位置 |
|---|---|---|
| `ReviewOf`、`JudgeRoots`、`ApprovalNotReady` | 3.2 的单元测试 | `wiki/domain` |
| notebook 的批准的写 | `TestAnUpgradeIsApproved` | `notebook`（真库） |
| 哈希的重建 | `page/app/source_rebuild_test.go`、`SourceRebuildHeads` 的 store 测试、`TestTheSourceRebuildIsTheInvariant` | `page/app`、`page/adapter/postgres`、`page`（真库） |
| wiki 的两条 SQL | `TestTheReviewsRows` | `wiki/adapter/postgres`（真库） |
| 三个用例与提交的改动 | 3.6–3.9 的单元测试；`submit_upgrade_test.go` 照旧 | `wiki/app` |
| 契约与码 | `TestTheUpgradesCodesAreAnswered`、`TestTheUpgradeIsAnswered`、`history_test.go`、`start_background_test.go` | 各模块的 `adapter/http` |
| 第 181 条的判定 | `contract_test.go` 的六格 | `page/domain` |
| **批准**（整个程序） | `TestUpgradeApproval` | 第八份 |
| **退回与驳回**（整个程序） | `TestUpgradeReview` | 第八份 |
| 交错第 9 条与新的两条 | `TestUpgradeReviewInterleavings` | 第八份 |
| 交错第 7 条的退回、第 8 条的批准 | `TestUpgradeSubmitInterleavings`、`TestUpgradeCheck` 加格 | 第八份 |
| 事件 | `TestUpgradeStatesReachTheStreams` 加步 | 第七份 |
| 矩阵 | `TestUpgradePermissionMatrix` 加三行 | 第七份 |
| 第 181 条（整个程序） | `TestUpgradeTrashReservedTitles` | 第七份 |
| prompt | `TestUpgradePromptMatrix`、`TestUpgradePrompt`；改写的 `TestThePromptReadsLLMWikiMatrix`、`TestThePromptListsAreTheContract` | 第七份；第三份 |
| 成本 | `TestUpgradeApprovalTakesItsTimeAtAnyStatistics` | costs-1（`!race`） |
| 网页 | 4.3 的 vitest | `W/` |
| 故事 | UP6–UP9 | e2e |

**`TestUpgradeApproval`**（`bootstrap/upgrade_approve_test.go`，新）。夹具 `tm.reviewed(t, title)`：一本普通笔记本，根下一页带干净 frontmatter 的笔记（`cleanNote`）与一页 `raw`，`raw` 下：两页带正文的来源、一页空页、一页带正文的页与它下面一页空的子页、一个附件（`tm.upload`）、一个零字节的附件；`tm.upgraded` 发起（认出 `raw`、补写 `AGENTS`）、`tm.submitted` 提交通过；交回笔记本、升级与提交的 id。发起、提交、批准是三个不同的账户：alice 发起，第三位管理员 erin 提交，dana 批准（erin 不在 acme 的四个账户里，`interleavings_workspace_test.go:120` 只注册 alice、bob、carol、dana，`tm.join` 只写成员行、要求账户已经存在，`:141-147`：夹具先以 `registerAccount` 注册 erin，再加入工作区、加为这本笔记本的管理员；设计审查 P3A-N12）（每个标量互不相同；dana 既提交又批准时，变异"`Rows.Approve` 把 `updated_by_id` 写成 `submitted_by_id`"抓不到，设计审查 P3A-N5）。子测试：

1. **码的次序**：没有笔记本 404；carol 看不到 404；bob（编辑者）403；没有升级 409；`upgrading` 409；审阅中没有报告、刚提交 409（`detail` 不是"still being checked"）；同样而以 SQL 把 `submitted_at` 拨回一小时 409；每格之后行与笔记本不变（`tm.submission` 与模式）。
2. **批准**：dana（第二位管理员）批准，答 200，答复整值断言（`state: approved`、`submitted_by` 是 erin、`report` 原样、`ending`、`rollback_*` 为空；`started_by`、`submitted_by` 与批准人是三个不同的人）；笔记本 `mode = llm_wiki`、`source_root_id` 是 `raw`、升级的四列为空；`upgrade_nodes` 没有这次升级的行；`notebook_upgrades` 的行 `approved`、审计列是 dana 与时刻；`assertSourceHashesMatchRebuild`、`checkPages`（含 `sourceHashGap` 与升级规则）、`checkLinks`、`checkSearch`；`getUpgrade` 答 404 `wiki.upgrade_not_found`；笔记本的读答 `mode: llm_wiki`、`upgrade: null`；之后 LLM Wiki 的契约生效：删除根下的 `AGENTS` 答 `page.protected`、编辑一页来源答 `page.source_immutable`；info 一行（3.14），`sources_hashed` 等于期望的数（五：两页、一页带正文的文件夹页、两个附件）。
3. **另一个标签页**：alice 的流在批准之前打开，批准之后收到一帧 `mode`（`tm.modesOf` 开的是 alice 的流，`upgrade_submit_test.go:98-106`；设计审查 P3A-N9）。
4. **闸门在锁下再核**：另一本 `tm.reviewed`，以 SQL 把根下 `AGENTS` 的正文清空（绕过冻结，模拟缺陷；同时改 `byte_size` 与 `content_hash`，`00013_page_page_contents.sql:12-15` 的检查），批准答 409 `wiki.upgrade_not_ready`，`detail` 写着 `agents` 与"still in review"；行与模式不变、没有 info；error 一行整行断言（`notebook_id`、`upgrade_id`、`failed_gates` 是 `["agents"]`，决定 293）。
5. **空的重建**（00 号文档第 9 节的反向对照）：另配一个应用，`wiring.wiki` 把 `SourceHashes` 换成什么也不做、交回 0 的替身；批准答 200，而 `sourceHashRebuildMismatches`（`source_hash_invariant_test.go:43`）答出的节点的集合恰是这本笔记本的来源（`checkPages` 只给每条规则的计数句子，读不出集合；设计审查 P3B-N5），不是调 `checkPages` 让测试失败。
6. **自己的期限**（照发起的"its own deadline"，`upgrade_start_test.go:216-280`）：另配应用，第一、三格 `ApproveTimeout` 300 ms，第二格 2 s（见下）；三格各答 503、`Retry-After: 1`、warn 一行（`timeout_ms` 是这一格的期限）、没有 error 行、什么也没写（模式、四列、哈希、基线行、wiki 的行）、`checkPages`：
   - 语句之中、判定之前：测试持着笔记本行（`tm.holdNotebook`），批准等在锁上到期；
   - 语句之中、写了之后：测试持着 `raw` 下一页来源的 `nodes` 行（`SELECT … FOR UPDATE`），批准写了模式、清了哈希之后，`SetSourceHashes` 等在那一行上到期（`ForgetSources` 不碰它：普通笔记本的哈希都是空，`WHERE source_hash IS NOT NULL` 不匹配）。钉住"写了之后到期"，不只看 503 与"什么也没写"（期限在更早的一步到期，例如慢的 runner 上 300 ms 在 `SourceNodes` 就用完，也一样绿）：包住 `SourceHashes` 的一层在进 `Rebuild` 时记下（不拦，照 `slowCapture.waited`），并以 `pgtest.WaitForLockWaitsOn(nodes, 1)` 看到批准等在那一行上，两样都断言（设计审查 P3B-N6）。这一格的 `ApproveTimeout` 取 2 s：这个等待只在"走到 `SetSourceHashes`"与期限到期之间，`WaitForLockWaitsOn` 每 10 ms 轮询、到它自己的时限就 `t.Fatal`（`pgtest/lockwait.go:45`、`:136-155`），300 ms 在慢的 runner 上可能还没走到这一步就到期，会偶发地红；2 s 下等待者在锁上一直等到期限，这一格只多约 2 秒（设计审查 P3B-N9）；
   - 两条语句之间：包住 `SourceHashes` 的替身在 `Rebuild` 里等 ctx 到期（照 `slowCapture`，`:286-298`）。
   - 另一格：请求的 ctx 先到期（客户端断开）时不记 warn、不答 503（单元测试已有，这里不重复）。

**`TestUpgradeReview`**（`bootstrap/upgrade_review_test.go`，新）：

1. **退回的码**：404、404（carol）、403（bob）、没有升级 409、`upgrading` 409、`rolling_back` 409；审阅中没有报告、刚提交：409、`detail` 含"still being checked"；以 SQL 拨回一小时：200（接管）。
2. **退回**：有报告的升级，dana 退回，答复整值断言（`upgrading`，提交与报告为空）；行是 `backToUpgrading`（`upgrade_submit_test.go:88-91`）；一帧 `mode`；之后 alice 的 agent 经 MCP 写一页原有页的 frontmatter 被放行（`tm.agent`），再交通过、存下新的报告（`submission_id` 不同）；info 一行（`reported: true`）。
3. **驳回的码**：404、404、403、没有升级 409、`upgrading` 409（什么也没写、没有任务）。
4. **驳回**：有报告的升级，dana 驳回：答 `rolling_back`、`ending: rejected`、带任务；info 一行带 `job_id`；回滚做完（`tm.endedBackground`）之后数 `mode`，共 2 帧（驳回一帧、做完一帧：回滚不停住时小笔记本几十毫秒就做完，在做完之前数会读到 1 或 2 帧，所以等做完再数；设计审查 P3A-M4，取"在 `endedBackground` 之后数"的那一种）；行是 `rolled_back`、`rejected`、`rollback_complete = true`，提交与报告留着（`notebook_upgrades_rejected_check`），基线复原（发起时的 frontmatter），`checkPages`。审阅中没有报告的也可以驳回：答 200、之后同样做完。
5. **日志**：三种 info 的字段整行断言（3.14）。

**`TestUpgradeTrashReservedTitles`**（`bootstrap/upgrade_trash_test.go`，新，第七份）：一本普通笔记本，根下建 `index`、`log` 两页并各自删掉（两个变更集），再建一页笔记；发起、交、批准（同 `tm.reviewed` 的做法）。之后：

- `revertChangeset` 删掉 `index` 的那个变更集：409 `page.virtual_file`，什么也没写（回收站里仍有它，根下没有）；
- `startRevert` 同一个变更集：409 `page.virtual_file`，没有任务；
- `restoreNode` 回收站里的 `index`：200，名为 `index 2`，在根下；
- `startTrashRestore` 回收站里的 `log`：任务做完，名为 `log 2`；
- 撤销改名（设计审查 P3A-M5）：同一本笔记本根下另有一页 `index.md`，发起之前就在、不删；升级中 alice 的 agent 经 MCP 把它改名为 `Index notes`（闸门 3 才过），再交、批准。之后 `revertChangeset` 那次改名：409 `page.virtual_file`，名字仍是 `Index notes`。同步撤销一次就够，后台撤销由上面恢复的那一格盖住；
- `checkPages`（根下没有保留的键，00 号文档的 LLM Wiki 规则 `:150-152`）。

**`TestUpgradeStatesReachTheStreams`**（`upgrade_start_test.go:388-480`）在现有的一串之后加：发起、交（存下报告，2 帧）、退回（1）、交（2）、驳回（1）、回滚做完（1）、发起、交（2）、批准（1，`From standard`、`To llm_wiki`）。驳回的一步照现有的放弃（`:425-450`）：`stops.arm(nb, 1, 0)` 之后驳回，任务停在第一个计划，数 1 帧；`stops.goOn()` 之后回滚做完，再数 1 帧（设计审查 P3A-M4）。三个流（alice 管理员、bob 读者、carol 看不到）照旧 `[n, n, 0]`。反向对照：组合根交给 `notebook.NewUpgradeStates` 空的订阅者时这些步收不到帧（同现有的步，P1 设计 3.15）。

**成本守卫 `TestUpgradeApprovalTakesItsTimeAtAnyStatistics`**（`bootstrap/upgrade_approve_cost_test.go`，`//go:build !race`，costs-1，第 280 条）：

- **种子**（SQL，05-lessons 第 1 节；照设计时的探针，7.6）：一本普通笔记本，根下 `AGENTS`（正文非空）、`raw` 下 500 个文件夹页（带正文）、每个下面 90 页带正文、5 页空页、4 个附件（`asset_blobs` 的行）：约 4.75 万个要写的哈希、约 6 万个节点；根下另一个文件夹里 1 万页带干净 frontmatter 的笔记。库里另有九本各 5 万页、来源带哈希的 LLM Wiki 与它们各自一次升级的基线行（约 51 万行 `upgrade_nodes`），各另有 2,000 个附件（附件节点与 `asset_blobs` 的行），让 `AttachmentInfos` 丢了键时读到别本的行（决定 240；设计审查 P3B-I1：库里只有这一本的附件时，`asset_blobs` 的顺序扫描本来就最便宜，丢了键看不出来）。之后经 REST 发起与提交（真的路径，报告由提交存下），再以这个库作模板。模板里照发起的守卫关掉这几张表的 autovacuum（表的存储参数随副本复制）。
- **配置**：生产的请求 15 秒（`ApproveTimeout` 11.25 秒），同 `bootstrap/wiki_lint_cost_test.go:27-34` 的写法另配；`wiring.wiki` 里断言 `d.ApproveTimeout == 15 s × 3/4`，守住组合根的 `* 3 / 4`（同发起的守卫断言 `StartTimeout`，`upgrade_start_cost_test.go:114-117`）。
- **统计状态 × 计划**（设计审查 P3B-M2）：三种状态——"从未分析"（在副本上清 `nodes`、`page_contents`、`asset_blobs`、`upgrade_nodes`、`notebook_upgrades` 的统计）；"发起留下的统计"（种子之后 `ANALYZE`，发起自己分析 `upgrade_nodes`；模板原样）；"这本笔记本的页在统计之后才写入"（另一个模板：另九本种好就 `ANALYZE`，再种这一本、经 REST 发起与提交，统计以为它很小：批准真会遇到的过时，一本刚导入的大笔记本）——× `plan_cache_mode` 自定义与通用（05-lessons 第 4 节）。初稿的"只在别的升级的统计下（发起之前分析）"经 REST 发起做不出来（发起在提交之后 `ANALYZE upgrade_nodes`，`start_upgrade.go:178`），删掉。每一格三次批准，每次在模板的一份新副本上（`pgtest.NewDatabaseFrom`，第 280 条），新起一个应用。每份副本上 `ALTER DATABASE <副本> SET plan_cache_mode`：副本不继承模板的库级设置（`CREATE DATABASE … TEMPLATE` 不复制它），新应用的连接以 `SHOW plan_cache_mode` 断言是这一格的方式。新应用先 `awaitJob` 等启动时的周期作业做完（`assets_jobs_test.go:122`），再批准、计时。
- **计划为主**（决定 236、240、244；设计审查 P3B-I1）：经 `wiring.queryTracer`（同 `startCostTrace`，`upgrade_start_cost_test.go:311`）记下批准的事务里的语句与它们的参数，在同一状态、同一计划方式下 `EXPLAIN`（通用计划用 `EXPLAIN (GENERIC_PLAN)`，同 `startCostPlans`，`:441-496`）。照 `startCostPlanned()` 的写法列出被判的语句，不写"每条语句"：
  - **判**：`ForgetSources`、`LintNodeMetadata`、`SourceRebuildHeads`（决定 292）、`AttachmentInfos`、`SetSourceHashes`、`DeleteUpgradeNodes`、`RootNodesByKey`、`FindNodeIn`、`PageContent`。`nodes` 只经首列是 `notebook_id` 的索引或 `nodes_pkey`，`page_contents` 只经 `page_contents_pkey`，`asset_blobs` 只经 `asset_blobs_node_id_key`（`AttachmentInfos` 按 `node_id = ANY(…)` 读，走这个唯一约束，不是主键 `id`，`00027_asset_asset_blobs.sql:10-11`），`upgrade_nodes` 只经 `upgrade_nodes_pkey` 的前缀；都不顺序扫描；嵌套循环内侧逐行重复的扫描以外行的键为条件（不逐行重读整个集合）；`SetSourceHashes` 每块的计划至多读这本笔记本一遍（哈希建在哪一边都可以），或逐行经键（3.4 的"界与分块"：自定义计划把哈希建在这本笔记本上，通用计划逐行经键或把哈希建在这一块上；设计审查 P3B-M1）。
  - **不判**：`notebooks`、`notebook_upgrades`、`workspaces`、`users` 与成员表的点读（`LockNotebook`、`CurrentUpgrade`、`ShareByID`、`named`、判定）：守卫的库里它们只有几行，顺序扫描是对的计划（审查 B 实测分析过的两种状态下 `LockNotebook`、`CurrentUpgrade` 两种计划都是 `Seq Scan`），判它们要另种上千本笔记本，不值。
  - **分块的语句判第一块**（满的一块，一万个）：`SourceRebuildHeads`、`SetSourceHashes` 每次批准跑五、六块，`startCostTrace` 那样按名字存只留最后一块（7,000 或附件那一块），块的大小不同自定义计划可以不同；这个守卫的 tracer 按名字留第一次的参数。
  - 列出的语句在记下的语句里没有出现就失败（照 `startCostPlans`，`upgrade_start_cost_test.go:447-450` 的"the start ran no %s"；设计审查 P3B-N11）。计划有问题就停在计时之前。自定义计划由设计时的探针、通用计划由审查 B 的探针核对过（`asset_blobs` 与小表除外，见上），`SourceRebuildHeads` 的两种由修订时的探针核对过（3.4）。
- **只核对计划的种子**（决定 292）：同上的形状，另几本只有四本（这一本约占 `nodes` 的 22%，审查 B 的变体：`SourceHeads` 在这里换成每块顺序读整张 `page_contents`），三种统计状态 × 两种计划，每格一次批准（在副本上），只判重建的几条语句（`ForgetSources`、`LintNodeMetadata`、`SourceRebuildHeads`、`AttachmentInfos`、`SetSourceHashes`）的计划，同上的断言，不计时（同决定 244 的"原来的形状只核对计划"）。`SourceRebuildHeads` 两种计划都只经键，`page_contents` 不顺序扫描。不判 `DeleteUpgradeNodes`、`RootNodesByKey`、`FindNodeIn`、`PageContent`：这一本占比大时，删基线读走整张 `upgrade_nodes` 是对的计划（设计者的日志里 22% 时分析过的自定义计划与"统计之后才写入"的通用计划都是如此），照上面的清单全判会在对的计划上失败（设计审查 P3B-N8）。三种状态照上面的做法得到，所以这一种子也要两个模板：一个是另四本与这一本都种好再 `ANALYZE`、经 REST 发起与提交（"发起留下的统计"，副本上清统计就是"从未分析"）；另一个是另四本种好就 `ANALYZE`，再种这一本、经 REST 发起与提交（"这本笔记本在统计之后才写入"；设计审查 P3B-N12）。
- **时长为辅**：每格三次的服务端 `elapsed_ms`（`upgrade approved` 一行）的中位数在界之内，每次都答 200；界取本机两轮较大的中位数 × 3（设计时探针里 5 万个来源的重建与删基线本机 0.99–1.19 秒，整个批准估 1.2–1.4 秒，界约 4 秒，远在 11.25 秒之下）；带 `-v` 打出每格的中位数与各段。
- 本机估计约 4–5 分钟（四个模板：计时的两个、只核对计划的两个，种子各约 10–20 秒，发起与提交各约 3 秒，另一个模板约 20–30 秒，审查 B 估；18 次计时的批准与 6 次只核对计划的，各一份副本，副本约 0.5 秒一份，审查 B 实测）。`Makefile` 的这一行显式带 `-timeout 15m`（下面与 7.5；设计审查 P3B-M7）。

### 7.2 交错

05-lessons 第 3 节：持住双方都要的行（`pgtest.WaitForLockWaits`），再放开，先后在共用的日志里断言；每个交错结束时 `checkPages`、`checkLinks`、`checkSearch`。编号照 00 号文档 4.16。

| # | 交错 | 怎样逼出 | 断言 | 在哪里 |
|---|---|---|---|---|
| 7 之退回（一） | 提交的计算在算，退回 | `submitApp` 上 `heldRules` 停住计算（同 `:631` 的第 15 条） | 退回答 409、`still being checked`、没有新的计算；放开之后提交存下报告（`reviewedReported`） | `TestUpgradeSubmitInterleavings` |
| 7 之退回（二） | 第二个事务等着笔记本行时过了接管的时刻，退回 | 另配一个 `submitApp` 那种应用（请求 20 s、lint 10 s、两个名额），只把 `wiring.submitTimeout` 设为 3 s，`Runs.Do` 约 1.5 s 才放弃（照 `TestUpgradeSubmit` 期限那一格，测试持笔记本行，`upgrade_submit_test.go:380-420`）：停住计算，测试持笔记本行，再放开计算。持行要在 `Runs.Do` 放弃之前（期限的一半之内）：`heldRules` 只停住计算，不停 `Do` 的等候，`Do` 一放弃第二个事务就开，那时行没被持住，它自己拿到行、走 `submitFailed` 退回 `upgrading`，dana 的退回就答 409（设计审查 P3A-M7：300 ms 下只剩约 150 ms 减去收到 `held` 的那一段，竞态构建、负载高的 runner 上会偶发）。第二个事务等在笔记本行上（`WaitForLockWaitsOn(notebooks, 1)`）；dana 退回，也等在行上（`WaitForLockWaitsOn(notebooks, 2)`）；测试从行里读 `submitted_at`（`upgrade_submit_test.go:383-386`），过了 `submitted_at + 3 s` 之后放开行。多花约 3 秒 | 第二个事务在等锁时被自己的期限中止，什么也不写；提交答 503，`Retry-After` 照第 274 条（至少 1），记一行 warn；退回拿到行，读到审阅中、没有报告、已过接管的时刻（`ReviewStale`），答 200；行是 `backToUpgrading`。不能靠"停住的计算本身过期限"：`Runs.Do` 只等期限的一半就答 `server_busy`（300 ms 下约 150 ms，审查 A 实测），提交自己退回了 `upgrading`，到期限再退回答 409（设计审查 P3A-I1）。放开行之后第二个事务先拿到行，随即在已关的连接上失败、回滚（pgx 的默认处理只关连接、不发取消，它的后端一直排在锁上），dana 再拿到行 | 同上 |
| 8 之批准 | 检查排队等名额时升级被批准 | 照第 8 条（`upgrade_check_test.go:469`）：两条规则的条目的读占住两个名额，检查的 flight 排队，dana 批准，放开 | 两条读答 200；排队的检查在快照里读到升级已不在，答 409 `wiki.upgrade_state` | `TestUpgradeCheck` |
| 9 | 批准与驳回同时 | `interleaveOn(notebookRow(nb), …)`（`interleavings_workspace_test.go:191`），两种次序 | 先到的答 200；后到的答 409 `wiki.upgrade_state`、什么也没写；先驳回时回滚做完 | `TestUpgradeReviewInterleavings` |
| 9 | 批准与删除笔记本同时 | 同上，两种次序 | 批准先：200，删除 204（`api/modules/notebook.yaml:113`、`:124`），之后笔记本已删、升级的行随之软删除；删除先：批准答 404 `notebook.not_found` | 同上 |
| 新 | 批准与退回同时 | 同上，两种次序 | 后到的答 409 `wiki.upgrade_state` | 同上 |
| 新 | 批准与一个等着笔记本行的写 | bob 先加为这本笔记本的编辑者；测试持笔记本行；bob 经 REST 改 `raw` 下带正文的一页来源的正文（`putPageContent`：冻结与契约都在单元里、锁下判；设计审查 A 核对过锁外只有判定 `Writer.Allowed`，`page/app/unit.go:158-169`，没有冻结的预检；挑带正文的：`sourceImmutable` 对正文为空的来源放行第一次写入，`page/domain/contract.go:161-171`，设计审查 P3A-N4）、dana 批准，都等着；放开 | 次序一：批准先到，答 200；写在它之后读到 LLM Wiki，答 409 `page.source_immutable`；次序二：写先到，答 409 `notebook.upgrading`（审阅中冻结），批准 200 | 同上 |

第 9 条的"批准与驳回"在 00 号文档里写"后一个答 `wiki.upgrade_state`"，"批准与删除笔记本同时：串行"：两个都先取笔记本行，`interleaveOn` 的 FIFO 定次序，不死锁。

### 7.3 变异

照 `tools/mut.py`，每条在 `server/` 或 `web/` 下改一处、跑抓它的测试、期望失败。

| 变异 | 抓它的测试 |
|---|---|
| `ReviewOf` 以 `!at.After` 判期限（边界错一微秒） | `TestReviewOf` 的"恰在期限"一格 |
| `ReviewOf` 先判期限（`at.Before(…)`）再判报告 | `TestReviewOf` 的"有报告而在期限之内"（只对调 `Report != nil` 与 `SubmittedAt == nil` 两支是等价的：有报告时提交一定在，`00057:49-50`；设计审查 P3B-N1） |
| 批准接受 `ReviewStale` | `TestUpgradeApproval` 码的次序"拨回一小时"一格；单元 |
| 批准不看报告的 `Passed` | 单元"报告不通过"一格 |
| 批准不再核闸门 1–3 | `TestUpgradeApproval` 第 4 格；单元三格 |
| 批准时闸门不过不记 error（决定 293） | 单元三格的 error 一行；`TestUpgradeApproval` 第 4 格 |
| 以"外面的变量非空"认闸门不过（回滚的失败答成 409） | 单元的"回调答 409 而回滚失败"一格 |
| `JudgeRoots` 的闸门 2 不比 `AGENTS` | `TestJudgeRoots`；`upgrade_check_test.go:87` |
| 批准用 `UpgradeNotReady`（"upgrading again"） | `TestUpgradeApproval` 第 4 格的 `detail`；`ApprovalNotReady` 的单元 |
| notebook 的 `ApproveUpgrade` 不核 `upgrade_state` | `TestAnUpgradeIsApproved` 的 `upgrading` 一格 |
| 批准不重建哈希 | `TestUpgradeApproval` 第 2 格的 `assertSourceHashesMatchRebuild` 与 `checkPages` |
| 重建不先 `ForgetSources` | `TestTheSourceRebuildIsTheInvariant` 的区外一格 |
| 重建跳过缺文件行的附件 | 同上的缺附件一格；`source_rebuild_test.go` |
| 重建不核每块写的行数 | `source_rebuild_test.go` 的"写少一个" |
| 重建的调用换回 `SourceHeads`（决定 292） | 守卫的两个种子都以"没跑过 `SourceRebuildHeads`"失败（列出的语句没出现就失败，7.1；设计审查 P3B-N11） |
| `SourceRebuildHeads` 名字不变、SQL 改回连接的形状（决定 292） | 守卫的只核对计划的种子（这一本约占 22%，分析过、自定义计划顺序读 `page_contents`）；在一成的种子上抓不到（审查 B 的探针：三种状态都逐个经 `page_contents_pkey`） |
| 重建以 `%v` 包 store 的错误 | `source_rebuild_test.go` 的 `DeadlineExceeded` 一格（批准会把 503 答成 500） |
| 重建给零字节的页写哈希 | `TestTheSourceRebuildIsTheInvariant`（空页一格）；`checkPages` |
| 重建不给零字节的附件写哈希 | 同上（零字节附件一格） |
| 批准不删基线 | `TestUpgradeApproval` 第 2 格；`checkPages` 的升级规则 |
| 批准在重建之前 `Publish` | 单元的调用次序 |
| 批准不 `Publish` | `TestUpgradeStatesReachTheStreams` 的批准一步；`TestUpgradeApproval` 第 3 格 |
| 批准的 SQL 不核 `submission_id` | `TestTheReviewsRows` |
| 批准用请求的 ctx、不另设期限 | `TestUpgradeApproval` 第 6 格（等笔记本行的那一格在请求的 4 秒上答 500） |
| 期限的判断不看 `ctx.Err() == nil` | 单元"请求的 ctx 先到期"一格 |
| 批准的 info 在提交之前记 | 单元的提交失败一格（替身的 `commit` 钩子：回调成功之后提交失败，没有 info；调用的次序里 info 在 `commit` 之后；设计审查 P3A-M3 = P3B-M3 第 1 条：把 info 挪进回调的末尾时，"`Rows.Approve` 答假"一格在 info 之前就返回了，看不见） |
| 驳回的 info 在提交之前记 | `reject_upgrade_test.go` 的提交失败一格 |
| 退回的 info 在提交之前记 | `send_back_upgrade_test.go` 的提交失败一格 |
| 驳回接受 `upgrading` | `TestUpgradeReview` 驳回的码的 `upgrading` 一格（`notebook_upgrades_rejected_check` 会让它 500，而测试要 409） |
| 驳回记 `abandoned` | `TestUpgradeReview` 第 4 格的 `ending`；单元 |
| 退回不判 `ReviewChecking` | `TestUpgradeReview` 退回的码"刚提交"一格；交错 7 之退回（一） |
| 退回的 SQL 不清报告 | `TestTheReviewsRows`（`notebook_upgrades_report_check` 会让写失败，测试要整行） |
| 退回的 `taken_over` 用 `at` 不减 `SubmitTimeout` | `send_back_upgrade_test.go` 的整值一格（`TestTheReviewsRows` 以显式的 `taken_over` 测 SQL，看不见用例传了什么；`TestUpgradeReview` 里 `ReviewOf` 先答 409，到不了 SQL；设计审查 P3B-N2） |
| 退回不 `SetState`（笔记本留在审阅中） | `TestUpgradeReview` 第 2 格：agent 的写仍被冻结 |
| 第 181 条不限 `ChangesetRevert` | `TestUpgradeTrashReservedTitles` 的 `restoreNode` 与 `startTrashRestore` 两格（答 `page.virtual_file` 而不是 200）；`contract_test.go` |
| 去掉第 181 条的改动 | 同一个测试的两个撤销格 |
| `UpgradePlaybook` 在 `reviewing` 也答文字 | `TestUpgradePromptMatrix` 的 `reviewing` 列 |
| `UpgradePlaybook` 不判 `lint.read` | `upgrade_playbook_test.go` 的"看不到答 `notebook.not_found`"一格与调用的次序（替身的判定）。整个程序里看不到的、删掉的笔记本在 HTTP 的边上由 `Gate`（`node.list`，`page/mcp_entry.go:29-31`、`mcpserver/edge.go:232-249`）就答 404，到不了用例（设计审查 P3B-M3 第 2 条 = P3C-N6） |
| `upgrade` 的 prompt 经 `Playbooks.Execute`（`PlaybookText` 收进它） | `TestUpgradePromptMatrix` 的 `llm_wiki` 列（答文字而期望 `wiki.upgrade_state`）与 `upgrading` 列（答 `notebook.not_llm_wiki` 而期望文字）。只给 `PlaybookText` 的 `switch` 加一支而没有调用者是等价变异（设计审查 P3B-N3） |
| `HealthRule` 在 `cursor === undefined` 时不渲染"更多" | `upgrade-check.test.tsx` 的"更多从第一页读" |
| 审阅一节的读不经 `itemsReader`（403 不重读列表） | `upgrade-check.test.tsx` 审阅一节的 403 一格 |
| 审阅一节留在这一次进入的 `SWRConfig` 之外 | `upgrade-check.test.tsx` 的"离开再进入从存下的报告重新开始"（审阅一节的键不带这一次进入的 `nonce`，留在外面就接着旧的条目，4.3） |
| 没有报告时也显示"批准" | `upgrade-page.test.tsx` 的按钮一格 |
| 审阅中仍显示"放弃"（决定 291） | `upgrade-page.test.tsx` 的 `reviewing` 一行（`:175-196`）与 `sectionsInOrder` 的审阅一行（`:856-867`） |
| `reviewing` 的横幅仍写"放弃"（决定 291） | `upgrade-banner.test.tsx` 的两格（`:49-51`、`:127-129`） |
| 批准用 `upgrade.notReady` | `upgrade-page.test.tsx` 的"批准答 `wiki.upgrade_not_ready`"一格（设计审查 P3C-M1） |
| 批准成功之后又读升级（经 `change`） | `upgrade-page.test.tsx` 的"不再读升级"一格（设计审查 P3C-M2）；UP7、UP8 的页面版本 |
| 退回的说明按打开对话框时的状态定 | `upgrade-page.test.tsx` 的"重读之后已不在审阅中"一格（设计审查 P3C-N14） |
| 退回之后不清 `checked` | `upgrade-page.test.tsx` 的"退回之后检查一节没有提交之前的结果"一格（设计审查 P3C-N15） |
| 退回的 409 显示服务端的 `detail` | `upgrade-page.test.tsx` 的 `sendBackChecking` 一格 |
| 不设 20 秒的再读、或卸下之后不清 | 同上（假的计时器两格） |
| 批准之后不跳转、或不交 `arrived` | `upgrade-page.test.tsx` 的焦点一格 |
| 假服务端的退回在没有报告时答 200 | `upgrade-page.test.tsx` 的 409 一格（替身要在真东西会失败的地方失败） |

等价的变异（改了而行为不变的）照 05-lessons 的"等价变异的对照"写明理由，记在 10.x，不算漏网。

### 7.4 门禁

每个 Step 合并之前：`make lint`、`make gen-check`、`make test-go-short`；本机经 `heavy.sh` 跑它碰到的组合根的份（S1：第三、四、六、七、八份（第三份有覆盖的检查 `TestThePermissionMatrixCoversEveryOperation`）；S2：第三、七份；S3 不碰服务端）与 costs-1 的新一行两轮；`make test-web`；改动的 e2e 故事（`pnpm --filter e2e exec playwright test stories/upgrade/up6-… …`，页面版本在三种浏览器的那一支）；`tools/mut.py` 跑 7.3 里这个 Step 的变异。之后分支的 CI 19 项全绿。

### 7.5 CI 的余量与分份

**起点**（7763 上的 Test 一步与 `ok … bootstrap` 一行的包时间；7.6 的本机两轮）。比用包对包：Test 一步比包多一段约 32 秒的竞态构建（run 43027580603 里第四份 445 − 412.8、第六份 327 − 294.8、第八份 289 − 255.7，都是 32–33 秒），挪测试只改包时间（设计审查 P3B-M5；初稿以 Test 一步对本机的包算比，第四份 1.972、第六份 1.534、第八份 1.734，偏大）：

| 份 | 7763 上的 Test 一步 | 7763 上的包（`ok` 一行） | 本机两轮的包（秒） | 包/本机 |
|---|---|---|---|---|
| 第四份（124 个） | 445（`5d3a0f35` 的 `main`）、440（同一棵树的分支），均值 442.5 | 412.8、407.3，均值 410.1 | 220.44、228.31，均值 224.37 | 1.828 |
| 第六份（11 个） | 327（`5d3a0f35` 的 `main`） | 294.8 | 212.00、214.43，均值 213.21 | 1.383 |
| 第七份（27 个） | 没有 7763 的样本：9V74 265、8573C 292、9V45 203 | 没有 7763 的样本：9V74 239.6；照第八份的包在 9V74 与 7763 上的比（255.7/215.1 = 1.189）估约 285，加构建约 317 | 181.90、182.10，均值 182.00 | 约 1.565（估） |
| 第八份（9 个） | 289（`5d3a0f35` 的 `main`） | 255.7 | 167.25、166.24，均值 166.74 | 1.533 |

**第四份挪走的四个**（第 278 条；本机两轮，秒）：

| 测试 | 第 1 轮 | 第 2 轮 | 均值 |
|---|---|---|---|
| `TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches`（`revert_job_test.go`） | 8.66 | 8.73 | 8.70 |
| `TestRevertingTheNewestChangesetsPutsTheNotebookBack`（`revert_property_test.go`） | 7.44 | 7.56 | 7.50 |
| `TestTheBackgroundRevertOfASegmentPastABatchStops`（`revert_job_test.go`） | 6.94 | 6.61 | 6.78 |
| `TestSoleAdminDeactivatingWhileOneJoins`（`interleavings_deactivation_test.go`） | 6.55 | 6.89 | 6.72 |
| 合计 | | | 29.70 |

折到 7763：29.70 × [1.383, 1.828] = 41.1–54.3 秒（在第四份里照它的比、在第六份里照第六份的比，取两端）。审查 B 在新模式下实测（`p3-review/b/shards/`）：第六份两轮 241.07、239.79 秒（挪前 212.0、214.4），四个在第六份里 29.3 秒、与在第四份里相同，第一个跑的仍是 `TestIngestCleanup`；第四份一轮 192.29 秒（挪前 220.4、228.3）。

第四份里更大的还有 `TestRevertingAndPurgingTheChangeset`（5.33、7.71，两轮跳），不挪它（P2 的分份旁支挑稳定的那个）。

**P3 加的**（本机的估计，设计时没有代码可量；实施时照 7.6 量过替换）：第八份 `TestUpgradeApproval`（两本 `tm.reviewed`、两个另配的应用、三格期限）、`TestUpgradeReview`（两次驳回都等回滚做完、一次退回之后再交）、`TestUpgradeReviewInterleavings`（四个交错各两种次序，每种一本发起并提交过的笔记本）与第 7、8 条的三格，对照已有的 `TestUpgradeSubmit` 20.6 秒、`TestUpgradeRollback` 14.8 秒、`TestUpgradeInterleavings` 7.3 秒，合约 35–50 秒（设计审查 P3B-M5；初稿估 26 秒偏低），加"7 之退回（二）"改用 3 s 的期限多出的约 3 秒（设计审查 P3A-M7），合约 38–53 秒；第七份 `TestUpgradeTrashReservedTitles` 约 5 秒（多了撤销改名的一格）、`TestUpgradePromptMatrix` 约 4 秒（208 格、只读、每种状态一份副本）、`TestUpgradePrompt` 约 2 秒、矩阵三行与流的九步约 4 秒，合约 15 秒。契约每长一截各份慢一截（P2S2B-Q2）：P3 的契约约长 2%（三个操作与两处码），照 P2/S2 的"长 6.8%、慢约 5%"估各份约慢 1.5%。

**预计**（7763 的 Test 一步）：

| 份 | 之前 | 之后 |
|---|---|---|
| 第四份 | 442.5 | 442.5 − [41.1, 54.3] = 388–401，加契约 1.5% 约 394–407 |
| 第六份 | 327 | 327 + [41.1, 54.3] = 368–381，约 374–387 |
| 第七份 | 约 317 | 317 + 15 × 1.565 ≈ 340，约 345 |
| 第八份 | 289 | 289 + [38, 53] × 1.533 = 347–370，加契约约 350–375 |

都在 450 之下，最贴近的约 407（第四份，余 43 秒）；挪四个的结论不变，初稿附注里"只挪三个"的退路删掉。审查 B 以本机实测折回（包的比再加约 32 秒的构建）：第四份 192.3 × 1.828 + 32 ≈ 384，第六份 240.4 × 1.383 + 32 ≈ 364，都在上面的区间之内或之下。全放进第七份时约 317 + (15 + [38, 53]) × 1.565 = 400–423，再加契约约 405–430；分摊之后两份都在 345–375，给 M11 收尾与 M12 留的余量更多（第 290 条）。

**要不要把契约在进程里只读一次**（P2 第 9 节：每建一个应用 `apitest.Load` 一次，竞态构建下约 0.63 秒）：P3 不做。它是测试设施的取舍，牵动全部组合根的测试；这一轮挪测试已够，留给 M12 的测试设施（第 9 节）。

**成本的作业**：costs-1 加批准的守卫：本机约 4–5 分钟（四个模板，7.1；7763 上约 7–9 分钟），另加这个作业里第一次编译不带竞态的 `bootstrap` 测试（costs-1 今天各行都是 platform 与 linking 的；costs-2、costs-3 已有这次编译），7763 上约 1 分钟（设计审查 P3B-N7）。基数取 S3 的三次运行（修订核对第二轮 B 以 `gh run view` 取）：

| 运行 | CPU | 作业 | Test 一步 |
|---|---|---|---|
| 43033276772 | EPYC 9V74 | 14:31 | 11:18 |
| 43027580603 | Xeon 8573C | 15:06 | 11:50 |
| 43022417961 | Xeon 8370C | 15:41 | 12:18 |

作业从开始到 Test 一步约 3:10–3:19。P3 之后作业约 23–26 分钟。正常跑一遍这一行就用掉 Go 默认的 10 分钟超时的七到九成，慢的 runner 上会红在 Go 的超时上而不是守卫自己的界上，所以这一行显式带 `-timeout 15m`（`Makefile:433-436` 回滚先删那一行的 `-timeout 20m`、`:471-478` 守卫的 `-timeout 15m` 是先例；"从不提高 `-timeout`"是组合根分份的规矩，不管成本的行；设计审查 P3B-M7）。作业时限照两条判（修订核对第二轮 P3B-M8，第 294 条）：

- **约两倍**（`ci.yml:38`、`:85`：runner 的 CPU 可差两倍，各份的时限约为 CI 上实测的两倍）：作业约 23–26 分钟，两倍是 46–52 分钟；今天的 35 分钟只有约 1.35–1.5 倍，比它慢 1.4–1.5 倍的 runner 会在第 35 分钟被整个取消、Go 来不及打出卡住的测试（`ci.yml:81` 记的 42093077490 那一种）。照决定 267（costs-2 35 → 45）的先例，costs-1 的时限从 35 分钟提到 45 分钟。
- **Go 的超时先到**：这一行排在 costs-1 的最后，约在作业的第 15–17 分钟开始（审查 B 的估计，含作业前面的几步与这次编译），加 15 分钟是第 30–32 分钟，早于 45 分钟。

（第一轮修订照"开始的时刻加 Go 的时限早于 35 分钟"写了"作业时限不改"，漏了约两倍的那一条。）

**e2e 的作业**（设计审查 P3C-N16）：P3 加 UP6–UP9，UP7 的整个文件（页面、API、MCP 三个版本）在三种浏览器里跑，约多 12–14 个用例、本机约多 40–60 秒；P2/S3 的分支 CI 的 e2e 作业 8:42（E2E 一步 7:09），估 P3 之后约 10–11 分钟，作业限 20 分钟（`ci.yml:255`），不改。实测写进 10.3。

### 7.6 P3 怎样量

- **设计时**（这一份初稿）：临时 worktree（`955fa579`），经 `heavy.sh`，`df` 28G。
  - 组合根第四、六、七、八份：`make -j1 test-go-bootstrap-<n> GO_TEST_RACE="go test -race -count=1 -timeout 15m -v"`，每份两轮，P2 的分份旁支的 `measure.sh` 与 `times.py`；数在 7.5。
  - 批准的重建：一个只在临时 worktree 里的探针测试（`page/adapter/postgres`，真库）：上面的种子在 10 万个来源（1,000 个文件夹）与 5 万个来源（500 个）各跑；两种统计（从未分析；分析过）× 两种方式（`plan_cache_mode` 的 `force_custom_plan` 与 `force_generic_plan`），每格三次，在一个回滚的事务里依次计时 `ForgetSources`、`SourceNodes`、十块 `SourceHeads`、附件的读、十块 `SetSourceHashes`、`DeleteUpgradeNodes`，并 `EXPLAIN` 每一条。结果：10 万个来源合计 2.44–3.68 秒（两轮），5 万个来源 0.99–1.19 秒（两轮，几乎相同）；`SetSourceHashes` 的几种写法都省不下（3.4）。计划（只是自定义计划：带参数经扩展协议发的 `EXPLAIN` 把参数当常量折进计划，`plan_cache_mode` 管不到它，探针的 `force_generic_plan` 一栏其实也是自定义计划；通用计划由审查 B 以 `EXPLAIN (GENERIC_PLAN)` 补量，`nodes`、`page_contents`、`upgrade_nodes` 都经键，见设计审查记录与 3.4 的"界与分块"；设计审查 P3B-M1）：5 万个来源、每格之前 `VACUUM ANALYZE` 的两轮里没有顺序扫描，`nodes` 经 `nodes_notebook_id_*` 的索引与 `nodes_pkey`，`ForgetSources` 在分析过之后经 `nodes_notebook_id_kind_source_hash_idx`；10 万个来源的第二轮在同一个库里反复回滚、关着 autovacuum 之后再 `ANALYZE`，`ForgetSources` 与 `LintNodeMetadata` 换成了整表的顺序扫描（这本笔记本占 `nodes` 的约 20%：110,003 / 560,003，设计审查 P3B-N10 改正初稿的"约 16%"；表里堆着几百万条死行），所以守卫每次批准用一份新副本（第 280 条）。
- **修订时**（设计审查之后）：临时 worktree（`955fa579`），经 `heavy.sh`，`df` 27–28G；以审查 B 的探针为底，比决定 292 的读头的四种写法在两个种子（这一本占 `nodes` 11.4% 与 22.4%）× 三种统计状态 × 自定义与通用（`EXPLAIN (GENERIC_PLAN)`）上的计划，只看计划、不计时；结果在 3.4。
- **实施时**：S1 的第 1 个提交照 P2 的分份旁支量挪前挪后（第四、六份各两轮），写进 10.1；守卫照 7.1 量两轮定界；P3 的新测试在第七、八份的实际用时在 S1、S2 的记录里。

## 8. 完成标准

P3 完成，当且仅当：

1. 三个 Step 各自合并进 `main`，合并之前 6.4 的门禁全绿、审查与修复核对结束；每个 Step 合并之后两次 `main` 的 CI 里组合根各份的 Test 一步的平均都在 450 秒之下（7.5），否则照 06-environment 第 50 行的规则处理之后才开下一个 Step。
2. 第 3–5 节的每个测试在，7.3 的每个变异被它旁边的测试抓到（第 10 节记下），3.22 的每个反向对照在测试里。
3. 第 2 节的移交的落实做完；四份移交（`handoffs/M10-mode.md`、`handoffs/M9-mcp-upgrade.md`、`handoffs/M5-events.md`、`handoffs/M8-P4-background.md`）的"处理进展"各加一行（每个 Step 的文档提交写它做了的项：S1 写 `M10-mode.md` 的第 2、4 项、P2 补充的哈希重建与收尾补充的回收站，`M5-events.md` 一行（批准、驳回、退回经模式变化的订阅链发 `mode`，`TestUpgradeStatesReachTheStreams` 加的步：退回、驳回与它的回滚做完、批准），`M8-P4-background.md` 一行（第 5 条的驳回一半：驳回经 REST，回滚任务的客户端是 `web` 或 `api`，3.7 的 `ClientOf`）；S2 写 `M10-mode.md` 收尾补充的已有的笔记库与 upgrade 的 prompt、`M9-mcp-upgrade.md` 第 1 项末句；S3 在 `M10-mode.md` 加网页的一行，`M5-events.md` 加网页的一行（批准之后跳到笔记本首页、升级页卸下，之后的 `mode` 不再读升级）），状态仍是 `open`。`M8-P4-background.md` 的第 1–4 条在 P1 已落实或不适用（它的"处理进展"写着），P3 只补第 5 条的驳回（设计审查 P3C-M10）。
4. **M11 收尾要的东西里在 P3 做的**：四份移交的每一项都已落实或写明不适用（P3 之后"处理进展"里没有未做的项）；00 号文档第 3 节的完成标准里与实现有关的全部做完（P3 之后只剩收尾的审查与文档）；README 的"升级为 LLM Wiki"一节与人工清单的 K 在；v0.1 总体设计 13.x 里 M11 的每一处都写到 P3。**不在 P3 做**：四份移交改为 `done`、00 号文档的状态改为完成、v0.1 总体设计 12.6 的 M11 一行改为"实现完成，待人工验收"、M11 的收尾审查（第 9 节）。
5. **00 号文档**（`docs/v0.1/M11-upgrade/00-M11-design.md`）：
   - **接受本文的文档提交**：以 `p3-apply-00.py`（`--check` 与 `--path`，每处一个带标签的替换，一处不匹配就什么也不写）改：2.1 的十二条照各条写的新文字（第 1 条连带第 5 节 `approveUpgrade` 一行加 `server_busy`、4.9 加"批准有自己的期限"一句、4.16 码的次序加批准的 503、4.18 加 warn 一行；第 2 条连带第 9 节成本守卫表"批准"一行与"写给后面的 M"的 M12 性能一条；第 4 条连带 4.9 第 2 步与 4.18 的 error 一行（决定 293）；第 6 条在第 5 节 MCP 表的 prompt 一行；第 7 条在 4.14；第 8 条在 4.15；第 9 条在第 9 节的测试清单；第 10 条在 4.15 的两处、4.3 的一句与决定 246 的条目；第 11 条在 4.20；第 12 条在第 10 节的风险表）；2.3 的十七条（278–294）写进第 11 节，另起一节"<接受本文的日期>：P3 的设计审查之后的取舍 278–…（主持者的决定，负责人可以改判）"，每条一句、带它在本文与审查记录里的位置，接在"2026-11-11：P2/S3 的取舍 277"一节之后；头的"状态"改为"进行中（P2 完成，P3 设计已审查、S1 开工；…）"；第 12 节的 P3 一行（`:917`）`| P3 审阅、批准与 playbook（S1–S3） | — | 未开始 | — | — |` 改为 `| P3 审阅、批准与 playbook（S1–S3） | [03-P3](03-P3-review-approve-playbook.md) | 进行中（设计已审查） | [设计](reviews/P3-design-review.md) | — |`；第 13 节加一行。
   - **每个 Step 的文档提交**（在 Step 的分支上、合并之前）：第 12 节的 P3 一行的"状态"写到这个 Step、"审查"一格加它的记录 `reviews/P3-S<m>-<slug>-review.md`，"合并"一格合并之后在 `main` 的小文档提交里填入；第 13 节加一行；本文第 3 节加"3.23 S<m> 的实施对设计的修正"、第 10 节填这个 Step。
   - **`docs: M11/P3 done (M11/P3)`**：第 12 节的 P3 一行"完成"；头的"状态"改为"进行中（P3 完成，收尾中）"；第 13 节加"P3 完成"一行。
6. **v0.1 总体设计**（`docs/v0.1/v0.1-design.md`；00 号文档第 7 节"总体设计的修订"里属于 P3 的部分，各在实现它的 Step 的文档提交里改，用脚本断言锚点恰好出现一次；行号照 `955fa579`）：
   - 5.8（`:557`，第 1 步 `:575`）与 5.9（`:606-614`）：核对 README 的升级一节与 prompt `upgrade` 的说法一致（`:575` 已写"照 README 把 agent 连到这个笔记本的 MCP 并运行 prompt upgrade"），S2 不改，若措辞不合在 S2 的文档提交里改。
   - 13.1 第 1 条（`:1049`）末尾加——S1："审阅的三个动作与批准在笔记本行的锁下写；批准在同一个事务里写模式与来源根、清升级的四列、重建来源的哈希、删基线，之后通知模式的订阅者（M11/P3）。"
   - 13.1 第 4 条（`:1071`）末尾加——S1："批准的码：404 → 403 → 状态的 409 → 闸门的 409 → 503（自己的期限，在任何一步上）；退回：404 → 403 → 状态的 409（提交还在算时同码、说明不同）；驳回：404 → 403 → 状态的 409 → 503（任务的队列满）（M11/P3）。"
   - 13.1 第 5 条（`:1073`）末尾加——S1："批准的事务另有期限（请求的四分之三），到期答 503、什么也不写；它在锁下重建全部来源的哈希，不分批提交（M11/P3）。"
   - 13.1 第 8 条（`:1100`）末尾加——S1："`revertChangeset`、`startRevert` 声明 `page.virtual_file`：撤销里的恢复回到 LLM Wiki 根下保留的标题时拒绝；回收站与后台的恢复照旧加序号（M11/P3）。"
   - 13.1 第 10 条（`:1103`）末尾加——S1："批准、驳回、退回各一行 info，在提交之后；批准的期限到期一行 warn；批准时锁下再核闸门不过（缺陷）答 409 的同时一行 error（M11/P3）。"
   - 13.1 第 21 条（`:1127`）末尾加——S1："批准的哈希重建 `page.NewSourceRebuild(pool, attachments)` 另带附件的元数据（组合根交单元的那一个），wiki 经 `Deps.SourceHashes` 调它（M11/P3）。"S2：在"MCP 工具与 prompt 的注册表"一条（`:1135`）末尾加"M11 的 prompt `upgrade` 登记在 wiki 的 `Prompts()`、只在笔记本入口，取它只在升级中答文字（M11/P3）。"
   - 13.1 第 31 条（`:1164`）末尾加——S2："playbook 至多 16 KiB，升级的那份由单元测试量（M11/P3）。"
   - 13.2 第 9 条（`:1199`）末尾加——S3："UP6 的页面版本一个刻意的 404（流以 `holdStream` 挡住、回滚排队到页面显示任务之后）、UP7 的一个刻意的 409，以 `pageWatch.expectConsole` 声明；批准的那一页的事件流以 `holdStream` 推迟到跳转之后，不留先后不定的 404（M11/P3）。"第 18 条（`:1209`）末尾加——S3："审阅一节读存下的报告，'更多'从第一页读条目，缓存按这一次进入；批准之后去笔记本首页（M11/P3）。"
   - 13.4 第 3 条（`:1264`）末尾加——S1："M11/P3：UP6、UP7 的 API 与 MCP 版本、UP8 的 API 版本；UP6 的'驳回之后 MCP 写被拒'以另一本笔记本的导入占住导入 worker、回滚排队时确定地断言。"S2："UP9 只有 MCP 版本。"S3："UP7 的页面版本在三种浏览器里跑。"
   - 13.4 第 4 条（`:1265`）末尾加——S1："M11/P3：提交在算时的退回与第二个事务等着笔记本行时过了接管时刻的退回、排队中的检查与批准、批准与驳回、批准与退回、批准与删除笔记本、批准与等着笔记本行的写，各一个确定的交错。"
   - 13.4 第 6 条（`:1273`）末尾加——S1："M11/P3：批准、驳回、退回的行在升级的变体上；批准的管理员一格答状态的 409（变体都没有存下的报告）。"S2："prompt 的矩阵按'prompt × 状态'，在第七份的 `TestUpgradePromptMatrix`。"
   - 13.4 第 8 条（`:1275`）末尾加——S1："批准在 5 万个来源上，三种统计 × 两种计划，每次在库的一份新副本上；计划为主，被判的语句列出来；另有一个这本笔记本约占四分之一、只核对计划的种子（十万个来源在 M12）（M11/P3）。"
   - 第 14 节风险表"大笔记本升级时的冻结时长"一行（`:1318`）——S1："发起、检查、批准与回滚在 10 万页上有成本守卫，30 万页由 M12 实测" → "发起、检查与批准在 5 万页（批准是 5 万个来源）上、回滚的合成与先删在 10 万上有成本守卫；10 万页与 30 万页由 M12 实测"（发起、检查早已是 5 万，第 232、262 条；设计审查 P3C-M10）。
   - 第 15 节：每个 Step 的文档提交加一行（日期、改了上面哪几条、依据是本文与 Step 的审查记录、`M11/P3/S<m>`）。
7. `Makefile` 与 `ci.yml` 的分份照 6.1 第 1 个提交；`06-environment.md` 第 50 行的"八份"不变（没有开第九份）。
8. README 的"升级为 LLM Wiki"一节与人工清单的 K 照 3.19、3.20 在。

## 9. 交给后面

- **M11 收尾**（00 号文档第 7 节"写给后面的 M"之前的收尾）：
  - 四份移交（`M5-events`、`M8-P4-background`、`M9-mcp-upgrade`、`M10-mode`，都在 `handoffs/`）的状态改为 `done`，"处理进展"写收尾的那一行（docs/README：一个 M 完成前不能留有 `open` 的 handoff；设计审查 P3C-M10）；
  - 决定 291（网页上审阅中由"驳回"代替"放弃"）碰到负责人第 1 条的原话，主持者在 M11 的最后报告里单独列出；
  - 00 号文档的状态改为完成，第 12 节加收尾一行；v0.1 总体设计 12.6 的 M11 一行改为"实现完成，待人工验收"（00 号文档第 7 节"总体设计的修订"第 27 条）；
  - 收尾的审查：整个升级的流程走一遍（发起、整理、检查、提交、审阅的三个动作、放弃与回滚、结束回滚），核对 00 号文档第 3 节的每一条；人工清单的 J、K 在 v0.1 的最后执行（00 号文档 4.20），收尾不等它；
  - 要盯着的：P3 合并之后各份的数（7.5 的预计），第四、六份是否照估计。
- **M12 的性能**（追加到 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)，接在 M11 的那几条之后）：批准在 10 万个来源上本机 2.4–3.7 秒（7.6），7763 约是两倍，生产的 11.25 秒之内、默认 4 秒的请求之外；大头是约 9.5 万行的非 HOT 的 `UPDATE`；`SetSourceHashes` 经 `Planned`，生产里是自定义计划，每块以这本笔记本的键读一遍、建一次哈希表，块数 × 笔记本的节点数在 30 万个来源上约 30 × 33 万（设计审查 P3B-M1）；`page/app/source_precheck.go` 分块调 `SourceHeads`，这本笔记本占得多时（约两成以上、分析过）有同样的计划形状：每块顺序读整张 `page_contents`（审查 B 实测 23%、38%）。P3 只给批准的重建另写了按键的语句（决定 292），预检没有量，M12 实测；批准把这本笔记本数万个节点的 `source_hash` 从空写成有值而不 `ANALYZE nodes`，库里有别的大笔记本时可能不到自动分析的阈值（审查 B 的未核实的怀疑），一并看；这本笔记本占 `nodes` 的大头、表里有大量死行时，`ForgetSources` 与 `LintNodeMetadata` 的计划会换成整表的顺序扫描（7.6 的第二轮）。守卫的十万个来源进这里（第 280 条）。
- **M12 的测试设施**：组合根里每建一个应用就 `apitest.Load` 一次，契约每长一截各份慢一截；要不要在进程里只读一次（P2 第 9 节、7.5）。
- **M12 的打磨**（追加到 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)）：退回附说明（第 183 条，已在列）；已结束的升级的报告（已在列）。审阅中同时有"驳回"与"放弃"的问题由决定 291 解决（网页收起审阅中的"放弃"），不进打磨。

## 10. 结果

（空，S1–S3 合并之后填。）

## 附录：核对过的代码

行号都是 `955fa579` 的。

- **wiki**：`wiki/app/start_upgrade.go:27-51`（`UpgradeDeps`）、`:58-74`（`locked`）、`:147-187`（发起的期限与日志）；`wiki/app/abandon_upgrade.go:47-98`、`:106-128`、`:135-158`、`:163-172`、`:175-189`；`wiki/app/submit_upgrade.go:84-151`、`:158-161`、`:164-209`；`wiki/app/end_upgrade_rollback.go`；`wiki/app/upgrade_ports.go:15-175`；`wiki/app/playbooks.go:14-46`；`wiki/app/lint.go:137-150`；`wiki/domain/upgrade.go:18-29`；`wiki/domain/upgrade_check.go:20-112`、`:149-260`；`wiki/domain/upgrade_errors.go:36-81`；`wiki/domain/playbook.go:12-49`；`wiki/domain/playbook-lint.md`、`playbook-ingest.md`；`wiki/adapter/postgres/queries/upgrades.sql:1-89`；`wiki/adapter/postgres/store.go:37-178`；`wiki/adapter/http/upgrade.go:15-130`、`handler.go:15-60`、`upgrade_test.go:25`、`:98`、`:138-143`、`:139`、`:187-213`；`wiki/adapter/mcp/prompts.go:1-56`；`wiki/lint.go:108-200`；`wiki/prompts.go:9`。
- **notebook**：`notebook/upgrade_states.go:30-125`；`notebook/adapter/postgres/queries/notebooks.sql:40-48`、`:104-123`。
- **page**：`page/source_area.go:1-45`；`page/app/source_area.go:12-59`；`page/app/lint_metadata.go:24-35`；`page/app/ports.go:596-616`；`page/app/unit_sources.go:40-110`；`page/adapter/postgres/source_area.go:15-37`、`:70-100`；`page/adapter/postgres/lint_metadata.go:13-58`；`page/adapter/postgres/queries/sources.sql:1-63`；`page/upgrades.go:116-158`；`page/domain/contract.go:110-160`；`page/module.go:85-88`、`:213`。
- **迁移**：`00048_notebook_notebooks_mode.sql:10-12`；`00055_notebook_notebooks_upgrade.sql:1-16`；`00057_wiki_notebook_upgrades.sql:1-63`。
- **契约**：`api/modules/wiki.yaml:160-228`；`api/modules/page.yaml:1062-1106`；`api/modules/transfer.yaml:258-300`；`api/mcp/prompts.json`；`api/mcp/tools.json`（没有撤销的工具）。
- **组合根**：`bootstrap/wire.go:55-95`、`:205-296`；`bootstrap/wiki_upgrade.go`；`bootstrap/assets.go:136-144`；`bootstrap/deps.go:165-178`；`bootstrap/upgrade_start_test.go:195-298`；`bootstrap/upgrade_submit_test.go:25-135`、`:225-262`、`:600-660`；`bootstrap/upgrade_check_test.go:462-500`；`bootstrap/upgrade_fixtures_test.go`、`upgrade_permission_matrix_test.go`、`permission_matrix_mcp_wiki_test.go`、`mcp_prompts_test.go:140-175`、`interleavings_page_test.go:50-56`（经子 agent 的核对，`work/bootstrap-survey.md`）；`Makefile:247-348`、`:360-440`；`platform/httpserver/mcpserver/prompt.go:75-112`。
- **网页**：`W/pages/notebook/upgrade-page.tsx`、`upgrade-check.tsx`、`health-rule.tsx`、`health-page.tsx`；`W/services/upgrade.service.ts`；`W/stores/notebook.store.ts`；`W/test/upgrade-server.ts`；`W/app/problem-messages.test.ts:10-64`（经子 agent 的核对，`work/web-survey.md`，P2 第 9 节的七条都在原行）。
- **e2e**：`e2e/fixtures/browser.ts:40-160`；`e2e/fixtures/events.ts:200-225`；`e2e/stories/upgrade/up3-abandon-rollback.spec.ts:236-275`；`e2e/stories/upgrade/up5-check-submit.spec.ts:312`、`:321`；`e2e/playwright.config.ts:3`。
- **文档**：`README.md:212-260`；`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md:8`、`:288-323`；`docs/v0.1/M11-upgrade/handoffs/M10-mode.md`、`M9-mcp-upgrade.md`；`docs/v0.1/v0.1-design.md:1049-1275`；`docs/v0.1/claude_to_codex/06-environment.md:27`、`:50`。
- **修订时另核对的**（设计审查第一轮之后）：`page/adapter/postgres/queries/sources.sql:1-7`（`SourceHeads`）、`page/adapter/postgres/source_area.go:19-36`；`page/app/source_precheck.go:65`；`wiki/app/check_upgrade.go:222-236`；`wiki/domain/upgrade_errors.go:70-81`；`wiki/app/submit_upgrade.go:144-145`（`failed_gates`）；`api/modules/wiki.yaml:86-358`（升级的路径）、`:322-332`；`api/mcp/tools.json:589`、`:1298`；`W/pages/notebook/upgrade-banner.tsx:36-48`、`upgrade-banner.test.tsx:23`、`:30-58`、`:116-130`；`W/pages/notebook/upgrade-page.test.tsx:122`、`:175-196`、`:840-870`；`W/pages/notebook/upgrade-check.tsx:88-118`、`:160-172`、`:214-228`、`:380-388`；`W/pages/notebook/health-rule.tsx:104-140`；`W/pages/notebook/health-item.tsx:60-68`；`W/pages/notebook/upgrade-page.tsx:115`、`:139`、`:257`；`W/i18n/messages/en.ts:677-679`、`:700`、`:724-725`，`zh-CN.ts:650-656`、`:675`、`:697-702`、`:761`；`e2e/fixtures/upgrades.ts:40-56`、`:95-120`；`e2e/stories/upgrade/up3-abandon-rollback.spec.ts:140-195`；`e2e/stories/upgrade/up5-check-submit.spec.ts:300-325`；`e2e/stories/wiki/lw11-playbook.spec.ts:8-12`、`:60-64`；`README.md:212-248`、`:299`、`:319`；`docs/v0.1/M11-upgrade/handoffs/M5-events.md`、`M8-P4-background.md`；`docs/v0.1/v0.1-design.md:1316-1319`；`docs/v0.1/M11-upgrade/00-M11-design.md:122`、`:351`、`:361`、`:413-421`、`:428`、`:723`、`:842`。
