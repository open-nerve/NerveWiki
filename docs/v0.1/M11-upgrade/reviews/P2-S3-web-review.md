# M11/P2 S3（网页）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p2-s3`（实施的 4 个提交：`HealthRule` 的读一页、"范围变了"的码与标题级别做成参数（`read`、`moved`、`level`），`HealthItem` 收报告的条目，`HealthScope.notebookId` 删去 `dc8a135a`；检查（`UpgradeService.check`、`checkItems`，`UpgradePage` 以 `key={location.key}` 渲染 `UpgradeRead`，这一次进入的 `nonce`、代数、自己的缓存与一直挂着的 `SWRConfig`，`UpgradeCheckSection`、`UpgradeGates`、`UpgradeCounts`，两种语言的文字，假服务端的检查与条目两条路由）`1c070620`；提交与报告（`UpgradeService.submit`、`NotebookStore.submitUpgrade`，"提交审阅…"与对话框，`change` 的拒绝表加两个码，`UpgradeReportSection`，假服务端的提交照真序分两段）`08f858d8`；e2e 的 UP5 页面版本 `f0ba987d`；从 `main` 的 `edb4faae` 开始，审查时在 `f0ba987d` 上，18 个文件、加 2,477 行、减 52 行，没有碰 `server/`，没有改 `playwright.config.ts`；3 号推送之前 `--amend` 过一次，没有推过的东西被改写。修复一轮 6 个：P2S3A-M1 = C-M1 `41389247`，P2S3A-N1 `904d0d83`，P2S3A-N3、P2S3C-N1–N3 `cfbf80fa`（改了 e2e 的期望），P2S3B-M1、M2、N5、N1 `2c78dc31`，P2S3B-I1、M5 `b9790af0`，P2S3A-M2 = B-M3、B-M4 = C-N4、A-N2、C-N5、C-N6、B-N4 `ff50d516`（8 个文件、加 386 行、减 91 行，没有碰 `server/`；推送之前改写过两个还没推的：1 号只改标题，6 号补 `lint-web` 的 `no-shadow`）；修复第二轮 1 个：P2S3B-N7、A-N5、C-N15 `3717deba`（2 个文件、加 83 行、减 17 行，只改测试）；一共 11 个；合并 `5d3a0f35`），对照 [02-P2](../02-P2-guardrails-gate-submit.md) 的 S3（第 0 节的 S3、2.1 第 13 条、4.3、第 5 节 UP5 的页面版本、6.3、6.4、7.1 的网页与故事两行、7.3 第 67–86 行、7.5 的 e2e、7.6 的 S3、第 8 节标 S3 的句子、第 9 节的 P3/S3）与 3.19、3.20、10.1、10.2，[设计的审查记录](P2-design-review.md)（P2C-I1、M1、M2、N6、N8、N10），[00 号文档](../00-M11-design.md) 4.15、4.7"存与读"、第 5 节的码表、第 9 节与第 11 节（第 259、264 条），[v0.1 总体设计](../../v0.1-design.md) 13.2 第 9、16、18、23 条与 13.4 第 3、7 条，实施者的报告（4 个提交与逐个核对、门禁、37 条变异、实测、L1 的复现与分析、CI、8 条偏离、自己加的、留下的事与文档稿） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（前几个 Step 的三个工作树挪到 `f0ba987d`，`pnpm install --frozen-lockfile --prefer-offline` 过），先读设计与代码、自己形成判断之后才看实施者的报告；三位都只跑单个 vitest 文件，没有经 `heavy.sh` 的运行：网页行为的正确性（A：检查、提交、竞态与 SWR；读 SWR 2.5.1 的源码核实实施者的两条说法；一个临时探针文件 10 格（R1–R8、R6b），R1、R2 失败即 P2S3A-M1、N1；`mut.py` 12 条加对探针的 4 条，另手工两次核实 `SWRConfig` 随一节挂卸时的症状），测试、假服务端与 e2e（B：28 条变异与一个只改一条、逐个打出测试结果的小脚本，重跑实施者的 11 条；一个临时测试文件（`upgrade-check.test.tsx` 的副本，"检查在飞时"三格另断言卡的第一页，外加两格建议的新测试）；在 `1c070620` 上跑四个相关文件；拿真服务端的检查、提交、判定、`upgrade_errors.go`、存储与契约核对假服务端；在实施者的协议日志上重跑他的分析脚本），设计一致、文字、无障碍、给 P3 的复用与文档稿（C：核对 SWR 的源码；`knip` 与一个临时的没人用的导出，证明它会报；一个临时探针复现 P2S3C-M1；12 条变异）。修复核对一轮三位各在自己的工作树、头 `ff50d516` 上，同样只跑单个 vitest 文件：A 逐个读 6 个提交，重跑审查时存活的 7 条与对照 a-swrconfig-per-section、为 P2S3A-M1、N1 新写 7 条，审查时的 10 格探针在头上重跑；B 重跑在头上仍适用的 27 条、C 的 c1 与新写的 n1–n8，"在飞"几格逐格跑；C 重跑探针与 c1、c4、c5、c12、c13，另写 m1、m1b，手工把 c14 放到头上看失败的是哪一句断言，逐段读文档稿（`try.diff`）。修复核对第二轮只有 B（三条都是测试），在自己的工作树、头 `3717deba` 上：三个文件 107 个的基线，重跑 n1、n2、b06 与 A 的 m1-any-code，另写 r1–r3 专查新夹具。 |
| 日期 | 2026-11-11（实施、审查、两轮修复与两轮修复核对） |
| 结论 | Important 1：P2S3B-I1，"检查在飞时"的三格只断言卡的标题，看不见卡的第一页；卡恒为空、照审查 C 当时的前提让卸下的 hook 不建视图，三格都照过，所以 7.3 第 85 行的变异在它们上"没抓到"不能作等价的证据（等价的结论本身对）。Minor 6（去重之后）：卡的"更多"答 403 不重读笔记本列表，页面停在管理员的样子（P2S3A-M1 = P2S3C-M1）；检查本身答 409 `wiki.upgrade_state` 的那条路径没有测试（P2S3A-M2 = P2S3B-M3）；闸门投影里不过的闸门数、`pages_worse`、`stale_pages` 跨行是常量（P2S3B-M1）；报告的每条规则至多一项，"至多五项"只列第一项照过（P2S3B-M2）；闸门的节点在树读着时写"加载中"没有测试（P2S3B-M4 = P2S3C-N4）；"更多"读到的条目在一节卸下又装上之后还在没有测试（P2S3B-M5）。Nit：A 4 条，B 6 条，C 10 条（C-N4 并入 B-M4；A-N2 的"各节次序""对话框的语气"即 C-N5、C-N6）。Q 1：第 74 行的三层防御（P2S3B-Q1）。分开数：A 0I 2M 4N，B 1I 5M 6N 1Q，C 0I 1M 11N。三位都没有查出页面在主路径上的错：问题在"更多"的 403 一条路径、测试看不见的几处与文字。实施者的 8 条偏离三位都判对，其中第 1 条的症状、第 6 条与 74x、74y 的理由要照实改写；"自己加的"第 1 条（`revalidateOnMount`）是对设计的修正（P2S3C-N9）。主持者的修复说明 `fix-1.md` 让代码与测试的十三项都在修复一轮改，C-N7 交给 P3，B-N6 不改，B-Q1 留着、写明测不到，文档的发现由本提交改。修复一轮（6 个提交，头 `ff50d516`）照改了十三项：点名的变异除 a-key-no-upgrade-id 都死，85 在看得见卡的条目的三格上仍照过，这时才是等价的证据；a-key-no-upgrade-id 在修复之后等价（代数在一次进入里单调，键里的 `upgrade.id` 与 `nonce` 一样是防御，主持者接受）。修复核对一轮：A 全关、新 P2S3A-N5；B 关八条、P2S3B-M2 部分关，剩下的一半记为 N7；C 全关、新 P2S3C-N12–N15；都是 Nit。N12–N14 是文档稿的，在本提交；N5、N7、N15 由修复第二轮（`3717deba`，只改测试）改：n1、n2、m1-any-code 改前照过、改后死，b06 照旧死；修复核对第二轮 B 判三条都关上，另加的 r1–r3 都死，新夹具里没有同类的口子。两轮修复、两轮核对之后没有 Important、Minor 留下；Nit 都已改或在本提交，P2S3C-N7 交给 P3（设计第 9 节），P2S3B-N6 不改，P2S3B-Q1 的三层防御保留；等价的变异：85、74x、74y、a-key-no-upgrade-id 与 `readItems` 不判 `scope.current()`（m1-no-current、n6、m1b）。 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：P2S3A-M1、M2、N1–N4，P2S3B-I1、M1–M5、N1–N6、Q1，P2S3C-M1、N1–N11。同一件事合为一行：A-M1 = C-M1；A-M2 = B-M3；B-M4 = C-N4（按 Minor）；A-N2 的"各节次序""对话框的语气"= C-N5、C-N6；A-N4 与 B-N2、N3 说的是实施报告的同两处理由。修复核对一轮的新发现 P2S3A-N5、P2S3B-N7、P2S3C-N12–N15 列在后面。修复一轮照主持者的修复说明（`p2s3/review/fix-1.md`，第 N 条即它"要改的"第 N 条）在 `f0ba987d` 之上加新提交，一类事一个、各自绿；第二轮照 `fix-2.md`。审查时的代码位置是 `f0ba987d` 的，修复核对一轮的是 `ff50d516` 的；`W/` 是 `web/apps/web/src/`） | — |
| P2S3B-I1 | 高（测试） | `W/pages/notebook/upgrade-check.test.tsx:561`（别人的提交）、`:590`、`:668`（自己的提交的两种次序，`test.each`）：结局只断言结果一行与卡的标题在（`cardsOf(back).map((each) => each[1])).toContain("Missing type")`），卡里列的条目不断言。设计 4.3 要的是装回时结果一行、闸门、计数与每张卡的第一页都来自 `data`，7.3 第 85 行指定由这几格抓"视图在 `onSuccess` 里建"。对照变异 b25o（卡恒为空）三格全过；b26（85 另加"卸下的 hook 不建视图"，即审查 C 在修订核对里的前提）三个文件 100 个全过：即使 SWR 照那个前提不调卸下的 hook 的回调，三格照样绿，用户看到的是检查一节回来、"缺少 type"一卡却是空的、没有"更多"。B 在临时文件里给三格加上条目的断言：b26 三格都红，实施者的 85（b24）仍三格都过。所以 85 等价的结论对（SWR 2.5.1、React 19.3.0，`callbackSafeguard` 只比键），作证的"没抓到"是在一组看不见差别的测试上得出的 | 修复一轮改（第 5 条，`b9790af0`）：三格各断言回来之后"缺少 type"一卡的条目（Note 1–5）与"更多"在；b25o、b26 与审查 C 的 c1 要死，85 仍应照过，这时才算等价的证据。7.3 第 85 行换成 c1、3.21 与 10.3 照此写（P2S3C-N9，本提交）。这一类的教训补进 05-lessons 第 1 节（本提交） |
| P2S3A-M1 = P2S3C-M1 | 中（行为） | 卡的 `read` 直接交 `upgrades.checkItems`（`W/pages/notebook/upgrade-check.tsx:159-183`），拒绝落在 `health-rule.tsx:141-146` 的 `onError`，那里只认 `moved`，其余只写进卡。管理员按"检查"之后被另一位管理员降为编辑者（角色的变化不发 `mode`），再按某张卡的"更多"：答 403，卡里写"你没有权限这样做"与"再试一次"，再按还是 403；笔记本列表不重读，"提交审阅…""放弃"都还在。设计 4.3 的"403 时调 `rereadNotebooks`"在说检查与"更多"的那一条里；v0.1 13.2 第 18 条（不经 SWR 的后续页失败时同样交给它）。A 的探针 R1 与 C 的 `probe-more-403.test.tsx` 复现：列表 0 次重读、页面不写"只给管理员"。实施者的移交草稿写了"不另重读"，第 8、9 节没有列为与设计不同之处 | 修复一轮改（第 1 条，`41389247`）：照 A 的写法在 `UpgradeCheckSection` 里包一层卡的 `read`，`forbidden` 且 `scope.current()` 时 `rereadNotebooks()`，再抛原拒绝；不改 `HealthRule` 的接口（C 建议的 `refused` 参数不取），P3 的审阅一节照同样写法包它的 `read`（设计第 9 节，本提交）；测试照 R1 或 C 的探针（列表恰多读一次、页面写只给管理员）。设计 4.3、13.2 第 18 条的句子与移交随之改（本提交） |
| P2S3A-M2 = P2S3B-M3 | 中（测试） | 检查本身答 `wiki.upgrade_state` 的两处没有一格：`upgrade-check.tsx:125`（`rereadUpgrade()`）与 `:138` 的后半（写"升级的状态变了"、不给"再试一次"）。现有的 `wiki.upgrade_state` 一格（`upgrade-check.test.tsx:400`）走的是卡的"更多"；A 的 a-check-409-no-reread、a-check-409-not-moved（B 的 b02、b03）两个文件 73 个全过，那时检查答 409 之后一节写 `NotLoaded` 的通用文字与"再试一次"、不重读升级。设计 4.3 写"检查或'更多'"，4.3 的 vitest 只写了"更多" | 修复一轮改（第 6 条，`ff50d516`）：加 B 写好的那一格（`b/upgrade-check-rb.test.tsx` 末尾第一格）或 A 的 R6、R6b，两个变异要死；设计 4.3 的 vitest 补这一条（本提交） |
| P2S3B-M1 | 中（测试） | `upgrade-check.test.tsx:119-177`、`:182`、`:195`：五行闸门组合里有不过的三行都是 4 道不过，`links` 不过时 `pages_worse` 恒为 3，`index` 不过时 `stale_pages` 恒为 4；行内互不相同，跨行是常量。b01（不过的数写死 4）、b07、b08（两个数写死）照过。05-lessons"投影的测试"那一类：对调的变异都抓到，常量的没核 | 修复一轮改（第 7 条，`2c78dc31`）：各行不过的闸门数不同（例如 2、3、5），`pages_worse`、`stale_pages` 由行给、各行不同，八列仍两两不同、各列有真有假；b01、b07、b08 要死 |
| P2S3B-M2 | 中（测试） | `upgrade-page.test.tsx:879-950` 的 `keptReport` 八条规则的条目都是 0 或 1 项；`upgrade-check.tsx:261` 改成只列第一项（b06）照过，e2e 的报告也不看条目 | 修复一轮改（第 8 条，连同 P2S3B-N5，`2c78dc31`）：一条规则 2–5 项（例如 `undigested_sources` 三项），整组断言；各条规则的数互不相同；b06 要死。修复核对一轮 B 判部分关上：b06 死，但每条至多 3 项（`wanted` 两项，`missing_type`、`undigested_sources` 各三项），截到 3 或 4 照过，剩下的一半记为 P2S3B-N7 |
| P2S3B-M4 = P2S3C-N4 | 中（测试） | `upgrade-check.tsx:370`：树在读时闸门的节点写"加载中"，现有的格子都在树读完之后才检查；b04、c4（恒写"（已不在）"）照过。刚进升级页、树还没回来时按"检查"，在树里的节点会写成"（已不在）" | 修复一轮改（第 9 条，`ff50d516`）：加 B 写好的那一格（持住 `GET nodes`，节点写"Loading…"，放开之后是链接与"(no longer there)"）；b04、c4 要死 |
| P2S3B-M5 | 中（测试） | 卡的"更多"读到的条目在 `[...key, "view"]`（`upgrade-check.tsx:129-130`，设计 4.3），一节卸下又装上还在；b28（视图的键带每次装上的随机数，等于视图随一节挂卸）两个文件 73 个照过 | 修复一轮改（第 10 条，`b9790af0`）：检查、"更多"读到 7 项，`mode` 让一节卸下又装上，仍是 7 项、没有新的检查与条目请求；b28 要死 |
| P2S3A-N1 | 细节（行为） | `upgrade-page.tsx:113` 的代数在 `UpgradeRead` 上、不随升级换：按过"检查"之后升级被放弃、回滚结束，另一位管理员发起新的一次，`mode` 让检查一节装上，键里的 `upgrade.id` 换了、代数仍是 1、缓存里没有，`revalidateOnMount` 为真，不按"检查"就读了新升级的检查（探针 R2：两次）。变异 a-key-no-upgrade-id（键里去掉 `upgrade.id`，新升级会显示上一次的结果）全过 | 修复一轮改（第 2 条，`904d0d83`）：代数随升级记，同一次进入里换了升级，检查一节回到初始、按"检查"才读；R2 一格，a-key-no-upgrade-id 要死；设计 4.3 随之写（本提交）。修复之后 a-key-no-upgrade-id 照过，是等价的：代数在一次进入里单调，新的一次升级按"检查"拿到比用过的都大的一代，两次升级的键在 `generation` 上已不同；用户看得见的错由 `904d0d83` 的那一格钉住，实施者加的 f2（代数不看升级）死。实施者报为新的偏离，主持者接受为等价，三位核对都同意（7.3 照此写，P2S3C-N13） |
| P2S3A-N2（含 P2S3C-N5、N6） | 细节（测试） | 没钉住的四处：离开时中止在飞的检查（a-leave-no-abort）；检查答错之后结果一行不写"检查中"、按钮不 `aria-busy`（a-checking-ignores-error）；各节的次序（a-check-after-rest，C 的 c12、c13）；"提交审阅…"对话框的 `default` 语气（a-submit-destructive，C 的 c5；`ConfirmDialog` 自己的测试钉住了语气到样式的映射，P1 的"继续回滚"同样没在页面上钉住）。都全过 | 修复一轮改（第 11 条，`ff50d516`）：A 的 R3（`signal.aborted`）、R7；状态表的 `upgrading`、`reviewing` 两格断言 `main` 里各节标题与按钮的先后；确认按钮没有 `bg-destructive`；点名的变异要死 |
| P2S3A-N3 | 细节（文字） | `zh-CN.ts:696`、`en.ts:722` 的 `upgrade.agentOthers` 对其他管理员仍只写"你可以放弃这次升级"；00 号文档 4.15 写的是"你可以检查、提交、放弃"，P1 只有放弃，S3 加了前两样，这句没有跟上 | 修复一轮改（第 3 条，`cfbf80fa`）：两种语言改为"……你可以检查、提交审阅，或放弃这次升级。"（en 同义），改 P1 那一格的期望；设计 4.3 记一句（本提交） |
| P2S3A-N4（含 P2S3B-N2、N3） | 细节（文档） | 实施报告的两处理由不准，结论都对。(1) 第 8 节第 1 条说照字面让 `SWRConfig` 随检查一节挂卸时"另发一次、读两次"。A、B 在头上实测（a-swrconfig-per-section、b27）是停在"检查中…"：在飞的请求经旧的全局状态的 setter 写进同一个 `Map`、只通知旧的订阅者，装回的 hook 订的是新状态，收不到；去掉 `revalidateOnMount` 时另发一次，后到的写入与 `Map` 里的相等、不重渲染，仍停在"检查中…"。C 的 c14 是同一种写法（C 在 S3 审查时记作"读两次"，修复核对一轮看断言自己更正，P2S3C-N12）。(2) 第 4 节说 74x、74y 等价是因为"健康度页的缓存是公共的"：健康度页同样每次进入自己一个 `Map`（`health-page.tsx:77`，`HealthEntry` 以 `key={location.key}` 渲染），等价的理由是每次进入一个新的 `Map` | 本提交：3.21、4.3 照实写症状：答复在一节回来之后才到的两格停在"检查中…"，在一节卸着时就到的那一格照过，再去掉 `revalidateOnMount` 时读两次、仍停住；74x、74y 的理由去掉那半句（3.21、7.3 第 74 行） |
| P2S3B-N1 | 细节（测试） | `upgrade-check.tsx:153` 检查一节按 `healthRuleOrder` 排规则，与报告（`:247`）是同一个判断复写两处；报告的夹具次序反过来（own-o1 抓到），检查一节的 `rulesCheck()` 本来就是这个次序，b05 照过。对真服务端两处都等价（它照 `domain.LintRules` 的固定次序答） | 修复一轮改（第 12 条，`2c78dc31`）：`rulesCheck()` 的夹具次序与 `healthRuleOrder` 不同；b05 要死 |
| P2S3B-N4 | 细节（假服务端） | `W/test/upgrade-server.ts:419` 的 `not_ready` 的 `detail` 与真服务端（`wiki/domain/upgrade_errors.go:78-79`）不同；网页不显示它，无害 | 修复一轮改（第 13 条，`ff50d516`）：抄真服务端的 |
| P2S3B-N5 | 细节（测试） | `keptReport` 的规则数有重复（四条 0、两条 1）；标题与数取自同一个对象，写不出照过的变异，只是不合 05-lessons 那条的字面 | 随 P2S3B-M2 改（第 8 条） |
| P2S3B-N6 | 细节（e2e） | UP5 页面版本（`up5-check-submit.spec.ts:243`、`:276`）：结果一行以正则断言、不比答复的 `computed_at`；第一次检查的闸门只看 `frontmatter` 一行；报告只看规则的标题与数、不看条目 | 不改（主持者）：vitest 已钉住投影 |
| P2S3B-Q1 | 问题（给主持者） | 第 74 行：每次进入自己的 `Map` 随以 `location.key` 为键的 `UpgradeRead` 换新，`nonce`、键里的 `locationKey` 与离开时的 `checks.clear()` 三者在行为上都看不出来（74x、74y 等价，B 复核同意）：写明是防御，还是删去其中的？A、C 在"过度设计"里同样提到，都不建议删 | 保留（主持者：设计要求，无害）；设计 4.3 与 7.3 第 74 行写明"防御，测不到"（本提交） |
| P2S3C-N1 | 细节（文字） | zh 把同一个数一处叫"待建页面"（`wanted` 一卡），一处叫"待建链接"（`counts.wanted_before`、`wanted_now`、`gateLinksWorse`，`zh-CN.ts:746`、`:753-754`）；en 两处都是"Wanted pages"；文字照的是设计 4.3 | 修复一轮改（第 4 条，`cfbf80fa`）：统一为"待建页面"；设计 4.3 同改（本提交） |
| P2S3C-N2 | 细节（文字） | `frontmatter` 闸门的文字（`zh-CN.ts:739-740`、`en.ts:770-771`，设计 4.3 的表）读起来把"能解析"也豁免给了来源根与根下的 AGENTS；00 号文档 4.7 闸门 4：`invalid_frontmatter` 收区外每页（含这两页），后两条才豁免它们 | 修复一轮改（第 4 条，`cfbf80fa`）：写清来源区以外的页（含这两页）的 frontmatter 都能解析、除这两页外都带 type 与 summary，en 的"but"改"except"；vitest 与 e2e 的整句同改；设计 4.3 的表同改（本提交） |
| P2S3C-N3 | 细节（文字） | en 的 `gateLinksWorse`"than as the upgrade started"不通；`counts.newPages`"Pages written since …"会读成改过的页；`upgrade.notReady`"已重新读取 / was read again"在对话框出现时常常还没读（409 先到时检查一节还卸着） | 修复一轮改（第 4 条，`cfbf80fa`）：前两处照 C；`notReady` 两种语言改为"会重新读取 / is read again"；设计 4.3 同改（本提交） |
| P2S3C-N7 | 细节（行为） | `upgrade-page.tsx:156` 的 `readingTree = pages.nodes === undefined \|\| tree.isValidating`：树的第一次读失败，`nodes` 留在 `undefined`、`isValidating` 为假，闸门的节点与报告的条目一直写"加载中"，没有错误也没有重试；健康度页有 `NotLoaded` 与重试。左栏的树显示错误、重试之后这里跟上，窄；设计 4.3 只要 `readingTree`，是设计的口子 | 不在 S3 改（主持者）：写进设计第 9 节给 P3，审阅一节同样受影响，那时照健康度页把 `tree.error` 与重试交下（本提交） |
| P2S3C-N8 | 细节（文档） | 第 8 节第 6 条（`paths` 不做事）的理由站不住：健康度页的树同样在公共的 provider 里、同样由 `pages` 事件跟上，照样在节点不在树里时重读一次。差别只是检查的答复先于 agent 那次写的 `pages` 事件到时，新页的条目短暂写"（已不在）"（健康度页这时写"加载中"），事件的重读回来之后成链接 | 本提交：3.21 照 C 改写（升级页不另重读：短暂的"（已不在）"由 `pages` 事件的重读纠正，最终一致）；改法照旧 |
| P2S3C-N9 | 细节（文档） | 文档稿的五处：10.3 与 3.21 漏了 `revalidateOnMount`，它修正设计 4.3 的"已读过的一代……不再读"（只靠 `unchanging` 时，只留了错误的一代在一节卸下又装上时会再读，等于自动重试）；7.3 第 85 行换成 c1（视图建在检查一节自己的 `useState` 里、由 `onSuccess` 写，"检查在飞时"三格抓到），第 74 行写两半合起来；13.2 第 18 条的"检查只在按'检查'时读"与同一句后面的"就再检查"打架，宜"打开页不读检查"，另加"检查与'更多'答 403 时重读"；移交草稿三处（"更多"不另重读、"没保留的节点列成链接"、"闸门 8 照 S2 的文字"）；00 号文档 4.15 说做成参数的是"读一页"与"模式的核对"，实际是 `read`、`moved`、`level` | 本提交：都照改 |
| P2S3C-N10 | 细节（给 P3） | `UpgradeReportSection` 在这一次进入的 `SWRConfig` 之外（`upgrade-page.tsx:269-271`、`:292`）；今天不读，没有问题；P3 的审阅一节加"更多"时要挪进去（或给它自己的一份） | 本提交：写进设计第 9 节 |
| P2S3C-N11 | 细节（给 P3） | 设计第 9 节的"P3/S3"一条低估了要改的：报告的规则由 `UpgradeReportSection` 自己渲染，要换成 `HealthRule`；`HealthCard.items` 与 `paths` 的类型是 `LintItem`，要放宽到 `UpgradeReportItem`；`read` 的 `cursor` 与 `checkItems` 的 `cursor` 必填，"从第一页读"要让它可缺；报告一节挪进 `SWRConfig`（N10）；"更多"的 403（M1）；`health-rule.tsx:210` 现在是 `:178`、`:238` | 本提交：第 9 节照 C 的六条写全，第 5 条写成"'更多'的 403 照 S3 的写法在审阅一节包它的 `read`"（S3 的修复不改 `HealthRule` 的接口），另加 P2S3C-N7 |
| P2S3A-N5 | 细节（测试） | 修复核对一轮的新发现：`readItems` 只在 `forbidden` 时重读列表（`upgrade-check.tsx:163`），没有一格钉住"别的码不重读"：变异 m1-any-code（任何 `ApiError` 都重读列表）在两个文件上照过，"更多"答 409 `wiki.upgrade_state` 一格与检查本身的 409 一格都不数列表的读。后果只是多读一次列表，用户看不见 | 修复第二轮改（`fix-2.md` 第 2 条，`3717deba`）："更多"答 409 那一格接管列表的读并计数，断言没有多；m1-any-code 改前照过、改后死。修复核对第二轮 B 判关上 |
| P2S3B-N7 | 细节（测试） | 修复核对一轮的新发现，P2S3B-M2 剩下的一半：`upgrade-page.test.tsx:900-990` 的 `keptReport` 每条规则至多 3 项，真服务端每条至多存 5 项（`wiki/domain/upgrade_check.go:139`）；n1、n2（报告的列表截到 3、4 项）照过 | 修复第二轮改（`fix-2.md` 第 1 条，`3717deba`）：`keptReport` 照真服务端给条目，每条至多 5 项、数在 5 以内的全列（`wanted` 41+ 与 `undigested_sources` 6 各 5 项），八条的数仍互不相同、整组断言；n1、n2 改前照过、改后死，b06 照旧死。修复核对第二轮 B 判关上，另加 r1（去掉第一项）、r2（反序）、r3（按 `min(count, 4)` 截）都死 |
| P2S3C-N12 | 细节（文档稿） | 修复核对一轮的新发现：c14 不是"读两次"（C 在 S3 审查时没看清失败的是哪一句断言）。它与 a-swrconfig-per-section、b27 是同一种写法；放到 `ff50d516` 上，"检查在飞时"里答复在一节回来之后才到的两格停在"Checking…"，在一节卸着时就答复的那一格照过（装回时 `Map` 里已有 `data`）；只有另去掉 `revalidateOnMount` 时才读两次，页面照样停住。稿子沿用"读两次"六处（3.21 第 1 条、4.3、7.3 表尾、10.3 第 1 条与本记录两处）；`review-c.md` 第 3 节第 1 条与第 7 节 c14 一行同样作废 | 本提交：六处照改，合成一句 |
| P2S3C-N13 | 细节（文档稿） | 修复核对一轮的新发现：7.3 表尾把 a-key-no-upgrade-id 写成由"同一次进入里换了升级"一格抓到；修复之后它照过，主持者已接受为等价 | 本提交：那一行写成等价（键里的 `upgrade.id` 与 `nonce` 一样是防御、测不到），另记抓"代数不看升级"的 f2；4.3 与 3.21 补同一句 |
| P2S3C-N14 | 细节（文档稿） | 修复核对一轮的新发现：4.3 的几处比修复之后的代码旧：仍写 `generation` 的 state 与 `setGeneration`（现在是 `checked = { generation, upgrade }`、`recheck(upgrade)` 与 `setChecked`，`upgrade-page.tsx:115`、`:133-140`、`:257`）；卡的一条仍写直接交 `upgrades.checkItems` 的 `read`（现在是 `read={readItems}`，同一节"范围变了"一条已写了这层包装）；第 9 节的行号是 `f0ba987d` 的，到 `ff50d516` 有的已移位 | 本提交：4.3 照现在的代码写；第 9 节与 3.21 的行号照 `ff50d516`（报告的规则 `upgrade-check.tsx:261-281`，`readingTree` 在 `upgrade-page.tsx:162`，`SWRConfig` 在 `:300`，报告一节在 `:277-279`） |
| P2S3C-N15 | 细节（测试） | 修复核对一轮的新发现：`upgrade-page.test.tsx:164` 的测试名仍是"another admin is told whose agent writes and that they can abandon the upgrade, not the prompt"，期望已改成"检查、提交审阅，或放弃" | 修复第二轮改（`fix-2.md` 第 3 条，`3717deba`）：名字改为"… that they can check, submit or abandon the upgrade, not the prompt"。修复核对第二轮 B 判关上 |
| 偏离 | — | 实施者的 8 条（[02-P2](../02-P2-guardrails-gate-submit.md) 3.21、10.3），都标"实施中定"，没有新编号的决定：1 这一次进入的 `SWRConfig` 一直挂着（SWR 2.5.1 在 `SWRConfig` 卸下时删掉这个 provider 的全局状态，`swr/dist/config-context-*.mjs:512-520`）；2 审查 C 在修订核对里的前提"SWR 不调卸下的 hook 的回调"不对（React 18 起 `callbackSafeguard` 只比键，`swr/dist/use-swr-*.mjs:317-322`），视图照设计从 `data` 推出，理由改为不依赖回调由谁、何时调，7.3 第 85 行在这里等价；3 第 81 行的测法：持住升级的第二次起的读，不只第二次；4 第 75 行的变异写成"检查一节卸着时要的一代，装回时不读"（`revalidateOnMount: false`）；5 假服务端的 503 是名额满的那一种；6 `HealthRule` 的 `paths` 在升级页上不做事；7 `HealthScope.notebookId` 删去；8 "检查在飞时自己提交"两种次序各一格 | 三位都判对。A 逐条判 1–8（读 SWR 的源码核实第 1、2 条；第 1 条的症状照 P2S3A-N4 改写）；B 判第 1 条（必要，b27 两格红，症状见 P2S3B-N2）、第 3、4 条与 74x、74y、85 的结论（85 的证据不够，即 P2S3B-I1）；C 逐条判 1–8（第 1 条以 c14 核实；第 6 条改法可接受、理由不对，即 P2S3C-N8；第 8 条有用：c1 两种次序都抓到；C 当时写 c14 也是，修复核对一轮更正为只抓到答复在一节回来之后才到的那一种，P2S3C-N12）。设计照实改在 3.21 |
| 自己加的 | — | 实施报告第 9 节：1 `revalidateOnMount`：这一代在这一次进入的缓存里已有条目（读过、在读，或被拒）时一节装回不读；没有它，`server_busy` 之后一节卸下又装上会自己再读一次，等于自动重试；2 测试：另一个管理员先提交了（第一段答 409 `wiki.upgrade_state`，页面重读成审阅中、焦点给标题、检查没有读），提交被拒为 `server_busy`、`forbidden`（对话框的文字、检查不读，`forbidden` 时页面写"只给管理员"），状态表 `upgrading`、`reviewing`、`rolling_back` 三格（检查、"提交审阅…"、报告各在不在，只读了一次升级），store 的重读失败时仍抛原来的拒绝、`offline` 不重读；3 报告的夹具每个值互不相同，规则的次序与页面的不同（`toReversed()`），抓"照答复的次序渲染"；4 UP5 页面版本的反向对照：去掉提交被拒时的 `recheck()`、重新 build，故事在等再检查的答复处失败（94 秒），还原之后再 build；5 写代码时的两次抽查（`spot-c2.json`、`spot-c3.json`）：第一次查出"离开、回来"只断言了"回来时不读"，"公共缓存且不带 `nonce`"照过，补成回来之后再按"检查"、断言读了新的一次 | 第 1 条：A 读源码核对初次按"检查"、再检查、装回三种情形都对；C 判它是对设计 4.3"已读过的一代……不再读"的修正，进 3.21 与 10.3 的"与设计不同之处"（P2S3C-N9，本提交）。第 2–5 条：C 在自己的范围里看过都对；B 抽查重跑的实施者的 11 条与实施者的结果相同 |
| 留下的事 | — | 实施报告第 10 节：1 L1 要主持者定；2 决定的编号：说明写"从 276 起"，00 号文档的第 276 条已是分份旁支的，S3 没有要编号的决定；3 Step 的文档提交是主持者的，附文档清单；4 给 P3/S3：`UpgradeReportSection` 自己渲染规则（`h3`、数、至多五项 `HealthItem`），不经 `HealthRule`，设计第 9 节引的 `health-rule.tsx:210` 现在是 `:238`（渲染）与 `:178`（读之前的判断）；5 本机的 `bin/nervewiki` 为 UP5 的反向对照以改坏的网页 build 过一次，之后的每次 `make build` 都重新 build 过，门禁用的是 `f0ba987d` 的 | 1 即第 277 条（"主持者的决定"、"偶发"）；2 照实，S3 的决定从 277 起；3 是本提交（清单由审查 C 补全，P2S3C-N9）；4 写进设计第 9 节，连同 P2S3C-N7、N10、N11；5 照实，UP5 的三轮、L1 的复现、逐个提交与门禁都在还原之后的 build 上 |

## 主持者的决定

负责人可以改判（277 在 S3 的实施之后、审查之前定下）：

277. **e2e 的 L1 照现状**（实施报告第 6 节；S2 的审查记录"偶发"交下，2026-11-11）：`e2e/stories/links/l1-links.spec.ts:178` 的故事不改；CI 再红就只重跑 e2e（`gh run rerun --failed`），记下 run 与 attempt。向 Playwright 上游报问题（对外，可带 `m11/p2s3/l1/` 的两份协议日志）与改测法（例如断言带修饰键的点击不被路由拦下、来源页不动，另开一页 `goto` 链接的地址断言标题与重载之后安静：不再测"浏览器真的开了新标签页"，削弱测试）留给负责人。依据：本机复现约九次一次（`--repeat-each=30` 空载 3 次、满负载 4 次失败）；协议日志显示失败的新标签页在会话建立之中提交了文档、没有 `Page.frameNavigated`，Playwright 只靠这个事件报新页，根在 Chromium，产品照样开了新标签页（文档与它的请求都 200）；两种只改测试的改法各有 5/30 的别的失败，换了样子、没有更少（"偶发"）。实施者列的第四条路（沿第二种改法再查报了新页却看不到标题的原因，估计还要同样多的力气、结局不定）没有选。

修复一轮的处置照 `fix-1.md`：三份审查去重之后，代码与测试的十三项都改（"要改的"第 1–13 条），在 `f0ba987d` 之上加新提交、不改写已推的提交，一类事一个提交、各自绿，范围照实（`web`，改了 e2e 的加 `e2e`）；"更多"的 403 照 A 的写法在升级页包一层 `read`，不改 `HealthRule` 的接口；P2S3B-I1 要让"检查在飞时"的三格看得见卡的第一页，重跑之后 b25o、b26、c1 要死、85 仍照过，这时才算等价的证据；审查者点名的变异都要死；逐个提交照 `percommit-p2s3.sh`，头上跑 `make lint gen-check knip build` 与经 `heavy.sh` 的 `make test-web`、`make e2e`（文字改了，UP5 的页面版本要过）、`make test-go-short`，服务端没改、`make test` 不跑。主持者另定（不编号）：P2S3C-N7 不在 S3 改，交给 P3（设计第 9 节）；P2S3B-N6 不改；P2S3B-Q1 的三层防御保留、写明测不到；文档的发现（P2S3A-N4，P2S3B-N2、N3，P2S3C-N8–N11）由本提交改。

修复核对一轮之后的处置照 `fix-2.md`：a-key-no-upgrade-id 接受为等价（修复一轮的实施者报的新偏离：代数单调，键里的 `upgrade.id` 与 `nonce` 一样是防御、测不到；用户看得见的错由 `904d0d83` 的那一格钉住，f2 死；要让它单独死得让代数每次升级各自从 1 起，不这样做）；`readItems` 不判 `scope.current()` 的那一种接受为等价；P2S3A-N5、P2S3B-N7（P2S3B-M2 剩下的一半）、P2S3C-N15 由修复第二轮改，只改测试、一个提交、范围 `web`；P2S3C-N12–N14 由本提交改。

## 接受与推后的

- **接受**：
  - 等价的变异：实施者的 74x、74y（A、B 复核同意；理由是每次进入一个新的 `Map`、`UpgradeRead` 按 `location.key` 重新装上，健康度页也是这样，P2S3A-N4、B-N3）与 85（A 读源码核实；B 给"检查在飞时"三格加了条目的断言之后 85 仍照过、b26 三格红，修复一轮把这些断言加进测试之后才算证据，P2S3B-I1）。A 的 a-current-no-generation、a-onerror-no-current-guard（再检查中止全部 `controllers`，不当前的请求只以 `AbortError` 结束）、a-recheck-no-alive-guard（离开之后的 `setGeneration` 是空操作）。B 的 b17–b20、b22 对真服务端等价（同一个判定的两面：通过即没有节点、`links` 通过即没有变坏的页、`index` 通过即 `stale_pages` 为 0、`passed` 即没有不过的闸门、报告里有数就有条目）；b05 同样对真服务端等价，修复一轮照样补测试（两处复写的同一判断两处都测，P2S3B-N1）。修复一轮之后：a-key-no-upgrade-id（代数在一次进入里单调，键里的 `upgrade.id` 与 `nonce` 一样是防御，P2S3C-N13）；`readItems` 不判 `scope.current()`（A 的 m1-no-current、B 的 n6、C 的 m1b：在飞的"更多"在再检查与离开时被中止，拒绝是 `AbortError`，到不了；与检查的 `onError` 一致）；85 在看得见卡的条目的三格上仍照过（B 核过）。
  - 过度设计：三位都指出 `nonce`、键里的 `locationKey` 与离开时的 `clear()` 在这个结构里测不到（P2S3B-Q1），保留、写明是防御；C：`entry` 的 `useMemo` 依赖里有一个实例里不会变的 `locationKey`，无害。
  - 未核实的怀疑：A 没有。B：L1 的两点（见"偶发"）；假服务端没有模拟"审阅中、没有报告且过期"的接管与第二段过期的 503，现有测试到不了这两种状态，没有看出影响。C：按钮在"检查中"带 `aria-busy`、仍可按（再按就中止上一代、读下一代），读屏器怎样读没在真实读屏器里核对，不影响行为。
  - 核对过、没有问题的（三位各自的第 4、5 节）：SWR 2.5.1 的两条说法与 `revalidateOnMount` 的三种情形；这一次进入、键、按"检查"才读、再检查中止上一代（A 的 R4：在飞的"更多"也中止）、403 与 `wiki.upgrade_state` 的重读、`server_busy` 不自动重试；提交的第一段照实显示审阅中、两种次序与"提交前已检查过一次"（A 的 R8）、没有事件流时（A 的 R5）都恰再读一次；对话框与焦点；`NotebookStore.submitUpgrade` 的码与重读；状态 × 角色的每一格；闸门、计数与 `preserved` 的投影的对调与大半的常量；假服务端三条路由的码与次序对得上真服务端与契约（13.4 第 7 条）；e2e 的刻意 409、`apiFailures`、"新的一代"由卡回到五项证明；标题级别、`output` 的状态、两种语言的 41 个键、按确切的名字找、13.2 第 16、18、23 条；knip 与导出；文档稿里的数。
  - 流程（照实写下）：3 号在推送之前 `--amend` 过一次（`.reverse()` → `.toReversed()`，`lint-web` 的 `unicorn(no-array-reverse)`：实施者在加那一行之前跑过 lint，之后没再跑，4 号提交之前的 `lint-web` 才看到），4 号是之后才提交的，没有推过的东西被改写。说明写进文件再 `git commit -F`；1–3 号范围 `web`（3 号的说明写了假服务端分两段），4 号 `e2e`（说明写了夹具的改动）；每次提交之前看过 `git status`，没有提交本机的东西；没有推 `main`，没有 force-push。
- **推后**：
  - L1 的上游问题与改测法（第 277 条）：留给负责人，M11 的汇报里列出。
  - P3/S3：设计第 9 节的七条（审阅一节把报告的规则换成 `HealthRule`、条目与 `paths` 的类型、"从第一页读"、报告一节挪进这一次进入的 `SWRConfig`、"更多"的 403 照 S3 的写法包 `read`、行号、树的第一次读失败时的"加载中"；P2S3C-N7、N10、N11）。
  - P2S3B-N6：不改。
  - vitest 的 22 行 `TimeoutNaNWarning`：S2 的门禁日志里同样有，不是本 Step 的（[P1/S4 的审查记录](P1-S4-web-review.md)"留下的事"：不属于 M11）。

## 偶发

- **e2e 的 L1**（`e2e/stories/links/l1-links.spec.ts:178`，page，chromium）：S2 的审查记录"偶发"交给 S3 另查。带修饰键点链接之后 `page.context().waitForEvent("page")` 等满 30 秒；CI 上记到的同一行、同一症状：M10/P3/S1（41188823890）、M10/P4/S1、M11/P1/S3、M11/P2/S2 的修复一轮（42913206700），每次只重跑 e2e 就绿。实施者有界地查（约本 Step 一成多一点的力气，复现与分析三轮，每轮约 3 分钟的机器时间），东西在 `m11/p2s3/l1/`、`l1-fix-unroute/`、`l1-fix-wait/`、`l1tools/`，脚本 `m11/tools/l1-repro-s3.sh`、`l1-fix-s3.sh`：
  - **已有的证据**：M10/P3/S1 的审查记录（`M10-llm-wiki/reviews/P3-S1-lint-protocol-review.md:430-470`）与 42913206700 的 trace 里有第二个 page id，新文档与另外 40 个请求都 200，`page` 事件只有来源页。Playwright 1.63 的源码（`playwright-core/lib/coreBundle.js` 的 `FrameSession._initialize` 与 `_onFrameNavigated`）：新 target 的 `Page.getFrameTree` 答的主 frame 的 URL 是空（初始的空页）时，`_initialize` 等 `_firstNonInitialNavigationCommittedPromise`，它只由一个非初始的 `Page.frameNavigated` 兑现，兑现之后才 `reportAsNew`；客户端的 `waitForEvent("page")` 与 `context.pages()` 都只由这一个事件填，所以"也认 `context.pages()`"不是改法。
  - **复现**（经 `heavy.sh`，头 `f0ba987d`，`make build` 之后，`DEBUG=pw:protocol`，HeadlessChrome 153.0.8010.12，4 个 worker，只跑 L1（page）一格 `--repeat-each=30`；每轮两份协议日志各约 170 MB）：

    | 轮 | 结果 | 用时 |
    |---|---|---|
    | 空载 | 3 失败、27 通过 | Playwright 48.5 秒，整轮 53 秒 |
    | 负载（脚本自己起 8 个空转的 shell 占满 8 个 CPU，按 PID 停） | 4 失败、26 通过 | 1.3 分钟，整轮 82 秒 |

    7 次失败都是原样：第 178 行等新标签页 30 秒超时。负载下没有多多少，本机约九次一次。
  - **协议日志**（`l1tools/opened.py`、`order.py`、`trace.py`）：每轮 60 个 page target，30 个是故事自己开的（有 `Page.navigate`），30 个是带修饰键点开的新标签页。30 个新标签页全部是文档的 `Network.responseReceived` 先于 `Page.enable` 的答复（文档在新 target 的会话还在建立时就答了）。失败的 7 个：`Page.getFrameTree` 答的 URL 是空，之后这个标签页照样提交了文档（`Runtime.executionContextsCleared` 又 `Created`、`DOMContentLoaded`、`load`，网络都 200），但没有 `Page.frameNavigated`（其中 6 个连新 loader 的 `init` 生命周期事件也没有）；通过的 53 个，`frameNavigated` 都在 `Runtime.runIfWaitingForDebugger` 答复之后到。失败在 Chromium（会话建立之中的提交漏了这个事件），Playwright 只靠这一个事件报新页，于是永远等不到；与 CI 的 trace 吻合。
  - **试过的两种只改测试的改法**（都不改 timeout、不加 retry、不 skip，仍断言新标签页的标题与重载之后安静；都不收，故事照原样还原（`git restore`），没有提交，`c5.txt` 是没有用上的提交说明）：
    1. 经 `page.context().route(<目标页的地址>, late)` 把新标签页的文档压后 1 秒再 `route.continue()`，拿到新页之后 `unroute`（`l1-fix-unroute/`）：空载 30/30 通过（60 个 target 的文档都在 `runIfWaitingForDebugger` 答复之后到、都有 `frameNavigated`）；负载下 5/30 失败，"route.continue: Route is already handled!"（处理函数里的错，测试随之失败）。从源码推断（没有逐条核对时序）：`unroute` 时这次拦截还在处理函数手里，服务端就把它放行（`Route.removeHandler` → `continue({ isFallback: true })`），处理函数一秒后醒来再 `continue` 就抛错；也就是开了拦截之后，Playwright 有时在文档放行之前就报了新页。
    2. 同上，改用 `unrouteAll({ behavior: "wait" })`（`l1-fix-wait/`，`l1tools/l1-unrouteAll-wait.diff`）：负载下 30/30 通过；空载 5/30 失败，新标签页报了、`expect.poll` 等它的标题 10 秒不可见（协议日志里每个 target 都有 `frameNavigated`），原因没有再查（到了约定的力气）。
  - **审查 B 的核对**：在实施者的 `l1/idle.protocol.log` 上重跑 `order.py`、`opened.py`（`python3 -I`）：30 个故事自己开的页都有 `frameNavigated`；30 个新标签页 27 个有、3 个没有，都以 `waitingForDebugger=true` 附上、只附上一次（没有换 session 或重新附上），整个日志里没有一条 `frameNavigated` 带它们的 frame id，却有 16–17 个生命周期事件与 `loadEventFired`，其中一个在 `runIfWaitingForDebugger` 答复之前就 `executionContextsCleared` 了；都支持实施者的结论，没有看出推理有错。没核实的两点：复现都开着 `DEBUG=pw:protocol`，日志本身改变时序，不开日志时的比例没有量；这算 Chromium 的错，还是 CDP 在附上与提交赛跑时本来如此、该由 Playwright 兜住，从日志分不出，只影响报给谁（报给 Playwright 是对的入口）。
  - **处置**：第 277 条（"主持者的决定"）。分支 CI（42997780694）的 e2e 一次过，L1 这一次没有出。修复一轮的 CI（43010476398）e2e 一次过，L1 没有出。修复第二轮的 CI（43016566282）e2e 一次过，L1 没有出。

## 反向对照

建成时以 `mut.py` 跑（`NW_SERVER` 指实施者的工作树的 `web/apps/web`；表 `m11/p2s3/mut/final.json` 37 条：7.3 的 S3 19 行 23 条（第 73 行的两种写法；第 74 行设计写的"或"两半各一条，另加两半合起来的一条；第 79 行两处各一条）与实施者自己的 14 条；`m11/tools/mut-p2s3.sh` 经 `heavy.sh`，先在没改的树上把 5 条不同的命令各跑一次，都 rc=0：49、22、51、73、80 个测试；输出 `mut/run.out`、`mut/base-*.log`，头 `f0ba987d`；之后 `git status` 干净，没有 `*.mutbak`）。第 85 行照 6.3 在写 3 号提交时先跑过一次（`spot-c3.json`，那时"检查在飞时自己提交"只有一种次序，没抓到；之后加了另一种次序），整张表在头上再跑。34 条抓到；3 条没抓到，实施者记作等价（"接受与推后的"；85 的证据由修复一轮补，P2S3B-I1）。

**7.3 的 S3 各行与实施者自己的**：

| # | 变异（怎样改坏） | 跑的测试 | 结果 |
|---|---|---|---|
| 67 | 检查的卡交 `wiki.items`（lint 的条目）为 `read` | `upgrade-check.test.tsx` 的"更多" | 抓到 |
| 68 | `HealthRule` 的"范围变了"写死 `notebook.not_llm_wiki` | `health-rule.test.tsx` 的 `moved` 三格；`upgrade-check.test.tsx` 的"更多被拒" | 抓到 |
| 69 | 标题写死 `h2` | `health-rule.test.tsx` 的 `level`；`upgrade-check.test.tsx` 的卡 | 抓到 |
| 70 | 代数从 1 起（打开页就读） | `upgrade-check.test.tsx`"按'检查'才读" | 抓到 |
| 71 | `recheck` 不中止上一代 | 再检查一格 | 抓到 |
| 72 | 去掉 `onErrorRetry: healthRetry`（`server_busy` 照 SWR 的缺省重试） | 503 一格（假计时器推过 300 秒） | 抓到 |
| 73a | 403 不重读列表 | 403 一格 | 抓到 |
| 73b | 403 经这一次进入的 `mutate` 重读 | 403 一格 | 抓到 |
| 74 | 检查在公共的缓存里、键不带 `nonce`（去掉 `SWRConfig`，`nonce` 恒 0） | "离开、回来"两格、"再点同一页的链接" | 抓到 |
| 74x | 只让键不带 `nonce` | 同上 | 没抓到（等价） |
| 74y | 只去掉离开时的 `checks.clear()` | 同上 | 没抓到（等价） |
| 75 | 检查一节卸着时要的一代，装回时不读（`revalidateOnMount: false`） | `upgrade-page.test.tsx` 409 先到的两格 | 抓到 |
| 76 | `change` 的拒绝表去掉 `not_ready`、`stale_index` | 同上 | 抓到 |
| 77 | 提交被拒不 `recheck()` | 同上 | 抓到 |
| 78 | "提交审阅…"不挂 ref（焦点回不到它） | 同上（关上之后的焦点） | 抓到 |
| 79a | 审阅中仍给"提交审阅…" | 状态表的 `reviewing`；409 先到的两格 | 抓到 |
| 79b | 审阅中仍给检查一节 | 同上 | 抓到 |
| 80 | 报告的规则带"更多"按钮 | 审阅中的报告一格 | 抓到 |
| 81 | 提交的答复不进缓存（`change(…, false)`） | 成功一格（第二次起的读都持住） | 抓到 |
| 82 | `submitUpgrade` 不经 `inTurn` | `notebook.store.test.ts` 的"等同一本笔记本的改名" | 抓到 |
| 83 | `behindSubmit` 去掉 `wiki.upgrade_state` | store 的 409 一格 | 抓到 |
| 85 | 视图在 `onSuccess` 里建（卡的第一页只从 `view` 取） | `upgrade-check.test.tsx`"检查在飞时自己提交"两格；连同 `upgrade-page.test.tsx` | 没抓到（等价） |
| 86 | `UpgradePage` 不以 `key={location.key}` 渲染 `UpgradeRead` | "再点同一页的链接" | 抓到 |
| own-revalidate-on-mount-default | 去掉 `revalidateOnMount`（实施者自己加的） | 503 之后一节卸下又装上 | 抓到 |
| own-gate-swap-agents-source | 闸门的次序对调 `agents`、`source_root` | 五种闸门组合的整行断言 | 抓到 |
| own-passed-const | 结果一行的 `passed` 写死为真 | 同上 | 抓到 |
| own-count-swap | 两个 `wanted` 计数对调 | 计数两格（封顶两向） | 抓到 |
| own-preserved-swap | `preserved` 的 `nodes` 与 `nodes_kept` 对调 | 闸门的整行断言 | 抓到 |
| own-o1 | 报告的规则照答复的次序 | 报告一格（夹具次序反过来） | 抓到 |
| own-o2 | 报告的规则标题 `h2` | 同上 | 抓到 |
| own-o3 | `passed` 的报告也写"正在核对" | 报告一格；成功一格 | 抓到 |
| own-o4 | 对话框不给 `not_ready` 自己的文字 | 409 先到的 `not_ready` 一格 | 抓到 |
| own-o5 | 提交被拒为任何码都 `recheck()` | `server_busy` 一格（检查不该读） | 抓到 |
| own-o6 | `behindSubmit` 加 `wiki.upgrade_not_ready` | store 的"不重读"四格 | 抓到 |
| own-o7 | 报告的数不带封顶的"+" | 报告一格 | 抓到 |
| own-o8 | 报告不渲染闸门 | 报告一格 | 抓到 |
| own-o9 | 确认不等提交的答复（对话框立刻关） | 409 先到的两格 | 抓到 |

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，等价的除外；规格与输出在 `p2s3/review/a/`、`b/`、`c/`）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | a-swrconfig-per-section（`SWRConfig` 随检查一节挂卸）、a-submit-no-change-reread-on-success（`change` 成功之后不重读） | 抓到（前者"检查在飞时"的格停在"检查中…"；手工再跑去掉 `revalidateOnMount` 的一版，读两次、同样停住） |
| A | a-check-409-no-reread、a-check-409-not-moved | 存活：P2S3A-M2 |
| A | a-leave-no-abort、a-checking-ignores-error、a-check-after-rest、a-submit-destructive | 存活：P2S3A-N2 |
| A | a-key-no-upgrade-id | 存活：P2S3A-N1 |
| A | a-current-no-generation、a-onerror-no-current-guard、a-recheck-no-alive-guard | 存活，等价 |
| A | 对探针的 4 条（a-leave-no-abort 对 R3、a-check-409-no-reread 对 R6、a-check-409-not-moved 对 R6b、a-checking-ignores-error 对 R7） | 抓到 |
| B | b09–b16、b23（`preserved` 的常量与对调、计数的对调与常量、封顶、闸门一行恒写通过、`agents` 读别的闸门、有问题的规则以"有条目或封顶"判） | 抓到 |
| B | b01、b07、b08 | 存活：P2S3B-M1 |
| B | b02、b03 | 存活：P2S3B-M3（= A-M2）；临时文件的新格抓到 |
| B | b04 | 存活：P2S3B-M4；临时文件的新格抓到 |
| B | b06 | 存活：P2S3B-M2 |
| B | b28 | 存活：P2S3B-M5 |
| B | b05、b17–b20、b22 | 存活，对真服务端等价（b05 即 P2S3B-N1） |
| B | b24（实施者的 85） | 临时文件的"检查在飞时"三格（加了条目的断言）照过：等价 |
| B | b25o（卡恒为空，对照）、b25（同上，对临时文件） | 原三格全过；临时文件三格红：P2S3B-I1 |
| B | b26（85 另加"卸下的 hook 不建视图"） | 原三个文件 100 个全过；临时文件三格红：P2S3B-I1 |
| B | b27（`SWRConfig` 随检查一节挂卸） | 两格红，停在"Checking…"（P2S3B-N2） |
| B | 实施者的 11 条重跑（67、74x、74y、75、76、77、81、85、own-revalidate-on-mount-default、own-preserved-swap、own-o1） | 与实施者的结果相同 |
| C | c1（视图建在检查一节自己的 `useState` 里、由 `onSuccess` 写，卸下即丢） | "检查在飞时"三格抓到（7.3 第 85 行换成它） |
| C | c6–c11（检查的结果不是 `output`、`level` 缺省为 3、报告的规则 `h4`、报告不列条目、报告的那句不带时刻、结果不带检查的时刻）、c14（只在有检查一节时挂 `SWRConfig`，与 a-swrconfig-per-section、b27 同一种写法） | 抓到（c14：S3 审查时记作"检查在飞时"的格读两次；修复核对一轮在 `ff50d516` 上看断言，是答复在一节回来之后才到的两格停在"Checking…"，另一格照过，P2S3C-N12） |
| C | c4、c5、c12、c13 | 存活：P2S3C-N4（= B-M4）、N6、N5 |
| C | 探针 `probe-more-403.test.tsx` | 失败：P2S3C-M1 |

A 12 条（2 抓到、7 存活成发现、3 等价）加对探针的 4 条（都抓到）；B 28 条（表在 B 的报告第 6 节），另重跑实施者的 11 条；C 12 条（8 抓到、4 存活成发现）。各自先在没改的树上跑过同样的命令：A 两个文件 73 个、B 三个文件 100 个、C 四个文件 114 个，都过。

**修复一轮**（实施者的 `fix1/mut/`：规格取自三位审查者的目录，加实施者为第 1、2 条写的 f1、f2；`m11/tools/mut-fix1.sh` 经 `heavy.sh`，先在没改的树上把每条不同的命令各跑一次，都 rc=0；命令"三文件"= `upgrade-check.test.tsx`、`upgrade-page.test.tsx`、`health-rule.test.tsx`，改前 100 个、改后 107 个测试；"在飞"= `upgrade-check.test.tsx -t 'under way'`；从备份复原，之后 `git status` 干净、没有 `*.mutbak`）：

| 变异 | 条目 | 命令 | 改前（`f0ba987d`） | 改后（`ff50d516`） |
|---|---|---|---|---|
| f1-more-403-no-reread（`readItems` 不重读列表） | 1 | 三文件 | —（改前没有这段代码） | 抓到 |
| f2-generation-of-any-upgrade（代数不看升级） | 2 | 三文件 | — | 抓到 |
| a-key-no-upgrade-id | 2 | 三文件 | 照过 | 照过（等价，"主持者的决定"） |
| a-check-409-no-reread（= b02）、a-check-409-not-moved（= b03） | 6 | 三文件 | 照过 | 抓到 |
| b01-failed-count-const4、b07-links-worse-const3、b08-stale-const4 | 7 | 三文件 | 照过 | 抓到 |
| b06-report-first-item-only | 8 | 三文件 | 照过 | 抓到 |
| b04-gate-node-loading-ignored（= c4） | 9 | 三文件 | 照过 | 抓到 |
| b28-view-per-mount | 10 | 三文件 | 照过 | 抓到 |
| a-leave-no-abort、a-checking-ignores-error | 11 | 三文件 | 照过 | 抓到 |
| a-check-after-rest（= c12）、c13-report-after-abandon | 11 | 三文件 | 照过 | 抓到 |
| a-submit-destructive、c5-submit-dialog-destructive | 11 | 三文件 | 照过 | 抓到 |
| b05-check-rules-as-answered | 12 | 三文件 | 照过 | 抓到 |
| b25o（卡恒为空） | 5 | 三文件 | 抓到（只有"更多"一格红） | 抓到 |
| b25o | 5 | 在飞 | 照过（B 在 `f0ba987d` 上跑的） | 抓到：在飞的三格都红 |
| b26（卸下的 hook 不建视图） | 5 | 三文件；在飞 | 照过；— | 抓到；在飞的三格都红 |
| c1（视图在检查一节自己的 `useState` 里） | 5 | 三文件、在飞 | 抓到 | 抓到 |
| 85（实施者的，视图在 `onSuccess` 里建） | 5 | 三文件、在飞 | 照过 | 照过（等价；这时三格看得见卡的条目） |

**修复核对一轮的**（各在自己的工作树、头 `ff50d516` 上；"改前"取审查时在 `f0ba987d` 上的结果）：

| 审查者 | 变异 | 改前 | 改后 |
|---|---|---|---|
| A | a-leave-no-abort、a-check-409-no-reread、a-check-409-not-moved、a-checking-ignores-error、a-check-after-rest、a-submit-destructive | 照过 | 抓到 |
| A | a-swrconfig-per-section（对照） | 抓到 | 抓到 |
| A | a-key-no-upgrade-id | 照过 | 照过（等价） |
| A | m1-no-reread、m1-not-wrapped（= `f0ba987d` 的直接交 `checkItems`）、n1-gen-ignores-upgrade（= `f0ba987d`）、n1-submit-recheck-other、n1-submit-no-recheck | —（m1-not-wrapped、n1-gen-ignores-upgrade 即 `f0ba987d` 的样子，那时探针 R1、R2 失败） | 抓到 |
| A | m1-any-code（任何 `ApiError` 都重读列表） | — | 照过：P2S3A-N5 |
| A | m1-no-current（不判 `scope.current()`） | — | 照过，等价 |
| B | b01–b08、b28 | 照过 | 抓到 |
| B | b09–b16、b23、b27 | 抓到 | 抓到 |
| B | b17–b20、b22 | 照过 | 照过（对真服务端等价） |
| B | b24（实施者的 85） | 照过（测试看不见条目） | 照过（"在飞"五格全过，看得见条目：等价） |
| B | b25o、c1 | "在飞"三格全过；— | "在飞"三格与"更多"的条目卸下又装上一格都红 |
| B | b26 | 三个文件全过 | "在飞"三格红 |
| B | n1、n2（报告的条目截到 3、4） | — | 照过：P2S3B-N7 |
| B | n3–n5、n7、n8（不过的数不算 `index`、stale 取 worse、`links` 的节点取 `reserved` 的、`preserved` 的两个"保留"数写成常量） | — | 抓到 |
| B | n6（`readItems` 不判 `scope.current()`） | — | 照过，等价 |
| C | c1 | 抓到 | 抓到（"在飞"三格） |
| C | c4、c5、c12、c13 | 照过 | 抓到 |
| C | m1（`readItems` 不重读列表） | —（探针在改前失败） | 抓到 |
| C | m1b（重读不看 `scope.current()`） | — | 照过，等价 |
| C | c14（手工，看断言） | 抓到 | 抓到：两格停在"Checking…"，不是"读两次"（P2S3C-N12） |
| C | 探针 `probe2.tsx`（"更多"答 403） | 失败 | 通过 |

A 15 条（基线两个文件 80 个），B 36 条（基线三个文件 107 个），C 7 条加 c14 与探针（基线五个文件 201 个），基线都过；C 第一次跑 c12、c13 时脚本把命令误配成"在飞"一组（`startswith("c1")` 匹配到了），照过，改成两个文件重跑，都抓到。

**修复第二轮**（实施者的 `fix2/mut/`：规格取自 B 的 `check1/spec-check1.json`（n1、n2、b06）与 A 的 `check1/mut-check1.json`（m1-any-code），命令同修复一轮的三文件；`m11/tools/mut-fix1.sh` 经 `heavy.sh`，先在没改的树上跑一次命令，107 个都过；从备份复原，之后 `git status` 干净、没有 `*.mutbak`）：

| 变异 | 改前（`ff50d516`） | 改后（`3717deba`） | 改后抓到它的测试 |
|---|---|---|---|
| n1（报告的条目截到 3 项） | 照过 | 抓到 | 审阅中的报告一格 |
| n2（报告的条目截到 4 项） | 照过 | 抓到 | 同上 |
| b06（报告的条目只列第一项） | 抓到 | 抓到 | 同上 |
| m1-any-code（任何 `ApiError` 都重读列表） | 照过 | 抓到 | "更多"答 409 一格 |

**修复核对第二轮的**（B，头 `3717deba`，三个文件 107 个的基线都过；规格与结果在 `p2s3/review/b/check2/`）：上面四条结果相同；另加 r1（报告的条目去掉第一项）、r2（反序）、r3（按 `min(count, 4)` 截）都死。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `ff50d516`（修复一轮 6 个提交），三位各在自己的工作树里 | A：P2S3A-M1、M2、N1–N3 关，审查时的 10 格探针在头上全过（R1、R2 由失败变为通过）；同意 a-key-no-upgrade-id 等价；P3 的审阅一节照抄 `readItems` 够，要带上三样（设计第 9 节第 5 条）；新的 P2S3A-N5。B：P2S3B-I1、M1、M3–M5、N1、N4、N5 关，M2 部分关；新的 P2S3B-N7（M2 剩下的一半）。C：P2S3C-M1、N1–N6 关，文档稿里 N7–N11 与 A-N4、B-N2、N3 落了，c14 那一处不对；新的 P2S3C-N12–N15 | P2S3A-N5、P2S3B-N7、P2S3C-N15 由修复第二轮改；P2S3C-N12–N14 在本提交；a-key-no-upgrade-id 与 `readItems` 不判 `scope.current()` 的接受为等价 |
| 2 | `3717deba`（修复第二轮 1 个提交），B | P2S3B-N7、P2S3A-N5、P2S3C-N15 关：n1、n2、m1-any-code 改前活、改后死，b06 照旧死，另加的 r1–r3 都死；新夹具里没有同类的口子（`ambiguous` 的数封顶却只有 2 项、`AGENTS.md` 在 `undigested_sources` 里标成附件，不像真服务端，但不影响投影的测试，不算发现）；没有新发现 | 没有 Important、Minor 留下，核对到此为止 |

## 实测

- **vitest**：`edb4faae` 上 192 个文件、4,345 个测试（S2 的记录）；头 `f0ba987d` 上 193 个文件（新 `upgrade-check.test.tsx`）、4,437 个测试，多 92 个（逐个提交 4,352、4,404、4,437、4,437：`health-rule.test.tsx` 7 个，`upgrade-check.test.tsx` 22 个，其余在 `upgrade-page.test.tsx`、`notebook.store.test.ts` 与按文字的键逐个核对的 i18n 测试里）；22 行 `TimeoutNaNWarning` 在 S2 的门禁日志里同样有。`make test-web` 59–61 秒（逐个提交的四次），`make test` 里的 vitest 60.5 秒。改到的文件单独跑（变异的基线）：`upgrade-page.test.tsx` 51 个约 5 秒，`upgrade-check.test.tsx` 22 个约 3 秒，`notebook.store.test.ts` 80 个，`health-rule.test.tsx` 与 `upgrade-check.test.tsx` 合起来 49 个。
- **e2e**：`make e2e` 523 个（S2 末 522 个，多的是 UP5 的页面版本；设计 7.5 估的"S3 末 523 个"），Playwright 2.4 分钟，整项 144 秒。
- **UP5 的页面版本单独三轮**（`m11/tools/up5page-s3.sh 3`，`make build` 之后，Chromium，头 `f0ba987d`，`up5-page-{1,2,3}.log`）：都通过；测试本身 1.0、1.0、0.99 秒，Playwright 3.9、3.5、3.9 秒，每轮 4 秒。
- **L1 的复现**：见"偶发"。
- 审查者都没有跑重的：B 在 `1c070620` 上跑四个相关文件 100 个通过；C 的 `knip --treat-config-hints-as-errors` rc=0。
- **修复一轮**（头 `ff50d516`）：vitest 193 个文件、4,444 个测试（逐个提交 4,438、4,439、4,439、4,440、4,441、4,444），`make test-web` 60–61 秒；`make e2e` 523 个，Playwright 2.3 分钟，整项 140 秒；3 号的 UP5 三个故事 4.2 秒。修复核对一轮三位只跑单个 vitest 文件：A 两个文件 80 个，B 三个文件 107 个，C 五个文件 201 个，都过；C 的 `knip` rc=0。
- **修复第二轮**（`3717deba`，只改既有的格）：vitest 193 个文件、4,444 个测试，`make test-web` 58 秒；改到的两个文件 80 个测试。修复核对第二轮 B 三个文件 107 个，都过。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**（`m11/tools/percommit-p2s3.sh`，整个经 `heavy.sh`：在一个干净的临时 worktree（从 `edb4faae` 开出，`pnpm install --frozen-lockfile --prefer-offline`）里逐个 `git checkout --detach`、`git clean -fdx`（留 `node_modules`、`bin`），每个跑 `make build`、`make lint-web`、`make knip`、`make test-web`、`make gen-check`；改了故事的另在 Chromium 里跑它改的故事；没有一个改了 `server/`，所以不跑 `lint-go`、`test-go-short`；日志 `percommit/`，汇总 `percommit.out`，跑完删掉临时 worktree）：

  | 提交 | build | lint-web | knip | test-web | gen-check | 故事 | 之后 `git status` |
  |---|---|---|---|---|---|---|---|
  | `dc8a135a` | 0 | 0 | 0 | 0（192 个文件、4,352 个测试，61 秒） | 0 | — | 干净 |
  | `1c070620` | 0 | 0 | 0 | 0（193、4,404，61 秒） | 0 | — | 干净 |
  | `08f858d8` | 0 | 0 | 0 | 0（193、4,437，59 秒） | 0 | — | 干净 |
  | `f0ba987d` | 0 | 0 | 0 | 0（193、4,437，60 秒） | 0 | UP5 三个（API、MCP、页面）通过，5 秒 | 干净 |

  （数字是 rc。）写代码时每个提交之前另在工作树里跑过 `tsc`、`oxfmt`、`make lint-web`、`make knip` 与改到的测试文件；3 号的 `lint-web` 是在加 `.reverse()` 之前跑的（"接受与推后的"的流程）。审查 B 读了逐个提交的脚本与日志，与报告一致。
- **建成时的头**（`f0ba987d`，`m11/tools/gates-p2s3.sh`：轻的一项直接跑，重的三项各自经 `heavy.sh`、一次一个；每一项之前判 df，整轮 28–29G 可用，没有修剪；开始与结束时工作树都干净；日志 `gates/`，汇总 `gates.out`），一轮全绿：

  | 门禁 | 结果 | 用时 |
  |---|---|---|
  | `make lint gen-check knip build` | rc=0 | 18 秒 |
  | `heavy.sh make test` | rc=0（竞态、组合根八份、成本三份、lint 的成本、守卫两份，vitest 193 个文件、4,437 个测试 60.5 秒） | 3,617 秒 |
  | `heavy.sh make test-go-short` | rc=0 | 43 秒 |
  | `heavy.sh make e2e` | rc=0，523 个 | 144 秒 |

  S3 没有改服务端，`make test` 的 Go 部分是照跑；它比 S2 第二轮的 3,501 秒多 116 秒，没有查（机器上同时没有别的重的运行，`heavy.sh` 的锁一次一个）。
- **修复一轮**（逐个提交照上面的脚本，`FROM=f0ba987d`，整个经 `heavy.sh`，日志 `fix1/percommit/`，汇总 `fix1/percommit.out`；推送之前 1 号改过标题，6 号补了 `lint-web` 的 `no-shadow`：实施者把 `lint-web` 与提交接在一条命令里、没有先判 rc，提交之后才看到，`--amend` 之后 rc=0，日志里的 sha 是最终的）：

  | 提交 | build | lint-web | knip | test-web | gen-check | 故事 | 之后 `git status` |
  |---|---|---|---|---|---|---|---|
  | `41389247` | 0 | 0 | 0 | 0（193、4,438，61 秒） | 0 | — | 干净 |
  | `904d0d83` | 0 | 0 | 0 | 0（193、4,439，61 秒） | 0 | — | 干净 |
  | `cfbf80fa` | 0 | 0 | 0 | 0（193、4,439，61 秒） | 0 | UP5 三个通过，5 秒 | 干净 |
  | `2c78dc31` | 0 | 0 | 0 | 0（193、4,440，61 秒） | 0 | — | 干净 |
  | `b9790af0` | 0 | 0 | 0 | 0（193、4,441，60 秒） | 0 | — | 干净 |
  | `ff50d516` | 0 | 0 | 0 | 0（193、4,444，61 秒） | 0 | — | 干净 |

  头 `ff50d516`（`m11/tools/gates-p2s3-items.sh`，`ITEMS="test-web e2e test-go-short"`，轻的一项直接跑，重的各自经 `heavy.sh`、一次一个，每项之前判 df，28G；前后工作树干净；日志 `fix1/gates/`）：

  | 门禁 | 结果 | 用时 |
  |---|---|---|
  | `make lint gen-check knip build` | rc=0 | 9 秒 |
  | `heavy.sh make test-web` | rc=0（193 个文件、4,444 个测试） | 60 秒 |
  | `heavy.sh make e2e` | rc=0，523 个（Playwright 2.3 分钟） | 140 秒 |
  | `heavy.sh make test-go-short` | 第一次 rc=2：Go 的链接器在 `/tmp` 写测试二进制，"disk quota exceeded"（见"环境"）；以 `GOTMPDIR` 指向磁盘上的临时目录再跑 rc=0 | 29 秒（失败的那次）、47 秒 |

  服务端没改，照 `fix-1.md` 不跑 `make test`。
- **修复第二轮**（照 `fix-2.md` 的核对：逐个提交照上面的脚本，`FROM=ff50d516`，经 `heavy.sh`，日志 `fix2/percommit/`）：`3717deba` 的 build、`lint-web`、knip、`test-web`（193、4,444，58 秒）、gen-check 都 rc=0，没有服务端文件与故事，之后干净；提交之前在工作树里跑过 `tsc`、`oxfmt`、`make lint-web`、`make knip` 与两个改到的测试文件，判过 rc。没有改产品代码与 e2e，头上不另跑 `make e2e`、`make test-go-short`。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 作业 | 42997780694（建成，`f0ba987d`） | 43010476398（修复一轮，`ff50d516`） | 43016566282（修复第二轮，`3717deba`） |
|---|---|---|---|
| server (race) | 11:14（10:49） | 11:08（10:47） | 11:23（11:01） |
| server (bootstrap-1) | 6:45（6:24） | 6:50（6:28） | 6:57（6:32） |
| server (bootstrap-2) | 6:35（6:16） | 6:32（6:10） | 6:30（6:06） |
| server (bootstrap-3) | 7:06（6:44） | 6:45（6:25） | 4:54（4:36） |
| server (bootstrap-4) | 6:35（6:13） | 7:34（7:13） | 7:49（7:26） |
| server (bootstrap-5) | 4:26（4:06） | 4:25（4:07） | 7:21（7:01） |
| server (bootstrap-6) | 5:10（4:49） | 5:46（5:24） | 3:55（3:36） |
| server (bootstrap-7) | 5:39（5:14） | 4:04（3:45） | 5:14（4:55） |
| server (bootstrap-8) | 3:46（3:27） | 4:23（3:59） | 3:43（3:25） |
| server (costs-1) | 17:15（13:40） | 15:31（12:13） | 17:07（13:40） |
| server (costs-2) | 20:56（20:35） | 18:00（17:35） | 24:43（24:19） |
| server (costs-3) | 13:44（13:25） | 12:25（12:05） | 17:41（17:21） |
| server (costs-lint) | 10:40（10:20） | 12:22（12:02） | 12:58（12:33） |
| server (guards-core) | 19:43（19:23） | 17:54（17:35） | 19:31（19:10） |
| server (guards-sources) | 13:34（13:11） | 16:46（16:26） | 16:28（16:09） |
| server (short) | 3:40（3:14） | 3:01（2:43） | 4:16（3:57） |
| web | 4:41（4:05） | 5:22（4:45） | 4:02（3:29） |
| e2e | 8:42（7:09） | 8:48（7:18） | 9:03（7:28） |
| image | 1:25 | 1:30 | 1:28 |

- 建成时一次全绿，19 项，普通推送（新分支，`f0ba987d235f77756447ae1cee8ed5d785728855`，rc=0），`docs/v0.1/claude_to_codex/tools/ci-wait.sh` 等到第 1 次 attempt（09:45–10:14 UTC，输出 `ci.out`）。八份的 Test 一步 384、376、404、373、246、289、314、207 秒，都在 450 秒之下（最长第三份 404 秒）；没有记各作业落在哪种 CPU 上。e2e 一次过，L1 这一次没有出。
- 修复一轮：普通推送（`f0ba987d..ff50d516`，rc=0），`ci-wait.sh ff50d516e69e054fe0411c14f23c98e04ce262ad`：run 43010476398 的第 1 次 attempt 19 项一次全绿（输出 `fix1/ci.out`）。八份的 Test 一步 388、370、385、433、247、324、225、239 秒，都在 450 秒之下（最长第四份 433 秒）；e2e 一次过，L1 没有出。
- 修复第二轮：普通推送（`ff50d516..3717deba`，rc=0），`ci-wait.sh 3717deba80b2d2c337c132ded6cdb894f66bbe15`：run 43016566282 的第 1 次 attempt 19 项一次全绿（输出 `fix2/ci.out`）。八份的 Test 一步 392、366、276、446、421、216、295、205 秒；第四份 446 秒贴着 450 秒的线，S3 没有动服务端，合并之后看两次 `main`。e2e 一次过，L1 没有出。

S3 开工之前 `main` 的几次 CI（S2 合并之后的两个旁支，详见 [P2 设计](../02-P2-guardrails-gate-submit.md) 10.2 的"旁支"）：记合并号的 `1b0eb605`（run 42962539666）第一次 attempt 第六份红在 `TestIngestInvariantCleanupWindow/live_trimmed_empty`，只重跑那一项绿，由旁支 `m11-p2-flake`（合并 `edb4faae`）修；第 276 条的旁支 `m11-p2-shards`（合并 `3959baf9`）把第三、第五份各一个测试挪到第八份；挪份之后两次 `main`（42972302721、42976488607）都一次全绿，八份平均 329.5、350、397、358.5、395.5、304.5、255.5、243 秒，都在 450 秒之下。S3 从 `edb4faae` 开出。

## 环境

实施者的门禁每一项之前判 df，整轮 28–29G 可用，没有修剪；L1 的协议日志每份约 170 MB，在 scratchpad 里，不在仓库里。三位审查者都只跑单个 vitest 文件（B 看过 df，28G 可用）；临时测试与探针都已挪出工作树（B 的 `upgrade-check-rb.test.tsx` 存档在 `b/`，A 的探针在 `a/`，C 的在 `c/`），三个工作树最后都回到 `f0ba987d`、`git status` 干净、没有 `*.mutbak`。修复一轮：每项门禁之前判 df（28G）；`make test-go-short` 第一次撞上 `/tmp` 的配额：`/tmp` 是带 `usrquota` 的 tmpfs，这个用户在上面用了 12G，多是各 Step 的 scratchpad 与两天前留下的 `go-build*`。实施者把自己的大日志（L1 的六份协议日志、S3 门禁的 `test.log`）以 `gzip -1` 压缩，`/tmp` 从 12G 降到 11G，另以 `GOTMPDIR` 指向磁盘上的一个临时目录（跑完删掉）再跑；别人的 `go-build*` 没有动；上面"偶发"引的协议日志因此是 `l1/*.protocol.log.gz` 等。修复核对一轮的三个工作树最后都在 `ff50d516`、`git status` 干净、没有 `*.mutbak`。修复第二轮：每次重的之前 `df -h /` 28G；`tsc -b` 留下的 `web/apps/web/tsconfig.tsbuildinfo` 没进 git，已删；实施者的工作树在 `3717deba`、干净、与 `origin/m11-p2-s3` 一致；B 的工作树在 `3717deba`、干净、没有 `*.mutbak`。

合并之后 `main` 的第一次 CI（run 43027580603，`5d3a0f35`）19 项一次全绿，数见 [P2 设计](../02-P2-guardrails-gate-submit.md) 10.3 的"合并之后"；第四份在 EPYC 7763 上 445 秒，最贴近线，交给 P3 的第一个 Step。
