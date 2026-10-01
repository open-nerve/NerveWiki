# M2/P6/S4 停用对话框：实施计划

上级：[P6 文档](../06-P6-web-members-invitations.md) 3.5。

## 任务

1. `ConfirmDialog` 的可选 `texts`（`ProblemTexts`：problem 码 → 文案键；同一机制在 `errorText`、`formErrors`、`useForm`、`CredentialsForm`，P6 文档 3.5）。
2. `DeactivateDialog` 把 `workspace.sole_admin` 换成停用的说法。
3. 文案。

## 测试

- 安全页：停用答 409 `workspace.sole_admin` 时，对话框保持打开，显示停用的说法，会话仍在。
- 反向对照：不换说法时测试失败。

## 完成检查

`make check` 为绿。
