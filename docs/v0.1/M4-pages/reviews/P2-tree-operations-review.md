# M4/P2 树操作：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m4-p2-tree-operations`（`main...2d4bdb3`：S1–S4 共 5 个提交，S1 两个——`d7cce0a` 拆分 `unit.go`、`ec0b284` 领域与仓储，S2 `1b62802`、S3 `2f8e5e8`、S4 `2d4bdb3` 各一个；48 个文件，+2399/−224。审查请求里写的"6 个提交"与 `git rev-list --count main..2d4bdb3` 不符，以这里为准），对照 [02-P2-tree-operations.md](../02-P2-tree-operations.md) 第 3–5 节，四份 Step 计划，[M4 总设计](../00-M4-design.md)第 4、5、8、9 节，[P1](../01-P1-page-module-pipeline.md) 3.6、3.10、3.13，总体设计 13.1（第 5、6、10、21 条） |
| 审查方式 | 独立审查者在 `git archive 2d4bdb3` 的快照上实测（快照里 `git init` 过），仓库与别的 Docker 容器都没动过：<br>• 门禁：`GOFLAGS=-p=3 go test -race -count=1 ./...`，48 个包全部 ok；`make lint`（Go 两处 0 issues，`go mod tidy -diff` 干净，md 样例集自检、oxlint、oxfmt、三个包的 `tsc` 通过）；`make knip` 通过；`make gen-check` 退出 0、无差异；vitest 63 个文件 1003 个测试通过；`server/tools` 的测试、`TestCheckCostsAboutTheBody` 通过；`make e2e`（含 `make build`）124 个全部通过；<br>• 重复：交错 34–37 与树与读取一致 `-race -count=5 -v`，25 次顶层、55 次子测试 PASS、0 失败；page 的各包 `-race -count=3` 全部 ok；页面的七个故事 `--repeat-each 3`，21 次全部通过；<br>• 中间提交：`d7cce0a`、`ec0b284`、`1b62802`、`2f8e5e8` 各自 `go vet ./...`、`go test -short ./...` 与 page 模块的全部测试（含真实数据库），全部通过；拆分提交 `d7cce0a` 逐行核对：从 `unit.go` 删掉的 176 行与三个新文件加的行排序后相同，只多出包头、import 与文件注释；<br>• 反向对照：Go 45 项（34 项失败、11 项存活，其中 2 项在不变量之下等价），e2e 4 项（3 项失败、1 项存活），汇总见下；另在快照里量了删除子树的耗时（Q3）。没有跑 `make image-smoke` |
| 日期 | 2026-10-02 |
| 结论 | 可以合并，合并前补上 T1–T5 的测试；T6–T8、N1、N2 可随手处理。<br>• 处置：5 项 Minor（T1–T5）与 5 项 Nit（T6–T8、N1、N2）都在合并前处理（修复 `2586b2c`，`7d0e3b1` 合并），每个新检查都做了反向对照，见"发现与处置"；D-a…D-h 与审查者的判断一致，D-h 与 Q1 写进给 M9（抄送 M7）与 M5 的移交；Q2 改 P2 文档 3.2 的措辞；Q3 记进 P2 文档的风险；Q4 是审查请求的笔误（5 个提交）；文档的 9 处不一致在合并之后的文档提交里同步。<br>• 没有发现生产代码的缺陷；没有 Critical、Major。5 项 Minor 都是测试守不住的行为（反向对照存活）：子树的递归只测到三层（T1）、层级只在"链"上测（T2）、同父页内的重排（T3）、原地不动只在根下（T4）、锁下找不到节点（T5）。3 项 Nit 是测试（T6–T8），2 项 Nit 是代码与注释（N1、N2）。<br>• 写入单元的两个操作与设计一致：`Move` 依次是锁下读节点（404）→ 新父页（422 `parent_id`）→ 去掉自己的兄弟与 `after_id`（422 `after_id`，`after_id` 是自己也是 422）→ 原地不动（不写、不插变更集、不调观察者、不记日志）→ 环（新父页在 `Subtree` 里，含自己）→ 重名（只在换父页时）→ 层级（新父页的层级 + 高度 − 1）→ 守卫 → 重排去掉自己的兄弟 → `MoveNode` → 只有被移动的页记条目，后代以前后相同进守卫与事件 → 参与者；`Delete` 锁下读子树（没有即 404）→ 守卫（每个节点，无后状态）→ `DeleteNodes` 同一时刻 → 每个节点一条条目、随节点进回收站 → 参与者，兄弟不重排，变更集留在活着的笔记本里。<br>• 仓储的三条 SQL 只改未删的行、走 `(notebook_id, parent_id)` 与 `node_id` 的索引，23505 译为 `page.title_taken`；与清理器配合：活着的笔记本里删掉的子树过了保留期一次清完（`TestPurgeTakesADeletedSubtree`），条目不进回收站时清不掉（ITEM-NODELETEDAT 让它失败）。<br>• 加锁：两个用例都是树单元（笔记本行 `FOR NO KEY UPDATE`）；交错 34–37 持笔记本行 `FOR SHARE`，TREE-SHARE 让四个交错都在等锁的期限失败；我另把子树改成在锁外读（STALE-SUBTREE），34、35、36 的断言本身也抓得到（成环、超过十层、子页留在已删的父页下），交错的断言有力。<br>• D-a…D-h 都同意；D-h 建议从代码注释升格为给 M9（与 M7）的移交 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Minor | `Subtree` 的递归只在三层以内有测试。把 `queries/nodes.sql:64`（生成代码里的常量）的界 `s.level < 64` 改成 `s.level < 3`（SUBTREE-BOUND3），仓储、模块根、交错 34–37、树与读取一致，以及重新构建后的 PG3、PG4、PG12 全部通过：`TestSubtree`（`adapter/postgres/store_test.go:331`）的树只有三层（R、A、A1），其余经真实数据库的移动与删除，子树都不超过三层（交错 35 的 Top 是两到三层，PG3 的 A、PG4 的 Doomed 都是三层），层级的表格测试走假仓储。`MaxDepth` 是 10，子树最高可到 10 层。失败场景：递归的界或连接条件回归之后只返回前几层——删除第四层以下还有页的子树时，深处的页留在已删的父页下：树里看不到（`PreOrder` 丢掉父页不在的节点），节点与正文仍是活的，清理器的"删叶子"看任何子节点，已删的祖先因此永远清不掉；移到曾孙页之下不被判为环；高度少算，移动之后超过十层。建议：`TestSubtree` 加一条十层的链（`MaxDepth`），断言最后一页的层级是 10、`Height()` 是 10；或在仓储测试里删一棵四层以上的子树，核对每一页都删了 | 仓储测试加 `TestSubtreeReachesTheDeepestLevel`：一条 `MaxDepth` 层的链，`Subtree` 读到最后一页、高度 10。反向对照：SUBTREE-BOUND3 让它失败 |
| T2 | Minor | 层级的检查只在"链"上测过，高度与节点数分不开。把 `app/unit_move.go:58` 的 `sub.Height()` 换成 `len(sub)`（MOVE-HEIGHT-LEN），用例层、模块根与交错 34–37 全部通过：`TestMoveNodeRefusesASubtreeTooDeep`（`move_test.go:168`）的 Top→Middle→Bottom、交错 35 的 Top→Bottom→Deep、PG3 的 A→A1→A2 都是一条链，节点数正好等于高度。领域的 `TestSubtree` 测了 `Height()` 本身（"三层、深的分支在后"），但没有测单元用的是它。失败场景：把"子树的节点数"当成高度，一页带十个子页（高 2、11 个节点）的任何移动——连同一父页内的排序、移到根下——都答 409 `page.too_deep`。建议：`TestMoveNodeRefusesASubtreeTooDeep` 的子树加一个兄弟（Top 下两个子页、一个孙页：高 3、4 个节点），断言它仍能移到第七层之下 | `TestMoveNodeRefusesASubtreeTooDeep` 的子树加一个兄弟（Top 下两个子页、一个孙页：高 3、4 页），照旧能移到第七层之下、第八层之下答 409。反向对照：MOVE-HEIGHT-LEN 让它失败 |
| T3 | Minor | 同父页内移动时的重排没有覆盖"被移动的页在插入点之前"。把 `unit_move.go:77` 的 `others[i].ID` 换成 `siblings[i].ID`（MOVE-RENUMBER-SIBLINGS），用例层、模块根与交错 34–37 全部通过：唯一触发重排的用例 "between two with no gap"（`move_test.go:82`）移动的 C 排在最后，`siblings` 与 `others` 的前两项相同。失败场景：兄弟 [X, A, B]，A 与 B 之间没有间隔，把 X 移到 A 之后：正确的写法给 A 0、B 2、X 1；写错下标时 0 写给了 X（随后被 `MoveNode` 覆盖）、2 写给了 A、B 不动，结果 X 排在 A 之前，次序错了，而且只有间隔用尽时才出现，很难在别处发现。建议：位置的表格加一格"从最前移到没有间隔的两页之间"（A、B、C，B 与 C 之间没有间隔，把 A 移到 B 之后，期望 B、A、C） | 位置的表格改在一个父页之下，加"from the first place between two with no gap"（A、B、C，B 与 C 之间没有间隔，把 A 移到 B 之后，期望 B、A、C）。同时按 N1 把取次序与重排收进 `placeAmong`，下标只有一处。反向对照：把 `placeAmong(others, …)` 换成 `siblings`（与 MOVE-RENUMBER-SIBLINGS 同类），两格失败；`placeAmong` 不写回重排，新建与移动的位置表都失败 |
| T4 | Minor | 原地不动与位置的表格只在根下。把 `unit_move.go:40` 的 `domain.SameParent(n.ParentID, parentID)` 换成指针比较 `n.ParentID == parentID`（MOVE-SAMEPARENT-PTR），用例层、模块根与交错全部通过：`TestMoveNodeToWhereItIs`（`move_test.go:138`）与 `TestMoveNodePlacesItAmongItsSiblings` 的页都在根下（两边都是 `nil`），领域的 `TestChangeMoves` 里前后两个状态共用同一个 `&p`，也比较不出指针与取值。失败场景：在某页之下"移到原处"（拖拽放回原位、键盘对话框选了原来的位置）不再被认出：重新取次序、写节点（审计列变了）、插变更集、通知观察者、记日志（次序值变了时另有一条条目），与设计"不写"相反。建议：原地不动的表加一格"在某页之下、排在它原来跟着的那页之后"（父页 id 由请求另行构造，不与节点共用指针）；位置的表加一格"同一父页之下排到最前" | 原地不动与位置的表格都在一个父页之下，请求里的父页 id 每次另行构造、不与节点共用指针；位置表加"in from another parent"。反向对照：MOVE-SAMEPARENT-PTR 让原地不动的三格都失败 |
| T5 | Minor | 锁下找不到节点的两条路径没有测试：移动、删除在等锁期间被删的页。把 `unit_move.go:24` 与 `unit_delete.go:20` 的 `found(err, domain.ErrNotFound)` 改成直接返回 `err`（MOVE-NF-RAW、DELETE-NF-RAW），用例层、HTTP 适配器、模块根与交错 34–37 全部通过：两个用例的码的次序表（`move_test.go:189`、`delete_test.go:69`）里的"404"都发生在锁外的 `FindNode` 或笔记本的锁上；交错 36 只测了删除与新建、改名，没有删除与删除、删除与移动。失败场景：两个人同时删除同一页（或一人移动、另一人删除它的祖先），第二个在锁下读不到节点，仓储的 `app.ErrNotFound` 不再换成 `page.not_found`，平台把它答成 500。现在的代码是对的，测试守不住。建议：两张码的次序表各加一格"a node deleted while it waited"（假仓储在 `LockNotebook` 时删掉节点）；或在交错 36 加"删除与删除""删除在先、移动答 404" | 两张码的次序表各加"a node deleted while it waited"：假的笔记本端口在锁上调用 `waited`，删掉节点，答 `page.not_found`、什么都不写。反向对照：MOVE-NF-RAW、DELETE-NF-RAW 各让一格失败 |
| T6 | Nit | 码的次序缺两格：环先于层级、重名先于层级。层级的检查挪到环之前（MOVE-DEPTH-FIRST）或重名之前（MOVE-DEPTH-BEFORE-TITLE），用例层、模块根与交错 34–37 全部通过；表里"under its grandchild, before the title"只区分了环与重名（同 P1 审查 T9）。建议加两格：一棵高 6 的子树移到它第六层的后代之下（既是环、又超过十层，答 `page.cycle`）；把子树移到第八层、那里已有同名的页（既重名、又超过十层，答 `page.title_taken`） | 码的次序加两格：移到它第六层的后代之下（既是环、又超过十层，答 `page.cycle`）；移到第八层、那里已有同名的页（既重名、又超过十层，答 `page.title_taken`）。反向对照：MOVE-DEPTH-FIRST 让两格都失败，MOVE-DEPTH-BEFORE-TITLE 让后一格失败 |
| T7 | Nit | 删除之后调用参与者没有测试：`unit_delete.go:28` 的 `apply(…, true, …)` 改成 `false`（DELETE-NOPARTICIPATE），用例层与模块根全部通过（移动有 `TestTheAnswerIsThePageAsTheUnitLeftIt` 守着，MOVE-NOPARTICIPATE 失败）。M4 总设计第 4 节"参与者在每个操作之后被调用"，M6 的参与者是否要看删除另说，但现在的行为应当钉住。建议：`TestDeleteNodeDeletesTheSubtreeUnderTheLock` 登记一个参与者，核对它看到 `OpDelete` 的那一步 | `TestDeleteNodeDeletesTheSubtreeUnderTheLock` 登记一个参与者，核对它看到 `OpDelete` 的一步、子树的每个节点。反向对照：DELETE-NOPARTICIPATE 让它失败 |
| T8 | Nit | 整个程序的不变量不看"已删节点的跟随行"。`DeleteNodes` 不删正文、版本或旧条目（DELNODES-NOCONTENTS、DELNODES-NOREV、DELNODES-NOITEMS），`RecordItem` 不给删除的条目写 `deleted_at`（ITEM-NODELETEDAT），交错 34–37 与树与读取一致都通过（ITEM-NODELETEDAT 另在整个清理任务与 `TestDeletingANotebookDeletesItsPages` 上通过，前三项另在模块根上通过）；抓到它们的只有仓储测试、PG4，以及模块根对删除自己的条目的那一条断言（只抓 ITEM-NODELETEDAT）。`checkPages`（`bootstrap/interleavings_page_test.go:26`）只查未删的页面。"跟随的行随节点进回收站"正是清理器能清完的前提（M4 总设计第 4 节的生命周期表）。建议：`checkPages` 加一条"节点已删、跟随它的正文、版本或条目未删"的计数为 0，交错 36 的"写在先"（新写的页、正文、版本、条目随子树）与可视性测试随之守住它 | `checkPages` 加"节点已删、跟随它的正文、版本或条目未删"的计数为 0。反向对照：`DeleteNodes` 不删正文（DELNODES-NOCONTENTS）、删除的条目不写 `deleted_at`（ITEM-NODELETEDAT），交错 36 的四个子测试都失败 |
| N1 | Nit | 放进父页的取值与重排写了两遍：`unit_create.go:47-64` 与 `unit_move.go:61-79` 都是"取兄弟的次序 → `domain.Place` → 在写里逐个 `SetSortOrder`"，连注释"Renumbering keeps the siblings' order"都相同；T3 的那种下标错正是两份抄写容易出的。`unit_place.go` 本来就是"新建、改名、移动共用的放置"。建议在 `unit_place.go` 加一个小函数（例如由兄弟与 `after` 给出次序值与一个写重排的闭包），两处共用 | `unit_place.go` 加 `placeAmong(siblings, after)`：给出次序值与写重排的闭包，新建与移动共用（见 T3） |
| N2 | Nit | `app/ports.go:117` 的 `Nodes.Subtree` 注释没写"没有、已删或不在这个笔记本时答 `ErrNotFound`"；`Unit.Delete` 正靠它答 404（`found(err, …)`），假仓储与真仓储都这样实现，同文件的 `FindNodeIn` 写明了。建议补上一句 | `Nodes.Subtree` 的注释补上"没有、已删或不在这个笔记本时答 `ErrNotFound`" |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| D-a | 移动的位置与原地不动的判断在用例层 `app/unit_move.go`，不在领域；领域只加 `Subtree`（`Height`、`Holds`）与导出的 `SameParent` | 同意：`Position`、`slotOf` 在 P1 已在 `app`，领域没有"位置"的类型；把判断挪进领域要连带 `Position` 一起挪，只为了让一张表不经假仓储，收益小。用例层的表格经假仓储同样是表格驱动、跑得快。但这两张表都只在根下（T4），重排没有覆盖被移动的页在插入点之前（T3）。文档 3.1、3.2 同步 |
| D-b | `Subtree` 按层排（level, sort_order, id），不是先序 | 同意：唯一依赖次序的是"节点本身在第一个"（`Move` 的 `sub[1:]`），按层排保证这一点且是确定的；`Height`、`Holds`、删除的 id 集合都与次序无关；假仓储同序。SUBTREE-ORDER-DESC 由 `TestSubtree` 抓到。3.3 的"先序"改为"按层" |
| D-c | 删除的条目进回收站由仓储决定：`RecordItem` 见到没有"后"就写 `deleted_at = it.At`，插入与合并都写 | 同意：生命周期表说条目随节点，"后状态为空"就是"这条条目删除了它的节点"，用例层再传一个标记只是冗余。合并时的 `deleted_at = excluded.deleted_at` 在现在的流程里与 `DeleteNodes` 的 CTE 重复（`apply` 先写后记条目，同一变更集里先前的条目已被 CTE 放进回收站；ITEM-MERGE-NODELETEDAT 只让仓储测试失败），无害，留着也让仓储单独成立。提醒：一个有"后"的改动合并进已在回收站的条目时会把 `deleted_at` 清空；M4 没有这条路径（节点删了就再碰不到），M8 的恢复若与删除在同一变更集里要重新看 |
| D-d | 用例的请求值 `app.Destination{ParentID, Position}`；`Unit.Move(ctx, id, parentID, Position)`；`Unit.Delete` 返回删掉的子树，日志记 `nodes=<数目>` | 同意：`Destination` 与 `PageDraft` 同形；`Delete` 返回子树只供日志计数，不暴露多余的东西；`Move` 返回节点与 `Rename` 一致（用例照旧在单元里重读作答，MOVE-NOREREAD 一类的回归由 `TestTheAnswerIsThePageAsTheUnitLeftIt` 的移动一段守住） |
| D-e | `lineOf`、`slotOf`、`titleFree` 与 `Position` 一起在 `unit_place.go` | 同意，文档 3.1 已与代码一致。拆分提交只搬了代码（见审查方式）。建议顺手把 N1 的重复也收进这个文件 |
| D-f | 可视性测试里删除 team 的页由 `callerDefaultEditor` 经接口做；矩阵的移动一行把每列的页移到根下最前，已在最前的原地不动、同样答 200 | 同意：判定在 `Run` 里、原地不动之前，阅读者的格照样 403（RULE-MOVE-READERS 正是在 team、wiki 那几格失败，它们的页本来就原地不动）；priv 的两列做了真正的写。经接口删除走的是真实路径，比直接写 SQL 更好 |
| D-g | e2e fixture：`postMove`（答复）、`moveNode`（核对 200 后返回节点）、`deleteNode`（答复） | 同意：与 P1 的 `postPage`/`createPage` 同一约定；`deleteNode` 的 204 没有数据，返回答复即可。3.9 与 S4 计划同步 |
| D-h | 同一单元先建后删的合并得到前后都空的条目、被 `changeset_items_state_check` 拒绝；P2 不处理，只写在 `recordItem` 的注释里 | 同意 P2 不处理（M4 的用例都是一个操作，参与者只追加改名与正文写）。但只写在代码注释里不够：除了条目违反 CHECK（答 500），事件里也会出现一个前后都空、`Revision` 可能不为零的 `Change`，观察者与守卫都要认得它。它最先在 M9 的 batch 或 M7 的导入里出现。建议写进给 M9（抄送 M7）的移交：合并到前后都空时删掉这一行条目（或不插入），并从改动集里去掉这个节点；连同 `domain.Change.Then` 的语义一起定 |
| Q1 | 守卫与事件的值随子树变大：移动带每个后代，删除带每个节点 | 设计如此（M4 总设计第 8 节"移动与删除是整棵子树"）。提醒 M5：`NOTIFY` 的载荷上限是 8000 字节，若把改动集序列化进去，删除或移动一棵几十页的子树就会超出、整个单元回滚。P1 审查 Q2 已定"收到事件就重读"，M5 的推送只带笔记本与变更集 id 即可。建议写进给 M5 的移交 |
| Q2 | 同父页内的排序也把每个后代以"前后相同"放进守卫与事件 | 3.2 的理由是"位置没变，路径变了"，但同父页内的排序不改变任何后代的路径，M6 会多重算一次。无害，且让"移动是整棵子树"保持一种形状；我倾向保持代码，把 3.2 的措辞改为"换父页时后代的路径变了；同父页内排序时后代照样在内" |
| Q3 | 删除的开销 | 每个节点一条 `RecordItem`（一次往返），全在笔记本行的独占锁下。快照里实测（本机容器，Subtree + DeleteNodes + 每个节点的条目，一个事务）：100 个节点 20 ms、1000 个 163 ms、3000 个 432 ms，约每节点 0.14 ms。v0.1 可以接受；数据库在远端或笔记本更大时，条目可以改成一条 `INSERT … SELECT unnest(…)`，`ON CONFLICT` 的合并语义不变。记进风险即可 |
| Q4 | 审查请求说作者的反向对照"列在 P2 各 Step 的提交里" | 五个提交的信息里没有反向对照的清单；四份 Step 计划第"测试"一节列了。我按计划里列的逐项复核（见"反向对照"），都成立 |

处置：D-h 写进[给 M9 的移交](../../M9-mcp/handoffs/M4-P2-unit-merge.md)（M7 的导入同样适用，见[给 M7 的移交](../../M7-assets-transfer/handoffs/M4-P2-unit-merge.md)）；Q1 写进[给 M5 的移交](../../M5-collab-editing/handoffs/M4-P1-tree-refresh.md)第 3 项；Q2 保持代码，P2 文档 3.2 改为"换父页时后代的路径变了；同父页内排序时后代照样在内"；Q3 记进 P2 文档第 7 节"留给后面的"；Q4：审查请求写错了提交数，作者的反向对照列在各 Step 计划里，以此为准。

## 文档与代码的不一致

作者已知的偏差（D-a…D-h）之外：

- D1：3.1 的文件清单与 3.2：`domain/tree.go` 只有 `Subtree`（`Height`、`Holds`），"移动的位置"在 `app/unit_move.go`（D-a）；`SameParent` 导出、在 `change.go`。清单漏列 `domain/actions.go`（`node.move`、`node.delete`）、`module.go`，以及测试文件 `app/move_test.go`、`delete_test.go`、`fakes_test.go`、`write_test.go`、`domain/tree_test.go`、`adapter/postgres/store_test.go`、`purge_test.go`、`adapter/http/handler_test.go`、`extension_test.go`。
- D2：3.3 的 `Subtree` 写"先序"与"先序需要的列"（D-b）；代码另按笔记本限定（`Subtree(ctx, notebookID, id)`，别的笔记本的节点答 `ErrNotFound`），文档没写。
- D3：3.4 `Unit.Delete` 第 1 步写"锁下读出节点（`FindNodeIn`）；`Subtree` 读出整棵子树"；代码只调 `Subtree`，读不到即 404。3.4 也没写 `Delete` 返回子树（D-d）。
- D4：3.5 没有 `app.Destination`（D-d）。
- D5：3.2 的"后代一条（前后相同：位置没变，路径变了）"：同父页内的排序路径不变，后代照样在内（Q2）。
- D6：3.9 的 PG3 写"把高 3 的子树移到第 9 层的页下 409"；故事移到第 8 层之下（最深一页到第 11 层，正好越界），再移到第 7 层之下成功（最深一页恰好第 10 层）。代码的边界更好，文档照它改。3.9 与 S4 计划的 fixture"`moveNode`、`deleteNode`（只返回答复）"与代码不同（D-g）。
- D7：第 5 节的反向对照"删除的条目不进回收站（整个清理任务的测试里，删掉的子树清不掉）"：整个清理任务的测试（`bootstrap/purge_test.go`）用 SQL 种回收站里的子树，与 `RecordItem` 无关，ITEM-NODELETEDAT 之下照样通过；抓到它的是仓储的 `TestPurgeTakesADeletedSubtree`、`TestADeletionsItemIsDeletedWithItsNode`，模块根的 `TestTheTreesWritesReachTheUnit` 与 PG4。S1 计划的"清理的测试失败"是对的，第 5 节照它改。
- D8：第 2 节与第 5 节的"领域的表格（环、深度、位置、原地不动、连续移动后的重排）"：位置、原地不动、层级的表格在用例层（D-a）；没有"连续移动后的重排"的测试，重排由直接构造的数据触发（"between two with no gap"），且没有覆盖被移动的页在插入点之前（T3）。S1 计划"原地不动的四种（同父页同位置、同父页换位置、换父页、移到根下）"：原地不动的表三格、都在根下（T4），其余几种在位置的表里。
- D9：收尾时：P2 文档头的状态、第 7 节"结果"、M4 总设计第 12 节进度表；D-h 与 Q1 的移交（M9、M5）。

处置：D1–D9 在合并之后的文档提交里同步进 P2 文档与 Step 计划（3.1 的文件清单与 `domain/tree.go` 的内容、3.3 按层与按笔记本限定、3.4 的 `Delete`、3.5 的 `Destination`、3.2 的后代措辞、3.9 的 PG3 边界与 fixture、第 5 节的反向对照与表格的层次、文档头与第 7 节、M4 总设计第 12 节）。

核对过、与实现一致的：

- 写入单元：两个用例都照 `renameNode`——事务之外 `FindNode`（`page.not_found`），`Tree: true`（笔记本行 `FOR NO KEY UPDATE`，由用例层的调用次序与交错 34–37 守住），单元里在锁下重读（`FindNodeIn`、`Subtree`，都限本笔记本）；`moveNode` 在单元里重读作答；日志 `node moved`（只在有变更集时）、`node deleted`（带 `nodes=<数目>`），只有 id 与客户端，没有标题。`apply` 先守卫、再变更集、写、条目、参与者；单元结束时观察者一次。
- 码的次序：404 → 403 → 422（`parent_id`、`after_id`；`after_id` 是自己答 422）→ 原地不动 → 409（`page.cycle` → `page.title_taken` → `page.too_deep`）→ 守卫，与 3.4、M4 总设计第 5 节一致（缺的两格见 T6）。原地不动的判断 `sameParent && after == 原下标 − 1` 对最前、某页之后、最后三种都对；`sameParent &&` 在不变量之下是多余的（不同父页时节点不在新兄弟里，原下标为 −1），留着可读。
- 防环与层级：`Holds` 含节点自己，`parentID == nil` 不判环；层级 = `Depth(新父页的祖先链)` + 高度 − 1，根下为 0 + 1。重名只在换父页时用"去掉自己的兄弟"判断，同父页内的排序不查；唯一索引兜底（MoveNode 的 23505 → `page.title_taken`，`TestMoveNode` 守住）。
- 改动：被移动的页一条（前后都在，`Moves()` 为真，是条目）；后代 `sub[1:]` 各一条前后相同（不是条目，进守卫与事件），每次迭代各自的 `state`；删除每个节点一条只有"前"的改动，`Moves()` 为真、都是条目。多操作的单元里，后代的"前后相同"与先前的改动合并时保留先前的"前"、不再插条目，对的。
- 仓储：`Subtree` 起点要求本笔记本、未删，递归只走未删的子节点、按 `(notebook_id, parent_id)` 连接（走 `nodes_notebook_id_parent_id_idx`），界 64 与 `Ancestors` 相同；`MoveNode` 写父页、次序与审计列；`DeleteNodes` 的 CTE 以同一时刻改未删的节点（同时写 `updated_by_id`、`updated_at`）与这些节点未删的正文、版本、条目，回收站里的行保留原时刻，不碰变更集；`RecordItem` 对无"后"的条目在插入与合并时写 `deleted_at`。`changeset_items` 的唯一约束是完整的 `(changeset_id, node_id)`（不是部分索引），被 CTE 放进回收站的条目仍能被 `ON CONFLICT` 合并；条目的父页列没有外键，被删的父页不会挡住清理。
- 清理：活着的笔记本里删掉的子树，节点、正文、版本、条目同一时刻，"删叶子"的循环一次清完，变更集留下（`TestPurgeTakesADeletedSubtree` 断言五个清理器 `[6, 3, 3, 3, 0]` 与两个变更集都在）。清理只锁已删的行（`SKIP LOCKED`），树操作只改未删的行，不互等。
- 加锁顺序（13.1 第 5 条）：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 节点与跟随的行；`MoveNode` 改 `parent_id` 时外键检查对新父页取 `FOR KEY SHARE`，与别的写不冲突；`DeleteNodes` 不改树的外键列、不锁父页；两者写 `updated_by_id` 时对执行者的账户行取外键检查的 `FOR KEY SHARE`，与 P1 的新建、改名相同（P1 审查已核对它与 `FOR NO KEY UPDATE` 不冲突）。P4 的正文写持笔记本行 `FOR SHARE`，与树操作互斥；没有找到会成环的路径。
- 交错 34–37：两种先后各一个用例（34、37 用 `orders`，35、36 各写两种），结束时 `checkPages` 与 `checkNotebooks`；断言与 3.8 一致，34 的两种先后对称，37 的改名与移动各在先一次。
- 契约与适配器：`moveNode` 的 `x-problem-codes` 六个码与 3.5 一致，`NodeMove` 的 `parent_id` 必填可空（缺省答 400，`TestTheParentIsRequired`）、`after_id` 省略为最后、`null` 为最前，`additionalProperties: false`；`deleteNode` 204；id 不是 uuid 时 400；每个声明的码各答出一次；客户端按凭证。`bodyshape` 的表号重排（`NodeMove` 2、`PageCreate` 5）由 `gen-check` 核对。
- 权限：`node.move`、`node.delete` 给 `writers()`，`Actions()` 加了两个，规则表与模块的操作一致（`TestTheRuleTableIsTheModulesActions`）；矩阵两行的格与种子（priv 的子页换父页，team、orphan 的页原地不动），阅读者三格 403、看不到的 404。
- 树与逐项读取：经接口删除 team 的页（带子页）之后，每一列的树里没有、逐项读取 404，并调用 `checkPages`。
- 文案：`page.cycle` 中英两份，`problem-messages.test.ts` 核对契约里的每个码都有文案。
- e2e：`expectMoved` 核对条目的前父页、后父页与次序等于节点的、名称不变、节点的 `updated_by_id` 是移动的人、`updated_at` 等于这次变更集的时刻；`expectSubtreeDeleted` 核对子树的节点（`deleted_at`、`updated_at`、`updated_by_id`）、正文同一时刻，版本与条目没有别的时刻，删除自己的条目每页一条、在删除的人这次变更集里。PG3 的层级正好压在边界上（11 拒、10 收）。
- 代码风格：注释与周围一致（英文、以被注释的名字开头、说"为什么"），文件按操作分开，`unit_move.go` 84 行、`unit_delete.go` 32 行，没有上帝文件；新端口方法都经 `ports.go` 声明、由组合根接上；没有过度设计。

## 反向对照

做法：在快照里改代码（生成的 SQL 直接改 `gen/*.sql.go` 里的常量），只跑相关的包或测试（`go test -p 3 -count=1 [-run …]`），再 `git checkout -- .` 复原；e2e 的服务端变异重新 `make build` 后只跑 PG3、PG4、PG12。"锁外读子树"（STALE-SUBTREE）是用一个按节点 id 的 `sync.Map` 让用例在锁外读子树、单元改用它，只在交错 34–36 上跑。另在快照里加过一个只供审查的测试量删除的耗时（Q3），量完删除。

会失败的（34 项 Go、3 项 e2e）：

- 计划里列的（逐项复核，都成立）：MOVE-CYCLE-PARENTONLY 防环只看新父页本身（"under its grandchild"）；HEIGHT-ONE 高度恒为 1（领域的 `TestSubtree`；单元里忽略高度时 `TestMoveNodeRefusesASubtreeTooDeep` 与交错 35"新建在先"失败）；ITEM-NODELETEDAT `RecordItem` 不给删除的条目写 `deleted_at`（`TestPurgeTakesADeletedSubtree`、`TestADeletionsItemIsDeletedWithItsNode`、模块根；e2e 的 PG4：`deletions` 0、`items_apart` 3）；DELNODES-ALLDELETED `DeleteNodes` 也改已删的行（`TestDeleteNodes`）；MOVE-INPLACE-WRITES 原地不动也写；MOVE-DESC-NONE 后代不进事件；MOVE-GUARD-FIRST 先调守卫再查环（码的次序表"a new sibling's title"）；TREE-SHARE 树锁改 `FOR SHARE`（交错 34–37 的十个子测试都在等锁的期限失败：没有语句等笔记本行）；MOVENODE-NOAUDIT `MoveNode` 不写审计列（`TestMoveNode`；e2e 的 PG3：`by_mover`、`at_move` 为假）。
- 写入单元：MOVE-INPLACE-OFF 原地不动的下标差一；MOVE-DEPTH-BEFORE-CYCLE 层级与重名都挪到环之前（被"环先于重名"那格抓到）；MOVE-HEIGHT-OFF1 高度不减一；MOVE-TITLE-NEVER 换父页不查重名（用例层；整个程序上见下）；MOVE-NOPARTICIPATE 移动不调参与者；MOVE-DESC-ALL 后代从 `sub` 而不是 `sub[1:]` 取；MOVE-STEP-OPRENAME 守卫看到的操作种类错；CHILDREN-OTHERS-SAME 兄弟不去掉自己（`after_id` 是自己被接受）；DELETE-ROOT-ITEM-ONLY 只有根的改动没有"后"；DELETE-NOTREE、MOVE-LOGALWAYS（用例层的调用次序与日志）。
- 仓储：SUBTREE-ORDER-DESC 按层倒排；SUBTREE-NONOTEBOOK 起点不限笔记本；SUBTREE-ROOT-DELETED 起点可以是已删的；DELNODES-NOREV、DELNODES-NOCONTENTS、DELNODES-NOITEMS（`TestDeleteNodes`、`TestPurgeTakesADeletedSubtree`；DELNODES-NOITEMS 在 e2e 的 PG4 上 `items_apart` 3）；ITEM-MERGE-NODELETEDAT 合并时不写 `deleted_at`；ITEM-ALWAYS-TRASHED 有"后"的条目也进回收站（清理的测试）；MOVENODE-NO23505 不译 23505。
- 权限：RULE-MOVE-READERS、RULE-DELETE-READERS（矩阵的三个阅读者格）。
- 交错的断言本身：STALE-SUBTREE-move（34 的两种先后都 200、成环；35"新建在先"移到 11 层）、STALE-SUBTREE-delete（36"新建、写在先"：新页留在已删的 Child 之下）。

改了却没有失败的（11 项 Go，其中 2 项在不变量之下等价；1 项 e2e）：

- 对应发现：SUBTREE-BOUND3（T1，e2e 的 PG3、PG4、PG12 也通过）；MOVE-HEIGHT-LEN（T2）；MOVE-RENUMBER-SIBLINGS（T3）；MOVE-SAMEPARENT-PTR（T4）；MOVE-NF-RAW、DELETE-NF-RAW（T5）；MOVE-DEPTH-FIRST、MOVE-DEPTH-BEFORE-TITLE（T6）；DELETE-NOPARTICIPATE（T7）。
- 等价、不作为发现：MOVE-NOSAMEPARENT（原地不动的条件去掉 `sameParent &&`：不同父页时节点不在新兄弟里，原下标为 −1，条件本来就不成立）；SUBTREE-RECURSE-NONOTEBOOK（递归的连接去掉 `c.notebook_id = s.notebook_id`：复合外键保证子节点与父节点同一笔记本）。
- 部分层次存活、别的层次抓到（计入"会失败的"，作为 T8 的依据）：DELNODES-NOCONTENTS、DELNODES-NOREV、DELNODES-NOITEMS 在交错 34–37、树与读取一致与模块根上全部通过，ITEM-NODELETEDAT 在交错 34–37、树与读取一致、整个清理任务与 `TestDeletingANotebookDeletesItsPages` 上全部通过；MOVE-TITLE-NEVER 在交错 37 与模块根上通过（第二个写撞唯一索引，`MoveNode` 把 23505 译成同一个 `page.title_taken`；单元里先查的必要性在多操作的单元，由用例层的表守住，不作为发现）。

## 没能验证的风险

- 删除的耗时只在本机容器上量过（Q3：3000 个节点约 0.43 秒）；数据库在远端时每节点一次往返会放大，期间整本笔记本的写都在等。没有压测并发的写在这段时间里的排队。
- 守卫与事件的值随子树线性增长；M5 的 `NOTIFY` 若带改动集会超出 8000 字节（Q1），要到 M5 才能验证。
- 同一单元先建后删的合并（D-h）在 M4 走不到，现在没有测试；M9 的 batch、M7 的导入第一次用到时才会暴露。
- 删除时结束编辑会话、交错 39 与 42 的"删除页面"一侧在 P4；本 Phase 的删除不碰 `edit_sessions`，P4 加上之后加锁次序（`nodes → page_contents → edit_sessions`）要再核对一次。
- e2e 只有接口版本，页面上的移动（拖拽、"移动到…"对话框）与删除在 P5。
- 交错与故事的重复次数有限（见审查方式），竞态类的偶发失败不能完全排除。
- 没有跑 `make image-smoke`（按要求）。
