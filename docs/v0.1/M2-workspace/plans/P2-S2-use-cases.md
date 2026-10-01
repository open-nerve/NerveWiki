# M2/P2/S2 用例与扩展点：实施计划

上级：[P2 文档](../02-P2-workspace-members.md) 3.2–3.6。

## 任务

1. `app/extension.go`：`MembershipEnd`、`EndCause`（`removed`、`left`、`deactivated`）、`MembershipEndVetoer`、`MembershipEndSubscriber`、`WorkspaceDeletion`、`WorkspaceDeletionSubscriber`。
2. `app/end_membership.go`：`membershipEnder{members MemberUpdater, vetoers, subscribers}`，`end(ctx, e)`：否决者 → `EndMemberships` → 订阅者。调用方已锁住工作区行。
3. 六个用例，各自只依赖要用的端口，顺序照 3.2：
   - `UpdateWorkspace`、`DeleteWorkspace`（删除事件的订阅者）、`ListMembers`；
   - `UpdateMember`、`RemoveMember`、`LeaveWorkspace`（后两个经 `membershipEnder`）。
   - 按 slug 寻址的先以 `domain.ValidSlug` 判断；`ErrNotVisible` 译成各自的 404 码。
   - 写成功之后记日志：`workspace_id`、`user_id`，移出另记 `member_id`；不记名称与邮箱。

## 测试

单元测试用替身（`app/fakes_test.go` 扩充：锁、成员、资料的替身记录调用与是否在事务里）：

- 每个用例的顺序：锁在判定之前、判定在检查之前、检查在写之前；不合格式的 slug 不查库。
- 404 的各种来源：工作区不存在或已删除（锁读到 0 行）、成员行不存在、已结束、调用者不可见。
- 403：成员与访客改名、删除、改角色、移出。
- 422：名称、角色的取值，都在判定之后。
- 409：改自己、移出自己、唯一管理员离开；两位管理员时可以离开。
- `membershipEnder`：否决者的错误原样返回、`EndMemberships` 没被调用；订阅者的错误原样返回；顺序是否决者、写、订阅者。
- 删除：成员行先于工作区，同一个时刻；订阅者收到同一个时刻与删除者。
- 成员列表：访客得到的邮箱全是空；资料缺了某个账户是错误。
- 反向对照：`LeaveWorkspace` 不数管理员，唯一管理员的测试失败；`ListMembers` 不按调用者的角色置空邮箱，访客的测试失败。

## 完成检查

`make check` 为绿。
