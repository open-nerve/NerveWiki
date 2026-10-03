# M5/P2/S2 events 的领域与 app：事件与载荷、hub、连接的建立、发布者：实施计划

上级：[P2 文档](../02-P2-event-stream.md) 3.4–3.6；[M5 总设计](../00-M5-design.md) 4.10、第 8 节。

## 任务

1. `events/domain`：`Event`、四种类型的数据、`Encode`（7999 字节处按类型退化）与 `Decode`、帧（`hello`、事件、`reset`、心跳的注释）、`reset` 的原因。
2. `events/app/ports.go`：`Visibility`（`WorkspacesOf`、`NotebooksIn`）、`Notifier`。
3. `events/app/hub.go`、`connection.go`：登记与注销、`Dispatch`（按笔记本、按工作区、`access`、`notebooks_deleted`）、`ResetAll`、`Listening`；每个连接容量 64 的缓冲，满了标为 `overflow`。
4. `events/app/open_stream.go`：没在监听答 `not_ready`（503，`Retry-After: 1`）；先登记、再算可见集合、再交出集合并处理排着的事件。
5. `events/app/publisher.go`：`Publish` 与 `PagesWritten`、`LockChanged`、`AccessChanged`、`NotebooksDeleted`，经 `Notifier`。

## 测试

P2 文档第 5 节"events"一项，全部用假端口。反向对照：hub 不按笔记本过滤、`access` 不看 `reached`、缓冲满了丢事件、先计算后登记、`pages` 不带 `tree`、`lock` 不带会话 id。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；hub 的测试另跑 `-race -count=10`。
