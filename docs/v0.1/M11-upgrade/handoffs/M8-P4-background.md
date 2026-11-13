```yaml
status: done
from: M8/P4
to: M11
created: 2026-10-15
```

# 回滚一次升级：大的撤销走后台，停下时怎么办

M11 回滚一次升级，照从新到旧逐个撤销它的变更集（[M8 总设计](../../M8-history-search/00-M8-design.md)"写给后面的 M"）。一次请求撤销不了的（422 `page.revert_too_large`）走 M8/P4 的后台撤销（`POST /api/v0/notebooks/{notebook_id}/reverts`，任务的类型 `revert`），变更集至多 `MaxJobItems` = 60,000 个条目。后台的撤销分批进行，每批在锁下再核对，第一个冲突、第一个被锁的页就停下，已撤销的部分留着，不是整组的（[M8 总设计](../../M8-history-search/00-M8-design.md)第 11 节第 7 条；[M8/P4 文档](../../M8-history-search/04-P4-background.md) 3.5、3.11）。M11 的回滚照这些做：

1. **开始之前**：请求里照 `revertChangeset` 核对一遍，冲突与被锁的页整体拒绝（409 `page.revert_conflict`、`page.locked`），不开任务；条目超过 `MaxJobItems` 答 422，连后台也做不了。一本笔记本同时一个写入任务（导入、恢复、撤销），有任务在排队或运行时答 409 `transfer.busy`：回滚逐个开、逐个等它结束（任务没有推送，读任务行 `GET /api/v0/transfer-jobs/{job_id}`）。
2. **停下时**（冲突、被锁、截掉、取消，或服务重启记成 `interrupted`）：报告写明原因、已处理的与剩下的；任务行的 `changeset_id` 是它写的变更集 R（类型 `revert`、`reverts_id` 是被撤销的 C），各批并入它。
   - **没停在一组中间**：回到之前就撤销 R（同步或在后台），R 的"前"就是 C 的"后"；之后 C 可以再整组撤销。R 的撤销要恢复 R 删掉的各个最上层，各是一组，至多 `MaxJobItems` 组（没有另外的组数界线）。
   - **停在一组中间**（这一组的最上层已恢复，剩下的成员各是已恢复的父节点下回收站的一项）：回收站的几项合不回一项，回不到任务之前。往前：不要先撤销 R，逐项恢复剩下的，父节点活着，各自回到原处，就是 C 的撤销要的样子；计划里之后的组各是回收站的一项，逐项恢复，写回经各页的版本还原。已撤销了 R 时：先恢复 R 的撤销删成的那一项（回到原处），再逐项恢复剩下的，否则它们的父节点在回收站里，会被放到根下。往回：撤销 R 之后这一组留在回收站里分成几项，C 再撤销时答 `changed`。
   - 报告不分这两种（网页的任务列表两种都说，[M8/P4 文档](../../M8-history-search/04-P4-background.md) 4.3）：回滚要分时，M11 设计时定怎么判断。
3. **撤销任务正在写时，不在请求里撤销它的变更集**：一个删除页的撤销任务（例如撤销一次导入）在跑时，在请求里撤销它的 R 常答 409 `page.revert_conflict`，把任务这期间删的页列为 `changed`：请求未加锁地解析之后、加锁之前，任务的下一批又把删除并入了 R（[M8/P3](../../M8-history-search/03-P3-log-revert.md) 3.6 的规则）。什么都不写、任务照常。先取消任务（`POST /api/v0/transfer-jobs/{job_id}/cancel`，在两批之间停下）或等它结束，再撤销 R（[M8/P4A 审查](../../M8-history-search/reviews/P4A-background-server-review.md) A-M2，服务端不改）。
4. **冻结与护栏**：后台撤销的每一批是普通的单元，守卫看到的 `Write.Kind` 是 `revert`、`RevertsID` 是 C，同同步的撤销（[M8/P4 文档](../../M8-history-search/04-P4-background.md) 3.8）：冻结放行回滚的撤销照这个认，后台的批同样放行。
5. **客户端**：任务行的 `client` 只许 `web`、`api`（M7）。回滚若经命令行或别的客户端开后台任务，要先放开它（同 [M9 的移交](../../M9-mcp/handoffs/M8-P4-background.md)第 1 条）。

## 处理进展

- M11 开工（2026-11-08）：照[M11 总设计](../00-M11-design.md)第 7 节"移交的落实"：回滚改为按基线整体复原（00-M11 第 178 条），一个回滚任务、先删升级中新建的子树、计划由基线合成，所以第 1 条（逐个开撤销任务）与第 2 条（停在一组中间）由它取代，合成的计划里没有恢复，继续就是重算，确定性的停下可以结束回滚（第 212 条），都在 P1/S3；第 3 条不适用（回滚中冻结，回滚的任务是唯一的写者）；第 4 条按 `Kind == revert` 放行由第 177 条的 `UpgradeOwn` 取代，在 P1/S1；第 5 条照旧，放弃与驳回经 REST，客户端是 `web` 或 `api`，不放开。仍是 `open`，做完的 Step 各加一行，M11 收尾时改为 `done`。
- P1/S1（2026-11-09，合并 `27b30c7f`）：第 4 条：冻结不按 `Kind == revert` 放行，按 `Write.UpgradeOwn`（第 177 条）：`UnitSpec.UpgradeOwn` 照抄进 `Write`，等于笔记本当前的升级就放过，不等就是缺陷 `ErrStaleUpgradeWrite`（第 225 条），REST 与 MCP 的适配器建不出它（`TestTheFreezeLetsItsOwnUnits`；别人在网页上撤销旧的变更集被拒，`TestUpgradeFreezeLetsOnlyItsOwnWrites`）。S1 里没有设它的生产代码，回滚的单元在 P1/S3 设它。升级中后台恢复与撤销的开始与它们的每一批都被拒（`notebook.upgrading`）。仍是 `open`。
- P1/S2（2026-11-09，合并 `9c7c6821`）：第 4 条的 `UpgradeOwn` 有了第一处设它的生产代码：发起补写的页经 `SeedSpec.Upgrade` 成为这次升级自己的写，变更集带 `upgrade_id`（`TestASeedForAnUpgradeIsItsOwn`），回滚的单元仍在 P1/S3；第 1 条的"一本笔记本同时一个写入任务"：发起时有排队或运行的写入任务答 `transfer.busy`，正在上传的导入在发起之后建任务被拒、文件删掉（交错第 4 条）。仍是 `open`。
- P1/S3（2026-11-10，合并 `8af1bff6`）：第 1、2 条由回滚取代、做完：一个回滚任务（种类 `rollback`、带 `upgrade_id`），每一遍先删不含基线节点的新建子树（不计入界），没有了就由基线合成一遍撤销，至多 `MaxRollbackItems = 300,000` 个条目，复用撤销的批；没有恢复、没有"停在一组中间"，继续就是重新算差，确定性的停下可以结束回滚（`TestUpgradeRollback`）；第 4 条：回滚的单元设 `UpgradeOwn`，冻结按它放过，升级换了时单元答 `ErrStaleUpgradeWrite`（交错第 16 条），"任务的批"的 `rolling_back` 一行里别的任务的一批被拒、回滚自己的批放过；第 5 条：放弃经 REST，回滚任务的客户端是 `web` 或 `api`。仍是 `open`。
- P1/S4（2026-11-10，合并 `ce370441`）：第 2 条的网页一半：升级页以任务页的整行 `TransferJobRow` 显示回滚的任务（进度、状态、报告），停下之后不刷新页面就出现"继续回滚…"与"结束回滚，保留现在的样子…"（结束的对话框写剩下的三个数与链接可能指不到的一句，三个都为 0 时只写"已经复原完"）；任务页与升级页的"取消"对回滚的任务只给笔记本管理员（第 229 条）。仍是 `open`。
- P3/S1（2026-11-12，合并 `daf14dd1`）：第 5 条的驳回一半：驳回经 REST（`rejectUpgrade`），回滚任务的客户端照 `shared.ClientOf` 是 `web` 或 `api`（3.7，单元断言"from web"）；任务排在别的写入任务之后时笔记本冻结到回滚做完（UP6 以另一本笔记本的导入占住导入 worker、确定地断言 agent 的写答 `notebook.upgrading`）。仍是 `open`。
- M11 收尾（2026-11-13）：做完，状态改为 `done`。第 1、2 条由回滚取代：一个 `rollback` 任务，先删新建的子树，计划由基线合成，停下之后继续就是重算，确定性的停下可以结束回滚（00-M11 第 178、193、212 条；`TestUpgradeRollback`、`TestRollbackPlansPutTheBaselineBack`）；第 3 条不适用（回滚中冻结，回滚的任务是唯一的写者）；第 4 条由 `Write.UpgradeOwn` 取代（第 177 条），冻结不看 `Kind`（`page/domain/freeze.go`，`TestUpgradeFreezeLetsOnlyItsOwnWrites`、`TestTheFreezeLetsItsOwnUnits`）；第 5 条照旧，放弃与驳回经 REST，任务的客户端是 `web` 或 `api`。第 2 条的"之后撤销 R"对回滚的变更集在一种常见的形状下答冲突（第 302 条），v0.1 由一个整个程序的测试钉住，修好在 [M12 的历史移交](../../M12-release/handoffs/M8-history.md)第 6 条。证据逐项记在[收尾审查](../reviews/M11-closeout-review.md)。
