# M12/P1 旁支 `m12-p1-navigation-settle`（帮手不再等一个已离开的文档的请求）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p1-navigation-settle`，从 `main` 的 `4190bac0`（S2 的合并）开始（主持者的决定 356）。实施的 3 个提交，审查时在 `cb13cc9a`：<br>`settle` 读完页面后再看在途的请求、A15 第 10 格 `d8cbb216`；新文档提交时清掉旧文档留下的请求、A15 第 11 格、门禁的表 A15 由 2 改为 3 `860bf289`；A15 第 3 格在 Firefox 上确定 `cb13cc9a`。<br>修复一轮在它之上 2 个提交，头 `5db08ef1`：读完页面为空时隔一个 `pollMs` 再看 `b48e8c26`；A15 第 12、13 格，第 11 格的第一个地址带 `#` `5db08ef1`。<br>修复核对之后：改注释一个 `57d883d4`；把 `main`（`ad0bf40c`，分配守卫的旁支 `m12-p1-prepared-alloc` 的合并）合进来一个 `ea86d371`。<br>到 `5db08ef1` 3 个文件（`e2e/fixtures/navigation.ts`、A15、`tools/e2e-navigation/check.mjs`），加 306 行、减 20 行；只改 e2e，不改产品 |
| 审查方式 | 三位审查者（Opus）并行，各在自己的分离的工作树里（挪到 `cb13cc9a`），重的都经 `heavy.sh`；探针临时放进 `e2e/stories/identity/`（才进三种浏览器），跑完删掉；不碰实施者的工作树：<br>A 帮手（`navigation.ts`）：两处改法的每条路径；导航请求的地址与提交的配对（15 种地址、`reload`、`location.assign(location.href)`，三种引擎）；同文档的移动、重定向、iframe、`request.frame()` 抛错；与 Playwright 1.63 的 `_onClearLifecycle` 对照；读页面末尾发出的读的路由探针。<br>B 测试、变异与三种浏览器：第 10、11 格与改过的第 3 格在旧帮手、旧第 3 格上是否红，是否确定（12 个 worker 加压；第 3 格 Firefox 默认 30 轮、8 个 worker 80 轮）；实施报告的变异表与自加的 nA、nE、nF；三个提交逐个 `lint-web`、`knip`、`check:types`；全量一次。<br>C 第 3 格、门禁与文档草稿：第 3 格的两处改动改没改它要证的事（读 `routes.tsx`、`guards.tsx`、`components/loading.tsx` 与实施者的探针日志）；门禁的表，三个提交从 `git archive` 的树上各跑一次；实施报告第 10 节的草稿与代码、设计、CI 的 trace 逐句对照 |
| 日期 | 2026-11-15（实施、审查、修复一轮、修复核对一轮） |
| 结论 | Important 0。Minor 3：页面上有路由时，读完页面马上再看会漏读页面期间发出的请求（NAVA-M1）；认提交的按地址配对与提交之后复位没有测试（NAVB-M1）；照实改的清单漏了写着旧个数、旧做法的地方（NAVC-M1）。Nit 3（NAVC-N1–N3）。三位都确认：提交时清空是对的，同文档的移动不清；第 10、11 格三种浏览器确定，在旧帮手上红；第 3 格的改动只让测试稳，证的仍是 U1；门禁的表由 2 改 3 对；三个提交各自为绿，全量 571 个。代码一侧由修复一轮 2 个提交改，文档一侧在本提交。修复核对一轮：三位都没有 Important、Minor，第一轮的发现都关上；新的 Nit 六条（NAVA-N1 = NAVC-N4，去重五条），NAVA-N2 由修复核对之后的注释提交改，其余在本提交 |

## 起因

- **LW3 (page) at 375 wide**：S2 的文档提交（`8a8f8d7c`）的分支 CI run 43686031128，第一次 attempt，Chromium。`lw3-source-duplicates.spec.ts:339` 的 `reload` 报 "goto, reload: after 10000 ms the page still waits for GET /api/v0/notebooks/<id>/assets"。服务端每个 `GET …/assets` 都很快答了（最后一个 2 ms）。trace 里那一个在帮手读页面（`sessionWaits`）的 12.7 ms 里发出，响应头到了（`wait` 6.955 ms）、响应体没有（`receive` −1），随后 `goto(raw)` 换了文档。第二次 attempt 只重跑 e2e，为绿。
- **A15 第 3 格（Firefox）**：S2 合并之后 `main` 的第一次 CI run 43689756257，第一次 attempt。带逃生口离开之后页面停在 `/settings/security`、仍登录着，`expectSentBackToSignIn` 等不到 "Sign in"。本机 S1 的帮手与本分支的帮手上，第 3 格 Firefox 都是 30 轮 1 次。第二次 attempt 只重跑 e2e，为绿。
- 主持者的决定 356：S2 照常合并；帮手开这个旁支修，照常审查，在 `docs: M12/P1 done` 之前合并；第二件一并交给它。定因与改法在 [P1 设计](../01-P1-foundation.md) 3.17。

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `cb13cc9a` 的。修复一轮照主持者的修复说明（`p1nav/review/fix-1.md`）在 `cb13cc9a` 之上加 2 个提交，普通推送、不改写已推的历史，每个单独为绿。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| NAVA-M1 | 中（帮手） | 页面或上下文上有任一路由时，Playwright 开着拦截，要等浏览器的 `requestPaused` 才报 `request`（`playwright-core` 1.63 `coreBundle.js:36147-36160`）。页面在读的末尾发出的请求，可能在读的答复之后才报到，`d8cbb216` 的"读完再看一次"看不见它，帮手带着它在途就换了文档：A 的路由探针在头上 Chromium 1/30、Firefox 1/10 漏，没有路由的一组 0。第 10 格让读发出之后 300 ms 才答，测不到这一种。报告与草稿说"读之后到 `page.goto` 发出之间还有几毫秒的窗口"，实际一直到下一个文档提交（Chromium、Firefox 在那之前照样跑旧文档） | `b48e8c26`：读回来为空时先等一个 `pollMs`（25 ms）再看，同一轮都空才放行；`settle` 的说明照实写：有路由时请求报得晚，之后直到下一个文档提交发出的请求帮手都不等，提交时清掉。偶发，没有确定的格（主持者照 fix-1 不另加格）：改前 240 次漏 3 次，改后 210 次 0 次。核对 A：关上 |
| NAVB-M1 | 中（测试） | `followRequests` 认提交的三样里，"与最后一个文档请求的地址配对"与"提交之后复位"没有测试：不比地址（nA）、不复位（nE），A15 33 个三种浏览器各三轮全照过；比地址时不去掉提交的 `#`（nF）也照过。实施报告第 6 节对"去掉配对时何时不同"说反了：不提交的导航之后，同文档移到任何别的地址都会被误清 | `5db08ef1`：照 B 的两格探针补第 12 格（204 之后同文档移到别的地址）、第 13 格（同文档移到它提交的地址），第 11 格的第一个地址带 `#a15`；`followRequests` 里地址那一句的注释照 B 改正。第 12 格不再断言答复是 204：Firefox 对这种导航请求不报答复，改为等它结束、断言页面没动。nA、nE、nF 各由一格抓到。核对 B：关上 |
| NAVC-M1 | 中（文档稿） | 草稿"照实改的各节"漏了：01-P1 的第 0 节、3.4、3.6 的格数、逃生口一条、第 3 格的做法、"为什么确定"、第二张表，3.14，7.1，7.4；00-M12 的 4.4 两处、第 7 节 S1 一行、决定 356、第 12、13 节 | 修复报告 5.4 补全；本提交照它改，另加决定 358（NAVC-N5）。核对 C：关上 |
| NAVC-N1 | 细节（文档稿） | 第 3 格的第一种失败，草稿把推断（"导航开始那一刻发起的一问，Firefox 既不发出也不报失败"）写成事实；05-lessons 的候选同。看得到的只是"最后一问答完之后旧文档再没有请求、也没有失败"，日志里下一问应比逃生口早 13–60 ms 发起 | P1 设计 3.17 写事实加推断；05-lessons 写成不依赖原因的一条。`cb13cc9a` 的说明与 A15 `holdTheFirstRefresh` 的注释仍是那一说法，已推、不改写。核对 C：关上 |
| NAVC-N2 | 细节（文档稿） | 10.2 补记的 `make e2e` 153 秒是 `860bf289` 的，不是头的 | 照头上的写（`5db08ef1`：577 个、159 秒）。核对 C：部分关上，余下的在 N6 |
| NAVC-N3 | 细节（文档稿） | 3.17 草稿没写"按地址认提交"没有测试 | 由 NAVB-M1 的两格关上；3.17 写明三样各由哪一格钉住。核对 C：关上 |
| NAVA-N1 = NAVC-N4 | 细节（决定与草稿，修复核对） | 决定 358 与 3.17 草稿说"今天的应用没有 `replaceState`"：应用经 React Router 8.4 的 replace 调 `history.replaceState`（17 处：守卫的 `<Navigate replace>`、`setParams(…, { replace: true })`、`navigate(…, { replace: true })`） | 本提交：写成"都是渲染时换到别的地址、或由用户的选择触发，没有在请求下一个文档之后、以同一地址 replace 的"；两位都没找到今天会咬到的故事，结论不变 |
| NAVA-N2 | 细节（注释，修复核对） | `followRequests` 的注释说"下载不提交"：WebKit 把路由答的下载当文档显示、提交（修复一轮的探针 H） | 修复核对之后的注释提交 `57d883d4` 改为 "a 204, or a download the browser does not show"；P1 设计 3.17 同 |
| NAVC-N5 | 细节（文档稿，修复核对） | 修复报告 5.4 的清单没有决定 358 | 本提交：00-M12 第 11 节加决定 358 |
| NAVC-N6 | 细节（文档稿，修复核对） | 10.2 补记没跟上第一轮修复：仍写"三个提交""十一格 96 通过""run 43691920310" | 本提交照修复报告第 3、4 节写（P1 设计 10.1 的旁支一条） |
| NAVC-N7 | 细节（文档稿，修复核对） | 05-lessons 的新候选写"读完页面再看在途的请求要隔一个轮询再看"，像是隔 25 ms 就够了；滞后没有上界 | 本提交写"隔一个轮询再看能少漏（240 次漏 3 次对 210 次 0 次），消不掉" |

## 主持者的决定

负责人可以改判：

- **356**（S2 合并之前定，00-M12 第 11 节）：S2 的文档提交的分支 CI 红在 S1 的帮手的竞态上，只重跑 e2e、照常合并 S2；帮手开这个旁支修，在 `docs: M12/P1 done` 之前合并，照常审查（照 M11 决定 249 的先例）。S2 合并之后第一次 `main` 的 e2e 又红在 A15 第 3 格（Firefox），一并交给这个旁支。
- **358**（修复核对之前定）：下一个文档请求之后、提交之前，旧文档以同一地址 `replaceState` 时，帮手把它当成提交，提前清空、忘掉地址，之后旧文档再发的读留下（Chromium、Firefox；审查 A 的怀疑，修复一轮的探针 G 核实）。不改帮手，作为已知的窄口写进 P1 设计 3.17。理由：应用经 React Router 的 replace 调 `replaceState`，但没有一处在请求下一个文档之后以同一地址 replace（NAVA-N1 = NAVC-N4 的写法）；补法"文档请求结束时才忘地址"要先证三种浏览器里文档请求的结束一定晚于提交，否则漏清；下载在 Firefox 报 `requestfinished`、不提交，补不全。修复核对 A 另想了"这个导航请求有了答复才把同地址的 `framenavigated` 当提交"，同样要先证次序，Firefox 对 204 的导航又不报答复，不比它稳。
- 修复一轮（`fix-1.md`）：NAVA-M1 照 A 的改法，偶发、不另加格；`cb13cc9a` 的 run 里 `server (race)` 红在 `TestPreparedSourceDoesNotCopyTextForComparison`，与本分支无关，不重跑，另一个旁支 `m12-p1-prepared-alloc` 修；它合进 `main`（`ad0bf40c`）之后把 `main` 合进本分支（`ea86d371`）。
- 修复核对之后不再开第二轮：三位都没有 Important、Minor；NAVA-N2 由注释提交 `57d883d4` 改，其余的 Nit 在本提交。

## 实施中定的

实施报告第 9 节与 11.6（三位审查者各在自己的范围里判过，都对）：

- 认新文档用"主框架的导航请求与它的地址提交"，不用 Playwright 内部的 `newDocument`：公开接口里 `Request` 没有文档的标识，`framenavigated` 也不分同文档与新文档。
- 第 10 格以换掉页面的 `LockManager.prototype.query` 造出"帮手读页面时发出的读"。
- 第 11 格以扣住下一个文档 1 秒造出"请求之后、提交之前旧文档发的读"；WebKit 里那个读发不出，断言随浏览器写；第 11 格经逃生口，门禁的表写 3。
- 第 3 格离开之前等"Loading…"，不等某个模块文件的请求：它不依赖构建出来的文件名，也是用户看得见的状态。

## 接受与推后的

- **接受**：
  - LW3 本身在本机没有复现：旧帮手上 LW3 "at 375 wide" 三种浏览器各 20 轮，60 个全过（窗口十几毫秒）。确定的复现是第 10、11 格。
  - n1、n4、旧帮手在第 11 格的 WebKit 上照过：WebKit 在请求下一个文档之后旧文档发不出请求，本来就留不下。
  - "等一个 `pollMs`"挡不住超过 25 ms 的滞后，没有确定的格；代价是每次没什么可等的调用多 25 ms（全量约 485 次调用，CI 的 2 个 worker 上约 6 秒；本机 `make e2e` 155 → 159 秒）。修复核对 A 判过两种别的关法：在页面里对账（包一层 `fetch` 与 XHR 计数）要改被测的页面、还要等答复体，读之后到提交的那一段照样消不掉；多读一次页面挡不住 `requestPaused` 的滞后。都不更值得。
  - 第 10–13 格的 1 秒是"帮手等不等"的界（审查 B 的未核实），12 个 worker 下仍在界内；只影响变异抓不抓得到。
  - 第 3 格的两种失败是偶发的，各自的变异（只改一样，varA、varB）抓到的是比率，不是每一轮。
  - 帮手只在提交的地址与最后一个导航请求对得上时清；没有导航请求的新文档提交（`about:blank`、`data:`、`setContent`）不清。今天的故事里没有，14 处后退与前进都在同一个文档里（审查 A）。实施报告第 1.2 节说"与 Playwright 一致"宽了一点：同在新文档提交那一刻清，范围不同。
  - `cb13cc9a` 的说明与 A15 的注释把第 3 格第一种失败的原因写成事实（NAVC-N1），已推、不改写。
- **推后、交出去的**：
  - 有路由时，旧文档在提交之前最后几毫秒发的请求，它的 `request` 可能晚于提交才报到（审查 A 的怀疑 1）：那样它在清空之后才进集合，Chromium 不再报它，下一次换文档等满 10 秒，即 LW3 那种失败的窄版本。实施者与审查者都没有造出来，全量 577 个与 LW3 各 20 轮里都没见到。
  - 决定 358 的窄口。
  - 答 204、或浏览器不显示的下载之后，同文档恰好移到那个地址会被当成提交而误清；在文档请求的 `requestfailed` 时忘掉地址能盖住 204 与 Chromium 的下载，盖不住 Firefox 的下载（它报 finished）。今天没有故事这样，帮手没有改。

## 各浏览器对"文档走了的请求"报什么

实施者的探针（`p1nav/probes/`；`logs/probe1.log`、`probe2.log`、`mut-e2e-final.log`）：

| 情形 | Chromium | Firefox | WebKit |
|---|---|---|---|
| B：应用自己的事件流（答了，响应体没完），之后 reload | 不报结束也不报失败 | `NS_BINDING_ABORTED` | `Load request cancelled` |
| C：路由扣着的读，在逃生口 reload 之前发出 | 不报；下一个 `goto` 等满 10 秒失败 | 报失败，帮手 98 ms 就走 | 报失败，56 ms |
| E：答复的体页面没读（`fetch` 之后不 `.text()`），文档还在 | 不报 `requestfinished` | 报 finished | 报 finished |
| E 对照：读了答复的体 | finished | finished | finished |
| F：路由扣着的读，之后页面自己 `location.assign` 换文档 | 下一个 `goto` 等满 10 秒失败 | 报失败，帮手 99 ms 就走 | 报失败，53 ms |
| 下一个文档已请求（被扣 1 秒）、还没来时，旧文档发的读（第 11 格） | 不报 | 也不报 | 旧文档发不出 |

修复一轮的探针 H（`logs/probe-gh.txt`），不提交的导航请求：

| 情形 | Chromium | Firefox | WebKit |
|---|---|---|---|
| 答 204 | `requestfailed` `net::ERR_ABORTED` | `requestfailed` `NS_BINDING_ABORTED` | `requestfailed` "Aborted: 204 No Content" |
| 下载（`content-disposition: attachment`，路由答的） | `requestfailed` `net::ERR_ABORTED` | `requestfinished`，不提交 | 提交了（当文档显示） |

探针 G：reload 的文档请求扣 1 秒，旧文档在第 300 ms `replaceState(location.href)`、第 400 ms 发一个永不答复的读，再经帮手 `goto`。Chromium、Firefox 的次序是"同文档的 `framenavigated`（与记下的地址对上，提前清空、复位）→ 读 → 真正的提交（不清）"，帮手等满 10 秒失败；WebKit 97 ms 就走（决定 358）。

## 反向对照

工具：实施者的 `tools/mut_e2e.py`、审查 B 的 `b/mut.py`（改一处、断言旧文字恰好一处、从 `.mutbak` 复原、核对 sha256）。都经 `heavy.sh`，A15 三种浏览器各三轮；先在没改的树上跑同样的命令；之后工作树干净、没有 `*.mutbak`。

**实施时**（头 `860bf289`，十一格；`logs/mut-e2e-summary.txt`）：

| 变异 | 失败的格（每种 3/3） | 报错 |
|---|---|---|
| 旧帮手（`4190bac0` 的 `navigation.ts`） | 第 10 格三种；第 11 格 Chromium、Firefox | "the next document is asked for once the read was answered"；"after 10000 ms the page still waits for GET /api/v0/instance" |
| n1 提交时不清空 | 第 11 格 Chromium、Firefox | 等满 10 秒 |
| n2 读完页面后不再看在途的集合 | 第 10 格三种 | 下一个文档在读放开之前就请求了 |
| n3 主框架的每次 `framenavigated` 都清（同文档的移动也清） | 第 11 格三种 | "once this document's read was answered"：`pushState` 清掉了在途的读 |
| n4 请求下一个文档时清，不等它提交 | 第 11 格 Chromium、Firefox | 等满 10 秒 |

**修复一轮**（"改前"是 `cb13cc9a`，审查 B 的 `b/batch.out`；"改后"是头 `5db08ef1`，十三格，`logs/mut-fix1-summary.txt`）：

| 变异 | 改前 | 改后失败的格（每种 3/3） |
|---|---|---|
| nA 不比地址：文档请求之后主框架的第一次 `framenavigated` 就清 | A15 33 个全过 | 第 12 格三种 |
| nE 提交之后不复位 | A15 33 个全过 | 第 13 格三种 |
| nF 比地址时不去掉提交的 `#…` | 全过 | 第 11 格 Chromium、Firefox（等满 10 秒；WebKit 发不出留下的读） |
| n1 | 第 11 格 Chromium、Firefox | 同左 |
| n2 | 第 10 格三种 | 同左 |
| n3 | 第 11 格三种 | 第 11、12、13 格三种 |
| n4 | 第 11 格 Chromium、Firefox | 同左 |
| 旧帮手 | — | 第 10 格三种、第 11 格 Chromium 与 Firefox；第 12、13 格照过（旧帮手不清，也就不误清） |

**NAVA-M1**（偶发，按比例；A 的路由探针 `review/a/a15-zprobe-nav.spec.ts.txt`：在帮手读页面的那次 `navigator.locks.query()` 里发一个读，看帮手是否带着它在途就请求了下一个文档）：

| 树 | Chromium | Firefox | WebKit |
|---|---|---|---|
| 改前（读完立刻再看），审查 A | 1/30 + 0/10 | 1/10 + 0/30 | 0/40 |
| 改前，修复一轮各 40 轮 | 1/40 | 0/40 | 0/40 |
| 改后，审查 A 的 `look-after-a-poll` | 0/30 | 0/30 | 0/30 |
| 改后（头），修复一轮各 40 轮 | 0/40 | 0/40 | 0/40 |
| 改后（头），修复核对 A 各 30 轮 | 0/30 | 0/30 | 0/30 |

改前 240 次漏 3 次；改后 300 次漏 0 次。

**第 3 格**（Firefox，经 `heavy.sh`；`logs/var-*.txt`、`c3-*.txt`）：

| 变体 | 运行 | 结果 |
|---|---|---|
| 两样都不改（加记录） | 60 轮、80 轮，8 个 worker | 发不出续期 1、4 次，路由错误 2、2 次 |
| 只等"Loading…"（varA） | 80 轮，8 个 worker | 78 通过：2 次发不出续期，0 次路由错误 |
| 只改闸门（varB） | 80 轮，8 个 worker | 77 通过：0 次发不出续期，3 次路由错误 |
| 两样都改（varAB） | 80 轮，8 个 worker | 80/80 |
| 提交的写法：第 3、4、7 格，Chromium 与 Firefox | 各 40 轮，8 个 worker | 240/240 |
| 提交的写法：第 3 格 | 30 轮，默认 worker | 30/30 |
| 旧第 3 格（`860bf289` 的 A15），审查 B | 80 轮，8 个 worker | 8 次失败（6 次停在 `/settings/security`，2 次路由错误） |

## 核对

**修复核对一轮**（2026-11-15）：三位各把自己的工作树挪到 `5db08ef1`，之后干净、没有 `*.mutbak`。都没有 Important、Minor。

- **A**：M1 关上：头上自己的路由探针三种各 30 轮都 0 漏；说明照实。代价每次 25 ms，全量串行约 12 秒、CI 的 2 个 worker 约 6 秒，与本机 155 → 159 秒、CI 的 E2E 一步 5:45 → 5:58 相符。三条怀疑的处理同意（第 1 条照旧未核实，第 2 条照决定 358，第 3 条帮手不改）。新 Nit 两条：NAVA-N1（"没有 `replaceState`"不对）、NAVA-N2（注释说"下载不提交"）。
- **B**：M1 关上：头上 nA 第 12 格、nE 第 13 格三种各 3/3 失败，nF 第 11 格 Chromium、Firefox 各 3/3 失败；第 12 格不断言 204 之后仍证同一件事（nA 三种都红，说明那次不提交的导航确实记下了地址）。第 11–13 格三种各 20 轮 180/180；12 个 worker、各六轮 228 通过、6 跳过、0 失败。`b48e8c26`、`5db08ef1` 各自 `lint-web`、`knip`、`check:types` 为 0，A15 32、38 个通过、1 个跳过。全量 577 个（576 通过、1 跳过），157 秒。没有新发现。
- **C**：M1、N1、N3 关上，N2 部分关上。门禁的表仍是 A15 ×3（逃生口在 `a15-leave-mid-refresh.spec.ts:218`、`:280`、`:606`），第 12、13 格不经逃生口；第 12 格 `location.href = "/a15-no-content"` 门禁看不见（决定 344 接受），这次导航答 204、不离开文档，不算违例。两个提交从 `git archive` 的树上门禁 rc=0、175 个文件。新 Nit 四条：NAVC-N4（同 NAVA-N1）、N5（清单没有决定 358）、N6（10.2 补记没跟上修复）、N7（05-lessons 说得比证据满）。

一轮之后没有 Important、Minor 留下，不开第二轮修复。

## 门禁与 CI

- **逐个提交**（实施者在干净的临时 worktree 里逐个检出，经 `heavy.sh`；`logs/percommit.out`、`percommit-c3.out`、`fix1-percommit.out`）：

  | 提交 | `make lint-web`（含 `check:types`） | `make knip` | 故事 | 门禁 |
  |---|---|---|---|---|
  | `d8cbb216` | 0 | 0 | 全部：567 通过、1 跳过 | A15 ×2 |
  | `860bf289` | 0 | 0 | 全部：570 通过、1 跳过 | A15 ×3 |
  | `cb13cc9a` | 0 | 0 | A15 三种：32 通过、1 跳过 | A15 ×3 |
  | `b48e8c26` | 0 | 0 | A15 三种各三轮：96 通过、3 跳过 | A15 ×3 |
  | `5db08ef1` | 0 | 0 | A15 三种各三轮：114 通过、3 跳过 | A15 ×3 |

  每个检出之后都没有被改动的跟踪文件。审查 B 另逐个跑了前三个的 `lint-web`、`knip`、`check:types` 与 A15。
- **合并之前的门禁**（本机，经 `heavy.sh`）：

  | 头 | `make lint gen-check knip build` | `make e2e` | A15 三种各三轮 | LW3 "at 375 wide" 三种各 20 轮 | LW3 整个文件各 5 轮 |
  |---|---|---|---|---|---|
  | `860bf289` | 25 秒 | 571 个（570 通过、1 跳过），153 秒 | 96 通过、3 跳过，61 秒 | 60/60 | — |
  | `cb13cc9a` | 10 秒 | 571 个，155 秒 | 96 通过、3 跳过，62 秒 | 60/60 | 135/135 |
  | `5db08ef1` | 9 秒 | 577 个（576 通过、1 跳过），159 秒 | 114 通过、3 跳过，74 秒 | 60/60 | 135/135 |

  门禁报出 175 个文件，没有直接的 `goto`、`reload`，逃生口 A15 ×3。第 50 条那一行每次都打出（waited 534、311、633 ms）。
- **分支的 CI**（普通推送，没有改写已推的历史）：

  | 推送 | 头 | `ci.yml` | 结果 | e2e（E2E 一步） |
  |---|---|---|---|---|
  | 1 | `860bf289` | 43691920310 | 19 项第一次 attempt 全绿 | 7:14（5:45，571 个；第 10 格 1.4–2.5 秒、第 11 格 2.5–3.3 秒；waited 730 ms） |
  | 2 | `cb13cc9a` | 43694312336 | 三次 attempt 都是 `server (race)` 红在 `TestPreparedSourceDoesNotCopyTextForComparison`（`TotalAlloc` 2,462,776、4,631,752、2,561,368 字节，上限 2,097,152；与本分支无关，另一个旁支修），`image`、`e2e` 跳过，其余 16 项绿 | 没有跑 |
  | 3 | `5db08ef1` | 43705519890 | 19 项第一次 attempt 全绿，`server (race)` 也绿 | 7:24（5:58，577 个；A15 38 个通过；第 3 格 Chromium 0.5、Firefox 0.9 秒，第 3 格的改动第一次在 CI 上跑 e2e；waited 213 ms） |

- **合并 `main` 之后的分支 CI**：`ea86d371`，run 43721745639，19 项第一次 attempt 全绿，`server (race)` 也绿；e2e 作业 8:06；`server (guards-sources)` 在 EPYC 9V74 上绿（服务端与 `ad0bf40c` 相同，只差 e2e 的三个文件；`main` 上 `ad0bf40c` 的这一项在 7763 上红，见 00 号文档决定 359）
- **本提交自己的分支 CI**：run 43726327301，19 项第一次 attempt 全绿（`server (race)` 11:09，`server (guards-sources)` 13:46，e2e 9:45，E2E 一步 8:12）

## 环境

实施、审查与核对各在自己的工作树里，`TMPDIR` 与日志在 `~/.cache` 下各自的目录，锁固定在 `/tmp`，用完删掉。探针与审查者的变异都在自己的工作树里跑，跑完删掉或从备份复原；没有提交、推送或重跑别人的 CI（`cb13cc9a` 的两次重跑是实施者对自己的 run 做的，修复说明之后不再重跑）。CI 的作业日志与产物（`~/.cache/s2-e2e-art/`、`~/.cache/s2-e2e-art-main1/`）只读。要留的汇总在设计工作目录的 `m12/p1nav/`（`logs/`、`probes/`、`review/a/`、`review/b/`、`review/b-check1/`、`review/c/`）。
