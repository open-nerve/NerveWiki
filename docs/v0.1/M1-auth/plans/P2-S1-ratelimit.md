# M1/P2/S1 平台的限流：实施计划

上级：[P2 文档](../02-P2-sessions-ratelimit.md) 3.2、3.3、3.7。

## 任务

1. `platform/clock/clocktest`：`Fixed`（`At`、`Now`、`Advance`），互斥锁保护；架构测试规则 8 把它列为测试辅助包。需要推进时间的测试（S3 的会话过期）用它；只读一个时刻的测试替身保持原样。
2. `platform/ratelimit`：`Rate`、`Limiter`、`Bucket`（`Allow`、`Reserve` 与退还）、`AllowAll`、满桶的清扫（取单位时顺带进行，没有后台 goroutine）；单调时钟。
3. 配置：`ratelimit` 节（`ipv6_prefix_len` 与六个桶）、`auth.refresh_deadline`；校验（每个桶至少 1、前缀 1–128、`refresh_deadline + database.commit_timeout < 8s`）；日志表示；test 配置把桶调到用不完。
4. httpserver：客户端 IP 的计数键（IPv6 按前缀）进入请求信息；`limit.go`（`Limiter` 端口、限流中间件、429 与 `Retry-After`）；失败闸门并入认证；`Authenticator` 返回凭证的键，过期的访问令牌是可退还的 401。
5. identity 的 `authn`：凭证键 `session:<id>`；`app.ErrAccessTokenExpired` 包装成过期的 401。
6. 组合根：一个 `ratelimit` 实例、平台的三个桶；顶层 `x-problem-codes` 加 `rate_limited`；整个程序的测试证明三个桶按配置生效。

## 测试

- 令牌桶的单元测试；配置的校验；IP 键（IPv4、IPv6 的前缀、IPv4 映射的 IPv6）。
- httpserver：公开操作按 IP、其余按凭证；429 带 `Retry-After`；失败闸门在认证之前、并发不超额、过期退还。
- 反向对照：闸门在认证之后才取单位 → 并发测试失败；过期不退还 → 闸门测试失败；`refresh_deadline + commit_timeout` 达到 8 秒 → 校验测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
