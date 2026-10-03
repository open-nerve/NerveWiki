# M5/P2 事件流（后端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M5/P2 事件流（后端） |
| 状态 | 进行中 |
| 基线 | `59e6c33`（P1 合并与它的文档提交之后的 main）；本文与各 Step 计划提交之后开分支 `m5-p2` |
| 上级文档 | [M5 总设计](00-M5-design.md) 第 4.10、7–10 节；[M0/P1 移交](handoffs/M0-P1-sse-proxies.md)；[M3/P2 移交](handoffs/M3-P2-visibility.md)；[M4/P1 移交](handoffs/M4-P1-tree-refresh.md)；[M4/P4 移交](handoffs/M4-P4-edit-sessions.md) 第 8 项；[总体设计](../v0.1-design.md) 3.11、12.4、13.1 |

---

## 1. 基线

- **长连接**：平台有包级的 `httpserver.LongLived(logger, h)`（`longlived.go`）：解除写期限，停机开始时取消 `h` 的 context。没有生产代码用它，也不经请求信息、失败闸门、认证与限流。`API` 的中间件（`api.go`）是未导出的方法：请求信息 → 期限 → 请求体上限 → 认证（先过失败闸门）→ 按凭证限流 → 请求体结构。`/api/` 的答复都带 `Cache-Control: no-store`。
- **认证**：`Authenticator.Authenticate(ctx, token)` 返回带 `shared.Actor` 的 context 与限流的键。identity 读到了访问令牌与 PAT 的到期时刻，但丢掉了（`identity/app/authenticate.go`）。认证之后令牌不再保存，没有"再认证一次"的入口。
- **数据库**：没有 LISTEN。River 不是只轮询，已经占一条 `Hijack` 出来的连接（README 写"`max_conns + 1`"）；架构规则不许别处用 River 的 listener，`platform/postgres` 不能导入 `platform/clock`。`pgtest` 有建库与等锁的工具，没有断开连接的。
- **扩展点**（都在调用方的事务里告诉）：
  - page 的观察者 `PageObserver`：每个写入单元一次，没有变化不告诉；开启会话、心跳、相同的保存、笔记本删除不告诉。`Change.Moves()` 说明是否改了树，`Revision` 不为零是写了正文。`extension.go` 的注释写"一个事务至多一条通知"。
  - page 的会话订阅者：开启（P1）、本人结束、接管、强制解锁、删页与删子树、笔记本删除；值都带会话 id。
  - notebook 的可见性变化订阅者：`VisibilityChange{WorkspaceID, UserIDs, Reached, At}`，没有执行者；触发路径见 [M3/P2 移交](handoffs/M3-P2-visibility.md)第 1 项（新建、改开放程度、加与恢复成员、移除、离开、接管、邀请加入、角色跨过访客、成员关系结束与恢复）。组合根现在没有注册者。
  - notebook 的删除订阅者：`{WorkspaceID, NotebookIDs, By, At}`，来自删笔记本、删无主笔记本、删工作区（工作区有笔记本时）。
- **可见性**：workspace 的根只导出单项的 `Memberships.RoleOf`；notebook 的根只导出单项的 `Facts`。列表的查询（`ListWorkspacesOf`、`ListNotebooks(ws, user, reached)`）在各自模块里面。`bootstrap/notebook_visibility_test.go` 经 HTTP 核对"列表等于逐项判定"。
- **契约测试**：`apitest.CheckResponse` 与 `bootstrap` 的 `sendRequest` 都读答复到结束；`Operation` 不认 `x-long-lived`。
- **规则表**只有工作区级与笔记本级，没有"任何登录的账户"一级。

## 2. 目标与范围

**目标**：一条 `GET /api/v0/events` 的事件流。页面的写、会话的开启与结束经 NOTIFY 推给看得到这本笔记本的连接；可见性变化、笔记本删除、凭证到期或失效、LISTEN 重连、慢客户端都以 `reset` 结束连接，客户端重连并整体刷新。

**做**：

- 平台：`postgres.Notify`、`postgres.Listener`；`API.LongLived` 与心跳时的重新认证；认证结果带凭证的到期时刻；`apitest` 认 `x-long-lived`；配置 `events.heartbeat_interval`。
- identity：认证带到期时刻。
- workspace、notebook 的根各导出一个只读的列表端口。
- 新模块 events：事件与载荷、hub、连接的建立、发布者、流的处理器、契约。
- 组合根：四个注册者（页面的观察者、会话的订阅者、可见性与删除的订阅者），Listener 在启动与停机中的位置。
- `not_ready` 的中英文案；改 `page/app/extension.go` 里"一次通知"的注释；README 的事件流一节、反向代理的配置；总体设计 3.11 与模块清单的修订。
- 测试：见第 5 节。e2e：C7、C8 的接口版本。

**不做**：前端（P3）；任务项的勾选（P6 自己补它的最后一跳）；多实例的验证（M12）。

## 3. 设计

### 3.1 文件

```
server/
  internal/platform/postgres/notify.go、listener.go 与测试        3.2
  internal/platform/httpserver/api.go、longlived.go、credential.go 与测试
                                                                 API.LongLived、Reauthenticate、凭证的到期（3.3）
  internal/platform/httpserver/apitest/operations.go、apitest.go   x-long-lived（3.9）
  internal/platform/config/config.go、validate.go；configs/config.yaml
                                                                 events.heartbeat_interval（3.10）
  internal/modules/identity/app/authenticate.go、adapter/authn/     认证带到期时刻（3.3）
  internal/modules/workspace/memberships.go                      WorkspacesOf（3.7）
  internal/modules/notebook/visible.go                           VisibleNotebooks（3.7）
  internal/modules/events/
    domain/event.go、payload.go、frame.go                         事件、载荷与它的退化、帧（3.4）
    app/ports.go                                                 Visibility、Notifier
    app/hub.go、stream.go                                        hub 与连接（3.5）
    app/open_stream.go                                           连接的建立（3.5）
    app/publisher.go                                             发布：pages、lock、access、notebooks_deleted 与别的 M 的类型（3.6）
    adapter/postgres/notifier.go                                 Notifier 经 postgres.Notify
    adapter/http/handler.go                                      流的处理器（3.8）
    module.go、publisher.go                                      New(Deps)；NewPublisher()
  internal/bootstrap/registrants.go、deps.go、wire.go、app.go、events_registrants.go
                                                                 注册与适配、Listener 的启动与停机（3.11）
api/modules/events.yaml、api/openapi.yaml                         3.9
Makefile                                                         API_MODULES 去掉 events（3.9）
web/apps/web/src/i18n/messages/*.ts                              not_ready 的文案
README.md                                                        事件流一节、反向代理
docs/v0.1/v0.1-design.md                                         3.11 的心跳、模块清单
e2e/fixtures/events.ts、e2e/stories/collab/c7-*.spec.ts、c8-*.spec.ts
```

### 3.2 平台：NOTIFY 与 LISTEN

**`postgres.Notify(ctx, channel, payload)`**：在 ctx 的事务里执行 `SELECT pg_notify($1, $2)`。ctx 不带事务就报错：事务之外的 NOTIFY 立即送出，会在写入提交之前到达。载荷超过 7999 字节也报错（PostgreSQL 的上限是 8000 字节，含结尾的零），由调用方先退化（3.4）。

**`postgres.Listener`**（`NewListener(pool, channel, logger, opts)`）：

- `Run(ctx)`（不返回错误，一直重连到 ctx 结束）：从池里取一条连接再 `Hijack`（照 River，不占 `database.max_conns`），`LISTEN`，然后 `WaitForNotification` 循环，每条通知交给 `OnNotify(payload)`；
- 安静 `PingInterval`（默认 30 秒）没有通知就 `Ping`（至多 5 秒），失败按断开处理：悄悄断掉的连接（数据库切换、NAT 忘了它）由此发现，不必等 TCP keepalive 的约 150 秒。超时的等待不关连接，`Ping` 期间到的通知由 pgx 缓存（审查 A-m2）；
- 连接出错时关掉它，按退避（初始 100 毫秒，翻倍，上限 5 秒）重连；重连成功、`LISTEN` 生效之后先调 `OnListening(true)`，再调 `OnReconnect()`；断开时调 `OnListening(false)`；
- `ctx` 取消时关掉连接（至多等 2 秒）返回。`Run` 返回之后连接一定已经关闭（`pgtest.NewDatabaseFrom` 要求模板库上没有别的连接）；
- 只用 `time`，不用 `platform/clock`；退避的间隔经选项注入，测试用短的。

### 3.3 平台与 identity：长连接、到期与重新认证

- **到期**：`httpserver.WithCredentialExpiry(ctx, at)` 与 `CredentialExpiry(ctx) (time.Time, bool)`。identity 的认证用例改为返回到期时刻（访问令牌取它的 `exp`；PAT 取 `expires_at`，没有就不到期），`authn` 把它放进 context。`Authenticator` 接口不变，测试的假认证不必改：没有到期时刻就是不到期。
- **`API.LongLived(h)`**：请求信息 → 在 `server.request_timeout` 之内：失败闸门与认证 → 按凭证限流 → 包级 `LongLived`（解除写期限放在最后：整体测试用 `ResponseRecorder` 调用它，在那里解除会失败，而 401、429 要先答出来）。处理器得到认证放进 context 的值，不带这个期限，取消跟着请求自己的（审查 F-M2）；没有请求体。
- **重新认证**：`API.LongLived` 记下这次请求的 bearer 令牌，在 context 里放一个 `Reauthenticate(ctx) error`：用同一个令牌再调一次 `Authenticator.Authenticate`，不经失败闸门（这个凭证已经认证过一次），不计限流。PAT 的 `last_used_at` 照旧至多一分钟写一次。
- **`X-Accel-Buffering: no`** 由流的处理器设置。

### 3.4 events 的领域：事件、载荷、帧

- **`Event{Type, WorkspaceID, NotebookID, Data}`**：`NotebookID` 为零是工作区级的事件；`Data` 是只含 id 的 JSON。别的 M 经发布者加自己的类型（总设计第 8 节），hub 不认识的类型也照转。
- **本 Phase 的类型**：

  | 类型 | 数据 | 按什么过滤 |
  |---|---|---|
  | `pages` | `{tree, pages: [{id, revision}]}`：`tree` 是否改了树；写了正文的页与新的 revision，没有是 `[]`，多于 20 页是 `null` | 笔记本 |
  | `lock` | `{page_id, session_id}`：会话 id 让同一事务里的"结束"与"开启"不被 PostgreSQL 合并 | 笔记本 |
  | `access` | `{user_ids, reached}`：`user_ids` 超出载荷时是 `null`，按 `reached` 处理 | 见 3.5 |
  | `notebooks_deleted` | `{notebook_ids}`：超出载荷时是 `null`，整个工作区一起 | 见 3.5 |

- **载荷**：`{"type","workspace_id","notebook_id"?,"data"}` 的 JSON，不超过 7999 字节。`Encode` 先照原样编码，超出时按类型退化（`pages` 去掉 `pages`，`access` 去掉 `user_ids` 并置 `reached`，`notebooks_deleted` 去掉 `notebook_ids`）；别的 M 的类型超出就报错（数据只含 id，超出是它的 bug）。`Decode` 读不懂的载荷记一条警告、丢掉。
- **帧**（SSE）：`event: <类型>\ndata: <JSON>\n\n`，`data` 是 `{"workspace_id","notebook_id"?, …数据}`；`hello` 的数据是 `{"heartbeat_seconds"}`；`reset` 的数据是 `{"reason"}`；心跳是一行注释 `: heartbeat\n\n`。
- **`reset` 的原因**：`access`、`notebooks_deleted`、`expired`（凭证到期）、`unauthenticated`（心跳时重新认证失败）、`reconnected`（LISTEN 重连）、`overflow`（缓冲满了）。

### 3.5 hub 与连接

**hub**（每个进程一个，`app.Hub`）：

- `Dispatch(Event)`：Listener 每收到一条就调一次（解码之后），对每个连接：
  - `access`：连接的账户在 `user_ids` 里，或 `reached`（或省去了 `user_ids`）而连接看得到这个工作区：`reset(access)`；
  - `notebooks_deleted`：连接看得到其中一本，或省去了 `notebook_ids` 而看得到这个工作区：`reset(notebooks_deleted)`；
  - 其余：带笔记本 id 的，连接看得到这本笔记本才送；只带工作区 id 的，看得到这个工作区才送。
- `ResetAll(reason)`：LISTEN 重连之后，对每个连接 `reset(reconnected)`。
- `Listening(bool)`：没在监听时，新的连接答 503 `not_ready`（`Retry-After: 1`）：不让一条连接漏掉它订阅之前的事件。
- 读写连接集合用一把互斥锁；送事件不阻塞：每个连接一个容量 64 的缓冲（`BufferSize`），满了就记下 `overflow`、不再收，由连接自己送出 `reset` 后关闭。慢的客户端不拖住别人，也不悄悄丢事件。建立中的连接还不知道看得到什么，整个服务器的事件都排着，另有 1024 条（`PendingSize`，审查 A-m3）。

**连接的建立**（`OpenStream.Execute(ctx)`，在 `server.request_timeout` 之内，审查 B-m4）：

1. 没在监听：`not_ready`。
2. 在 hub 登记（这时还没有可见集合，事件先排着）。
3. 不加锁地算可见集合：`Visibility.WorkspacesOf(user)`，再对每个工作区 `Visibility.NotebooksIn(ws, user, role)`。
4. 交出集合：排着的事件里有涉及本账户或这些工作区的 `access`、`notebooks_deleted`，就直接 `reset`；其余按集合过滤后放进缓冲。
5. 计算出错：注销，返回错误（500）。

先登记后计算：先算后登记会有一个窗口，可见性在两步之间变化，这条连接就带着旧的集合活下去。

### 3.6 发布与注册

- **端口**：`app.Notifier.Notify(ctx, payload)`，`adapter/postgres` 经 `postgres.Notify(ctx, "nwiki_events", …)` 实现，必须在事务里。通道名不与 River 的（`river_*`）冲突。
- **`Publisher`**（模块根 `events.NewPublisher()`，只建 Notifier，用 ctx 里的事务，不碰 hub、HTTP、任务，CLI 的组合也能建它）：
  - `Publish(ctx, Event)`：别的 M 的入口（总设计第 8 节）；
  - `PagesWritten(ctx, PagesWritten{WorkspaceID, NotebookID, Changes []PageChange{PageID, Tree, Revision}})`：`tree` 是任一变化改了树，`pages` 是 `Revision` 不为零的；
  - `LockChanged(ctx, LockChanged{WorkspaceID, NotebookID, PageID, SessionID})`；
  - `AccessChanged(ctx, AccessChanged{WorkspaceID, UserIDs, Reached})`；
  - `NotebooksDeleted(ctx, NotebooksDeleted{WorkspaceID, NotebookIDs})`。
- **一个事务几条**：照总体设计 3.11 的修订（总设计 4.10），不合并。`page/app/extension.go` 的注释改为"事件流发一条 pages 事件，此外单元开启或结束的每个会话各一条 lock 事件"。
- **次序**：删除的订阅者先是页面的部分（结束会话，发 `lock`），再是事件流（`notebooks_deleted`）。
- **注册**（组合根的适配，模块之间不互相导入）：
  - `pageEvents`：实现 `page.PageObserver`（`Change.Moves()` 给 `Tree`）与 `page.EditSessionSubscriber`（开启与结束都发 `lock`）；
  - `notebookEvents`：实现 notebook 的可见性变化订阅者与删除订阅者；
  - `pageRegistrants(pool)` 加观察者与会话订阅者，`page.Deps` 与 `notebookRegistrants(pool)` 都从它取（总设计第 8 节的两处接线）；`notebookRegistrants(pool)` 加可见性与删除的订阅者，workspace 的组合与 CLI 的组合都经它。

### 3.7 可见的笔记本：两个端口

- workspace 的根：`Memberships.WorkspacesOf(ctx, userID) ([]Membership{WorkspaceID, Role}, error)`，用已有的列表查询（停用的成员不在里面）。
- notebook 的根：`VisibleNotebooks.VisibleIn(ctx, workspaceID, userID, role) ([]uuid.UUID, error)`（`NewVisibleNotebooks(pool)`），用已有的 `ListNotebooks` 与 `shared.ReachedByAccess(role)`。
- 组合根把两者接成 events 的 `Visibility`。
- 测试（总体设计 13.1 第 3 条）：权限矩阵种子上的每一列，`VisibleIn` 的结果等于对这个工作区的每本笔记本逐项判定 `page.read` 的结果；`WorkspacesOf` 等于逐个 `RoleOf`。

### 3.8 流的处理器

`GET /api/v0/events` 经 `API.LongLived` 挂在 `router.Handle` 上（不经生成的代码）：

1. `OpenStream.Execute`（在 `server.request_timeout` 之内）：`not_ready` 或出错时照常答 problem。
2. 答复头：`Content-Type: text/event-stream`、`X-Accel-Buffering: no`（`Cache-Control: no-store` 平台已加）；写 `hello`，`Flush`。
3. 循环，直到 context 结束（客户端走了、停机）：
   - 缓冲里的帧：写出、`Flush`；连接被标为 `reset`：先写出缓冲里还有的事件，再写 `reset` 帧，结束：帧按事件的次序，`reset` 在最后；
   - 心跳的计时器（`events.heartbeat_interval`）：`Reauthenticate`，401 就 `reset(unauthenticated)`，别的错误不带帧结束（客户端重连），请求的 context 已结束就安静返回；成功就写一行注释；
   - 凭证到期的计时器：`reset(expired)`。凭证到期或失效时不写出缓冲里的事件。
4. 结束时在 hub 注销。每一帧在一个心跳之内写出，写完解除期限：写超时或失败就结束（客户端走了，或停止了读取，审查 A-I1、F-M1）。

时刻取模块的 `Clock`（`Now`）；计时器经 `Deps.After`（`func(time.Duration) <-chan time.Time`，默认 `time.After`）：到期的计时器是 `After(到期时刻 − Now())`。测试注入假的，自己推进。

### 3.9 契约与生成

- `api/modules/events.yaml`：`streamEvents`（`x-long-lived: true`，`x-problem-codes: [not_ready]`），200 的 `text/event-stream`；描述写明帧、事件类型、`reset` 的原因、心跳与到期、不带 `reset` 的结束。帧的数据各有 schema（`EventHello`、`EventPages`、`EventLock`、`EventReset`），放在 200 的 schema 的 `oneOf` 里（Redocly 丢掉没被引用的组件），处理器的测试用 `CheckSchema` 核对每种帧，P3 用生成的 TS 类型。每个模块的 Problem 答复的 `Retry-After` 都写上 `not_ready`（各模块的组件必须相同）。
- `api/openapi.yaml` 加路径与 `events` 标签。
- Makefile：`API_MODULES` 去掉 `events`：它唯一的操作不生成代码，生成的包会是空的（已验证：`exclude-operation-ids` 排除唯一的操作也能编译，但只留下用不到的类型）。
- `apitest`：`Operation.LongLived` 读 `x-long-lived`；`CheckResponse` 对它只核对状态与答复头（200 时 `Content-Type`），不读到结束。`bootstrap` 的 `sendRequest` 同样只读答复头就关闭。"每个操作都接受 PAT"与"公开的操作"两项测试随之。
- `events/adapter/http` 的测试用 `apitest.Main`：`not_ready` 由测试答出。

### 3.10 配置

`events.heartbeat_interval`：默认 20 秒，校验 5–50 秒（短于 nginx 的 `proxy_read_timeout` 默认的 60 秒），照 `page.edit_session_cleanup_interval` 的写法（`Config`、`LogValue`、`validate`、`config.yaml`、测试的配置）。环境变量 `NWIKI_EVENTS__HEARTBEAT_INTERVAL`。

### 3.11 组合根：启动与停机

- `newApp`：建 `postgres.Listener`（不连接）与 events 模块；Listener 的三个回调接到 hub（`OnNotify` 解码后 `Dispatch`，`OnReconnect` 是 `ResetAll(reconnected)`，`OnListening` 是 `Listening`）。
- `serve`：检查数据库 → 后台任务 → **Listener 启动**（不等它生效：生效之前流答 503）→ HTTP 开始服务 → 停机：HTTP（`LongLived` 的 context 在停机开始时取消）→ **Listener 停止**（等 `Run` 返回）→ 后台任务 → 连接池。`TestServeRunsUntilCancelled` 的日志次序加 Listener 的两行。
- 连接数：README 改为 `max_conns + 2`（River 与 Listener 各一条）。停机的总时长加上 Listener 关连接的至多 2 秒：默认 38 秒，`docker stop -t 40`。

### 3.12 权限

`streamEvents` 只要认证：事件按可见性过滤，不按资源判定。所以不加动作、不进规则表（总设计 4.10 原写"`events.stream` 在规则表里有一行"，规则表没有"任何登录的账户"一级，为一个操作加一级不值得；修订总设计）。权限矩阵整模块豁免 `events`，理由写"按可见性过滤，没有资源的判定"。

### 3.13 端到端（接口版本）

夹具 `e2e/fixtures/events.ts`：用 Node 的 `fetch` 开流，逐帧解析，`next(type)` 等下一帧（带超时），`close()`。

- **C7**：A、B 在笔记本 Eng；C 在工作区里但看不到 Eng。B、C 各开一条流。A（PAT）新建、改名、移动、写正文、删除一页：B 依次收到 `pages`（`tree` 与 `pages` 的 revision 对得上），C 什么也收不到（C 的流在一个对照写之后收到的第一帧是它自己笔记本的事件）。A 开启、结束会话：B 收到两次 `lock`。
- **C8**：
  - 短的访问令牌（`nervewikiWith`）：流在到期时以 `reset expired` 结束；
  - B 被移出 Eng：`reset access`，重连之后收不到 Eng 的事件；
  - B 退出登录：一个心跳之内（短的心跳间隔）`reset unauthenticated`；
  - Eng 被删除：`reset notebooks_deleted`；
  - 没有令牌：401。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 平台：NOTIFY、Listener、长连接与重新认证、凭证的到期、`x-long-lived`、配置 | [P2-S1](plans/P2-S1-platform.md) |
| S2 | events 的领域与 app：事件与载荷、hub、连接的建立、发布者 | [P2-S2](plans/P2-S2-events-core.md) |
| S3 | 可见的端口、events 的适配器与模块根、契约、文案、矩阵豁免 | [P2-S3](plans/P2-S3-adapters-contract.md) |
| S4 | 组合根：注册、Listener 的启动与停机、最后一跳、README、反向代理的验证 | [P2-S4](plans/P2-S4-wiring-whole.md) |
| S5 | 端到端 | [P2-S5](plans/P2-S5-e2e.md) |

## 5. 测试与验证

- **平台**：
  - `Notify`：事务里提交才送达、回滚不送；不在事务里报错；超长报错；
  - `Listener`（测试数据库）：收到通知；`pg_terminate_backend` 断开它的连接之后重连，先 `OnListening(true)` 再 `OnReconnect`，之后的通知照收；`ctx` 取消后连接关闭；
  - `API.LongLived`：未认证 401、失败闸门与限流照常；写期限解除；`Reauthenticate` 不经闸门；凭证的到期在 context 里；
  - identity：访问令牌与 PAT 的到期时刻（PAT 没有就不到期）。
- **events**（假的 Visibility、Notifier、时钟与连接）：
  - 载荷：每种类型的编码与解码，各自在 7999 字节处退化；
  - hub 的过滤（按笔记本、按工作区、不认识的类型）；`access` 与 `notebooks_deleted` 的四种情形（在 `user_ids` 里、`reached`、省去、看不到的工作区不受影响）；缓冲满了 `reset overflow`；`ResetAll`；没在监听时 503；
  - 连接的建立：登记与计算之间到达的 `access`（涉及本账户、涉及这些工作区、无关）与普通事件（按集合过滤后送出）；
  - 发布者：`tree` 与 `pages`（多于 20 页省去）、`lock` 带会话 id、`access`、`notebooks_deleted`。
- **处理器**（`httptest` 的真实服务与假时钟）：答复头、`hello`、事件帧、心跳、心跳时重新认证失败、到期、停机时结束；每种帧经 `CheckSchema`。
- **可见的端口**：3.7。
- **整个程序**（`bootstrap`，开一条真实的流，做写，读到事件或 `reset`）：
  - 观察者的每条路径：新建（带与不带正文）、改名、移动、删除、写正文（带与不带会话）各收到 `pages`；不触发的：开启、心跳、结束、相同的保存、笔记本删除（[M4/P1 移交](handoffs/M4-P1-tree-refresh.md)第 4 项）；
  - 会话订阅者的每条路径：开启、本人结束、接管、强制解锁、删页与删子树、笔记本删除的三条（[M4/P4 移交](handoffs/M4-P4-edit-sessions.md)第 8 项）；过期不告诉；
  - 可见性的每条路径（[M3/P2 移交](handoffs/M3-P2-visibility.md)第 1 项）：表格驱动，每行一个触发，核对谁的流被 `reset`；
  - 笔记本删除的三条路径；
  - 组合根交空时以上失败；
  - "谁收得到 lab 的事件"：权限矩阵种子的笔记本级 13 列各开一条流，在 lab 写一次，收到的列等于能读 lab 的列；
  - Listener 的重连：断开它的连接，开着的流收到 `reset reconnected`；
  - `serve` 的启动与停机次序。
- **e2e**：3.13。
- **反向代理**：作者用 Caddy 与 nginx 的容器各验证一次（帧不被缓冲、心跳让连接活过 `proxy_read_timeout`），配置写进 README（[M0/P1 移交](handoffs/M0-P1-sse-proxies.md)第 1、2 项）。
- **反向对照**：
  - `Notify` 不要求事务；
  - Listener 重连之后不调 `OnReconnect`；
  - `LongLived` 不经认证；`Reauthenticate` 经失败闸门；
  - hub 不按笔记本过滤；`access` 不看 `reached`；缓冲满了丢事件而不 `reset`；
  - 先计算后登记（登记与计算之间的 `access` 漏掉）；
  - 心跳不重新认证；不设到期的计时器；
  - `pages` 不带 `tree`；`lock` 不带会话 id；
  - 组合根不注册某一个注册者（四个各一次）。

## 6. 完成标准

- `GOFLAGS=-p=3 make check`、`make gen-check`、`make e2e` 为绿；上面的反向对照都按预期失败。
- Opus 审查与修复；修复的差异较大时另经核对。
- 持续集成为绿；`--no-ff` 合并；`make image-smoke`；文档提交（本文的结果一节、总设计第 12 节）。

## 7. 结果

- 分支 `m5-p2`：S1 `7c86df2`；S2 `3ec34ee`；S3 `33abbdd`；S4 `6016bee`；S5 `371503d`；审查修复 `514c792`、核对之后的修复 `5e8574c`；`f0b3f6c` 合并（`--no-ff`）。
- 门禁：每个 Step 与两轮修复的 `make check` 为绿；`make gen-check`、`make e2e`（163 个）、`make image-smoke` 为绿；整个程序的事件测试 `-race -count=5` 为绿；持续集成为绿。
- 审查：[P2 审查](reviews/P2-event-stream-review.md)。两位审查者，没有阻断合并的问题。Important 1：A-I1（停止读取的客户端让处理器的写永远阻塞：心跳、重新认证、到期都停了，停机等满 `server.shutdown_timeout`）；Minor 9，合并之前全部处置或记下。修复的核对没有 Important，Minor 2、Nit 5 一并处置。
- 反向对照：S1 13、S2 11、S3 10、S4 8、S5（e2e）4，审查修复 7、核对之后的修复 5（其中一项让测试挂住而不是失败），都没有通过。
- 反向代理（S4，自己起的容器，服务在本机，心跳 20 秒，读 78 秒）：`caddy:2.10-alpine` 只写 `reverse_proxy`，与 `nginx:1.29-alpine`（`proxy_http_version 1.1`、`Connection ""`、`proxy_read_timeout 30s`、`proxy_buffering` 默认开，由答复的 `X-Accel-Buffering: no` 关掉）结果相同：`hello` 在 3 毫秒内到达，`pages` 帧与写入的答复在同一毫秒，心跳在 20、40、60 秒，连接活过 78 秒，75 秒的写照常到达。对照：`proxy_read_timeout 10s` 的 nginx 在最后一帧之后 10 秒断开。Caddy 开 `encode zstd gzip`（[M0/P1 移交](handoffs/M0-P1-sse-proxies.md)第 1 项，合并之后补测）：客户端声明 gzip、zstd 或什么也不声明，答复都没有 `Content-Encoding`，帧同样即时到达，不必排除事件流。

**与计划的出入**（已同步进上文）：

1. `Listener.Run` 不返回错误：它一直重连到 ctx 结束（3.2）。
2. events 模块与 Listener 的接线从 S4 挪到 S3：契约的操作要在 bootstrap 的契约测试里有路由（3.11）。
3. 端口：workspace 的 `Memberships` 加 `WorkspacesOf`（根类型 `Membership`）；notebook 另有 `VisibleNotebooks.VisibleIn`（`NewVisibleNotebooks(pool)`），不放在 `Notebooks` 上（3.7）。
4. 帧的 schema 放在 200 的 `text/event-stream` 的 `oneOf` 里：Redocly 丢掉没被引用的组件，`x-` 扩展里的引用会被内联（3.9）。
5. 每个模块的 Problem 答复的 `Retry-After` 都写上 `not_ready`：各模块的组件必须相同（3.9）。
6. `events.NewPublisher()` 不带连接池：Notifier 用 ctx 里的事务（3.6）。
7. 模块根给出 `Notified`、`Listening`、`Reconnected`、`Streams`，不交出 hub（3.11）。
8. `pages` 的数据：没有写正文是 `[]`，多于 20 页是 `null`（不是省去）；退化时 `access` 的 `user_ids`、`notebooks_deleted` 的 `notebook_ids` 是 `null`（3.4）。
9. 心跳时认证服务出错（不是 401）：流不带帧就结束，客户端重连（3.8）。
10. 因 `access`、`notebooks_deleted`、`reconnected`、`overflow` 而 `reset` 时，先写出缓冲里已有的事件：帧按事件的次序，`reset` 在最后。原来 `select` 在两者之间随机挑，`reset` 可能越过缓冲里的事件（S4 的 lab 测试发现：3 个事件只到 1 个）。凭证到期或失效时不写出（3.8）。
11. 审查修复：每一帧在一个心跳之内写出，写完解除期限（A-I1、F-M1）；Listener 安静 `PingInterval`（30 秒）就 `Ping`（A-m2，总设计第 6 节"照 River 的 notifier"原本就这样写）；建立中的连接另有 1024 条的队列（A-m3）；连接的建立（认证与读可见集合）受 `server.request_timeout` 约束（B-m4、F-M2）；心跳被客户端的离开打断不记 ERROR（A-m4）；整个程序的测试与 e2e 在开流之前等到之前的通知都已分发（B-m1）（3.2、3.3、3.5、3.8）。

**留给后面的**：

- **给加工作区级事件类型的 M**：流的工作区集合在建立时算好；访客接受邀请、恢复成访客而没有归还的笔记本、新建工作区都不发 `access`，所以这些工作区级的事件到不了，直到别的原因让流重连（审查 B-m3）。P2 的类型都按笔记本过滤，不受影响；加工作区级类型的 M 要同时让这些路径发 `access`。
- **给 P3**：流也会不带 `reset` 就结束（停机、心跳时认证服务出错、一个心跳之内收不下一帧），客户端在任何结束之后都重连并刷新；打开时的 503 `not_ready` 按 `Retry-After` 重试。`/readyz` 不看接收通知的连接。
- **接受**：整个程序的"每个操作都接受 PAT"对流只要求不是 401（503 也算通过）；e2e C7 用 PAT 开流得到 200，补上了。
