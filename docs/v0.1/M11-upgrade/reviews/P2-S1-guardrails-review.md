# M11/P2 S1（护栏，服务端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p2-s1`（实施的 6 个提交：移动与改名必须改写链接（`Step.KeepLinks`、`page.upgrade_keep_links`）`54b1be8b`，基线页只许改 frontmatter 与账本（`domain.UpgradeBody`、`StepContent.BodyHash` 与 `BeforeHash`、`UpgradeBaselineContent`、`page.upgrade_immutable`、`AcceptUpgradeContent` 核对写之前的哈希）`8ad3fe43`，升级中盲写计进读的界与单元比计划时的状态（第 266 条）`937a257c`，`batch` 的折进（`Folded`、`page.upgrade_batch_rewrite`、正文的两个码标在 `Last`）`f6de159e`，护栏的两个成本守卫 `90134668`，e2e 的 UP4 `f4d88dd4`；推送之前改写过一次 4 号，5、6 号原样重放；从 `main` 的 `c37a3faf` 开始，审查时在 `f4d88dd4` 上，36 个文件、加 2,317 行、减 143 行。修复一轮四个：P2S1A-I1、P2S1C-M1（决定 268）`060b365a`，P2S1A-N1、P2S1A-N2、P2S1B-N1 `8d739bbd`，P2S1B-M1（决定 269 与第 267 条的提前）`80da6b1f`，P2S1C-N1 `bff70ffc`（16 个文件、加 300 行、减 142 行）；修复第二轮两个：P2S1A-M1（决定 270）`0b016421`，P2S1C-N4、P2S1C-N5 `198b0cd5`；一共 12 个；合并 `（待填）`），对照 [02-P2-guardrails-gate-submit.md](../02-P2-guardrails-gate-submit.md) 第 0 节的 S1、2.1、2.3（尤其第 254、265、266 条）、3.1–3.6、第 5 节的 UP4、6.1、6.4、7.1、7.3 的 S1 各行（1–24、84）、7.5、7.6 与第 8 节，[设计的审查记录](P2-design-review.md)，[00-M11-design.md](../00-M11-design.md) 4.6、4.16–4.18、第 9 节与第 11 节（第 190、191、208、210、236、240、243、244 条与 P2 的 254–267），[P1 设计](../01-P1-freeze-start-rollback.md) 3.25–3.28、10.1–10.4 与 P1 四个 Step 的审查记录，[v0.1 总体设计](../../v0.1-design.md) 第 13 节，02-workflow 3.2–3.3 的检查单，05-lessons，实施者的报告（提交、逐个提交的核对、门禁、变异的表、实测、CI、14 条偏离、10.1 与总体设计的句子的草稿） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（P1 时的三个工作树挪到 `f4d88dd4`），重的都经 `heavy.sh`：行为的正确性（A：守卫的分支、`body`、`Folded` 与码序、冻结在最前；单元的 `BodyHash`、`BeforeHash`、`keepLinks`、`differs(…, "state")`；账本；批的计划的 `Folded`、`rewrote`、`CheckReads`、正文的两个码标在 `Last`；3.4 的表与 g1–g22 逐格；一个临时的组合根探针三格；两个变异），SQL、成本与守卫（B：两条新语句的语义、并发与计划，两个成本守卫，第 266 条的上界，`Makefile` 与 `ci.yml`；账本的写在并发下的临时测试三格；读的守卫上 7 个改坏生成的 SQL 的变异、单元的守卫上 1 个；run 42530568840 到 42770402937 的 23 次运行里 guards-core 与 costs-2 的作业日志），测试、MCP 与提交（C：码表三处在答出新码的那个提交里都齐、三个码只在 MCP、UP4 对 00 号文档第 9 节；`54b1be8b`、`937a257c`、`f6de159e` 三个提交各自 build、vet、`make lint-go`、short、page 模块与竞态下的组合根六个测试；第七份的归类；7 种改坏、一个临时测试）。修复核对一轮三位各在自己的工作树里核对 `bff70ffc`（A：临时组合根探针第二版七格，试第 268 条能否绕过、两种拆法，4 个变异；B：`ci.yml` 两处时限与注释的数从日志与 API 重算，本机第七份一轮与归类，`060b365a`、`8d739bbd` 上的 build、vet 与 short；C：逐个提交与 UP4，照新代码重跑 `mv`，加 `ra`、`rw`、`rg` 与关掉字节不变那一处的两条，两个临时测试，核对留给文档提交的清单）；修复核对第二轮三位各在自己的工作树里核对 `198b0cd5`（A：临时组合根探针第三版，重跑 probe2 的两种形状，试改名文件夹之后放回、改名再改回原名、写不带链接的字节再加回、frontmatter 里的链接、P 自己移动之后放回相对链接与拆法，3 个变异只跑 g27；B：新的读的成本，两个临时测试量一般的最大规模与 M9 已接受的"许多次改名之间的带链接的页"，本机第七份一轮与归类，CI 42819115296 各作业对时限与 runner，`0b016421` 上的 build、vet 与 short，`TestTheMCPCallStopsWhenTheClientGoes` 那次失败；C：两个提交逐个跑，重跑 `ra`，把文档稿 `try.diff` 打到 `198b0cd5` 的副本上，逐行对照 `check1-c.md` 第 4 节的三张表） |
| 日期 | 2026-11-10（实施、审查、两轮修复与修复核对） |
| 结论 | Important 1：P2S1A-I1：一个 `batch` 先改名、再以原来的正文写一页链向它的基线页，护栏放过；那一页的链接断了，或静静地指向改名之后另建的同名页，闸门 6、7 都看不出（设计 3.4 的表第 5 行本身错）。Minor 2：P2S1B-M1：guards-core 的 30 分钟（与 S2 之前 costs-2 的 35 分钟）已护不住"卡住时 go test 的默认超时先到"；P2S1C-M1：编辑的路径上"留着 `Folded`"没有测试钉住，赋值的写法活过全部测试。Nit 6：重新计划的测试只钉住"没有升级 → `upgrading`"（P2S1A-N1）；g14、g15 成功的一半没断言写下了什么（P2S1A-N2）；形状 a 的界量不到护栏加的那一份（P2S1B-N1）；UP4 的版本号从库里拼（P2S1C-N1）；只写名字的链接移动之后照样解析，设计没写全（P2S1C-N2）；`90134668` 的范围漏了 `build`、`ci`（P2S1C-N3）。实施者的 14 条偏离三位各在自己的范围里判过，核对过的都对（第 8、10 条没有人提出异议），第 14 条由 P2S1C-N1 改回。修复一轮四个提交（`fix-1.md`，决定 268、269、第 267 条提前），实施者另加了说明里没写的一处：升级中带 `Reached` 的字节不变的写也过守卫（`Unit.guarded`），三位都判它必要、不误拒。修复核对一轮：旧发现里要改代码的都已关闭，P2S1C-N2、N3 的文档部分在本提交；另有 Minor 2（P2S1A-M1：链接先拿掉、改名、再放回，绕过第 268 条；P2S1C-M2：留给文档提交的清单没覆盖第 268 条要改的全部地方）、Nit 2（P2S1C-N4、N5）；B 没有新发现，另建议 guards-sources 下次改到时提到 40 分钟（推后）。修复第二轮两个提交（`fix-2.md`，决定 270）；P2S1C-M2 由本提交关上。修复核对第二轮：A 判 P2S1A-M1 关，第 270 条的判法范围对、不偏严，没有新发现；C 判 P2S1C-N4、N5 关，P2S1C-M2 部分（文档稿里 g21 的拆法与 00 号文档第 9 节一处旧判据），另报文档稿的 Nit 3（P2S1C-N6–N8），都由本提交改了；B 判第一轮的 M1、N1 仍关着、报告的数相符，新的 Minor 1（P2S1B-M2：第 270 条让 M9 已接受的一个形状在升级中计划翻倍，代价没写出），由决定 271 关上；另建议 `mcp_test.go` 的两个日志等待从 2 秒改成 10 秒（推后到 P2/S2）。之后不再有修复轮 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：P2S1A-I1、N1、N2，P2S1B-M1、N1，P2S1C-M1、N1–N3；修复核对一轮的新发现接着编：P2S1A-M1，P2S1C-M2、N4、N5；第二轮的：P2S1B-M2，P2S1C-N6–N8。修复一轮照主持者的修复说明（`p2s1/review/fix-1.md`）分四个提交，第二轮照 `fix-2.md` 分两个。下面的代码位置：审查时的发现是 `f4d88dd4` 的，修复核对一轮的是 `bff70ffc` 的；路径相对 `server/internal/modules/`，`bootstrap/` 在 `server/internal/` 之下） | — |
| P2S1A-I1 | 高（护栏被绕过） | 守卫比的是"写之后的正文"与"库里现在的正文"（`page/app/freeze_guard.go:120`：相同就放过，不看 `Folded`）；在一个 `batch` 里改名的改写还没有写进库。agent 在改名之后以原来的正文写链向它的基线页 P（只加 frontmatter，正文照批前读到的写）：计划里写的不是改写的原样，`Folded` 为假（`page/domain/batch_contents.go:91-95` 清掉它）；守卫放过；推后的改写只跟这一页最后一个正文操作之后的移动（`page/app/apply_batch.go:544`），不跟这次改名。结果 P 的字节不变，链接留在旧的名字上，正是 00 号文档 4.6 立 `page.upgrade_keep_links` 要挡的事。分开调用与单个 `move` 带 `update_links: false` 都被拒，只有 `batch` 漏过；g21 那种被拒为 `immutable` 之后，agent 最自然的改法（把正文写回原样）恰好过。A 的探针（`TestZZRevAProbe`，`a/probe1.log`）：一个 `batch` 改名 A、写 P 为新的 frontmatter 加原来的正文，成功，P 的链接 `A -> <none>`；另写一页新的 `A.md` 再写 P，成功，链接 `A -> A`，指向新页，闸门 6、7 都看不出。编辑同样走得到：改名之后把折进的 `[[Map]]` 编辑回 `[[A]]`。设计 3.4 的表第 5 行"放过"是 A 自己在设计修订核对第一轮（P2A-N6）建议的文字，当时只纠正了码 | 已改（`060b365a`，提交 1；决定 268，下面"主持者的决定"）：计划的 `PlannedPage.Folded` 改名为粘住的 `Reached`（写的路径 `holdsAt` 的 `rewrote` 为真就设，删掉清除；编辑的路径照旧只设不清）；`ContentWrite.Reached` → `StepContent.Reached`；守卫先判缺陷、读基线行（不是基线页放过）、再看标记（答 `page.upgrade_batch_rewrite`）、最后比正文；码的英文文字改成写下的正文不带改写时也成立的说法。实施者另查出说明没写的一处：写下的字节与库里相同（写回 P 原来的字节，或 g25 的编辑改回）时 `writeContent` 在第一个分支就返回，没有正文步、守卫根本不跑，只改守卫的次序挡不住；所以升级中带 `Reached` 的这种写照一次写去过单元的守卫、什么也不写，守卫的准备与判定从 `applyStep` 抽成 `Unit.guarded`（`unit_content.go:80-89`、`unit.go:432-450`）。测试：`TestThePlanTellsAFoldedRewrite` 改名 `TestThePlanTellsAPageAMoveReached`，表的第 5、6、13 行改为带标记，加 A 的三种（写改写的原样、写别的、写 P 自己的字节）；`TestTheFreezeGuard` 加"什么也没改"一格、每格带与不带标记；新的 `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds` 四格；g21 改答 `page.upgrade_batch_rewrite`，加 g23（断链：新的 frontmatter 加原来的正文、P 原来的字节；拆成先写后移动做完）、g24（指向别处）、g25（编辑改回），拒绝的格都经 `refused` 核对笔记本的行与账本不变。变异 n1–n7 与第 17、18、20 条都抓到（设计 7.3）。修复核对一轮：A 关（探针第二版重跑 I1 答 `batch_rewrite`；改名文件夹、P 自己移动、同一页写多次、删除之后重建、带 `Merged` 的写、不是基线页的页都挡住或不在范围里；两种拆法都过，P 被改写成 `[[Map]]`），另报出 P2S1A-M1；三位都判 `Unit.guarded` 那一处必要、不误拒，造出的步与真的写一致 |
| P2S1B-M1 | 中（CI 的时限） | `.github/workflows/ci.yml:95-102` guards-core 那段注释给 30 分钟的理由是"照约两倍的规则……卡住时 go test 的时限仍先到"：这一份的每行 `go test` 不带 `-timeout`，挂住时由 Go 默认的每个测试二进制 10 分钟打出卡住的测试，作业的时限要晚于它。B 读 23 次运行的作业日志：guards-core 在 EPYC 7763 上九次作业 18.5–20.0 分钟，末行在作业开始之后 18.1–19.5 分钟开始；S1 的分支上新的一行在 18.7 分钟开始、最后一行在 19.4 分钟，作业 19.8 分钟，挂住时约 29.4 分钟之后才打出，离 30 分钟只有约半分钟。作业与时限之比约 1.5，"约两倍"在 S1 之前就不成立，S1 再加约 0.6 分钟。costs-2 同样：7763 上 `main` 的七次作业 22.2–24.3 分钟，S1 的新行接在末尾、约在第 23–24.3 分钟开始，挂住时约 33–34.3 分钟才打出，时限 35 分钟；第 267 条把它提到 45 分钟，但那是 S2 的第 8 个提交，S1 先合并 | 已改（`80da6b1f`，提交 3，范围 `ci`）：guards-core 30 → 40 分钟（决定 269），costs-2 35 → 45 分钟（第 267 条提前到 S1）；注释各一段，带 run 号、作业用时与 runner，写明照约两倍的规则、go test 的默认超时要先于作业时限到，以及 00-M11 第 267、269 条。修复核对一轮：B 关：注释里的数从日志与 API 重算都相符（costs-2 在 7763 上 22.2、23.2、23.7，最长 42713408119 日志 24.3、API 24.4 分钟；guards-core 九次 18.5–20.0 或 18.6–20.1 分钟，末行 18.1–19.5 分钟）；改后余量足：guards-core 末行在第 19.2 分钟开始、卡住时约第 29.2 分钟打出，costs-2 卡住时约第 34 分钟打出；修复的 run 里 costs-2 落在 EPYC 9V74 上 22.0 分钟（上一次同种 runner 16.6 分钟），与 S1 无关的 linking 几行也慢了 1.22–1.33 倍，同一种 runner 的实例慢了约三成，套到 7763 上两处也仍成立 |
| P2S1C-M1 | 中（测试） | `page/domain/batch_contents.go:107-109` 编辑的路径 `if rewritten != content { pg.folded = true }` 只设不清，写法对；但改成赋值 `pg.folded = rewritten != content` 的变异 `mv` 在 `TestThePlanTellsAFoldedRewrite`、`go test ./internal/modules/page/...` 与组合根的 `TestUpgradeGuardrails`、`TestUpgradeToolsErrorsCarryTheirCodes` 上都活下来。形状：移动 A（P 链向 A）、编辑 P 的 frontmatter（A 的改写折进）、移动 B（P 不链向 B）、再编辑 P；变异让第四步清掉 `Folded`，守卫答 `page.upgrade_immutable`，agent 没有改正文却被告知只许改 frontmatter，拆法也不对。表的 `moved int` 只容一个 `Moved` 的操作（`:409`）。C 的临时测试照这个形状断言 `Folded` 为真：没改的树上过，变异时失败 | 已改（`060b365a`，提交 1）：第 268 条之后标记只设不清；表的 `moved` 改成 `[]int`，加 C 的一行"an edit reached, a move P links to none of, another edit"（标记为真、`Moved` 是 op 1 与 op 3）。修复核对一轮：C 关：`mv` 照新代码改写成 `pg.reached = rewritten != content`，被 `TestThePlanTellsAPageAMoveReached`（失败在上面那一行）、`page/domain` 全包与 C 原来的复现三处抓到，未变异的树上三者都过 |
| P2S1A-N1 | 细节（测试） | `page/app/apply_batch.go:568` 比的是 `State` 本身，对；但 `TestTheBatchPlansAgainOnceTheUpgradeStarted`（`apply_batch_test.go:608`）只钉住"没有升级 → `upgrading`"。A 的 MA1（只比有没有升级）与 MA2（只在"计划没有升级、单元 `upgrading`"时重新计划）都活下来。它们漏掉的是"计划时 `reviewing`、单元时回到 `upgrading`"：发起人的 agent 在提交的计算中发 `batch`，计划读到 `reviewing`；提交答 `not_ready` 退回 `upgrading`；之后单元才拿到笔记本行。这时盲写不计，守卫却整读，与 P2A-N8 的情形一样 | 已改（`8d739bbd`，提交 2）：加两格，计划时是同一个升级的 `reviewing`、`LockNotebook` 时改成 `upgrading`，过界答 `ops[2]` 的 `page.batch_too_large`、什么也没写，界内写下，都计划两次。修复核对一轮：A 关，MA1、MA2 都被这两格抓到 |
| P2S1A-N2 | 细节（测试） | `bootstrap/upgrade_guardrails_test.go:143-154`：g14、g15 成功的那一半只断言 `g.accepted`（账本等于库里的 `content_hash`），写若根本没落下，两者仍都是发起时的值。设计 3.6 写"`done` 的格断言写下了什么"；g1、g4、g10、g21 都有 `holds` | 已改（`8d739bbd`）：各加一句 `g.holds`。变异 an2（升级中非改写的写一律当作字节不变、什么也不写）由这两句抓到：g14 只有 `holds` 能抓；g15 失败在 `holds`，其后 Other 的拒绝也变成了成功。修复核对一轮：A 关 |
| P2S1B-N1 | 细节（成本守卫） | `bootstrap/upgrade_guardrails_cost_test.go:21-36`：形状 a 的界 5.2 s 是本机较大的中位数 1,712 ms 的三倍，但它的单元里护栏只加 400 次账本的写：本机普通笔记本与升级中 1,746 / 1,807 ms，7763 上 3,054 / 3,165 ms，护栏的那一份约 60–110 ms，其余是 M9 的 400 页改写（`TestABatchAtItsRewriteBoundTakesItsTime` 以 11 s 守着）。账本的写即使慢到每次 5 ms，本机仍在界内，所以这条界抓的是 M9 的回归。同一个改写在 CI 上：7763 十次 3.12–3.41 s，最慢一次对 5.2 s 是 1.52 倍；哪天来了更慢的 runner，最先红的是 a，原因与护栏无关 | 已改（`8d739bbd`）：形状 a 前加注释，写明这条界守的主要是 M9 的改写，护栏多出的约 15–65 ms 在本机的抖动之内，护栏的那一份由 b–e 量；界与形状不变。修复核对一轮：B 关；注释里"15 to 65 ms"是本机的数，7763 上升级中减普通笔记本是 79 ms（这次）与 111 ms（上一轮），同一个改写十次的散布约 290 ms，"在抖动之内"在 7763 上同样成立 |
| P2S1C-N1 | 细节（e2e） | `e2e/stories/upgrade/up4-guardrails.spec.ts:64-71` 的 `pageNow` 以 `page_contents` 的 `revision` 拼基线页的 `base_version`（实施报告偏离 14）。00 号文档第 9 节的 e2e 总则写"值来自答复"；`createPage` 的答复带 `revision`，别的故事就这样写（`lw10-virtual.spec.ts:186`）；Plan、Old、Broken、Other 在被写之前都没有别的写 | 已改（`bff70ffc`，提交 4）：`versionOf(<createPage 的答复>)`，Draft 用写的答复里的 `draft.version`；Old 的正文没有链接、改名只改写 Notes，注释写明；库只为断言读（`contentNow`、`rows`、`state()`）。修复核对一轮：C 关，本机 `make build` 之后 UP4 三个都过 |
| P2S1C-N2 | 细节（文档） | 设计第 5 节 UP4 一行写"`move` Plan 进一个新的文件夹（成功，Notes 被系统改写）"：按名字的链接（`[[Plan]]`、`Plan.md`）在 Plan 移进文件夹之后照样解析到它（`linking/domain/rewrite.go:398-417` 的 `markdownTarget` 先试 `Written(n, …)`），系统不改写，只有从根起或相对的路径会被改写，改名则两种都改写。实施把 Notes 写成 `[the plan](/Plan.md)`，对。00 号文档 4.6、决定 190 与第 9 节都没说"移动改写每一条链接"，没有写错的地方；但第 5 节照字面写不出"被系统改写"，playbook 不讲这一点，agent 会以为移进文件夹会改它的 `[[…]]` | 本提交：设计第 5 节的 UP4 一行与第 9 节 P3/S2 的 playbook 各补一句"只写名字的链接，移动之后照样解析，系统不改写；'移动之后系统改写链接'只对写了路径的链接成立"；3.19 写明。修复核对一轮：C 判"照计划留给文档提交" |
| P2S1C-N3 | 细节（提交） | `90134668 server: the guardrails' reads and units take their time (M11/P2/S1)` 改了 `Makefile` 与 `ci.yml`，范围只写 `server`（`docs/README.md:100` 的部件含 `build`）。P1/S3 的审查记下过同一件事（S3C-N1）；这次设计 6.1 第 5 条给的标题本身就只写 `server`，6.2 第 1、8 个提交同样改 `Makefile` 与 `ci.yml` | 只记下，已推的不改写；修复一轮的提交 3 范围写 `ci`。本提交把设计 6.1 第 5 条、6.2 第 1、8 条的范围写成 `server, build, ci`（6.2 第 1 条另有 `docs`）。修复核对一轮：C 判"部分"：四个新提交的范围都与它们改的文件相符，其余留给本提交 |
| P2S1A-M1 | 中（护栏被绕过，窄） | 修复核对一轮的新发现：标记只在"这一次正文操作之前的移动，改写了这一页当时（批里）的内容"时设（`page/domain/batch_contents.go:91`、`:105`），`holdsAt` 与编辑的路径都只改写 `pg.content`，不看批前的内容。一个 `batch` 先把 P 里的 `[[A]]` 编辑成 `(A)`、再改名 A→Map、再编辑回 `[[A]]`：改名的那一刻 P 不链向 A，标记不设；最终的字节与库里相同，守卫不跑；推后的改写也不跟。A 的探针（`a/probe2.log`）：成功，P 原样，链接 `A -> <none>`；改名之后另写 `A.md`，链接 `A -> A`（新页）。正文另有改动时也一样。后果同 I1，但要先拿掉、再在同一个批里放回，不是 agent 自然的写法，定 Minor | 已改（`0b016421`，修复第二轮提交 1；决定 270）：升级中（计划读到的状态是 `upgrading`），`CheckReads` 之后 `reachedBefore` 按各页最后一个正文操作的次序，对本批不新建、最后一个正文操作之前有移动的页，以现有的 `BatchLinks.Reached` 按批前的索引问一次（移动、冻结的节点、移动之后新建的节点与树都取到 `pg.Last`，树的读用 `readsAt(pg.Last)` 的缓存）；写的页只有一个正文操作、头已问过时用头的答案、不再读；够得着就 `plan.MarkReached`，在正文的一遍之前设上，之后照第 268 条粘住。测试：`TestTheBatchJudgesAPageByItsLinksBeforeIt` 十格（A 的两种形状够得着与够不着、头的页、头的页有后来的正文操作、普通笔记本数读）；`TestThePlanTellsAPageAMoveReached` 的 `MarkReached` 各格；g27（断链、指向别处、先放回后改名做完；另一本里基线页 Q 由本批在 frontmatter 加链接，正文一遍才标得上）。变异 x1–x4d 都抓到。修复核对第二轮：A 关：probe2 的两种形状重跑都答 `page.upgrade_batch_rewrite`（`op` 2、3），P 与链接都没动；改名文件夹之后放回、写不带链接的字节再加回、frontmatter 里的链接拿掉再放回、P 自己移动之后放回相对链接都被拒，改名再改回原名与拆开的写法都过、结果对 |
| P2S1C-M2 | 中（文档） | 修复核对一轮的新发现：`fix-1.md` 的"只改文档"一节与修复报告第 7 节合起来的清单，没有覆盖第 268 条要改的全部地方。C 在 `check1-c.md` 第 4 节列出三张表：P2 设计的第 0 节、2.1 第 2 条、2.2、2.3、3.1、3.3 的代码块与测试段、3.4 表以外的注释块、写的路径与"粗的判据"、3.6 的码文字与格表、3.18、6.1 第 4 条、7.1、7.5、第 8、9 节；00 号文档的 4.6 两处、第 5 节码表、第 3 节第 4 条、第 7 节的 Phase 表与第 30 条、第 8 节 page、第 9 节变异、第 11 节第 190、208、254、265 条；v0.1 5.8。照清单做完之后，本 M 的权威设计与 P2 设计 3.3、3.6 仍会与代码不符，13.1 第 1 条还会把旧的判据写进长期约定 | 本提交：三张表逐行改，另照第 270 条一并写（P2 设计 3.19 列出；00 号文档第 11 节第 190、208、254、265 条加注，第 7 节第 14 条记下 5.8 的改法）。修复核对第二轮：C 判部分：三张表的 23 + 13 + 1 行都落了，绝大多数对；没关上的两处另立 P2S1C-N6、N7，本提交改了 |
| P2S1C-N4 | 细节（注释） | 修复核对一轮的新发现：`page/app/unit_content.go:121-122` 的 `contentStep` 注释写"A no-op never enters apply or the edit lock"；现在升级中、被移动到达的字节不变的写经 `guarded` 走到编辑锁的 `GuardWrite` | 已改（`198b0cd5`，修复第二轮提交 2）：注释改为它由改了的写、契约仍适用的少数不改的写、以及升级中被同批移动够着的不改的写共用，最后一种经 `guarded` 见到全部守卫（编辑锁的也在内）、什么也不写；不改的写从不进 `apply`。修复核对第二轮：C 关，注释与 `:80-89` 的代码一致 |
| P2S1C-N5 | 细节（测试） | 修复核对一轮的新发现：组合根没有"不是基线页、被移动到达"的格。C 的变异 `ra`（`body` 在读基线行之前就看 `Reached`，凡被到达的页都拒，agent 自己新建的页也拒）在整个 `TestUpgradeGuardrails` 下活下来，只被 `TestTheFreezeGuard` 的替身抓到；而这一处新加的路径（字节不变的写在升级中也过三个守卫）在整个程序上还没有一格钉住"不是基线页照常做完" | 已改（`198b0cd5`）：g26，照 C 的临时测试：N 是 agent 升级以来写的页、链向 A，三种各在自己的笔记本里——`batch` 先把 A 移成 Map，再写 N 原来的字节、写别的字节、或编辑把改写放回——都做完，`holds` 断言 N 是 agent 写的样子，P（基线页、本批不写）由推后的改写写成 `[[Map]]`。`ra` 在 g26 上死。修复核对第二轮：C 关：未变异的树上 g26 通过，`ra` 在 g26 与 `TestTheFreezeGuard` 上都死 |
| P2S1B-M2 | 中（成本，窄） | 修复核对第二轮的新发现：`reachedBefore`（`page/app/apply_batch.go:445`、`:476-501`）对最后一个正文操作之前有移动的已有页各问一次 `BatchLinks.Reached`，每次在那一页自己的树上展开被移动的子树；计划一次只留一棵树的读（`readsAt`），正文的一遍再从头展开一次。一般的最大规模上很便宜（B 的临时测试：改名一页没人链向的页、再编辑 49 页满负载的页，升级中计划 222 ms，普通笔记本 182–186 ms，49 次问合计约 36–40 ms）；但 M9/P3 记下、接受并交给 M12 的"许多次改名之间的带链接的页"（50 次改名一个 10 万个节点的文件夹、每次之后编辑一页带链接的页，`M9-mcp/03-P3-batch-revert.md:994-1010`）在升级中计划翻倍：普通笔记本调用 6.12–6.37 s、计划 3.54–3.80 s，升级中调用 10.06、10.07 s、计划 7.47、7.52 s，单元不变（2.55–2.59 s）；照 7763 约是本机 1.85 倍估约 18.6 s，过了 15 秒的请求期限（未经 CI 核实）。过了时计划被截断、什么也不写，拆开即可。报告"界不动、量不到"对现有守卫是对的，但漏了这个代价 | 决定 271（只改文档，不改代码）：第 270 条在 00 号文档第 11 节与设计 3.19 里各加一句代价；设计第 9 节的 M12 一条写明这个数接在 M12 的性能移交里 M9 那一项之后；不另立守卫、不优化 |
| P2S1C-N6 | 细节（文档稿） | 修复核对第二轮的新发现：文档稿的设计 3.4"粗的判据"写"（g21）拆开（先移动，……；再写，或分两个 `batch`）就过"，括号里读起来是两种做法，前一种"同一个批里先移动再写"正是 g21 被拒的形状；g21 只有分两个 `batch` 这一种拆法（同一个批里先写后移动，写下的 `[[Map]]` 在移动之前就是正文变了，答 `page.upgrade_immutable`） | 本提交：改为"（g21）分两个 `batch`（先移动，系统以 `Rewrite` 改写、账本跟上；再写）就过" |
| P2S1C-N7 | 细节（文档稿） | 修复核对第二轮的新发现：00 号文档第 9 节 page 的 `domain` 一条（`:661`）仍写"批的计划的 `Folded`（……写的路径上 agent 的内容不等于改写的结果时不是）"，照第 268 条最后一句已不对，也缺第 270 条；C 第一轮的表漏列了它 | 本提交：改为 `Reached` 的判据，写进第 268、270 条 |
| P2S1C-N8 | 细节（文档稿） | 修复核对第二轮的新发现：文档稿的设计 10.1 有三处与别处对不上：偏离编成 1–13（形状 c 与 bench 的请求体上限合成一条），3.19 与本记录用 1–14；"审查 C 另在三个提交上跑同样的门禁"不准（C 在 S1 没跑 `make gen-check`，page 模块的测试不带竞态）；第二轮单元守卫的数是改订之前的 `1960105e` 的 | 本提交：10.1 的偏离改回 1–14、与 3.19 同号；C 跑的照本记录审查方式那一行写；单元守卫的数换成 `0b016421` 的 |
| 偏离 | — | 实施者的 14 条偏离（[02-P2](../02-P2-guardrails-gate-submit.md) 3.19、10.1）：1 单元守卫的期限 14 秒；2 形状 b 改写；3 `CheckReads` 一遍按 `p.order`；4 读的守卫三种统计 × 两种计划；5 g22 以 SQL 种下；6 `StepContent.Folded` 在 4 号加；7 sqlc 的表的别名；8 形状 c 是一个操作的 `batch`；9 批的 bench 给 `/mcp` 以 wire 的请求体上限；10 单元守卫的种子以 SQL 写回；11 读的守卫本机约 17 秒；12 多一个 `TestTheUnitStepCarriesKeepLinks`；13 UP4 的 Notes 以路径链向 Plan；14 UP4 读版本号用数据库 | 三位各在自己的范围里判过：A 核对 3（按操作的次序才与"at the op that passes them"一致，`TestTheReadsBoundCountsBlindWritesWhileUpgrading` 的"两个盲写之后的编辑在 `ops[2]` 被拒"分得出两种写法）、5、6、7、12、13；B 核对 1（三倍的界都 ≤ 5.2 s，比 14 秒紧得多，不会因此放过什么）、2（算术：头 14,502,300 ≤ 14,680,064，最终 8,301,900 ≤ 8,388,608，删掉的 6,200,400 ≤ 6,291,456，对字面的改写是必需的）、4（覆盖相同）、9（与 `wire.go:232` 同一个常量）、11、13；C 核对 3、9（bench 上 `/mcp` 的期限因而加上 `ReadTimeout`，与生产一致，已有的断言拿用时与 `RequestTimeout` 明比，不因此放松）、12（不算过度设计）、13（即 P2S1C-N2）；都对。14 即 P2S1C-N1，改回 |
| 留下的事 | — | 实施报告第 8 节留下的五项：1 Step 的文档提交；2 单元守卫在 7763 上余量最小的是形状 a（3,165 ms 对 5.2 s，约 1.64 倍），若之后的 runner 更慢，这一格先红；3 读的守卫的种子比估的重，costs-2 在 9V45 上 13:03，S2 提时限时一并看 7763 上的数；4 UP4 的 Notes 要以路径链向 Plan，P3/S2 写 playbook 时同样只对写了路径的链接成立；5 S2、S3 照设计 | 1 是本提交；2 即 P2S1B-N1，只加注释，记在"推后"；3 costs-2 的时限照第 267 条提前到 S1，读的守卫在 7763 上仍没有跑过（B 的未核实的怀疑 3）；4 即 P2S1C-N2，写进设计第 9 节；5 — |
| 修复一轮的说明 | — | 实施者在修复一轮的报告（`fix-1-report.md` 第 2、7 节）：1 第 268 条在实现上多了说明里没写的一处：字节不变的写（写回 P 原来的字节，或编辑把折进的改写改回）不产生正文步，守卫看不到，所以单元在升级中对带 `Reached` 的这种写也跑一次守卫、什么也不写；审查 A 说 g25 那种"守卫在看 `Folded` 之前就放过"，实际是守卫没有被调用；2 guards-sources 没改，数写在报告里（7763 上作业 16.7–16.9 分钟、末行在第 16.4–16.5 分钟，时限 30 分钟，约 1.8 倍；前六行带 `-timeout 10m`，后五行不带）；3 变异 an2 之下 g15 的失败一半来自 `holds`、一半来自 Other 的拒绝变成成功，g14 只由 `holds` 抓到；4 单元守卫的形状没有因 1 号改变（a–e 都不是"移动之后写同一页"），不另量 | 1 三位修复核对都判必要、不误拒（A：变异 MB1、MB2；B：沿代码走一遍"移动 A，再写 P 原来的字节"；C：关掉这一处带 `-v` 重跑，g23 失败在 `:276`、g25 在 `:290`，答复都是 `batch` 做完了），写进设计 3.3 与 3.19；2 B 判不算 S1 的发现，记在"推后"；3、4 B 同意 |
| 修复第二轮的说明 | — | 实施者在修复第二轮的报告（`fix-2-report.md` 第 3、5、8 节）：1 判法统一走索引：整读的页也有批前的索引（同一修订号），问法与写的页一样，不另写"批前正文经那些移动改写"的路径；头的问本来就用 `links.Reached`（M9/P3 A-I1），参数现成；索引不是当前修订或当前抽取器时 `Reached` 答否，与批的改写看的是同一份索引，护栏不比改写更松、也不更严；2 不加新 SQL：新的读就是 `BatchLinks.Reached`，它的计划由 costs-2 的 `TestLinksReachedReadPlans`、`TestLinksReachedDenseReadPlans` 守着；一页至多两次索引的读，整批不过 `MaxBatchOps` 减去移动数；3 成本守卫的界不动：M9 的批的守卫在普通笔记本里跑，升级中的那一格（形状 a 只有一次移动、没有正文操作，b–e 没有移动）一次也走不到新的读；4 复用头的答案的条件是 `First == Last`，`x2c`（问过就用）在第一版的测试上活下来，补了 `askedFirst` 两格；5 n4、20 在 g9、g25、g21、g23 上活下来，因为第 270 条的索引判据先把 P 标上；正文那一遍在升级中仍有用的情形是被够着的链接是本批加的、批前的索引里没有，只能在 frontmatter 里，所以 g27 加了 Q 的两格，核实过批内的改名会改写 frontmatter 里的属性链接 `see: "[[A]]"`；6 中途磁盘剩 14G 停下（"环境"）；7 头上第一次 `make test` 失败在第五份的 `TestTheMCPCallStopsWhenTheClientGoes`（`mcp_test.go:273`，2 秒之内没等到 debug 的那一行；七份组合根并行，5 分钟负载 7.73），单独 `-race -count=5` 都过，整个 `make test` 重跑全绿，实施者判是负载下的计时、与本轮无关 | 1–3 主持者认可（`fix-2.md` 把判法交给实施者，五条约束都守住）；4、5 写进设计 7.3；6 见"环境"；7 B 判与本轮无关（本轮没碰 MCP 的传输、断开与日志；CI 上第五份独占一台 runner），记在"推后"。修复核对第二轮：A 核对判法（范围正好到最后一个正文操作、`First == Last` 的复用、不偏严、新建的页不设、状态变了会重新计划），都对；B 判 3 对现有的守卫成立，但漏了 M9 已接受的一个形状上的代价，即 P2S1B-M2（决定 271） |

## 主持者的决定

负责人可以改判（268、269 与第 267 条的提前在第一轮的修复说明 `fix-1.md` 里定下，270 在第二轮的 `fix-2.md` 里定下，271 在修复核对第二轮之后定下；负责人 2026-11-06 的话照旧："调测试用例量级或者时限即可"）：

268. **一个 `batch` 里前面的改名或移动改写过一页基线页的链接，之后对这一页的正文操作一律答 `page.upgrade_batch_rewrite`，不论写下什么**（P2S1A-I1、P2S1C-M1，2026-11-10）：计划的标记改为粘住的"到达过"（`Reached`）：这一页任一个正文操作之前、在它上一个正文操作之后，有移动改写了它，就设上，之后不清；守卫先看标记、再比正文；字节不变的这种写也照一次写过守卫、什么也不写。拆法总是成立：先写这一页、再移动（推后的改写以 `Rewrite` 写，守卫不判），或者分两个 `batch`。它撤回第 254 条里"写的不是改写的原样就清掉"的一半，也收掉第 265 条说的"粗的判据"里 g21 那一种偏严的形状，规则更简单。理由：改写一旦够着这一页，之后的写不论写了什么，都可能让链接留在旧的名字上，而字节可能与库里相同，比正文看不出；拆开的写法对 agent 总是可行，码的文字也说得清。变异 n1–n7 与 7.3 第 17、18、20 条抓得到。
269. **guards-core 的作业时限从 30 分钟提到 40 分钟**（P2S1B-M1，2026-11-10）：B 的实测，7763 上作业约 20 分钟，最后一行 `go test` 在第 19.4 分钟才开始，卡住时 Go 默认的 10 分钟超时只比作业时限早约半分钟，打不出卡住的测试；"约两倍"的规则 S1 之前就不成立，S1 又加了约 0.6 分钟。理由：负责人 2026-11-06 的话，调时限。
270. **升级中判"这一页被够着过"，还要看这一页在 `batch` 之前的链接**（P2S1A-M1，2026-11-10，补第 268 条）：已有的一页，只要它最后一个正文操作之前，有带 `update_links` 的移动或改名照它在批前的链接会改写它，就设 `Reached`，不论这一页在批里当时的内容是什么。理由：改写的对象是批前就有的链接，agent 在批里先拿掉、再放回，链接仍要跟着改名走，而推后的改写只跟最后一个正文操作之后的移动；拆开的写法照旧总是可行；新的判据只在升级中多读，普通笔记本的 `batch` 的读不变。约束：只照计划时的状态在升级中做（状态变了由 `differs` 重新计划）；判出来交给计划，在正文的一遍之前设上；新的读满足成本守卫的约定。判法由实施者定（"修复第二轮的说明"）。变异 x1–x4d 抓得到。代价见第 271 条。
271. **第 270 条的代价照实记下，不改代码**（P2S1B-M2，2026-11-10）：第 270 条让 M9/P3 已接受、交给 M12 的"许多次改名之间的带链接的页"在升级中的计划约翻倍，本机 7.5 s 对 3.5–3.8 s，调用 10.1 s，照 7763 的比估约 18.6 s，过 15 秒的请求期限。过期限时什么也不写，拆开就能过。理由：负责人 2026-11-06 的话，不为它优化、不另立守卫；写进第 270 条的代价与 M12 的性能移交，在 M9 的那一项后面加这个数（设计第 9 节的 M12 一条，同第 262 条进 M12 的写法）。

**第 267 条提前到 S1**（P2S1B-M1，2026-11-10）：costs-2 的 35 → 45 分钟在 S1 的修复里改（`80da6b1f`），不等 S2 的第 8 个提交。依据同 B：7763 上 `main` 的作业 22–24 分钟（42530568840、42752100006、42762546769），最后几行在第 20 分钟之后才开始，S1 又在末尾加一行，与第 269 条同样的情形；S1 先合并，等到 S2 时 `main` 上这一份卡住时只剩约 1 分钟。设计 6.2 第 8 个提交原写"改 `timeout: 45`"，改为"S1 已改"。

修复一轮的处置照 `fix-1.md`：三份审查全部接受，一组一个提交、各自绿、接在 `f4d88dd4` 之后、不改写已推的历史；改了生产代码，头上跑全部门禁（`make lint gen-check knip build`，经 `heavy.sh` 的 `make test`、`make test-go-short`、`make e2e`）；审查者留下的变异都要杀死（A 的 MA1、MA2，C 的 `mv`）；只改文档的（P2S1C-N2、N3，设计 3.4、2.3、7.3、7.1 的"6 s"，00 号文档第 11 节加 268、269）留给 Step 的文档提交。第二轮照 `fix-2.md`：修复核对一轮的 P2S1A-M1、P2S1C-N4、N5 接受，两个提交、接在 `bff70ffc` 之后；P2S1C-M2 是文档的，在本提交；B 的 guards-sources 记作推后；文档提交另起一份说明，等第二轮的核对过了再定稿。修复核对第二轮之后：B 的 P2S1B-M2 由决定 271 关上，只改文档；C 的 P2S1C-N6–N8 是文档稿的，在本提交；不再有修复轮。

## 接受与推后的

- **接受**：
  - C 判可以接受、不另立发现的一处变化（修复核对一轮）：被冻结的人（不是发起人，或发起人已不是管理员）的 `batch`"先写或编辑 P，再一次够着 P 的移动，最后把 P 写回库里的字节"，以前单元在 op 0 什么也不写、在 op 1 的移动上答 `notebook.upgrading`，现在在 op 0 就答同一个码；两种都什么也没写，"当作一次写来判"本来就该标在单元写的那个操作上。
  - B 判不记作发现的一点（修复核对一轮）：`body` 先读基线行与正文、再看 `Reached`，带标记的基线页正文读出来也不用；这一读在第 266 条的上界之内。
  - B 判不记作发现的另一点：字节不变的那一支造的步，`Change.Revision` 是 `current+1` 而这一版并没有写下；守卫里没有谁读它（冻结、编辑锁都不看），B 逐个看过。
  - B 审查时的未核实的怀疑：1 单个 `move`（不经 `batch`）改写的页数不设界（同 REST），升级中每页多一条账本的写（本机每条中位数 0.15–0.20 ms），一万页链向同一页时单元约多 2 秒、持着笔记本行，设计 3.5 已接受，没有量这么大的扇入；2 批的 bench 给 `/mcp` 以 `mcpBodyLimit`，但没有照 `wire.go:237` 设 `LargeBodyAbove`，单个顺序的请求下判断不影响 `unit_ms`，没有跑对照；3 读的守卫还没有在 7763 上跑过（分支的两次 CI 落在 9V45、9V74：22.73、36.47 秒），照约 2 倍估约 35 秒。
  - B 在修复核对第二轮量的一般的最大规模（改名一页没人链向的页、再编辑 49 页满负载的页，49 页恰在解析的界上）：升级中计划多约 36–40 ms，每次问不到 1 ms；不必为第 270 条的读另立守卫。
  - A 在修复核对第二轮指出、不算发现的一种形状：本批自己加在 frontmatter 里的链接，先拿掉、改名、再放回，不会被标上；那是 agent 在本批自己加的链接，不是基线页原有的，不在护栏要守的范围里，断了闸门 6 在提交时看得到。
  - C 审查时的未核实的怀疑：P2S1C-M1 的整个程序答 `page.upgrade_immutable` 是沿代码走出的；第 268 条之后标记只设不清，这个形状答 `page.upgrade_batch_rewrite`，不再成立。
  - 变异 n4b、n4c、20b、20c 在 g9、g25、g21、g23 上活下来（修复第二轮）：第 270 条的索引判据先把 P 标上，这几格分不出正文一遍的两个判据；它们由计划的单元测试抓到，g27 的 Q 两格让 n4、20 在组合根上也死（`mut/g27q.out`）。
  - 流程（照实写下）：实施者推送之前改写过一次 4 号（`f940065a` → `f6de159e`：变异 o9 第一轮没抓到，给表补一行；原来的 5、6 号 `4695e313`、`aefc4785` 原样重放成 `90134668`、`f4d88dd4`，`git cherry-pick`）；修复第二轮的两个提交推之前改订过两次（第一版 `ac13346e`、`9761259e`；第二版 `1960105e`、`35478201`），旧的都没推过。分支的推送都是普通的 push。
- **推后**：
  - guards-sources 的时限（B，修复核对一轮）：7763 上作业 16.7–16.8 分钟（实施者写 16.7–16.9，差在取 API 还是日志的时刻）、末行在第 16.4–16.5 分钟开始，时限 30 分钟，约 1.8 倍；十次里"卡住时默认超时先到"每次都成立，最小余量约 3.5 分钟；对"同一种 runner 慢三成"不稳（16.5 × 1.33 ≈ 21.9，加 10 分钟约 31.9）。下次改到这一份时照第 269 条的写法提到 40 分钟。
  - costs-2 在 S2 之后的余量（B，修复核对一轮）：S2 往 costs-2 再加检查的守卫（设计估 7763 上约 1–1.75 分钟）之后末行约在第 25.5 分钟开始；慢三成的实例上约第 34 分钟开始、打出约第 44 分钟，离 45 分钟只剩约 1 分钟。S2 量完那一行时再看一次。
  - 单元守卫的形状 a（P2S1B-N1）：7763 上对 5.2 秒的界约 1.64 倍，界守的主要是 M9 的改写；来了更慢的 runner 时最先红的是它，原因与护栏无关，到时照负责人 2026-11-06 的话调界或规模。
  - P3/S2 的 playbook：拆法（第 268、270 条）与只写名字的链接（P2S1C-N2），写在设计第 9 节。
  - `mcp_test.go:269`、`:272` 等日志的两个 2 秒改成 10 秒（`TestTheMCPCallStopsWhenTheClientGoes`），由 P2/S2 的一个测试提交做。依据：本机第三次在负载下失败（M9/P1 S2 负载 29、M11/P1/S1 负载 24、本轮 7.7），`heavy.sh` 挡不住（本轮已在 `heavy.sh` 下、只有 `make test` 自己在跑）；这两次等的是异步写下的日志，出现了就立刻返回，不是产品的时限（产品要的"客户端走开之后 1 秒之内结束"是 `:263`）；CI 上第五份独占一台 runner，最近 23 次的失败里没有它（审查 B，修复核对第二轮第 3 节）。
  - 交给 M12 的性能移交：决定 271 的数，接在 M9 那一项之后（设计第 9 节）。

## 反向对照

建成时以 `mut.py` 跑（`NW_SERVER` 指实施者的工作树的 `server`；规格由 `m11/p2s1/mut/mkspec.py` 生成 56 条：7.3 的 S1 各行（1–24、84）44 条，设计列几个测试的每个测试各一条，12 号的两个方向拆成 12、12x，加实施者自己的 11 种 o1–o11、12 条；先在没改的树上把 26 条不同的命令各跑一次，都 rc=0，没有"no tests to run"；整轮经 `heavy.sh`；改坏生成的 SQL 常量时不跑 `make gen*`）。第一轮在 `aefc4785` 上：54 条抓到；o4 的规格编不过（`Step` 没有 `Notebook.ID`），o9 没抓到（`holdsAt` 的 `rewrote` 恒真：表里没有"一次没够着这一页的移动之后，写下它当时的原样"的一格）；给表补一行、4 号改写之后的第二轮 o4（改成 `uuid.UUID{}`）、o9 都抓到。组合根的格子单独以 `-run '^TestUpgradeGuardrails$/^g<n>$'` 跑。审查者的在 `p2s1/review/` 下（A 的 `a/`、B 的 `b/`、C 的 `c/`）。

**7.3 的 S1 各行与实施者自己的**（建成时的语义；第 17–22 条在修复一轮照第 268 条改写，见下）：

| # | 变异 | 抓它的测试 | 结果 |
|---|---|---|---|
| 1 | `guardrails` 不判移动与改名 | `TestTheFreezeGuard`、g5、g6 | 抓到 |
| 2 | 只看 `Options.UpdateLinks` | `TestTheFreezeGuard`、g13 | 抓到 |
| 3 | `step()` 不带 `keepLinks` | g13、`TestTheUnitStepCarriesKeepLinks` | 抓到 |
| 4、5 | 正文不去 BOM；去掉开头的全部 BOM | `TestTheUpgradeBodyShapes` | 抓到 |
| 6 | 比全文（`UpgradeBody` 不切 frontmatter） | g1、g14、g15、`TestTheFreezeGuard` | 抓到 |
| 7 | `Rewrite` 的步也比正文 | g4、`TestTheFreezeGuard` | 抓到 |
| 8、9 | 账本的写不核对 `before`（生成的 SQL）；基线行在而没写到时不答错 | g17、g18、`TestAcceptUpgradeContent` | 抓到 |
| 10 | 回滚的单元也交 `BeforeHash` | `TestTheUnitKeepsTheLedger`（`rolling_back`） | 抓到 |
| 11 | `BodyHash` 为空时放行 | `TestTheFreezeGuard` | 抓到 |
| 12、12x、13 | 单元在 `reviewing` 也算 `BodyHash`；在 `upgrading` 不算；为 `Rewrite` 的步也算 | `TestTheUnitHashesTheBodyWhileUpgrading`；12x 另有 g1（缺陷） | 抓到 |
| 14 | `UpgradeBaselineContent` 不限 `baseline`（生成的 SQL） | `TestUpgradeBaselineContent`、g8 | 抓到 |
| 15、16 | `CheckReads` 不计盲写；没有升级时也计 | `TestTheReadsBoundCountsBlindWritesWhileUpgrading`、g22 | 抓到 |
| 17–20 | `Folded` 取 `Moved`；不传 `Folded`；写的路径不清；写的路径只看有没有改写 | `TestThePlanTellsAFoldedRewrite`、g9、g11、g21 | 抓到 |
| 21、22 | 正文的拒绝标在单元写的那个操作；所有错误都标在 `Last` | `TestTheBatchMarksABodysRefusalAtTheLast` | 抓到 |
| 23、24 | 账本只在 `!Rewrite` 时写；账本的写不限 `baseline` 与 `kind`（生成的 SQL） | g4、`TestTheUnitKeepsTheLedger`、`TestAcceptUpgradeContent` | 抓到 |
| 84 | 单元不比计划时的状态 | `TestTheBatchPlansAgainOnceTheUpgradeStarted`（两格） | 抓到 |
| o1–o3 | 账本对每种步都写；没有升级也写；`BeforeHash` 交成写之后的哈希 | `TestTheUnitKeepsTheLedger`、`TestTheUnitHashesTheBodyWhileUpgrading`、g1 | 抓到 |
| o4 | 守卫读别的升级的行 | `TestTheFreezeGuard` | 第一轮规格编不过；第二轮抓到 |
| o5、o6 | `UpgradeBaselineContent` 去掉 LATERAL 的 `LIMIT 1`；不限 `deleted_at` | 读的守卫（判计划，14 秒失败）；`TestUpgradeBaselineContent` | 抓到 |
| o7、o8 | 盲写不跳过新建的页；编辑的路径不设 `folded` | `TestTheReadsBoundCountsBlindWritesWhileUpgrading`；`TestThePlanTellsAFoldedRewrite` | 抓到 |
| o9 | `holdsAt` 的 `rewrote` 恒真 | `TestThePlanTellsAFoldedRewrite` | 第一轮存活；补一行之后抓到 |
| o10、o11 | 单元的 `BodyHash` 不切 frontmatter；守卫的切分不切 frontmatter | `TestTheUnitHashesTheBodyWhileUpgrading`；`TestTheFreezeGuard` | 抓到（"两处的切分同一"的两半） |

**审查者的变异**（照各自的命名；存活的都成了一条发现）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | MA1（只比有没有升级）、MA2（只在"没有升级 → `upgrading`"时重新计划） | 存活：P2S1A-N1 |
| A | 探针 `TestZZRevAProbe` 三格（不是变异）：分开调用、一个批断链、一个批指向别处 | 分开调用答 `page.upgrade_immutable`；两种批都成功：P2S1A-I1 |
| B | r1–r7：改坏生成的 SQL 常量（丢 `node_id` 或 `upgrade_id` 的键、两个键包进 `IS TRUE`、`LATERAL` 丢外行的键、`page_contents` 换成不能下推的子查询） | 都抓到：读的守卫在第一种统计状态（从未分析）、两种计划的计划判定就停下，在计时之前 |
| B | u1（`CheckReads` 升级中也不计盲写） | 抓到：单元守卫的形状 e 进了单元、写下了 |
| B | 账本的写在并发下的临时测试三格（去掉生产里的正文行锁） | 语句本身"宁可报缺陷也不放过"；生产里同一页的写由正文行锁排成先后 |
| C | `mv`（编辑的路径赋值 `pg.folded = rewritten != content`） | 四处里三处存活：P2S1C-M1 |
| C | `mw`（写的路径赋值）、`mf`（账本比 `content_hash`）、`mr`（守卫不判改名） | 抓到 |

**修复一轮的变异**（实施者在头 `bff70ffc` 上跑，规格 `m11/p2s1/mut/spec-f1.json` 31 条，先在没改的头上把 13 条命令各跑一次都 rc=0，31 条都抓到；写成设计 7.3 的第 17–20 行与表尾）：

| # | 变异 | 抓它的测试 |
|---|---|---|
| n1 | 写的路径清掉标记（设上之后 `if !same { pg.reached = false }`） | `TestThePlanTellsAPageAMoveReached`、g21、g23 |
| n2 | 编辑的路径清掉标记（`pg.reached = rewritten != content`） | `TestThePlanTellsAPageAMoveReached`（"编辑被到达、无关的移动、再编辑"） |
| n3 | 守卫先比正文、相同就放过（原来的次序） | `TestTheFreezeGuard`、g23、g25 |
| n4 | 标记只在写的路径设，编辑的路径不设（同 o8） | `TestThePlanTellsAPageAMoveReached`、g9、g25 |
| n5 | 字节不变的写不过守卫（`if false && w.Reached …`） | `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds`、g23、g25 |
| n6、n7 | 字节不变时不论升级都过守卫；升级中不论标记都过守卫 | `TestTheUnitJudgesAReachedWriteOfTheBytesAPageHolds`（"no upgrade, reached"、"upgrading, not reached"） |
| 17、18 | `Reached` 取 `Moved`；批不把 `Reached` 交给单元 | `TestThePlanTellsAPageAMoveReached`、g11；g9、g23 |
| 20 | 写的路径照原来的规则只在"写的是改写的原样"时设（`rewrote && same`） | `TestThePlanTellsAPageAMoveReached`、g21、g23 |
| 21、22、o9 | 同上 | 同上 |
| MA1、MA2 | A 的两个 | `TestTheBatchPlansAgainOnceTheUpgradeStarted`（"from reviewing"两格） |
| `mv` | C 的 | `TestThePlanTellsAPageAMoveReached`、`page/domain` 全包 |
| an2 | 升级中非改写的写一律当作字节不变、什么也不写 | g14、g15 |

原来表里的第 19、20 条照第 268 条已不成立：19（"写的路径不清标记"）现在就是规定，换成 n1；20（"写的路径只看有没有改写"）现在就是规定，换成上表的 20。o8 即 n4。

**修复核对一轮**（在 `bff70ffc` 上）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | MA1、MA2 重跑 | 抓到（"from reviewing"两格） |
| A | MB1（`unit_content.go:80` 的条件前加 `false &&`）、MB2（同一处去掉 `upgrading` 的条件） | 抓到：单元测试的两格与 g23、g25；"no upgrade, reached" |
| A | 探针第二版七格（不是变异） | I1、改名文件夹、P 自己移动都被拒；"拿掉再放回"的两格成功：P2S1A-M1；两种拆法成功 |
| C | `mv` 照新代码改写（`pg.reached = rewritten != content`）：计划的表、`page/domain` 全包、原来的复现 | 都抓到 |
| C | `rw`（写的路径只在 `rewrote && !same` 时设） | 抓到：`TestThePlanTellsAPageAMoveReached` |
| C | `ra`（`body` 在读基线行之前先看 `Reached`） | `TestTheFreezeGuard` 与临时组合根测试三格抓到；整个 `TestUpgradeGuardrails` 下存活：P2S1C-N5 |
| C | `rg`（`applyStep` 扔掉 `guarded` 交回的步） | 抓到：`page/app` 全包（`TestSourceGuardKeepsTheFirstKeyAcrossRewrites` 等） |
| C | 关掉字节不变那一处（`if false && w.Reached …`），带 `-v` 重跑 g23、g25 | 抓到：`:276` 的"写 P 原来的字节"与 `:290` 的编辑都做完了 |

**修复第二轮的变异**（实施者在 `35478201` 上跑整轮，规格 `mut/spec-f2.json` 52 条，先在没改的树上把 20 条命令各跑一次都 rc=0：48 条抓到，n4b、n4c、20b、20c 在组合根的那一格上存活、同一个变异在单元测试上抓到；加 g27 的 Q 两格之后 n4、20 在 g27 上重跑（`mut/spec-g27q.json`），都死。`0b016421` 与 `35478201` 的提交 1 只差 g27 的这 11 行测试，代码相同。整轮没有在最后的头上重跑：`0b016421` 与 `35478201` 的提交 1 只差 g27 的 11 行测试，代码相同）：

| # | 变异 | 抓它的测试 |
|---|---|---|
| x1 | 不做新的判据 | `TestTheBatchJudgesAPageByItsLinksBeforeIt`（takenOut, upgrading）、g27 |
| x2a、x2b | 原来的范围：`reachedBefore` 的各项都取到 `pg.First`；只把 `MovesBetween` 的终点改成 `pg.First` | 同上两处 |
| x2c | 头问过就用头的答案（去掉 `First != Last`） | `TestTheBatchJudgesAPageByItsLinksBeforeIt` 的 `askedFirst` 格（第一版的测试上存活，补了这两格） |
| x3 | 新的判据在普通笔记本里也做 | 同上，普通笔记本的格（数读与拒都错） |
| x4a | 判出来不交给计划 | 单元测试、g27 |
| x4b、x4c | 正文的一遍把标记清掉：写的路径；编辑的路径 | `TestThePlanTellsAPageAMoveReached` 的 `MarkReached` 格、单元测试；x4c 另有 g27 |
| x4d | `MarkReached` 只标新建的页 | `TestThePlanTellsAPageAMoveReached`、单元测试 |
| 第一轮的照新代码重跑 | n1–n7、17、18、20–22、o9、MA1、MA2、`mv`、an2 | 都抓到；n4、20 在 g9、g25、g21、g23 上存活（"接受"），在单元测试与 g27 的 Q 格上死 |
| C 的 | `ra`、`rw`、`rg` | `ra`：`TestTheFreezeGuard` 与 g26（P2S1C-N5 要的）；`rw`：`TestThePlanTellsAPageAMoveReached`；`rg`：`page/app` 全包 |

**修复核对第二轮**（在 `198b0cd5` 上）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | n4（编辑的路径不设标记）、20（写的路径只在 `rewrote && same` 时设）、x（`read` 里不调 `reachedBefore`），都只跑 g27 | 都抓到：n4 在 Q 的编辑一格（`:344`），20 在 Q 的写回一格（`:346`），x 在 P 的第一格（`:329`） |
| A | 探针第三版（不是变异） | probe2 的两种形状与改名文件夹、写不带链接的字节、frontmatter 里的链接、P 自己移动之后放回都被拒；改名再改回原名、拆开的写法都过 |
| B | 两个临时测试（不是变异）：一般的最大规模；M9 已接受的"许多次改名之间的带链接的页" | 前者升级中多约 36–40 ms；后者计划翻倍：P2S1B-M2 |
| C | `ra`（`body` 在读基线行之前先看 `Reached`）：g26、`TestTheFreezeGuard` | 都抓到 |

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `bff70ffc`（修复一轮 4 个提交），三位各在自己的工作树里 | A：P2S1A-I1、N1、N2 关；第 268 条的规则对，`Unit.guarded` 那一处必要、正确，没有改到别处的行为；新的 P2S1A-M1（拿掉、改名、放回）。B：P2S1B-M1、N1 关；注释里的数重算相符，第七份本机一轮 179.5 秒、归类 542 个照 `Makefile` 直接读出也是 145/51/121/124/62/11/28；没有新发现，guards-sources 建议下次改到时提到 40 分钟。C：P2S1C-M1、N1 关，N2 照计划留给文档提交，N3 部分；逐个提交（`060b365a`、`8d739bbd` 各自 build、vet、`make lint-go`、`make gen-check`、short、`page/...` 的测试（8 个包）与组合根的竞态测试都 rc=0；`80da6b1f` 只改 `ci.yml`，`bff70ffc` 只改 UP4，UP4 本机三个都过）；CI 42787571274 18 项全绿；新的 P2S1C-M2（文档的清单）、N4（`contentStep` 的注释）、N5（组合根没有"不是基线页、被到达"的格） | Minor 2、Nit 2 全部接受：修复第二轮（`fix-2.md`）两个提交；P2S1C-M2 在本提交；guards-sources 推后 |
| 2 | `198b0cd5`（修复第二轮 2 个提交），三位各在自己的工作树里 | A：P2S1A-M1 关；第 270 条的判法对，试过的别的路都被拒，没有新发现。B：第一轮的 M1、N1 仍关着；报告的数相符（第七份本机一轮 171.6 秒，归类照 `Makefile` 直接读出同样是 542 个、145/51/121/124/62/11/28，CI 的起止与报告的表逐项相同）；新的 P2S1B-M2（第 270 条的代价）；`TestTheMCPCallStopsWhenTheClientGoes` 那次失败与本轮无关、建议改等待。C：P2S1C-N4、N5 关，两个提交逐个都绿；文档稿对 `check1-c.md` 第 4 节逐行核过，P2S1C-M2 部分；新的 P2S1C-N6–N8（文档稿） | P2S1B-M2 由决定 271 关上（只改文档）；P2S1C-M2 与 N6–N8 在本提交；`mcp_test.go` 的等待推后到 P2/S2；不再有修复轮 |

## 实测

- **护栏的读的守卫**（`TestUpgradeGuardrailReadsTakeTheirTimeAtAnyStatistics`，`page/adapter/postgres`，costs-2；建成时本机两轮，`m11/p2s1/logs/c5-reads-{1,2}.log`）：这次升级 10 万行基线（每页 1 KiB 的正文）、另一次升级 10 万行、另九本各 5 万页的笔记本与它们的升级；三种统计（从未分析、分析过、`upgrade_nodes` 只有别的升级的统计）× 自定义与通用的计划，每格先判两条语句的计划，再在强制成那种计划的连接上各 1,000 次读与写取中位数。六格的计划都合规；读 0.12–0.16 ms、写 0.15–0.19 ms（两轮相同到 0.01 ms），界 5 ms（三倍是 0.57 ms，向上取整到 5 ms）；种子 12,705、12,672 ms，整个测试约 17 秒。B 本机一轮 18.9 秒（种子 14.7 秒），六格读 0.12–0.17 ms、写 0.15–0.20 ms。
- **护栏在批的界上的单元**（`TestUpgradeGuardrailsAtTheBatchBoundTakeTheirTime`，组合根，guards-core；批的 bench，生产的 15 秒请求；五种形状各三次的中位数，ms，升级中 / 同一形状在普通笔记本上）：

  | 形状 | 第一轮 | 第二轮 | 界（期限 14 s） |
  |---|---|---|---|
  | a. 一次改名改写 400 页（每页 84 KB、200 条链接）：系统改写，守卫不读，每页一次账本的写 | 1,701 / 1,686 | 1,712 / 1,647 | 5.2 s |
  | b. 100 个编辑各删一行约 62 KB 的 frontmatter，头合计约 13.8 MiB：计划读一次、守卫再读一次 | 343 / 293 | 348 / 294 | 1.1 s |
  | c. 一页 `MaxContentBytes` 的 frontmatter 编辑 | 47 / 35 | 44 / 36 | 200 ms |
  | d. 三个盲写（两页 5 MiB、一页约 4 MiB，头合计恰在 14 MiB 之内）各删掉 frontmatter | 23 / 15 | 26 / 17 | 100 ms |
  | e. 100 个盲写各删掉 5 MiB 的 frontmatter：升级中由计划在 ops[2] 答 `page.batch_too_large`，不进单元；普通笔记本上做完 | 2 / 251 | 3 / 252 | 100 ms |

  种子 4.4 秒，整个测试约 21 秒。护栏加的：a 约 15–65 ms，b 约 50 ms，c 约 10 ms，d 约 8 ms；e 在升级中由计划先拒，反而比普通笔记本的写快。设计 7.1 引审查 A 实测"本机 1.58 秒"的 e 是改动之前（盲写不计）的数。B 本机一轮：升级中 a 1,807、b 348、c 53、d 30、e 3 ms，普通笔记本 1,746、314、43、16、243 ms。修复一轮的逐个提交与 `make test` 里三次都在界内（升级中 a 1,765、1,732、1,788 ms）；修复第二轮的 `0b016421` 上 a 1,763 / 1,682、b 352 / 300、c 51 / 44、d 37 / 15、e 3 / 247（种子 4.8 秒）。第 270 条在 M9 已接受的一个形状上的代价见 P2S1B-M2。
- **组合根的各份**（竞态构建的口径）：建成时与修复一轮之后 `go test -race -list` 都是 542 个顶层，七份 145、51、121、124、62、11、28，各在恰好一份（同 P1 末；S1 只加子测试与 `page/app`、`page/domain` 的测试，两个新守卫带 `//go:build !race`）。第七份本机单独两轮（`heavy.sh make -j1 test-go-bootstrap-7`）：建成时 172.4、174.6 秒（P1 末 168.9、171.1 秒），修复一轮之后 181.6、176.4 秒（g23–g25 三格各一本笔记本与一次发起，加上 g14、g15 的两句）。修复第二轮之后（头 `198b0cd5`）183.9、181.4 秒（g26 多三本笔记本、g27 的 Q 多一本，两轮平均多 3–4 秒；量时刚跑完 `make test`，15 分钟负载 5.4–6.3），B 一轮 171.6 秒（负载 1.0–1.6）；归类照旧，542 个、145/51/121/124/62/11/28。
- **e2e**：UP4 的故事单独跑三个通过，3–4 秒；在 `make e2e` 里 P1 的删除 76 ms，正文与移动 172 ms，`batch` 83 ms。`make e2e` 520 个（P1 末 518 个，多的是 UP4 的两个）。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**：建成时的六个各自 detached，每个跑 `go build ./... && go vet ./...`、`make lint-go`、`make gen-check`、`make test-go-short`，再跑它自己的组合根测试（1–5 号的六个码与矩阵的测试，竞态，16–24 秒；2 号另跑 page 模块的全部竞态测试 196 秒，5 号两个守卫，6 号 `make lint-web`、`make knip`、`make test-web`（192 个文件、4,341 个测试）、`make build` 与 UP4），每一项 rc=0；改写 4 号之后对 4–6 号再核一次，每一项 rc=0。审查 C 另在三个中间提交上各自跑，加第五份的 `TestTheMCPToolsErrorsCarryTheirCodes`，都 rc=0。修复一轮的四个同样逐个跑（1 号另跑 page 模块的全部竞态测试 205 秒与单元的守卫一行，2 号 `page/app` 的竞态测试，4 号 `make lint-web`、`make knip`、`make build` 与 UP4），每一项 rc=0。修复第二轮：改订之前的 `1960105e`、`35478201` 各自 build、vet、`make lint-go`、`make gen-check`、short（43、44 秒）、组合根七个测试（57、56 秒），`1960105e` 另跑 `page/...` 竞态（202 秒）与单元的守卫，`35478201` 另跑 `page/app` 竞态（60 秒），每一项 rc=0；最后的 `0b016421`、`198b0cd5` 同样各自跑（short 44、44 秒，组合根七个测试 64、67 秒；`0b016421` 另跑 `page/...` 竞态 198 秒与单元的守卫 23 秒，`198b0cd5` 另跑 `page/app` 竞态 56 秒），每一项 rc=0。审查 C 另在这两个上各自跑 build、vet、`make lint-go`、`make gen-check`、short、`page/...` 的测试与组合根的竞态测试（约 53 秒），都 rc=0；B 在 `0b016421` 上跑 build、vet 与 short。每个提交前后工作树都干净。
- **建成时的头**（`f4d88dd4`，开始前 `df -h /` 可用 21G，结束时 21G）：`make lint gen-check knip build` 8 秒通过；`make test` 3,359 秒通过（竞态、组合根七份、成本三份、lint 的成本、守卫两份、vitest 192 个文件 4,341 个测试；日志里一处 `--- FAIL: TestWikiCostSamplesWaitForFinished/sample-failure` 是那个测试在子进程里故意跑的失败的样本，外层通过）；`make test-go-short` 39 秒；`make e2e` 520 个 139 秒。
- **修复一轮**（`bff70ffc`；开始时 df 18G 可用，`make test` 中间最低到 15G）：`make lint gen-check knip build` 8 秒；`make test` 3,579 秒通过（日志里没有 `--- FAIL`）；`make test-go-short` 44 秒；`make e2e` 520 个 145 秒。
- **修复第二轮**（`198b0cd5`；每一项之前判 df，都是 46G）：`make lint gen-check knip build` 10 秒；第一次 `make test` rc=2，576 秒停在第五份的 `TestTheMCPCallStopsWhenTheClientGoes`（`mcp_test.go:273`："the log has map[], want the answer never written at debug"，2 秒之内没等到 debug 的那一行；七份组合根并行，5 分钟负载 7.73），`make test-go` 因此停下，成本、守卫与 web 都没跑；这个测试单独 `-race -count=5` 五次都过（1.1–2.5 秒）；为了让其余的也在头上跑过，整个 `make test` 重跑，3,457 秒全绿（`go test` 的 ok 181 行，没有 FAIL；vitest 192 个文件 4,341 个测试）；`make test-go-short` 44 秒；`make e2e` 520 个 147 秒。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 作业 | 42770402937（建成，`f4d88dd4`） | 42787571274（修复一轮，`bff70ffc`） | 42819115296（修复第二轮，`198b0cd5`） |
|---|---|---|---|
| server (race) | 7:55（7:38） | 13:59（13:37） | 13:19（13:01） |
| server (bootstrap-1) | 4:40（4:22） | 6:37（6:13） | 6:44（6:22） |
| server (bootstrap-2) | 4:54（4:37） | 6:05（5:47） | 6:06（5:45） |
| server (bootstrap-3) | 5:03（4:45） | 7:41（7:15） | 7:14（6:49） |
| server (bootstrap-4) | 6:21（5:58） | 7:33（7:08） | 6:37（6:15） |
| server (bootstrap-5) | 6:02（5:38） | 6:56（6:34） | 6:56（6:35） |
| server (bootstrap-6) | 3:45（3:26） | 5:42（5:21） | 5:49（5:30） |
| server (bootstrap-7) | 5:11（4:52） | 3:40（3:21） | 4:22（4:00） |
| server (costs-1) | 15:35（12:14） | 10:20（7:49） | 9:26（7:06） |
| server (costs-2) | 13:03（12:44） | 22:01（21:37） | 17:14（16:48） |
| server (costs-3) | 16:55（16:36） | 16:59（16:39） | 13:31（13:06） |
| server (costs-lint) | 10:21（9:59） | 12:58（12:35） | 10:30（10:01） |
| server (guards-core) | 19:53（19:31） | 19:46（19:23） | 17:54（17:31） |
| server (guards-sources) | 16:42（16:23） | 13:38（13:18） | 14:44（14:24） |
| server (short) | 3:35（3:17） | 3:20（3:01） | 4:12（3:53） |
| web | 5:04（4:31） | 3:55（3:27） | 5:05（4:28） |
| e2e | 9:50（7:22） | 7:40（6:11） | 12:01（7:15） |
| image | 1:24 | 1:30 | 1:34 |

- 建成时一次全绿，18 项，普通推送（新分支，`git ls-remote` 是 `f4d88dd4d56aa21e07f386181ffef14aef48be5c`）。细看三份的日志：第七份在 EPYC 7763 上包 259.0 秒、Test 一步 292 秒（P1 末在 7763 上三次 288、284、282 秒），多约 4–8 秒，比设计 7.5 估的 15–25 秒少，离 450 秒的线约 158 秒；costs-2 在 EPYC 9V45 上，读的守卫 22.73 秒（种子 17.4 秒），六格读 0.07–0.13 ms、写 0.10–0.17 ms；guards-core 在 EPYC 7763 上，单元的守卫 37.83 秒（种子 8.3 秒），升级中 a 3,165、b 544、c 95、d 47、e 5 ms（普通笔记本 3,054、464、78、23、362 ms），对界的余量 a 1.64 倍、b 2.0 倍、c 2.1 倍、d 2.1 倍，期限 14 秒的余量 4.4 倍；7763 约是本机的 1.85 倍。
- 修复一轮（普通推送 `f4d88dd4..bff70ffc`，`git ls-remote` 是 `bff70ffc4d426e9c2e328f73214d24d469d66d23`）18 项全绿：第七份在 EPYC 9V45 上包 180.5 秒、Test 一步 201 秒；costs-2 在 EPYC 9V74 上 22:01（新的时限 45 分钟），读的守卫 36.47 秒（种子 28.6 秒），六格读 0.15–0.27 ms、写 0.23–0.33 ms；guards-core 在 EPYC 7763 上 19:46（新的时限 40 分钟），单元的守卫 37.86 秒，升级中 a 3,152、b 552、c 85、d 47、e 5 ms，普通笔记本 3,073、469、61、24、389 ms，a 对 5.2 秒约 1.65 倍。B 把这次 costs-2 与上一次同种 runner（42749131694，16.6 分钟）逐行比，与 S1 无关的 linking 几行也慢了 1.22–1.33 倍，是实例的差别。
- 修复第二轮（普通推送 `bff70ffc..198b0cd5`，rc=0）18 项全绿，`gh run view` 的起止与报告的表逐项相同（B）。B 从作业日志取的 runner 与余量：costs-2 在 EPYC 9V74 上 17:14 对 45 分钟，末行第 16.6 分钟开始；guards-core 在 Xeon Platinum 8573C（新见的 runner）上 17:54 对 40 分钟，末行第 17.4 分钟；guards-sources 在 8573C 上 14:44 对 30 分钟，末行第 14.4 分钟；race 在 8370C 上 13:19 对 20 分钟（`-timeout 15m`）；"卡住时 go test 的默认超时先于作业时限到"在三份守卫与 costs-2 上都成立。组合根七份的 Test 一步 382、345、409、375、395、330、240 秒，都在 450 秒之下；最慢的第三份落在 7763 上，里面没有本轮的测试，与设计 7.5 记的"第三份最坏 430 秒"同一档；第七份在 9V74 上。单元的守卫在 8573C 上 34.03 秒，形状 a 升级中 2,606 ms，对 5.2 s 是 2.0 倍；读的守卫在 9V74 上 29.47 秒；`TestLinksReachedReadPlans` 29.80 秒、`TestLinksReachedDenseReadPlans` 108.98 秒。

S1 开工之前 `main` 的两次 CI（00 号文档第 13 节）：P1 完成的 `f002672b`（run 42752100006）全绿，组合根七份与 `ce370441`（run 42749131694）两轮的均值 367、395、354、358、284、319、241 秒，都在 450 之下，costs-2 在 7763 上 Test 22:57（上一次在 9V74 上 16:18，各条守卫一律约慢 1.4–1.5 倍，是 runner 的差别）；P2 设计的 `c37a3faf`（run 42762546769）18 项全绿，七份的 Test 一步 361、430、389、356、391、304、240 秒，costs-2 23:45（Test 23:20），又一次贴近第 267 条的依据。

## 环境

修复第二轮中途，本机根分区可用只剩 14G（125G，用了 106G）：变异每条都要重新编译，Go 的构建缓存（`~/.cache/go-build`）从 45G 涨到 47G；本轮开始时可用 16G。实施者停下，没有动全机共享的缓存与 scratchpad 里别的轮次的证据。之后主持者照 [06-environment](../../claude_to_codex/06-environment.md) 第 3 节（第 42 行）经 `heavy.sh` 删掉 12 小时没用过的条目（`find ~/.cache/go-build -mindepth 2 -type f -mmin +720 -delete`），缓存降到 15G，df 回到 46G；不动 Docker 的镜像、容器与卷。实施者随后把逐个提交与门禁的两个脚本改成每一项之前都判 df、低于 15G 就停，之后各项开始前都是 46G；审查者第二轮每一项重的之前判过 df（44–46G）。
