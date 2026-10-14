```yaml
status: open
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
