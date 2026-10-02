# M4/P1 页面模块与写入管线：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m4-p1-page-module-pipeline`（`main...4f64a84`：S1–S5 共 7 个提交，S1、S3 各两个；98 个文件，+8769/−67。审查请求里写的"六个提交、89 个文件、约 +8300"与 `git diff --stat` 不符，以这里为准），对照 [01-P1-page-module-pipeline.md](../01-P1-page-module-pipeline.md) 3.1–3.14、第 4、5 节，五份 Step 计划，[M4 总设计](../00-M4-design.md)第 4、5、8、9 节，[M3/P1 移交](../handoffs/M3-P1-notebook-deletion.md)、[M2/P4 移交](../handoffs/M2-P4-purge-page-tree.md)，总体设计 13.1（第 4、5、6、9、21 条） |
| 审查方式 | 独立审查者在 `git archive 4f64a84` 的快照上实测（快照里 `git init` 过），仓库与别的 Docker 容器都没动过：<br>• 门禁：`GOFLAGS=-p=3 go test -race -count=1 ./...`，48 个包全部 ok；`make lint`（Go 两处 0 issues，前端 lint、format、types 通过）；`make knip` 通过；vitest 63 个文件 1000 个测试通过；`server/tools` 的测试、`TestCheckCostsAboutTheBody`、`make build-web` 通过；`make gen-check` 退出 0、无差异；`make e2e` 122 个全部通过；<br>• 重复：页面的五个故事 `--repeat-each 3`，15 次全部通过；PG13 另跑 `--repeat-each 20`，20 次通过；page、notebook、shared 各包 `-race -count=5` 全部 ok；交错 30–33、三条删除路径、整个清理任务、树与逐项读取一致、清理顺序、规则表 `-race -count=5 -v`，45 次 PASS、0 失败；<br>• 中间提交：S1–S4 的六个中间提交各自 `go vet ./...` 与 `go test -short ./...`，全部通过；<br>• 反向对照：Go 72 项（54 项失败、18 项存活，其中 2 项在不变量之下等价），e2e 4 项（汇总见下） |
| 日期 | 2026-10-02 |
| 结论 | 可以合并，合并前补上 T1–T8 的测试并改 PG13 的标记（T7）。<br>• 处置：表中 8 项 Minor（T1–T8）、7 项 Nit（T9、T10、N1–N5）都在合并前处理（修复 `3ec2c9d`，`bb90d03` 合并），每个新检查都做了反向对照，见"发现与处置"；D-a 的加固（`apply` 拒绝不带单元事务的 ctx）已加；Q1 在 P2 开工时拆分，Q2、Q4 写进给 M5、M9 的移交，Q3 的措辞随文档提交改；文档的 11 处不一致在合并之后的文档提交里同步。<br>• 写入单元的次序（事务之外读工作区 → 工作区行 `FOR SHARE` → 笔记本行，树操作 `FOR NO KEY UPDATE` → 判定 → 422 → 409 → 守卫 → 变更集、写入、条目与版本 → 参与者；之后观察者一次）、只读一次时钟、已有事务时报错，都与设计一致；没有先写后判、在锁外判定、或参与者追加的操作绕过守卫的路径。<br>• 码的次序与各操作的 404 码、标题键与 `NULLS NOT DISTINCT`、只差大小写的改名、次序与重排、清理的循环与 `NOT EXISTS`、`SKIP LOCKED`、清理顺序、笔记本删除的注册者（三条路径、事件的时刻、回收站里的行保留原时刻）、契约与适配器、加锁顺序，都核对过，没有发现生产代码的缺陷。<br>• 没有 Critical、Major。8 项 Minor 都是测试守不住的行为（反向对照存活）或会偶发失败的断言，5 项 Nit。D-a…D-h 都同意，D-a 附一条加固建议，D-g 的 PG13 部分不同意（T7） |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Minor | 交错 32 区分不出"树操作持笔记本行的独占锁"。把单元的树锁改成 `FOR SHARE`（`unit.go:103`，反向对照 TREE-SHARE），交错 30–33 全部照样通过：32 的两次新建在 `FOR SHARE` 下并行，都读到没有同名，后插入的撞上 `nodes_notebook_id_parent_id_name_key_idx`，`CreateNode` 把 23505 译成 `page.title_taken`，答复同样是 201 + 409。P1 文档 3.4、M4 总设计第 9 节与测试的注释（`interleavings_page_test.go:178-180`："the unique index is never reached"）说 32 证明了锁下就看得到对方，实际上它证明不了；只有用例层假端口记下的 `LockNotebook in tx`（`TestCreatePageLocksThenDecides`）守住这把锁。在 `FOR SHARE` 下并行的新建会算出同一个 `sort_order`、重排互相覆盖，P2 的防环与层级判断也全靠这把锁。建议：`checkPages` 加一条"未删的兄弟没有相同的 `sort_order`"，32 再加一种"两个不同的标题同时新建在同一父页的最后"：串行时两者的次序是 0 与 1，`FOR SHARE` 下两者都是 0，只有串行才过 | 交错 32 改为持笔记本行的 `FOR SHARE`（`sharedNotebookRow`），与不改树的页面写持的锁相同：新建只因为锁树才等它，第二个在第一个提交之后的锁下读到同名。注释改为测试实际证明的。反向对照：TREE-SHARE，交错 32 两种先后都在等锁的期限失败（没有语句等笔记本行） |
| T2 | Minor | 清理里 `page_revisions` 一支与变更集的 `SKIP LOCKED` 没有测试。`TestPurgeKeepsWhatAHeldFollowerNeeds`（`adapter/postgres/purge_test.go:192`）只持有正文与条目。去掉 `PurgeNodeLeaves` 的 `NOT EXISTS page_revisions`（PURGE-NOREV）、`PurgeChangesets` 的 `NOT EXISTS page_revisions`（PURGE-CS-NOREV）、`PurgePageRevisions` 的 `SKIP LOCKED`（PURGE-REV-NOSKIP）、`PurgeChangesets` 的 `SKIP LOCKED`（PURGE-CS-NOSKIP），page 的全部测试照样通过。失败场景：版本行被别的事务持有时，版本的清理器会等锁；或者版本被跳过之后，节点与变更集的删除经 `ON DELETE CASCADE` 去碰这一行而等锁，违反 13.1 第 6 条"从不等锁"（与 M2/P4 审查 T3、M3/P1 审查 T2 同一类）。建议：在这个测试里再持有一行版本（例如 `d`/`g` 之外另一组的）与一行变更集，断言各清理器的数目，并保持 5 秒期限 | `TestPurgeKeepsWhatAHeldFollowerNeeds` 换成表格测试 `TestPurgeKeepsWhatAHeldRowNeeds`：分别持有 g 的正文、g 的版本、c 的条目与 g 自己的变更集（夹具给 g 一个单独的变更集），核对两次运行在五张表各删几行。反向对照：PURGE-NOREV、PURGE-CS-NOREV、PURGE-REV-NOSKIP 让"g's version"失败，PURGE-CS-NOSKIP 让"g's changeset"失败 |
| T3 | Minor | 三个扩展点的分发循环没有"两个注册者"的测试（13.1 第 21 条："分发循环按登记的顺序调用每一个、第一个错误即停，由两个注册者的测试守住"）。`unit.go:324`（守卫）、`:343`（参与者）、`:395`（观察者）三处改成倒序（GUARD-ORDER、PART-ORDER、OBS-ORDER），或守卫出错之后继续调用其余守卫再答第一个错误（GUARD-NOSTOP），用例层与模块根的测试全部通过：所有测试都只登记一个注册者。M5 的编辑锁与 M10、M11 的守卫会同时登记，先后与"第一个错误即停"会直接决定答哪个码。建议：照 `workspace/app/registrants_test.go` 各加一个两注册者的用例（记录调用次序；第一个出错时第二个没有被调用） | `unit_test.go` 加 `TestTheRegistrantsRunInTheirOrder`：两个守卫、两个参与者、两个观察者，核对调用次序；第一个守卫拒绝时答它的错误，第二个守卫没有被调用。反向对照：GUARD-ORDER、PART-ORDER、OBS-ORDER、GUARD-NOSTOP 都让它失败（GUARD-NOSTOP 另让码的次序与回滚的测试失败） |
| T4 | Minor | 变更集条目的父页与次序没有任何测试核对。`RecordItem` 不写 `after_parent_id`（ITEM-NOPARENT，`adapter/postgres/changesets.go:28`）或把 `after_sort_order` 写成 0（ITEM-NOSORT），Go 测试全部通过，ITEM-NOPARENT 带上改名审计的变异重新构建之后页面的五个故事也全部通过：`TestRecordItemAndRevisionMerge`（`store_test.go:204`）、模块根的参与者测试与 e2e 的 `expectNewPage`、`expectRenamed`（`e2e/fixtures/assert/page.ts:41`、`:65`）都只比较名称。条目的前后父页与次序正是 M8 判断"当前状态仍是这个变更集写的"所比较的（M4 总设计第 4 节"次序"），P2 的移动条目更是只有父页在变。建议：仓储测试对一个子页的新建与改名核对条目的 `before_/after_parent_id` 与 `_sort_order`，`expectNewPage` 加上 `i.after_parent_id IS NOT DISTINCT FROM n.parent_id AND i.after_sort_order = n.sort_order` | 仓储测试 `TestRecordItemKeepsTheTreeStates` 对子页的新建与改名读回条目的前后父页与次序；e2e 的 `expectNewPage` 核对条目的 `after_parent_id`、`after_sort_order` 等于节点的，`expectRenamed` 核对改名条目的前后父页与次序都是节点所在的位置。反向对照：ITEM-NOPARENT、ITEM-NOSORT 让仓储测试失败；ITEM-NOPARENT 重新构建后 PG1 失败（PG2 的页都在根下，比较不出父页） |
| T5 | Minor | 改名的审计列没有测试核对。把 `RenameNode` 的 SQL（`queries/nodes.sql:52-55`）改成不写 `updated_by_id`、`updated_at`（RENAME-SQL-AUDIT），Go 全部测试与重新构建后的页面故事都通过：用例层用的是假仓储，矩阵的 `renameNode` 行只比较名称，PG2 的 `expectRenamed` 虽然比较 `updated_by_id`，但改名的人也是建页的人（`pg2-rename-page.spec.ts:15-24`，同一个 `adminId`），比较不出来；`updated_at` 没有人比较。契约说 `updated_at` 是"最后一次新建、改名或移动的时刻"，答复也经重读取它。建议：`TestNodesReadBack` 或新的仓储测试改名之后读回 `UpdatedBy`、`UpdatedAt`；PG2 让另一位编辑者改名，`expectRenamed` 再比较 `n.updated_at` 等于那次变更集的 `created_at` | 仓储测试 `TestRenameNodeWritesItsAuthor`：另一位用户改名，读回 `UpdatedBy`、`UpdatedAt`，建页的两列不变。PG2 改由工作区成员（笔记本对工作区开放为 `editor`）改名，`expectRenamed` 另核对 `n.updated_at` 等于这次变更集的时刻、变更集的执行者是改名的人。反向对照：RENAME-SQL-AUDIT、只不写 `updated_at` 的 RENAME-SQL-NOAT，仓储测试与重新构建后的 PG2 都失败 |
| T6 | Minor | `TitleKey` 的第一次 NFC 没有测试（反向对照 TK1：`NFC(fold(s))` 全部通过）。审查者在快照里穷举得到反例：`"àͅ"` 与它的规范等价形式 `"àͅ"`，按现在的写法都得到 `"àι"`；去掉第一次 NFC，前者先把 U+0345 折叠成 ι 再与重音组合，得到 `"aὶ"`，两个规范等价的字符串键不同。P1 里标题先经 `CheckTitle` 变成 NFC，这一步暂时是空操作；但 M6 的链接解析把链接的目标文本直接交给同一个函数（P1 文档 3.2），那里它承重。建议：`shared/title_key_test.go:9` 的表格加这一对，并在 3.2 的例子里写上 | 表格加上这一对（`"a\u0345\u0300"` 与 `"\u00e0\u0345"`）。反向对照：TK1，`TitleKey("àͅ") = "aὶ"`，测试失败 |
| T7 | Minor | PG13 的"清理没有出错"可能偶发失败，D-g 的说法与代码不符。`pg13-notebook-deletion-pages.spec.ts:57` 在 `deletedDaysAgo` **之前**取 `river_job` 的最大 id，`:72-76` 检查其后插入的清理任务都没有 `errors`。e2e 的 `purge_interval` 是 2 秒（`server/configs/config.test.yaml:23`），一次清理是十几条各自提交的语句，与 `deletedDaysAgo` 的十一条语句可以交错：清理先走过 `nodes`（那时节点还没推后），推后的语句随后把节点、变更集……笔记本依次推到保留期之前，清理再走到 `notebooks` 时，笔记本已经过期而节点还在，`nodes_notebook_id_fkey` 的 RESTRICT 让这次运行失败、记下错误、由 River 重试。若这次运行的任务是在取标记之后插入的，故事就失败。`purge.ts:6-11` 的注释说"叶到根，所以清理永远碰不到子行还没推后的父行"，这只对"清理整个落在两条语句之间"成立，清理本身跨越多条语句时不成立（换成一个事务也一样：清理可以在提交前走过 `nodes`、提交后走到 `notebooks`）。重复 20 次没有复现，属于低概率的竞态。建议：标记改在 `deletedDaysAgo` 之后取（之后插入的任务都从一致的状态开始），与 D-g 的说法一致；`purge.ts` 的注释改为只保证"不让清理在两条语句之间看到半推后的树" | 标记改在 `deletedDaysAgo` 之后取：之后入队的清理都从推后完成的状态开始；故事的注释写明推后进行中的清理可能失败、由 River 重试。`purge.ts` 的注释改为只保证两条语句之间的清理看不到半推后的树，跨越推后的清理仍可能失败。竞态无法稳定复现，不做反向对照；页面的五个故事照常通过 |
| T8 | Minor | D-f 的重读没有测试。改名用例不重读、直接答 `u.Rename` 的结果（RENAME-NOREREAD，`app/rename_node.go:40-45`），新建不重读节点、用单元返回的节点拼答复（CREATE-NOREREAD，`app/create_page.go:39`），全部测试通过。`extension_test.go:271-273` 的注释写着"The answer is the page as the unit left it"，但 `TestAParticipantAddsToTheUnit` 的参与者改的是另一页 Notes，测试也不看答复的内容。建议：加一个参与者改名被操作的那一页的用例（新建 New，参与者把它改成 Journal），断言 201 的 `name` 是 Journal；改名同理 | `write_test.go` 加 `TestTheAnswerIsThePageAsTheUnitLeftIt`：参与者把被操作的那一页改名（假参与者的 `retitle`），新建与改名的答复都是改名之后的名称。反向对照：CREATE-NOREREAD、RENAME-NOREREAD 都让它失败 |
| T9 | Nit | 层级的 409 与 `after_id` 的 422 的先后没有测试：把层级检查挪到 `slotOf` 之前（DEPTH-BEFORE-422，`unit.go:185-194`）全部通过；`TestCreatePageAnswersItsCodesInOrder`（`write_test.go:154`）只覆盖了 `page.title_taken` 与 `after_id` 的先后。建议加一格"第十层的父页 + 不存在的 `after_id`"答 422 `after_id` | 码的次序表加"the values before a page too deep"：第十层的父页下、`after_id` 不存在，答 422 `after_id`。反向对照：DEPTH-BEFORE-422 让它失败 |
| T10 | Nit | 父页是附件时答 422 没有测试：去掉 `lineOf` 里的 `parent.Kind != domain.KindPage`（PARENT-ASSET，`unit.go:274`）全部通过。M4 没有附件，M7 加附件时这条才有意义；可以现在用假仓储补一格，或在 M7 的移交里写上 | 现在用假仓储补一格"a parent that is no page"：父节点是附件，答 422 `parent_id`。反向对照：PARENT-ASSET 让它失败 |
| N1 | Nit | `isPureLibraryDependency`（`archtest/rules_test.go:137`）的 `isPureLibrary(path) ||` 是多余的：三个纯库都以 `golang.org/x/text/` 开头 | 去掉多余的 `isPureLibrary(path) ||`，注释写明纯库与它们带来的包都在 `golang.org/x/text/` 下；archtest 通过 |
| N2 | Nit | 正文的字节数由两处规则给出：`recordRevision` 按 `len(r.Content)` 算（`unit.go:384`），`CreatePage` 的 `Content` 不填 `ByteSize`（`unit.go:206`），空正文时碰巧是 0。P4 写正文时，若照新建的写法漏填，就是数据库的 `page_contents_byte_size_check` 答 500。建议统一由单元按内容计算（与哈希一起），或在 `Content` 上加一个构造函数 | 单元加 `content(nodeID, text, revision)`，与哈希一起按内容计算字节数；`recordRevision` 直接取 `Content` 的值，两处同一个来源。P4 的正文写沿用它 |
| N3 | Nit | `app/ports.go:83-84` 的注释"Item … Revision is the content's version in it"：`Item` 没有 `Revision` 字段（版本号在 `Change.Revision`），注释是旧的 | 注释改为条目只记树的状态，版本号在 `Change.Revision`、由版本行记 |
| N4 | Nit | 矩阵的种子（`permission_matrix_seeded_test.go:405-410`）只写节点与正文，没有变更集、条目与版本行，`checkPages` 的"正文的 revision 等于最新的版本"在这份数据上不成立。现在没有人在矩阵数据上调用 `checkPages`；以后有人这样复用时会莫名失败。建议在种子的注释里写明，或补上版本行 | 补上版本行：种子的每一页由 `seededPageHistory` 写一个自己的变更集、条目与版本 1（时刻与状态取节点的），gone 的删除也带上它们；树与读取一致的测试给两张额外的页同样写上，并调用 `checkPages`。反向对照：种子不写这些行，`checkPages` 报 5 页的正文不是最新版本 |
| N5 | Nit | `schema_test` 的反例缺两格：`changesets_message_check` 只有空串，没有超过 4096 字节；`page_revisions_byte_size_check` 只有"大小与正文不符"，没有超过 5 MB（`schema_test.go:392`、`:398` 附近）。同一条 CHECK 的两个分支各一个反例更稳 | `schema_test` 加"a message of 4097 bytes"与"a version over 5 MB"。反向对照：把两条 CHECK 的上限放宽一倍，两格都失败 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| D-a | `Writer.Run(ctx, spec, do func(ctx, u))` 把 ctx 交给 `do` 与各操作 | 同意：Go 不在结构体里存 ctx，显式传递也让取消与期限照常生效。但它带来一个设计里没有的失误方式：用例若把外层的 ctx（不带事务）交给 `u.CreatePage`，`postgres.DB(ctx, pool)` 会在连接池上自动提交，写入既不在单元的事务里、也不在锁下，测试里的假事务不一定看得出来。现在三处闭包都以同名参数遮住外层的 `ctx`，是对的。建议在 `apply`（或 `CreatePage`、`rename` 的开头）加一行 `if !u.w.d.Tx.InTx(ctx) { return errors.New(...) }`，让传错 ctx 当场失败 |
| D-b | 守卫与参与者的值合成一个 `app.Step` | 同意：两者要的字段相同（写入的上下文、操作的种类、这次的改动），分成两个同形的类型只多一层转换。M5、M6 若给守卫加只属于它的字段（编辑会话、解析结果），参与者也会看到，无害；真的分化时再拆 |
| D-c | `TxManager.InTx` 一行方法；page 的 `Tx` 端口是 `WithinTx` + `InTx` | 同意：`app` 不能导入平台包，这是最小的做法；notebook 模块根的 `NewNotebooks` 直接用 `postgres.InTx`，层次不同、各自合规。`TestAUnitRefusesAnOuterTransaction`（真实事务）与用例层的测试守住它（RUN-INTX 两处都失败） |
| D-d | 矩阵四行与种子挪进 S3 | 同意：`page.yaml` 一进入 `api/dist`，覆盖检查就要求它们（ROW-NOPAGES 让覆盖检查失败）；计划已预见。文档第 4 节的步骤表同步即可 |
| D-e | `Outcome.By`；`Position` 是可比较的值，零值为最后 | 同意。零值即"省略 → 最后"与契约的语义一致，`reflect.DeepEqual` 也因此能直接比较适配器交给用例的值（AFTER-SWAP 被抓到） |
| D-f | 改名与新建在单元里、操作之后重读 | 同意做法：参与者可能改了这一页，答复应是单元结束时的状态；重读在观察者之前、事务之内，也对。但它没有测试（T8） |
| D-g | 整个清理任务的测试断言没有任何清理任务带 `errors`；PG13 只看自己推后之后插入的任务 | Go 测试同意：数据在启动之前写好，没有竞态；"等到 `completed AND errors IS NULL`"在出错时只会超时，现在的写法给出错误本身，更好。PG13 不同意：代码在推后**之前**取标记，与"之后插入"的说法相反，且会偶发失败（T7） |
| D-h | PG12 的阅读者是工作区访客加显式的阅读者成员 | 同意，而且必须如此：笔记本开放为 `editor` 时，工作区成员的默认角色是编辑者，高于显式的阅读者（13.1 第 3 条"有效角色取较高的一个"），只有访客（不得默认角色）才会是阅读者。建议在 3.14 的 PG12 里写一句 |
| Q1 | `unit.go` 已有 411 行，P2 加移动、删除子树，P4 加正文写、会话 | 现在不拆。P2 开工时按操作拆成 `unit.go`（`Run`、`apply`、变更集与事件）与 `unit_create.go`、`unit_rename.go`、`unit_move.go`……，避免它变成上帝文件 |
| Q2 | 重排兄弟不记条目、不进守卫与事件（设计如此） | 同意设计。提醒 M5：事件里只有新建的那一页，别的客户端若只按事件局部更新树，会看不到兄弟的次序变了；M4 总设计第 4 节"前端的页面数据"定的是"每次答复之后重读整棵树"，M5 的推送沿用"收到事件就重读"即可，写进给 M5 的移交 |
| Q3 | 模块根测试证明观察者、参与者"经 Deps 到达每个写用例"（M4 总设计第 8 节） | 守卫经新建与改名各证明一次；观察者与参与者只经新建。两个用例共用 `module.go` 里同一个 `writer`，结构上成立，不必补；文档的措辞可改为"到达写入单元" |
| Q4 | `changesets_client_check` 的 `[:cntrl:]` 随数据库的 `LC_CTYPE` 而变，`Client.Valid` 不随 | 在快照的测试库（`datctype = C.UTF-8`）上核对了 U+0085、U+009F、U+007F（两边都拒）与 U+2028、U+200B、U+00AD（两边都收），一致。只有 M9 的 `mcp:<名>` 会碰到；部署在别的区域设置上时值得在 M9 再核对一次 |

处置：Q1 在 P2 开工时按操作拆分 `unit.go`（P2 文档）；Q2 写进 [给 M5 的移交](../../M5-collab-editing/handoffs/M4-P1-tree-refresh.md)；Q3 的措辞在 M4 总设计第 8 节改为"到达写入单元"；Q4 写进 [给 M9 的移交](../../M9-mcp/handoffs/M4-P1-client-check.md)。D-a 的加固：`apply` 先核对 ctx 带着单元的事务，`TestAnOperationRefusesAContextOutsideTheUnit` 守住（去掉核对，测试失败）。D-g 的 PG13 部分随 T7 改正。D-h 写进 3.14。

## 文档与代码的不一致

作者已知的偏差（D-a…D-h）之外：

- D1：3.1 的文件清单：没有 `adapter/postgres/nodes.go`（节点的方法在 `store.go`）；漏列 `queries/contents.sql`、`domain/client.go`、`archtest/rules_cases_test.go`、`notebook/notebooks_test.go`、`notebook/adapter/postgres/store.go`（`ShareNotebook`）与它的测试注释、`platform/postgres/tx.go` 与 `tx_test.go`（D-c）、`bootstrap/permission_matrix_test.go`（`matrixRows`）、`e2e/stories/notebook/n13-workspace-deletion.spec.ts`（改用 `purge.ts`）。
- D2：3.3 与 S1 计划说要改 `notebook/app/extension.go` 里"M4 的页面写只锁笔记本行"的注释；基线的 `extension.go` 里没有这句（只有 `notebooks.sql` 与它的生成代码、`store_test.go` 有），代码也没改它。删掉文档里的这个文件名即可。
- D3：3.4 没写 `nodes_sort_order_check`（排除 NaN 与 ±Infinity），代码与 `schema_test` 都有。
- D4：3.5 的 `Change{NodeID, Kind, Before, After, Revision}` 与 `Kind` 的五个取值：代码的 `Change` 没有 `Kind`，操作的种类是 `domain.Operation`（`create`、`rename`），放在 `app.Step` 上；`TreeState` 在 `change.go`（S2 计划写在 `node.go`），`MaxDepth` 在 `node.go`（3.5 写在 `tree.go`）；"领域只校验链的完整"实际在仓储（`Store.Ancestors`）。
- D5：3.6 的 `Run` 签名（D-a）、守卫与参与者的值（D-b）、`postgres.InTx`（D-c）；S3 计划第 2 项把 `Client`、`Options` 写在 `unit.go`，代码在 `domain/client.go` 与 `app/extension.go`。
- D6：3.10 写"等任务完成的查询加上 `errors IS NULL`"，代码是另一条查询（D-g）；D-g 说 PG13 看"推后之后插入的"任务，代码看的是推后之前取标记之后的（T7）。
- D7：3.4 第 4 项、3.13 的交错表与 M4 总设计第 9 节说交错 32"碰不到唯一索引、第二个在锁下看得到第一个"，测试分不出这一点（T1）。
- D8：3.14 的 fixture：只返回答复的两个叫 `postPage`、`getTree`；`renameNode`、`getPage` 只有返回答复的一种。PG12 的阅读者是访客（D-h），可以写进 3.14。
- D9：S2 计划的反向对照"复合外键改为普通的自引用（schema 名单失败）"：约束名不变时 `TestConstraintAndIndexNames` 照样通过（FK-PLAIN），抓到它的是 `TestAParentInAnotherNotebook`。计划改为后者，或让名单测试也比较外键的列。
- D10：第 4 节的步骤表（S1、S3 各两个提交；矩阵与种子在 S3，D-d）；合并时要更新 P1 第 7 节"结果"、文档头的状态、M4 总设计第 7 节移交表（M2/P4 第 1、2、4 项，M3/P1 第 1、3 项）与第 12 节进度表。
- D11：收尾时补进 13.1 的：第 11 条的窄端口例子加 `notebook.NewNotebooks`；第 21 条的组合改为 `notebookRegistrants(pool)`、`pageRegistrants()`，注册者例子加 `page.NewNotebookDeletion`；第 6 条"有自引用外键的表另写测试"可以点名 `TestPurgeSkipsAHeldNodeAndKeepsItsAncestors`；第 21 条的"两个注册者的测试"在 page 模块落实之后（T3）再引用。

处置：D1–D9 在合并之后的文档提交里同步进 P1 文档与 Step 计划（3.1 的文件清单、3.3、3.4 的 `nodes_sort_order_check`、3.5 的 `Change` 与 `Operation`、3.6 的 D-a…D-c、3.10 与 PG13 的标记、交错 32 的说法、3.14 的 fixture 与 PG12 的访客、S2 计划的 FK-PLAIN 改为 `TestAParentInAnotherNotebook`）；D10 的第 4 节、第 7 节、文档头、M4 总设计第 7 节与第 12 节一起更新；D11 记进 M4 总设计的收尾清单，收尾时写进总体设计 13.1。

核对过、与实现一致的：
- 写入单元：`Run`（`unit.go:78-125`）先拒已有事务与不合法的客户端，事务之外 `WorkspaceOf`，事务里工作区行 `FOR SHARE` → 笔记本行（`Tree` 时 `LockByID`，两个锁语句都带 `deleted_at IS NULL`，读到 0 行答 `spec.NotFound`）→ `Authorize`（`ErrNotVisible` 换成 `spec.NotFound`）→ 读一次时钟 → `do` → 观察者一次 → 提交。`CreatePage` 依次是标题 422、父页 422、`after_id` 422、重名 409、层级 409，然后 `apply`：守卫 → 变更集（第一次写入时）→ 重排、节点、正文、版本 → 条目 → 参与者；参与者经 `appender` 追加的改名走同一个 `apply`、经守卫、不再调用参与者；单元里的改名只在本笔记本里找节点（`FindNodeIn`）。没有改动（改成同名）时不插变更集、不调用观察者、不写日志。
- 码与 404：`listNodes`、`createPage` 答 `notebook.not_found`，`getPage`、`renameNode` 答 `page.not_found`；读不开事务、先读后判定；`getPage` 对附件答 404；父页、`after_id` 在别的笔记本时一律 422，不泄露存在与否。
- 标题键：`TitleKey = NFC(fold(NFC(s)))`，每次新建 `Caser`；唯一索引 `(notebook_id, parent_id, name_key) NULLS NOT DISTINCT WHERE deleted_at IS NULL` 与 23505 的翻译（`TestSiblingNamesAreUnique` 含根下）；只差大小写的改名不与自己比较、照常写（PG2 也验证了 `Straße`→`STRASSE`、NFD 的 `Café`）。
- 次序：`Place` 的四种位置与重排（重排保持兄弟的相对次序、跳过新节点的槽位，`TestPlaceKeepsTheOrderStrict` 200 次严格递增），`Children` 按 `(sort_order, id)` 排，与 `PreOrder` 的平局规则一致。
- 清理：`PurgeNodes` 每条语句的 `LIMIT` 是这一批剩下的数，删不出或凑满即停，ctx 里没有事务、每条语句各自提交；"删叶子"的 `NOT EXISTS` 看任何子节点（不论是否删除）与三种跟随的行，`FOR UPDATE SKIP LOCKED`；变更集只删已没有条目与版本的；顺序 `changeset_items → page_revisions → page_contents → nodes → changesets → notebook → workspace` 与外键一致（PURGE-ORDER、PURGE-ORDER-CS 都让顺序检查与整个清理任务失败）；整个清理任务的种子含 `live` 里 61 天前删除的子树，无错误的检查有效（PURGE-NOLOOP 让它失败）。
- 笔记本删除的注册者：一条语句以事件的时刻、事件的执行者软删除未删的节点及其未删的正文、版本、条目与这些笔记本未删的变更集，回收站里的行保留原时刻；三条发布路径（删除笔记本、删除工作区、删除无主笔记本）都由整个程序的测试守住，组合根任一处交空，对应的测试失败（REG-NB、REG-WS）；`pageNotebookDeletion` 逐字段转换；注册者只凭连接池构造。
- 契约与适配器：`parent_id` 必填可空（`bodyshape` 的 `Required`，缺省答 400），答复里根页的 `parent_id` 用 `NewNullNullable` 显式写 `null`（PARENT-ZERO 被抓到）；`after_id` 省略为最后、`null` 为最前；客户端按凭证（PAT 是 `api`，会话的访问令牌是 `web`），由适配器测试与模块根测试各守一次；每个声明的码各答出一次；id 不是 uuid 时 400（与 notebook 的 `TestANotebookIDThatIsNoUUID` 先例一致）；中英文案三个码。
- 权限：四行规则、`writers()`；矩阵四行的格子与种子（priv 的子页核对祖先链、gone-nb 的页面随笔记本删除、orphan 的成员可写）；树与逐项读取一致的测试含子页与已删页。
- `checkPages`：五条查询都对；层级那条在 `depth >= 10 AND parent_id IS NOT NULL` 时正好表示深于十层或成环。交错 30–33 的断言与两种先后都对（30、31、33 的反向对照 DECIDE-FIRST、NO-WS-LOCK、REG-WS 都让它们失败；32 见 T1）。
- e2e：`expectNewPage` 核对节点、空正文、版本 1、变更集的种类、客户端、执行者与时刻、条目与版本；`expectPagesDeletedWith` 核对五张表的时刻都等于笔记本的；`purge.ts` 从叶到根推后，n13 改用它。
- 加锁顺序（13.1 第 5 条）：页面写的第一条语句是工作区行 `FOR SHARE`，然后笔记本行，再碰节点与跟随的行；外键检查只对账户、笔记本、父页取 `FOR KEY SHARE`，与 `FOR NO KEY UPDATE` 不冲突；删除笔记本与删除工作区的注册者已持笔记本行的独占锁；注册者的 `UPDATE` 只匹配未删的行，清理只锁已删的行（`SKIP LOCKED`），两者不互等；没有找到会成环的路径。重排的 `UPDATE` 只在笔记本行的独占锁下改兄弟，P4 的正文写对节点只取外键检查的 `FOR KEY SHARE`，不冲突。

## 反向对照

做法：在快照里改代码（生成的 SQL 直接改 `gen/*.sql.go` 里的常量），只跑相关的包或测试（`go test -count=1 -run …`），再 `git checkout` 复原；e2e 的服务端变异重新 `go build` 二进制。跑的过程中 Docker 两次启动容器超时，受影响的两项（GUARD-NOSTOP、页面故事的一次）已重跑，下面是重跑的结果。

会失败的（54 项 Go、2 项 e2e）：

- 标题与端口：TK2 去掉第二次 NFC、TKF 用 `cases.Lower` 代替完整折叠（`TestTitleKeysMatchAcrossCaseAndNormalization`）；SHARE-TX `ShareByID` 不检查事务、LOCK-SHARE `LockByID` 改用 `FOR SHARE`（`TestNotebooksFindAndLock`）。
- 写入单元：GUARD-FIRST 守卫挪到检查之前（码的次序、模块根的守卫测试）；OBS-PER-OP 观察者每个操作一次（两操作单元、参与者测试）；PART-NOGUARD 参与者追加的操作不经守卫、PART-RECURSE 参与者递归（`TestAParticipantAddsToTheUnit`）；ITEM-ALWAYS 条目在参与者之后才记；DECIDE-FIRST 判定挪到加锁之前（用例层的调用次序，交错 33"移出在先"）；NO-WS-LOCK 不锁工作区行（用例层，交错 31、33 共四个子测试）；TREE-SHARE 树锁改 `FOR SHARE`（只有用例层的调用次序失败，见 T1）；RUN-INTX 不拒已有事务（用例层与模块根）；CLOCK-TWICE 变更集再读一次时钟（`TestCreatePageWritesThePageAndItsChangeset`）；TELL-EMPTY 没有改动也通知观察者；RENAME-NOOP、RENAME-SELF、TITLE-BY-NAME（改成同名照写、只差大小写与自己冲突、按名称而不是键比较）；DEPTH-OFF1 层级差一；TAKEN-BEFORE-422 重名挪到 `after_id` 之前；UNIT-FINDNODE 单元里的改名不限本笔记本（用例层的调用次序）；RENUMBER-OFF 不重排、RENUMBER-NOWRITE 重排算出不写回。
- 数据与仓储：NND 去掉 `NULLS NOT DISTINCT`（名单与 `TestSiblingNamesAreUnique`）；FK-PLAIN 复合外键改为普通自引用、名字不变（只有 `TestAParentInAnotherNotebook`，见 D9）；CHILDREN-DELETED 兄弟带上已删的；ITEM-BEFORE 合并时也改写 before；ANCESTORS-NOCHECK 不核对祖先链到根；PREORDER-NOTIE 先序不按 id 断开平局；PARENT-ZERO 根页的 `parent_id` 省略。
- 清理：PURGE-NOLOOP 不循环（三层树、两个持有的测试与整个清理任务）；PURGE-NOCHILD、PURGE-NOCONTENT、PURGE-NOITEM（"删叶子"不看子节点、正文、条目，都等到 5 秒期限）；PURGE-NOSKIP、PURGE-CONTENT-NOSKIP、PURGE-ITEM-NOSKIP（去掉 `SKIP LOCKED`）；PURGE-CS-NOITEM 变更集不看条目；PURGE-ORDER、PURGE-ORDER-CS 清理器顺序颠倒（顺序检查与整个清理任务）。
- 笔记本删除：DEL-TRASH 也改已删的节点（`TestDeleteNotebooksPages`；模块根的测试没有回收站里的页，照样通过）；DEL-NOCS 不删变更集、DEL-NOREV 不删版本（仓储、模块根、三条路径）；REG-NB 笔记本模块的用例拿到空订阅者（删除笔记本、删除无主两条路径与交错 30"写入在先"）；REG-WS 工作区删除拿到空订阅者（工作区路径与交错 31"新建在先"）。
- 适配器与权限：CLIENT-WEB 客户端写死 `web`（适配器、模块根）；AFTER-SWAP 省略与 `null` 对调；RULE-CREATE-READERS、WRITERS-READER（矩阵的阅读者格）；ROW-NOPAGES `workspaceOfRow` 不认页面（覆盖检查）；GETPAGE-NF、CREATE-NF 404 的码换错（用例层与矩阵）；CHECK-PARENT、CHECK-REV `checkPages` 的查询写错（注册者测试、交错）。
- e2e：`expectNewPage` 的客户端写死 `web`，PG1 失败（答复是 `api`）；`purge.ts` 不推页面的表，PG13 等工作区被清掉超时（15 秒）。

改了却没有失败的（18 项 Go，其中 2 项在不变量之下等价；2 项另在 e2e 上复核）：

- 对应发现：PURGE-NOREV、PURGE-CS-NOREV、PURGE-REV-NOSKIP、PURGE-CS-NOSKIP（T2）；GUARD-ORDER、GUARD-NOSTOP、OBS-ORDER、PART-ORDER（T3）；ITEM-NOPARENT、ITEM-NOSORT（T4，ITEM-NOPARENT 在 e2e 上也存活）；RENAME-SQL-AUDIT（T5，e2e 上也存活）；TK1（T6）；RENAME-NOREREAD、CREATE-NOREREAD（T8）；DEPTH-BEFORE-422（T9）；PARENT-ASSET（T10）。
- 另：TREE-SHARE 计入"会失败的"（用例层失败），但整个程序上的交错 30–33 全部通过（T1）。
- 等价、不作为发现：CONTENT-META-DELETED（`ContentMeta` 读到已删的正文行：未删的页面只有未删的正文，`checkPages` 守着这条不变量）；CHILDREN-NOID（`Children` 只按 `sort_order` 排：经接口写出的兄弟次序互不相同，平局只来自绕过 `Place` 的数据；`PreOrder` 的平局规则另有测试）。
