# M2/P4/S4 清理：实施计划

上级：[P4 文档](../04-P4-deactivation-commands-purge.md) 3.4。

## 任务

1. `platform/jobs/purge.go`：`Purger{Table, Purge}`、`PurgeConfig{Interval, Retention, Logger}`、`PurgeJob`；worker 按顺序调用清理器，每个直到一批不满，失败即停。
2. 配置：`jobs.purge_interval`（默认 1h，至少 1s；测试配置 2s）、`jobs.purge_retention`（默认 1440h，至少 1h）；配置文件的注释、校验与测试。
3. workspace 模块根 `purgers.go`：`Purgers(pool)`：邀请、成员、工作区。
4. 组合根：`registrants.go` 的 `purgers(pool)`；`newApp` 把 `PurgeJob` 交给 `jobs.New`。

## 测试

- `platform/jobs`：清理器的顺序；一批满时再删一批；失败时后面的不运行、错误交给 River；删了行才有日志。
- 组合根（真实数据库）：每张有 `deleted_at` 的表恰有一个清理器；每条指向被清理表的外键，引用方也被清理、排在前面；超过保留期的工作区连同成员与邀请被删、保留期内的不动（直接调用组合根的清理器列表，按任务的循环）。
- 运行时角色：清理任务以运行时角色完成一次。
- 反向对照：清理器颠倒顺序，外键顺序的测试失败；去掉成员的清理器，登记测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
