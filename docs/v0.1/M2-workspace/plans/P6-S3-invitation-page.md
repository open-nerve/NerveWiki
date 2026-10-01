# M2/P6/S3 公开的邀请页：实施计划

上级：[P6 文档](../06-P6-web-members-invitations.md) 3.2、3.4。

## 任务

1. `InvitationPreviewService`（公开客户端）与 `InvitationPreviewStore`；`RootStore.invitationPreviews`（每代都有）。
2. `WorkspaceService.accept`、`WorkspaceStore.accept`（放进列表，按 id 去重；列表未读时留给读取）。
3. `AuthService.register`、`AuthStore.signUp` 可带邀请。
4. `pages/invitation.tsx`：链接不完整、预览（加载、404）、未登录（登录与注册两个表单）、已登录（接受、退出）、邮箱不符；接受之后 `replace` 到 `/:slug`。
5. 路由 `/invitations/:id`：布局之下、守卫之外；`invitations` 从 `[reserved]` 移到 `[app]`（并改名单文件头的说明）。
6. 文案。

## 测试

- `WorkspaceStore.accept`：加入、不重复、未读时留给读取。
- 邀请页：见 P6 文档 3.6 的邀请页一行；注册的请求体带邀请；页内登录之后页面还在同一地址。
- 保留名单测试照旧通过；Go 的 `reserved_test` 与 workspace 模块的测试照旧通过（名单文件改过）。
- 反向对照：注册不带邀请、邀请页挂在 `SignedIn` 之下、接受之后不放进列表、`[app]` 漏掉 `invitations`，各自的测试失败。

## 完成检查

`make check` 为绿。
