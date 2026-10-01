# M2/P3/S1 数据与令牌：实施计划

上级：[P3 文档](../03-P3-invitations.md) 3.1、3.2、3.3、3.8。

## 任务

1. 迁移 `00008_workspace_workspace_invitations.sql`：列、约束、索引照 3.1；`sqlc.yaml` 的 workspace 一条加上它；`runtime-grants.sql`；`schema_test` 列出约束与索引名，加 CHECK 的反例（大写或带空白的邮箱、第四种角色、`accepted_at` 与 `deleted_at` 不同）。
2. `shared.CheckEmail`：从 identity 领域的 `checkEmail` 移来，测试随之移动；identity 改用它。
3. identity：`signing.Keys.Derive(info)`；模块根 `DerivedKey(info) []byte`。
4. `workspace/adapter/mac`：`Tokens` 的 `Token`、`Valid`。
5. workspace 领域：`invitation.go`（`CheckInvitation(email, role)`：一次列出全部字段问题）；操作名 `workspace_invitation.list/create/delete`；码 `workspace.invitation_not_found`（404）、`workspace.invitation_email_mismatch`（403）。access 的规则表三行。
6. 仓储 `queries/invitations.sql`：插入（唯一索引翻译成 `email: duplicate`）、按工作区列出（新的在前）、按 id 读、`FOR UPDATE` 重读、软删除、接受（`accepted_at = deleted_at`）、按（工作区，邮箱）删除、按工作区删除；成员的查询加"按工作区与账户读成员行（含已结束）""恢复""按账户是否有效成员"；工作区 `FOR SHARE` 的锁。
7. `pgtest.WaitForKeyWaitOn` 与它的测试。

## 测试

- 令牌：同一 id、同一密钥得到同一令牌；换 id 或密钥不同；改一个字符不通过；前缀与长度。`Derive` 的不同 info 得到不同的键。
- 领域：邀请的规则表格。
- 仓储：照 P3 文档第 5 节的集成一行。
- 反向对照：唯一索引不带 `WHERE deleted_at IS NULL`，"删除之后可再邀请"失败；`Valid` 不比较，篡改的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
