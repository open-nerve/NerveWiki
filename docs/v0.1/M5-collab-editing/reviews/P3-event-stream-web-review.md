# M5/P3 事件流与实时刷新（前端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m5-p3`（`8a9309f..7d87163`：4 个提交，S1 `d05783c`、S2 `561f993`、S3 `b8cd669`、S4 `7d87163`；44 个文件，+2925/−36），对照 [03-P3-event-stream-web.md](../03-P3-event-stream-web.md) 第 1–6 节、四份 Step 计划、[M5 总设计](../00-M5-design.md)第 4.10、4.11 节与 [P2 文档](../02-P2-event-stream.md)，以及实现者列出的 14 处有意的出入 |
| 审查方式 | 两位独立审查者（Opus）在仓库上只读：A 看机制（`src/events/` 的帧、连接、选举、频道、hub、重读的合并，`RootStore` 与 `main.tsx` 的接线），B 看事件在应用里做什么与测试（`<EventStream/>` 的路由、阅读视图的键、页面树的读、正在编辑的提示，单元、组件与 e2e 的测试）。两位都跑了 P3 的单元与组件测试和类型检查，各在临时目录（不进仓库）写了一个探针测试复现自己的发现。修复之后另由一位 Opus 核对（见"修复的核对"） |
| 日期 | 2026-10-04 |
| 结论 | 没有阻断合并的问题。选举、重连、停止与再启动、换代时停止都成立，SWR 的键在读与改两处一致，由外向内的刷新对重连有效。合并之前修 B 的 Important I1（第一次读还没答复时来的事件被丢掉，旧内容留着），其余 Minor 修掉或记下。A：Minor 4；B：Important 1、Minor 5 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | **第一次读还没答复时来的事件被丢掉**：`route` 只在缓存里有阅读视图、且其 `revision` 更小时重读；第一次读出去时缓存里有键没有数据，事件被跳过，那次读答复的是写之前的 revision，页面停在旧内容，直到下一次写、重连或重新聚焦。B 用探针复现（扣住第一次读，期间发 revision 2 的事件，放出 revision 1：只读了一次，显示旧内容） | 已修：缓存里有这个键而没有数据，算比任何 revision 都旧，交给重读的合并；从没在这里读过的不读（没挂着的 `mutate` 本来也不读，编辑中的标签页照旧不读自己的页）。组件测试扣住 Install 的第一次读；first-read-event-dropped 失败 |
| A-m1 | Minor | **冻结回来的跟随者抢活着的持有者**：`#heard` 只在启动、听到消息、领导结束时更新；冻结期间没听到心跳，恢复之后 `visibilitychange` 算出几分钟的沉默就 `steal()`。另一条路：Web Locks 让出期间的 `steal()` 留到重新加入，租约在 50 毫秒的等待里收到的 `steal()` 留到下一轮。结果是一次没必要的交接与每个标签页的整体刷新。A 用探针复现（两种选举都抢了） | 已修：`resume`、`pageshow` 时沉默从现在算起；让出期间不接受 `steal()`，重新加入时忘掉；租约取得之后清掉等着的 `steal()`。hub 测试"冻结期间持有者说过话的标签页回来不抢"（`FakeChannels` 加了冻结页面听不到的 `deaf`），选举测试两项；resume-keeps-silence、weblock-steal-survives-yield、lease-steal-survives-yield、lease-steal-survives-settle 失败 |
| A-m2 | Minor | **失去租约而不知道的持有者照样重连、转发**：冻结或存储事件晚到时，别的标签页已取得租约；它的流结束之后 1 秒重连，`hello` 时的续期才发现租约不是自己的，但这一帧（和同一块里的 `pages`）照样转发：每个标签页整体刷新，跟随者收到两次事件。A 用探针复现 | 已修：每次连接之前、每一帧都先续期（租约不是自己的就中止 `lost`），中止之后不转发；`connect` 在信号中止之后不再交出同一块里剩下的帧。hub 测试两项（帧到时、流结束时），连接测试一项；lost-lease-forwards、lost-lease-reconnects、aborted-connection-hands-on 失败 |
| A-m3 | Minor | **存储拒绝写租约时选举结束**：配额满或被禁写时 `setItem` 抛错，`run()` 拒绝，这个标签页再不持有也不跟随，控制台"Uncaught (in promise)"；每个标签页都这样就没有人持有 | 已修：写不进去算没取得，过一个租约的长度或被唤醒时再看；续期写不进去就让租约到期。选举测试"存储拒绝时不持有，恢复之后取得"；lease-write-throws 失败 |
| A-m4 | Minor | `src/events` 的 lint 规则没有限制 `session`（设计 3.11：只取 `SessionChangedError`） | 已改：`../session/token-manager` 只许导入 `SessionChangedError`，`../session` 的别的模块禁止（`../session/testing` 的测试替身除外）。导入 `TokenManager`、`Session` 报错，`SessionChangedError` 不报 |
| B-m2 | Minor | **删除正在编辑的页时先读锁、得到 404**：服务端在删除的写单元里结束会话（先发 `lock`），观察者之后才发 `pages`；看着这一页的标签页先读锁（404，控制台报错），提示停在"A 正在编辑"直到树的重读卸下这一页。由外向内只管重连，不管事件 | 已修：`lock` 事件先重读这本笔记本的树、等 React 提交，再读锁。组件测试"删掉的页先离开，锁不读"；lock-before-tree 失败 |
| B-m3 | Minor（测试） | 没有"更旧的 revision 不读"的测试：`!==` 的变体也通过 | 已补：重连读到 revision 2 之后来 revision 1 的事件，不读。最初写在同一个测试里时被重读合并的 5 秒挡住、变体照样通过，改成由重连读到新的；revision-not-equal 失败 |
| B-m4 | Minor（测试） | `pages: null` 的测试太松：读别的笔记本、不经合并也通过 | 已补：别的笔记本的 `null` 不读，同一本的两次只读一次；null-any-notebook、null-not-merged 失败 |
| B-m5 | Minor（e2e） | C9 的 `not.toContain(holder)` 不会失败（`holders()` 已滤掉关闭的页）；C9 接手之后、C7 开头，写可能在新流登记之前落下，连接的整体刷新照样显示出来，证明不了事件的路径 | 已改：`holdStream` 扣住页面的流直到页面显示出来，连接的整体刷新确定地再读一次，读完之后 A 的写只能经事件到达（C7）；C9 关掉持有者之前对每个别的标签页等阅读视图的读，读完再写；那条断言换成"新开了一条流" |
| B-m6 | Minor（e2e） | 退出登录之后"流关闭"只看开着的个数：一直重连、每次 401 的 hub 也通过 | 不改：换代时 hub 停止由单元测试（"换代之后 hub 停止"）与组件测试（"流随这一代结束"）守住 |

## 对出入的判断

两位都认为 14 处有意的出入成立。A 补充：沉默的看门狗与跟随者抢的门槛相同，有别的看得见的标签页时多半变成一次交接，无害，它要防的是只有一个标签页的情形；让出的消息丢了，跟随者多等三个心跳。B 补充：事件流的 4xx 不进安静检查之后，只有 C7–C9 能经页面上的结果发现坏掉的流。

## 修复的核对

修复（`76225d8`）之后另由一位 Opus 只读核对：跑了事件与 `EventStream` 的测试和类型检查，在临时目录的副本里逐项撤掉修复，确认每个新测试都因此失败（"让出期间不接受 `steal()`"与"重新加入时忘掉"两半互为冗余，要一起撤掉才失败），并写了两个探针。结论：没有 Important，B-I1、A-m1（hub 与 Web Locks）、A-m2、A-m3、A-m4、B-m2–B-m5 都对（每次连接之前的续期对 Web Locks 不做事，对租约只在键已不是自己的时候中止；写不进去的租约不会空转；锁的读总在最后一个事件之后开始，不会停在旧的）。Minor 2、Nit 1，处置如下。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-M1 | Minor | **B-I1 让编辑过的标签页回来之后多读一次**：保存时 `mutate(键, undefined)` 留下没有数据的缓存，B-I1 把它当成"第一次读在外"，自己保存的事件交给重读的合并；5 秒之内两次保存，第二次排了计时器，离开编辑之后它照样读一次（探针：3 次对 2 次）。设计 3.8：编辑中的标签页不处理这一页的正文 | 已修：没有数据的缓存只在第一次读正在外（SWR 缓存的 `isLoading`）时才算更旧。组件测试"编辑过的标签页回来只读一次，不为自己保存的事件再读"（假的计时器走过合并的 5 秒）；dataless-view-always-read 失败，loading-view-not-read 让 B-I1 的测试失败 |
| C-M2 | Minor | **A-m1 对租约只修了一半**：等待的 50 毫秒里收到的 `steal()`，只在自己取得时清掉；别的标签页赢了时它留着，赢家下一次续期的存储事件把它唤醒去抢一个刚连上的持有者 | 已修：等待结束就清掉，无论谁取得。选举测试"等待时被要求抢、别人赢了：赢家续期不会让它去抢"；settle-keeps-steal 失败 |
| C-N1 | Nit | `src/events` 的路径规则按导入的文本匹配，`events/testing/` 里写 `../../session/session` 不报 | 不改：同一条规则的别的组本来就是这样，`events/testing/` 只放测试替身 |

## 反向对照

审查修复：first-read-event-dropped、resume-keeps-silence、weblock-steal-survives-yield、lease-steal-survives-yield、lease-steal-survives-settle、lost-lease-forwards、lost-lease-reconnects、aborted-connection-hands-on、lease-write-throws、lock-before-tree、revision-not-equal（最初被合并的 5 秒挡住、照样通过，改了测试之后失败）、null-any-notebook、null-not-merged；e2e 的 tree-not-read、lock-not-read、no-election、no-yield-on-pagehide、connected-all-at-once 在改过的 C7、C9 上重跑。核对之后的修复：dataless-view-always-read、loading-view-not-read、settle-keeps-steal。都失败。lint 规则另以三个导入核实（`TokenManager`、`../session/session` 报错，`SessionChangedError` 不报）。
