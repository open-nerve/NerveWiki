```yaml
status: done
from: M3/P1
to: M4
created: 2026-10-02
```

# 笔记本删除事件的第一个注册者

> 已全部处理（2026-10-03）：第 1 项是 `bootstrap/page_registrants_test.go` 的 `TestDeletingANotebookDeletesItsPages`（删笔记本、删工作区、删无主笔记本三个子测试），任一路径不交注册者时对应的子测试失败（收尾审查的反向对照）；第 2 项：订阅者在笔记本行之后写节点，M4 没有跨笔记本的写（移动不跨笔记本），"按 `id` 升序"留在总体设计 13.1 第 5 条；第 3 项：`nodes` 引用 `notebooks` 的外键是 `RESTRICT`，节点的清理器排在 notebook 的之前。经 [M4 收尾审查](../reviews/M4-closeout-review.md)核实。

M3/P1 建了笔记本删除事件（[M3 总设计](../../M3-notebook/00-M3-design.md)第 8 节；[P1 文档](../../M3-notebook/01-P1-notebooks-access.md) 3.8）：订阅者在笔记本行软删除之后、同一事务内调用，以事件的时刻作为自己的 `deleted_at`。M3 没有注册者，组合根的 `notebookRegistrants()` 返回空集合，交给每一条发布路径；把任何一处换成 nil，测试照样全部通过（[P1 审查](../../M3-notebook/reviews/P1-notebooks-access-review.md) Q3，与 [M2 移交](../../M3-notebook/handoffs/M2-workspace.md)第 1 项同一种情形）。

M4 注册第一个订阅者（软删除节点与正文）时：

1. **每条发布路径一个整个程序上的行为测试**：`deleteNotebook`（接口）；删除工作区（经 notebook 模块的工作区删除注册者，一次带全部笔记本的 id）；删除无主笔记本（M3/P3 的 `DELETE /ownerless-notebooks/{id}`）。每条都断言节点以笔记本的删除时刻软删除；任一路径没交注册者时，对应的测试失败。
2. **加锁**：订阅者在笔记本行（工作区删除时还有工作区行）已锁之后运行；节点的写入接在 `notebooks → notebook_members` 之后（总体设计 13.1 第 5 条）。工作区删除的注册者按 `id` 升序锁笔记本行（P1 审查 Q2），M4 里同时持有两本笔记本的写（例如跨笔记本移动页面）也按 `id` 升序取锁。
3. **清理**：节点表引用 `notebooks` 的外键用 `ON DELETE RESTRICT`（跨模块，`TestCrossModuleForeignKeysToPurgedTablesRestrict` 会检查），节点的清理器排在 notebook 的之前；自引用的父指针见[页面树的清理顺序](M2-P4-purge-page-tree.md)。
