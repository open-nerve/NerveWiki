# M2/P6/S1 成员与离开：实施计划

上级：[P6 文档](../06-P6-web-members-invitations.md) 3.2、3.3。

## 任务

1. `services/member.service.ts`：`list(slug)`、`update(id, role)`、`remove(id)`，转出 `WorkspaceMember`、`WorkspaceRole`。
2. `stores/member.store.ts`：`list`、`load`（丢弃被写答复越过的读）、`changeRole`、`remove`（404 `workspace.member_not_found` 也移出）；`RootStore.membersOf(workspace)` 按 id 缓存；`useMembers`。
3. `WorkspaceService.leave`、`WorkspaceStore.leave`（同 `remove`，404 `workspace.not_found` 也算离开）。
4. `components/ui/native-select.tsx`。
5. 设置导航加"成员"；路由 `/:slug/settings/members`。
6. `pages/workspace/members-page.tsx`、`member-row.tsx`：成员一节（列表、"你"、访客不显示邮箱列、管理员改别人的角色与移出、写的失败在列表上方、403 之后重新读取工作区）；离开一节。
7. 文案（两种语言）。

## 测试

- `MemberStore`：交错；改角色；移出与 404；`RootStore.membersOf` 同一工作区同一个、别的工作区与新一代是新的。
- `WorkspaceStore.leave`：成功与 404 都移出并记下；403、409 不移出。
- 成员页：见 P6 文档 3.6 的成员与离开两行。
- 反向对照：读覆盖写、自己的一行有控件、访客显示邮箱列、离开之后不记下 slug，各自的测试失败。

## 完成检查

`make check` 为绿。
