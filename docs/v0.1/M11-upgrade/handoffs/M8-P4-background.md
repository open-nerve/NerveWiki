```yaml
status: open
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
