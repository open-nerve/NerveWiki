# M2/P4/S1 数据：实施计划

上级：[P4 文档](../04-P4-deactivation-commands-purge.md) 3.1、3.3、3.4。

## 任务

1. identity：查询 `ShareAccountByEmail`（按规范化的邮箱 `FOR SHARE`，带回 id、是否可用）；端口 `AccountSharer.ShareAccountByEmail`；`ActiveAccounts.ShareActiveAccountByEmail(ctx, email) (uuid.UUID, error)`，与 `ShareActiveAccount` 同样答 `account_not_found`、`account_deactivated`；模块根的 `Accounts` 接口加这个方法。
2. workspace 领域：`Member.EndedAt *time.Time` 与 `Active()`；`Standing{WorkspaceID, Slug, Role, Admins, Members}` 与 `BlocksDeactivation()`（规则二）；`ErrSoleAdminOf(slugs)`：409 `workspace.sole_admin`，detail 列出 slug。
3. workspace 仓储：
   - `FindMembership` 带回 `ended_at`，不再另带 `active`；接受与创建邀请改用 `Member.Active()`。
   - `LockWorkspacesOf(ctx, userID)`：他有效成员关系所在、未删除的工作区，按 `id` 升序 `FOR NO KEY UPDATE`，一条语句。
   - `ListStandings(ctx, userID, workspaceIDs)`：锁下读他在这些工作区的有效成员关系，带各工作区有效的管理员与成员人数。
   - 清理：`PurgeInvitations`、`PurgeMembers`、`PurgeWorkspaces`，各是 `DELETE … WHERE id IN (SELECT … WHERE deleted_at < $1 LIMIT $2 FOR UPDATE SKIP LOCKED)`，返回删了几行。

## 测试

- identity：`ShareAccountByEmail` 的锁模式（与停用的 `FOR NO KEY UPDATE` 冲突，与另一个 `FOR SHARE` 不冲突）；等锁期间邮箱被改走时答没有这个账户；大小写不同的邮箱经规范化找到。
- 领域：规则二的表格（管理员与否、管理员人数、成员人数）。
- 仓储：`LockWorkspacesOf` 不含已结束、已删除的成员关系与已删除的工作区，按 `id` 升序，持有的是 `FOR NO KEY UPDATE`；`ListStandings` 的人数不计已结束的成员；`FindMembership` 带回结束的时刻；三个清理：保留期的边界（早于才删）、批的上限、跳过别的事务持有的行。
- 反向对照：清理的条件去掉 `deleted_at <`，边界测试失败；`ShareAccountByEmail` 去掉 `FOR SHARE`，锁测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
