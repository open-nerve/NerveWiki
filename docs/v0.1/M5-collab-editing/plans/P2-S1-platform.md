# M5/P2/S1 平台：NOTIFY、Listener、长连接与重新认证、凭证的到期、x-long-lived、配置：实施计划

上级：[P2 文档](../02-P2-event-stream.md) 3.2、3.3、3.9、3.10；[M5 总设计](../00-M5-design.md) 4.10。

## 任务

1. `platform/postgres/notify.go`：`Notify(ctx, channel, payload)`，只在 ctx 的事务里执行，超过 7999 字节报错。
2. `platform/postgres/listener.go`：`NewListener(pool, channel, logger, opts)`、`Run(ctx)`；回调 `OnNotify`、`OnListening`、`OnReconnect`；退避经选项注入；`Run` 返回时连接已关闭。`pgtest` 加断开某个连接的工具（`pg_terminate_backend`，按 `application_name` 或 pid）。
3. `platform/httpserver`：`WithCredentialExpiry`、`CredentialExpiry`；`API.LongLived(h)`（请求信息 → 闸门与认证 → 限流 → 包级 `LongLived`）；context 里的 `Reauthenticate(ctx)`（同一个令牌、不经闸门、不计限流）。
4. identity：认证用例返回到期时刻；`authn` 把它放进 context。
5. `apitest`：`Operation.LongLived`（`x-long-lived`）；`CheckResponse` 对它只看状态与答复头。
6. 配置 `events.heartbeat_interval`（默认 20 秒，5–50 秒），照 `page.edit_session_cleanup_interval`；`bootstrap/app_test.go` 的测试配置一并设上。

## 测试

P2 文档第 5 节"平台"一项；配置的默认值、校验的边界（5、50 秒）与失败的次序。反向对照：`Notify` 不要求事务、Listener 重连之后不调 `OnReconnect`、`LongLived` 不经认证、`Reauthenticate` 经闸门。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；Listener 的测试另跑 `-race -count=10`。
