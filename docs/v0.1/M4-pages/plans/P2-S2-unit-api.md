# M4/P2/S2 单元的操作与接口：实施计划

上级：[P2 文档](../02-P2-tree-operations.md) 3.4–3.6。

## 任务

1. `app/unit.go`：`Unit.Move`、`Unit.Delete`（P2 文档 3.4 的顺序）；`app/move_node.go`、`delete_node.go`（照 `rename_node.go`：先不加锁读节点，单元里在锁下重读；日志 `node moved`、`node deleted`）。
2. `domain/actions.go`：`node.move`、`node.delete`；`access/domain/rules.go` 两行（`writers()`）。
3. 契约：`page.yaml` 加 `moveNode`、`deleteNode` 与 `NodeMove`；`make gen`；`adapter/http`：两个处理函数，`handler_test.go` 答出每个新码、`after_id` 的省略与 `null`。
4. 模块根接线；文案 `page.cycle`；矩阵两行（`permission_matrix_page_test.go`）。

## 测试

- 用例（假的端口）：码的次序（404 → 403 → 422 → 409 → 守卫）；移动的条目只记被移动的页，后代进事件与守卫的值、不进条目；原地不动不写、不调用观察者；同一父页内排序不查重名；重排；删除的事件与守卫的值含整棵子树；日志（id 在、标题不在、失败与原地不动时没有）。
- 模块根：经路由移动与删除，真实数据库上的变更集、条目与时刻。
- 反向对照：原地不动也写；后代不进事件；先调守卫再查环。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check`、前端的 lint、format、types 与 `make knip` 为绿。
