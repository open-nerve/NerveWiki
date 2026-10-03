# M5/P2/S3 可见的端口、events 的适配器与模块根、契约、文案、矩阵豁免：实施计划

上级：[P2 文档](../02-P2-event-stream.md) 3.6–3.9、3.12；[M5 总设计](../00-M5-design.md) 4.10。

## 任务

1. workspace 的根 `Memberships.WorkspacesOf`；notebook 的根 `Notebooks.VisibleIn`。
2. `events/adapter/postgres/notifier.go`：`Notifier` 经 `postgres.Notify(ctx, "nwiki_events", …)`。
3. `events/adapter/http/handler.go`：流的处理器（P2 文档 3.8），`Deps.After` 注入计时器。
4. `events/module.go`：`New(Deps{…, HeartbeatInterval, After})`、`Register(router, api)` 经 `api.LongLived`、`Hub()`；`events/publisher.go`：`NewPublisher(pool)`。
5. 契约：`api/modules/events.yaml`（`streamEvents`，`x-long-lived`，帧数据的四个 schema），`api/openapi.yaml` 的路径与标签；Makefile 的 `API_MODULES` 去掉 `events`；`make gen`。
6. `bootstrap` 的 `sendRequest` 对长连接的操作只读答复头；"每个操作都接受 PAT"与"公开的操作"两项测试随之。
7. 权限矩阵整模块豁免 `events`；`not_ready` 的中英文案。

## 测试

处理器（`httptest` 的真实服务、假的 hub 端口与计时器）：答复头、`hello`、事件帧、心跳、重新认证失败、到期、停机，每种帧经 `CheckSchema`；`apitest.Main` 答出 `not_ready`。可见的端口：权限矩阵种子的每一列"列表等于逐项判定"。反向对照：心跳不重新认证、不设到期的计时器。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
