# M3/P2/S2 笔记本成员的接口与用例：实施计划

上级：[P2 文档](../02-P2-notebook-members.md) 3.2–3.8。

## 任务

1. 契约：`api/modules/notebook.yaml` 的五个操作，`NotebookMember`、`NotebookMemberList`、`NotebookMemberCreate{user_id, role}`、`NotebookMemberUpdate{role}`；`openapi.yaml` 登记路径；`make gen`。
2. access：规则表的五行（3.3）；notebook 的五个操作名。
3. `notebook/domain`：`CheckRole`、`CheckAddition`（实际一次列出添加的全部问题）、`CheckLeave`（规则一：唯一的有效管理员）；`ErrMemberNotFound`、`ErrOwnMembership`、`ErrSoleAdmin`。
4. 仓储：`queries/members.sql` 的新查询（3.7）、`adapter/postgres/members.go`。
5. `notebook/app`：
   - `ports.go`：`WorkspaceMembers`、`MemberProfiles`（实际的名字，与 workspace 的相同）、成员的仓储端口 `MemberFinder`、`MemberWriter`；
   - `extension.go`：`VisibilityChange`、`VisibilitySubscriber`、分发；`WorkspaceMemberEvents`（转发规则：加入的是管理员或成员；角色跨过访客）；
   - `manage.go`：`lock` 带上调用方的 404；
   - 五个用例与 `members.go`（成员与资料、邮箱按工作区角色）；建笔记本、改开放程度跨过 `none` 时发可见性事件。
6. HTTP 适配器的五个处理器；模块根：`Deps` 加 `WorkspaceMembers`、`Profiles`、`VisibilitySubscribers`，`NewWorkspaceMemberEvents`。
7. 组合根：`notebookDeps` 的两个端口（资料经 `notebook_profiles.go` 转换）；`notebookExtensions.visibilitySubscribers`（空）；`workspaceRegistrants` 把工作区的两个事件交给笔记本的转发（值逐字段转换）。
8. 文案：三个码的中英文。
9. 矩阵：成员行的 id 与本 Phase 的行（3.8）。

## 测试

- 领域：角色的取值、规则一的表格。
- 用例（替身）：锁、判定、校验、写入、可见性的次序；添加的三种情形；改、移出自己 409；离开时只靠默认角色 404、唯一的管理员 409；邮箱给谁。
- 仓储：列表的顺序与过滤、恢复保留 `created_at`、计数只数有效的管理员、`FindMemberOf`（实际的名字）含已结束的。
- 模块根（真实数据库）：每个可见性触发点在事务内、值、失败回滚；转发的表格；两个注册者的分发。
- 契约：五个操作的每个码；组合根：转发接上了线。
- 反向对照：添加不核对工作区成员关系；恢复时改写 `created_at`；访客加入也转发；可见性在提交之后调用；离开不数管理员；改角色不拒绝自己。

## 完成检查

`make check`、`make gen-check` 为绿。
