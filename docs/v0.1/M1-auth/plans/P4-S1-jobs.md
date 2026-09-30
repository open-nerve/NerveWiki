# M1/P4/S1 River 与后台任务：实施计划

上级：[P4 文档](../04-P4-admin-jobs.md) 3.2–3.4、3.10、3.11。

## 任务

1. go.mod：`river`、`riverdriver/riverpgxv5` v0.47.0。
2. 迁移 `00005_river_main_v2_to_v7.sql`：锁定版本的 `river migrate-get` 导出 v2–v7，`StatementBegin/End` 包起来；`.gitattributes`、`.editorconfig` 保护它；`migrations/river_test.go` 与 `rivermigrate` 内嵌的 SQL 逐字比较。
3. `schema_test`：约束与索引名排除 `river_` 的表；up/down 的对象清单加上类型与函数（排除扩展成员）。
4. 架构测试：River 只由 `platform/jobs`、模块的 `adapter/river` 与 bootstrap 导入。
5. `platform/jobs`：`Job`、`New`、`Start`（同步，无重试；C1：启动的 ctx 不随调用方取消）、`Stop`（期限加 1 秒宽限；没启动过什么也不做）。
6. 配置 `jobs.shutdown_timeout`（默认 10 秒，必须为正），进 `LogValue` 与内置配置的测试。
7. bootstrap：`newApp` 构造 jobs（模块的 `Jobs()` 此时为空）；`run` 并发地服务 HTTP 与"等迁移再启动任务"（每 2 秒 `CheckUpToDate`，启动失败取消 serve）；HTTP 返回之后停任务，再 `close`。
8. `pgtest.WaitForLockWaitsOn(t, pool, table, n, limit)`：只数等待某张表的锁的语句。
9. README 与 `image-smoke.sh`：停机的宽限期 40 秒；River 多占一个连接。

## 测试

- 迁移核对；schema 的 up/down。
- jobs：用测试注册的任务（每秒一次的定时任务、不理会取消的任务）：按间隔运行、停机等待与超时、最多 2 个 worker、同种 worker 重复被拒、serve 的 ctx 取消之后 River 仍在运行直到 `Stop`。
- serve：待执行的迁移时不启动（日志有 warn、没有 "jobs started"），`migrate up` 之后启动；停机顺序（卡在行锁上的请求、日志顺序）。
- 反向对照：任务先于 HTTP 停；`Start` 用调用方的 ctx；不等迁移就启动。

## 完成检查

`make check`、`make gen-check` 为绿；`make e2e` 照旧通过（同一个库上两个 River 客户端）。
