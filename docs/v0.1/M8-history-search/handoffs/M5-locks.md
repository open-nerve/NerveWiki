```yaml
status: done
from: M5
to: M8
created: 2026-10-03
```

# 编辑锁与还原、撤销

M5 的编辑锁（[M5 总设计](../../M5-collab-editing/00-M5-design.md)第 4.1、4.4 节）：一页有活着的编辑会话时，写入守卫拒绝不带这个会话的正文写（409 `page.locked`），也拒绝别的账户删除这一页或它的上级页；改名、移动照常。

M8 的还原一个版本、整组撤销一个变更集，都是写入单元里的写，经同一个守卫：

1. **整组撤销碰上被锁的页**：一个变更集可能改过多页，其中一页正被人编辑时，守卫的拒绝让整个单元回滚。M8 要决定：整组拒绝并说明哪一页被谁锁着（与 M5 的 `page.locked` 一致，最简单），还是跳过被锁的页、只撤销其余的（违反"整组"的语义，要先问负责人）。
2. **恢复回收站里的页**（M8 的恢复）：被删的页没有活着的会话（删除时本人的会话随之结束，别人持锁时删除被拒），恢复本身不会被锁挡住；恢复之后的正文写同样经守卫。
3. **测试**：在整个程序上加"撤销时其中一页被锁"的行为测试。

## 处理进展

- M8/P1（2026-10-13，合并 `12fc5d7`）：还原版本是正文的写，经守卫：这一页有活着的编辑会话时 409 `page.locked`，调用者自己在别处编辑同样被拒（[M8 总设计](../00-M8-design.md) 4.4；`bootstrap/page_versions_test.go` 的 `TestRestoringAVersionThroughServe`）。
- M8/P2（2026-10-14，合并 `25d250a`）：第 2 项：编辑锁只拦正文写与删除，`OpRestore` 放行，不改代码、加测试（`page/app/edit_lock_test.go` 的 `TestTheEditLockGuardsByOperation`；4.6、第 8 节）；`restoreNode` 的 `x-problem-codes` 里没有 `page.locked`。
- M8/P3（2026-10-14，合并 `314d9ec`）：第 1 项：有一页被人编辑就整组拒绝，409 `page.locked` 带第一页的持锁人，不跳过（4.8、第 11 节第 3 条）。第 3 项：`bootstrap/interleavings_history_test.go` 的 `TestRevertingWhileAPageOfItIsEdited`（别人持锁、本人在别处编辑、撤销新建时那一页被人编辑）。
- M8/P4（2026-10-15，合并 `2f8077ec`）：后台的撤销同样：开始之前 409 `page.locked`，之后的批碰上被锁的页以 `page_locked` 停下（`bootstrap/interleavings_background_stops_test.go` 的 `TestABackgroundRevertAndAnEditSession`）。
- M8 收尾（2026-10-16）：核对过各项的落实（[M8 收尾审查](../reviews/M8-closeout-review.md) C-I1），关闭。
