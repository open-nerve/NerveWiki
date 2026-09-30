```yaml
status: open
from: M0/P1
to: M5
created: 2026-09-30
```

# SSE 经过反向代理的验证

M0/P1 的实验 ③（[结果](../../M0-foundation/01-P1-spikes.md) 第 7 节）验证了直连与 Caddy 默认配置下的 SSE：不缓冲、心跳有效、`Authorization` 在日志中脱敏。以下情况没有测，由 M5 实测，并把可用的配置写进部署文档：

1. Caddy 开启 `encode`（gzip / zstd）时，`text/event-stream` 是否被缓冲；如果被缓冲，给出排除事件流的写法。
2. nginx：`proxy_buffering`、`proxy_read_timeout` 与 `X-Accel-Buffering: no` 的配合。
3. HTTP/1.1（没有 TLS）部署下，多个标签页共用一条事件流（总体设计 3.11）的实际表现：开 10 个标签页，普通请求不被阻塞，持有连接的标签页关闭后由别的标签页接手。

4. **长连接路由绕过的中间件**（M0 收尾审查）：事件流用 `httpserver.LongLived` 挂载，它豁免请求期限，同时绕过全部逐路由中间件（总体设计 13.1 第 14 条）。事件流要自己完成认证（带 Bearer 的 `fetch`），不依赖接口操作的认证中间件；由测试证明未认证、令牌过期的请求被拒绝。
   M1 之后逐路由的链上还有请求信息（经 `server.trusted_proxies` 认出的客户端 IP；`LongLived` 上 `RequestMetaFrom` 是零值）、认证之前的失败闸门与按凭证的限流，都在 httpserver 未导出的中间件里（M1 收尾审查）。第一个挂载长连接路由的 M 由平台导出一个复用这些部件的入口（请求信息、失败闸门、`Authenticator`、按凭证的限流、请求体上限），不在模块里重写（总体设计 13.1 第 14 条）。
