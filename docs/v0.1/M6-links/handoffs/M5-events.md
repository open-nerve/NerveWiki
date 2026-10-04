```yaml
status: open
from: M5 收尾
to: M6
created: 2026-10-04
```

# 实时推送的事件类型：加一个类型

M5 建了实时推送的事件类型（[M5 总设计](../../M5-collab-editing/00-M5-design.md)第 8 节；总体设计 12.4）。M5 自己的类型是 `pages` 与 `lock`；`hello` 与 `reset` 管连接，`access` 与 `notebooks_deleted` 只在服务端，让受影响的流以 `reset` 结束。M6 的链接状态是第一个后来的类型，M10、M11 的笔记本模式随后（它们的移交指向这里）。

1. **服务端**：别的模块经 events 的模块根发布（`server/internal/modules/events/publisher.go`）。
   - 在调用方的事务里调 `Publisher.Publish(ctx, events.Event{Type, WorkspaceID, NotebookID, Data})`；发布者由组合根的 `events.NewPublisher()` 给出，照 `bootstrap/registrants.go` 的 `pageEvents`。
   - `NotebookID` 不为零时按笔记本的可见性过滤，为零时是工作区级的事件（见第 4 项）。
   - `Data` 是只含 id 的 JSON 对象，不能是数组或标量：监听的一侧丢掉读不懂的载荷。
   - 载荷不超过 `MaxPayload`（7999 字节）。超出时 `Publish` 答 `ErrTooLong`，整个写入单元回滚，所以数据随写入变大的类型要照 `pages` 退化：多于 20 页时 `pages` 为 `null`，见 `events/app/publisher.go` 的 `shedding`。
   - 同一事务里载荷相同的两条 `NOTIFY` 会被 PostgreSQL 合并成一条，要带区分它们的内容。
   - 类型不能为空，不能含冒号或换行，不能是流自己的帧 `hello`、`reset`：这样的类型 `Publish` 答 `ErrNoType`（`events/domain` 的 `CheckType`），整个写入单元回滚。也不要用 M5 的 `pages`、`lock`、`access`、`notebooks_deleted`：它们不被拒绝，但 hub 与前端会按 M5 的意思解释（`access`、`notebooks_deleted` 让流 `reset`）。
   - hub 不认识的类型照转（`events/app/hub_test.go`）。
   - 模块根（`events/publisher.go`）现在只导出 `Publisher`、`Event` 与 M5 自己的几个值，`ErrTooLong`、`ErrNoType`、`MaxPayload` 在 `events/domain` 里，组合根导入不到（只有组合根导入模块，而且只导入模块根，archtest）：要按它们分支或退化时，先在模块根以别名导出。
2. **前端**：在 `web/apps/web/src/events/handlers.ts` 的 `eventHandlers` 里加一项 `[类型, 处理函数]`，流与标签页之间会把不认识的类型连同数据转过来（帧 `other`）。
   - 处理函数拿到的是服务端发的 JSON（类型自己断言），以及 SWR 的 `cache`、`mutate`、合并阅读视图重读的 `refresher` 与 `stopped`。它只让 SWR 重读挂着的东西。
   - 每次连上时，`app/event-stream.tsx` 的 `refreshedOnConnect` 从外向内一层层重读工作区、笔记本、树、阅读视图与锁。新类型的数据若有自己的 SWR 键，就加进对应的一层（这是 M5 的代码，加一行）。
   - 契约 `api/modules/events.yaml` 的事件描述加这个类型，`services/event.service.ts` 导出它的类型。
3. **最后一跳的测试**（总体设计 13.1 第 21 条）。
   - M5 已有的替身测试：服务端 `events/app/publisher_test.go` 与 `hub_test.go` 用 `links` 类型；前端 `events/hub.test.ts` 证明不认识的类型连同数据到达每个标签页，`app/event-stream.test.tsx` 证明"后来的类型到达它的处理函数"。
   - M6 注册时，在整个程序上经触发它的每条写入路径各加一个行为测试：在 `serve` 上开一条流，读到这个类型的事件（照 `bootstrap/events_pages_test.go`），组合根不发布时失败。
   - 前端加一个经组合根的 `eventHandlers` 的组件测试，注册表里没有这一项时失败。
4. **工作区级的事件到不了一部分人**（[M5/P2 审查](../../M5-collab-editing/reviews/P2-event-stream-review.md) B-m3，[P2 文档](../../M5-collab-editing/02-P2-event-stream.md)第 7 节）：流的工作区集合在建立时就算好了。以下路径不发 `access`，所以这些工作区的工作区级事件到不了，直到别的原因让流重连：
   - 访客接受邀请；
   - 恢复成访客而没有归还的笔记本；
   - 新建工作区。

   M5 的类型都按笔记本过滤，不受影响。加工作区级类型的 M 要同时让这些路径发 `access`，并各加一个行为测试。
