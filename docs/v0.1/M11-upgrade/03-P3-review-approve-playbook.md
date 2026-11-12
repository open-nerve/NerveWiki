# M11/P3 审阅、批准与 playbook：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M11/P3 审阅、批准与 playbook（三个 Step：S1 审阅（服务端）；S2 playbook 与文档（服务端）；S3 网页） |
| 日期 | 2026-11-11（初稿）；2026-11-12（照设计审查第一轮修订；同日照修订核对第一、二轮再修订，加第 294 条）；2026-11-12（照 S1 的实施与审查修订）；2026-11-12（照 S2 的实施与审查修订）；2026-11-12（照 S3 的实施与审查修订） |
| 状态 | 进行中（设计已审查，S1 已合并 `daf14dd1`，S2 已合并 `7d2e895a`，S3 已合并 `07abd0f7`；设计的[审查记录](reviews/P3-design-review.md)，三轮修订核对在它的末三节；S1、S2、S3 的审查记录 [P3-S1-review-actions-review.md](reviews/P3-S1-review-actions-review.md)、[P3-S2-playbook-docs-review.md](reviews/P3-S2-playbook-docs-review.md)、[P3-S3-web-review.md](reviews/P3-S3-web-review.md)；实施与审查之后与代码不同之处照实改在各节（3.23、3.24、3.25 列出），结果在 10.1、10.2、10.3） |
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
| `api/modules/wiki.yaml`、`page.yaml`、`transfer.yaml`；`api/openapi.yaml`（打包的根，加三个路径；初稿漏了，S1 的实施，3.23）；`api/dist/openapi.yaml`；各模块的 `adapter/http/gen` | 三个操作；两个撤销的 `page.virtual_file`（3.11） |
| `bootstrap/wiki_upgrade.go`、`bootstrap/wire.go` | `wikiUpgradeStates.Approve`；`SourceHashes`、`ApproveTimeout` 的接线（3.12） |
| `Makefile`、`.github/workflows/ci.yml` | 第四份挪四个测试到第六份、第八份的模式扩大（第 1 个提交，第 278、290 条，7.5）；costs-1 加批准的守卫一行（7.1），作业时限 35 → 45 分钟（第 294 条） |
| `web/packages/api-client/src/schema.gen.ts` | 生成（4.1） |
| `e2e/fixtures/upgrades.ts`、`e2e/stories/upgrade/up6-…`、`up7-…`、`up8-…` | UP6–UP8 的 API 与 UP6、UP7 的 MCP 版本（第 5 节） |
| 测试 | 各节写出；组合根的在 7.1；三个用例的单元测试在 `wiki/app/review_upgrade_test.go`，共用的替身在 `review_fake_test.go`（S1 的实施，3.23） |

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

**测试**（单元，`wiki/app/review_upgrade_test.go` 的 `TestApproveUpgrade`（三个用例的单元测试在一个文件里，共用的替身在 `review_fake_test.go`；已有的四个替身包一层 `unapproved`：两个端口的 `Approve` 同名不同签名，一个类型装不下；初稿写 `approve_upgrade_test.go`，S1 的实施，3.23），替身的端口，照 `submit_upgrade_test.go` 的写法）：调用的次序整列断言（锁、`Current`、`Roots`、`RootNodes`、名字、`Approve`、`Rebuild`、`DeleteBaseline`、`Rows.Approve`、`Publish`）；`Publish` 的值整值断言（`From`、`To`、`By`、`At` 互不相同）；每个码一格且什么也没写；`ReviewChecking`、`ReviewStale` 各一格答 `wiki.upgrade_state`；报告不通过一格答缺陷；三道闸门各一格不过（闸门 2 另有"来源根是根下的 `AGENTS`"一格，钉住批准交给 `JudgeRoots` 的来源根；审查 P3S1A-N1，修复一轮）：答 409 `wiki.upgrade_not_ready`，error 一行整值断言（`notebook_id`、`upgrade_id`、`failed_gates` 恰是那一道的键，没有多余的键），没有 info、没有 warn（决定 293）；回调答 409 而替身的事务回滚失败一格：答回滚的那个错误（不是 409），答复照 `WithinTx` 交回的错误（设计审查 P3A-N11）；`Rows.Approve` 答假是缺陷；提交失败一格：替身的事务在回调成功之后提交失败（`submit_upgrade_test.go:59`、`:100-103` 的 `commit` 钩子），答那个错误，没有 info，也没有 warn，调用的次序里 info 在 `commit` 之后（设计审查 P3A-M3 = P3B-M3 第 1 条）；期限：替身的 `Rebuild` 等到 ctx 到期，答 503 与 warn 一行、没有 info；请求的 ctx 先被取消时不记 warn、不答 503。整个程序的在 7.1。

### 3.7 S1：驳回

`wiki/app/reject_upgrade.go`（新）：`RejectUpgrade.Execute(ctx, id) (UpgradeView, error)`，照放弃（`abandon_upgrade.go:47-98`）去掉"继续回滚"一支：

1. actor、`shared.ClientOf`（回滚任务的客户端，`web` 或 `api`）、不加锁读工作区（404）。
2. 一个事务（请求的上下文，同放弃）：`locked`（404、403）；升级不是 `reviewing` 答 409 `wiki.upgrade_state`（含 `upgrading`：驳回只给审阅中的，00 号文档 4.3）；`a.d.rollBack(ctx, actor, client, workspace, id, up.ID, domain.UpgradeRejected, at)`（`abandon_upgrade.go:106-128`，`MaxQueued` 满时 503 `server_busy`）；`view`（`:163-172`）。
3. 提交之后记 info `upgrade rejected`（3.14），答 `view`（`rolling_back`、`ending = rejected`、带任务）。

审阅中没有报告时也可以驳回（00 号文档 4.3 的状态表）：正在算的那次提交的第二个事务看到 `rolling_back`，什么也不写，答 409 `wiki.upgrade_state`（P2 3.12 的 `superseded`）。`notebook_upgrades_rejected_check`（`00057:53`）要驳回的行有提交，审阅中一定有，它是第二道防线。

`rollBack` 的注释改为"The abandonment's and the rejection's"。码：404、403、409 `wiki.upgrade_state`、503 `server_busy`。不声明 `transfer.busy`：审阅中没有写入的任务（冻结，P1），`startBackground` 的写入键答不出它。

单元测试 `TestRejectUpgrade`（`wiki/app/review_upgrade_test.go`，与批准、退回的在一个文件里；S1 的实施，3.23）：每个码一格；`upgrading` 一格答 `wiki.upgrade_state`、什么也没写；`rollBack` 的调用带 `UpgradeRejected`（替身记下结局，整值断言）；提交失败一格（同 3.6：答那个错误，没有 info，也没有 warn；调用的次序里 info 在 `commit` 之后）。

### 3.8 S1：退回

`wiki/app/send_back_upgrade.go`（新）：`SendBackUpgrade.Execute(ctx, id) (UpgradeView, error)`：

1. actor、不加锁读工作区（404）。
2. 一个事务（请求的上下文）：`locked`（404、403）；`at` 截到微秒（同提交，`submit_upgrade.go:169`：Go 与 SQL 判同一个时刻）；升级不是 `reviewing` 答 409 `wiki.upgrade_state`；`Rows.Current` 与笔记本的不同是缺陷；`ReviewOf`：`ReviewChecking` 答 409 `ErrUpgradeSubmitting`；`States.SetState(…, upgrading, …)`；`Rows.SendBack(ctx, up.ID, actor.UserID, at, at.Add(-SubmitTimeout))`，答假是缺陷；`Publish(From == To == standard)`；之后照放弃与驳回在写之后读 `view`（`a.d.view(ctx, id, up.ID)`，`abandon_upgrade.go:83`、`:163-172`：行是 `upgrading`，`Rows.Current` 读得到，提交与报告已清，不用手工改成 `nil`）。日志要的 `submission_id` 与 `reported` 取写之前读到的 `up`（设计审查 P3A-N6）。
3. 提交之后记 info `upgrade sent back`（3.14），答 `view`（`upgrading`，提交的三列与报告为空）。

码：404、403、409 `wiki.upgrade_state`。退回之后发起人的 agent 又能写（冻结只放过 `upgrading` 中发起人的 MCP 写，P1），UP6 走这一步。

单元测试 `TestSendBackUpgrade`（`wiki/app/review_upgrade_test.go`，3.23；拒绝表另有"别的升级的行"一格，审查 P3S1A-N2，修复一轮）：每个码一格；三种 `Review` 各一格（`ReviewChecking` 的 `detail` 是"still being checked"）；`SendBack` 收到的 `takenOver` 等于 `at - SubmitTimeout`（整值断言，`at` 带纳秒、断言截过的值）；答复是写之后读的行（提交与报告为空）；info 的 `submission_id`、`reported` 是写之前的；提交失败一格（同 3.6）。

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

- `page/domain/contract_test.go`：`OpRestore`、`ChangesetRevert`、`After` 在根下、键 `index` 答 `ErrVirtualFile`；同样而键是 `log.md` 一格（`Reserved` 的另一种形状）；同样而不在根下放过；同样而 `ChangesetTrashRestore` 放过；同样而笔记本是普通的放过；`OpRestore`、`ChangesetRevert` 回到根下一个不保留的键放过。每格一个反例只钉一种形状（05-lessons）。 S1 建成：新加四格（根下 `index`、根下 `log.md`、`Notes` 下的 `index`、根下不保留的 `Logs`），原来"撤销把一页恢复到根下名为 log"一格由放过改为答 `page.virtual_file`；"`ChangesetTrashRestore` 放过"由已有的 `:171` 一格、"普通笔记本放过"由 `:205` 的循环对每一格都判一次盖住，覆盖等价（审查 C，3.23）。
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

处理函数（`wiki/adapter/http/upgrade.go`）照 `AbandonUpgrade`（`:43-53`）各一个，答复经 `upgradeOf`（`:83-130`）。`UpgradeUseCases`（`handler.go:22-28`）加 `ApproveUpgrade`、`RejectUpgrade`、`SendBackUpgrade`。`TestTheUpgradesCodesAreAnswered`（`wiki/adapter/http/upgrade_test.go:187`）的表加三个操作的每个声明的码（approve 五个、reject 四个、send-back 三个），替身 `upgradeFake`（`:25`）加三个方法；`TestTheUpgradeIsAnswered`（`:139`）加三个操作各一格答 200 的投影（`approved`、`rolling_back` 带 `ending: rejected`、`upgrading` 没有提交），夹具的每个值互不相同、整值断言；`:138-143` 的"不收请求体"一格加三个路径（S1 另起一个循环，三个路径都不带请求体、各自断言整份投影，`upgrade_test.go:197-247`，比加进原来那一格强，3.23）。打包的根 `api/openapi.yaml` 逐个 `$ref` 模块的路径，三个操作的路径也要加进去，不然不进 `api/dist`、`TestAPIRoutesAreTheContractsOperations` 等答"no documented operation"（S1 的实施，3.23）。

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
| `wiki/prompts.go`、`wiki/lint.go` | `Module` 加 `upgradePlaybook`，`Prompts()` 交它；模块根另导出 `UpgradePlaybookText`、`PlaybookText`、`Playbook` 与三个常量，给组合根的测试整值比（S2 的实施，3.24） |
| `api/mcp/prompts.json` | 笔记本一项加 `upgrade` |
| `bootstrap/permission_matrix_mcp_wiki_test.go`、`mcp_prompts_test.go`、`upgrade_prompt_test.go`（新） | 3.18 |
| `README.md` | "升级为 LLM Wiki"一节，连同 `:214`、`:234`、`:236`、`:246` 与 MCP 一节的两个工具（3.19） |
| `api/mcp/tools.json`；`wiki/adapter/mcp/tools.go`、`wiki/adapter/mcp/testdata/tools.json` | `upgrade_submit` 的说明加"or sends it back"（契约两个入口各一项，`:589`、`:1298`；设计审查 P3C-N4）；常量与 testdata 的两个入口同改，初稿漏了这两个文件（S2 的实施，3.24） |
| `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md` | K（3.20） |
| `e2e/stories/upgrade/up9-playbook.spec.ts`（新）、`e2e/fixtures/mcp.ts` | UP9（第 5 节）；LW11 的 `textOf` 挪进 `fixtures/mcp.ts`、导出为 `promptTextOf`，两个故事共用（S2 的实施，3.24） |
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

单元测试：`wiki/app/upgrade_playbook_test.go`：调用的次序（"workspace""authorize lint.read""current"）；看不到答 `notebook.not_found`；没有升级、`reviewing`（没有报告与有报告各一格，后者修复一轮加，审查 P3S2A-N1）、`rolling_back` 各答 `wiki.upgrade_state`；`upgrading` 答文字。`wiki/adapter/mcp/prompts_test.go`：四个 prompt 的名字与次序；`upgrade` 的 `Get` 把笔记本交给用例、错误原样交回（`:67-70` 的拒绝一格改按名字取，不按下标）。`wiki/domain/playbook_test.go`（`playbooks_test.go:129-152` 那几格）：`UpgradePlaybookText` 的第一行、每段要有的句子（3.17 的"必有的句子"：修复一轮之后每句或分句一格，47 格，各恰好出现一次；审查 P3S2B-M1 = C-M1）、不过 `MaxPlaybookBytes`、已去掉首尾空白。

### 3.17 S2：playbook 的正文

`wiki/domain/playbook-upgrade.md`，英文（第 176 条），照三份已有的 playbook 的写法：第一行一句祈使的标题，之后平铺的段落，没有小标题，直呼工具与码的名字。初稿去掉首尾空白 4,827 字节（离 16 KiB 的界很远；设计审查 P3C-N3 改正初稿写的"约 5.6 KB"），S2 建成时逐字照它。修复一轮（`8321734b`）照审查 P3S2C-N3、N4 改了三处：闸门 4 写成来源根与 `AGENTS` 之外的页才要 `type`、`summary`；`server_busy` 对两个工具都适用；`notebook.upgrading` 那一句后接"and forbidden from upgrade_verify or upgrade_submit means your user is not an admin of it"（写"is not"：读者也取得到这份 playbook，他的 agent 调这两个工具答 `forbidden` 是因为从来不是管理员；写两个工具的名字：这一段还没讲到 `upgrade_submit`，"either tool"无所指；3.24）。下面是修复之后的全文，去掉首尾空白 4,979 字节，与 `wiki/domain/playbook-upgrade.md` 逐字相同：

```
Upgrade this notebook to an LLM Wiki.

Read and follow AGENTS.md and this notebook's instructions. The notebook is standard while it upgrades, and frozen: only the agent of the admin who started the upgrade, through MCP, may write, and only while the upgrade is upgrading. notebook.upgrading means you are not that agent, the upgrade is no longer upgrading, or your user is no longer an admin of this notebook, and forbidden from upgrade_verify or upgrade_submit means your user is not an admin of it: stop and tell your user. The upgrade proposed a source root, raw at the notebook's root, and AGENTS at the root; it may have written them.

Start with upgrade_verify without a rule. It names eight gates and eight rules: AGENTS has a body; the source root is a live page and not AGENTS; no node at the root is titled index, log, index.md or log.md; every page outside the source root has valid frontmatter, and every one but the source root and AGENTS has type and summary; every sources reference is a link to a page or attachment under the source root; no original page links to more missing pages than when the upgrade started; every original page and node is still there, its body unchanged; and the link index is up to date. Read one rule's items with upgrade_verify and that rule, and continue next_cursor until it is null. Call upgrade_verify again after each round of changes.

Sort the original pages: a source is evidence you keep as it is; a wiki page is knowledge you maintain. Move each source, with the pages under it, under the source root. Pages already under the source root that are not sources: move them out of it; after the approval, everything under it is a source, read-only. Give each wiki page, folder pages too, frontmatter with type and summary; a page that rests on sources also lists them in sources, quoted links to their paths under the source root, for example sources: ["[[raw/Note.md]]"]. You may change an original page's frontmatter only: its body after the frontmatter, and after a byte order mark, must stay byte for byte, or the write is page.upgrade_immutable. A page that starts with --- and has no closing --- line has no frontmatter: add a valid block before it. A page whose block between two --- lines is not valid YAML: rewrite that block in place, for example keeping its old text in a quoted property, or move the page under the source root, where frontmatter is not checked; a new block before it would make the old one part of the body.

Keep an existing AGENTS page whose body is not empty, or rename it and write a new AGENTS; rename any node at the root titled index, log, index.md or log.md. Never delete an original node or the proposed source root: that is page.upgrade_protected. Pages you created during the upgrade may be deleted.

Moves and renames must let the system rewrite the links that lead to what moved: never pass update_links: false, or the change is page.upgrade_keep_links. A link by name alone resolves after a move and is not rewritten; a link that names a path is rewritten when its target moves, and a rename rewrites both. A new page's title becomes the target of existing short links that end with it: choose new titles knowing that, and read the ambiguous rule's items after creating pages.

In a batch, once a move or a rename has rewritten an original page's links, or would by the links the page had before the batch, writing that page later in the same batch is page.upgrade_batch_rewrite, whatever you write: write the page first and move afterwards, or use two batches. A page changed and changed back within one batch passes, unless a move in between rewrote it or reached it. Splitting the batch does not make a body change acceptable: it is still page.upgrade_immutable. Large original pages that a batch writes whole count toward its read bound; page.batch_too_large means split the batch.

When upgrade_verify reports every gate passed, call upgrade_submit. It checks again and puts the upgrade in review, where nothing can be written. wiki.upgrade_not_ready names the gates that did not pass: the upgrade is upgrading again, so verify, fix and submit again. wiki.upgrade_stale_index means the link index is out of date: tell your user that a server admin runs nervewiki reindex; once upgrade_verify passes the index gate, submit again. wiki.upgrade_state saying the previous submission is still being checked means wait a moment and call upgrade_verify before submitting again; wiki.upgrade_state otherwise means the upgrade is already in review or no longer under way: tell your user. server_busy from either tool means wait retry_after_seconds and call it again.

Once submitted, an admin reviews the upgrade in the web app, then approves it, sends it back for more work, or rejects it, which puts the notebook back as it was. Report what you changed, which gates passed, and what remains for your user to decide. Never claim a gate passed that upgrade_verify did not report as passed.
```

**必有的句子**（单元测试逐句断言，一句一格，防止改写时漏掉一类）：修复一轮之后每格是一整句，一句里有两件事时是一个分句，共 47 格（`94db07e4` 46 格，`8321734b` 把 `notebook.upgrading` 那一句拆成两格；审查 P3S2B-N2），每格断言恰好出现一次（一个码出现在两句里时，一句不能顶替另一句）；47 格合起来覆盖第一行之外的整段正文（第一行另判），改任何一句都要同时改测试里那一格。00 号文档 4.13 清单的每一项因此都有一格：只改 frontmatter（`page.upgrade_immutable` 钉在这一句）、来源移进来源根、把原有页归为来源或 wiki 页、先 `upgrade_verify`、全过再 `upgrade_submit`、审阅中冻结。测试钉的是该有的句子在，不看有没有多出来的句子（新加一句相反的话照过，审查 P3S2B-N3，不改：新加的文字由审查看 diff）。S2 建成时片段止于条件与码，这几项删掉、做法换成错的都照过，`page.upgrade_immutable` 一格被"拆开仍是 `immutable`"那一句代为满足（审查 P3S2B-M1 = C-M1、B-N1，修复一轮照改，3.24）。初稿列的是：第一行；"only the agent of the admin who started the upgrade"；`notebook.upgrading`；八道闸门的那一段里每道闸门的说法；`next_cursor`；`page.upgrade_immutable`、`page.upgrade_protected`、`page.upgrade_keep_links`、`page.upgrade_batch_rewrite`、`page.batch_too_large`；"has no closing --- line"与"is not valid YAML"两种 frontmatter；"A new page's title becomes the target"；`wiki.upgrade_not_ready`、`wiki.upgrade_stale_index`、"still being checked"、`retry_after_seconds`；最后一句；设计审查 P3C-M6 的三句："is a link to a page or attachment under the source root"、"tell your user that a server admin runs nervewiki reindex"、"Pages already under the source root that are not sources"（决定 214）。设计审查之后改了六处（P3C-M6 的三处、P3C-N18 的三处：每一轮改动之后再验、建页之后读 `ambiguous` 的条目、只留正文非空的原有 `AGENTS`，空正文的改不了正文，只能改名另建，第 172 条）。逐项对着 P2 第 9 节"P3/S2"的四条与 00 号文档 4.13 的清单核对过：护栏的三个码、无效 frontmatter 的两种改法、改了又改回、拆的写法、拆开之后仍是 `immutable`、只写名字的链接、盲写计进读的界、`retry_after_seconds`、"正在计算"时等一会再检查、已在审阅中就告诉用户。

### 3.18 S2：prompt 的测试改写

- **`TestThePromptReadsLLMWikiMatrix`**（`bootstrap/permission_matrix_mcp_wiki_test.go:153-208`，第三份）：循环改为只取 `ingest`、`query`、`lint` 三个（按名字列出，不是 `prompts.json` 的每一项）；`:177` 的"没有行就失败"随之只对这三个；`:205-206` 的断言改为"`prompts.json` 里恰有这三个与 `upgrade`，`checked == 2 * 13 * 3`"，加一个 prompt 而没有在这里或 `TestUpgradePromptMatrix` 里有期望时失败。
- **`TestThePromptListsAreTheContract`**（`bootstrap/mcp_prompts_test.go:101`，第三份）：`:105-106` 的 `!= 3` 改为 `!= 4`，注释（`:97-100`）"lists the three"改为"lists the four"。
- **`TestTheWikiPromptsAreReached`**（`:145-259`）：注释与表（`:140-144`、`:167-173`）照旧是三个 LLM Wiki 的 prompt；`upgrade` 的到达在新的测试里。
- **`TestUpgradePromptMatrix`**（新，`bootstrap/upgrade_prompt_test.go`，第七份，第 222 条）：四个 prompt × 四种状态（`llm_wiki`；没有升级的 `standard`；`upgrading`；`reviewing`）× 13 个笔记本列。期望表：`ingest`、`query`、`lint` 在 `llm_wiki` 照 `promptCellOf`（`permission_matrix_mcp_wiki_test.go:131-147`）答文字，在后三种答 `notebook.not_llm_wiki`；`upgrade` 只在 `upgrading` 答文字，别的三种答 `wiki.upgrade_state`；看不到的列在 HTTP 的边上答 404。升级的两种状态在升级的变体上（`TestUpgradePermissionMatrix` 的同一组建法）：每种升级状态一份库的副本（`upgradedBy` 的建法），矩阵的几本笔记本在副本上以 `upgradeIn` 设好，4 个 prompt × 13 列在同一份上读，不为每格复制（prompt 只读；设计审查 P3C-N10）。计数断言 `checked == 4 * 4 * 13`。变异"prompt 在审阅中也答"由 `reviewing` 一格抓住（00 号文档 4.13）。S2 建成：两份副本在任何应用连上准备好的库之前做（`pgtest.NewDatabaseFrom` 要求没有人连着模板），每份设四本活着的矩阵笔记本（13 列落在五本上，`gone-nb` 已删）并断言副本里恰有四个这种状态的升级，`llm_wiki`、`standard` 在准备好的库上读，三个应用各建 13 个 PAT；种的审阅中没有报告（3.24）。
- **`TestUpgradePrompt`**（新，同一个文件）：经真的笔记本入口：`upgrading` 的笔记本取 `upgrade`，一条 `user` 的消息、文字等于 `UpgradePlaybookText()`、`description` 等于 `prompts.json` 的；读者（能读、不是管理员）也取得到（`lint.read`）；没有升级的普通笔记本、审阅中、回滚中、LLM Wiki 各答 `-32602`、`data` 是 `wiki.upgrade_state`；`prompts/list` 在 LLM Wiki 与普通笔记本里都列出它；取用的日志一行（`prompt` 是 `upgrade`，不记参数，照 `:240-258`）。 S2 建成：审阅中由 alice 经 REST 真的提交（存下报告），回滚中以 SQL 设（真的驳回之后小笔记本很快回滚完，`rolling_back` 与 `rolled_back` 同码，钉不住要测的那一种）；整值比的 `UpgradePlaybookText()` 经模块根导出（组合根只能 import 模块根）（3.24）。

### 3.19 S2：README 的"升级为 LLM Wiki"一节

`README.md` 的"### LLM Wiki 与 playbook"（`:230-246`）之后、"## 接入 agent（MCP）"（`:248`）之前加一节：

```markdown
### 升级为 LLM Wiki

一本普通笔记本可以升级为 LLM Wiki：笔记本的管理员在"设置 → 模式"里点"升级为 LLM Wiki…"。升级中笔记本对所有人只读，只有发起人自己的 agent 经 MCP 能写（提交审阅之前）；冻结没有时限，任何笔记本管理员都可以随时放弃（审阅中网页上由"驳回"代替，效果相同）。REST 的路径都在 `/api/v0/notebooks/{notebook_id}/upgrade` 之下：`POST` 它发起、`GET` 它读，另有 `check`、`submit`、`approve`、`send-back`、`reject`、`abandon`、`end-rollback`（`api/modules/wiki.yaml`）。

1. **发起**：根下有标题为 `raw` 的页时，它成为来源区（批准之后它下面的都是只读的来源）；没有就补写一页 `raw`。根下没有 `AGENTS` 时补写一页（取界面语言的模板）。原有页的正文不改。
2. **接 agent**：照下面"接入 agent（MCP）"一节，用发起人自己的访问令牌，把 agent 连到这本笔记本的入口 `/mcp?notebook=<id>`（id 在网页地址 `/notebooks/` 之后），运行 prompt `upgrade`。agent 照它把来源移进 `raw`、给来源区之外的 wiki 页补上 `type`、`summary`（依据来源的页另列 `sources`），反复用 `upgrade_verify` 检查。升级中原有的页只能改 frontmatter、不能删，移动与改名必须改写链接；被拒时工具的错误说明怎么改。
3. **提交**：八道闸门都过了，agent 调 `upgrade_submit`，或在升级页上点"提交审阅…"。服务端再检查一次，通过就进入审阅，存下这次的报告；不过就回到升级中。
4. **审阅**：任何笔记本管理员在升级页上看报告，然后"批准"（笔记本成为 LLM Wiki，来源区只读、`AGENTS` 与来源根受保护）、"退回继续修改"（回到升级中，agent 可以再写）或"驳回"（在后台撤销这次升级的全部改动）。批准之后没有整体的撤销：升级的变更集仍可在历史里照 LLM Wiki 的规则逐个撤销；要回到普通笔记本用降级。
5. **放弃与回滚**：放弃与驳回都在后台按发起时记下的样子把笔记本放回去；回滚停下时可以继续，或者"结束回滚，保留现在的样子"。

批准时服务端为来源区里每一页正文非空的页与每个附件算一次来源的哈希，数万个来源要几秒；批准的期限是 `server.request_timeout` 的四分之三，太短时答 503 `server_busy`、什么也不写（`server/configs/config.yaml` 的 15 秒在十万个来源上有余）。

**已有的笔记库**：要把一份按 LLM Wiki 整理过的库（例如从另一个实例导出的 LLM Wiki）搬进来，先导入一本新建的普通笔记本（根下正好是 `AGENTS` 与 `raw`，导入不加序号；根下已有 `raw`、`AGENTS` 的笔记本照样加序号），再升级：发起认出根下的 `raw` 作来源区、留着原有的 `AGENTS`，agent 只需补齐 frontmatter。根下若有 `index`、`log`、`index.md`、`log.md`，要由 agent 改名，闸门才会过。
```

同一个提交一并改 README 里 S2 让它不实的几处（设计审查 P3C-M7）：

- `:234`"`prompts/list` 提供英文的 `ingest`、`query`、`lint`"改为"提供英文的 `ingest`、`query`、`lint` 与 `upgrade`（`upgrade` 只在升级中取得到，见'升级为 LLM Wiki'）"；
- `:236`"普通笔记本也列出同样三个名字，取用答 JSON-RPC -32602，码是 `notebook.not_llm_wiki`"改为"普通笔记本也列出同样四个名字，取用前三个答 JSON-RPC -32602、码是 `notebook.not_llm_wiki`；`upgrade` 不在升级中时同样答 -32602、码是 `wiki.upgrade_state`"；
- `:214` 的内置说明的字节数照 S2 实测时 `TestTheBuiltInInstructionsAreShort -v` 的数写（审查 C 在 `955fa579` 上量到 notebook 3225/3276、account 3294/3346，距 4 KiB 余 820、750；S2 不改内置说明，它不列 prompt），"保留M11至少300B余量"改为写实测的余量；
- MCP 一节的工具（`:219` 一条之后）加一句："升级中的笔记本入口另有 `upgrade_verify`（只读，检查闸门与规则）与 `upgrade_submit`（提交审阅），见'升级为 LLM Wiki'"（S2 建成时照第 196 条写"两个入口另有"、另起一条"**升级**"放在"工具"一条之后；修复一轮照审查 P3S2C-N2 写"笔记本的管理员在升级中用得上（`upgrade_verify` 审阅中也可）"；3.24）；`:299` 的工具数（"笔记本入口有 14 个工具（账户入口 16 个）"）与 `:319` 的注解列表 S2 照 `api/mcp/tools.json` 一并核对（M10、M11 都加过工具）；
- `:246`"**重新导入的限制**"一段末尾加"要这样的库成为 LLM Wiki：导进新的普通笔记本再升级，见下一节"，新的一节不再重复 `AGENTS 2`、`raw 2` 那一句。

上面的第 2 步与"已有的笔记库"末句是修复一轮照审查 P3S2C-N1、N6 改过的（初稿写"给其余的页补上 frontmatter（`type`、`summary`、`sources`）"，读来每页都要 `sources`；"根下的……要由 agent 改名"读来一定要改名；3.24）。文字照 00 号文档 4.4、4.8–4.10、4.19 与负责人的四个回答写；"导入不加序号"一句照 `bootstrap/virtual_export_import_test.go:33-36` 核对过。S2 的文档提交另核对 `README.md:157`、`:169-172`、`:481-525` 的"升级"指的是服务端的升级（第 4.13 条的"server update"只改了工具的措辞，README 的中文不歧义），不改。

### 3.20 S2：MCP 人工清单的 K

`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`：J 一节（`:288-308`）之后、"## 做完"（`:310`）之前加：

```markdown
### K：升级（M11）

这一节由 M11/P3 加。每步只记客户端实际做了什么、显示了什么，不评判 agent 整理得好不好。

准备：沿用上面隔离的客户端环境，记下 Claude Code 与 Codex 的版本和日期，不碰自己的配置。两个客户端各用自己的一本：同一位管理员在网页上在两本普通笔记本上各发起一次升级（设置 → 模式 → 升级为 LLM Wiki），每本留一页正文非空的原有页。一个客户端做完 K4，它那本就进入审阅、对所有人冻结，所以另一个客户端从 K1 做起用的是另一本。接入用的 PAT 必须是发起人的，否则所有写都被冻结（`notebook.upgrading`）。另备一本没有升级的普通笔记本，K5 两个客户端共用它。

| 步 | 做什么 | 看什么 |
|---|---|---|
| K1 | 在升级中的笔记本接笔记本入口 | prompt 列表里 `upgrade` 怎样列出（名字、标题、说明） |
| K2 | 取 `upgrade` | 文字是否完整显示；客户端是否把它当作一条用户消息 |
| K3 | 让客户端照它调 `upgrade_verify`，并让它改一页原有页的正文（在提交之前） | `upgrade_verify` 的审批怎样问（只读）；`page.upgrade_immutable` 在客户端里怎样显示，agent 是否读懂、改为只改 frontmatter |
| K4 | 让客户端调 `upgrade_submit` | 审批怎样问（`upgrade_submit` 不是毁坏性的：`destructiveHint: false` 是否让客户端少问一次）；笔记本入口的 `upgrade_submit` 的 `inputSchema` 没有 `properties`，客户端是否接受；提交不过时（多半是 frontmatter 没补齐：答 `wiki.upgrade_not_ready`，说明列出没过的闸门，升级回到升级中）它怎样显示、agent 是否照它补齐，补齐之后再交，直到通过；提交通过之后再写答 `notebook.upgrading` 怎样显示 |
| K5 | 在没有升级的普通笔记本上取 `upgrade` | `wiki.upgrade_state` 怎样显示 |

逐客户端记通过或失败和处置。发现 upgrade playbook 的措辞有偏差，记下原文，由负责人定怎样改，不放松服务端的守卫。
```

同时把 `:308` 的"…M11 的 upgrade prompt 不在这一节。"改为"…M11 的 upgrade prompt 在 K。"，`:323`"M11 的 upgrade prompt 由 M11 登记。"改为"M11 的 upgrade prompt 在 K（M11/P3 加）。"；头（`:8`）把 J 的结果交 M10 收尾审查的那一句之后加"K 的结果交 M11 的收尾审查"。K3、K4 落实 P2 第 9 节"P3/S2"的第三条与审查 C 的未核实的怀疑。改正文在提交之前：提交通过之后升级进入审阅、对所有人冻结，再改答的是 `notebook.upgrading`，看不到 `page.upgrade_immutable`（设计审查 P3C-M9；00 号文档 4.20 照改，2.1 第 11 条）。K 与 J 一起在 v0.1 的最后执行（00 号文档 4.20）。 上面的块是修复一轮照审查 P3S2C-M2、N5 改过的（`7daa3452`）：初稿只备一本升级中的笔记本，第一个客户端的 K4 把升级送进审阅之后，第二个的 K2、K3 看不到要看的东西；修复之后准备照 J 加"沿用上面隔离的客户端环境……不碰自己的配置"，两个客户端各用自己的一本、K5 共用那本普通笔记本，K4 写明提交不过（`wiki.upgrade_not_ready`）时记什么、补齐再交，表后照 J 加"逐客户端记……"（3.24）。

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
| 重建的读头只经键（决定 292） | 守卫（7.1）：调用换回 `SourceHeads` 时两个种子都以"没跑过 `SourceRebuildHeads`"失败；SQL 改成普通的连接（16a）或 `SourceHeads` 的形状（16b）时两个种子的每格都失败（这本笔记本的 `nodes` 按笔记本的索引读、不按键），16b 在只核对计划的种子"发起留下的统计"的两格另顺序读 `page_contents`（"读头只经键"一条，S1 的变异第一轮补的，3.23）；短测试 `TestTheApprovalGuardJudgesItsReads` 的"读头连上读一次的这本笔记本"一格 |
| 批准删掉基线 | `TestUpgradeApproval` 断言 `upgrade_nodes` 没有这次升级的行；`checkPages` 的升级规则（`upgradeRules()`）对已结束的升级的基线报违规 |
| 批准发 `mode` | `TestUpgradeStatesReachTheStreams` 的"approval"一步（1 帧）；`TestUpgradeApproval` 的另一个标签页的流 |
| 批准自己的期限 | `TestUpgradeApproval` 的"its own deadline"三格（语句之中等笔记本行、语句之中等一个来源的行、两条语句之间） |
| 驳回只给审阅中 | `TestUpgradeReview` 的 `upgrading` 一格答 409、什么也没写 |
| 退回的接管时刻 | `TestUpgradeReview` 的"没有报告、刚提交"答 409 `still being checked`、"一小时之前"答 200；`TestTheReviewsRows` 的 `taken_over` 两格；单元的边界三格 |
| 退回清掉报告 | `TestUpgradeReview` 断言行回到 `backToUpgrading`；之后再提交通过并存下新的报告（`submission_id` 不同） |
| 第 181 条 | `TestUpgradeTrashReservedTitles` 的同步与后台两格答 `page.virtual_file`；`contract_test.go` 的各格；两个模块的 `adapter/http` 表；撤销改名的一格守着今天就有的 `OpRename` 一支在批准之后走得到 |
| 第 181 条不碰回收站的恢复 | 同一个测试的 `restoreNode`、`startTrashRestore` 两格答 200、名为 `index 2`、`log 2`；`contract_test.go` 的 `ChangesetTrashRestore` 一格 |
| prompt 只在 `upgrading` | `TestUpgradePromptMatrix` 的 `reviewing`、`standard`、`llm_wiki` 三列（种的审阅中没有报告）；`TestUpgradePrompt` 的各格（审阅中由真的提交，有报告）；单元的 `reviewing` 两格（没有报告、有报告，后者修复一轮加，审查 P3S2A-N1） |
| playbook 的每一类建议（00 号文档 4.13 的清单） | `TestUpgradePlaybookText` 的"必有的句子"，每句或分句一格（47 格，合起来覆盖第一行之外的全文）、各恰好出现一次（3.17；审查 P3S2B-M1 = C-M1，修复一轮） |

### 3.23 S1 的实施对设计的修正

S1 的审查与修复见 [S1 的审查记录](reviews/P3-S1-review-actions-review.md)，结果在 10.1。与上文最初的设计不同之处已改在各节里（3.1、3.6–3.8、3.10、3.11、3.22，6.1，7.1–7.3、7.5、7.6）。写了行号的，是实施的头 `d7b85bae` 的；修复的提交见 10.1。

- **S1，实施时定下的**（实施者的 16 条偏离，括号里是它的号；三位审查者各在自己的范围里判过：A 判第 1–3、5–8 条，B 判第 9–13、16 条，C 逐条判进不进本节、判第 4、14 条；都对。第 3 条的断言不靠回滚也成立，修复一轮改（审查 P3S1A-N4）。第 8 条"退回之后 agent 的写只改 frontmatter"不是与设计不同：设计 7.1 本来就写"写一页原有页的 frontmatter"，是实施中改掉的测试错误，只记在 10.1；第 15 条"第四、六份只在 1 号提交上量"是过程，也在 10.1（审查 C 的判断））：
  - 3.3、3.6 的测试：已有的四个替身（发起、放弃、提交、回滚的结束）包一层 `unapproved`。wiki 的端口 `UpgradeStates.Approve(ctx, id, upgradeID, rootID, by, at) error` 与 `UpgradeRows.Approve(ctx, id, submissionID, by, at) (bool, error)` 同名而签名不同，生产里是两个类型；测试里一个类型同时当 `States` 与 `Rows`，Go 的一个类型装不下两个同名方法。名字照设计不改，`States: unapproved{f}`（它的 `Approve` 是缺陷），批准的替身另用 `reviewStates`；只动测试（1）。
  - 7.1：`stops.arm` 有四个参数（`nb, plan, batch, idle`），设计写三个；流的驳回一步照代码 `stops.arm(nb, 1, 0, false)`（2）。
  - 7.1 `TestUpgradeReview` 第 4 格的"基线复原（发起时的 frontmatter）"：`tm.reviewed` 的升级里发起只写了 `AGENTS`，实施时断言为 `AGENTS` 不在、笔记页的正文同发起时（3）。"正文同发起时"不靠回滚也成立，真正证明回滚做了事的只有"`AGENTS` 不在"；修复一轮在驳回之前先由 dana 退回（审阅中冻结 agent 的写），alice 的 agent 再经 MCP 改 `Note` 的 frontmatter（仍干净，同"sent back"那一格的写法），erin 再交，然后驳回，断言正文回到 `cleanNote`；回滚不写回正文的变异 n4 改前活、改后死（审查 P3S1A-N4，`cc751dca`；`fix-1.md` 与文档稿的初稿都没写"先退回"，修复核对一轮 P3S1C-N7）。
  - 3.1、3.11：`api/openapi.yaml` 加三个路径。打包的根逐个 `$ref` 模块的路径，不加的话三个操作不进 `api/dist`，`TestAPIRoutesAreTheContractsOperations` 等答"no documented operation"；设计的文件表与 3.11 漏了这个文件（4；审查 C 判必要）。
  - 7.1 `TestUpgradeApproval` 第 6 格：期限的三格共用一个应用、都取 2 s（设计写第一、三格 300 ms、第二格 2 s）。第二格必须 2 s（设计审查 P3B-N9），另起一个 300 ms 的应用只为两格各快约 1.7 秒，不值；2 s 下第一格（等笔记本行）与第三格（在 `Rebuild` 里等 ctx）一样确定。三格照设计各断言 503、`Retry-After: 1`、warn 一行（`timeout_ms` 2000）、没有 error、什么也没写、`checkPages`；第二格另断言包住 `SourceHashes` 的一层记下了进入、`WaitForLockWaitsOn(nodes, 1)` 看到批准等在那一行上（5；A 判对，只多约 3.4 秒）。换期限的钩子里原来不看组合根交的值，竞态的八份里没有一格看 `ApproveTimeout = RequestTimeout * 3 / 4`，只有 costs-1 的守卫看；修复一轮在钩子里先断言它是 3 s（测试应用的请求 4 s 的四分之三），再换成 2 s，o6 在第八份里也死（审查 P3S1A-N6，`cc751dca`）。
  - 7.1 `TestUpgradeApproval` 第 2、3 格合成一格（"approved, another tab told"）：同一次批准，alice 的流在批准之前打开（`tm.modesOf`），一帧 `mode`；其余断言照第 2 格（6）。
  - 3.6–3.8、7.3：三个用例的单元测试在一个文件里（`wiki/app/review_upgrade_test.go` 的 `TestApproveUpgrade`、`TestRejectUpgrade`、`TestSendBackUpgrade`，共用的替身在 `review_fake_test.go`）；7.3 写的 `reject_upgrade_test.go`、`send_back_upgrade_test.go` 的格在后两个测试里（7）。
  - 7.1 守卫：另几本是 LLM Wiki，每本根下 `raw`（来源根）收 45,000 页与 2,000 个附件、带哈希，另 4,999 页在根下，各页有正文；基线行用一个没有行的升级 id（`md5(notebook_id)`，照设计审查 B 的探针），每个节点一行（第 295 条，实施中定；9）。设计只写了"九本各 5 万页、来源带哈希的 LLM Wiki……各另有 2,000 个附件"，没说页与附件放在哪里；B 判合 7.1 的意图：附件节点、`asset_blobs` 的行与带哈希的来源都在别本里，丢了键会读到。
  - 7.1 守卫：计划在哪里判（第 296 条，实施中定，审查之后改写；10）。实施时在第一次批准的事务里、删基线之后，在同一个事务里 `SHOW plan_cache_mode` 并 `EXPLAIN` 记下的语句。审查 B 指出那时 `SetSourceHashes` 已写完 47,500 行（非 HOT，`nodes` 的堆与每个索引都长了），被判的九条里有八条在写之前或写之中真跑，判的是生产那一刻不在的状态：B 的探针在每条第一次真跑之前、同一连接上 `EXPLAIN`，两个种子 × 三种统计 × 两种方式共 84 条，82 条形状相同，2 条不同（计时种子"从未分析"的 `ForgetSources`，真跑时经 `nodes_notebook_id_name_key_idx`、守卫判的是 `nodes_notebook_id_kind_source_hash_idx`，都合规则；审查 P3S1B-M2）。修复一轮照 B 改：`EXPLAIN` 挪到 tracer 的 `TraceQueryStart`，每条被判的语句第一次真跑之前、在同一连接上判（自定义带它的参数，通用经 simple protocol 的 `EXPLAIN (GENERIC_PLAN)`），用时照旧从 `elapsed_ms` 里减；`SHOW plan_cache_mode` 留在包住 `DeleteBaseline` 的一层，每次批准断言；`EXPLAIN` 本身出错时先报计划的错误，不只报"批准答 500"（审查 P3S1B-N1）（`9c637540`；守卫一轮 117.8 秒，B 的那两条 `ForgetSources` 现在判的是真跑时的 `nodes_notebook_id_name_key_idx`，每格第一次批准的 `EXPLAIN` 约 12–14 ms，计划都合规）。
  - 7.1 守卫：判计划时内侧逐行的 `nodes` 认条件里的 `(id = `（第 297 条，实施中定；11）：`SetSourceHashes` 的通用计划在内侧经 `nodes_notebook_id_id_key`（`(notebook_id, id)` 的唯一约束）以 `notebook_id = $1 AND id = v.id` 逐行读，这是按外行的键读、合 7.1 的规则；第一版只认 `nodes_pkey`，把它判错了。审查之后另一面收紧，见下面"判计划的规则与它的短测试"。
  - 7.1 守卫：批准之前先 `awaitJob(jobs.PurgeKind)`（`RunOnStart` 的周期作业是一起插入的，purge 做完时别的都已在表里），再等 `river_job` 里到期的作业都不在 `available`、`running`、`retryable`（设计写"等启动时的周期作业做完"；12）。
  - 6.1、7.1、7.5：守卫比估的快，本机整个 111–121 秒（设计估 4–5 分钟；种子 9–28 秒一份，副本与应用每次约 1.5 秒）；`Makefile` 的说明照实测写（6.1 规定的"本机约 4–5 分钟、7763 上约 7–9 分钟"），`ci.yml` 的时限照第 294 条仍是 45 分钟（13；数在 10.1）。
  - 第 5 节：UP6、UP7 的 API 版本由 alice 经 API 提交，MCP 版本经 `upgrade_submit`（设计写"agent … `upgrade_submit`"，没分版本）；UP6、UP7、UP8 另加四处 409 `wiki.upgrade_state` 的拒绝（审阅外的驳回、回滚中的退回、批准之后的驳回、提交之前的批准），`postReject`、`postSendBack`、`postApprove` 因此有用处（`knip` 要导出的都被用到，设计审查 P3C-N8 要它们在），也让 e2e 看见状态的 409 经过 HTTP 的边（14；C 判都照契约）。
  - 7.3 第 16 行做成两条（16）：16a 把 `SourceRebuildHeads` 写成"`unnest` 连 `nodes`、左连 `page_contents`"的普通连接，16b 换成 `SourceHeads` 自己的形状（"改回连接的形状"的字面）；见下一条。
- **"读头只经键"一条的来历**（7.1、7.3 第 16 行、3.22）：设计 7.1 写"`SourceRebuildHeads` 两种计划都只经键"，比判计划的通用规则严；实施的第一版只有通用规则，而通用规则放行"按笔记本的索引读一次这本笔记本的 `nodes`"（`LintNodeMetadata`、`SetSourceHashes` 正需要它）。变异第一轮的 16a 因此没抓到：它的计划正是把这本笔记本的节点读进哈希表、`page_contents` 逐行经主键（3.4 表的"每块一次"），不顺序扫描 `page_contents`。实施者在推送之前改写 6 号提交（`232cd613` → `bba17208`）：`approveCostScans` 对 `SourceRebuildHeads` 要求每个 `nodes`、`page_contents` 的读都按行的键（`nodes_pkey`，或条件里有 `(id = `；`page_contents_pkey` 带 `node_id = `），这本笔记本的 `nodes` 一次也不读；`TestTheApprovalGuardJudgesItsReads` 加一格"读头连上读一次的这本笔记本"答拒绝。第二轮 16a、16b 都抓到：计时种子的六格都以"nodes read by … not by its key"失败；16b 在 22% 的种子"发起留下的统计"的两格是 `page_contents` 的 `Seq Scan`（3.4 与设计审查 B 说的那一格），其余以按笔记本读 `nodes` 失败；o8（去掉这一条）由新的一格抓到。四轮守卫里 `SourceRebuildHeads` 的计划只有一种（`Function Scan` → 逐行 `nodes_pkey` → `Memoize` → 逐行 `page_contents_pkey`），这一条不拒它。7.3 第 16 行原写"SQL 改回连接的形状……在一成的种子上抓不到"，是加这一条之前的说法，照实拆成 16a、16b。
- **S1，实施者自己加的测试**（实施报告第 8 节；审查者在各自的范围里看过都对）：`TestTheReviewsRows` 多三格（`Approve(another upgrade)`、`Approve(approved)`、`SendBack(another upgrade)`：设计列的是 3.5 每个合取项与析取项一个反例，`id = $n` 一项没单列，去掉它时别的升级的行会被改）；判计划的短测试 `TestTheApprovalGuardJudgesItsReads`（组合根，与守卫同文件、`//go:build !race`，不要数据库，在 short 一行里跑：守卫只在 costs-1、两分钟，规则写错（第 297 条那一次）要在短测试里先看见；修复一轮为每个子条件补一格反例，见下面）；守卫的应用断言自己的 `ApproveTimeout`（`approveCostApp`：15 s 的请求下 11.25 s，变异 o6 由它抓）；七条变异 o1–o7（7.3）；e2e 的四处拒绝（上面第 14 条）。
- **审查另指出、也进本节的**（审查 C 的第 3 节）：
  - 7.1 `TestUpgradeApproval` 第 2 格：设计列了 `checkLinks`、`checkSearch`，实施只调了 `assertSourceHashesMatchRebuild` 与 `checkPages`，报告也没列这处不同（审查 P3S1A-N3 = P3S1C-N3）；修复一轮补上两行（`cc751dca`）。7.2：`TestUpgradeReviewInterleavings` 在全部交错之后对整个库查一次 `checkPages`、`checkLinks`、`checkSearch`（`upgrade_review_test.go:349-352`），不是"每个交错结束时"；检查读的是整个库，库里有每个交错的笔记本，效果相同，7.2 照实改。
  - 3.10、7.1：`contract_test.go` 的"六格"是新加四格（根下 `index`、根下 `log.md`、`Notes` 下的 `index`、根下不保留的 `Logs`）与原来"撤销把一页恢复到根下名为 log"一格由放过改为答 `page.virtual_file`；设计要的"`ChangesetTrashRestore` 放过"由已有的 `:171` 一格盖住，"普通笔记本放过"由 `:205` 的循环对每一格都判一次盖住，覆盖等价。
  - 3.11："不收请求体"一格没有加三个路径，另起一个循环（`wiki/adapter/http/upgrade_test.go:197-247`），三个路径都不带请求体、各自断言整份投影，比加进原来那一格强。
  - 7.1 `TestUpgradeStatesReachTheStreams`：设计写批准一步"1，`From standard`、`To llm_wiki`"。`mode` 帧里没有前后模式（`bootstrap/events_registrants.go:63-66` 发的是 `{}`，`EventMode` 只有 `workspace_id`、`notebook_id`，M10-mode 移交第 3 条），流的测试只数帧；`Publish` 的 `From`、`To` 由单元 `TestApproveUpgrade` 整值断言（`review_upgrade_test.go:123-125`）。照实改（审查 P3S1A-N7 = P3S1C-N2）。
- **S1，审查与修复带来的**（编号是 S1 的审查记录的；修复一轮照主持者的修复说明 `fix-1.md`，提交见 10.1）：
  - **判计划的规则与它的短测试**（P3S1B-M1）：`approveCostScans` 对 `nodes_pkey` 不看条件（`upgrade_approve_cost_test.go:616-619`），没有条件的 `Index Scan using nodes_pkey`（Merge Join 或按 `id` 排序时按键序读整张 `nodes`）在任何一条被判的语句里都放过，"读头只经键"一条也放过它；这正是 P1/S2 的 S2A-N4 对 `page_contents` 补上的那一种，这里对 `nodes` 漏了。短测试有六个子条件各自去掉照过（B 的 r1–r6）。修复一轮：`nodes_pkey` 只在条件里有 `(id = ` 时才认；内侧逐行的 `nodes` 读条件含 `= ANY` 的拒绝（B 的第五种；守卫在真的计划上照样绿，所以加了）；短测试照 S2A-N4 的做法为每个子条件补一格反例（无条件的 `nodes_pkey` 一次与内侧逐行各一、只以 `id` 用 `nodes_notebook_id_id_key`、无条件的 `page_contents_pkey`、不带 `node_id` 的 `asset_blobs_node_id_key`、`upgrade_nodes` 的 `Seq Scan` 或经 `upgrade_nodes_notebook_id_idx`、内侧逐行不带 `node_id` 的 `upgrade_nodes_pkey`、非 `SourceRebuildHeads` 的语句内侧逐行按笔记本读 `nodes`），r1–r6 与新的 `nodes_pkey` 条件各自去掉时都有一格失败（`9c637540`；短测试 9 → 19 格，另一格正例"一次按键读一组节点"放行，钉住 `= ANY` 那一条只管内侧；r1–r6 在 `d7b85bae` 上活、修复之后死，r7（`nodes_pkey` 不看条件）、r8（去掉"内侧不许 `= ANY`"）死，B 在核对时另加的 r9（`= ANY` 连只读一次的也拒）被那一格正例抓到）。
  - **计划在真跑之前判**（P3S1B-M2、N1）：见上面第 296 条。
  - **界**（P3S1B-Q1 → 第 298 条，主持者的决定）：3.9 s 不改；合并之后看两次 `main` 的 costs-1，任一次在 EPYC 7763 或更慢的 runner 上任一格的中位数过 3.25 s（余量不到 1.2 倍），就照负责人 2026-11-06 的话把界调到 4.5 s，不调量级、不做优化；守卫里界的注释写明这个条件（`9c637540`；修复一轮的分支 CI 上 costs-1 在 Xeon 8573C，六格中位数 2.01–2.23 s，余量约 1.75 倍）。
  - **测试补全**（P3S1A-N1、N2、N5，C-N4；N4、N6 见上面第 3、5 条）：批准的闸门表加"来源根是根下的 `AGENTS`"一格（批准交给 `JudgeRoots` 的来源根换成零值时照过，a1）；退回的拒绝表加"别的升级的行"一格（a2）；退回一格照批准的做法与 `getUpgrade` 之后的读整值比较；UP7 以 `node:crypto` 对 `"The paper.\n"` 与附件的字节各算 SHA-256、与 `source_hash` 比（7 号提交原写"hashed as their bytes"，比的却是库里的两列）（`cc751dca`、`9dac9244`；a1、a2 改前活、改后死；UP7 一条由审查 C 以变异核过：上传时的 SHA-256 每块前多喂一个字节，库里两列一起错，旧的 UP7 照过、新的失败）。
  - **注释**（P3S1C-N5）：`Makefile` 的计数注释改为头上的 551 个、145/50/120/120/61/15/28/12（6.1 第 1 条与 1 号提交时的是 547 个、…/27/9）（`a44602b0`；另写第七、八份本机两轮的数）。
  - **文档稿**（本提交）：P3S1A-N7 = C-N2（上面一条）；P3S1C-N1：v0.1 总体设计 13.4 第 8 条写"5 万个来源（47,500 个要写的哈希……）"，设计 2.1、第 280 条、`Makefile` 与守卫里笔记本的名字都叫"5 万个来源"，守卫的常量 `approveCostSources` 叫的是要写的哈希；"约占四分之一"改为 22%（60,003 / 268,003）；P3S1C-N6：13.1 第 1 条写"审阅的三个动作（批准、驳回、退回）"，不写"三个动作与批准"，第 8 条写"回收站的恢复（同步与后台）照旧加序号"（后台的撤销 `startRevert` 是被拒的那一个）。
  - **修复一轮**（4 个提交，头 `a44602b0`，10.1）与 `fix-1.md` 不同之处（实施者的号，主持者都接受）：内侧逐行不许 `= ANY` 一条加了，守卫在真的计划上照样绿（1）；短测试多一格正例"一次按键读一组节点"，钉住那一条只管内侧（2）；A-N4 先退回再写：审阅中冻结 agent 的写，"改 frontmatter、再交、再驳回"只能在退回之后，驳回一格因此多一次退回与一次提交，退回在 `modesOf` 之外、驳回一段照旧数两帧（3，见上面第 3 条）；A-N5 保留逐项断言，整值比较加在后面（4）；C-N5 的注释另写第七、八份本机两轮的数（5）；"the approval ran no %s"挪进 `end()` 判，错误文字去掉"before the baseline's deletion ended"（6）；第 296 条的说法照改（7，本提交）。
  - **修复核对一轮**（三位各在 `a44602b0`）：A 判 P3S1A-N1–N6 关上（a1、a2、n4、o6 在 `d7b85bae` 上活、在头上死），没有新发现；B 判 P3S1B-M1、M2、N1、Q1 关上（短测试 19 格，r1–r8 各有一格失败，第一轮的 5 个探针都被拒；24 次判计划里"内侧不许 `= ANY`"没拒掉真的计划）；C 判 P3S1C-N3–N5 关上，文档稿里 N1、N2、N6 与 A-N7 落对。新的 Nit 两条：P3S1B-N2（批准在跑到被判的语句之前就失败时，`PlanErr` 先报、只有状态码，丢了答复的正文）不改，只影响守卫失败时的诊断、不影响判定；P3S1C-N7（A-N4 的修复在本节、7.1 与审查记录漏了"先退回"）在本提交改。一轮之后没有 Important、Minor 留下，不再有修复轮。
- **交给后面的**：守卫在慢的 runner 上的余量与第四、六、八份在 7763 上的数，以合并之后两次 `main` 为准（第 298 条，第 8 节第 1 条，10.1）；`TestTheApprovalGuardJudgesItsReads` 在 short 一行，不在组合根的竞态八份里，`-list` 看不到它；S2、S3 的部分照第 0 节。

### 3.24 S2 的实施对设计的修正

S2 的审查与修复见 [S2 的审查记录](reviews/P3-S2-playbook-docs-review.md)，结果在 10.2。与上文最初的设计不同之处已改在各节里（3.15–3.20，6.2，7.3、7.5、7.6）。写了行号的，是实施的头 `d2512eeb` 的；修复的提交见 10.2。

- **S2，实施时定下的**（实施者的 8 条偏离，括号里是它的号；三位审查者各在自己的范围里判过，都对：A 判第 3–5 条与第 2 条，B 判第 1、2、4、6、8 条，C 逐条判进不进本节。第 2 条不是与设计不同：3.16 写的就是 `wiki/domain/playbook_test.go`，括号里的 `playbooks_test.go:129-152` 是照着写的那几格（审查 A），不列；第 8 条"1、2 号提交的标题写行为的改变、1 号范围带 `build`"是过程，只记在 10.2（审查 C）；实施者没有要编号的产品决定）：
  - 3.15：`upgrade_submit` 的说明改在三处：契约的两个入口、`wiki/adapter/mcp/tools.go` 的常量、`wiki/adapter/mcp/testdata/tools.json` 的两个入口；3.15 的文件表只列了 `api/mcp/tools.json`。三处不一致时 `TestTheToolListsAreTheContract`（服务 ↔ 契约）或 `TestLintToolObjects`（服务 ↔ testdata）失败（实施者的 o14，审查 A 的 a4、a5，B 的 c1、c2）。措辞"an admin approves or rejects it in the web, or sends it back for more work."，与 playbook 的"sends it back for more work"一致（1）。
  - 3.18：模块根 `wiki/prompts.go` 导出 `UpgradePlaybookText`、`PlaybookText` 与 `Playbook` 的别名、三个常量：`TestUpgradePrompt` 要整值比 `UpgradePlaybookText()`，组合根只能 import 模块根（archtest"bootstrap imports only a module's root"）；先例是 `TemplatePages`。组合根拿它们作期望值，只验证"哪个 prompt 答哪份文字、参数怎么带"，文字本身由 `wiki/domain` 与 `wiki/app` 的单元测试验证（3；审查 A 判对，不是同义反复）。
  - 3.18 `TestUpgradePromptMatrix`：两份副本（`upgrading`、`reviewing`）在任何应用连上准备好的库之前做（`pgtest.NewDatabaseFrom` 要求没有人连着模板）；13 列落在五本笔记本上，`gone-nb` 已删，所以每份副本设四本，每本照 `upgradedBy(state)` 由它的创建人设一次，并断言副本里恰有四个这种状态的升级；`llm_wiki`、`standard` 两种状态照 `TestThePromptReadsLLMWikiMatrix` 在准备好的库上读；三个应用各建 13 个 PAT（PAT 存在各自的库里），应用在顶层起（4；审查 B 的 t3、t4 证明副本的自我核对是活的）。
  - 3.18 `TestUpgradePrompt`：审阅中由 alice 经 REST 真的提交（一页干净的笔记就能过闸门，存下报告）；回滚中以 SQL 设（同 `upgrade_start_test.go:358-360` 的做法，并断言行是 `rolling_back rolling_back true false true`）：真的驳回之后小笔记本几十毫秒就回滚完，`rolling_back` 与 `rolled_back` 答的是同一个码，钉不住要测的那一种状态（5）。审查 A 的 a1 说明真提交不可替代：矩阵与单元当时都只有"没有报告"的审阅中（P3S2A-N1，修复一轮在单元补"有报告"一格）。
  - 第 5 节、3.15：LW11 的 `textOf` 挪进 `e2e/fixtures/mcp.ts`，导出为 `promptTextOf`，UP9 共用；LW11 的断言不变（6）。
  - 3.19：README 写"两个入口另有 `upgrade_verify` 与 `upgrade_submit`"，另起一条"**升级**"放在"工具"一条之后：两个工具在两个入口都登记（第 196 条），3.19 写的"升级中的笔记本入口另有"不对（7；审查 C）。修复一轮另照审查 P3S2C-N2 写"笔记本的管理员在升级中用得上（`upgrade_verify` 审阅中也可）"。
- **S2，实施者自己加的**（实施报告第 8 节；审查者在各自的范围里看过都对）：`TestUpgradePlaybookText` 在 3.17 的"必有的句子"之外另测设计审查 P3C-N18 的三处与 3.17 末尾说逐项核对过的几样，另断言最后一句在末尾、文件本身带首尾空白（否则"去掉空白"的变异 o12 看不见）；用例的单元多三格（别的状态、`Current` 出错原样交回、`detail` 是普通的那一句）、替身的 `StateOf` 被调到会记下（钉住"不读模式"）；适配器的 `TestThePromptsAreTheFour`、`TestTheUpgradePromptPassesItsNotebook`；`TestUpgradePrompt` 的看不到的 carol、被拒的一次也有日志一行、日志的键整值比、`prompts/list` 在升级中的笔记本也比；矩阵另比 `description`、`prompts.json` 多一个没有期望的 prompt 就失败；UP9 另断言整个文字与 `description`；README 的 `permissions.allow` 样例加 `lint`、`pending_sources`、`upgrade_verify`（"读的十二个"之后样例还只有九个，人工清单的 D2 照样例写设置）；人工清单 A1、A3、A5、B1 的工具数（笔记本入口 16 → 18、账户入口 18 → 20，M11/P2 加了两个工具时没改）（审查 C 逐项数过）。
- **README 先写了 S3 才有的事**（实施报告第 9 节第 2 条）：3.19 的新一节写审阅中网页上由"驳回"代替与升级页上的"批准""退回继续修改""驳回"三个按钮，这些在 S3 才有；S2 合并到 S3 合并之间，`main` 的 README 比网页先走一步（审阅中网页上仍是"放弃"，三个动作只经 REST）。照第 207 条与 P2/S2 的同类处理接受（主持者），措辞与 4.3、决定 291 一致（审查 C 核过）。playbook 末段的"an admin reviews the upgrade in the web app"与 `upgrade_submit` 说明的"in the web, or sends it back"同样先于网页，属于同一类，同样接受（核对一轮的审查 C）。
- **S2，审查与修复带来的**（编号是 S2 的审查记录的；修复一轮照主持者的修复说明 `fix-1.md`，提交见 10.2）：
  - **playbook 的"必有的句子"钉做法**（P3S2B-M1 = P3S2C-M1，P3S2B-N1）：测试对每个片段只判 `strings.Contains`，片段多止于条件或码。00 号文档 4.13 清单里五项各自删掉照过（只改 frontmatter、来源移进来源根、归类、先 `upgrade_verify`、全过再 `upgrade_submit`），审阅中冻结的一句删掉也照过；`page.upgrade_immutable` 在两句里出现，删掉"只改 frontmatter"那一句之后，它的一格由"拆开仍是 `immutable`"那一句满足（实施者的 s16 把每一处都换掉，所以看起来抓到了）。保留条件、把做法换成错的（审查 C 的 c1–c7，其中两条正是设计审查 P3C-M6、N18 修过的那两句）或相反的（审查 B 的 pb01–pb20），都照过；UP9 与 `TestUpgradePrompt` 拿嵌入的同一份文字比，也看不见。3.17 的清单本身没有覆盖 4.13 的这几项，又止于码。修复一轮（`94db07e4`）：每格是 playbook 的一整句，一句里有两件事时是一个分句，4.13 清单的六项各有一格（`page.upgrade_immutable` 钉在"只改 frontmatter"那一句），每格断言恰好出现一次；`8321734b` 把 `notebook.upgrading` 那一句拆成两格之后共 47 格（实施报告写 46，是 `94db07e4` 上的数，审查 P3S2B-N2），合起来覆盖第一行之外的整段正文，改任何一句都要同时改测试里那一格（3.17 的"必有的句子"照改）。C 的 c1–c7、B 的 pb01–pb20、B 的逐句删除里照过的五段与 A 的 a1 共 33 条变异改前活、改后死（`8321734b` 之后 38 条，另 5 条盖新的措辞），B 的逐句删除 30 段改前 9 段照过、改后都失败。测试钉的是该有的句子在，不看有没有多出来的句子：新加一句相反的话照过（核对一轮的 P3S2B-N3），不改，新加的文字由审查看 diff。
  - **playbook 的两处措辞**（P3S2C-N3、N4）：闸门 4 写成"every page outside the source root has valid frontmatter, and every one but the source root and AGENTS has type and summary"（`invalid_frontmatter` 管来源区外的每一页，`missing_type`、`missing_summary` 不管来源根与根下 `AGENTS`）；`server_busy` 写成"server_busy from either tool means wait retry_after_seconds and call it again"，`forbidden` 并进"your user is no longer an admin"那一句："… or your user is no longer an admin of this notebook, and forbidden from upgrade_verify or upgrade_submit means your user is not an admin of it: stop and tell your user."（3.17 照改；`8321734b`，playbook 4,827 → 4,979 字节）。
  - **单元的"审阅中、有报告"一格**（P3S2A-N1，a1 在单元一层就死）与**注释**（P3S2A-N2：适配器的 `Prompts` 写笔记本入口的四个，组合根的 `TestTheWikiPromptsAreReached` 写 LLM Wiki 的三个、`upgrade` 由 `TestUpgradePrompt` 测）（`94db07e4`）。
  - **人工清单的 K**（P3S2C-M2、N5）：两个客户端各用自己的一本升级中的笔记本（同一位管理员各发起一次升级），K5 共用那本普通笔记本，照 J 加"逐客户端记"与"沿用上面隔离的客户端环境……不碰自己的配置"；K4 写明提交不过（`wiki.upgrade_not_ready`）时记什么、补齐再交（3.20 的块照改；`7daa3452`）。
  - **README 的三处**（P3S2C-N1、N2、N6）：第 2 步写"给来源区之外的 wiki 页补上 `type`、`summary`（依据来源的页另列 `sources`）"（设计 3.19 的原文就读来每页都要 `sources`，同设计审查 P3C-M6 第 2 处在 playbook 里修掉的那一种）；MCP 一节写"笔记本的管理员在升级中用得上（`upgrade_verify` 审阅中也可）"；"已有的笔记库"末句写"根下若有……"（3.19 照改；`7daa3452`）。
  - **决定 299**（P3S2A-Q1，主持者的决定）：`AGENTS` 的两份模板照旧写"三个 prompt"，S2 不改；升级中发起补写的 `AGENTS` 由握手的说明读到，中文按字面不实，进 M12 的打磨移交（本提交）。
  - **第三份**（P3S2B-Q1）：在 7763 上的样本记进 10.2，合并之后看两次 `main`，现在不挪。
  - **文档稿**（P3S2C-N7，本提交）：10.2 写明 README 的行号是哪一版的，`:214` 的数是一并改的，第 8 条只进 10.2。
  - **修复一轮与 `fix-1.md` 不同之处**（修复报告第 5 节）：片段走到整句或分句，不只延长修复说明点名的那几句（上面）；`forbidden` 一句写"is not"不写"no longer"（读者也取得到这份 playbook，`TestUpgradePrompt` 的 bob；他的 agent 调这两个工具答 `forbidden`，是因为从来不是管理员），写两个工具的名字不写"either tool"（这一段还没讲到 `upgrade_submit`，"either tool"无所指）；K 表后连 J 的第二句一起抄来（"发现 upgrade playbook 的措辞有偏差……不放松服务端的守卫"）；B 的 pb02、pb16 的原文随 `8321734b` 改写，在新原文上做同一种改法，另加 5 条盖新的措辞。都与修复说明的意思一致，审查核对过。
  - **修复核对一轮**：三位把各自的发现都关上；新的 Nit 五条：P3S2B-N2（格数是 47）与 P3S2C-N8–N10（文档稿：3.17 的块漏了 `forbidden` 那半句；3.20 的块还是初稿，照改在原处；决定 299 的一条理由"改模板要动 LW1 等读模板的断言"不实，没有断言钉这一句，去掉）在本提交；P3S2B-N3（新加一句相反的话照过）不改，见上。没有 Important、Minor 留下，不再有修复轮。
- **交给后面的**：`AGENTS` 模板的"三个 prompt"（第 299 条，M12 的打磨）；第三份贴线，以合并之后两次 `main` 为准（第 8 节第 1 条，10.2）；S3 的部分照第 0 节。

### 3.25 S3 的实施对设计的修正

S3 的审查与修复见 [S3 的审查记录](reviews/P3-S3-web-review.md)，结果在 10.3。与上文最初的设计不同之处已改在各节里（4.3，第 5 节，6.3，7.3、7.5、7.6，第 8 节第 6 条）。写了行号的，是实施的头 `05423d08` 的；修复的提交见 10.3。

- **S3，实施时定下的**（实施者的 9 条偏离，括号里是它的号；三位审查者各在自己的范围里判过：A 判第 1、4、5、7 条对、第 6 条不对，B 判第 1、2、8 条对、第 3 条的理由不成立，C 逐条判进不进本节。第 6 条不对，由修复一轮改代码，不列（下面"审查与修复带来的"第 1 条）；第 8 条"UP7 的 API、MCP 版本也在三种浏览器里跑"不是偏离：7.5 本就写"UP7 的整个文件（页面、API、MCP 三个版本）在三种浏览器里跑"，与它不一致的是第 5 节的"UP7（三种浏览器…）"与第 8 节第 6 条 S3 的那一句，两处照 7.5 改（审查 C）；第 9 条"提交的标题写行为的改变"是过程，只记在 10.3（审查 C）。实施者没有要编号的产品决定，"实施中定"的几条都记在本节）：
  - 4.3 `HealthScope`：提交的标识是这次提交的时刻 `submitted_at`，不是 `submission_id`：契约的 `Upgrade` 没有 `submission_id`（只有 `submitted_by`、`submitted_at`，`api/modules/wiki.yaml`），S3 不碰服务端；每次提交（接管也是）在笔记本行的锁下写新的 `now`（`upgrades.sql` 的 `SubmitUpgrade`），退回把它清成空、再交写新的，一次提交至多存一份报告（`KeepUpgradeReport` 要 `report IS NULL` 与这次的 `submission_id`），所以它就是"这一次提交"；同一次提交从"没有报告"到"有报告"键不变。"退回之后再交的新报告不接着旧的条目"由"the report's cards are its submission's"一格与变异 o05 钉住。4.3 的 `key`、`current()` 与 vitest 清单、00 号文档决定 287 写 `submission_id` 的四处照改（审查 P3S3C-N3；设计审查记录里 P3C-M4 的原话是历史，不改）（1）。
  - 4.3 假服务端：`reportJSON` 没有加形状：`upgrade-page.test.tsx` 已有的 `keptReport` 就是"每个值互不相同、带条目、`count` 比条目多"的报告；`upgrade-check.test.tsx` 要的报告（缺 type 七条、存五条）在测试文件里组，放进 `upgrade-server.ts` 只有一处用，knip 要它不导出（2）。
  - 第 5 节 UP7、UP8 的页面版本：批准那一页照第 5 节在笔记本首页放开流，之后等这次连接的最后一层（`GET …/assets`）答了，才断言连接没有读升级、UP7 才去下一页（`e2e/fixtures/upgrade-pages.ts` 的 `letInAtHome`：放开之前登记 `waitForRequest`，等它的 `response()`）。第 5 节没写这个"等"：不等的话，下一次载入会截断还在飞的读（WebKit、Firefox 的 `Load request cancelled`），断言也可能早于重连的读（3，修复一轮重写）。经过：实施时照第 5 节在首页放开、不等，WebKit 失败一次，改为撤销那一段之后才放开，说"放开之后的重连只重读装着的键"照样看得见；审查 B 判这个理由不成立：`page.goto` 是一次文档载入，SWR 的缓存每个文档一份，变更集页的新文档从没装过升级页，"没有读升级"无论怎样都成立（P3S3B-I1，下面）。修复一轮改回首页放开、加上等；UP7 三种浏览器本机 12 次、核对时 27 次都过。
  - 4.3 树读失败：提示在列节点的地方之上（设计写"整节的顶上一处"）：检查一节在按"检查"、读到检查之后才列闸门与条目，提示在闸门之上；审阅一节在存下的报告之下、闸门之上；按"检查"之前与没有报告时没有节点要说（4）。重试时"读着"先于"失败"：`tree.error` 还在、`isValidating` 为真，格子写"加载中"、提示收起，读完仍失败才空着，`HealthItem`、`GateNode`、`TreeFailure` 三处判法一致；变异 o09 由重试挡住的一格钉住（5）。
  - 4.3 审阅一节：卡的"更多"被拒为 `wiki.upgrade_state` 时写"升级的状态变了"并重读升级，同检查一节；设计只写了重读升级（7）。
  - 4.3 三个动作："退回继续修改…"是 `outline`（批准是默认的实心、驳回是 `destructive`），三个按钮的轻重照后果排，与回滚一节"继续" `outline`、"结束" `destructive` 一致（实施中定，审查 C）。
- **S3，实施者自己加的**（实施报告第 8 节；审查者在各自的范围里看过都对）：UP6 的页面版本重新打开之前先等页面经 `mode` 显示新的报告（不然发起人经 API 再交的 `mode` 的重读可能被 `page.goto` 截断，Chromium 记成 `net::ERR_ABORTED`）；20 秒的计时器的两格（第一轮变异 s3-12b 活下来之后补：卸下之后 SWR 绑定的 `mutate` 对没有 hook 挂着的键什么也不做，设计的"卸下之后不再读"看不见计时器没清，改成"离开之后 5 秒再回来：打开时读一次，之后 60 秒不再读"，另加"`mode` 带来报告之后不再读"一格，s3-12c、s3-12d 各被一格抓到）；树的重试挡住的一格（o09）；换了一次提交之后才到的 403 不重读（o29）；断网时退回不挂计时器（o30）；store 的"不重读"按动作各列真服务端答得出的码（批准 `not_ready`、503、断网，驳回 503、断网，退回断网）；e2e 的帮手 `changeButtons`、`reviewDialog`（`fixtures/upgrade-pages.ts`）。实施时 UP7 的页面版本末尾另有一段"最后才放开、断言不读升级"，修复一轮删去（上面第 3 条）。
- **S3，审查与修复带来的**（编号是 S3 的审查记录的；修复一轮照主持者的修复说明 `fix-1.md`，提交见 10.3）：
  - **退回的说明在拒绝的那一次重读回来时定一次**（P3S3A-I1 = P3S3C-M1；实施者的第 6 条）：实现的 `explain` 每次渲染读页面此刻的升级：重读回来之前先按旧的说（别人刚驳回时先写"正在计算"），之后报告经 `mode` 或 20 秒的再读到了，开着的对话框改说"状态不允许"，而这时恰恰可以退回；`Alert` 是 `role="alert"`，读屏先后读出两句矛盾的话。修复一轮：`change` 对这几个码等它的那一次重读回来再抛出，不另发第二个读；`sendBack` 按重读到的升级判，"审阅中、没有报告"时记下这一次拒绝与它的提交（`submitted_at`）、挂计时器；说明只认这一次拒绝，重读失败或读到别的状态时写通用的文字；20 秒只在同一次提交仍"审阅中、没有报告"时挂（P3S3A-N1 = P3S3C-N2：次数在状态变了之后不清，之后又回到"审阅中、没有报告"时没有新的拒绝也再读一次）；重读答 404 时不是"正在计算"（P3S3C-N1，c21）（4.3 照改；`a040dcc1`）。说明从 SWR 的缓存里读这一次重读的结果（组件的 `data` 在闭包里是旧的），记下的是那个 `ApiError` 对象与这次提交的 `submitted_at`，下一次退回开始时清掉两者。变异：c21、ra05 改前活、改后死；A1（`explain` 按此刻）、A2（`change` 不等重读）、A3（计时器不看 `submitted_at`）改后死（改前是缺的行为，由 A、C 的临时测试复现）。ra05（退回开始时不清）第一轮判作"照 N1 改了之后等价"，改后仍活，不是等价的：被拒、计时器挂着，再按一次退回、这次断网，留着旧的提交，到点多读一次；补"refused, then sent back again, the network failing"一格。核对一轮另查出一个窄的竞态：拒绝之后的重读被 `mode` 事件的读超过时，SWR 丢掉先发的那一个的答复，`catch` 读到的缓存仍是旧的"审阅中、没有报告"，对话框一直写"正在计算"（P3S3A-M3）；修复二轮：这一项仍 `isValidating`（有更新的读在飞）时再等一次重读、再读缓存（`a25fef6d`）。
  - **批准之后重连不读升级要看得见**（P3S3B-I1）：UP8 在首页放开流之后等这次连接的最后一层读答了，再断言放开之后没有 `GET …/upgrade`、`apiFailures` 为空；vitest 的"approved"一格带事件流，批准之后发 `mode`、断开重连，请求记录仍只有批准之前的；UP7 见上第 3 条（第 5 节照改；`bc1a24e5`，vitest 那一格在 `a040dcc1`）。B 的 `leak` 改后在 UP8 与 UP7 三种浏览器 4 个全红（红在 `not.toContain(GET …/upgrade)`），vitest 里它挂着页面时就多读、改前改后都被原有的格抓到；另一条只在卸下之后才读的 `leak-after-unmount`（B 的 `leakA`、`leakM` 同类）只有新的"approved"一格抓得到；o27（批准之后重读升级）在 e2e 照旧 4 个全红。
  - **三处缺测试**：批准答复时页面已离开就不跳转（P3S3A-M1 = P3S3B-M2，去掉 `mounted()` 照过）；离开页面时审阅一节的"更多"被中止（P3S3A-M2 = P3S3B-M3，`controllers` 换成不登记照过）；驳回、退回先盖住答复再重读（P3S3A-N4 = P3S3B-M1，`change(…, false)` 照过）。修复一轮各补一格（4.3 的测试清单照改；`a040dcc1`）：ra01（A5）、ra04、ra10、ra11 改前活、改后死。
  - **批准之后按"后退"**（P3S3A-N2）：以 `mutate(["upgrade", id], undefined, { revalidate: false })` 清掉这一项，按"后退"回来不再先显示审阅中的旧按钮（4.3；`a040dcc1`，A4 一格；那一格批准之前等 2.1 秒，SWR 的 `dedupingInterval` 2 秒内不重发同一个键）。修复一轮在 `await navigate(…)` 之后就清，说那时页面已卸下；核对一轮查明不成立：react-router 以 `startTransition` 提交新的路由，`navigate` 回来时升级页还挂着，清掉之后它闪成"加载中"（P3S3A-N5）；修复二轮改为批准成功、页面还装着时先记下，`UpgradeRead` 卸下时才清（`a25fef6d`）。
  - **假服务端的提交每次给新的 `submitted_at`**（P3S3A-N3：S3 以它认"这一次提交"，写死同一个值时"同一次进入里提交、退回、再提交"的格子会接着旧的条目；从 `2026-10-06T09:40:00Z` 起每次加一分钟）（4.3；`a040dcc1`）。
  - **UP6 末段的注释**照实写"离开之后才放开"（P3S3B-N1）；**驳回之后断言任务的整行**（P3S3B-N2：名字、状态、开始的时刻、"取消"；驳回是本账号发起的，行里本就不写"由谁"）（`a040dcc1`、`bc1a24e5`）。
  - **决定 300**（P3S3A-Q1，主持者的决定）：别的管理员退回之后，检查一节同样清掉提交之前的结果；只在回到 `upgrading` 的那一次读之前显示过报告、或 `submitted_at` 换过时清，提交被拒为 `not_ready`、`stale_index` 时照旧保留 `recheck` 的那一代（4.3；`a040dcc1`，A6 一格；A6（不清）、A6b（回到 `upgrading` 一律清）改后都死，后者死在 `upgrade-check.test.tsx` 别人的提交拿走又还回检查一节的两格）。"显示过报告"按这一次进入的页面读到的算：页面只读到"审阅中、没有报告"就看到 `upgrading`，分不出是提交被拒还是过了接管时刻的退回，不清（审查 A 判对，是这一判法固有的边界）。"换过提交"一支没有测试（核对一轮 P3S3A-N6，rb04 活），修复二轮补一格（`a25fef6d`）。
  - **决定 301**（P3S3C-Q1，主持者的决定）：健康度页在树第一次读失败时条目不再一直写"加载中"：`readingTree` 照升级页加 `tree.error === undefined`，交 `treeFailed={tree.error !== undefined}`（4.3 那一句"健康度页不交 `treeFailed`，照旧"照改；`a040dcc1`，REVIEWC-H 一格，H1（不交）、H2（`readingTree` 不看 `tree.error`）改后死）。交了 `treeFailed`，树**再读**失败时也一样："更多"带来、树里没有的节点同样空着，不再写"节点已不可用"（树没读到，说它不可用是假话，同 P3C-N12）；`health-page-lifecycle.test.tsx` 的"missing More nodes…"一格随之改（逐个提交的核对抓到的；核对一轮 P3S3C-N7 要文字写上这一半）。
  - **文档稿**（本提交）：`submission_id` 的四处（P3S3C-N3，见上第 1 条）；v0.1 总体设计 13.2 第 18 条加审阅一节"更多"的 403、第 9 条的括号写成不在首页放开的理由、第 15 节加一行（P3S3C-N4）；"No issues (1 checks)"进 M12 的打磨移交第 41 条（P3S3C-N5）。
  - **修复一轮与 `fix-1.md` 不同之处**（修复报告第 5 节；审查 A 判第 1、3 条，C 判第 4 条）：所有经 `change` 的对话框（放弃、接着做、结束回滚、提交、驳回、退回，六个；修复报告写五个，漏了结束回滚，审查 A）遇到 `transfer.busy`、`wiki.upgrade_state`、`wiki.upgrade_not_ready`、`wiki.upgrade_stale_index` 都等重读回来再抛出，按钮的"正在…"到那时才收起，取消与 Esc 也挡着；批准走自己的函数，不等。修复说明要的是退回，代价是拒绝的文字晚一次读出现，重读挂住时对话框跟着挂住（与 POST 挂住同一类），好处是文字与身后的页面一致，P1、P2 的格都照过（审查 A 判可以接受）（1）。批准之后先跳转、再清缓存（2；核对一轮查明会闪，修复二轮改，见上）。决定 300 的"显示过报告"按这一次进入读到的算（3，见上）。决定 301 也管树再读失败（4，见上）。另：ra05 不是等价的，补了一格（见上）。
  - **修复核对一轮**：A 判 I1、M1、M2、N1–N4 关上、决定 300 做对了，新的 P3S3A-M3（Minor：拒绝之后的重读被 `mode` 的读超过时说明读到旧的缓存）、N5（Nit：批准之后清缓存时升级页还挂着、闪成"加载中"）、N6（Nit：决定 300 的"换过提交"没有测试），由修复二轮改（`fix-2.md`）；B 判 I1、M1–M3、N1、N2 都关上，UP7 三种浏览器 27 次全过，`leak` 与它的两种变体 `leakA`、`leakM` 改前活、改后死，新测试没有同类的空断言，没有新发现；C 判 M1、N1、N2、Q1（决定 301）关上，N3–N5 在文档稿里关上，新的 P3S3C-N6–N9 都在文档稿（UP7 的说法照修复之后写、决定 301 与 4.3 加上树再读失败、4.3 写明六个对话框都等重读、M12 第 41 条不用 `W/`），在本提交。
  - **修复二轮**（`fix-2.md`，P3S3A-M3、N5、N6，一个 `web` 提交 `a25fef6d`）：退回被拒之后，缓存里这一项仍 `isValidating` 时再 `await reread()` 一次（新发的读比 `mode` 的那一次新，答复不会被丢），再读缓存、定说明；批准成功、页面还装着时记下 `approvedAway`，`UpgradeRead` 一个只有清理的 effect（依赖在这一页的一生里不变，只在卸下时跑）为真时清缓存；决定 300 的"换过提交"一格（审阅中没有报告，经 `mode` 换了提交，再经 `mode` 回到 `upgrading`）。变异：rb04 与"清缓存挪到跳转之前"改前活，A 的 A8 原样在 `bc1a24e5` 上过、A10 失败；改后 6 条全死（不再读一次、A8 原样的期望、照修复一轮在跳转之后清、跳转之前清、从不清、rb04）。修复报告另更正修复一轮第 5 节第 1 条：受影响的是六个对话框（上面）。
  - **修复核对二轮**（审查 A）：P3S3A-M3、N5、N6 关上，A8、A10、rb04 改前活、改后死，新加的 4 条变异都被抓到；"再读一次"只剩第二个 `mode` 又晚于这次再读的更窄的一种，不再追；卸下时才清的 effect 只在卸下时跑，跳转被打断时留到离开时清，也对。新的 P3S3A-N7（Nit："再读一次"不看拒绝的码，断网等拒绝碰上有读在飞时也多读一次升级）不改：只在断网而有读在飞时多读一次，无害。没有 Important、Minor 留下，修复到此为止。
- **交给后面的**：英文的"1 checks"（M12 的打磨第 41 条）；P3 的完成由之后的 `docs: M11/P3 done` 写（第 8 节第 5 条）；M11 收尾照第 9 节。

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
| `W/pages/notebook/health-page.tsx` | `paths` 的参数放宽；`read` 在没有游标时抛缺陷（下面）；树读失败时交 `treeFailed`、`readingTree` 不再恒真（决定 301，修复一轮） |
| `W/pages/notebook/health-rule.test.tsx` | 替身的 `read` 的类型（`:416`）；`undefined` 游标的几格 |
| `W/pages/notebook/health-item.tsx`、`W/pages/notebook/upgrade-check.tsx` | 树读失败时路径一格空着（`treeFailed`）；`UpgradeReportSection` 经 `HealthRule`、它的 `HealthScope`；`itemsReader` |
| `W/pages/notebook/upgrade-page.tsx` | 审阅一节挪进这一次进入的 `SWRConfig`；三个动作与批准自己的函数；审阅中不给"放弃"（决定 291）；退回的说明与 20 秒的再读；退回之后清掉 `checked` |
| `W/pages/notebook/upgrade-banner.tsx` | `reviewing` 时管理员那一句写"驳回"（决定 291） |
| `W/services/upgrade.service.ts`、`W/stores/notebook.store.ts` | 三个动作；`checkItems` 的游标可缺 |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | 三个按钮、三个对话框、`upgrade.approveNotReady`、`upgrade.sendBackChecking`、`upgrade.bannerAnyAdminReject`、`upgrade.approveRootUnread`；`upgrade.startBody` 的"批准或放弃"改为"批准、驳回或放弃"（en"approved, rejected or abandoned"，`en.ts:704-705`、`zh-CN.ts:680`；设计审查 P3C-N23） |
| `W/pages/notebook/general-page.test.tsx` | `:439` 断言发起对话框的正文，随 `upgrade.startBody` 改 |
| `W/test/upgrade-server.ts` | 三条路由；提交路由每次给新的 `submitted_at`（修复一轮）；`reportJSON` 不加形状（3.25） |
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
//   treeFailed?: boolean; // passed on to each HealthItem as is (:223-232); the health page passes its tree's too (decision 301)
```

"更多"照旧在 `card.cursor !== null` 时渲染（`:238`），读之前同样判（`:178`），把 `card.cursor`（可能是 `undefined`）交给 `read`。从第一页读到的前几项与存下的重复，`HealthRule` 已按 `item_key` 去重（`:155-160`、`:181`、`:194`），不多出行。`HealthRule` 内部记下的读的游标（`operation` 的 `cursor`，`:107`）随之是 `string | undefined`。健康度页随之改两处（审查 C 只改类型、`tsc --noEmit` 报出三处，设计审查 P3C-M3；初稿说"行为不变、tsc 守着"不对）：`paths`（`health-page.tsx:247`）的参数放宽为 `readonly HealthRuleItem[]`（`healthNodeId` 本就收 `UpgradeReportItem`，`health-item.tsx:17`）；`read`（`:248`）在游标为 `undefined` 时抛一个缺陷（`throw new Error("a health card reads on from its cursor")`），不放宽 `WikiService.items`（`W/services/wiki.service.ts:21`）：健康度页的卡的游标只会是 `string | null`，第一页在摘要里，抛缺陷守住 P2 4.3 的这个用意（主持者倾向的那一种）。`health-rule.test.tsx:416` 的替身的 `read` 的类型随之放宽。检查一节的卡的游标是 `string | null`，行为不变。`UpgradeService.checkItems`（`W/services/upgrade.service.ts:69`）的游标改为可缺，缺时不带 `cursor` 这个查询参数（契约里它本就可缺，`api/dist/openapi.yaml` 的 `listUpgradeCheckItems`）。

**审阅一节**（`UpgradeReportSection`，`W/pages/notebook/upgrade-check.tsx:238-286`）：

- 规则改由 `HealthRule` 渲染（`level={3}`、`mode="llm_wiki"`、`moved="wiki.upgrade_state"`、`paths={noPaths}`，同检查一节的 `card()`，`:170-199`），替掉自己渲染的 `h3`、数与 `HealthItem`（`:261-281`）。卡的初值：`items` 是存下的条目；`cursor` 在 `count > items.length` 或 `count_capped` 时是 `undefined`，否则是 `null`；其余成员照检查一节的 `card()`。规则照检查一节分两组（`:167-169`、`:219-224`）：计数大于 0 的照 `healthRuleOrder` 列出，计数为 0 的收进 `<details>`（`health.clear`）。卡的"更多"被拒为 `wiki.upgrade_state` 时写"升级的状态变了"并重读升级，同检查一节（S3 的实施，3.25）。
- 挪进这一次进入的 `<SWRConfig value={checksConfig}>`（`upgrade-page.tsx:300` 那一个；P2 第 9 节第 4 条）：卡的状态与"更多"读到的条目随这一次进入，离开升级页再回来从存下的报告重新开始。
- 审阅一节的 `HealthScope`（`HealthRule` 的键、"更多"的取舍与 403 的重读都靠它，`health-rule.tsx:106-137`；设计审查 P3C-M4）：
  - `key`：`["upgrade-report", store.loginId, workspace.id, notebook.id, upgrade.id, upgrade.submitted_at]`。不带这一次进入的 `locationKey`、`nonce`（检查一节带，`upgrade-check.tsx:93-101`）：缓存已是这一次进入的（`checksConfig`），键里不再带它，"审阅一节留在 `SWRConfig` 之外"的变异才会让离开再进入接着旧的条目、被那一格抓到。带这次提交的时刻 `submitted_at`（契约的 `Upgrade` 没有 `submission_id`；每次提交、接管也是，在笔记本行的锁下写新的 `submitted_at`，一次提交至多存一份报告；S3 的实施，审查 P3S3C-N3，3.25）：退回之后再提交存下的新报告不接着旧的条目。
  - `current()`：这一次进入还活着（`entry.control.current.alive`）、登录没变（`store.auth.state.loginId === store.loginId`）、显示的升级的 `submitted_at` 等于键里的（每次渲染写入的 ref）。不看检查一节的 `generation`：审阅中没有检查。
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
- 树读失败（P2 第 9 节第 7 条）：`UpgradeRead` 把 `tree={{ error, reading, retry }}` 交给检查与审阅两节，照健康度页（`W/pages/notebook/health-page.tsx:51`、`:60`、`:245`、`:292-294`）：`reading` 是 `pages.nodes === undefined && tree.error === undefined || tree.isValidating`，失败时列节点的地方之上一处显示"加载失败"与"重试"（`fieldset aria-label="Page paths"` 里的 `NotLoaded`，同健康度页；检查一节在闸门之上，审阅一节在存下的报告之下、闸门之上：按"检查"之前与没有报告时没有节点要说，3.25），不再一直写"加载中"；闸门的节点（`GateNode`，`upgrade-check.tsx:384`）与条目的路径（`HealthItem`，`health-item.tsx:64`）收一个可缺的 `treeFailed`，为真时那一格空着，不写"（已不在）"（`upgrade.gateNodeGone`）、"节点已不可用"（`health.nodeUnavailable`）：树没读到，说节点不在是假话（设计审查 P3C-N12）。检查与审阅两节的条目都经 `HealthRule` 渲染，`HealthRule` 给 `HealthItem` 今天只交 `readingTree`（`health-rule.tsx:223-232`），所以 `HealthRule` 的 props 加可缺的 `treeFailed`、原样交给 `HealthItem`（设计审查 P3C-N20）。健康度页同样交：`readingTree` 照升级页加 `tree.error === undefined`，交 `treeFailed={tree.error !== undefined}`（决定 301，修复一轮；初稿写"健康度页不交 `treeFailed`，照旧"，它的树第一次读失败时 `pages.nodes` 一直没有，条目一直写"加载中"，审查 P3S3C-Q1）；树再读失败时同样，"更多"带来、树里没有的节点空着，不写"节点已不可用"（树没读到，说它不可用是假话，同 P3C-N12）。重试时 `tree.error` 还在、`isValidating` 为真，"读着"先于"失败"：格子写"加载中"、提示收起，读完仍失败才空着（S3 的实施，3.25）。
- 没有报告时照旧写"正在核对，或上一次提交没有做完。"（`upgrade.reportPending`）。

**三个动作**（第 288、291 条）：`reviewing` 时按钮一行（`upgrade-page.tsx:280-291`）依次是"批准…"（en"Approve…"，只在有报告时）、"退回继续修改…"（"Send back for more work…"）、"驳回…"（"Reject…"）；不给"放弃这次升级…"（决定 291：今天 `:280-291` 在 `reviewing` 也给它，这一处改为只在 `upgrading`）。`upgrading` 照旧是"提交审阅…"与"放弃这次升级…"。对话框都是 `ConfirmDialog`，`held={dialogOf(which)}`（`:228-234`），`type Change`（`:64`）加 `"approve" | "sendBack" | "reject"`：

| 动作 | 语气 | 标题（en / zh） | 正文（zh） | 确认按钮（en / zh） |
|---|---|---|---|---|
| 批准 | `default` | Approve the upgrade of “{name}”? / 批准「{name}」的升级？ | 笔记本将成为 LLM Wiki：来源区（{root}）下的页与附件只读，`AGENTS` 与来源根受保护，生成的目录与日志出现在页面树里。批准之后没有整体的撤销：升级的变更集仍可在历史里照 LLM Wiki 的规则逐个撤销；要回到普通笔记本用降级。 | Approve the upgrade / 批准升级 |
| 退回 | `default` | Send back the upgrade of “{name}”? / 退回「{name}」的升级？ | 升级回到进行中，发起人的 agent 可以接着改；这次的报告不再保留，之后要重新提交。 | Send it back / 退回 |
| 驳回 | `destructive` | Reject the upgrade of “{name}”? / 驳回「{name}」的升级？ | 照 `upgrade.abandonBody` 的说法（`zh-CN.ts:699-700`、`en.ts:726-727`；决定 291 的理由是两者后果相同，文字也相同）："一个后台任务把笔记本复原到升级开始时的样子：原有的页与附件回到原处，名称与正文复原，之后新建的进回收站。回滚结束之前，笔记本照旧只读。"再接一句"回滚停下时可以继续，或者结束回滚、保留当时的样子。"（设计审查 P3C-N22：初稿写"agent 新建的页进回收站"，漏了发起补写的 `raw`、`AGENTS` 与附件、文件夹） | Reject the upgrade / 驳回升级 |

`{root}` 是提议的来源根的路径（`nodeTarget(pages, upgrade.source_root_id)`）；树还没读到或读失败时写"提议的来源根"（`upgrade.approveRootUnread`，en"the proposed source root"），不等树（设计审查 P3C-N13）。中文标题照升级的另四个对话框（`upgrade.startTitle`、`abandonTitle`、`continueTitle`、`endTitle`，`zh-CN.ts:678`、`:698`、`:711`、`:716`）用「」，英文照旧用 “”（设计审查 P3C-N21）。"生成的目录与日志出现在页面树里"照降级的说法（`zh-CN.ts:654`、`en.ts:678` 的"生成的目录与日志从页面树里消失"）。

拒绝的文字：驳回与退回经 `change`（`:176-204`），它对 `wiki.upgrade_state`、`wiki.upgrade_not_ready` 重读升级再抛出（`:183-192`），对话框照 `problemMessages` 显示。批准有自己的函数（设计审查 P3C-M2：`change` 成功之后在页面还装着时一定 `reread()`、`shows` 时还先把答复写进缓存，`:198-203`，批准经它就在跳转之前多读一次升级、答 404，UP7、UP8 会因为未声明的 404 失败，`holdStream` 挡不住它）：拒绝那一支照 `change`（`wiki.upgrade_state`、`wiki.upgrade_not_ready` 重读升级再抛出）；成功那一支不写缓存、不 `reread`，只在 `mounted()` 时跳转（下面）。批准答 `wiki.upgrade_not_ready` 时对话框显示 `upgrade.approveNotReady`（zh"批准前重新核对时有检查没有通过，升级仍在审阅中：可以退回继续修改，或者驳回。"，en"Checked again before the approval, a check did not pass; the upgrade is still in review: send it back, or reject it."；批准的 `texts` 照提交的 `:343-353` 写）：`upgrade.notReady` 与通用的 `problem.wiki.upgrade_not_ready` 都写"已回到升级中"，批准时不实（同 2.1 第 4 条在服务端改掉的那一句；设计审查 P3C-M1）。`server_busy` 显示通用的忙的文字，不自动重试（同发起）。

**退回在提交正在计算时**（00 号文档 4.15，核对 C3-M2）：退回答 409 `wiki.upgrade_state`、而 `change` 重读之后的升级仍是"审阅中、没有报告"时（经 `ConfirmDialog` 的 `explain` 按重读之后的状态定，不按打开对话框时的：409 也可能来自别的管理员刚驳回或退回；设计审查 P3C-N14。所有经 `change` 的对话框（放弃、接着做、结束回滚、提交、驳回、退回，六个）遇到 `transfer.busy`、`wiki.upgrade_state`、`wiki.upgrade_not_ready`、`wiki.upgrade_stale_index` 都等它的那一次重读回来再抛出，不另发第二个读，按钮的"正在…"到那时才收起；批准走自己的函数，不等（修复一轮，3.25）。`sendBack` 从缓存里读这一次重读的结果，"审阅中、没有报告"时记下这一次拒绝与它的提交（`submitted_at`）；这一项仍 `isValidating`（拒绝之后的重读被 `mode` 事件的读超过，SWR 丢掉先发的那一个的答复）时再等一次重读、再读缓存（审查 P3S3A-M3，修复二轮；"再读一次"不看拒绝的码，断网等拒绝碰上有读在飞时也多读一次升级，无害，审查 P3S3A-N7，不改）；说明只认这一次拒绝，之后的读不再改它，重读失败（例如答 404）或读到别的状态时写通用的文字。S3 建成时 `explain` 每次渲染读页面此刻的升级：重读回来之前先按旧的说，报告到了之后改说"状态不允许"，`role="alert"` 先后读出两句矛盾的话；审查 P3S3A-I1 = C-M1、C-N1，修复一轮，3.25），对话框显示自己的文字 `upgrade.sendBackChecking`（zh"上一次提交正在计算，稍后再试。"，en"The previous submission is still being checked: try again in a moment."），不显示服务端英文的 `detail`；`change` 已重读升级；另在 20 秒之后再自动重读一次（`setTimeout`，只在被拒的那一次提交仍"审阅中、没有报告"时挂，`UpgradeRead` 卸下、升级的状态变了或下一次退回时清掉，状态变了之后不再挂（S3 建成时次数不清，之后又回到"审阅中、没有报告"时没有新的拒绝也再读一次，审查 P3S3A-N1 = C-N2，修复一轮）；20 秒是 `LintTimeout + 10 s` 的上界，网页拿不到实例的 `LintTimeout`）。有报告时的 409（例如别人已经批准）照通用的文字。

**批准之后**（第 289 条）：`store.approveUpgrade` 成功之后，若页面还装着（`useMounted`，`W/app/mounted.ts:12`），`navigate(\`/${slug}/notebooks/${id}\`, { state: arrived })`，笔记本首页的标题接到达的焦点（`notebook-home.tsx:34`、`:44`）；对话框随页面卸下，不走 `onClosed` 的焦点。不再读 `["upgrade", id]`（批准之后它答 404 `wiki.upgrade_not_found`）。批准成功、页面还装着时先记下，`UpgradeRead` 卸下时（只在卸下时跑的 effect 的清理里）以 `mutate(["upgrade", id], undefined, { revalidate: false })` 清掉这一项：没有装着的 hook，不发请求，按"后退"回到升级页不再先显示审阅中的旧按钮（审查 P3S3A-N2，修复一轮；修复一轮在 `await navigate(…)` 之后就清，react-router 以 `startTransition` 提交新的路由，那时升级页还挂着，闪成"加载中"，审查 P3S3A-N5，修复二轮改为卸下时清；清理所在的 effect 的依赖在这一页的一生里不变，只在卸下时跑）。驳回与退回留在升级页，照现有的 `change` 盖住答复再重读，焦点照 `dialogOf` 回到按钮或标题。退回成功之后清掉这一次进入的 `checked`（`upgrade-page.tsx:115`，`setChecked({ generation: 0, upgrade: "" })`），检查一节回到"还没有检查"：不显示提交之前缓存里的结果，那容易让人误读（设计审查 P3C-N15，主持者倾向的那一种）。别的管理员退回（页面经 `mode` 看到）同样清掉：只在回到 `upgrading` 的那一次读之前显示过报告、或 `submitted_at` 换过时清；提交被拒为 `not_ready`、`stale_index` 时也从审阅中回到 `upgrading`，那时照旧保留 `submit` 故意 `recheck` 的那一代（决定 300，修复一轮）。"显示过报告"按这一次进入的页面读到的算：页面只读到"审阅中、没有报告"就看到 `upgrading` 时，分不出是提交被拒还是过了接管时刻的退回，不清。

**横幅**（决定 291）：`upgrade-banner.tsx:42` 的管理员那一句按状态：`upgrading` 照旧 `upgrade.bannerAnyAdmin`；`reviewing` 用新的 `upgrade.bannerAnyAdminReject`（zh"任何笔记本管理员都可以驳回这次升级。"，en"Any admin of the notebook can reject this upgrade."）；`rolling_back` 照旧不写（决定 246）。

**store 与 service**：

- `UpgradeService`（`W/services/upgrade.service.ts`）加 `approve(notebookId)`、`reject(notebookId)`、`sendBack(notebookId)`，各 `POST` 它的路径、答 `Upgrade`；`checkItems` 的游标可缺（上面）。
- `NotebookStore`（`W/stores/notebook.store.ts`）的构造参数的 `Pick`（`:54`）加 `"approve" | "reject" | "sendBack"`；`approveUpgrade(id)`、`rejectUpgrade(id)`、`sendBackUpgrade(id)` 经 `inTurn` 与 `upgraded`（`:195-207`），拒绝之后要重读列表的码：三个都是 `forbidden`、`notebook.not_found`、`wiki.upgrade_state`（同 `behindRollback`，`:310`）。批准不收 `wiki.upgrade_not_ready`：它只在缺陷时到得了，列表也不变（同 `behindSubmit` 不收它；设计审查 P3C-N11）。成功之后照 `upgraded` 重读列表（批准之后列表里这本笔记本是 LLM Wiki、没有升级，横幅随之消失）。

**假服务端**（`W/test/upgrade-server.ts`，05-lessons 第 2 节：只答真服务端答得出的码，码序照真序）：

- `POST …/upgrade/approve`：不是管理员 403；不是 `reviewing` 或没有报告 409 `wiki.upgrade_state`；可配的结局 `not_ready`（409 `wiki.upgrade_not_ready`，`detail` 照服务端的句子）与 `busy`（503 `server_busy`、`Retry-After: 1`）；否则 200 `approved`，笔记本的 `mode` 改为 `llm_wiki`、`upgrade` 为 `null`，发一次 `mode`。
- `POST …/upgrade/reject`：403；不是 `reviewing` 409；否则 200 `rolling_back`、`ending: "rejected"`、带 `rollbackJobJSON`（`:159`），发一次 `mode`。
- `POST …/upgrade/send-back`：403；不是 `reviewing` 409；没有报告而没有设"已过接管的时刻"时 409 `wiki.upgrade_state`、`detail` 是"The previous submission is still being checked: check again in a moment."；否则 200 `upgrading`、提交与报告清空，发一次 `mode`。
- `GET …/upgrade/check/{rule}`：不带游标时答第一页、`reviewing` 时也答，今天已是如此（`upgrade-server.ts:350-356`、`:470`），不改（设计审查 P3C-N7）。
- `reportJSON`（`:130`）不加形状：`upgrade-page.test.tsx` 已有的 `keptReport` 就是"每个值互不相同、带条目、`count` 比条目多"的报告，`upgrade-check.test.tsx` 的报告在测试文件里组（S3 的实施，3.25；初稿写"加一个……的形状"）。
- 提交路由每次给新的 `submitted_at`：审阅一节以它认"这一次提交"（S3 建成时写死同一个值，审查 P3S3A-N3，修复一轮）。

**测试**（vitest；夹具的标量互不相同、整值断言，05-lessons 的"投影的测试"）：

- `health-rule.test.tsx`：游标为 `undefined` 的卡渲染"更多"、点它以 `undefined` 调 `read`、第一页里与已有条目同 `item_key` 的不重复、之后照答复的 `next_cursor` 继续；游标为 `null` 的不渲染、不读；`UpgradeReportItem` 的条目照样渲染（没有 `cursor` 成员）。
- `upgrade-check.test.tsx`：审阅一节经 `HealthRule` 渲染每条规则（数、至多五项）；"更多"从第一页读、请求不带 `cursor`；读到的追加在存下的之后；条目已是全部的规则没有"更多"；审阅一节的"更多"答 403 时重读笔记本列表（且只在这一代当前时）；检查一节的 403 一格照旧；树读失败时两节显示"加载失败"与"重试"，点重试之后读出路径；离开再进入，审阅一节从存下的报告重新开始（这一次进入的缓存）；退回之后再提交的新报告不带旧的条目（键带 `submitted_at`）；离开升级页时审阅一节的"更多"被中止（审查 P3S3A-M2 = B-M3，修复一轮）；计数为 0 的规则在 `<details>` 里；树读失败时闸门的节点与条目的路径一格空着、没有"（已不在）"与"节点已不可用"。
- `upgrade-page.test.tsx`：审阅中有报告时三个按钮依次在、没有"放弃这次升级…"（`:175-196` 的 `reviewing` 一行改为断言没有"放弃"；`:856-867` 的 `sectionsInOrder` 的按钮表加三个名字，审阅一行是 `["Where it is", "Check report", "Approve…", "Send back for more work…", "Reject…"]`）；没有报告时没有"批准"、有另两个；`upgrading` 照旧有"放弃"；每个对话框的语气、标题与正文（树没读到时 `{root}` 是"提议的来源根"）；批准成功之后在笔记本首页、焦点在标题、不再读升级（请求的记录里批准之后没有 `GET upgrade`）；批准答 `wiki.upgrade_not_ready` 时对话框留着、显示 `upgrade.approveNotReady`、升级被重读；退回成功之后检查一节没有提交之前的结果；退回答 409 而重读之后已不在审阅中时显示通用的文字、不是 `sendBackChecking`；批准答 503 的文字；驳回之后页面是回滚一节（任务的整行）；退回之后回到升级中、接入一节与"提交审阅…"出现；没有报告时退回答 409：显示 `upgrade.sendBackChecking`、重读一次、20 秒之后再读一次（假的计时器），卸下之后不再读；有报告时的 409 显示通用的文字；`mode` 事件之后审阅一节随新的报告更新。修复一轮另加：退回被拒为"正在计算"之后报告到了、对话框仍写 `sendBackChecking`，挡住重读时从头到尾不写它，重读答 404 时不是"正在计算"，状态变了之后没有新的拒绝不再挂 20 秒（审查 P3S3A-I1 = C-M1、C-N1、A-N1 = C-N2）；批准答复时页面已离开不跳转（P3S3A-M1 = B-M2）；驳回、退回先盖住答复（挡住之后的那次 `GET upgrade`，P3S3A-N4 = B-M1）；批准之后按"后退"不先显示旧按钮（P3S3A-N2）；"approved"一格带事件流，批准之后发 `mode`、断开重连，请求记录仍只有批准之前的（P3S3B-I1）；驳回之后断言任务的整行（P3S3B-N2）；别的管理员退回之后检查一节清掉（决定 300）；退回被拒、计时器挂着时再退回而断网，不留旧的提交（ra05）。修复二轮另加：拒绝之后的重读被 `mode` 的读超过时只出现通用的文字（P3S3A-M3）；批准之后跳转之前升级页不闪成"加载中"（P3S3A-N5）；决定 300的"换过提交"：审阅中没有报告、经 `mode` 换了提交、再经 `mode` 回到 `upgrading`，检查一节清掉、不重读检查（P3S3A-N6）。
- `upgrade-banner.test.tsx`：`:49-51` 与 `:127-129` 两格改为：管理员在 `upgrading` 看到"Any admin of the notebook can abandon this upgrade."，在 `reviewing` 看到"Any admin of the notebook can reject this upgrade."、没有"abandon"那一句，`rolling_back` 两句都没有（决定 291）。
- 健康度页（决定 301，修复一轮）：树第一次读失败时条目不写"加载中"、节点一格空着（`health-page.test.tsx`，审查 C 的 REVIEWC-H 一格）；树再读失败时"更多"带来、树里没有的节点空着，不写"节点已不可用"（`health-page-lifecycle.test.tsx` 的"missing More nodes…"一格照改）。
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
- UP7（三种浏览器，`playwright.config.ts:3` 的 `threeBrowsers` 加一支 `upgrade\/up7-`；`threeBrowsers` 按文件，整个文件（API、MCP 与页面三个版本）都在三种浏览器里跑，同 7.5，审查 C 判实施者的第 8 条不是偏离）：dana 批准的那一页以 `holdStream`（`e2e/fixtures/events.ts:214`）打开，它的事件流在跳转到笔记本首页之后才放开：`mode` 可能早于批准的答复到达，那时升级页还装着，会重读升级、多出一条先后不定的 404；放开之后的重连只重读装着的键，升级页已卸下。放开之后等这次连接的最后一层（`GET …/assets`）答了，才断言它不读升级、才去下一页（`e2e/fixtures/upgrade-pages.ts` 的 `letInAtHome`：放开之前登记 `waitForRequest`、等它的 `response()`）：不等的话，下一次载入会截断还在飞的读（WebKit、Firefox 的 `Load request cancelled`），断言也可能早于重连的读（S3 建成时流挪到撤销那一段之后才放开，那时连接在变更集页的新文档里，SWR 的缓存每个文档一份，"没有读升级"无论怎样都成立，审查 P3S3B-I1；修复一轮改回首页放开、加上等，3.25）。断言跳转之后焦点在笔记本首页的标题、横幅消失、首页是 LLM Wiki 的样子。alice 的另一个标签页停在笔记本首页，先等它的事件流连上（照 UP1 的写法），批准之后不刷新就看到横幅消失。撤销 `index` 的那次删除在历史页上点"撤销"，答 409 `page.virtual_file`，以 `expectConsole({ errors: [failedToLoad(409)] })` 声明，对话框显示它的文字。
- UP8：发起、检查、提交、批准都在网页上；批准的那一页同 UP7 以 `holdStream` 打开；对话框与按钮按确切的名字找（05-lessons 第 6 节）。跳到首页之后放开流，同 UP7 以 `letInAtHome` 等这次连接的最后一层读答了，再断言放开之后的请求里没有 `GET …/upgrade`、`apiFailures` 为空：UP8 放开之后不再换页，"批准之后重连不读升级"在这里看得见（S3 建成时放开之后只做两次 API 调用就断言，不等连接，审查 P3S3B-I1，修复一轮）。

UP5 的页面版本随 S3 的第 1 个提交改两处断言（4.3 末）。LW11（`e2e/stories/wiki/lw11-playbook.spec.ts:27-81`）按契约文件比较 prompt 的列表，加了 `upgrade` 照样通过（普通笔记本一格只取 `lint`）；它的标题写着"three playbooks"，`:10` 的注释"takes the three prompts"、`:62` 的"A standard notebook lists the same three"也随 S2 不实：S2 的第 2 个提交改为"the LLM Wiki's playbooks""takes the LLM Wiki's three prompts""lists the same prompts"，不改断言（设计审查 P3C-N5）。

## 6. 实施步骤

每个提交都过 `make lint`、`make test-go-short` 与它碰到的测试；分支的 CI 在合并之前全绿。

### 6.1 S1（`m11-p3-s1`）

实施照这个次序，第 1–7 个是 `f86b7a6d`、`9432d60a`、`8abb2a1d`、`34d77c98`、`63cc0b84`、`bba17208`、`d7b85bae`（推送之前改写过一次 2–7 号，补变异第一轮的 o1、16a，10.1）；之后修复一轮 4 个（第 8–11 个，头 `a44602b0`，10.1），连实施的 7 个一共 11 个；第 12 个是本 Step 的文档提交，在它们之后（3.23）。

1. `build, ci: move four of the fourth shard's tests to the sixth, and the review's tests to the eighth (M11/P3/S1)`：`Makefile` 的 `BOOTSTRAP_SIXTH`（第 278 条）、`BOOTSTRAP_EIGHTH`（第 290 条）、两处的 `##` 与 `#` 说明、计数注释（`Makefile:265-266`）145/50/120/124/61/11/27/9 → 145/50/120/120/61/15/27/9（1 号提交时的数；头上 551 个、145/50/120/120/61/15/28/12，修复一轮照审查 P3S1C-N5 改注释，`a44602b0`）；`ci.yml` 的 bootstrap-4、bootstrap-6、bootstrap-8 一项前的说明。核对：`go test -race -list . ./internal/bootstrap` 的全部名字照 `make -n test-go` 展开的 `-run`/`-skip` 归一次，每个恰在一份（P2 的分份旁支的做法，`tools/shards8-p2.py`），只有这四个换了份；`make -n test-go` rc=0。
2. `server: the review's judgments, and the approval's write of the notebook (M11/P3/S1)`：3.2 的 `ReviewOf`、`JudgeRoots`、`ApprovalNotReady` 与测试；`JudgeUpgrade` 改用 `JudgeRoots`；提交的第一个事务改用 `ReviewOf`（3.9）；3.3 的 notebook 的 SQL、`UpgradeStates.Approve`、wiki 的端口与 `wikiUpgradeStates.Approve`、测试。
3. `server: rebuild an approved notebook's source hashes (M11/P3/S1)`：3.4 的 `RebuildSources`、`SourceRebuild`、`SourceRebuildHeads`（SQL、store、`gen/`，决定 292）与三组测试。
4. `server, api, web: approve, reject and send back an upgrade (M11/P3/S1)`：3.5 的 SQL 与 store；3.6–3.8 的用例与单元测试；3.11 的契约、处理函数、`make gen`（含 `schema.gen.ts`）与模块的码的测试；3.12 的接线；3.13 的矩阵的行；3.14 的日志（含决定 293 的 error 一行）；组合根的 `TestUpgradeApproval`、`TestUpgradeReview`、`TestUpgradeReviewInterleavings`、`TestUpgradeStatesReachTheStreams` 加的步、`TestUpgradeSubmitInterleavings` 第 7 条的退回与 `TestUpgradeCheck` 第 8 条的批准（7.1、7.2）。
5. `server, api, web: a revert does not bring a node back to a reserved root title (M11/P3/S1)`：3.10 的 `CheckContract`、两个操作的契约与 `make gen`（说明进 `schema.gen.ts` 的 JSDoc，所以带 `web`，设计审查 P3C-N2）、模块的码的测试、`TestUpgradeTrashReservedTitles`（含撤销改名的一格）。
6. `server, build, ci: the approval's cost guard (M11/P3/S1)`（改了 `Makefile`，范围带 `build`，S1 的实施）：`bootstrap/upgrade_approve_cost_test.go`（7.1）、`Makefile` 的 `test-go-costs-1` 加一行（带 `-v` 与显式的 `-timeout 15m`；说明照 `Makefile:433-436` 回滚先删那一行与 `:471-478` 的写法："本机约 4–5 分钟、7763 上约 7–9 分钟，过了 Go 默认的 10 分钟超时的一半，所以这一行 -timeout 15m：红在守卫自己的界上，不在 Go 默认的超时上"；设计审查 P3B-M7；S1 照实测改写，`Makefile:389-393`，守卫本机约 2 分钟，3.23）、`.github/workflows/ci.yml` 的 costs-1：`timeout` 从 35 分钟提到 45 分钟（第 294 条），那一行的注释写实测的三次（14:31、15:06、15:41）与估计（P3 之后约 23–26 分钟，照约两倍的规则取 45；开始约第 15–17 分钟，加 `-timeout 15m` 到第 30–32 分钟，Go 的超时先于作业时限到）。不拆行：只核对计划的种子与计时的在同一个测试里，一个二进制。本机两轮，界照实写进说明与 10.1。
7. `e2e: approve, reject and send back through the API and MCP (M11/P3/S1)`：UP6、UP7、UP8 的 API 版本与 UP6、UP7 的 MCP 版本、`fixtures/upgrades.ts` 的帮手（每个动作导出原样答复的 `post*` 与断言成功的一个，共六个，另导出 `postRevert`；`post*` 各配一处 409 的用法，S1 的实施，3.23）。
8. `server: the approval's guard judges each plan just before its statement runs, and each read by the key it needs (M11/P3/S1)`（修复一轮，`9c637540`）：`approveCostScans` 的 `nodes_pkey` 要条件里有 `(id = `、内侧逐行的 `nodes` 不许 `= ANY`；判计划挪到 `TraceQueryStart`，每条被判的语句第一次真跑之前、同一连接上判（第 296 条）；`EXPLAIN` 出错时先报计划的错误；界的注释写第 298 条的条件；短测试 9 → 19 格（审查 P3S1B-M1、M2、N1、Q1）。
9. `server: the review's tests see the approval's source root, the send-back's row, the rollback's page and the wired deadline (M11/P3/S1)`（`cc751dca`）：只改测试，审查 P3S1A-N1–N6（驳回一格先退回、再改、再交，3.23）。
10. `e2e: UP7 checks the sources' hashes against the SHA-256 of the bytes written (M11/P3/S1)`（`9dac9244`）：审查 P3S1C-N4。
11. `build: the shards' count comments as of S1's end (M11/P3/S1)`（`a44602b0`）：`Makefile` 的两处计数注释，审查 P3S1C-N5。
12. Step 的文档提交（第 8 节第 4、5 条）。

### 6.2 S2（`m11-p3-s2`）

实施照这个次序，第 1–3 个是 `567f1201`、`9d7e9004`、`d2512eeb`（1、2 号的标题照实施说明写行为的改变，下面照实写）；之后修复一轮 3 个（第 4–6 个，头 `7daa3452`，10.2），连实施的 3 个一共 6 个；第 7 个是本 Step 的文档提交，在它们之后（3.24）。

1. `server, api, build: the notebook entry serves the upgrade prompt, its playbook only while the notebook upgrades (M11/P3/S2)`（初稿的标题是名词短语；改了 `Makefile` 的计数注释，范围带 `build`）：3.16 的用例、`playbook-upgrade.md`、`UpgradePlaybookText`、`prompts.go`、`prompts.json` 与单元测试；3.18 的四处组合根测试（第三份的 `TestThePromptReadsLLMWikiMatrix`、`TestThePromptListsAreTheContract`，第七份的 `TestUpgradePromptMatrix`、`TestUpgradePrompt`），计数注释 +2；`upgrade_submit` 的工具说明加"or sends it back"（`api/mcp/tools.json:589` 与账户入口的同一项 `:1298`，设计审查 P3C-N4），连同 `wiki/adapter/mcp/tools.go` 的常量与 `testdata/tools.json` 的两个入口；模块根导出 playbook 的文字给组合根的测试（3.24）。
2. `e2e: UP9, the agent of an upgrading notebook takes the upgrade playbook, a notebook not upgrading refuses it (M11/P3/S2)`：UP9；LW11 的 `textOf` 挪进 `fixtures/mcp.ts`（`promptTextOf`），两个故事共用；LW11 的标题与 `:10`、`:62` 的注释（第 5 节末；设计审查 P3C-N25）。
3. `docs: README's upgrade section and the MCP checklist's K (M11/P3/S2)`：3.19（新的一节，连同 `README.md:214`、`:234`、`:236`、`:246` 与 MCP 一节的两个工具）、3.20（K1–K5）。
4. `server: the upgrade playbook's test pins each sentence's advice, once; the prompt's unit sees a reported review (M11/P3/S2)`（修复一轮，`94db07e4`）：只改测试与注释：`TestUpgradePlaybookText` 每句或分句一格、各恰好出现一次（46 格）；单元表加"审阅中、有报告"一格；两处注释（审查 P3S2B-M1 = C-M1、B-N1，A-N1、N2）。
5. `server: the upgrade playbook's gate 4 leaves AGENTS and the source root without type and summary; server_busy and forbidden from either tool (M11/P3/S2)`（`8321734b`）：playbook 的三处措辞与测试的三格，47 格，4,979 字节（审查 P3S2C-N3、N4；3.17）。
6. `docs: README's upgrade lines and the MCP checklist's K for two clients (M11/P3/S2)`（`7daa3452`）：README 的三处与 K（审查 P3S2C-M2、N1、N2、N5、N6；3.19、3.20）。
7. Step 的文档提交（第 8 节第 4、5 条）。

### 6.3 S3（`m11-p3-s3`）

实施照这个次序，第 1–3 个是 `4484ca7c`、`ee298a0a`、`05423d08`（标题照实施说明写行为的改变，下面照实写；推送之前改写过两次，补变异抓不到的测试，并进 1、2 号，10.3）；之后修复一轮 2 个（第 4、5 个）、修复二轮 1 个（第 6 个，头 `a25fef6d`），连实施的 3 个一共 6 个（10.3）；第 7 个是本 Step 的文档提交，在它们之后（3.25）。

1. `web, e2e: the report in review shows its rules as cards whose More reads from the first page, in this entry's cache; a tree that failed to read says so (M11/P3/S3)`（初稿的标题是名词短语）：`HealthRule` 的类型与游标、健康度页随之改的两处（`paths` 的参数、`read` 在没有游标时抛缺陷）、`checkItems` 的可缺的游标、`itemsReader`、审阅一节挪进这一次进入的 `SWRConfig`（它的 `HealthScope`）、树读失败；对应的 vitest；UP5 的页面版本的两处断言。
2. `web: an admin approves, sends back or rejects the upgrade in review on its page, Reject standing for Abandon; approved, the page goes to the notebook's home (M11/P3/S3)`：service、store、按钮与对话框（批准自己的函数）、审阅中收起"放弃"与横幅的"驳回"一句（决定 291）、两种语言的文字、退回的说明与 20 秒的再读、批准之后的跳转、假服务端的三条路由；vitest。
3. `e2e: UP6-UP8 on the upgrade's page, an admin sending back, rejecting and approving it; UP7's in the three browsers (M11/P3/S3)`：UP6–UP8 的页面版本、`threeBrowsers` 加 `upgrade\/up7-`、`fixtures/upgrade-pages.ts` 的 `changeButtons`、`reviewDialog`。
4. `web: a send-back refused while checked says so as the upgrade read again after it says, until closed; another admin's send-back clears the check too; approved, the upgrade leaves the cache; the health page leaves a failed tree's paths empty (M11/P3/S3)`（修复一轮，`a040dcc1`）：退回的说明在拒绝之后那一次重读回来时定一次（所有经 `change` 的对话框等重读），20 秒只为被拒的那一次提交；决定 300、301；批准之后清缓存；假服务端每次提交给新的 `submitted_at`；vitest 加的各格（审查 P3S3A-I1 = C-M1、C-N1、A-N1 = C-N2、A-M1 = B-M2、A-M2 = B-M3、A-N4 = B-M1、A-N2、A-N3、B-N2，B-I1 的 vitest 一半）。
5. `e2e: UP7 and UP8 let the approving page's stream in at the notebook's home and see its connection read no upgrade (M11/P3/S3)`（`bc1a24e5`）：`fixtures/upgrade-pages.ts` 的 `letInAtHome`，UP7、UP8 照它放开与断言，UP6 末段的注释（审查 P3S3B-I1、N1）。
6. `web: a send-back refused as its read again is overtaken by a mode event's reads the upgrade once more before it says why; approved, the upgrade leaves the cache once the page is gone (M11/P3/S3)`（修复二轮，`a25fef6d`）：拒绝之后的重读被超过时再读一次再定说明；批准之后卸下时才清缓存；决定 300 的"换过提交"一格（审查 P3S3A-M3、N5、N6）。
7. Step 的文档提交（第 8 节第 4、5 条）。

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
| 第 181 条的判定 | `contract_test.go` 的六格（S1 建成：新加四格、原来的一格改答 `page.virtual_file`，另两种放过由已有的格盖住，3.23） | `page/domain` |
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
2. **批准**：dana（第二位管理员）批准，答 200，答复整值断言（`state: approved`、`submitted_by` 是 erin、`report` 原样、`ending`、`rollback_*` 为空；`started_by`、`submitted_by` 与批准人是三个不同的人）；笔记本 `mode = llm_wiki`、`source_root_id` 是 `raw`、升级的四列为空；`upgrade_nodes` 没有这次升级的行；`notebook_upgrades` 的行 `approved`、审计列是 dana 与时刻；`assertSourceHashesMatchRebuild`、`checkPages`（含 `sourceHashGap` 与升级规则）、`checkLinks`、`checkSearch`（后两个 S1 建成时漏了，修复一轮补上，审查 P3S1A-N3 = C-N3）；`getUpgrade` 答 404 `wiki.upgrade_not_found`；笔记本的读答 `mode: llm_wiki`、`upgrade: null`；之后 LLM Wiki 的契约生效：删除根下的 `AGENTS` 答 `page.protected`、编辑一页来源答 `page.source_immutable`；info 一行（3.14），`sources_hashed` 等于期望的数（五：两页、一页带正文的文件夹页、两个附件）。
3. **另一个标签页**：alice 的流在批准之前打开，批准之后收到一帧 `mode`（`tm.modesOf` 开的是 alice 的流，`upgrade_submit_test.go:98-106`；设计审查 P3A-N9）。与第 2 格是同一次批准，S1 合成一格"approved, another tab told"（3.23）。
4. **闸门在锁下再核**：另一本 `tm.reviewed`，以 SQL 把根下 `AGENTS` 的正文清空（绕过冻结，模拟缺陷；同时改 `byte_size` 与 `content_hash`，`00013_page_page_contents.sql:12-15` 的检查），批准答 409 `wiki.upgrade_not_ready`，`detail` 写着 `agents` 与"still in review"；行与模式不变、没有 info；error 一行整行断言（`notebook_id`、`upgrade_id`、`failed_gates` 是 `["agents"]`，决定 293）。
5. **空的重建**（00 号文档第 9 节的反向对照）：另配一个应用，`wiring.wiki` 把 `SourceHashes` 换成什么也不做、交回 0 的替身；批准答 200，而 `sourceHashRebuildMismatches`（`source_hash_invariant_test.go:43`）答出的节点的集合恰是这本笔记本的来源（`checkPages` 只给每条规则的计数句子，读不出集合；设计审查 P3B-N5），不是调 `checkPages` 让测试失败。
6. **自己的期限**（照发起的"its own deadline"，`upgrade_start_test.go:216-280`）：另配一个应用，三格的 `ApproveTimeout` 都取 2 s（初稿写第一、三格 300 ms、第二格 2 s：第二格必须 2 s，另起一个 300 ms 的应用只为两格各快约 1.7 秒，不值，2 s 下第一、三格一样确定；S1 的实施，3.23）；配应用的钩子先断言组合根交的 `ApproveTimeout` 是测试应用的请求 4 s 的四分之三，再换成 2 s，竞态的八份里也看得见 `* 3 / 4`（审查 P3S1A-N6，修复一轮）；三格各答 503、`Retry-After: 1`、warn 一行（`timeout_ms` 是这一格的期限）、没有 error 行、什么也没写（模式、四列、哈希、基线行、wiki 的行）、`checkPages`：
   - 语句之中、判定之前：测试持着笔记本行（`tm.holdNotebook`），批准等在锁上到期；
   - 语句之中、写了之后：测试持着 `raw` 下一页来源的 `nodes` 行（`SELECT … FOR UPDATE`），批准写了模式、清了哈希之后，`SetSourceHashes` 等在那一行上到期（`ForgetSources` 不碰它：普通笔记本的哈希都是空，`WHERE source_hash IS NOT NULL` 不匹配）。钉住"写了之后到期"，不只看 503 与"什么也没写"（期限在更早的一步到期，例如慢的 runner 上 300 ms 在 `SourceNodes` 就用完，也一样绿）：包住 `SourceHashes` 的一层在进 `Rebuild` 时记下（不拦，照 `slowCapture.waited`），并以 `pgtest.WaitForLockWaitsOn(nodes, 1)` 看到批准等在那一行上，两样都断言（设计审查 P3B-N6）。这一格的 `ApproveTimeout` 取 2 s：这个等待只在"走到 `SetSourceHashes`"与期限到期之间，`WaitForLockWaitsOn` 每 10 ms 轮询、到它自己的时限就 `t.Fatal`（`pgtest/lockwait.go:45`、`:136-155`），300 ms 在慢的 runner 上可能还没走到这一步就到期，会偶发地红；2 s 下等待者在锁上一直等到期限，这一格只多约 2 秒（设计审查 P3B-N9）；
   - 两条语句之间：包住 `SourceHashes` 的替身在 `Rebuild` 里等 ctx 到期（照 `slowCapture`，`:286-298`）。
   - 另一格：请求的 ctx 先到期（客户端断开）时不记 warn、不答 503（单元测试已有，这里不重复）。

**`TestUpgradeReview`**（`bootstrap/upgrade_review_test.go`，新）：

1. **退回的码**：404、404（carol）、403（bob）、没有升级 409、`upgrading` 409、`rolling_back` 409；审阅中没有报告、刚提交：409、`detail` 含"still being checked"；以 SQL 拨回一小时：200（接管）。
2. **退回**：有报告的升级，dana 退回，答复整值断言（`upgrading`，提交与报告为空）；行是 `backToUpgrading`（`upgrade_submit_test.go:88-91`）；一帧 `mode`；之后 alice 的 agent 经 MCP 写一页原有页的 frontmatter 被放行（`tm.agent`），再交通过、存下新的报告（`submission_id` 不同）；info 一行（`reported: true`）。
3. **驳回的码**：404、404、403、没有升级 409、`upgrading` 409（什么也没写、没有任务）。
4. **驳回**：有报告的升级，dana 驳回：答 `rolling_back`、`ending: rejected`、带任务；info 一行带 `job_id`；回滚做完（`tm.endedBackground`）之后数 `mode`，共 2 帧（驳回一帧、做完一帧：回滚不停住时小笔记本几十毫秒就做完，在做完之前数会读到 1 或 2 帧，所以等做完再数；设计审查 P3A-M4，取"在 `endedBackground` 之后数"的那一种）；行是 `rolled_back`、`rejected`、`rollback_complete = true`，提交与报告留着（`notebook_upgrades_rejected_check`），基线复原（驳回之前先由 dana 退回，alice 的 agent 经 MCP 改 `Note` 的 frontmatter（仍干净），erin 再交，再驳回；之后正文回到 `cleanNote`、发起写的 `AGENTS` 不在。审阅中冻结 agent 的写，所以要先退回（审查 P3S1C-N7）。S1 建成时只断言后者与正文同发起时，不靠回滚也成立，审查 P3S1A-N4，修复一轮 `cc751dca`），`checkPages`。审阅中没有报告的也可以驳回：答 200、之后同样做完。
5. **日志**：三种 info 的字段整行断言（3.14）。

**`TestUpgradeTrashReservedTitles`**（`bootstrap/upgrade_trash_test.go`，新，第七份）：一本普通笔记本，根下建 `index`、`log` 两页并各自删掉（两个变更集），再建一页笔记；发起、交、批准（同 `tm.reviewed` 的做法）。之后：

- `revertChangeset` 删掉 `index` 的那个变更集：409 `page.virtual_file`，什么也没写（回收站里仍有它，根下没有）；
- `startRevert` 同一个变更集：409 `page.virtual_file`，没有任务；
- `restoreNode` 回收站里的 `index`：200，名为 `index 2`，在根下；
- `startTrashRestore` 回收站里的 `log`：任务做完，名为 `log 2`；
- 撤销改名（设计审查 P3A-M5）：同一本笔记本根下另有一页 `index.md`，发起之前就在、不删；升级中 alice 的 agent 经 MCP 把它改名为 `Index notes`（闸门 3 才过），再交、批准。之后 `revertChangeset` 那次改名：409 `page.virtual_file`，名字仍是 `Index notes`。同步撤销一次就够，后台撤销由上面恢复的那一格盖住；
- `checkPages`（根下没有保留的键，00 号文档的 LLM Wiki 规则 `:150-152`）。

**`TestUpgradeStatesReachTheStreams`**（`upgrade_start_test.go:388-480`）在现有的一串之后加：发起、交（存下报告，2 帧）、退回（1）、交（2）、驳回（1）、回滚做完（1）、发起、交（2）、批准（1；帧里没有前后模式，`Publish` 的 `From standard`、`To llm_wiki` 由单元 `TestApproveUpgrade` 整值断言，初稿写在这里，审查 P3S1A-N7 = C-N2）。驳回的一步照现有的放弃（`:425-450`）：`stops.arm(nb, 1, 0, false)`（四个参数，S1 的实施）之后驳回，任务停在第一个计划，数 1 帧；`stops.goOn()` 之后回滚做完，再数 1 帧（设计审查 P3A-M4）。三个流（alice 管理员、bob 读者、carol 看不到）照旧 `[n, n, 0]`。反向对照：组合根交给 `notebook.NewUpgradeStates` 空的订阅者时这些步收不到帧（同现有的步，P1 设计 3.15）。

**成本守卫 `TestUpgradeApprovalTakesItsTimeAtAnyStatistics`**（`bootstrap/upgrade_approve_cost_test.go`，`//go:build !race`，costs-1，第 280 条）：

- **种子**（SQL，05-lessons 第 1 节；照设计时的探针，7.6）：一本普通笔记本，根下 `AGENTS`（正文非空）、`raw` 下 500 个文件夹页（带正文）、每个下面 90 页带正文、5 页空页、4 个附件（`asset_blobs` 的行）：约 4.75 万个要写的哈希、约 6 万个节点；根下另一个文件夹里 1 万页带干净 frontmatter 的笔记。库里另有九本各 5 万页、来源带哈希的 LLM Wiki 与它们各自一次升级的基线行（约 51 万行 `upgrade_nodes`；第 295 条：每本根下 `raw` 收 45,000 页与 2,000 个附件、带哈希，另 4,999 页在根下，基线行用一个没有行的升级 id），各另有 2,000 个附件（附件节点与 `asset_blobs` 的行），让 `AttachmentInfos` 丢了键时读到别本的行（决定 240；设计审查 P3B-I1：库里只有这一本的附件时，`asset_blobs` 的顺序扫描本来就最便宜，丢了键看不出来）。之后经 REST 发起与提交（真的路径，报告由提交存下），再以这个库作模板。模板里照发起的守卫关掉这几张表的 autovacuum（表的存储参数随副本复制）。
- **配置**：生产的请求 15 秒（`ApproveTimeout` 11.25 秒），同 `bootstrap/wiki_lint_cost_test.go:27-34` 的写法另配；`wiring.wiki` 里断言 `d.ApproveTimeout == 15 s × 3/4`，守住组合根的 `* 3 / 4`（同发起的守卫断言 `StartTimeout`，`upgrade_start_cost_test.go:114-117`）。
- **统计状态 × 计划**（设计审查 P3B-M2）：三种状态——"从未分析"（在副本上清 `nodes`、`page_contents`、`asset_blobs`、`upgrade_nodes`、`notebook_upgrades` 的统计）；"发起留下的统计"（种子之后 `ANALYZE`，发起自己分析 `upgrade_nodes`；模板原样）；"这本笔记本的页在统计之后才写入"（另一个模板：另九本种好就 `ANALYZE`，再种这一本、经 REST 发起与提交，统计以为它很小：批准真会遇到的过时，一本刚导入的大笔记本）——× `plan_cache_mode` 自定义与通用（05-lessons 第 4 节）。初稿的"只在别的升级的统计下（发起之前分析）"经 REST 发起做不出来（发起在提交之后 `ANALYZE upgrade_nodes`，`start_upgrade.go:178`），删掉。每一格三次批准，每次在模板的一份新副本上（`pgtest.NewDatabaseFrom`，第 280 条），新起一个应用。每份副本上 `ALTER DATABASE <副本> SET plan_cache_mode`：副本不继承模板的库级设置（`CREATE DATABASE … TEMPLATE` 不复制它），新应用的连接以 `SHOW plan_cache_mode` 断言是这一格的方式。新应用先 `awaitJob(jobs.PurgeKind)`（`assets_jobs_test.go:122`），再等 `river_job` 里到期的作业都不在 `available`、`running`、`retryable`（启动时的周期作业做完；S1 的实施），再批准、计时。
- **计划为主**（决定 236、240、244；设计审查 P3B-I1）：经 `wiring.queryTracer`（同 `startCostTrace`，`upgrade_start_cost_test.go:311`）记下批准的事务里的语句与它们的参数；每条被判的语句在它第一次真跑之前，在 tracer 的 `TraceQueryStart` 里、同一连接上 `EXPLAIN`（自定义带它的参数，通用计划经 simple protocol 用 `EXPLAIN (GENERIC_PLAN)`，同 `startCostPlans`，`:441-496`），用时从 `elapsed_ms` 里减；`SHOW plan_cache_mode` 在包住 `DeleteBaseline` 的一层、每次批准断言（第 296 条：S1 建成时在删基线之后判，`SetSourceHashes` 已写完，判的不是生产那一刻的状态，审查 P3S1B-M2，修复一轮改）。`EXPLAIN` 本身出错时先报计划的错误（审查 P3S1B-N1）。规则另由不要数据库的短测试 `TestTheApprovalGuardJudgesItsReads`（short 一行）以手写的计划逐条判，每个子条件一格反例（S1 的实施者加的，修复一轮补全，审查 P3S1B-M1）。照 `startCostPlanned()` 的写法列出被判的语句，不写"每条语句"：
  - **判**：`ForgetSources`、`LintNodeMetadata`、`SourceRebuildHeads`（决定 292）、`AttachmentInfos`、`SetSourceHashes`、`DeleteUpgradeNodes`、`RootNodesByKey`、`FindNodeIn`、`PageContent`。`nodes` 只经首列是 `notebook_id` 的索引（条件里有 `notebook_id = `）或条件里有 `(id = ` 的 `nodes_pkey`（没有条件的 `nodes_pkey` 按键序读整张表；S1 建成时不看条件，审查 P3S1B-M1，修复一轮），`page_contents` 只经 `page_contents_pkey`，`asset_blobs` 只经 `asset_blobs_node_id_key`（`AttachmentInfos` 按 `node_id = ANY(…)` 读，走这个唯一约束，不是主键 `id`，`00027_asset_asset_blobs.sql:10-11`），`upgrade_nodes` 只经 `upgrade_nodes_pkey` 的前缀；都不顺序扫描；嵌套循环内侧逐行重复的扫描以外行的键为条件（不逐行重读整个集合；内侧逐行的 `nodes` 认条件里的 `(id = `，经 `nodes_notebook_id_id_key` 也算，第 297 条；条件含 `= ANY` 的拒绝，每行重读一组，修复一轮加的）；`SourceRebuildHeads` 另更严：每个 `nodes`、`page_contents` 的读都按行的键，这本笔记本的 `nodes` 一次也不读（"读头只经键"，S1 的变异 16a 补的，3.23）；`SetSourceHashes` 每块的计划至多读这本笔记本一遍（哈希建在哪一边都可以），或逐行经键（3.4 的"界与分块"：自定义计划把哈希建在这本笔记本上，通用计划逐行经键或把哈希建在这一块上；设计审查 P3B-M1）。
  - **不判**：`notebooks`、`notebook_upgrades`、`workspaces`、`users` 与成员表的点读（`LockNotebook`、`CurrentUpgrade`、`ShareByID`、`named`、判定）：守卫的库里它们只有几行，顺序扫描是对的计划（审查 B 实测分析过的两种状态下 `LockNotebook`、`CurrentUpgrade` 两种计划都是 `Seq Scan`），判它们要另种上千本笔记本，不值。
  - **分块的语句判第一块**（满的一块，一万个）：`SourceRebuildHeads`、`SetSourceHashes` 每次批准跑五、六块，`startCostTrace` 那样按名字存只留最后一块（7,000 或附件那一块），块的大小不同自定义计划可以不同；这个守卫的 tracer 按名字留第一次的参数。
  - 列出的语句在记下的语句里没有出现就失败（照 `startCostPlans`，`upgrade_start_cost_test.go:447-450` 的"the start ran no %s"；设计审查 P3B-N11）。计划有问题就停在计时之前。自定义计划由设计时的探针、通用计划由审查 B 的探针核对过（`asset_blobs` 与小表除外，见上），`SourceRebuildHeads` 的两种由修订时的探针核对过（3.4）。
- **只核对计划的种子**（决定 292）：同上的形状，另几本只有四本（这一本约占 `nodes` 的 22%，审查 B 的变体：`SourceHeads` 在这里换成每块顺序读整张 `page_contents`），三种统计状态 × 两种计划，每格一次批准（在副本上），只判重建的几条语句（`ForgetSources`、`LintNodeMetadata`、`SourceRebuildHeads`、`AttachmentInfos`、`SetSourceHashes`）的计划，同上的断言，不计时（同决定 244 的"原来的形状只核对计划"）。`SourceRebuildHeads` 两种计划都只经键，`page_contents` 不顺序扫描。不判 `DeleteUpgradeNodes`、`RootNodesByKey`、`FindNodeIn`、`PageContent`：这一本占比大时，删基线读走整张 `upgrade_nodes` 是对的计划（设计者的日志里 22% 时分析过的自定义计划与"统计之后才写入"的通用计划都是如此），照上面的清单全判会在对的计划上失败（设计审查 P3B-N8）。三种状态照上面的做法得到，所以这一种子也要两个模板：一个是另四本与这一本都种好再 `ANALYZE`、经 REST 发起与提交（"发起留下的统计"，副本上清统计就是"从未分析"）；另一个是另四本种好就 `ANALYZE`，再种这一本、经 REST 发起与提交（"这本笔记本在统计之后才写入"；设计审查 P3B-N12）。
- **时长为辅**：每格三次的服务端 `elapsed_ms`（`upgrade approved` 一行）的中位数在界之内，每次都答 200；界取本机两轮较大的中位数 × 3（S1 建成时 1.285 s × 3，向上取整到 100 ms，3.9 s；合并之后两次 `main` 的 costs-1 任一次在 EPYC 7763 或更慢的 runner 上任一格的中位数过 3.25 s，就调到 4.5 s，第 298 条；设计时探针里 5 万个来源的重建与删基线本机 0.99–1.19 秒，整个批准估 1.2–1.4 秒，界约 4 秒，远在 11.25 秒之下）；带 `-v` 打出每格的中位数与各段。
- 本机估计约 4–5 分钟（S1 实测 111–121 秒，`Makefile` 的说明照实测写，10.1；四个模板：计时的两个、只核对计划的两个，种子各约 10–20 秒，发起与提交各约 3 秒，另一个模板约 20–30 秒，审查 B 估；18 次计时的批准与 6 次只核对计划的，各一份副本，副本约 0.5 秒一份，审查 B 实测）。`Makefile` 的这一行显式带 `-timeout 15m`（下面与 7.5；设计审查 P3B-M7）。

### 7.2 交错

05-lessons 第 3 节：持住双方都要的行（`pgtest.WaitForLockWaits`），再放开，先后在共用的日志里断言；`checkPages`、`checkLinks`、`checkSearch` 在全部交错之后对整个库查一次（`TestUpgradeReviewInterleavings` 的末尾；库里有每个交错的笔记本，与每个交错结束时各查一次效果相同；初稿写"每个交错结束时"，S1 的实施，审查 P3S1C-N3，3.23）。编号照 00 号文档 4.16。

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

照 `tools/mut.py`，每条在 `server/` 或 `web/` 下改一处、跑抓它的测试、期望失败。S1 的实施与审查之后，"读头"一行拆成 16a、16b，三处单元测试的名字照实；S1 的各行之后接实施者自己的与审查者留下、修复一轮杀的变异，照它们在 [S1 的审查记录](reviews/P3-S1-review-actions-review.md) 里的名字记（3.23）。S2 的各行之后同样接实施者自己的与审查者留下、修复一轮杀的变异，照它们在 [S2 的审查记录](reviews/P3-S2-playbook-docs-review.md) 里的名字记（3.24）。S3 的各行之后同样接实施者自己的与审查者留下、修复一轮杀的变异，照它们在 [S3 的审查记录](reviews/P3-S3-web-review.md) 里的名字记（3.25）。

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
| `SourceRebuildHeads` 名字不变、SQL 改成"`unnest` 连 `nodes`、左连 `page_contents`"的普通连接（16a；决定 292） | 守卫的两个种子的每格（这本笔记本的 `nodes` 按笔记本的索引读、不按键）。"读头只经键"一条之前抓不到：通用规则放行"按笔记本读一次 `nodes`"，`page_contents` 逐行经主键、不顺序扫描；S1 的变异第一轮发现，补了这一条与短测试一格（3.23） |
| `SourceRebuildHeads` 换成 `SourceHeads` 的形状（16b；初稿写的"改回连接的形状"，决定 292） | 同上的每格；只核对计划的种子（这一本约占 22%）"发起留下的统计"的两格另顺序读 `page_contents`（自定义计划）。初稿写"在一成的种子上抓不到"（审查 B 的探针：三种状态都逐个经 `page_contents_pkey`），是"读头只经键"一条之前的说法 |
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
| 驳回的 info 在提交之前记 | `TestRejectUpgrade` 的提交失败一格（`review_upgrade_test.go`，3.23） |
| 退回的 info 在提交之前记 | `TestSendBackUpgrade` 的提交失败一格（`review_upgrade_test.go`，3.23） |
| 驳回接受 `upgrading` | `TestUpgradeReview` 驳回的码的 `upgrading` 一格（`notebook_upgrades_rejected_check` 会让它 500，而测试要 409） |
| 驳回记 `abandoned` | `TestUpgradeReview` 第 4 格的 `ending`；单元 |
| 退回不判 `ReviewChecking` | `TestUpgradeReview` 退回的码"刚提交"一格；交错 7 之退回（一） |
| 退回的 SQL 不清报告 | `TestTheReviewsRows`（`notebook_upgrades_report_check` 会让写失败，测试要整行） |
| 退回的 `taken_over` 用 `at` 不减 `SubmitTimeout` | `TestSendBackUpgrade` 的整值一格（`TestTheReviewsRows` 以显式的 `taken_over` 测 SQL，看不见用例传了什么；`TestUpgradeReview` 里 `ReviewOf` 先答 409，到不了 SQL；设计审查 P3B-N2） |
| 退回不 `SetState`（笔记本留在审阅中） | `TestUpgradeReview` 第 2 格：agent 的写仍被冻结 |
| 第 181 条不限 `ChangesetRevert` | `TestUpgradeTrashReservedTitles` 的 `restoreNode` 与 `startTrashRestore` 两格（答 `page.virtual_file` 而不是 200）；`contract_test.go` |
| 去掉第 181 条的改动 | 同一个测试的两个撤销格 |
| `UpgradeStates.Approve` 不判事务（o1，实施者加的） | `TestAnUpgradeIsApproved` 的"事务外"一格（放在升级进了审阅之后；第一轮放在 `upgrading` 上，事务外的写本来就改不到行，没抓到，推送之前改） |
| `SourceRebuild.Rebuild` 不判事务（o2） | `TestTheSourceRebuildIsTheInvariant` |
| `SourceRebuildHeads` 不判笔记本、不判删除（o3、o4，生成的常量） | `TestSourceRebuildHeadsReadTheirKeys` |
| 退回的 info 的 `reported` 恒为假（o5） | `TestUpgradeReview` 的退回一格 |
| 组合根的 `ApproveTimeout` 取整个请求（o6） | 守卫（`approveCostApp` 断言 11.25 s）；修复一轮之后另由 `TestUpgradeApproval` 换期限的钩子（第八份，审查 P3S1A-N6） |
| 驳回的处理函数调退回的用例（o7） | `TestTheUpgradeIsAnswered` |
| 守卫去掉"读头只经键"一条（o8，对照） | `TestTheApprovalGuardJudgesItsReads` 的"读头连上读一次的这本笔记本"一格 |
| 判计划的规则的子条件各自去掉（审查 B 的 r1–r6：`nodes_notebook_id_*` 不要求 `notebook_id = `、`page_contents_pkey` 不要求 `node_id = `、`asset_blobs_node_id_key` 不要求 `node_id = `、`upgrade_nodes` 怎么读都放行、`upgrade_nodes` 内侧不要求 `node_id = `、去掉通用的"内侧逐行按外行的键"）；`nodes_pkey` 不看条件 | `TestTheApprovalGuardJudgesItsReads` 的反例各一格（S1 建成时都照过，审查 P3S1B-M1；修复一轮 `9c637540` 补全之后都死） |
| 去掉"内侧逐行的 `nodes` 不许 `= ANY`"（修复一轮加的一条，r8） | `TestTheApprovalGuardJudgesItsReads` 的"a set of nodes by their key again for each id"一格 |
| "内侧逐行不许 `= ANY`"连只读一次的也拒（审查 B 在修复核对时加的 r9） | 短测试的正例"a set of nodes by their key once"（修复一轮） |
| 批准交给 `JudgeRoots` 的来源根换成零值（审查 A 的 a1） | `TestApproveUpgrade` 的"来源根是根下的 `AGENTS`"一格（修复一轮，审查 P3S1A-N1） |
| 退回的"行不是这次升级的"判断只剩 `!ok`（a2） | `TestSendBackUpgrade` 的"别的升级的行"一格（修复一轮，审查 P3S1A-N2） |
| 驳回的回滚不写回正文（审查 P3S1A-N4；n4：`RollbackState` 不出正文的版本） | `TestUpgradeReview` 的驳回一格（驳回之前先退回、改过 `Note` 的 frontmatter、再交，修复一轮 `cc751dca`；n4 改前活、改后死：任务以 `internal` 停、行留在 `rolling_back`、正文仍是改过的） |
| `SourceRebuildHeads` 的 `page_contents` 改为普通的 `LEFT JOIN`、保留 `nodes` 的 `LATERAL … LIMIT 1`（审查 B 的 b-16c） | 没有（等价：两个种子上 24 个计划与原样相同，只少 `Memoize` 下的一个 `Limit`） |
| `UpgradePlaybook` 在 `reviewing` 也答文字 | `TestUpgradePromptMatrix` 的 `reviewing` 列（种的审阅中没有报告）；单元的 `reviewing` 一格；`TestUpgradePrompt` 的审阅中（真的提交，有报告） |
| `UpgradePlaybook` 不判 `lint.read` | `upgrade_playbook_test.go` 的"看不到答 `notebook.not_found`"一格与调用的次序（替身的判定）。整个程序里看不到的、删掉的笔记本在 HTTP 的边上由 `Gate`（`node.list`，`page/mcp_entry.go:29-31`、`mcpserver/edge.go:232-249`）就答 404，到不了用例（设计审查 P3B-M3 第 2 条 = P3C-N6） |
| `upgrade` 的 prompt 经 `Playbooks.Execute`（`PlaybookText` 收进它） | `TestUpgradePromptMatrix` 的 `llm_wiki` 列（答文字而期望 `wiki.upgrade_state`）与 `upgrading` 列（答 `notebook.not_llm_wiki` 而期望文字）。只给 `PlaybookText` 的 `switch` 加一支而没有调用者是等价变异（设计审查 P3B-N3） |
| 只给 `PlaybookText` 的 `switch` 加 `upgrade` 一支、没有调用者（e1） | 没有（等价：适配器只把三个固定的 `Playbook` 交给 `Playbooks.Execute`；对照是上一行，同一处加上调用者就抓到） |
| 没有升级、回滚中也答文字；读升级出错当成状态；先读后判；答 `wiki.upgrade_not_found`；拒绝用"still being checked"的 `detail`（实施者的 o1–o6） | 单元 `TestUpgradePlaybookAuthorizesAndRequiresUpgrading`；o1、o2、o5、o6 另由矩阵或 `TestUpgradePrompt` |
| `Get` 交零值的笔记本、不交 `description`；`Prompts()` 漏了 `upgrade`、登记在两个入口（o7–o10） | 适配器的 `TestTheUpgradePromptPassesItsNotebook`、`TestThePromptsAreTheFour`；矩阵、`TestUpgradePrompt`、`TestThePromptListsAreTheContract` |
| `Playbooks.Execute` 放过普通笔记本（o11，另一处复写的判断） | `TestPlaybooksAuthorizeAndRequireMode`；矩阵的 `standard` 列 |
| `UpgradePlaybookText` 不去首尾空白；`MaxPlaybookBytes` 缩到 4 KiB（对照）；用例答 lint 的文字；第一行改掉（o12、o13、x1、x2） | `TestUpgradePlaybookText`；x1 另由单元、矩阵与 `TestUpgradePrompt` |
| `upgrade_submit` 的说明只改了一处（o14；审查 A 的 a4、a5 与 B 的 c1、c2：契约或 testdata 只改一个入口） | `TestTheToolListsAreTheContract`、`TestLintToolObjects` |
| `prompts.json` 多一个没有期望的 prompt（o15） | `TestThePromptReadsLLMWikiMatrix`、`TestUpgradePromptMatrix` |
| 审阅中而有报告也答文字（审查 A 的 a1） | `TestUpgradePrompt` 的审阅中（真的提交）；修复一轮之后另由单元的"审阅中、有报告"一格（审查 P3S2A-N1） |
| playbook 的一句删掉，或保留条件、把做法换成错的（实施者的 s01–s36 每一处都换；审查 B 的逐句删与 pb01–pb20，C 的 c1–c7） | `TestUpgradePlaybookText` 的"必有的句子"，每句一格。S2 建成时 s01–s36 都抓到，而 B 单删的 9 句、pb01–pb20 与 c1–c7 照过（片段止于条件与码，`page.upgrade_immutable` 被另一句顶替；审查 P3S2B-M1 = C-M1）；修复一轮之后每句或分句一格、各恰好一次（47 格），c1–c7、pb01–pb20、B 单删的那几句与 a1 都抓到，B 的逐句删除 30 段、逐分句删除 52 个都失败，实施者盖新措辞的五条也抓到；新加一句相反的话照过（审查 P3S2B-N3，不改：测试钉的是该有的句子在，新加的文字由审查看 diff） |
| `HealthRule` 在 `cursor === undefined` 时不渲染"更多" | `upgrade-check.test.tsx` 的"更多从第一页读" |
| 审阅一节的读不经 `itemsReader`（403 不重读列表） | `upgrade-check.test.tsx` 审阅一节的 403 一格 |
| 审阅一节留在这一次进入的 `SWRConfig` 之外 | `upgrade-check.test.tsx` 的"离开再进入从存下的报告重新开始"（审阅一节的键不带这一次进入的 `nonce`，留在外面就接着旧的条目，4.3）。实施者的两种写法：审阅中整个不经 `SWRConfig` 连带让它在进出审阅时卸下又装上，先被检查一节的两格抓到（s3-03）；`SWRConfig` 一直挂着、审阅一节放在它外面才是这一行（s3-03b），被这一格抓到 |
| 没有报告时也显示"批准" | `upgrade-page.test.tsx` 的按钮一格 |
| 审阅中仍显示"放弃"（决定 291） | `upgrade-page.test.tsx` 的 `reviewing` 一行（`:175-196`）与 `sectionsInOrder` 的审阅一行（`:856-867`） |
| `reviewing` 的横幅仍写"放弃"（决定 291） | `upgrade-banner.test.tsx` 的两格（`:49-51`、`:127-129`） |
| 批准用 `upgrade.notReady` | `upgrade-page.test.tsx` 的"批准答 `wiki.upgrade_not_ready`"一格（设计审查 P3C-M1） |
| 批准成功之后又读升级（经 `change`） | `upgrade-page.test.tsx` 的"不再读升级"一格（设计审查 P3C-M2）；UP7、UP8 的页面版本 |
| 退回的说明按打开对话框时的状态定 | `upgrade-page.test.tsx` 的"重读之后已不在审阅中"一格（设计审查 P3C-N14；实施者的 s3-09）。"按页面此刻的升级定"是 S3 建成时的实现，这一格看不见（审查 P3S3A-I1 = C-M1，下面） |
| 退回之后不清 `checked` | `upgrade-page.test.tsx` 的"退回之后检查一节没有提交之前的结果"一格（设计审查 P3C-N15） |
| 退回的 409 显示服务端的 `detail` | `upgrade-page.test.tsx` 的 `sendBackChecking` 一格 |
| 不设 20 秒的再读、或卸下之后不清 | 同上（假的计时器）；实施者拆成 s3-12a–d（不读、不清、卸下时不清、状态变了不清）：卸下之后 SWR 绑定的 `mutate` 对没挂着的键不发请求，"卸下之后不再读"看不见计时器没清，改为"离开 5 秒再回来"一格，另加"`mode` 带来报告之后不再读"一格（3.25） |
| 批准之后不跳转、或不交 `arrived` | `upgrade-page.test.tsx` 的焦点一格 |
| 假服务端的退回在没有报告时答 200 | `upgrade-page.test.tsx` 的 409 一格（替身要在真东西会失败的地方失败） |
| `HealthScope` 的键不带提交的时刻（实施者的 o05）；报告的卡的 React 键只是规则（o20） | o05：`upgrade-check.test.tsx` 的"the report's cards are its submission's"；o20 没有（等价：卡的状态在按提交分的缓存里，`HealthRule` 内部按 `identity` 重置，同一格在 o20 下通过；对照 o05） |
| 卡的初值的游标不看 `count_capped`、条目已全也给"更多"、计数为 0 的不收进 `<details>`、卡的标题是 `h2`、初值的条目为空、时刻写提交的、数不写"+"（o01–o04、o21–o23） | `upgrade-page.test.tsx` 审阅中的报告一格（每张卡整组）；`upgrade-check.test.tsx` 的"更多从第一页读" |
| 审阅一节不显示树读失败、卡不交 `treeFailed`、`GateNode` 不看它、`reading` 不看 `isValidating`（o06–o09） | `upgrade-check.test.tsx` 树读失败两格与重试挡住的一格 |
| 报告一节不写"升级的状态变了"（o10）；`itemsReader` 的 403 不看 `current()`（o29） | `upgrade-check.test.tsx` 的 409 一格与"换了一次提交之后才到的 403 不重读"一格 |
| 批准的 `not_ready` 不重读、任何拒绝都重读、`wiki.upgrade_state` 不重读、假服务端从不答 `not_ready`（o11、o12、o28、o18）；store 的 `behindReview` 少 `wiki.upgrade_state`、多 `not_ready`（o16、o17） | `upgrade-page.test.tsx` 批准的拒绝表；`notebook.store.test.ts` |
| 对话框的正文与语气、三个按钮没有 ref、批准成功之后 `reread()`、"更多"不按 `item_key` 去重、退回的任何拒绝都挂计时器（o13–o15、o24–o27、o19、o30） | `upgrade-page.test.tsx` 三个对话框、批准成功、断网一格；`health-rule.test.tsx`；o27 另由 UP7、UP8 的页面版本 |
| 批准答复时页面已离开也跳转（审查 A 的 ra01、B 的 b12：去掉 `mounted()`） | S3 建成时没有（P3S3A-M1 = B-M2）；修复一轮之后 `upgrade-page.test.tsx` 的"approved once the page is left…" |
| 离开页面时审阅一节的"更多"不中止（ra04、b08：`controllers` 登记到别的 `Set`） | S3 建成时没有（P3S3A-M2 = B-M3）；修复一轮之后 `upgrade-check.test.tsx` 的"the report's More under way, the page left…" |
| 驳回、退回不先盖住答复（ra10、ra11、b06、b07：`change(…, false)`） | S3 建成时没有（P3S3A-N4 = B-M1）；修复一轮之后"rejected / sent back, the page shows the … answered while it reads … again"两格 |
| 退回的说明随之后的读改变、重读回来之前先说"正在计算"（审查 A 的 A1、A2，C 的 REVIEWC-1、2）；`checking` 不看 `error`（c21） | S3 建成时实现就是这样（P3S3A-I1 = C-M1），c21 照过（P3S3C-N1）；修复一轮之后"refused, then its report kept…""…the read again held""…another admin approved it meanwhile"三格（C 的 k1–k3 同） |
| 拒绝之后的重读被 `mode` 事件的读超过时，说明读到旧的缓存（审查 A 的 A8，修复核对一轮） | S3 修复一轮之后没有（P3S3A-M3）；修复二轮之后"…its read again overtaken by the rejection's mode event…"一格（不再读一次、A8 原样的期望都死） |
| 20 秒的次数在状态变了之后不清（A 的 A3、C 的 REVIEWC-4）；`sendBack` 开头不把次数归零（ra05） | S3 建成时实现就是这样（P3S3A-N1 = C-N2），ra05 照过；修复一轮之后"…the new submission still checked"与"…then sent back again, the network failing"两格（ra05 不是等价的）；计时器到点不清 `waiting`（A 的 rb08）没有（等价：到点之后不再挂，只有读出错又好了才会再挂一次） |
| 卸下之后仍在重连时读升级（审查 B 的 `leak`、`leakA`、`leakM`，实施者的 `leak-after-unmount`） | S3 建成时 UP7、UP8 的页面版本都照过（P3S3B-I1）；修复一轮之后 UP8 与 UP7 三种浏览器（`letInAtHome`），vitest 带事件流的"approved, …nor by a connection after it" |
| 批准之后不清缓存（A4）；跳转之前就清（A 的 A10，修复核对一轮） | 修复一轮之后"approved, then Back…"；跳转之前就清是修复一轮的实现（P3S3A-N5），修复二轮之后"approved, the upgrade's page goes without showing the upgrade loading…"一格（跳转之后清、跳转之前清都死；从不清死在"approved, then Back…"） |
| 别的管理员退回之后检查一节仍是提交之前的结果（A 的 A6），回到 `upgrading` 一律清（A6b），只看换过提交（A 的 rb05），只看显示过报告（rb04）（决定 300）；健康度页不交 `treeFailed`、`readingTree` 不看 `tree.error`（H1、H2，C 的 k5、k6，决定 301） | 修复一轮之后"…sent back by another admin…"一格、`upgrade-check.test.tsx` 别人的提交拿走又还回检查一节的两格、健康度页的"tree failure has its own retry…"与 lifecycle 的"missing More nodes…"；rb04 活（P3S3A-N6），修复二轮之后死在"…taken over and submitted again…(decision 300)"一格 |
| 审阅一节 `current()` 不看登录、不看 `alive`（ra03、b09、b10，检查一节的 c01、c02 同）；键去掉 `upgrade.id`（ra08）；批准的拒绝不看 `mounted()` 就重读（ra12）；审阅一节闸门的节点不看 `treeFailed`（b01）；批准答 `forbidden`、`notebook.not_found` 也重读升级（b11）；`checkItems` 恒带 `{ cursor }`（c05） | 没有（等价：换登录是新一代的 providers、整页重挂，`alive` 为假时这一次进入的缓存已清；`submitted_at` 唯一；卸下之后绑定的 `mutate` 对没挂着的键不发请求；存下的报告只在闸门全过时有，带节点的闸门通过时节点表为空；store 先重读列表、`useSWR` 的键随之为 `null`；openapi-fetch 跳过 `undefined`） |

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

**S1 的实测**（10.1）：本机第四份挪后 193.9、191.8 秒、第六份 242.2、242.2 秒，用本机挪前的两轮算比、折到 7763 约 394、370 秒；第七份 182.8、185.9 秒、第八份 187.2、185.8 秒，折到约 321、318 秒；P3 加的测试比上面估的少（`TestUpgradeApproval` 10.0 秒、`TestUpgradeReviewInterleavings` 2.7 秒、`TestUpgradeReview` 1.9 秒、`TestUpgradeTrashReservedTitles` 1.1 秒）。分支 CI（43088294571）八份的 Test 一步 377、238、411、345、308、333、322、307 秒：第七份在 7763 上 322 秒；第四、六、八份都在 9V74 上，照第八份的 9V74 → 7763 之比 1.189 折约 411、396、359 秒，都在 450 之下，最贴近的是第四份；以合并之后两次 `main` 为准（第 8 节第 1 条）。

**S2 的实测**（10.2）：本机第三份 236.9、231.7 秒（S2 几乎没有增减），第七份 191.8、190.4 秒（比 S1 多 6.8 秒，两个新测试合 3.6 秒，上面估约 6 秒），折到 7763 约 400、331 秒；分支 CI（43129936469）第三、七份都在 7763 上，412、325 秒（第七份比上面预计的约 345 少）。第三份是离 450 秒最近的一份：在 7763 上 411、401、393、419、412 秒，合并之后两次 `main` 看它（审查 P3S2B-Q1）。

**要不要把契约在进程里只读一次**（P2 第 9 节：每建一个应用 `apitest.Load` 一次，竞态构建下约 0.63 秒）：P3 不做。它是测试设施的取舍，牵动全部组合根的测试；这一轮挪测试已够，留给 M12 的测试设施（第 9 节）。

**成本的作业**：costs-1 加批准的守卫：本机约 4–5 分钟（四个模板，7.1；7763 上约 7–9 分钟；S1 实测本机 111–121 秒，分支 CI 在 Xeon 8370C 上守卫一行包 253 秒、作业 20:05，10.1），另加这个作业里第一次编译不带竞态的 `bootstrap` 测试（costs-1 今天各行都是 platform 与 linking 的；costs-2、costs-3 已有这次编译），7763 上约 1 分钟（设计审查 P3B-N7）。基数取 S3 的三次运行（修订核对第二轮 B 以 `gh run view` 取）：

| 运行 | CPU | 作业 | Test 一步 |
|---|---|---|---|
| 43033276772 | EPYC 9V74 | 14:31 | 11:18 |
| 43027580603 | Xeon 8573C | 15:06 | 11:50 |
| 43022417961 | Xeon 8370C | 15:41 | 12:18 |

作业从开始到 Test 一步约 3:10–3:19。P3 之后作业约 23–26 分钟。正常跑一遍这一行就用掉 Go 默认的 10 分钟超时的七到九成，慢的 runner 上会红在 Go 的超时上而不是守卫自己的界上，所以这一行显式带 `-timeout 15m`（`Makefile:433-436` 回滚先删那一行的 `-timeout 20m`、`:471-478` 守卫的 `-timeout 15m` 是先例；"从不提高 `-timeout`"是组合根分份的规矩，不管成本的行；设计审查 P3B-M7）。作业时限照两条判（修订核对第二轮 P3B-M8，第 294 条）：

- **约两倍**（`ci.yml:38`、`:85`：runner 的 CPU 可差两倍，各份的时限约为 CI 上实测的两倍）：作业约 23–26 分钟，两倍是 46–52 分钟；今天的 35 分钟只有约 1.35–1.5 倍，比它慢 1.4–1.5 倍的 runner 会在第 35 分钟被整个取消、Go 来不及打出卡住的测试（`ci.yml:81` 记的 42093077490 那一种）。照决定 267（costs-2 35 → 45）的先例，costs-1 的时限从 35 分钟提到 45 分钟。
- **Go 的超时先到**：这一行排在 costs-1 的最后，约在作业的第 15–17 分钟开始（审查 B 的估计，含作业前面的几步与这次编译），加 15 分钟是第 30–32 分钟，早于 45 分钟。

（第一轮修订照"开始的时刻加 Go 的时限早于 35 分钟"写了"作业时限不改"，漏了约两倍的那一条。）

**e2e 的作业**（设计审查 P3C-N16）：P3 加 UP6–UP9，UP7 的整个文件（页面、API、MCP 三个版本）在三种浏览器里跑，约多 12–14 个用例、本机约多 40–60 秒；P2/S3 的分支 CI 的 e2e 作业 8:42（E2E 一步 7:09），估 P3 之后约 10–11 分钟，作业限 20 分钟（`ci.yml:255`），不改。实测写进 10.3：S3 的分支 CI（43169439216）e2e 作业 7:58（E2E 一步 6:41，538 个），余约 12 分钟；本机比 S2 末多 9 个、合约 11 秒（UP6、UP8 的页面版本，UP7 的页面版本在三种浏览器，UP7 的 API、MCP 版本的 Firefox、WebKit；UP9 在 S2），比上面估的少。UP7 的 API、MCP 版本也在三种浏览器里跑是这里写好的，实施者把它列为偏离（第 8 条），审查 C 判不是。

### 7.6 P3 怎样量

- **设计时**（这一份初稿）：临时 worktree（`955fa579`），经 `heavy.sh`，`df` 28G。
  - 组合根第四、六、七、八份：`make -j1 test-go-bootstrap-<n> GO_TEST_RACE="go test -race -count=1 -timeout 15m -v"`，每份两轮，P2 的分份旁支的 `measure.sh` 与 `times.py`；数在 7.5。
  - 批准的重建：一个只在临时 worktree 里的探针测试（`page/adapter/postgres`，真库）：上面的种子在 10 万个来源（1,000 个文件夹）与 5 万个来源（500 个）各跑；两种统计（从未分析；分析过）× 两种方式（`plan_cache_mode` 的 `force_custom_plan` 与 `force_generic_plan`），每格三次，在一个回滚的事务里依次计时 `ForgetSources`、`SourceNodes`、十块 `SourceHeads`、附件的读、十块 `SetSourceHashes`、`DeleteUpgradeNodes`，并 `EXPLAIN` 每一条。结果：10 万个来源合计 2.44–3.68 秒（两轮），5 万个来源 0.99–1.19 秒（两轮，几乎相同）；`SetSourceHashes` 的几种写法都省不下（3.4）。计划（只是自定义计划：带参数经扩展协议发的 `EXPLAIN` 把参数当常量折进计划，`plan_cache_mode` 管不到它，探针的 `force_generic_plan` 一栏其实也是自定义计划；通用计划由审查 B 以 `EXPLAIN (GENERIC_PLAN)` 补量，`nodes`、`page_contents`、`upgrade_nodes` 都经键，见设计审查记录与 3.4 的"界与分块"；设计审查 P3B-M1）：5 万个来源、每格之前 `VACUUM ANALYZE` 的两轮里没有顺序扫描，`nodes` 经 `nodes_notebook_id_*` 的索引与 `nodes_pkey`，`ForgetSources` 在分析过之后经 `nodes_notebook_id_kind_source_hash_idx`；10 万个来源的第二轮在同一个库里反复回滚、关着 autovacuum 之后再 `ANALYZE`，`ForgetSources` 与 `LintNodeMetadata` 换成了整表的顺序扫描（这本笔记本占 `nodes` 的约 20%：110,003 / 560,003，设计审查 P3B-N10 改正初稿的"约 16%"；表里堆着几百万条死行），所以守卫每次批准用一份新副本（第 280 条）。
- **修订时**（设计审查之后）：临时 worktree（`955fa579`），经 `heavy.sh`，`df` 27–28G；以审查 B 的探针为底，比决定 292 的读头的四种写法在两个种子（这一本占 `nodes` 11.4% 与 22.4%）× 三种统计状态 × 自定义与通用（`EXPLAIN (GENERIC_PLAN)`）上的计划，只看计划、不计时；结果在 3.4。
- **实施时**：S1 的第 1 个提交照 P2 的分份旁支量挪前挪后（第四、六份各两轮），写进 10.1；守卫照 7.1 量两轮定界；P3 的新测试在第七、八份的实际用时在 S1、S2 的记录里。 S1 照做：第四、六份在 `75f5d077` 与它加 1 号的 `Makefile` 上交替各两轮（`measure.sh`），第七、八份在头上各两轮，守卫四轮（两轮定界，3.9 s），`-list` 的归类用 `m11/tools/shards8-p3.py`；数在 10.1 与 [S1 的审查记录](reviews/P3-S1-review-actions-review.md)"实测"。 S2：第三、七份在头上交替各两轮（`shards/measure.sh`，S2 没改 `Makefile` 的模式），`-list` 照旧用 `shards8-p3.py`；数在 10.2 与 [S2 的审查记录](reviews/P3-S2-playbook-docs-review.md)"实测"。 S3：没有改服务端，不量组合根的份；UP6–UP8 的页面版本在头上经 `heavy.sh` 单独跑三轮（`tools/pages3.sh`，UP7 三种浏览器），数在 10.3 与 [S3 的审查记录](reviews/P3-S3-web-review.md)"实测"。

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
   - 13.4 第 3 条（`:1264`）末尾加——S1："M11/P3：UP6、UP7 的 API 与 MCP 版本、UP8 的 API 版本；UP6 的'驳回之后 MCP 写被拒'以另一本笔记本的导入占住导入 worker、回滚排队时确定地断言。"S2："UP9 只有 MCP 版本。"S3："UP7 整个文件（API、MCP 与页面三个版本）在三种浏览器里跑，UP6、UP8 的页面版本只在 Chromium。"（照 7.5 改，审查 C）
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

### 10.1 S1：审阅（服务端）（2026-11-12，合并 `daf14dd1`）

- **P2 完成之后的 `main`**（S1 开工之前）：记合并号的 `0ffe1758` 与 `docs: M11/P2 done` 的 `955fa579` 一起推，那次的 CI（run 43033276772）e2e 四次（attempt 1–4）都卡在"Install the Playwright browsers"：apt 的镜像 azure.archive.ubuntu.com 每个包都超时，作业到 20 分钟被取消，E2E 一步没有跑；其余 18 项第一次就绿，八份的 Test 一步 376、218、403、310、351、227、203、242 秒。外部故障，代码不改、不开旁支；attempt 2–4 只重跑 e2e。代码与它相同、只多本文的 `75f5d077`（run 43060437171）19 项一次全绿，e2e 8:41（镜像恢复），作为这份代码的 e2e 证据；那一次第四份在 EPYC 7763 上 439 秒（S3 之后三次 445、440、439），第三份 398、第五份 417 秒（都在 7763 上）。S3 之后两次 `main`（43027580603、43033276772）八份的 Test 一步平均 365.5、294.5、369.5、377.5、389、277、234、265.5 秒，都在 450 秒之下（P2 设计第 8 节第 1 条）。S1 从 `75f5d077` 开出。
- 提交（从 `main` 的 `75f5d077` 开始，照 6.1 的次序，7 个；Step 的文档提交是主持者的；修复一轮另有 4 个（下面），一共 11 个）：
  - 第 278 条：第四份的四个（两个后台撤销的停止、撤销最新的变更集、唯一的管理员停用时有人加入）挪进第六份；第 290 条：第八份加 `Approval|Review`；`Makefile` 的计数注释与量出的数，`ci.yml` 三行注释，时限不变（`f86b7a6d`）。
  - 审阅的判定：`ReviewOf`（`Reported`、`Checking`、`Stale`，第 283 条）、`JudgeRoots` 与 `RootGates.Failed`（第 284 条，`JudgeUpgrade` 改用它）、`ApprovalNotReady`（第 282 条，与 `UpgradeNotReady` 共用 `keysOf`）；提交的第一个事务改问 `ReviewOf`（3.9）；notebook 的 `ApproveUpgrade` 一条 `UPDATE`（第 281 条）与 `UpgradeStates.Approve`（事务外拒绝、根为零拒绝、写了不是一行是缺陷），wiki 的端口与组合根的转发；已有的四个替身包一层 `unapproved`（`9432d60a`）。
  - 批准的哈希重建：`page/app.RebuildSources`（先清全本、再按块读头与附件的文件、写每块核行数，缺头、缺文件、写少都是错误，store 的错误以 `%w` 包）、按键的 `SourceRebuildHeads`（第 292 条）、模块根 `page.NewSourceRebuild` 与它的 `Rebuild`（事务外拒绝）；三组测试（替身、store、真库的不变式）（`8abb2a1d`）。
  - 批准、驳回、退回：wiki 的两条 SQL 与 `TestTheReviewsRows`；三个用例（3.6–3.8）与单元测试（一个文件，共用的替身 `review_fake_test.go`）；`UpgradeDeps.SourceHashes`、`ApproveTimeout`（请求的四分之三）；契约 `approveUpgrade`、`rejectUpgrade`、`sendBackUpgrade`（`wiki.yaml`，`api/openapi.yaml` 加三个路径，`submitUpgrade` 的一句）与生成物；HTTP 适配器与码的测试；`wiki.Deps` 与组合根；矩阵三行；`TestUpgradeApproval`、`TestUpgradeReview`、`TestUpgradeReviewInterleavings`，流的九步，交错第 7 条的两种退回、第 8 条的批准（`34d77c98`，范围带 `api`、`web`）。
  - 第 181 条：`CheckContract` 把撤销里的恢复当创建判，`revertChangeset`、`startRevert` 声明 `page.virtual_file`，两个模块的码的测试；`contract_test.go` 四格新加、一格改答；`TestUpgradeTrashReservedTitles`（第七份）（`63cc0b84`，范围带 `api`、`web`）。
  - 批准的成本守卫（costs-1 的末尾一行，`-v`、`-timeout 15m`）与判计划的短测试 `TestTheApprovalGuardJudgesItsReads`；costs-1 的作业时限 35 → 45 分钟（第 294 条）（`bba17208`，范围 `server, build, ci`）。
  - e2e：UP6（API、MCP）、UP7（API、MCP）、UP8（API）与六个夹具（`postApprove`/`approveUpgrade`、`postReject`/`rejectUpgrade`、`postSendBack`/`sendBackUpgrade`），导出 `postRevert`（`d7b85bae`）。
  - 每个提交单独在工作树里 build、vet、`make lint-go`、`make gen-check`、short 测试都通过，另跑它自己的（单元、真库、组合根、网页三项、守卫、UP 的故事）；审查 C 在改写之后的 2–6 号上逐个重跑过一轮（S1 的审查记录"门禁与 CI"）。推送之前改写过一次 2–7 号（旧的都没推过，旧头 `4b852247`）：补变异第一轮抓不到的两处（2 号的 `TestAnUpgradeIsApproved` 事务外一格挪到审阅中；6 号的守卫对 `SourceRebuildHeads` 只许按键读），逐个 `cherry-pick`、不用交互的 rebase。说明写进文件再 `git commit -F`；没有推 `main`，没有 force-push。
- 与设计不同之处（实施者的号；都是最小的改动，3.23 列出）：
  1. 已有的四个替身包一层 `unapproved`：两个端口的 `Approve` 同名不同签名，一个类型装不下（3.3）。
  2. `stops.arm` 四个参数（7.1）。
  3. 驳回之后"基线复原"断言为发起写的 `AGENTS` 不在、笔记页的正文同发起时（7.1）；不靠回滚也成立，修复一轮改（审查 P3S1A-N4）。
  4. `api/openapi.yaml` 加三个路径（3.1、3.11）。
  5. `TestUpgradeApproval` 期限的三格共用一个 2 s 的应用（7.1；设计审查 P3B-N9）。
  6. "批准"与"另一个标签页"合成一格（7.1）。
  7. 三个用例的单元测试在一个文件里（3.6–3.8、7.3）。
  8. 退回之后 agent 的写只改 frontmatter：起初写了正文，答 `page.upgrade_immutable`，是测试写错，照设计改；不是与设计不同（审查 C），不进 3.23。
  9. 守卫的另几本是根下 `raw` 收 45,000 页与 2,000 个附件的 LLM Wiki（第 295 条，实施中定；7.1）。
  10. 守卫的计划在第一次批准的事务里、删基线之后判（第 296 条，实施中定）；审查之后改为每条被判的语句第一次真跑之前、在同一连接上判（审查 P3S1B-M2，修复一轮；7.1）。
  11. 判计划时内侧逐行的 `nodes` 认条件里的 `(id = `（`nodes_notebook_id_id_key`）（第 297 条，实施中定；7.1）。
  12. 守卫在批准之前等启动时的作业（7.1）。
  13. 守卫本机 121 秒（设计估 4–5 分钟），`Makefile` 的说明照实测写（6.1、7.5）。
  14. UP6、UP7 的 API 版本经 API 提交、MCP 版本经 `upgrade_submit`；另加四处 409 的拒绝（第 5 节）。
  15. 第四、六份只在 1 号提交上量了挪前挪后，没有在头上再量：2–7 号给这两份加的只有契约里的三个操作（逐个操作的几个测试各多三次），设计估约 1.5%；七、八份才是新测试所在，在头上量了（下面"实测"）。
  16. 7.3 第 16 行做成两条（普通的连接与 `SourceHeads` 的形状）；16a 让变异第一轮发现了守卫的缺口（"读头只经键"，3.23）。
- 实测（本机，经 `heavy.sh`；出处见 S1 的审查记录"实测"）：`go test -race -list` 551 个顶层测试，八份 145、50、120、120、61、15、28、12，各在恰好一份（基点 547 个，新模式下 145、50、120、120、61、15、27、9；与旧模式比，换份的恰是第 278 条的四个）；第四份挪前 215.4、221.4 秒、挪后 193.9、191.8 秒，第六份 211.5、210.8 → 242.2、242.2 秒（四个在第六份里 29.3、29.1 秒，在第四份里 29.5、29.7 秒），用本机挪前的两轮算比、折到 7763 约 394、370 秒（设计预计 394–407、374–387）；第七份 182.8、185.9 秒、第八份 187.2、185.8 秒，折到 7763 约 321、318 秒（设计预计约 345、350–375）；第八份的新测试 `TestUpgradeApproval` 10.0 秒、`TestUpgradeReviewInterleavings` 2.7 秒、`TestUpgradeReview` 1.9 秒，第七份 `TestUpgradeTrashReservedTitles` 1.1 秒，比设计估的 38–53 秒少（设计按 `TestUpgradeSubmit` 一类估）。批准的守卫：四个模板，四轮六格的中位数 1.08–1.29 s，界 3.9 s（本机两轮最大的中位数 1.285 s 的三倍，向上取整到 100 ms），远在 `ApproveTimeout` 11.25 s 之内；72 次计时的批准里 `SetSourceHashes`（五块合计）0.74–0.94 s、`SourceRebuildHeads` 0.14–0.18 s、`LintNodeMetadata` 0.11–0.15 s；22% 的种子上 `SourceRebuildHeads` 两种计划都只经两个主键（`Function Scan` → 逐行 `nodes_pkey` → `Memoize` → 逐行 `page_contents_pkey`），`SetSourceHashes` 每块至多读这本笔记本一遍；整个守卫 111–121 秒。
- 反向对照：7.3 的 S1 各行（1–36）每个点名的测试各跑一次（36 行 46 条，设计列几个测试的各一条）加实施者自己的 7 条，共 53 条；第一轮 51 条抓到。没抓到的两条都是真的缺口：o1（`UpgradeStates.Approve` 不判事务：事务外的一格放在 `upgrading` 上，本来就改不到行；挪到审阅中）；16a（`SourceRebuildHeads` 写成普通的连接：守卫的通用规则放行"按笔记本读一次 `nodes`"，没有落实 7.1 的"读头只经键"；补这一条与短测试一格）。第二轮 4 条（o1、16a、16b 即 `SourceHeads` 的形状、o8 即去掉新规则的对照）都抓到；16b 在 22% 的种子、分析过的自定义计划上正是 `Seq Scan on page_contents`。7.3 里没有等价的变异：第 2 行实施者改的是把报告一支挪到期限之后，不是设计点名的等价对调，抓到。
- 门禁（本机，头 `d7b85bae`，改写之后）：`make lint gen-check knip build` 8 秒；经 `heavy.sh`：`make test` 3,549 秒通过（vitest 193 个文件、4,447 个测试；批准的守卫 112.6 秒，六格中位数 1.09–1.22 s，计划都合规），`make test-go-short` 41 秒（含判计划的短测试九格），`make e2e` 138 秒（528 个；UP6 API 341 ms、MCP 355 ms，UP7 API 134 ms、MCP 108 ms，UP8 51 ms）；一轮全绿，没有偶发。
- CI：分支的 run 43088294571（`d7b85bae`）19 项一次全绿；组合根八份的 Test 一步 377、238、411、345、308、333、322、307 秒（第三、七份在 EPYC 7763 上 411、322 秒，第七份与本机折的 321 秒相合；第四、六、八份都在 EPYC 9V74 上，照第八份的 9V74 → 7763 之比 1.189 折约 411、396、359 秒，比本机折的高 10–40 秒，都在 450 之下，以合并之后两次 `main` 为准）；costs-1 在 Xeon 8370C 上 20:05（设计估 23–26 分钟，时限 45 分钟），批准的守卫那一行在作业的第 15.7 分钟开始、包 253 秒，六格中位数 2.25–2.51 s（界 3.9 s，余量约 1.55 倍），`SetSourceHashes` 1.54–1.71 s，计划都合规；costs-2 24:33；e2e 10:43（E2E 7:43，528 个），"Install the Playwright browsers" 2:16 装好。
- **审查**（2026-11-12，三位各在 `d7b85bae` 的自己的工作树里，重的经 `heavy.sh`）：A 审服务端行为的正确性，B 审成本、计划、分份与变异的抓手，C 审契约、HTTP、MCP、矩阵、e2e 与文档稿。A 0I 0M 7N，B 0I 2M 1N 1Q，C 0I 0M 6N；去重之后 Important 0，Minor 2（判计划的规则对 `nodes_pkey` 不看条件、短测试的六个子条件没有反例，P3S1B-M1；第 296 条判的是写过之后的状态下的计划，84 条里 2 条与真跑时不同，P3S1B-M2），Nit 12（去重之后：三份合计 14，P3S1A-N3 = C-N3，A-N7 = C-N2）。三位都没有查出用户或数据会碰到的问题；16 条偏离都判对，第 8 条不是偏离、第 15 条是过程（审查 C），另有 C 列的三处与 A-N3 = C-N3 进 3.23。主持者的修复说明 `fix-1.md`：代码与测试的四个提交（守卫、测试、e2e、注释）；P3S1B-Q1 定为第 298 条；文档的发现（A-N7 = C-N2、C-N1、C-N6）在本提交。
- **修复一轮**（2026-11-12，`a44602b0`；主持者的修复说明 `fix-1.md`）：从 `d7b85bae` 起 4 个提交，普通推送，没有改写已推的历史；6 个文件、加 210 行、减 108 行，没有改产品代码：
  - 守卫与它的短测试：`approveCostScans` 的 `nodes_pkey` 只在条件里有 `(id = ` 时才认，内侧逐行的 `nodes` 条件含 `= ANY` 的拒绝；判计划挪到 tracer 的 `TraceQueryStart`，每条被判的语句第一次真跑之前、在同一连接上判，`SHOW plan_cache_mode` 留在包住 `DeleteBaseline` 的一层（第 296 条）；`EXPLAIN` 出错时先报计划的错误；界的注释写第 298 条的条件；短测试 9 → 19 格（每个子条件一格反例，另一格正例）（P3S1B-M1、M2、N1、Q1）（`9c637540`）。
  - 测试：批准的闸门表加"来源根是根下的 `AGENTS`"；退回的拒绝表加"别的升级的行"；批准一格补 `checkLinks`、`checkSearch`；驳回一格先退回、agent 改 `Note` 的 frontmatter、再交、再驳回，断言正文回到 `cleanNote`；退回一格与 `getUpgrade` 之后的读整值比较；换期限的钩子先断言 `ApproveTimeout` 是 3 s（P3S1A-N1–N6）（`cc751dca`）。
  - UP7 以 `node:crypto` 对写下的字节算的 SHA-256 比 `source_hash`，测试名照实（P3S1C-N4）（`9dac9244`）。
  - `Makefile` 的两处计数注释改为头上的数（P3S1C-N5）（`a44602b0`）。
  - 与 `fix-1.md` 不同之处 7 条（3.23）："内侧不许 `= ANY`"加了；短测试多一格正例；A-N4 先退回再写（审阅中冻结 agent 的写）；A-N5 保留逐项断言；注释多写第七、八份的数；"ran no %s"挪进 `end()`；第 296 条的说法。
  - 变异（`mut.py`，经 `heavy.sh`，改前在 `d7b85bae`、改后在 `a44602b0`，12 条）：B 的 r1–r6、A 的 a1、a2、n4（回滚的 `RollbackState` 不出正文的版本）、o6（第八份的那一格）改前活、改后死；r7（`nodes_pkey` 不看条件）、r8（去掉"内侧不许 `= ANY`"）改后死。
  - 逐个提交（`verify.sh`，经 `heavy.sh`）：四个各自 `make lint-go`、`make gen-check`、`make test-go-short`（40–41 秒）rc=0；1 号守卫一轮 117.8 秒（六格中位数 1.07–1.17 s，计划都合规，短测试 19 格）；2 号 `wiki/...` 与竞态的 `TestUpgradeApproval`、`TestUpgradeReview`（11.2、2.0 秒）；3 号 `lint-web`、`knip` 与 UP7 的两个；4 号 `make -n test-go`。
  - 门禁（头 `a44602b0`）：`make lint gen-check knip build` 9 秒；经 `heavy.sh`：`make test-go-short` 40 秒，`make e2e` 133 秒（528 个）；照修复说明不跑整轮 `make test`。
  - CI：run 43099927925（`a44602b0`）19 项一次全绿；八份的 Test 一步 393、337、401、308、415、370、268、213 秒，第六份在 EPYC 7763 上 370 秒（本机折的约 370 秒），第四份在 Xeon 6973P-C 上；costs-1 在 Xeon 8573C 上 18:46，批准的守卫 222.6 秒，六格中位数 2.01–2.23 s（界 3.9 s，余量约 1.75 倍），计划在每条语句真跑之前判、都合规；e2e 9:06（E2E 7:20，528 个），L1 没有出。
- **修复核对一轮**（2026-11-12，三位各在 `a44602b0` 的自己的工作树里，C 另核文档稿）：A 判 P3S1A-N1–N6 关上，a1、a2、n4、o6 在头上都死，没有新发现；B 判 P3S1B-M1、M2、N1、Q1 关上（短测试 19 格过，r1–r8 各有一格失败，另加的 r9 被正例抓到，第一轮的 5 个探针都被拒；24 次判计划里"内侧不许 `= ANY`"没拒掉真的计划；第一轮探针的两条 `ForgetSources` 现在判的是真跑时的计划），新的 P3S1B-N2（Nit：批准在跑到被判的语句之前就失败时，`PlanErr` 先报、只有状态码，丢了答复的正文）不改：只影响守卫失败时的诊断，不影响判定；C 判 P3S1C-N3–N5 关上（N4 由上传时的 SHA-256 多喂一个字节的变异核过：旧的 UP7 照过、新的失败），文档稿里 N1、N2、N6 与 A-N7 落对、本节与 00 号文档第 13 节的 CI 数经 `gh api` 逐项核过，新的 P3S1C-N7（Nit，文档稿：A-N4 的修复漏了"先退回"）在本提交改。一轮之后没有 Important、Minor 留下，不再有修复轮。
- 实施者留下的（报告第 9 节）：Step 的文档提交是本提交（3.23、10.1、7.3 第 16 行的措辞、"读头只经键"的出处、00 号文档第 11 节的 295–297、移交与总体设计）；守卫在慢的 runner 上的余量（第 298 条）；第四、六、八份没有 7763 的样本（合并之后两次 `main` 为准）；`TestTheApprovalGuardJudgesItsReads` 在 short 一行；S2、S3 的部分。量用的临时工作树与 `GOTMPDIR` 的目录已删；这一 Step 没有手动起容器。
- **合并之后**：run 43112344486（`daf14dd1`）19 项第一次 attempt 全绿；八份的 Test 一步 392（EPYC 7763）、367（7763）、393（7763）、284（EPYC 9V45）、279（Xeon 6973P-C）、373（7763）、317（7763）、263（Xeon 8573C）秒，都在 450 之下，第四份挪走四个之后不再贴线（第 278 条）；costs-1 在 EPYC 9V74 上 16:16（Test 一步 13:19），批准的守卫六格中位数 1.79–1.91 s（界 3.9 s；第 298 条的条件看 7763 上的数，这一次不在 7763 上）；e2e 7:55（E2E 6:14）。第二次是记合并号的文档提交的 run，第 298 条与第四、六、八份在 7763 上的数以两次为准。 第二次：run 43117118697（`d33b4544`）19 项第一次 attempt 全绿；八份的 Test 一步 383、363、419（EPYC 7763）、396（7763）、422（7763）、255、312、213 秒；两次的平均 387.5、365、406、340、350.5、314、314.5、238 秒，都在 450 之下，第四份挪走四个之后在 7763 上 396 秒，第三份两次 393、419 秒是离线最近的一份（10.2）；costs-1 这一次在 EPYC 7763 上 21:24（Test 一步 17:53），批准的守卫六格中位数 2.23–2.37 s，都在 3.25 s 之下，第 298 条不触发、界不改；e2e 9:02。

### 10.2 S2：playbook 与文档（服务端）（2026-11-12，合并 `7d2e895a`）

- **S1 合并之后的 `main`**：见 10.1 末条（`daf14dd1` 与记合并号的 `d33b4544`，两次都一次全绿，第 298 条不触发）；S2 从 `d33b4544` 开出。
- 提交（从 `main` 的 `d33b4544` 开始，照 6.2 的次序，3 个；Step 的文档提交是主持者的；修复一轮另有 3 个（下面），一共 6 个）：
  - prompt `upgrade`：`wiki/domain/playbook-upgrade.md`（3.17 的正文，去掉首尾空白 4,827 字节）与 `UpgradePlaybookText`（第 286 条，不进 `PlaybookText` 的 `switch`）；用例 `UpgradePlaybook`（`lint.read` 之后读 wiki 的行，只在 `upgrading` 答文字，别的答 `wiki.upgrade_state`，不读模式）；笔记本入口的第四个 prompt（`NotebookEntry`、没有参数），`api/mcp/prompts.json` 一项；`upgrade_submit` 的说明加"or sends it back for more work"（契约两个入口、`wiki/adapter/mcp/tools.go` 的常量与 `testdata/tools.json`，设计审查 P3C-N4）；模块根导出两个 playbook 的文字给组合根的测试；单元测试三组；`TestThePromptReadsLLMWikiMatrix` 只读三个、`TestThePromptListsAreTheContract` 改为四；第七份新的 `TestUpgradePromptMatrix`（四个 prompt × 四种状态 × 13 列，每种升级状态一份库的副本，答文字的格整值比较）与 `TestUpgradePrompt`；`Makefile` 的计数注释（`567f1201`，范围 `server, api, build`）。
  - e2e：UP9（MCP）；LW11 的帮手挪进 `fixtures/mcp.ts`（`promptTextOf`），标题与两处注释照第 5 节末（`9d7e9004`）。
  - 文档：README 的"升级为 LLM Wiki"一节与它让之不实的几处（`:234`、`:236`、`:246`、MCP 一节的两个工具、`:299` 的工具数 18 与 20、`:319` 的注解与放行的样例；行号是 `d33b4544` 的 README 的，合并之后 `:299`、`:319` 是 `:314`、`:334`），一并改的 `:214`（内置说明的字节数照实测，它从 P2 起就过时，不是 S2 让它不实）；MCP 人工清单的 K1–K5、`:8`、`:308`、`:323` 与 A1、A3、A5、B1 的工具数（`d2512eeb`）。
  - 每个提交单独在它的树上 build、vet、`make lint-go`、`make gen-check`、short 测试都通过；1、2 号另跑 `lint-web`、`knip`、`test-web`；1 号另跑它的组合根测试与读工具表、契约的测试（竞态，16 个顶层）和 e2e 的 MC1、LW8、LW11，2 号跑 UP9、LW11、MC1，3 号跑链接检查；审查 B 在 1、2 号上各自检出另跑过一轮。没有改写历史。1、2 号的标题照实施说明写行为的改变（6.2 写的是名词短语），1 号改了 `Makefile` 的注释，范围带 `build`（实施者的第 8 条，过程，不进 3.24）。
- 与设计不同之处（实施者的号；都是最小的改动，3.24 列出）：
  1. `upgrade_submit` 的说明改在三处：常量在 `tools.go`，`TestLintToolObjects` 读 `testdata/tools.json`（3.15）。
  2. `UpgradePlaybookText` 的测试在新的 `wiki/domain/playbook_test.go`：设计引的 `playbooks_test.go` 是 `wiki/app` 的。设计 3.16 写的就是这个文件，不是偏离（审查 A），不进 3.24。
  3. 模块根导出 `UpgradePlaybookText`、`PlaybookText`：组合根只能 import 模块根（3.18）。
  4. `TestUpgradePromptMatrix` 的两份副本在任何应用连上准备好的库之前做；每份副本设四本活着的矩阵笔记本；三个应用各建 13 个 PAT（3.18）。
  5. `TestUpgradePrompt` 的审阅中由真的提交得到、回滚中以 SQL 设（3.18）。
  6. LW11 的 `textOf` 挪进 `fixtures/mcp.ts`，UP9 共用（3.15、第 5 节）。
  7. README 写"两个入口另有 `upgrade_verify` 与 `upgrade_submit`"：两个工具在两个入口都登记（第 196 条；3.19 写"升级中的笔记本入口"）。
  8. 1、2 号提交的标题写行为的改变，1 号范围带 `build`（6.2；过程，只记在这里）。
- 实测（本机，经 `heavy.sh`）：`go test -race -list` 553 个顶层测试，八份 145、50、120、120、61、15、30、12，各在恰好一份，与 S1 末比只多了第七份的两个；第三份 236.9、231.7 秒（均值 234.3，折到 7763 约 400 秒），第七份 191.8、190.4 秒（S1 末 182.8、185.9，折到 7763 约 331 秒）；`TestUpgradePromptMatrix` 2.6 秒、`TestUpgradePrompt` 1.0 秒（设计估约 4、2 秒）。playbook 4,827 字节（界 16 KiB）。内置说明照旧：笔记本入口 SDK 3,276、账户入口 3,346 字节，余 820、750。
- 反向对照：7.3 的 S2 三行，第 1 行跑矩阵、单元与 `TestUpgradePrompt` 三条，第 2 行单元一条（矩阵另跑一条作对照：照设计看不见，看不到的笔记本在 HTTP 的边上就 404），第 3 行跨两个文件（`multi.py`）；等价的"只给 `PlaybookText` 加一支"照过，以第 3 行作对照（设计审查 P3B-N3）。实施者自己的 17 条（没有升级、回滚中也答文字，读升级出错当成状态，先读后判，`wiki.upgrade_not_found`，"still being checked"的 `detail`，交错笔记本，丢 `description`，漏登记，登记在两个入口，`Playbooks.Execute` 放过普通笔记本，不去空白，界缩到 4 KiB 的对照，说明只改一边，契约多一个 prompt，答 lint 的文字，第一行）与 playbook 要的 36 句每句一条（每一处都换掉），都抓到。审查之后知道，36 句那一组把出现两次的码两处都换，掩住了"一句顶替另一句"；片段止于条件与码，做法换掉照过（审查 P3S2B-M1 = C-M1，下面）。
- 门禁（本机，头 `d2512eeb`）：`make lint gen-check knip build` 9 秒；经 `heavy.sh`：`make test` 3,491 秒通过（vitest 193 个文件、4,447 个测试；批准的守卫 117.5 秒，六格中位数 1.08–1.20 s），`make test-go-short` 41 秒，`make e2e` 136 秒（529 个，UP9 55 ms）；一轮全绿。
- CI：分支的 run 43129936469（`d2512eeb`）19 项第一次 attempt 全绿；八份的 Test 一步 363、230、412、283、254、374、325、267 秒，S2 碰的第三、七份都在 EPYC 7763 上（412、325 秒，本机折的约 400、331 秒），都在 450 之下；costs-1 在 EPYC 9V74 上 15:31，批准的守卫六格中位数 1.64–1.80 s；e2e 9:12（E2E 7:35，529 个），L1 没有出。
- **第三份**（审查 P3S2B-Q1）：S2 没有改动它的时长（`TestThePromptReadsLLMWikiMatrix` 78 格同前，契约的 `api/dist` 没变），但它是八份里离 450 秒最近的一份：在 EPYC 7763 上 411（S1 分支）、401（S1 修复一轮）、393（S1 合并之后的 `main`）、419（S1 记合并号的 `main`）、412（S2 分支）秒，平均约 407（S2 修复一轮的分支 CI 第三份在 EPYC 9V45 上，282 秒，不算 7763 的样本）。合并之后两次 `main` 看它（第 8 节第 1 条），现在不挪；要腾地方时，`TestThePromptReadsLLMWikiMatrix` 的 78 格是 `TestUpgradePromptMatrix` 同样 78 格的子集，可以只留它对 `prompts.json` 的名单断言（审查 A、B 的"过度设计"）。
- **审查**（2026-11-12，三位各在 `d2512eeb` 的自己的工作树里，重的经 `heavy.sh`）：A 审用例、登记与组合根的 prompt 测试，B 审测试看不看得见、e2e 与分份，C 审 playbook、README、人工清单的 K 与文档稿。A 0I 0M 2N 1Q，B 0I 1M 1N 1Q，C 0I 2M 7N；去重之后 Important 0，Minor 2（playbook 的"必有的句子"只钉条件与码、不钉做法，4.13 清单里五项删掉照过，`page.upgrade_immutable` 被另一句顶替，P3S2B-M1 = C-M1；人工清单的 K 没写第二个客户端怎样做，P3S2C-M2），Nit 10，Q 2（`AGENTS` 模板的"三个 prompt"，P3S2A-Q1 → 第 299 条；第三份贴线，P3S2B-Q1）。三位都没有查出 playbook、README 或 K 说错码的意思、指错路；8 条偏离都判对，第 2 条不是偏离、第 8 条是过程。主持者的修复说明 `fix-1.md`：三个提交（只改测试与注释；playbook 的两处措辞；README 与人工清单）；文档的发现（P3S2C-N7）、决定 299 的移交与 3.24 的取舍在本提交。
- **修复一轮**（2026-11-12，`7daa3452`；主持者的修复说明 `fix-1.md`）：从 `d2512eeb` 起 3 个提交，普通推送，没有改写已推的历史；7 个文件、加 96 行、减 69 行，产品代码只改 playbook 的文字：
  - 测试与注释：`TestUpgradePlaybookText` 的"必有的句子"每格是一整句，一句里有两件事时是一个分句，4.13 清单的六项各有一格（`page.upgrade_immutable` 钉在"只改 frontmatter"那一句），每格断言恰好出现一次（`strings.Count`），合起来覆盖第一行之外的整段正文；单元表加"审阅中、有报告"一格；适配器 `prompts.go` 与组合根 `TestTheWikiPromptsAreReached` 的注释（P3S2B-M1 = C-M1、B-N1，A-N1、N2）（`94db07e4`，46 格）。
  - playbook 的三处措辞：闸门 4 写"every page outside the source root has valid frontmatter, and every one but the source root and AGENTS has type and summary"；"server_busy from either tool means wait retry_after_seconds and call it again"；`notebook.upgrading` 那一句后接"and forbidden from upgrade_verify or upgrade_submit means your user is not an admin of it"；测试的三格随之改，`notebook.upgrading` 那一句拆成两格，共 47 格；4,827 → 4,979 字节（P3S2C-N3、N4）（`8321734b`）。
  - README 的三处（第 2 步的 frontmatter、MCP 一节的"笔记本的管理员在升级中用得上"、"根下若有……"）与人工清单的 K（两个客户端各用自己的一本、K5 共用普通笔记本、照 J 的准备与"逐客户端记"、K4 的提交不过）（P3S2C-M2、N1、N2、N5、N6）（`7daa3452`）。
  - 与 `fix-1.md` 不同之处 4 条（3.24）：片段走到整句或分句；`forbidden` 写"is not"、点出两个工具名；K 表后连 J 的第二句一起抄来；pb02、pb16 随新原文换写法，另加 5 条盖新的措辞。另有过程一条：2 号的标题约 150 字符，写全了三处措辞的行为变化。实施报告写 46 格，是 1 号上的数，2 号起是 47（核对一轮的 P3S2B-N2）。
  - 变异（`mut.py`，经 `heavy.sh`，改前在 `d2512eeb`）：C 的 c1–c7、B 的 pb01–pb20、B 的逐句删除里照过的五段与 A 的 a1 共 33 条改前活，1 号之后都死；2 号之后 38 条（pb02、pb16 换成新原文上的同一种改法，另 5 条盖新的措辞）都死。逐句删除 30 段：改前 9 段照过，1、2 号之后都失败。
  - 逐个提交（`verify.sh`，经 `heavy.sh`）：1、2 号各自 `make lint-go`、`make gen-check`、`make test-go-short`（40 秒）、`wiki/...` 的单元、组合根四个 prompt 测试（竞态，包 7.8、7.7 秒）rc=0；2 号另 `make build` 之后 UP9、LW11；3 号只改 README 与人工清单。
  - 门禁（头 `7daa3452`）：`make lint gen-check knip build` 8 秒；经 `heavy.sh`：`make test-go-short` 41 秒，第三份 236 秒、第七份 189 秒（同建成时），`make e2e` 133 秒（529 个）；照修复说明不跑整轮 `make test`。
  - CI：run 43138853267（`7daa3452`）19 项一次全绿；八份的 Test 一步 274、374、282、316、247、374、223、210 秒，第三份这一次在 EPYC 9V45 上（282 秒，不是 7763 的样本），第二、六份在 7763 上各 374 秒；costs-1 在 EPYC 9V74 上 20:11（Test 一步 16:42），批准的守卫六格中位数 2.14–2.41 s（界 3.9 s；不在 7763 上，第 298 条不看）；e2e 8:05（E2E 6:26），L1 没有出。
- **修复核对一轮**（2026-11-12，三位各在 `7daa3452` 的自己的工作树里，C 另核文档稿）：A 判 P3S2A-N1、N2 关上（a1 在头上单元失败），新的三处措辞与代码的码一致，没有新发现；B 判 P3S2B-M1、N1 关上（逐句删除 30 段、逐分句 52 个、pb01–pb20 在头上都失败，47 格各恰好一次，4.13 的六项各有一格），新的 P3S2B-N2（Nit：格数是 47，不是 46）在本提交，P3S2B-N3（Nit：新加一句相反的话照过）不改：这个测试钉的是"该有的句子在"，新加的文字由审查看 diff；C 判 P3S2C-M1、M2、N1–N6 关上（c1–c7 改前活、改后死），N7 在文档稿里关上，文档稿的取舍、10.1 的第二次 `main` 与第三份的样本核过（S1 修复一轮的 401 秒从作业日志查明在 7763 上），新的 P3S2C-N8（3.17 的块漏了 `forbidden` 那半句）、N9（3.20 的块还是初稿）、N10（决定 299 与 M12 第 40 条的一条理由不实：没有断言钉这一句）都在本提交改。一轮之后没有 Important、Minor 留下，不再有修复轮。
- 实施者留下的（报告第 9 节）：没有要编号的产品决定；Step 的文档提交是本提交（3.24、10.2、00 号文档、两份移交与总体设计）；README 先于网页说了 S3 的事（3.24）；UP9 只有 MCP 版本（第 5 节、10.1 的例外）；`GOTMPDIR` 已删，没有手动起容器，没有修剪 Go 的缓存。
- **合并之后**：run 43149201273（`7d2e895a`）19 项第一次 attempt 全绿；八份的 Test 一步 375（Xeon 8573C）、290（EPYC 9V74）、407（EPYC 7763）、393（7763）、262（EPYC 9V45）、253（9V45）、238（9V45）、216（9V45）秒，都在 450 之下，第三份在 7763 上仍是最贴近的一份（B-Q1）；costs-1 在 Xeon 8573C 上 15:53（Test 一步 13:07），批准的守卫六格中位数 1.67–1.74 s（界 3.9 s，第 298 条不触发）；e2e 8:49（E2E 7:19）。第二次是记合并号的文档提交的 run，第三份的均值以两次为准。 第二次：run 43153150986（`5a303fde`）19 项第一次 attempt 全绿；八份的 Test 一步 390（EPYC 7763）、291（EPYC 9V74）、400（7763）、407（7763）、310（9V74）、319（9V74）、330（7763）、210（EPYC 9V45）秒；两次的平均 382.5、290.5、403.5、400、286、286、284、213 秒，都在 450 之下，第三份（两次都在 7763 上）403.5、第四份 400 秒最贴近（B-Q1）；costs-1 这一次在 Xeon 8370C 上 19:40（Test 一步 16:17），批准的守卫六格中位数 2.23–2.50 s，都在 3.25 s 之下，第 298 条不触发、界不改；e2e 8:01（E2E 6:47）。

### 10.3 S3：网页（2026-11-12，合并 `07abd0f7`）

- **S2 合并之后的 `main`**：见 10.2 末条（`7d2e895a` 与记合并号的 `5a303fde`，两次都一次全绿，第 298 条不触发）；S3 从 `5a303fde` 开出。
- 提交（从 `main` 的 `5a303fde` 开始，照 6.3 的次序，3 个；Step 的文档提交是主持者的；修复一轮另有 2 个、修复二轮 1 个（下面），一共 6 个）：
  - 报告经 `HealthRule`：`HealthRuleItem`、游标可为 `undefined`（下一次从第一页读）、`summary`/`paths`/`read` 的类型、可缺的 `treeFailed`；健康度页的两处（`paths` 放宽，`read` 没有游标时抛缺陷）；`checkItems` 的游标可缺；`itemsReader`；审阅一节经 `HealthRule`（初值的游标照 `count > items.length || count_capped`，计数为 0 的在 `<details>`），在这一次进入的 `SWRConfig` 里，`HealthScope` 的键带这次提交的时刻；树读失败两节说失败、节点空着、重试时"加载中"；UP5 页面版本的两处断言（`4484ca7c`，范围 `web, e2e`）。
  - 三个动作：service、store（`behindReview` 是 `forbidden`、`notebook.not_found`、`wiki.upgrade_state`，批准不收 `not_ready`）、审阅中的按钮一行（批准只在有报告时，不给放弃，决定 291）、三个对话框、批准自己的函数（成功不写缓存不重读，页面还装着时跳到首页、`arrived`）、退回之后清 `checked`、退回在算时的文字与 20 秒的再读、横幅的"驳回"一句、两种语言、假服务端的三条路由（`ee298a0a`）。
  - e2e：UP6–UP8 的页面版本，`threeBrowsers` 加 `upgrade\/up7-`，`fixtures/upgrade-pages.ts` 的 `changeButtons`、`reviewDialog`（`05423d08`）。
  - 每个提交单独在干净的临时 worktree 里 `make build`、`lint-web`、`knip`、`test-web`、`gen-check` 与全部升级故事、LW8 都通过；审查 B 在 1、2 号上各自检出另跑过一轮。推送之前改写过两次（变异有活下来的，补的测试并进 1、2 号；3 号另 `--amend` 一次），都非交互地重建、核对新头的树等于旧头加改动，推过的历史没有改写。标题照实施说明写行为的改变（6.3 写的是名词短语；实施者的第 9 条，过程，不进 3.25）。
- 与设计不同之处（实施者的号；3.25 列出）：
  1. 提交的标识是 `submitted_at`：契约的 `Upgrade` 没有 `submission_id`，S3 不碰服务端（4.3；00 号文档决定 287 与 4.3 的另三处照改，审查 P3S3C-N3）。
  2. `reportJSON` 不加形状：`keptReport` 已是要的形状，`upgrade-check.test.tsx` 的报告在测试里组（4.3）。
  3. UP7 的批准那一页的流最后才放开：首页放开时下一次载入截断连接的重读（WebKit）；"放开之后的连接不读升级照样看得见"的理由不成立（审查 P3S3B-I1）。修复一轮改回首页放开、等连接的最后一层答了再断言、再去下一页，偏离照此重写：第 5 节没写的只是这个"等"（第 5 节）。
  4. 树读失败的提示在列节点的地方之上（4.3）。
  5. 重试时"读着"先于"失败"（4.3）。
  6. 退回的说明按页面此刻显示的升级定：审查判不对（P3S3A-I1 = C-M1），修复一轮改代码，修复报告删去这一条，不进 3.25。
  7. 报告一节的卡被拒为状态变了时写"升级的状态变了"，同检查一节（4.3）。
  8. UP7 的 API、MCP 版本也在三种浏览器里跑：不是偏离，7.5 本就这样写；第 5 节与第 8 节第 6 条的句子照 7.5 改（审查 C）。
  9. 提交的标题写行为的改变（过程，只记在这里）。
- 实测（本机，经 `heavy.sh`）：vitest 193 个文件、4,521 个测试（S3 多 74 个，没有新文件），`make test-web` 57–58 秒；`make e2e` 538 个、142 秒（S2 末 529 个、136 秒）；UP6–UP8 的页面版本三轮每轮 7.2–7.6 秒（UP6 2.4–2.9 秒，UP7 Chromium 1.6–1.7、Firefox 2.4–2.5、WebKit 2.5 秒，UP8 约 1 秒），都通过；审查 B 另以 `--repeat-each=5` 跑 25 个，全过。
- 反向对照：7.3 的 S3 14 行 19 条（"不设或不清"拆成四条，"不跳转或不交 `arrived`"两条，"留在 `SWRConfig` 之外"两种写法）与实施者自己的 30 条，共 49 条；48 条抓到，没抓到的 o20（报告的卡的 React 键不带提交的时刻）是等价的，以 o05（`HealthScope` 的键不带它）作对照。写代码时活下来的四条各补了测试：计时器不清（卸下之后 SWR 对没挂着的键不发请求，"卸下之后不读"看不见，改为离开再回来，另加状态变了一格）、`reading` 不看 `isValidating`（重试挡住的一格）、`itemsReader` 的 403 不看 `current()`（换了提交之后才到的 403）、退回的任何拒绝都挂计时器（断网一格）。审查之后知道，"不跳转""不中止""不先盖住答复"三处没有测试（去掉 `mounted()`、`controllers` 登记到别处、`change(…, false)` 都照过），"卸下之后仍在重连时读升级"的 `leak` 在 UP7、UP8 都照过，`checking` 不看 `error`（c21）也照过（下面）。
- 门禁（本机，头 `05423d08`）：`make lint gen-check knip build` 8 秒；经 `heavy.sh`：`make test` 3,652 秒（vitest 4,521 个；批准的守卫六格中位数 1.15–1.29 s），`make test-go-short` 43 秒，`make e2e` 142 秒（538 个）；一轮全绿。
- CI：分支的 run 43169439216（`05423d08`）19 项第一次 attempt 全绿；八份的 Test 一步 389（EPYC 7763）、292（EPYC 9V74）、270（EPYC 9V45）、338（Xeon 8573C）、377（8573C）、395（7763）、230（9V45）、271（9V74）秒，都在 450 之下，S3 没改服务端，各份与 S2 的差别来自机器；costs-1 在 EPYC 9V74 上 16:06（Test 一步 13:16），批准的守卫六格中位数 1.65–1.96 s；e2e 7:58（E2E 6:41，538 个），作业限 20 分钟（7.5），L1 没有出。
- **审查**（2026-11-12，三位各在 `05423d08` 的自己的工作树里，重的经 `heavy.sh`）：A 审网页的行为，B 审测试看不看得见、e2e 与逐个提交，C 审设计一致、文字、无障碍、回归与文档稿。A 1I 2M 4N 1Q，B 1I 3M 2N，C 0I 1M 5N 1Q；去重之后 Important 2（退回的说明随页面此刻的升级变，P3S3A-I1 = C-M1；批准之后重连不读升级没有测试看得见，P3S3B-I1），Minor 3（不跳转、不中止、不先盖住答复三处没有测试），Nit 9，Q 2（别人退回之后清检查一节，P3S3A-Q1 → 第 300 条；健康度页树读失败时一直"加载中"，P3S3C-Q1 → 第 301 条）。三位都没有查出码、请求次数、投影或焦点上的错；偏离第 1、2、4、5、7 条判对，第 3 条的理由不成立，第 6 条不对，第 8 条不是偏离，第 9 条是过程。主持者的修复说明 `fix-1.md`：两条 Important、三条 Minor 与 Nit 里改代码或测试的几条，加两个决定，按 `web` 与 `e2e` 分提交；文档的发现（P3S3C-N3–N5）与 3.25 的取舍在本提交。
- **修复一轮**（2026-11-12，`bc1a24e5`；主持者的修复说明 `fix-1.md`）：从 `05423d08` 起 2 个提交，普通推送，没有改写已推的历史；11 个文件、加 440 行、减 61 行，没有服务端的改动：
  - 网页：退回的说明在拒绝之后那一次重读回来时定一次（`change` 对四个码等重读再抛出，`sendBack` 从缓存读这一次重读的结果，记下那个拒绝与它的提交），20 秒只为被拒的那一次提交挂，重读答 404 时写通用的文字；别的管理员退回之后同样清掉检查一节（决定 300）；批准之后把升级从缓存里清掉；健康度页树读失败时交 `treeFailed`（决定 301）；假服务端每次提交给新的 `submitted_at`；vitest 4,531 个（多 10 个），新加或改写的格：A1、A2、REVIEWC-3、REVIEWC-4、ra05 的断网一格，批准时已离开、后退、带事件流的"approved"，驳回、退回先盖住答复，审阅一节的"更多"离开时中止，决定 300、301 各一格，驳回的任务整行（P3S3A-I1 = C-M1、C-N1、A-N1 = C-N2、A-M1 = B-M2、A-M2 = B-M3、A-N4 = B-M1、A-N2、A-N3、B-N2、B-I1 的 vitest 一半，决定 300、301）（`a040dcc1`）。
  - e2e：`fixtures/upgrade-pages.ts` 的 `letInAtHome`；UP7、UP8 的批准那一页在首页放开流、等连接的第四层答了再断言不读升级，UP7 再去变更集页，末段的放开与断言删去；UP6 末段的注释（P3S3B-I1、N1）（`bc1a24e5`）。
  - 推送之前在本地改写过两次：删掉一个没人读的字段；逐个提交的核对在 `make test-web` 里红了一格（`health-page-lifecycle.test.tsx` 的"missing More nodes…"，决定 301 的后果：树再读失败时"更多"带来的节点也空着），改了那一格并进网页那一个。
  - 与 `fix-1.md` 不同之处 4 条（3.25）：`change` 等重读影响六个对话框（修复报告写五个，漏了结束回滚，审查 A）；批准之后先跳转再清缓存（核对一轮查明会闪，修复二轮改）；决定 300 的"显示过报告"按这一次进入读到的算；决定 301 也管树再读失败。实施者的偏离 3 照最后的做法重写，偏离 6 删去。
  - 变异（`mut.py`，经 `heavy.sh`，改前在 `05423d08`、基线 270 个，改后基线 280 个）：ra01（A5）、ra04、ra05、ra10、ra11、c21 改前活；A1–A4、A6、REVIEWC-4、REVIEWC-H 对应的是改前缺的行为，由 A、C 的临时测试在 `05423d08` 上复现；改后 16 条全死（A1–A6、A6b、ra05、ra10、ra11、ra04、c21、H1、H2、`leak`、`leak-after-unmount`）。e2e：B 的 `leak` 改前 UP7、UP8 都过，改后 UP8 与 UP7 三种浏览器 4 个全红；o27 照旧 4 个全红。
  - 逐个提交（干净的 worktree，经 `heavy.sh`）：两个各自 `make build`、`lint-web`、`knip`、`test-web`（4,531 个，58 秒）、`gen-check` rc=0，升级的全部故事与 LW8 107 个通过（36 秒）。UP6–UP8 页面版本 `--repeat-each=3` 一次与三轮各起一次 Playwright（每轮 6.6–7.4 秒），UP7 三种浏览器 12 次都过。
  - 门禁（头 `bc1a24e5`）：`make lint gen-check knip build` 12 秒；经 `heavy.sh`：`make test-web` 59 秒（4,531 个），`make e2e` 139 秒（538 个）；照修复说明不跑整轮 `make test`。
  - CI：run 43181567930（`bc1a24e5`）19 项一次全绿；八份的 Test 一步 385（EPYC 7763）、366（7763）、327（EPYC 9V74）、344（9V74）、290（Xeon 6973P-C）、373（7763）、242（EPYC 9V45）、323（7763）秒，都在 450 之下；costs-1 这一次在 EPYC 7763 上 21:09（Test 一步 17:37），批准的守卫六格中位数 2.21–2.40 s，都在 3.25 s 之下（第 298 条的条件看合并之后的 `main`，这是分支的一次）；e2e 9:24（E2E 7:32），L1 没有出。
- **修复核对一轮**（2026-11-12，三位各在 `bc1a24e5` 的自己的工作树里，C 另核文档稿）：A 判 P3S3A-I1、M1、M2、N1–N4 关上（A1–A6 在头上照新的期望过；变异里 rb04 活、rb08 等价，其余改坏的都死），决定 300 做对了，偏离 1 可以接受，新的 P3S3A-M3（Minor：拒绝之后的重读被 `mode` 事件的读超过时，说明读到旧的缓存、一直写"正在计算"）、N5（Nit：批准之后清缓存时升级页还挂着，跳转之前闪成"加载中"）、N6（Nit：决定 300 的"换过提交"一支没有测试）由修复二轮改；B 判 P3S3B-I1、M1–M3、N1、N2 关上（7 条变异改后全死，`leak` 在 e2e 4 个全红，头上 UP6–UP8 页面版本三次各 `--repeat-each=3`、45 个全过，UP7 三种浏览器各 9 次；`a040dcc1` 单独绿），新测试没有同类的空断言，没有新发现；C 判 P3S3C-M1、N1、N2、Q1 关上（6 条变异都死），N3–N5 在文档稿里关上，新的 P3S3C-N6–N9（Nit，文档稿：UP7 的说法照修复之后写、决定 301 与 4.3 加上树再读失败、4.3 写明六个对话框都等重读、M12 第 41 条不用 `W/`）在本提交改。
- **修复二轮**（2026-11-12，`a25fef6d`；主持者的修复说明 `fix-2.md`）：从 `bc1a24e5` 起 1 个提交，普通推送；只改 `upgrade-page.tsx` 与它的测试，2 个文件、加 119 行、减 5 行：
  - 退回被拒之后，缓存里这一项仍 `isValidating`（拒绝之后的重读被 `mode` 事件的读超过，SWR 丢了它的答复）时再 `await reread()` 一次，再读缓存、定说明（P3S3A-M3）；批准成功、页面还装着时记下 `approvedAway`，`UpgradeRead` 卸下时才清缓存（P3S3A-N5）；决定 300 的"换过提交"一格（P3S3A-N6）；vitest 多 3 格（A8、A10 的写法与"换过提交"），新的三格连跑 15 次都过。修复报告另更正修复一轮第 5 节第 1 条：受影响的是六个对话框。
  - 变异（`mut.py`，经 `heavy.sh`，改前在 `bc1a24e5`、基线 280 个，改后 283 个）：rb04、"清缓存挪到跳转之前"改前活，新的 M3、N5 两格放到 `bc1a24e5` 的页面上红（复现 A8、A10）；改后 6 条全死（不再读一次、A8 原样的期望、照修复一轮在跳转之后清、跳转之前清、从不清、rb04）。
  - 核对（干净的 worktree，经 `heavy.sh`）：`make build`、`lint-web`、`knip`、`test-web`（4,534 个，59 秒）、`gen-check` rc=0，升级的全部故事与 LW8 107 个通过（34 秒）；UP6–UP8 页面版本一轮 5 个全过（7.5 秒）。
  - CI：run 43189150092（`a25fef6d`）19 项一次全绿；八份的 Test 一步 393（EPYC 9V74）、369（EPYC 7763）、324（9V74）、342（9V74）、349（9V74）、363（9V74）、232（EPYC 9V45）、311（7763）秒，都在 450 之下；costs-1 在 EPYC 7763 上 21:31（Test 一步 17:57），批准的守卫六格中位数 2.21–2.37 s，都在 3.25 s 之下；e2e 8:19（E2E 6:32），L1 没有出。
- **修复核对二轮**（2026-11-12，审查 A 在 `a25fef6d` 的自己的工作树里）：P3S3A-M3、N5、N6 关上（A8、A10、rb04 改前活、改后死，新加的 4 条变异都被抓到），没有新的竞态；新的 P3S3A-N7（Nit："再读一次"不看拒绝的码，断网等拒绝碰上有读在飞时也多读一次升级）不改：只在断网而有读在飞时多读一次，无害。两轮之后没有 Important、Minor 留下，修复到此为止。
- 实施者留下的（报告第 9 节）：提交的标识（第 1 条，S3 用 `submitted_at`；要 `submission_id` 是服务端与契约的改动，不在 S3，审查与主持者照 `submitted_at`）；没有要编号的产品决定（"实施中定"的几条记在 3.25）；Step 的文档提交是本提交（3.25、10.3、00 号文档、两份移交、总体设计与 M12 的打磨）；o20 的 React 键留着与检查一节一个写法；4.3 的 `upgrade.service` 的测试（"若有"）没有这个文件，三个动作的路径与方法、`checkItems` 不带游标由页面的测试经假服务端的请求记录看住；没有手动起容器，临时 worktree 已删，没有修剪缓存。
- **合并之后**：run 43199957538（`07abd0f7`）19 项第一次 attempt 全绿；八份的 Test 一步 336（EPYC 9V74）、367（EPYC 7763）、380（Xeon 8573C）、419（7763）、302（9V74）、273（EPYC 9V45）、238（9V45）、315（7763）秒，都在 450 之下；costs-1 在 9V45 上 12:51（Test 一步 10:18），批准的守卫六格中位数 1.43–1.57 s（第 298 条不触发）；e2e 6:52（E2E 5:38）。第二次是记合并号与 P3 完成这次推送的 run，记进 M11 收尾的记录。

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
