# M11 总设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `dbce5828` 的 [00-M11-design.md](../00-M11-design.md)（初稿），对照 `dbce5828` 的代码（与 `7ff74a67` 相同，其后只改了文档）、总体设计与 4 份写给 M11 的移交 |
| 审查方式 | 三位审查者并行，只读仓库，实验都在自己的副本里。A：冻结、护栏、回滚（4.3–4.6、4.10、4.11、4.16）；在 `dbce5828` 的副本里跑了两个领域的探针：`zz_review_a_moved_test.go`（`batch` 的计划在"无关的移动之后改正文"与"索引过时"两种形状下都把 `Moved` 置真），以及给 `contract.go` 的保留标题一条加上 `OpRestore` 之后的 `zz_review_a_restore_test.go`（回收站恢复的预检的 facts 答 `page.virtual_file`），跑完复原 `contract.go`；没有起 PostgreSQL。B：数据、闸门、批准、代价（4.1、4.2、4.7–4.9、4.12、4.17–4.19）；`git archive dbce5828` 到副本，加一个 `wiki/app` 的单元测试 `TestReviewBJoinAfterSnapshot`（第一个调用者的快照已取之后，第二个调用者以同一个 `RunKey` 并入它的结果，`check=111 submit=111 computations=1`）；经 `heavy.sh` 起一次性的 `postgres:18.6-trixie`，照初稿原文建 00055 与 00057 的检查，逐条插反例（`checks.sql` 的 A–H：00055 在"状态为空、另三列有值"时接受；00057 接受五种状态机到不了的行）。C：接口、网页、Phase、约定（4.13–4.15、4.20，第 5、7–13 节）；在副本里跑 `TestTheBuiltInInstructionsAreShort` 量 SDK 的字节数（今天 3093/3163）；用脚本核对"总体设计的修订"里写出的 32 段旧文字各在 `v0.1-design.md` 里恰好出现一次；用 Python 按 `Makefile` 的六个正则把组合根的 568 个测试名分到各份 |
| 日期 | 2026-11-08 |
| 结论 | 先改 00 号文档，复核之后再开工 P1。A：Important 2、Minor 6、Nit 6、Question 1；B：Important 2、Minor 9、Nit 8、Question 3；C：Important 2、Minor 14、Nit 10、Question 4。全部处理：Important 照主持者的裁定修（A-I2 取审查者的第一条建议，另加主持者第 212 条；B-I2 两条都取）；Minor 与 Nit 照审查者的建议，一处没有照字面取（C-M13，理由在表里）；Question 由主持者定（212–214）或照审查者的意思；未核实的怀疑逐条核对，结果写在下面 |

## 主持者的决定

**开始 M11 之前的 172–186**：主持者在初稿之前交给设计者的说明（依据负责人 2026-11-08 的四条回答与三份只读调查），写进 00 号文档第 11 节"开始 M11 之前的主持者的决定"。审查之后改了两条、加注一条：

| 决定 | 审查之后 | 落在 |
|---|---|---|
| 175 冻结只管内容的写 | 加注：原文"预检用请求真实的客户端"没有可观察的效果，预检只判有没有升级（A-M6） | 4.5、第 11 节 |
| 180 闸门 6 的口径 | 改为逐页（B-I2） | 4.7、第 11 节 |
| 184 MCP 地址 | 改为网页上不显示 MCP 地址（C-I2） | 4.15、第 11 节 |

**设计审查之后的 212–214**（主持者的修订说明，负责人可以改判；00 号文档第 11 节"设计审查之后的主持者的决定"）：

| 疑问 | 定下的 | 落在 |
|---|---|---|
| A-I2、A-Q1 | 212：回滚停下之后，管理员的首选是"继续"；另给"结束回滚、保留现在的样子"：升级不批准地结束，标出回滚没有做完，解除冻结，记下剩下的条目；一个 REST 操作、矩阵的行、文字与 `destructive` 的确认对话框 | 4.3、4.10、4.15、第 5 节 |
| B-Q3 | 213：新建页让原有页的短链接改指新页，是 Obsidian 的语义，接受；写进 5.8 的修订与 playbook，报告不另列 | 4.6、4.13、修订第 10 条 |
| B-Q1 | 214：根下已有的 `raw` 照用作提议的来源根；发起的对话框写明它将成为来源区与节点数，agent 可以移出，报告计数 | 4.4、4.15 |

**设计者在修订中另做的取舍 215–224**（主持者的决定，负责人可以改判；00 号文档第 11 节）：

| 决定 | 内容 | 来自 |
|---|---|---|
| 215 | 提交的第二段脱离请求的上下文（`WithoutCancel` 加 `LintTimeout + 10 s`）；"审阅中、没有报告"由退回或再一次提交出来 | A-M5 |
| 216 | 存下报告也发 `mode`；网页的 `modeChanged` 也重读这本笔记本的升级；不在 `NotebookUpgrade` 加字段 | B-M5 |
| 217 | 结束回滚：只在回滚的任务停下时；结局不改，记 `rollback_complete = false` 与剩下的三个数 | 第 212 条 |
| 218 | 回滚的任务只有笔记本管理员能取消 | A 的未核实的怀疑 |
| 219 | 发起经 `Runs.Hold` 占 lint 的名额、受 `LintTimeout`；`startUpgrade` 加 `server_busy` | B-M8 |
| 220 | 发起提交之后对 `upgrade_nodes` 做一次 `ANALYZE`，给它 `MAINTAIN` | B-M9 |
| 221 | 驳回与放弃之后提交与报告留在行里 | B-M2 |
| 222 | 升级的权限、工具与 prompt 的矩阵写成自己的 `TestUpgrade…` 测试，进第七份 | C-M6、C-M3、C-M4 |
| 223 | 分份的正确性用记录里的 `go test -list` 证明，不列为变异 | C-M7 |
| 224 | 横幅照已有的滚动锚定；WebKit 上有没有一跳由 P1/S4 人工看一次 | C 的未核实的怀疑 |

另：设计者自己的 188、190、191、193、201、203、207、210 条照下面的处置改写，在 00 号文档里标出"设计审查 X 改"。

## 负责人 2026-11-08 的决定（背景）

初稿据以写成，审查不改它们（00 号文档第 11 节的前四条，原话"开 M11 吧，按标准流程推进"）：

1. 冻结不设时限：一直冻结到有人操作，任何笔记本管理员都能随时放弃，放弃时整体回滚。
2. 网页也能检查和提交，两条路走同一套闸门。
3. 批准之后不提供整体撤销，退回普通笔记本用降级。
4. 任何笔记本管理员都可以批准或驳回，发起人本人也可以。

C 问到这四条是否原话：出处是主持者交给设计者的负责人回答的记录，设计照它逐字抄；仓库里没有原始对话。第 212 条触及第 1 条的"放弃时整体回滚"，是主持者的决定，负责人可以改判。

## 发现与处置

节号是修订后的。几位审查者说到同一件事的合在一行。

### Important

| # | 发现 | 处置 | 改在 |
|---|---|---|---|
| A-I1 | 让 `CheckContract` 判 `OpRestore` 之后，LLM Wiki 里从回收站恢复根下 `index` 的一项会被预检拒绝：加序号只在单元里做，`restoreNode` 与 `startTrashRestore` 的预检用原来的名字造 facts（`restore_node.go:110-117`、`source_restore_precheck.go:51-58`），答一个两个操作都没声明的 `page.virtual_file` | 主持者接受：新规则只判撤销里的恢复（`OpRestore && Kind == ChangesetRevert`）；回收站与后台的恢复照旧加序号；加"恢复同一项答 200、名为 `index 2`"的正例（同步与后台）与变异"`OpRestore` 不分种类" | 4.11、第 3 节第 8 条、第 9 节、第 181 条 |
| A-I2 | 回滚的树的条目含升级中新建、仍活着的节点，没有界（冻结不设时限、新建不受限）；超过界时第一遍就停，继续重算同样的数再停；`ErrRevertUnordered`、一段放不进一批、缺版本同样循环；`rolling_back` 不能批准、退回，笔记本永久只读 | 取审查者的第一条建议：不含基线节点的新建子树在计划之外按单元删（每单元至多 `MaxUnitNodes`，更大的从下往上），不计入界；剩下的条目都是先删之后的活节点，写明规模之内到不了界；主持者第 212 条加"结束回滚"（`endUpgradeRollback`），第 217 条定写法；A-Q1 由此回答 | 4.3、4.10、4.15、4.17、第 5 节、第 193、212、217 条 |
| B-I1 | 提交的计算会并入提交之前开始的检查：`RunKey` 不带升级与提交的 id，`Do` 不看 flight 的快照何时取（`lint_runs.go:89-103`）；存下的报告可能不是冻结那一刻的，批准又只复核闸门 1–3（B 的单元测试复现） | 检查的键带升级的 id，提交的键带 `submission_id`、从不并入；加交错（检查的快照已取 → 发起人的写 → 提交）与变异 | 4.7、4.16 第 6 条、第 9 节、第 10 节 |
| B-I2 | agent 用 `update_links: false` 改名或移动原有页，原有页的链接断了或改指别处，字节不变；闸门 6 是全本净数（被新建的待建目标抵消、到 10,000 封顶即过），闸门 7 只看字节，两道都过 | 两条都取：(1) 护栏拒绝不改写链接的移动与改名（单个 `move` 看 `Options.UpdateLinks`，`batch` 让 `KeepLinks` 进 `Step`），新码 `page.upgrade_keep_links` 进 MCP 的码表、playbook 与 UP4；(2) 闸门 6 逐页：发起时记下每个基线页发出的待建目标数，检查时对区外的基线页逐页比；新建页的不算；第 180、188 条照改。场景 3 见第 213 条 | 4.6、4.7、4.13、第 5 节、修订第 14、15 条、第 180、188 条 |
| C-I1 | 存下的报告去掉游标就不合 `UpgradeCheck`（契约里八个摘要与条目把游标列为必填、`additionalProperties: false`），留着游标最坏约 98 KB，过了 65,536 字节；审阅页的"更多"接不上 | 存下的报告单立 `UpgradeReport`（不带游标、有界）；审阅页的"更多"从第一页读 `listUpgradeCheckItems`（审阅中冻结，现算与提交时一致；代价是每次占一个 lint 名额，写明）；契约测试核对一份存下的报告 | 4.7、4.15、第 5 节、第 9 节 |
| C-I2 | 横幅与升级页显示 MCP 地址，与负责人 M9 的决定（"网页上显示 MCP 地址的界面等 v0.1 之后的打磨"，`00-M9-design.md:648`）矛盾，却记成了主持者的决定 | 主持者改第 184 条：网页不显示 MCP 地址；升级页写 prompt 名 `upgrade` 与"照 README 的'接入 agent（MCP）'一节接入"；README 的升级一节写入口与 id 在网页地址里的位置（`README.md:259` 已有）；横幅只给管理员"查看升级"的链接 | 第 1、2 节、4.15、第 184 条 |

### Minor

| # | 发现 | 处置 | 改在 |
|---|---|---|---|
| A-M1 | `Folded` 按 `Moved` 判：两个正文操作之间有任何移动就为真，不论改写是否改了这一页（A 的探针）；"无关的移动之后改正文"答 `page.upgrade_batch_rewrite`、叫 agent 拆开，拆开之后又答 `page.upgrade_immutable`；拒绝标在第一个正文操作上 | `Folded` = 计划里这一页的某次改写真的改了内容；护栏的拒绝标在这一页的最后一个正文操作（计划的 `Last`）；反例"无关的移动之后改正文答 `upgrade_immutable`"与两条变异 | 4.6、第 9 节、第 190 条 |
| A-M2 | 照初稿把冻结的判断放进 `StartDeps.lock` 会冻住导出（`start.go:73` 也调它）；没有"冻结之外照常"的正例 | 判断放在导入建任务与 `startBackground` 里，不放进 `lock`；加 `TestUpgradeFreezeLeavesTheRest`（导出、改名称与开放程度、成员、删除笔记本、强制解锁，三种状态）与变异 | 4.5、第 3 节第 3 条、第 9 节 |
| A-M3 | 发起只判会话活着，不先删过期的行：一个在过期之前读了时钟的心跳可以把会话续活（M5 为此让开启先删）；第 7 步要的时刻在第 9 步才读 | 第 3 步锁下读一次时钟；第 7 步先删这本笔记本过期的会话行、再读活着的；加交错 | 4.4 第 3、7 步、4.16 第 3 条 |
| A-M4 | 回滚依赖"基线节点删不掉"，而删除的护栏在 P2/S1；P1/S4 之后、P2/S1 之前的 `main` 上可以从网页走进永久冻结 | 删除的护栏挪进 P1/S1（一条 `node_id = ANY` 的读）；第 207 条随之改 | 4.6、第 7 节、第 207 条 |
| A-M5 | 提交的第二个事务用请求的上下文：MCP 客户端在计算中断开，第二个事务开不起来，升级留在没有报告的审阅里，agent 自己出不来 | 第一个事务之后脱离请求的上下文（`WithoutCancel` 加 `LintTimeout + 10 s`，照 `job_run.go:186`）；进程崩溃时任何管理员退回或再提交一次（第 1 步认"审阅中、没有报告"），都有测试 | 4.3、4.8、第 9 节、第 215 条 |
| A-M6 | "预检用 `ClientWeb`"是杀不掉的变异：四个预检只经 REST 到达，客户端只会是 `web` 或 `api` | 删掉那条变异，换成"预检不判冻结"；预检只判有没有升级，`CheckRevert` 的签名不改；第 175 条加注 | 4.5、第 9 节、第 175 条 |
| B-M1 | 00055 的检查有 NULL 的洞：状态为空而另三列有值的行被接受（B 实测） | 改写为每个合取都不因 NULL 放过；反例加这一形状 | 4.2 |
| B-M2 | `notebook_upgrades` 的检查不把提交、报告、回滚的任务绑到状态上（B 实测五种到不了的行都被接受） | 加 `submission_state`、`report`（要提交、不在 `upgrading`）、`approved`、`rollback_job`、`rollback_end` 的检查，各有反例；驳回与放弃之后提交与报告留着（第 221 条） | 4.2、第 221 条 |
| B-M3 | 闸门 7 的"全部的"取自现读的基线行，检查排队时升级被批准或回滚完，0/0 通过；快照里不核对算的是哪一次升级 | 快照里先读升级列与 wiki 的行，不是同一次或不在 `upgrading`/`reviewing` 答 409；闸门 7 的分母取 wiki 行的数，现读不等时不过并记 error；加交错 | 4.7、4.16 第 8 条 |
| B-M4 | 闸门 7 比的账本由同一条写路径维护，恒真，复核不了护栏；"闸门 7 不比账本"的变异只有 SQL 改 `page_contents` 才杀得掉 | 主持者的处置：账本留作绊线，第 191 条写明它恒真的理由、杀它的测试只能用 SQL 绕过单元造；不写成独立的证明，报告与 5.8 写"核对" | 4.6、4.7、修订第 15、16 条、第 191 条 |
| B-M5 | 存下报告的那个转移不发 `mode`，`Notebook.upgrade` 也不变：开着的审阅页一直停在"没有报告" | 存下报告也发 `mode`；`modeChanged` 也重读这本笔记本的升级（`["upgrade", notebook_id]`）；不在 `NotebookUpgrade` 加字段：报告在 wiki 的表，notebook 读不到 | 4.3、4.8、4.12、4.15、第 216 条 |
| B-M6 | `ListNotebooks` 自己列列（`notebooks.sql:55-68`），不在"多选四列"的查询里：列表里的 `upgrade` 永远为空，网页放开每个编辑控件 | 改成四条查询；`listNotebooks` 的断言与变异 | 4.5、第 9 节 |
| B-M7 | 发起时补写的 `AGENTS` 的 id 没有存处，`seeded_page_ids` 答不出 | `seeded_agents` 布尔换成 `seeded_agents_id`；`checkPages` 核对它 | 4.2、第 5 节、第 9 节 |
| B-M8 | 发起时的那一次 lint 不占名额、不受 `LintTimeout`，与 M10 的移交相反 | `Runs.Hold`：同样的等待预算、只占名额、不开 goroutine；计算包在 `LintTimeout` 里；`startUpgrade` 加 `server_busy`；移交的落实逐条写 | 4.4、4.7、第 5 节、第 7 节、第 219 条 |
| B-M9 | 成本守卫没写统计状态，也没写预算；`upgrade_nodes` 是新表、写完立刻被读，没人 `ANALYZE` 它 | 第 9 节的成本守卫表：每个守卫的规模、三种统计状态 × 两种计划、预算（检查照决定 160 的例外，其余照 13.4 第 8 条的默认，放不下另立决定）；发起之后 `ANALYZE`、给 `MAINTAIN`；"新建的页"用 `NOT EXISTS` | 第 9 节、4.2、4.4、4.7、第 220 条 |
| C-M1 | `wiki.upgrade_state` 第一次出现在 P1/S3 的 `abandonUpgrade`，第 7 节却把它的码与文字派给 P2/S2，P1/S3 的网页一份会红 | 码、两种语言的文字、`adapter/http` 的答出移到 P1/S3；新的码表加"加的 Step"一列 | 第 5 节、第 7 节 |
| C-M2 | 两个工具的码的行漏了 `bad_request`、`validation_failed`、`notebook.not_found`，次序与 REST 不同 | 补全，次序照 REST | 第 5 节 |
| C-M3 | prompt 的矩阵与清单测试写死三个 prompt、两种模式，`upgrade` 方向相反、要第三、四种状态 | 按"prompt × 状态"的期望表改写，放进第七份的 `TestUpgradePromptMatrix` | 4.13、第 222 条 |
| C-M4 | 矩阵要一组升级的变体，不是"加一本升级中的笔记本"：九个操作各要不同的起始状态，不能把 `lab` 的笔记本设成升级中 | 照 M10 的 `wikiSeeded` 在单独的工作区建五本 × 三种状态（`upgrading` 闸门能过、`reviewing` 有报告、`rolling_back` 任务已停下），SQL 种下 | 4.14 |
| C-M5 | 第七份没有进 `make test-go` 的显式列表（`Makefile:190`），`make test` 会跳过全部 `TestUpgrade…` | 第 9 节写明 `Makefile:190` 的列表与 `:184` 的注释、CI 的注释、`06-environment.md` 的两处，P1/S1 改 | 第 9 节 |
| C-M6 | CI 的余量只算了第七份；M11 要扩写的矩阵在第三、四、五份，那几份已近 450 秒 | 升级的矩阵写成自己的 `TestUpgrade…` 测试进第七份，已有测试只改几行 | 4.14、第 10 节、第 222 条 |
| C-M7 | 变异"第三份不跳过第七份"没有会失败的测试 | 拿掉，改为 P1/S1 记录里的 `go test -list` 证明 | 第 9 节、第 223 条 |
| C-M8 | 总体设计里几处与 M11 不符而修订里没有（6.3 的 prompts 一句、12.4 的规则表、清理器、笔记本删除三行、8.5、6.1 的数据格式、5.8 图下一段）；第 6、14、19 条没写出旧文字 | 加修订第 7、18（数据格式）、19（prompts）、21、26（三行）条；第 6、15、20 条补旧文字；全部旧文字重新以脚本核对各出现一次 | 第 7 节"总体设计的修订" |
| C-M9 | 控件表漏了上传的重试（`upload-rows.tsx:48-51` 的 `canRetry`，编辑器的上传行用同一个组件） | 控件表加这一项，各一条 vitest | 4.15 |
| C-M10 | 回滚任务在网页上的报告：`shownCounts` 不由 tsc 强制、没有 `rollback` 时退回导入的计数；失败的文字是撤销的；假服务端的 `writingKinds` 不认 `rollback` | 加 `rollback` 的计数与失败文字；`test/jobs-server.ts:10` 与 `stores/transfer.store.ts:27` 一起加，P1/S3 带 | 4.15、第 7 节 |
| C-M11 | e2e 表没有写每个故事的 MCP 版本与豁免的理由 | 照 M10 第 9 节逐个写：UP6、UP7 加 MCP 版本；UP2、UP3、UP8 写豁免与盖住它们的 Go 测试；UP4 写 API 豁免 | 第 9 节 |
| C-M12 | 横幅与升级页的"接入 agent"对不能写的人也照样邀请；playbook 不说只有发起人的 agent 能写 | 随 C-I2 不再显示地址；升级页的接入一节对发起人（仍是管理员）与其他管理员分开写；playbook 写 `notebook.upgrading` 在升级中的意思 | 4.13、4.15 |
| C-M13 | wiki 的动作答 `Notebook`，但 wiki 没有按执行者读视图（角色、成员数）的端口 | 没有照字面取审查者的端口建议：`Notebook` 的 schema 在 `notebook.yaml`，wiki 的生成代码不能引用别的模块的生成包（`archtest/rules_test.go:41-42`），端口之外还要跨模块引 schema；改为十个操作答 wiki 自己的 `Upgrade`，网页在动作之后重读笔记本列表（第 201 条改） | 4.1、4.15、第 5 节、第 201 条 |
| C-M14 | 检查与条目写成"是 lint 的 `Execute`"：它判 `lint.read`，读者会读到升级的条目；没写交付前再判；移交的 P3/S1 补充有三点没落实 | 检查与条目是新入口，次序写明（判 `notebook.change_mode`、规则、状态、`limit`、算、交付前再判）；移交的落实把零 UUID、事务、名额与期限、交付前授权逐条写 | 4.7、第 7 节"移交的落实" |

### Nit

| # | 发现 | 处置 | 改在 |
|---|---|---|---|
| A-N1 | `RollbackEnd` 与任务结束的锁序没写，分两次或先写任务行会与笔记本删除死锁或留下"任务成功、升级未结束" | 在 `jobRun.finish` 的 `succeed` 里一个事务：笔记本行 → `RollbackEnd` 的写 → `FinishJob`（照导出的 `succeed`）；加交错 | 4.10、4.16 第 12 条 |
| A-N2 | 合成的条目的 `Revision` 必须是 0（否则 `sameVersion` 记 `missing`、停在 `changeset_trimmed`）；合成版本的 `Rewrite`、回滚单元的 `UpdateLinks` 没写 | 写明 `Revision` 为 0、`Restored` 为空、版本 `Rewrite = false`、单元 `UpdateLinks = false` 与理由；两条变异 | 4.10、第 9 节 |
| A-N3 | 删除护栏的拒绝默认标子树的根，可能是 agent 新建的文件夹 | 以第一个受保护的节点 `Refuse`；变异 | 4.6、第 9 节 |
| A-N4 | `checkPages` 的新不变式要限定在活着的笔记本；wiki 的表 P1/S2 才有 | 写明只看活着的笔记本，P1/S2 起，补 P1/S1 夹具的 wiki 行 | 第 7、9 节 |
| A-N5 | 同步的 `revertChangeset` 没有冻结的预检 | 写明不加及其理由（只多一次计划，守卫的码本来在领域的码之后） | 4.5 |
| A-N6 | 在 wiki 的事务里调 `startBackground`：它在外层提交之前就记"queued"，队列的 advisory 锁持到 wiki 的事务提交 | `Rollbacks().Start` 不记日志，由 wiki 在提交之后记；`Publish` 放在开任务之前 | 4.10、4.18 |
| B-N1 | 时钟在第 9 步才读，`computeIn` 又读一次 | 第 3 步锁下读一次；`computeIn` 收调用方的时刻 | 4.4、4.7 |
| B-N2 | `LIMIT 101` 数不出总数，而 `detail` 要写总数 | `LIMIT 100` 加 `count(*) OVER ()` | 4.4、4.17、第 203 条 |
| B-N3 | `NewUpgradeStates` 一并调订阅者，批准时次序会反（订阅者在重建之前） | 端口的方法分开（`Lock`、`Start`、`SetState`、`Approve`、`Clear`、`Publish`），批准的 `Publish` 在重建之后 | 4.1、4.9、第 8 节 |
| B-N4 | 发起时的"一个快照"靠的是锁，没写 | 写进 4.7：写 lint 所读的行的每条路径都先取笔记本行（单元、`reindex`、笔记本删除），发起持着 `FOR NO KEY UPDATE` | 4.7 |
| B-N5 | 00057 的 `locale` 没有约束名，`baseline_wanted` 与 `capped` 没照 `LintCount` 的不变式 | 加 `locale` 与 `baseline` 两个检查 | 4.2 |
| B-N6 | 批准时重建遇到缺元数据的附件，要报错而不是跳过 | 同单元的做法报错，批准 500、整个回滚；反例 | 4.9、第 9 节 |
| B-N7 | `checkPages` 的新不变式不够 | 加基线行数与 wiki 行的数相等、提议的根活着且属于这本笔记本、`seeded_agents_id` 属于这本笔记本 | 第 9 节 |
| B-N8 | "移进来源区的基线节点"怎样数没写，会数成"现在在区里的" | 按基线行的 `parent_id` 在 Go 里重建发起时的树、算发起时根的后代再相减 | 4.7 |
| C-N1 | 许可每个 M 定写法的是 `docs/README.md:103`，`:104` 是 M10 的一条 | 照改 | 第 7 节"提交说明" |
| C-N2 | 两行说明 SDK 编码下是 183 字节，之后 3276/3346，余 820/750 | 照改 | 4.13、第 10 节 |
| C-N3 | `stores/transfer.store.ts` 的 `writingKinds` 在 `:27`；`lockedText` 只读单个锁，读 `locks` 的是 `pages-locked.tsx:19` 且只认 `linking.pages_locked` | 改行号（初稿提交前的一次核对误改成了 `:28`）；`pages-locked.tsx` 放宽到新码 | 4.15 |
| C-N4 | 4.16 的总次序与 `listUpgradeCheckItems` 不一致 | 写明这个例外（照 `listLintItems`），并把规则名的 404 放在状态的 409 之前，与 lint 的规则页一致 | 4.7、4.16、第 5 节 |
| C-N5 | `EventMode.description`（`events.yaml:223`）也要改 | 一并改 | 4.12、第 5 节 |
| C-N6 | 横幅在升级页上会有指向自己的链接与第二份地址 | 升级页上不带链接；地址随 C-I2 不再显示 | 4.15 |
| C-N7 | `ci.yml` 的注释与 `06-environment.md` 的"六份、17 项"要在 P1/S1 一起改 | 写进第七份一条 | 第 9 节 |
| C-N8 | MCP 报告里闸门的节点只有 id | MCP 的投影给路径，不在活的树里的给 `null` | 4.7 |
| C-N9 | 设置的"模式"一节是 M10 加的，不是 M3 | 照改 | 4.15、第 8 节 |
| C-N10 | 人工清单 K：PAT 必须是发起人的；记一次护栏的拒绝；准备一节写怎样在网页上发起 | 照加 | 4.20 |

### Question

| # | 问题 | 定下的 | 落在 |
|---|---|---|---|
| A-Q1 | 回滚确定性失败时管理员能做什么 | 主持者第 212 条：结束回滚、保留现在的样子 | 4.10 |
| B-Q1 | 根下已有的 `raw` 文件夹被自动当作来源区 | 主持者第 214 条 | 4.4、4.15 |
| B-Q2 | 来源根可以就是根下的 `AGENTS` | 照审查者的意思：闸门 2 要求来源根不是根下的 `AGENTS`，批准时在锁下再核 | 4.7、4.9、修订第 15 条 |
| B-Q3 | 新建页改了原有页的链接指向 | 主持者第 213 条 | 4.6、4.13、修订第 10 条 |
| C-Q1 | 12.2 的规模写"中"还是"中偏重" | "中偏重"（三个 Phase、10 个 Step，冻结横跨每条写的路径） | 修订第 24 条 |
| C-Q2 | `checkPages` 的新规则从哪个 Step 起 | P1/S2 起；P1/S1 的夹具到 S2 补 wiki 的行，写进第 7 节 | 第 7、9 节 |
| C-Q3 | 只有 UP1、UP7 跑三种浏览器，对吗 | 对：其余对话框与 LW12 是同一个 `ConfirmDialog`、同一套焦点规则，LW12 已在三种浏览器里跑；写明理由 | 第 9 节 |
| C-Q4 | 网页上提交不过之后怎样 | 升级页自动再读一次检查、显示不过的闸门，占一个 lint 名额 | 4.8、4.15 |

### 未核实的怀疑：核对的结果

| 来自 | 怀疑 | 核对的结果与处理 |
|---|---|---|
| A | 发起的 `computeIn` 每条语句各取快照，挡不住在单元之外写链接索引的东西（例如服务运行时另开的 `reindex`） | 不成问题：`reindex` 先取笔记本行 `FOR NO KEY UPDATE`（`linking/app/reindex.go:94`），发起持着它时 `reindex` 等；单元与笔记本删除同样先取。推理写进 4.7（同 B-N4） |
| A | 合成的"变更集 C"比测过的都大、形状都杂，`schedule` 没在大的随机形状上跑过 | 是要补的测试：第 9 节加大的随机形状的性质测试（改名环、跨层移动、新建文件夹收着基线节点）；仍停下时落到"结束回滚"（第 212 条） |
| A | 非管理员的写者能否取消回滚任务 | 能：任务的发起人或笔记本管理员都能取消（`transfer/app/cancel.go:45-64`、`views.go:94-103`，规则 `transfer.cancel` 给读者以上），开回滚任务的管理员被降为写者之后还能取消。改为回滚的任务只有笔记本管理员能取消（第 218 条） |
| A | 大的回滚是否会碰到 River 的任务超时 | 会：`transfer.job_timeout`（`platform/config/config.go:252-254`）到时以 `timeout` 失败（`transfer/app/job_run.go:233-235`）；已做的批留着，继续重算差、能做完。写进 4.10 与网页的失败文字 |
| B | 发起在 10 万、30 万节点上的时长 | 没有在设计阶段量；第 9 节的成本守卫量 10 万节点，预算照 13.4 第 8 条的默认，放不下时那个 Step 另立决定；30 万进 M12 |
| B | `upgrade_nodes` 在两种统计下的计划；"新建的页"写成 `NOT IN` 会退化 | 成本守卫量三种统计状态 × 两种计划；"新建的页"写明用 `NOT EXISTS`；发起之后 `ANALYZE`（第 220 条） |
| B | 提交的第二个事务在请求被取消之后开不了 | 同 A-M5，第 215 条 |
| B | `listUpgradeCheckItems` 复用 `Execute` 要拆开才能插入 403 与 409 | 同 C-M14：是新入口，不复用 `Execute` 的授权，调 `computeIn` 的规则种类 |
| C | 横幅出现时 WebKit 上内容跳动 | 核实了页面靠窗口的滚动锚定（`pages/workspace/workspace-layout.tsx:107-113` 的注释），横幅插入与正文里的公式、图表变高是同一种情形；设计阶段不跑浏览器，没有在三种引擎里实测。接受，P1/S4 在 WebKit 里人工看一次并记录，有跳动进 M12 的打磨（第 224 条） |
| C | 负责人四条回答是否原话 | 见上"负责人的决定（背景）" |
| C | 第三、四、五份会长多少 | 升级的矩阵进第七份（第 222 条），已有测试只改几行；P1/S2、P2/S2 的记录照规则量各份 |
| C | MCP 报告的 `Fit` 从哪里截 | 写明：先截规则的条目，再截闸门的节点，计数与 `passed` 不截（4.7） |

### 过度设计

| 来自 | 指出的 | 处理 |
|---|---|---|
| A、B | 账本与每个正文步的一条 `UPDATE` 没有增加闸门 7 的判断力 | 主持者的处置：留作绊线，第 191 条写明它恒真、只防将来绕过单元的写，不写成证明 |
| A | `CheckRevert` 收请求的客户端、预检调完整的 `FreezeAdmits` | 去掉（A-M6） |
| B | 批准时的 `ForgetSources` 永远不改行 | 留着（与"从头重建"对称，代价是一条走索引的空 `UPDATE`），不另写说明 |
| B | `seeded_root`、`seeded_agents` 两个布尔可以推出 | `seeded_agents` 换成 `seeded_agents_id`（B-M7）；`seeded_root` 留着：wiki 读不到 page 的保护行，靠它答"补写的来源根" |
| C | `TestUpgradeFreezeAnswersFirst` 钉住的次序在实际里到不了 | 留着（便宜），4.5 与第 9 节写明它以 SQL 造出、不是用户看得到的效果 |
| C | 横幅与升级页都显示 MCP 地址 | 都不显示（C-I2） |

## 修订核对第一轮

| 项 | 内容 |
|---|---|
| 核对对象 | `fe9aa7d7` 的 [00-M11-design.md](../00-M11-design.md)（第一轮修订）与本记录，对照 `dbce5828` 的初稿（`git diff dbce5828 fe9aa7d7`）与主持者的第一份修订说明 |
| 核对方式 | 三位各自对照自己上一轮的发现，读修订在自己范围里新加的部分，对着 `fe9aa7d7` 的代码核对新引用的行。A：没有起 PostgreSQL。B：经 `heavy.sh` 起一次性的 `postgres:18.6-trixie`，照修订原文建 00055–00057 的约束，插反例并照 4.3、4.8、4.10 的写法走一遍状态机（`checks2.sql`）；在 `dbce5828` 的副本里加 `platform/postgres/zz_review_b_timeout_test.go`，用平台的 `TxManager` 在真库上复现"写事务里内层的期限到了"答成 500。C：用脚本（`seq_rev.py`、`check_rev2.py`）照第 7 节取出 45 对旧文字与新文字，在 `v0.1-design.md` 上依次替换，每段旧文字在替换的那一刻恰好出现一次 |
| 日期 | 2026-11-08 |
| 结论 | 旧发现：A 15 条里 14 条关、A-N1 大体关；B 28 条里 25 条关、3 条部分关（B-I2、B-M8、B-M9，各引出一条新发现）；C 30 条全关（C-M2、C-M3、C-M5、C-M6、C-M10、C-N7 各留一处 Nit），C-M13 的替代办法 C 认为成立。新发现：Important 1（B）、Minor 10（A 4、B 3、C 3）、Nit 20（A 5、B 7、C 8）、Question 1（C）。全部处理：Minor 与 Important 照主持者的裁定，Nit 照审查者的建议；设计者在这一轮另做的取舍是 225–231 |

### 三位的结论（旧发现）

- **A**：A-I1、A-I2、A-M1–M6、A-N2–N6 关闭；A-M1 关闭而表述有一处歧义（A2-N3）；A-M4 关闭而 e2e 的一处安排不对（A2-N4）；A-M5 关闭而引出 A2-M2；A-N1 大体关闭（`succeed` 里失败时的写法，A2-N1）；A-Q1 由主持者第 212 条答了。上一轮 A 的三条未核实的怀疑的核对结果都成立。
- **B**：B-I1、B-M1–M7、B-N1–N8、B-Q2 关闭，B-Q1、B-Q3 已答。部分关闭三条：B-I2（护栏与逐页都对，逐页的数法在代码里做不到，B2-I1）；B-M8（发起的期限超时答成 500，B2-M1；在锁下等名额，B2-N1）；B-M9（`MAINTAIN` 漏了没有测试失败，B2-M3）。B 实测 00055–00057 的约束：初稿接受的五种行都被挡住，状态机到得了的行都写得进；例外是 B2-M2 与 B2-N5。
- **C**：30 条全关。留下的 Nit：C-M2 → C2-N1，C-M3 → C2-N2，C-M5、C-N7 → C2-N5，C-M6 → C2-N3，C-M10 → C2-N4。C-M13 的替代办法（动作答 `Upgrade`）成立：答 `Notebook` 要么让 wiki 的生成代码引 notebook 的生成包，违反 `archtest/rules_test.go:42`，要么把 `Notebook` 搬进 `common.yaml`；唯一的代价是动作答复到列表重读之间 `Notebook.upgrade` 旧一瞬，可以接受。45 对旧文字与三处锚点各恰好出现一次。

### 主持者的裁定

- **A2-M1**：接受。凡带 `UpgradeOwn` 的单元，标记不等于笔记本当前的升级（含已没有升级）就拒，放在守卫的第 1 步之前。
- **A2-M2**：接受。"审阅中、没有报告"只在 `submitted_at + LintTimeout + 10 秒` 之后才能被重新提交或退回接管；之前答状态的码。第 215 条照改。
- **A2-M3**：接受。`rollback` 进 Rescue 的种类表，`TestTheRescue` 改成逐个核对全部种类。
- **A2-M4**：取"写明"，不做"每批之间链接一致"。
- **A 的问题（`Runs.Hold` 在笔记本行的锁下）**：只许立即取，取不到就答 503 带 `Retry-After`，从不在行锁下等名额；写明并有测试。B2-N1 随之了结。
- **B2-I1**：取第一种：发起时对提议的来源区里的基线页另读一次，照来源区以外同样的口径数；`upgrade_nodes_page_check` 不放宽。
- **B2-M1**：接受，用答得出 503 的那种，写明为什么不再是 500，有测试。
- **B2-M2**：接受，先开任务，再一条 `UPDATE` 写 `rolling_back`、结局与任务的 id。
- **B2-M3**：接受审查者的两处测试的改法。
- **C2-M1**：接受。停下与结束各经订阅链发 `mode`（与 A-N1 的"一个事务"一起写），进度照已有的任务界面的做法。
- **C2-M2**：接受，升级页的键进 `refreshedOnConnect`。
- **C2-M3**：接受，存下或给成本守卫，取便宜的那种，写明。
- **C2-Q1**：第 212 条仍是主持者的决定（负责人可以改判），不阻塞；第 11 节写明它放宽了负责人第 1 条，并在 M11 的汇报里第一条请负责人看。主持者 2026-11-08 已在对话里告诉负责人。
- **Nit** 与 C-M3 的剩余一处：照审查者的建议改。

### 新发现与处置

| 编号 | 级别 | 一句话 | 处置 | 改在哪里 |
|---|---|---|---|---|
| B2-I1 | Important | 来源区里的页在 SQL 里就被排除（`linking/adapter/postgres/queries/lint.sql:18`、`:34`），从 `raw` 移出的原有页基线记为 0，闸门 6 误报 | 主持者取第一种：发起时对提议的来源区里的基线页另读一遍（`= ANY(这些页)`），同一事务、同一把锁、同样的目标判定；去掉"在 `generalEmitter` 之前数"；有测试与变异，进发起的成本守卫 | 4.4 第 13 步、4.7 闸门 6、第 8 节 linking、第 9 节、第 187、188 条 |
| A2-M1 | Minor | 结束回滚之后，Rescue 记为中断而 worker 仍在跑的旧回滚单元照第 1 步放过，会删用户的新页 | 守卫第 1 步之前拒过期的 `UpgradeOwn`（`ErrStaleUpgradeWrite`），回滚的每一遍核对升级；交错第 16 条与变异 | 4.5、4.10、4.16、第 9 节、第 225 条 |
| A2-M2 | Minor | "审阅中、没有报告"也是正在算的状态，再提交会顶掉正在算的那一次 | 只在 `submitted_at + LintTimeout + 10 s` 之后接管，之内 409 `wiki.upgrade_state`（`detail`"上一次提交正在计算"）；退回同样 | 4.3、4.8、4.15 审阅、4.16 交错第 15 条、第 10 节、第 215 条 |
| A2-M3 | Minor | `rollback` 不在 Rescue 的种类表（`transfer/app/maintain.go:108`），River 丢掉的回滚任务永远 `queued`，继续与结束都答 `transfer.busy` | 进表，`riverKind`、worker、`Kind.Writes()` 同改；`TestTheRescue` 逐个核对全部种类；整个程序的测试与变异 | 4.10、第 8 节 transfer、第 9 节 |
| A2-M4 | Minor | 结束回滚时还没写回正文的原有页的链接可能指向空处（`schedule` 最后才写回，`revert_schedule.go:313-342`） | 主持者取"写明"：第 212 条、对话框、`Upgrade.rollback_left` 的说明写明，健康度面板列出 | 4.10、4.15、第 5 节 `Upgrade`、第 10 节、第 212 条 |
| B2-M1 | Minor | 写事务里内层的 `LintTimeout` 到期时 `ROLLBACK` 失败、错误只剩文字，答 500（已复现） | 整个发起的事务开在 `ctxStart` 上，返回之后按 `ctxStart.Err()` 答 503；不用 `SET LOCAL statement_timeout` 与理由；测试断言 503、没有 error 日志 | 4.4"期限"、4.7、第 9 节、第 219、227 条 |
| B2-M2 | Minor | 先写 `rolling_back` 与结局、后记任务的 id 过不了 `notebook_upgrades_rollback_job_check`（已复现） | 先 `SetState`、`Publish`、开任务，最后一条 `UPDATE` 一起写 `state`、`ending`、`rollback_job_id`；适配器测试 | 4.10"开始"、第 9 节 |
| B2-M3 | Minor | `upgrade_nodes` 的 `MAINTAIN` 漏了也没有测试失败 | `runtime_role_test.go` 的 `analyzed`（`:389-392`）加 `upgrade_nodes`；`TestTheRuntimeRoleServesWithTheGrantsFile` 以运行时角色发起，断言 `last_analyze` 变了、没有 warn；变异 | 4.4 第 16 步、第 9 节、第 220 条 |
| C2-M1 | Minor | 回滚停下之后升级页看不出来：没有轮询，停下也不发 `mode` | 回滚任务的 `fail` 与 `succeed` 对称（一个事务：笔记本行、`FinishJob`、`RollbackEnd.Stopped` 发 `mode`）；升级页照任务页轮询任务（`transfer-page.tsx:46`、`:178`），结束时重读升级；vitest | 4.10、4.12、4.15、第 8、9 节、第 228 条 |
| C2-M2 | Minor | `["upgrade", notebook_id]` 不在 `refreshedOnConnect`，重连之后不重读 | 进第三层（`app/event-stream.tsx:15-34`）；vitest 与变异 | 4.12、第 8、9 节 |
| C2-M3 | Minor | 剩下的差的读没有成本守卫，`endUpgradeRollback` 却在笔记本行的锁下做它 | 取现读加成本守卫（停下有不经 wiki 的路，存下的数会缺）：10 万节点、10 万页、三种统计状态、请求 4 s；风险表加结束回滚 | 4.10、第 9 节成本守卫、第 10 节、第 226 条 |
| A2-N1 | Nit | `RollbackEnd` 在 `succeed` 里失败时任务留在 `running`（`job_run.go:189-190`、`jobs.sql:67-70`） | 先回滚结束的事务，第二个事务 `FinishJob(failed, internal)` 与 `Stopped`；测试 | 4.10"做完"、第 9 节 |
| A2-N2 | Nit | 规则放在 `views.sees` 会连读一起挡 | 放进 `Cancel.Run`（`sees` 之后），答 403 `forbidden`，读与列表不变 | 4.3、4.15、第 5 节、第 8 节、第 218、229 条 |
| A2-N3 | Nit | `Folded` 在写的路径上有歧义；能标 `Last` 的只有两个码 | 写的路径要这一次写等于改写的结果；两个正文的码标 `Last`，`keep_links` 标移动，`protected` 标删除 | 4.6、第 8 节 page、第 9 节、第 190 条 |
| A2-N4 | Nit | UP4 的删除一步在 P1/S1 跑不了 | 改为 P1/S2 起；P1/S1 由 `TestUpgradeGuardrails` 以 SQL 设列盖住 | 第 9 节 UP4 |
| A2-N5 | Nit | 先删一个单元一棵子树，10 万个零散的新页就是 10 万个单元 | 一个单元装多棵、合计至多 `MaxUnitNodes`；进成本守卫 | 4.10、第 9 节、第 193、231 条 |
| B2-N1 | Nit | 第 9 步在锁下等名额 | 随主持者对 A 的问题的裁定：`Runs.TryHold` 只立即试取 | 4.4 第 9 步、4.7、4.16、第 219 条 |
| B2-N2 | Nit | 条目一路的键与快照没写 | 条目的键带升级的 id，快照核对升级的 id 与状态 | 4.7"名额与期限"、第 9 节 |
| B2-N3 | Nit | `logFinished` 与 `Stop` 不认新入口 | `logFinished` 给新种类名字；`TryHold` 的持有者计入 `active`，`Stop` 之后答不可并等持有者 | 4.7、第 8、9 节 |
| B2-N4 | Nit | 变异表里账本那一条对错了测试 | 分成两条："账本不在改写时更新"由普通测试杀，"闸门 7 不比账本"由 SQL 绕过单元的测试杀 | 第 9 节变异 |
| B2-N5 | Nit | 表允许"驳回而没有提交"的行（实测） | 加 `notebook_upgrades_rejected_check` 与反例 | 4.2 的 00057、第 9 节 |
| B2-N6 | Nit | 成本守卫漏了剩下的差的现读与检查时逐页读 `wanted` | 各进成本守卫表 | 第 9 节 |
| B2-N7 | Nit | sqlc 解析不了 `FROM` 里多参数的 `unnest` | 照 `SetSourceHashes` 在选择列表里各 `unnest` 一个数组（`sources.sql:15-17`、`:46-54`） | 4.4 第 13 步 |
| C2-N1 | Nit | MCP 表头"照 REST 的次序"不准 | 写成"照 REST 的次序；参数的 `validation_failed` 照 `lint` 的行在前"（`permission_matrix_mcp_test.go:114`） | 第 5 节 MCP |
| C2-N2 | Nit | `upgrade` 进 `prompts.json` 后两个原测试出问题 | `TestThePromptReadsLLMWikiMatrix` 只取三个 LLM Wiki 的 prompt（`:177`、`:205`）；`TestThePromptListsAreTheContract` 改为四（`mcp_prompts_test.go:96-106`） | 4.13 |
| C2-N3 | Nit | "每个码都答出过"的测试在哪、P1/S1 怎样种没写 | 放进第七份的 `TestUpgradeToolsErrorsCarryTheirCodes`，核对读两边；P1/S1 以 SQL 设列 | 第 5 节 MCP |
| C2-N4 | Nit | `TransferFailure.description` 没提回滚 | 第 5 节"改的 schema"加一行，P1/S3 | 第 5 节 |
| C2-N5 | Nit | 第七份漏了几处写份数的注释 | `Makefile:183`、`:185`、`:195`、`:272`，`ci.yml:48`；`06-environment.md:27` 与 `:49` 分开写 | 第 9 节第七份 |
| C2-N6 | Nit | UP4 的"P1/S1 起"对不上 | 同 A2-N4 | 第 9 节 UP4 |
| C2-N7 | Nit | 差已空时结束回滚仍记 `false` | 三个数都是 0 时记 `true`、不记三个数；对话框"已经复原完，结束即可" | 4.3、4.10、4.15、第 217、230 条 |
| C2-N8 | Nit | 只改 `views.sees` 时列表与读不一致 | 同 A2-N2 | 同 A2-N2 |
| C2-Q1 | Question | 第 212 条是否列为"需要负责人确认" | 主持者：不阻塞；第 11 节写明它放宽负责人第 1 条，M11 的汇报里第一条请负责人看 | 第 11 节第 212 条与"需要负责人确认" |

### 未核实的怀疑：核对的结果

| 来自 | 怀疑 | 核对的结果与处理 |
|---|---|---|
| A | `Runs.Hold` 在笔记本行的锁下会不会等名额 | 主持者的裁定：只立即试取（`Runs.TryHold`），交错第 17 条与变异 |
| C | `HealthRule` 在没有游标时藏起"更多"（`health-rule.tsx:210`） | 属实；"从第一页读"做成 `HealthRule` 的参数，P3/S3 写代码时核对，写进 4.15 审阅 |
| C | 剩下的差的读在 10 万节点上要多久 | 同 C2-M3，进成本守卫 |

### 设计者在这一轮的取舍 225–231（主持者的决定，负责人可以改判）

- **225**：守卫先拒过期的 `UpgradeOwn`（`ErrStaleUpgradeWrite`，不是给用户的码），回滚的每一遍核对升级（A2-M1）。
- **226**：剩下的差现数、给成本守卫，不在停下时存下：停下有不经 wiki 的路（Rescue、崩溃）（C2-M3）。
- **227**：整个发起的事务受 `LintTimeout`，到期答 503；不套内层的期限，不用 `SET LOCAL statement_timeout`（B2-M1）。
- **228**：回滚停下也发 `mode`（与 `succeed` 对称的 `fail`），升级页轮询回滚的任务（C2-M1）。
- **229**：取消回滚的规则放在 `Cancel.Run`，答 403 `forbidden`，读与列表不变（A2-N2、C2-N8）。
- **230**：结束回滚时差已空就记做完（C2-N7）。
- **231**：先删一个单元装多棵子树，合计至多 `MaxUnitNodes`（A2-N5）。

另照裁定改了第 187、188、190、193、212、215、217、218、219、220 条。
