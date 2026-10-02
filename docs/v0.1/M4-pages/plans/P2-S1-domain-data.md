# M4/P2/S1 领域与仓储：实施计划

上级：[P2 文档](../02-P2-tree-operations.md) 3.2、3.3。

## 任务

0. 拆分 `app/unit.go`（P2 文档 3.1，P1 审查 Q1），单独一个提交，不改行为：`unit.go` 留 `Writer`、`Run`、`Unit`、`apply` 与变更集、条目、版本、事件、`appender`；`unit_place.go`（`Position` 与 `lineOf`、`slotOf`、`titleFree`）、`unit_create.go`、`unit_rename.go` 接走其余。page 的测试原样通过。
1. `domain`：`OpMove`、`OpDelete`；`ErrCycle`（`page.cycle`，409）；`Height`；移动的位置：去掉自己之后的兄弟上的 `slotOf`，以及"原地不动"的判断（父页不变且取到的位置与原来的相同）。
2. 仓储：`Subtree`（递归，相对深度，先序，不含已删的，深度上界 64）；`MoveNode`（父页、次序、审计列，23505 → `page.title_taken`）；`DeleteNodes`（CTE：节点与未删的正文、版本、条目，同一时刻，只改未删的行）；`RecordItem` 对后状态为空的条目写 `deleted_at`（插入与合并更新都写）。
3. 端口：`Nodes.Subtree`、`NodeWriter.MoveNode`、`NodeWriter.DeleteNodes`；用例的假实现跟上。

## 测试

- 领域的表格：移到自己、子页、孙页之下是环，移到兄弟、叔伯之下不是；高度（单页 1、三层 3）；层级恰好 10 通过、11 不通过；位置（最前、某页之后、最后；`after_id` 是自己）；原地不动的四种（同父页同位置、同父页换位置、换父页、移到根下）。
- 仓储：`Subtree` 的顺序与深度、跳过已删的子页；`MoveNode` 撞名答 `page.title_taken`；`DeleteNodes` 只改未删的行、时刻一致、`updated_by_id` 写上；删除的条目带 `deleted_at`，先改名后删除的条目经合并也带；经 `DeleteNodes` 与删除的条目删掉的子树，过了保留期由清理器一次清完。
- 反向对照：防环只看新父页本身；高度恒为 1；`RecordItem` 不给删除的条目写 `deleted_at`（清理的测试失败）；`DeleteNodes` 也改已删的行。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
