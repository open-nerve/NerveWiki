# M11/P3 S3（网页）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p3-s3`（实施的 3 个提交：报告经 `HealthRule`（`HealthRuleItem`、游标可为 `undefined`、可缺的 `treeFailed`）、健康度页随之改的两处、`checkItems` 的可缺的游标、`itemsReader`、审阅一节挪进这一次进入的 `SWRConfig`（它的 `HealthScope`，键带这次提交的 `submitted_at`）、树读失败、UP5 页面版本的两处断言 `4484ca7c`；三个动作（service、store 的 `behindReview`、审阅中的按钮一行、三个对话框、批准自己的函数与跳转、退回之后清 `checked`、退回在算时的文字与 20 秒的再读、横幅的"驳回"一句、两种语言、假服务端的三条路由）`ee298a0a`；UP6–UP8 的页面版本、`threeBrowsers` 加 `upgrade\/up7-`、`fixtures/upgrade-pages.ts` 的两个帮手 `05423d08`；从 `main` 的 `5a303fde` 开始，审查时在 `05423d08`，23 个文件、加 2,047 行、减 190 行，没有服务端、契约与依赖的改动）；修复一轮在它之上加 2 个提交（网页 `a040dcc1`；e2e `bc1a24e5`），头 `bc1a24e5`（11 个文件、加 440 行、减 61 行）；修复二轮在它之上加 1 个提交 `a25fef6d`（网页，2 个文件、加 119 行、减 5 行），头 `a25fef6d` |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（前几个 Step 的三个工作树挪到 `05423d08`），重的都经 `heavy.sh`，先读设计与代码、自己形成判断之后才看实施者的报告（报告里的数、49 条的变异表与 o20 的等价、9 条偏离、"自己加的"都当要核实的说法）：网页的行为（A：`git diff` 的网页部分逐行，连同没改但相关的 `ConfirmDialog`、`useMounted`、`modeChanged`、`health-request.ts`、契约的 `Upgrade` 与 wiki 的 `upgrades.sql` 怎样写 `submitted_at`；六个临时测试 A1–A6 复现，跑完删掉；`mut.py` 12 条加一条对照，四个测试文件 152 个；没有跑重的）；测试看不看得见、e2e 与逐个提交（B：七个测试文件 312 个的基线；自己的 19 条变异与实施者的 19 条重跑，五个临时测试 TMP-B1–B5；经 `heavy.sh` 的三组：头上 UP6–UP8 页面版本 `--repeat-each=5`（UP7 三种浏览器，25 个）与 o27、`leak` 两个页面变异各 `make build` 一次，UP7 的变体（首页放开）三种浏览器 × 3 与 `leak` 对它和"多等 1.5 秒"的 UP8，1、2 号各自检出跑 `make build`、`lint-web`、`knip`、`test-web` 与全部升级故事加 LW8）；设计一致、文字、无障碍、回归与文档稿（C：P2 第 9 节的七条、设计审查 C 的各条、决定 287–291 逐项对着代码；两种语言的文字逐条对过；五个探针 REVIEWC-1–4、REVIEWC-H；`mut.py` 16 条；判实施报告第 7 节各条进不进 3.25，看第 10–12 节的稿；没有跑重的） |
| 日期 | 2026-11-12（实施、审查、两轮修复与两轮修复核对） |
| 结论 | Important 2：退回的说明随页面此刻的升级变，不是"按重读之后的状态定"：重读回来之前先按旧的说，报告到了之后改说"状态不允许"，而那时恰恰可以退回（P3S3A-I1 = P3S3C-M1，实施者的偏离 6 判错）；"批准之后重连不再读升级"没有一处测试看得见：UP7 末段在变更集页的新文档里放开，那个文档从没装过升级页，UP8 放开之后不等就断言，vitest 的那一格没有事件流，一个卸下之后仍在重连时读升级的变异照过（P3S3B-I1，偏离 3 的理由不成立）。Minor 3（去重之后）：批准答复时页面已离开就不跳转（去掉 `mounted()` 照过，P3S3A-M1 = P3S3B-M2）、离开页面时审阅一节的"更多"被中止（`controllers` 换成不登记照过，P3S3A-M2 = P3S3B-M3）、驳回与退回先盖住答复再重读（`change(…, false)` 照过，P3S3A-N4 = P3S3B-M1），都没有测试。Nit 9：20 秒的次数在状态变了之后不清（P3S3A-N1 = P3S3C-N2）、批准之后按"后退"先显示旧按钮（P3S3A-N2）、假服务端的提交写死同一个 `submitted_at`（P3S3A-N3）、UP6 末段的注释（P3S3B-N1）、驳回之后只断言"Queued"（P3S3B-N2）、`checking` 的 `error === undefined` 没有测试（P3S3C-N1）、文档稿三处（`submission_id` 的四处、13.2 第 9、18 条与第 15 节、"1 checks"：P3S3C-N3–N5）。Q 2：别的管理员退回之后检查一节要不要也清（P3S3A-Q1 → 主持者的决定 300）；健康度页树第一次读失败时一直"加载中"（P3S3C-Q1 → 主持者的决定 301）。三位都没有查出码、请求次数、投影或焦点上的错；实施者的 9 条偏离：第 1、2、4、5、7 条判对，第 3 条的做法对、理由不成立，第 6 条不对，第 8 条不是偏离，第 9 条是过程。修复一轮（2 个提交，头 `bc1a24e5`）照 `fix-1.md` 改了全部代码与测试的发现、加两个决定：退回的说明在拒绝之后那一次重读回来时定一次，UP7、UP8 在首页放开流、等连接的最后一层答了再断言不读升级，三处缺的测试各一格；16 条变异改后全死（改前能改坏的 6 条与 e2e 的 `leak` 活，其余是改前缺的行为，由临时测试复现）；分支 CI 一次全绿；与修复说明不同之处 4 条（P3 设计 3.25）。修复核对一轮：B 把自己的发现都关上、没有新发现；C 把代码的发现关上，新的 Nit 四条（P3S3C-N6–N9）都在文档稿、在本提交；A 把自己的发现都关上、判决定 300 做对了，新的 P3S3A-M3（Minor）、N5、N6（Nit）由修复二轮改（`fix-2.md`）。修复二轮（1 个提交，头 `a25fef6d`）照 `fix-2.md` 改了 A 的三条：拒绝之后的重读被超过时再读一次再定说明、批准之后卸下时才清缓存、决定 300 的"换过提交"一格；改前活的 rb04 与"跳转之前清"、A 的探针 A8（`bc1a24e5` 上过）与 A10（失败）所示的两处，改后 6 条变异全死；分支 CI 一次全绿。修复核对二轮（A）：三条都关上，新的 4 条变异都被抓到，没有新的竞态；新的 P3S3A-N7（Nit：再读一次不看拒绝的码）不改。没有 Important、Minor 留下，修复到此为止 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：P3S3A-I1、M1、M2、N1–N4、Q1，P3S3B-I1、M1–M3、N1、N2，P3S3C-M1、N1–N5、Q1。同一个口子合为一行：A-I1 = C-M1，A-M1 = B-M2，A-M2 = B-M3，A-N4 = B-M1（取 B 的级别），A-N1 = C-N2。修复一轮照主持者的修复说明（`p3s3/review/fix-1.md`）在 `05423d08` 之上加新提交，按 `web` 与 `e2e` 分，普通推送、不改写已推的历史，每个单独为绿。审查时的代码位置是 `05423d08` 的；`W/` 指 `web/apps/web/src/`） | — |
| P3S3A-I1 = P3S3C-M1 | 重要（网页） | `W/pages/notebook/upgrade-page.tsx:491-495`（`explain` 读渲染时的 `checking`）、`:284`（`checking` 是页面此刻显示的升级）、`:217-218`（`change` 对拒绝 `void reread()`，不等它）。设计 4.3（设计审查 P3C-N14）要说明"按 `change` 重读之后的状态定"，实现每次渲染重读 `checking`，两头都会错：一、重读回来之前（A 的 A2、C 的 REVIEWC-2）：对话框在"审阅中、没有报告"时打开，别的管理员随后驳回，退回答 409，`role="alert"` 先写"The previous submission is still being checked: try again in a moment."，重读回来（`rolling_back`）之后改写"The upgrade's current state does not allow this."，读屏先后读出两句矛盾的话；反过来也一样。二、重读之后状态又变了（A 的 A1、C 的 REVIEWC-1，20 秒的再读要服务的那条路）：退回被拒为"正在计算"，用户开着对话框等，报告存下、`mode` 或 20 秒的再读把它带到页面上，开着的对话框改说"状态不允许"，这时恰恰可以退回。现有的格只数请求（`:1613`）或只看终态（`:1649`）。实施报告第 7 节第 6 条把它列为偏离、说"最终按重读之后的状态"，只对紧接着的那一次重读成立 | 修复一轮改（`a040dcc1`）：`change` 对这几个码 `await reread()` 再抛出，不另发第二个读；`sendBack` 的 `catch` 拿到重读的结果，"审阅中、没有报告"时记下这一次拒绝并挂计时器；`explain` 只认这一次拒绝，重读失败或读到别的状态时写通用的文字；测试：A1（报告到了之后仍写 `sendBackChecking`，或至少不写"状态不允许"）、A2（挡住重读时 `alert` 从头到尾没有出现 `sendBackChecking`）、C 的 REVIEWC-3（重读答 404 时不是"正在计算"，c21 要死）；同时改 A-N1 = C-N2；设计 4.3 与 3.25 照改（本提交）。核对一轮另查出一个窄的竞态（P3S3A-M3：拒绝之后的重读被 `mode` 的读超过时说明读到旧的缓存），修复二轮改 |
| P3S3B-I1 | 重要（测试） | `e2e/stories/upgrade/up7-approve.spec.ts:259`（`page.goto(changesetPath…)`）、`:267-274`（放开与断言）；`up8-web-only.spec.ts:134`（放开）、`:145`（断言）；`upgrade-page.test.tsx:1438`（"approved…is not read again"，没有事件流）。设计第 2 节给 S3 的 M5-events 一项与第 5 节 UP7 的页面版本都要"批准之后跳到首页、升级页卸下，之后的 `mode` 与重连不再读 `["upgrade", id]`"。实施把 dana 那一页的流挪到最后才放开（偏离 3），说"照样看得见"，不成立：`:259` 的 `page.goto` 是一次文档载入，SWR 的缓存每个文档一份（`W/app/providers.tsx:18`），变更集页的新文档从没装过升级页，`:274` 的 `not.toContain(GET …/upgrade)` 无论怎样都成立；UP8 在同一个文档里放开，但只做两次 API 调用就断言 `apiFailures` 为空，不等连接的重读；vitest 的"approved"一格没有事件流。B 的变异 `leak`（升级页挂上时往事件 hub 订一个不退订的回调，`connected` 时读升级）下现在的 UP7、UP8 都通过；UP7 改为在首页放开、等树的重读答了再断言，或 UP8 在断言之前多等 1.5 秒，都红。对照：o27（批准成功之后 `reread()`）由 UP7、UP8 抓到，看不见的只是重连那一半 | 修复一轮改（vitest `a040dcc1`，e2e `bc1a24e5`）：UP8 放开之前登记 `waitForRequest`、放开之后等这次连接的最后一层读答了，再断言放开之后没有 `GET …/upgrade`、`apiFailures` 为空；vitest 的"approved"一格换 `withEvents`，批准之后发一次 `mode`、断开重连，请求记录仍只有批准之前的；UP7 在首页放开、等到第四层（`GET …/assets`）也答了再去变更集页，三种浏览器经 `heavy.sh` 各三轮，不稳就删掉末段的断言与注释的说法；实施者选了第一种（帮手 `letInAtHome`），本机 12 次、核对时 27 次都过；`leak` 改后 UP8 与 UP7 三种浏览器 4 个全红，只在卸下之后才读的变体由新的 vitest 一格抓到；偏离 3 照最后的做法重写（3.25，本提交） |
| P3S3A-M1 = P3S3B-M2 | 中（测试） | `upgrade-page.tsx:276-278`：设计 4.3（第 289 条，"若页面还装着（`useMounted`）"）与 v0.1 设计 13.2 第 16 条：写的答复改路由，页面已经离开就哪也不去。批准在飞时按浏览器的"后退"离开，答复回来时变异下用户被拉回笔记本首页。A 的 ra01、B 的 b12（去掉 `mounted()`）对全部测试照过；A 的临时测试 A5（停在 `/lab`）、B 的 TMP-B3 都抓到 | 修复一轮补一格（A5 的写法）：去掉 `mounted()` 要红（`a040dcc1`；ra01、b12 改后死） |
| P3S3A-M2 = P3S3B-M3 | 中（测试） | `W/pages/notebook/upgrade-check.tsx:309-312`：审阅一节的 `HealthScope` 的 `controllers` 是这一次进入的（设计 4.3；`upgrade-page.tsx:108` 的注释也这么写），离开升级页中止"更多"的请求。A 的 ra04、B 的 b08（`controllers.add` 登记到别的 `Set`）照过；检查一节有"a check under way, the page left: its request is aborted"（`upgrade-check.test.tsx:545`），审阅一节没有（05-lessons"两处复写的同一判断各自有测试"）。后果窄：答复回来时由 `current()` 丢掉 | 修复一轮照 `:545` 补一格：把 `controllers` 改坏要红（`a040dcc1`；ra04、b08 改后死） |
| P3S3A-N4 = P3S3B-M1 | 中（测试） | `upgrade-page.tsx:505`（驳回）、`:307`（退回）：设计 4.3"驳回与退回留在升级页，照现有的 `change` 盖住答复再重读"。两格的重读都立刻答，看不出答复有没有先盖上；放弃、继续、提交各有一格把第二次读停住再断言（`:299`、`:321`、`:1251`）。`shows` 改成 `false`（A 的 ra10、ra11，B 的 b06、b07）照过：对话框关上之后、重读回来之前，页面仍是审阅中、三个按钮都在。B 的 TMP-B1、TMP-B2 抓到 | 修复一轮驳回、退回各补一格：挡住答复之后的那次 `GET upgrade`，断言对话框关上时页面已是回滚一节或接入一节；ra10、ra11 要死（`a040dcc1`；ra10、ra11、b06、b07 改后死） |
| P3S3A-N1 = P3S3C-N2 | 细节（网页） | `upgrade-page.tsx:281-293`：`sentBackRefused` 只在计时器到时与下一次退回开始时归零，状态变了时 effect 清掉了计时器、次数仍是 1，之后页面再显示"审阅中、没有报告"（例如报告存下、别的管理员退回、agent 再交）时，没有新的退回被拒也挂上 20 秒、多读一次升级（A 的 A3 与对照 A3c，C 的 REVIEWC-4）。A 的 ra05（`sendBack` 开头不归零）同一个原因照过 | 随 P3S3A-I1 改：被拒的是哪一次提交照 `submitted_at` 记，只在同一次提交仍"审阅中、没有报告"时挂；REVIEWC-4 与 A3 一格，ra05 成为等价或死 |
| P3S3A-N2 | 细节（网页） | `upgrade-page.tsx:276-278`：批准成功那一支不动缓存，批准之后按"后退"回到升级页，SWR 先显示审阅中的旧数据与三个按钮，直到重读答 404（A 的 A4）；离上一次读不到 2 秒时根本不发请求。按下去只会答 409、再重读，没有害处 | 修复一轮改：跳转完成之后（页面已卸下）`mutate(["upgrade", id], undefined, { revalidate: false })` 清掉这一项；A4 作一格（`a040dcc1`）。实施在 `await navigate` 之后就清，核对一轮查明那时升级页还挂着、闪成"加载中"（P3S3A-N5），修复二轮改为卸下时才清 |
| P3S3A-N3 | 细节（测试） | `W/test/upgrade-server.ts:425`：假服务端的提交总写 `"2026-10-06T09:40:00Z"`；真服务端每次提交（含接管）写锁下读的 `now`。S3 以 `submitted_at` 认"这一次提交"，将来"同一次进入里提交、退回、再提交"的格子会接着旧的条目。现在"另一次提交"的格子都以 `withUpgrade(...)` 直接给新值，没有失真 | 修复一轮改：提交路由每次给新的 `submitted_at`（例如计数加分钟）（`a040dcc1`：从 `09:40` 起每次加一分钟，原有的格都照过） |
| P3S3B-N1 | 细节（测试） | `e2e/stories/upgrade/up6-send-back-reject.spec.ts:322-329`：注释说"连接读的是显示的东西、不是升级"，但 `:323` 的 `page.goto` 是新文档，`:328` 的横幅立刻成立，`:329` 不等连接就断言。设计只要"离开升级页之后才放开"，这一段本身无害 | 修复一轮照实改注释为"离开之后才放开"，或删掉末两行的说法（`bc1a24e5`：改注释） |
| P3S3B-N2 | 细节（测试） | `upgrade-page.test.tsx:1495-1513`：驳回之后只断言任务行里的"Queued"，设计 4.3 写"任务的整行"（与放弃那一格同一个写法，P1 起如此） | 修复一轮改：断言任务的整行（名字、状态、发起人）（`a040dcc1`：名字、状态、开始的时刻、"取消"；驳回是本账号发起的，行里本就不写"由谁"） |
| P3S3C-N1 | 细节（测试） | `upgrade-page.tsx:284`：`checking` 的 `error === undefined` 没有测试看着（C 的 c21 活）：页面显示"审阅中、没有报告"，其间报告已存下、别的管理员已批准，退回答 409、重读答 404，SWR 留着旧的 `data`；去掉这一合取时对话框写"正在计算"、20 秒之后再读一次 404。C 的 REVIEWC-3 原样通过、c21 下失败 | 随 P3S3A-I1 改：REVIEWC-3 作一格，c21 要死 |
| P3S3C-N3 | 细节（文档） | 偏离 1（键用 `submitted_at`）对，但实施报告的文档稿漏了另几处：00 号文档决定 287（"键带 `submission_id`"）、设计 4.3 的 `key`、`current()`（"显示的升级的 `submission_id` 等于键里的"）与 vitest 清单（"键带 `submission_id`"） | 本提交：四处改为这次提交的时刻 `submitted_at`，3.25 第 1 条注明；设计审查记录里 P3C-M4 的原话是历史，不改 |
| P3S3C-N4 | 细节（文档） | 实施报告第 11 节的稿：13.2 第 18 条（只给管理员的读）没写审阅一节卡的"更多"答 403 时同样重读 `["notebooks", ws]`（只在这一次进入、这次提交当前时）；13.2 第 9 条的"（跳到首页就放开，下一次载入会截断连接的重读，WebKit）"读来像做法，其实是不在首页放开的理由；设计第 8 节第 6 条末项要每个 Step 在 v0.1 总体设计第 15 节加一行，稿里没有 | 本提交：三处照改；13.2 第 9 条的句子另照 P3S3B-I1 修复之后的 UP7、UP8 写 |
| P3S3C-N5 | 细节（文字，范围之外） | `W/i18n/messages/en.ts:346` 的 `health.clear` 是"No issues ({count} checks)"，只有一条计数为 0 的规则时写"1 checks"；健康度页（M10）与检查一节（P2/S3）早就这样，S3 的报告一节照用，`upgrade-page.test.tsx:1113` 把它写进了断言。不是 S3 引入的 | 本提交：进 M12 的打磨移交（第 41 条） |
| P3S3A-Q1 | 问题（给主持者） | `upgrade-page.tsx:314`（只有自己的退回 `setChecked({ generation: 0, upgrade: "" })`）、`:348`：检查、提交、别的管理员退回、`mode` 事件之后，检查一节又出现，写"Every check passed …"，是提交之前缓存里的那一代（A 的 A6）。设计 4.3（P3C-N15）清掉自己退回之后的，理由"不显示提交之前缓存里的结果，那容易让人误读"对别人的退回同样成立；不能简单地"从审阅中回到 upgrading 就清"：提交被拒为 `not_ready`、`stale_index` 时 `submit` 故意 `recheck`，要保留新的一代 | 主持者的决定 300（下面）：两者一样清，判法照 A；修复一轮改，A6 作一格 |
| P3S3C-Q1 | 问题（给主持者） | `W/pages/notebook/health-page.tsx:245`（`readingTree={pages.nodes === undefined \|\| tree.reading}`）：树第一次读失败时 `pages.nodes` 一直是 `undefined`，健康度页上方写"加载失败"、条目却一直写"Loading…"（C 的 REVIEWC-H）。这正是 P2 第 9 节第 7 条在升级页上要 S3 改掉的那一种；设计 4.3 有意让健康度页"不交 `treeFailed`，照旧"，不是 S3 的回归 | 主持者的决定 301（下面）：S3 的修复轮里改，REVIEWC-H 作一格；设计 4.3 那一句由本提交改 |

## 主持者的决定

负责人可以改判（300、301 在 S3 的第一轮修复说明里定下）：

300. **别的管理员退回之后，检查一节同样清掉提交之前的结果**（P3S3A-Q1，2026-11-12）：与自己的退回一样（设计 4.3，P3C-N15 的理由对两者都成立）。判法照 A：只在回到 `upgrading` 的那一次读之前显示过报告，或 `submitted_at` 换过时清；提交被拒为 `not_ready`、`stale_index` 时也从审阅中回到 `upgrading`，那时照旧保留 `submit` 故意 `recheck` 的那一代。A 的 A6 作一格，被拒的提交那一格照旧过。"显示过报告"按这一次进入的页面读到的算：页面只读到"审阅中、没有报告"就看到 `upgrading` 时，分不出是提交被拒还是过了接管时刻的退回，不清（修复一轮的做法，审查 A 判对）。
301. **健康度页在树第一次读失败时，条目不再一直写"加载中"，在 S3 的修复轮里改**（P3S3C-Q1，2026-11-12）：`readingTree` 照升级页加 `tree.error === undefined`，交 `treeFailed={tree.error !== undefined}`；C 的 REVIEWC-H 作一格。树再读失败时同样："更多"带来、树里没有的节点空着，不写"节点已不可用"（修复一轮的后果，核对一轮 P3S3C-N7 要文字写上）。理由：`HealthRule` 在 S3 已收 `treeFailed`，改两处即可，与升级页的两节一个样子；设计 4.3"健康度页不交 `treeFailed`"那一句由本提交改。

另几项不编号的处置：实施者"实施中定"的几条（提交的标识用 `submitted_at`、树读失败的提示的位置、"读着"先于"失败"、UP7 的流、退回的按钮 `outline`）都不是要编号的产品决定，记在 P3 设计 3.25（审查 C）；P3S3C-N3–N5 由本提交改。

修复一轮的处置照 `fix-1.md`：两条 Important、三条 Minor、Nit 里 A-N1 = C-N2、A-N2、A-N3、B-N1、B-N2、C-N1 与两个决定都改代码或测试，在 `05423d08` 之上加提交，按 `web` 与 `e2e` 分，每个单独为绿，普通推送；变异先在 `05423d08` 上跑、要活（A 的 A1–A6 写法对应的改坏与 ra05、ra10、ra11，B 的 `leak`，C 的 c21、REVIEWC-4、REVIEWC-H 对应的改坏，去掉 `mounted()`、改坏 `controllers`），改后要死；逐个提交跑 build、`make lint-web`、`make knip`、`make test-web`、`make gen-check`，碰 `e2e/` 的另经 `heavy.sh` 跑相关故事；头上 `make lint gen-check knip build`，经 `heavy.sh` 跑 `make test-web`、`make e2e`，不必整轮 `make test`（只改网页与 e2e）；e2e 卡在 apt 镜像或红在 L1 时照第 277 条只重跑 e2e。文档的 P3S3C-N3–N5 与 3.25 的取舍在本提交。

## 接受与推后的

- **接受**：
  - 等价的变异：o20（报告的卡的 React 键只用规则，不带提交的时刻）：卡的状态在 `[...scope.key, "view"]`，键带提交的时刻，`HealthRule` 与 `useHealthLoadMore` 的内部状态（`operation`、`busy`、`readers`、`focus`、`blocked`）都按 `identity`（键加规则）分，换了提交就是新的 `identity`；对照 o05（`HealthScope` 的键不带它）被"the report's cards are its submission's"抓到，同一格在 o20 下通过（实施者、A、B 各自核过）。A 的 ra03（审阅一节 `current()` 不看登录）、B 的 b09、b10（不看登录、不看 `alive`）与检查一节同样的 c01、c02：换登录是新一代的 providers、整页重挂，`alive` 为假时这一次进入的缓存已清，P2 起如此。A 的 ra08（键去掉 `upgrade.id`）：`submitted_at` 唯一时等价，设计要的，留着。A 的 ra12（批准的拒绝不看 `mounted()` 就重读）：卸下之后绑定的 `mutate` 对没有 hook 挂着的键不发请求。B 的 b01（审阅一节闸门的节点不看 `treeFailed`）：存下的报告只在闸门全过时有，带节点的三个闸门通过时节点表为空。B 的 b11（批准答 `forbidden`、`notebook.not_found` 也重读升级）：store 先重读列表，页面随之不再是管理员或笔记本不在，`useSWR` 的键变成 `null`，`reread()` 什么也不读（TMP-B5）。C 的 c05（`checkItems` 的查询恒为 `{ cursor }`）：openapi-fetch 0.17.0 跳过 `undefined`。A 的 ra09（`checking` 不看 `error`）：设计没说重读失败时说哪一句，照 P3S3A-I1 改了之后这一项自然消失。
  - 未核实的怀疑：A：自己退回成功的那一刻，`change` 先 `mutate(answered)`，React 可能在 `setChecked(0)` 之前以旧的一代渲染一次检查一节，那一代还在飞时可能多发一次检查（"checked, submitted and sent back"一格断言检查只读一次，覆盖了常见的路；C 同样没有实测）；批准时 `mode` 先于答复到达，升级页还装着，重读答 404、对话框后面先一闪"没有进行中的升级"（e2e 以 `holdStream` 挡住，vitest 没有带事件流的批准的格子）。B：20 秒的计时器那几格用 `shouldAdvanceTime: true`，先推 19 秒断言"还没读"，余量是不到 1 秒的真时间，CI 的 web 作业负载高时可能早读一次（P1 的回滚几格同一个写法，没见过红，没有复现）。C：o20 的等价只读了代码（A、B 核过）。
  - 核对过、没有问题的（三位各自的"核对过"一节）：P2 第 9 节交给 P3/S3 的七条（C 逐条）；设计审查 C 的各条（P3C-I1、M1–M4、N12–N15、N20–N24）；决定 287–291；三个动作的 service 与 store（路径与方法、`inTurn` 与 `upgraded`、`behindReview` 的三个码、批准不收 `not_ready`）；按钮的显示条件与横幅（状态 × 角色整组断言）；批准自己的函数（拒绝为状态与 `not_ready` 时重读、503 不读、成功不写缓存不重读、对话框随页面卸下时 `onCloseAutoFocus` 不抢首页标题的焦点）；"更多"从第一页读与按 `item_key` 去重；这一次进入的缓存与 SWR 2.5.1 的两个行为（卸下的 hook 照调回调、provider 卸下删全局状态）；假服务端的三条路由只答契约里有的码、码序照 3.21；两种语言的文字逐条对过设计 4.3 的表；无障碍（按确切的名字找、横幅仍是 `<output>`、焦点、`fieldset aria-label="Page paths"`）；健康度页的回归（`paths` 放宽、`read` 抛缺陷不可达、`health-page.test.tsx` 未改而通过）；e2e 的刻意错误（UP6 恰好一个 404、UP7 一个 409，都在触发之前声明）、值来自答复、稳定性（头上五轮 25 个全过，连同实施者的三轮、B 的两次 `stories/upgrade/` 与 CI 没见过一次红）；偏离 1、2、4、5、7（A、B、C 判对）。
  - 过度设计：报告的卡的 React 键带 `submitted_at`（o20）、键里的 `upgrade.id`（ra08）、`checkItems` 的 `cursor === undefined ? {} : { cursor }`（c05）：都多余但无害，留着与检查一节一个写法、读来更明白，不要求改。
  - 实施者的 9 条偏离（实施报告第 7 节）：第 1、2、4、5、7 条判对，进 3.25；第 3 条（UP7 的流最后才放开）做法对、"照样看得见"的理由不成立（P3S3B-I1），照修复之后的做法写进 3.25；第 6 条（退回的说明读页面此刻的升级）不对（P3S3A-I1 = C-M1），改代码，不进 3.25；第 8 条（UP7 的 API、MCP 版本也在三种浏览器）不是偏离，7.5 本就这样写，第 5 节与第 8 节第 6 条的句子照 7.5 改（C）；第 9 条（标题写行为的改变）是过程，只进 10.3（C）。
  - 流程（照实写下）：推送之前改写过两次、都没有推过（变异有活下来的，补的测试并进 1、2 号；3 号另 `--amend` 一次），非交互地重建、核对新头的树等于旧头加改动，重建之后逐个提交、整张变异表、门禁都在新头上重跑；第二次重建时停下刚开始的门禁用的是 `TaskStop`，没有留下锁；每次提交之前看过 `git status`；没有推 `main`，没有 force-push。
  - 修复核对一轮接受的：A 判修复的偏离 1（所有经 `change` 的对话框等重读回来再抛出，六个）可以接受：拒绝的文字晚一次读出现，重读挂住时对话框跟着挂住（与 POST 挂住同一类），文字与身后的页面一致，P1、P2 的格都照过；按情形走过重读失败、读着时离开、同一个对话框再按、提交被拒为 `not_ready` 四种。rb08 等价（上面）。未核实的怀疑：B：`letInAtHome` 等的是第四层里的 `assets` 一个读，首页以后若在第四层多挂一个键，UP7 的 `page.goto` 可能截断它，可以在帮手的说明里写明这个前提（没有复现）。C：别人退回（决定 300，经 `useEffect` 清）与自己退回时，检查一节都先以旧的那一代提交一次渲染再变空（自己退回那一种在 `05423d08` 上就是这样），都没有多发请求，按 React 18 应在上屏之前清掉，没有在浏览器里证实；要彻底避免可在渲染时算 `generation` 或改 `useLayoutEffect`。
  - 修复核对二轮的 P3S3A-N7（"再读一次"不看拒绝的码，断网等拒绝碰上有读在飞时多读一次升级）：不改，只在断网而有读在飞时多读一次，无害。
- **推后**：
  - 英文的"No issues (1 checks)"：M12 的打磨（第 41 条）。
  - P3 的完成：之后的 `docs: M11/P3 done`（P3 设计第 8 节第 5 条）。

## 偶发

实施、审查、两轮修复、两轮修复核对与三次分支 CI 里都没有。三次分支 CI 的 e2e 都一次过，L1 没有出，不用照第 277 条重跑。实施时 UP7 的页面版本照设计在首页放开流，WebKit 失败一次（`Load request cancelled`，下一步的 `page.goto` 截断连接的重读），是写法的问题，不是偶发（偏离 3）。

## 反向对照

建成时以 `mut.py`（`NW_SERVER` 指实施者的工作树的 `web/apps/web`）跑，经 `heavy.sh`，在最后的头 `05423d08` 的树上（`mut/run-final.out`；s3-03b 是看了结果之后加的一条，单独跑，`mut/run-03b.out`）；先在没改的树上把表里 7 条不同的命令各跑一次（都 rc=0：71、138、14、38、105、67、104 个测试）；从 `.mutbak` 复原，之后 `git status` 干净、没有 `*.mutbak`。一共 49 条：7.3 的 S3 14 行 19 条（"不设 20 秒的再读、或卸下之后不清"拆成 s3-12a–d 四条，"批准之后不跳转、或不交 `arrived`"两条，"审阅一节留在 `SWRConfig` 之外"两种写法）与实施者自己的 30 条；48 条抓到，o20 等价（上面）。写代码时活下来的四条各补了测试、并进 1、2 号：s3-12b（计时器不清：卸下之后 SWR 对没挂着的键不发请求，设计的"卸下之后不再读"一格看不见，改为离开 5 秒再回来，另加"`mode` 带来报告之后不再读"一格，拆成 s3-12c、s3-12d）、o09（`reading` 不看 `isValidating`，重试挡住的一格）、o29（`itemsReader` 的 403 不看 `current()`，换了一次提交之后才到的 403）、o30（退回的任何拒绝都挂计时器，断网一格）。s3-03 的第一种写法（审阅中整个不经 `SWRConfig`）连带让 `SWRConfig` 在进出审阅时卸下又装上，先被检查一节的两格抓到，说明不了设计指的那一格；第二种（s3-03b，`SWRConfig` 一直挂着、审阅一节放在它外面）才是设计说的变异。

**7.3 的 S3 各行与实施者自己的**：

| 名字 | 变异（怎样改坏） | 跑的测试：抓到它的第一格 | 结果 |
|---|---|---|---|
| s3-01-undefined-cursor-no-more | `HealthRule` 在游标为 `undefined` 时不渲染"更多"（只认字符串） | `upgrade-check.test.tsx`：“the report's More reads the check's items from their first page, without a cursor: the ite” | 抓到 |
| s3-02-report-not-through-itemsReader | 审阅一节的读不经 `itemsReader`（直接 `checkItems`，403 不重读列表） | `upgrade-check.test.tsx`：“the report's More refused as forbidden reads the notebooks again: the account no longer th” | 抓到 |
| s3-03-report-outside-swrconfig | 审阅一节留在这一次进入的 `SWRConfig` 之外（审阅中整个不包：`SWRConfig` 随之卸下） | `upgrade-check.test.tsx`：“a check under way as another's submission takes the check away and gives it back: it shows” | 抓到 |
| s3-03b-report-outside-swrconfig-kept-mounted | 审阅一节留在 `SWRConfig` 之外，`SWRConfig` 一直挂着 | `upgrade-check.test.tsx`：“the report's cards are this entry's: More read, the page left and come back to starts agai” | 抓到 |
| s3-04-approve-without-report | 没有报告时也显示"批准…" | `upgrade-page.test.tsx`：“an upgrade reviewing without a report: the check and Submit for review false, the report t” | 抓到 |
| s3-05-abandon-in-review | 审阅中仍显示"放弃这次升级…" | `upgrade-page.test.tsx`：“an upgrade reviewing: the agent's section false, Abandon false; a rollback's Continue and ” | 抓到 |
| s3-06-banner-review-says-abandon | `reviewing` 的横幅仍写"放弃"那一句 | `upgrade-banner.test.tsx`：“an upgrade reviewing: a notebook's admin reads it in a status above the page” | 抓到 |
| s3-07-approve-uses-notReady | 批准的 `not_ready` 用 `upgrade.notReady` | `upgrade-page.test.tsx`：“an approval refused as a gate checked again not passing: the dialog says so, the upgrade r” | 抓到 |
| s3-08-approve-through-change | 批准经 `change`（成功之后写缓存、重读升级） | `upgrade-page.test.tsx`：“approved, the page goes to the notebook's home, its title focused, Plans an LLM Wiki witho” | 抓到 |
| s3-09-sendback-explain-at-open | 退回的说明按打开对话框时的状态定（ref 记下打开时的 `checking`） | `upgrade-page.test.tsx`：“sent back while its submission was still checked, refused as another admin rejected it mea” | 抓到 |
| s3-10-sendback-keeps-checked | 退回之后不清 `checked` | `upgrade-page.test.tsx`：“checked, submitted and sent back, the check shows nothing of before: Check is to be presse” | 抓到 |
| s3-11-sendback-shows-detail | 退回的 409 显示服务端的 `detail` | `upgrade-page.test.tsx`：“sent back while its submission is still checked, refused: the dialog says so, not the serv” | 抓到 |
| s3-12a-no-20s-reread | 不设 20 秒的再读（到时不 `reread`） | `upgrade-page.test.tsx`：“sent back while its submission is still checked, refused: the dialog says so, not the serv” | 抓到 |
| s3-12b-20s-not-cleared | 20 秒的计时器不清（effect 不返回清理） | `upgrade-page.test.tsx`：“sent back while its submission is still checked, refused, the page left and come back to: ” | 抓到 |
| s3-12c-not-cleared-on-unmount | 卸下时不清（只在还装着时清） | `upgrade-page.test.tsx`：“sent back while its submission is still checked, refused, the page left and come back to: ” | 抓到 |
| s3-12d-not-cleared-on-state-change | 状态变了不清（依赖里去掉 `checking`） | `upgrade-page.test.tsx`：“sent back while its submission is still checked, refused, then its report kept, shown by t” | 抓到 |
| s3-13a-approve-no-navigate | 批准之后不跳转 | `upgrade-page.test.tsx`：“approved, the page goes to the notebook's home, its title focused, Plans an LLM Wiki witho” | 抓到 |
| s3-13b-approve-no-arrived | 批准之后跳转不交 `arrived` | `upgrade-page.test.tsx`：“approved, the page goes to the notebook's home, its title focused, Plans an LLM Wiki witho” | 抓到 |
| s3-14-fake-sendback-200-without-report | 假服务端的退回在没有报告时答 200 | `upgrade-page.test.tsx`：“sent back while its submission is still checked, refused: the dialog says so, not the serv” | 抓到 |
| o01-first-cursor-ignores-capped | 卡的初值的游标不看 `count_capped` | `upgrade-check.test.tsx upgrade-page.test.tsx`：“in review, the page shows the report kept: each gate, each count, each rule a card of its ” | 抓到 |
| o02-first-cursor-more-when-whole | 条目已是全部（`count == items.length`）也给"更多"（`>=`） | `upgrade-check.test.tsx upgrade-page.test.tsx`：“in review, the page shows the report kept: each gate, each count, each rule a card of its ” | 抓到 |
| o03-report-no-disclosure | 计数为 0 的规则不收进 `<details>` | `upgrade-check.test.tsx upgrade-page.test.tsx`：“in review, the page shows the report kept: each gate, each count, each rule a card of its ” | 抓到 |
| o04-report-cards-level-2 | 报告的卡的标题是 `h2` | `upgrade-check.test.tsx upgrade-page.test.tsx`：“the report's More reads the check's items from their first page, without a cursor: the ite” | 抓到 |
| o05-report-key-without-submission | `HealthScope` 的键不带提交的时刻 | `upgrade-check.test.tsx`：“the report's cards are its submission's: sent back and submitted again, the new report sta” | 抓到 |
| o06-report-no-tree-failure | 审阅一节不显示树读失败 | `upgrade-check.test.tsx`：“the tree failed to read: the report says so above its nodes, left empty; Try again reads i” | 抓到 |
| o07-report-items-not-treeFailed | 审阅一节的卡不交 `treeFailed` | `upgrade-check.test.tsx`：“the tree failed to read: the report says so above its nodes, left empty; Try again reads i” | 抓到 |
| o08-gate-node-not-treeFailed | `GateNode` 不看 `treeFailed`（写"（已不在）"） | `upgrade-check.test.tsx`：“the tree failed to read: the check, checked says so above its nodes, left empty; Try again” | 抓到 |
| o09-tree-reading-not-validating | `tree.reading` 不看 `isValidating` | `upgrade-check.test.tsx`：“the tree failed to read: the check, checked says so above its nodes, left empty; Try again” | 抓到 |
| o10-report-no-state-changed | 报告一节不写"升级的状态变了" | `upgrade-check.test.tsx`：“the report's More refused as the upgrade no longer in review says its state changed and re” | 抓到 |
| o11-approve-not-ready-no-reread | 批准的 `not_ready` 不重读升级 | `upgrade-page.test.tsx`：“an approval refused as a gate checked again not passing: the dialog says so, the upgrade r” | 抓到 |
| o12-approve-any-refusal-rereads | 批准的任何拒绝都重读升级（503 也读） | `upgrade-page.test.tsx`：“an approval refused as busy: the dialog says so, the upgrade read again where that says th” | 抓到 |
| o13-approve-root-never-named | 批准的正文从不写来源区的路径 | `upgrade-page.test.tsx`：“the review's dialogs: Approve, plain, names the source area it makes read only by its path” | 抓到 |
| o14-approve-destructive | 批准的对话框是 `destructive` | `upgrade-page.test.tsx`：“the review's dialogs: Approve, plain, names the source area it makes read only by its path” | 抓到 |
| o15-reject-plain | 驳回的对话框是 `default` | `upgrade-page.test.tsx`：“the review's dialogs: Approve, plain, names the source area it makes read only by its path” | 抓到 |
| o16-behind-review-no-state | store 的 `behindReview` 没有 `wiki.upgrade_state` | `notebook.store.test.ts`：“an upgrade's approval refused as wiki.upgrade_state reads the list again, as it is now, an” | 抓到 |
| o17-behind-review-not-ready | store 的 `behindReview` 收 `wiki.upgrade_not_ready` | `notebook.store.test.ts`：“an upgrade's approval refused as ApiError does not read the list again” | 抓到 |
| o18-fake-approve-never-not-ready | 假服务端的批准从不答 `not_ready` | `upgrade-page.test.tsx`：“an approval refused as a gate checked again not passing: the dialog says so, the upgrade r” | 抓到 |
| o19-items-read-again-listed-twice | `HealthRule` 不按 `item_key` 去重 | `health-rule.test.tsx upgrade-check.test.tsx`：“duplicate pages add no row or focus target; a later unique item and repeated announcements” | 抓到 |
| o20-report-cards-keyed-by-rule-only | 报告的卡的 React 键只是规则（不带提交的时刻） | `upgrade-check.test.tsx` | **没抓到** |
| o21-report-first-items-none | 卡的初值的条目为空（常量） | `upgrade-check.test.tsx upgrade-page.test.tsx`：“the report's More reads the check's items from their first page, without a cursor: the ite” | 抓到 |
| o22-report-time-is-submission | 报告的时刻写提交的时刻（`submitted_at` 代 `computed_at`） | `upgrade-check.test.tsx upgrade-page.test.tsx`：“the report's More refused as forbidden once another submission is shown reads nothing agai” | 抓到 |
| o23-card-count-never-capped | 卡的数不写"+"（`count_capped` 写死 `false`） | `health-rule.test.tsx upgrade-check.test.tsx upgrade-page.test.tsx`：“a check's rules with issues are cards headed under the check, the others in a disclosure; ” | 抓到 |
| o24-approve-button-no-ref | "批准…"没有 ref（关上之后焦点去标题） | `upgrade-page.test.tsx`：“the review's dialogs: Approve, plain, names the source area it makes read only by its path” | 抓到 |
| o25-sendback-button-no-ref | "退回继续修改…"没有 ref | `upgrade-page.test.tsx`：“the review's dialogs: Approve, plain, names the source area it makes read only by its path” | 抓到 |
| o26-reject-button-no-ref | "驳回…"没有 ref | `upgrade-page.test.tsx`：“the review's dialogs: Approve, plain, names the source area it makes read only by its path” | 抓到 |
| o27-approve-rereads-after-success | 批准成功之后 `reread()` | `upgrade-page.test.tsx`：“approved, the page goes to the notebook's home, its title focused, Plans an LLM Wiki witho” | 抓到 |
| o28-approve-state-no-reread | 批准的 `wiki.upgrade_state` 不重读升级 | `upgrade-page.test.tsx`：“an approval refused as in another state: the dialog says so, the upgrade read again where ” | 抓到 |
| o29-items-forbidden-rereads-when-not-current | `itemsReader` 的 403 不看 `current()` | `upgrade-check.test.tsx`：“the report's More refused as forbidden once another submission is shown reads nothing agai” | 抓到 |
| o30-sendback-refusal-any-code-arms | 退回的任何拒绝都记一次（断网也挂计时器） | `upgrade-page.test.tsx`：“sent back while its submission is still checked, the network failing: the dialog says so; ” | 抓到 |

**审查者的变异**（照各自的命名；"活"的都成了一条发现，等价的除外；规格与输出在 `p3s3/review/a/`、`b/`、`c/`；每条都先在没改的树上跑过同样的命令）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | ra01（批准成功之后不看 `mounted()` 就跳转） | 四个测试文件照过：P3S3A-M1；临时测试 A5 抓到（ra01k） |
| A | ra02（审阅一节的 `current()` 不比显示的 `submitted_at`） | 抓到 |
| A | ra03（审阅一节的 `current()` 不看登录）、ra08（键去掉 `upgrade.id`）、ra12（批准的拒绝不看 `mounted()` 就重读） | 照过：等价（上面） |
| A | ra04（审阅一节的 `controllers.add` 加进一个新的 `Set`） | 照过：P3S3A-M2 |
| A | ra05（`sendBack` 开头不把次数归零） | 照过：与 P3S3A-N1 同一个原因 |
| A | ra06、ra07（`HealthItem`、`GateNode` 失败压过读着） | 抓到 |
| A | ra09（`checking` 不看 `error`） | 照过：设计没说，照 P3S3A-I1 改了之后消失 |
| A | ra10、ra11（驳回、退回经 `change(…, false)`） | 照过：P3S3A-N4 |
| A | 临时测试 A1–A6 | 照"现在的行为"写，都通过：A1、A2 复现 P3S3A-I1，A3 与对照 A3c 复现 N1，A4 复现 N2，A5 复现 M1，A6 复现 Q1 |
| B | b01（审阅一节闸门的节点不看 `treeFailed`）、b09、b10（`current()` 不看登录、不看 `alive`）、b11（批准答 `forbidden`、`notebook.not_found` 也重读）、c01、c02（检查一节的两项，对照） | 照过：等价（上面） |
| B | b02–b05（报告在 `rolling_back` 也显示、初值游标对调、两组对调、"更多"总带 `cursor`） | 抓到 |
| B | b06、b07（驳回、退回 `shows=false`） | 照过：P3S3B-M1；临时的 TMP-B1、TMP-B2 抓到 |
| B | b08（`controllers` 登记到丢掉的 `Set`） | 照过：P3S3B-M3；TMP-B4 抓到 |
| B | b12（批准成功之后无条件 `navigate`） | 照过：P3S3B-M2；TMP-B3 抓到 |
| B | b13–b16（"驳回"那一句给所有角色、`upgrading` 多一个"驳回…"、退回的 `explain` 不看 `checking`、20 秒不看 `checking` 也挂） | 抓到 |
| B | 实施者的 19 条重跑（s3-03b、s3-08、s3-09、s3-12b/c/d、s3-13b、s3-14、o01、o05、o09、o16、o18、o20、o22、o24、o27、o29、o30） | 18 条抓到，o20 照过，与实施报告一致 |
| B | o27（e2e：批准成功之后 `reread()` 再跳转） | UP7、UP8 页面版本（Chromium，`make build` 之后）都红 |
| B | `leak`（e2e：升级页挂上时往事件 hub 订一个不退订的 `connected` 回调，读升级） | 现在的 UP7、UP8 页面版本都过：P3S3B-I1；UP7 的变体（首页放开、等树）与"末尾多等 1.5 秒"的 UP8 都红 |
| C | c01–c04、c07–c14、c17、c19（`fieldset` 的标签、`HealthItem`、`GateNode`、`HealthRule` 的 `treeFailed`、`approveRootUnread`、卡的级别、初值的游标、`tree.reading`、两个对话框的语气、横幅、批准的 `not_ready`、`startBody`、批准的正文） | 14 条抓到 |
| C | c05（`checkItems` 的查询恒为 `{ cursor }`） | 照过：等价（openapi-fetch 跳过 `undefined`） |
| C | c21（`checking` 去掉 `error === undefined`） | 照过：P3S3C-N1；探针 REVIEWC-3 抓到 |
| C | 探针 REVIEWC-1、2、4、H | 头上复现 P3S3C-M1 的两种、N2、Q1 |

**修复一轮的**（实施者以 `mut.py` 经 `heavy.sh` 跑，命令是 `upgrade-page`、`upgrade-check`、`health-rule`、`health-page`、`upgrade-banner` 的测试与 `notebook.store.test.ts`；先在没改的树上跑同一组命令，改前基线 270 个、改后 280 个都过；从 `.mutbak` 复原，收尾干净；规格与结果在 `p3s3/fix1/`）：

| 变异 | 改坏的写法 | 改前（`05423d08`） | 改后（`bc1a24e5` 的代码） |
|---|---|---|---|
| ra01（A5） | 批准不看 `mounted()` | 活 | 死：approved once the page is left… |
| ra04 | 审阅一节的 `controllers` 换成另一个 `Set` | 活 | 死：the report's More under way, the page left… |
| ra05 | 退回开始时不清 | 活 | 死：…then sent back again, the network failing（第一轮判作"照 N1 改了之后等价"，不是：补了这一格） |
| ra10、ra11 | 驳回、退回不先显示答复 | 活 | 死：rejected / sent back, the page shows the … answered while it reads … again |
| c21 | 不看重读的 `error` | 活 | 死：…another admin approved it meanwhile（REVIEWC-3） |
| A1 | `explain` 按此刻显示的升级 | 是当时的实现（A1、REVIEWC-1 复现） | 死：refused, then its report kept… |
| A2 | `change` 不等重读 | 是当时的实现（A2、REVIEWC-2 复现） | 死：…the read again held |
| A3 | 计时器不看 `submitted_at` | 缺的行为（A3、REVIEWC-4 复现） | 死：…the new submission still checked |
| A4 | 批准之后不清缓存 | 缺的行为（A4 复现） | 死：approved, then Back… |
| A6、A6b | 别人退回不清检查；回到 `upgrading` 一律清 | 缺的行为（A6 复现） | 死：…sent back by another admin…（决定 300）；`upgrade-check` 的两格 |
| H1、H2 | 健康度页不交 `treeFailed`；`readingTree` 不看 `tree.error` | 缺的行为（REVIEWC-H 复现） | 死：tree failure has its own retry… |
| `leak`、`leak-after-unmount` | B 的写法；只在卸下之后才读 | `leak` 在 vitest 里死（挂着时就多读）、e2e 里活 | 死：each connection reads…；approved, …nor by a connection after it |
| `leak`（e2e，`make build` 之后） | 同上 | UP7、UP8 页面版本都过 | UP8 与 UP7 三种浏览器 4 个全红（`not.toContain(GET …/upgrade)`） |
| o27（e2e） | 批准之后重读升级 | 4 个全红 | 4 个全红（`apiFailures` 多一条 404） |

**修复核对一轮的**（规格与结果在 `p3s3/review/a/check1/`、`b/check1/`、`c/check1/`；都在头 `bc1a24e5` 上，先在没改的树上跑过）：A 的 18 条（四个测试文件，基线 162 个）：ra01b、ra02、ra04、ra05b、ra06、ra07、ra09b、ra10、ra11、rb01–rb03、rb05–rb07 死；rb04（决定 300 只看显示过报告，不看换过提交）活，成了 P3S3A-N6；rb08（计时器到点不清 `waiting`）活，等价（到点之后 `due` 不变就不再挂，只有读出错又好了才会再挂一次，无害）；两条只跑探针 A10 的试验（不清缓存、只在 `!mounted()` 时清）说明闪来自清缓存（P3S3A-N5）。A 的探针 A8（P3S3A-M3）在头上通过（对话框只出现过"正在计算"）。B 的 7 条（六个测试文件，基线 280 个）：b06、b07、b08、b12 照新代码重写，改前活、改后死；`leak` 死；`leakA`（批准跳转之后、重连时读升级）改前活、改后死在带事件流的"approved"一格；`leakM`（批准跳转之后、`mode` 时读升级）改后死在同一格；e2e 的 `leak` 改后 UP7 三种浏览器与 UP8 4 个全红。C 的 6 条（k1 `explain` 按此刻、k2 `change` 不等重读、k3 c21、k4 计时器不看 `submitted_at`、k5 健康度页不交 `treeFailed`、k6 `readingTree` 不看 `tree.error`）都死；探针 REVIEWC-1–4、H 在头上都过，把 `upgrade-page.tsx` 换回 `05423d08` 的时 REVIEWC-1、2、4 失败。

**修复二轮的**（实施者以 `mut.py` 经 `heavy.sh` 跑同一组命令，改前把 `bc1a24e5` 的页面与测试放进树里跑、基线 280 个，改后基线 283 个；跑完从副本放回、`cmp` 相同；规格与结果在 `p3s3/fix2/`）：

| 变异 | 改坏的写法 | 改前（`bc1a24e5`） | 改后（`a25fef6d`） |
|---|---|---|---|
| m3-no-read-once-more | 不看 `isValidating`、不再读 | 是当时的实现（A 的 A8 原样在头上过；新格放到旧页面上红） | 死：…its read again overtaken… |
| a8-as-it-was | 新格的期望改回 A8 原样（只出现 `stillChecked`） | 过（即 P3S3A-M3） | 死：同上 |
| n5-clear-once-gone-to | 照修复一轮：`await navigate` 之后在 `approve` 里清 | 是当时的实现（A10 失败；新格放到旧页面上红） | 死：…goes without showing the upgrade loading… |
| n5-clear-before-going | 在 `approve` 里、跳转之前清 | 活 | 死：同上 |
| n5-no-clear | 不记 `approvedAway`，从不清 | — | 死：approved, then Back… |
| rb04 | 决定 300 只看显示过报告 | 活（P3S3A-N6） | 死：…taken over and submitted again… (decision 300) |

**修复核对二轮的**（A，`p3s3/review/a/check2/`，基线 165 个）：探针 A8 原样在新头上失败（对话框只出现"状态不允许"），A10 通过；rb04、m3x（不再读一次）、n5x（清缓存放回跳转之后）、n5y（不标记 `approvedAway`）都死。新的探针 A11：先发 `mode`、挡住它的读，再退回、断网，请求记录多一次 `GET upgrade`（P3S3A-N7）。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `bc1a24e5`（修复一轮 2 个提交），三位各在自己的工作树里；C 另核文档稿（`try.diff`，在 `05423d08` 的副本上起草） | A：P3S3A-I1、M1、M2、N1–N4 关上（A1–A6 在头上照新的期望过），决定 300 照判法做了、对；修复的偏离 1（`change` 等重读）可以接受，受影响的是六个对话框（放弃、接着做、结束回滚、提交、驳回、退回），批准不等；偏离 3（决定 300 按这一次进入读到的算）对；新的 P3S3A-M3（Minor：拒绝之后的重读被 `mode` 事件的读超过时，SWR 丢掉先发的那一个的答复，`catch` 读到的缓存仍是旧的"审阅中、没有报告"，对话框一直写"正在计算"；探针 A8）、N5（Nit：react-router 以 `startTransition` 提交新的路由，`await navigate` 回来时升级页还挂着，清缓存让它闪成"加载中"；探针 A10）、N6（Nit：决定 300 的"换过提交"一支没有测试，rb04 活）。B：P3S3B-I1、M1–M3、N1、N2 关上；`leak`、`leakA`、`leakM` 改后都死；头上 UP6–UP8 页面版本三次各 `--repeat-each=3`，45 个全过，UP7 Chromium、Firefox、WebKit 各 9 次；`a040dcc1` 单独 `make build`、`lint-web`、`knip`、`test-web`（4,531 个）、`gen-check` 与升级故事加 LW8（107 个）都绿；新测试没有同类的空断言（放开都在批准的那个文档里，`letInAtHome` 先等到 `GET /api/v0/workspaces` 再等第四层）；没有新发现。C：P3S3C-M1、N1、N2、Q1（决定 301）关上，N3–N5 在文档稿里关上；决定 301 的后果（树再读失败时"更多"带来的节点也空着）对；文档稿的 3.25 取舍、`submitted_at` 的几处、决定 300、S2 之后第二次 `main` 的记法（抽查作业日志）、05-lessons 新的一条与两份移交核过；新的 P3S3C-N6（UP7 的说法没跟上修复）、N7（决定 301 与 4.3 只写了第一次读失败）、N8（4.3 只写了退回等重读，实际是所有经 `change` 的对话框）、N9（M12 第 41 条用了那份文件里没有的 `W/`），都是 Nit、都在文档稿 | P3S3A-M3、N5、N6 由修复二轮改（`fix-2.md`，一个 `web` 提交：重读被超过时再等一次；批准之后卸下时才清缓存；决定 300 的"换过提交"一格）；P3S3C-N6–N9 在本提交：3.25、第 5 节、10.3、13.2 第 9 条与 M5-events 一行照修复之后的 UP7、UP8 写，决定 301 与 4.3 加上树再读失败，4.3 写明六个对话框都等重读，第 41 条写完整的路径；没有 Important 留下 |
| 2 | `a25fef6d`（修复二轮 1 个提交），A 在自己的工作树里（B、C 在第一轮已没有留下的发现） | A：P3S3A-M3、N5、N6 关上（A8、A10、rb04 改前活、改后死，新加的 4 条变异都被抓到）。"再读一次"可靠：SWR 2.5.1 里被超过的读答了就返回、不改缓存，`isValidating` 由更新的读留着为真，再发的是最新的读；只剩第二个 `mode` 又晚于这次再读的更窄的一种。卸下时才清的 effect 依赖在这一页的一生里不变，只在卸下时跑，StrictMode 的模拟卸下在批准之前；跳转被打断时留到离开时清，那时已批准，清掉是对的。新的 P3S3A-N7（Nit：`upgrade-page.tsx:363` 的"再读一次"不看拒绝的码，断网、403、404、503 这些拒绝碰上一个在飞的读时也多发一次 `GET upgrade`，对话框晚一个来回显示；探针 A11） | P3S3A-N7 不改：只在断网而有读在飞时多读一次，无害。没有 Important、Minor 留下，核对到此为止 |

## 实测

- **vitest**：`5a303fde` 上 193 个文件、4,447 个测试；头上 193 个文件、4,521 个测试（多 74 个，没有新文件；逐个提交 4,462、4,521、4,521：`health-rule.test.tsx`、`upgrade-check.test.tsx` 的加项在 1 号，`upgrade-page.test.tsx`、`upgrade-banner.test.tsx`、`notebook.store.test.ts` 的加项与按文字的键逐个核对的 i18n 测试在 2 号）。`make test-web` 57–58 秒（逐个提交的三次），`make test` 里的 vitest 58.7 秒。改到的文件单独跑：`upgrade-page.test.tsx` 67 个、`upgrade-check.test.tsx` 38 个、`health-rule.test.tsx` 71 个、`notebook.store.test.ts` 104 个、`upgrade-banner.test.tsx` 14 个。审查 B 在没改的树上跑改到的 7 个文件，312 个、8 秒。
- **`make e2e`**：538 个、142 秒（Playwright 2.3 分钟），S2 末 529 个、136 秒；多的 9 个是 UP6、UP8 的页面版本，UP7 的页面版本在三种浏览器，UP7 的 API、MCP 版本的 Firefox、WebKit，本机合起来约 11 秒。
- **UP6–UP8 的页面版本单独三轮**（`tools/pages3.sh`，经 `heavy.sh`，`make build` 之后，每轮一次 `playwright test`，UP7 三种浏览器，共 5 个）：三轮都 5 个通过；墙钟 7.6、7.2、7.6 秒（Playwright 报 7.2、6.9、7.2 秒）；UP6 2.9、2.8、2.4 秒，UP7 Chromium 1.7、1.7、1.6 秒、Firefox 2.4、2.5、2.5 秒、WebKit 2.5、2.5、2.5 秒，UP8 1.0、1.0、0.95 秒。写故事时另跑过 UP6–UP8 的页面版本各三遍（15 个）与改过之后的 UP7 三种浏览器各三遍（9 个），都通过。审查 B 在头上以 `--repeat-each=5` 跑 25 个，全过、23 秒（UP6 2.3–2.9 秒，UP7 Chromium 1.4–1.7、Firefox 2.6–2.7、WebKit 2.7–3.3 秒，UP8 0.94–1.0 秒）。
- **修复一轮**（头 `bc1a24e5`）：vitest 193 个文件、4,531 个测试（多 10 个），`make test-web` 58–59 秒；`make e2e` 538 个、139 秒；UP6–UP8 的页面版本 `--repeat-each=3` 一次 15 个全过，另三轮各起一次 Playwright，每轮 5 个全过、6.6–7.4 秒（UP7 Chromium 1.5–1.7、Firefox 2.3–2.4、WebKit 2.4 秒，UP6 2.8–2.9 秒，UP8 1.0–1.1 秒），UP7 三种浏览器合起来 12 次都过。核对时审查 B 三次各 `--repeat-each=3`，45 个全过，UP7 Chromium 1.3–1.6、Firefox 2.4–3.7、WebKit 2.6–4.2 秒。
- **修复二轮**（头 `a25fef6d`）：vitest 193 个文件、4,534 个测试（多 3 个），`make test-web` 59 秒；新的三格连跑 15 次都过；UP6–UP8 的页面版本一轮 5 个全过，7.5 秒（UP7 Chromium 1.5、Firefox 2.3、WebKit 2.4 秒，UP6 2.7 秒，UP8 1.0 秒）。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**（`tools/percommit.sh`，整个经 `heavy.sh`：在一个干净的临时 worktree 里逐个 `git checkout --detach`、`git clean -fdx`（留 `node_modules`、`bin`），跑完删掉；放在根分区上，`/tmp` 是带配额的 tmpfs；日志 `percommit-final/`）：

  | 提交 | build | `lint-web` | `knip` | `test-web` | `gen-check` | 故事（`stories/upgrade/` 全部与 LW8） | 之后 `git status` |
  |---|---|---|---|---|---|---|---|
  | `4484ca7c` | 0 | 0 | 0 | 0（193 个文件、4,462 个测试，58 秒） | 0 | 0，98 个，32 秒 | 干净 |
  | `ee298a0a` | 0 | 0 | 0 | 0（193、4,521，58 秒） | 0 | 0，98 个，32 秒 | 干净 |
  | `05423d08` | 0 | 0 | 0 | 0（193、4,521，58 秒） | 0 | 0，107 个（多 UP6、UP8 的页面版本与 UP7 三个版本在 Firefox、WebKit），35 秒 | 干净 |

  没有一个提交改了 `server/`，所以不跑 `lint-go`、`test-go-short`。写代码时每个提交之前另在工作树里跑过 `tsc`、`oxfmt`、`make lint-web`、`make knip` 与改到的测试文件；两次重建之前的提交也照这样各跑过一次，都绿（`percommit/`、`old/`），3 号 `--amend` 之后另跑过一次（`percommit-c3b/`）。审查 B 在 1、2 号上各自检出另跑一轮 `make build`、`make lint-web`（含三个包的 `tsc --noEmit`）、`make knip`、`make test-web`、`stories/upgrade/` 全部与 LW8，都 rc=0、数相同（1、2 号上 UP7 还不在 `threeBrowsers` 里）。
- **建成时的头**（`05423d08`，`tools/gates.sh`：轻的一项直接跑，重的三项各经 `heavy.sh`、一次一个，每一项之前判 `df`（43–44G）；日志 `gates/`，`make test` 的日志 178 MB 已 `gzip`；开始与结束时工作树干净），一轮全绿：

  | 门禁 | 结果 | 用时 |
  |---|---|---|
  | `make lint gen-check knip build` | rc=0 | 8 秒 |
  | `heavy.sh make test` | rc=0（vitest 193 个文件、4,521 个测试 58.7 秒；Go 各包照跑，S3 没改服务端；批准的守卫六格中位数 1.15–1.29 s） | 3,652 秒 |
  | `heavy.sh make test-go-short` | rc=0 | 43 秒 |
  | `heavy.sh make e2e` | rc=0，538 个 | 142 秒 |

  vitest 的 22 行 `TimeoutNaNWarning` 在 S2 的门禁日志里同样有，不是本 Step 的。第二次重建之前，同样的门禁在 `4d2c5bba` 上刚跑完轻的一项就停下了，没有用它的结果。
- **修复一轮**（`fix1/`，各在干净的 worktree 里、经 `heavy.sh`）：`a040dcc1`、`bc1a24e5` 各自 `make build`、`lint-web`、`knip`、`test-web`（4,531 个，58 秒）、`gen-check` 都 rc=0，升级的全部故事与 LW8 107 个通过（36 秒）；改写之前的那一次核对在 `make test-web` 里红了一格（`health-page-lifecycle.test.tsx`，决定 301 的后果），改了那一格并进网页那一个。头 `bc1a24e5` 的门禁（每项之前 `df` 43G）：`make lint gen-check knip build` rc=0（12 秒）；经 `heavy.sh`：`make test-web` rc=0（59 秒，4,531 个），`make e2e` rc=0（139 秒，538 个）；照修复说明不跑整轮 `make test`（只改了网页与 e2e）。e2e 的两个变异之后 `bin/` 从没改的树重建。
- **修复二轮**（`fix2/`，干净的 worktree、经 `heavy.sh`）：`a25fef6d` 的 `make build`、`lint-web`、`knip`、`test-web`（4,534 个，59 秒）、`gen-check` 都 rc=0，升级的全部故事与 LW8 107 个通过（34 秒）；UP6–UP8 页面版本一轮全过；只改了网页的状态与缓存的时机，照修复说明不跑整轮 `make test`、`make e2e`。

CI（作业，括号里是 Test 一步；e2e 是 E2E 一步）：

| 作业 | 43169439216（建成，`05423d08`） | 43181567930（修复一轮，`bc1a24e5`） |
|---|---|---|
| server (race) | 15:00（14:35） | 14:58（14:35） |
| server (bootstrap-1) | 6:53（6:29），EPYC 7763 | 6:44（6:25），EPYC 7763 |
| server (bootstrap-2) | 5:09（4:52），EPYC 9V74 | 6:25（6:06），EPYC 7763 |
| server (bootstrap-3) | 4:48（4:30），EPYC 9V45 | 5:45（5:27），EPYC 9V74 |
| server (bootstrap-4) | 5:59（5:38），Xeon 8573C | 6:07（5:44），EPYC 9V74 |
| server (bootstrap-5) | 6:37（6:17），Xeon 8573C | 5:10（4:50），Xeon 6973P-C |
| server (bootstrap-6) | 6:53（6:35），EPYC 7763 | 6:34（6:13），EPYC 7763 |
| server (bootstrap-7) | 4:08（3:50），EPYC 9V45 | 4:22（4:02），EPYC 9V45 |
| server (bootstrap-8) | 4:55（4:31），EPYC 9V74 | 5:48（5:23），EPYC 7763 |
| server (costs-1) | 16:06（13:16），EPYC 9V74 | 21:09（17:37），EPYC 7763 |
| server (costs-2) | 25:04（24:43） | 17:55（17:31） |
| server (costs-3) | 11:12（10:54） | 18:02（17:38） |
| server (costs-lint) | 8:14（7:54） | 8:45（8:26） |
| server (guards-core) | 19:57（19:37） | 20:53（20:30） |
| server (guards-sources) | 16:38（16:19） | 14:52（14:32） |
| server (short) | 4:27（4:07） | 4:25（4:06） |
| web | 5:13（4:38） | 4:00（3:27） |
| e2e | 7:58（6:41） | 9:24（7:32） |
| image | 1:24 | 1:27 |

- 建成时一次全绿，19 项（`p3s3/ci.out`；CPU 照作业日志的"Runner CPU"一步，主持者的文档稿取），普通推送（新分支，`05423d080a511f7d6299d6dc5f18d5979a1b77a7`，rc=0），`ci-wait.sh` 等到第 1 次 attempt。八份的 Test 一步 389、292、270、338、377、395、230、271 秒，包 356.2、266.3、248.8、311.9、348.1、361.0、207.8、243.9 秒，都在 450 秒之下；S3 没有改服务端，各份与 S2 的差别来自机器（第三份这一次在 EPYC 9V45 上，不是 7763 的样本）。costs-1 在 EPYC 9V74 上，批准的守卫六格中位数 1,770、1,808、1,963、1,720、1,663、1,649 ms（界 3.9 s；第 298 条看 7763 上的数，这一次不在 7763 上）。e2e 一次过，538 个，作业 7:58、作业限 20 分钟（P3 设计 7.5），余约 12 分钟；S2 的分支是 9:12（529 个）、合并之后 8:49，这一次反而短，多半是机器的差别。L1 没有出。
- 修复一轮：普通推送（`05423d08..bc1a24e5`，快进），`ci-wait.sh bc1a24e54fc3ab181e730e6a1ccc528f04504ea1`：run 43181567930 的第 1 次 attempt 19 项一次全绿，没有重跑（`fix1/ci.out`；CPU 由主持者的文档稿从作业日志取）。八份的 Test 一步 385、366、327、344、290、373、242、323 秒，包 351.8、332.6、301.1、317.2、265.8、340.9、218.1、288.3 秒，都在 450 秒之下；服务端没有改，各份与建成时的差别来自机器。costs-1 这一次在 EPYC 7763 上，批准的守卫六格中位数 2,253、2,236、2,396、2,258、2,212、2,208 ms，都在 3.25 s 之下（第 298 条的条件看合并之后的 `main`）。e2e 一次过，9:24（E2E 7:32），538 个，L1 没有出。
- 修复二轮：普通推送（`bc1a24e5..a25fef6d`，快进），`ci-wait.sh a25fef6df036360fbe6b3f459263cc73abea7fc6`：run 43189150092 的第 1 次 attempt 19 项一次全绿，没有重跑（`fix2/ci.out`）。八份的 Test 一步 393（EPYC 9V74）、369（EPYC 7763）、324（9V74）、342（9V74）、349（9V74）、363（9V74）、232（EPYC 9V45）、311（7763）秒，包 358.6、335.7、298.6、316.3、315.9、331.2、209.1、277.9 秒，都在 450 秒之下。costs-1 在 EPYC 7763 上 21:31（Test 一步 17:57），批准的守卫六格中位数 2,300、2,300、2,366、2,297、2,250、2,214 ms，都在 3.25 s 之下。web 4:05（3:30），e2e 8:19（E2E 6:32），一次过，L1 没有出；race 9:43，short 3:31，costs-2 17:48，costs-3 11:41，costs-lint 13:02，guards-core 13:51，guards-sources 17:03，image 1:28。

S3 开工之前 `main` 的两次 CI（详见 [P3 设计](../03-P3-review-approve-playbook.md) 10.2 的"合并之后"）：S2 合并的 `7d2e895a`（run 43149201273）与记合并号的 `5a303fde`（run 43153150986）都一次全绿；两次的八份平均都在 450 秒之下，第三份 403.5、第四份 400 秒最贴近；costs-1 两次在 Xeon 8573C、8370C 上，批准的守卫六格中位数 1.67–1.74、2.23–2.50 s，都在 3.25 s 之下，第 298 条不触发、界不改。S3 从 `5a303fde` 开出。

## 环境

实施者整轮根分区约 43–44G 可用，没有修剪任何缓存；逐个提交的临时 worktree 放在根分区上、跑完删掉；没有手动起容器（库都由测试经 testcontainers 起、收，e2e 的 PostgreSQL 由 `global-setup` 起停）；大的日志已 `gzip`（门禁的 `gates/`）。三位审查者：A、C 没有跑重的，只直接跑单个 vitest 文件；B 的重的都经 `heavy.sh`，每次之前看 `df`（43G），改坏的都从 `.mutbak` 复原，没有自己起容器；A 的两个临时测试文件、C 的探针跑完复原；三个工作树最后都回到 `05423d08`、`git status` 干净、没有 `*.mutbak`。修复一轮：每项之前判 `df`（43G）；逐个提交的 worktree 干净、跑完删掉；`git status` 干净，没有 `*.mutbak`；e2e 的变异之后 `bin/` 从没改的树重建；大的日志已 `gzip`；普通推送，没有改写已推的历史。修复核对一轮：三个工作树最后都在 `bc1a24e5`、`git status` 干净、没有 `*.mutbak`；B 的 `GOTMPDIR` 已删、e2e 变异之后重建了 `bin/`，C 换回旧代码跑探针之后复原。修复二轮：每项之前判 `df`；`git status` 干净，没有 `*.mutbak`；改前那一轮放进树里的 `bc1a24e5` 的两个文件跑完从副本放回、`cmp` 相同；临时的 worktree 已删；普通推送。修复核对二轮：A 的工作树在 `a25fef6d`、`git status` 干净、没有 `*.mutbak`，临时文件已删。

合并之后 `main` 的第一次 CI：run 43199957538（`07abd0f7`）19 项第一次 attempt 全绿；八份的 Test 一步 336（EPYC 9V74）、367（EPYC 7763）、380（Xeon 8573C）、419（7763）、302（9V74）、273（EPYC 9V45）、238（9V45）、315（7763）秒，都在 450 之下；costs-1 在 9V45 上 12:51（Test 一步 10:18），批准的守卫六格中位数 1.43–1.57 s（第 298 条不触发）；e2e 6:52（E2E 5:38）。
