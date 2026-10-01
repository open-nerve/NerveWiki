# M2/P6/S2 邀请：实施计划

上级：[P6 文档](../06-P6-web-members-invitations.md) 3.2、3.3。

## 任务

1. `services/invitation.service.ts` 的 `InvitationService`：`list(slug)`、`create(slug, body)`、`remove(id)`，转出 `WorkspaceInvitation`。
2. `stores/invitation.store.ts` 的 `InvitationStore`：`list`、`load`、`invite`（放到最前，按 id 去重）、`withdraw`（404 `workspace.invitation_not_found` 也移出）；`RootStore.invitationsOf(workspace)`；`useInvitations`。
3. `app/invitation-link.ts`：`invitationLink(origin, link)`、`linkOf(id, hash)`。
4. `pages/workspace/invitations-section.tsx`：只对管理员挂载；表单（邮箱、角色，`useForm`，422 在邮箱下方；邮箱 `trim()`，全角空格浏览器不去，审查 Q8）；列表（复制、没有剪贴板时的链接框、撤回；"已复制"在撤回、复制失败时清掉，审查 T14）。
5. 文案：`field.email.not_allowed`、`field.email.duplicate` 与这一节的文字。

## 测试

- `invitationLink`、`linkOf` 的表格。
- `InvitationStore`：交错；邀请放到最前、不重复；撤回与 404。
- 邀请一节：见 P6 文档 3.6 的邀请一行；成员与访客不挂载、不读取；没有剪贴板（`navigator.clipboard` 为 `undefined`，审查 T7）。
- 反向对照：令牌放进查询参数、非管理员也去读，各自的测试失败。

## 完成检查

`make check` 为绿。
