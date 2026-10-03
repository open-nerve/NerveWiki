# M5/P3/S1 服务、帧与连接、持有者的选举、频道、重读的合并、目录规则：实施计划

上级：[P3 文档](../03-P3-event-stream-web.md) 3.2–3.5、3.8 的合并、3.11；[M5 总设计](../00-M5-design.md) 4.11。

## 任务

1. `services/event.service.ts`：`EventService.open(signal)`（`parseAs: "stream"`，不是 200 照 `unwrap` 抛 `ApiError`）；`services/page.service.ts` 加 `lock(id)`、`releaseLock(id)`。
2. `events/frames.ts`：`parseFrames`，按行解析，产出 `hello`、`pages`、`lock`、`reset`、`beat`、`other`；类型取生成的 `EventHello`、`EventPages`、`EventLock`、`EventReset`。
3. `events/connection.ts`：`connect(open, signal, onFrame)`，返回 `reset`、`closed`、`failed`、`aborted` 四种结束。
4. `events/leadership.ts`：`Leadership`（`run`、`steal`、`yield`）的 Web Locks 与租约两种实现；租约的取得（写入、等 50 毫秒、读回）、续期（`renew(ttl)`）、过期的检查。
5. `events/channel.ts`：消息的类型（`frame`、`beat`、`connected`、`reconnecting`、`yield`，都带 `loginId`）与收发，别的一代的消息丢掉。
6. `events/refresher.ts`：按键合并重读（看得见时 5 秒至多一次、隐藏时等到可见）。
7. `.oxlintrc.json`：`src/events/**` 的目录规则。

## 测试

P3 文档第 5 节"单元"的帧、连接、选举（两种各一遍）、重读的合并；服务的 `open`（200 给流，503 带 `retryAfter`）、`lock`、`releaseLock`。反向对照：持有者不续租约；合并不限 5 秒。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿。
