# M2/P1/S5 权限矩阵：实施计划

上级：[P1 文档](../01-P1-access-workspaces.md) 3.10。

## 任务

1. `pgtest.NewDatabaseFrom(t, url)`：从一个准备好的库复制出新库（`TEMPLATE`）。复制之前，连向它的连接都要关闭。
2. `bootstrap/permission_matrix_test.go`：
   - 列、格、行的类型；
   - 准备数据：账户经接口；工作区与成员经仓储；结束与删除暂由 SQL 代替；
   - 只读的格共用一份库，写的格各一份副本；
   - 同时运行的应用数有上限，以免耗尽连接。
3. `permission_matrix_coverage_test.go`：
   - 每个操作要有行，除了豁免的模块与公开的操作；
   - 路径参数指向本列的工作区，或列为"不是目标"并写明理由；
   - 写方法的行必须标 `write`；
   - 豁免项本身必须存在。
   - 每种缺口各有一个反例测试。
4. `permission_matrix_seeded_test.go`：准备好的 id 表，供行组请求与核对回答。
5. `permission_matrix_workspace_test.go`：本 Phase 四个操作的行（3.10）。

## 测试

- 矩阵本身，以及覆盖检查的每个反例。
- 反向对照：
  - 规则表的 `workspace.read` 去掉访客，访客一列失败；
  - 删掉一行，覆盖检查失败；
  - 读成员关系不看 `ended_at`，"已结束"一列失败。

## 完成检查

`make check` 为绿；矩阵在本机的耗时写进结果一节。
