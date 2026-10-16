```yaml
status: open
from: M4/P2
to: M9
created: 2026-10-02
```

# 一个单元里先建后删的节点

M4 的写入单元把同一个节点的多次改动合并成一条：最早的"前"、最新的"后"（[M4 总设计](../../M4-pages/00-M4-design.md)第 4 节"变更集"；`domain.Change.Then`），变更集的条目也按（变更集、节点）合并（`RecordItem` 的 `ON CONFLICT`）。M4 的用例都只做一种操作，参与者只追加改名与正文写，所以没有一个单元会先建一个节点、再把它删掉（[P2 文档](../../M4-pages/02-P2-tree-operations.md) 3.2；`app/unit.go` 的 `recordItem` 注释）。

M9 的 batch（一个单元多个操作）是第一个能这样做的地方，M7 的导入同样可能。到那时，合并的结果前后都为空：

1. **条目**：`changeset_items_state_check` 要求前后至少有一个状态，合并之后的那一行被拒绝，整个单元答 500。
2. **事件与守卫的值**：改动集里出现一个前后都空、`Revision` 可能不为零的 `Change`，观察者（M5 的推送、M6 的索引）与守卫都要认得它。M5 的推送（`bootstrap/events_registrants.go` 的 `pageEvents`）与锁的守卫（`page.NewEditLock`）都没有为它做特别处理：从改动集里去掉这个节点之后，二者自然不见它；不去掉时，`pageEvents` 会把它发成 `pages` 事件里的一页（`Tree` 取 `Change.Moves()`，`Revision` 原样）（[M5 收尾审查](../../M5-collab-editing/reviews/M5-closeout-review.md) C-Q1）。

M9 引入多操作的单元时一并定下（[P2 审查](../../M4-pages/reviews/P2-tree-operations-review.md) D-h）：合并到前后都空时删掉这一行条目（或不插入），并从改动集里去掉这个节点；连同 `Change.Then` 的语义写进 M9 的设计，并加一个两操作单元（建后删）的测试。另：一个有"后"的改动合并进已在回收站的条目时，`RecordItem` 会把 `deleted_at` 清空，M8 的恢复要看这一点，写在 [M8 的移交](../../M8-history-search/handoffs/M4-history.md)第 3 项。

## 处理进展

- M7 开工（2026-10-08）：M7 的导入只建不删，不会遇到；M7 的那份以"不适用"关闭，仍由 M9 定下（[M7 总设计](../../M7-assets-transfer/00-M7-design.md) 4.2）。
- M8 收尾（2026-10-16）：不适用，仍由 M9 定下。M8 的单元不先建后删，也不在同一变更集里删除再恢复同一节点（恢复、撤销各自一个变更集）。M8/P3 起删除的条目带页的版本号，迁移 `00038` 加了 `changeset_items_revision_state_check`（`revision` 只在"前"或"后"为空时可以有值）；条目的合并（`page/adapter/postgres/queries/changesets.sql` 的 `RecordItems`，`ON CONFLICT`）留第一次的"前"，取最后的"后"与 `deleted_at`，`revision` 除合并成删除时取删除的、否则留第一次的，`restored` 不改。所以 `batch` 在一个变更集里删除再恢复同一页时，合并的条目前后都在、带着删除时的版本号，被这条检查拒绝，单元答 500；`Change.Then` 留第一项的 `Restored`（空）。恢复再删除同一节点时前后都空，同上面的建后删。上面"M8 的恢复要看这一点"转给 M9：引入 `batch` 时连同建后删一起定下这几种合并（删除再恢复：前后都在时不带 `revision`，`restored` 照合并之后是不是恢复；恢复再删除：同建后删），在真的数据库上加两个操作的单元的测试（[M8/P3 的移交](M8-P3-revert.md)；[M8 收尾审查](../../M8-history-search/reviews/M8-closeout-review.md) C-M11）。`RecordItem`、`recordItem` 现名 `RecordItems`、`recordItems`。
