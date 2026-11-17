# M12 旁支 `m12-e2e-l1-new-tab`（M6 的 L1 不再等浏览器开出的标签页）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-e2e-l1-new-tab`，从 `main` 的 `41d0cfda`（`docs: M12/P3 done`）开始（主持者的决定 541）。实施的 1 个提交，审查时在 `12e868bf`：L1 (page) 在 window 上最后拿到带修饰键的点击，核对应用没有取消它、它带着修饰键、链接的地址是真的，然后取消它，地址在故事自己开的标签页里打开（`e2e/stories/links/l1-links.spec.ts`，加 33 行、减 7 行）。<br>修复一轮在它之上 2 个提交，头 `d716d9a9`：阅读视图的链接的单元测试在修饰键那一格记下每次点击的 `defaultPrevented` `1b01b1d9`（L1A-M1）；故事的标签页改由 `anotherTab` 开，`expect` 带说明，点击的记录加 `download`，注释照实写 `d716d9a9`（L1A-N1，L1B-M1、N1，L1C-N1–N3）。<br>修复核对之后：只改注释的一个 `5ea4ade0`（L1B-N2）；把 `main`（`cfa31e00`，P4/S4、S5 的合并与负责人 2026-11-17 的回答）合进来一个 `56537688`。<br>到 `5ea4ade0` 2 个文件（`l1-links.spec.ts`、`web/apps/web/src/reading/app-links.test.ts`），加 56 行、减 13 行；只改测试，不改产品 |
| 审查方式 | 调查者（之后也是实施者）在分离的工作树 `nerve-wiki-m12-l1-flake`（`89e2389a`）里复现，读 16 份失败的 trace 与 80 次的协议日志，试改法。三位审查者并行，各在自己的分离的工作树里（`nerve-wiki-m11-p1-s1-rev-a`、`-b`、`-c`，挪到 `12e868bf`），重的都经 `heavy.sh`、只跑 Chromium；不碰实施者的工作树：<br>A 产品一侧：`app-links.ts` 与它的单元测试；改后的 L1 是否仍证 L1 的故事；Ctrl、Meta、Shift、Alt 与中键五种点击（探针，在真的 Chromium 里跑真的应用）；window 上的监听器是否排在应用的之后（应用挂在哪一层、捕获还是冒泡、React 的根）；`closest("a").href` 是否就是浏览器去的地址；"浏览器真的开了标签页"别处有没有人守；变异 mA、mB、mC。<br>B 原因：在 `41d0cfda` 上用原测试复现 200 次；开协议日志再 200 次（4 批各 50），逐次判新标签页的事件；读 Playwright 1.63 的代码路径，对照本机只读的 1.62.1 与 1.65 alpha；同一修订的完整版 Chromium 200 次；AS3 的 `popup` 200 次。<br>C 测试、变异、提交说明与文档的去处：重跑 m1–m4，加 c1、c2、捕获阶段的对照与 vitest 的 c1；`make lint-web`、L1 200 次、AS3 100 次；照 00-M12 第 7 节"旁支"与先例列文档的清单 |
| 日期 | 2026-11-17（调查、实施、审查、修复一轮、修复核对一轮） |
| 结论 | Important 0。Minor 4：单元测试只看带修饰键的点击没有走路由、不看它没有被取消，改后"不取消"只有 L1 的 Ctrl 一处守（L1A-M1）；提交说明说红的那几次没有 lifecycle 的 `init`，13 次红里 3 次有（L1B-M1）；提交说明没提 M11 决定 277 与 M11/P2/S3 的调查（L1C-M1）；文档的清单比先例少几处（L1C-M2）。Nit 7（L1A-N1、N2，L1B-N1，L1C-N1–N4），Q 4（L1A-Q1 = L1C-Q1，去重 3）。A 确认改后的 L1 证的仍是 L1 的故事里属于应用的那一半，地址一点比旧的更严，window 上冒泡阶段的监听器最后拿到点击；B 确认原因的核实部分成立（红 13/13、绿 0/187 合"没有 `Page.frameNavigated`，且 `Page.getFrameTree` 答 url 为 `""` 的初始文档"）；C 确认改后的断言在改坏的代码上都红，挂在捕获阶段就分不出取消了点击的应用。L1A-Q1、L1C-Q1 由负责人 2026-11-17 第 7 问关上；代码一侧由修复一轮 2 个提交改，文档一侧在本提交。修复核对一轮：三位都没有 Important、Minor，代码一侧的发现都关上；新的 Nit 两条，L1B-N2 由注释提交 `5ea4ade0` 改，L1C-K1-N1 在"更正" |

## 起因

- **P4/S4 的分支 CI**：合并 `main` 的 `37096b2a`，run 43979118856 第一次 attempt，`e2e` 只红在 L1 (page)（`l1-links.spec.ts:179`，当时的行）：`browserContext.waitForEvent: Test timeout of 30000ms exceeded`，失败时焦点在 `a "Target"`。只重跑 e2e，第二次 attempt 全绿。P4/S4 的实施者在本机 Chromium 里交替各跑 20 次，`main`（`7696d8df`）与分支各超时 3 次；S4 没碰正文的链接，判为 `main` 上已有的不稳（[P4/S4 的审查记录](P4-S4-page-tree-review.md)"门禁与 CI""持锁与负载"）。
- **主持者的调查说明**原写"`main` 上的 CI 一直没见它红过"，不对：调查者经 `gh` 只读取了 2026-10-18 起的 400 次 `ci.yml` 运行，L1 (page) 在第一次 attempt 红过 3 次，红法相同（`waitForEvent` 超时，`focused: a "Target"`），重跑都绿：
  - 41188823890：`main`，`9d4ea2d1`，2026-10-31；
  - 42913206700：`m11-p2-s2`，`b6ef8599`，2026-11-10；
  - 43979118856：`m12-p4-s4`，`37096b2a`，2026-11-16（UTC）。
- **更早的记录**：
  - M10/P3/S1：上面的 41188823890。trace 里有第二个 page id，新文档与另外 40 个请求都 200，`page` 事件只有来源页的一个（`M10-llm-wiki/reviews/P3-S1-lint-protocol-review.md:430`）。
  - M10/P4/S1 本机的另一种样子：新标签页报了，"Target" 的标题 10 秒不可见（`l1-links.spec.ts:181`，当时的行；[M10/P4/S1 的审查记录](../../M10-llm-wiki/reviews/P4-S1-virtual-files-review.md) `:222`）。
  - M11/P1/S3 本机：修复一轮的头上 `make e2e` 第一次，负载约 10，同一行、同一症状（`M11-upgrade/reviews/P1-S3-abandon-rollback-and-end-review.md:55`）。
  - M11/P2/S2：修复一轮的 CI 42913206700，"这是第四次记下"，交给 P2/S3 另查（`M11-upgrade/reviews/P2-S2-gate-check-submit-review.md:72`、`:100`）。
  - M11/P2/S3 查了原因（下面"定因"的"M11 已核实的"），主持者的决定 277（[00-M11](../../M11-upgrade/00-M11-design.md) `:897`）：L1 照现状，CI 再红只重跑 e2e；向 Playwright 上游报问题（对外）与改测法（不再测"浏览器真的开了新标签页"，削弱测试）留给负责人。M11 收尾记录把它列为留给负责人定的（`M11-closeout-review.md:97`、`:176`）；00-M12 第 10 节的风险"e2e 的偶发挡发布"照它写"其余的偶发照 M11 第 277 条的做法只重跑 e2e"。
- **这一次**：主持者另开调查，查清之后照 P1 的旁支的先例开旁支修（决定 541）。审查 A、C 指出这正是 277 留给负责人的那一条（L1A-Q1、L1C-Q1）；主持者问了负责人，2026-11-17 第 7 问选"采用改法"，取代 277（[00-M12](../00-M12-design.md) 第 11 节"2026-11-17：L1 的改测法与 P4/S3、P5 之前的四问"）。

## 定因

### M11 已核实的

M11/P2/S3 的"偶发"（[S3 的审查记录](../../M11-upgrade/reviews/P2-S3-web-review.md) `:75-90`；决定 277）：

- 本机开着协议日志，`--repeat-each=30` 两轮：空载 3 次、满负载 4 次失败，约九次一次。不开日志时的比例当时没量。
- 失败的 7 个新标签页，`Page.getFrameTree` 答的主框架 URL 是空的；之后标签页照样提交了文档（执行上下文清掉又建、`DOMContentLoaded`、`load`，网络都 200），只是没有 `Page.frameNavigated`。通过的 53 个都有，在 `Runtime.runIfWaitingForDebugger` 的答复之后到。审查 B 当时在实施者的日志上重核了 3 个。
- Playwright 1.63 的 `FrameSession._initialize` 在主框架 URL 为空时等 `_firstNonInitialNavigationCommittedPromise`，只有非初始的 `Page.frameNavigated` 兑现它，兑现之后才 `reportAsNew`；`waitForEvent("page")` 与 `context.pages()` 都只由这一步填，所以"也认 `context.pages()`"不是改法。
- 两种只改测试的改法（把新标签页的文档压后一秒，之后 `unroute` 或 `unrouteAll({ behavior: "wait" })`）各有 5/30 别的失败。

### 这一次新核实的

- **不开协议日志的比例**（只跑 L1 (page) 一格，Chromium，`--repeat-each`，4 个 worker，经 `heavy.sh`）：

  | 树 | 谁 | 红 / 次数 |
  |---|---|---|
  | `main` 的 `89e2389a` | 调查者 | 8/200（1/40 + 7/160） |
  | `7fdb9f58`（P2/S1 合并之前的 `main`） | 调查者 | 7/200 |
  | `41d0cfda`（旁支的起点） | 审查 B | 9/200（2.9 分钟） |

  本机约 25 次一次。前两个都在 4% 上下，分不出差别，没有二分：与 P2/S1 无关。
- **每次红时的状态**（调查者，16 份 trace：本机 `89e2389a` 8 份、`7fdb9f58` 7 份、CI 1 份）：点击的坐标都在链接 "Target" 的框内，`ControlOrMeta` 在 Linux 上就是 Ctrl；CDP 收到 `Page.frameRequestedNavigation`，`reason: anchorClick`、`disposition: newTab`；新标签页开了，请求了 Target 页的文档，应用接着发出 12 个接口读；可是整个 trace 里只有第一个页的那一个 `page` 事件；原页的地址没变。
- **漏报的条件是两条合起来**（审查 B，协议日志 200 次，红 13 次）：标签页的文档没有 `Page.frameNavigated`，且 `Page.getFrameTree` 答的是 url 为 `""` 的初始文档。判法：取 Ctrl 点击（`Input.dispatchMouseEvent` 的 `mousePressed`，`modifiers: 2`）之后第一个 `Target.attachedToTarget`（type page），看它那个 session 第一个文档的事件，到 `reload` 的 `frameStartedNavigating` 为止。

  | 结果 | 第一个文档的 `frameNavigated` | `getFrameTree` 的 url | 次数 |
  |---|---|---|---|
  | 红 | 没有 | `""` | 13 |
  | 绿 | 有 | `""` | 182 |
  | 绿 | 有 | `about:blank` | 2 |
  | 绿 | 没有 | Target 页的地址 | 2（`b2-r000`、`b2-r003`） |
  | 绿 | 没有 | `about:blank` | 1（`b3-r009`） |

  - 合两条的：红 13/13，绿 0/187。
  - 3 次绿也没有 `frameNavigated`：`getFrameTree` 答的已不是 `""`，Playwright 只把 url 为 `""` 或 `":"` 的当作初始空页（`coreBundle.js:37612`），当场兑现、报告页面（`37619`）。所以 Chromium 漏发这个事件比红的次数更常见，多数被 `getFrameTree` 掩住。
  - 调查者早先的 80 次（`89e2389a`，开日志，红 3）同样：红的 3 次都没有 `frameNavigated`，`getFrameTree` 都答空文档。
  - 每次运行都有新标签页的 `Target.attachedToTarget`，带 `waitingForDebugger: true`；设置期间的命令都有答复，含 `runIfWaitingForDebugger`（13/13）。错误只有 `Emulation.setLocaleOverride: Another locale override is already in effect`（绿的也有）与 `Network.getResponseBody`。
- **是 Chromium 没发，不是 Playwright 漏收**（审查 B）：`CRConnection._onMessage` 在分给 session 之前记下每条收到的消息（`coreBundle.js:34945-34946`），13 份红的日志里点击之后任何 session 都没有主框架的 `Page.frameNavigated`。Playwright 也不丢晚到的事件：管道每条消息各用一个 `setImmediate` 分发（`39435-39455`），session 的事件晚一个微任务才发出（`35015`），主框架的 `frameNavigated` 监听器在处理 `getFrameTree` 的答复时挂上（`37598`）；唯一会丢的是 `frameNavigated` 早于 `getFrameTree` 的答复，200 次里 0 次。
- **Playwright 的等待没有时限**（审查 B，1.63 的 `coreBundle.js`）：`CRPage` 在 `_mainFrameSession._initialize()` 完成之后才 `reportAsNew`（`37294-37297`）；`_initialize`（`37577-37672`）把 `_firstNonInitialNavigationCommittedPromise` 放进 `Promise.all`（`37671-37672`），它只由非初始的 `_onFrameNavigated` 兑现（`37741-37747`）、由 `dispose` 拒绝（`37684`）；`_markInitialized` 才发出上下文的 Page 事件（`22331-22350`）。红的那几次，目标在 +30.3–30.7 秒断开，就是测试拆掉 context 的时候。
- **新版的 Playwright 一样**：本机只读的 1.62.1、1.63.0 与 1.65.0-alpha-2026-10-30 的 `FrameSession._initialize` 都有同样的 `isInitialEmptyPage`，都等这个承诺、不带超时；1.65 alpha 的改动与这里无关。升级 Playwright 去不掉它。
- **同一修订的完整版 Chromium 没有**（审查 B 的 L1B-Q1，决定 545）：临时配置把 chromium 项目换成 `channel: "chromium"`，即 `chromium-1243/chrome-linux-arm64/chrome` 的 new headless（跑的进程核对过），在 `41d0cfda` 上跑原测试 200 次：0 红，2.2 分钟。同机同时段的 headless shell 9/200；若真实的比例是 4.5%，200 次一次不红的概率约 1e-4。"只在 headless shell 里"由此是观察到的事实。
- **AS3 的 `popup`**：它过 Playwright 的同一段代码（`popup` 也从 `_onPage` 发出），`as3-render.spec.ts:327` 点 `target="_blank"` 的 PDF 链接。审查 B 在 `12e868bf` 上 200 次、C 100 次，都 0 红。那条路有 opener，是渲染进程开的窗口；L1 的 Ctrl 点击是浏览器开的后台标签页，没有 `openerId`。

### 推断

没有往 Chromium 里查（源码本机没有，没有上网）：

- **headless shell 里，新标签页的导航与 DevTools 自动接上之间的竞态**。日志里与它相符的观察（审查 B）：200 次每一次，标签页的文档请求与响应都在 `runIfWaitingForDebugger` 答复之前就到了，即 `waitForDebuggerOnStart` 没有拦住这次导航；新文档的主世界上下文早于这个答复建立的，红 4/13，绿 3/187；187 次绿里 176 次在 `frameNavigated` 之后紧跟一个被取消的文档请求（`ERR_ABORTED`，请求号在本 session 从未出现过），13 次红一次也没有。这些只说明红走的是一条少见的路，说不出 Chromium 为什么不发这个事件。
- lifecycle 的 `init` 不是标志：13 次红里 10 次没有新文档的 `init`，3 次有（`b1-r010`、`b1-r036`、`b4-r008`；如 `b1-r010` +588 ms 收到 `init`，正是标签页文档请求的 loaderId，之后主世界与工具世界的上下文都建在应用的地址上，到 +30525 ms 断开都没有 `frameNavigated`）。
- **读者不受影响**：浏览器每次都开了标签页、载入了页面，缺的只是 DevTools 协议里的一个事件。有界面的 Chrome 没试。
- CI 上（多半是 x86_64）的比例与机制没有核对：三次失败的协议层面读不到。开日志之后从 9/200 升到 13/200，暗示与负载有关；没跑单 worker 的对照。新版的 headless shell（1.65 alpha 带的 Chromium 1252）本机没装，没试。

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `12e868bf` 的。修复一轮照主持者的修复说明（`l1-flake/fix-1.md`）在 `12e868bf` 之上加 2 个提交 `1b01b1d9`、`d716d9a9`，普通推送、不改写已推的历史，每个单独为绿；文档的发现在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| L1A-Q1 = L1C-Q1 | 问 | 这一改正是 M11 决定 277 留给负责人的"改测法"（`00-M11-design.md:899`；M11 收尾记录 `:97`、`:176`；00-M12 第 10 节"其余的偶发照 M11 第 277 条的做法只重跑 e2e"），主持者的说明却记成主持者的决定：主持者自己定，还是合并之前问负责人 | 主持者问了负责人：2026-11-17 第 7 问选"采用改法"，取代 277（00-M12 第 11 节）。决定 541 写明取代与理由，277 里向上游报问题的一半仍是负责人的；第 10 节的风险一行加一句。核对 A、C：关上 |
| L1A-M1 | 中（测试） | `app-links.test.ts:127-154` 修饰键那一格把每次点击都取消、只看没有走路由：A 的 mA（带修饰键或别的按键的点击取消之后返回）与 mB（只取消 Shift、Alt）vitest 10 个全过。改后"不取消"在 e2e 里只有 L1 的 Ctrl 一处守，Shift、Alt 哪里都没人守 | `1b01b1d9`：本地的记录器 `handled` 在取消之前记下 `defaultPrevented`，断言 `[...clicks.map(() => false), true]`（决定 543）。核对 A：mA、mB 红在 `:161`，关上 |
| L1A-N1 | 细节（测试） | 丢掉的那一步的一种样子：A 的 mC 给页面的链接加 `download`，Ctrl 点击就成了下载，新测试照过，旧测试超时；window 上的记录看不到它 | `d716d9a9`：记录加 `download: link?.hasAttribute("download") ?? false`，断言为假。核对 A：mC 红在 `:210`，`download: true`，关上 |
| L1A-N2 | 细节（文档） | P1 设计决定 339 的"L1 的新标签页晚于加载才被监视（`l1-links.spec.ts:183`），照样等得对"（`01-P1-foundation.md:113`；`:50` 也引了 `:183-184`）：改后套件里不再有晚于加载才被监视的页 | 本提交照决定 481 的做法加两处后注：这句说的性质不变，仍由页面里的读与 A15 的各格守住 |
| L1B-M1 | 中（提交说明） | `12e868bf` 的说明说红的那几次"没有 lifecycle init 事件"：B 的 13 次红里 3 次有（`b1-r010`、`b1-r036`、`b4-r008`）。写进 lessons 或记录，以后的人会拿它当判据 | 注释由 `d716d9a9` 改，不提 `init`；05-lessons 那一条也不提（决定 544）；说明见"更正"。核对 B：代码一侧关上 |
| L1B-N1 | 细节（提交说明） | 说明的"exactly""the 77 green runs each had the event""the green ones show"比数据绝对：B 的 187 次绿里 3 次也没有 `frameNavigated`（`getFrameTree` 答的已不是 `""`），11 次没有被取消的文档请求 | 注释由 `d716d9a9` 把两条写在一句里；说明见"更正"。核对 B：代码一侧关上，另见 L1B-N2 |
| L1B-Q1 | 问 | 同一修订的完整版 Chromium 200 次 0 红：要不要写进记录与 lessons；B 不建议因此改用完整版 | 写进本记录的"定因"，作观察到的事实；套件与 CI 不改用完整版（决定 545）；05-lessons 那一条带一句本机的对照 |
| L1C-M1 | 中（提交说明、文档稿） | 提交说明与文档的清单都没提 M11 决定 277 与 M11/P2/S3 的调查（`P2-S3-web-review.md:75-90`）：同一个原因 M11 已查到、写下，读来像新查出的；这次的做法正是 277 列出、留给负责人的那一种。漏了反向的先例，照先例算 Minor（`P1-source-revert-deadline-review.md` 的 SRCC-M3） | 不改写分支上的提交（先例 `642f9196`）：本记录的"起因""定因"引 277 与 P2/S3，分开写 M11 已核实的、这次新核实的与推断；"更正"补一句；决定 541 引 277 |
| L1C-M2 | 中（文档稿） | 文档的清单比 00-M12 第 7 节"旁支"与先例（`642f9196`、`8635667a`、`db009edd`）少：第 11 节带号的决定、第 12 节的链接与合并号、第 13 节一行、合并之后的文档提交；决定的号要与 P4/S4、S5 对好（先例 SRCC-M2 是撞号） | 本提交写全：本记录、第 11 节决定 541–547（P4/S5 已占到 540，从其后起）、第 12 节 P4 一行、第 13 节一行、第 10 节风险一行、05-lessons、P1 设计两处后注；不属 Phase，没有 Phase 设计的"结果"可写。合并号与合并之后的第一次 `main` 由合并之后的文档提交补 |
| L1C-N1 | 细节（测试） | c2（应用在容器上 `stopPropagation()` 带修饰键的点击）让断言只报 `Received: undefined`，读的人要回到代码才知道 window 没拿到点击 | `d716d9a9`：`expect` 带说明 `… (undefined: it never got there)`。核对 C：c1、c2 的失败信息都以它起头，关上 |
| L1C-N2 | 细节（测试） | 改后 L1 是唯一直接 `page.context().newPage()` 的故事，失败时"What the pages showed last"里没有它；另 10 个故事文件用 `anotherTab` | `d716d9a9`：改用 `anotherTab(page)`，不再 `watchPage`、`expectQuietPage`、`close`，由夹具在结束时查它安静。核对 C：报告有 `another tab 1`，夹具的检查盖住原来显式的那一次，关上 |
| L1C-N3 | 细节（注释） | 注释的"about 1 run in 25"是本机 4 个 worker 的比例，M11 开着日志量到约九次一次，CI 上是几周一次 | `d716d9a9`：写 "about 1 run in 25 here"。核对 C：关上 |
| L1C-N4 | 细节（提交说明） | "After: 0 of 200 runs at 89e2389a"：提交在 `41d0cfda` 上，两者只差文档，读的人要自己去对 | 见"更正"；C 在 `12e868bf` 上另跑 200/200（"门禁与 CI"） |
| L1C-Q2 | 问 | 第 12 节的链接与合并号放在哪一行：旁支不属哪个 Phase | 放在 P4 一行：在 P4/S4 的分支 CI 上见到（决定 546） |
| L1B-N2 | 细节（注释，修复核对） | `d716d9a9` 的注释"answered the empty first document"没分开 url 为 `""` 的与 `about:blank` 的：`b3-r009` 绿，`getFrameTree` 答 `about:blank`、没有 `frameNavigated`，Playwright 不等；照一般的读法它也合注释的条件 | 只改注释的 `5ea4ade0`：`the first document with no address (url "", not about:blank)`；`d716d9a9` 说明里同一句见"更正"（决定 547） |
| L1C-K1-N1 | 细节（提交说明，修复核对） | `d716d9a9` 的说明"It stays open to the end, as theirs do"：另 10 个用 `anotherTab` 的故事文件里，c3、c6 的第一个故事与 hi3 关掉自己的标签页 | 不改写已推的提交，见"更正"（决定 547） |

## 主持者的决定

负责人可以改判；正文在 [00-M12](../00-M12-design.md) 第 11 节"2026-11-17：L1 的旁支的决定"。

- **541 改测法、不改产品，取代 M11 第 277 条里 L1 的"照现状、CI 再红只重跑 e2e"**：负责人 2026-11-17 第 7 问选"采用改法"（L1A-Q1、L1C-Q1）。旁支照常审查、合并，同 P1 的旁支（决定 356、359）。277 里"向 Playwright 上游报问题（对外）"的一半仍是负责人的，不做。00-M12 第 10 节"e2e 的偶发挡发布"一行加一句。
- **542 测试名不改**：名字说的是产品的行为，带修饰键时浏览器在新标签页打开，这没变；测试证的是"应用把这次点击留给浏览器、地址是真的"，注释写明。
- **543 `app-links.test.ts` 记下 `defaultPrevented` 随旁支改**（L1A-M1，`1b01b1d9`），不交给 P2 的打磨。
- **544 05-lessons 第 6 节加一条，随本提交**：漏报的条件写成两条合起来，不提 `init`，写明不能挂在捕获阶段，比例写成本机的。
- **545 完整版 Chromium 的对照写进本记录的"定因"**（L1B-Q1），套件与 CI 不改用完整版。
- **546 第 12 节的链接与合并号放在 P4 一行**（L1C-Q2）。
- **547 修复核对之后不开第二轮**：L1B-N2 由 `5ea4ade0` 改；已推的提交说明不改写，不准处写在"更正"（L1B-M1、N1、N2，L1C-M1、N4、K1-N1）。

## 实施中定的

调查与实施的报告没有落成文件（工具不让写），主持者摘在设计工作目录的 `m12/l1-flake/host-facts.md`。

1. 提交的后缀写 `(M6 story, found in M12)`，不用主持者说明里的 `(M12, found in M12/P4/S4)`：L1 带修饰键的一格是 M6/P3/S5 加的（`52ac9870`），照 00-M12 第 7 节"旁支"。审查 C 核过。
2. Firefox、WebKit 的探针（临时配置 `e2e/l1-probe.config.ts`，各 10 次全过，跑完删了）不进门禁、不提交：`threeBrowsers` 不含 L1。
3. 测试名不改，是主持者的决定（决定 542）。

## 接受与推后的

- **接受**：
  - e2e 不再看"浏览器为一次没被取消、带修饰键的链接点击真的开出标签页"，算作浏览器的事（决定 541）。对页面的链接，别处没有人守这一步：AS3 守的是 `target="_blank"` 的附件链接开出弹窗，单元测试不看这一步（审查 A 第 3 节第 5 条）。丢掉的那一步的一种样子是 A 的 mC（链接带 `download`，Ctrl 点击成了下载），由 `d716d9a9` 的 `download` 一项补上一部分（L1A-N1）；别的会改变浏览器动作的东西，window 上的记录仍看不到。
  - 测试名不改（决定 542）。
  - e2e 只点 Ctrl：Meta、Shift、Alt 与中键由 `app-links.test.ts` 守，`1b01b1d9` 之后连"不取消"也守；中键在浏览器里发的是 `auxclick`，应用不听它，今天的写法下坏不了（审查 A 的探针）。
  - 故事的标签页开到测试结束，与来源页同在一个 context（共用会话的记录与续期的锁），同另外几个不关标签页的故事；核对 A 跑 50 次、C 跑 100 次，没看到它带来的不稳。
  - 不用 CDP（`newBrowserCDPSession` 加 `Target.setDiscoverTargets`）去找浏览器开的那个标签页：它的应用正在启动、续期，这时关掉它可能丢掉续期（`e2e/fixtures/navigation.ts` 的文件头），不关它就在故事旁边一直跑着。
- **推后**：向 Playwright 上游报问题（对外）仍是负责人的，不做（决定 541）。
- **未核实**：Chromium 一侧为什么不发 `Page.frameNavigated`；新版的 headless shell 还有没有；CI 上的比例与机制；负载与 worker 数的影响；有界面的 Chrome（都见"定因"的"推断"）。AS3 的 `popup` 本机 300 次没碰上，`target="_blank"` 的打开不排除，不扩到 AS3（审查 B、C）。
- **不改的文档**：
  - M6 设计 `00-M6-design.md:375`（"修饰键与中键照浏览器的默认行为，地址是真的"）与 M6/P3 设计 `03-P3-index.md:398`（"Ctrl/⌘ 点击开新标签页"）说的是产品，产品没变；M11 的 277、P2/S3 的"偶发"与 M11 收尾记录是当时的记录。都不改，取代写在 00-M12 第 11 节的第 7 问与决定 541。
  - `v0.1-design.md`：第 13 节的长期约定没变，跨 M 的契约与扩展点也没变（先例 357 也没改）。
  - Phase 设计：这条旁支不属哪个 Phase，没有"结果"可写；P1 设计只加两处后注（L1A-N2）。
  - 移交：没有与 L1 有关的 `open` 事项；M11 收尾记录 `:176` 说 277 不进移交。

## 更正

已推送的说明不改写（先例 `642f9196` 的"更正"），在这里写对：

- `12e868bf`：
  - "the three red runs are exactly the ones where Chromium sent no Page.frameNavigated for the tab's document" 与 "the 77 green runs each had the event"：作判据不对。B 的 200 次里 3 次绿也没有这个事件（`getFrameTree` 答的已是 Target 页的地址或 `about:blank`）；红的条件是两条合起来：没有 `Page.frameNavigated`，且 `Page.getFrameTree` 答 url 为 `""` 的初始文档（红 13/13，绿 0/187；L1B-N1、N2）。
  - "in the red runs the tab's navigation commits without the lifecycle init event and without the cancelled document request the green ones show"：13 次红里 3 次有新文档的 `init`，`init` 不是红的标志（L1B-M1）；被取消的文档请求，187 次绿里 11 次也没有，应是"most green ones"（L1B-N1）。
  - "After: 0 of 200 runs at 89e2389a"：是在 `89e2389a` 上加这一改跑的；提交在 `41d0cfda` 上，`89e2389a..41d0cfda` 只改 00-M12 与 P3 设计，代码相同；`12e868bf` 上审查 C 另跑 200/200（L1C-N4）。
  - 说明读来像这次才查出原因，没有引 M11 决定 277 与 M11/P2/S3 的调查：失败的标签页没有 `Page.frameNavigated`、Playwright 等不到就不报，M11 已查到、写下；这次的改法是 277 当时列出、留给负责人的那一种（L1C-M1）。
- `d716d9a9`：
  - "It stays open to the end, as theirs do"：另 10 个用 `anotherTab` 的故事文件里，c3（`c3-release.spec.ts:83`）、c6 的第一个故事（`c6-expired.spec.ts:76`）与 hi3（`hi3-log-revert.spec.ts:382`）关掉自己的标签页，"as theirs do"只对一部分成立（L1C-K1-N1）。
  - "Page.getFrameTree answered the empty first document"：Playwright 只等 url 为 `""` 的初始文档，`about:blank` 的不等（`b3-r009`）；注释由 `5ea4ade0` 改（L1B-N2）。

## 反向对照与变异

产品的变异都在 `app-links.ts`，每条重新 `make build`，跑 L1 (page) 一格，Chromium，经 `heavy.sh`；先在没改的树上跑同样的命令。

**实施时**（`12e868bf`；实施者，每条 2 次；m1 在调查时也在 `89e2389a` 加改法上跑过，4/4 红在 `"cancelled": true`）：

| # | 变异 | 结果 |
|---|---|---|
| m1 | Ctrl 走路由（条件里去掉 `event.ctrlKey` 一项） | 2/2 红，`"cancelled": true` |
| m2 | 带修饰键也走路由、不取消 | 2/2 红，来源页的标题 "Source" 不见了 |
| m3 | 按着 Ctrl 时，`pointerdown` 去掉链接的 `href` | 2/2 红，`"address": ""` |
| m4 | 去掉应用的 `preventDefault` | 2/2 红，`notReloaded()` 为假 |

`mut.py` 把 m1、m3、m4 标成 `FAILED(unnamed)`：输出被截在 "2 failed" 那一行之前；三条都手查过日志。

**审查时**（`12e868bf`；L1 每条 2 次，2 个 worker；C 先在没改的树上 200/200，A 10/10）：

| # | 谁 | 变异 | vitest `app-links.test.ts` | L1 (page) |
|---|---|---|---|---|
| m1–m4 | C | 同上 | m1 红在"a click with a modifier … is the browser's" 一格 | 都 2/2 红：m1 `:200` `cancelled: true`，m2 `:215` 来源页的标题，m3 `:200` `address: ""`，m4 `:221` `notReloaded` |
| c1 | C | 带修饰键的点击取消、什么也不做（用户的 Ctrl 点击没反应） | 10/10 绿（活下来） | 2/2 红，`:200`，`cancelled: true` |
| c2 | C | 带修饰键的点击在容器上 `stopPropagation()` | — | 2/2 红，`:200`，`Received: undefined`（L1C-N1） |
| m1 + 捕获 | C | m1，故事的监听器改成 `{ once: true, capture: true }` | — | 2/2 绿（活下来）：它先于应用，它的 `preventDefault` 让应用的处理器在 `defaultPrevented` 上早退，路由不走 |
| c1 + 捕获 | C | c1，同上 | — | 2/2 绿（活下来）：它在应用取消之前读，`cancelled` 恒为假 |
| 捕获 | C | 只把故事的监听器改成捕获（对照） | — | 2/2 绿：好的代码上也绿，只有放在最后才分得出 |
| mA | A | 带修饰键或别的按键：`preventDefault()` 之后返回，不走路由 | 10/10 绿（活下来） | 2/2 红，`:200`，`cancelled: true` |
| mB | A | 只取消 Shift、Alt 的点击 | 10/10 绿（活下来） | 2/2 绿（L1 只点 Ctrl） |
| mC | A | 页面的链接加 `download` | — | 新测试 2/2 绿；`41d0cfda` 的旧测试 1 次，30 秒超时 |
| 探针 | A | Ctrl、Meta、Shift、Alt 与中键各点一次，在 window 上记下并取消 | — | 前四种都发 `click`，`defaultPrevented` 为假，地址等于 `${baseURL}${targetPath}`；中键只发 `auxclick`；五种在 window 上的 `eventPhase` 都是 3 |

**修复一轮**（实施者；vitest 在 `1b01b1d9`，L1 在 `d716d9a9`，`:210` 是新的 `toEqual` 那一行）：

| # | vitest | L1 (page) |
|---|---|---|
| m1 | 红 | 红 `:210` |
| m2 | 红 | 红 `:224` |
| m3 | 绿 | 红 `:210` |
| m4 | 红 | 红 `:230` |
| c1 | 红（改前活下来） | 红 |
| c2 | 红 | 红，`Received: undefined`，带说明 |
| mA | 红（改前活下来） | 红 |
| mB | 红（改前活下来） | 绿（L1 只点 Ctrl） |
| mC | 绿 | 红，`download: true`（改前活下来） |

**修复核对**（`d716d9a9`，vitest 在 `1b01b1d9`）：A 的 mA、mB 在 vitest 上红在 `:161`（mA 前 13 次都收到 `true`，mB 只有第 3、4 次 Shift、Alt 是 `true`），mC 在 L1 上 2/2 红在 `:210`，`download: true`；C 的 c1、c2 红在 `:210`，失败信息以 `expect` 的说明起头；C 的探针：故事的标签页去错的地址，红在 `:221`，报告先是 `another tab 1` 与那个地址、再是 `page`；标签页载入之后往它的控制台写一条错误，红在测试结束时 `anotherTab` 的安静检查上，报告的 `another tab 1` 写着这条错误。

## 核对

**修复核对一轮**（2026-11-17）：三位把工作树挪到 `d716d9a9`，只看 `12e868bf..d716d9a9`。

- **A**：L1A-M1、N1 关上（数见上）；L1A-Q1 由负责人的回答关上；N2 交本提交。新断言在没改的树上稳：L1 50 次全过（30.5 秒），整个 `l1-links.spec.ts` 一遍 5 格全过，`app-links.test.ts` 10 个全过；两个新字段只读点击那一刻的状态，没有新的等待。没有新发现。
- **B**：没跑重的，用第一轮的协议日志与 Playwright 的代码核对 `l1-links.spec.ts:179-185` 的注释与 `d716d9a9` 的说明。L1B-M1、N1 代码一侧关上，说明的部分交"更正"；Q1 交本提交。注释里关于原因的几句（Playwright 不总报 Chromium 开的标签页、它等那次导航才报、本机约 25 次一次）都成立，经得起 `b2-r000`、`b2-r003`；`b3-r009` 落在"the empty first document"的缝上：新的 Nit L1B-N2。
- **C**：L1C-N1、N2、N3 关上；Q1 由负责人关上；M1、M2、N4、Q2 交本提交。`make lint-web` 过，L1 100/100（57 秒）；夹具查安静盖住原来显式的 `expectQuietPage`（同样几样，时机在测试结束、窗口更长）。新的 Nit L1C-K1-N1。

修复核对之后：`5ea4ade0` 改注释（L1B-N2）；不开第二轮（决定 547）。

## 门禁与 CI

- **门禁**（本机 arm64，经 `heavy.sh`，Chromium，4 个 worker，除注明的）：

  | 头 | 结果 |
  |---|---|
  | `89e2389a` 加改法（调查） | L1 (page) 0/200；开协议日志另 10 次，0/10。改前 80 次每次都有 `"disposition":"newTab"`、每个 context 两个 page target；改后没有 `newTab`，仍是两个 page target（原页与故事自己开的） |
  | `12e868bf`（实施者） | `make lint-web` 绿；L1 (page) `--repeat-each=200` 200 过；Firefox、WebKit 用临时配置各 10 次，20 次全过；`make e2e` 576 过、1 跳过，156 秒 |
  | `12e868bf`（审查） | C：`make lint-web` 过（e2e 的换文档门禁 189 个文件，逃生口 A15 ×3），L1 200/200（1.8 分钟），AS3 (page) 100/100（1.2 分钟），`app-links.test.ts` 10/10；A：L1 10 次全过；B：AS3 200 次 0 红（2.5 分钟） |
  | `1b01b1d9`、`d716d9a9`（实施者） | `make lint-web` 两个提交都绿；`make test-web` 194 个文件、4,616 个测试全过；L1 200/200；`make e2e` 576 过、1 跳过，159 秒 |
  | `d716d9a9`（核对） | A：L1 50 次全过，整个文件 5 格全过；C：`make lint-web` 过，L1 100/100 |
  | `5ea4ade0` | `make lint-web` 绿 |

- **分支的 CI**（普通推送，没有改写已推的历史）：

  | 推送 | 头 | `ci.yml` | 结果 |
  |---|---|---|---|
  | 1 | `12e868bf` | 43989817910 | 19 项第一次 attempt 全绿；e2e 9:17（E2E 一步 7:27），race 14:35，web 3:06，image 1:27 |
  | 2 | `d716d9a9` | 44001727074 | 19 项第一次 attempt 全绿；e2e 8:36（E2E 一步 6:30），race 14:45，web 3:07，image 1:28 |

- `5ea4ade0` 与合并 `main` 的 `56537688` 没有单独推送，分支 CI 在本提交上跑。
- **本提交自己的分支 CI**：`0b227025` 的 run 44077138544，19 项第一次 attempt 全绿（web 3:49，e2e 9:33）。旁支合并为 `7ce00077`，之后第一次 `main` 的 run 44104429780 同样全绿（web 3:56，e2e 8:11；L1 (page) 过）。

## 环境

调查在分离的工作树 `nerve-wiki-m12-l1-flake`（`89e2389a`），改法存成 diff 之后还原、删掉了工作树；实施在 `nerve-wiki-m12-e2e-l1`；审查与核对各在 `nerve-wiki-m11-p1-s1-rev-a`、`-b`、`-c`（`12e868bf`，核对时 `d716d9a9`），`pnpm install --frozen-lockfile --prefer-offline`，跑 e2e 之前 `make build`。重的都经 `heavy.sh`，每项之前看 `df`（可用 24–25 G）；`TMPDIR` 与日志在 `~/.cache/m12-l1-*`，用完删掉。变异由脚本改文件、从 `.mutbak` 复原，探针的文件删掉；审完各工作树干净，A、C 的 `bin/nervewiki` 最后从没改的代码 build。本机 aarch64（Parallels 虚机），`chromium_headless_shell-1243`（HeadlessChrome/153.0.8010.12），Playwright 1.63.0，不重试。调查者经 `gh` 只读 CI 的历史；审查 C 挪工作树时 `git fetch` 过一次分支（只读），之后与 A、B 都没有上网。要留的在设计工作目录的 `m12/l1-flake/`（`host-facts.md`、`review-*.md`、`check1-*.md`、`a/`、`b/evidence/`、`c/logs/`）。
