# M11/P1 S4（网页）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p1-s4`（实施的 6 个提交：一处判断笔记本收不收写（`writesNotebook` 与 `frozenByUpgrade`、19 处调用、未建链接的 `upgrading`、两句说明、假服务端的冻结、控件表）`3120f767`，编辑器失去编辑 `a3092bc1`，横幅 `278a8f2e`，从设置发起、落到升级页（`UpgradeService`、`NotebookStore.startUpgrade`、对话框、`pages-locked`、升级页的路由与状态一节、`ConfirmDialog` 的 `more`、假服务端 `upgrade-server.ts`）`3a87226b`，升级页的接入、放弃与回滚，`mode` 与重连重读升级，任务页回滚的取消 `65a08e2f`，e2e 的 UP1–UP3 页面版本与 `threeBrowsers` `3948f362`；推送之前改写过三次 4–6 号；从 `main` 的 `e42558f2` 开始，审查时在 `3948f362` 上，54 个文件、加 3,617 行、减 108 行，没有碰 `server/`。修复一轮四个：S4B-M1 = S4A-N2、S4C-M1、S4A-N4、S4C-N1 `1f7da037`，S4A-N1、S4A-N3 `9070f34b`，S4B-N1、S4C-N2 `0d6eb002`，S4C-Q3 `b1b66372`（12 个文件、加 375 行、减 74 行）；修复第二轮两个：S4A-N5、S4A-N6、S4B-N2 `c416420a`，S4B-N3、S4C-N3 `e9af6107`；一共 12 个；Step 的文档提交之前把 `main` 的 `d589e5a2` 普通地合进分支；合并 `ce370441`），对照 [01-P1-freeze-start-rollback.md](../01-P1-freeze-start-rollback.md) 第 0 节的 S4、4.4、第 5 节、6.4、6.5、7.3 的 S4 各行（54–59、74–79）与第 8 节，[S3 的审查记录](P1-S3-abandon-rollback-and-end-review.md)，[00-M11-design.md](../00-M11-design.md) 4.12、4.15、第 9 节与第 11 节（第 184、201、214、224、228–230 条），[v0.1 总体设计](../../v0.1-design.md) 13.2 第 9、16、17、18、23、26 条与 13.4 第 3 条，05-lessons 第 2、6 节，实施者的报告（决定 246–248 的落实、250–252、20 条偏离、逐个提交的核对、门禁、变异的表、实测、WebKit 的那一眼、CI、留下的事） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（S1–S3 时的三个工作树挪到 `3948f362`），用到 Docker 的都经 `heavy.sh`：网页行为的正确性（A：store 与 service、编辑器的 `lostBy` 与 `settle`、升级页整份、设置的 `ModeSection`、横幅、任务行的取消、`ConfirmDialog`、`pages-locked`、事件的分层，三个假服务端；拿服务端的 `unit_content.go`、`views.go` 与契约核对假服务端的码序；10 个变异，都是 vitest；三个临时探针：发起之后落到升级页时缓存里旧的 404、照真服务端的码序走"编辑器失去编辑"、结束回滚的对话框的可访问描述），契约、权限与"只读收成一处"（B：拿真服务端的 `unit.go`、`unit_content.go`、`freeze_guard.go`、发起、放弃、结束回滚、`upgrade_errors.go`、`cancel.go`、`upgrade_freeze_test.go` 与已装的 SWR 2.5.1 的 `mutate` 核对；基线 7 个 vitest 文件 237 个测试；12 个变异；三次对照实验 o1–o3，试真服务端的码序），测试、变异、e2e、逐个提交、CI 与数（C：重跑 7.3 的 S4 各行，核对实施者记作等价的三条，自己 20 种改坏，共 35 种 195 秒；6 个提交在干净的临时工作树里各自跑 `gen-check-web`、`lint-web`、`knip`、`build-web` 与经 `heavy.sh` 的 `test-web`；新的页面故事经 `heavy.sh` 两轮；不提交的探针故事在三种浏览器里各两次查 94 像素的原因；以一个容器加一个新开的 Chromium 复现 `ERR_NETWORK_CHANGED`；CI 的用时与 `gh run view` 的起止逐一对过）。修复核对一轮三位各在自己的工作树里核对 `b1b66372`（A：重跑审查时的 10 个变异、两个只跑部分格子的变异核对实施者的说明、7 个新的，探针 1、3 重跑，新的探针 4、5；B：13 个变异，临时给冻结的答复加日志跑 8 个测试文件，`ip` 调用的小探针；C：19 个变异，逐个提交，经 `heavy.sh` 的页面故事五轮（三轮新、两轮换回旧的全局准备作对照），10 轮网络探针）；修复核对第二轮 A、C 核对 `e9af6107`（A：探针 4、5 重跑与 8 个变异；C：在隔离的容器网络里造出一次 DAD 失败，5 轮网络探针，页面故事一轮） |
| 日期 | 2026-11-10（实施、审查、两轮修复与两轮修复核对） |
| 结论 | Important 0。Minor 2：S4B-M1 = S4A-N2（两位各自报出，按 Minor 处置）：假服务端对冻结中的内容保存先答 `notebook.upgrading`，真服务端先查会话、答 `page.edit_session_ended`，重开时才答冻结；`page-editing` 的冻结测试钉住了一串真服务端走不出的请求，真实的路径没有测试。S4C-M1：结束回滚的对话框里"已经复原完"的条件，三个数各自没有被测试钉住，去掉 `created` 时 vitest 全过，而只剩补写的 `raw`、`AGENTS` 正是常见的停下状态。Nit 6：发起之后落到升级页先显示缓存里旧的读（S4A-N1）；"做完之后焦点给标题"只对放弃钉住、"只重读一次"没有钉住（S4A-N3）；对话框的 `more` 不在可访问描述里（S4A-N4）；UP3 页面版本的数据库断言只看状态（S4B-N1）；根下 `raw` 按标题键认没有测试（S4C-N1）；UP3 的任务行按开头的正则找（S4C-N2）。Q 3：升级页对发起人写的 prompt `upgrade` 要到 P3/S2 才有（S4A-Q1 = S4B-Q1 = S4C-Q1，三份的 Q1 是同一个问题，决定 253）；横幅出现时正文跳 94 像素的原因（S4C-Q2，C 查清，进 M12 的打磨）；本机 e2e 偶发的 `net::ERR_NETWORK_CHANGED` 的原因（S4C-Q3，C 查清，修复一轮缓解）。三位都没有查出网页行为本身的错：问题在测试钉住了什么、假服务端的码序与一处无障碍。实施者的 20 条偏离三位都判过，都对。修复一轮四个提交（`fix-1.md`），实施者有三处与说明不同（W2 由"重读挂着时页面重新渲染"那一格杀死，W1 在"继续"上等价，可访问描述以 ByRole 的 `description` 断言），A、B 都接受。修复核对一轮旧发现都已关闭，另有 5 条新的 Nit（S4A-N5、S4A-N6、S4B-N2、S4B-N3、S4C-N3），没有 Important、Minor；修复第二轮两个提交（`fix-2.md`），两处偏离（写缓存前比较 `data` 与 `error`、不用 `mutate` 的函数形式；`-dadfailed` 实施者只依据 iproute2 文档）。修复核对第二轮 A、C 判五条都已关闭，A 接受第一处偏离、收回原来的建议，C 实测了第二处；没有新发现，审查到此结束 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：S4A-N1–N4、S4A-Q1，S4B-M1、S4B-N1、S4B-Q1，S4C-M1、S4C-N1、S4C-N2、S4C-Q1–Q3。修复一轮照主持者的修复说明（`s4-review/fix-1.md`）分四个提交。同一件事合为一个修复：S4A-N2 与 S4B-M1（按 Minor）；三份的 Q1。下面的代码位置是 `3948f362` 的，`W/` 是 `web/apps/web/src/`，服务端的路径相对 `server/internal/modules/`） | — |
| S4B-M1 = S4A-N2 | 低（假服务端与测试） | `W/test/page-server.ts` 的 `frozenWhileUpgrading` 在路由之前就答冻结；`W/stores/page-editing.test.ts:488-515` 的"an upgrade's freeze refusing a save"先给 Plans 一个升级、`server.sessions.clear()`（注释写"发起删掉了其间过期的会话"），再保存，`:514` 断言第一条拒绝是 `409 notebook.upgrading PUT`。真服务端答不出这个码：发起时先清过期的会话、有活着的就拒绝（`wiki/app/start_upgrade.go`），升级中这本笔记本一个会话也没有；内容的写先经 `writersSession` 查会话（`page/app/unit_content.go`，没有就答 409 `page.edit_session_ended`），冻结的守卫在其后的 `apply` 里。所以升级中的每一次保存都是"同时有两个拒绝理由"的写，实施报告偏离 8 说这种写少见，对内容的保存不成立。A 的探针 2 与 B 的 o1–o3 照真序走：请求是 `PUT … in session-1`，再 `409 notebook.upgrading POST …/edit-sessions`，网页经 `forget` → 重开 → `{reason: "upgrading"}`，锁住编辑器、留住正文、不再保存，行为是对的；现有测试钉住的是"PUT 被冻结拒绝，再结束一个发起时已删掉的会话"。只把假服务端改成真序时只有 `:514` 失败。`edit-session.ts:69`、`:184-186` 的注释写的也是走不到的情形（05-lessons 第 2 节那一类） | 已改（`1f7da037`，提交 1）：假服务端对 `PUT /api/v0/pages/*/content` 照真序，会话不在或不是这一页的交回原路由、答 `page.edit_session_ended`，会话在才答冻结，注释写"内容的保存先查会话"；测试断言 `PUT …` 之后是 `409 notebook.upgrading POST …/edit-sessions`、没有 `END`，只读、正文留着、不再保存照旧；`edit-session.ts` 的注释照实改，`settle` 对 `upgrading` 的 `end()` 留作防御。实施者另照真服务端改了两处：已删的页答 404，内容不变的写升级中答 200。其余冻结的写（打开会话与单元里的新建、改名、移动、删除等）假服务端仍先答冻结，真服务端先答 404 或领域的冲突；实施者临时给冻结的答复加日志跑遍给 Plans 设升级的测试文件，冻结只答到一次（重开会话），没有测试依赖这些差别。变异"假服务端先答冻结"（实施者的 F1、A 的 N2、B 的 b13）被 `page-editing.test.ts` 杀死（7.3 第 126 条）。修复核对一轮：A、B 关；B 逐条核过实施者对别的冻结写的对照，自己加日志跑 8 个文件 288 个测试，冻结同样只答一次；注释只写了"只有同时有两个拒绝理由的写才分得出"，没写"没有测试依赖"，不算发现 |
| S4C-M1 | 低（测试） | `W/pages/notebook/upgrade-page.tsx:190` 的 `left.nodes + left.pages + left.created === 0`：`upgrade-page.test.tsx` 只测了三个都不为 0 与三个都为 0。r08 去掉 `created` 时 vitest 全过。漏的正是常见的停下状态：发起补写的 `raw`、`AGENTS` 回滚时算"新建的、还在的"（UP3 页面版本里的 3 就是 `Fresh`、`raw`、`AGENTS`）；发起之后 agent 还没做什么就放弃、回滚排队被取消，剩 `{0, 0, 2}`，少了 `created` 的条件会写"已经复原完，结束即可"、不列三个数，而 `raw` 与 `AGENTS` 其实还在。e2e 的 UP3 是 `{1, 0, 3}`，也抓不到 | 已改（`1f7da037`，提交 1）：在"a stopped rollback that left nothing"旁加"只有一个数不为 0"的三格 `{1,0,0}`、`{0,1,0}`、`{0,0,2}`，每格断言结束的对话框列出三个数、有"链接可能指不到"那一句、没有 `endNothingLeft`。r08 与对称的去掉 `nodes`、去掉 `pages` 各被一格杀死（7.3 第 124 条）。修复核对一轮：C 关 |
| S4A-N1 | 细节（读） | `W/pages/notebook/general-page.tsx:235-241` 发起成功之后只 `mutate(["upgrade", notebook.id])` 就跳转，发起答回来的升级被扔掉；升级页（`upgrade-page.tsx:132-133`）对缓存里的 404 写"没有进行中的升级"。管理员先看过升级页（缓存里是 404），再去设置发起，落到升级页时标题有焦点、标题下写"This notebook has no upgrade under way."、同一屏的横幅却写"正在升级"，读回来才换（A 的探针 1，真机上只闪一次 GET 那么长）。缓存里是上一次已结束的升级时，会先显示上一次的发起人与时间 | 已改（`9070f34b`，提交 2）：发起成功之后 `mutate(["upgrade", notebook.id], started, { revalidate: false })`，再去升级页；写缓存放在 `mounted()` 之外，设置页已离开时也写，跳转仍只在还挂着时做；放弃与继续同样先把答复写进缓存、再重读；结束回滚之后升级已不在，照旧重读、期望 404。vitest：先读到 404，再发起，让之后的读挂起，标题下是 "Started by Ada…"、不是"没有进行中的升级"。修复说明预期的"少一次读"不成立：原来的 `mutate(key)` 没有挂着的 hook，本来就不读（A 同意）。修复核对一轮：A 关（探针 1 在头上重跑），另报出 S4A-N5、S4A-N6；第二轮之后放弃与继续只盖住发出时显示的那一份 |
| S4A-N3 | 细节（测试） | 三个对话框共用"做完之后焦点给标题"（`upgrade-page.tsx:121-127` 的 `onClosed`），只有放弃那一格让重读挂着（实施者补的 x25）：A 的 W1（继续、结束做完之后焦点给仍在的按钮）全过，因为那两条测试关对话框时重读已经回来、按钮已经没了。`RollbackJob` 的"每个任务只重读一次"（`:370-376`）：W2 去掉 `reread.current.has(jobId)` 的守卫全过；这道守卫防的是重读失败时的连环重读（失败 → 重新渲染 → `rereadUpgrade` 是新函数 → effect 再跑），没有测试让那次重读失败或在它回来之前重新渲染（C 的 r04 是同一个改坏，C 判近乎等价） | 已改（`9070f34b`，提交 2）：继续、结束各加一格，重读挂着时关上对话框，断言焦点在标题；`RollbackJob` 加一格，任务结束之后那次重读失败（这一轮答 503，第二轮改 500，S4B-N2），断言只发出一次 `GET upgrade`、页上有"再试一次"。与说明不同的两处，A、B 都接受：W1 只被结束那一格杀死，继续的答复先进了缓存、其中 `rollback_left` 为 null，按钮在关对话框之前就没了，W1 对继续等价；失败的那一格杀不死 W2，读失败时 `UpgradeRead` 整页换成 `NotLoaded`、`RollbackJob` 随之卸载，effect 不会再跑，守卫真正防的是重读回来之前页面又渲染一次（`rereadUpgrade` 是新函数），实施者另加一格"重读挂着时切换语言让页面重新渲染，只读一次"，W2 由它杀死（7.3 第 128、129 条）。修复核对一轮：A 关，收回审查时"失败之后连环重读"的理由；按代码看，实际会触发它的是重读还没回来时到达的 `mode` 事件，切换语言是它的替身；A 只跑继续的格子时 W1 存活、只跑失败的那一格时 W2 存活，印证实施者的两句说明 |
| S4A-N4 | 细节（无障碍） | `W/app/confirm-dialog.tsx:142-144` 的 `more` 是 `AlertDialogDescription` 之外的兄弟节点：结束回滚的对话框的 `aria-describedby` 只指向描述那一句，剩下的三个数与"链接可能指不到"不在里面，焦点在"Cancel"上，读屏的人听不到这次结束会留下什么；发起对话框的"根下的 raw 将成为来源区（N 个节点）"一样（A 的探针 3：`includes counts=false links=false`）。内容都在对话框里、读得到；撤销对话框的 `history.revertLinks` 也写在描述之外（`pages/notebook/revert-dialog.tsx:327-331`，M8 的先例），定 Nit | 已改（`1f7da037`，提交 1）：有 `more` 时 `AlertDialogDescription asChild` 包住一个 `div`，里面是描述的 `<p>` 与 `more`（`space-y-4` 等于原来的 `gap-4`，颜色不变），没有 `more` 时结构不变；项目没有 jest-dom，测试以 Testing Library 的 `getByRole(…, { description })` 断言，它用同一个 `dom-accessibility-api` 的 `computeAccessibleDescription`（与说明不同，A、B 都接受）。变异"`more` 不在描述里"被杀死（7.3 第 127 条）。撤销对话框的先例不在本轮，进 M12 的打磨（00 号文档"写给后面的 M"）。修复核对一轮：A 关（探针 3 在头上重跑：`includes counts=true links=true`，焦点仍在 Cancel，文档顺序与外观不变） |
| S4B-N1 | 细节（e2e） | `e2e/stories/upgrade/up3-abandon-rollback.spec.ts` 的页面版本断言了页上的三个数（1、0、3），数据库里只断言 `upgradesOf == ["rolled_back"]`；结束回滚存下的 `rollback_complete = false` 与三个数没有和页上的对上。API 版本都盖住了，UP1、UP2 的页面版本也照实，所以只是少绑一处 | 已改（`0d6eb002`，提交 3）：先断言结束的答复是 `rolled_back`、未复原完、`{1,0,3}`，与对话框里列出的三个数相同，再 `expectUpgrade(db, <结束的答复>)`（它比对库里存下的 `rollback_left` 与笔记本的四列已清掉）。修复核对一轮：B 关 |
| S4C-N1 | 细节（测试） | `W/pages/notebook/general-page.tsx:168` 按 `titleKey(node.name) === titleKey("raw")` 认根下的 `raw`，同服务端（`wiki/app/start_upgrade.go` 按 `shared.TitleKey(SourceRootTitle)`），`Raw`、`RAW` 也会成为来源区；`general-page.test.tsx` 的几格都用小写的 `raw`，r13（按大小写严格比较）照过 | 已改（`1f7da037`，提交 1）：对话框表加根下的页叫 `Raw`、下面 2 个节点的一格，断言那一段与 N 照样出现（用 2：`upgrade.startRaw` 一个节点时读成 "1 nodes"，消息表没有复数形式，记下没改）。r13 被它杀死（7.3 第 125 条）。修复核对一轮：C 关 |
| S4C-N2 | 细节（e2e） | UP3 的任务行与它的"取消"按开头的正则找（`/^Undo the upgrade/`、`/^Cancel Undo the upgrade/`）；任务的名字里带时间，写不出确切的名字，用正则是对的，但已有的 `e2e/fixtures/wiki-transfer.ts` 的 `jobRow` 锚到 `^<what>, `、到逗号为止。这个故事里列表只有一行，没有歧义 | 已改（`0d6eb002`，提交 3）：照 `jobRow` 锚到逗号，对得上 `transfer.jobName` 的 `"{title}, {time}"`。修复核对一轮：C 关 |
| S4A-Q1 = S4B-Q1 = S4C-Q1 | 问题 | 升级页对发起人写"The prompt to give it: `upgrade`"（`upgrade-page.tsx:272-278`，`upgrade.agentPrompt`），UP1 的页面版本也断言了它；服务端的 prompt 现在只有 `ingest`、`lint`、`query`（`wiki/adapter/mcp/prompts.go`），设计第 9 节把 prompt `upgrade` 与 README 的"升级"一节放在 P3/S2。P1/S4 合并之后网页就能发起，到 P3/S2 之前，发起人照页上的话找不到这个 prompt。实施报告第 9 节先提出；A、B 各给两种选择（P3/S2 之前只写 README 那一句，或接受这段空档），C 建议接受并在第 9 节写一句核对 | 决定 253：照旧写 prompt 名（下面"主持者的决定"）；只改文档，写进 P1 设计第 9 节与 P3 的进度 |
| S4C-Q2 | 问题（第 224 条） | 实施者量到横幅出现时正在读的段落在三种浏览器里都往下跳 94 像素、`scrollY` 不变，原因未核实。C 以不提交的探针故事（80 段的长页、视口 1280×800、第 40 段滚到视口中间）在三种浏览器里各跑两次：A 经 REST 发起、横幅随 `mode` 出现，三种都不补偿（top 373 → 467）；B 横幅出现之后在它前面单插一个 94 像素的块，三种都补偿，WebKit 也补（`scrollY` 加 94）；C 同样的块插在正文第 1 段之前，同样补偿。所以滚动锚定本身都正常，左栏与侧栏的 `overflow-anchor: none` 也没挡住它。A 里的 MutationObserver 记下同一批变动：横幅的 `<output>` 插进 `main`，同一时刻 `article.nw-reading` 的子节点全部重新加了一遍，之前的 80 个 `<p>` 一个都不在文档里（`stillConnected: 0`），`article` 本身没换，锚点祖先链上的样式都没变。原因在代码：`W/pages/page/reading-view.tsx:132` 的 `editable = canEditPageBody(pages, page.id)` 是 `:133-138` 那个 `useLayoutEffect` 的依赖（`:224`），effect 每次 `container.innerHTML = html`；升级一开始 `writesNotebook` 变假、`editable` 随之变假，正文全部换掉，浏览器选中的锚点（正文里的一段，C 的近似算法在三种浏览器里都选第 35 段）被移除，这一帧不补偿。升级结束、横幅消失时正文同样往上跳；角色变化也会，`role` 也在依赖里 | 照第 224 条进 M12 的打磨，S4 不修；移交写原因与 C 给的改法：`editable` 不进那个 effect 的依赖，任务勾选框的 `disabled` 由另一个 effect 改，`toggleTask` 经 ref 读当时的可编辑性（它今天在调用时已再判一次 `canEditPageBody`）；照 B 的结果，这样之后横幅的插入在三种浏览器里都会被补偿；修时要对照 M6 以来阅读视图依赖 `editable` 的增强逐一核对，比如任务勾选的焦点恢复。00 号文档 4.15 的"靠浏览器的滚动锚定"照实改 |
| S4C-Q3 | 问题（e2e 的环境） | 实施者新故事的第一轮里 UP3（页面）在 Chromium 上失败：文档载进来了，一个子资源（入口模块）答 `net::ERR_NETWORK_CHANGED`，页面空着、没有一个 API 请求。C 把时间对上（`m11/s4/e2e-new-1.log` 与 `journalctl`）：06:28:10.457 `global-setup.ts` 的 `startPostgres()` 起了这一轮的容器，06:28:12.436 它在主机一侧的 veth 完成 DAD、得到 IPv6 链路本地地址（systemd-networkd 记"Gained IPv6LL"），UP3 的 `page.goto` 落在约 12.3–12.74 秒之间，正好包住它。复现（`c/netchange/probe3.cjs`）：起一个容器，1.2 秒后启动一个新的 Chromium，连续载入带 30 个模块脚本的页，5 轮里 4 轮出现，各 6–15 个请求失败，失败的时刻与各自 veth 的"Gained IPv6LL"差不到 5 毫秒；之前浏览器已开着一段时间、载图片与普通页的三种探针，340 多次载入一次也没失败，"新开的浏览器"与"模块脚本"哪个是必要的没有分开查。与页面和 S4 无关；每一轮 e2e 的头两秒都有这个窗口；M10 的两次（`M10-llm-wiki/reviews/P1-S4-web-review.md`、`P3-S2-health-web-review.md` 记的"初始模块装载时"失败）是同一类，当时没有查出原因；CI 的 runner 上会不会发生没有查 | 已改（`b1b66372`，提交 4，范围 `e2e`，"M10 and M11 stories, found in M11"）：`e2e/global-setup.ts` 在 `startPostgres()` 之后、与建模板库和迁移并行（`Promise.all`），每 100 毫秒看一次 `ip -6 addr show tentative`，为空才放 worker，最多 5 秒；`ip` 不在或出错时退为等 3 秒（这一轮从调用 `startPostgres()` 之前计时，S4C-N3，第二轮改）；只改等待。实施者先查过判断可靠：veth 的地址从容器启动起 tentative 约 1.66 秒，期间这条命令有输出、之后为空。`make e2e` 518 个过；新的页面故事经 `heavy.sh` 单独五轮都过、0 次 `ERR_NETWORK_CHANGED`，五轮都真的等过（`startPostgres()` 之后约 1.07 秒去看，等了 315、423、418、750、527 毫秒）；CI 的 E2E 一步 354 秒（上一轮 392 秒）。修复核对一轮：C 关：以探针照这个等法在新开 Chromium 之前等，取最坏的情形（容器一启动就看），10 轮每一轮第一次看都已是 tentative，等待都在"Gained IPv6LL"之后 10–87 毫秒结束，之后新开的 Chromium 载入模块页 0 次失败；同时段不等的探针 5 轮也 0 次失败（负载约 8–11，浏览器启动变慢，窗口错开），所以证据是结构上的（等到地址不再 tentative 才放浏览器）加等待总在 IPv6LL 之后结束；本机每轮约多 0.2 秒（从容器启动到第一个测试开始，新的三轮平均 1.79 秒，旧的两轮 1.56 秒）；CI 上看不出变慢（全局准备到"Running 518 tests"20.7 → 18.6 秒）。C 另报出 S4C-N3，B 报出 S4B-N3。修复核对第二轮：C 再跑 5 轮探针（换成 `tentative -dadfailed`、带 1 秒超时），等待都在 IPv6LL 之后 31–95 毫秒结束，0 次失败，两轮核对共 15 轮 |
| S4A-N5 | 细节（缓存） | 修复核对一轮的新发现：`W/pages/notebook/upgrade-page.tsx:120-122` 把放弃、继续的答复写进缓存时不看缓存里现在是什么，而 store 的 `upgraded` 要等笔记本列表重读完才把答复交给页面（`W/stores/notebook.store.ts:182-194`）。其间到达的 `mode` 事件重读升级，若回滚的任务这时已停下或做完，读答停下的回滚（三个数与 Continue、End）或 404；列表回来之后，答复（回滚中、任务 J、`rollback_left` 为 null）把它盖掉，页上退回"任务已结束、却没有剩下的数也没有按钮"，服务端答不出这样的状态，直到随后的重读回来。A 的探针 4 复现。修复之前只有读，SWR 以最后发出的读为准，不会这样；要两件事同时成立，只持续一次读的时间，焦点不受影响，定 Nit | 已改（`c416420a`，第二轮提交 1）：发出改动时从 SWR 的缓存记下这把键的状态（键用 `unstable_serialize(["upgrade", id])`，与 `useSWR` 同一个序列化），答复回来时只有 `data` 与 `error` 都还是那两个对象才写，否则不写、只重读。与说明不同：说明照 A 的建议用 `mutate` 的函数形式；SWR 2.5.1 的 `mutate` 写缓存时总会清掉 `error`（函数原样返回 `current` 也一样），而读答错时 SWR 留着旧的 `data`、只设 `error`，事件的读答 404 时函数形式会把 404 抹掉、显示旧的答复。vitest 照探针 4：事件的读答停下的回滚时 Continue 与 `{2,1,3}` 一直在；答 404 时"没有进行中的升级"一直在。变异：无条件写、只比 `data`、只比 `error`、答复回来时才记都被杀死（7.3 第 130 条）。修复核对第二轮：A 关；对 SWR 的说法属实（`config-context` 的写回是 `set({ data, error: UNDEFINED })`，读出错时只加 `error`、合并写留着旧的 `data`），换成函数形式只被 404 那一格杀死，A 收回原来的建议 |
| S4A-N6 | 细节（测试） | 修复核对一轮的新发现：新的写缓存有两点没有钉住：写在重读之前（A 的 N1-reread-before-cache 存活：先重读、后写缓存时重读在写之前发出，SWR 丢掉它的答复，页上一直是答复；现有测试里重读答的与答复相同，看不出；探针 5 让重读把发起人答成 "Zed" 证实）；页面已离开时也写（N1-start-cache-only-mounted、N1-change-cache-only-mounted 存活：实施者的选择，没有一格走"答复回来时页面已离开，再回来"的路） | 已改（`c416420a`，第二轮提交 1）：放弃之后的重读答出 "Zed"，断言页上是 "Started by Zed…"、读了 2 次；放弃的答复等着列表时离开升级页、看到横幅变成"正在撤销"之后回来，读都挂着，页上是答复里的回滚与排队的任务、没有 Abandon；设置页上发起挂着时离开，答复之后去升级页、读挂着，页上是 "Started by Ada…"、一共 2 次读（7.3 第 131、132 条）。修复核对第二轮：A 关，三个变异照新代码改写后都被杀死 |
| S4B-N2 | 细节（假服务端） | 修复核对一轮的新发现：新加的"任务结束之后的重读答 503"那一格（`upgrade-page.test.tsx:373-397`，`problem(503, "server_busy")`，不带 Retry-After）：契约里 `getUpgrade` 只列 `notebook.not_found`、`forbidden`、`wiki.upgrade_not_found`；真服务端的 `server_busy` 只出在大请求体的槽、解析预算与发起，而且总带 Retry-After（带了网页就按它重试）。修复说明点名要的 503，05-lessons 第 2 节那一类，但没有掩盖网页的行为 | 已改（`c416420a`，第二轮提交 1）：改答 `problem(500, "internal_error")`，测试名的 "refused as busy" 改为 "failing"；仍断言 3 秒内只读一次、页上有 "Try again"。修复核对第二轮：A 关 |
| S4B-N3 | 细节（e2e） | 修复核对一轮的新发现：`e2e/global-setup.ts` 的 `tentative()` 调 `ip` 没有超时，`ip` 卡住（如 netlink 不回）时 global setup 一直等，也走不到"判不了就等 3 秒"的退路；概率很低，本机一次约 3 毫秒 | 已改（`e9af6107`，第二轮提交 2）：`run("ip", […], { timeout: 1_000 })`，超时以 SIGTERM 杀掉子进程并 reject，落进 `catch`、走退路。修复核对第二轮：C 关（挂住的 `sleep 5` 在 1,006 毫秒时被拒，`killed: true`；`ip` 不存在答 `ENOENT`，同样走退路） |
| S4C-N3 | 细节（e2e） | 修复核对一轮的新发现：`ip` 不可用时的退路从 `startPostgres()` 之前起算（`const starting = Date.now()` 在它之前），实际等的是"setup 开始满 3 秒"，注释与提交说明却写"容器起来满 3 秒"；`startPostgres()` 本身用了 3 秒以上（拉镜像、initdb 慢）时退路一点也不等。本机与 CI 都有 `ip`，今天不走这条路。C 另有一条未核实的怀疑（A 也提到）：DAD 失败的地址一直带 `tentative dadfailed`，会让每一轮等满 5 秒，可改用 `ip -6 addr show tentative -dadfailed` | 已改（`e9af6107`，第二轮提交 2）：`started` 挪到 `await startPostgres()` 之后，退路等到它之后 3 秒，注释与说明照实写；加上 `-dadfailed`，注释写明原因。实施者核对：闲置时两种写法都空，起一个容器每 200 毫秒看一次，前 1.8 秒两者都列出这个容器的 veth 的 3 行、2.0 秒起都空，逐次相同；真的 DAD 失败的地址实施者造不出（本机不许非特权的 user namespace），只依据 iproute2 文档的写法。修复核对第二轮：C 关，并在一个 `--network none --cap-add NET_ADMIN` 的容器里以一对 veth 配同一个地址造出一次 DAD 失败：`tentative` 列出带 `dadfailed tentative` 的地址，`tentative -dadfailed` 不列它，正在检测的地址照样列出；不认这个写法的 iproute2 会把它当设备名，rc=1，落进 `catch` 走退路 |
| 偏离 | — | 实施者的 20 条偏离（[01-P1](../01-P1-freeze-start-rollback.md) 10.4、3.28）：1 横幅在升级页上不带链接的规则在提交 4；2 决定 246；3 决定 247；4 决定 248；5 `canAddSource` 的替换等价（c7）；6 回滚的任务以整行 `TransferJobRow` 显示；7 "成功之后重读升级"由组件做；8 假服务端随冻结拒绝、码序简化；9 `ConfirmDialog` 的 `more`、`explain` 答 `ReactNode`；10 `wiki.upgrade_pages_locked` 的首句另写；11 提交的标题多半句行为；12 决定 250–252；13 三个对话框挂在 `UpgradeRead` 里、读的键对非管理员为 `null`；14 `mode` 与重连的整合测试在 `upgrade-page.test.tsx`；15 只重读一次按"升级读到时任务还在跑"判；16 放弃不另成一节、三个都为 0 时不列数；17 任务页取消的条件；18 e2e（`holdStream`、UP2 两个 409、UP3 两个 404、在任务行里取消）；19 `UpgradeService` 的方法名；20 编辑器的 `upgrading` 先 `end()` | 三位各在自己的范围里判过，都对：A 核对 7（行为不变）、13（降为编辑者时对话框带着拒绝留着）、15（`rollback_left === null` 等于服务端快照里任务排队或在跑，S3 修复之后三个读在一个快照里，"看到任务结束就重读一次"不漏不多，打开时已停下、停下、做完、继续之后换任务、别的管理员继续了五种逐个走过）、19（无妨）、20 与 250–252；B 核对 8 与 17（替身的 14 个键就是 `TestUpgradeFreezeReachesEveryPath` 的 14 个写，导出、取消、解锁不在里面；码序只在内容的保存上走不到，即 S4B-M1）、13 与 246、247；C 核对 18（两个 404 一条来自 `change()` 之后的重读、一条来自 `mode` 事件，SWR 每次 `mutate` 先删 `FETCH[key]`，不会被去重合并，4xx 不重试、升级页没有轮询，不会有第三条）与 11（范围照实：1–5 `web`，6 `e2e`；改 `playwright.config.ts` 属于 e2e 这个部件），5 号正文一行 93 个字符是重排留下的，没有规则管行宽，不列为发现 |
| 留下的事 | — | 实施报告第 9 节留下的七项：1 Step 的文档提交；2 prompt `upgrade` 现在还没有；3 横幅出现时正文跳 94 像素；4 回滚做完或结束之后升级页只写"没有进行中的升级"；5 第一轮 UP3 的 `net::ERR_NETWORK_CHANGED`；6 `general-page.test.tsx` 跑时 Node 报 `TimeoutNaNWarning`（整个 vitest 22 次，S3 的日志同样 22 次）；7 `main` 的 `e42558f2` 的 CI 红在 guards-core | 1 是本提交；2 即三份的 Q1，决定 253；3 即 S4C-Q2，进 M12 的打磨；4 设计如此，已在 00 号文档"写给后面的 M"的打磨（已结束的升级的报告）；5 即 S4C-Q3，修复一轮缓解；6 S4 之前就有，没有查，不属于 M11；7 由旁支 `m11-p1-m9-revert-flake` 与决定 249 处理，S4 的分支 CI 上这一格在 EPYC 7763 上三次取最少 1.909 秒，在 2.1 秒的旧界之内 |
| 修复一轮的说明 | — | 实施者在修复一轮的报告（`fix-1-report.md` 第 7 节）：1 S4A-N4 用 ByRole 的 `description`（没有 jest-dom）；2 W2 由重新渲染那一格杀死；3 W1 在继续上等价；4"少一次读"不成立；5 写缓存不看 `mounted()`，页面已离开也写；6 等待与迁移并行；7 假服务端另照真服务端改两处（已删的页答 404、内容不变的写答 200）。留下的：`upgrade.startRaw` 一个节点读成 "1 nodes"；`revert-dialog.tsx` 的 `history.revertLinks`；其余冻结的写假服务端仍先答冻结，没有测试依赖；CI 的 runner 上等待有没有触发没有查（job 日志要登录） | 1–3 A、B 都接受（S4A-N3、S4A-N4）；4 A 同意；5 主持者认可，第二轮加测试钉住（S4A-N6）；6 B 核对：多出的只是等待超过迁移的那一段；7 照真服务端，B 核过；留下的前两项进 M12 的打磨，第三项写在替身的注释里 |
| 修复第二轮的说明 | — | 实施者在第二轮的报告（`fix-2-report.md` 第 7 节）：1 S4A-N5 不用 `mutate` 的函数形式，先比较 `data` 与 `error`（函数形式会把 404 抹掉），为此加了 404 那一格；2 `-dadfailed` 只依据 iproute2 文档，没有实测 | 1 A 接受（S4A-N5）；2 C 在修复核对第二轮实测（S4C-N3） |

## 主持者的决定

负责人可以改判（249 是旁支的，记在 [S3 的审查记录](P1-S3-abandon-rollback-and-end-review.md)"旁支"一节；246–248 开工之前定下，250–252 在实施中定下，253 在第一轮的修复说明 `fix-1.md` 里定下）：

246. **横幅的"任何笔记本管理员都可以放弃这次升级"只在 `upgrading`、`reviewing`；设置"模式"一节的"查看升级"只给管理员**（2026-11-10，开工之前）：`rolling_back` 时横幅的链接照给，这一句不写；非管理员在设置里只看到状态。理由：`rolling_back` 时已经放弃或驳回了，再说"可以放弃"不对；13.2 第 18 条不给非管理员只给管理员的页的入口。变异 x04、b1、b3、r01 抓得到。
247. **升级中，变更集页与导入的按角色的说明换成"升级结束之前，这本笔记本只读"**（2026-11-10，开工之前）：`writesNotebook` 为假而角色能写（`frozenByUpgrade`）时，`history.readersNote` 与 `transfer.importReaders` 换成 `notebook.upgradeReadOnly`（两种语言）；读者照旧看到按角色的说明，未建链接对读者仍写"编辑者可以从这个链接新建"。理由：这两句说"编辑者与管理员可以撤销""能编辑的人才能导入"，升级中对管理员也不实。控件表的历史与导入两行（c14、c19）钉住。
248. **接入 agent 一节只在 `upgrading`；停下的 `rolling_back` 只有"继续回滚…"与"结束回滚，保留现在的样子…"，不另给"放弃"**（2026-11-10，开工之前）：改判 00 号文档 4.15 的"停下时放弃就是继续"。理由：`reviewing`、`rolling_back` 时发起人的 agent 也不能写，`reviewing` 显示什么随 P3/S3 的审阅一节定；放弃与继续是同一个操作（`abandonUpgrade`），设计里有两种说法，一屏上同时给"放弃"与"继续"会让人以为是两件事。
250. **设置"模式"一节的"升级为 LLM Wiki…"在页面树读到之前不出现**（2026-11-10，实施中）：与左栏同键 `["pages", id]`、去重。理由：对话框的"根下的 raw 将成为来源区（N 个节点）"一段要靠这棵树数出 N，树没读到就开对话框会漏掉这一段。C 指出边角：树的读失败时这一节既没有按钮也不说原因，但左栏的树会显示读失败与"再试一次"，可以接受。变异 x09、b9 抓得到。
251. **停下的回滚的"继续回滚…"也经一个对话框**（2026-11-10，实施中）：`default` 的语气，写"一个新的后台任务接着复原停下的任务没有复原的……"。理由：它与放弃是同一个操作，对话框让拒绝留在原处，焦点规则与另两个一致（C 的 r06 抓得到取消之后焦点回到按钮）。
252. **放弃、继续、结束回滚答 `transfer.busy` 时，升级页写"回滚已经在进行。"并重读升级**（2026-11-10，实施中）：不用通用的 `transfer.writingBusy`。理由：这三个操作的 `transfer.busy` 只在回滚的任务排队或在跑时答（契约里两个操作的说明；升级期间别的写入任务开不起来，发起时已经拒过；放弃在 `upgrading` 时真服务端不答它），A、B、C 都对过。变异 x38、b5、b6 抓得到。
253. **升级页对发起人照旧写 prompt 名 `upgrade`**（S4A-Q1 = S4B-Q1 = S4C-Q1，2026-11-10）：理由：`main` 在两个 Phase 之间不发布，这段空档只在开发中的 `main` 上；P3/S2 加上这个 prompt 时，核对页上的这句话、UP1 页面版本的断言与 prompt 的名字三者一致。写进 P1 设计第 9 节"交给后面的 Phase"与 P3 的进度；只改文档。

修复一轮的处置照 `fix-1.md`：三份审查全部接受，一组一个提交、各自绿、接在 `3948f362` 之后、不改写已推的历史；这一轮不碰服务端，照决定 242 本机不跑完整的 `make test`，头上跑 `make lint gen-check knip build` 与经 `heavy.sh` 的 `make test-web`、`make e2e`，分支 CI 跑全部；审查者留下的变异都要杀死（A 的 W1、W2，C 的 r08、r13 与两个对称的变异，"假服务端先答冻结"，"`more` 不在描述里"）；只改文档的（S4C-Q2、决定 253、7.3 的等价各行、实施报告第 8 节的偏离、10.4）留给 Step 的文档提交。第二轮照 `fix-2.md`：修复核对一轮的 5 条新 Nit 全部接受，两个提交、接在 `b1b66372` 之后；S4A-N5 照 A 的第一种建议，只在缓存仍是发出改动时那一份时写；页面已离开也写缓存是实施者的选择，主持者认可，各加一格钉住；e2e 的等待照 C 的怀疑排除 `dadfailed`，核对得了就改；本机门禁照 6.5 跑网页的五步、`make e2e` 与新故事两轮。修复核对第二轮之后审查结束，S4 不再有修复轮。

## 接受与推后的

- **接受**：
  - 等价与近乎等价的变异：c7（`canAddSource` 先要 `llm_wiki`，而有升级的笔记本恒为 standard，两者出自同一次读）、c18（预览的后台恢复的提议只交给预览的按钮，按钮只在 `writesNotebook` 的块里，c17 抓到）、x14（发起之后不 `mutate(["upgrade", id])`：SWR 2.5.1 在没有挂着的 hook 时只删去重的记录，读失败时它本来就立刻删掉；只有上一次读成功、在 2 秒的 `dedupingInterval` 之内才会沿用旧的答复，那要旧的升级在这 2 秒里结束、列表也重读过，人做不到；C 同意；为它写的测试测的行为是真的，留着）、A 的 W15 与 C 的 r04（W15 把 `upgrade` 挪到重连的第四层，只晚一层读；r04 去掉"只重读一次"的 Set，只在那次重读回来之前 `UpgradeRead` 又渲染时多读一次，无害；W2 是同一个改坏，A 记为测试的缺口，修复一轮补了测试）、C 的 r15（`frozenByUpgrade` 不看升级：单元的格抓到，在两处调用处是冗余的，那里 `writesNotebook` 为假而 `writesPages` 为真就意味着有升级）、r19（`lostBy` 里 `notebook.upgrading` 挪到最后：每一支认的状态与码互不相同）。
  - A 的未核实的怀疑：1 store 把成功之后的重读列表当一次改动，`load` 只防比改动旧的读：放弃之后 `reread` 还没回来、回滚已经做完，`mode` 事件的 `load` 先写进 `upgrade: null`，`reread` 的旧列表（`rolling_back`）晚到又被放进去，横幅停在"正在撤销"直到下一个事件或重连；要两个列表读的答复调换次序、回滚在毫秒级做完，没有复现，降级被拒之后的重读本来就是这样写的，是已有的一类。2 回滚停了很久、任务行按保留期被清理之后，升级页的任务一节会显示"读不到"与"再试一次"，按钮与三个数照样在；没有造出来。
  - B 的未核实的怀疑：1 管理员被降级之后取消回滚答 403 时，`transfer-job-row.tsx` 只重读任务、不重读笔记本列表（13.2 第 18 条的做法是重读列表）；取消答 `forbidden` 在 M11 之前就可能出现，行为没有变，升级页自己的读下次答 403 时会重读列表，不算 S4 的发现。2 UP3 页面版本的两个 404 是确定的，前提是这段时间里事件流不重连：重连时 `refreshedOnConnect` 再读一次，多出第三个 404，`expectConsole` 会失败；UP1 断言了不重连，UP3 没有；概率很低，没有复现。
  - C 的未核实的怀疑：UP1（页面）按下 Ctrl+E 之后立刻断言编辑器的个数为 0，如果 Ctrl+E 的防线坏了，编辑器要等会话打开的答复回来才出现，这一句可能照过（05-lessons 第 6 节最后一条）；这道防线已由 vitest 控制表的 Ctrl+E 一行钉住，`enter()` 进去之后还会再判一次 `canEditPageBody`；没有做只改 Ctrl+E 的 e2e 变异去证实。
  - 修复核对两轮的观察：A 审查时那条未核实的怀疑（store 的 `reread` 当作改动）仍未核实，本轮没改到；W15 仍近乎等价；C 的不等的对照探针 5 轮 0 次失败受时机影响，不单独作证据；`ip` 的等待看的是整个主机，Ryuk 容器的 veth 一并等到，一轮 e2e 只起一个 PostgreSQL 容器，只在启动时等一次；CI 的 runner 上等待有没有真的触发没有查（job 日志要登录），CI 的 e2e 一步没有变慢。A 在第二轮另有一个存活的变异：比较整个状态对象而不是 `data` 与 `error`，两者只在答复回来时恰好有一次读在途时不同，那时整个状态的比较会放弃写，页上先显示发出前的样子，直到重读回来，现在的写法是对的，不算发现；页面已离开、又被别的实例挂上而那个实例的读在途时，写缓存会丢掉那次读、没有人重读，页上停在答复，`RollbackJob` 轮询到任务结束会自己重读，A 读代码得出，没有另跑。
  - 流程（照实写下）：实施者推送之前改写过三次 4–6 号（都还没推过；1–3 号没动）：5 号 `e6f4c29f` → `d3a13258`（对话框改挂在 `UpgradeRead` 上、读的键对非管理员为 `null`，加"降为编辑者之后结束回滚被拒"一条）；4、5 号 `79ba3279`/`d3a13258` → `3a87226b`/`96ac4792`（补变异第一轮没抓到的五条测试，生产代码没改）；5 号 `96ac4792` → `65a08e2f`（补的测试里计数器叫 `reads`，遮住同名的帮手，oxlint 的 `no-shadow` 报警告：实施者补测试之后只跑了 vitest，逐个提交的核对查出；改名 `answered`）；6 号每次原样重放。开发中另有两串在改写之前的头上开始，门禁一步换成了空脚本，结果不算，没有中途杀进程。
- **推后**：
  - M12 的打磨：横幅出现时正文跳 94 像素（S4C-Q2），原因与改法写进 00 号文档"写给后面的 M"，M11 收尾时追加到 M12 的打磨移交；撤销对话框的 `history.revertLinks` 在可访问描述之外（S4A-N4 的先例）；回滚做完或结束之后的结局不在升级页上（已在同一条里）。
  - 发起对话框的 `upgrade.startRaw` 一个节点时读成 "1 nodes"（消息表没有复数形式，修复一轮的报告），进 M12 的打磨。
  - 其余冻结的写假服务端仍先答冻结（与真服务端只差在同时有别的拒绝理由的写上），写在替身的注释里，没有测试依赖。
  - 决定 253 的核对在 P3/S2。
  - 没有交给 M12 的性能的。

## 反向对照

建成时以 `mut.py` 跑（`NW_SERVER` 指实施者的工作树的 `web/apps/web`，规格 `m11/s4/mut/spec.json` 69 条，经 `heavy.sh`，395 秒；先在没改的树上把规格里的 14 种命令各跑一次，都 rc=0；改坏时从 `.mutbak` 复原，没有用 `git checkout --`），在 `7c1890e2` 上（生产代码与最后的头相同）；第二轮只跑第一轮该抓没抓到的五条（`mut2.out`）。审查者的在 `s4-review/` 下（A 的 `a/`、B 的 `b/`、C 的 `c/`）。

**7.3 的 S4 各行**（12 行，都被点名的测试抓到；C 在自己的树上重跑一遍，同样 12 条都抓到）：

| # | 变异 | 抓它的测试 | 结果 |
|---|---|---|---|
| 54 | `writesNotebook` 不看升级 | `effective-role.test.ts` 的格；控件表 | 抓到 |
| 55 | `lostBy` 不认 `notebook.upgrading` | `edit-session.test.ts` 的心跳过期与保存两条；`page-editing.test.ts` 挂着的编辑器 | 抓到 |
| 56 | `modeChanged` 不重读升级 | `handlers.test.ts`；升级页的 mode 事件 | 抓到 |
| 57 | `refreshedOnConnect` 不含 `upgrade` | 升级页的"每次连上在笔记本之后重读升级" | 抓到 |
| 58 | 横幅在升级页上也带链接 | `upgrade-banner.test.tsx` 在升级页上的三格 | 抓到 |
| 59 | 任务页回滚的取消对写者也显示 | `transfer-page.test.tsx` 的回滚的取消（编辑者一格） | 抓到 |
| 74 | 任务转为结束时不重读升级 | 升级页"每秒读任务……停下之后读一次升级" | 抓到 |
| 75 | 轮询在任务结束之后不停（间隔恒为 1 秒） | 同上（推进 5 秒没有新的 `GET job`） | 抓到 |
| 76 | `pages-locked.tsx` 不认 `wiki.upgrade_pages_locked` | `pages-locked.test.tsx`；`general-page.test.tsx` 的列出页与编辑者 | 抓到 |
| 77 | 升级页对非管理员也发读（`admin` 恒真） | 升级页的编辑者、读者两格（没有 `GET upgrade`） | 抓到 |
| 78 | 接入一节对其他管理员也写 prompt 名 | 升级页的"另一位管理员……不写 prompt" | 抓到 |
| 79 | 发起成功之后跳转不带 `arrived` | `general-page.test.tsx` 的发起（标题有焦点） | 抓到 |

**控件表**（c1–c19，每处调用改回只看角色）：17 处各被控件表自己的一条抓到；c7（`canAddSource`）、c18（预览的后台恢复的提议）等价，见上。

**实施者自己加的 38 种**（x01–x38）：第一轮 34 种抓到：未建链接的 `upgrading` 一支（x01）；横幅的文字互换、链接给每个人、`rolling_back` 也写"可以放弃"、改成 `alert`（x02–x05）；发起按钮对 LLM Wiki、编辑者与读者、有升级时也出现（x06–x08）；N 只数直接的子节点、`raw` 下没有节点也写那一段、`template_locale` 写死 `en`（x10–x12）；store 不经 `inTurn`、成功之后不重读列表、`transfer.busy` 也重读、重读失败吞掉拒绝（x15–x18）；403 不重读笔记本列表（x19）；放弃在回滚中也显示、继续与结束在任务在跑时也显示、三个数换位、"已经复原完"在不全为 0 时也写（x21–x24）；`mode` 的升级键写错两种（x26、x27）；`upgrade` 放进重连的第二层（x28）；回滚的取消对管理员也藏起（x29）；`upgrading` 的失去不 `end()`（x30）；新码用 linking 的首句（x31）；`more` 不渲染（x32）；假服务端的冻结去掉（x33）；拒绝之后、做完之后不重读升级（x34、x35）；继续调成结束（x36）；接入一节在 `reviewing` 也显示（x37）；`transfer.busy` 用通用的文字（x38）。x09（发起按钮不等树）、x13（发起答复之后不核对页还在）、x20（404 当作读失败）、x25（做完之后焦点给按钮）补了测试（4 号三处、5 号一条），第二轮都抓到；x14 实际上等价，见上。

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，等价的除外）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | W3（`readsUpgrade` 恒真）、W4（只有 `succeeded` 才算结束）、W5（`wiki.upgrade_state` 被拒之后不重读）、W6（结束回滚不经 `inTurn`）、W16（三个数都为 0 也列数）、W17（不显示回滚的任务行）、W21（`behindRollback` 去掉 `notebook.not_found`） | 抓到 |
| A | W1（继续、结束做完之后焦点给仍在的按钮）、W2（去掉"每个任务只重读一次"的守卫） | 存活：S4A-N3 |
| A | W15（`upgrade` 挪到重连的第四层） | 存活，近乎等价 |
| A | 探针 1–3（不是变异） | 证实 S4A-N1；证实真服务端的次序下行为正确、S4A-N2 只是测试的问题；证实 S4A-N4 |
| B | b1–b12（"查看升级"对所有人、横幅的链接与句子不看管理员、`rolling_back` 仍写"可以放弃"、403 不重读列表、`transfer.busy` 不重读与用默认文字、`behindRollback` 去掉 `wiki.upgrade_state`、`behindStart` 去掉 `notebook.upgrading`、发起按钮不等树、回滚的取消对写者也显示、"继续"改成结束、成功之后不重读升级） | 都抓到 |
| B | o1（假服务端对内容的保存先查会话）、o2、o3（打出两种码序下的 `sent`） | o1 只有 `page-editing.test.ts:514` 失败：S4B-M1 |
| C | 7.3 的 S4 各行 12 种（重跑） | 都抓到 |
| C | c7、c18、x14 | 存活，c7、c18 等价，x14 实际上等价（核对过理由） |
| C | r01–r03、r05–r07、r09–r12、r14、r16–r18、r20 | 抓到 |
| C | r15（`frozenByUpgrade` 不看升级） | 单元的格抓到，在两处调用处冗余（C 的小结把它记作等价） |
| C | r08（"已经复原完"不看 `created`） | 存活：S4C-M1 |
| C | r13（`raw` 按大小写严格比较） | 存活：S4C-N1 |
| C | r04（去掉"只重读一次"的 Set）、r19（`lostBy` 的次序） | 存活：近乎等价、等价 |

A 10 个（7 个抓到、2 个存活、1 个近乎等价），另三个探针；B 12 个全抓到，另三次对照实验；C 重跑 7.3 的 12 行、核对实施者的三条等价、自己的 20 种（15 种抓到，r15 单元抓到，r08、r13 存活，r04、r19 近乎等价与等价），共 35 种 195 秒，先把 17 种命令在没改的树上各跑一次、都 rc=0。C 另核对"不出现、不可用、不再请求、焦点"四类各有一条改坏就红的测试，唯一没有钉住的是 r04 的 Set。

**修复一轮的变异**（实施者在头 `b1b66372` 上跑，规格 `m11/s4/fix1-mut/spec-all.json` 45 条，先在没改的树上跑 7 条命令都 rc=0，45 条全被杀死；连同第二轮的写成 [01-P1](../01-P1-freeze-start-rollback.md) 7.3 的第 124–132 条，第 123 条是 x14）：

| 来源 | 变异 | 杀死它的测试 |
|---|---|---|
| A 的 W1 | 继续、结束做完之后焦点给仍在的按钮 | 结束那一格（重读挂着，焦点在标题、不在仍显示的 End 上）；继续那一格杀不死，W1 对继续等价 |
| A 的 W2 | 去掉"每个任务只重读一次"的守卫 | "重读挂着时页面重新渲染"那一格；失败的那一格杀不死 |
| C 的 r08、r08b、r08c | "已经复原完"不看 `created`、`nodes`、`pages` | `{0,0,2}`、`{1,0,0}`、`{0,1,0}` 各一格 |
| C 的 r13 | `raw` 按大小写严格比较 | `Raw` 一格 |
| F1 | 假服务端回到"先答冻结" | `page-editing.test.ts` 的那一格 |
| F3 | `more` 不在描述里 | `confirm-dialog.test.tsx` 的那一格 |
| 新代码 | 做完之后焦点一律给按钮（W1b）、`settle` 对 `upgrading` 不 `end()`（F2）、发起不写缓存（N1a）、放弃与继续都不写（N1b）、只有放弃不写（N1c）、只有继续不写（N1d）、结束也写（N1e）、做完之后不重读（N1f） | 结束那一格、`edit-session.test.ts` 的防御一格、`general-page.test.tsx` 的发起一格、升级页的各格 |
| 旧变异重跑 | 与本轮改到的代码有关的 28 条（55、74、75、77–79、x06–x13、x19–x25、x30、x32–x38；x13、x32、x36 照新代码改写） | 都杀死；x14 现在就是 N1a，x25 由结束那一格杀死 |

**修复核对一轮**（在 `b1b66372` 上）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | 审查时的 10 个 | W1、W2 现在被杀死，W3–W6、W16、W17、W21 照旧被杀死，W15 照旧存活（近乎等价） |
| A | W1 只跑继续的格子、W2 只跑失败的那一格 | 都存活，印证实施者的两句说明 |
| A | 发起不写缓存、写缓存不带 `revalidate: false`、内容的保存冻结挪到会话检查之前、`more` 回到描述之外 | 都杀死 |
| A | 发起或放弃、继续只在挂着时写缓存（两个）、先重读后写缓存 | 存活：S4A-N6 |
| B | b1–b12 与新的 b13（内容的保存放回冻结的路由） | 都抓住 |
| C | r08、r08n、r08p、r13、r04（= W2，只被重新渲染那一格杀死）、r05–r07、r09、74、75、79、x14b、s1–s3、r17、W1（只被结束那一格杀死） | 都杀死 |
| C | r19 | 存活，等价 |

**修复第二轮的变异**（实施者在头 `e9af6107` 上跑，规格 `m11/s4/fix2-mut/spec-c1.json` 12 条，先在没改的树上跑两条命令都 rc=0，12 条全被杀死）：无条件写缓存（U1）、只比较 `data`（U2，被事件答 404 那一格杀死）、只比较 `error`（U3）、答复回来时才记下发出时的缓存（U4）、先重读后写缓存、放弃与继续只在挂着时写、发起只在挂着时写、写缓存不带 `revalidate: false`、发起之后不写缓存、W1、W2、做完之后不重读。修复核对第二轮 A 在 `e9af6107` 上：先重读后写缓存与两个只在挂着时写的都被杀死，无条件写被杀死；换成函数形式只被事件答 404 那一格杀死（只跑停下那一格时存活），印证实施者的偏离；比较整个状态对象存活，不算发现（"接受与推后的"）。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `b1b66372`（修复一轮 4 个提交），三位各在自己的工作树里 | A：S4A-N1–N4 关，S4A-Q1 由决定 253 关、文档待改；实施者与说明不同的三处都接受，收回审查时对 W2 的理由；新的 S4A-N5（答复盖掉事件读回来的更新状态，探针 4）、S4A-N6（写在重读之前、页面已离开也写没有钉住，探针 5 与三个存活的变异）。B：S4B-M1、N1 关，Q1 由决定 253 关；13 个变异都抓住；新的 S4B-N2（失败的那一格答真服务端答不出的 503）、S4B-N3（`ip` 没有超时）。C：S4C-M1、N1、N2、Q3 关，Q1 由决定 253 处置，Q2 照第 224 条进 M12；逐个提交都通过；10 轮网络探针与五轮页面故事；新的 S4C-N3（退路从 `startPostgres()` 之前起算），并疑 `dadfailed` 的地址 | 五条 Nit 全部接受，修复第二轮（`fix-2.md`）两个提交 |
| 2 | `e9af6107`（修复第二轮 2 个提交），A、C 在自己的工作树里 | A：S4A-N5、N6 与 S4B-N2 关；探针 4、5 在头上都过；比较 `data` 与 `error` 的偏离接受：实施者对 SWR 的说法属实，函数形式在事件的读答 404 时会出错，收回原来的建议；没有新发现。C：S4C-N3、S4B-N3 关；在一个 `--network none` 的容器里以一对 veth 配同一个地址造出一次 DAD 失败，`tentative -dadfailed` 只排除检测失败的地址、正在检测的照样列出，本机空闲时输出为空；不认这个写法的 iproute2 会把它当设备名，rc=1，落进 `catch` 走退路；探针 5 轮等待都在 IPv6LL 之后 31–95 毫秒结束、0 次失败，页面故事一轮 5 个都过；没有新发现 | 审查结束，S4 不再有修复轮 |

## 实测

- **vitest**（头 `3948f362`，`make test-web`，经 `heavy.sh`）：192 个文件、4,327 个测试，55.05 秒；`make test` 里那一次 55.17 秒（S3 时 189 个、4,121 个）。新的三个文件：控件表 `upgrade-read-only.test.tsx`、横幅 `upgrade-banner.test.tsx`、升级页 `upgrade-page.test.tsx`。
- **e2e**：`make e2e` 518 个，Playwright 报 2.2 分钟，整个目标 133 秒（S3 时 509 个；多的 9 个，C 对过：UP1 从 2 个测试变成 3 个，多的页面版本在 Chromium 一次；三个测试在 Firefox、WebKit 各一次，共 6 个；UP2、UP3 的页面版本各一个）。其中升级的故事：Chromium 上 UP1 的 API 114 毫秒、MCP 69 毫秒、页面 815 毫秒，UP2 的 API 492 毫秒、页面 848 毫秒，UP3 的 API 237、710 毫秒、页面 2.1 秒；Firefox 上 UP1 的 API 161 毫秒、MCP 66 毫秒、页面 1.6 秒；WebKit 上 UP1 的 API 179 毫秒、MCP 94 毫秒、页面 2.2 秒。UP1 的 API、MCP 两个测试因 `threeBrowsers` 在 Firefox、WebKit 各多跑一次，共多约 0.5 秒。
- **新的页面故事单独跑**（`--grep "\(page\)"`，经 `heavy.sh`；实施者四轮，`m11/s4/e2e-new-1.log`–`e2e-new-4.log`；第 2 轮在 `7c1890e2`，第 3、4 轮在 `3948f362`，之间只改了 vitest）：

  | 故事 | 实施者第 2 轮 | 第 3 轮 | 第 4 轮 | C 第 1 轮 | C 第 2 轮 |
  |---|---|---|---|---|---|
  | UP1（页面），Chromium | 1.0 s | 1.0 s | 1.0 s | 1.1 s | 1.1 s |
  | UP1（页面），Firefox | 1.6 s | 1.6 s | 1.6 s | 1.6 s | 1.7 s |
  | UP1（页面），WebKit | 1.7 s | 1.8 s | 1.7 s | 1.7 s | 1.8 s |
  | UP2（页面），Chromium | 0.90 s | 0.80 s | 0.80 s | 814 ms | 827 ms |
  | UP3（页面），Chromium | 2.4 s | 2.4 s | 2.5 s | 2.4 s | 2.4 s |
  | 整个（Playwright 报的） | 5.4 s | 5.6 s | 5.4 s | 5.8 s | 5.5 s |

  实施者第 1 轮（`7c1890e2`）UP3 在 Chromium 上失败（`net::ERR_NETWORK_CHANGED`，S4C-Q3），同一轮另四个都过。开发中另跑过两次：第一次三个失败，改了 UP1 的流（加 `holdStream`：Chromium 与 Firefox 上流在笔记本列表读到之前就连上了，连上的重读没有发生）与 UP3 的 404（偏离 18）；第二次五个都过。
- **WebKit 的那一眼**（第 224 条；实施者不提交的临时故事 `up1-webkit-look.spec.ts`，80 段的长页、视口 1280×800，流照 UP1 持住，第 40 段滚到视口中间，经 REST 发起，横幅经 `mode` 出现前后各记一次、截图，截图在 `m11/s4/webkit/`）：

  | 浏览器 | 之前 top / scrollY | 之后 top / scrollY | 横幅高 |
  |---|---|---|---|
  | WebKit | 372 / 2621 | 466 / 2621 | 70 |
  | Chromium | 373 / 2620 | 467 / 2620 | 70 |
  | Firefox | 373 / 2621 | 467 / 2621 | 70 |

  正文跳 94 像素（横幅 70 加它的 `mb-6` 24），`scrollY` 不变，三种浏览器都一样。原因由 C 查清（S4C-Q2）：C 的探针故事（`c/anchor/`，三种浏览器各两次，`anchor1.jsonl`、`anchor2.jsonl`）里经 REST 发起的一组三种都不补偿；横幅出现之后在它前面、或在正文第 1 段之前单插一个 94 像素的块，三种都补偿。
- **`ERR_NETWORK_CHANGED` 的复现**（C，`c/netchange/probe3.cjs`、`probe3.out`）：5 轮里 4 轮出现，各 6–15 个请求失败；失败的时刻与 veth 的"Gained IPv6LL"：09.942 对 09.939，16.856 对 16.852，31.063 对 31.059，38.039 对 38.035。用过的容器按 id 删掉了。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**（实施者的 `m11/tools/percommit-s4.sh`，一个干净的临时工作树、`pnpm install --frozen-lockfile --prefer-offline`，经 `heavy.sh`）：每个提交 `make build`、`make lint-web`、`make knip`、`make test-web`、`make gen-check`；没有一个碰 `server/`，所以不跑 `make lint-go`、`make test-go-short`。改写之前的六个每一项 rc=0（`percommit.out`；1–3 号之后没改写，这就是它们的结果）；第一次改写之后的 5、6 号每一项 rc=0（`percommit2.out`，128 秒）；第二次改写之后的 4–6 号，4 号每一项 rc=0，5、6 号 `make lint-web` rc=2（补的测试里的 `no-shadow`），其余 rc=0（`percommit3.out`，187 秒）；第三次改写之后的 5、6 号每一项 rc=0（`percommit4.out`，126 秒）。每步之后工作树干净。审查 C 另在自己的干净临时工作树里逐个跑 `make gen-check-web`、`lint-web`、`knip`、`build-web` 与经 `heavy.sh` 的 `make test-web`，6 个提交每一项 rc=0，vitest 的数随提交与报告一致。
- **建成时的头**（`3948f362`，`m11/s4/gates/`；开始前 `df -h /` 可用 26G，之后 25G）：`make lint gen-check knip build` 17 秒通过；`make test` 3,288 秒通过（竞态、组合根七份、成本、守卫两份、vitest 192 个文件 4,327 个测试 55.17 秒；同机还有别的 agent 的重的运行在等锁、跑在前后，开始时负载约 11–26；S4 没有改服务端，服务端的部分是对照）；`make test-go-short` 39 秒；`make e2e` 518 个 133 秒。
- **修复一轮**（`b1b66372`，`m11/s4/percommit-fix1/`、`gates-fix1/`；照决定 242 不跑完整的 `make test`）：4 个提交各自在干净的工作树里 build、`make lint-web`、`make knip`、`make test-web`（各 56 秒）、`make gen-check` 都是 rc=0，工作树干净，没有碰 `server/`；头上 `make lint gen-check knip build` 8 秒，经 `heavy.sh` 的 `make test-web` 55 秒（192 个文件、4,336 个测试），`make e2e` 518 个、2.2 分钟，没有 `ERR_NETWORK_CHANGED`；新的页面故事经 `heavy.sh` 单独五轮（`m11/s4/rounds-fix1/`），每轮 5 个过、约 6 秒。审查 C 另在干净的临时工作树里逐个跑：`gen-check-web`、`lint-web`、`knip` 都 rc=0，两个网页的提交另跑 `build-web` 与 `make test-web`（4,332、4,336 个测试），两个 e2e 的提交另跑 `make test-web` 与故事（`0d6eb002` 的 UP3 3 个，`b1b66372` 整个 `stories/upgrade/` 15 个），都过。
- **修复第二轮**（`e9af6107`，`m11/s4/percommit-fix2/`、`gates-fix2/`）：2 个提交各自 build、`make lint-web`、`make knip`、`make test-web`（各 57 秒）、`make gen-check` 都是 rc=0；头上 `make lint gen-check knip build` 11 秒，`make test-web` 57 秒（4,341 个测试），`make e2e` 518 个、2.2 分钟，没有 `ERR_NETWORK_CHANGED`；新的页面故事单独两轮，各 5 个过、约 6 秒，`startPostgres()` 返回之后 5–7 毫秒第一次看就是 tentative，等了 622、737 毫秒。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 作业 | 42722296424（建成，`3948f362`） |
|---|---|
| server (race) | 14:05（13:35） |
| server (bootstrap-1) | 5:31（5:10） |
| server (bootstrap-2) | 7:33（7:14） |
| server (bootstrap-3) | 6:06（5:44） |
| server (bootstrap-4) | 6:58（6:38） |
| server (bootstrap-5) | 7:01（6:37） |
| server (bootstrap-6) | 3:53（3:34） |
| server (bootstrap-7) | 4:17（3:55） |
| server (costs-1) | 13:08（10:07） |
| server (costs-2) | 16:49（16:26） |
| server (costs-3) | 17:11（16:45） |
| server (costs-lint) | 12:57（12:38） |
| server (guards-core) | 19:10（18:49） |
| server (guards-sources) | 13:39（13:19） |
| server (short) | 3:18（2:56） |
| web | 4:02（3:29） |
| e2e | 8:25（6:32） |
| image | 1:30 |

建成时一次全绿，18 项，普通推送（新分支，`git ls-remote` 是 `3948f362f0bc268e419f4bec40687ff4ee18237a`）；C 把每个作业的用时与 `gh run view` 的起止逐一对过，都一致，e2e 作业日志"518 passed (6.3m)"。guards-core 里 `main` 的 `e42558f2` 上红过的 M8 的 `TestStartRevertAtMaxJobItems/many_groups` 这次在 EPYC 7763 上过了：三次 2.019、1.968、1.909 秒，取最少 1.909 秒，界 2.1 秒，与 S4 无关（旁支照决定 249 调了界）。修复一轮的 run 42734556937（`b1b66372`，普通推送 `3948f362..b1b66372`）18 项全绿：e2e 作业 7:13（E2E 354 秒，上一轮 392 秒），guards-core 18:35；全局准备到"Running 518 tests"那一段 18.6 秒（上一轮 20.7 秒，C 量的）。修复第二轮的 run 42741553383（`e9af6107`，`b1b66372..e9af6107`）18 项全绿：e2e 作业 7:07（E2E 5:30，518 个过、5.3 分钟），那一段 19.6 秒。两次 CI 的其余作业的用时实施报告没有另记。

合并之后 `main` 的第一次 CI（run 42749131694，`ce370441`）18 项一次全绿，数见 [P1 设计](../01-P1-freeze-start-rollback.md) 10.4 的"合并之后"。
