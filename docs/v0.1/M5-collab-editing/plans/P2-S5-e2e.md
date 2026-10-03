# M5/P2/S5 端到端：实施计划

上级：[P2 文档](../02-P2-event-stream.md) 3.13；[M5 总设计](../00-M5-design.md)第 3 节的 C7、C8。

## 任务

1. `e2e/fixtures/events.ts`：用 Node 的 `fetch` 开流，逐帧解析，`next(type)`（带超时）、`close()`。
2. `e2e/stories/collab/c7-push.spec.ts`：C7 的接口版本。
3. `e2e/stories/collab/c8-stream-life.spec.ts`：C8 的接口版本（短的访问令牌与心跳经 `nervewikiWith`）。

## 测试

每条故事核对帧与它的数据，看不到的人收不到。反向对照（e2e）：hub 不按笔记本过滤（C7 失败）、心跳不重新认证（C8 退出登录一格失败）。

## 完成检查

`make e2e` 为绿。
