# M5/P3 事件流与实时刷新（前端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M5/P3 事件流与实时刷新（前端） |
| 状态 | 进行中 |
| 基线 | `349d30f`（P2 合并与它的文档提交之后的 main）；本文与各 Step 计划提交之后开分支 `m5-p3` |
| 上级文档 | [M5 总设计](00-M5-design.md) 第 3 节（C7–C9）、4.11、第 9 节；[P2 文档](02-P2-event-stream.md) 第 7 节"给 P3"；[M0/P1 移交](handoffs/M0-P1-sse-proxies.md) 第 3 项；[总体设计](../v0.1-design.md) 3.11、9.4 |

---

## 1. 基线

前端的调研（作者，2026-10-03），路径在 `web/apps/web/src` 之下：

- **换代**：`SessionRoot` 在 `useMemo` 里按 `loginId` 建 `RootStore`，`<AppProviders key={loginId}>` 随换代重挂，SWR 的缓存每代一个（`provider: () => new Map()`）。`RootStore` 没有拆除；`main.tsx` 开着 `StrictMode`（开发时 `useMemo` 与 effect 各跑两次）。`Session` 的 `locks`、`storage`、`onStorage` 是私有的依赖，`RootStore` 碰不到。
- **接口**：服务经 `unwrap(await this.api.GET(…))`，错误是 `ApiError(status, problem, retryAfter)`。认证中间件在请求与答复头两处核对换代，401 时续期一次、再 401 就结束会话；答复体之后的读取不受保护。没有人用过 `parseAs: "stream"` 与 `AbortSignal`。契约已有 `streamEvents`、`getEditLock`、`releaseEditLock` 与它们的类型，`PageService` 还没有锁的方法。
- **SWR**：键 `"workspaces"`、`["notebooks", workspaceId]`、`["pages", notebookId]`（取数的是 `PageTreeStore.load`）、`["page-view", pageId]`（`{html, revision}`）。没有锁的键。`mutate(key)` 只让挂着的 hook 重读；重读期间旧数据留着。
- **页面树**：`PageTreeStore.load` 按"读开始之后有写答复就丢掉这次读"的规则对写排序，两次重叠的读按答复的先后覆盖。
- **页面**：`PageShell` 的 `editing` 状态在阅读视图与 `PageEdit` 之间切换：编辑时阅读视图卸载。没有"本标签页在编辑哪一页"的共享记录。
- **浏览器接口**：应用里没有 `BroadcastChannel`、`pagehide`、`freeze`；`navigator.locks` 只在会话的续期锁里，不带选项。M1 的 `leaseLock` 是一次性的短锁（固定的键、10 秒），不能当持有者的租约。
- **测试**：vitest（jsdom，没有 `navigator.locks`，有 Node 的 `BroadcastChannel`、`Response`、`ReadableStream`）；`signedInApp` 的 `byRoute` 对未列出的路由答 404；会话的测试有跨标签页的替身（`SharedStorage`、`RecordingLock`、`FakeServer`），`describe.each` 两种锁。e2e 有 `anotherPage`（另一个上下文）与 `openEvents`（Node 读流），同一上下文的第二个标签页由故事自己开。

## 2. 目标与范围

**目标**：一个浏览器一条事件流。页面树、阅读视图与"某某正在编辑"随别人的写入与锁的变化更新；流断开、`reset`、换持有者之后整体刷新；关掉持有连接的标签页，别的标签页接手。

**做**：

- 服务：`EventService.open(signal)`；`PageService` 的 `lock`、`releaseLock`。
- `src/events/`：帧的解析、一条连接、持有者的选举（Web Locks 与租约两种）、标签页之间的频道、`EventHub`、正文重读的合并；`.oxlintrc.json` 的目录规则。
- 接线：`AppStores` 带浏览器的依赖；`RootStore.events()`；`<EventStream/>` 把事件接到这一代的 SWR 与 store。
- 阅读视图的"某某正在编辑"与管理员的"解除锁定"；阅读视图的键加笔记本 id；`PageTreeStore` 的读按开始的次序生效。
- 文案；e2e：C7、C8、C9 的页面版本，同一上下文第二个标签页的夹具，已有故事里的请求清单不算事件流。

**不做**：编辑锁的前端（P4：先拿锁再编辑、失锁只读、接管）；自动保存与闲置（P5）；任务项（P6）；多实例（M12）。

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  services/event.service.ts                     EventService.open（3.2）
  services/page.service.ts                      lock、releaseLock（3.2）
  events/frames.ts                              帧的解析（3.3）
  events/connection.ts                          一条连接：读帧，可中止（3.3）
  events/leadership.ts                          持有者：Web Locks 与租约（3.4）
  events/channel.ts                             标签页之间的消息（3.5）
  events/hub.ts                                 EventHub（3.6）
  events/refresher.ts                           正文重读的合并（3.8）
  events/deps.ts                                EventDeps 与 browserEventDeps（3.7）
  stores/root.store.ts                          AppStores 带 EventDeps；RootStore.events()（3.7）
  stores/page-tree.store.ts                     读按开始的次序生效（3.9）
  app/event-stream.tsx、app/providers.tsx       <EventStream/> 与事件的路由（3.8）
  pages/page/edit-lock-note.tsx、page-layout.tsx  正在编辑与解除锁定（3.10）
  pages/page/reading-view.tsx、page-edit.tsx    阅读视图的键（3.8）
  i18n/messages/en.ts、zh-CN.ts                 文案
  main.tsx                                      浏览器的 EventDeps
.oxlintrc.json                                  src/events 的目录规则（3.11）
e2e/fixtures/test.ts、browser.ts、events.ts       同一上下文的标签页、事件流的计数（3.12）
e2e/stories/collab/c7-push.spec.ts、c8-stream-life.spec.ts、c9-one-stream.spec.ts
```

### 3.2 服务

- **`EventService.open(signal)`**：`GET /api/v0/events`，`parseAs: "stream"`，带 `signal`；不是 200 时照 `unwrap` 抛 `ApiError`（503 `not_ready` 带 `retryAfter`），是 200 时返回 `ReadableStream<Uint8Array>`。经这一代的客户端，所以认证中间件照常核对换代、401 时续期一次；答复体的读取不受它保护，换代时由 hub 中止（3.6）。
- **`PageService.lock(id)`**：`GET /api/v0/pages/{id}/edit-lock`，`EditLock{holder, expires_in}`；**`releaseLock(id)`**：`DELETE` 同一地址。

### 3.3 帧与连接

- **`parseFrames(chunks)`**：把字节流按行解析成帧：`event:` 与 `data:` 组成一帧（空行结束），`data` 解析成 JSON；注释行（`: heartbeat`）是心跳。产出 `{type: "hello" | "pages" | "lock" | "reset", data}`、`{type: "beat"}`，不认识的事件类型产出 `{type: "other", event, data}`，由调用者跳过。解析失败的一帧丢掉（不终止流）。
- **`connect(open, signal, onFrame)`**：调 `open(signal)`，逐帧交给 `onFrame`，返回流是怎样结束的：`{ended: "reset", reason}`、`{ended: "closed"}`（没有 `reset` 的结束）、`{ended: "failed", error}`（开流或读取出错，`ApiError` 带状态与 `retryAfter`）、`{ended: "aborted"}`（`signal` 中止，或 `SessionChangedError`：换代了）。

### 3.4 持有者的选举

`Leadership` 接口：`run(lead: (lost: AbortSignal) => Promise<void>, signal)`：成为持有者时调 `lead`，`lost` 在被抢、让出或 `run` 结束时中止；`lead` 返回就放手。另有 `steal()`（让出期间不接受）、`yield()`、`rejoin()`（重新加入，忘掉之前要的 `steal`）与 `renew(ttl)`（持有者的存活，租约的到期；Web Locks 下不做事）。两种实现，键都是 `nwiki.events.<loginId>`：

- **Web Locks**（`navigator.locks` 在时，安全上下文与 localhost）：
  - `request(name, {signal}, cb)`：排队，前一个放手时浏览器把锁给下一个；`cb` 里调 `onLead`，持有到它结束；
  - `steal()`：`request(name, {steal: true}, cb)`：原持有者的 `request` 以 `AbortError` 失败，它中止 `lost`、重新排队；
  - `yield()`：结束 `cb`，放手；`rejoin()` 时重新排队。
- **租约**（没有 `navigator.locks`，如局域网地址上的 HTTP）：
  - 存储里的 `{tab, until}`；持有者每收到一帧（事件或心跳）、重连期间每个心跳间隔，就把 `until` 续到"现在 + 心跳间隔的 3 倍"：网络任务不受后台计时器节流，后台的持有者照样续得上；
  - 取得：键不在或已过期时写入自己，等 50 毫秒，读回是自己才算取得（几个标签页同时写，只有最后一个读回是自己）；等待中要的 `steal` 由这次等待回答，无论谁取得；存储拒绝写入（已满、被禁）算没取得，过一个租约的长度或被唤醒时再看；
  - 别的标签页在存储变化（`onStorage`）或租约该到期时检查；
  - `steal()` 照取得做，不看 `until`；`yield()` 与 `run` 结束时立刻删掉自己的键，不等 `lead` 返回：离开的页面未必等得到；
  - 被冻结的持有者不续期，它的租约过期，别的标签页取得；它解冻之后发现键不是自己的（下一次续期时读回，每次连接之前与每一帧都续期），中止 `lost`，那一帧不转发。

**谁抢**：Web Locks 下，看得见的标签页在心跳间隔的 3 倍之内没听到持有者的任何消息（事件、心跳、"正在重连"），就 `steal()`；被冻结的持有者不放锁（Nerve 的 `refresh-lock.ts` 记下过这一点）。隐藏的标签页不抢：它们的计时器被节流，重新可见时再看。租约下过期就是这个条件，任一标签页都可以取得。

**让出**：持有者在 `pagehide`、`freeze` 时 `yield()`，中止它的连接；`pageshow`、`resume` 时重新加入，沉默从这时算起：冻结期间没听到的心跳可能在页面显示之后才到，活着的持有者不该被抢。心跳间隔取持有者的 `hello`，经频道告诉别的标签页；没听到之前按 20 秒（服务端的默认）。

### 3.5 标签页之间的频道

`BroadcastChannel("nwiki.events")`，消息都带 `loginId`，别的一代的消息丢掉：

| 消息 | 谁发 | 收到的做什么 |
|---|---|---|
| `frame`：一帧事件 | 持有者，每帧 | 交给本标签页的订阅者；记下"听到了" |
| `beat`：心跳与间隔 | 持有者，每个心跳 | 记下"听到了"与间隔 |
| `connected` | 持有者，每次 `hello` | 本标签页整体刷新（3.8）；记下"听到了" |
| `reconnecting` | 持有者，重连期间每个心跳间隔 | 记下"听到了"：服务端重启或断网时，两个看得见的窗口不互相抢 |
| `yield` | 让出的持有者 | 记下"听到了"：接手由锁的队列（Web Locks）或删键的存储事件（租约）唤醒 |

### 3.6 EventHub

每一代一个（`RootStore.events()` 建，不启动），只经 `EventService` 与注入的依赖：

- **`start()` / `stop()`**：`start` 打开频道、挂上页面生命周期的监听、开始选举；`stop` 中止连接（`AbortController`）、放手、关闭频道、去掉监听与计时器。可以再次 `start`：`StrictMode` 下 effect 会挂、卸、再挂；再次 `start` 的选举等上一次的结束了才开始，一个标签页不会同时持有两次。
- **`subscribe(listener)`**：本标签页的事件，`{type: "pages" | "lock", data}` 与 `{type: "connected"}`（一次连上，要整体刷新），无论是自己的连接还是持有者转来的。
- **持有者的循环**（`onLead` 里，直到 `lost` 中止）：
  1. `connect`：每帧交给本标签页、发到频道、续租约；`hello` 记下间隔，发 `connected`（本标签页也收到），退避归零；
  2. 结束于 `reset`：立刻重连（`expired` 时开流前令牌照常续期；`unauthenticated` 时开流答 401，中间件续期失败就结束会话、换代，hub 随之停止）；
  3. 结束于"没有 `reset`"或出错：按退避等待（1 秒起，翻倍，上限 30 秒）再连；503 `not_ready` 按 `Retry-After`；
  4. `aborted`：放手，停止；
  5. 等待与连接期间每个心跳间隔发一次 `reconnecting`；
  6. 一条连接 3 个心跳间隔没有一帧（开流也没有答复）就放弃、按退避重连：断网没有消息的半开连接，只有一个标签页时没有跟随者来接手。
- **跟随者**：订阅频道；Web Locks 下排队等锁；记下最后一次听到持有者的时刻，看得见且超过 3 个心跳间隔就 `steal()`（重新可见时检查一次）。

**每次连上都整体刷新**，第一次也是：页面加载时的读取在流登记之前完成，其间的写的事件收不到。

### 3.7 依赖与接线

- **`EventDeps`**：`locks`（`LockManager` 的 `request`，带选项）或没有、`storage` 与 `onStorage`、`channel(name)`、`page`（可见与否，`visibilitychange`、`pagehide`、`pageshow`、`freeze`、`resume` 的订阅）、`now`、`tabId`。`browserEventDeps(storage, onStorage, tabId)` 在 `main.tsx` 里用浏览器的对象建，存储、存储事件与随机 id 与会话共用（站点数据被禁时的内存存储没有别的标签页共享，频道照常）。
- **`AppStores`** 多一个可选的 `events?: EventDeps`；`RootStore.events()` 在登录的这一代、有 `EventDeps` 时建 hub，否则 `undefined`；`RootStore.newRefresher()` 以 `EventDeps.page` 的可见与否建重读的合并。测试的 `signedInApp` 默认不带：已有的页面测试不开流、不起计时器；事件的测试自己给替身。

### 3.8 `<EventStream/>` 与刷新

放在 `AppProviders` 里、`DocumentSync` 旁边：会话 `signed-in` 且 hub 在时，effect 里 `start`、清理时 `stop`（换代时 `AppProviders` 重挂，旧 hub 随之停止）。它订阅 hub，经这一代的 `useSWRConfig()` 刷新：

- **`pages`**：
  - `tree` 为真：`mutate(["pages", notebookId])`，挂着的树（`PageTreeStore.load`）重读；没挂着的下次挂上时 SWR 自己重读；
  - 每个 `{id, revision}`：缓存里 `["page-view", notebookId, id]` 的 `revision` 小于它，或第一次读还在外（缓存有键没数据、SWR 的 `isLoading`：那次读可能是写之前的），就交给重读的合并；`pages` 是 `null` 时，这本笔记本的阅读视图按一个键交给它（挂着的才读）。
- **`lock`**：先重读这本笔记本的树、等 React 提交，再 `mutate(["edit-lock", pageId])`：删除正在编辑的页时会话先结束（`lock` 在 `pages` 之前），这一页要先离开，不读成 404。
- **`connected`**：由外向内，一层读完、React 提交之后再读下一层：`"workspaces"`，`["notebooks", …]`，`["pages", …]`，`["page-view", …]` 与 `["edit-lock", …]`（只有挂着的会重读）。看不到的工作区、笔记本或删掉的页先离开，里面的东西不读成 404。
- **重读的合并**（`events/refresher.ts`，按键）：看得见时，同一个键 5 秒之内至多读一次，期间再来的并成最后一次；隐藏时记下，重新可见时读。有人编辑时每次自动保存都发事件，看着这一页的每个标签页若都立刻重读，服务端每 2 秒要为每个读者解析一次。
- **正在编辑的标签页**不处理这一页的正文：编辑时阅读视图卸载，`mutate` 碰不到它；离开编辑时 `PageEdit` 照 M4 预先填好阅读视图。
- **阅读视图的键**加笔记本 id：`["page-view", notebookId, pageId]`（`reading-view.tsx` 与 `page-edit.tsx` 两处），`pages` 为 `null` 时才找得到一本笔记本的视图。

### 3.9 页面树的读

`PageTreeStore.load` 给每次读编号：答复时已经有更晚开始的读生效过，就丢掉这次的结果。事件让重叠的读变多，按答复的先后覆盖会让旧树盖过新树。原有的"读开始之后有写答复就丢掉"照旧。

### 3.10 正在编辑与解除锁定

`EditLockNote`（`PageShell` 里、不在编辑时显示在阅读视图之上）：

- `useSWR(["edit-lock", page.id], () => pages.editLock(page.id))`；有持锁人时，到 `expires_in` 之后再读一次（`refreshInterval` 是 `onSuccess` 算出的数：SWR 只在轮询开始之后才调函数形式，挂上时还没有数据）：过期不是事件，租约到了锁就空出来。
- 一行带边框的提示，文字在 `<output>` 里，按钮在旁边。
- 持锁人是别人："{name} 正在编辑这一页"；是自己："你正在别处编辑这一页"（另一个标签页或令牌）。没人持锁时不显示。
- 笔记本的管理员多一个"解除锁定"，确认之后 `releaseLock`，再重读锁；失败照常显示 problem 的文案。被解除的人那边由 P4 处理（只读与说明）。

### 3.11 目录规则

`.oxlintrc.json` 加 `src/events/**`：重申 `@nervewiki/api-client` 的禁令；禁止导入 `app`、`stores`、`pages`、`components`、`onboarding`、`editor`、`reading`；`services` 只许导入类型（`allowTypeImports`）；`session` 只许导入错误类型（`SessionChangedError`）。hub 拿到的是 `EventService` 的实例，经 `RootStore` 注入。

### 3.12 端到端

- **夹具**：`anotherTab(page)`：同一上下文的新标签页，与测试的页一样受监视、结束时核对安静（关掉的除外）；`followStreams(context)`：上下文里开着的事件流的个数与持有它们的页、开过几条；`holdStream(page)`：扣住页面的流直到页面显示出来，连接的整体刷新确定地再读一次，之后别人的写只经事件到达。`watchPage` 的 `apiRequests`、`apiFailures` 不再记事件流（它是后台的长连接），已有故事的请求清单不受影响。
- **C7（页面）**：A（PAT）在 Eng 写、B 在阅读视图上：A 保存之后 B 的阅读视图更新；A 新建、改名、移动、删除一页之后 B 的页面树更新；A 开会话之后 B 看到"A 正在编辑"，结束之后消失。"看不到的人收不到"由接口版本守住。
- **C8（页面）**：
  - 短的访问令牌（`nervewikiWith`，3 秒）：流到期、重连之后，A 的保存照样到达 B；
  - B 被移出 Eng：B 的页面刷新之后不再显示 Eng 的页（找不到），工作区的笔记本列表里也没有 Eng；
  - B 在第二个标签页退出登录：两个标签页都退出，事件流关闭；
  - Eng 被删除：B 的笔记本列表里没有了。
- **C9（页面）**：同一上下文 10 个标签页只有一条事件流，每个标签页的普通请求照常答复（HTTP/1.1，每个源 6 条连接）；关掉持有者之后，别的标签页在几秒之内开出新的一条；去掉 `navigator.locks`（租约）同样。
- 浏览器对事件流的 4xx 的报告不进安静检查，另记在 `eventStreamErrors`：它们与故事的步骤竞速（会话结束时的重连答 401），声明不了个数；C8 的页面版本断言它为空。锁的 4xx 照常在 `expectConsole` 里声明；不用 `networkidle`。
- **PG3（页面）** 扣住事件流：它的前提是树还没看到别的标签页的写。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 服务、帧与连接、持有者的选举、频道、重读的合并、目录规则 | [P3-S1](plans/P3-S1-events-parts.md) |
| S2 | EventHub、依赖与接线（`AppStores`、`RootStore.events()`、`main.tsx`） | [P3-S2](plans/P3-S2-hub.md) |
| S3 | `<EventStream/>` 与刷新、阅读视图的键、页面树的读、正在编辑与解除锁定、文案 | [P3-S3](plans/P3-S3-refresh-lock-note.md) |
| S4 | 端到端：夹具、C7–C9 的页面版本、已有故事的请求清单 | [P3-S4](plans/P3-S4-e2e.md) |

## 5. 测试与验证

- **单元**（vitest，假的流、频道、存储、锁与计时器）：
  - 帧：跨块的行、多行的 `data`、注释即心跳、不认识的类型、解析失败的一帧丢掉；
  - 连接：四种结束；`signal` 中止；`SessionChangedError` 算换代；
  - 选举（`describe.each` Web Locks 与租约）：一个持有者；放手之后下一个接手；`steal()` 之后原持有者的 `lost` 中止并重新排队；租约的续期、过期、同时写入只有一个取得、解冻的旧持有者发现键不是自己的；让出；
  - hub：持有者把帧交给自己与频道；跟随者收到；`connected` 让每个标签页刷新；重连的退避（1、2、4……30 秒）、`Retry-After`、`reset` 立刻重连；`reconnecting` 让看得见的跟随者不抢；沉默 3 个心跳间隔之后看得见的跟随者抢、隐藏的不抢；`pagehide`、`freeze` 让出，`pageshow` 重新加入；`stop` 之后没有计时器、频道关闭、连接中止；再 `start` 照常；
  - 重读的合并：5 秒之内并成一次、隐藏时等到可见、按键独立；
  - 页面树：重叠的读，晚开始的先答复，早开始的不覆盖它。
- **组件**（`renderApp`，事件的替身经 `EventDeps` 注入）：`pages` 的 `tree` 让树重读；`revision` 更大才重读阅读视图、相同或更小不读；`pages` 为 `null` 时重读这本笔记本的视图；`lock` 让锁重读；`connected` 刷新列表、树、视图与锁；编辑时不重读正文；正在编辑的提示（别人、自己）、管理员的解除锁定（确认、失败的文案）、非管理员没有按钮；到期之后重读锁；换代时 hub 停止。
- **e2e**：3.12。
- **反向对照**：
  - 跟随者不检查沉默（C9 的接手失败，hub 的单元测试失败）；
  - 持有者不续租约（租约的单元测试）；
  - 不在 `pagehide` 让出（C9 关掉持有者之后接手变慢、超时）；
  - 连上之后不整体刷新（`connected` 的组件测试）；
  - `revision` 不比较、总是重读（组件测试）；合并不限 5 秒（单元测试）；
  - `tree` 不重读树（C7 的页面树一格）；`lock` 不重读锁（C7 的正在编辑一格）；
  - 页面树的读按答复覆盖（单元测试）；
  - 一个标签页一条流（不选举，C9）。

## 6. 完成标准

- `GOFLAGS=-p=3 make check`、`make gen-check`、`make e2e` 为绿；上面的反向对照都按预期失败。
- Opus 审查与修复；修复的差异较大时另经核对。
- 持续集成为绿；`--no-ff` 合并；`make image-smoke`；文档提交（本文的结果一节、总设计第 12 节）。

## 7. 结果

- 分支 `m5-p3`：S1 `d05783c`；S2 `561f993`；S3 `b8cd669`；S4 `7d87163`；审查修复 `76225d8`、核对之后的修复 `62c3656`；`600b480` 合并（`--no-ff`）。
- 门禁：每个 Step 与两轮修复的 `make check` 为绿（前端 1478 个测试）；`make gen-check`、`make e2e`（170 个）、`make image-smoke` 为绿；hub 与选举的测试连跑 10 次为绿；持续集成为绿。
- 审查：[P3 审查](reviews/P3-event-stream-web-review.md)。两位审查者，没有阻断合并的问题。Important 1：B-I1（第一次读还没答复时来的事件被丢掉，旧内容留着）；Minor 9，合并之前修掉 8 项、记下 1 项。修复的核对没有 Important，Minor 2 修掉、Nit 1 记下。
- 反向对照：S1 7、S2 17、S3 13、S4 单元 1 与 e2e 5，审查修复 13（另在改过的 C7、C9 上重跑 e2e 的 5 项）、核对之后的修复 3，都没有通过。

**与计划的出入**（已同步进上文）：

1. `Leadership` 的接口：`run(lead(lost), signal)`、`steal`、`yield`、`rejoin`、`renew(ttl)`（3.4）。
2. `connection.ts` 从 `session/token-manager` 取 `SessionChangedError`（值），lint 规则只许这一个（3.3、3.11）。
3. 持有者自己的看门狗：一条连接 3 个心跳间隔没有一帧就放弃、重连（3.6）。
4. 租约在让出与停止时立刻删键，等待中让出或停止也放掉；写不进去算没取得（3.4）。
5. 再次 `start` 的选举等上一次的结束（3.6）。
6. `browserEventDeps(storage, onStorage, tabId)`；`RootStore.newRefresher()`（3.7）。
7. 让出的消息只记"听到了"，接手靠锁的队列与存储事件（3.5）。
8. `pages: null` 按笔记本合并成一个键（3.8）。
9. 锁的提示按 `onSuccess` 算出的数轮询，`<output>` 一行（3.10）。
10. 连上之后由外向内刷新；`lock` 事件先重读树；第一次读在外的阅读视图算更旧（3.8）。
11. e2e：事件流的 4xx 另记在 `eventStreamErrors`；`holdStream`、`followStreams`；PG3 扣住事件流（3.12）。
12. 审查修复：冻结回来的标签页沉默从现在算起，让出期间与等待中的 `steal` 不留到之后；失去租约的持有者每次连接之前与每一帧都续期、不转发；中止的连接不再交出同一块里剩下的帧（3.4、3.6、3.3）。

**留给后面的**：

- **给 P4**：被解除锁定的人那边（只读与说明）；编辑中的标签页不处理自己这一页的正文事件，P4 的只读状态照此。
- **接受**：退出登录之后"流关闭"的 e2e 只看开着的个数（审查 B-m6），hub 在换代时停止由单元与组件测试守住；`src/events` 的路径规则按导入的文本匹配，`events/testing/` 的测试替身里更深的相对路径不报（核对 C-N1）。
