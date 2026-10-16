```yaml
status: done
from: M4 收尾
to: M8
created: 2026-10-03
```

# 页面的历史、回收站与恢复

M4 的写入单元给每个写记变更集、按节点合并的条目与正文版本（[M4 总设计](../../M4-pages/00-M4-design.md)第 4 节"变更集"）；删除是软删除，跟随的行以同一时刻软删除（第 4 节"软删除"）。M8 做历史与恢复时（[M4 收尾审查](../../M4-pages/reviews/M4-closeout-review.md) C-M4）：

1. **重排不记条目**：新建一页时兄弟的次序挤不下，单元给整组兄弟重新编号，不移动任何一页，不记条目、不进事件（[P1 审查](../../M4-pages/reviews/P1-page-module-pipeline-review.md) Q2）。所以条目里的 `before_sort_order`、`after_sort_order` 与之后的次序值可能对不上：判断"移动过没有"、还原次序时比较兄弟之间的相对位置，不比较原始的次序值。
2. **回收站里的页面照样有历史**：删子树时节点、正文、条目与版本以同一时刻软删除，变更集留着（里面可能还有别的页的条目）；删笔记本时连同它的变更集。清理之前都在。恢复时连同跟随的行一起清空 `deleted_at`（同一时刻删的那一批）。
3. **同一变更集里的删除与恢复**：一个有"后"的改动合并进已在回收站的条目时，`RecordItem` 会把条目的 `deleted_at` 清空；M4 没有这条路径（[M9 的移交](../../M9-mcp/handoffs/M4-P2-unit-merge.md)末句，P2 审查 D-c）。恢复若与删除在同一变更集里，要重新看。
4. **前端的 `removedTo`**：`PageTreeStore` 记下本代删除的页，好让它们的外壳去父页；这张表在一代之内从不清除（`web/apps/web/src/stores/page-tree.store.ts`）。恢复一页之后它要从表里去掉，否则树里看不到它（M4 收尾审查 B-Q2）。

## 处理进展

- M8/P1（2026-10-13，合并 `12fc5d7`）：第 2 项：回收站里的页的版本、正文与阅读视图照活着的页读，权限相同（[M8 总设计](../00-M8-design.md) 4.3；`bootstrap/page_versions_test.go` 的 `TestAPageInTheTrashKeepsItsHistory`）。
- M8/P2（2026-10-14，合并 `25d250a`、`5116194`）：第 2 项的恢复：连同同一时刻删除的正文、版本与各变更集里的条目清空 `deleted_at`（4.6；`page/adapter/postgres/queries/nodes.sql` 的 `RestoreNodes`）。第 4 项：一次开始于删除答复之后的树的读把 `removed` 里的页列为活着时，从 `removed` 去掉它，同步与后台的恢复、撤销、别的标签页与别人的恢复都覆盖（4.13；`page-tree.store.ts` 的 `read`，`page-tree.store.test.ts`，e2e TB1 的页面版本）。
- M8/P3（2026-10-14，合并 `314d9ec`）：第 1 项改判：撤销按状态比较，次序值也比（`domain/change.go` 的 `TreeState.Same`；`domain/revert_test.go` 的 `TestPlanRevertConflicts`），兄弟被重新编号时显示为冲突 `changed`，罕见，写明（4.8、第 11 节第 5 条）。第 3 项：M8 的恢复与撤销各自一个变更集（类型 `trash_restore`、`revert`），没有单元在同一变更集里删除再恢复同一节点；留给 M9 的 `batch`。
- M8 收尾（2026-10-16）：核对过各项的落实（[M8 收尾审查](../reviews/M8-closeout-review.md) C-I1），关闭。第 1 项的精确做法（条目记"排在哪个兄弟之后"）写进 [M12 的移交](../../M12-release/handoffs/M8-history.md)第 1 项。第 3 项转给 M9：M8/P3 之后，同一变更集里删除再恢复同一页时合并的条目撞上 00038 的检查、单元答 500，写进 [M4/P2 一个单元里先建后删](../../M9-mcp/handoffs/M4-P2-unit-merge.md)的处理进展（C-M11）。
