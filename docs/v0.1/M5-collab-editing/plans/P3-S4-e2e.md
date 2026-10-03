# M5/P3/S4 端到端：实施计划

上级：[P3 文档](../03-P3-event-stream-web.md) 3.12；[M5 总设计](../00-M5-design.md)第 3 节的 C7–C9、第 9 节。

## 任务

1. 夹具：`anotherTab`（同一上下文的新标签页，受监视、结束时核对安静）、`eventStreams(context)`（开着的事件流的个数）；`watchPage` 的 `apiRequests` 不记事件流。
2. `e2e/stories/collab/c7-push.spec.ts`、`c8-stream-life.spec.ts` 加页面版本；`c9-one-stream.spec.ts`（Web Locks 与租约各一遍）。
3. 跑全部故事，处理事件流带来的变化（请求清单、控制台、短令牌的故事）。

## 测试

每条故事核对页面上看得到的结果；反向对照（e2e）：`tree` 不重读树、`lock` 不重读锁（C7）；不选举（C9）；不在 `pagehide` 让出（C9 的接手）。

## 完成检查

`make e2e` 为绿。
