# M1/P6/S2 安全：改密码与停用：实施计划

上级：[P6 文档](../06-P6-settings.md) 3.4、3.5。

## 任务

1. `components/ui/dialog.tsx`、`alert-dialog.tsx`：按 shadcn 的写法包 radix-ui 的 `Dialog`、`AlertDialog`。
2. `services/account.service.ts`：`changePassword(current, next)`（204）、`deactivate()`（204）；`stores/account.store.ts` 对应的方法。
3. `stores/auth.store.ts`：`endSession()`（令牌管理器的 `endSession(loginId)`）。
4. `app/problem-messages.ts`：`formErrors` 的 `onField`；文案 `identity.current_password_incorrect`、`field.new_password.*`；契约核对加上 `changePassword`、`deactivateMe`。
5. `pages/settings/security-page.tsx`：改密码的表单（本地检查、字段错误、成功清空与提示）；停用的区块。
6. `pages/settings/deactivate-dialog.tsx`：确认、发送中禁用、失败时保持、成功之后 `endSession`。

## 测试

- `formErrors` 的 `onField`：码到字段、上方不重复。
- 改密码：本地检查（新密码长度）；`current_password_incorrect` 在当前密码下方并得到焦点；新密码的 422 在字段下方；429 的秒数在上方；成功之后两个字段清空、提示出现。
- 停用：确认之后状态 `signed-out`、记录被删、到 `/sign-in?next=%2Fsettings%2Fsecurity`，没有 `logout` 请求；503 时对话框保持、错误在对话框里、会话保留；双击只发一次；取消不发请求。
- 反向对照：停用成功之后不结束本地会话 → 测试失败；`current_password_incorrect` 不放到字段下方 → 测试失败。

## 完成检查

`make check` 为绿。
