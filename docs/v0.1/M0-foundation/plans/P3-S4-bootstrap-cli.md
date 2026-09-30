# M0/P3/S4 共享内核、组合根与命令行：实施计划

上级：[P3 文档](../03-P3-server-platform.md) 3.2、3.5。

## 任务

1. `internal/shared`：`error.go`（`Kind`、平台错误码、通用的字段错误码）、`tx.go`（`TxManager` 端口）。只依赖标准库。
2. `internal/bootstrap`：
   - `app.go`：装配连接池、迁移器、路由（数据库与迁移两个就绪检查）；`run` 依次做迁移（按配置）、数据库自检、服务；`close` 先关迁移器，再关连接池（等待有上限）。
   - `commands.go`：`Serve`、`MigrateUp`（之后执行自检）、`MigrateDown`、`MigrateStatus`。
   - 编译期断言：`postgres.TxManager` 满足 `shared.TxManager`。`*shared.Error` 满足 `httpserver.ProblemError` 的断言随 P4 的 `ProblemError` 一起加入。
3. `cmd/nervewiki`：`main.go`（第一次信号优雅停机，第二次立即退出）、`commands.go`（`serve`、`migrate up|down|status`、`version`）。
4. `Makefile`：`run`（`NWIKI_ENV=dev go run ./cmd/nervewiki serve`）。
5. 依赖：cobra，取最新版。

## 测试

- `shared`：错误的状态映射、`Error()`、字段错误。
- `bootstrap`（集成）：
  - `Serve` 在 `:0` 上启动，`/healthz`、`/readyz` 为 200；
  - 取消 context 后返回 nil；
  - 自检失败时拒绝启动，并记录结构化日志；
  - `migrate` 三个命令的输出；
  - `auto_migrate` 关闭且有待执行的迁移时，`/readyz` 为 503。
- `cmd/nervewiki`：未知子命令、多余参数失败；`version` 的输出格式；配置错误的退出码与信息。

## 完成检查

`make check` 为绿；本地 `make dev-db` + `make run` 冒烟通过。
