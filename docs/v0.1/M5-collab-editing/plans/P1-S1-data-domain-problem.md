# M5/P1/S1 数据、领域与 problem 的成员：实施计划

上级：[P1 文档](../01-P1-edit-lock.md) 3.2–3.4、3.6。

## 任务

1. 迁移 `00018_page_edit_session_ends.sql`：三列与 `edit_sessions_ended_check`；`schema_test.go` 记下新的检查；`sqlc.yaml` 的 page 一条加上它。
2. 查询（`queries/sessions.sql`）：`AliveSessionsOf`、`DeleteExpiredSessionsOf`、`EndAliveSessions`、`FindEndedSession`；`FindLiveSession`、`HeartbeatSession` 加 `ended_reason IS NULL`；`EndSession` 也删本人的墓碑；各条返回新的三列。`make gen-go`；仓储（`adapter/postgres`）的新方法与转换。
3. 领域：`EditSessionLease` 120 秒；`EndedTakenOver`、`EndedUnlocked`；`ErrLocked`、`ErrEditSessionTakenOver`、`ErrEditSessionUnlocked` 与构造 `Locked(pageID, userID, name)`、`Unlocked(userID, name)`；动作 `ActionReleaseEditLock` 与规则表一行。
4. `app.EditSession` 的三个新字段与 `Alive`；端口 `Sessions`、`SessionWriter` 的新方法；`Names` 端口（`DisplayNames(ctx, ids) (map[uuid.UUID]string, error)`）。
5. `shared.Error` 加 `Lock *LockMember`、`EndedBy *PersonMember` 与方法 `ProblemLock()`、`ProblemEndedBy()`（返回字符串与是否有）；平台 `Problem` 加 `Lock`、`EndedBy`，`APIErrors.Write` 经可选接口填上；`api/common.yaml` 的两个成员；平台的契约测试加两行。

## 测试

- 迁移：检查拒绝三列不同在、`ended_at` 早于 `created_at`、未知的原因。
- 仓储：墓碑不被 `HeartbeatSession` 续活；`EndAliveSessions` 只动活着的（接管时只动本人的），`expires_at` 取较晚的；`DeleteExpiredSessionsOf` 只动这一页过期的；`EndSession` 删墓碑。
- 领域：`Alive` 的表格；两个构造带成员。
- 平台：带 `lock`、`ended_by` 的 problem 符合契约；没有时不出现。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
