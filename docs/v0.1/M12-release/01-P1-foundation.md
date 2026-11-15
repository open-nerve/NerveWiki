# M12/P1 地基：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M12/P1 地基（两个 Step：S1 e2e 的帮手；S2 工具链与安全更新） |
| 日期 | 2026-11-15（初稿；同日照设计审查修订，再照修订核对第 1、2 轮修订；同日照 S1、S2 的实施与审查修订） |
| 状态 | 进行中（设计已审查，S1 已合并 `bbf3eb26`，S2 已合并 `（待填：S2 的合并号）`；设计的[审查记录](reviews/P1-design-review.md)，两轮修订核对在它的末两节；S1 的[审查记录](reviews/P1-S1-e2e-navigation-review.md)；S2 的[审查记录](reviews/P1-S2-toolchain-review.md)；实施与审查之后与代码不同之处照实改在各节（3.15、3.16 列出），结果在 10.1、10.2） |
| 上级文档 | [M12 总设计](00-M12-design.md) 4.1、4.3、4.4、第 3 节第 1、2、11 条、5.3、第 7 节 P1 一行与"分份""移交的落实""总体设计的修订"第 4、5、12、18 条、9.3、9.6、9.7、第 10 节、第 11 节决定 308、310、311、334；[P1 设计审查](reviews/P1-design-review.md)；[M12 设计审查](reviews/M12-design-review.md)的 C-I3、C 的 U1–U3、B-I1、B-I2、B-M1、B-M3、B-M12、B-M13、B-M15、B-N9、B-N11、B-N12、C-N8；[移交](handoffs/)：M5-polish 第 50、51 条，M0-P6-image-release 第 4 条；[v0.1 总体设计](../v0.1-design.md) 7.1、13.1 第 32 条、13.4 第 1、3 条、13.5 第 2 条；[M11/P1](../M11-upgrade/01-P1-freeze-start-rollback.md)（写法） |
| 代码位置的写法 | 同 00 号文档：模块写作 `page/domain/grep.go:149`（`server/internal/modules/` 之下）；`platform/…` 在 `server/internal/` 之下；前端写作 `W/session/token-manager.ts:289`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/fixtures/browser.ts:123`；其余写仓库根起的路径。行号都是 `main` 的 `a22adfd2` 的（代码与 `5e8af5b5` 相同，其后只改了文档与 `docs/v0.1/claude_to_codex/tools/`），00 号文档与总体设计的行号也是它的 |
| 工作笔记 | 本文引的每个行号都在 `a22adfd2` 上读过（附录）。P1 的设计不跑重的东西，要量的在 7.5；第一次修订的 A15 原型是唯一经 `heavy.sh` 的运行。只读地查了公开的版本：go.dev 的下载列表（最新 go1.27.4）、proxy.golang.org（`golang.org/x/vuln` 最新 v1.9.0）、四个镜像的摘要（`docker buildx imagetools inspect`）、pnpm 12 的 `audit` 文档；没有跑 `govulncheck`、`pnpm audit`、`pnpm outdated`。续期的两个探针的数据是 M12 设计审查 C 的（00 号文档 4.4、`m12/review-c/`）；各引擎在"下一个文档已请求"时的行为是 P1 设计审查 C 的探针（`m12/p1-review-c/`），`sessionWaits` 在三个窗口里的读数是 P1 设计审查 A 的（`m12/p1-review-a/`）。第一次修订在 scratchpad 的临时 worktree（`a22adfd2`）里跑了 A15 新的第 3、4、6 格的原型，三种浏览器各三轮，帮手完整与去掉一样各一组（经 `heavy.sh`；`m12/p1-rev1/`，3.6） |

---

## 0. 分两个 Step

照 00 号文档第 7 节，P1 分两个 Step，各自一个分支（`m12-p1-s<m>`）、一次审查与修复核对、一次合并；S2 从 S1 合并之后的 `main` 开始。每个提交单独为绿（02-workflow 第 3.1 节）。

- **S1 e2e 的帮手**（`m12-p1-s1`）：
  - 帮手 `e2e/fixtures/navigation.ts`：`goto`、`reload` 先等页面没有在途的接口请求、续期的锁空着（没有 Web Locks 时看租约）、这个文档有了给它令牌的答复（第一次续期，或在这个文档里登录、注册），再换文档；显式的逃生口 `{ inFlight: "leave" }`（3.2）；
  - `watchPage` 登记每个页面的在途请求，并记下页面落到 `/sign-in` 的每一次（3.3）；
  - 110 个文件换上帮手：109 个故事、`e2e/fixtures/wiki-health.ts`（3.4）；
  - 门禁 `tools/e2e-navigation/check.mjs`，进 `make lint-web`：整个 `e2e/` 里只有帮手的模块直接调 `goto`、`reload`；逃生口按表核对个数；先在样例上证明它看得见违例（3.5）；
  - 新故事 A15：离开续期在途的页面会被登出，经帮手不会；九格，三种浏览器，第 3 格只在 Chromium 与 Firefox（3.6）；
  - `e2e/global-setup.ts` 的网络等待触发时打一行日志（3.7，第 50 条）；
  - **单独为绿**：只改 `e2e/`、`tools/`、`Makefile` 的 `lint-web` 一行与 `knip.jsonc`。产品代码、契约与组合根不变。CI 的 `web` 一项（`make lint-web`、`make knip`）与 `e2e` 一项证明它。
- **S2 工具链与安全更新**（`m12-p1-s2`）：
  - Go 1.27.4：两份 `go.mod` 的 `toolchain`，Dockerfile 的 golang 镜像的 tag、摘要与构建时的断言，README 的版本，一个短测试把它们钉在一起（3.9）；
  - PostgreSQL 18.7：五处，守卫加上 README（3.10）；
  - node 与 distroless 的摘要、两个 action（3.11）；
  - `make vuln`、`make outdated`、新的 `.github/workflows/vuln.yml`（3.12）；`vuln.yml` 报出的模块与包（3.13）；
  - 实施与审查加的：`ci.yml` 只让竞态构建的九份取存 Go 的缓存（决定 355），第三、第五份各挪一个整个程序的测试给第八份（决定 354）（3.16 第 2、3 条）；
  - 两处代码注释与文档里写着 1.27.1 的五处；
  - **单独为绿**：一个分支、一次完整的 CI；`vuln.yml` 在同一个分支上由 `push` 触发，跑一次为绿。PostgreSQL 的五处由守卫一起改，漏一处就红。

**S1 先做**：S2 要一次干净的完整 CI，不该被 `goto` 的偶发打红（00 号文档第 7 节 P1）。

## 1. 基线

从 `main` 的 `a22adfd2` 开始（M12 总设计定稿的提交；它之前的 `b54cd679`、`8c247237` 只改了 `docs/v0.1/claude_to_codex/` 的文档与工具）。

**e2e 与续期**：

- **续期在文档开始时就发**：每个文档的 `main.tsx` 建一个会话、`session.start()`（`W/main.tsx:28-30`）。访问令牌只在内存里（`W/session/token-manager.ts:4-12`），所以有记录 `nwiki.auth`（`:16`）的文档一开始就续期一次（`#start` `:221-229`、`#firstRefresh` `:236`）。续期在锁 `nwiki.auth.refresh` 之下（`W/session/refresh-lock.ts:14`、`:20-22`；没有 Web Locks 时是 localStorage 里的租约 `nwiki.auth.lease`，`:25`，用完删掉，`:99-105`）：锁里读记录、发 `POST /api/v0/auth/refresh`、写回新的记录（`token-manager.ts:288-310`）。
- **打断续期就被登出**：服务端换了令牌、浏览器的文档已经走了，新的记录没写下；下一个文档拿旧的令牌续期，服务端认作重复使用，撤销会话，页面落到 `/sign-in`。M1 第 10 节接受这个后果（`docs/v0.1/M1-auth/00-M1-design.md:193`），产品不改（00 号文档 4.4）。`a5-refresh-reuse.spec.ts:38-66` 在一个文档里造出了它（续期的答复丢了）。
- **第一次续期常在 `load` 之后才发**：它要先等锁授予（`refresh-lock.ts:21`）。审查 C 的探针 1：`page.goto` 返回时，新文档的第一次续期还没发出的，Chromium 15 次里 9 次、WebKit 15 次里 8 次、Firefox 0 次（`m12/review-c/probe.log`）。探针 2 只等在途的请求，三种引擎各 20 次，没有一次落到登录页（`probe2.log`）。剩下的窗口是"换文档的请求已发出、旧文档恰在这时发出续期"（00 号文档 4.4"第一次续期"），**只在 Chromium 与 Firefox 有**：P1 设计审查 C 的探针 D 里，下一个文档被扣 1 秒，旧文档每 50 ms 发一次的请求在 Chromium 与 Firefox 各有 20–21 次到了路由，WebKit 0 次；探针 B 里 WebKit 旧文档的请求报 "TypeError: Load failed"（`m12/p1-review-c/probe-d.log`、`probe-abc.log`）。另外，Chromium 在下一个文档还没提交时，对旧文档的 `page.evaluate` 不返回（探针 B、C："no answer in 3000 ms"）。
- **调用**：`e2e/stories` 里 109 个文件、259 处 `.goto(`、10 处 `.reload(`（都在这 109 个文件里），14 处 `.goBack(`/`.goForward(`；夹具里一处 `.goto(`（`e2e/fixtures/wiki-health.ts:157`，`connectedHealth` `:146-…`），`e2e/global-setup.ts` 与 `playwright.config.ts` 里没有。接收者都是单个标识符（`page` 227 处，其余 `tab`、`b`、`matePage`、`reader`、`writer`、`adminPage`、`second` 等 19 种）。注释里没有写成 `.goto(`、`.reload(` 的文字。故事里没有名为 `goto`、`reload` 的标识符。
  - 带选项的一处：`lw8-health.spec.ts:701` 的 `{ waitUntil: "load" }`；用返回值的：`s4-routing-fallback.spec.ts:17`（`const document = await page.goto(…)`）；跨行的一处：`lw6-source-history.spec.ts:356`。
  - 14 处后退与前进逐处看过，都是单页应用自己的历史条目，同一个文档里的 popstate（附录"后退与前进"）。TR2 的那一处（`tr2-import.spec.ts:254`）在扣住导入的 POST 之后（`:241-259`），是 `useBlocker` 拦下的同一文档的导航（`W/pages/notebook/import-dialog.tsx:3`、`:64`）。
  - **关标签页**同样离开一个文档、会丢在途的续期：故事里六处关应用的标签页（`collab/c6-expired.spec.ts:75`、`collab/c3-release.spec.ts:82`、`collab/c9-one-stream.spec.ts:80`、`history/hi3-log-revert.spec.ts:381`、`links/l1-links.spec.ts:187`、`asset/as3-render.spec.ts:333`；`mcp/mc1-connect.spec.ts:172` 关的是 MCP 的客户端）。前五处关之前都已断言了要访问令牌才看得到的内容（C9 是持有事件流的那个标签页，事件流要令牌），第一次续期早已答完、写下；AS3 关的是 PDF 的弹窗，没有应用。今天没有风险，不进帮手与门禁；帮手与门禁的文件头各写一句，提醒以后的故事（3.2、3.5；P1 设计审查 C-N4 数了五处，C9 的 `holder?.close()` 是第一次修订补的）。
- **页面的监视**：`watchPage`（`e2e/fixtures/browser.ts:123-207`）在每个页面第一次导航之前登记事件：`apiRequests`、`apiFailures`（`:154-175`，事件流除外，`apiPath` `:109-112`）、控制台与 CSP。`page` 夹具（`e2e/fixtures/test.ts:144-158`）、`anotherPage`（`:214-245`）、`anotherTab`（`:250-273`）都经它；A4、A6 自己开的标签页也调它（`a4-refresh.spec.ts:114-115`、`a6-sign-out.spec.ts:46-47`、`:88`、`:120`），L1 的新标签页在 `reload` 之前调它（`l1-links.spec.ts:183-184`）。失败时 `pageState` 写出地址、焦点、请求与失败（`browser.ts:251-280`），`pageStatesOf` 把它写进错误，CI 的注解里看得见（`:296-314`）。它认不出"被跳回登录页"。
- **路由**：上下文第一次开拦截时，正在那一刻发出的请求 Playwright 什么事件也不报（`a4-refresh.spec.ts:65-68` 的注释；审查 C 的 U2）；去掉一页的最后一个路由会关掉拦截，正在发出的请求就挂住（05-lessons 第 6 节，`05-lessons.md:78`）。设计时逐个看了用路由又换文档的 14 个故事（附录"路由与导航"），没有一处在 `goto`、`reload` 时扣着请求不放：扣着不放的有 TR2 的导入（之后是后退）、PG3 的事件流（`pg3-move-page.spec.ts:189`，事件流不算在途）。
- **配置**：Playwright 不重试（`e2e/playwright.config.ts:6-29` 没有 `retries`）；`threeBrowsers`（`:3-4`）。默认的测试时限 30 秒，`expect` 10 秒（`:13`）。
- **第 50 条**：`networkSettled`（`e2e/global-setup.ts:46-58`）等主机上 tentative 的 IPv6 地址，最多 5 秒；`ip` 判不了时从 `startPostgres()` 返回起等 3 秒（`:55-57`）。触发与否都不打日志。
- **门禁的先例**：`make lint-web` 先跑 `node tools/md-fixtures/check.mjs`（`Makefile:159-165`），`knip.jsonc` 把它列为入口（`:7`）；`oxlint --max-warnings=0 tools web e2e`（根 `package.json:8`）。knip 管 e2e 的未用导出（`make knip` 今天为绿）。

**工具链**：

- **Go**：`toolchain go1.27.1`（`server/go.mod:5`、`server/tools/go.mod:5`）；`go 1.27`。Dockerfile 的 server 阶段 `FROM golang:1.27.1-trixie@sha256:3b77fc61…`（`deploy/Dockerfile:23`）。golang 镜像设了 `GOTOOLCHAIN=local`，`go 1.27` 让 1.27.1 也满足：只改 `toolchain` 时镜像悄悄用旧的 Go 构建，没有守卫（I1 第 5、8 节）。README 写"会自动下载 `server/go.mod` 指定的 Go 1.27.1"（`README.md:16`）。CI 的 `setup-go` 读 `server/go.mod`（`.github/workflows/ci.yml:148-150`、`:272-274`），之后一步打出 `go version`（`:155-157`）。
- **写着 1.27.1 的代码注释**：`platform/httpserver/mcpserver/edge.go:106`（`encoding/json/v2` 在默认打开的 GOEXPERIMENT `jsonv2` 之后；`:7` 导入它）；`page/domain/grep.go:149`（引了标准库 `regexp/syntax/parse.go:212`）。`page/adapter/postgres/search_guard_test.go:13` 也写着 Go 1.27.1，那是 M10 校准时的环境，照实留着。
- **写着 1.27.1 的文档**：`docs/v0.1/claude_to_codex/06-environment.md:10`、`:53`，`05-lessons.md:89`，`03-project-background.md:15`；总体设计 13.1 第 32 条（`v0.1-design.md:1179`）。
- **PostgreSQL**：`postgres:18.6-trixie` 在 `platform/postgres/pgtest/pgtest.go:35`、`deploy/compose.dev.yaml:8`、`e2e/fixtures/db.ts:13`、`deploy/image-smoke.sh:86`、`README.md:629`；守卫 `TestContainerMatchesTheOtherDatabases`（`pgtest/image_test.go:12-41`）核对前四处，不看 README。总体设计 7.1 的例子写着它（`v0.1-design.md:722`）。
- **基础镜像**：node `24-trixie-slim@sha256:8ec5d755…`（`Dockerfile:7`，24.21.0）、distroless `static-debian13:latest@sha256:58133991…`（`:41`）。2026-11-15 读到的：`node:24-trixie-slim` 是 `af02d57b57719b9657a45199a79264a9437940cd55b3c7b3fc99615654f124a0`（24.22.3，I1 第 5 节），`gcr.io/distroless/static-debian13:latest` 是 `1a64f46364620465ef9e9a8f5413757dfdf22695e050bbfd7303eeaf17a59383`，`golang:1.27.4-trixie` 是 `b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9`，`postgres:18.7-trixie` 是 `9f4fd4aa918f52ae3d6f3b687f1de90d8f867cd4d520021545a9a50c120eae3b`。
- **actions**：`ci.yml` 按 SHA 引用五个（`:144`、`:148`、`:184`、`:229`、`:239` 等）。`actions/setup-node` v7.0.0（`:229`、`:276`），最新 v7.1.0 `949feb2413d6458794dcd2491c4babbbce0c15c1`；`actions/upload-artifact` v7.0.1（`:184`、`:301`），最新 v7.0.2 `cf430e030ddbb5b0abf93d22962f4752f3646cd9`；checkout v7.0.1、setup-go v7.0.0、cache v6.1.0 已是最新（I1 第 5 节的表）。
- **漏洞扫描**：没有。`.github/workflows/` 只有 `ci.yml`。依赖的版本精确固定（总体设计 13.5 第 2 条）；`pnpm-workspace.yaml` 有 `minimumReleaseAge`、`trustPolicy`（`:11-20`），没有 `audit` 一节。
- **npm 一侧**：打进二进制的前端是 `@nervewiki/web` 的 `dependencies`（react、mermaid、katex、codemirror 一组等 27 个）与 `@nervewiki/api-client` 的 `openapi-fetch`；e2e 与根只有 `devDependencies`。pnpm 12.8.1 的 `pnpm audit` 有 `--prod`、`--audit-level`、`--ignore <GHSA>`、`--ignore-unfixable`（本机 `pnpm audit --help`，不联网）；文档写明忽略的设置是 `pnpm-workspace.yaml` 的 `audit.ignore`，级别也可写成 `audit.level`，低于级别的公告不打印、也不让它失败（pnpm.io/cli/audit）。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P1 一行（`:722-726`）：e2e 的帮手与门禁；工具链与安全更新、`vuln.yml`。

不做：P2 的打磨（它们的 e2e 断言照本 Phase 的帮手写）；P4 的实测（`e2e/perf/` 在 P4/S2，门禁照样管它）；P5 的发布流水线、版本号的守卫、actionlint（P5/S1；本 Phase 的 `vuln.yml` 那时由 actionlint 一起核对）；产品代码的改动（M1 第 10 节的取舍不动）。

**移交的落实**（00 号文档"移交的落实"里 P1 的部分）：

| 移交的项 | P1 的落实 |
|---|---|
| [M5-polish](handoffs/M5-polish.md) 第 51 条（`goto` 打断续期） | S1：帮手、110 个文件、门禁、A15、`pageWatch` 写明"被跳回登录页"（3.2–3.6）。"重新看 M1 第 10 节的取舍"不在 M12 做：写进已知限制与 v0.2 的 `M12-release.md`（00 号文档 4.4，收尾时写） |
| 同上第 50 条（`ERR_NETWORK_CHANGED`） | S1：`networkSettled` 触发时一行日志（3.7）；CI 上是否触发，S1 的记录与收尾记录照 CI 的输出写 |
| [M0-P6-image-release](handoffs/M0-P6-image-release.md) 第 4 条（依赖、工具链与基础镜像） | S2：与安全相关的更新、`make vuln`（Go 与 npm）、`make outdated`、`vuln.yml`（3.9–3.13）。Node 26 评估过、不升；功能性的大版本与 Dependabot 的版本更新进 v0.2 的 `M12-release.md`（收尾时写）；Dependabot 的版本更新由负责人定（00 号文档第 11 节"需要负责人确认"第 6 项） |

### 2.1 对 00 号文档的修订

照代码与本文的设计，00 号文档有下面几处要改（接受本文的文档提交里一并改，第 8 节；行号是 `a22adfd2` 的）：

1. **不做"跨文档的后退"**（4.4 `:207`；第 7 节 P1/S1 一行 `:725`；"总体设计的修订"第 12 条 `:947`；决定 311 `:1162`）：14 处后退与前进都在同一个文档里（附录"后退与前进"），没有故事要它；做了没人用，knip 也会报出没人用的导出。以后要跨文档后退的故事，先在帮手里加一个，照 A15 加一格对照（第 9 节）。`:207` 的"帮手另给一种'跨文档的后退'，只在后退真的换文档时用，故事照实选。"改为"14 处都在同一个文档里（P1 设计附录），帮手不另给跨文档的后退；以后要它的故事先在帮手里加，照 A15 加一格对照（P1 设计 2.1 第 1 条）。"；`:725` 的"（`goto`、`reload`、跨文档的后退、显式的逃生口）"改为"（`goto`、`reload`、显式的逃生口）"；`:947` 引号里的整句（"故事与夹具经帮手 `goto` 与 `reload`（跨文档的后退同样）……门禁数逃生口（M12/P1/S1）"）换成第 8 节第 5 条 S1 那一条加的句子，从"M12/P1/S1：e2e 换文档只经"起、到"（M12/P1）。"止，一字不差（2.1 第 3、4 条要改的"帮手先等在途的续期与读答完""门禁数逃生口"一并换掉）；决定 311 末尾加"（P1 设计 2.1 第 1 条：没有故事要跨文档的后退，不做）"。
2. **TR2 不用逃生口**（4.4 `:208`；第 7 节 P1/S1 一行 `:725` 的"TR2 经逃生口"；决定 311 `:1162`）：TR2 扣住导入之后的是 `page.goBack()`，同一个文档里的导航，帮手与门禁都不管它，它照旧。逃生口在 P1 里只有 A15 的两格。`:208` 改为"**逃生口**：帮手带一个显式的选项 `{ inFlight: "leave" }`，不等就离开，只给有意在请求途中离开的故事用；P1 里只有 A15 的两格（P1 设计 3.6）。TR2 扣住导入的 POST 之后的后退（`e2e/stories/transfer/tr2-import.spec.ts:241-259`）在同一个文档里，帮手与门禁都不管，用不到逃生口。"；`:725` 的"TR2 经逃生口"改为"逃生口只有 A15 的两格"；决定 311 的"显式的逃生口给 TR2 与对照测试"改为"显式的逃生口只给有意在请求途中离开的故事（P1 里是 A15）"，并在决定 311 的括注里加"P1 设计 2.1 第 2 条改，原来是'显式的逃生口给 TR2 与对照测试'"，同第 11 节的写法留下原文。
3. **帮手多等两样，各由 A15 的一格定下**（4.4 `:205-206`、"第一次续期" `:210`；第 7 节 P1/S1 一行 `:725` 的"'等第一次续期'由对照测试定"）：帮手除了等在途的请求，还等"这个文档的第一次续期已答"（U1）与"续期的锁空着"，每一样各有一格只在它去掉时失败（3.6 的表，原型在三种浏览器里各跑三轮，`m12/p1-rev1/`）：
   - **第一次续期**由第 4 格定下要：init script 不经锁推迟应用的第一次续期（应用调 `navigator.locks.request` 之前先等），`locks.query()` 里什么也没有、在途的集合是空的，只有"第一次续期"一样拦着帮手。去掉它，帮手立刻换页，Chromium 与 Firefox 落到 `/sign-in`；WebKit 没有这个窗口（第 1 节），旧文档的续期发不出去，那一格以"the old document never sent its refresh"失败。初稿用 init script 持住锁来推迟，那时只看锁的帮手也等，第 4 格分不出两样（P1 设计审查 A-I1，`m12/p1-review-a/session-waits-probe.log` 的窗口 A）。
   - **锁**由第 6 格定下要：第一次续期答完之后，测试在页面里持住 `nwiki.auth.refresh`，帮手要等它放开才换页；去掉锁的检查，新文档在放开之前就请求了（三种浏览器）。锁盖住"续期的答复已到、新的记录还没写下"那一小段（`token-manager.ts:296-306` 在锁里写记录），也盖住别的标签页正在写的续期。

   `:210` 的"P1/S1 的设计考虑帮手再等'这个文档的第一次续期已答'，由实施时的对照测试定要不要。"改为"帮手再等这个文档的第一次续期已答、续期的锁空着。A15 的第 4 格不经锁推迟第一次续期，只看在途请求与锁的帮手在 Chromium 与 Firefox 里落到 `/sign-in`；第 6 格在第一次续期之后持住锁，不看锁的帮手在放开之前就换页。WebKit 在请求下一个文档之后旧文档发不出请求，没有这个窗口（P1 设计 3.2、3.6；P1 设计审查 A-I1、C-I1）。"；`:725` 的"'等第一次续期'由对照测试定"改为"等这个文档的第一次续期与续期的锁"。
4. **门禁的范围、形式与逃生口的表**（第 3 节第 2 条 `:87`；4.4 门禁 `:211`；9.3 `:1052`）：门禁管整个 `e2e/`（除去 `node_modules`、`test-results`、`playwright-report`），不只 `e2e/stories` 与 `e2e/fixtures`：P4/S2 的 `e2e/perf/` 同样换文档。它是 `tools/e2e-navigation/check.mjs`，进 `make lint-web`。逃生口不只"报出个数"：门禁里有一张表，写每个用逃生口的文件、几处、理由，对不上就失败（3.5）。`:87` 的"故事与夹具（`e2e/stories`、`e2e/fixtures`）里不再直接调"改为"`e2e/` 里（故事、夹具与以后的 `e2e/perf/`）不再直接调"，"门禁报出逃生口的个数，每一处的理由写在 S1 的记录里"改为"门禁按它的表核对每个文件的逃生口的个数，理由写在表里"；`:211` 整条改为"- **门禁**：`e2e/` 里（故事、夹具与以后的 `e2e/perf/`）不许直接调 `goto`、`reload`，只许经帮手。帮手所在的那一个模块是唯一直接调用它们的地方，门禁对它整个例外：不报它的直接调用，也不数它里面的逃生口（修订核对第 1 轮 C 的 N8；P1 设计审查 A-M1、C-M2）。逃生口要显式写，门禁按它的表核对每个文件的逃生口的个数，理由写在表里。门禁是 `tools/e2e-navigation/check.mjs`（`make lint-web`），先在样例上证明它看得见每一种违例，带反向对照（13.4 第 1 条）。"；`:1052` 的"逃生口多一处，门禁报出的个数随之变"改为"逃生口多一处、少一处，或出现在表外的文件里，门禁都失败"。
5. **`pageWatch` 的位置**（4.4 `:209`）：页面的监视在 `e2e/fixtures/browser.ts` 的 `watchPage`（`:123-207`），`test.ts` 只把它交给故事。`（\`e2e/fixtures/test.ts\`）` 改为"（`e2e/fixtures/browser.ts` 的 `watchPage`）"；"失败时写明'被跳回登录页'"后加"：它记下每一次落到 `/sign-in` 与之前那次续期是否答 401，失败时页面的状态第一行写出（P1 设计 3.3）"。
6. **对照测试是一个新故事 A15**（第 3 节第 2 条 `:88`；4.4 `:212`；9.3 `:1051`）：`e2e/stories/identity/a15-leave-mid-refresh.spec.ts`，六格，三种浏览器，第 3 格只在 Chromium 与 Firefox（3.6）。照 10.1，故事一个文件，A15 在身份的分组里；它只有页面版本，接口那一面（旧令牌再用就是重复使用）是 A5 (API)（`a5-refresh-reuse.spec.ts:68`）。`:88`、`:1051` 不改：A15 的第 1、2 格就是它们写的"把续期的答复延后，经逃生口落到 `/sign-in`、经帮手不落"。`:212` 末尾加"对照测试是故事 A15（`e2e/stories/identity/`，三种浏览器）：续期的答复在途时、第一次续期还没发出时，经逃生口离开都落到 `/sign-in`（后一格只在 Chromium 与 Firefox：WebKit 在请求下一个文档之后旧文档发不出请求）；经帮手都不落；另几格各核对帮手等第一次续期、续期的锁与在途的读（P1 设计 3.6）。"
7. **S1 改的部件**（第 7 节 P1/S1 一行 `:725` 的"e2e、docs"）：另有 `tools/`（门禁）、`Makefile` 的 `lint-web` 与 `knip.jsonc`。改为"e2e、tools、build（`Makefile`、`knip.jsonc`）、docs"。
8. **P1/S1 的变异**（9.6 `:1082`）：整条改为"- P1/S1：帮手什么也不等（A15 第 2、4、5、6 格抓）；帮手不等这个文档的第一次续期（第 4 格，Chromium 与 Firefox 落到 `/sign-in`、WebKit 报旧文档没发续期）；帮手不看续期的锁（第 6 格）；帮手不等在途的读（第 5 格）；`reload` 不先等（第 2 格）；门禁放过直接的 `goto` 或 `reload`；门禁不看 `e2e/fixtures/`；门禁不核对逃生口的表；门禁把帮手模块里的逃生口文字也算进去（P1 设计 7.2）。"。理由：初稿的"帮手漏等 `refresh`"只去掉在途集合里的续期，是等价的变异：在途的续期若是文档的第一次，"第一次续期"一样等它；若是后来的，它在锁里发、写完记录才放锁（`token-manager.ts:288-310`），锁的检查等它（7.2）。
9. **`govulncheck` 的版本**（4.3 `:184`）：`@<固定版本>` 改为 `@v1.9.0`（写本文时最新，2026-10-11；`Makefile` 的 `GOVULNCHECK_VERSION`，3.12）。
10. **06-environment 的行号不改**（4.3 `:173`；附录 B `:1315`）：00 号文档的行号以它头上写的 `5e8af5b5` 为准（`:12`），那里 `06-environment.md:52` 是对的；改成 `a22adfd2` 上的 `:53` 会让 00 号文档混用两个基点（同一行的 `v0.1-design.md:1178` 照 `5e8af5b5` 也对）。本文引的是 `a22adfd2` 上的 `:53`（第 8 节第 6 条）。初稿要改这两处，照 P1 设计审查 A-N2 撤回；编号留着，免得后面各条的号变。
11. **README 的 Go 版本也进守卫，断言带一句错误信息**（4.3 `:170-171`；"总体设计的修订"第 5 条 `:933`；第 3 节第 1 条 `:83`）：README 写给贡献者的版本（`README.md:16`）同样由短测试核对，同 PostgreSQL 的守卫把 README 也核对进去。构建时的断言写成 `RUN test "$(go env GOVERSION)" = go1.27.4 || { echo …; exit 1; }`，失败时说清是哪个版本（3.9）。`:171` 的 `RUN [ "$(go env GOVERSION)" = go1.27.4 ]` 改为这一行；"一个短测试核对 tag 与这一行的版本都等于两份 `go.mod` 的 `toolchain`"改为"一个短测试核对 tag、这一行与 README 写的版本都等于两份 `go.mod` 的 `toolchain`"；第 5 条的新文字里"（tag 与 server 阶段构建时断言的 `go env GOVERSION`）"之后加"与 README 写给贡献者的版本"。第 3 节第 1 条 `:83` 不改：它写的 tag 与断言仍对，README 一行是守卫多核对的一处，完成的证据照第 8 节。
12. **`make outdated` 也在改了依赖清单的推送上跑，失败不染红**（4.3 `:197`；5.3 的表 `:674` 与"工作流" `:680`）：`workflow_dispatch` 只对已在默认分支上的工作流生效，只手动触发的作业在 S2 的分支上跑不了，合并之前就验证不了它。改为"手动触发与改了依赖清单的推送"两种，作业带 `continue-on-error: true`：它只列出过时的，自己出错也不该让主持者每个 Step 看的 `vuln.yml` 的结论变红（3.12）。`:197` 的"它在 `vuln.yml` 里一个只手动触发的作业上跑"改为"它在 `vuln.yml` 里一个手动触发、改了依赖清单的推送也跑的作业上（`continue-on-error`，它的失败不让运行变红）"；`:674` 整行改为"| `make outdated` | 只读地列出过时的 Go 模块、npm 包、actions 与镜像 | `vuln.yml` 里一个手动触发、改了依赖清单的推送也跑的作业（`continue-on-error`，失败不让运行变红），发布之前手动触发一次；**本机不跑** | P1/S2 |"；`:680` 的"手动触发时另可跑 `make outdated`"改为"手动触发与改了依赖清单的推送时另跑 `make outdated`"。
13. **`vuln.yml` 的 `push` 只在分支上，路径加 `tools/outdated.sh`**（4.3 `:188`）：只写 `paths` 时推 tag 也会触发（GitHub 对 tag 的推送不看 `paths`）；发布的闸门自己跑 `make vuln`，这里不必再跑。`push` 加 `branches: ["**"]`；`paths` 加 `make outdated` 的脚本 `tools/outdated.sh`（3.12）。`:188` 的路径一句末尾加"、`tools/outdated.sh`；只在分支的推送上，tag 不触发"。

### 2.2 需要负责人确认

无。第 11 节的负责人与主持者的决定在本文里都照做；2.1 的十三条（第 10 条撤回，不改）是照代码与审查改说法、位置、范围或做法，第 1、2 条少做了 00 号文档写的一样东西（没有故事要它），都不改那些决定的本意。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

正式编号 337–352，主持者在设计审查之后给的；括号里是临时号，审查记录用它。

- **337（P1-1）帮手是函数，不是 `page` 的方法**：`goto(page, url, options)`、`reload(page, options)`，模块 `e2e/fixtures/navigation.ts`。不改写 `page` 夹具给它换方法：`anotherPage`、`anotherTab` 与故事自己开的标签页也要它，包一层 `Page` 会让 Playwright 的类型与 `page.context()` 的页面对不上。替换是机械的：`X.goto(` → `goto(X, `（3.4）。
- **338（P1-2）在途的请求只算接口的 fetch 与 XHR**：`/api/v0/` 之下、事件流 `/api/v0/events` 除外、资源类型是 `fetch` 或 `xhr`。图片、媒体与附件的内容（阅读视图里的 `<img>`、`<video>`）不算：它们打断不了续期，媒体的请求还可能一直开着。
- **339（P1-3）帮手的三样等待**：在途的接口请求答完；续期的锁空着（`navigator.locks.query()` 里没有 `nwiki.auth.refresh`；没有 Web Locks 时租约 `nwiki.auth.lease` 不在或已过期），有没有会话的记录都看：登录、注册在锁里写第一份记录；这个文档有会话的记录时，它有了给它令牌的答复（文档的 `performance` 里有续期、登录或注册的一条：`/api/v0/auth/refresh`、`/api/v0/auth/login`、`/api/v0/auth/register`）。后两样在页面里读，与 Playwright 何时开始监视无关：L1 的新标签页晚于加载才被监视（`l1-links.spec.ts:183`），照样等得对（3.2）。三样各有 A15 的格只在它去掉时失败：在途的读是第 5 格，第一次续期是第 4 格，锁是第 6、8 格，租约是第 9 格，令牌的答复里注册一支是第 7 格（登录一支是 A3、S4 与第 8 格）（3.6 的表）；只从在途的集合里去掉续期是等价的变异（7.2）。P1/S1 实施改，原来是"这个文档有会话的记录时，它的第一次续期已答（文档的 `performance` 里有 `/api/v0/auth/refresh` 的一条）；续期的锁空着"，先看记录：在这个文档里登录、注册的不发第一次续期，照原来的写法 A3、S4 在登录页登录之后换文档时等满 10 秒失败；登录、注册在锁里写记录，先看记录会在写之前放行，A15 第 8 格（3.15 第 2、9 条）。
- **340（P1-4）帮手等 10 秒，然后失败并写出在等什么**：比续期自己的 8 秒（`token-manager.ts:24`）长。故事的默认时限 30 秒，帮手的错误先于测试的超时出来，写明"传 `{ inFlight: "leave" }` 才能不等"。在页面里读的那一次也受这个时限：Chromium 在下一个文档还没提交时 `page.evaluate` 不返回（第 1 节），所以它与剩下的时间赛跑，没答就算在等"the next document (pending)"（3.2；P1 设计审查 C-N2）。
- **341（P1-5）没被监视的页面，帮手直接失败**：帮手在 `watchPage` 登记的页面上才知道在途的请求；故事自己开的标签页先 `watchPage`（今天的四处都这样，第 1 节）。不在第一次调用时才开始监视：那时已在途的请求看不到。
- **342（P1-6）落到 `/sign-in` 只写进失败时的页面状态，不让通过的测试失败**：很多故事有意走到登录页（退出、A5、停用、改密码），让它们都声明会让 50 多个文件跟着改。失败时的状态第一行写"sent back to sign in: …"，带之前那次续期答没答 401（3.3）；收尾数"被跳回登录页"红过几次，照 CI 注解里这一行数：数这一行带 ` after a refresh answered 401` 的失败，再除去有意造重用的故事（A5）（决定 334）。P1/S1 实施改：这一行只在页面落到过 `/sign-in` 时写，故事自己去登录页的也有，原来写"照这一行数"（3.15 第 10 条）。
- **343（P1-7）门禁是一个读源码的 Node 脚本，不是 oxlint 的规则**：oxlint 1.86 有 `no-restricted-properties`（本机的二进制里有这个规则名），能禁 `.goto`，却数不了逃生口、核不了表。一个脚本做全两件事，照 `tools/md-fixtures/check.mjs` 的先例进 `make lint-web`。脚本先在内置的样例上跑一遍，看不见样例里的违例就失败，所以它不会因为什么也没读到而通过（3.5）。
- **344（P1-8）门禁不看注释与字符串，一律按文字找；帮手的模块整个例外**：`.goto(`、`.reload(`、`inFlight: "leave"` 出现在 `e2e/` 的任何地方都算，注释与字符串也算。今天注释里没有这样写的（第 1 节）；写说明时不写成调用或逃生口的样子（写"the escape""逃生口"）即可。帮手的模块两样都不查：它的报错信息要写出逃生口怎样写，oxfmt 把那个字符串改成单引号之后，源码里就有 `inFlight: "leave"` 这段文字（P1 设计审查 A-M1、C-M2 实测）；内置样例 7 核对这一条（3.5）。不引入 TypeScript 的解析器（e2e 用的 TypeScript 7 没有稳定的 JS 接口）。
- **345（P1-9）A15 放在身份的分组，三种浏览器**：它是一个用户看得到的行为（续期途中离开会被登出，发布说明的已知限制），又是帮手的对照。探针显示三种引擎的窗口不同（Firefox 0/15），所以三种都跑；第 3 格例外，见决定 351。不放进 A5：A5 的第二格的控制台期望是 Chromium 的原文（`a5-refresh-reuse.spec.ts:56`），整个文件进 `threeBrowsers` 会红。
- **346（P1-10）`make vuln`、`make outdated` 在 runner 之外拒绝运行**：`GITHUB_ACTIONS` 不是 `true` 时打一句说明、退出码 1。规矩是本机不往外发依赖清单（决定 308）；目标存在就有人顺手在本机跑，拒绝比写在注释里可靠。它自己的反向对照就是在本机跑一次（3.12）。P1/S2 实施：判断与干活写在同一个 shell 行，`make -i` 绕不过；`tools/outdated.sh` 自己也判，直接执行也拒绝；"退出码 1"是配方的，`make` 自己以 2 退出（3.16 第 1、12 条）。
- **347（P1-11）Go 版本的守卫放在 `platform/buildinfo`**：它管"镜像用哪个 Go 构建"，与版本号同属"构建出的是什么"。00 号文档第 3 节第 8 条只说版本号的默认值"由短测试守住"（`:120`），没定位置；本文建议 P5/S1 把那条守卫也放在这个包，与这条相邻，由 P5/S1 的设计定（第 9 节；P1 设计审查 A-M6）。不放在 `pgtest`：那是测试数据库的包。
- **348（P1-12）断言放在 server 阶段 `FROM` 之后的第一行**：在 `go mod download` 之前，摘要不对时最早失败。
- **349（P1-13）第 50 条只在等待真的发生时打日志**：没有 tentative 的地址时什么也不打（今天本机每轮约 0.2 秒、多数是这种）。"没看到这一行"要能当作"没触发过"，S1 先做一次正向对照：临时让等待真的发生一次，看到这一行出现在运行的输出里，再复原（3.7；P1 设计审查 A-M5）。有了它，CI 上一次也没看到这一行，才算没触发过。
- **350（P1-14）不改 `search_guard_test.go:13` 的 "Go 1.27.1"**：它记的是 M10 校准那一刻的环境，不是要求。
- **351（P1-15）A15 的第 3 格只在 Chromium 与 Firefox 跑，WebKit 跳过并写明理由**：它要造的是"下一个文档已请求、旧文档恰在这时发出第一次续期"。WebKit 在请求下一个文档之后，旧文档的请求一个也发不出去（P1 设计审查 C 的探针 B、D；第一次修订的原型：旧文档等来"放"之后，续期仍没有发出），没有这个窗口，这一格在 WebKit 里造不出、也不必防。`test.skip(browserName === "webkit", …)` 的理由写这一句。第 4 格三种都跑，WebKit 里它只证明"帮手等第一次续期"（去掉那一样时以"the old document never sent its refresh"失败），不证明 U1（3.6；主持者照 P1 设计审查 C-I1 定）。
- **352（P1-16）S2 之后第五份在 EPYC 7763 上的余量照数据看，不拖住合并**：第 8 节第 1 条照 06-environment 第 4 节判两次 `main` 的平均；第五份的风险在 EPYC 7763 上（收尾之后两次平均 425 秒，余量 25 秒）。S2 合并之后，`main` 的第五份第一次落在 7763 上时，记下它的 Test 一步，与 425 秒比；两次落在 7763 上的平均过 450 秒时，照 06-environment 第 4 节挪份。不为等 7763 拖住 S2 的合并，也不硬比不同 CPU 的数（7.5；主持者对 P1 设计审查 A-Q1 的决定）。

## 3. 实现

P1 没有服务端的产品代码。S1 改 e2e 的夹具、故事与一个门禁；S2 改工具链的各处、一个守卫测试、Makefile 与一个新工作流。

### 3.1 S1 的文件

| 文件 | 改动 |
|---|---|
| `e2e/fixtures/navigation.ts` | 新：导出 `followRequests`、`goto`、`reload`；不导出的 `Leaving` 与 `settle`（3.2） |
| `e2e/fixtures/browser.ts` | `watchPage` 调 `followRequests`；`PageWatch.signInLandings`；`pageState` 的第一行（3.3） |
| `e2e/stories/**`（109 个文件）、`e2e/fixtures/wiki-health.ts` | `X.goto(` → `goto(X, `，`X.reload()` → `reload(X)`，加导入（3.4） |
| `tools/e2e-navigation/check.mjs` | 新：门禁（3.5） |
| `Makefile` | `lint-web`（`:159-165`）加一行 `node tools/e2e-navigation/check.mjs` |
| `knip.jsonc` | 根的 `entry`（`:7-15`）加 `tools/e2e-navigation/check.mjs` |
| `e2e/stories/identity/a15-leave-mid-refresh.spec.ts` | 新：A15（3.6） |
| `e2e/playwright.config.ts` | `threeBrowsers`（`:3-4`）加一支 `identity\/a15-` |
| `e2e/global-setup.ts` | `networkSettled` 触发时打一行（3.7） |

### 3.2 S1：帮手 `e2e/fixtures/navigation.ts`

文件头的说明写成因与规则：一个文档在续期的答复回来之前走了，新的令牌就丢了；下一个文档拿旧的续期，服务端认作重复使用，会话结束、页面落到 `/sign-in`（M1 第 10 节接受，A15 演示）。所以 `goto` 与 `reload` 先等，有意在请求途中离开的写 `{ inFlight: "leave" }`；门禁不许别的模块直接调 `page.goto`、`page.reload`，并核对逃生口。关标签页（`page.close()`）同样离开文档、会丢在途的续期，帮手不管它：今天关应用的五处都在要令牌的内容出现之后才关，AS3 关的是 PDF 的弹窗（第 1 节），以后的故事照样先等一个这样的内容。

```ts
import type { Page, Request, Response } from "@playwright/test";

/** A story's choice besides Playwright's options: "leave" goes at once, whatever is out. The gate counts the calls that pass it. */
interface Leaving {
  inFlight?: "leave";
}

/** How long settle waits before it fails, naming what it waits for: longer than a refresh's own 8 s. */
const settleMs = 10_000;
const pollMs = 25;
/** How long one read in the page may take at least, even past settleMs: what it reads names the wait. */
const readMs = 1_000;
// The session's record, its refresh lock, the lease without Web Locks (W/session/token-manager.ts:16,
// W/session/refresh-lock.ts:14, :25), and the answers that give a document its tokens: the first refresh of one
// that started with the record (token-manager.ts:221-229), or the sign-in or sign-up that wrote it there
// (token-manager.ts:164-172, W/services/auth.service.ts:17, :26), after which no first refresh comes.
const authKey = "nwiki.auth";
const lockName = "nwiki.auth.refresh";
const leaseKey = "nwiki.auth.lease";
const tokenPaths = ["/api/v0/auth/refresh", "/api/v0/auth/login", "/api/v0/auth/register"];

/** Each watched page's API requests not answered yet. */
const unanswered = new WeakMap<Page, Set<Request>>();

/**
 * Follows page's API requests from now on: fetch and XHR under /api/v0/, but the event stream, which stays
 * open. watchPage calls it before the page's first navigation. A request that a route holds and never
 * answers stays here once its document is gone where the browser reports no failure for it (Chromium): the
 * page's next goto or reload waits for it, and fails.
 */
export function followRequests(page: Page): void {
  const open = new Set<Request>();
  unanswered.set(page, open);
  page.on("request", (request) => {
    const { pathname } = new URL(request.url());
    const type = request.resourceType();
    if ((type === "fetch" || type === "xhr") && pathname.startsWith("/api/v0/") && pathname !== "/api/v0/events") {
      open.add(request);
    }
  });
  page.on("requestfinished", (request) => open.delete(request));
  page.on("requestfailed", (request) => open.delete(request));
}

/** page.goto, once the page's document has nothing out that leaving would lose. */
export async function goto(
  page: Page,
  url: string,
  options: NonNullable<Parameters<Page["goto"]>[1]> & Leaving = {}
): Promise<Response | null> {
  const { inFlight, ...rest } = options;
  if (inFlight !== "leave") await settle(page);
  return page.goto(url, rest);
}

/** page.reload, the same way. */
export async function reload(
  page: Page,
  options: NonNullable<Parameters<Page["reload"]>[0]> & Leaving = {}
): Promise<Response | null> {
  const { inFlight, ...rest } = options;
  if (inFlight !== "leave") await settle(page);
  return page.reload(rest);
}
```

`settle(page)`（不导出）：

1. 取 `unanswered.get(page)`；没有就抛错："goto, reload: the page is not watched; call watchPage before its first navigation"（决定 341）。
2. 循环，每 25 ms 看一次：
   - 在途的集合不空：等的是它们（`"<方法> <路径>"`）。
   - 空了：在页面里看会话（下面的 `sessionWaits`），与剩下的时间赛跑，至少 1 秒（`readMs`：最后一圈剩下的只有几毫秒时，读总输给计时，报错就写不出真在等的一样，3.15 第 3 条）：到时还没答，就算在等 `"the next document (pending)"`（Chromium 在下一个文档还没提交时 `page.evaluate` 不返回，第 1 节；P1 设计审查 C-N2）。
   - 两样都没有要等的：返回。
   - 过了 10 秒：抛错，写出在等的每一样，与"a story that leaves it on purpose passes { inFlight: \"leave\" }"。
   - 循环里的 `await` 照仓库的写法带 `// oxlint-disable-next-line no-await-in-loop -- …` 的理由（`e2e/global-setup.ts:50-53` 的先例）。

`sessionWaits(page)` 以一次 `page.evaluate` 在页面里读，交回在等的几样（空就是没有）：

```ts
async ({ authKey, lockName, leaseKey, tokenPaths }) => {
  try {
    localStorage.getItem(authKey);
  } catch {
    return []; // an opaque origin (an attachment shown sandboxed at its address, about:blank): no session here
  }
  // Not the app (W/main.tsx:17 mounts on #root): no session to lose.
  if (document.getElementById("root") === null) return [];
  // A refresh, a sign-in or a sign-up writes the record under the lock (W/session/token-manager.ts:164-172,
  // :288-310): a document whose sign-in was answered has no record until it holds it.
  if ("locks" in navigator) {
    const { held = [], pending = [] } = await navigator.locks.query();
    if ([...held, ...pending].some((lock) => lock.name === lockName)) return ["the refresh lock"];
  } else {
    const lease = JSON.parse(localStorage.getItem(leaseKey) ?? "null") as { expires?: number } | null;
    if (lease !== null && (lease.expires ?? 0) > Date.now()) return ["the refresh lease"];
  }
  // Read after the lock: no session's record by then, no refresh to lose.
  if (localStorage.getItem(authKey) === null) return [];
  // A document's resource timings are its own requests, each added once its answer was read.
  const tokens = performance
    .getEntriesByType("resource")
    .some((entry) => tokenPaths.includes(new URL(entry.name).pathname));
  return tokens ? [] : ["the document's first refresh"];
}
```

- **给文档令牌的答复**：开始时就有记录的文档（`token-manager.ts:221-229`）先续期，令牌来自第一次续期的答复；在这个文档里登录或注册的，令牌来自那次答复（`token-manager.ts:164-172` 的 `signIn` 在锁里写记录），不发第一次续期（3.15 第 2 条）。给文档令牌的只有这三种答复（S1 的审查 A 逐个数过入口）。文档的 `PerformanceResourceTiming` 只记这个文档自己发的请求、在答复读完时加，所以"有一条续期、登录或注册"就是"这个文档的令牌到了"，单页应用里换路由也还在；答复的码不看（被拒的登录也留一条，今天没有在那之后换文档的流程，S1 的记录"接受与推后的"）。开始时没有记录、之后跟着别的标签页登录的文档，在它下一次读（`me`）时于锁里续期，帮手等的是那次续期（S1 的审查 A 的探针）。缓冲默认 250 条，第一次续期是文档最早的接口请求之一（别的读都等访问令牌），落在缓冲里。续期在网络一层失败的（没有答复）不留这一条，帮手会等满 10 秒后失败：今天没有故事在这种文档上再换文档（第 1 节；A5 的第二格在那之后不再导航）；有了就用逃生口。
- **锁**：`navigator.locks.query()` 列出这个浏览器上下文里所有标签页持有与等着的锁。续期在锁里读记录、发请求、写回（`token-manager.ts:288-310`），所以锁空着就没有写了一半的续期；别的标签页正在续期时也等，它很短，等了无害。没有 Web Locks 时（A4 的一种，`a4-refresh.spec.ts:105-109` 删掉了 `Navigator.prototype.locks`）看租约：用完删掉（`refresh-lock.ts:99-105`），留着的过期就不算。锁先于记录看，有没有记录都看：登录、注册的答复读完之后，`signIn` 在锁里写第一份记录（`token-manager.ts:164-172`），锁被别的标签页持着的那一段文档还没有记录，先看记录就会放行，新文档丢了这次登录（3.15 第 9 条，A15 第 8 格）；记录在锁之后才读。没有记录的应用文档在别的标签页持锁时也等：持锁的只有续期、登录、退出，至多 8 秒。
- **读不了**：`page.evaluate` 抛出"Execution context was destroyed"（页面正被别的导航换掉，例如故事刚点了一个跨文档的链接）时，交回 `["the next document"]`，下一圈看新的文档；到时限还没答时交回 `["the next document (pending)"]`，帮手照时限失败、写出它（上面第 2 步）。这一条没有专门的测试：要造它，得让故事扣住下一个文档再调帮手、等满 10 秒；它只决定帮手失败时写出什么，不决定等得对不对；别的错误交回空：读不了的文档跑不了应用，也就没有会话可丢。这一条若掩住了帮手自己的错，A15 的帮手几格会失败（7.2）。
- **沙箱里的附件**：AS5 打开一个 SVG 附件的地址（`as5-content-safety.spec.ts:176`），它的文档是不透明的源，`localStorage` 抛错，交回空；它的下一个 `goto`（`:183`，一次下载）照常。`expectQuietPage` 在那个文档上 `page.evaluate` 今天就通（AS5 为绿），所以读得进去。
- **`Leaving` 不导出**：故事不写它的名字，只写 `{ inFlight: "leave" }`。导出时只要它用在有人导入的函数签名里，knip 也不报（A 的探针；C-M1 的前提不成立，P1 修订核对第 1 轮 C 的 M5），不导出更简单；不导出时 `tsc`、knip、oxlint 都干净（第一次修订在临时 worktree 里核对：`navigation.ts` 的 `goto`、`reload` 签名用不导出的 `Leaving`，`watchPage` 调 `followRequests`，一个故事换上帮手；`m12/p1-rev1/leaving-static-checks.txt`）。

### 3.3 S1：`watchPage` 的在途记录与登录页的落点

`e2e/fixtures/browser.ts`：

- `watchPage`（`:123`）开头调 `followRequests(page)`（从 `./navigation` 导入）。
- `PageWatch`（`:25-55`）加：

  ```ts
  /**
   * Each time the page's top document went to /sign-in, as "<path and query>", followed by " after a refresh
   * answered 401" when the last refresh before it was refused: a session that ended under the story.
   */
  readonly signInLandings: string[];
  ```

- 答复的处理（`:163-169`）里，遇到 `POST /api/v0/auth/refresh` 就记下它是不是 401（一个变量 `refusedRefresh`，每次续期的答复改写它）。
- 加 `page.on("framenavigated", …)`：主框架的地址的路径是 `/sign-in` 时，记一条 `${pathname}${search}`，`refusedRefresh` 时后面加 `" after a refresh answered 401"`；与上一次主框架的地址相同就不记（`replaceState` 会再报一次同一地址）；`/sign-in` → 别处 → 同一个 `/sign-in` 照记两次（3.15 第 5 条）。单页应用的路由改地址也会报 `framenavigated`，所以会话结束后应用自己跳到 `/sign-in` 也记得到。
- `pageState`（`:251-280`）在页面落到过 `/sign-in` 时，第一行加 `sent back to sign in: ${watch.signInLandings.join(", ")}`，在 `url:` 之前；没有落点时没有这一行（3.15 第 10 条）。故事自己去登录页的也有这一行：数被跳回的失败，数带 ` after a refresh answered 401` 的，再除去有意造重用的故事（A5）（决定 342）。失败的测试的错误与附件都带它（`pageStatesOf` `:296-314`），CI 的注解里读得到。

### 3.4 S1：换 110 个文件

一个一次性的 Node 脚本（不提交；脚本与它的输出贴进 S1 的审查记录），对 `e2e/stories/**/*.ts` 与 `e2e/fixtures/wiki-health.ts`：

1. `/\b([A-Za-z_$][\w$]*)\.goto\(\s*/g` → `goto($1, `（跨行的 `page.goto(\n  …` 一并）。
2. `/\b([A-Za-z_$][\w$]*)\.reload\(\)/g` → `reload($1)`（10 处都没有参数）。
3. 改过的文件加一行导入：故事是 `import { goto, reload } from "../../fixtures/navigation";`（只导入用到的那个；故事都在 `stories/<分组>/` 下一层，第 1 节），夹具是 `from "./navigation"`；插在已有的 `../../fixtures/…` 导入之中、照路径的字母序（`…/fixtures/links` 与 `…/fixtures/notebook-members` 之间是 `navigation`），同各文件今天的次序。
4. `pnpm run fix:format`（oxfmt 重排换了的行），`make lint-web`。

**核对一个不漏**：

- 改之前：`grep -rcE '\.goto\('` 合计 259（故事）加 1（夹具），`\.reload\(` 合计 10；文件 110 个。改之后：门禁报出 0 处直接调用（3.5），这是"一处不漏"的证明；新写法的个数用不按行的脚本数，允许跨行的空白：oxfmt 会把长的调用拆成几行（例如 `lw6-source-history.spec.ts:356` 拆成 `await goto(\n  page,\n  …)`），按行的 `grep` 改之后只数得到 259（P1 修订核对第 1 轮 C 的 M4）。正则是 `(?<![.\w$])goto\(\s*[A-Za-z_$][\w$]*\s*[,)]`，对整个文件的内容匹配（例如 Python 的 `re.findall` 读整个文件），`reload` 同样；改之前是 0 与 0，改之后要是 260 与 10。`\bgoto\(` 不能用：`.` 与 `g` 之间也是词的边界，改之前就是 260；不带前面的排除，`\b(goto|reload)\([A-Za-z_$]…` 改之前也有 28 处（`page.goto(变量)`）（P1 设计审查 A-N9、C-N1；第一次修订在 `a22adfd2` 上数过）。第 1 个提交的 `git diff --stat` 是 112 个文件：这 110 个，加 `navigation.ts`、`browser.ts`。
- **只做了这一种改动**：另一个一次性的脚本把每个改过的文件做反向的变换（`goto(X, ` → `X.goto(`、`reload(X)` → `X.reload()`、去掉那一行导入），与改之前的文件比较，两边都去掉全部空白之后逐字相同；反向的变换同样不按行：`goto(` 之后与接收者后面的逗号之后都可能换行（`lw6-source-history.spec.ts:356`），正则写成 `goto\(\s*X\s*,\s*`，110 个文件才都相同（P1 修订核对第 1 轮 C 的 M4，第 2 轮 C 的 N6）。110 个文件都相同，才算替换是机械的。
- `pnpm -r run check:types` 为绿（漏了导入、接收者不是 `Page` 都会报）；`make knip` 为绿。
- 行为由全量 `make e2e` 证明：故事的个数不变（加 A15 之前与之后各记一次；A15 是九格乘三种浏览器共 27 个，其中 WebKit 的第 3 格跳过（3.15 第 6 条）），全绿。

**会碰到的**（设计时看过，实施时以全量运行为准）：

- **扣着请求不放再换文档的**：设计时没看到（第 1 节"路由"）。有的话帮手 10 秒后失败、写出是哪个请求；那个故事的写法若是有意的，改用逃生口并进门禁的表，写进 S1 的偏离。
- **TR2**：后退之后点"Stop and leave"停下上传（`tr2-import.spec.ts:255`），`unrouteAll` 用 `ignoreErrors`（`:259`）。上传的请求由应用取消，报 `requestfailed`，离开在途的集合；下一个 `goto`（`:266`）照常。若某种浏览器不报 `requestfailed`，帮手会在那里等满 10 秒：S1 的全量运行会看到，处置同上。
- **CI 的用时**：多数 `goto` 之前没有在途的读；有任务时设置页每秒轮询一次（审查 C 的 U3），帮手可能多等一两次各几十毫秒。

### 3.5 S1：门禁 `tools/e2e-navigation/check.mjs`

照 `tools/md-fixtures/check.mjs` 的写法（文件头写它管什么与怎样跑，`problems` 收集，最后统一报）。

```js
// The e2e's gate on leaving a document (00-M12 4.4, M12/P1/S1). A document left while its refresh is out
// loses the refreshed token, and the next document ends the session as reuse. Only e2e/fixtures/navigation.ts
// calls page.goto and page.reload; everything else under e2e/ goes through its goto and reload, which wait
// first. A story that leaves mid-request on purpose passes the escape: LEAVES lists each file that does, how
// many times, and why. The escape counts written with any quote, a backtick too. The helper's module is read
// but not checked: its calls are the ones allowed, and its message names the escape; the exception is its whole
// path, not every module of that name. Elsewhere comments and strings count too, so they name it in words, not
// in its form. It reads every file with an extension Playwright runs by default (.ts, .tsx, .mts, .cts and the
// .js ones). Closing a tab leaves a document as well; the helper does not cover it (the stories close tabs
// once a page showed what only a signed-in session reads). The check first runs on samples of each breach and
// on the extensions, and fails unless it sees them, and fails unless the tree it read holds the helper's
// module, so it cannot pass by reading nothing, or by missing e2e/fixtures/ or the stories the table names.
// Usage: node tools/e2e-navigation/check.mjs
import { readFileSync, readdirSync } from "node:fs";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "../..");
const HELPER = "e2e/fixtures/navigation.ts";
const SKIP = new Set(["node_modules", "test-results", "playwright-report"]);
// Playwright's default testMatch is **/*.@(spec|test).?(c|m)[jt]s?(x); e2e/playwright.config.ts narrows it
// only for Firefox and WebKit.
const SOURCE = /\.[cm]?[jt]sx?$/;
const DIRECT = /\.(goto|reload)\s*\(/g;
const LEAVE = /inFlight:\s*["'`]leave["'`]/g;

/** Each file under e2e/ that leaves a document mid-request on purpose, and how many times. */
const LEAVES = {
  // A15's two tests of what the helper prevents: one leaves while the refresh's answer is out, one before
  // the document's first refresh has gone out (Chromium and Firefox only); both land on /sign-in (M12/P1/S1).
  "e2e/stories/identity/a15-leave-mid-refresh.spec.ts": 2,
};

/** The breaches in files ({ path, text }), against leaves: a list of "<path>:<line>: <why>". */
function check(files, leaves) {
  const problems = [];
  const counted = {};
  for (const { path, text } of files) {
    if (path === HELPER) continue;
    for (const m of text.matchAll(DIRECT)) {
      problems.push(`${path}:${lineOf(text, m.index)}: calls ${m[1]} directly; use goto or reload from ${HELPER}`);
    }
    const leaving = [...text.matchAll(LEAVE)].length;
    if (leaving > 0) counted[path] = leaving;
  }
  for (const path of new Set([...Object.keys(counted), ...Object.keys(leaves)])) {
    if ((counted[path] ?? 0) !== (leaves[path] ?? 0)) {
      problems.push(`${path}: leaves mid-request ${counted[path] ?? 0} times, LEAVES says ${leaves[path] ?? 0}`);
    }
  }
  return { problems, counted };
}
```

- **样例**（每次运行先跑，任一条报出的问题与期望的整张表不同就失败，写"the gate does not see …"；只比个数时，报错了地方或种类的变异也会过，3.15 第 7 条）：
  1. 故事里 `await page.goto("/");` → 一处；
  2. 帮手之外的夹具里跨行的 `await tab\n  .reload();` → 一处；
  3. 帮手的模块里 `return page.goto(url, rest);` → 没有；
  4. 故事里 `goto(page, "/", { inFlight: "leave" })`、表是空的 → 一处（表外的逃生口）；
  5. 同一个文件、表里写 2 → 一处（个数不对）；
  6. 表里写着、文件里没有 → 一处（表里的过时）；
  7. 帮手的模块里一句报错信息 `throw new Error('… passes { inFlight: "leave" }');`（oxfmt 之后的单引号写法）、表是空的 → 没有（帮手整个例外）；
  8. 故事里反引号写的逃生口 `` goto(page, `/`, { inFlight: `leave` }) ``、表是空的 → 一处（逃生口认三种引号；oxfmt 不把反引号改成双引号）；
  9. 另一个叫 `navigation.ts` 的模块（`e2e/perf/navigation.ts`）里的直接 `goto` → 一处（帮手按整个路径例外，不按名字）。

  第 8、9 条由 S1 的修复一轮加（3.15 第 7 条）。

  第一次修订把这个 `check()` 与七个样例原样跑过，七个都对；在只换了一个故事、帮手带着那句信息的树上，帮手报 0 处（`m12/p1-rev1/gate-check.mjs`、`gate-check.log`）。初稿只让直接调用跳过帮手，逃生口照数，第 2 个提交就红（P1 设计审查 A-M1、C-M2）。
- **真的运行**：从 `e2e/` 递归读 Playwright 默认 `testMatch` 的八种结尾（`.ts`、`.tsx`、`.mts`、`.cts` 与 `.js`、`.jsx`、`.mjs`、`.cjs`，跳过 `SKIP`），每次运行先核对 `x1-sample.spec.<结尾>` 八个名字都被 `SOURCE` 取到（3.15 第 7 条），路径写成仓库根起的。读到的文件里没有帮手的模块就失败，写"the gate did not read e2e/fixtures/navigation.ts: it reads every source file under e2e/"：内置样例直接交给 `check()`，证明不了遍历读到了 `e2e/`（3.15 第 7 条）。有问题就逐条打出、退出码 1；没有就打一行：`e2e navigation: <n> files, no direct goto or reload; leaving mid-request: <file> ×<k>, … (<total>)`。
- **不导出**：入口文件，knip 只要它在 `entry` 里（3.1）。
- **反向对照**：除了内置的样例，7.2 的变异 4–8、19、x6、x7、g1、g1b、g2、c3 各在真的树上改一处、看它失败。

### 3.6 S1：A15（对照测试）

`e2e/stories/identity/a15-leave-mid-refresh.spec.ts`，页面版本，九格。文件头写：A15，离开续期在途的页面（M1 第 10 节：会话结束，用户重新登录；发布说明的已知限制）；故事经 `fixtures/navigation.ts` 换文档，它先等；这里把答复、文档与第一次续期扣住，让次序确定地发生，不靠重复跑（05-lessons 第 6 节）。接口那一面是 A5 (API)。

每格新注册一个账户（`registerOnboarded`），`signedInPage` 给出页面。文件里两个路由的帮手，都在第一次导航之前装好（免得碰上第 1 节"路由"的第一条）。

`routeTheRefreshAgainstTheNextDocument(page)`（第 1–4 格）：

- 续期的路由（`**/api/v0/auth/refresh`）：
  - 第一次：`route.fetch()`（服务端换了令牌），之后 `fetched` 兑现；接着等"下一个文档被请求"或 2 秒，先到的算。下一个文档先到：这个答复不再交回（旧文档已走，它收不到）；2 秒先到：`route.fulfill({ response })`。
  - 之后的，或已记下问题之后的：照常 `fetch` 再 `fulfill`。
- 文档的路由（`**/settings/**`，只管 `resourceType() === "document"`，别的 `fallback`）：
  - 第一个文档放过；
  - 之后的一个：`nextDocument` 兑现；扣到第一次续期 `fetched`，最多 5 秒。5 秒还没来，记下问题 `"the old document never sent its refresh"` 再放过，不卡到测试超时（P1 设计审查 C-I1 的建议 3）。
- 交回 `{ fetched, nextDocument, problems }`。用它的格最后断言 `problems` 为空。

`holdTheFirstRefresh(page)`（第 3、4 格）：不经锁推迟第一次续期，由旧文档自己放开。

- `page.addInitScript`，只在 `/settings/profile` 上：包一层 `LockManager.prototype.request`，应用要锁时先等一个闸门，开了再调原来的。先例是 `a4-refresh.spec.ts:105-109` 在 init script 里删 `Navigator.prototype.locks`。
  - 这时 `navigator.locks.query()` 里什么也没有，在途的集合也是空的（读都等访问令牌）。只有"第一次续期"一样拦着帮手（P1 设计审查 A 的窗口 B）。
  - 初稿用 init script 持住锁来推迟，那时只看锁的帮手也等，分不出两样（A-I1，窗口 A）。
- 闸门由旧文档自己开：init script 每 25 ms `fetch("/__a15/release", { cache: "no-store" })` 问一次，路由答 `wait` 或 `go`，答 `go` 就开闸。
  - 这个地址不在 `/api/v0/` 之下，帮手的在途集合与 `pageWatch` 的接口记录都不看它。
- 交回 `release()`：从此路由答 `go`。
- 不用 `page.evaluate` 放开：Chromium 在下一个文档还没提交时，对旧文档的 `evaluate` 不返回（第 1 节）。初稿那样写，第 3 格在 Chromium 里会卡到超时（C-I1）。

| # | 格 | 浏览器 | 做法 | 断言 |
|---|---|---|---|---|
| 1 | 续期的答复在途时经逃生口重新加载：登出 | 三种 | `routeTheRefreshAgainstTheNextDocument`；`goto(page, "/settings/profile")`；等 `fetched`；`reload(page, { inFlight: "leave" })` | h1 是"Sign in"，地址 `/sign-in?next=%2Fsettings%2Fprofile`；记录为空（`recordOf`）；`expectRevoked(db, tokens.refresh_token, "reuse_detected")`；`pageWatch.signInLandings` 是 `["/sign-in?next=%2Fsettings%2Fprofile after a refresh answered 401"]`；`apiFailures` 含 `401 POST /api/v0/auth/refresh`；控制台以 `expectConsole({ errors: [failedToLoad(401)] })` 声明；`problems` 为空；之后在登录页上以它自己的地址 `replaceState`，`signInLandings` 仍是那一条；再 `pushState` 到 `/`、回到同一个登录页地址（各等一次 `framenavigated`），是两条相同的落点（3.15 第 5 条） |
| 2 | 同样的时刻经帮手重新加载：不登出 | 三种 | 同上，但 `reload(page)`：帮手等那个答复（2 秒后交回）、等锁空了、等随后的读，再重新加载 | 地址 `/settings/profile`，h1 是"Settings"、h2 有"Profile"；`generationOf((await recordOf(page)).refresh_token)` 是 2；`sessionOf(db, tokens.refresh_token).revoked_at` 为空；`signInLandings` 为空 |
| 3 | 第一次续期还没发出时经逃生口换页：登出（U1） | Chromium、Firefox | 两个路由的帮手都装；`goto(page, "/settings/profile")`；不 `await` 地 `goto(page, "/settings/security", { inFlight: "leave" })`；等 `nextDocument`（新文档已请求、被扣着）；`release()`。旧文档开闸、发出第一次续期，服务端换了令牌，`fetched`；新文档放过；旧文档走了，答复交不回 | 同第 1 格，地址 `/sign-in?next=%2Fsettings%2Fsecurity` |
| 4 | 同样的时刻经帮手换页：不登出（U1） | 三种 | 同第 3 格，但 `goto(page, "/settings/security")`（帮手），不 `await`；等"`nextDocument` 或 1 秒"先到的一个；`release()`；等帮手的 `goto` | `problems` 为空；地址 `/settings/security`，h1 是"Settings"、h2 有"Password"；代数 2；会话没有撤销；`signInLandings` 为空 |
| 5 | 读在途时经帮手换页：等读答完 | 三种 | 不装上面两个（续期照常）；只装 `**/api/v0/me` 的路由与文档请求的信号（`page.on("request")` 里主框架的文档请求）。读的路由第一次 `fetch`，扣着答复："下一个文档被请求"先到就不交回，2 秒先到就交回。`goto(page, "/settings/profile")`；等那次读到达路由；`goto(page, "/settings/security")` | 路由记下的结局是"answered"（不是"dropped"：被丢掉的读在 Chromium 里既不答也不报失败，3.15 第 4 条），再断言那次读的 `response()` 是 200；`pageWatch.apiFailures` 为空（读没有被换文档打断）；地址 `/settings/security`，h2 有"Password" |
| 6 | 续期的锁被持住时经帮手换页：等锁放开 | 三种 | 不装路由，续期照常；记下主框架请求 `/settings/security` 那个文档的时刻。`goto(page, "/settings/profile")`；等文档的 `performance` 里有 `/api/v0/auth/refresh` 一条（第一次续期已答）；`page.evaluate` 在页面里 `navigator.locks.request("nwiki.auth.refresh", …)` 持住锁，放开的函数挂在 `window` 上；不 `await` 地 `goto(page, "/settings/security")`；等"下一个文档被请求或 1 秒"先到的一个；记下时刻，`page.evaluate` 放开（写成 `?.()`，出错不管：错的帮手已经换了文档，这里没有扣着的导航，`evaluate` 不会卡住）；等帮手的 `goto` | 文档请求的时刻晚于放开的时刻；地址 `/settings/security`，h2 有"Password"；会话没有撤销 |
| 7 | 在这个文档里注册之后经帮手重新加载：不等，不登出 | 三种 | 不装路由；`goto(page, "/sign-up")`；`signUpWith` 注册（201），应用到 `/onboarding`、h1 是"Your name"；记录是注册的代数 0，`pageWatch.apiRequests` 里没有 `POST /api/v0/auth/refresh`（令牌来自注册的答复，这个文档不发第一次续期）；`reload(page)` | 地址 `/onboarding`，h1 是"Your name"；新文档续期一次，代数是 1（`expect.poll`）；会话没有撤销 |
| 8 | 登录答 200 之后、写记录还在等锁时经帮手换页：等锁放开 | 三种 | 不装路由；没登录的页面 `goto(page, "/settings/profile")`，守卫送到登录页（h1 是"Sign in"）；照第 6 格持住锁（`holdTheRefreshLock`）；在登录页登录（`signInWith`，200），记录还是空的（应用在锁里写它）；不 `await` 地 `goto(page, "/settings/security")`；等"下一个文档被请求或 1 秒"先到的一个；记下时刻、放开；等帮手的 `goto` | 文档请求的时刻晚于放开的时刻（"the next document is asked for once the sign-in wrote its record"）；地址 `/settings/security`，h2 有"Password"；代数 1；会话没有撤销；`signInLandings` 只有起初那一条 `/sign-in?next=%2Fsettings%2Fprofile` |
| 9 | 没有 Web Locks 时，别的标签页的租约有效时经帮手换页：等租约放开 | 三种 | 上下文的 init script 删掉 `Navigator.prototype.locks`（同 A4），先断言页面里 `"locks" in navigator` 为假；`goto(page, "/settings/profile")`；等第一次续期已答；写下别的标签页的租约（`owner: "another-tab"`，5 秒）；不 `await` 地 `goto(page, "/settings/security")`；等"下一个文档被请求或 1 秒"先到的一个；记下时刻、删掉租约；等帮手的 `goto` | 文档请求的时刻晚于删掉租约的时刻（"the next document is asked for once the lease is let go"）；地址 `/settings/security`，h2 有"Password"；会话没有撤销 |

**每一格在哪里跑、去掉帮手的哪一样它失败**（"原型"是第一次修订在三种浏览器里各跑三轮的，`m12/p1-rev1/a15-proto.log`；"C 的原型"是 P1 设计审查 C 各跑两轮的，`m12/p1-review-c/proto25.log`）：

| 格 | 浏览器 | 帮手完整时 | 去掉哪一样时失败（7.2 的变异） | 去掉哪一样照过 | 依据 |
|---|---|---|---|---|---|
| 1 | 三种 | 不经帮手（逃生口），落到 `/sign-in` 是它的期望 | —（对照；帮手怎样改它都不变） | — | C 的原型 proto 1 |
| 2 | 三种 | 绿：帮手等了 2.0–2.2 秒 | 什么也不等（1）；`reload` 不先等（16） | 不等第一次续期（2）：在途的集合等着那次续期；不等在途的（3）：答复读完之前 `performance` 里没有那一条；不看锁（18） | C 的原型 proto 2；失败的一侧照第 1 格推出 |
| 3 | Chromium、Firefox | 落到 `/sign-in` 是它的期望：原型 6/6，地址 `/sign-in?next=%2Fsettings%2Fsecurity`，会话撤销，续期 2 次 | —（对照） | — | 原型 |
| 4 | 三种 | 绿：原型 9/9，帮手等了 3.06–3.23 秒，代数 2 | 不等第一次续期（2）：Chromium、Firefox 6/6 落到 `/sign-in`（错的帮手 6–27 ms 内就请求了新文档）；WebKit 3/3 以 `problems` = `["the old document never sent its refresh"]` 失败。什么也不等（1）同样 | 不看锁（18）：原型 9/9 照过 | 原型 |
| 5 | 三种 | 绿 | 不等在途的（3）；什么也不等（1）：读被换文档打断 | 不等第一次续期（2）、不看锁（18）：读不在会话的检查里 | C 的原型 proto 5；失败的一侧照 proto 5 的路由推出 |
| 6 | 三种 | 绿：原型 9/9，新文档在放开之后 5–40 ms 才请求 | 不看锁（18）：原型 9/9 在放开之前 12–23 ms 就请求了；什么也不等（1）同样 | 不等第一次续期（2）：原型 9/9 照过 | 原型 |
| 7 | 三种 | 绿：S1 三轮 0.4–0.8 秒，帮手什么也不用等 | `tokenPaths` 去掉注册（x1）：三种 9/9 以帮手 10 秒的报错"the document's first refresh"失败；只认续期（设计原来的写法）同样 | 什么也不等（1）、不看 `performance`（2）、不等在途的（3）、不看锁（18）：本就没有要等的 | S1（`ce2cc0fc`）与它的记录 |
| 8 | 三种 | 绿：修复一轮三轮 1.4–1.9 秒 | 先看记录再看锁（m20，`8e128a2d` 的次序）、只去掉锁（m18a）、锁与租约都不看（18）、什么也不等（1）：三种 9/9，"once the sign-in wrote its record"；`tokenPaths` 去掉登录（x2）、不记落点（17）也抓 | 不等第一次续期（2）、不等在途的（3） | 修复一轮（`6ec05d13`）；审查 A 的探针 |
| 9 | 三种 | 绿：修复一轮三轮 1.4–1.8 秒 | 去掉租约一支（m21）、租约的键改名（m22）、锁与租约都不看（18）、什么也不等（1）：三种 9/9，"once the lease is let go" | 只去掉锁（m18a）、不等第一次续期（2）、不等在途的（3） | 修复一轮（`995ebf02`）；审查 B 的探针 |

- **WebKit 为什么不跑第 3 格**（决定 351）：WebKit 在请求下一个文档之后，旧文档的请求一个也发不出去，没有 U1 的窗口。
  - P1 设计审查 C 的探针 D：下一个文档被扣 1 秒，旧文档每 50 ms 发的请求到了路由 0 次（Chromium 20 次、Firefox 20–21 次）。探针 B：WebKit 里旧文档的请求报 "TypeError: Load failed"。
  - 原型：旧文档等来"放"之后，续期仍没有发出，3/3 记下 `"the old document never sent its refresh"`，新文档带着原来的令牌续期、没有登出；页面另报一条 "…/__a15/release due to access control checks" 的未捕获错误。
  - 所以这一格在 WebKit 里造不出、也不必防：`test.skip(browserName === "webkit", "WebKit sends nothing from the old document once the next one is requested: no window to leave mid-refresh in (M12/P1 design 3.6)")`。
- **WebKit 为什么跑第 4 格**：帮手对的时候，放开在任何导航之前，三种都绿。WebKit 里它只证明"帮手等第一次续期"：去掉那一样时以 `problems` 失败，不以 U1 的登出失败。
- **为什么确定**：
  - 第 1、3 格的失败由扣住的答复、扣住的文档与旧文档自己开闸排定次序，不看时刻。
  - 第 2、5 格的 2 秒是"帮手等不等"的界：帮手对，它一定等到答复交回；帮手不等，下一个文档在 2 秒之内就请求了，答复丢掉，第 2 格落到登录页、第 5 格的读被打断。
  - 第 4、6 格的 1 秒只决定"什么时候放开"：错的帮手在调用后 30 ms 之内就请求新文档（原型的第 4、6 格），`nextDocument` 先到；对的帮手在等，1 秒之后才放开。只有一台机器慢到错的帮手过了 1 秒才请求新文档，这两格才会放过它。这只影响变异抓不抓得到，不影响对的帮手为绿。
- **第 5 格的读**：取应用登录之后读账户的 `GET /api/v0/me`（`W/services/account.service.ts:12`）。P1 设计审查 C 的原型核对过 `/settings/profile` 发它（`proto25.log` 的请求列表）。
- **标题**：设置页的 h1 是"Settings"（`W/pages/settings/settings-layout.tsx:27`），各栏的标题是 h2：个人资料是"Profile"（`profile-page.tsx:26`），安全是"Password"与"Deactivate account"（`security-page.tsx:18`、`:22`）。断言照这个写，并核对地址（P1 设计审查 C-N3；第一次修订的原型先按 h2"Security"写，没找到）。
- **控制台与失败的请求**：第 1、3 格的 401 在 Chromium 与 WebKit 的控制台各一行（`failedToLoad(401)` 照引擎处理，Firefox 不出，`browser.ts:141-146`）。旧文档那次没交回的续期，各引擎报不报中止不一样，断言只要求 `apiFailures` 含那条 401；确切的列表照实写进 S1 的记录，与两个原型看到的相同：

  | 格 | Chromium | Firefox | WebKit |
  |---|---|---|---|
  | 1 | `["401 POST /api/v0/auth/refresh"]` | `["NS_BINDING_ABORTED POST /api/v0/auth/refresh", "401 POST /api/v0/auth/refresh"]` | `["Load request cancelled POST /api/v0/auth/refresh", "401 POST /api/v0/auth/refresh"]` |
  | 3 | `["401 POST /api/v0/auth/refresh"]` | `["401 POST /api/v0/auth/refresh"]` | 不跑 |

  初稿写"Chromium 报 `net::ERR_ABORTED`"是推断，不对（P1 设计审查 C-M3）。
- **三种浏览器**：`playwright.config.ts:3-4` 的 `threeBrowsers` 加 `|identity\/a15-`。
- **逃生口**：第 1、3 格各一处，门禁的表写 2（3.5）。WebKit 跳过第 3 格不改变源码里的个数。
- **实施时的核对**：S1 先把第 3、4、6 格照原型写进 A15，在三种浏览器各跑三轮，再做别的；原型的源码在 `m12/p1-rev1/a15-proto.spec.ts.txt`。合并之前的三轮核对在 7.3。

### 3.7 S1：`global-setup.ts` 的一行日志（第 50 条）

`networkSettled`（`e2e/global-setup.ts:46-58`）：

- 第一次 `tentative()` 交回非空（有地址在 DAD）：循环之后打 `global setup: waited <ms> ms for the host's tentative IPv6 addresses (<第一次看到的，换行改成 " | ">)`，还有没等完的另加 `, some still tentative at the 5 s deadline`。
- `tentative()` 交回 `undefined`（`ip` 判不了）：打 `global setup: ip could not tell the host's tentative IPv6 addresses; waiting <ms> ms from PostgreSQL's start`，再睡。
- 第一次就是空的：什么也不打（决定 349）。
- 日志走 `console.log`：Playwright 的 list 报告器打出全局准备的标准输出，CI 的 e2e 作业日志里看得见。这一句没有核过，由下面的正向对照核。
- **正向对照**（P1 设计审查 A-M5）：今天本机多数轮第一次就是空的，日志的分支走不到，"没看到这一行"还不能当作"没触发"。S1 的 `c330217f`（6.1 第 6 步）之前，本机临时让 `tentative()` 第一次交回一行固定的文字（例如 `inet6 fe80::1/64 scope link tentative`）、之后交回空，跑一个故事（`cd e2e && pnpm exec playwright test stories/smoke/s1-`，经 `heavy.sh`），看那一行 `global setup: waited … ms for the host's tentative IPv6 addresses (inet6 fe80::1/64 …)` 出现在输出里；再让它交回 `undefined` 跑一次，看另一行。复原之后跑一次，看不到（S1 在本机做不出：复原后每次都真的等了，改由恒交回空的一次证明，3.15 第 8 条）。三次的输出摘进 S1 的记录。之后 CI 的 e2e 作业日志里没有这一行，才算那次没触发。

### 3.8 S2 的文件

| 文件 | 改动 |
|---|---|
| `server/go.mod`、`server/tools/go.mod` | `:5` 的 `toolchain go1.27.1` → `toolchain go1.27.4` |
| `deploy/Dockerfile` | `:23` 换 tag 与摘要，下一行加构建时的断言；`:7`、`:41` 换摘要（3.9、3.11） |
| `server/internal/platform/buildinfo/toolchain_test.go` | 新：`TestTheImageBuildsWithTheModulesToolchain`（3.9） |
| `README.md` | `:16` 的 Go 版本；`:630` 的 PostgreSQL 镜像（3.16 第 10 条） |
| `platform/httpserver/mcpserver/edge.go`、`modules/page/domain/grep.go` | `:106`、`:149` 的注释照 1.27.4 的核对写（3.9） |
| `platform/postgres/pgtest/pgtest.go`、`image_test.go` | `:35` 的镜像；守卫加 README 一行（3.10） |
| `deploy/compose.dev.yaml`、`e2e/fixtures/db.ts`、`deploy/image-smoke.sh` | `:8`、`:13`、`:86` 的镜像（3.10） |
| `.github/workflows/ci.yml` | `setup-node` 两处、`upload-artifact` 两处的 SHA 与注释（3.11）；server 矩阵的 `setup-go` 只让竞态构建的九份取存 Go 的缓存（决定 355，3.16 第 2 条） |
| `Makefile` | `vuln`、`outdated` 两个目标（3.12）；第三、第五份各挪一个整个程序的测试给第八份（决定 354，3.16 第 3 条） |
| `tools/outdated.sh` | 新：`make outdated` 做的事（3.12） |
| `.github/workflows/vuln.yml` | 新（3.12） |
| `pnpm-workspace.yaml`、两份 `go.mod`/`go.sum`、`pnpm-lock.yaml` | 只在 `vuln.yml` 报出时（3.13） |

### 3.9 S2：Go 1.27.4 与它的守卫

写本文时 1.27 线最新是 go1.27.4（go.dev/dl，2026-11-15）。S2 开工时若已有更新的 1.27.x，取最新的，下面各处照改，记进 S2 的偏离。

- **两份 `go.mod`**：`toolchain go1.27.4`。`go 1.27` 不变。`go.sum` 不变。
- **Dockerfile 的 server 阶段**（`deploy/Dockerfile:20-27`）：

  ```dockerfile
  FROM golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS server
  # golang 镜像设了 GOTOOLCHAIN=local：生效的是摘要，它若是别的 Go，就悄悄用它构建。这一行在最前面断言版本；
  # 它与 tag、两份 go.mod 的 toolchain、README 一致，由 server/internal/platform/buildinfo/toolchain_test.go 核对
  RUN test "$(go env GOVERSION)" = go1.27.4 || { echo "golang 镜像是 $(go env GOVERSION)，go.mod 要 go1.27.4：换摘要" >&2; exit 1; }
  WORKDIR /src/server
  ```

  摘要在 S2 开工时用 `docker buildx imagetools inspect golang:1.27.4-trixie` 重读（2026-11-15 读到的在第 1 节）。
- **守卫** `server/internal/platform/buildinfo/toolchain_test.go`，短测试（不要数据库，`-short` 照跑）：

  ```go
  // The image builds with the Go the modules ask for (00-M12 4.3, M12/P1/S2). The golang image sets
  // GOTOOLCHAIN=local, so a toolchain line that moves on while deploy/Dockerfile stays behind builds the
  // release with the older Go, silently: the image's tag, the version its server stage asserts first thing
  // and the version that assertion's message names must all be the two modules' toolchain, and so must the
  // version README.md tells contributors.
  func TestTheImageBuildsWithTheModulesToolchain(t *testing.T)
  ```

  断言（文件都从 `../../../../` 读，同 `pgtest/image_test.go:31` 的写法）：
  1. `server/go.mod` 与 `server/tools/go.mod` 各恰有一行 `^toolchain (go\S+)$`，两者相同，记作 `want`；
  2. `deploy/Dockerfile` 恰有一处匹配跨行的正则：server 阶段的 `^FROM golang:(\S+)-trixie@sha256:[0-9a-f]{64} AS server\n`，之后只许注释行（`(?:#.*\n)*`），接着整行的断言 `RUN test "\$\(go env GOVERSION\)" = (go\S+) \|\| \{ echo "golang 镜像是 \$\(go env GOVERSION\)，go\.mod 要 (go[^：\s]+)：换摘要" >&2; exit 1; \}$`。`"go" + tag`、比较的版本与报错里写的版本三个都等于 `want`，各有一句报错。这样断言是 server 阶段的第一条指令（决定 348），升版本时漏改报错里的一处也红（审查 A-M1、N1，3.16 第 11 条）；
  3. 整个 Dockerfile 恰有一行 `^RUN test "\$\(go env GOVERSION\)".*$`（没有这一条，第 2 条放过"断言写两遍"）；
  4. `README.md` 里 `指定的 Go (\S+?)。` 恰匹配一次，`"go" +` 取出的等于 `want`。

  每条不符时写出文件、找到的与 `want`，README 一条写出它写着的版本（审查 A-N2）。匹配不是恰好一个时，多于一个列出每个匹配的分组，一个也没有时列出以给定前缀起头的行与行号（Dockerfile 是 `FROM golang:` 与 `RUN test `，`go.mod` 是 `toolchain`）。
- **README**：`:16` 的"指定的 Go 1.27.1。"改为"指定的 Go 1.27.4。"。
- **两处注释**：
  - `edge.go:106`：mcpserver 导入 `encoding/json/v2`（`:7`），那个包只在实验打开时存在，实验关了这个包编译不过（CI 的每一份都编译它）。所以核对的就是"S2 的构建为绿"；注释改为"In go1.27.4 it is still behind GOEXPERIMENT jsonv2, on by default …"。1.27.4 若已让它离开实验，照实写，并照总体设计 13.1 第 32 条改（第 8 节）。
  - `grep.go:149`：读 `$(go env GOROOT)/src/regexp/syntax/parse.go` 里 `calcSize` 检查 `maxSize` 的那一行，行号没变就把 "Go 1.27.1" 改为 "Go 1.27.4"，变了连行号一起改。
- **反向对照**：7.2 的变异 9–13。

### 3.10 S2：PostgreSQL 18.7

写本文时 18 线最新是 18.7（postgresql.org，2026-11-12），`postgres:18.7-trixie` 在（摘要见第 1 节）。

- 五处 `postgres:18.6-trixie` → `postgres:18.7-trixie`：`pgtest.go:35`、`compose.dev.yaml:8`、`e2e/fixtures/db.ts:13`、`image-smoke.sh:86`、`README.md:629`（S2 时是 `:630`，3.16 第 10 条）。这五处只按 tag 引用，同今天（总体设计 13.5 第 2 条只要求镜像相同；I1 第 8 节第 1 条）。
- `TestContainerMatchesTheOtherDatabases`（`pgtest/image_test.go:13-29`）加一行：`{file: "README.md", want: []string{"`" + image + "`"}}`。注释"The test container, the development database, the end-to-end stories and the image smoke test run the same image"加上"and README names it"。
- **开发库**：18.x 之间不要 dump/restore（18.7 的发布说明）。`make dev-db` 拉新镜像、沿用旧的卷即可，S2 的记录写一句给本机的开发者。
- **计划的测试**（13.4 第 9 条）在 18.7 上跑：它们在组合根之外的包里，CI 的 `race` 一份与几份成本守卫跑它们。计划换了就在 S2 里查清是哪条语句、为什么，不放宽测试（00 号文档 4.3）。
- 18.7 的说明里 `gist_trgm_ops` 要 reindex：我们的表上没有三元组索引（总体设计 7.1），不涉及；GIN 的修复正对着 `page_search`，不要我们做什么。
- **反向对照**：7.2 的变异 14。

### 3.11 S2：基础镜像的摘要与两个 action

- `deploy/Dockerfile:7`：`node:24-trixie-slim@sha256:af02d57b57719b9657a45199a79264a9437940cd55b3c7b3fc99615654f124a0`；`:41`：`gcr.io/distroless/static-debian13:latest@sha256:1a64f46364620465ef9e9a8f5413757dfdf22695e050bbfd7303eeaf17a59383`。S2 开工时重读；S2 的记录写出每个摘要对应的 Node 版本（`docker run --rm <摘要> node --version`）与读的时刻。
- `ci.yml:229`、`:276`：`actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1 # v7.1.0`；`:184`、`:301`：`actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2`。SHA 在 S2 开工时以 `git ls-remote https://github.com/actions/<名>.git refs/tags/<版本>` 再核一次（I1 用的同一个办法，只读）。

### 3.12 S2：`make vuln`、`make outdated` 与 `vuln.yml`

**Makefile**（放在 `knip` 目标之后，`Makefile:167-169`；下面是 S2 第一轮修复之后的样子，3.16 第 12 条）：

```make
# 依赖的已知漏洞（00-M12 4.3、M12/P1/S2）。Go 一侧 govulncheck 只扫 server/，只报代码调用得到的
# （server/tools 是不随镜像发出的生成器）；npm 一侧 pnpm audit 只看 dependencies，即打进二进制的前端，
# VULN_AUDIT_LEVEL 以下的不挡（vuln.yml 照 low 报给人看，发布的闸门传 high；修不了或与我们无关的写进
# pnpm-workspace.yaml 的 audit.ignore，理由写在那个提交）。两侧都跑完再合起来判。
# pnpm audit 与 make outdated 把依赖的清单发给 npm registry 与 Go 的模块代理；govulncheck 只向 vuln.go.dev
# 要数据库里已知有漏洞的模块的条目，本机的权限分类器拒过它（00-M12 4.3）。所以两个目标只在 GitHub 的
# runner 上跑，本机拒绝；结论看 .github/workflows/vuln.yml。判断与干活写在同一个 shell 行里：make -i（或环境里的
# MAKEFLAGS=i）只忽略一行的失败、接着跑下一行，分成两行时它绕得过去；tools/outdated.sh 自己也判一次（M12/P1/S2）
GOVULNCHECK_VERSION := v1.9.0
VULN_AUDIT_LEVEL ?= low
ci-only = test "$$GITHUB_ACTIONS" = true || { echo "make $@ 只在 GitHub 的 runner 上跑：它把依赖的清单发给外部的服务。结论看 .github/workflows/vuln.yml"; exit 1; }

.PHONY: vuln
vuln: ## 依赖的已知漏洞：Go（govulncheck，server/）与 npm（pnpm audit --prod，级别 VULN_AUDIT_LEVEL）；只在 CI 上跑
	@$(ci-only); \
	status=0; \
	(cd server && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...) || status=1; \
	pnpm audit --prod --audit-level=$(VULN_AUDIT_LEVEL) || status=1; \
	exit $$status

.PHONY: outdated
outdated: ## 只读地列出过时的 Go 模块、npm 包、actions、工具链与 PostgreSQL 的版本、基础镜像的摘要；不挡；只在 CI 上跑
	@$(ci-only); tools/outdated.sh
```

- `go run …@版本` 不进 `server/tools/go.mod`（00 号文档 4.3）。`govulncheck` 找到调用得到的漏洞以 3 退出；`pnpm audit` 有达到级别的公告以 1 退出。
- 写法照 GNU Make 3.81（`Makefile:2`）：`$@` 在配方里展开；`ci-only` 是普通变量。判断与干活写在同一个 shell 行：判断单独一行时，`make -i` 或环境里的 `MAKEFLAGS=i` 忽略它的失败、接着跑下一行（审查 C-M1）。配方 `exit 1`，`make` 报 "Error 1"、自己以 2 退出（3.16 第 1 条）。

**`tools/outdated.sh`**（可执行，`set -uo pipefail`；只读；一节出错照样往下，最后的退出码是 0，除非某一节的命令根本没有；`GITHUB_ACTIONS` 不是 `true` 时自己也拒绝、以 1 退出，直接执行也绕不过；输出的写法与出错的认法见 3.16 第 5 条）：

1. **Go 模块**：`server` 与 `server/tools` 各 `go list -m -u -f '{{if and (not .Main) (not .Indirect) .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}' all`，行首写模块。
2. **Go 工具链**：`server/go.mod` 的 `toolchain` 与 go.dev 的 `?mode=json&include=all` 里同一小版本线的最新（`include=all` 让旧的小版本线也读得到，审查 C-N3；`curl` 取、`jq` 解析，分开跑）。
3. **npm 包**：`pnpm outdated -r`（有过时的以 1 退出，接 `|| true`）。
4. **actions**：从 `.github/workflows/*.yml` 取 `uses: <owner>/<repo>@<sha> # <版本>`，每个以 `gh api repos/<owner>/<repo>/releases/latest --jq .tag_name` 比较，不同就列出。只对读到的写"没有过时的（n 个 action）"，读到一部分时写"读到 k 个，共 n 个"，一个也没读到只有 `!!` 行（审查 C-M2）。
5. **基础镜像**：`deploy/Dockerfile` 的每个 `FROM <镜像>:<tag>@sha256:<摘要>`，以 `docker buildx imagetools inspect <镜像>:<tag> --format '{{json .Manifest}}'` 取、`jq -r .digest` 解析（分开跑），与摘要比较，tag 指向了新的摘要（Debian 的安全更新会这样）就列出；同第 4 节，只对读到的写"没有过时的"。
6. **Node 与 PostgreSQL 的补丁版本**：`.node-version` 的大版本在 `nodejs.org/dist/index.json` 里的最新；`pgtest.go` 的镜像与 `postgresql.org/versions.json` 里 18 的最新（I1 用的同两个出处）。

**`.github/workflows/vuln.yml`**：

```yaml
name: Vulnerabilities

# 依赖的已知漏洞（00-M12 4.3、M12/P1/S2）：每天、手动，与改了依赖清单或这里的推送时跑 make vuln。
# 不在 ci.yml 里：Go 的下一个安全版本一出，每个 Step 都会一起红。发布的闸门另跑一次（P5/S1）。
# pnpm audit 与 make outdated 把依赖的清单发给外部的服务，本机的权限分类器也拒过 govulncheck：只在这里跑，本机不跑（00-M12 4.3）。
# 定时运行的通知只发给最后改 cron 的人：每个 Step 合并之前主持者看一眼
#   gh run list --workflow vuln.yml --branch main --limit 1
on:
  schedule:
    - cron: "23 4 * * *"
  workflow_dispatch:
  # workflow_dispatch 与 schedule 只对已在默认分支上的工作流生效：引入它的分支与升依赖的分支靠 push 跑。
  # 只在分支上：推 tag 时发布的闸门自己跑 make vuln
  push:
    branches: ["**"]
    paths:
      - .github/workflows/vuln.yml
      - Makefile
      - tools/outdated.sh
      - server/go.mod
      - server/go.sum
      - pnpm-lock.yaml
      - pnpm-workspace.yaml
      # 它的 packageManager 定了 corepack 装哪个 pnpm 来跑 pnpm audit（audit.ignore 的读法随 pnpm 的版本）
      - package.json

permissions:
  contents: read

# 同 ci.yml：分支上的新推送取消较早的运行；main 的每次运行各占一个组，定时的与推送的撞在一起时，
# 排队的那个不会被更晚的替掉
concurrency:
  group: ${{ github.ref == 'refs/heads/main' && format('vuln-main-{0}', github.run_id) || format('vuln-{0}', github.ref) }}
  cancel-in-progress: true

jobs:
  vuln:
    name: vuln
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      # 不取也不存 Go 的缓存（M12 决定 355）：setup-go 的键由 runner 的系统与架构、Go 的版本与所列 go.sum 的哈希组成，工作流之间共用，
      # 键归先存下的那一份、存下就不再改。照设计列 go.sum 时，这一项的键与 ci.yml 的 e2e 相同，outdated 一项的与 ci.yml 竞态的各份相同；
      # 它们一分钟内跑完，先存下的没有竞态的编译结果，竞态的各份取回它照样从头编译（道理写在 ci.yml 的 setup-go 旁边）（M12/P1/S2）
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: server/go.mod
          cache: false
      - uses: actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1 # v7.1.0
        with:
          node-version-file: .node-version
      - name: Enable corepack
        run: corepack enable
      - name: Vulnerabilities
        run: make vuln

  # 只列出过时的，不挡：它自己出错也不让这次运行变红（continue-on-error），主持者看的是 vuln 一项的结论
  outdated:
    name: outdated
    if: github.event_name != 'schedule'
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    continue-on-error: true
    env:
      GH_TOKEN: ${{ github.token }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      # 同 vuln 一项：不存 Go 的缓存
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: server/go.mod
          cache: false
      - uses: actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1 # v7.1.0
        with:
          node-version-file: .node-version
      - name: Enable corepack
        run: corepack enable
      - name: Install
        run: pnpm install --frozen-lockfile
      - name: Outdated
        run: make outdated
```

- **`pnpm audit` 要不要先装**：pnpm 的文档说它查"已安装的包"，又说它对着锁文件判。`vuln` 一项先不装；S2 第一次推送时它若报要 `node_modules`，加一步 `pnpm install --frozen-lockfile`（同 `ci.yml:235-244` 的缓存），写进 S2 的偏离。S2：不装也跑得了，`vuln` 一项不加（3.16 第 6 条）。`outdated` 一项装：`pnpm outdated` 比较的是装了的版本。
- **`audit.ignore`**：S2 不预先建这一节；`vuln.yml` 报出的公告照 3.13 的五步都修不了、或确认与我们无关时，在 `pnpm-workspace.yaml` 加

  ```yaml
  # pnpm audit 不挡的公告，各注明理由（00-M12 4.3）：
  audit:
    ignore:
      - GHSA-xxxx-xxxx-xxxx # <包>：<为什么修不了或与我们无关；试过 3.13 的哪几步>
  ```

  键的写法照 pnpm 12 的文档（`audit.ignore`）；加之后 `vuln.yml` 的下一次运行要不再报它，作为键写对了的证据。S2 为 katex 的 GHSA-238p-pmpm-9mq7 建了这一节（决定 353），下一次运行 43664040257 为绿、"1 ignored: 1 low"。
- **`ci-wait.sh`**：读 `vuln.yml` 的运行写 `ci-wait.sh <sha> vuln.yml`（`b54cd679`）。
- **并发**（P1 设计审查 B-N1）：同一组里只留一个在跑、一个在排队，更晚来的替掉排队的那个，与 `cancel-in-progress` 无关。初稿在 `main` 上共用一个组，每天的定时运行与一次改了依赖清单的推送撞上时，可能有一次被取消，`gh run list --workflow vuln.yml --branch main --limit 1` 就看到 `cancelled`。照 `ci.yml:7-11` 的写法改。
- **反向对照**：在本机跑 `make vuln` 与 `make outdated`，各自拒绝、`make` 以 2 退出（配方 `exit 1`，3.16 第 1 条），什么也没发出；`make -i`、`MAKEFLAGS=i` 与直接执行 `tools/outdated.sh` 一样拒绝（3.16 第 12 条）。7.2 的变异 15、15b–15e、split 是它的反面：把判断去掉，用 PATH 里的桩证明配方真的往下走（不联网）；同一套桩加上 `GITHUB_ACTIONS=true`，证明原样的配方在 runner 上两侧都跑、合起来失败（配方以 1、`make` 以 2 退出）。`vuln.yml` 的 `push` 路径：S2 的分支改了它自己，第一次推送就该跑；S2 的记录写出那次运行的号。

### 3.13 S2：`vuln.yml` 报出的模块与包

00 号文档 4.3："S2 先推一次分支，读结果，再升报出的模块。"

- 提交到第 6 节 S2 的第 5 个提交为止，推送分支；读 `vuln.yml` 的那次运行（`ci-wait.sh <sha> vuln.yml`；作业日志要认证，`gh run view <id> --log`）。
- **Go 一侧报出的模块**：升到修了的最小版本（`go get <模块>@<版本>`、`go mod tidy`，`make lint-go` 的 `go mod tidy -diff` 照过）。报的若是有连带的模块，写明连带、照做或请主持者定：River（新的版本若带新的迁移，`server/migrations/river_test.go` 的 `TestTheRiverMigrationIsRiversOwn` 会失败，要另加一条迁移）；`x/text`（换 Unicode 表就要 `nervewiki reindex`，总体设计第 14 节）；pgx、goose。
- **Go 一侧报出的标准库**（P1 设计审查 B-N4）：`go get` 修不了，修法是把两份 `go.mod` 的 `toolchain` 升到修了它的 1.27.x，照 3.9 改每一处：Dockerfile 的 tag、摘要与断言，README 的版本，守卫照样核对；`edge.go`、`grep.go` 的两处注释照新版本重核。3.9 只写了"开工时若已有更新的就取最新"，这一条管"开工之后才出、由这次运行报出"的。提交写成 `server, deploy, docs: Go 1.27.<k> for <公告> (M12/P1/S2)`。S2 合并之后才报出的，照 00 号文档 4.3 走旁支（第 9 节）。
- **npm 一侧报出的**（P1 设计审查 B-M1）：打进二进制的依赖在 `web/apps/web/package.json` 与 `web/packages/api-client/package.json` 的 `dependencies`，根的 `package.json` 只有 `devDependencies`（第 1 节）。照下面的次序修：
  1. **声明的范围之内修得了的**：`pnpm update -r <包>@<修了的版本>`，或 `pnpm update --filter @nervewiki/web <包>@<版本>`。不带 `-r` 或 `--filter`，根目录的 `pnpm update` 只看根这一个项目，web 的那一份不动（`pnpm update --help`：`-r` 是"Run the command for every project in the workspace instead of only the project in `--dir`"）。版本照 13.5 第 2 条精确写。对传递依赖（不是哪个项目的直接依赖），pnpm 12 不认带版本的写法（"is not a direct dependency, so the requested version cannot be recorded"）：写不带版本的 `pnpm update -r <包>`，它在上一级声明的范围之内取满 `minimumReleaseAge` 的最新（S2 的 dompurify，3.16 第 7 条）。
  2. **修好的版本在上一级声明的范围之外**（mermaid、katex 底下的传递依赖多是这样）：在 `pnpm-workspace.yaml` 手写 `overrides`，键写到报出的包与它的受影响范围（例如 `"<包>@<受影响的范围>": "<修了的版本>"`），注释写公告号与理由。不在本机跑 `pnpm audit --fix`：它要查审计的接口（决定 308）。
  3. **修复版本发布不满一天**：`minimumReleaseAge: 1440` 是严格的，太新的版本直接报错（`pnpm-workspace.yaml:12-16`）。等它满一天再装；急的写进 `minimumReleaseAgeExclude`，注明公告号与理由，满一天之后的下一个提交删掉这一项。
  4. **katex 与 `@lezer/markdown`**：补丁按确切的版本挂（`pnpm-workspace.yaml:33-34`，`katex@0.16.47`、`@lezer/markdown@1.7.2`）。哪怕只升一个补丁版本，补丁也要照 13.5 第 5 条重做、重核（补丁修的两件事各有测试，M6/P6、M6 收尾），文件名与键随版本改。
  5. **只有上面几步都走不通**（没有修了的版本；修了的版本要升功能性的大版本，例如 mermaid 12；或确认与我们无关，例如只在 Node 的服务端用到的代码），才写进 `audit.ignore`。理由写明试过哪几步、为什么不行，进 v0.2 的 `M12-release.md`（收尾时写）。

  验证照旧靠推送之后 `vuln.yml` 的下一次运行：`pnpm-lock.yaml` 或 `pnpm-workspace.yaml` 改了，`push` 触发它。
- 这些提交照提交说明写，范围照改到的部件（`server`、`web`），说明写报出的公告号。之后再推，`vuln.yml` 为绿才算 S2 的这一条完成（00 号文档第 3 节第 1 条）。
- 什么也没报出时，没有这一组提交，记录写"那次运行为绿"。

### 3.14 每个新检查的反向对照

| 检查 | 反向对照 | 在哪里 |
|---|---|---|
| 门禁 | 内置的九个样例每次运行先跑，比整张问题表；八种结尾都读；读到的文件里没有帮手的模块就失败；7.2 的变异 4–8、19、x6、x7、g1、g1b、g2、c3 在真的树上各改一处 | 3.5、7.2 |
| 帮手 | A15 的第 1、3 格经逃生口确定地落到 `/sign-in`（第 3 格在 Chromium 与 Firefox）；第 2、4、5、6、8、9 格经帮手不落，第 7 格经帮手立刻换；7.2 的变异 1–3、16、18、x1、x2、m18a、m20–m22 各去掉帮手的一样，3.6 的表里写的那几格（x2 另有 A3、S4）就失败 | 3.6、7.2 |
| `signInLandings` | A15 的第 1、3、8 格断言它的内容，第 1 格另断言同一地址再报一次不记、离开又回到记两条；变异 17、x3、x4、m23 | 3.3、7.2 |
| 第 50 条的日志 | 本机临时让等待发生一次，看到那一行；`tentative()` 恒交回空时没有（复原后本机每次都真的等了，3.15 第 8 条） | 3.7 |
| Go 版本的守卫 | 变异 9–12（10 分 10a、10b）、x2、x3、x7、x7b、x10、x11、r1–r3；r6–r8 不该报，照旧不报；变异 13 由构建时的断言抓 | 3.9、7.2 |
| PostgreSQL 的守卫加 README | 变异 14；x6a–d（另四处） | 3.10、7.2 |
| `make vuln`、`make outdated` 在本机拒绝 | 本机各跑一次，`make -i` 与 `MAKEFLAGS=i` 各一次；变异 15、15b、15c、split（PATH 里的桩，不联网）；`tools/outdated.sh` 自己拒绝：直接执行一次；变异 15d、15e | 3.12、7.2 |
| `make vuln` 两侧都跑、合起来判 | `GITHUB_ACTIONS=true` 下桩的四种退出码；变异 x4、x5 | 3.12、7.2 |
| `outdated.sh` 只对读到的写"没有过时的"，写出出错的命令 | 桩下的 O2、O4–O7（O4、O5 修前写"没有过时的"、修后不写） | 3.12、3.16 第 5 条 |

### 3.15 S1 的实施对设计的修正

S1 的审查与修复见 [S1 的审查记录](reviews/P1-S1-e2e-navigation-review.md)，结果在 10.1。与上文最初的设计不同之处已改在各节里（头的状态，第 0 节，2.3 的决定 339、342，3.2–3.7，3.14，第 4、5 节，6.1，7.1、7.2、7.4，第 8 节第 3、5、7 条）；00 号文档的 4.4、第 7 节 S1 一行、第 7 节"总体设计的修订"第 12 条与第 11 节的决定 339、342 同改；设计 3.4 的脚本与输出贴在 S1 的记录。写了行号的，网页与已有故事的是 S1 开始时 `179a7aed` 的，S1 的代码是头 `937df215` 的；提交见 10.1。

- **S1，实施时定下的**（第 1–8 条：实施报告第 7 节的 8 条与第 8 节自己加的；三位审查者各在自己的范围里判过，都对：A 判第 1–3、5 条，B 判第 4、6 条与变异，C 判第 7 条与替换、逐条判进不进本节。第 50 条那一行写 `ip` 第一次看到的全文、时限的秒数取 `settleMs`，与 3.7 写的相同，不列；实施者没有要编号的产品决定，"实施中定"的几条都记在本节，主持者可以给号或改判）：
  1. **A6 的第二个页面测试自己开的标签页先 `watchPage`**：第 1 节与决定 341 说故事自己开的标签页都先受监视，`a6-sign-out.spec.ts:88` 的 `tabB` 没有（`context.newPage()` 之后直接载 `/favicon.svg`）。照决定 341 帮手对它直接失败，所以另加一个提交给它加上（同 `:46-47`、`:120-121`），替换的提交仍是纯机械的。不给帮手开"没被监视、还在 `about:blank` 的页面放过"的口子：那样决定 341 就有了例外（6.1 第 1 步）。
  2. **帮手等"这个文档有了给它令牌的答复"，不只是第一次续期**（3.2 的 `sessionWaits`、决定 339）：在这个文档里登录或注册的，令牌来自那次答复（`W/session/token-manager.ts:164-172` 的 `signIn` 在锁里写记录），不发第一次续期；照原来的写法，第一次全量 `make e2e` 里 A3 (page) 与 S4 在登录页登录之后换文档时等满 10 秒失败（538 个里 2 个）。设计以为"有记录的文档都先续期"，只对开始时就有记录的文档（`token-manager.ts:221-229`）成立。改为文档的 `performance` 里有 `/api/v0/auth/refresh`、`/api/v0/auth/login`、`/api/v0/auth/register` 任一条即可（常量 `tokenPaths`）。给文档令牌的只有这三种答复（审查 A 逐个数过入口）。注册一支由 A15 第 7 格守（变异 x1），登录一支由 A3、S4 与 A15 第 8 格守（变异 x2）。开始时没有记录、之后跟着别的标签页登录的文档，在它下一次读（`me`）时于锁里续期，帮手等的是那次续期（审查 A 的探针，三种浏览器在 42–242 ms 内换了文档；实施报告说它会等满 10 秒，不成立，P1S1A-M2）。
  3. **页面里的一次读至少 1 秒**（3.2 第 2 步）：与"剩下的时间"赛跑时，最后一圈的读总输给几毫秒的计时，报错写成 "the next document (pending)"，不是真在等的一样；改为 `Math.max(1_000, 剩下的)`（常量 `readMs`），帮手最多约 11 秒才失败（测试 30 秒）。Chromium 里下一个文档没提交时读不答，照旧算作 "the next document (pending)"。
  4. **A15 的第 5 格先断言路由记下的结局**（"answered" 或 "dropped"），再断言答复是 200（3.6）：被换文档丢掉的、路由扣着不交回的请求，Chromium 既不报答复也不报失败，只等 `response()` 会等到测试超时；错的帮手立刻以 `Expected "answered", Received "dropped"` 失败（变异 1、3）。
  5. **`signInLandings` 按上一次主框架的地址去重**（3.3 写"与上一条相同就不重复记"）：`replaceState` 再报同一地址时一样不记；`/sign-in` → 别处 → 同一个 `/sign-in` 照记两次，按"上一条"会漏掉第二次。A15 的流程里应用从不再报同一地址（守卫以 `<Navigate replace>` 从设置页换到登录页，地址不同，`W/app/guards.tsx:53`），去掉去重的变异 x4 原来全照过；第 1 格末尾把登录页的地址以它自己 `replaceState`，断言仍是一条（`8e128a2d`，x4 由此抓到）；再 `pushState` 到 `/`、回到同一个登录页的地址，断言两条相同的落点（修复一轮 `6e48227d`，审查 C-N6：设计的写法 m23 在 `8e128a2d` 上活、在头上第 1 格三种浏览器 9/9 失败）。
  6. **A15 九格**（3.6、第 5 节）：第 7 格"自己注册的文档没有第一次续期，帮手立刻换"，钉住 `tokenPaths` 里的注册一支，也钉住"什么也不用等时帮手不等"；第 8 格"登录答 200 之后、写记录还在等锁时经帮手换文档，等锁放开"（第 9 条）；第 9 格"没有 Web Locks 时，别的标签页的租约有效时经帮手换文档，等租约放开"（审查 B-M1：原来没有一格走到租约，去掉租约一支或改了它的键，全部故事照过）。27 个测试，WebKit 的第 3 格跳过；`make e2e` 565 个（538 + 27）。
  7. **门禁**（3.5）：另核对读到了帮手的模块（`e3059737`）：内置样例直接交给 `check()`，证明的是 `check()` 看得见，不是遍历读到了 `e2e/`；只读 `e2e/stories` 的变异 5 原来只有"在 `wiki-health.ts` 手写一处"的探针抓得到，读错了目录时样例照绿、真的树 0 个文件也照过。内置样例比较整张问题表，不只比个数。修复一轮（`91718e48`，审查 C-N5）：读 Playwright 默认 `testMatch` 的八种结尾（`.ts`、`.tsx`、`.mts`、`.cts` 与 `.js`、`.jsx`、`.mjs`、`.cjs`；3.5 写的四种之外，`.spec.tsx`、`.spec.cts` 等的故事照跑、门禁看不见），每次运行先核对八种结尾都读；逃生口认三种引号（反引号 oxfmt 不改）；九个样例，新的两个是反引号写的逃生口、另一个叫 `navigation.ts` 的模块里的直接 `goto`（帮手按整个路径例外，不按名字）。
  8. **3.7 的"复原之后看不到"在本机做不出**：今天本机复原后的代码每次都真的等了（0.2–1.1 秒，容器 veth 的链路本地地址），这一行照实出现；"没有就不打"改由让 `tentative()` 恒交回空的一次运行证明（没有这一行）。
- **审查与修复带来的**（修复一轮，照主持者的修复说明；提交见 10.1）：
  9. **帮手先看续期的锁（或租约），再看记录**（3.2 的 `sessionWaits` 原来先看记录，没有就交回空，走不到锁）：登录、注册的答复读完之后，`signIn` 在锁里写记录（`token-manager.ts:164-172`）；锁被别的标签页持着的那一段，文档还没有记录，原来的写法不等就换文档，新文档丢了这次登录、落到 `/sign-in`（审查 A-M1：探针在三种浏览器里都在放开之前约 1 秒就请求了新文档）。锁之后才读记录，连"读到记录为空之后、看锁之前记录被写下"的那一刻也盖住了（修复核对 A）。A15 第 8 格守它：原来的写法三种浏览器各三轮都失败（`6ec05d13`）。没有记录的应用文档在别的标签页持锁时也等，持锁的只有续期、登录、退出，至多 8 秒；有记录、第一次续期正排在锁上的文档，报错写 "the refresh lock"，等的东西不变（修复核对 A）。
  10. **失败时页面状态的 `sent back to sign in:` 一行只在页面落到过 `/sign-in` 时写**（3.3 写 `|| "no"`，每个失败都带这一行；审查 C-N7，`3c2425c1`）。故事自己去登录页的也有这一行（没登录被守卫转去、在 UI 上登录、有意退出或停用；A15 第 8 格即是一例）：数"被跳回登录页"红过几次（决定 334、342），数这一行带 ` after a refresh answered 401` 的失败，再除去有意造重用的故事（A5）。`3c2425c1` 的提交说明写"有这一行的失败就是被跳回的"，不对，已推、不改；`937df215` 的说明更正了它，代码的注释随之（修复核对 A-N2 = C-N8）。修复一轮取"只在落到过时写"，不改成计数：计数的写法（`sent back to sign in: 0`）还得读出数来比（实施中定）。
- 修复一轮另定的（实施中定，主持者可以给号或改判）：第 9 格照审查 B 的探针，上下文的 init script 删掉 `Navigator.prototype.locks`，先断言页面里 `"locks" in navigator` 为假，租约写成别的标签页的（`owner: "another-tab"`，5 秒，长于帮手的 1 秒窗口）；门禁的八种结尾不进 `SAMPLES`（`SOURCE` 用在遍历里，不在 `check()` 里），每次运行另核对 `x1-sample.spec.<结尾>` 八个名字都被 `SOURCE` 取到；`page["goto"](…)`、`.bind`、`location.href = …` 照决定 344（按文字找）不管，今天 `e2e/` 里一处也没有（审查 C 也这样建议）。

### 3.16 S2 的实施对设计的修正

S2 的审查与修复见 [S2 的审查记录](reviews/P1-S2-toolchain-review.md)，结果在 10.2。与上文最初的设计不同之处已改在各节里（头的状态，第 0 节，2.3 的决定 346，3.8、3.9、3.10、3.12、3.13、3.14，6.2，7.1、7.2、7.4，第 8 节第 6、7 条）；00 号文档的 4.3（`vuln.yml` 的 `paths`）、第 7 节 S2 一行与"分份"、第 11 节的决定 353–355 同改。写了行号的，是 S2 的头 `80d76191` 的；提交见 10.2。

- **S2，实施时定下的**（实施报告第 10 节的 8 条与第 11 节；三位审查者各在自己的范围里判过（A 工具链、镜像与 Go 模块，B 分份、缓存与 CI，C `make vuln`、`outdated.sh`、`vuln.yml` 与 npm）：第 2 条的理由经审查 B 推翻、照修复一轮改写，第 8 条的说法照审查 A 改，其余都对；实施者没有要编号的产品决定，"实施中定"的几条记在 S2 的记录）：
  1. **本机拒绝时 `make` 以 2 退出**：配方 `exit 1`，`make` 报 "Error 1"、自己以 2 退出；3.12、决定 346 写的"退出码 1"是配方的。变异 15 看的同样是 2。
  2. **Go 的缓存只给竞态构建的九份**（`ci.yml` 的 server 矩阵，决定 355；3.12 的 `vuln.yml` 写 `cache: false`）。setup-go 的键由 runner 的系统与架构、Go 的版本与 `cache-dependency-path` 所列文件（默认 `go.sum`）的哈希组成，工作流之间、`ci.yml` 的十六份之间共用；键归先跑完、先存下的那一份，存下就不再改。竞态与不带竞态的编译结果不共用（审查 B 的本机实验：先用 `go test -short ./...` 填满的缓存，对竞态的构建零收益，22.7 对 22.9 秒）。S2 之前 `main` 的键是 2026-10-17 的 `bootstrap-1` 存的；S2 改了 Go 与 `go.sum`，分支第一、二次没有缓存，第三到五次取回 `vuln.yml` 的 `outdated` 一项存下的 3 MB，组合根在 7763 上 Test 减包都是 81–88 秒，包的时间没变。不改的话合并之后 `main` 的新键由最先跑完的 `short`（不带竞态）存下，每一次都这样（审查 B 的 I1；实施时以为 3 MB 是原因，`2811d13d` 只让 `vuln.yml` 不存，P1S2B-M1）。改后七份（`short`、`costs-*`、`guards-*`）不取也不存，只少约 2 秒的模块下载；`vuln.yml` 的两项不取也不存。判断看是哪一份存的键（post 一步的 "Cache saved with the key"），不看大小。
  3. **挪份（决定 354）**：第三份的 TestTheSearchIsWhatEachReadAllows、第五份的 TestTheWikiToolsAnswerEveryCode 挪到第八份（也进第七份的模式）。以竞态的份取回竞态的缓存为前提（第 2 条），折到 7763 暖的时候第三、四、五份约 396、396、382 秒，第八份约 352；冷的第一次加约 50 秒，两次的平均约 407–421，在 450 之下，不分第九份。八份、19 项不变。
  4. **变异 10 两份 `go.mod` 各一格**（10a、10b）；**变异 13 用 Dockerfile 原来的 `golang:1.27.1-trixie` 的摘要 `3b77fc61…`**（这个 tag 今天指向 `8f58fd67…`；原摘要是"只改 tag、漏换摘要"的真样子，也正是 3.9 要摘要与断言并存的理由）。
  5. **`tools/outdated.sh` 的输出**（3.12 只写了六节与"一节出错照样往下"）：每节一行 `== ` 标题，出错打 `!! ` 一行、照样往下，末行 `outdated: <n> sections, <k> with errors`；`go list -m -u -f` 对不列的模块也打一个空行，滤掉；`pnpm outdated -r --format list` 认 `ERR_PNPM` 或大于 1 的退出码为出错；Node 一节只列线上最新（镜像里的 Node 由第 5 节的摘要管）。第 2 节读 `go.dev/dl/?mode=json&include=all`（3.12 写 `?mode=json`：`include=all` 让旧的小版本线也读得到）（审查 C-N3）。第 4、5 节只对读到的写"没有过时的"（"5 个 action"，不是"5 处引用"），读到一部分时写"读到 k 个，共 n 个"；取与解析分开跑，`!!` 行写出出错的命令与它的退出码（修复一轮，审查 C-M2、C-N1）。
  6. **`pnpm audit` 不装 `node_modules` 也跑得了**（3.12 的疑问），`vuln` 一项不加 `pnpm install`。
  7. **3.13 第 1 步对传递依赖**：`pnpm update -r <包>@<版本>` 被 pnpm 12.8.1 拒绝（"is not a direct dependency, so the requested version cannot be recorded"）；不带版本的 `pnpm update -r <包>` 在声明的范围之内取满一天的最新（设计审查 B 留下的未核实一条，这里答了）。
  8. **`vuln.yml` 报出的 Go 模块只在测试里**：`klauspost/compress`、`moby/go-archive` 只经 testcontainers-go 到 `pgtest`，二进制不含（`go list -deps ./cmd/nervewiki`）。二进制链接 goose 的 11 个包，不链接 `klauspost/compress`：goose 的 `go.mod` 以 indirect 要求它，我们链接的 goose 包没有一个导入它（`63de975d` 的说明写"两个都不链接"，不准，审查 A-N3）。3.13 没分这两种，照"升到修了的最小版本"做了，`moby/go-archive` 另带 `moby/sys/user` v0.4.1。
  9. **katex 的 GHSA-238p-pmpm-9mq7 进 `audit.ignore`**（决定 353，3.13 第 5 步）：0.16 线没有修，升 0.18 是 4.3 不升的功能性大版本、补丁要重做；我们的选项以自有属性写 `trust: false`，mermaid 的公式也经它。
  10. **行号**：README 的 PostgreSQL 一行是 `:630`（设计写 `:629`），`05-lessons.md` 的 golang 镜像一句是 `:92`（第 8 节写 `:89`）：S1 的文档提交在它们之前各加了行。
- **审查与修复带来的**（修复一轮，照主持者的修复说明；提交见 10.2）：
  11. **守卫读整行的断言**（审查 A-M1、N1、N2，`42e6ff6f`）：3.9 的第 2、3 条合成一个跨行的正则，钉住断言是 server 阶段的第一条指令（之前只许注释，决定 348），tag、比较的版本与报错里写的版本都等于两份 `toolchain`；另核对 Dockerfile 恰有一行断言；README 取出写着的版本放进报错；匹配不是恰好一个时写出找到的。Dockerfile 不变。
  12. **本机的拒绝与干活在同一个 shell 行**（审查 C-M1，`e8b115c9`）：3.12 的 `Makefile` 原文把 `@$(ci-only)` 单独放一行，`make -i` 或 `MAKEFLAGS=i` 会接着跑下一行；改为 `@$(ci-only); \` 接干活的行，`outdated` 写成 `@$(ci-only); tools/outdated.sh`。`tools/outdated.sh` 自己也判 `GITHUB_ACTIONS`，直接执行也拒绝。
  13. **`vuln.yml` 的 `paths` 加根 `package.json`**（审查 C-N2，`f480a9e5`）：它的 `packageManager` 定了跑 `pnpm audit` 的 pnpm（`audit.ignore` 的读法随 pnpm 的版本）；00 号文档 4.3 的清单随之加（2.1 第 13 条是当时的修订，不改）。

## 4. 网页

不改网页的代码与测试。帮手读网页的几样东西（会话的记录、锁与租约的名字、`#root`、给文档令牌的三个接口的路径），照它们今天的值写成常量，注释写出处（3.2）；网页改了它们，A15 会先失败（登录的路径另有 A3、S4；租约的键是第 9 格，S1 的审查 B 之后加）。

## 5. e2e

| 故事 | 版本与 Step | 内容 |
|---|---|---|
| `identity/a15-leave-mid-refresh.spec.ts` A15 续期途中离开 | 页面：S1（三种浏览器；第 3 格只在 Chromium 与 Firefox） | 3.6 的九格。接口版本豁免：接口那一面是 A5 (API)（`a5-refresh-reuse.spec.ts:68`）；MCP 豁免：MCP 不经浏览器的会话 |
| 109 个故事与 `wiki-health.ts` | S1 | 换上帮手（3.4），断言不变 |

- `e2e/playwright.config.ts:3-4` 的 `threeBrowsers` 加 `|identity\/a15-`。
- 故事的个数：A15 加 9 格乘 3 种浏览器，共 27 个，其中 WebKit 的第 3 格跳过（报告里算 skipped）；`make e2e` 改之前 538 个，之后 565 个（10.1）。
- README 的 e2e 一节（`README.md:492-493`）加一条（第 8 节第 6 条）。

## 6. 实施步骤

每个 Step 一个分支，从前一个合并之后的 `main` 开始；分支上的每个提交都绿。提交的写法照 00 号文档第 7 节"提交说明"：代码 `<范围>: <说明> (M12/P1/S<m>)`；分支上的文档 `docs: … (M12/P1/S<m>)`；合并之前的修复写到 Step，核对写 `fix check <k>`。接受本文的文档提交同时照 2.1 改 00 号文档（第 8 节）。

**每个提交单独是绿的**：knip 报没人用的导出（`make knip`），所以帮手与它的第一批用处在同一个提交；门禁要树上没有直接的调用，所以它在替换之后；`signInLandings` 在 A15 之前（A15 断言它）。

### 6.1 S1（`m12-p1-s1`）

1. `e2e: A6's tab that keeps another sign-in is watched from its start (M12/P1/S1)`：`a6-sign-out.spec.ts:88` 的标签页先 `watchPage`，下一步的替换才单独为绿（3.15 第 1 条；实施时加，`479d14e5`）。
2. `e2e: the stories leave a page once its requests and its refresh are answered (M12/P1/S1)`：`navigation.ts`、`watchPage` 调 `followRequests`、110 个文件的替换（3.2–3.4）。本机 `make e2e` 全绿（经 `heavy.sh`）（`bafbd418`）。
3. `e2e, tools, build: a gate keeps the e2e's gotos and reloads on the helper (M12/P1/S1)`：`tools/e2e-navigation/check.mjs`、`Makefile` 的 `lint-web` 一行、`knip.jsonc`（3.5）；表是空的（`c18562fe`）。
4. `e2e: a failed test's page state says when it was sent back to sign in (M12/P1/S1)`：`signInLandings` 与 `pageState` 的一行（3.3）（`6cf1c32c`）。
5. `e2e: A15, a page left while its refresh is out, in three browsers (M12/P1/S1)`：A15 的七格、`threeBrowsers`、门禁的表写 2（3.5、3.6）。之前照 3.6 末条先在三种浏览器跑第 3、4、6 格（`ce2cc0fc`）。
6. `e2e: the run says when it waited for the host's network (M12/P1/S1)`：3.7。正向对照在这个提交之前的工作树上做、复原，不提交（3.7）（`c330217f`）。
7. `tools: the gate fails unless it read the helper's module (M12/P1/S1)`：3.5（3.15 第 7 条；实施时加，`e3059737`）。
8. `e2e: A15 checks that an address replaced with itself is one sign-in landing (M12/P1/S1)`：A15 第 1 格末尾的 `replaceState`（3.15 第 5 条；实施时加，`8e128a2d`）。
9. `e2e: the helper waits for the refresh lock before it reads the record, which a sign-in writes under it (M12/P1/S1)`：先看锁或租约、再看记录，A15 第 8 格，帮手的说明（3.15 第 9 条；审查 A-M1、A-N1、C-N4，修复一轮，`6ec05d13`）。
10. `e2e: A15 checks that without Web Locks the helper waits for another tab's lease (M12/P1/S1)`：A15 第 9 格（3.15 第 6 条；审查 B-M1，修复一轮，`995ebf02`）。
11. `tools: the gate reads every file Playwright runs and counts an escape in backticks (M12/P1/S1)`：八种结尾、三种引号、九个样例与文件头（3.15 第 7 条；审查 C-N4、C-N5，修复一轮，`91718e48`）。
12. `e2e: A15 checks that away and back to the sign-in page is a second landing (M12/P1/S1)`：第 1 格末尾的来回（3.15 第 5 条；审查 C-N6，修复一轮，`6e48227d`）。
13. `e2e: a failed test's page state names the sign-in page only when it was sent there (M12/P1/S1)`：那一行只在落到过 `/sign-in` 时写（3.15 第 10 条；审查 C-N7，修复一轮，`3c2425c1`）。
14. `e2e: the sign-in line in a failed test's page state is counted by its 401 mark (M12/P1/S1)`：`pageState` 的说明，只改注释（修复核对 A-N2 = C-N8；主持者，`937df215`）。
15. `docs: … (M12/P1/S1)`：审查记录、10.1、3.15、00 号文档与总体设计的 S1 部分、移交、README 与 05-lessons（第 8 节）。

### 6.2 S2（`m12-p1-s2`）

1. `server, deploy, docs: Go 1.27.4, the image asserting it, and a test holding the two together (M12/P1/S2)`：两份 `go.mod`、Dockerfile 的 server 阶段、守卫、README `:16`、两处注释（3.9）。README 与守卫同一个提交：守卫核对它。
2. `server, deploy, e2e, docs: PostgreSQL 18.7 wherever the tests, the development database and the smoke run it (M12/P1/S2)`：五处与守卫的 README 一行（3.10）。
3. `deploy: the node and distroless images' current digests (M12/P1/S2)`：3.11 的两个摘要。
4. `build: setup-node 7.1.0 and upload-artifact 7.0.2 (M12/P1/S2)`：`ci.yml` 的四处（3.11）。
5. `build: make vuln and make outdated, run daily, by hand and on a changed dependency list in vuln.yml (M12/P1/S2)`：Makefile、`tools/outdated.sh`、`vuln.yml`（3.12）。之后推送分支，读 `vuln.yml` 与 `ci.yml` 的两次运行。
6. 视 `vuln.yml` 报出的而定：`server: <模块> <版本> for <公告> (M12/P1/S2)`、`server, deploy, docs: Go 1.27.<k> for <公告> (M12/P1/S2)`（标准库）、`web: <包> <版本> for <公告> (M12/P1/S2)`、`build: pnpm audit leaves <公告> aside, <理由> (M12/P1/S2)`（3.13）。S2 实施时是四个（`63de975d`、`0f61a57f`、`f3e76505`、`c654db5f`）；之后另加 `vuln.yml` 的缓存（`2811d13d`）、挪份（`28980104`，决定 354）与改正注释（`4a0c2585`）三个；审查之后第一轮修复六个（10.2）。
7. `docs: … (M12/P1/S2)`：审查记录、10.2、3.16、00 号文档与总体设计的 S2 部分、06-environment、05-lessons、03-project-background、移交（第 8 节）。

### 6.3 合并的门禁

02-workflow 第 3.1 节：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`（经 `docs/v0.1/claude_to_codex/tools/heavy.sh`；`TMPDIR` 放 `~/.cache` 下自己的目录，锁固定在 `/tmp`，06-environment 第 3 节）；7.2 的每个变异；7.3 的 A15 三轮；7.5 要量的；分支的 CI 全绿（读法：`ci-wait.sh <sha>`，S2 另读 `ci-wait.sh <sha> vuln.yml`）。

- **S2 的镜像**：`make image`、`make image-smoke` 不能在 worktree 里跑（`Makefile:82-90`、06-environment 第 2 节）：在一个普通的克隆里跑一次，并跑变异 13。
- **合并之前**：主持者看一眼 `vuln.yml` 在 `main` 上最近一次的结论（`gh run list --workflow vuln.yml --branch main --limit 1`；00 号文档第 7 节 P1）。S1、S2 合并时 `vuln.yml` 还不在 `main` 上，`main` 上的列表是空的：S1 没有这一步；S2 看的是自己分支上那次由 `push` 触发的运行（上面的 `ci-wait.sh <sha> vuln.yml`）。`main` 上的这一眼从 S2 之后的 Step 起（P1 设计审查 A-N5）。

## 7. 测试与验证

### 7.1 各项与它们的测试

| 项 | 测试 | 在哪里 | Step |
|---|---|---|---|
| 帮手等在途的续期 | A15 第 2 格（经 `reload`） | e2e，三种浏览器 | S1 |
| 帮手等这个文档的令牌：第一次续期（U1），或在这个文档里登录、注册的答复 | A15 第 4 格（经 `goto`；WebKit 里只证明等第一次续期，3.6）；第 7 格（注册之后不等第一次续期）；A3 (page)、S4（登录之后，Chromium）（3.15 第 2 条） | 同上 | S1 |
| 帮手等续期的锁，先于记录 | A15 第 6 格（第一次续期之后）、第 8 格（登录答复之后、写记录之前，3.15 第 9 条） | 同上 | S1 |
| 没有 Web Locks 时帮手等租约 | A15 第 9 格 | 同上 | S1 |
| 帮手等在途的读 | A15 第 5 格 | 同上 | S1 |
| 经逃生口离开就登出（M1 第 10 节；已知限制） | A15 第 1 格；第 3 格（U1） | 第 1 格三种浏览器，第 3 格 Chromium 与 Firefox | S1 |
| 帮手不改变故事的结果 | 全量 `make e2e`，个数与改之前相同（加 A15） | e2e | S1 |
| 替换是机械的、一个不漏 | 3.4 的计数与反向变换的比较；`check:types` | S1 的记录 | S1 |
| 门禁 | 内置样例（比整张问题表）；读到帮手的模块；八种结尾；变异 4–8、19、x6、x7、g1、g1b、g2、c3 | `make lint-web`，CI 的 `web` | S1 |
| 落到 `/sign-in` 写进页面状态 | A15 第 1、3 格断言 `signInLandings`，第 1 格另断言同一地址再报一次不记、离开又回到记两条；第 2、4 格断言它为空，第 8 格只有起初那一条；那一行只在落到过时写（修复一轮的探针，S1 的记录） | e2e | S1 |
| 第 50 条的日志 | 正向对照：本机临时让等待发生一次与 `ip` 判不了一次，各看到那一行，`tentative()` 恒交回空时没有（3.7；复原后本机每次都真的等了，3.15 第 8 条）；之后 CI 的 e2e 作业日志里照实记 | S1 的记录 | S1 |
| Go 版本一致 | `TestTheImageBuildsWithTheModulesToolchain` | `platform/buildinfo`，`test-go-short` 与 `race` | S2 |
| 镜像真的用 1.27.4 构建 | Dockerfile 的断言；CI 的 `image` 一项，构建日志里 `RUN test …` 一步为绿 | `make image` | S2 |
| CI 用 1.27.4 | 各份的 `Go version` 一步（`ci.yml:155-157`）打出 `go1.27.4` | S2 的分支 CI | S2 |
| `jsonv2` 仍打开 | `mcpserver` 编译与它的测试（实验关了 `encoding/json/v2` 不存在） | CI 每一份 | S2 |
| PostgreSQL 相同 | `TestContainerMatchesTheOtherDatabases`（加 README） | `pgtest`，短测试 | S2 |
| 计划在 18.7 上照旧 | 13.4 第 9 条的计划测试照过 | CI 的 `race` 与成本几份 | S2 |
| `make vuln` 为绿 | `vuln.yml` 在 S2 分支上由 `push` 触发的那次运行 | GitHub | S2 |
| `make outdated` 跑得通 | 同一次运行的 `outdated` 一项。它带 `continue-on-error`，`outdated.sh` 一节出错也退 0，所以"作业为绿"不算证据：S2 的记录贴出它的输出，写明六节各有结果（列出的过时项，或"没有过时的"），没有一节报错（P1 设计审查 A-N6） | GitHub | S2 |
| 本机不跑 | 本机 `make vuln`、`make outdated` 各拒绝一次，`make -i`、`MAKEFLAGS=i` 与直接执行 `tools/outdated.sh` 也拒绝；变异 15、15b–15e、split（PATH 里的桩） | S2 的记录 | S2 |
| 竞态的份取回竞态的缓存 | 每份 setup-go 的 `cache` 输入（九份 `true`、七份 `false`）；存键的那一份 post 一步的 "Cache saved with the key" 是竞态的，`short` 不存；7763 上 Test 减包约 33 秒（决定 355） | S2 的分支 CI 与合并之后两次 `main` | S2 |
| 挪份每个顶层测试恰在一份 | 实施者的 `shards.py` 与审查 B 用 Go 的匹配器另核；变异 p1、p1b | S2 的记录 | S2 |

### 7.2 变异

每个变异在本机对代码改一处，跑它旁边写的测试，要它失败（不提交；改回从备份复原，不用 `git checkout --`；改坏时不跑 `make gen*`）。先在没改过的树上跑一遍同样的命令。A15 的变异（1、2、3、16、18）在"浏览器"一列写的每种浏览器各跑三轮（`--repeat-each=3`），每一轮都要失败。第 10 节记下结果。

| # | 变异 | 让它失败的测试 | 浏览器 | Step |
|---|---|---|---|---|
| 1 | 帮手什么也不等（`settle` 直接返回） | A15 第 2、4、5、6、8、9 格 | 三种 | S1 |
| 2 | 帮手不等这个文档的第一次续期（`sessionWaits` 不看 `performance`） | A15 第 4 格：Chromium 与 Firefox 落到 `/sign-in`，WebKit 以 `problems` 失败（原型各三轮）；第 2、5、6 格照过 | 三种 | S1 |
| 3 | 帮手不等在途的请求（集合不看），只看页面里的会话 | A15 第 5 格（读被打断）；第 2 格照过：第一次续期的检查等着那次续期 | 三种 | S1 |
| 4 | 门禁放过故事里直接的 `goto` | 内置样例 1；在 `a5-refresh-reuse.spec.ts` 里改回一处 `page.goto`，`make lint-web` 失败 | — | S1 |
| 5 | 门禁不看 `e2e/fixtures/`（目录的遍历只读 `e2e/stories`） | 门禁自己：读到的文件里没有帮手的模块就失败（3.15 第 7 条）；另在 `wiki-health.ts` 改回 `page.goto`，`make lint-web` 失败。内置样例抓不到它：样例直接交给 `check()`，不经遍历（P1 设计审查 A-N10）；S1 加那条核对之前，提交的门禁照过它 | — | S1 |
| 6 | 门禁对帮手以外的文件也例外（例外按目录而不是按文件） | 内置样例 2 | — | S1 |
| 7 | 门禁不核对逃生口的表 | 内置样例 4–6；在 A15 多写一处逃生口 | — | S1 |
| 8 | 门禁的正则不认跨行的 `.reload(` | 内置样例 2 | — | S1 |
| 9 | Dockerfile 的 golang 的 tag 改回 1.27.1，别的不动 | `TestTheImageBuildsWithTheModulesToolchain` | — | S2 |
| 10 | 一份 `go.mod` 的 `toolchain` 改回 1.27.1（S2 两份各一格：10a、10b） | 同上 | — | S2 |
| 11 | Dockerfile 的断言一行写 1.27.1 | 同上 | — | S2 |
| 12 | README 的 Go 版本改回 1.27.1 | 同上 | — | S2 |
| 13 | 摘要换成 `golang:1.27.1-trixie` 的（tag 与断言照旧写 1.27.4） | `make image`（普通的克隆里）：断言那一步失败，打出"golang 镜像是 go1.27.1" | — | S2 |
| 14 | README 的 PostgreSQL 改回 18.6 | `TestContainerMatchesTheOtherDatabases` | — | S2 |
| 15 | `ci-only` 的判断去掉 | 本机 `make vuln`，PATH 前面放两个桩（放在 `TMPDIR` 下自己的目录）：`go` 把参数写进一个文件、退出 3，`pnpm` 写参数、退出 0。原样时在调用桩之前就拒绝（文件是空的，`make` 以 2 退出，3.16 第 1 条）；去掉判断后两个桩都被调到（文件里是 `run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` 与 `audit --prod --audit-level=low`），什么也没发出。另用同一套桩加 `GITHUB_ACTIONS=true` 跑原样的 `make vuln`：两侧都跑，合起来以 2 退出（配方以 1，`status` 的累计，3.12）。不必请主持者同意（P1 设计审查 A-M3、B-N3） | — | S2 |
| 16 | `reload` 不先等（只 `goto` 等） | A15 第 2 格 | 三种 | S1 |
| 17 | `framenavigated` 里不记 `/sign-in` | A15 第 1、3、8 格的 `signInLandings` 断言 | 第 1、8 格三种，第 3 格 Chromium 与 Firefox | S1 |
| 18 | 帮手不看续期的锁（`sessionWaits` 不调 `navigator.locks.query()`，也不看租约） | A15 第 6、8、9 格：新文档在放开之前就请求了（原型三种各三轮）；第 4 格照过。审查时只有第 6 格，只由锁那一半抓到：没有 Web Locks 的故事走不到租约（P1S1B-M1）；修复一轮加了第 8、9 格 | 三种 | S1 |
| 19 | 门禁数帮手模块里的逃生口（`LEAVE` 不跳过帮手） | 内置样例 7；真的树上帮手的报错信息被算作一处，`make lint-web` 失败 | — | S1 |
| x1 | `tokenPaths` 去掉注册 | A15 第 7 格：帮手 10 秒后报"the document's first refresh" | 三种 | S1 |
| x2 | `tokenPaths` 去掉登录 | A3 (page)、S4 与 A15 第 8 格：同上的报错 | A3、S4 在 Chromium；第 8 格三种 | S1 |
| x3 | `refusedRefresh` 恒为假 | A15 第 1、3 格：落点少了" after a refresh answered 401" | 第 1 格三种，第 3 格 Chromium 与 Firefox | S1 |
| x4 | 落点不去重 | A15 第 1 格：两条相同的落点（`8e128a2d` 之前全照过，3.15 第 5 条） | 三种 | S1 |
| x5 | 事件流也算在途 | A15 第 2、4–9 格（多数 9/9）：帮手报 `GET /api/v0/events` | 三种 | S1 |
| x6 | 门禁的 `check()` 什么也不报 | 内置样例 | — | S1 |
| x7 | 门禁的遍历读 `web/apps/web/src/session` | 读到帮手的模块的核对；表里 A15 一行也对不上 | — | S1 |
| m18a | 只去掉锁的一支（租约留着） | A15 第 6、8 格 | 三种 | S1 |
| m20 | 先看记录再看锁（`8e128a2d` 的次序） | A15 第 8 格："the next document is asked for once the sign-in wrote its record" | 三种 | S1 |
| m21 | 去掉租约一支 | A15 第 9 格："the next document is asked for once the lease is let go" | 三种 | S1 |
| m22 | 租约的键改名 `nwiki.auth.lease.v2` | A15 第 9 格 | 三种 | S1 |
| m23 | 落点按上一条去重（3.3 原来的写法） | A15 第 1 格：少了第二条落点 | 三种 | S1 |
| g1 | 门禁的 `SOURCE` 改回四种结尾（g1b：少 `x`） | 八种结尾的核对："the gate does not read a story ending .spec.tsx" 等 | — | S1 |
| g2 | `LEAVE` 只认双引号 | 内置样例 8 | — | S1 |
| c3 | 门禁的例外按名字 `navigation.ts`，不按整个路径 | 内置样例 9 | — | S1 |
| x2 | Dockerfile 去掉断言那一行 | 守卫：第 2 条 0 个匹配，列出 `FROM` 那一行；"0 lines assert the Go version" | — | S2 |
| x3 | 断言一行写两遍（相邻） | 守卫："2 lines assert the Go version"（第一轮修复之后由第 3 条抓） | — | S2 |
| x4 | `make vuln` 的 Go 一侧失败不计 | 桩 go 退 3、`GITHUB_ACTIONS=true`：`make` 以 0 退出（原样 2） | — | S2 |
| x5 | `make vuln` 的 npm 一侧失败不计 | 桩 pnpm 退 1：同上 | — | S2 |
| x6a–d | `pgtest.go`、`compose.dev.yaml`、`db.ts`、`image-smoke.sh` 各自改回 18.6 | `TestContainerMatchesTheOtherDatabases` | — | S2 |
| x7 | README 的"指定的 Go 1.27.4。"写两遍 | 守卫 | — | S2 |
| x7b | README 另有一处"指定的 Go 1.27.1。" | 守卫："2 matches …; found ["1.27.4" "1.27.1"]" | — | S2 |
| x10 | `server/tools/go.mod` 多一行 `toolchain go1.27.1` | 守卫："2 matches" | — | S2 |
| x11 | `FROM … AS server` 之后先插一行 `WORKDIR /go` | 守卫：0 个匹配，列出 `FROM` 与断言两行 | — | S2 |
| r1 | 只改断言报错里的版本（`go.mod 要 go1.27.1`） | 守卫："the assertion's message says go.mod asks for go1.27.1" | — | S2 |
| r2 | 断言挪到 `RUN go mod download` 之后 | 守卫：0 个匹配，列出 `FROM` 与 `RUN test` 两行与行号 | — | S2 |
| r3 | 断言挪进 web 阶段 | 同上 | — | S2 |
| p1 | 挪份的两个名字只进第八份、不进第七份的模式 | `shards.py`："NOT IN EXACTLY ONE"（两份都跑） | — | S2 |
| p1b | 第三份的 `-skip` 去掉第七份 | 同上，28 个 | — | S2 |
| 15b | `vuln` 去掉 `$(ci-only);` | 本机 `make vuln` 调到两个桩 | — | S2 |
| 15c | `outdated` 去掉 `$(ci-only);` | 行为上看不出：`tools/outdated.sh` 自己拒绝（第二层），15e 证明两层都要去掉才调到 | — | S2 |
| 15d | `tools/outdated.sh` 去掉自己的判断 | 直接执行调到桩（14 次）；`make outdated` 仍由 `Makefile` 拒绝 | — | S2 |
| 15e | 15c 加 15d | `make outdated` 调到桩（14 次） | — | S2 |
| split | 判断挪回单独的一行（3.12 原来的写法），`make -i vuln` | 调到两个桩 | — | S2 |

x1–x7 是 S1 的实施者加的（实施报告第 4 节）；m18a、m20–m23、g1、g1b、g2、c3 是修复一轮照审查加的（修复报告第 3 节），m20–m23 在 `8e128a2d` 上都活着（m20 即那时的帮手；审查 B 的 mL、mLK 即 m21、m22），g1、g2、c3 的探针那时门禁都看不见。

S2 的 x 与 S1 的 x1–x7 不是同一组，看"Step"一列。x2–x7、p1、p1b 是 S2 的实施者加的（实施报告第 4 节）；r1–r3 出自审查 A，x7b、x10、x11 与 15b–15e、split 是第一轮修复的变异表里的（修复报告第 3 节）。r1–r3、x7b、x11 在 `4a0c2585` 上活着，守卫读整行的断言（`42e6ff6f`）之后死；r6–r8（另有 `golang:1.27.1 AS tools` 的阶段、注释里写旧的 `FROM` 与 `RUN test`、`go.mod` 的注释写旧的 `toolchain`）不该报，之前之后都不报。split 即 3.12 原来的 `Makefile`：`make -i vuln` 在 `e8b115c9` 之前调到两个桩（审查 C-M1）。修复核对 C 的 N7：15b–15e、split、x4、x5 与 3.14 的一格由 S2 的文档提交补上。p1、p1b 只由 `shards.py` 抓到，仓库里没有核对分份的测试。

等价的、不列的：只从在途的集合里去掉续期（`/api/v0/auth/refresh` 不进集合）。在途的续期若是这个文档的第一次，"第一次续期"一样等它：答复读完之前 `performance` 里没有那一条。若是后来的，它在锁里发、写完记录才放锁（`token-manager.ts:288-310`），锁的检查等它。记录里写明它是等价的。

初稿把"只去掉锁的检查"也写成等价，不对：第一次续期答完之后由别人持住锁，完整的帮手等、去掉锁的不等（P1 设计审查 A-I1 的窗口 C），第 6 格把这个区别做成了测试。初稿第 4 格下面写的"变异 2 在本机与 CI 各跑一次"也去掉：变异不提交，在 CI 上跑就得推一个带变异的提交（A-M2）；第 4、6 格的 1 秒是宽的界，错的帮手 30 ms 之内就换页（3.6）。

### 7.3 门禁

6.3。另：

- `make gen-check` 照过（P1 不改契约）；`TestTheToolListsAreTheContract` 照旧（P1 不改 `tools.json`）。
- **A15 的三轮**（S1，主持者照 P1 设计审查定）：实施者在三种浏览器各跑三轮 `cd e2e && pnpm exec playwright test stories/identity/a15- --repeat-each=3`（经 `heavy.sh`）。每一轮全绿，WebKit 的第 3 格是跳过。7.2 的变异 1、2、3、16、18 同样三轮，3.6 的表里写的每一格每一轮都失败。结果与每格的用时写进 10.1。

### 7.4 CI 的余量

- **S1**：组合根不变。e2e 作业今天 6:52–9:23（00 号文档第 7 节 P1），时限 20 分钟（`ci.yml:263`）。
  - A15 加 26 个要跑的测试（27 个里 WebKit 的第 3 格跳过；S1 加了第 7–9 格，第 7 格每次 0.4–0.8 秒，第 8、9 格 1.4–1.9 秒）。有四格在等：第 2、5 格各约 2 秒，第 4 格约 1 秒加 2 秒，第 6 格约 1 秒。本机原型量到的每格用时（含注册与登录）：第 1 格 0.3–1 秒、第 2 格 2.3–2.9 秒、第 5 格 2.3–2.9 秒（P1 设计审查 C 的原型），第 3 格 0.8–1.5 秒、第 4 格 3.9–4.5 秒、第 6 格 1.9–2.3 秒（第一次修订的原型）。三种浏览器加起来串行约 40 秒。
  - `playwright.config.ts` 没有设 `workers`，Playwright 默认用一半的逻辑 CPU，CI 的 4 核 runner 上是 2 个；同一个文件里的测试串行。所以 A15 在 CI 上约多 20–40 秒。这是推断：CPU 不同，并且没在 CI 上量过（P1 设计审查 A-N1）。S1 的分支 CI 上 A15 一轮串行约 41 秒（七格，`8e128a2d`），修复之后约 68 秒（九格，`3c2425c1`，那台 runner 慢约 1.5 倍）（10.1）。
  - 帮手对已有的故事估多 0–30 秒（推断，3.4）。合计估多 1 分钟以内。S1 的记录写出分支 CI 与合并之后两次 `main` 的 E2E 一步。S1 实测：A15 之外帮手只真的等过 6 次、合计 0.23 秒（审查 B 插记录的全量）；分支 CI 的 e2e 作业 7:35（E2E 6:03），S1 之前 `main` 的三次 6:48–8:16，在机器的差别之内；修复之后 9:58（E2E 8:25），慢在 runner（10.1）。合并之后两次 `main` 记在 S2 的记录（7.5 的"之前"）。
- **S2**：组合根各份照新的 Go 与 PostgreSQL 重量（7.5）。第五份收尾之后两次在 EPYC 7763 上平均 425 秒，余量 25 秒（00 号文档第 7 节"分份"）：Go 小版本与 PostgreSQL 小版本的变化通常在噪声之内，但余量小，所以照数据看。过 450 秒照 06-environment 第 4 节先挪给邻份，挪不下再分一份，不提高 `-timeout`。第五份落在别的 CPU 上时看不出 7763 上的影响，所以另照决定 352 看它第一次落在 7763 上的那次（第 8 节第 1 条）。`vuln.yml` 与 `ci.yml` 并行，不算进各份。`image` 一项要拉新的基础镜像，第一次多十几秒。S2 实测：分支 CI 上组合根在 7763 上 417–458 秒，多出的约 50 秒是编译，竞态的份没取回竞态的编译结果（3.16 第 2 条）；改为只让竞态的九份取存 Go 的缓存（决定 355），并把第三、第五份各挪一个给第八份（决定 354），暖的时候 7763 上第三、四、五份约 396、396、382 秒。合并之后第一次 `main` 是冷的，约多 50 秒；看两次的平均（10.2）。

### 7.5 P1 怎样量

- **S1**：A15 单独三轮（`cd e2e && pnpm exec playwright test stories/identity/a15-` 在三种浏览器）；全量 `make e2e` 改之前与之后各一次的用时与个数（经 `heavy.sh`）；分支 CI 与合并之后两次 `main` 的 E2E 一步。第 50 条的日志在这三次 CI 里出没出现。
- **S2，工具链升级前后各量一次组合根**：
  - **之前**：S1 合并之后的两次 `main`（S1 只改 e2e，组合根与 S1 之前相同，Go 1.27.1、PostgreSQL 18.6）。
  - **之后**：S2 合并之后的两次 `main`（Go 1.27.4、PostgreSQL 18.7）；S2 的分支 CI 一次作预览。
  - 记八份组合根与另八份 server 的 Test 一步、e2e 与 image 两项，每次运行写 runner 的 CPU（`Runner CPU` 一步，`ci.yml:159-160`）。只比较同一种 CPU 上的数；没有同一种时写明，不硬比。
  - **第五份与 EPYC 7763**（决定 352）：S2 合并之后，`main` 的第五份第一次落在 EPYC 7763 上时，记下它的 Test 一步，与收尾之后的 425 秒比，写进 00 号文档第 13 节的那一行或下一个 Step 的记录。两次落在 7763 上的平均过 450 秒，照 06-environment 第 4 节挪份。不为等它拖住 S2 的合并。
  - 本机不跑组合根两轮：之前之后的 CI 各两次已够判余量，本机的数又不能当作 CI 的 Test 一步（`ci.yml:29` 的注释）。
- **S2 的其余**：`make image` 在普通的克隆里的用时（拉新镜像之后）；`vuln.yml` 两项的用时。

## 8. 完成标准

P1 完成，当且仅当：

1. 两个 Step 各自合并进 `main`，合并之前 6.3 的门禁全绿、审查与修复核对结束；每个 Step 合并之后两次 `main` 的组合根每一份的 Test 一步平均在 450 秒之下（7.4），否则照 06-environment 第 4 节处理之后才开下一个 Step（S2 合并之后：才开 P2–P5 的并行的几个）。第五份另照决定 352：S2 合并之后，`main` 的第五份第一次落在 EPYC 7763 上时，记下它的 Test 一步，与收尾之后的 425 秒比；两次落在 7763 上的平均过 450 秒时，照 06-environment 第 4 节挪份。不为等 7763 拖住 S2 的合并；那一次来时若已开了后面的 Step，挪份记在当时进行中的 Step 里。
2. 第 3、5 节的每个测试在，7.2 的每个变异被它旁边的测试抓到（第 10 节记下）；3.14 的每个反向对照在。
3. 00 号文档第 3 节第 1、2 条里 P1 的部分有证据：`vuln.yml` 在 S2 分支上为绿的运行号；门禁报出的逃生口个数与表；A15 三种浏览器为绿。第 2 条的"P1/S1 合并之后到 M12 收尾，`main` 的 e2e 没有一次因为被跳回登录页而红"由收尾记录核对（决定 334）：S1 的记录写明从哪个提交起数（S1 的合并），怎样数（`gh run list --workflow ci.yml --branch main`，红了的看 E2E 的注解里 `sent back to sign in:` 一行，数这一行带 ` after a refresh answered 401` 的失败，再除去有意造重用的故事（A5）；故事自己去登录页的也有这一行，3.15 第 10 条）。
4. **00 号文档**（`docs/v0.1/M12-release/00-M12-design.md`）：
   - **接受本文的文档提交**：2.1 的各条照写的新文字改（第 10 条撤回，不改）；2.3 的取舍 337–352 写进第 11 节，接在"2026-11-15：设计审查之后新加的主持者的决定 332–336"之后，另起一节"P1 的设计审查之后的取舍 337–352（主持者的决定，负责人可以改判）"，每条一句、带它在本文与审查记录里的位置（审查记录用临时号 P1-1–P1-16，对照在它的"主持者的决定"一节）；头的"状态"（`:7`）改为"进行中（P1 设计已审查，S1 开工；[审查记录](reviews/M12-design-review.md)；总体设计照第 7 节"总体设计的修订"中标"设计定稿时"的几条改过）"；第 12 节的 P1 一行 `| P1 地基（S1、S2） | — | 未开始 | — | — |`（`:1234`）改为 `| P1 地基（S1、S2） | [01-P1](01-P1-foundation.md) | 进行中（设计已审查） | [设计](reviews/P1-design-review.md) | — |`；第 13 节加一行"P1 设计已审查，S1 开工；2.1 的修订落进本文"，依据是 P1 的设计与它的审查记录。
   - **每个 Step 的文档提交**（在 Step 的分支上、合并之前）：第 12 节的 P1 一行的"状态"写到这个 Step，"审查"一格加它的记录 `reviews/P1-S<m>-<slug>-review.md`，"合并"一格写"见本文第 12 节"的占位，合并之后在 `main` 的小文档提交里填入合并的提交号；第 13 节加一行。
   - **`docs: M12/P1 done (M12/P1)`**：第 12 节的 P1 一行"完成"；头的"状态"改为"进行中（P1 完成；P2、P3、P4/S1、P5/S1 可以开工；…）"；第 13 节加"P1 完成"一行。
5. **v0.1 总体设计**（`docs/v0.1/v0.1-design.md`；00 号文档第 7 节"总体设计的修订"里标给 P1 的第 4、5、12、18 条，各在实现它的 Step 的文档提交里，用脚本断言锚点恰好出现一次）：
   - **S1，13.4 第 3 条**（`:1265`）：末尾（`UP6、UP8 的页面版本只在 Chromium（M11/P3）。` 之后）加 " M12/P1/S1：e2e 换文档只经 `e2e/fixtures/navigation.ts` 的 `goto` 与 `reload`：它们先等页面没有在途的接口请求、续期的锁（没有 Web Locks 时是租约）空着、这个文档有了给它令牌的答复（第一次续期，或在这个文档里登录、注册），再换；有意在请求途中离开的写显式的逃生口 `{ inFlight: "leave" }`。门禁 `tools/e2e-navigation/check.mjs`（`make lint-web`）不许 `e2e/` 里别处直接调 `page.goto`、`page.reload`，按表核对每个文件的逃生口，并核对它读到了帮手的模块；同一个文档里的后退与前进不经帮手。A15 演示续期途中离开会被登出（M1 第 10 节），并核对帮手等的每一样（含在锁里写记录的登录、没有 Web Locks 时的租约）、自己注册的文档不必等；只有页面版本，接口那一面是 A5 (API)；在三种浏览器里跑，第一次续期发出之前离开的那一格只在 Chromium 与 Firefox（WebKit 在请求下一个文档之后旧文档发不出请求）（M12/P1）。"（S1 照实施改了几处：锁先于令牌（没有 Web Locks 时是租约）、令牌的答复、门禁核对读到了帮手；A15 一句多了锁里写记录的登录、租约与自己注册的文档不必等，3.15）
   - **S2，7.1**（`:722`）：`例如 \`postgres:18.6-trixie\`` → `例如 \`postgres:18.7-trixie\``（第 4 条）。
   - **S2，13.5 第 2 条**（`:1286`）：`Node 本身按大版本（\`.node-version\`）。` → `Node 本身按大版本（\`.node-version\`）。Dockerfile 的 golang 镜像的版本（tag 与 server 阶段构建时断言的 \`go env GOVERSION\`）与 README 写给贡献者的版本等于两份 \`go.mod\` 的 \`toolchain\`（golang 镜像设了 \`GOTOOLCHAIN=local\`，\`FROM\` 里生效的是摘要）；测试、开发、部署样例与 smoke 的 PostgreSQL 镜像相同，\`pgtest/image_test.go\` 列出要核对的各处；都由测试核对（M12）。`（第 5 条，加了 2.1 第 11 条的 README）。
   - **S2，13.1 第 32 条**（`:1179`）：从 `go1.27.1 里它在默认打开的 GOEXPERIMENT \`jsonv2\` 之后` 起的那一句（恰好出现一次）照 1.27.4 的核对结果写：仍在实验之后时是"go1.27.4 里它仍在默认打开的 GOEXPERIMENT `jsonv2` 之后（实验关掉时 `encoding/json/v2` 不存在，mcpserver 编译不过，CI 的每一份都会红）：升级工具链时核对它仍打开、或已离开实验（M12/P1）。"；已离开实验时照实写（第 18 条）。
   - **第 15 节**：每个 Step 的文档提交加一行（日期、改了上面哪几条、依据是本文与 Step 的审查记录、`M12/P1/S<m>`）。
6. **其余的文档**：
   - **S1，`README.md:492-493`**（e2e 一节）之后加一条："- 故事与夹具换文档用 `e2e/fixtures/navigation.ts` 的 `goto(page, url)`、`reload(page)`：先等页面在途的接口请求与续期答完，免得打断续期、被登出（M1 第 10 节）。直接调 `page.goto`、`page.reload` 过不了 `make lint-web` 的门禁；有意在请求途中离开的写 `{ inFlight: \"leave\" }`，并在 `tools/e2e-navigation/check.mjs` 的表里写明理由。"
   - **S1，`05-lessons.md:80`**：末尾加"M12/P1/S1 起换文档经帮手 `goto`、`reload`，它先等在途的请求与续期，门禁守着（`e2e/fixtures/navigation.ts`）；断言之前等一个肯定的东西照旧。"
   - **S2，`06-environment.md:10`**：`会自动下载 \`go.mod\` 锁定的 \`go1.27.1\`` → `会自动下载 \`go.mod\` 的 \`toolchain\` 锁定的那个版本`。
   - **S2，`06-environment.md:53`**：`在 \`golang:1.27.1-trixie\` 容器里复现` → `在与 \`go.mod\` 的 \`toolchain\` 同版本的 golang 镜像（\`deploy/Dockerfile\` 的 server 阶段那个）里复现`。
   - **S2，`06-environment.md`** 第 4 节（读 CI，`:50` 之后）加一条："- **依赖的漏洞**：`make vuln`、`make outdated` 只在 GitHub 的 runner 上跑（本机拒绝），结论看 `vuln.yml`：`gh run list --workflow vuln.yml --branch main --limit 1`，或 `tools/ci-wait.sh <sha> vuln.yml`。每个 Step 合并之前看一眼（00-M12 4.3）。"S2 照实施另加：不在 runner 上三者都拒绝、`make -i` 也一样，作业日志的 `-R` 与读 `outdated` 一项的输出；"分份"一条另加 Test 一步含编译的读法（3.16 第 2 条）。
   - **S2，`05-lessons.md:92`**（写本文时是 `:89`，3.16 第 10 条；第 7 节另加三条，S2 的记录）：`在 \`golang:1.27.1-trixie\` 容器里复现` → `在与 \`go.mod\` 的 \`toolchain\` 同版本的 golang 镜像里复现`。
   - **S2，`03-project-background.md:15`**：`Go 1.27（toolchain go1.27.1）` → `Go 1.27（\`toolchain\` 见 \`server/go.mod\`）`。
7. **移交的"处理进展"**各加一行（状态仍是 `open`）。引号里以反引号括起的链接是它在 `handoffs/` 里的写法，写进移交时去掉反引号：
   - **S1**，`handoffs/M5-polish.md`："- M12/P1/S1（2026-11-15，合并见 `[00-M12](../00-M12-design.md)` 第 12 节）：第 51 条做完：帮手 `e2e/fixtures/navigation.ts` 的 `goto`、`reload` 先等在途的请求、续期的锁（或租约）与这个文档的令牌（第一次续期，或在这个文档里登录、注册的答复），110 个文件换上，门禁 `tools/e2e-navigation/check.mjs`，A15 在三种浏览器里演示续期途中离开会被登出；"重新看 M1 第 10 节"进 v0.2 的 `M12-release.md`（收尾时写）。第 50 条：`networkSettled` 等待时打一行；分支的两次 CI 一次触发了（等了 0.42 秒，容器 veth 的链路本地地址）、一次没有（那台 runner 没有暂定的地址）（`[P1 设计](../01-P1-foundation.md)` 10.1）。"
   - **S2**，`handoffs/M0-P6-image-release.md`："- M12/P1/S2（2026-11-15，合并见 `[00-M12](../00-M12-design.md)` 第 12 节）：第 4 条的安全一半做完：Go 1.27.4（镜像的断言与守卫）、PostgreSQL 18.7、node（24.22.3）与 distroless 的摘要、setup-node 7.1.0 与 upload-artifact 7.0.2；`make vuln`（Go 与 npm）与 `make outdated` 在 `vuln.yml` 里，第一次运行报出的：`klauspost/compress` 1.20.2、`moby/go-archive` 0.3.0（只在测试里）、dompurify 3.4.17 升了，katex 的 GHSA-238p-pmpm-9mq7（low，修在 0.18.2）进 `audit.ignore`（决定 353），`vuln.yml` 为绿。Node 26、功能性的大版本（katex 0.18/0.19 与补丁重做在内）与 Dependabot 的版本更新，收尾时写进 v0.2 的 `M12-release.md`（`[P1 设计](../01-P1-foundation.md)` 10.2）。"

## 9. 交给后面的 Phase

- **P2 的各 Step**：新的 e2e 断言换文档经帮手；改到的故事若有意在请求途中离开，用逃生口并进门禁的表。P2/S4 的阅读视图要核对"后退、前进到锚点"（8.1），那是同一个文档里的，不经帮手。
- **P4/S2**：`e2e/perf/` 在门禁的范围里（2.1 第 4 条）。
  - `e2e/perf/` 的页面在第一次导航之前要 `watchPage`：帮手对没被监视的页面直接失败（决定 341）。perf 有自己的 Playwright 配置，未必用 `page` 夹具（P1 设计审查 A-N7）。
  - 量"冷打开"时经帮手的 `goto`。T1 照 00 号文档 4.7.2（`:426`）以新文档的 `performance` 时间为准，零点是 `timeOrigin`，即导航开始；帮手在旧文档上的等待在零点之前，自然不算进去。不从 `goto` 返回（`load` 之后）起算：那样就不含文档的请求、JS 的下载与执行，T1 量小了，没过的也会判"过"。00 号文档写的 element timing 本来就以 `timeOrigin` 为零点（P1 设计审查 A-I2；初稿写成"从 `goto` 返回之后起"，不对）。
  - knip 的 e2e 入口照 00 号文档 4.7.4 加。
- **P5/S1**：发布的闸门跑 `make vuln VULN_AUDIT_LEVEL=high`（`GITHUB_ACTIONS` 在 runner 上是 `true`）；actionlint 一起核对 `vuln.yml`。版本号默认值的守卫放在哪里由 P5/S1 的设计定，本文建议放在 `platform/buildinfo`，与本 Phase 的 Go 版本的守卫相邻（决定 347；00 号文档第 3 节第 8 条没定位置）。
- **每次发布之前跑一次 `make outdated`**（00 号文档 `:197`、`:674`；给负责人的第 6 项的推荐答复 `:1222` 也靠它；P1 设计审查 A-N8）：P5/S2 的发布演练或收尾时，手动触发 `vuln.yml`（`gh workflow run vuln.yml --ref main`），读 `outdated` 一项的输出，六节各有结果；要升的照 S2 的做法另开旁支或进 v0.2 的 `M12-release.md`。
- **P5/S3**：README 的 PostgreSQL 一句挪进 `deploy/README.md` 时，`TestContainerMatchesTheOtherDatabases` 的 README 一行换成 `deploy/README.md`，另加 `deploy/compose.yaml` 的镜像与 initdb 参数（00 号文档 4.3）。README `:16` 的 Go 版本若挪动，Go 的守卫随之。
- **`vuln.yml` 在 M12 期间红了**：照 00 号文档 4.3 与第 7 节 P1 的说明，旁支 `m12-toolchain-<版本>` 照 S2 的做法升：标准库的照 3.9 与 3.13 升 `toolchain` 的各处（守卫逐处核对），模块与 npm 包照 3.13 的步骤。
- **第五份在 EPYC 7763 上的余量**（决定 352）：S2 合并之后，`main` 的第五份第一次落在 EPYC 7763 上时，当时进行中的 Step 的主持者记下它的 Test 一步，与收尾之后的 425 秒比；两次落在 7763 上的平均过 450 秒时，照 06-environment 第 4 节挪份。
- **以后要跨文档后退的故事**：在帮手里加 `goBackAcross`（先 `settle`，再 `page.goBack()`，断言返回的不是 `null`，即真的换了文档），照 A15 加一格对照，门禁照旧（2.1 第 1 条）。
- **收尾**：P1/S1 合并之后 `main` 的 e2e 红过的每一次与总次数（决定 334）；`vuln.yml` 的两件进 v0.1.1 的 `M12-after-release.md`（00 号文档 4.13）；"续期途中刷新会被登出"进已知限制与 v0.2 的 `M12-release.md`（00 号文档 4.4）。

## 10. 结果

每个 Step 合并之前在这里写它的实施、审查与验证的结果；合并的提交见 00 号文档第 12 节。

### 10.1 S1：e2e 的帮手（2026-11-15，合并见 00 号文档第 12 节）

- 提交（从 `main` 的 `179a7aed` 开始，14 个，6.1 的第 1–14 步：设计的五个照次序，实施时另加三个，审查之后修复一轮五个，主持者另加一个只改注释的；Step 的文档提交是主持者的）：
  - A6 第二个页面测试自己开的标签页先受监视（`479d14e5`，3.15 第 1 条）。
  - 帮手 `e2e/fixtures/navigation.ts`：`goto`、`reload` 先等在途的接口请求、续期的锁（没有 Web Locks 时是租约）、这个文档有了给它令牌的答复（第一次续期，或在这个文档里登录、注册），至多 10 秒（页面里的一次读至少 1 秒），之后失败并写出在等什么；`watchPage` 调 `followRequests`；109 个故事与 `wiki-health.ts` 机械地换上（改之前旧写法 260、10，改之后新写法 260、10，反向变换 110/110 相同，diffstat 112 个文件；脚本与输出在 S1 的记录）（`bafbd418`）。审查之后先看锁再看记录、帮手的说明照新代码写（`6ec05d13`，3.15 第 9 条）。
  - 门禁 `tools/e2e-navigation/check.mjs`，进 `make lint-web` 与 knip 的入口，内置样例比整张问题表（`c18562fe`）；另核对读到了帮手的模块（`e3059737`，3.15 第 7 条）；审查之后读 Playwright 默认的八种结尾、逃生口认三种引号、九个样例、文件头照代码写（`91718e48`）。
  - `PageWatch.signInLandings` 与失败时页面状态的 `sent back to sign in:` 一行（`6cf1c32c`）；审查之后那一行只在落到过 `/sign-in` 时写（`3c2425c1`，3.15 第 10 条）；修复核对之后 `pageState` 的说明改为按 ` after a refresh answered 401` 数（`937df215`，只改注释）。
  - A15，`threeBrowsers` 加 `identity\/a15-`，门禁的表写 A15 两处（`ce2cc0fc`）；第 1 格末尾的 `replaceState`（`8e128a2d`）；审查之后加第 8 格（`6ec05d13`）、第 9 格（`995ebf02`），第 1 格末尾的来回（`6e48227d`）。
  - `networkSettled` 等待时打一行（`c330217f`）。
  - 每个提交单独在干净的 worktree 里 `make lint-web`、`make knip` 与碰到的故事都绿（改帮手与 `browser.ts` 的跑全部故事：`bafbd418` 538 个，`6ec05d13` 561 个通过、1 个跳过，`3c2425c1` 564 个通过、1 个跳过）；审查 B 另逐个跑了 `pnpm -r run check:types`，修复一轮的五个由修复核对 B、C 再逐个跑过。
- 与设计不同之处（3.15 列出）：A6 的标签页；令牌的答复认登录与注册（照原写法 A3、S4 在登录页登录之后换文档时等满 10 秒失败）；页面里的读至少 1 秒；第 5 格先断言路由的结局；落点按上一次主框架的地址去重；A15 九格（27 个测试）；门禁核对读到了帮手、读八种结尾、认三种引号；3.7 的"复原之后看不到"在本机做不出（每次都真的等了）；先看锁再看记录；那一行只在落到过 `/sign-in` 时写。
- 反向对照（7.2）：S1 的 12 行（1–8、16–19）、实施者的 7 条（x1–x7）与修复一轮的 9 条都抓到。A15 的变异在三种浏览器各三轮，每条只在该失败的格失败（修复一轮在头 `3c2425c1` 上重跑了整张表）：只去掉锁的一支（m18a）第 6、8 格，先看记录再看锁（m20）第 8 格，去掉租约一支（m21）或把租约的键改名（m22）第 9 格，设计的去重（m23，比上一条落点）第 1 格；m20–m23 在审查之前的 `8e128a2d` 上都活着（审查 B 的 mL、mLK 即 m21、m22）。门禁的新变异：只读四种结尾（g1、g1b）、逃生口只认双引号（g2）、例外按名字 `navigation.ts`（c3），各由内置的核对报出；审查之前的门禁对 `.spec.tsx`、`.spec.cts`、`.spec.jsx` 里的 `page.goto` 与反引号的逃生口都看不见。写代码时活下来的两条各补了：门禁只读 `e2e/stories`（变异 5）补了"读到帮手的模块"；落点不去重（x4）补了第 1 格的 `replaceState`。
- 第 50 条的正向对照（3.7）：让 `tentative()` 交回固定的文字、到时限、`undefined` 各一次，都看到那一行；恒交回空时没有。复原后的代码在本机每次都真的等了 0.2–1.1 秒（容器 veth 的链路本地地址），这一行照实出现。CI 上：分支的 e2e 作业（run 43632495423）在 "Running 559 tests" 之前打了 `global setup: waited 423 ms for the host's tentative IPv6 addresses (… veth … fe80::…/64 scope link tentative …)`，即在 runner 上触发了；审查之后的 run 43642007840 没有打（那台 runner 没有暂定的地址）。合并之后两次 `main` 的 e2e 与这一行，记在 S2 的记录（7.5 的"之前"）。
- 实测（本机，经 `heavy.sh`）：`make e2e` 改之前 538 个、142 秒；帮手之后 538 个、140 秒；`8e128a2d` 559 个、149 秒；头 `3c2425c1` 565 个（564 个通过、WebKit 的 A15 第 3 格跳过）、151 秒（修复核对 B：148 秒）。帮手对已有故事的花费（审查 B 在 `8e128a2d` 上给帮手插记录跑一次全量量到）：帮手被调 485 次，A15 之外 450 次，只真的等过 6 次（N7、PG13、LW3、LW6 ×3，都是在途的读或写，每次 30–55 毫秒），合计 0.23 秒；不用等的读页面每次最多 69 毫秒、合计约 2.7 秒。A15 三轮全绿（78 个通过、3 个跳过），每格（三种浏览器）：第 1 格 0.5–1.2 秒，第 2 格 2.3–2.8，第 3 格 0.3–0.5，第 4 格 3.3–3.8，第 5 格 2.3–2.8，第 6 格 1.3–1.8，第 7 格 0.4–0.8，第 8 格 1.4–1.9，第 9 格 1.4–1.8；九格一轮串行约 45 秒（各格之和）。审查 B 在 `8e128a2d` 上跑七格的 A15 共 220 次（含 8 个 worker 六轮），0 次失败。
- 门禁（本机）：头 `8e128a2d`：`make lint gen-check knip build` 9 秒；经 `heavy.sh`：`make e2e` 149 秒（559 个）；A15 三轮 40 秒。头 `3c2425c1`：`make lint gen-check knip build` 8 秒；`make e2e` 151 秒（565 个）；A15 三轮 50 秒。门禁报出：175 个文件，没有直接的 `goto`、`reload`，逃生口 A15 ×2（表一行，理由写在表里）；今天 `e2e/` 下没有另四种结尾的文件，读的个数不变。
- CI：
  - 审查之前，run 43632495423（`8e128a2d`）：19 项第一次 attempt 全绿；e2e 作业 7:35（E2E 一步 6:03，559 个、1 个跳过，2 个 worker），作业限 20 分钟；S1 之前 `main` 的三次是 7:21（E2E 6:10）、8:16（6:51）与 6:48（5:16），帮手与 A15 多出的时间在机器的差别之内。A15 在 CI 上一轮串行约 41 秒。
  - 修复一轮之后，run 43642007840（`3c2425c1`）：19 项第一次 attempt 全绿，e2e 没有卡在 apt 镜像；e2e 作业 9:58（E2E 一步 8:25，565 个、1 个跳过，2 个 worker），余量 10 分钟。慢在 runner：同一批测试逐个比，A15 之外每个测试的用时中位数是上次的 1.45 倍（修复核对 B 重算 502 个，1.48 倍，四分位 1.29–1.68），不经帮手的纯接口测试约 1.5 倍（标题带 `(API)` 的 152 个，中位数 1.53）。A15 在 CI 上一轮串行约 68 秒，新加的第 8、9 格 6 个测试共约 15 秒。八份组合根的 Test 一步 4:29、4:15、4:49、5:38、6:21、4:33、5:20、4:27（S1 不改服务端）。
  - 主持者的 `937df215`（只改注释）：run 43645603202，19 项第一次 attempt 全绿；e2e 作业 8:48（E2E 一步 7:15，565 个、1 个跳过），第 50 条那一行打出（waited 733 ms）
  - 合并之后两次 `main`（7.5 的"之前"，也记在 S2 的记录与 00 号文档第 13 节）：`bbf3eb26` 的 run 43650412259 与记合并号的 `0da2fb5d` 的 run 43653071437，19 项第一次 attempt 全绿；八份的 Test 一步两次的平均 328、273、416、299.5、259.5、377.5、284、236 秒，都在 450 之下，第三份（两次都在 EPYC 7763 上，422、410）最贴近；第五份两次都没落在 7763 上（6975P、9V45），决定 352 的那一次还没来；e2e 作业 9:38、8:29（E2E 一步 8:08、7:11），第 50 条那一行两次都打出（waited 432 ms、213 ms）。
- **审查**（2026-11-15，三位各在 `8e128a2d` 的自己的工作树里，重的经 `heavy.sh`）：A 审帮手与偏离，B 审测试、变异、浏览器与 CI，C 审替换、门禁与文档稿。A 0I 2M 1N，B 0I 1M 2N，C 0I 1M 7N；去重之后 Important 0，Minor 4：在这个文档里登录、注册时，帮手在记录还没写、锁还被持着时就换文档（P1S1A-M1）；"跟着别的标签页登录的文档会等满 10 秒"不成立（P1S1A-M2）；租约一支没有测试（P1S1B-M1）；文档稿漏了照实改的各节（P1S1C-M1）。Nit 9（三份合计 10，B-N2 = C-N1）。三位都确认偏离对且必要、替换机械、门禁抓得到、自己加的三个提交必要、每个提交单独为绿。
- **修复一轮**（2026-11-15，照主持者的修复说明，从 `8e128a2d` 起 5 个提交，普通推送，没有改写已推的历史；4 个文件，加 185 行、减 50 行，只改 `e2e/fixtures/`、A15 与门禁）：先看锁或租约、再看记录，A15 第 8 格，帮手的说明（A-M1、A-N1、C-N4，`6ec05d13`）；A15 第 9 格（B-M1，`995ebf02`）；门禁的八种结尾、三种引号、两个新样例、文件头（C-N4、C-N5，`91718e48`）；第 1 格末尾的来回（C-N6，`6e48227d`）；那一行只在落到过 `/sign-in` 时写（C-N7，`3c2425c1`）。第 8 格在旧帮手上三种浏览器各三轮 9 个全失败，报 "the next document is asked for once the sign-in wrote its record"；文档一侧的（A-M2、C-M1、C-N1–N3、B-N1、B-N2）在本提交。
- **修复核对**（一轮，2026-11-15，三位各在 `3c2425c1` 的自己的工作树里）：没有 Important、Minor。A 判 M1、M2、N1 关上：审查的探针三种浏览器都绿（新文档在放开之后 36–134 ms 才请求），换回旧帮手三种都失败；第 8 格 9/9，`stories/identity/` 75 个通过、1 个跳过。B 判 M1、N1、N2 关上：m21、m22 只有第 9 格 9/9 失败，A15 九格三种各三轮 78 个通过、3 个跳过，五个提交逐个 `lint-web`、`knip`、`check:types` 为 0，全量 565 个、148 秒。C 判 M1、N1–N7 关上：第一轮的 16 个探针在新门禁上都报出，门禁的 20 条变异照旧，五个提交从 `git archive` 的树上门禁都 rc=0。新的 Nit 三条，都在本提交：A-N2 = C-N8（"有那一行就是被跳回"不对，故事自己去登录页也有；`937df215` 改注释，文档写按 ` after a refresh answered 401` 数），C-N9（决定 342 在本文里另写锚；修复报告说明 13.4 新句的差别时漏了"、自己注册的文档不必等"，说明补上），B-N3（纯接口测试是约 1.5 倍，不是 1.35 倍）。
- 留下的：Chromium 里路由扣着、从没交回的请求留在在途的集合里，同一页面之后再经帮手换文档会等满 10 秒（只有 A15 的两处逃生口，之后不再换文档；`followRequests` 的说明写了）；续期在网络一层失败的文档帮手等满 10 秒（3.2）；WebKit 把编辑页 `pagehide` 里 keepalive 的 `DELETE` 报成 `Load request cancelled`，服务端是否收到没有查（审查 A，产品一侧，交主持者）。从 S1 的合并起数 `main` 的 e2e 因为被跳回登录页而红的次数（决定 334、342）：`gh run list --workflow ci.yml --branch main`，红了的看 E2E 注解里 `sent back to sign in:` 一行，数带 ` after a refresh answered 401` 的失败，再除去有意造重用的故事（A5）。

### 10.2 S2：工具链与安全更新（2026-11-15，合并见 00 号文档第 12 节）

- 提交（从 `main` 的 `0da2fb5d` 开始，18 个：6.2 的五个照次序，`vuln.yml` 报出的四个，`vuln.yml` 的缓存一个，挪份一个，改正注释一个；第一轮修复六个；Step 的文档提交是主持者的）：
  - Go 1.27.4：两份 `go.mod` 的 `toolchain`，Dockerfile 的 server 阶段 `golang:1.27.4-trixie` 的摘要与 `FROM` 之后第一行的断言，`TestTheImageBuildsWithTheModulesToolchain`（`platform/buildinfo`）核对 tag、断言与 README 都等于两份 `toolchain`；README、`edge.go`、`grep.go` 照 1.27.4 的核对写（`jsonv2` 仍在默认打开的实验之后，`parse.go:212` 不变）（`6dc814ea`）。这个提交的说明写 1.27.2–1.27.4 修的 `html/template` 在服务端的路径上，不准：二进制不链接它（`go list -deps ./cmd/nervewiki` 里是 `net/http`、`crypto/tls`、`crypto/x509` 与 `text/template`）；1.27.2 修了 crypto/tls、html/template、net/http、net/textproto、os 与 go 命令，1.27.3 修了 crypto/x509，1.27.4 修了 cgo、go 命令、编译器、链接器与 net/http（审查 A-N3；已推，不改写）。
  - PostgreSQL 18.7 五处，守卫加 README（`69de9e08`）。
  - node（Node 24.22.3，原摘要是 24.21.0）与 distroless 的摘要（`a7987bd0`）；setup-node 7.1.0、upload-artifact 7.0.2（`597b0729`）。
  - `make vuln`、`make outdated`（本机拒绝）、`tools/outdated.sh`、`vuln.yml`（`ed5c1f56`）。
  - `vuln.yml` 第一次运行（43657896140）报出的：GO-2026-6865 `klauspost/compress` 1.20.2（`63de975d`）、GO-2026-6253 `moby/go-archive` 0.3.0（`0f61a57f`），两个只在 `pgtest` 的测试里（3.16 第 8 条；`63de975d` 的说明写"二进制两个都不链接"，二进制其实链接 goose，不链接的是 `klauspost/compress`）；dompurify 3.4.17（`f3e76505`，GHSA-cg2g-xh64-v4q7 critical、GHSA-mrv2-3pqr-j5jr high、GHSA-c6gr-qvv4-5v3f low，mermaid 的范围之内）；katex 的 GHSA-238p-pmpm-9mq7（low）进 `audit.ignore`（`c654db5f`，决定 353）。`vuln.yml` 在 43664040257 为绿。
  - `vuln.yml` 不取也不存 Go 的缓存（`2811d13d`；它的说明说"3 MB 是原因"、"五次都取回 3 MB"，不准，见 3.16 第 2 条与审查 B-M1）；第三、第五份各挪一个给第八份（`28980104`，决定 354，3.16 第 3 条）；这两处注释里比的"S1 之前"改为"S2 之前的 `main`"（`4a0c2585`）。
  - 第一轮修复：竞态的九份才用 Go 的缓存（`0844464d`，决定 355，审查 B-I1；它的说明更正了 `2811d13d`）；守卫读整行的断言（`42e6ff6f`，A-M1、N1、N2）；拒绝与干活同一行、`outdated.sh` 自己拒绝（`e8b115c9`，C-M1）；`outdated.sh` 只对读到的写"没有过时的"、写出出错的命令（`d9ff11f6`，C-M2、N1）；`vuln.yml` 的 `paths` 加 `package.json`（`f480a9e5`，C-N2）；第八份的帮助文字（`80d76191`，B-N2）。
  - 到 `28980104` 的 11 个提交单独在干净的 worktree 里 `make lint knip` 为绿，碰 Go 的 `make test-go-short`，`db.ts` 的用库的故事、锁文件的 web 测试与 L4、`Makefile` 的本机拒绝与份的核对都为绿；`4a0c2585` 只改注释，`make lint` 与份的核对为绿（审查 C-N4）；第一轮修复的 6 个在干净的 worktree 里各自 `make lint` 为绿，改守卫的跑 `make test-go-short` 与守卫，改 `Makefile`、`outdated.sh` 的跑桩的各种情形。
- 版本与摘要在开工时重读（2026-11-15 UTC 00:01–00:03）：go1.27.4 仍是 1.27 线最新；四个镜像的摘要、两个 action 的 SHA（`git ls-remote`）与设计相同。
- 与设计不同之处（3.16 列出）：本机拒绝时 `make` 以 2 退出；Go 的缓存只给竞态的九份、`vuln.yml` 不取不存；挪份不分第九份；变异 10 两格、变异 13 用原摘要；`outdated.sh` 的输出；`pnpm audit` 不必先装；传递依赖的 `pnpm update`；报出的 Go 模块只在测试里；katex 进 `audit.ignore`；守卫读整行的断言；拒绝与干活同一行；`vuln.yml` 的 `paths` 加 `package.json`。
- 反向对照（7.2）：变异 9、10a、10b、11、12、14 由守卫抓到，报错写出文件、找到的与要的（变异 12 的报错在第一轮修复之后才写出 README 写着的版本）；13 由 `make image` 的断言一步抓到（"golang 镜像是 go1.27.1，go.mod 要 go1.27.4：换摘要"）；15 用 PATH 里的桩（不联网）：原样拒绝、桩没被调，去掉判断两个桩都被调到，`GITHUB_ACTIONS=true` 时两侧都跑、合起来失败。实施者另加 x2–x7、p1、p1b，都抓到（p1、p1b 只由实施者的份的核对抓到，仓库里没有核对分份的测试）。第一轮修复：A 的 r1、r2、r3 与 x7b、x11 在修之前活、修后死；`make -i vuln`、`MAKEFLAGS=i make vuln`、`make -i outdated` 与直接执行 `tools/outdated.sh` 修前调到桩、修后一个也不调；15b、15d、15e、split 抓到，15c 由 `outdated.sh` 自己的判断守住、行为上看不出（修复核对 C-N7：这几行与 3.14 的一格由本提交补上）；`outdated.sh` 的 O4、O5 修前写"没有过时的"、修后不写。
- `outdated` 一项六次都六节各有结果、没有一节报错（第一次的输出贴在 S2 的记录）；第 7 个提交之后，两项都不取不存 Go 的缓存。
- 实测：本机第五份在 Go 1.27.1/1.27.4、PostgreSQL 18.6/18.7 上没有差别（222.4–224.7 秒）。分支 CI 上组合根在 7763 上多出的约 50 秒，是竞态的份没取回竞态的编译结果（第一、二次没有缓存，第三到五次取回 3 MB），包的时间没变；第一轮修复之后由第一个跑完的竞态份存键。挪份照本机八份各两轮的顶层测试用时，挪后本机第三、五、八份 227.2/225.2、207.0/208.0、212.5/213.2 秒，折到 7763 暖的时候约 396、382、352 秒；分支 CI 上第三份在 7763 上包 347.6 秒。`vuln.yml` 两项 0:36–0:57；`make image` 48 秒（基础镜像已在本机）、`make image-smoke` 8 秒。
- 门禁（本机）：头 `28980104`：`make lint gen-check knip build` 9 秒、`make test` 3,489 秒、`make test-go-short` 41 秒、`make e2e` 146 秒（564 个通过、1 个跳过），都为绿。头 `80d76191`：`make lint gen-check knip build` 8 秒、`make test-go-short` 40 秒（修复不碰产品代码与测试的分份）。
- CI：
  - 实施的五次（43657895833、43660640426、43664040225、43672581394、43676717979）19 项第一次 attempt 全绿；组合根在 7763 上 417–458 秒（Test 减包 81–88 秒，S2 之前的 `main` 32–35 秒）；e2e 的 E2E 一步 5:28–7:08（S2 之前的 `main` 7:11、8:08）；image 一项 65–77 秒（之前 61、72）。
  - 第一轮修复之后（43681898393，`80d76191`）：19 项第一次 attempt 全绿，冷的（主持者先删了分支上 3 MB 的键）：九个竞态份取缓存、都 "Cache is not found"，`bootstrap-2` 第一个跑完、存下约 269 MB 的键，`short` 与另六份不取不存；7763 上第六、七、八份 431、369、389 秒（Test 减包 87、83、81）。`vuln.yml` 43681898372 为绿。
  - 本提交（暖的第一次）：（待填：本提交的分支 CI 的 run 与 attempt；竞态的各份是否取回 `bootstrap-2` 存的键、约 269 MB；7763 上 Test 减包是否回到约 33 秒）
- **审查**（2026-11-15，三位各在 `4a0c2585` 的自己的工作树里）：A 审工具链、镜像与 Go 模块，B 审分份、缓存与 CI，C 审 `make vuln`、`make outdated`、`vuln.yml`、npm 与文档稿。A 0I 1M 3N，B 1I 1M 2N 1Q，C 0I 2M 6N：Important 1（合并之后 `main` 的缓存键会由不带竞态的 `short` 存下，P1S2B-I1），Minor 4（守卫不核对报错里的版本，A-M1；"取回 3 MB"说多了，B-M1；`make -i` 绕过本机的拒绝，C-M1；`outdated.sh` 一项也没读到时照样写"没有过时的"，C-M2），Nit 11，Q 1（删不删分支上的 3 MB 键：主持者删了）。三位都确认挪份每个顶层测试恰在一份，摘要与 SHA 相同，两个 Go 模块不在二进制里，变异都抓到。
- **修复核对**（一轮，2026-11-15，三位各在 `80d76191` 的自己的工作树里）：没有 Important、Minor。A 判 M1、N1、N2 关上，N3 在文档稿里关上；B 判 I1、M1、N1、N2 关上（`costs-3` 末行不取缓存约多 10 秒，可以接受）；C 判 M1、M2、N1–N6 关上，新 Nit C-N7（`outdated.sh` 自己拒绝的反向对照没进 3.14 与 7.2）在本提交。
- 留下的：合并之后第一次 `main` 是冷的（新的 Go 与 `go.sum`），由第一个跑完的竞态份存键，第二次起竞态的份取回它，7763 上 Test 减包应回到约 33 秒；看那一份 post 一步的 "Cache saved with the key"，并看 `short` 没有存。决定 352 的那一次若是冷的第一次，减去约 50 秒的编译再比。katex 0.18/0.19 与补丁重做、Node 26、功能性的大版本（mermaid 12 等）与 Dependabot 的版本更新，收尾时写进 v0.2 的 `M12-release.md`（决定 353）。开发库：18.x 之间不要 dump/restore，`make dev-db` 拉新镜像、沿用旧的卷即可。

## 附录：核对过的代码

每个决定在 `a22adfd2` 上核对过的地方（读法同头的表）。

| 决定（节） | 核对过的 |
|---|---|
| 续期与锁（1、3.2、3.6） | `W/main.tsx:17-30`；`W/session/session.ts:11-16`、`:38-40`、`:64-80`；`W/session/token-manager.ts:4-24`、`:84-133`、`:221-262`、`:270-310`；`W/session/refresh-lock.ts:14`、`:20-22`、`:25-31`、`:84-105`；`web/apps/web/index.html`（`#root`）；`docs/v0.1/M1-auth/00-M1-design.md:193` |
| 页面的监视（3.3） | `e2e/fixtures/browser.ts:25-55`、`:106-117`、`:123-207`、`:228-242`、`:251-314`；`e2e/fixtures/test.ts:96-108`、`:144-172`、`:208-273` |
| 调用与替换（1、3.4） | `e2e/stories` 的 109 个文件（`grep -rlE '\.(goto|reload)\('`）；`e2e/fixtures/wiki-health.ts:146-160`；`e2e/stories/smoke/s4-routing-fallback.spec.ts:17-33`；`e2e/stories/wiki/lw8-health.spec.ts:701`；`e2e/stories/wiki/lw6-source-history.spec.ts:356`；`e2e/stories/asset/as5-content-safety.spec.ts:173-190`；`e2e/stories/links/l1-links.spec.ts:176-187`；`e2e/stories/identity/a4-refresh.spec.ts:60-91`、`:100-125`；`e2e/stories/identity/a6-sign-out.spec.ts:36-47`；`e2e/fixtures/auth.ts:84`、`:103-122`、`:132`；关标签页的七处 `close()`（第 1 节） |
| 后退与前进（2.1 第 1 条） | `e2e/stories/trash/tb1-trash.spec.ts:268-275`（点链接之后后退）；`e2e/stories/wiki/lw8-health.spec.ts:905-934`（`lw8Document` 标记与 `healthDocumentStayed`，同一文档）、`:936-955`；`e2e/stories/links/l1-links.spec.ts:190-210`（`notReloaded` 标记）；`e2e/stories/transfer/tr2-import.spec.ts:239-260`（`useBlocker`，`W/pages/notebook/import-dialog.tsx:3`、`:64`）；`e2e/stories/search/se1-search.spec.ts:285-295`（SWR 的去重，同一文档）；`e2e/stories/links/l2-create.spec.ts:74-101`（对话框里新建之后后退） |
| 路由与导航（1、3.4） | `as5-content-safety.spec.ts:173`；`c10-tasks.spec.ts:172-189`；`c6-expired.spec.ts:97-111`；`hi3-log-revert.spec.ts:425-461`；`a11-deactivate.spec.ts:129-136`；`a2-sign-up-refused.spec.ts:113-117`；`a4-refresh.spec.ts:73-91`；`a5-refresh-reuse.spec.ts:38-66`；`pg3-move-page.spec.ts:189-190`；`tr2-import.spec.ts:241-266`；`lw13-index.spec.ts:271-275`；`lw1-create.spec.ts:154-155`、`:195-196`；`lw3-source-duplicates.spec.ts:249-258`、`:405-410`；`lw8-health.spec.ts:944-955`、`:1061-1065`、`:1215-1217` |
| A15（3.6） | `e2e/stories/identity/a5-refresh-reuse.spec.ts:1-66`、`:68`；`e2e/fixtures/assert/identity.ts:34`、`:55-80`、`:156-181`；`e2e/fixtures/auth.ts:13`、`:119-122`、`:152`；`e2e/playwright.config.ts:3-4`、`:13`；`m12/review-c/probe.log`、`probe2.spec.ts`、`probe2.log`；`W/services/account.service.ts:12`；`W/pages/settings/settings-layout.tsx:27`、`profile-page.tsx:26`、`security-page.tsx:18`、`:22`；`e2e/stories/identity/a4-refresh.spec.ts:105-109`；`m12/p1-review-a/session-waits-probe.*`、`perf-entry-probe.*`；`m12/p1-review-c/probe-abc.*`、`probe-d.*`、`proto.*`、`proto25.log`；`m12/p1-rev1/a15-proto.*` |
| 门禁（3.5） | `Makefile:159-169`；`knip.jsonc:4-16`；根 `package.json:5-10`；`.oxlintrc.json`；`tools/md-fixtures/check.mjs:1-20`；oxlint 1.86 的规则名（本机 `@oxlint/binding-linux-arm64-gnu` 里有 `no-restricted-properties`）；`.oxfmtrc.json`；`m12/p1-review-a/gate-probe.*`、`evidence.txt`；`m12/p1-rev1/gate-check.*`、`leaving-static-checks.txt` |
| 第 50 条（3.7） | `e2e/global-setup.ts:14-67` |
| Go（3.9） | `server/go.mod:1-5`；`server/tools/go.mod:1-15`；`deploy/Dockerfile:1-57`；`README.md:12-18`；`server/internal/platform/httpserver/mcpserver/edge.go:7`、`:100-123`；`server/internal/modules/page/domain/grep.go:145-153`；`server/internal/modules/page/adapter/postgres/search_guard_test.go:13`；`server/internal/platform/buildinfo/buildinfo.go:1-40`；`.github/workflows/ci.yml:144-160`、`:268-274` |
| PostgreSQL（3.10） | `server/internal/platform/postgres/pgtest/pgtest.go:30-36`；`pgtest/image_test.go:1-41`；`deploy/compose.dev.yaml:8-9`；`e2e/fixtures/db.ts:13`；`deploy/image-smoke.sh:84-87`；`README.md:629`；`v0.1-design.md:722` |
| 镜像与 actions（3.11） | `deploy/Dockerfile:7`、`:23`、`:41`；`.github/workflows/ci.yml:144`、`:148`、`:184`、`:226-239`、`:268-301`、`:320`；`m12/survey-i1/actions-pins.txt`、`actions-new-shas.txt`、`base-images.txt`；四个镜像的摘要（2026-11-15 读） |
| `make vuln` 与 `vuln.yml`（3.12、3.13） | `Makefile:1-37`、`:97-104`、`:139-169`；`pnpm-workspace.yaml:1-36`；`web/apps/web/package.json`、`web/packages/api-client/package.json` 的 `dependencies`；`pnpm audit --help`（pnpm 12.8.1）；`.github/workflows/ci.yml:1-15`、`:220-258`；`docs/v0.1/claude_to_codex/tools/ci-wait.sh`；`server/migrations/river_test.go`（`TestTheRiverMigrationIsRiversOwn`）；`.github/workflows/ci.yml:7-11`（并发）；`pnpm-workspace.yaml:12-16`、`:24-34`；`pnpm update --help`（pnpm 12.8.1，P1 设计审查 B） |
| 00 号文档与总体设计的行（2.1、第 8 节） | `00-M12-design.md:7`、`:12`、`:39`、`:82-89`、`:120`、`:170-197`、`:205-214`、`:418-426`、`:674`、`:680`、`:722-729`、`:933`、`:947`、`:1051-1052`、`:1082`、`:1162`、`:1222`、`:1234`、`:1315`；`v0.1-design.md:722`、`:1179`、`:1265`、`:1286`、`:1325`；`docs/v0.1/claude_to_codex/06-environment.md:10`、`:50-53`；`05-lessons.md:78-81`、`:89`；`03-project-background.md:15`；`README.md:16`、`:484-495`、`:629` |
