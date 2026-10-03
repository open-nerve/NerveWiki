# M5/P2/S4 组合根：注册、Listener 的启动与停机、最后一跳、README、反向代理：实施计划

上级：[P2 文档](../02-P2-event-stream.md) 3.6、3.11、第 5 节；[M5 总设计](../00-M5-design.md)第 7、8 节。

## 任务

1. `bootstrap/events_registrants.go`：`pageEvents`（观察者与会话订阅者）、`notebookEvents`（可见性与删除的订阅者），都经 `events.NewPublisher(pool)`；`pageRegistrants(pool)`、`notebookRegistrants(pool)` 登记它们。
2. `newApp` 建 Listener 与 events 模块，回调接到 hub；`serve` 的启动与停机次序（P2 文档 3.11）；`TestServeRunsUntilCancelled` 的日志次序。
3. 改 `page/app/extension.go` 里"一次通知"的注释。
4. 整个程序的测试：观察者、会话订阅者、可见性、笔记本删除的每条路径与不触发的路径（表格驱动，开一条真实的流读到事件或 `reset`）；"谁收得到 lab 的事件"按笔记本级 13 列；Listener 断开之后开着的流收到 `reset reconnected`。
5. README：事件流一节（路由、事件、`reset`、心跳与配置、连接数 `max_conns + 2`）、Caddy 与 nginx 的配置；总体设计 3.11 的心跳与模块清单。
6. 反向代理的验证：Caddy、nginx 的容器各一次，帧及时到达、连接活过 `proxy_read_timeout`；结果写进 P2 文档的结果一节。

## 测试

上面第 4 项本身就是测试；反向对照：组合根不注册某一个注册者（四个各一次）。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；整个程序的事件测试另跑 `-race -count=5`。
