# M0/P3/S1 配置、日志与时钟：实施计划

上级：[P3 文档](../03-P3-server-platform.md) 3.1、3.7。

## 任务

1. `internal/platform/config`：从 Nerve 拷贝 `config.go`、`load.go`、`validate.go`，只保留 `server`（去掉 `request_timeout`、`max_body_bytes`、`trusted_proxies`）、`database`、`log` 三节。
   - 改名：`NERVE_` → `NWIKI_`，`nerve` → `nervewiki`；删掉指向 Nerve 文档的编号引用。
   - 严格解码的钩子原样保留（空值、时长、数字越界、列表）。`listHook` 在 M0 没有列表键，暂不保留，随第一个列表键加入。
2. `configs`：`config.yaml`、`config.{dev,test,prod}.yaml`、`embed.go`。dev 的数据库地址指向 `make dev-db`（55433）。
3. `internal/platform/logging`、`internal/platform/clock`：拷贝、改名。
4. 依赖：koanf v2（yaml 解析器、env v2、rawbytes 提供者）、mapstructure v2，取最新版。

## 测试

- 拷贝 Nerve 对应的测试，裁掉已删配置节的用例；改名。
- `configs` 的内置 profile 测试：三个 profile 的完整期望值；prod 缺少 `database.url` 时报错。
- 环境变量：`NWIKI_DEV_DB_PORT`、`NWIKI_ENV` 这类不带 `__` 的变量不是配置键（P2 审查的要求）。

## 完成检查

`make check` 为绿。
