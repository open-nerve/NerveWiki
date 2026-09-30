# M1/P4/S5 运行时角色：实施计划

上级：[P4 文档](../04-P4-admin-jobs.md) 3.9。

## 任务

1. `deploy/runtime-grants.sql`：`nervewiki_runtime` 的授权（schema、业务表、River 的表与序列、`river_job` 的 `MAINTAIN`、`goose_db_version` 的 `SELECT`），幂等。
2. bootstrap 的测试：所有者角色拥有库并迁移；幂等地建组角色；服务的登录角色；执行授权文件。
3. README 的部署：所有者迁移、执行授权文件、服务以运行时角色登录并关闭 `auto_migrate`。

## 测试

- 以运行时角色服务：`/readyz` 200、清理任务完成、主要接口各走一遍、日志没有 "permission denied"、River 的索引 `REINDEX CONCURRENTLY` 成功。
- 目录：`public` 中每个表、视图、序列、函数的权限恰好等于预期。
- 反向对照：授权文件漏一张表 → 目录测试失败。

## 完成检查

`make check` 为绿。
