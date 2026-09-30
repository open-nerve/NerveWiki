# M0/P3/S2 数据库：实施计划

上级：[P3 文档](../03-P3-server-platform.md) 3.3。

## 任务

1. `internal/platform/postgres`：从 Nerve 拷贝 `pool.go`、`tx.go`、`migrator.go`，改名。
2. 新写 `check.go`：`CheckDatabase(ctx, pool) error`。
   - 一条查询读出 `pg_encoding_to_char(encoding)`、`datlocprovider`、`datcollate`、`datctype`，以及 pg_trgm 是否已安装。
   - pg_trgm 已安装时再查 `show_trgm('中文')`。
   - 所有问题一次报出，并附上正确的建库命令（总体设计 7.1）。
3. `internal/platform/postgres/pgtest`：从 Nerve 拷贝 `pgtest.go`。
   - 镜像改为 `postgres:18.6-trixie`，初始化参数设为 builtin `C.UTF-8`（testcontainers 的环境变量 `POSTGRES_INITDB_ARGS`）。
   - `lockwait.go` 是 M2 的并发测试工具，随用到它的 M 引入。
4. `migrations`：`embed.go`、文件名约定的测试，以及 `sql/00001_platform_pg_trgm.sql`（`+goose Up`：`CREATE EXTENSION IF NOT EXISTS pg_trgm`；`+goose Down`：`DROP EXTENSION IF EXISTS pg_trgm`）。
5. 依赖：pgx v5、goose v3、testcontainers-go 的 postgres 模块，取最新版。

## 测试

- 拷贝 Nerve 的 `pool_test`、`tx_test`、`migrator_test`，改名、按需裁剪。
- `check_test`（集成）：
  - 模板库通过；
  - 以 `LC_COLLATE 'C' LC_CTYPE 'C'` 新建的库，报出 ctype 与 collate；
  - libc provider 的库报出 provider；
  - 没有安装 pg_trgm 的库不做 `show_trgm` 检查，但其余检查照常。
- `pgtest` 建出的库本身满足自检（即上一项的第一条）。

## 完成检查

`make check` 为绿；需要 Docker。
