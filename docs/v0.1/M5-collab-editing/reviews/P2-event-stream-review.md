# M5/P2 事件流（后端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m5-p2`（`d093ea7..371503d`：5 个提交，S1 `7c86df2`、S2 `3ec34ee`、S3 `33abbdd`、S4 `6016bee`、S5 `371503d`；83 个文件，+4634/−75），对照 [02-P2-event-stream.md](../02-P2-event-stream.md) 第 1–6 节、五份 Step 计划、[M5 总设计](../00-M5-design.md)第 4.10、4.11、8 节与三份移交（M4/P1 第 4 项、M4/P4 第 8 项、M3/P2 第 1 项） |
| 审查方式 | 两位独立审查者（Opus）在仓库上只读：A 看机制的正确性与并发（平台的 NOTIFY、Listener、长连接与重新认证，events 的领域、hub、连接、发布者与处理器，`serve` 的启动与停机），B 看集成、授权、契约与测试（注册者、可见端口、流的认证、契约与文档、整个程序与 e2e 的测试）。A 在 events、httpserver、Listener 上跑 `-race`（events 的三个包 `-count=20`），并用 `go test -overlay`（探针在临时目录，不进仓库）复现了 A-I1；B 跑了 events 与 httpserver 的测试、bootstrap 的全部事件测试与"每个操作都接受 PAT"。修复之后另由一位 Opus 核对（见"修复的核对"） |
| 日期 | 2026-10-03 |
| 结论 | 没有阻断合并的问题。注册者齐全且都在写入的事务里，流看得到的笔记本与读接口的判定一致，`access` 按提交次序先于之后的事件到达，hub 的两把锁次序固定。合并之前修 A 的 Important I1（停止读取的客户端让处理器永远阻塞），其余 Minor 修掉或记下。A：Important 1、Minor 4；B：Minor 5 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-I1 | Important | **停止读取的客户端让处理器的写永远阻塞**：`LongLived` 解除了写期限，处理器自己不设；客户端不读之后 TCP 缓冲写满，`Write`/`Flush` 一直阻塞，`select` 不再转：不发心跳、不重新认证、到期的计时器不触发，hub 标了 `overflow` 却写不出 `reset`，`Close` 不执行；停机等满 `server.shutdown_timeout` 并返回错误。A 实测：心跳 300 毫秒、不读的原始 TCP 客户端，撤销凭证 2 秒之后流仍在 hub 里，`Serve` 3 秒之后以 deadline exceeded 返回。冻结的持有者标签页（总设计 4.11）就是这样的客户端 | 已修：每一帧写之前设一个心跳的写期限，写完解除（见 F-M1）；写超时就结束流，客户端重连。`TestAClientThatStopsReadingEndsItsStream`（读缓冲 4 KB、不读的客户端，每毫秒一个 32 KB 的事件，流在 10 秒内离开 hub）；no-write-deadline 失败 |
| A-m2 | Minor | Listener 的 `WaitForNotification` 没有期限：半开的连接（数据库切换、NAT 忘了它）要等 Go 默认的 TCP keepalive 约 150 秒才发现，这期间的通知全丢，开着的流也不 `reset` | 已修：等 `PingInterval`（默认 30 秒）没有通知就 `Ping`（至多 5 秒），失败按断开处理、重连并 `ResetAll`（总设计第 6 节"照 River 的 notifier……定时打断去 Ping"原本就这样写，实现漏了）。超时的等待不关连接（pgx 的 `DeadlineContextWatcherHandler`），`Ping` 期间到的通知由 pgx 缓存。`TestAListenerKeepsAQuietConnection`、`TestAListenerFindsASilentConnectionLost`（一个能让已有连接静默、不关闭的 TCP 代理）；listener-waits-forever、listener-never-pings、listener-drops-quiet-connection 失败 |
| A-m3 | Minor | 连接建立时还不知道看得到什么，整个服务器的事件都排进它的 64 条：建立的那几毫秒里别处来了 64 条以上，新流一开就 `reset overflow` | 已修：建立中的队列另有 `PendingSize`（1024），客户端的缓冲仍是 64；hub 测试加"别人的事件超过 64 条不 `reset`""超过 1024 条 `reset`"；pending-capped-by-buffer 失败 |
| A-m4 | Minor | 客户端离开或停机与心跳同时就绪，或在重新认证读数据库时取消，`context.Canceled` 被当成认证服务出错记 ERROR | 已修：请求的 context 已结束时安静返回。`TestAHeartbeatCutShortIsNoFault`；cancelled-heartbeat-logged 失败 |
| A-m5 | Minor | 停机的总时长（`config.yaml` 的 `jobs.shutdown_timeout` 注释、README）没算 Listener 关连接的至多 2 秒：最坏 38 秒，仍在 `docker stop -t 40` 之内 | 已改：两处都加上（20 + 2 + 10 + 1 + 5 = 38 秒） |
| B-m1 | Minor | 测试会偶发失败：没有什么保证建立数据的写的通知在被测的流登记之前分发完；晚到的通知会被当成等着的帧（`eventsTeam` 的 Marker、删除测试的开启、可见性测试里建 Private 的 `access([alice])`、C8 的 Control 与 `access([B])`、C7 里 C 的 Control） | 已修：开流之前"沉淀"：在自己的一条流上等到一次标记写的帧，按提交次序，之前的通知都已分发；这条流被之前的写 `reset` 就重开。Go 的 `settle` 与 e2e 的 `settleEvents` |
| B-m2 | Minor | `/readyz` 不看 Listener；测试把 503 `not_ready` 当成失败（`nervewikiWith` 的 C8 注册之后立刻开流） | 已修：测试的开流按 `Retry-After` 重试 503；README 写明 `/readyz` 不看这条连接（它断开重连时实例照常就绪，其余接口不受影响）。不把 Listener 放进就绪：它重连的几秒里把整个实例摘下来不值得 |
| B-m3 | Minor（潜在） | 流的工作区集合在建立时算好；访客接受邀请、恢复成访客而没有归还的笔记本、新建工作区都不发 `access`，之后这些工作区的工作区级事件到不了 | 不改，记在 P2 第 7 节：P2 的类型都按笔记本过滤；加工作区级类型的 M 要让这些路径发 `access` |
| B-m4 | Minor | 连接的建立没有期限：`seenBy` 的 N+1 次查询用长连接请求的 context，大家同时重连时会在 `pool.Acquire` 里无限等 | 已修：`Execute` 受 `server.request_timeout` 约束，超时答 500（与别的请求的期限相同）；认证也一样（见 F-M2）。`TestASlowOpeningEndsAtItsTimeout`；no-open-timeout 失败 |
| B-m5 | Minor | 文档说得比代码多：README 与总体设计 3.11 说 `reset` 之前先写出缓冲里的事件，`expired`、`unauthenticated` 却不写；契约说流"开到 `reset` 或客户端离开"，停机、心跳时认证服务出错它也会不带帧结束 | 已改：README、总体设计、契约写明是哪四种 `reset` 先写出、流也会不带 `reset` 就结束而客户端同样重连。凭证到期或失效时不写出是有意的：不再把事件写给已经失效的凭证 |
| — | 缺口 | "每个操作都接受 PAT"对流只要求不是 401，503 也算通过 | 不改：e2e C7 用 PAT 开流得到 200 |

## 修复的核对

修复（`514c792`）之后另由一位 Opus 只读核对：events 与 Listener 的测试 `-race -count=3`，新加的测试 `-count=10`，bootstrap 的事件测试；在临时目录写了一个小程序核实 F-M1。结论：没有 Important，A-I1、A-m2、A-m3、A-m4 与 B-m1 的沉淀都对（沉淀不会空过：按笔记本与 revision 匹配；不会死循环：每帧有 10 秒的上限）。Minor 2、Nit 5，处置如下（`5e8574c`）。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| F-M1 | Minor | 写期限写完之后还留着：下一次写总在它之后。HTTP/1.1 上，在心跳时不带帧结束的流（认证出错、请求已结束）由 net/http 在期限之后写结尾的块，客户端读到截断的分块（`unexpected EOF`），nginx 会记"上游提前关闭"；HTTP/2（现在不提供）上期限是每个流的计时器，安静的流会在每次心跳之前被 reset | 已修：每一帧 `Flush` 之后解除期限。`TestAStreamEndsCleanlyLongAfterItsLastFrame`；deadline-left-set 失败 |
| F-M2 | Minor | B-m4 只修了一半：`API.LongLived` 的认证（读会话，PAT 还可能写最后使用时刻）没有期限 | 已修：`API.LongLived` 的认证与限流在 `server.request_timeout` 之内（`opening`），之后处理器得到认证放进 context 的值，不带这个期限，取消跟着请求自己的（`opened`，`context.WithoutCancel` 加 `AfterFunc`）。`TestALongLivedRoutesOpeningIsBounded`；opening-unbounded、opened-keeps-deadline 失败，opened-ignores-client 让测试挂住 |
| N1 | Nit | README"后四种"数错了 | 已改：写出 `access`、`notebooks_deleted`、`reconnected`、`overflow` |
| N2 | Nit | hub 测试丢了"建立期间自己看得到的事件超过 64 条，交出集合时 `reset overflow`" | 已补；settle-routes-past-buffer 失败 |
| N3 | Nit | Listener 的测试以 50、100 毫秒为 ping 的期限，负载下会误判重连 | 已改：250 毫秒 |
| N4 | Nit | 测试的流一直 503 时报"context canceled"，看不出是 503 | 已改：带上最后的答复 |
| N5 | Nit | P2 文档没写 ping、写期限、`PendingSize`、建立的期限 | 已同步进 P2 文档第 3 节 |

## 反向对照

审查修复 7 项、核对之后的修复 5 项：no-write-deadline、cancelled-heartbeat-logged、no-open-timeout、pending-capped-by-buffer、listener-waits-forever、listener-never-pings、listener-drops-quiet-connection；deadline-left-set、opening-unbounded、opened-keeps-deadline、settle-routes-past-buffer 都失败，opened-ignores-client（处理器听不到客户端离开）让测试挂住。B-m1 的沉淀与开流的重试没有反向对照：它们防的是时序，造不出来。
