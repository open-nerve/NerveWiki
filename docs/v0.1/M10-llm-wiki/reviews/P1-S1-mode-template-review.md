# M10/P1 S1（模式与模板（服务端））：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m10-p1-s1`（实施的 7 个提交：`shared.ClientOf`，page、asset、transfer 的三份 `clientOf` 删去 `46e3d20c`，迁移 00048（`mode`、`source_root_id` 与两个检查）、notebook 的 `Mode`、`Locale`、读笔记本的五条查询与两处手写的转换、根的 `State` 与 `StateOf`、契约的 `Notebook` 的两个必填成员与 `NotebookMode` `d0ce2cf8`，page 的 `Writer.Seed` 与 `run`、`SeedWrites`、`TreeWrites().Seed`、`ParseTemplate` `456fe026`，播种者的扩展点、建的事务的次序、wiki 模块（契约表、四页模板、播种者、`Actions()`）、组合根的次序、`checkPages` 分出 `pageViolations` 与 LLM Wiki 的规则、建的三条交错与两个整个程序的测试、e2e 的夹具与 LW1 (API) `33edc078`，`downgradeNotebook`、`notebook.change_mode`、`transfer.NewWritingJobs`、模式变化的扩展点、矩阵的变体 `wlab` 与降级的两行 `32bba1f8`，事件 `mode` 与契约的 `EventMode` `e8962f7b`，page 的 `StateOf` 与组合根的 `pageNotebooks`、MCP 的 `notebooks` 与 `open_notebook` 答 `mode`、e2e 的 LW1 (MCP) 与 MC1 `66602404`；第 4 步因 rc11 加一个反例在推送之前改正，第 4–7 步重放（偏离 14）；从 `main` 的 `f92ecac7` 开始，审查时在 `66602404` 上，115 个文件、加 4,894 行。修复一轮十三个：B-M1 = C-M1 `cb9de147`、B-M2 `9daac068`、A-M1 `d37f57aa`、A-M2 `ec5e67b4`、A-M3 `e21332db`、A-N1 = C-N2 `5008ee7d`、A-N2 `3884692a`、A-N3 `75e34bb8`、A-N4 `ee464e7d`、B-N1 `670765c6`、B-N2 = C-N6 `2e954be5`、C-N3 `9c2da5c7`、C-N4 `86e4c8da`（20 个文件、加 365 行）；一共 20 个；合并 `45d086a1`），对照 [01-P1-contract-frame.md](../01-P1-contract-frame.md) 第 0–2 节、3.1–3.16 与 3.29 的 S1 的各行、4.1、第 5 节的 S1 的部分、6.1、6.5、第 7 节的 S1 的各行（7.3 的第 1–22 条）与第 8 节，[P1 设计审查](P1-design-review.md)（主持者的决定与两轮复核），[00-M10-design.md](../00-M10-design.md) 4.1–4.5、4.15、4.17、4.18 与第 5、8、9、11 节，[v0.1 总体设计](../../v0.1-design.md) 12.4 与第 13 节，实施者的报告（15 条偏离、门禁、实测、CI、变异与反向对照的表、留下的一项） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验，用到 Docker 的都经 `heavy.sh`，不下载任何东西：建的事务、播种者、`Seed`、wiki 模块与客户端（A：快的测试（`shared`、`wiki/...`、notebook 的 `app`、`domain`、`adapter/http`，page 的 `app`）；经 `heavy.sh` 在竞态下跑组合根的建的三个测试与自己的探针 `TestReviewAReentryWithAQueuedDeletion`（工作区的删除、bob 的移出、bob 的降级三种：`notebook_members` 上的触发器等测试持着的 advisory lock，让建停在管理员的插入之后、持着 acme 的行 `FOR SHARE`，另一个改动排在行上之后才放开），notebook 与 page 的模块根的播种者与 `Seed` 的测试；`a-work/tpl.py` 从 3.9 取出四页，与文件逐字节比（四个都相同：648、593、2,209、2,107 字节）；自己的变异 a1–a19 与三个探针：段落对应的四种形状、`Seed` 照交来的事实、以随正文变的扩展解析）；降级、事件、模式的读、迁移、契约与矩阵（B：`make gen-check-go`，离线的 `pnpm install` 之后的 `gen-check-web`；快的包；经 `heavy.sh`：迁移、notebook 的 postgres 适配器与模块根、transfer 的模块根，组合根在竞态下的降级与事件的四个测试、矩阵一族的八个测试（包 57.6 秒）；两个只在自己的工作树里的整个程序的探针：两个管理员同时降级（在笔记本行上逼出：200 与 409，流只收到一个 `mode`）、`NOTIFY` 之后由一个推迟的约束触发器回滚的降级（500，行照旧，流的下一帧是标记）；自己的变异 b1–b7）；测试、变异、e2e、网页、提交、门禁、CI 与实测（C：7 个提交各自 `go build`、`go vet`、整个服务端模块与 `server/tools` 的 golangci-lint、`go mod tidy -diff`、整个 `go test -short`，碰契约的提交 `make gen-check`，碰网页、e2e 或契约的提交 md-fixtures、oxlint、oxfmt、每个包的 `check:types`、knip 与 vitest；第 3–6 步加或改的库上的测试各在它自己的提交上；整个 e2e 在第 2、4、6 步与头上（282、283、283、284 个）；以 `-race` 的 `go test -list` 核对分份；读一次 CI 的作业；自己的变异与重跑（C1–C40 里的 22 个）与构建的二进制上的 e2e 的变异 E1、E2）。修复核对一轮在 `86e4c8da` 上，三位各在新的工作树里：各核对自己的发现与自己范围里别人的，重跑自己存活过的变异与探针、另加新的（A：A-N3 的生产的改动的变异 h3–h5、f1、f2，新的子测试对一个让 page 的单元升级 acme 的锁的变异 h7、h8；B：b1–b10，两个探针再跑；C：13 个提交各自的门禁（整个模块的 lint 与 short、每个提交碰到的库上的测试、`2e954be5` 上网页的门禁），A-N3 的探针 F6、F6b、F6c，新的子测试是不是逼出来的 F7、F7c，子测试三次的用时，整个 e2e，读一次 CI 的作业） |
| 日期 | 2026-10-24（修复核对一轮到 2026-10-25） |
| 结论 | 没有 Important。Minor 六条，不同的五条（B-M1 = C-M1），都已改。记为中低的一条：`checkPages` 的 LLM Wiki 的规则的"根下的 `AGENTS`"一半没有反例，去掉 `a.parent_id IS NULL` 的变异过它的测试（B-M1 = C-M1）：S2 的守卫不许 `AGENTS` 离开根下，`checkPages` 是它后面的整个程序的网。其余四条是低，都是测试的空当：模板的段落对应的四种块的形状各没有反例（A-M1）；"在启动时解析一次"没有测试钉住（A-M2）；`TestATemplatesParseIsItsContents` 的夹具的 Markdown 什么也不提取，比较对任何正文都成立（A-M3）；没有测试读降级写下的 `updated_by_id`（B-M2）。细节 12 条，不同的 10 条（A-N1 = C-N2、B-N2 = C-N6）：九条在代码、测试与契约的文字里改，其中 A-N3 是这一轮唯一的生产的改动（主持者的决定 23：`Seed` 与 wiki 的 `Seeds` 端口交回写下的页的 id，模板交回写下的来源根）；C-N1 重跑、留下记录；C-N5 在修复一轮重量，7.5 改正（决定 26）。三位都没有找到答错的结果：A 在真的库上逼出建与工作区的删除、bob 的移出与降级在模板的单元重入 acme 的行时的三种交错，都没有死锁、没有半个创建；B 在真的库上核对两个管理员同时降级与 `NOTIFY` 之后回滚的降级；C 的每个提交单独都绿，整个 e2e 在四个提交上都通过。修复一轮由三位核对，没有新的发现（Important 0、Minor 0、Nit 0）。建成与修复一轮的两次 CI 运行都全绿，组合根三份的测试一步都在 7:30 之下，照 6.5 的规则不动。实施者的 15 条偏离由三位在各自的范围里判过，都对（A 改正第 3 条的理由）。主持者的 4 个决定见下（第 23–26 条，接着 P1 设计审查的第 1–22 条）。合并 `45d086a1` |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：A-M1–A-M3、A-N1–A-N4，B-M1、B-M2、B-N1、B-N2，C-M1、C-N1–C-N6；修复核对一轮三位都没有新的发现。修复一轮的条目照主持者的修复说明编号 1–8，第 6 项是九条细节。同一个发现合为一个修复：B-M1 与 C-M1；A-N1 与 C-N2；B-N2 与 C-N6） | — |
| B-M1 = C-M1 | 中低（测试） | `checkPages` 的 LLM Wiki 的规则（`bootstrap/interleavings_page_test.go:133`）说活着的 LLM Wiki 有它的来源根与"根下的 `AGENTS`"；`TestPageViolationsCatchEachLLMWikiGap`（`:156-222`）的各行删掉 `raw` 或 `AGENTS`、把两者改成附件、让来源根指别的笔记本的一页，照收普通的与删掉的 LLM Wiki，没有一行把 `AGENTS` 移离根下。B 的 b1、C 的 C1（去掉 `AND a.parent_id IS NULL`：`AGENTS` 在哪里都算）都过这个测试；规则的其余每一处都有一行钉住（C 逐个追过：`a.notebook_id`、`a.name_key` 由"AGENTS deleted"，`a.kind` 由"AGENTS an attachment"，来源根的三处由 rc12–rc14 与"raw deleted"）。S2 的 R1 不许移动 `AGENTS`，`checkPages` 是它后面的整个程序的网（`TestTheModeGuardReachesEveryPath`、`TestTheProtectionAndADowngrade` 以它结束）：这一半不查时，一个放 `AGENTS` 移到 `raw` 之下的守卫也过 `checkPages`。3.14 与 3.29 都没有列这一行 | 已改（`cb9de147`，修复一轮第 1 项）：表加"AGENTS under raw"一行（`UPDATE nodes SET parent_id = raw WHERE id = AGENTS`，以 `parent_id = NULL` 改回）；改回之后"什么也没坏"的核对照过，排序不用另改。变异：b1 让它失败（`fix1/item1.out`）。修复核对 B：b1 在 `86e4c8da` 上失败（"AGENTS under raw: the violations [] hold … = false, want true"），不带变异 2.57 秒通过；修复核对 C：C1 与 b1 都失败，这一行在它自己的提交上通过 |
| A-M1 | 低（测试） | `TestTheTemplatesCorrespond`（`modules/wiki/domain/template_test.go:131-136` 的反例）没有一个反例只靠块的形状被抓到：`blockOf`（`:59-76`）给每块一个形状（标题与它的级、围栏与它的信息串与行数、列表与它的项数、段落与它的行数），反例都被块数或行内代码抓到。"a list item gone"删掉的一项带着行内代码 `lint`，比较失败在代码上，不在项数上，3.9 与 3.29 却把它写成项数的反例。A 的变异 a3（不比项数）、a10（不比标题的级）、a11（不比围栏）、a12（不比段落的行数）都过这个测试：译者把两项并成一行、`##` 改成 `###`、去掉围栏的一行或把一段拆成两行时测试照绿。偏离 14 为块数补了一行，其余四种形状仍没有（13.4 第 1 条） | 已改（`d37f57aa`，第 3 项）：加 A 的四行，各保留行内代码："two items on one line"（列表的项数）、"a heading a level down"（标题的级）、"a fence a line short"（围栏的行数）、"a paragraph in two lines"（段落的行数）；注释写明每一行钉住什么。变异：a3、a10–a12 失败，a2（块数）与 a4（行内代码）照旧失败（`fix1/item3.out`）。修复核对 A 同；修复核对 C：每个 `strings.Replace` 都匹配（不匹配时 `zh` 不变、测试失败） |
| A-M2 | 低（测试） | 没有测试钉住"在启动时解析一次"（13.1 第 19 条与 13.3 第 3 条的新的例外）：两处能把解析挪进创建的变异都存活。a1（`Template.NotebookCreated` 对每页调 `t.seeds.Parse`，`modules/wiki/app/template.go:78`）过 `wiki/...` 的每个测试：`TestTheTemplateSeedsAnLLMWiki` 只在 `NewTemplate` 之后数解析（4），不在创建之后数；a13（`SeedWrites.Seed` 在每次 `CreatePage` 之前 `p.Facts = s.md.Facts(p.Content)`，`modules/page/app/seed_writes.go:45-52`）过 page 的 `Seed` 的测试与两个整个程序的测试：重新解析得到相同的事实。今天什么也不坏，但之后的改动能把解析挪进事务、挪到预算之外（`md.Facts` 不取预算），没有测试看得出 | 已改（`ec5e67b4`，第 4 项）：`TestTheTemplateSeedsAnLLMWiki` 在三次创建之后再数一次（仍是 4）；page 加 `TestASeedPassesTheFactsItIsGiven`（A 的探针，另先核对两份正文的事实不同）：以一份带 frontmatter 的别的正文的事实 `Seed` `raw`，观察者收到的是交来的。变异：a1、a13 失败（`fix1/item4.out`）。修复核对 A：a13 照 `75e34bb8` 改写过的循环重写，失败；修复核对 C 同 |
| A-M3 | 低（测试） | `TestATemplatesParseIsItsContents`（`modules/page/tree_writes_test.go:363-400`）比较的事实对任何正文都相等：page 的夹具以 `markdown.New(nil)` 建模块（`extension_test.go:182`），没有扩展，没有 frontmatter 的正文都解析成 `{frontmatter: absent, extracted: {}}`，测试里的 `[[Notes]]`、`#tag`、链接与任务什么也提取不出。A 的 a19（`ParseTemplate` 不看正文，答 `s.md.Facts("")`）过它、`Seed` 的测试与三个整个程序的测试（真的模板什么也不链接、不标）。今天的后果是零；之后一份带链接的模板会以错的事实进索引，没有测试失败 | 已改（`e21332db`，第 5 项）：夹具的 Markdown 是 `markdown.New([pageMark()])`，即 `TestTheContentsWritesReachTheUnit` 的随正文变的测试扩展；测试另核对种下的页的事实里 `page-mark` 是正文的字节数。变异：a19 失败（`fix1/item5.out`）。修复核对 A：a19 也让 `TestASeedPassesTheFactsItIsGiven` 失败（经它的"两份事实不同"的核对）；修复核对 C：类型断言不成立时答 0，不等于长度，测试照样失败 |
| B-M2 | 低（测试） | 没有测试钉住降级写下的 `updated_by_id`：3.10 第 5 步写"by the caller at that time"，读降级之后的行的测试（`notebook/extension_test.go:452` 读 `mode, source_root_id, updated_at`；`TestTheDowngradeAnswersInItsOrder` 与 `GET` 比；矩阵的变体的 `after` 读两列）都不读它，REST 的 `Notebook` 没有它。B 的 b2（生成的 `SetNotebookMode` 写 `updated_by_id = coalesce(updated_by_id, $3)`）过 notebook 的每个包、四个整个程序的降级的测试与矩阵的降级的行（`rB-mut/b2.log`，269 行 PASS）；替身的 `"SetMode standard by <alice>"` 只钉住用例交给端口的，不钉住 SQL 写的。情形：建的人之外的管理员降级，行仍记建的人是最后改它的 | 已改（`9daac068`，第 2 项）：`TestADowngradePassesTheModeSubscribers`（模块根，真库）在 alice 降级之前把这本 LLM Wiki 的 `updated_by_id` 设成 bob，之后读它：成功之后是 alice，失败的订阅者回滚之后仍是 bob（夹具的笔记本都是 alice 的，不先设成 bob，这个变异在这里也看不见）。变异：b2 让成功的一行失败（`fix1/item2.out`）。修复核对 B：b2 在"the subscribers follow"失败，回滚的一行也抓得到一个留下 alice 的回滚；修复核对 C 同 |
| A-N1 = C-N2 | 细节（测试） | 实施者留下的一项：`TestLLMWikiCreationWritesItsTemplate`（`bootstrap/wiki_template_test.go:114-118`）把 `min(c.message)` 读进 `string`，没有说明的变更集（第 12b 条变异）让它在读时失败（`cannot scan NULL into *string`），不在比较处；没有变更集时 `min(c.kind)`、`min(c.client)` 也是 `NULL`，`changesets != 1` 的那一句从不打出。该失败的照样失败，只是说得少 | 已改（`5008ee7d`，第 6 项，主持者的决定 25）：`kind`、`client`、`message` 都经 `coalesce(min(…), '')`，`bool_and` 经 `coalesce(…, false)`。变异：12b 在比较处失败，`1 changesets, of kind edit from api, message "", …; want one edit from api, "LLM Wiki 模板", …`（`fix1/item6a.out`、`fix1/item6a-m12b.log`）。修复核对 A（`h6.log`，三行）与 C（`rC2-m12b.log`）同 |
| A-N2 | 细节（替身） | notebook 的 app 的假的 `CreateNotebook`（`modules/notebook/app/fakes_test.go:91-95`）收下 `notebooks_source_root_check` 拒绝的行：偏离 13 让假的 `SetMode` 拒绝没有来源根的 LLM Wiki，`CreateNotebook` 没有。今天没有变异因此存活（去掉 `n.SourceRootID = uuid.NewV7()` 由 `TestCreateNotebookMakesTheCallerItsAdmin` 对存下的 id 的核对抓到；播种者的空的交回本就让建失败） | 已改（`3884692a`，第 6 项）：假的 `CreateNotebook` 与 `SetMode` 经同一个 `checked(n)` 拒绝表的两个检查拒绝的。变异：x7（建 LLM Wiki 不取来源根）让 `TestCreateNotebookMakesTheCallerItsAdmin` 以假的仓库的 `violates notebooks_source_root_check` 失败（`fix1/item6b.out`、`item6b.log`）。修复核对 A（`f4.log`）、B 同；`SetMode` 照旧只记下通过的，偏离 13 不变 |
| A-N3 | 细节（生产的代码） | `Template.NotebookCreated`（`modules/wiki/app/template.go:77-86`）在任何成功的 `Seed` 之后答 `c.SourceRootID`，不论它种下的页里有没有标着 `SourceRoot` 的；播种者的契约是"交回它写下的来源根"。今天成立只因为 `TestTheContractsAreTheModes` 钉住 `pages[0].SourceRoot`；契约的页没有这个标记时，建的 `seededItsRoot` 照过，一本 `llm_wiki` 带着一个不是节点的来源根提交，只有整个程序的测试与 `checkPages` 看得出 | 已改（`75e34bb8`，第 6 项，主持者的决定 23）：page 的 `TreeWrites().Seed`（与 `app.SeedWrites.Seed`）交回 `([]uuid.UUID, error)`，照 spec 的次序是它写下的页的 id；wiki 的 `Seeds` 端口与组合根的 `wikiSeeds` 照传。`Template.NotebookCreated` 交回 `Seed` 写下标着 `SourceRoot` 的那一页的 id，没有时是零；交回的 id 的个数不等于页数是缺陷。建的 `seededItsRoot` 因此比较笔记本的来源根与写下的页。测试：新的 `TestTheTemplateAnswersTheSourceRootWritten`（`wiki/app`）；`TestASeedJoinsItsCallersTransaction` 核对交回的 id，`TestATemplatesParseIsItsContents` 核对没有 id 的一页答它写下的新 id；`TestTheSeedersRunInTheirOrder` 不改（它测 notebook 的分发，第 7 条变异照旧让它失败）。变异：x8（模板又答创建的 id）、x9（`Seed` 答草稿的 id）失败（`fix1/item6c.out`）。修复核对 A：h5（组合根丢掉 `SourceRootID`，`raw` 以新的 V7 写下）在每一本 LLM Wiki 由建的 500 抓到，h4（page 一边不答 id）由模板的个数的缺陷抓到，h3 = x9、f1 = x8、f2（去掉个数的核对）都抓到；B 的 b8 同 h5：在 `66602404` 上这会提交一本 `source_root_id` 不指任何节点的笔记本；C 的 F6 同 h5，F6c（F6 加上把 `seededItsRoot` 放宽成 `len(roots) == 1`）让建提交、测试在"the root holds"与 `checkPages` 失败（500 就是那一处核对），F6b（F6 而组合根照旧答定下的 id，即 A-N3 之前）只由整个程序的核对抓到：这正是 A-N3 关上的空当。`Seed` 的闭包里的 `ids = ids[:0]` 是等价的（`Writer.Seed` 加入调用方的事务，不重跑闭包） |
| A-N4 | 细节（测试） | "建在先"里第二步排在测试持着的锁之后，放开之后再排在建的 `FOR SHARE` 之后；模板的单元重入 acme 的行时它多半已在等，但没有东西逼出它（`bootstrap/interleavings_wiki_test.go:9-13`、`:58-125`）。A 的逼出的探针通过（"核对过、没有问题的"）；建议收为第四个子测试，给设计在那里依靠的 PostgreSQL 的行为一个回归测试 | 已改（`ee464e7d`，第 6 项，主持者的决定 24）：`TestWorkspaceChangesAndCreatingAnLLMWiki` 的第四个子测试"the template's unit behind a queued deletion"：`notebook_members` 上的触发器等测试持着的 advisory lock（建停在管理员的插入之后、持着 acme 的行 `FOR SHARE`），删除发出、在 `workspaces` 上等（两处等待都轮询），然后放开：201 再 204，这本 LLM Wiki 的行随 acme 删掉，`checkPages`、`checkNotebooks`。A 的三种改动只取删除：三者在 acme 的行上排的都是同一个 `FOR NO KEY UPDATE`，钉住的是同一次重入。竞态下子测试 0.53 秒、测试 5.08 秒（第一个子测试带着容器的启动），名字不变，仍在第三份。修复核对 A：它是逼出来的，但没有一个我们的代码的变异让它失败：h7（page 的单元以 `FOR NO KEY UPDATE` 取 acme 的行，在删除排着时升级）照样通过（`h7b.log`），因为建是这一行唯一的持有者，PostgreSQL 不让唯一的持有者等在等待者之后；它是平台的回归测试，不是我们的锁序的检查。修复核对 C：F7（`wikiTemplate.NotebookCreated` 只在有后端等着 `workspaces` 的元组时放行）竞态下三次都通过，对照的 F7c 让 `TestLLMWikiCreationWritesItsTemplate` 的每次建答 500：删除在每次播种时都排着；用时 0.55 秒 × 3 |
| B-N1 | 细节（测试） | `TestNotebooksAnswerTheirModeOverMCP`（`bootstrap/mcp_account_test.go:73-96`）降级之前读一次 `notebooks`，之后只读 `open_notebook` | 已改（`670765c6`，第 6 项）：降级之前与之后经同一个帮手读 `notebooks`，之后两本都是 `standard`。修复核对 B：反向对照 b10（降级之后期待 `llm_wiki`）失败；C：x1 照旧被它抓到 |
| B-N2 = C-N6 | 细节（契约的文字） | `createNotebook` 的说明（`api/modules/notebook.yaml:38-45`，照 3.13 原样）说两次"the values are checked after both"，第二次的"both"跟在 LLM Wiki 的一句之后，不再指 404 与 403 | 已改（`2e954be5`，第 6 项）：结尾改为"…in one change; the mode and the template's language are checked with the other values."（C 的写法），`make gen` 重新生成 `api/dist/openapi.yaml` 与 `web/packages/api-client/src/schema.gen.ts`，别的没变；3.13 随之。修复核对 A、B：三处的文字相同，`make gen-check` 干净；C：`gen-check` 在这个提交上通过 |
| C-N1 | 细节（记录） | 实施者的原始记录少两条结果：`mut/s1.out` 记的 m07a、m07b 是第一次编译不过的写法（`BUILD-ERROR`），`x.out` 只有 x2–x4，x1 没有记录。C 以实施者自己的 spec 重跑：m07a 被 `TestTheSeedersRunInTheirOrder/the_first_refuses`、m07b 被 `TestACreationPassesTheSeeders/the_first_refuses`、x1 被 `TestNotebooksAnswerTheirModeOverMCP` 抓到 | 重跑（修复一轮第 7 项）：`fix1/rerun.out` 在修好的头上重跑 7.3 的第 1–22 条（26 次，含 a/b）、x1–x5、12b 与反向对照 rc01–rc20、rc21b，54 行都被抓到，m07a、m07b、x1 记在其中；rc11 照它原来的 spec 也被偏离 14 的一行抓到。修复核对 C：在头上再跑三者，都被抓到（`rC2-mut4.out`） |
| C-N3 | 细节（测试） | `TestTheTemplatesAreSmall`（`modules/wiki/domain/template_test.go:233`）以 `MaxTemplateBytes` 自己造"过 8 KiB 一个字节"的一行：把常量提到 64 KiB（C35）每行照绿，00-M10 4.18 的 8 KiB 只靠注释 | 已改（`9c2da5c7`，第 6 项）：测试写出界线（`const bound = 8 << 10`），三行：恰 8 KiB 的一页照收（`"# A\n"` 加 8,187 字节加一个换行）、8,193 字节的被拒、frontmatter 被拒。变异：C35 与反过来的 x10（`4 << 10`）都失败（`fix1/item6g.out`）。修复核对 A（f5、f6）与 C 同 |
| C-N4 | 细节（测试） | `pageNotebooks.StateOf`（`bootstrap/page_notebooks.go:21-23`）的"找不到"一支没有钉住：C40（条件只留 `err != nil`，找不到的笔记本答 `ok` 与零值）过 `TestNotebooksAnswerTheirModeOverMCP`、`TestOpenNotebookOfAnEmptyNotebook` 与 `TestTheInstructionsFollowTheAGENTSPage`；只有笔记本在门与 `StateOf` 之间被删时走到它（x4 以 page 的替身钉住用例的一半），坏了时 `open_notebook` 答 `"mode": ""`（契约的枚举之外）、接着读它的计数 | 已改（`86e4c8da`，第 6 项）：新的 `TestPageNotebooksConvertTheState`（`bootstrap/page_notebooks_test.go`，不要库：一个嵌着 `notebook.Notebooks` 的替身）：LLM Wiki 与普通的状态逐字段转换，找不到答 `ok` 为假与零值，错误原样。C40 失败（`fix1/item6h.out`）。代码做的：`StateOf` 找不到时 `OpenNotebook` 答 `domain.ErrNotebookNotFound`（`page/app/open_notebook.go:63-64`），即门对没有的笔记本答的值（`decideNotebook`，`authorize.go:38-39`），MCP 的适配器照传，`toolError` 认作 `notebook.not_found`，即 REST 的 404（`mcp_reads_test.go:453` 对删了的笔记本钉住）；`TestTheOpenNotebookAnswer` 另核对适配器原样答 `ErrNotebookNotFound`。修复核对 B：b9（转换丢掉 `SourceRootID`）也被它的 LLM Wiki 一行抓到；C：它在第二份 |
| C-N5 | 细节（估计） | 7.5 的第三份的估计没有算 `wlab` 给 `prepareMatrix` 的每个用者的代价：第三份本机多 13.1 秒，它的三个新测试只有 4.9 秒；CI 上 6:13 对 7.5 的底 5:41（+32 秒，一次）。多出的是 `prepareMatrix` 种 `wlab`：第三份五个测试调它（`TestTheNotebookListIs…`、`TestThePermissionMatrixOverMCP`、`TestTheSearchIs…`、`TestTheStreamSees…`（另读 `wlab` 的笔记本，偏离 8）、`TestTheTreeIs…`），第二份三个，7.5 只算 `TestPermissionMatrix` 的变体的行。照第三份的 CI 与本机之比（1.91），S1 用掉 7.5 的"约一分钟"的约 25 秒，P1 的第三份的十一个测试还有八个没来；加上 7.5 的 S2、S3 的数，P1 结束时第三份约 6:45–7:00，仍在 7:30 之下 | 修复一轮第 8 项重量，7.5 改正（主持者的决定 26）：第二、三份单独各两次（下面"组合根的各份"）：S1 带修复在本机比起点多约 7–9 秒与 8–13 秒，按 S1 自己的 CI 与本机之比约 17–23 秒与 15–25 秒，即 C 说的第三份的约 25 秒，C 的推算成立。修复核对 C 核对 |
| 存疑 | 细节（存疑） | B：一个降级可以等在一串 `FOR SHARE` 之后：笔记本行上新的 `FOR SHARE` 在现有的持有者相容时不排在等着的 `FOR NO KEY UPDATE` 之后，一本不停有页的写的笔记本能让降级等到请求的期限。这是 M3 起的形状（改名、改开放程度与删除经同一个 `manager.lock`），没有量；记下，因为降级是第一个也会被一个写入任务（它的各批）挡住的管理的写。C：没有别的；C36（降级的事务里发两次 `mode`）是等价的：PostgreSQL 把一个事务里相同的 `NOTIFY` 合成一条，两个事务里的两次降级由 `TestDowngradeNotebookRefuses` 的"找到时 `llm_wiki`、锁下 `standard`"一行守着 | 记下、不改（"接受与推后的"） |
| 偏离 | — | 实施者的 15 条偏离（[01-P1](../01-P1-contract-frame.md) 10.1、3.30）：1 没有 `TestCreateNotebookLogs`，建的一行在 `TestCreateNotebookMakesTheCallerItsAdmin` 里核对；2 `TemplateMessage` 在 `wiki/domain`；3 `TestLLMWikiCreationWritesItsTemplate` 另数 `indexed_pages`；4 `TestPageViolationsCatchEachLLMWikiGap` 在 `interleavings_page_test.go`，每一行改回之后再断言为空；5 没有 `page/domain/mode_test.go`；6 调用列表照已有的替身、不带 id；7 矩阵的变体以它自己的表种进 `seeded.wiki`；8 另两个读每个工作区的测试读 `wlab`；9 `targetViolation` 取行的列所在的工作区的 slug；10 变体的 `raw` 与 `AGENTS` 是英文模板的两页；11 `TestImportsAndDowngradingAnLLMWiki` 两种次序都持住导入的 worker；12 `TestTheModeEventReachesTheStreams` 的私有的 LLM Wiki、读者与看不到它的成员；13 假的 `SetMode` 拒绝表拒绝的；14 rc11 不承重，`TestTheTemplatesCorrespond` 加"末尾多一段"，第 4 步改正、第 4–7 步重放；15 LW1 (API) 以 `Promise.all` 建三本 | 三位在各自的范围里判过，都对：A 判 1、2、3、14（第 1 条另要改 3.10 的"照 `TestCreateNotebookLogs` 的写法"；第 3 条无害，但理由不对：一份没有解析过的模板不会悄悄通过，链接索引的观察者拒绝不是 Markdown 的事实（`linking/adapter/markdown/facts.go:36-44`），建会答 500；这一数加的是链接索引对模板的两页的最后一跳，不钉住事实是正文的，那是 A-M3；第 14 条照它的范围对，其余的形状见 A-M1），B 判 5–13（第 7 条比设计的写法好：`matrixWorkspaces()` 有别的用者，`seeded.notebooks` 按名称取，加进去会撞；第 11 条是要的，不是装饰），C 判 4、15，对其余的没有异议 |
| 修复一轮的偏离与发现 | — | 实施者在修复一轮一次在 `heavy.sh` 之外以 `go test -run` 在 `./internal/bootstrap` 上跑 `TestPageNotebooksConvertTheState`（这个包没有 `TestMain`，选的测试不要库，没有起容器）；之后每次都经 `heavy.sh`。a13 在 `75e34bb8` 之后不再照原样施加（循环改写了），修复核对 A、C 照新的循环重写 | 修复核对判过：无害（C）；重写的 a13 失败 |

## 主持者的决定

负责人可以改判（接着 [P1 设计审查](P1-design-review.md)的"主持者的决定"第 1–22 条编号）：

23. **修复一轮的处置**（`fix-brief-1.md`，2026-10-24）：每条 Minor 与 Nit 照审查者的建议改，一组一个提交，各自绿：B-M1 = C-M1 加"AGENTS under raw"一行；B-M2 读 `updated_by_id`；A-M1 加四种形状的反例；A-M2 在两处钉住"解析一次"；A-M3 用 `pageMark()`；A-N1 = C-N2 经 `coalesce`（决定 25）；A-N2 假的 `CreateNotebook` 照表拒绝；A-N4 收为子测试（决定 24）；B-N1 降级之后再读 `notebooks`；B-N2 = C-N6 去掉重复的一句、重新生成，3.13 随之；C-N3 写出 8 KiB；C-N4 测 `pageNotebooks.StateOf` 的"找不到"，并看 `open_notebook` 对门之后被删的笔记本答什么（现有的 404）；C-N1 重跑、留下记录；C-N5 重量（决定 26）。其中唯一的生产的改动是 A-N3：page 的 `TreeWrites().Seed` 与 wiki 的 `Seeds` 端口交回写下的页的 id（照 spec 的次序），模板交回它真的写下的来源根的 id（没有时为零，个数不对是缺陷），所以建的"恰有一个、等于先定下的"说的是写下的；`TestTheSeedersRunInTheirOrder` 照旧失败得了（[01-P1](../01-P1-contract-frame.md) 3.4、3.6、3.8，00 号文档 4.4 与第 8 节）。
24. **A-N4 的逼出的子测试留作 PostgreSQL 的锁的回归测试**：模板的单元在排着的工作区删除之后重入 acme 的行，PostgreSQL 把已持的锁交给唯一的持有者，不让它等在更强的等待者之后。它不是我们的锁序的测试：修复核对 A 找到我们的代码没有一处改动让它失败（h7：page 的单元在删除排着时以 `FOR NO KEY UPDATE` 升级 acme 的行，照样通过：唯一的持有者从不等在等待者之后）。子测试的注释说 PostgreSQL 的反事实，照实；3.15 与 7.2 照此写（7.2 第 8 条）。
25. **实施者留下的说明的读由 `coalesce` 改正**（A-N1 = C-N2）：`TestLLMWikiCreationWritesItsTemplate` 的 `kind`、`client`、`message` 经 `coalesce(min(…), '')`，没有说明或没有变更集时在比较处失败、说出是什么；第 12b 条变异照此在比较处被抓到（3.16、7.3）。
26. **第三份到 P1 结束约 6:45–7:00**（C-N5）：7.5 的估计（约 40–50 秒，5:41 之后约 6:20–6:30）没有算 `prepareMatrix` 在每个用者里种 `wlab`（第三份五个、第二份三个用者）；S1 带修复在第三份的 CI 上多约 15–25 秒，比它的三个新测试多。7.5 照此改正，S2、S3 的估计算上它；每个 Step 合并之后照 6.5 读 CI（两次的平均，第 10 节记下），不提前挪界线。

## 接受与推后的

- **接受**：
  - A-N4 的子测试钉住的是 PostgreSQL，不是我们的代码（决定 24）。
  - 等价或无害的变异照留：C36（一个事务里两个相同的 `NOTIFY` 合成一条）；`Seed` 的闭包里的 `ids = ids[:0]`（`Writer.Seed` 不重跑闭包，修复核对 C）。
  - B 的存疑（降级等在一串 `FOR SHARE` 之后）记下、不改：同改名、改开放程度与删除的 `manager.lock`，M3 起如此，没有量。
  - 偏离 3 的理由照 A 改正（[01-P1](../01-P1-contract-frame.md) 10.1、3.30）。
  - 修复一轮的运行的组合根第一份 7:12（建成时 5:11）：第一份没有 S1 的测试，这一轮也没改它的测试，第二、三份反过来快了（6:19 → 5:12、6:13 → 5:07），M9/P4 记过同样的测试在 4:49–7:12，读作 CI 的波动（修复核对 C）。两次的平均 6:11.5、5:45.5、5:40，都在 7:30 之下。
  - CI 的作业日志要认证：每个测试在 CI 上的用时没有记下，只有每一步的时间。
  - 流程（照实写下）：
    - 实施者建成时的两处疏忽，都没有改结果：`make test` 在工作树里编译时，在同一个工作树里重跑了短的另加的变异 x2–x4（各施加几秒）；一个变异只会让测试失败，那次全绿，之后的 `test-go-short`、e2e、份的用时与 CI 都从干净的树建（C：编进依赖它的包的测试二进制的变异没有被找过，所以"那次绿"本身证明不了每个二进制跑的是干净的代码，但 CI 在干净的 `66602404` 上绿，C 自己在头上的运行也干净：无害）；一次读 CI 的运行在上一次之后 49 秒，在两分钟的间隔之内。
    - 实施者的另加的反向对照的第一次与份的用时同时：rc17 等 `heavy.sh` 的锁超时（不算结果），实施者按 PID 停下那一次、以 helper 自己的备份复原文件，份的用时之后重跑；rc21b 的第一次同样等在起点的份的用时之后，重跑。
    - 修复一轮的一次在 `heavy.sh` 之外的运行（上表"修复一轮的偏离与发现"）：不要库，无害。
    - 实施者与审查者都没有用 Monitor 起监视，后台的命令都结束了；`heavy.sh` 一直在。
- **推后**：
  - S2、S3：第三份的估计算上 `wlab` 的种（决定 26）；S2 的变体的行（3.22）落在第二份的 `TestPermissionMatrix`，7.5 已算。
  - 6.5 的两次的平均在合并之后读（第 10 节记下）。
  - 没有交给 v0.1 之后的打磨的。

## 反向对照

建成时以 `mut.py` 跑（库的经 `heavy.sh`，每个之后由 helper 复原），7.3 的 spec 与输出在 `mut/s1.json`、`mut/s1.out`，反向对照在 `mut/rc.json`、`mut/rcx.json`，另加的在 `mut/x.json`；修复一轮的在 `fix1/`，在修好的头上重跑的在 `fix1/rerun.out`（54 行，都被抓到）。这些文件都在 `scratchpad/m10/p1s1/` 下；审查者的（`rA-mut.py`、`rB2-mut/`、`rC2-mut*.out` 之类）在 `scratchpad/m10/p1s1-review/` 下。

**7.3 的第 1–22 条**（建成时都被点名的测试抓到；修复一轮在 `86e4c8da` 上重跑 26 次，含第 5、7、11、16 条的 a/b 与 12b，都被抓到）：

| # | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| 1 | 建的事务先发可见性、后调播种者 | `TestCreateNotebookMakesTheCallerItsAdmin`（调用次序） | 抓到 |
| 2 | 播种者拿到第二次读的时钟 | `TestACreationPassesTheSeeders` | 抓到（每一行）；C 的 C3 在整个程序里也抓到（三本 LLM Wiki） |
| 3 | 不核对播种者交回的来源根 | 同上 | 抓到（四行 500） |
| 4 | 核对写成"至少一个" | 同上 | 抓到（两个都交回的一行） |
| 5 | `raw` 的 id 在 `Seed` 里另取一个 V7 | `TestTheTemplateSeedsAnLLMWiki`；`TestLLMWikiCreationWritesItsTemplate` | 两者都抓到 |
| 6 | `CheckDraft` 只在 `llm_wiki` 时核对语言 | `TestCheckDraftsModeAndLanguage` | 抓到 |
| 7 | `seed` 照原样交回播种者的问题 | `TestTheSeedersRunInTheirOrder`；`TestACreationPassesTheSeeders` | 两者都抓到（各自的 `the_first_refuses`）。第一种写法留下没用的 import、编译不过，改成以 `%w` 包着问题；原始记录是编译不过的那一次（C-N1），C 与修复一轮重跑 |
| 8 | `Seed` 照 `Run` 开自己的事务 | `TestASeedJoinsItsCallersTransaction` | 抓到 |
| 9 | `run` 照旧读时钟 | `TestASeedReadsNoClock` | 抓到；C 的 C2 在整个程序里也抓到（真的时钟分得出两次读） |
| 10 | `ClientOf` 对没有 `Actor` 的 ctx 答 `web` | `TestClientOfIsTheCredential` | 抓到 |
| 11 | 模板先写 `AGENTS` | `TestTheTemplateSeedsAnLLMWiki`；`TestLLMWikiCreationWritesItsTemplate` | 两者都抓到 |
| 12a | 去掉 `Template.NotebookCreated` 的 `Message:`（组合根于是交 `&""`） | `TestLLMWikiCreationWritesItsTemplate`；e2e 的 LW1 (API) | 两者都失败，但因建的 500：`changesets_message_check` 拒绝空的说明（`m12e2e.out`） |
| 12b | `wikiSeeds.Seed` 交 `Message: nil`（库照收的一行） | 同上 | 两者都抓到：LW1 (API) 比较说明（`"message": null`，`m12be2e.out`）；Go 的测试建成时在读说明时失败（`cannot scan NULL into *string`），修复一轮之后在比较处（`message ""`，决定 25） |
| 13 | 段落对应只比块数 | `TestTheTemplatesCorrespond` | 抓到（行内代码的一行） |
| 14 | `ListOwnerless` 的手写的转换漏了 `Mode` | `TestTheNotebooksAnswerTheirMode` | 抓到；B 的 b3、C 的 C34 是 `ListNotebooks` 的那一处，同样抓到 |
| 15 | 降级先看任务、后看模式 | `TestDowngradeNotebookRefuses` | 抓到 |
| 16 | `SetNotebookMode` 留着 `source_root_id`（生成的 SQL 写 `coalesce($2, source_root_id)`） | `TestTheDowngradeAnswersInItsOrder`；`TestPermissionMatrix` 的准备与降级的行 | 两者都抓到（变体的 `after`）；C 的 C10 重跑同 |
| 17 | 降级在锁之前判定 | `TestDowngradeNotebookMakesItStandard` | 抓到 |
| 18 | 降级看锁之前读的模式 | `TestDowngradeNotebookRefuses` | 抓到（找到时 `llm_wiki`、锁下 `standard`）；C 的 C5 重跑同 |
| 19 | 不登记模式的订阅者 | `TestTheModeEventReachesTheStreams` | 抓到；C 的 C6 重跑同（11.6 秒，等在流上） |
| 20 | `checkPages` 的新规则不看 `r.deleted_at` | `TestPageViolationsCatchEachLLMWikiGap` | 抓到 |
| 21 | 新规则不看 `n.deleted_at` | 同上 | 抓到；C 的 C7 重跑同 |
| 22 | `targetViolation` 只认 `lab` | `TestThePermissionMatrixCoversEveryOperation` | 抓到 |

**另加的变异**（建成时；修复一轮重跑，都被抓到）：

| # | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| x1 | `pageNotebooks.StateOf` 总答普通 | `TestNotebooksAnswerTheirModeOverMCP` | 抓到（建成时经 `heavy.sh`、没有留下记录；C 重跑，`fix1/rerun.out` 记下） |
| x2 | MCP 的 `notebooks` 总答 `standard` | `TestTheNotebooksAnswerTheirItems` | 抓到；C 的 E2 在构建的二进制上让 LW1 (MCP) 失败（MC1 通过：它的笔记本都是普通的） |
| x3 | MCP 的 `open_notebook` 总答 `standard` | `TestTheOpenNotebookAnswer` | 抓到；C 的 E1 同 E2 |
| x4 | `OpenNotebook` 不管门之后被删的笔记本 | `TestOpenNotebookAnswersInOrder` | 抓到 |
| x5 | 模板的说明总是英文的 | `TestLLMWikiCreationWritesItsTemplate` | 抓到（中文的一行，`mut/m12b.out`） |

**3.29 的 S1 的反向对照**（去掉它所测的检查；与 7.3 同一处改动的记在那里：`ClientOf` 第 10 条，播种者的交回第 3、4 条，播种者的问题第 7 条，`Seed` 的时钟第 9 条，降级的次序第 15 条，锁下的模式第 18 条，`checkPages` 的 `deleted_at` 第 20、21 条，五条查询第 14 条）：

| # | 去掉的检查 | 跑的测试 | 结果 |
|---|---|---|---|
| rc01 | 00048 的 `notebooks_mode_check` | `TestChecksRejectCounterexamples` | 抓到（第三个模式与大写的两行） |
| rc02 | Down 留着 `source_root_id` | `TestTheNotebooksModeGoesDown` | 抓到 |
| rc03 | "no seeders"的 panic | `TestTheModuleRefusesNoSeeders` | 抓到 |
| rc04 | "no writing jobs"的 panic | `TestTheModuleRefusesNoWritingJobs` | 抓到 |
| rc05 | `Seed` 收没有事务的 ctx | `TestASeedRefusesNoTransactionAndNoTime` | 抓到 |
| rc06 | `Seed` 收零的时刻 | 同上 | 抓到 |
| rc07 | 模板的大小的界线 | `TestTheTemplatesAreSmall` | 抓到 |
| rc08 | frontmatter 与标题 | 同上 | 抓到 |
| rc09 | 找 `[[` | `TestTheTemplatesLinkNothingOutsideCode` | 抓到 |
| rc10 | 找 `#tag` | 同上 | 抓到 |
| rc11 | 段落对应的块数 | `TestTheTemplatesCorrespond` | 第一次没抓到：三个反例也都被逐块的比较抓到（去掉块数时循环照英文的块走，中文末尾多出的一块看不见，少一块则在第一处错开的块失败）。加"末尾多一段"之后抓到（偏离 14），在 `66602404` 上再跑一次也抓到 |
| rc12 | 规则的 `r.kind = 'page'` | `TestPageViolationsCatchEachLLMWikiGap` | 抓到（`raw` 改成附件） |
| rc13 | 规则的 `a.kind = 'page'` | 同上 | 抓到（`AGENTS` 改成附件） |
| rc14 | 规则的 `r.notebook_id = n.id` | 同上 | 抓到（来源根是别的笔记本的一页） |
| rc15 | `targetViolation` 只认 `lab` | `TestMatrixViolationsCatchesEachGap` | 抓到 |
| rc16 | 事件流是模式的订阅者 | `TestTheModeEventReachesTheStreams` | 抓到 |
| rc17 | 00048 的 `notebooks_source_root_check`（与它的 Down） | `TestChecksRejectCounterexamples` | 抓到（第一次等锁超时，不算） |
| rc18 | `CheckDraft` 的语言 | `TestCheckDraftsModeAndLanguage` | 抓到 |
| rc19 | 建管播种者的错误 | `TestTheTemplateRollsBackWithItsNotebook` | 抓到（日志没有触发器的话） |
| rc20 | 测试的触发器拦 `AGENTS`（改拦 `raw`） | 同上 | 抓到（序列数到 1） |
| rc21 | 降级看锁与判定之前找到的模式（写者在普通的笔记本上答 409） | `TestDowngradeNotebookRefuses` 的"an editor, a standard notebook" | 抓到（rc21b）；先对 `TestTheDowngradeAnswersInItsOrder` 跑时照过：那个测试没有写者的一行，本就该照过 |

**修复一轮的变异**（`fix1/`；写成 [01-P1](../01-P1-contract-frame.md) 7.3 的第 65–80 条，S2–S4 照旧是第 23–64 条）：

| # | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| b1（65） | `checkPages` 的新规则不看 `a.parent_id` | `TestPageViolationsCatchEachLLMWikiGap` | 抓到（之前没有；第 1 项） |
| b2（66） | `SetNotebookMode` 留着 `updated_by_id`（`coalesce(updated_by_id, $3)`，生成的 SQL） | `TestADowngradePassesTheModeSubscribers` | 抓到（之前没有；第 2 项） |
| a3（67） | 不比列表的项数 | `TestTheTemplatesCorrespond` | 抓到（之前没有；第 3 项） |
| a10（68） | 不比标题的级 | 同上 | 抓到（之前没有） |
| a11（69） | 不比围栏的信息串与行数 | 同上 | 抓到（之前没有） |
| a12（70） | 不比段落的行数 | 同上 | 抓到（之前没有） |
| a2、a4 | 不比块数；不比行内代码 | 同上 | 抓到（同前） |
| a1（71） | `Template.NotebookCreated` 每次创建再解析 | `TestTheTemplateSeedsAnLLMWiki` | 抓到（之前没有；第 4 项） |
| a13（72） | `SeedWrites.Seed` 写之前再解析 | `TestASeedPassesTheFactsItIsGiven` | 抓到（之前没有） |
| a19（73） | `ParseTemplate` 答 `s.md.Facts("")` | `TestATemplatesParseIsItsContents` | 抓到（之前没有；第 5 项） |
| x7（74） | 建 LLM Wiki 不取来源根的 id | `TestCreateNotebookMakesTheCallerItsAdmin` | 抓到：假的仓库的 `violates notebooks_source_root_check`（A-N2） |
| x8（75） | 模板答创建的 id，不答写下的 | `TestTheTemplateAnswersTheSourceRootWritten` | 抓到（A-N3） |
| x9（76） | `SeedWrites.Seed` 答草稿的 id，不答写下的 | `TestATemplatesParseIsItsContents` | 抓到（A-N3） |
| f2（77） | 模板不核对 `Seed` 交回的 id 的个数（修复核对 A 的） | `TestTheTemplateAnswersTheSourceRootWritten` | 抓到（"one id for two pages"一行） |
| C35（78） | `MaxTemplateBytes = 64 << 10` | `TestTheTemplatesAreSmall` | 抓到（之前没有；C-N3） |
| x10（79） | `MaxTemplateBytes = 4 << 10` | 同上 | 抓到 |
| C40（80） | `pageNotebooks.StateOf` 对找不到的答 `ok` | `TestPageNotebooksConvertTheState` | 抓到（之前没有；C-N4） |
| 12b | 同上表 | `TestLLMWikiCreationWritesItsTemplate` | 在比较处抓到（`message ""`；之前在读时） |

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，修复一轮之后都被抓到）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | a1（`NotebookCreated` 再解析）、a13（`Seed` 再解析） | 存活：A-M2 |
| A | a2（不比块数，rc11 再一次）、a4（不比行内代码） | 抓到 |
| A | a3、a10、a11、a12（不比项数、标题的级、围栏、段落的行数） | 存活：A-M1；对 A 的形状的探针各被抓到 |
| A | a5（`ClientOf` 总答 `api`）、a6（处理函数的客户端总是 `web`）、a7（`seed` 留下零的根）、a8（默认的语言 zh-CN）、a9（没有"no seeders"的 panic） | 抓到 |
| A | a14（`wikiSeeds` 总交 `web`） | 抓到（PAT 的一行） |
| A | a15–a18（链接、标签、大小、标题的检查） | 各被抓到 |
| A | a19（`ParseTemplate` 不看正文） | 存活：A-M3；对 A 的解析的探针被抓到 |
| B | b1（规则不看 `a.parent_id`） | 存活：B-M1 |
| B | b2（`SetNotebookMode` 留着 `updated_by_id`） | 存活：B-M2 |
| B | b3（`ListNotebooks` 的转换漏了 `Mode`）、b4（`Writing` 漏了 `revert`） | 抓到 |
| B | b5（`notebook.change_mode` 给读者） | 抓到（矩阵的变体的读者与编辑者的格） |
| B | b6（模式的订阅者在 `SetMode` 之前） | 抓到（两个子测试） |
| B | b7（`Writing` 在事务之前读） | 抓到（"the start first"答 200、不是 409：这个交错是逼出来的、有牙） |
| C | C1（规则不看 `a.parent_id`） | 存活：C-M1 |
| C | C2、C3（第 9、2 条在整个程序里） | 抓到（三本 LLM Wiki） |
| C | C4（来源根的检查写成 `(mode = 'llm_wiki') = (source_root_id IS NOT NULL)`） | 抓到（只有 `TestTheNotebooksModeGoesDown`：带来源根的普通的一行） |
| C | C5、C6、C7、C10（第 18、19、21、16 条重跑） | 抓到 |
| C | C8（`mode` 不带 `NotebookID`） | 抓到（1.4 秒） |
| C | C9a、C9b（`publishMode` 在 `SetMode` 之前，用例与模块根） | 抓到（两个用例的测试；模块根的两行） |
| C | C14（`createNotebook` 的处理函数总交 `web`） | 抓到（`TestCreateNotebookTakesItsModeAndClient`） |
| C | C17（`WritingJobs` 在调用方的事务之外读） | 抓到（事务里的一行） |
| C | C21（`notebook.not_llm_wiki` 不在 `problemMessages`）、C22（它的中文去掉） | 抓到（`problem-messages.test.ts`；`check:types`） |
| C | C34（`ListNotebooks` 的转换漏了 `Mode`） | 抓到 |
| C | C35（`MaxTemplateBytes` 为 64 KiB） | 存活：C-N3 |
| C | C36（降级的事务里发两次 `mode`） | 通过：等价（上表"存疑"） |
| C | C40（`StateOf` 对找不到的答 `ok`） | 存活：C-N4 |
| C | E1、E2（构建的二进制的 `open_notebook`、`notebooks` 总答 `standard`） | LW1 (MCP) 失败，MC1 通过（它的笔记本都是普通的） |
| C | m07a、m07b、x1（实施者的 spec 重跑） | 抓到（C-N1） |

**修复核对一轮**（在 `86e4c8da` 上）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | a1、a3、a10、a11、a12（存活过的）；a2、a4–a9 | 都抓到 |
| A | f1（= x8）、f2（去掉 id 的个数的核对）、f3（= 第 7 条）、f4（= x7）、f5（= C35）、f6（= x10） | 都抓到（f2 由"one id for two pages"一行） |
| A | h1（= a13，照新的循环重写）、h2（= a19）、h3（= x9） | 都抓到 |
| A | h4（page 一边不答 id） | 抓到（模板的个数的缺陷） |
| A | h5（组合根丢掉 `SourceRootID`，`raw` 以新的 V7 写下，模板答那个 V7） | 抓到（每一本 LLM Wiki 由建的 500） |
| A | h6（= 12b） | 在比较处抓到（`h6.log`，三行） |
| A | h7、h8（page 的单元以 `FOR NO KEY UPDATE` 取 acme 的行：对逼出的子测试、对没有逼出的子测试） | 都通过：子测试钉住的是 PostgreSQL（决定 24） |
| B | b1、b2（存活过的）；b3–b7 | 都抓到 |
| B | C40；b9（转换丢掉 `SourceRootID`） | 抓到（`TestPageNotebooksConvertTheState`） |
| B | b8（`wikiSeeds.Seed` 丢掉定下的 id，`raw` 以新的 V7 写下） | 抓到：`TestLLMWikiCreationWritesItsTemplate` 与 `TestNotebooksAnswerTheirModeOverMCP` 得 500（在 `66602404` 上会提交一本来源根不指任何节点的笔记本） |
| B | b10（B-N1 的反向对照：降级之后期待 `llm_wiki`） | 抓到 |
| C | C1 与 b1；C35 与 x10；C40 | 都抓到 |
| C | a1、a2、a3、a4、a10–a12；a13（照新的循环重写）；a19、x9；x7、x8；b2；m07a、m07b、x1 | 都抓到 |
| C | F6（组合根丢掉 `p.ID`） | 抓到（三次建都答 500） |
| C | F6c（F6 加上 `seededItsRoot` 放宽成 `len(roots) == 1`） | 建提交，测试在"the root holds"与 `checkPages` 的"3 live LLM Wikis without their source root"失败：500 就是那一处核对 |
| C | F6b（F6 而组合根照旧答定下的 id，A-N3 之前的行为） | 建的核对放过，只有整个程序的核对抓到：A-N3 关上的空当 |
| C | F7（只在有后端等着 `workspaces` 的元组时放行模板）、F7c（同一个探针对 `TestLLMWikiCreationWritesItsTemplate`） | F7 竞态下 `-count=3` 都通过（删除在每次播种时都排着）；F7c 每次建答 500：探针分得出两种状态 |
| C | C2–C10、C14、C17、C21、C22、C34 重跑；C36 | 都抓到；C36 照旧等价 |

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `86e4c8da`（修复一轮 `cb9de147`–`86e4c8da`，13 个提交），三位各在新的工作树里 | A：自己的七条都改了，范围里的 B-N2 = C-N6、C-N2、C-N3 也是；A-N3 的生产的改动：调用者只有组合根的 `wikiSeeds`，错误照传（第 7 条变异照旧失败），id 的个数不对是缺陷，h3–h5、f1、f2 都抓到，`TestACreationPassesTheSeeders` 的"另一个 id"一行照旧钉住建的拒绝；新的子测试是逼出来的、在时间之内（0.56 秒，测试 5.41 秒），但钉住的是 PostgreSQL（h7、h8 通过）；改了的测试与替身照旧失败得了；快的包与经 `heavy.sh` 的模块根、组合根的测试都通过。B：四条都改了，各在真的库上核对；C-M1、C-N4、C-N6、A-N2 也是；A-N3 从建的一边看：`seededItsRoot` 不改，`CreatePage` 留着交来的 id，模板能答一个建没有定下的根只在一页以别的 id 写下时，b8 证明那时建答 500；两个探针（两个管理员同时降级、回滚的降级）再跑照旧；`TestImportsAndDowngradingAnLLMWiki` 两种次序、`TestWorkspaceChangesAndCreatingAnLLMWiki` 的四个子测试（新的 0.74 秒）都通过。C：七条都改了，A、B 在 C 的范围里的十条也是；13 个提交各自 build、vet、整个模块与 `server/tools` 的 lint（0 个问题）、tidy、short 测试与每个提交碰到的库上的测试都通过，`2e954be5` 上 `make gen-check` 与网页的门禁通过（vitest 166 个文件、3,346 个测试）；`make e2e` 284 个 36.9 秒；`go test -race -list` 420 个、168 / 117 / 135；读一次 CI 运行 40107114331 的作业，全绿；报告的"设计该记下的改动"盖住每一处碰到设计的修复。三位：Important 0、Minor 0、Nit 0 | 合并（见 00 号文档第 12 节） |

## 实测

- **模板的字节**（`wc -c server/internal/modules/wiki/domain/template-*.md`）：英文的 `raw` 648、`AGENTS` 2,209，中文的 `raw` 593、`AGENTS` 2,107，共 5,557，同 3.9 的四个数（A 的 `tpl.py` 逐字节比过；C 在头的 blob 上量同样）；测试钉住每页 8 KiB（修复一轮起写出 `8 << 10`）。
- **八个新的整个程序的测试**（建成时，`go test -race -count=1 -v -run '^(…)$' ./internal/bootstrap` 经 `heavy.sh`，开始时负载 1.8）：包 11.9 秒，测试本身 10.3 秒。第二份的五个共 5.4 秒（`TestPageViolationsCatchEachLLMWikiGap` 1.95、`TestImportsAndDowngradingAnLLMWiki` 1.66（0.93 与 0.73）、`TestTheDowngradeAnswersInItsOrder` 0.69、`TestLLMWikiCreationWritesItsTemplate` 0.59、`TestNotebooksAnswerTheirModeOverMCP` 0.52），乘 1.73 约 9.4 秒；第三份的三个共 4.9 秒（`TestWorkspaceChangesAndCreatingAnLLMWiki` 3.06，三个子测试各 1.02；`TestTheModeEventReachesTheStreams` 1.31；`TestTheTemplateRollsBackWithItsNotebook` 0.49），乘 1.73 约 8.4 秒。`TestPermissionMatrix` 带变体的行单独跑，包 21.0 秒。审查 B 量的：`TestTheDowngradeAnswersInItsOrder` 0.72、`TestImportsAndDowngradingAnLLMWiki` 1.88（0.96 与 0.92）、`TestTheModeEventReachesTheStreams` 1.49、`TestPageViolationsCatchEachLLMWikiGap` 2.18 秒。
- **修复一轮改了的测试**（竞态下，`fix1/shards.out`）：`TestPageViolationsCatchEachLLMWikiGap` 2.09 秒、`TestWorkspaceChangesAndCreatingAnLLMWiki` 3.70 秒（之前 3.06；新的子测试 0.53，修复核对 A 0.56、B 0.74、C 三次 0.55）、`TestNotebooksAnswerTheirModeOverMCP` 0.52、`TestLLMWikiCreationWritesItsTemplate` 0.56、`TestPageNotebooksConvertTheState` 0.00 秒。
- **e2e**（不带竞态）：建成时 `make e2e` 284 个 55.4 秒（LW1 (API) 50 毫秒、LW1 (MCP) 29 毫秒、MC1 989 毫秒）；审查 C 284 个 36.4 秒（41、30、237 毫秒），第 2、4、6 步 282、283、283 个（36.4、36.9、36.1 秒）；修复一轮 284 个 36.0 秒；修复核对 C 284 个 36.9 秒。

## 组合根的各份

照 6.5 以 `-race` 列出（`BOOTSTRAP_FIRST = ^Test([A-D]|E([a-u]|very[A-R]))`、`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])`）：建成时 419 个测试，三份 168、116、135，八个新的照 7.5 的表（第二份五个、第三份三个、第一份没有）；改过的已有的测试：`TestPermissionMatrix`、`TestTheAccountsNotebooks…`、`TestMatrixViolationsCatchesEachGap` 在第二份，`TestThePermissionMatrixOverMCP`、`TestTheStreamSees…`、`TestThePermissionMatrixCoversEveryOperation` 在第三份（审查 C 核对）。修复一轮之后 420 个、168 / 117 / 135：新的 `TestPageNotebooksConvertTheState`（不要库）在第二份，新的子测试在第三份的 `TestWorkspaceChangesAndCreatingAnLLMWiki` 里，第一份没有新的或改过的测试。

本机各份单独、竞态下，经 `heavy.sh`，测试二进制的时间（秒；建成时的开始时负载 3.4–4.9，起点 1.8–3.7，修复一轮 1.7–3.8）：

| 份 | 起点 `f92ecac7` | 建成 `66602404` | 修复一轮 `86e4c8da` |
|---|---|---|---|
| 第一 | — | 257.5、266.2，平均 261.8 | 不量（没有新的或改过的测试） |
| 第二 | 148.3、143.4，平均 145.8 | 150.6、159.9，平均 155.2（+9.4） | 155.2、149.8，平均 152.5 |
| 第三 | 184.1、179.9，平均 182.0 | 198.2、192.1，平均 195.1（+13.1） | 186.6、193.7，平均 190.1 |

- 第三份多出的比它的三个新测试（4.9 秒）多：`prepareMatrix` 给它的每个用者多种一个 `wlab`（第三份五个、第二份三个），`TestTheStreamSeesWhatEachReadAllows` 也读 `wlab` 的笔记本（偏离 8；审查 C-N5）。修复一轮的两份的平均都在建成时两次的波动之内（这里两次之间差 5–7 秒）。起点的两次的负载更低，增长若有偏差是偏大。
- CI 与本机之比（建成的运行的测试一步对建成时的平均）：1.19、2.44、1.91。S1 带修复比起点多：本机第二份约 7–9 秒、第三份约 8–13 秒，按这两个比约 17–23 秒与 15–25 秒。7.5 的底（第二份约 6:12、第三份约 5:41）：第二份的五个新测试 CI 上约 9.4 秒，变体的格在 7.5 的 15–25 秒之内，同 7.5；第三份是 C 的"约 25 秒"，加上 7.5 的 S2、S3 的数，P1 结束时约 6:45–7:00，仍在 7:30 之下，7.5 的 6:20–6:30 没有算 `wlab` 的种（决定 26）。照 6.5 的规则什么也不挪。
- CI 的测试一步（下面"门禁与 CI"）：建成 5:11、6:19、6:13，修复一轮 7:12、5:12、5:07，都在 7:30 之下；两次的平均 6:11.5、5:45.5、5:40。第一份的 7:12 是 CI 的波动（上面"接受与推后的"）。没有分份，没有提高 `-timeout`，CI 的文件与 `Makefile` 没改。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`，机器同时有别的会话在跑测试）：

- 建成时（`66602404`）：`make lint gen-check knip build` 约 32 秒通过（golangci-lint 0 个问题，oxlint、oxfmt 干净，`gen-check` 没有差异，knip 干净，二进制建成）；`make test` 928 秒（竞态的一份与组合根的三份同时：三份 204.5、253.5、319.0 秒；之后成本与守卫的两行；`test-web` 166 个文件、3,346 个测试）；`make test-go-short` 62 秒；`make e2e` 284 个 55.4 秒。7 个提交各自在一个检查的工作树里 `go build ./...`、`go vet ./...`、碰到的包的 golangci-lint（0 个问题）、gofmt、碰到的包的 `go test -short`；碰网页、e2e 或契约的另过 oxfmt、oxlint、tsc（网页与 e2e）、knip 与 vitest（第 2、4 步 3,344 个测试，第 5 步起 3,346 个）；第 4 步因 rc11 改正之后第 4–7 步再核对一次。审查 C 在每个提交上另跑整个模块的 lint、tidy、整个 short 测试、`make gen-check` 与网页的五步，与那个提交加或改的库上的测试：都通过。
- 修复一轮（`86e4c8da`）：`make lint gen-check knip build` 通过（golangci-lint 在 server 与 server/tools 0 个问题，`go mod tidy -diff` 干净）；`make test` 838 秒（三份 192.8、235.8、300.6 秒；`test-web` 166 个文件、3,346 个测试）；`make test-go-short` 54 秒；`make e2e` 284 个 36.0 秒；每次之后树是干净的。13 个提交各自 build、vet、碰到的包的 golangci-lint、gofmt 与 short 测试（经 `heavy.sh`）；网页的一个（`2e954be5`，不碰 Go 的包）另过 oxfmt、oxlint、tsc、knip、vitest（166 个文件、3,346 个测试）与 `make gen-check`。修复核对 C 照审查时的范围再跑一遍，都通过。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 运行 | 头 | 竞态 | 组合根一 | 组合根二 | 组合根三 | 守卫 | 成本 | short | web | e2e | 镜像 | 共 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 40098894279（建成） | `66602404` | 9:51（9:30） | 5:28（5:11） | 6:42（6:19） | 6:36（6:13） | 8:45（8:22） | 8:15（6:05） | 3:20（3:01） | 4:07（3:36） | 4:29（3:40） | 1:28 | 14:24 |
| 40107114331（修复一轮） | `86e4c8da` | 8:13（7:55） | 7:33（7:12） | 5:33（5:12） | 5:28（5:07） | 12:46（12:27） | 8:10（6:03） | 3:23（3:03） | 4:08（3:34） | 4:01（3:09） | 1:14 | 16:51 |

两次都全绿。组合根三份的测试一步都在 6.5 的 7:30 之下；守卫那一份的 12:27 在它的 30 分钟之内（这一轮不改守卫跑的，读作运行机的差别）。分支的两次的测试一步的平均见上面"组合根的各份"。
