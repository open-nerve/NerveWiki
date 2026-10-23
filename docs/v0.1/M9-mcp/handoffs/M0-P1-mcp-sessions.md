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
   M1 之后逐路由的链上还有请求信息（经 `server.trusted_proxies` 认出的客户端 IP；`LongLived` 上 `RequestMetaFrom` 是零值）、认证之前的失败闸门与按凭证的限流，都在 httpserver 未导出的中间件里（M1 收尾审查）。第一个挂载长连接路由的 M 由平台导出一个复用这些部件的入口（请求信息、失败闸门、`Authenticator`、按凭证的限流、请求体上限），不在模块里重写（总体设计 13.1 第 14 条）。`httpserver.Authenticator` 同时接受会话的访问令牌与 PAT，而总体设计 6.3 说 MCP 只收 PAT：是否拒绝访问令牌由 M9 决定。

## 处理进展

- M9/P1（2026-10-18，合并 `384d9f96`、`a36e92df`、`c9468e5b`）与 P5（2026-10-23，合并 `053d3ba1`）：**第 2 项（做完）**：选无状态（`mcpserver.New` 的 `Stateless`、`JSONResponse`），没有会话与 `SessionTimeout`；协议版本至 2025-11-25；客户端名称取 `clientInfo.name` 或请求的 `_meta`，清理之后记在 PAT 的 `client_name`（迁移 00043；`TestTheClientNameIsTheTokens`）（[M9 总设计](../00-M9-design.md) 4.3、4.5）。**第 4 项（做完）**：`/mcp` 不经 `LongLived`，经 `API.Wrap` 的逐路由中间件（请求信息、期限、请求体上限、失败闸门与认证、按凭证的限流、大的请求体的名额、`mcpserver.Bodies()` 的结构检查，拒绝 JSON-RPC 的批）；只收 PAT，会话的访问令牌答 403；答复一律 `no-store`；SDK 里的 panic 恢复为 `internal_error`（`TestTheMCPEdgeAnswersInOrder`、`TestTheMCPRefusesABatch`、`TestTheMCPRecoversAToolsPanic`、`TestEveryMCPAnswerIsNotStored`、`TestTheMCPRoutesTakeTheBuckets`；4.3、4.4）。**第 1 项的代码部分**：重启对客户端不可见（`TestTheMCPOutlivesARestart`：同一个库上重建应用，官方 Go 客户端不重新握手就能调用）；在 Claude Code、Codex 上的实测是[人工清单](../manual/P5-mcp-clients-checklist.md)的 G 部分（G1、G2）。**第 3 项的代码部分**：工具的注解（第 5 节），README 的"审批"不一律推荐 `approve`；各审批选项的实际效果是清单的 D 部分（D1–D8），README 据此补写每个选项问什么。
- M9 收尾（2026-10-23）：第 2、4 项做完；第 1、3 项等清单的 G、D 两部分（[M9 收尾审查](../reviews/M9-closeout-review.md)"handoff"与"人工验收"）。清单由负责人在 v0.1 的最后与输入法清单一起执行，跑过之后照记下的结果关闭这一份，同 M9 的状态"实现完成，待人工验收"（M9 总设计第 11 节主持者的决定第 98 条、设计的取舍第 12 条）；在那之前照旧 `open`。
