```yaml
status: done
from: M8/P4
to: M9
created: 2026-10-15
```

# MCP 开的后台任务与 batch 的大撤销

M8/P4 加了两种后台任务：从回收站恢复一项（`trash_restore`，`POST /api/v0/notebooks/{notebook_id}/trash-restores`）与整组撤销（`revert`，`POST /api/v0/notebooks/{notebook_id}/reverts`），同导入是 transfer 的任务、在导入的队列上（[M8/P4 文档](../../M8-history-search/04-P4-background.md)）。网页先在请求里恢复、撤销，答 422 时问是否在后台进行。留给 M9 的：

1. **MCP 开的后台任务**：`transfer_jobs.client` 只许 `web`、`api`（M7），MCP 开导入、后台的恢复与撤销之前要先放开它为 `mcp:<客户端名>`；任务以 `Actor{UserID, JobID}` 代发起人执行，写入单元记任务行的客户端。agent 照网页的流程：先在请求里撤销（`revertChangeset`）、恢复（`restoreNode`），答 422 时开后台的任务，读任务行等它结束（没有推送）。开始的答复丢了时，照网页的规则找这次的任务：这个账户这种类型与目标的、进行中的不论时刻，已结束的只取开始之前没见过、创建不早于开始之前一分钟的（[M8/P4 文档](../../M8-history-search/04-P4-background.md) 4.2）。
2. **`batch` 的大变更集的撤销**：同走后台，受 `MaxJobItems` = 60,000 与每批的再计划：每批在锁下以只含这一批的节点再跑 `PlanRevert`，一组跨批时之后各批的成员不进再计划、逐个核对（3.5）。[M8/P3 的移交](M8-P3-revert.md)的推后、多轮与临时名称要在分批里同样成立：推后的步与临时名称的改名跨批时怎么切、一批的再计划看不到之后的批的步，M9 引入 `batch` 时定，并加进后台撤销的交错。一批至多 100 个节点，远在 `MaxRevertSteps` 之内。
3. **`log` 看得到任务的变更集**：任务行的 `changeset_id`（建行时定下，第一个写了东西的批以它建变更集）是普通的变更集：类型 `trash_restore` 或 `revert`，各批的条目带它那一批的时刻（撤销认最上层的例外按条目的时刻，3.7）。`log` 照常列出它，撤销它照普通的变更集。
4. **在请求里撤销正在写的撤销任务的变更集答 409**（[M8/P4A 审查](../../M8-history-search/reviews/P4A-background-server-review.md) A-M2）：同 [M11 的移交](../../M11-upgrade/handoffs/M8-P4-background.md)第 3 条，agent 先取消任务或等它结束，再撤销。

## 处理进展

- M9/P2、P3（2026-10-19 至 10-21，合并 `f3854b9a`、`56ef34c8`）（[M9 总设计](../00-M9-design.md) 4.12）：
  - 第 1 项：不做（M9 总设计第 11 节负责人的决定第 1 条）：`transfer_jobs.client` 仍只许 `web`、`api`（`00029_transfer_transfer_jobs.sql`，之后没有迁移改它），transfer 没有 MCP 的适配器；放开它留给 [M11 的移交](../../M11-upgrade/handoffs/M8-P4-background.md)第 5 条（M9 收尾另写 [M11 的 M9 的移交](../../M11-upgrade/handoffs/M9-mcp-upgrade.md)第 4 条）。
  - 第 2 项（P3 的 S3）：任务开始时以 `schedule()` 排一次计划（`JobWrites.PlanRevert`，`page/app/jobs.go`；紧凑的 `domain.JobStep`）；三种不可分的段（`asideSpans`、`pageJoins`；段里的恢复只截到段里的放回的目标父节点，`needed`）；删除照那一刻的子树、照单元读子树的次序；transfer 的 `backgroundRun.batch` 只在段之间切，`halved` 落在段的边界上；一段过了单元的界线（`errPastNodes`、`errPastPages`、`errPastHistory`、`errPastLinks`，节点与页按计划的大小在任何读之前，`withinSizes`）以 `revert_too_large` 停下（`FailureRevertTooLarge`，记一行 warn；网页的文字 `transfer.failure.revert_too_large`，中英）；每批（`Unit.RevertBatch`）核对 L（`CheckRevertPart`）与续接，再按次序核对前提（`CheckRevertSteps`），冲突照 S\* 的名称（`starNamed`），一组在一批里恢复一次，写回在刚核对过的版本号上。测试：`TestTheJobsStepsReplayToStar` 与 `bootstrap/revert_job_test.go` 的六个（M9 收尾加了 `TestTheRevertsJobWaitsForAParentsLineToMoveUp`，`14dec63c`）。契约 `startRevert` 的描述照建成的改写（M9 收尾 C-M12，`c2f0bcd8`）。
  - 第 3 项：不用改：任务的变更集是普通的变更集，`log`、`changeset` 读得到（`TestTheMCPHistoryIsRESTs` 里有一个导入任务的）。
  - 第 4 项：不涉及：agent 不撤销（负责人的决定第 1 条）。
- M9 收尾（2026-10-23）：核对过各项的落实（[M9 收尾审查](../reviews/M9-closeout-review.md)"handoff"，C 逐项对照代码），关闭。
