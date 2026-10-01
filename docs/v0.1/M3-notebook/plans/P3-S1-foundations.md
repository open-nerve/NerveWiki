# M3/P3/S1 游标、审计表与领域：实施计划

上级：[P3 文档](../03-P3-cascade-ownerless.md) 3.2、3.4、3.5。

## 任务

1. `shared/cursor.go`：`EncodeCursor`、`DecodeCursor`、`InvalidCursor()`（400 `bad_request`，字段 `cursor`）；封套 `{"v":1,"p":…}`，无填充的 base64url，解码后重新编码必须与输入相同；`PageSize`：缺省 50，1–100，越界 422 `limit`。
2. 迁移 `00011_notebook_notebook_audit_events.sql`：表、CHECK、两个索引（第 3.4 节）；清理器 `PurgeAuditEvents`；工作区删除的注册者同一时刻软删除这个工作区的审计记录（笔记本都已不在时也删）。
3. workspace：模块根导出 `EndRemoved`、`EndLeft`、`EndDeactivated`；`NewWorkspaces` 的端口加 `Slugs(ctx, ids)`，在调用方的事务里读未删除的工作区。
4. notebook 的领域：`Holding`（他在一本笔记本里的角色、有效管理员数、有效显式成员数）、`SoleAdmin()`；规则二 `RuleTwo(holdings)`（阻止的笔记本按工作区计数）；`ErrSoleAdminOf(bySlug)` 的原因文字（按 slug 排序，只给数量）；审计的三种动作、`AuditEvent` 与游标载荷 `AuditCursor`（`[created_at, id]`）。

## 测试

- 游标：往返；别的拼法（有填充、标准 base64、多余的字段、版本不是 1、载荷不是对象、空串）一律 `bad_request`；`limit` 的边界 0、1、100、101、缺省。
- 迁移：约束与索引名（`migrations/schema_test.go`）。清理：照笔记本的三个清理测试（截止时刻、分批、跳过被持有的行）。工作区删除：审计记录以删除的时刻、删除者软删除，别的工作区的不动，订阅者失败时回滚；笔记本都已清理的工作区也删审计。
- `Slugs`：已删除的工作区不在结果里。
- 规则二的表格：唯一管理员且有别的成员 → 阻止；两位管理员 → 不阻止；只有他一人 → 不阻止；别的成员只是已结束的 → 不阻止；按工作区计数。
- 反向对照：解码不做重新编码的比较 → 别的拼法的用例失败；`limit` 的上限放宽 → 边界失败；规则二不看别的成员、不看管理员数 → 表格失败；游标的时刻不转 UTC → 带偏移的拼法被接受；没有笔记本时不删审计 → 模块根测试失败；审计的清理不跳过被持有的行 → 清理测试超时。

## 完成检查

`make check`、`make gen-check` 为绿。
