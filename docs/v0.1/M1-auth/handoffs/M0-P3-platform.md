```yaml
status: open
from: M0/P3
to: M1
created: 2026-09-30
```

# 平台层留给 M1 的部分

M0/P3 只做了 M0 用得到的平台层（[P3 文档](../../M0-foundation/03-P3-server-platform.md) 第 2 节"不做"一表）。以下几项在 Nerve 中都有现成实现，M1 引入第一个需要它们的功能时一起拷贝、改名、裁剪，按 M0 的"拷贝即接管"原则逐个文件审查：

1. **客户端 IP**：`httpserver/clientip.go` 与配置项 `server.trusted_proxies`（CIDR 列表）。配置层要恢复列表键的解码钩子（Nerve 的 `listHook`，以及 `netip.Prefix` 的解码），并补上对应的越界、空值、逗号分隔等测试。部署在反向代理后面时必须配置，否则所有人都按代理的地址限流。
2. **限流**：`platform/ratelimit` 与配置节 `ratelimit`（各个桶的 `per_minute`、`burst`，IPv6 按前缀计数）。test profile 的桶要调高，否则大量注册、登录的测试会被限流。
3. **认证的接入**：`httpserver` 的 `Authenticator` 接口、公开操作的清单（`PublicOperations`），以及接口操作的逐路由中间件里认证、限流的顺序。P4 先建好逐路由中间件（请求期限、请求体上限），M1 在其中加入认证与限流。长连接路由（`LongLived`）怎样认证，由 M1 与 M5 决定：它们不经过接口操作的中间件。
4. **后台任务**：`platform/jobs`（River）与配置节 `jobs`（`shutdown_timeout`）。停机顺序改为 HTTP → 后台任务 → 迁移器 → 连接池；River 的表与业务表在同一条迁移链上（总体设计 7.1）。
5. **非 prod 却对外监听的告警**：Nerve 的 `warnIfExposed`。它告警的是开放注册与临时签名密钥，M1 有了这两样才有意义。
6. **命令行的组合**：`nervewiki users` 这类管理命令只组合连接池与模块的管理用例，不构建 HTTP 服务、限流与后台任务。Nerve 用 `archtest/composition_test.go` 沿静态调用图检查这一点，随第一个管理命令引入。
7. **架构测试中与 sqlc 相关的规则**（`sqlc_test`、`rawsql_test`）：随第一个用 sqlc 的模块引入。
8. **测试用的固定时钟**：Nerve 的 `clock/clocktest`。M0 没有用到它的代码，P3 审查时删掉了。它不是并发安全的，而会话过期这类测试会在 HTTP 集成测试中与处理请求的 goroutine 同时读它，引入时加锁，并把它加进架构测试规则 8（测试辅助包只被测试导入）。
9. **时区数据库**：个人偏好里如果有时区，二进制要内嵌 `time/tzdata`，并恢复 Nerve 检查它的架构测试；否则可以接受的时区会随宿主机变化。
10. **指向机密的 `*_file` 配置键**（例如 JWT 私钥文件）：M0 唯一的 `*_file` 键 `server.addr_file` 不是机密，启动日志与错误信息都显示它的路径（P3 审查 N2）。Nerve 对所有 `*_file` 键只记录"是否设置"；M1 加入私钥文件时按键决定，并补上对应的日志测试。

## 处理进展

- M1/P1（2026-09-30）：第 1 项（`clientip.go`、`server.trusted_proxies` 与列表、`netip.Prefix` 的解码）、第 3 项（`Authenticator`、`PublicOperations`、认证中间件；`LongLived` 的认证仍由 M5 决定）、第 5 项（`warnIfExposed`）、第 7 项（`sqlc_test`、`rawsql_test`）、第 10 项（`auth.jwt.private_key_file` 只记 `private_key_file_set`，有日志测试）。见 [P1 文档](../01-P1-identity-foundation.md)第 7 节。
- M1/P2（2026-09-30）：第 2 项（`platform/ratelimit`、平台与模块的桶、失败闸门、IPv6 前缀）、第 8 项（`clocktest.Fixed`，加锁，列入架构测试的测试辅助包）。见 [P2 文档](../02-P2-sessions-ratelimit.md)第 7 节。
- M1/P4（2026-10-01）：第 4 项（`platform/jobs`、`jobs.shutdown_timeout`、停机顺序 HTTP → 后台任务 → 迁移器 → 连接池、River 的迁移与业务表同一条链）、第 6 项（`nervewiki users` 只组合连接池与管理用例，`archtest/composition_test.go`）。见 [P4 文档](../04-P4-admin-jobs.md)第 7 节。
- 其余：第 9 项已在 M1 总设计第 2 节关闭（不做服务端时区）。全部落实，状态在 M1 收尾时改为 done。
