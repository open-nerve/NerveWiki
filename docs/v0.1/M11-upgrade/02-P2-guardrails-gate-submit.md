# M11/P2 护栏、闸门与提交：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M11/P2 护栏、闸门与提交（三个 Step：S1 护栏（服务端）；S2 闸门、检查与提交（服务端）；S3 网页） |
| 日期 | 2026-11-10（初稿）；2026-11-10（照设计审查第一轮修订）；2026-11-10（照修订核对第一轮修订）；2026-11-10（照修订核对第二轮修订，加第 267 条）；2026-11-10（照 S1 的实施与审查修订）；2026-11-11（照 S2 的实施与审查修订）；2026-11-11（照 S3 的实施与审查修订） |
| 状态 | 完成（2026-11-11；S1 已合并 `8cf7af36`，S2 已合并 `c61df45a`，S3 已合并 `5d3a0f35`；设计的[审查记录](reviews/P2-design-review.md)，两轮修订核对在它的末两节；S1–S3 的审查记录 [P2-S1-guardrails-review.md](reviews/P2-S1-guardrails-review.md)、[P2-S2-gate-check-submit-review.md](reviews/P2-S2-gate-check-submit-review.md)、[P2-S3-web-review.md](reviews/P2-S3-web-review.md)；实施与审查之后与代码不同之处照实改在各节（3.19–3.21 列出），结果在 10.1–10.3） |
| 上级文档 | [M11 总设计](00-M11-design.md) 4.6、4.7、4.8 的提交、4.13 的两个工具与说明、4.14、4.15 的检查与提交、4.16–4.18、第 5、7、8、9、10 节、第 11 节（尤其 180、182、186–191、196、197、208、210、215、216、219、222、232、236、240、244、249）与"移交的落实""写给后面的 M""总体设计的修订"里属于 P2 的条目；[P1 设计](01-P1-freeze-start-rollback.md)（写法，3.25–3.28 的修正，第 9 节交给 P2 的事）；[P1 的审查记录](reviews/)（设计与 S1–S4）；[v0.1 总体设计](../v0.1-design.md) 5.8、6.3、13.1–13.4 |
| 代码位置的写法 | 同 00 号文档与 P1 设计：模块写作 `page/app/unit.go:392`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；迁移写作 `00056_page_upgrade_nodes.sql:9`（`server/migrations/sql/`）；前端写作 `W/pages/notebook/upgrade-page.tsx:144`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/stories/upgrade/up4-guardrails.spec.ts:23`；契约写作 `api/modules/wiki.yaml:163`。行号都是 P1 收尾之后的 `main` `f002672b` 的（它在初稿读的 `d589e5a2`、`455f692c` 与 `ce370441` 之上只改了文档：`git diff d589e5a2 f002672b -- server api`、`git diff 455f692c f002672b -- web e2e Makefile .github` 都为空），修订时照它复核过一遍 |
| 工作笔记 | 代码的核对分四份（page 的单元、守卫与批的计划；wiki 的 lint、`Runs` 与升级的用例；MCP 的注册、码表与说明；网页、e2e、组合根的测试设施与 CI），本文引的每个行号都在其中核对过（附录）。设计不跑重的东西，要量的在 7.6；审查者 A、B、C 的实测（守卫读正文的代价、闸门的读的计划与时长、CI 各份的用时）写在审查记录里，本文引它们的数 |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P2 分三个 Step，各自一个分支（`m11-p2-s<m>`）、一次审查与修复核对、一次合并；后一个从前一个合并之后的 `main` 开始。每个提交都绿，服务端的 Step 带上它改的契约在网页与 e2e 上要的东西（生成的类型、码的两种语言的文字、假服务端的字段），网页的 service、store 方法与组件只在 S3 加（knip）。P2 没有迁移：账本的列 `upgrade_nodes.accepted_hash`（00056）、报告的列 `notebook_upgrades.report` 与提交的三列（00057）在 P1 都已建好。

- **S1 护栏（服务端）**（`m11-p2-s1`）：
  - 不改写链接的移动与改名：`Step.KeepLinks`，`page.upgrade_keep_links`（3.2）；
  - 正文的切分与逐步的比较：`domain.UpgradeBody`、`StepContent.BodyHash`、守卫读基线页的正文（`UpgradeNodes.BaselineContent`），`page.upgrade_immutable`；升级中一次 `batch` 盲写的已有页按 head 的字节数计进读的界（第 266 条）（3.3）；
  - `batch` 的到达：计划的 `Reached`（此前的移动或改名改写过这一页就带着，升级中另照批前的链接判；第 268、270 条）、`ContentWrite.Reached`、两个正文的码标在 `Last`，`page.upgrade_batch_rewrite`（3.4）；
  - 账本：单元在每个正文步之后写 `accepted_hash`，升级中先核对它等于写之前的哈希（3.5）；
  - 三个码进 `mcpToolCodes()`、`upgradeToolCodes()`、`mcpOwnCodes()`；`TestUpgradeGuardrails` 与 UP4 补全；护栏的成本守卫（3.6、7.1）。
  - 网页：不改。三个码只在 MCP（第 210 条），REST 的操作不声明它们，`problem-messages.test.ts` 不要它们的文字（同 P1 的 `page.upgrade_protected`，P1 设计 4.1）。
  - 单独为绿：只有发起人的 MCP 写在 `upgrading` 中到得了护栏（P1 的冻结），别的写的行为不变；`StepContent.BodyHash` 只在 `upgrading` 时为不是 `Rewrite` 的正文步由单元算，账本只在笔记本有升级时写，普通笔记本的单元多一次判断、零额外读写。
- **S2 闸门、检查与提交（服务端）**（`m11-p2-s2`）：
  - 第一个提交开组合根的第八份（第 261 条，6.2）；
  - `Runs` 的三种新键与结果的泛化、`logFinished` 的名字；lint 的 `prepareIn` 交回它读到的节点，`WantedByPage` 可与摘要同用、第二遍另成一项（3.8）；
  - page 的闸门的读：根下的 `AGENTS` 与来源根、基线行、内容保全、本次升级的变更集数（3.9）；
  - 闸门与报告的判定（`wiki/domain`）、`UpgradeCheck`、`UpgradeReport` 与它的存储（3.10）；
  - `checkUpgrade`、`listUpgradeCheckItems`、`submitUpgrade`（3.11、3.12）；`Upgrade.report`（3.13）；
  - `upgrade_verify`、`upgrade_submit`（`tools.json`、矩阵、码、说明、`Fit`），"server update"的措辞（3.14）；
  - `wiki.upgrade_not_ready`；矩阵的行、交错、成本守卫（3.15–3.17、7.1、7.2）；
  - 网页：生成的类型、`upgradeJSON` 的 `report: null`、`wiki.upgrade_not_ready` 的文字（4.2）；e2e：UP5（API、MCP）。
  - 单独为绿：合并之后 agent 与 REST 都能检查、提交，提交通过之后升级停在审阅中（批准、驳回与退回在 P3/S1）；审阅中任何管理员可以放弃（P1），所以不会卡住。网页还没有检查与提交的入口（S3），这一段只在开发中的 `main` 上（同 00 号文档第 207 条的处理）。
- **S3 网页**（`m11-p2-s3`）：升级页的"检查"一节（八道闸门、八条规则的卡与"更多"）、"提交审阅…"、自己的提交在算时照实显示审阅中（第 264 条）、提交不过之后自动再检查、审阅中只读地显示存下的报告；`HealthRule` 的"读一页""范围变了的码"与标题级别做成参数（4.3）；e2e：UP5 的页面版本（第 5 节）。

## 1. 基线

从 P1 收尾之后的 `main` `f002672b` 开始（服务端与契约同 `d589e5a2`，网页与 e2e 同 P1/S4 的 `455f692c`，P1/S4 合并为 `ce370441`，其后两个提交只改文档）。P1 留下的、P2 要用的：

- **冻结的守卫**（`page/app/freeze_guard.go`）：`GuardWrite`（`:50-69`）在 `FreezeAsk` 且 `Grants.Ask(notebook.change_mode)` 准了之后调 `guardrails`（`:77`），今天只判 `OpDelete`（`:78`）；读者是 `UpgradeNodes`（`:15`，只有 `Protected`），由模块根以 `postgresadapter.New(pool)` 交（`page/freeze_guard.go:22-24`）。登记在 `[freeze, mode, lock]` 的最前（`bootstrap/registrants.go`，P1 设计 3.4）。
- **写入单元**：`Step`（`page/app/extension.go:57-77`）有 `Content *StepContent`、`Rewrite bool`、`Batch *BatchRewrite`，没有"不改写链接"的标记；`StepContent`（`:81-85`）只有哈希、字节数、`Merged` 与之前的版本号。`applyStep`（`page/app/unit.go:381-428`）：守卫的循环 `:392-396`，之后 `ensureChangeset`、写、`writeSourceHashes`、`recordItems`、参与者；`step()`（`:366-368`）不带任何单元的状态。`u.keepLinks`（`:349`）只在 `Relocate`（`page/app/unit_relocate.go:40-41`）里为批的移动设上，`follow`（`page/app/unit.go:545-579`）读它决定推后的改写跟不跟（`:564`），守卫看不到。`appender.WriteContent`（`:769-773`）给参与者的写设 `Rewrite = true`。`WriterDeps`（`:25-58`）没有 Markdown 的端口，模块根在 `page/module.go:302` 建了 `md`、`:304` 建单元时没有交它。
- **正文的写**：生产里每个正文的写都经 `Unit.WriteContent` → `writeContent`（`page/app/unit_content.go:50-107`）→ `apply`：`put_page_content.go:64`、`edit_page.go:202`、`write_path.go:222`、`toggle_task.go:86`、`restore_version.go:65`、`unit_revert.go:130`、`:573`、`apply_batch.go:727`，链接的改写经 `appender`（`linking/app/rewrite.go:943`、`rewrite_window.go:144`，组合根 `bootstrap/linking.go:271`）。字节不变的写在守卫之前返回（`unit_content.go:66-78`）。`LockContent`（`:51`）只交回版本号、哈希与字节数（`page/app/ports.go:110-114`），不读正文。
- **批**（M9/P3）：单元整体 `UpdateLinks: true`（`page/app/apply_batch.go:254-256`）；移动以 `Placement.KeepLinks: !s.UpdateLinks`（`:770`）进 `Relocate`；一页的正文在它的第一个正文操作处写最终的正文（`content`，`:706-739`，`pg.First != i` 时什么也不做，`:717`），出错经 `a.failed(i, err)`（`:621-634`）标在 `i`。计划（`PlanBatch`，`page/domain/batch_plan.go:328`）带着没加锁读到的笔记本的状态（含 `Upgrade`，`page/domain/mode.go:26-40`）；计划的页（`:265-274` 的 `PlannedPage`，由 `:986` 造）有 `First`、`Last`；内容一遍（`page/domain/batch_contents.go:34-`）在写的路径以 `holdsAt`（`:183-197`）、编辑的路径（`:90-108`）以 `MovesBetween` 把此前的移动对这一页的改写折进正文，`ps.Moved`（`batch_plan.go:233`）只说两个正文操作之间有移动。读的界：`Reads`（`:847-862`）把编辑的页与被移动的改写够着的写的页（`ReadWhole`）列为整读，其余写的页（盲写）只读 head；`CheckReads`（`:884-905`）只把整读的页按 head 的字节数加起来，过 `MaxBatchReadBytes = MaxUnitBytes + MaxBatchStringBytes`（`:19-27`，14 MiB）答 `page.batch_too_large`；调用在 `apply_batch.go:436`。
- **frontmatter 的切分**：`platform/markdown.FrontmatterEnd`（`platform/markdown/frontmatter.go:102-108`，`frontmatterSpan` `:70-96`）；page 经 `Markdown.FrontmatterEnd`（`page/app/ports.go:786-788`，适配器 `page/adapter/markdown/markdown.go:52-54` 不用它的配置）。`AgentsBody`（`page/domain/instructions.go:34-44`）是"切掉 frontmatter、去掉一个 BOM、去掉首尾空白"。
- **lint**：`Lint.Execute`（`wiki/app/lint.go:24-121`）；`compute`（`:196-212`）→ `computeIn`（`:217-234`）→ `prepareIn`（`:239-298`，读状态、显式或隐式的范围、节点与来源，`PrepareRules`）。`prepareIn` 不交回它读到的节点（闸门 3 与报告的计数要它们）。`RuleQuery.WantedByPage`（`wiki/app/lint_ports.go:82-87`）只许与 `wanted` 一条规则同用（`linking/app/lint_rules.go:44-46`），而且在 LLM 契约下连带第二遍（`:120-122`）。
- **`Runs`**（`wiki/app/lint_runs.go`）：`Do`（`:62-124`）只按 `RunKey`（`wiki/app/lint_run_types.go:13-23`）合并，结果是 `domain.LintResult`；调用者的等待取 `min(timeout, 余下的期限/2)`（`:66-77`），到了就答 `server_busy`，不等 worker；flight 的 ctx 是 `WithTimeout(WithoutCancel(ctx), timeout)`（`:91`），名额在 `work` 里取、worker 返回才还；建 flight 时发 `RunCreated`（`:103`，`lint_run_types.go:55-58`）。`logFinished`（`:295-352`，`:300-305`）只认 `LintSummary`、`LintItems`，别的记 `unknown`。`TryHold`（`:130-154`）是 P1 的。组合根的 `lintTimeout = min(10 s, RequestTimeout/2)`（`bootstrap/wire.go:246`）；生产 `request_timeout: 15s`（`server/configs/config.yaml:17`），所以 `LintTimeout` 7.5 s；组合根的测试应用请求 4 s（`bootstrap/app_test.go:63`）、`LintTimeout` 2 s、名额 1（`bootstrap/wiki_lint_runtime_test.go:69` 断言）。REST 路由的期限就是 `RequestTimeout`，带请求体上限的路由（`/mcp` 在内）另加 `BodyReadTimeout`（`platform/httpserver/api.go:297-310`）；期限错误而请求的 ctx 也到期时平台答 500、记 warn（`platform/httpserver/apierrors.go:278`）。
- **升级的用例**（`wiki/app`）：`UpgradeDeps`（`start_upgrade.go:22-40`）与 `locked`（`:47`，时钟在锁与判定之后读，`:62`）；`StartUpgrade`、`GetUpgrade`、`AbandonUpgrade`、`EndUpgradeRollback`；端口在 `upgrade_ports.go`；wiki 的行 `UpgradeRows`（`:110-116`，`Current` 不选 `submission_id` 与 `report`，`wiki/adapter/postgres/queries/upgrades.sql:10-17` 的注释写着"Its report is P2's"）。`domain.Upgrade`（`wiki/domain/upgrade.go`）没有提交的 id 与报告。模块的 `Deps`（`wiki/lint.go:108-135`）与 `New`（`:149-172`）。
- **契约**：`api/modules/wiki.yaml` 有 `getLint`、`listLintItems`（`:7-85`）与四个升级的操作（`:86-223`）；`Upgrade`（`:780-857`）没有 `report`。lint 的 schema：`LintSummary`（`:322-343`）、`LintPage`（`:344-365`）、八种条目（`:366-549`）与八种摘要（`:550-722`）。
- **MCP**：wiki 的 `Tools`（`wiki/adapter/mcp/tools.go:18-27`）有 `lint`、`pending_sources`，两个入口；`tools.json` 笔记本入口 16 个、账户入口 18 个（`api/mcp/tools.json`，`lint` 在 `:506-533`、`:1157-1188`），14 个只读工具都是 `"idempotentHint": false`、不写 `destructiveHint`；`annotationsOf`（`platform/httpserver/mcpserver/tool.go:150-156`）只在 `Destructive` 为真时写它，`TestTheAnnotationsAreTheHints`（`mcpserver/tools_test.go:420-432`）只有两格；内置说明的工具段 `page/adapter/mcp/instructions.go:36-52`，SDK 的编码下笔记本入口约 3093、账户入口约 3163 字节（`instructions_test.go:66-70` 的界 3796）。码表 `mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:93-123`）、`upgradeToolCodes()`（`bootstrap/upgrade_tool_codes_test.go:18-24`，注释 `:13-17` 写着它由第七份答出）、`mcpOwnCodes()`（`bootstrap/mcp_client_test.go:335-341`；工具的错误带上没有声明的码就失败，`:305-334`）。
- **网页**：升级页 `W/pages/notebook/upgrade-page.tsx`（`UpgradeRead` `:64-222`：`upgrading` 有接入一节 `:155`、`upgrading` 与 `reviewing` 有"放弃" `:166-170`，`reviewing` 今天只有状态与放弃，P1 决定 248 把它的内容留给"审阅一节"；三个对话框在按状态的各节之外，`:176-220`；`change` 只对 `transfer.busy` 与 `wiki.upgrade_state` 重读，`:116-122`）；`UpgradeService`（`W/services/upgrade.service.ts`：`start` `:17`、`abandon` `:30`、`endRollback` `:39`、`get` `:48`）；`NotebookStore` 的三个升级动作（`W/stores/notebook.store.ts:154-194`）；`modeChanged` 重读 `["upgrade", notebook_id]`（`W/events/handlers.ts:279-283`）；健康度的 `HealthRule`（`W/pages/notebook/health-rule.tsx:48`，读一页 `:92-96`、模式的核对 `:114`、`:119-122`、`:160` 与卡的 `h2`（`:183`）都写死）；健康度页每次进入一个 `nonce`（`W/pages/notebook/health-page.tsx:74`），键带 `locationKey` 与它（`:127`）；SWR 的缓存整个应用一个（`W/app/providers.tsx:18`）。
- **测试的设施**：`upgradeIn`（`bootstrap/upgrade_fixtures_test.go:42`，`reviewing` 时写提交的三列、不写报告）、`tm.agent`（`bootstrap/mcp_writes_test.go:21`）、`toolError`（`bootstrap/mcp_client_test.go:305`，`["op"]` 是批的失败操作，来自 `page/adapter/mcp/batch.go:209-226`）、`TestUpgradeGuardrails`（`bootstrap/upgrade_guardrails_test.go:21-91`）、`TestUpgradeInterleavings`（`bootstrap/upgrade_interleavings_test.go:75`）与它的 `heldRules`（`:45-58`：包住 lint 的规则，在 `PrepareRules` 里停到放开，不看 ctx）、`newAcmeTeamOn`（`bootstrap/interleavings_workspace_test.go:98`，配置与 `wiring` 的钩子）、`interleaveOn`（`:191`）、lint 的运行时夹具的 `wiring.wiki` 写法（`bootstrap/wiki_lint_runtime_test.go:40-57`）、发起的成本守卫与它判计划的帮手（`bootstrap/upgrade_start_cost_test.go:89`、`:441`、`:501`、`:526`）、回滚的读的守卫判计划的帮手 `rollbackReadProblems`（`page/adapter/postgres/rollback_cost_test.go:354-419`，关系表 `:369-371` 只有 `upgrade_nodes`、`nodes`、`page_contents`）、lint 的十万页夹具（`bootstrap/wiki_lint_cost_fixture_test.go:41`，配置 `bootstrap/wiki_lint_cost_test.go:31-34`）。第七份 `BOOTSTRAP_SEVENTH = ^Test(Upgrade|IngestBefore$$|ABackground)`（`Makefile:282`）。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P2 一行（`00-M11-design.md:515`）：护栏的其余三块与账本；七道闸门与索引、检查与条目、提交（两个事务、脱离请求的第二段）、存下的报告；两个 MCP 工具；P2 的网页。

不做（P3）：批准（`approveUpgrade`、`page.NewSourceRebuild`）、驳回、退回（`sendBackUpgrade`，含"审阅中、没有报告"过期之后的退回）、第 181 条、prompt `upgrade` 与 README、审阅页的"更多"与三个动作。P2 的提交在"审阅中、没有报告"过期之后能再提交（4.8 第 1 步），退回的那一半在 P3/S1。

**移交的落实**（00 号文档"移交的落实"里 P2 的部分）：

| 移交的项 | P2 的落实 |
|---|---|
| [M10-mode](handoffs/M10-mode.md) P2 补充：保全 `rewrite` | S1：护栏放行 `Rewrite` 的步（3.3） |
| 同上 P3/S1 补充：显式范围 | S2：检查与条目用显式的 LLM Wiki 契约与提议的来源根（3.11） |
| 同上 P3/S1 补充：交付前授权 | S2：检查与条目在交付之前再判一次 `notebook.change_mode`（3.11） |
| 同上 P3/S1 补充：`stale_pages`；P4 补充：旧格式先 reindex、Unknown 不是 ready | S2：索引一格不过时检查不过、提交答 `wiki.upgrade_stale_index`（3.10、3.12） |
| 同上 P4 补充与收尾补充：根下的保留标题 | S2：闸门 3（3.10） |
| 同上 收尾补充：闸门、第 6 条的数 | S2：逐页（3.10） |
| 同上 收尾补充：内容保全按 `rewrite` 判 | S1 的护栏与账本、S2 的闸门 7（3.3、3.5、3.10） |
| 同上 收尾补充：说明与降级（"upgrade"一词） | S2："server update"（3.14） |
| [M9-mcp-upgrade](handoffs/M9-mcp-upgrade.md) 1 两个工具 | S2（3.14） |
| 同上 2 冻结放行的写：`batch` 一个单元、整个放行或整个回滚 | S1：`batch` 的护栏（3.4） |

### 2.1 对 00 号文档的修订

照代码，00 号文档有下面几处要改（行号是 `f002672b` 的 00 号文档的；第 4–10 节的行号与 `d589e5a2` 的相同，第 11 节在第 838 行之后多了 P1/S4 的一节）。接受本文的文档提交以脚本照改（`p2-apply-00.py`，每处一个带标签的替换，一处不匹配就什么也不写），第 8 节与第 9 节在同一个提交里一并改：

1. **`StepContent.Body []byte` 是哈希，并且守卫要自己的切分**（4.6"比较" `:193`、第 8 节 page 一条 `:649`）：比的是两份 `body` 的 SHA-256，单元只要交"之后的那份"的哈希，`StepContent` 加 `BodyHash []byte`（不是正文本身：一个写最多 `MaxContentBytes` 字节，守卫用不着它）。"之前的那份"由守卫读（00 号文档的写法），守卫也要算 `body`，所以它与单元都要 frontmatter 的切分：`FrontmatterEnd` 不用 platform 的 Markdown 的配置（`page/adapter/markdown/markdown.go:52-54`），适配器加一个包级的 `FrontmatterEnd` 与零大小的 `Frontmatter` 类型，模块根的 `NewFreezeGuard(pool, auth, admin)` 自己交它，签名不变；单元经 `WriterDeps.Markdown`（模块根已有的 `md`，`page/module.go:302`）。守卫读"之前的那份"与"是不是基线页"合成一条读（`UpgradeBaselineContent`，3.3），不是 `Nodes.PageContent` 加一次基线的读。单元只为 `upgrading` 中不是 `Rewrite` 的正文步算 `BodyHash`（守卫不比 `Rewrite` 的步，单个 `move` 的改写页数不设界）。`StepContent.Body []byte` 改为 `StepContent.BodyHash []byte`，第 8 节 page 一条加"`WriterDeps.Markdown`、`WriterDeps.Ledger`、`UpgradeNodes.BaselineContent`"（3.3、3.5）。
2. **`Folded` 取这一页最后一个正文操作时的样子，只修"之后的写整个换掉"这一种**（4.6"`batch` 的折进" `:197`、第 190 条 `:772`）：00 号文档写"一页的任何正文操作有过这样的改写就是 `Folded`"。一页的写（`write`）会整个换掉正文：折进之后又被一个不等于改写结果的写换掉，最终的正文里已没有改写，这时正文变了是 agent 自己的写，该答 `page.upgrade_immutable`，照 00 号文档的写法答 `page.upgrade_batch_rewrite`、叫 agent 去拆，拆了还是被拒。改为：编辑的路径上改写真的改了这一页就设上（编辑保留其余的正文，此前的折进也留着）；写的路径上改写改了这一页、写的恰是改写的结果就设上，写的不等于这一页当时的正文就清掉（它换掉了此前折进的改写），等于当时的正文就不动（第 254 条，3.4）。`Folded` 仍只说"最终的正文里带着折进的改写"，不说"正文的差别只来自改写"：折进之后又改了正文、或折进之前就写过别的正文时，守卫照样答 `page.upgrade_batch_rewrite`，拆开之后正文本身的改动仍被拒为 `page.upgrade_immutable`；反过来，"移动之后写下新的 frontmatter 与改写之后的正文"答 `page.upgrade_immutable`，分两个 `batch` 才过。这几种形状保留粗的判据，由码的文字与 playbook 说清（第 265 条，3.4 的表）。**S1 之后**：这一条里"写的不等于这一页当时的正文就清掉"与"'移动之后写下新的 frontmatter 与改写之后的正文'答 `page.upgrade_immutable`，分两个 `batch` 才过"已由第 268 条收回：此前的移动或改名改写过一页基线页之后再写它，写什么都答 `page.upgrade_batch_rewrite`，升级中另照批前的链接判（第 270 条）；00 号文档 4.6 与第 190、254、265 条随之改（3.19）。
3. **闸门 2 的"不是活着的页"是缺陷**（4.7 闸门表 `source_root` 一行的"从哪里读" `:220`）：00 号文档写"不过时第 4–6 条与提示不算、记为不过"。提议的来源根是发起时写下的 `raw`（有保护行）或已有的 `raw`（基线节点），守卫不许删，种类也变不了；`upgrade_title_taken` 挡掉了附件（4.4 第 8 步）。它不是活页只能是缺陷，这时 lint 的显式范围本身答错误（`page/adapter/postgres/lint_metadata.go:63-65`），检查答 500、记一行 error，不另造一份没有规则的报告（契约的 `rules` 恒为八项、`stale_pages` 恒有值）。闸门 2 照旧判"活着的页，且不是根下的 `AGENTS`"，后一半是到得了的（B-Q2 的改名）（第 256 条，3.10）。
4. **检查的快照里只读 wiki 的行**（4.7"一个快照" `:209`"读笔记本的升级列与 wiki 的行"）：两边在同一个事务里写、`checkPages` 守着一致（P1/S2 的升级规则），升级的 id、状态与提交的 id 都在 wiki 的行里，多读笔记本的列只是第二份同样的东西。改为"读 wiki 的行（进行中的那一行）"（3.11）。
5. **检查只要第一遍的按页的数，第二遍另成一项**（4.7 闸门 6 `:224`、第 8 节 linking 一条 `:652`）：闸门 6 只比"现在在来源区以外"的基线页，这些页都在第一遍（`generalEmitter`）里；第二遍（发起时来源区里的基线页）只有发起要。今天 `WantedByPage` 只许与 `wanted` 一条规则同用、在 LLM 契约下连带第二遍（`linking/app/lint_rules.go:44-46`、`:120-122`），检查要的是"八条规则的摘要 + 第一遍的按页的数"。改为：`RuleQuery.WantedByPage` 可与摘要（`Rule == nil`）或 `wanted` 一条同用，只有第一遍；新的 `RuleQuery.AreaWantedByPage` 是第二遍，只与 `WantedByPage` 同用；发起两个都设（行为不变），检查只设前一个（3.8）。
6. **"新建的页"在 Go 里数**（4.7"报告" `:232`"SQL 用 `NOT EXISTS`"；第 9 节成本守卫表"检查"一行 `:699` 同）：检查本来就读全部基线行（闸门 6 要每页的 `wanted`，"移进来源区的"要每个节点的 `parent_id`）与 lint 的全部活节点，"没有基线行的活页"是两个集合在 Go 里相减，同一个快照，不另发一条反连接（3.10）。
7. **提交的第二段的期限是 `min(submitted_at + SubmitTimeout, 请求的期限)`，交付之前的判定在第二个事务里**（4.8 第 2 步 `:247`"`context.WithTimeout(context.WithoutCancel(ctx), LintTimeout + 10 s)`"、第 215 条 `:794`）：照那样写，期限从第一个事务提交之后才开始，比 `submitted_at + LintTimeout + 10 s` 晚一个提交的时长，"过了这一刻上一次提交脱离请求的一段一定已经结束"（4.8 第 1 步）并不成立（安全仍在：接管之后旧的第二个事务看到别的 `submission_id`，什么也不写）。另一面，`submitted_at + SubmitTimeout` 在默认配置下比 REST 的请求期限晚至少 2.5 秒（`LintTimeout = RequestTimeout/2`，`bootstrap/wire.go:246`），第二段若用它，到期时请求早已到期，平台答 500（审查 P2A-I1）。改为 `context.WithDeadline(context.WithoutCancel(ctx), min(submitted_at + SubmitTimeout, 请求的期限))`：第二段不随请求取消，但不晚于请求的期限；接管的安全只要求它在 `submitted_at + SubmitTimeout` 之前结束，收短不破坏它。`submitted_at` 是第一个事务读的那一刻（截到微秒，同库里存的值），`SubmitTimeout = LintTimeout + 10 s` 是 `UpgradeDeps` 的一项，接管的判断用同一个值；`detached` 到期就答 503 `server_busy`（REST 与 MCP 同码），客户端已断开时照平台的 `FaultGone`。"交付之前的判定"挪进第二个事务、在 `COMMIT` 之前读：它只决定答复，不挡存下；info 的日志在提交之后、答复之前记，与判定的结果无关。`COMMIT` 脱开取消、至多 `commitTimeout`（`platform/postgres/tx.go:199-203`），落在期限之后的提交由笔记本行的锁与 `submission_id` 的核对守着（3.12，第 258 条）。于是 4.16 第 7 条的"两个事务之间的另一次提交"只能落在前一次的期限之后：前一次的第二个事务到期、答 503，什么也不写（7.2 第 7 条）；`submission_id` 的核对由用例的单元测试钉住。
8. **`listUpgradeCheckItems` 的游标绑定在状态之后核对**（第 5 节 `:438`、`:446`"游标先解"，4.16 `:372`）：lint 的游标绑定显式的范围与根（`bootstrap/wiki_lint.go:212-234`），提议的根要读了 wiki 的行才知道，而那一读在判权限与规则名之后。游标的形状（解不开）照旧在最前答 400；"绑在别的范围、规则或根上"在读到 wiki 的行之后、`limit` 之前答 400 `bad_request`（同一个码，次序写进契约的说明）。绑定看根、不看升级：同一次升级的条目给出的游标都绑着同一个根；普通 `lint` 的游标（不是显式的范围）答 400；同一本笔记本前后两次升级提议同一个已有的 `raw` 时，上一次的游标照样被接受（范围相同，位置照样有效），两次的根不同（例如前一次补写的 `raw` 已随回滚删去）时答 400（第 257 条，3.11）。
9. **`UpgradeCheck.counts` 的两个 `wanted` 是对象**（第 5 节 `UpgradeCheck` `:453`"后两个是 `LintCount`"）：`LintCount`（`api/modules/wiki.yaml:294-301`）是一个整数，"是否封顶"在 lint 的摘要里是另一个成员 `count_capped`。改为 `wanted_before`、`wanted_now` 各是 `UpgradeWanted {count: LintCount, count_capped}`（3.13）。
10. **`Runs.Do` 的结果**（4.7"名额与期限" `:213`"泛化为'一次运行的结果'"）：写成 `RunResult{Lint domain.LintResult; Check *domain.UpgradeCheck}`，`RunOutput` 同样多一项 `Check`；lint 只用 `Lint`（3.8）。
11. **MCP 的两个工具的矩阵行另成一组**（4.14 `:343`"`TestThePermissionMatrixCoversEveryTool` 读到它们的行"）：覆盖的检查今天读 `mcpRows()`（`bootstrap/permission_matrix_mcp_test.go:65-67`、`:444-448`），而 `mcpRows()` 由 `TestThePermissionMatrixOverMCP`（`:559-646`，第三份）在 lab 的笔记本上跑，那里没有升级。照 P1 的 REST 的写法（`coveredMatrixRows()`，`bootstrap/permission_matrix_coverage_test.go:158`）：两个工具的行在 `upgradeMCPRows()`，由第八份（第 261 条）的 `TestUpgradeToolsMatrix` 在升级的变体上跑，覆盖的检查改读 `coveredMCPRows() = mcpRows() + upgradeMCPRows()` 的 `mcpRow`（3.15）。
12. **`submitUpgrade` 在矩阵里管理员的一格答 409 `wiki.upgrade_not_ready`**（4.14 的矩阵）：lab 的笔记本不是整理好的 LLM Wiki（没有根下的 `AGENTS`、页没有 `type`），以 SQL 设成 `upgrading` 之后提交过不了闸门。矩阵钉的是"谁能提交"，409 与 403、404 分得开；"通过并存下报告"由 `TestUpgradeSubmit` 与 UP5 盖住（第 260 条，3.15）。
13. **升级页的"检查"只在 `upgrading`；自己的提交在算时照实显示**（4.15"检查（P2/S3）" `:358`）：审阅中对所有人冻结，现算与存下的报告一样，审阅页读存下的那份（4.7"存与读"）；`reviewing` 时这一节换成存下的报告（P2/S3 只读、不带"更多"；"更多"与三个动作在 P3/S3）。"提交审阅…"经一个确认对话框（`default` 的语气，同决定 251 的理由：拒绝留在原处、焦点规则一致）（第 259 条，4.3）。提交的第一个事务一发 `mode`，页面就换成审阅中、报告一节写"正在核对…"，对话框留在原处等答复；不过时的再检查经 SWR 的键（`generation` 在进入的层级上），检查一节卸下又装上也恰好读一次（第 264 条，4.3）。
14. **提交的日志在第二个事务提交之后记，带结局**（4.18 `:417`）：加 `outcome`（`stored`、`stale`、`not_ready`、`failed`、`superseded`），在提交之后、答复之前记，与交付之前的判定的结果无关；检查与条目没有自己的用例日志，`Runs` 的"lint computed"/"lint stopped"一行带 `query_kind` 的新名字（`upgrade_check`、`upgrade_items`、`upgrade_submit`）与 `upgrade_scope`（升级或提交的 id，3.8、3.16）。
15. **`upgrade_submit` 的 `destructiveHint: false` 要改 `mcpserver`**（4.13 `:316`）：`mcpserver` 只在 `Annotations.Destructive` 为真时写 `destructiveHint`（`platform/httpserver/mcpserver/tool.go:150-156`），MCP 的规范里没写的读作真，照今天的写法 `upgrade_submit` 会被客户端当成毁坏性的工具。改为不是只读的工具都写这一项；今天不是只读的五个写工具都已是 `true`，`tools.json` 只多 `upgrade_submit` 的两处 `false`（3.14）。
16. **存下的报告不在用例里另量大小**（4.17 上限表"存下的报告"一行 `:403`"用例在写之前量"）：报告去掉游标之后最坏约 33 KB（3.10 算出），过 65,536 字节只能是缺陷；表的检查 `notebook_upgrades_report_check` 已是界，越界时写失败、答 500。用例再量一次是到不了的分支，变异杀不掉它（去掉之后由表的检查答同样的 500）。改为"表的检查；一个单元测试以最坏的形状钉住它在界之内"（第 263 条）。
17. **账本的写自己核对**（4.6"账本" `:195`、第 191 条 `:773`）：00 号文档的账本无条件写 `accepted_hash`。系统的链接改写（`Rewrite`）不经正文的比较，一页基线页被绕过单元改过之后，一次改写就把账本更新成改写之后的哈希，绊线被洗掉（审查 P2A-M2）。改为：升级中（`upgrading`）账本只在 `accepted_hash` 等于这一步写之前的哈希（单元在 `LockContent` 已有的 `current.Hash`，不多读）时更新，基线行在而不等时答缺陷的错误（500），单元整个回滚；回滚的单元（`rolling_back`）照旧无条件写。守卫不再自己核对账本（3.3、3.5）。
18. **升级中 `batch` 的盲写计进读的界**（4.17 上限表，加一行）：守卫为每个基线页的正文步读一次当前的正文。编辑与被改写够着的写的页，计划已整读、在 `MaxBatchReadBytes`（14 MiB）之内；盲写的页计划只读 head，不在界里，而一页的 frontmatter 不限大小，删掉一块大的 frontmatter 正文不变、护栏放过，100 个这样的盲写每页 5 MiB，本机多读约 1.6 秒（审查 P2A-M4 实测）。改为：升级中（计划读到的状态是 `upgrading`），`CheckReads` 把盲写的已有页的 head 的字节数也加进同一个和，过 `MaxBatchReadBytes` 答 `page.batch_too_large`；heads 计划里已有，零额外的读；单元在锁下看到的状态与计划时的不同就重新计划（第 266 条，3.3）。上限表加一行"升级中一次 `batch` 整读与盲写的已有页 | 合计至多 `MaxBatchReadBytes` | 计划的 `CheckReads`"。
19. **第 222 条的 `TestUpgradeToolsMatrix` 进第八份**（第 222 条 `:801`"进第七份"）：S2 开第八份 `^TestUpgrade(Check|Gates|Submit|Tools)`（第 261 条），它收下 `TestUpgradeToolsMatrix`；`TestUpgradePermissionMatrix` 留在第七份，`TestUpgradePromptMatrix`（P3/S2）照名字落在第七份。改为"`TestUpgradePermissionMatrix`、`TestUpgradePromptMatrix` 进第七份，`TestUpgradeToolsMatrix` 进第八份（M11/P2 第 261 条）"。
20. **`upgrade_verify` 的注解照只读工具的写法**（4.13 `:315`"`idempotentHint: true`"）：M9 起只读工具只设 `ReadOnly`，`tools.json` 的 14 个只读工具都是 `"idempotentHint": false`；MCP 的规范里这一项只对不是只读的工具有意义。改为"注解 `readOnlyHint: true`"，`tools.json` 写 `"idempotentHint": false`（3.14，审查 P2C-N2）。
21. **检查的成本守卫在 5 万页**（第 9 节成本守卫表"检查"一行 `:699`"10 万页"）：审查 B 实测，十万页的检查折到 EPYC 7763 约 2.5–3.0 秒，是 worker 3 秒的 84–100%（`not_kept` 并进 `UpgradePreserved` 之后、闸门 7 过与不过都只走一遍，修订核对里重量的数，3.9）。照负责人 2026-11-06 的话调量级、不再优化：守卫定在 5 万页，十万页进 M12 的性能移交（第 262 条，7.1）。

### 2.2 需要负责人确认

无。上面的二十一条是照代码改说法、位置或做法，不改负责人与主持者的决定；第 2 条细化第 190 条的判据（码更准，拒绝的范围不变；S1 的第 268 条之后拒绝的范围扩大了，3.19），第 3 条把一种只有缺陷才到得了的情形从"记为不过"改成"答 500"，第 7 条让第 215 条的期限不晚于请求，第 15 条让 00 号文档已定的注解真的写出来，第 17 条让第 191 条的绊线不被系统的改写洗掉，第 18 条让升级中 `batch` 的读有界（`page.batch_too_large` 是 `batch` 已有的码）。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

00 号文档没有定、本文取的做法。254–263 是初稿的十条（P2-1 到 P2-10），设计审查之后照次序给了正式编号；264–266 是审查之后新定的三条；267 是主持者在提交之前加的。接受本文的文档提交把它们写进 00 号文档第 11 节（`p2-apply-00.py`）；S1 的取舍 268–271 与第 267 条的提前在 3.19，S1 的文档提交写进 00 号文档第 11 节：

- **254　`Folded` 取这一页最后一个正文操作时的样子**（2.1 第 2 条，3.4）：它只修"折进的改写已被 agent 自己之后的写整个换掉"这一种，那时答 `page.upgrade_immutable` 而不是叫它去拆；拒绝的范围不变。其余的形状照第 265 条。S1 的第 268 条收回了写的路径上的清除，"拒绝的范围不变"也不再成立（3.19）。
- **255　守卫以一条读取"是不是基线页"与它现在的正文**（2.1 第 1 条，3.3）：`upgrade_nodes` 的主键连 `page_contents` 的主键，不是基线页就没有行、不读正文；不在发起时记下每个基线页的正文哈希（那要在发起的事务里读并解压整本笔记本的正文，发起的成本守卫已经贴着期限，第 232 条）。账本的核对不在守卫里，在账本的写里（2.1 第 17 条）。
- **256　提议的根不是活页是缺陷，检查答 500**（2.1 第 3 条，3.10）。
- **257　条目的游标绑定在状态之后答 400；绑定看根、不看升级**（2.1 第 8 条，3.11）。
- **258　提交的第二段的期限是 `min(submitted_at + SubmitTimeout, 请求的期限)`，交付之前的判定在第二个事务里、`COMMIT` 之前**（2.1 第 7 条，3.12）。
- **259　升级页的"检查"只在 `upgrading`；审阅中只读地显示存下的报告；提交经确认对话框**（2.1 第 13 条，4.3）。
- **260　矩阵里提交的管理员一格答 `wiki.upgrade_not_ready`**（2.1 第 12 条，3.15）。
- **261　S2 开第八份**（7.5）：P2 的新组合根测试都以 `TestUpgrade` 起名。不开时 S2 之后第七份在 EPYC 7763 上约 375–420 秒（审查 C 照 P1 的同类实测的估计，7.5），邻份没有余量（最坏的一次第一份 446、第二份 443、第三份 430、第四份 418、第五份 443 秒；第六份 354 秒，P1 刚因 460 秒挪出过 `TestIngestBefore`），P3 还要往这一份加批准、驳回、退回与 prompt 的测试。所以 S2 的第一个提交开第八份 `BOOTSTRAP_EIGHTH = ^TestUpgrade(Check|Gates|Submit|Tools)`（第七份 `-skip` 它；改动的每一处与新文字在 6.2 第 1 个提交，照 P1 设计 3.8 与 00 号文档第 186 条），不提高 `-timeout`，不挪别的份；第 222 条的 `TestUpgradeToolsMatrix` 随之进第八份（2.1 第 19 条）。S1 不开：它只给第七份加 `TestUpgradeGuardrails` 的格（设计时二十二格；S1 的审查之后二十七格，3.19、7.5）。
- **262　检查的成本守卫定在 5 万页，十万页进 M12 的性能移交**（2.1 第 21 条，7.1）：审查 B 实测十万页的检查另加的读本机约 0.31–0.34 秒（并进 `not_kept` 之后，含 PostgreSQL 16 起的预排序聚合的一次排序，3.9）、lint 摘要 0.785 秒，7763 与本机之比 2.2–2.6，折到 7763 约 2.5–3.0 秒（worker 3 秒的 84–100%）；5 万页约 1.3–1.5 秒。照负责人 2026-11-06 的话（`docs/v0.1/M10-llm-wiki/00-M10-design.md:907`）调量级而不再优化，同第 232 条的写法。
- **263　存下的报告的大小由表的检查守着，用例不另量**（2.1 第 16 条，3.10）：去掉游标之后最坏约 33 KB，过 65,536 字节只能是缺陷，表的检查拒绝、答 500；`wiki/adapter/postgres` 的 `TestTheWorstReportFits` 以最坏的形状经真库写进去，钉住它在界之内。
- **264　自己提交时，升级页照实显示审阅中**（审查 P2C-I1、P2C-Q1，4.3）：第一个事务发出 `mode` 之后，页面换成审阅中，报告一节写"正在核对…"；对话框留在原处，答复回来之前不动。理由：与服务端的真状态（`reviewing`、`report: null`）一致，最省，不必在 `UpgradeRead` 里压住重读。
- **265　`Folded` 保留粗的判据，把话说准**（审查 P2A-M3，3.4）：码的文字加 "…; the page's body itself may not change either"；playbook（P3/S2）写明拆开之后正文本身的改动仍被拒为 `immutable`；3.4 的表写出三种偏严的形状各自的码。理由：准确的做法要为每页带一份影子正文，代价与复杂度都高；粗的判据只在少数形状上给出偏严的码，文字与 playbook 能让 agent 读懂（负责人 2026-11-06：不要无限优化）。S1 的第 268 条之后，g21 那一种（移动之后写下新的 frontmatter 与改写之后的正文）改答 `page.upgrade_batch_rewrite`，码的文字随之改写（3.19）。
- **266　升级中 `batch` 的盲写按 head 的字节数计进读的界**（审查 P2A-M4，2.1 第 18 条，3.3）：计划读到的状态是 `upgrading` 时，`CheckReads` 把盲写的已有页的 head 也加进 `MaxBatchReadBytes` 的和，过了答 `page.batch_too_large`。计划不知道哪些页是基线页，已有页里不是基线的只有升级以来新建的页，一并计入只会让升级中的批略早被拒，agent 拆开即可；不多一次读。计划的状态不加锁读，单元在锁下看到的状态与它不同就重新计划，界没有例外（审查 P2A-N8）。
- **267　costs-2 的作业时限从 35 分钟提到 45 分钟**（主持者在提交之前加；原在 6.2 第 8 个提交，S1 的修复 `80da6b1f` 已改，提前到 S1，3.19、7.5）：`main` 的 CI 上 costs-2 在 EPYC 7763 上 22.2 分钟（run 42530568840）、23:17（run 42752100006，`f002672b`），在 EPYC 9V74 上 16:42（run 42749131694）。P2 往这一份加护栏的读（约 3 秒）与检查的守卫（估 40–70 秒，在 7763 上约 1.5 倍），作业在快机上约 18.5 分钟、在 7763 上约 25–26 分钟，35 分钟不到 `ci.yml` 注释里"约两倍"的规则。P1/S2 的审查 C 已经提过，当时说合并之后看 `main` 上的数。理由：负责人 2026-11-06 的话，调时限，不挪测试、不优化。

## 3. 服务端

### 3.1 S1 的文件

| 文件 | 改动 |
|---|---|
| `page/domain/errors.go` | 三个码（3.6） |
| 新的 `page/domain/upgrade_body.go` | `UpgradeBody`、`UpgradeBodyHash`（3.3） |
| `page/domain/batch_plan.go`、`batch_contents.go` | `plannedPage.reached`、`PlannedPage.Reached`、`MarkReached`（第 268、270 条）、`holdsAt` 交回改写是否改了这一页（3.4）；`CheckReads` 在升级中把盲写的已有页计进读的界（3.3，第 266 条） |
| `page/app/extension.go` | `Step.KeepLinks`、`StepContent.BodyHash`、`StepContent.BeforeHash`、`StepContent.Reached`（3.2–3.5） |
| `page/app/unit.go`、`unit_content.go` | `step()` 带 `keepLinks`；`WriterDeps.Markdown`、`WriterDeps.Ledger`；`ContentWrite.Reached`；`writeContent` 算 `BodyHash`，升级中带 `Reached` 的字节不变的写也过守卫，守卫的准备与判定从 `applyStep` 抽成 `Unit.guarded`；`applyStep` 写账本（3.2–3.5，第 268 条） |
| `page/app/apply_batch.go` | `content` 交 `Reached`、两个正文的码标在 `Last`；升级中 `reachedBefore` 照批前的链接问 `BatchLinks.Reached`（3.4，第 270 条）；`batchAttempt.upgrade` 记下计划读到的状态（3.3） |
| `page/app/freeze_guard.go` | `guardrails` 加移动与改名、正文；`UpgradeNodes.BaselineContent`；`Frontmatters`（3.2、3.3） |
| `page/app/ports.go` | `UpgradeLedger`（3.5）；`BatchLinks.Reached` 的注释（第 270 条） |
| `page/adapter/markdown/markdown.go` | 包级的 `FrontmatterEnd` 与 `Frontmatter`（3.3） |
| `page/adapter/postgres/queries/upgrade.sql`、`upgrade.go` | `UpgradeBaselineContent`、`AcceptUpgradeContent`（3.3、3.5） |
| `page/freeze_guard.go`、`page/module.go` | 模块根交 `Frontmatter`；`WriterDeps` 交 `md` 与 `store`（3.3、3.5） |
| `bootstrap/permission_matrix_mcp_test.go`、`upgrade_tool_codes_test.go`、`mcp_client_test.go`、`upgrade_guardrails_test.go`；新的 `bootstrap/upgrade_guardrails_cost_test.go` | 码的表、整个程序的测试、成本守卫（3.6、7.1） |
| `page/adapter/postgres/upgrade_cost_test.go` | 两条新读的计划（7.1） |
| `Makefile`、`.github/workflows/ci.yml` | 两个守卫的行与注释（7.1） |
| `e2e/stories/upgrade/up4-guardrails.spec.ts` | 补全（第 5 节） |

### 3.2 S1：不改写链接的移动与改名

`Step`（`page/app/extension.go:57-77`）在 `Rewrite` 旁加：

```go
	// KeepLinks is a batch's move or rename with update_links false
	// (Placement.KeepLinks): its unit's deferred rewrite follows neither
	// the node nor what it carries. A single move's is Options.UpdateLinks
	// false. M11's guardrails refuse both (M11 design 4.6).
	KeepLinks bool
```

`Unit.step`（`page/app/unit.go:366-368`）改为 `Step{Write: u.write, Operation: op, Changes: changes, KeepLinks: u.keepLinks}`：`u.keepLinks` 只在 `Relocate` 里为批的移动设上（`unit_relocate.go:40-41`），`Relocate` 造的那一步（`renameTo` 的 `OpRename`，`unit_rename.go:54`；`moveSubtree` 的 `OpMove`，`unit_move.go:84`）由此带上它，别的步恒为假。

护栏（`page/app/freeze_guard.go:77`）：

```go
// guardrails are the initiator's agent's limits (M11 design 4.6): a
// delete of a node the upgrade started with or of its seeded source root
// (P1); a move or a rename that leaves the links to it as they are; a
// write of a page of the baseline whose body, past its frontmatter, is not
// what it was before the step, but for the system's rewrite of its links.
func (g *FreezeGuard) guardrails(ctx context.Context, s Step) error {
	switch {
	case s.Operation == domain.OpDelete:
		return g.deletion(ctx, s) // P1's, as it is
	case s.Operation == domain.OpMove || s.Operation == domain.OpRename:
		if !s.Options.UpdateLinks || s.KeepLinks {
			return domain.Refuse(domain.ErrUpgradeKeepLinks, s.Changes[0].NodeID)
		}
	case s.Operation == domain.OpContent && !s.Rewrite:
		return g.body(ctx, s)
	}
	return nil
}
```

- 单个的 `move`：`relocate_path.go:85-86` 把 `update_links` 放进 `Options.UpdateLinks`；`update_links: false` 时这一步答 `page.upgrade_keep_links`，经工具的错误答出（单元回滚，什么也没写）。改名是同一个工具的同一个父节点（`relocate_path.go:77-80` 的 `ActionRename`），一样答它。
- `batch`：单元整体 `UpdateLinks: true`（`apply_batch.go:254-256`），每个 `update_links: false` 的移动经 `Placement.KeepLinks`（`:770`）设 `u.keepLinks`，这一步带 `KeepLinks`，拒绝经 `a.failed(i, err)`（`:621-634`）标在那个移动的操作上（`OpFailed.Op`，工具的错误的 `op`），整个 `batch` 回滚。
- 参与者推后的改写只给参与者一步（`unit.go:584-606`，不经守卫），它们写正文时 `Rewrite = true`（`unit.go:769-773`），由下一节放行。
- 被标的节点是这一步的根（`s.Changes[0]`）；同 P1 的决定 237，它只在 page 之内看得到。
- REST 的移动与改名在冻结里就被拒（P1），到不了这里。

### 3.3 S1：正文的切分与比较

**切分**（`page/domain/upgrade_body.go`，新）：

```go
// UpgradeBody is a page's body as an upgrade's guardrails compare it (M11
// design 4.6, decision 189): content past its frontmatter, which ends at
// end (Markdown.FrontmatterEnd: 0 without one, an invalid block with its
// closing line being one), and past a byte order mark there.
func UpgradeBody(content string, end int) string { return strings.TrimPrefix(content[end:], bom) }

// UpgradeBodyHash is UpgradeBody's SHA-256.
func UpgradeBodyHash(content string, end int) []byte {
	sum := sha256.Sum256([]byte(UpgradeBody(content, end)))
	return sum[:]
}
```

`bom` 是 page/domain 已有的常量（`AgentsBody` 用它，`page/domain/instructions.go:35`）。开头的 BOM 在 frontmatter 之前时 `FrontmatterEnd` 已跳过它（`frontmatter.go:70-75`），没有 frontmatter 时 `end` 是 0、去掉的就是开头的 BOM，有 frontmatter 时去掉的是紧跟收尾行的那一个（第 189 条）。

**适配器**（`page/adapter/markdown/markdown.go`）：

```go
// FrontmatterEnd is where content's frontmatter ends, past its closing
// line, as the parse reads it; 0 without one. It needs none of the parse's
// settings: the freeze guard, built from the pool, has it too.
func FrontmatterEnd(content string) int { return markdown.FrontmatterEnd([]byte(content)) }

// Frontmatter is FrontmatterEnd as a port (app.Frontmatters).
type Frontmatter struct{}

func (Frontmatter) FrontmatterEnd(content string) int { return FrontmatterEnd(content) }
```

`(*Markdown).FrontmatterEnd`（`:52-54`）改为调它，行为不变。

**之后的那份**：`StepContent`（`page/app/extension.go:81-85`）加

```go
	// BodyHash is UpgradeBodyHash of the content written, while the
	// notebook is upgrading, but for a system's rewrite (Rewrite), which the
	// guardrails let by unread; nil otherwise (M11 design 4.6).
	BodyHash []byte
	// BeforeHash is the hash of the content the step writes over, as the
	// unit locked it (ContentLock.Hash), in a notebook being upgraded: the
	// ledger takes the step's only over it (M11/P2 design 3.5).
	BeforeHash []byte
	// Reached says a rename or move earlier in the step's batch rewrote
	// this page's links (ContentWrite.Reached, PlannedPage.Reached).
	Reached bool
```

`WriterDeps`（`page/app/unit.go:25-58`）加 `Markdown Frontmatters`（新的窄端口，`app.Markdown` 已含它，`page/app/ports.go:786-788`；模块根交 `md`，`page/module.go:302-304`）。`writeContent`（`unit_content.go:50-107`）在 `contentStep`（`:111-122`）里：`u.write.Notebook.Upgrade.State == domain.UpgradeUpgrading && !w.Rewrite` 时设 `BodyHash: domain.UpgradeBodyHash(w.Content, u.w.d.Markdown.FrontmatterEnd(w.Content))`（参与者的改写守卫不比，不为它算一次全文的哈希：单个 `move` 的改写页数不设界，`batch_plan.go:41`）；笔记本有升级时设 `BeforeHash: current.Hash`（`LockContent` 已交回，`:51`，不多读）；`Reached: w.Reached`。字节不变的写在 `:66-78` 已返回，不算 `BodyHash`；例外见下一段。会被冻结拒掉的写（别人的）在守卫之前分不出来，照算，不管。

**字节不变、被到达的写**（第 268 条；实施者在 S1 的修复一轮加，三位审查者都判它必要、不误拒）：写下的字节与库里相同时 `writeContent` 在第一个分支就返回，不造正文步，守卫看不到。一个 `batch` 改名之后写回一页基线页原来的字节（g23 的后一种），或把折进的 `[[Map]]` 编辑回 `[[A]]`（g25），推后的改写又只跟这一页最后一个正文操作之后的移动，链接就留在旧的名字上。所以升级中（`upgrading`）带 `Reached` 的这种写照一次写去过单元的守卫（`unit_content.go:80-89`）：`contentStep` 照真的写造步（`Change.Revision` 是 `current+1`，`BodyHash`、`BeforeHash`、`Reached` 都有），`Unit.guarded`（从 `applyStep` 抽出：来源行 → 来源哈希 → 守卫 → `refusalOf`，`unit.go:432-449`，`applyStep` 与它共用）判它，什么也不写、不写账本，返回原来的版本。基线页答 `page.upgrade_batch_rewrite`；不是基线页的，冻结放过（读一次基线行），之后模式与锁两个守卫照一次真的写判：升级中笔记本必定是 `standard`，模式守卫立即返回；升级中开不了编辑会话，锁守卫没有可拒的。没有升级、`reviewing`、`rolling_back` 与不带标记的写照旧不过守卫。

**之前的那份与比较**（`page/app/freeze_guard.go`）：

```go
// Frontmatters tell where a page's frontmatter ends (Markdown.FrontmatterEnd).
type Frontmatters interface{ FrontmatterEnd(content string) int }

// UpgradeNodes are an upgrade's rows the freeze reads, in the transaction
// ctx carries.
type UpgradeNodes interface {
	Protected(ctx context.Context, upgradeID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error)
	// BaselineContent is the content now of the page id when it is of the
	// upgrade's baseline, false when it is not (M11 design 4.6): read under
	// the page's content row lock, which the unit holds as it writes it.
	BaselineContent(ctx context.Context, upgradeID, id uuid.UUID) (string, bool, error)
}

func NewFreezeGuard(auth shared.Authorizer, admin shared.Action, nodes UpgradeNodes, md Frontmatters) *FreezeGuard

// body refuses a write of a page of the baseline, marked with the page:
// page.upgrade_batch_rewrite when a rename or move earlier in its batch
// rewrote its links (StepContent.Reached), whatever the write (M11/P2
// decision 268); page.upgrade_immutable when its body is not the one
// before the step (M11 design 4.6). It compares the body of the content
// the page holds now, under the lock of its content row the unit took,
// with the step's, which the unit hashed (StepContent.BodyHash). A page
// not of the baseline it lets by, reading no content; a step without the
// body's hash is a defect's error.
func (g *FreezeGuard) body(ctx context.Context, s Step) error {
	id := s.Changes[0].NodeID
	if s.Content == nil || s.Content.BodyHash == nil {
		return fmt.Errorf("page: a content step of page %s while upgrading carries no body hash", id)
	}
	before, baseline, err := g.nodes.BaselineContent(ctx, s.Notebook.Upgrade.ID, id)
	if err != nil || !baseline {
		return err
	}
	if s.Content.Reached {
		return domain.Refuse(domain.ErrUpgradeBatchRewrite, id)
	}
	if bytes.Equal(domain.UpgradeBodyHash(before, g.md.FrontmatterEnd(before)), s.Content.BodyHash) {
		return nil
	}
	return domain.Refuse(domain.ErrUpgradeImmutable, id)
}
```

- 账本的核对不在守卫里，在账本的写里（3.5，2.1 第 17 条）：守卫只管它比的那些步，系统的改写（`Rewrite`）它不读；账本的写在每个正文步之后、含改写，在那里核对才每一步都核到。初稿放在守卫里，一次绕过之后的改写照样洗掉绊线（审查 P2A-M2）。
- 比的是"这一步之前的正文"与"这一步之后的正文"，不是基线的正文：第一次系统改写之后基线就不再是它（00 号文档 4.6）。"之前"是单元在 `LockContent`（`unit_content.go:51`）之后、`apply` 之前的那一份，守卫在同一个事务里读它，没有别的写插得进来。
- 只对基线页读：新建的页（agent 的、发起补写的 `raw`/`AGENTS`）没有基线行，`BaselineContent` 交回 `false`，不读正文。补写的 `raw` 的保护行是 `baseline = false`，同样不算（它的正文本来就可以改，3.3 的 SQL 只认 `baseline AND kind = 'page'`）。
- `Rewrite` 的步不比（参与者的链接改写与撤销写回的改写，`unit.go:769-773`）：系统的改写放行（第 7 条闸门"差异只来自系统的链接改写"）。
- 模块根（`page/freeze_guard.go:22-24`）改为 `app.NewFreezeGuard(auth, admin, postgresadapter.New(pool), markdownadapter.Frontmatter{})`，`NewFreezeGuard(pool, auth, admin)` 的签名不变，13.1 第 21 条的例外照旧。

**读**（`page/adapter/postgres/queries/upgrade.sql`）：

```sql
-- name: UpgradeBaselineContent :one
-- The content now of the page node_id when it is of the upgrade upgrade_id's baseline (M11 design 4.6): an upgrade's
-- guardrail compares its body with a write's, in the unit's transaction, under the page's content row lock the unit
-- holds. By upgrade_nodes_pkey, then page_contents_pkey; no row for a page not of the baseline, whose content is not
-- read. A seeded source root's protection row is not of the baseline.
SELECT c.content FROM upgrade_nodes b
JOIN LATERAL (
    SELECT c.content FROM page_contents c WHERE c.node_id = b.node_id AND c.deleted_at IS NULL LIMIT 1
) c ON true
WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.node_id = sqlc.arg(node_id) AND b.baseline AND b.kind = 'page';
```

`LATERAL … LIMIT 1` 照 `CaptureBaseline`（`upgrade.sql:12-32`）的写法，让 `page_contents` 由外行的键逐个读、不被拉成连接（决定 240）。适配器（`page/adapter/postgres/upgrade.go`）：没有行交回 `("", false, nil)`。

**读的上界**（第 266 条；审查 P2A-M4 在本机 arm64 实测，每 MiB 约 3.1 毫秒：按主键再读一次 lz4 的正文、守卫的切分与 SHA-256、单元的 SHA-256；数据在页缓存里，冷读更慢）：

- 单个的 `write`、`edit`：守卫读一页，至多 `MaxContentBytes`（5 MiB，`page/domain/content.go:12`），本机约 16 毫秒。
- 单个的 `move`、改名：参与者的改写是 `Rewrite`，守卫不读、单元不算 `BodyHash`（上一段）；每页只多一条账本的写（3.5）。
- `batch`：守卫为每个写了正文的基线页读一次。编辑的页与被移动的改写够着的写的页，计划已整读，`CheckReads`（`batch_plan.go:884-905`）已把它们按 head 的字节数计进 `MaxBatchReadBytes`（14 MiB，`:19-27`）；盲写的页计划只读 head（`Reads`，`:847-862`），不在这个和里。而 `FrontmatterEnd` 不限 frontmatter 的大小（`frontmatter.go:70-96`）：一页以 `---` 起、很后面又有一行 `---`，前面一大块都是 frontmatter，agent 写一份很小的内容删掉它，正文不变、护栏放过。100 个这样的盲写各 5 MiB，本机 1.58 秒（A 实测），折到 7763 约 2–3.7 秒，单元持着笔记本行。
- 所以升级中盲写也计进同一个和（第 266 条）：

  ```go
  // CheckReads … While the notebook is being upgraded, the freeze guard reads
  // whole the page each content op of the initiator's agent writes (M11
  // design 4.6): its blind writes of pages there count too (M11/P2 decision
  // 266), the pages it created none.
  func (p *BatchPlan) CheckReads(heads map[uuid.UUID]PageHead) *OpFailed {
  	if p.final {
  		return nil
  	}
  	texts, blind := p.Reads()
  	if p.notebook.Upgrade.State != UpgradeUpgrading {
  		blind = nil
  	}
  	sum, whole := 0, false
  	for _, id := range append(texts, blind...) {
  		first := p.pages[id].first
  		whole = whole || p.steps[first].Kind != OpKindEdit
  		if sum += heads[id].Bytes; sum > MaxBatchReadBytes {
  			// … as it is; a blind write's hint: "while the notebook is being
  			// upgraded, each page a write replaces is read whole: split the batch"
  		}
  	}
  	return nil
  }
  ```

  `heads` 里已有盲写的页（`apply_batch.go:410-421` 一并读了 `texts` 与 `heads` 的状态），不多一次读。计划不知道哪些页是基线页；已有页里不是基线的只有升级以来新建的页，一并计入只让升级中的批略早被拒，agent 拆开即可。单元在笔记本行下时这些页的版本与计划读到的相同（不同就是 `revision_mismatch`、重新计划），所以一个单元里守卫至多读 14 MiB，本机约 45 毫秒，被拒的那一页也在这个和里。计划读到的状态不是 `upgrading` 时不计：别人的写在冻结里整个被拒，审阅中与回滚中都没有发起人的写。计划的状态是不加锁读的（`planned`，`apply_batch.go:277`），计划之后、单元之前可能有人发起：单元在锁下看到的 `Write.Notebook.Upgrade.State` 与计划时的不同，就 `differs(0, "state", nil, nil)` 重新计划（`apply_batch.go` 的 `apply` 开头，`:560`；`planned` 把读到的状态存进 `batchAttempt`），第二次计划读到 `upgrading`、照上面计数。所以"一个单元里守卫至多读 14 MiB"没有例外（审查 P2A-N8）。码的次序同计划已有的界：`page.batch_too_large` 是计划的拒绝，在单元的冻结之前（P1 起别人的 `batch` 同样先答计划的界）。

**测试**：`page/domain` 的 `TestTheUpgradeBodyShapes`（形状表，每种一个反例，05-lessons"一个反例只钉住一种形状"）以真的 `FrontmatterEnd` 切（放在 `page/adapter/markdown` 的测试里，那里能用 platform 的切分）：

| 形状 | 之前 | 之后 | 同一个 `body`？ |
|---|---|---|---|
| 没有 frontmatter，加一块有效的 | `Text\n` | `---\ntype: note\n---\nText\n` | 是 |
| 开头的 BOM，没有 frontmatter，加一块 | `﻿Text` | `---\ntype: note\n---\nText` | 是 |
| frontmatter 之后的 BOM | `---\na: 1\n---\nText` | `---\na: 2\n---\n﻿Text` | 是 |
| 两个 BOM 只去一个 | `Text` | `---\na: 1\n---\n﻿﻿Text` | 否 |
| `\r\n` 的界线 | `---\r\na: 1\r\n---\r\nText` | `---\r\na: 2\r\n---\r\nText` | 是 |
| `\r\n` 的 frontmatter 改成 `\n` | `---\r\na: 1\r\n---\r\nText` | `---\na: 1\n---\nText` | 是 |
| 有效的，改正文一字 | `---\na: 1\n---\nText` | `---\na: 1\n---\nTexT` | 否 |
| 无效的、带收尾行，就地改成有效的 | `---\na: [\n---\nText` | `---\nraw: "a: ["\n---\nText` | 是 |
| 无效的、带收尾行，前面另加一块 | `---\na: [\n---\nText` | `---\ntype: x\n---\n---\na: [\n---\nText` | 否 |
| 只有开头一行（没有收尾行），前面加一块 | `---\nText` | `---\ntype: x\n---\n---\nText` | 是（旧的整页成了正文，同 4.6"核实"的第一种形状） |
| 只有开头一行，把它当 frontmatter 补上收尾行 | `---\nText` | `---\nText\n---\n` | 否（`Text` 成了 frontmatter，正文由 `---\nText` 变成空） |
| 空的正文 | `---\na: 1\n---\n` | `---\na: 2\n---\n` | 是 |
| 空的正文写进一字 | `---\na: 1\n---\n` | `---\na: 1\n---\nx` | 否 |

`page/app` 的 `TestTheFreezeGuard` 加：基线页的正文变了答 `page.upgrade_immutable`；带 `Reached` 时不论正文都答 `page.upgrade_batch_rewrite`（只改 frontmatter、"什么也没改"也答），都标在那一页；不是基线页的带不带 `Reached` 都放过；只改 frontmatter 放过；`Rewrite` 的步放过、不读；不是基线页放过、只读一次 `BaselineContent`、不读正文以外的东西；`BodyHash` 为空答缺陷的错误；移动与改名的两种不改写链接（`Options.UpdateLinks` 为假；`KeepLinks`）各答 `page.upgrade_keep_links`、标在这一步的根，改写链接的移动与改名放过、什么也不读。`page/app` 的 `TestTheUnitHashesTheBodyWhileUpgrading`：`upgrading` 时 `StepContent.BodyHash` 是 `UpgradeBodyHash`、`BeforeHash` 是锁到的哈希；`upgrading` 中 `Rewrite` 的步、`reviewing`、`rolling_back` 与没有升级时 `BodyHash` 为空；没有升级时 `BeforeHash` 也为空。`page/app` 的 `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds`（第 268 条）：升级中带 `Reached` 的字节不变的写，守卫看到一个带 `Reached`、带 `BodyHash` 的正文步，守卫放过时照常做完、拒绝时答它、什么也没写；不带标记、或没有升级，守卫什么也看不到。`page/adapter/postgres` 的 `TestUpgradeBaselineContent`（基线页答正文；新建的页、保护行、附件的基线行、已删的页都答 `false`）。`page/domain` 的 `TestTheReadsBoundCountsBlindWritesWhileUpgrading`：同一个批（盲写的已有页 head 合计过 14 MiB，编辑的页在界内）在 `upgrading` 时答 `page.batch_too_large`、标在跨过界的那一页的第一个正文操作，在没有升级、`reviewing` 与 `rolling_back` 时过；升级中新建的页（批里新建的）不计。`page/app` 的 `TestTheBatchPlansAgainOnceTheUpgradeStarted`：计划时假的笔记本答没有升级，单元的 `Write.Notebook` 是 `upgrading`（计划之后、单元之前发起），单元重新计划一次，第二次计划把盲写计进和：过界时答 `page.batch_too_large`、什么也没写，在界内时照写。

### 3.4 S1：`batch` 的折进

**计划**（`page/domain`）：`plannedPage`（`batch_plan.go:279-290`）加 `reached bool`，`PlannedPage`（`:265-274`）加

```go
	// Reached says a rename or a move earlier in the batch rewrote the
	// page's links before one of its content ops: an edit's after a move
	// whose rewrite changed the page, a write's after one whose rewrite
	// would have, whatever the write's bytes; and, while upgrading, a move
	// before its last content op that reaches a link the page had before
	// the batch, whatever the page holds in the batch then (MarkReached,
	// M11/P2 decision 270: a link taken out and put back follows the move
	// no more than one kept). It sticks: a later write does not clear it.
	// While upgrading, the guardrails answer
	// page.upgrade_batch_rewrite for a page of the baseline so reached,
	// whatever its content (M11/P2 decision 268, which withdrew decision
	// 254's clearing): the batch's deferred rewrite follows only the moves
	// after the page's last content op, so a write after the rename leaves
	// the page's links on the old name, its bytes those the page holds,
	// which no comparison of its body tells. Not ps.Moved: that is any move
	// between two content ops, even one that rewrote nothing.
	Reached bool
```

由 `:986` 照抄。`holdsAt`（`batch_contents.go:183-197`）多交回一项 `rewrote bool`：有移动、改写的结果不等于改写之前的内容。`content`（`:65-`）里：

- 写的路径（`:78-89`）：`same, size, rewrote, err := p.holdsAt(…)`；之后 `if rewrote { pg.reached = true }`，不论写的字节，之后不清（第 268 条：写恰是改写的结果、写别的、写回这一页原来的字节，都可能让链接留在旧的名字上；初稿的 `case !same: pg.folded = false` 已删）。
- 编辑的路径（`:90-108`）：`rewritten != content` 时 `pg.reached = true`（编辑在改写之后的正文上替换，其余留着），只设不清；没有移动或改写什么也没改时不动。
- 新建的页（`:76-77`）不设：它们没有基线行，护栏不比。
- 批前的链接（第 270 条，修复核对 P2S1A-M1）：一个 `batch` 先把 P 的 `[[A]]` 编辑成 `(A)`、再改名 A、再编辑回 `[[A]]`，改名的那一刻 P 在批里的内容不链向 A，正文的一遍设不上标记，最终的字节与库里相同，链接留在旧名字上。所以计划读到的状态是 `upgrading` 时，批的读（`apply_batch.go` 的 `read`）在 `CheckReads` 之后调 `reachedBefore`：按各页最后一个正文操作的次序，对本批不新建、最后一个正文操作之前有移动的页，以现有的 `BatchLinks.Reached` 按批前的索引问一次那些移动够不够得着它（移动、冻结的节点、移动之后新建的节点与树都取到 `pg.Last`，树的读用 `readsAt(pg.Last)` 的缓存）；写的页第一个正文操作就是最后一个、头已经问过时用头的答案，不再读。够得着就 `plan.MarkReached(id)`，在正文的一遍之前设上，之后照第 268 条粘住；新建的页不设。一页至多两次索引的读，整批不过 `MaxBatchOps` 减去移动数；普通笔记本、`reviewing`、`rolling_back` 一个操作也不多读。索引不是当前修订或当前抽取器时 `Reached` 答否，与批的改写看的是同一份索引（M6 的跳过），护栏不比改写更松、也不更严。新的读就是 `BatchLinks.Reached`，它的计划由 costs-2 的 `TestLinksReachedReadPlans`、`TestLinksReachedDenseReadPlans` 守着；单元守卫的五种形状里没有"移动之后写同一页"，量不到它，界不动。代价（第 271 条）：M9/P3 已接受、交给 M12 的"许多次改名之间的带链接的页"在升级中的计划约翻倍，本机 7.5 s 对 3.5–3.8 s，调用 10.1 s，照 7763 的比估约 18.6 s，过 15 秒的请求期限；过期限时什么也不写，拆开就能过（3.19，第 9 节）。正文的一遍在升级中仍有用：被够着的链接是本批加的、批前的索引里没有（只能在 frontmatter 里，g27 的 Q）。

**单元**：`ContentWrite`（`unit_content.go:17-25`）加 `Reached bool // internal: a batch's page a rename or move earlier in it rewrote (PlannedPage.Reached)`，随 `contentStep` 进 `StepContent.Reached`（3.3）。`apply_batch.go` 的 `content`（`:706-739`）：

```go
	revision, err := u.WriteContent(ctx, ContentWrite{NodeID: at.Node.ID, Content: pg.Content, Facts: a.facts[pg.ID],
		Base: lock.Revision, Merged: &pg.Merged, Reached: pg.Reached})
	switch {
	case errors.Is(err, domain.ErrRevisionMismatch):
		// … as it is
	case errors.Is(err, domain.ErrUpgradeImmutable), errors.Is(err, domain.ErrUpgradeBatchRewrite):
		// The unit writes at the page's first content op what its last decided
		// (M11 design 4.6): a refusal of its body is the last's.
		return a.failed(pg.Last, err)
	case err != nil:
		return a.failed(i, err)
	}
```

`errors.Is` 认得包在 `Refused` 里的码（`shared.Error.Is` 比较种类与码，`shared/error.go:215-218`；`Refused` 有 `Unwrap`）。`page.upgrade_keep_links` 由 `move`（`:744-776`）标在那个移动上，`page.upgrade_protected` 由 `delete` 标在那个删除上（P1），都是默认的 `a.failed(i, err)`。

**粗的判据**（第 265 条；第 268 条之后更粗）：`Reached` 只说"此前的移动或改名改写过这一页"，不看写下的正文。守卫对带标记的基线页一律答 `page.upgrade_batch_rewrite`，于是有几种形状的码偏严：折进之后又改了正文、折进之前就写过别的正文（g19、g20），这两种拆开之后正文本身的改动仍被拒为 `page.upgrade_immutable`；"移动之后写下新的 frontmatter 与改写之后的正文"（g21）分两个 `batch`（先移动，系统以 `Rewrite` 改写、账本跟上；再写）就过。初稿的"写的不是当时的正文，就换掉了此前的改写，答 `page.upgrade_immutable`"不成立：推后的改写只跟这一页最后一个正文操作之后的移动，改名之后的写不论写了什么，都可能让链接留在旧的名字上（g23 断链、g24 指向别处、g25 编辑改回），而字节可能与库里相同，比正文看不出（审查 P2S1A-I1，第 268 条）。准确的做法要为每页带一份不折进改写的影子正文、编辑在影子上找不到原文时记为依赖改写，再把影子的 `body` 的哈希交给守卫；代价与复杂度都高，而拆开的写法对 agent 总是可行。所以保留粗的判据，`page.upgrade_batch_rewrite` 的文字写明拆法、并说"正文本身也不能改"（3.6），playbook（P3/S2）写明拆开之后正文的改动仍被拒（第 9 节）。

**为什么不是"改了又改回"**：同一页在一个 `batch` 里改了又改回，单元只写最终的正文（M10 的 `Merged`），护栏只比最终的，放过（第 208 条），与分开调用不同；中间有移动改写过它（g25，第 268 条）、或中间的移动照它批前的链接够着它（g27，第 270 条）时不放过；playbook 与 5.8 写明（P3/S2）。

**测试**：`page/domain` 的 `TestThePlanTellsAPageAMoveReached`（原名 `TestThePlanTellsAFoldedRewrite`；表，每种一个反例）：

| 形状 | `Reached` | 这一页是基线页时守卫答的码（编辑与写只改 frontmatter，除非另写） |
|---|---|---|
| 移动 A（这一页链向 A），再编辑这一页 | 真 | `page.upgrade_batch_rewrite`（g9） |
| 无关的移动（这一页不链向它），再编辑这一页 | 假（`ps.Moved` 为真） | 放过（编辑改正文时 `page.upgrade_immutable`，g11） |
| 链向 A 的这一页索引过时、改写什么也没改，再编辑 | 假 | 放过 |
| 移动 A，再写这一页、内容恰是改写的结果 | 真 | `page.upgrade_batch_rewrite` |
| 移动 A，再写这一页、内容是别的字节 | 真（第 268 条） | `page.upgrade_batch_rewrite`（g23 的前一种，g24） |
| 移动 A，再写这一页原来的字节 | 真（第 268 条） | `page.upgrade_batch_rewrite`：字节不变，单元照一次写过守卫（g23 的后一种） |
| 编辑（被到达），再写别的内容 | 真（第 268 条，不清） | `page.upgrade_batch_rewrite` |
| 编辑（被到达），再写同样的内容 | 真 | `page.upgrade_batch_rewrite` |
| 编辑（被到达），再编辑一处无关的字 | 真 | `page.upgrade_batch_rewrite` |
| 编辑（被到达），一次这一页不链向的移动，再编辑 | 真（`Moved` 是 op 1 与 op 3） | `page.upgrade_batch_rewrite`（审查 P2S1C-M1） |
| 先编辑、后移动 A（改写由推后的参与者写） | 假 | 放过（改写以 `Rewrite` 写；移动在最后一个正文操作之后，批前的链接也不问） |
| 移动 A，再编辑这一页的正文一处（折进之后又改了正文） | 真 | `page.upgrade_batch_rewrite`；拆开之后那个编辑单独被拒为 `page.upgrade_immutable`（g19，第 265 条） |
| 写这一页（正文变了、写下的内容链向 A 或 B），移动 A、移动 B，再编辑这一页 | 真（编辑处 `MovesBetween` 非空、改写改了内容） | `page.upgrade_batch_rewrite`；拆开之后那个写被拒为 `page.upgrade_immutable`（g20，第 265 条） |
| 移动 A，再写这一页 = 新的 frontmatter 与改写之后的正文 | 真（第 268 条） | `page.upgrade_batch_rewrite`；分两个 `batch` 过（g21） |
| 编辑这一页的正文，一次这一页不链向的移动，再写同样的字节 | 假（`Moved` 是那个写） | `page.upgrade_immutable`（正文变了；实施的变异 o9 补的一行） |
| 新建的页（`MarkReached` 也不标） | 假 | 放过（不是基线页） |
| 正文的一遍之前已标上（`MarkReached`，第 270 条），之后拿掉链接、一次无关的移动、放回；或写别的字节、一次无关的移动、写回 | 真（粘住） | `page.upgrade_batch_rewrite`（g27） |

`page/domain` 的表断言 `Reached`；第三列的码由 `TestUpgradeGuardrails` 的 g9、g11、g19–g21、g23–g25、g27 经整个程序断言（3.6）。`page/app` 的 `TestTheBatchMarksABodysRefusalAtTheLast`（替身的守卫在带 `Reached` 的写上答两个码之一）：失败项的 `Op` 是这一页的 `Last`，别的码仍标在 `First`。`page/app` 的 `TestTheBatchJudgesAPageByItsLinksBeforeIt`（第 270 条，十格；替身照冻结的守卫拒带标记的正文步）：A 的两种形状（编辑拿掉→改名→编辑放回；写不带链接的字节→改名→写原来的字节）在升级中、索引说够得着时答 `page.upgrade_batch_rewrite`、标在 P 的最后一个操作、P 没被写、问一次，够不着时做完；头的页只写一次时用头的答案；头的页有后来的正文操作时问两次（`askedFirst` 两格）；普通笔记本的格做完、只有头的问（数读）。字节不变、被到达的写由 `page/app` 的 `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds` 钉住（3.3）。

### 3.5 S1：账本

端口（`page/app/ports.go`）：

```go
// UpgradeLedger keeps the hash of each baseline page's content that the
// guardrails last let by (M11 design 4.6, decision 191), in the unit's
// transaction: none for a page not of the baseline. With before, the hash
// of the content the step writes over, it takes after only over the hash
// it accepted last, and is a defect's error for a baseline page whose
// accepted hash is another: a content written by the units (M11/P2 design
// 3.5). Nil before takes it anyway, a rollback's.
type UpgradeLedger interface {
	AcceptUpgradeContent(ctx context.Context, upgradeID, nodeID uuid.UUID, before, after []byte) error
}
```

`WriterDeps.Ledger UpgradeLedger`（模块根交 `store`，`page/module.go:304`）。`applyStep`（`unit.go:381-428`）在 `writeSourceHashes` 之后：

```go
	if up := u.write.Notebook.Upgrade; s.Operation == domain.OpContent && up.ID != (uuid.UUID{}) {
		var before []byte // a rollback's unit writes it anyway
		if up.State == domain.UpgradeUpgrading {
			before = s.Content.BeforeHash
		}
		if err := u.w.d.Ledger.AcceptUpgradeContent(ctx, up.ID, s.Changes[0].NodeID, before, s.Content.Hash); err != nil {
			return err
		}
	}
```

```sql
-- name: AcceptUpgradeContent :one
-- The ledger (M11 design 4.6, decision 191): a content written in a notebook being upgraded, of a page of the
-- upgrade's baseline, is the one its guardrails accepted. With before (upgrading) only over the hash accepted last,
-- which the content written over must have (M11/P2 design 3.5): a baseline row whose hash is another counts
-- baseline 1, accepted 0, its caller's defect. No row for a page not of the baseline. In the unit's transaction, by
-- upgrade_nodes_pkey; b reads the row as the statement started, u writes it.
WITH b AS (
    SELECT 1 FROM upgrade_nodes
    WHERE upgrade_id = sqlc.arg(upgrade_id) AND node_id = sqlc.arg(node_id) AND baseline AND kind = 'page'
), u AS (
    UPDATE upgrade_nodes SET accepted_hash = sqlc.arg(after)
    WHERE upgrade_id = sqlc.arg(upgrade_id) AND node_id = sqlc.arg(node_id) AND baseline AND kind = 'page'
        AND (sqlc.narg(before)::bytea IS NULL OR accepted_hash = sqlc.narg(before)::bytea)
    RETURNING 1
)
SELECT (SELECT count(*) FROM b)::integer AS baseline, (SELECT count(*) FROM u)::integer AS accepted;
```

适配器（`page/adapter/postgres/upgrade.go`）：`baseline == 1 && accepted == 0` 时答 `fmt.Errorf("page: page %s of the upgrade's baseline holds content its guardrails never accepted", nodeID)`（缺陷的错误，500），单元整个回滚、什么也没写。

- 写的是每个正文步，含参与者的改写（`Rewrite`）：发起人的写只有经过护栏才写得进，别的写在冻结里整个回滚，所以对每个基线页 `page_contents.content_hash = accepted_hash` 恒成立；它是绊线（00 号文档 4.6），闸门 7（S2）核对它。
- **账本的写自己核对**（2.1 第 17 条，审查 P2A-M2）：升级中，写之前的哈希（`StepContent.BeforeHash`，单元在 `LockContent` 里锁到的 `current.Hash`）必须是账本认可的那份。不等就是有一次绕过单元的正文写（今天没有这样的路：写 `page_contents` 正文的只有单元的 `CreateContent` 与 `WriteContent`），答缺陷的错误。核对在每个正文步上，含系统的改写：只在守卫里核（初稿的写法）时，绕过之后的一次 `move` 让参与者以 `Rewrite` 改写那一页，守卫不读它，账本被更新成改写之后的哈希，绊线就被洗掉了（g18）。零额外的读：`before` 是单元已有的，`b` 与 `u` 都按主键读同一行。
- 回滚的单元（`rolling_back`，`UpgradeOwn`）交 `nil`、无条件写：它本来就在复原，基线行在升级结束时删去。发起时补写的页（`OpCreate`）不是正文步，不写。审阅中对所有人冻结，没有正文步。
- 代价：笔记本有升级时每个正文步多一条按主键的语句；`batch` 的推后的改写至多 `MaxBatchRewrites` 页，单个 `move` 的改写不设界（同 REST，`batch_plan.go:41`），参与者改写多少页就多写多少条，每条按主键，由 7.1 的护栏的守卫量。
- **测试**：`page/app` 的 `TestTheUnitKeepsTheLedger`（替身的账本：`upgrading` 时每个正文步调一次、带锁到的与写下的两个哈希，参与者的写也调；`rolling_back` 时 `before` 为空；没有升级时不调；`OpCreate` 不调）；`page/adapter/postgres` 的 `TestAcceptUpgradeContent`（基线页在 `before` 等于账本时改、不等时答缺陷的错误且不改、`before` 为空时照改；新建的页与保护行、附件的行不改也不答错）。

### 3.6 S1：码、MCP 的码表与日志

**码**（`page/domain/errors.go`，照 P1 的 `ErrUpgradeProtected`；第 210 条：文字写清是"升级开始时已有的"）：

```go
	// ErrUpgradeImmutable: a write of a page the upgrade started with whose
	// body, past its frontmatter, changed (M11 design 4.6).
	ErrUpgradeImmutable = shared.NewError(shared.KindConflict, "page.upgrade_immutable",
		"The page was there when the upgrade started: an upgrade may change its frontmatter, not the content after it.")
	// ErrUpgradeBatchRewrite: a batch's write of a page the upgrade started
	// with whose links a rename or move earlier in the batch rewrote,
	// whatever it writes (M11 design 4.6, decision 190; M11/P2 decision
	// 268).
	ErrUpgradeBatchRewrite = shared.NewError(shared.KindConflict, "page.upgrade_batch_rewrite",
		"A move earlier in the batch rewrote the links of this page, which was there when the upgrade started; the batch may not write it after that: write the page first and move after, or use two batches. The page's body itself may not change either.")
	// ErrUpgradeKeepLinks: a move or rename that leaves the links to the
	// node as they are, while upgrading (M11 design 4.6).
	ErrUpgradeKeepLinks = shared.NewError(shared.KindConflict, "page.upgrade_keep_links",
		"While the notebook is being upgraded, a move or rename rewrites the links to what it moves: leave update_links true.")
```

**MCP 的码表**（00 号文档第 5 节的 MCP 表）：

| 工具 | `mcpToolCodes()` 的行加（`bootstrap/permission_matrix_mcp_test.go`） | `upgradeToolCodes()` 加的对 |
|---|---|---|
| `write`（`:99-101`） | `page.upgrade_immutable` | (`write`, `page.upgrade_immutable`) |
| `edit`（`:102-103`） | `page.upgrade_immutable` | (`edit`, `page.upgrade_immutable`) |
| `move`（`:104-106`） | `page.upgrade_keep_links` | (`move`, `page.upgrade_keep_links`) |
| `batch`（`:109-112`） | `page.upgrade_immutable`、`page.upgrade_batch_rewrite`、`page.upgrade_keep_links` | 三对 |

各加在行末（`notebook.upgrading`、`page.upgrade_protected` 之后：护栏在冻结的放行之后）。`mcpOwnCodes()`（`bootstrap/mcp_client_test.go:335-341`）加三个码，注释改为"an upgrade's guardrails', which only the initiator's agent meets, REST's writes frozen (M11/P1 design 3.6, M11/P2 design 3.6)"。`TestUpgradeToolsErrorsCarryTheirCodes`（`bootstrap/upgrade_tool_codes_test.go:33-81`）的表加六个情形（两个入口各一次）：`write` 与 `edit` 改基线页的正文、`move` 带 `update_links: false`、`batch` 的三种（基线页的正文；先移动后编辑折进的改写；带 `update_links: false` 的移动）。第五份的 `TestTheMCPToolsErrorsCarryTheirCodes`（`bootstrap/mcp_reads_test.go:475-482`）照旧减去 `upgradeToolCodes()`。

**日志**：护栏不加日志行（拒绝是工具的错误，单元的日志照旧）。

**整个程序的测试** `TestUpgradeGuardrails`（`bootstrap/upgrade_guardrails_test.go:21-91`，P1 的删除照旧）加（`refused` 闭包的码从写死的 `page.upgrade_protected` 改为参数，每格断言"什么也没写"：`sourcePersistentRows` 不变；`done` 的格断言写下了什么）：

| # | 发起人的 agent | 期望 |
|---|---|---|
| g1 | `write` 一页基线页，只改 frontmatter（加 `type`、`summary`） | 成功；账本等于新的哈希 |
| g2 | `edit` 一页基线页的正文一字 | `page.upgrade_immutable` |
| g3 | `write` 一页基线页、正文变了 | `page.upgrade_immutable` |
| g4 | `move` 一页被别的基线页链向的页（`update_links` 缺省） | 成功；那几页基线页被系统改写，账本等于改写之后的哈希（变异"账本不在改写时更新"在这一格死） |
| g5 | `move` 带 `update_links: false`（改父节点） | `page.upgrade_keep_links` |
| g6 | `move` 带 `update_links: false`（同一父节点改名） | `page.upgrade_keep_links` |
| g7 | 新建一页、再改它的正文 | 成功（不是基线页） |
| g8 | 改补写的 `raw` 的正文 | 成功（保护行不是基线） |
| g9 | `batch`：移动 A、再编辑链向 A 的基线页 P 的 frontmatter | `page.upgrade_batch_rewrite`，`op` 是 P 的最后一个正文操作 |
| g10 | g9 拆开：`batch` 先编辑 P、再移动 A | 成功；P 被推后的改写写过，账本等于它 |
| g11 | `batch`：无关的移动、再编辑基线页 P 的正文 | `page.upgrade_immutable`（不是 `batch_rewrite`） |
| g12 | `batch`：编辑 P 两次，第二次改回 | 成功（第 208 条） |
| g13 | `batch` 带 `update_links: false` 的移动 | `page.upgrade_keep_links`，`op` 是那个移动 |
| g14 | 一页以 `---` 起、没有收尾行的基线页：前面加一块有效的 frontmatter | 成功（旧的整页成了正文，字节不变） |
| g15 | 一页带收尾行的无效 frontmatter 的基线页：就地改成有效的；另一页在前面另加一块 | 前者成功，后者 `page.upgrade_immutable` |
| g16 | 字节不变的写（没有被同一个 `batch` 里的移动到达；被到达的见 g23 的后一种） | 成功、什么也没写（第 211 条） |
| g17 | 以 SQL 绕过单元改一页基线页 P 的正文（`UPDATE page_contents` 的正文与哈希），再由发起人只改 P 的 frontmatter | 工具答 `internal_error`，什么也没写，账本照旧（护栏放过，账本的写核对 `before` 不等，3.5）；S2 的 `TestUpgradeGates` 以同一个绕过核对闸门 7 不过 |
| g18 | 以 SQL 同样绕过 P，再 `move` 一页 P 链向的页（`update_links` 缺省） | 工具答 `internal_error`，什么也没写（参与者以 `Rewrite` 改写 P，守卫不读，账本的写核对 `before` 不等，3.5；审查 P2A-M2） |
| g19 | `batch`：移动 A、再编辑链向 A 的基线页 P 的正文一处 | `page.upgrade_batch_rewrite`；拆开（先编辑、再移动）答 `page.upgrade_immutable`（第 265 条） |
| g20 | `batch`：写 P（正文变了、链向 A）、移动 A、移动 B、再编辑 P 的 frontmatter | `page.upgrade_batch_rewrite`；拆开之后那个写答 `page.upgrade_immutable`（第 265 条） |
| g21 | `batch`：移动 A、再写 P = 新的 frontmatter 与改写之后的正文 | `page.upgrade_batch_rewrite`（第 268 条；原来是 `page.upgrade_immutable`）；分两个 `batch`（先移动、再写）成功（第 265 条） |
| g22 | `batch`：盲写 3 页基线页，各删掉一块约 5 MiB 的 frontmatter（发起之前以 SQL 写下这三页：经 REST 在竞态下解析约 15 MiB，贴着测试的请求期限，3.19），head 合计过 14 MiB | `page.batch_too_large`，标在跨过界的那一页的第一个正文操作，什么也没写（第 266 条）；同一个批在没有升级的笔记本上过计划的界 |
| g23 | `batch`：移动 A、再以原来的正文加新的 frontmatter 写 P；或写 P 原来的字节（第 268 条，断链） | 两种都答 `page.upgrade_batch_rewrite`，`op` 1，什么也没写（后一种字节不变，单元照一次写过守卫）；拆成先写后移动的一个 `batch` 做完，P 是 `type: b` 与 `[[Map]]`，账本相符 |
| g24 | `batch`：移动 A、新写一页 `A.md`、再同样写 P（第 268 条，指向别处） | `page.upgrade_batch_rewrite`，`op` 2 |
| g25 | `batch`：移动 A、再编辑 P 把 `[[Map]]` 改回 `[[A]]`（第 268 条） | `page.upgrade_batch_rewrite`，`op` 1 |
| g26 | agent 升级以来写的页 N 链向 A；`batch` 移动 A 之后写 N 原来的字节、写别的字节、或编辑把改写改回，各在自己的笔记本里（审查 P2S1C-N5） | 都做完，N 是 agent 写的样子；P（基线页、本批不写）由推后的改写写成 `[[Map]]`。守卫在读基线行之前就看 `Reached` 时在这里失败 |
| g27 | `batch`：编辑把 P 的 `[[A]]` 拿掉成 `(A)`、改名 A、再放回；同样的形状，改名之后另写 `A.md`；另一本里基线页 Q 由本批在 frontmatter 加 `see: "[[A]]"`、改名 A、再编辑 Q 的 `type`，或以旧名写回整个 Q（第 270 条） | 前两种答 `page.upgrade_batch_rewrite`，`op` 2、3，什么也没写；先放回、后改名做完，P 是 `[[Map]]`，账本相符；Q 的两种在 `op` 2 答 `page.upgrade_batch_rewrite`（这条链接批前的索引里没有，只有正文的一遍标得上，第 268 条） |

### 3.7 S2 的文件

| 文件 | 改动 |
|---|---|
| `wiki/domain/lint.go` | `LintQueryKind` 加三种（3.8） |
| 新的 `wiki/domain/upgrade_check.go`、`upgrade_check_test.go` | 闸门、报告、判定（3.10） |
| `wiki/domain/upgrade.go`、`upgrade_errors.go` | `Upgrade.SubmissionID`、`Upgrade.Report`；`ErrUpgradeNotReady`、`UpgradeNotReady`、`ErrUpgradeSubmitting`（3.10、3.12） |
| `wiki/app/lint_run_types.go`、`lint_runs.go`、`lint_ports.go`、`lint.go` | `RunKey.Scope`、`RunResult`、`RunOutput.Check`、`logFinished` 的名字；`RuleQuery.AreaWantedByPage`；`prepareIn` 交回读到的输入（3.8） |
| `wiki/app/upgrade_ports.go`、`start_upgrade.go` | `UpgradeGatePages`、`UpgradeRows` 的三个写；`UpgradeDeps.Gates`、`SubmitTimeout`、`ReservedKeys`；发起设 `AreaWantedByPage`（3.8–3.12） |
| 新的 `wiki/app/check_upgrade.go`、`submit_upgrade.go` 与它们的测试 | 两个用例（3.11、3.12） |
| `wiki/adapter/postgres/queries/upgrades.sql`、`store.go`；新的 `wiki/adapter/postgres/report.go` | `CurrentUpgrade` 多选两列、三个写、报告的编码（3.12、3.13） |
| `wiki/lint.go` | `Deps` 加 `Gates`、`ReservedKeys`；模块的两个方法 `CheckUpgrade`、`SubmitUpgrade`（`wiki/lint.go:206-215`）；`wiki/module.go` 不改（3.11、3.12；S2 的实施，审查 P2S2C-N2） |
| `wiki/adapter/http/handler.go`、`upgrade.go`、新的 `check.go` | 三个操作、`Upgrade.report`（3.13） |
| `wiki/adapter/mcp/tools.go`、新的 `upgrade.go`、`upgrade_projection.go`、`testdata/tools.json` | 两个工具与投影（3.14） |
| `linking/app/lint_rule_ports.go`、`lint_rules.go`；`bootstrap/wiki_lint.go` | 第二遍另成一项（3.8） |
| `page/upgrades.go`（`ReservedKeys` 在 `:42`；`page/module.go` 不改）、`page/app/upgrades.go`；`page/adapter/postgres/queries/upgrade.sql`、`upgrade.go` | 闸门的读、模块根导出 `ReservedKeys`（3.9） |
| `page/adapter/mcp/instructions.go`、`page/adapter/mcp/tools.go`、`linking/adapter/mcp/tools.go`、`platform/httpserver/mcpserver/tool.go`、`tools_test.go` | 说明的两行、"server update"、`destructiveHint` 与它的单元一格（3.14） |
| `api/modules/wiki.yaml`、`api/openapi.yaml`、`api/modules/events.yaml`、`api/mcp/tools.json` | 契约（3.13、3.14） |
| `bootstrap/wire.go`、`bootstrap/wiki_upgrade.go` | 接线（3.11） |
| 新的 `bootstrap/upgrade_check_test.go`、`upgrade_submit_test.go`（`TestUpgradeSubmit` 与交错的 `TestUpgradeSubmitInterleavings` 都在这里，没有另开 `upgrade_submit_interleavings_test.go`，审查 P2S2A-N3）、`upgrade_tools_matrix_test.go`、`upgrade_check_cost_test.go`；`upgrade_permission_matrix_test.go`、`upgrade_tool_codes_test.go`、`permission_matrix_mcp_test.go`、`permission_matrix_coverage_test.go`（`/upgrade/check/{rule}`）、`mcp_tools_test.go`、`mcp_reads_test.go`（`validToolArguments`）、`upgrade_interleavings_test.go`、`wiki_lint_cost_fixture_test.go`（页数做成参数）；`upgrade_fixtures_test.go` 不改 | 测试（第 7 节）；另配的应用（7.2 的"名额与期限"）的帮手随 6.2 第 5 个提交加（第 8 条先用它）；`wiring` 加 `submitTimeout`（第 6 个提交） |
| `page/adapter/postgres/upgrade_cost_test.go`、`rollback_cost_test.go` | 闸门的读的计划；判计划的帮手认 `changesets`（3.9、7.1） |
| `Makefile`、`.github/workflows/ci.yml`、`docs/v0.1/claude_to_codex/06-environment.md` | 第八份（6.2 第 1 个提交）；检查的守卫的一行（7.1） |
| `W/…`（4.2）、`e2e/…`（第 5 节） | 随服务端的部分；e2e 的 LW8（`e2e/stories/wiki/lw8-health.spec.ts`）的工具数随工具表改（3.14） |

### 3.8 S2：lint 与 `Runs`

**键**（`wiki/app/lint_run_types.go:13-23`）：`RunKey` 加

```go
	// Scope is an upgrade's check's and items' upgrade id, a submission's
	// id (M11 design 4.7): checks of one upgrade join, two upgrades' never,
	// a submission none (it would keep a report of a snapshot older than
	// its freeze). Zero for lint's.
	Scope uuid.UUID
```

`wiki/domain/lint.go:61-67` 的 `LintQueryKind` 加 `LintUpgradeCheck`、`LintUpgradeItems`、`LintUpgradeSubmit`（只用在键上，结果的 `Kind` 照旧是 `LintSummary` 或 `LintItems`）。

**结果**：

```go
// RunOutput gives the owner statistics even on a returned error; callers
// receive only a successful independent Result, never a partial response.
// Check is an upgrade's check's or submission's, nil for lint's.
type RunOutput struct {
	Result domain.LintResult
	Check  *domain.UpgradeCheck
	Stats  RunStats
}

// RunResult is a flight's one result, a copy for each caller.
type RunResult struct {
	Lint  domain.LintResult
	Check *domain.UpgradeCheck
}

func (r *Runs) Do(ctx context.Context, key RunKey, run func(context.Context) (RunOutput, error)) (RunResult, error)
```

`lintFlight.result` 改为 `RunResult`；`finish`（`lint_runs.go:198-234`）发布 `RunResult{Lint: CloneLintResult(output.Result), Check: domain.CloneUpgradeCheck(output.Check)}`，`Do` 交回同样的副本。`Runner`（`wiki/app/lint_ports.go:130-132`）随之改；`Lint.Execute`（`lint.go:111-121`）取 `.Lint`。

**日志的名字**（`logFinished`，`lint_runs.go:295-352`）：`LintUpgradeCheck` 记 `query_kind = "upgrade_check"`、`rule = "summary"`；`LintUpgradeItems` 记 `"upgrade_items"` 与规则名；`LintUpgradeSubmit` 记 `"upgrade_submit"`、`"summary"`；这三种另加 `upgrade_scope`（键的 `Scope`，升级或提交的 id，13.1 第 10 条：只是 id）。不再记成 `unknown`（核对 B2-N3）。

**第二遍另成一项**：`RuleQuery`（`wiki/app/lint_ports.go:82-87` 与 linking 的 `lint_rule_ports.go:39-46`）：

```go
// WantedByPage, with the summary or with the wanted rule, also counts each
// general emitter's distinct wanted targets: an upgrade's check compares
// them with its baseline (M11 design 4.7 gate 6). AreaWantedByPage, with
// WantedByPage only, adds an LLM contract's area's current pages, as an
// upgrade's start records them (the second pass).
type RuleQuery struct {
	Rule             *domain.LintRule
	After            *domain.LintPosition
	Limit            int
	WantedByPage     bool
	AreaWantedByPage bool
}
```

linking 的 `lintQuery`（`lint_rules.go:43-71`）：`AreaWantedByPage && !WantedByPage`、`AreaWantedByPage` 而没有规则（摘要不跑第二遍，不拒就静静丢掉它；审查 P2S2A-N2，修复一轮）、`WantedByPage` 而规则既不是空也不是 `wanted` 时答 `errLintQuery`；摘要的计划带 `byPage`（今天 `:44-46` 拒它）；`Prepare`（`:120-122`）只在 `plan.area && scope.mode == LLM` 时跑第二遍。组合根的 `wikiRules`（`bootstrap/wiki_lint.go:142`）多抄一项。发起（`wiki/app/start_upgrade.go:258`）两项都设，行为不变（`TestUpgradeStart` 与 P1 的第二遍的测试照旧过）。测试：`linking/app` 的 `TestTheWantedCountsByPage` 加"摘要带按页的数、不跑第二遍""`AreaWantedByPage` 单独答 `errLintQuery`"两格；修复一轮另加"摘要带 `AreaWantedByPage` 答 `errLintQuery`"一格。

**`prepareIn` 交回它读到的输入**（`wiki/app/lint.go:239-298`）：签名改为 `prepareIn(ctx, scope, query) (PreparedRules, RunStats, ReadInput, error)`，交回 `input`（全部活节点 `Nodes` 与区里的 `SourceIDs`，已是副本，`:279-283`）；`computeIn` 与发起忽略它，检查用它（闸门 3 与报告的计数）。

**测试**（`wiki/app/lint_runs_test.go`）：`TestTheUpgradeKeysJoinOnlyTheirOwn`（同一次升级的两次检查合并为一个 flight；两次升级的检查、同一次升级的检查与条目、任何东西与提交，都不合并；同一个根的前后两次升级的条目不合并，核对 B2-N2）；`TestTheRunsCopyACheck`（两个调用者拿到的 `Check` 互不相干：改一个的切片不动另一个）；`TestTheUpgradeKindsAreNamed`（照审查 B 的 `TestReviewBJoinAfterSnapshot` 的写法读日志：三种名字、`upgrade_scope`，不出现 `unknown`）。

### 3.9 S2：page 的闸门的读

wiki 的端口（`wiki/app/upgrade_ports.go`）：

```go
// UpgradeGatePages are the page module's reads of an upgrade's check (M11
// design 4.7), each in the snapshot ctx carries.
type UpgradeGatePages interface {
	// Roots are gates 1 and 2's: the page at the notebook's root keyed
	// agents, nil for none (an attachment is none), whether its body past
	// its frontmatter and a BOM, trimmed, is not empty; whether rootID is
	// a live page of the notebook.
	Roots(ctx context.Context, notebookID, rootID uuid.UUID) (domain.UpgradeRoots, error)
	// Baseline is the upgrade's baseline rows: each node's kind and parent
	// then, a page's wanted count then.
	Baseline(ctx context.Context, upgradeID uuid.UUID) ([]domain.BaselineNode, error)
	// Preserved is gate 7's (M11 design 4.7): the baseline's nodes and
	// pages, those alive and those whose content is the ledger's, and the
	// first five that are not, by id, in one pass.
	Preserved(ctx context.Context, upgradeID uuid.UUID) (domain.Preserved, error)
	// Changesets is how many changesets the upgrade wrote.
	Changesets(ctx context.Context, upgradeID uuid.UUID) (int, error)
}
```

page 的模块根 `Upgrades`（`page/upgrades.go`）加四个同名的方法，组合根的 `wikiUpgradePages`（`bootstrap/wiki_upgrade.go:109`）转换；模块根另导出 `func ReservedKeys() []string { return domain.ReservedKeys() }`（`page/domain/contract.go:88`；第 8 节 page 一条已列），组合根交给 `wiki.Deps.ReservedKeys`（闸门 3 不在 wiki 另写一份，00 号文档 4.7）。

`Roots` 不另写 SQL：`RootNodesByKey(notebookID, ["agents"])`（`upgrade.sql:51-55`，P1 的发起已在它的成本守卫里核对过计划）、是页时 `PageContent`（`page/adapter/postgres/store.go:337-343`，按主键）、`FindNodeIn(notebookID, rootID)`（按主键），正文的判断是 `strings.TrimSpace(domain.UpgradeBody(content, markdownadapter.FrontmatterEnd(content))) != ""`（同 `AgentsBody` 的读法，`page/domain/instructions.go:34-35`）。

```sql
-- name: UpgradeGateBaseline :many
-- An upgrade's baseline rows for its check (M11 design 4.7 gate 6 and the report's counts), by upgrade_nodes_pkey's
-- prefix: each node's kind and parent as it started, a page's distinct wanted targets then.
SELECT node_id, kind, parent_id, wanted FROM upgrade_nodes
WHERE upgrade_id = sqlc.arg(upgrade_id) AND baseline;

-- name: UpgradePreserved :one
-- Gate 7 of an upgrade's check (M11 design 4.7, decision 191): its baseline's nodes and pages, those alive, the
-- pages whose content now is the one the guardrails last accepted, and the first five not kept, by id: dead, or a
-- page whose content is not the ledger's. The baseline rows by upgrade_nodes_pkey's prefix, each row's node by
-- nodes_pkey and its content by page_contents_pkey, one row at a time (decisions 240, 244): the LIMIT 1 keeps each
-- LATERAL from being pulled up into a join. No content is read; one pass, a failing gate's ids included.
SELECT count(*)::integer AS nodes,
    (count(*) FILTER (WHERE n.alive))::integer AS nodes_kept,
    (count(*) FILTER (WHERE b.kind = 'page'))::integer AS pages,
    (count(*) FILTER (WHERE b.kind = 'page' AND c.content_hash = b.accepted_hash))::integer AS pages_kept,
    COALESCE((array_agg(b.node_id ORDER BY b.node_id) FILTER (
        WHERE n.alive IS NULL OR b.kind = 'page' AND c.content_hash IS DISTINCT FROM b.accepted_hash))[1:5],
        '{}')::uuid[] AS not_kept
FROM upgrade_nodes b
LEFT JOIN LATERAL (
    SELECT true AS alive FROM nodes n WHERE n.id = b.node_id AND n.deleted_at IS NULL LIMIT 1
) n ON true
LEFT JOIN LATERAL (
    SELECT c.content_hash FROM page_contents c WHERE c.node_id = b.node_id AND c.deleted_at IS NULL LIMIT 1
) c ON b.kind = 'page'
WHERE b.upgrade_id = sqlc.arg(upgrade_id) AND b.baseline;

-- name: UpgradeChangesets :one
-- How many changesets an upgrade wrote (M11 design 4.7's report), by changesets_upgrade_id_idx alone. No
-- deleted_at: a changeset's goes only with its notebook's (nodes.sql:315-316), whose upgrade's wiki row goes in the
-- same moment, so a check that read the upgrade under way reads a live notebook's; with it, the planner meets the
-- condition with the partial index of every notebook's live changesets (00037) in a BitmapAnd when the statistics
-- were never gathered (design review P2B-M1: 132-140 ms at two million changesets, 0 ms without).
SELECT count(*)::integer FROM changesets WHERE upgrade_id = sqlc.arg(upgrade_id);
```

- 写法照 P1/S3 的回滚的读（`RollbackLeft`，`upgrade.sql:141-168`）：基线行驱动、每行的节点与正文各经主键、`LATERAL … LIMIT 1`；sqlc 把 `LEFT JOIN LATERAL` 子查询的列当非空（P1 设计 3.27 的偏离 7），这一条只交计数与 `not_kept`（`COALESCE … ::uuid[]` 让它非空），不撞它。
- 不过的前五个与计数在同一遍里（初稿另有一条 `UpgradeNotPreserved`，闸门 7 不过时再走一遍全部基线行，审查 B 实测十万行另 212–241 毫秒；照 B 的可选建议并进来，失败的情形不慢一倍）。`array_agg … FILTER` 只收不过的行，通常是空的；但它的 `ORDER BY b.node_id` 让 PostgreSQL 16 起的预排序聚合先把这次升级的全部基线行按 `node_id` 排好，闸门过与不过都付这一笔（审查 B 在修订核对里实测，十万行：六格里五格是 `Aggregate ← Sort ← Nested Loop`，按 `upgrade_nodes_pkey` 有序读的那一格不排）。排序只作用于这次升级的行，计划仍只经键；`UpgradePreserved` 247–316 毫秒（并进之前 217–261 毫秒），三条读在一个快照里 312–343 毫秒（之前 271–283 毫秒）；闸门 7 不过时仍只走一遍，比两遍（约 0.44 秒）省（审查 P2B-N5，只改数，不在 Go 里排序）。
- 审查 B 在设计审查里实测（并进之前；十万行基线，另九本各五万页与它们的升级；这次升级 1,000 个、别的升级各 1,000 个、普通的 10 万个变更集，另一个变体 200 万个活的变更集；三种统计 × 两种计划）：`UpgradeGateBaseline` 28–50 毫秒，`UpgradePreserved` 的两次十万个 `LATERAL` 217–261 毫秒，`UpgradeChangesets` 0–3 毫秒；计划都只经键。初稿的 `UpgradeChangesets` 带 `deleted_at IS NULL`，从未分析时两种计划都是 `BitmapAnd`（`changesets_upgrade_id_idx` 与没有索引条件的 `changesets_notebook_id_created_by_id_created_at_id_idx … WHERE deleted_at IS NULL`，`00037_page_changesets_log.sql:11-12`），把全库活的变更集读进位图；去掉之后只经 `changesets_upgrade_id_idx`。
- `Preserved` 的 `Nodes`、`Pages` 是现读的行数，闸门 7 的分母是 wiki 行的 `baseline_nodes`、`baseline_pages`（3.10）。
- `Changesets` 数的是这次升级写下、历史里还留着的变更集：拖过回收站保留期（`jobs.purge_retention`，默认 60 天、至少 24 小时）的升级，其中内容已被清掉的（`PurgeTrimmedChangesets` 删 `trimmed`、条目与版本都已清掉的）不再计入；`DropEmptyChangeset` 删的是单元在自己的事务里建了又没写下东西、从没提交过的变更集，不影响它。这只是报告里的一个数、不是闸门，与审阅的人在历史里能看到的一致（审查 B 的未核实的怀疑，修复一轮查代码、B 核过，3.20）。
- **测试**（`page/adapter/postgres/upgrade_test.go`）：`TestTheUpgradeGateReads`（基线行带 `parent_id` 与 `wanted`、附件没有 `wanted`；保全：全在、删一个基线附件、以 SQL 改一页的正文而账本不变、改账本而正文不变、保护行不算，前五个按 id；变更集的数只数这次升级的、不数别的升级的）；`page` 模块根的 `TestTheUpgradeRoots`（根下的 `AGENTS` 是页、正文只有 frontmatter、只有 BOM 与空白、是附件、没有；来源根是活页、已删、是另一本笔记本的页）。

### 3.10 S2：闸门与报告的判定

`wiki/domain/upgrade_check.go`（纯函数，表格驱动的单元测试）：

```go
// UpgradeGateKey names a gate of an upgrade's check (M11 design 4.7), in
// the order the check and the refusal list them.
type UpgradeGateKey string

const (
	GateAgents         UpgradeGateKey = "agents"
	GateSourceRoot     UpgradeGateKey = "source_root"
	GateReservedTitles UpgradeGateKey = "reserved_titles"
	GateFrontmatter    UpgradeGateKey = "frontmatter"
	GateSources        UpgradeGateKey = "sources"
	GateLinks          UpgradeGateKey = "links"
	GatePreserved      UpgradeGateKey = "preserved"
	GateIndex          UpgradeGateKey = "index"
)

// UpgradeGates are a check's gates: the seven of overall design 5.8 and
// whether the link index is fresh.
type UpgradeGates struct {
	Agents, SourceRoot, Frontmatter, Sources, Index bool
	ReservedTitles NodesGate     // at most the four reserved keys' nodes
	Links          LinksGate     // the first five worse pages, by id
	Preserved      PreservedGate // the first five not kept, by id
}

type NodesGate struct {
	Passed  bool
	NodeIDs []uuid.UUID
}

type LinksGate struct {
	Passed     bool
	PagesWorse int
	NodeIDs    []uuid.UUID
}

type PreservedGate struct {
	Passed                           bool
	Nodes, NodesKept, Pages, PagesKept int
	NodeIDs                          []uuid.UUID
}

// Failed are the gates that did not pass, in their order.
func (g UpgradeGates) Failed() []UpgradeGateKey

// UpgradeCounts are what the notebook holds as checked (M11 design 4.7).
type UpgradeCounts struct {
	Sources, MovedIntoSources, WikiPages, NewPages, Changesets int
	WantedBefore, WantedNow                                    LintCount
}

// UpgradeCheck is an upgrade's check (M11 design 4.7): its time, stale
// pages, gates, the eight rules' summaries of an LLM Wiki's explicit
// contract with the proposed source root, and its counts; Paths, for MCP,
// the gates' nodes' paths, those in the live tree.
type UpgradeCheck struct {
	ComputedAt time.Time
	StalePages int64
	Passed     bool
	Gates      UpgradeGates
	Rules      []LintRuleResult
	Counts     UpgradeCounts
	Paths      map[uuid.UUID]string
}

// UpgradeReport is a check a submission kept (M11 design 4.7): its rules
// with their count and first five items, no cursor; no paths.
type UpgradeReport struct {
	ComputedAt time.Time
	StalePages int64
	Passed     bool
	Gates      UpgradeGates
	Rules      []ReportRule
	Counts     UpgradeCounts
}

type ReportRule struct {
	Rule  LintRule
	Count LintCount
	Items []LintItem // no cursor nor path, at most five
}

// ReportOf is the report a submission keeps of c.
func ReportOf(c UpgradeCheck) UpgradeReport

// UpgradeRoots, BaselineNode, Preserved: what the page module read (3.9).
type UpgradeRoots struct {
	AgentsID   *uuid.UUID
	AgentsBody bool
	RootPage   bool
}

type BaselineNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Kind     NodeKind
	Wanted   int // a page's; 0 for an attachment
}

type Preserved struct {
	Nodes, NodesKept, Pages, PagesKept int
	NotKept                            []uuid.UUID
}

// GateNode is a live node as lint read it.
type GateNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Kind     NodeKind
	Key      string
}

// UpgradeRead is what one snapshot read for an upgrade's check.
type UpgradeRead struct {
	Upgrade      Upgrade
	Roots        UpgradeRoots
	Nodes        []GateNode
	Area         []uuid.UUID // the proposed root's strict descendants
	Lint         LintResult  // the summary
	WantedByPage map[uuid.UUID]int
	Baseline     []BaselineNode
	Preserved    Preserved
	Changesets   int
	Reserved     []string
}

// JudgeUpgrade is the check of r (M11 design 4.7), but its time and paths.
func JudgeUpgrade(r UpgradeRead) UpgradeCheck
```

判定（每格一个反例的表，`TestJudgeUpgrade`）：

| 键 | 过的条件 | 反例 |
|---|---|---|
| `agents` | `Roots.AgentsID != nil && Roots.AgentsBody` | 没有；是附件；正文只有 frontmatter；只有空白 |
| `source_root` | `Roots.RootPage && (AgentsID == nil \|\| *AgentsID != Upgrade.SourceRootID)` | 来源根被改名为 `AGENTS`（B-Q2） |
| `reserved_titles` | `Nodes` 里没有 `ParentID == nil` 且 `Key` 在 `Reserved` 里的；列出它们（按 id） | 根下的 `index` 页、`log.md` 附件各一；不在根下的 `index` 不算 |
| `frontmatter` | `invalid_frontmatter`、`missing_type`、`missing_summary` 三条的数都是 0、都不封顶 | 三条各一 |
| `sources` | `invalid_sources` 的数是 0 | 一个 |
| `links` | 对 `WantedByPage` 里每一页：有基线行（页）且现在的数大于 `Baseline.Wanted` 的算"更差"；没有更差的就过；`PagesWorse` 是数，`NodeIDs` 是按 id 的前五个 | 抵消（一页多一个待建、另一页少一个：不过，第 180 条）；一页的待建到 10,000（一页的链接数的上限，`linking/domain/facts.go:114`，与 `upgrade_nodes_wanted_check` 同界，所以不封顶，第 188 条）：基线 10,000、现在 10,000 过，基线 3、现在 4 不过；新建页的待建不算；基线页不在表里（现在 0）过；从发起时的 `raw` 移出、基线 `wanted` 是第二遍记下的 1、现在 1：过 |
| `preserved` | `Preserved.Nodes == Upgrade.BaselineNodes && Preserved.Pages == Upgrade.BaselinePages && NodesKept == Nodes && PagesKept == Pages`；`NodeIDs` 是 `NotKept` | 删了一页（缺陷之外到不了）；一个基线附件保留着（过）、已删、它的基线行没了（经 `withAttachment` 带一个基线附件，后两格各单独不过；审查 P2S2A-M2，修复一轮）；一页的正文不是账本的；现读的行数少一行（分母取 wiki 行） |
| `index` | `Lint.StalePages == 0` | 一页过时 |

`Passed` 是八格都过。计数：`Sources` 是 `len(Area)`；`MovedIntoSources` 是现在在 `Area` 里的基线节点、减去发起时就在提议的根之下的（以 `Baseline` 的 `ParentID` 在 Go 里重建发起时的树，提议的根没有基线行即发起时补写的 `raw`，发起时它之下没有节点，审查 B-N8）；`WikiPages` 是 `Nodes` 里不在 `Area` 里的页、除去提议的根与 `AgentsID`；`NewPages` 是 `Nodes` 里没有基线行的页（2.1 第 6 条）；`WantedBefore` 是 wiki 行的 `baseline_wanted` 与是否封顶；`WantedNow` 是 `wanted` 规则的数。计数各有一格（S2 拆成 `TestJudgeUpgradesCounts`）：`raw` 原有、下面的两页发起时就在（不算移进）、agent 再移进一页（算一）；补写的 `raw`，移进两页（算二）；`wanted` 的数封顶（`{10000, true}`）时 `WantedNow` 照样封顶（审查 P2S2C-M2，修复一轮）。

**报告的大小**（第 263 条）：`ReportOf` 去掉游标与条目的 `path`（REST 的条目本来就没有它，3.13）之后，最坏是三条带目标的规则（`wanted`、`ambiguous`、`invalid_sources`）各 5 项、目标各 256 字节而全是要转义的控制字符（`jsonb` 的文本把它们写成 `\u00XX`，至多 6 倍，约 1.5 KB），其余五条各 5 项（`item_key` 至多 140 字节加 id），加闸门与计数，约 33 KB，在 `notebook_upgrades_report_check` 的 65,536 字节之内。用例与适配器不另量（2.1 第 16 条）：过了界只能是缺陷，表的检查拒绝、答 500。`wiki/adapter/postgres` 的 `TestTheWorstReportFits` 以这个最坏的形状编码、经真库写进去、读回来相等，并打出编码之后的字节数（S2 实测 35,125 字节；修复一轮把它的 `wanted_now` 也封顶之后 35,128 字节，后四轮不变）。

**测试**：`TestJudgeUpgrade`（上表）、`TestJudgeUpgradesCounts`（计数；七个值互不相同的一格在修复第三轮加，审查 P2S2C-M5）、`TestReportOf`（游标与条目的 `path` 都去掉、每条至多五项、计数与闸门照抄、`Paths` 不进；修复第四轮改成整个结构体与写死的 `UpgradeReport` 比，计数与闸门的值互不相同，审查 P2S2C-M6）、`TestTheGatesFailInTheirOrder`（`Failed()` 的次序）。

### 3.11 S2：检查与条目

**用例**（`wiki/app/check_upgrade.go`）：

```go
// CheckUpgrade checks a notebook's upgrade, or reads one rule's items in
// its scope (M11 design 4.7): GET …/upgrade/check, GET …/upgrade/check/
// {rule} and MCP's upgrade_verify.
type CheckUpgrade struct {
	d    UpgradeDeps
	lint *Lint
	runs Runner
}

// UpgradeChecked is a check (query without a rule) or a page of a rule's
// items: one of the two.
type UpgradeChecked struct {
	Check *domain.UpgradeCheck
	Items *domain.LintResult
}

func NewCheckUpgrade(d UpgradeDeps, lint *Lint, runs Runner) *CheckUpgrade

func (c *CheckUpgrade) Execute(ctx context.Context, id uuid.UUID, query LintQuery) (UpgradeChecked, error)
```

次序（00 号文档 4.7、4.16；第 5 节的码依次）：

1. 游标的形状：`query.Cursor` 不为空时 `Cursors.Decode`，解不开 400 `bad_request`（同 `Lint.Execute`，`lint.go:37-48`）。
2. `Notebooks.WorkspaceOf`（没有 404 `notebook.not_found`），判定 `UpgradeAction`（`notebook.change_mode`；看不到 404，不是管理员 403 `forbidden`）。
3. 没有规则而有 `limit` 或 `cursor`：422 `validation_failed`（字段 `limit`、`cursor`，`requires a rule`，同 `lint.go:63-73`；只有 MCP 到得了，REST 的两个操作一个不带参数、一个必带规则）。有规则而不认识：404 `wiki.lint_rule_not_found`。
4. `Rows.Current(id)` 不加锁：没有，或状态不是 `upgrading`、`reviewing`：409 `wiki.upgrade_state`。范围是 `LintScope{NotebookID: id, Contract: &llm_wiki, AreaRootID: &up.SourceRootID}`。
5. 条目：`PositionFor(scope, rule)` 不合：400 `bad_request`（2.1 第 8 条）；`shared.PageSize(query.Limit)`：422 `validation_failed`；`CursorCanonical` 照 `lint.go:101-106`。
6. `runs.Do(ctx, key, worker)`，`key` 是 `{NotebookID: id, Explicit: true, Contract: llm_wiki, HasRoot: true, RootID: up.SourceRootID, Kind: LintUpgradeCheck 或 LintUpgradeItems, Rule, Limit, CursorCanonical, Scope: up.ID}`；名额与等待照 `Do`（503 `server_busy`，`Retry-After: 1`）。
7. 交付之前再判一次第 2 步（同 lint 在运行之后再判，`lint.go:114-116`；合并的计算由别人开始，每个调用者各判各的）；`ctx.Err()`。

**worker**（在 `Snapshots.WithinSnapshot` 里，`at` 是快照里读的一刻，同 `compute`，`lint.go:196-212`）：

- 再读 `Rows.Current(id)`：不是第 4 步读到的那一次升级（id 不同），或不在 `upgrading`、`reviewing`：`ErrUpgradeState`（审查 B-M3，交错第 8 条）。
- 条目：`lint.computeIn(snapshot, scope, RuleQuery{Rule, After, Limit}, LintItems, at)`。
- 检查（`checkIn`，提交也用它，3.12）：
  1. `Gates.Roots(id, up.SourceRootID)`；`!RootPage` 记一行 error（"upgrade source root is no live page"：`notebook_id`、`upgrade_id`）、答缺陷的错误（2.1 第 3 条）。
  2. `lint.prepareIn(snapshot, scope, RuleQuery{WantedByPage: true})`，再 `lint.project`（`wiki/app/lint_projection.go:11`）出八条规则的摘要（`Kind: LintSummary`，`ComputedAt: at`）。
  3. `Gates.Baseline(up.ID)`、`Gates.Preserved(up.ID)`、`Gates.Changesets(up.ID)`。
  4. `domain.JudgeUpgrade(…)`；`Preserved` 的行数不等于 wiki 行的两个数时记一行 error（"upgrade baseline rows not its counts"：`notebook_id`、`upgrade_id`、四个数，00 号文档 4.7 闸门 7）。
  5. 三道闸门的节点（至多 14 个）经 `lint.deps.Paths.Paths`（`lint_ports.go:69-71`）取路径，不在活的树里的（缺陷之外只有闸门 7 的已删节点）不进 `Paths`。
- 全部读在一个快照里（REPEATABLE READ、只读，`platform/postgres/tx.go:137-144`），所以闸门、规则与计数是同一刻的（S3A-M2 的教训）。

**`UpgradeDeps`**（`start_upgrade.go:22-40`）加 `Gates UpgradeGatePages`、`Cursors Cursors`、`Paths Paths`（照 `LintDeps`；`New` 从 `Deps` 抄）、`ReservedKeys []string`、`SubmitTimeout time.Duration`。模块（`wiki/lint.go:149-172`）建 `NewCheckUpgrade(upgrades, lint, runs)`、`NewSubmitUpgrade(upgrades, check, runs)`，加方法：

```go
// CheckUpgrade checks notebook id's upgrade under way, or reads one rule's
// items in its scope (M11 design 4.7).
func (m *Module) CheckUpgrade(ctx context.Context, id uuid.UUID, query LintQuery) (UpgradeChecked, error)

// SubmitUpgrade submits notebook id's upgrade for its admins' review
// (M11 design 4.8).
func (m *Module) SubmitUpgrade(ctx context.Context, id uuid.UUID) (UpgradeView, error)
```

组合根（`bootstrap/wire.go:247-276`）：`ReservedKeys: page.ReservedKeys()`；`SubmitTimeout: lintTimeout + 10*time.Second`（`lintTimeout` 在 `:246`），测试的 `wiring`（`:62-88`）加 `submitTimeout`，不为零时取它（只有 `TestUpgradeSubmit` 的期限一格设它：6 s，比标准应用 4 s 的请求期限长、比 MCP 的 9 s 短，照默认配置的比例；交错第 7 条的"另一次提交"就是这一格，7.1、7.2）；`Upgrades` 一项的 `wikiUpgradePages` 同时实现 `UpgradeGatePages`（`wiki.Deps.Gates`）。

**测试**（`wiki/app/check_upgrade_test.go`，假的端口）：`TestCheckUpgradeAnswersInOrder`（上面的每个码各一格，两两相邻的码各一格"前一个先答"：坏的游标与看不到的笔记本答 400；看不到又不认识的规则答 404 `notebook.not_found`；不认识的规则而升级在回滚中答 `wiki.lint_rule_not_found`；升级在回滚中而游标绑别的根答 409；绑别的根而 `limit` 越界答 400；`limit` 越界而名额满答 422）；`TestTheCheckReadsInOneSnapshot`（读的次序恰是 `Current, Roots, state, nodes, prepare, encode, paths, Baseline, Preserved, Changesets, Paths`，都在一次 `WithinSnapshot` 里）；`TestTheCheckAuthorizesBeforeItsAnswer`（第二次判定答 `forbidden` 时不交结果）；`TestTheCheckRefusesAnotherUpgradesSnapshot`（快照里读到别的 id 或 `rolling_back`）。

### 3.12 S2：提交

**用例**（`wiki/app/submit_upgrade.go`），`Execute(ctx, id) (UpgradeView, error)`，00 号文档 4.8 的次序：

1. actor、客户端；`WorkspaceOf`（404）。
2. 第一个事务（请求的 ctx）：`UpgradeDeps.locked`（工作区 `FOR SHARE` → 笔记本 `FOR NO KEY UPDATE` → 判定 404、403 → 时钟，`start_upgrade.go:47`、`:62`）；提交用 `at := now.Truncate(time.Microsecond)`：库里的 `timestamptz` 只到微秒，`submitted_at`、接管的判断与第 3 步的期限都以存下的那个值为准（审查 P2A-N4）。状态：
   - 没有升级、`rolling_back`：409 `wiki.upgrade_state`。
   - `upgrading`：往下。
   - `reviewing`：读 `Rows.Current(id)`；有报告：409 `wiki.upgrade_state`；没有报告而 `at < submitted_at + SubmitTimeout`：409 `ErrUpgradeSubmitting`（码 `wiki.upgrade_state`，`detail` 写"The previous submission is still being checked: check again in a moment."，核对 A2-M2）；否则接管（崩溃或到期留下的"审阅中、没有报告"）。
   - 写：`submission := uuid.NewV7()`；`States.SetState(id, up.ID, reviewing, actor, at)`；`Rows.Submit(up.ID, submission, actor, at, at.Add(-SubmitTimeout))`（0 行是缺陷）；`Publish(ModeChange{From: standard, To: standard, …})`；提交。
3. 脱离请求的取消，不晚于请求的期限（2.1 第 7 条，第 258 条）：

   ```go
   deadline := at.Add(d.SubmitTimeout)
   if req, ok := ctx.Deadline(); ok && req.Before(deadline) {
   	deadline = req
   }
   detached, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
   defer cancel()
   ```

   之后都用 `detached`。接管的安全只要求第二段在 `submitted_at + SubmitTimeout` 之前结束，取较早的那一个不破坏它。默认配置（`request_timeout: 15s`，`LintTimeout` 7.5 s，`SubmitTimeout` 17.5 s）下两个入口的第二段都到请求的期限为止，第 4 步 `Do` 的预算 `min(7.5 s, (15 s − e)/2) ≈ 7.47 s`（`e` 是请求开始到调用 `Do` 的时长），与 `LintTimeout` 几乎相同，第二个事务还剩约 7.5 秒。MCP 的工具调用的期限是读完请求体之后的 `RequestTimeout`（`platform/httpserver/mcpserver/call.go:46-53`；`/mcp` 路由的期限另加 `BodyReadTimeout`，`platform/httpserver/api.go:297-310`，总晚于它），`at` 又在读完请求体之后，所以 `request_timeout` 不超过 20 秒（默认配置在内）时两个入口都是请求的期限先到；超过 20 秒时 `SubmitTimeout` 恒为 20 秒，第二段到 `submitted_at + SubmitTimeout` 为止；两种情形接管都安全（第 273 条，审查 P2S2A-M4、P2S2C-N1；初稿写"MCP 的请求期限另加 `BodyReadTimeout`，第二段到 `submitted_at + SubmitTimeout` 为止"，不对）。`at` 由用例的时钟读，ctx 的期限按运行的时钟走，生产里两者是同一个；用例的测试的假时钟要以"现在"起算（审查 P2A-N5）。
4. `runs.Do(detached, RunKey{…检查的范围…, Kind: LintUpgradeSubmit, Scope: submission}, worker)`：worker 在快照里读 `Current`，要它是这一次升级、`reviewing`、`SubmissionID == submission`、没有报告，否则 `ErrUpgradeState`；之后照 `checkIn`（3.11）。
5. 第二个事务（`detached`）：工作区 `FOR SHARE` → 笔记本 `FOR NO KEY UPDATE` → `Rows.Current(id)`；不是这一次提交（升级的 id、`reviewing`、`SubmissionID`、没有报告，任一不合）：什么也不写，`outcome = superseded`，答 409 `wiki.upgrade_state`。否则：
   - 第 4 步交回错误（503 或别的）：`unsubmit`（`States.SetState(…, upgrading, …)`、`Rows.Unsubmit(up.ID, submission, …)`、`Publish`），答那个错误，`outcome = failed`。
   - `StalePages > 0`：`unsubmit`，答 409 `wiki.upgrade_stale_index`，`outcome = stale`。
   - `!Passed`：`unsubmit`，答 409 `UpgradeNotReady(check.Gates.Failed())`（码 `wiki.upgrade_not_ready`，`detail` 列出不过的闸门的键，不带报告，第 182 条），`outcome = not_ready`。
   - 过：`Rows.KeepReport(up.ID, submission, domain.ReportOf(check), actor, at2)`（`at2` 是这个事务读的一刻；0 行是缺陷），`Publish`（`From == To`，第 216 条），`outcome = stored`。
   - 写完之后、`COMMIT` 之前：交付之前的判定（同第 1、2 步的 404、403，同检查的"交付之前的授权"），在这个事务里、以 `detached` 读（`Authorize` 每次读库，`access/app/authorizer.go:27-48`）。它只决定答复：拒绝时答它（404 或 403），写下的照样提交、报告照样存下；不拒时答上面的结果。初稿把它放在提交之后，那时 `detached` 可能已到期、REST 的请求也已到期，判定本身失败，存下了报告却答 500（审查 P2A-I1）。每种结局（含 `superseded`）都照这个判定、照 13.1 第 4 条的码序，拒绝时答它；这个事务里锁不到笔记本（已删）时记 `superseded`、什么也不写，答 `notebook.not_found`、不再判（同第一个事务的锁会答的）。退回经组合根的 `wikiUpgradeStates.SetState`，它原来只认 `reviewing`、`rolling_back`，加 `upgrading`（S2 的实施，3.20）。
6. 第 5 步的 `WithinTx` 交回包着 `context.DeadlineExceeded` 的错误、而 `detached` 已到期：记一行 warn（3.16），答 503 `server_busy`（`shared.ServerBusy`；`Retry-After` 是到能接管为止的秒数 `max(1 s, ceil(at + SubmitTimeout − now))`，第 274 条：两个入口在默认配置下都在请求的期限答，这一刻离能接管还有约 2.5 秒，初稿的 1 秒让重试落进"正在计算"的 409，审查 P2S2A-N1；契约 `submitUpgrade` 的说明同改；lint 的名额满时的 503 照旧 1 秒）。它是 `ProblemError`，请求的期限过了平台也照样写出（`platform/httpserver/apierrors.go:266-268`）；REST 与 MCP 答同样的码（13.4 第 3 条）。客户端已断开（请求的 ctx 被取消）时没有人收答复，照平台的 `FaultGone`。行留在"审阅中、没有报告"，过了 `submitted_at + SubmitTimeout` 任何管理员与发起人的 agent 都可以再提交（第 2 步认这种状态）。`WithinTx` 交回 nil 就是已提交：`COMMIT` 脱开取消（`tx.go:199-203`），落在期限之后照样答它的结果（判定已在 `COMMIT` 之前做过）；这时若已有人接管，接管的第一个事务在笔记本行上等这个 `COMMIT`，之后看到报告、答 `wiki.upgrade_state`。`COMMIT` 自己在 `commitTimeout` 内没有返回时（`tx.go:176-178`），它可能其实成功了：这时答 503 而行可能已存下报告，下一次提交读到报告、答 `wiki.upgrade_state`，不出错，只是 warn 那一行的"行留着"不一定成立（审查 A 的未核实的怀疑，不另处理）。
7. 第二个事务提交之后：记一行 info（3.16，带 `outcome`，与第 5 步的判定的结果无关）；答第 5 步定下的答复：200 `Upgrade`（MCP 答 `{state, submitted_at}`），或判定的拒绝，或领域的 409、503。

`UpgradeRows`（`upgrade_ports.go:110-116`）加：

```go
	// Submit sets the upgrade id reviewing, submission submissionID by by
	// at at: from upgrading, or from reviewing without a report submitted
	// at takenOver or before (M11 design 4.8 step 1); false for no row so.
	Submit(ctx context.Context, id, submissionID, by uuid.UUID, at, takenOver time.Time) (bool, error)
	// KeepReport keeps report as the submission submissionID's, reviewing
	// without one; Unsubmit sets it upgrading again, its submission
	// cleared. False for no row so.
	KeepReport(ctx context.Context, id, submissionID uuid.UUID, report domain.UpgradeReport, by uuid.UUID, at time.Time) (bool, error)
	Unsubmit(ctx context.Context, id, submissionID, by uuid.UUID, at time.Time) (bool, error)
```

```sql
-- name: SubmitUpgrade :execrows
-- An upgrade's submission's first transaction (M11 design 4.8 step 1), under the notebook's row lock: reviewing, a new
-- submission, who and when; from upgrading, or from reviewing without a report whose submission was taken over.
UPDATE notebook_upgrades
SET state = 'reviewing', submission_id = sqlc.arg(submission_id), submitted_by_id = sqlc.arg(by)::uuid,
    submitted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
    AND (state = 'upgrading' OR state = 'reviewing' AND report IS NULL AND submitted_at <= sqlc.arg(taken_over)::timestamptz);

-- name: KeepUpgradeReport :execrows
-- A submission's report, as its check passed (M11 design 4.8 step 4), under the notebook's row lock.
UPDATE notebook_upgrades
SET report = sqlc.arg(report)::jsonb, updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND submission_id = sqlc.arg(submission_id) AND state = 'reviewing' AND report IS NULL
    AND deleted_at IS NULL;

-- name: UnsubmitUpgrade :execrows
-- A submission whose check did not pass, or failed (M11 design 4.8 step 4): upgrading again, its submission cleared.
UPDATE notebook_upgrades
SET state = 'upgrading', submission_id = NULL, submitted_by_id = NULL, submitted_at = NULL,
    updated_by_id = sqlc.arg(by)::uuid, updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND submission_id = sqlc.arg(submission_id) AND state = 'reviewing' AND report IS NULL
    AND deleted_at IS NULL;
```

00057 的检查都过得了：`upgrading` 的提交三列为空、报告为空（`submission_state_check`、`report_check`）；`reviewing` 有提交（`submission_state_check`）。`CurrentUpgrade`（`upgrades.sql:10-17`）多选 `submission_id, report`，注释的"Its report is P2's"去掉；适配器把 `report` 解成 `domain.UpgradeReport`（`wiki/adapter/postgres/report.go` 的私有的结构，成员名与契约的 `UpgradeReport` 相同，3.13；存与读是同一个编码，`TestTheReportRoundTrips`）。`domain.Upgrade`（`wiki/domain/upgrade.go`）加 `SubmissionID *uuid.UUID`、`Report *UpgradeReport`。

**错误**（`wiki/domain/upgrade_errors.go`）：

```go
	// ErrUpgradeNotReady: a submission's check did not pass, the upgrade is
	// upgrading again (M11 design 4.8); UpgradeNotReady names the gates.
	ErrUpgradeNotReady = shared.NewError(shared.KindConflict, "wiki.upgrade_not_ready",
		"The upgrade does not pass its checks: it is upgrading again. Check it, then submit it again.")
	// ErrUpgradeSubmitting: wiki.upgrade_state while the upgrade's previous
	// submission is still checked (M11 design 4.8 step 1, A2-M2).
	ErrUpgradeSubmitting = shared.NewError(shared.KindConflict, "wiki.upgrade_state",
		"The previous submission is still being checked: check again in a moment.")

// UpgradeNotReady is ErrUpgradeNotReady naming the gates that did not pass.
func UpgradeNotReady(failed []UpgradeGateKey) error
```

`detail` 写"These gates did not pass: links, preserved. The upgrade is upgrading again: check it, then submit it again."（只有键，13.1 第 10 条的精神：不带标题与路径）。

**测试**（`wiki/app/submit_upgrade_test.go`，假的端口、事务管理器与授权；假的时钟以 `time.Now()` 起算）：`TestSubmitUpgradeAnswersInOrder`（404、403、`upgrade_state`（没有、回滚中、有报告）、"正在计算"、接管（恰在 `submitted_at + SubmitTimeout` 时接管、早一微秒答"正在计算"）、503、`stale_index`、`not_ready`，每格断言写了什么、发了几次 `mode`）；`TestTheSubmissionDetaches`（请求的 ctx 在计算中取消，第二个事务照样写下报告；期限是 `min(T + SubmitTimeout, 请求的期限)`：请求的期限较晚时 worker 看到的 ctx 的期限恰是 `T + SubmitTimeout`，较早时恰是请求的期限；`T` 是截到微秒的）；`TestTheSecondTransactionWritesOnlyItsOwn`（读到别的 `SubmissionID`、`rolling_back`、已有报告，各什么也不写、答 409）；`TestTheSubmissionsDeadline`（第 6 步：`WithinTx` 交回包着期限的错误而 `detached` 到期，请求的 ctx 活着与已到期两格都答 503 并记 warn；晚到的 `COMMIT`：`fn` 在期限之前返回、`COMMIT` 在期限之后返回 nil，假的授权在它的 ctx 到期时答错误，答 200，钉住判定在 `COMMIT` 之前；领域的 409 在期限之后照答）；`TestTheSubmitterIsDecidedBeforeTheAnswer`（第 5 步：报告写下了，第二个事务里的判定答 `forbidden`，提交照样、报告留着、info 一行 `outcome = stored`，答 403）。S2 另有：`TestSubmitUpgradeAnswersInOrder` 的"只有 `AGENTS` 不过"一格（变异 o1），修复一轮在它末尾加每格只让一道闸门不过的七格（`agents`、`source_root`、`reserved_titles`、`frontmatter`、`sources`、`links`、`preserved`，各答 `wiki.upgrade_not_ready`、`detail` 恰列出那一道、写下退回的三步；`index` 由 stale 一格盖住，那一格断言 `stale_index` 的整句；审查 P2S2A-M1、P2S2C-N4）；新的 `TestTheSubmissionsSnapshotIsItsOwn`：第一个事务之后、计算的快照之前经 `duringRun` 钩子把行改成 `rolling_back`、别的 `SubmissionID` 或已有报告，各答 `wiki.upgrade_state`，第二个事务什么也不写、`outcome = superseded`，trace 里没有闸门的读（审查 P2S2A-M3）；`TestTheSubmissionsDeadline` 加时钟前进的几格，钉住期限的 503 的秒数（第 274 条）。

### 3.13 S2：契约

`api/modules/wiki.yaml`（三个操作；`api/openapi.yaml` 的路径表加三条，wiki 标签的说明已有"and its upgrade"）。参数、错误答复与块的展开照 `listLintItems`（`:38-85`）与 `abandonUpgrade`（`:163-194`）的写法，下面是缩写：

```yaml
  /api/v0/notebooks/{notebook_id}/upgrade/check:
    parameters:
      - $ref: "#/components/parameters/NotebookID"
    get:
      operationId: checkUpgrade
      tags: [wiki]
      summary: Check the upgrade against an LLM Wiki's contract, for its admins.
      description: >-
        Every gate, the health checks of an LLM Wiki whose source root is the
        one the upgrade proposes, with their counts and first five items,
        and what the notebook holds, from one read snapshot; passed or not,
        200. Its admins can (forbidden). A notebook that does not exist, is
        deleted, or that the caller cannot see is notebook.not_found; one
        without an upgrade upgrading or reviewing is wiki.upgrade_state; an
        exhausted computation or waiting budget is server_busy, with
        Retry-After: 1. These checks occur in that order; the upgrade, read
        again in the check's snapshot once a computation slot is had, may
        still be wiki.upgrade_state then. Checks of one upgrade share a
        computation.
      security: [{ bearer: [] }]
      x-problem-codes: [notebook.not_found, forbidden, wiki.upgrade_state, server_busy]
      responses:
        "200": {description: The check., content: {application/json: {schema: {$ref: "#/components/schemas/UpgradeCheck"}}}}
  /api/v0/notebooks/{notebook_id}/upgrade/check/{rule}:
    parameters:
      - $ref: "#/components/parameters/NotebookID"
      - $ref: "#/components/parameters/Rule"
    get:
      operationId: listUpgradeCheckItems
      tags: [wiki]
      summary: Read more items of a rule of the upgrade's check.
      description: >-
        One page of a rule in the upgrade's check's scope, an LLM Wiki whose
        source root is the one the upgrade proposes, from the current read
        snapshot. A malformed cursor is bad_request; then a notebook that
        does not exist, is deleted, or that the caller cannot see is
        notebook.not_found; a caller not its admin forbidden; an unknown
        rule wiki.lint_rule_not_found; an upgrade not upgrading or reviewing
        wiki.upgrade_state; a cursor bound to another notebook, rule or
        scope, including a lint's, bad_request; a limit outside 1-100
        validation_failed; an exhausted computation or waiting budget
        server_busy, with Retry-After: 1. These checks occur in that order;
        the upgrade, read again in the snapshot once a computation slot is
        had, may still be wiki.upgrade_state then. A cursor is bound to the
        proposed source root, not to the upgrade. The limit is 50 when
        absent. Continue with next_cursor.
      security: [{ bearer: [] }]
      x-problem-codes: [bad_request, notebook.not_found, forbidden, wiki.lint_rule_not_found, wiki.upgrade_state,
        validation_failed, server_busy]
      parameters:
        - $ref: "../common.yaml#/components/parameters/Limit"
        - $ref: "../common.yaml#/components/parameters/Cursor"
      responses:
        "200": {description: A page of the rule., content: {application/json: {schema: {$ref: "#/components/schemas/LintPage"}}}}
  /api/v0/notebooks/{notebook_id}/upgrade/submit:
    parameters:
      - $ref: "#/components/parameters/NotebookID"
    post:
      operationId: submitUpgrade
      tags: [wiki]
      summary: >-
        Submits the upgrade for its admins' review: freezes it for everyone,
        checks it again and keeps the report once every gate passes.
      description: >-
        The upgrade, upgrading, goes to reviewing, its content then frozen
        for everyone, the initiator's agent too; it is checked from a
        snapshot of that moment, apart from the request, and stays reviewing
        with its report kept once every gate passes. Otherwise it is
        upgrading again: wiki.upgrade_stale_index for an old link index,
        wiki.upgrade_not_ready for a gate that did not pass (detail names
        them), or the check's own error. Its admins can (forbidden). A
        notebook that does not exist, is deleted, or that the caller cannot
        see is notebook.not_found; one without an upgrade upgrading is
        wiki.upgrade_state, also while a previous submission is still
        checked (detail says so; a reviewing upgrade without a report may be
        submitted again once its check is surely over); an exhausted
        computation or waiting budget is server_busy. These checks occur in
        that order. Once the check is done, a submission taken over or an
        upgrade abandoned meanwhile is wiki.upgrade_state, before the
        check's own answer; the second step past its deadline, which is no
        later than the request's, is server_busy, its Retry-After the
        seconds until another submission may take it over, at least 1. Its
        admins are asked again before the answer, as the report is kept: a
        caller no longer one is notebook.not_found or forbidden then, the
        report kept all the same. The answer is the upgrade, reviewing,
        with its report.
      security: [{ bearer: [] }]
      x-problem-codes: [notebook.not_found, forbidden, wiki.upgrade_state, server_busy, wiki.upgrade_stale_index,
        wiki.upgrade_not_ready]
      responses:
        "200": {description: The upgrade, reviewing, with its report., content: {application/json: {schema: {$ref: "#/components/schemas/Upgrade"}}}}
```

新的 schema（全部 `additionalProperties: false`、成员全部必填）：

```yaml
    UpgradeCheck:
      description: An upgrade's check, from one read snapshot.
      type: object
      additionalProperties: false
      required: [computed_at, stale_pages, passed, gates, rules, counts]
      properties:
        computed_at: {type: string, format: date-time}
        stale_pages: {description: Pages with an old link index; an administrator can reindex them., type: integer, format: int64, minimum: 0}
        passed: {description: Every gate passed., type: boolean}
        gates: {$ref: "#/components/schemas/UpgradeGates"}
        rules:
          description: The eight health checks of an LLM Wiki, in their fixed order, including zero counts.
          type: array
          minItems: 8
          maxItems: 8
          items: {$ref: "#/components/schemas/LintRuleSummary"}
        counts: {$ref: "#/components/schemas/UpgradeCounts"}
    UpgradeGates:
      type: object
      additionalProperties: false
      required: [agents, source_root, reserved_titles, frontmatter, sources, links, preserved, index]
      properties:
        agents: {description: An AGENTS page at the root whose body is not empty., allOf: [{$ref: "#/components/schemas/UpgradeGate"}]}
        source_root: {description: The proposed source root is a page, not the root's AGENTS., allOf: [{$ref: "#/components/schemas/UpgradeGate"}]}
        reserved_titles: {$ref: "#/components/schemas/UpgradeTitlesGate"}
        frontmatter: {description: Each page outside the source area has frontmatter that parses, with type and summary but at the source root and the root's AGENTS., allOf: [{$ref: "#/components/schemas/UpgradeGate"}]}
        sources: {description: Each sources reference leads to a node of the source area., allOf: [{$ref: "#/components/schemas/UpgradeGate"}]}
        links: {$ref: "#/components/schemas/UpgradeLinksGate"}
        preserved: {$ref: "#/components/schemas/UpgradePreservedGate"}
        index: {description: The link index is fresh (stale_pages is 0)., allOf: [{$ref: "#/components/schemas/UpgradeGate"}]}
    UpgradeGate:
      type: object
      additionalProperties: false
      required: [passed]
      properties: {passed: {type: boolean}}
    UpgradeTitlesGate:
      description: No node at the root holds a title an LLM Wiki keeps, index, log, index.md or log.md; those that do.
      type: object
      additionalProperties: false
      required: [passed, node_ids]
      properties:
        passed: {type: boolean}
        node_ids: {type: array, maxItems: 4, items: {type: string, format: uuid}}
    UpgradeLinksGate:
      description: >-
        No page the upgrade started with, outside the source area now, leads to more distinct pages that do not
        exist than as it started; how many do, and the first five, by id.
      type: object
      additionalProperties: false
      required: [passed, pages_worse, node_ids]
      properties:
        passed: {type: boolean}
        pages_worse: {type: integer, minimum: 0}
        node_ids: {type: array, maxItems: 5, items: {type: string, format: uuid}}
    UpgradePreservedGate:
      description: >-
        Each node the upgrade started with is alive, each page's content the one the guardrails let by; the nodes
        and pages, those kept, and the first five not, by id.
      type: object
      additionalProperties: false
      required: [passed, nodes, nodes_kept, pages, pages_kept, node_ids]
      properties:
        passed: {type: boolean}
        nodes: {type: integer, minimum: 0}
        nodes_kept: {type: integer, minimum: 0}
        pages: {type: integer, minimum: 0}
        pages_kept: {type: integer, minimum: 0}
        node_ids: {type: array, maxItems: 5, items: {type: string, format: uuid}}
    UpgradeCounts:
      type: object
      additionalProperties: false
      required: [sources, moved_into_sources, wiki_pages, new_pages, changesets, wanted_before, wanted_now]
      properties:
        sources: {description: The nodes in the source area., type: integer, minimum: 0}
        moved_into_sources: {description: The nodes the upgrade started with moved into the source area since., type: integer, minimum: 0}
        wiki_pages: {description: The pages outside the source area, but the source root and the root's AGENTS., type: integer, minimum: 0}
        new_pages: {description: The pages written since the upgrade started, the ones its start wrote included., type: integer, minimum: 0}
        changesets: {description: The changes the upgrade wrote., type: integer, minimum: 0}
        wanted_before: {$ref: "#/components/schemas/UpgradeWanted"}
        wanted_now: {$ref: "#/components/schemas/UpgradeWanted"}
    UpgradeWanted:
      description: The distinct pages links lead to that do not exist yet, as the wanted health check counts them.
      type: object
      additionalProperties: false
      required: [count, count_capped]
      properties:
        count: {$ref: "#/components/schemas/LintCount"}
        count_capped: {type: boolean}
    UpgradeReport:
      description: The check a submission kept, its rules' first five items without cursors.
      type: object
      additionalProperties: false
      required: [computed_at, stale_pages, passed, gates, rules, counts]
      properties:
        computed_at: {type: string, format: date-time}
        stale_pages: {type: integer, format: int64, minimum: 0}
        passed: {type: boolean}
        gates: {$ref: "#/components/schemas/UpgradeGates"}
        rules: {type: array, minItems: 8, maxItems: 8, items: {$ref: "#/components/schemas/UpgradeReportRule"}}
        counts: {$ref: "#/components/schemas/UpgradeCounts"}
    UpgradeReportRule:
      type: object
      additionalProperties: false
      required: [rule, count, count_capped, items]
      properties:
        rule: {$ref: "#/components/schemas/LintRule"}
        count: {$ref: "#/components/schemas/LintCount"}
        count_capped: {type: boolean}
        items: {type: array, maxItems: 5, items: {$ref: "#/components/schemas/UpgradeReportItem"}}
    UpgradeReportItem:
      oneOf:
        - $ref: "#/components/schemas/UpgradeReportWantedItem"
        # … the eight, as LintItem's
      discriminator:
        propertyName: rule
        mapping:
          wanted: "#/components/schemas/UpgradeReportWantedItem"
          # … the eight
    UpgradeReportWantedItem:
      type: object
      additionalProperties: false
      required: [rule, item_key, target, target_cut, pages]
      properties: # LintWantedItem's (:387-409) but cursor
```

八种 `UpgradeReport<X>Item` 各是对应的 `Lint<X>Item`（`:387-549`）去掉 `cursor` 一个成员与它在 `required` 里的一项，别的照抄；契约测试 `wiki/adapter/http` 的 `TestTheReportItemsAreTheLintItemsButTheirCursor` 读 `wiki.yaml`，逐对核对（以后改一边漏了另一边时失败）。

`Upgrade`（`:780-857`）加必填的 `report`：

```yaml
        report:
          description: >-
            The report its submission kept once its check passed, kept after, through an abandonment too; null
            before, and while a submission is still checked.
          oneOf:
            - $ref: "#/components/schemas/UpgradeReport"
            - type: "null"
```

`events.yaml` 的 `mode` 的说明（`:48-51`）已写"submission"，不改。`common.yaml` 不改。

**HTTP 适配器**：`handler.go:21-26` 的 `UpgradeUseCases` 加 `CheckUpgrade`、`SubmitUpgrade`；`check.go` 的 `CheckUpgrade`（`UpgradeCheck` 的投影，规则照 `lintSummary`，`projection.go:21`）、`ListUpgradeCheckItems`（照 `ListLintItems`，`handler.go:65-87`）、`SubmitUpgrade`；`upgradeOf`（`upgrade.go:55`）投影 `report`（条目照 `lintItem`，`projection.go:80`，去掉游标；REST 的条目本来就没有 `path`，它只在 MCP 的投影里，`ReportOf` 也去掉它：存下的路径会随改名、移动与回滚变旧，带上它最坏的报告会过 64 KiB，S2 的实施，3.10、3.20）。每个声明的码在 `wiki/adapter/http` 的测试里经 `apitest.CheckResponse` 答出（假的用例），一份存下的报告（八条规则各五项、三道闸门带节点）经它核对（00 号文档第 9 节"契约"）。

### 3.14 S2：MCP 的两个工具、说明与措辞

**注册**（`wiki/adapter/mcp/tools.go:18-27`）：`Tools(lint LintUseCase, upgrades UpgradeUseCase)` 加

```go
		mcpserver.NewTool(mcpserver.Spec{Name: "upgrade_verify", Title: "Check the notebook's upgrade",
			Description: upgradeVerifyDescription, Annotations: mcpserver.Annotations{ReadOnly: true},
			Entry: mcpserver.BothEntries, Notebook: true}, t.verify),
		mcpserver.NewTool(mcpserver.Spec{Name: "upgrade_submit", Title: "Submit the notebook's upgrade",
			Description: upgradeSubmitDescription, Annotations: mcpserver.Annotations{},
			Entry: mcpserver.BothEntries, Notebook: true}, t.submit),
```

`UpgradeUseCase` 是 `CheckUpgrade(ctx, id, LintQuery) (app.UpgradeChecked, error)` 与 `SubmitUpgrade(ctx, id) (app.UpgradeView, error)`，`*wiki.Module` 实现它（`wiki/tools.go:9` 改为 `mcpadapter.Tools(m, m)`）。`t.verify` 收 `lintIn`（`lint.go:11-15`，同样的三个参数与说明），有 `Items` 时交 `fitLint`（`lint_projection.go:89`），有 `Check` 时交 `fitUpgradeCheck`；`t.submit` 收 `struct{}`，答 `{"state": "reviewing", "submitted_at": …}`。

**`destructiveHint`**（`platform/httpserver/mcpserver/tool.go:150-156`）：今天只在 `Destructive` 为真时写；MCP 的规范里没写的 `destructiveHint` 读作真，`upgrade_submit` 不毁掉任何东西（00 号文档 4.13 写 `destructiveHint: false`）。改为不是只读的工具都写它（`if !a.ReadOnly { hints.DestructiveHint = new(a.Destructive) }`；读的工具照旧不写，协议说它只在 `readOnlyHint` 为假时有意义）；今天不是只读的工具（`write`、`edit`、`move`、`delete`、`batch`）都已是 `true`，`tools.json` 只有 `upgrade_submit` 的两项写 `false`（2.1 第 15 条）。`Annotations` 的注释（`tool.go:28-29`"destructiveHint only for a write"）与 `TestTheAnnotationsAreTheHints`（`mcpserver/tools_test.go:420-432`）的说明改为"written for every write, false for one that destroys nothing"，后者加一格 `{Annotations{}, `{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`}`（审查 P2C-N1：只有两格时改了 `annotationsOf` 它照样过）。

**`upgrade_verify` 的注解**：只设 `ReadOnly`，`idempotentHint` 是 `false`，同全部只读的工具（M9 起的写法，`docs/v0.1/M9-mcp/01-P1-platform-reads.md:2181`；`tools.json` 的 14 个只读工具都是 `false`）；00 号文档 4.13 写的 `idempotentHint: true` 照 2.1 第 20 条改。

**检查的答复**（`wiki/adapter/mcp/upgrade_projection.go`）：

```json
{
  "computed_at": "…", "stale_pages": 0, "passed": false,
  "gates": {
    "agents": {"passed": true}, "source_root": {"passed": true},
    "reserved_titles": {"passed": false, "paths": ["index.md"]},
    "frontmatter": {"passed": false}, "sources": {"passed": true},
    "links": {"passed": false, "pages_worse": 1, "paths": ["Notes/Plan.md"]},
    "preserved": {"passed": true, "nodes": 312, "nodes_kept": 312, "pages": 300, "pages_kept": 300, "paths": []},
    "index": {"passed": true}
  },
  "rules": [ /* lint's summary's rules, as lint's tool answers them (lint_projection.go:26-32) */ ],
  "counts": {"sources": 40, "moved_into_sources": 12, "wiki_pages": 260, "new_pages": 3, "changesets": 9,
             "wanted_before": {"count": 4, "count_capped": false}, "wanted_now": {"count": 3, "count_capped": false}}
}
```

`paths` 照闸门的 `node_ids` 的次序，不在活的树里的写 `null`（00 号文档 4.7"MCP 的投影"）；规则的条目带路径，照 `lint` 的（`lint_projection.go:42-78`）。`Fit`（`mcpserver/answer.go:127`）：第一段对"每条规则留几项"`k = 5 … 0` 找最大的放得下的（同 `lintSummaryCandidate`，`lint_projection.go:113`、`:137-141`，`next_cursor` 是第 `k` 项之后的游标），`k = 0` 也放不下时第二段对"闸门的路径各留几项"再找（计数与 `passed` 不截）；两段都放不下答 `errNothingFits` 的缺陷（三道闸门至多 14 个路径，每个至多 `MaxDepth` 段的名称，实际到不了）。测试 `wiki/adapter/mcp` 的 `TestTheCheckFits`（一个只有规则的条目放不下的、一个连闸门的路径也要截的，各断言截掉的是哪些、计数不变）。

**`tools.json`**（两个入口的末尾，`pending_sources` 之后；笔记本入口 18 个、账户入口 20 个）。笔记本入口的两项：

```json
    {
      "name": "upgrade_verify",
      "title": "Check the notebook's upgrade",
      "description": "Check this notebook's upgrade to an LLM Wiki, from a snapshot for this call, against an LLM Wiki's contract with the source root the upgrade proposes. Omit rule for every gate, each health check's count and first items, and the counts; limit and cursor require rule. Set rule to read its items, then continue with next_cursor. passed is true once every gate passes: then upgrade_submit. The notebook's admins only, while it is upgrading or reviewing.",
      "inputSchema": { /* lint's: rule, limit, cursor (tools.json:511-526) */ },
      "annotations": {"idempotentHint": false, "openWorldHint": false, "readOnlyHint": true}
    },
    {
      "name": "upgrade_submit",
      "title": "Submit the notebook's upgrade",
      "description": "Submit this notebook's upgrade for its admins' review: it freezes the notebook for everyone, your writes too, and checks it again from that moment; once every gate passes it stays in review with its report kept, an admin approves or rejects it in the web. Otherwise it is upgrading again, refused with wiki.upgrade_not_ready (detail names the gates) or wiki.upgrade_stale_index. Run upgrade_verify first. wiki.upgrade_state whose detail says a previous submission is still being checked: wait, then upgrade_verify.",
      "inputSchema": {"type": "object", "additionalProperties": false},
      "annotations": {"destructiveHint": false, "idempotentHint": false, "openWorldHint": false, "readOnlyHint": false}
    }
```

笔记本入口的 `upgrade_submit` 的 `inputSchema` 没有 `properties`：SDK 对空结构体就这样编码，账户入口的 `notebooks` 自 M9 就是这样（S2 的实施；初稿写 `"properties": {}`）。账户入口的两项另有 `notebook`（第一个属性、`required: ["notebook"]`，`mcpserver/schema.go:49-56` 自动加）。描述是 Go 的 `upgradeVerifyDescription`、`upgradeSubmitDescription`（`wiki/adapter/mcp/tools.go:31` 旁），`tools.json` 由 `TestTheToolListsAreTheContract`（`bootstrap/mcp_tools_test.go:156-160`）逐项对照。

**内置的说明**（`page/adapter/mcp/instructions.go:51-52` 之后，`pending_sources` 一行下）：

```
- upgrade_verify: an upgrading notebook's LLM Wiki checks, or one rule's items with next_cursor.
- upgrade_submit: submit the upgrade for an admin's review once every check passes.
```

两行原文 181 字节，SDK 的编码下 183（00 号文档 4.13，审查 C-N2 实测）；笔记本入口约 3093 → 3276、账户入口约 3163 → 3346，距 `TestTheBuiltInInstructionsAreShort` 的界 3796（`instructions_test.go:68-70`）各剩约 520、450。S2 照 SDK 实测写进记录。

**"server update"**（00 号文档 4.13）：`page/adapter/mcp/tools.go:271-272` 与 `:281-282` 的"an upgrade"改为"a server update"，`linking/adapter/mcp/tools.go:63-64` 的"after an upgrade"改为"after a server update"，`tools.json` 的六处（`grep` 的 `:205`、`:782`，`search` 的 `:409`、`:1048`，`index` 的 `:242`、`:823`）随之改；`api/modules/events.yaml:35` 的"after an upgrade"改为"after a server update"（`make gen` 之后生成的类型的注释随之变）。

**测试的计数**：`TestTheWikiToolsContract`（`bootstrap/mcp_tools_test.go:344-409`）的 16、18（`:347-353`）改为 18、20，名字（`:355`）照旧只核对 `lint`、`pending_sources`，另加一段核对两个新工具的属性、`required` 与注解（`upgrade_submit` 在笔记本入口没有属性、在账户入口只有 `notebook`）；`wiki/adapter/mcp/tools_test.go:35` 的 2 改为 4，`testdata/tools.json` 加两项；`TestTheBuiltInInstructionsNameEveryTool`（`:193-205`）照旧要每个工具有一行。工具表的长度另驱动两处，同一个提交改：第三份的 `TestToolStringsDoNotAnswerInternalErrors`、`TestToolStringsCatchASchemaRefusal` 要每个工具一组能到用例的参数（`bootstrap/mcp_reads_test.go` 的 `validToolArguments` 加 `upgrade_verify`、`upgrade_submit`），e2e 的 LW8 断言两个入口的工具数（16、18 → 18、20）（S2 的实施，第一轮门禁才查出）。

### 3.15 S2：矩阵的行与码

**REST**（`TestUpgradePermissionMatrix`，`bootstrap/upgrade_permission_matrix_test.go:32` 的 `upgradeMatrixRows()` 加；`prepare` 照 `upgradedBy(state)`，每格一个库的副本加一个应用）：

| 操作 | 变体 | 管理员 | 看得到而不是管理员 | 看不到 |
|---|---|---|---|---|
| `checkUpgrade` | `upgrading`、`reviewing` | 200 | 403 `forbidden` | 404 `notebook.not_found` |
| `listUpgradeCheckItems`（`wanted`） | `upgrading` | 200 | 403 | 404 |
| `submitUpgrade` | `upgrading` | 409 `wiki.upgrade_not_ready`（lab 的笔记本过不了闸门，2.1 第 12 条） | 403 | 404 |

管理员的 200 一格以 `check` 断言答复的形状（八个闸门、八条规则）；`submitUpgrade` 的 409 一格之后断言升级回到 `upgrading`、没有提交的列（`after`）。覆盖的检查 `coveredMatrixRows()`（`bootstrap/permission_matrix_coverage_test.go:158`）已读 `upgradeMatrixRows()`，不改。

**MCP**（新的 `TestUpgradeToolsMatrix`，`bootstrap/upgrade_tools_matrix_test.go`，第八份）：`upgradeMCPRow{mcpRow; prepare}` 与 `upgradeMCPRows()`；`TestThePermissionMatrixCoversEveryTool`（`bootstrap/permission_matrix_mcp_test.go:444-448`）改读 `coveredMCPRows()`（`mcpRows()` 加 `upgradeMCPRows()` 的 `mcpRow`）。跑法照 `TestThePermissionMatrixOverMCP`（`:559-646`：列 × 两个入口，笔记本入口里看不到的列是 `notebook.not_found`，写的行每格一个库的副本），只多一步 `prepare`；读的一行也每格一个库的副本（`prepare` 以 SQL 设这一格的笔记本，并行的格共用一个库会互相踩；S2 的实施），每行照 `r.entries` 只在它的入口跑（同 `:610`；审查 P2S2C-N6，修复一轮）。

| 工具 | 变体 | 管理员 | 看得到而不是管理员 | 看不到 |
|---|---|---|---|---|
| `upgrade_verify`（不带 `rule`） | `upgrading` | 成功（`check` 断言八个闸门与路径） | `forbidden` | `notebook.not_found` |
| `upgrade_submit` | `upgrading` | `wiki.upgrade_not_ready` | `forbidden` | `notebook.not_found` |

**码**：`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:93-123`）加两行（00 号文档第 5 节的 MCP 表）：

```go
		"upgrade_verify": {"bad_request", "validation_failed", "notebook.not_found", "forbidden",
			"wiki.lint_rule_not_found", "wiki.upgrade_state", "server_busy"},
		"upgrade_submit": {"bad_request", "notebook.not_found", "forbidden", "wiki.upgrade_state", "server_busy",
			"wiki.upgrade_stale_index", "wiki.upgrade_not_ready"},
```

两行整行进 `upgradeToolCodes()`（P1 设计第 9 节：第五份的应用种不出升级）；`TestUpgradeToolsErrorsCarryTheirCodes` 在两个入口答出每一个：`bad_request`（`cursor` 解不开；`upgrade_submit` 带一个不认识的参数）、`validation_failed`（`limit` 而没有 `rule`）、`notebook.not_found`（看不到的笔记本，账户入口）、`forbidden`（编辑者的 PAT）、`wiki.lint_rule_not_found`、`wiki.upgrade_state`（没有升级的笔记本）、`server_busy`（`heldRules`（`bootstrap/upgrade_interleavings_test.go:48`，`TestUpgradeInterleavings` 第 17 条的同一个钩子）占住唯一的名额：组合根的测试配置 `MaxLintRuns` 是 1，`bootstrap/wiki_lint_runtime_test.go:69`；等待的预算是 `LintTimeout` 2 s，`lint_runs.go:66-77`）、`wiki.upgrade_stale_index`（以 SQL 把一页的索引的版本记旧）、`wiki.upgrade_not_ready`（lab 的笔记本）。`wiki.*` 的码由 REST 声明，不进 `mcpOwnCodes()`。

**网页的码**：`wiki.upgrade_not_ready` 的两种语言的文字（4.2）。

### 3.16 S2：日志

13.1 第 10 条：只记事实。每条一个测试，只读自己子测试开始之后的行。

- 提交：info "upgrade submitted"，在第二个事务提交之后、答复之前，与第二个事务里交付之前的判定的结果无关（S3A-N1 的教训；审查 P2A-I1：初稿把判定放在日志之前，判定一失败、存下了报告的提交就没有这一行）：`workspace_id`、`notebook_id`、`upgrade_id`、`submission_id`、`user_id`、`outcome`（`stored`、`stale`、`not_ready`、`failed`、`superseded`）、`passed`、`failed_gates`（键的数组，没有时为空）、`stale_pages`、`elapsed_ms`（从第一个事务读时钟算起）。warn "upgrade submission timed out"（3.12 第 6 步）：同样的 id、`elapsed_ms`、`timeout_ms`（`detached` 的期限减第一个事务读的时刻，请求的期限较早时是它）。
- 检查与条目：`Runs` 的"lint computed"/"lint stopped"一行，`query_kind` 是 `upgrade_check`、`upgrade_items` 或 `upgrade_submit`，带 `upgrade_scope`（3.8）。
- 缺陷：error "upgrade source root is no live page"（3.11）；error "upgrade baseline rows not its counts"（`nodes`、`pages` 现读的与 wiki 行的四个数）。两行经 `ErrorContext` 记：wiki 的 `Logger`（`wiki/app/lint_run_types.go:68`）加这一个方法（`*slog.Logger` 本来就有；S2 的实施）。

### 3.17 每个操作改后的码

| 操作 | `x-problem-codes`（次序） | Step |
|---|---|---|
| `checkUpgrade`（新） | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`server_busy` | S2 |
| `listUpgradeCheckItems`（新） | `bad_request`、`notebook.not_found`、`forbidden`、`wiki.lint_rule_not_found`、`wiki.upgrade_state`、`validation_failed`、`server_busy`（绑定的 400 在状态之后，说明写明） | S2 |
| `submitUpgrade`（新） | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`server_busy`、`wiki.upgrade_stale_index`、`wiki.upgrade_not_ready` | S2 |
| `getUpgrade` | 不变（`notebook.not_found`、`forbidden`、`wiki.upgrade_not_found`）；答复多 `report` | S2 |
| MCP `write`、`edit` | 加 `page.upgrade_immutable` | S1 |
| MCP `move` | 加 `page.upgrade_keep_links` | S1 |
| MCP `batch` | 加 `page.upgrade_immutable`、`page.upgrade_batch_rewrite`、`page.upgrade_keep_links` | S1 |
| MCP `upgrade_verify`、`upgrade_submit`（新） | 3.15 | S2 |

新的码：`page.upgrade_immutable`、`page.upgrade_batch_rewrite`、`page.upgrade_keep_links`（S1，只在 MCP，进 `mcpOwnCodes()`）；`wiki.upgrade_not_ready`（S2，`submitUpgrade` 与 `upgrade_submit`；P3/S1 的批准再用它）。

### 3.18 每个新检查的反向对照

每个新检查都有一个去掉它就失败的测试（全表在 7.3）：

- 不改写链接的移动与改名：`TestUpgradeGuardrails` 的 g5、g6、g13；`TestTheFreezeGuard` 的两种。
- 正文的比较：形状表 `TestTheUpgradeBodyShapes`；g2、g3、g15 的后一半；只改 frontmatter 放过（g1、g14、g15 的前一半）与 `Rewrite` 放过（g4、g10）各是反向的钉。
- 账本的写核对写之前的哈希：g17、g18；`TestAcceptUpgradeContent` 的不等一格；`TestTheUnitKeepsTheLedger` 的 `before`。
- 升级中盲写计进读的界：`TestTheReadsBoundCountsBlindWritesWhileUpgrading`；单元看到的状态与计划时的不同就重新计划：`TestTheBatchPlansAgainOnceTheUpgradeStarted`；g22。
- `Reached`：`TestThePlanTellsAPageAMoveReached` 的表；g9 与 g11（码不同）、g19–g21（第 265 条的三种）、g23–g25（第 268 条）；字节不变、被到达的写也过守卫：`TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds`、g23 的后一种、g25；不是基线页照常做完：g26；照批前的链接判：`TestTheBatchJudgesAPageByItsLinksBeforeIt`、g27；标在 `Last`：g9 的 `op`、`TestTheBatchMarksABodysRefusalAtTheLast`。
- 账本：g4、g10 的账本；`TestTheUnitKeepsTheLedger`。
- 闸门：`TestJudgeUpgrade` 的每格（闸门 7 的附件三格经 `withAttachment`，在修复一轮加，审查 P2S2A-M2）；整个程序的 `TestUpgradeGates`（7.1）；提交只看 `Passed`：`TestSubmitUpgradeAnswersInOrder` 的每格一道闸门不过的表（修复一轮，审查 P2S2A-M1）。
- 快照里的核对：交错第 8 条；`TestTheCheckRefusesAnotherUpgradesSnapshot`；提交的那一份：`TestTheSubmissionsSnapshotIsItsOwn`（修复一轮，审查 P2S2A-M3）。
- 提交的两个事务：交错第 5、6、7、15 条；`TestTheSecondTransactionWritesOnlyItsOwn`。
- 脱离请求、期限取较早的：`TestUpgradeSubmit` 的"断开"与期限两格；`TestTheSubmissionDetaches`。
- 键：`TestTheUpgradeKeysJoinOnlyTheirOwn`；交错第 6 条。
- 交付前的判定：`TestTheCheckAuthorizesBeforeItsAnswer`、`TestTheSubmitterIsDecidedBeforeTheAnswer`；提交的判定在 `COMMIT` 之前：`TestTheSubmissionsDeadline` 的晚到的 `COMMIT`。
- 报告的大小：`TestTheWorstReportFits`。
- 检查与报告的投影与编码（REST 的检查、规则页、报告与带报告的视图，MCP 的检查，存与读，`ReportOf`）不把一个成员写成另一个或常量：各自的测试的夹具每个标量互不相同、闸门排成真假交错的四行，整个答复与写死的期望比（修复第三、四轮，审查 P2S2C-M4–M6；7.1）；组合根把 page 的 `Preserved` 转成 wiki 的那一行：`TestUpgradeGates` 删一个基线附件的一格，四个数里 `nodes_kept` 与别的都不同（修复第五轮，审查 P2S2C-M7）。
- 闸门的读的计划：`TestTheUpgradeGateReadsPlansAtAnyStatistics`（7.1；`nodes` 只经主键在修复一轮另判，判法本身由 `TestTheGateReadsReadANodeByItsKey` 钉住，审查 P2S2B-M1）；`changesets` 只经 `changesets_upgrade_id_idx`：它的"从未分析"一格与 `TestTheRollbackGuardJudgesThePlans` 的合成的 `BitmapAnd` 一格。
- 自己的提交在算时页面照实显示、再检查恰读一次；检查的视图从 `data` 推出，检查在飞时卸下又装回也显示结果；同一页的链接再点一次是新的一次进入：4.3 的 vitest（"检查在飞时自己提交"两种次序各一格；视图存在不随一节卸下丢掉的地方：7.3 第 85 行的 c1 由"检查在飞时"三格抓到，修复一轮让三格看得见卡的第一页，审查 P2S3B-I1；设计原写的那一种变异（视图在 `onSuccess` 里建）在 SWR 2.5.1 与 React 18 下等价，3.21）；检查与"更多"答 403 重读笔记本列表：4.3 的 vitest 末条（审查 P2S3A-M1 = C-M1，修复一轮）。

### 3.19 S1 的实施对设计的修正

S1 的审查与两轮修复核对见 [S1 的审查记录](reviews/P2-S1-guardrails-review.md)，结果在 10.1。与上文最初的设计不同之处已改在各节里（第 0 节，2.1 第 2 条与 2.2、2.3，3.1–3.4、3.6、3.18，第 5 节，6.1、6.2，7.1、7.3、7.5、7.6，第 8、9 节）。S1 新写的代码多写文件与名字；写了行号的，是修复第二轮之后的头 `198b0cd5` 的。

- **S1，实施时定下的**（实施者的 14 条偏离，括号里是它的号。三位审查者各在自己的范围里判过：A 判第 3、5、6、7、12、13 条，B 判第 1、2、4、9、11、13 条，C 判第 3、9、12、13 条，都对；第 8、10 条没有人提出异议，B 核过五种形状的算术；第 14 条由审查 P2S1C-N1 改回）：
  - 7.1：单元守卫的期限取 14 秒，不是"守卫配置下的请求期限 6 s"。批的 bench 用生产的 15 秒请求（`TestABatchAtItsRewriteBoundTakesItsTime` 在同一个 bench 上，自己的界 11 秒），14 秒是在这个请求里留 1 秒（决定 153、243）。最大的单元（形状 a）本机 1.7 秒、7763 上 3.2 秒。回归的界另取本机两轮较大的中位数的三倍、向上取到 100 毫秒：a 5.2 s、b 1.1 s、c 200 ms、d 100 ms、e 100 ms（1）。
  - 7.1：形状 b 照字面（100 页各约 140 KB 的 frontmatter 编辑），最终内容合计约 14 MiB，过 `MaxUnitBytes`（8 MiB），`CheckUnit` 答 `page.batch_too_large`，单元不会跑。改为每页一行约 62 KB 的 frontmatter 由编辑删掉：`old_string` 合计约 6 MiB，在 `MaxBatchStringBytes` 之内；头合计约 13.8 MiB，最终约 8 MiB。这是计划整读一次、守卫再读一次的最大的编辑单元（2）。B 核过算术：头 14,502,300 ≤ 14,680,064，最终 8,301,900 ≤ 8,388,608，删掉的 6,200,400 ≤ 6,291,456。
  - 3.3：`CheckReads` 一遍按 `p.order`（第一个正文操作的次序）计，盲写插在其中；过界时标的页与已有的注释"at the page that passes it"一致。设计的草稿是先计整读的、再计盲写的，会把一个排在盲写之后、跨过界的编辑标到更早的盲写上（3）。
  - 7.1、7.6：读的守卫是三种统计 × 两种计划。第四种（`force_generic_plan`）就是"通用计划"那一列；每格既判计划又计时（4）。
  - 3.6：g22 的三页以 SQL 种下（`seedPagesSQL`）。三页各约 5 MiB 的 frontmatter 经 REST 在竞态下要解析约 15 MiB，贴着测试 4 秒的请求期限（05-lessons 第 1 节"大的种子用 SQL"）（5）。
  - 6.1：`StepContent.Folded` 在 4 号提交加，它在 4 号才有写者与读者（6）。
  - 3.5：`AcceptUpgradeContent` 的两个 CTE 带表的别名（`b` 里叫 `r`，`u` 里叫 `w`）。两个 CTE 都读写 `upgrade_nodes`，不带别名时 sqlc 报 `upgrade_id` 有歧义；语义不变（7）。
  - 7.1：形状 c 是一个 `edit` 操作的 `batch`，在批的 bench 上与单独的 `edit` 走同一个单元（8）。批的 bench 给 `/mcp` 以 wire 的请求体上限（`mcpBodyLimit`）；bench 原来用默认的 1 MiB，形状 b 的约 6 MiB 的 `old_string` 会答 413（9）。
  - 7.1：单元守卫的种子：b–e 的页以 SQL 写，每轮之间以 SQL 写回（账本一起）；a 与 c 由下一轮自己写回；e 的 100 页以一条 `generate_series` 写。不另逐页核对，只断言账本与最后一个变更集带升级（10）。
  - 7.5：读的守卫本机约 17 秒（种子 12.7 秒），不是"约 3 秒"。照决定 240 种了两次升级各 10 万行的基线，另有九本各 5 万页的笔记本与它们的升级（11）。
  - 3.2、7.3 第 3 条：多一个 `TestTheUnitStepCarriesKeepLinks`（`page/app`），钉住单元的每一步带 `keepLinks`，第二个改名为假（清掉的那一步）（12）。
  - 第 5 节：UP4 的 Notes 以从根起的路径链向 Plan（`[the plan](/Plan.md)`），改名的一半照旧用 `[[Old]]`；文件夹是先写的一页 `Work.md`（MCP 的 `move` 不建父页）（13）。只写名字的链接（`Plan.md`、`[[Plan]]`）在 Plan 移进文件夹之后照样解析到它（`linking/domain/rewrite.go` 的 `markdownTarget`），系统不改写。设计没有写错：00 号文档 4.6、决定 190 都没说"移动改写每一条链接"。但第 5 节照字面会写出按名字链接的 Notes，所以第 5 节与第 9 节的 playbook 各补一句（审查 P2S1C-N2）。
  - 第 5 节：实施读版本号用数据库（14）。审查 P2S1C-N1 照 00 号文档第 9 节"值来自答复"改为从 `createPage` 的答复取（`bff70ffc`），库只为断言读。
- **S1，审查与两轮修复带来的**（编号是 S1 的审查记录的；主持者的决定 268–271 与第 267 条的提前见审查记录与 00 号文档第 11 节）：
  - **第 268 条**（P2S1A-I1、P2S1C-M1；`060b365a`）：一个 `batch` 里前面的改名或移动改写过一页基线页的链接，之后对这一页的正文操作一律答 `page.upgrade_batch_rewrite`，不论写下什么。情形：先改名 A，再以原来的正文写链向 A 的基线页 P；正文与库里的相同（改名的改写还没写进库），推后的改写又只跟 P 最后一个正文操作之后的移动，P 的链接于是断了，或静静地指向改名之后另建的 A，闸门 6、7 都看不出。设计 3.4 的表第 5 行"放过"本身就错（它是设计修订核对时 P2A-N6 建议的文字）。
    - 计划的 `PlannedPage.Folded` 改名为 `Reached`：这一页某个正文操作之前、上一个正文操作之后，有改名或移动改写了它，就设上，之后不清。写的路径是 `holdsAt` 的 `rewrote` 为真，不论写的字节；编辑的路径照旧是改写改了内容。`ContentWrite.Reached` 经 `StepContent.Reached` 到守卫（3.3、3.4）。
    - 守卫 `body` 先判缺陷（没有 `BodyHash`），再读基线行（不是基线页就放过，不读正文），再看 `Reached`（带标记答 `page.upgrade_batch_rewrite`），最后比正文（不同答 `page.upgrade_immutable`）（3.3）。
    - 码的英文文字改成写下的正文不带改写时也成立的说法（3.6）。
    - 3.4 的表的第 5、6、13 行与 g21 改为带标记、答 `page.upgrade_batch_rewrite`；整个程序加 g23（断链）、g24（指向别处）、g25（编辑改回）（3.6）。
    - 它收回第 254 条写的路径上的清除，第 265 条说的偏严形状里 g21 那一种改了，2.2 的"拒绝的范围不变"不再成立（2.1 第 2 条、2.2、2.3 的注）。
  - **字节不变、被到达的写也过守卫**（实施者在修复一轮自加，说明里没写；三位修复核对都判它必要、不误拒；`060b365a`）：写下的字节与库里相同时，`writeContent` 在第一个分支就返回，不造正文步，守卫根本看不到。g23 的"写 P 原来的字节"与 g25 的"编辑把 `[[Map]]` 改回 `[[A]]`"只改守卫的次序挡不住。所以升级中带 `Reached` 的这种写照一次写去过单元的守卫，什么也不写。守卫的准备与判定从 `applyStep` 抽成 `Unit.guarded`，两处共用（3.3）。
  - **第 270 条**（修复核对一轮的 P2S1A-M1；`0b016421`）：升级中判"这一页被够着过"，还要看这一页在 `batch` 之前的链接。情形：先把 P 里的 `[[A]]` 编辑成 `(A)`，再改名 A，再编辑回 `[[A]]`；改名的那一刻 P 在批里的内容不链向 A，标记设不上，最终的字节与库里相同，链接留在旧名字上。批的读在 `CheckReads` 之后调 `reachedBefore`：照现有的 `BatchLinks.Reached` 按批前的索引问到各页最后一个正文操作为止，够得着就 `plan.MarkReached`，在正文的一遍之前设上，之后照第 268 条粘住。只在计划读到的状态是 `upgrading` 时做（3.4）。代价（第 271 条，修复核对 P2S1B-M2）：最后一个正文操作之前有移动的已有页各多一次索引的问，在计划里、至多到解析的界；一般的最大规模上很便宜（改名一页、再编辑 49 页满负载的页，合计约 36–40 ms），但 M9/P3 已接受、交给 M12 的"许多次改名之间的带链接的页"在升级中的计划约翻倍：本机 7.5 s 对 3.5–3.8 s，调用 10.1 s，照 7763 的比估约 18.6 s，过 15 秒的请求期限；过期限时什么也不写，拆开就能过；不改代码，写进 M12 的性能移交（第 9 节）。
  - **第 269 条与第 267 条的提前**（P2S1B-M1；`80da6b1f`）：guards-core 的作业时限 30 → 40 分钟，costs-2 35 → 45 分钟。7763 上 guards-core 的末行在第 19.4 分钟才开始，costs-2 加了 S1 的一行之后末行约在第 23–24.3 分钟开始；卡住时 Go 默认的 10 分钟超时离作业时限只剩约半分钟到 1 分钟，runner 先把作业杀掉，打不出卡住的测试。6.2 第 8 个提交的"改 `timeout: 45`"改为"S1 已改"（6.2、7.1、7.5）。
  - **测试**：重新计划加"从同一个升级的 `reviewing` 到 `upgrading`"两格（P2S1A-N1）；g14、g15 成功的一半各加 `holds`（P2S1A-N2）；形状 a 的注释写明这条界守的主要是 M9 的改写（P2S1B-N1）；`contentStep` 的注释照字节不变、被到达的那一种改（P2S1C-N4）；组合根加 g26，钉住"不是基线页、被移动到达"的写照常做完（P2S1C-N5）；g27 钉住第 270 条，其中 Q 的两格让第 268 条正文一遍的判据在组合根上也被抓到（3.6、7.3）。
  - **只记下、不改历史的**：`90134668` 改了 `Makefile` 与 `ci.yml`，范围只写 `server`，部件该写 `build`、`ci`（P2S1C-N3）。6.1 第 5 条、6.2 第 1、8 条的范围随之写成 `server, build, ci`（6.2 第 1 条另有 `docs`）。
  - **交给后面的**：
    - guards-sources 下次改到它时，照第 269 条的写法提到 40 分钟（审查 B：7763 上作业 16.7–16.9 分钟，时限 30 分钟，约 1.8 倍）。
    - S2 加检查的守卫之后再看 costs-2 的余量（审查 B：慢三成的实例上末行约第 34 分钟开始，卡住时约第 44 分钟打出，离 45 分钟约 1 分钟）。S2 看过：审查 B 估 7763 上末行约第 24.5 分钟开始，卡住时第 34.5 分钟打出，先于 45 分钟，不再提（3.20）。
    - P3/S2 的 playbook 写拆法与只写名字的链接（第 9 节）。
    - P2/S2 的一个测试提交把 `mcp_test.go:269`、`:272` 等日志的两个 2 秒改成 10 秒（`TestTheMCPCallStopsWhenTheClientGoes` 在本机负载下第三次失败，`heavy.sh` 挡不住；S1 的审查记录"推后"）。S2 做了（`967fdbea`），没有治好：失败时 SDK 已在客户端走后把答复写进断开的连接，"答复没写出"那一行根本不会来；修复一轮照审查 P2S2C-M1 删掉那一条断言，这一条关上（3.20）。
    - M12 的性能移交：第 271 条的数，接在 M9 那一项之后（第 9 节）。

### 3.20 S2 的实施对设计的修正

S2 的审查与修复见 [S2 的审查记录](reviews/P2-S2-gate-check-submit-review.md)，结果在 10.2。与上文最初的设计不同之处已改在各节里（3.7–3.10、3.12–3.16、3.18、3.19 的"交给后面的"，4.2，第 5 节，6.2，7.1–7.3、7.5，第 9 节）。写了行号的，是实施的头 `c3c35bbc` 的；修复的提交见 10.2。

- **S2，实施时定下的**（实施者的 26 条偏离，括号里是它的号。三位审查者各在自己的范围里判过：A 判第 4、5、11、12、16、22、25 条，B 判第 18、19 条，C 判第 2、7、13、15、20、23、24 条，都对；第 11 条的"总早于"只在 `request_timeout` 不超过 20 秒时成立，由第 273 条写对；第 26 条的根因由审查 P2S2C-M1 查清；其余没有人提出异议）：
  - 6.2：检查的类型（`domain.UpgradeCheck` 等，`wiki/domain/upgrade_check.go`）随 `RunResult{Lint, Check}` 与 `CloneUpgradeCheck` 提前到 3 号提交，判定与报告仍在 5 号（1）。
  - 4.2：`wiki.upgrade_stale_index` 的服务端 `detail` 原写"…, then the upgrade can start."，提交也答它，改成不说是哪个动作，同网页（2）；修复一轮照审查 P2S2C-N4 改顺语气。
  - 3.16：wiki 的 `Logger`（`wiki/app/lint_run_types.go:68`）加 `ErrorContext`，检查的两行 error 要它（`*slog.Logger` 本来就有）（3）。
  - 3.12：第二个事务的每种结局（含 `superseded`）都照 13.1 第 4 条的码序判，拒绝时答它；第二个事务里锁不到笔记本（已删）时记 `superseded`、什么也不写，答 `notebook.not_found`、不再判，同第一个事务的锁会答的（4）。
  - 3.12：组合根的 `wikiUpgradeStates.SetState` 认 `upgrading`（原来只认 `reviewing`、`rolling_back`；"回到 `upgrading`"要它）（5）。提交的答复的 `submitted_by` 照用已有的 `named`（6，不是偏离）。
  - 6.2：`permission_matrix_coverage_test.go` 的 `targetViolation` 照 lint 的 `/lint/{rule}` 放过 `/upgrade/check/{rule}`（设计写"不改"）；`TestLintMatrixTargets` 另有反例，不在 `upgrade` 之下的 `/check/{rule}` 照旧报出（7）。
  - 组合根测试里的第二位管理员 dana 是工作区的成员、笔记本的管理员：`notebook.change_mode` 要笔记本的管理员，工作区管理员不够；降级经 `PATCH /api/v0/notebook-members/{id}` 改成 `editor`（8）。
  - 7.2：交错第 5 条以 `interleaveOn` 持住笔记本行、批的命令一步，不另包 `States`、不停在 `SetState` 之后（9）。
  - 6.2：`TestUpgradeCheck`、`TestUpgradeSubmit` 的 MCP 一半在 8 号（两个工具那时才有），8 号另给码依次加"不带 `rule` 的 `limit`"（10）；期限的"另一格"（第一个事务落在请求的期限上：平台答 500、warn 一行、没有提交的两行日志、行照旧 `upgrading`）也在 8 号加（12）。
  - 3.12、7.1、7.2：MCP 的工具调用的期限是读完请求体之后的 `RequestTimeout`（`platform/httpserver/mcpserver/call.go:46-53`），不是开始加 `BodyReadTimeout + RequestTimeout`，`at` 又在读完请求体之后；期限一格的 MCP 一半因此照 REST 断言（11）。第 273 条照审查 P2S2A-M4、P2S2C-N1 写出条件：`request_timeout ≤ 20 s`（默认配置在内）时两个入口都是请求的期限先到，`> 20 s` 时是 `submitted_at + SubmitTimeout`。设计审查记录核对 P2A-I1 时算的"MCP 4 s + `ReadTimeout` 5 s = 9 s"同样不对，在这里更正，那份记录不改。
  - 3.14：`upgrade_submit` 笔记本入口的 `inputSchema` 是 `{"type":"object","additionalProperties":false}`，没有 `properties`：SDK 对空结构体就这样编码，账户入口的 `notebooks` 自 M9 就是这样（13）。
  - 7.1：`TestUpgradeStatesReachTheStreams` 的提交是两格、各两帧（不过的提交：第一个事务一帧、退回一帧；补好之后通过的提交：第一个事务一帧、存下报告一帧），不是"三格"（14）。
  - 3.10、3.13：`ReportOf` 也去掉条目的 `path`：REST 的 lint 条目本来没有它（`path` 只在 MCP 的投影里），存下的路径会随改名、移动与回滚变旧，带上它最坏的报告（每条至多 `MaxDepth` 段名称，40 项）会过 64 KiB；最坏的报告 35,125 字节，修复一轮把它的 `wanted_now` 也改成封顶之后 35,128 字节（15）。
  - 7.3：`TestJudgeUpgrade` 的计数一格拆成 `TestJudgeUpgradesCounts`（`raw` 原有、下面两页发起时就在、一页移进来；`raw` 是补写的、两页移进来），变异 33 由它抓到（16）。
  - 3.15：`TestUpgradeToolsErrorsCarryTheirCodes` 的几种情形：`notebook.not_found` 只在账户入口、用一个不存在的 id（笔记本入口是边缘的 404）；`server_busy` 的名额由另一次升级的条目读占着（lint 读标准笔记本不占名额，先答 `notebook.not_llm_wiki`）；`wiki.upgrade_stale_index` 以 SQL 把一页的 `extractor` 记旧、测试末尾写回（17）。
  - 7.1：检查的守卫的"另一份种子"是同一个库里以 SQL 把一页的账本记成别的哈希：正文与账本不一致的两种写法对闸门 7 一样，改账本不动页、不用再种一份 5 万页（18）；守卫在 costs-2 的末尾，与发起的守卫之间隔护栏的读一行（19）。
  - 第 5 节：UP5 审阅中的冻结用 `edit`：`write` 不带 `base_version` 是创建，路径上已有页时先答 `page.title_taken`，到不了冻结（20）；页名用 `Page.name`，生成的 `Page` 类型没有 `title`（21）。
  - 7.3：变异 46 改在 linking 的 `LintCursor.PositionFor`（不比范围）：用例只经它绑定游标，没有单独"核对根"的一句；`TestCheckUpgradeAnswersInOrder` 的假游标自己判根，看不到这个变异，46 只对 `TestUpgradeCheck` 跑（22）。
  - 3.15：`TestUpgradeToolsMatrix` 读的一行也每格一个库的副本：`prepare` 以 SQL 设这一格的笔记本，并行的格共用一个库会互相踩（23）。
  - 6.2、3.14：8 号的范围带 `e2e`，并改第三份的 `validToolArguments`：工具表多了两个工具，LW8 断言的两个入口的工具数（16、18 → 18、20）与 `TestToolStrings*` 要的"每个工具一组能到用例的参数"跟着变；逐个提交的核对没跑它们，第一轮门禁才查出（24）。
  - 7.2：交错第 7 条读放弃之后的行不看回滚到了哪一步：`state` 在 `rolling_back` 或 `rolled_back`、提交的三列还在、没有报告，之后照旧等回滚的作业 `succeeded`；回滚的 worker 在本机常常在测试读之前就做完（25）。
  - 3.19：2 号提交的 10 秒没有治好 `TestTheMCPCallStopsWhenTheClientGoes`，第一轮 `make test` 里它又失败一次、10 秒里一行也没等到（26）；根因与修复见下。
- **S2，审查与修复带来的**（编号是 S2 的审查记录的；主持者的决定 272–275 见审查记录与 00 号文档第 11 节）：
  - **第 272 条**（分支 CI 上第二份 457 秒；审查 P2S2B-Q1；`d7c82c01`）：合并之前把第二份最大的 `TestVirtualExport` 挪进第七份与第八份的模式，第八份仍是第七份的子集：`BOOTSTRAP_SECOND = ^Test(E|Source|The(MCP[C-Z]|Contents))`，`BOOTSTRAP_SEVENTH = ^Test(Upgrade|IngestBefore$$|ABackground|VirtualExport$$)`，`BOOTSTRAP_EIGHTH = ^Test(Upgrade(Check|Gates|Submit|Tools)|VirtualExport$$)`。只改 `SECOND` 与 `EIGHTH` 时它在第三份（`[U-Z]`）与第八份各跑一次。第二份慢约 5% 的原因审查 B 查清：契约变大，组合根每建一个应用就 `apitest.Load` 一次，竞态构建下每次多约 62 ms，不是产品的热路径（7.5、第 9 节）。`Makefile` 的计数注释照挪后的 `-list` 写（547 个，145/50/121/124/62/11/27/7；P2S2B-N1、P2S2C-N5）。挪后本机第二份约 203 秒、第八份约 100 秒，折到 7763 约 360、193 秒；修复第三、四轮的 CI 上第二份、第八份各有一次落在 7763：Test 一步 400、187 秒，都在 450 秒之下（7.5）。
  - **第 273 条**（P2S2A-M4、P2S2C-N1）：MCP 的期限只改说法（上面第 11 条），3.12 第 3 步、7.1 的期限一格与"正在计算"的那一段、7.2 第 7 条照改。
  - **第 274 条**（P2S2A-N1、P2S2A-Q1；`968bb586`）：提交的期限答 503 时，`Retry-After` 取到能接管为止的秒数 `max(1 s, ceil(first.at + SubmitTimeout − now))`（`wiki/app/submit_upgrade.go:130`）：默认配置下这一刻离能接管还有约 2.5 秒，1 秒的重试落进"正在计算"的 409（3.12 第 6 步、3.13 的契约说明、7.1 第 5 步、第 9 节的 playbook）。契约 `submitUpgrade` 的说明原写 `Retry-After: 1`，同改，所以这个提交的范围带 `api`、`web`。秒数随服务端答复的那一刻差一秒（本机 REST 2 秒、MCP 3 秒），组合根的期限一格只断言下界、再照它等过让另一位管理员接管，精确值由单元 `TestTheSubmissionsDeadline` 钉住（实施者的修复报告第 9 节第 1 条，A 判对）。
  - **第 275 条**（P2S2B-M2；`f962a593`）：costs-3 的作业时限 35 → 40 分钟：S2 在 `-timeout 20m` 那一行之前加了闸门的读的计划一行，7763 上那一行的超时落在作业的第 33.5–35.2 分钟（6.2、7.1、7.5）。
  - **`TestTheMCPCallStopsWhenTheClientGoes`**（P2S2C-M1；`701c736d`）：不是等得不够久。客户端走的那一刻，go-sdk 的 `release` 与工具交回错误的结果在同一把锁上竞争：后者先拿到时 SDK 把答复写进已断的连接、`wrote` 为真，"答复没写出"那一行（`mcpserver/server.go:208-219`）就不会来；平台两种结局都对，测试只认其中一种。M9/P1 在单元里已照两种结局改过（`mcpserver/call_test.go:385-389`，"没写出"那一支由 `stubborn` 钉住）。修复一轮删掉组合根的那一条断言，3.19 交下的那一条关上；修复核对一轮 C 在审查时复现它的负载下 40 次全过。
  - **`expectUpgrade`**（P2S2C-M3）：第 5 节原写"`submission_id IS NOT NULL` 恰在 `reviewing`"，不是表的不变式：批准、驳回与审阅中放弃之后提交的三列都留着（`00057` 的两条检查、`RollBackUpgrade` 不清它们）；改成"`submission_id` 有无与答复的 `submitted_at` 有无相同"，夹具在修复一轮改（`b6ef8599`）。
  - **测试**（修复一轮，每条以审查者的变异核对改前活、改后死）：`TestSubmitUpgradeAnswersInOrder` 每格只让一道闸门不过的七格（P2S2A-M1，a6）；`TestJudgeUpgrade` 经 `withAttachment` 带一个基线附件的三格（保留着、已删、基线行没了；P2S2A-M2，a1、a2；3.10 的表）；新的 `TestTheSubmissionsSnapshotIsItsOwn` 三格（P2S2A-M3，a4；3.18），它经 `submitFake` 已有的 `duringRun` 钩子改行，第二个事务也读到改过的行（只改快照的钩子造出的是整个程序里到不了的状态）；`wanted_now.count_capped` 四层各一个封顶的夹具（P2S2C-M2，c1、c11–c13；`48bb2e18`）；闸门的读的计划守卫另判 `nodes` 只经主键，另加不要数据库的 `TestTheGateReadsReadANodeByItsKey`（P2S2B-M1，gm3、gm7；`3893c318`；7.1）；`TestUpgradeToolsMatrix` 照 `r.entries` 只在它的入口跑（P2S2C-N6；3.15）；M8 留下的 `TestARestoreJobSkipsAnItemRestoredBetweenItsBatches` 与 `TestARevertJobSkipsAMemberRestoredBetweenItsBatches` 认负载下的另一种码（审查 B 的 4.7；`e88cf289`）。
  - **修复第二轮**（P2S2C-M4，修复核对一轮的新发现；`9dce4658`）：修复一轮把三个夹具的 `WantedNow` 改成封顶之后，它们的 `wanted_before` 与 `wanted_now` 完全相同（封顶时值必是 10,000），存与读、REST 投影把两个成员弄混，单元测试照过（C 的 s1、s2）；s1 在组合根与 e2e 也没有测试抓。改成每层让一个成员封顶、另一个不封顶，两个方向各一个夹具；最坏的报告保留两个都封顶（35,128 字节），另一个方向由 `TestTheReportRoundTrips` 末尾的另一格往返承担。s1、s2 改后死。修复核对第二轮 C 判它关上，另加的五条两个方向的探针也都死。
  - **修复第三、四轮**（P2S2C-M5、M6，修复核对第二、三轮的新发现；`ef8ac10f`、`d4324ab8`，都只改测试）：与 M4 是同一类：投影或编码把一个成员写成另一个、或写成常量，因为夹具里有重复的值、或只断言了部分成员，测试照过，组合根与 e2e 也看不到。M5 是 S2 建成时就有的：存下的报告的五个计数在夹具里都是 100000，MCP 投影的计数只断言两项（C 的 p1、p2、m1–m3）；M6 在计数之外：REST 报告的规则的 `count` 与 `count_capped`、`preserved` 的 `nodes` 与 `nodes_kept`、MCP 投影的闸门（hr1–hr3、hg1、xg1–xg3）。第三轮让检查与报告的计数在判定、REST、存与读、MCP 每一层互不相同、整值断言（72 条变异，改前活 28 条）；第四轮不再按成员补，按投影收尾：S2 新写的七个投影（REST 的检查、规则页、报告与带报告的视图，MCP 的检查，存与读，判定的 `ReportOf`）的夹具每个标量互不相同（`count_capped` 有真有假、与值无关，规则的数不等于条目数），八道闸门与 `passed` 排成四行、每道闸门那一列互不相同，整个答复与 JSON 字面或写死的结构体比，MCP 另加 `TestTheCheckIsAnsweredWhole`；555 条对调与常量的变异改前活 217 条、改后全死，没有发现投影本身有错（3.10、3.18、7.1、7.3）。最坏的报告两轮都不变（35,128 字节）。修复核对第四轮 C 判 M6 关，整轮重跑 555 条全死，七个投影里这一类收尾。这一类的教训记进 05-lessons。
  - **修复第五轮**（P2S2C-M7、N9、N10，修复核对第四轮的新发现；`43caa32a`，只改测试）：C 把范围放宽到 S2 写的所有逐字段转换。组合根把 page 的 `Preserved` 转成 wiki 的那一行（`bootstrap/wiki_upgrade.go:188`）是跨模块的转换，单元碰不到，组合根读闸门 7 的两格 `nodes` 都等于 `nodes_kept`，把 `NodesKept` 写成 `Nodes` 整个程序照过（wp1），删掉的基线附件就会过闸门 7；`TestUpgradeGates` 加一格"a baseline attachment deleted past the units"（以 SQL 绕过单元删一个基线附件，`preserved` 整值 4/3/2/2，断言后还原；7.1）。另两条 Nit：REST 报告条目的 `kind` 写成常量（夹具里只有 `asset`，加一项 `page`；规则页的 `mode` 同样以 `standard` 再答一次），提交的日志补断言 `user_id`。11 条变异改前活 4 条、改后全死，第四轮的 555 条在新头上仍全死；三处转换本身都没有错。修复核对第五轮 C 判三条都关上，并补查了 S2 其余的逐字段转换（组合根的 `SetState` 与 `AreaWantedByPage`、MCP `upgrade_verify` 的参数都有测试分得出；`Baseline` 的 `kind` 恒为 page 是等价变异），这一类收尾；新的 P2S2C-N11（Nit，不改）：`TestUpgradeGates` 以 SQL 改库的两格把还原写在 `check` 里，那一格先 `Fatal` 时后面各格跟着红，以后改这个测试时另用 `t.Cleanup` 注册同一句还原（7.1）。修复核对到第五轮为止，没有 Important、Minor 留下。05-lessons 那一条补上跨模块的逐字段转换。
  - **代码**（修复一轮，`debec8dc`）：linking 的 `lintQuery` 拒绝摘要带 `AreaWantedByPage`（P2S2A-N2；3.8）；`wiki.upgrade_stale_index` 的服务端文字改为"ask a server admin to run nervewiki reindex, then try again."，单元的 stale 一格断言整句（P2S2C-N4；4.2）。
  - **只记下的**：P2S2C-N7（MCP 矩阵的提交一格不读它留下的行，别处已盖住）不改；P2S2B-N2（`TestTheGateReadsReadANodeByItsKey` 断言了共用帮手 `rollbackReadProblems` 的口子，以后收紧帮手时它会失败）不改，收紧帮手时一并改（7.1）；`PurgeTrimmedChangesets`、`DropEmptyChangeset` 与进行中的升级的变更集（审查 B 的未核实的怀疑）：不是错误，实施者查代码、B 核过，3.9 记一句；C 的 eg2（存下的报告的两道闸门对调）在生产里看不到（只有通过的检查才存报告），修复第四轮的往返仍钉住它。
  - **交给后面的**：契约每长一截组合根各份都慢一截，P3 定分份时算进去（P2S2B-Q2，第 9 节）；P3/S2 的 playbook 写照 `retry_after_seconds` 等过再交，K3 看客户端是否接受没有 `properties` 的 `inputSchema`（第 9 节）；guards-sources 的时限 S2 没有改到，照旧下次改到时照第 269 条提到 40 分钟；costs-2 的余量 S2 看过（审查 B：7763 上末行约第 24.5 分钟开始，卡住时第 34.5 分钟打出，先于 45 分钟），不再提；e2e 的 L1（带修饰键点链接、等新标签页 30 秒超时，修复一轮的 CI 上第四次记下的旧偶发）交给 P2/S3 另查（主持者定）。

### 3.21 S3 的实施对设计的修正

S3 的审查与修复见 [S3 的审查记录](reviews/P2-S3-web-review.md)，结果在 10.3。与上文最初的设计不同之处已改在各节里（3.18，4.3，6.3，7.1、7.3、7.6，第 9 节）。写了行号的，是修复一轮的头 `ff50d516` 的（修复第二轮只改测试；`swr/dist/…` 是已装的 SWR 2.5.1 的）；修复的提交见 10.3。

- **S3，实施时定下的**（实施者的 8 条偏离与"自己加的"第 1 条，括号里是它的号；都标"实施中定"，没有新编号的决定。三位审查者都判对：A 逐条判 1–8，B 判第 1、3、4 条与第 74、85 行的等价，C 逐条判 1–8 并判第 9 节第 1 条是对设计的修正；第 1 条的症状、第 6 条与第 74 行的理由照审查改写；第 85 行等价的证据要修复一轮让"检查在飞时"的三格看得见卡的第一页才成立，审查 P2S3B-I1，修复一轮之后成立）：
  - 4.3：这一次进入的 `SWRConfig` 一直挂着，只包住检查一节的位置（`upgrade-page.tsx:300`：`UpgradeRead` 始终渲染 `<SWRConfig value={checksConfig}>{check}</SWRConfig>`，里面只有 `upgrading` 时的检查一节，别的不变）。设计写缓存"只包住检查一节"、"检查一节卸下又装上用的是同一份"，照字面让 `SWRConfig` 随检查一节挂与卸：SWR 2.5.1 在它卸下时删掉这个 provider 的全局状态（`initCache` 的 `unmount`，`swr/dist/config-context-*.mjs:512-520`：`SWRGlobalState.delete(provider)`，里面有 `FETCH` 与订阅），装回时同一个 `Map` 配一份新的状态。在飞的请求经旧的全局状态的 setter 写进同一个 `Map`、只通知旧的订阅者，装回的 hook 订的是新状态、收不到：答复在一节回来之后才到的两格（别人的提交；自己的提交，检查在一节回来之后答复）停在"检查中…"，在一节卸着时就到的那一格照过（装回时 `Map` 里已有 `data`）；再去掉 `revalidateOnMount` 时另发一次、读两次，后到的写入与 `Map` 里的相等、不重渲染，仍停在"检查中…"（审查 A、B、C 实测，a-swrconfig-per-section、b27、c14 是同一种写法）。所以一直挂着（1；实施报告写的"另发一次、读两次"只在另去掉 `revalidateOnMount` 时才有，页面照样停住，审查 P2S3A-N4、B-N2；C 在 S3 审查时记的"c14 读两次"不对，修复核对一轮自己更正，P2S3C-N12）。
  - 4.3：`unchanging` 另加 `revalidateOnMount`（`upgrade-check.tsx:121`，这一代的键在这一次进入的缓存里没有条目时才读）：读过、在读或被拒的一代，一节装回不读。它修正设计 4.3 的"已读过的一代从缓存显示、不再读（选项照健康度页的 `unchanging`）"：只靠 `unchanging` 时，只留了错误（`server_busy`）的一代在一节卸下又装上时会再读一次（卸着时没有订阅者，`data` 为空就读），等于自动重试，与"`server_busy` 不自动重试"相反。`upgrade-check.test.tsx` 的"a check refused as busy, its section taken away and given back"钉住，变异 own-revalidate-on-mount-default 抓到（实施报告第 9 节第 1 条；审查 P2S3C-N9；A 读源码核对过初次按"检查"、再检查、装回三种情形）。
  - 4.3、7.3 第 85 行：审查 C 在修订核对里的前提"SWR 不调卸下的 hook 的回调"不对：React 18 起 `useSWR` 的 `callbackSafeguard` 只比键（`swr/dist/use-swr-*.mjs:317-322`，`IS_REACT_LEGACY` 为假，本项目是 React 19.3），卸下的 hook 照样调 `onSuccess`、`onError`；装回的 hook 在请求在飞时不另发。视图照设计从 `data` 推出，结论不变，变的只是理由：它不依赖回调由谁、何时调。设计写的第 85 行的变异（视图在 `onSuccess` 里建，同健康度页）因此在这里是等价的：检查在一节卸着时答复，卸下的 hook 的 `onSuccess` 照样把视图写进这一次进入的缓存；检查在一节回来之后答复，装回的 hook 不另发、照样得到 `onSuccess`。实施时拿来作证的"两种次序都抓不到"是在只断言卡的标题的三格上得出的，对照的"卡恒为空"在它们上也照过（审查 P2S3B-I1）；修复一轮（`b9790af0`）让三格断言卡的第一页与"更多"之后，卡恒为空（b25o）、卸下的 hook 不建视图（b26）与 c1 都在三格上死，85 仍照过，这才是证据（修复核对一轮 B 核过）。7.3 第 85 行换成设计真正要防的 c1：视图建在检查一节自己的 `useState` 里、卸下即丢（审查 P2S3C-N9）（2）。
  - 4.3、7.3 第 81 行：提交成功一格持住升级的第二次起的读（`answers` 覆盖升级的读），不只第二次（`secondReadHeld`）。假服务端分两段之后，答复回来之前还有第二次 `mode` 与 `change` 的 `reread()` 两次读；只持住第二次，它们很快带回审阅中与报告，"答复不进缓存"的变异等一等就照过。持住时断言报告已显示、对话框已关、焦点在标题，放开之后横幅变为审阅中、报告还在（3）。
  - 7.3 第 75 行：变异"`recheck` 直接发请求，不经 SWR 的键"在这个结构里写不出：`recheck` 只加一代，没有一处小改能让它自己发请求。照它要抓的行为写成"检查一节卸着时要的一代，装回时不读"（`revalidateOnMount: false`），由 409 先到的那一格抓到（4）。
  - 4.3：假服务端的 503 是名额满的那一种。真服务端提交的 503 有两种：名额满（第二个事务照样退回 `upgrading`、发 `mode`）与第二段过期（什么也不写，行留在"审阅中、没有报告"）。"检查在飞时自己提交"要前一种，假服务端只做这一种；后一种在网页上就是"正在核对，或上一次提交没有做完"，由"报告为 `null`"一格盖住（5）。
  - 4.3：升级页的 `HealthRule` 的 `paths` 交一个不做事的函数（设计没有说）：健康度页在条目的节点不在树里时经它重读一次外层的树；升级页不另重读，检查的答复先于 agent 那次写的 `pages` 事件到时，新页的条目短暂写"（已不在）"（健康度页这时写"加载中"），由 `pages` 事件的重读纠正，最终一致（6；实施报告写的理由"升级页的树在外层读，`pages` 事件让它跟上"两页一样，不是差别，审查 P2S3C-N8）。
  - 4.3：`HealthScope.notebookId` 删去（设计没有说）：`read` 做成参数之后没有人读它（7）。
  - 4.3 的 vitest："检查在飞时自己提交"两种次序各一格。设计只写了检查在一节回来之后才答复的一种，另加检查在一节卸着时就答复的一格，第 85 行的变异两格都跑（8）。
  - 7.3 第 74 行：设计写的"或"（键不带 `nonce`，或离开时不清）在这里两半各自是等价的变异（74x、74y）：每次进入一个新的 `Map`，`UpgradeRead` 以 `location.key` 为键、每次进入重新装上，只去掉 `nonce`，新的一次进入照样找不到上一次的条目；只不清，上一次的 `Map` 已没有人拿着。合起来（公共的 provider 且键不带 `nonce`）才是会犯的错，那一条抓到。`nonce`、键里的 `locationKey` 与离开时的 `clear()` 都是防御，在行为上测不到，照设计留着（审查 P2S3B-Q1，三位都在"过度设计"里提到）。实施报告写的"'或'是照健康度页的公共缓存写的"不对：健康度页同样每次进入自己一个 `Map`（`health-page.tsx:77`；审查 P2S3A-N4、B-N3）。
- **S3，实施者自己加的测试**（实施报告第 9 节第 2–5 条；C 在自己的范围里看过都对）：另一个管理员先提交了（第一段答 409 `wiki.upgrade_state`，页面重读成审阅中、焦点给标题、检查没有读）；提交被拒为 `server_busy`、`forbidden`（对话框的文字、检查不读）；状态表 `upgrading`、`reviewing`、`rolling_back` 三格；store 的重读失败时仍抛原来的拒绝、`offline` 不重读；报告的夹具每个值互不相同、规则的次序与页面的不同（`toReversed()`），抓"照答复的次序渲染"（own-o1）；UP5 页面版本的反向对照（去掉提交被拒时的再检查，故事失败）。
- **e2e 的 L1**（3.20 的"交给后面的"）：本机复现，根在 Chromium（新标签页在会话建立之中提交了文档、不发 `Page.frameNavigated`，Playwright 只靠这个事件报新页；审查 B 在协议日志上重跑实施者的脚本，支持这个结论）；把新标签页的文档压后一秒的两种只改测试的改法各有 5/30 的别的失败。故事不改、没有第 5 个提交，第 277 条照现状（6.3、10.3，审查记录"偶发"）。
- **S3，审查与修复带来的**（编号是 S3 的审查记录的；修复一轮照主持者的修复说明 `fix-1.md`，提交见 10.3）：
  - **"更多"的 403**（P2S3A-M1 = P2S3C-M1）：卡的"更多"答 403 时页面停在管理员的样子，笔记本列表不重读；设计 4.3 的"403 时调 `rereadNotebooks`"说的是检查与"更多"两者。修复一轮（`41389247`）以 `UpgradeCheckSection` 里的 `readItems`（`upgrade-check.tsx:159-166`）包住 `upgrades.checkItems`，`forbidden` 且 `scope.current()` 时重读列表、再抛原拒绝；不改 `HealthRule` 的接口，P3 的审阅一节照同样写法（4.3、第 9 节第 5 条）。
  - **测试看得见卡的第一页**（P2S3B-I1）：见上面第 85 行一条。
  - **测试补全**（4.3 的 vitest 末条；`2c78dc31`、`b9790af0`、`ff50d516`）：检查本身答 409（P2S3A-M2 = B-M3）；闸门投影跨行的常量（B-M1）；报告一条规则多项、各条的数互不相同（B-M2、N5；修复第二轮补到 5 项，N7）；树在读时闸门的节点写"加载中"（B-M4 = C-N4）；"更多"读到的条目在一节卸下又装上之后还在（B-M5）；离开时中止在飞的检查、检查答错之后不写"检查中"、各节的次序、对话框的语气（A-N2、C-N5、C-N6）；检查一节的规则次序（B-N1）。
  - **同一次进入里换了升级**（P2S3A-N1，`904d0d83`）：`UpgradeRead` 记下每一代是为哪次升级要的（`checked = { generation, upgrade }`，`recheck(upgrade)`，`upgrade-page.tsx:115`、`:133-140`），交给检查一节的 `generation` 在显示的升级不同时为 0（`:257`），检查一节回到初始、按"检查"才读（4.3）。修复之后 a-key-no-upgrade-id（键里去掉 `upgrade.id`）照过，是等价的：代数在一次进入里单调，新的一次升级按"检查"拿到的是比用过的都大的一代，两次升级的键在 `generation` 上已不同；键里的 `upgrade.id` 与 `nonce` 一样是防御、测不到。要让它单独死，得让代数每次升级各自从 1 起，多一层、少一分稳，没有这样做。用户看得见的错（新的一次在按"检查"之前就显示或读了检查）由 2 号的那一格钉住，实施者加的 f2（代数不看升级）在它上面死（修复一轮的实施者报为新的偏离，主持者接受为等价；7.3，审查 P2S3C-N13）。
  - **文字**（`cfbf80fa`）：其他管理员的接入一句写全（P2S3A-N3）；zh 的"待建链接"统一为"待建页面"、`frontmatter` 闸门的文字写清、en 的两处与"会重新读取"（P2S3C-N1–N3）（4.3）；假服务端 `not_ready` 的 `detail` 抄真服务端（P2S3B-N4）。
  - **只记下的**：P2S3B-N6（e2e 整组比闸门与时刻）不改，vitest 已钉住投影；P2S3B-Q1 的三层防御留着、写明测不到（4.3、7.3 第 74 行）。
  - **修复一轮**（6 个提交，头 `ff50d516`，10.3）与 `fix-1.md` 不同之处只有上面的 a-key-no-upgrade-id。fix-1 第 11 条的"检查答错之后"加在原来的 503 一格里，"对话框的语气"加在提交的帮手 `submitting()` 里、各提交格都走它。
  - **修复核对一轮**：A 全关、新 P2S3A-N5；B 关八条、P2S3B-M2 部分关（报告每条至多 3 项，截到 3 或 4 照过，记为 N7）；C 全关、新 P2S3C-N12–N15。`readItems` 不判 `scope.current()`（A 的 m1-no-current、B 的 n6、C 的 m1b）照过，是等价的：在飞的"更多"在再检查与离开时被中止，拒绝是 `AbortError`，到不了；这个判断是防御，与检查的 `onError` 一致（主持者接受）。
  - **修复第二轮**（只改测试，`fix-2.md`，`3717deba`）：`keptReport` 照真服务端给条目，每条规则至多 5 项（`wiki/domain/upgrade_check.go:139`）、数在 5 以内的全列，八条的数仍互不相同（P2S3B-N7）；"更多"答 409 `wiki.upgrade_state` 那一格断言笔记本列表没有多读（P2S3A-N5）；P1 那一格的测试名跟上新文字（P2S3C-N15）。与 `fix-2.md` 没有不同；n1、n2、m1-any-code 改前照过、改后死。修复核对第二轮（B）三条都关上，另加的三条新夹具的变异都死，没有新发现。两轮修复、两轮核对之后没有 Important、Minor 留下；Nit 都已改或在本提交，P2S3C-N7 交给 P3（第 9 节），P2S3B-N6 不改，P2S3B-Q1 的三层防御保留；等价的变异：85、74x、74y、a-key-no-upgrade-id 与 `readItems` 不判 `scope.current()`（m1-no-current、n6、m1b）。
  - **文档稿**（P2S3C-N12–N14，本提交）：c14 的症状（上面第 1 条）；7.3 的 a-key-no-upgrade-id 写成等价、另记 f1、f2；4.3 的 `checked`、`recheck(upgrade)` 与 `readItems` 照修复之后的代码；本节与第 9 节的行号照 `ff50d516`。
- **交给后面的**：P3/S3 的审阅一节，第 9 节的七条（审查 P2S3C-N7、N10、N11；第 5 条带上修复核对一轮 A 的意见）；L1 的上游问题与改测法留给负责人（第 277 条）。

## 4. 网页

网页的门禁（tsc、knip、vitest、`make gen-check-web`）在每个 Step 的合并时都跑，所以服务端的 Step 自带它改的契约在网页上要的东西；service、store 的方法与组件只在 S3 加（knip 报出没人用的导出）。

### 4.1 S1 随服务端的部分

不改。三个码只在 MCP（第 210 条），REST 的操作不声明它们，`problem-messages.test.ts` 不要它们的文字；它们的文字是服务端的 `detail`（3.6），同 P1 的 `page.upgrade_protected`（P1 设计 4.1）。

### 4.2 S2 随服务端的部分

- `make gen-web` 重新生成 `web/packages/api-client/src/schema.gen.ts`：`UpgradeCheck`、`UpgradeGates` 等十几个 schema 与三个操作；`Upgrade` 加必填的 `report`。没有 service 用它们（S3 才加），生成的类型经 `export type *` 再导出，knip 看不到（同 P1 设计 4.2）。
- `Upgrade` 必填 `report` 之后 tsc 报出的字面量：`W/test/upgrade-server.ts:22-40` 的 `upgradeJSON` 加 `report: null`（别的测试与 e2e 都由它或已有的升级展开得到，`upgrade-page.test.tsx:238`、`e2e/stories/upgrade/up3-abandon-rollback.spec.ts:59-60`）。
- `wiki.upgrade_not_ready` 的文字：`W/app/problem-messages.ts` 的表（`:44-49` 的 wiki 码之后）加 `"wiki.upgrade_not_ready": "problem.wiki.upgrade_not_ready"`；`W/i18n/messages/zh-CN.ts`（`:1115` 旁）"升级还没有通过检查，已回到升级中：检查一次，修好没通过的，再提交。"，`en.ts`（`:1175` 旁）"The upgrade does not pass its checks yet: it is upgrading again. Check it, fix what did not pass, then submit it again."。`problem-messages.test.ts` 读契约，核对 `submitUpgrade` 声明的每个码都有文字。
- `wiki.upgrade_stale_index` 的文字今天写"……再开始升级"（`zh-CN.ts:1114`、`en.ts:1173`），`submitUpgrade` 也声明它，改为不说是哪个动作："链接的索引过时了：请服务器管理员运行 nervewiki reindex，再试一次。"、"The link index is out of date: ask the server's administrator to run nervewiki reindex, then try again."。服务端的 `detail`（`wiki/domain/upgrade_errors.go`）原写"…, then the upgrade can start."，同样不说是哪个动作，改为"The link index is out of date: ask a server admin to run nervewiki reindex, then try again."（S2 的实施；修复一轮照审查 P2S2C-N4 改顺语气）；发起与提交都答它。
- "server update"改了 `events.yaml` 的一处说明（3.14），生成的类型的注释随之变，`make gen-check-web` 照过。

### 4.3 S3：网页

00 号文档 4.15 的 P2 部分，照本文 2.1 第 13 条（第 259、264 条）。

**service 与 store**：

- `W/services/upgrade.service.ts` 的 `UpgradeService` 加三个方法（以笔记本 id 为参数，同 P1/S4 的四个）：

  ```ts
  check(notebookId: string, signal?: AbortSignal): Promise<UpgradeCheck>;
  checkItems(notebookId: string, rule: LintRule, cursor: string, signal?: AbortSignal): Promise<LintPage>;
  submit(notebookId: string): Promise<Upgrade>;
  ```

  `checkItems` 的 `cursor` 必填（同 `wiki.service.ts:21` 的 `items`：第一页在检查的答复里）。
- `W/stores/notebook.store.ts`：构造的 `Pick<UpgradeService, …>`（`:54`）加 `"submit"`；`submitUpgrade(notebookId)` 经 `inTurn`（`:47`），成功之后重读笔记本列表（`upgrade.state` 变了，横幅要跟上）；被拒时只有表示列表落后的码照 `downgrade` 重读列表（`behindSubmit = {forbidden, notebook.not_found, wiki.upgrade_state}`，同 `behindRollback`，`:297`），重读失败仍抛原来的拒绝；`wiki.upgrade_not_ready`、`wiki.upgrade_stale_index`、`server_busy` 不重读（升级在第二个事务里已回到 `upgrading`，`mode` 事件会让列表跟上）。store 不碰 SWR，`["upgrade", notebook_id]` 由组件在答复之后写与重读（同 P1/S4 的放弃）。

**`HealthRule` 的三个参数**（`W/pages/notebook/health-rule.tsx:48`）：今天读一页写死成 `wiki.items`（`:92-96`），"范围变了"写死成 `notebook.not_llm_wiki`（`:114`），卡的标题写死成 `h2`（`:183`）。加

```ts
  /** Reads a page of the rule's items after cursor. */
  read: (rule: LintRule, cursor: string, signal: AbortSignal) => Promise<LintPage>;
  /** The code that says the scope the summary was read in is gone. */
  moved: string;
  /** The card's heading's level: 2 on a page of its own, 3 under a section's. */
  level?: 2 | 3;
```

健康度页（`W/pages/notebook/health-page.tsx:239-259`）交 `read={(rule, cursor, signal) => wiki.items(notebook.id, rule, cursor, signal)}`、`moved="notebook.not_llm_wiki"`，`level` 缺省为 2，行为不变（`health-rule.test.tsx`、`health-page.test.tsx` 照旧过）。升级页交 `level={3}`：卡落在"检查"这个 `h2` 之下，大纲不与它平级（审查 P2C-N4）。模式的核对（`:119-122`、`:160`）不改：升级的条目经显式的 LLM Wiki 契约，答复的 `mode` 恒是 `llm_wiki`，升级页交 `mode="llm_wiki"`。`HealthItem` 与 `healthNodeId`（`W/pages/notebook/health-item.tsx:13`、`:24`）的参数类型改为 `UpgradeReportItem`（不带游标的条目；每种 `LintItem` 在结构上都能赋给它，它们不读游标），存下的报告的条目也由它显示。`HealthScope.notebookId`（`W/pages/notebook/health-request.ts`）在 `read` 做成参数之后没有人读，删去（S3 的实施，3.21）。

**升级页**（`W/pages/notebook/upgrade-page.tsx` 的 `UpgradeRead`，`:144-173` 按状态的各节）：

| 状态 | 各节（次序） |
|---|---|
| `upgrading` | 状态；接入 agent（P1）；**检查**（新）；**"提交审阅…"**（新）；"放弃这次升级"（P1） |
| `reviewing` | 状态；**检查报告**（新，只读）；"放弃这次升级"（P1） |
| `rolling_back` | 照 P1 |

接入 agent 一节对其他管理员写"只有发起人 {name} 的 agent 能写这本笔记本；你可以检查、提交审阅，或放弃这次升级。"（`upgrade.agentOthers`，`{name}` 是显示名；00 号文档 4.15；P1 只写了放弃，S3 建成时没有跟上，审查 P2S3A-N3，修复一轮）。

新的 `W/pages/notebook/upgrade-check.tsx` 导出 `UpgradeCheckSection` 与 `UpgradeReportSection`，内部的 `UpgradeGates`（八道闸门的列表）与 `UpgradeCounts`（`<dl>`）两者共用。

- **这一次进入**（照健康度页的 `HealthEntry`，`health-page.tsx:66-99`，审查 P2C-M1）：`UpgradeRead` 每次装上取一个 `nonce`（`useState(() => ++nextCheckEntry)`），持一个 `control` 的 ref（`alive`、`generation`、`controllers`）与 `checked` 的 state（`{ generation, upgrade }`：最近的一代与它是为哪次升级要的，`upgrade-page.tsx:115`；初稿写 `generation` 的 state，修复一轮改，审查 P2S3A-N1、C-N14），以及这一次进入自己的 SWR 缓存（`useState(() => new Map())`，只包住检查一节，经 `<SWRConfig value={{ provider: () => cache }}>`；升级的读 `["upgrade", notebook_id]` 照旧在全局的缓存里，`mode` 事件读得到它）。检查一节卸下又装上（自己的提交在算时，见下面"提交"一条）用的是同一份；`SWRConfig` 本身一直挂着、只包住检查一节的位置（`upgrade-page.tsx:300`，里面只有 `upgrading` 时的检查一节）：SWR 2.5.1 在带 `provider` 的 `SWRConfig` 卸下时删掉这份缓存的全局状态（在飞的请求与订阅），同一个 `Map` 装回时配一份新的状态；随一节挂卸时，在飞的请求经旧的全局状态写进同一个 `Map`、只通知旧的订阅者，答复在一节回来之后才到时装回的 hook 停在"检查中"；再去掉 `revalidateOnMount` 时另发一次、读两次，仍停在"检查中"（S3 的实施，审查 P2S3A-N4、B-N2、C-N12，3.21；初稿照字面让它随检查一节挂与卸）。`nonce`、键里的 `location.key` 与离开时清缓存在这个结构里是防御：每次进入一个新的 `Map`、`UpgradeRead` 又按 `location.key` 重新装上，它们在行为上测不到，照设计留着（审查 P2S3B-Q1）。`UpgradeRead` 卸下（离开升级页）时 `alive = false`、中止 `controllers`、清掉这一份缓存（同 `:85-90`）。回来是一次新的进入：检查一节是初始的，不读。`UpgradePage`（`upgrade-page.tsx:44`）以 `key={location.key}` 渲染 `UpgradeRead`，同健康度页的 `<HealthEntry key={location.key}>`（`health-page.tsx:54`）：在升级页上再点一次同一页的链接也是新的一次进入，`nonce` 与代数随之重来，不会因键里的 `location.key` 变了而自己读一次（审查 P2C-N8）；对话框随之关上，同健康度页。
- **外层的读**（审查 P2C-N7）：这一次进入的缓存只包住检查一节，里面的 `useSWRConfig().mutate` 碰不到全局的缓存。所以笔记本列表的重读与 `rereadUpgrade` 一样，由 `UpgradeRead` 以外层的 `mutate(["notebooks", ws])` 交下（`rereadNotebooks`，同它的 `onError`，`:91-95`）；页面树由 `UpgradeRead` 在外层读（`["pages", id]`，与左栏、`UpgradeState` 的同键，`:233-234`），与它是否在读（`readingTree`）一起经 props 交下，同健康度页（`health-page.tsx:45-51`："The tree stays in the public provider"），检查一节不在这一次进入的缓存里另读树。
- **检查**（`UpgradeCheckSection`，props：`notebook`、`upgrade`、`rereadUpgrade`、`rereadNotebooks`、`pages` 与树的读的状态 `readingTree`（卡的 `HealthRule` 要它，`health-rule.tsx:62`，同健康度页交的 `tree.isValidating`），以及 `UpgradeRead` 交下的 `entry`、`generation`、`recheck`）：
  - 一个标题"检查"（`h2`）、一句"检查这本笔记本是否满足 LLM Wiki 的约定；全部通过才能提交审阅。"、一个按钮"检查"（读过一次之后是"再检查一次"）；按钮调 `recheck(upgrade.id)`，交当时显示的升级。打开页不自动读（00 号文档 4.15："检查"按钮读；agent 才是主要的检查者）。
  - `recheck(upgrade)`（`UpgradeRead` 的，`upgrade-page.tsx:133-140`）：`control.generation` 加一、中止上一代的 `controllers`、`setChecked({ generation, upgrade })`；"检查"按钮交当时显示的升级，提交被拒交提交时的升级（`submit` 在发出之前取）。交给检查一节的 `generation` 是 `checked.upgrade` 为显示的升级时的 `checked.generation`，否则为 0（`:257`；初稿写 `setGeneration`，修复一轮改，审查 P2S3A-N1、C-N14）。检查经 SWR 的键 `["upgrade-check", store.loginId, workspace.id, notebook.id, upgrade.id, location.key, nonce, generation]`，`generation > 0` 时才给键：一节装上就读这一代，已读过的一代从缓存显示、不再读（选项照健康度页的 `unchanging`，不随焦点与重连重读；另加 `revalidateOnMount`：这一代在这一次进入的缓存里已有条目，读过、在读或被拒，一节装回时不读。只靠 `unchanging` 时，只留了错误的一代在一节卸下又装上时会再读一次，`server_busy` 之后等于自动重试；S3 的实施者加的，是对这一句的修正，审查 P2S3C-N9，3.21）。同一次进入里换了升级（放弃、回滚结束之后另一位管理员发起了新的一次）：新的一次没有代，检查一节回到初始、按"检查"才读，不因键里的升级 id 变了而自己读（见上面的 `checked`；审查 P2S3A-N1，修复一轮）。代数在一次进入里单调，两次升级的键在 `generation` 上已不同，键里的 `upgrade.id` 与 `nonce` 一样是防御，测不到（审查 P2S3C-N13，3.21）。所以再检查不靠检查一节当时挂着：一节卸着时 `recheck()` 只加一代，装上时读一次（审查 P2C-I1）。
  - 请求的归属照健康度页（`health-page.tsx:125-139`）：`HealthScope` 的 `key` 是上面的键，`controllers` 是 `control` 的那一个 `Set`，`current()` 是"`alive`、`generation` 没变、登录没换"。视图从这个 hook 的 `data` 与 `error` 推出：结果一行、闸门、计数与每张卡的第一页都来自 `data`，"检查中"是"有键、没有 `data` 也没有 `error`"；`onSuccess` 不建视图（健康度页在 `onSuccess` 里建，`:162-178`）。只有卡的"更多"读到的条目与它的提示照健康度页存在 SWR 的 `[...key, "view"]`（`:144-150`）。所以装回时缓存里有 `data` 就显示它，不停在"检查中"：检查在飞时一节卸下（自己的提交的第一段），答复照样进这一次进入的缓存，装回时显示它：视图不依赖回调由谁、何时调（审查 C 在修订核对里的怀疑）。初稿另写的理由"SWR 不调卸下的 hook 的回调"不对：SWR 2.5.1 在 React 18 起 `callbackSafeguard` 只比键，卸下的 hook 照样调 `onSuccess`、`onError`，装回的 hook 在请求在飞时不另发；结论不变。"视图在 `onSuccess` 里建"在这里因此是等价的，要防的是视图存在会随一节卸下丢掉的地方（7.3 第 85 行，审查 C 的 c1）（S3 的实施，审查 P2S3B-I1、C-N9，3.21）。读经 `healthRequest(scope, (signal) => upgrades.check(notebook.id, signal))`，重试经 `healthRetry`（`server_busy` 不自动重试，显示"再试一次"）。
  - 结果：一行 `output`（`role="status"`）"全部通过，可以提交审阅。"或"有 N 道检查没有通过。"，带检查的时刻（`computed_at`，经 `i18n/format.ts`）；`UpgradeGates`：八项，每项是闸门的文字（下表）与"通过"或"没有通过"；三道带节点的闸门列出节点（经页面树的 `nodeTarget` 成链接，树是 `UpgradeRead` 交下的 `pages`；不在树里的写"（已不在）"），`links` 另写"N 页指向的待建页面比升级开始时多"（S3 建成时写"待建链接"，与 `wanted` 一卡的"待建页面"不一，修复一轮统一，审查 P2S3C-N1），`preserved` 写"节点 a/b 个、页 c/d 页保留着"；`index` 不过时写 `stale_pages` 与"请服务器管理员运行 nervewiki reindex"；`UpgradeCounts`：七个数，两个 `wanted` 封顶时带"+"（同 `health-rule.tsx` 的写法）；规则：照健康度页的"有问题的规则成卡、没有问题的收进一个 `details`"（`health-page.tsx:233-234`、`:299`），每张卡是 `HealthRule`，`read={readItems}`（包住 `upgrades.checkItems(notebook.id, rule, cursor, signal)`，见"范围变了"；初稿直接交 `checkItems`，修复一轮改）、`moved="wiki.upgrade_state"`、`mode="llm_wiki"`、`level={3}`；`paths` 交一个不做事的函数：健康度页在条目的节点不在树里时经它重读一次外层的树，升级页不另重读，检查的答复先于 agent 那次写的 `pages` 事件到时，新页的条目短暂写"（已不在）"，由 `pages` 事件的重读纠正，最终一致（S3 的实施，审查 P2S3C-N8，3.21）。
  - 范围变了（检查或"更多"答 `wiki.upgrade_state`：升级已不在升级中）：这一节写"升级的状态变了"，调 `rereadUpgrade`；之后按新的状态显示（`reviewing` 时这一节换成报告）。403 时调 `rereadNotebooks`（见"外层的读"）：检查与卡的"更多"都是；卡的 `read` 是 `UpgradeCheckSection` 里的 `readItems`（`upgrade-check.tsx:159-166`），包住 `upgrades.checkItems`，`forbidden` 且 `scope.current()` 时重读列表、再抛原拒绝，不改 `HealthRule` 的接口（S3 建成时只做了检查的，审查 P2S3A-M1 = C-M1，修复一轮）。只认 `forbidden`，同检查本身；`scope.current()` 是防御：在飞的"更多"在再检查与离开时被中止，拒绝是 `AbortError`，到不了（修复核对一轮）。
- **提交**：按钮"提交审阅…"（`upgrading`；`ref` 进 `buttons`，`Change` 加 `"submit"`）开 `ConfirmDialog`（`held` 与 `onClosed`，`tone="default"`，同 P1 的"继续回滚"，决定 251），标题"提交审阅？"，描述"提交之后这本笔记本对所有人冻结，包括发起人的 agent。系统从这一刻再检查一次：全部通过就进入审阅，等管理员批准；没有通过就回到升级中。"，按钮"提交审阅"。确认时 `change(() => notebooks.submitUpgrade(notebook.id), true)`（`:109-134`：答复只盖住发出时显示的那一份，之后重读）。`change` 的拒绝表（`:116-122`，今天是 `transfer.busy` 与 `wiki.upgrade_state`）加 `wiki.upgrade_not_ready` 与 `wiki.upgrade_stale_index`：这两种之后升级已回到 `upgrading`，页面要重读。
  - **自己的提交在算的那一段**（第 264 条，审查 P2C-I1、P2C-Q1）：提交的第一个事务一提交就发 `mode`（3.12 第 2 步），`modeChanged`（`W/events/handlers.ts:279-283`）重读升级，页面照实换成审阅中：检查一节与"提交审阅…"卸下，报告一节写"正在核对，或上一次提交没有做完。"；对话框在按状态的各节之外（`upgrade-page.tsx:176-220`），留在原处、按钮在发送中，答复回来之前不动。不在 `UpgradeRead` 里压住这段时间的重读：页面显示的就是服务端的真状态（`reviewing`、`report: null`）。
  - 成功：对话框关上，焦点给标题（`done`，同放弃）；页面显示答复（`reviewing`、带报告）。中间那次 `mode` 的重读常常已换过缓存，`change` 的"只盖住发出时那一份"于是不写，之后的重读补上，无害。
  - `wiki.upgrade_not_ready`、`wiki.upgrade_stale_index`：对话框带着拒绝留着（P1 的写法），`explain` 给前者"检查没有通过，升级已回到升级中。下面的检查会重新读取。"（`upgrade.notReady`；初稿写"已重新读取"，409 先到时检查一节还卸着，对话框出现时常常还没读，审查 P2S3C-N3，修复一轮；不显示服务端英文的 `detail`），后者照 `problemMessages`；同时 `recheck` 提交时的升级一次（00 号文档 4.8"提交答 409 之后自动读一次检查"）、重读升级。第二次 `mode` 的重读与 409 谁先到不定：检查一节在 409 时还卸着，就在升级回到 `upgrading`、它装上时读那一代；已装上就立即读；两种次序都恰读一次。关上之后焦点回到"提交审阅…"；页还在审阅中（重读还没回来）时它不在，焦点落到标题（`dialogOf` 的退路，`:140`）。
  - `wiki.upgrade_state`：照 `change` 重读升级（`:119`），文字照 `problemMessages`。审阅中、没有报告（别人或 agent 的提交正在算）时报告一节写"正在核对"（下一条）。
  - `server_busy`、`forbidden`、`notebook.not_found`：照 `problemMessages`；后两个由 store 重读列表。
- **检查报告**（`UpgradeReportSection`，`reviewing`）：标题"检查报告"（`h2`）、一句"提交时的检查（时间），全部通过。"；`UpgradeGates`、`UpgradeCounts` 同上；八条规则各写名称（`h3`）与数，有条目时列出至多五项（`HealthItem`），不带"更多"与按钮（"更多"从第一页读与审阅的三个动作在 P3/S3，00 号文档 4.15）。规则照健康度页的次序（`healthRuleOrder`），不照答复的次序；`UpgradeReportSection` 自己渲染这八条，名称与数经 `health-rule.tsx` 导出的 `ruleTitle`、`issueCount`，不经 `HealthRule`（S3 的实施，第 9 节）。`report` 为 `null` 时只写"正在核对，或上一次提交没有做完。"（同 4.15 审阅一节的那句；P2 里到得了：自己或别人的提交的计算之中，或第二个事务没有做完）。

闸门的文字（`upgrade.gate.<key>`，两种语言；en 照同样的意思）：

| 键 | zh-CN |
|---|---|
| `agents` | 根下的 AGENTS 页有正文 |
| `source_root` | 来源根是一页，不是根下的 AGENTS |
| `reserved_titles` | 根下没有名为 index、log、index.md、log.md 的节点 |
| `frontmatter` | 来源区以外的页（含来源根与根下的 AGENTS）的 frontmatter 都能解析；除这两页外都带 type 与 summary（初稿读起来把"能解析"也豁免给了这两页，00 号文档 4.7 闸门 4 只豁免后两条；审查 P2S3C-N2，修复一轮） |
| `sources` | 每个 sources 都指向来源区里的节点 |
| `links` | 升级开始时已有的页，指向的不存在的页没有变多 |
| `preserved` | 升级开始时已有的节点都在，已有的页的正文没有变 |
| `index` | 链接的索引是新的 |

计数的文字（`upgrade.counts.<key>`）：来源区里的节点、移进来源区的原有节点、来源区以外的页、升级以来新建的页、这次升级的变更、升级开始时的待建页面、现在的待建页面（初稿写"待建链接"，审查 P2S3C-N1；en 的"Pages created since …"，审查 P2S3C-N3；修复一轮）。

**vitest**：

- `upgrade-check.test.tsx`（新，经升级页即 `UpgradeRead` 渲染，不单独渲染 `UpgradeCheckSection`：这一次进入的缓存、外层的列表与树只有这样才在，审查 P2C-N7）：按"检查"才读（打开页时假服务端没收到检查的请求）；八道闸门的过与不过、三道闸门的节点成链接、不在树里的写"（已不在）"；七个数、封顶带"+"；有问题的规则成卡、没有问题的在 `details` 里，卡的标题是 `h3`；卡的"更多"读 `GET …/upgrade/check/{rule}?cursor=…`（假服务端核对路径与游标），不读 lint 的条目；"更多"答 `wiki.upgrade_state` 时写"升级的状态变了"并重读升级；检查答 `server_busy` 不自动重试、"再试一次"再读；403 重读笔记本列表；再检查中止上一代的请求（上一代的答复回来也不显示）；检查一次、离开升级页、回来：检查一节是初始的，假服务端没有再收到检查的请求（审查 P2C-M1），回来之后再按"检查"读了新的一次（S3 的实施者写代码时的抽查补的：只断言"回来时不读"，"公共缓存且不带 `nonce`"照过）；在升级页上再点一次同一页的链接：检查一节同样是初始的，没有请求（审查 P2C-N8）；**检查在飞时自己提交**（6.3 第 3 个提交加）：按"检查"，答复之前点"提交审阅…"、确认，提交答 503 `server_busy`（假服务端照真序把升级退回 `upgrading`、发 `mode`），之后检查才答复：检查一节回来时显示那一次检查的结果，假服务端只收到一次检查（审查 C 在修订核对里的怀疑）；另一种次序（检查在一节卸着时就答复，一节回来时显示它）另成一格（S3 的实施，3.21）。假服务端的 503 是名额满的那一种：真服务端提交的 503 另有第二段过期的一种（什么也不写，行留在"审阅中、没有报告"），网页上就是"正在核对，或上一次提交没有做完"，由"报告为 `null`"一格盖住（S3 的实施）。
- `upgrade-page.test.tsx` 加：`upgrading` 有检查与"提交审阅…"，`reviewing` 没有、有报告、报告的规则没有"更多"按钮；**提交的第一段**（假服务端照真序分两段，下一条）：第一段之后检查一节不在、报告一节写"正在核对…"、对话框在；放开答 409 `wiki.upgrade_not_ready` 之后检查一节回来、对话框写 `upgrade.notReady`、假服务端恰收到一次检查，第二次 `mode` 的重读先到与 409 先到两种次序各一格（409 先到：假服务端把第二次 `mode` 留到测试放开，页面靠 `change` 对 `wiki.upgrade_not_ready` 的重读回到升级中，检查一节装上、读这一代；审查 P2C-I1）；关上之后焦点在"提交审阅…"；`wiki.upgrade_stale_index` 同样再检查；**提交成功**持住升级的第二次起的读（S3 以 `answers` 覆盖升级的读；初稿写以 `secondReadHeld`（`upgrade-page.test.tsx:243-262`）只持住第二次，即第一段的 `mode` 引起的那一次：假服务端分两段之后，答复回来之前还有第二次 `mode` 与 `change` 的 `reread()` 两次读，它们很快带回审阅中与报告，"答复不进缓存"的变异等一等就照过，3.21）：答复回来时缓存还是发出时那一份，`change` 把答复写进去，持住时页面已显示审阅中与报告（审查 P2C-M2；不持住时这次重读先换了缓存，答复照 `change` 的规则不写、之后的重读补上，测不出答复进没进缓存），持住时另断言对话框已关、焦点在标题，放开之后横幅的状态随列表的重读变为审阅中、报告还在；`wiki.upgrade_state` 时重读升级；报告为 `null` 时写"正在核对，或上一次提交没有做完。"；`mode` 事件把审阅中改回升级中（第二个事务的退回）时检查一节回来。
- `notebook.store.test.ts`：`submitUpgrade` 照降级的七条（成功、403、404、409 `wiki.upgrade_state` 重读列表，`wiki.upgrade_not_ready` 不重读，并发的两次只发一次，卸载，重读失败仍抛原来的拒绝）。
- `health-rule.test.tsx`：`read`、`moved`、`level` 三个参数各一格（交别的读时"更多"经它；`moved` 的码到时调 `mismatch`，`notebook.not_llm_wiki` 在升级的卡上不算；`level={3}` 时标题是 `h3`）。
- 修复一轮补的（S3 的审查的发现，编号省去前缀 P2S3；在 `upgrade-check.test.tsx` 与 `upgrade-page.test.tsx`）：检查本身答 409 `wiki.upgrade_state`：一节写"升级的状态变了"、结果为空、不给"再试一次"、重读升级（A-M2 = B-M3）；卡的"更多"答 403 重读笔记本列表恰一次、页面写只给管理员（A-M1 = C-M1）；"检查在飞时"的三格断言回来之后"缺少 type"一卡的条目与"更多"（B-I1）；闸门投影六行，不过的数、`pages_worse`、`stale_pages` 跨行不同（B-M1）；报告一条规则多项、八条的数互不相同（B-M2、N5）；树在读时闸门的节点写"加载中"（B-M4 = C-N4）；"更多"读到的条目在一节卸下又装上之后还在、不另读（B-M5）；离开时中止在飞的检查，检查答 503 之后结果一行为空、按钮不 `aria-busy`，状态表三格各节标题与按钮的先后，"提交审阅…"的确认按钮不是 `destructive`（A-N2、C-N5、C-N6）；同一次进入里换了升级、检查不自己读，按"检查"读新的一次（A-N1）；检查一节的规则夹具与 `healthRuleOrder` 的次序不同（B-N1）。修复第二轮补的（只改测试）：报告的 `undigested_sources` 数 6、存 5 项，数在 5 以内的规则条目个数与数一致，同真服务端每条规则至多存 5 项（B-N7）；"更多"答 409 那一格断言笔记本列表没有多读（A-N5）。
- 假服务端（`W/test/upgrade-server.ts`，照 `lint-server.ts:179` 的写法）加三条路由，只答真服务端答得出的码、照真的次序（05-lessons 第 2 节）：检查 404、403、409 `wiki.upgrade_state`、503；条目 400（游标解不开）、404、403、404 规则、409、400（绑定）、422、503；提交 404、403、409 `wiki.upgrade_state`、503、409 `wiki.upgrade_stale_index`、409 `wiki.upgrade_not_ready`。提交照真序分两段（照 P1 的 `heldServer`，`:266-294`，它有 `events`）：收到请求时先把升级设成 `reviewing`、`report: null`、`submitted_*` 有值，经 `events` 发 `mode`；测试放开（`releaseSubmit`）之后才答，不过的两种与名额满的 503 把升级退回 `upgrading`、清掉提交的列，通过的设上测试给的报告，各再发一次 `mode`；升级在两段之间变了答 409 `wiki.upgrade_state`（S3 的实施）；`not_ready` 的 `detail` 抄真服务端的（审查 P2S3B-N4，修复一轮）；这第二次 `mode` 可以由测试留住、之后放开（`holdMode`）。不分两段的提交（测试不持住时）照样两段，只是不停。

## 5. e2e

**夹具**：`e2e/fixtures/upgrades.ts` 加（S2）`checkUpgrade`、`getUpgradeCheckItems`、`postSubmit` 与 `submitUpgrade`（照 `postUpgrade`、`startUpgrade`，`:11`、`:20`）；`e2e/fixtures/assert/wiki.ts` 的 `expectUpgrade`（`:15`）加 `submitted_by_id`、`submitted_at` 与答复相同（时刻在 SQL 里比到微秒）、`submission_id` 有无与答复的 `submitted_at` 有无相同（批准、驳回与审阅中放弃之后提交的三列都留着：`00057` 的两条检查要它们，`RollBackUpgrade` 不清它们；初稿写的"`submission_id IS NOT NULL` 恰在 `reviewing`"不是表的不变式，审查 P2S2C-M3，修复一轮改夹具）、报告：有无与答复相同，有时 `report->'passed'`、`report->'gates'`、`report->'counts'` 与答复的相同（时刻与条目不比：时刻的写法两边各自定，条目由 `TestTheReportRoundTrips` 管）；`e2e/fixtures/assert/page.ts` 加 `expectBaselineKept(db, upgradeId)`（S1）：每个基线节点活着、每个基线页的 `content_hash = accepted_hash`（账本的绊线没有响）。MCP 照 `e2e/fixtures/mcp.ts` 的 `mcpClient`（`:152`）、`errorOf`（`:227`）、`resultOf`（`:220`）。

| 故事（`e2e/stories/upgrade/`） | 版本与 Step | 内容 |
|---|---|---|
| `up4-guardrails.spec.ts` UP4 护栏 | MCP：S1 补全（P1 的删除一个测试照旧） | 两个新的测试。**正文与移动**：一本有 Plan、Notes（链向 Plan 与 Old）、Old（以 `---` 起、没有收尾行）、Broken（带收尾行的无效 frontmatter）、Other（同样无效）的笔记本，经 REST 种下、发起（基线页的 `base_version` 取 `createPage` 的答复，审查 P2S1C-N1）；发起人的 agent：给 Plan 加 frontmatter（成功）；`edit` Plan 的正文一字（`page.upgrade_immutable`）；`move` Plan 带 `update_links: false`（`page.upgrade_keep_links`）；`move` Plan 进一个新的文件夹（成功，Notes 被系统改写：Notes 以从根起的路径 `[the plan](/Plan.md)` 链向 Plan，文件夹是先写的一页 `Work.md`。只写名字的链接，移动之后照样解析，系统不改写；"移动之后系统改写链接"只对写了路径的链接成立，审查 P2S1C-N2）；`move` Old 改名为 `Legacy.md`（成功，Notes 里指向 Old 的链接被系统改写；00 号文档第 9 节 UP4 一行的"改名放行"，审查 P2C-M4）；新建一页、改它的正文（成功）；Legacy（原 Old）前面加一块（成功）；Broken 就地改成有效的（成功）、Other 前面另加一块（`page.upgrade_immutable`）。数据库：被拒的页的正文不变，Old 的名字是 `Legacy`、Legacy 的正文（切掉 frontmatter 之后）不变，Notes 的正文是两次改写之后的，`expectBaselineKept`。**`batch`**：一个 `batch` 先把 Plan 移走、再改 Notes 的 frontmatter，答 `page.upgrade_batch_rewrite`、`op` 是 Notes 的那个操作、什么也没写；拆成先改 Notes、再移 Plan 的一个 `batch`，成功，Notes 带着新的链接；`expectBaselineKept`。API 豁免：REST 的写在冻结里就被拒（P1 设计第 5 节） |
| `up5-check-submit.spec.ts` UP5 检查与提交 | API（MCP 写作准备）、MCP：S2；页面：S3 | **API**：一本有 Plan（链向一个不存在的 Missing）与六页 Note 1–6 的笔记本，经 REST 种下、发起；`checkUpgrade` 答 200、`passed` 为假、`frontmatter` 不过、`missing_type` 七项（摘要五项、`next_cursor`）；`listUpgradeCheckItems` 以那个游标读到其余两项；`submitUpgrade` 答 409 `wiki.upgrade_not_ready`（`detail` 列出 `frontmatter`），`getUpgrade` 是 `upgrading`、`submitted_by` 为空；agent 经 MCP 给七页加 `type` 与 `summary`；再检查 `passed` 为真、`counts.wanted_before` 与 `wanted_now` 都是 1、`new_pages` 是 2（补写的两页）；`submitUpgrade` 答 200 `reviewing`、带报告；`getUpgrade` 的报告等于提交的答复的；`expectUpgrade`；agent 的 `edit` 答 `notebook.upgrading`（审阅中对所有人冻结；`write` 不带 `base_version` 是创建，路径上已有页时先答 `page.title_taken`，到不了冻结，S2 的实施）。**MCP**：同样的一本，经 `upgrade_verify`（不带 `rule`：闸门与路径；带 `rule: "missing_type", limit: 1`：一项与 `next_cursor`）、`upgrade_submit`（工具的错误 `wiki.upgrade_not_ready`；修好之后 `{state: "reviewing", submitted_at}`），审阅中 `upgrade_verify` 照答、`passed` 为真，`edit` 答 `notebook.upgrading`；`expectUpgrade` 读 REST 的 `getUpgrade`。**页面**（Chromium）：管理员打开升级页，按"检查"，`frontmatter` 没有通过、"缺少 type"一卡五项与"更多"，按"更多"读到其余两项；按"提交审阅…"、确认，对话框写 `upgrade.notReady`，检查自动重读一次（计算之中页面可能照实闪过审阅中，第 264 条；断言只看结局：对话框的拒绝、检查一节回来、读到的是新的一代）；关上，焦点在"提交审阅…"；agent 经 MCP 修好；"再检查一次"全部通过；提交，对话框关上、焦点在标题，页面显示审阅中与报告，横幅写"已提交，等待管理员审阅"。刻意的 409 一个，以 `pageWatch.expectConsole({ errors: [failedToLoad(409)] })` 声明，另断言 `apiFailures` 恰是这一条（13.2 第 9 条） |

UP5 的页面版本只在 Chromium（00 号文档第 9 节：三种浏览器只给 UP1 与 UP7；对话框与 LW12 是同一个 `ConfirmDialog`）。`wiki.upgrade_stale_index` 不进 e2e：故事里造不出过时的索引（要以 SQL 记旧，同 P1 的发起），由 `TestUpgradeSubmit` 盖住。对话框与按钮按确切的名字找（05-lessons 第 6 节）。`make e2e` 从 518 个到 523 个（UP4 两个、UP5 三个；实测 S1 末 520 个、S2 末 522 个）。S2 的 UP5 以 `Page.name` 认页（生成的 `Page` 类型没有 `title`）。

## 6. 实施步骤

每个 Step 一个分支，从前一个合并之后的 `main` 开始；分支上的每个提交都绿。提交的写法照 00 号文档第 7 节"提交说明"（`00-M11-design.md:520-527`）：代码 `<范围>: <说明> (M11/P2/S<m>)`，范围列出碰到的每个部件（改契约的提交重新生成 `schema.gen.ts`，所以带 `web`）；分支上的文档 `docs: … (M11/P2/S<m>)`；合并之前的修复写到 Step，核对写 `fix check <k>`。接受本文的文档提交同时以 `p2-apply-00.py` 照 2.1 改 00 号文档（第 8 节第 4 条）。

**每个提交单独是绿的**：

- golangci-lint 的 `unused`（`server/.golangci.yml:5`，测试文件也算）要每个不导出的函数在它落下的提交里有调用者。
- 契约声明一个码的提交要同时在模块的 `adapter/http` 测试里答出它（`apitest` 的 `unanswered`）、在网页加它的文字。
- MCP 的码：工具的错误一带上没有声明的码就失败（`bootstrap/mcp_client_test.go:305-334` 的 `toolError` 与 `declaredCodes`）。所以一个提交答出一个新的码时，同时把它加进 `mcpToolCodes()` 的行、`upgradeToolCodes()` 的对（由 `TestUpgradeToolsErrorsCarryTheirCodes` 答出），只在 MCP 的码另加进 `mcpOwnCodes()`（审查 P2A-N1）。
- 契约加一个操作的提交要同时加它的矩阵的行（`coveredMatrixRows()` 要每个操作有行），`tools.json` 加一个工具的提交要同时加它的 MCP 的行（`coveredMCPRows()`）。
- 守卫与账本在同一个提交：账本的写核对写之前的哈希（3.5），每个正文步（含系统的改写）都要写它；少了哪一种写，那一页的下一次写就核对不上、答缺陷。

### 6.1 S1（`m11-p2-s1`）

1. `server: an upgrade's move or rename rewrites the links to what it moves (M11/P2/S1)`：3.2；`page.upgrade_keep_links`：`mcpToolCodes()` 的 `move`、`batch` 两行、`upgradeToolCodes()` 的两对、`mcpOwnCodes()`；`TestUpgradeToolsErrorsCarryTheirCodes` 的两种情形（`move`、`batch` 带 `update_links: false`，两个入口各一次）；`TestTheFreezeGuard` 的两种不改写链接；`TestUpgradeGuardrails` 的 g5、g6、g13。
2. `server: an upgrade's agent changes a page's frontmatter, not its body, and the ledger checks each write (M11/P2/S1)`：3.3（读的上界除外）、3.5；`page.upgrade_immutable`：`mcpToolCodes()` 的 `write`、`edit`、`batch` 三行、`upgradeToolCodes()` 的三对、`mcpOwnCodes()`；`TestUpgradeToolsErrorsCarryTheirCodes` 的三种情形（`write`、`edit`、`batch` 改基线页的正文）；g1–g4、g7、g8、g14–g18；`TestTheUpgradeBodyShapes`、`TestTheFreezeGuard` 的加项、`TestTheUnitHashesTheBodyWhileUpgrading`、`TestTheUnitKeepsTheLedger`、`TestUpgradeBaselineContent`、`TestAcceptUpgradeContent`。
3. `server: while upgrading, a batch's blind writes count in its reads' bound (M11/P2/S1)`：3.3 的"读的上界"（第 266 条）；`CheckReads`；单元比计划时的状态（`differs(…, "state")`）；`TestTheReadsBoundCountsBlindWritesWhileUpgrading`、`TestTheBatchPlansAgainOnceTheUpgradeStarted`；g22。码是 `batch` 已有的 `page.batch_too_large`，码表不变。
4. `server: a batch's rewrite folded into a page's body is refused at its last content op (M11/P2/S1)`：3.4；`page.upgrade_batch_rewrite`：`batch` 一行、一对、`mcpOwnCodes()`；`TestUpgradeToolsErrorsCarryTheirCodes` 的一种情形（先移动后编辑折进的改写）；g9–g12、g19–g21；`TestThePlanTellsAFoldedRewrite`、`TestTheBatchMarksABodysRefusalAtTheLast`。（记录：`f6de159e` 照这里写的 `Folded` 实施；修复一轮的 `060b365a` 照第 268 条改成粘住的 `Reached`，`TestThePlanTellsAFoldedRewrite` 改名 `TestThePlanTellsAPageAMoveReached`，加 g23–g25；第二轮的 `0b016421` 照第 270 条补批前的链接与 g27，3.19。）
5. `server, build, ci: the guardrails' reads and units take their time (M11/P2/S1)`：7.1 的两个守卫（已推的 `90134668` 范围只写 `server`，部件该写 `build`、`ci`，不改写，审查 P2S1C-N3）。成本分份的说明（行号是 `f002672b` 的）：
   - `Makefile`：`test-go-costs-2` 末尾（`:362` 之后）加一行与它的注释；说明（`:347`）"原成本命令第 14–20 条、LinksReached 物理守卫与删除护栏的读（CI 第二份；需要 Docker）"改为"原成本命令第 14–20 条、LinksReached 物理守卫、升级的发起与两种护栏的读（CI 第二份；需要 Docker）"（发起的守卫是 P1/S2 加的，说明当时没改，一并补上）。`test-go-guards-core` 在 `TestABatchAtItsRewriteBoundTakesItsTime` 那一行（`:456-461`）之后加一行与它的注释。
   - `ci.yml`：costs 的注释（`:72-84`）末尾加"M11/P2/S1：costs-2 末尾加护栏的读一条（本机约 N 秒）；现共40条（13/12/15），时限不变。"；guards-core 的注释（`:94-98`）末尾加"M11/P2/S1：加护栏在界上的单元一行（本机约 N 秒），时限不变。"。
   - S1 的修复（`80da6b1f`，范围 `ci`）另把 guards-core 的时限从 30 分钟提到 40 分钟、costs-2 从 35 分钟提到 45 分钟，注释各一段，带 run 号、作业用时与 runner（第 269 条，第 267 条提前，审查 P2S1B-M1）。
   - `06-environment.md`：第 50 行只列 CI 的各项，成本分份的条数与内容不在里面，不改。
6. `e2e: UP4, the guardrails over MCP (M11/P2/S1)`：第 5 节；`expectBaselineKept`。
7. `docs: …`（第 8 节第 4、5 条）。

### 6.2 S2（`m11-p2-s2`）

实施时 S1 留下的测试提交（`TestTheMCPCallStopsWhenTheClientGoes` 的两个日志等待，3.19）排在第 2 个，下面第 2–9 个是实施的 3–10 号，第 10 个是本 Step 的文档提交；修复一轮的提交见 10.2（3.20）。

1. `server, build, ci, docs: the composition root's eighth shard (M11/P2/S2)`：第 261 条。照 P1 设计 3.8 的写法，每一处的旧文字与新文字（行号是 `f002672b` 的；审查 P2C-M3）：
   - `Makefile`：
     - `BOOTSTRAP_SEVENTH`（`:282`）之后加一段注释与 `BOOTSTRAP_EIGHTH = ^TestUpgrade(Check|Gates|Submit|Tools)`。注释："M11/P2/S2：升级的检查、闸门、提交与 MCP 工具的整个程序测试单列一份（00-M11 第 261 条）：不开时 S2 之后第七份在 EPYC 7763 上约 375–420 秒（P2 设计 7.5），邻份没有余量，P3 还要往 TestUpgrade 加测试。它是第七份的子集，第七份 -skip 它；第一、三、六份 -skip 第七份，第四份跳过第三份的 [U-Z] 时已跳过它们，别的份的模式不收它们。"之后接 S2 第 1 个提交时两轮的数。
     - `test-go-bootstrap-7`（`:315-316`）：说明"名字匹配 BOOTSTRAP_SEVENTH 的：升级的，……"改为"名字匹配 BOOTSTRAP_SEVENTH 且不匹配 BOOTSTRAP_EIGHTH 的：升级的（检查、闸门、提交与工具除外），……"；命令加 `-skip '$(BOOTSTRAP_EIGHTH)'`。
     - 新的 `test-go-bootstrap-8` 照 `-7` 写：说明"竞态检测下运行组合根的整个程序测试中名字匹配 BOOTSTRAP_EIGHTH 的：升级的检查、闸门、提交与工具（CI 的一份；需要 Docker）"，命令 `-run '$(BOOTSTRAP_EIGHTH)'`。
     - `test-go-bootstrap`（`:288`）的依赖加 `test-go-bootstrap-8`，说明"七份依次"改为"八份依次"。
     - `test-go`（`:190`）的列表加 `test-go-bootstrap-8`。
     - 注释：`:183`"组合根按测试名七份"改为"八份"；`:184`"竞态检测的这八个最多六个同时跑"改为"这九个最多六个同时跑"（`-j6` 不变）；`:185`"各份之和就是下面 CI 的十四份"改为"十五份"；`:195`"CI 的十四份（short 是第十五份"改为"CI 的十五份（short 是第十六份"。
     - 计数的注释 `:265`"当前 542 个顶层（竞态构建，M11/P1/S3 审查修复之后）静态分配为 145/51/121/124/62/11/28（见第五份与第七份的说明）"改为"当前 N 个顶层（竞态构建，M11/P2/S2 开第八份时）静态分配为 145/51/121/124/62/11/27/1（见第五份、第七份与第八份的说明）"。N 与八个数照这个提交时的 `go test -race -list`；照本文的设计 S1 不加顶层测试，N 仍是 542，第八份只有 P1 的 `TestUpgradeToolsErrorsCarryTheirCodes`（P1/S1 的 S1C-N2 就是这一处漏改）。
   - `.github/workflows/ci.yml`：
     - `bootstrap-7`（`:70-71`）之后加 `- leg: bootstrap-8`、`timeout: 25`，上面一行注释"M11/P2/S2：升级的检查、闸门、提交与工具（TestUpgrade(Check|Gates|Submit|Tools)…）单列一份（Makefile 的 BOOTSTRAP_EIGHTH，00-M11 第 261 条）：不开时第七份在 7763 上约 375–420 秒，邻份没有余量"。
     - `:23`"-4、-5、-6 与 -7"改为"-4、-5、-6、-7 与 -8"；`:25`"server 共十五份（含 short），连同 web、e2e 与 image 共十八项"改为"十六份……十九项"；`:48`"按测试名分成七份（Makefile 的 FIRST、SECOND、THIRD、FIFTH、SIXTH、SEVENTH 与第四份互补规则"改为"八份（Makefile 的 FIRST、SECOND、THIRD、FIFTH、SIXTH、SEVENTH、EIGHTH 与第四份互补规则"。
   - `docs/v0.1/claude_to_codex/06-environment.md`：
     - `:27`"`make test-go-bootstrap-1`（`-2` 到 `-7` 同理）"改为"（`-2` 到 `-8` 同理）"。
     - `:50`"组合根现在分七份（`Makefile` 的 `BOOTSTRAP_FIRST` 到 `BOOTSTRAP_SEVENTH`），CI 共 18 项：15 个 server 的 leg（race、bootstrap-1 到 bootstrap-7、costs-1 到 costs-3、costs-lint、guards-core、guards-sources、short），加 web、e2e、image。"改为"组合根现在分八份（`Makefile` 的 `BOOTSTRAP_FIRST` 到 `BOOTSTRAP_EIGHTH`），CI 共 19 项：16 个 server 的 leg（race、bootstrap-1 到 bootstrap-8、costs-1 到 costs-3、costs-lint、guards-core、guards-sources、short），加 web、e2e、image。"；同一条的"`make -j1 test-go-bootstrap-1` 到 `test-go-bootstrap-7`"改为"到 `test-go-bootstrap-8`"。
   - 测试的注释：`bootstrap/upgrade_tool_codes_test.go:15-16`"the seventh shard's TestUpgradeToolsErrorsCarryTheirCodes answers these"改为"the eighth shard's"；`bootstrap/permission_matrix_mcp_test.go:91-92`"upgradeToolCodes lists them, the seventh shard answers them"改为"the eighth shard answers them"。`permission_matrix_coverage_test.go:156` 与 `wiki/app/start_upgrade_test.go:439` 说的是 `TestUpgradePermissionMatrix`、`TestUpgradeStart`，它们留在第七份，不改。
   - 还没有新的测试。记录里写出八份各自的 `go test -race -list`，合起来等于全部、每个恰在一份。第 272 条在修复一轮把 `TestVirtualExport` 挪进第七、八份的模式（`BOOTSTRAP_SECOND` 去掉它），计数的注释照挪后的 `-list` 写（3.20）。
2. `server: the lint's runs key and answer an upgrade's check, and the second pass is asked on its own (M11/P2/S2)`：3.8；检查的类型（`domain.UpgradeCheck` 等，3.10）随 `RunResult` 提前到这个提交（`wiki/domain/upgrade_check.go`），判定与报告仍在第 4 个（S2 的实施）。
3. `server: the page module reads an upgrade's gates (M11/P2/S2)`：3.9（模块根的四个方法与 `ReservedKeys`，测试是它们的调用者）。
4. `server: an upgrade's gates and report are judged (M11/P2/S2)`：3.10 的 `wiki/domain`。
5. `server, api, web: an upgrade's check and its rules' items (M11/P2/S2)`：3.11、3.13 的两个操作与 schema、HTTP 适配器、接线；`checkUpgrade`、`listUpgradeCheckItems` 的矩阵的行；`TestUpgradeCheck`、`TestUpgradeGates`（`TestUpgradeCheck` 的 MCP 一半在第 7 个提交，两个工具那时才有）；7.2 的另配的应用与第 8 条。
6. `server, api, web: an upgrade's submission keeps its report (M11/P2/S2)`：3.12、`Upgrade.report`、`wiki.upgrade_not_ready`；网页的 `report: null` 与两处文字（4.2）；`submitUpgrade` 的矩阵的行；`TestUpgradeSubmit`（MCP 的一半与第一个事务的期限一格在第 7 个提交）、`TestUpgradeSubmitInterleavings`（第 5、6、7、15 条、降级、检查跨过一次提交）、`TestUpgradeStatesReachTheStreams` 的提交两格；日志（3.16）；`wiring.submitTimeout`。
7. `server, api, web, e2e: the agent checks and submits the upgrade (M11/P2/S2)`：3.14、3.15 的 MCP 部分；`mcpserver` 的 `destructiveHint` 与 `TestTheAnnotationsAreTheHints` 的一格、两处注释；"server update"（`events.yaml` 改了，所以带 `web`）；`TestUpgradeToolsMatrix`、`coveredMCPRows()`；`upgrade_submit` 的两行码、`upgradeToolCodes()` 的两对与 `TestUpgradeToolsErrorsCarryTheirCodes` 的情形；工具表的长度驱动的两处同在这个提交：第三份的 `validToolArguments` 与 e2e LW8 的工具数（16、18 → 18、20），所以范围带 `e2e`（S2 的实施，第一轮门禁才查出）；`TestUpgradeCheck`、`TestUpgradeSubmit` 的 MCP 一半与第一个事务的期限一格也在这里。
8. `server, build, ci: an upgrade's check of fifty thousand pages takes its time, its reads plan by their keys (M11/P2/S2)`：7.1 的两个守卫（闸门的读的计划，检查的时长）；lint 的成本夹具的页数做成参数；`rollbackReadProblems` 加 `changesets` 与 `TestTheRollbackGuardJudgesThePlans` 的合成的一格。成本分份的说明（行号是 `f002672b` 的，S1 之后的文字）：
   - `Makefile`：`test-go-costs-2` 末尾加检查的守卫一行，说明改为"原成本命令第 14–20 条、LinksReached 物理守卫、升级的发起、检查与两种护栏的读（CI 第二份；需要 Docker）"；`test-go-costs-3` 在 `TestARollbacksReadsTakeTheirTimeAtAnyStatistics` 那一行（`:392`）之后加计划的守卫一行，说明（`:365`）"原成本命令第 21–32 条与回滚的三个守卫（CI 第三份；需要 Docker）"改为"原成本命令第 21–32 条、回滚的三个守卫与升级闸门的读的计划（CI 第三份；需要 Docker）"。
   - `ci.yml`：costs-2 的时限 S1 已改（45 分钟，第 267 条提前到 S1 的修复 `80da6b1f`，注释在那里）；costs 的注释末尾加"M11/P2/S2：costs-2 末尾加检查的守卫一条（5 万页，本机约 N 秒）；costs-3 在回滚的读之后加闸门的读的计划一条（本机约 N 秒）；现共42条（13/13/16），时限不变。"；加了这一条之后 costs-2 在 7763 上的作业用时与末行开始的时刻写进 S2 的记录（S1 的审查 B：慢三成的实例上卡住时离 45 分钟约 1 分钟）。修复一轮照第 275 条把 costs-3 的时限从 35 分钟提到 40 分钟，注释写 run 号、runner 与 `-timeout 20m` 那一行开始的分钟数（审查 P2S2B-M2，范围 `ci`）。
   - `06-environment.md`：同 S1，不改。
9. `e2e: UP5 over the API and MCP (M11/P2/S2)`：第 5 节；`expectUpgrade` 的加项。
10. `docs: …`（第 8 节第 4、5 条）。

### 6.3 S3（`m11-p2-s3`）

实施照这个次序，第 1–4 个是 `dc8a135a`、`1c070620`、`08f858d8`、`f0ba987d`；L1（S2 交下的 e2e 旧偶发）没有站得住的只改测试的改法，没有另一个提交（第 277 条）；之后修复一轮 6 个、第二轮 1 个（10.3），连实施的 4 个一共 11 个；第 5 个是本 Step 的文档提交，在它们之后（3.21）。

1. `web: a health rule reads its pages, knows its scope is gone and its heading's level through its caller (M11/P2/S3)`：`HealthRule` 的 `read`、`moved`、`level`；`HealthItem` 的参数类型；健康度页照旧。
2. `web: the upgrade page checks the upgrade and reads its rules' items (M11/P2/S3)`：这一次进入（`nonce`、`control`、自己的 SWR 缓存，`UpgradePage` 以 `key={location.key}` 渲染 `UpgradeRead`）、外层的读（`rereadNotebooks`、`pages`）、视图从 `data` 推出、`check`、`checkItems`、`UpgradeCheckSection`、文字、假服务端的两条路由、`upgrade-check.test.tsx`。
3. `web: the upgrade page submits the upgrade and shows its report in review (M11/P2/S3)`：`submit`、`submitUpgrade`、对话框、自己的提交在算的那一段（第 264 条）、`change` 的拒绝表、`UpgradeReportSection`、假服务端分两段的提交、`upgrade-page.test.tsx` 与 store 的加项；`upgrade-check.test.tsx` 加"检查在飞时自己提交"两格（两种次序，S3 的实施；它要"提交审阅…"与分两段的假提交，第 2 个提交里还没有；7.3 第 85 行的变异也在这个提交里跑，审查 P2C-N10）。
4. `e2e: UP5 on the page (M11/P2/S3)`。
5. `docs: …`。

### 6.4 合并的门禁

02-workflow 3.1：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`（经 `docs/v0.1/claude_to_codex/tools/heavy.sh`）；7.3 的每个变异；7.6 要量的；分支的 CI 全绿。合并之前的修复提交只改成本或守卫测试本身、且该测试只由一行跑时，照决定 242。

## 7. 测试与验证

### 7.1 各项与它们的测试

| 项 | 测试 | 在哪里 | Step |
|---|---|---|---|
| 正文的切分 | `TestTheUpgradeBodyShapes`（3.3 的形状表） | `page/adapter/markdown` | S1 |
| 守卫：移动与改名、正文、`Reached` 的码（带标记的基线页一律 `batch_rewrite`，不是基线页的带不带都放过） | `TestTheFreezeGuard` 的加项（3.3） | `page/app` | S1 |
| 单元的每一步带 `keepLinks` | `TestTheUnitStepCarriesKeepLinks`（3.2） | `page/app` | S1 |
| 单元算 `BodyHash`（不为 `Rewrite`）、交 `BeforeHash`、写账本 | `TestTheUnitHashesTheBodyWhileUpgrading`、`TestTheUnitKeepsTheLedger` | `page/app` | S1 |
| 升级中带 `Reached` 的字节不变的写也过守卫、什么也不写（第 268 条） | `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds`（3.3） | `page/app` | S1 |
| 升级中盲写计进读的界；单元看到的状态与计划时的不同就重新计划 | `TestTheReadsBoundCountsBlindWritesWhileUpgrading`、`TestTheBatchPlansAgainOnceTheUpgradeStarted`（3.3） | `page/domain`、`page/app` | S1 |
| 计划的 `Reached` 与批的标记 | `TestThePlanTellsAPageAMoveReached`、`TestTheBatchMarksABodysRefusalAtTheLast` | `page/domain`、`page/app` | S1 |
| 升级中照批前的链接判"到达过"（第 270 条） | `TestTheBatchJudgesAPageByItsLinksBeforeIt`（3.4） | `page/app` | S1 |
| 两条新读写 | `TestUpgradeBaselineContent`、`TestAcceptUpgradeContent`（`before` 等、不等、为空） | `page/adapter/postgres` | S1 |
| 护栏经 MCP | `TestUpgradeGuardrails` 的 g1–g27（3.6） | 第七份 | S1 |
| MCP 的码 | `TestUpgradeToolsErrorsCarryTheirCodes`（S1 六种，S2 两行的全部）；第五份的 `TestTheMCPToolsErrorsCarryTheirCodes` 照旧减去 `upgradeToolCodes()` | 第七份（S2 起第八份）、第五份 | S1、S2 |
| `Runs` 的键、结果、日志的名字；第二遍另成一项 | `TestTheUpgradeKeysJoinOnlyTheirOwn`、`TestTheRunsCopyACheck`、`TestTheUpgradeKindsAreNamed`；linking 的 `TestTheWantedCountsByPage` 两格 | `wiki/app`、`linking/app` | S2 |
| 闸门的读 | `TestTheUpgradeGateReads`（含 `not_kept`：不在的节点、正文不是账本的页）、`TestTheUpgradeRoots` | `page/adapter/postgres`、`page` | S2 |
| 判计划的帮手认得 `changesets` | `TestTheRollbackGuardJudgesThePlans` 加一格手写的计划：`BitmapAnd` 里有一个没有条件的部分索引，判为有问题（审查 P2B-M1） | `page/adapter/postgres` | S2 |
| 判定与报告 | `TestJudgeUpgrade`（闸门 7 的附件三格经 `withAttachment`，在修复一轮加，审查 P2S2A-M2）、`TestJudgeUpgradesCounts`（计数，S2 从 `TestJudgeUpgrade` 拆出；封顶的 `wanted` 一格在修复一轮加，七个值互不相同的一格在修复第三轮加）、`TestReportOf`（修复第四轮整值比）、`TestTheGatesFailInTheirOrder` | `wiki/domain` | S2 |
| 报告的存与读 | `TestTheReportRoundTrips`、`TestTheWorstReportFits`；`CurrentUpgrade` 的测试加两列 | `wiki/adapter/postgres` | S2 |
| 报告的条目与 lint 的条目 | `TestTheReportItemsAreTheLintItemsButTheirCursor` | `wiki/adapter/http` | S2 |
| `wanted_now` 的封顶 | 判定、REST 的投影、存与读、MCP 的投影各一个封顶的夹具（审查 P2S2C-M2，修复一轮）；REST 与存与读两个方向各一个夹具，`wanted_before` 与 `wanted_now` 一个封顶、一个不封顶，两个成员分得开（审查 P2S2C-M4，修复第二轮） | `wiki/domain`、`wiki/adapter/http`、`wiki/adapter/postgres`、`wiki/adapter/mcp` | S2 |
| 检查与报告的每个投影整值断言 | REST 的 `TestTheCheckIsAnswered`（检查与规则页）、`TestTheReportIsAnswered`（报告与带报告的视图）；MCP 的 `TestTheCheckIsAnsweredWhole`、`TestTheCheckFits`；存与读的 `TestTheReportRoundTrips`；判定的 `TestReportOf`：夹具里每个标量互不相同（`count_capped` 有真有假、与值无关，规则的数不等于条目数），八道闸门与 `passed` 排成四行（`gateRows`，每道闸门那一列互不相同、都不全真全假），各投影四行各过一遍；整个答复与 JSON 字面或写死的结构体比（审查 P2S2C-M5、M6，修复第三、四轮；对调与常量的变异 555 条全死；REST 报告条目的 `kind` 与规则页的 `mode` 两种值各有一项在修复第五轮补，审查 P2S2C-N9） | `wiki/adapter/http`、`wiki/adapter/mcp`、`wiki/adapter/postgres`、`wiki/domain` | S2 |
| 检查的码、快照、交付前的判定 | `TestCheckUpgradeAnswersInOrder`、`TestTheCheckReadsInOneSnapshot`、`TestTheCheckAuthorizesBeforeItsAnswer`、`TestTheCheckRefusesAnotherUpgradesSnapshot` | `wiki/app` | S2 |
| 提交的码、脱离、两个事务、期限、判定在 `COMMIT` 之前 | `TestSubmitUpgradeAnswersInOrder`（修复一轮加每格一道闸门不过的七格，审查 P2S2A-M1）、`TestTheSubmissionDetaches`、`TestTheSecondTransactionWritesOnlyItsOwn`、`TestTheSubmissionsDeadline`（修复一轮加钉住期限的 503 的秒数的几格，第 274 条）、`TestTheSubmissionsSnapshotIsItsOwn`（提交在快照里的核对，修复一轮，审查 P2S2A-M3）、`TestTheSubmitterIsDecidedBeforeTheAnswer`（3.12） | `wiki/app` | S2 |
| 注解 | `TestTheAnnotationsAreTheHints` 加 `{Annotations{}, {"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}}` 一格（3.14，审查 P2C-N1） | `platform/httpserver/mcpserver` | S2 |
| 检查经整个程序 | `TestUpgradeCheck`：3.11 的码依次（REST 与 MCP 两种入口各答同样的码与次序，13.4 第 3 条）；条目的游标：普通 `lint` 的游标答 400；上一次升级的游标，那次的根是发起时补写、已随回滚删去的 `raw`，答 400（根不同）；已有的 `raw` 先后被两次升级提议，上一次的游标被接受（绑定看根、不看升级，审查 P2B-N1）；`limit` 越界 422；名额占满 503（标准应用的一个名额，`heldRules` 停住的那一次占着它）；日志的 `query_kind` 与 `upgrade_scope`；`reviewing` 时也答 | 第八份 | S2 |
| 闸门经整个程序 | `TestUpgradeGates`：每道闸门过与不过各一格（不过的经发起人的 agent 或 SQL 造出：根下的 `AGENTS` 只有 frontmatter；来源根改名为 `AGENTS`；根下的 `index.md` 附件；一页缺 `summary`；一个 `sources` 指向区外；抵消的两页（第 180 条）；以 SQL 绕过单元改一页基线页的正文（闸门 7，第 191 条，`node_ids` 是那一页；只有它杀得掉"闸门 7 不比账本"）；以 SQL 把一页的索引记旧）；一页在发起时的 `raw` 里、带一个解析不到的链接，agent 把它移出来，闸门 6 照过（第二遍记下的 1，核对 B2-I1）；一页基线页移进 `raw`、它的待建变多，闸门 6 照过（第二遍不进检查）；七个数各有一格；以 SQL 绕过单元删一个基线附件，`preserved` 整值 4/3/2/2、`node_ids` 恰是它，断言之后还原（组合根把 page 的 `Preserved` 转成 wiki 的那一行把 `NodesKept` 写成 `Nodes` 时，原有的格都照过；审查 P2S2C-M7，修复第五轮）；这一格与"以 SQL 把一页的索引记旧"那一格的还原写在 `check` 里，`tm.checked` 先 `Fatal` 时被跳过，后面各格跟着红，以后改这个测试时两格的 `change` 里另用 `t.Cleanup` 注册同一句幂等的还原（审查 P2S2C-N11，不改）；每格结束 `checkPages`、`checkLinks` | 第八份 | S2 |
| 提交经整个程序 | `TestUpgradeSubmit`：码依次（404、403、`upgrade_state` 的三种、"正在计算"、`stale_index`、`not_ready`（一页缺 `type` 与只有 `AGENTS` 不过各一格，变异 o1），后两种之后升级回到 `upgrading`、提交的列为空、`mode` 两次）；通过：`reviewing`、报告、`mode` 两次（第一个事务与存下报告，第 216 条）、一行 info 的日志。**另配的应用**（7.2）上：客户端在计算中断开（`heldRules` 停住、取消请求的 ctx、放开），报告照样存下、日志的 `outcome` 是 `stored`；计算答错误（`heldRules` 让规则答错误）之后第二个事务退回 `upgrading`、`outcome = failed`、可以再提交。**期限一格**（审查 P2A-I1、P2A-M1、P2C-I2；标准应用，`wiring.submitTimeout` 6 s）见下 | 第八份 | S2 |
| 提交的交错 | `TestUpgradeSubmitInterleavings`（7.2） | 第八份 | S2 |
| 状态到流 | `TestUpgradeStatesReachTheStreams` 加提交的两格、各两帧：不过的提交（第一个事务一帧、退回一帧），补好之后通过的提交（第一个事务一帧、存下报告一帧）（S2 的实施；初稿写"三格"） | 第七份 | S2 |
| 矩阵 | `TestUpgradePermissionMatrix` 的三个操作（3.15）；`TestUpgradeToolsMatrix`；覆盖的检查改读 `coveredMCPRows()` | 第七份、第八份、第三份 | S2 |
| 工具的契约、说明 | `TestTheToolListsAreTheContract`、`TestTheWikiToolsContract`、`TestTheBuiltInInstructionsAreShort`、`TestTheBuiltInInstructionsNameEveryTool`；`wiki/adapter/mcp` 的 `TestTheCheckFits` 与工具数 | 第五份、`wiki/adapter/mcp`、`page/adapter/mcp` | S2 |
| 网页 | 4.3 的 vitest（S3：新的 `upgrade-check.test.tsx` 22 个，`health-rule.test.tsx` 加 7 个，其余在 `upgrade-page.test.tsx`、`notebook.store.test.ts` 与 i18n 的测试里；"检查在飞时自己提交"两种次序各一格，提交成功一格持住升级的第二次起的读，假服务端的 503 是名额满的那一种，3.21；修复一轮补的各格见 4.3 的 vitest，审查 P2S3） | `web` | S2、S3 |
| 故事 | UP4（S1）、UP5（S2 的 API 与 MCP，S3 的页面） | e2e | S1–S3 |

**`TestUpgradeSubmit` 的期限一格**照默认配置的比例造期限。默认配置下两个入口的请求期限（15 s；MCP 的工具调用是读完请求体之后的 `RequestTimeout`，3.12 第 3 步）都早于 `submitted_at + SubmitTimeout`（17.5 s 之后）。标准应用的请求 4 s，`wiring.submitTimeout` 设 6 s：两个入口的 4 s 都早于 `submitted_at + 6 s`，比例同默认（初稿以为 MCP 另加 `BodyReadTimeout` 5 s、9 s 才到，晚于它；第 273 条，审查 P2S2A-M4）；6 s 大于两倍的计算时长（`Do` 只等余下的一半），`Do` 的预算仍约 2 秒。REST 与 MCP 各一次，依次：

1. `heldRules` 停住提交的计算（`held`；窗口只有下面两步，远短于 `Do` 的预算）；
2. 这时计算停住、笔记本行没有人持：另一位管理员提交，第一个事务读到"审阅中、没有报告"、未过期，答 409 `wiki.upgrade_state`、`detail` 是"正在计算"的那句，不建 flight（一次短请求，远短于 `Do` 约 2 秒的预算）；
3. 测试在自己的事务里持住笔记本行（`SELECT … FOR NO KEY UPDATE`；第一个事务已提交，行是空的）；
4. 放开计算；`pgtest.WaitForLockWaits` 等到第二个事务在锁上等；
5. 过期限：REST 与 MCP 都在各自请求的期限答 503 `server_busy`（MCP 是工具的错误），早于 `submitted_at + 6 s`（期限取了较早的那一个）；`Retry-After` 是到能接管为止的秒数（第 274 条，修复一轮；实施时是 1）：秒数随服务端答复的那一刻差一秒（本机 REST 2 秒、MCP 3 秒），所以断言它不小于从收到答复到 `submitted_at + 6 s` 的秒数（向上取整、至少 1），收到答复之后再恰等那么多秒，让第 6 步的接管成功；精确值由单元 `TestTheSubmissionsDeadline` 钉住。各 warn 一行，什么也没写：行在"审阅中、没有报告"，`submission_id` 是这一次的；
6. 测试放开锁；过了 `submitted_at + 6 s`，另一位管理员提交成功（接管）：报告存下，`submission_id` 是第二次的（7.2 第 7 条的"另一次提交"）。

"正在计算"的探测放在第 2 步、两个入口一样（审查 P2A-M5、P2C-M5）：测试持着笔记本行时，另一次提交的第一个事务要在同一把锁上等，等到自己的请求期限答 500，或等到放锁时已过期限、成了接管。两个入口第 5 步都在请求的期限答，之后到 `submitted_at + 6 s` 之间再提交答"正在计算"，那时什么也没在算，第 274 条的 `Retry-After` 因此取到能接管为止（初稿另写"MCP 那一次第 5 步恰在 `submitted_at + 6 s` 才答"，照第 273 条不对）。

另一格：第一个事务落在请求的期限上，同别的写（平台答 500、warn 一行），什么也没写。

**成本守卫**（13.4 第 8 条；`-short` 跳过、`!race`；统计状态 × 计划照 05-lessons 第 4 节；计划为主、时长为辅，决定 236、240、244；种子用 SQL，库里另种别的笔记本的行）：

| 守卫 | 规模 | Step | 判什么 |
|---|---|---|---|
| 护栏的读（`TestUpgradeGuardrailReadsTakeTheirTimeAtAnyStatistics`，`page/adapter/postgres`，`test-go-costs-2` 一行，紧接 P1 的删除护栏的读） | 照 P1 的删除护栏的夹具：两次升级各 10 万行基线（这一次的与另一本笔记本的），另种九本各 5 万页的笔记本作不该读的行；读 1,000 个基线页的 `UpgradeBaselineContent`、写 1,000 次 `AcceptUpgradeContent`（`before` 等于账本） | S1 | 三种统计状态（从未分析、分析过、只有另一次升级的统计）× 自定义与通用两种计划（第四种 `force_generic_plan` 就是通用计划那一列，3.19），每格先判计划再计时：`upgrade_nodes` 只经 `upgrade_nodes_pkey`、两列都在索引条件里；`page_contents` 只经 `page_contents_pkey`、在嵌套循环的内侧以外行的 `node_id` 读（`LATERAL … LIMIT 1` 不被拉成连接）；没有顺序扫描，一格计划有问题就停在计时之前。时长：每次读、每次写的中位数，界取本机两轮较大的中位数 × 3、向上取整到 5 ms |
| 护栏在界上的单元（`TestUpgradeGuardrailsAtTheBatchBoundTakeTheirTime`，组合根，`test-go-guards-core` 一行，紧接 `TestABatchAtItsRewriteBoundTakesItsTime`） | 升级中的笔记本上由发起人的 agent 跑五种（审查 P2A-M4）：a. 一次改名改写 M9 的界上的那么多基线页（`MaxBatchRewrites` 页、每页 84 KB 与 200 条链接，`page/domain/batch_plan.go:33-45`；系统改写：不比正文，每页一次账本的写）；b. 一个 `batch` 的 100 个编辑各删掉基线页 frontmatter 里一行约 62 KB，这些页的 head 合计约 13.8 MiB、恰在 `MaxBatchReadBytes`（14 MiB）之内，最终约 8 MiB（计划读一次、守卫再读一次、两次哈希；照字面的 140 KB 编辑过 `MaxUnitBytes`，单元不会跑，3.19）；c. 一页 `MaxContentBytes` 的基线页改 frontmatter（一个 `edit` 操作的 `batch`）；d. 一个 `batch` 盲写基线页、各删掉一块大的 frontmatter，head 合计恰在 14 MiB 之内（两页 5 MiB、一页约 4 MiB）：升级中放得进的最大的单元；e. 100 个盲写、各删掉一页 5 MiB 基线页的大 frontmatter（A 实测本机 1.58 秒的那一种）：现在由计划答 `page.batch_too_large`，什么也没写。b–e 的页以 SQL 种下、每轮之间以 SQL 写回（e 的 100 页 × 5 MiB 一条 `generate_series`，lz4 之后约 207 MiB）；批的 bench 给 `/mcp` 以 wire 的请求体上限 | S1 | 单元持着笔记本行：a–d 每种的单元短于 14 s（批的 bench 是生产的 15 秒请求，留 1 秒，3.19；决定 153、243 判最大的单元；不取三倍的规则作上限），三倍的规则另作回归的界（本机两轮较大的中位数 × 3、向上取到 100 ms：a 5.2 s、b 1.1 s、c 200 ms、d 100 ms、e 100 ms；a 的界守的主要是 M9 的改写，护栏多出的约 15–65 ms 在抖动之内，护栏的那一份由 b–e 量，审查 P2S1B-N1）；e 答它的码，用时在三倍的界之内（只有计划，不进单元）；同一形状在普通笔记本上的中位数一起打出，增量写进记录 |
| 闸门的读的计划（`TestTheUpgradeGateReadsPlansAtAnyStatistics`，`page/adapter/postgres/upgrade_cost_test.go`，与帮手 `rollbackReadProblems` 同包，写法同 P1/S3 的 `TestARollbacksReadsTakeTheirTimeAtAnyStatistics`；`test-go-costs-3` 一行，紧接它；审查 P2B-N4） | 以 SQL 种下一本 10 万页的笔记本与它的升级的 10 万行基线（各带 `parent_id`，页带 `wanted`）、1,000 个本次升级的变更集、这本笔记本的 10 万个普通变更集；另九本各 5 万页的笔记本与它们的升级的基线行、各 1,000 个变更集（决定 240，审查 B 的种子） | S2 | `UpgradeGateBaseline`、`UpgradePreserved`、`UpgradeChangesets` 在三种统计状态（从未分析、发起之后分析过、只有别的升级的统计）× 自定义与通用计划下：`upgrade_nodes` 只经 `upgrade_nodes_pkey` 的前缀、索引条件含 `upgrade_id`；`nodes`、`page_contents` 只经主键、在嵌套循环的内侧以外行的键读；`changesets` 只经 `changesets_upgrade_id_idx`（帮手的关系表加 `changesets`：`Index Scan`、`Bitmap Heap Scan` 之下的每一个 `Bitmap Index Scan` 都是它，条件里含 `upgrade_id =`）；在内侧逐行重复的扫描必须经外行的键，不许逐行重读整个集合（非 hashed 的 SubPlan、内侧的 `Materialize`、`CTE Scan`）。`UpgradePreserved` 的预排序聚合的 `Sort` 只在这次升级的行上，不判为问题（3.9）。修复一轮另判每个 `Relation == "nodes"` 的扫描都经 `nodes_pkey`、条件含 `id = `，在计时之前：帮手对 `nodes` 认任何以 `notebook_id =` 为条件的索引，内侧每行按笔记本读整本的 `nodes` 它看不出（审查 P2S2B-M1；帮手本身不改）。这一判法由不要数据库的 `TestTheGateReadsReadANodeByItsKey`（`make test-go-short` 跑到）以五格手写的计划钉住：按键、按键的位图过，按外行的笔记本、按笔记本与 `parent_id`、Seq Scan 不过；它也断言共用的 `rollbackReadProblems` 放过前两种不过的，以写明补的是哪个口子，以后收紧帮手时这一段要一并改（审查 P2S2B-N2，不改）。一格计划有问题就失败；每条读的中位数带 `-v` 打出，不设时长的界，时长由下一行判 |
| 检查（`TestUpgradeCheckTakesItsTimeAtAnyStatistics`，组合根，`test-go-costs-2` 的末尾，与发起的守卫之间隔护栏的读一行，S2 的实施） | 第 262 条：5 万页（审查 P2B-M2）。lint 的成本夹具（`bootstrap/wiki_lint_cost_fixture_test.go:41`）的页数做成参数（lint 的守卫照旧 10 万），这里取 5 万、改成普通笔记本、以 SQL 设成升级中：5 万行基线（各带 `parent_id`，页带 `wanted`）、1,000 个本次升级的变更集；另种九本各 5 万页的笔记本与它们的升级的基线行、各 1,000 个变更集，这本笔记本另有 10 万个普通的变更集（决定 240，B 的种子）；配置照 lint 的守卫（请求 6 s、worker 3 s，`bootstrap/wiki_lint_cost_test.go:31-34`，M10 决定 160）；另一份种子里闸门 7 不过（S2 以 SQL 把一页的账本记成别的哈希：正文与账本不一致的两种写法对闸门 7 一样，不用再种一份 5 万页） | S2 | 只量时长，计划由上一行判（组合根导入不了 `page/adapter/postgres` 测试包里的帮手，审查 P2B-N4）：整个检查（`Module.CheckUpgrade`，经 `Runs`）在两份种子上（发起之后分析过）各三格的中位数，界取本机两轮较大的 × 3（向上取整到 100 ms）与 worker 的 80%（2.4 s）中较小的（B 的建议）。S2 取 1.5 s（`checkCostBound`：本机两轮较大的中位数 489 ms × 3）。估计：B 实测十万页的检查另加的读并进 `not_kept` 之后本机约 0.31–0.34 秒（lint 摘要 0.785 秒），5 万页约一半；7763 与本机之比 2.2–2.6，折到 7763 约 1.3–1.5 秒，在 2.4 s 之内 |

`Makefile` 的四行各带一行注释（规模、判什么、本机的数），同 P1 的写法；成本分份的说明随之改（6.1 第 5 个提交、6.2 第 8 个提交写出旧文字与新文字），costs-3 照旧 35 分钟（修复一轮照第 275 条提到 40 分钟，审查 P2S2B-M2）；guards-core 与 costs-2 由 S1 的修复（`80da6b1f`）从 30、35 分钟提到 40、45 分钟（第 269 条；第 267 条提前到 S1，审查 P2S1B-M1，7.5）。

### 7.2 交错

00 号文档 4.16 里 P2 的五个（第 5、6、7、8、15 条）与两个补充，各一个确定的测试：持住双方都要的行，`pgtest.WaitForLockWaits`（`platform/postgres/pgtest/lockwait.go:25`）等到另一方在等，再放开；或以 `heldRules`（`bootstrap/upgrade_interleavings_test.go:45-58`，包住 lint 的规则，装上之后在下一次准备的 `PrepareRules` 里停住，快照已取、名额已占）停住一次计算。先后记在共用的日志里；结束时 `checkPages`、`checkLinks`、`checkSearch`。都在第八份的 `TestUpgradeSubmitInterleavings` 里，第 8 条在 `TestUpgradeCheck` 里。

**名额与期限**（审查 P2C-I2、P2A-M1）。

- **约束**：
  - 标准的测试应用请求 4 s（`bootstrap/app_test.go:63`）、`LintTimeout` 2 s、一个名额（`bootstrap/wiki_lint_runtime_test.go:69` 断言）。
  - `Runs.Do` 的调用者只等 `min(LintTimeout, 余下的期限/2)`（`wiki/app/lint_runs.go:66-77`），到了答 `server_busy`，不等 worker；flight 自己的期限是建起之后的 `LintTimeout`（`:91`）；名额在 worker 返回之前不还。
  - `heldRules` 只停得住计算，停不住 `Do` 的等待。所以停住的窗口必须短于被停一方的 `Do` 预算（标准应用里检查约 1.9 秒、提交约 2 秒），不然被停的一方答 503，而不是它自己的结果。
  - 要等的一方以 `RunObserver`（`wiki.Deps.RunObserver`，照 `wiki_lint_runtime_test.go:76` 与它的 `event`）等到它的 flight 已建（`RunCreated`）再放开，不靠睡眠（05-lessons 第 2 节；`main` 上的 `d08ae0e4`、`dc3c623d` 修的正是这类）。
- **另配的应用**：第 6、7（放弃）、8、15 条、降级、"检查跨过一次提交"，以及 `TestUpgradeSubmit` 停住计算的两格（断开、规则答错误），都在它上面跑。
  - 配置：`newAcmeTeamOn`（`bootstrap/interleavings_workspace_test.go:98`）的配置钩子设 `Server.RequestTimeout` 20 s、`Server.WriteTimeout` 60 s（要大于 `BodyReadTimeout + RequestTimeout`，`platform/httpserver/api.go:77`；照 `bootstrap/mcp_batch_test.go:270`）、`Database.MaxConns` 8。于是 `LintTimeout = min(10 s, 20 s/2) = 10 s`（`bootstrap/wire.go:246`），`MaxLintRuns = max(1, 8/2/2) = 2`（`wire.go:256`），`SubmitTimeout` 20 s。`wiring.wiki` 断言这两个数（照 `wiki_lint_runtime_test.go:69`）并装上 `RunObserver`。
  - 检查与提交的 `Do` 预算约 9.9 秒，停住的窗口（几次请求）有十倍以上的余量。
  - 每条停住的一方与等名额的一方各占一个名额：第 6 条停住的检查占一个，提交的计算占另一个；第 8 条要检查真的排队，两个名额都由停住的条目读占着；其余各条只停一方，另一个名额空着。
- **仍用标准应用的**：`TestUpgradeCheck` 的 503 一格（一个名额，停住的那一次占着它，另一个检查约 1.9 秒后答 503）；`TestUpgradeSubmit` 的期限一格（它要的是默认配置的比例，7.1）；第 5 条（不停计算）。

| # | 交错 | 怎样逼出 | Step |
|---|---|---|---|
| 5 | 提交的第一个事务与发起人正在跑的 `batch` | 标准应用。照 `acmeTeam.interleaveOn`（`bootstrap/interleavings_workspace_test.go:191`）：`batch` 先持笔记本行，提交等它，存下的报告看得到它写的（一页补上 `type` 之后 `missing_type` 少一）；提交先（S2 以 `interleaveOn` 持住笔记本行、批的命令一步，不另包 wiki 的 `States`；初稿写照 P1 第 14 条包住 `States`、停在 `SetState` 之后），`batch` 等笔记本行，之后答 `notebook.upgrading`、什么也没写 | S2 |
| 6 | 检查的快照已取 → 发起人的写提交 → 提交的第一个事务 → 放开 | 另配的应用。`heldRules` 装上，管理员检查，等它停住（`held`）：它的快照取在写之前，占一个名额；发起人的 agent 补上最后一页的 `summary`；管理员提交：第一个事务、`mode`；提交的 flight（键带提交的 id）建起（`RunCreated`），拿另一个名额算完，存下的报告看得到那一次写、`passed` 为真，答 200；放开：先停住的那次检查答它自己的旧结果（快照里是 `upgrading`，合法；`passed` 为假）。变异"提交并入已有的检查"在这里失败：提交只有 `RunJoined`、没有自己的 `RunCreated`，等它的那一步失败 | S2 |
| 7 | 提交的两个事务之间的放弃与另一次提交 | **放弃**（另配的应用）：`heldRules` 停住管理员 A 的提交的计算；管理员 B 放弃（`rolling_back`，回滚的任务排上）；放开：第二个事务读到别的状态，什么也不写，答 409 `wiki.upgrade_state`，日志的 `outcome` 是 `superseded`；回滚照常做完（读放弃之后的行不看回滚到了哪一步：`state` 在 `rolling_back` 或 `rolled_back`、提交的三列还在、没有报告，之后照旧等回滚的作业 `succeeded`；回滚的 worker 常在测试读之前就做完，S2 的实施）。**另一次提交**：就是 7.1 `TestUpgradeSubmit` 的期限一格，不另造。第一次的第二个事务在测试持住的笔记本行上等到请求的期限（两个入口都是，第 273 条），答 503、什么也没写；放开锁、过了期限之后，第二次提交接管、存下报告，`submission_id` 是第二次的。`heldRules` 停住计算造不出这一种：第一次的 `Do` 在 `submitted_at + SubmitTimeout/2` 前后就答 `server_busy`，它的第二个事务在期限之内退回 `upgrading`，已没有"审阅中、没有报告"可接管（审查 P2A-M1、P2C-I2）。提交的 id 的核对另由 `TestTheSecondTransactionWritesOnlyItsOwn` 钉住：接管只在前一次的期限之后，期限照存下的 `submitted_at` 算（截到微秒），前一次的第二个事务那时已到期，所以整个程序里到不了"期限之内读到别的提交"（2.1 第 7 条） | S2 |
| 8 | 检查排队等名额时升级被放弃 | 另配的应用，两个名额都要占着：`heldRules` 依次装上两次，管理员读两条规则的条目（`listUpgradeCheckItems` 的 `missing_type`、`missing_summary`，键不同，两个 flight），各停住；管理员检查，它的 flight 建起（`RunCreated`）而拿不到名额，在 `Do` 里等；管理员放弃；放开两次：检查的 worker 拿到名额，在快照里读到 `rolling_back`，答 409 `wiki.upgrade_state`（审查 B-M3；批准的一半在 P3/S1） | S2 |
| 15 | 第一次提交在算、第二次提交 | 另配的应用。`heldRules` 停住第一次的计算；第二次（另一位管理员）的第一个事务读到审阅中、没有报告、未过期，答 409 `wiki.upgrade_state`、`detail` 是"正在计算"的那句，不建 flight；放开，第一次存下报告。期限之后的一半在 `TestUpgradeSubmit` 的期限一格 | S2 |
| — | 提交人在两个事务之间被降为写者 | 另配的应用。`heldRules` 停住计算；另一位管理员把提交人降为写者；放开：第二个事务写下报告，`COMMIT` 之前的判定答 403 `forbidden`；报告照样存下，info 一行、`outcome` 是 `stored`；升级停在审阅中、带报告（之后任何管理员都可以放弃，P3 的批准照常） | S2 |
| — | 检查跨过一次提交 | 另配的应用，两步（审查 P2C-I2）：① `heldRules` 停住一次检查（快照取在提交之前）；管理员提交，第一个事务把升级设成 `reviewing`，提交的 flight 建起（`RunCreated`）；放开检查：它答 `upgrading` 快照的结果（合法，不因提交而失败）。② 提交答 200、报告存下之后，再检查答 `reviewing` 的现算（`passed` 为真）。一个名额下也成立（提交的计算等检查放开），另配的应用只是给时间 | S2 |

### 7.3 变异

每个变异是实施时在本机对代码的一处改动，跑它旁边写的测试，要它失败（不提交；改回从备份复原）；第 10 节记下结果。第 84–86 行是修订核对第一轮之后加的，接在表尾、不重编前面的号（审查记录与对照里引的行号照旧）。S1 的审查之后照第 268 条改第 17–20 行（第 19 行即修复一轮的 n1）；表尾接 S1 的两轮修复与审查者留下的变异，照它们在 [S1 的审查记录](reviews/P2-S1-guardrails-review.md) 里的名字记、不编号（3.19）。S2 的审查之后，第 25、42 行的后一个测试改正归属，第 33、46、61 行照实施改；表尾再接 S2 的实施者的 o1、审查者留下的与修复新加的变异（修复第三、四轮的对调与常量的变异 72、555 条只记审查者点名的几条，全部见审查记录），照它们在 [S2 的审查记录](reviews/P2-S2-gate-check-submit-review.md) 里的名字记（3.20）。S3 的实施与审查之后，第 74、75、81、85 行照实改（第 74 行写成两半合起来，两半各自是防御、测不到；第 85 行换成审查 C 的 c1，设计原写的那一种在 SWR 2.5.1 与 React 18 下等价）；表尾再接 S3 的实施者自己的 14 条与审查者留下、修复一轮与第二轮杀的变异，照它们在 [S3 的审查记录](reviews/P2-S3-web-review.md) 里的名字记（3.21）。

| # | 变异 | 让它失败的测试 | Step |
|---|---|---|---|
| 1 | `guardrails` 不判移动与改名 | `TestTheFreezeGuard`；g5、g6 | S1 |
| 2 | 只看 `Options.UpdateLinks`、不看 `KeepLinks` | `TestTheFreezeGuard` 的 `KeepLinks` 一格；g13（批的单元 `UpdateLinks` 恒为真，只有 `KeepLinks` 说它不改写） | S1 |
| 3 | `step()` 不带 `keepLinks` | g13；`TestTheUnitStepCarriesKeepLinks` | S1 |
| 4 | 正文不去 BOM | `TestTheUpgradeBodyShapes` 的两个 BOM 的格 | S1 |
| 5 | 去掉开头的全部 BOM | 同上"两个 BOM 只去一个" | S1 |
| 6 | 比全文的哈希 | g1、g14、g15 的前一半；`TestTheFreezeGuard` 的"只改 frontmatter 放过" | S1 |
| 7 | `Rewrite` 的步也比正文 | g4；`TestTheFreezeGuard` 的 `Rewrite` 一格 | S1 |
| 8 | 账本的写不核对 `before`（升级中也无条件写，00 号文档原来的写法） | g17、g18（答 200、绊线被洗掉）；`TestAcceptUpgradeContent` 的不等一格 | S1 |
| 9 | 基线行在而不等时不答错（0 行一律当作不是基线页） | `TestAcceptUpgradeContent` 的不等一格；g17 | S1 |
| 10 | 回滚的单元也交 `BeforeHash` | `TestTheUnitKeepsTheLedger` 的 `rolling_back`（`before` 为空） | S1 |
| 11 | `BodyHash` 为空时放行 | `TestTheFreezeGuard` 的"`BodyHash` 为空" | S1 |
| 12 | 单元在 `reviewing` 也算 `BodyHash`；或在 `upgrading` 不算 | `TestTheUnitHashesTheBodyWhileUpgrading`；后者另有 g1（缺陷） | S1 |
| 13 | 单元为 `Rewrite` 的步也算 `BodyHash`（审查 P2A-N3） | `TestTheUnitHashesTheBodyWhileUpgrading` 的 `Rewrite` 一格 | S1 |
| 14 | `BaselineContent` 不限 `baseline`（保护行也比） | `TestUpgradeBaselineContent` 的保护行；g8 | S1 |
| 15 | `CheckReads` 不计盲写 | `TestTheReadsBoundCountsBlindWritesWhileUpgrading`；g22 | S1 |
| 16 | 盲写在没有升级时也计 | `TestTheReadsBoundCountsBlindWritesWhileUpgrading` 的"没有升级时过"；g22 的后一半 | S1 |
| 17 | `Reached` 取 `Moved` | `TestThePlanTellsAPageAMoveReached` 的"无关的移动"；g11 | S1 |
| 18 | 不传 `Reached`（`ContentWrite` 到 `StepContent`） | g9；g23 | S1 |
| 19 | 写的路径清掉标记（第 254 条的清除，第 268 条收回；修复一轮记作 n1） | `TestThePlanTellsAPageAMoveReached` 的"写别的字节"一类；g21；g23 | S1 |
| 20 | 写的路径照第 254 条的规则，只在写的是改写的原样时设（`rewrote && same`） | `TestThePlanTellsAPageAMoveReached` 的"写别的字节"一类；g27 的 Q 格（第 270 条之后 g21、g23 分不出：批前的链接先把 P 标上） | S1 |
| 21 | 正文的拒绝标在 `First` | `TestTheBatchMarksABodysRefusalAtTheLast` | S1 |
| 22 | 所有错误都标在 `Last` | 同上的"别的码仍标在 `First`" | S1 |
| 23 | 账本不在改写时更新（只在 `!Rewrite` 时写） | g4（账本不等于改写之后的哈希）；`TestTheUnitKeepsTheLedger` | S1 |
| 24 | 账本的写不限 `baseline` 与 `kind` | `TestAcceptUpgradeContent` 的保护行与附件 | S1 |
| 25 | `RunKey` 不带 `Scope` | `TestTheUpgradeKeysJoinOnlyTheirOwn`（初稿另写 7.2 第 6 条，那是第 26 行的：提交的键的 `Kind` 与检查的不同，去掉 `Scope` 也不并入） | S2 |
| 26 | 提交并入已有的检查（键用 `LintUpgradeCheck` 与升级的 id） | 7.2 第 6 条（提交没有自己的 `RunCreated`） | S2 |
| 27 | `Runs` 交给每个调用者同一个 `Check` | `TestTheRunsCopyACheck` | S2 |
| 28 | `logFinished` 不认新的种类 | `TestTheUpgradeKindsAreNamed` | S2 |
| 29 | 检查也跑第二遍（`AreaWantedByPage` 随 `WantedByPage`） | `TestTheWantedCountsByPage` 的"摘要带按页的数、不跑第二遍"；`TestUpgradeGates` 的"移进 `raw` 的基线页" | S2 |
| 30 | 发起不设 `AreaWantedByPage` | `TestUpgradeStart` 的第二遍（基线 `wanted` 是 1 而不是 0） | S2 |
| 31 | 闸门 6 改回全本的净数 | `TestJudgeUpgrade` 的抵消；`TestUpgradeGates` 的抵消 | S2 |
| 32 | 闸门 6 算新建页 | `TestJudgeUpgrade` 的新建页 | S2 |
| 33 | `MovedIntoSources` 不重建发起时的树（只数现在在区里的基线节点） | `TestJudgeUpgradesCounts`（S2 从 `TestJudgeUpgrade` 拆出）的计数"`raw` 原有、下面的两页发起时就在"（审查 P2B-N3；初稿的"闸门 6 不认封顶"没有可改的代码，删去） | S2 |
| 34 | 闸门 7 的分母取现读的行 | `TestJudgeUpgrade` 的"现读的行数少一行" | S2 |
| 35 | 闸门 7 不比账本 | `TestUpgradeGates` 的 SQL 绕过（只有它，第 191 条）；`TestTheUpgradeGateReads` 的"改正文而账本不变" | S2 |
| 36 | `UpgradePreserved` 的 `not_kept` 只收不在的节点、不收正文不是账本的页 | `TestTheUpgradeGateReads` 的"改正文而账本不变"（`not_kept` 是那一页）；`TestUpgradeGates` 的 SQL 绕过的 `node_ids` | S2 |
| 37 | `UpgradeChangesets` 带回 `deleted_at IS NULL` | `TestTheUpgradeGateReadsPlansAtAnyStatistics` 的"从未分析"一格（`BitmapAnd` 里没有条件的部分索引；审查 P2B-M1、P2B-N4） | S2 |
| 38 | `rollbackReadProblems` 不判 `changesets` | `TestTheRollbackGuardJudgesThePlans` 的合成的 `BitmapAnd` 一格 | S2 |
| 39 | 闸门 2 不查 `AGENTS` | `TestJudgeUpgrade` 的"来源根改名为 `AGENTS`" | S2 |
| 40 | 闸门 3 不限根下 | `TestJudgeUpgrade` 的"不在根下的 `index`" | S2 |
| 41 | 闸门 1 不切 frontmatter | `TestTheUpgradeRoots` 的"正文只有 frontmatter" | S2 |
| 42 | 索引一格不看 `stale_pages` | `TestJudgeUpgrade` 的一页过时（提交答 `stale_index` 看的是检查的 `StalePages`、不经闸门 8，所以初稿另写的 `TestUpgradeSubmit` 的 `stale_index` 抓不到它） | S2 |
| 43 | 快照里不核对升级的 id 与状态 | 7.2 第 8 条（两个名额由停住的条目读占着，检查排队之后读到 `rolling_back`）；`TestTheCheckRefusesAnotherUpgradesSnapshot` | S2 |
| 44 | 闸门的读不在快照里（在 `WithinSnapshot` 之外读） | `TestTheCheckReadsInOneSnapshot` | S2 |
| 45 | 检查交付之前不再判定 | `TestTheCheckAuthorizesBeforeItsAnswer` | S2 |
| 46 | 条目不核对游标绑定的根（改在 linking 的 `LintCursor.PositionFor`：用例只经它绑定游标，没有单独"核对根"的一句） | `TestUpgradeCheck` 的普通 `lint` 的游标与"上一次升级补写的 `raw`"的游标（`TestCheckUpgradeAnswersInOrder` 的假游标自己判根，看不到它） | S2 |
| 47 | 提交的第二个事务不核 `submission_id` | `TestTheSecondTransactionWritesOnlyItsOwn` | S2 |
| 48 | 第二个事务不核状态 | 7.2 第 7 条的放弃 | S2 |
| 49 | 提交的第二段用请求的上下文 | `TestUpgradeSubmit` 的断开；`TestTheSubmissionDetaches` | S2 |
| 50 | 第二段的期限从第一个事务之后算（00 号文档原来的写法） | `TestTheSubmissionDetaches` 的"请求的期限较晚"一格（恰是 `T + SubmitTimeout`） | S2 |
| 51 | 期限不取 min（只取 `submitted_at + SubmitTimeout`） | `TestTheSubmissionDetaches` 的"请求的期限较早"一格；`TestUpgradeSubmit` 的期限一格（REST 的 503 在 `submitted_at + 6 s` 之前到） | S2 |
| 52 | `at` 不截到微秒（期限用纳秒的 `at`，审查 P2A-N4） | `TestTheSubmissionDetaches`（`T` 是截到微秒的）；`TestSubmitUpgradeAnswersInOrder` 的"恰在 `submitted_at + SubmitTimeout` 时接管" | S2 |
| 53 | 第 6 步仍要求请求的 ctx 活着（初稿的条件） | `TestTheSubmissionsDeadline` 的"请求的 ctx 已到期"一格；`TestUpgradeSubmit` 的期限一格（REST 答 500） | S2 |
| 54 | 第二个事务的期限错误答 500 | `TestTheSubmissionsDeadline`；`TestUpgradeSubmit` 的期限一格 | S2 |
| 55 | 交付之前的判定在 `COMMIT` 之后、以 `detached` 读（初稿的次序） | `TestTheSubmissionsDeadline` 的晚到的 `COMMIT`（假的授权在 ctx 到期时报错，答 500 而不是 200） | S2 |
| 56 | 提交的交付之前不判定 | `TestTheSubmitterIsDecidedBeforeTheAnswer`；7.2 的降级 | S2 |
| 57 | info 日志只在判定不拒时记 | `TestTheSubmitterIsDecidedBeforeTheAnswer`（答 403 也有 `outcome = stored` 一行）；7.2 的降级 | S2 |
| 58 | 提交的日志记在第二个事务之前 | 7.2 第 7 条的放弃（日志写了 `stored`，应是 `superseded`；审查 P2C-M2） | S2 |
| 59 | 期限之内的再提交顶掉正在算的那一次 | 7.2 第 15 条；`TestUpgradeSubmit` 的期限一格的第 2 步；`TestSubmitUpgradeAnswersInOrder` | S2 |
| 60 | 不过时不退回（留在 `reviewing`） | `TestUpgradeSubmit` 的 `not_ready` | S2 |
| 61 | 存下报告不发 `mode` | `TestUpgradeStatesReachTheStreams` 的提交两格（通过的那一格） | S2 |
| 62 | 报告不去游标 | `TestReportOf`；契约的 `apitest`（`additionalProperties: false`） | S2 |
| 63 | `destructiveHint` 照旧只在真时写 | `TestTheAnnotationsAreTheHints` 的 `Annotations{}` 一格；`TestTheToolListsAreTheContract` | S2 |
| 64 | 说明少一行 | `TestTheBuiltInInstructionsNameEveryTool` | S2 |
| 65 | `upgrade_verify` 的 `Fit` 只截规则、不截闸门的路径 | `TestTheCheckFits` 的第二格 | S2 |
| 66 | MCP 的两行不进 `coveredMCPRows()` | `TestThePermissionMatrixCoversEveryTool` | S2 |
| 67 | `HealthRule` 的读写死成 lint 的 | `upgrade-check.test.tsx` 的"更多" | S3 |
| 68 | "范围变了"写死成 `notebook.not_llm_wiki` | `upgrade-check.test.tsx` 的 `wiki.upgrade_state`；`health-rule.test.tsx` 的 `moved` | S3 |
| 69 | `HealthRule` 的标题写死成 `h2`（不看 `level`） | `upgrade-check.test.tsx` 的"卡的标题是 `h3`"；`health-rule.test.tsx` 的 `level` | S3 |
| 70 | 打开页就读检查 | `upgrade-check.test.tsx` 的"按'检查'才读"（审查 P2C-N6） | S3 |
| 71 | 再检查不中止上一代 | `upgrade-check.test.tsx` 的再检查一格 | S3 |
| 72 | 检查的 `server_busy` 自动重试 | `upgrade-check.test.tsx` 的 `server_busy` 一格 | S3 |
| 73 | 检查的 403 不重读笔记本列表，或经这一次进入的 `mutate` 重读（碰不到全局的列表） | `upgrade-check.test.tsx` 的 403 一格 | S3 |
| 74 | 检查的缓存在公共的 provider、键又不带这一次进入（`nonce`）（两半合起来；初稿写"或"：这里每次进入一个新的 `Map`、`UpgradeRead` 按 `location.key` 重新装上，只不带 `nonce`、只离开时不清都碰不到下一次进入，是等价的变异 74x、74y；`nonce`、键里的 `locationKey` 与离开时清缓存是防御，测不到，照设计留着，审查 P2S3B-Q1；S3 的实施） | `upgrade-check.test.tsx` 的"检查一次、离开、回来"两格（回来之后再按"检查"读了新的一次）与"再点一次同一页的链接"（审查 P2C-M1） | S3 |
| 75 | 再检查只在检查一节挂着时读：检查一节卸着时要的一代，装回时不读（`revalidateOnMount: false`；初稿写"`recheck` 直接发请求，不经 SWR 的键"，这个结构里 `recheck` 只加一代，没有一处小改能让它自己发请求，S3 的实施） | `upgrade-page.test.tsx` 的提交的第一段，409 先到的那一格（检查一节卸着时 409 到，装上之后恰读一次；审查 P2C-I1） | S3 |
| 76 | `change` 的拒绝表不含 `wiki.upgrade_not_ready`、`wiki.upgrade_stale_index` | `upgrade-page.test.tsx` 的 409 先到的那一格（第二次 `mode` 留住时检查一节已回来） | S3 |
| 77 | 提交答 409 之后不再检查 | `upgrade-page.test.tsx` 的 `not_ready` | S3 |
| 78 | `not_ready` 关上之后焦点不回"提交审阅…" | `upgrade-page.test.tsx` 的焦点一格 | S3 |
| 79 | 审阅中仍给"检查"与"提交审阅…" | `upgrade-page.test.tsx` 的 `reviewing` | S3 |
| 80 | 报告一节带上"更多" | `upgrade-page.test.tsx` 的 `reviewing`（没有"更多"按钮） | S3 |
| 81 | 提交的答复不进缓存（`change(…, false)`；重读之前先闪旧的） | `upgrade-page.test.tsx` 的成功一格：持住升级的第二次起的读（初稿写 `secondReadHeld` 只持住第一段的 `mode` 引起的那次重读：假服务端分两段之后，答复回来之前还有第二次 `mode` 与 `change` 的 `reread()` 两次读，它们很快带回审阅中与报告，这个变异等一等就照过，S3 的实施），答复回来时缓存还是发出时那一份，持住时页面已显示审阅中与报告、对话框已关、焦点在标题（审查 P2C-M2） | S3 |
| 82 | `submitUpgrade` 不经 `inTurn` | `notebook.store.test.ts` 的并发 | S3 |
| 83 | `behindSubmit` 不含 `wiki.upgrade_state` | `notebook.store.test.ts` 的 409 | S3 |
| 84 | 单元不比计划时的状态（`apply` 开头的 `differs(…, "state")` 去掉） | `TestTheBatchPlansAgainOnceTheUpgradeStarted`（审查 P2A-N8） | S1 |
| 85 | 检查一节的视图建在会随一节卸下丢掉的地方：检查一节自己的 `useState`，由 `onSuccess` 写（审查 C 的 c1；初稿写"视图在 `onSuccess` 里建，同健康度页"，那一种在这里等价：SWR 2.5.1 在 React 18 起照调卸下的 hook 的回调，装回的 hook 在请求在飞时不另发，审查 P2S3C-N9，3.21） | `upgrade-check.test.tsx` 的"检查在飞时"三格：别人的提交、自己的提交的两种次序（审查 C 在修订核对里的怀疑；修复一轮让三格断言回来之后卡的第一页，审查 P2S3B-I1） | S3 |
| 86 | `UpgradePage` 不以 `key={location.key}` 渲染 `UpgradeRead` | `upgrade-check.test.tsx` 的"再点一次同一页的链接"（审查 P2C-N8） | S3 |
| n2 | 编辑的路径清掉标记（`pg.reached = rewritten != content`） | `TestThePlanTellsAPageAMoveReached` 的"编辑被到达、一次不链向的移动、再编辑"；`page/domain` 全包 | S1 |
| mv | 审查 C 的，同 n2（审查时写作 `pg.folded = rewritten != content`，审查 P2S1C-M1） | 同 n2 | S1 |
| n3 | 守卫先比正文、相同就放过（第 268 条之前的次序） | `TestTheFreezeGuard`；g23；g25 | S1 |
| n4 | 标记只在写的路径设，编辑的路径不设 | `TestThePlanTellsAPageAMoveReached`；g27 的 Q 格（第 270 条之后 g9、g25 分不出） | S1 |
| n5 | 字节不变、被到达的写不过守卫 | `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds`；g23 的后一种；g25 | S1 |
| n6 | 字节不变时不论有没有升级都过守卫 | `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds` 的"no upgrade, reached" | S1 |
| n7 | 字节不变时升级中不论标记都过守卫 | 同上的"upgrading, not reached" | S1 |
| o9 | `holdsAt` 的 `rewrote` 恒真 | `TestThePlanTellsAPageAMoveReached` 的"编辑正文、一次没够着的移动、再写同样的字节" | S1 |
| an2 | 升级中非改写的写一律当作字节不变、什么也不写 | g14、g15 的 `holds`（审查 P2S1A-N2） | S1 |
| MA1 | 单元只比有没有升级（`(State == "") != (a.upgrade == "")`） | `TestTheBatchPlansAgainOnceTheUpgradeStarted` 的"from reviewing"两格（审查 P2S1A-N1） | S1 |
| MA2 | 只在"计划没有升级、单元 `upgrading`"时重新计划 | 同上 | S1 |
| ra | `body` 在读基线行之前先看 `Reached` | `TestTheFreezeGuard` 的 `fresh × reached`；g26（审查 P2S1C-N5） | S1 |
| rw | 写的路径只在 `rewrote && !same` 时设 | `TestThePlanTellsAPageAMoveReached` | S1 |
| rg | `applyStep` 扔掉 `guarded` 交回的步 | `page/app` 全包（`TestSourceGuardKeepsTheFirstKeyAcrossRewrites` 等） | S1 |
| x1 | 升级中不照批前的链接判 | `TestTheBatchJudgesAPageByItsLinksBeforeIt`；g27 | S1 |
| x2a | `reachedBefore` 的移动、冻结、新建、树与读都只取到 `pg.First`（原来的范围） | 同上 | S1 |
| x2b | 只把 `MovesBetween` 的终点改成 `pg.First` | 同上 | S1 |
| x2c | 头问过就用头的答案（去掉 `First != Last`） | `TestTheBatchJudgesAPageByItsLinksBeforeIt` 的 `askedFirst` 两格 | S1 |
| x3 | 照批前的链接判在普通笔记本里也做 | 同上，普通笔记本的格（数读） | S1 |
| x4a | 判出来不交给计划 | 同上；g27 | S1 |
| x4b | 正文的一遍在写的路径上清掉标记（`pg.reached = rewrote`） | `TestThePlanTellsAPageAMoveReached` 的 `MarkReached` 格；`TestTheBatchJudgesAPageByItsLinksBeforeIt` | S1 |
| x4c | 正文的一遍在编辑的路径上清掉标记 | 同上；g27 | S1 |
| x4d | `MarkReached` 只标新建的页 | `TestThePlanTellsAPageAMoveReached`；`TestTheBatchJudgesAPageByItsLinksBeforeIt` | S1 |
| o1 | 提交只看 `frontmatter` 一道闸门（`case !check.Passed` → `!check.Gates.Frontmatter`；实施者的） | `TestSubmitUpgradeAnswersInOrder` 的"只有 `AGENTS` 不过"；`TestUpgradeSubmit` 的码依次的同一格 | S2 |
| a6 | 提交只看 `frontmatter` 与 `agents` 两道（`!check.Gates.Frontmatter \|\| !check.Gates.Agents`，审查 P2S2A-M1） | `TestSubmitUpgradeAnswersInOrder` 的每格只让一道闸门不过的表 | S2 |
| a1 | 闸门 7 不比 `NodesKept == Nodes`（审查 P2S2A-M2） | `TestJudgeUpgrade` 的"a baseline attachment deleted" | S2 |
| a2 | 闸门 7 不比 `Nodes == BaselineNodes` | `TestJudgeUpgrade` 的"an attachment's baseline row gone" | S2 |
| a4 | 提交在快照里只核对升级的 id（审查 P2S2A-M3） | `TestTheSubmissionsSnapshotIsItsOwn` 三格 | S2 |
| c1 | REST 的投影丢掉 `wanted_now.count_capped`（审查 P2S2C-M2） | `TestTheCheckIsAnswered`、`TestTheReportIsAnswered` | S2 |
| c11 | 判定的 `WantedNow` 去掉封顶 | `TestJudgeUpgradesCounts` 的"the wanted pages now capped" | S2 |
| c12 | 存报告时丢掉 `wanted_now.count_capped` | `TestTheReportRoundTrips`、`TestTheWorstReportFits` | S2 |
| c13 | MCP 的投影丢掉 `wanted_now.count_capped` | `TestTheCheckFits` | S2 |
| s1 | 存报告时 `wanted_before` 取 `WantedNow`（审查 P2S2C-M4） | `TestTheReportRoundTrips`（两个方向的夹具） | S2 |
| s2 | REST 的投影的 `wanted_before` 取 `WantedNow` | `TestTheCheckIsAnswered`、`TestTheReportIsAnswered` | S2 |
| gm3 | `UpgradePreserved` 的内侧按 `n.notebook_id = b.notebook_id` 读 `nodes`（审查 P2S2B-M1） | `TestTheUpgradeGateReadsPlansAtAnyStatistics` 另判 `nodes` 只经主键 | S2 |
| gm7 | 同上，另按 `n.parent_id = b.parent_id` 读一个文件夹（B 的未核实的怀疑） | 同上（规划器选只有 `parent_id` 的索引时，改前的帮手也判得出；"按笔记本与文件夹"的那一种由 `TestTheGateReadsReadANodeByItsKey` 手写的一格盖住） | S2 |
| r274a | 提交的期限的 503 的 `Retry-After` 取回 1 秒（第 274 条） | `TestTheSubmissionsDeadline` | S2 |
| r274b | 同上 | `TestUpgradeSubmit` 的期限两格（REST、MCP） | S2 |
| n9 | `lintQuery` 不拒摘要带 `AreaWantedByPage`（审查 P2S2A-N2） | `TestTheWantedCountsByPage` 的 the-second-pass-alone | S2 |
| p1 | 存报告时 `WikiPages` 取 `NewPages`（审查 P2S2C-M5） | `TestTheReportRoundTrips` | S2 |
| p2 | 读报告时 `Sources` 取 `Changesets` | `TestTheReportRoundTrips` | S2 |
| m1–m3 | MCP 的投影 `wanted_before` 取 `WantedNow`、`wanted_before.count_capped` 恒假、`wiki_pages` 取 `Sources` | `TestTheCheckFits`（m1、m3 另有 `TestTheCheckIsAnsweredWhole`） | S2 |
| hr1–hr3 | REST 的报告的规则 `count_capped` 恒真、恒假，`count` 取条目的个数（审查 P2S2C-M6） | `TestTheReportIsAnswered` | S2 |
| hg1 | REST 的 `preserved` 的 `nodes` 取 `nodes_kept` | `TestTheCheckIsAnswered`、`TestTheReportIsAnswered` | S2 |
| xg1–xg3 | MCP 的 `preserved` 的 `nodes_kept` 与 `pages` 对调、`frontmatter` 取 `sources`、`index` 取 `agents` | `TestTheCheckIsAnsweredWhole` | S2 |
| wp1 | 组合根把 page 的 `Preserved` 转成 wiki 时 `NodesKept` 取 `Nodes`（审查 P2S2C-M7） | `TestUpgradeGates` 的"a baseline attachment deleted past the units" | S2 |
| k1 | REST 的报告条目的 `kind` 恒为 `asset`（审查 P2S2C-N9） | `TestTheReportIsAnswered` | S2 |
| hi-MD=llm | REST 的规则页的 `mode` 恒为 `llm_wiki` | `TestTheCheckIsAnswered` | S2 |
| lg1 | 提交的日志的 `user_id` 取工作区的 id（审查 P2S2C-N10） | `TestTheSubmitterIsDecidedBeforeTheAnswer` | S2 |
| own-revalidate-on-mount-default | 检查一节装回时，这一代在这一次进入的缓存里已有条目也读（去掉 `revalidateOnMount`；实施者自己加的） | `upgrade-check.test.tsx` 的"a check refused as busy, its section taken away and given back" | S3 |
| own-gate-swap-agents-source | 闸门的次序对调 `agents`、`source_root` | `upgrade-check.test.tsx` 五种闸门组合的整行断言 | S3 |
| own-passed-const | 结果一行的 `passed` 写死为真 | 同上 | S3 |
| own-count-swap | 两个 `wanted` 计数对调 | `upgrade-check.test.tsx` 的计数两格（封顶两向） | S3 |
| own-preserved-swap | `preserved` 的 `nodes` 与 `nodes_kept` 对调 | 闸门的整行断言 | S3 |
| own-o1 | 报告的规则照答复的次序 | `upgrade-page.test.tsx` 的报告一格（夹具的次序反过来） | S3 |
| own-o2 | 报告的规则标题 `h2` | 同上 | S3 |
| own-o3 | `passed` 的报告也写"正在核对" | 报告一格；成功一格 | S3 |
| own-o4 | 对话框不给 `wiki.upgrade_not_ready` 自己的文字 | `upgrade-page.test.tsx` 的 409 先到的 `not_ready` 一格 | S3 |
| own-o5 | 提交被拒为任何码都 `recheck()` | `upgrade-page.test.tsx` 的 `server_busy` 一格（检查不该读） | S3 |
| own-o6 | `behindSubmit` 加 `wiki.upgrade_not_ready` | `notebook.store.test.ts` 的"不重读"四格 | S3 |
| own-o7 | 报告的数不带封顶的"+" | `upgrade-page.test.tsx` 的报告一格 | S3 |
| own-o8 | 报告不渲染闸门 | 同上 | S3 |
| own-o9 | 确认不等提交的答复（对话框立刻关） | `upgrade-page.test.tsx` 的 409 先到的两格 | S3 |
| c14、b27 | `SWRConfig` 随检查一节挂卸（设计的字面；审查 C、B，A 的 a-swrconfig-per-section，同一种写法） | "检查在飞时"答复在一节回来之后才到的两格（停在"检查中"；审查 P2S3C-N12） | S3 |
| b25o | 卡恒为空（对照，审查 P2S3B-I1） | "检查在飞时"三格（修复一轮让它们断言卡的第一页） | S3 |
| b26 | 设计原写的第 85 行另加"卸下的 hook 不建视图"（审查 C 在修订核对里的前提） | 同上 | S3 |
| a-check-409-no-reread（b02） | 检查答 `wiki.upgrade_state` 不重读升级（审查 P2S3A-M2 = B-M3） | `upgrade-check.test.tsx` 的检查本身答 409 一格（修复一轮） | S3 |
| a-check-409-not-moved（b03） | "升级的状态变了"只认卡的"更多"的拒绝 | 同上 | S3 |
| a-key-no-upgrade-id | 检查的键去掉升级的 id（审查 P2S3A-N1）：修复一轮之后等价，代数在一次进入里单调，两次升级的键在 `generation` 上已不同；键里的 `upgrade.id` 与 `nonce` 一样是防御，测不到（审查 P2S3C-N13，3.21） | 没有（等价） | S3 |
| f2 | 交给检查一节的代数不看升级（修复一轮的实施者写的，即修复之前的样子） | `upgrade-check.test.tsx` 的"checked, the upgrade ended and another started"（修复一轮） | S3 |
| f1 | `readItems` 不重读笔记本列表（修复一轮的实施者写的） | `upgrade-check.test.tsx` 的"a card's More refused as forbidden"（修复一轮） | S3 |
| m1-any-code | `readItems` 对任何 `ApiError` 都重读笔记本列表（修复核对一轮 A，审查 P2S3A-N5） | "更多"答 409 那一格数列表的读（修复第二轮） | S3 |
| a-leave-no-abort | 离开时不中止在飞的检查（审查 P2S3A-N2） | 离开时请求被中止一格（修复一轮） | S3 |
| a-checking-ignores-error | "检查中"不看 `error` | 检查答错一格（修复一轮） | S3 |
| a-check-after-rest（c12、c13） | 检查一节挪到按钮之后；报告挪到"放弃"之后（审查 P2S3A-N2、C-N5） | 状态表的 `upgrading`、`reviewing` 两格的次序（修复一轮） | S3 |
| a-submit-destructive（c5） | "提交审阅…"的对话框 `destructive`（审查 P2S3A-N2、C-N6） | 提交的对话框一格（确认按钮没有 `bg-destructive`，修复一轮） | S3 |
| b01、b07、b08 | 不过的闸门数、`pages_worse`、`stale_pages` 写成常量（审查 P2S3B-M1） | 闸门组合的各行（修复一轮让三者跨行不同） | S3 |
| b06 | 报告的每条规则只列第一项（审查 P2S3B-M2） | 报告一格（修复一轮让一条规则 2–3 项） | S3 |
| n1、n2 | 报告的每条规则截到 3、4 项（修复核对一轮 B，审查 P2S3B-N7） | 报告一格（修复第二轮让一条规则 5 项） | S3 |
| b04（c4） | 树在读时闸门的节点也写"（已不在）"（审查 P2S3B-M4 = C-N4） | 树读着时一格（修复一轮） | S3 |
| b28 | 视图的键带每次装上新取的随机数（视图随一节挂卸，审查 P2S3B-M5） | "更多"读到的条目卸下又装上一格（修复一轮） | S3 |
| b05 | 检查一节的规则照答复的次序（审查 P2S3B-N1） | 检查一节的规则一格（修复一轮让夹具的次序不同） | S3 |

### 7.4 门禁

6.4。另：`make gen-check` 在每个改契约的提交上；`TestTheToolListsAreTheContract` 在 S2 的第 7 个提交上（`tools.json` 加两个工具与六处措辞）；`TestTheBuiltInInstructionsAreShort` 照旧（3.14：两个入口各剩约 520、450 字节，S2 照 SDK 实测写进记录）。

### 7.5 CI 的余量

P1 末（`f002672b`，服务端同 `ce370441`）：七份是 145、51、121、124、62、11、28 个顶层测试（竞态构建，共 542）；第七份本机两轮 168.9、171.1 秒（`make -j1`，包的时间；审查 C 带 `-json` 的一轮 169.9 秒）。CI 上第七份的 Test 一步（`ABackground` 挪入之后，审查 C 拉的九次运行）：

| runner | Test 一步（秒） | 运行 |
|---|---|---|
| EPYC 7763 | 288、284、282（包 255.6、252.7、250.3） | 42713408119、42718191474、42741553383 |
| EPYC 9V74 | 244、239、235、243 | 42695633505、42700292442、42722296424、42734556937 |
| EPYC 9V45 | 197（`ce370441`） | 42749131694 |

初稿写的"CI 在 9V45 上 211 秒"是 S3 分支的运行 42635761979 在 7763 上、`ABackground` 挪入之前的数，标错了 runner（审查 P2C-N5）。7763 与本机之比约 285/170 ≈ 1.68。邻份最坏的一次：第一份 446、第二份 443、第三份 430、第四份 418、第五份 443 秒；第六份 354 秒，它在 P1 刚因 460 秒挪出过 `TestIngestBefore`。

- **S1**：第七份只加 `TestUpgradeGuardrails` 的二十二格（每格一次 MCP 调用，夹具按格各一本笔记本、一次发起）。照 P1 的同类（每次发起本机约 0.3 秒，`TestUpgradeStart` 五格 1.1 秒）估 7763 上加 15–25 秒，到约 300–315 秒。不开新份。实测（3.19、10.1）：S1 加了二十七格（g23–g25 由第 268 条、g26 由审查 P2S1C-N5、g27 由第 270 条加）；建成时本机两轮 172.4、174.6 秒，分支 CI 的 7763 上 Test 一步 292 秒（包 259.0 秒，比 P1 末多 4–8 秒，少于估的）；修复一轮之后本机 181.6、176.4 秒，CI 的 9V45 上包 180.5 秒；修复第二轮之后本机 183.9、181.4 秒（审查 B 一轮 171.6 秒），CI 的 9V74 上 Test 一步 240 秒。
- **S2**：新的 `TestUpgradeCheck`、`TestUpgradeGates`、`TestUpgradeSubmit`、`TestUpgradeSubmitInterleavings`、`TestUpgradeToolsMatrix`、`TestUpgradePermissionMatrix` 的三个操作、`TestUpgradeToolsErrorsCarryTheirCodes` 的两行。照 P1 的同类（`TestUpgradePermissionMatrix` 92 格并行、本机墙钟 7.65 秒；P1 的 12 条交错共 7.0 秒、最长 2.8 秒）估 7763 上加 75–120 秒（审查 C）；另有期限一格的等待（REST 约 4 秒、MCP 约 6 秒，再等过期限接管，两个入口共约 15 秒墙钟，不占 CPU）。不开第八份时第七份到约 375–420 秒，余量小于 P3 要加的，邻份又没有地方挪，所以 S2 的第一个提交开第八份（第 261 条）：第八份收 `^TestUpgrade(Check|Gates|Submit|Tools)`（含 P1 的 `TestUpgradeToolsErrorsCarryTheirCodes`），7763 上估约 100–150 秒；第七份留 P1 的与 `TestUpgradeGuardrails`、`TestUpgradePermissionMatrix`、`TestUpgradeStatesReachTheStreams`，约 305–320 秒。数由 S2 的记录实测。实测（3.20、10.2）：S2 加了五个顶层（`TestUpgradeCheck`、`TestUpgradeGates`、`TestUpgradeSubmit`、`TestUpgradeSubmitInterleavings`、`TestUpgradeToolsMatrix`），P1 的 `TestUpgradeToolsErrorsCarryTheirCodes` 挪进第八份：`go test -race -list` 547 个，八份 145、51、121、124、62、11、27、6；本机第七份两轮 185.1、179.5 秒，第八份 46.7、46.3 秒；分支 CI（42882105107）第七份在 9V74 上 Test 一步 260 秒，第八份在 Xeon 8573C 上 97 秒。第二份没有加测试，却在 7763 上 Test 一步 457 秒，过了 450 秒的线（`main` 上 438、430 秒）：本机 `ba72cda4` 上 250.8、252.7 秒，`c3c35bbc` 上 263.2、264.4 秒，慢约 5%，摊在许多小测试上；审查 B 查出原因是契约变大（`api/dist/openapi.yaml` 286,572 → 306,033 字节），组合根的夹具每建一个应用就 `apitest.Load` 一次，竞态构建下每次多约 62 ms（第 9 节交给 P3）。第 272 条在合并之前把第二份最大的 `TestVirtualExport`（本机约 52 秒）挪进第七份与第八份的模式（第八份仍是第七份的子集；只挪进 `EIGHTH` 时它在第三份与第八份各跑一次，审查 P2S2B-Q1）；B 估挪后第二份在 7763 上约 374 秒，第八份约 190 秒。挪后（3.20、10.2）：`-list` 547 个，145、50、121、124、62、11、27、7；本机第二份 204.4、202.0、204.1、202.6 秒，第八份 98.6、99.8、99.3、100.3 秒，B 照 1.61 加 33 秒折到 7763 约 360、193 秒；修复的五次 CI 上第二份在 Xeon 8573C 上 288 秒、9V74 上 292、292 秒、9V45 上 229 秒，第八份在 9V45 上 147、138、132 秒、9V74 上 170 秒（B 照第一次的两个 runner 交叉核对，折到 7763 第二份约 357 秒、第八份约 185–205 秒）；落在 7763 上的各一次：第二份 400 秒（修复第三轮，挪后第一次，在线下，比两个估计都高，那一轮第二份的测试没有动），第八份 187 秒（修复第四轮，与估的相符）。合并之后照第 8 节第 1 条看两次 `main`：S2 合并那次 `main` 与同一棵树的分支 run 上第三份平均 450 秒、第五份 451.5 秒，到了线；第 276 条把两份各一个测试挪到第八份，挪后两次 `main` 八份平均 329.5、350、397、358.5、395.5、304.5、255.5、243 秒（10.2 的"旁支"）。
- **第五份**：只改 `TestTheWikiToolsContract` 的计数与 `TestTheToolListsAreTheContract` 读的文件，不加运行的时间；第三份的 `TestThePermissionMatrixCoversEveryTool` 是静态的核对。
- 每个 Step 的记录写出各份的 `go test -race -list`（合起来等于全部，新的测试只在第七、八份）与本机在 heavy 锁下各跑两轮的平均（`make -j1 test-go-bootstrap-<n>`）；合并之后两次 `main` 的 CI 里每份 Test 一步的平均在 450 秒之下，否则照 06-environment 第 50 行处理之后才开下一个 Step。
- 成本守卫：S1 的两行进 costs-2（估约 3 秒；实测本机约 17 秒，种子 12.7 秒是大头，CI 上 9V45 22.7 秒、9V74 36.5 秒，3.19）与 guards-core（五种形状与 100 × 5 MiB 的种子，估 40–90 秒；实测本机约 21 秒，7763 上 37.8 秒）；S2 的一行进 costs-2（5 万页加九本各 5 万页的种子，照发起的守卫估 40–70 秒，在 7763 上约 1.5 倍）；一行进 costs-3（10 万行基线加九本各 5 万页的种子、十八格计划，照回滚的读的守卫估 40–60 秒），作业约 18 分钟加约 1 分钟，时限照旧 35 分钟。costs-2 在 `main` 的 CI 上：EPYC 7763 22.2 分钟（run 42530568840）、23:17（run 42752100006，`f002672b`），EPYC 9V74 16:42（run 42749131694）；加上 P2 的两条，快机约 18.5 分钟、7763 约 25–26 分钟，35 分钟的时限不到 `ci.yml` 里"约两倍"的规则，所以把它提到 45 分钟（第 267 条；P1/S2 的审查 C 提过，当时说合并之后看 `main` 上的数）。S1 的审查 P2S1B-M1 之后提前到 S1 的修复（`80da6b1f`），guards-core 同时从 30 分钟提到 40 分钟（第 269 条：7763 上作业约 20 分钟，末行在第 19.4 分钟才开始，卡住时 Go 默认的 10 分钟只比 30 分钟早约半分钟）；P2/S2 的 `main` 上 costs-2 在 7763 上 23:45（42762546769，`c37a3faf`）。S2 加检查的守卫之后再看 costs-2 的余量（审查 B：慢三成的实例上约 1 分钟）；guards-sources（7763 上约 1.8 倍）下次改到时照第 269 条提到 40 分钟。S2 实测：costs-2 在 9V45 上 14:48（检查的守卫 30.7 秒）；审查 B 估它在 7763 上作业约 25.4 分钟、末行约第 24.5 分钟开始，卡住时 Go 默认的 10 分钟在第 34.5 分钟，先于 45 分钟，不再提。costs-3 在 9V74 上 14:46（闸门的读的计划 34.3 秒）；`main` 上落在 7763 时 `-timeout 20m` 那一行在第 12.7–14.4 分钟开始，S2 在它之前再加约 0.75 分钟，它的超时落在第 33.5–35.2 分钟，贴着 35 分钟，第 275 条提到 40 分钟（审查 P2S2B-M2；修复一、二轮的 CI 上 costs-3 都在 Xeon 8573C 上，15:11、16:33，那一行在第 11.7、12.7 分钟开始，三至五轮在 9V74 上 13:42、13:38、13:55）；修复第三轮的 CI 上 costs-2 落在 7763：24:54，与 B 估的约 25.4 分钟相符。guards-sources S2 没有改到。
- e2e：S1 加 UP4 两个、S2 加 UP5 两个、S3 加一个，都只在 Chromium；作业的 20 分钟照 P1/S4 的量有余。实测：S3 末 `make e2e` 523 个（本机 2.4 分钟）；S3 的分支 CI 的 e2e 作业 8:42（E2E 一步 7:09）。S3 没有改服务端，分支 CI 的八份 384、376、404、373、246、289、314、207 秒；修复第二轮的 run 第四份 446 秒，贴着 450 秒的线，合并之后看两次 `main`（10.3）。

### 7.6 P2 怎样量

- S1：第七份两轮；两个护栏的守卫的本机中位数（三种统计 × 两种计划的每格；单元的五种形状与它们在普通笔记本上的对照）；`go test -race -list`。
- S2：第八份开之前与之后八份的 `go test -race -list`；第七、八份各两轮；闸门的读的计划守卫三种统计 × 两种计划的计划与每条读的本机中位数；检查的守卫两份种子的本机中位数、它在 2.4 s 之内的余量，各段（`Roots`、`prepareIn` 与 `PreparationTimings`、`Baseline`、`Preserved`、`Changesets`、判定）分开记；`TestTheWorstReportFits` 打出的字节数；两个入口的内置说明的 SDK 实测字节数；合并之后两次 `main` 上 costs-2 的作业用时与 runner（第 267 条的余量）。
- S3：vitest 的文件与测试数；`make e2e` 的个数与用时；UP5 的页面版本单独三轮。实测（10.3）：vitest 193 个文件、4,437 个测试（S3 多一个文件、92 个测试），`make test-web` 59–61 秒；`make e2e` 523 个、2.4 分钟（整项 144 秒）；UP5 的页面版本三轮 1.0、1.0、0.99 秒（Playwright 3.9、3.5、3.9 秒）；L1 的复现另记（审查记录"偶发"）。修复一轮之后（`ff50d516`）vitest 193 个文件、4,444 个测试，`make test-web` 60 秒，`make e2e` 523 个、140 秒；修复第二轮（`3717deba`）只改既有的格，仍是 4,444 个，`make test-web` 58 秒。

## 8. 完成标准

P2 完成，当且仅当：

1. 三个 Step 各自合并进 `main`，合并之前 6.4 的门禁全绿、审查与修复核对结束；每个 Step 合并之后两次 `main` 的 CI 里组合根各份的 Test 一步的平均都在 450 秒之下（7.5），否则照 06-environment 第 50 行的规则处理之后才开下一个 Step。
2. 第 3–5 节的每个测试在，7.3 的每个变异被它旁边的测试抓到（第 10 节记下），3.18 的每个反向对照在测试里。
3. 第 2 节的移交的落实做完；两份移交（`handoffs/M10-mode.md`、`handoffs/M9-mcp-upgrade.md`）的"处理进展"各加一行（每个 Step 的文档提交写它做了的项；S3 只加与网页有关的，`M9-mcp-upgrade.md` 的两条与网页无关），状态仍是 `open`（M11 收尾时改为 `done`）。
4. **00 号文档**（`docs/v0.1/M11-upgrade/00-M11-design.md`）：
   - **接受本文的文档提交**：以 `p2-apply-00.py`（`--check` 与 `--path`，每处一个带标签的替换，一处不匹配就什么也不写）改：
     - 2.1 的二十一条照各条写的新文字。连带的：第 1、5 条连带第 8 节 page、linking 两条；第 2 条连带第 190 条；第 7 条连带第 215 条；第 15、20 条在 4.13 的工具表；第 16、18 条在 4.17 的上限表；第 17 条在 4.6"账本"与第 191 条；第 19 条在第 222 条；第 21 条在第 9 节的成本守卫表。
     - 2.3 的十四条写进第 11 节，另起一节"2026-11-10：P2 的设计审查之后的取舍 254–267（主持者的决定，负责人可以改判）"，每条一句、带它在本文与审查记录里的位置。这一节接在"2026-11-10：P1 的 S4 的取舍 246–248、250–253"一节之后：那一节是第 11 节里最后一个取舍节，本身就在"旁支的取舍 249"一节之后。
     - 头的"状态"改为"进行中（P1 完成，P2 设计已审查、S1 开工；…）"。
     - 第 12 节的 P2 一行 `| P2 护栏、闸门与提交（S1–S3） | — | 未开始 | — | — |` 改为 `| P2 护栏、闸门与提交（S1–S3） | [02-P2](02-P2-guardrails-gate-submit.md) | 进行中（设计已审查） | [设计](reviews/P2-design-review.md) | — |`。
     - 第 13 节加一行"P2 设计已审查，S1 开工；2.1 的二十一条修订落进本文；P1 完成那次 `main` 的 CI（42752100006，`f002672b`）18 项全绿，costs-2 在 7763 上 23:17；第 11 节加 P2 的取舍 254–267"，依据是 P2 的设计与它的审查记录。
   - **每个 Step 的文档提交**（在 Step 的分支上、合并之前）：第 12 节的 P2 一行的"状态"写到这个 Step、"审查"一格加它的记录 `reviews/P2-S<m>-<slug>-review.md`，"合并"一格写"见本文第 12 节"的占位，合并之后在 `main` 的小文档提交里填入合并的提交号；第 13 节加一行（实施的结果、照实与设计不同之处、随之改的节）；本文第 3 节加"3.19 S<m> 的实施对设计的修正"、第 10 节填这个 Step。
   - **`docs: M11/P2 done (M11/P2)`**：第 12 节的 P2 一行"完成"；头的"状态"改为"进行中（P2 完成，P3 设计中；…）"；第 13 节加"P2 完成"一行。
5. **v0.1 总体设计**（`docs/v0.1/v0.1-design.md`；00 号文档第 7 节"总体设计的修订"第 30 条里属于 P2 的部分，各在实现它的 Step 的文档提交里改，用脚本断言锚点恰好出现一次；下面的行号照 `f002672b` 复核过，与 `455f692c` 的相同）：
   - 13.1 第 1 条（`:1049`）末尾加——S1（照第 268、270 条，S1 的文档提交已写进）："升级中发起人的写另过护栏：移动与改名必须改写链接（单独的移动看 `Options.UpdateLinks`，`batch` 的看 `Step.KeepLinks`）；基线页只许改 frontmatter，比的是切掉 frontmatter 与一个 BOM 之后的正文（单元交 `StepContent.BodyHash`，守卫只为基线页读当前的正文），系统的链接改写放行；`batch` 里此前的移动或改名改写过一页基线页的链接（照它在批里的内容，升级中另照它批前的链接），之后再写它，写什么都一样，答它自己的码（`ContentWrite.Reached`），字节与库里相同的这种写也照一次写过守卫、什么也不写；正文的两个码标在这一页最后一个正文操作；每个正文步写升级的账本，升级中先核对账本等于写之前的哈希；升级中一次 `batch` 盲写的已有页也计进读的界，计划读到的升级状态与单元在锁下读到的不同时重新计划（M11/P2）。"
   - 13.1 第 4 条（`:1071`）末尾加——S2："升级的检查照 lint 的次序，条目的游标在读到升级之后核对它绑定的根；提交的码：404 → 403 → 状态的 409 → 503 → 内容的 409（索引、闸门）（M11/P2）。"
   - 13.1 第 5 条（`:1073`）末尾加——S2："升级的提交分两个事务，都在笔记本行的 `FOR NO KEY UPDATE` 之下，中间的计算不持行；第二个事务脱离请求的取消，期限取第一个事务读的时刻加 `SubmitTimeout` 与请求的期限中较早的，交付之前的判定在它的 `COMMIT` 之前，只写它自己那一次提交（M11/P2）。"
   - 13.1 第 8 条（`:1100`）末尾加——S1："`page.upgrade_immutable`、`page.upgrade_batch_rewrite`、`page.upgrade_keep_links` 只在 MCP，进 `mcpOwnCodes()`（M11/P2）。"S2："`wiki.upgrade_not_ready` 的 `detail` 只列出不过的闸门的键（M11/P2）。"
   - 13.1 第 10 条（`:1103`）末尾加——S2："升级的提交在第二个事务提交之后记一行 info，带结局，与交付之前的判定的结果无关；检查与条目记在 `Runs` 的那一行，带种类与升级或提交的 id（M11/P2）。"
   - 13.1 第 23 条（`:1137`）末尾加——S2："`Runs` 的键带升级的检查、条目与提交的种类与 id：同一次升级的检查合并，提交不并入任何计算（M11/P2）。"
   - 13.1 第 31 条（`:1164`）末尾加——S2："存下的报告每条规则至多五项、不带游标，由表的检查限在 64 KiB 之内（M11/P2）。"
   - 13.2 第 9 条（`:1199`）末尾加——S3："UP5 的页面版本里刻意的 409 一个，以 `pageWatch.expectConsole` 声明（M11/P2）。"第 18 条（`:1209`）末尾加——S3："升级页的检查与报告只给管理员；检查的缓存按这一次进入，自己的提交在算时照实显示审阅中（M11/P2）。"
   - 13.4 第 3 条（`:1264`）末尾加——S1："M11/P2：UP4 只有 MCP 版本。"S2："UP5 的 API 版本以 MCP 写作准备（REST 的写在冻结里被拒）。"S3："UP5 的页面版本只在 Chromium。"
   - 13.4 第 4 条（`:1265`）末尾加——S2："M11/P2：提交与发起人的 `batch`、检查的快照与之后的写、两个事务之间的放弃与接管、排队中的检查与放弃、两次提交、提交人被降级、检查跨过提交各一个确定的交错；停住检查的计算用包住 lint 的规则的钩子，停住的窗口短于被停一方的 `Do` 预算，要等的一方以 `RunCreated` 等到，名额与期限不够时另配一个应用。"
   - 13.4 第 6 条（`:1273`）末尾加——S2："M11/P2：MCP 的升级工具的行另成一组（`upgradeMCPRows()`），在升级的变体上跑，覆盖的检查读 `coveredMCPRows()`。"
   - 13.4 第 8 条（`:1275`）末尾加——S1（照实施的 14 秒，S1 的文档提交已写进）："护栏的两条读，三种统计 × 两种计划；护栏在批的界上的五种单元各短于 14 秒（批的 bench 的生产请求 15 秒之内，决定 153、243），中位数另有三倍的界；升级中盲写计进读的界（M11/P2）。"S2："检查在 5 万页上，库里另有九本笔记本的升级的行；`changesets` 进判计划的帮手（十万页在 M12）。"
   - 第 15 节：每个 Step 的文档提交加一行（日期、改了上面哪几条、依据是本文与 Step 的审查记录、`M11/P2/S<m>`）。
6. `docs/v0.1/claude_to_codex/06-environment.md` 的分份两条（第 27、50 行）在 S2 的第一个提交里照 6.2 第 1 个提交写的新文字改（第 261 条）。

## 9. 交给 P3

- **P3/S1（审阅）**：
  - 批准在笔记本行下读"审阅中、带报告"、且报告的 `passed` 为真（`Upgrade.Report`，3.12）；批准不再算检查（存下的报告就是依据，00 号文档 4.9），`wiki.upgrade_not_ready` 已在 S2 定义、两种语言的文字已有，P3/S1 照用。
  - 退回（`sendBackUpgrade`）的"审阅中、没有报告"的接管用同一个 `SubmitTimeout`（`UpgradeDeps.SubmitTimeout`，`submitted_at + SubmitTimeout` 之后，期限照存下的 `submitted_at` 算），清提交的列的写可照 `UnsubmitUpgrade`（3.12）另写一条（它要的是不论 `submission_id`）。
  - `TestUpgradeSubmitInterleavings` 第 7 条的"退回"一格与第 9 条（批准与驳回）在 P3/S1 加；第 8 条的"批准"一半同样。停住计算的照 7.2 的"名额与期限"，在另配的应用上跑。
  - **P3 的整个程序测试落在哪一份**（审查 P2C-N5）：`TestUpgradeApprove`、`TestUpgradeReject`、`TestUpgradeSendBack` 与它们的交错照名字落在第七份（`^TestUpgrade` 去掉第八份的模式），`TestUpgradePromptMatrix` 照第 222 条在第七份。P3 的设计照 P2/S2 合并之后两次 `main` 的第七、八份的数定：第七份放不下时扩第八份的模式（例如加 `Approve|Reject|SendBack`），扩了还过线就照 06-environment 第 50 行再分一份。第八份另有第 272 条从第二份挪来的 `TestVirtualExport`（B 估 7763 上约 190 秒）；S2 合并之后第 276 条又从第三、第五份各挪来一个（`TestTheSourceRebuildMatchesSuccessfulRandomSteps`、`TestTheMCPToolsErrorsCarryTheirCodes`），本机折到 7763 约 306 秒，挪后两次 `main` 都在 9V74 上 243 秒（10.2 的"旁支"）。契约每长一截，组合根里每建一个应用就 `apitest.Load` 一次（读、校验、建路由整份 `api/dist/openapi.yaml`，竞态构建下约 0.63 秒），各份照比例慢一截：S2 让契约长约 6.8%，第二份慢约 5%（审查 P2S2B-Q2）；P3 定分份时把这一项算进去，要不要把契约在进程里只读一次是测试设施的取舍。
- **P3/S2**：
  - playbook 写明：护栏的三个码；无效 frontmatter 的两种改法（00 号文档 4.6"核实"）；"同一页在一个 `batch` 里改了又改回放过"（第 208 条，3.4），中间有移动改写过它、或中间的移动照它批前的链接够着它时不放过（g25、g27）；`page.upgrade_batch_rewrite` 的拆法：一个 `batch` 里改名或移动之后，再写链向它的原有页一律要拆开（先写这一页、再移动，或分两个 `batch`），写回原来的字节、在批里先拿掉再放回链接也一样（第 268、270 条，3.4）；拆开之后正文本身的改动仍被拒为 `page.upgrade_immutable`（第 265 条）；只写名字的链接，移动之后照样解析，系统不改写；"移动之后系统改写链接"只对写了路径的链接成立，改名则两种都改写（审查 P2S1C-N2）；升级中一次 `batch` 盲写的大页计进读的界、过了答 `page.batch_too_large`，拆开即可（第 266 条）。
  - prompt `upgrade` 让 agent 先 `upgrade_verify`、通过之后 `upgrade_submit`、`wiki.upgrade_state` 的"正在计算"时等一会再检查。提交答 `server_busy` 时照它的 `retry_after_seconds` 等过再交：第 274 条让它取到能接管为止，不必另写"等几秒再交"（审查 P2S2A-N1）。
  - MCP 人工清单的 K3 记 `page.upgrade_immutable` 在客户端里的样子与 `upgrade_submit` 的 `destructiveHint: false` 是否让客户端少问一次；笔记本入口的 `upgrade_submit` 的 `inputSchema` 没有 `properties`（同账户入口的 `notebooks`），看客户端是否接受（审查 C 的未核实的怀疑）。
- **P3/S3（网页）**：审阅一节复用 `UpgradeReportSection`（4.3），加"更多"与三个动作；"退回"的"上一次提交正在计算"的文字与 20 s 的重读（00 号文档 4.15）。`upgrade-check.tsx` 的 `UpgradeGates`、`UpgradeCounts` 只收数据与树，`HealthRule` 的 `level` 交 3，照用；审阅一节的缓存照 4.3 的"这一次进入"。照 S3 的代码（行号是修复一轮的头 `ff50d516` 的，修复第二轮只改测试），P3 还要改的（审查 P2S3C-N11 的六条与 P2S3C-N7）：
  1. 报告的规则今天由 `UpgradeReportSection` 自己渲染（`h3`、数、至多五项 `HealthItem`，`upgrade-check.tsx:261-281`），不经 `HealthRule`；加"更多"时换成 `HealthRule`。
  2. `HealthCard.items` 的类型是 `readonly LintItem[]`（`health-rule.tsx:19`），存下的条目是 `UpgradeReportItem`（没有游标），要放宽；`paths` 的参数同样。
  3. `read` 的 `cursor: string` 与 `UpgradeService.checkItems` 的 `cursor` 必填（4.3 有意如此）；"从第一页读"要让它可缺：存下的报告没有游标。
  4. 报告一节今天在这一次进入的 `SWRConfig` 之外（`upgrade-page.tsx:277-279`、`:300`，它不读，没有问题）；它的"更多"要用这一次进入的缓存，挪进去或给它自己的一份（审查 P2S3C-N10）。
  5. "更多"的 403 照 S3 的 `readItems`（`upgrade-check.tsx:159-166`，`41389247`）在审阅一节包它的 `read`：`forbidden` 且这一代当前时重读 `["notebooks", ws]`、再抛原拒绝，不改 `HealthRule` 的接口（审查 P2S3A-M1 = C-M1）。要带上三样（修复核对一轮 A）：`UpgradeReportSection` 今天不收 `rereadNotebooks`，从 `UpgradeRead` 交下（外层的 `mutate`，不是这一次进入的缓存的）；`current()` 判审阅一节自己那一代；审阅一节自己的 403 一格测试，不然同一个判断两处复写只在检查一节测。也可以把 `readItems` 提成模块里的一个函数（收 `read`、`current`、`rereadNotebooks`），两节共用，P3 仍各测一格。
  6. `cursor === null` 时 `HealthRule` 不渲染"更多"（`health-rule.tsx:238`），读之前同样判（`:178`）；这一条初稿引的 `:210` 是 S3 之前的行号。
  7. 树的第一次读失败时 `readingTree = pages.nodes === undefined || tree.isValidating` 恒真（`upgrade-page.tsx:162`），闸门的节点与报告的条目一直写"加载中"，没有错误也没有重试；审阅一节同样受影响，照健康度页把 `tree.error` 与重试交下（审查 P2S3C-N7，S3 不改：左栏的树显示错误、重试之后这里跟上）。
- **M12**：检查的十万页（第 262 条）写进 M12 的性能移交，同第 232 条的写法：审查 B 实测十万页的检查折到 7763 约 2.5–3.0 秒，是 worker 3 秒的 84–100%。第 270 条的代价（第 271 条，S1 的修复核对 P2S1B-M2）同样写进 M12 的性能移交，接在 M9 那一项（第 18 条"MCP"）之后：M9/P3 已接受的"许多次改名之间的带链接的页"（50 次改名一个 10 万个节点的文件夹、每次之后编辑一页带链接的页，`M9-mcp/03-P3-batch-revert.md`"许多次改名之间的带链接的页"一段）在升级中计划约翻倍，本机 7.5 s 对 3.5–3.8 s，调用 10.1 s 对 6.1–6.4 s，照 7763 的比估约 18.6 s，过 15 秒的请求期限；过期限时什么也不写，拆开就能过。

## 10. 结果

（每个 Step 的文档提交填写：做了什么、照实与设计不同之处、测试与变异、量到的数、审查的结果。）

### 10.1 S1：护栏（服务端）（2026-11-10，合并 `8cf7af36`）

- 提交（从 main 的 `c37a3faf` 开始，照 6.1 的次序，6 个；Step 的文档提交是主持者的）：
  - 移动与改名必须改写链接：`Step.KeepLinks`（批的移动的 `update_links: false`，单元的每一步带它），单独的移动看 `Options.UpdateLinks`；不改写答 `page.upgrade_keep_links`，标在步的根；`guardrails` 按操作分支，P1 的删除抽成 `deletion` 照旧（`54b1be8b`）。
  - 基线页只许改 frontmatter：`domain.UpgradeBody`（切掉 frontmatter 与一个 BOM）、markdown 适配器的包级 `FrontmatterEnd`；单元在升级中、不是系统改写时交 `StepContent.BodyHash`，守卫只为基线页读一次当前正文（`UpgradeBaselineContent`），不同答 `page.upgrade_immutable`，没有 `BodyHash` 答缺陷；每个正文步写升级的账本（`AcceptUpgradeContent`），升级中只在账本等于写之前的哈希（`StepContent.BeforeHash`）时写，基线页对不上是缺陷，回滚的单元照写（`8ad3fe43`）。
  - 升级中 `batch` 盲写的已有页按第一个正文操作的次序计进 `MaxBatchReadBytes`（第 266 条）；批记下计划读到的升级状态，单元在锁下读到的不同就重新计划（设计审查 P2A-N8）（`937a257c`）。
  - `batch` 把移动的改写折进基线页：计划的 `PlannedPage.Folded`（编辑在改写改了这一页之后设，写的是改写的原样时设、写别的内容时清），`ContentWrite.Folded` → `StepContent.Folded`，守卫答 `page.upgrade_batch_rewrite`；正文的两个码标在这一页最后一个正文操作（第 254、265 条）（`f6de159e`）。审查之后照第 268 条改成粘住的 `Reached`（修复一轮）。
  - 两个成本守卫：护栏的两条读（costs-2）与护栏在批的界上的五种单元（guards-core）；`Makefile` 两行与说明，`ci.yml` 两段注释，时限不变（`90134668`）。这个提交改了 `Makefile` 与 `ci.yml`，范围只写 `server`，部件该写 `build`、`ci`（P2S1C-N3；已推的，不改写）。
  - e2e：UP4 的两个新测试与 `expectBaselineKept`（`f4d88dd4`）。
  - 三个码进 `mcpToolCodes()`（write、edit、move、batch 四行）、`upgradeToolCodes()`、`mcpOwnCodes()`；`TestUpgradeToolsErrorsCarryTheirCodes` 在两个入口答出六种情形。
  - 每个提交单独在工作树里 build、vet、`make lint-go`、`make gen-check`、short 测试都通过，另跑它自己的组合根测试（2 号另跑 page 模块的全部竞态测试，5 号两个守卫，6 号 `make lint-web`、`make knip`、vitest、`make build` 与 UP4）。推送之前改写过一次 4 号（`f940065a` → `f6de159e`：补变异 o9 抓不到的一行表），5、6 号原样重放。审查 C 另在 `54b1be8b`、`937a257c`、`f6de159e` 上各自跑 build、vet、`make lint-go`、short、page 模块（不带竞态）与竞态下的组合根六个测试（含第五份的 `TestTheMCPToolsErrorsCarryTheirCodes`）。
  - 修复一轮另有 4 个、第二轮 2 个（下面），一共 12 个；合并 `8cf7af36`。
- 与设计不同之处（都是最小的改动；已改在各节，3.19 列出）：
  1. 单元守卫的期限 14 秒（决定 153、243；批的 bench 是生产的 15 秒请求），不是"守卫配置下的 6 s"；最大的单元本机 1.7 秒、7763 上 3.2 秒（7.1）。
  2. 形状 b：100 页各删一行约 62 KB 的 frontmatter，头合计约 13.8 MiB、最终约 8 MiB（照字面的 140 KB 编辑过 `MaxUnitBytes`，单元不会跑）（7.1）。
  3. `CheckReads` 一遍按第一个正文操作的次序计（3.3）。
  4. 读的守卫是三种统计 × 两种计划，每格判计划又计时（7.1、7.6）。
  5. g22 的三页以 SQL 种下（3.6）。
  6. `StepContent.Folded` 在 4 号提交加（6.1）。
  7. `AcceptUpgradeContent` 的两个 CTE 带表的别名（sqlc）（3.5）。
  8. 形状 c 是一个操作的 `batch`（7.1）。
  9. 批的 bench 给 `/mcp` 以 wire 的请求体上限（7.1）。
  10. 单元守卫的种子以 SQL 写回，不另逐页核对（7.1）。
  11. 读的守卫本机约 17 秒（种子 12.7 秒），不是"约 3 秒"（7.5）。
  12. 多一个 `TestTheUnitStepCarriesKeepLinks`（7.3 第 3 条）。
  13. UP4 的 Notes 以从根起的路径链向 Plan：只写名字的链接在移动之后照样解析，系统不改写（第 5 节，审查 P2S1C-N2）。
  14. UP4 的版本号原来从库里拼，审查之后改从答复取（第 5 节，P2S1C-N1）。
- 实测（本机，经 `heavy.sh`；出处见 [S1 的审查记录](reviews/P2-S1-guardrails-review.md)"实测"）：
  - 建成时：`go test -race -list` 542 个顶层测试，七份 145、51、121、124、62、11、28，各在恰好一份（同 P1 末）；第七份两轮 172.4、174.6 秒（P1 末 168.9、171.1）。护栏的读：三种统计 × 两种计划，计划都合规，读的中位数 0.12–0.16 ms、写 0.15–0.19 ms（界 5 ms），种子 12.7 秒。护栏在批的界上的单元（ms，两轮，升级中 / 普通笔记本）：a 1,701、1,712 / 1,686、1,647（界 5.2 s）；b 343、348 / 293、294（1.1 s）；c 47、44 / 35、36（200 ms）；d 23、26 / 15、17（100 ms）；e 2、3（计划先拒）/ 251、252（100 ms）；期限 14 秒。vitest 192 个文件、4,341 个测试；`make e2e` 520 个，2.3 分钟。
  - 修复一轮之后：第七份两轮 181.6、176.4 秒（g23–g25 三格各一本笔记本与一次发起，加上 g14、g15 的两句；审查 B 一轮 179.5 秒）；顶层的归类不变；单元守卫的形状不变，逐个提交与 `make test` 里的三次都在界内。
  - 修复第二轮之后：第七份两轮 183.9、181.4 秒（g26 多三本笔记本、g27 的 Q 多一本；量时刚跑完 `make test`，审查 B 一轮 171.6 秒）；顶层的归类不变（542 个，145、51、121、124、62、11、28）。新的读不落在任何成本守卫的形状里（形状 a 只有一次移动、没有正文操作，b–e 没有移动），不另量；`0b016421` 的逐个提交核对里单元的守卫照常过（升级中 / 普通笔记本，ms：a 1,763 / 1,682，b 352 / 300，c 51 / 44，d 37 / 15，e 3 / 247）。第 270 条的代价由审查 B 量出（第 271 条，3.19）：M9 已接受的"许多次改名之间的带链接的页"在升级中计划 7.5 s 对 3.5–3.8 s，调用 10.1 s。
- 反向对照（每一条的结果见 S1 的审查记录"反向对照"）：
  - 建成时：7.3 的 S1 各行（1–24、84）每个点名的测试各跑一次（共 44 条，12 号拆成两种改坏），加实施者自己的 11 种（o1–o11，12 条），共 56 条。第一轮 o9（`holdsAt` 的 `rewrote` 恒真）没抓到、o4 的规格编不过；表补一行（编辑正文、一次没够着这一页的移动、再写同样的字节，标记为假）之后两条都抓到；其余都被点名的测试抓到，组合根的各格单独跑、点名到格。
  - 修复一轮：第 268 条的四条（n1–n4）、字节不变那一处的三条（n5–n7）、第 17、18、20–22 条与 o9 照新的语义重跑、审查者留下的 MA1、MA2、`mv`、an2，共 31 条，都抓到。原来的第 19、20 条照第 268 条已是规定：第 19 行换成 n1，第 20 行换成"照原来的规则只在写的是改写的原样时设"。
  - 修复第二轮：第 270 条的九条（x1、x2a–x2c、x3、x4a–x4d），第一轮的照新代码重跑，C 的 `ra`、`rw`、`rg`；在 `35478201` 上共 52 条，48 条抓到，n4、20 在 g9、g25、g21、g23 上活下来（第 270 条的索引判据先把 P 标上，这几格分不出正文一遍的判据），它们被计划的单元测试抓到；g27 补了 Q 的两格之后，n4、20 在 g27 上也死。`x2c`（头问过就用头的答案）在第一版的测试上活下来，补了 `askedFirst` 两格。整轮没有在最后的头上重跑：`0b016421` 与 `35478201` 的提交 1 只差 g27 的 11 行测试、代码相同；修复核对第二轮 A 在头上另跑 n4、20 与去掉 `reachedBefore` 三条，都死在 g27。
  - 审查者的变异里存活的（A 的 MA1、MA2，C 的 `mv`；修复核对一轮 C 的 `ra` 在组合根上）各成了一条发现，修复之后都被抓到。
- 门禁（本机）：
  - 建成时头 `f4d88dd4` 上 `make lint gen-check knip build` 通过；经 `heavy.sh`：`make test` 3,359 秒通过，`make test-go-short` 39 秒，`make e2e` 139 秒（520 个）。
  - 修复一轮头 `bff70ffc`：`make lint gen-check knip build` 8 秒；`make test` 3,579 秒，`make test-go-short` 44 秒，`make e2e` 145 秒（520 个）。
  - 修复第二轮头 `198b0cd5`：`make lint gen-check knip build` 10 秒；第一次 `make test` 576 秒停在第五份的 `TestTheMCPCallStopsWhenTheClientGoes`（负载下 2 秒之内没等到日志，与本轮无关，单独五次都过），整个重跑 3,457 秒全绿；`make test-go-short` 44 秒；`make e2e` 147 秒（520 个）。
- CI：
  - 分支的 run 42770402937（`f4d88dd4`）18 项全绿；第七份在 EPYC 7763 上 Test 一步 292 秒（包 259.0 秒；P1 末 282–288 秒）；costs-2 在 9V45 上 13:03，读的守卫 22.7 秒；guards-core 在 7763 上 19:53，单元的守卫 37.8 秒，a 的中位数 3,165 ms（界 5.2 s，余量最小，约 1.64 倍）。
  - 修复一轮的 run 42787571274（`bff70ffc`）18 项全绿；第七份在 9V45 上包 180.5 秒、Test 一步 201 秒；costs-2 在 9V74 上 22:01（新的时限 45 分钟），读的守卫 36.47 秒（种子 28.6 秒）；guards-core 在 7763 上 19:46（新的时限 40 分钟），a 3,152 ms。
  - 修复第二轮的 run 42819115296（`198b0cd5`）18 项全绿；组合根七份的 Test 一步 240–409 秒（第七份在 9V74 上 240 秒）；costs-2 在 9V74 上 17:14；guards-core 在 Xeon 8573C 上 17:54，单元的守卫 34.03 秒，a 2,606 ms（约 2.0 倍）；`TestLinksReachedReadPlans`、`TestLinksReachedDenseReadPlans` 都过。
- **修复一轮**（2026-11-10，`bff70ffc`；三份审查与主持者的修复说明 `fix-1.md`，决定 268、269 与第 267 条提前）：从 `f4d88dd4` 起 4 个提交，一组一个，普通推送，没有改写已推的历史：
  - `060b365a`：`PlannedPage.Folded` 改名 `Reached`，粘住（写的路径 `rewrote` 为真就设，删掉清除；编辑的路径照旧只设不清）；`ContentWrite.Reached` → `StepContent.Reached`；守卫先看标记、再比正文；码的英文文字改写；升级中带 `Reached` 的字节不变的写也照一次写过守卫、什么也不写（`Unit.guarded`）；`TestThePlanTellsAPageAMoveReached`（原名 `TestThePlanTellsAFoldedRewrite`，`moved` 改 `[]int`，加 A 的三种与 C 的一行）、`TestTheFreezeGuard` 的"什么也没改"与带不带标记、新的 `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds`；g21 改码，加 g23–g25（P2S1A-I1、P2S1C-M1，决定 268）。
  - `8d739bbd`：重新计划加"从 `reviewing`"两格；g14、g15 加 `holds`；形状 a 的注释（P2S1A-N1、P2S1A-N2、P2S1B-N1）。
  - `80da6b1f`：`ci.yml` 的 guards-core 30 → 40 分钟、costs-2 35 → 45 分钟，注释各一段，带 run 号、作业用时与 runner（P2S1B-M1，决定 269，第 267 条提前）。guards-sources 不改，数写在报告里。
  - `bff70ffc`：UP4 的 `base_version` 从 `createPage` 的答复取（P2S1C-N1）。
  - 只改文档的在本 Step 的文档提交：P2S1C-N2、N3；设计 2.1 第 2 条、2.2、2.3、3.1、3.3、3.4、3.6、3.18、6.1、6.2、7.1、7.3、7.5 与第 8、9 节；00 号文档 4.6、第 5、8、9 节与第 11 节。
- **修复核对一轮**（2026-11-10，三位各在 `bff70ffc` 的自己的工作树里）：三位都判旧发现里要改代码的都已关闭（P2S1C-N2、N3 的文档部分在本提交），都判实施者自加的"字节不变、被到达的写也过守卫"必要、不误拒、造出的步与真的写一致。A 以探针第二版试了几条可能绕过第 268 条的路（移动文件夹、P 自己移动、同一页写多次、删除之后重建、带 `Merged` 的写、不是基线页的页），都被挡住或不在范围里；"拿掉、改名、放回"绕得过去（P2S1A-M1）。B 从日志与 API 重算 `ci.yml` 注释里的数，都相符；guards-sources 约 1.8 倍不记作发现，建议下次改到时提到 40 分钟。C 照新代码重跑 `mv` 三处都抓到；加 `ra`、`rw`、`rg` 与关掉字节不变那一处的两条，`ra` 在整个 `TestUpgradeGuardrails` 下活下来（P2S1C-N5）；留给文档提交的清单漏了第 268 条要改的多处（P2S1C-M2）；`contentStep` 的注释过时（P2S1C-N4）。
- **修复第二轮**（2026-11-10，`198b0cd5`；`fix-2.md`，决定 270）：从 `bff70ffc` 起 2 个提交，普通推送：
  - `0b016421`：升级中 `reachedBefore` 照批前的链接问 `BatchLinks.Reached`（到各页最后一个正文操作为止；写的页只有一个正文操作、头已问过时用头的答案），够得着就 `plan.MarkReached`；`TestTheBatchJudgesAPageByItsLinksBeforeIt` 十格（替身照冻结守卫拒带标记的正文步，普通笔记本的格数读）、`TestThePlanTellsAPageAMoveReached` 的 `MarkReached` 各格、g27（含 Q 的两格）（P2S1A-M1，决定 270）。
  - `198b0cd5`：`contentStep` 的注释；g26（P2S1C-N4、P2S1C-N5）。
  - 推之前改订过两次，旧的都没推过：第一版 `ac13346e`、`9761259e`（`x2c` 存活，补 `askedFirst` 两格）；第二版 `1960105e`、`35478201`（n4、20 在组合根上存活，补 g27 的 Q 两格）。中途磁盘剩 14G，停下修剪 Go 的构建缓存（审查记录"环境"）。
- **修复核对第二轮**（2026-11-10，三位各在 `198b0cd5` 的自己的工作树里）：A 判 P2S1A-M1 关：第 270 条的判法范围正好到最后一个正文操作、头的答案复用的条件对、不偏严、新建的页不设、状态变了会重新计划，试过的别的路都被拒，没有新发现；g27 的 Q 两格让 n4、20 在组合根上也死（A 重跑）。C 判 P2S1C-N4、N5 关（`ra` 在 g26 上死），两个提交逐个都绿；P2S1C-M2 部分，另报文档稿的三条 Nit（P2S1C-N6–N8），都在本提交改了。B 判报告的数相符，新的 P2S1B-M2：第 270 条让 M9 已接受的一个形状在升级中计划翻倍，代价没写出，由决定 271 关上（只改文档）；`TestTheMCPCallStopsWhenTheClientGoes` 那次失败与本轮无关，它的两个日志等待推后到 P2/S2 改成 10 秒。之后不再有修复轮。
- **审查的结论**：Important 1（P2S1A-I1），Minor 2（P2S1B-M1、P2S1C-M1），Nit 6；修复核对一轮另 Minor 2（P2S1A-M1、P2S1C-M2）、Nit 2（P2S1C-N4、N5）；修复核对第二轮另 Minor 1（P2S1B-M2）、Nit 3（P2S1C-N6–N8）。都已改或记下。问题在一处规则（第 254 条的"写别的就清掉"让改名之后的写漏过护栏）与它的两个变体（字节不变的写、批里先拿掉再放回）、第 270 条在一个已接受的形状上的代价、两份 CI 的时限与几处测试；守卫的次序、码、账本与两条读的计划三位都没有查出错。
- **合并之后**（2026-11-10）：`main` 在分支开出之后没有动过，分支不用先合 `main`；Step 的文档提交 `6808d8db` 的分支 CI（run 42828548509）18 项全绿之后 `--no-ff` 合进 `main`（`8cf7af36`，树与分支头相同）。`main` 上合并那次的 CI（run 42834026668）18 项一次全绿：七份的测试一步 254、438、405、396、403、318、197 秒；与它之前那次 `main`（42762546769，`c37a3faf`：361、430、389、356、391、304、240 秒）平均 307.5、434、397、376、397、311、218.5 秒，都在 450 秒之下。第二份两次都在 430 秒以上，最贴近界；S2 的第一个提交开第八份（第 261 条）时，照 06-environment 的规矩看要不要从第二份挪测试过去。costs-2 两次 24:41、24:40（时限 45 分钟），guards-core 12:51、12:58（时限 40 分钟）。记合并号的 `ba72cda4` 那次 `main`（run 42839913448）同样全绿：七份 355、346、342、304、287、320、382 秒（第七份换了 runner），与合并那次平均 304.5、392、373.5、350、345、319、289.5 秒，都在 450 秒之下；guards-core 19:44、guards-sources 16:38、costs-2 24:21。
- 实施者留下的五项：Step 的文档提交是本提交；形状 a 在 7763 上余量最小（约 1.64 倍），界守的主要是 M9 的改写，若来了更慢的 runner 最先红的是它（P2S1B-N1）；读的守卫的种子比估的重，costs-2 的时限照第 267 条提前到 S1；UP4 的 Notes 以路径链向 Plan，playbook 照第 9 节写；S2、S3 照设计。

### 10.2 S2：闸门、检查与提交（服务端）（2026-11-11，合并 `c61df45a`）

- 提交（从 main 的 `ba72cda4` 开始，照 6.2 的次序，S1 留下的测试提交排第 2，10 个；Step 的文档提交是主持者的）：
  - 第八份 `BOOTSTRAP_EIGHTH = ^TestUpgrade(Check|Gates|Submit|Tools)`：第七份 `-skip` 它，`test-go`、`test-go-bootstrap` 与 CI 的 `bootstrap-8`（25 分钟），06-environment 的两行（第 261 条）（`9ff43717`）。
  - `TestTheMCPCallStopsWhenTheClientGoes` 的两个日志等待 2 秒 → 10 秒（3.19 交给 S2 的那一条）（`967fdbea`）。没有治好：第一轮 `make test` 里它又失败一次，10 秒里一行也没等到；根因是 SDK 在客户端走后可能照样把答复写进已断的连接，"答复没写出"那一行就不会来（审查 P2S2C-M1），修复一轮删掉那一条断言，3.19 的那一条由此关上（3.20）。
  - `Runs` 认升级的检查、条目与提交：`RunKey.Scope`（检查与条目是升级的 id，提交是提交的 id）、三个 `LintQueryKind`；`Do` 交 `RunResult{Lint, Check}`，各调用者一份自己的；`logFinished` 带 `upgrade_scope`；按页的待建数与第二遍分开问（`WantedByPage`、`AreaWantedByPage`）；检查的类型提前到这里（3.8）（`5d804808`）。
  - page 模块的四条读：`Roots`、`Baseline`、`Preserved`（`LATERAL … LIMIT 1` 逐行经键）、`Changesets`（只经 `changesets_upgrade_id_idx`）；`ReservedKeys()`（3.9）（`8fa3b7c7`）。
  - `JudgeUpgrade`（八道闸门、计数）、`Failed`、`ReportOf`（每条规则的数与前五项，不带游标与路径）（3.10）（`29b1124a`）。
  - 检查与条目：`CheckUpgrade` 照 lint 的码序，经 `Runs`、在快照里再读升级、经 `UpgradeGatePages` 读、判定、取闸门节点的路径，交付之前再判 `notebook.change_mode`；契约 `checkUpgrade`、`listUpgradeCheckItems`；HTTP 适配器；矩阵两行（3.11、3.13）（`223e7d09`）。
  - 提交：两个事务，中间的计算脱离请求的取消，期限取 `submitted_at + SubmitTimeout` 与请求的期限中较早的；第二个事务只写自己那一次提交，交付之前的判定在 `COMMIT` 之前，过期限答 503 与 warn，提交之后 info 一行；`Upgrade.report` 与它存下的编码；网页 `wiki.upgrade_not_ready` 两种语言的文字（3.12、3.13、3.16）（`74a3057a`）。
  - MCP：`upgrade_verify`（lint 的参数；不带 `rule` 答检查，闸门以路径列出节点）、`upgrade_submit`（`{state, submitted_at}`）；非只读的工具带 `destructiveHint: false`；"server update" 的措辞；内置说明两行；`TestUpgradeToolsMatrix`、`coveredMCPRows()`；两行码进 `upgradeToolCodes()`；`validToolArguments` 与 LW8 的工具数（3.14、3.15）（`d8d2bbaf`，范围带 `e2e`）。
  - 两个成本守卫：闸门的读的计划（costs-3）、5 万页的检查（costs-2）；`rollbackReadProblems` 认 `changesets`；`Makefile` 两行、`ci.yml` 的注释，时限不变（7.1）（`a6e8cb67`）。
  - e2e：UP5 的 API 与 MCP 两个版本（第 5 节）（`c3c35bbc`）。
  - 每个提交单独在工作树里 build、vet、`make lint-go`、`make gen-check`、short 测试都通过，另跑它自己的（单元、组合根、网页三项、守卫、UP 的故事）；审查 B 另在 3、6、7、8 号上各自跑同样的四项，8 号另跑竞态的 `^TestToolStrings` 与 `TestUpgradeToolsErrorsCarryTheirCodes`。推送之前改写过两次 7–10 号，旧的都没推过：补变异 o1 抓不到的"只有 `AGENTS` 不过"两格（7 号）；修第一轮门禁的三处（7 号的交错第 7 条，8 号的 `validToolArguments` 与 LW8 的工具数）。
  - 修复一轮另有 9 个、第二至五轮各 1 个（下面），一共 23 个；合并 `c61df45a`。
- 与设计不同之处（实施者的号，同 3.20；都是最小的改动，已改在各节）：
  1. 检查的类型在 3 号提交（6.2）。
  2. `wiki.upgrade_stale_index` 的服务端 `detail` 不说是哪个动作，同网页（4.2）；修复一轮改顺语气。
  3. `Logger` 加 `ErrorContext`（3.16）。
  4. 第二个事务每种结局都照码序判；锁不到笔记本时 `superseded`、答 `notebook.not_found`、不再判（3.12）。
  5. 组合根的 `SetState` 认 `upgrading`（3.12）。
  6. 提交的答复的 `submitted_by` 照用 `named`（不是偏离）。
  7. 覆盖测试认 `/upgrade/check/{rule}`（6.2"不改"）。
  8. 组合根测试的第二位管理员是笔记本的管理员。
  9. 交错第 5 条以 `interleaveOn` 持笔记本行，不包 `States`（7.2）。
  10. `TestUpgradeCheck`、`TestUpgradeSubmit` 的 MCP 一半在 8 号（6.2）。
  11. MCP 的期限是读完请求体之后的 `RequestTimeout`：`request_timeout` 不超过 20 秒时两个入口都是请求的期限先到，期限一格的 MCP 一半照 REST 断言（3.12、7.1；第 273 条）。
  12. 第一个事务的期限一格在 8 号加（7.1）。
  13. `upgrade_submit` 笔记本入口的 `inputSchema` 没有 `properties`（SDK 对空结构体的编码）（3.14）。
  14. `TestUpgradeStatesReachTheStreams` 的提交两格、各两帧（7.1"三格"）。
  15. 报告的条目也不带 `path`（3.10、3.13）。
  16. `TestJudgeUpgrade` 的计数拆成 `TestJudgeUpgradesCounts`（7.3 第 33 行）。
  17. `TestUpgradeToolsErrorsCarryTheirCodes` 的几种情形（3.15）。
  18. 检查的守卫的另一份种子以 SQL 改一页的账本（7.1）。
  19. 检查的守卫在 costs-2 的末尾（7.1）。
  20. UP5 审阅中的冻结用 `edit`（第 5 节）。
  21. UP5 的页名用 `Page.name`（第 5 节）。
  22. 变异 46 改在 `LintCursor.PositionFor`（7.3）。
  23. `TestUpgradeToolsMatrix` 读的行也每格一个库的副本（3.15）。
  24. 8 号的范围带 `e2e`，改 `validToolArguments` 与 LW8 的工具数（6.2、3.14）。
  25. 交错第 7 条读放弃之后的行不看回滚到了哪一步（7.2）。
  26. 2 号的 10 秒没有治好 `TestTheMCPCallStopsWhenTheClientGoes`（3.19；审查 P2S2C-M1）。
- 实测（本机，经 `heavy.sh`；出处见 [S2 的审查记录](reviews/P2-S2-gate-check-submit-review.md)"实测"）：`go test -race -list` 547 个顶层测试，八份 145、51、121、124、62、11、27、6，各在恰好一份；第七份两轮 185.1、179.5 秒，第八份 46.7、46.3 秒；第二份 263.2、264.4 秒（`ba72cda4` 上 250.8、252.7 秒，多的约 5% 摊在许多小测试上：契约变大，组合根每建一个应用就 `apitest.Load` 一次，竞态构建下每次多约 62 ms，审查 B 查清）。5 万页的检查：中位数（每页都保留 / 一页没保留）定界两轮 487 / 487、489 / 485 ms，之后三次 491–501 / 487–494 ms，界 1.5 s（本机三倍与 worker 的八成中较小的）；一次里规则 285–312 ms、来源 76–92 ms、`UpgradePreserved` 86–96 ms，种子约 12 秒。闸门的读：三种统计 × 两种计划，计划都合规，基线 33–63 ms、保留 241–314 ms、变更集 0.3–0.5 ms（十万行的基线，另九本），种子约 14 秒。最坏的报告 35,125 字节（界 65,536；修复一轮之后 35,128 字节）。内置说明经 SDK 3,276、3,346 字节（界 3,796）。vitest 192 个文件、4,345 个测试；`make e2e` 522 个，2.3 分钟。修复一轮之后（第 272 条挪后）：547 个，145、50、121、124、62、11、27、7；第二份 204.4、202.0 秒（f1）、204.1、202.6 秒（头），第八份 98.6、99.8、99.3、100.3 秒（修复第五轮加一格之后 97.9 秒），折到 7763 约 360、193 秒；检查的中位数 474、473 ms。
- 反向对照：7.3 的 S2 各行（25–66）每个点名的测试各跑一次（42 行 59 条）加实施者自己的 10 种（o1、o3–o11，15 条），共 74 条；第一轮 69 条抓到。没抓到的：25b、42b（7.3 把别的条的测试归到这两行，它们由 25a、42a 抓到；7.3 已改）；33 对 `TestJudgeUpgrade`（计数拆进 `TestJudgeUpgradesCounts`，另跑抓到）；o1a、o1b（提交只看 `frontmatter` 一道闸门）是真的缺口，补"只有 `AGENTS` 不过"的单元一格与组合根一格之后都抓到。交错第 7 条的断言改过之后，48、58 在它上面重跑，都抓到。审查者的变异里活下来的 a1、a2、a4、a6、c1、c11–c13、gm3 各成了一条发现（P2S2A-M1–M3、P2S2C-M2、P2S2B-M1）；修复一轮：它们与 B 的怀疑写成的 gm7 在新测试上都死，另加第 274 条的 r274a、r274b 与第 9 条的 n9，13 条都死；修复核对一轮 A 另加 a4b、a6b 与三条改坏秒数的变异，都死；C 的 s1、s2（`wanted_before` 与 `wanted_now` 弄混）在修复一轮的夹具上活，修复第二轮之后死（P2S2C-M4）；修复核对第二、三轮 C 又找到同一类的 p1、p2、m1–m3（P2S2C-M5）与 hr1–hr3、hg1、xg1–xg3（P2S2C-M6）；修复第三轮 72 条（改前活 28 条）、第四轮按投影写的 555 条对调与常量的变异（改前活 217 条），改后都全死；修复核对第四轮 C 把范围放宽到所有逐字段转换，又找到组合根的 wp1（P2S2C-M7）与 k1、lg1（N9、N10），修复第五轮 11 条改前活 4 条、改后全死。
- 门禁（本机，头 `c3c35bbc`）：`make lint gen-check knip build` 8 秒；经 `heavy.sh`：`make test` 3,501 秒通过，`make test-go-short` 41 秒，`make e2e` 138 秒（522 个）。第一轮（`c3bfdd54`）红在 `TestToolStrings*` 两个、LW8 的三个、交错第 7 条，都改了；另有两次偶发：`TestTheMCPCallStopsWhenTheClientGoes`（3.19 交给 S2 的那一个，2 号的 10 秒没有治好，见上）与 M8 留下的 `TestARestoreJobSkipsAnItemRestoredBetweenItsBatches`（负载下作业抢先拿到锁，请求的恢复答它说明里的另一个 409，测试只认一种；审查 B 的 4.7，修复一轮改测试）。修复一轮（头 `b6ef8599`）：`make lint gen-check knip build` 7 秒，`make test` 3,342 秒通过，`make test-go-short` 41 秒，`make e2e` 135 秒（522 个），那两次偶发都没有再出；修复第二轮（`9dce4658`，只改单元测试的夹具）：`make test-go-short` 39 秒与 wiki 模块全部竞态单元 48 秒；第三至五轮（`ef8ac10f`、`d4324ab8`、`43caa32a`）同样跑 build、vet、`make lint-go`、`make gen-check`、`make test-go-short` 与 wiki 模块全部竞态单元（第四轮 40 秒、52 秒），都 rc=0；第五轮改了组合根的一格，另经 `heavy.sh` 跑 `TestUpgradeGates` 与第八份一轮（包 97.9 秒）。
- CI：分支的 run 42882105107（`c3c35bbc`）19 项全绿；第七份在 9V74 上 Test 一步 260 秒（包 234.0 秒），第八份在 Xeon 8573C 上 97 秒（包 66.9 秒）；第二份在 7763 上 457 秒（包 423.8 秒），过 450 秒的线（`main` 上 438、430 秒），即第 272 条；costs-2 在 9V45 上 14:48，检查的守卫 30.7 秒，中位数 726、720 ms（界 1.5 s）；costs-3 在 9V74 上 14:46，闸门的读的计划 34.3 秒，六格合规，`-timeout 20m` 一行在第 11.8 分钟开始（7763 上贴着 35 分钟，第 275 条）。修复一轮的 run 42913206700（`b6ef8599`）第一次 attempt e2e 红在 L1 的旧偶发（带修饰键点链接、等新标签页 30 秒超时，这一轮没有碰网页），只重跑 e2e 之后 19 项全绿，L1 交给 P2/S3 另查；第二份在 Xeon 8573C 上 Test 一步 288 秒，第八份在 9V45 上 147 秒，costs-3 在 Xeon 8573C 上 15:11（时限 40 分钟）。修复第二轮的 run 42921758062（`9dce4658`）19 项一次全绿；第二份在 9V74 上 292 秒，第八份在 9V74 上 170 秒，costs-3 在 Xeon 8573C 上 16:33。修复第三轮的 run 42927783261（`ef8ac10f`）19 项一次全绿：第二份落在 7763 上，Test 一步 400 秒（第 272 条挪后第一次落在 7763，在 450 秒的线之下，比估的 360–374 秒高），第八份在 9V45 上 138 秒，costs-3 在 9V74 上 13:42，costs-2 在 7763 上 24:54。修复第四轮的 run 42940795835（`d4324ab8`）19 项一次全绿：第八份落在 7763 上，187 秒（B 估约 190 秒），第二份在 9V74 上 292 秒，costs-3 在 9V74 上 13:38。修复第五轮的 run 42951453163（`43caa32a`）19 项一次全绿：第二份在 9V45 上 229 秒，第八份在 9V45 上 132 秒，costs-3 在 9V74 上 13:55。
- **审查**（2026-11-11，三位各在 `c3c35bbc` 的自己的工作树里）：A 审检查与提交的正确性，B 审读、成本与份，C 审契约、MCP、矩阵与码、网页的文字、e2e 与文档稿。Important 0，Minor 9（P2S2A-M1–M4、P2S2B-M1、M2、P2S2C-M1–M3），Nit 11，Q 4；修复核对一轮另 Minor 1（P2S2C-M4）、Nit 2（P2S2B-N2、P2S2C-N8），第二、三轮各 Minor 1（P2S2C-M5、M6，与 M4 同一类），第四轮 Minor 1、Nit 2（P2S2C-M7、N9、N10），第五轮 Nit 1（P2S2C-N11）；P2S2B-N2、P2S2C-N7、N11 不改，其余都已改或记下。问题在三处只钉住了一部分的判断（提交的"过没过"、闸门 7 的附件、提交的快照核对）与一个四层都没钉住的成员（`wanted_now.count_capped`）、判计划的帮手对 `nodes` 的口子、设计对 MCP 期限的假定与它带出的 `Retry-After`、costs-3 的时限、两个偶发的根因、e2e 夹具的一个错的不变式；两个事务、期限、码序、键与合并、报告的编码与大小、两个工具与 `Fit` 三位都没有查出错。主持者的决定 272–275（审查记录"主持者的决定"、00 号文档第 11 节）。
- **修复一轮**（2026-11-11，`b6ef8599`；三份审查与主持者的修复说明 `fix-1.md`，决定 272–275）：从 `c3c35bbc` 起 9 个提交，一类事一个，普通推送，没有改写已推的历史：
  - `d7c82c01`（`build, ci`）：第 272 条，照 P2S2B-Q1 改三个模式，`TestVirtualExport` 进第七、八份，各份的命令不改；计数的注释照挪后的 `-list` 写（P2S2B-N1、P2S2C-N5），`SEVENTH`、`EIGHTH` 前的说明、`ci.yml` 的 bootstrap-8 注释、`shards8.py`。
  - `f962a593`（`ci`）：第 275 条，costs-3 35 → 40 分钟，注释写 run 号、runner 与 `-timeout 20m` 那一行开始的分钟数（P2S2B-M2）。
  - `701c736d`：`TestTheMCPCallStopsWhenTheClientGoes` 删掉"答复没写出"那一条断言，注释写明两种结局都对（P2S2C-M1）。
  - `e88cf289`：M8 的两个后台任务的交错，"第三批在先"一支同时认 `page.trash_changed`（审查 B 的 4.7）。
  - `48bb2e18`：测试的缺口：`TestSubmitUpgradeAnswersInOrder` 每格一道闸门不过的七格（P2S2A-M1），`TestJudgeUpgrade` 经 `withAttachment` 的附件三格（P2S2A-M2），新的 `TestTheSubmissionsSnapshotIsItsOwn` 三格（P2S2A-M3），`wanted_now` 四层封顶的夹具（P2S2C-M2），`TestUpgradeToolsMatrix` 照 `r.entries` 与注释折行（P2S2C-N6）。
  - `3893c318`：闸门的读的计划另判 `nodes` 只经 `nodes_pkey`（`gateNodesProblems`），不要数据库的 `TestTheGateReadsReadANodeByItsKey`（P2S2B-M1）。
  - `968bb586`（`server, api, web`）：第 274 条，`retryAfter`；`Execute` 与契约 `submitUpgrade` 的说明同改；`TestTheSubmissionsDeadline` 钉住秒数，组合根两格断言下界并照它等过接管。
  - `debec8dc`：`lintQuery` 拒绝摘要带 `AreaWantedByPage`（P2S2A-N2）；`wiki.upgrade_stale_index` 的服务端文字与它的断言（P2S2C-N4）。
  - `b6ef8599`（`e2e`）：`expectUpgrade` 的 `submission_id` 与答复的 `submitted_at` 有无相同（P2S2C-M3）。
  - 每个提交单独跑 build、vet、`make lint-go`、`make gen-check`、short 测试与它碰到的测试，每一项 rc=0。实施者查了修复说明第 12 条：清掉的变更集不是错误，3.9 记一句。
  - 只改文档的在本 Step 的文档提交：P2S2A-M4、N3，P2S2C-N1–N3、N8 与第 5 节的 P2S2C-M3 那一句；设计 3.7–3.10、3.12–3.16、3.18、3.19、4.2、第 5 节、6.2、7.1–7.3、7.5、第 9 节，加 3.20；00 号文档第 3、5、7、9 节、4.8 与第 11 节。
- **修复核对一轮**（2026-11-11，三位各在 `b6ef8599` 的自己的工作树里）：A 判 P2S2A-M1–M3、N1、N2 关，第 274 条的秒数、契约与两层测试对，组合根断言下界不会偶发，没有新发现。B 判 P2S2B-M1、M2、N1、Q1 与 B 的 4.7 关，挪后的数够，清掉的变更集的结论对；新的 P2S2B-N2（Nit）：`TestTheGateReadsReadANodeByItsKey` 断言了共用帮手的口子，不改，收紧帮手时一并改（7.1）。C 判 P2S2C-M1–M3、N4–N6 关，C-M1 在审查时的负载下 40 次全过；文档稿里 N1–N3 与 P2S2A-M4、N3 都落对了；新的 P2S2C-M4（Minor，下面）与 P2S2C-N8（Nit：稿子里几处测试名与格数，本提交改）。
- **修复第二轮**（2026-11-11，`9dce4658`；`fix-2.md`）：1 个只改测试的提交：REST 与存储的夹具让 `wanted_before`、`wanted_now` 一个封顶、一个不封顶，两个方向各一个（P2S2C-M4）；最坏的报告不变（35,128 字节）。s1、s2 改前活、改后死，c1、c11–c13 照旧死。
- **修复核对第二轮**（C，`9dce4658`）：P2S2C-M4 关（s1、s2 与另加的五条两个方向的探针都死，最坏的报告的取法对）；新的 P2S2C-M5（Minor，S2 建成时就有）：存下的报告的五个计数在夹具里都是 100000，MCP 投影的计数只断言两项，把一个成员写成另一个照过（p1、p2、m1–m3）。
- **修复第三轮**（2026-11-11，`ef8ac10f`；`fix-3.md`）：1 个只改测试的提交，一次扫清计数这一类：判定、REST 的检查与报告、存与读、MCP 的检查，计数的每个成员在夹具里互不相同、整值断言（判定加"each count its own"一格，`fullReport` 的五个数取 11–15，MCP 的 `bigCheck` 七个成员互不相同并另加前封顶、后不封顶一格）；72 条变异改前活 28 条、改后全死；最坏的报告不变。
- **修复核对第三轮**（C，`ef8ac10f`）：P2S2C-M5 关，实施者的 72 条整轮重跑全死、换回旧夹具抽 8 条与改前相同；新的 P2S2C-M6（Minor）：计数之外同类的三处：REST 报告的规则的 `count` 与 `count_capped`（hr1–hr3），`preserved` 的 `nodes` 与 `nodes_kept`（REST 的 hg1、MCP 的 xg1），MCP 投影的闸门的布尔值（xg2、xg3）。
- **修复第四轮**（2026-11-11，`d4324ab8`；`fix-4.md`）：M4、M5、M6 是同一类，这一轮按投影收尾：1 个只改测试的提交，S2 新写的七个投影的夹具每个标量互不相同，八道闸门与 `passed` 排成真假交错的四行，整个答复与写死的期望比，MCP 另加 `TestTheCheckIsAnsweredWhole`；555 条对调与常量的变异改前活 217 条、改后全死，没有发现投影本身有错；最坏的报告不变（35,128 字节）。
- **修复核对第四轮**（C，`d4324ab8`）：P2S2C-M6 关，555 条整轮重跑全死，`gateRows` 九列互不相同，七个投影里这一类收尾。C 再把范围放宽到 S2 写的所有逐字段转换：新的 P2S2C-M7（Minor）：组合根把 page 的 `Preserved` 转成 wiki 的那一行，`NodesKept` 写成 `Nodes` 整个程序照过（wp1），删掉的基线附件会过闸门 7；P2S2C-N9（Nit）：REST 报告条目的 `kind` 写成常量，夹具里只有 `asset`（k1）；P2S2C-N10（Nit）：提交的日志的 `user_id` 没有断言（lg1）。
- **修复第五轮**（2026-11-11，`43caa32a`；`fix-5.md`）：1 个只改测试的提交，三条一起改：`TestUpgradeGates` 加一格"a baseline attachment deleted past the units"（以 SQL 删一个基线附件，`preserved` 整值 4/3/2/2，断言后还原）；`reportedView()` 加一项 `kind: page`，规则页的 `mode` 以 `standard` 再答一次；提交的日志断言 `user_id`。11 条变异改前活 4 条、改后全死，第四轮的 555 条在新头上仍全死；三处转换本身都没有错。
- **修复核对第五轮**（C，`43caa32a`）：P2S2C-M7、N9、N10 关（wp1、k1、lg1 改后死，wp2、k2、k3 照旧死，`hi-MD=llm` 死；新格 4/3/2/2 的取法与"`pages` 必等于 `pages_kept`"的理由对，规则页以 `standard` 再答一次合理）；补查 S2 其余的逐字段转换（ws1、wl1、mv1 死，wb3 等价），这一类收尾。新的 P2S2C-N11（Nit）：`TestUpgradeGates` 以 SQL 改库的两格把还原写在 `check` 里，那一格先 `Fatal` 时后面各格跟着红；不改（只在那一格已经红时影响诊断），以后改这个测试时两格的 `change` 里另用 `t.Cleanup` 注册同一句幂等的还原（7.1）。修复核对到第五轮为止，没有 Important、Minor 留下。
- 实施者留下的八项（报告第 9 节）：Step 的文档提交是本提交；MCP 的提交到不了 `at + SubmitTimeout`，即第 273 条；`TestTheMCPCallStopsWhenTheClientGoes` 与 `TestARestoreJobSkipsAnItemRestoredBetweenItsBatches` 的根因在审查里查清，修复一轮改；第二份过线，即第 272 条；第七、八份合并之后看两次 `main`；costs-2 的余量审查 B 看过，不再提；S3 照设计（网页 `wiki.upgrade_not_ready` 两种语言的文字与 `upgrade_stale_index` 的新文字已在 7 号，契约已生成进 `api-client`）。
- **合并之后**（2026-11-11）：`main` 在分支开出之后没有动过，分支不用先合 `main`；Step 的文档提交 `788cc874` 的分支 CI（run 42956107070）19 项全绿之后 `--no-ff` 合进 `main`（`c61df45a`，树与分支头相同）。`main` 上合并那次的 CI（run 42959287160）19 项一次全绿：八份的测试一步 317、298、457、367、434、235、293、186 秒；同一棵树的分支 run 是 270、363、443、314、469、287、213、187 秒。第三份两次都在 EPYC 7763 上（457、443 秒，均值 450 秒），第五份在 7763 上 469 秒、在 Xeon 8573C 上 434 秒（均值 451.5 秒），都到了 450 秒的线；S2 没有改这两份的测试，是契约变大让各份按比例变慢（审查 P2S2B-Q2）。挪后的第二份在 7763 上 363 秒、在 9V74 上 298 秒；第八份两次都在 7763 上，186、187 秒。照 06-environment 第 4 节，开 S3 之前先把第三、第五份的一部分测试挪到第八份（00 号文档第 276 条），不提高 `-timeout`。costs-2 两次 17:39、18:44（时限 45 分钟），costs-3 11:30、14:25（时限 40 分钟），guards-core 15:36、20:29（时限 40 分钟）。
- **旁支**（2026-11-11，S2 合并之后、S3 开工之前；两条都记在 S3 的文档提交里，同 `m11-p1-shards` 之后的两次 `main` 记在 P1/S2）：
  - **分份**（第 276 条）：旁支 `m11-p2-shards`（`919cc43e`，范围 `build, ci`，从记合并号的 `1b0eb605` 开出，合并 `3959baf9`）。本机量了第三、第五、第八份每个顶层测试的用时（`make -j1`、`-v`，包的时间两轮：第三份 266.1、260.9 秒，第五份 274.6、257.9 秒，第八份 99.5、102.5 秒；7763 的 Test 一步与本机的包之比 1.71、1.76、1.85），挪得最少、各留约 50 秒：第三份最大的 `TestTheSourceRebuildMatchesSuccessfulRandomSteps`（本机 28.0、27.4 秒，7763 约 47 秒），第五份最大且稳定的 `TestTheMCPToolsErrorsCarryTheirCodes`（37.2、36.7 秒，约 65 秒；`TestTheMCPBatchIsRefusedWhole` 均值相同，两轮 44.9、28.4 秒，一个子测试 16.5 秒或 0.6 秒，照负责人的话不追）；两个都不是所在份第一个跑的，不改换付冷启动的测试。两个都进第七、八份的模式（第八份仍是第七份的子集），第五份另 `-skip` 第七份的模式（两份只在这一个测试上重合）。`go test -race -list` 547 个，八份 145、50、120、124、61、11、27、9，换份的恰是这两个（主持者以 `make -n` 展开的 `-run`、`-skip` 另归一次，相同）。挪后本机两轮第三份 238.0、233.7 秒，第五份 224.5、221.6 秒，第八份 167.2、164.0 秒，折到 7763 约 403、393、306 秒；`-timeout`（15m）与作业的 25 分钟不变。分支 CI（42968206091）19 项全绿，第三、五、八份都没有落在 7763 上。
  - **偶发**：记合并号的 `1b0eb605` 那次 `main`（run 42962539666，只改文档）第一次 attempt 第六份红在 `TestIngestInvariantCleanupWindow/live_trimmed_empty`（"default checker silently assumed a purge window"），代码与合并那次全绿的相同；只重跑那一项（attempt 2）全绿。第一次 attempt 的八份 314、371、283、310、403、325、305、164 秒（第六份的 325 秒是红的那一次）。成因：清理作业在应用启动时跑一次（River 的 RunOnStart），之后测试里每小时一次；启动那次删掉 trimmed、没有剩下东西的活的变更集（`PurgeTrimmedChangesets`）与删于保留期之前的（`PurgeChangesets`）。River 的领导晚启动它时，它落在一格的写与核对之间，默认的核对器什么也找不到；在那里手工跑同一条删除，测试照同一句失败。旁支 `m11-p2-flake`（`4e654640`、`77ab7d00`，合并 `edb4faae`）只改两个测试文件：这个测试先等启动那一次（`awaitJob(PurgeKind)`，同文件的 `TestIngestCleanup`、`TestIngestCleanupHeldChangesetJob` 的写法），各格的断言不变。审查者 C 审 `4e654640`：成因对（把同一组清理器在一格的写与核对之间跑一次，空的四格报同一句），共用的库与别的周期作业都排除了，River 换领导之后再跑一次 RunOnStart 实际可不计；改法够、没有掩盖要钉的东西；Nit 两条：P2F-C-N1，注释只点了两种形状，启动那次的截止晚于测试的 `before`，拿得走每个空的格与 `old_with_item` 插条目之前的那一行；P2F-C-N2，格里的 `exec` 用父测试的 `t`，一格的语句失败就结束整个测试、打不出错。C 另找到同类的 `TestARestoreJobAndThePurge/between_two_batches`：启动那次落在 `trashedAgo` 之后时先拿走 Old，两批之间的清理什么也不删，测试照过。`77ab7d00` 照这三条改（注释、`exec` 带格的 `t`、那一格也先等）。本机：改后这个测试 5 次、第六份一轮（211 秒）都过，`make lint-go` 0 issues；第二个提交之后两个测试各 3 次过。分支 CI：`4e654640` 的 run 42968124862 被下一次推送取消，`77ab7d00` 的 run 42968920731 19 项全绿。
  - **挪份之后两次 `main`**：合并 `3959baf9` 那次（run 42972302721）与合并 `edb4faae` 那次（run 42976488607）都一次全绿，八份的 Test 一步 259、336、404、346、379、328、259、243 秒与 400、364、390、371、412、281、252、243 秒，平均 329.5、350、397、358.5、395.5、304.5、255.5、243 秒，都在 450 秒之下（第 8 节第 1 条）。第三份在 7763 上 404 秒（估 403 秒），第五份在 7763 上 412 秒（估 393–422 秒，`TestTheMCPBatchIsRefusedWhole` 慢的那种另加约 29 秒）。S3 从 `edb4faae` 开出。

### 10.3 S3：网页（2026-11-11，合并 `5d3a0f35`）

- 提交（从 `main` 的 `edb4faae` 开始，照 6.3 的次序，4 个；L1 没有站得住的只改测试的改法，没有第 5 个；Step 的文档提交是主持者的；修复一轮另有 6 个、第二轮 1 个（下面），一共 11 个）：
  - `HealthRule` 的读一页、"范围变了"的码与标题级别做成参数（`read`、`moved`、`level`），健康度页交 `wiki.items` 与 `notebook.not_llm_wiki`、行为不变；`HealthItem`、`healthNodeId` 收报告的条目（`UpgradeReportItem`）；`HealthScope.notebookId` 删去（4.3）（`dc8a135a`）。
  - 检查：`UpgradeService.check`、`checkItems`；`UpgradePage` 以 `key={location.key}` 渲染 `UpgradeRead`，这一次进入的 `nonce`、代数、自己的缓存与一直挂着的 `SWRConfig`；`UpgradeCheckSection`（按"检查"才读，视图从 `data`、`error` 推出，八道闸门、三道列节点成链接，七个数，规则的卡与"更多"读检查的条目，`h3`），403 重读外层的列表，`wiki.upgrade_state` 重读升级，`server_busy` 不自动重试；两种语言的文字；假服务端的检查与条目两条路由照真的码序，条目的游标绑在升级的显式范围上（4.3，第 259 条）（`1c070620`）。
  - 提交与报告："提交审阅…"与对话框，`NotebookStore.submitUpgrade`（`inTurn`、`behindSubmit`），自己的提交在算时照实显示审阅中（第 264 条），被拒为没有通过或索引过时就重读并再检查，焦点回"提交审阅…"，成功时焦点给标题；审阅中的 `UpgradeReportSection`（只读，没有"更多"；没有报告时写"正在核对…"）；`health-rule.tsx` 导出 `ruleTitle`、`issueCount`；假服务端的提交照真序分两段（4.3）（`08f858d8`）。
  - e2e：UP5 的页面版本，只在 Chromium（第 5 节）；`fixtures/upgrade-pages.ts` 加 `bannerText.reviewing`（`f0ba987d`）。
  - 每个提交单独在干净的 worktree 里 `make build`、`lint-web`、`knip`、`test-web`、`gen-check` 都通过，4 号另跑 UP5 的三个故事；没有一个碰 `server/`。3 号推送之前 `--amend` 过一次（`.reverse()` → `.toReversed()`，`lint-web` 在 4 号之前才看到），没有推过的东西被改写。
- 与设计不同之处（都是最小的改动；3.21 列出）：
  1. 这一次进入的 `SWRConfig` 一直挂着、只包住检查一节的位置：SWR 2.5.1 在 `SWRConfig` 卸下时删掉 provider 的全局状态，检查在飞时一节卸下又装上，答复在一节回来之后才到时装回的 hook 停在"检查中"（再去掉 `revalidateOnMount` 时另发一次、读两次，仍停住；审查 P2S3C-N12）（4.3）。
  2. `unchanging` 另加 `revalidateOnMount`：已有条目的一代一节装回不读，否则 `server_busy` 之后一节卸下又装上会自己再读（4.3；实施者自己加的，审查 P2S3C-N9 判为对设计的修正）。
  3. "SWR 不调卸下的 hook 的回调"不对（React 18 起只比键）：视图照旧从 `data` 推出，理由改为不依赖回调由谁、何时调；设计写的第 85 行的变异在这里等价，7.3 第 85 行换成 c1（4.3、7.3）。
  4. 提交成功一格持住第二次起的升级的读，不只第二次（4.3、7.3 第 81 行）。
  5. 7.3 第 75 行的变异写成"检查一节卸着时要的一代，装回时不读"（7.3）。
  6. 假服务端的 503 是名额满的那一种（4.3）。
  7. 升级页的 `HealthRule` 的 `paths` 不做事，短暂的"（已不在）"由 `pages` 事件纠正；`HealthScope.notebookId` 删去（4.3）。
  8. "检查在飞时自己提交"两种次序各一格（4.3）。
- 实测（本机，经 `heavy.sh`）：vitest 193 个文件、4,437 个测试（S3 多一个文件、92 个测试），`make test-web` 59–61 秒；`make e2e` 523 个、2.4 分钟；UP5 的页面版本三轮 1.0、1.0、0.99 秒（每轮 4 秒）。
- 反向对照：7.3 的 S3 各行（67–83、85、86）23 条与实施者自己的 14 条，共 37 条；34 条抓到。没抓到的三条都是等价的：第 74 行的"或"两半各自（每次进入一个新的 `Map`，键不带 `nonce` 或离开时不清都碰不到下一次进入；两半合起来即公共的 provider 且不带 `nonce` 抓到），第 85 行（SWR 在 React 18 起照调卸下的 hook 的回调；作证的三格那时看不见卡的第一页，审查 P2S3B-I1；修复一轮让三格看得见之后，卡恒为空、卸下的 hook 不建视图与 c1 都死，85 仍照过）。UP5 的页面版本另有一次反向对照：去掉提交被拒时的再检查，故事失败。
- L1：本机复现（`--repeat-each=30`，空载 3 次、满负载 4 次失败），协议日志显示失败的新标签页在会话建立之中提交了文档却没有 `Page.frameNavigated`，Playwright 因此不报这一页；把文档压后一秒的两种只改测试的改法各有 5/30 的别的失败，不收，故事不改；第 277 条照现状，CI 再红只重跑 e2e、记 run 与 attempt，上游的问题与改测法留给负责人（审查记录"偶发"）。
- 门禁（本机，头 `f0ba987d`）：`make lint gen-check knip build` 18 秒；经 `heavy.sh`：`make test` 3,617 秒，`make test-go-short` 43 秒，`make e2e` 144 秒（523 个），一轮全绿。
- CI：分支的 run 42997780694（`f0ba987d`）19 项一次全绿；八份的 Test 一步 384、376、404、373、246、289、314、207 秒；costs-2 20:56、costs-3 13:44、guards-core 19:43；e2e 8:42（E2E 7:09），L1 这一次没有出。
- **审查**（2026-11-11，三位各在 `f0ba987d` 的自己的工作树里，只跑单个 vitest 文件；修复核对一轮另 Nit 6，见下）：A 审网页行为（检查、提交、竞态与 SWR），B 审测试、假服务端与 e2e，C 审设计一致、文字、无障碍、给 P3 的复用与文档稿。A 0I 2M 4N，B 1I 5M 6N 1Q，C 0I 1M 11N；去重之后 Important 1（P2S3B-I1："检查在飞时"的三格看不见卡的第一页，第 85 行的"没抓到"不能作等价的证据），Minor 6（"更多"的 403 不重读笔记本列表，P2S3A-M1 = C-M1；检查本身答 409、闸门投影跨行的常量、报告一条规则只有一项、树在读时的"加载中"、"更多"的条目卸下又装上之后还在，五处没有测试）。三位都没有查出页面在主路径上的错；8 条偏离都判对，第 1 条的症状、第 6 条与第 74 行的理由照实改写，`revalidateOnMount` 是对设计的修正（3.21）。主持者的修复说明 `fix-1.md`：代码与测试的十三项在修复一轮改；P2S3C-N7 交给 P3（第 9 节）；P2S3B-N6 不改；P2S3B-Q1 的三层防御留着、写明测不到；文档的发现（P2S3A-N4，B-N2、N3，C-N8–N11）在本提交。
- **修复一轮**（2026-11-11，`ff50d516`；主持者的修复说明 `fix-1.md`）：从 `f0ba987d` 起 6 个提交，一类事一个，普通推送，没有改写已推的历史（推送之前改写过两个还没推的：1 号只改标题，6 号补 `lint-web` 的 `no-shadow`，实施者把 `lint-web` 与提交接在一条命令里、没有先判 rc，提交之后才看到）；8 个文件、加 386 行、减 91 行，没有碰 `server/`，只有 3 号改了 e2e：
  - 卡的"更多"答 403 重读笔记本列表：`UpgradeCheckSection` 的 `readItems` 包住 `upgrades.checkItems`，`forbidden` 且 `scope.current()` 时调 `rereadNotebooks()`、再抛原拒绝，`HealthRule` 的接口不变（第 1 条，P2S3A-M1 = C-M1）（`41389247`）。
  - 同一次进入里换了升级，检查不自己读：`UpgradeRead` 的 `checked = { generation, upgrade }`、`recheck(upgrade)`，交给检查一节的 `generation` 在显示的升级不同时为 0（第 2 条，A-N1）（`904d0d83`）。
  - 其他管理员的接入一句；zh 的"待建页面"、`frontmatter` 闸门、en 的两处与"会重新读取"；vitest 与 UP5 页面版本的期望一并改（第 3、4 条，A-N3、C-N1–N3）（`cfbf80fa`）。
  - 闸门投影的六行（不过的数 4、2、3、5、0、1，`pages_worse` 3、7，`stale_pages` 4、12）；报告八条规则的数互不相同、`wanted` 两项、`missing_type` 与 `undigested_sources` 各三项；检查一节的规则夹具反序（第 7、8、12 条，B-M1、M2、N5、N1）（`2c78dc31`）。
  - "检查在飞时"三格断言卡的第一页（Note 1–5）与"更多"；"更多"读到的 7 项在一节卸下又装上之后还在、不另读（第 5、10 条，B-I1、M5）（`b9790af0`）。
  - 检查本身答 409、树在读时的"加载中"、离开时中止、检查答 503 之后结果一行为空且按钮不 `aria-busy`、状态表三格的各节次序、提交的对话框不是 `destructive`、假服务端 `not_ready` 的 `detail`（第 6、9、11、13 条，A-M2 = B-M3、B-M4 = C-N4、A-N2、C-N5、C-N6、B-N4）（`ff50d516`）。
  - 与 `fix-1.md` 不同之处：a-key-no-upgrade-id（键里去掉 `upgrade.id`）修复之后照过，是等价的（实施者报为新的偏离，主持者接受，3.21）；用户看得见的错由 2 号的那一格钉住，实施者加的 f2（代数不看升级）死。
  - 变异（`mut.py`，经 `heavy.sh`，命令统一成三个文件，改前 100 个、改后 107 个测试）：点名的各条与实施者加的 f1、f2 在头上都死，只有 a-key-no-upgrade-id 照过；b25o、b26、c1 在"在飞"三格上死，85 仍照过（等价的证据，3.21）。
  - 逐个提交单独绿（`percommit-p2s3.sh`，经 `heavy.sh`）：vitest 193 个文件，4,438、4,439、4,439、4,440、4,441、4,444 个测试，每次 60–61 秒；3 号另跑 UP5 的三个故事。
  - 门禁（头 `ff50d516`）：`make lint gen-check knip build` 9 秒；经 `heavy.sh`：`make test-web` 60 秒（4,444 个），`make e2e` 140 秒（523 个），`make test-go-short` 第一次撞上 `/tmp` 的配额（链接器写测试二进制时"disk quota exceeded"），压缩自己的大日志、以 `GOTMPDIR` 指向磁盘上的临时目录之后 47 秒通过；服务端没改，`make test` 不跑。
  - CI：run 43010476398（`ff50d516`）19 项一次全绿；八份的 Test 一步 388、370、385、433、247、324、225、239 秒，都在 450 秒之下；e2e 8:48（E2E 7:18），L1 没有出。
- **修复核对一轮**（2026-11-11，三位各在 `ff50d516` 的自己的工作树里，只跑单个 vitest 文件）：A 判 P2S3A-M1、M2、N1–N3 关，审查时的 10 格探针在头上全过；新的 P2S3A-N5（Nit）："更多"被拒为别的码时不重读列表，没有一格钉住（m1-any-code 照过）。B 判 P2S3B-I1、M1、M3–M5、N1、N4、N5 关，M2 部分关：报告每条规则至多 3 项，截到 3 或 4 照过，剩下的一半记为 P2S3B-N7（Nit）。C 判 P2S3C-M1、N1–N6 关，文档稿里 N7–N11 都落了；新的 P2S3C-N12–N14（Nit，文档稿：c14 的症状、7.3 的 a-key-no-upgrade-id、4.3 与第 9 节比修复之后的代码旧）与 N15（Nit，P1 那一格的测试名）。三位都同意 a-key-no-upgrade-id 等价；`readItems` 不判 `scope.current()`（A 的 m1-no-current、B 的 n6、C 的 m1b）照过，也是等价：在飞的"更多"在再检查与离开时被中止，拒绝是 `AbortError`，到不了（主持者接受）。N12–N14 在本提交；N5、N7、N15 由修复第二轮改。
- **修复第二轮**（2026-11-11，`3717deba`；`fix-2.md`）：只改测试，一个提交，2 个文件、加 83 行、减 17 行，接在 `ff50d516` 之后：`keptReport` 照真服务端给条目，每条规则至多 5 项、数在 5 以内的全列（`wanted` 41+ 与 `undigested_sources` 6 各 5 项），八条的数仍互不相同、整组断言（P2S3B-N7）；"更多"答 409 那一格接管笔记本列表的读并计数，断言没有多读（P2S3A-N5）；P1 那一格的测试名跟上新文字（P2S3C-N15）。变异（命令同修复一轮的三个文件）：n1、n2、m1-any-code 在 `ff50d516` 上照过、改后死，b06 照旧死。逐个提交（经 `heavy.sh`）build、`lint-web`、knip、`test-web`（193 个文件、4,444 个测试，58 秒）、gen-check 都 rc=0；没有改产品代码与 e2e，头上不另跑 `make e2e`、`test-go-short`。CI：run 43016566282（`3717deba`）19 项一次全绿；八份的 Test 一步 392、366、276、446、421、216、295、205 秒，第四份 446 秒贴着 450 秒的线（S3 没有动服务端，合并之后看两次 `main`）；e2e 9:03（E2E 7:28），L1 没有出。
- **修复核对第二轮**（2026-11-11，B 在 `3717deba` 的自己的工作树里，三条都是测试）：P2S3B-N7、P2S3A-N5、P2S3C-N15 都关上；另加三条专查新夹具的变异（报告的条目去掉第一项、反序、按 `min(count, 4)` 截）都死，新夹具里没有同类的口子；没有新发现。两轮修复、两轮核对之后没有 Important、Minor 留下；Nit 都已改或在本提交，P2S3C-N7 交给 P3（第 9 节），P2S3B-N6 不改，P2S3B-Q1 的三层防御保留；等价的变异：85、74x、74y、a-key-no-upgrade-id 与 `readItems` 不判 `scope.current()`（m1-no-current、n6、m1b）。
- 实施者留下的五项（报告第 10 节）：Step 的文档提交是本提交；L1 即第 277 条；决定的编号从 277 起（说明写的 276 已是分份旁支的）；给 P3/S3 的写进第 9 节（连同审查 C 补的）；本机的 `bin/nervewiki` 为 UP5 的反向对照 build 过改坏的网页，之后每次 `make build` 都重新 build 过，门禁用的是 `f0ba987d` 的。
- **合并之后**（2026-11-11）：`main` 在分支开出之后没有动过，分支不用先合 `main`；Step 的文档提交 `d475d944` 的分支 CI（run 43022417961）19 项全绿之后 `--no-ff` 合进 `main`（`5d3a0f35`，树与分支头相同）。`main` 上合并那次的 CI（run 43027580603）19 项一次全绿：八份的测试一步 355、371、336、445、427、327、265、289 秒；同一棵树的分支 run 是 362、256、402、440、310、279、292、242 秒，平均 358.5、313.5、369、442.5、368.5、303、278.5、265.5 秒，都在 450 秒之下。第四份两次都在 EPYC 7763 上（445、440 秒），S3 的分支 CI 上另有 446、433 秒，是最贴近线的一份：S3 没有改服务端，是契约变大让各份按比例变慢（审查 P2S2B-Q2，交给 P3）；P3 的第一个 Step 先给第四份减负（挪给余量大的第七、第八份或开第九份），不提高 `-timeout`。costs-2 24:42（7763，时限 45 分钟），guards-core 15:25；e2e 的 E2E 一步 395 秒，L1 没有出。

## 附录：核对过的代码

行号是 P1 收尾之后的 `main` `f002672b` 的。初稿在一个从 `455f692c` 取出的临时 worktree 里逐行读过（服务端、契约与迁移同 `d589e5a2`，`git diff d589e5a2 455f692c -- server api` 为空）；修订时在一个从 `f002672b` 取出的临时 worktree 里照它复核（`git diff d589e5a2 f002672b -- server api`、`git diff 455f692c f002672b -- web e2e Makefile .github` 都为空，其间只改了文档），并核对了修订新引的每一处（下面带"修订"的）。

**page**（`server/internal/modules/page/`）

- `app/extension.go`：`Options` `:18-21`；`Write` `:27-52`；`Step` `:57-77`（`Content`、`Rewrite`、`Batch`）；`StepContent` `:81-85`；`BatchRewrite.MaxPages` `:110`。
- `app/unit.go`：`WriterDeps` `:25-58`；`keepLinks` `:349`；`refusalOf` `:357-363`；`step()` `:366-368`；`applyStep` `:381-428`（守卫的循环 `:392-396`）；`follow` `:545-579`（`:564`）；`Participate` `:584-606`；`appender.WriteContent` `:769-773`。
- `app/unit_content.go`：`ContentWrite` `:17-25`；`writeContent` `:50-107`（`LockContent` `:51`，字节不变 `:66-78`）；`contentStep` `:111-122`。
- `app/unit_relocate.go:19-24`、`:40-41`；`app/unit_rename.go:54`；`app/unit_move.go:84`；`app/relocate_path.go:77-80`、`:85-86`。
- 正文的写的入口：`app/put_page_content.go:64`、`app/edit_page.go:202`、`app/write_path.go:222`、`app/toggle_task.go:86`、`app/restore_version.go:65`、`app/unit_revert.go:130`、`:573`、`app/apply_batch.go:727`；链接的改写 `linking/app/rewrite.go:943`、`linking/app/rewrite_window.go:144`，组合根 `bootstrap/linking.go:271`。
- `app/apply_batch.go`：`maxRewrites` `:66-84`；`planned` 不加锁读状态 `:272-298`（`:277`）、`apply` `:560`、`differs` `:613-618`（第二轮修订）；读的状态 `:410-421`、`CheckReads` 的调用 `:436`（修订）；单元的 `UpdateLinks` `:254-256`；`failed` `:621-634`；`content` `:706-739`（`:717`、`:727`、`:734`）；`move` `:744-776`（`:770`）；推后的改写 `:845`。
- `app/ports.go`：`ContentLock` `:110-114`；`Markdown.FrontmatterEnd` `:786-788`。
- `app/freeze_guard.go`：`UpgradeNodes` `:15`；`GuardWrite` `:50-69`；`guardrails` `:77`（`OpDelete` `:78`）。
- `domain/batch_plan.go`：`MaxBatchReadBytes` `:19-27`（修订）；`MaxBatchParseBytes`、`MaxBatchRewrites` `:28-45`；`Reads` `:847-862`、`CheckReads` `:884-905`（修订）；`Moved` `:233`；`PlannedPage` `:265-274`；`plannedPage` `:279-290`；`:986`。`domain/batch_contents.go`：`content` `:65-`（写 `:78-89`、编辑 `:90-108`、新建 `:76-77`）；`holdsAt` `:183-197`。`domain/batch.go`：`MaxBatchOps` `:44-49`。`domain/content.go:12`（`MaxContentBytes`）。
- `domain/instructions.go:34-44`（`AgentsBody`，`:35`）；`domain/contract.go:88`（`ReservedKeys`）。
- `adapter/markdown/markdown.go:52-54`；`adapter/postgres/queries/upgrade.sql`：`CaptureBaseline` `:12-32`、`RootNodesByKey` `:51-55`、`RollbackLeft` `:141-168`；`adapter/postgres/store.go:337-343`（`PageContent`）；`adapter/postgres/lint_metadata.go:63-65`；`adapter/postgres/queries/nodes.sql:315-316`（变更集的 `deleted_at` 随笔记本写，修订）；`adapter/postgres/rollback_cost_test.go:354-419`（`rollbackReadProblems`，关系表 `:369-371`，修订）。
- `adapter/mcp/batch.go:209-226`（`atOp`）；`adapter/mcp/instructions.go:36-52`（工具段，`lint` `:51`、`pending_sources` `:52`）；`adapter/mcp/instructions_test.go:66-70`（界 3796）；`adapter/mcp/tools.go:271-272`、`:281-282`。
- 模块根：`freeze_guard.go:22-24`；`module.go:232-238`（`MaxBatchRewrites`）、`:302`、`:304`、`:325`；`upgrades.go:33`；`link_targets.go:95`。

**wiki**（`server/internal/modules/wiki/`）

- `app/lint.go`：`Execute` `:24-121`（游标 `:37-48`、没有规则 `:63-73`、`CursorCanonical` `:101-106`、`Runs.Do` `:111-113`、运行之后再判 `:114-116`）；`lintEffective` `:172-180`；`compute` `:196-212`；`computeIn` `:217-234`（`:226`）；`prepareIn` `:239-298`（`:253-259`、`:279-283`）。
- `app/lint_runs.go`：`Do` `:62-124`（预算 `:66-77`、worker `:91`、交回 `:122`）；`TryHold` `:130-154`；`work` `:156`；`finish` `:198-234`（`:216`）；`logFinished` `:295-352`（`:296-305`）。
- `app/lint_run_types.go`：`RunKey` `:13-23`；`RunOutput` `:43-46`。`app/lint_ports.go`：`Paths` `:69-71`；`RuleQuery` `:82-87`；`PreparedRules` `:101-109`；`Runner` `:130-132`；`LintDeps` `:137-145`。`app/lint_projection.go:11`、`:82-101`。
- `app/start_upgrade.go`：`UpgradeDeps` `:22-40`；`locked` `:47`；发起的 `prepareIn` `:258`。`app/upgrade_ports.go:110-116`（`UpgradeRows`）。
- `domain/lint.go:61-67`（`LintQueryKind`）；`domain/upgrade.go`；`domain/upgrade_errors.go`。
- `adapter/postgres/queries/upgrades.sql:10-17`（`CurrentUpgrade`）。
- `adapter/http/handler.go:21-26`（`UpgradeUseCases`）、`:29`、`:65-87`（`ListLintItems`）；`adapter/http/projection.go:21`、`:80`；`adapter/http/upgrade.go:55`。
- `adapter/mcp/tools.go:13-15`、`:18-27`、`:29`、`:31`；`adapter/mcp/lint.go:11-15`、`:22-28`；`adapter/mcp/lint_projection.go:42-78`、`:89`、`:113`、`:120-128`；`adapter/mcp/tools_test.go:35`。
- 模块根：`lint.go:108-135`（`Deps`）、`:149-172`（`New`）；`tools.go:9`。

**linking**：`domain/facts.go:114`（`MaxLinks`，修订）；`app/lint_rule_ports.go:39-46`；`app/lint_rules.go:43-71`（`:44-46`）、`:118-123`（第二遍）。

**platform 与 shared**（`server/internal/`）：`platform/markdown/frontmatter.go:66-96`（`frontmatterSpan`）、`:102-108`（`FrontmatterEnd`）、`:112-119`；`platform/postgres/tx.go:137-144`（`WithinSnapshot`）、`:176-178`（提交的错误，修订）、`:199-203`（`end`）；`platform/httpserver/api.go:77`（`WriteTimeout` 要大于 `BodyReadTimeout + RequestTimeout`，修订）、`:297-310`（`deadline`，修订）；`access/app/authorizer.go:27-48`（`Authorize` 每次读库，修订）；`platform/postgres/pgtest/lockwait.go:25`、`:45`；`platform/httpserver/mcpserver/tool.go:20-26`、`:30`、`:33-41`、`:57`、`:150-156`（`annotationsOf`）；`platform/httpserver/mcpserver/answer.go:17`、`:127`（`Fit`）；`platform/httpserver/mcpserver/schema.go:45-56`；`platform/httpserver/apierrors.go:195-201`、`:258-282`（`ProblemOf`；`ProblemError` `:266-268`、期限 `:278`，修订）；`platform/httpserver/mcpserver/tools_test.go:420-432`（`TestTheAnnotationsAreTheHints`，修订）；`shared/error.go:215-218`。

**组合根**（`server/internal/bootstrap/`）：`wire.go:62-88`（`wiring`）、`:246`（`lintTimeout`）、`:247-276`（`wikiDeps`）、`:256`（`MaxLintRuns`，修订）；`deps.go:304`（`BodyReadTimeout` 是 `ReadTimeout`，修订）；`app_test.go:63`（请求 4 s，修订）；`wiki_lint.go:142`、`:212-234`；`wiki_upgrade.go:109`；`linking.go:271`；`registrants.go`。测试：`upgrade_fixtures_test.go:29`、`:42`；`upgrade_guardrails_test.go:21-91`；`upgrade_interleavings_test.go:46-77`（`heldRules`）、`:75`；`upgrade_permission_matrix_test.go:27`、`:32`、`:111`、`:221`；`permission_matrix_coverage_test.go:158`；`permission_matrix_mcp_test.go:27-41`、`:65-67`、`:93-123`、`:128-150`、`:397-442`、`:444-448`、`:559-646`；`upgrade_tool_codes_test.go:18-24`、`:33-81`；`mcp_client_test.go:305-313`、`:335-341`；`mcp_reads_test.go:429`、`:475-482`；`mcp_tools_test.go:156-160`、`:193-205`、`:344-409`；`mcp_writes_test.go:21`；`interleavings_workspace_test.go:98`（`newAcmeTeamOn`，修订）、`:182-191`；`mcp_batch_test.go:270`（另配期限的写法，修订）；`wiki_lint_runtime_test.go:40-57`、`:76`（`RunObserver`）、`:125`（`event`，修订）；`permission_matrix_mcp_test.go:91-92`、`upgrade_tool_codes_test.go:13-17`（"第七份"的注释，修订）；`upgrade_start_cost_test.go:89`、`:311`、`:441`、`:501`、`:526`、`:633`；`wiki_lint_cost_test.go:27-34`；`wiki_lint_cost_fixture_test.go:41`、`:45`；`wiki_lint_runtime_test.go:69`；`virtual_export_load_test.go:66`。

**迁移**（`server/migrations/sql/`）：`00014_page_changesets.sql:8-23`；`00037_page_changesets_log.sql:11-12`（部分索引，修订）；`00055_notebook_notebooks_upgrade.sql:7-18`；`00056_page_upgrade_nodes.sql:9-38`（`upgrade_nodes_pkey` `:23`，`changesets_upgrade_id_idx` `:34`）；`00057` 的 `notebook_upgrades` 的检查（`submission_state_check`、`report_check`）。

**契约**：`api/modules/wiki.yaml`：`getLint`、`listLintItems` `:7-85`（`:38-85`、`:63-71`）；`abandonUpgrade` `:163-194`；`LintCount` `:294-301`；`LintSummary` `:322-343`；`LintPage` `:344-365`；条目 `:366-549`（`LintWantedItem` `:387-409`）；摘要 `:550-722`；`Upgrade` `:780-857`。`api/modules/events.yaml:35`、`:48-51`；`api/mcp/tools.json`（`lint` `:506-533`、`:1157-1188`，`pending_sources` `:535`、`:1191`；"upgrade"一词 `:205`、`:242`、`:409`、`:782`、`:823`、`:1048`）。

**网页**（`web/apps/web/src/`）：`app/providers.tsx:18`（修订）；`events/handlers.ts:279-283`（修订）；`pages/notebook/health-page.tsx:45-54`（树在公共的缓存里、`key={location.key}`，第二轮修订）、`:66-99`、`:74`、`:127`（这一次进入，修订）、`:162-178`（`onSuccess` 建视图，第二轮修订）；`pages/notebook/upgrade-page.tsx:44`、`:233-234`（`UpgradeRead` 没有 `key`、`UpgradeState` 在外层读树，第二轮修订）；`pages/notebook/health-rule.tsx:183`（卡的 `h2`，修订）；`pages/notebook/upgrade-page.test.tsx:243-262`（`secondReadHeld`）、`:266-294`（`heldServer`，修订）；`pages/notebook/upgrade-page.tsx:33-47`、`:64-222`（读 `:86-96`、`onError` `:91-95`、`change` `:109-134`（`:119`）、`dialogOf` `:136-142`、各节 `:144-173`、对话框 `:179-219`）；`pages/notebook/health-rule.tsx:48`、`:59-70`、`:92-96`、`:114`、`:119-122`、`:160`、`:210-214`；`pages/notebook/health-page.tsx:46-60`、`:125-139`、`:144-150`、`:153-180`、`:188-197`、`:233-234`、`:239-259`、`:299`；`pages/notebook/health-request.ts:7-12`、`:18`、`:40-56`；`pages/notebook/health-item.tsx:13`、`:24`；`services/upgrade.service.ts:17`、`:30`、`:39`、`:48`；`services/wiki.service.ts:12`、`:21`；`stores/notebook.store.ts:47`、`:54`、`:154-194`、`:297`；`app/problem-messages.ts:11-81`（`:44-49`）；`i18n/messages/zh-CN.ts:1110-1115`；`i18n/messages/en.ts:1167-1175`；`test/upgrade-server.ts:22-40`、`:113-235`；`test/lint-server.ts:179`；`pages/notebook/upgrade-page.test.tsx:238`。

**e2e**：`e2e/fixtures/upgrades.ts:11`、`:20`、`:30`、`:46`、`:64`；`e2e/fixtures/assert/wiki.ts:15-52`；`e2e/fixtures/mcp.ts:152`、`:220`、`:227`；`e2e/stories/upgrade/up4-guardrails.spec.ts:8-10`、`:23-58`；`e2e/stories/upgrade/up3-abandon-rollback.spec.ts:59-60`；`e2e/playwright.config.ts:3`。

**构建与 CI**：`Makefile:183-195`（`test-go` 的注释与列表）、`:265`（计数的注释）、`:282`（`BOOTSTRAP_SEVENTH`）、`:288`（`test-go-bootstrap`）、`:314-316`（`test-go-bootstrap-7`，修订）、`:318-320`、`:346-362`（costs-2，说明 `:347`）、`:364-397`（costs-3，说明 `:365`，回滚的读 `:392`，第二轮修订）、`:399-422`（costs-lint）、`:429-466`（guards-core，`:456-461`）；`.github/workflows/ci.yml:22-25`、`:48`（份数的注释，修订）、`:42-110`（`bootstrap-7` `:68-71`，costs 的注释 `:72-84`、guards-core 的注释 `:94-98`，第二轮修订）；`docs/v0.1/claude_to_codex/06-environment.md:27`、`:50`（修订）。

**文档**：`docs/v0.1/M11-upgrade/00-M11-design.md`（`f002672b` 的行号，第 4–10 节同 `d589e5a2`；第 11 节的取舍节 `:813-848`，最后一个是 `:839` 的 246–253；第 12 节 `:862`；4.6 `:179-204`、4.7 `:205-241`、4.8 `:242-259`、4.13 `:311-336`、4.15 `:345-369`、4.16 `:370-393`、4.17 `:394-411`、4.18 `:412-420`、第 5 节 `:429-505`、第 7 节 P2 一行 `:515`、提交说明 `:520-527`、第 8 节 `:634-656`、第 190 条 `:772`）；`docs/v0.1/v0.1-design.md`（`f002672b` 的行号，同 `455f692c`：13.1 `:1047-1179`、13.2 `:1180-1233`、13.4 `:1258-1281`）；`docs/v0.1/M10-llm-wiki/00-M10-design.md:850-860`（决定 160 的表，修订）、`:907`。
