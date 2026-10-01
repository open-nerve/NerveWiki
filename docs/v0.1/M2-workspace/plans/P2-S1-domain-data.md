# M2/P2/S1 领域与数据：实施计划

上级：[P2 文档](../02-P2-workspace-members.md) 3.2、3.3、3.6、3.8。

## 任务

1. `workspace/domain`：
   - `actions.go`：`workspace.update`、`workspace.delete`、`workspace.leave`、`workspace_member.list`、`workspace_member.update`、`workspace_member.remove`，加进 `Actions()`；
   - `member.go`：`CheckRole(s)`，三种角色之外是字段 `role` 的 `invalid_format`；
   - `errors.go`：`workspace.member_not_found`（404）、`workspace.own_membership`（409）、`workspace.sole_admin`（409）。
2. `access/domain/rules.go`：六行，照 3.8。`bootstrap/actions_test.go` 原样通过（并集等于键）。
3. `workspace/adapter/postgres/queries/workspaces.sql` 新查询：
   - `LockWorkspaceBySlug`、`LockWorkspaceByID`：`FOR NO KEY UPDATE`，带 `deleted_at IS NULL`；
   - `RenameWorkspace`、`DeleteWorkspace`（`deleted_at`、`updated_by_id`、`updated_at`）；
   - `FindMember`（按 id，未删除，含 `ended_at`）、`ListActiveMembers`（按 `created_at`、`id`）、`CountActiveAdmins`；
   - `UpdateMemberRole`、`EndMemberships`（工作区 id 的数组 × 账户，只改有效的行）、`DeleteMembersOf`（工作区的全部未删除行）。
4. `workspace/app/ports.go` 加窄端口：`WorkspaceLocker`、`WorkspaceUpdater`、`MemberFinder`、`MemberUpdater`、`MemberProfiles`（带本模块的 `Profile`）；仓储实现前四个。
5. identity：`queries` 加 `ProfilesByID`（`id = ANY`）；仓储方法；模块根 `profiles.go`：`Profile`、`Profiles` 接口、`NewProfiles(pool)`。

## 测试

- 领域：`CheckRole` 的表格（三种角色、空、大写、第四种）。
- access：判定的表格加新的六行（三种角色 × 规则）。
- workspace 仓储（真实数据库）：
  - 锁的两条语句：已删除的工作区读到 0 行；在别的事务持锁时等待（`WaitForLockWaitsOn`）；
  - 改名写 `updated_by_id`、`updated_at`；
  - 删除连带：`DeleteMembersOf` 与 `DeleteWorkspace` 用同一个时刻，已结束的行也软删除，别的工作区不动；
  - `CountActiveAdmins` 不数已结束、已删除、非管理员的行；
  - `EndMemberships` 只改有效的行，写 `ended_at`、`updated_by_id`、`updated_at`，返回改了几行；
  - `ListActiveMembers` 的顺序与过滤。
- identity 仓储：`ProfilesByID` 读多个、读不存在的 id 得到较少的结果、空数组。
- 反向对照：锁的语句去掉 `deleted_at IS NULL`，"已删除读到 0 行"失败；`CountActiveAdmins` 不看 `ended_at`，计数测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
