# M8/P4 后台的恢复与撤销：架构设计 & 实施规划

## 0. 分两部分

照 P1–P3 的做法分成 A 与 B，各自一个分支、一次审查与修复核对、一次合并。B 以 A 合并之后的契约为准。

- **A（服务端）**：transfer 的两种新任务 `trash_restore`、`revert`；迁移；导入的分批抽出共用；page 给任务的端口与每批的单元；回收站一项按深度优先的先序分批恢复与它的边界；撤销的分批再计划；很多组一起读；最上层的例外改比条目的时刻；契约、权限、实测。
- **B（前端）**：回收站、预览与撤销确认框的 422 改为问是否在后台进行；任务列表认识两种新任务与"在历史里查看"；e2e HI5、TB2。

A 的网页改动只有契约带来的门禁：新类型的标题、新失败的文字、被新类型说错了的旧文字、假接口里新的必有字段，同 P3A。

## 1. 基线

- A 从 `main` 的 `1f0e3a98` 开始（P3A 合并于 `314d9ec`，P3B 合并于 `16468979`；服务端与契约自 `e08745e` 起没有改动）。实施从 `main` 的 `1f732c96` 开始，审查之前变基到 `3cb0c51e`（第 9 节）。
- **transfer**：
  - 任务只有 `import`、`export` 两种（`00029:15`、`domain/job.go:9-15`）。
  - 一本笔记本一个导入，靠三样：唯一索引 `transfer_jobs_importing_key`（`00031:5-6`，只认 `import`）、`Importing` 查询、进程内的 `Uploads`（导入的 `Check` 占住笔记本直到写行，`uploads.go:14-56`）。
  - 导入的变更集 id 只在内存（`importRun.changeset`），任务行不记。
  - 分批的上限是 `import_write.go:25-31` 的 100 个节点、8 MiB、20,000 条链接；503 退避重试、单元的"结果未知"、`unitFailure` 都写在导入里。
  - River 的 `Held` 把导入之外的种类都当作导出（`river/jobs.go:148-166`）。收拾与清扫只走 `export`、`import`（`maintain.go:108`、`:187`），清理每个到期的行都删文件（`maintain.go:282-299`）。
  - 报告只有失败与五个计数，问题只在单个任务的读里给出（`jobs.sql:47-51`）。心跳每次写进度，报告至少隔 10 次心跳才写（`job_run.go:113-173`）。
- **page**：
  - 恢复与撤销只在一个单元里做，超过 4.9 的界线答 422（`restore_node.go:79-102`、`revert_changeset.go:97-136`）。
  - `TreeWrites` 只有附件与导入的五个方法（`tree_writes.go:38-62`）。
  - `RestoreNodes` 按传入的 ids 恢复、五个数按这些 ids 数，可以只是一组的一部分（`nodes.sql:134-182`）；`bringBack` 恢复与撤销共用（`unit_restore.go:111-140`）。
  - `PlanRevert` 是纯函数（`domain/revert.go:356-373`）。
  - `revertReads.groups` 每组读一次 `TrashGroup`（根一条、每层一条，`unit_revert.go:254-280`、`trash.go:90-110`）。
  - `NodesNow` 的 `deleted_by_revert` 按变更集的 `created_at` 比（`history.sql:148-168`）。
  - 单元并入的变更集必须已在（`lockInto`，`unit.go:282-300`）；新建的 id 由单元自己取（`ensureChangeset`，`unit.go:306-326`）。
- **网页**：
  - 回收站与撤销确认框的 422 只说太大（`trash-row.tsx:124-125`、`revert-dialog.tsx:166-173`）；预览的同步恢复带路由状态回到回收站页（`trash-preview.tsx:125-129`）。
  - 任务列表只认两种（`transfer-names.ts:7-12`、`transfer-report.tsx:79-97`）。设置里的入口叫"导入与导出"（`zh-CN.ts:474`）。列表运行中每秒轮询（`transfer-page.tsx:42-52`），没有任务的事件。

## 2. 目标与范围

做的事对应 M8 设计 4.1、4.2、4.8–4.11、4.13、4.14 的后台部分与第 3 节第 5 条：

- **迁移**：`transfer_jobs` 的两种新类型；新列 `changeset_id`（建行时定下）、`target_changeset_id`、`message`；一本笔记本一个写入任务的唯一索引。
- **transfer**：
  - 新操作 `startTrashRestore`、`startRevert`，从核对到写行占住笔记本（同导入的上传）。
  - 两种任务的运行：共用从导入抽出的分批、退避、`ANALYZE`、心跳、取消与收拾。
  - 任务行建行时定下它写的变更集（导入同样）。
  - 报告与新的失败。
- **page**：
  - 给任务的端口：核对、计划、每批一个单元。
  - 回收站一项按组的深度优先先序分批恢复：批之间的根与统计、跳过别人恢复的、期限；计划只留骨架。
  - 撤销的分批：每批在锁下对自己的节点再计划；一组太大时同恢复那样分批，续段逐个核对。
  - 很多组一起读：每层一条，同步的撤销也用。
  - 请求里的编辑锁。
  - 最上层的例外改比条目自己的时刻。
  - 后台撤销的条目上限 `MaxJobItems`（实测）。
- 契约、权限矩阵、交错、实测，以及守住它们的测试。
- **前端**：
  - 422 之后问是否在后台进行：回收站页、预览、撤销确认框。
  - 任务列表的两种新任务、进度、报告、"在历史里查看"。
  - 被新类型说错了的文字。
  - e2e HI5、TB2。

不做：

- 途中停下的任务"继续"。停下的状态都是一致的，见 3.4、3.5；"继续"写进 M12 的移交。
- 停在一组中间的撤销任务回到任务之前：回收站的几项合不回一项（3.5）。
- 任务等编辑锁放开：开始时有锁就整体拒绝，途中有了锁就停下（3.5）。
- MCP 开的后台任务：`transfer_jobs.client` 只许 `web`、`api`，是 M9 的事。
- 搜索：P5。

### 2.1 对 00 号文档的修订

P4 的文档提交随这份文档一起改 `00-M8-design.md`。行号指 `1f0e3a98` 的，理由在本文各节。

1. **分批的上限**（4.10 :194）：
   - 批照导入的上限切：100 个节点、8 MiB 正文、20,000 条链接（`import_write.go:25-31`）。
   - 写回的历史至多 `MaxUnitHistory`。page 在每批解析之前以已有的 `TrashHistory` 数；多于一步而超过时答 `ErrBatchTooLarge`，transfer 把批减半，同链接。
   - 单独超过的一步自成一批，包括一页自己的历史超过 `MaxUnitHistory` 的：去掉"按历史的行分批"。
   - 理由：一页的版本与条目若与节点分开恢复，中途停下时只有两种结果，都不对：
     - 已删的节点带着活着的版本与条目：违反 `checkPages` 的"已删，跟随它的却没删"；节点被清理时外键级联把它们一起删掉，它们的变更集不被标上 `trimmed`，之后会整组撤销一个缺了条目的变更集。
     - 活着的页缺版本：版本链断开，之后被清理拿走。
   - 而这一页的删除曾在一个单元里提交，恢复约是删除的 1.2–1.9 倍（[02-P2](02-P2-trash.md) 3.11 的表），P4A 实测它（3.11）。
   - 同一段"两批写同一页的正文"改为：M8 的任务里一页的正文至多写一次（撤销的单轮每页至多一步写回，恢复不写版本，导入每页建一次），这种情形不出现；`RecordRevision` 的版本链守卫照旧是最后的防线（3.5）。
2. **恢复的任务**（4.10 :195）：
   - "之后按层"改为按组的深度优先先序：孩子按次序值与 id，父节点总在孩子之前，同名先到的留名不变。
   - 一批恢复先序里连续的一段，要设上根的只有这一段右边缘上各祖先余下的孩子。每批设根的数与途中停下时剩下的项数都约是"层数 × 扇出"，不是一整层（3.4，审查 A-I2）。
   - 第一批在锁下核对：根仍以 T 删除、仍是根、统计等于计划读到的。不在每批重读整组：组里不是根的节点谁也改不了（3.4）。
   - 之后每批只读自己的节点：不再以 T 删除的跳过；父节点不活着、放回去太深，停下 `tree_changed`；名称照旧加序号；期限照旧。
   - 已恢复的父节点之下仍在回收站的节点，在同一个单元里设上根与准确的统计。
   - 计划只留每个节点的 id、父节点、类型、正文大小与组内统计，每批按 id 读这一批的成员（3.4，A-M8）。
3. **撤销的任务的删除**（4.10 :196；4.2 :93；4.9 :189）：
   - "删除从叶子往上，一批之内的删除用任务开始时定下的同一个时刻"改为：每个要删的最上层连同它的子树，在一个单元里以单元的时刻删除（同 `Unit.Delete`）；一批是若干个这样的删除（3.5）。
   - 理由：任何一次停下，每个最上层要么整棵还在、要么整棵在回收站里；撤销这次撤销也按最上层整组回来。`DeleteNodes` 的 `stray`、`RecordItems` 的删除版本号都不用跨单元（[P3](03-P3-log-revert.md) 第 8 节的那两条随之不需要）。
   - 这重开了 M8 设计审查的 A-I3、C-I3 与 M7 移交第 4 项（"不能一个事务删几万个节点"）。依据是之后的实测：P2A 的 `BenchmarkTrashDelete` 一个单元删 5 万个节点 1.8–2.1 秒，P3 重跑 1.6–2.1 秒（03 号文档 3.11"别的"）。
   - 代价：深的导入的撤销一个单元删它整个最上层，同用户自己 `DeleteNode` 它。P4A 在 `MaxJobItems` 处以满负载的页实测，照 5 秒的规则（3.11）。
   - 4.9 :189 的"（一次 5 万个条目的导入在一个单元里删不完，A-I3、C-I3）"改为"（超过界线的撤销在后台分批；一个最上层的子树在一个单元里删，同 `DeleteNode`，P4 在 `MaxJobItems` 处实测）"。
   - 4.2 :93 的"一次删除的根之下、早先以同一时刻删除的（分批撤销一次导入时，4.10，跨单元）随之取消，统计重算"一句去掉。
4. **撤销的任务的分批**（4.10 :196）：
   - 按开始时的计划的步切批。每批在锁下，以只含这一批的步的节点的状态再跑 `PlanRevert`（3.5）。
   - 一组要恢复的节点超过一批时，同恢复的任务那样按先序分批：
     - 第一批带着整组再计划。
     - 之后各批的成员（续段）不进 `PlanRevert`，逐个核对，冲突照撤销的原因，不加序号（第 11 节第 6 条，A-M3）。
     - 读者先恢复了、正好在它"前"的位置上的续段成员，跳过（A-M9）。
   - 第一个冲突、第一个被锁的页就停下。
5. **请求的答复**（4.10 :193、:196；4.14 :244）：
   - `startRevert` 在请求里未加锁地跑 [P3](03-P3-log-revert.md) 3.6 的第 1–7 步，经 `revertCheck`，各个界线作为参数留在 P3 的位置上：同步的次序不变（A-M2、B-M1）。
   - 条目的 422 之后、读任何东西之前答 `transfer.busy`（B-M2）。
   - 计划之后、冲突之后：计划要删或要写回的页正被别人编辑的，答 409 `page.locked`（第一页），不开任务（第 11 节第 3 条，B-M3）。
   - 唯一留下的 422 是条目超过 `MaxJobItems`，连后台也做不了；另有实测才定的 `MaxJobGroups`（3.11）。
   - `startTrashRestore` 只核对这一项（404、403、三种 409），没有 422：任何一项都能在后台恢复。
   - 撤销的说明（`message`）记在任务行上，第一批建变更集时写进去。
6. **4.9**（:189 之后）加一段：
   - 后台撤销的变更集至多 `domain.MaxJobItems` 个条目，超过答 422，网页说连后台也撤销不了。
   - 请求与任务开始时各读一遍它的全部条目、节点、组与树。很多组按层一起读，每层一条语句，同步的撤销同样（A-I1）。
   - 实测取值：请求满负载不超过 5 秒（含很多组的形状）；最大的一步（一个最上层的删除、一组的第一批）不超过 5 秒。它要覆盖第 3 节第 5 条的导入（2.2）。
   - 后台的恢复没有这样的界线（删除也没有）：节点超过 `MaxJobItems` 的恢复任务，它的变更集撤销时答 422，再删一次恢复回来的根也是一样（A-M4）。
7. **报告与任务行**（4.10 :197）：
   - 恢复的任务报恢复的页数、附件数、改名数、跳过数（别人先恢复了的）与根放在哪里。
   - 撤销的任务报恢复、删除、放回的节点数与写回的页数；冲突的节点（同撤销的原因与 `name`、`holder_id`，至多 100 项与总数，只在单个任务的读里）；被锁的页与持锁人。
   - 剩下的数目就是进度的总数减已处理的。
   - 任务行的 `changeset_id` 在建行时定下，任务写的第一个单元以这个 id 建变更集（A-M7）。任务行另记 `target_changeset_id` 与撤销的说明。
   - 撤销的任务的 `name` 是被撤销的变更集的类型（网页据此写"撤销一次导入"）。
   - 设置里的入口改名"导入、导出与恢复"。
   - 新的失败：`trash_changed`、`restore_expired`、`revert_conflict`、`changeset_trimmed`、`page_locked`（3.6）。
8. **最上层的例外的时刻**（4.8 :153）："'C 之后'按单元的时刻"改为按条目自己的时刻（3.7）。理由：后台的任务并入自己的变更集，后面的批在之后的时刻写，变更集却建在第一批。
9. **4.14**：
   - 码的次序加两个开始（3.6）。
   - 加锁（:246）加两句："任务的每一批是树的单元，批之间放开笔记本行"；"两个开始从核对到写行占住笔记本，同导入的上传"。
   - 交错（:248）的"恢复的任务与在已恢复的部分里删除、在已恢复的父节点下新建同名的页"改为 3.10 的列表。
10. **名称与 transfer 的一句**（4.1 :72、:75；第 8 节 :329、:332）：
    - "`TreeWrites` 加 `RestoreBatch`、`RevertBatch` 与它们的读"改为 `CheckRestore`、`PlanRestore`、`CheckRevert`、`PlanRevert`、`RunBatch`（3.3）。
    - :332 的"导入报告的 `changeset_id`"改为"任务行的 `changeset_id`（建行时定下；导入、恢复、撤销）"。
11. **4.13**（:227、:228）："422 时说明太大（P4 起问是否在后台进行）"写成 4.2 的流程；任务列表一句（4.3）。
12. **第 5 节**（:266–269）：两个开始的请求体、答复与码（`startRevert` 含 `page.locked`）；`listTransferJobs` 等一行加 `target_changeset_id`、报告的新成员。
13. **第 9 节**（:342、:352、:354、:357）：
    - HI5 撤销的导入是根下 1,501 篇空的笔记。过单轮的步数界线才答 422：撤销一次导入不解析，页数的界线碰不到它。单个变更集页显示"1,000+"个节点。
    - TB2 恢复的子树有 102 页，过页数的界线。e2e 以数据库连接持着笔记本行，任务停在第一批之前，看得到进度（4.5）。
    - 后台任务一条加：批之间的交错、每个批的边界上停下、停下的状态可以撤销、导入的撤销在 `MaxJobItems` 的规模计时。
14. **第 10 节**（:367）：缓解一栏加：
    - 停下的状态是一致的：恢复剩下的各自是回收站的一项。
    - 撤销的任务没停在一组中间时，它的变更集撤销之后可以再整组撤销。
    - 停在一组中间时，逐项恢复剩下的就完成这一组（3.5）。
15. **4.2 :91**："`transfer_jobs_importing_key` 改为覆盖……"改为"删掉 `transfer_jobs_importing_key`，建 `transfer_jobs_writing_key` 覆盖 `import`、`trash_restore`、`revert`"；`changeset_id` 一句加"建行时定下"。
16. **4.8 :175**："transfer 在 `Tree.Import` 的结果上接住它"改为"在 `Tree.Import` 与 `RunBatch` 的结果上接住它"。
17. **第 3 节第 5 条**（:53）："一次默认上限（5 万个条目）的导入可以撤销"改为按变更集的条目说："一次在默认上限之内的导入可以撤销：5 万个 zip 条目，路径隐含的文件夹也各是一页，至多约 55 万个条目；`MaxJobItems` 实测不够时导入另有与它相同的节点上限"（2.2，A-M6、B-M4）。
18. **写给后面的 M**：
    - M11（:306）：加"停下时：没停在一组中间的，撤销那次撤销回到之前、再整组撤销；停在一组中间的，逐项恢复剩下的（3.5）"。
    - M12（:307）：加"后台任务的实测、很宽的一层停下时的碎片、计划的内存、'继续'（P4，第 16 项）"。
19. **第 13 节**：加一行变更记录（P4 设计）。

### 2.2 需要负责人确认

1. **第 3 节第 5 条与 `MaxJobItems`（条件项，A-M6、B-M4）**：

   **要覆盖的数**。按变更集的条目数，默认上限之内的导入至多建 N = 55 万个节点：
   - zip 至多 5 万个条目（`import_max_entries`）。
   - 每个文件夹是一页，不论有没有同名的 `.md`（`import_plan.go:64-111`）。
   - 一个条目之上至多 10 个由路径隐含的文件夹页：页至多 10 层（`MaxDepth`），附件挂在第 10 层的页下。

   真实的库约是条目数的 1.1–1.2 倍。例如 4.5 万篇笔记、5 千个附件、8 千个没有同名笔记的文件夹，约 5.8 万个条目。

   `MaxJobItems` 要不小于 N，第 3 节第 5 条才照 2.1 第 17 条成立。P3A 的数推到 55 万：读 C 约 8–15 秒，一个最上层的删除约 20 秒。所以预计实测会小于 N。P4A 照实测的值做：
   - **`MaxJobItems` ≥ N**：没有要做的。
   - **5 万 ≤ `MaxJobItems` < N**：P4A 照 (a) 做，不停下（主持者的决定）。(a) 是给导入加一个节点的上限，等于 `MaxJobItems`：
     - 导入的计划（条目加隐含的文件夹）建的节点超过它时整包失败，同条目超过上限。
     - 新失败 `too_many_nodes`，transfer 的契约、M7 的文档与网页的文字随之改；`MaxJobItems` 由组合根交给 transfer。
     - 之后第 3 节第 5 条对任何在上限之内的导入都成立，被拒的只是路径很深、很碎的 zip。
     - 负责人可以否决 (a)，例如宁可降低第 3 节第 5 条的要求，也不拒绝这样的 zip；那时撤掉这个上限、改第 3 节第 5 条。
   - **`MaxJobItems` < 5 万**：连平铺的默认上限的导入也撤销不了；加节点的上限就等于降低默认的条目上限。停下来问负责人。

   **结果**（P4A）：满负载定为每页 20 条解析的链接之后，`MaxJobItems` = 60,000（3.11），5 万 ≤ 它 < N：照 (a) 做了，导入的节点上限 60,000（`too_many_nodes`，3.6、3.9）。负责人可以否决。

第 11 节的 12 条都不受影响：

- **第 3 条**（被锁整体拒绝）：在请求里照做，`startRevert` 答 409 `page.locked`，不开任务。
- **第 7 条**（后台分批，不是整组的，途中停下时已完成的部分留着；与导入共用一个写入任务与队列）：照做。开始之后才有的锁让任务停下，落在这一条里。
- **第 6 条**（撤销删除放回原位置、放不回就冲突）：在分批的组里照做，不加序号。

## 3. A：服务端

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `server/migrations/sql/00039_transfer_background_jobs.sql` | 两种新类型、三列、约束、排队的导入的变更集 id，一本笔记本一个写入任务的唯一索引（3.2） |
| `page/domain/jobs.go` | `JobStep`；`GroupPreOrder`（组的深度优先先序；`PreOrder` 这个名字已有，偏离 4）；`GroupCounts`（一组里每个节点的子树在组内的页数、附件数、层数）；`Frontier`（一批之后要设上根的节点与统计）；`MaxJobItems` = 60,000（没有 `MaxJobGroups`，3.11） |
| `page/domain/errors.go` | `ErrBatchTooLarge`（未分类：调用者减半，3.3） |
| `page/app/ports.go` | `NodeWriter.MarkTrashRoots`；`Trash.TrashNodesAt`、`TrashGroups`、`TrashShape`；`History.ChangesetItemsOf`、`ChangesetVersionsOf` |
| `page/app/unit.go` | `UnitSpec.Create`：任务的变更集由它第一个写了东西的单元以给定的 id 建（3.3） |
| `page/app/jobs.go` | `JobWrites`：`CheckRestore`、`PlanRestore`、`CheckRevert`、`PlanRevert`、`RunBatch`；`JobPlan`、`BatchDone`；每批的读与解析（单元之前） |
| `page/app/unit_restore.go` | `Unit.RestorePart`（恢复的任务的一批、撤销里一组的一段）：放置（只第一段）、名称、`bringBack`、`markFrontier` |
| `page/app/unit_revert.go`、`revert_changeset.go` | `Unit.RevertBatch`（只含这一批的节点的再计划、续段的逐个核对、应用，删除走 P3 的路）；`revertReads.groups` 改用 `TrashGroups`；未加锁的核对抽成 `revertCheck(…, revertBounds)`，`RevertChangeset` 与 `CheckRevert` 共用 |
| `page/adapter/postgres/queries/trash.sql`、`nodes.sql`、`history.sql`；`store.go`、`trash.go`、`history.go` | `TrashNodesAt`、`TrashChildrenAt`、`TrashShapeOfAll`、`MarkTrashRoots`、`ChangesetItemsOf`、`ChangesetVersionsOf`；`TrashGroups` 按层；`NodesNow` 的例外按条目的时刻（3.7）；回收站已有的四条读按键连接，`changesets.sql` 的 `RecordItems` 按键读删除的页的版本号（实测找到的两个缺陷，3.3） |
| `page/tree_writes.go`、`page/module.go` | `TreeWrites` 加五个方法与 `JobSpec`、`JobPlan`、`JobStep`、`BatchDone`；导出 `ActionRestore`、`ActionRevert`、`ErrBatchTooLarge`、`ErrRestoreExpired`、`ErrRevertConflict`、`ErrChangesetTrimmed`、`ErrLocked`；`Deps.MaxJobItems`（0 即 domain 的值） |
| `transfer/domain/job.go`、`report.go`、`errors.go` | `KindTrashRestore`、`KindRevert`；`Job.ChangesetID`、`TargetChangesetID`、`Message`；报告的 `Placed`、`Conflicts`、`ConflictCount`、`Lock` 与四个新计数；五个新失败；`ErrImportBusy` 改名 `ErrWriting`，详情说写入任务 |
| `transfer/app/batches.go`（新）、`import_write.go`、`import.go` | 从导入抽出的：批的上限（`Deps.Batch`）、`retry`/`busy`、单元的"结果未知"与 `unitFailure`、`ANALYZE` 的节拍；导入改用它，并入任务行的变更集；导入的节点上限（`ImportDeps.MaxNodes`，2.2 的 (a)） |
| `transfer/app/start_background.go`（新）、`background.go`（新） | `StartTrashRestore`、`StartRevert`；两种任务共用的 `Background.Run` |
| `transfer/app/ports.go`、`import_ports.go`、`start_import.go`、`maintain.go`、`views.go` | `Tree` 加五个方法；`Rows.Writing`（代替 `Importing`）；`Queue.TrashRestore`、`Revert`；收拾的种类；清理只删导入、导出的文件；报告里持锁人的显示名。没有加 `Uploads.uploading`：两个开始用已有的 `claim`，它已挡住导入的上传（偏离 3） |
| `transfer/adapter/postgres/queries/jobs.sql`、`store.go` | 新列的读写，`Writing`，列表去掉报告的 `conflicts` |
| `transfer/adapter/river/jobs.go` | 两个 River 种类 `transfer.trash_restore`、`transfer.revert`，在 `transfer_import` 队列上；`Held` 按种类 |
| `transfer/adapter/http/server.go`、`handler.go` | 两个开始的处理器（严格的服务器，不是 `x-raw`） |
| `transfer/module.go`；`bootstrap/transfer.go`、`deps.go`、`wire.go` | `Deps` 加两个动作、`Batch` 与 `ImportMaxNodes`（组合根交 `page.MaxJobItems`）；`transferTree` 的五个方法与错误的映射（`treeError`）；`wireApp` 带批（交错以 3 个节点一批），`newApp` 交零值（偏离 17）；生产接线的守卫核对 `transferDeps` 不设批、导入的节点上限是 `MaxJobItems` |
| `api/modules/transfer.yaml`、`api/modules/page.yaml`、`api/common.yaml`、`api/openapi.yaml` | 两个操作；`TransferKind`、`TransferJob`、`TransferJobDetail`、`TransferReport`、`TransferCounts`、`TransferProgress`、`TransferFailure`；被说错了的描述（3.9）；`TrashPlacement` 移到 `common.yaml`；`PageLock`（新，`Problem.lock` 引用它） |
| `web/apps/web/src/pages/notebook/transfer-names.ts`（另有 `transfer-names.test.ts`，偏离 18）、`transfer-report.tsx`、`i18n/messages/{en,zh-CN}.ts`、`test/jobs-server.ts`；`e2e/stories/transfer/tr1`–`tr4` | 契约带来的门禁：两种类型的标题、五个新失败与 `too_many_nodes` 的文字、三个旧失败按类型的文字、忙的文字（TR2 逐字断言英文）、假接口的必有字段（3.9）；TR1–TR4 以接口比较整份报告，随新的必有计数与 `placed`、`lock` 改（这张表原来只列了 TR2，偏离 30） |
| `bootstrap/background_test.go`、`interleavings_background_test.go`、`interleavings_background_stops_test.go`（停下的交错）、`background_cost_test.go`、`permission_matrix_transfer_test.go`、`transfer_jobs_test.go`（`seededJob.kind`、重启）、`interleavings_transfer_test.go`（`checkTransfers` 的种类）、`interleavings_history_test.go`（交错 5 改读任务行的 `changeset_id`）；`page/adapter/postgres/jobs_test.go`、`history_test.go`（V）；`transfer/app/background_test.go`、`start_background_test.go`、`transfer/adapter/http/start_background_test.go`；`migrations/background_jobs_test.go`；`server/sqlc.yaml`（列上 00039，偏离 1）；`Makefile`、`.github/workflows/ci.yml`；`server/configs/config.yaml`、`README.md`（导入的节点上限） | 3.12、3.11。数组语句的两种计划在组合根的基准里量（`BenchmarkBackgroundSteps/arrays`，在真的撤销留下的回收站上），没有 `page/adapter/postgres/jobs_cost_test.go`（偏离 25） |

### 3.2 迁移

**`00039_transfer_background_jobs.sql`**（归 transfer，13.1 第 7 条）：

```sql
-- +goose Up
-- The background restores and reverts (M8/P4 design 3.2): a trash item's restore (root_id the item) and a changeset's
-- revert (target_changeset_id the changeset, message its revert's), and the changeset each writing job writes, an
-- import's too: its id decided as the row is made, the changeset made by the job's first unit that writes, with it.
-- An import made before has none. No foreign keys: the changesets are the page module's, purged by its purger, which
-- a job's row must not hold.
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export', 'trash_restore', 'revert')),
    ADD COLUMN changeset_id uuid,
    ADD COLUMN target_changeset_id uuid,
    ADD COLUMN message text,
    ADD CONSTRAINT transfer_jobs_changeset_check CHECK (CASE kind WHEN 'export' THEN changeset_id IS NULL
        WHEN 'import' THEN true ELSE changeset_id IS NOT NULL END),
    ADD CONSTRAINT transfer_jobs_target_check CHECK ((target_changeset_id IS NOT NULL) = (kind = 'revert')),
    ADD CONSTRAINT transfer_jobs_message_check CHECK (message IS NULL
        OR kind = 'revert' AND octet_length(message) BETWEEN 1 AND 4096),
    ADD CONSTRAINT transfer_jobs_root_check CHECK (CASE kind WHEN 'trash_restore' THEN root_id IS NOT NULL
        WHEN 'revert' THEN root_id IS NULL ELSE true END);
-- A queued import runs as the new ones do, merging into the changeset its row names; a running one is interrupted
-- by the restart this migration runs in, and runs no more.
UPDATE transfer_jobs SET changeset_id = uuidv7() WHERE kind = 'import' AND state = 'queued';
-- One writing job queued or running in a notebook at once, whoever started it (M8 design 4.10): an import, a
-- restore and a revert each write the notebook's tree in units that would race another's.
DROP INDEX transfer_jobs_importing_key;
CREATE UNIQUE INDEX transfer_jobs_writing_key ON transfer_jobs (notebook_id)
    WHERE kind IN ('import', 'trash_restore', 'revert') AND state IN ('queued', 'running') AND deleted_at IS NULL;

-- +goose Down
DELETE FROM transfer_jobs WHERE kind IN ('trash_restore', 'revert');
DROP INDEX transfer_jobs_writing_key;
CREATE UNIQUE INDEX transfer_jobs_importing_key ON transfer_jobs (notebook_id)
    WHERE kind = 'import' AND state IN ('queued', 'running') AND deleted_at IS NULL;
ALTER TABLE transfer_jobs DROP CONSTRAINT transfer_jobs_root_check, DROP CONSTRAINT transfer_jobs_message_check,
    DROP CONSTRAINT transfer_jobs_target_check, DROP CONSTRAINT transfer_jobs_changeset_check,
    DROP COLUMN message, DROP COLUMN target_changeset_id, DROP COLUMN changeset_id,
    DROP CONSTRAINT transfer_jobs_kind_check,
    ADD CONSTRAINT transfer_jobs_kind_check CHECK (kind IN ('import', 'export'));
```

- **`changeset_id`**：
  - 写入任务的 `changeset_id` 在开始的事务里建行时取（`uuidv7()` 的值由程序取，同别的 id）。
  - page 在任务第一个写了东西的单元里以这个 id 建变更集（3.3），之后的单元并入。所以不论任务在哪里停下、进程何时中断，任务行都指着它写的那个变更集；什么都没写的任务指着一个不存在的变更集，网页这时不给链接（4.3）。
  - 不留空的变更集，page 也不写 transfer 的表（A-M7）。
  - `expired`、`result_bytes` 的约束只许导出，新类型不碰（`00029:50-52`）。
- **唯一索引**改名、覆盖三种写入任务，撞上即 409 `transfer.busy`；`store.go` 的约束名映射随之改。
- **迁移测试**：
  - `schema_test.go` 核对新约束与索引的名字、旧索引不在。
  - `TestChecksRejectCounterexamples` 加反例：导出带 `changeset_id`、恢复或撤销没有 `changeset_id`、撤销没有 `target_changeset_id`、恢复带 `message`、空的 `message`、恢复没有 `root_id`、撤销带 `root_id`。
  - Up 带数据跑：排队的导入取得 id，结束了的导入留空。
  - Down 带数据跑一次（P3A 审查 A-4 的教训）：有一个恢复的任务与一个带 `changeset_id` 的导入时，Down 成功、导入留着。
- **Down 与 River**：Down 之后，River 里这两个种类的排队任务没有 worker。River 以"没有这个种类"失败它们，`MaxAttempts: 1`，不重试。Down 只在开发时跑（A-N7）。
- **page 没有迁移**：新的读都走已有的索引（3.3）。
- **实施时**：`server/sqlc.yaml` 列上 00039（sqlc 只读列出的迁移，偏离 1）；迁移测试的一个反例由"第三种任务"改为"第五种任务带 `changeset_id`"：PostgreSQL 按约束的名字的次序核对，否则先撞上类型的约束（偏离 2）。

### 3.3 page：给任务的端口与查询

**模块根**（`tree_writes.go`，组合根经 `bootstrap/transfer.go` 适配给 transfer 的 `Tree` 端口，同导入）：

```go
// JobSpec is a background job's unit's (M8/P4 design 3.3): the job's notebook and client; Changeset the id its row
// names, which its first unit that writes makes the changeset with and its later units merge into; Message a
// revert's, written as it is made.
type JobSpec struct {
	NotebookID uuid.UUID
	Client     Client
	Changeset  uuid.UUID
	Message    *string
}

// A background restore's and revert's writes, besides TreeWrites' import:
//	CheckRestore(ctx, notebookID, nodeID uuid.UUID) (name string, err error)
//	CheckRevert(ctx, notebookID, changesetID uuid.UUID, message *string, admit func(context.Context) error) (kind string, err error)
//	PlanRestore(ctx, spec JobSpec, nodeID uuid.UUID) (JobPlan, error)
//	PlanRevert(ctx, spec JobSpec, changesetID uuid.UUID) (JobPlan, error)
//	RunBatch(ctx, spec JobSpec, plan JobPlan, from, to int) (BatchDone, error)
```

- **`Check*`** 在请求里、不加锁，码与次序见 3.6。答出任务行的 `name`：恢复是这一项的名称，撤销是被撤销的变更集的类型。`admit` 是 transfer 的忙的核对，`CheckRevert` 在条目的界线之后、读任何东西之前调用它（B-M2），它的错误原样传出。
- **`Plan*`** 在任务开始时、不加锁：
  - 先 `Writer.Allowed`：失去权限 → 任务 `forbidden`。
  - 读出整个计划。`JobPlan` 是 page 的值，transfer 只经 `Steps() []JobStep` 与 `Total() int` 读它，原样交回 `RunBatch`。
  - `JobStep{Nodes int; Bytes int64}` 是不可再分的一步写：
    - 恢复：一个节点。
    - 撤销：放回一个节点、删除一个最上层的子树、恢复组里的一个节点、写回一页。
  - `Nodes` 是这一步碰的节点（写回算 1），`Total` 是它们的和；`Bytes` 是要解析的正文。
- **`RunBatch`** 写第 `from` 到 `to` 步（不含 `to`）：
  - 单元之前：读这一批要解析的正文，以 `TrashHistory`（已有，按参数规划）数这一批要写回的历史。
  - 多于一步而历史超过 `MaxUnitHistory` 时，答 `ErrBatchTooLarge`，在解析之前；解析之后链接超过 20,000 时同样答它。单独一步不判。
  - `DecidedAll`：从大到小，等待合计有界，取不到预算答 503。
  - 然后一个单元：`Writer.Run`，`Tree: true`，`UnitSpec{Changeset: spec.Changeset, Create: true}`。
  - 答 `BatchDone{Nodes, Pages, Assets, Renamed, Skipped, Restored, Deleted, PutBack, Written, Placed}`。
  - 批的节点数与字节由 transfer 切（3.6），历史与链接由 page 判，所以 transfer 不必知道 page 的界线。
  - `RunBatch` 只答它的哨兵与单元的错误；`KindNotFound` 只来自 `Writer` 的判定（→ `forbidden`）。不在了的 C 答 `ErrChangesetTrimmed`，不是 404（B-N11）。
- **`UnitSpec.Create`**（`unit.go`）：`lockInto` 找不到 `Changeset` 时，若 `Create`，就在这个单元第一次写时以这个 id 建变更集（`ensureChangeset`）；找到了照旧核对、并入；不写就不建。导入同样：它的每个单元带任务行的 id（3.6）。

**新的查询**：

```sql
-- name: TrashNodesAt :many
-- Nodes of the notebook, each deleted at its own time, by key (M8/P4 design 3.3): many trash items' roots at once, or
-- a background job's batch's members; with a root's counts, and a page's content's revision and size as
-- TrashChildrenOfAll reads them. One no longer deleted at its time is not read.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, n.trash_root, n.trash_pages, n.trash_assets, n.trash_height, c.revision, c.byte_size
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(deleted_ats)::timestamptz[]) AS deleted_at) g
JOIN nodes n ON n.id = g.id AND n.deleted_at IS NOT DISTINCT FROM g.deleted_at
LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.notebook_id = sqlc.arg(notebook_id);

-- name: TrashChildrenAt :many
-- The children of parents of the notebook, each deleted at its parent's time (M8/P4 design 3.3): a level of many trash
-- items' groups in one statement, through the siblings' index as TrashChildrenOfAll reads one group's, in its order:
-- each group's part of the level is the one TrashGroup reads.
SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
    n.created_at, n.updated_at, c.revision, c.byte_size
FROM (SELECT unnest(sqlc.arg(parents)::uuid[]) AS id, unnest(sqlc.arg(deleted_ats)::timestamptz[]) AS deleted_at) p
JOIN nodes n ON n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id = p.id AND n.deleted_at IS NOT DISTINCT FROM p.deleted_at
LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
ORDER BY n.sort_order, n.id;

-- name: TrashShapeOfAll :many
-- A level of a trash item's group as a background restore's plan keeps it (M8/P4 design 3.4): no names, a page's
-- content's size; by parent, each parent's children in order, as TrashChildrenOfAll reads them.
SELECT n.id, n.parent_id, n.kind, c.byte_size
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NOT DISTINCT FROM n.deleted_at
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id = ANY(sqlc.arg(parents)::uuid[])
    AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz
ORDER BY n.parent_id, n.sort_order, n.id;

-- name: MarkTrashRoots :execrows
-- The nodes of a trash item a background restore left in the trash under parents it brought back (M8 design 4.10;
-- M8/P4 design 3.4): each its own trash item, with its part of the group's counts and height. Joined by key, as
-- RestoreNodes; a node no longer deleted at the time, or a root already, is not marked, which its caller counts.
UPDATE nodes n SET trash_root = true, trash_pages = u.pages, trash_assets = u.assets, trash_height = u.height
FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(pages)::integer[]) AS pages,
        unnest(sqlc.arg(assets)::integer[]) AS assets, unnest(sqlc.arg(heights)::integer[]) AS height) u
WHERE n.id = u.id AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz AND NOT n.trash_root;

-- name: ChangesetItemsOf :many
-- A changeset's items of some nodes, by its key on (changeset_id, node_id): a background revert's batch's (M8/P4 design
-- 3.5), as ChangesetItems reads them all.
SELECT node_id, before_parent_id, before_name, before_sort_order, after_parent_id, after_name, after_sort_order,
    restored, revision
FROM changeset_items WHERE changeset_id = sqlc.arg(id) AND node_id = ANY(sqlc.arg(node_ids)::uuid[]) ORDER BY node_id;
```

（实施的样子：`TrashNodesAt`、`TrashChildrenAt`、`TrashShapeOfAll` 在 `trash.sql`，`MarkTrashRoots` 在 `nodes.sql`，`ChangesetItemsOf`、`ChangesetVersionsOf` 在 `history.sql`；按键连接与时刻的比较见下。）

- **`TrashGroups(notebookID, roots map[uuid.UUID]time.Time)`**：很多组一起按层读，第一层是 `TrashNodesAt`，之后每层一条 `TrashChildrenAt`。任何多的组都是"层数"条语句（实际至多约 11 层），不再每组两条以上（A-I1）。`TrashChildrenAt` 照 `TrashChildrenOfAll` 按次序值与 id 排，store 在 Go 里按组分开（这里原来先按 `parent_id` 排；一种次序给两个调用者，偏离 5）。
  - `revertReads.groups` 改用它，同步的撤销、`CheckRevert`、任务开始与每批都经它。
  - `TrashGroup`（一组）留给同步的恢复。
- `ChangesetVersionsOf` 同 `ChangesetVersions` 加 `AND r.node_id = ANY(...)`。
- `MarkTrashRoots` 答的行数不等于传入的个数是 `ErrTrashChanged`。这是防线：组里不是根的节点谁也改不了（3.4）。
- 六条数组语句都经 `s.planned`（按参数规划，同 `TrashChildrenOfAll`）。3.11 比过两种计划：通用计划没有灾难性的，慢的至多 3.9 倍（`MarkTrashRoots`），快的至多约两成；都留在 `planned`（改回缓存只省一次往返，取保守的默认）。`NodesNow` 照 P3 是缓存的语句：它的通用计划不慢（5 万个 id 252 对 270 ms；实施报告一度说它也在 `planned`，P4A 审查 A-N1 改正）。计划测试（`TestTheJobsReadsUseTheirIndexes`）核对它们走主键、`(changeset_id, node_id)` 键与兄弟的索引（`nodes_notebook_id_parent_id_idx`）；一层的读取 100 个父节点里的 10 个（一层的一部分，偏离 12）。
- `RestoreNodes` 不改，按批的 ids 用。`NodesNow` 的签名不改，例外改了（3.7）。
- **按键连接**（3.11 的实测找到的两个缺陷，偏离 20）：
  - 回收站的读以节点的删除时刻连接 `page_contents` 时，规划器在大的删除之前的统计下（这次删除的行一个都没有）经 `deleted_at` 的部分索引走，每个节点读这一时刻的每一行：4.5 万页的一项的后台恢复，计划 55 秒（10 万个节点 222 秒，20 万 1,107 秒）。七条读（`ListTrash`、`FindTrashRoot`、`TrashGroupRoot`、`TrashChildrenOfAll` 与上面的三条）改为按键连接、以 `IS NOT DISTINCT FROM` 比时刻（没有索引能用它；比的都是已删的行与删除的时刻，等同相等）：5 万处 161 ms，55 万处 1,045 ms。
  - P3 的 `RecordItems` 读删除的页的版本号是同一个陷阱：一个 4.5 万页的最上层，任务的第一批 92 秒，用户的 `DeleteNode` 过了 15 秒的期限（500）。改为按键读、之后比时刻：2.5 秒。
  - 计划测试在大的删除之前的统计下钉住它们（`TestTheTrashsReadsJoinByKeyWhateverTheStatistics`、`TestADeletionsItemsReadTheirPagesContentsByKey`）；最大一步的守卫也抓到 `RecordItems` 的回归（3.11）。

### 3.4 恢复的任务：计划、分批与每批的核对

**计划**（`PlanRestore`，不加锁）：

1. `FindAnyNode` → `trashRoot`（`trash.go:98`）：不在回收站、不是根、过期的，任务以 `trash_changed`、`restore_expired` 停下，什么都没写。
2. 以 `TrashNodesAt` 读根与它的统计。
3. `TrashShapeOfAll` 按层读组的骨架。
4. `domain.GroupPreOrder` 排成深度优先的先序（孩子按次序值与 id；`PreOrder` 这个名字已有，偏离 4），`domain.GroupCounts` 一遍算出每个成员在组内的子树的页数、附件数与层数；算出的根的统计与根上记下的不等时同样 `trash_changed`（偏离 13）。

步是组的成员，按先序：父节点总在孩子之前，同一父节点下的相对次序同 `TrashGroup`，所以 P2 的"组内同键先到的留名"不变。

计划只留骨架：每个节点的 id、父节点的下标、类型、正文大小与组内统计，约 40 字节。按 `TrashMember` 留约 300 字节、带名称，最大的一项会到几百 MB：
- 删除没有界线，一项可以是 2.2 的 55 万个节点的导入。
- 按 `TrashMember` 留，55 万个约 165 MB，`jobs.import_workers` 至多 8 个同时跑；骨架约 22 MB。

每批按 id 读这一批的成员（A-M8）。3.11 量它。

**一批的单元**（`Unit.RestorePart`，笔记本行 `FOR NO KEY UPDATE` 之下；这一批的成员以 `TrashNodesAt` 在 T 读）：

| 核对 | 不成立时 |
|---|---|
| 以单元的时刻 `Restorable(T)` | 停下 `restore_expired`（`ErrRestoreExpired`） |
| 第一批：根仍在这本笔记本、仍以 T 删除、仍是根，统计等于计划的 | 停下 `trash_changed`（这一项在任务开始之后变了） |
| 之后的批：这一批的成员仍以 T 删除（`TrashNodesAt` 读到它） | 不是：别人恢复了它所在的那一项（之前设上根的节点可以被恢复），跳过，`Skipped` |
| 仍以 T 删除、父节点不在这一批：它是根，统计是之前的批设的 | `trash_changed`（防线） |
| 父节点此刻活着（之前的批或这一批恢复的；不在这一批里的父节点以 `FindNodeIn` 读） | 停下 `tree_changed`（`ErrNoParent`：别人删了已恢复的部分） |
| 页的层级：父节点的深度（`Ancestors`，这一批每个不在这一批里的父节点一次）加一不超过 10 | 停下 `tree_changed`（`ErrTooDeep`：别人把已恢复的部分移深了） |

- **放置与名称**：
  - 第一批的根照 [P2](02-P2-trash.md) 3.5：`TrashLines` → `PlacementOf`；原处时次序值不变，根下时放在末尾；`free` 加序号。六种放置（父节点活着、根下原处、正好放得下的深度、父节点在回收站、太深、根下有同名的兄弟）由 `TestARestoreJobPlacesItsRootAsRestoreDoes`（每批一个成员）与经 serve 的 `TestABackgroundRestorePlacesItsRoot` 钉住；正好 10 层的页与它下面的附件可以恢复（P4A 审查 C-M1、C-M2）。
  - 之后的节点留在原父节点下、原次序值。名称与那个父节点下活着的兄弟比（`Nodes.Children`，这一批里每个不在这一批的父节点一次，含之前的批恢复的兄弟；不另加端口，偏离 6），也与这一批先到的同父节点的成员比，撞上加序号。分批之间别人可能在已恢复的父节点下建了同名的页（A-I5）。
- **写**：
  - `bringBack(批里仍以 T 删除的成员, T, 状态, 键, Facts)`。`Restoration` 的 `Root` 是这一批的第一个，续段的第一个的"新"父节点与次序值就是它自己的，`RestoreNodes` 照写不变（`nodes.sql:134-182`）。五个数按这一批核对（`unit_restore.go:127-139`）。
  - 然后 `markFrontier`：这一批恢复了的成员、在这一批之后的孩子，由 `MarkTrashRoots` 设上根与它在组内的子树的统计（`domain.Frontier`）。这一批跳过的成员（别人已恢复的）的孩子不设：它们随别人恢复的那一项回来了（偏离 8；P4A 审查 C-M3 加了测试）。
  - 按先序，这些只是这一批右边缘上各祖先余下的孩子，至多约"层数 × 扇出"。例如根下 50 个文件夹、每个 1,000 页：第一批设约 950 个，之后每进一个文件夹多约 1,000 个；按层时第一批要设 49,951 个（A-I2）。
  - 组里不是根的节点谁也改不了，所以计划读到的子树就是此刻的，统计准确，不必在锁下重读整组：
    - 恢复只认根（`page.not_trash_root`）。
    - 撤销一次删除要这一组的最上层是根且统计相符（`rootHolds`），已恢复了根的组不再符合。
    - 清理被一小时的间隔挡开。
- **变更集与条目**：
  - 第一个写了东西的批以任务行的 id 建类型 `trash_restore` 的变更集，`created_at` 是那一批的时刻；之后的批并入，`TouchChangeset` 只改 `updated_at`。
  - 每个恢复的节点一项恢复的条目：`restored`、`revision`，"后"是它恢复到的位置与名称，时刻是它那一批的。
  - 所以撤销这次任务就是撤销一个普通的 `trash_restore` 变更集（最上层是根，从上往下删），最上层的例外按条目的时刻认出它（3.7）。
- **批之间读者看到的**：
  - 根与已恢复的部分在树里，每批一个 `pages` 事件（`tree: true`）。
  - 设上根的节点各是回收站的一项：原位置是已恢复的父节点，期限不变（`deleted_at` 仍是 T），恢复去原处。读者可以先恢复它们，任务之后跳过。
  - 链到之后的批里的页的链接暂时不解析，那一批恢复时由 linking 的观察者重新解析，同导入的后面的批。
- **清理**：每批以单元的时刻核对期限；清理只取 `deleted_at < now − R`，间隔一小时，任务还在恢复的组清理碰不到。任务因期限停下之后，剩下的各项照常到期、被清理。
- **停下的状态**（取消、`tree_changed`、`restore_expired`、失败、服务重启）：
  - 已恢复的部分活着。剩下的是回收站里的若干项（已恢复的父节点下各自一项，期限照旧，至多约"层数 × 扇出"），可以逐项恢复、回到原处。
  - 不变式 `checkPages` 照旧成立：根 ⇔ 父节点不是同一时刻删除的，统计等于组。
  - 很宽的一层（一页下几千个附件）停在中间仍会留下几千项，第 11 节第 7 条的"不是整组的"包括它，写进网页的报告说明（4.3）。
- **可以撤销**：
  - 任务的变更集（类型 `trash_restore`，各批并入）是普通的变更集。撤销它，把已恢复的部分以撤销的时刻删成一项（最上层是原来的根），设上根的那些仍各是一项。之后那次原来的删除不能再整组撤销：它的组已分成几项，答 `changed`，写明。
  - 节点多于 `MaxJobItems` 的恢复，它的变更集连后台也撤销不了（422）；再删一次恢复回来的根（`DeleteNode`，没有界线）效果相同，不另设界线（A-M4）。
  - 任务完成之后撤销它、再撤销原来的删除，照旧成立（例外按条目的时刻，3.7）。

### 3.5 撤销的任务：计划、分批与每批的再计划

**请求里**（`CheckRevert`，不加锁）：[P3](03-P3-log-revert.md) 3.6 的第 1–7 步经 `revertCheck(ctx, c, spec, b)` 共用。各个界线是参数，位置同 P3：

```go
// revertBounds are where a revert's unlocked check stops it (M8/P4 design 3.5): Items before the items are read
// (MaxUnitNodes, a job's MaxJobItems); Admit right after it, before anything else is read (a job's transfer.busy);
// Groups after the nodes, before their groups are read (a job's MaxJobGroups; 0 none); Parse at step 6, the unit's
// pages, bytes and History rows of history (the sync's); Steps after the plan's conflicts (MaxRevertSteps, the
// sync's; 0 none); Locks after them, page.locked for a page the plan deletes or writes back that an edit session holds (a
// job's). The 404 of a changeset gone between its reads is spec.NotFound, the one the caller's first read answers.
type revertBounds struct {
	Items   int
	Admit   func(context.Context) error
	Groups  int
	Parse   bool
	History int
	Steps   int
	Locks   bool
}
```

- **同步**：`{Items: MaxUnitNodes, Parse: true, History: MaxUnitHistory, Steps: MaxRevertSteps}`（历史的行的界线留在 P3 的位置，偏离 10）。次序与 P3 的表原样，3.12 钉住（A-M2、B-M1）。
- **后台**：`{Items: MaxJobItems, Admit: transfer 的忙, Locks: true}`（没有 `MaxJobGroups`，`Groups` 为 0，3.11）。次序：
  1. 422 说明（`validation_failed`）
  2. 404 `notebook.not_found`
  3. 403 `forbidden`
  4. 404 `page.changeset_not_found`（不是这本笔记本的）
  5. 409 `page.changeset_trimmed`
  6. 422 `page.revert_too_large`：条目超过 `MaxJobItems`
  7. 409 `transfer.busy`：`Admit`
  8. 读条目、版本，再读变更集（不在了：再判定，答 404 `notebook.not_found` 或 403，否则 409 `page.changeset_trimmed`）
  9. `NodesNow`
  10. `TrashGroups`、读其余、`PlanRevert`
  11. 409 `page.revert_conflict`（带 `conflicts`），然后 409 `page.changeset_trimmed`（读不到的版本）
  12. 409 `page.locked`（带 `lock`）
- **编辑锁**（B-M3）：
  - 计划要删的页（各最上层此刻活着的子树里的页，从读到的树取）与要写回的页，以 `AliveSessionsOf(ids, now)` 读一次。
  - 删除被别的账户的会话挡住，写回被任何会话挡住（同 `GuardWrite`，`edit_lock.go:44-73`）。
  - 按计划的次序第一个被挡住的答 409 `page.locked`：第 11 节第 3 条的"被锁整体拒绝"在请求里做到。
  - 任务开始时的计划再核对一次：有锁就停下 `page_locked`，什么都没写。
  - 开始之后才有的锁让任务在那一批停下（第 7 条）。

**计划**（`PlanRevert`，任务开始时，不加锁）：同一遍读、`PlanRevert` 与锁的核对。这期间有了冲突或锁，就以 `revert_conflict`、`page_locked` 停下，什么都没写。

`RevertPlan.Ops` 按单轮的次序展开成步，每一步记下它的节点集：

| 步 | `Nodes` | 节点集 |
|---|---|---|
| 放回一个节点 | 1 | 那个节点 |
| 删除一个最上层 | 它此刻活着的子树的大小 | 最上层与它子树里 C 的节点 |
| 恢复一组的第一步（最上层，带着整组） | 1 | 整组 |
| 恢复一组的续段成员（组的先序） | 1 | 空：续段不进再计划（A-M3） |
| 写回一页 | 1 | 那一页 |

**一批的单元**（`Unit.RevertBatch`，`UnitSpec{Kind: revert, RevertsID: C, Changeset: 任务行的, Create: true, Message}`，`Options` 为空）：

1. **读**：
   - 这一批的步的节点集里 C 的条目、版本（`ChangesetItemsOf`、`ChangesetVersionsOf`），按步的种类读：放回只读条目，写回只读版本，删除与一组的第一步读两者（偏离 7）；再读 C 的行，先读条目、后读 `trimmed`（同 P3 3.3）。截掉或不在 → 停下 `changeset_trimmed`。
   - 续段的成员只读配对要的：`TrashNodesAt`（在组的时刻，带根、统计、版本号与大小）、要解析的 `TrashContents`、配对的 `VersionHashes`。
   - 这一批要写回的历史以 `TrashHistory` 每组一次数（超过 → `ErrBatchTooLarge`）。
   - 第一步在这一批的组，先以一条 `NodesNow`（不带被撤销的变更集，一批至多 100 个根）读它们的根此刻的删除时刻，历史的计数与回收站的正文按它读。组在计划之后被恢复又原样删除一次时，按计划的时刻读不到这一页的正文，任务会以假的 `changed` 停下，历史也少数、这样的批从不减半（P4A 审查 A-M1，交错 21）。
2. **再计划**：
   - 以只含这些条目与版本的状态，在单元的时刻跑 `PlanRevert`。状态包括 `NodesNow`、第一步在这一批的组的 `TrashGroups`、`ContentsNow`、`VersionHashes` 与整本笔记本活着的树。S\* 就是前面的批已应用之后、再应用这一批的样子（单轮的前缀）；还没轮到的 C 的节点，在 S\* 里照现在的。
   - 有冲突 → 停下 `revert_conflict`，报告带冲突（3.6）。
   - `Missing` → `changeset_trimmed`。
   - `ErrRevertUnordered` 是缺陷（500，任务 `internal`）。
   - 删除的一步，最上层子树里有不是 C 的活着的节点（这期间别人放进来的，或别的最上层的 C 节点被移进来），照 `children_added` 答出：一批的节点集只含这一步的 C 节点。
3. **续段**（最上层在之前的批里恢复了的组，这一批里的成员），逐个核对：
   - 仍以组的时刻删除、是之前的批设上的根与统计：恢复它。
   - 不再以那个时刻删除：活着且正在它"前"的父节点、名称与次序值上（读者从回收站恢复了它所在的一项，回到的正是撤销要放的位置）→ 跳过、计入 `skipped`（A-M9）；否则 `changed`。
   - 它"前"的父节点此刻活着（`parent_gone`）；"前"的名称的键在那里空着（`title_taken`，`holder_id` 是占着的）；页的层级不超过 10（`too_deep`）；期限（`expired`）。
   - 冲突的 `name` 同 P3 3.5。不加序号：撤销放回原位置、放不回就冲突（第 11 节第 6 条）。
   - 一个冲突的成员之下、同一批里的成员（父节点没回来）跳过、不另列，`conflict_count` 只数真正的冲突（P4A 审查 A-M3：原来列为 `changed`，排在真正的原因之前）。
   - 成员的版本这期间被清理拿走：`changeset_trimmed`，同计划的 `Missing`（交错 17 找到的缺陷：原来按空的哈希答 `changed`）。
4. **配对**：解析过的正文照 P3 3.6 第 3 步配对：写回配版本 `Base` 的哈希，恢复配回收站里的版本的哈希。只配对这一批要写的页：一组跨批时，第一批再计划出的是整组的恢复，`parsedAsPlanned` 只看这一批里的成员。
5. **应用**，按步的次序：
   - 放回（`place`）。
   - 删除：每个最上层照 P3 的路（`Subtree` → `apply(OpDelete)` → `DeleteNodes`，根与统计，单元的时刻）。一批至多 100 个最上层；没有加一起读很多最上层的 `Subtrees`：每个最上层的子树读不随笔记本增长（3.11）。
   - 恢复的段：`RestorePart`，第一段的最上层放到 C 的"前"，各节点照"前"的名称，之后 `markFrontier`。第一段的组的时刻取锁下再计划的 `DeletedAt`（偏离 13）；单元答出它恢复各组用的时刻（`RevertPart.retimed`），`Writer.Run` 无错返回之后计划才取它，回滚的单元不改计划（P4A 审查 A-M1、A-N2）。
   - 写回：`WriteContent`，基于锁下读到的版本号。
   - 错误：
     - 已分类的错误照 P3 包成未分类：名称被占、版本号不符；再计划已核对过，撞上是缺陷。
     - 守卫的 `page.locked` 原样 → 停下 `page_locked`，报告带被锁的页与持锁人。
     - `ErrTrashChanged`（`RestoreNodes` 的计数、附件的行数）→ `changeset_trimmed`。

- **导入的撤销**（M7 的移交第 4 项）：
  - 一次导入的变更集只有新建（`import_write.go:311-361`），撤销它的计划只有删除，每个最上层一步。
  - 根下平铺的 5 万篇笔记是 5 万步、500 批，每批 100 个最上层，各是回收站的一项。
  - 一个文件夹下的 N 个节点是一步、一个单元，同用户删除那个文件夹；P4A 在 `MaxJobItems` 处实测（3.11）。
  - 不解析正文，没有组。请求与任务开始时各读一遍它的条目（至多 `MaxJobItems`）。
  - 撤销这次撤销，是恢复 5 万组（一起读，各一步），或一组 N 个节点（同恢复的任务分批）。
- **变更集与条目**：第一个写了东西的批以任务行的 id 建类型 `revert`、`reverts_id = C` 的变更集 R（带说明）；之后的批并入，`lockInto` 核对类型与 `RevertsID`。每个节点的条目写在它那一批的时刻；删除的条目的版本号读这一单元刚删的正文，与同步的相同。
- **一页的正文至多写一次**：单轮里每页至多一步写回，恢复不写版本，所以不会两批写同一页的版本行（00 号文档 4.10 原来担心的情形）。批之间别人写了它，第 2 步的再计划以 `changed` 答出；`RecordRevision` 的守卫照旧是最后的防线。
- **步数的界线**：一批至多 100 个节点，所以至多 100 步，远在 `MaxRevertSteps` 之内。单轮里很多最上层时的超线性（P3A 审查 C-I2）只剩每批之内的。
- **被锁**：开始时有锁，请求整体拒绝；开始之后才有的，任务在那一批停下。不等锁放开（编辑可以几十分钟），也不跳过被锁的页（跳过违反第 11 节第 3 条的"不跳过"）。报告说是谁、哪一页。
- **没有要改的**：开始时的计划没有一步（C 碰过的都已回到之前），任务成功，计数为零，不建变更集；报告说没有要改的（A-N5）。
- **停下的状态**（冲突、被锁、截掉、取消、失败、服务重启）：
  - 已做的批留着，它们的变更集 R（类型 `revert`、`reverts_id = C`）是普通的变更集。每个删除的最上层整棵在回收站里或整棵还在（2.1 第 3 条）。计划的次序是放回、删除、恢复、写回，停在哪一类之前，之后的都没做。
  - **没停在一组中间**：
    - 回到之前：撤销 R（同步或在后台）。R 的"前"就是 C 的"后"，C 的节点回到 C 留下的样子。
    - 再来一次：之后 C 可以再整组撤销。
    - R 的撤销本身要恢复 R 删掉的各个最上层（各是一组），至多 `MaxJobItems` 组（没有 `MaxJobGroups`，3.11）。
  - **停在一组中间**（这一组的最上层已恢复，剩下的成员各是已恢复的父节点下回收站的一项）：回收站的几项合不回一项，这一组回不到任务之前。
    - **往前**：不要撤销 R，逐项恢复剩下的：父节点活着，各自回到原处，即 C 的"前"，这一组就是 C 的撤销要的样子。计划里这一组之后的步没做：之后的组各是回收站的一项，可以逐项恢复，名称被占时加序号而不是冲突；写回经各页的版本还原。
    - **已撤销了 R 时**：R 的撤销把已恢复的部分以 t_r 删成一项，剩下那几项的父节点都进了回收站。这时逐项恢复会放到笔记本的根下（`parent_in_trash`）。要先恢复 R 的撤销删成的那一项（回到原处），再逐项恢复剩下的。
    - **往回**：撤销 R 之后，这一组留在回收站里分成几项，C 再撤销时它答 `changed`。
  - 写进报告的说明（4.3）与 M11 的移交（第 8 节）。
- **请求与任务开始之间**：
  - C 被别人撤销了（同步撤销或它的撤销）、改了，或页被锁了：任务开始时的计划停下，什么都没写。
  - 别人撤销了任务正在写的 R：下一批 `Run` 答 `ErrChangesetReverted`，任务 `changeset_reverted`（P3 3.7，同一条路）。
- **任务正在写时在请求里撤销它的 R**（P4A 审查 A-M2，偏离 28）：
  - 撤销任务的 R 带着它删除的最上层时（例如撤销一次导入），请求未加锁地读 R、解析它要恢复的页；这期间任务的下一批把更多删除并入 R；锁下 R 多出没有解析过的组，答 409 `page.revert_conflict`，把这些页列为 `changed`（P3 3.6：没有解析过的恢复的页算 `changed`）。
  - 什么都不写、任务照常；"R 在请求读它之后变了"字面上是对的。批接连落下时这样的请求几乎每次被拒，R 过了同步的界线之后答 422；走得通的是先取消任务或等它结束，再撤销 R。
  - 服务端与契约不改（主持者的决定），网页在冲突之前说明（4.2）。只恢复的 R（恢复的任务、只恢复的撤销任务）的撤销只删除、不解析，照上一条：撤销成功，任务的下一批以 `changeset_reverted` 停下。

### 3.6 transfer：开始、运行、报告与失败

**开始**（`StartTrashRestore`、`StartRevert`，照 `StartImport` 的 `Check` 与 `Create`，`start_import.go:66-98`、`:227-290`）：

- **`startTrashRestore`**（请求体 `{node_id}`）：
  1. `Tree.CheckRestore`：笔记本不在或看不到 404 `notebook.not_found` → 读者 403 `forbidden`（`node.restore`）→ 节点不在这本笔记本或已被清理 404 `page.not_found` → 409 `page.not_in_trash`、`page.not_trash_root`（带 `root_id`）、`page.restore_expired`
  2. **忙**：在 `Uploads.checking` 之下占住笔记本（`claim`，不带字节、不 `admit`，所以不算进队列），再读 `Writing`。已被占住（正在上传的导入或另一个开始）或有排队、运行中的写入任务，答 409 `transfer.busy`。
  3. 队列满 503 `server_busy`。
  4. 事务：
     1. 工作区、笔记本 `FOR SHARE`。
     2. 再判定 `node.restore`（动作由组合根从 page 交来，`Deps.RestoreAction`）。
     3. 任务创建的 advisory 锁。
     4. 队列（503）。
     5. 写入任务（409，唯一索引兜底）。
     6. 写行：`queued`，`root_id` 是这一项，`name` 是它的名称，`changeset_id` 新取，客户端按凭据；投递。
  5. 202 `TransferJob`。

  提交或失败之后放开占住的笔记本（`release`，`defer`）。这样一次导入的 `Check` 在这期间答 409，不会传完整个文件之后才在 `Create` 撞上唯一索引（A-M11、B-N6）。
- **`startRevert`**（请求体 `{changeset_id, message?}`）：
  - `Tree.CheckRevert` 按 3.5 的次序；第 7 步的 `Admit` 就是上面的"忙"（占住、读 `Writing`）。
  - 然后 503，然后同上的事务（`changeset.revert`）。写行：`target_changeset_id`、`message`，`name` 是 C 的类型，`changeset_id` 新取。202。
  - 占住从第 7 步到提交：计划不超过约 5 秒，期间别的开始与导入答 409，正是对的。
- 小的一项、小的变更集同样可以在后台做（一批做完），不另答码。

**运行**（`Background.Run(ctx, id)`：River 的两个种类各一个 worker 类型，都在 `transfer_import` 队列上，`jobs.import_workers` 个 worker，同导入的超时）：`StartJob` → `Actor{UserID: 发起人, JobID}` → 心跳 → `Tree.PlanRestore`/`PlanRevert`（进度的总数是 `plan.Total()`）→ 逐批：

1. **停下的检查**：心跳要求取消、行已删。在这里停，即"在两批之间停下"。
2. **切批**：从下一步起，节点数到 `Deps.Batch.Nodes`、正文字节到 `Deps.Batch.Bytes` 为止，第一步总在批里（单独超过的一步自成一批）。
   - `Deps.Batch` 零值即 100 个节点、8 MiB、20,000 条链接；交错以 3 个节点一批跑（B-M11），经 `wireApp` 的批（偏离 17）；`newApp` 交零值，同 serve。生产用的批由经 serve 的 `TestABackgroundRestoreThroughServe`（103 个条目、两批）钉住，`transferDeps` 的守卫只核对它不设批（P4A 审查 B-M1：原来的守卫核对的不是生产用的批，不会失败）。
   - `RunBatch`：503 退避重试（1 秒起、加倍、至多 30 秒，同导入）；`ErrBatchTooLarge` 把 `to` 减到一半（至少一步）再试。page 的 `ErrBatchTooLarge` 经组合根的 `treeError` 换成 transfer 的，减半才发生（P4A 审查 B-I1：这条映射原来没有测试；`TestABackgroundBatchPastTheLinksIsHalved`：第一批 100 个节点带 20,790 条链接，减半之后 102 个恢复落在两个单元里）。
   - 单元用 River 的上下文，已开始的单元做完。
3. **成功**：进度加这一批的步的 `Nodes` 之和，至多到总数（B-N4：不按单元答的计数，一个数错不会让每次心跳违反 `progress_done <= progress_total`）；计数前进，报告交给心跳（`publish`）；每写 10,000 个节点与结束时 `ANALYZE` page、linking、asset 的表（同导入）。
4. **失败**：照下表停下，报告写明已完成的部分（计数）与原因。

| page 答的 | 任务的失败 |
|---|---|
| `ErrTrashChanged`（恢复的任务） | `trash_changed`（新） |
| `ErrRestoreExpired`（恢复的任务；撤销里过期是冲突的 `expired`） | `restore_expired`（新；不叫 `expired`，免得与导出的状态 `expired` 同名，A-N1） |
| `ErrNoParent`、`ErrTooDeep`（恢复的任务） | `tree_changed` |
| `ErrRevertConflict`（带 `Conflicts`、`ConflictCount`） | `revert_conflict`（新），报告带冲突 |
| `ErrChangesetTrimmed`（含撤销里的 `ErrTrashChanged`） | `changeset_trimmed`（新） |
| `ErrLocked`（带 `Lock`） | `page_locked`（新），报告带被锁的页与持锁人 |
| `ErrChangesetReverted` | `changeset_reverted` |
| 判定拒绝、看不到笔记本、未认证 | `forbidden` |
| 503 / `ErrBatchTooLarge` | 退避重试 / 批减半 |
| 别的 | `internal` |

组合根的 `treeError` 加这些映射：page 的哨兵 → transfer 的哨兵，同 `ErrChangesetReverted`（`bootstrap/transfer.go:149-159`）。冲突与持锁人从 `*shared.Error` 的 `Conflicts`、`ConflictCount`、`Lock` 读：这是共享内核的类型，transfer 不碰 page 的码。组合根以 `fmt.Errorf("%w: %w", …)` 把 transfer 的哨兵与 page 的 `*shared.Error` 一起包（偏离 9）。

- **抽出共用**（`transfer/app/batches.go`）：批的上限（`Deps.Batch`）、`retry`/`busy`、单元的"结果未知"（导入另加删文件）、`unitFailure`、`ANALYZE` 的节拍。
  - 导入照旧边解析边切批：链接按解析的结果，503 时先写已解析的批。只是换用这些，每个单元带任务行的 `changeset_id`（`Create: true`）。
  - 两种新任务的解析在 `RunBatch` 里（page 读正文、`DecidedAll`），transfer 只切节点与字节。
  - 导入的测试原样通过（`import_test.go`），只是变更集的 id 来自任务行。
- **导入的节点上限**（2.2 的 (a)）：导入的计划（条目加路径隐含的文件夹）建的节点超过 `ImportDeps.MaxNodes` 时整包失败 `too_many_nodes`，什么都不写、删掉文件（同条目超过上限）。组合根交 `page.MaxJobItems`（60,000），不是配置；0 即不设，只给测试；生产接线的守卫钉住它。
- **取消与重启**：
  - 取消照旧（`cancel.go`）：排队的立即取消；运行中的在两批之间停下，状态 `cancelled`，报告写明已完成的部分。两批之间的停下检查由经 serve 的 `TestCancellingABackgroundJob`、`TestABackgroundJobStoppedAtEachBatch` 守住；单元测试 `TestABackgroundJobStopsBetweenTwoBatches` 里停下之前要的第三批答忙、等运行结束（CI 的竞态检测下它曾让任务接着跑，偏离 29），它分不出两批之间不查停下的运行。
  - 服务重启：收拾记 `interrupted`，保留心跳写的报告（`jobs.sql:116-117`）。进度至多落后一次心跳、计数至多落后 `reportBeats` 次（同导入，写进契约的描述）；任务行的 `changeset_id` 不受影响。
  - 收拾的"River 已不持有的排队任务"走四种（`maintain.go:108`），`Held` 按种类映射（`river/jobs.go:148-166` 原来把导入之外的都当作导出）。
  - 清扫与到期只认导入、导出的文件；清理删行时只给这两种删文件（`maintain.go:282-299`，B-N5）。
- **报告**（`domain.Report`，存在 `transfer_jobs.report`）：
  - 计数加 `restored`、`deleted`、`put_back`、`written`（撤销的节点数与写回的页数）。撤销任务的 `restored` 是节点数，同步撤销的日志的 `restored` 是组数，契约的描述写明（A-N8）。
  - 恢复的任务用 `pages`、`attachments`、`renamed`、`skipped`；撤销的任务另用 `skipped`（续段，A-M9）。
  - `placed`：恢复的根放在哪里，第一批之后。
  - `conflicts`（至多 100 项）与 `conflict_count`。
  - `lock`：被锁的页与持锁人的 id，读时经 `Names` 给显示名。
  - 列表的读去掉 `problems` 与 `conflicts`（`jobs.sql:51`），单个任务的读给出。剩下的数目是进度的总数减已处理的，不另记。
- **日志**：开始、结束各一条：`job_id`、`notebook_id`、`user_id`、`client`、种类、状态、原因的码、计数；名称与说明不进日志（13.1 第 10 条）。

### 3.7 撤销的例外按条目的时刻

`NodesNow` 的 `deleted_by_revert`（P3 3.4、3.5）有三处比变更集的 `created_at`。P4 的任务并入自己的变更集之后，三处都不对：后台撤销的后一批在 C 与 r 之间删除了这个节点，它的变更集却建在第一批、C 之前；"之间"漏掉它，例外误成立，悄悄改回（P3A 修复核对第三轮 N-2）。改比条目自己的时刻（P3 第 8 节核对者的写法）：

```sql
CASE WHEN n.trash_root THEN EXISTS (
    -- The root's deletion now, by its item: the item of the node a changeset's unit wrote as it deleted it, at the
    -- node's deletion time, of a revert; the restore that revert reverted, by its own item's first write; the
    -- changeset reverted's deletion of the node, by its item; no deletion of the node's between the two, by their items.
    SELECT 1 FROM changeset_items i
    JOIN changesets c ON c.id = i.changeset_id AND c.kind = 'revert'
    JOIN changeset_items ri ON ri.changeset_id = c.reverts_id AND ri.node_id = n.id AND ri.restored
    JOIN changeset_items di ON di.changeset_id = sqlc.arg(reverted)::uuid AND di.node_id = n.id
        AND di.after_name IS NULL AND di.updated_at < ri.created_at
    WHERE i.node_id = n.id AND i.after_name IS NULL AND i.updated_at = n.deleted_at
        AND NOT EXISTS (
            SELECT 1 FROM changeset_items x
            WHERE x.node_id = n.id AND x.after_name IS NULL
                AND x.updated_at > di.updated_at AND x.updated_at < ri.created_at)
) ELSE false END::boolean AS deleted_by_revert
```

- **为什么条目的时刻可靠**：
  - 删除的条目最后一次写就是删除：`RecordItems` 的 `DO UPDATE` 改 `updated_at`；`DeleteNodes` 只改条目的 `deleted_at`（`nodes.sql:129-131`）。
  - 恢复的条目的 `created_at` 是它第一次写、即恢复的时刻：`DO UPDATE` 不改它。
  - 所以一个任务的每一批的删除、恢复各带自己的时刻。
  - c 改经条目的 `changeset_items_node_id_idx` 找（不再经日志的索引），r、C 的条目经 `(changeset_id, node_id)` 键。仍依赖时钟不往回走（同 P3 3.5）。
- **`TestTheRevertsReads` 的 V 翻转**（`history_test.go:496`、`:525`，A-M1）：
  - V 的撤销变更集建在它自己的删除条目之后一小时。P3 以"变更集建在节点被删的时刻"认出删它的单元，所以 V 为假。新规则以条目认（这个条目就是节点现在的删除），V 为真。
  - 这是对的：变更集由它的第一个单元建，建在自己的条目之后不会出现；建在之前、在之后的批里删除，正是 P4 的任务（R 建在第一批）。
  - V 改为这个真实的形状：`made` 在 `back` 与 `at` 之间，期望为真，注释照改。"撤销的条目不是节点现在的删除"仍由 U（条目不是删除）与 Y（删除早于现在的这一次）守住，二者仍为假。
  - 别的各例结果不变：S、T、E 真，W、B、O 假（审查 A 逐条核过）；`TestTheUndoOfAnUndoGoesNewestFirst` 每个变更集一个单元，不受影响。
- **计划与计时**：`history_plans_test.go` 里 `NodesNow` 走日志索引的断言改为走条目的 `node_id` 索引；数组语句的计时照 P3 3.11，在 1 万个例外成立的根上重量。

### 3.8 观察者与守卫

不改。每一批是普通的单元：

- 恢复带 `Restored`：asset 按组的时刻恢复这一批的附件的行并核对行数，linking 照新建建索引并重新解析受影响的键。
- 删除照删除；放回照移动、改名（`UpdateLinks` 为空）；写回照正文的写。
- `pages` 事件每批一个（`tree: true`）。
- 守卫：恢复对编辑锁放行（`OpRestore`）；撤销的删除与写回经编辑锁（3.5）。
- `MarkTrashRoots` 不是树的变化（节点仍在回收站），不记条目、不发事件。
- 守卫与观察者看到的 `Write.Kind`、`RevertsID` 照旧（M10、M11 靠它）。

### 3.9 契约

`api/modules/transfer.yaml`：

- **`startTrashRestore`**：
  - `POST /api/v0/notebooks/{notebook_id}/trash-restores`，请求体 `TrashRestoreStart {node_id}`（`additionalProperties: false`），答 202 `TransferJob`。
  - `x-problem-codes: [notebook.not_found, forbidden, page.not_found, page.not_in_trash, page.not_trash_root, page.restore_expired, transfer.busy, server_busy]`。
  - 描述写：恢复的分批、批之间设上根的节点、停下的原因、任务的变更集可以撤销（在 `MaxJobItems` 之内）、码的次序；`node_id` 可以是一个附件的项（B-N13）。
- **`startRevert`**：
  - `POST /api/v0/notebooks/{notebook_id}/reverts`，请求体 `RevertStart {changeset_id, message?}`，答 202。
  - `x-problem-codes: [validation_failed, notebook.not_found, forbidden, page.changeset_not_found, page.changeset_trimmed, page.revert_too_large, page.revert_conflict, page.locked, transfer.busy, server_busy]`。
  - 描述写：`revertChangeset` 的核对在请求里先做一遍；只剩条目的 422（`MaxJobItems`）；忙在条目之后；锁在冲突之后、整体拒绝；分批与停下。
- **`TransferKind`** 加 `trash_restore`、`revert`。
- **`TransferJob`**、**`TransferJobDetail`**（照旧两处同步改）：
  - 加必有的 `changeset_id`、`target_changeset_id`（`uuid|null`）。`changeset_id` 的描述：建行时定下；任务第一次写之后才有这个变更集；导出与 P4 之前的导入为空。
  - `root_id`、`name` 的描述写每种类型的含义：恢复是这一项（页或附件）与它的名称；撤销的 `root_id` 为空、`name` 是被撤销的变更集的类型。
  - `TransferJobDetail` 另加 `conflicts`（`RevertConflict[]`，`maxItems 100`）与 `conflict_count`：`revert_conflict` 的；别的为空数组与 0。
- **`TransferReport`** 加 `placed: TrashPlacement|null`（恢复的任务）、`lock: PageLock|null`（`page_locked` 的），两者必有、可为空；`TransferJobDetail` 的 `conflicts`、`conflict_count` 总有（偏离 16）。
- **`TransferCounts`** 加必有的 `restored`、`deleted`、`put_back`、`written`，描述写各种类型用哪几个、`restored` 数的是节点；`renamed`、`skipped` 的描述按类型写（:653-666 现在只说导入与导出）。
- **`TransferProgress`**（:573-577）的描述加两种：已处理的节点，含跳过的；中断时可能落后一次心跳。
- **`TransferFailure`** 加 `trash_changed`、`restore_expired`、`revert_conflict`、`changeset_trimmed`、`page_locked`，各写一句；`forbidden`、`tree_changed`、`changeset_reverted` 的描述按类型写（:600-615，B-M7）。另加 `too_many_nodes`：导入的计划建的节点（条目加路径隐含的文件夹）超过一次撤销能撤销的 60,000（2.2 的 (a)）。
- **被新类型说错了的描述**（B-M8）：
  - `startImport` 的 `transfer.busy`（:82-87）："这本笔记本有排队、运行中的写入任务（导入、恢复、撤销）或正在上传的导入"。
  - `listTransferJobs`（:137-140）："导入、导出、恢复与撤销"。
  - `getTransferJob`（:165-167）：问题与冲突。
  - `TransferJob`（:382-388）：四种。
- **`api/common.yaml`**：
  - `TrashPlacement` 从 `page.yaml` 移来：两个模块用它，照 P3 移 `RevertConflict`。`page.yaml` 的 `TrashItem`、`NodeRestored` 改为 `$ref`。
  - `PageLock {page_id, user_id, display_name}`（新），`Problem.lock` 改为 `$ref` 它：一种形状一个名字（00-M8 第 5 节，B-N10），同 `Problem.conflicts` 引用 `RevertConflict`；平台的契约测试随之。`Problem.locks` 的项同样引用它（偏离 15）。
- **服务端的文字**：`ErrImportBusy` 改名 `ErrWriting`，详情 "A write into this notebook (an import, a restore or a revert) is under way: wait for it to end."（`transfer/domain/errors.go:28-31`）。
- **网页的门禁**（A 第 4 步）：
  - 门禁是 `transfer-report.tsx:13` 的 `Record<TransferFailure, PlainKey>`；`transfer-report.test.ts` 是手写的表，随之加行（B-N1）。
  - 两种类型的标题与五个失败的文字；三个旧失败按类型的文字（B-M7）。
  - 忙与列表的文字（4.3）：`problem.transfer.busy`（zh-CN.ts:817、en.ts:845）、`transfer.importBusy`（zh-CN.ts:660、en.ts:677，导入对话框 :217 用它，TR2 在 `tr2-import.spec.ts:289-291` 逐字断言英文，随之改）、`transfer.jobsBody`（zh-CN.ts:573、en.ts:588）、`notebookSettings.transfer`（zh-CN.ts:474、en.ts:486）。
  - `make gen` 让新字段成为必有的，假接口的 `jobJSON`（`test/jobs-server.ts:22-45`）与别的 `TransferJob` 字面量同一步补上（B-N2）。
  - `problem-messages.test.ts` 不新增码（两个操作的码都已有文字）。
  - `transfer.importCut`（上传被进行中的写入任务挡住）说导入、恢复或撤销（P4A 审查 B-N1）。

### 3.10 权限、日志、加锁与交错

- **权限**：
  - 两个开始照同步的动作（`node.restore`、`changeset.revert`，写者）：page 在请求里判定，transfer 在写行的事务里再判定（动作由组合根交来）；任务运行时每批的单元照常判定。列表、单个、取消照旧是 `transfer.read`、`transfer.cancel`。
  - **权限矩阵**：`startTrashRestore`、`startRevert` 各一行，`editorsOnly(202, notebookNotFound)`，`write: true`，种子用已有的 `matrixTrash()`、`matrixRevert()`，`check` 核对任务行的类型、`queued`、`root_id` 或 `target_changeset_id`、`changeset_id` 不空。`seededJob.kind()`、`checkTransfers` 的种类随之改（`transfer_jobs_test.go:48-53`、`interleavings_transfer_test.go:74`）。
- **加锁**：
  - 开始照导入（工作区、笔记本 `FOR SHARE` → 判定 → advisory 锁 → 写行），从忙的核对到提交占住进程内的笔记本（3.6）。
  - 每一批是树的单元：工作区 `FOR SHARE` → 笔记本 `FOR NO KEY UPDATE` → 判定 → 并入的变更集 `FOR NO KEY UPDATE` → 节点与跟随它的行；批之间放开。
  - 一本笔记本同时一个写入任务（唯一索引）。
  - 清理 `SKIP LOCKED`，任务持着自己的变更集时它的条目留到下一次（P3 3.3）；期限之内的组由一小时的间隔与清理分开。
  - 13.1 第 5 条不加新的支：任务的单元照页面一支。
- **交错**（`bootstrap/interleavings_background_test.go`；任务停下的 6–8、11、12、15、17–20 在 `interleavings_background_stops_test.go`）：
  - 照 `interleavings_import_test.go` 的 `betweenUnits`（重试直到请求排进两批之间）与 `holdNotebook` 写，`Deps.Batch` 以 3 个节点一批，小的组就跨好几批（B-M11）。
  - 每个结束时核对 `checkPages`、`checkLinks`、`checkAssets`、`checkNotebooks` 与"没有停在排队或运行中的任务"。
  - 列表：
    1. 恢复的任务与在已恢复的父节点下新建同名的页：后面的批把组里的那一页恢复为"名称 2"，报告 `renamed`。
    2. 恢复的任务与删除已恢复的部分：下一批停下 `tree_changed`；被删的那部分之下设上的根各是一项，统计准确。测试让下一批正好有成员在被删的部分之下（先序时下一批就在它下面，A-N6）。
    3. 恢复的任务与把已恢复的节点移深：`tree_changed`。
    4. 恢复的任务与同步恢复一个设上根的节点：任务跳过它们（`skipped`），成功。
    5. 恢复的任务与清理：
       - 两批之间清理以 `now − R` 跑，剩下的（期限之内）一点不碰，下一批照常。
       - 一组在任务开始之前以 `trashedAgo` 挪到期限与清理的截止之间：第一批 `restore_expired`，什么都没写，同一次清理也不碰它（一小时的间隔）。
       - 在两批之间越过期限由 `page/app` 的假时钟测：`trashedAgo` 会改掉组的时刻，让任务以为是别人恢复了，不能用。
    6. 两种任务与同步撤销它们自己的变更集：下一批 `changeset_reverted`，什么都没写。撤销的任务这一例让第 2、3 批删除附件：撤销正在写的、删除页的 R 时，请求的未加锁的读与锁之间任务又删了页，答 409 `page.revert_conflict`，把这些页列为 `changed`，不是成功（3.5 的"任务正在写时在请求里撤销它的 R"，P4A 审查 A-M2）；这时先取消任务或等它结束。
    7. 撤销的任务与两批之间对一页的写：下一批 `revert_conflict`（`changed`），之前的批留着，报告列出它。之后撤销 R、再整组撤销 C 成功（B-M12）。
    8. 撤销的任务与编辑锁（另一个账户编辑）：
       - 开始之前就在编辑：`startRevert` 答 409 `page.locked`，不开任务。
       - 开始之后才编辑：任务到那一批停下 `page_locked`，报告带编辑的人；会话结束之后撤销 R、再整组撤销 C 成功。
    9. 撤销的任务与在要删的最上层下新建页：`children_added`。
    10. 撤销一次大的删除（组跨批）与在已恢复的父节点下新建同名：续段 `title_taken`（不加序号），带 `holder_id`。
    11. 取消：两种任务在两批之间停下、`cancelled`。之后撤销任务的变更集：撤销的任务回到任务之前，再整组撤销 C 成功；恢复的任务，回收站里多出一项，设上根的仍各是一项。
    12. 写入任务互斥：导入（含正在上传的）、恢复、撤销互相 409 `transfer.busy`，导出不受影响。
    13. 后台撤销的后一批在 C 与 r 之间删除了这个节点，再撤销 C：409 `changed`（3.7；按变更集的时刻会悄悄改回）。
    14. 恢复的任务与同步撤销那次原来的删除 C（A-M10）：
        - 计划之后、第一批之前：C 的撤销恢复了整组，第一批 `trash_changed`。
        - 第一批之后：C 的撤销答 409 `changed`，任务照常。
    15. 两种任务与删除笔记本：批之间删除，任务行随笔记本删掉，任务在下一次心跳停下（`errGone`）；设上根的项随笔记本留在回收站里，`checkPages` 的"随笔记本删除的不是根"与它们共存。
    16. 撤销一次大的删除，批之间读者从回收站恢复一个设上根的项：任务跳过它（`skipped`），成功（A-M9）。
    17. 撤销的任务与组的期限、清理：续段过期 → `revert_conflict`（`expired`）；只跑版本的清理器 → `changeset_trimmed`（同 P3 的交错 10）。
    18. 一次导入的上传与两个开始：开始占住笔记本时导入的 `Check` 答 409、不开始上传；导入正在上传时两个开始答 409（A-M11）。
    19. 每个批的边界上停下（B-M12）：两种任务各 n 批，对每个 k = 1..n 在第 k 批之后取消，`checkPages` 成立。没停在一组中间的撤销任务，撤销 R 回到任务之前；恢复的任务逐项恢复剩下的就完成。
    20. 停在一组中间的撤销任务（B-M12）：
        - 逐项恢复剩下的，这一组就是 C 的撤销要的样子。
        - 先撤销了 R 时，先恢复 R 的撤销删成的那一项再逐项恢复，结果相同。
        - 撤销 R 之后再撤销 C，这一组答 409 `changed`。
    21. 撤销的任务的一组在计划之后被恢复、又原样删除一次（P4A 审查 A-M1，修复时加，`TestARevertJobOfAGroupDeletedAgainSinceItsPlan`）：开始它的那一批按组现在的删除时刻读，任务成功。
  - **实施时的写法**（偏离 27）：要第 1 批先提交的写放在第 2、3 批之间（1、2、3、6、7、10、16：放在第 1、2 批之间时，这个写在排到第 1 批之后之前就过不了自己的核对）；5 在计划时就以 `restore_expired` 停下（过期的一项 `PlanRestore` 就拒绝）；13 的种子把 N 恢复在根下（原处恢复照例外的规则不是冲突）；16 由编辑者恢复 C6（读者不能恢复）；19 在第 k − 1 批的边缘之外留一个第 k 批的节点；8 的第二例在计划之后、第 1 批等着时开编辑会话。排进两批之间靠重试，在自己的笔记本里重来（`betweenUnits`）。
  - 经 serve 另有：后台恢复的根放在哪里（`TestABackgroundRestorePlacesItsRoot`）、一批过了链接的界线经组合根减半（`TestABackgroundBatchPastTheLinksIsHalved`）、生产用的批（`TestABackgroundRestoreThroughServe`）（P4A 审查 C-M1、B-I1、B-M1）。
  - P3 的交错 5 改读任务行的 `changeset_id`（`interleavings_history_test.go:265`）。
- **日志**：3.6。page 的每批不另记日志（P2、P3 的同步用例照旧各记一条）。

### 3.11 大小与实测

实施之后照实测改写。数是 P4A 审查的修复一轮的（共用的 Apple M5 Max，负载写在各处）；第一轮的数标明。

- **一批的界线**照导入：100 个节点、8 MiB、20,000 条链接，另加历史至多 `MaxUnitHistory`。一批持笔记本行约是导入的一批（导入的取值本就为了不挡保存，M7 4.11）。撤销的一批比恢复多三样：读整本活着的树、只含这一批的再计划、要恢复的组第一次出现时带着整组读。
- **满负载**（P4A 审查 C-I1 之后定下，负责人可以改判）：2 KiB 正文、每页 20 条解析的链接（指向这次导入的别的页）、十分之一的节点是附件。实施者第一轮取每页 10 条（偏离 22：P2 的满负载每页 84 KiB、200 条链接，9 万页是 7.5 GB，过了导入的 4 GiB）；审查 C 指出这对正文成立、对链接不成立（每页 200 条链接只要 2–3 KiB 正文，没有导入的上限限住链接），而负载正决定 2.2 走哪一支。
- **`MaxJobItems` = 60,000**：请求与任务开始时各读一遍 C 的全部条目、节点、组、正文的哈希与树，并跑 `PlanRevert`；最大的一步（深的导入的一个最上层的删除；撤销它的撤销，一组的第一批）随 C 的条目增长。规则同 `MaxUnitNodes`：最大的一步每次运行都不超过 5 秒的最大的万位一档（P3A 的先例：每次运行，不是中位数）。5 秒而不是请求的 15 秒：持笔记本行时，同期的保存要在请求的期限里做完自己的工作（A-M5）。
  - `BenchmarkBackgroundSteps/top`（深的导入：一个最上层下 5 层；ms，三次，无统计 / ANALYZE 之后，负载 5–14）：

    | 节点 | 最上层的删除 | 撤销的撤销：一组的第一批 | 撤销的撤销的请求 |
    |---|---|---|---|
    | 60,000 | 3,611、3,544、3,981 / 4,642、4,220、4,571 | 1,364、1,281、1,555 / 1,874、2,203、1,295 | 642、628、737 / 1,231、908、620 |
    | 70,000 | 6,907、3,973、4,147 / 4,564、4,267、4,369 | 3,210、1,561、1,840 / 1,670、1,490、1,632 | 1,636、688、772 / 645、645、662 |
    | 80,000 | 4,361、4,278、4,325 / 5,946、6,108、5,999 | 2,137、1,968、2,328 / 2,065、2,695、2,205 | 896、730、867 / 1,007、884、1,004 |

  - 6 万处三次都在 5 秒之内（至多 4,642 ms；修复核对另三次 3.5–3.8 秒）。7 万处有一次 6,907 ms：那次每一步都慢，机器忙，但照"每次运行"的规则 7 万不取。8 万处 ANALYZE 之后 5.9–6.1 秒，9 万处 6,465、6,508 ms。6 万在 5 万以上、小于 2.2 的 N：照 (a)，导入的节点上限 60,000（3.6）。
  - **每条链接**（6 万个节点，ANALYZE 之后，ms）：每页 10 条，删除 3,227、3,317、3,299，一组的第一批 927、1,088、1,106；20 条 4,642、4,220、4,571 与 1,874、2,203、1,295；40 条 5,864、6,244、5,930 与 2,798、2,860、3,773。约 1.6 µs 一条（10 到 20 条之间 2.4 µs，20 到 40 条之间 1.3 µs）。每页多于 20 条时这一步随链接增长、过 5 秒，接受：用户 `DeleteNode` 同一个最上层是同一个单元，一样长。界线的注释与基准写明它成立的链接密度与每条的代价。
  - **让删除的链接便宜**（审查 C 的 (d)，先试）：9 万个节点、每页 20 条、ANALYZE 之后逐条计时，`DeleteNodes` 2,211 ms、`LinksReached` 1,378、`RecordItems` 933、`DeleteLinksOf` 879，整个删除 6,508；不把已删的页当来源读的候选 6,489，在噪声之内（一组的第一批 2,559 对 2,475）。`DeleteLinksOf` 之后 `LinksReached` 经 `page_links_pkey` 的索引扫描 1.27 秒、答 0 行（命中 106 万个缓冲、读 57 万个）：规划器估 156 万行，导入的链接指向自己的页，已删的页自己的链接正对上它的键与目标。换次序、换参数，它都要走过这个单元刚删的每一条链接的索引项：没有便宜的改法，不留。
  - **请求**（`BenchmarkStartRevert`，6 万处三次，ms，无统计 / ANALYZE 之后）：平铺 324–358 / 273–310，深 417–663 / 353–592，带文件夹 338–386 / 292–308，很多组（根下 6 万个附件的导入的撤销的撤销）744–836 / 736–807；一组的撤销的撤销（上表）至多 1,231。请求至多约 1.2 秒；两种撤销的撤销平均差不多长。第一轮的平铺 ANALYZE 之后 9 万处 0.5 秒、20 万处 1.0 秒、55 万处 3.3 秒。
  - **没有 `MaxJobGroups`**：很多组的形状（平铺的导入与附件的导入的撤销的撤销，一两个节点的组）请求不比一组的长，恢复一批 70–411 ms（第一轮，9 万处）。3.5 的"撤销 R 再整组撤销 C"至多 `MaxJobItems` 组。
- **计划的内存**（第一轮，深的导入，ANALYZE 之后，回收之后的堆）：撤销的计划 5 万、9 万、55 万处 271 ms 不到 1 MiB、488 ms 3 MiB、3,662 ms 18 MiB；恢复的计划 101 ms 1 MiB、198 ms 2 MiB、1,045 ms 15 MiB；撤销的撤销（一组含每个节点）的计划 5 万、10 万、20 万处 7、14、29 MiB。约每个条目 150 字节（原来估约 1 KB）；8 个 worker 也远在 1 GiB 之内，不是取更低一档的理由。
- **恢复的大步**：
  - 一页 100 万行历史：删除 6,235 ms，后台恢复（一步）8,925 ms，1.43 倍（2.1 第 1 条的 1.2–1.9 倍之内）。两者都过 5 秒、分不开：M12 的性能移交第 14 项。
  - 一页下 5,000 个附件：第一批把留下的 4,901 个设为各自的项，50 ms；51 批共 710 ms。审查 C 另量了很宽的一层：一页下 4.5 万页与 5 千个附件的后台恢复 501 批，第一批 676 ms、设约 4.5 万个根，每批平均 158–199 ms。每批设根的数受扇出而不受批的节点数限。
- **整个任务**（`BenchmarkBackgroundJobs`，第一轮：`MaxJobItems` = 90,000、每页 10 条链接；没有在新的负载下重量，批照旧切在 100 个节点，基准的说明保留这张表并标明）：
  - ANALYZE 之后每批的 p95 与总时间：导入的撤销，平铺 325 ms（901 批，191 秒）、带文件夹 228 ms（926 批，120 秒）、附件 266 ms（900 批，173 秒）、深的一步 5.2 秒；撤销的撤销（恢复），平铺 411 ms（250 秒）、深 252 ms（171 秒）、带文件夹 357 ms（240 秒）、附件 184 ms（108 秒）。
  - 一项（深的最上层）的后台恢复：计划 276 ms、4 MiB，900 批，p95 310 ms，191 秒；5 万处 84 秒（回收站的读按键之前，计划 56.8 秒、103 秒）。
  - 无统计的批在任务第一次 ANALYZE（1 万个节点）之前慢 3–4 倍，之后同 ANALYZE 之后；笔记本满的时候一批至多慢约 1.75 倍。
- **不加 `Subtrees`**：平铺导入的撤销（5 万个节点，ANALYZE 之后）早晚各十批的语句计时：最上层的子树读（`ChildrenOfAll`，约一个最上层一条）409 对 388 ms，`DeleteNodes` 403 对 397 ms，每个最上层不变。随笔记本增长的是每批再计划读整本活着的树（`ListNodes`，5 万个节点活着时一批 24 ms，近空时 3 ms），设计接受（M12 的移交第 15、16 项）。
- **数组语句的两种计划**（`BenchmarkBackgroundSteps/arrays`，在真的撤销留下的回收站上；ms，按参数 / 通用，1 万 / 5 万个 id）：`TrashNodesAt` 26/30、135/188；`TrashGroups` 25/35、154/210；`ChangesetItemsOf` 13/8、56/45；`ChangesetVersionsOf` 7/9、62/56；`NodesNow` 42/44、270/252；`MarkTrashRoots` 3/9、14/54。没有 P2A 那样灾难性的通用计划；六条新的留在 `planned`，`NodesNow` 照 P3 缓存（3.3）。
- **实测找到的两个缺陷**：`RecordItems` 与回收站的读按时刻连接，大的删除之前的统计下走 `deleted_at` 的部分索引（3.3）。
- **守住**（`//go:build !race`，在 `test-go-guards` 里各单独一行、带 `-v`；时限照本机的三倍，同 `8c83ccb7` 给 P2、P3 的守卫定的规则）：

  | 守卫 | 本机 | 时限 |
  |---|---|---|
  | `TestStartRevertAtMaxJobItems`：平铺的导入的撤销的请求；多一个节点答 422，在读条目之前、不调用 `Admit` | 298–319 ms | 1 秒 |
  | 同上，"many groups"：根下 `MaxJobItems` 个附件的导入的撤销的撤销（那次撤销经 SQL 照任务写它的样子写成；300 个附件时与真的任务逐行比过，905 行 0 处不同） | 680–697 ms | 2.1 秒 |
  | `TestARevertJobOfAnImportAtMaxJobItems`，"in folders"：1 万个节点带文件夹的导入的撤销任务（计划加各批；9 万处本机 93 秒，过了 60 秒，照这一节原来的规则改用 1 万） | 5.6–6.6 秒 | 17 秒 |
  | 同上，"its largest step"：`MaxJobItems` 处深的最上层的删除（抓到 `RecordItems` 的回归，327 秒） | 3.58–4.52 秒（基准最慢 4.6 秒） | 14 秒 |
  | 同上：撤销的撤销（一组含每个节点）的第一批，6 万个节点的再计划与它的边缘的设根（修复核对 K-4） | 748–992 ms（基准 1.3–2.2 秒） | 6.6 秒 |

  - `TestMaxJobItemsIsTheMeasuredSize`（`domain/jobs_test.go`）钉住 60,000；`TestTransferDepsLeaveTheBatchesTheImports` 核对组合根不设批、交出两个动作、导入的节点上限是 `page.MaxJobItems`；`TestPageDepsLeaveTheUnitsBoundsTheDomains` 另核对 `MaxJobItems` 为 0（domain 的值）。
- **CI 的分份**：
  - 成本那一份在 P4A 之前作业已到 15:00（测试 13:37），过了这一节的 15 分钟：组合根的预算与守卫分出为 `guards`（`make test-go-guards`，偏离 24）。
  - 组合根竞态检测下那一份三次测试一步 12:00–12:06（`main` 上 7:12），到了 15 分钟的八成（审查 C-M6）：按测试名分成两份，`test-go-bootstrap-1` 跑 `-run '^Test[A-D]'`、`test-go-bootstrap-2` 以同一个表达式 `-skip`（Makefile 的 `BOOTSTRAP_FIRST`），两份之和总是整个包，新的测试落在其中一份；本机两份的测试时间之和 199 与 210 秒。
  - CI 的服务端成了五份：`race`（组合根之外的包）、`bootstrap-1`、`bootstrap-2`、`costs`、`guards`；`make test-go` 仍一次跑全部。run 38618552830 的测试一步：竞态 6:00、组合根 4:58 与 5:05、成本 8:01、守卫 9:37。
- 数字与结论写回 00 号文档 4.9；M12 的性能移交第 14、16 项。

### 3.12 测试

- **domain**：
  - `GroupPreOrder`：父节点在前；同一父节点下按次序值与 id；与 `TrashGroup` 的同父节点次序相同。
  - `GroupCounts`、`Frontier`：一层很宽、跨三层、批的边界正好在一个父节点的孩子中间、组内的附件不算一层、先序时每批的边界至多"层数 × 扇出"。
  - `MaxJobItems` 的值由 `TestMaxJobItemsIsTheMeasuredSize` 钉住（60,000），`pageDeps` 不设（`TestPageDepsLeaveTheUnitsBoundsTheDomains`）。
- **page/app**（假的端口与时钟）：
  - `CheckRestore`、`CheckRevert` 的每个码与次序：含 `MaxJobItems` 的边界、`Admit` 在条目之后且在读之前（`Admit` 失败时什么都没读）、冲突、读不到的版本、锁（别人的会话挡住删除、自己的不挡；任何会话挡住写回；按计划的次序第一个）、两次读之间不在的变更集答 `spec.NotFound`。
  - `RevertChangeset` 改用 `revertCheck` 之后码与次序照旧：P3 的表原样通过，含"读条目之前答 422"（A-M2）。
  - `RunBatch` 的恢复：第一批的根的核对与放置、续批跳过别人恢复的、父节点不活着、太深、名称加序号（含之前的批恢复的同父兄弟）、期限、设根的统计、五个数按批、`ErrBatchTooLarge`（历史在解析之前、链接在解析之后、单独一步不判）。
  - `RunBatch` 的撤销：
    - 再计划只含这一批的节点。
    - 续段不进 `PlanRevert`：续段与一个新的最上层同批时不答假的 `changed`（A-M3）。
    - 续段的五种冲突与跳过（活着且在"前"的位置上）。
    - 删除的 `children_added`、被锁原样、已分类的错误包成未分类、`ErrTrashChanged` 换码。
    - `Kind`、`RevertsID`、`Message` 只在建变更集时写。
  - `UnitSpec.Create`：第一次写以给定的 id 建，之后并入；不写不建；找到的照旧核对。
  - 没有一步的计划：成功、不建变更集。
- **适配器**（真的 PostgreSQL）：
  - 六条新查询：`TrashNodesAt` 只读仍以各自时刻删除的、`TrashChildrenAt` 很多组一层一条、`TrashShapeOfAll` 的次序、`MarkTrashRoots` 只认这一时刻且不是根的、`ChangesetItemsOf` 与 `ChangesetVersionsOf` 只读给出的节点。
  - `TrashGroups` 与逐组的 `TrashGroup` 读出相同的组。
  - `NodesNow` 的例外：`TestTheRevertsReads` 的 V 改为真实的形状、期望为真，别的各例照旧（3.7）；另加"一个变更集的两批、后一批在 C 与 r 之间删除"为假、"恢复的任务后面的批恢复了它"为真。
  - 成本与计划测试（3.11）。
- **transfer/app**（假的 `Tree`）：
  - 两个开始的码与次序：`uploading`、占住、`Writing`、队列、事务里再判定、失败时放开占住的笔记本、建行时取 `changeset_id`。
  - `Background.Run`：切批（注入的 `Batch`）、503 退避、减半、进度按步的 `Nodes`、`ANALYZE` 的节拍、每种失败的映射与报告、取消在两批之间、心跳写报告、删除的任务停下、`internal` 时报告照写。
  - 导入改用共用的分批与任务行的变更集之后，它的测试原样通过。
  - 清理只给导入、导出删文件。
- **重启**（`transfer_jobs_test.go`，B-M12）：运行中的恢复、撤销在 `AtStart` 记 `interrupted`，保留心跳写的报告；River 已不持有的排队的两种任务，经按种类的 `Held` 记失败。
- **整个程序**（`bootstrap/background_test.go`，13.1 第 21 条：组合根交空时失败）：经接口 422 → 开始 → 任务成功：
  - 恢复一项：102 页与 1 个附件、带链接。之后附件下载 200、`checkLinks`、`links` 事件；一个 `trash_restore` 变更集带 103 个恢复的条目，条目的时刻分两批。
  - 撤销一次导入：根下 1,501 个，回收站列出 1,501 项。
  - 撤销一次大的删除（组跨批），回到原位置；撤销那次撤销。
  - 任务行的 `changeset_id`（建行时就有，等于任务写的变更集）、`target_changeset_id`、报告。
  - 交错（3.10）；`checkPages` 在每批之后成立（测试在批之间读）。
- **反向对照**：
  - 每批的核对各去掉一项：期限、根与统计、跳过、父节点、层级、名称、设根、续段的五种与跳过、续段不进再计划、再计划只读这一批的节点、被锁停下、请求里的锁、`Admit` 的位置、`Create`、减半、`Writing` 覆盖三种、开始时占住笔记本、`Held` 按种类、例外按条目的时刻、先序（换成按层，设根数的断言要失败）。测试要失败。
  - 守卫在界线处的变异照 P3A。
- **审查与修复核对加的**：放置的六种与经 serve 的一例、正好 10 层与附件（恢复与续段）、跳过的成员的孩子不设根、根自己的历史过了界线仍单独恢复（P4A 审查 C-M1–C-M4）；组在计划之后又被删除（单元的两种批与经 serve 的交错 21，A-M1；它的历史按新的时刻数，修复核对 K-1）；续段只列真正的冲突，含孙子（A-M3，K-2）；减半与生产用的批经 serve（B-I1、B-M1）；列表里撤销的报告没有冲突（B-N3）。

### 3.13 审查之后的改动

三位审查者（A：任务的语义与并发；B：契约、transfer 与模块的边界；C：测试、实测与设计的符合）与修复核对的处置见 `reviews/P4A-background-server-review.md`。与上文最初的设计不同之处已改在各节里：

- **实施时定下的**（实施者的 30 条偏离，三位审查者判过）：sqlc 列上 00039、迁移测试的反例（偏离 1、2，3.2）；两个开始用 `claim`，不加 `Uploads.uploading`（偏离 3，3.1、3.6）；`GroupPreOrder`（偏离 4，3.4）；`TrashChildrenAt` 按次序值与 id 排、store 按组分开（偏离 5，3.3）；一批按父节点读兄弟、父节点与深度（偏离 6，3.4）；撤销的再计划按步的种类读（偏离 7，3.5）；设根取组内的子树、不碰跳过的成员的孩子（偏离 8，3.4）；组合根以 `%w: %w` 包两个错误（偏离 9，3.6）；`revertBounds.History`（偏离 10，3.5）；一层的计划测试取一部分父节点（偏离 12，3.3）；计划的统计与根比、第一段取再计划的时刻（偏离 13，3.4、3.5）；假端口的 `NodesNow` 按条目的时刻（偏离 14）；`Problem.locks` 引用 `PageLock`，报告的 `placed`、`lock` 必有且可为空（偏离 15、16，3.9）；`wireApp` 的批（偏离 17，3.6）；`transfer-names.test.ts`（偏离 18）；设计之外修的三处缺陷：续段被清理拿走的版本、`RecordItems` 与回收站的读按键（偏离 20，3.3、3.5）；三个守卫与 CI 的第四份（偏离 23、24，3.11，修复时又改过）；数组语句在组合根的基准里量（偏离 25，3.11）；不加 `Subtrees`（偏离 26，3.11）；交错的写法（偏离 27，3.10）；TR1–TR4 的报告断言（偏离 30，3.1）。偏离 21、22（`MaxJobItems` = 90,000、每页 10 条链接）由审查 C-I1 取代；偏离 28 即审查 A-M2；偏离 11、19、29 是测试的写法与流程。
- **审查与修复核对带来的**：满负载定为每页 20 条链接，`MaxJobItems` = 60,000，导入的节点上限随之，先试过让删除的链接便宜（C-I1，3.11、3.6）；撤销的一批按组现在的删除时刻读、计划只在单元成功之后取单元的时刻（A-M1、A-N2，3.5、交错 21）；在请求里撤销正在写的撤销任务的 R 答 409，写明（A-M2，3.5、3.10 第 6 条）；续段只列真正的冲突（A-M3，3.5）；减半经组合根的映射、生产用的批经 serve 钉住（B-I1、B-M1、C-N1，3.6）；导入的节点上限写进 `config.yaml` 与 README（B-M2）；上传被挡的文字（B-N1，3.9）；`NodesNow` 照旧缓存（A-N1、C-N2，3.3）；很多组的请求与撤销的撤销的第一批各有守卫（C-M5、修复核对 K-4，3.11）；组合根按测试名分成两份（C-M6，3.11）；测试的补足（C-M1–C-M4、B-N3，修复核对 K-1、K-2，3.12）。

## 4. B：前端

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/services/transfer.service.ts` | `startTrashRestore(notebookId, nodeId)`、`startRevert(notebookId, changesetId, message?)` 与类型 |
| `web/apps/web/src/stores/transfer.store.ts` | 两个开始（经导出的 `starts` 同一时刻一个，答复的任务放在列表最前一次、在途的读丢掉，同 `start`）；`importing` 改为 `writing`（三种写入任务排队或运行中）；`press()` 记按下时的时刻与持有的任务；`startedJob(kind, target, me, press)`：结果未知时重读列表找这次的任务（4.2） |
| `web/apps/web/src/pages/notebook/trash-row.tsx`、`trash-page.tsx`、`trash-preview.tsx` | 422 的说明带"在后台恢复「X」"；`useBackgroundRestore`；状态行；预览的开始带路由状态回到回收站页（`StartedState`） |
| `web/apps/web/src/pages/notebook/revert-dialog.tsx`、`changeset-page.tsx`、`changeset-text.ts`；`services/api.ts` | 422 之后确认框换成"在后台撤销"；状态行；`reasonText` 移到 `changeset-text.ts`、`lost()` 移到 `services/api.ts`，任务的报告与回收站不必引用确认框（偏离 6） |
| `web/apps/web/src/pages/notebook/transfer-job-row.tsx`、`transfer-report.tsx`、`transfer-page.tsx`、`import-dialog.tsx` | 链接、计数、放置、冲突、持锁人、每种停下的说明（两种类型的标题与入口的改名已随 A 的门禁，`transfer-names.ts`、文字）；导入的预检认三种写入任务 |
| `web/apps/web/src/i18n/messages/{zh-CN,en}.ts` | 文字（前缀 `transfer.`、`trash.`、`history.`） |
| `web/apps/web/src/test/jobs-server.ts`、`test/history-server.ts` | 假接口的两个开始（码的次序同服务端）与两种任务的状态，开关 `startsHeld`、`busy`、`queueFull`、`lost`、`listWaits`、`clockBehind`；导入答 `changeset_id`（P4A 审查 B-N2）；历史的 `jobTooLarge`。回收站的太大用已有的历史计数（`item.history`），不另加开关（偏离 2） |
| `trash-background.test.tsx`、`revert-background.test.tsx`、`transfer-background.test.tsx`（新）；store、服务、导入、确认框的测试 | 4.4 |
| `e2e/fixtures/db.ts`、`transfer.ts`、`assert/transfer.ts`、`assert/page.ts`；`e2e/stories/history/hi5-revert-import.spec.ts`、`e2e/stories/trash/tb2-background-restore.spec.ts`、`tb1-trash.spec.ts` | 4.5 |

### 4.2 422 之后：在后台进行

- **回收站页**：
  - 恢复答 422 `page.restore_too_large` 时，列表之上的说明（`Alert`）照旧说太大（`trash.tooLarge`），另带按钮"在后台恢复「X」"；行上的"恢复"不变。名称照行上的"恢复"区分同名的两项（`namesOf`，`trash-row.tsx:161-179`，B-N3；同一分钟的带 id 的末尾）。按钮在说明之下，离这一项的行远（偏离 7）。
  - 按下：`transfers.startTrashRestore(id)`，按钮 `aria-disabled` 且 `aria-busy`，一次按下至多一个请求；按钮离开时焦点给 `h1`（`useFocusLeaving`，同 `RestoreButton`）。
  - 202：说明换成状态行（一开始就在的 `<output>`）"已开始在后台恢复「X」。 查看进度"：
    - "查看进度"链到"导入、导出与恢复"，路由状态 `focusJob`，到达时焦点在那一行（`transfer-page.tsx:24-30` 已有）。
    - 焦点经 `watchReader` 到 `h1`，由状态行描述（13.2 第 26 条）。
  - 被拒：
    - 409 `transfer.busy`："这本笔记本有进行中的导入、恢复或撤销：等它结束再试"。
    - 503 照 `transfer.queueFull`。
    - `page.*` 的 409 照 `refusalText`。
    - 403、404 照现有，另重读笔记本（读者失去按钮）。
    - 500、网络错误、超时：结果未知，照下面的规则，说在状态行里：有，"已开始……查看进度"；没有，"没有开始在后台恢复「X」，可以再试"，留着按钮；列表读不出，"不知道是否已开始……：任务列表读不出来"，带"查看任务"（偏离 4）。
  - 之后重读回收站与单项（同恢复）。任务进行中，回收站随每批的 `pages` 事件重读，设上根的节点作为项出现、恢复之后消失，不另说明。
- **预览**（B-M5）：
  - 422 的说明同样带按钮，按下同上。
  - 202 时带路由状态 `{...arrived, started: {name, job}}` 回到回收站页（同同步恢复的 `RestoredState`，`trash-preview.tsx:125-129`），回收站页的 `<output>` 说"已开始……查看进度"。
  - 不留在预览上：第一批恢复了根之后，预览的单项读答 `page.not_in_trash`，会换掉整个外壳。
  - 按钮在"恢复"旁、标题的一行里（偏离 7）。开始在途时这一项照旧显示，第一批的事件读它已不在也不换（`starting` 算在途）。
  - 被拒与结果未知说在预览的警示里（它没有自己的状态行，偏离 4），被拒照回收站页的七种（P4B 审查 A-M2、C-M1）：忙与队列满留着按钮、焦点不动；别处已恢复与过期换成"不在了"的视图，回收站的标题取得焦点、由拒绝描述；成了别的项的成员时转到根的预览；403 时"恢复"离开、焦点到标题；笔记本没了时"页面不存在"、重读笔记本。每次开始之后重读这一项。
- **撤销确认框**：
  - 撤销答 422 `page.revert_too_large` 时，确认框的内容换成"这次改动太大，不能在这里撤销……它碰过 {n} 个节点。在后台撤销？后台的撤销分批进行，途中停下时已撤销的部分留着。"（第 11 节第 7 条；`{n}` 照同步的 422，上千时是"1,000+"）。按钮"在后台撤销"与"取消"，说明的输入留着；小标题取得焦点（同冲突）。小标题（"……太大……它碰过 {n} 个节点。"）是确认框的描述，问的一段在它之后（两段都作描述进打磨）。
  - 按下：`transfers.startRevert(id, message)`，发送中同确认框（`aria-disabled`、忙的守卫，焦点留在按钮上）。
  - 答复：
    - 202：关闭，单个变更集页的状态行"已开始在后台撤销这次改动。 查看进度"，焦点到 `h1`。
    - 409 `page.revert_conflict`：换成冲突的列表（同步的那一个）。
    - 409 `page.locked`："「页」正在被 X 编辑：等它结束再撤销"（同步的守卫的文字）。
    - 422（`MaxJobItems`）："这次改动太大，后台也撤销不了。"（不带数：问题里没有它，B-M9）。按钮照旧可按，再按得到同样的答复（偏离 5，只留"取消"进打磨）。
    - 409 `transfer.busy`、503：确认框里的说明，可以再试（503 只在后台的模式里说队列满，同步的照旧说服务器忙）。
  - **冲突与进行中的写入任务**（主持者加的，P4A 审查 A-M2）：同步或后台的撤销答 409 `page.revert_conflict` 时，确认框先重读任务列表；这本笔记本有排队或运行中的写入任务（导入、恢复、撤销；导出不算，P4B 审查 A-N2、C-M2），冲突的列表之前另说"这本笔记本有进行中的导入、恢复或撤销：等它结束或取消后再撤销，下面的冲突可能因它而起。"（`history.conflictsWriting`）。局限：列表只给不是管理员的写者他自己的任务，别的账户的任务引起的冲突不带这一句（P4B 审查 A-N1；要服务端的提示，进打磨）。
    - 截掉、403、404：照同步的关闭与说明。
- **结果未知**（`lost()`，移到 `services/api.ts`；两个开始都照它，B-M6）：
  - 按下时 store 的 `press()` 记下浏览器的时刻与它持有的任务的 id。答复丢了（500、网络错误、超时）时重读这本笔记本的任务列表（`TransferStore.load`），找这个账户这种类型、这个目标的任务（撤销看 `target_changeset_id = C`，恢复看 `root_id` 是这一项）：
    - 进行中（排队或运行）的，不论时刻：一本笔记本一次一个写入任务，这次按下要么开始了它，要么因它被拒，说"已开始"对用户都对。
    - 已结束的，只取按下时 store 没有持有、且创建不早于按下之前一分钟（`clockMargin`）的。
  - 不比两个时钟：任务行的 `created_at` 是服务端的时钟，按下的时刻是浏览器的。原来的"创建在按下之后"在浏览器快过几百毫秒时就找不到刚开始的任务，说"没有开始"，再按答忙（P4B 审查 A-M1、B-M4、C-M3）。剩下的：两个时钟差过一分钟时可能取错或漏掉已结束的任务；同一账户两个标签页的查找可以取到另一个的（打磨）。
  - 有：说"已开始"与"查看进度"。没有：说"没有开始，可以再试"。列表读不出：说不知道，带"查看任务"。
  - 不看 C 的 `reverted_by`：任务第一批之前它是空的。
  - 单个变更集页上，一次的查找的结果只在之后没有别的一次说过什么时才说（`shown`，在 `done()` 里设）：晚到的结果不盖掉较新的一次的；之后的一次在确认框里结束并取消的不挡它（P4B 审查 A-N3、修复核对 F-2）。P3 的"结果未知"（`findOut`）用同一个守卫。
- 只在 422 之后问：网页先照旧在请求里撤销、恢复（小的一次做完），不按 `node_count` 预先猜。

### 4.3 任务列表

- **入口与页标题**：设置的入口与页标题改为"导入、导出与恢复"（英文 "Import, export and restore"）；页的说明"你在这本笔记本的导入、导出、恢复与撤销……"（`transfer.jobsBody`）；列表名称"最近的任务"不变。
- **每行的标题**：
  - 恢复："从回收站恢复「X」"。
  - 撤销："撤销一次{类型}"：`name` 是被撤销的变更集的类型，照 `history.kind.*` 写；英文 "Undo: {Kind}"，不带冠词（B-N9）；同名的照 `jobNames` 带编号。
  - 状态、进度（照旧"进度：{done} / {total}"，偏离 1；带单位的写法进打磨）、发起人与时刻、取消照旧。
- **链接**（可访问的名称带这个任务的名称，同 `transfer.downloadOf`，B-N3）：
  - "在历史里查看"去任务的变更集的单个变更集页，导入也有。`changeset_id` 不空、且已处理的不为零或任务开始过之后被中断时才给（A-M7：什么都没写的任务没有变更集；中断时进度可能落后一次心跳。队列在开始之前丢掉、救援记为 `interrupted` 的任务 `started_at` 为空，不给，P4B 审查 B-M2）。
  - 撤销另有"被撤销的改动"（`target_changeset_id`）：报告的冲突只列原因与名称，各节点的链接在 C 的确认框里（B 过度设计 5）；任务什么都没写时，这是回到 C 的唯一一条路。
  - 工作区的 slug 经 `useWorkspace()`。
- **报告**：
  - 恢复的计数：页面、附件、改名、跳过（"别人先恢复了的"）；另一句放在哪里（照回收站的放置文字）。
  - 撤销的计数：恢复、删除、放回的节点与写回的页、跳过的；没有一步时"没有要撤销的：它碰过的都已回到之前"（A-N5），只在成功时说（计划时就停下的撤销也是 0/0，P4B 审查 B-M5）。计数与剩下的数目经 `Intl.NumberFormat`（"1,501"，P4B 审查 B-M3）。
  - 失败的文字各一句，按类型写。`forbidden`、`tree_changed`、`changeset_reverted` 在两种新类型下各有自己的（B-M7）：例如恢复的 `tree_changed` 是"恢复途中，已恢复的部分被删除或移到太深的地方了"。
  - 停下且已处理的不为零时说剩下的数目（总数减已处理的）与可以做什么，按种类与失败只说对它成立的（P4B 审查 B-M1：原来每种停下一样，四种与失败相反）：
    - 恢复的一般停下（取消、中断、`forbidden` 等）："已恢复的留在笔记本里。剩下的 N 个节点在回收站里，分成几项，每项可以单独恢复。"；`tree_changed` 只说没处理到的在回收站里；`restore_expired` 说剩下的期限已过、不能再恢复；`changeset_reverted` 说"已恢复的又回到了回收站，和没处理到的节点一起。"；`trash_changed` 不另说。
    - 撤销的一般停下（取消、中断、`revert_conflict`、`page_locked` 等）："已撤销的部分留着，剩下 N 个节点没有处理。要回到之前，可以在历史里撤销这次撤销，之后再整组撤销这次改动。"，另加一句："停在恢复一组的中间时：回收站里由这次撤销留下的几项，逐项恢复就完成这一组；不要先撤销这次撤销。"（A-M4）；`changeset_trimmed` 只说可以撤销这次撤销（清理拿走了一部分，不能再整组撤销）；`changeset_reverted` 不另说（组的中间停下之后"再整组撤销"不总成立，3.10 第 20 条）。
    - 英文不用复数（"Nodes left: {left}"，00 号文档 4.13）；中文不说"各自"：剩下的是设了根的几项，不是每个节点一项（P4B 审查 B-M3）。
  - `revert_conflict` 列出冲突：原因与名称，至多 100 项与总数，读单个任务时才有；另说"在「被撤销的改动」里重新撤销时列出它们的链接"。
  - `page_locked` 说"「页」正在被 Ada 编辑"，并链到那一页；树里叫不出名字的页被这个账户自己锁着时说"你正在别处编辑撤销要改的一页。"（P4B 审查 B-N1）。
- **轮询与预检**：运行中照旧每秒轮询（`pollInterval`），不加事件。导入的对话框的预检（`transfers.writing`）与 409 说"这本笔记本有进行中的导入、恢复或撤销"（`transfer.importBusy`、`problem.transfer.busy` 一起改，3.9）。

### 4.4 测试（vitest）

- **服务与 store**：两个开始的请求、一次一个、答复的任务放在最前、`writing`（运行中的导出不算）、`press()`、`startedJob` 的匹配（类型、目标、发起人；进行中的不论时刻，已结束的按持有的与一分钟的界线）。
- **回收站页**：
  - 422 的说明带"在后台恢复"，同名的两项区分。
  - 按下之后的状态行、焦点、"查看进度"的去处与到达时的焦点。
  - 被拒的每种：`transfer.busy`、503、409 三种、403、404，以及结果未知的三种答案（有；没有，带这个账户同一项早已结束的任务；列表读不出），服务端的时钟慢 0、300 ms、3 秒（P4B 审查 A-M1、A-M3）。
  - 连按两下只发一个请求；读者没有按钮。
- **预览**：202 回到回收站页，状态行与"查看进度"；被拒同回收站页的七种（B-M5，P4B 审查 A-M2、C-M1）；开始在途时第一批的事件不换掉显示的这一项。
- **撤销确认框**：
  - 422 之后的内容与焦点。
  - "在后台撤销"的每种答复：202、冲突、锁、422、409 忙、503、截掉、403、404，以及 500 的结果未知（重读任务列表，找到、找不到、读不出；晚到的结果不盖掉较新的，之后取消的一次不挡它）；冲突时有写入任务进行的那一句（导出不算）。
  - 说明带过去；只发一次。
- **任务列表**：
  - 两种类型的标题、同名、英文无冠词。
  - 链接：导入的"在历史里查看"、撤销的两个链接、什么都没写时不给、开始过之后中断时给、开始之前被丢掉的不给；可访问的名称。
  - 进度、报告的计数与放置。
  - 每种新失败与三种旧失败按类型的文字与可以做什么，用服务端发得出的状态（`changeset_reverted` 在已处理不为零时）；0/0 停下的撤销没有"没有要撤销的"；冲突的列表（100 项与总数，没有链接；全部列出时不说"前 N 个"，没有冲突时没有那一节）；持锁人（别人、自己、树里叫不出名字的页）；停下的导入逐行核对。
  - 入口的名称；导入的预检与 409 认三种。
- **假接口**（`test/jobs-server.ts`）照服务端的码的次序：
  - 恢复：404 笔记本 → 403 → 404 节点 → 409 三种 → 409 忙 → 503。
  - 撤销：说明 422 → 404 → 403 → 404 变更集 → 409 截掉 → 422 → 409 忙 → 409 冲突 → 409 截掉（版本）→ 409 锁 → 503。
  - 开关：`startsHeld`、`busy`、`queueFull`、`lost`（500 或网络）、`listWaits`、`clockBehind`；历史的 `jobTooLarge`；任务的状态由测试推进，形状同服务端（恢复与撤销从创建起有 `changeset_id`，撤销数节点）。

### 4.5 e2e

- **HI5**（`e2e/stories/history/hi5-revert-import.spec.ts`，显式的时限，同 TR2，B-N12）：
  - **页面版本**：
    1. 经"导入、导出与恢复"导入一个根下有 1,501 篇空笔记的 zip（夹具生成；1,501 个最上层才过单轮的步数界线，撤销一次导入不解析，页数的界线碰不到它）。
    2. 任务成功之后，行上的"在历史里查看"进单个变更集页：类型导入，节点数显示"1,000+"（`node_count` 只数到 1,000，B-M9）。
    3. "撤销这次改动" → 确认框换成"在后台撤销" → 按下 → 状态行，"查看进度" → 任务的行取得焦点、到"完成"。
    4. 报告：删除"1,501"（数字分位）。回收站列出导入的最上层（第一页 50 项与"更多"，`trashListed`）；树里没有它们。
  - **接口版本**（PAT）：`startImport` → `revertChangeset` 答 422 → `startRevert` 答 202 → `endedJob` 成功。
  - 两个版本调用同样的断言（`assert/transfer.ts` 的 `expectBackgroundJob`）：
    - 任务行的类型；`target_changeset_id` 是导入的变更集；`changeset_id` 是 `revert` 变更集，它的 `reverts_id` 是导入的；导入的任务行的 `changeset_id`。
    - 1,501 个回收站的根各有统计（`expectTrashRoot`，每组 {1, 0, 1}）；撤销的条目数 1,501、版本 0（`expectRevert`），`revertsOf` 只有它——1,501 从数据库与接口读，不从页面读。
- **TB2**（`e2e/stories/trash/tb2-background-restore.spec.ts`，显式的时限）：
  - 经接口导入一个 `Big/` 下 101 篇笔记（成环的链接，第一篇嵌入 `photo.png`）的 zip（两个版本都经接口：这里要的是导入，不是它的界面，偏离 8），从树的菜单删除 `Big`（102 页）。页面版本先建 Index，Big 导入在它之后，打开 Index 再从树的菜单删除：Big 自己的页开着时删除，`links` 事件过 20 个目标、不点名页面，网页重读 Big 的反向链接答 404，夹具的安静页面检查让故事不稳定（P4B 修复核对第一轮 F-1，15 次失败 5 次）。
  - **页面版本**：
    1. 回收站里恢复 → 422 的说明 → "在后台恢复「Big」"。
    2. 按下之前，测试以一条专用的数据库连接在一个事务里以 `FOR SHARE` 持着笔记本行（`db.hold`，`holdNotebookRow`；`Database` 现在只有连接池的 `query`，`e2e/fixtures/db.ts:24-33`，B-N8）。任务的第一批在它上面等：行显示"运行中"与进度"进度：0 / 103"（计划不加锁，总数已在）。核对过：开始只以 `FOR SHARE` 共享笔记本，不被挡；一批是树的单元，笔记本 `FOR NO KEY UPDATE`，被挡；计划与进度不锁笔记本；服务端不设 `lock_timeout`，批等着而不是重试。持着的连接归 `Database` 管，`close()` 在连接池之前结束没放开的，带 `'error'` 监听（P4B 审查 C-N3）。
    3. 放开之后到"完成"。报告：页面 102、附件 1、放在原处；树里有 `Big` 与它的孩子，附件下载 200。
  - **接口版本**：`restoreNode` 答 422 → `startTrashRestore` 202 → `endedJob`。
  - 断言：一个 `trash_restore` 变更集，103 个恢复的条目，条目的 `created_at` 有两种（两批，100 + 3：步各一个节点，Big、Note 001–099 一批，Note 100、101 与 `photo.png` 一批，Note 001 对 `photo.png` 的嵌入跨两批解析）；任务行的 `changeset_id` 与报告；`expectSubtreeRestored`（`kind: "trash_restore"`）、`expectAssetsRestored`、`expectIndexedLinks`。
- **分批的断言辅助**（B-M10）：
  - `expectSubtreeRestored`：每个节点的 `updated_at` 等于它自己的恢复条目的 `created_at`，不再等于变更集的 `created_at`（`assert/page.ts:213-259`）。
  - `expectTrashRoot`：一个根的组按"从根往下、以根的时刻删除的子孙"数（同 `checkPages`，`interleavings_page_test.go:74-84`），不再按笔记本与 `deleted_at` 分组（`assert/page.ts:187-199`），另比页面、附件与高度（偏离 8）。单个删除（TB1 四处、TB2 两处）带 `{ alone: true }`，另核对笔记本里在根的时刻删除的正是这一组（P4B 审查 C-N2）；HI5 一批在同一时刻删除很多最上层，不带。
  - `endedJob(..., timeout)`：HI5 60 秒、TB2 30 秒，别处照旧 15 秒；两个页面版本 `test.slow()`（偏离 8）。
  - TB1、HI4 照旧通过。
- **反向对照的 e2e 变异**：开始的按钮去掉、任务行的类型标题、"在历史里查看"的去处、`holdNotebookRow` 不起作用（进度断言要失败）。实施时 6 个，都被抓到（第 9 节）。

### 4.6 实施时定下的与审查带来的

三位审查者（A：开始与它们的答复；B：任务列表、文字与假接口；C：测试、e2e 与设计的符合）与两轮修复核对的处置见 `reviews/P4B-background-web-review.md`。与上文最初的设计不同之处已改在各节里：

- **实施时定下的**（实施者的 9 条偏离，审查者判过）：进度的文字照旧（偏离 1，4.3）；假接口另有的开关（偏离 2，4.1、4.4）；结果未知在哪里说（偏离 4，4.2）；确认框里后台的 422 留着按钮（偏离 5，4.2）；`reasonText`、`lost()` 移动（偏离 6，4.1）；按钮的位置（偏离 7，4.2）；TB2 经接口导入、`expectTrashRoot` 另比页面、附件与高度、`endedJob` 带时限（偏离 8，4.5）；主持者加的两项：冲突之前的那一句（P4A 审查 A-M2）与假接口的导入带 `changeset_id`（P4A 审查 B-N2）（偏离 9）。偏离 3（结果未知按两个时钟比较）由审查改掉。
- **审查与修复核对带来的**：结果未知不比两个时钟（A-M1、B-M4、C-M3，4.2）；报告按失败说（B-M1，4.3）；"在历史里查看"只给开始过的中断任务（B-M2，4.3）；不用复数、数字分位、中文不说"各自"（B-M3，4.3）；晚到的结果只在之后的一次说过什么时才不说（A-N3、修复核对 F-2，4.2）；自己锁着叫不出名字的页（B-N1，4.3）；`expectTrashRoot` 的 `alone`、`Database.hold` 归它管（C-N2、C-N3，4.5）；TB2 的页面版本在 Index 上删除 Big（修复核对 F-1，4.5）；测试的补足（A-M2、A-M3、A-N2、C-M1、C-M2、C-N1、B-M5、B-N2，修复核对 F-3，4.4）。
- **局限**：不是管理员的写者看不到别的账户的任务，那种冲突不带 A-M2 的那一句（A-N1，要服务端的提示）。

## 5. 实施步骤

A（分支 `m8-p4a`）：

1. 迁移与它的测试。transfer 的 domain（类型、列、失败、报告）、`Writing`、`Held` 与收拾、清理的种类。共用的分批从导入抽出，带 `Deps.Batch`；导入改用任务行的 `changeset_id`（导入的测试原样通过）。page 的 `UnitSpec.Create`。
2. page 的 domain（`PreOrder`、`GroupCounts`、`Frontier`、`MaxJobItems`）、五条查询与 `TrashGroups`（`revertReads` 改用它，P3 的测试原样通过）、`MarkTrashRoots`。`NodesNow` 的例外改比条目的时刻：P3 的测试除 V 外原样通过，V 改为真实的形状，加 P4 的两例。适配器、计划与数组语句的测试。
3. `revertCheck` 与 `revertBounds` 抽出（`RevertChangeset` 的测试原样通过）；`CheckRestore`、`CheckRevert`（`Admit`、锁）、`PlanRestore`、`PlanRevert`、`RunBatch`、`Unit.RestorePart`、`Unit.RevertBatch`；用例测试。
4. transfer 的两个开始（占住笔记本）与 `Background.Run`、River 的两个种类；组合根的适配与映射、生产接线的守卫；契约、处理器、`make gen`；矩阵的两行；网页的门禁：标题、失败的文字、忙与列表的文字、TR2 的断言、假接口的必有字段（B-N2）。
5. 整个程序、重启、交错（含每个批的边界）、反向对照；基准实测 `MaxJobItems`（三项）与 `MaxJobGroups`、大步、整个任务、计划的内存（3.11），写回；照 2.2 处理 `MaxJobItems` 与 N 的比较（5 万 ≤ `MaxJobItems` < N 时加导入的节点上限与它的测试，低于 5 万时停下来问）；守卫与 Makefile、CI 的分份。
6. 门禁、审查。

B（分支 `m8-p4b`，A 合并之后；服务函数随第一次用到它的那一步加入，每一步 knip 都绿）：

1. 服务、store、回收站页与预览的"在后台恢复"，结果未知。
2. 撤销确认框的"在后台撤销"，结果未知。
3. 任务列表的两种类型、链接、报告；入口改名；导入的预检。
4. vitest 补齐；e2e 的辅助与 HI5、TB2；反向对照、门禁、审查。

## 6. 测试与验证

- **门禁**：`make lint-go lint-web knip gen-check test-go test-web build`、e2e 全量。
- **反向对照**：
  - A 见 3.12 末条。
  - B 的这些各改坏一处，测试要失败：每种答复、焦点、状态行、"查看进度"的去处、预览回到回收站页、结果未知的匹配、标题与链接、报告的每种失败、`writing` 的预检。
- **审查**：每部分三位 Opus 审查者并行，之后 Opus 核对修复，直到一轮没有行为上的发现。
  - A：分批的语义与停下的状态；数据与并发（清理、并入、锁、例外的时刻、先序）；契约、权限、测试与实测。
  - B：界面与无障碍；状态与轮询；测试与 e2e。

## 7. 完成标准

- 第 2 节的每一项做完，第 3.12、4.4、4.5 节的测试通过，门禁全绿。
- `MaxJobItems`（三项）、大步、整个任务、计划的内存实测并写回（3.11）；2.2 的比较做完：`MaxJobItems` ≥ N，或导入有了等于它的节点上限（5 万 ≤ `MaxJobItems` < N），或负责人答复了（低于 5 万）。
- 第 3 节第 5 条的导入可以在后台撤销（守卫或基准，3.11）。
- 审查记录：设计 `reviews/P4-design-review.md`，实施 `reviews/P4A-background-server-review.md`、`reviews/P4B-background-web-review.md`；00 号文档的进度表与本文档的"结果"一节更新。
- **P4 的文档提交做的改动**（随本文档一起）：`00-M8-design.md` 的 2.1 的 19 条。
- **随 A、B 的合并改的**：
  - `v0.1-design.md`：
    - 3.8：在"超过一个单元的撤销在后台分批进行"之后加"每批在锁下重新核对，途中停下时已撤销的部分留着；任务的变更集可以撤销"。
    - 3.10：在"一次请求恢复的大小有界，超过时答 422"之后加"之后在后台按先序分批恢复，批之间已恢复的父节点下仍在回收站的各自是一项"。
    - 7.2：`transfer_jobs` 一行加类型、`changeset_id`（建行时定下）、`target_changeset_id`、说明与一本笔记本一个写入任务。
    - 8.5：任务表加"从回收站恢复、整组撤销（M8，同导入的队列）"。
    - 13.1：
      - 第 1 条：任务的单元经 `TreeWrites` 的 `RunBatch`；恢复与撤销的分批、设根、每批再计划。
      - 第 2 条：恢复与撤销的任务各一个并入的变更集，id 建行时定下。
      - 第 4 条：两个开始的码的次序。
      - 第 5 条：任务的单元照页面一支，一本笔记本一个写入任务，开始时占住笔记本。
      - 第 21 条：最后一跳，任务经 `RunBatch` 到达 page 的写。
      - 第 23 条：两个 River 种类在导入的队列上。
      - 第 31 条：`MaxJobItems`、`Deps.Batch` 可注入。
    - 13.2 第 1、26 条：B 的后台的开始与焦点、预览回到回收站页、结果未知。
    - 13.4：
      - 第 3 条：e2e 以专用连接持锁（`db.hold`）。
      - 第 4 条：P4 的交错，含每个批的边界。
      - 第 6 条：矩阵按里程碑的两行。
      - 第 8 条：新的守卫与 CI 两份的分份。
      - 第 9 条：新查询的计划测试。
  - `README.md` :173：队列一句加"从回收站恢复与整组撤销在导入的队列里"；任务一句加两种新任务。
  - M12 的移交：
    - `M12-release/handoffs/M4-performance.md` 加第 16 项：3.11 的数；撤销每批读整本树；很宽的一层停下时的碎片；计划的内存；`MaxJobGroups` 若有。
    - `M7-transfer.md` 第 5 项加一句：卡住的排队任务挡住三种写入任务，每人排队的上限也算恢复与撤销。
    - `M5-polish.md` 加 B 的打磨项。
  - M11 的移交：新建 `M11-upgrade/handoffs/M8-P4-background.md`，写大的回滚走后台的撤销与 3.5 的停下的状态（没停在一组中间的：撤销那次撤销再整组撤销；停在一组中间的：逐项恢复剩下的，不要先撤销那次撤销）（B-N8）。
  - 导入加了节点的上限时（2.2 的 (a)）：M7 的导入文档、`config.yaml` 的注释、`README.md` 的导入一句、契约的 `TransferFailure` 与网页的文字随之写它；00 号文档第 3 节第 5 条已写（2.1 第 17 条）。
  - M7 的文档另外不改（同 P3 的理由）；`M8-history-search/handoffs/M7-assets-transfer.md` 第 4 项的落实记在 00 号文档的"移交的落实"表（已写 P4）。

## 8. 交给后面的 Phase

- **P5**：无。恢复与撤销的节点照常进出搜索，靠活着的过滤。
- **M9**：MCP 开后台任务要先放开 `transfer_jobs.client`；`batch` 的大变更集的撤销同走后台，受 `MaxJobItems` 与每批的再计划；`log` 看得到任务的变更集（[M9 的移交](../M9-mcp/handoffs/M8-P4-background.md)）。
- **M11**：
  - 回滚一次升级照从新到旧撤销它的变更集，大的走后台的撤销。
  - 停下时（冲突、被锁、服务重启）：没停在一组中间的，撤销那次撤销回到之前，再整组撤销（至多 `MaxJobItems` 组，没有 `MaxJobGroups`）；停在一组中间的，逐项恢复剩下的，不要先撤销那次撤销（3.5）。
  - 撤销任务正在写时不在请求里撤销它的变更集（答 409，3.5）：先取消任务或等它结束。
  - 写进 [M11 的移交](../M11-upgrade/handoffs/M8-P4-background.md)。
- **M12**：
  - 途中停下的任务"继续"：恢复是一项里剩下的各项一起；撤销是并入同一个 R、跳过 R 已做的节点，也让停在一组中间的撤销能回到之前。
  - 撤销的任务等编辑锁放开而不是停下。
  - 撤销每批读整本活着的树（只读附近的节点，同第 15 项）。
  - 很宽的一层停在中间时回收站里的碎片。
  - 任务没有推送，靠轮询。
  - 3.11 的实测数、链接多的导入的最大一步、很大的最上层的同步删除到期限、守卫与 CI 各份的余量（[性能移交](../M12-release/handoffs/M4-performance.md)第 16 项；一页 100 万行历史的一步，第 14 项）。
  - 卡住的排队任务挡住三种写入任务（[M7 的移交](../M12-release/handoffs/M7-transfer.md)第 5 项）。
  - B 的打磨（[打磨移交](../M12-release/handoffs/M5-polish.md)第 24 条），含 A-M2 的那一句要服务端的提示。

## 9. 结果

### 9.1 A：服务端（2026-10-15，合并 `2f8077ec`）

- 提交：实施：写入任务的行、共用的分批与任务行的变更集 `d22581dc`，组的先序与设根、任务的读、例外按条目的时刻 `715fa30d`，page 里任务的核对、计划与批 `3942faea`，两个开始与运行 `bb591bbf`，经 serve 的整个程序 `4543eb43`，续段被清理拿走的版本答 `changeset_trimmed` `9a0c22f3`，交错 `f7e6a33d`，删除的条目按键读版本号 `13a80ac6`，回收站的读按键连接 `efdfa36f`，`MaxJobItems` 与导入的节点上限 `4242df86`，实测与守卫 `3e392003`，实测写进基准与界线的注释 `68d452c4`，两批之间的取消等心跳停下 `400eee34`，transfer 故事的报告 `660bc780`（从 `main` 的 `1f732c96` 开始，是 `5f206a38..7727b791`，之后变基到 `main` 的 `3cb0c51e`，补丁不变）；审查的修复：撤销的一批按组现在的删除时刻读、续段只列真正的冲突 `4583ce68`，上传被挡的文字与列表的测试 `3a0f8e44`，减半、根的放置与再删除的组经 serve、生产用的批经 `newApp` `6da921c8`，满负载下的 `MaxJobItems` = 60,000 与很多组的请求的守卫 `3474681b`，组合根按测试名分成两份 `5c1a6ec7`；修复核对的测试、守卫与注释 `f0bafbba`。合并 `2f8077ec`。
- 实测（3.11）：满负载定为 2 KiB 正文、每页 20 条解析的链接、十分之一是附件；`MaxJobItems` = 60,000（6 万处最大的一步每次都在 5 秒之内，至多 4.6 秒；7 万处有一次 6.9 秒），请求至多约 1.2 秒，没有 `MaxJobGroups`，不加 `Subtrees`；每页多于 20 条链接时最大的一步随链接增长约 1.6 µs 一条，同用户的删除，接受；让删除的链接便宜试过，没有便宜的改法。6 万小于 2.2 的 N（55 万）：照 (a)，导入另有 60,000 的节点上限（`too_many_nodes`）。一页 100 万行历史的恢复一步 8.9 秒（删除 6.2 秒），移交 M12。实测找到 P2、P3 的两个规划器的陷阱（回收站的读与 `RecordItems` 按时刻连接），改为按键。
- 审查：三位审查者（Opus）。B、C 各一条 Important：`MaxJobItems` = 90,000 只在链接很少的页上成立，负载要在合并之前定（高，C-I1；满负载定为每页 20 条，60,000）；批的减半经组合根的映射没有测试（中高，B-I1）。中低 1 条：撤销任务的一组在计划之后被恢复又删除，开始它的那一批按计划时的时刻读，以假的冲突停下（A-M1）。其余是低（在请求里撤销正在写的撤销任务的变更集答 409，写进文档（A-M2）；续段的冲突列表；生产接线的守卫不会失败；导入的节点上限没写进配置与 README；放置、层级的边界、跳过的成员的孩子、单独一步的四处测试缺口；很多组的请求没有守卫；组合根那一份到了时限的八成）与细节。修复核对一轮，没有行为上的发现：两处测试的缺口与四条细节，补上（`f0bafbba`，另给撤销的撤销的第一批加了守卫）。逐条见[审查记录](reviews/P4A-background-server-review.md)。
- 与设计不同之处已改在第 3 节（3.1–3.6、3.9–3.12），3.13 分实施时定下的与审查带来的列出；2.2 记下结果；第 8 节加移交的链接。
- 反向对照：实施之后不经数据库的单元变异 62 个都被抓到，数据库 12 个（1 个第一次存活，补了测试之后被抓到），交错一级 8 个；审查者 B 10 个、C 21 个（存活的即各条测试的缺口或等价），A 以探针代替；审查的修复之后 17 个（`a-m1-no-retime-serve` 经 serve 存活，它的单元测试抓到）；修复核对 24 个（存活的 K-1、K-2 两个补了测试之后被抓到，另一个同上）。
- 门禁：本机 `7727b791`（变基之前）与 `5c1a6ec7` 上 `make lint-go gen-check test-go`、`make lint-web knip test-web build-web`（3,097 个测试）、`make e2e`（241 个）通过，`f0bafbba` 上改到的单元测试与守卫通过；CI：`56525ed8` 竞态那一份的取消测试不稳定（偏离 29），`176e47b9` 的 e2e 抓到 TR1–TR4 的报告（偏离 30），`7727b791` 全绿；变基之后 `660bc780` 全绿；审查的修复 `5c1a6ec7` 全绿（组合根两份测试一步 4:58、5:05，守卫 9:37）；`f0bafbba`（run 38624976370）全绿。
- 交给后面的：B（`m8-p4b`）以这份契约为准（`too_many_nodes`；在请求里撤销正在写的撤销任务的变更集答 409，网页在冲突之前说明；假接口的导入带 `changeset_id`）；M9：MCP 开的后台任务、`batch` 的大变更集的撤销（[M9 的移交](../M9-mcp/handoffs/M8-P4-background.md)）；M11：大的回滚走后台的撤销与停下的状态（[M11 的移交](../M11-upgrade/handoffs/M8-P4-background.md)）；M12：[性能移交](../M12-release/handoffs/M4-performance.md)第 14 项（一页 100 万行历史的一步）与第 16 项（3.11 的数、链接多的导入、很大的最上层的同步删除、撤销每批读整本树、很宽的一层的碎片、计划的内存、守卫与 CI 各份的余量），[M7 的移交](../M12-release/handoffs/M7-transfer.md)第 5 项（卡住的排队任务挡住三种写入任务）。
- 负责人可以改判的取舍：满负载取每页 20 条链接，多于它时最大的一步过 5 秒（同 `DeleteNode`）；导入的节点上限 60,000（2.2 的 (a)，可以宁可降低第 3 节第 5 条的要求而撤掉它）；在请求里撤销正在写的撤销任务的变更集答 409（A-M2，不改服务端与契约）。

### 9.2 B：前端（2026-10-15，合并 `947812e6`）

- 提交：实施：服务、store、回收站页与预览、确认框与单个变更集页、任务列表、导入的预检、文字与假接口 `31b754b7`，HI5、TB2 `aaab10a2`，反向对照找到的测试的缺口 `7fa44cb4`（从 `m8-p4a` 的 `660bc780` 开始）；审查的修复：结果未知不比两个时钟 `71a955f6`，报告按失败说 `6316e2aa`，e2e 的帮手 `e1901710`；之后变基到 `m8-p4a` 的 `5c1a6ec7`，没有冲突，是 `19952930`、`e131a67f`、`52154abe`、`ffc6a3d1`、`30303cd6`、`5bcd2af9`；修复核对第一轮的修复：晚到的结果只在之后的一次说过什么时才不说、三处测试 `35852717`，TB2 在别的页开着时删除 Big `f7fb9f5b`；合并之前再变基到 `m8-p4a` 的 `f0bafbba`，是 `bff9509c`、`183964dc`、`fd7169e2`、`829a9331`、`548fe1da`、`9b65c5c3`、`4e249547`、`cfa9dc8e`。合并 `947812e6`。
- 审查：三位审查者（Opus），都没有 Important。中低 2 条：结果未知按两个时钟比较，浏览器快过一点就说"没有开始"（A-M1、B-M4、C-M3）；报告的"留下的与要做的"对四种停下说反了（B-M1）。其余是低（"在历史里查看"给了开始之前被丢掉的任务；复数、数字与中文的"各自"；几处测试的缺口）与细节。修复核对两轮：第一轮 TB2（页面）不稳定（审查之前就有，测试的缺陷，中高）、A-N3 的修复太宽（细节）与三处测试的缺口；第二轮没有行为上的发现。逐条见[审查记录](reviews/P4B-background-web-review.md)。
- 与设计不同之处已改在第 4 节（4.1–4.5），4.6 分实施时定下的与审查带来的列出。
- 反向对照：实施之后 vitest 76 个（73 个被抓到；补了测试之后另 3 个与新加的 4 个都被抓到）、e2e 6 个都被抓到；审查者 A 7 个、B 18 个、C 15 个（存活的即各条测试的缺口或等价）；审查的修复之后 31 个都被抓到；修复核对第一轮核对者 8 个（存活 3 个即 F-3），它的修复之后 7 个都被抓到；第二轮 15 个都被抓到。
- 门禁：本机 `7fa44cb4`、`e1901710`、`f7fb9f5b` 上 `make lint-web knip test-web build-web`（3,213、3,237、3,239 个测试）、`make build` 之后整个 e2e 245 个通过；TB2 两个版本连跑 15 次（实施者）与 10 次（核对者）都通过；这一阶段没有服务端与契约的改动。CI：`f7fb9f5b`（run 38622786286）与再变基之后的 `cfa9dc8e`（run 38625107556）全绿。
- 交给后面的：M12：打磨见审查记录，已写进 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)第 24 条（含 A-M2 的那一句要服务端的提示）。
- 负责人可以改判的取舍：结果未知的规则（进行中的不论时刻，已结束的按持有的与一分钟的界线；两个标签页与时钟差过一分钟时可能说错）；不是管理员的写者看不到别的账户的任务时冲突不带那一句（A-N1）；进度的文字照旧（偏离 1）；确认框里后台的 422 之后按钮仍可按（偏离 5）。

## 附：查过的代码

行号是 `main` 的 `1f0e3a98`（服务端与契约同 `e08745e`）。

| 决定 | 依据 |
|---|---|
| 一批照导入的上限、单独超过的自成一批、可注入（2.1 第 1 条、3.6） | `transfer/app/import_write.go:25-31`（`batchNodes`、`batchBytes`、`batchLinks` 是常量）、`:82-120`（单独一页总在批里）、`:160-197`（`retry`、`busy`、`release`）、`:205-261`（`unit`、"结果未知"、`unitFailure`，`:252-261` 把 `KindNotFound` 当作 `forbidden`）；`page/domain/trash.go:47-73`（单元的界线、`UnitTooLarge` 只在多于一页时） |
| 不按历史的行分开一页；历史在每批数（2.1 第 1 条、3.3） | `page/adapter/postgres/queries/nodes.sql:102-132`（`DeleteNodes` 同一时刻删节点与它的正文、版本、条目）、`:134-182`（`RestoreNodes` 按节点恢复这一时刻的全部）；`queries/purge.sql:14-50`（清理按表先条目、版本，后正文、节点；`trimmed` 只在条目、版本的清理里标）；`migrations/sql/00015_page_changeset_items.sql:10`、`00016_page_page_revisions.sql:9`（外键 `ON DELETE CASCADE`）；`queries/trash.sql:76` 起的 `TrashHistory`（数到 `lim`，按参数规划）；`02-P2-trash.md` 3.11 的历史表 |
| 先序（2.1 第 2 条、3.4） | `queries/trash.sql:58-68`（`TrashChildrenOfAll` 整层按 `sort_order, id` 排，一层跨所有父节点）；`adapter/postgres/trash.go:90-110`（按层读）；`app/unit_restore.go:49-102`、`:177-210`（`restoredNames`：同一父节点下先到的留名，只看同父节点的相对次序）；00 号文档 4.10 :195（"之后按层"） |
| `RestoreNodes` 可以只恢复一组的一部分、续段的根参数无害（3.4） | `nodes.sql:134-182`（只更新传入的、仍以 T 删除的 ids；根的父节点与次序按 `root` 改，别的保留）；`app/unit_restore.go:111-140`（`bringBack` 以 `group[0]` 为根、五个数按传入的组）；`adapter/postgres/store.go:302-315` |
| 组里不是根的节点谁也改不了，第一批只核对根与统计（2.1 第 2 条、3.4） | `app/restore_node.go:65-79`、`app/trash.go:98-116`（恢复只认根，`NotTrashRoot`）；`domain/revert.go:318-336`（`rootHolds`）、`:460-498`（`leftDeleted`）；`domain/trash.go:19-31`（期限与间隔）；`queries/trash.sql:15-32`、`:50-68` |
| 设根的写法与统计（3.3、3.4） | `migrations/sql/00035_page_nodes_trash.sql`（`nodes_trash_root_check`、`nodes_trash_counts_check`、补值的根的定义）；`nodes.sql:113-121`（`DeleteNodes` 设根与统计）；`bootstrap/interleavings_page_test.go:43-120`（`checkPages` 的根与统计） |
| 计划只留骨架（3.4、3.11） | `page/domain/node.go:27-39`（`Node` 十一个字段、两个字符串）；`domain/trash.go:100-105`（`TrashMember` 再加三个整数）；`app/delete_node.go:29-46`（删除没有界线）；`configs/config.yaml:110`（`import_workers: 1`）；`platform/config/config.go:196-204`（至多 `MaxTransferWorkers` = 8） |
| 放置、名称与序号沿用（3.4） | `app/unit_restore.go:49-102`（`TrashLines`、`PlacementOf`、`placeAmong`、`restoredNames`）；`app/unit_import.go:40`、`:172`（`children`、`free`）；`domain/trash.go:136-165`（`PlacementOf`）；`queries/nodes.sql:74-80`（`ChildrenOfAll`，按参数规划） |
| 很多组一起读（2.1 第 6 条、3.3） | `app/unit_revert.go:254-280`（`revertReads.groups`：每个根一次 `TrashGroup`）；`adapter/postgres/trash.go:90-110`（每组根一条、每层一条）、`store.go:165`（`subtreeLevels`）；`migrations/sql/00012_page_nodes.sql:36`（`nodes_notebook_id_parent_id_idx`）；00 号文档 4.9 :187（"每组读两遍组、一遍历史"，2,000 组到了期限） |
| 撤销的删除一个最上层一个单元，重开 A-I3、C-I3（2.1 第 3 条、3.5） | `app/unit_delete.go:20-45`（`Subtree` → `DeleteNodes`，单元的时刻）；`nodes.sql:110-113`（`stray` 只看本单元的时刻）；`queries/changesets.sql:28-29`（删除的条目的版本号读单元的时刻删除的正文）；`domain/revert.go:898-914`（`deleteSubtree`）；`reviews/M8-design-review.md:20`（A-I3、C-I3）；`handoffs/M7-assets-transfer.md:15`（第 4 项）；`02-P2-trash.md` 3.11、`03-P3-log-revert.md` 3.11"别的"（`BenchmarkTrashDelete` 5 万个节点 1.6–2.1 秒）；00 号文档 4.9 :189 |
| 每批以只含这一批的节点再计划，续段不进去（2.1 第 4 条、3.5） | `domain/revert.go:290-305`（`topOf` 只沿传入的条目往上）、`:318-336`（`rootHolds` 比整组的统计）、`:356-373`（`PlanRevert` 纯函数）、`:413-443`（`left` 按传入的条目）、`:541-623`（`before`）、`:799-848`（`round` 的四步次序）；`app/unit_revert.go:61-118`（`Revert` 的读、计划、应用）、`:190-215`（`parsedAsPlanned`）、`:219-317`（`revertReads`） |
| 续段跳过读者恢复的（2.1 第 4 条、3.5） | `domain/trash.go:153-165`（`PlacementOf`：父节点活着时回到原处，原来的名称与次序值）；`app/restore_node.go:65-111`（读者恢复设上根的一项） |
| 一页的正文至多写一次（2.1 第 1 条、3.5） | `domain/revert.go:255-285`（`readReverted` 的 `writes` 只取没新建、恢复、删除的页）；`app/unit_import.go`（导入只新建）；`queries/changesets.sql:43-54`（`RecordRevision` 的版本链守卫） |
| `revertCheck` 的界线是参数（2.1 第 5 条、3.5） | `app/revert_changeset.go:79-93`（先找变更集，`spec.NotFound` 是 `page.changeset_not_found`）、`:97-103`（条目的 422 在读条目之前）、`:106-114`（不在了时再判定）、`:122-124`（页数、字节、历史的 422 在计划之前）、`:130-136`（步数在冲突与 `Missing` 之后）、`:178-209`（`withinBounds`） |
| 请求里的编辑锁（2.1 第 5 条、3.5） | `app/edit_lock.go:38-73`（`GuardWrite`：删除挡别的账户的会话，正文挡别的会话）；`app/ports.go:438`（`AliveSessionsOf`）；`queries/sessions.sql:23`；00 号文档第 11 节第 3、7 条 |
| `MaxJobItems` 与第 3 节第 5 条的 N（2.1 第 6、17 条、2.2、3.11） | `transfer/domain/import_plan.go:12-15`（`MaxDepth` = 10）、`:64-111`（文件夹由路径隐含地建，每个文件夹一页）；`configs/config.yaml:142`（`import_max_entries: 50000`）；`platform/config/config.go:260`；`app/import.go:128-133`（条目超过上限整包失败 `too_many_entries`）；`03-P3-log-revert.md` 3.11 的表 |
| 最上层的例外按条目的时刻；V 翻转（2.1 第 8 条、3.7） | `queries/history.sql:148-168`（`deleted_by_revert` 比 `created_at`）；`queries/changesets.sql:1-9`（变更集建在第一个单元的时刻，并入只改 `updated_at`）、`:11-41`（`RecordItems` 的 `DO UPDATE` 改 `updated_at`、不改 `created_at`）；`nodes.sql:129-131`；`adapter/postgres/history_test.go:421-431`（各例的说明）、`:496`（V：`made` 在它的条目之后一小时）、`:525`（期望 S、T、E）；`03-P3-log-revert.md` 第 8 节 P4 的第 8 条 |
| 任务行建行时定下变更集的 id（2.1 第 7 条、3.2、3.3） | `app/unit.go:61-89`（`UnitSpec.Changeset` 必须已在）、`:151-210`（`Run`）、`:282-300`（`lockInto`：找不到是缺陷）、`:306-326`（`ensureChangeset` 自己取 id）；`queries/changesets.sql:1-4`（`CreateChangeset` 带 id）；变更集与条目都不记任务（`migrations/sql` 里没有 `job_id`）；`transfer/app/import.go:114-115`（变更集只在内存）；`app/job_run.go:113-173`（进度每次心跳写，报告隔 `reportBeats` = 10 次） |
| 并入任务的变更集、被撤销时停下（3.5、3.6） | `app/unit.go:282-300`（`lockInto` 核对类型、`RevertsID`、执行者、客户端、`reverted`）；`queries/changesets.sql:56-64`（`LockChangeset`） |
| 守卫与观察者（3.8） | `app/edit_lock.go:44-73`（正文与删除被锁、恢复放行）；`bootstrap/assets.go:78-102`（附件的行按 `Restored.DeletedAt` 恢复并核对行数） |
| transfer 的运行骨架与抽出（3.6） | `transfer/app/import.go:64-88`（`Run`）；`app/job_run.go:60-75`（`authorizeJob`）、`:118-173`（心跳与报告）、`:181-239`（结局、`failureOf`）；`app/cancel.go:49-92`；`app/maintain.go:87-114`、`:184-245`（收拾、清扫的种类）、`:282-299`（清理每个到期的行都删文件）；`adapter/river/jobs.go:35-38`（两个队列）、`:127-166`（`Held` 把导入之外的当作导出，`MaxAttempts: 1`）；`adapter/postgres/queries/jobs.sql:14-16`（`CountActive` 数所有种类）、`:26-31`（`Importing`）、`:47-56`（列表去掉问题）、`:60-80`、`:116-117`（收拾保留报告）；`00029_transfer_transfer_jobs.sql:40`（`progress_done <= progress_total`） |
| 开始的码与次序、占住笔记本、事务里再判定（3.6） | `transfer/app/start_import.go:66-98`（`Check`：判定 → 位置 → `checking` 之下占住 → `admit` 读行与队列）、`:227-290`（`Create`）；`app/uploads.go:14-56`（按笔记本占住，`claim` 不 `admit` 时不算进队列）；`app/start.go:105-123`（`lock`、`authorize`）、`:179-195`（`room`）；`api/modules/transfer.yaml:49-129`（`startImport` 的码与描述） |
| 一本笔记本一个写入任务（3.2） | `migrations/sql/00031_transfer_one_import.sql:5-6`；`00029_transfer_transfer_jobs.sql:15`、`:50-52`（类型与只许导出的约束）、`:12-14`（`root_id` 不建外键的理由）；`00032_transfer_running_report.sql:5-8`；`adapter/postgres/store.go:40-59`（唯一索引的映射） |
| 报告与契约（3.6、3.9） | `transfer/domain/report.go:9-48`（失败）、`:109-121`（计数与报告）；`domain/errors.go:26-31`（`ErrImportBusy` 的详情）；`adapter/postgres/store.go:343-394`（报告的编码）；`api/modules/transfer.yaml:82-87`（`transfer.busy` 只说导入）、`:137-140`、`:165-167`、`:381-468`（`TransferJob`）、`:469-562`（`TransferJobDetail` 不用 `allOf`）、`:563-631`（类型、进度与失败）、`:632-667`（计数）；`api/common.yaml:66-81`（`Problem.lock` 内联）、`:124-139`（`Problem.conflicts` 引用 `RevertConflict`）；`api/modules/page.yaml:1425-1438`（`TrashPlacement`）；`shared/error.go:117-132`（`Lock`、`Conflicts`、`ConflictCount`）；`app/revert_changeset.go:166-170`（同步撤销的日志 `restored` 是组数） |
| 组合根的映射（3.6） | `bootstrap/transfer.go:83-108`（`transferTree`）、`:149-159`（`treeError`）；`bootstrap/deps.go:213-241`（`transferDeps`、队列的 worker）；`page/tree_writes.go:38-62`、`:123-144`；`page/module.go:84-93`、`:96-135`、`:187-195` |
| 权限（3.10） | `access/domain/rules.go:107-110`（`node.restore`、`changeset.revert` 写者）、`:128-135`（transfer 的四个动作）；`bootstrap/permission_matrix_transfer_test.go:116-134`（`startImport` 一行）；`permission_matrix_page_test.go:278`、`:347`（`restoreNode`、`revertChangeset` 的种子）；`permission_matrix_test.go:163`、`:258`（写的格在副本上跑）；`bootstrap/transfer_jobs_test.go:48-53`、`interleavings_transfer_test.go:74`（写死的种类） |
| 交错的写法（3.10） | `bootstrap/interleavings_import_test.go:289`（`betweenUnits`）、`:295-334`（两批之间删除、移深）；`interleavings_trash_test.go:28-45`（`holdNotebook`、`waitingFor`）；`interleavings_history_test.go:237-305`（`threeBatches`、交错 5 以 SQL 读 C）；`trash_test.go:417`（`trashedAgo`）；`interleavings_page_test.go:43-120`（`checkPages`） |
| 实测的规则与 CI（3.11） | `02-P2-trash.md` 3.11（5 秒之内的最大一档）；`03-P3-log-revert.md` 3.11（基准的方法、热缓存、很多最上层）；P3A 审查 C-I1、C 存疑（CI 三份的时间：组合根 9:56–11:07 / 15 分钟、成本 13:22 / 20 分钟）；`M12-release/handoffs/M4-performance.md` 第 15 项（组合根那一份过 13 分钟再分） |
| 网页的 422 与开始（4.2） | `pages/notebook/trash-row.tsx:114-129`（`refusalText`）、`:161-179`（`namesOf`）、`:243-275`（`useTrashRestore`）、`:298`（`useFocusLeaving`）；`trash-page.tsx:83-139`（状态行、说明）；`trash-preview.tsx:100-110`（外壳只有拒绝的说明）、`:125-129`（同步恢复带路由状态回到回收站页）、`:154-156`（`not_in_trash` 换掉外壳）；`revert-dialog.tsx:53-55`（`lost`）、`:166-173`（422 的说明）、`:190-233`（答复的分支）；`changeset-page.tsx:98-126`（结果未知读 `reverted_by`）、`:182-227`（状态行与链接）、`:233-263`（R 的页链到被撤销的 C）；`pages/page/page-menu.tsx:55-72`（开始任务之后带 `focusJob` 去任务列表）；`history.sql:67`、`page.yaml:1542-1547`、`changeset-text.ts:83-86`（`node_count` 只数到 1,000） |
| 任务列表与文字（4.3、3.9） | `pages/notebook/settings-layout.tsx:11-15`、`zh-CN.ts:474`、`en.ts:486`（入口的名称）；`zh-CN.ts:573`、`en.ts:588`（`transfer.jobsBody`）；`zh-CN.ts:601`、`:609-610`（三个旧失败的文字只说导入、导出）；`zh-CN.ts:660`、`en.ts:677`（`transfer.importBusy`）、`import-dialog.tsx:217`、`e2e/stories/transfer/tr2-import.spec.ts:289-291`（逐字断言英文）；`zh-CN.ts:817`、`en.ts:845`（`problem.transfer.busy`）；`en.ts:253-257`（`history.kind.*` 的英文）；`transfer-page.tsx:24-52`（`focusJob`、轮询）；`transfer-job-row.tsx:97-174`、`:133`（`transfer.downloadOf`）；`transfer-names.ts:7-12`（只认导入，别的当作导出）；`transfer-report.tsx:13`（`Record<TransferFailure, PlainKey>`）、`:28-43`（只为导入换文字）、`:79-97`（按类型的计数）；`stores/transfer.store.ts:79-81`（`importing`）、`:182-217`（开始）；`test/jobs-server.ts:22-45`（`jobJSON`）；`events/handlers.ts:272-276`（没有任务的事件） |
| e2e（4.5） | `e2e/fixtures/transfer.ts:62`、`:83`、`:139`（`startImport`、`holdImport`、`endedJob`）；`fixtures/wiki-transfer.ts:15-59`（`jobRow`、`importWith`、`openReport`）；`fixtures/assert/transfer.ts:108-164`（`expectImported` 按类型找变更集）；`fixtures/assert/page.ts:187-199`（`expectTrashRoot` 按 `deleted_at` 分组）、`:213-259`（`expectSubtreeRestored` 比变更集的 `created_at`）、`:367-385`（`expectRevert`、`revertsOf`）；`fixtures/db.ts:24-33`（只有连接池的 `query`）；`fixtures/trash.ts:73`（`trashedAgo`）；`fixtures/wiki-pages.ts:363-381`（`changesetPath`、`revertDialog`、`revertWith`）；`e2e/stories/transfer/tr2-import.spec.ts:171`、`:195`（经设置的入口导入、显式的时限） |
