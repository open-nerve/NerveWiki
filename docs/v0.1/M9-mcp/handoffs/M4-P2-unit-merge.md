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
