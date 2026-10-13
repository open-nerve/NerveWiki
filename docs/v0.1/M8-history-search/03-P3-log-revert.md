# M8/P3 日志与整组撤销：架构设计 & 实施规划

## 0. 分两部分

照 P1、P2 的做法分成 A（服务端：迁移、截掉的标记与空变更集的清理、日志与单个变更集的读、整组撤销、被撤销的变更集不再被并入、契约、权限）与 B（前端：历史页、单个变更集页、撤销的确认框与冲突、左栏的"历史"、页面菜单与版本页的链接；e2e HI3、HI4），各自一个分支、一次审查与修复核对、一次合并。B 以 A 合并之后的契约为准，并在 P2B 合并之后开始：左栏的入口加进 P2B 的 `NotebookPlaces`，列表用 P2B 的 `readPages`，同一分钟的同名行用 P2B 的 `twinsOf`、`shortId`，撤销删除之后树的重新显示靠 P2B 的 `removed` 规则。

合并的次序：P2B 先于 A；A 的网页改动（契约带来的门禁：文字、`problem-messages.ts`、`transfer-report.tsx`）在 P2B 合并之后变基到它之上。

## 1. 基线

- A 从 `main` 的 `613adfc` 开始（P1 合并于 `12fc5d7`、`df306e3`，P2A 合并于 `25d250a`）；P2B 在 `m8-p2b`（`4daf892`，修复核对中），B 在它与 A 合并之后开始。
- **变更集**：`changesets` 已有 `kind` 的五种、`reverts_id`（`CHECK ((kind = 'revert') = (reverts_id IS NOT NULL))`）与 `trimmed`（没有谁设它，00034）；索引只有 `changesets_notebook_id_idx (notebook_id)` 与 `deleted_at` 的部分索引（00014），没有日志的次序。`changesets.notebook_id` 是 `notebooks` 的外键 `ON DELETE RESTRICT`。
- **条目与版本**：`changeset_items` 与 `page_revisions` 各有唯一键 `(changeset_id, node_id)`（一个变更集的条目、版本按节点 id 的次序就是它），另有 `changeset_items (node_id)`、`page_revisions (node_id, revision)`；`restored`、`revision` 只许在"前"为空、"后"不空的条目上（00036 的 `changeset_items_restored_check`）。删除的条目没有页的版本号。
- **单元**：`Write.Kind`、`Write.RevertsID` 已在（P1），`Run` 拒绝没有 `RevertsID` 的撤销；并入别的变更集时 `ensureChangeset` 在第一次写时才 `LockChangeset`（`FOR NO KEY UPDATE`），不知道它是否被撤销过。条目一步一条 `RecordItems`，合并时保留第一次的 `restored`、`revision`；`RecordRevision` 的版本链守卫拒绝时是未分类的错误（P1A 审查 C4）。`Unit.Delete` 经 `DeleteNodes` 标根、记统计，`stray` 拒绝先删子孙再删祖先。`MoveNode` 带着节点现在的名称移动；`RenameNode`、`MoveNode` 撞上兄弟的唯一索引答 `domain.ErrTitleTaken`，`Unit.WriteContent` 基不符答 `domain.ErrRevisionMismatch`，两者都是已分类的 409。
- **恢复的部件**（P2A）：`TrashGroup`（按层、按参数规划）、`TrashContents`、`TrashHistory`（按参数规划）、`ContentParser.DecidedAll(ctx, contents, tooLarge)`、`RestoreNodes` 的五个数、`Change.Restored`、asset 观察者的行数（不符时组合根答 `page.ErrTrashChanged`）；根上记着组的 `trash_pages`、`trash_assets`（00035）；界线 `MaxUnitPages`/`Bytes`/`Links`、`MaxUnitNodes` = 10,000、`MaxUnitHistory` = 400,000。
- **守卫与参与者**：编辑锁拒绝不在持锁会话里的正文写（自己在别处编辑也拒绝），拒绝删除别人持锁的页；改名、移动、恢复放行。M6 的改写只在 `Options.UpdateLinks` 时动作。
- **清理**：条目 → 版本 → 正文 → 节点 → 变更集，每批 1,000 行，答回的行数少于一批时停下；变更集只清理随笔记本删除的（`deleted_at < before`）。
- **transfer**：导入的后续单元经 `ImportSpec.Changeset` 并入第一个单元的变更集；`treeError` 只套在单元里的 `CreatePage`、`CreateAsset` 上，`transferTree.Import` 原样返回 `Run` 的错误；`importRun.unit` 把既不在 `do` 里失败、又不是 `*shared.Error` 的错误当作结果未知（可能已提交），不删这一批已写入存储的文件，留给孤儿清扫。
- **前端**：P1B 已有变更集类型的文字；版本页显示变更集的类型、执行者与时刻，没有链接（P1 的移交：P3 从版本页链到单个变更集页）。P2B 有 `lib/twins.ts`（`twinsOf`、`shortId`）、预览的 `nodeIdPattern`（地址里不是 UUID 的 id 直接 `NotFoundPage`）、`BreadcrumbTrail`、`watchReader`；`NotebookPlaces` 的 `nav` 的名称键是 `trash.places`。

## 2. 目标与范围

做（M8 设计 4.1、4.2、4.7、4.8、4.9、4.11、4.13、4.14 的日志与撤销部分）：

- 迁移：日志、按人、`reverts_id`、`trimmed` 的索引；删除的条目记页的版本号。
- 清理：条目与版本的清理同时锁住所属的变更集、设上 `trimmed`；`trimmed` 而条目与版本已删光的变更集的清理。
- 读：日志（按人、客户端、页面筛选）、单个变更集、明细。
- 写：整组撤销（按状态比较、对 S\* 一次核对、单轮应用），类型 `revert`；超过一个单元答 422（后台进行在 P4）。
- 被撤销的变更集不再被并入：并入的单元在开始时发现它，导入的任务停下并报告、删掉这一批的文件。
- 权限 `changeset.revert`；契约的四个操作、两个扩展成员与 transfer 的新失败。
- 前端：历史页、单个变更集页、撤销的确认框与冲突；左栏"历史"；页面菜单"查看这一页的历史"；版本页链到变更集；e2e HI3、HI4。

不做：后台的撤销与恢复（P4，422 之后的"在后台进行"）；搜索（P5）；`batch` 与它要的多轮应用、临时名称（M9，理由见 3.5，移交见第 7 节）。

### 2.1 对 00 号文档的修订

P3 的文档提交随这份文档一起改 `00-M8-design.md`（行号指 `613adfc` 的；理由在本文各节）：

1. **删除的状态不比删除的时刻**（4.8 :153；4.2 :87）："删除的还要 `deleted_at` 等于 C 里删除它的那一项的时刻"改为：C 删除的节点仍在回收站里；每个最上层（"前"的父节点不是 C 删除的节点）各是一项的根，它的组恰是 C 删除的、它之下的节点，组里的其余各在 C 删除它时的父节点与次序之下；每页回收站里的正文的哈希等于 C 删除它时的（附件没有正文，不比）。为此**删除的条目也记页的版本号**（4.2 `changeset_items` 一行的约束随之改：`restored` 仍只在恢复上，`revision` 可以在新建、恢复与删除的条目上；合并成删除时取删除的版本号）。理由：撤销一次恢复会以撤销的时刻把这一组再删一次（回收站里的项不改名、不移动，不能"放回"原来那一项），按时刻比较时"撤销这次恢复、再撤销更早的那次删除"永远冲突，第 1 节目标 2 的"从新到旧连续撤销"与第 9 节的性质测试（操作里有删除与恢复）都做不到（3.5）。
2. **S\* 里的恢复与删除**（4.8 :157）：C 恢复的节点在 S\* 里不再活着：撤销以它的时刻把它们（从最上层）删除，名称与位置是恢复之后的（序号、根下的位置留着），回收站的期限从撤销时算起。C 删除的节点整组回来，每个节点回到 C 删除它时的父节点、名称与次序，不改放根下、不加序号，放不回就冲突（第 11 节第 6 条照旧）；回来时用的是这一组现在的删除时刻。
3. **`revertChangeset` 的码的次序**（4.14 :238）：在解析之前未加锁地对 S\* 核对一遍，冲突的 409 排在 503 之前（冲突的撤销不去占解析的预算，P4 开任务之前同样要这一遍）。C 删除的每一组先核对根（在回收站里、是根、根上记下的页数与附件数等于 C 删除的、它与它之下的节点里页面与附件的数目），不符的不读组、算 `changed`：组的大小只对确是 C 的那一组答 422。照 `restoreNode` 有两个例外：链接数的 422 要解析之后才知道，可能排在 503 之后；要比较的版本或要写回、恢复的正文未加锁时已被清理拿走，答 409 `page.changeset_trimmed`，排在 422 与冲突之后、503 之前。锁下再核对一遍（`page.changeset_trimmed`、`page.revert_too_large`、`page.revert_conflict`），最后是守卫的 `page.locked`。撤销里的恢复撞上防线（`RestoreNodes` 的计数、附件的行数）答 409 `page.changeset_trimmed`，不是契约之外的 `page.trash_changed`；单元里别的已分类的错误（名称被占、版本号不符）在撤销里是缺陷，答 500。完整的次序见 3.6。
4. **日志的索引不带 `WHERE`**（4.2 :86）：`(notebook_id, created_at DESC, id DESC)` 去掉 `WHERE deleted_at IS NULL`，于是 `changesets_notebook_id_idx` 可以去掉。核对过别的用法：`notebooks` 的清理删除一本笔记本时，外键 `RESTRICT` 的核对按 `notebook_id` 找 `changesets`、不带 `deleted_at`，部分索引用不上它，只剩整表扫描；`DeleteNotebooksPages`、`NotebookActivities` 以 `notebook_id = ANY(…) AND deleted_at IS NULL` 读，前缀相同。按人的索引仍是部分索引（只有日志读它）。
5. **日志的每条与单个变更集**（4.7 :141–143）：计数是 `node_count`（条目，即树的变化）与 `content_count`（版本行，即写了正文的页），照契约已有的写法"数到 1,000，1,000 表示这么多或更多"（`linking.yaml` 的反链计数）；单个变更集（`getChangeset`，4.7 :143）另带 `renames`、`moves`：数到 1,000 的那些条目里有没有改名、移动（网页的确认框据此提示链接不跟着改回、冲突里据此给"改回原名"，不必翻遍明细；日志的每条不带，一页 100 个大的变更集要读约 10 万行条目）；`reverts` 在被撤销的那一个已被清理时为空（那时撤销自己也只剩空壳，3.3）；`reverted_by` 至多 100 个、新的在前；`page_id` 不是这本笔记本的节点（含已被清理的）答 404 `page.not_found`，同 4.12 的约定与 `listAssets` 的 `parent_id`。
6. **冲突的每项**（4.8 :169、:166；第 3 节第 3 条 :51）：`name` 是这个节点在 S\* 里的名称（`title_taken` 即被争的名称），S\* 里不在的节点是它现在的名称；`title_taken` 另带占着名称的节点 `holder_id`（4.13 要链到它）；`children_added` 改为"C 新建或恢复、S\* 里不存在的节点下有不在 C 里的活着的节点"，列出的是之后放进去的那个节点（它会被一起删掉），不是 C 的那个；第 3 节第 3 条的"新建的节点下有别的变更集放进的节点"同改。
7. **正在写入的变更集**（4.8 :175）：并入的单元在 `Run` 加锁之后、做任何事之前锁住并核对要并入的变更集（不是第一次写时），被撤销过（撤销又被撤销的也算）答 page 的哨兵错误 `ErrChangesetReverted`；transfer 在 `Tree.Import` 的结果上接住它，算确定的拒绝（什么都没提交，这一批的文件删掉），导入的任务以新的失败 `changeset_reverted` 停下（transfer 契约的 `TransferFailure` 加这一项，第 5 节的表一并写）。在第一次写时才核对，导入的下一批会先因父节点已被撤销删掉而以 `tree_changed` 停下，原因不对。
8. **撤销的答复**（第 5 节 :259）：`{changeset_id, changed}`，同还原版本的形状；什么都不用改时（例如一次编辑会话最后写回了原来的字节）不写，`changeset_id` 为空、`changed: false`。
9. **大小**（4.9 :181）：撤销的树的操作数就是 C 的条目数（新建的被删、删除的回来、恢复的被删、改名与移动改回，各一个节点），对 `MaxUnitNodes`；放回一个节点同 `Unit.Move`，子树里的节点各一项交给观察者，代价与子树成正比、不计入（原来的移动也是这样，撤销不比它多）；写回的历史是要恢复的各组的版本与条目（`TrashHistory`），对 `MaxUnitHistory`；撤销里的删除与 `DeleteNode` 一样没有历史的界线（M12 的性能移交第 14 项已写）。撤销比恢复多读整本笔记本活着的树（3.5），P3A 实测（3.11）：与 C 成正比的部分在界线处超过 5 秒时撤销另取自己的界线；10 万个节点的笔记本里是读树超过时，改法是只读附近的节点（M12 的移交），不降界线。
10. **清理的写法**（4.2 :95）：标记与删除在一条语句里，删除在最外层（`:execrows` 答删掉的行数，A-M2），标记在数据修改的 CTE 里；空变更集由变更集的清理器在删完随笔记本删除的之后接着清理（3.3）。
11. **单轮应用**（4.8 :171；第 9 节 :338；Phase 表 :281；"移交的落实" :296；"写给后面的 M" :302）："按依赖逐个应用"改为：P3 只做一轮，次序是放回 → 删除 → 恢复 → 写回正文；`PlanRevert` 模拟这一轮，每一步的前提在它那一刻成立，不成立是缺陷（未分类的错误）。M8 的每个变更集只有一种操作（撤销是被撤销者的反面），一轮做完（设计审查 A 核对过，找不到反例）；之后发生的互换名称、移入新建的文件夹、删除再新建同名在 S\* 里是 `title_taken`、`children_added`，以冲突答出。推后、多轮与临时名称交给 M9 的 `batch`（`M9-mcp/handoffs/M8-P3-revert.md`）：它们在数据库上有 P3 用不到、也测不到的问题（`MoveNode` 带着现在的名称移动会撞上兄弟的唯一索引；临时名称会超出 255 字节的名称界线、丢掉附件的扩展名）。第 9 节"按依赖应用的单元测试（互换名称、移入新建的文件夹、删除再新建同名）"随之移到 M9；:296、:302 的"M8 的撤销按依赖应用，M9 的 batch 可以直接用"改为指向这份移交。
12. **用例的名字**（4.1 :75）：写的用例 `Revert` 改为 `RevertChangeset`。
13. **上限**（4.14 :243）：加"`reverted_by` 至多 100 个"。
14. **第 13 节**：加一行变更记录（P3 设计）。

### 2.2 需要负责人确认

无。第 11 节的 12 条都不受影响：第 3 条（被锁整体拒绝）、第 4 条（不经改写的参与者）、第 5 条（次序值也比）、第 6 条（撤销删除放回原位置、放不回就冲突）、第 8 条（截掉的整个不能撤销）照做。2.1 第 1、2 条是让第 1 节目标 2 成立的设计细节：撤销一次恢复之后，这一组在回收站里以撤销者、撤销的时刻列出，期限重新算起（它是一次新的删除），这一点写进 4.8。2.1 第 11 条把通用的应用次序从 M8 移到 M9：它不在第 11 节里，M8 的变更集用不到它。

## 3. A：服务端

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `server/migrations/sql/00037_page_changesets_log.sql` | 日志、按人、`reverts_id`、`trimmed` 的索引；去掉 `changesets_notebook_id_idx` |
| `server/migrations/sql/00038_page_changeset_items_deleted_revision.sql` | 删除的条目可以记 `revision`：约束与补值 |
| `page/domain/changeset.go` | `Changeset`、`ChangesetBrief`、`ChangesetFacts`（两个计数、`RevertedBy`）、`ChangesetMoves`（`Renames`、`Moves`）、`ChangesetCursor`（UTC 写）、`ChangesetFrom`、`LogFilter` 与 `ParseClientFilter`、`ChangeCursor`、`MaxLogCount` = 1,000、`MaxRevertedBy` = 100、`ChangesetItem`、`ChangesetVersion`（带 `BaseBytes`）、`VersionRef`、`ChangeRow` 与 `ChangeKind` |
| `page/domain/revert.go` | `ConflictReason`、`RevertConflict`、`MaxConflicts` = 100、`RevertState`、`RevertPlan`/`RevertOp`、`PlanRevert`、`ErrRevertUnordered`（未分类：一轮做不完） |
| `page/domain/errors.go`、`actions.go` | `ErrChangesetNotFound`、`ErrChangesetTrimmed`、`ErrRevertTooLarge`、`RevertConflicts(conflicts, count)`；`ActionRevert = "changeset.revert"` |
| `page/app/ports.go` | `History` 端口、`NodeNow`；`Changeset.Reverted`；`WriterDeps.History` |
| `page/app/unit.go` | `UnitSpec.Message`；`Run` 先锁要并入的变更集（`ErrChangesetReverted`） |
| `page/app/unit_restore.go`、`unit_revert.go` | 从 `Restore` 抽出 `bringBack`（变更、`apply`、`RestoreNodes` 与五个数），恢复与撤销共用；`Unit.Revert`、`place`；名称被占、版本号不符包成未分类的错误 |
| `page/app/history.go`、`list_changesets.go`、`get_changeset.go`、`list_changes.go`、`revert_changeset.go` | 共用的读（`readableChangeset`、每条的计数与前三项）；四个用例 |
| `page/adapter/postgres/queries/history.sql`、`history.go` | `History` 端口的查询与实现 |
| `page/adapter/postgres/queries/changesets.sql`、`changesets.go` | `RecordItems` 给删除的条目记版本号、合并成删除时取删除的；`LockChangeset` 答 `reverted` |
| `page/adapter/postgres/queries/purge.sql`、`purge.go` | 条目、版本的清理设 `trimmed`；`PurgeTrimmedChangesets`，`PurgeChangesets` 接着跑它 |
| `page/adapter/http/history.go`、`handler.go` | 四个处理器 |
| `page/module.go`、`tree_writes.go` | 接线（`History` 是 page 模块自己的存储，同 `Trash`、`Versions`）；导出 `ErrChangesetReverted` |
| `shared/error.go`、`platform/httpserver/apierrors.go`、`problem.go` | 扩展成员 `conflicts`、`conflict_count` |
| `access/domain/rules.go` | `changeset.revert` |
| `transfer/domain/report.go`、`transfer/app/import_ports.go`、`import_write.go`；`bootstrap/transfer.go` | `FailureChangesetReverted`、`ErrChangesetReverted`；`Tree.Import` 的结果经 `treeError`；`unit` 把它当作确定的拒绝 |
| `api/common.yaml`、`api/modules/page.yaml`、`transfer.yaml` | 四个操作与结构（`Changeset` 与 `ChangesetDetail`）、`ChangesetID` 参数、两个扩展成员、`changeset_reverted` |
| `web/apps/web/src/app/problem-messages.ts`、`i18n/messages/{en,zh-CN}.ts`、`pages/notebook/transfer-report.tsx`、`test/fakes.ts` | 契约带来的门禁：四个新码的文字、新失败的文字 |
| `bootstrap/history_test.go`、`interleavings_history_test.go`、`revert_property_test.go`、`revert_cost_test.go`、`permission_matrix_page_test.go`；`page/adapter/postgres/history_cost_test.go`、`history_plans_test.go`；`transfer/app/import_write_test.go`；`Makefile` | 整个程序、交错、性质测试、矩阵；基准与守住它们的成本测试（3.11）；计划测试（`planOf` 的副本）；导入停下的单元测试；不带竞态检测的一步 |

### 3.2 迁移

**`00037_page_changesets_log.sql`**：

```sql
-- +goose Up
-- A notebook's log, newer first (M8 design 4.7), and the foreign key's look-up from notebooks, whose purge's RESTRICT
-- check reads changesets by notebook_id alone: the index the log reads serves both, so it is not partial.
CREATE INDEX changesets_notebook_id_created_at_id_idx ON changesets (notebook_id, created_at DESC, id DESC);
CREATE INDEX changesets_notebook_id_created_by_id_created_at_id_idx
    ON changesets (notebook_id, created_by_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
-- What reverted a changeset: the log's reverted_by and a unit's merge (M8/P1 review C3).
CREATE INDEX changesets_reverts_id_idx ON changesets (reverts_id) WHERE reverts_id IS NOT NULL;
-- The changesets the purge trimmed, whose emptied ones it deletes.
CREATE INDEX changesets_trimmed_idx ON changesets (id) WHERE trimmed;
DROP INDEX changesets_notebook_id_idx;

-- +goose Down
CREATE INDEX changesets_notebook_id_idx ON changesets (notebook_id);
DROP INDEX changesets_trimmed_idx, changesets_reverts_id_idx, changesets_notebook_id_created_by_id_created_at_id_idx,
    changesets_notebook_id_created_at_id_idx;
```

- 按客户端筛选不建索引：取值很少，在日志的索引上过滤（3.11 的计划测试与成本测试核对稀有取值的最坏情形）。按页面筛选经条目与版本的 `node_id` 索引。
- `schema_test.go` 核对四个新索引的名字、旧的不在。

**`00038_page_changeset_items_deleted_revision.sql`**：

```sql
-- +goose Up
-- An item's revision is the page's revision as the item left it where its changeset wrote no version of it: a creation's,
-- a restore's, and from M8/P3 a deletion's, which the revert of the deletion compares the trash's content with
-- (M8/P3 design 3.5). restored stays a restore's alone.
ALTER TABLE changeset_items DROP CONSTRAINT changeset_items_restored_check,
    ADD CONSTRAINT changeset_items_restored_check CHECK (NOT restored OR (before_name IS NULL AND after_name IS NOT NULL)),
    ADD CONSTRAINT changeset_items_revision_state_check CHECK (revision IS NULL OR before_name IS NULL OR after_name IS NULL);
-- The deletions so far whose page is still in the trash at their time: its content there is the one deleted.
UPDATE changeset_items i SET revision = c.revision
FROM page_contents c
WHERE i.after_name IS NULL AND i.revision IS NULL AND c.node_id = i.node_id AND c.deleted_at = i.updated_at;

-- +goose Down
UPDATE changeset_items SET revision = NULL WHERE after_name IS NULL;
ALTER TABLE changeset_items DROP CONSTRAINT changeset_items_revision_state_check, DROP CONSTRAINT changeset_items_restored_check,
    ADD CONSTRAINT changeset_items_restored_check CHECK ((NOT restored AND revision IS NULL)
        OR (before_name IS NULL AND after_name IS NOT NULL));
```

- 补值只补"删除之后一直在回收站里"的（删除的条目的 `updated_at` 就是删除的时刻，M8 的删除一个单元写完）；之后恢复过的补不到，留空：撤销时这一页按 `changed` 冲突（3.5）。附件没有正文行，补不到，也不需要。v0.1 之前没有发布的实例，同 `trimmed` 不补记的理由（M8/P1 审查 C2）。
- 迁移测试：`TestChecksRejectCounterexamples` 加反例（改名的条目带 `revision`、删除的条目 `restored`）；补值的测试：迁到最新、`Down` 一步，以 SQL 写删除的节点、正文与条目（`revision` 为空，旧的约束只许这样；不经程序，程序的 `RecordItems` 已会给删除记版本号），一例仍在回收站、一例已恢复，再 `Up`，核对前者补上、后者为空。

### 3.3 截掉的标记与空变更集的清理

```sql
-- name: PurgeChangesetItems :execrows
-- The items deleted before before, each locked with its changeset without waiting (M8 design 4.2): a row another
-- transaction holds, or whose changeset another holds (a unit merging into it), waits for a later run, and the purge
-- never waits (v0.1 design 13.1, item 5). The changesets of live notebooks whose items it deletes are marked trimmed:
-- they can no longer be reverted whole. The deletion is the statement, so that its rows are what :execrows counts.
WITH doomed AS (
    SELECT i.id, i.changeset_id
    FROM changeset_items i JOIN changesets c ON c.id = i.changeset_id
    WHERE i.deleted_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(batch)
    FOR UPDATE OF i SKIP LOCKED FOR NO KEY UPDATE OF c SKIP LOCKED
), marked AS (
    UPDATE changesets c SET trimmed = true
    WHERE c.id IN (SELECT d.changeset_id FROM doomed d) AND c.deleted_at IS NULL AND NOT c.trimmed
)
DELETE FROM changeset_items i USING doomed d WHERE i.id = d.id;
```

`PurgePageRevisions` 同样的写法（`page_revisions r`）。要点：

- 拿不到变更集的行（导入的单元正以 `LockChangeset` 并入它）时，这些条目留到下一次；撤销自己不锁 C 的行（下面的"先读条目、后读标记"足够）。
- 只标活着的笔记本里的（`c.deleted_at IS NULL`）；已标的不重写（不留死行）。标记与删除同一个事务：读得到少了的条目，就读得到标记。
- 撤销按"先读条目与版本、后读 `trimmed`"（READ COMMITTED，每条语句一个快照，M8 设计审查 A-M3）：一次清理在两次读之间提交，后一次读看得到它的标记；在后一次读之后提交的，删的只能是 C 碰过、删除超过保留期的节点的行，这样的节点在 S\* 的核对里本来就是冲突（`changed` 或 `expired`，一小时的间隔之内）。这一条两边（未加锁与锁下）都照做。时钟偏差超过一小时时由读不到的版本哈希与 `RestoreNodes`、附件的计数兜住（3.6，交错 10）。
- **空变更集**：

  ```sql
  -- name: PurgeTrimmedChangesets :execrows
  -- The trimmed changesets of live notebooks whose items and versions the purge has all deleted: nothing of them is
  -- left to read or revert. Through the partial index on trimmed; at any time, as nothing deleted them.
  DELETE FROM changesets WHERE id IN (
      SELECT s.id FROM changesets s
      WHERE s.trimmed
          AND NOT EXISTS (SELECT 1 FROM changeset_items x WHERE x.changeset_id = s.id)
          AND NOT EXISTS (SELECT 1 FROM page_revisions x WHERE x.changeset_id = s.id)
      LIMIT sqlc.arg(batch) FOR UPDATE SKIP LOCKED);
  ```

  `Store.PurgeChangesets` 先跑原来的（随笔记本删除的），不满一批时以剩下的批量跑它（同 `PurgeNodes` 的接续），答两者的和；`Purgers` 的表不变。一个空的 C 被删时，撤销它的 R 只碰过 C 的节点（3.5），这时也已是空的、`trimmed`，同一次或下一次清理删掉它：日志里不会长期留着一个 `reverts` 为空的撤销。
- 截掉了一部分、还剩条目的变更集每次清理被部分索引扫到一次（随这样的变更集增多而变慢）：3.11 量一次，写进 M12 的性能移交。
- 测试（适配器）：删一批时标记它们的变更集、已删笔记本的不标、拿着变更集的行时这些条目不删也不标（另一个事务 `FOR NO KEY UPDATE` 它）、`:execrows` 是删掉的条目数（批满时循环继续）；`PurgeTrimmedChangesets` 只删空的、`trimmed` 的。

### 3.4 读：日志、单个变更集与明细

`History` 端口（`page/app/ports.go`），未加锁或在单元的事务里：

```go
type History interface {
	// FindChangeset is the changeset id of a live notebook, with the brief of the one it reverts when that one is
	// still there; ErrNotFound for none.
	FindChangeset(ctx context.Context, id uuid.UUID) (domain.Changeset, error)
	// ListChangesets is the notebook's changesets the filter keeps, newer first, below the cursor, at most limit.
	ListChangesets(ctx context.Context, notebookID uuid.UUID, f domain.LogFilter, below domain.ChangesetCursor, limit int) (
		[]domain.Changeset, error)
	// ChangesetFacts are the changesets' counts, each to MaxLogCount, and their reverts, newest first, MaxRevertedBy
	// at most, by changeset.
	ChangesetFacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.ChangesetFacts, error)
	// ChangesetMoves is whether the changeset's first MaxLogCount items rename or move a node: the single changeset's.
	ChangesetMoves(ctx context.Context, id uuid.UUID) (domain.ChangesetMoves, error)
	// ListChanges are the changeset's changes by node id after after, at most limit; FirstChanges the first n of each.
	ListChanges(ctx context.Context, id, after uuid.UUID, limit int) ([]domain.ChangeRow, error)
	FirstChanges(ctx context.Context, ids []uuid.UUID, n int) (map[uuid.UUID][]domain.ChangeRow, error)
	// The revert's reads (3.6): the changeset's items counted to limit; its items and versions; its nodes as they
	// are now, alive or not, a root with its group's counts; the alive pages' contents' revisions and hashes;
	// versions' hashes and contents.
	CountItems(ctx context.Context, id uuid.UUID, limit int) (int, error)
	ChangesetItems(ctx context.Context, id uuid.UUID) ([]domain.ChangesetItem, error)
	ChangesetVersions(ctx context.Context, id uuid.UUID) ([]domain.ChangesetVersion, error)
	NodesNow(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]NodeNow, error)
	ContentsNow(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]ContentLock, error)
	VersionHashes(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef][]byte, error)
	VersionContents(ctx context.Context, refs []domain.VersionRef) (map[domain.VersionRef]string, error)
}
```

`NodeNow` 是 `AnyNode` 加根上记下的 `TrashPages`、`TrashAssets`（不是根时为空）。

**日志**：三条语句，各走一个索引，筛选的组合不让一条语句兼走几种路（缓存的语句的通用计划只能选一种）：

```sql
-- name: ListChangesets :many
-- A notebook's log newer first below the cursor (M8 design 4.7): a range of its index backward, whatever plan is kept;
-- the client a filter on it, its values few. The brief of the changeset a revert reverts by its key.
SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.message, c.created_by_id, c.created_at, c.updated_at,
    c.trimmed, v.kind AS reverts_kind, v.created_by_id AS reverts_by, v.created_at AS reverts_at
FROM changesets c LEFT JOIN changesets v ON v.id = c.reverts_id
WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.deleted_at IS NULL
    AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
    AND (sqlc.narg(client)::text IS NULL OR c.client = sqlc.narg(client)::text)
    AND (NOT sqlc.arg(all_mcp)::boolean OR c.client LIKE 'mcp:%')
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(max_rows);
```

- `ListChangesetsBy`：同上，另加 `AND c.created_by_id = sqlc.arg(actor)`，走按人的索引。
- `ListChangesetsOfPage`（有 `page_id` 时，可再带人与客户端）：

  ```sql
  WITH touched AS (
      SELECT i.changeset_id FROM changeset_items i WHERE i.node_id = sqlc.arg(page_id)
      UNION
      SELECT r.changeset_id FROM page_revisions r WHERE r.node_id = sqlc.arg(page_id)
  )
  SELECT …（同上的列）
  FROM touched t JOIN changesets c ON c.id = t.changeset_id LEFT JOIN changesets v ON v.id = c.reverts_id
  WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.deleted_at IS NULL
      AND (c.created_at, c.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
      AND (sqlc.narg(actor)::uuid IS NULL OR c.created_by_id = sqlc.narg(actor)::uuid)
      AND （客户端的两个条件，同上）
  ORDER BY c.created_at DESC, c.id DESC
  LIMIT sqlc.arg(max_rows);
  ```

  代价与这一页的历史成正比（每页读它全部的条目与版本的变更集 id，再按主键连接、排序取一页）：一页几万个版本时每页日志几十到几百毫秒，3.11 的成本测试量它；按页面的日志是少用的路，不另建跨表的结构。条目与版本不按 `deleted_at` 过滤：回收站里的页的历史照列。
- 游标 `ChangesetCursor{At, ID}`，`MarshalJSON` 以 UTC 写（同 `TrashCursor`）；第一页从 `ChangesetFrom()`（9999-12-31 与最大的 uuid）开始，不写"零表示不限"。
- `client` 的取值：`web`、`api`、`cli`、`mcp`（全部 MCP 客户端，`all_mcp`）、`mcp:<名>`（同 `changesets_client_check` 的规则），别的答 422 `validation_failed`（字段 `client`）；`ParseClientFilter` 在 domain，先拒绝不是合法 UTF-8 的（Go 的正则把坏字节读成 U+FFFD，`[^[:cntrl:]]` 会放过它，PostgreSQL 再以 500 拒绝；13.4 第 5 条的 `TestFreeTextParametersDoNotAnswer5xx` 会碰到）。

**每条的计数、撤销与前三项**（一页至多 100 个变更集，两条语句）：

```sql
-- name: ChangesetFacts :many
-- Each changeset's items and versions counted to 1,000 (M8 design 4.14), through its key on (changeset_id, node_id),
-- and the reverts of it, newest first, 100 at most, through the partial index on reverts_id.
SELECT u.id,
    (SELECT count(*) FROM (SELECT 1 FROM changeset_items i WHERE i.changeset_id = u.id LIMIT 1000) x)::integer AS node_count,
    (SELECT count(*) FROM (SELECT 1 FROM page_revisions r WHERE r.changeset_id = u.id LIMIT 1000) x)::integer AS content_count,
    ARRAY(SELECT r.id FROM changesets r WHERE r.reverts_id = u.id AND r.deleted_at IS NULL
        ORDER BY r.created_at DESC, r.id DESC LIMIT 100)::uuid[] AS reverted_by
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id) u;
```

**单个变更集的改名与移动**（只有 `getChangeset` 读，日志的每条不读）：

```sql
-- name: ChangesetMoves :one
-- Whether a changeset's first 1,000 items rename or move a node, which the web's revert warns of (links written to
-- the new place stay) and its conflicts offer to undo by hand: the single changeset's alone, as a log page of 100
-- large changesets would read some 100,000 item rows for it.
SELECT coalesce(bool_or(i.before_name <> i.after_name), false)::boolean AS renames,
    coalesce(bool_or(i.before_name IS NOT NULL AND i.after_name IS NOT NULL
        AND i.before_parent_id IS DISTINCT FROM i.after_parent_id), false)::boolean AS moves
FROM (SELECT before_name, after_name, before_parent_id, after_parent_id FROM changeset_items
      WHERE changeset_id = sqlc.arg(id) LIMIT 1000) i;
```

- 一次至多读一个变更集的 1,000 行条目。M8 的变更集里改名、移动各自只有一项（或撤销它们的那一项），数到 1,000 不会漏；M9 的 `batch` 第 1,000 项之后的改名不算，同计数的"或更多"。日志的每条不带它们：一页 100 个大的变更集要读约 10 万行条目（计数只读索引）；日志的行以前三项为摘要，撤销只从单个变更集页发起。

```sql
-- name: ListChanges :many
-- A changeset's changes by node id after after (M8 design 4.7): its items' and its versions' node ids, each a range of
-- its key, at most max_rows of each, merged; then each one's item, version, node and parents' names by key. A node
-- the purge took took its item and version with it (the foreign keys, the purge's order): every row has its node.
WITH ids AS (
    SELECT node_id FROM (
        (SELECT i.node_id FROM changeset_items i WHERE i.changeset_id = sqlc.arg(id) AND i.node_id > sqlc.arg(after)::uuid
            ORDER BY i.node_id LIMIT sqlc.arg(max_rows))
        UNION
        (SELECT r.node_id FROM page_revisions r WHERE r.changeset_id = sqlc.arg(id) AND r.node_id > sqlc.arg(after)::uuid
            ORDER BY r.node_id LIMIT sqlc.arg(max_rows))
    ) u ORDER BY node_id LIMIT sqlc.arg(max_rows)
)
SELECT ids.node_id, n.kind, n.name, n.deleted_at,
    (i.node_id IS NOT NULL)::boolean AS has_item, i.before_parent_id, bp.name AS before_parent_name, i.before_name,
    i.before_sort_order, i.after_parent_id, ap.name AS after_parent_name, i.after_name, i.after_sort_order,
    coalesce(i.restored, false) AS restored, r.base_revision, r.revision
FROM ids
JOIN nodes n ON n.id = ids.node_id
LEFT JOIN changeset_items i ON i.changeset_id = sqlc.arg(id) AND i.node_id = ids.node_id
LEFT JOIN page_revisions r ON r.changeset_id = sqlc.arg(id) AND r.node_id = ids.node_id
LEFT JOIN nodes bp ON bp.id = i.before_parent_id
LEFT JOIN nodes ap ON ap.id = i.after_parent_id
ORDER BY ids.node_id;
```

- 每次至多读 `2 × max_rows` 个索引条目，与变更集的大小无关（一次 5 万个节点的导入也是）。父节点已被清理时名称为空（`parent_id` 不空、`parent_name` 为空：已清理；`parent_id` 为空：根下）。
- `FirstChanges`：同一个 `ids … SELECT` 放进 `CROSS JOIN LATERAL`，对一页的每个变更集取前 3 项，一条语句。前三项按节点 id，是摘要；网页不靠它判断改名与移动（单个变更集的 `renames`、`moves`）。
- `ChangeKind`（domain）：没有条目是 `content`；没有"前"时 `restored` 是 `restored`、否则 `created`；没有"后"是 `deleted`；父节点不同是 `moved`（名称也可能不同，前后的名称都在）；名称不同是 `renamed`；其余是 `reordered`。版本行另以 `content` 给出（`base_revision` 为空即新建）。

**撤销的读**：

```sql
-- name: CountItems :one
SELECT count(*)::integer AS items FROM (SELECT 1 FROM changeset_items WHERE changeset_id = sqlc.arg(id) LIMIT sqlc.arg(lim)) x;

-- name: ChangesetItems :many
SELECT node_id, before_parent_id, before_name, before_sort_order, after_parent_id, after_name, after_sort_order,
    restored, revision
FROM changeset_items WHERE changeset_id = sqlc.arg(id) ORDER BY node_id;

-- name: ChangesetVersions :many
-- A changeset's versions with the size of the one each is based on: what a revert writes back.
SELECT r.node_id, r.base_revision, r.revision, r.content_hash, b.byte_size AS base_bytes
FROM page_revisions r LEFT JOIN page_revisions b ON b.node_id = r.node_id AND b.revision = r.base_revision
WHERE r.changeset_id = sqlc.arg(id) ORDER BY r.node_id;

-- name: NodesNow :many
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at,
    deleted_at, trash_root, trash_pages, trash_assets
FROM nodes WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ContentsNow :many
SELECT node_id, revision, content_hash, byte_size FROM page_contents
WHERE node_id = ANY(sqlc.arg(ids)::uuid[]) AND deleted_at IS NULL;

-- name: VersionHashes :many
SELECT r.node_id, r.revision, r.content_hash
FROM (SELECT unnest(sqlc.arg(node_ids)::uuid[]) AS node_id, unnest(sqlc.arg(revisions)::integer[]) AS revision) v
JOIN page_revisions r ON r.node_id = v.node_id AND r.revision = v.revision;
```

`VersionContents` 同 `VersionHashes`，读 `r.content` 与 `r.content_hash`（至多 100 页、8 MiB，3.6 的界线之后才读）。`ids` 至多是 C 的条目（10,000）：三条数组语句按 P2A 的教训在界线上比较缓存的通用计划与按参数的（3.11），明显更慢的经 `s.planned(ctx)`。

**单元的两处改动**（`changesets.sql`）：

- `RecordItems` 的 `revision` 一列改为：

  ```sql
  CASE WHEN u.has_after THEN NULLIF(u.revision, 0)
      ELSE (SELECT c.revision FROM page_contents c WHERE c.node_id = u.node_id AND c.deleted_at = sqlc.arg(now)::timestamptz) END
  ```

  删除的页的正文行刚由同一单元的 `DeleteNodes` 以单元的时刻删除（`apply` 先写、后记条目），主键读一行；附件没有正文行，为空。不改 `Change`（观察者把 `Revision` 当作写了正文），不加一次读。`DO UPDATE` 加一列 `revision = CASE WHEN excluded.after_name IS NULL THEN excluded.revision ELSE changeset_items.revision END`：合并成删除时取删除的版本号（一个单元里先改名再删除同一页，M9 的 `batch` 会这样），别的合并保留第一次的（新建、恢复的）；`restored` 照旧保留第一次的，00038 的两个 `CHECK` 都满足。3.11 重跑 P2A 的 `BenchmarkTrashDelete`（5 万个节点）核对它的代价。
- `LockChangeset` 另答 `reverted`：

  ```sql
  SELECT c.id, c.notebook_id, c.kind, c.reverts_id, c.client, c.created_by_id,
      EXISTS (SELECT 1 FROM changesets r WHERE r.reverts_id = c.id AND r.deleted_at IS NULL) AS reverted
  FROM changesets c
  WHERE c.id = sqlc.arg(id) AND c.deleted_at IS NULL
  FOR NO KEY UPDATE OF c;
  ```

**用例**：

- **`ListChangesets`**（`GET /notebooks/{notebook_id}/changesets?actor&client&page_id&limit&cursor`），码的次序同 `listAssets`：游标先核对（400；`actor`、`page_id` 不是 uuid 时平台答 400）；笔记本的 `WorkspaceOf` → `page.history`（看不到 404 `notebook.not_found`）；有 `page_id` 时 `NodesNow([page_id])`：不在、或不是这本笔记本的 → 404 `page.not_found`（回收站里的照列）；`client` 与 `limit`（422）；按筛选选语句，读 `limit + 1` 条；`ChangesetFacts`、`FirstChanges(ids, 3)` 各一次；执行者与被撤销者的显示名经 `Names` 一次。
- **`GetChangeset`**（`GET /changesets/{changeset_id}`）：`readableChangeset`：`FindChangeset`（没有、笔记本已删 → 404 `page.changeset_not_found`）→ 笔记本的 `WorkspaceOf`（已删 → 同 404）→ `page.history`（没有角色 → 同 404）；`ChangesetFacts`、`FirstChanges` 与 `ChangesetMoves` 各一次；形状是日志的一条另加 `renames`、`moves`。
- **`ListChanges`**（`GET /changesets/{changeset_id}/changes?limit&cursor`）：游标（`ChangeCursor{After}`）先核对 → `readableChangeset` → `limit` → 读 `limit + 1` 项。
- 读不记日志。

### 3.5 撤销：按状态比较与 S\*

一切判断在 domain 的纯函数 `PlanRevert(RevertState) (RevertPlan, error)` 里，用例与单元只负责读与写，未加锁的预先核对（3.6 第 7 步）、锁下的核对与 P4 的任务共用它。`RevertPlan` 带冲突（前 100 项与总数）、`Missing`（读不到版本哈希的页）与 `Ops`；有冲突或 `Missing` 时没有 `Ops`；`error` 只有 `ErrRevertUnordered`（一轮做不完，缺陷）。

**输入**（`RevertState`）：C 的条目（`ChangesetItem{NodeID, Before, After *TreeState, Restored bool, Revision int}`）与版本行（`ChangesetVersion{NodeID, Base *int, BaseBytes int, Revision int, Hash []byte}`）；C 碰过的节点现在的样子（`NodesNow`：活着、在回收站里与删除的时刻、是否根与根上的页数、附件数，不在即已被清理）；活着的页的正文的版本号与哈希（`ContentsNow`）；要比较的版本的哈希（`VersionHashes`，见下）；C 删除的每一组现在的成员（`TrashGroup`，只读通过了根的核对的，3.6 第 6 步）；笔记本活着的树（`ListNodes`，C 有条目时才读：只写正文的 C 不需要树）；单元的时刻与保留期。

`VersionHashes` 读的版本：C 恢复的页的 `(页, item.Revision)`；C 删除的页的 `(页, item.Revision)` 与 `(页, 组里它现在的版本号)`（回收站里的正文的哈希就是它的版本号那一行的：每个正文的版本号都有一行版本，`checkPages` 的"正文是最新的版本"）；要写回的页的 `(页, Base)`。按哈希而不按版本号比：撤销一次编辑之后版本号变了、字节没变，正是要成立的情形。读不到的（清理拿走了这一页的版本）不算冲突，记进 `Missing`，3.6 答 409 `page.changeset_trimmed`。

**C 写出的状态**（L）与**之前的状态**（S\*），按节点：

| C 对节点做了什么 | L（现在必须是这样，否则 `changed`） | S\* |
|---|---|---|
| 新建（条目没有"前"，不是恢复） | 活着，父节点、名称（原样的字符串）、次序值等于"后"；页的正文哈希等于 C 的版本行的 | 不存在：撤销删除它（从最上层删） |
| 恢复（条目 `restored`） | 同上；正文哈希等于版本 `item.Revision` 的（恢复不写版本行） | 在回收站里：撤销删除它，名称与位置是现在的 |
| 删除（条目没有"后"） | 在回收站里；每个最上层（"前"的父节点不是 C 删除的节点）是一项的根，它的组恰是 C 删除的、它之下的节点；其余各在"前"的父节点与次序之下；页的回收站里的正文的哈希等于版本 `item.Revision` 的（没有 `revision` 的旧条目：页面算 `changed`，附件没有正文、不比）；最上层的删除时刻仍可恢复（否则 `expired`） | 活着，在"前"：整组以它现在的删除时刻恢复，每个节点写回"前"的名称，最上层写回"前"的父节点与次序 |
| 改名、移动、重新排序（前后都有） | 活着，在"后"；有版本行时正文哈希等于它的 | 活着，在"前" |
| 只写正文（只有版本行） | 活着；正文哈希等于版本行的 | 活着，在现在的位置；正文是版本 `Base` 的 |
| 不是 C 碰过的 | — | 照现在 |

- **按状态而不按时刻或版本号**：撤销把旧的字节写成新的版本、把恢复了的再删一次，按版本号或删除的时刻比较时，从新到旧的连续撤销会永远冲突（00 号文档 4.8 的理由，2.1 第 1 条把它推到删除上）。所以正文比哈希，树比活着与否、父节点、名称、次序值，回收站里的组比成员与位置。连续撤销因此成立：从新到旧逐个撤销时，轮到 C 的时候它之后的变更集都已撤销，C 碰过的节点回到 C 写出的状态（每个撤销只碰它撤销的那个碰过的节点），C 的核对通过；其中撤销一次恢复（以新的时刻再删）之后，更早的那次删除的核对仍通过（组还是那些节点、在那些位置、正文没变）。
- 次序值照值比较：之后的重新排序是冲突，之前的撤销照原值写回它不冲突；兄弟被重新编号（M4：不记条目）会改它而显示为冲突，罕见（00 号文档第 10 节）。恢复的节点照原值放回，兄弟这期间被重新编号过时落在按值排的位置（同 P2 3.5 第 3 步）。

**对 S\* 一次核对**：S\* 的活着的节点 = 现在活着的 − C 里在 S\* 不存在的 + C 删除的（S\* 里回来）；C 的节点在 S\* 的位置，其余在现在的位置；C 的节点的键以 `shared.TitleKey` 重算（同恢复，`reindex` 只重算活着的）。逐项：

| 原因 | 核对 |
|---|---|
| `changed` | 上表的 L 不成立（每个节点一项） |
| `expired` | C 删除的一组，最上层现在的删除时刻已不可恢复（`domain.Restorable`，单元的时刻） |
| `parent_gone` | C 的节点在 S\* 活着，S\* 的父节点在 S\* 不活着（在回收站、已被清理） |
| `children_added` | 不是 C 的节点在 S\* 活着，父节点是 C 新建或恢复、S\* 里不存在的节点：列出这个节点（撤销会把它一起删掉） |
| `title_taken` | S\* 里同一父节点下两个活着的节点键相同，其中有 C 的、位置或名称与现在不同的：列出 C 的那个，`holder_id` 是另一个（优先不是 C 的） |
| `cycle` | 从 C 的移回或恢复的节点沿 S\* 的父节点往上回到它自己 |
| `too_deep` | C 的移回或恢复的节点，它在 S\* 的子树里有页的层级超过 10（附件不算一层，同 `Subtree.Height`） |

每项的 `name` 是这个节点在 S\* 里的名称（`title_taken` 即被争的名称），S\* 里不在的是它现在的名称。冲突按表的次序、再按节点 id 排，全部数出（`conflict_count`），答前 100 项（`MaxConflicts`）。核对在内存里：读整本笔记本活着的树是 O(笔记本)，与页面树的 `listNodes` 每次读的相同（3.11 量它在 10 万个节点的笔记本上的代价）；只在节点附近读要为每种组合（移进新建的文件夹、互换名称、恢复到被移走的父节点下）各写一种读法，超过预算时再改（M12 的移交）。

已知的局限：C 删除的一组里两个节点在 Unicode 升级之后键相同（P2 的恢复为此加序号），撤销 C 永远 `title_taken`，两边都是 C 的节点，页面树里改不了；冲突的提示给"从回收站恢复（恢复会加序号）"（4.4）。罕见，不另做。

**单轮应用**（`RevertPlan.Ops`，`PlanRevert` 在 S\* 合法时从现在模拟一轮得出，纯函数，可单测），次序固定：

1. **放回**（C 的节点在现在与 S\* 都活着而位置不同）：父节点或次序不同时一步移动，名称不同时一步改名，先移动后改名（`MoveNode` 带着现在的名称移动）；模拟按这两步的中间状态判：目标父节点此刻活着或是根、不在节点此刻的子树里、名称的键此刻在那里空着。
2. **删除**（S\* 不存在的 C 的节点里最上层的）：按 id，从上往下、一棵子树删一次（`DeleteNodes` 的 `stray`，P2 的移交）；此刻的活着的子树里没有 S\* 里活着的节点。
3. **恢复**（C 删除的各组）：按根的 id；根的 S\* 父节点此刻活着或是根、名称的键此刻空着。
4. **正文**：S\* 里活着、C 有版本行且 `Base` 不空的页，按页 id 的次序（13.1 第 5 条），写版本 `Base` 的正文，基于锁下读到的版本号。

哪一步的前提在它那一刻不成立，`PlanRevert` 答 `ErrRevertUnordered`（未分类，500）。理由（2.1 第 11 条）：M8 的每个变更集只有一种操作（撤销是被撤销者的反面：撤销一次导入删的几个最上层、撤销那次撤销恢复的几组，父节点都是活着的、不是 C 的节点，组之间没有依赖），一轮做完；推后、多轮、临时名称与"恢复到另一组之下"只有 M9 的 `batch` 用得到，在数据库上另有要解决的问题（M9 的移交），P3 的整个程序与性质测试都走不到它们，不在 P3 做。

### 3.6 撤销：用例与单元

**`RevertChangeset`**（`POST /changesets/{changeset_id}/revert`，请求体 `{message?}`，可以没有），码照 2.1 第 3 条修订后的次序：

1. `message`：有时要是 1–4,096 字节的 UTF-8、不含 NUL，否则 422 `validation_failed`（字段 `message`；`""` 也是）。
2. `FindChangeset`：没有、笔记本已删 → 404 `page.changeset_not_found`；`Writer.Allowed(spec)`（`changeset.revert`，`NotFound` 是 `ErrChangesetNotFound`）：没有角色 404、读者 403。
3. `trimmed` → 409 `page.changeset_trimmed`。
4. `CountItems(id, MaxUnitNodes + 1)` 超过 `MaxUnitNodes` → 422 `page.revert_too_large`（读条目之前，一次 5 万个节点的导入不读它的全部）。
5. `ChangesetItems`、`ChangesetVersions`，然后再 `FindChangeset`：`trimmed`（或这期间被清理删掉）→ 409 `page.changeset_trimmed`（先读条目、后读标记，3.3）。
6. `NodesNow`（C 的节点）；C 删除的每个最上层先核对根：在回收站里、`trash_root`、`trash_pages`、`trash_assets` 等于 C 删除的、它与它之下的节点里页面、附件的数目（按删除条目的"前"的父节点连成的树，类型取 `NodesNow` 的）；通过的才读组（`TrashGroup`，以它现在的删除时刻），不通过的不读、交给 `PlanRevert` 记 `changed`（后来又被包进更大的删除、恢复之后又放进几万个节点再删除的，都不先读一遍、也不以 422 盖住冲突）。要解析的页 = 要写回正文的页（版本行有 `Base`，大小是 `BaseBytes`）与要恢复的页（组里的页，大小来自组），每页一份（M8 的变更集里一页不会既要恢复又要写回；M9 见第 8 节）：`UnitTooLarge`（多于一页时 100 页、8 MiB）→ 422；各组的 `TrashHistory(ids, T, MaxUnitHistory + 1)` 合计超过 `MaxUnitHistory` → 422。
7. `ContentsNow`、`VersionHashes`、（有条目时）`ListNodes` → `PlanRevert`（时刻是 `Clock.Now()`）→ 有冲突 409 `page.revert_conflict{conflicts, conflict_count}`；`ErrRevertUnordered` 原样（500）。
8. `Missing` 不空 → 409 `page.changeset_trimmed`；`VersionContents`（各页的 `Base`）与 `TrashContents`（各组的页、它的时刻）：少了 → 409 `page.changeset_trimmed`（清理拿走了，同 P2 3.5 第 5 步，不解析空的正文）；`DecidedAll(ctx, contents, ErrRevertTooLarge)`：从大到小、等待合计有界，503 `server_busy`；多于一页时链接数超过 20,000 → 422。
9. `Writer.Run(UnitSpec{NotebookID, Action: changeset.revert, Tree: true, Client, NotFound: ErrChangesetNotFound, Kind: revert, RevertsID: C, Message}, u.Revert(ctx, RevertRead{Changeset: C, Parsed, Retention}))`。`Options` 留空：不经改写的参与者（00 号文档 4.8、第 11 节第 4 条）。`Parsed` 按页：解析过的正文 `{Hash, Content, Facts}`。
10. 单元答的 `page.ErrTrashChanged`（`RestoreNodes` 的计数、asset 观察者的行数：撤销里的恢复撞上防线）换成 409 `page.changeset_trimmed`：清理拿走了这些行就截掉了 C。
11. 答 `{changeset_id, changed}`；写了的记日志 `changeset reverted`（变更集、被撤销的、笔记本的 id，删除、恢复、放回、正文各几个，客户端；不记名称）。

**`Unit.Revert`**（`unit_revert.go`，笔记本行 `FOR NO KEY UPDATE` 之下）：

1. 重读第 5 步（条目、版本，再 C 的行）：`trimmed` 或不在 → 409 `page.changeset_trimmed`；条目超过 `MaxUnitNodes`（这期间又被并入）→ 422。
2. 重读第 6、7 步的状态（根的核对、组、正文、哈希、树），以单元的时刻 `PlanRevert` → 冲突 409（锁下的这一次是权威的；之前的一次只为在解析之前答出）；`Missing` → 409 `page.changeset_trimmed`。
3. 计划要写回或恢复的每页，`Parsed` 里要有它、哈希等于锁下要写的那一版的（写回：版本 `Base` 的；恢复：回收站里的）：没有或不等（这期间 C 被并入了新的正文，或这一页变了）→ 这一页算 `changed`，409。按哈希配对：两遍之间一页"恢复 → 编辑 → 撤销编辑 → 撤销恢复"（字节相同、版本号变了）不是冲突；写下的版本号取锁下读到的。
4. 逐个应用计划：
   - 放回：`u.place(ctx, id, state)`：一步 `OpMove`（同 `Unit.Move`，子树里的节点各一项同样的前后，好让观察者重新解析路径；`MoveNode` 写原样的次序值，不经 `placeAmong`），再一步 `OpRename`（`RenameNode`，键以 `TitleKey`）。不再做 `Move`/`Rename` 的核对：S\* 与这一轮已核对，兄弟的唯一索引是最后的防线。
   - 删除：`u.Delete(ctx, id)`（`DeleteNodes` 标根、记统计；条目带页的版本号，3.4）；守卫拒绝删除别人持锁的页。
   - 恢复：`u.bringBack(…)`（从 `Restore` 抽出的后半：每个节点一项没有"前"的变更，`Restored{DeletedAt: T, Asset}`，页带锁下的版本号与 `Facts`；`apply(OpRestore)`；`RestoreNodes(Restoration{Root: 最上层, ParentID、SortOrder: 它的"前", IDs, Names: 各自"前"的名称, NameKeys, DeletedAt: T})`；五个数照 P2 核对，不符 `ErrTrashChanged`）。
   - 正文：`u.WriteContent(ctx, ContentWrite{NodeID, Content, Facts, Base: 锁下读到的版本号})`：守卫拒绝任何活着的会话持锁的页（调用者自己在别处编辑也拒绝）→ 409 `page.locked`，带第一页的持锁人，整个单元回滚（第 11 节第 3 条）。与现在相同的字节不写。树的单元持笔记本 `FOR NO KEY UPDATE`，正文的写（`FOR SHARE`）进不来，版本号不会变。
   - **已分类的错误在这里是缺陷**：`place` 的 `ErrTitleTaken`（`RenameNode`、`MoveNode` 撞上唯一索引）与正文一步的 `ErrRevisionMismatch` 以 `%v` 包成未分类的错误（同 P2A 审查 B-4 对 `RestoreNodes` 的处理），答 500，不漏出 `revertChangeset` 契约之外的 409；`RecordRevision` 的版本链拒绝本来就是未分类的。`page.locked` 照原样（契约里有）。
5. 什么都没写（各步都与现在相同）时没有变更集，答 `changed: false`。例如一次编辑会话最后写回了原来的字节：撤销不写，会话的下一次保存照旧并入 C（C 没有被撤销过，3.7），这是预期的。

**`Run` 的改动**：`UnitSpec.Message` 由 `ensureChangeset` 写进新建的变更集（并入时不改别人的）；`spec.Changeset` 不空时，`Run` 在授权之后、`do` 之前 `LockChangeset`：不在 → 错误（原样），`reverted` → `ErrChangesetReverted`（3.7），别的核对照旧（类型、撤销的、执行者、客户端）。`ensureChangeset` 仍在第一次写时 `TouchChangeset`。加锁的次序：笔记本 → 变更集的行（v0.1 13.1 第 5 条记下，与清理的 `SKIP LOCKED` 并列）。

### 3.7 被撤销的变更集不再被并入

- `app.ErrChangesetReverted`（未分类的哨兵：只有任务并入，接口不会答它），`page.ErrChangesetReverted` 导出。`reverted` 是"被撤销过"：撤销又被撤销的变更集同样不再被并入（它碰过的节点的状态已由两次撤销定下，任务另起变更集即可）。
- 导入：
  - `transfer` 的 `app.ErrChangesetReverted`（`import_ports.go`）与 `domain.FailureChangesetReverted = "changeset_reverted"`。
  - `bootstrap/transfer.go`：`transferTree.Import` 答 `id, treeError(err)`（`Run` 的错误不经 `CreatePage`，只套在单元里的映射接不到它）；`treeError` 加 `page.ErrChangesetReverted → transfer.ErrChangesetReverted`。
  - `importRun.unit`：`errors.Is(err, ErrChangesetReverted)` 不算结果未知（`Run` 在 `do` 之前拒绝，什么都没提交），删掉这一批已写入存储的文件；`unitFailure` 报 `failed(FailureChangesetReverted, err)`（任务停下，报告里是已写的批的计数与问题）。
  - transfer 契约的 `TransferFailure` 加 `changeset_reverted`（"它写入的变更集在它运行时被撤销了，之后的节点没有写"），网页的报告文字一并加（门禁）。HTTP 适配器不核对取值，契约在第 5 步改之前第 2 步照样绿。
- P4 的恢复与撤销的任务并入的是自己的变更集，同一条路（第 8 节）。
- 并入的单元与撤销都是树的单元，笔记本的锁让它们一先一后：撤销在先，任务的下一批在 `Run` 里看到 `reverted`；任务的这一批在先，撤销在锁下读到的条目含这一批（再按 3.6 第 1 步核对大小）。
- 编辑会话不受影响：它经 `changesetOf` 并入，不经 `UnitSpec.Changeset`；会话活着时撤销它的变更集答 `page.locked`（交错 9）。

### 3.8 观察者与守卫

不改观察者。撤销的单元里：删除照删除（asset 删附件的行、linking 删索引行）；恢复带 `Restored`（asset 以组现在的删除时刻恢复行并核对行数，linking 照新建建索引）；放回照移动、改名（`UpdateLinks` 为空，参与者不改写别的页）；正文照正文的写（linking `ReplacePage`）。`pages` 事件照常（树变了时 `tree: true`），`links` 事件在索引变了时到达。`page/extension_test.go` 另加 `TestTheObserversFollowARevert`：一个撤销的单元里四种操作各有一项，观察者看到 `Kind` 是 `revert`、`RevertsID` 是 C、恢复的 `Restored`；一个登记的守卫同样看到 `Kind`、`RevertsID`（第 1 节目标 6：M10、M11 靠它）。

### 3.9 契约

- **`listChangesets`**：参数 `actor`（uuid）、`client`（字符串）、`page_id`（uuid，页或附件）、`limit`、`cursor`；答 `ChangesetPage {data: Changeset[], next_cursor}`；`[bad_request, notebook.not_found, page.not_found, validation_failed]`（`page.not_found`：`page_id` 不是这本笔记本的节点）。
- **`getChangeset`**：`ChangesetDetail`；`[page.changeset_not_found]`。
- **`listChangesetChanges`**：`ChangesetChangePage {data: ChangesetChange[], next_cursor}`；`[bad_request, page.changeset_not_found, validation_failed]`。
- **`revertChangeset`**：请求体 `required: false`、`ChangesetRevert {message?: string}`（`additionalProperties: false`；空的 POST 即没有说明，oapi-codegen 接受 EOF；`""` 答 422）；答 200 `ChangesetReverted {changeset_id: uuid|null, changed: boolean}`；`[validation_failed, page.changeset_not_found, forbidden, page.changeset_trimmed, page.revert_too_large, page.revert_conflict, server_busy, page.locked]`；描述写清按状态比较、整组拒绝、冲突的原因、一个单元的界线（同 `restoreNode` 的数字），以及 3.6 的码的次序与两个例外、锁下的再核对。
- **结构**（可空的对象照 P2 3.9 写成 `anyOf: [{$ref}, {type: 'null'}]`，所以都命名）：
  - `Changeset {id, notebook_id, kind: ChangesetKind, client, message: string|null, created_by: UserRef, created_at, updated_at, reverts: ChangesetBrief|null（不是撤销，或被撤销的那一个已被清理）, reverted_by: uuid[]（maxItems 100，新的在前）, trimmed, node_count, content_count（integer，minimum 0、maximum 1000，"1,000 表示这么多或更多"）, changes: ChangesetChange[]（maxItems 3）}`：日志的每条。
  - `ChangesetDetail`：`Changeset` 的每个属性加 `renames`、`moves`（boolean，前 1,000 项条目里有没有改名、移动），`getChangeset` 答它。照 `TransferJobDetail` 不用 `allOf`（`additionalProperties: false` 会拒绝多出的两项），两处同步改，写同样的注释。
  - `ChangesetBrief {id, kind, created_by: UserRef, created_at}`。
  - `ChangesetChange {node: ChangeNode, kind: created | restored | deleted | moved | renamed | reordered | content, before: ChangeState|null, after: ChangeState|null, content: ChangeContent|null}`；`ChangeNode {id, kind: NodeKind, name, alive}`；`ChangeState {parent_id: uuid|null, parent_name: string|null（根下或已被清理）, name}`；`ChangeContent {base_revision: integer|null, revision}`。
  - 新的路径参数 `ChangesetID`。
- **扩展成员**（`common.yaml` 的 `Problem`，照 `root_id` 的路：`shared.Error.Conflicts []shared.Conflict`、`ConflictCount *int`、`ProblemConflicts()`；`httpserver.Problem` 的两个字段与平台的契约测试）：`conflicts`（`RevertConflict[]`，maxItems 100，`RevertConflict {node_id, name, reason: changed | parent_gone | title_taken | too_deep | cycle | children_added | expired, holder_id?: uuid}`，"page.revert_conflict 的冲突；别的码不带"；`name` 照 3.5）与 `conflict_count`（全部的数目）。`holder_id` 只在 `title_taken` 上。
- **transfer**：`TransferFailure` 加 `changeset_reverted`。
- 网页的门禁：四个新码的 `problemMessages` 与文字（`problem-messages.test.ts` 读契约里每个操作的码）、`changeset_reverted` 的报告文字（`transfer-report.test.ts`、`transfer-import.test.tsx` 各加一行）。

### 3.10 权限、日志与交错

- `changeset.revert`：编辑者、管理员（`rules.go`，`ActionRevert` 进 `Actions()`；`bootstrap/actions_test.go` 的"规则 = 动作"）。日志与明细用 `page.history`（读者以上）。撤销别人的变更集不另要管理员（第 11 节第 2 条）。
- **权限矩阵**（与契约、处理器同一步，第 5 节：覆盖测试要每个契约操作有一行）：`listChangesets`（`readers(notebookNotFound)`，`check`：种下的变更集列出）、`getChangeset`、`listChangesetChanges`（`readers(changesetNotFound)`）、`revertChangeset`（`editorsOnly(cellOK(), changesetNotFound)`，`write: true`，每格在自己的副本上）。种子 `matrixRevert()`：每本目标笔记本一页"Renamed X"，以 SQL 写一个改名的变更集（条目"前"是"Was X"、"后"是"Renamed X"，执行者是种子的账户）；`check`：`changed`，这一页回到"Was X"。种下的变更集 id 登记进 `seeded.workspaceOfRow`，`targetViolation` 才认 `{changeset_id}`。现有的种子以 SQL 写删除的条目时补上 `revision`（3.12 的新不变式）。
- **交错**（`bootstrap/interleavings_history_test.go`，每个结束时 `checkPages`、`checkLinks`、`checkAssets`、`checkNotebooks`）：
  1. 撤销与对其中一页的正文写（`notebookRow`）：写在先 → 撤销 409 `page.revert_conflict`（`changed`），什么都没写；撤销在先 → 写 409 `page.revision_mismatch`。
  2. 撤销与对其中一个节点的改名：改名在先 → 409（`changed`）；撤销在先 → 改名照撤销之后的名称核对。
  3. 撤销时其中一页被锁（M5 的移交第 3 项）：C 是一次改名，改写了 P 与 Q 的链接；Alice 的编辑会话持着 P，Bob 撤销 C → 409 `page.locked`，带 Alice；没有撤销的变更集，名称、P 与 Q 照旧。另两例：Bob 自己在别处编辑 P → 409 `page.locked` 带 Bob；撤销 C 新建、Alice 正在编辑的页 → 409 `page.locked`。
  4. 撤销与清理截掉 C（先读条目、后读标记）：C 删除了 B 的子树（在期限之内）。撤销未加锁的一遍通过、在笔记本行上等着时，清理以偏差的截止时刻只跑条目与版本两个清理器（`purgeBefore(now + 2h, …)`，模拟时钟偏差，同 P2A 的交错 5），拿走 C 的条目并标记 C → 放开之后锁下读到的条目少了、标记在，409 `page.changeset_trimmed`，什么都没写（没有标记的检查时它会以空的计划答 `changed: false`）；撤销在先 → B 回来，之后的清理什么都不删。另一例：测试的事务以 `FOR NO KEY UPDATE` 拿着 C 的行，清理什么都不删、不标；放开之后的下一次删掉并标记。两次读之间（未加锁的那一遍里）的竞态不在整个程序上交错：`page/app` 的假存储在读标记之前跑"清理"（3.12）。
  5. 撤销正在被导入任务并入的变更集（照 `TestAnImportWhosePlaceIsDeleted`，`betweenUnits` 次里至少一次排到）：三批的导入，第三批带一个附件，第二批停在 `holdAssetBlobs` 上；撤销 C（未加锁时读到第一批）在笔记本行上排在第二批之后；放开之后撤销排在第三批之前 → 锁下读到两批、一起撤销（它们的最上层节点在回收站里），第三批在 `Run` 里看到 C 被撤销，任务 `failed`、`changeset_reverted`，报告是两批的计数，没有第三批的节点，第三批的文件已删（`checkAssets` 的"存储里没有无行的文件"钉住）；第三批在前 → 撤销整个导入，任务成功。C 的 id 由测试以 SQL 读（任务行的 `changeset_id` 在 P4）。
  6. 同一个变更集的两次撤销：后一个 409 `page.revert_conflict`（`changed`），只有一个 `revert` 变更集。
  7. 撤销一次删除与从回收站恢复同一项：恢复在先 → 撤销 409（`changed`：它活着）；撤销在先 → 恢复 409 `page.trash_changed`（它在锁下重读，根已不在回收站，P2 3.5 第 1 步）。
  8. 撤销一次删除与删除它原来的父节点：撤销在先 → 回到 P 下，再随 P 一起删除（同一个新时刻，不是根）；删除在先 → 409（`parent_gone`）。
  9. 撤销编辑会话自己的变更集（00 号文档 4.8"正在写入的变更集"的前半）：Alice 的会话写了 P（C）。会话活着时 Bob 撤销 C → 409 `page.locked`（带 Alice）；会话过期之后撤销 → P 回到原来的字节，Alice 旧标签页的保存 409 `page.edit_session_ended`，她的新会话另起变更集。净改动为零的会话（写回了原来的字节）：撤销答 `changed: false`、不写，会话的下一次保存照旧并入 C，C 没有 `reverted_by`（3.6 第 5 步）。
  10. 撤销一次删除时清理到了一半（照 P2A 的 `TestAGroupThePurgeIsMidwayThrough`）：C 删除了带附件的子树，撤销在笔记本行上等着时，以偏差的截止时刻只跑版本的清理器（`purgeBefore(now + 2h, "page_revisions")`：条目留着、C 不被标记）→ 放开之后锁下读不到组里页的版本哈希（`Missing`），409 `page.changeset_trimmed`，什么都没写；另一例只跑附件的清理器（`"asset_blobs"`）→ 恢复的附件的行数不符，单元答 `ErrTrashChanged`，换成 409 `page.changeset_trimmed`，回滚。这是整个程序上走到这两条路的测试。

### 3.11 大小与实测

- **界线**照 P2A（`domain/trash.go`，"or reverts (M8/P3)" 的注释已写）：C 的条目数对 `MaxUnitNodes`；要解析的页、字节、链接对 `MaxUnitPages`/`Bytes`/`Links`（只有一页时不限）；要恢复的组的历史对 `MaxUnitHistory`。撤销比恢复多三样：读整本活着的树（O(笔记本)）、C 的条目与节点的读、S\* 的计算，且未加锁与锁下各一遍。
- **实测撤销**：`bootstrap/revert_cost_test.go`（`//go:build !race`）的 `BenchmarkRevert`，经 `heavy.sh` 手动运行，基准自己组 page 模块并经处理器计时（同 P2A 的 `newTrashBench`，界线 `cmp.Or(参数, pageDeps 的)`）：
  - 撤销一次删除：P2A 界线处的满负载组（100 页、9,900 个附件、8 MiB、20,000 条链接、400,000 行历史）；撤销一次新建：一次 10,000 个节点的导入形状的变更集（100 页加 9,900 个附件，经 SQL 成批写入、记成一个 `import` 变更集）。各在另有 0 与 100,000 个活着的节点的笔记本里；统计照 P2A（无统计、活着时 ANALYZE、强制通用计划）。
  - 计时分开记：单元（请求减去解析）、其中读树与 `PlanRevert` 的部分（与笔记本成正比）、其余（与 C 成正比）、解析。与 C 成正比的部分在满负载下不超过 5 秒就沿用共用的界线，超过时撤销另取 `MaxRevertNodes`、`MaxRevertHistory`（5 秒之内的最大一档向下取整），写进常量的注释与 4.9；10 万个节点的笔记本里是读树超过时，降界线压不住它：写进 M12 的移交（改法是只读附近的节点），不降界线。
  - 守住：`TestARevertAtTheUnitsBoundTakesItsTime`（bootstrap，不带竞态检测的一步，单独一行带 `-v`）：P2A 界线测试的那一组删除之后撤销这次删除，10 秒之内；再多一行历史时 422 `page.revert_too_large`，不写 `revert` 变更集（钉住生产的接线）。
- **日志的成本**：`page/adapter/postgres/history_cost_test.go`（`!race`，同样加进 Makefile 不带竞态检测的一步，13.4 第 8 条）：一本笔记本 10 万个变更集（三个执行者；客户端 `web`、`api`、`mcp:a`、`mcp:b`，`cli` 只占 0.1%）、一页 2 万个版本与 2 万条条目、一个 5 万个条目的变更集；一个连接上各读 8 次：第一页、中间的游标、按人、稀有的客户端（最坏：读过整本的索引）、`mcp`、按页面、一页 100 个的 `ChangesetFacts` 与 `FirstChanges`（含那个 5 万个的）、那个 5 万个的 `ChangesetMoves`（读 1,000 行条目）、`ListChanges` 的深处；无统计、ANALYZE 之后、强制通用计划各一遍，各 200 ms 之内（实测之后照实写下本机的数）。
- **计划测试**（13.4 第 9 条，`history_plans_test.go`；page 的适配器还没有 `planOf`，照 `linking/adapter/postgres/reads_test.go` 的复制一份）：`TestTheLogReadsUseTheirIndexes`（关掉整表扫描，参数照实绑定）：日志与按人的语句走各自的索引、没有排序节点；按页面的读条目的 `node_id` 索引与版本的 `(node_id, revision)`；`ListChanges`、计数与 `ChangesetMoves` 走两个 `(changeset_id, node_id)` 唯一键；`reverted_by` 与 `LockChangeset` 走 `reverts_id` 的部分索引；清理的两条走 `deleted_at` 的部分索引与变更集的主键，`PurgeTrimmedChangesets` 走 `trimmed` 的部分索引；外键的核对（`SELECT 1 FROM changesets WHERE notebook_id = $1`，2.1 第 4 条的依据）走日志的索引。
- **数组语句的两种计划**（P2A 的教训：缓存的语句第 6 次起的通用计划）：`NodesNow`、`ContentsNow`、`VersionHashes` 在 10,000 个 id 处、`ChangesetFacts`、`FirstChanges` 在 100 处，以活着时取的统计比较通用与按参数的计划；明显更慢的经 `Planned`，并加"不在 `pg_prepared_statements` 里"的测试（同 `TestRestoreBlobsOfNodesIsPlannedWithItsNodes`）。
- **别的**：重跑 P2A 的 `BenchmarkTrashDelete`（5 万个节点）核对 `RecordItems` 的版本号子查询（与 P2A 的 1.8–2.1 秒相差不超过一成）；一次清理在 10 万个截掉了一部分的变更集上的耗时量一次。这两项与读树、日志的数写进 M12 的移交（第 7 节）。

### 3.12 测试

- **domain**：`PlanRevert` 的表格测试：每种条目（页面与附件的新建、删除、恢复、改名、移动、重新排序、写正文、还原版本、撤销）各一例通过、一轮做完；每种冲突各一例（之后又改过、标题被占带 `holder_id`、父节点不在、太深、成环、新建或恢复的节点下有别的节点、期限、页面的旧删除条目没有版本号；删除的附件不因没有版本号而冲突）；冲突的 `name`（S\* 里的、S\* 里不在的）；根的核对不通过（没有组）是 `changed`；回收站里的页编辑、撤销编辑之后版本号变了而哈希相同不冲突；读不到的哈希进 `Missing`；撤销一次导入的撤销恢复几组；冲突的次序与 100 项的上限（100 与 101）；一轮做不完答 `ErrRevertUnordered`（构造一个 M9 形状的变更集：两页互换名称）；`ChangeKind`；`ChangesetCursor` 的 UTC；`ParseClientFilter` 的每种取值、非法值与坏的 UTF-8。
- **用例**（`page/app`，假的端口与时钟）：`RevertChangeset` 的每个码与次序（含冲突在 503 之前、链接数的 422 在 503 之后、正文或版本哈希被清理的 409 在 503 之前、锁下的三种再核对）；根的核对不通过时不读组、答 409 `changed` 而不是 422；先读条目后读标记（假的存储在读标记之前把 C 标上）；锁下 `Parsed` 缺了一页或哈希不等 → `changed`、哈希相等而版本号变了照写；`ErrTrashChanged` → `page.changeset_trimmed`；假的写入者在放回时答 `ErrTitleTaken`、在正文时答 `ErrRevisionMismatch` → 500，不是 409；什么都不用改时没有变更集；`Kind`、`RevertsID`、`Message`、`Options` 为空；`Run` 对被撤销的变更集答 `ErrChangesetReverted`、在 `do` 之前；日志的 `page_id` 不是这本笔记本的 → 404 `page.not_found`、回收站里的照列；日志与明细的游标、筛选选的语句、计数、前三项、显示名；`GetChangeset` 带改名与移动、`ListChangesets` 不读它们。
- **适配器**（真的 PostgreSQL）：三条日志语句的次序、游标、筛选；`ChangesetFacts` 在 1,000 与 1,001 处、`reverted_by` 100 与 101；`ChangesetMoves` 的改名、移动、重新排序（不算移动）与第 1,001 项的改名（不算）；`ListChanges` 的合并、父节点的名称（活着、已删、已清理）；`RecordItems` 给删除的页记版本号、附件为空、合并时保留第一次的、改名之后删除的条目带删除时的版本号；`LockChangeset` 的 `reverted`；`NodesNow` 的根的统计；清理的标记与跳过（3.3）；成本与计划测试（3.11）。
- **transfer**（`transfer/app`）：假的 `Tree.Import` 在第二批答 `ErrChangesetReverted` → 任务 `failed`、`changeset_reverted`，报告是第一批的计数，这一批的文件已删。
- **handler 与平台**：契约的结构、参数、各码；`conflicts`、`conflict_count` 只在 `page.revert_conflict` 上（平台的契约测试、`apierrors_test.go`、`shared/error_test.go`）。
- **整个程序**（`bootstrap/history_test.go`；13.1 第 21 条）：经接口做每种变更再撤销：编辑会话的正文、改名（改写过的页逐字节回到原样、`checkLinks`、`links` 事件）、移动、重新排序、新建页与附件（附件的行随之删除）、删除带附件的子树（附件的行回来、新签的地址下载 200、无主列表的大小）、从回收站恢复、还原版本、一次小的导入、撤销的撤销；每种冲突；截掉的 409；422 的各项；`pages` 事件；日志按人、客户端、页面筛选；明细的分页。
- **性质测试**（`revert_property_test.go`，固定的几个种子）：随机的操作序列（新建页与附件、改名、移动、重新排序、删除、从回收站恢复、写正文、还原版本、撤销一个随机的较早变更集，409 的跳过）之后，从新到旧连续撤销 k 个，每一步都成功，最后活着的树（父节点、名称、兄弟间的相对次序）、正文的字节、链接索引与活着的附件的行等于这 k 个之前的；这 k 个之前就在回收站里的仍在（时刻与名称可以不同，2.1 第 2 条），这 k 个里新建的在回收站里。生成器的重新排序在兄弟间随机取位置，不会连续约 30 次插进同一个间隔（重新编号会显示为冲突，3.5）。
- **不变式**：`checkPages` 加"撤销的 `reverts_id` 指向同一本笔记本的变更集，或已被清理的"与"页面的删除条目带 `revision`"。

## 4. B：前端

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/services/page.service.ts` | `changesets`、`changeset`、`changes`、`revertChangeset` 与类型（`Changeset`、`ChangesetDetail`、`ChangesetBrief`、`ChangesetPage`、`ChangesetChange`、`ChangesetReverted`、`RevertConflict`），各随第一次用到它的那一步加入 |
| `web/apps/web/src/stores/page-tree.store.ts` | `revert(id, message)`：同一变更集一个一个来（`oneAtATimeById`）、经树的写入队列；撤销删掉的页记进 `removed` |
| `web/apps/web/src/pages/notebook/history-page.tsx`、`history-row.tsx`、`history-filters.tsx` | 历史页 |
| `web/apps/web/src/pages/notebook/changeset-page.tsx`、`changeset-change.tsx` | 单个变更集页与明细的一行 |
| `web/apps/web/src/pages/notebook/revert-dialog.tsx` | 撤销的确认框与冲突 |
| `web/apps/web/src/pages/notebook/changeset-text.ts` | 类型、客户端、变化的文字（日志、明细、版本页共用） |
| `web/apps/web/src/pages/notebook/notebook-places.tsx` | 加"历史"；`nav` 的名称键改为中性的 `notebook.places` |
| `web/apps/web/src/pages/page/page-menu.tsx`、`version-page.tsx` | "查看这一页的历史"；版本页的变更集链到它的页 |
| `web/apps/web/src/app/routes.tsx` | `notebooks/:id/history`、`history/:changesetId` |
| `web/apps/web/src/events/handlers.ts`、`app/event-stream.tsx` | 新键的重读、连上时的重读 |
| `web/apps/web/src/i18n/messages/{zh-CN,en}.ts` | 文字（前缀 `history.`） |
| `web/apps/web/src/test/page-server.ts` | 假接口的日志、明细与撤销（码的次序同服务端） |
| `e2e/fixtures/history.ts`、`wiki-pages.ts`、`assert/page.ts` | 夹具（4.8） |

### 4.2 历史页

- 路由 `/:slug/notebooks/:id/history`，在笔记本的外壳里、页面的外壳之外。`h1`"历史"，文档标题"历史 · 笔记本名"。
- 列表：SWR 键 `["changesets", nb, actor, client, page]`（空的筛选写 `""`），`readPages`（每页 50 条），"更多"用 `useLoadMore`；列表有名称"改动"。
- 每条：类型与执行者（"编辑 · Ada"），客户端（"网页""接口""命令行"，`mcp:<名>` 写"MCP（名）"），相对时刻（`<time>` 带完整时刻），说明（`<bdi>`），前三项变化的摘要（"新建「A」、改名「B」为「C」…"）与"共 N 个节点、M 篇正文"（1,000 写"1,000 多"，英文不用复数形式）；标记："已撤销"（链到最新的那次撤销的页）、"已截掉"、撤销的"撤销了「编辑 · Ada · 时刻」"（链到被撤销的；它已被清理时写"撤销了一个已被清理的改动"）。整条的主链接到单个变更集页，名称"类型 · 执行者 · 完整时刻"；完整时刻只到分钟，以 `twinsOf` 按（类型、执行者、分钟）找出同名的行，给它们加 `shortId`（"ID 0000e1"，同 P2B 回收站的行，13.2 第 17 条）；"撤销了「…」"的链接同样。行里的控件用 `useFocusLeaving`：重读之后行没了，焦点不落到 `body`。
- **筛选在地址里**（`?actor=&client=&page=`，替换而不压历史）：人的选择从工作区成员（能读到时）与已读到的日志里的执行者里来；客户端：全部、网页、接口、命令行、全部 MCP 与已读到的 `mcp:<名>`；页面只经页面菜单、明细与冲突里的"这个节点的历史"进来，显示为可以去掉的"页面：X"（树里没有它时写"已删除的页"）。每条的执行者旁有"只看此人"（可及名称"只看 Ada 的改动"）。筛选变了焦点留在控件上，结果礼貌地播报（"列出 50 条，还有更多"、"列出 12 条"、"没有改动"）。
- 手改的地址：`actor`、`page` 不是 UUID 时不读（同 P2B 预览的 `nodeIdPattern`），422（`client` 非法）与 404 `page.not_found`（这一页不在这本笔记本里，或已被清理）同样说明并给"清除筛选"。读者与写者看到的一样。

### 4.3 单个变更集页

- 路由 `/:slug/notebooks/:id/history/:changesetId`；`:changesetId` 不是 UUID 时 `NotFoundPage`、不读。SWR 键 `["changeset", nb, id]` 与 `["changeset-changes", nb, id]`（`readPages`，每页 100 项）。答复的 `notebook_id` 与地址不同时 `NotFoundPage`；404 同。文档标题"编辑 · Ada · 历史 · 笔记本名"；面包屑经 `BreadcrumbTrail`：历史 › 这一条（左栏的"历史"是 `end` 的，这里不标为当前）。
- 信息：类型、执行者、客户端、创建与最后写入的完整时刻、说明、撤销了哪一个与被哪些撤销过（链接）、截掉的说明（"这次改动碰过的一些页已被清理，不能整组撤销"）、两个计数。
- 明细（列表名称"这次改动的节点"）：每项是节点（活着的页链到页面；附件链到它所在的位置，同 P2B 的"打开"；回收站里的写"在回收站里"并链到 `/trash/:id`，不是根时预览会换成它的根的，P2B 的 A-M1；之后写"这个节点的历史"链到 `?page=`）与变化（"新建""从回收站恢复""删除""从「P」移到「Q」""改名：「A」→「B」""调整次序"，可兼"写了正文"；页活着时链到版本页 `/pages/:pageId/versions/:revision`，它的"与上一版比较"就是这次的改动，版本页只认树里的页；父节点已被清理的写"（已清理）"）。
- 写者的"撤销这次改动"（英文 "Undo change"）：截掉的不显示按钮、显示上面的说明；已被撤销过的照样可以撤销，按钮旁写"已被撤销过：再撤销只在那次撤销被撤销之后才会成功"。读者看到"编辑者与管理员可以撤销这次改动"（M8 设计审查 B-N8）。

### 4.4 撤销的确认框与冲突

- 确认框：标题"撤销这次改动？"；正文"N 个节点与 M 篇正文回到这次改动之前，记为一次新的改动（撤销）。这次改动之后又改过的、放不回去的会列出，什么都不会改。"；单个变更集页读到的 `renames` 或 `moves` 为真时另一句"改名、移动之后新写的、指向新位置的链接不会跟着改回"（第 11 节第 4 条）；可选的"说明"。经 `useForm`（13.2 第 12 条）：说明的字节数在送出前核对，服务端对 `message` 的 422 落在字段下面；说明为空时不带它。发送中两个按钮禁用、Esc 不关闭；`pages.revert(id, message)`。
- 答复：
  - 200 `changed`：关闭，单个变更集页的 `<output>`"已撤销 [查看这次撤销]"（链到 R），焦点经 `watchReader` 回到 `h1`（13.2 第 26 条，同 P2B 的恢复；`aria-describedby` 指向状态行）；重读这一页、明细与日志的键；树由写入队列重读。
  - 200 未改动：关闭，状态行"没有要撤销的改动"。
  - 409 `page.revert_conflict`：确认框留着，内容换成冲突：小标题"不能撤销：K 个节点在这之后改过或放不回去"取得焦点（多于 100 时"列出前 100 个"），列表每项"「名称」：原因"与可以去做什么：`changed` →"这个节点的历史"（`?page=`）；`title_taken` → 链到占着名称的节点，"先给它改名"，这个节点不在树里（在回收站里，撤销的是一次删除）时另说"或从回收站恢复（恢复会加序号）"；`parent_gone` →"它原来的上级不在了（在回收站里或已被清理）"，链到回收站；`children_added` → 链到这个节点，"它是之后放进来的，撤销会把它一起删掉：先把它移走"；`too_deep`、`cycle` → 说明；`expired` →"已过回收站的期限"。`renames` 为真时另一句"也可以在页面树里把它改回原名（普通的改名，会改写链接）"（00 号文档 4.8）。"关闭"把焦点还给"撤销"。
  - 409 `page.changeset_trimmed`：说明，重读这一页（按钮随之消失）。
  - 409 `page.locked`：照现有的锁的说明（持锁人，`lockedText`）。
  - 422 `page.revert_too_large`：照 P2B 的 `trash.tooLarge` 不说是哪一项："这次改动太大，不能在这里撤销：它的节点、页面、正文、链接或这些页面的历史超出了一次撤销能做的量。它碰过 N 个节点。"（N 是 `node_count`，1,000 写"1,000 多"；P4 改为问是否在后台进行）。
  - 503：照现有的忙的说明，可以再试。
  - 403、404：`errorText`，关闭，重读这一页与 `["notebooks", ws]`（同 P2B 回收站的拒绝：被降为读者的写者随之没有按钮）。
  - 500 `internal_error`（请求的期限是 500，提交可能已落下）、网络错误、超时：说"结果未知"，重读这一页：`reverted_by[0]` 与请求之前知道的最新那个不同时读 `getChangeset(reverted_by[0])`，执行者是自己就说"已撤销 [查看这次撤销]"，不是就说"这次改动已被 X 撤销"；相同就说"没有撤销，可以再试"。比较第一个而不比个数：个数到 100 不再增长，也数得到别人的撤销。

### 4.5 入口、页面菜单与版本页

- 左栏 `NotebookPlaces` 的单独命名的 `nav` 里，"历史"在"回收站"之前（`NavItem end`：只在历史页上是当前，单个变更集页不是）；`nav` 的名称键从 `trash.places` 改为 `notebook.places`（它现在有两处）。
- 页面菜单（读者也有）加"查看这一页的历史"：去 `history?page=<id>`。
- 版本页：版本的"执行者 · 类型"链到它的变更集页（P1 的移交）。右栏的版本列表不加：每行已有版本的链接，再加的链接每行同名（13.2 第 17 条）。

### 4.6 页面树、事件与列表的模式

- `PageTreeStore.revert`：`this.restores(id, () => this.write(…))` 同一个变更集一个一个来、经树的写入队列（13.2 第 1 条）。撤销删掉的页照 `remove` 记进 `removed`：撤销之前的树里有、写入队列的重读之后没有的页，`to` 是之后仍在的最近的祖先，`after` 是答复时的 `readsStarted`；这一比较也会收进这期间别人删掉的页，无害（它们确实不在了）。撤销恢复回来的页由 P2B 的规则去掉（一次开始于答复之后的读列出它）；答复不列出带回了哪些页，所以 `removed` 不空时再读一次树（同 P2B 的 `restore`：写入队列的那次读失败时，这个标签页删掉、撤销带回的页会一直显示为已删）。
- 事件：`pages` 事件（任何）经 `refresher` 以默认的间隔（5 秒）重读这本笔记本的 `changesets`、`changeset`、`changeset-changes` 键（00 号文档 4.13）；`refreshedOnConnect` 的最内一层加这三个。撤销答复之后本标签页立即重读，不等事件。
- 列表照 P2B：会变、会消失的项用 `readPages`"按已读的页数重读"（13.2 第 19 条）；新的变更集到来时已读的最后一页的尾部会掉出去，同这个模式的约定。

### 4.7 测试（vitest）

- 服务与 store：`revert` 只发一次（连点两下）、经写入队列、`removed` 记下撤销删掉的页（以及之后的读列出它时去掉）、写入队列的读失败而 `removed` 不空时再读一次。
- 历史页：行的文字与可及名称（两种语言；同一分钟同类型同执行者的两条带 `shortId`，"撤销了「…」"的链接同样）、计数在 1,000 处、标记与链接（被撤销的已清理时的文字）、"更多"与重读的竞争（照回收站）、筛选进地址、"只看此人"、客户端的选择、页面的筛选与去掉、播报的文字、422、`page.not_found`、不是 UUID 的 `actor`、`page` 不读、事件与连上时的重读、经组合根到达。
- 单个变更集页：信息、明细的每种变化的文字与链接（活着、回收站里链到 `/trash/:id`、已清理的父节点；正文的链接只在活着的页上）、分页、面包屑、读者的说明、截掉时没有按钮、已被撤销过的说明、不是 UUID 的地址不读。
- 确认框：每种答复（200 两种；409 三种，冲突另有超过 100 项的一例；422；503；403、404 的重读；500、网络错误各三种结果）与焦点、冲突的每种原因的文字与链接（`title_taken` 的回收站提示只在节点不在树里时）、100 项与总数、`renames`/`moves` 的提示、说明的字节上限与 422 落在字段下、只发一次。
- 版本页的变更集链接（右栏没有）、页面菜单的项、左栏的入口在单独的 `nav` 里（中性的名称、`end`）。

### 4.8 e2e

- **HI3**（`e2e/stories/history/hi3-log-revert.spec.ts`）：
  - 接口版本：两个账户经 PAT 写（`api`：新建、改名并改写别的页的链接、移动），其中一个另经 `login` 的登录会话写一次正文（`web`）；`listChangesets` 按人、按页面（另一本笔记本的页 → 404）、按客户端筛选，游标；`getChangeset`、`listChangesetChanges`；撤销那次正文 → 字节回到原样（`expectVersions` 的 `reverts_id`）；撤销那次改名 → 被改写的页逐字节回到原样，`links.ts` 的索引断言、`links` 事件到达；之后改过一页再撤销另一条 → 409，`conflicts` 里是它与 `changed`，数据库里没有新的 `revert` 变更集。
  - 页面版本：左栏进历史页；"只看此人"与页面菜单的"查看这一页的历史"筛选；打开那次改名的一条，看明细；撤销（状态行、焦点）；调用接口版本的同样断言（`expectVersions` 的 `reverts_id`、`links.ts` 的索引、被改写的页的字节）；冲突时确认框列出节点与原因，数据库里没有新的 `revert` 变更集（13.4 第 3 条）。
- **HI4**（`hi4-revert-delete.spec.ts`）：
  - 接口版本（PAT）：删除带子页与附件的子树，撤销这次删除，再撤销这次撤销。
  - 页面版本：同一个标签页里在树的菜单删除一页（带子页与附件），进历史撤销这次删除 → 树里看得到它（`treeTitles` 轮询）、打开得了、附件下载 200；撤销这次撤销 → 它回到回收站（回收站页列出它）。
  - 两个版本调用同样的断言：`expectSubtreeRestored(…, {kind: "revert"})`、`expectAssetsRestored`、删除的条目带 `revision`、两个 `revert` 变更集的 `reverts_id` 成链、撤销的撤销之后 `expectSubtreeDeleted`。
- 夹具：`e2e/fixtures/history.ts`（`listChangesets`、`getChangeset`、`listChanges`、`revertChangeset`）；`wiki-pages.ts` 的 `historyPath`、`changesetPath`、`revertWith`；`assert/page.ts` 的 `expectSubtreeRestored` 加参数 `kind`（`trash_restore` | `revert`，P2B 的写死了 `trash_restore`）与 `expectRevert(db, revertId, {revertsId, items, versions})`。

## 5. 实施步骤

A（分支 `m8-p3a`）：

1. 迁移两条与 schema、补值的测试；domain 的类型、错误、动作与规则；`shared.Error` 的两个扩展成员、`httpserver.Problem`、`common.yaml` 的 `Problem.conflicts`/`conflict_count` 与平台的契约测试（`TestPlatformProblemsMatchTheContract` 以封闭的 `Problem` 核对平台的问题，三者同一步才绿）。
2. `RecordItems` 的删除版本号与合并、`LockChangeset` 的 `reverted` 与 `Run` 的提前加锁、`ErrChangesetReverted` 与 transfer 的失败（`Import` 的映射、删文件、`transfer/app` 的测试）；清理的两条改写与 `PurgeTrimmedChangesets`；适配器测试。现有的测试照旧通过（种子补 `revision`）。
3. `History` 端口、查询与适配器；三个读的用例；计划与成本测试。
4. `PlanRevert` 与它的单元测试；`bringBack` 的抽出（P2 的测试照旧）、`Unit.Revert`、`RevertChangeset` 与测试。
5. 契约、处理器、接线（`page/module.go`）、`make gen`；四个操作的矩阵行、`matrixRevert` 种子与 `seeded.workspaceOfRow` 的登记（覆盖测试要每个契约操作有一行，同 P2A）；网页的门禁。
6. 整个程序、性质测试、交错、观察者与守卫的扩展测试；`BenchmarkRevert` 实测并写回，守住的测试与 Makefile；M12 的移交写上实测的数。
7. 反向对照、门禁、审查。

B（分支 `m8-p3b`，P2B 与 A 合并之后；服务函数与类型随第一次用到它们的那一步加入，每一步 knip 都绿）：

1. 历史页：`changesets` 与类型、路由、左栏（中性的名称键）、页面菜单、行（`twinsOf`）与筛选。
2. 单个变更集页：`changeset`、`changes`、明细、面包屑、版本页的链接。
3. 撤销：`revertChangeset`、store 的 `revert` 与 `removed`、确认框与冲突（`useForm`）；文字。
4. 事件与连上时的重读；vitest 补齐；e2e HI3、HI4；反向对照、门禁、审查。

## 6. 测试与验证

- 门禁：`make lint-go lint-web knip gen-check test-go test-web build`、e2e 全量。
- 反向对照：A 的每个用例与查询（L 的每一行、S\* 的每种冲突与 `name`、冲突的次序与上限、单轮的次序与 `ErrRevertUnordered`、根的核对、码的次序与两个例外、先读条目后读标记、`Missing`、锁下的再核对、`Parsed` 的哈希配对、`ErrTrashChanged` 的换码、已分类错误的包装、`Run` 的提前加锁、transfer 的映射与删文件、删除条目的版本号与合并、清理的标记与跳过、计数的上限、`ChangesetMoves`、`page_id` 的 404、筛选选的语句）、B 的确认框的每种答复（含"结果未知"的三种）、同名行的 `shortId`、`removed` 与再读一次、筛选与列表的重读，各改坏一处，测试要失败。
- 审查：每部分三位 Opus 审查者并行（A：撤销的规则与 S\*、数据与并发（清理、并入、锁）、契约与权限与测试；B：界面与无障碍、状态与事件、测试与 e2e），之后 Opus 核对修复，直到一轮没有行为上的发现。

## 7. 完成标准

- 第 2 节的每一项做完，第 3.12、4.7、4.8 节的测试通过，门禁全绿；撤销在界线处实测并写回（3.11）。
- 审查记录：设计 `reviews/P3-design-review.md`，实施 `reviews/P3A-log-revert-server-review.md`、`reviews/P3B-log-revert-web-review.md`；00 号文档的进度表与本文档的"结果"一节更新。
- **P3 的文档提交做的改动**（随本文档一起）：
  - `00-M8-design.md`：2.1 的 14 条。
  - `v0.1-design.md`（00-M8 第 7 节"随这份设计一起改的"里与 P3 有关的，至今没有改）：3.8 的"还原与撤销"写明按状态比较（树、正文的哈希、回收站里的组）、对目标状态一次核对、次序值也比、截掉的不能撤销；4.5 加"撤销改名不经改写：C 改写过的页逐字节写回，之后新写的指向新名称的链接不跟着改回"；6.1 的新码与扩展成员 `conflicts`、`conflict_count`，内嵌显示名的例外加"日志带执行者与被撤销者的显示名"；7.2 的 `changesets` 的索引与 `changeset_items.revision` 在删除的条目上；9.2 左栏加笔记本的"历史"与"回收站"（P2 没有改它）。
  - 新的移交 `M9-mcp/handoffs/M8-P3-revert.md`（`status: open`，`from: M8/P3`，`to: M9`）：通用的多轮应用与临时名称（推后、名称的环；`MoveNode` 带着现在的名称会撞上唯一索引，先改名或经临时名称；临时名称截短到 255 字节之内、附件保留扩展名；模拟按两步的中间状态判；在数据库上测）；恢复到另一组里的父节点之下；00-M8 第 9 节点名的三种（互换名称、移入新建的文件夹、删除再新建同名）连同数据库上的测试；一页既要恢复又要写回正文时 `Parsed` 按（页、哈希）分开、恢复的那一步带它自己的 `Facts`；一个单元里先改名再删除同一页的条目带删除的版本号（P3 的 `RecordItems` 已这样合并，M9 加整个程序上的测试）。
- v0.1 设计第 13 节的条目随实现改（A、B 各自的提交）：13.1 第 1 条（撤销的单元、单轮应用、并入的单元先锁变更集）、第 2 条（`reverts_id`、`trimmed`、被撤销的不再被并入）、第 4 条（撤销的码的次序与例外）、第 5 条（清理同时以 `SKIP LOCKED` 锁条目与变更集；page 的一支：并入的单元在笔记本之后锁变更集的行）、第 6 条（截掉的标记、空变更集的清理）、第 8 条（`conflicts`、`conflict_count`）、第 10 条（冲突里的名称不进日志）、第 19 条（撤销在单元之前解析，从大到小）、第 21 条（最后一跳：撤销里的恢复经 asset 观察者；transfer 的失败经 `TreeWrites` 的 `Import`）、第 27 条（日志与明细的游标）、第 31 条（冲突 100 项、计数 1,000、`reverted_by` 100），13.2 第 1、17、19、23、25（冲突列表的上限）、26 条（B），13.4 第 4、6、8（两个新的不带竞态检测的成本测试与 Makefile）、9 条。
- `M12-release/handoffs/M4-performance.md` 加第 15 项（P3A 实测之后、随 A 的提交）：撤销读整本活着的树（未加锁与锁下各一遍）在 10 万个节点的笔记本里的数与改法；截掉了一部分的变更集每次清理都扫到；按页面的日志与这一页的历史成正比；`RecordItems` 的版本号子查询在 5 万个节点的删除上的数。
- M7 的文档不改：它们记的是当时的设计；新的失败 `changeset_reverted` 记在 00 号文档 4.8 与 transfer 的契约里。别的已有的移交文件不改（落实记在 00 号文档的"移交的落实"表里：M5 第 1、3 项、M6 第 4、5 项在 P3）。

## 8. 交给后面的 Phase

- **P4**：
  - 撤销的任务开始之前在请求里未加锁地跑 3.6 的第 2–7 步（同一个 `PlanRevert`、同样的根的核对），冲突照 409 答、不开任务；422 改为问是否在后台进行。
  - 分批按 `RevertPlan.Ops` 的次序切，每批在锁下对它的节点重新 `PlanRevert`（只看这一批的节点与它们的父节点、名称），第一次冲突停下并报告。
  - 分批删除用任务定下的同一个时刻时：`RecordItems` 给删除的条目记版本号的子查询认的是"单元的时刻删除的正文"，批的单元时刻与删除时刻不同时要改为删除的时刻；上层的根之下早先标上的根取消、统计重算（P2 的移交）；`DeleteNodes` 的 `stray` 只管一个单元。
  - 两批写同一页的正文：后一批在锁下核对这一页的版本号是前一批写下的，作为冲突停下；`RecordRevision` 的拒绝只是最后的防线（00 号文档 4.10）。
  - 恢复与撤销的任务并入自己的变更集，被撤销时同样以 `changeset_reverted` 停下、删掉这一批的文件（3.7）；`transfer_jobs.changeset_id` 与"在历史里查看"链到单个变更集页。
  - 撤销任务的报告列出冲突的节点时用同样的原因、`name` 与 `holder_id`。
- **P5**：无。
- **M9**：`log` 调用 `ListChangesets`（`actor`、`client`、`page_id`、游标都已备）；`batch` 的撤销要的多轮应用、临时名称与别的见 `M9-mcp/handoffs/M8-P3-revert.md`（第 7 节）；一个单元里删除再恢复同一节点仍是 M9 的事（`RecordItem` 的 upsert，00 号文档"写给后面的 M"）；单个变更集的 `renames`、`moves` 只看前 1,000 项，`batch` 的大变更集里之后的改名不算（3.4）。
- **M11**：从新到旧连续撤销对恢复与删除也成立（3.5），回滚可以照它；冲突时停下并报告。
- **M12**：撤销里的删除与 `DeleteNode` 一样没有历史的界线；截掉了一部分的变更集每次清理都扫到（3.3）；撤销读整本活着的树（3.5，实测写在 3.11 与 M12 的移交第 15 项）；次序值被重新编号时显示为冲突（条目记前一个兄弟的精确做法，已在 00 号文档第 7 节）。

## 9. 结果

（实施之后填写。）

## 附：查过的代码

行号是 `main` 的 `613adfc`（网页的 P2B 部分是 `m8-p2b` 的 `4daf892`）。

| 决定 | 依据 |
|---|---|
| 日志的索引不带 `WHERE`、去掉 `changesets_notebook_id_idx`（2.1 第 4 条） | `migrations/sql/00014_page_changesets.sql:7`（`notebook_id … REFERENCES notebooks ON DELETE RESTRICT`）、`:22-23`；`modules/notebook/adapter/postgres/queries/purge.sql:17-24`（`PurgeNotebooks` 删行，触发外键核对）；`modules/page/adapter/postgres/queries/nodes.sql:184-205`（`DeleteNotebooksPages` 带 `deleted_at IS NULL`）；`queries/activity.sql:5-10`（`NotebookActivities` 同）；`queries/versions.sql:1-23`（按主键连接） |
| 一个变更集的条目、版本按节点 id 读、计数（3.4） | `00015_page_changeset_items.sql:23`（唯一键 `(changeset_id, node_id)`）、`:31-32`；`00016_page_page_revisions.sql:22`；`00033_page_revisions_node_revision.sql:7` |
| 删除不比时刻、删除的条目记版本号与合并（2.1 第 1 条、3.2、3.4） | `queries/nodes.sql:102-132`（`DeleteNodes` 以单元的时刻删除正文、版本与所有条目）、`:134-182`（`RestoreNodes` 恢复这一时刻的全部，含删除它们的变更集的条目）；`queries/changesets.sql:11-34`（`RecordItems` 的 `revision` 只在"前"空"后"有时、删除的条目 `deleted_at = now`、`:32-34` 的 `DO UPDATE` 不碰 `revision`）；`adapter/postgres/changesets.go:54-87`；`00036_page_changeset_items_restored.sql`（约束）；`app/unit.go:221-248`（`apply` 先写后记条目）；`domain/change.go:30-45`（`Revision` 是写了正文）、`:67-73`（`Then`） |
| 根的核对先于读组（2.1 第 3 条、3.6 第 6 步） | `00035_page_nodes_trash.sql`（`trash_root`、`trash_pages`、`trash_assets` 只在根上）；`queries/trash.sql:50-56`（`TrashGroupRoot` 按节点与时刻读、不核对是根）；`app/unit_restore.go:57`（P2 的恢复先查根） |
| 回收站里的正文的哈希经版本读（3.5） | `domain/trash.go:100-105`（`TrashMember` 只有 `Revision`、`Bytes`）；`queries/trash.sql:50-68`；`ContentsNow` 只读活着的 |
| 恢复的部件复用、放置与命名不复用（2.1 第 2 条、3.6） | `app/unit_restore.go:49-126`（重读、`PlacementOf`、`restoredNames` 的序号、变更、`RestoreNodes` 的五个数）、`:157-196`（组内同键加序号）；`app/restore_node.go:65-129`（`TrashGroup`、`UnitTooLarge`、`TrashHistory`、`TrashContents`、`DecidedAll`）；`domain/trash.go:39-73`（界线）、`:153-165`；`adapter/postgres/trash.go:90-91,145-146`（按参数规划） |
| `DecidedAll` 的 `tooLarge` 与等待合计（3.6 第 8 步） | `app/content.go:78-131` |
| 正文的写回、无改动不写、基的核对与版本链（3.6） | `app/unit_content.go:42-96`（`:63-66`、`:65-66` 的 `ErrRevisionMismatch`、`:68`、`:117-118` 的 `edit_session_ended`）；`queries/changesets.sql:36-47`（`RecordRevision` 的 `WHERE` 守卫，`:execrows`） |
| 删除从上往下、子树一次（3.5） | `app/unit_delete.go:20-45`；`queries/nodes.sql:110-112`（`stray`）；`ports.go:172-176` |
| 放回不经 `Move`/`Rename` 的核对、移动要告诉子树、已分类的错误要包（3.6） | `app/unit_move.go:21-77`（`:61-69` 的 `placeAmong` 与子树的变更）；`app/unit_rename.go:21-58`；`app/unit_place.go:117-144`；`queries/nodes.sql:96-100`（`MoveNode` 带着现在的名称）；`adapter/postgres/store.go:262-281`（`ErrTitleTaken`）、`:296-312`（P2A 对 `RestoreNodes` 的包装） |
| 单轮足够、多轮交给 M9（2.1 第 11 条、3.5） | `domain/change.go:59-61`（只有 `Moves()` 成为条目）；`app/unit.go:317-319`；`app/unit_import.go`（导入只新建）；`migrations/sql/00012_page_nodes.sql`（名称 255 字节） |
| 守卫：正文与删除被锁（3.6、交错 3、9） | `app/edit_lock.go:44-73`（`OpContent` 任何别的会话、`OpDelete` 别人的会话） |
| 不经改写的参与者（3.6） | `modules/linking/app/rewrite.go:49`；`bootstrap/linking.go:64` |
| 并入的单元先锁并核对（2.1 第 7 条、3.6、3.7） | `app/unit.go:58-83`（`UnitSpec.Changeset`）、`:139-195`（`Run`）、`:264-295`（`ensureChangeset` 在第一次写时锁）、`:337-350`（会话经 `changesetOf`）；`queries/changesets.sql:49-55`；`app/import_writes.go:74-83`；`app/unit_import.go:76-80`（建页先读父节点：父节点被撤销删掉时先报 `ErrNoParent`） |
| transfer 的失败、映射与文件（3.7） | `bootstrap/transfer.go:98-104`（`Import` 原样返回）、`:127,136,145-153`（`treeError` 只在单元里）；`transfer/app/import_write.go:214-256`（`unit` 的 `uncertain`、`dropFiles`、`unitFailure`）；`transfer/app/import_ports.go:99-103`；`transfer/domain/report.go:29-45`；`api/modules/transfer.yaml:600-625`；`bootstrap/assets_test.go:105-128`（`checkAssets` 的无行的文件） |
| 清理的写法与批的循环（3.3） | `queries/purge.sql:1-43`；`adapter/postgres/purge.go:16-70`（`PurgeNodes` 的接续）；`page/purgers.go:15-24`；`platform/jobs/purge.go:94-104`（不满一批即停） |
| 计数的写法（2.1 第 5 条、3.9） | `api/modules/linking.yaml:282-286`（"counted up to 1000: 1000 is as many or more"、`minimum`/`maximum`） |
| `page_id` 不是这本笔记本的答 404（2.1 第 5 条、3.4） | 00-M8 4.12（"与别的接口一致，不答空列表"）；`api/modules/asset.yaml:89`（`listAssets` 的 `[bad_request, notebook.not_found, page.not_found, validation_failed]`） |
| 数组语句的通用计划、计划测试（3.11） | `platform/postgres/planned.go:10-24`；`adapter/postgres/store.go:38-45`；`linking/adapter/postgres/reads_test.go:592-593`（`planOf`，page 的适配器没有）；v0.1 设计 13.4 第 9 条 |
| 扩展成员的路（3.9） | `api/common.yaml:34-123`（`lock`、`locks`、`ended_by`、`root_id`）；`shared/error.go:118-127,230-251`；`domain/errors.go:76-90`；`platform/httpserver/contract_test.go:19`（`TestPlatformProblemsMatchTheContract`） |
| 权限与矩阵（3.10、第 5 节） | `access/domain/rules.go:104-108`；`domain/actions.go:7-40`；`bootstrap/permission_matrix_page_test.go:263-283`（`restoreNode`，`write: true`）；`permission_matrix_test.go:152-163`；`permission_matrix_coverage_test.go:61-66`（每个操作要有一行）、`:139-144`（`workspaceOfRow`） |
| 交错的写法（3.10） | `bootstrap/interleavings_trash_test.go:28-80`（`holdNotebook`、`waitingFor`）；`interleavings_assets_test.go:192,233`（`purgeBefore`、`holdRow`）；`interleavings_import_test.go:289,295-334`（`betweenUnits`、两批之间删除）；`interleavings_transfer_test.go:51`（`holdAssetBlobs`）；`trash_test.go:417`（`trashedAgo`）；`interleavings_page_test.go:43-110`（`checkPages`） |
| 写入单元的读与 `ListNodes`（3.5） | `app/ports.go:122-129`（`AnyNode`）、`:133-159`；`queries/nodes.sql:30-35` |
| 答复的形状、可空的请求体（2.1 第 8 条、3.9） | `app/restore_version.go:16,72-73`；`api/modules/page.yaml:1110-1126`（`VersionRestored`）；`page/adapter/http/gen/server.gen.go:3057-3061`（`required: true` 时空的 POST 是 400） |
| 契约的类型与回收站的操作（3.9） | `api/modules/page.yaml:667-787`、`:1076-1100`（`VersionChangeset`、`ChangesetKind`）；`api/modules/transfer.yaml:469-471`（`TransferJobDetail` 不用 `allOf`） |
| 请求的期限答 500（4.4） | `platform/httpserver/apierrors.go:185-187,228-230` |
| 左栏的入口、同名行、预览的地址与列表的模式（4.2、4.3、4.5、4.6） | `web/apps/web/src/pages/notebook/notebook-places.tsx:14-24`（`NavItem end`、`trash.places`）、`lib/twins.ts`（`twinsOf`、`shortId`）、`pages/notebook/trash-preview.tsx:37-38,57-58`（`nodeIdPattern`）、`pages/page/breadcrumbs.tsx:36-40`（`BreadcrumbTrail`）、`pages/notebook/trash-page.tsx:17,99`（`watchReader`）、`lib/focus-leaving.ts:10`、`app/form`（`useForm`）、`lib/read-pages.ts:13-23`、`stores/page-tree.store.ts:450-463`（`restore` 再读一次树）、`i18n/messages/zh-CN.ts:235-236`（`trash.tooLarge`）（`4daf892`） |
| 事件与连上时的重读（4.6） | `web/apps/web/src/events/handlers.ts:56-100`、`app/event-stream.tsx:15-31`（`4daf892`）；`events/refresher.ts:10-11`（`INTERVAL_MS` = 5,000） |
| 页面菜单、路由与版本页（4.5） | `web/apps/web/src/pages/page/page-menu.tsx:25-40`；`app/routes.tsx:140-170`；`pages/page/page-versions.tsx:111-121,143-149`（右栏每行已有版本的链接、`versionBy`）；01-P1 第 8 节（只要版本页的链接） |
| e2e 的断言（4.8） | `e2e/fixtures/assert/page.ts:213-259`（`expectSubtreeRestored` 写死 `trash_restore`，`:245`）、`:149-173`（`expectSubtreeDeleted` 不限类型）；`e2e/fixtures/auth.ts:35`（`login` 的登录会话）（`4daf892`）；v0.1 设计 13.4 第 3 条 |
