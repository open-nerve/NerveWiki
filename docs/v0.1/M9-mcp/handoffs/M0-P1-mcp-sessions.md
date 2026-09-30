```yaml
status: open
from: M0/P1
to: M9
created: 2026-09-30
```

# MCP 会话生命周期的验证

M0/P1 的实验 ⑤（[结果](../../M0-foundation/01-P1-spikes.md) 第 7 节）验证了 go-sdk v1.8.0 的挂载、认证、按笔记本下发 instructions，以及 Claude Code、Codex 的实际连接。会话的生命周期没有验证，由 M9 处理。设计上的要求已经写进总体设计 6.3：`SessionTimeout`、server 缓存按 LRU 回收、每个请求先判权限再查缓存、工具注解。

1. **重启后的恢复。** 会话存在进程内，服务重启后失效。用 Claude Code 与 Codex 实测：
   - 服务重启之后，客户端能否自动重新握手并继续调用工具；
   - 会话因 `SessionTimeout` 被回收后，结果是否一样。
2. **无状态模式。** 评估 SDK 的 Stateless 模式：它不需要会话状态，重启无感，多实例也不需要会话粘滞；代价是握手之后的请求拿不到 clientInfo，变更集的 `mcp:<客户端名>` 要换一种来源（例如由 PAT 的名称或请求头给出）。两种模式择一，写进 M9 的设计。
3. **审批设置的建议。** 在 Codex 与 Claude Code 上核对工具注解的效果：`readOnlyHint` 的工具能否免审批。据此在接入文档里给出各审批选项的建议，不一律推荐 `approve`。

4. **长连接路由绕过的中间件**（M0 收尾审查）：MCP 的 Streamable HTTP 路由用 `httpserver.LongLived` 挂载，它绕过全部逐路由中间件（总体设计 13.1 第 14 条），包括请求体上限、认证与限流。MCP 的 POST 带请求体，要自己限制大小（例如 `http.MaxBytesReader`），自己完成 PAT 认证与限流，并有测试覆盖。
