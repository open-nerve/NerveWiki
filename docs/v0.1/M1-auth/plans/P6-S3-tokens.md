# M1/P6/S3 访问令牌：实施计划

上级：[P6 文档](../06-P6-settings.md) 3.6。

## 任务

1. `services/api-token.service.ts`：`list()`、`create(body)`、`revoke(id)`；`stores/api-token.store.ts`：`tokens`、`load()`、`create()`、`revoke()`（读不覆盖之后答复的写；创建加在开头、不存秘密；撤销的 404 当作已撤销）；`RootStore` 一代另有 `apiTokens`。
2. `i18n/format.ts`：`formatDate`、`formatDateTime`（`Intl.DateTimeFormat`，界面语言）。
3. `pages/settings/tokens-page.tsx`：列表、空状态、加载失败与重试、"创建令牌"。
4. `pages/settings/token-row.tsx`：名称、日期、已过期的标记、撤销的确认。
5. `pages/settings/create-token-dialog.tsx`：表单（名称、有效期的预设、当前密码）；令牌视图（只读输入框、复制、警告、只有"完成"关闭）。
6. 文案；契约核对加上 `listApiTokens`、`createApiToken`、`revokeApiToken`；字段文案 `field.name.too_long`、`field.expires_at.out_of_range`。

## 测试

- store：读挂起时创建答复，读答复旧列表，列表仍有新令牌；创建的项不含 `token`；撤销的 404 从列表删除。
- `formatDateTime` 在 `en` 与 `zh-CN` 下的输出。
- 页面：日期与"从未使用""永不过期""已过期"；空状态；本地检查；错误的密码在密码下方；创建之后令牌在对话框里，完成之后页面上没有它；Esc 不关闭令牌视图；迟到的答复（取消之后才到）不显示令牌、列表有它；撤销之后列表没有它；撤销答 404 同样。
- 反向对照：令牌存进 store → 测试失败；`load` 不管之后的写 → 测试失败。

## 完成检查

`make check` 为绿。
