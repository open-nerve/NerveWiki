```yaml
status: done
from: M2/P4
to: M4
created: 2026-10-01
```

# 页面树的清理顺序

> 已全部处理（2026-10-03）：第 1 项选了循环删叶：`PurgeNodes` 一次调用里只删已没有子页的页面，直到一批删不满（[M4 总设计](../00-M4-design.md)第 7 节）；第 2 项是 `TestPurgeSkipsAHeldNodeAndKeepsItsAncestors` 与 `TestPurgeKeepsWhatAHeldRowNeeds`（`page/adapter/postgres/purge_test.go`），一棵三层的软删子树一次运行清完由 `TestThePurgeDeletesWhatOutlivedTheRetention` 守着；第 3 项照办；第 4 项给页面的五张表各加了 `*_deleted_at_idx` 部分索引。经 [M4 收尾审查](../reviews/M4-closeout-review.md)对照代码核实；自引用外键的测试写法补进总体设计 13.1 第 6 条。

M2/P4 建了软删除的清理注册表（总体设计 12.4、13.1 第 6 条；[M2/P4 文档](../../M2-workspace/04-P4-deactivation-commands-purge.md) 3.4）。它的数据库测试 `TestPurgersComeBeforeTheTablesTheyReference`（`server/internal/bootstrap/purge_test.go`）核对"引用被清理表的表先清理"，但排除了自引用的外键（`conrelid <> confrelid`）：M2 没有这样的表，而页面树的父指针就是一个（[P4 审查](../../M2-workspace/reviews/P4-deactivation-commands-purge-review.md) Q2）。

M4 加页面的清理器时：

1. **同一张表里先子后父**：一批里同时有父页与子页时，父页的删除不能经 `ON DELETE CASCADE` 连带删掉子页，也不能等子页的锁。照工作区的清理器（`NOT EXISTS` 已没有子行）办：只删已没有子页的页面，一棵软删的子树要几次运行才清完；或者按深度从叶到根分批。选哪种，连同一棵大子树要几次运行，在 M4 设计里写明。
2. **测试**：给自引用的外键补一条数据库测试（父页与子页都过了保留期、子页被别的事务持有时，父页留下、不等锁，放开之后下一次运行清完），不靠上面那条测试。
3. M3 的移交（[M3/handoffs/M2-workspace.md](../../M3-notebook/handoffs/M2-workspace.md)第 4 项）里清理器的其余约束同样适用。
4. **`deleted_at` 的索引**：M2、M3 的表都没有，每批顺序扫描（M3 总设计第 7 节第 4 项）。页面表大到一批扫不完时，加部分索引或调长清理的期限，在 M4 设计里定。
