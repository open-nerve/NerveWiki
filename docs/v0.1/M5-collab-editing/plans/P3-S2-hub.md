# M5/P3/S2 EventHub、依赖与接线：实施计划

上级：[P3 文档](../03-P3-event-stream-web.md) 3.6、3.7；[M5 总设计](../00-M5-design.md) 4.11。

## 任务

1. `events/hub.ts`：`EventHub`（`start`、`stop`、`subscribe`）；持有者的循环（帧交给自己与频道、续租约、`hello` 之后 `connected`、`reset` 立刻重连、退避 1–30 秒、`Retry-After`、重连期间的 `reconnecting`）；跟随者（排队、记下最后听到的时刻、看得见且沉默 3 个心跳间隔就抢）；页面生命周期（`pagehide`、`freeze` 让出，`pageshow`、`resume` 重新加入）。可以 `stop` 之后再 `start`。
2. `events/deps.ts`：`EventDeps`、`browserEventDeps(storage)`。
3. `stores/root.store.ts`：`AppStores` 的可选 `events`；`RootStore.events()`（登录的这一代、有依赖时建 hub，不启动）。
4. `main.tsx`：浏览器的 `EventDeps`，存储与会话共用。
5. 测试的替身：可控的流（测试持有 `ReadableStream` 的控制器，推帧、关闭）、共用的频道与锁（沿用 `session/testing` 的 `SharedStorage`、`RecordingLock` 的写法，锁支持 `steal`、`signal`）。

## 测试

P3 文档第 5 节"单元"的 hub 一项，Web Locks 与租约各一遍。反向对照：跟随者不检查沉默；不在 `pagehide` 让出。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；hub 的测试另连跑 10 次。
