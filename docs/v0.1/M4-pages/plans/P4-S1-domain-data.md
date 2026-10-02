# M4/P4/S1 领域与数据：实施计划

上级：[P4 文档](../04-P4-content-sessions.md) 3.2、3.3。

## 任务

1. 迁移 `00017_page_edit_sessions.sql`（P4 文档 3.2 的列、CHECK 与三个索引）；`sqlc.yaml` 的 page 一条加上它；`schema_test` 的名单加约束与索引名，CHECK 的反例（`changeset_id` 与 `revision` 一个为空、客户端不合法）。
2. 领域：`content.go`（`MaxContentBytes`、`CheckContent`）、`session.go`（`EditSessionLease`、`EditSessionHeartbeat`、`EndReason`）、`errors.go` 三个码、`actions.go` 的 `page.write`、`page.edit`（`Actions()` 随之；`access/domain/rules.go` 两行 `writers()`）、`change.go` 的 `OpContent` 与 `Change.Parsed`（`Then` 随 `Revision` 取后一个的）。
3. 仓储：`contents.sql` 的 `LockContent`、`WriteContent`，`PageContent` 带 `content_hash`；`changesets.sql` 的 `TouchChangeset`；`sessions.sql`：`CreateSession`、`LockSession`（`FOR UPDATE`）、`SetSessionWrite`、`FindLiveSession`、`HeartbeatSession`、`EndSession`、`DeleteNodeSessions`、`DeleteNotebookSessions`（`:many`，返回删掉的行）、`DeleteExpiredSessions`（`SKIP LOCKED`）；`activity.sql` 的两条；`store.go` 与新文件 `sessions.go`、`activity.go` 实现端口（端口的类型在 `app/ports.go` 一并加上）。

## 测试

- 领域：`CheckContent` 的表格（空、恰好上限、多一个字节、NUL、非法 UTF-8）；常量；`Then` 带解析结果。
- 仓储（真实 PostgreSQL）：P4 文档第 5 节"仓储"一行；`LockContent` 被另一个事务持有时等待（`WaitForLockWaitsOn`）。
- 反向对照：`LockContent` 去掉 `FOR NO KEY UPDATE`；心跳与结束去掉 `expires_at > now` 或 `user_id`；`DeleteExpiredSessions` 去掉 `SKIP LOCKED`；活动去掉 `deleted_at IS NULL`。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
