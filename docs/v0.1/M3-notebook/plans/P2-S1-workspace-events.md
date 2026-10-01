# M3/P2/S1 工作区的两个事件：实施计划

上级：[P2 文档](../02-P2-notebook-members.md) 3.6。

## 任务

1. `workspace/app/extension.go`：`MembershipAddition{WorkspaceID, UserID, Role, By, At}` 与 `MembershipAdditionSubscriber.MembershipAdded`；`MemberRoleChange{WorkspaceID, UserID, From, To, By, At}` 与 `MemberRoleChangeSubscriber.MemberRoleChanged`。
2. `accept_invitation.go`：`AcceptInvitationDeps.Subscribers` 改名 `Restored`，加 `Added`；`joinAdded` 写入成员行之后调用 `Added`，同一事务、同一时刻。
3. `update_member.go`：`UpdateMemberDeps.Subscribers`；写入之后、角色确有变化时调用，值带原角色与新角色。
4. `module.go`：类型别名；`Deps.MembershipAdditionSubscribers`、`Deps.MemberRoleChangeSubscribers`；组合根的 `workspaceExtensions` 加两个空的列表（S2 接上笔记本的转发）。

## 测试

- 用例：调用的次序（写入之后、在事务内）、值；恢复与保持原样不调用 `Added`；同角色不调用 `MemberRoleChanged`；订阅者失败则用例失败、事务回滚。
- `app/registrants_test.go`：两个注册者的分发（次序、第一个错误停下）。
- 模块根（真实数据库，照恢复事件）：订阅者在事务内看得到新的成员行与新角色；订阅者失败时成员行、邀请、角色都不变。
- 反向对照：在提交之后调用 → 模块根测试失败；恢复时也调用 `Added` → 用例测试失败；同角色也调用 → 用例测试失败。

## 完成检查

`make check` 为绿；M2 的测试与交错全部保留。
