# M1/P1/S1 数据：实施计划

上级：[P1 文档](../01-P1-identity-foundation.md) 3.2、3.3。

## 任务

1. 迁移 `00002_identity_users.sql`、`00003_identity_auth_sessions.sql`（P1 文档 3.2 的列与约束；时间列不设默认值）。
2. sqlc 进 `server/tools/go.mod`；`server/sqlc.yaml` 为 identity 一条；`make gen-go` 执行 sqlc，`GEN_GO_OUT` 加上 `adapter/postgres/gen`。
3. identity 的 `adapter/postgres`：`queries/users.sql`（插入账户、按 id 读账户）、`queries/sessions.sql`（插入会话、认证时连同账户读会话）；`store.go`（一个 `Store`，有事务时走事务，唯一冲突的识别）；针对真实 PostgreSQL 的测试。
4. 架构测试 `sqlc_test`、`rawsql_test` 与各自的用例文件，从 Nerve 拷贝、改名。
5. `server/migrations` 的数据库测试：约束与索引名的清单、每条 CHECK 的反例。

## 测试

- `make gen-check-go` 核对 sqlc 的生成物。
- 反向对照：查询读别的模块的表 → 生成失败；模块里直接执行 SQL → `rawsql_test` 失败；去掉一条 CHECK → 数据库测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
