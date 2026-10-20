# M9/P3 batch 与多轮撤销：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M9/P3 batch 与多轮撤销（三个 Step：S1 batch；S2 撤销的调度；S3 后台撤销） |
| 日期 | 2026-10-19（修订 2026-10-20、2026-10-21） |
| 状态 | S1 已合并（`f19d0b6f`），S2 已合并（`43b03b36`），下一步是 S3；设计已审查（三位审查者），照主持者的处置修订一轮，复核三轮（"第一轮复核"、"第二轮复核"、"第三轮复核"），前两轮之后再修订，第三轮的两处由主持者直接改；审查记录 [P3-design-review.md](reviews/P3-design-review.md)；S1 的审查记录 [P3-S1-batch-review.md](reviews/P3-S1-batch-review.md)、S2 的 [P3-S2-revert-review.md](reviews/P3-S2-revert-review.md)，实施与审查之后与代码不同之处照实改在各节（3.28、3.29 列出），结果在第 9 节 |
| 上级文档 | [M9 总设计](00-M9-design.md) 4.1、4.2、4.5、4.7、4.10、4.11、4.12、4.14、4.15、第 5、7、9、10、11 节与"移交的落实"；[P1](01-P1-platform-reads.md) 第 8 节；[P2](02-P2-writes-history.md) 3.5、3.8、3.10、3.11、3.13、3.18、3.20、3.22 与第 6、8、9 节；[v0.1 总体设计](../v0.1-design.md) 4.5、5.3、6.1–6.4、13.1、13.2、13.4 |
| 代码位置的写法 | 同 00 号文档与 P2：模块写作 `page/app/unit.go:168`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；迁移写作 `00014_page_changesets.sql:13`（`server/migrations/sql/`）；前端写作 `web/…/transfer-report.tsx:24`；e2e 写作 `e2e/fixtures/mcp.ts:114`。代码的行号是 `f3854b9a` 的（P2 的 S2 合并；之后的 `f1733403` 只改文档），00 号文档、P2 与 v0.1 总体设计的行号是 `f1733403` 的；S1 合并时照实改的各处是 `f19d0b6f` 的（2.1 第 36–47 条引的 00 号文档的行号也是），S2 合并时照实改的各处是 `43b03b36` 的（2.1 第 48–56 条引的 00 号文档的行号也是） |
| 模拟与原型 | 撤销的调度的随机模拟在 `scratchpad/m9/p3/sim/`（`schedule.py`、`ce.py`、`ce2.py` 与各次的输出 `out-*.txt`），照设计复核 B 的模拟（`scratchpad/m9/review-B/`）改写：加了正文、回收站里的正文、"先写回、再删除"、临时名称避开 S\* 的键与段的量（3.16.4、3.22.2）。审查之后的复核在 `scratchpad/m9/p3/sim2/`（`run_all.sh` 跑全部，结果在 `results.txt`）：每条后备的计数与轮数的上限（`schedule_cap.py`）、`-short` 的下限（`short_floor.py`）、每批的核对（`batches_rev.py`；复核之后的 `delcheck`、`reads` 与注入 `movegone` 由 `run_r2.sh` 跑，第二轮复核的 `naming` 由 `run_r3.sh` 跑）、减半（`halving_rev.py`）、恢复的循环（`restore_loop_rev.py`）。链接的规则的原型在 `scratchpad/m9/p3/exp/`（`f1733403` 的 `server/` 的副本里的 `zz_p3*.go`，`go test -short`，不要数据库，3.7.1；复核之后加的在 `results.txt` 的 ROUND 2 与 ROUND 3） |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P3 与 P1 最重，分三个 Step，各自一个分支、一次审查与修复核对、一次合并；后一个从前一个合并之后的 `main` 开始。S1 与 S2 在服务端与 e2e；S3 另有网页的一条文字（第 7 节"唯一的网页改动"）。每个 Step 里的每个提交都是绿的（第 5 节）。

- **S1 batch**（分支 `m9-p3-s1`）：
  - 先量搜索的积压（任何 batch 的代码之前，它定入队的方式，3.10.1）；`ErrBatchTooLarge` 改名 `ErrJobBatchTooLarge` 是单独的一个提交，在新的哨兵之前（3.9.2）；
  - 字段名的一种写法：P2 的每一步的规则都收它报的字段名（`domain.Args`），`batch` 以 `ops[i].` 起头，一条规则一份（3.2；S1 审查 B-N3）；
  - `CheckBatch`（取值 422，合计在各项之前，项的次序）、`PlanBatch`（两遍：树的一遍调 P2 的规则、给新建的页 id、结束时答条目的界线；先以大小拒绝、再一起读 `edit` 的页的正文与版本号，`write` 的页只读头；正文的一遍以 `BatchLinks` 改写链接，计划自己的解析有界、按（页、哈希）重用事实、与最后的正文共用一个等待）、规范化（`merged_into`）、B1 与 B3、正文合成、回到原处的条目删去（3.3–3.5）；
  - 用例 `ApplyBatch`：计划、锁的预检、按（页、哈希）一次解析、单元里重做每一步并核对节点 id（新建的也是）与版本号（字节相同、不带令牌的 `write` 也是）、不同就从头重做一次（3.6）；
  - 推后的链接改写：单元的参与者推到全部操作之后一次；没写的页照合并的改动（含新建之后又被移动的节点）；写了的页各照它最后一个正文操作之后自己的改动（(e)；它们改名、移动的节点的子树经 `spread`，一个 batch 读一次、存一份），链接取自 `DecidedAll` 为它最后的正文留下的事实（(a)–(d)），不论索引碰不碰得到它；删除了的页的链接行与别名在 `resolutions` 之前去掉；`Recased` 是集合；`PathBefore` 从根撤回、原来的父节点的路径在单元开始时读出；改写的页数在解析之前数；batch 持着的事实在推后的改写之前放还份额；linking 的 `BatchLinks`（3.7）；
  - 界线（读的页以大小、计划自己的解析、`MaxBatchRewrites` 的实测与它的守卫、`mcpserver.MaxBodyValues` 的对照；S1 的审查另加移动带着走的节点、改写的链接与页的字节的界线与改写过的页留下的解析额度的检查，3.8）、答复（之后被删除的页的正文操作、每项的 `changed`、`building`）、错误与码的次序、加锁的次序、搜索（S1 量过了线：批量入队，3.10.1）与事件（3.8–3.11）；
  - 工具 `batch`、`tools.json` 的两个条目、内置的说明的一行（写着 4.11 的与分开调用的不同的那一条规则）、码的表、矩阵的一行、工具的字符串的合法参数、`page.batch` 的动作与规则（3.12、3.13）；
  - 测试：合并的每种在真的库上（含改名再改回）、与分开调用相同（19 种，日志里没有 error 级的行）、与分开调用不同之处（4.11 的规则的各种）、整个被拒与每个界线、交错、两个"等于重建"的测试加 `batch`，4.11 末条的性质在其中的 batch 一步核对（3.14）；e2e：MC5 (MCP)（第 4 节）。
- **S2 撤销的调度**（分支 `m9-p3-s2`）：
  - `schedule()` 代替 `round()`：前提、三条后备（第 3 条留作防御：有第 2 条时任何搜索都没用到它）、轮数的上限写进代码、`ErrRevertUnordered` 只是缺陷；"为什么做得完"写成测试能核对的几条（3.16）；
  - 临时名称 `AsideName`（后 8 位、32 位、再加序号，避开现在的与 S\* 的键；UTF-8 的边界、255 字节、附件的扩展名）（3.17）；
  - 先恢复再写回（写在哪个版本号上、按（页、哈希）解析与配对、界线）、撤销的撤销"先写回、再删除"、`RevertWrites` 改为所有带 `Base` 的版本（3.18）；
  - 步数：临时的步计入 `MaxRevertSteps`，后台不对整个计划核对（3.19）；
  - `changesets.renames`、`moves`：迁移 00045 与补值、单元结束时设上、`getChangeset` 读列（3.20）；
  - 测试：4.12 的每种在真的库上（两种以 SQL 种下）、审查的反例、钉住 M8 的行为的三处改期望、撤销的性质测试加 `batch` 与定向的生成器、比之前就在回收站里的页的正文、domain 的性质测试以它自己的模型判定（3.21）；e2e：MC5 (page)（网页整组撤销）。
- **S3 后台撤销**（分支 `m9-p3-s3`）：
  - 任务开始时的计划就是 `schedule()` 的步，任务只留紧凑的几样（次序、种类与序号、目标与名称、写回的 `Base` 与哈希、删除那一刻的子树、恢复的成员），`revertPlan.groups` 照今天；删除照它在调度里那一刻的子树；恢复按成员展开，每页的写回紧跟它的恢复；不可分的段在 `JobStep.Joined` 上标出（只在内存里，不存）；段里的恢复只展开到段里后面的步要的成员（3.22）；
  - transfer 只在段与段之间切批，减半到第一段的末尾与一半之内的最后一个边界中靠后的那个；一段放不进一批（超过单元的历史或链接，且不只一个节点）以 `revert_too_large` 停下；单元的"一步不受界线"改为"一个节点不受"（3.23）；
  - 每批在锁下：L（成员 0（一组的第一个成员）在这一批里时读全组；这一批恢复的页照回收站里的正文核对），再按次序核对每一步的前提（不再对每一批建 S\*；`canDelete` 比那一步记下的节点，以多出的每一部分的顶为名）；步按序号取任务开始时的计划，恢复的组与时刻取这一批读的；一组在一批里一次恢复；写回以刚核对过的版本号为基（3.24）；
  - `revert_too_large`：transfer 的报告、契约的取值、生成的类型、网页的一条文字（中英）（3.25）；
  - 测试：四个整个程序的测试（名称环在每个批的边界上读树、批之间的写含一个后面的批的写回与把一页改成别的字节再改回的两次写、一段过了一批、大的文件夹的三种形状与一组在一批里一次恢复）、transfer 的切批与减半的单元测试、任务的步的重放照 `RevertBatch` 读每一批（注入含 `movegone`）；HI5 照常（3.26）。

**为什么这样分、每一步怎样单独是绿的**：S1 是新的写的工具与它的单元，S2 改网页的整组撤销（同步的单元与计划），S3 改后台的撤销与 transfer。

- **S1** 合并时 `batch` 的变更集已能在网页上撤销，但 M8 的单轮只做得了 M8 的形状：batch 里有名称环、"新建同名再移入"一类时 `round()` 的前提不成立，是 `ErrRevertUnordered`（`page/domain/revert.go:89-97`，网页 500，单元回滚、什么都不写）；"先改正文、再删除"（B3 不拦它，4.12）的撤销今天会以删除时的正文恢复这一页、悄悄地给错的正文（`RevertWrites` 不取删除的页，`:280-286`）。S1 不让 `main` 有悄悄写错的路：`readReverted` 加一条临时的检查，C 删除的页有带 `Base` 的版本行时 `PlanRevert` 答 `ErrRevertUnordered`（M8 的变更集从没有这种形状：编辑会话只写正文，导入只建，M8 的撤销与恢复不先写再删；`page/domain/revert.go:257-290`），S2 以"先恢复、再写回"换掉它；钉住这种形状的 `page/domain/revert_test.go:342-347` 一行在加检查的同一个提交里改期望（3.21.1）。S1 的测试只撤销 M8 的单轮做得了的 batch（撤销的性质测试在 S2 才加 `batch`；`TestTheSearchIndexIsItsRebuild` 在 S1 打开 `batch` 但不撤销 batch 的变更集，3.14.6），所以是绿的；S1 与 S2 之间网页撤销这些形状答 500，不发版（v0.1 在 M12 才发）。
- **S2** 合并时同步的撤销完整。后台的路径 S2 也换成 `schedule()` 的步，但 transfer 还会在任何两步之间切批（S3 才认段）：临时名称会在两批之间提交、`RevertBatch` 的重新计划也对不上临时的步。所以 S2 在任务的计划（`page/app/jobs.go:281-301`）里加一条临时的拒绝：计划里有临时的步、恢复之后的写回或"写回再删除"时，`PlanRevert` 照 S1 的样子以 `ErrRevertUnordered` 拒绝（任务以 `internal` 失败，什么都没写；M8 与 S1 时这些形状同样失败，见上一条），其余的照 M8 切批、照 M8 每批重新计划。这样 S2 的 `main` 从不提交临时名称，也不写错正文；S3 之前，几种 batch 的形状的后台撤销还可能以假的冲突停下（审查 B-I1 的"移入新建的文件夹"），停下时已写的照 M8 留着、报告写明，同样不写错。建成的拒绝是 `errUnsplit`：以 "a background revert of steps its batches would split, refused until M9/P3 S3" 包住 `ErrRevertUnordered`（`errors.Is` 仍认得它），说出它是什么，不把运维引向调度的缺陷（S2 审查 A-N4，主持者的决定 6）。每批的重新计划也跑 `schedule()`：在一部分的状态上它原则上能排出整个计划里没有的临时的步，`batchOps` 让那一批失败、任务以 `internal` 失败，之前的批已提交、没写错；这同属 S3 之前接受的假的冲突，S3 换掉每批的重新计划（S2 审查 A 的存疑，主持者的决定 11）。
- **S3** 只动后台：任务的计划与段、transfer 的切批、`RevertBatch` 的核对、报告的失败与网页的一条文字。S2 那条临时的拒绝留到 S3 的第 5 步（transfer 认段）才删去：之前的提交里 transfer 还在任何两步之间切，而 M8 的形状没有多于一步的段（没有临时的步、恢复的页不写回、没有"写回再删除"），每一步都是段的边界，所以前四步的每个提交对 M8 的形状与今天切得一样（5.3）。

## 1. 基线

从 P2 完成之后的 `main` 开始：`f1733403`（P2 的 S2 的文档提交；代码同 S2 的合并 `f3854b9a`，本文代码的行号是它的）。

- **写入单元**（M4–M9/P2）：
  - `Writer.Run(ctx, spec, do)`（`page/app/unit.go:168-230`）：事务之外读工作区；事务里锁工作区行 `FOR SHARE`、笔记本行 `FOR SHARE` 或（`Tree`）`FOR NO KEY UPDATE`（`:196-206`），锁下判定，读一次时钟，`do`，搜索的入队（`queueSearch`，`:434-446`），观察者一次（`tell`，`:450-461`）。`UnitSpec.Message` 写进新建的变更集（`:65-105`、`ensureChangeset` `:339-365`）。
  - 每个操作经 `apply`（`:264-291`）：守卫 → 第一次写入时建变更集 → 写 → `recordItems`（`:370-392`：合并进单元的改动，只有 `Change.Moves()` 的进条目）→ 参与者（`participate` 为真时）。`recordItems` 的注释说"M4 的单元没有先建后删"（`:366-369`）。
  - 条目的合并在 SQL 里（`page/adapter/postgres/queries/changesets.sql:11-44`）：第一次的"前"、最后的"后"与 `deleted_at`；合并进删除时取删除的版本号，否则留第一次的；`RecordRevision` 在同一变更集同一页、行的版本号等于新的基时更新那一行（`:46-57`）：一个变更集里一页至多一个版本行。条目的检查：至少有一个状态（`00015_page_changeset_items.sql:25-28`），版本号只在前或后为空时（`00038_page_changeset_items_deleted_revision.sql:8-9`）。
  - 正文的写 `writeContent`（`page/app/unit_content.go:46-96`）：锁正文行、内容相同不写、`Base` 不是当前的答 `page.revision_mismatch`、经守卫写、记版本行；它的注释说"在 `FOR SHARE` 下写几页的单元按节点 id 的次序锁"。
  - P2 的单元操作：`Unit.Relocate`（`page/app/unit_relocate.go:34-74`，名称 422 → 环 409 → 名称 409 → 层级 409，"Move 的次序"）、`Unit.Subtree` 与 `Unit.DeleteSubtree`（`page/app/unit_delete.go:26-62`）、`CreatePage`（`unit_create.go:28-53`，新的页的 id 由它做：`uuid.NewV7()`，`:63`）。
- **P2 的四个用例**（`page/app/write_path.go:75-252`、`edit_page.go:74-214`、`relocate_path.go:55-157`、`delete_path.go:51-95`）：每个在单元里以 `NodePaths` 按路径找（`page/app/node_paths.go:36-122`，`Walk` 每段一次点查）；每一步的规则在 `page/domain/steps.go:31-92`：`CreationOf(field, …)` 与 `MissingAncestors(field, …)` 收字段名，`MoveKinds` 写死 `"to"`（`:77-82`），`DeletionOf` 经 `Recursive` 写死 `"recursive"`（`page/domain/errors.go:203-212`），`CheckEdit`、`Replace` 写死 `"old_string"`、`"new_string"`（`page/domain/edit.go:21-39`、`:72-119`），用例里另有两处写死：`write`、`edit` 拒绝附件的路径（`write_path.go:84-87`、`edit_page.go:83-86`）与 `move` 的父页链的 `holdsNoNodes`、`PathNotFound.ParentOf`（`relocate_path.go:132-157`）。`Placement.Field` 收字段名（`unit_relocate.go:16-20`）。
- **撤销**（M8/P3、P4，P2 的一步的放回）：`PlanRevert`（`page/domain/revert.go:358-375`）：`left()` 比 C 写出的状态（`:415-500`），`before()` 对 S\* 一次核对（`:543-625`），没有冲突就 `round()`（`:811-861`）：放回（按 id）→ 删除（最上层）→ 恢复（各组）→ 写回，任何一步的前提不成立就是 `ErrRevertUnordered`（`:89-97`，网页 500）。`RevertWrites` 只取既没新建、恢复、也没删除的页（`readReverted`，`:280-286`）。`RevertPlan.Steps()` 数放回、删除、恢复（`:133-141`），请求里至多 `MaxRevertSteps` = 1,500（`:41-55`）。单元的 `Revert`（`page/app/unit_revert.go:61-118`）以计划时活着的页的 `state.Contents[id].Revision` 为写回的基（`:106-107`），解析好的正文按页 id（`RevertRead.Parsed`，`:30-36`；`parsedAs`，`:232-244`）。
- **后台撤销**（M8/P4）：`PlanRevert`（`page/app/jobs.go:281-301`）不传步数的界线（`revertBounds{Items, Locks}`，`:296`）；`revertSteps`（`:305-358`）把计划展开成 `JobStep{Nodes, Bytes}`（`page/domain/jobs.go:36-39`）：放回一个节点、删除它的子树、恢复按成员的先序一个一步、写回一页；`RunBatch`（`:370-382`）在单元之前数历史（多于一步时过了 `MaxUnitHistory` 是 `ErrBatchTooLarge`，`withinHistory` `:424-439`）、解析正文（多于一页时过了链接是 `ErrBatchTooLarge`）；单元的 `RevertBatch`（`page/app/unit_revert.go:402-485`）对这一批的节点重新跑一轮 `planned`（`replan`，`:535-551`），要求它的步与这一批的相同（`batchOps` 按（类型、节点），`:555-574`）。transfer 的 `batch`（`transfer/app/background.go:134-156`）按 `Batch{Nodes: 100, Bytes: 8 MiB}` 长一批、第一步总在里面，`ErrBatchTooLarge` 时减半；失败的种类在 `transfer/domain/report.go:13-77`，网页的文字在 `web/…/transfer-report.tsx:24-75` 与 `en.ts:690-696`、`zh-CN.ts:669-674`。
- **链接的参与者**（M6/P4，P2）：`linking.Rewrite.Participate`（`linking/app/rewrite.go:47-89`）跟着一个操作：`Relocation` 碰上带版本号的、新建的、删除的改动就不改写（`linking/domain/change.go:67-80`）；链接取自索引的行（`reached`，`:95-132`）；内容的版本号比索引新就跳过（`:227-231`）；追加的写以索引的版本号为基（`:309`）；`tree.Before` 由 `PathBefore` 从现在的路径推回（`:166-206`；`PathBefore` 碰到路径上第一个改动就返回，`change.go:101-122`），原来的父节点的路径读现在的树（`FormerParents`，`:169-172`）；只有索引是现在的版本与提取器的页才处理（`IndexedOf`，`:57-68`）；合并的改动什么都不搬时在 `:49` 就返回（`A → B → A` 合并成 `A → A`，`change.go:32-37`）；`Relocation` 只留最后一个只改了大小写的（`Recased`，`change.go:74-78`）；链接与别名的来源页由 `resolutions` 按 `Pages.Paths` 找，找不到（单元删除了的页，它的行与别名行还在：`AliasesByKeys` 不看页活不活着，`linking/adapter/postgres/queries/pages.sql:60-64`）是 500（`rewrite.go:107-117`、`linking/app/resolver.go:119-166`），在读任何正文之前；来源页的正文读不到也是 500（`:219-226`）；没有写法通向它的链接以 error 级记日志（`:252-254`、`:315-318`）。
- **MCP 的平台与写的工具**：`mcpserver.Tool`、`NewTool[In]`、`Fit`、`ToolError{Err, Text, Extra, Fit}`、`ErrorFit`、`MaxAnswerBytes` = 48 KiB、`MaxErrorLocks` = 20（`platform/httpserver/mcpserver/tool.go:32-210`、`answer.go:11-187`、`errors.go:18-231`）；输入 schema 由 jsonschema-go 推导再整理，`addNode` 只收 `type`、`properties`、`required`、`additionalProperties`、`items`、`description`（`schema.go:21-197`）：没有 `oneOf`、`enum`、`maxItems`，所以 `ops` 的项只能是一个平的对象。`MaxBodyValues` = 10,000，只在边缘数（`edge.go:56-77`，它的注释已写 P3 的 `batch` 约 700 个值）。page 的 `adapter/mcp`：四个写的工具（`write.go`、`edit.go`、`move.go`、`delete.go`）、`toolErr`、`writeErr`、`moveErr`、`lockErr`（`text.go:13-75`、`locks.go:38-98`）、内置的说明（`instructions.go:10-62`；笔记本入口 2,961 字节、账户入口 3,031 字节，4 KiB 之下各剩 1,135 与 1,065 字节，`instructions_test.go:15-27`）。
- **bootstrap 的检查**：码的表 `mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:78-95`）与 `TestTheMCPToolsErrorsCarryTheirCodes`（`bootstrap/mcp_reads_test.go:386-450`，写的调用在 `bootstrap/mcp_write_codes_test.go:37-161`）；矩阵的 `mcpRow`（`permission_matrix_mcp_test.go:25-41`）与写的四行（`bootstrap/permission_matrix_mcp_writes_test.go:46-158`），共 260 格；`validToolArguments`（`bootstrap/mcp_reads_test.go:667-686`）与 `withArgumentAt` 对 `ops[]` 只换第一项的成员（`:570-606`）；`declaredCodes` 的注释说"P3 加 MCP 自己的 `page.batch_too_large`"（`bootstrap/mcp_client_test.go:322-329`）。
- **名称的冲突**：`domain.ErrBatchTooLarge` 已是后台任务减半的哨兵（`page/domain/errors.go:101-106`，根的再导出 `page/tree_writes.go:57`；用在 `page/app/jobs.go:363-478` 的四处，其中两处交给 `DecidedAll`，`page/app/jobs_test.go:462-486`、`:746`，`bootstrap/background_cost_test.go:303`、`bootstrap/background_test.go:269`；组合根译成 transfer 的，`bootstrap/transfer.go:231-232`）。新的 API 的哨兵若沿用这个名字，漏改的一处照样编译，悄悄地与 API 的错误比较，任务的减半就停了（审查 A-N4）。
- **测试**：撤销的性质测试 `TestRevertingTheNewestChangesetsPutsTheNotebookBack`（`bootstrap/revert_property_test.go:238-290`，操作的表 `:109-110`，只比回收站里的 id，`:272-276`）；`TestTheIndexIsItsRebuild`（`bootstrap/links_rebuild_test.go:33-95`）、`TestTheSearchIndexIsItsRebuild`（`bootstrap/search_rebuild_test.go:18-45`）；后台的帮助函数 `newBatchedTeam`、`atFirstBatch`、`behindFirstBatch`（`bootstrap/background_test.go:160-213`）；MCP 的交错 `bootstrap/interleavings_mcp_test.go:21-300`。
- **CI**（主持者告知的 S2 合并之后的 run 39245208047）：组合根三份的测试一步 6:58、4:32、3:24；第一份最近两次 7:09、6:58（平均约 7:04，慢的约 7:10），线在约 7:30（`-timeout 15m` 的一半，00 号文档第 11 节第 28 条）：**第一份几乎没有余地**，`TestA…`、`TestBatch…`、`TestDeleting…` 这样的名字都落在它里面（`Makefile:217-219` 的 `BOOTSTRAP_FIRST`、`BOOTSTRAP_THIRD`）。
## 2. 目标与范围

做的事对应 00 号文档第 7 节 P3 一行（`:430`）与第 3 节的 `batch`、整组撤销两条：

- **S1**：`batch`（`ops`、`message`）：几个写操作与一句说明，一个变更集原子执行，结果等于按次序分开调用，与分开调用的不同写成一条规则（4.11）；它的计划（给新建的页的 id、先以大小拒绝再读、计划自己的解析的界线）、规范化、B1 与 B3、合并、正文合成、回到原处的条目、推后的链接改写、界线、答复、错误与码的次序；工具的契约、说明、码的表、矩阵与工具的字符串；字段名的写法；e2e MC5 的 MCP 部分。
- **S2**：网页整组撤销 batch 的变更集：按依赖的调度、三条后备与轮数的上限、临时名称、先恢复再写回、撤销的撤销、步数；`changesets.renames`、`moves`；e2e MC5 的网页部分。
- **S3**：后台的整组撤销按任务开始时的紧凑的计划切批：删除照那一刻的子树、不可分的段、只在段的边界上减半、`revert_too_large`，每批的 L 与按次序的前提、一组在一批里一次恢复。

不做：恢复、还原、撤销的工具（第 11 节第 1 条：agent 只读历史）；MCP 开后台任务（同条）；`grep`、`search`、`index`、`backlinks`（P4）；其余故事的 MCP 版本与接入文档（P5）。batch 的批量入队原定只在 3.10 的实测过线时才做：S1 的修正的第 1 轮照 batch 的积压（最新的页）重量，过了线，做了（3.10.1，S1 审查 C-I2）。

**P2 第 8 节交给 P3 的各项**，逐项落实：

| P2 的项 | 落实 |
|---|---|
| 单元里的路径缓存与它在移动、改名、删除之后的作废（P2 2.1 第 1 条） | 不建缓存：计划在单元之前读整棵树（`Nodes.ListNodes`，一条语句）在内存里模拟，单元里的重做每一步以 `NodePaths.Walk` 逐段点查（至多 100 项 × 11 段 = 1,100 次点查，约 0.1 秒），只核对找到的节点 id 等于计划的；作废的规则由"每一步在单元的事务里重新找"代替，没有可以过时的缓存（3.6） |
| `PlanBatch` 在模拟的树上调 P2 的每一步的规则（`CreationOf`、`MoveKinds`、`DeletionOf`），不另写一份 | 3.4：模拟的树实现 P2 的规则要的读（`found` 的节点、子树、兄弟的键、父页的一线），规则本身是同一组函数；`Unit.Relocate` 里的名称、环、名称、层级四个检查与它们的次序抽进 domain 的 `RelocationOf`（今天写在 `unit_relocate.go:39-68`），单元与计划共用；同一个父节点下的改名照旧不读子树（审查 A-N11） |
| `batch` 的错误经 `ToolError.Fit` 带出错的一步与它的锁，`page.batch_too_large`；每一步的码的次序与文字照 P2 的四个用例 | 3.9：每一步的错误由 P2 的 `writeErr`、`toolErr`、`moveErr`、`lockErr` 写出，`atOp` 在文字的码之后加 `ops[i] (<工具>): `、在 `Extra` 加 `op`；`Fit` 的错误照样（`ErrorFit.Build` 包一层）；码的次序 3.9.3 |
| 撤销：P3 只做多轮的调度、三条后备、临时名称、写回的 `Base` 与后台的切批 | 3.16–3.19（S2）、3.22–3.24（S3）；一步的放回照 P2 的 `place`（`page/app/unit_revert.go:125-157`），临时的步也经它 |
| 工具的字符串的检查：`batch` 加自己的一项合法的参数（`ops` 里一个合法的 `write`），没有它测试失败；`ops[].path` 等只换第一个 `op` 里的 | 3.13.3：`validToolArguments` 加 `"batch": {"ops": [{"op": "write", "path": "Strings.md", "content": "x"}], "message": "strings"}`；`withArgumentAt` 已只换第一项的成员（`bootstrap/mcp_reads_test.go:578-595`）。平的项让 `ops[].from` 一类"这一项的工具不收"的参数也被推导出来，它们到达用例、答 `ops[0].from` 的 422 `not_allowed`，不是 `bad_request`，也不是 `internal_error`：这正是检查要的"到达用例" |
| 说明的 `batch` 一行（账户入口 1,065 字节，与 P4 的四行共用） | 3.12.3：425 字节（`TextBytes` 的数法，换行 2 个字节；UTF-8 424），笔记本入口之后 3,386 字节、账户入口之后 3,456 字节，给 P4 各剩 710 与 640 字节；工具的各行两个入口共用，起作用的是账户入口的 640（第 8 节）。这一行把 4.11 的那一条规则（第 19 条）缩成一句 |
| 码的表、矩阵（带 `write` 与计数的 `after`）、`tools.json` 的条目照 P2 的写法 | 3.13.1、3.13.2、3.12.2 |
| 422 的字段名的写法（S1 审查 B-N3，主持者的决定第 33 条）：每条规则收字段，或一个改写字段的函数 | 每条规则收字段（3.2）。改写的函数不行：`Recursive` 与 `PathNotFound.ParentOf` 的字段也写进文字（`page/adapter/mcp/text.go:35-38`、`:50-53`），`VersionMismatch` 一类没有字段、文字带路径，改写 `shared.FieldError` 改不到它们；而它要在错误生成之后认得每一种带字段的类型，新加一种就漏一种 |
| `batch` 的上限点名 `mcpserver.MaxBodyValues` | 3.8.1：最大的合法 `batch` 约 715 个值（每项至多 7 个：对象、`op` 与 `edit` 的五个参数），带上平的项里全部 11 个成员也只约 1,215 个，是 10,000 的约八分之一；`domain.MaxBatchOps` 的注释写这个对照；整个程序的测试 `TestTheLargestBatchIsFarWithinTheValuesBound` 把这两个请求体发出去（前者成功，后者以 422 `not_allowed` 拒绝，都不是 400），不数值的个数：bodyshape 不导出它的数法，两个请求体就是对照本身（审查 C-M9、C-O2、A 的过度设计一条） |
| `move` 的父页写成页的路径答 `write` 那样的 422（`MissingAncestors`）：`batch` 的 `move` 照用 | 3.4：计划的 `move` 走同一个 `parentOf` 的规则（抽进 domain 的 `ParentChainOf`），字段是 `ops[i].to` |

**00 号文档"移交的落实"里交给 P3 的**：

| 移交 | 项 | 落实 |
|---|---|---|
| [M4/P2 一个单元里先建后删](handoffs/M4-P2-unit-merge.md) | 条目的检查、事件与守卫的值、删除再恢复、恢复再删除 | 新建后删除由 B1 拒绝（422），先删子孙再删祖先被规范化，改名或移动又回到原处的条目在单元结束时删去（3.5.4）；事件与守卫的值：同一页的几个正文操作合成一个版本行、守卫看到一次最后的正文（3.5.2），事件一个单元一次、取合并之后的改动（3.10.2）；删除再恢复、恢复再删除不会在一个单元里相遇（batch 没有恢复的操作；撤销的一个单元里一个节点至多删除或恢复一次，4.12）；每种合并在真的库上有多操作的单元的测试（3.14.2）；`recordItems` 的注释改写（3.5.3） |
| [M5 编辑锁与 MCP 的写](handoffs/M5-locks.md) | 2 batch 整个回滚，答复说出第几步 | 锁的预检与守卫在某一步拒绝时整个单元回滚，答 `page.locked` 带 `op` 与持锁人（3.4.5、3.9）；预检也看每个删除的子树里的页，按项的次序（S1 审查 A-M1）；MC5 第三步（第 4 节） |
| [M6/P4 链接改写](handoffs/M6-P4-rewrite.md) | 1 一个单元里几次改名；3 大笔记本的改名 | 1：推后的一次调用；没写的页照合并的改动（含新建之后又被移动的节点），写了的页照它最后一个正文操作之后自己的改动，(a)–(e)；删除了的页的行与别名在 `resolutions` 之前去掉；`Recased` 是集合；`PathBefore` 从根撤回，原来的父节点的路径在单元开始时读出（3.7）；整个程序的测试：24 种（两种次序、"改名、改正文、再改名"、嵌套的改名、移出再删除、删除链到被改名的页的页、改名再改回、新建之后拿走链接、写了的页链进只被移动的文件夹；第 20–24 种是 S1 的修正加的：移动、改名之后新建，冻住的节点之下，3.14.3）；3：`MaxBatchRewrites` 照 M8 的规则以满负载的页实测为 400（写了的页算 2），另有改写的链接（120,000）与页的字节（34,406,400）的界线和改写过的页留下的解析额度的检查（3.8、3.8.2）；单个 `move` 不受它们限，一个共用的界线留给 M12 |
| [M8/P3 batch 的整组撤销](handoffs/M8-P3-revert.md) | 1–9 与"另" | 1：3.16（第 3 条后备留作防御，轮数的上限写进代码）；2：一步的放回在 P2（[P2](02-P2-writes-history.md) 3.5.4），多轮的经它，batch 里"移动加改名的中间态"在真的库上测（3.21.2）；3：3.17；4：调度让恢复等它的父组（以构造的计划者核对，`TestARestoreWaitsForTheGroupItGoesUnder`），B3 让"移出再删除"的折叠不出现（3.4.3），"恢复到另一组之下"以 SQL 种下（batch 与撤销都做不出它）：M8 的 L 把上下相套的两个回收站项读成一组，撤销答 409 `page.revert_conflict`、什么都不写（3.21.2，S2 的主持者的决定 1）；5、7：3.18；6：B3（3.4.3）与 `store_test.go:757` 的合并照留；8：3.19；9：3.21；"另"：`renames`、`moves` 3.20；`batch` 的说明经 `UnitSpec.Message`（3.6） |
| [M8/P4 MCP 开的后台任务](handoffs/M8-P4-background.md) | 2 batch 的大撤销 | 3.22–3.24：紧凑的计划、删除照那一刻的子树、段、只在段的边界上切批与减半、每批的 L 与按次序的前提、一组在一批里一次恢复、`revert_too_large` |
| [M8/P5 grep 用搜索的索引](handoffs/M8-P5-search.md) | 2 写完即搜 | batch 批量入队（`UnitSpec.BulkSearch`），答复带 `building`：积压的最坏（约 1,200 个加推后的改写写的页，`MaxBatchRewrites` = 400 时约 1,600 个）照 3.10.1 实测；S1 的第 0 步照常量没过线，修正的第 1 轮以最新的页的积压重量，n = 2,200 多 16 个，过了 10 个（S1 审查 C-I2） |

### 2.1 对 00 号文档的修订

P3 的文档提交随这份文档一起改 [`00-M9-design.md`](00-M9-design.md)（照 P1、P2 的做法，以脚本落下：`scratchpad/m9/p3/apply-00m9.py`，每段原文恰好出现一次才改、全部对上才写，`--check` 核对过）。行号指 `f1733403` 的 00 号文档（P2 的 S2 的文档提交之后）。每条说 00 号文档里改什么、理由在哪一节；改了的原文在脚本里。第 36–47 条是 S1 的实施与审查之后加的，随它的合并改进 00 号文档（主持者以 `scratchpad/m9/p3s1docs/s3_m9design.py` 落下，同样每段原文恰好出现一次才改；行号指 `f19d0b6f` 的），与第 2、6、7、15、19、21、25、33 条不同之处以它们为准。第 48–56 条是 S2 的实施与审查之后加的，随它的合并改进 00 号文档（以 `scratchpad/m9/p3s2docs/03_m9.py` 落下，同样每段原文恰好出现一次才改；行号指 `43b03b36` 的），与第 8、9、13、34、35 条不同之处以它们为准。

1. **字段名的写法**（第 11 节主持者的决定第 33 条 :675；4.11 的"约束" :266）：第 33 条加"P3 定：每条规则收字段"（`domain.Args`，3.2）。
2. **计划的两遍与它的码的次序**（4.11 的"两段"第 1 条 :264；4.15 的 `batch` :368）：4.11 写明两遍：第一遍在不加锁的整棵树上模拟每一步，结束时答条目与读的界线（第 25 条），都在任何正文的读之前；第二遍只跑第一遍出错的项之前的，更早的项出错时取代第一遍的；给新建的页带 `base_version` 的 422 在令牌的节点不是这一页的 409 之前；计划之后以 `Locks.Of` 预检（3.4、3.4.5）。4.15 的 `batch` 一条照 3.9.3 写成："……→ 403 → 计划的第一遍按项的次序（第一个出错的项的 404、422、409；条目的 422）→ 读的 422（以大小）→ 计划的第二遍按项的次序（第一个出错的项的 422、409，与它要的计划里的解析的 422、503）→ 锁的预检的 `page.locked` → 422（页数、字节）→ 503 → 422（链接）→ 单元里按发生的次序：409（重新计划之后仍不同）、守卫的 `page.locked`、推后的改写的 422（改写的页数）、`linking.pages_locked`、503"。
3. **约束在哪里核对**（4.11 的"约束" :266）：改为"计划里核对，单元以节点 id 再核对一次"（3.4.3、3.6.2）。
4. **推后的改写的接法**（4.11 的"推后的链接改写" :270）：写明 `UnitSpec.DeferParticipants`、`Unit.Participate` 与 `Step.Batch` 带的 `BatchRewrite`：写了的每页（"之前"、它自己的改动、`DecidedAll` 为它留下的事实，第 23 条）、原来的父节点的路径、删除了的节点、改写的页数的界线、第一个改名或移动的项；linking 的 `Moved` 照抄，`FirstMove` 留在 page（3.7.1、3.7.2）。
5. **页数的数法与一页的例外**（4.11 的"界线" :279）：最后的正文按（页、哈希）各数一次；"只有一页时不限"按不同的页数，只管最后的正文与撤销的两份正文，不管计划的解析（第 24 条）；多于一页时计划的解析也数进 `MaxUnitPages` 与 `MaxUnitBytes`，422 的文字说它们也算在里面、改名可以与之后的 `edit` 分开发；`ops` 为空是 `validation_failed`，多于 100 是 `page.batch_too_large`（3.4.4、3.18.4）。
6. **答复**（4.11 的"答复" :282）：正文操作的 `version` 是单元结束时的（推后的改写写过的是改写之后的）；之后被删除的页的正文操作答它合成的写的版本与那一项时的路径；`created` 是单元结束时的路径；每一项的 `changed` 是它的工具在那一步会答的，batch 的 `changed` 说它留下了改动没有（3.6.3、3.9.1）。
7. **搜索的实测**（4.11 的"搜索" :280）：积压的最坏加推后的改写写的页（约 2,200 个）；在任何 batch 的代码之前量 n = 0、100、1,200、2,200，只看相对于 n = 0 的条件（3.10.1）。
8. **后备的第 2、3 条与轮数的上限**（4.12 的"卡住时的三条后备" :290-295）：第 3 条留作防御，以构造的卡住的状态直接测 `fallback()`；(T2) 写进代码：轮数过了 |P₀|+|D₀|+|R₀|+|W₀|+2N+1，`schedule()` 答 `ErrRevertUnordered`（3.16.3、3.21.1）。
9. **临时名称**（4.12 的"临时名称" :297）：候选是后 8 位、全部 32 位、再以 `-2`、`-3`……接在 32 位之后；取第一个此刻空着、也不是 S\* 在那里的任何一个键的；名称取节点在计划开始时的名称（3.17）。
10. **不可分的段**（4.12 的"后台撤销"第 2 条 :304）："名称环的段只有几步"改为三种段：一个临时的步到那个节点最后的放回之间的全部步（重叠的合成一段；段里的恢复只展开到段里后面的步要的成员）、一页的恢复与它的写回、一个删除与它带走的写回；一段多于一个节点、超过单元的历史或链接时才是 `revert_too_large`（3.22.2）。
11. **每批的核对**（4.12 的"后台撤销"第 4 条 :306；第 7 节 P3 一行 :430；"移交的落实"的 M8/P4 一行 :447）：改为"L，再按次序核对前提"：步取任务开始时的计划；L 照 M8 对这一批的节点做（成员 0（一组的第一个成员）在这一批里时读全组的条目与版本，恢复的页的写回照它在回收站里的正文核对）；之后以调度的前提函数按次序核对每一步，不建 S\*：`canDelete` 比那一步那一刻的子树与它记下的节点，多出的节点里父节点在记下的节点之中的（新加的每一部分的顶，变更集自己的也是）各是 `children_added`、以它为名（同 M8 的 `before()` 以加进来的顶为名）；写回写在刚核对过的版本号上（3.24）。
12. **一个节点不受批的界线**（4.12 的"后台撤销"第 2 条 :304）：`RunBatch` 的历史与链接的界线对只碰一个节点的一批不适用；只有这样的一段才"不能再分而总写得下"，减半照第 28 条（3.23）。
13. **`RevertWrites`**（4.12 的"先恢复、再写回" :298 与"撤销的撤销" :299）：写回变更集里所有带 `Base` 的版本行，不再看条目；`readReverted` 的条件随之；"已经相同"只对活着的页比（3.18）。
14. **行号**（审查 C-N1）：4.11 :262 的 `store.go:286-297` 是 `:396-407`；4.12 :289 的 `round()` 是 `revert.go:811-861`；:296 的 `store.go:270-284` 是 `MoveNode`（`:367`）、`unit_revert.go:124-150` 是 `:125-157`；:298 的 `revert.go:278-283` 是 `:280-286`、`unit_revert.go:320-326` 是 `:475`、`revert_changeset.go:262-272` 是 `:259-275`、`unit_revert.go:204-237` 是 `:197-244`；:299 的 `revert.go:479-497` 是约 `:476-500`；:305 的 `unit_revert.go:367-372` 是 `stepKey`（`:376`）、`:548-567` 是 `:555-574`；:307 的撤销的性质测试是 `revert_property_test.go:81-190`（操作）与 `:238-290`（测试）。
15. **4.14 的 `batch` 一行**（:356）：写成"1–100 项；字符串合计 6 MiB，正文与 `new_string` 合计 5 MiB；条目 `MaxUnitNodes`（第一遍结束时）；读正文的页以大小合计 `MaxUnitBytes` + 6 MiB（任何正文的读之前；只有一页时不限；`write` 的页只读头、不数）；计划自己的解析 2 × `MaxContentBytes`（按不同的（页、哈希）；与最后的正文共用一个等待）；页、字节、链接照 `MaxUnit*`，按（页、哈希）数（只有一页时最后的正文不限）；改写 `MaxBatchRewrites`（实测）；重新计划 1 次"（3.8）。撤销的步一行（:359）不改。
16. **"写给后面的 M"的 M12（:465）与第 10 节的风险（:616-618）**：性能一条的"batch 计划里的改写"改为"batch 的计划每次读整棵树（同 `list`）、读正文的页以大小拒绝（14 MiB）、计划自己的解析至多 10 MiB；最坏的 batch 解析至多 20 MiB，正常约 5 秒、病态约 10 秒，预算被别人占着时加一个合计的等待 2 秒，约 7 与 12 秒；重新计划一次时约 6 与 14 秒、被占着时约 8 与 16 秒（P3 3.8）；`MaxBatchRewrites` 的实测值"；风险的"batch 的'等于分开调用'"一行改为"(a)–(e)"与"与分开调用的不同照 4.11 的一条规则写进说明"；"多轮调度与临时名称的正确性"一行加轮数的上限；加一行：一段放不进一批的后台撤销以 `revert_too_large` 停下（3.23）；加一行：病态的正文、重新计划与预算被占着都碰上的 batch 约 16 秒，过了调用的 15 秒的期限，接受：单元回滚、什么都不写（3.8）。
17. **MC5**（第 9 节 :550）：步与断言照本文第 4 节写细，00 号文档不改。
18. **第 12 节**：P3 一行"进行中（设计已审查）"，带本文与审查记录的链接；各 Step 合并时写上。第 13 节加这次设计的一行。
19. **与分开调用的不同：一条规则，只写在 4.11**（4.11 的"与分开调用不同的两处" :277 与"交给它的改动" :271；第 1 节第 5 条 :20、第 3 节第 7 条 :61、第 7 节 P3 一行 :430、"移交的落实"的 M6/P4 一行 :444、"总体设计的修订"的 4.5 :477、第 9 节 :520；总体设计 4.5 第 4 条）：4.11 写"链接跟着 batch 的改名与移动（按节点合并），从它们在 batch 之前的去向跟起；batch 写了的页里的，从这一页在这些改名、移动之前最后一次写时的去向跟起（那时的树，batch 之前的别名）。其间别的项对它们的去向做的（新建、改名、移动或删除一页）看不见，batch 改的别名也看不见"，加锁的码与写明而不测的 (d)、(f)；别处只指向 4.11（工具的说明与内置的说明各缩成一句，第 32 条）。"交给它的改动"加这个 batch 新建、之后又被改名或移动的节点（3.7.1、3.12、3.14.4）。
20. **4.11 末条的性质**（:283）：例外加"或被这个 batch 写进的别名抢走的"；写了的页的"那一刻解析到的节点"由测试自己的模型给出；只对 `update_links` 为真的改名、移动成立；batch 删除了的来源页不比（3.14.6）。
21. **计划给新建的页的 id；单元里的核对**（4.11 的"两段"第 2 条 :265）：用例在计划之前为每个要新建的节点做 UUIDv7 的 id（重新计划时再做），单元以它建页、重做也核对；每个正文操作在锁下读到的版本号与计划的相同，不带 `base_version`、字节相同的 `write` 也是正文操作；之后 `WriteContent` 的 `ErrRevisionMismatch` 是缺陷；计划的正文与它的版本号一条语句读（3.4.2、3.5.2、3.6.2）。
22. **删除了的页的行与别名**（4.11 的"跳过" :273）：参与者在 `resolutions` 之前去掉单元删除的来源页的行；`resolutions` 把被删除的页（参与者里是 `Gone`，计划里是那一项及以前删除的）的别名行当作没有；别的缺页照旧是内部错误；"之前"的目标已不在的链接跳过，被这个 batch 删除的记 info、被别人删除的记 warn（3.7.2）。
23. **写了的页照它自己的改动**（4.11 的"'之前'" :272 与"锁下再核对" :275）：写了的页的"之前"是它最后一个正文操作时的去向（带 `Ambiguous`）；参与者照它那之后自己的改动改写：按节点合并，取那时与单元之后都活着的、或那之后新建的节点，那之后有一个 `update_links` 为假的节点不在；它们改名、移动的节点之下的节点不列，参与者对每页经 `spread` 读这些节点的子树与它们的别名键，一个 batch 每个节点读一次、存一份（内存随子树，不随写了的页数）；链接取自 `DecidedAll` 留下的事实；不论索引碰不碰得到、合并的改动什么都不搬时也做。"锁下再核对"改为：推后的改写在锁下照每页的"之前"改写它碰到的链接，计划在更早的项选的链接的文字锁下不再核对（3.7.1、3.7.2）。
24. **计划自己的解析的界线**（4.11 的"两段"第 1 条 :264 与"界线" :279）：`MaxBatchParseBytes` = 2 × `MaxContentBytes`（10 MiB），按不同的（页、哈希）；会过界的那次解析不做，答那一项的 422；只有一页时也限；事实按（页、哈希）留到请求结束；计划的解析与最后的正文的 `DecidedAll` 共用一个合计的等待（2 秒；会话是一次请求的，跨过重新计划）；每次取预算之前放还 batch 持着的事实的份额，事实留着、不计数（计划约 1 MiB、单元约 0.8 MiB；重新计划而正文都被别人改了时合计至多约 3.6 MiB）。最坏的 batch 解析至多 20 MiB：正常约 5 秒、病态约 10 秒，预算被别人占着时约 7 与 12 秒；重新计划一次再解析至多 15 MiB，约 6 与 14 秒、被占着时约 8 与 16 秒，最后这一种过了 15 秒的期限，接受（3.4.4、3.4.5、3.8）。
25. **读的界线**（4.11 的"两段"第 1 条 :264 与"界线" :279）：页按它在 batch 里的第一个正文操作分三种：读正文的（`edit`）、只读头的（`write`，带不带 `base_version`：版本号、大小与哈希，`PageStates` 加 `content_hash`）、都不读的（新建的）；读正文的页的大小按项的次序累加，过了 `MaxBatchReadBytes`（14 MiB）的那一项答 422，只有一页时不限；带过期的令牌、字节相同的 `write` 是未改动，同单个 `write`（3.4.4）。
26. **`Recased` 是集合**（4.11 的"交给它的改动" :271）：每个只改大小写的改名都照它自己的单元那样改写（3.7.2）。
27. **回到原处的条目**（4.11 的"合并的语义" :269）：单元结束时删去前后相同的条目，写了正文的节点留着它的改动；变更集因此空了时一起删去，batch 答 `changed: false`、没有 `changeset_id`（3.5.4）。
28. **减半**（4.12 的"后台撤销"第 2 条 :304）：`to = max(第一段的末尾, 不大于 from + (to-from)/2 的最后一个段的边界)`，`to` 已是第一段的末尾（而它不只一个节点）才以 `revert_too_large` 停下（3.23）。
29. **任务的紧凑的计划**（4.12 的"后台撤销"第 4 条 :306）：任务从开始时的计划只留紧凑的步，`revertPlan.groups`（`GroupNode`）照今天留着；恢复的组、时刻与状态取这一批读的（3.22.1）。
30. **删除照那一刻的子树**（4.12 的"后台撤销"第 1 条 :303）：删除的步、`JobStep.Nodes`、开始时的 `locked()` 与这一批读的节点都是它在调度里那一刻的子树（3.22.1、3.24）。
31. **一组在一批里一次恢复**（4.12 的"后台撤销"第 1 条 :303）：在这一组在这一批里的第一步以一次 `bringBack` 恢复这一批里它的全部成员，最后一个成员之后标一次边界（3.24）。
32. **第 11 节设计的取舍第 13 条**（:704）："batch 与分开调用不同的两处……照 4.11 写明"改为指向 4.11 的那一条规则，其余不变；主持者的决定第 4 条（:640）不改。
33. **推后的改写的数、锁与错误的次序**（4.11 的"界线与锁" :276）：改写的页数在 `reached` 与索引的过滤之后、`refuseLocked` 与任何解析之前数，过了答 422；然后 `linking.pages_locked`（文字说"the batch's renames and moves"）；然后 503；三者的 `op` 都是第一个改名或移动的项；`MaxPages` 为 0 是不限。`MaxBatchRewrites` 照 M8 的规则以满负载的页实测：本机约 4 秒之内的最大一档，守卫断言三倍、至多 12 秒，预计 200 一档（3.7.2、3.8.2）。
34. **M8 的测试不改的例外**（4.12 的"按依赖的调度" :289）："M8 的测试不改"加：钉住 M9 的形状答 `ErrRevertUnordered` 的三处照 P3 3.21.1 改期望（`page/domain/revert_test.go:342-347`、`:864-886`，`page/app/revert_test.go:456-464`）。
35. **4.12 的测试**（:307）："恢复到另一组之下"改为以 SQL 种下；同一毫秒的两个同名附件以 SQL 种下；后台撤销的测试照 P3 3.26：名称环在每个批的边界上读树、两批之间别人的写（含一个后面的批的写回，与把一页改成别的字节再改回的两次写）、一段过了一批、大的文件夹（一组在一批里一次恢复）。
36. **读整页的 `write`、锁的预检与重新计划**（4.11 的"两段"第 1 条 :264、第 2 条 :265；第 11 节主持者的决定第 48、50 条）：第一个正文操作是 `write` 而之前的改名、移动经索引碰到它的链接的页也读整页，照那些改动改写之后的正文判"相同的字节"与 `changed`，同分开调用，算进读的界线（S1 审查 A-I1 = B-M2、A 的 N3，3.4.4）；锁的预检加上每个删除的子树里的页，按项的次序答第一个写或删除被编辑的页的项（S1 审查 A-M1，3.4.5）；计划读了树之后、读头或正文之前页被删除，也重新计划一次（S1 审查 A-M2，3.6.2）。
37. **交给参与者的与"之前"的路径**（4.11 的"推后的链接改写" :270、"交给它的改动" :271、"路径" :274；第 11 节主持者的决定第 45、49 条）：另交冻住的节点与它们合并的改名和移动、每个被改名或移动的节点之后新建在它下面的节点；`update_links` 为假的节点的链接在每一页里都不改，不论它的祖先怎样改名、移动（S1 审查 B-M1，3.7.2）；它们之下被跟随的节点的"之前"的路径取它们之前的名称与原来的父节点（S1 审查 B1-N1，3.7.3）；batch 在一个节点最后一次改名、移动之后新建在它下面的节点不进它的触及，新建只重新解析（S1 审查 B1-M1，3.7.2）。
38. **推后的改写的界线与锁**（4.11 的"界线与锁" :276；第 11 节主持者的决定第 46、47 条）：`MaxBatchRewrites` = 400，写了的页只在它自己的改动碰到它的链接时才数、每页算 2；`MaxBatchRewriteLinks` = 120,000；`MaxBatchRewriteBytes` = 34,406,400；三个都在读树与任何解析之前，次序是页数、链接、字节；逐页改写时一次解析在改写过的页留下的份额旁放不下答 422；单个 `move` 不受它们限，链接密与页重时的同样的风险与一个共用的界线交 M12（S1 审查 B-I1、B-M3、B1-M2、B1-M3、B1-N3、B2-M1、B3-M1，复核 A 的 F2-2、F2-3；3.7.2、3.8、3.8.2）。
39. **一条规则里锁的码**（4.11 的"与分开调用的不同：一条规则" :277；第 11 节主持者的决定第 50 条）：batch 写或删除的页（删除的子树里的也算）正被编辑时，在项的次序里第一个写或删除它的项答 `page.locked`；只被改名、移动的改写碰到的页在单元里才查到，答第一个改名或移动的项的 `linking.pages_locked`；`update_links` 按节点算，为假的节点的链接整个不改写，七种有测试（S1 审查 A-M1、B-M1，3.4.5、3.14.4）。
40. **界线**（4.11 的"界线" :279；第 11 节主持者的决定第 46–48 条）：不止一次移到别的父节点下时移动带着走的节点合计至多 10,000，在那次移动自己的检查之后（S1 审查 A-I2、A 的 N2、N4，3.4.3）；读的页里有之前的改名、移动碰到它的链接的 `write`，过界的 422 说为什么（S1 审查 A-I1、A 的 N3，3.4.4）；改写的页、链接与页的字节的界线与改写过的页留下的解析额度（第 38 条）。
41. **搜索与答复**（4.11 的"搜索" :280、"答复" :282；第 11 节主持者的决定第 44 条）：第 0 步以最新的页的积压重量，照 3.10.1 的规则改为批量入队（`UnitSpec.BulkSearch`），答复与搜索在建好之前答 `building`（S1 审查 C-I2，3.9.1、3.10.1）；第 7 条随之。
42. **4.11 末条的性质**（"索引的测试" :283）：经被这个 batch 的写拿掉的别名解析到的链接也只重新解析，同新建与删除（实施的偏差 19，3.14.6）。
43. **4.14 的 `batch` 一行与 4.15 的 `batch` 的次序**（:358、:370）：照建成的写：移动带着走的节点、读整页的 `write`、推后的改写的三个界线与改写过的页留下的解析额度的检查，锁的预检的次序，页被删除也重新计划一次，逐页改写时的 422 与 503（3.8、3.9.3）；第 2、15 条随之。
44. **"移交的落实"的 P3 各行的 S1 部分**（M4/P2 :444、M5 第 2 条 :445、M6/P4 第 1、3 条 :446、M8/P5 第 2 条 :450）：照 S1 建成的写上落实（第 7 节"文档"的 00 号文档一条）；M8/P3 与 M8/P4 第 2 条是 S2、S3 的。
45. **"写给后面的 M"的 M12、第 9 节与第 10 节**（:467、:522、:619 之后加一行；第 11 节主持者的决定第 47、52 条）：M12 的性能一条加推后的改写的三个界线、界线之内仍慢的形状（很重的页、许多次改名之间的带链接的页）、单个 `move` 的同样的风险与共用的界线，P3 的 S1 也写进性能移交第 18 项；第 9 节的界线的测试加移动带着走的节点、推后的改写的三个界线与解析额度；第 10 节加这些慢的形状的一行（接受，3.8）。
46. **第 11 节主持者的决定第 44–55 条**（:692 之后）：S1 的实施与审查里主持者定下的（[S1 的审查记录](reviews/P3-S1-batch-review.md)的"主持者的决定"，第 2.2 节）。
47. **状态与第 12、13 节**（:7、:716、:735 之后）：状态"进行中（P1、P2 完成，P3 的 S1 已合并）"；第 12 节 P3 一行"进行中（S1 已合并）"、S1 的审查记录与合并 `f19d0b6f`；第 13 节加 S1 合并的一行。
48. **调度与后备**（4.12 的"按依赖的调度" :289 与"卡住时的三条后备" :290-295；第 11 节主持者的决定第 57、62 条）：`schedule()` 建成（`page/domain/revert_schedule.go`），`round()` 删去；钉住 M9 的形状的三处里用例的那一处改为存着的缺陷"比 `MaxDepth` 深的页被改名"（S2 的实施的偏离 2）；第 2 条后备只取待放回的、没有临时移动过的节点，先取 id 最小的（S2 审查 A-N2、C-N1）；"临时改名两次"与"名称取计划开始时的"在合法的状态里到不了，各有构造的测试（偏离 8、审查 C-N5）；轮数的上限从下面钉住（审查 A-M1、修复核对 A 的 N1，3.16.4）。
49. **临时名称**（4.12 的"临时名称" :297；第 11 节第 59、65 条）：扩展名长到连主干的第一个字符都放不下时照页的名称，M7 的 `Numbered` 一并改（S2 审查 A-N1，3.17）；守卫只凭名称认得临时的步，M10 可以在 `Step` 上带步的种类或一个标记（审查 B-N4，3.17、第 8 节）。
50. **先恢复、再写回**（4.12 的"先恢复、再写回" :298；第 11 节第 58 条）：守卫看到恢复的一步的事实，链接索引、搜索与事件看到单元合并的改动、它的事实是写回的（S2 审查 B-N2，3.18.3）；一页的两份正文在最小的解析预算下先还掉本次调用留着的份额（审查 B-M1，3.18.4）。
51. **4.12 的测试**（:309；第 11 节第 56 条）："恢复到另一组之下"以 SQL 种下，断言 M8 的 L 的拒绝（409，什么都不写）；domain 的那一条是包里构造的计划者（S2 的实施的偏离 1，3.21.1、3.21.2）。
52. **4.2 的 `changesets` 一行、4.14 的撤销的步与 4.15 的日志**（:96、:361、:373；第 11 节第 66 条）：00045 的补值的用时与它持着的锁（3.20.1）；`Steps()` 数临时的步、写回不算，S2 的任务拒绝有临时的步的计划（第 0 节、3.19）；"changeset reverted" 那一行的 `aside`（3.21.3）。
53. **4.11 的"事件"**（:281；第 11 节第 60 条）：写过的页只在单元之后活着时列出（S2 审查 B-N3，3.10.2）。
54. **"移交的落实"的 M8/P3 一行、"写给后面的 M"与第 10 节**（:448、:464、:465、:467、:622-623；第 11 节第 65、66 条）：M8/P3 一行照 S2 建成的写上落实（M8/P4 第 2 条是 S3 的）；M10 的守卫只凭名称认得临时的步，M11 的回滚用 `schedule()`，M12 的 00045 的用时与锁；风险的"多轮调度与临时名称的正确性"一行加上限的钉住与构造的测试，加 00045 的锁一行（第 8 节）。
55. **第 11 节主持者的决定第 56–66 条**（:708 之后）：S2 的实施与审查里主持者定下的（[S2 的审查记录](reviews/P3-S2-revert-review.md)的"主持者的决定"，第 2.2 节）。
56. **状态与第 12、13 节**（:7、:732、:752 之后）：状态"进行中（P1、P2 完成，P3 的 S1、S2 已合并）"；第 12 节 P3 一行"进行中（S1、S2 已合并）"、S2 的审查记录与合并 `43b03b36`；第 13 节加 S2 合并的一行。

### 2.2 需要负责人确认

无。

- 负责人的四个决定照做：agent 只读历史，`batch` 的变更集在网页上整组撤销（S2、S3 兑现 5.3）；锁拒绝、不能接管（batch 里的写照 P2 的锁的规则，3.9）；`AGENTS` 与说明不变（只加 `batch` 的一行）；MCP 不开后台任务。
- 2.1 的各条是实现的写定、代码查出的更正或 00 号文档里的说法写细，由主持者定（负责人可以改判）。第 10 条改的是 4.12 的一个论断（段只有几步），主持者的决定第 10 条（"后台撤销里放不进一批的不可分的段以 `revert_too_large` 停下"）照旧成立，只是这样的段比 4.12 说的多两种来源（3.23 写明，并进第 10 节的风险）。第 19、32 条把"不同的两处"改成一条规则：它细化的是主持者的决定第 4 条（B-Q1）与第 11 节设计的取舍第 13 条，两者都是主持者的（修订的简报），第 4 条照旧成立；不在计划里为每次移动另算"之前"照旧。第 8 条把第 3 条后备留作防御，不改负责人批准的 4.12；去掉它要改 4.12，是负责人的事，本文不提。S1 与 S2、S2 与 S3 之间的临时的拒绝（第 0 节）不发版，是主持者一级的。
- S1 的实施与审查里主持者定下的，同样负责人可以改判，写进 00 号文档第 11 节（主持者的决定第 44–55 条）与 [S1 的审查记录](reviews/P3-S1-batch-review.md)的"主持者的决定"：搜索批量入队、答复带 `building`；冻住的节点的链接从不改写；改写的页数的数法（写了的页算 2）；三个新的界线（移动带着走的节点、碰到的链接、改写的页的字节）与改写过的页留下的解析额度的检查；之前的改名、移动碰到它的页的 `write` 读整页；新建的节点不在跟随的操作的触及里；改名的改写碰到被编辑的页只在单元里查到；计划一次只留一棵树的读；许多次改名之间的带链接的页、界线之内的重页与单个链接密的移动 v0.1 接受、交 M12；M8 的清理测试的摆动在 S1 里改；成本那一份的时限、batch 的守卫在 CI 上的余量与组合根第一份现在不动；River 重选时的疑点记下、不改。
- S2 的实施与审查里主持者定下的，同样负责人可以改判，写进 00 号文档第 11 节（主持者的决定第 56–66 条）与 [S2 的审查记录](reviews/P3-S2-revert-review.md)的"主持者的决定"：上下相套的回收站项照 M8 的 L 拒绝；(T1) 的判断留着、以构造的测试为反向对照；一页的两份正文在最小的解析预算下先还掉本次调用留着的份额；空的主干照页的名称（M7 的 `Numbered` 一并改）；页面事件只列单元之后活着的写过的页；后台撤销的临时的拒绝说出它是什么；轮数的上限从下面钉住；测试加强的几处；CI（两份正文的测试用生产的期限，第一份过线时把 `TestEvery[M-R]` 挪到第三份）；给 M10 的一条（守卫只凭名称认得临时的步）；A 的存疑、00045 的锁与成本、守卫两份的时限记下、不改。

## 3. 服务端

### 3.1 S1 的文件

| 包 | 文件 | 新 / 改 | 内容 |
|---|---|---|---|
| page/domain | `args.go` | 新 | `Args`、`OpArgs`、`Args.Field`（3.2） |
| page/domain | `steps.go`、`edit.go`、`errors.go`、`path.go` | 改 | 规则收 `Args`：`CreationOf`、`MoveKinds`、`DeletionOf`（`Recursive.Field`）、`CheckEdit`、`Replace`；新的 `PagePath`、`ParentChainOf`、`RelocationOf`、`RenameTitle`（从用例与单元抽出，3.2、3.4.2）；`ErrBatchTooLarge` 是 422 `page.batch_too_large`，后台减半的哨兵改名 `ErrJobBatchTooLarge`；`OpFailed`、`BatchTooLarge`、`BatchChanged` |
| page/domain | `batch.go`、`batch_tree.go`、`batch_plan.go`、`batch_contents.go` | 新 | `BatchArgs`、`OpKind`、`BatchStep`、`CheckBatch`（3.3）；`BatchPlan`、`PlannedStep`、`PlanBatch`、`(*BatchPlan).Contents`、`ReadWhole`、`PageText`、`PageHead`、`LinkRewriter`、`LinksBefore`、`Lead`、模拟的树 `batchTree`（3.4）；`TreeAt`、`MovesBetween`、`FrozenBetween`、`FrozenMovesBetween`、`CreatedAfterMoves`（3.7）；`MaxBatchReadBytes`、`MaxBatchParseBytes`、`MaxBatchRewrites`、`WrittenRewriteWeight`、`MaxBatchRewriteLinks`、`MaxBatchRewriteBytes`（3.8、3.8.2）。计划分在四个文件里（实施的偏差 4） |
| page/domain | `actions.go` | 改 | `ActionBatch = "page.batch"` 进 `Actions()` |
| page/domain | `revert.go` | 改 | S1 临时的一条：C 删除的页有带 `Base` 的版本行时 `ErrRevertUnordered`（第 0 节；S2 换掉） |
| page/app | `apply_batch.go` | 新 | 用例 `ApplyBatch`（3.6） |
| page/app | `unit.go`、`extension.go` | 改 | `UnitSpec.DeferParticipants`、`Unit.Participate`、`Unit.DropUnchanged`、`Step.Batch`（`BatchRewrite`）、`recordItems` 的注释（3.5.3、3.5.4、3.7.1）；`UnitSpec.BulkSearch` 与 `Outcome.Building`（3.10.1，S1 审查 C-I2） |
| page/app | `unit_create.go` | 改 | `PageDraft.ID`：给了就以它建页，没给时照旧 `uuid.NewV7()`（`unit_create.go:63`，3.4.2） |
| page/app | `content.go` | 改 | `ContentParser.Planned`：计划的解析的会话：按（页、哈希）留事实；它的解析与最后的正文的 `DecidedAll` 共用一个合计的等待；每次取预算之前先放还它持着的事实的份额，事实留着、不计数（3.4.5） |
| page/app | `ports.go` | 改 | 端口 `BatchLinks`（3.7.4）；`PathStore.LinePaths`（原来的父节点的路径，带 id 与键，3.6.2）；`Changesets.DropUnchanged`（3.5.4）；`PageState.Hash`（3.4.4） |
| page/adapter/postgres | `store.go`、`queries/changesets.sql`、`queries/contents.sql`、`paths_test.go` | 改 | `LinePaths`：同 `PathsOf` 读 `LinkTargetsByIDs`（`store.go:140-143`，一条语句），答各段而不是字符串；`DropUnchanged`：删去前后相同的条目、再删去既无条目又无版本行的变更集（3.5.4）；`PageStates` 多读 `content_hash`、填进 `PageState.Hash`（`contents.sql:40-44`、`store.go:194`，3.4.4）；`TestPageStates` 在同一个提交里改为逐字段比、断言哈希（`paths_test.go:94-113`：带 `[]byte` 的 `PageState` 不能以 `!=` 比，3.14.1，审查 NC2-M1） |
| page/app | `write_path.go`、`edit_page.go`、`relocate_path.go`、`delete_path.go`、`unit_relocate.go`、`unit_rename.go` | 改 | 传 `domain.Args{}`；`holdsNoNodes`、`parentOf` 的判断与附件路径的拒绝挪进 domain；`Relocate` 调 `RelocationOf` |
| page/app | `jobs.go`、`jobs_test.go` | 改 | 哨兵改名（3.9.2，单独的一个提交，在新的哨兵之前） |
| page（根） | `module.go`、`tools.go`、`tree_writes.go` | 改 | `Deps.BatchLinks`；`ApplyBatch` 交给 `adapter/mcp`；再导出的哨兵改名 |
| page/adapter/mcp | `batch.go` | 新 | 工具 `batch`、`batchIn`、`batchAnswer`、`atOp`（3.9、3.12） |
| page/adapter/mcp | `tools.go`、`instructions.go`、`text.go` | 改 | 第 11 个工具、它的说明的常量、内置的说明的一行 |
| linking/app | `batch_links.go` | 新 | `BatchLinks`：计划里的解析与改写（3.7.4） |
| linking/app | `rewrite.go`、`resolver.go`、`ports.go` | 改 | 参与者对 batch 写的页的 (a)–(e)（链接取自 batch 留下的事实；`spread` 也展开它们自己的改动移动的节点，每个节点的子树与它的别名键一个 batch 读一次）、跳过、数改写的页、链接与页的字节（`PageSizes`），逐页改写时改写过的页留下的份额的检查（`keptShares`、`RewriteParser.Takes`）（3.7.2）；`resolutions` 不看删除了的页的别名行（3.7.2、3.7.4）；`Moved` 的字段 |
| linking/domain | `change.go`、`rewrite.go` | 改 | `PathBefore` 从根撤回路径上的每个改动（3.7.3）；`Recased` 是一个集合（3.7.2） |
| linking（根） | `module.go` | 改 | `NewBatchLinks(pool)` |
| linking/adapter/markdown | `facts.go` | 改 | `Takes` 与 `Parsed.Kept`（改写过的页留下的份额，修正的第 6 轮，3.7.2） |
| platform/markdown | `budget.go` | 改 | `Budget.Size`、`Budget.Takes`、`Hold.Held`（修正的第 6 轮，3.7.2） |
| access/domain | `rules.go` | 改 | `"page.batch": {Level: LevelNotebook, Notebook: writers()}` |
| bootstrap | `deps.go`、`registrants.go`、`linking.go`、`transfer.go` | 改 | `pageDeps` 设 `Deps.BatchLinks`（`bootstrap/deps.go:141-171`，`wire.go:155` 与基准都经它）；`BatchLinks` 的适配（page 的类型与 linking 的互换，同参与者的适配，`linking.go`）；参与者的适配抄 `Step.Batch`，`TooManyRewrites` 译成 `page.batch_too_large`，`PageSizes` 接页模块的 `ContentSizes`；哨兵改名（`transfer.go:231-232`） |
| bootstrap（测试） | `mcp_batch_test.go`、`mcp_batch_links_test.go`、`mcp_batch_merge_test.go`、`links_rebuild_batch_test.go`、`interleavings_batch_test.go`、`batch_cost_test.go`（`//go:build !race`）与几个已有的文件（`background_cost_test.go:303`、`background_test.go:269` 的哨兵改名；M8 的 `interleavings_history_test.go` 的两种情形等启动时的清理，修正的第 4 轮） | 新、改 | 3.14 |
| api/mcp | `tools.json` | 改 | 两个入口各加 `batch`（3.12.2） |
| e2e | `stories/mcp/mc5-batch-revert.spec.ts`、`fixtures/assert/page.ts` | 新、改 | MC5 (MCP)（第 4 节）；`fixtures/mcp.ts` 不改：4.1 写明 `resultOf` 与 `errorOf` 不用改（实施的偏差 22） |
| 根 | `Makefile`、`.github/workflows/ci.yml` | 改 | 守卫那一份加 batch 的一行（五个测试：页数、链接与字节的界线，写了的页的最后的大小，长路径；3.8.2、6.5）；`.github/workflows/ci.yml` 只改守卫那一份的时限，由 25 分钟提到 30 分钟（修正的第 7 轮：第 6 轮时作业到 12:47，照约两倍的规则；复核 C 的 N3-f4）；它跑 `make "test-go-${{ matrix.leg }}"`（`ci.yml:95`），新的几行在 `Makefile` 里 |

没有迁移（S1 的表与列都已在：条目、版本行、`changesets.message`）；没有新的配置。

### 3.2 字段名的一种写法：`domain.Args`

S1 审查 B-N3 与主持者的决定第 33 条要 P3 定一种写法、一条规则一份。定为**每条规则收它报的字段名**，以一个小的值传：

```go
// Args is where a write's arguments are in its call, as its 422s name
// them (M9/P3 design 3.2): the zero value for a tool's own call, whose
// fields are its arguments' names ("path", "to"); OpArgs(i) for the op i
// of a batch, "ops[i].path". Each rule of a step names its fields through
// it, so that a batch's op answers the rule's own 422 at its op.
type Args struct{ prefix string }

// OpArgs is the arguments of a batch's op i, from 0.
func OpArgs(i int) Args { return Args{prefix: "ops[" + strconv.Itoa(i) + "]."} }

// Field is the field of the argument name: "to", or "ops[3].to".
func (a Args) Field(name string) string { return a.prefix + name }
```

P2 的规则照它改（今天收字符串的照收字符串，调用的传 `a.Field("…")`）：

| 规则 | 今天 | 改为 |
|---|---|---|
| `ParsePath`、`CheckContent`、`ParseVersionToken`、`MissingAncestors` | 收 `field string` | 不变，调用的传 `a.Field("path")` 一类 |
| `CreationOf(field, path, found)` | 收字段名 | `CreationOf(a Args, path, found)`：`a.Field("path")` |
| `MoveKinds(from, to)` | 写死 `"to"`（`steps.go:77-82`） | `MoveKinds(a Args, from, to)` |
| `DeletionOf(sub, recursive)` → `Recursive` | 写死 `"recursive"`（`errors.go:209-212`） | `DeletionOf(a Args, sub, recursive)`，`Recursive{Field, Pages, Assets}` |
| `CheckEdit(old, new)`、`Replace(content, old, new, all)`、`notInPage` | 写死 `"old_string"`、`"new_string"`（`edit.go:21-39`、`:87-92`、`:119`） | 第一个参数收 `a Args` |
| `write`、`edit` 拒绝附件的路径 | 用例里写死 `"path"`（`write_path.go:84-87`、`edit_page.go:83-86`） | domain 的 `PagePath(a Args, p Path, tool string) error`，两个用例与 batch 调它 |
| `move` 的父页链：路过附件 `holdsNoNodes`、缺的段 `PathNotFound{ParentOf: "to"}` | 用例里写死（`relocate_path.go:132-157`） | domain 的 `ParentChainOf(a Args, to Path, chain []Node, on *AttachmentOnPath) error`（`on` 是 `Walk` 答的路过附件的段，没有时为空）：第一个不成的段定答复，规则同今天的 `parentOf` |
| `Placement.Field`、`Unit.Relocate` 的名称（`nameOf`） | 收字段名（`unit_relocate.go:16-20`、`unit_rename.go:60-65`） | `RenameTitle(field string, n Node, name string)`（`nameOf` 挪进 domain），`Relocate` 照传 `Placement.Field`；它调的 `CheckTitle`、`CheckAssetRename` 与后者调的 `CheckAssetName` 照收字段名 |

`Args` 是值、不是接口：规则在 domain，零值就是今天的写法，所以四个用例的改动只是传 `domain.Args{}`，它们的表格测试照旧（字段名不变）。

**为什么不是"改写字段的函数"**：要改的不只是 `shared.FieldError.Field`：`Recursive` 与 `PathNotFound.ParentOf` 的字段写进 MCP 的文字（`page/adapter/mcp/text.go:35-38`、`:50-53`："`ops[3].to`'s parent"），`VersionMismatch`、`PathTaken` 的文字带路径而没有字段；一个事后改写的函数要认得每一种带字段的类型，新加一种就漏一种，漏了的在 `batch` 里答错字段、测试不一定看得见。

**它的检查**：`TestEveryRuleNamesTheFieldsItIsGiven`（`page/domain`）以 `OpArgs(7)` 调每一条规则的每一种拒绝（表的每一行是一种输入），断言错误的每个字段（`shared.Error.Fields`，与 `Recursive`、`PathNotFound` 的 `Field`、`ParentOf`）都以 `ops[7].` 起头，再以零值调同样的一行、断言字段与今天的相同。表的完整由另一个断言管：`page/domain` 的非测试文件里每个产生 422 的函数（以 `go/parser` 找 `NotAllowed(`、`shared.Invalid(`、`invalidPath(` 的调用与带字段的错误类型（`Recursive`、`PathNotFound`）的字面量所在的函数；在 `f1733403` 有 29 个，分在 `steps.go`、`edit.go`、`path.go`、`errors.go`、`content.go`、`version_token.go`、`changeset.go`、`node.go`、`name.go` 与别的工具的文件里）要么在表里出现过，要么在测试里两张明写的表之一里，各带理由：**不算的**：batch 碰不到的别的工具的规则：`ParseActor`、`clientFilterInvalid`（`log` 的过滤）、`NotATask`、`TaskWouldGo`（`toggle_task`）、`ParseGlob`（`glob`）、`CheckListLimit`（`list`）、`CheckWindow`、`columnOutOfRange`（`read`）、`ParseSearchQuery`、`SearchLimit`（`search`）、`CheckRevision`、`CheckVersionsArgs`（`versions` 与历史），与 batch 自己的、不属于某一项的 `message` 的 `CheckMessage`（`CheckBatch` 调它，字段照旧是 `message`）；**构造函数**：`NotAllowed`、`invalidPath`、`notInPage`、`(*Recursive).Unwrap`、`NotAVersion` 只造 422、不判定，经调它们的规则算（`CreationOf` 一类、`ParsePath`、`Replace`、`DeletionOf`、`ParseVersionToken` 的行就是它们的行）；三处都不在的失败（同 P2 的 `TestReplaceSearchesOnlyWithItsMatcher` 读源文件的做法，`page/domain/edit_test.go`）。29 个里，表里的规则 11 个、不算的 13 个、构造函数 5 个（审查 A3-N4）。`CheckTitle`、`CheckAssetRename` 与 `CheckAssetName` 在表里，各有一行：batch 的 `move` 改页或附件的名称时经 `RenameTitle` 碰到前两个（`unit_rename.go:59-65`），`CheckAssetRename` 又调 `CheckAssetName`（`page/domain/name.go:32-33`：把附件改名为 `x.md` 答它的 422）。反向对照：把 `MoveKinds` 改回写死 `"to"`，第一个断言失败；在 `steps.go` 加一个哪张表里都没有的拒绝，第二个断言失败。

### 3.3 `batch` 的参数与取值的检查

#### 3.3.1 输入

mcpserver 的 schema 只有 `type`、`properties`、`required`、`additionalProperties`、`items`、`description`（`mcpserver/schema.go:150-156`，没有 `oneOf`、`enum`），jsonschema-go 不从 Go 的类型推导出 `oneOf`；把四个工具的输入结构嵌进来也不行：同名的 `path` 被 jsonschema-go 与 `reflect.VisibleFields` 丢掉，`writeIn` 不是指针的 `Content` 变成必填，解码在 `DisallowUnknownFields` 下答 `json: unknown field "path"`，是 `internal_error`（`mcpserver/tool.go:194-201`；P3 的调查以 jsonschema-go v0.4.3 实测，`scratchpad/m9/p3/notes/union/main.go`）。所以 `ops` 的项是**一个平的对象**：`op` 必填，四个工具的参数的并都是可选的指针：

```go
// batchIn is batch's arguments (M9 design 4.11; M9/P3 design 3.3): ops,
// each one flat object, op naming the tool and its other members that
// tool's arguments. The schema has no oneOf, so an op's members are the
// use case's: a member its tool takes not given is 422 required at
// ops[i], one it does not take 422 not_allowed.
type batchIn struct {
	Ops     []batchOp `json:"ops" jsonschema:"The writes, applied in their order as one change: 1 to 100."`
	Message string    `json:"message" jsonschema:"Why the batch is made, shown with its change in the history: 1 to 4,096 bytes."`
}

// batchOp is one op of batch: op and the arguments of its tool.
type batchOp struct {
	Op          string  `json:"op" jsonschema:"The tool this op is: write, edit, move or delete. Its other members are that tool's arguments."`
	Path        *string `json:"path,omitempty" jsonschema:"write, edit and delete: the path, from the notebook's root, as that tool takes it."`
	Content     *string `json:"content,omitempty" jsonschema:"write: the page's whole content, as it will be stored."`
	BaseVersion *string `json:"base_version,omitempty" jsonschema:"write and edit: the page's version as read before the batch. None for a page the batch creates."`
	OldString   *string `json:"old_string,omitempty" jsonschema:"edit: the text to replace, byte for byte, which must occur exactly once unless replace_all is set."`
	NewString   *string `json:"new_string,omitempty" jsonschema:"edit: the text to put in its place."`
	ReplaceAll  *bool   `json:"replace_all,omitempty" jsonschema:"edit: replace every occurrence of old_string. Default false."`
	From        *string `json:"from,omitempty" jsonschema:"move: the path of the page or attachment that moves."`
	To          *string `json:"to,omitempty" jsonschema:"move: the full path it will have, never a page to move it into."`
	UpdateLinks *bool   `json:"update_links,omitempty" jsonschema:"move: rewrite the links to what moves so that they follow it. Default true."`
	Recursive   *bool   `json:"recursive,omitempty" jsonschema:"delete: delete a page with its children. Default false."`
}
```

`checkFields` 收它（指针、可选、没有 map 与接口，`mcpserver/schema.go:73-127`）；推导出的项是 `{"type": "object", "properties": {…}, "required": ["op"], "additionalProperties": false}`，同 mcpserver 的测试已预备的 `opIn` 的形状（`mcpserver/tools_test.go:125-179`）。`normalize` 去掉数组与它的项的 `null`（`schema.go:129-145`）：`ops: null` 与 `ops: [null]` 是 `bad_request`，可选的标量的 `null` 照"没有"收下。

#### 3.3.2 `CheckBatch`

```go
// BatchArgs is an op of a batch as the agent gave it: Op names the tool,
// the rest are the arguments, nil when left out.
type BatchArgs struct {
	Op                                      string
	Path, Content, Base, Old, New, From, To *string
	All, UpdateLinks, Recursive             *bool
}

// OpKind is the tool an op of a batch is (M9/P3 design 3.3.2). Its values
// are OpKind*, since the write guards' Operation has OpMove and OpDelete.
type OpKind string

// The tools a batch's op is one of.
const (
	OpKindWrite  OpKind = "write"
	OpKindEdit   OpKind = "edit"
	OpKindMove   OpKind = "move"
	OpKindDelete OpKind = "delete"
)

// BatchStep is an op checked: its kind, where its arguments are, and
// them parsed; Base nil without base_version.
type BatchStep struct {
	Kind                        OpKind
	Args                        Args
	Path, From, To              Path
	Content, Old, New           string
	Base                        *VersionToken
	All, UpdateLinks, Recursive bool
}

const (
	// MaxBatchOps is the most ops a batch takes (M9 design 4.11). Its
	// largest body, 100 edits of five arguments, is some 715 JSON values,
	// some 1,215 with every member of the flat op set: an eighth of
	// mcpserver.MaxBodyValues, which the edge counts before any decode
	// (M9/P2 design 3.2.6); a tool with lists stays far below it too.
	MaxBatchOps = 100
	// MaxBatchStringBytes bounds a batch's strings together, paths and
	// messages included; its contents and new_strings together are within
	// MaxContentBytes, as one write's content is (M9 design 4.11, 4.14).
	MaxBatchStringBytes = 6 << 20
)

// CheckBatch checks a batch's values, before any read (M9 design 4.15,
// the value 422s): its number of ops, none validation_failed on ops,
// past MaxBatchOps page.batch_too_large; its strings together and its
// contents and new_strings together, page.batch_too_large; its message
// (CheckMessage); then each op in their order: its tool, its members as
// the tool takes them, required and not_allowed at ops[i], and their
// values by the tool's own rules and order (ParsePath, PagePath,
// CheckContent, CheckEdit, ParseVersionToken, MoveKinds). The first that
// fails is the answer, an *OpFailed for an op's.
func CheckBatch(ops []BatchArgs, message string) ([]BatchStep, error)
```

每一项的次序：`op`（不是四个之一：`ops[i].op` 的 422 `not_allowed`，"is one of write, edit, move and delete"）→ 这个工具要的成员（缺：`ops[i].<名>` `required`，"is required for <工具>"）→ 它不收的成员（`ops[i].<名>` `not_allowed`，"is no argument of <工具>"）→ 工具自己的取值规则，次序同 P2 的用例：`write` 是路径、页的路径、正文、令牌（`write_path.go:80-99`）；`edit` 是路径、页的路径、两个串、令牌（`edit_page.go:79-97`）；`move` 是 `from`、`to`、类型（`relocate_path.go:60-70`）；`delete` 是路径（`delete_path.go:56`）。合计的字符串：每个字符串参数与 `message` 的字节数之和；正文与 `new_string` 的另一个和；两个和在逐项检查之前（取值的 422 的第一段，4.15），答 `page.batch_too_large`，文字给出是哪一个和、多少与上限。

### 3.4 `PlanBatch`：两遍

单元之前、不加锁：先判定 `page.batch`（4.15：读者在任何读之前得 403），再读这本笔记本的整棵树（`Nodes.ListNodes`，一条语句，`page/app/ports.go:154`；`list` 也这样读，10 万个节点约 70 ms，"写给后面的 M"的 M12 的性能一条）。

#### 3.4.1 模拟的树

`batchTree`（`page/domain/batch_tree.go`，不导出）：原来的节点（id、父、名、键、类型、次序值），每个父节点下按键的索引，加上一份按步记下的改动（新建、改名与移动、删除）。它回答 P2 的规则要的每一种读，与单元里的读同义：

| 读 | 单元里（锁下，真的库） | 模拟的树 |
|---|---|---|
| 按键找子节点 | `PathStore.RootByKey`、`ChildByKey`（`NodePaths.Walk`） | `child(parent, key)` |
| 子树 | `Nodes.Subtree`（逐层） | `subtree(id)`，同样逐层、每层按次序值与 id |
| 父页的一线 | `lineOf`（`unit_place.go:40-63`） | `line(parent)` |
| 兄弟里占着一个键的 | `holderOf(siblings, …)`（`unit_place.go:150-157`） | `holder(parent, key, self)` |

linking 的改写要的三种读（按键、按 id 给路径、子树）由 `(*BatchPlan).TreeAt(k)` 给出，是第 k 步之后的样子：原来的树加上第 k 步及以前的改动（3.7.4）。

#### 3.4.2 第一遍：树

```go
// BatchPlan is a batch planned (M9 design 4.11): each op on the tree its
// plan simulates, the first op that fails and why, what the unit
// changes and parses, and, after Contents, each page's content.
type BatchPlan struct {
	Steps  []PlannedStep
	Failed *OpFailed // the first op that fails, of either pass; nil for none
	Items  int       // the changeset's items: created, deleted (merged deletions' too), moved and renamed nodes
	tree   *batchTree
	…
}

// PlannedStep is an op on the simulated tree: the node it acts on (a
// write's page, created or there; an edit's; what a move moves; what a
// delete deletes) and its path after the op, as written; the nodes its
// path, or a move's from, walked, the last the node when it was there;
// the pages a move's to's parent's line walked; the pages a write creates,
// root first, each with the id the use case gave it; the subtree a delete
// takes with the deletions merged into it, and its pages and attachments;
// where it merged into a later delete; where a move puts its node; for a
// content op, the content op of the same page before it (-1 for none) and
// whether a rename or a move between them reaches the page's links
// (Contents rewrites them first); what the op's tool would answer as
// changed at its step, and an edit's replacements.
type PlannedStep struct {
	Node          uuid.UUID
	Path          string
	Found         []uuid.UUID
	Chain         []uuid.UUID
	Created       []CreatedPage
	Subtree       []uuid.UUID
	Pages, Assets int
	MergedInto    int // -1 for none
	To            Destination
	Previous      int
	Moved         bool
	Changed       bool
	Replacements  int
}

// Destination is where a batch's move puts its node: under ParentID, nil
// for the root, named Name. (Placement is a trash item's.)
type Destination struct {
	ParentID *uuid.UUID
	Name     string
}

// PlanBatch simulates steps in their order on nodes, the notebook's tree
// alive as read unlocked (M9 design 4.11, the tree's pass), as each tool
// would on the tree the steps before it leave: a step's paths found on it,
// P2's rules (CreationOf, ParentChainOf, MoveKinds, RenameTitle, RelocationOf,
// DeletionOf), a delete that takes a subtree an earlier delete took from
// merged into it, B1 and B3. ids are the use case's, a UUIDv7 a segment
// of each write's path: a page a write creates at its segment d takes
// ids[i][d], the id its unit creates it with. A write without
// base_version at a page there is left to Contents, which compares its
// base_version at a page there is left to Contents, which compares its
// content. It counts the items as it goes, and the nodes the moves to
// another parent carry, their subtrees, each move's again (M9/P3 review
// A-I2: the plan and the unit read and change each move's subtree again):
// past MaxUnitNodes either fails the op that passes it, the second for
// more moves than one, a lone move as P2's move. The first op that fails
// ends the pass, its error at its op in Failed.
func PlanBatch(nodes []Node, steps []BatchStep, ids [][]uuid.UUID) *BatchPlan
```

每一步的规则与码，照它的工具（P2 3.8 与 4.15），字段是 `ops[i].*`、文字由 3.9 的 `atOp` 加上第几步：

- **`write` 不带 `base_version`**：`Walk(path)` 在模拟的树上 → `CreationOf(a, path, found)`：路过附件、缺的祖先以 `.md` 结尾的 422，附件占着叶子的 409 `PathTaken{Asset}`，超过 `MaxDepth` 的 409 `too_deep`；叶子是页时记下它（第二遍以头的大小与哈希比字节：同样的字节不写，不同是 409 `PathTaken`；同样的字节仍是这一页的正文操作，3.5.2、3.6.2）；否则在模拟的树上依次建 `Titles`。
- **新建的页的 id 由计划给**：用例在 `PlanBatch` 之前为每项 `write` 的路径的每一段做一个 UUIDv7（`ids[i][d]`，至多 100 × 11 个），计划给第 i 项在第 d 段新建的页（祖先也是）这个 id，单元以它建页（`PageDraft.ID`；`unit_create.go:63` 只在没给时 `uuid.NewV7()`）。所以 `Written`、`Leads` 与 `Targets`（3.4.4、3.7）里的 id 就是单元建的页的 id，不需要对照；重做（3.6.2）也比新建的页的 id：锁下 `Walk` 找到的前缀之后要建的，与计划的 id、父节点与标题都相同。重新计划的那一次，用例另做一组 id（第一次的单元回滚了，它建的页不在了）。
- **`write` 带 `base_version`、`edit`**：`Find(path)`（页的路径找到附件、附件的路径找到页、什么都没有，同 `NodePaths.Find` 的 `PathNotFound` 与提示）→ 记下页；页是这个 batch 前面新建的、又带 `base_version`：`ops[i].base_version` 的 422 `not_allowed`（"refers to the notebook before the batch: ops[k] created this page"，4.11）；之后才是 `base_version` 的节点不是这一页的 409 `VersionMismatch{OtherNode}`（batch 之前的令牌不会指着它新建的页，409 在前时 422 永远答不出来，所以 422 在前，同 4.11 与 3.9.3 第 5 步）；版本号与"相同的字节"的比较在第二遍（3.4.4）。
- **`move`**：`Find(from)` → `ParentChainOf(a, to, Walk(to 的父页链))` → `RenameTitle(a.Field("to"), n, 末段)` → 同一父节点下只查名称（409 `PathTaken`，路径是 `to`；不读子树，同今天的 `renameTo`，`unit_relocate.go:43-45`：一个文件夹的改名不读它的一万个子孙）；换父节点时 `RelocationOf(sub, parent, line, holder)`（环 409、名称 409 `PathTaken`、层级 409，次序同 `Unit.Relocate`），之后数它带着走的节点（3.4.3）；原地同名不改（"未改动"）。
- **`delete`**：`Find(path)` → 模拟的子树 → `DeletionOf(a, sub, recursive)` → 规范化 → B1、B3（3.4.3）。

`RelocationOf` 与 `RenameTitle` 是从 `Unit.Relocate`、`nameOf` 抽出的纯函数（今天的检查与次序写在 `unit_relocate.go:39-68` 与 `unit_rename.go:60-65`）：单元调它们、计划调它们，一条规则一份；`RelocationOf` 只用于换父节点的移动，单元在同一父节点下照旧不调它、不读子树：

```go
// RelocationOf checks a node's move under another parent, as a unit and a
// batch's plan check it, in Move's order (M4/P1 design 3.6; M9/P2 design
// 3.5.1); a rename under its parent reads no subtree and is not one:
// parent in the node's subtree, page.cycle; holder, the sibling there
// that holds the new name's key, *PathTaken of its kind; the subtree's
// deepest page past MaxDepth under line, the parent's line from the root,
// page.too_deep.
func RelocationOf(sub Subtree, parent *uuid.UUID, line []Ancestor, holder *Node) error
```

#### 3.4.3 规范化、B1 与 B3

- **规范化**（第 11 节主持者的决定；4.11）：`delete` 第 j 项时，模拟的树上之前被删除的每个最上层 t（第 i 项删的），它被删时的父节点在第 j 项的子树里（今天活着的那部分），t 的删除并进 j：第 i 项的 `MergedInto = j`，j 的 `Subtree` 加上 t 的那一组。为什么安全：第 i 项与第 j 项之间的步不能依赖 t 已删除——用到 t 的位置（新建同名、移进那个名称）的节点在 j 的子树里，被 j 删除时由 B1 或 B3 拒绝；按路径找 t 的在计划里就答 404；所以规范化之后的重做与原来的次序写出同样的树。单元里第 i 项不做，第 j 项以整个子树删一次：不会有同一时刻的两次删除碰上 `DeleteNodes` 的 `stray`（`page/adapter/postgres/store.go:396-407`）。`recursive` 照计划：第 j 项的子树在计划里（它那一刻活着的）决定要不要 `recursive`，并进来的组不算（t 已删除，同分开调用）。
- **B1**：第 j 项的子树（含并进来的组）里有这个 batch 新建的节点：`ops[j]` 的 422 `not_allowed`，"ops[j] deletes a page that ops[k] created: a batch may not delete what it creates (the change would hold a node that is neither before nor after it); leave both out, or delete it in a later call"。
- **B3**：子树里有这个 batch 改过名或移动过的节点（含移进来的）：`ops[j]` 的 422，"…that ops[k] moved or renamed: …"。两条在计划里核对，第一个出错的项定答复；单元里每一步找到的节点 id 与删除的子树的 id 都等于计划的（3.6.2），计划的判断在锁下仍成立（2.1 第 3 条）。

**`Items`**：新建的节点、删除的子树里的节点（含并进来的组）、改过名或移动过的节点，各数一次；第一遍边走边数，过了 `MaxUnitNodes` 的那一项就是第一遍出错的项，答 422 `page.batch_too_large`（3.8）。**移动带着走的节点**同样边走边数（S1 审查 A-I2）：移到别的父节点下的每次移动，在它自己的检查（`RelocationOf`）之后加上它的子树（每次移动各算一次，来回移动同一个文件夹算两次，S1 审查 A 的 N2），不止一次这样的移动时合计过了 `MaxUnitNodes` 的那一项答 422（"N nodes the batch's moves carry, at most 10000: move each folder once, or in separate calls"），第一遍到此为止；那次移动是环时先答它的 409（S1 审查 A 的 N4）。只有一次时不限，同单个 `move`；同一父节点下的改名不带走什么。计划与单元都为每次移动重读、重改它的子树：50 次来回移动一个 10 万页的文件夹，单元持着笔记本的行 12 秒（S1 审查 A-I2）。它与读的界线（3.4.4）都在第一遍结束时、任何正文的读之前答，不再跑第二遍：第二遍在更早的项上会出的错（例如第 3 项的 `old_string` 找不到）这时看不到，同 `CheckBatch` 的合计先于各项的取值（A-M11 的两种情形各一个测试，3.14.1）。

#### 3.4.4 第二遍：正文

**三种页**（第一遍之后、任何正文的读之前）：第一遍出错的项之前的正文操作碰到的、batch 之前就在的页，按它在 batch 里的**第一个**正文操作分（之后的正文操作作用在 batch 写下的正文上，不要原来的）：

| 页 | 第一个正文操作 | 计划读 |
|---|---|---|
| 读正文的 | `edit` | 头，再读正文 |
| 读正文的 `write` | `write`，而它之前的改名、移动经索引碰到这一页的链接（`BatchLinks.Reached`，3.7.4；读了头之后问）：带 `base_version` 的整页替换；不带的与这一页照它们改写之后的正文比字节，相同是未改动、不同是 409 `PathTaken`（S1 审查 A-I1、B-M2） | 头，再读正文 |
| 只读头的 | 别的 `write`：带 `base_version` 的整页替换；不带的与页字节相同是未改动、不同是 409 `PathTaken`，以头的大小与哈希比（同 `write` 自己的，`write_path.go:185-187`） | 头：版本号、大小、哈希 |
| 都不读的 | 没有：batch 新建的页 | — |

**之前的改名、移动碰到的 `write` 照改写之后的正文比**（S1 审查 A-I1、B-M2）：分开调用里先做的那次改名已把这一页改写了，`move A.md → B.md` 之后不带令牌的 `write P.md` 写的若正是 P 改写之后的字节，是未改动；写的是改名之前的字节，是 409。只比头（batch 之前的大小与哈希）时两者正相反：batch 收下分开调用拒绝的，拒绝分开调用收下的。所以读了头之后，对第一个正文操作是 `write`、它之前有改名或移动的页，用例问 `BatchLinks.Reached`：这些改动按改写的规则（`ownLinks`，`update_links` 为假的节点的链接除外，3.7.2）碰不碰得到索引里这一页的链接（索引不是这一页的版本号或不是现在的提取器的，照 M6 不改写，答"碰不到"）；碰得到的页读正文（`BatchPlan.ReadWhole`），`write` 与它照之前的改动改写之后的正文比字节，比的是同一个改写（3.7.4 的 `Rewrite`，`edit` 在 `Replace` 之前也用它）。碰不到的照旧只读头：它的字节在那之前没有变。不带令牌的、带过期的令牌的都这样比；带当前的令牌的照旧整页替换。

**先以大小拒绝，再读**：用例以 `PageStates` 一条语句读前两种页的头（版本号、`byte_size` 与新加的 `content_hash`，`page/adapter/postgres/queries/contents.sql:40-44`，不读正文；一行之内是一致的），读正文的页的大小按项的次序累加（上面那种 `write` 的页也在其中，S1 审查 A-I1 的第 3 条疑点：之前的改名碰到的三页各 5 MiB、带令牌（`base_version`）的整页替换，以前只读头能过，现在过了 14 MiB，答读的界线的 422，提示说为什么："a write after a rename or move that reaches its page reads the page whole: send the renames or moves in another batch"（S1 审查 A 的 N3、N5）。带当前的令牌的整页替换也读整页：它的 `changed` 要比字节。把改名与之后的写分开发就不读它们），过了 `MaxBatchReadBytes` = `MaxUnitBytes` + `MaxBatchStringBytes`（8 + 6 = 14 MiB）的那一项答 422 `page.batch_too_large`（"N bytes in the pages its ops read, at most M: split the batch"，读整页的 `write` 在其中时是上面的提示；实施的偏差 15），什么正文都不读，也不跑第二遍。只读头的页（之前的改动碰不到的 `write`）不数：它们的正文不读，三页 5 MiB 的整页替换（15 MiB）不该被拒绝；三页 5 MiB 上不带 `base_version`、字节不同的 `write` 答第一项的 409 `PathTaken`，不是第三项的读的 422（审查 A3-N1）。只有一页时不限（一页至多 5 MiB）。为什么是这个数：多于一页时最后的正文合计至多 `MaxUnitBytes`，batch 自己的字符串至多 6 MiB；读的页大过两者之和，除非它的 `edit` 删去的比它带来的多，最后的正文就过了 `MaxUnitBytes`。这样的请求只有约 10 KB，不占 `/mcp` 的大请求体的位置（`bootstrap/mcp.go:31-41`），一个 PAT 又可以同时发 200 个：计划不为它读上百兆（读 14 MiB 时活着的堆约 27 MiB；100 页 5 MiB 的实测在审查记录）。

**版本号与正文一起读**：读正文的页逐页以 `Nodes.PageContent` 读正文与它的版本号（一条语句答两者，同 `edit` 的读，`page/app/edit_page.go:138-142`），至多 100 页；它们的头只用来拒绝。分两条语句读（先读正文、后读版本号）会把正文 r 与版本号 r+1 配成一对：单元锁下看到 r+1、等于计划的，就把其间的那次写盖掉了。只读头的页的版本号、大小与哈希来自同一行，没有这个问题。

```go
// PageText is a page's content and its revision, read in one statement.
type PageText struct {
	Content  string
	Revision int
}

// PageHead is a page's revision, size and content hash, read in one
// statement without its content: a page whose first content op in a batch
// is a write, which replaces it whole with base_version, and without one
// holds its bytes already or is PathTaken, as write tells by size and hash.
type PageHead struct {
	Revision int
	Bytes    int
	Hash     []byte
}

// Contents is the plan's contents' pass, of the ops before its failed one
// (M9 design 4.11), on the pages before the batch, read unlocked, by id:
// texts those whose first content op is an edit, or a write the renames and
// moves before it reach (ReadWhole), heads the other writes'. A
// base_version against the page's revision then: past,
// page.revision_mismatch, unless the op is a write of the bytes the page
// holds at that op (a page read by its head alone compares the head's size
// and hash, the unit's sha256), which is unchanged, as write's own is; a
// write's content (a write without base_version of the bytes the page holds
// is still the page's content op, writing nothing); an edit's Replace on
// the page's content as the ops before it leave it, unchanged when it
// leaves the bytes as they were, as edit's own is (M9/P3 review A-M3). A
// write or an edit sees
// the page's content rewritten first by rw where a rename or a move between
// its content ops reaches its links, as separate calls would leave it
// (M9/P3 review A-I1: a write's bytes too); a creation's content is
// compared with a page there by its head's size and hash. Each page's
// content at its last content op is the one the unit writes. A failure of
// an op before Failed's replaces it, so that the first op that fails
// decides; its error is a problem's, rw's too (its parses' bounds and
// budget). Any other error of rw's is Contents' own. A plan that failed for
// good, past its items or its reads, is not run.
func (p *BatchPlan) Contents(texts map[uuid.UUID]PageText, heads map[uuid.UUID]PageHead, rw LinkRewriter) error
```

过期的令牌、相同的字节，答"未改动"：同单个 `write`（`writeContent` 在比 `Base` 之前先比字节，`page/app/unit_content.go:61-66`）；`edit` 带过期的令牌照旧是 409（它在 `Replace` 之前比令牌，`edit_page.go:145-147`）。"相同"是与这一页在那一项时的正文比：之前的改名、移动改写过它的，是改写之后的（上一段）。`edit` 替换之后的字节与那一步的页相同时（例如 CRLF 的规则让 `old_string` 与 `new_string` 成了同一个串）答 `changed: false`，同单个 `edit`（S1 审查 A-M3）。

- **第一个出错的项定答复**：第二遍只跑第一遍出错的项之前的项；第二遍在更早的项出错时取代它（条目与读的界线除外：它们在第一遍结束时就答，3.4.3）。两遍互不依赖：树的操作不读正文（"同样的字节"留到第二遍，在第一遍里当作"未改动"往下走，第二遍出错时从那一项起作废），正文的操作在第一遍里只找页。
- **计划里的改写**（4.11 的"两段"第 1 条）：第 j 项是 `edit`，它的页 P 在 j 之前最近的正文操作是第 k 项（没有时 k = -1，即 batch 之前），k 与 j 之间有 `update_links` 为真的改名、移动（节点的每个操作的 `update_links` 都为真，"一个为假就整个不改写"）：先以 `rw.Rewrite` 把 P 此刻的正文照这些改动改写，再在改写之后的正文上 `Replace`。"之前"：k = -1 时是 batch 之前的索引（`rw.Indexed`；P 的索引不是当前的版本号、或不是现在的提取器的时照 M6 跳过，不改写）；否则是 P 在第 k 项时的正文在第 k 项之后的树上的解析（`Resolve`）。`write` 带 `base_version` 整页替换，不改写（agent 写的就是最后的）；不带的与改写之后的正文比字节（上面，S1 审查 A-I1）。k = -1 时 P 的链接从 batch 之前的索引跟起：batch 前面的项对它们的去向做的（例如在根下新建一个同名的页，分开调用会让 `[[X]]` 改指它）这里看不见，这是 4.11 的规则（3.14.4 的 (c)），工具的说明写着。
- **`LinkRewriter`**：domain 的接口，用例以 `BatchLinks`（3.7.4）实现；它的解析经 `ContentParser.Planned`（3.4.5 的计划的解析）。`Indexed` 给一页的第一个正文操作的"之前"（batch 之前的索引），`Rewrite` 收"之前"所在的那一项（`since`，-1 是索引）：计划要从索引或从上一个正文操作改写（实施的偏差 3）：

```go
// Lead is where a link led at a moment of a batch's plan: the node it
// resolved to, none for nothing; whether ambiguously, the least id
// deciding between paths equally short; whether to an attachment. As
// linking's Resolution, which bootstrap copies field by field: a link
// ambiguous before and after is not written again (M9/P3 review A-M3).
type Lead struct {
	ID        uuid.UUID
	Ambiguous bool
	Asset     bool
}

// PathStep is a node on a path from the root: its id, title key and name.
type PathStep struct {
	ID        uuid.UUID
	Key, Name string
}

// LinksBefore is where a page's links led at a moment of a batch's plan
// (M9/P3 design 3.4.4, 3.7): each link, by where its target starts, and
// the nodes they led to as they were then, their paths from the root.
type LinksBefore struct {
	Leads   map[int]Lead
	Targets map[uuid.UUID][]PathStep
}

// LinkRewriter writes again, for a batch's plan, a page's links that the
// renames and moves since its content was read reach (M9 design 4.11).
// Its parses are the plan's (ContentParser.Planned): facts kept by page
// and hash, so that a content parsed once is parsed no more; each parse
// is admitted by the plan first (BatchPlan.Admit), within its bounds.
type LinkRewriter interface {
	// Indexed is where the links of the page id led by the index as the
	// batch began, at revision: false when the index is not of revision or
	// not of the current extractor (M6's skips), its links then not
	// written again.
	Indexed(id uuid.UUID, revision int) (LinksBefore, bool, error)
	// Resolve is where the links of content, the page id's, lead on the
	// tree after the op at: the before of the page's next rewrite, and of
	// the deferred one (M9/P3 design 3.7).
	Resolve(id uuid.UUID, content string, at int) (LinksBefore, error)
	// Rewrite is content, the page id's, its links resolved as before
	// tells at the op since (-1: the index before the batch), written
	// again so that each leads where it led on the tree after the op
	// until: as the participant writes them (M6/P4), through the renames
	// and moves between the two (BatchPlan.MovesBetween).
	Rewrite(id uuid.UUID, content string, before LinksBefore, since, until int) (string, error)
}
```

- **计划自己的解析的界线**：计划解析的是最后的正文之前的正文（被改写的正文、改写的读回、下一次改写的"之前"）；每页最后的正文只由 `DecidedAll` 解析一次（3.4.5）。它们按不同的（页、哈希）数字节，至多 `MaxBatchParseBytes` = 2 × `MaxContentBytes`（10 MiB：最大的一页改写一次，改写的正文与读回的写法各 5 MiB，放得下）；一次会过界线的解析不做，答 422 `page.batch_too_large`（"N bytes of contents the plan parses to rewrite links, at most M: send the renames and moves in a call apart from the edits after them"），`op` 是要它的那一项。事实按（页、哈希）留到请求结束（3.4.5）：`Resolve` 解析过的正文，下一次 `Rewrite` 不再解析。只有一页时也受它限：只有一页的豁免只管最后的正文（与撤销的两份正文，3.18.4），不管计划的解析（50 对"把 Ai 改名、改 Index"的 batch，Index 5 MiB 时要解析 100–150 次，过了 15 秒的期限，审查 A-I4）。
- **多于一页时计划的解析也数进 `MaxUnitPages` 与 `MaxUnitBytes`**：按（页、哈希）与最后的正文一起数，过了答 422，`op` 是那一项；一个多页的 batch 解析的不多于一个单元可以的（8 MiB），`MaxBatchParseBytes` 对它不起作用。只有一页时最后的正文不限（至多 5 MiB），计划的解析至多 10 MiB。所以一页 5 MiB 的改名加改写能过，再加第二页的一个字节的 `edit` 就过了 8 MiB：文字说计划的解析也算在里面、怎样拆开（"N bytes in the pages the batch writes and the contents its plan parses to rewrite their links, at most M: send the renames and moves in a call apart from the edits after them"，页数过了时是 "N contents of the pages the batch writes and the plan parses to rewrite their links, at most 100: …"，3.9.2；审查 A2-N8；实施的偏差 15）。

#### 3.4.5 计划的结果

计划之后用例得到：每一步的节点、每页最后的正文（与它第一个正文操作的项、计划读到的版本号）、每个被改写过的页的"之前"、条目数、计划里解析过的（页、哈希）与它们的事实。

- **计划的解析**：`ContentParser.Planned`（新）给这次请求一个解析的会话：每次解析照 `Decided` 取预算（`len(content)`），解析之后只留事实的那一份（`KeepFacts`），事实按（页、哈希）留到请求结束。
  - **一个合计的等待**：会话的各次取预算与之后最后的正文的 `DecidedAll`（会话的方法）共用一个剩余的等待（`page.parse_max_wait`，2 秒，`server/configs/config.yaml:98`），用完了而请求还有时间是 503 `server_busy`（同 `DecidedAll` 今天的，`page/app/content.go:76-131`）：不是计划与 `DecidedAll` 各等 2 秒（审查 A2-N3）。
  - **会话跨过重新计划**（审查 A3-N2）：单元里的重做从头再来一次时（3.6.2），第二次的计划与它的 `DecidedAll` 用同一个会话：同一个剩余的等待（一次请求一份 `parse_max_wait`，不是每次计划一份），第一次解析过的（页、哈希）不再解析（只因树被别人改了而重新计划时正文没变，什么都不再解析）。界线照每次计划各数各的：第二次的计划从零数它要的（页、哈希），重用的也数，所以它答的 422 与一个新的请求答的相同。
  - **持着的事实不挡自己的解析**：会话每次取预算之前，先放还它持着的事实的份额，事实留着、不再计数（同 M6/P4 的 c1、c5-6，`linking/app/rewrite.go:243-249`）。不放还时，预算最小的 5 MiB 下，一页 5 MiB 的改写的读回要整个预算，等到 503 也等不出计划自己持着的约 0.5 MiB（审查 A2-M2）。不计数的至多是计划解析的十分之一，约 1 MiB；重新计划时第一次留下的（计划的，与 `DecidedAll` 的：第一次的单元回滚时放还了它们的份额）也留着、不计数，正文都被别人改了时合计至多约 3.6 MiB。
- **锁的预检**：计划之后、解析最后的正文之前，以 `Locks.Of` 读正文操作的页（batch 之前就在的）与每个删除的子树里的页（并进来的删除的在内）的编辑锁；按项的次序，第一个写或删除被编辑的页的项答 `page.locked`：正文操作的说那一页（同 P2 的 `edit` 的预检，`edit_page.go:151-161`）；删除的照单个 `delete`（M9/P2 3.6），列出它的子树里每一页被编辑的，按子树在 batch 之前的次序（batch 不删除它新建、改名、移动过的节点，这棵子树在 batch 里没有变，就是单元读到的、守卫列出它们的次序）。不为一个守卫会拒绝的 batch 花解析的预算；分开调用也在那个删除处答（S1 审查 A-M1）。单元里守卫照旧再拒绝一次（预检之后才上的锁）。只被链接的改写碰到的页不在预检里，在单元里由推后的改写查（3.7.2）。用例因此另收锁的持有者与时钟（实施的偏差 7）。
- **最后的正文**：按（页、哈希）以会话的 `DecidedAll` 一次解析（从大到小，在会话剩余的等待之内，`page/app/content.go:87-131`；取第一份之前放还计划持着的事实的份额），持到单元结束；之后数链接（`MaxUnitLinks`，3.8）。推后的改写之前，用例放还它们的份额，事实留着到单元结束、不再计数（至多最后的正文的十分之一：多于一页时约 0.8 MiB，只有一页时约 0.5 MiB）：参与者对写了的页取这些事实、不再解析（3.7.2），它的读回（改写的写法）以 `ParseNow` 取整份，不被 batch 自己持着的挡住。只放还这一页的不够：P 5 MiB、Q 3 MiB 都写了时，Q 的 0.3 MiB 仍挡着 P 的 5 MiB 的读回（原型的预算核对，`exp/results.txt`）。
- **最后的正文的"之前"**：每页最后一个正文操作时的 `LinksBefore`（推后的改写的"之前"，3.7.2），以 `Resolve` 在那一项之后的树上算，用 `DecidedAll` 的事实（同一个（页、哈希）），不再解析。

### 3.5 合并、正文合成与条目

#### 3.5.1 合并的每一种

单元的改动以 `Change.Then` 合并（`page/domain/change.go:67-73`，不改），条目与版本行在 SQL 里合并（`changesets.sql:11-57`）。batch 里会出现的每一种（4.11 的"合并的语义"）：

| 一页在 batch 里 | 条目 | 版本行 | 撤销（S2） |
|---|---|---|---|
| 新建，之后改名、移动 | 前空、后是最后的；版本号 1（合并取第一次的，`:41-44`） | 新建时写最后的正文：一行，版本号 1，没有 `base` | 删除 |
| 新建，之后 `edit`（给新建的页带 `base_version` 是 422，4.11） | 前空，版本号 1 | 正文合成：建页时就写最后的正文，一行，版本号 1（推后的改写又写它时同一行升到 2、仍没有 `base`；`changeset` 照版本行答 2，`history.sql:83-104`） | 删除 |
| 改名、移动，之后（或之前）写正文 | 前、后都在，版本号空（00038 的检查） | 一行：`base` 是 batch 之前的版本号 | 放回、写回 |
| 写正文，之后删除 | 删除的条目，版本号是删除时的（合并进删除取删除的，`:44`） | 一行：`base` 是 batch 之前的 | 恢复、再写回（S2） |
| 删除子孙，之后删除含它的祖先 | 规范化：一次删除 | — | 恢复一组 |
| 新建后删除 | B1 拒绝 | — | — |
| 改名、移动后删除 | B3 拒绝 | — | — |
| 改名、移动，之后又回到原处（父节点、名称、次序值都与之前相同） | 合并成前后相同的一条；单元的最后删去它（3.5.4） | — | 不在其中 |
| 推后的改写又写了 batch 写过的页 | 不变（正文的改动不进条目，`unit.go:384`） | 同一行更新（行的版本号等于新的 `base`，`:57`）：仍是一行，`base` 不变 | 同上 |

删除后恢复、恢复后删除不会出现（没有恢复的操作）。合并成前后都空的条目由 B1 挡在计划里，单元里仍被检查拒绝（`00015` 的 `changeset_items_state_check`，缺陷，500）。移出再移回的节点排在新的兄弟的最后（`unit_relocate.go:69`），次序值通常与之前不同：那一条留着，历史照 `ChangeRow.Change()` 显示为"调整了次序"（`page/domain/changeset.go:268-284`），它也确实被换了位置。

#### 3.5.2 正文合成

同一页的几个正文操作合成一步：单元里在这一页**第一个**正文操作的位置写它**最后的**正文（计划算出的），一个版本行、一次守卫：M10 的模式守卫看到 agent 要的最后的正文。第一个是新建的，就以最后的正文建（`PageDraft.Content`，版本号 1，`unit_create.go:67`）。否则在那个位置先以 `LockContent` 锁这一页的正文行、读锁下的版本号，与计划读到的比：不同就是"计划之后别人写过"（3.6.2 的重做）；相同时最后的正文与读到的不同才 `WriteContent{Base: 那个版本号}` 写它。之后 `WriteContent` 再答 `ErrRevisionMismatch` 是缺陷（锁持着、版本号刚比过），500，不是重做。同一页之后的正文操作在单元里只核对路径找到的还是这一页，不写。

**不带 `base_version`、字节相同的 `write` 也是正文操作**（审查 A-M4）：它在 batch 里不写，但单元照样锁这一页的正文行、比版本号，合成的写也落在它的位置（它是这一页第一个正文操作时）。分开调用时它在锁下比字节（`page/app/write_path.go:177-191`），计划之后别人写了别的字节就答 409 `PathTaken`；batch 不比版本号就会答 `changed: false`，因为没有一个 `Base` 的检查看得到那次写。比了，就重做，第二次的计划看到别的字节，答同样的 409。所以显式的比较留着，不靠 `WriteContent` 的 `Base`（审查 C-O1 不取）。

中间态不进任何地方：合成的写在第一个正文操作的位置写下的正文可能带着之后才改名的目标的新名称（计划里改写过的），观察者在单元结束时才看到合并的改动（`unit.go:450-461`），索引与搜索只看最后的样子；守卫看到的是那一步的状态加最后的正文。

#### 3.5.3 `recordItems` 的注释

`page/app/unit.go:366-369` 的"No unit of M4 creates a node and deletes it"改为写明这几种：一个 batch 的单元里新建之后改名、移动、写正文合并到新建的条目；先写正文再删除是删除的条目，带删除时的版本号；新建后删除、改名或移动后删除由 B1、B3 在计划里拒绝，单元里仍出现就是缺陷，由条目的检查拒绝；回到原处的节点的条目在单元的最后删去（3.5.4）。

#### 3.5.4 回到原处的条目

`move A.md → B.md` 再 `move B.md → A.md` 在一个 batch 里合并成一条前后相同的条目（`changesets.sql:41-44` 留第一次的"前"与最后的"后"）：历史会显示一个没有被调整次序的节点"调整了次序"（`ChangeRow.Change()` 在父节点与名称相同时落到 `ChangeReordered`，`page/domain/changeset.go:268-284`），撤销与它的前提看到一条什么都不做的条目。分开调用时是两次改名。所以：

- **删去它**：batch 的 `do` 的最后（推后的改写之后、答复的读之前）调 `u.DropUnchanged(ctx)`：前后相同的条目**都**以一条语句删去（正文的写不进条目，`recordItems` 只记 `c.Moves()`，`unit.go:370-392`，它在版本行里）；单元合并的改动里前后都在、`TreeState.Same` 为真（父节点、名称、次序值都相同）的节点，没写正文的从 `u.changes` 里去掉，写了正文的留着它的改动（观察者要照它重建索引），被一次还在的移动带着走的节点也留着（观察者跟着它们到新的父节点下，实施的偏差 6）。删去的语句（`Changesets.DropUnchanged`：`DELETE … WHERE changeset_id = $1 AND before_name IS NOT NULL AND after_name IS NOT NULL AND before_parent_id IS NOT DISTINCT FROM after_parent_id AND before_name = after_name AND before_sort_order = after_sort_order`）。检查允许：`00015` 的 `changeset_items_state_check` 与 `00038` 的 `changeset_items_revision_state_check` 都是一行之内的，没有一条要求变更集有条目；记下：删去，不是只写明它的显示。
- **空的变更集**：删去之后变更集既没有条目、又没有版本行（这个 batch 什么都没留下，例如只有那两次改名，或 `edit` a→b 再 b→a，单元本来就不写，`unit_content.go:61-64`），`DropUnchanged` 的第二条语句把这个单元建的变更集也删去（只删没有条目、没有版本行的那一个），`u.changeset` 归零：`Outcome.ChangesetID` 为空，答复的 `changed` 为假、没有 `changeset_id`，同一个什么都没改的 batch；`u.changes` 也空了，观察者、搜索与事件都不跑（`tell` 在没有改动时什么都不做，`unit.go:450-453`）。
- **每一项的 `changed`** 仍是那一项的工具在那一步会答的（3.9.1）：两次改名各答 `changed: true`，batch 的 `changed` 为假。工具的说明写着两者的不同。
- 测试在 3.14.2。

### 3.6 用例 `ApplyBatch`：计划、解析、单元里的重做

```go
// PathBatch is batch's arguments (M9 design 4.11).
type PathBatch struct {
	Ops     []domain.BatchArgs
	Message string
}

// OpDone is an op's outcome: the node it acted on and its path after the
// batch, or at the op for a node a later op deleted; a content op's page
// version as the batch leaves it (the deferred rewrite's write too), or
// as the op's composed write left it for a page a later op deleted;
// whether its tool would have changed anything at its step; the nodes a
// write created, root first, at their paths after the batch; an edit's
// replacements; a delete's pages and attachments, none for one merged
// into the later delete MergedInto.
type OpDone struct {
	Kind          domain.OpKind
	Path          string
	Node          domain.Node
	Version       domain.VersionToken
	Changed       bool
	Created       []Placed
	Replacements  int
	Pages, Assets int
	MergedInto    int // -1 for none
}

// Batched is a batch's outcome: each op's, in their order, and the unit's.
// Outcome's ChangesetID is zero when the batch left nothing; its Building
// is set when the batch queued nodes for the search, in bulk (M9/P3 design
// 3.10.1).
type Batched struct {
	Ops     []OpDone
	Outcome Outcome
}

// ApplyBatch applies a batch of MCP's writes as one changeset (M9 design
// 4.11; M9/P3 design 3.6): planned on the notebook's tree read unlocked,
// then done again op by op in one unit under the notebook's lock, which
// must find what the plan found.
type ApplyBatch struct {
	writer    *Writer
	parser    *ContentParser
	paths     NodePaths
	nodes     Nodes
	states    PageStates
	links     BatchLinks
	locks     *LockHolders
	notebooks Notebooks
	auth      shared.Authorizer
	// maxRewrites bounds the pages the deferred rewrite writes, maxLinks
	// their links, maxBytes their bytes: 0 is the domain's,
	// domain.MaxBatchRewrites, domain.MaxBatchRewriteLinks and
	// domain.MaxBatchRewriteBytes, which only a measurement raises (M9/P3
	// design 3.8.2; review B1-M3; fix round 5, B2-M1).
	maxRewrites, maxLinks, maxBytes int
	clock                           Clock
	logger                          *slog.Logger
}

// Execute applies p in the notebook notebookID from client. Its order: the
// actor; CheckBatch's 422s; the notebook, notebook.not_found; page.batch's
// 403, unlocked; the tree's pass (PlanBatch): the first op that fails, the
// items past MaxUnitNodes at the op that passes them; the sizes of the
// pages its edits read, past MaxBatchReadBytes at the op that passes them,
// before any content is read; the contents' pass (Contents): the first op
// that fails, its parses' bounds (422) and budget (503) at it; the edit
// locks of the pages it writes or deletes, page.locked at the first op that
// meets one; the unit's bounds on what it parses, page.batch_too_large; the
// final contents parsed, 503; their links, page.batch_too_large; then the
// unit, which does each op again under the notebook's lock and must find
// what the plan found: else, as for a page gone between the plan's reads of
// the tree and of the contents, the whole of it once more from the plan,
// in the same parse session, and then page.revision_mismatch at the first
// op that differs. A refusal of an edit lock carries its pages' paths and
// the op it met.
func (b *ApplyBatch) Execute(ctx context.Context, notebookID uuid.UUID, p PathBatch, client domain.Client) (Batched, error)
```

#### 3.6.1 单元的规格

`UnitSpec{NotebookID, Action: domain.ActionBatch, Tree: true, Client: client, Options: Options{UpdateLinks: true}, Message: &p.Message, NotFound: domain.ErrNotebookNotFound, DeferParticipants: true, BulkSearch: true}`：改树的单元（笔记本行 `FOR NO KEY UPDATE`），`Writer.Run` 在锁下再判定一次 `page.batch`（`unit.go:207-210`）；变更集的类型是 `edit`、带说明（`UnitSpec.Message`，`unit.go:354-357`）；为搜索批量入队（`BulkSearch`，3.10.1）。

#### 3.6.2 单元里按次序重做

`do` 里，先读计划里被改名、移动的节点原来的各个父节点的路径（锁下、任何一步之前，一条语句：3.7.2 的"原来的父节点"）。`NodePaths.Paths` 只答字符串（`page/app/node_paths.go:111-113`），这里要各段的 id、键与名称：新的 `PathStore.LinePaths(ctx, notebookID, ids) (map[uuid.UUID][]domain.PathStep, error)`，同 `PathsOf` 读 `LinkTargetsByIDs`（`page/adapter/postgres/store.go:140-143`）而答各段。然后按次序重做每一项（被规范化的跳过）：

| 项 | 锁下找 | 要等于计划的 | 做 |
|---|---|---|---|
| `write` 新建 | `Walk(path)` | 找到的前缀的 id；要建的页的 id、父节点与标题 | 依次 `CreatePage(PageDraft{ID: 计划的 id, …})`（最后一个带合成的最后的正文） |
| `write` 覆盖（带不带 `base_version`，字节相同的也是）、`edit` | `Find(path)` | 节点 id；这一页第一个正文操作时 `LockContent` 读到的版本号等于计划读到的 | 第一个正文操作：最后的正文与读到的不同时 `WriteContent{Base}` 写它（3.5.2）；之后的：不写 |
| `move` | `Find(from)`、`Walk(to 的父页链)` | 两者的 id | `Unit.Relocate(id, Placement{Parent, Name, Field: a.Field("to")})`（P2 的一步） |
| `delete` | `Find(path)`、`Unit.Subtree` | 节点 id；子树的 id 的集合等于计划的（含并进来的组） | `Unit.DeleteSubtree` |

任何一处不等，或单元的操作答出计划里的种类的错误（404、422 的规则、409 的名称、环、层级：锁下才看见的，例如计划之后别人在目标下建了同名的页），都是"计划之后别人改了树或正文"：单元回滚（`errReplan`），用例从头再来一次（读树、新的 id、计划、解析、单元，4.11 的"重新计划一次"；同一个解析的会话：一个合计的等待与按（页、哈希）留着的事实都跨过它，3.4.5）。`WriteContent` 的 `ErrRevisionMismatch` 不在其中：版本号已在锁下比过，它是缺陷（3.5.2）。第二次的计划照常答它的错误（那时别人建的同名的页就是第二次计划的 409）；第二次的单元还不同，答 409 `page.revision_mismatch`，`op` 是第一个不同的项：正文操作是 `VersionMismatch{Path, Current}`（带现在的版本令牌），`Why` 照这一项，同 `edit` 的 `mismatch`（`page/app/edit_page.go:197-214`）：不带 `base_version` 的是 `KeptChanging`（"changed twice while the edit was made"，`page/adapter/mcp/text.go:62-74`），带令牌的是 `Changed`，令牌的节点已不是路径上的这一页的是 `OtherNode`；路径上已没有页的正文操作与树的操作是 `domain.BatchChanged`（前者是实施的偏差 10）（文字 "ops[i] (move): the notebook changed while the batch was planned, twice: read it again and send the batch again"）。守卫的 `page.locked` 与推后的改写的错误不重做，照原样答（3.9）。计划读了树之后、读头或正文之前页被删除，也是"计划之后别人改了树或正文"（S1 审查 A-M2）：用例同样从头再来一次，不在那一页的第一个正文操作答通用的 `page.not_found`（实施的偏差 9 不再有）。

重做不建路径的缓存：每一项在单元的事务里重新找（P2 的 `Walk` 与 `Find` 经单元的事务读，`page/adapter/postgres/store.go:42-44`），至多 100 项、每项至多 11 段；单元前面的项建的、改名的、移动的、删除的，后面的项在同一个事务里都看得见，不需要作废的规则（P2 2.1 第 1 条留给 P3 的，2 节的表）。

#### 3.6.3 答复要的读

单元的最后（推后的改写与 `DropUnchanged` 之后、观察者之前），正文操作的页的版本号以 `PageStates` 读一次（单元的事务里），每个正文操作答它这一页最后的版本（4.11 的"同一页的几个正文操作都答它最后的版本"；推后的改写写过的是改写之后的，2.1 第 6 条）；每一项的路径是单元结束时它的节点的路径（`NodePaths.Paths`，一条语句），`created` 也是（之后移动了新建的文件夹的，答移动之后的）。

**之后被删除的页**：`PageStates` 只读没删除的正文（`contents.sql:40-44`），`NodePaths.Paths` 只给活着的节点，所以"先 `edit P.md`、再 `delete P.md`"（3.5.1 允许）的第 0 项读不到。这样的正文操作答它合成的写留下的版本（单元知道：`WriteContent` 或 `CreatePage` 答的版本号），与这一页在那一项时的路径（计划的树在那一项时的路径，单元的重做核对过它的节点）；删除的项答它被删时的路径。

**每一项的 `changed`** 是那一项的工具在那一步会答的，同分开调用（`edit` 替换之后的字节与那一步的页相同的答 `false`，S1 审查 A-M3）：`edit` a→b 再 b→a 两项都答 `true`，单元什么都不写、没有变更集，batch 的 `changed` 为假（3.5.4）。工具的说明写明：每一项的 `changed` 是那一步的，batch 的 `changed` 说它在历史里留下了没有。

### 3.7 推后的链接改写

#### 3.7.1 page 一侧：推到全部操作之后、一次

- `UnitSpec.DeferParticipants`：`apply` 不调参与者（`unit.go:283-289` 的 `participate` 对这个单元总是假）。`appender` 的操作本来就不调（`unit.go:465-480`）。
- `do` 的最后，用例调 `u.Participate(ctx, b BatchRewrite)`：以合并之后的改动造一个步，调每个参与者一次：

```go
// BatchRewrite is what a batch's deferred participants follow besides its
// changes (M9 design 4.11; M9/P3 design 3.7): each page the batch wrote,
// alive after it; the paths the moved and renamed nodes' former parents
// had as the unit began; the nodes the unit deleted; the most pages the
// rewrite may write, a page of Written counting WrittenWeight (M9/P3
// review B-M3), the most links (review B1-M3), and the most bytes of the
// pages it writes (M9/P3 fix round 5, B2-M1); and the first op that
// renames or moves with update_links, which a refusal names.
type BatchRewrite struct {
	Written       map[uuid.UUID]WrittenPage
	Parents       map[uuid.UUID][]domain.PathStep
	Gone          []uuid.UUID
	MaxPages      int
	WrittenWeight int
	MaxLinks      int
	MaxBytes      int
	FirstMove     int
	// Frozen is the nodes an op of the batch renamed or moved with
	// update_links false, in id order, which Unit.Participate sets: the
	// links that led to them are left as they are, in every page, whatever
	// their ancestors' changes (M9 design 4.11; M9/P3 review B-M1).
	Frozen []uuid.UUID
	// CreatedAfter is, for each node an op of the batch renamed or moved
	// with update_links, the nodes it created after the last such op of
	// it: no link reached them then, so the rewrite of the pages it did not
	// write leaves them out of that node's subtree, which it reads as the
	// unit left it (domain.BatchPlan.CreatedAfterMoves; M9 design 4.11: a
	// creation only resolves again; M9/P3 review B1-M1).
	CreatedAfter map[uuid.UUID][]uuid.UUID
	// FrozenMoves is the renames and moves of the nodes of Frozen, merged
	// node by node over the batch (domain.BatchPlan.FrozenMovesBetween): not
	// followed, but where they were, which the paths before of the nodes
	// under them that the rewrite follows take their names and former
	// parents from, those parents' paths among Parents (M9/P3 review
	// B1-N1: a Markdown link's text that was the old path is recognized).
	FrozenMoves []domain.Change
}

// WrittenPage is a page a batch wrote, for its deferred rewrite: where its
// links led at its last content op, on the plan's tree after that op; its
// own changes, the renames and moves after that op, merged node by node
// with no revision, of the nodes alive then, or created since, and alive
// after the unit, that no op after it moves with update_links false: those
// nodes alone, the participant reading the subtree of each it renames or
// moves once for the batch (M9/P3 design 3.7.2 (e)), so that a moved
// folder's nodes are held once, not once a page; and the facts DecidedAll
// kept for its final content, which the rewrite takes its links from
// instead of parsing it again. Frozen is the nodes the ops after that op
// rename or move with update_links false, whose links it leaves;
// CreatedAfter, for each node they rename or move, the nodes they create
// after the last that does, out of its subtree (BatchRewrite.CreatedAfter).
type WrittenPage struct {
	Before       domain.LinksBefore
	Changes      []domain.Change
	Facts        Facts
	Frozen       []uuid.UUID
	CreatedAfter map[uuid.UUID][]uuid.UUID
}

// Participate has the participants follow the unit's changes once, as one
// operation: those of the nodes alive before and after it that a rename
// or a move changed with every op's update_links true, merged, with no
// revision (M9 design 4.11), and the nodes under the moved ones as they
// are; b. Its appends go through the guards as any; it answers how many
// pages they wrote, which the batch's log line counts.
func (u *Unit) Participate(ctx context.Context, b BatchRewrite) (int, error)
```

  `BatchRewrite` 另带 `WrittenWeight`（写了的页在 `MaxPages` 里算几页，`WrittenRewriteWeight` = 2，S1 审查 B-M3，3.8.2）、`MaxLinks`（改写的链接的界线，3.8）、`MaxBytes`（改写的页的字节的界线，3.8；修正的第 5 轮）、`Frozen`（`update_links` 为假的操作改名或移动的节点，`Unit.Participate` 设它：通向它们的链接在每一页里都不改，S1 审查 B-M1，3.7.2）、`CreatedAfter`（每个被 `update_links` 为真的操作改名或移动的节点，batch 在它最后一次这样的操作之后新建在它下面的节点，照那个窗口的最后一棵树，`BatchPlan.CreatedAfterMoves`；S1 审查 B1-M1，修正的第 5 轮，3.7.2）与 `FrozenMoves`（`update_links` 为假的操作改名或移动的节点在整个 batch 里合并的改名、移动，`BatchPlan.FrozenMovesBetween`：不跟随，只给"之前"的路径，S1 审查 B1-N1，3.7.3）；`WrittenPage` 另带它自己的 `Frozen` 与 `CreatedAfter`（这一页最后一个正文操作之后的，3.7.2）。

  交给参与者的改动（4.11 的"交给它的改动"）：单元前后都活着、被 `update_links` 为真的操作改名或移动过的节点，以合并之后的"前""后"，`Revision` 为 0、不带 `Facts`（否则 `Relocation` 碰上带版本号的就答"不用改"，`linking/domain/change.go:69-71`）；被移动的节点的子孙的项（"前""后"相同）留着；只写了正文的节点不在其中；一个节点只要有一个操作的 `update_links` 为假就整个不在其中。**另加**这个 batch 新建、之后又被改名或移动的节点，"前"是它新建之后的样子：没写的页的链接从单元之前的索引跟起，新建的节点在它最后的位置抢走的，照 M6 写回原来的目标，同分开调用时那次移动的参与者（3.14.4 的 (1)）；只给前后都活着的节点，它们就悄悄被抢走。这一份合并的改动只管 batch 没写的页。

  **batch 写了的页各有自己的改动**（`WrittenPage.Changes`，计划给出）：
  - 它最后一个正文操作之后的改名与移动，按节点合并（第一个"前"、最后的"后"，同 `Change.Then`），只取那时与单元之后都活着的节点；
  - 在那之后才新建、又被改名或移动的节点也在，"前"是它新建之后的样子（同上，它抢走这一页的链接时照 M6 写回）；
  - 那之后有一个操作的 `update_links` 为假的节点整个不在其中（只看那之后的操作：之前的那些，这一页的正文写的时候已经看到了）；
  - 只有这些节点：它们改名、移动的节点之下的节点不在其中，参与者对这一页经 `spread` 读这些节点的子树（3.7.2 (e)），一个 batch 每个节点的子树只读、只存一次。所以一个被移动的大的文件夹的节点一个 batch 只存一份，不是每个写了的页一份（审查 A3-I1：100 页、文件夹 1 万个节点，每页一份时约 275–298 MiB，一份时约 1 MiB；审查 A 与 `exp/` 的原型量的）。不读被移动的节点的子树时，`[[Top/F/X]]` 的键是 `x`、目标是 X，F 只被移动时 `Affected` 碰不到这条链接，链接就悄悄悬空（3.14.3 的第 19 种，审查 A2-I1）。

  从这一页最后一个正文操作算起，是因为 `A → B → A` 合并成 `A → A`，`Relocation` 什么都不搬（`linking/domain/change.go:32-37`、`:68-80`），而写下 `[[B]]` 之后的 `B → A` 在分开调用时会把它改回 `[[A]]`（审查 A-I3）。碰不碰得到、`update_links` 与 `Recased` 都走今天的 `Relocation`、`Affected` 与 `spread`。

  两者合起来就是 4.11 的规则：链接跟着 batch 的改名与移动（按节点合并），从它们在 batch 之前的去向跟起，batch 写了的页里的从这一页在这些改名、移动之前最后一次写时的去向跟起（那时的树、batch 之前的别名）；其间别的项对去向做的看不见。与分开调用的不同都由它而来，测试在 3.14.4。原型（`scratchpad/m9/p3/exp/`，`results.txt`）在 `f1733403` 的 linking 上照这些规则跑了每一种，写出的都是规则说的字节；为什么不用合并的改动、不只比去向，在审查记录。

  `Step.Batch *BatchRewrite` 带上其余的；组合根的参与者的适配把它抄进 linking 的 `Moved`（`linking/app/ports.go:242-247`；同今天逐字段抄改动，`bootstrap/linking.go:55-66` 的 `linkRewrite`）。

#### 3.7.2 linking 一侧：参与者

`linking/app/ports.go` 的 `Moved` 加 `Written map[uuid.UUID]WrittenPage`（linking 的同形状的类型：`Leads` 是 linking 的 `domain.Resolution`，目标的路径是 linking 的 `[]domain.Step`，改动是 linking 的 `domain.Change`，事实是 linking 的 `domain.Facts`（链接与别名），组合根照抄，同它为观察者抄改动的事实）、`Parents map[uuid.UUID][]domain.Step`、`Gone []uuid.UUID`、`MaxPages int`（0 是不限：REST 与单个 `move` 的单元不设它），与组合根照 `BatchRewrite` 抄的 `WrittenWeight`、`MaxLinks`、`MaxBytes`、`Frozen`、`FrozenMoves`、`CreatedAfter`（3.7.1）。`Participate`（`rewrite.go:47-89`）分两部分，先改写写了的页、再改写没写的页（(a)，实施的偏差 27）：

**没写的页**（今天的路，合并的改动）：`reached` 经 `Store.Links`（`rewrite.go:107`）读碰到的索引的行，`resolutions` 之前去掉两种行：来源页在 `Gone` 里的（单元删除了它，它的行还在：观察者在参与者之后才删，`linking/adapter/postgres/queries/pages.sql:70-72`）、来源页在 `Written` 里的（它们的行是旧正文的，由下面那一部分照最后的正文做）。其余照今天。

**写了的页**（`Written` 里、不在 `Gone` 里的每一页 W）：不论索引碰不碰得到它，也不受 `IndexedOf` 的过滤（`rewrite.go:57-68`：它的索引本来就是旧正文的；它的链接取自它的正文）。合并的改动什么都不搬时也做（`rewrite.go:49` 的返回不管它们）。

- **(e) W 自己的改动**：`Relocation(W.Changes)` 什么都不搬、也没有只改了大小写的，W 就不读；否则 `Affected(W.Changes)` 与 `spread` 定 W 的哪些链接被碰到：目标的末段的键在其中、"之前"的目标在其中，或 W 自己在其中（它被移动了，它的每条链接都算）。对写了的页，`spread` 展开 `W.Changes` 改名的**与换了父节点的**节点的子树（审查 A3-I1；单个 `move` 的改动本来就列出被移动的节点之下的节点，所以今天只展开改名的，没写的页照旧）。子树在单元的事务里、全部操作之后读，就是"单元之后活着的、它之下的节点"。各页的这些节点都是整个 batch 的：参与者为这个 batch 记下读过的子树，每个节点的子树只读、只存一次，每页一页一页地把它们并进自己的触及、用完就丢；所以内存随子树，不随写了的页数（嵌套的两个节点各存一份，一个节点至多在 `MaxDepth` 份里），多出的读是每个这样的节点一次（单个 `move` 今天也读它一次，`page/app/unit_move.go:45`）。子树里的节点的别名键（`spread` 末尾的 `AliasKeys`，`linking/app/index.go:102`）也随子树读一次、存在它旁边，每页的 `AliasKeys` 只查它自己的改动的目标，不每页再查一遍整个子树（审查 A4-N2）。不从合并的改动取这些节点：一个节点有一个操作的 `update_links` 为假时整个不在合并的改动里，从被移动的文件夹沿合并的改动往下走就停在它那里，而它之下的节点照样在读到的子树里（`exp/` 的"`update_links` 为假的 X 在 F 之下"：X 的改名在 P 最后一次写之前，P 照分开调用写成 `[[Y]] more`）。**为假的节点的链接整个不改**（4.11，S1 审查 B-M1）：W 最后一个正文操作之后有一个 `update_links` 为假的操作改名或移动的节点（`WrittenPage.Frozen`，由 `BatchPlan.FrozenBetween` 给），W 里"之前"通向它的链接不碰，`spread` 从它的祖先碰到它也不改；没写的页同样（`Moved.Frozen`，单元里冻住的节点：`reached` 在 `resolutions` 之前去掉"之前"通向它们的行），计划里的改写也是（3.7.4）。以前没写的页从被移动的祖先碰不到它（它的那一项不在合并的改动里）、写了的页经 `spread` 碰得到它，同一个 batch 里同一条链接在两种页里一个悬空、一个跟过去；祖先只是改名时两种页都经 `spread` 碰到它，`PathBefore` 却给它现在的名称，"之前"的路径从来没有过，写成 `[[Y|F/X]]`。

  **新建的节点不进之前的改名、移动的触及**（S1 审查 B1-M1）：`spread` 读的子树是单元之后的（计划里是那一项之后的），里面有 batch 在那次改名、移动之后才新建的节点，它们的键进了触及，一条被新建的页以更短的路径抢走的链接就被写回原来的目标；分开调用里那次改名、移动碰不到它们，新建只重新解析（4.11）。所以对每个被 `update_links` 为真的操作改名、移动的节点，batch 在它最后一次这样的操作之后新建在它下面的节点不进它的子树（`stepsNot`）。"下面"照那个窗口的最后一棵树，也就是读这棵子树的那一棵；新建在别处的不在这棵子树里，不列（S1 复核 A 的 F2-1：以前列出窗口里每个新建的，同样的改动的触及因此每次新建都换键、重建）：没写的页照整个 batch（`Moved.CreatedAfter`），写了的页照它最后一个正文操作之后（`WrittenPage.CreatedAfter`），计划的 `Rewrite` 照它的两个正文操作之间、`Reached` 照 batch 开始到它的第一个正文操作（3.7.4）。合并的改动一个节点一项，所以按节点算：一个文件夹改名两次，其间新建的在第二次的触及里，同分开调用；在那次操作之前新建的照样在（3.14.4 的 (1a2)）；新建的节点自己的改名、移动照样跟随（3.14.3 的 17、18）。S1 审查 B 的第一个改法（每棵子树读成它那一项留下的样子）要计划在 linking 的读里给每一项的树、单元在它的操作之间给它的树；按节点给的集合在现在读的一棵树上得到同样的触及。

  **没有链接的页不展开**（S1 审查 A 的 N1）：一页没有链接就什么都碰不到，`metLinks` 在展开子树之前就返回（`Reached`、计划的 `Rewrite` 与参与者的写了的页共用它）。展开的结果（触及）与读过的子树放在一起，只留最后一个，以改动与留在外面的节点为键（`subtrees.reach`）：同一棵树上同样的改动接着再问时不再展开（一次改名之后的几个正文操作）；参与者把写了的页按它们自己的改动排好再碰（列出时照旧按 id），同样的改动的页挨着、共用一个；链接的键与目标在触及排好的切片里二分查找。以前 50 次改名一个 10 万个节点的文件夹、每次之后 `write` 一个没有链接的页，每个 `write` 都展开一次（S1 审查 A 量得 3.27、4.19 秒、分配 15.2 GB），现在 0.80、0.98 秒、0.93 GB（3.8）。

  **一次只留一棵树的读**（修正的第 3 轮，主持者的决定）：

  - **以前**：计划为每一棵树留着它的子树（S1 审查 B-N2）与触及（S1 审查 A 的 N1），一直到计划结束；参与者为写了的页的每一组改动留一个触及。50 次改名一个 10 万个节点的文件夹、每次之后改一个带链接的页，每次改名留下一棵 10 万个节点的子树与一个触及，堆的峰值 +1.1–1.45 GB（修正的第 1 轮之后 +0.7 GB，S1 +0.27 GB）。一个约 10 KB 的请求过了 MCP 的约 1 GB（M12 的性能移交第 18 项）。
  - **现在**：计划只留它最后问的那一棵树的读（`readsAt`，3.7.4）；子树只留一个触及；参与者按改动的次序碰写了的页（上一段）。同一个形状 +248–268 MiB（S1 +230–280 MiB），分配不变（3.8）。新建不算改树（修正的第 5 轮，3.7.4）。
  - **用时的代价**（3.8 的实测）：
    - 留着的少了，回收器跑得更勤，约多 0.4 秒；
    - 计划的两遍各按项的次序问它的树：读的检查（`Reached`，按页的第一个正文操作的次序），然后正文的一遍（`Rewrite`）。读的检查碰到的 `write`（之前的改名改写它的链接，读整页）在正文的一遍里为它的改写把那棵树再展开一次，v0.1 接受（修正的第 4 轮，3.8）；
    - 一棵树上两种改动交替的正文操作每次重新建触及；子树照旧只读一次。

- **(a) 链接取自最后的正文**：在单元里读 W 的正文（`Contents.Content`，单元的事务里，读到的就是 batch 写的、`DecidedAll` 解析过的那一份），链接取自 `W.Facts`（`DecidedAll` 为同一个（页、哈希）留下的事实；计划的重用也靠它们带着链接），不再解析（审查 A2-M2）。改写的写法的读回照今天以 `ParseNow` 取整份（预算不够是 503 `server_busy`，`linking/adapter/markdown/facts.go:245-252`）；batch 持着的事实的份额在推后的改写之前已放还（3.4.5），参与者又先改写写了的页、再改写索引里的页，所以预算最小的 5 MiB 下一页 5 MiB 的读回也取得到（实施的偏差 27：反过来时，索引里的页每改写一页，它的事实的份额留到单元结束，满负载的页约 8.6 KB，399 页约 3.4 MB，之后写了的那页 5 MiB 的读回只剩 8 MiB 里的 4.98 MiB，答 `server_busy`；量最坏的 batch 时发现，`TestABatchRewritesThePagesItWroteFirst` 钉住这个次序）。碰到的链接按起点对上 `W.Before.Leads`："之前"是计划给的去向（带 `Ambiguous`：之前与之后都是歧义的链接不改写，`Rewrites`，`linking/domain/rewrite.go:62`），"之后"是这些链接在最后的树上的解析（`resolutions`，行由解析造：来源是 W），照 `domain.Rewrites` 选要改写的。
- **(b) 不照"正文比索引新就跳过"**（`rewrite.go:227-231`）：这些页的正文一定比索引新。
- **(c) 追加的写以 batch 写的版本号为基**（今天以索引的，`rewrite.go:309`）：读到的 `current` 就是它。
- **(d) `was` 是目标在这一页最后一个正文操作时的样子与路径**（`W.Before.Targets`），不是单元之前的：每个写了的页有自己的 `tree.Before`（今天一个单元共用一棵，`rewrite.go:184-191`）。"先把 A 改名 B、再改 P、再把 B 改名 C"：P 的"之前"是 B（第二步的树），所以写成 `[[C]]`，与分开的三次调用相同（按单元之前的 `was` 叫 A，`byTitle` 为假，会写成 `[[C|B]]`）。

  写了的页的目标之后的路径与按键的页只读一次（S1 审查 B-M3）：参与者先为每个写了的页定它碰到的链接（`ownLinks`），再以一条 `Paths` 读全部这些页的目标与这些页自己、一条 `ByKeys` 读它们的键（`readWritten`），各页只在自己的 `tree.Before`（`W.Before.Targets`）上不同（`writtenTrees.of`）：读从每页两次到一共两次。S1 审查 B 估计每页各读一次时，99 个写了的页、一个被移动的 1 万个节点的文件夹，树占约 1.3 秒；同一次会话里量，S1 审查 B 的混合（99 个写了的页与 300 个索引里的页）改之前 5.56–5.61 秒、改之后 5.32–5.52 秒：读的用时随目标的个数（这里 9,900 个），不随语句的条数。

**`Recased` 是一个集合**（`linking/domain/change.go:54-80`）：今天 `Relocation` 只留最后一个只改了大小写的（`:74-78`），一个 batch 把 `Notes.md` 与 `Plan.md` 都只改了大小写时 `[[Notes]]` 不改（它自己的单元会改，M9/P2 3.5.3；审查 A-M2 (e)）。改为 `type Recased map[uuid.UUID]string`（节点到它新的标题），`Relocation` 答每一个，`reached` 碰到每一个，`Rewrites` 看"之前"的目标在不在其中；单个操作的单元至多一个，结果同今天。W 的 `Recased` 来自 W 自己的改动。

**跳过与删除了的页**（审查 A-I2、A-M13）：

- **删除了的页的别名**：`resolutions`（`linking/app/resolver.go:54-167`）多收一个 `gone` 的集合：别名行的页在其中的，当作没有这个别名（`AliasesByKeys` 不看页活不活着，`linking/adapter/postgres/queries/pages.sql:60-64`；观察者删了那一页就是这样）。参与者给它 `Gone`；计划里的 `BatchLinks` 给它这个 batch 在那一项及以前删除的页（3.7.4）。其余不在的页照今天答内部错误（"linking: the pages [...] of links or aliases are not pages of ..."，`rewrite.go:107-117`、`resolver.go:119-166`），一致性的检查不变。没有这两条时，"删除 P（别名 Roadmap）、把 Plan 改名为 Roadmap"与删除了的来源页都在 `reached` 里答 500（审查 A-I2）。
- **"之前"的目标已不在的链接安静地跳过**：目标被这个 batch 删除了（`Gone`）的，记 info；计划之后被别人删除了的（`W.Before.Leads` 是不加锁时算的，重做只比 batch 自己的节点），记 warn（"a link is not rewritten: where it led is gone"）。今天两者都以 error 级记"没有写法通向它"（`rewrite.go:252-254`、`:315-318`）。删除只重新解析，不改写（4.5）。

**数、锁与错误的次序**（审查 A-M5）：改写的页数在 `reached` 与索引的过滤之后、读改写要的树（`treeOf`：目标的路径与按键的页）、`refuseLocked` 与任何解析之前数（S1 审查 B-I1：过界的 batch 不为它读树，立即答），界线在它限的工作之前：没写的页里有碰到的链接的，加上写了的页里它自己的改动碰到它的链接的（`metLinks`：照 `ownLinks` 的触及与对上，在任何解析之前；链接通向一个节点、不经别名的值、不通向冻住的节点），后者每页算 `WrittenWeight`；都只数有一条通向 batch 没删除的页的链接的页（S1 审查 B1-M2：以前写了的页只要自己的改动搬了什么就数，碰不到它任何链接的也算 2 页，S1 审查 B 的 w1 被拒）；过了 `MaxPages` 答 linking 的 `ErrTooManyRewrites`（`TooManyRewrites.Pages`），组合根译成 page 的 422 "N pages whose links the batch's renames and moves would rewrite, each page the batch wrote counting 2, at most M: move with update_links false, or in separate calls"（`BatchTooLarge`，3.9.2；同 `ErrGuardLocked` 的译法，`linking/app/ports.go:271-274`）。然后数这些页里要改写的链接：没写的页里 `domain.Rewrites` 要写的，与写了的页里碰到的，都只数通向 batch 没删除的页的，过了 `MaxLinks` 答 `ErrTooManyRewrites`（`TooManyRewrites.Links`），组合根译成 422 "N links the batch's renames and moves would rewrite, at most M: move with update_links false, or in separate calls"（S1 审查 B1-M3）。然后数这些页的字节（S1 复核 B 的 B2-M1）：页的头里的大小（`PageSizes.ContentSizes`，页模块的 `PageStates` 那条语句，在单元里），写了的页按它最后的内容、算一次（它的改写读它最后的内容，事实是 batch 的，只解析写出的那一份）；过了 `MaxBytes` 答 `ErrTooManyRewrites`（`TooManyRewrites.Bytes`），组合根译成 422 "N bytes of pages the batch's renames and moves would rewrite, at most M: move with update_links false, or in separate calls"。每个改写的页留着它的事实的份额（约它写出的内容的字节的十分之一）到单元结束，改写的解析立即取预算（`TakeNow`），所以过重的页在两个界线之内也会答重试不了的 `server_busy`；数在读树之前，过界的 batch 立即答（3.8.2）。字节数的是页，写出的内容可以长得多（链接写出目标新位置的路径，3.8），所以 batch 的改写还记着它改写过的页留下的份额（`keptShares`，只在 `MaxBytes` 设了时）：每次解析之前（读索引里的页的那一次与每次写出的），这一次要取的（`RewriteParser.Takes`）加上这些份额过了预算，就在取之前答 `ErrTooManyRewrites`（`TooManyRewrites.Kept`，S1 复核 B 的 B3-M1），组合根译成 422 "N bytes of the parse budget the pages the batch's renames and moves rewrite would hold, at most M: move with update_links false, or in separate calls"（M 是配置的预算）：别人放手也腾不出来，等也没用。放得下、只是别人的份额（或在等的取）挡着的，照旧立即答 503。检查在参与者里：份额是它留到单元结束的（`Appender.Defer`），只有它知道预算里哪些是自己的；页模块的解析会话在它之前已经放掉了自己的份额，也看不到改写的解析。单个 `move` 不检查，同 M6/P4。`MaxBytes` 为 0 时不读大小（单个 `move`）。写了的页碰到的链接在三个数之后才解析（`rewrittenOf`），`refuseLocked` 读的是数过的页。然后 `refuseLocked`（`rewrite.go:136-145`）：要改写的页里有被编辑的，答 `linking.pages_locked`，由用例带上 `op = FirstMove`（4.11 的"界线与锁"），文字说"the batch's renames and moves"而不是 `FirstMove` 的 `from`（到了被锁的页的不一定是它，审查 A-N14）。然后逐页解析与追加：解析的 503、守卫的 `page.locked`（译成 `linking.pages_locked`，`guardLocked`）。所以单元里的次序是：改写的页数的 422、改写的链接数的 422、改写的页的字节的 422、`linking.pages_locked`，然后逐页改写时，改写过的页留下的份额旁放不下的 422 或 503。

**锁下再核对**（4.11）：没写的页的链接由推后的调用在锁下、在真的树上从索引改写；写了的页里它最后一个正文操作之后的改名、移动碰到的链接，也在锁下从那一项时的去向改写（`W.Before` 是计划在不加锁的树上算的，重做核对过 batch 自己的节点）。计划在更早的项为一页选的链接的文字，锁下不再核对：计划之后、单元之前别人的写让它改了去向的，同 batch 前面的项让它改了去向的一样，之后的改名、移动看不见（4.11 的规则）。

#### 3.7.3 `PathBefore` 从根撤回

今天的 `PathBefore` 在路径上碰到第一个改动就返回（`linking/domain/change.go:101-122`）：先把文件夹 F 改名 G、再把其下的 X 改名 Y，`[[X]]` 写成 `[[Y|X]]` 而不是 `[[Y]]`。改为：在节点现在的路径上找**最深的一个被移动的**节点 m（父节点变了）：m 之前的路径是它原来的父节点在单元开始时的路径（`Moved.Parents`，3.6.2 在锁下、任何一步之前读出，含之后被这个 batch 删除的，`Pages.Paths` 从不列出已删的，`linking/app/ports.go:193-203`）加上 m 之前的名称；m 之下的各段的父节点没变，名称换成各自之前的名称（改名过的）；路径上没有被移动的，从根起每段换成之前的名称。

```go
// PathBefore is n, at its path after changes, at its path before them
// (M9/P3 design 3.7.3): below the deepest node on its path that changes
// moved, the path parents gives that node's former parent, then its name
// before; each node under it, and each on a path no change moved, with
// its name before. parents are the former parents' paths as the unit
// began (FormerParents' nodes), read before any of its ops. A page off the
// nodes changes relocated has the path it has, a copy. It indexes changes
// for n alone: the paths of many nodes take BeforesOf's, built once.
func PathBefore(n Node, changes []Change, parents map[uuid.UUID][]Step) Node
```

一次改写为它碰到的全部目标只把改动按节点索引一次（`Befores`，`BeforesOf(changes)`），目标去重之后各算一次"之前"的路径；`PathBefore(n, changes, parents)` 留着，是 `BeforesOf(changes).PathBefore(n, parents)`（S1 审查 B-I1）。每条碰到的链接都重建一次整个改动的表时，一次移动一个 1 万个节点的文件夹（1 万个改动）、400 页各 100 条链接进到其中，是 4 万次重建：单个的 `move` 与 REST 的比 main 慢约四倍，到了 15 秒的期限，batch 的改写的页数的 422 要约 15 秒之后才答。`TestTheTreeOfALargeMoveIsBuiltOnceForItsLinks`（1 万个被移动的节点、4 万条链接）断言 `treeOf` 的分配至多 64 MiB：现在约 20 MiB，每个目标重建一次时约 17,918 MiB（每条链接一次时约 71,640 MiB）。S1 审查 B 的 REST 的差分测试（4,000 个随机的世界）在修好之后与 main 的输出逐字节相同。

**冻住的节点的名称**（S1 审查 B1-N1）：`update_links` 为假的操作改名、移动的节点不在交给参与者的改动里（4.11），它们之下被跟随的节点的"之前"的路径就带着它们现在的名称与父节点：先把 `Top/F` 移到 `G/F`、再把其下的 X 改名 Y（为假），Q 的 `[Top/F/X/D](Top/F/X/D.md)` 的文字不是 D 有过的路径，留着，写成 `[Top/F/X/D](D.md)`；分开调用写成 `[D](D.md)`。所以 batch 另交它们在整个 batch 里合并的改名、移动（`BatchRewrite.FrozenMoves` → `Moved.FrozenMoves`，由 `BatchPlan.FrozenMovesBetween` 给），`treeOf` 把它们与改动一起索引（`BeforesOf`）、取它们原来的父节点（`FormerParents`），只为"之前"的路径，不跟随；用例在单元开始时也读这些父节点的路径（`Parents`）。写了的页与计划里的改写的"之前"取自那时的树，本来就对。

单个操作的单元（REST 与 MCP 的 `move`）只有一个改动，新旧写法的结果相同（M6 的改写的样例与 `TestRelocationIsWhatARewriteFollows` 照旧通过）；`Parents` 为空时参与者照今天读现在的树（`rewrite.go:169-172`）。

#### 3.7.4 `BatchLinks`：计划里的改写

计划的改写由 linking 做（链接的解析与改写是 linking 的，4.1），page 经端口：

```go
// BatchLinks resolves and rewrites links for a batch's plan, on the tree
// the plan simulates (M9 design 4.1, 4.11; M9/P3 design 3.7.4):
// linking's, which bootstrap adapts from linking.NewBatchLinks(pool). It
// reads the index's rows and aliases as the batch began, on the pool; the
// tree and the parse are its caller's.
type BatchLinks interface {
	// Indexed is where the links of the page id led by the index, at its
	// revision, with their targets at their paths on tree, the plan's
	// before the batch: false when the index is not of revision or not of
	// the current extractor (M6's skips), its links then not rewritten.
	Indexed(ctx context.Context, notebookID, id uuid.UUID, revision int, tree domain.PlanTree) (domain.LinksBefore, bool, error)
	// Reached reports whether changes, the renames and moves of a batch's
	// ops before a page's first content op, a write, reach a link of the
	// page id by the index at revision, on tree after them, as Rewrite
	// would write it again (M9/P3 review A-I1), those that led to the
	// nodes of frozen left, the nodes of createdAfter out of the subtrees
	// of the nodes they key (BatchRewrite.CreatedAfter): the plan then
	// reads the page whole and compares the write with its content
	// rewritten for them. False when the index is not of revision or not
	// of the current extractor, as Indexed. What it reads of tree it keeps
	// in reads.
	Reached(ctx context.Context, notebookID, id uuid.UUID, revision int, changes []domain.Change, frozen []uuid.UUID,
		createdAfter map[uuid.UUID][]uuid.UUID, tree domain.PlanTree, reads *TreeReads) (bool, error)
	// Resolve is where each link of q's content, the page's at its path on
	// q's tree, leads there, by where its target starts, with the targets
	// as they are there.
	Resolve(ctx context.Context, q LinksOn) (domain.LinksBefore, error)
	// Rewrite is q's content with its links that led where q.Before tells
	// written again, as the participant writes them, so that each leads
	// where it led on q's tree after q's changes.
	Rewrite(ctx context.Context, q LinksMoved) (string, error)
}
```

`LinksOn` 带笔记本、页、正文、`parse` 与模拟的树的一个视图（`PlanTree`：按键、按 id 给路径、子树，`(*BatchPlan).TreeAt(k)`；与这个 batch 在第 k 项及以前删除的页（`PlanTree.Gone`），`resolutions` 的 `gone`：它们的别名行当作没有，3.7.2；`LinksOn` 不另带 `Gone`，linking 的适配从这个视图读它们，实施的偏差 11）；`LinksMoved` 另带"之前"、其间合并的改动、其间 `update_links` 为假的操作改名或移动的节点（`Frozen`：通向它们的链接不改，3.7.2）、每个被改名或移动的节点在其间它最后一次这样的操作之后新建在它下面的节点（`CreatedAfter`，3.7.2）与这棵树的读（`Reads`）。原来的父节点的路径不带：一页的改写的"之前"的路径来自它的 `Before`，不读被移动的节点原来的父节点（实施的偏差 12，S1 审查 B-N1：以前算好、换好却没人读）。**一棵树读一次**（S1 审查 B-N2）：计划的每一棵树（每个改名、移动与删除的项之后的那一棵）有一份 `TreeReads`，两个这样的项之间的正文操作的 `Rewrite` 与之后写的页的 `Reached` 共用它，其间的新建不另起一份（S1 复核 A 的 F2-1）。新建不改这份读里的任何东西：窗口里被改名、移动的节点的子树读在新建之前，少的只是在它最后一次改名、移动之后新建的节点，它的展开本来就不要它们（3.7.2）；之后才第一次读的子树取自更早的树，差的也是这些节点；解析、读整页的判断与写出用的都是这一次调用自己的树。在一次之后的移动之前新建的，那次移动另起一份读。以前新建也另起一份：一次改名一个 10 万个节点的文件夹、之后 49 次新建一页再 `write` 一个带链接的页，每次都重新展开，3.77–3.90 秒、分配 15.2 GB；现在 1.02–1.06 秒、1.22 GB，计划的时间（0.17 秒）同没有新建时。linking 把 `spread` 读的子树与它们的别名键存在里面（`BatchLinks.subtreesOf`），还有最后展开的结果（触及，S1 审查 A 的 N1）。移动一个 1 万个节点的文件夹之后的 33 个小页的 `edit`，以前每项各读一遍那棵子树与 1 万个节点的别名键，现在读一遍（同一次会话：调用从 698–798 毫秒到 267–274 毫秒，分配从 1,243 MiB 到 235 MiB）。计划只留它最后问的那一棵树的一份（`readsAt`）：读的检查与正文的一遍各按项的次序问它的树，过了一棵树就丢它的读，正文的一遍再问时重新读（修正的第 3 轮。每棵树的都留着时，50 次改名一个 10 万个节点的文件夹、每次之后改一个带链接的页，堆的峰值 +1.1–1.45 GB；3.7.2）。"之前"的 `Leads` 带 `Ambiguous`（`Lead`，审查 A-M3）：只有 id 时，之前与之后都是歧义的链接会被改写成路径的写法，下一项 `old_string` 里的 `[[X]]` 就找不到了。组合根的适配把 page 的类型换成 linking 的（`PlanTree` 换成 linking 的 `Pages`，`Facts` 换成 linking 的 `domain.Facts`，同观察者与参与者的换法）；linking 的 `BatchLinks` 以同一组函数做：`resolutions`（`linking/app/resolver.go:54-…`）、`domain.Rewrites`、`domain.Rewrite` 与 `Written`（`linking/domain/rewrite.go:58-170`）、改过的 `PathBefore`。只凭连接池：它读索引的行与别名（`Store`），树由调用者给（4.1："只凭连接池的 `linking.NewBatchLinks(pool)`"）。计划里看不到 batch 前面写进 frontmatter 的别名（它们不在 batch 之前的索引里），写进工具的说明（4.11）。

### 3.8 界线

13.1 第 31 条，每个上限处有测试；超过的都是 422 `page.batch_too_large`，文字给出哪一项、多少与上限，什么都不写（4.11）。

| 量 | 上限 | 数在哪里 | 码的次序里的位置 |
|---|---|---|---|
| 项数 | 1–100（`MaxBatchOps`；0 项是 `validation_failed` `ops` `too_short`） | `CheckBatch` | 取值的 422，最先 |
| 字符串合计 | 6 MiB（`MaxBatchStringBytes`），其中正文与 `new_string` 5 MiB（`MaxContentBytes`）；每项另照工具的（`old_string` 1 MiB） | `CheckBatch` | 取值的 422 |
| 条目 | `MaxUnitNodes` = 10,000（新建、删除的子树里的、改名与移动的节点） | 第一遍里边走边数 | 第一遍结束时、任何读之前，答在过界的那一项；先于第二遍的错误（3.4.3） |
| 移动带着走的节点 | 不止一次移到别的父节点下时，这些移动的子树之和至多 `MaxUnitNodes` = 10,000：每次移动各算一次，来回移动同一个文件夹也是（`move F → G`、`move G/F → F` 算两次 F 的子树）；只有一次这样的移动时不限，同单个 `move`；同一父节点下的改名不带走什么 | 第一遍里边走边数，在那次移动自己的检查（`RelocationOf`：环、名称、层级）之后（S1 审查 A 的 N4：过界的那次移动是环时答它的 409，同条目的界线等它的那一项） | 过界的那一项，第一遍到此为止，先于第二遍的错误（S1 审查 A-I2：50 次来回移动一个 10 万页的文件夹，单元持着笔记本的行 12 秒） |
| 读的页 | `MaxBatchReadBytes` = `MaxUnitBytes` + `MaxBatchStringBytes` = 14 MiB：第一遍出错的项之前、读正文的页（第一个正文操作是 `edit` 的，或是 `write` 而之前的改名、移动经索引碰到它的链接的，3.4.4，S1 审查 A-I1）的大小（`PageStates` 的 `byte_size`），各页一次；只读头的页（别的 `write`，带不带 `base_version`）不数；只有一页时不限；过界时，读整页的 `write`（之前的改名、移动碰到它）在加起来的页里时，提示说为什么："a write after a rename or move that reaches its page reads the page whole: send the renames or moves in another batch"；否则 "split the batch"（S1 审查 A 的 N3） | 第一遍之后、任何正文的读之前 | 过界的那一项；条目之后，先于第二遍的错误（3.4.4） |
| 计划的解析 | `MaxBatchParseBytes` = 2 × `MaxContentBytes` = 10 MiB：计划在最后的正文之前解析的（被改写的正文、读回的写法、"之前"），按不同的（页、哈希）；只有一页时也限。等待：与最后的正文的 `DecidedAll` 共用一个合计的 `parse_max_wait`（2 秒，一次请求一份，跨过重新计划；每次计划各数各的，3.4.5） | 计划里每次解析之前：会过界的解析不做 | 要它的那一项（3.4.4） |
| 页数、字节 | `MaxUnitPages` = 100、`MaxUnitBytes` = 8 MiB（`page/domain/trash.go:46-73`）：计划的解析与最后的正文按（页、哈希）数一次；只有一页时不限：最后的正文至多 5 MiB，计划的解析受上一行限（`UnitTooLarge` 加不同的页数的参数，3.18.4） | 计划里每次解析之前与计划之后 | 计划里的那一项；计划之后 |
| 链接 | `MaxUnitLinks` = 20,000：计划里与最后的正文的解析的链接之和 | 解析之后：会话的 `DecidedAll` 边解析边数，过界时答它的界线（`func(links int) error`，拒绝说出条数，实施的偏差 13） | 503 之后 |
| 改写的页 | `MaxBatchRewrites` = 400（实测，3.8.2）：没写的页里有碰到的链接的，加上写了的页里它自己的改动碰到它的链接的（S1 审查 B1-M2），都只算有一条通向 batch 没删除的页的链接的；写了的页每页算 `WrittenRewriteWeight` = 2 页（S1 审查 B-M3，3.8.2）；`Moved.MaxPages` 为 0 是不限（REST 与单个 `move`） | 推后的参与者里：`reached` 与索引的过滤之后、在读改写要的树之前（S1 审查 B-I1）、`refuseLocked` 与任何解析之前 | 单元里，先于 `linking.pages_locked` 与 503（3.7.2） |
| 改写的链接 | `MaxBatchRewriteLinks` = 120,000（实测，3.8.2）：上一行那些页里要改写的链接，没写的页里 `domain.Rewrites` 要写的、写了的页里它自己的改动碰到的，都只算通向 batch 没删除的页的；`Moved.MaxLinks` 为 0 是不限（REST 与单个 `move`） | 推后的参与者里，数过页数之后、读改写要的树之前（S1 审查 B1-M3） | 单元里，改写的页数之后，先于 `linking.pages_locked` 与 503（3.7.2） |
| 改写的页的字节 | `MaxBatchRewriteBytes` = 400 × 84 KiB = 34,406,400（实测与预算，3.8.2）："改写的页"一行数的那些页的内容的字节，按页的头里的大小，写了的页按它最后的内容、算一次；`Moved.MaxBytes` 为 0 是不限（REST 与单个 `move`） | 推后的参与者里，数过链接之后、读改写要的树之前（S1 复核 B 的 B2-M1） | 单元里，改写的链接数之后，先于 `linking.pages_locked` 与 503（3.7.2） |
| 改写过的页留下的解析额度 | 配置的解析预算（`page.parse_budget_bytes`，默认 8 MiB）：batch 改写过的页的事实留到单元结束的份额，加上下一次解析要取的；单个 `move` 不查 | 推后的参与者里，逐页改写时、每次解析取预算之前（S1 复核 B 的 B3-M1） | 单元里，`linking.pages_locked` 之后，与 503 同处（3.7.2） |
| 重新计划 | 1 次（同一个解析的会话，3.4.5） | 用例 | — |
| 请求体的 JSON 值 | `mcpserver.MaxBodyValues` = 10,000，在边缘（`mcpserver/edge.go:56-77`）：最大的 `batch` 约 715 个，带上平的项的全部成员约 1,215 个 | 边缘 | 400，在一切之前 |

**最坏的 batch**（审查 A-I4、A-I5、A2-N3、A3-N2）：一个 batch 解析的正文至多是：

- 只有一页时，计划的 10 MiB、最后的 5 MiB 与参与者对它的改写的读回 5 MiB，共 20 MiB；
- 多于一页时，计划与最后的正文合计 8 MiB，加读回至多 8 MiB，共 16 MiB；
- 重新计划一次（计划之后别人写了 batch 要写的页，3.6.2）：第二次的计划与 `DecidedAll` 再解析至多 15 MiB（一页时 10 + 5），再读一遍树与正文；第一次的单元在重做里停下，不到推后的改写。

写了的页的链接取自 `DecidedAll` 的事实，参与者不再解析它们（3.7.2）。按 `f1733403` 上实测的解析（`BenchmarkParse`，Apple M5 Max：100 KB 8.68 ms、1 MiB 43.0 ms、5 MiB 210.4 ms 与 301.8 MB 的分配），20 MiB 约 0.84 秒、分配约 1.2 GB（活着的只有事实，约十分之一）；按 M4 记下的病态的正文（5 MiB 约 1.4 秒），约 5.6 秒。读：树一条语句（10 万个节点约 70 ms），正文至多 14 MiB（大小先拒绝），合计约 0.2 秒。推后的改写的 R 页以单元在本机约 4 秒之内定档（3.8.2）。

| 最坏的 batch | 正常的正文 | 病态的正文 |
|---|---|---|
| 预算空着 | 约 5 秒 | 约 10 秒 |
| 预算被别人占着：加一个合计的等待 2 秒（计划的解析与 `DecidedAll` 共用，参与者的 `ParseNow` 不等） | 约 7 秒 | 约 12 秒 |
| 重新计划一次，预算空着（再解析 15 MiB 与再读：正常约 0.8 秒、病态约 4.4 秒） | 约 6 秒 | 约 14 秒 |
| 重新计划一次，预算被别人占着（会话跨过重新计划：一次请求一个 2 秒的等待） | 约 8 秒 | 约 16 秒 |

除了最后一行的病态的一格，都在 `server.request_timeout` 的 15 秒之内（`server/configs/config.yaml:17`）。那一格（约 16 秒）是三者都碰上：正文是病态的、计划之后别人写了 batch 要写的页、预算又被别人占着。**接受**（审查 A3-N2，00 号文档第 10 节的风险）：期限到时请求的上下文结束，单元回滚，什么都不写（batch 全成或全不成），答 500 `internal_error`，日志照过了期限的请求记（`platform/httpserver/apierrors.go:272-273`，同今天任何过了期限的调用）；agent 可以再发。只因树被别人改了而重新计划时正文没变，会话重用第一次的事实、不再解析，同前两行。每次计划各等一个 `parse_max_wait` 时这一格约 18 秒，计划与 `DecidedAll` 各等 2 秒、参与者再解析写了的页时不重新计划也约 16 秒（审查 A2-N3）：所以一次请求一个等待，参与者不再解析。

**修正的第 1 轮实测**（2026-10-20，最后的代码（`754c9790`；量的那棵树只差一个测试文件与一处注释），生产的请求期限 15 秒、预算 8 MiB、等待 2 秒，经 MCP 调用；量法同 S1）：一页 5 MiB 的普通正文，改名、编辑、改名（共 20 MiB）调用 1.32 秒、单元 0.45 秒；同样的病态正文 5.2–6.0 秒、单元 1.4–1.6 秒；两页各 4 MiB（链接少）1.11–1.24 秒；读 14 MiB 的三页 52 毫秒；一页 5 MiB 的普通正文加 398 个满负载的页（那一页是写了的页，算 2：界线上，改写 399 页）4.57 秒、单元 3.67 秒；病态正文加 398 页 8.63 秒、单元 4.76 秒。移动带着走的节点在界线上（S1 审查 A-I2）：两个各 5,000 个节点的文件夹移到别处 113 毫秒、单元 91 毫秒；一个 100 个节点的文件夹在两个父节点之间来回 100 次 210 毫秒、单元 197 毫秒。推后的改写在界线上的混合（3.8.2）：99 个写了的页与 202 个索引里的页，每页 100 条链接进到一个被移动的 1 万个节点的文件夹，单元 4.36 秒；只有 400 个这样的索引里的页 4.46–5.17 秒（单元，两次）。都在 15 秒之内。

**链接的多少**（S1 审查 B1-M3，修正的第 2 轮实测，2026-10-20，`09e75680`）：页数的界线之内，单元的用时随碰到的链接。只有索引里的 400 页、链接各约 18 字节（每行一条 `[[…]]`）、一个 1 万个节点的文件夹单独被移动（3.8.2 的链接密的一档，三次的中位数）：每页 100 条 2.14 秒、250 条 3.06–3.09 秒、300 条 3.44–3.45 秒、375 条 4.25–4.28 秒、500 条 4.74–4.82 秒、750 条 6.75 秒、1,000 条 8.30 秒、1,500 条 11.47 秒，2,000 条在 15 秒的期限的边上（一次到了期限，一次 14.97 秒），2,500 条到了期限（S1 审查 B 量得 2,000 条 14.27 秒、2,500 条到了期限，那时笔记本一直锁着）。所以改写的链接至多 120,000（每页 300 条那一档）：过了的答 422，在读树之前（S1 审查 B 的行：500、1,000、2,000、2,500 条的在 0.36–2.11 秒里答）。页的字节另加它的解析：400 页各约 76 KB、100 条碰到的链接（40,000 条，在链接的界线之内）单元 4.2–5.2 秒，过了约 4 秒，在 15 秒之内（主持者接受，修正的第 1 轮的决定；连同上面的曲线留给 M12）。页更重时不只是慢：400 页各一条碰到的链接，每页 256 KiB 时 8.0 秒后答 `server_busy`、重试也一样（S1 复核 B 的 B2-M1）；所以另有页的字节的界线（下面的"**页的字节**"一段）。**单个 `move` 有同样的风险**，不受这两个界线限：MCP 的 `move`（REST 的同一个单元）400 页各 2,000 条碰到的链接，S1 修正的第 1 轮之后 14.52 秒、main 14.57 秒（S1 审查 B），这一轮交替量 head 与 main（负载 5–7）：各一次到了 15 秒的期限，另一次 14.26 与 14.45 秒；每页 500 条时 4.83–4.93 秒，两者相同。**留给 M12 的性能移交**：参与者的一个界线（页数、链接与页的字节），单个 `move` 与 batch 共用（三个数都在 `reached` 之后、读树之前，字节多一条读页头的语句）；很重的页、链接密与页的字节的曲线（上面与下面）；许多次改名之间的带链接的页（下一段）。

**许多次改名之间的带链接的页**（修正的第 2、3 轮实测，2026-10-20，本机；主持者接受，留给 M12 的性能移交）：

- **形状**：50 次改名一个 10 万个节点的文件夹，每次之后 `write` 一个带链接的页（它自己的字节）。
- **用时与分配**：
  - S1 0.81–0.98 秒、分配 0.93 GB；
  - 修正的第 2 轮之后 3.0–3.4 秒、15.2 GB；
  - 第 3 轮之后 3.45–3.78 秒、15.2 GB：回收器更勤，堆的峰值从 +1.09–1.23 GB 回到 +0.27 GB。
- **链接以路径通向那个文件夹时**：改名改写它，读的检查碰到这个 `write`、读整页，写的是改名留下的字节。计划一次只留一棵树的读（3.7.2），正文的一遍为它的改写把读的检查展开过的树再展开一次。
  - 第 3 轮之后：6.83、7.08 秒，分配 31.0 GB，堆的峰值 +0.27 GB。
  - 第 2 轮留着每棵树的读：3.76、3.77 秒，16.7 GB，堆的峰值 +1.36–1.43 GB。
- **为什么**：为知道之前的改名碰不碰到 `write` 的页的链接（S1 审查 A-I1），`Reached` 在每一棵树上把被改名的文件夹展开一次。一次剖析里，计划分配的约 14.5 GB 里：
  - 在计划的树上走它的子树约 12.0 GB：复制节点，切片逐个追加；
  - 触及的三个切片 1.3 GB；
  - 读别名键的语句（每棵树 10 万个 id）0.8 GB。

  S1 的 `write` 只读头，不展开。`edit` 的一行 S1 本来就每棵树展开一次：3.7–4.5 秒、16.7 GB，各轮相同。
- **决定**：在 15 秒的期限之内，batch 有界，v0.1 接受。
  - `write` 的一行是修正的第 3 轮的决定。
  - 再展开的一行是修正的第 4 轮的决定：第 3 轮省下的内存在并发时更要紧。
  - 不做让读的检查把碰到的链接交给正文的一遍。
- **M12 可以考虑的两个改法**（都没有做）：
  - (a) 把触及建成它的大小：每次展开省掉它的切片增长时多分配的（`write` 的一行约 1.3 GB 里的一半；再展开的一行两次展开各省一份）；子树的读与别名键照旧。
  - (c) S1 审查 A 的另一个改法：不展开。按页的每条链接，沿计划的树往上找它的目标与它的键的节点有没有被改名、移动的祖先。工作随链接，不随文件夹的大小，也省掉正文的一遍的再展开。

**页的字节**（S1 复核 B 的 B2-M1，修正的第 5 轮实测，2026-10-20，本机）：

- **为什么要它**：参与者每改写一页，都留着它的事实在解析预算里的份额，约它的字节的十分之一（`FactsRatio` 30 对解析的 300），到单元结束；下一页的解析立即取预算（`TakeNow`：单元握着笔记本的行，不等），取不到就答 503。页数与链接的界线之内，400 页各一条碰到的链接，每页 256 KiB 时 8.0 秒后答 `server_busy`，1 MiB 时 4.1 秒后（S1 复核 B 量的），重试也一样，期间笔记本一直锁着。
- **曲线**（3.8.2 的字节的一档，400 个索引里的页，各一条链接通向一个 1 万个节点的文件夹，它单独被移动；三次的中位数，两次）：每页 76 KiB 3.50、3.54 秒，84 KiB 3.39、3.49 秒，96 KiB 3.42、3.50 秒，112 KiB 3.61、4.08 秒，128 KiB 3.98、4.29 秒，160 KiB 4.71、4.90 秒，192 KiB 5.24、5.75 秒；256 KiB、512 KiB 与 1 MiB 在 5.00、4.70、4.33 秒后答 `server_busy`。一条链接一页时，到 96 KiB 用时由页的写定，之后随字节。
- **界线**：`MaxBatchRewriteBytes` = 400 × 84 KiB = 34,406,400 字节。M8 的规则单独会给 128 KiB 那一档；但 batch 自己要给它还做的最大的解析（一页最大的内容，5 MiB，最后解析）在别的页留着的份额之外留出地方：默认的预算 8 MiB，（界线 − 5 MiB）/10 + 5 MiB 不过它，界线至多 35 MiB；两者之内最大的一档是每页 84 KiB（96 KiB 那一档是 37.5 MiB）。界线上，5 MiB 的页之前的 399 页留着 2,921,877 字节，加它的解析 8,164,757，离预算还有 219 KiB，给前置元数据的值与链接改写后寻常的变长（每条几十字节）；更长的由下一条的检查拒。实测：界线上的字节、一页最大的内容最后改写，3.76 秒做完；96 KiB 那一档的字节这样排，3.15 秒后答 `server_busy`。配置的预算更小时（至少 5 MiB）这个余地也更小，同每一次改写。
- **写出的比页长得多时**（S1 复核 B 的 B3-M1，修正的第 6 轮实测，2026-10-20，本机）：界线数的是页的字节，改写过的页留下的份额随它写出的内容。相对链接与路径形式的链接写出时长出目标新位置的路径，一条至多约 2.5 KiB（10 层、每层 255 字节）。400 页各 150 条相对链接（共 60,000 条、1.25 MB），目标文件夹被移到 7 层 250 字节的名字下面，每条长出约 1,760 字节、每页写出约 266 KB：三个界线之内，以前 2.42 秒后答 `server_busy`，重试也一样（每页 300 条时 2.60 秒，S1 复核 B 量的）。现在由改写里的检查拒（3.7.2）：一次解析在 batch 改写过的页留下的份额旁放不下时，取之前答 422，150 条 2.34 秒、300 条 1.95 秒（之前约 300 页的改写）；预算的一半被别人占着时照旧立即答 `server_busy`（1.13、1.04 秒）；预算 32 MiB 时做完，单元 6.62 与 11.17 秒，后者过了 M8 的约 4 秒，在三个界线之内，默认的预算下被拒。链接的界线（120,000）是以每条约 18 字节的完整路径的链接量的，这样写出的长得多。

#### 3.8.1 `MaxBodyValues` 的对照

`domain.MaxBatchOps` 的注释写这个对照（3.3.2：100 项 `edit` 约 715 个值，带上平的项的全部成员约 1,215 个，是 `MaxBodyValues` 的八分之一），`mcpserver/edge.go:67-72` 的注释已写"P3 的 `batch` 约 700 个值"，不改。整个程序的测试 `TestTheLargestBatchIsFarWithinTheValuesBound`（3.14.6）造最大的合法 `batch`（100 项 `edit`，五个参数都在）与每项带全部 11 个成员的 `batch`，发出去：前者不是 400（成功），后者以 422 `not_allowed` 拒绝（不收的成员），也不是 400。不数值的个数：bodyshape 不导出它的数法（只导出 `Middleware`、`Bytes`、`Table.Check`，`values` 不导出，`bodyshape/ambiguity.go:48-57`），两个请求体发出去就是对照本身（审查 C-M9、C-O2）。

#### 3.8.2 `MaxBatchRewrites`：照 M8 的规则实测

M8 的规则：以一个手跑的基准选档，满负载下单元在本机约 4 秒之内的最大一档；一个守卫在界线处核对用时，断言本机量得的用时的三倍、至多 12 秒，测试的注释写下量得的用时（`bootstrap/revert_cost_test.go:24-31` 的 `BenchmarkRevert`；`TestARevertAtTheUnitsBoundTakesItsTime` 是 4.0 秒的三倍 12 秒，`:565-577`、`:625`；`TestARevertOfManyTopsAtItsBoundTakesItsTime` 是 2.6 秒的三倍 8 秒，`:640-650`、`:672`）。P3 照做（审查 A2-M3）：

- **R 页是满负载的页**：trash 的基准的满载的一页，约 84 KB、200 条链接（8 MiB / 100 页，`bootstrap/trash_cost_test.go:166-170`），M8 的"满负载"；其中一条链到同一个目标页 T。改写一页要解析它两次（它的正文与读回的写法），100 KB 的一次约 8.7 ms，一页约 17 ms：200 页约 3.4 秒、300 页约 5.1 秒、1,000 页约 17 秒。所以预计在 200 一档（M6 的典型的页约 3 ms 一页，不是满负载）。
- **基准** `BenchmarkBatchRewrites`（`bootstrap/batch_cost_test.go`，`//go:build !race`，手跑：`go test -run '^$' -bench 'BenchmarkBatchRewrites' -benchtime 3x ./internal/bootstrap/`）：trash 的基准的满载的笔记本（`newTrashBench`），再以 SQL 建 R 页并重建它们的索引（R 取 100、200、250、300、400、500、1,000；过了界线的档，基准经 `page.Deps.MaxBatchRewrites` 把界线抬高，0 是 domain 的值，生产不设，实施的偏差 7）；一个 batch 是"`edit` 一页 + 把 T 改名 + `edit` 另一页"（推后的参与者改写 R 页；先编辑的那一页 P1 也链到 T，在改名之前写过，推后的改写也数它、改写它，所以一档 n 是 n − 1 个 R 页加 P1，实施的偏差 18）。`newTrashBench` 只挂 page 的 REST 路由（`trash_cost_test.go:135-145`），基准在它的路由上挂 MCP 的服务，在进程里经 MCP 的工具 `batch` 调用例（page 模块只把用例交给它的 MCP 适配器，为测试另开一个取用例的口子就是生产代码；`trashBench` 加了 `cfg`、`mux`、`api`、`pg`，`benchAuth` 把令牌 "pat" 当作基准的用户的个人访问令牌，MCP 不收会话的令牌；实施的偏差 17）：MCP 的适配只加解码与答复的文字，不在单元里。单元的用时：用例在 `Writer.Run` 前后计时，"batch applied" 那一行 info 日志带上它（`unit_ms`，生产里也有用）；基准以自己的日志的处理器读它。每档跑三次，记下单元的用时与整个调用的用时、中位数。`Deps.BatchLinks` 在 `pageDeps` 里设（`bootstrap/deps.go:141-171`），`wire.go:155` 与基准都经它（3.1）。
- **定档**：单元的用时在本机约 4 秒之内的最大一档（M8 的 4.0 秒的单元量得 3.9–5.2 秒：以 5 秒定档，本机就在边上）。S1 的实施在 5.1 的第 12 步量（基准要 batch 已经能跑），写进审查记录、`MaxBatchRewrites` 的注释与守卫的注释。量得（2026-10-20，Apple M5 Max，`-benchtime 3x`，三次的中位数，单元的用时）：100 页 0.86 秒、200 页 1.90、250 页 2.24、300 页 2.70、400 页 3.56、500 页 4.44；1,000 页答 `server_busy`（改写的页的事实的份额合计过了 8 MiB 的预算）。照 M8 的规则取 400（`MaxBatchRewrites`，`page/domain/batch_plan.go`）。
- **写了的页算 2 页**（S1 审查 B-M3）：基准另有混合的一组（`mixed`，每次在一个新做的基准上，不计时）：99 个写了的页（100 项里与那次移动一起最多写这么多）与索引里的页，每页约 76 KB、200 条链接，其中 100 条进到一个 1 万个节点的文件夹里各不相同的节点，那个文件夹在编辑之后被移动。一页算一页时，99 个写了的页加 301 个索引里的页在界线之内，单元 5.59 秒，过了 M8 的约 4 秒。两半各量（两次，各三次的中位数）：只有 99 个写了的页 2.21、2.12 秒，只有 400 个索引里的页 5.17、4.46 秒：写了的一页在单元里（编辑与改写）约是索引里的一页的 1.4–1.9 倍（第一次扣掉移动本身的 0.69 秒是 1.4，按整个单元算是 1.7 与 1.9；只有 99 个写了的页的一组含它们的编辑；S1 审查 B 在一次读树之后把改写单独量出约 0.7–0.8 页，S1 审查 B1-N3），进位为 2（`WrittenRewriteWeight`，`page/domain/batch_plan.go`；`BatchRewrite.WrittenWeight` 交给 linking 的 `Moved.WrittenWeight`，0 算 1）。照这样数，界线上的混合是 99 个写了的页加 202 个索引里的页，单元 4.36 秒。之前先照 S1 审查的第二个改法让写了的页只读一次树（`readWritten`：一条 `Paths` 读全部目标与这些页，一条 `ByKeys`；3.7.2）：读从每页两次到一共两次，S1 审查 B 的混合（99 + 300）从 5.56–5.61 秒到 5.32–5.52 秒（同一次会话）：用时随目标的个数，不随读的次数。每页 100 条碰到的链接时，只有索引里的页的 400 页也要 4.46–5.17 秒：按页数的界线管不到链接的多少，所以另有链接的界线（下面的"**改写的链接**"一条）；那一行（约 76 KB 的页、40,000 条链接）在链接的界线之内，仍过约 4 秒：主持者接受（修正的第 1 轮的决定），连同链接密的曲线留给 M12（3.8）。
- **守卫** `TestABatchAtItsRewriteBoundTakesItsTime`（同一个文件，守卫那一份单独一行）：只以 SQL 建一个笔记本的 R 页、T 与要改的两页；P1 在改名之前写过，是写了的页，算 `WrittenRewriteWeight` 页（实施的偏差 18：设计的"R = `MaxBatchRewrites`"没数它，照它会被拒）。R = `MaxBatchRewrites` − `WrittenRewriteWeight` + 1 = 399 时数得 401，过界一页，答 `page.batch_too_large`，文字说 "401 pages whose links the batch's renames and moves would rewrite, each page the batch wrote counting 2, at most 400"，什么都没写（目标的名称、R 页的正文与变更集的数目不变；参与者在读树与解析之前就数过了，3.7.2）；删掉一个 R 页之后改写 399 页（398 个 R 页与 P1），单元的用时不超过定档时量得的 3.56 秒的三倍、至多 11 秒（读 "batch applied" 那一行的 `unit_ms`），整个调用成功。修正的第 1 轮量得单元 3.26、3.32 秒，测试约 6.5 秒；按守卫那一份的比 2.4，CI 上单元约 8 秒。按耗时判定，`-short` 下跳过（第 11 节第 38 条）。
- **改写的链接**（S1 审查 B1-M3）：基准另有链接密的一组（`dense`，每次在一个新做的基准上，不计时）：以 SQL 建 400 个索引里的页（`MaxBatchRewrites`），每页 n 条只有链接的行（`[[Top/F/X…]]`，各通向一个 1 万个节点的文件夹 `Top/F` 里不同的节点），重建它们的索引；batch 只把 F 移到 G（单元只改写这些页）。n 取 100、250、300、375、500、750、1,000、1,500、2,000、2,500；答复不是 200 或是错误的一档记下它、跳过（2,500 到了请求的 15 秒；2,000 在它的边上）。数在 3.8 的"链接的多少"；照 M8 的规则取单元约 4 秒之内的最大一档，300 条，`MaxBatchRewriteLinks` = 400 × 300 = 120,000（`page/domain/batch_plan.go`；S1 审查 B 估计约 200,000，它在 500 条量得的 4.15 秒这里是 4.74、4.82 秒）。`Deps.MaxBatchRewriteLinks` 为 0 是 domain 的值，基准为更高的档把它抬高。测试 `TestABatchPastItsLinksBoundIsRefused`（同一个文件，在守卫那一行，`-v`）：400 页各 300 条，其中一页另有一条通向 `Top/F/Extra`，共 120,001 条，答 ops[0] 的 422、说 120001，什么都没写；先删掉那一条的目标，界线上改写 120,000 条（单元 3.51 秒，只打出、不判：守卫照旧只在页数的界线上判用时）。
- **改写的页的字节**（S1 复核 B 的 B2-M1）：基准另有字节的一组（`heavy`，每次在一个新做的基准上，不计时）：以 SQL 建 400 个索引里的页，每页一条链接（`[[Top/F/X…]]`，各通向一个 1 万个节点的文件夹里不同的节点）、其余是纯文字，到每页 76 KiB、84、96、112、128、160、192、256、512 KiB 与 1 MiB，重建它们的索引；batch 只把 F 移到 G。另两档把一页最大的内容（5 MiB）放在最后改写（`heaviestLast`），其余的页平分剩下的字节：界线上的字节与 96 KiB 那一档的字节。答复不是 200 或是错误的一档（`server_busy`）记下它、跳过。数与界线在 3.8 的"**页的字节**"。`Deps.MaxBatchRewriteBytes` 为 0 是 domain 的值，基准为更高的档把它抬高；基准的各档在自己的一次里做它的基准，`-bench 'BenchmarkBatchRewrites/bytes'` 只跑字节的各档。测试 `TestABatchPastItsBytesBoundIsRefused`（同一个文件，在守卫那一行，`-v`）：400 页共界线的字节再多一个字节，最后一页 5 MiB，答 ops[0] 的 422、说 34406401，什么都没写（65、67 毫秒）；batch 先把 R0 改短一个字节，界线上改写 400 页，最后的 5 MiB 也在预算之内（单元 3,656、3,876 毫秒，只打出、不判）。基准另有长路径的形状（`longPaths`，S1 复核 B 的：1,000 个节点的文件夹 `Top/F`、7 层 250 字节的名字的文件夹、400 页各 n 条相对链接 `[r](./Top/F/Xk.md)`，batch 把 F 移到那 7 层之下），可以带配置（`newBatchBenchOf`），留着它的预算。测试 `TestABatchWhosePagesOutgrowTheParseBudgetIsRefused`（同一个文件，在守卫那一行，`-v`）：每页 150 条，配置的预算（8 MiB）、等待抬到一分钟，答 ops[0] 的 422、说预算，什么都没写，在请求的 15 秒之内（先等再判的会到请求的期限）；预算的一半被别人占着时，同一个 batch 立即答 ops[0] 的 `server_busy`，什么都没写；预算 16 MiB 时改写 400 页（S1 复核 B 的 B3-M1）。测试 `TestABatchCountsThePagesItWroteAtTheirFinalSize`（同一行）：batch 写了的页按它最后的内容数，改大与改小各一次（S1 复核 B 的 B3-N1）。

单个 `move` 不受这三个界线限，也不查改写过的页留下的份额（同 REST，M6/P4 实测 5,000 页到 15 秒的期限，[M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 3 项）；碰到的链接密时、页重时它有同样的风险：400 页各 2,000 条碰到的链接，单个 `move` 在 main 与修正的第 2 轮的代码上都要 14.3–14.5 秒，负载高时到了期限（S1 审查 B1-M3）；400 页各 256 KiB、一条碰到的链接，M6 起就在约 4.8 秒后答 `server_busy`（S1 复核 B 的 B2-M1）；长路径的形状（每页 150 条）上 2.45 秒后答 `server_busy`（修正的第 6 轮）。留给 M12：一个参与者的、单个 `move` 与 batch 共用的界线（页数、链接与页的字节，连同写出的长度与界线的提示；3.8）。

### 3.9 答复、错误与码的次序

#### 3.9.1 答复

```go
// batchAnswer is batch's answer (M9 design 4.7, 4.11; M9/P3 design
// 3.9.1): the changeset's id and time, null when nothing changed; building,
// whether it queued nodes for the search, in bulk, which the search finds
// once its index has taken them (3.10.1); the ops' results in their order,
// as many as fit, with the nodes each write created; results_count all of
// them, the rest read with changeset.
type batchAnswer struct {
	ChangesetID  *string    `json:"changeset_id"`
	At           *time.Time `json:"at"`
	Changed      bool       `json:"changed"`
	Building     bool       `json:"building"`
	Results      []opResult `json:"results"`
	ResultsCount int        `json:"results_count"`
}

// opResult is an op's result, its tool's answer without the changeset
// (the batch's): path, the node's after the batch, a delete's when
// deleted, and at the op for a page a later op deleted; a content op's
// version, its page's as the batch left it, or as the op's write left it
// for a page a later op deleted; changed, what the op's tool would answer
// at its step; created, the nodes a write created, root first, at their
// paths after the batch, created_count all of them; a delete merged into
// a later one, merged_into, with no pages or attachments of its own.
type opResult struct {
	Op           string        `json:"op"`
	Path         string        `json:"path"`
	NodeID       string        `json:"node_id"`
	Changed      bool          `json:"changed"`
	Version      *string       `json:"version,omitempty"`
	Created      []createdNode `json:"created,omitempty"`
	CreatedCount *int          `json:"created_count,omitempty"`
	Replacements *int          `json:"replacements,omitempty"`
	Pages        *int          `json:"pages,omitempty"`
	Attachments  *int          `json:"attachments,omitempty"`
	MergedInto   *int          `json:"merged_into,omitempty"`
}
```

- **各项的成员**同它的工具（P2 3.11）：`write` 有 `version`、`created`、`created_count`；`edit` 有 `version`、`replacements`；`move` 只有路径与节点；`delete` 有 `pages`、`attachments`，被规范化的删除是 `merged_into: j` 与两个 0（后面那一项的数目含它，4.11）。`changed` 是那一项的工具在那一步会答的（同分开调用：`write`、`edit` 的内容相同、`move` 原地同名是 `false`），不看之后的项把它抵消了没有（3.6.3）。整个 batch 什么都没留下时没有变更集（单元第一次写入才建，`unit.go:339-365`；回到原处的条目删去之后空了的也删去，3.5.4），`changeset_id`、`at` 是 null、`changed` 为假。
- **`building`**：batch 为搜索入队了节点（批量，3.10.1）：它写的要等搜索的任务建好才搜得到，其间搜索也答 `building`。没留下改动的，与只有移动、排序而推后的改写没有写页的（不入队），为假；推后的改写写了页的，那些页入队，为真（S1 复核 C 的 N1-f1）。
- **`Fit`**（4.7："答复总留着变更集的 id 与时刻……各步的结果放得下多少放多少、标出截断"）：项与 `write` 建的节点按次序排成一列（第 0 项、它建的节点、第 1 项……），`Fit(n, build)` 在这一列上找最大的 k（`mcpserver/answer.go:127-187`）；`build(k)` 依次放进项与它建的节点直到 k 个，被截的项的 `created` 少于 `created_count`、`results` 少于 `results_count`，两处都是截断的标记。k = 0 的答复只有变更集的 id 与时刻、`changed`、`building` 与 `results_count`，约 150 字节，总放得下；一项的结果最坏约 16.8 KB（一条由带 `&` 的标题组成的最长的路径：`TextBytes` 把 `&`、`<`、`>` 各数 6 个字节，P2 3.10.3 的最坏的路径），`Fit` 照样截。100 项的平常的答复约 15 KB，放得下。
- **路径**：单元结束时节点的路径（`NodePaths.Paths`，单元的事务里一条语句，3.6.3），`created` 也是单元结束时的（之后被移动了的新建的页答移动之后的）；删除的项答它被删时的路径（单元里重做那一项时找到的）；之后被删除的页的正文操作答它在那一项时的路径与它合成的写留下的版本（3.6.3）。

#### 3.9.2 错误的写法

**用例**：每一项的错误包一层，带上第几项与它的工具：

```go
// OpFailed is an op of a batch that failed, the batch written in none of
// it (M9 design 4.11): Op, its place from 0, and its tool, none for an op
// whose op names none; Err as that tool's use case has it, its 422s'
// fields at ops[Op].
type OpFailed struct {
	Op   int
	Tool OpKind
	Err  error
}

func (e *OpFailed) Error() string { return e.Place() + ": " + e.Err.Error() }
func (e *OpFailed) Unwrap() error { return e.Err }

// Place is where the op is in its batch, as an answer says it: "ops[3]
// (move)", or "ops[3]" for an op of no tool.
func (e *OpFailed) Place() string

// ErrBatchTooLarge: a batch is past one of its bounds (M9 design 4.11,
// 4.14): MCP's own code, which no REST operation answers. BatchTooLarge
// returns it saying which bound.
var ErrBatchTooLarge = shared.NewError(shared.KindInvalid, "page.batch_too_large", "The batch is too large for one change.")

// BatchTooLarge is ErrBatchTooLarge saying which bound (M9/P3 design
// 3.9.2): n of what, a count with its unit ("ops", "nodes changed",
// "bytes of strings"), at most max, and what to do, hint, when there is
// something.
func BatchTooLarge(what string, n, max int, hint string) error
```

`BatchTooLarge` 的文字是 "n what, at most max: hint."（没有 hint 时到 max 为止）：3.9.2 原来没定写法，下表的文字都是这个样子（实施的偏差 15）；`Place` 是答复里的位置 "ops[3] (move)"（`ToolError.At`，下面）。

`page/domain/errors.go:101-106` 今天的 `ErrBatchTooLarge`（后台任务减半的哨兵，不是 API 的错误）改名 `ErrJobBatchTooLarge`，每一处用到的都改：根的再导出（`page/tree_writes.go:57`，与 `:114` 的注释）；`page/app/jobs.go:363-478` 的四处（两处是交给 `DecidedAll` 的 `tooLarge`，`:402`、`:478`）；`page/app/jobs_test.go:462-486`、`:746`；`bootstrap/background_cost_test.go:303`；`bootstrap/background_test.go:269` 的注释；组合根的映射（`bootstrap/transfer.go:231-232`）。transfer 自己的 `ErrBatchTooLarge`（`transfer/app/import_ports.go:170`）不动。**改名是 S1 单独的一个提交，在新的哨兵之前**（5.1 第 1 步）：新的 API 的哨兵在同一个包里拿旧的名字，漏改的一处照样编译，却悄悄地和 API 的错误比，后台任务的减半就停了。改名的提交里没有叫 `ErrBatchTooLarge` 的东西，漏改的编译不过。

**适配器**：`batchErr(err, c, in.Ops)`：`*OpFailed` 照它的工具交给 P2 的写法（`write` 的 `writeErr(e.Err, c, path)`、`edit` 与 `delete` 的 `toolErr(e.Err, c, path)`、`move` 的 `moveErr(e.Err, c, from)`，路径是这一项的参数），再 `atOp`；别的照 `toolErr`。`atOp` 不改内层的文字与 `Fit`，只加两样：

- **`ToolError.At`**（mcpserver 加的成员）：`"ops[3] (move)"`。mcpserver 在答复的文字里把它写在码之后：`<码>: <At>: <其余>`，三种文字都是（`ToolError.Text`、`Fit.Build` 的、问题自己的 `code: detail` 加字段，`mcpserver/errors.go:117-150`）；模块的文字都以 `<码>: ` 起头（`page/adapter/mcp/text.go`、`locks.go`、`write.go`、`move.go` 的每一处），不以它起头的是缺陷，mcpserver 照 `<码>: <At>: ` 加在最前面并记 error。不在适配器里拼：适配器拿不到问题自己的文字（`problemAnswer` 在 mcpserver 里），`Fit` 的每一个 k 也都要带上它。
- **`Extra["op"] = i`**：结构的 `op`（从 0 起），每个 k 的 `Build` 都带（`op` 不是保留的成员，`errors.go:50-52`）。

| 错误 | 码 | `op` | 文字（英文，码与 `At` 之后） |
|---|---|---|---|
| 一项的取值、找路径、规则、409 | 工具自己的 | 那一项 | 工具自己的；字段是 `ops[i].*`（3.2） |
| B1、B3 | `validation_failed` | 那一项 | `ops[i]`：`not_allowed`，"deletes a page that ops[k] created (moved, renamed): a batch may not delete what it creates (moves, renames); leave both out, or delete it in a later call" |
| 给新建的页带 `base_version` | `validation_failed` | 那一项 | `ops[i].base_version`：`not_allowed`，"refers to the notebook before the batch: ops[k] created this page" |
| 锁的预检、守卫拒绝一项的写 | `page.locked` | 那一项（预检：项的次序里第一个写或删除被编辑的页的，删除的子树里的与并进来的删除的也算，3.4.5） | `lockErr` 的（持锁人的名字、"you, in a browser"、不能接管），路径是这一项的；`delete` 的子树列出全部被持着的页（P2 的锁的规则，至多 20 个与总数）；删除的照单个 `delete`："deleting it would end their edits"，按子树在 batch 之前的次序列出 |
| 推后的改写碰到被编辑的页 | `linking.pages_locked` | 第一个 `update_links` 为真的改名、移动 | `lockErr` 的；文字说 "the batch's renames and moves would rewrite the links of pages being edited"，不说 `FirstMove` 的 `from`（碰到被锁的页的不一定是它） |
| 推后的改写过了 `MaxBatchRewrites` | `page.batch_too_large` | 同上 | "N pages whose links the batch's renames and moves would rewrite, each page the batch wrote counting 2, at most M: move with update_links false, or in separate calls"（写了的页不在其中时没有 "each page the batch wrote counting 2"；N 是照 3.8 的数法数的，S1 审查 B-M3） |
| 推后的改写过了 `MaxBatchRewriteLinks` | `page.batch_too_large` | 第一个 `update_links` 为真的改名、移动 | "N links the batch's renames and moves would rewrite, at most M: move with update_links false, or in separate calls"（S1 审查 B1-M3） |
| 推后的改写过了 `MaxBatchRewriteBytes` | `page.batch_too_large` | 第一个 `update_links` 为真的改名、移动 | "N bytes of pages the batch's renames and moves would rewrite, at most M: move with update_links false, or in separate calls"（S1 复核 B 的 B2-M1） |
| 推后的改写的一次解析，在 batch 改写过的页留下的份额旁放不下 | `page.batch_too_large` | 第一个 `update_links` 为真的改名、移动 | "N bytes of the parse budget the pages the batch's renames and moves rewrite would hold, at most M: move with update_links false, or in separate calls"，M 是配置的解析预算（S1 复核 B 的 B3-M1） |
| 条目过了 `MaxUnitNodes` | `page.batch_too_large` | 过界的那一项 | "N nodes changed, at most M: split the batch"（实施的偏差 15） |
| 移动带着走的节点过了 `MaxUnitNodes` | `page.batch_too_large` | 过界的那一项 | "N nodes the batch's moves carry, at most 10000: move each folder once, or in separate calls"（S1 审查 A-I2） |
| 读的页过了 `MaxBatchReadBytes` | `page.batch_too_large` | 过界的那一项 | "N bytes in the pages its ops read, at most M: split the batch"；读整页的 `write`（之前的改名、移动碰到它，3.4.4）在加起来的页里时，提示改为 "a write after a rename or move that reaches its page reads the page whole: send the renames or moves in another batch"（S1 审查 A 的 N3） |
| 计划的解析过了 `MaxBatchParseBytes` | `page.batch_too_large` | 那一项 | "N bytes of contents the plan parses to rewrite links, at most M: send the renames and moves in a call apart from the edits after them"（实施的偏差 15） |
| 多于一页时计划的解析与最后的正文过了页数、字节 | `page.batch_too_large` | 那一项 | 页数是 "N contents of the pages the batch writes and the plan parses to rewrite their links, at most M: …"，字节是 "N bytes in the pages the batch writes and the contents its plan parses to rewrite their links, at most M: send the renames and moves in a call apart from the edits after them"（计划的解析也算在里面，3.4.4；实施的偏差 15） |
| 重新计划之后仍不同 | `page.revision_mismatch` | 第一个不同的项 | 正文操作是 `mismatchText`（带现在的版本；`Why` 照这一项：不带 `base_version` 的 `KeptChanging`、带令牌的 `Changed`、令牌的节点已不是这一页的 `OtherNode`）；树的操作 "the notebook changed while the batch was planned, twice: read it again and send the batch again"；正文操作的路径上已没有页时也是这一句（`BatchChanged`，实施的偏差 10） |
| 项数、字符串、计划之后的页数、字节、链接 | `page.batch_too_large` / `validation_failed`（0 项） | 无 | 哪一项、多少、上限，照 `BatchTooLarge` 的写法（如 "101 ops, at most 100: split the batch"；链接是 "N links in the contents the batch writes and its plan parses, at most 20000: split the batch"，`DecidedAll` 的界线 `func(links int) error` 说出条数；实施的偏差 13、15） |
| 笔记本、403 | 照通用 | 无 | 照通用 |
| 解析的 503 `server_busy` | 照通用（带 `Retry-After`） | 计划的：要那次解析的项；最后的正文的：无；推后的改写的：第一个改名、移动的项 | 照通用 |

整个答复的上限：`At` 与 `op` 加约 40 字节（三种文字各一次）；P2 的最坏的错误答复是 38,522 字节（`TestEachErrorFitsAtItsWorst`，[P2](02-P2-writes-history.md) 3.10.3），带上之后仍在 48 KiB 之内，`batch` 的适配器测试照 P2 的做法以 `ErrorAnswerBytes` 在最坏的取值处核对每一种（3.14.1）。

#### 3.9.3 码的次序

照 4.15 的 `batch` 一条，按 2.1 第 2 条修订（"第一个出错的项定答复"含计划里的解析）：

1. `bad_request`（mcpserver：参数的结构；`ops` 不是数组、项不是对象、项里有 schema 之外的成员）。
2. 取值的 422（`CheckBatch`，不读库）：`ops` 为空（`validation_failed` `too_short`）、多于 100 项（`page.batch_too_large`）→ 字符串合计、正文与 `new_string` 合计（`page.batch_too_large`）→ `message` → 各项按次序：`op`、成员、工具的取值规则。
3. 笔记本：`notebook.not_found`（看不到、已删除）。
4. 403 `forbidden`（`page.batch`，不加锁，在读树之前）。
5. 计划的第一遍（树），按项的次序：第一个出错的项的 404、422（路径的规则、B1、B3、`recursive`、要补建的祖先、名称、给新建的页的 `base_version`）、409（名称、环、层级、令牌的节点不是这一页：给新建的页带令牌的 422 在它之前，3.4.2）；条目过了 `MaxUnitNodes` 的那一项（422 `page.batch_too_large`）。不止一次移到别的父节点下时，这些移动带着走的节点过了 `MaxUnitNodes` 的那一项（422 `page.batch_too_large`，"nodes the batch's moves carry"，S1 审查 A-I2），在那一项自己的 404、422、409 之后（S1 审查 A 的 N4）。条目在这里就答，不再跑第二遍。
6. 读正文的页过了 `MaxBatchReadBytes` 的那一项（422 `page.batch_too_large`），以大小算，任何正文的读之前；读正文的页是第一个正文操作是 `edit` 的，与是 `write` 而之前的改名、移动经索引碰到它的链接的（3.4.4，S1 审查 A-I1）；在这里就答，不再跑第二遍。
7. 计划的第二遍（正文），只跑第 5 步出错的项之前的项，按项的次序，第一个出错的项定答复，比第 5 步的更早时取代它：422（`old_string` 的匹配与替换之后的大小）、409（过时的 `base_version`；`write` 的字节与页相同时是未改动，3.4.4），以及它要的计划里的解析：422 `page.batch_too_large`（`MaxBatchParseBytes`、页数、字节）与解析的 503。
8. 锁的预检：项的次序里第一个写或删除被编辑的页（batch 之前就在的；删除的子树里的，并进来的删除的在内）的项的 `page.locked`：正文操作的说那一页，删除的照单个 `delete` 的文字列出子树里每一页被编辑的（3.4.5，S1 审查 A-M1）。
9. 422 `page.batch_too_large`：页数、字节（最后的正文与计划的解析合起来）。
10. 503 `server_busy`：最后的正文的 `DecidedAll`。
11. 422 `page.batch_too_large`：链接（解析之后才知道）。

读树之后、读头与读正文之前页被删除（第 5、6 步之间），同单元里找到的不同（3.6.2）：从头再计划一次，第二次的第一遍在这一页不在的那一项答它的错误（P2 的路径与提示），第二次还这样就答 409 `page.revision_mismatch`（S1 审查 A-M2：实施的偏差 9 不再有）。

12. 单元里，按发生的次序：409 `page.revision_mismatch`（重新计划一次之后仍不同）、守卫的 `page.locked`（那一项；预检之后才开的会话）；然后推后的改写，依次：422 `page.batch_too_large`（改写的页数，再是改写的链接数，再是这些页的字节，都在读树与任何解析之前数，3.7.2）、`linking.pages_locked`（`refuseLocked`），然后逐页改写时 422 `page.batch_too_large`（一次解析在改写过的页留下的份额旁放不下，取之前判，S1 复核 B 的 B3-M1）或 503 `server_busy`（它的 `ParseNow`，别人的份额挡着），这些的 `op` 都是第一个改名、移动的项（3.7.2）。

一个要被改写的页在它的第一个正文操作之前正被编辑时，答的是那个正文操作处的 `page.locked`（锁的预检），分开调用则在移动处答 `linking.pages_locked`（4.11 的规则的后一半，3.14.4 的 (2)）。只被推后的改写碰到、batch 不写也不删除的页正被编辑时，单元里才查到（第 12 步），所以任何一项的 `page.locked`（第 8 步）都先答，在那个改名或移动之后的也一样（S1 审查 A-M1，照设计留着，4.11 写着）。503 的 `op`：第 7 步的是要那次解析的项；第 10 步的没有；第 12 步的是第一个改名、移动的项（推后的改写是它们的）。

### 3.10 加锁、搜索与事件

**加锁**：`batch` 是改树的单元：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 节点与正文行，正文行按操作的次序（`LockContent`，`FOR NO KEY UPDATE`）→ 搜索的队列（按 `node_id` 升序，`unit.go:434-446`）→ linking 的 advisory lock（观察者）。同一笔记本的别的单元都先锁笔记本行（`FOR SHARE` 与它冲突），心跳与结束只碰会话的行，清理 `SKIP LOCKED`，`reindex` 也先锁笔记本行，所以此刻没有谁持着这本笔记本的正文行，按操作的次序锁不会成环。13.1 第 5 条"在 `FOR SHARE` 下写多页正文的按节点 id 升序"之后加一句："改树的单元（`FOR NO KEY UPDATE`）写多页的正文按它的操作的次序：笔记本行已把这本笔记本的写串起来（M9 的 `batch`）"；`page/app/unit_content.go:38-41` 的注释同此。`batch` 的推后的改写照 M6：参与者在单元里不排队地取解析的预算（13.1 第 19 条）。

#### 3.10.1 搜索：批量入队，答复说 `building`

`batch` 的单元批量入队（`UnitSpec.BulkSearch`：只管入队，`Background` 的别的都不是），单元的 `Outcome.Building` 说它入队了节点，答复的 `building` 照它（3.9.1）：batch 写的要等搜索的任务建好它们的行才搜得到，其间搜索也答 `building`，同后台的作业（M8/P5 3.8）。任务每 5 秒跑一次（`page.search_index_interval`），准确的行先建；最坏的 batch 约 1,600 个节点，按建索引的速度（M8/P5 的 189–253 行每秒，这次第 0 步 249 行每秒）约 6–8.5 秒建完，所以 batch 之后约 5 秒加 1–8.5 秒之内搜得到它写的（S1 审查 C-I2 的窗口）。工具的说明说"What the batch writes reaches the search some seconds after it: building says it is on its way."（3.12.1）。

一个 batch 入队的活着的节点：写了正文的页至多 100 个，新建的空的上级页至多约 1,000 个（每个 `write` 至多补建 10 层），改名的至多 100 个，再加推后的改写写的页，至多 `MaxBatchRewrites` = 400 个（3.8.2）：最坏约 1,600 个。删除的子树入队的节点读时被复查丢掉，同 REST 的一次大的删除（M8 已接受）。准确地入队时它们都是搜索的候选，两种走法都按 `greatest(n.updated_at, c.updated_at) DESC` 排候选（`page/adapter/postgres/queries/search.sql`），而 batch 写的都是最新的，最先被走到：稀有的词的第一页就为空而带游标，答复也不说为什么（S1 审查 C-I2：一个 100 项的 batch 留下 984 个准确的行，紧接着稀有的词的第一页 0 个结果、带游标，`building` 为假，第二页才有那 5 个）。

**量的**（5.1 第 0 步，在 S1 的修正的第 1 轮重量，S1 审查 C-I2）：`BenchmarkSearch`（`bootstrap/search_bench_test.go`，手跑）加环境变量 `NERVE_SEARCH_BACKLOG`（n 的列表，如 `0,100,1600,2200`：一次建索引量各档，实施的偏差 16）：建好索引之后、量每一格之前，在被搜的那本笔记本里取 n 个活着的页以 `bulk = false` 放进队列（不跑任务），并把它们的 `nodes.updated_at` 与 `page_contents.updated_at` 设为现在（同 batch 写的页），量完删掉、时间复原。n = 0、100、1,600（`MaxBatchRewrites` = 400 时的最坏）与 2,200，同一份造的语料（M8/P5 的语料已删，第 0 步造了一份，实施的偏差 16：种子固定，306,000 页，活着的 299,911 页，200 本笔记本；任务以 249 行每秒建了 299,911 行）：

| n | 两种可见范围的 90 个第一页里为空而带游标的 | 比 n = 0 多 | 加上"一本笔记本"（135 个） | 第一页的 p99 的中位数（405 格） |
|---|---|---|---|---|
| 0 | 5 | — | 6 | 34.5 ms |
| 100 | 5 | 0 | 7 | 43.5 ms |
| 1,600 | 21 | 16 | 38 | 69.6 ms |
| 2,200 | 21 | 16 | 38 | 75.7 ms |

三个阶段（没有统计、分析过、对任何参数的计划）的数相同。S1 的实施第一次量时没动这些页的时间，积压落在它们的时间所在之处：n = 1,200、2,200 时是 10、12 个（多 7 个），判为不改；batch 的积压是最新的，那样量低估了它（S1 审查 C-I2）。

**改的条件**（4.11 写着，照旧）：只看相对于 n = 0 的：n = 100 比 n = 0 多出 3 个以上为空的第一页，或 n = 2,200 比 n = 0 多出的为空的第一页多于 M8/P5 在 1,000 个时多出的（17 − 7 = 10 个），`batch` 的单元改为批量入队（`UnitSpec` 加只管入队的 `BulkSearch`，答复的 `building` 照它）；否则不改。这次 n = 100 多 0 个，n = 2,200 多 16 个：**改**。由这一次的数决定，不另问负责人。`TestTheMCPBatchReachesItsUseCase` 断言答复的 `building`、任务跑之前搜不到（答 `building`）、跑了之后找到 batch 新建的三页；第 45 条变异反过来（batch 的单元准确入队，那个测试与 `TestABatchQueuesForTheSearchInBulk` 失败）。

#### 3.10.2 事件

一个单元一次（观察者在单元的最后，`unit.go:450-461`）：`pages` 多于 20 页时为 `null`（`events/app/publisher.go:50-64`），网页整体重读；`links` 同 M6。中间态不发：同一页的几个操作合成一个改动（`Change.Then`）。S2 的修正的第 1 轮起，写过的页只在它的改动有"后"（单元之后还活着）时列出（`bootstrap/events_registrants.go`，S2 审查 B-N3，`TestThePagesEventListsNoPageInTheTrash`）：batch 编辑再删除的页、撤销的撤销里写回再删除的页不再列作写过的；网页照删除读这样的事件（树变了、没有写过的页），M5 起就这样。

### 3.11 日志与配置

- 用例每次成功时记一行 info `"batch applied"`（没留下改动的也记，那时不带变更集；实施的偏差 14）：变更集、项数、写了正文的页数、新建、删除、改名与移动的节点数、推后的改写改写的页数（`Unit.Participate` 答它追加的写的数目）、单元的用时（`unit_ms`，`Writer.Run` 前后，3.8.2 的基准与守卫读它）；只有 id、数目与用时，不记路径、正文与说明（13.1 第 10 条；P2 的 `logged`）。重新计划时记一行 info `"batch planned again"`：笔记本、第一个不同的项的序号与种类（节点、版本号、树的错误）。mcpserver 的调用行照旧（工具名、码、耗时；从不记参数）。
- 日志的测试照 P2 的 `TestTheMCPWritesLogNoPathOrContent`（`bootstrap`，`TestThe…`，第三份）加 `batch` 的一次调用：用例的行与调用行都没有路径、正文与说明的任何字节。
- 没有新的配置：上限都是常量（`MaxBatchOps`、`MaxBatchStringBytes`、`MaxBatchReadBytes`、`MaxBatchParseBytes`、`MaxBatchRewrites`、`MaxBatchRewriteLinks`、`MaxBatchRewriteBytes`，写了的页的 `WrittenRewriteWeight`），同 `MaxUnit*`；`page.Deps` 的 `MaxBatchRewrites`、`MaxBatchRewriteLinks`、`MaxBatchRewriteBytes` 为 0 是 domain 的值，只有基准为更高的档抬高它们，生产不设（实施的偏差 7）。改写过的页留下的份额的检查用已有的解析预算（`page.parse_budget_bytes`，3.8）。

### 3.12 工具 `batch`

#### 3.12.1 规格与说明

`page/adapter/mcp/tools.go` 的 `Tools` 加第 11 个（`tools_test.go:80-113` 断言的确切的清单随之）：

```go
mcpserver.NewTool(mcpserver.Spec{Name: "batch", Title: "Apply several writes as one change", Description: batchDescription,
	Annotations: mcpserver.Annotations{Destructive: true}, Entry: mcpserver.BothEntries, Notebook: true}, t.batch),
```

不是幂等的（同 `edit`：再调一次，`edit` 与新建会答别的），破坏性的，不碰外部。`batchDescription`（`page/adapter/mcp/batch.go`，就是 `tools.json` 的那一段）：

> Apply up to 100 writes as one change, all or none, with a message the history shows. Each op is a write, edit, move or delete: op names the tool and its other members are that tool's arguments; it acts on the notebook as the ops before it left it, their paths and contents. base_version is a page's version from before the batch; a page the batch creates takes none. A refused op writes nothing at all: the error has its op, from 0, and that tool's code. A delete of a page whose descendants an earlier op deleted takes them in one deletion, the earlier op answering merged_into; a batch may not delete what it creates, renames or moves. Unlike separate calls, links follow the batch's renames and moves, merged node by node, from where they led before the batch, or, in a page the batch writes, at its latest write before them (on the tree then, with the aliases from before the batch): what other ops did in between to where they lead (creating, renaming, moving or deleting a page) is not seen, nor any alias the batch changed. A page the batch writes or deletes that is being edited in the web editor refuses at the first op that writes or deletes it, as page.locked; one that only the links' rewrite of its renames and moves reaches is checked after those, as linking.pages_locked at its first rename or move. The answer has the changeset's id and time and each op's result, as many as fit, results_count all of them: changeset reads the rest. What the batch writes reaches the search some seconds after it: building says it is on its way. Each op's changed is what its tool would answer at that step; the batch's tells whether it left a change.

修正的第 1 轮改了两句：锁的一句（S1 审查 A-M1）、搜索的一句（S1 审查 C-I2）。

#### 3.12.2 `api/mcp/tools.json` 的条目

两个数组里都排在第一个（按名称的字母序，`changeset` 之前）。下面的全文是合并时的，与 `tools.json` 逐个成员相同（说明在修正的第 1 轮改了两句，3.12.1）。笔记本入口：

```json
    {
      "name": "batch",
      "title": "Apply several writes as one change",
      "description": "Apply up to 100 writes as one change, all or none, with a message the history shows. Each op is a write, edit, move or delete: op names the tool and its other members are that tool's arguments; it acts on the notebook as the ops before it left it, their paths and contents. base_version is a page's version from before the batch; a page the batch creates takes none. A refused op writes nothing at all: the error has its op, from 0, and that tool's code. A delete of a page whose descendants an earlier op deleted takes them in one deletion, the earlier op answering merged_into; a batch may not delete what it creates, renames or moves. Unlike separate calls, links follow the batch's renames and moves, merged node by node, from where they led before the batch, or, in a page the batch writes, at its latest write before them (on the tree then, with the aliases from before the batch): what other ops did in between to where they lead (creating, renaming, moving or deleting a page) is not seen, nor any alias the batch changed. A page the batch writes or deletes that is being edited in the web editor refuses at the first op that writes or deletes it, as page.locked; one that only the links' rewrite of its renames and moves reaches is checked after those, as linking.pages_locked at its first rename or move. The answer has the changeset's id and time and each op's result, as many as fit, results_count all of them: changeset reads the rest. What the batch writes reaches the search some seconds after it: building says it is on its way. Each op's changed is what its tool would answer at that step; the batch's tells whether it left a change.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "ops": {
            "type": "array",
            "description": "The writes, applied in their order as one change: 1 to 100.",
            "items": {
              "type": "object",
              "properties": {
                "op": {
                  "type": "string",
                  "description": "The tool this op is: write, edit, move or delete. Its other members are that tool's arguments."
                },
                "path": {
                  "type": ["null", "string"],
                  "description": "write, edit and delete: the path, from the notebook's root, as that tool takes it."
                },
                "content": {
                  "type": ["null", "string"],
                  "description": "write: the page's whole content, as it will be stored."
                },
                "base_version": {
                  "type": ["null", "string"],
                  "description": "write and edit: the page's version as read before the batch. None for a page the batch creates."
                },
                "old_string": {
                  "type": ["null", "string"],
                  "description": "edit: the text to replace, byte for byte, which must occur exactly once unless replace_all is set."
                },
                "new_string": {
                  "type": ["null", "string"],
                  "description": "edit: the text to put in its place."
                },
                "replace_all": {
                  "type": ["null", "boolean"],
                  "description": "edit: replace every occurrence of old_string. Default false."
                },
                "from": {
                  "type": ["null", "string"],
                  "description": "move: the path of the page or attachment that moves."
                },
                "to": {
                  "type": ["null", "string"],
                  "description": "move: the full path it will have, never a page to move it into."
                },
                "update_links": {
                  "type": ["null", "boolean"],
                  "description": "move: rewrite the links to what moves so that they follow it. Default true."
                },
                "recursive": {
                  "type": ["null", "boolean"],
                  "description": "delete: delete a page with its children. Default false."
                }
              },
              "required": ["op"],
              "additionalProperties": false
            }
          },
          "message": {
            "type": "string",
            "description": "Why the batch is made, shown with its change in the history: 1 to 4,096 bytes."
          }
        },
        "required": ["ops", "message"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
```

账户入口的条目与它只差 `notebook`：`properties` 的第一个成员与 `required` 的第一项（照 P1、P2 的账户入口的写法，`mcpserver/schema.go:19` 的 `notebookParameter`）：

```json
    {
      "name": "batch",
      "title": "Apply several writes as one change",
      "description": "Apply up to 100 writes as one change, all or none, with a message the history shows. Each op is a write, edit, move or delete: op names the tool and its other members are that tool's arguments; it acts on the notebook as the ops before it left it, their paths and contents. base_version is a page's version from before the batch; a page the batch creates takes none. A refused op writes nothing at all: the error has its op, from 0, and that tool's code. A delete of a page whose descendants an earlier op deleted takes them in one deletion, the earlier op answering merged_into; a batch may not delete what it creates, renames or moves. Unlike separate calls, links follow the batch's renames and moves, merged node by node, from where they led before the batch, or, in a page the batch writes, at its latest write before them (on the tree then, with the aliases from before the batch): what other ops did in between to where they lead (creating, renaming, moving or deleting a page) is not seen, nor any alias the batch changed. A page the batch writes or deletes that is being edited in the web editor refuses at the first op that writes or deletes it, as page.locked; one that only the links' rewrite of its renames and moves reaches is checked after those, as linking.pages_locked at its first rename or move. The answer has the changeset's id and time and each op's result, as many as fit, results_count all of them: changeset reads the rest. What the batch writes reaches the search some seconds after it: building says it is on its way. Each op's changed is what its tool would answer at that step; the batch's tells whether it left a change.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "ops": {
            "type": "array",
            "description": "The writes, applied in their order as one change: 1 to 100.",
            "items": {
              "type": "object",
              "properties": {
                "op": {
                  "type": "string",
                  "description": "The tool this op is: write, edit, move or delete. Its other members are that tool's arguments."
                },
                "path": {
                  "type": ["null", "string"],
                  "description": "write, edit and delete: the path, from the notebook's root, as that tool takes it."
                },
                "content": {
                  "type": ["null", "string"],
                  "description": "write: the page's whole content, as it will be stored."
                },
                "base_version": {
                  "type": ["null", "string"],
                  "description": "write and edit: the page's version as read before the batch. None for a page the batch creates."
                },
                "old_string": {
                  "type": ["null", "string"],
                  "description": "edit: the text to replace, byte for byte, which must occur exactly once unless replace_all is set."
                },
                "new_string": {
                  "type": ["null", "string"],
                  "description": "edit: the text to put in its place."
                },
                "replace_all": {
                  "type": ["null", "boolean"],
                  "description": "edit: replace every occurrence of old_string. Default false."
                },
                "from": {
                  "type": ["null", "string"],
                  "description": "move: the path of the page or attachment that moves."
                },
                "to": {
                  "type": ["null", "string"],
                  "description": "move: the full path it will have, never a page to move it into."
                },
                "update_links": {
                  "type": ["null", "boolean"],
                  "description": "move: rewrite the links to what moves so that they follow it. Default true."
                },
                "recursive": {
                  "type": ["null", "boolean"],
                  "description": "delete: delete a page with its children. Default false."
                }
              },
              "required": ["op"],
              "additionalProperties": false
            }
          },
          "message": {
            "type": "string",
            "description": "Why the batch is made, shown with its change in the history: 1 to 4,096 bytes."
          }
        },
        "required": ["notebook", "ops", "message"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
```

`batchIn`、`batchOp` 的 `jsonschema` 标签就是这些说明（3.3.1 的结构照此写；`TestTheToolListsAreTheContract`，`bootstrap/mcp_tools_test.go:152-160`，比整个工具对象，`required` 的次序也比）。文件照 oxfmt 排版（`make lint-web` 的 `oxfmt --check .`）。MC1 的第 5 步照文件读清单（P2 4.5），不用改。

#### 3.12.3 内置的说明的一行

`page/adapter/mcp/instructions.go` 的 Tools 一段，`delete` 之后、`versions` 之前：

```
- batch: up to 100 write, edit, move and delete ops as one change with a message, all or none; each op sees what the ones before it did; base_version is from before the batch. Unlike separate calls, links follow its renames and moves as one, from where they led before the batch or at their page's last write, blind to what ops did between, its aliases included. A page it writes that is being edited refuses at that write.
```

以 `TestTheBuiltInInstructionsAreShort` 的数法（`mcpserver.TextBytes`，`page/adapter/mcp/instructions_test.go:17`：换行数 2 个字节）是 425 字节（UTF-8 是 424）：笔记本入口 2,961 → 3,386、账户入口 3,031 → 3,456，4 KiB 之下各剩 710 与 640 字节。工具的各行是两个入口共用的（`pathsAndTools`，`page/adapter/mcp/instructions.go:13`、`:27`、`:30-31`），所以起作用的是账户入口：P4 的四行（`grep`、`search`、`index`、`backlinks`）连同 `grep` 与 `search` 怎样分工，合计要在 640 字节之内（第 8 节）。它告诉 agent 4.11 的规则（缩到一句：链接跟着 batch 的改名与移动，从 batch 之前、或这一页最后一次写时的去向跟起，看不见其间各项做的，别名也是）与锁在写那一页的项拒绝。`TestTheBuiltInInstructionsAreShort` 守 4 KiB，`TestTheBuiltInInstructionsNameEveryTool` 要每个工具一行。数法的脚本在 `scratchpad/m9/p3/notes/instr3.py`。

### 3.13 码的表、矩阵、工具的字符串与动作

#### 3.13.1 码的表与码的调用

`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:78-95`）加一行，是四个写的工具的并加 `page.batch_too_large`：

```go
"batch": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
	"page.title_taken", "page.cycle", "page.too_deep", "page.revision_mismatch", "page.locked", "linking.pages_locked",
	"page.batch_too_large"},
```

`declaredCodes`（`bootstrap/mcp_client_test.go:325-333`）是契约的码加 MCP 自己的 `mcpOwnCodes()`（`:338-340`，答 `[]string{"page.batch_too_large"}`）（注释照 4.7："MCP 只多一个"）；`apitest.ProblemCodes` 的注释（`platform/httpserver/apitest/problems.go:106-109`）改为"除了 bootstrap 的 `mcpOwnCodes`"。`TestEveryMCPToolHasItsCodes`（`TestEvery…`，第一份，只读文件，不加时间）照旧核对每个码都声明过。

`TestTheMCPToolsErrorsCarryTheirCodes`（`bootstrap/mcp_reads_test.go:386-450`，写的调用在 `mcp_write_codes_test.go:37-161`）加 `batch` 的一组调用，每个码至少答出一次（表外的码使调用失败）：

| 码 | 调用 |
|---|---|
| `bad_request` | `{"ops": "x", "message": "m"}` |
| `validation_failed` | `{"ops": [], "message": "m"}`；`{"ops": [{"op": "rename"}], "message": "m"}` |
| `notebook.not_found` | 账户入口里看不到的笔记本（同别的工具） |
| `page.not_found` | 一项 `edit` 不存在的 `Missing.md` |
| `forbidden` | carol（读者）的一项 `write` |
| `server_busy` | 同 `write` 的那一个（取走解析的预算之后），一项 `write` |
| `page.title_taken` | 一项 `write` 不带令牌、在已有的页上写不同的内容 |
| `page.cycle` | 一项 `move` 把一页移到它自己的子页之下 |
| `page.too_deep` | 一项 `write` 建第 11 层 |
| `page.revision_mismatch` | 一项 `write` 带过时的令牌 |
| `page.locked` | 先 `edit` 一页、再 `write` 种下的会话持着的页（第 1 项答 `page.locked`，什么都没写） |
| `linking.pages_locked` | 一项 `move` 一页，链到它的页被种下的会话持着 |
| `page.batch_too_large` | 101 项 |

#### 3.13.2 矩阵的一行

`pageWriteMCPRows()`（`bootstrap/permission_matrix_mcp_writes_test.go:46-158`）加一行，两个入口，写者得到结果、读者 `forbidden`、别的 `notebook.not_found`（`writers`）：

- `args`：`{"ops": [{"op": "write", "path": "mcp-batched.md", "content": matrixMark + ", batched\n"}, {"op": "move", "from": pagePathOf(c), "to": moved(c)}], "message": "matrix"}`：两项，所以每一格走一次多步的单元（建、移动）。
- `check`：`changed` 为真、`results_count` 为 2、两项的路径是 `mcp-batched.md` 与移动之后的路径、`changeset_id` 不空。
- `after`：经 `read` 读回 `mcp-batched.md` 的内容与移动之后的那一页；`after` 拿不到答复（只有 `call`，`bootstrap/permission_matrix_mcp_test.go:40`、`:520-523`），所以先以 `log`（`client: "mcp"`、`limit: 1`）读 agent 最新的一条、取它的 id（不带 `client` 时，种下的改名比种下晚一秒，一格可能就在那一秒之内，最新的一条就不是 batch 的：第一次运行遇到，实施的偏差 23），再 `changeset` 读这个变更集：类型 `edit`、说明是 `"matrix"`、两项变化。

加 26 格（13 列 × 2 个入口），写的格从 104 到 130（各起一个应用、复制一个库，至多 8 个同时）。`TestThePermissionMatrixCoversEveryTool` 要每个工具有行。

#### 3.13.3 工具的字符串

`validToolArguments`（`bootstrap/mcp_reads_test.go:667-686`）加 `"batch": {"ops": [{"op": "write", "path": "Strings.md", "content": "x"}], "message": "strings"}`。`stringParameters`（`:528-557`）从 schema 推出平的项的 8 个字符串成员（`op`、`path`、`content`、`base_version`、`old_string`、`new_string`、`from`、`to`；11 个里另三个是布尔）与 `message`；`withArgumentAt`（`:570-606`）只换第一项的成员。平的项让这一项的工具不收的参数（`ops[].from` 之于 `write`）也被推出：它们到达用例、答 `ops[0].from` 的 422 `not_allowed`，不是 `bad_request` 与 `internal_error`，这正是检查要的"到达用例"。`TestToolStringsDoNotAnswerInternalErrors`（`:761-809`）照旧断言最后的参数里有那个 NUL。

#### 3.13.4 动作与规则

- `page/domain/actions.go:8-46`：`ActionBatch = "page.batch"`，进 `Actions()`。
- `access/domain/rules.go:90-110`：`"page.batch": {Level: LevelNotebook, Notebook: writers()}`（编辑者与管理员）。`bootstrap/actions_test.go` 的"规则 = 动作"照旧核对两边相同。
- 用例在读树之前判定一次（不加锁，读者的 403 在解析之前，13.1 第 4 条），`Writer.Run` 在锁下再判定一次（`UnitSpec.Action`）。`batch` 里的每一项不另判定它的工具的动作：`page.batch` 是写者的，`page.create`、`page.write`、`node.move`、`node.rename`、`node.delete` 也都是写者的（`rules.go:90-110`），一个写者能做的 `batch` 都能做；以后某个动作的规则与写者不同时，`ActionBatch` 的注释要求改这里（一项一项判定）。

### 3.14 S1 的测试

#### 3.14.1 domain、用例、mcpserver 与适配器（不要数据库；竞态那一份）

- **`page/domain`**：
  - `TestEveryRuleNamesTheFieldsItIsGiven`（3.2）。
  - `TestCheckBatch`：项数 0、1、100、101；字符串合计在 6 MiB 处与多一字节，正文与 `new_string` 合计在 5 MiB 处与多一字节；`message` 的界线；`op` 不认识；每种工具缺一个必填的成员、多一个它不收的成员（每种每个成员一行）；取值的规则照工具的次序（第 3 项的路径坏、第 5 项的 `op` 坏：答第 3 项）。"先查合计、后查各项"的三行，各带一个坏的项：101 项且第 0 项的 `op` 坏；字符串过 6 MiB 且第 0 项的路径坏；`message` 坏且第 0 项的 `op` 坏：三行都答合计或 `message`（第 4 条变异，审查 C-M1）。
  - `TestPlanBatch`（模拟的树上的表）：P2 的每一步的每种拒绝在第 i 项（字段 `ops[i].*`）；前面的改名、移动、新建、删除之后的路径（"先 `move A.md → B.md`、再 `edit B.md`"找得到，"再 `edit A.md`"是 404）；新建的页（祖先也是）取 `ids[i][d]`；`Items` 的数法；规范化（先删子孙再删祖先：`MergedInto`、祖先的 `Subtree` 含它；两层的折叠；不在子树里的不并；并进来的组不算 `recursive`）；B1（新建之后删它、删含它的祖先）、B3（改名、移动、移进来的之后删它或含它的祖先），先写正文再删除可以；给新建的页带 `base_version` 而令牌的节点又不是它：答 422 而不是 409（审查 A-M12）；同一父节点下的改名不看子树（审查 A-N11：给一个子树的读会失败的假树）；`write` 不带令牌在已有的页上（第二遍比内容）；第一个出错的项定答复（第一遍在第 5 项、第二遍在第 3 项：答第 3 项）；条目在第一遍里答（审查 A-M11 的两种：第 40 项过了条目、第 3 项的 `old_string` 第二遍会找不到：答第 40 项的条目，不跑第二遍；第 3 项第一遍就出错、条目在第 40 项才过：答第 3 项）。
  - `TestBatchPlanContents`（第二遍，假的 `LinkRewriter` 记下它被调用的参数与它数的解析）：令牌过时；审查 A2-M4 的两行：带过时的令牌的 `write`、字节与页相同（以头的大小与哈希比）是未改动、与页不同是 `page.revision_mismatch`；只读头的页只在 `heads` 里、`texts` 里没有它也照常；同样字节的覆盖（仍是这一页的正文操作）、`edit` 在改写过的正文上匹配（"先移动、再以含 `[[B]]` 的 `old_string` 编辑"）、"之前"的来源（第一个正文操作之前是 batch 之前的、之后是前一个正文操作时的）、`update_links` 为假的节点不改写、索引不是当前的（版本号或提取器）页不改写；歧义的链接：之前与之后都歧义的不改写，下一项以 `[[X]]` 为 `old_string` 匹配得上（审查 A-M3）；计划的解析：按（页、哈希）留事实，`Resolve` 之后同一份正文的 `Rewrite` 不再解析；`MaxBatchParseBytes` 在 10 MiB 处与多一字节，过界的解析不做、答那一项（多一字节的一行只有一页：两页时单元的界线先拒，第 15 条变异的另一种写法与第 53 条都过了它，实施的偏差 25）；多页时先数进页数与字节（过线在那一项）；第一遍出错的项（`Failed`）及其后的项，`LinkRewriter` 一次都没被调、一次都没解析（审查 C-M1 的第 8 条）。
  - `TestRelocationOfIsMovesOrder`：环、名称、层级的次序与 `Unit.Relocate` 的相同（它的表照 `unit_relocate_test.go` 的；P2 的 `TestRelocateRefuses` 照旧留着，两张表都通过：只核对"调了 `RelocationOf`"要在单元里开一个口子，实施的偏差 5）。
- **`linking/domain`**：`TestPathBeforeUndoesEveryChangeFromTheRoot`：文件夹 F 改名 G 与其下的 X 改名 Y；X 移出改名的 F；两层的移动（最深的被移动的节点）；路径上没有被移动的、只有改名的；只有一个改动时与今天的结果相同（M6 的表照旧通过）。`TestRelocationRecasesEveryPage`：一组改动里两页只改了大小写，`Relocation` 答两页，`Rewrites` 对两页的链接都答真（审查 A 的 `TestReviewATwoCaseOnlyRenamesInOneUnit` 今天答 `[[Notes]]` 为假）；只有一页时同今天。
- **`linking/app`**：参与者的表格测试（`rewrite_test.go`）加：
  - `Written` 的页：(a)–(e)：正文的链接按起点对上计划的"之前"（带 `Ambiguous`）；版本号比索引新也改写；追加的写以当前的版本号为基；每页自己的 `tree.Before`；每页自己的改动（`A → B → A` 合并成什么都不搬时，写了的页照它最后一个正文操作之后的 `B → A` 改写；索引没有它的行、`IndexedOf` 不是当前的，也照样做，审查 A-I1）；`update_links` 为假的节点不改写；
  - 写了的页的链接取自 `Written` 给的事实：假的解析器对它们最后的正文一次都没被调，只解析改写的读回（审查 A2-M2）；
  - 写了的页自己的改动移动的节点（审查 A3-I1）：`W.Changes` 只有 F 的移动，`spread` 读 F 的子树，W 的 `[[Top/F/X]]` 照分开调用改写（第 19 种的样子）；两页都链进 F、之后移动 F：两页都改写，假的 `Pages` 的 `Subtree(F)` 只被调一次（一个 batch 一份），假的 `Store` 的 `AliasKeys` 也只对 F 的子树查一次（审查 A4-N2）；`update_links` 为假地改了名的 X 在 F 之下、W 在那之后写下 `[[Top/F/X2/Y]]`、再移动 F：照分开调用改写（X2 不在合并的改动里，在读到的子树里）；
  - 删除了的页：`Gone` 的来源页的行在 `resolutions` 之前去掉；`Gone` 的页的别名行当作没有（审查 A-I2 的两个场景：今天都答 "linking: the pages [...] are not pages of ..."）；别的不在的页照旧答那个内部错误；"之前"的目标在 `Gone` 里的链接不改写、记 info；计划之后被别人删除的，不改写、记 warn（审查 A-M13）；都没有 error 级的行；
  - 数与次序：`MaxPages` 在解析之前数（过了答 `ErrTooManyRewrites`，`Locks` 与解析器一次都没被调）；`MaxPages` 为 0 不限；被编辑的页与过界同时：答过界。
  - 先改写写了的页、再改写索引里的页（实施的偏差 27）：`TestABatchRewritesThePagesItWroteFirst`（`batch_rewrite_test.go`；预算 44 KiB，写了的页 40 KiB，索引里两页各 30 KiB：旧的次序答 `server_busy`，把两段对调的变异被它抓到）。
  `BatchLinks` 的测试：在假的树上解析与改写，与参与者对同一个改动写出的字节相同（一条规则一份）；那一项之前删除的页的别名行当作没有。
- **`page/app`**：`apply_batch_test.go`（照 `path_writes_test.go` 的假的端口）：
  - 新建的页的 id：单元以计划的 id 建页（`PageDraft.ID`）；参与者收到的每个 id（`Written` 的键、`Leads` 与 `Targets` 的、改动的）都是假的库建过或读到的（审查 C-I2）；重新计划时用了另一组 id；
  - 正文合成：同一页的三个正文操作，守卫只看到一次写、在第一个正文操作的位置、内容是最后的；版本行一行；不带令牌、字节相同的 `write` 也锁正文行、比版本号（计划之后别人写过：重做，第二次答 409 `PathTaken`，审查 A-M4）；比过之后 `WriteContent` 答 `ErrRevisionMismatch` 是 500，不重做；
  - 推后：参与者在全部操作之后被调一次，改动是合并的、`Revision` 为 0，`Written`（各页的 `Before`、`Changes` 与 `Facts`）、`Parents`、`Gone`、`MaxPages`、`FirstMove` 照计划；`update_links` 为假的节点不在改动里；新建之后又移动的节点在（"前"是新建之后的）；写了的页的 `Changes` 只有它自己改名、移动的节点，不带被移动的节点之下的节点（它们一个 batch 一份，由参与者读，审查 A3-I1）；`Facts` 是 `DecidedAll` 为它最后的正文留下的那一份；
  - 重做：计划之后正文被写过，从头再来一次而成功；仍不同答 409（正文操作是 `VersionMismatch`，`Why` 照那一项：不带令牌的 `KeptChanging`、带的 `Changed`，审查 A-N7；树的操作是 `BatchChanged`），`op` 是第一个不同的项；锁下找到的节点与计划的不同（两次插进来的网页的改名，一次交换）：重做（第 17 条变异）；守卫的拒绝不重做；
  - 界线：条目在 10,000 与 10,001（第一遍里答，`PageStates` 与 `PageContent` 都没被调）；读的页在 14 MiB 与多一字节（以 `PageStates` 的大小，过界时 `PageContent` 一次都没被调，审查 A-I5）；只有一页时不受它限；100 项 `write`：50 项带 `base_version`、以小的正文整页替换 50 页 5 MiB 的页，50 项不带、与 50 页约 1 KiB 的页字节相同（正文合计在 `CheckBatch` 的 5 MiB 之内）：计划成功、`PageContent` 一次都没被调（只读头，审查 A2-I2、A3-N1、NC3-M1）；三页 5 MiB 上不带 `base_version`、字节不同的 `write`：答第一项的 409 `PathTaken`，不是第三项的读的 422（审查 A3-N1）；锁的预检在 `DecidedAll` 之前（被编辑的页：答 `page.locked`，没有解析）；页数与字节按（页、哈希）数、同一页两份是一页的例外；链接在 20,000 与 20,001（假的解析答链接数）；
  - 答复：之后被删除的页的正文操作答它合成的写的版本与那一项时的路径（审查 A-M9）；`edit` a→b 再 b→a：两项各答 `changed` 为真，没有变更集；
  - 回到原处：两次改名之后 `DropUnchanged` 被调，`Outcome` 没有变更集；
  - 解析的预算（真的 `ContentParser`、假的 5 MiB 的预算，审查 A2-M2）：一页 5 MiB、之后的改名碰到它的链接、再 `edit` 它（计划的改写的读回要整个预算）：成功；P 5 MiB 与 Q 3 MiB 都写了、P 的链接被之后的改名碰到（假的参与者照读回以 `TakeNow` 取 5 MiB）：成功；假的解析器按哈希数，每份最后的正文只解析一次；
  - 每种错误包成 `OpFailed`，`Op` 与 `Tool` 对；计划之外的错误不包。
- **`page/app` 的 `content_test.go`**：`Planned` 的会话：取预算与它的 `DecidedAll` 共用一个等待（一个假的持有人占着预算：计划等过的 `DecidedAll` 不再有，用完时 503，不是再等一个 `parse_max_wait`）；每次取预算之前放还会话持着的事实的份额（5 MiB 的预算上两次 5 MiB 的解析都取得到，事实仍在）。
- **`page/adapter/postgres` 的 `TestPageStates`**（要数据库，同这个包今天的测试；`paths_test.go:94-113`，审查 NC2-M1）：在加 `Hash` 的同一个提交里改为逐字段比：带 `[]byte` 的 `PageState` 不能以 `!=` 比，一个字段不比的字面量也不再相等；代码里没有内容哈希的定长类型（`page/app/ports.go` 的哈希都是 `[]byte`），所以不另造一个。A 断言版本号、大小、最后的写与哈希（空的正文的 SHA-256）；B 的 `UPDATE` 同时设 `content_hash`（`sha256('twelve bytes')`），断言 B 的哈希；注释改为"revision, size, content hash and last write"。第 70 条变异（`PageStates` 不填 `Hash`）由它抓。
- **mcpserver**：`TestAnErrorsPlaceFollowsItsCode`：`At` 在三种文字里都在码之后、`Fit` 的每个 k 都带；不以码起头的文字照规则加在最前面并记 error。
- **page 的 `adapter/mcp`**（`batch_test.go`，假的用例）：每种错误的文字与结构（`op`、`At`、字段、锁）；最坏的取值处每种错误都在 48 KiB 之内（`ErrorAnswerBytes`，照 P2 的 `TestEachErrorFitsAtItsWorst`）；答复的 `Fit`：100 项最长的路径时截在某一项、建的节点截在某一项之中，`results_count` 与 `created_count` 照全部；什么都没改时 `changeset_id` 为 null。

#### 3.14.2 合并的每种在真的库上

`TestTheMergesOfABatchHoldOnTheDatabase`（`bootstrap/mcp_batch_merge_test.go`；`TestTheM…`，第三份）：经 MCP 的 `batch`，每种一个子测试，读 `changeset_items` 与 `page_revisions` 的行：

| 子测试 | batch | 断言 |
|---|---|---|
| 新建之后改名、移动 | `write X.md`、`move X.md → F/Y.md` | 一个条目：前空、后是 `F` 下的 `Y`、版本号 1；一个版本行：版本号 1、`base` 空、哈希是写的内容 |
| 新建之后 `edit` | `write X.md`（"a"）、`edit X.md` a → b | 一个条目；一个版本行，版本号 1、内容是 "b"（守卫只看到一次写，`page/app` 的测试） |
| 改名之后写正文 | `move A.md → B.md`、`write B.md` 带令牌 | 一个条目（前后都在、版本号空）；一个版本行，`base` 是 batch 之前的版本号 |
| 写正文之后改名 | `edit A.md`、`move A.md → B.md` | 同上 |
| 写正文之后删除 | `edit A.md`、`delete A.md` | 删除的条目、版本号是删除时的（batch 写的）；版本行一行，`base` 是 batch 之前的 |
| 删子孙之后删祖先 | `delete F/X.md`、`delete F.md` 带 `recursive` | 规范化：一次删除，F 与 X 同一时刻、F 是回收站的根；第 0 项答 `merged_into: 1` |
| 推后的改写又写了 batch 写的页 | `edit P.md`（加 `[[A]]`）、`move A.md → B.md` | P 的版本行仍是一行，内容是改写之后的 `[[B]]`，版本号比 batch 之前多 2（合成的写、再改写的追加，同一行更新） |
| 推后的改写又写了 batch 新建的页 | `write N.md`（含 `[[A]]`）、`move A.md → B.md` | N 的版本行一行，版本号 2、`base` 空；N 的条目版本号 1（N 以计划的 id 建，参与者照它读，审查 A-I1） |
| 改名再改回 | `move A.md → B.md`、`move B.md → A.md` | 没有条目、没有变更集：答复的 `changed` 为假、没有 `changeset_id`，两项各答 `changed: true`；`log` 里没有这一次（审查 A-M8） |
| 改名再改回，另写一页 | 同上，再 `edit C.md` | 变更集只有 C 的版本行，没有条目；历史里没有"调整了次序" |
| 改名再改回，中间写它 | `move A.md → B.md`、`edit B.md`、`move B.md → A.md` | 一个版本行（`base` 是 batch 之前的）、没有条目；第 1 项与 batch 的 `changed` 为真；A 的链接的索引是新的正文的（它的改动留在 `u.changes` 里，观察者照它重建，审查 A2-N2） |
| 过期的令牌、相同的字节 | 先以 REST 把 A 写成 "b"（MCP 读到的令牌 r 过期）；batch：带令牌 r 的 `write A.md` 写 "b"、`edit C.md` | 第 0 项答 `changed: false` 与 A 现在的版本，A 没有新的版本行；C 的版本行一行；batch 成功，同单个 `write`（`PageStates` 的哈希来自真的库，审查 A2-M4、NC2-M1；第 70 条变异下答 `page.revision_mismatch`） |

每个子测试之后 `checkPages`、`checkLinks`，并断言日志的捕获里没有 error 级的行（3.14.3 同）。B1、B3 的拒绝在 3.14.5。

修正的第 1 轮另加（S1 审查 C-M1、C-N2）：batch 之后的 `created`，与新建再改名的文件夹（`write N/X.md`、`move N.md → M.md`：`M.md`、`M/X.md`）；推后的改写之后第 0 项的 `version`（P 在 3）。

#### 3.14.3 链接与分开调用相同

`TestTheLinksOfABatchAreThoseOfSeparateCalls`（`bootstrap/mcp_batch_links_test.go`；`TestTheL…`，第三份）：每种情形建两本一样的笔记本（同一组 REST 的写），一本以一个 `batch`、一本以按次序的分开的 MCP 调用，比较每一页的正文的字节与索引的解析（按路径对上节点）：

1. 先改正文、再移动 A（P 加 `[[A]]`，再 `move A.md → B.md`）：P 写成 `[[B]]`。
2. 先移动 A、再以含 `[[B]]` 的 `old_string` 编辑 P（P 原有 `[[A]]`）：计划里改写过的正文匹配得上，与两次调用相同。
3. 改名、改正文、再改名（`A → B`、编辑 P、`B → C`）：P 是 `[[C]]`，不是 `[[C|B]]`（(d)）。
4. 嵌套的改名（文件夹 F → G、其下的 X → Y，两种次序）：没写的页 Q 的 `[[F/X]]` 与 `[[X]]` 与分开调用相同（`PathBefore` 从根撤回）。
5. 移出文件夹再删掉它（`move F/X.md → X.md`、`delete F.md`）：Q 的 Markdown 链接 `[F/X](F/X.md)` 写成 `[X](X.md)`（原来的父节点的路径在单元开始时读出；在单元之后读，F 已没有路径，写成 `[F/X](X.md)`）。用 wikilink 时两种读法都写成 `[[X]]`，测不出这条规则（审查 A-M1，`TestReviewACase5FormerParentReadAfterTheDeletion`）。
6. 删除链到被改名的页的页（`move A.md → B.md`、`delete P.md`，P 链到 A）：不答 500（`Gone` 的来源页的行在 `resolutions` 之前去掉），Q 照常改写。
7. 删除一个有别名的页、把另一页改名为那个别名（P 有 `aliases: [Roadmap]`，Q 有 `[[Roadmap]] [[Plan]]`；`delete P.md`、`move Plan.md → Roadmap.md`）：不答 500（`Gone` 的页的别名行当作没有），Q 与分开调用相同；再加一项之后的 `edit Q.md`（计划的解析在那一项看到已删的 P 的别名行）也相同（审查 A-I2）。
8. "之前"的目标被这个 batch 删除、它的键又被碰到（`delete T.md`、`move A.md → T.md`，Q 链到 `[[T]]`）：键 t 被碰到、"之前"是已删的 T：不改写、没有 error 级的行（键要被碰到，这条跳过才测得出，审查 A-M1）。
9. 新建、链到它、再改名移动它（`write X.md`、`edit P.md` 加 `[[X]]`、`move X.md → F/Y.md`）：P 写成 `[[Y]]`（或分开调用写的那种路径的写法），与分开调用相同（改名与移动都做：只移不改名时 `[[X]]` 在标题唯一时仍解析到 X，测不出这条规则，审查 A-M1、C-I2）。
10. 新建一页带链接、再移动它链到的页（`write N.md` 含 `[[A]]`、`move A.md → B.md`）：N 写成 `[[B]]`（审查 A-I1：计划给的 id 就是单元建的 N 的）。
11. 改名再改回，中间改正文（`move A.md → B.md`、`edit P.md` 写下 `[[B]]`、`move B.md → A.md`）：P 写回 `[[A]]`（审查 A-I3）。
12. 移出再移回，中间改正文（`move P.md → Archive/P.md`、`edit Q.md` 加 `[[Archive/P]]`、`move Archive/P.md → P.md`）：Q 写成 `[[P]]`。
13. 新建的节点改名再改回（`write X.md`、`move X.md → Y.md`、`edit P.md` 加 `[[Y]]`、`move Y.md → X.md`）：P 写成 `[[X]]`。
14. 写了的页的链接在它最后一个正文操作之后被新建拿走（只有 `F/X` 时 `write W.md` 含 `[[X]]`，再 `write X.md` 在根下）：W 的 `[[X]]` 改指新的 X、不被写回，同分开调用（3.7.1：写了的页只照它自己的改动改写）。另跑一次之后再改名（W 含 `[[X]] [[A]]`，再 `move A.md → B.md`：W 写成 `[[X]] [[B]]`）：照设计的这一种没有移动，写了的页不被跟随，第 49 条变异在那里看不出来；加了改名，W 被跟随，它就被抓到（实施的偏差 26）。
15. 同一单元两次改名（M6/P4 的移交第 1 条）：`A → B`、`C → D`，没写的页 Q 链到 A 与 C，两条各跟着改。
16. 两页只改了大小写（`move Notes.md → notes.md`、`move Plan.md → plan.md`，Q 有 `[[Notes]]` 与 `[[Plan]]`）：两条都照分开调用改成新的大小写（`Recased` 是集合，审查 A-M2 (e)）。
17. 新建再改名、抢走没写的页的链接（Q 有 `[[Y]]` 通向 `G/Y`；`write X.md`、`move X.md → Y.md`，都在根下）：Q 写回 `[[G/Y]]`，同分开调用那次改名的参与者（新建之后被改名的节点交给参与者，3.7.1）。
18. 同上，抢走的是写了的页的链接（`edit W.md` 加 `[[Y]]`、再 `write X.md`、`move X.md → Y.md`）：W 写回 `[[G/Y]]`（W 自己的改动里有那之后新建又改名的节点）。
19. 写了的页以完整的路径链进一个之后只被移动的文件夹（`Top`、`Top/F`、`Top/F/X`、`G` 与 `P`；`edit P.md` 写下 `[[Top/F/X]] more`、`move Top/F.md → G/F.md`，不改名）：P 写成 `[[X]] more`，同分开调用（参与者对 P 读它自己的改动移动的 F 的子树，X 在其中；只展开改名的节点时 P 留着悬空的 `[[Top/F/X]]`，什么都不记，审查 A2-I1、A3-I1）。
20. 在被移动的文件夹下新建、抢走一个标题的链接（S1 审查 B1-M1 的 K1r、K1r2 与对照 K0）：`A/B/C/N` 在，Q 与 R 有 `[[N]]` 通向它；`move Top/F.md → G/F.md`、`write G/F/N.md`、`edit R.md`（`old_string` 带 `[[N]]`，或不带）：新建只重新解析，R 的 `[[N]]` 改指 `G/F/N`，不被写回，`edit` 找得到它；对照：新建在没被移动的 `H/N.md`。
21. 同上，链到 N 的页在移动之前写（S1 审查 B1-M1 的 K2：`edit P.md` 写下 `[[N]]`、移动、新建）：P 留着 `[[N]]`。另一种：N 在移动之前新建在 `Top/F` 下、被那次移动带着走：Q 的 `[[N]]` 没被新建抢走（路径一样长时 id 小的旧页赢），移动的改写照样留着它通向 `A/B/C/N`，写成 `[[A/B/C/N]]`，同分开调用。
22. 新建在被改名的文件夹下，没写的页（S1 审查 B1-M1 的 K3b；`move F.md → G.md`、`write G/N.md`）：Q 留着 `[[N]]`；对照 K3c：改名的 `update_links` 为假。
23. 被跟随的节点在一个冻住的节点之下（S1 审查 B1-N1）：d5，`move Top/F.md → G/F.md`、`move G/F/X.md → G/F/Y.md`（为假），Q 的 `[[Top/F/X/D]] [Top/F/X/D](Top/F/X/D.md)` 写成 `[[D]] [D](D.md)`；d5r，F 改名；d5k，X 移到 F 下的 K 之下（为假）：Markdown 链接的文字是"之前"的路径，照 M6 写成 `[D](D.md)`。
24. 在一个文件夹两次改名、移动之间新建在它下面（S1 复核 B 的 B2-N2，B 的 K4r 与 K4tw）：新建的页与链接的目标路径一样长，id 小的旧页赢，新建不抢走链接；第二次改名、移动把它带到更短的路径，它的改写把链接写回原来的目标，同分开调用。K4r：`A/B/C/D/N` 在，Q 有 `[[N]] q`；`move Top/FFF.md → Top/EEE.md`、`write Top/EEE/N.md`、`move Top/EEE.md → Top/E.md`：Q 写成 `[[A/B/C/D/N]] q`（没写的页，索引的展开，改名）。K4tw：P 先被 `edit` 成 `p [[N]]`，`move Top/F.md → Top/E.md`、`write Top/E/N.md`、`move Top/E.md → G/E.md`：P 写成 `p [[A/B/C/N]]`（写了的页）。窗口取每个节点的第一个操作时两种都错（变异）。

每种的断言：两本的每页正文逐字节相同、索引的每条链接解析到对应的节点、日志的捕获里没有 error 级的行；第 1、2、3、9、10、11 种另断言 batch 的那一本只有一个变更集。第 1、3、6–14、16–19 种是原型核对过的（3.7.1；`scratchpad/m9/p3/exp/results.txt`）；第 20–23 种是修正的第 2 轮加的，其中 S1 审查 B 的 K1r、K1r2、K2、K3b、d5、d5r 在 S1 与第 1 轮之后的代码上与分开调用不同（S1 审查 B 量的）；第 24 种是修正的第 5 轮加的。

#### 3.14.4 与分开调用不同之处

`TestTheWaysABatchDiffersFromSeparateCalls`（同一个文件；`TestTheW…`，第三份）：同样两本笔记本，断言结果**不同**、且各是 4.11 的规则说的样子（规则只写在 4.11：链接跟着 batch 按节点合并的改名与移动，从 batch 之前、或写了的页在它们之前最后一次写时的去向跟起；其间别的项对去向做的看不见，batch 改的别名也看不见）。没写的页与写了的页各有：

- **(1) 没写的页的"之前"是单元之前的索引**：
  - (1a) 先在根下新建 X、再把 `F/X` 移到 `G/X`：batch 把 Q 的 `[[X]]` 改写回被移动的 X（`[[G/X]]`），分开的调用留在新的根下的 X 上；
  - (1a2) 在被改名的文件夹下、改名之前新建（`write F/N.md`、`move F.md → G.md`，Q 有 `[[N]]` 通向 `A/B/C/N`）：分开调用里新建先让 `[[N]]` 改指 `F/N`，改名再跟着写成 `[[N]]` 通向 `G/N`；batch 里 Q 的"之前"是单元之前的索引（`A/B/C/N`），改名的触及里有 N（它在改名之前新建，同分开调用里那次改名碰到它），写回 `[[A/B/C/N]]`。在改名之后新建的不进它的触及（3.14.3 的第 22 种，S1 审查 B1-M1）。
  - (1b) 先把 A 改名为 B（Q 里有一条悬空的 `[[B]]`）、再把 B 改名为 C：分开调用让 `[[B]]` 先接到被改名的页、再改写成 `[[C]]`；batch 不动它（悬空）；
  - (1c) 中途被抢走又放开的链接：先把 A 改名为 X（Q 的 `[[X]]` 通向 `F/X`）、再改名为 Z：分开调用把 Q 的链接写成完整的路径 `[[F/X]]`，batch 不动它；两者都通向 `F/X`（指向相同、文字不同）；
  - (a) 删除让链接改了去向（审查 A-M2）：Q 有 `[[X]]`，`F/X` 与 `G/H/X` 都在（`[[X]]` 解析到路径短的 `F/X`，`linking/domain/resolve.go:310-336`）；`delete F/X.md`、`move G/H/X.md → G/H/Y.md`：分开调用里删除让 `[[X]]` 改指 `G/H/X`，改名再写成 `[[Y]]`；batch 里"之前"是已删的 `F/X`，跳过，`[[X]]` 悬空；
  - (b) 去掉一个别名：P 有 `aliases: [Foo]`，Q 有 `[[Foo]]`；`edit P.md` 去掉这个别名、`move A.md → Foo.md`：分开调用里 `[[Foo]]` 先悬空、再解析到改名之后的 A，"之前"为空，不改写；batch 里"之前"是 P（索引的），"之后"是 A，改写成 `[[P|Foo]]`，通向一个已没有这个别名的页；
- **(e) 写了的页从它最后一次写时的去向跟起**（审查 A2-M1；原型的 X1b、X5 与写了的页上的 (1b)，`exp/results.txt`）：
  - (e1) 之前的新建：`edit P.md` 写下 `[[X]]`（那时只有 `F/X`）、`write X.md`（根下）、`move F/X.md → F/Y.md`：分开调用里新的 X 拿走了 `[[X]]`，P 留着 `[[X]]`；batch 从 P 写时的去向 `F/X` 跟起，写成 `[[Y]]`；
  - (e2) batch 改的别名：P 有 `aliases: [Foo]`；`edit P.md` 去掉这个别名、`edit Q.md` 写下 `[[Foo]]`、`move A.md → Foo.md`：分开调用里 Q 的 `[[Foo]]` 先悬空、再通向改名之后的 A，留着 `[[Foo]]`；batch 以 batch 之前的别名解析 Q 写时的 `[[Foo]]`（P），写成 `[[P|Foo]]`；
  - (e3) 按节点合并藏起中间的名称：`edit P.md` 写下悬空的 `[[B]]`、`move A.md → B.md`、`move B.md → C.md`：分开调用写成 `[[C]]`；P 自己的改动是合并的 `A → C`，不碰悬空的 `[[B]]`，batch 留着它；
- **(c) 写了的页在第一次写之前**（审查 A-M2）：Q 有 `[[X]]`、只有 `F/X`；`write X.md`（根下；分开调用让 Q 的 `[[X]]` 改指它）、`move F/X.md → G/X.md`、`edit Q.md`（`old_string` 是 `[[X]]`）：分开调用三次都成功；batch 的计划在第 2 项之前把 Q 的 `[[X]]` 改写成 `[[G/X]]`（从 batch 之前的索引跟起），第 2 项答 422（`old_string` 找不到）。
- **(d) `update_links` 为假的节点，它的祖先照常改名、移动**（S1 审查 B-M1；4.11 的"一个为假就整个不改写"照字面成立）：通向这个节点的链接在每一页里都照原样留着，不论它的祖先怎样改；分开调用里祖先的改动先把链接改写了，节点的改名再让它悬空：
  - (d1) `move F.md → G.md`、`move G/X.md → G/Y.md`（为假）：分开调用先把 Q 的 `[[F/X]]` 写成 `[[X]]`、再让它悬空；batch 留着悬空的 `[[F/X]]`；
  - (d1r) 反过来的次序，`move F/X.md → F/Y.md`（为假）、`move F.md → G.md`：两者都留着悬空的 `[[F/X]]`，相同；
  - (d3) 移动而不是改名：`move Top/F.md → G/F.md`、`move G/F/X.md → G/F/Y.md`（为假）：分开调用写成 `[[X]]` 再悬空，batch 留着悬空的 `[[Top/F/X]]`。修正之前，batch 没写的页里悬空、写了的页经 `spread` 从 F 碰到 X 跟到 `[[Y]]`；祖先只改名时（d1）两种页都经 `spread` 碰到 X，`PathBefore` 给它现在的名称，写成从来没有过的 `[[Y|F/X]]`；
  - (d3w) 同 (d3)，链接在 batch 写的页里（移动之前 `edit P.md` 写下 `[[Top/F/X]]`）：batch 留着 `p [[Top/F/X]]`，分开调用 `p [[X]]`，都悬空；
  - (d3r) 反过来的次序：两者相同，都留着悬空的 `[[Top/F/X]]`；
  - (d2)、(d4) 为假的是祖先，X 自己的改名为真：`move F.md → G.md`（为假）、`move G/X.md → G/Y.md`，与 `move Top/F.md → G/F.md`（为假）、`move G/F/X.md → G/F/Y.md`：batch 跟着 X 自己的改动写成 `[[Y]]`（通向 `G/Y`、`G/F/Y`）；分开调用里祖先的改动先让 `[[F/X]]`、`[[Top/F/X]]` 悬空，X 的改名不再碰它。"一个为假就整个不改写"只对那一个节点：通向它下面的节点的链接照样跟着它们自己的改动。
  - (d2m) 同 (d2)，Markdown 链接：`move F.md → G.md`（为假）、`move G/X.md → G/Y.md`，Q 的 `[F/X](F/X.md)`：batch 跟着 X 自己的改名，"之前"的路径取 F 之前的名称（S1 审查 B1-N1），文字是那条路径，写成 `[Y](Y.md)`；分开调用留着悬空的 `[F/X](F/X.md)`；
  - (d6)、(d6w) 计划里的改写同样留着通向冻住的节点的链接（S1 审查 B1-N2）：`move Top/F.md → G/F.md`、`move G/F/X.md → G/F/Y.md`（为假），再 `edit Q.md`（`old_string` 是 `[[Top/F/X]] q`）或不带令牌的 `write Q.md`（它的字节）：batch 成功、Q 留着 `[[Top/F/X]]`；分开调用里 F 的移动先把它写成 `[[X]]`，`edit` 答 `validation_failed`（`old_string` 找不到），`write` 答 `page.title_taken`。
- **(2) 锁的码**：Q 链到 A，在 batch 写它的第一个正文操作之前正被网页编辑（种下的会话）：`move A.md → B.md`、`edit Q.md`：batch 答第 1 项的 `page.locked`（锁的预检，什么都没写）；分开调用的 `move` 答 `linking.pages_locked`（Q 不链到 A 时，那次 `move` 什么都不改写，不答它，审查 C-N9）。

**写明而不测的不同**（审查 A-M2 (f)，4.11 写着）：

- (f) 照设计的：`base_version` 指 batch 之前（`edit` 一页之后再带令牌 `write` 它，batch 成功，分开调用答 409）；`linking.pages_locked` 答在第一个改名、移动的项，不是碰到被锁的页的那一项；规范化把子树的 `page.locked` 从第 i 项挪到第 j 项。
- 两页只改了大小写不再是不同（`Recased` 是集合，3.14.3 第 16 种）。

#### 3.14.5 整个被拒、界线与交错

- `TestTheMCPBatchIsRefusedWhole`（`TestTheM…`，第三份；它的应用用生产的期限，请求 15 秒、写 60 秒，测试的是 4 秒与 10 秒：计划在停下之前解析的 8 MiB 在竞态检测下要几秒，实施的偏差 20）：每种拒绝之后，笔记本的节点、正文、索引、变更集的数目都与之前相同，答复的 `op` 与码照 3.9.3：
  - 第 3 项 404（前两项是新建与移动）；第 2 项的守卫的 `page.locked`（持锁人的名字与 `op`）；第 0 项的删除的子树里有被编辑的页、第 1 项 `edit` 另一个被编辑的页：答第 0 项的 `page.locked`，照单个 `delete` 的文字（锁的预检按项的次序，S1 审查 A-M1）；第 0 项的 `move` 的 `linking.pages_locked`；B1、B3 各一种；给新建的页带 `base_version`；
  - 界线：101 项；字符串合计过 6 MiB（各项都合法）；条目：以 SQL 种下 10,001 个节点的子树，`delete` 它（第一遍里答，`op` 是那一项）；移动带着走的节点：以 SQL 在 F 下种下 4,999 个、在 H 下种下 5,000 个节点，`move F.md → G/F.md`、`move H.md → G/H.md`，两棵子树合计 10,001 个，第 1 项答（S1 审查 A-I2）；一个文件夹来回移动：以 SQL 在 F 下种下 5,000 个节点（F 的子树 5,001 个），`move F.md → G/F.md`、`move G/F.md → F.md`，10,002 个，第 1 项答（S1 审查 A 的 N2：每次移动各算一次）；读的页：以 SQL 种下三页、大小合计正好 14 MiB（每页一行标题加一个 1 KiB 的单位重复到各不过 5 MiB），各一项 `edit` 以 `replace_all: true` 与空的 `new_string` 去掉它的每一处（`old_string` 是那个单位，在 `old_string` 的 1 MiB 之内，审查 NC2-N1；空的 `new_string` 是许可的，`page/domain/edit.go:33-37`），最后的正文只剩标题、合计在 `MaxUnitBytes` 之内：成功，断言每页最后的大小；多一字节时答过界的那一项、什么都没写（`page/app` 的测试断言没有读；审查 NC-M2：每项只改一点时最后的正文仍约 14 MiB，过了 8 MiB）；计划的解析：审查 A 的 50 对（4 MiB 的 Index 链到 A0…A49，这些页以 SQL 做出、由重建建索引：以 batch 做出时，做 Index 的那个 batch 在竞态检测下与别的份同时跑，过了测试的 4 秒，实施的偏差 20；50 对"`move Ai.md → Bi.md`、`edit Index.md` 改一个小的串"）：第一次改写解析 Index 的正文与读回的写法（8 MiB，不过 10 MiB），第二次要再解析第 1 项之后的正文，过界，答 `ops[3]`、什么都没写，整个调用在 2 秒之内（竞态检测下乘 6：单独跑时 0.37 秒，带竞态检测 3.7–3.8 秒；没有界线时它要解析 100–150 次；`page/app` 的测试以数解析的假的解析器断言之后一次都没解析）；多页的计划的解析过了页数：`move A.md → B.md` 之后 60 项 `edit`（每页都链到 A）：计划在第 51 项过线，答那一项的 `op`（`ops[51]`）；
  - 重新计划之后的 409 由 `page/app` 的测试守（两次不同要两次插进来的写，整个程序里排不出确定的次序）。
- 交错（4.15 的三个，加审查 A-M4 的一个；`interleaveOn` 与 `pgtest.WaitForLockWaitsOn`，结束时 `checkPages`、`checkLinks`、`checkSearch`；都在第二、第三份）：
  - `TestTheMCPBatchMeetsAWebRename`（第三份）：两种次序。网页的改名先到、在测试持着的笔记本行上等；batch 计划（不加锁、看到旧的树）之后排在它后面；放开：改名先做，batch 在锁下找到的节点与计划的不同，重新计划一次：它的那一项（改名之前的路径）答 404、什么都没写，或（另一组参数）在新的树上成功、改名的链接改写也在。另一种次序：batch 先持着笔记本行，改名等它、之后照常，链接的改写包括 batch 写的页。路径改指另一个节点（两次网页的改名，一次交换）留给 `apply_batch_test.go`（第 17 条变异，审查 C-M1）。
  - `TestTheBatchsRewriteMeetsAnOpening`（第二份）：batch 的推后的改写要碰 Q，Q 上开会话：会话先提交时 batch 答 `linking.pages_locked`、`op` 是第一个移动的项、什么都没写；batch 先持着笔记本行时开会话等它，之后在改写过的正文上开。
  - `TestTheBatchIsPlannedAgainAfterAWrite`（第二份）：测试持着 P 的正文行，网页的正文写在它上面等（已持笔记本行 `FOR SHARE`）；batch 计划之后在笔记本行上等写；放开：写先提交，batch 在锁下读到的版本号与计划的不同，从头再来一次，在新的正文上 `edit` 成功（`old_string` 仍在），答复的版本是新的。第二个子测试（审查 A-M4）：batch 的那一项是不带令牌、与 P 字节相同的 `write`，网页写了别的字节：batch 在锁下比版本号、重做，第二次的计划答那一项的 409 `page.title_taken`（`PathTaken`），同分开调用；不比版本号时它不是 409（第 18 条变异）。

#### 3.14.6 索引、性质与值的上限

- **`TestTheIndexIsItsRebuild`**（`bootstrap/links_rebuild_test.go:33-95`；`TestTheI…`，第三份）：`writer.random()` 的操作加两种：大约每 13 步一次随机的 `batch`（经 MCP，2–6 项，从同样的几种写里随机取：新建、覆盖、`edit`、改名、移动、删除；移动都带 `update_links` 为真：性质只对它们成立），与每 20 步一次定向的 `batch`（`edit P` 加 `[[T]]`，T 是那一刻标题唯一的页，再把 T 改名：写的链接之后有改名，随机的生成器里少见，审查 C-M6）。每一步之后照旧比索引与它的重建。batch 的一步另核对 4.11 末条的性质（2.1 第 20 条的写法）：
  - batch 没有写的页：batch 之前每条解析到节点 N 的链接，之后仍解析到 N，除非 N 被这个 batch 删除、它现在解析到这个 batch 新建的节点或写进的别名，或它经一个被这个 batch 的写拿掉的别名通向 N（`ledTo` 之前与之后，按页、按链接的次序对上，同 `leadStill`；拿掉的别名同新建与删除，只重新解析：写的别名不是改名，经保留的别名的照样核对，S1 审查 B-N3，实施的偏差 19）；batch 删除了的与写了的来源页不比（`leadStill` 要每页的链接数不变，删除了的页没有链接，`:195-207`）；
  - batch 写了的页：生成器写进正文的链接里，那一刻在模型里标题唯一的 `[[T]]`（模型是测试自己按生成的各项推出的树），之后解析到那一刻有标题 T 的节点，例外同上；
  - batch 被拒（任何码）就失败，不跳过（S1 审查 C-N1：以前跳过别的拒绝；这个测试里没有并发的写，答 `page.revision_mismatch` 或 `BatchChanged` 就是计划的树与单元不一致，13.4 第 13 条"同一套规则"的检查）；
  - 不空转的计数（跨种子合计，数做成了的 batch，不数试过的）：batch 至少 20 次、它的推后的改写写过页至少 5 次、"写了的页在它最后一个正文操作之后有改名或移动"而被核对的链接至少 10 条，否则失败。
- **`TestTheSearchIndexIsItsRebuild`**（`bootstrap/search_rebuild_test.go:18-45`；`TestTheS…`，第三份）：随机的写加一种 `batch`（新建、改正文、改名、删除），任务在随机的时刻跑，`page_search` 的行等于从头分词的。它用 `newRevertWorld` 与 `w.change(t)`（`search_rebuild_test.go:22-26`），`change` 的第 9 种是撤销最新的 20 个变更集之一（`bootstrap/revert_property_test.go:105`、`:166-169`），不是 2xx 或 409 就失败（`:182-184`）：S1 里撤销 batch 的变更集会答 500（`round()` 的 `ErrRevertUnordered`、S1 的临时的检查）。所以 `revertWorld` 加一个选用的字段 `batches`：只有这个测试在 S1 打开它，`change` 才发 `batch`；第 9 种在 S1 不挑 batch 的变更集（世界记下每次 `batch` 答的变更集的 id），S2 去掉这个限制、撤销的性质测试也打开 `batches`（第 0 节："撤销的性质测试在 S2 才加 `batch`"，审查 C-M4）。
- **`TestTheLargestBatchIsFarWithinTheValuesBound`**（`TestTheL…`，第三份）：3.8.1，两个请求体发出去，不数值的个数。
- **`TestTheMCPBatchReachesItsUseCase`**（`TestTheM…`，第三份）：一个 ingest 样的 batch 经两个入口（建三页、改一页、改名一页、删一页，带说明）：答复的每一项、一个变更集（类型 `edit`、说明、客户端 `mcp:<名>`）、`log` 与 `changeset` 读得到它；答复的 `building` 为真，搜索在任务跑之前也答 `building`，任务跑了之后找得到新建的三页（批量入队，3.10.1）。
- 成本：`TestABatchAtItsRewriteBoundTakesItsTime`、`TestABatchPastItsLinksBoundIsRefused`、`TestABatchPastItsBytesBoundIsRefused`、`TestABatchCountsThePagesItWroteAtTheirFinalSize` 与 `TestABatchWhosePagesOutgrowTheParseBudgetIsRefused`（守卫那一份的同一行，3.8.2）。

### 3.15 S2 的文件

| 包 | 文件 | 新 / 改 | 内容 |
|---|---|---|---|
| page/domain | `revert_schedule.go` | 新 | `revertSim` 与四个前提（`canPlace`、`canDelete`、`canRestore`、`canWrite`）、`schedule()`（轮数有上限）、三条后备（3.16） |
| page/domain | `revert.go` | 改 | `round()` 删去，`PlanRevert` 调 `schedule()`；`RevertAside`；`RevertOp` 加 `Seq`、`Current`；`Steps()` 数临时的步；`RevertWrites` 改为所有带 `Base` 的版本，S1 那条临时的检查删去；`ErrRevertUnordered` 的注释；`planner.maxDepth`（3.16、3.18、3.19） |
| page/domain | `revert_test.go` | 改 | 钉住 M8 的行为的两处（3.21.1）："a page written, then deleted (M9's shape)"（`:342-347`）与 `TestPlanRevertOfASwapIsUnordered`（`:864-886`） |
| page/domain | `name.go` | 改 | `AsideName`（3.17） |
| page/domain | `changeset.go` | 改 | `ContentRef`；`ChangesetMoves` 的注释（3.18、3.20） |
| page/app | `unit_revert.go` | 改 | 临时的步（`RevertAside`）经 `place`；写回以 `op.Current` 为基；解析好的正文按 `ContentRef`（`RevertRead.Parsed`、`plannedContents`、`parsedAs`）（3.18） |
| page/app | `revert_test.go` | 改 | "a round out of order"（`:456-464`）改为存着的缺陷"比 `MaxDepth` 深的页被改名"（3.21.1，S2 的实施的偏离 2） |
| page/app | `history.go` | 改 | `HistoryReads.detail` 的注释（`:85-87`，3.20.3） |
| page/app | `revert_changeset.go`、`content.go` | 改 | 两份正文都解析、按（页、哈希）数界线（3.18.4）；`DecidedContents`，一次取放不下本次调用留着的份额时先还掉它们（3.18.4，S2 的修正的第 1 轮，审查 B-M1）；"changeset reverted"那一行加 `aside`（临时的步的个数，3.21.3） |
| page/app | `jobs.go` | 改 | 任务的计划的临时的拒绝 `errUnsplit`（第 0 节；S3 换掉）；`revertContents` 按 `ContentRef` |
| page/app | `unit.go` | 改 | 单元结束时设变更集的 `renames`、`moves`（3.20.2） |
| page/adapter/postgres | `queries/changesets.sql`、`queries/history.sql`、`store.go`、生成的 sqlc | 改 | `MarkChangesetMoves`；`ChangesetMoves` 读列（3.20） |
| migrations | `00045_page_changesets_moves.sql`、`changesets_moves_test.go` | 新 | 两列与补值（3.20.1） |
| api | `api/modules/page.yaml`、生成的 Go 与 TS | 改 | `renames`、`moves` 的说明（"前 1,000 项"去掉） |
| page/domain（测试） | `revert_schedule_test.go`、`revert_schedule_internal_test.go`、`revert_schedule_property_test.go`、`export_test.go`、`name_test.go` | 新、改 | 3.21.1（包里的测试构造 `revertSim` 与计划者；`export_test.go` 的 `ScheduleRounds` 与 `planRevert` 共用 `scheduled`） |
| page/app | `ports.go` | 改 | `ParseBudget.Takes`、`BudgetHold.Held`（3.18.4，S2 的修正的第 1 轮）；`ChangesetMoves` 的注释（3.20.3） |
| page/adapter/markdown | `budget.go` | 改 | 以平台的 `Budget.Takes`、`Budget.Size` 与 `Hold.Held` 答（S2 的修正的第 1 轮） |
| bootstrap | `events_registrants.go` | 改 | 页面事件只列单元之后活着的写过的页（3.10.2，S2 的修正的第 1 轮） |
| bootstrap（测试） | `revert_batch_test.go`、`revert_property_test.go`、`changesets_moves_test.go`、`interleavings_revert_test.go`、`events_pages_test.go`、`permission_matrix_seeded_test.go` | 新、改 | 3.21；页面事件（3.10.2）；矩阵以 SQL 种下的改名的变更集带 `renames`（S2 的实施的偏离 4） |
| e2e | `stories/mcp/mc5-batch-revert.spec.ts` | 改 | MC5 (page)（第 4 节） |

### 3.16 `schedule()`：按依赖分轮

#### 3.16.1 待做的步与模拟的状态

`PlanRevert` 的 L、S\* 与冲突照 M8（`left()`、`before()`，`revert.go:415-625`），没有冲突时以 `schedule()` 代替 `round()`（`:811-861`）。模拟的状态 `revertSim` 是今天的 `simulation`（`revert.go:767-808`）加上：每页的高度（只数页，`Subtree.Height` 的数法）、每页正文的版本号与哈希（活着的取 `Contents`，回收站里的取它那一组的成员，`TrashMember.Revision` 与 `RevertRefs` 读来的哈希）、S\* 在每个父节点下的键（临时名称要避开）。

待做的步（4.12）：

- **放回** P：C 的节点，现在与 S\* 里都活着，而父节点、名称或次序与 S\* 的不同（`place` 的判断，`revert.go:869-875`）。
- **删除** D：S\* 里不存在、此刻活着的最上层（C 新建的、C 恢复的，`gone`），每一轮按此刻的树重新找（`tops`：父节点不是活着的待删除的节点）。
- **恢复** R：C 删除的各组（`c.tops`，组从 `Groups` 读）。
- **写回** W：`RevertWrites` 的每一页（3.18.1），此刻的正文的哈希与 `Base` 的不同。

#### 3.16.2 前提

每个前提答"成立"或一种冲突的原因（`ConflictReason`）：同步的撤销只看成立与否；后台的每批在锁下以同一组函数核对，不成立的原因就是它停下的冲突（3.24）。

| 步 | 前提（此刻） | 不成立时的原因 |
|---|---|---|
| 放回 | S\* 的父节点活着或是根；不在节点自己的子树里；S\* 的键在那里空着（节点自己占着的不算）；父节点那一线的深度加子树的高度减一不超过 `MaxDepth`（`Unit.Relocate` 的同一条，`unit_relocate.go:59-68`） | `parent_gone`、`cycle`、`title_taken`（带占着者）、`too_deep` |
| 临时的步 | 临时名称在目标父节点下空着（3.17）；移到根下时，子树的高度不超过 `MaxDepth` | `title_taken`、`too_deep` |
| 删除 | 子树里没有 S\* 里活着的节点；子树里没有待做的写回（"先写回、再删除"） | `children_added` |
| 恢复 | S\* 的父节点活着或是根；根的键空着；这一组最深的成员落下之后不超过 `MaxDepth` | `parent_gone`、`title_taken`、`too_deep` |
| 写回 | 页活着 | `changed` |

后台的一批不建 S\*（3.24）：那里的删除的前提比那一步那一刻的子树与这一步记下的节点（`RevertOp.Subtree`）：多出的节点里父节点在记下的节点之中的（新加的每一部分的顶，变更集自己的也是）各是 `children_added`，以它为名、不列它之下的，同 M8 的 `before()` 以加进来的顶为名（`page/domain/revert.go:578-581`）；同步的撤销与调度照上表。

M8 的单轮从不需要层级的前提（它只做一种操作），所以 `round()` 没有。整个模拟的树在每一步之后都不超过层级，由性质测试在每一步之后以它自己的模型数出（3.16.4 的 (T5)；`page/domain` 没有 `checkDepth` 这样的函数，数层级的是测试），生产的代码不另数。

#### 3.16.3 一轮与三条后备

```go
// schedule is the revert's steps from now (M9 design 4.12; M9/P3 design
// 3.16), on s: rounds, each trying every step pending in a fixed order
// (places, deletions, restores, writes back, each by node id) and doing
// those whose premise holds at its moment, until none is pending. A round
// that does none takes the first fallback that applies (fallback): a
// holder of a key another step waits for renamed aside; else a node S*
// keeps, under one the revert deletes, moved aside to the root; else, a
// defence no reachable state has needed, a node to place, not at the root
// and not moved aside, moved aside to the root; each node at most once by
// each. None applying is ErrRevertUnordered, a defect; so is a round past
// |P|+|D|+|R|+|W|+2N+1 (T2's bound, N the nodes the changeset touched),
// which no revert reaches and a defect would otherwise spin past under
// the notebook's lock. The steps are numbered in their order (Seq).
func (p *planner) schedule(s *revertSim) ([]RevertOp, error)

// fallback is the first of the three that applies to s stuck, the step it
// takes and the node it moves; false for none (M9/P3 design 3.16.3):
// first, each place and restore pending in turn, by id, whose parent in
// S* is alive and whose key there a node to place holds that is not
// renamed aside yet: that holder renamed aside where it is. Second, of
// the nodes S* keeps alive under a node the revert deletes, the deletions
// pending, those to place and not moved aside yet, the least by id moved
// aside to the root: it is to be placed, its parent in S* alive while the
// one it is under now is none of S*'s; one not to place, which only a
// defect has there, it never takes, to end unplaced under a name aside.
// Third, a defence: of the nodes to place, not at the root and not moved
// aside yet, the least by id moved aside to the root. A node moved aside
// is renamed too, from its name as the plan began, never another aside
// name on one; it stays to be placed.
func (p *planner) fallback(s *revertSim) (RevertOp, bool)
```

M8 的每种变更集照旧一轮做完、次序与 `round()` 相同：一轮里先放回、再删除、再恢复、再写回，M8 的一种操作的变更集的每一步在第一轮的前提都成立（3.21.1 核对每种 M8 的形状的步与 `round()` 的逐个相同）。M8 的撤销的测试里有三处钉住的是 `round()` 做不到的形状，S2 改它们的期望（审查 C-M3；3.21.1 列出）：它们是 M9 的 batch 才写得出的变更集，不是 M8 的形状。

三条后备（4.12，照模拟写定细节，`sim/schedule.py:228-370`）：

1. 依次看待做的放回与恢复（各按 id）：它的 S\* 父节点活着、它的键被一个**待放回**的节点 h 占着、h 没有临时改过名：h 在原地改成临时名称（`RevertAside`，`Move` 为假）。
2. 否则：此刻各个待删除的子树里，S\* 里活着、此刻的父节点是待删除的节点（C 新建或恢复的）、待放回而没有临时移动过的节点里，id 最小的那个（候选先按 id 排，S2 审查 C-N1）带临时名称移到根下（`RevertAside`，`Move` 为真，次序值不变）。合法的状态里这样的节点一定是待放回的：S\* 里它的父节点活着，而此刻的父节点是 S\* 里不存在的节点；条件里仍写明"待放回"（S2 审查 A-N2）：一个让它取到不待放回的节点的缺陷，会把那个节点带着临时名称移到根下、不放回就结束，悄悄提交临时名称；有了这一条它不取那个节点（`TestTheSecondFallbackMovesOnlyANodeToPlace`）。
3. 否则：待放回的、此刻不在根下、没有临时移动过的节点里，id 最小的那个带临时名称移到根下。**这一条是防御性的**：有第 2 条时，没有一个可达的状态用到过它（3.16.4 的计数：所有的生成器、审查 B 与 C 的对抗的生成器、穷举的搜索，都是 0 次）；00-M9 4.12（负责人批准的）有它，留作防御，测试以构造的状态直接调 `fallback()` 核对它（3.21.1）。

后备之后被挪的节点在 S\* 的位置之外，回到 P（它本来就在 P 里：第 1、3 条只取 P 里的，第 2 条取到的也在 P 里）。三条的标记各记在节点上：第 1 条用掉"临时改名"，第 2、3 条用掉"临时移动"（第 2、3 条移到根下时也改了名，同时用掉两个：之后不会再在临时名称上再加一层）。

**第 3 条的候选包含第 2 条的**（2.1 第 8 条）：第 2 条取的节点此刻的父节点是待删除的节点，不在根下、没有临时移动过、在 P 里，所以第 3 条也会考虑它；第 2 条的作用是**选得准**：第 3 条按 id 取，可能先挪一个与卡住无关的节点（多一步、多一个段）。模拟里去掉第 2 条仍是 0 次失败（`sim/schedule.py` 的变异 `no-fb2`，2 万次：第 3 条接住了），所以第 2 条的反向对照是一个构造的计划测试（3.21.1 的 `TestTheSecondFallbackPicksTheNodeItFrees`）：去掉它时步的清单不同（第 3 条先挪 id 更小的另一个节点）。

#### 3.16.4 为什么做得完：测试能核对的几条

4.12 的论证落成下面几条，每条都在 domain 的性质测试（3.21.1）的每一次运行里核对，不只靠推理：

- **(T1) 一个节点至多一次临时改名、一次临时移动、一次放回，放回在它的临时的步之后**：第 1 条只取没改过名的、第 2、3 条只取没移动过的；后备只取 P 里的节点，所以已放回的节点不会再被挪开（P 只因后备而增加，而后备只挪 P 里的）。所以一个节点的树的步至多三个（4.12 的"一个节点至多三步"），后备至多 2N 次（N 是 C 碰过的节点数）。"临时改名两次"在合法的状态里到不了：临时名称从不被等着的步要，第 1 条只改名 S\* 里父节点活着（或在根下）的占着者，第 2 条只取父节点要被删除的节点，只有防御的第 3 条会移开一个临时改过名的节点；这条判断留着，它的反向对照是构造的 `TestAFallbackRenamesANodeAsideOnce`（S2 的实施的偏离 8，主持者的决定 2）。
- **(T2) 每一轮要么做成至少一步，要么用掉一次后备，要么结束**；做成的步离开待做的集合，只有后备让一个放回回到 P。所以轮数至多是步数加后备的次数加一，步数至多是 |P₀| + |D₀| + |R₀| + |W₀| 加后备的次数：调度一定结束。**这个上限写成代码**：过了 |P₀| + |D₀| + |R₀| + |W₀| + 2N + 1 轮，`schedule()` 答 `ErrRevertUnordered`（审查 C-I1）。没有一次撤销到得了它；一个打转的缺陷或变异因此在锁下答 500，而不是在笔记本的锁下空转到请求的期限（`planned()` 在锁下跑 `PlanRevert`，`page/app/unit_revert.go:78`、`:178-179`），测试也不会挂到 go test 的超时：这就是测试依靠的轮数的上限。S2 建成：开始时待做的步记在 `revertSim.pending`、上限由它算，调度开始的轮数记在 `revertSim.rounds`；`planRevert` 与测试的钩子 `ScheduleRounds`（`export_test.go`）共用 `scheduled`，计划者的构造、L、S\* 的核对与计划只写一处（修复核对 A 的 N4）。上限的大小由 `TestAScheduleTakesMoreRoundsThanItsSteps` 从下面钉住：审查 A 找到的形状要 6 轮与 7 轮、开始时待做 4 步（多于待做的步数加一），另一行没有 D、要 5 轮；以 `len(p.c.ids)/4` 或 `/3` 为后备项的上限让它们答 `ErrRevertUnordered`。上限的准确大小是 (T2) 的证明，大一些无害：以 N 代 2N、上限不加 1 不测（S2 审查 A-M1、修复核对 A 的 N1，主持者的决定 7）。
- **(T3) 卡住只能是 4.12 的三种边之一**：等待的种类只有表里那几种；恢复等恢复的链沿着 S\* 的树向上（S\* 是树，没有环）；等写回的只有删除，写回只等页活着（只等恢复），不在环里；占着键的只能是待删除的（C 新建或恢复的）或待放回的节点（S\* 是合法的树）。所以一个卡住的环经过"等一个待放回的占着者"（第 1 条拆）、"删除等子树里 S\* 的节点移出"（第 2 条拆）或"等父节点移出子树或上移"（第 3 条拆）。性质测试断言随机的合法输入从不得到 `ErrRevertUnordered`。
- **(T4) `ErrRevertUnordered` 只是缺陷**：三条都不适用而仍卡住时答它（不打转：(T1)、(T2) 保证）。测试以一个非法的状态构造它（S\* 里同一父节点下两个同键的节点，只有缺陷才会有），断言答 `ErrRevertUnordered` 而不是循环或别的步。
- **(T5) 每一步之后不超过层级**：放回、临时的步、恢复的层级前提；测试把每一步用在它自己的模型上之后数一次层级（3.21.1）。

**模拟的核对**（`scratchpad/m9/p3/sim/`；照设计复核 B 的 `sched*.py` 改写：加了正文与回收站里的正文、先写正文再删除的 batch、"先写回、再删除"、临时名称避开 S\* 的键、规范化、B1 与 B3，以及上面各条与段的量）：MaxDepth 2、3、4 的短 batch（1–6 项）与 MaxDepth 3、4 的长 batch（8–24 项、5–12 个节点）各 10 万次，每次连撤四层（C→R1→R2→R3→R4），**0 次失败**：每层之后树、活着的页的正文与 C 之前就有的节点在回收站里的正文都回到 S\*，没有超过层级的中间态，(T1) 与后备至多 2N 都成立。审查的反例照设计走出 `[临时移动 y, 删除新的 G, 恢复 G, 放回 x, 放回 y]`（`sim/ce.py`）。变异（MaxDepth 3 的短 batch，2 万次）：不"先写回、再删除"7,494 次失败（R2 的写回落在已删除的页上，答 `ErrRevertUnordered`）；恢复不核对层级 49 次超过层级；`RevertWrites` 照 M8 只取活着的页 7,555 次正文不对；临时名称不避开 S\* 的键 0 次（防御性的：S\* 的键被占时放回本来就要等，3.17）。

**复核**（`scratchpad/m9/p3/sim2/`，`schedule_cap.py` 加每条后备的计数与 (T2) 的轮数上限，`sh run_all.sh` 重跑；表与各次的数在 `sim2/results.txt` 与审查记录）：五种设置各 2 万次与审查 B 的对抗的生成器 12 万次，0 次失败、0 次到上限，轮数至多是上限的一半。第 3 条在这些、审查 B 的 20 万次搜索与审查 C 的约 405 万对的穷举里都没有用到：它是防御性的，测试以构造的状态直接调 `fallback()`、不计它的次数（变异"第 3 条取根下的节点"在随机的运行里只多走一步、仍正确，抓不到，3.21.1）。S2 的修正的第 1 轮起，domain 的性质测试的全量一次另断言没有一步把节点从根或 S\* 里活着的父节点下移到根下：只有第 3 条后备会这样移，检查按移动的样子数，不按哪条后备（S2 审查 C-N2、修复核对 C 的 N1）。去掉第 2 条时第 3 条接过它的全部 584 次移动，其中 582 次的样子同第 2 条的，2 次看得出，所以第 75 条变异在全量里也被抓到，`-short` 下由 `TestTheSecondFallbackPicksTheNodeItFrees` 抓到。第 2 条的下限：完整的一次（2 万次）至少 50 次（最少的设置 204 次）；`-short` 的 2,000 次轮流跑五种设置，至少 20 次（200 个种子里最少 34 次；只跑一种设置时有低于 20 的种子）。次数成对出现：用到第 2 条的一次运行在 R1 与 R3（或 R2 与 R4）各用一次。

### 3.17 临时名称

```go
// AsideName is the name a revert moves a node aside with for a while (M9
// design 4.12; M9/P3 design 3.17): name, the node's as the revert's plan
// began, with " ~" and the last 8 hex digits of id, which a UUIDv7 draws
// at random (its first ones are its time, those of a batch's nodes
// alike); else all 32; else all 32 and "-2", "-3" and on: the first whose
// key taken says is free. Built as Numbered builds a name, "~" and the
// digits for the number: name cut at a UTF-8 boundary so that the whole
// is at most shared.MaxTitleBytes, an attachment's extension kept after
// the digits ("photo ~1a2b3c4d.png"), one too long for that numbered as a
// page's name is.
func AsideName(name string, id uuid.UUID, asset bool, taken func(key string) bool) string
```

- **后面的 8 位**：`hex.EncodeToString(id[12:])`。UUIDv7 的前 48 位是毫秒的时刻（`uuid.NewV7`），前 8 个十六进制字符约 65 秒不变，一次导入或一个 batch 建的节点都相同；后面是随机的（4.12）。冲突时取全部 32 位（`id` 的 `%x`），再冲突时接 `-2`、`-3`……（一个人或 agent 可以把一页叫作 `x ~<另一个节点的 32 位>`，`-2` 一条为它）。
- **`taken(key)`**：此刻在目标父节点下有别的节点占着这个键，或它是 S\* 在那个父节点下的任何一个节点的键。后一半让临时名称从不挡住一个之后的放回（模拟的变异 `temp-ignores-star` 0 次失败：被挡住的放回会以第 1 条再挪开它；这条让那种额外的一步不出现）。
- **名称取计划开始时的**：节点先被第 1 条改成临时名称、后又被第 2、3 条移到根下时，根下的临时名称仍从它原来的名称造，不在临时名称上再加一层。这在合法的状态里到不了：第 1 条只改名 S\* 里父节点活着或在根下的占着者，第 2 条只取父节点要被删除的节点，只有防御的第 3 条会移开一个临时改过名的节点；`TestTheThirdFallbackMovesAPlaceOffTheRoot` 以构造的状态核对它（S2 审查 C-N5，主持者的决定 2）。
- **长度**：`numbered`（`page/domain/name.go:70-87`）照字节截在 UTF-8 的边界上：整个至多 255 字节（`shared.MaxTitleBytes`；今天的写法从 246 字节起超出 `nodes_name_check`，`00012_page_nodes.sql:14`，00-M9 4.12 已更正移交写的 250）。后缀 `" ~" + 8` 是 10 字节、`" ~" + 32` 是 34 字节，所以 245 字节的名称正好放得下（245 + 10 = 255），从 246 字节起被截短（取 32 位时从 222 字节起）。附件的扩展名长到连主干的第一个字符都放不下时，照页的名称处理：整个名称截短，编号接在后面（`numbered` 的同一条）。`numbered` 先截主干，截空了就照页的名称。以前只在一个字节都不剩时才这样；编号与扩展名只给主干留下 1 到 3 个字节、而主干的第一个字符更宽时，主干被截空，名称以空格开头，不是标题：`中.` 加 242 个 a 的附件，临时名称是 `" ~d3e4f5a6.aaa…"`。M7 的 `Numbered` 是同一条：`中.` 加 251 个 a 的两个附件，第二个的名称造成 `" 2.aaa…"`，导入经 `CheckTitle` 去掉开头的空格，存下的是 `2.aaa…`（一个也没有以空格开头）。两处一并改正，存下的名称一个也不改，改的只是很少见的新名称怎么造：现在是 `中.aaa…a 2`（修正的第 1 轮，审查 A 的 A-N1；修正的第 2 轮更正了"存成以空格开头"的说法）。
- **只在单元之中**：同一个节点之后的放回改成 S\* 的名称；条目合并成第一次的"前"与最后的"后"，观察者拿到合并的改动，所以临时名称不进条目、索引、搜索与事件（4.12）。同一单元里先恢复、再写回的页也是这样：守卫看到恢复的一步与它的事实（回收站里的正文的），链接索引、搜索与事件只看到单元合并的改动，它的事实是写回的（S2 审查 B-N2，3.18.3）。守卫看得到每一步：M10、M11 的守卫要认得撤销里的临时名称（`Write.Kind` 是 `revert`，第 8 节）；它们只凭名称认得：临时的步以普通的 `OpRename` 或 `OpMove` 的一步到达守卫（S2 审查 B-N4，第 8 节）。撤销不经链接的参与者（`Options{}`，`page/app/revert_changeset.go:89-90`），临时名称不会被改写进链接。后台的撤销里一段整个在一批里（3.22），批与批之间也没有临时名称留下。

### 3.18 先恢复、再写回；撤销的撤销

#### 3.18.1 `RevertWrites`

```go
// RevertWrites are the versions whose bases a revert writes back (M9
// design 4.12): every version of the changeset on a base. A page the
// changeset created has none, its version a first one; a page it deleted
// after writing it is restored, then written back; a page it restored
// and wrote, a revert's, is written back, then deleted.
func RevertWrites(items []ChangesetItem, versions []ChangesetVersion) []ChangesetVersion
```

`readReverted` 的条件（`revert.go:280-286`）从"`v.Base != nil && (!item || it.Before != nil && it.After != nil)`"改为"`v.Base != nil`"，S1 加的那条临时的检查同时删去（第 0 节）。`RevertRefs` 照旧为每个写回的页读 `Base` 的哈希（`:223-224`），所以 C 删除的、有版本行的页的 `Base` 的哈希也读到。新建的页的版本行没有 `base`（`changesets.sql:46-57`），不在其中；同一变更集里一页至多一个版本行（P3 的正文合成与推后的改写都更新那一行，3.5.1）。

#### 3.18.2 写回写在哪个版本号上

`RevertOp` 加一个成员 `Current`，计划在模拟里定下：写回写在的版本号。此刻活着的页是 `Contents[id].Revision`；同一单元里恢复的页是它在回收站里那一项的版本号（`TrashMember.Revision`，恢复不改版本号）。"已经相同"（不写）的比较照旧对活着的页的 `Contents[id].Hash` 做；恢复回来的页总要写回：它在回收站里的正文是 C 写的，与 `Base` 不同（否则没有那个版本行），L 又保证回收站里的就是它（审查 B-N8：不另加 `CurrentHash`）。

单元的写回（`unit_revert.go:106-107` 今天的 `Base: state.Contents[op.NodeID].Revision`）改为 `Base: op.Current`：恢复的页在 `state.Contents` 里没有，今天的写法以 0 为基、答 `ErrRevisionMismatch`、成为未分类的 500（4.12）。锁下的计划与不加锁的计划以同样的状态算出同样的 `Current`（单元在锁下重新计划，`planned`，`:76`），所以版本号不符仍是缺陷。

#### 3.18.3 解析按（页、哈希）

一页可以在一次撤销里解析两份正文：恢复的一步要回收站里那份正文的 Facts，写回要 `Base` 的。守卫看到恢复的一步与它的 Facts；链接索引是观察者，一个单元照合并的改动跑一次，它的 Facts 是写回的（`Change.Then`，`page/domain/change.go:67-73`；`linking/app/index.go:46-55`），所以索引、搜索与事件只看到写回的正文，恢复的那份只到守卫（S2 审查 B-N2）。这也是第 81 条变异在组合根的"先写正文再删除"那一行看不出的原因：由 `TestARevertRestoresAPageThenWritesItBack` 读守卫看到的每一步的事实抓到（S2 的实施的偏离 10）。今天解析好的正文按页 id（`RevertRead.Parsed`，`unit_revert.go:30-36`；`parsedAs`，`:232-244`；任务的 `revertContents`，`jobs.go:540`），第二份会盖掉第一份。改为按 `ContentRef`：

```go
// ContentRef is a page's content by its hash: what a revert parses, as a
// page may come back with one content and be written back with another
// in one revert (M9 design 4.12; M9/P3 design 3.18.3).
type ContentRef struct {
	NodeID uuid.UUID
	Hash   string // hex, as a map key
}
```

`RevertRead.Parsed map[ContentRef]Parsed`；`plannedContents`、`parsedAs` 按它配对：锁下的计划要的一份在不加锁时没有以同一个哈希解析过，照今天答 `changed` 的冲突（`parsedAs` 的约定，`unit_revert.go:191-196`：其间别人改了那一页），不改成缺陷（审查 B-N3）；恢复的一步取（成员、回收站的哈希），写回取（页、`Base` 的哈希）。`DecidedAll` 本来就按条目解析（`page/app/content.go:87-131`），两份是两条：`DecidedContents` 是 `DecidedAll` 旁的泛型（两者都是 `decidedAll[K]`），按 `ContentRef` 交回；`DecidedAll` 的签名不改，回收站的恢复不动（S2 的实施的偏离 5）。

#### 3.18.4 界线

两份都算进页数与字节（`withinBounds` 已分开数恢复与写回，`page/app/revert_changeset.go:259-275`）；链接是两份的链接之和。"只有一页时不限"按不同的页：`domain.UnitTooLarge`（`page/domain/trash.go:69-73`）今天按份数判一页的例外，同一页恢复与写回的两份 5 MiB 的正文会被当作两页、以 10 MiB 拒绝，所以它加一个参数，份数与不同的页数分开传，只有一个不同的页时不限（batch 的界线用同一个，2.1 第 5 条）。所以"先写正文、再删除"的一页在撤销里算两份，比 M8 多：100 页的这种 batch 的撤销是 200 份、过了页数的界线，在后台做（同 M8 的大的撤销，4.9）。

同一页的两份正文在一次 `DecidedContents` 里依次取解析的预算（大的先）。第一份解析完，留着它的事实的份额（约十分之一），第二份的取要在这份额旁边放得下。预算取配置收的最小值 5 MiB（`config.MinParseBudgetBytes`，一页最大的正文）时，两份都接近 5 MiB 的一页（各过约 4.55 MiB）放不下；即使没有别的请求在解析也一样，撤销每次都答 503 `server_busy`（修正的第 1 轮，审查 B 的 B-M1）。

所以 `decidedAll`（`DecidedAll` 与 `DecidedContents` 共用）在一次取因本次调用留着的份额而放不进预算的大小时，先把这些份额还掉，事实留着、不再计入，同 S1 的 `ParseSession` 每次取之前还掉（3.4.5）。放得下时，或挡着的是别的请求的份额时，照旧等、答 `server_busy`。页模块的端口为此加 `ParseBudget.Takes` 与 `BudgetHold.Held`，适配器以平台的 `Budget.Takes`、`Budget.Size` 与 `Hold.Held` 答（S1 为 batch 的改写加的）。

M8 的撤销与回收站的恢复走不到这一条，照旧解析：
- 一页只有一份正文；
- 几页时，所有的份数都在单元的 8 MiB 之内，大的先取：下一次取的 x 至多 4 MiB，前面的份额约是 8 − x 的十分之一，合起来至多约 4.4 MiB，在最小的预算之内。

S3 的任务的 `revertBatch` 用同一个 `DecidedContents`，一并得到。

#### 3.18.5 撤销的撤销："先写回、再删除"

撤销 R1 恢复了一页 X、再写回了它（C 先改正文再删除的那种）。R2 撤销 R1 时，S\* 里 X 在回收站里、正文是 R1 恢复时的那份（M8 把回收站里的正文算作状态，`revert.go:479-497`）。R1 的 X 的版本行的 `base` 是那份正文的版本号，所以 R2 的写回取它（3.18.1）；X 是待删除的（R1 恢复的），删除的前提要求子树里没有待做的写回，于是 R2 先写回、再删除；之后从回收站恢复得到的是 C 删除时的正文，不是 R1 写回的（4.12）。这里的写回写在 X 此刻的版本号上（它活着，`Current` 照活着的页取）。

### 3.19 步数

```go
// Steps is how many of the plan's steps MaxRevertSteps bounds: its
// places, its steps aside, deletions and restores. A content's write is
// bounded as the pages a unit parses are.
func (p RevertPlan) Steps() int
```

- 请求里的撤销：放回、临时的改名或移动、删除、恢复各算一步，合计至多 `MaxRevertSteps` = 1,500，未加锁时（`CheckRevert`）与锁下（`Unit.Revert`，`unit_revert.go:82-83`）照旧核对。一个节点至多三步（3.16.4 (T1)）；batch 至多 100 项，它的撤销远在界线之内（最坏约 300 步加删除与恢复）。`MaxRevertSteps` 的注释加"临时的步在内"。
- 后台的撤销不对整个计划核对 `MaxRevertSteps`（4.12：那会让任何平铺的大导入都撤销不了，HI5 正是后台撤销 1,501 个笔记的导入，`e2e/stories/history/hi5-revert-import.spec.ts:27-28`；任务的计划今天有意不传步数，`page/app/jobs.go:296`）。每批的步由这一批的节点数管着（transfer 的 `Batch.Nodes` = 100），临时的步按批计入，合计在 `MaxJobItems` 之内（一个节点至多三步，`MaxJobItems` 是条目，所以至多 18 万步：每一步一次单元的操作，按批的节点数切）。HI5 照旧通过是它的守卫（S3）。

### 3.20 `changesets.renames` 与 `moves`

今天网页的撤销对话框（`web/…/revert-dialog.tsx:315`）与 `changeset` 的答复读"前 1,000 项里有没有改名、移动"（`ChangesetMoves`，`page/adapter/postgres/queries/history.sql:73-81`，按节点 id 的前 1,000 项）。batch 的变更集可以很大（条目至多 1 万），第 1,001 项之后的改名不该被漏掉（M8/P3 的移交"另"第 1 条）。

#### 3.20.1 迁移 `00045_page_changesets_moves.sql`

```sql
-- changesets.renames, moves: whether a changeset renames a node, or moves one to another parent, of all its items
-- (M9/P3 design 3.20): the web's revert warns that links written to the new place stay. A unit sets them as it
-- ends, from its merged changes (page/app/unit.go); until 00045 a read took them from the first 1,000 items.

-- +goose Up
ALTER TABLE changesets ADD COLUMN renames boolean NOT NULL DEFAULT false,
    ADD COLUMN moves boolean NOT NULL DEFAULT false;
-- The changesets so far, each from all its items: a changeset's units each merged their changes into its items,
-- which hold them as the read did.
UPDATE changesets c SET renames = m.renames, moves = m.moves
FROM (SELECT i.changeset_id,
             coalesce(bool_or(i.before_name <> i.after_name), false) AS renames,
             coalesce(bool_or(i.before_name IS NOT NULL AND i.after_name IS NOT NULL
                 AND i.before_parent_id IS DISTINCT FROM i.after_parent_id), false) AS moves
      FROM changeset_items i GROUP BY i.changeset_id) m
WHERE m.changeset_id = c.id AND (m.renames OR m.moves);

-- +goose Down
ALTER TABLE changesets DROP COLUMN renames, DROP COLUMN moves;
```

补值是一次对 `changeset_items` 的整表聚合（同 00038 的补值，`00038_page_changeset_items_deleted_revision.sql:13-16`）：大的库上它是迁移的一部分，写进升级的说明（M12 的部署文档已有"迁移在启动时跑"的说明，不另加）。默认值 `false` 让列在加的那一刻对所有行都成立，`NOT NULL`。S2 量了补值：00044 的库上 100 万个变更集、各 3 项（300 万条条目，十分之一改名、十分之一移到别的父节点），升到 00045 用 1.87 与 2.19 秒（审查 C 重量 2.08、3.10 秒）；`ADD COLUMN … DEFAULT false` 是常量的默认值，不重写表。补值在 goose 的一个事务里持着 `changesets` 的 ACCESS EXCLUSIVE 锁，只在启动时：约每条条目 0.6 µs，5,000 万条约 30 秒（审查 B 的注记，主持者的决定 11），写进 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 18 项。

#### 3.20.2 单元结束时设上

`changesets.sql` 加：

```sql
-- name: MarkChangesetMoves :exec
-- A unit's renames and moves, from its merged changes, kept once true (M9/P3 design 3.20): the units of a job's
-- changeset each add theirs.
UPDATE changesets SET renames = renames OR sqlc.arg(renames)::boolean, moves = moves OR sqlc.arg(moves)::boolean
WHERE id = sqlc.arg(id) AND (sqlc.arg(renames)::boolean OR sqlc.arg(moves)::boolean);
```

`Writer.Run` 在 `do` 之后、搜索的入队之前（`unit.go:218-221` 之间），以单元合并的改动（`recordItems` 的同一份）算：有一个改动前后都在而名称不同是 `renames`，前后都在而父节点不同是 `moves`（同补值的两个条件）；都没有就不发语句。根下与文件夹下的换次序都不设，文件夹下的改名只设 `renames`；单元测试 `TestAUnitMarksItsChangesetsRenamesAndMoves` 核对这几种，并断言标记在 `PagesChanged` 与搜索的入队之前（S2 审查 B-M2、C-M2、B-N1 = C-N4）。"一旦为真就留着"对跨单元的变更集是准的：编辑会话只写正文；导入只建；后台的恢复只恢复；后台的撤销里一个节点的树的步（临时的改名、移动与它的放回）在同一段、同一批（3.22），所以不会有一批改了名、另一批又改回的（它的写回可以在别的批，不碰名称与父节点）。batch 的单元在最后删去回到原处的条目（3.5.4），它们的改动也不算。`TestTheChangesetsRenamesCountEveryItem`（3.21.4）核对这一点。

#### 3.20.3 读它的

- `ChangesetMoves` 的语句（`history.sql:73-81`）改为读这两列（`SELECT renames, moves FROM changesets WHERE id = $1`），端口与类型的注释去掉"前 `MaxLogCount` 项"（`page/app/ports.go:418-420`、`page/domain/changeset.go:84-90`），`HistoryReads.detail` 的注释（"whether its first items rename or move a node"，`page/app/history.go:85-87`）同改，用例照旧（`page/app/history.go:89-93`）。读它的三处不改代码：REST 的 `getChangeset`（`page/adapter/http/history.go:63`）、MCP 的 `changeset`（`page/adapter/mcp/changeset.go:49`）、网页的撤销对话框。
- **一处 REST 的答复会变**（审查 B-N7）：被清理截过的变更集，今天的读照清理留下的条目算，列留着它当初的值（"改过名"）。被截的变更集撤销不了（`page.changeset_trimmed`），对话框不会为它打开，所以无害；契约的说明照列写。
- 契约：`api/modules/page.yaml:1867-1872` 的两个说明改为"Whether it renames a node."、"Whether it moves a node under another parent."；重新生成 Go 与 TS（`make gen`，`gen-check` 照常核对）。`getChangeset` 与 `ChangesetDetail` 的说明也写着"前 1,000 项"，一并改为全部条目（S2 的实施的偏离 3）。README 的"日志"一条（`README.md:172`）"另答前 1,000 项里有没有改名、移动"改为"另答它有没有改名、移动"；v0.1 总体设计 13.1 第 31 条"单个变更集的 `renames`、`moves` 只看前 1,000 项条目"删去（第 7 节）。

### 3.21 S2 的测试

#### 3.21.1 domain（不要数据库）

- **钉住 M8 的行为的三处**（审查 C-M3），各 Step 的期望：
  - `page/domain/revert_test.go:342-347` "a page written, then deleted (M9's shape)"（今天要 `[]string{"restore P"}`）：S1 是 `ErrRevertUnordered`（临时的检查，这一行在 S1 改期望）；S2 是 `restore P`、`content P`（先恢复、再写回；测试的清单把写回记作 `content`）。
  - `revert_test.go:864-886` `TestPlanRevertOfASwapIsUnordered`（名称互换、两个同名的页互换父节点，今天要 `ErrRevertUnordered`）：S2 两种都排得出，都是第 1 条后备（把一个占着键的待放回的节点临时改名，再两次放回）；测试改名 `TestPlanRevertOfASwapGoesAside`，断言步的清单。
  - `page/app/revert_test.go:456-464` "a round out of order"（一次名称互换，今天要 `ErrRevertUnordered`）：S2 改为一个存着的缺陷"比 `MaxDepth` 深的页被改名"（"a defect's schedule: a page deeper than MaxDepth renamed"）：放回在临时的步前后都放不下，仍要 `ErrRevertUnordered`，用例那一条 500 的路照旧有测试。S\* 里同一父节点下两个同键的节点在用例上到不了调度（`before()` 先答 `title_taken`），那个构造在 domain 的 `TestScheduleAnswersUnorderedForADefect`（S2 的实施的偏离 2）。
- `TestScheduleIsTheRoundForM8sChangesets`：M8 的每种一种操作的变更集（导入、编辑会话、改名、移动、删除、恢复、撤销的撤销、MCP 的移动加改名）的步与 `round()` 的逐个相同（搬 `revert_test.go` 的样例，在 `round()` 删去之前以它生成期望的清单，写进测试里；上面第一处不搬，它不是 M8 的形状）。
- `TestScheduleRevertsEachBatchShape`：4.12 的每种在 domain 里：互换名称（在根下，断言步的清单：在文件夹下时去掉第 1 条后备，第 3 条会以别的步接住，第 74 条变异（去掉第 1 条）就抓不到，审查 C-N14）、改名之后新建同名、删除之后新建同名、移入新建的文件夹、移动加改名的中间态、审查的反例（步的清单恰是 3.16.4 的那五步）、先写正文再删除（恢复、写回，`Current` 是回收站的版本号）、撤销的撤销（写回、删除）、层级（父节点一线先上移才放得下的放回与恢复）、MaxDepth 上带附件的页被删除、再恢复（MaxDepth 2 的 `Q/P/a.png`：恢复的层级不数附件，S2 审查 A-M2）。恢复到另一组之下不在其中：M8 的 L 把上下相套的两个回收站项读成一组，`PlanRevert` 在调度之前就答冲突；"恢复等它的父组"由包里构造的计划者 `TestARestoreWaitsForTheGroupItGoesUnder` 核对（恢复 B 等 A 的那一组一轮），真的库上的那一行以 SQL 种下、断言 409（3.21.2；S2 的实施的偏离 1，主持者的决定 1）。
- `TestScheduleEndsAndPutsTheNotebookBack`（性质，模拟的 Go 版）：随机的树与 batch（短的 1–6 项、长的 8–24 项，规范化、B1、B3 与生成器同模拟），连撤四层，`planner.maxDepth` 取 2、3、4（测试注入；生产是 `MaxDepth`）。**判定不用被测的模拟**（审查 C-M7）：测试有它自己的模型（节点到父节点、名称、活着与否、正文的哈希），把 `schedule()` 答的每一步依次用在它上面，每一步之后数层级（(T5)），每层之后与生成器的 T0（S\*）比：树、活着的页的正文哈希、回收站里的正文哈希；下一层的变更集也由用上的步造（同模拟造每一层的变更集）。名称或父节点写错的步、漏掉的步都会让比较失败，`schedule()` 自己推进的 `revertSim` 不作判定。S2 的修正的第 1 轮起模型里有附件（审查 A-M2）：附件照 `Subtree.Height` 不算一层（可以在 MaxDepth 的页下多一层），附件的放回与临时的步要留着扩展名。(T1)–(T4) 每一步核对；从不得到 `ErrRevertUnordered`（轮数的上限让打转的变异答它，而不是挂住）。一共 2 万次（本机约 1–2 秒；`-short` 下 2,000 次）。不空转的计数：第 1 条后备与"先写回、再删除"各至少 50 次（都在几千次，`-short` 下也是）；第 2 条在完整的一次至少 50 次，`-short` 下（2,000 次，五种设置轮流）至少 20 次（3.16.4 的量：200 个种子里最少 34）；第 3 条：全量的一次断言没有一步把节点从根或 S\* 里活着的父节点下移到根下（只有第 3 条会这样移；按移动的样子数，S2 审查 C-N2）；它不可达，下下一条以构造的状态测它。
- `TestTheSecondFallbackPicksTheNodeItFrees`（构造：两个待放回的节点，id 小的那个与卡住无关、不在根下；卡住的是待删除的子树里 id 大的那个）：步的清单是第 2 条挪 id 大的那一个；去掉第 2 条时第 3 条先挪 id 小的，清单不同。
- `TestTheSecondFallbackTakesTheLeastFirst`（S2 审查 C-N1）：新的 G 下五个候选、照 id 的反序放，计划 20 次，第 2 条每次先挪 id 最小的：候选不排序时它照 map 的次序取，计划不确定（S3 按 `Seq` 从任务开始时的计划取步，那时确定性要紧）。
- `TestTheSecondFallbackMovesOnlyANodeToPlace`（S2 审查 A-N2；包里的）：一个不待放回的、id 更小的 S\* 节点在待删除的父节点下，第 2 条不取它。
- `TestTheThirdFallbackMovesAPlaceOffTheRoot`（审查 C-I1 的做法，生产的代码里不加开关）：包里的测试构造一个卡住的 `revertSim`（第 1、2 条都不适用：没有占着键的待放回的节点、没有待删除的子树），直接交给 `fallback()`：它挪的是待放回的、不在根下、没有临时移动过的节点里 id 最小的那个，带临时名称到根下；在根下的、临时移动过的从不被取（构造里放一个 id 更小的在根下的、一个 id 更小的已临时移动过的）。最小的候选已临时改成 32 位的那种名称，期望根下的名称是 `AsideName("L", …)`：3.17 的"名称取计划开始时的"由它核对（S2 审查 C-N5）。
- `TestScheduleAnswersUnorderedForADefect`：(T4) 的构造。
- `TestAFallbackRenamesANodeAsideOnce`（包里的）：(T1) 的"临时改名两次"的反向对照，性质测试在合法的状态里到不了它（S2 的实施的偏离 8）。
- `TestAStepAsideTakesNoKeyOfS`（包里的）：临时名称避开 S\* 的键（`AsideName` 的 `taken` 由调用者给，S\* 的那一半在 `aside()`；S2 的实施的偏离 7）。
- `TestARestoreWaitsForTheGroupItGoesUnder`（包里构造的计划者）：恢复等它的父组。
- `TestAScheduleTakesMoreRoundsThanItsSteps`（S2 审查 A-M1、修复核对 A 的 N1）：审查 A 的种子 400 的形状 6 轮、没有 D 的 5 轮、种子 5111 的 7 轮，开始时待做 4、3、4 步，都多于步数加一；经 `ScheduleRounds` 读轮数与开始时待做的步。
- `TestAsideName`：后 8 位；与兄弟的键冲突时 32 位；再冲突时 `-2`（兄弟里有一页叫 `x ~<这个 id 的 32 位>`）；是 S\* 的键时同样换下一个；245（不截）、246、254、255 字节的名称（截短在 UTF-8 的边界上，结果 ≤ 255 字节、是合法的标题）；附件保留扩展名、扩展名太长时照页的名称；`~` 与标题的规则（`shared` 的标题检查通过）；空的主干照页的名称（`中.` 加 242 个 a 的附件；`ü.` 加 250 个 a 留它两个字节的主干），每个结果原样过 `CheckTitle`，`TestNumbered` 同样（S2 审查 A-N1）。
- `TestRevertWritesAreEveryVersionOnABase`：新建的页没有、删除的有、改名并写正文的有、只写正文的有。
- `TestStepsCountTheStepsAside`：临时的两种都算，写回不算。

#### 3.21.2 每种在真的数据库上

`TestTheRevertOfEachBatchShape`（`bootstrap/revert_batch_test.go`；`TestTheR…`，第三份）：每种一个子测试，先以 MCP 的 `batch` 写出那种形状的变更集，再在网页的接口上整组撤销（同步，`POST /changesets/{id}/revert`）：

| 子测试 | batch | 撤销之后核对 |
|---|---|---|
| 互换名称 | `move A.md → T.md`、`move B.md → A.md`、`move T.md → B.md` | A、B 回到原来的名称与节点；撤销的条目里没有临时名称（合并之后） |
| 改名之后新建同名 | `move Plan.md → Old.md`、`write Plan.md` | 新的 Plan 在回收站、原来的 Plan 回到 Plan |
| 删除之后新建同名 | `delete X.md`、`write X.md` | 新的 X 在回收站、原来的 X 恢复 |
| 移入新建的文件夹 | `write F.md`、`move X.md → F/X.md` | X 回到原处、F 在回收站 |
| 移动加改名的中间态 | `move F/B.md → G/B.md`、`move G/B.md → G/A.md`：两项，batch 合并成一个移动加改名的条目，F 下的 A 占着它在 G 下的名称（S2 的实施的偏离 12） | B、A 回到 F 下原处；一步的放回（P2）之外，batch 里的那种也成立 |
| 审查的反例 | G 下有 x、y：移出 x、移出 y、删除 G、新建 G、把 y 移进新的 G | 新的 G 在回收站，G、x、y 回到原处 |
| 长名称与附件 | 名称 246–255 字节的两页互换名称；两个文件夹里的同名附件一起要临时移到根下：上传的接口排不出同一毫秒，以 SQL 种下这两个节点，id 的时刻前缀相同（审查 C-N18） | 临时名称不超过 255 字节、彼此不撞；都回到原处；"changeset reverted" 那一行记 `aside` 3（附件的文件经 `storage.OpenLocal` 写进应用的目录，同 `assets_jobs_test.go`，S2 的实施的偏离 13） |
| 先写正文再删除 | `edit X.md`、`delete X.md` | X 恢复、正文是 batch 之前的（不是删除时的） |
| 撤销的撤销 | 上一行的撤销 R1 再撤销 | X 在回收站、回收站里的正文是 C 删除时的 |
| 恢复到另一组之下 | 经接口做不出（batch 的规范化把"先删子孙、再删祖先"并成一组，B3 拒绝"移出再删"；撤销只删最上层，撤销的撤销至多恢复并列的组，审查 B-M4、C-M11）：以 SQL 种下这个变更集，它的条目与回收站的行（同 3.14.5 以 SQL 种 10,001 个节点），两组一上一下 | M8 的 L（`readReverted.topOf`）把上下相套的两个回收站项读成一组，外面那一项只有它自己的成员：撤销答 409 `page.revert_conflict`（A、B 都是 `changed`），什么都不写（S2 的实施的偏离 1，主持者的决定 1）；"恢复等它的父组"由 domain 的构造的计划者核对（3.21.1） |

每个子测试核对：树（父节点、名称、次序值）、每页正文的字节、回收站里的正文的字节、索引（`checkRebuilt`）、`checkPages`、`checkLinks`、`checkSearch`；撤销的变更集是一个、类型 `revert`；日志里没有 error 级的行，每个子测试只读它开始之后的日志（`logsFrom`，S2 审查 C-N3）。

另有 `TestTheRevertOfALonePagesTwoContentsAtTheLeastBudget`（`TestTheR…`，第三份；S2 的修正的第 1 轮，审查 B-M1）：页 X 约 5 MiB，agent 的 batch 把它写成另一份同样大小的正文、再删除它，在最小的解析预算（`MinParseBudgetBytes`，5 MiB）下网页整组撤销答 200，X 恢复、正文是 batch 之前的。应用用生产的期限（请求 15 秒、写 60 秒），同 `TestTheMCPBatchIsRefusedWhole`：写 5 MiB 正文的 batch 在 CI 的竞态检测下过了测试的 4 秒（run 39481193146）；默认预算的那一种在单元测试里（修正的第 2 轮，S2 修复核对 C 的 N2）。

#### 3.21.3 撤销的性质测试

`TestRevertingTheNewestChangesetsPutsTheNotebookBack`（`bootstrap/revert_property_test.go:238-290`；第二份）：

- 操作的表（`:81-190`，今天 `IntN(11)`，`:105`）加两种：`batch`（打开 `revertWorld.batches`，3.14.6；随机的 2–6 项，经 MCP 的 `batch`，生成器照 B1、B3 与规范化避开会被拒的形状，被拒的照 `agentMove` 的做法以它自己的码跳过：`title_taken`、`cycle`、`too_deep`、`not_allowed`）与定向的生成器，轮流发两种 batch：移出、删除父节点、新建同名、移回（4.12）；`edit X`、`delete X`（"先写正文再删除"：随机的 2–6 项里少见，审查 C-M5 估一个种子撤销到它一次或一次都没有，而 S2 的"先恢复、再写回"与"先写回、再删除"正是为它）。第 9 种（撤销）在 S2 也挑 batch 的变更集。
- 比较：`notebookState`（活着的树、正文的 md5、`page_links`、`asset_blobs`）之外加回收站里的正文：只比撤销之前就在回收站里的页，按 id（`trashContents`：每个这样的页在回收站里的正文的 md5），从新到旧连续撤销 k 个之后等于之前的（4.12）。撤销之后回收站里还多了被撤销的变更集新建的页，今天的测试另核对它们（`revert_property_test.go:272-281`），所以不比整个回收站（审查 C-M5）。
- 不空转（照 `:281-289` 的"没有种子撤销过 agent 的移动"）：种子合起来至少撤销过 3 个 batch 的变更集、至少一次用到了临时的步、至少一个"先写正文再删除"的变更集（S2 建成的按种类数，见下面的"S2 建成"）。临时的步从"changeset reverted"那一行 info 日志数（`page/app/revert_changeset.go:129-133`）：它在 `placed`、`deleted`、`restored`、`written` 之外加 `aside`（临时的步的个数，生产的日志里也有用），测试经一个记日志的团队读它（`newAcmeTeamLogging`，`bootstrap/interleavings_workspace_test.go:71`）。不从条目里认：条目合并成第一次的"前"与最后的"后"，临时名称到不了条目（`changesets.sql:11-44`）；也不数守卫的写：那要测试的应用没有的接缝（`bootstrap/wire.go:58-71`）（审查 C-M5、B-N4）。
- S2 建成（实施的偏离 9，审查 C-M1）：两种定向的 batch 在 k 个变更集的开头先做（每种至多试三次）；"先写正文再删除"的那个 batch 立即撤销一次，从新到旧连续撤销时链里就有撤销的撤销（先写回、再删除），不这样，第 72 条变异在这个测试里活下来。下限按种类数：世界记下每个 batch 的变更集的种类，k 阶段先做的另记为"强制的"；agent 的移动是不属于任何 batch 的 MCP 变更集（S2 打开 batch 之后每个 batch 都是 MCP 的变更集，原来的数法让"agent 的移动"的下限永不失败）。四个种子合起来至少：agent 的移动 2、agent 的随机 batch 1、定向的 batch 1、强制的 4，临时的步与先写正文再删除的各 1（今天 3、2、2、8、5、5）。
- 用时：本机今天 5.2–5.8 秒，加两种操作约 +2–4 秒。S2 建成时竞态下 5.28–5.45 秒（随机的序列随新的两种操作变了，`main` 上 8.0–9.1 秒），修正的第 1 轮之后 7.02 秒。

#### 3.21.4 迁移与列

- `TestTheChangesetsMovesAreBackfilled`（`server/migrations/changesets_moves_test.go`，照 `TestTheDeletionsRevisionsAreBackfilled` 的写法，`downTo(t, m, 45)`：`downTo` 撤销它点名的那一版，`server/migrations/background_jobs_test.go:14-25`；00038 的测试以 `downTo(t, m, 38)` 写成 00037 的样子）：以 00044 的样子写下改名的、移动的、只建的、只删的、第 1,001 项才改名的变更集，升到 00045：列等于各自全部条目的；降回去两列不在。只建、只删的两行放在 P 之下（父节点不为 NULL），去掉补值的两条 `IS NOT NULL` 守卫（任一条也是）时失败（S2 审查 B-M3）。
- `TestTheChangesetsRenamesCountEveryItem`（`bootstrap`；`TestTheC…`，第二份）：REST 的改名与移动、MCP 的移动、`batch`（改名在按节点 id 的第 1,001 项之后：以 SQL 种下 1,000 个更小 id 的节点给它删）、导入、后台的撤销任务之后，每个变更集的两列等于它全部条目的 `bool_or`。S2 里的后台的撤销任务撤销一个没有临时的步、也没有"先恢复、再写回"的变更集（一个改名的 batch）：S2 的临时的拒绝会让别的形状的任务失败（审查 C-N15）；带临时的步的那一种在 S3 加进这个测试。S2 的修正的第 1 轮加经 REST 的 `Folder` 下的改名（`renames` 真、`moves` 假）与换次序（两个都假），以每个变更集做了什么核对 10 个（S2 审查 B-M2、C-M2）。

#### 3.21.5 交错

`TestTheMultiRoundRevertMeetsAWrite`（`bootstrap/interleavings_revert_test.go`；`TestTheM…`，第三份；4.15 的"多轮撤销与撤销之后的写"）：撤销一个要临时改名的 batch 的变更集，两种次序：网页的改名先在笔记本行上等、撤销在它之后：改名先提交，撤销在锁下重新计划，答 409 `page.revert_conflict`（`changed`），什么都没写；撤销先持着笔记本行：改名等它、之后在撤销之后的树上照常，没有临时名称留下。结束时 `checkPages`、`checkLinks`、`checkSearch`。

### 3.22 S3：任务的计划与不可分的段

S3 的文件：`page/domain/revert_schedule.go`（`schedule()` 记下每个删除那一刻的子树，`RevertOp.Subtree`）、`page/domain/jobs.go`（`JobStep.Joined`）、`page/app/jobs.go`（`revertSteps`、紧凑的 `revertStep`、S2 那条临时的拒绝删去）、`page/app/revert_changeset.go`（`locked()` 照删除那一刻的子树）、`page/app/unit_revert.go`（`RevertBatch`、`batchOps`、一组在一批里一次恢复）、`page/domain/revert.go`（`CheckRevertPart`）、`transfer/app/import_ports.go`（`JobStep.Joined`、`ErrBatchTooLarge` 的注释）、`transfer/app/background.go`（`batch`、`stopped`）、`transfer/domain/report.go`（`FailureRevertTooLarge`）、`bootstrap/transfer.go`（步的适配抄 `Joined`）、`api/modules/transfer.yaml` 与生成的 Go、TS、`web/…/transfer-report.tsx`、`web/…/i18n/messages/en.ts`、`zh-CN.ts`、它们的测试与 mock；没有迁移（段只在内存里，任务的计划在任务开始时算、不存，`transfer/app/background.go:86-126`）。

#### 3.22.1 计划就是 `schedule()` 的步

任务开始时 `JobWrites.PlanRevert`（`page/app/jobs.go:281-301`）照旧以 `check` 算出 L、S\* 与计划，计划的步就是 `schedule()` 的（3.16，带 `Seq`）；S2 加的临时的拒绝删去。`revertSteps`（`:305-358`）把它展开成 `JobStep`：

- 放回、临时的步：一个节点一步。
- 删除：**它在 `schedule()` 里那一刻的子树**一步（`RevertOp.Subtree`：模拟走到这一步时这个节点的子树，审查 B-I1 的第 1 条）。今天给的是计划读到的树上的子树（`jobs.go:325-330`），而 `schedule()` 在删除之前把 S\* 里活着的节点放回或挪开（"移入新建的文件夹"），那些节点就被多算进删除的步：步的节点数、`JobStep.Nodes`、开始时的 `locked()`（`page/app/revert_changeset.go:312-320`：只是被撤销移动的页，有人正在编辑它，后台撤销就在开始时答 `page.locked`，而移动是过得了编辑锁的守卫的，00-M9 4.10 的表）与这一批读的条目，都因此不对。那一刻的子树里只有要删的节点；之后有人在它下面加了节点，第 4 步的 `canDelete` 照 `children_added` 抓到。
- 恢复：一组按先序一个成员一步（同今天，`GroupPreOrder`）；**这一页有写回时，写回紧跟在它那个成员的恢复之后**（不是排到这一轮的全部恢复之后，4.12）。挪前不改变别的步的前提：写回只要页活着；等写回的只有删除（"先写回、再删除"），挪前只让它更早成立。
- 写回：一页一步。

**任务从开始时的计划只留紧凑的几样**（审查 B-M2）：每一步的次序、种类与 `seq`；放回与临时的步的目标父节点与名称；写回的 `Base` 与哈希；删除那一刻的子树；恢复的步的组、成员的位置与成员的 id（`node`）。同 M8/P4 的 `GroupNode`（每个节点约 40 字节、不留名称，`page/domain/jobs.go:51-66`），不留整个 `RevertOp`（带 `Group` 与 `States`，名称在内）。`revertPlan.groups`（`GroupNode` 的表，每个成员约 40 字节）照今天留着：成员 0（一组的第一个成员）在这一批里时，这一批要读这一组全部成员的条目与版本（3.24 第 1 步，审查 N-M1）。恢复的组、时刻与状态照今天由每一批的读给（`restoreSegment` 用这一批读到的 `op.DeletedAt` 并记进 `r.retimed`，`unit_revert.go:770-774`；`begun()` 读根此刻的 `DeletedAt`，`jobs.go:505-532`）：这是 M8/P4A 审查 A-M1 的修正，"计划之后那一组被恢复又在原处删除"时恢复用的是新的时刻；取开始时的计划会让 `RestoreNodes` 一行都对不上、答 `page.trash_changed`，`RunBatch` 把它译成 `changeset_trimmed`（`jobs.go:378-380`），M8 做得完的撤销 S3 反而失败。

```go
// revertStep is a step of a background revert as its job keeps it from
// the plan at its start (M9/P3 design 3.22): its kind and place in the
// schedule; a place's or a step aside's node, its parent and name there;
// a write back's page, base and hash; a deletion's top and its subtree at
// the deletion's moment; a restore's member, its node, and its group,
// whose members revertPlan.groups keeps as today. A restore's time and
// states, and every item and version, are the batch's reads.
type revertStep struct {
	kind   domain.RevertOpKind
	seq    int32
	node   uuid.UUID
	parent uuid.UUID // a place's or an aside's, the zero id for the root
	name   string
	base   int32
	hash   []byte
	nodes  []uuid.UUID // a deletion's subtree at its moment
	group  int32
	member int32
}
```

`seq` 是它在 `schedule()` 的步里的序号，一组的成员共用恢复的序号、另有成员的位置（今天的 `member`）。`RevertBatch` 按它取步（3.24），同一个节点的临时的步与最后的放回不再在（类型、节点）上相撞（`batchOps`，`unit_revert.go:555-574`）。

```go
// JobStep is a step of a background job (M8/P4 design 3.3; M9/P3 design
// 3.22), a write no batch splits: a restore's node; a revert's place of a
// node or step aside, deletion of a top's subtree, node of a group it
// restores or page it writes back. Nodes are the nodes it writes, a write
// back's 1; Bytes the content it parses; Joined that the next step is in
// its batch too: a segment of steps no batch splits.
type JobStep struct {
	Nodes  int
	Bytes  int64
	Joined bool
}
```

transfer 的 `JobStep`（`transfer/app/import_ports.go:73-76`）加同一个成员，组合根的适配照抄。

#### 3.22.2 不可分的段

**为什么要段**：每批在锁下对它自己的节点做 L（3.24）。一个节点的树的步若分在两批，后一批开始时它已不是 C 写出的样子（例如带着临时名称在根下），L 会把它当作 `changed`，或者前一批之后留下临时名称（4.12）。所以三样东西各在同一批里：**一个节点的树的步**（它的临时的步与放回）、**一页的恢复与它的写回**、**一个删除与它带走的写回**；三种段就是它们。不是"每个节点的全部步"：一个既被移动或改名、又被改了正文的节点，它的放回（单独的，或在一段里）与它的写回常在不同的批，这无害，因为写回的一步读的是版本，不读条目（审查 B-N2）。

1. **临时的步的段**：从一个节点的临时的步到它最后的放回，包括其间计划里的全部步（它放开的删除、这些删除放开的恢复与写回、被它放开的放回）；重叠的合成一段。段里的恢复只展开到段里后面的步要的成员（要它活着的：放回的目标父节点、恢复的 S\* 父节点、写回的页；按先序到最后一个要的为止），这一组其余的成员排在段之后、不连着（它们不被段里的步要，晚一点恢复不改变段里任何一步的前提：段里的放回在它们还在回收站时键已空着，它们之后的键也仍空着，3.26 的 replay 测试核对；它们落在同一批时，照 3.24 的第 6 条随这一组的第一步一起恢复）。
2. **恢复与写回**：一页的恢复与紧跟的写回（一个节点的两步）。
3. **写回与删除**：撤销的撤销里，一个待删除的子树里的页的写回与那个删除（"先写回、再删除"：删除带走这些页，它们的步要在一批里）。

`revertSteps` 在每段除最后一步之外的步上设 `Joined`。

**段的大小**（2.1 第 10 条更正 4.12 的"一个名称环的段只有几步"）：模拟里（`sim2/batches_rev.py`，三种段、删除照那一刻的子树、四层都算），各生成器最大的段 10–19 步、13–22 个节点（各生成器的数在 `sim2/results.txt`）。真的 batch 能比模拟的长（100 项）：一个 L 个节点的改名环是一段 L + 1 步，L 至多约 100，放回不读历史、不解析，所以它总放得进一批（3.23）。一段只有带上大的恢复（历史）或写回（链接）时才可能放不进一批：

- 第 1 种：第 2、3 条后备的段从临时移动一直到那个节点最后的放回，其间有它放开的删除与这些删除放开的恢复（审查的反例就是"临时移动 y、删除新的 G、恢复 G、放回 x、放回 y"）；第 1 条放开的那一步也可以是一组的恢复。段里要的成员在先序里很靠后时（x、y 的 S\* 父节点是 G 下最后一个子页，前面是几百页），恢复的大部分都在段里。
- 第 3 种：写回的页的链接之和过了一批的链接。

### 3.23 S3：transfer 只在段与段之间切批

`backgroundRun.batch`（`transfer/app/background.go:134-156`）：

- **长一批**：从 `from` 起一段一段地加（一段是到第一个 `Joined` 为假的步为止），第一段总在里面（同今天的"第一步总在里面"），加下一段会过 `Batch.Nodes` 或 `Bytes` 就停。
- **减半**（transfer 自己的 `ErrBatchTooLarge`，组合根由 page 改名之后的 `ErrJobBatchTooLarge` 译来，`bootstrap/transfer.go:231-232`）：`to = max(第一段的末尾, 不大于 from + (to-from)/2 的最后一个段的边界)`；`to` 已经是第一段的末尾时不能再分：第一段只有一步、或它的步都是同一个节点时，`RunBatch` 不会答太大（下一条）；否则任务以 `revert_too_large` 停下（审查 B-I3）。取 `max(…)`：第一段长过这一批的一半时，不大于一半的最后一个边界总是 `from`，在那里停下会停下放得下的批（`[r X, w X]` 一段、再 `[r Y]`：`[0,3)` 太大，而 `[0,2)` 与 `[2,3)` 各自都放得下）。复核（`sim2/halving_rev.py`，三种生成器各 10 万个计划）：这条规则只在第一段自己就太大时停下（数在 `sim2/results.txt` 与审查记录）。

```go
// batch writes the batch from the plan's step from: its segments, a run of
// steps each Joined to the next, up to the bounds' nodes and bytes, the
// first always in it, as RunBatch writes them (M8/P4 design 3.3; M9/P3
// design 3.23). One past a unit's history or links ends at the later of
// the first segment's end and the last segment's end at most half way;
// one that ended at the first segment's end already, of more than one
// node, stops the job, revert_too_large.
func (r *backgroundRun) batch(ctx, running context.Context, spec JobSpec, plan JobPlan, from int) (int, BatchDone, error)
```

- **一个节点不受批的界线**（2.1 第 12 条）：`RunBatch`（`page/app/jobs.go:370-382`）今天"多于一步"才答 `ErrBatchTooLarge`（`withinHistory` 的 `steps == 1`，`:424-440`；链接的那一处是 `DecidedAll` 的"多于一页"，`:402`、`:478`）；改为"步碰的节点多于一个"（链接照 3.18.4 按不同的页）：同一页的恢复与写回是两步、一个节点，它就是 M8 的"单独的一步"，总要写得下（M8 的界线本来让单个节点总放得下：恢复一个节点的历史至多是它一页的，写回一页的链接是它一页的）。
- **新的失败**：`transfer/domain/report.go` 加

```go
	// FailureRevertTooLarge is a revert one of whose segments, steps no
	// batch splits, holds more history or links than one batch writes
	// (M9/P3 design 3.23): what it wrote before kept.
	FailureRevertTooLarge Failure = "revert_too_large"
```

  `stopped`（`background.go:182-211`）在 transfer 的 `ErrBatchTooLarge`（`transfer/app/import_ports.go:170`，组合根由 page 的 `ErrJobBatchTooLarge` 译来）到达它时（只有不能再分的一段会）答它；恢复的任务没有段，不会到这里。报告照别的失败：已写的部分留着（`counts`），`stoppedText` 照旧说留下了什么（`web/…/transfer-report.tsx:147-169`）。
- **日志**：任务结束的那一行（`counted`）照旧；停在 `revert_too_large` 时另记那一段的起止步、节点数与是历史还是链接（warn，只有 id 与数目）。

### 3.24 S3：每批在锁下的核对

`Unit.RevertBatch`（`page/app/unit_revert.go:402-485`）今天以这一批的条目再跑一轮 `planned`（`replan`，`:535-551`），要求它的步与这一批的相同（`batchOps`）。多轮之后这不再成立（同一节点的几步，跨节点的依赖由调度定，不能对一部分节点重排），所以改为 **L，再按次序核对前提**（2.1 第 11 条）：

1. **读**：`batchChangeset`、`reads.state` 读这一批的步的节点的条目、版本、此刻的状态与整棵活着的树；删除的步的节点是它那一刻的子树（3.22.1）；恢复的：成员 0（一组的第一个成员）在这一批里时，读这一组全部成员的条目与版本（同今天的 `s.nodes`：成员 0 的步带全组的 id，`page/app/jobs.go:340-349`、`unit_revert.go:418`，审查 N-M1：L 判断这一组的根要整组，`rootHolds`、`groupIsDeleted`、`topOf`，`page/domain/revert.go:320`、`:504`、`:292`）；后面的批里的成员是续接，`continuation` 照旧处理，不读全组：前面的批恢复的成员那时已活着，按全组读，`leftDeleted` 会把它们当作活着的、答 `changed`（审查 N2-N1）。
2. **L**：`domain.CheckRevertPart(state)`：M8 的 `left()` 对这一批的节点（`changed`、`expired`），不排步、不建 S\*。这一批里恢复的页的写回，照它在回收站里的正文核对（同 `leftDeleted` 与 `continuation()` 对回收站里的成员的核对），不当作活着的页：它在这一批开始时还在回收站里，按活着的页核对会看到一个没有条目的版本、答 `changed`（审查 B-I1 的 (b)）。有冲突就答 `RevertConflicts`（任务以 `revert_conflict` 停下，报告列出冲突）。

```go
// CheckRevertPart is a background revert's check of a batch (M9/P3 design
// 3.24): L as PlanRevert has it, of the batch's nodes alone, a page the
// batch restores checked as the trash holds it, the conflicts in their
// order; no S* and no steps: the job's plan has the steps, and each
// step's premise, checked in its order, stands for S* at its moment.
func CheckRevertPart(s RevertState) RevertPlan
```

3. **步取任务开始时的计划，按序号**：`batchOps(steps, plan)` 改为按 `seq` 从任务的紧凑的计划取步（3.22.1，存在 `revertPlan` 里，同它今天存的步，`jobs.go:243-247`），不再与重新计划出的比较；恢复的组、时刻与状态取第 1 步读到的。
4. **前提按次序**：以第 1 步读到的状态建 `revertSim`，按次序对每一步调 `canPlace`、`canDelete`、`canRestore`、`canWrite`（S2 的同一组函数，3.16.2），不成立时那一步的原因就是冲突（`title_taken` 带占着者），整批回滚、任务以 `revert_conflict` 停下；成立就做、更新模拟。**批里的 `canDelete` 比那一步那一刻的子树与这一步记下的节点**（`revertStep.nodes`，3.22.1）：多出的节点里，父节点在记下的节点之中的（新加的每一部分的顶，变更集自己的也是）各是 `children_added`，以它为名，它之下的不列；多个时照 `before()` 的次序（原因、再节点 id，`CompareConflicts`，`revert.go:719-723`）。这同 M8 的 `before()`：它只对父节点要被删除的、变更集没碰过的节点答 `children_added`（`page/domain/revert.go:578-581`），即加进来的部分的顶，所以加进来的一个有 500 页的文件夹在任务开始时是一个冲突，在一批里也是一个（审查 N2-N2）。M8 的 `TestARevertJobsBatchPlansItsOwnNodes` 的后一半断言 `e.Conflicts[0].NodeID == z.ID`（z 没有子节点），照旧通过；以删除的顶为名的第 103 条变异照旧被它抓到。只看"子树里没有 S\* 里活着的节点"不够：一个 C 新建、要在后面的批删除的节点，被别人在两批之间移到这一批要删的节点之下，会被一起删掉，而这一批的 L 没读过它（审查 N-M2）。它们在每一步那一刻核对父节点、键、环、层级与加进来的子节点，就是 S\* 在那一刻要的，所以不再对每一批建 S\*（`before()`）：每批的 S\* 以活着的树加这一批的条目建，跨批的一组的后面的成员与"移入新建的文件夹"的那些节点在它里面都放错了地方，答出 `parent_gone`、`title_taken`（审查 B-I1 的 (a)、(c)，与审查 B 的 over-design 一条）。
5. **写回的基**：写回写在第 2 步刚对着 C 的哈希核对过的版本号上（活着的页是 L 核对过的那个版本号；这一批恢复的页是回收站里那一项核对过的版本号），不是任务开始时的 `Current`：两批之间别人写过的页在 L 里是 `changed`，不会被悄悄覆盖；别人把它改成别的字节、又改回 C 写的字节（两次写：版本号新了、哈希一样），L 放行，写回写在新的版本号上，`WriteContent` 不答 `ErrRevisionMismatch`。只存一次相同的字节不会有新的版本号：`writeContent` 遇到相同的字节什么都不写（`page/app/unit_content.go:61-64`）。
6. **一组在一批里一次恢复**（审查 B-I2）：段把一组的成员截到段要的，写回又紧跟它的成员的恢复，所以一批里一组的成员的步之间可以夹着别的步；今天的恢复的循环只恢复一组的一段连着的步、碰到别的步就停（`unit_revert.go:463-471`），之后的那一段从成员 0 之后起、走续接的分支，只恢复 `cont.members`，而 `conts` 里没有这一批新开的组（`:413-416`、`:775-784`）：成员悄悄留在回收站里，任务照样 `succeeded`（审查 B 的 `restore_loop.py`：`[restore F, write F, restore c1, c2, c3]` 只恢复了 F）。改为：在这一组在这一批里的第一步，以一次 `bringBack` 恢复这一批里这一组的全部成员，然后按次序做其间的步（写回只要页活着；3.22.2 截短恢复的论证反过来也成立：成员的键在 S\* 里空着，`schedule()` 以整个组核对过层级）；这一组在这一批里的最后一个成员之后标一次边界（`markFrontier`，`:795`）。

所以两批之间别人改了的页、改了名的节点照冲突停下，不会被悄悄覆盖或改回（4.12，设计审查 B-I6）；临时名称在两批之间不会被别人看见（段整个在一批里），别人在两批之间占了一个计划里的临时名称，那一步的前提不成立、照 `title_taken` 停下。

**模型的复核**（`scratchpad/m9/p3/sim2/batches_rev.py`，`run_all.sh` 与 `run_r2.sh`；表与各次的数在 `sim2/results.txt` 与审查记录）：五种生成器、R1 与 R2–R4，在段的边界上随机切批、批之间什么都不改时，这些规则 0 次假冲突（今天的读法在 R1 是 9–40%）；批之间注入的八种真的冲突（含 `movegone`）全被抓到。`canDelete` 只看"S\* 里活着的"时，`movegone` 的节点在 L 读它之前被一起删除；以多出的每个节点为名时，`movegone` 以 `children_added` 停下的约两成另列出 B 之下变更集自己的节点（2 到 10 个），以顶为名时都恰是 B 一个（`run_r3.sh`）；第 1 步只读这一批的成员时，跨批的组答假的 `changed`（R1 11–25%）；恢复的循环只恢复连着的一段时漏成员。模型没有的：每批 100 个节点、`locked()`、恢复的时刻取每批的读、名称的大小写折叠。

### 3.25 S3：契约与网页的一条文字

- **契约**：`api/modules/transfer.yaml:839-892` 的 `TransferFailure` 加 `revert_too_large`，它的说明照别的失败写："A revert one of whose segments, steps that must be in one batch, holds more history or links than one batch writes: what it wrote before is kept."。重新生成 Go（`transfer/adapter/http/server.gen.go:41-105`）与 TS（`web/…/schema.gen.ts:2949-2952`），`gen-check` 照常核对。
- **网页**：`transfer-report.tsx:24-43` 的 `failures` 加 `revert_too_large: "transfer.failure.revert_too_large"`（`Record<TransferFailure, PlainKey>` 少一个就编译失败）。文字（`web/…/i18n/messages/en.ts:693-695` 与 `zh-CN.ts:671-673` 之后）：
  - en："Part of the undo had to be done at once and is more than one batch can write: the history or links it brings back. What was undone before it stays."
  - zh-CN："撤销中必须一次做完的一段太大，一批写不下（它带回的历史或链接）。在它之前撤销的部分留着。"
- **测试**：`transfer-report.test.ts:11-49` 加这一种的文字（中英）；`jobs-server.ts` 的 mock 能答它；`messages.test.ts` 照旧核对两种语言的键相同。

### 3.26 S3 的测试

- **page/app**（不要数据库）：`TestTheJobsStepsReplayToStar`（性质，3.16.4 的生成器，加上一组跨批的生成器：一组的成员多于一批、写回在组里、截过的恢复后面还有成员）：随机的 batch 的变更集，`revertSteps` 的步（写回挪到它的恢复之后、段与截过的恢复）按次序重放，每一步以 S2 的前提函数核对、最后等于 S\*；再把步按随机的大小切批（只在段的边界上），每一批**照 `RevertBatch` 读**（第 1 步的节点与条目、成员 0（一组的第一个成员）在这一批里时读全组、跨批的组的续接、L 对这一批恢复的页照回收站里的正文核对），再照第 4 步按次序核对前提、照第 6 步一组一次恢复：从不报冲突，每批结束时没有临时名称留在树上，最后每一组的每个成员都恢复了（审查 B-I1 的第 3 条、B-I2）。2,000 次（`-short` 下 200 次）。批之间另注入审查 B 的 `movegone`（C 新建、都要删的两个顶 A 与 B，A 的删除在前一批、B 的在后面的批，两批之间别人把 B 移到 A 之下）：照冲突停下，停下时 B 还活着，没有冲突以 A 为名；B 的删除在后面的批时 A 的删除以 `children_added` 停下，冲突恰是 B 一个（B 之下变更集自己的节点不列，3.24 第 4 步）；同一批时 L 以 B 的 `changed` 停下（第 102 条变异）。模型的数在 3.24 与 `sim2/results.txt`。另有表格测试：三种段的 `Joined`、恢复截到要的成员、写回紧跟它的恢复、`seq` 唯一、删除的步的节点是那一刻的子树、紧凑的步不带 `Group` 与 `States`。
- **page/app** 的 `unit_revert` 的表格测试（假的端口）：一批里一组的成员被写回隔开（`[restore F, write F, restore c1, c2, c3]`）与截过的恢复之后还有同一组的成员（审查的反例，G 下另有子页：`[aside y, delete new G, restore G (截到 G), place x, place y, restore g1, g2]`）：每个成员都恢复、边界标一次；把恢复的循环改回只恢复连着的一段（第 98 条变异），这两行失败。
- **transfer/app**：`background_test.go` 加：一批按段长（段跨过 `Batch.Nodes` 时整段在里面）；减半到 `max(第一段的末尾, 一半之内的最后一个边界)`；`[三步的段, 重的一步]`：合起来太大、各自都放得下，任务做完（审查 B-I3：退到不大于一半的边界、是 `from` 就停的规则在这里停下）；`to` 已是第一段的末尾、多于一个节点而仍太大时以 `revert_too_large` 停下、报告带已写的数；同一个节点的两步不减半也不停（`RunBatch` 不答）；恢复的任务照旧。
- **组合根**（`bootstrap/revert_job_test.go`；都是 `TestTheB…`，第二份：第三份在 S1、S2 之后已多了一分多钟，6.5；批的大小以 `newAcmeTeamWired(t, …, transfer.Batch{Nodes: n})` 注入）：
  - `TestTheBackgroundRevertOfANameCycleKeepsItsSegments`：批的大小 2；batch 的变更集先有一个不相干的改名、再有名称环（三页轮换名称）与"改名之后新建同名"，所以没有 `Joined` 时会有一刀落在环的段里。后台撤销，在**每一个批的边界**读树：测试以 `holdNode`（`background_test.go:215`）持着下一批的一个节点，下一批在它上面等，测试读树（没有名称带 ` ~` 的节点）、再放开（`behindFirstBatch` 排的是一个写，不是读，`:196-211`；审查 C-M8、B-N5）。任务成功、树与正文等于 batch 之前的；撤销的变更集一个。
  - `TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches`：同样的变更集，再加一项 `edit E.md`。网页的写落在哪两批之间由 `holdNode` 定（NC-N5）：测试持着某一批要写的一个节点，那一批在它上面等、持着笔记本行；网页的写在 `notebooks` 上等（`pgtest.WaitForLockWaitsOn` 看到它）；测试放开，那一批提交、写落下，下一批在它之后（PostgreSQL 按到达的次序交出那一行，`background_test.go:196-228`）。
    - 第一个子测试（批的大小 2：`[放回 O, 删除新的 Plan]`、`[写回 E, 放回 Old]`、再名称环的段）：持着 O，第 1 批等着时网页改了 E 的正文、又把 Old 改了名；放开。任务在第 2 批以 `revert_conflict` 停下，报告列出两个 `changed`：E 与 Old（L 只报它所在的那一批的，所以改名的是与 E 同一批放回的 Old，审查 N-N1）；E 的正文是网页写的（没有被写回盖掉），Old 是网页的名称（没有被改回）；第一批做的留着；没有临时名称（审查 B-M3、C-M8）。
    - 第二个子测试（第 94 条变异；批的大小 1：第 1 批放回 O、第 2 批删除新的 Plan、第 3 批写回 E）：持着 O 与新的 Plan。第 1 批等着时网页把 E 改成别的字节，放开 O；看到第 2 批在新的 Plan 上等之后，网页再把 E 改回 C 写的字节，放开新的 Plan：两次写各落在一批之后，第二次在第一次提交之后。E 的版本号比任务开始时多 2、哈希等于 C 的：第 3 批的 L 放行，写回写在刚核对过的版本号上，任务做完，E 是 batch 之前的正文。写回以任务开始时的 `Current` 为基时，`WriteContent` 看到别的字节与过时的基，答 `ErrRevisionMismatch`，任务以 `internal` 失败（审查 N-M3 = NC-M1：只存一次相同的字节不会有新的版本号，`unit_content.go:61-64`）。
  - `TestTheBackgroundRevertOfASegmentPastABatchStops`：G 下有 101 页（各 210 条链接，同 `TestABackgroundBatchPastTheLinksIsHalved`）与最后一个子页 K，K 下有 x、y；batch：先改名另一页 O（不相干的一步，第一批就是它）、移出 x、移出 y、删除 G、新建同名的 G、把 y 移进新的 G。撤销的段从临时移动 y 到放回 y，K 在 G 的先序的最后，所以段里有 G 这一组的 103 个成员（G、101 页、K；同恢复的成员的数法，G 算一个）、21,210 条链接：任务以 `revert_too_large` 停下，报告说它；O 的改名已撤销、留着（O 的改名是更早的一批，"之前的批做的留着"才有东西可核对，审查 C-M8）。网页的报告里那条文字由 vitest 核对。
  - `TestTheBackgroundRevertOfALargeFolderHoldsItsGroup`（审查 B-I1 的第 4 条与 B-I2 的测试；批的大小 10，各子测试的文件夹 F 有 15 页：多于一批，生产的每批 100 个节点只是更大的同一种）：(1) 移出 F 里先序靠后的 `q/x.md`、再删除 F（4.11 的"移出文件夹再删掉它"：q 的续接与 x 的放回落在同一批，每批建 S\* 时答 `parent_gone`）；(2) `edit` F 里先序靠后的 z、再删除 F（z 的恢复与写回在第 2 批，照活着的页核对写回时答 `changed`）；(3) 写一个文件夹 D、把 12 页移进去，撤销的放回与删除 D 分在两批（删除照计划时的子树时答 `changed` 或 `title_taken`）；(4) `edit F`、删除 F（F 的写回把一组的成员隔开，只恢复连着的一段时只恢复了 F）。每个都：任务成功、树、正文与回收站里的正文等于 batch 之前的、没有成员留在回收站里；(3) 里有人正编辑一页被移动的页时，任务不在开始时答 `page.locked`（删除的步只算那一刻的子树）。
  - 恢复到另一组之下：domain 的那一条（`TestARestoreWaitsForTheGroupItGoesUnder`）；3.21.2 以 SQL 种下的那一行在 S2 改为断言 409 的拒绝（M8 的 L 把上下相套的回收站项读成一组，S2 的主持者的决定 1）。
  - HI5（e2e）照旧通过：平铺的 1,501 个笔记的导入的撤销没有段，切批同今天。
- **交错**：`TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches` 就是 4.15 的"后台撤销一个带名称环的 batch 变更集与批之间的写"（结束时 `checkPages`、`checkLinks`、`checkSearch`）。
- **`batchOps` 按 `seq`**：按种类与节点取步的变异不列：临时的步是它自己的种类（`RevertAside`），(临时, x) 与 (放回, x) 不相撞；要相撞得有一个节点既被第 1 条临时改名、又被临时移动，而第 1 条之后它的父节点在 S\* 里活着、第 2 条只取父节点要被删除的节点，第 2、3 条又同时用掉两个标记，第 3 条没有一个可达的状态用到过：没有一个变更集给得出（审查 B、C 的模型都数过，0 个节点有两种临时的步）。按 `seq` 取步的次序由 `TestTheJobsStepsReplayToStar` 守。

### 3.27 每个新检查的反向对照

每个新的检查都要在去掉它（或把它改错）时有测试失败（v0.1 总体设计 12 的"反向对照"，00 号文档第 9 节）。变异的完整清单在 6.3。

| 检查 | 改错的样子 | 失败的测试 |
|---|---|---|
| 每条规则报它收的字段（3.2） | `MoveKinds` 写死 `"to"`；在 `page/domain` 的任一个非测试文件里加一个不在表里的拒绝 | `TestEveryRuleNamesTheFieldsItIsGiven` 的两个断言 |
| `CheckBatch` 的次序与界线（3.3） | 先查各项再查合计；100 项也拒；`op` 不认识不拒 | `TestCheckBatch`（合计带着坏的项的三行） |
| 计划的两遍、第一个出错的项（3.4） | 第二遍跑全部的项；第一遍出错时不让第二遍更早的取代 | `TestPlanBatch` 的"第一遍在第 5 项、第二遍在第 3 项"；`TestBatchPlanContents`（`Failed` 及其后的项没有改写、没有解析） |
| 计划给新建的页的 id（3.4.2） | 单元自己做 id | `apply_batch_test.go`（参与者收到假的库没建过的 id）；`TestTheLinksOfABatchAreThoseOfSeparateCalls` 的第 10 种 |
| 条目在第一遍里答（3.4.3） | 计划之后才比 | `TestPlanBatch` 的审查 A-M11 的两种 |
| 先以大小拒绝（3.4.4） | 先读正文、再比大小 | `apply_batch_test.go`（过界时 `PageContent` 被调了） |
| 只读头的页（3.4.4） | 读 `write` 的页的正文；带过期的令牌、字节相同的 `write` 答 `page.revision_mismatch`；`PageStates` 不填哈希 | `apply_batch_test.go` 的 100 项 `write`（`PageContent` 被调了）与三页不带 `base_version` 的（答第三项的 422）；`TestBatchPlanContents` 的审查 A2-M4 的两行；`TestPageStates`；`TestTheMergesOfABatchHoldOnTheDatabase` 的"过期的令牌、相同的字节" |
| 计划的会话与一个合计的等待（3.4.5） | 取预算之前不放还持着的事实的份额；推后的改写之前不放还；`DecidedAll` 另等一个 `parse_max_wait` | `content_test.go`；`apply_batch_test.go` 的 5 MiB 的预算的两行（503） |
| 计划的解析的界线与重用（3.4.4） | 不限；不按（页、哈希）留事实 | `TestBatchPlanContents`；`TestTheMCPBatchIsRefusedWhole` 的 50 对 |
| `Lead` 带 `Ambiguous`（3.4.4） | 只留 id | `TestBatchPlanContents` 的歧义的链接 |
| 锁的预检（3.4.5） | 没有 | `apply_batch_test.go`（被编辑的页的正文照样被解析） |
| 规范化（3.4.3） | 不并；并进不在子树里的 | `TestPlanBatch`；`TestTheMergesOfABatchHoldOnTheDatabase` 的"删子孙之后删祖先"（不并时 `DeleteNodes` 的 `stray` 答 500） |
| B1、B3（3.4.3） | 去掉一个 | `TestPlanBatch`；`TestTheMCPBatchIsRefusedWhole` 的两种（不拒时条目的检查答 500，或撤销永远 `changed`） |
| 给新建的页带 `base_version`（3.4.2） | 不拒；排在 409 之后 | `TestPlanBatch`；`TestTheMCPBatchIsRefusedWhole` |
| 计划里的改写（3.4.4） | 不改写就匹配 | `TestBatchPlanContents`；`TestTheLinksOfABatchAreThoseOfSeparateCalls` 的第 2 种 |
| 计划的解析先数进页数（3.4.4） | 解析之后才数；不带 `op` | `TestBatchPlanContents`；`TestTheMCPBatchIsRefusedWhole` 的 60 项 `edit` |
| 正文合成（3.5.2） | 每个正文操作各写一次 | `apply_batch_test.go` 的守卫只看到一次写；`TestTheMergesOfABatchHoldOnTheDatabase` 的版本号 |
| 回到原处的条目（3.5.4） | 不删；连写了正文的节点的改动也去掉 | `TestTheMergesOfABatchHoldOnTheDatabase` 的"改名再改回"三行（第三行：`checkLinks`） |
| 单元里的核对与重做一次（3.6.2） | 不比节点 id；不比版本号；重做多次；守卫的拒绝也重做 | `apply_batch_test.go`；`TestTheBatchIsPlannedAgainAfterAWrite` 的两个子测试 |
| 答复（3.6.3） | 之后被删除的页的正文操作答不出版本与路径 | `apply_batch_test.go` |
| 推后的改写（3.7） | 参与者每个操作调一次；改动带版本号；新建之后移动的不交 | `apply_batch_test.go`；`TestTheLinksOfABatchAreThoseOfSeparateCalls` 的第 1、3、17 种 |
| (a)–(e)（3.7.2） | 各去掉一处；写了的页照合并的改动；写了的页的链接全都比；`spread` 不展开它自己的改动移动的节点；再解析写了的页、不用 `Written` 的事实 | `rewrite_test.go` 的各行；第 1（(a)(b)(c)）、3（(d)）、11–14、18、19（(e)）种 |
| 删除了的页的行与别名、"之前"已不在（3.7.2） | 不去掉删除了的来源页的行；不当作没有它们的别名；对已不在的目标记 error | 第 6、7、8 种；`rewrite_test.go` 的 warn 一行 |
| `Recased` 是集合（3.7.2） | 只留最后一个 | `TestRelocationRecasesEveryPage`；第 16 种 |
| 数改写的页、错误的次序（3.7.2） | 在 `refuseLocked` 或解析之后数 | `rewrite_test.go` |
| `PathBefore` 从根（3.7.3） | 改回碰到第一个改动就返回 | `TestPathBeforeUndoesEveryChangeFromTheRoot`；第 4 种 |
| 原来的父节点在单元开始时读（3.7.3） | 读现在的树 | 第 5 种（Markdown 链接的文字） |
| `MaxBatchRewrites`（3.8.2） | 不数；差一 | `TestABatchAtItsRewriteBoundTakesItsTime`（恰在界线处成功、多一页被拒） |
| 条目与解析的界线（3.8） | 不数条目；按页数而不按（页、哈希）；一页的例外按份数 | `apply_batch_test.go` 的界线；`TestTheMCPBatchIsRefusedWhole` 的 10,001 个节点 |
| 答复的 `Fit`（3.9.1） | 不截；截掉变更集 | 适配器的 `batch_test.go` |
| `At` 与 `op`（3.9.2） | 不写进 `Fit` 的文字；`Extra` 没有 `op` | `TestAnErrorsPlaceFollowsItsCode`；适配器的测试；`TestTheMCPBatchIsRefusedWhole` 的 `op` |
| 码的表、`declaredCodes`（3.13.1） | 少一个码；`page.batch_too_large` 不在声明里 | `TestTheMCPToolsErrorsCarryTheirCodes`；`TestEveryMCPToolHasItsCodes` |
| 矩阵的一行（3.13.2） | 没有行；`after` 不读回 | `TestThePermissionMatrixCoversEveryTool`；`after` 的计数 |
| 工具的字符串（3.13.3） | 没有 `batch` 的合法参数 | `TestToolStringsDoNotAnswerInternalErrors` |
| 清单与说明（3.12） | `tools.json` 与代码不同；说明少一行或过 4 KiB | `TestTheToolListsAreTheContract`；`TestTheBuiltInInstructionsNameEveryTool`、`TestTheBuiltInInstructionsAreShort` |
| `page.batch` 的规则（3.13.4） | 规则表缺它 | `bootstrap/actions_test.go`；矩阵的读者列 |
| 4.11 的性质（3.14.6） | 不比 batch 写了的页；计数在每个种子开头就有；数试过的而不是做成了的 | `TestTheIndexIsItsRebuild` 的 batch 一步与它的计数 |
| 读整页的 `write`（3.4.4，S1 审查 A-I1） | 之前的改名、移动碰到它的 `write` 只读头、照改写之前的正文比 | `TestBatchPlanContents`、`TestBatchPlanChecksItsReads`、`TestABatchsWriteSeesItsPageAsTheMovesBeforeItLeaveIt`；`TestTheWritesOfABatchAreJudgedAsSeparateCallsJudgeThem` |
| 移动带着走的节点（3.4.3，S1 审查 A-I2） | 不数；在那次移动自己的检查之前数 | `TestPlanBatchBoundsWhatItsMovesCarry`；`TestTheMCPBatchIsRefusedWhole` 的两种 |
| 锁的预检的删除与次序（3.4.5，S1 审查 A-M1） | 不看删除的子树；不按项的次序 | `TestABatchsLockPrecheckIsInItsOpsOrder`；`TestTheMCPBatchIsRefusedWhole` 的"删除被编辑的页，再编辑另一页" |
| 页被删除也重新计划（3.6.2，S1 审查 A-M2） | 答 `page.not_found` | `TestABatchIsPlannedAgainOnce` |
| 冻住的节点（3.7.2、3.7.3，S1 审查 B-M1、B1-N1） | 不交给参与者；它们的改名、移动不交给"之前"的路径 | `TestABatchLeavesTheLinksToTheNodesItFroze`；`TestTheWaysABatchDiffersFromSeparateCalls` 的 d1–d6w；`TestTheLinksOfABatchAreThoseOfSeparateCalls` 的第 23 种 |
| 新建的节点不进触及（3.7.2，S1 审查 B1-M1、复核 A 的 F2-1） | 不排除；每个新建的都排除；新建之后读与触及重来 | `TestABatchLeavesOutTheNodesItCreatedAfterAMove`、`TestABatchsLinksLeaveOutTheNodesItCreatedAfterAMove`、`TestBatchPlanMovesBetween`、`TestABatchsPlanReadsATreeAcrossItsCreations`；`TestTheLinksOfABatchAreThoseOfSeparateCalls` 的第 20–22、24 种 |
| 改写的三个界线与它们的次序（3.8.2，S1 审查 B-M3、B1-M2、B1-M3，复核 B 的 B2-M1、A 的 F2-2、F2-3） | 写了的页不算 2 或都数；不数链接或字节；在读树之后数；次序反了 | `TestABatchCountsAPageItWroteAsItsWeight`、`TestABatchCountsTheLinksItWouldWrite`、`TestABatchCountsTheBytesItWouldWrite`、`TestABatchCountsItsPagesThenTheirLinksThenTheirBytes`、`TestABatchCountsItsRewritesFirst`；守卫那一行的五个测试 |
| 改写过的页留下的解析额度（3.7.2，复核 B 的 B3-M1、B4-N1） | 不查；先等再判；份额不记；写过的页给空的份额；单个 `move` 也查 | `TestABatchRefusesAParseItsOwnSharesLeaveNoRoomFor`；`TestABatchWhosePagesOutgrowTheParseBudgetIsRefused` |
| 先改写写了的页（3.7.2 (a)，实施的偏差 27） | 两段对调 | `TestABatchRewritesThePagesItWroteFirst` |
| 一棵树的读（3.7.2、3.7.4，修正的第 3 轮，复核 A 的 F3-1） | 所有的树共用一份；每棵树的读都留着；删除之后不重来 | `TestABatchsPlanHoldsOneTreesReads`、`TestABatchsRewritesHoldOneReach`、`TestABatchsPlanReadsATreeAgainAfterADelete` |
| 批量入队与 `building`（3.9.1、3.10.1，S1 审查 C-I2） | 准确入队；答复不带 `building` | `TestABatchQueuesForTheSearchInBulk`；适配器的 `TestTheBatchAnswersEachOp`；`TestTheMCPBatchReachesItsUseCase` |
| `schedule()` 的前提（3.16.2） | 恢复不核对层级；删除不等写回；放回不核对环；恢复的层级数附件（S2 审查 A-M2） | `TestScheduleEndsAndPutsTheNotebookBack`（模拟的变异：49、7,494 次失败；放回不核对环只有它抓到，`TestScheduleRevertsEachBatchShape` 与 `TestPlanRevertOfASwapGoesAside` 单独都抓不到）；`TestScheduleRevertsEachBatchShape`；数附件：它的"MaxDepth 上带附件的页被删除"一行与修正的第 1 轮起的性质测试 |
| 三条后备（3.16.3） | 去掉第 1 条；去掉第 2 条；第 3 条取根下的节点；第 2 条不按 id 排（S2 审查 C-N1）；第 2 条取不待放回的节点（S2 审查 A-N2） | `TestScheduleRevertsEachBatchShape` 的互换名称（在根下）与审查的反例；`TestTheSecondFallbackPicksTheNodeItFrees`（修正的第 1 轮起也有性质测试的全量一次，按移动的样子，S2 审查 C-N2）；`TestTheThirdFallbackMovesAPlaceOffTheRoot`；`TestTheSecondFallbackTakesTheLeastFirst`；`TestTheSecondFallbackMovesOnlyANodeToPlace` |
| 轮数的上限（3.16.4 (T2)） | 去掉它：正确的代码到不了它，没有单独的变异；"去掉第 2 条"与"第 3 条不论"合在一起时，有它，性质测试以 `ErrRevertUnordered` 失败，没有它就挂到 go test 的超时 | `TestScheduleEndsAndPutsTheNotebookBack`（与那两个变异合在一起；模拟：随机的 2 万次里短的 1 次、长的 14 次到上限；S2 建成：0.18 秒失败，没有上限时挂到 `-timeout 90s`）；上限太小（没有 `2N`、以 `len(p.c.ids)/4` 或 `/3` 代它、开始时待做的步不记）：`TestAScheduleTakesMoreRoundsThanItsSteps`（S2 审查 A-M1、修复核对 A 的 N1、N4；以 N 代 2N、不加 1 不测，主持者的决定 7） |
| (T1)–(T4)（3.16.4） | 一个节点临时改名两次；卡住时不答 `ErrRevertUnordered` | 临时改名两次：`TestAFallbackRenamesANodeAsideOnce`（构造的；性质测试抓不到，合法的状态里到不了，S2 的实施的偏离 8、审查 C-N6，主持者的决定 2）；卡住时不答：`TestScheduleAnswersUnorderedForADefect`；其余由 `TestScheduleEndsAndPutsTheNotebookBack` 以测试自己的模型判定 |
| 临时名称（3.17） | 取前 8 位；不截短；附件的扩展名丢掉；不避开 S\* 的键；空的主干留着（S2 审查 A-N1）；名称取此刻的（S2 审查 C-N5） | `TestAsideName`；`TestTheRevertOfEachBatchShape` 的长名称与附件（不截短）；扩展名丢掉：`TestAsideName` 与修正的第 1 轮起的 domain 的性质测试，组合根的附件那一行抓不到（丢了扩展名仍是合法的标题，只在单元之中，S2 的实施的偏离 11、审查 C-N6）；不避开 S\* 的键：包里的 `TestAStepAsideTakesNoKeyOfS`（偏离 7）；空的主干：`TestNumbered`、`TestAsideName`；名称取此刻的：`TestTheThirdFallbackMovesAPlaceOffTheRoot` |
| `RevertWrites`、先恢复再写回（3.18） | 照 M8 只取活着的页；写回以 `state.Contents` 为基；解析按页 id | `TestRevertWritesAreEveryVersionOnABase`；`TestTheRevertOfEachBatchShape` 的"先写正文再删除"（错的正文、500）；性质测试（模拟的变异：7,555 次失败）；解析按页 id：`TestARevertRestoresAPageThenWritesItBack`（读守卫看到的每一步的事实）与 M8 的两个用例的测试（`TestRevertChangesetAnswersItsCodesInOrder` 的 "a page in the trash at other bytes meanwhile"、`TestRevertChangesetsLockedConflictsComeByNodeID`），组合根的那一行看不出（恢复的事实只到守卫，S2 的实施的偏离 10、审查 B-N2、C-N6） |
| 撤销的撤销（3.18.5） | 先删除、再写回 | `TestTheRevertOfEachBatchShape` 的"撤销的撤销"（回收站里的正文不对）；撤销的性质测试（比回收站里的正文） |
| 一页的例外按不同的页（3.18.4） | 按份数 | `TestARevertRestoresAPageThenWritesItBack`：一页恢复与写回两份近 5 MiB 的正文，合起来过了 `MaxUnitBytes`，不被拒 |
| 一页的两份正文在最小的预算下（3.18.4，S2 审查 B-M1、修复核对 B 的 N1、N2） | 不还份额；总是还；早还；还掉之后不清零；适配器的 `Held` 答 0、`Takes` 答原数 | `TestDecidedAllGivesItsSharesBackBeforeATakeThatCannotFit`（恰好等于预算的大小、多一个字节、还掉之后的第三份）；`TestTheAdapterKeepsTheFactsShare`；`TestTheRevertOfALonePagesTwoContentsAtTheLeastBudget`（不还：503） |
| 步数含临时的步（3.19） | 不数 | `TestStepsCountTheStepsAside` |
| `renames`、`moves`（3.20） | 补值只看前 1,000 项；单元不设；读还走旧的语句；换次序设 `moves`、任何有父节点的节点设 `moves`（S2 审查 B-M2、C-M2）；补值去掉 NULL 守卫（审查 B-M3）；标记挪到搜索的入队之后（审查 B-N1 = C-N4） | `TestTheChangesetsMovesAreBackfilled`；`TestTheChangesetsRenamesCountEveryItem`；`TestAUnitMarksItsChangesetsRenamesAndMoves`；读：适配器的 `TestChangesetMoves` |
| 临时的步的计数（3.21.3） | "changeset reverted"那一行不带 `aside` | 撤销的性质测试的不空转（没有一个种子用到临时的步） |
| S1、S2 的临时的拒绝（第 0 节） | 去掉 | S1：`revert_test.go` 的"先写正文再删除"一行（S1 答 `ErrRevertUnordered`）；S2：`TestARevertJobRefusesTheStepsItsBatchesWouldSplit`（任务的计划拒绝带临时的步、恢复之后的写回或"写回再删除"的；拒绝不包文字也失败，S2 审查 A-N4） |
| 页面事件只列单元之后活着的写过的页（3.10.2，S2 审查 B-N3） | 照样列 | `TestThePagesEventListsNoPageInTheTrash` |
| 删除照那一刻的子树（3.22.1） | 照计划时的树 | `revertSteps` 的表；`TestTheBackgroundRevertOfALargeFolderHoldsItsGroup` 的 (3)（假冲突、开始时的 `page.locked`） |
| 紧凑的计划（3.22.1） | 恢复的时刻与组取开始时的计划 | `TestARevertJobOfAGroupDeletedAgainSinceThePlan`（`page/app/jobs_test.go:714`，M8/P4A 审查 A-M1 的那一个） |
| 段（3.22） | 不标 `Joined`；恢复不截；写回不紧跟恢复 | `TestTheJobsStepsReplayToStar`；`TestTheBackgroundRevertOfANameCycleKeepsItsSegments`（批的边界上看得见临时名称） |
| 减半与只在段的边界切批（3.23） | 退到不大于一半的边界、是 `from` 就停；一段太大时死循环或答 `internal` | transfer 的测试（`[三步的段, 重的一步]`）；`TestTheBackgroundRevertOfASegmentPastABatchStops` |
| 一个节点不受界线（3.23） | 改回"多于一步" | transfer 的测试（同一节点的两步）；`revert_changeset` 的一页两份正文 |
| 每批的 L 与前提（3.24） | 不做 L；每批仍建 S\*；这一批恢复的页的写回照活着的页核对；写回以任务开始时的 `Current` 为基；成员 0（一组的第一个成员）在这一批里时只读这一批的成员；`canDelete` 只看 S\* 里活着的、或以删除的顶为名 | `TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches`（两个子测试）；`TestTheJobsStepsReplayToStar`（含 `movegone`）；`TestTheBackgroundRevertOfALargeFolderHoldsItsGroup` 的 (1)、(2)；`TestARevertJobOfAGroupDeletedAgainSinceThePlan`；`TestARevertJobsBatchPlansItsOwnNodes` |
| 一组在一批里一次恢复（3.24） | 只恢复连着的一段 | `unit_revert` 的表的两行；`TestTheBackgroundRevertOfALargeFolderHoldsItsGroup` 的 (4) |
| `revert_too_large` 的文字（3.25） | 缺一种语言 | `messages.test.ts`；`transfer-report.test.ts` |

### 3.28 S1 的实施对设计的修正

S1 的审查与修复核对见 [S1 的审查记录](reviews/P3-S1-batch-review.md)，结果在 9.1。与上文最初的设计不同之处已改在各节里：

- **S1，实施时定下的**（实施者的 27 条偏差，S1 审查 C 判过；修正的第 1 轮去掉第 9、12 条，改了第 18 条）：`OpKind` 的值叫 `OpKindWrite` 等（1，3.3.2）；`PlannedStep.To` 是 `Destination`（2，3.4.2）；`LinkRewriter` 的 `Indexed` 与 `Rewrite` 的 `since`（3，3.4.4）；计划分在四个文件（4，3.1）；P2 的 `TestRelocateRefuses` 照旧留着（5，3.14.1）；`DropUnchanged` 留下被一次移动带着走的节点的改动（6，3.5.4）；`ApplyBatch` 另收锁的持有者、时钟与 `maxRewrites`（7，3.4.5、3.11）；`OpDone.Created` 是 `[]Placed`（8，3.6）；第二次的单元里正文操作的路径上没有页答 `BatchChanged`（10，3.6.2、3.9.2）；`LinksOn` 没有 `Gone`（11，3.7.4）；`DecidedAll` 的界线是 `func(links int) error`（13，3.8）；"batch applied" 每次成功都记（14，3.11）；`BatchTooLarge` 的文字（15，3.9.2）；第 0 步在造出来的语料上量（16，3.10.1）；基准经 MCP 的工具调用例（17，3.8.2）；守卫与基准数进 P1（18，3.8.2）；4.11 的性质也允许经被 batch 的写拿掉的别名的链接重新解析（19，3.14.6）；计划的解析的时限在竞态检测下乘 6，`TestTheMCPBatchIsRefusedWhole` 用生产的期限（20，3.14.5）；MC5 (MCP) 末尾用 `expectNoAsideNames`（21，4.1、4.2）；`fixtures/mcp.ts` 没改（22，3.1、4.1）；矩阵的 batch 一行以 `client: "mcp"` 读 `log`（23，3.13.2）；第 15、16 条变异的写法（24，6.3）；`TestBatchPlanContents` 加一页过 10 MiB 一个字节（25，3.14.1）；第 14 种之后再改名（26，3.14.3、6.3 第 49 条）；推后的改写先改写 batch 写了的页（27，3.7.2 (a)）。
- **S1，审查与修复核对带来的**（编号是 S1 的审查记录的；主持者的决定见 2.2 与 00 号文档第 11 节第 44–55 条）：之前的改名、移动碰到它的页的 `write` 读整页、照改写之后的正文判（A-I1 = B-M2、A 的 N3、N5，3.4.4、3.8）；移动带着走的节点的界线（A-I2、A 的 N2、N4，3.4.3、3.8）；锁的预检取删除的子树、按项的次序（A-M1，3.4.5、3.9.3、3.12.1）；两次读之间页被删除也重新计划一次（A-M2，3.6.2）；`edit` 留下相同的字节答未改动（A-M3，3.4.4、3.6.3）；一次改写只建一次改动的表、改写的页数在读树之前数（B-I1，3.7.2、3.7.3）；冻住的节点的链接从不改写，它们的改名与移动交给"之前"的路径（B-M1、B1-N1，3.7.1–3.7.4、3.14.4）；写了的页一共读一次树，在 `MaxBatchRewrites` 里算 2 页，只在它自己的改动碰到它的链接时才数（B-M3、B1-M2、B1-N3，3.7.2、3.8.2）；新建的节点不进之前的改名、移动的触及（B1-M1、复核 A 的 F2-1，3.7.2、3.14.3、3.14.4）；`MaxBatchRewriteLinks`（B1-M3，3.8、3.8.2）；`MaxBatchRewriteBytes` 与三个数的次序（复核 B 的 B2-M1，复核 A 的 F2-2、F2-3，3.8、3.8.2、3.9.3）；改写过的页留下的解析额度的检查（复核 B 的 B3-M1、B4-N1，复核 C 的 N1-f4、N2-f4，3.7.2、3.8.2、3.9.2）；计划一次只留一棵树的读，删除之后重来（修正的第 3 轮、A 的 N1、复核 A 的 F3-1，3.7.2、3.7.4）；搜索批量入队、答复带 `building`（C-I2，3.9.1、3.10.1、3.12、6.3 第 45 条）；单元的重做的四种比较、合并的 `created` 与版本、错误的位置的测试（C-I1、C-M1、C-N2、C-N3，3.6.2、3.14.2、3.14.5）；性质的 batch 被拒就失败、种子合起来至少 3 个 batch（C-N1、C-M2，3.14.6）；M8 的清理测试等启动时的清理（修正的第 4 轮，9.1）；守卫那一份的时限 30 分钟（复核 C 的 N3-f4，3.1、6.5）。没有交给 v0.1 之后的打磨的。

### 3.29 S2 的实施对设计的修正

S2 的审查与修复核对见 [S2 的审查记录](reviews/P3-S2-revert-review.md)，结果在 9.2。与上文最初的设计不同之处已改在各节里：

- **S2，实施时定下的**（实施者的 13 条偏离，S2 审查 A、B、C 各在自己的范围里判过）：上下相套的回收站项照 M8 的 L 拒绝（1，3.21.1、3.21.2、3.26）；用例的 (T4) 一行是存着的缺陷"比 `MaxDepth` 深的页被改名"（2，3.15、3.21.1、5.2）；契约里 `getChangeset` 与 `ChangesetDetail` 的说明一并改为全部条目（3，3.20.3）；钉住旧的读法的三个测试（适配器的 `TestChangesetMoves`、`TestTheLogReadsUseTheirIndexes`、权限矩阵种下的变更集，4，3.15）；`DecidedContents` 是 `DecidedAll` 旁的泛型（5，3.18.3）；MC5 (page) 的树照兄弟的次序（6，4.3）；临时名称避开 S\* 的键由包里的 `TestAStepAsideTakesNoKeyOfS` 核对（7，3.21.1、3.27）；(T1) 的"临时改名两次"只有构造的测试抓到（8，3.16.4、3.21.1、3.27）；性质测试的两种定向的 batch 先做、"先写正文再删除"的立即撤销（9，3.21.3、6.3 第 72 条）；第 81 条在组合根的那一行看不出（10，3.18.3、3.27、6.3）；第 79 条同样（11，3.27、6.3）；"移动加改名的中间态"以两项写出（12，3.21.2）；同一毫秒的两个附件的文件经 `storage.OpenLocal` 写（13，3.21.2）。
- **S2，审查与修复核对带来的**（编号是 S2 的审查记录的；主持者的决定见 2.2 与 00 号文档第 11 节第 56–66 条）：一页的两份正文在最小的解析预算下解析，`decidedAll` 先还掉本次调用留着的份额（B-M1、修复核对 B 的 N1、N2，3.15、3.18.4、3.27）；轮数的上限从下面钉住、开始时待做的步只写一处（A-M1、修复核对 A 的 N1、N4，3.16.4、3.21.1、3.27）；domain 的性质测试的模型有附件（A-M2，3.21.1、3.27、6.3 第 79 条）；换次序与文件夹下的改名、换次序的标记（B-M2、C-M2，3.20.2、3.21.4）；迁移测试的 NULL 守卫（B-M3，3.21.4）；组合根的性质测试的下限按种类数（C-M1，3.21.3）；空的主干照页的名称，M7 的 `Numbered` 一并改（A-N1，3.17）；第 2 条后备只取待放回的、先取 id 最小的（A-N2、C-N1，3.16.3、3.21.1）；任务的临时的拒绝说出它是什么（A-N4，第 0 节、3.15、3.27）；标记在搜索的入队之前（B-N1 = C-N4，3.20.2）；恢复的事实只到守卫（B-N2，3.17、3.18.3）；页面事件只列单元之后活着的写过的页（B-N3，3.10.2、3.15、3.27）；守卫只凭名称认得临时的步（B-N4，3.17、第 8 节）；第 3 条后备的检查按移动的样子数（C-N2、修复核对 A 的 N3 = C 的 N1，3.16.4、3.21.1、6.3 第 75 条）；每个子测试只读自己的日志（C-N3，3.21.2）；"名称取计划开始时的"的构造的测试（C-N5，3.17、3.21.1）；3.27 与 6.3 的第 79、81 条与 (T1) 照建成的测试写（C-N6）；两份正文的组合根测试用生产的期限、只留最小预算那一行（修复一轮的 CI 的红、修复核对 C 的 N2，3.21.2）；第一份过线时把 `TestEvery[M-R]` 挪到第三份、第二份的估计（修复核对 C 的 N3，6.5）。没有交给 v0.1 之后的打磨的。

## 4. e2e

MC5（00 号文档第 9 节）是两个测试，在同一个文件里：S1 加 MCP 的那一个，S2 加网页的那一个。照 P2 的故事的写法：库里的断言以 `db` 读，网页的以 `signedInPage` 操作，MCP 经 `mcpClient`（`e2e/fixtures/mcp.ts:114-155`）。

### 4.1 断言与 fixture 的改动

- `fixtures/assert/page.ts` 的 `ChangesetOf`（`:195-200`）加可选的 `message` 与 `kind`，`expectChangesetOf`（`:207-218`）给了就一起比（今天不比说明）。
- `fixtures/assert/page.ts` 加 `expectNoAsideNames(db, notebookId)`：这本笔记本里没有临时名称的节点（活着的与回收站里的），条目的前后名称里也没有，以正则在 SQL 里比：` ~[0-9a-f]{8}([0-9a-f]{24}(-[0-9]+)?)?(\.[^./]+)?$`（8 位或 32 位、32 位之后的 `-2` 一类、附件的扩展名，3.17）。它带一个正向的对照（审查 C-N7）：同一个正则先对几个造出来的临时名称（`Plan ~1a2b3c4d`、`Plan ~<32 位>-2`、`photo ~1a2b3c4d.png`）断言匹配，再查库，免得正则写错而查询什么都抓不到。S1 建成它，MC5 (MCP) 的末尾也用它（S2 的 MC5 (page) 之前 knip 会报没用到的导出，实施的偏差 21）。
- 不改 `resultOf`、`errorOf`（`mcp.ts:165-176`；S1 照此没改 `fixtures/mcp.ts`，实施的偏差 22）：`errorOf` 已答结构的 `error`，故事读它的 `op` 与 `lock`。网页的撤销照 HI4 的 `revertWith`（`fixtures/wiki-pages.ts:381-392`），链接照 `expectIndexedLinks`（`fixtures/assert/links.ts:34-48`）。

### 4.2 MC5 (MCP)（S1）

"MC5 (MCP): an agent ingests two papers in one batch with a message, one changeset, Index's links following the rename; a batch writing a page a teammate is editing is refused whole, naming the op and the holder"：

1. 种下：笔记本 Research，页 `Index`（`[[Draft]] and [[Old]]\n`）、`Draft`、`Old`；队友 Bea 是编辑者。
2. agent 的一个 `batch`（说明 `"Ingest two papers"`）：`write Sources/Paper A.md`（补建 `Sources`）、`write Sources/Paper B.md`、`write Notes/Summary.md`（`[[Paper A]] [[Paper B]]`，补建 `Notes`）、`edit Index.md`（在末尾加 `- [[Summary]]`）、`move Draft.md → Plan.md`、`delete Old.md`。断言：
   - 答复：`changed` 为真、`results_count` 6；第 0 项 `created` 是 `Sources.md` 与 `Sources/Paper A.md`（补建的上级是页，带 `.md`，同 `e2e/stories/mcp/mc3-write-edit.spec.ts:61-66`）；第 3 项的 `version` 等于库里 Index 现在的版本（推后的改写写过之后的）；第 4 项的路径 `Plan.md`；第 5 项 `pages` 1。
   - 一个变更集（`expectChangesetOf`，带 `message` 与 `kind: "edit"`）：客户端 `mcp:<名>`，条目是新建的五个节点、`Draft`、`Old`，版本是新建的五页（补建的两个上级页也各有一行）与 `Index`（Index 的正文写与链接的改写合在一个版本行里）。
   - Index 的正文是 `[[Plan]] and [[Old]]\n- [[Summary]]\n`（链到被改名的页的跟着改，链到被删的页的不动）；`expectIndexedLinks`：`Plan` 与 `Summary` 解析到它们的节点，`Old` 不解析。
   - `log` 的第一行是这个变更集、带说明（经 MCP 的 `log`，P2 的工具）。
3. Bea 开 `Index` 的编辑会话（`openSession`，`e2e/stories/collab/c4-guard.spec.ts:50` 的那个 fixture）。agent 的 `batch`：`write Sources/Paper C.md`、`edit Index.md`。断言：`errorOf` 的 `code` 是 `page.locked`、`op` 是 1、`lock.display_name` 是 Bea 的、`message` 含 `ops[1] (edit)`；库里没有 `Paper C`，变更集的数目不变；末尾 `expectNoAsideNames`（实施的偏差 21）。

### 4.3 MC5 (page)（S2）

"MC5 (page): the web reverts an agent's batch that swapped two pages' names and renamed a page before creating another of its name, the tree, the contents and the links back, no name of the revert left"：

1. 种下：页 `Notes`（"notes\n"）、`Glossary`（"terms\n"）、`Plan`（"plan v1\n"）、`Index`（`[[Notes]] [[Glossary]] [[Plan]]\n`）；记下它们的节点 id 与 Index 的字节。
2. agent 的 `batch`（说明 `"Reorganize"`）：`move Notes.md → Swap.md`、`move Glossary.md → Notes.md`、`move Swap.md → Glossary.md`、`move Plan.md → Plan 2025.md`、`write Plan.md`（"plan v2\n"）。断言 Index 被改写成链到新名称（`[[Glossary]] [[Notes]] [[Plan 2025]]`）。
3. 写者在网页上打开这个变更集，`revertWith` 撤销；点"撤销"之前断言对话框里有改名、移动的提醒（`history.revertLinks` 的文字 "Links written since to the new names or places are not changed back."，`web/apps/web/src/i18n/messages/en.ts:340`；`renames || moves` 时显示，`web/apps/web/src/pages/notebook/revert-dialog.tsx:315`；今天没有一个 e2e 断言它，审查 C-N6）。
4. 断言：三个原来的节点各回到原来的名称（按 id）；新建的 `Plan` 在回收站；每页的正文与 batch 之前的字节相同，Index 的字节等于第 1 步记下的（撤销写回它）；`expectIndexedLinks`：Index 的三条解析到原来的三个节点；撤销的变更集一个、`reverts_id` 是 batch 的；`expectNoAsideNames`；树上的标题照兄弟的次序是 `Notes`、`Glossary`、`Plan`、`Index`（`treeTitles`；也核对撤销放回了次序值，S2 的实施的偏离 6）。

### 4.4 e2e 的时间

两个测试各约 6–8 秒（P2 的 MC3、MC4 约 5–7 秒；撤销多一次网页的往返），e2e 约 +15 秒；CI 的 E2E 一步今天 3:35–4:17，作业 20 分钟，不受影响。S1 实测：MC5 (MCP) 约 0.2 秒（210 毫秒），E2E 一步在 S1 与各轮里 2:50–3:39（9.1）。S2 实测：MC5 (page) 在整个 e2e 里 524 毫秒（单独 587 毫秒），E2E 一步在 S2 与各轮里 2:56–3:43（9.2）。HI5 不改，S3 照旧跑它（`e2e/stories/history/hi5-revert-import.spec.ts`）。

## 5. 实施步骤

**每个提交都是绿的**（P2 的规则，[P2](02-P2-writes-history.md) 第 5 节，`02-P2-writes-history.md:2431`）：每个 Step 合并时是绿的，Step 里的每个提交也是：一个提交加了工具，就在同一个提交里加它在 `tools.json` 的两个条目、内置的说明的一行、码的表的一行、`declaredCodes`、矩阵的一行与工具的合法参数（读 `tools.json` 的三个测试在其间会红：`TestEveryMCPToolHasItsCodes`，`bootstrap/permission_matrix_mcp_test.go:99-108`；`TestThePermissionMatrixCoversEveryTool`，`:328-331`；`TestToolStringsDoNotAnswerInternalErrors`，`bootstrap/mcp_reads_test.go:734`，审查 C-M2）；一个提交改了钉住旧行为的测试，就在同一个提交里改它的期望。下面的次序照此排。

### 5.1 S1（`m9-p3-s1`）

0. **搜索的实测**（在任何 batch 的代码之前：它定单元怎样入队、答复的 `building`、工具的说明与第 45 条变异，3.10.1）：`BenchmarkSearch` 加 `NERVE_SEARCH_BACKLOG`（只改基准的测试代码），在 M8/P5 的语料上量 n = 0、100、1,200、2,200，照 3.10.1 的相对的条件定改不改，写进审查记录；改了，第 8 步的规格就带 `BulkSearch`。S1：M8/P5 的语料已删，在造出来的语料上量（约 30 万个节点、200 本笔记本，实施的偏差 16），照常的积压量得不改；修正的第 1 轮让积压是最新的页（同 batch 写的）重量，n = 0、100、1,600、2,200，照规则改为批量入队（S1 审查 C-I2，3.10.1）。
1. **哨兵改名**（单独的一个提交，在新的哨兵之前）：`ErrBatchTooLarge` → `ErrJobBatchTooLarge`，3.9.2 列的每一处（page 的 domain、根的再导出、`page/app/jobs.go` 的四处、`jobs_test.go`、`bootstrap/background_cost_test.go`、`background_test.go`、`transfer.go` 的映射）；行为不变，后台任务的测试照旧。
2. **字段名**：`domain.Args`、P2 的规则收它、`PagePath`、`ParentChainOf`、`RelocationOf`、`RenameTitle` 挪进 domain，四个用例传零值；`TestEveryRuleNamesTheFieldsItIsGiven`（扫 `page/domain` 的全部非测试文件；规则、不算的与构造函数三张表，3.2）。P2 的全部测试照旧通过（字段名不变）。
3. **API 的错误与动作**：新的 `ErrBatchTooLarge`（`page.batch_too_large`）、`OpFailed`、`BatchTooLarge`、`BatchChanged`；`ActionBatch` 与 `rules.go` 的一行（`bootstrap/actions_test.go` 核对两边相同）。用例要 `ActionBatch`，所以它在用例之前（审查 C-M2）。
4. **`CheckBatch` 与 `PlanBatch`**（domain：两遍、计划给的 id、规范化、B1、B3、第一遍里的条目、`LinkRewriter`、`Lead`、`LinksBefore`、`PathStep`），它们的表格测试。
5. **linking**：`PathBefore` 从根撤回、`Recased` 是集合（它的测试的字面量同改）、`resolutions` 的 `gone`、`Moved` 的字段、参与者对写了的页的 (a)–(e)（链接取自 `Written` 的事实；`spread` 也展开它们自己的改动移动的节点，每个节点的子树与它的别名键一个 batch 读一次）、跳过、在解析之前数改写的页、`BatchLinks` 与 `NewBatchLinks`，它们的测试；M6 的样例与随机测试照旧通过。
6. **单元与解析**：`UnitSpec.DeferParticipants`、`Unit.Participate`、`Unit.DropUnchanged` 与 `Changesets.DropUnchanged`、`PageDraft.ID`、`PathStore.LinePaths`、`Step.Batch`、`recordItems` 的注释；`ContentParser.Planned`（会话：一个合计的等待、它的 `DecidedAll`、取预算之前放还持着的事实的份额）与 `content_test.go`；`PageStates` 多读 `content_hash`（`contents.sql`、sqlc 生成、`PageState.Hash`，`store.go:194` 填它）；`TestPageStates` 在同一个提交里改为逐字段比、断言哈希（它今天以 `!=` 比整个 `PageState`，带 `[]byte` 之后编译不过，3.14.1，审查 NC2-M1）；`list` 的测试照旧（以带键的字面量造、不比）；`page/extension_test.go` 照旧。
7. **S1 的临时检查**：`readReverted` 对 C 删除的、有 `Base` 的页答 `ErrRevertUnordered`（第 0 节）；`page/domain/revert_test.go:342-347` 那一行的期望在同一个提交里改为 `ErrRevertUnordered`（审查 C-M3）。在工具之前：从工具能写出这种变更集的那个提交起，它就在。
8. **用例 `ApplyBatch`**：计划、读与解析的界线、锁的预检、解析、单元里的重做、答复要的读、日志（`unit_ms`）；`apply_batch_test.go`；组合根的 `pageDeps` 设 `Deps.BatchLinks`、`BatchLinks` 与参与者的适配。
9. **mcpserver 的 `ToolError.At`** 与它的测试。
10. **工具**（一个提交）：`batch.go`（`batchIn`、`batchAnswer`、`batchErr`、`atOp`）、`Tools` 的第 11 个、`tools.json` 的两个条目、内置的说明的一行、码的表的一行、`declaredCodes`（`mcpOwnCodes`）、矩阵的一行、`validToolArguments`；适配器的测试、码的调用（3.13.1）。
11. **组合根的测试**：3.14.2–3.14.6 的整个程序测试与交错；`revertWorld.batches`（只有 `TestTheSearchIndexIsItsRebuild` 打开，审查 C-M4）；按名字放进第二、第三份（6.5），以 `-race` 的 `go test -list` 核对，量各份。
12. **`MaxBatchRewrites`**：`BenchmarkBatchRewrites` 手跑定档，写进常量的注释与审查记录；守卫 `TestABatchAtItsRewriteBoundTakesItsTime` 进 `test-go-guards` 的一行（带 `-v`），断言定档时量得的本机用时的三倍、至多 12 秒，注释写下量得的用时；`MaxBatchRewrites` 过了 1,000 时再照 3.10.1 量一次搜索。S1 定为 400；修正的第 2、5 轮另以链接密与页的字节的两档定 `MaxBatchRewriteLinks`、`MaxBatchRewriteBytes`，守卫那一行加它们的测试（3.8.2）。
13. **e2e**：MC5 (MCP) 与 4.1 的 fixture（`expectNoAsideNames` 带它的正向的对照）。
14. **文档**：第 7 节的 S1 的那些（README 的 `batch` 一条与工具的一句、v0.1 总体设计 13.1 第 1、5、31 条的 batch 部分与 13.4 第 10 条）、本文第 9 节、00 号文档第 12 节；门禁（6.4）、审查、修复、核对、合并、推送 `main`。

### 5.2 S2（`m9-p3-s2`）

1. **`revertSim` 与四个前提**，`schedule()`（轮数的上限）与三条后备、`RevertAside`、`Seq`、`Current`，`round()` 删去；单元执行 `RevertAside`（经 `place`，`unit_revert.go`：没有它，带临时的步的计划没有执行的人，审查 C-M2）；`TestScheduleIsTheRoundForM8sChangesets` 先以 `round()` 生成期望（同一个提交里删去 `round()`）；同一个提交改 `TestPlanRevertOfASwapIsUnordered`（改名 `TestPlanRevertOfASwapGoesAside`，断言步）与 `page/app/revert_test.go:456-464`（改为存着的缺陷"比 `MaxDepth` 深的页被改名"，S2 的实施的偏离 2），审查 C-M3。
2. **`AsideName`** 与它的表。
3. **`RevertWrites`**、`ContentRef`、写回以 `op.Current` 为基、两份正文的界线（`UnitTooLarge` 的不同的页），S1 的临时检查删去；`revert_test.go:342-347` 那一行改为 `restore P`、`content P`（同一个提交）。
4. **步数**：`Steps()` 与 `MaxRevertSteps` 的注释。
5. **任务的临时的拒绝**（第 0 节）与 `jobs_test.go` 的一行。
6. **迁移 00045** 与它的测试；`MarkChangesetMoves`、单元结束时设上、`ChangesetMoves` 读列、`HistoryReads.detail` 的注释；契约的两个说明、生成。
7. **"changeset reverted" 那一行的 `aside`**。
8. **domain 的性质测试**（`TestScheduleEndsAndPutsTheNotebookBack`，自己的模型作判定）与构造的测试（第 2、3 条后备、缺陷）；**组合根**：`TestTheRevertOfEachBatchShape`（两行以 SQL 种下）、撤销的性质测试打开 `batches` 与两种定向的生成器、`TestTheChangesetsRenamesCountEveryItem`、`TestTheMultiRoundRevertMeetsAWrite`；量各份。
9. **e2e**：MC5 (page)。
10. **文档**：README 的"日志"与"整组撤销"两条、v0.1 总体设计 13.1 第 1 条的整组撤销一段、第 31 条的整组撤销一段、13.4 第 13 条；本文第 9 节、00 号文档第 12 节；门禁、审查、修复、核对、合并、推送。

### 5.3 S3（`m9-p3-s3`）

1. **domain**：`schedule()` 记下每个删除那一刻的子树（`RevertOp.Subtree`）；`JobStep.Joined`（page 与 transfer）。
2. **`revertSteps`**：紧凑的 `revertStep`（带 `seq`）、删除照那一刻的子树（`locked()` 同）、段、写回紧跟恢复、恢复截到要的成员。S2 的临时的拒绝还在。
3. **`RunBatch`** 的"一个节点"。
4. **`CheckRevertPart`** 与 `RevertBatch` 的新的核对：L（成员 0（一组的第一个成员）在这一批里时读全组），再按次序核对前提（批里的 `canDelete` 比那一步记下的节点、以多出的每一部分的顶为名）；按 `seq` 从紧凑的计划取步、恢复的组与时刻取这一批读的；写回的基；一组在一批里一次恢复。M8 的 `TestARevertJobOfAGroupDeletedAgainSinceThePlan` 与 `TestARevertJobsBatchPlansItsOwnNodes` 不改而通过。
5. **transfer 的 `batch`**：按段长、减半到 `max(第一段的末尾, 一半之内的最后一个边界)`、`revert_too_large`；`stopped`；报告。S2 的临时的拒绝在这一步删去：到这里每一批才都整段、每一批的核对才是新的。
6. **契约与网页**：`TransferFailure` 的取值、生成、`transfer-report.tsx` 的一行、中英两条文字、vitest 与 mock。
7. **测试**：`TestTheJobsStepsReplayToStar`（照 `RevertBatch` 读每一批，注入含 `movegone`）、`unit_revert` 的表、transfer 的测试、组合根的四个与 `TestTheChangesetsRenamesCountEveryItem` 带临时的步的那一种；HI5 与 M8 的后台的测试照旧。
8. **文档**：README 的"任务"一条、v0.1 总体设计 13.1 第 1 条的后台一段与第 31 条的后台一段；本文第 9 节、00 号文档第 12 节（P3 完成）与变更记录、"移交的落实"的 P3 的各行写上落实；门禁、审查、修复、核对、合并、推送。

## 6. 测试与验证

### 6.1 各项与它们的测试

| 项 | 单元与包的测试 | 整个程序 | e2e |
|---|---|---|---|
| 字段名（3.2） | `TestEveryRuleNamesTheFieldsItIsGiven`；P2 的表格测试照旧 | 码的调用（3.13.1） | — |
| 取值与计划（3.3、3.4） | `TestCheckBatch`、`TestPlanBatch`、`TestBatchPlanContents`、`TestBatchPlanChecksItsReads`、`TestPlanBatchBoundsWhatItsMovesCarry`、`TestBatchPlanMovesBetween`、`TestRelocationOfIsMovesOrder`、`TestPageStates` | `TestTheMCPBatchIsRefusedWhole`、`TestTheMergesOfABatchHoldOnTheDatabase` 的"过期的令牌、相同的字节"、`TestTheWritesOfABatchAreJudgedAsSeparateCallsJudgeThem` | MC5 (MCP) 第 3 步 |
| 合并、正文合成与回到原处的条目（3.5） | `apply_batch_test.go` | `TestTheMergesOfABatchHoldOnTheDatabase` | MC5 (MCP) 第 2 步 |
| 单元里的重做（3.6） | `apply_batch_test.go`（`TestABatchIsPlannedAgainOnce`、`TestABatchsLockPrecheckIsInItsOpsOrder`） | `TestTheBatchIsPlannedAgainAfterAWrite`、`TestTheMCPBatchMeetsAWebRename` | — |
| 推后的改写（3.7） | `rewrite_test.go`、`TestPathBeforeUndoesEveryChangeFromTheRoot`、`TestRelocationRecasesEveryPage`、`BatchLinks` 的测试、`batch_rewrite_test.go`（冻住的节点、新建的节点、先改写写了的页、一棵树的读） | `TestTheLinksOfABatchAreThoseOfSeparateCalls`、`TestTheWaysABatchDiffersFromSeparateCalls`、`TestTheBatchsRewriteMeetsAnOpening` | MC5 (MCP) 第 2 步 |
| 界线（3.8） | `TestCheckBatch`、`TestBatchPlanContents`、`apply_batch_test.go`、`content_test.go`、`batch_rewrite_test.go` 的三个数与解析额度 | `TestTheMCPBatchIsRefusedWhole`、`TestTheLargestBatchIsFarWithinTheValuesBound`；守卫那一行的 `TestABatchAtItsRewriteBoundTakesItsTime`、`TestABatchPastItsLinksBoundIsRefused`、`TestABatchPastItsBytesBoundIsRefused`、`TestABatchCountsThePagesItWroteAtTheirFinalSize`、`TestABatchWhosePagesOutgrowTheParseBudgetIsRefused` | — |
| 答复与错误（3.9） | 适配器的 `batch_test.go`、`TestAnErrorsPlaceFollowsItsCode` | 码的调用、`TestTheMCPBatchIsRefusedWhole` | MC5 (MCP) 第 3 步 |
| 搜索与事件（3.10） | `TestABatchQueuesForTheSearchInBulk` | `TestTheMCPBatchReachesItsUseCase`、`TestTheSearchIndexIsItsRebuild`、`TestThePagesEventListsNoPageInTheTrash`（S2） | — |
| 工具、清单、说明（3.12） | `tools_test.go`、`instructions_test.go` | `TestTheToolListsAreTheContract`、说明的两个测试 | MC1 第 5 步（照文件） |
| 码的表、矩阵、字符串、动作（3.13） | — | 3.13 的四处 | — |
| 4.11 的性质（3.14.6） | — | `TestTheIndexIsItsRebuild` 的 batch 一步 | — |
| 调度（3.16） | `TestScheduleIsTheRoundForM8sChangesets`、`TestScheduleRevertsEachBatchShape`、`TestScheduleEndsAndPutsTheNotebookBack`、`TestTheSecondFallbackPicksTheNodeItFrees`、`TestTheThirdFallbackMovesAPlaceOffTheRoot`、`TestScheduleAnswersUnorderedForADefect`、`TestPlanRevertOfASwapGoesAside`、`TestTheSecondFallbackTakesTheLeastFirst`、`TestTheSecondFallbackMovesOnlyANodeToPlace`、`TestAFallbackRenamesANodeAsideOnce`、`TestARestoreWaitsForTheGroupItGoesUnder`、`TestAScheduleTakesMoreRoundsThanItsSteps` | `TestTheRevertOfEachBatchShape`、撤销的性质测试、`TestTheMultiRoundRevertMeetsAWrite` | MC5 (page) |
| 临时名称（3.17） | `TestAsideName`、`TestNumbered`、`TestAStepAsideTakesNoKeyOfS` | `TestTheRevertOfEachBatchShape` 的长名称与附件 | MC5 (page) 的 `expectNoAsideNames` |
| 先恢复再写回、撤销的撤销（3.18） | `TestRevertWritesAreEveryVersionOnABase`、`TestARevertRestoresAPageThenWritesItBack`、`TestARevertsContentsAreParsedByPageAndHash`、`TestDecidedAllGivesItsSharesBackBeforeATakeThatCannotFit`、适配器的 `TestTheAdapterKeepsTheFactsShare` | `TestTheRevertOfEachBatchShape` 的两行、`TestTheRevertOfALonePagesTwoContentsAtTheLeastBudget`、撤销的性质测试（回收站里的正文） | — |
| 步数（3.19） | `TestStepsCountTheStepsAside` | — | HI5（S3） |
| `renames`、`moves`（3.20） | `TestTheChangesetsMovesAreBackfilled`、`TestAUnitMarksItsChangesetsRenamesAndMoves`、适配器的 `TestChangesetMoves` | `TestTheChangesetsRenamesCountEveryItem` | MC5 (page) 的对话框提醒 |
| 后台的段、切批与核对（3.22–3.24） | `TestTheJobsStepsReplayToStar`、`revertSteps` 与 `unit_revert` 的表、transfer 的 `background_test.go`、`TestARevertJobOfAGroupDeletedAgainSinceThePlan`、`TestARevertJobsBatchPlansItsOwnNodes` | `TestTheBackgroundRevertOfANameCycleKeepsItsSegments`、`TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches`、`TestTheBackgroundRevertOfASegmentPastABatchStops`、`TestTheBackgroundRevertOfALargeFolderHoldsItsGroup` | HI5 |
| 网页的一条文字（3.25） | `transfer-report.test.ts`、`messages.test.ts` | — | — |

### 6.2 P3 怎样量它的界线

本文没有跑需要数据库的东西；审查、修订与复核在 `f1733403` 的副本上跑了不要数据库的 Go 测试（`-short`）与 Python 的模型：

- **`MaxBatchRewrites`**：照 M8 的规则（满负载下单元在本机约 4 秒之内的最大一档）由实施者以 `BenchmarkBatchRewrites` 手跑定档（3.8.2：R 页是满负载的页，约 84 KB、200 条链接；100、200、250、300、400、500、1,000 页，每档三次的中位数，单元之内的用时读"batch applied"那一行的 `unit_ms`，与整个调用的用时），在 CI 上由 `TestABatchAtItsRewriteBoundTakesItsTime` 守着界线处的用时（量得的三倍、至多 12 秒；带 `-v`，用时进作业的日志）。本文的预计（200 一档）来自审查 A 的实测：100 KB 的正文解析一次 8.68 ms，改写一页解析两次，约 17 ms 一页。S1 定为 400；碰到的链接与改写的页的字节的界线由修正的第 2、5 轮以链接密与页的字节的两档同样定（3.8.2）。
- **读的与计划的解析的界线**（3.4.4、3.8）：审查 A 在副本上的实测，`BenchmarkParse`（Apple M5 Max）：100 KB 8.68 ms、1 MiB 43.0 ms、5 MiB 210.4 ms 与 301.8 MB 的分配；M4 记下的病态的正文 5 MiB 约 1.4 秒；`TestReviewAPlanContentsHeld`：100 页 5 MiB 读进来、各改一次，活着的堆 +979 MiB、分配 1,469 MiB；复核的 `TestCheckAPlanTextsOfWritesWithABase`：100 页 5 MiB 只是被带 `base_version` 的 `write` 替换，读进来也是 +489 MiB（所以它们只读头，不带 `base_version` 的 `write` 也是，第二轮复核 A3-N1）。所以：计划、最后的正文与参与者的读回至多解析 20 MiB（一页时）或 16 MiB（多页时），正常约 0.84 秒、病态约 5.6 秒；重新计划一次再加计划与最后的正文的至多 15 MiB，正常约 0.63 秒、病态约 4.2 秒（3.8）；读的正文至多 14 MiB，约 27 MiB 的堆。原型的预算核对（`exp/`，5 MiB 的预算）：计划持着约 0.50 MiB、单元持着约 0.80 MiB 时整份的取预算都等不到，放还份额之后取得到（3.4.5）。界线处由 `apply_batch_test.go`、`content_test.go` 与 `TestTheMCPBatchIsRefusedWhole` 核对（3.14.1、3.14.5）。
- **搜索的第一页**：3.10.1 的 `NERVE_SEARCH_BACKLOG`，在 M8/P5 的语料上手跑（n = 0、100、1,200、2,200，5.1 第 0 步），判断的条件写在 3.10.1。S1 的语料是造的（M8/P5 的已删），修正的第 1 轮以最新的页的积压重量 n = 0、100、1,600、2,200（3.10.1）。
- **`MaxBodyValues` 的对照**：最大的合法 `batch` 与带全部成员的 `batch` 发出去（`TestTheLargestBatchIsFarWithinTheValuesBound`），不数值；715 与 1,215 是按 schema 算的、写在 `MaxBatchOps` 的注释里：每项至多 7 个值（对象、`op` 与 `edit` 的五个参数）乘 100 加外层的约 15 个；带上平的项的全部 11 个成员是每项 12 个。
- **答复的上限**：照编码算（3.9.1），由适配器的测试在最坏的取值处核对；错误的答复加 `At` 与 `op` 约 40 字节，P2 的最坏 38,522 字节之上仍在 48 KiB 之内。
- **内置的说明**：以 `TextBytes` 的数法（3.12.3，`scratchpad/m9/p3/notes/instr3.py`；审查 A 与 C 的数一致），`TestTheBuiltInInstructionsAreShort` 守 4 KiB。
- **链接的规则**：`scratchpad/m9/p3/exp/` 的原型（`f1733403` 的 `linking/app` 与 `linking/domain`，内存里的假的端口、真的解析器，`go test -short`），每种情形一本 batch、一本分开调用，断言 batch 写出 4.11 的规则说的字节（3.7.1、3.14.3、3.14.4）；第二轮复核加了 `spread` 展开被移动的节点的写法（每个节点的子树一个 batch 读一次）：39 种情形都与每页带一份子树时写出的相同，100 页 × 1 万个节点时活着的堆由约 298 MiB 降到约 1 MiB（`results.txt` 的 ROUND 3，3.7.2 (e)）。
- **调度与后台**：随机的模型（3.16.4，`scratchpad/m9/p3/sim/` 与审查之后的 `sim2/`），离线；段的量（3.22.2）、每批的核对（3.24，复核之后的 `delcheck`、`reads` 与 `movegone` 由 `run_r2.sh` 跑）、减半（3.23）、恢复的循环（3.24）同在 `sim2/`。
- **组合根的各份**：6.5 是估计与规则，实施者在每个 Step 量两次。

### 6.3 变异

实施者逐个做下面的改动（每个之后还原），确认右边的测试失败，结果写进第 9 节（照 P2 的 6.3：每个变异列出的测试都要能因它失败）。"第 n 种"是 3.14.3 的情形。审查与复核去掉、换掉的变异与理由在审查记录里。

| # | 变异 | 应失败的测试 |
|---|---|---|
| 1 | `MoveKinds` 写死 `"to"` | `TestEveryRuleNamesTheFieldsItIsGiven` |
| 2 | `DeletionOf` 的 `Recursive.Field` 写死 `"recursive"` | 同上；适配器的 `batch_test.go`（文字里是 `recursive`） |
| 3 | `page/domain/content.go` 加一个不在表里的 422 | `TestEveryRuleNamesTheFieldsItIsGiven` 的表的完整（扫全部非测试文件） |
| 4 | `CheckBatch` 先查各项、后查合计 | `TestCheckBatch`（合计带着坏的项的三行） |
| 5 | `MaxBatchOps` 的比较差一（100 项也拒） | `TestCheckBatch` 的 100 项一行 |
| 6 | 不认识的 `op` 当作 `write` | `TestCheckBatch` 的"`op` 不认识"一行 |
| 7 | 这一项的工具不收的成员不拒 | `TestCheckBatch` |
| 8 | 第二遍跑全部的项 | `TestBatchPlanContents`（`Failed` 及其后有改写、有解析） |
| 9 | 不规范化 | `TestPlanBatch`；`TestTheMergesOfABatchHoldOnTheDatabase`（500） |
| 10 | 规范化并进不在子树里的删除 | `TestPlanBatch` |
| 11 | 去掉 B1 | `TestPlanBatch`；`TestTheMCPBatchIsRefusedWhole` |
| 12 | 去掉 B3 | 同上 |
| 13 | 给新建的页的 `base_version` 不拒 | 同上 |
| 14 | 计划里不改写就匹配 | `TestBatchPlanContents`；第 2 种 |
| 15 | 计划的解析在解析之后才数（`Admit` 不数单元的页数与字节，解析之后由 `CheckUnit` 不带 `op` 拒绝；用例里先解析、后 `Admit` 的写法，真的库上的测试答的相同，由 `TestBatchPlanContents` 的一页过 10 MiB 一个字节抓到：实施的偏差 24、25） | `TestBatchPlanContents`；`TestTheMCPBatchIsRefusedWhole` 的 60 项 `edit`（没有 `op`） |
| 16 | 每个正文操作各写一次（合成的写落在最后一个正文操作的写法，真的库上的测试答的相同，由 `apply_batch_test.go` 抓到：实施的偏差 24） | `apply_batch_test.go`（守卫两次）；`TestTheMergesOfABatchHoldOnTheDatabase`（新建之后 `edit` 的版本号是 2） |
| 17 | 重做时不比节点 id | `apply_batch_test.go`（两次网页的改名，一次交换） |
| 18 | 重做时不比版本号 | `apply_batch_test.go`；`TestTheBatchIsPlannedAgainAfterAWrite` 的第二个子测试（字节相同的 `write` 不是 409） |
| 19 | 重做两次 | `apply_batch_test.go` |
| 20 | 守卫的拒绝也重做 | `apply_batch_test.go` |
| 21 | 参与者每个操作调一次（不推后） | `apply_batch_test.go`；第 3 种 |
| 22 | 交给参与者的改动带版本号 | 第 17 种（X 的项带着它新建时的版本号 1，`Relocation` 答不用改，`linking/domain/change.go:68-73`） |
| 23 | 新建之后改名的节点不放进合并的改动 | `apply_batch_test.go`；第 17 种（没写的页的链接被抢走） |
| 24 | (a)：写了的页的链接取自索引 | `rewrite_test.go`；第 1 种 |
| 25 | (b)：照"正文比索引新就跳过" | 同上 |
| 26 | (c)：追加的写以索引的版本号为基 | 同上（500） |
| 27 | (d)：`was` 取单元之前的 | `rewrite_test.go`；第 3 种（`[[C\|B]]`） |
| 28 | 不去掉 `Gone` 的来源页的行 | 第 6 种（500）；`rewrite_test.go` |
| 29 | "之前"的目标被删的链接记 error | 第 8 种（日志的捕获里有 error 级的行） |
| 30 | `PathBefore` 碰到第一个改动就返回 | `TestPathBeforeUndoesEveryChangeFromTheRoot`；第 4 种 |
| 31 | 原来的父节点的路径读现在的树 | 第 5 种（`[F/X](X.md)`） |
| 32 | `MaxBatchRewrites` 的比较差一 | `TestABatchAtItsRewriteBoundTakesItsTime` |
| 33 | 不数条目 | `apply_batch_test.go`；`TestTheMCPBatchIsRefusedWhole` 的 10,001 个节点 |
| 34 | 一页的例外按份数 | `apply_batch_test.go` |
| 35 | 答复的 `Fit` 截掉变更集 | 适配器的 `batch_test.go` |
| 36 | `At` 不写进 `Fit` 的文字 | `TestAnErrorsPlaceFollowsItsCode` |
| 37 | `Extra` 没有 `op` | 适配器的测试；`TestTheMCPBatchIsRefusedWhole`；MC5 (MCP) 第 3 步 |
| 38 | 码的表少 `page.batch_too_large` | `TestTheMCPToolsErrorsCarryTheirCodes` |
| 39 | `declaredCodes` 不加 MCP 自己的码 | `TestEveryMCPToolHasItsCodes` |
| 40 | 矩阵没有 `batch` 的行 | `TestThePermissionMatrixCoversEveryTool` |
| 41 | `validToolArguments` 没有 `batch` | `TestToolStringsDoNotAnswerInternalErrors` |
| 42 | `tools.json` 的说明与代码差一个词 | `TestTheToolListsAreTheContract` |
| 43 | 说明没有 `batch` 的一行 | `TestTheBuiltInInstructionsNameEveryTool` |
| 44 | 规则表没有 `page.batch` | `bootstrap/actions_test.go`；矩阵的写者列（`forbidden`） |
| 45 | batch 准确入队（S1 照第 0 步的重量改为批量入队之后反过来，S1 审查 C-I2） | `TestABatchQueuesForTheSearchInBulk`；`TestTheMCPBatchReachesItsUseCase`（答复的 `building`；任务之前搜索答 `building`、找不到） |
| 46 | 单元不用计划给的 id（`PageDraft.ID` 不看） | `apply_batch_test.go`（参与者收到假的库没建过的 id）；第 10 种 |
| 47 | `resolutions` 不当作没有 `Gone` 的页的别名 | `rewrite_test.go`；第 7 种（500） |
| 48 | 写了的页照合并的改动（在 `rewrite.go:49` 返回） | `rewrite_test.go`；第 11–13 种 |
| 49 | 写了的页的链接全都比，不看它自己的改动 | 第 14 种之后再改名的一行（新的 X 拿走的链接被写回；照设计的第 14 种写了的页不被跟随，看不出：实施的偏差 26） |
| 50 | `Recased` 只留最后一个 | `TestRelocationRecasesEveryPage`；第 16 种 |
| 51 | `Lead` 只留 id | `TestBatchPlanContents` 的歧义的链接 |
| 52 | 先读正文、再比大小 | `apply_batch_test.go`（`PageContent` 被调） |
| 53 | 计划的解析不限 | `TestBatchPlanContents`；`TestTheMCPBatchIsRefusedWhole` 的 50 对（答 `ops[3]`） |
| 54 | 计划的解析的事实不按（页、哈希）留 | `TestBatchPlanContents`（同一份正文解析两次） |
| 55 | 条目在第二遍之后才比 | `TestPlanBatch` 的审查 A-M11 的第一种 |
| 56 | 不调 `DropUnchanged` | `TestTheMergesOfABatchHoldOnTheDatabase` 的"改名再改回"（有变更集、有一条"调整了次序"） |
| 57 | 参与者再解析写了的页的最后的正文（不用 `Written` 的事实） | `rewrite_test.go`（假的解析器对它们最后的正文被调了） |
| 58 | 改写的页在 `refuseLocked` 与解析之后才数 | `rewrite_test.go`（过界时 `Locks` 与解析器被调了） |
| 59 | 没有锁的预检 | `apply_batch_test.go`（被编辑的页的正文照样解析） |
| 60 | 给新建的页带令牌的 422 排在 409 之后 | `TestPlanBatch`（审查 A-M12 的一行） |
| 61 | 之后被删除的页的正文操作答最后的版本 | `apply_batch_test.go`（没有版本、没有路径） |
| 62 | 计划之后被别人删除的目标记 error | `rewrite_test.go` 的 warn 一行 |
| 63 | `spread` 对写了的页只展开改名的节点，不展开它自己的改动移动的 | 第 19 种（P 留着悬空的 `[[Top/F/X]]`）；`rewrite_test.go` 的移动的节点的一行 |
| 64 | 计划读只读头的页的正文 | `apply_batch_test.go`（100 项 `write`：`PageContent` 被调；三页不带 `base_version` 的：答第三项的 422） |
| 65 | 带过期的令牌、字节相同的 `write` 答 `page.revision_mismatch` | `TestBatchPlanContents` 的审查 A2-M4 的第一行 |
| 66 | 会话取预算之前不放还它持着的事实的份额 | `content_test.go`；`apply_batch_test.go` 的一页 5 MiB 改写一次（503） |
| 67 | 推后的改写之前不放还 `DecidedAll` 的份额（或只放还写了的那一页的） | `apply_batch_test.go` 的 P 5 MiB 与 Q 3 MiB（503） |
| 68 | `DecidedAll` 另等一个 `parse_max_wait`，不与计划的解析共用 | `content_test.go` 的共用一个等待 |
| 69 | `DropUnchanged` 也去掉写了正文的、回到原处的节点的改动 | `TestTheMergesOfABatchHoldOnTheDatabase` 的"改名再改回，中间写它"（`checkLinks`：A 的链接的索引是旧的正文的） |
| 70 | `PageStates` 不填 `Hash`（`store.go:194`） | `TestPageStates`（A 与 B 的哈希）；`TestTheMergesOfABatchHoldOnTheDatabase` 的"过期的令牌、相同的字节"（答 `page.revision_mismatch`） |
| 71 | 恢复不核对层级 | `TestScheduleEndsAndPutsTheNotebookBack`（模拟：49 次） |
| 72 | 删除不等写回 | `TestScheduleEndsAndPutsTheNotebookBack`（模拟：7,494 次）；`TestTheRevertOfEachBatchShape` 的撤销的撤销；撤销的性质测试（"先写正文再删除"的 batch 立即撤销一次之后，S2 的实施的偏离 9） |
| 73 | `RevertWrites` 照 M8 | `TestRevertWritesAreEveryVersionOnABase`；`TestTheRevertOfEachBatchShape` 的"先写正文再删除"（模拟：7,555 次） |
| 74 | 去掉第 1 条后备 | `TestScheduleRevertsEachBatchShape` 的互换名称（在根下）；`TestTheRevertOfEachBatchShape` |
| 75 | 去掉第 2 条后备 | `TestTheSecondFallbackPicksTheNodeItFrees`；S2 的修正的第 1 轮起也有 domain 的性质测试的全量一次（按移动的样子，审查 C-N2） |
| 76 | 第 3 条后备取根下的节点 | `TestTheThirdFallbackMovesAPlaceOffTheRoot` |
| 77 | 临时名称取前 8 位 | `TestAsideName` |
| 78 | 临时名称不截短 | `TestAsideName`；`TestTheRevertOfEachBatchShape` 的长名称（`nodes_name_check`） |
| 79 | 附件的临时名称丢掉扩展名 | `TestAsideName`；S2 的修正的第 1 轮起也有 domain 的性质测试（模型的附件要留着扩展名，审查 A-M2）；组合根的附件那一行抓不到：丢了扩展名仍是合法的标题，只在单元之中（S2 的实施的偏离 11，审查 C-N6） |
| 80 | 写回以 `state.Contents` 为基 | `TestTheRevertOfEachBatchShape` 的"先写正文再删除"（500） |
| 81 | 解析好的正文按页 id | `TestARevertRestoresAPageThenWritesItBack`（读守卫看到的每一步的事实）；M8 的两个用例的测试：`TestRevertChangesetAnswersItsCodesInOrder` 的 "a page in the trash at other bytes meanwhile" 与 `TestRevertChangesetsLockedConflictsComeByNodeID`（键里没有哈希时 `parsedAs` 看不出两次读之间改了的正文）；组合根的"先写正文再删除"那一行看不出：恢复的事实只到守卫，索引照写回的（S2 的实施的偏离 10，审查 B-N2、C-N6） |
| 82 | `Steps()` 不数临时的步 | `TestStepsCountTheStepsAside` |
| 83 | 补值只看前 1,000 项 | `TestTheChangesetsMovesAreBackfilled` |
| 84 | 单元结束时不设两列 | `TestTheChangesetsRenamesCountEveryItem`；`TestAUnitMarksItsChangesetsRenamesAndMoves` |
| 85 | S2 的任务的临时的拒绝去掉（在 S2 里） | `TestARevertJobRefusesTheStepsItsBatchesWouldSplit` |
| 86 | "changeset reverted"那一行不带 `aside` | 撤销的性质测试的不空转 |
| 87 | 不标 `Joined` | `TestTheJobsStepsReplayToStar`；`TestTheBackgroundRevertOfANameCycleKeepsItsSegments`（批的边界上的临时名称） |
| 88 | 恢复不截到要的成员 | `revertSteps` 的表 |
| 89 | 写回不紧跟它的恢复 | `revertSteps` 的表；`TestTheJobsStepsReplayToStar` |
| 90 | 减半退到不大于一半的最后一个边界、是 `from` 就停 | transfer 的 `background_test.go`（`[三步的段, 重的一步]` 停下） |
| 91 | 一段太大时答 `internal` | transfer 的测试；`TestTheBackgroundRevertOfASegmentPastABatchStops` |
| 92 | `RunBatch` 改回"多于一步" | transfer 与 page/app 的测试（同一节点的两步） |
| 93 | 每批不做 L | `TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches`（网页写的正文被盖掉） |
| 94 | 写回以任务开始时的 `Current` 为基 | 同上的第二个子测试（网页把 E 改成别的字节再改回：版本号 +2、哈希不变，任务以 `internal` 失败） |
| 95 | 删除的步照计划时的子树 | `revertSteps` 的表；`TestTheBackgroundRevertOfALargeFolderHoldsItsGroup` 的 (3) |
| 96 | 每批仍建 S\*（`before()`） | `TestTheJobsStepsReplayToStar`；`TestTheBackgroundRevertOfALargeFolderHoldsItsGroup` 的 (1) |
| 97 | 这一批恢复的页的写回照活着的页核对 | `TestTheJobsStepsReplayToStar`；同上的 (2) |
| 98 | 恢复的循环只恢复连着的一段 | `unit_revert` 的表的两行；同上的 (4) |
| 99 | 恢复的时刻取任务开始时的计划 | `TestARevertJobOfAGroupDeletedAgainSinceThePlan` |
| 100 | 网页少一种语言的文字 | `messages.test.ts` |
| 101 | 成员 0（一组的第一个成员）在这一批里时只读这一批的成员 | `TestARevertJobOfAGroupDeletedAgainSinceThePlan`（批的大小 1 与 3）；`TestTheJobsStepsReplayToStar`（跨批的组答假的 `changed`） |
| 102 | 批里的 `canDelete` 只看子树里有没有 S\* 里活着的节点 | `TestTheJobsStepsReplayToStar` 的注入 `movegone`（那个节点在 L 读它之前被一起删除） |
| 103 | 批里的 `canDelete` 以删除的顶为名 | `TestARevertJobsBatchPlansItsOwnNodes`（后一半：`e.Conflicts[0].NodeID == z.ID`） |

### 6.4 门禁

每个 Step 合并之前，本机：`make lint`、`make knip`、`make gen-check`、`make test-go`（竞态检测下的全部、组合根的三份、`test-go-costs` 与 `test-go-guards`）、`make test-go-short`、`make test-web`、`make build`、`make e2e`，都通过；CI 的各份都是绿的。之后照 M9 的做法：审查（Opus）、修复、核对、合并、推送 `main`（负责人的规则：完成并核对过的单元由主持者推送）。

### 6.5 CI 的余量

**现在**（主持者告知的 S2 合并之后的 run 39245208047）：组合根三份的测试一步 6:58、4:32、3:24；第一份最近两次 7:09 与 6:58，平均约 7:04、慢的约 7:10；线在约 7:30（`-timeout 15m` 的一半，第 11 节第 28 条）。**第一份几乎没有余地**。

**规则照旧**（[P2](02-P2-writes-history.md) 6.5，第 11 节第 28 条）：一份过了线（两次运行的平均里测试一步过了约 7.5 分钟），只在一个邻居挪过之后两次运行的平均仍在线之下时才挪它们之间的界线；否则那个 Step 按测试名把它再分一份；从不提高 `-timeout`。所以 P3 **把每个新的整个程序测试的名字取在第二、第三份里**：`TestThe` 之后的第一个字母在 F–Z 的（`TestTheM…`、`TestTheL…`、`TestTheR…`、`TestTheW…`、`TestTheI…`、`TestTheS…`）落在第三份（`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])`），`TestTheB…`、`TestTheC…` 与 `TestReverting…` 落在第二份；组合根里**没有一个新的测试叫 `TestA…`、`TestBatch…`、`TestDeleting…`、`TestEvery[A-R]…`**（第一份）。S3 的四个取 `TestTheBackgroundRevert…`（第二份）：第三份在 S1、S2 之后已多了一分多钟，第二份只多约半分钟（下表）。

**守卫不在任何一份里**（审查 C-N13）：`TestABatchAtItsRewriteBoundTakesItsTime` 的名字匹配 `BOOTSTRAP_FIRST`，但它的文件 `batch_cost_test.go` 是 `//go:build !race`，组合根的三份以 `$(GO_TEST_RACE)`（`go test -race`，`Makefile:179`、`:228-237`）编译，它不在其中，只在 `test-go-guards` 自己的一行里跑（`Makefile:259-280`）。所以实施者核对落点时以 `-race` 列出：`cd server && go test -race -list '<表达式>' ./internal/bootstrap`，对 `BOOTSTRAP_FIRST`、`BOOTSTRAP_THIRD` 各一次（第二份是其余的），每个新的或改了名的测试各在哪一份写进审查记录；不带 `-race` 列出时守卫会出现在第一份的表达式之下，看起来像落在第一份。

| 份 | 现在（测试一步） | S1 加的（本机，竞态） | S2 加的 | S3 加的 | P3 之后的估计（CI） |
|---|---|---|---|---|---|
| `bootstrap-1` | 7:09、6:58（约 7:04） | 无（`TestEveryMCPToolHasItsCodes` 多一行，只读文件；守卫不在这一份，上一段） | 无 | 无（`TestABackgroundBatchPastTheLinksIsHalved` 在这一份：它是回收站的恢复的任务，恢复的步没有段，每一步是一个边界，减半的新规则对它与今天相同，用时不变） | 不变，约 7:04 |
| `bootstrap-2` | 4:32 | `TestTheBatchsRewriteMeetsAnOpening` 约 2 秒、`TestTheBatchIsPlannedAgainAfterAWrite` 约 3 秒（两个子测试）：约 +5 秒 | 撤销的性质测试 +2–4 秒（今天本机 5.2–5.8 秒，3.21.3）；`TestTheChangesetsRenamesCountEveryItem` 约 3 秒（含一个导入与一个后台任务）：约 +5–7 秒 | 四个 `TestTheBackgroundRevert…`：名称环约 3–4 秒、批之间的写约 5 秒（两个子测试，第二个的批的大小 1）、一段过了一批约 3 秒（101 页、各 210 条链接的种子）、大的文件夹约 7–8 秒（四个子测试）：约 +18–20 秒 | S1、S2 实测本机 +10.3 秒（131.4 → 141.7 秒，S2 的修正的第 1 轮之后），S3 估计 +18–20 秒；乘这一份最近的比约 2.33（329.5 ÷ 141.7）：约 6:12–6:17（S2 的修复核对 C 的 N3；原估计乘 1.93 约 5:26–5:34） |
| `bootstrap-3` | 3:24 | `TestTheMCPBatchReachesItsUseCase` 约 3 秒、`TestTheMergesOfABatchHoldOnTheDatabase` 约 4 秒（加了改名再改回的两行与过期的令牌的一行）、`TestTheLinksOfABatchAreThoseOfSeparateCalls` 约 10–12 秒（19 种，每种两本笔记本）、`TestTheWaysABatchDiffersFromSeparateCalls` 约 5–6 秒（十种）、`TestTheMCPBatchIsRefusedWhole` 约 7–9 秒（10,001 个节点与 14 MiB 以 SQL 种下、50 对）、`TestTheMCPBatchMeetsAWebRename` 约 2 秒、`TestTheLargestBatchIsFarWithinTheValuesBound` 约 1 秒；`TestTheIndexIsItsRebuild` +3 秒、`TestTheSearchIndexIsItsRebuild` +1 秒；矩阵 +26 格约 +5–8 秒、码的调用 +1–2 秒、工具的字符串、清单与日志约 +1 秒：约 +43–52 秒 | `TestTheRevertOfEachBatchShape` 约 12–14 秒、`TestTheMultiRoundRevertMeetsAWrite` 约 2 秒：约 +14–16 秒 | 无 | +57–68 秒本机，CI ×1.6–1.9 约 +1:31–2:09：约 4:55–5:33 |
| `guards` | 作业约 10–11 分钟 / 25 | `TestABatchAtItsRewriteBoundTakesItsTime` 单独一行、带 `-v`（不带竞态，本机约 8–10 秒：R 页、T 与两页以 SQL 种下并建索引，界线上的 batch，过界的 batch，3.8.2），乘这一份的比 2.4 约 19–24 秒 | 无 | 无 | 作业约 +0.5 分钟；这一行远在 Go 默认的 10 分钟的一半之下，不加 `-timeout` |
| `race`（组合根之外的包） | 作业约 8 分钟 / 20 | domain、app、linking、mcpserver、适配器的测试约 +5–10 秒 | 调度的性质测试 2 万次（不带竞态本机约 1–2 秒，竞态下按 5–10 倍约 5–20 秒）、迁移的测试约 1 秒 | `TestTheJobsStepsReplayToStar` 约 1 秒、transfer 的测试 | 约 +15–35 秒，作业仍在 20 分钟的一半之下 |
| `short` | — | domain 的表与性质测试的 `-short` 份（2,000 次） | 同 | 同 | 不变 |
| `e2e` | E2E 一步 3:35–4:17 / 20 | MC5 (MCP) 约 6–8 秒 | MC5 (page) 约 6–8 秒 | 无 | 约 4:00–4:35 |

S1 之后，守卫那一份的作业在修正的第 6 轮到了 12:47（batch 的一行有五个测试，本机 47.9 秒），两倍过了 25 分钟，时限照约两倍的规则提到 30 分钟（修正的第 7 轮）；那一行乘 2.4 约 115 秒，仍在 Go 默认的 10 分钟的一半之下，不拆。

S1 合并时（9.1）：三份本机单独两次的平均 250.6、131.4、145.3 秒；CI 上第二、三份的测试一步在 3:41–5:33 与 3:20–4:56 之间，第一份没有 S1 的测试，在 5:01 与 7:17 之间摆动，合并之后最近两次（第 7 轮的 5:47 与 `main` 的 7:00）平均 6:23.5：都在线下，界线没有挪、没有分、没有提高 `-timeout`。

S2 合并时（9.2）：三份本机单独两次的平均 251.9、133.0、153.0 秒（S1 的 +1.3、+1.6、+7.7；估计第二、三份 +5–7、+14–16：`TestTheRevertOfEachBatchShape` 竞态下 1.3 秒，不是估计的 12–14 秒，一个团队与一个库服务每一行）；修正的第 1 轮之后第二、三份 141.7、168.3 秒，第 2 轮之后第三份 161.1 秒。CI 上第二份的测试一步在 3:45–5:31、第三份在 3:24–5:20 之间（运行机的摆动：3:24 那次与本机之比 1.33，5:20 那次 1.90）；第一份没有 S2 与各轮的测试，修正的第 1 轮的两次 7:12、7:07，平均 7:09.5，离线约 20 秒，第 2 轮之后最近两次 7:07、6:08，平均 6:37.5：都在线下，界线没有挪、没有分、没有提高 `-timeout`。

**每个 Step 的做法**：合并之前以本机每份单独跑两次的平均乘这一份的 CI 与本机之比估计，合并之后以 CI 的两次运行核对，写进那个 Step 的审查记录（同 P2）。第二份在 P3 之后约 6:12–6:17、离线一分多钟，第三份 S2 之后最近两次平均 5:17（修正的第 1 轮之后）与 4:25（第 2 轮之后），不用挪界线、也不用分。万一第一份因 runner 的摆动或别人的改动过了线，`BOOTSTRAP_FIRST` 末尾的 `TestEvery[M-R]…`（本机竞态下 28.4 秒：两个 vault 的测试 10.75、7.79 秒，`TestEveryPageWriteKeepsTheIndex` 3.84 秒，`TestEveryOperationAcceptsAPersonalAccessToken` 2.57 秒）挪到**第三份**：`BOOTSTRAP_FIRST = ^Test([A-D]|E([a-u]|very[A-L]))`、`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z]|Every[M-R])`，第二份的 `-skip` 照旧对。第三份接过它们约 (168.3 + 28.4) × 1.88 ≈ 6:10，第一份降到约 7:09.5 − 28.4 × 1.71 ≈ 6:21；第二份接过它们约 7:18–7:23，离线只约 10 秒，所以不是第二份（S2 的修复核对 C 的 N3，主持者的决定 9）。这不是 P3 的测试引起的，但由发现它的那个 Step 照规则做；S3 照这一条记第一份的两次运行。`ci.yml` 不为新的一行改（审查 C-N12）：它跑 `make "test-go-${{ matrix.leg }}"`（`.github/workflows/ci.yml:95`），守卫的新的一行在 `Makefile` 里；S1 只改了守卫那一份的时限，由 25 分钟提到 30 分钟（修正的第 7 轮，S1 复核 C 的 N3-f4）。

## 7. 完成标准

**S1**：
- `batch` 在两个入口里，`tools.json` 的两个条目与代码相同；内置的说明有它的一行（`TextBytes` 425 字节），两个入口都在 4 KiB 之内（3,386 与 3,456）。
- 3.14 的每个测试、6.3 的第 1–70 条变异、MC5 (MCP) 通过或被抓到；3.14.2、3.14.3 的每种的日志的捕获里没有 error 级的行；搜索的实测（5.1 第 0 步）与判断、`MaxBatchRewrites` 的各档写进审查记录，`MaxBatchRewrites` 照 M8 的规则定下（本机约 4 秒之内的最大一档），守卫在 CI 上（断言量得的三倍、至多 12 秒）。
- P2 的全部测试照旧（字段名不变；`ErrBatchTooLarge` 改名不改行为，改名是单独的一个提交、在新的哨兵之前）；`page/domain/revert_test.go:342-347` 那一行照 3.21.1 改期望（S1 的临时检查）。
- 网页撤销 batch 的变更集：M8 的单轮做得了的照常；别的形状答 500（S1 的临时检查保证不写错的正文），S2 合并之前不发版（v0.1 在 M12 发）。

**S2**：
- 网页的整组撤销做得了 4.12 的每种形状；`TestTheRevertOfEachBatchShape`（"恢复到另一组之下"与同一毫秒的附件以 SQL 种下）、撤销的性质测试（加 `batch` 与两种定向的生成器、比之前就在回收站里的页的正文、临时的步的计数读"changeset reverted"那一行）、domain 的性质测试（以它自己的模型判定、轮数的上限在代码里）、第 71–86 条变异、MC5 (page) 通过或被抓到；M8 的撤销的测试不改而通过，钉住 M9 的形状的三处照 3.21.1 改期望。
- 迁移 00045 上下都通过；`renames`、`moves` 是全部条目的。

**S3**：
- 后台撤销 batch 的变更集：段整个在一批里，批之间没有临时名称；删除照那一刻的子树（开始时的 `page.locked` 只看要删的页）；每批 L、再按次序核对前提，不报假的冲突，批之间别人的写照冲突停下、网页写的正文留着；一组在一批里一次恢复；减半照 3.23；一段放不进一批以 `revert_too_large` 停下、网页有它的文字（中英）；"计划之后那一组被恢复又删除"照 M8 做完（`TestARevertJobOfAGroupDeletedAgainSinceThePlan`：成员 0（一组的第一个成员）在这一批里时读全组）；批里的 `canDelete` 以多出的每一部分的顶为名（`TestARevertJobsBatchPlansItsOwnNodes`）；HI5 与 M8 的后台的测试照旧；第 87–103 条变异被抓到。

**文档**（P3 的文档提交，照 P1、P2 以脚本落下、每段原文恰好出现一次才改）：
- **00 号文档**：2.1 的 35 条（`scratchpad/m9/p3/apply-00m9.py`，随这份设计的提交落下：第 12 节的 P3 一行写"进行中（设计已审查）"，第 13 节加一行）；各 Step 合并时第 12 节的 P3 一行与第 13 节的变更记录；"移交的落实"里 P3 的各行写上落实（M4/P2、M5 第 2 条、M6/P4 第 1、3 条、M8/P3、M8/P4 第 2 条、M8/P5 第 2 条；九份移交的 `status` 由 M9 收尾时改为 `done`，00 号文档 :450）。
- **v0.1 总体设计**（P3 自己的修订，随各 Step 的合并；今天的 `v0.1-design.md` 没有与 P3 无关的错，不另写 `apply-v01.py`）：
  - 4.5 第 4 条（S1）："不同的两处（新建之后的改名、移动；锁的码）写进工具的说明"改为指向 M9 总设计 4.11（规则只写在那里，审查 A2-M1）："与分开调用的不同照 M9 总设计 4.11 的那一条规则（链接跟着 batch 按节点合并的改名与移动，从 batch 之前、或写了的页最后一次写时的去向跟起）；batch 写的页在被写之前正被编辑时，在写它的那一项答 `page.locked`；这些写进工具的说明"，其余不变（与实现一致）。S1 合并时照建成的写：`update_links` 为假的节点的链接在每一页里都不改，batch 写或删除的页正被编辑时在第一个写或删除它的项答 `page.locked`，只被链接的改写碰到的页在单元里才查到（S1 审查 A-M1、B-M1）。
  - 13.1 第 1 条："页面的写入单元"一段的"M4 的单元都只有一个操作"之后加"M9 的 `batch` 是第一个多个操作的单元：参与者推到全部操作之后一次（`DeferParticipants`），同一页的正文操作合成一个版本行，前后相同的条目在单元结束时删去"（S1）；"整组撤销"一段的"单轮应用……多轮与临时名称归 M9 的 `batch`"改为 `schedule()`、三条后备（第 3 条是防御的）与轮数的上限、临时名称、先恢复再写回、写在 `Current` 上、按（页、哈希）解析（S2）；"后台的恢复与撤销"一段加段、删除照那一刻的子树、只在段之间切批、`revert_too_large`、每批的 L 与按次序的前提、一组在一批里一次恢复、一个节点不受界线、紧凑的计划（S3）；"变更集"一段加 `renames`、`moves`（S2）。
  - 13.1 第 5 条："页面一支"的"在 `FOR SHARE` 下写多页正文的按节点 id 升序"之后加 3.10 的一句（S1）。
  - 13.1 第 31 条："MCP"一段加 `batch` 的界线：项、字符串、条目（第一遍结束时）、读正文的页按大小 14 MiB（任何正文的读之前；`write` 的页只读头）、计划自己的解析 10 MiB（按不同的（页、哈希），与最后的正文共用一个等待）、页数与字节按（页、哈希）、链接、`MaxBatchRewrites` 的实测值、重新计划一次（S1）；"整组撤销"一段的步数加"含临时的步"、删去"单个变更集的 `renames`、`moves` 只看前 1,000 项条目"（S2）；"后台的恢复与撤销"一段的"单独超过的一步自成一批"改为"一个节点的步、不可分的一段在一批里，一段太大以 `revert_too_large` 停下"，"多于一页而超过时减半"改为"减半只落在段的边界上（第一段的末尾与一半之内的最后一个边界中靠后的那个）"（S3；审查 C-N16）。
  - 13.4 第 10 条：两个"等于重建"的测试加 `batch` 与 4.11 末条的性质（S1）；第 13 条：计划与单元的重做是同一组规则、调度的性质测试（Go 版的模拟，以测试自己的模型判定）与任务的步的重放（S2、S3）。
- **README**："日志"一条（`README.md:172`）的"另答前 1,000 项里有没有改名、移动"改为"另答它有没有改名、移动"（S2）；"整组撤销"一条（`:173`）加一句：batch 一类有几种操作的变更集按依赖分轮撤销，必要时把一个节点暂时改成带 `~` 与它的 id 一段的名称、或暂时移到根下，结束时都回到原来的名称与位置，这些步算进 1,500 步（S2）；"任务"一条（`:187`）加一句：撤销里必须一起做的一段（暂时的名称与它的放回、一页的恢复与写回）在同一批里，一段放不进一批时任务停下、报告写明（S3）；"工具"一条（`:210`）的"写的四个工具与历史的读见下两条；`batch` 与 `grep`……随 M9 之后的 Phase 加上"改为"写的四个工具、`batch` 与历史的读见下三条；`grep`、`search`、`index`、`backlinks` 随 M9 之后的 Phase 加上"（S1）；"写"一条之后加"**batch**"一条（S1，审查 C-N17，照"写"一条写出它的界线与码）：两个入口都有；至多 100 项写、编辑、移动、删除与一句说明（1–4,096 字节），一个变更集、全成或全不成，每项看到前面各项的结果，`base_version` 指 batch 之前、新建的页不带；字符串合计至多 6 MiB，其中正文与 `new_string` 合计至多 5 MiB；新建、删除、移动与改名的节点至多 10,000 个，要读的已有的页（编辑的）合计至多 14 MiB，跟着前面的改名、移动改写要编辑的页时解析的正文至多 10 MiB，要解析的页至多 100 页、8 MiB、20,000 条链接（只有一页时，14 MiB 与这一条不限，10 MiB 仍限），改写链接的页至多 `MaxBatchRewrites`（实测的值）个，过了答 422 `page.batch_too_large`；被拒时答出第几项（`op`，从 0 起）与那个工具的码，什么都不写；先删子孙再删祖先合成一次删除（前面那一项答 `merged_into`），不能删除这个 batch 新建、改名或移动的；与分开调用的不同照 4.11 的规则写一句；每项的 `changed` 是它的工具在那一步会答的；在网页上可以整组撤销（S2 加最后一句）；"历史"一条（`:212`）的"经 MCP 的一次移动加改名在网页上一步撤销"之后加"一个 `batch` 在网页上整组撤销"（S2）。
- **网页的一条文字**：`transfer.failure.revert_too_large`（中英，3.25，S3）。
- **M12 的移交**：[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项加：batch 的计划每次读整棵树；读的页以大小拒绝（14 MiB，约 27 MiB 的堆）；计划自己的解析至多 10 MiB；最坏的 batch 解析至多 20 MiB，正常约 5 秒、病态约 10 秒，预算被别人占着时约 7 与 12 秒；重新计划一次时约 6 与 14 秒、被占着时约 8 与 16 秒（最后一种过了 15 秒的期限，接受，3.8）；`MaxBatchRewrites` 的实测值与它的守卫在 CI 上的用时；batch 之后的搜索的第一页的实测；链接密与页的字节的曲线、很重的页与单个 `move` 的同样的风险（参与者一个页数、链接与页的字节的界线，单个 `move` 与 batch 共用，连同写出的长度与界线的提示；3.8，修正的第 2、5、6 轮）；许多次改名之间的带链接的页与正文的一遍的再展开（3.8，修正的第 2–4 轮）；00045 的补值在大的库上的用时（S1、S2 合并时各写上）。

## 8. 交给后面的 Phase

- **P4（`grep`、`search`、`index`、`backlinks`）**：
  - 内置的说明在 P3 之后剩 710（笔记本入口）与 640（账户入口）字节（`TextBytes` 的数法，换行 2 个字节）。工具的各行是两个入口共用的（`pathsAndTools`），所以起作用的是账户入口：四行（`grep`、`search`、`index`、`backlinks`）连同 `grep` 与 `search` 怎样分工（00 号文档 :154、4.13 :332）要在 640 字节之内，审查 C 的草样约 590 字节；4.13 的提醒（短的第一页、一个汉字只找标题、`building`、`incomplete`）放进工具的说明。
  - P4 之后说明离 4 KiB 只剩约 50 字节：之后在说明里加字的（P5、M10）要先缩短已有的行，`TestTheBuiltInInstructionsAreShort` 照旧守 4 KiB；
  - 3.10.1 的实测改了 `batch` 的入队（批量、带 `building`，S1 的审查记录的主持者的决定 1），`grep` 与 `search` 的说明照它写"刚写的可能还没进索引"；
  - 码的表、矩阵、工具的字符串、`tools.json` 照 P2、P3 的写法；新的整个程序测试照 6.5 取名、以 `-race` 的 `go test -list` 核对落在第二、第三份。
- **P5（其余故事的 MCP 版本与接入文档）**：接入文档写 `batch`、4.11 的与分开调用的不同（那一条规则）与说明的别名；人工清单在真实的客户端里试 `batch`（破坏性、非幂等的工具的审批提示）。
- **M10（守卫）**：一个 batch 的单元里守卫看到每一项的写，同一页的正文在它第一个正文操作的位置以最后的正文出现一次，推后的改写的追加写另经守卫；撤销的步带 `Write.Kind = revert`，其中有临时名称（带 ` ~` 与 id 的一段），模式守卫要认得它们（写进"写给后面的 M"）。S2 建成时守卫只凭名称认得临时的步：它以普通的 `OpRename` 或 `OpMove` 的一步到达守卫（经 `place`，`page/app/unit_revert.go`），`Write.Kind` 是 `revert`、临时名称在 `Changes[0].After`，步上没有它是临时的步的标记；模式 `" ~<hex>"` 也是合法的用户标题，放回一个真叫 `x ~1a2b3c4d` 的节点的撤销在守卫看来相同。M10 可以在 `Step` 上带步的种类（`RevertOpKind`）或一个 `Aside` 标记（S2 审查 B-N4，主持者的决定 10）。同一单元里先恢复、再写回的页，守卫先看到恢复的一步与它的事实（回收站里的正文的），再看到写回（S2 审查 B-N2，3.18.3）。
- **M11（冻结与回滚）**：回滚一组变更集时用 `schedule()`（按依赖的调度与后备对任何变更集都成立，轮数的上限让缺陷答 `ErrRevertUnordered` 而不打转）。
- **M12（发布）**：第 7 节的性能移交各项；`revert_too_large` 的两种来源（一段里大的恢复、撤销的撤销里写回的链接）进风险（第 10 节）；00045 的补值写进升级的说明：100 万个变更集、300 万条条目约 2 秒，5,000 万条约 30 秒，其间持着 `changesets` 的 ACCESS EXCLUSIVE 锁（goose 的一个事务，只在启动时；S2 实测与审查 B 的注记，3.20.1，[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项）。

## 9. 结果

（各 Step 合并时写：合并、偏离、量出的数——`MaxBatchRewrites` 的各档、搜索的第一页、各份的用时、变异的结果。S1 的在 9.1：`MaxBatchRewrites` 400 与碰到的链接、改写的页的字节的两个界线，搜索的第一页与它的判断，各份没有挪、没有分，变异与各轮的反向对照都被抓到。S2 的在 9.2：调度与它的上限、临时名称、一页的两份正文在最小的预算下、`renames` 与 `moves` 的迁移与它的用时，各份没有挪、没有分（第一份过线时改为挪到第三份），变异与各轮的反向对照都被抓到，C 的两个上限的变异照决定存活。）

### 9.1 S1：batch（2026-10-20，合并 `f19d0b6f`）

- 提交（从 main 的 `c661ee97` 开始，照 5.1 的次序，14 个）：第 0 步搜索的积压的量法 `1995157d`；后台减半的哨兵改名 `ErrJobBatchTooLarge` `430fab83`（单独一个提交、在新的哨兵之前）；`domain.Args` 与 P2 的规则收它、`PagePath`、`ParentChainOf`、`RenameTitle`、`RelocationOf` 挪进 domain、`TestEveryRuleNamesTheFieldsItIsGiven` `51a2e412`；batch 的错误（`ErrBatchTooLarge`、`BatchTooLarge`、`OpFailed`、`BatchChanged`）与动作 `page.batch` `5e1f9d0d`；`CheckBatch`、`PlanBatch` 的两遍、`Admit`、`CheckReads`、`CheckUnit` `f7dd1ffa`；linking 的参与者跟着 batch 的单元（没写的页照索引、写了的页照它最后的正文的事实与它自己的改动）、`PathBefore` 从根撤回、`Recased` 是集合、`resolutions` 的 `gone`、`BatchLinks` `d7c25560`；单元推后参与者、按节点合并改动、`DropUnchanged`、`PageDraft.ID`、解析的会话、`LinePaths`、`PageState.Hash` `e4b5ea47`；S1 的临时检查（先写正文再删除的页的撤销答 `ErrRevertUnordered`，钉住它的一行同一提交改期望）`3ff6d629`；用例 `ApplyBatch` `f5593574`；MCP 的错误的位置 `ToolError.At` `e5cead4b`；工具 `batch` 与它的 `tools.json` 的两个条目、说明的一行、码的表、`declaredCodes`、矩阵的一行与合法参数 `8edeadbd`；整个程序的测试 `1d78a44d`；`MaxBatchRewrites` 的基准、取值与守卫 `19259b00`；MC5 (MCP) 与 4.1 的 fixture `a3f92485`。每个提交单独在一个工作树里 build、vet 与它碰到的包的 `go test -short` 都通过。组合根的份没有改（下面"组合根的各份"一条）。修正的七轮另有 42 个（下面各轮），一共 56 个，合并 `f19d0b6f`（第二个父提交 `dd1e8ffc`）。
- 与设计不同之处（实施时定下的 27 条，都是最小的改动，审查 C 判过；已改在各节，3.28 列出）：(1) `OpKind` 的值叫 `OpKindWrite` 等（同一个包里写的守卫的 `Operation` 已有 `OpMove`、`OpDelete`）；(2) `PlannedStep.To` 是 `Destination`（`Placement` 是回收站的条目的）；(3) `LinkRewriter` 多一个 `Indexed`（第一个正文操作的"之前"取自索引），它的 `Rewrite` 收"之前"所在的那一项（`since`）；(4) 计划分在 `batch.go`、`batch_tree.go`、`batch_plan.go`、`batch_contents.go`；(5) P2 的 `TestRelocateRefuses` 照旧留着，与 domain 的 `TestRelocationOfIsMovesOrder` 并存（只核对"调了 `RelocationOf`"要在单元里开一个口子）；(6) `DropUnchanged` 留下写了正文的节点的改动，也留下被一次移动带着走的节点的改动（那次移动还在时）；(7) `ApplyBatch` 另收锁的持有者与时钟（3.4.5 的预检），与 `maxRewrites`（`page.Deps.MaxBatchRewrites`，0 是 domain 的；只有基准抬高它，生产不设）；(8) `OpDone.Created` 是 `[]Placed`（同 P2 的 `write` 答出的）；(9) 计划读树之后、读头之前页被删，在它第一个正文操作答 `page.not_found`（修正的第 1 轮去掉：也重新计划一次，审查 A-M2）；(10) 第二次的单元里正文操作的路径上没有页时答 `BatchChanged`；(11) `LinksOn` 没有 `Gone` 一栏，删掉的页从计划的树的视图读；(12) `BatchLinks.Rewrite` 不用 `LinksMoved.Parents`（修正的第 1 轮删去这一栏，审查 B-N1）；(13) `DecidedAll` 的界线是 `func(links int) error`，以便用例说出条数；(14) "batch applied" 每次成功都记，没改动的也记；(15) `BatchTooLarge` 的文字是"n what, at most max: hint."；(16) 第 0 步在造出来的语料上量（M8/P5 的语料已删：306,000 页，活着的 299,911 页，200 本笔记本，2,418 MB），`NERVE_SEARCH_BACKLOG` 收一个列表，一次建索引量四档；(17) 基准经 MCP 的工具在进程里调用例（page 模块只把用例交给它的 MCP 适配器，为测试加一个取用例的口子是生产代码），`benchAuth` 把 "pat" 当作个人访问令牌（MCP 不收会话的令牌）；(18) 3.8.2 的 batch（编辑 P1、改名 T、编辑 P2）里 P1 在改名之前写过，推后的改写把它也数进去（3.7.2），所以 P1 也链到 T0：一档 n 是 n-1 个 R 页加 P1，守卫种 `MaxBatchRewrites` 个 R 页（加 P1 过界一页：ops[1] 答 422，什么都不写），删掉一页之后界线上的 batch 恰好改写界线那么多页（照 3.8.2 的"R = `MaxBatchRewrites`"会被拒；修正的第 1 轮写了的页算 2 之后，是 399 个 R 页与 P1）；(19) 4.11 的性质（P1）也允许"经被 batch 的写拿掉的别名解析到的链接"重新解析，同新建与删除；(20) 计划的解析的 2 秒在竞态检测下乘 6，`TestTheMCPBatchIsRefusedWhole` 的应用用生产的期限（请求 15 秒、写 60 秒；测试的是 4 秒与 10 秒），它的 10 MiB 一项的页经 SQL 做出、由重建建索引（第一次 `make test-go` 在那里失败：做出 4 MiB 那页的 batch 在竞态检测下与别的份同时跑，过了测试的 4 秒；修好之后单独量，拒绝本身不带竞态检测 0.37 秒、带 3.7–3.8 秒，CI 慢约 1.7 倍，也会过 4 秒）；(21) MC5 (MCP) 末尾用 `expectNoAsideNames`（S2 的 MC5 (page) 之前 knip 会报没用到的导出）；(22) `fixtures/mcp.ts` 没改（4.1 写明不用改，3.1 的表列了它）；(23) 矩阵的 batch 一行以 `client: "mcp"` 读 `log`（种下的改名比种下晚一秒，一格可能就在那一秒之内，第一次运行遇到）；(24) 第 15、16 条变异照命名的测试看得见的写法另做一种（15：`Admit` 不数单元的页数与字节，解析之后由 `CheckUnit` 不带 `op` 拒绝；16：每个正文操作写它自己的正文）；第一种写法（用例里先解析后 `Admit`；合成的写落在最后一个正文操作）答的与真的库上的测试看的相同，两种都在表里；(25) `TestBatchPlanContents` 加一页过 10 MiB 一个字节（原来"多一个字节"的那行有两页，单元的界线先拒，第 15 条的另一种写法与第 53 条都没被抓到）；(26) `TestTheLinksOfABatchAreThoseOfSeparateCalls` 的第 14 种加一行"之后再改名"（照设计的第 14 种没有移动，写了的页不被跟随，第 49 条看不出来；加了改名就抓到）；(27) 推后的改写**先改写 batch 写了的页**、再改写索引里的页（量最坏的 batch 时发现）：一页 5 MiB 加 399 个满负载的页答 `server_busy`：索引里的页每改写一页，它的事实的份额（满负载的页约 8.6 KB，399 页约 3.4 MB）留到单元结束，写了的那页 5 MiB 的读回只剩 8 MiB 里的 4.98 MiB；3.7.2 (a)"batch 的份额放还之后，预算最小的 5 MiB 下一页 5 MiB 的读回也取得到"照这个次序才成立；`TestABatchRewritesThePagesItWroteFirst`（预算 44 KiB，写了的页 40 KiB，索引里两页各 30 KiB；旧的次序答 `server_busy`）钉住它，把两段对调的变异被它抓到。
- 实测：
  - 搜索（第 0 步，3.10.1）：为空而带游标的第一页（每种可见范围 45 个，三个阶段相同）：n = 0、100、1,200、2,200 时两种可见范围（M8/P5 的 92 个，这里 90 个）是 5、5、10、12，加上"一本笔记本"是 6、7、27、29；p99 的中位数（无统计 / 分析过 / 通用计划）n = 0 是 19.8/21.9/36.0 毫秒，n = 2,200 是 51.4/61.2/64.5 毫秒。照 3.10.1 的条件：n = 100 多 0 个（少于 3），n = 2,200 多 7 个（不多于 M8/P5 的 10）：**不改**，照常准确入队。`MaxBatchRewrites` 是 400（小于 1,000），最坏的积压约 1,600，在量过的 2,200 之内，不再量。
  - `MaxBatchRewrites`（3.8.2）：满负载的页（84 KB、200 条链接），三次的中位数，单元的用时：100 页 0.86 秒、200 页 1.90、250 页 2.24、300 页 2.70、400 页 3.56、500 页 4.44；1,000 页答 `server_busy`（改写的页的事实的份额合计过了 8 MiB 的预算）。照 M8 的规则取 **400**。守卫断言 11 秒（3.56 秒的三倍，12 秒之内）；单独跑两次单元 3.334 与 3.300 秒、测试 6.47 与 6.42 秒；按守卫那一份的比 2.4，CI 上单元约 8 秒、那一行约 15.5 秒。
  - 最坏的 batch（3.8；生产的请求期限 15 秒、预算 8 MiB、等待 2 秒；经 MCP 调用）：一页 5 MiB 的普通正文，改名、编辑、改名（计划 10 MiB、最后的 5 MiB、读回 5 MiB，共 20 MiB）调用 1.31–1.38 秒、单元 0.44–0.47 秒；同样的病态正文（1 MiB 时最慢的"an e-mail run to a domain then '-' a_"，5 MiB 解析 1.65 秒）5.9–7.0 秒、单元 1.5–1.8 秒；两页各 4 MiB（链接少）：最后的 8 MiB 加读回 8 MiB，1.10–1.18 秒；读 14 MiB 的三页 51–53 毫秒；一页 5 MiB 的普通正文加 399 个满负载的页（改写 400 页，在界线上）4.43–4.52 秒、单元 3.44–3.64 秒（3.8 估计约 5 秒）；病态正文加 399 页 9.27 秒、单元 4.80 秒（3.8 估计约 10 秒）。没量：预算被别人占着（3.8 加一个 2 秒的等待）、重新计划一次。
  - 内置的说明：batch 一行 425 字节（`TextBytes`，UTF-8 是 424），笔记本入口 3,386、账户入口 3,456 字节（UTF-8 3,301 与 3,370），给 P4 留 710 与 640 字节。错误的答复最坏是删除的 25 个锁，38,568 字节（48 KiB 之内）。
- 反向对照：125 行（6.3 的第 1–70 条，有的在几层各一行；第 15、16 条的另一种写法；3.27 的 S1 各行的反向对照；偏差 27 的一个对照），119 个对它命名的测试失败；6.3 的第 1–70 条每条都有一行失败。通过的 6 个：`m15-db`、`m15d-db`、`m16-db`（第 15、16 条实施者先写的那种，真的库上的测试答的相同；设计的写法 `m15u-*`、`m16e-*` 都被抓到，先写的那种被单元测试抓到；偏差 24）；`m49-written-links-all-compared`（linking/app 的测试，6.3 没有给第 49 条指这些测试）与 `m49-db`（照设计的第 14 种：写了的页之后没有移动，不被跟随，变了的比较走不到；这是一个发现：第 14 种加一次之后的改名，`m49-db2` 被它抓到，偏差 26）；`rc-411-written-pages-compared`（拿第 48 条当"性质比较 batch 写了的页"的探针：性质的 batch 从不改名再改回，第 48 条改的正是这个形状；第 24、25 条把每个写了的页的跟随弄坏，性质测试都失败，第 48 条被第 11–13 种抓到）。别的发现都改在它们所属的提交里：`TestBatchPlanContents` 原来没有单独一页过计划的 10 MiB，第 53 条与第 15 条的另一种写法都过了，加了那一行（偏差 25）；量界线时发现推后的改写的次序（偏差 27），旧次序的对照 `rc-written-pages-last` 被新测试抓到。S1 的检查在 batch 缺了时都失败：工具清单的契约（`rc-no-batch-tool`，`TestTheToolListsAreTheContract`）、矩阵（`m40`，`TestThePermissionMatrixCoversEveryTool`）、码的表（`rc-no-codes-row`，`TestEveryMCPToolHasItsCodes`）、说明（`m43`，`TestTheBuiltInInstructionsNameEveryTool`）、合法参数（`m41`，`TestToolStringsDoNotAnswerInternalErrors`）。第一次 `make test-go` 失败、`TestTheMCPBatchIsRefusedWhole` 修好之后，它的 10 个变异在修好的测试上再跑一次，结果同前：第 53 条到了新的 15 秒的期限答 `internal_error`，仍失败；`m15-db`、`m15d-db` 照旧通过（偏差 24）。
- 门禁（本机，`a3f92485`）：`make lint-go gen-check knip` 与 `make lint-web` 通过（golangci-lint 两个模块都是 0 个问题，生成物不变，工作树干净）；经 `heavy.sh`：`make test-go` 707 秒（11:47）通过（竞态下组合根三份与别的包同时跑，三份 301.5、173.5、193.4 秒（第二、三份照单独跑时的大小对应），成本的 markdown 一行 43.6 秒、page/domain 一行 27.1 秒，守卫的各行 113.8、3.2、19.3、24.2、29.0、15.5、22.9 秒与 batch 的一行 7.1 秒（单元 3.246 秒、测试 6.54 秒）），`make test-go-short` 52 秒，`make e2e` 259 个（P2 之后的 258 个加 MC5 (MCP)，它 210 毫秒；Playwright 37.0 秒）；14 个提交各自 build、vet 与碰到的包的 short 测试都通过。第一次 `make test-go`（`af26bab7`）在 `TestTheMCPBatchIsRefusedWhole` 的 10 MiB 一项失败：做出 4 MiB 那页的 batch 在竞态检测下与别的份同时跑，过了测试的 4 秒（`internal_error: the call ran out of time`）；照偏差 20 修在 `1d78a44d` 里（分支重放，`git diff af26bab7 a3f92485` 只有那一个文件）。那一次的别的门禁都通过：`make test-go-short` 52 秒，`make e2e` 259 个，第三份单独 142.96 秒。
- 组合根的各份（6.5 的规则）：本机单独两次（go test 自己的时间，`a3f92485`）：第一份 256.6、244.5 秒，平均 250.6；第二份 124.9、138.0，平均 131.4；第三份 142.0、148.5，平均 145.3（P2 的平均 253.7、125.9、123.1：第三份多 22 秒，6.5 估计多 43–52 秒；第二份多 5.5 秒，估计约 5 秒）。照 6.5 以 `-race` 列出：三份 168、106、111 个测试；新的七个在第三份，`TestTheBatchIsPlannedAgainAfterAWrite`、`TestTheBatchsRewriteMeetsAnOpening` 在第二份，守卫不在任何一份（`//go:build !race`）。乘 6.5 的"现在"的比（1.67、2.16、1.66）估计 CI 上约 6:59、4:44、4:01，都在 7:30 之下：界线不动、不分、不提高 `-timeout`。CI 的测试一步两次（run 39337680280，`19259b00`；run 39339768246，`a3f92485`）：第一份 5:01、7:16，平均 6:08.5；第二份 5:33、5:23，平均 5:28；第三份 4:41、4:40，平均 4:40.5：都在 7:30 之下；CI 与本机之比 1.47、2.50、1.93。第一份没有 S1 的测试，同样的 Go 代码两次差 2:15（S2 合并时 6:58），一次接近线、平均不过线，6.5 的办法（第二份接过 `TestEvery[M-R]…`）留到平均过线时。守卫的新的一行本机约 7 秒（单元 3.2–3.3 秒），乘 2.4 约 17 秒（6.5 估计 19–24 秒）。
- CI（作业，括号里是测试一步）：run 39339768246（`a3f92485`）全绿：竞态 5:37（5:18）、组合根一 7:33（7:16）、二 5:46（5:23）、三 5:04（4:40）、成本 9:56（测试 6:54）、守卫 11:47（11:31）、short 3:09（2:50）、web 3:18（2:44）、e2e 3:38（E2E 一步 2:50；4.4 估计 e2e 约多 15 秒，今天 3:35–4:17：MC5 (MCP) 只约 0.2 秒）、镜像 1:21。run 39337680280（`19259b00`，还没有 MC5）全绿：竞态 8:03（7:44）、组合根一 5:19（5:01）、二 5:51（5:33）、三 5:02（4:41）、成本 10:03（测试 6:54）、守卫 11:10（10:47）、short 3:26（3:05）、web 3:07（2:38）、e2e 4:24（E2E 一步 3:32）、镜像 1:16。成本的作业两次约 10 分钟（S2 合并时 8:53，静态检查 1:33 对这里的 1:54 与 1:44，测试一步 6:17 对 6:54），到了它 20 分钟的时限的一半，是 ci.yml"时限约为作业的两倍"的边上；S1 没有加成本的行，它改了代码的那一行（page/domain 的 `TestReplaceStaysWithinItsTime` 与 glob）本机 27.1 秒，P2 是 27.0 秒；这次不改时限。
- 实施时留给审查与主持者的，都在下面的修正里定了：第 0 步的保留意见（积压落在最新的页之中时会改判断：修正的第 1 轮照这样重量，改为批量入队）；推后的改写的每页的事实的份额留到单元结束（修正的第 5、6 轮的字节的界线与精确的检查，单个 `move` 交 M12）；没量的预算被别人占着、重新计划一次时的最坏的 batch（审查 A 量了，3.8）；成本那一份的时限（主持者的决定：等下一个往里加的 Step）。合并时要改的设计文字都已改：3.8.2 的守卫（偏差 18）、3.14.3 的第 14 种与 6.3 的第 49 条（偏差 26）、3.7.2 (a) 的预算的说法（偏差 27）、6.3 的第 15、16 条（偏差 24）、3.1 的表（偏差 22）。CI 的作业日志要认证，每个测试在 CI 上的用时没有记下，只有每一步的时间。
- **修正的第 1 轮**（2026-10-20，分支 `m9-p3-s1` 的 `754c9790`；三份审查与主持者的决定，`fix-brief-1.md`）：从 `a3f92485` 起 19 个提交，每项一个，小问题按文件合并，每个提交单独 build、vet 与碰到的包的 short 测试都通过：
  - 重要的：B-I1 `55860ee9`（`PathBefore` 每条链接重建一次改动的表：一次改写只建一次（`Befores`），目标去重；改写的页数在读树之前数）；A-I1 = B-M2 `9583ac60`（`write` 与它照之前的改名、移动改写之后的正文比字节，碰得到的页读整页、算进 14 MiB）；A-I2 `f418cedd`（移动带着走的节点合计至多 10,000）；C-I1 `93b68f36`（重做的四种比较各有测试）；C-I2 `e927d227`（第 0 步的量法：积压是最新的页）与 `fb972a43`（重量之后照规则改为批量入队，答复的 `building`）。
  - 次要的：B-M1 `69f7773c`（`update_links` 为假的节点的链接在每一页里都不改）；B-M3 `ac97bf8b`（写了的页一共读一次树）与 `754c9790`（写了的页在 `MaxBatchRewrites` 里算 2 页）；A-M1 `b54e4cbd`（锁的预检加上删除的子树，按项的次序）；A-M2 `fb2d9709`（两次读之间页被删也重新计划一次：偏差 9 没有了）；A-M3 `e9aec859`（`edit` 留下相同的字节答 `changed: false`）；C-M1 `29966fa3`、C-M2 `b1835721`（测试）。
  - 小问题：B-N1 `4ef80e3f`（删掉没人读的 `LinksMoved.Parents` 与 `BatchPlan.ParentsAt`：偏差 12 没有了）；B-N2 `53c8e33a`（一棵树的读，计划里一棵树一份）；B-N3 与 C-N1 `c8387190`；C-N2 `30e10ec5`；C-N3 `1ebe1ec4`。
  - 改了的判断与偏差：上面"实测"的搜索一条作废：第 0 步让积压是最新的页（同 batch 写的）重量，两种可见范围的 90 个第一页里为空而带游标的 n = 0、100、1,600、2,200 时是 5、5、21、21，n = 2,200 多 16 个，过了 10 个，**改**为批量入队（3.10.1）；偏差 18 的守卫改为 399 个 R 页与 P1（P1 算 2）；偏差 9、12 没有了。
  - 实测：B 的 400 页的单个移动 main 4.64、5.09 秒，修好之后 4.52、4.64 秒（S1 的是 15 秒的期限到）；REST 的差分测试（4,000 个随机的世界）在最后的提交上与 main 逐字节相同；写了的页是索引里的页的 1.4–1.9 倍，进位为 2，界线上的混合单元 4.36 秒；守卫的单元 3.26、3.32 秒；一个被移动的 1 万个节点的文件夹之后的 33 个小页的 `edit`：调用从 698–798 毫秒到 267–274 毫秒、分配从 1,243 MiB 到 235 MiB；最坏的 batch 同 S1（界线上的一页 5 MiB 加 398 页 4.57 秒，带着走 10,000 个节点的两种 113 与 210 毫秒）。
  - 反向对照：C 的 `om15a`–`om15d`、`om4` 与实施者的各条被单元测试抓到，`om1`、`om2`、`om9`、A 的 1a–1e、带着走的界线、冻住的节点的过滤、反过来的第 45 条与写了的页的权重在真的库上也被抓到；`om15e`、`om3` 等价（C 的判断）。
  - 门禁：本机，`754c9790`：`make lint-go gen-check knip` 与 `make lint-web` 通过（golangci-lint 两个模块都是 0 个问题，生成物不变，工作树干净；第一次在 `3f82d009` 上报了测试导出的 `TreeOf` 是全局变量，改成函数并进 B-I1 的提交，之后的 17 个提交重放，`git diff` 只差那个文件）；经 `heavy.sh`：`make test-go` 711 秒通过（组合根三份 160.1、177.4、297.5 秒，batch 的守卫一行 7.0 秒、单元 3.263 秒），`make test-go-short` 51 秒，`make e2e` 259 个（Playwright 36.4 秒），第三份单独 151.8 秒（S1 的平均 145.25 秒，多 6.6 秒：这一轮新加的组合根的测试都在这一份；乘 1.93 约多 13 秒，在 7:30 之下，份不动）；19 个提交各自 build、vet 与碰到的包的 short 测试都通过。
  - CI：run 39362567732（`754c9790`）全绿，共 13:03（作业，括号里是测试一步）：竞态 8:30（8:10）、组合根一 7:30（7:08）、二 3:58（3:41）、三 5:13（4:50）、成本 8:57（静态检查 1:35、测试 6:09）、守卫 9:03（8:42）、short 2:02（1:46）、web 3:49（3:15）、e2e 3:55（E2E 一步 3:07）、镜像 1:28。第三份（这一轮的测试都在里面）4:50，S1 的两次是 4:41、4:40，多 10 秒，同估计；第一份没有 S1 与这一轮的测试，7:08（S1 的两次 5:01、7:16），三次的平均 6:28，在 7:30 之下，份不动。
  - 留给主持者：每页 100 条碰到的链接时，只有索引里的页的 400 页单元 4.46–5.17 秒，过了约 4 秒：按页数的界线管不到链接的多少（审查 B 的另一个改法是数链接）；B-N3 的探针：性质的六个种子里经保留的别名的链接一条也没有（经拿掉的别名的 2 条），放宽的那一条以前没有藏住什么，新的检查也没有走到；量的时候负责人另一个会话的测试同在本机跑（负载到 11–13），受影响的量都重量过。
- **修正的第 2 轮**（2026-10-20，分支 `m9-p3-s1` 的 `09e75680`；三份复核与主持者的决定，`fix-brief-2.md`）：从 `754c9790` 起 10 个提交，每项一个，另有一个测试是这一轮的变异要的；每个提交单独 build、vet 与碰到的包的 short 测试都通过：
  - 次要的：B1-M1 `ad0c509e`（batch 在一个节点最后一次 `update_links` 为真的改名、移动之后新建在它下面的节点，不进它的子树的触及：新建只重新解析）；B1-M2 `c2249e9d`（写了的页只在它自己的改动碰到它的链接时才数、才算 2 页）；B1-M3 `ea4b5865`（改写的链接另有界线 `MaxBatchRewriteLinks` = 120,000，照 M8 的规则以链接密的一档实测）；A 的 N1 `da73406d`（没有链接的页不展开子树，展开的结果按树留着）；A 的 N2 `d68c3245`（一个文件夹来回移动的测试）；A 的 N3 `f05301d7`（读的界线的 422 说为什么整页读）。
  - 小问题：A 的 N4 `040a51bf`（带着走的界线在那次移动自己的检查之后）；B1-N1 `c38eddc9`（冻住的节点的改名、移动交给"之前"的路径：修了，不写进 3.14.4）；B1-N2 `01fe6ef5`（d6、d6w 与数里删除了的页的测试）；`09e75680`（这一轮的变异找到的：写了的页里通向删除了的页的链接也不数，测试）。
  - 实测：链接密的一档（400 个索引里的页、每条链接约 18 字节、一个 1 万个节点的文件夹单独被移动，三次的中位数）：每页 100 条 2.14 秒、250 条 3.06 与 3.09 秒、300 条 3.44 与 3.45 秒、375 条 4.28 与 4.25 秒、500 条 4.74 与 4.82 秒、750 条 6.75 秒、1,000 条 8.30 秒、1,500 条 11.47 秒，2,000 条在 15 秒的期限的边上（一次到了期限，一次 14.97 秒），2,500 条到了期限；约 4 秒之内最大的一档是 300 条，所以 120,000；界线上改写 120,000 条的单元 3.51 秒。A 的形状（50 次改名一个 10 万个节点的文件夹，每次之后 `write` 或 `edit` 一个没有链接的页；与 S1 交替各两次）：`write` 0.80、0.98 秒、分配 0.93 GB（第 1 轮之后 A 量得 3.27、4.19 秒、15.2 GB；S1 0.81、0.84 秒）；`edit` 0.91、0.88 秒、0.93 GB（S1 4.06、4.03 秒、16.7 GB）。B 的 w1 收下（调用 3.79 秒，改写 250 页；第 1 轮之后在 ops[99] 被拒）；B 的链接密的行过了 120,000 条的在 0.36–2.11 秒里答 422（第 1 轮之后 500 条 4.15 秒、1,000 条 7.71 秒、2,000 条 14.27 秒做完，2,500 条到了期限）。
  - 反向对照：这一轮的 24 个单元测试的变异（B1-M1 的排除去掉、不按节点、窗口错；写了的页照旧都数；链接的界线去掉、写了的页的链接不数、用例不交 `MaxLinks`；早返回去掉、触及不留；A 的每个文件夹只算一次；带着走的界线在 `RelocationOf` 之前；提示总是 "split the batch"；冻住的节点的改动不交、原来的父节点只取跟随的；数里不看删除了的页）都被抓到，10 个数据库的（排除去掉：K1r、K1r2 等；组合根不映射 `MaxLinks`、文字不说链接；A 的变异；计划的 `Frozen` 不映射：d6、d6w；冻住的节点的改动与它们原来的父节点不交：d5、d5r、d5k）也都被抓到；写了的页的数与链接不看删除了的页的两个变异以前什么测试都过，加了 `09e75680` 之后被抓到。
  - 门禁：本机，`09e75680`：`make lint-go gen-check knip` 与 `make lint-web` 通过（golangci-lint 两个模块都是 0 个问题，生成物不变，工作树干净）；经 `heavy.sh`：`make test-go` 734 秒通过（组合根三份 166.3、184.4、293.8 秒，batch 的守卫一行 20.2 秒：守卫 6.61 秒、单元 3.035 秒，链接的界线 13.22 秒、单元 3,500 毫秒），`make test-go-short` 55 秒（87 个包），`make e2e` 259 个（Playwright 46.6 秒），第三份单独 160.7 秒（第 1 轮之后 151.8 秒；这一轮新加的组合根的情形都在这一份，乘 1.93 约多 17 秒，在 7:30 之下，份不动；第一份没有新的测试：新的成本测试是 `!race`，只在守卫那一行）；10 个提交各自 build、vet 与碰到的包的 short 测试都通过。
  - CI：run 39380278406（`09e75680`）全绿，共 15:30（作业，括号里是测试一步）：竞态 7:53（7:34）、组合根一 6:22（6:06）、二 5:49（5:29）、三 5:11（4:54）、成本 9:24（静态检查 1:42、测试 6:34）、守卫 11:23（11:02）、short 2:33（2:10）、web 4:10（3:37）、e2e 4:02（E2E 一步 3:10）、镜像 1:31。第三份（这一轮新加的组合根的情形都在里面）4:54，第 1 轮之后 4:50；第一份没有这一轮的测试，6:06，最近三次的平均 6:50，在 7:30 之下，份不动；守卫的测试一步 11:02，在它以前几次的范围里（S1 的 10:47、11:31，第 1 轮的 8:42），batch 的那一行多了约 32 秒，作业在 25 分钟的一半之下。
  - 留给主持者的（修正的第 3 轮定了，下一条）：计划的读按树留着（审查 A 的 N1 照决定做的）：50 次改名一个 10 万个节点的文件夹、每次之后改一个带链接的页，每棵树的子树（第 1 轮）与触及（这一轮）都留到计划结束，堆的峰值 +1.1–1.45 GB（第 1 轮之后 +0.7 GB，S1 +0.27 GB），时间与分配同第 1 轮（`write` 3.1–3.4 秒、15.2 GB，仍是对 S1 的 0.8 秒、0.93 GB 的退步：带链接的页照样每棵树展开一次；早返回只管没有链接的页）；可以把触及建成它的大小、过了一棵树就丢它的读，或照审查 A 的另一个改法按链接沿计划的树往上找被改名、移动的节点、不展开；400 页约 76 KB、各 100 条碰到的链接（40,000 条，在链接的界线之内）单元 4.2–5.2 秒，过了约 4 秒（第 1 轮留下的那一条：链接的界线不数页的字节）；单个 `move` 的同样的风险留给 M12（3.8）。
- **修正的第 3 轮**（2026-10-20，分支 `m9-p3-s1` 的 `8a7ae05c`；主持者对第 2 轮留下的两项的决定，`fix-brief-3.md`）：一个提交 `8a7ae05c`，单独 build、vet 与碰到的两个包的 short 测试都通过。
  - 改动：计划一次只留一棵树的读，过了一棵树就丢它的子树与触及，再问时重新读；两个改树的项之间的正文操作照旧共用一份（审查 B-N2）；linking 的子树一次只留最后展开的一个触及；参与者把写了的页按它们自己的改动排好再碰，同样的改动的页挨着、共用一个触及（3.7.2、3.7.4）。只改计划时 `edit` 的一行仍是 +621、+628 MiB：参与者为写了的页的每一组自己的改动留一个触及，50 页各有一组。
  - 实测（50 次改名一个 10 万个节点的文件夹，每次之后改一个带链接的页；与第 2 轮之后、S1 交替）：堆的峰值 +248–268 MiB（第 2 轮之后 +1,092–1,446 MiB，S1 +230–280 MiB）；审查 A 的形状（页没有链接）+162–253 MiB，同第 2 轮与 S1；分配不变。
  - 用时的代价三项（3.8）：
    - 留着的少了，回收器跑得更勤：链接通向外面的 `write` 一行 3.45–3.74 秒，第 2 轮之后同时量的 3.02–3.35 秒；
    - `write` 的链接以路径通向那个文件夹时，改名改写它，读的检查碰到它；正文的一遍再展开一次读的检查展开过的树：6.83、7.08 秒、31.0 GB，第 2 轮之后 3.76、3.77 秒、16.7 GB；
    - 一棵树上两种改动交替的 36 个 `edit` 每次重新建触及：1.06 秒，第 2 轮之后 0.74 秒；不交替时相同。
  - 测试（两个都数读与展开，不看时间与内存）：
    - `TestABatchsPlanHoldsOneTreesReads`：读的检查与正文的一遍各按项的次序问两次移动之后的树，每遍每棵树给新的一份，两次移动之间的项共用一份；
    - `TestABatchsRewritesHoldOneReach`：一棵树上问过别的改动之后再问的触及重新展开，子树只读一次；参与者的写了的页里同样的改动的共用一个，夹在它们中间的另一组改动不让它们各展开一次。
  - 变异：4 个（所有的树共用一份读、每棵树的读都留着、每个触及都留着、写了的页按 id 碰）都被抓到。
  - 门禁：本机，`8a7ae05c`：`make lint-go gen-check knip` 通过（golangci-lint 两个模块都是 0 个问题，生成物不变，工作树干净）；经 `heavy.sh`：`make test-go` 715 秒通过（组合根三份 158.6、185.6、294.2 秒；batch 的守卫一行 20.3 秒：守卫 6.24 秒、单元 3.078 秒，链接的界线 13.43 秒、单元 3,526 毫秒）；这一轮没有新的组合根的测试，份不动。
  - CI：run 39389664729（`8a7ae05c`）没有全绿：组合根二的测试一步在 `TestRevertingAndPurgingTheChangeset/C's_row_held`（M8 的清理的交错测试，这一轮的代码不在它的路上）失败，e2e 与镜像因此跳过；其余七个作业通过（作业，括号里是测试一步）：竞态 7:26（7:03）、组合根一 7:35（7:17）、三 5:10（4:52）、成本 7:43（静态检查 1:20、测试 5:29）、守卫 11:50（11:27）、short 3:24（3:06）、web 4:13（3:41）。那个测试在本机的 `make test-go` 里与另外带 `-race` 连跑的 20 次都通过。原因（修正的第 4 轮不靠时间复现了）：测试的应用自己的清理任务启动时跑一次（`RunOnStart`），它最后的 `PurgeTrimmedChangesets` 不看时间，在测试的第二次清理之后、读之前跑时，把已标记、已空的 C 整个删了。测试的修正与之后的 CI 见下一条。
  - 主持者的决定：
    - 上面的改动；
    - 带链接的页在许多次改名之间的 `write` 的用时 v0.1 接受，连同两个改法留给 M12（3.8）；
    - 很重的页（400 页约 76 KB、各 100 条碰到的链接，单元 4.2–5.2 秒）照修正的第 1 轮的决定，连同链接密的曲线留给 M12（3.8）。
  - 留给主持者的（修正的第 4 轮定了，v0.1 接受）：正文的一遍再展开读的检查展开过的树（上面的 6.83、7.08 秒）。
- **修正的第 4 轮**（2026-10-20，分支 `m9-p3-s1` 的 `dfbac1b0`；主持者的决定，`fix-brief-4.md`）：一个提交，只改测试。
  - **原因**：第 3 轮的 CI 红在 `TestRevertingAndPurgingTheChangeset/C's_row_held`（M8/P3A 的交错测试）。
    - 被测的服务在它的任务开始时清理一次，时刻由 River 选，本机在应用开始之后 2.2–3.6 秒做完。
    - 清理的最后一条语句 `PurgeTrimmedChangesets` 不看时间，删去被标记、条目与版本都已删去的变更集。
    - 这个测试自己的清理只删条目与版本，正好留下这样的 C。启动时的清理落在它的清理与检查之间，就把 C 整个删了：没有条目，也没有行与标记。
  - **探针**（不提交）在选定的位置跑启动时的清理做的事，不靠时间：
    - 第二次清理之后跑任务的整个清理，或只跑 `PurgeTrimmedChangesets`：检查都照 CI 的话失败，C 的行没了；
    - 不跑 `changesets` 那一个清理者，或在第二次清理之前跑，或不跑：检查都通过；
    - "the purge first" 有同样的窗口。
  - **修正**：这两种情形在它们的步骤之前等启动时的清理做完（`awaitJob`，同 `transferTeam` 与清理自己的测试），它的下一次在一小时之后。没改产品的代码。带 `-race` 跑 20 次：40 个情形里都只有一次启动时的清理，在情形的第一次清理之前 45–318 毫秒做完。
  - **别的清理与回收站的测试**：
    - 自己清理的十个测试，在它们每次清理之后立即跑 `PurgeTrimmedChangesets`；把回收站的项移到过去的五个测试，在移动之后立即跑任务的整个清理（照各自的保留期）。只有这两种情形失败。
    - 别的测试或者没什么可删，或者删了已空的变更集、拿走了移过保留期的项，都照样通过。
    - 不清理、也不把行移过保留期的测试碰不到它：新的库里它找不到早于保留期的行，只有清理才标记变更集。
  - **门禁**：`make lint-go gen-check knip` 通过；经 `heavy.sh`：这个测试带 `-race` 跑 20 次都通过（128 秒），组合根的三份竞态依次 550 秒通过（252.4、133.6、161.2 秒）。
  - **CI**：run 39395691055（`dfbac1b0`）全绿，共 16:20（作业，括号里是测试一步）：竞态 8:37（8:12）、组合根一 7:05（6:43）、二 5:35（5:08）、三 5:12（4:53）、成本 9:47（静态检查 1:47、测试 6:45）、守卫 11:53（11:32）、short 3:20（2:56）、web 4:08（3:35）、e2e 4:22（E2E 一步 3:36）、镜像 1:17。第二份（这个测试与它新的等待在里面）5:08，第 2 轮 5:29；第一份最近三次的平均 6:42，在 7:30 之下，份不动。
  - **主持者的决定**（第 3 轮留下的）：正文的一遍再展开读的检查展开过的树 v0.1 接受。
    - 那一行是链接以路径通向那个文件夹的 `write`：6.83–7.08 秒、31.0 GB；第 2 轮是 3.76–3.77 秒、16.7 GB。
    - 理由：在期限之内，batch 有界，第 3 轮省下的内存在并发时更要紧。
    - 写进 M12 的移交（3.8）；不做把碰到的链接交给正文的一遍。
- **修正的第 5 轮**（2026-10-20，分支 `m9-p3-s1` 的 `c8e605a5`；三份第二次复核的发现与主持者的决定，`fix-brief-5.md`）：五个提交，一项一个，各自 build、vet 与碰到的包的 short 测试都通过。
  - **页的字节的界线**（复核 B 的 B2-M1，`2f1b7b00`）：
    - **以前**：400 个索引里的页、各一条碰到的链接，在页数与链接两个界线之内，每页 256 KiB 时 8.0 秒后答 `server_busy`，1 MiB 时 4.1 秒后。每个改写的页留着它的事实在解析预算里的份额（约它的字节的十分之一）到单元结束，改写的解析立即取预算，取不到就答 503，重试也一样。
    - **现在**：推后的参与者在数过链接之后、读树之前数这些页的字节：页的头里的大小，单元里一条语句读出；写了的页按它最后的内容算一次。过了 `MaxBatchRewriteBytes` = 400 × 84 KiB（34,406,400 字节）答 422 `page.batch_too_large`。复核 B 的两个形状现在 18、19 毫秒答 422（之前 4.4、4.9 秒答 503）。
    - **界线**：M8 的规则单独会给每页 128 KiB 那一档（3.98、4.29 秒）；预算还要给最后解析的一页最大的内容（5 MiB）留出地方，所以至多 35 MiB，取两者之内的最大一档，每页 84 KiB（3.8.2）。一页最大的内容最后改写时，界线上的字节 3.76 秒做完，96 KiB 那一档的字节答 `server_busy`。
    - **单个 `move`** 照旧不受它限（M6、P2），留给 M12：一个页数、链接与字节共用的界线（3.8）。
  - **新建不让计划的读与触及重来**（复核 A 的 F2-1，`137368a0`）：
    - **以前**：一次改名一个 10 万个节点的文件夹，之后 49 次新建一页再 `write` 一个带链接的页：3.75–3.90 秒、分配 15.2 GB，没有新建时 0.6 秒。新建算作改树的项，每次之后的读重来；它也改了触及的键。
    - **现在**：计划的读只在移动（改名）与删除之后重来；`CreatedAfterMoves` 只列新建在那个节点下面的（照那个窗口的最后一棵树）。同一个形状 1.02–1.06 秒、1.22 GB，计划的时间同没有新建时；`edit` 的那一行照旧答单元的界线（147 个内容），在 0.16 秒里，之前 2.7–2.9 秒、14.4 GB（3.7.4）。
  - **测试**：
    - 链接的数在读树之前（复核 A 的 F2-2 = B 的 B2-N1 = C 的 N1-f2，`9897f380`）：`TestABatchCountsTheLinksItWouldWrite` 经 `askedPaths` 读树，过界时不读链接的目标的路径；字节的测试同样。
    - 页数、链接、字节的次序（复核 A 的 F2-3，`c8e605a5`）：三个都过了答页数的，后两个过了答链接的，只过字节的答字节的。
    - 两次改名之间新建的节点（复核 B 的 B2-N2，`667343f6`）：3.14.3 的第 24 种，B 的 K4r 与 K4tw。
  - **变异**：16 个（11 个 short、5 个数据库），都被抓到：字节的数去掉、移到读树之后、写了的页不数、写了的页都数、用例不交 `MaxBytes`、组合根不转它、错误的文字、大小读错；`readsAt` 在新建之后又重来、`CreatedAfterMoves` 列出每个新建的；每个节点的第一个操作作窗口（short 与数据库）；链接的数在读树之后；链接先于页数；字节先于链接。
  - **门禁**：本机，`c8e605a5`：`make lint-go gen-check knip lint-web` 通过；经 `heavy.sh`：`make test-go` 766 秒通过（第二次；组合根三份 166.8、192.3、302.0 秒；第一次只在 `platform/jobs` 起不了它的容器，负载 8.3 时，这一轮没碰它的代码），`make test-go-short` 56 秒；batch 的守卫一行多了字节的界线的测试，33.1 秒（守卫 6.82 秒、单元 3.271 秒；链接的界线 12.96 秒、单元 3,518 毫秒；字节的界线 12.59 秒：过一个字节 65 毫秒被拒，界线上单元 3,876 毫秒）；工具的说明与 `tools.json` 没变，不跑 `make e2e`。
  - **CI**：run 39416333916（`c8e605a5`）全绿，共 12:43（作业，括号里是测试一步）：竞态 6:37（6:13）、组合根一 5:18（5:01）、二 5:02（4:40）、三 4:58（4:33）、成本 6:23（静态检查 1:14、测试 4:19）、守卫 7:40（7:24）、short 3:05（2:48）、web 2:35（2:09）、e2e 4:56（E2E 一步 3:24）、镜像 1:19。每个作业都比前三次快（守卫那时 11:23–11:53），是 CI 的机器，不是这一轮；守卫多了字节的测试，7:40，在它的 25 分钟之内。第一份最近三次的平均 6:20，在 7:30 之下，份不动。
  - **记下、没改的**（复核 C 的疑点，没验证）：测试的应用的 River 客户端失去又重新得到 leader 时，River 再跑一次启动时的清理，所以第 4 轮的"它的下一次在一小时之后"只在它一直是 leader 时成立；要重选失败约 10 秒（TTL 5 秒加 10 秒的余量），一个客户端的测试库上不大可能，`transferTeam` 与清理自己的测试有同样的前提。写进审查记录。
- **修正的第 6 轮**（2026-10-20，分支 `m9-p3-s1` 的 `cc7d48ad`；三份第三次复核的发现与主持者的决定，`fix-brief-6.md`）：四个提交，各自 build、vet 与碰到的包的 short 测试都通过。
  - **改写过的页留下的解析额度**（复核 B 的 B3-M1，`612ca69b`、`a7632b4d`）：
    - **以前**：三个界线之内，batch 仍会被自己留下的份额答 `server_busy`。字节的界线数的是页，每个改写过的页留到单元结束的份额却随它写出的内容；相对链接与路径形式的链接写出时长出目标新位置的路径。400 页各 150 条相对链接 `[r](./Top/F/Xk.md)`，目标文件夹被移到 7 层 250 字节的名字下面，每条长出约 1,760 字节：2.42 秒后在 ops[0] 答 `server_busy`，每页 300 条时 2.60 秒，重试也一样。
    - **现在**：batch 的改写（`MaxBytes` 设了时）记着它改写过的页的事实留下的份额（`keptShares`）；每次解析之前（读索引里的页的那一次与每次写出的），问解析器这一次要取多少、预算多大（`RewriteParser.Takes`），两者加起来过了预算，就在取之前答 422 `page.batch_too_large`，不等：别人放手也腾不出来。放得下、只是别人的份额挡着的，照旧立即答 `server_busy`；单个 `move` 不变。检查放在参与者里：份额是它留到单元结束的，只有它知道预算里哪些是自己的；页模块的解析会话在它之前已经放掉了自己的份额，也看不到改写的解析。
    - **实测**：上面的形状每页 150 条 2.34 秒、300 条 1.95 秒答 422（时间是之前约 300 页的改写，改写从来不等预算）；预算的一半被别人占着时 1.13、1.04 秒答 `server_busy`；预算 32 MiB 时做完，单元 6.62、11.17 秒。界线上一页最大的内容最后改写的字节照旧做完（单元 3.83 秒）；96 KiB 那一档这样排，现在 2.96 秒答这个 422（之前 3.15 秒答 `server_busy`）。
    - **字节的界线照旧**，是读树之前的早拒；它留的 219 KiB 给寻常的变长（每条链接几十字节）与前置元数据的值，长路径的其余由这个检查拒（3.8）。
  - **测试**：
    - 删除让计划的读重来（复核 A 的 F3-1，`504ed16d`）：`TestABatchsPlanReadsATreeAgainAfterADelete`，F 改名 G、编辑 P、删除 G/X、编辑 Q，P 与 Q 的改写各拿一份读。A 的数据库形状没有提交：它显示过时的触及在读的界线附近的后果，short 测试直接看重来，变异破坏的正是它。
    - 字节的数自己的排除与写了的页的最后大小（复核 C 的 N1-f3、B 的 B3-N1，`cc7d48ad`）：`TestABatchCountsItsRewritesFirst` 里链接都通向被删的页的那个 batch 加 `MaxBytes: 1`；`TestABatchCountsThePagesItWroteAtTheirFinalSize`（数据库，守卫那一行）：batch 把 P 从一行改到约 600 KB 再移动 F，Q 与 P 最后的字节少一个字节被拒、刚好时做完；从约 600 KB 改回一行，同样。
  - **变异**：10 个，都被抓到。short 9 个：检查去掉、先等再判（解析用会等的 `Take`，取不到才判）、到预算就拒（`>=`）、改写过的页的份额不记、单个 `move` 也检查、份额按取的而不是事实留下的记；只在移动之后重来（F3-1）；字节数 `index.pages`、数 `met.pages`。数据库 3 个：检查去掉与先等再判（同上，数据库的测试也抓到）、在单元之外读大小（写了的页的改动之前的大小）。
  - **门禁**：本机，`cc7d48ad`：`make lint-go gen-check knip` 通过；经 `heavy.sh`：`make test-go` 771 秒通过（组合根三份 170.4、193.0、292.7 秒，这一轮没有新的竞态测试，份不动），`make test-go-short` 54 秒；batch 的守卫一行多了两个测试，47.9 秒（守卫 6.66 秒、单元 3.295 秒；链接的界线 12.64 秒、单元 3,437 毫秒；字节的界线 12.30 秒：过一个字节 63 毫秒被拒，界线上单元 3,906 毫秒；最后的大小 0.41 秒；长路径 15.36 秒：2.43 秒答 422，预算的一半被占着时 1.13 秒答 `server_busy`，16 MiB 时单元 8,124 毫秒做完），乘 2.4 约 115 秒，仍是一行；工具的说明与 `tools.json` 没变，不跑 `make e2e`。
  - **CI**：run 39429275198（`cc7d48ad`）全绿，共 17:21（作业，括号里是测试一步）：竞态 7:16（6:56）、组合根一 6:27（6:01）、二 4:09（3:51）、三 5:14（4:56）、成本 7:58（静态检查 1:25、测试 5:30）、守卫 12:47（12:29）、short 2:19（2:02）、web 3:44（3:15）、e2e 4:27（E2E 一步 3:37）、镜像 1:24。守卫多了两个测试（本机约 16 秒），12:47，在它的 25 分钟之内（上一次 7:40，再之前三次 11:23–11:53，机器的差别比这一轮加的大）；第一份最近三次的平均 5:55，在 7:30 之下，份不动。
  - **记下、没改的**：复核 A 的 F3-2：链接与字节的界线的提示说"或分开调用"，一个只有一次移动、远过界线的 batch，它的"分开调用"就是那个不受限的单个 `move`，最重的几档答 `server_busy`；照这些界线定的形状提示是对的，没改，写进 M12 的移交。复核 C 的一次准备时失败、之后四次都通过的运行，不用做什么。
- **修正的第 7 轮**（2026-10-20，分支 `m9-p3-s1` 的 `dd1e8ffc`；第四次复核只剩 Nit，主持者的决定，`fix-brief-7.md`）：两个提交，各自 build、vet 与碰到的包的 short 测试都通过。
  - **精确的检查在 batch 写过的页上，与 `MaxContent` 在它之前**（复核 B 的 B4-N1 = C 的 N1-f4、C 的 N2-f4，`4c921bf4`）：`TestABatchRefusesAParseItsOwnSharesLeaveNoRoomFor` 的 batch 也带它写过的页。新加的情形有四种：
    - 三个写过的页之后，索引里的页读进来时，少一个字节被拒、刚好时做完；
    - 第四个写过的页的写出，同样少一个字节被拒、刚好时做完；
    - 约 1 KiB 的页每次取 MinTake（4 KiB），第四页读进来时被拒；
    - 最后一页正好 `MaxContent`、写出长 200 字节时，照旧记下、留着，之前的三页写了，不拒。
    以前写过的页给空的份额（不查也不记）、适配器的 `Takes` 答 n 而不是取的、检查放在 `MaxContent` 之前，三个变异都通过全部测试；现在都失败。复核 B 的测试是 short 的，这几种情形已经有了它的，不提交。
  - **守卫那一份的时限**（复核 C 的 N3-f4，`dd1e8ffc`）：第 6 轮时作业到 12:47，两倍过了 25 分钟，照约两倍的规则提到 30 分钟（`.github/workflows/ci.yml`）；卡住时 go test 自己的时限仍先到，最晚约 21 分钟。
  - **变异**：12 个都被抓到：这一轮的 3 个，与第 6 轮的 short 9 个。
  - **门禁**：本机，`dd1e8ffc`：`make lint-go gen-check knip` 通过，碰到的包的 short 测试通过；这一轮只改一个 short 测试的文件与 CI 的时限，不跑数据库。
  - **CI**：run 39437145769（`dd1e8ffc`）全绿，共 14:57（作业，括号里是测试一步）：竞态 8:31（8:13）、组合根一 6:08（5:47）、二 4:51（4:33）、三 3:40（3:20）、成本 8:21（静态检查 1:29、测试 5:40）、守卫 10:00（9:31）、short 3:14（2:50）、web 4:20（3:47）、e2e 4:50（E2E 一步 3:39）、镜像 1:18。守卫在新的 30 分钟之下第一次跑，10:00，batch 那一行同第 6 轮的五个测试（那时 12:47，机器的差别）；第一份最近三次的平均 5:36，在 7:30 之下，份不动。
  - **报告里的一个数**（复核 C 的 N4-f4）：第 5、6 轮的 CI 一条里"再之前三次 11:02–11:53"混进了一次测试一步，作业是 11:23–11:53，已改。
- 合并之后：`main` 的 CI（run 39440293080，`f19d0b6f`，树同 `dd1e8ffc`）十个作业全绿（作业，括号里是测试一步；e2e 只有作业的时间）：竞态 5:45（5:24）、组合根一 7:26（7:00）、二 4:04（3:46）、三 3:57（3:39）、成本 9:44（测试 6:49）、守卫 13:09（12:43）、short 3:30（3:00）、web 4:07（3:33）、e2e 4:24、镜像 1:19。组合根一的测试一步最近两次（第 7 轮的 5:47 与合并之后的 7:00）平均 6:23.5，在 7:30 之下，份不动；守卫 13:09，在新的 30 分钟之内。
- 审查：三位审查者（Opus）：A（计划、重做、界线与答复）、B（推后的链接改写与 linking，另有 REST 的差分测试）、C（测试、变异、e2e、实测与 CI）。Important 五条（B-M2 是 A-I1 的同一个发现），都已改。记为高的两条：大文件夹的移动为每条碰到的链接重建一次改动的表，单个移动比 `main` 慢约四倍、400 页时到了 15 秒的期限（B-I1）；`write` 的"相同的字节"照 batch 之前的页判，batch 拒绝分开调用收下的、收下分开调用拒绝的（A-I1 = B-M2）。记为中高的三条：batch 里大文件夹的多次移动没有界线（A-I2）；单元的重做里四种比较有三种没有测试（C-I1）；第 0 步的积压没有放在最新的页里（C-I2）。Minor 记为中低的七条：冻住的节点（B-M1）、写了的页的代价（B-M3）、改名之间的 `write` 每棵树展开一次（修复核对第一轮 A 的 N1）、新建的节点进了触及（B1-M1）、链接密的单元（B1-M3）、重的页答 `server_busy`（第二轮 B2-M1）与写出比页长得多时仍是这样（第三轮 B3-M1）；其余十一条是低，细节 25 条。修复七轮，前六轮由三位核对四轮，第七轮只有 Nit、由主持者核对；修复核对第四轮已没有 Important 与 Minor。逐条见 [S1 的审查记录](reviews/P3-S1-batch-review.md)。
- 交给后面的：M12：batch 的计划与最坏的 batch、`MaxBatchRewrites` 与它的守卫、batch 之后的搜索、许多次改名之间的带链接的页与正文的一遍的再展开、链接密与页的字节的曲线、很重的页、单个 `move` 的同样的风险与一个共用的界线（连同写出的长度与界线的提示），[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项；P4：`grep` 与 `search` 的说明写刚写的可能还没进索引（第 8 节）；S2：组合根第一份照它最近的平均估计。没有交给 v0.1 之后的打磨的。CI 的作业日志要认证，每个测试在 CI 上的用时（batch 的守卫的单元在内）没有记下，只有每一步的时间。
- 负责人可以改判的取舍（主持者的决定）：搜索批量入队、答复带 `building`（由 3.10.1 的规则定）；冻住的节点的链接从不改写；写了的页在 `MaxBatchRewrites` 里算 2；移动带着走的节点、碰到的链接与改写的页的字节三个新的界线与改写过的页留下的解析额度的检查，单个 `move` 照旧不受限；之前的改名、移动碰到它的页的 `write` 读整页；新建的节点不在跟随的操作的触及里；改名的改写碰到被编辑的页只在单元里查到；计划一次只留一棵树的读；许多次改名之间的带链接的页、界线之内的重页与单个链接密的移动 v0.1 接受、交 M12；M8 的清理测试的摆动在 S1 里改；成本那一份的时限、batch 的守卫在 CI 上的余量与组合根第一份现在不动；River 重选时的疑点记下、不改。逐条见审查记录的"主持者的决定"（00 号文档第 11 节第 44–55 条）。

### 9.2 S2：撤销的调度（2026-10-21，合并 `43b03b36`）

- 提交（从 main 的 `9ee2f5bd` 开始，照 5.2 的次序，11 个）：
  - `revertSim` 与四个前提、`schedule()`（轮数的上限在代码里）与三条后备、`RevertAside`、`Seq`、`Current`、`planner.maxDepth`。单元经 `place` 执行临时的步；`round()` 删去，`TestScheduleIsTheRoundForM8sChangesets` 的期望在删去之前以它生成。同一提交改 `TestPlanRevertOfASwapGoesAside` 与用例的 (T4) 一行（`a47c22da`）。
  - `AsideName` 照 `numbered` 截短、留扩展名（`a682e52c`）。
  - `RevertWrites`、`ContentRef`、写回在 `Current` 上、按（页、哈希）解析、`DecidedContents`，界线按页数与份数分开。S1 的临时检查删去，`revert_test.go:342-347` 同一提交改为 `restore P`、`content P`（`1626301f`）。
  - `Steps()` 数临时的步（`d13c717e`）。
  - 任务的临时的拒绝 `unsplit`（`600bc0f1`）。
  - 迁移 00045、`MarkChangesetMoves`、单元结束时设上、`ChangesetMoves` 读列、契约的说明与生成（`d32b8ed4`）。
  - "changeset reverted" 那一行的 `aside`（`f475e642`）。
  - domain 的性质测试与构造的测试（`90ddd8cc`）。
  - 组合根的四个测试（`d9738402`）。
  - MC5 (page)（`96ab099d`）。
  - 第 72、81 条变异先活下来之后加强的两个测试（`59bd9e23`）。
  - 每个提交单独在一个工作树里 build、vet 与它碰到的包的 `go test -short` 都通过。第一次 `make test-go` 抓到权限矩阵以 SQL 种下的改名的变更集没设 `renames`，这一处改在迁移的提交里（原 `83694ef2`，现 `d32b8ed4`），之后的提交照次序重新摘取；审查 C 把重新摘取的每个提交各自核对过。
  - 修正的两轮另有 22 个（第 1 轮 17 个、第 2 轮 5 个，下面各轮），一共 33 个，合并 `43b03b36`（第二个父提交 `03ef0c0b`）。组合根的份没有改（下面"组合根的各份"一条）。
- 与设计不同之处（实施时定下的，都是最小的改动；已改在各节，3.29 列出）：
  1. "恢复到另一组之下"撤销不了（留给主持者）。L（M8 的 `readReverted.topOf`）把删除的节点沿它删除之前的父节点、只要也被这个变更集删除就一直往上找顶，两个上下相套的回收站项读成一组。`rootHolds`、`groupIsDeleted` 拿外面那一项比，它只有自己的成员，所以顶与里面的根都是 `changed`。真的库上的那一行以两次 REST 的删除、再在 SQL 里并成一个变更集种下，断言 409 `page.revert_conflict`（A、B `changed`）、什么都没写。domain 的那一条是包里构造的计划者（`TestARestoreWaitsForTheGroupItGoesUnder`）：恢复 B 等 A 的那一组一轮。要让它撤销得了，得改 M8 的 L，没改：batch 与撤销都做不出它，只有手写的数据。
  2. 用例的 (T4) 一行不用"S\* 里两个同键的节点"：用例上 `before()` 先把它答成 `title_taken`，到不了调度。改为存着的缺陷"比 `MaxDepth` 深的页被改名"：放回在临时的步前后都放不下，答 `ErrRevertUnordered`（500 的路）。同键的构造在 domain 的 `TestScheduleAnswersUnorderedForADefect`。
  3. 契约里 `getChangeset` 与 `ChangesetDetail` 的说明也写着"前 1,000 项"，一并改为全部条目。
  4. 钉住旧的读法的测试：
     - 适配器的 `TestChangesetMoves` 重写（设上之后留着、别的变更集、没有了的变更集）。
     - `TestTheLogReadsUseTheirIndexes` 的 `ChangesetMoves` 走 `changesets_pkey`。
     - 权限矩阵种下的改名的变更集带 `renames = true`（照它的单元会设的）。
  5. `DecidedContents` 是 `DecidedAll` 旁的泛型，不改 `DecidedAll` 的签名，回收站的恢复不动。
  6. MC5 (page) 的树照次序列出，是 `Notes, Glossary, Plan, Index`，不是按名称的 `Glossary, Index, Notes, Plan`。
  7. 临时名称避开 S\* 的键由 `TestAStepAsideTakesNoKeyOfS`（包里的）核对：`AsideName` 的 `taken` 由调用者给，S\* 的那一半在 `aside()`。
  8. (T1) "临时改名两次"的变异在性质测试里活下来，与"不避开 S\* 的键"合在一起也一样（2 万次）：临时名称从不被等着的步要，这条判断在合法的状态里到不了。加了构造的 `TestAFallbackRenamesANodeAsideOnce` 抓它。
  9. 撤销的性质测试：
     - 两种定向的 batch 在 k 个变更集的开头先做（每种至多试三次）。
     - "先写正文再删除"的那个 batch 立即撤销一次：从新到旧连续撤销时链里就有撤销的撤销（先写回、再删除）。不这样，第 72 条变异在 3.27 点名的性质测试里活下来。
  10. 第 81 条变异（按页 id 解析）在组合根的那一行里结束时一样：恢复的事实在同一个单元里被写回的换掉。`TestARevertRestoresAPageThenWritesItBack` 加读守卫看到的每一步的事实，抓到它。审查 B 核对：链接索引、搜索与事件看到的是单元合并的改动，它的事实是写回的，恢复的事实只到守卫（审查 B-N2，3.18.3）。
  11. 第 79 条变异在组合根的附件那一行活下来：附件的临时名称丢了扩展名仍是合法的标题，只是暂时的。`TestAsideName` 抓到它。
  12. 组合根的"移动加改名的中间态"以两项（`F/B → G/B → G/A`）写出，batch 合并成一个移动加改名的条目。
  13. 同一毫秒的两个附件以 SQL 种下，文件经 `storage.OpenLocal` 写进应用的目录（同 `assets_jobs_test.go`；那时没有上传在写）。
- 实测：
  - 00045 的补值：在 00044 的库上以 `generate_series` 造 100 万个变更集、各 3 项（300 万条条目，10 万页上；十分之一改名、十分之一移到别的父节点、其余新建或换次序）。升到 00045 用 1.87 与 2.19 秒，设上的改名、移动各 10 万。
  - 最大的 batch 的撤销（本机，不带竞态，各两次）：
    - 经第三个名称互换 33 对（99 项）：99 步（33 个临时的步、66 个放回），写回 66 页（链接的改写），171–206 毫秒。
    - 审查的反例 20 个（100 项）：100 步（20 个临时的步、20 个删除、20 个恢复、40 个放回），112–137 毫秒。
    - 33 页先写正文再删除、再新建（99 项）：66 步加写回 33 页，118 毫秒。
    - 都远在 `MaxRevertSteps` 的 1,500 之内。
    - 删除约 1 万个节点、再建回文件夹的 batch，请求照 M8 以单元的页数答 `page.revert_too_large`，它的撤销是后台的（S3）。
  - 性质测试：
    - domain 的 2 万次 0.94–0.99 秒，竞态下 6.15–6.22 秒；`-short` 的 2,000 次 0.11 秒，竞态下 0.62–0.63 秒。
    - 次数（确定的）：临时改名 18,570、从待删除的子树移到根下 616、别的移到根下 0、先写回再删除 21,052；`-short` 下 1,928、44、0、2,080。一次撤销至多 3 个临时的步、19 步。
    - 组合根的撤销的性质测试竞态下 5.28–5.45 秒（main 上 8.0–9.1 秒，随机的序列随新的两种操作变了），`TestTheSearchIndexIsItsRebuild` 3.23–3.26 秒。
    - 新的组合根测试竞态下：`TestTheRevertOfEachBatchShape` 1.28–1.30 秒、`TestTheMultiRoundRevertMeetsAWrite` 1.16–1.17 秒、`TestTheChangesetsRenamesCountEveryItem` 2.44–2.68 秒。
  - MC5 (page) 在整个 e2e 里 524 毫秒（单独 587 毫秒），4.4 估计 6–8 秒。
- 反向对照：
  - 6.3 的第 71–86 条都被它点名的测试，或为它加的测试抓到。
  - 第 72 条起初在组合根的性质测试里活下来，第 81 条在组合根的那一行活下来，各加强一个测试之后都抓到（上面第 9、10 条）。第 79 条组合根那一行抓不到，`TestAsideName` 抓到。
  - 3.27 的 S2 各行：
    - 放回不核对环：性质测试抓到（`TestScheduleRevertsEachBatchShape` 单独抓不到）。
    - 轮数的上限：与"去掉第 2 条""第 3 条不论"合在一起时，性质测试以 `ErrRevertUnordered` 失败；去掉上限挂到 `-timeout 90s`。
    - 卡住时不答 `ErrRevertUnordered`、临时名称不避开 S\* 的键、恢复不等它的父组、读还走旧的语句、按份数算一页的例外：都被抓到。
    - "临时改名两次"见第 8 条。
- 门禁（本机，`59bd9e23`）：
  - `make lint-go`、`gen-check`、`knip`、`lint-web`、`test-web`、`build-web`、`build` 都通过。
  - `make test-go-short` 56 秒。
  - `make test-go` 737 秒（第一次失败在权限矩阵的种子上，改后通过）。
  - `make e2e` 260 个 36.5 秒。
- 组合根的各份（6.5 的规则）：
  - 本机单独两次的平均 251.9、133.0、153.0 秒（S1 的 250.6、131.4、145.3：+1.3、+1.6、+7.7 秒；设计估计第二、三份 +5–7、+14–16 秒）。
  - `go test -race -list` 核对：`TestTheRevertOfEachBatchShape`、`TestTheMultiRoundRevertMeetsAWrite` 在第三份；`TestTheChangesetsRenamesCountEveryItem`、`TestRevertingTheNewestChangesetsPutsTheNotebookBack` 在第二份；第一份没有新的。
  - CI 的测试一步 4:49、5:25、3:24（与本机之比 1.15、2.44、1.33），都在约 7:30 的线下。界线没有挪、没有分、没有提高 `-timeout`，CI 的文件没改。
- CI（分支的 run 39463166210，`59bd9e23`，作业，括号里是测试一步）：竞态 5:19（5:01）、组合根一 5:10（4:49）、二 5:46（5:25）、三 3:43（3:24）、守卫 12:13（11:54，时限 30 分钟）、成本 10:06（6:59）、short 2:14（1:56）、web 4:19（3:37）、e2e 4:31（E2E 一步 3:34）、镜像 1:24，都是绿的。
- 实施时留给审查与主持者的，都在下面的修正里定了："恢复到另一组之下"（上面第 1 条）照现在的 L 拒绝（409，什么都不写；3.21.2 的那一行改为这样）；(T1) 的那条判断（第 8 条）留着，构造的测试是它的反向对照；README、v0.1 总体设计与本文的 S2 的文字由主持者写；M12 的性能移交记"00045 的补值：100 万个变更集、300 万条条目约 2 秒，`changesets` 的 ACCESS EXCLUSIVE 锁在那几秒里，只在启动时"。
- **修正的第 1 轮**（2026-10-21，分支 `m9-p3-s2` 的 `5b7c9a33`；三份审查与主持者的决定，`fix-brief-1.md`）：从 `59bd9e23` 起 17 个提交，每项一个，加 CI 的一个；每个提交单独 build、vet 与碰到的包的 short 测试都通过。
  - 实施时留下的两条：上下相套的回收站项照现在拒绝（(a)，3.21.2 的那一行合并时改）；(T1) 的那条判断留着，构造的测试是它的反向对照。
  - 次要的：
    - B-M1 `aa17adaf`：一次撤销里一页的两份正文，在最小的预算下也解析得了。`decidedAll` 在一次取放不下本次调用留着的份额时先还掉它们（3.18.4）。
    - B-M2 与 C-M2 `cefbfa63`：根下与文件夹下的换次序都不设标记，文件夹下的改名只设 `renames`；组合根的测试加经 REST 的文件夹下的改名与换次序。
    - B-M3 `3bc452ac`：迁移的测试里新建与删除的条目放在 P 之下。
    - C-M1 `70374a74`：撤销的性质测试按种类数。agent 自己的移动不算 batch 的；强制的定向 batch 另算；随机的 batch 与 op 12 的各有下限。
    - A-M1 `ebf52ca6`：A 的形状要 6 轮，开始时待做 4 步；没有 D 的要 5 轮、3 步。轮数记在 `revertSim.rounds`，由 `export_test.go` 的 `ScheduleRounds` 交给测试。
    - A-M2 `f60e68c2`：`TestScheduleRevertsEachBatchShape` 加一行"MaxDepth 上带附件的页被删除、再恢复"；domain 的性质测试的模型有了附件，层数照 `Subtree.Height` 不算附件，附件的临时名称要留着扩展名。
  - 小问题：
    - B-N1 = C-N4 `5795d2f9`：标记在搜索入队之前。
    - B-N3 `294ba2f2`：页面事件只列单元之后活着的写过的页；网页不用改，同删除。
    - C-N1 `cde45547`：第 2 条后备按 id 从小到大。
    - C-N2 `cb07812b`：性质测试断言没有一步把节点从根或 S\* 里活着的父节点下移到根下（只有第 3 条后备会这样移；按移动的样子数，不按哪条后备）。
    - C-N3 `63900811`：每一行只读自己开始之后的日志。
    - C-N5 `26bd04e5`：临时改过名再移到根下的节点，名称从计划开始时的造。
    - A-N1 `bdd85857`：3.17 的空主干，M7 的 `Numbered` 一并改（存下的名称一个也不改：导入经 `CheckTitle` 去掉开头的空格，从来没有以空格开头的名称存下；改的只是很少见的新名称怎么造）。
    - A-N2 `044dcc4c`：第 2 条后备只取待放回的节点。
    - A-N3 `07d984e8`：`stepKey` 的注释。
    - A-N4 `62187eda`：任务的临时的拒绝包一层文字，`errors.Is` 仍找得到 `ErrRevertUnordered`。
  - 实测：
    - B 的形状（真的库）：竞态下 5 MiB 的预算 833–894 毫秒、8 MiB 846–894 毫秒答 200（修之前 5 MiB 每次 503）；不带竞态 177 与 165 毫秒。
    - domain 的性质测试 2 万次：77,784 次撤销，临时改名 13,142、从待删除的子树移到根下 584、别的移到根下 0、先写回再删除 15,912、附件的步 35,262、带附件的恢复 28,328；0.99–1.00 秒，竞态下 6.31–6.32 秒；`-short` 0.13 秒，竞态下 0.70 秒。
    - 组合根的性质测试（四个种子）：agent 自己的移动 3、它的随机 batch 2、op 12 的定向 batch 2、强制的 8；下限 2、1、1、4。
    - 组合根的第二、三份单独两次的平均 141.7、168.3 秒（S2 的 133.0、153.0 秒），第三份多的主要是两份正文的那个测试（竞态下约 11 秒）。
  - 反向对照，都被抓到：
    - 审查者的：B 的"换次序设移动"；C 的 A6、A6b、A5、C6、C24、C24b、75（没有第 2 条后备时，第 3 条后备接过它的 584 次移动，其中 582 次的样子同第 2 条的，2 次不是：全量的性质测试抓到这 2 次，`-short` 由 `TestTheSecondFallbackPicksTheNodeItFrees` 抓到）。
    - 这一轮的：两条 `IS NOT NULL` 去掉；不还份额与总是还；没有 `2N` 与 `N/4`；`canRestore` 数附件；生成器不做 `agentBatch`，或不做 agent 的移动；空主干留着；`places` 的条件去掉；拒绝不包文字。
  - 门禁（本机，`62187eda`）：
    - `make lint-go gen-check knip` 通过；
    - 经 `heavy.sh`：`make test-go` 749 秒，`make test-go-short` 52 秒；
    - 17 个提交各自 build、vet 与碰到的包的 short 测试都通过。
  - CI：
    - run 39481193146（`62187eda`）红：组合根第三份里新加的两份正文的测试，最小预算那一行的 batch（写 5 MiB 的正文）答"the call ran out of time"，过了测试的 4 秒的请求期限（本机竞态下约 2.5 秒，CI 慢 1.7–2.5 倍）；别的作业都绿，e2e 与镜像没跑。实施者照修复说明在红的运行处停下，主持者同意之后才推 `5b7c9a33`。
    - 修在 `5b7c9a33`：这个测试的应用用生产的期限（请求 15 秒、写 60 秒），同 S1 的 `TestTheMCPBatchIsRefusedWhole`。写 5 MiB 正文的 batch（写、再删除）不带竞态约 0.25 秒、竞态下约 2.5 秒，与 `main` 相同：给这个测试生产的期限不藏慢。
    - run 39483864444（`5b7c9a33`）全绿，共 17 分钟（作业，括号里是测试一步）：竞态 6:59（6:30）、组合根一 7:28（7:07）、二 5:52（5:31）、三 5:42（5:20）、守卫 13:08（12:50）、成本 9:41（6:44）、short 3:16（2:55）、web 3:43（3:12）、e2e 3:42（E2E 一步 2:56）、镜像 1:22。第三份的测试一步比 S2 的 run 39463166210 的 3:24 多 1:56：本机单独只多约 15 秒，其余是运行机的摆动（这次与本机之比 1.90，同 6.5 的 1.93），在 7:30 之下。
    - 组合根第一份没有 S2 与这一轮的测试，最近两次 7:12、7:07，平均 7:09.5，在 7:30 之下约 20 秒：界线不挪；S3 照 6.5 记它两次，过了线时把 `TestEvery[M-R]` 挪到第三份。
- **修正的第 2 轮**（2026-10-21，分支 `m9-p3-s2` 的 `03ef0c0b`；三份修正核对与主持者的决定，`fix-brief-2.md`）：修正核对没有重要的与次要的，从 `5b7c9a33` 起 5 个提交，都是小问题；每个提交单独 build、vet 与碰到的包的 short 测试都通过。
  - B 的 N1、N2 `2b4a1b1b`：还份额的边界。份额与取恰好等于预算的大小时留着，多一个字节就还掉；还掉之后，第三份正文的份额留在第二份的旁边。第三份是 1 KiB 的那一行看不出"计数不清零"（两份的份额加起来仍远在预算之内），4.5 MB 的那一行看得出。
  - A 的 N4 与 C 对界线各项的意见 `997ceeef`：`planRevert` 与测试的钩子共用 `scheduled`，计划者的构造、L、S\* 的核对、计划只写一处；调度记下开始时待做的步数（`revertSim.pending`），界线由它算，测试也核对它。
  - A 的 N1 `428cc10a`：A 的 seed 5111 的形状（三页跨层换位，S\* `/c (P)`、`P/A (Q)`、`Q/c (R)`，之后 `/c (Q)`、`Q/c (P)`、`P/A (R)` 与新建的 `Q/a`）要 7 轮，开始时待做 4 步，N = 4：`N/3` 的界线答 `ErrRevertUnordered`。C 的两个活下来的变异（以 N 代 2N、界线不加 1）照主持者的决定不加测试：界线是 (T2) 的证明，大一些无害，测试从下面钉住它，到 A 找到的形状为止。
  - A 的 N2、N3 = C 的 N1 `d84c32c4`：注释重排；性质测试的第 3 条后备的检查按移动的样子数（没有一步把节点从根或 S\* 里活着的父节点下移到根下），注释与报错照实说。
  - C 的 N2 `03ef0c0b`：两份正文的组合根测试只留最小预算那一行。默认预算的那一行在哪个还份额的变异下都不失败，单元测试的那一行已管它。
  - C 的 N3：S3 的简报与文档的事，代码不改；6.5 照改（第一份过线时 `TestEvery[M-R]` 挪到第三份，第二份的估计随之，主持者的决定 9）。
  - 实测：两份正文的组合根测试竞态下 6.47 秒（第 1 轮 11.2 秒），撤销答 200 用 860 毫秒；第三份单独两次 158.0、164.2 秒，平均 161.1 秒（第 1 轮之后 168.3 秒）。
  - 反向对照：B 的 F6（早还半个 MiB）、F7（还掉之后计数不清零）、以 `len(p.c.ids)/3` 为界线的那一项、没有 `2N`、`pending` 不记、不还份额（真的库上最小预算那一行）、没有第 2 条后备（全量的性质测试）都被抓到；C 的两个界线的变异照决定活着。
  - 门禁：本机，`03ef0c0b`：`make lint-go gen-check knip` 通过，碰到的包的 short 测试通过，5 个提交各自 build、vet 与碰到的包的 short 测试都通过；经 `heavy.sh`：改了的组合根测试与它的变异、第三份单独两次。
  - CI：run 39491800113（`03ef0c0b`）全绿，共 15 分钟（作业，括号里是测试一步）：竞态 6:42（6:25）、组合根一 6:28（6:08）、二 4:02（3:45）、三 3:47（3:30）、守卫 10:03（9:43）、成本 6:42（4:35）、short 2:22（2:02）、web 3:54（3:22）、e2e 4:35（E2E 一步 3:43）、镜像 1:23。组合根第一份最近两次 7:07、6:08，平均 6:37.5，在 7:30 之下。
  - 主持者的核对（这一轮只有 Nit）：五个提交的差异、上面的 CI 运行与三个变异（还掉之后 `kept` 不清零、上限以 `len(p.c.ids)/3`、不还），各被这一轮的测试抓到。
- 合并之后：`main` 的 CI run 39494589373（`43b03b36`，合并之后的 `main`，合并的树与 `03ef0c0b` 的相同）全绿：竞态 8:37（8:17）、组合根一 6:49（6:28）、组合根二 5:58（5:33）、组合根三 5:52（5:30）、守卫 10:29（10:04）、成本 10:24（7:10）、short 3:17（2:53）、web 4:04（3:31）、e2e 4:20（3:34）、镜像 1:26。组合根第一份最近两次 6:08、6:28，平均 6:18。
- 审查：三位审查者（Opus）：A（调度、前提与后备、临时名称与任务的拒绝，另有对抗的生成器与 `main` 的 `round()` 的逐个比较）、B（单元、正文、迁移与读，另有真的库上的探针）、C（测试、变异、e2e、实测与 CI）。没有 Important，三位都没有找到答错的结果。Minor 七条：记为中低的一条，最小的解析预算下一页先恢复、再写回的撤销每次都答 503（B-M1）；其余六条是低，都是测试的空当（A-M1、A-M2、B-M2、B-M3、C-M1、C-M2；B-M2 与 C-M2 一个修复）。细节 14 条（C-N4 同 B-N1）。修复两轮：第一轮由三位核对，已没有 Important 与 Minor（细节 10 条，A 的 N3 同 C 的 N1）；第二轮只有 Nit，由主持者核对。逐条见 [S2 的审查记录](reviews/P3-S2-revert-review.md)。
- 交给后面的：M10：守卫只凭名称认得临时的步，可以在 `Step` 上带步的种类或标记（第 8 节）；M11：回滚一组变更集时用 `schedule()`（第 8 节）；M12：00045 的补值的用时与锁，[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项；S3：任务的临时的拒绝在第 5 步删去，组合根第一份照 6.5 记两次、过线时 `TestEvery[M-R]` 挪到第三份，`TestTheChangesetsRenamesCountEveryItem` 加带临时的步的后台撤销。没有交给 v0.1 之后的打磨的。CI 的作业日志要认证，每个测试在 CI 上的用时没有记下，只有每一步的时间。
- 负责人可以改判的取舍（主持者的决定）：上下相套的回收站项照 M8 的 L 拒绝；(T1) 的判断留着、以构造的测试为反向对照；一页的两份正文在最小的解析预算下先还掉本次调用的份额；空的主干照页的名称（M7 的 `Numbered` 一并改）；页面事件只列单元之后活着的写过的页；后台撤销的临时的拒绝说出它是什么；轮数的上限从下面钉住；测试加强的几处；CI：两份正文的测试用生产的期限，第一份过线时挪到第三份；给 M10 的一条；A 的存疑、00045 的锁与成本、守卫两份的时限记下、不改。逐条见审查记录的"主持者的决定"（00 号文档第 11 节第 56–66 条）。

### 9.3 S3：后台撤销

## 附录：核对过的代码

行号是 `f3854b9a` 的（`f1733403` 只改文档）。

| 决定 | 核对过的代码 |
|---|---|
| 字段名的一种写法（3.2） | `page/domain/steps.go:31-92`（`CreationOf`、`MissingAncestors` 收字段；`MoveKinds` 写死 `"to"`，`:77-82`）；`page/domain/errors.go:203-212`（`Recursive` 写死 `"recursive"`）；`page/domain/edit.go:21-39`、`:72-119`；`page/app/write_path.go:84-87`、`edit_page.go:83-86`；`page/app/relocate_path.go:55-157`（`parentOf`、`holdsNoNodes`、`PathTaken.Path = to.Written`、两次判定）；`page/app/unit_relocate.go:16-20`、`:34-74`（同一个父节点的改名不读子树，`:43-45`）；`page/app/unit_rename.go:60-65`；`page/adapter/mcp/text.go:22-61`（`Recursive` 与 `ParentOf` 的字段写进文字）；`page/domain` 的非测试文件里产生 422 的 29 个函数（`changeset.go`、`content.go`、`edit.go`、`errors.go`、`glob.go`、`list.go`、`name.go`、`node.go`、`path.go`、`read.go`、`search.go`、`steps.go`、`version.go`、`version_token.go`；`CheckAssetRename` 与 `CheckTitle` 经 `nameOf` 到 batch 的 `move`）；`page/domain/name.go:17-39`（`CheckAssetRename` 调 `CheckAssetName`，`:32-33`）；`page/domain/version_token.go:32-50`（`ParseVersionToken` 调 `NotAVersion`）；`page/domain/changeset.go:160-176`（`CheckMessage`，字段是 `message`）；`page/domain/errors.go:205-212`、`:257-261`、`edit.go:109-120`、`path.go:93-96`（构造函数） |
| 平的项、schema（3.3） | `platform/httpserver/mcpserver/schema.go:21-197`（`addNode` 的成员、`normalize`、`notebookParameter` `:19`）；`mcpserver/tools_test.go:125-179`（`opIn`、`nestedIn`、嵌入的结构）；`mcpserver/tool.go:194-201`（解码的错误是 `internal_error`） |
| 计划的读、模拟与两种界线（3.4） | `page/app/ports.go:154`（`ListNodes`）、`:211-216`（`PageStates`）；`page/adapter/postgres/queries/contents.sql:17-31`（`content_hash` 一列）、`:40-44`（`PageStates` 答版本号与 `byte_size`，不读正文；加 `content_hash`）；`page/app/unit.go:396-399`（哈希是 sha256）；`page/app/unit_content.go:61-66`（相同的字节在 `Base` 之前比）；`page/app/edit_page.go:138-142`（`PageContent` 一条语句答正文与版本号）、`:145-147`（`edit` 在 `Replace` 之前比令牌）、`:151-161`（锁的预检）；`page/domain/edit.go:21-39`（空的 `new_string` 可以）；`page/app/node_paths.go:36-122`（`Walk`、`Find`）；`page/app/unit_place.go:40-63`、`:150-157`；`page/adapter/postgres/store.go:396-407`（`DeleteNodes` 的 `stray`）；`page/app/content.go:55-67`（`Decided` 取 `len(content)`、留十分之一）、`:76-131`（`DecidedAll`、合计的等待）；`platform/markdown/budget.go:40-147`（`hold`、`KeepFacts` `:132-134`）；`linking/app/rewrite.go:243-249`（M6/P4 放还份额、事实留着）；`server/configs/config.yaml:17`（`server.request_timeout`）、`:94-98`（解析的预算与 `parse_max_wait`）；`bootstrap/mcp.go:31-41`（大的请求体的名额）；`page/app/write_path.go:185-187`（不带 `base_version` 的 `write` 以大小与哈希比）；`page/adapter/postgres/store.go:187-197`（`PageStates`，`:194` 填字段）、`paths_test.go:94-113`（`TestPageStates` 以 `!=` 比整个 `PageState`）；`platform/httpserver/apierrors.go:272-273`（过了期限的请求答 500、记作过了期限） |
| 合并与条目（3.5） | `page/adapter/postgres/queries/changesets.sql:11-57`；`00015_page_changeset_items.sql:25-28`；`00038_page_changeset_items_deleted_revision.sql:8-9`（两个检查都在一行之内，不要求变更集有条目）；`page/domain/change.go:63-73`；`page/domain/changeset.go:268-284`（父节点与名称相同时落到 `ChangeReordered`）；`page/app/unit.go:264-291`（`apply`）、`:339-365`（`ensureChangeset`）、`:366-392`（`recordItems` 与注释）、`:450-453`（`tell` 没有改动时什么都不做）；`page/app/unit_content.go:61-64`（相同的字节不写）；`page/adapter/postgres/queries/history.sql:83-104`（`ListChanges` 答版本行的版本号）；`page/domain/revert.go:415-500`（`left()` 对新建的页比版本行的哈希） |
| 单元与重做（3.6） | `page/app/unit.go:168-230`（`Writer.Run`、锁的模式 `:196-206`、锁下判定 `:207-210`、`do` `:218`、`queueSearch` `:221`）；`page/app/unit_content.go:38-41`、`:46-96`（`Base` 不是当前的答 `ErrRevisionMismatch`，`:62-66`）；`page/app/unit_create.go:28-67`（`uuid.NewV7()` 在 `:63`）；`page/app/write_path.go:177-191`（分开调用在锁下比字节）；`page/app/edit_page.go:197-214`（`mismatch` 的 `Why`）；`page/adapter/mcp/text.go:62-74`；`page/app/node_paths.go:111-113`（`Paths` 只答字符串）；`page/adapter/postgres/store.go:140-143`（`LinkTargetsByIDs`） |
| 推后的改写（3.7） | `linking/app/rewrite.go:47-89`（`Participate`，`:49` 的返回、`:57-68` 的 `IndexedOf` 的过滤）、`:95-132`（`reached`，`Store.Links` `:107`、`resolutions` 的缺页 `:115-117`）、`:136-161`（锁）、`:166-206`（`tree`、`FormerParents`）、`:216-310`（`rewrite`：`:219-226` 正文不在、`:227-231` 跳过、`:243-249` 解析之前放还、`:252-254` 与 `:315-318` 记 error、`:309` 基）；`linking/app/resolver.go:54-167`（`resolutions`；来源页缺 `:139`、别名的页缺 `:157`）；`linking/adapter/postgres/queries/pages.sql:60-64`（`AliasesByKeys` 不看页活不活着）、`:70-72`（观察者删行）；`linking/app/index.go:95-112`（`spread`）；`linking/domain/change.go:32-37`（`relocates`）、`:54-80`（`Recased`、`Relocation`，只留最后一个 `:74-78`）、`:85-94`（`FormerParents`）、`:101-122`（`PathBefore`）、`:174-198`（`Affected`）；`linking/domain/rewrite.go:58-97`（`Rewrites`：悬空的不改写，之前与之后都歧义的不改写 `:62`；`Rewrite`）；`linking/domain/resolve.go:80-84`（`Resolution`）、`:95-97`、`:310-336`（最短的路径、id 最小的定歧义）；`linking/adapter/markdown/facts.go:245-252`（`ParseNow` 的 503）；`linking/app/ports.go:193-216`、`:242-247`（`Moved`）、`:271-274`；`bootstrap/linking.go:55-80`（参与者的适配）；`bootstrap/deps.go:141-171`（`pageDeps`）、`bootstrap/wire.go:155`；`page/app/unit_move.go:45`、`:68-87`（单个 `move` 读一次子树、改动列出它之下的节点）；`linking/app/index.go:92-109`（`spread` 只展开给它的节点的子树）；`linking/app/ports.go:204-206`（`Pages.Subtree`）；`scratchpad/m9/p3/exp/`（原型，3.7.1） |
| 与分开调用的不同（3.14.4） | `linking/domain/rewrite.go:58-68`（`before.ID` 为空不改写：悬空的链接被改名接住、之后的改名照分开调用改写，batch 不改）；`linking/app/index.go:95-101`（祖先的改动经 `spread` 碰到子孙：`update_links` 按节点）；`bootstrap/links_rebuild_test.go:169-207`（`ledTo`、`leadStill`） |
| 界线（3.8） | `page/domain/trash.go:46-73`（`MaxUnit*`、`UnitTooLarge` 按份数判一页）；`mcpserver/edge.go:56-77`（`MaxBodyValues` 与注释）；`bodyshape/ambiguity.go:48-57`（值的个数不导出）；`bootstrap/revert_cost_test.go:24-31`（M8 的规则）、`:565-577`、`:625`、`:640-650`、`:672`（三倍：12 秒与 8 秒）；`bootstrap/trash_cost_test.go:135-145`（基准只挂 REST 的路由）、`:166-170`（满负载的页）；`Makefile:259-275`（守卫那一份） |
| 答复与错误（3.9） | `mcpserver/answer.go:17`、`:25-60`（`TextBytes`）、`:127-187`（`Fit`）；`mcpserver/errors.go:25-52`（`ToolError`、`ErrorFit`、保留的成员）、`:117-150`（`problemAnswer` 的文字）、`:187-205`（`ErrorAnswerBytes`）；`page/adapter/mcp/write.go:14-75`、`edit.go:14-30`、`move.go:20-57`、`delete.go:14-26`、`locks.go:38-98`、`tools.go:121-126`；`page/domain/errors.go:60-106`（`shared.NewError` 的写法、今天的 `ErrBatchTooLarge`）；`page/app/jobs.go:363-478`、`page/app/jobs_test.go:462-486`、`:746`、`bootstrap/background_cost_test.go:303`、`bootstrap/background_test.go:269`、`bootstrap/transfer.go:231-232`（哨兵改名的每一处）；`shared/error.go:111-133` |
| 加锁、搜索、事件（3.10） | `page/app/unit.go:196-206`、`:427-446`、`:465-480`（`appender`）；`bootstrap/search_bench_test.go:28-45`、`:189-198`；[M8/P5](../M8-history-search/05-P5-search.md) 3.9 的表（:897）；`events/app/publisher.go:50-64` |
| 工具与清单（3.12） | `page/adapter/mcp/tools.go:70-117`；`page/adapter/mcp/instructions.go:36-62`、`instructions_test.go:15-27`（`TextBytes` 的数法）；`api/mcp/tools.json`（`move` 的两个条目，字母序，`notebook` 在先）；`bootstrap/mcp_tools_test.go:152-160` |
| 码的表、矩阵、字符串（3.13） | `bootstrap/permission_matrix_mcp_test.go:40`、`:70-122`、`:328-331`、`:520-523`（`after` 只有 `call`）；`bootstrap/mcp_client_test.go:305-339`（`declaredCodes` 的注释）；`platform/httpserver/apitest/problems.go:95-130`；`bootstrap/permission_matrix_mcp_writes_test.go:1-80`；`bootstrap/mcp_reads_test.go:528-606`、`:667-686`、`:734`、`:750-753`、`:761-809`；`access/domain/rules.go:90-110`；`page/domain/actions.go:8-46` |
| 测试的落点（3.14、6.5） | `page/extension_test.go:160`、`:693-755`（真的库上的页的模块测试的写法）；`bootstrap/links_rebuild_test.go:25-95`、`:240-252`、`:300-360`；`bootstrap/search_rebuild_test.go:22-26`；`bootstrap/revert_property_test.go:105`、`:166-169`、`:182-184`、`:272-281`；`bootstrap/interleavings_mcp_test.go:21-300`；`bootstrap/interleavings_workspace_test.go:71`（`newAcmeTeamLogging`）；`bootstrap/background_test.go:160-215`（`behindFirstBatch`、`holdNode`）；`Makefile:179`、`:217-219`、`:228-237`（三份的表达式与 `-race`）；`.github/workflows/ci.yml:94` |
| 调度（3.16） | `page/domain/revert.go:41-55`（`MaxRevertSteps`）、`:89-97`（`ErrRevertUnordered`）、`:107-141`（`RevertState`、`RevertPlan`、`Steps()`）、`:146-170`（`RevertOp`）、`:355-375`（`PlanRevert`）、`:767-808`（模拟）、`:811-861`（`round()`）、`:869-901`（`place`）、`:904-918`、`:920-938`、`:941-962`；`page/app/unit_revert.go:78`、`:178-179`（`planned` 在笔记本的锁下）；钉住旧行为的 `page/domain/revert_test.go:342-347`、`:864-886` 与 `page/app/revert_test.go:456-464`；`scratchpad/m9/p3/sim/schedule.py:228-370`、`sim2/schedule_cap.py` |
| 临时名称（3.17） | `page/domain/name.go:50-87`（`Numbered`、`numbered`）；`00012_page_nodes.sql:14` |
| 先恢复再写回（3.18） | `page/domain/revert.go:194-230`（`RevertWrites`、`RevertRefs`）、`:257-286`（`readReverted`）；`page/app/unit_revert.go:30-36`、`:61-118`（`Revert`，写回的基 `:106-107`）、`:125-157`（`place`）、`:191-196`（`parsedAs` 答 `changed`）、`:197-244`；`page/app/revert_changeset.go:89-90`、`:129-133`（"changeset reverted" 那一行）、`:259-275`（`withinBounds`） |
| `renames`、`moves`（3.20） | `page/adapter/postgres/queries/history.sql:73-81`；`page/app/history.go:85-93`；`page/app/ports.go:414-420`；`page/domain/changeset.go:80-90`；`api/modules/page.yaml:1867-1872`；`server/migrations/deleted_revision_test.go:1-80`、`background_jobs_test.go:14-25`（`downTo` 撤回它点名的版本） |
| 后台（3.22–3.25） | `page/app/jobs.go:243-263`（`revertPlan`、`revertStep`）、`:281-301`、`:305-358`（`revertSteps`；删除的子树取计划读到的树 `:325-330`）、`:360-382`（`RunBatch`；`trash_changed` 译成 `changeset_trimmed` `:378-380`）、`:402`、`:420-440`（`withinHistory`）、`:478`、`:505-532`（`begun`）；`page/app/jobs.go:340-349`（成员 0 的步带全组的 id）；`page/app/jobs_test.go:828`（`TestARevertJobsBatchPlansItsOwnNodes`）；`page/domain/jobs.go:30-39`、`:51-66`（`GroupNode`）；`page/domain/revert.go:292`（`topOf`）、`:320`（`rootHolds`）、`:504`（`groupIsDeleted`）、`:578-581`（`before()` 只对父节点要被删除、变更集没碰过的节点答 `children_added`：加进来的部分的顶）、`:719-723`（`CompareConflicts`）；`page/app/revert_changeset.go:312-320`（`locked()`）；`page/app/unit_revert.go:395-485`（`RevertBatch`；读 `:407-423`、恢复的循环 `:463-471`）、`:493-551`（`batchChangeset` `:507-512`、`replan`）、`:555-574`（`batchOps`）、`:770-795`（`restoreSegment`、`retimed`、续接、`markFrontier`）；`page/domain/revert.go:425-432`（`left()`）、`:544-566`（`before()`）；`transfer/app/background.go:80-211`（`run`、`batch`、`stopped`）；`transfer/app/import_ports.go:67-76`、`:170`；`transfer/app/batches.go:24-28`；`transfer/app/maintain.go:65-95`；`transfer/domain/report.go:55-78`；`web/…/pages/notebook/transfer-report.tsx:20-75`；`web/…/i18n/messages/en.ts:693-695`、`zh-CN.ts:671-673`；`e2e/stories/history/hi5-revert-import.spec.ts:1-80` |
| e2e（第 4 节） | `e2e/fixtures/mcp.ts:114-176`；`e2e/fixtures/assert/page.ts:190-218`；`e2e/stories/mcp/mc3-write-edit.spec.ts:61-88`、`mc4-move-delete.spec.ts:1-60`；`e2e/stories/collab/c4-guard.spec.ts:14-91`（`openSession`）；`web/apps/web/src/i18n/messages/en.ts:340`、`web/apps/web/src/pages/notebook/revert-dialog.tsx:315` |
| 文档 | `00-M9-design.md`（`f1733403`）第 1 节第 5 条 `:20`、第 3 节第 7 条 `:61`、4.11 `:256-283`、4.12 `:285-308`、4.14 `:356`、`:359`、4.15 `:361-372`、第 7 节 `:430`、"移交的落实" `:436-450`、"写给后面的 M" `:465`、"总体设计的修订" `:477`、第 9 节 `:520`、`:550`、第 10 节 `:612-620`、第 11 节主持者的决定第 4 条 `:640`、第 33 条 `:675`、设计的取舍第 13 条 `:704`、第 12 节 `:712`、第 13 节 `:717-730`；`v0.1-design.md` 4.5 `:419-430`、13.1 `:1016-1144`（`:1019`、`:1023`、`:1048`、`:1138-1139`）、13.4 `:1220-1240`；`README.md:172-173`、`:187`、`:210-212` |
