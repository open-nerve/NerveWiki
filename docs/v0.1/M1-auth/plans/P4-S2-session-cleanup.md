# M1/P4/S2 会话清理：实施计划

上级：[P4 文档](../04-P4-admin-jobs.md) 3.5、3.10、3.11。

## 任务

1. 查询 `DeleteExpiredSessions`（`LIMIT` 加 `FOR UPDATE SKIP LOCKED` 的子查询，别名避免歧义）；仓储方法。
2. 用例 `CleanupSessions`：时钟读一次，每批 1000 行，一批不满即停，删了才记 info。
3. `adapter/river/cleanup.go`：worker 与定时任务 `identity.cleanup_expired_sessions`（`RunOnStart`）。
4. `identity.Module.Jobs()`；`Deps` 加清理间隔；bootstrap 汇总给 jobs。
5. 配置 `auth.session_cleanup_interval`（默认 1 小时，至少 1 秒，test 2 秒）。

## 测试

- 仓储：边界（等于现在的不删）；分批；持有行锁时在 2 秒的期限之内跳过；其他账户、未过期的不动。
- 用例：分批与停止条件；中途失败返回错误与已删的个数；不删不记日志。
- worker：取消的 ctx；清理失败让任务失败。
- bootstrap：按配置的间隔运行（`river_job` 中完成的任务）。
- 反向对照：去掉 `SKIP LOCKED`；删除等于现在的会话。

## 完成检查

`make check`、`make gen-check` 为绿。
