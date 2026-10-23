```yaml
status: done
from: M8/P3
to: M9
created: 2026-10-14
```

# batch 的整组撤销：多轮应用与临时名称

M8 的整组撤销只做一轮：放回（改名与移动改回，先移动后改名）→ 删除 C 新建的（S\* 里最上层的）→ 恢复 C 删除的各组 → 写回正文。`PlanRevert` 模拟这一轮，哪一步的前提在它那一刻不成立就答 `ErrRevertUnordered`（缺陷，500）（[M8/P3 文档](../../M8-history-search/03-P3-log-revert.md) 3.5，[00 号文档](../../M8-history-search/00-M8-design.md) 4.8"单轮应用"）。M8 的每个变更集只有一种操作（撤销是被撤销者的反面），一轮做完；之后发生的互换名称、移入新建的文件夹、删除再新建同名在 S\* 里是 `title_taken`、`children_added`，以冲突答出（[P3 设计审查](../../M8-history-search/reviews/P3-design-review.md)，审查者 A 核对过，找不到反例）。

M9 的 `batch`（一个单元多个操作）是第一个让一个变更集里有几种操作的地方。撤销这样的变更集，单轮会答 500。M9 引入 `batch` 时一并做：

1. **推后与多轮**：一步的前提（父节点活着、名称空着、不成环、层级）此刻不成立时推后，等别的步先做；只剩名称的环（C 里两页互换了名称）时，把其中一个先改成临时名称。S\* 合法时总能应用完，做不完仍是缺陷。
2. **移动带着现在的名称**：`MoveNode` 以节点现在的名称把它挂到新父节点下，撞上兄弟的唯一索引时适配器答 `ErrTitleTaken`（`page/adapter/postgres/store.go` 的 `MoveNode`）。C 把 X 从 P 移到 Q 并把它从 A 改名为 B，之后有人在 P 下建了 B：S\* 合法（P 下是 B 与 A），而"先移动后改名"在中间态撞上。要先改名再移动，或经临时名称；模拟按两步的中间状态判"空着"（P3 设计审查 A-M5）。
3. **临时名称的界线**：`名称 ~xxxxxxxx`（id 的前 8 个十六进制字符）对 250 字节以上的名称超出 `nodes_name_check` 的 255 字节（`migrations/sql/00012_page_nodes.sql`），要截短；附件的名称要保留扩展名的形状（平时的改名经 `CheckAssetRename`，`page/app/unit_rename.go`）。同一单元里条目按节点合并（第一次的"前"、最后的"后"），临时名称不留在条目里。
4. **恢复到另一组之下**：一组的父节点在 C 删除的另一组里时，排在那一组之后。
5. **一页既要恢复又要写回正文**：P3 的 `Parsed` 按页一份（M8 的变更集里一页不会两者都要）。`batch` 里一页可能先恢复、再写正文，要按（页、正文的哈希）分开解析，恢复的那一步带它自己的 `Facts`，界线按两份算。
6. **先改名再删除同一页**：P3 已让 `RecordItems` 在合并成删除时取删除的版本号（否则这一页之后撤销永远 `changed`，P3 设计审查 A-M2），适配器上有测试；M9 加整个程序上的测试。
7. **先写正文、再删除同一页**（[P3A 审查](../../M8-history-search/reviews/P3A-log-revert-server-review.md) A-3，P3A 的偏离 6）：C 有这一页的版本行，也有它的删除条目。P3 不写回它的 `Base`，以删除时的正文恢复它（`RevertWrites` 只取 C 既没新建、恢复也没删除的页；`TestPlanRevertReversesEachChange` 钉住，[M8/P3 文档](../../M8-history-search/03-P3-log-revert.md) 3.5）：结果是错的正文，没有冲突、不答 500。M8 做不出这样的变更集，`batch` 做得出。撤销要先恢复、再写回 `Base`（或解析 `Base` 而不是回收站里的正文），两步各自核对；与第 5 条的按（页、哈希）分开解析一起做。
8. **单轮的步数界线**（P3A 审查 C-I2）：P3A 让一次撤销的放回、删除、恢复合计至多 `domain.MaxRevertSteps` = 1,500 步（`RevertPlan.Steps`，写回正文不算，它由页数与字节限住），未加锁时在计划之后、读正文与解析之前核对，锁下再核对，超过答 422 `page.revert_too_large`：每一步是一次单元的操作，根下几千个最上层的撤销在 `MaxUnitNodes` 之内超过请求的期限（M8/P3 文档 3.11）。`batch` 的撤销同受它限：多轮时推后的步与临时名称的改名也是步，一并数进去；大的 `batch` 走 P4 的后台撤销。
9. **测试**：00 号文档第 9 节点名的三种（互换名称、移入新建的文件夹、删除再新建同名）、恢复到另一组之下、移动与改名的中间态、长名称与附件的临时名称、先写正文再删除同一页，都在真的数据库上测（不只 domain 的纯函数），并把 `batch` 作为一种操作加进撤销的性质测试（`bootstrap/revert_property_test.go`）的操作序列。

另：

- 单个变更集的 `renames`、`moves`（`getChangeset`，网页的撤销据此提示链接不跟着改回、给"改回原名"）只看前 1,000 项条目（`ChangesetMoves`）：M8 的变更集里改名、移动各自只有一项，`batch` 的大变更集里第 1,000 项之后的改名不算，同计数的"或更多"。M9 定下要不要另记。
- 一个单元里先建后删、删除再恢复同一节点仍按 [M4/P2 一个单元里先建后删](M4-P2-unit-merge.md)：它的处理进展写明 M8/P3 之后删除再恢复撞上的检查（M8 收尾 C-M11；原在 [M4 历史、回收站与恢复](../../M8-history-search/handoffs/M4-history.md)第 3 项，那份已关闭）。
- **`log`**（M8 收尾补上，[M8 收尾审查](../../M8-history-search/reviews/M8-closeout-review.md) C-I2）：调用 page 的 `ListChangesets` 用例（`page/app/list_changesets.go`），筛选与游标已备：`actor`（账户 id）、`client`（`web`、`api`、`cli`，`mcp` 表示全部 MCP 客户端，`mcp:<名>` 某一个，`domain.ParseClientFilter`）、`page_id`（碰过这一页的变更集，回收站里的页照列）、游标 `[created_at, id]`（`domain.ChangesetCursor`）；每条带类型、执行者、说明、撤销的与被哪些撤销过（[M8 总设计](../../M8-history-search/00-M8-design.md) 4.7）。
- **`batch` 写说明**（同上）：变更集的 `message`（1–4,096 字节，`domain.CheckMessage`）由单元的 `UnitSpec.Message` 写进 `changesets.message`，只在单元建这个变更集时写，并入的变更集保留它自己的；M8 只有撤销写它（`revertChangeset` 与撤销的任务）。`batch` 的说明照此交给 `UnitSpec.Message`。
- **要不要给 agent 历史、撤销与恢复的工具**（同上；[M8 设计审查](../../M8-history-search/reviews/M8-design-review.md) C-Q4）：总体设计 5.3 说"失败的 ingest 可以整组撤销"，6.3 的工具表只有 `log`。版本、单个变更集、回收站的读与还原、撤销、恢复的用例都在 page（M8 总设计 4.1），MCP 适配器可以直接调用。M9 的设计定给不给、给哪几个，权限照网页（读者读、写者还原撤销恢复），大的撤销与恢复走后台任务时先放开任务的客户端（[M8/P4 的移交](M8-P4-background.md)第 1 条）。

## 处理进展

- M9/P2、P3（2026-10-19 至 10-21，合并 `804c3561`、`f3854b9a`、`f19d0b6f`、`43b03b36`）（[M9 总设计](../00-M9-design.md) 4.12）：
  - 第 1 项：`schedule()`（`page/domain/revert_schedule.go`）代替 `round()`（Go 里不再有 `round()`）：每一轮放回、删除、恢复、写回前提成立的步；一轮什么都没做时取第一条适用的后备：把占着键的节点原地改成临时名称；否则把待放回、在待删除的节点之下的 id 最小的那个带临时名称移到根下；第三条留作防御。轮数的上限（开始时待做的步数加 2N + 1）在代码里，过了答 `ErrRevertUnordered`（没有问题的码，缺陷是 500）。测试：`TestScheduleRevertsEachBatchShape`、`TestAScheduleTakesMoreRoundsThanItsSteps`、`TestTheSecondFallbackTakesTheLeastFirst`、`TestTheSecondFallbackPicksTheNodeItFrees`、构造的 `TestTheThirdFallbackMovesAPlaceOffTheRoot`、`TestTheSecondFallbackMovesOnlyANodeToPlace`、`TestAFallbackRenamesANodeAsideOnce`、`TestScheduleAnswersUnorderedForADefect` 与 `TestScheduleEndsAndPutsTheNotebookBack`。
  - 第 2 项：既移动又改名的放回与每个临时的步都是一条 `RelocateNode`，经 `Unit.place`（`page/app/unit_revert.go`）；测试：`TestTheRevertOfEachBatchShape` 的"移动加改名越过一个兄弟"一行、`TestARevertPutsBackAnMCPMoveThatRenames`。
  - 第 3 项：`AsideName`（`page/domain/name.go`）：" ~"加节点 id 的后 8 个十六进制字符（不是这一份写的前 8 个：前面是 UUIDv7 的毫秒，一个 batch 建的节点都相同），冲突时 32 个、再接 `-2`……，避开现在的与 S\* 的键，照 `Numbered` 截短（246 字节起，更正这一份写的 250），保留附件的扩展名，扩展名长到主干一个字符都放不下时照页的名称（M9 总设计第 11 节主持者的决定第 59 条）；撤销合并的条目里没有临时名称。测试 `TestAsideName`、`TestNumbered`、`TestAStepAsideTakesNoKeyOfS` 与"经第三个名称互换"一行。
  - 第 4 项：恢复等它的父组（`TestARestoreWaitsForTheGroupItGoesUnder`）；"恢复到另一组之下"经任何接口都做不出，以 SQL 种下的那一种由 M8 的 L 读成一组、撤销答 409 `changed`、什么都不写（主持者的决定第 56 条；`TestTheRevertOfEachBatchShape` 的种下的一行）：与这一份要的"排在那一组之后"不同，照第 56 条。
  - 第 5、7 项：先恢复、再在恢复回来的版本号上写回（`RevertOp.Current` 作 `Base`），正文按（页、哈希）解析（`RevertRead.Parsed`、`DecidedContents`），一页的两份正文在 `MinParseBudgetBytes` 下也解析得了（`decidedAll`，第 58 条）；撤销的撤销先写回、再删除。测试：`TestTheRevertOfEachBatchShape` 的"写过正文再删除的页"与"它的撤销的撤销"、`TestTheRevertOfALonePagesTwoContentsAtTheLeastBudget`、`TestDecidedAllGivesItsSharesBackBeforeATakeThatCannotFit`。
  - 第 6 项：B3 不许先改名再删除（`deletesWhatItChanged`，`TestPlanBatchDeletesNothingItChanged`，`TestTheMCPBatchIsRefusedWhole` 的 B3 一格）；合并的条目留删除的版本号（`TestADeletionsItemHoldsItsPagesRevision`）。
  - 第 8 项：`RevertPlan.Steps()` 数临时的步（`TestStepsCountTheStepsAside`）；请求里的撤销在读正文之前与锁下核对 `MaxRevertSteps`；后台的撤销按批核对、只在段之间切（[M8/P4 的移交](M8-P4-background.md)第 2 条）。
  - 第 9 项：真的库上的 `TestTheRevertOfEachBatchShape`（互换、改名之后用旧名、删除之后用同名、移入新建的文件夹、移动加改名的中间态、审查的反例、246–255 字节的名称与附件、写过再删除、撤销的撤销、上下相套的 409；M9 收尾加了层级的等待的两行，`14dec63c`）；性质测试 `TestRevertingTheNewestChangesetsPutsTheNotebookBack` 带 agent 的 batch 与两种定向的形状；交错 `TestTheMultiRoundRevertMeetsAWrite`。
  - 另：`changesets.renames`、`moves` 由单元结束时合并的改动设上（`Unit.markMoves`），迁移 00045 补值（`TestTheChangesetsRenamesCountEveryItem`、`TestTheChangesetsMovesAreBackfilled`）；先建后删、删除再恢复照 [M4/P2](M4-P2-unit-merge.md) 的处理进展；`log` 照 `ListChangesets`（P2）；batch 的说明经 `UnitSpec.Message`（P3）；agent 的历史工具只读（`log`、`changeset`、`versions`；M9 总设计第 11 节负责人的决定第 1 条）。
- M9 收尾（2026-10-23）：核对过各项的落实（[M9 收尾审查](../reviews/M9-closeout-review.md)"handoff"，C 逐项对照代码），关闭。
