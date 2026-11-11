```yaml
status: open
from: M10/P1
to: M11
created: 2026-10-27
```

# 模式的契约表、模式变化的订阅者与冻结的拒绝

**P3/S1实际接入的移交（2026-10-31）**：内部LintScope的显式Contract＋AreaRootID与implicit identity隔离，effective standard的root=nil，stored standard可explicit LLM＋拟议root；零UUID不是nil。SourceArea/LintData完整整区不依赖hash；调用不得处于已有事务内，名额/期限及交付前授权仍适用。literal-v1现已实际计stale_pages，M11须按既有闸门拒批准或先reindex；不启动M11。完整item_key、cursor、规则与说明SDK余1003/933B见[M10/P3/S1审查](../../M10-llm-wiki/reviews/P3-S1-lint-protocol-review.md)。P4 Unknown/未改页coverage已随P4/S2实施，status仍open。

**P3来源格式前置的移交（S1-0a历史记录，以下后续状态是原时点）**：当前Extractor为2，literal-v1为旧格式。后续lint实施时须计入stale_pages；M11按[M10总设计](../../M10-llm-wiki/00-M10-design.md) §7拒绝批准或先reindex，不能过滤旧页后称规则全过。S1-0a未实现lint或M11闸门，前置验证见[P3 §10.1](../../M10-llm-wiki/03-P3-lint-health.md#101-s1)。后续ingest仍遵守[P3](../../M10-llm-wiki/03-P3-lint-health.md) §3.8.2的KnownAbsent/Ineligible/Known/Unknown及未改页coverage；新页已知不存在不等于全本旧消化为空。M11未开始，status保持open。

M10/P1 建成了模式与模板、模式守卫的框架、预检与后台拒绝的去处，以及网页的模式变化处理（[P1 设计](../../M10-llm-wiki/01-P1-contract-frame.md)第 8–10 节；[S4 审查](../../M10-llm-wiki/reviews/P1-S4-web-review.md)）。这里移交 P1 实施后明确的契约与接入位置；M11 的升级流程尚未开始。原有的[事件移交](M5-events.md)、[回滚一次升级](M8-P4-background.md)与[MCP、冻结及回滚调度移交](M9-mcp-upgrade.md)继续适用。

1. **模板与模式的契约表**：`wiki/domain.ContractOf` 是同一份契约表；普通模式没有模板，LLM Wiki 的两种语言模板依次写来源根 `raw` 与 `AGENTS`。模块根的 `wiki.TemplatePages(mode, locale)` 从它取模板，交回副本；组合根与跨模块接入经模块根，不直接导入别的模块的 domain。升级读取已有契约，不另存一套模板或从当前笔记本的正文推断契约（`wiki/domain/contract.go`、`wiki/module.go`；[P1](../../M10-llm-wiki/01-P1-contract-frame.md) 3.8–3.9）。

2. **先写行，再在同一个事务里通知订阅者**：`notebook.ModeSubscriber.ModeChanged(ctx, ModeChange)` 在笔记本的行写成新模式之后调用，仍在写行的事务内；按登记次序调用，第一处错误停止并回滚整件事。`ModeChange` 带工作区、笔记本、前后模式、执行者与同一个修改时刻。M11 的模式变化也经过这条订阅链，不能只写行或在事务之后另发事件。当前降级经 `SetMode` 写行后再 `publishMode`；`TestADowngradePassesTheModeSubscribers` 在真库里核对订阅者读到新行、事务、审计列与失败回滚（`notebook/app/extension.go`、`notebook/app/downgrade_notebook.go`、`notebook/extension_test.go`）。

3. **沿用同一个 `mode` 事件**：组合根的 `notebookEvents.ModeChanged` 经事件发布者发布 `mode`，按笔记本过滤。发布数据是 `{}`，帧里的 `workspace_id`、`notebook_id` 由事件流补入；不把前后模式另放进帧。服务端最后一跳是 `TestTheModeEventReachesTheStreams`，契约是 `EventMode`。网页的真实 `modeChanged` 已登记，重读该工作区的笔记本列表；`app/event-stream.test.tsx` 经实际处理表核对最后一跳，通用扩展测试的编造类型已改为 `later`。M11 沿用它，其他升级状态事件的细节留给 M11；原[M5 事件移交](M5-events.md)在 M11 开工时一并处理。

4. **模式与来源根一次写入；普通模式可先指定根**：`notebook/adapter/postgres/queries/notebooks.sql` 的 `SetNotebookMode` 用一条 `UPDATE` 同时写 `mode`、`source_root_id`、`updated_by_id`、`updated_at`，经存储的 `SetMode` 在调用者的事务与行锁下使用。`00048_notebook_notebooks_mode.sql` 的检查实际是 `mode <> 'llm_wiki' OR source_root_id IS NOT NULL`：它要求 LLM Wiki 有来源根，**不要求普通笔记本没有来源根**，为 M11 先定来源根、后批准模式留出了位置。来源根没有外键，理由与清理次序见该迁移。当前普通创建不给来源根、降级清为 null，是这两个用例的行为，不能反推为更强的数据库约束。升级状态仍是 M11 自己的状态，不增加 `notebooks_mode_check` 的模式值；批准时实际的写入与事务编排由 M11 设计。

5. **冻结守卫的授权与拒绝沿用单元的边界**：`Write.Grants` 是单元自己的额外授权备忘，以指针随步骤传递，不能把答案留在共用的守卫上。当前由模式守卫与单元锁下的检查填写，其他扩展可读；锁外预检单独授权，不把答案带进单元。M11 的冻结扩展以这条边界为基础。拒绝经 `page/domain.Refused` 标出节点并 `Unwrap` 原问题；`batch` 的失败项、重新计划与后台任务都已有处理，模块根 `page.Refusal` 给组合根读码与节点，transfer 的 `Refused` 保留原问题。`TransferRefusal.code` 是自由字符串，新增冻结码不必扩这个字段的枚举；新操作的声明、错误文字与各入口的错误契约仍照通用约定落实（[P1](../../M10-llm-wiki/01-P1-contract-frame.md) 3.19、3.22、3.26；[M9 移交](M9-mcp-upgrade.md)第 2 条）。

`notebook.change_mode` 给发起与批准升级、根下保留标题与 `AGENTS` 的处理，以及冻结的次序、lint 的显式契约与来源范围、批准时哈希的重建、内容保全、说明和降级的关系，已有[M10 总设计](../../M10-llm-wiki/00-M10-design.md)第 7 节“写给后面的 M”和 [P1 第 9 节](../../M10-llm-wiki/01-P1-contract-frame.md)的要求。本移交指向它们，不把 P2–P5 后续才提供的能力写成 P1 已经实现。

### M10/P2补充（来源契约；升级仍未开始）

P2/S1–S2已完成来源事实/rewrite/hash；S3网页与后续集成见[P2结果](../../M10-llm-wiki/02-P2-source-area.md#103-s3-完整来源网页)。批准变为LLM时须在同事务重建指定根严格后代source_hash并清区外；普通模式虽可预指定SourceRootID，hash仍NULL。保全revision.rewrite，旧无标记/Down-Up按保守规则；SourceHashes、ModeSubscriber顺序与free refusal.code沿既有接口。此为已实现契约的交接，不设计升级流程；status仍open。SourceRootID的注释已在M10收尾改正，SQL约束不加强。

### M10/P4补充（ingest；升级仍未开始）

P4的ingest只相信当前来源表示与充分旧／未改证据。旧v1、missing/stale必须先reindex，之后恢复未来可靠判断，不补此前历史标签；Unknown与PossibleTargets不是升级ready状态。批准升级前仍处理旧根index/log保留名，否则whole export可能contributor_conflict。本移交不启动M11，也不把未改页最终树／解析近似写成全unit旧快照精确。

### M10 收尾补充（升级仍未开始）

- **已有的笔记库**：把 LLM Wiki 的导出重新导入一本新的 LLM Wiki，得到 `AGENTS 2` 与 `raw 2`，来源落在来源区以外（`bootstrap/virtual_export_import_test.go:36`、LW10，README"重新导入的限制"）。M10 不合并。已有的笔记库（含这样导回来的）走升级定来源根，M11 设计时写明这种库怎样走。
- **根下的保留标题**：升级之前根下可能有 `index`、`log`、`index.md`、`log.md` 的真实节点，页与附件都算，含一个 `index.md` 的文件夹页，它让整本导出答 `contributor_conflict`。批准之前改名或拒绝（5.8 第二道闸第 3 条）。
- **回收站里的保留标题**（收尾审查 A-M1）：批准之前删掉的根下 `index`，批准之后撤销那次删除会把它带回 LLM Wiki 的根：`CheckContract` 只判建、改名、移动，撤销的恢复不加序号（`page/app/unit_revert.go`），放回保留的根答的 `page.virtual_file` 又不在 `revertChangeset` 的码里。M10 里走不到（根下没有这样的节点）。M11 要判 `OpRestore`，或批准时一并处理回收站。
- **upgrade 的 prompt**：登记在 wiki 的 `Prompts()`，`Entry: NotebookEntry`、静态，先写 `api/mcp/prompts.json`，`TestThePromptListsAreTheContract`、`TestThePromptReadsLLMWikiMatrix` 加行。方向相反：`wiki/app/playbooks.go` 的 `Execute` 对不是 `llm_wiki` 的笔记本答 `notebook.not_llm_wiki`，upgrade 要在升级中的普通笔记本上答、在别的状态答错误，不能原样经它。
- **闸门**：经 wiki 的 `Lint` 内部入口，传 `LintScope{Contract: LLM Wiki 的, AreaRootID: 提议的来源根}`，一个快照里取。第 4 条是 `invalid_frontmatter`，第 5 条是 `missing_type`、`missing_summary`、`invalid_sources`；第 6 条要自己的数：`wanted` 数不同的写下的目标，不含读作附件的、虚拟的与来源发出的，到 10,000 就停。`stale_pages > 0` 时拒绝提交或先 reindex。
- **内容保全**：第 7 条"差异只来自系统的链接改写"按版本行的 `rewrite` 判。
- **守卫的次序**：冻结与护栏登记在 `pageRegistrants` 的守卫里，建议排在模式与锁之前；`Write.Notebook` 是加升级状态的地方。
- **说明与降级**：说明里不用"upgrade"一词（它指服务端的升级）；降级在升级期间如何（拒绝或先放弃升级）由 M11 定；`notebook.change_mode` 可以给发起与批准用。

## 处理进展

- M11 开工（2026-11-08）：照[M11 总设计](../00-M11-design.md)第 7 节"移交的落实"逐项做。第 1 条（模板）与第 2、3 条（订阅链与 `mode`）在 P1/S2（`mode` 的说明在 P1/S1）；第 4 条：不先写来源根（第 194 条），批准时一次写模式与根，在 P3/S1；第 5 条（冻结沿用单元的边界）在 P1/S1。P2 补充的来源哈希重建在 P3/S1、护栏放行系统改写在 P2/S1；P3/S1 补充（显式范围、零 UUID、名额与期限、交付前授权、`stale_pages`）在 P1/S2 与 P2/S2，发起在写事务里算 lint 是写明的偏离（第 187 条）；P4 补充与收尾补充：已有的笔记库在 P3/S2 的 README，根下的保留标题是闸门 3（P2/S2），回收站里的保留标题（收尾 A-M1）是第 181 条（P3/S1），upgrade 的 prompt 在 P3/S2，闸门 6 逐页（第 180、188 条）与内容保全的绊线（第 191 条）在 P1/S1、P2/S1、P2/S2，守卫的次序在 P1/S1，说明里的"upgrade"一词在 P2/S2。仍是 `open`，做完的 Step 各加一行，M11 收尾时改为 `done`。
- P1/S1（2026-11-09，合并 `27b30c7f`）：第 5 条：冻结的守卫沿用单元的边界：发起人的 MCP 写每个单元经 `Grants` 问一次 `notebook.change_mode`，拒绝由 `refusalOf` 标出，`batch` 整个回滚、后台任务的一批以 `refused` 停下（`TestUpgradeFreezeReachesEveryPath` 的"任务的批"）；收尾补充的守卫的次序：`[freeze, mode, lock]`，冻结排在守卫与否决者的最前（`TestUpgradeFreezeAnswersFirst`），`Write.Notebook` 带升级；第 3 条的 `mode` 的说明改了（`events.yaml`），最后一跳在 P1/S2；内容保全的删除一半：基线节点与补写的来源根删不掉（删除的护栏，`page.upgrade_protected`，`TestUpgradeGuardrails`）。仍是 `open`。
- P1/S2（2026-11-09，合并 `9c7c6821`）：第 1 条：发起补写的 `raw`、`AGENTS` 取 `wiki.TemplatePages` 的那两页（启动时解析好的模板，`Template.SeedUpgrade`）；第 2 条：`notebook.NewUpgradeStates` 在发起的事务里先写笔记本行的升级四列、再 `Publish` 给降级用的那组模式订阅者，出错就回滚整个发起；第 3 条：发起发一次 `mode`（`From`、`To` 都是 `standard`），最后一跳 `TestUpgradeStatesReachTheStreams`（订阅者交空时失败）；P3/S1 补充：发起以显式的 LLM Wiki 契约与提议的根（从不为零）调 `prepareIn`，在笔记本行的锁下经 `Runs.TryHold` 立即持名额、整个事务受发起的期限（请求的四分之三，00-M11 第 232 条），`stale_pages > 0` 答 `wiki.upgrade_stale_index`。仍是 `open`。
- P1/S3（2026-11-10，合并 `8af1bff6`）：第 2 条：放弃、继续、回滚停下、回滚做完与结束回滚都经 `notebook.NewUpgradeStates` 先写笔记本行（`SetState`、`Clear`）、再在同一个事务里 `Publish` 给降级用的那组模式订阅者，出错就回滚整件事；回滚的结束在 transfer 的结束事务里经 wiki 的 `RollbackEnd`（先笔记本行、`FinishJob`、再 `End`）；第 3 条：这五种各发一次 `mode`（模式不变，`From`、`To` 都是 `standard`），`TestUpgradeStatesReachTheStreams` 的 S3 八步各断言一帧。仍是 `open`。
- P1/S4（2026-11-10，合并 `ce370441`）：第 3 条的网页一半：`modeChanged` 另以笔记本 id 重读 `["upgrade", notebook_id]`，每次连上在笔记本列表之后重读它（`refreshedOnConnect` 的第三层，连上时已不是管理员就不读）；横幅、只读（`writesNotebook`）与编辑器失去编辑都随 `Notebook.upgrade` 变，发起人的网页同样只读；收尾补充的 upgrade 的 prompt：升级页已对发起人写 prompt 名 `upgrade`，prompt 本身在 P3/S2，那时核对名字、页上的话与 UP1 页面版本的断言一致（决定 253）。仍是 `open`。
- P2/S1（2026-11-10，合并 `8cf7af36`）：P2 补充的"护栏放行系统改写"：守卫不比 `Rewrite` 的步（参与者的链接改写与撤销写回的改写），单元也不为它们算 `BodyHash`（g4、g10，`TestTheFreezeGuard` 的 `Rewrite` 一格）；收尾补充的"内容保全按 `rewrite` 判"的 S1 一半：护栏只许改基线页的 frontmatter（切掉 frontmatter 与一个 BOM 之后的正文逐字节比），每个正文步（含系统的改写）写升级的账本，升级中先核对账本等于写之前的哈希，绕过单元改过的页答缺陷（g17、g18），闸门 7 在 P2/S2；第 5 条：护栏是冻结的守卫放行之后的一段，拒绝经 `refusalOf` 标出，`batch` 整个回滚，正文的两个码标在这一页最后一个正文操作。仍是 `open`。
- P2/S2（2026-11-11，合并 `c61df45a`）：P3/S1 补充的 S2 一半：检查与提交以显式的 LLM Wiki 契约与提议的来源根（从不为零）经 `Runs`，在一个快照里读，同一组名额与 `LintTimeout`，超出答 503；交付之前再判 `notebook.change_mode`（检查在答复之前，提交在第二个事务的 `COMMIT` 之前）；`stale_pages > 0` 时检查的闸门 8 不过、提交答 `wiki.upgrade_stale_index`（服务端与网页的文字都不说是哪个动作，请服务器管理员 reindex 再试）。收尾补充：闸门以 `JudgeUpgrade` 判，`frontmatter` 是 `invalid_frontmatter`、`missing_type`、`missing_summary` 三条，`sources` 是 `invalid_sources`；根下的保留标题是闸门 3（根下键为 `index`、`log`、`index.md`、`log.md` 的页与附件，`page.ReservedKeys()`）；闸门 6 逐页（第 180、188 条），闸门 7 比账本（第 191 条，`UpgradePreserved`），每道闸门过与不过各有格（`TestJudgeUpgrade`、整个程序的 `TestUpgradeGates`）；说明里的"upgrade"一词改为"server update"（page、linking 的工具，`tools.json` 六处，`events.yaml`）。仍是 `open`。
