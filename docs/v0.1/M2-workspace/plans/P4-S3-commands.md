# M2/P4/S3 管理命令：实施计划

上级：[P4 文档](../04-P4-deactivation-commands-purge.md) 3.3。

## 任务

1. 用例：`CreateWorkspaceFor`（字段检查 → 事务：`ShareActiveAccountByEmail` → 插入工作区与管理员）；`ReactivateMember`（`ShareActiveAccountByEmail` → 锁工作区 → `FindMembership` → 恢复并发布恢复事件，或说明已是成员）。与 `CreateWorkspace`、`AcceptInvitation` 共用插入与恢复的那一段。
2. 模块根 `admin.go`：`AdminDeps{Pool, Tx, Clock, Logger, Accounts, MembershipRestoreSubscribers}`、`NewAdmin`、`Admin.CreateWorkspace`、`Admin.ReactivateMember`。
3. 组合根：`Users` 与 `Workspaces` 共用的一段（日志、连接池、等数据库、一行输出与错误）；`Workspaces`、`CreateWorkspace`、`ReactivateMember`；`cliFieldName` 加 `slug`、`name`。
4. `cmd/nervewiki/workspaces.go`：`workspaces create --slug --name --admin`、`workspaces reactivate-member --workspace --email`，参数都必填。
5. 组合检查：起点加 `Workspaces`，到达 `workspace.NewAdmin` 与 `workspaceRegistrants`。

## 测试

- 用例（替身）：语句顺序（账户行在先）；账户不存在、已停用时没有写；恢复沿用角色、发布恢复事件；已是成员时不发事件；日志带 `by=cli`、不记邮箱。
- 组合根（真实数据库）：命令的输出行；失败时的一行原因与数据库不变；`creation_enabled = false` 时照样创建；恢复的订阅者失败时整体回滚。
- `cmd/nervewiki`：参数缺失时失败并说明；子命令出现在帮助里。
- 反向对照：`CreateWorkspaceFor` 不先锁账户行，替身测试的顺序断言失败；恢复不发事件，恢复的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
