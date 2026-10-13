# M8/P2 回收站：架构设计 & 实施规划

## 0. 分两部分

照 P1 的做法分成 A（服务端：迁移、删除的标记与整组的条目、回收站的读、恢复、观察者、回收站里的页的链接、期限、契约、权限）与 B（前端：回收站页、预览、页面树的 `removed`、删除确认框、左栏的入口；e2e TB1），各自一个分支、一次审查与修复核对、一次合并。B 以 A 合并之后的契约为准，并在 P1B 合并之后开始：预览复用 P1B 的只读阅读视图（`getPageVersionView`，版本号取回收站单项答复里的 `revision`）。设计审查的处置见 `reviews/P2-design-review.md`。

## 1. 基线

- A 从 `main` 的 `e178c64` 开始（P1A 已合并，`12fc5d7`；这份设计在 `e178c64` 提交）；B 从 P1B 合并之后的 `main` 开始。
- 删除今天：`DeleteNode` → `Unit.Delete` 读活着的子树（`Subtree`，按层），每个节点一项没有"后"的变更；`DeleteNodes` 一条语句以单元的时刻 T 删除节点（`updated_by_id` 记删除者）与它们的正文、所有变更集里的版本行与条目；条目一个节点一条 `RecordItem`；asset 的观察者以 `e.At`（= T）删除附件的行；linking 的观察者删除这些页的索引行（不是软删除）。没有 `trash_root`，删除没有任何大小的界线。
- P1 留下：`readableAny`（只认页面）、`FindAnyNode`、版本的读与 `getPageVersionView`（回收站里的页读得到，但链接一律未解析：来源不在 `Paths` 的答复里，`bootstrap/page_versions_test.go` 写明"until P2"）；`changesets.kind` 已有 `trash_restore`；`RecordRevision` 的版本链守卫。
- 清理：`jobs.purge_retention` 默认 1440h、下限 1h；清理任务用 `time.Now()`；页面的表逐表各自提交（条目 → 版本 → 正文 → 节点，节点只删叶子；条目与版本每批 1,000 行、不排序），附件一个事务里先删文件再删行；`nodes` 的父节点外键 `ON DELETE RESTRICT`。

## 2. 目标与范围

做（M8 设计 4.1、4.2、4.5、4.6、4.9、4.11、4.13、4.14 的回收站部分）：

- 迁移：`nodes` 的 `trash_root` 与这一组的统计（页数、附件数、页的层数）；`changeset_items` 的 `restored`、`revision`。
- 删除：根的标记与统计；条目整步一条语句（`INSERT … SELECT unnest`，所有操作共用）；同一单元里先删子孙再删祖先的拒绝。
- 读：回收站的列表与单项（任何类型的根）；附件信息的端口（含已删的）。
- 写：从回收站恢复一个单元（`OpRestore`、`Change.Restored`、类型 `trash_restore`）；超过一个单元答 422（后台进行在 P4）。
- 观察者：asset 恢复附件的行并核对行数；linking 照新建建索引（不改）。
- 回收站里的页的链接从它原来的路径解析。
- 期限：保留期的下限 1 天、恢复的期限、`InstanceInfo.trash_retention_seconds`。
- 一个单元的树操作上限的实测。
- 权限 `node.restore`；契约里共用的 `UserRef`。
- 前端：回收站页与预览、左栏的入口、页面树的 `removed` 的去掉规则、删除确认框的文字；e2e TB1。

不做：后台的恢复（P4：P2 只答 422）；日志、撤销、截掉的标记（P3）；搜索（P5）；永久删除；只恢复一组里的一部分。

### 2.1 对 00 号文档的修订

P2 的文档提交随这份文档一起改 `00-M8-design.md`（理由在本文各节；行号指 `4891f87` 的）：

1. **"父节点不在"去掉**：第 1 节目标 3（:17）、第 3 节第 4 条（:52）、4.6 的放置（:132）、第 9 节 TB1（:349）同改；撤销的 `parent_gone`（:51、:162）不动（那是 S\* 里的父节点）。父节点的外键 `ON DELETE RESTRICT` 与清理只删叶子，回收站里的节点的父节点一定在。放置只剩三种：父节点活着且不太深（原处，原来在根下的回到根下）、父节点在回收站（根下）、放回去太深（根下）。TB1 的"原父节点不在时"改为"原父节点在回收站时"。
2. **附件的信息**：4.1 的表（:68）、`Deps` 一句（:77）、第 7 节 P2 一行（:278）、第 8 节 asset 一条（:326）、"总体设计的修订"的 13.1 第 11 条（:311）：`asset.NewMeta(pool)` 改为 `asset.NewAttachmentInfos(pool)`，`Deps` 的 `AssetMeta` 改为 `Attachments`（`page.AssetMeta` 已是守卫看到的新附件的文件，同名会混）。
3. **`Change.Restored`**：4.1 附件的行（:69）、4.6 单元（:133）、第 8 节 page 一条（:325）：不是布尔，而是 `*Restored{DeletedAt, Asset}`：asset 要删除时刻 T 才能"按同一时刻恢复"，要知道哪些是附件才能核对行数；`Then` 保留第一项的（与"前"同出于第一项）。行数不符答 409 `page.trash_changed`（bootstrap 把 asset 的计数换成 page 的码），不是 500。
4. **回收站里的页的链接**：4.1（:71）、4.3（:102）、第 8 节 page 与 linking 两条（:325、:327）："`Paths` 的来源变体"写成：渲染路径带"来源在回收站里"（`PageRef.Deleted` → `markdown.Page.Deleted` → linking `app.Page.Deleted`），linking 的视图这时经新的 `Pages.SourcePath` 读来源经已删祖先的路径；索引的维护不变。另写明：候选仍只有活着的节点，所以链到同组里的页的链接，预览里可能解析到别处同名的活着的页或不解析，恢复之后才回到组里的页（3.7）。
5. **4.2**：`nodes` 加 `trash_pages`、`trash_assets`、`trash_height`（只在根上，删除时记下）；补值认"随笔记本删除"用 page 自己的数据（同一时刻删除的变更集），不读 `notebooks`。`changeset_items` 的 `restored`、`revision` 在条目合并时保留第一次的；新建条目的补值写 1。"`trash_root` 的维护"加一句：同一单元里先删子孙、再删它的祖先，`DeleteNodes` 拒绝（从上往下删；P4 跨批的同一时刻照原文）。
6. **4.5**（:124）：页数与附件数是删除时记下的准确值，不再"数到 1,000"；4.14 的上限一句（:241）只留日志的计数。单项带 `notebook_id`、页的 `revision` 与附件的类型、大小；`getTrashItem` 的码是恢复的 409 三种；预览只显示根（页的阅读视图，或附件的名称、类型、大小），组里的其余只给数目。
7. **4.6**：放置照第 1 条；"只有根可能冲突"（:132）补"组内同一父节点下键相同的（Unicode 数据升级之后的 `reindex` 才有）同样加序号"。解析（:135）："按 `Take` 取预算"补一句：各次 `Take` 的等待合计至多 `page.parse_max_wait`，用完答 503（每次等 2 秒、100 页会超过 15 秒的请求期限）。核对（:134）另包括"每页的版本行从创建起连成链"（清理先删条目、版本，后删正文）。"移交的落实"表 M6 一行（:292）同补"等待合计有界"。
8. **4.9**（:183）："100 页"是页（附件计入树的操作）；只有一页的组不受页数、字节、链接数三项的限制（它曾在一个请求里写成，同导入的批"独自超过的一页单独成批"）；链接数要解析之后才知道，超过时的 422 可能排在 503 之后；树的操作上限是 `domain.MaxUnitNodes`，P2A 实测（3.11）。
9. **清理的时钟**：第 2 节"平台"（:31）、4.11（:203）、4.14 的交错（:244："注入的时钟"）、第 7 节 P2 一行（:278："清理的时钟"）、第 8 节平台一条（:330）：不加 `PurgeConfig.Clock`。交错测试直接调用清理器、自己给截止时刻，不经 `PurgeJob`（`tm.purge` 今天就是 `before := time.Now()`）；期限的边界（恰在期限的那一刻）在 `page/app` 以假时钟测。
10. **4.13**：从预览恢复之后回到回收站页（与列表的恢复同一个状态行）；左栏的入口是一个单独命名的 `nav`；删除确认框的 N 天向下取整（不把期限说长）；恢复的附件的"打开"去它所在的位置（同 4.12 的搜索结果）。
11. **4.14**：`boot/actions_test.go`（:235）是 `server/internal/bootstrap/actions_test.go`；`getTrashItem` 的码；`restoreNode` 的次序（:238）加上第 8 条的例外与锁下的 `page.trash_changed`。
12. **第 5 节**：`getInstanceInfo`（:264）是 `getInstance`；`{user_id, display_name}` 统一为 `common.yaml` 的 `UserRef`（3.9）。
13. **第 13 节**：加一行变更记录（P2 设计与它的审查）。

### 2.2 需要负责人确认

无。第 11 节的 12 条都不受影响（第 1、2、6、9 条照做）；2.1 的改动都是设计细节或与代码对齐。

## 3. A：服务端

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `server/migrations/sql/00035_page_nodes_trash.sql` | `nodes` 的 `trash_root`、`trash_pages`、`trash_assets`、`trash_height`，约束，部分索引，补值 |
| `server/migrations/sql/00036_page_changeset_items_restored.sql` | `changeset_items` 的 `restored`、`revision`，约束，补值 |
| `page/domain/change.go` | `Change.Restored *Restored`、`Restored{DeletedAt, Asset}`；`Then` 保留第一项的；`OpRestore` |
| `page/domain/tree.go` | `Subtree.Counts()`（页数、附件数） |
| `page/domain/trash.go` | `TrashRoot`、`TrashAncestor`、`TrashMember`、`TrashCursor`（JSON 以 UTC 写时刻）、`TrashFrom`、`Placement`、`PlacementOf`、`TrashMargin`（1 小时）、`RestorableUntil`、`Restorable`、`TrashSince`；单元的界线 `MaxUnitNodes`、`MaxUnitHistory`、`MaxUnitPages`、`MaxUnitBytes`、`MaxUnitLinks` 与 `UnitTooLarge` |
| `page/domain/errors.go` | `ErrNotInTrash`、`NotTrashRoot(rootID)`、`ErrRestoreExpired`、`ErrRestoreTooLarge`、`ErrTrashChanged` |
| `page/domain/actions.go` | `ActionRestore = "node.restore"` |
| `page/app/ports.go` | `Trash` 端口（含 `TrashHistory`）；`NodeWriter.DeleteNodes(ctx, Deletion)`、`RestoreNodes(ctx, Restoration) (RestoredRows, error)`；`ChangesetWriter.RecordItems`（代替 `RecordItem`）；`AttachmentInfos`；`AnyNode.TrashRoot`；`PageRef.Deleted` |
| `page/app/unit.go` | `apply` 一步的条目一次写（`recordItems`，没有要记的就不写） |
| `page/app/unit_delete.go` | 根与统计交给 `DeleteNodes` |
| `page/app/unit_restore.go` | `Unit.Restore` |
| `page/app/readable.go` | `readableNode`（不看类型）；`readableAny` 在它之上要求页面 |
| `page/app/content.go` | `ContentParser.DecidedAll`（从大到小、等待合计有界） |
| `page/app/trash.go`、`list_trash.go`、`get_trash_item.go`、`restore_node.go` | 共用的读与放置；三个用例；`RestoreBounds{Nodes, History}`（0 即 domain 的值） |
| `page/app/get_version_view.go` | 回收站里的页设 `PageRef.Deleted` |
| `page/adapter/postgres/queries/trash.sql`、`trash.go` | `Trash` 端口的查询与实现（另有 `TrashGroupRoot`、`TrashHistory`） |
| `page/adapter/postgres/queries/nodes.sql`、`changesets.sql`、`links.sql` | `DeleteNodes`、`RestoreNodes`、`RecordItems`、`SourcePath` |
| `page/adapter/markdown/markdown.go` | `PageRef.Deleted` → `markdown.Page.Deleted` |
| `page/link_targets.go` | `SourcePath` |
| `page/adapter/http/trash.go`、`handler.go`、`versions.go` | 三个处理器；`UserRef` |
| `page/module.go` | `Deps.Retention`、`Deps.Attachments`、`Deps.MaxUnitNodes` 与 `Deps.MaxUnitHistory`（0 即 `domain.MaxUnitNodes`、`MaxUnitHistory`，只有实测抬高它们）；导出 `ErrTrashChanged`、`TrashMargin`、`Restored`，另导出 `TreeState`（bootstrap 的附件观察者用） |
| `platform/markdown/markdown.go`、`budget.go` | `Page.Deleted`；`Budget.Wait()` |
| `linking/app/ports.go`、`views.go`、`resolver.go`；`linking/adapter/markdown/views.go` | `Page.Deleted`；`Pages.SourcePath`；`resolutions` 接受已读到路径的来源 |
| `asset/lifecycle.go`、`asset/app/follow.go`、`asset/app/ports.go`、`asset/adapter/postgres/queries/lifecycle.sql`、`lifecycle.go`、`store.go` | `NodesRestored`、`RestoreBlobsOfNodes`（按参数规划：store 加 `planned`） |
| `asset/attachment_infos.go`、`asset/adapter/postgres/queries/infos.sql` | `NewAttachmentInfos(pool)`（只凭连接池），答结构体 `asset.AttachmentInfos` |
| `linking/adapter/postgres/store.go` | M6 的 `SetResolutions` 按参数规划（3.11 的实测） |
| `instance/domain/info.go`、`instance/app/get_info.go`、`instance/module.go`、`instance/adapter/http/handler.go` | `TrashRetention` |
| `transfer/adapter/http/server.go` | `UserRef` |
| `shared/error.go`、`platform/httpserver/apierrors.go`、`problem.go` | 扩展成员 `root_id` |
| `platform/config/validate.go`、`server/configs/config.yaml`、`README.md` | 保留期的下限 1 天 |
| `access/domain/rules.go` | `node.restore` |
| `api/common.yaml`、`api/modules/page.yaml`、`instance.yaml`、`transfer.yaml` | 三个操作、`UserRef`、`TrashAttachment`、`root_id`、`trash_retention_seconds` |
| `bootstrap/deps.go`、`assets.go`、`linking.go` | `retention(cfg)`、`Attachments`、观察者的恢复、`SourcePath`、`trash_retention_seconds` |
| `web/apps/web/src/app/problem-messages.ts`、`i18n/messages/{en,zh-CN}.ts`、`test/fakes.ts`、`services/page.service.ts`；`e2e/stories/smoke/s3-instance-info.spec.ts`；`deploy/image-smoke.sh` | 契约带来的门禁：五个新码的文字（`problem-messages.test.ts` 读契约里每个操作的码，P1A 的 `23be652` 为 `page.version_not_found` 做过同样的事）；`instanceJSON` 的新字段；`EditLockHolder` 改为 `UserRef` 的别名；S3 与镜像的冒烟对 `InstanceInfo` 的整体比较（冒烟漏改，CI 的修补 `463d9e9`） |
| `bootstrap/trash_test.go`、`interleavings_trash_test.go`、`trash_cost_test.go`；`page/adapter/postgres/trash_cost_test.go` | 整个程序、交错；基准与守住它们的成本测试（3.11） |

### 3.2 迁移

**`00035_page_nodes_trash.sql`**（先加列、补值，再加约束与索引）：

```sql
ALTER TABLE nodes ADD COLUMN trash_root boolean NOT NULL DEFAULT false,
    ADD COLUMN trash_pages integer, ADD COLUMN trash_assets integer, ADD COLUMN trash_height integer;
-- 一次删除的根：父节点为空或父节点不是同一时刻删除的，且不是随笔记本删除的。笔记本的删除给它的
-- 变更集与节点同一个时刻（DeleteNotebooksPages），子树的删除从不删除变更集：认它不必读 notebooks。
UPDATE nodes n SET trash_root = true
WHERE n.deleted_at IS NOT NULL
    AND NOT EXISTS (SELECT 1 FROM changesets s WHERE s.notebook_id = n.notebook_id AND s.deleted_at = n.deleted_at)
    AND (n.parent_id IS NULL OR n.deleted_at IS DISTINCT FROM (SELECT p.deleted_at FROM nodes p WHERE p.id = n.parent_id));
WITH RECURSIVE g AS (
    SELECT id AS root, id, kind, deleted_at, 1 AS level FROM nodes WHERE trash_root
    UNION ALL
    SELECT g.root, c.id, c.kind, c.deleted_at, g.level + 1
    FROM g JOIN nodes c ON c.parent_id = g.id AND c.deleted_at = g.deleted_at
)
UPDATE nodes n SET trash_pages = s.pages, trash_assets = s.assets, trash_height = s.height
FROM (SELECT root, count(*) FILTER (WHERE kind = 'page') AS pages, count(*) FILTER (WHERE kind = 'asset') AS assets,
        coalesce(max(level) FILTER (WHERE kind = 'page'), 0) AS height
      FROM g GROUP BY root) s
WHERE n.id = s.root;
ALTER TABLE nodes ADD CONSTRAINT nodes_trash_root_check CHECK (NOT trash_root OR deleted_at IS NOT NULL),
    ADD CONSTRAINT nodes_trash_counts_check CHECK (trash_root = (trash_pages IS NOT NULL)
        AND trash_root = (trash_assets IS NOT NULL) AND trash_root = (trash_height IS NOT NULL)
        AND trash_pages >= 0 AND trash_assets >= 0 AND trash_height >= 0 AND trash_pages + trash_assets >= 1);
CREATE INDEX nodes_notebook_id_deleted_at_id_idx ON nodes (notebook_id, deleted_at DESC, id DESC) WHERE trash_root;
```

- 统计只在根上，删除时由单元记下（它手里有 `Subtree`）：回收站的一页不必为每一项读整组（几万个节点）去数数、求层数，恢复也能在读组之前就答 422（调研第 28 项）。一组在回收站里不会变（不能改名、移动、写正文），所以记下的值一直准。
- 补值只读 page 的表（v0.1 设计 7.1：模块只读自己的表；archtest 与 sqlc 都拦不住迁移里的数据语句，所以写明）。有节点的笔记本至少有建它们的变更集，清理在节点之后才删变更集；已删笔记本里正被清理的残余（变更集先没了）可能被标上，无害：回收站只列活着的笔记本。运行时一个节点先被删、之后它的笔记本被删，它仍是根（时刻不同），补值也一样。
- Down 删索引、约束与四列。v0.1 之前没有发布的实例，补值的递归只在开发库上跑一次。

**`00036_page_changeset_items_restored.sql`**：

```sql
ALTER TABLE changeset_items ADD COLUMN restored boolean NOT NULL DEFAULT false, ADD COLUMN revision integer;
-- 页面的新建条目：一页的创建写版本 1（运行时 RecordItems 同样记 1）。
UPDATE changeset_items i SET revision = 1
WHERE i.before_name IS NULL AND i.after_name IS NOT NULL
    AND EXISTS (SELECT 1 FROM page_revisions r
        WHERE r.changeset_id = i.changeset_id AND r.node_id = i.node_id AND r.base_revision IS NULL);
ALTER TABLE changeset_items
    ADD CONSTRAINT changeset_items_restored_check CHECK ((NOT restored AND revision IS NULL)
        OR (before_name IS NULL AND after_name IS NOT NULL)),
    ADD CONSTRAINT changeset_items_revision_check CHECK (revision >= 1);
```

- `revision` 是新建或恢复留下这一页的版本号（附件为空）；P3 撤销一次恢复时靠它找"C 留下的正文"（恢复不写版本行）。新建条目的 `revision` 在 M8 里没有读者，照 00-M8 4.2 的决定保留；补值与运行时同为 1，不取版本行的（同一变更集再写这一页时那一行的版本号会往前走）。
- 迁移测试：`schema_test.go` 核对新的约束与索引的名字、`TestChecksRejectCounterexamples` 加反例（活着的根、根没有统计、非根有统计、恢复的条目有"前"）；补值的测试：迁到最新、`Down` 两步回到 00034、写入删除过的数据（子树、其下更早删除的子孙、随笔记本删除的、页的新建条目），再 `Up`，核对 `trash_root`、统计与条目的 `revision`。

### 3.3 删除：根的标记与整组的条目

- `Unit.Delete` 把根、ids 与 `sub.Counts()`、`sub.Height()` 交给 `NodeWriter.DeleteNodes(ctx, Deletion{Root, IDs, Pages, Assets, Height, By, At})`。`DeleteNodes` 改为 `:one`：`gone` 一步加上根的标记与统计，原来最后的条目一步成为 CTE，最后数出"这一刻已删除、父节点在这次的 ids 里"的节点：

  ```sql
  -- name: DeleteNodes :one
  WITH stray AS (
      SELECT count(*) AS n FROM nodes c
      WHERE c.deleted_at = sqlc.arg(now)::timestamptz AND c.parent_id = ANY(sqlc.arg(ids)::uuid[])
  ), gone AS (
      UPDATE nodes n
      SET deleted_at = sqlc.arg(now)::timestamptz, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now),
          trash_root = (n.id = sqlc.arg(root)),
          trash_pages = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(pages)::integer END,
          trash_assets = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(assets)::integer END,
          trash_height = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(height)::integer END
      WHERE n.id = ANY(sqlc.arg(ids)::uuid[]) AND n.deleted_at IS NULL
      RETURNING n.id
  ), contents AS (…照旧…), revisions AS (…照旧…), items AS (…照旧的条目一步…)
  SELECT n::integer AS stray FROM stray;
  ```

  `stray` 不为零是调用者的缺陷（返回错误，单元回滚）：同一单元里先删了子孙、又删它的祖先，子孙会留着根的标记、祖先的统计少算它（审查 A-M6）。一个单元从上往下删，每棵子树一次；P2 没有这样的单元，P3 的撤销照此（第 8 节）。`stray` 读的是语句开始时的快照（CTE 看不到同一语句的更新），`deleted_at` 的部分索引让它只碰这一刻删除的行。其余照旧（`Subtree` 只读活着的，更早删除的子孙保留它自己的时刻与根的标记）。
- **条目整步一条语句**：`apply` 把一步的变更合并进单元之后，凡 `Moves()` 的一次交给 `ChangesetWriter.RecordItems(ctx, items)`，没有就不写（正文的写）；所有操作共用这一条路（不另留一个节点一条的 `RecordItem`：一条路更少的代码，删除与恢复几千个节点时只有一条语句）。一步里的节点各不相同（删除是子树；移动只有被移的节点的变更会动；其余一个节点），一条 `INSERT … ON CONFLICT` 不会两次碰同一行；`recordItems` 对重复的节点返回错误（调用者的缺陷）。

  ```sql
  -- name: RecordItems :exec
  INSERT INTO changeset_items (id, changeset_id, node_id, before_parent_id, before_name, before_sort_order,
      after_parent_id, after_name, after_sort_order, restored, revision, created_at, updated_at, deleted_at)
  SELECT u.id, sqlc.arg(changeset_id), u.node_id,
      CASE WHEN u.has_before THEN NULLIF(u.before_parent_id, '00000000-0000-0000-0000-000000000000'::uuid) END,
      CASE WHEN u.has_before THEN u.before_name END, CASE WHEN u.has_before THEN u.before_sort_order END,
      CASE WHEN u.has_after THEN NULLIF(u.after_parent_id, '00000000-0000-0000-0000-000000000000'::uuid) END,
      CASE WHEN u.has_after THEN u.after_name END, CASE WHEN u.has_after THEN u.after_sort_order END,
      u.restored, NULLIF(u.revision, 0), sqlc.arg(now), sqlc.arg(now),
      CASE WHEN u.has_after THEN NULL ELSE sqlc.arg(now)::timestamptz END
  FROM (SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(node_ids)::uuid[]) AS node_id,
          unnest(sqlc.arg(has_befores)::boolean[]) AS has_before, unnest(sqlc.arg(before_parent_ids)::uuid[]) AS before_parent_id,
          unnest(sqlc.arg(before_names)::text[]) AS before_name, unnest(sqlc.arg(before_sort_orders)::float8[]) AS before_sort_order,
          unnest(sqlc.arg(has_afters)::boolean[]) AS has_after, unnest(sqlc.arg(after_parent_ids)::uuid[]) AS after_parent_id,
          unnest(sqlc.arg(after_names)::text[]) AS after_name, unnest(sqlc.arg(after_sort_orders)::float8[]) AS after_sort_order,
          unnest(sqlc.arg(restored)::boolean[]) AS restored, unnest(sqlc.arg(revisions)::integer[]) AS revision) AS u
  ON CONFLICT (changeset_id, node_id) DO UPDATE
  SET after_parent_id = excluded.after_parent_id, after_name = excluded.after_name,
      after_sort_order = excluded.after_sort_order, updated_at = excluded.updated_at, deleted_at = excluded.deleted_at;
  ```

  空值照 `SetResolutions` 的写法用零 id 与 `has_*` 标出（根下的父节点也是空，所以"有没有前"要单独一列）。`restored`、`revision` 不在 `DO UPDATE` 里：合并时保留第一次的。适配器从 `Change` 取：`restored = Restored != nil`，`revision = Revision`（只在"前"空、"后"不空时，新建的页是 1）。`RecordItems` 一次只写一个变更集、一个时刻的条目（语句只有一个 `changeset_id` 与 `now`，一步的条目本来如此），别的拒绝（调用者的缺陷）。

### 3.4 读：端口与查询

`Trash` 端口（`page/app/ports.go`），未加锁或在单元的事务里：

```go
type Trash interface {
	// ListTrash is the notebook's trash roots deleted after since, newer first, below the cursor, at most limit; each
	// with its group's counts and height, and a page's content's revision.
	ListTrash(ctx context.Context, notebookID uuid.UUID, since time.Time, below domain.TrashCursor, limit int) ([]domain.TrashRoot, error)
	// FindTrashRoot is the trash root id, as ListTrash gives it; ErrNotFound when id is none.
	FindTrashRoot(ctx context.Context, id uuid.UUID) (domain.TrashRoot, error)
	// TrashRootOf is the root of the trash group the deleted node id is in.
	TrashRootOf(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	// TrashLines are the roots' ancestors, deleted or not, from the notebook's root down to each one's parent, with
	// when each was deleted, by root.
	TrashLines(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.TrashAncestor, error)
	// TrashGroup is the root id of the notebook deleted at at and its descendants deleted at at, level by level,
	// each page with its content's revision and size: zero when its content is gone.
	TrashGroup(ctx context.Context, notebookID, id uuid.UUID, at time.Time) ([]domain.TrashMember, error)
	// TrashContents are the contents of the pages ids deleted at at, by page.
	TrashContents(ctx context.Context, ids []uuid.UUID, at time.Time) (map[uuid.UUID]string, error)
	// TrashHistory is how many versions and items, of every changeset, of the nodes ids were deleted at at, each of
	// the two counted to limit at most: the rows a restore writes besides the nodes' (审查 A-M4).
	TrashHistory(ctx context.Context, ids []uuid.UUID, at time.Time, limit int) (int, error)
}
```

- `ListTrash`（部分索引上的范围；游标从最新开始时传 `domain.TrashFrom` = 9999-12-31T23:59:59Z 与最大的 uuid：pgx 不能经 `time.Time` 送 `infinity`；不写"零表示不限"，同 P1 审查 C-S1）：

  ```sql
  SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
      n.created_at, n.updated_at, n.deleted_at, n.trash_pages, n.trash_assets, n.trash_height, c.revision
  FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at = n.deleted_at
  WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.trash_root AND n.deleted_at > sqlc.arg(since)::timestamptz
      AND (n.deleted_at, n.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
  ORDER BY n.deleted_at DESC, n.id DESC
  LIMIT sqlc.arg(max_rows);
  ```

  `FindTrashRoot` 同样的列，`WHERE n.id = $1 AND n.trash_root`。删除者是根的 `updated_by_id`（删除时写的）。
- `TrashLines`：一页的根一条语句，每一步按父节点的键读一行（同 `LinkTargetsByIDs` 的 `LATERAL … LIMIT 1`，无统计时也不按笔记本扫）：

  ```sql
  WITH RECURSIVE chain AS (
      SELECT n.id AS root_id, p.id, p.parent_id, p.name, p.deleted_at, 1 AS up
      FROM nodes n JOIN nodes p ON p.id = n.parent_id
      WHERE n.id = ANY(sqlc.arg(ids)::uuid[])
      UNION ALL
      SELECT c.root_id, p.id, p.parent_id, p.name, p.deleted_at, c.up + 1
      FROM chain c CROSS JOIN LATERAL (SELECT id, parent_id, name, deleted_at FROM nodes WHERE id = c.parent_id LIMIT 1) p
      WHERE c.up < 64
  )
  SELECT root_id, id, parent_id, name, deleted_at, up::integer AS up FROM chain ORDER BY root_id, up DESC;
  ```

  另答 `parent_id`：一条线最上面的一行父节点不为空，即没有到根（只有缺陷会成环、超过 64 步），适配器答错误。原位置就是这些名称。`domain.PlacementOf(line, height)`（列表、单项与恢复共用）：线为空（原来在根下）→ `original`，`parent_id` 为空；父节点（`up = 1`）活着且 `len(line) + height ≤ domain.MaxDepth`（`Depth(line) + h - 1 ≤ 10` 的同一个式子）→ `original`；父节点已删 → 根下，`parent_in_trash`；否则根下，`too_deep`。
- `TrashRootOf`（`:one`）：从节点往上，直到 `trash_root`（非根的已删节点，父节点一定是同一时刻删除的）；列名都带表的别名（sqlc 对 `id = sqlc.arg(id)` 报歧义）：

  ```sql
  WITH RECURSIVE up AS (
      SELECT n.id, n.parent_id, n.trash_root, 0 AS hops FROM nodes n WHERE n.id = sqlc.arg(id) AND n.deleted_at IS NOT NULL
      UNION ALL
      SELECT p.id, p.parent_id, p.trash_root, u.hops + 1
      FROM up u JOIN nodes p ON p.id = u.parent_id
      WHERE NOT u.trash_root AND u.hops < 64
  )
  SELECT up.id FROM up WHERE up.trash_root;
  ```

- `TrashGroup`：根之后按层读，每层一条语句、以它的父节点规划（`postgres.Planned`，同 `Subtree`）。根由单独的 `TrashGroupRoot` 读：`FindTrashRoot` 的行没有正文的大小；它以 `id`、笔记本与时刻 T 认根（`n.deleted_at = T`，同下面每层的条件），不要求 `trash_root`，这期间被恢复或以别的时刻再删的根读不到，答 `page.trash_changed`。每层：

  ```sql
  -- name: TrashChildrenOfAll :many
  SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key, n.sort_order, n.created_by_id, n.updated_by_id,
      n.created_at, n.updated_at, c.revision, c.byte_size
  FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id AND c.deleted_at = n.deleted_at
  WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id = ANY(sqlc.arg(parents)::uuid[])
      AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz
  ORDER BY n.sort_order, n.id;
  ```

  更早删除的子孙（时刻不同）不在组里，它是自己的一项。`nodes_notebook_id_parent_id_idx` 含已删的行，这一读走它。
- `TrashContents`：`SELECT node_id, content FROM page_contents WHERE node_id = ANY($1) AND deleted_at = $2`。留作缓存的语句：它的通用计划与 `TrashHistory` 的相同（下），但 ids 与这一时刻的行都至多是一项的页（100），两种计划都约 0.3 ms（修复核对第一轮的实测，3.11）。
- `TrashHistory`（审查 A-M4 加的，恢复的第 4 步用）：这一组的节点以时刻 T 删除的版本行与所有变更集里的条目，两张表各数到 `lim`（用例给界线加一）为止：

  ```sql
  -- name: TrashHistory :one
  SELECT ((SELECT count(*) FROM (SELECT 1 FROM page_revisions r
              WHERE r.node_id = ANY(sqlc.arg(ids)::uuid[]) AND r.deleted_at = sqlc.arg(deleted_at)::timestamptz
              LIMIT sqlc.arg(lim)::integer) v)
      + (SELECT count(*) FROM (SELECT 1 FROM changeset_items i
              WHERE i.node_id = ANY(sqlc.arg(ids)::uuid[]) AND i.deleted_at = sqlc.arg(deleted_at)::timestamptz
              LIMIT sqlc.arg(lim)::integer) w))::integer AS rows;
  ```

  它恰是 `RestoreNodes` 的 `revisions`、`items` 两步要恢复的行。各数到界线加一不改变判定：和超过界线时，要么有一项被截在界线加一（已超过），要么两项都准。每次以它的 ids 规划（`s.planned(ctx)`）：作为缓存的语句，一个连接上第 6 次起服务端可能只规划一次、对任何参数都用，统计是活着时取的（生产库的常态）时这种计划估 `deleted_at = $2` 一行、经 `*_deleted_at_idx` 读这一时刻的行，再把每行与上万个 id 逐个比较：两个界线上、页排在 ids 最后时 5.0–5.5 秒，按参数的 41–67 ms（修复核对第一轮 I-1）。按参数的计划读 `*_deleted_at_idx`（这一时刻的行正是这一组的）、`node_id` 的索引，或在组占了表的大半时顺序扫描、由 `LIMIT` 停下。
- `Nodes.FindAnyNode` 多读 `trash_root`（`AnyNode.TrashRoot`）。时刻的比较一律 `time.Time.Equal`（答复的时刻可能与参数的不在同一个时区：测试的连接池按本地时区读，`==` 连时区也比）。
- `TrashCursor` 的 JSON 以 UTC 写时刻（`MarshalJSON`）：按别的时区读到的时刻会把同一个位置拼成两种，`shared.DecodeCursor` 拒绝第二种（13.1 第 27 条"每个位置只有一种拼法"）。

**附件的信息**（4.1，P5 的搜索也用）：asset 的根加只凭连接池的 `asset.NewAttachmentInfos(pool) AttachmentInfos`（带连接池的结构体，模块常用的构造），`Of(ctx, nodeIDs) (map[uuid.UUID]AttachmentInfo, error)`，`AttachmentInfo{MIME string; Bytes int64}`：

```sql
-- name: AttachmentInfos :many
SELECT node_id, mime, byte_size FROM asset_blobs WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]);
```

不按 `deleted_at` 过滤（回收站里的附件要读）；在调用者的读里、走 `asset_blobs_node_id_key`。page 的端口 `app.AttachmentInfos`，bootstrap 的 `attachmentInfos` 适配，`page.Deps.Attachments`；`page.New` 对空的端口 panic（组合根交空即接线的错，同 `markdownadapter.NewBudget`）。不叫 `AssetMeta`：`page.AssetMeta` 已是守卫看到的新附件的文件（含 SHA-256），两者用途不同。

计划：`ListTrash`、`TrashLines` 各有一个"一个连接上读 8 次"的成本测试（`//go:build !race`，照 `subtree_cost_test.go`：一本笔记本 10 万个节点、其中 1 万个回收站的根；无统计，修复核对第一轮之后另跑一遍有统计的），各 200 ms 之内；`TrashHistory` 有它自己的（3.11）。其余新的语句按参数的形状留作缓存的语句：`ListTrash`、`FindTrashRoot`、`TrashRootOf`、`TrashGroupRoot`、`SourcePath` 是标量参数；`TrashLines` 至多一页的根、按主键；`RecordItems` 从 `unnest` 插入；`DeleteNodes` 的 `stray` 读单元自己的时刻的行（平常没有）；`AttachmentInfos` 按 `node_id` 的唯一键、没有时刻（两种计划都约 6 ms）；`RestoreNodes` 连接 `u` 与 `back`、没有 `ANY` 的过滤（两个界线上两种计划都约 2.6–3.3 秒）。

### 3.5 用例

- **`ListTrash`**（`GET /notebooks/{notebook_id}/trash?limit&cursor`）：游标先核对（400）；笔记本的 `WorkspaceOf` → 以 `page.history` 判定（看不到答 404 `notebook.not_found`）；`limit`（422，`shared.PageSize`）；`since = now − (Retention − TrashMargin)`；读 `limit + 1` 项；`TrashLines`（一条）、删除者的显示名（`Names`，一次）、附件根的信息（`Attachments`，一次）。每项：`TrashRoot`、原位置、删除者、`RestorableUntil`、统计、放置。
- **`GetTrashItem`**（`GET /nodes/{node_id}/trash-item`）：`readableNode(id, page.history)`（不看类型；404 `page.not_found`）→ 活着的 409 `page.not_in_trash` → 不是根的 409 `page.not_trash_root`（`root_id` 由 `TrashRootOf`）→ 期限之外 409 `page.restore_expired` → 同列表的一项。两次读之间这一项被恢复了（`TrashRootOf`、`FindTrashRoot` 读不到）答 409 `page.not_in_trash`。
- **`RestoreNode`**（`POST /nodes/{node_id}/restore`，没有请求体），码照 4.14 的次序：
  1. `FindAnyNode`，笔记本活着，未加锁：没有、笔记本已删 404 `page.not_found`；`Writer.Allowed`（`node.restore`）：没有角色 404、读者 403。
  2. 活着 → 409 `page.not_in_trash`；不是根 → 409 `page.not_trash_root{root_id}`；`!Restorable(deletedAt, now, Retention)` → 409 `page.restore_expired`。两次读之间这一项被恢复了（`TrashRootOf`、`FindTrashRoot` 读不到）→ 409 `page.not_in_trash`，不是 500（审查 C-N1）。
  3. `FindTrashRoot` 的统计：`pages + assets > MaxUnitNodes` → 422 `page.restore_too_large`（不读组）。
  4. `TrashGroup`（未加锁；读不到这一组即这期间被恢复或清理了 → 409 `page.trash_changed`）：多于一页时，页数 `> MaxUnitPages`（100）或正文字节 `> MaxUnitBytes`（8 MiB）→ 422；然后 `TrashHistory(ids, T, MaxUnitHistory + 1)`：这一组的版本与条目 `> MaxUnitHistory`（400,000，只有一页也算）→ 422（审查 A-M4）。数历史不加锁：在回收站里它这一时刻的行只会被清理减少，单元写的不会多于数过的；锁下另核对组（`Unit.Restore` 第 2 步）。
  5. `TrashContents`：读到的正文少于页（清理拿走了正文，含版本号为 0 的页：它在这一时刻没有正文行）→ 409 `page.trash_changed`，不去解析空的正文。这个 409 在页数、字节、历史的 422 之后（同一遍读出的拒绝，先答永久的那一种，客户端不会重试一个太大的项；00-M8 4.14 的 422 在 409 之前），却在解析的 503 之前：4.14 次序的又一个例外。然后 `ContentParser.DecidedAll(ctx, contents, ErrRestoreTooLarge)`：按大小从大到小 `Take`，各次的等待合计至多 `parse_max_wait`（`page.New` 以 `Budget.Wait()` 交给 `ContentParser`，不另加配置；每次 `Take` 的期限是剩下的等待；它因这一期限失败、请求本身未到期时答 503 `server_busy`）；多于一页时边解析边数链接，超过 `MaxUnitLinks` 就停下、放回预算、答调用者给的码（P3 的撤销传它自己的，`page.revert_too_large`；链接数只能解析之后知道，所以这个 422 可能排在 503 之后，契约写明）。
  6. `Writer.Run(UnitSpec{NotebookID, Action: node.restore, Tree: true, Client, NotFound: ErrNotFound, Kind: trash_restore}, u.Restore(…))`。`Options` 留空：改写链接的参与者对没有"前"的变更什么都不做（`Relocation` 答 false）。
  7. 答 `NodeRestored`；记日志 `node restored`（ids、`nodes`、`pages`、`assets`、放置的原因、根是否改名、组内改名的个数、客户端；不记名称）。

  所以码的次序是：404 → 403 → 409（`not_in_trash`、`not_trash_root`、`restore_expired`）→ 422（节点数，按记下的统计；页数、字节；历史的行）→ 409 `page.trash_changed`（未加锁时正文已被清理）→ 503 → 422（链接数）→ 单元的 409（`page.trash_changed`：锁下的核对与观察者；`page.restore_expired`：以单元的时刻再核对）。`restoreNode` 的描述照这个次序写全（审查 B-3）。

**`Unit.Restore`**（`unit_restore.go`，笔记本行 `FOR NO KEY UPDATE` 之下）：

1. 重读根（`FindAnyNode`）：仍在这本笔记本、仍以 T 删除（`Equal`）、仍是根，否则 409 `page.trash_changed`（这期间别人恢复了它、清理拿走了它，或别的变化：一律"这一项变了"，网页重读）。以单元的时刻再核对期限（409 `page.restore_expired`）。
2. 重读 `TrashGroup`，与用例第 4 步未加锁读到的逐项比较（节点、类型、页的版本号）：不同则 409 `page.trash_changed`。不单独核对版本号 0：未加锁时已拒绝（用例第 5 步），之后被拿走的正文版本号不同（`TrashGroupRoot` 与每层读的是这一时刻的正文）。
3. **放在哪里**：`PlacementOf`，父节点是否活着由单元在锁下以 `TrashLines` 重读根的线（不是 `lineUnder`：列表、单项与恢复以同一个 `PlacementOf` 放置同一种线；已删的父节点即 `parent_in_trash`），层数用第 2 步的组。原处时次序值用它原来的（不重算：兄弟这期间被重新编号过时，它落在按值排的位置，与兄弟同值时按 id，写明即可，调研第 25 项）；根下时放在末尾（`placeAmong(根的子节点, len − 1)`，可能重新编号根的子节点）。
4. **名称**：目标位置活着的兄弟建 `children`，`free(root.Name, asset, nil)`：冲突时"名称 2"、附件加在扩展名之前（同导入）。`free` 照今天的规则重核对名称：存下的名称就是 `CheckTitle` 的输出，规则未变，核对必然通过，不进契约的码（调研第 26 项）。组里每个节点的键以 `shared.TitleKey` 重算（`reindex` 只重算活着的）；组内同一父节点下键相同的（Unicode 数据升级之后的 `reindex` 才会有），按次序值与 id 先到的留名，后到的照根的规则加序号（每个父节点一个 `children`，只含组里的兄弟：已删的页下面不会有活着的节点）。不答 409：那会是永久的拒绝，回收站里的这一项再也恢复不了（审查 A-N6）。
5. **变更**：每个节点一项 `Change{NodeID, After: &state, Revision: 页的版本号, Facts, Restored: &Restored{DeletedAt: T, Asset: 是附件}}`（根的 `state` 是新的父节点、名称与次序，改了名的组员是新名称）；`u.apply(ctx, u.step(OpRestore, changes...), true, write)`。守卫：编辑锁对 `OpRestore` 放行（不改代码，加测试）；参与者：改写的参与者对没有"前"的变更什么都不做。
6. **写**：重新编号（若有），然后 `NodeWriter.RestoreNodes`：

   ```sql
   -- name: RestoreNodes :one
   WITH u AS (
       SELECT unnest(sqlc.arg(ids)::uuid[]) AS id, unnest(sqlc.arg(names)::text[]) AS name,
           unnest(sqlc.arg(name_keys)::text[]) AS name_key
   ), back AS (
       UPDATE nodes n
       SET deleted_at = NULL, trash_root = false, trash_pages = NULL, trash_assets = NULL, trash_height = NULL,
           name = u.name, name_key = u.name_key, updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now),
           parent_id = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.narg(parent_id)::uuid ELSE n.parent_id END,
           sort_order = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(sort_order)::float8 ELSE n.sort_order END
       FROM u
       WHERE n.id = u.id AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz
       RETURNING n.id, sqlc.arg(deleted_at)::timestamptz AS deleted_at
   ), contents AS (
       UPDATE page_contents c SET deleted_at = NULL
       FROM back b
       WHERE c.node_id = b.id AND c.deleted_at IS NOT DISTINCT FROM b.deleted_at
       RETURNING c.node_id, c.revision
   ), revisions AS (
       UPDATE page_revisions r SET deleted_at = NULL
       FROM back b
       WHERE r.node_id = b.id AND r.deleted_at IS NOT DISTINCT FROM b.deleted_at
       RETURNING r.node_id, r.revision, r.base_revision
   ), items AS (
       UPDATE changeset_items i SET deleted_at = NULL
       FROM back b
       WHERE i.node_id = b.id AND i.deleted_at IS NOT DISTINCT FROM b.deleted_at
   )
   SELECT (SELECT count(*) FROM back)::integer AS nodes,
       (SELECT count(*) FROM contents)::integer AS contents,
       (SELECT count(*) FROM (SELECT node_id, revision FROM contents INTERSECT ALL
           SELECT node_id, revision FROM revisions) v)::integer AS current_versions,
       (SELECT count(*) FROM revisions WHERE base_revision IS NULL)::integer AS creations,
       (SELECT count(*) FROM (SELECT base_revision, lag(revision) OVER (PARTITION BY node_id ORDER BY revision) AS prior
           FROM revisions) v WHERE base_revision IS DISTINCT FROM prior)::integer AS gaps;
   ```

   版本行与条目在所有变更集里、`deleted_at` 等于 T 的都恢复（含删除它们的变更集的条目）。正文的 `updated_*` 不动（正文没变）。期望：`nodes` 是组的节点数；`contents`、`current_versions`、`creations` 都是页数；`gaps` 为零（每页恰有一个创建的版本行，之后每一行的 base 是前一行的版本号：`unit_content.go` 的写与 P1 的链守卫）。五个数整体比较，不符 → 409 `page.trash_changed`。理由：清理与这条语句之间没有锁，清理先删条目、版本（每批 1,000 行、不排序），后删正文；只数正文会放过"有正文、版本断了链"的页（`checkPages` 的"版本不连成链"，P1 的 `FindVersion` 会以下一版答链缝里的版本号）。`current_versions` 是唯一抓得到"只拿走了一页最新的版本"的：拿走链的最后一环不留缝（审查 A-M3）。兄弟的唯一索引逐行核对：组里的其余节点在更新之前仍是已删的（不在部分索引里），根与组员的名称已由第 4 步定下，不会途中撞上；真撞上了是缺陷，答未分类的错误（500），不是契约之外的 `page.title_taken`（审查 B-4）。

   **写法是实测定下的**（3.11），与最初的设计有三处不同：
   - 跟随节点的三步从 `back` 按节点 id 连接，时刻以 `x.deleted_at IS NOT DISTINCT FROM b.deleted_at` 比较，`back` 答 `$T AS deleted_at`。原来的 `node_id IN (SELECT id FROM back) AND deleted_at = $T` 在过时的统计下（统计从来不认识比它新的时刻）把这一时刻估成一行，规划器为每行整读一遍 `back`：恢复一组 10,100 个节点 14.8 秒、请求超时，测的各组上 14.8–33 秒（版本的一步规划器碰巧建了哈希、不慢，三步仍保持一种写法）。写成 `= b.deleted_at` 则每个节点读一遍 `deleted_at` 的索引（2 万 × 4 万个索引条目），27–48 秒。`IS NOT DISTINCT FROM` 对非空的 T 等于 `=`，只是没有索引用得上它。节点那一步照设计：强制通用计划与过时的统计下 2 万个节点不慢。
   - `current_versions` 是 `contents INTERSECT ALL revisions`（`(node_id, revision)` 唯一，与设计的连接等价）。
   - `gaps` 是每页按版本号排序后 `base_revision IS DISTINCT FROM lag(revision)` 的行：一页的第一行要是创建（base 为空），之后每一行的 base 是前一行的版本号，即 `checkPages` 的链，比设计的"base 在这次恢复的版本里"稍严，设计列的各例计数相同。CTE 的 `NOT EXISTS` 自连接被估成一行、成了嵌套循环，100 页 × 200 个版本 12 秒。
   **防线之外的**（写明，不检测）：被清理拿走的条目（便宜的办法查不出：恢复的页在旧变更集里少了条目，P3 的截掉标记之前不留痕）；附件的清理删了文件、提交失败留下的行（行数对得上，文件不在：只在下载时 404 并记警告）。两者都要清理的时钟比恢复的快一小时以上：清理只取 `deleted_at < now − R`，恢复只认 `deleted_at > now − R + 1h`。
7. 答复：根的节点（新的父节点、名称）、`renamed`、放置的原因、变更集。

### 3.6 观察者与 `Change.Restored`

- `domain.Change` 加 `Restored *Restored`（`Restored{DeletedAt time.Time; Asset bool}`，与 `LinkNode.Asset`、`ExportNode.Asset` 同样以布尔分开附件：page 的根不导出节点的类型），只在把节点从回收站带回来的变更上不空：`Before` 为空时，它分开恢复与新建。`Then` 取第一项的 `Restored`（它与 `Before` 同出于第一项；M8 的单元不在同一变更集里删除再恢复，M9 的 batch 另说）。
- **asset**：`asset.PageObserver` 加 `NodesRestored(ctx, ids []uuid.UUID, deletedAt time.Time) (int, error)` → `Follow` → `RestoreBlobsOfNodes`：

  ```sql
  -- name: RestoreBlobsOfNodes :execrows
  UPDATE asset_blobs SET deleted_at = NULL
  WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at = sqlc.arg(deleted_at)::timestamptz;
  ```

  按参数规划（asset 的 store 加 `planned`）：作为缓存的语句，统计是活着时取的时第 6 次起的通用计划读这一时刻的行、与 id 逐个比较，节点的界线上 108–114 ms 对 40–43 ms（修复核对第一轮 I-1 一并比过的，`TestRestoreBlobsOfNodesIsPlannedWithItsNodes` 核对它不在 `pg_prepared_statements` 里）。bootstrap 的 `assetObserver`：删除的照旧交 `NodesDeleted`；`Restored` 不空且 `Asset` 的按 `DeletedAt` 分组交 `NodesRestored`，答的行数不等于这一组的附件数 → `page.ErrTrashChanged`（清理拿走了行：恢复的附件会没有行，下载 404、`checkAssets` 失败）。asset 不知道 page 的码，由组合根换成它。行与节点的 `deleted_at` 出自同一个单元的 `At`，`Restored.DeletedAt` 从节点读回，与行的相等。
- **linking**：观察者不改。恢复的页的变更没有"前"、带 `Revision` 与 `Facts`，观察者 `ReplacePage` 并重新解析以这些名称为目标的链接（`links` 事件照常）。它写解析的 M6 语句 `SetResolutions` 改为按参数规划（3.11 的实测：通用计划 53 秒）。
- **`pages` 事件**：恢复的页带它（未变的）版本号出现在 `pages` 里、`tree: true`：网页把它当作这一页在那个版本写过，重读至多多一次视图，接受（调研第 24 项）。
- `page/extension_test.go` 另加 `TestTheObserversFollowARestore`（不并进 `TestTheObserversFollowAUnit`）：观察者看到 `Restored` 的时刻与是否附件。

### 3.7 回收站里的页的链接

- `app.PageRef`、`platform/markdown.Page`、`linking/app.Page` 各加 `Deleted bool`：渲染时来源在回收站里。`GetVersionView` 以 `n.DeletedAt != nil` 设它；`GetPageView` 只读活着的页，不设。
- linking 的 `Pages` 端口加 `SourcePath(ctx, notebookID, id) (domain.Node, bool, error)`：节点经它的祖先（已删与否）到根的路径；page 的 `LinkTargets.SourcePath`：

  ```sql
  -- name: SourcePath :many
  WITH RECURSIVE chain AS (
      SELECT n.id, n.parent_id, n.name, n.name_key, n.kind, 0 AS up FROM nodes n
      WHERE n.id = sqlc.arg(id) AND n.notebook_id = sqlc.arg(notebook_id)
      UNION ALL
      SELECT p.id, p.parent_id, p.name, p.name_key, p.kind, c.up + 1
      FROM chain c CROSS JOIN LATERAL (
          SELECT id, parent_id, name, name_key, kind, notebook_id FROM nodes WHERE id = c.parent_id LIMIT 1
      ) p
      WHERE c.up < 64 AND p.notebook_id = sqlc.arg(notebook_id)
  )
  SELECT id, parent_id, name, name_key, kind, up::integer AS up FROM chain ORDER BY up DESC;
  ```

  它对活着的节点也成立：两次读之间这一页被恢复了，路径照样对。
- `Views.Resolve`：`p.Deleted` 时不读索引（回收站里的页没有索引行），读 `SourcePath`，交给 `resolutions`；`resolutions` 加一个参数 `known []domain.Node`（已读到路径的来源），它们进 `byID`、不进 `Paths` 的 ids；候选与别名的页仍只有活着的。`Index.resolve` 传空，照旧把缺失的来源当作缺陷。
- **解析的含义**：从原来的路径出发、对着今天活着的树解析（00-M8 4.1"候选的目标仍只有活着的节点"）。所以链到同组里的页（例如 R/A 的 `[[B]]`，R/B 与它一起删除）：组里的 B 不是候选，链接解析到别处活着的同名页（"同一文件夹优先"找不到时取最短的路径），或不解析；恢复之后才回到 R/B。别名同理（组里的页的别名不在索引里）。不为预览把组加进候选：那要每次视图读一次组、再把赢的已删节点改成"不解析"，换来的只是一种不同的不准；预览是只读的、标明了是回收站里的页（4.3 的说明一句）。
- 回收站里的页嵌入的同组附件：候选只有活着的，仍未解析，与"不签发下载地址"一致；属性（`getPageProperties` 只读活着的页）预览不显示（同版本页，调研第 22 项）。
- 测试：P1 的 `TestAPageInTheTrashKeepsItsHistory` 改为断言链接解析到 Target；A、B 下各有一页 Notes，A 下的页删除之后，它的 `[[Notes]]` 在视图里仍解析到 A/Notes（按原路径的远近）；钉住上一条：R/A 与 R/B 一起删除、Other/B 活着，R/A 的视图里 `[[B]]` 解析到 Other/B，恢复之后解析到 R/B。

### 3.8 期限、保留期与 `InstanceInfo`

- `domain.TrashMargin = time.Hour`；`RestorableUntil(T, R) = T + R − TrashMargin`；`Restorable(T, now, R)` 即 `now < RestorableUntil`（与清理的 `deleted_at < now − R` 之间隔一小时，两者从不碰同一组）。列表的 `since = now − (R − TrashMargin)`。
- bootstrap 的一个 `retention(cfg)` 同时给清理任务的 `PurgeConfig.Retention`、`page.Deps.Retention` 与 `instance.Deps.TrashRetention`（它减 `page.TrashMargin`，bootstrap 算：instance 不能 import page）：间隔要求两边是同一个值，交错测试不经 `PurgeJob`，接错了看不出来。`domain.Info`、`app.Settings`、处理器跟着加，`trash_retention_seconds` 是秒的整数。
- `validate`：`jobs.purge_retention` 至少 24h（错误 "must be at least 24h"），否则一小时的间隔会让回收站几乎是空的；`config.yaml` 的注释改为"至少 1 天；回收站可以恢复的期限是它减 1 小时"，README 的"软删除与清理"一段同。
- **清理的时钟**：不加 `PurgeConfig.Clock`。交错测试直接调用清理器（`tm.purge`），截止时刻由测试给：`tm.purge` 改为 `tm.purgeBefore(t, before, tables...)`（原来的调用传 `time.Now()`；给了表名时只跑这些表的清理器，交错 5 用它停在版本与正文之间），期限两侧的测试传 `now − R` 或更晚的时刻（模拟时钟偏差）。期限的边界（恰在期限的那一刻不能恢复）在 `page/app` 以假时钟测；整个程序上的测试以 SQL 移动一组的时刻（3.10 的 `tm.trashedAgo`）。
- **随之改的固定值**：`platform/config/validate.go`、`validate_test.go:124`；`server/configs/config.yaml`、README；`instance/adapter/http/handler_test.go`（整个 JSON）、`instance/app/get_info_test.go`；`bootstrap/api_test.go`；`platform/httpserver/apitest/apitest_test.go` 的 `instanceJSON`（契约是封闭的）；`web/apps/web/src/test/fakes.ts` 的 `instanceJSON`；e2e `s3-instance-info.spec.ts`（默认 60 天减 1 小时，5,180,400）；`deploy/image-smoke.sh` 对 `/api/v0/instance` 的整体比较（镜像的 prod 配置同为 1440h，5,180,400；实施时漏了，`16de05b` 的 CI 只有它失败，`463d9e9` 补上）；`bootstrap/app_test.go` 的 1440h 与 `configs/embed_test.go` 的 60 天不变。

### 3.9 契约

- **`listTrash`**：`TrashItemPage {data: TrashItem[], next_cursor}`；`[bad_request, notebook.not_found, validation_failed]`。
- **`getTrashItem`**：`TrashItem`；`[page.not_found, page.not_in_trash, page.not_trash_root, page.restore_expired]`。
- **`restoreNode`**：没有请求体；答 200 `NodeRestored`；`[page.not_found, forbidden, page.not_in_trash, page.not_trash_root, page.restore_expired, page.restore_too_large, server_busy, page.trash_changed]`；描述写放置的规则、序号、期限、一个单元的界线（至多 10,000 个页与附件、它们的版本与条目至多 400,000 行；多于一页时至多 100 页、8 MiB、20,000 条链接），以及码按 3.5 的次序：正文被清理的 `page.trash_changed` 在解析之前，链接数的 422 可能排在 503 之后，到恢复时已过期限的是 `page.restore_expired`（审查 B-3）。码的集合不变。
- **结构**：
  - `TrashItem {id, notebook_id, kind: NodeKind, name, ancestors: Ancestor[]（原位置，含回收站里的祖先）, deleted_at, deleted_by: UserRef, restorable_until, pages, assets（组里的，含根）, restore_to: TrashPlacement, revision: integer|null（页的正文的版本号，预览以它读 getPageVersionView；附件、清理拿走了正文的页为 null，审查 B-2）, asset: TrashAttachment|null（页、清理拿走了文件的附件为 null）}`。`asset` 照 `EditLock.holder` 写成 `anyOf: [{$ref: TrashAttachment}, {type: 'null'}]`（契约的编写规则不认内联的可空对象）。
  - `TrashAttachment {mime, byte_size}`（`additionalProperties: false`）。
  - `TrashPlacement {parent_id: uuid|null, reason: original | parent_in_trash | too_deep}`：`original` 时 `parent_id` 是原父节点（原来就在根下则为 null），其余为 null（根下）。
  - `NodeRestored {id, kind, name, renamed, placed: TrashPlacement, changeset_id}`。
- **`UserRef`**（`common.yaml`，`{user_id, display_name}`，`additionalProperties: false`）：`TrashItem.deleted_by` 用它；P1A 的 `ChangesetAuthor` 与形状完全相同的 `EditLockHolder`、`TransferStarter` 一并改用它（P1A 审查 C6：一种形状一个名字，第四个同形的结构不再出现；各处的说明移到引用它的属性上；契约是 OpenAPI 3.1，`$ref` 旁本也可以写 `description`，实施报告"3.0 忽略它"的理由不对，做法照设计，审查 B）。审查以仓库的 `oapi-codegen` 与 `openapi-typescript` 实测过：生成 `apigen.UserRef`，`EditLock.Holder` 是 `nullable.Nullable[externalRef0.UserRef]`，`bodyshape.gen.go` 不变。Go 里要改的：`page/adapter/http/handler.go:330,333`、`versions.go:99`、`transfer/adapter/http/server.go:99`（各加 `apigen` 的 import：archtest 只拦模块自己的 `gen` 被别处 import）。网页只改 `services/page.service.ts` 的转出（`export type EditLockHolder = UserRef`），`edit-session.ts` 与 P1B 的分支只用字段。`Problem.ended_by` 形状相同但留作内联：`Problem` 与 `httpserver.Problem` 逐字段对照，不动它。合并次序：P1B 若先合并，P2A 在它之上改 `page.service.ts` 这一行。
- **扩展成员 `root_id`**：`Problem` 加可选的 `root_id`（"page.not_trash_root 的这一组的根；别的码不带"）；`shared.Error.RootID *uuid.UUID`、`ProblemRootID()`，`httpserver` 的接口与 `Problem.RootID`；`domain.NotTrashRoot(rootID)` 设它。测试照 `ended_by`：`platform/httpserver/contract_test.go` 加一例（平台的 problem 对照 `Problem` 结构）、`apierrors_test.go`、`shared/error_test.go` 的有与无。
- **`getInstance`**：`InstanceInfo` 加必有的 `trash_retention_seconds`（`minimum: 82800`），描述"删除的页与附件在回收站里可以恢复多久：`jobs.purge_retention` 减 1 小时"。
- `TreeNode.updated_at` 与 `Page.updated_at` 的描述加"或从回收站恢复"（调研第 33 项）；`deleteNode` 的描述加"可以经 restoreNode 恢复到 restorable_until"。
- 网页的门禁（3.1 的最后一行）：五个新码的 `problemMessages` 与 `problem.*` 文字（两种语言），否则 `problem-messages.test.ts` 失败。

### 3.10 权限、日志与交错

- `node.restore`：编辑者、管理员（`rules.go`，`ActionRestore` 进 `Actions()`；`bootstrap/actions_test.go` 的"规则 = 动作"随之通过）。列表与单项用 `page.history`（读者以上）。
- **权限矩阵**：`listTrash`（`readers(notebookNotFound)`，`check`：种下的那一项列出）、`getTrashItem`（`readers(pageNotFound)`）、`restoreNode`（`editorsOnly(cellOK(), pageNotFound)`，`write: true`）。种子：每本目标笔记本一页"Trashed X"，照矩阵的页建好（节点、正文、`seededPageHistory` 的变更集、条目与版本 1，都活着），再以 SQL 在同一时刻（一小时前）删除它的节点、正文、版本与条目并标根与统计；id 进 `newSeeded` 与 `workspaceOfRow`（新的 `matrixTrash()`），不进 `matrixPages()`（`treeOf`、`listNodes` 的核对只认活着的）。`gone-nb` 的删除循环（`permission_matrix_seeded_test.go:583` 起）改为只动 `deleted_at IS NULL` 的行（同 `DeleteNotebooksPages`），不覆盖回收站里的页更早的时刻。
- 日志：恢复记 `node restored`（3.5）；读不记。
- **一组的时刻移动**：`tm.trashedAgo(t, rootID, d)` 以这一组的删除时刻 T 为准整体移动：节点（`deleted_at = T` 的组）、正文、版本、条目、附件的行各一条 `SET deleted_at = deleted_at − $d WHERE deleted_at = $T AND …`，不用每条语句各自的 `now()`（那会让一组的时刻各不相同，`TrashGroup` 与 `RestoreNodes` 的相等全部落空）。e2e 的 `trashedAgo` 同（4.7）。
- **交错**（`bootstrap/interleavings_trash_test.go`，每个结束时 `checkPages`、`checkAssets`、`checkNotebooks`）：
  1. 恢复与删除它原来的父节点（`notebookRow`）：恢复在先 → 回到原处，再随父节点一起删除（同一个新时刻，不是根）；删除在先 → 恢复到根下（`parent_in_trash`）。
  2. 恢复与在原位置新建同名的页（`notebookRow`）：新建在先 → 恢复为"名称 2"、`renamed`；恢复在先 → 新建 409 `page.title_taken`。
  3. 同一项的两次恢复：后一个 409 `page.trash_changed`，只有一个 `trash_restore` 变更集。
  4. 恢复与清理，期限两侧（保留期 1 天，`newAcmeTeamWith` 的配置）：两组以 `trashedAgo` 移到"可恢复的期限之内一分钟"与"期限与清理的截止之间"（删除于 `now − R + 30 分钟`）。第一组：恢复在笔记本行上等着时清理以 `now − R` 运行，什么都不删，放开之后恢复 200；第二组：恢复在加锁之前就答 409 `page.restore_expired`，同一次清理不碰它（一小时的间隔）。
  5. 恢复时清理拿走了正文（模拟时钟偏差）：恢复在笔记本行上等着时，清理以 `now + 2h` 只运行条目、版本、正文三个清理器；放开之后 409 `page.trash_changed`，什么都没恢复。另一例清理停在版本与正文之间（只运行条目、版本两个清理器）：锁下读到的组与未加锁时的相同，只有 `RestoreNodes` 的计数拒绝（实施时由存活的变异 `lock-rows-whole-program` 发现，`a55095a`）。
  6. 恢复附件的行与附件的清理（M7 的移交第 2 项）：`holdRow` 拿着一个组里附件的行（清理的批），恢复在 `asset_blobs` 上等；拿着的一方删掉文件与行并提交 → 409 `page.trash_changed`、整个单元回滚；拿着的一方回滚 → 恢复 200、文件在、可以下载。
  7. 嵌套的根与它父节点的组（`notebookRow`）：D 先被删、之后它的父节点 P 被删；恢复 D 与恢复 P 交错：P 在先 → D 回到 P 下（`original`）；D 在先 → D 到根下（`parent_in_trash`）。
  8. 原父节点这期间被移深（`notebookRow`）：移动在先 → 恢复到根下（`too_deep`，单元在锁下重算，不用未加锁时的预测）；恢复在先 → 回到原处，之后的移动照它自己的层数核对。
  9. 恢复等着时删除组里的一页：删除未加锁的 `FindNode` 就答 404，不等；放开之后恢复 200；再删除恢复了的那个子页，它成为新的根、带它自己的统计，根的不变式成立。

  另有 `TestAGroupThePurgeIsMidwayThrough`（修复核对第一轮 N-1）：真的清理器拿走一组的条目、版本与正文，再删一批 1 个叶子，`checkPages` 对这样的组成立。未加锁的两次读之间的竞态（读根、读组、锁下根已被清理，各答 409，审查 C-N1）不在整个程序上交错：`page/app` 的假存储以 `recorder.before` 在指名的读之前跑"另一个事务"。

### 3.11 一个单元的大小与实测

- 界线（`domain/trash.go`，P3 的撤销同用）：`MaxUnitPages = 100`、`MaxUnitBytes = 8 MiB`、`MaxUnitLinks = 20,000`（与导入的一批相同；只有一页的组不受这三项限制，`UnitTooLarge`）；`MaxUnitNodes = 10,000`（树的操作：组里的页与附件，按根记下的统计，读组之前）；`MaxUnitHistory = 400,000`（这一组以它的时刻删除的版本行与所有变更集里的条目，`TrashHistory` 数，只有一页也算；审查 A-M4 加的：恢复与删除还随每页的版本与条目增长，原来的界线管不到它）。后两个实测，经 `RestoreBounds` 交给用例；`page.Deps.MaxUnitNodes`、`MaxUnitHistory` 为 0 即 domain 的值，生产的 `pageDeps` 不设，只有基准抬高它们。
- **实测**：`bootstrap/trash_cost_test.go`（`//go:build !race`）里的基准，手动经 `heavy.sh` 运行：`go test -run '^$' -bench 'BenchmarkTrash' -benchtime 3x -timeout 60m ./internal/bootstrap/`。
  - **经单元、进程内**：基准自己组一个 page 模块：`pageDeps(...)`（真的观察者：事件、索引、附件；真的预算），抬高两个界线；路由挂在以 `apiConfig` 组的 API 上（生产的中间件、15 秒的请求期限，连接池 10），不是 `httpservertest`；进程内经 `restoreNode` 的处理器计时。不经正式的 API：它的界线在实测之前会先答 422。
  - **组**：每组至多 100 页（根与 99 个子页面）。两种页的负载：轻（每页 8 KiB、20 条链接）与满（三项界线的上限：100 页合计 8 MiB、20,000 条链接，每页约 82 KiB、200 条，linking 每页至多记 10,000 条，所以至少两页）；附件 0 / 1,000 / 2,000 / 5,000 / 10,000 个（节点与行以 SQL 成批写入，分在各页之下）。数据经同一模块删除。
  - **统计**：表关掉 autovacuum（"无统计"要它），每次请求之前 `VACUUM`（不 `ANALYZE`、不计时）。设计没写这一步：迭代留下的死行会堆积，第四轮 `ANALYZE` 之后删除 1 万个节点 6 秒而不是 1 秒；服务器的 autovacuum 会清掉它们。`BenchmarkTrashRestore` 在无统计的一遍之后、这一项删除着时 `ANALYZE`（删除时的统计）；`BenchmarkTrashRestoreHistory` 在每档的版本写完之后、这一项活着时 `ANALYZE`（服务器的常态），两种统计各一个库。
  - **计时与取值**：单元（恢复的请求减去解析：未加锁的读也算进单元，偏保守）、解析、删除各记。界线取满负载下单元不超过 5 秒的最大一档，向下取整（15 秒的请求期限的三分之一，留给解析的等待与解析本身、更慢的服务器）。数字写在常量的注释里。`-benchtime 3x` 只报 Go 第一次 `b.N = 1` 之后的三次，每档冷的第一次恢复不在表里，由审查 C 的探针补上（下）。
- **`MaxUnitNodes = 10,000`**（`BenchmarkTrashRestore`；Apple M5 Max 18 核，PostgreSQL 18.6 在 Docker 里，旁边别的工作负载约 15，所以各档有噪声、不单调）：

  | 负载 | 附件 | 解析 ms | 单元 ms，无统计 | 单元 ms，删除时 ANALYZE | 删除 ms，无统计 | 删除 ms，ANALYZE |
  |---|---:|---:|---:|---:|---:|---:|
  | 轻 | 0 | 29 | 374 | 457 | 23 | 30 |
  | 轻 | 1,000 | 30 | 802 | 393 | 445 | 160 |
  | 轻 | 2,000 | 32 | 353 | 413 | 159 | 242 |
  | 轻 | 5,000 | 24 | 660 | 746 | 349 | 599 |
  | 轻 | 10,000 | 36 | 964 | 1,263 | 698 | 1,284 |
  | 满 | 0 | 198 | 1,046 | 901 | 55 | 46 |
  | 满 | 1,000 | 241 | 1,029 | 926 | 360 | 214 |
  | 满 | 2,000 | 232 | 477 | 848 | 152 | 266 |
  | 满 | 5,000 | 217 | 659 | 1,755 | 322 | 748 |
  | 满 | 10,000 | 220 | 999 | 2,684 | 591 | 2,520 |

  满负载的单元在 10,100 个节点处至多 2.7 秒，每档都在 5 秒之内，向下取整为 10,000。审查 C 另测了基准没有的状态：每次一组新的满负载 1 万个节点，删除之后立即恢复、不 `VACUUM`（每行一个死版本、索引条目不是 HOT 的），三轮，请求（解析约 0.2 秒）：

  | 状态 | 三轮的恢复 |
  |---|---|
  | 无统计 | 0.97 / 1.37 / 1.50 秒 |
  | 活着时 ANALYZE | 1.15 / 1.34 / 1.50 秒 |
  | 活着时 ANALYZE，强制通用计划 | 1.38 / 1.51 / 1.72 秒 |
  | 无统计，强制通用计划 | 2.73 / 2.63 / 2.82 秒 |

  最坏的单元约 2.6 秒，10,000 成立，`VACUUM` 也没有藏住问题。
- **`MaxUnitHistory = 400,000`**（`BenchmarkTrashRestoreHistory`；同一台机器，负载 5–8）：满负载在节点的界线上（100 页、9,900 个附件，8 MiB、最多的链接），每页 250 / 500 / 1,000 / 1,500 / 2,000 个版本，变更集共用，每个版本 1 KiB 不可压缩的正文（约是一行留在表里的最宽的一半；1.8 KiB 时界线处的恢复多约 8%，修复核对第一轮 N-2），每页每步一条条目；每轮一次删除、一次恢复，历史随它们的条目增长；解析约 205–209 ms。行数是一次恢复写的版本与条目的均值：

  | 历史的行（版本 + 条目） | 单元 ms，无统计 | 单元 ms，活着时 ANALYZE | 删除 ms，无统计 | 删除 ms，ANALYZE |
  |---:|---:|---:|---:|---:|
  | 109,900 | 1,133 | 1,176 | 673 | 696 |
  | 239,900 | 3,318 | 2,857 | 1,476 | 1,389 |
  | 419,900 | 2,852 | 4,288 | 2,505 | 2,299 |
  | 599,900 | 3,853 | 5,703 | 3,624 | 3,214 |
  | 779,900 | 5,553 | 7,284 | 4,858 | 4,233 |

  两种统计下都在 5 秒之内的最大一档是 419,900 行，向下取整为 400,000（与 `MaxUnitNodes` 同一条规则）。ANALYZE 的一列单调、近似线性（约每千行 7.3 ms，约 52 万行处过 5 秒）；无统计的一列在 24 万行处有一个离群（3.3 秒，42 万行 2.9 秒），趋势在 65–70 万行处过 5 秒（修复核对第一轮）。第一次运行 ANALYZE 的各档从 1,000 个版本起失败（建数据的语句被取消，之后的档 404），ANALYZE 的一列取自第二次运行。修复核对第一轮在正好 400,000 行、无统计、不 `VACUUM` 时测得请求 3.62、3.94、3.92 秒（最后一次 1.8 KiB 的版本），单元约 3.4–3.7 秒，余量约 25%。
- **测量中发现、改掉的计划**（都是一种写法让每行读一遍别的行，超过 15 秒的请求）：
  1. linking 的 `SetResolutions`（M6 的语句）：满负载、ANALYZE 的每次恢复都答 500。缓存的语句第 6 次起的通用计划在统计以为没有链接时两两比较 2 万条链接，53 秒（按参数 0.1 秒）。改为按参数规划（`s.planned(ctx)`），`TestTheLinksReachedAreReadPlannedWithTheReach` 随之核对它不在缓存的语句里。
  2. `RestoreNodes` 跟随节点的三步与计数：写法见 3.5 `Unit.Restore` 第 6 步（14.8–48 秒的两种写法、12 秒的自连接）。
  3. `TrashHistory`（修复核对第一轮 I-1，3.4）：缓存时的通用计划 5.0–5.5 秒，按参数 41–67 ms。别的新的数组语句一并在两个界线上比过通用与按参数的计划（无统计 / 活着时 ANALYZE / 删除时 ANALYZE）：`RestoreBlobsOfNodes` 41–64 / 108–114 / 42 ms 对 49–61 / 40–43 / 43 ms，改为按参数规划（3.6）；`TrashContents` 0.26–0.28 对 0.35–0.64 ms，ids 至多 100，留着缓存；`AttachmentInfos` 约 5–7 ms，没有时刻的参数，不变；`RestoreNodes` 2.6–3.3 秒两种计划相近，连接 `u` 与 `back`、没有 `ANY` 的过滤，不变。所以按参数规划的有：`TrashGroup` 的每层（`TrashChildrenOfAll`，设计里就是）、`TrashHistory`、`RestoreBlobsOfNodes`、`SetResolutions`。
- **守住它**（都是 `//go:build !race`，在 `make test-go` 不带竞态检测的一步）：
  - `TestARestoreAtTheUnitsBoundTakesItsTime`（bootstrap）：满负载、1 万个节点、正好 400,000 行历史，恢复 10 秒之内（本机 3.6–4.3 秒，CI 比开发机慢；耗时以 `t.Logf` 记下，这一条在 `make test-go` 里单独一行带 `-v`，CI 的日志里看得到）；再删除、多一行之后答 422 `page.restore_too_large`，不写变更集。基准对为 0 的参数保留 `pageDeps` 的界线，生产的接线抬高了任何一个，这个测试就失败（修复核对第一轮 M-1）。
  - `TestRestoreNodesTakesItsTimeAtAnyStatistics`（page 的适配器）：2 万个节点、100 页各 300 个版本与条目，在一个连接上恢复五次：无统计、活着时、删除时、别的时刻的统计、强制通用计划，各 8 秒之内（本机 0.5–0.8 秒，退化的写法 12–48 秒）。
  - `TestTrashHistoryCountsInTimeAtAnyStatistics`（page 的适配器）：两个界线上（1 万个节点、40 万行，页排在 ids 的最后，`TrashGroup` 按层排、附件多在页的上一层时就是这样），一个连接上数 8 次，三种统计各一遍，每次 1 秒之内（本机 30–200 ms；改回缓存的语句时 68 秒失败）。
- **删除**：`BenchmarkTrashDelete` 删除导入的上限 5 万个节点（100 页轻负载、其余是附件，每次一组新的：恢复它超过单元的界线），1.8 秒（无统计）、2.1 秒（活着时 ANALYZE），远在 15 秒之内。删除没有历史的界线：上表约 42 万行时 2.3–2.5 秒，更多时 3.2–4.9 秒，审查 A 的探针（100 页各 2,000 个版本与条目）2.73 秒；每轮删除与恢复给这一组的历史加约 2 × 节点数条条目，同一大项反复删除、恢复会到达 `MaxUnitHistory`、之后答 422。写进 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 14 项。

### 3.12 测试

- **domain**：`Then` 保留第一项的 `Restored`；`Subtree.Counts`；`Restorable` 的边界（期限当时不可恢复）；`PlacementOf` 的四种（原来在根下、原处、`parent_in_trash`、`too_deep`）；单元的界线（只有一页时不限）。
- **用例**（`page/app`，假的端口与时钟）：`RestoreNode` 的每个码与次序（含链接数的 422 在 503 之后的例外、未加锁时正文已不在的 409）、锁下的每种 `page.trash_changed`、放置与序号（附件"x 2.png"）、键的重算与组内同键的序号、变更的 `Restored` 与 `Revision`、`Kind` 是 `trash_restore`、`Options` 为空；`DecidedAll` 的从大到小、等待合计（假的预算：第一次等去大半，第二次超出剩下的 → 503）、请求到期时不是 503；`ListTrash`、`GetTrashItem` 的游标、期限的过滤、放置、显示名、附件信息；`Delete` 交给 `DeleteNodes` 的根与统计；`recordItems` 一步一次、没有要记的不写、重复的节点是错误；编辑锁对 `OpRestore` 放行。`page/app/asset_test.go` 的调用序列（`"RecordItem in tx"`）随之改名。
- **适配器**（真的 PostgreSQL）：`DeleteNodes` 只标根并记统计、更早删除的子孙保留它的、先删子孙再删祖先时 `stray` 不为零；`RecordItems` 的空值（根下、没有前、没有后）、冲突时保留 `restored` 与 `revision`，`store_test.go`、`purge_test.go` 里直接调用 `RecordItem` 的改为 `RecordItems`、旧的合并行为照旧；`RestoreNodes` 的五个数（删掉一行正文、当前的版本行、创建的版本行、中间的一行版本之后各自不符）；`ListTrash` 的次序、游标、`since`、`TrashFrom`；`TrashLines` 经已删的祖先；`TrashGroup` 不含更早删除的子孙；`TrashRootOf`；`SourcePath` 经已删的祖先；`AttachmentInfos` 含已删的行；`RestoreBlobsOfNodes` 只认同一时刻；两个成本测试（3.4）。
- **linking**：`Views.Resolve` 对 `Deleted` 的来源用 `SourcePath`、不读索引；`resolutions` 的 `known`；`Index.resolve` 对缺失的来源照旧报错。
- **handler 与平台**：契约的结构、路径参数、各码、`root_id` 只在 `page.not_trash_root` 上；`root_id` 的平台测试（3.9）。
- **整个程序**（`bootstrap/trash_test.go`；13.1 第 21 条：组合根交空时失败）：删除带附件与链接的子树 → 列表与单项（统计、原位置、放置、显示名、附件信息、`notebook_id`）→ 回收站里的页的视图从原路径解析（3.7 的三例）→ 恢复：正文、版本、条目（`restored`、`revision`）、附件的行（`GET /assets/{id}` 200、新签的地址下载 200、无主列表的大小算上它们，扩展 `assets_lifecycle_test.go`）、索引（`checkLinks`）、`pages`（`tree: true`）与 `links` 事件；放置的四种与序号；422 的各项界线；`trash_retention_seconds`。
- **不变式**：`checkPages` 加"活着，却有已删的正文、版本或条目"（`"deleted, with what follows it not deleted"` 的反面）与"已删的节点：是根 ⇔ 父节点为空或不是同一时刻删除、且不是随笔记本删除的"；`checkAssets` 加"已删的行的时刻等于它的节点的"。
- TB1 的接口版本在 B 里做（4.7），用到 A 的契约。
- **实施与审查另加的**：适配器里节点、正文、版本在别的时刻的各例，别的时刻的条目留在回收站；`TrashHistory` 只数同一时刻、组里的节点，各数到上限（`TestTrashHistoryCountsTheRowsOfItsTime`）；`RestoreBlobsOfNodes` 按参数规划；交错 5 的"清理停在版本与正文之间"与 `TestAGroupThePurgeIsMidwayThrough`（3.10）；`page/app` 的两次读之间的竞态（读根、读组、锁下根已被清理）各答 409，"这期间当前的版本被清理了"、一组两个父节点下的同名、根也改了名时的 `others_renamed`、100 页加附件可以恢复、超过历史的界线与它的次序；根的变更的 `After`（父节点、名称、次序），整个程序上恢复的条目的 `after_*` 与改名之后的链接（`TestARestoresChangesCarryItsState`）；整个程序上按根记下的统计超过节点的界线答 422（"Counted"）；处理器的版本号 0 答 `null`、带请求体的恢复照常（`TestARestoreReadsNoBody`）；`checkPages` 核对根的统计等于它的组（清理已到这一组、页的根没有正文时不少于剩下的）。守住计划与界线的五个成本测试（`ListTrash`、`TrashLines`、`RestoreNodes`、`TrashHistory` 的与 bootstrap 的界线测试，3.4、3.11）在 Makefile 不带竞态检测的一步。

### 3.13 审查之后的改动

三位审查者（A：数据、恢复的单元与并发；B：契约、用例的答复、权限与别的模块；C：测试、交错、实测与设计的符合）与修复核对的处置见 `reviews/P2A-trash-server-review.md`。与上文最初的设计不同之处已改在各节里：

- **实施时定下的**（设计没写或留了余地；实施者的偏离，审查 C 逐条判过）：`RestoreNodes` 跟随节点的三步从 `back` 按 id 连接、计数用集合运算与窗口，`gaps` 即 `checkPages` 的链（3.5 `Unit.Restore` 第 6 步，实测）；M6 的 `SetResolutions` 按参数规划（3.11）；单元在锁下以 `TrashLines` 重读线（3.5）；`TrashGroupRoot`，`TrashRootOf` 是 `:one`，`TrashLines` 答 `parent_id`，`TrashCursor` 以 UTC 写，`asset.AttachmentInfos` 是结构体（3.4）；`DecidedAll` 由调用者给超过链接数时的码（3.5）；`RecordItems` 一次一个变更集与时刻（3.3）；两次读之间被恢复答 `page.not_in_trash`，读组时 `page.trash_changed`（3.5）；页数、字节的 422 先于正文已被清理的 409（3.5）；`sameGroup` 不再单看版本号 0（3.5）；基准的 API、`VACUUM` 与 `BenchmarkTrashDelete`（3.11）；`purgeBefore` 的表的筛选（3.8）；观察者的恢复是单独的测试（3.6）；page 另导出 `TreeState`（3.1）；`node.restore` 与 `shared.Error.RootID` 在第 1 步、矩阵与观察者的扩展测试在第 6 步（第 5 节）。
- **审查与修复核对带来的**：历史的界线 `MaxUnitHistory` = 400,000 与 `TrashHistory`，未加锁、在页数与字节之后数（A-M4，3.4、3.5、3.11）；`TrashHistory`、`RestoreBlobsOfNodes` 按参数规划（核对 I-1，3.4、3.6、3.11）；两次未加锁的读之间被恢复、锁下根已被清理，都答 409（C-N1，3.5）；未加锁的"正文已被清理"不另设标记，由 `TrashContents` 少了页答出（C-N6，3.5）；`RestoreNodes` 撞上兄弟的唯一索引是缺陷的错误，不答契约之外的 `page.title_taken`（B-4，3.5）；契约写明清理拿走了正文、文件时 `revision`、`asset` 为空，`restoreNode` 的描述写全码的次序与历史的界线（B-2、B-3，3.9）；`checkPages` 核对根的统计（C-N4、核对 N-1，3.12）；测试的补足（A-M1–M3、A-N1、B-1、C-M1、C-M2、C-N5、核对 M-1，3.11、3.12）。

## 4. B：前端

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/services/page.service.ts` | `trash`（每页 50 项，游标）、`trashItem`、`restoreNode`（POST，没有请求体）与类型（`TrashItem`、`TrashItemPage`、`NodeRestored`） |
| `web/apps/web/src/stores/page-tree.store.ts` | `removed` 记下答复时读的序号与去掉规则；`restore`（按节点一个一个、经写入队列；之后的树的读失败而页仍在 `removed` 里时再读一次）；`trash`、`trashItem` 读 |
| `web/apps/web/src/lib/read-pages.ts` | "按已读的页数重读"，回收站的列表用（反链与版本列表的两份私有实现不动，留给 v0.1 之后的打磨） |
| `web/apps/web/src/lib/twins.ts` | `twinsOf(list, keyOf)`（另有一项与它同键的项）与 `shortId`（id 的末六位）：回收站与无主笔记本的清单共用（`ownerless-row.tsx`、`ownerless-page.tsx` 改用它，审查 C 的细节） |
| `web/apps/web/src/pages/page/version-view.tsx` | 从 P1B 的 `version-page.tsx` 移出的只读阅读视图 `VersionView`（参数是笔记本、页 id、版本号与名称）与 `useLeftBehind`（版本页与预览共用；`version-page.tsx` 只留 `useLeftRevision`） |
| `web/apps/web/src/pages/notebook/trash-page.tsx`、`trash-row.tsx` | 回收站页；`trash-row.tsx` 另有列表与预览共用的部分：一项的信息 `TrashFacts`，恢复 `useTrashRestore` 与按钮 `RestoreButton`，同名的区分 `twinsIn` 与名称 `namesOf`，拒绝的说明 `refusalText`，"打开"的去处 `openOf` |
| `web/apps/web/src/pages/notebook/trash-preview.tsx` | 预览：`PreviewShell`（随地址里的节点挂载，持有从这里恢复的结果）、`Preview`、说明页 `NotInTrash` 与 `Gone` |
| `web/apps/web/src/pages/notebook/notebook-places.tsx`、`notebook-layout.tsx` | 左栏的入口 `NotebookPlaces`，与页面树在同一个按笔记本挂载的门户里（P3 再加"历史"） |
| `web/apps/web/src/app/routes.tsx` | `notebooks/:id/trash`、`trash/:nodeId` |
| `web/apps/web/src/events/handlers.ts`、`app/event-stream.tsx` | 新键的重读、连上时的重读 |
| `web/apps/web/src/pages/notebook/page-tree.tsx`、`page-tree-item.tsx`、`pages/page/attachments-section.tsx` | 删除确认框；期限在 `PageTree` 读一次，经 `TreeContext` 交给各行的菜单；附件一节的 `useLocated`（路由状态点名的附件那一行取得焦点） |
| `web/apps/web/src/i18n/format.ts` | `formatPeriod` |
| `web/apps/web/src/pages/page/attachment-row.tsx` | 导出类型图标 `AssetIcon`（预览的附件用）、路由状态 `Located`（`{attachment: id}`）与 `locatedOf`；行的链接带 `data-attachment` |
| `web/apps/web/src/pages/page/breadcrumbs.tsx` | `BreadcrumbTrail`：`Breadcrumbs` 与预览的面包屑共用（审查 C 的细节） |
| `web/apps/web/src/pages/page/unresolved-link.tsx` | 只读视图的说明不只说"版本"（`unresolved.readOnly`） |
| `web/apps/web/src/i18n/messages/{zh-CN,en}.ts` | 文字（前缀 `trash.`；附件类型的名称 `asset.kind.*`） |
| `web/apps/web/src/test/trash-server.ts`、`test/page-server.ts` | 假接口的回收站（`trash-server.ts`，从 `page-server.ts` 移出，审查 C 的细节）与它的开关（4.6） |

### 4.2 回收站页

- 路由 `/:slug/notebooks/:id/trash`，在笔记本的外壳里、页面的外壳之外（页面的外壳对不在树里的页答 404）。文档标题"回收站 · 笔记本名"；标题 `h1`"回收站"，其下一句说明。
- 列表：SWR 键 `["trash", nb]`，`readPages` 读回缓存里已有的页数（每页 50 项）；"更多"用 `useLoadMore`（焦点到加进来的第一项，读完时按钮离开），只接在它读的游标仍是最后一页的 `next_cursor` 时，接上时有重读在途的再整体读一次（同反链）；失败在按钮旁说明，可以再按。列表有名称；每项一行：类型图标、名称（链到预览）、原位置（"原位置：A / B"，根下写"原位置：笔记本首页"）、"X 删除 · 相对时刻"（`<time>` 的提示是完整时刻）、"可恢复至 …（`restorable_until` 的完整时刻） · 含 N 个页面、M 个附件"、恢复到哪里（"恢复到「Y」下"，原来就在根下时"恢复到笔记本首页"；"恢复到根下：原上级页面在回收站里"；"恢复到根下：放回原处会超过 10 层"）。这些信息与预览共用（`TrashFacts`）；相对时刻在渲染时算，页面开着时不走（打磨）。
- **同名的项**：已读到的项里名称与原位置都相同的（`twinsOf`，与无主笔记本的清单共用 `lib/twins.ts`），按钮与名称链接的可及名称加删除时刻，"恢复「X」（原位置：A / B，删除于 …）"；删除时刻到分钟也相同的（一分钟里删除、重建、再删除）再加 id 的末六位，"…，删除于 …，编号 000140"，行上同样写出"编号 000140"（`twinsIn`，13.2 第 17 条，审查 A-M4、B-2）。不同名的只带原位置。只在已读的项里区分：后面一页的同名，"更多"读到之后才知道（打磨）。
- 写者的按钮"恢复「X」（原位置：A / B）"：`pages.restore(id)`。它在恢复出去期间 `aria-disabled` 且 `aria-busy`，再按什么也不发；store 里同一节点的恢复一个一个来（`oneAtATimeById`，与 P1B 的还原共用一队），连点两下只发一个请求。被拒而行留着时按钮不再忙（审查 B-5）。
  - 不论答什么（200、409、422、403、404），之后都重读 `["trash", nb]` 与 `["trash-item", nb, id]`（`useTrashRestore` 的 `finally`；单项只在挂着时真的读）：不只靠事件（事件流断开、重连时，列表会留着已恢复的项）。每个 `mutate` 都带 `.catch`：登录的缓存已经没了时它拒绝，页面也随之没了（P1B 核对第三轮 R3-1）。
  - 成功：留在回收站。状态行是先空着渲染、答复之后再填字的 `<output>`（同 P1B 的页面外壳），读屏才会念出："已恢复「X」到「Y」下。 打开"（根下"已恢复「X」到笔记本首页。"；改名时"已恢复为「X 2」，在「Y」下：那里已有同名的页面或附件。"）；Y 取恢复之后重读的树里的名称（列表读到之后改过名的，照现在的）。焦点到 `h1`，`h1` 的 `aria-describedby` 指向状态行：只在读者从按下起没有动作、焦点仍在按下的控件、`body` 或 `h1`（行随恢复离开时交给它）上时（`watchReader`，13.2 第 26 条，审查 A-M2）。"打开"：页去它的页面（`arrived`）；附件去它所在的位置：父页的页面，根下的去笔记本首页，路由状态 `Located` 点名它（同 00-M8 4.12 的搜索结果，P5 照用）。附件一节的 `useLocated` 在一次读到的列表有它那一行时把焦点交给那一行；只认附件一节出现之后的读（缓存里从前的列表可能还没有它），这样的一次读里没有它（在后面的页上）时交给这一节的标题；读者其间有动作、或焦点已不在 `body` 时都不动（照 `useArriving`，审查 B-1、B-7）。
  - 拒绝的说明在列表之上（`Alert`）：409 `page.not_in_trash`、`page.trash_changed`："「X」在这期间变了：回收站已按它现在的样子显示。"；`page.restore_expired`：已过可以恢复的期限；`page.not_trash_root`：它已随上面的页面再次删除、并入了那一项，请恢复那个页面（P2 里就会有：别人恢复了它、之后又删除它的上级，审查 A 的实验；原写的"P2 里不会有"不对）；422：一句覆盖每一种界线（页面、附件、正文、链接与历史版本：422 不说越过的是哪一个），带页数与附件数（审查 A-M3、修复核对 N3；P4 改为问是否在后台进行）；503：照现有的忙的说明。
  - 403（这期间被降为读者）、404（笔记本没了或看不到了）：`errorText`，并重读 `["notebooks", ws]`（同无主笔记本页的 `refused()`）。
  - 行带着焦点（名称的链接或"恢复"）离开时（恢复了、重读拿走了、降为读者），焦点交给 `h1`（`useFocusLeaving`）；行留着时焦点留在"恢复"上。
- 读者只看列表与预览。

### 4.3 预览

- 路由 `/:slug/notebooks/:id/trash/:nodeId`；地址里的节点 id 不是 UUID 时显示 `NotFoundPage`，不读（审查 A-N1，同版本页的版本号）。SWR 键 `["trash-item", nb, nodeId]`。文档标题"X · 回收站 · 笔记本名"；面包屑"笔记本名 / 回收站 / X"（`BreadcrumbTrail`）。读到的单项（200）的 `notebook_id` 与地址里的笔记本不同时答 `NotFoundPage`，之后的读答什么都不改变它（别的笔记本的项不在这本的外壳里显示，审查 A-N3）。
- 标题 `h1`（类型图标与名称）、与列表的行相同的信息（`TrashFacts`）、写者的"恢复"（名称只带原位置：页上只有这一项）。
- 页：一句说明"这是回收站里的页，链接按今天的树解析，链到同一次删除的页要恢复之后才回来"（3.7），下面是 `VersionView`（`getPageVersionView(nodeId, item.revision)`，键 `["page-version-view", nb, nodeId, revision]`，P1B 的重读规则照用：`tree: true` 时重读；`links` 事件不会点到回收站里的页，它没有索引）：只读，不能勾任务，未解析的链接只说明"这是只读的视图"。离开预览时 `useLeftBehind` 把这一页的视图从缓存去掉（几十个预览不把各自的 HTML 留在内存里、也不在每次 `tree: true` 时重读）。
- 附件：名称、类型（`assetKind` 的文字；标题旁是它的图标）、大小（`formatBytes`），"恢复之后可以下载"；不签发地址。
- 页的 `revision` 为空、附件的 `asset` 为空（期限之内不会有：正文或行已被清理）：显示"无法预览"，不去读视图。
- 写者的"恢复"：流程与答复同列表（`useTrashRestore`）。结果由 `PreviewShell` 持有（在途、已恢复、被拒）：它随地址里的节点挂载，`Preview` 与说明页都在它之下。恢复在途时它不理会单项的读答的错误，留着显示的那一项（SWR 在读失败时留着数据）：恢复自己的 `pages` 事件（`tree: true`）与答复几乎同时到达，立即重读挂着的单项，答 409 `page.not_in_trash` 比 store 在恢复之后对整棵树的读先到；不这样，预览会换成"它已不在回收站里"，焦点掉到 `body`，回不到回收站（审查 A-I1、C-I1：对着真的服务端六次都如此）。成功时 `PreviewShell` 自己 `<Navigate>` 到回收站（路由状态 `arrived` 与 `restored`），不论单项读到什么；回收站说出状态行，焦点在它的 `h1`。用户其间离开了的（`PreviewShell` 已卸载）不再跳转。单项照 4.2 在每次答复之后重读。
- 被拒：重读之后单项还在（422、503、项还在的 `page.trash_changed`）时，原因说在视图之上（`Alert`，描述 `h1`），视图留着，焦点留在"恢复"；单项随之不在（`not_in_trash`、`restore_expired`）时换成下面的说明页，原因同样说出、描述说明页的 `h1`（只说码相同的拒绝，不说之前别的原因的）；被换下的 `Preview` 带着焦点离开时（`useFocusLeaving` 记下），说明页的 `h1` 在焦点落到 `body` 时取得它（审查 A-M1）。原因的 `Alert` 由 `PreviewShell` 渲染一次，换成说明页时同一个元素留在文档里，只播报一次（修复核对 N2）；再按"恢复"时清掉。
- `page.not_trash_root`（被拒之后、事件的重读，或直接打开组员的地址）：`replace` 到 `root_id` 的预览，路由状态 `merged`；那一项的预览在 `Alert` 里说"你打开的内容已随它上面的页面再次删除，现在是这一项的一部分，随这一项一起恢复"，描述它的 `h1`。跳转不带 `arrived`（别人的改动也会引起它，焦点在别处时不动），预览挂载时焦点在 `body` 才给它的 `h1`（修复核对 N1）。
- `page.not_in_trash`：说明页（`h1`"回收站"，文档标题"回收站 · 笔记本名"）说"它已不在回收站里"，树里有它时给出"打开它"（页去它的页面，附件去它所在的位置，同"打开"），另有"回到回收站"；说明页挂着时读一次树，这本笔记本的树里没有它（别的笔记本的节点）时答 `NotFoundPage`，树读不到时不判断（审查 A-N3）。`page.restore_expired`：同样的说明页，说已过期限，给回收站的链接（别的笔记本的、过了期限的项，直接打开、从没读到时也这样说：409 不带笔记本）。404：`NotFoundPage`。直接打开的说明页不取焦点。

### 4.4 页面树

- `removed` 改为 `Map<string, {to: string | null; after: number}>`：`remove` 在删除的答复之后记下 `after = readsStarted`；`read()` 保留一次读（`readKept`）时，`read > after` 且这次读把它列为活着的页的条目去掉（与替换 `nodes` 同一个 `runInAction`，`sameNodes` 时也做）。同步恢复、别的标签页与别人的恢复、P4 的任务、P3 的撤销都是"之后的一次读列出它"，一条规则覆盖；删除自己的那次读（`read = after + 1`）就算（审查 C-M1 的测试）；早于答复开始的读去不掉它（它被 `changesAnswered` 丢弃或 `read ≤ after`）。`removedTo` 答 `entry?.to`。
- `restore(id)`：`this.restores(id, () => this.write(() => this.service.restoreNode(id)))`：同一节点一个一个来，再经树的写入队列（13.2 第 1 条）；422 与 409 不记什么。它在恢复之后的那次树的读答复之后才结束；那次读失败而页仍在 `removed` 里时再读一次，不然"打开"会去被删子树的父页（审查 C 的细节 1；树一直读不到时照旧）。

### 4.5 删除确认框与左栏的入口

- 页面与附件两个确认框的正文改为"移到回收站，{period}内可以恢复；能读这本笔记本的人在回收站里仍看得到它。"（附件另加"嵌入它的页面会显示它缺失"）；子页面与附件数目的两句不变（PG4、AS4 的断言照旧）。`{period}` 来自 `InstanceInfo.trash_retention_seconds`：`formatPeriod` 满一天写整天数、不满写整小时数，都向下取整（不把期限说长；默认写"59 天"）；信息未读到时用不带期限的一句（"移到回收站，在那里可以恢复；…"）。页面的确认框不在每一行读 `InstanceInfo`：`PageTree` 读一次（`useSWR("instance")`），经 `TreeContext` 的 `period` 交给各行的菜单；附件一节已经读它。
- 左栏：`NotebookLayout` 把页面树与 `NotebookPlaces` 放进同一个按笔记本挂载的门户，页面树之后一个单独的 `<nav aria-label="{notebook}的更多">`（英文 "More of {notebook}"），`NavItem`"回收站"带 `end`：只在回收站页上是当前的，预览上不是（预览的面包屑把它放在回收站之下，审查 A-N2）。不放进页面树的 `nav`：vitest 的帮助函数与 e2e 的 `treeTitles()` 把那里的每个链接当作页面。

### 4.6 事件、列表的模式与测试

- 事件：`pages` 的 `tree: true` 经 `refresher` 重读 `["trash", nb]` 与这本笔记本的 `["trash-item", nb, *]`；`refreshedOnConnect` 的最内一层加 `trash`、`trash-item`。
- `readPages` 新放在 `lib/read-pages.ts`，只给回收站的列表用；13.2 第 19 条按 4.13 改写（"按已读的页数重读"）。
- 假接口（`test/trash-server.ts`）只答网页到得了的，码照服务端的次序：404 → 403 → 409 `page.not_in_trash`、`page.not_trash_root`（带 `root_id`）、`page.restore_expired` → 422 的界线（节点、历史，多于一页时页数与字节）→ 409 `page.trash_changed`（`purged`）→ 503 → 422 的链接；放置按现在的树（活着的与回收站里的页）算，码相争的两处各有 `test.each`。测试用的开关：`restoresHeld`、`restoresBusy`、`trashWaits`（按住列表与单项的读）、`trashDown`（读不到，答 500）、`restoreElsewhere`、`trashNode`（答可改的一项，测试设它的 `deletedAt`），每项的 `expired`、`purged`、`history`、`links`。
- **vitest**：`readPages`；`PageTreeStore`：`removed` 被之后的读去掉、不被之前的读去掉、恢复经队列、同一节点的第二次恢复排在第一次之后、422 不记；回收站页：行的文字与名称、放置的四种、同名的项的可及名称、"更多"、恢复的每种答复（200、409 四种、422、503、403、404）与焦点、每种答复之后列表重读（事件流断开时）、连点两下只发一个请求、恢复的附件的"打开"去它所在的位置、读者没有按钮、事件与连上时的重读；预览：页的阅读视图以单项的版本号读、离开时视图的键从缓存去掉、附件的信息、"无法预览"、`notebook_id` 不符时的 `NotFoundPage`、`not_trash_root` 的跳转、`not_in_trash`、期限、预览里的恢复回到回收站并带状态行与焦点；删除确认框的文字与期限、未读到时的文字；`formatPeriod`；左栏的入口不在树的 `nav` 里；经组合根到达。审查之后另加：从预览恢复时恢复自己的事件先重读单项（事件流连着、树的读按住，A-I1）；预览的拒绝让单项不在时的原因与焦点、`not_trash_root` 的跳转与说明、别人的改动引起的跳转不拿焦点（A-M1，修复核对 N1、N2）；读者动了不抢焦点（回收站页与 `useLocated`，A-M2、B-1）；同一分钟的同名与同一分钟里别的名称（A-M4、B-2、修复核对）；同层不同位置的同名（B-6）；"重试"（B-3）；"更多"的竞争、失败与 101 项（B-4）；按钮不再忙（B-5）；交给标题（B-7）；预览的拒绝在再按时清掉、说明页的文档标题、404 的焦点（B-9）；删除自己的那次读（C-M1）；恢复之后树的读失败（C 的细节 1）；地址不是 UUID、别的笔记本的项（A-N1、A-N3）；`end`（A-N2）；`twinsOf`、`BreadcrumbTrail`。
- 删除确认框的旧断言随文字改：`attachments-section.test.tsx:634` 的整句。

### 4.7 e2e

- **TB1**（`e2e/stories/trash/tb1-trash.spec.ts`；回收站是自己的一块，P4 的 TB2 也放这里；接口版本、页面版本与预览版本都在 B 里做，照 P1 的 HI1、HI2）：
  - 接口版本（TB1 (API)）：PAT 建页（含链接）与附件，删除子树；数据库断言 `expectTrashRoot`（只有根有 `trash_root` 与统计：页数 2、附件 1、层数 2；组的其余节点没有，个数等于统计）与 `expectAssetsDeletedWithNodes`；`listTrash` 恰列出这一项（`restorable_until − deleted_at` 是 5,180,400 秒），`getTrashItem` 相同，组员答 409 `page.not_trash_root` 带 `root_id`；以单项的 `revision` 读视图（`getVersionView`），链接解析到活着的页；恢复：`pages` 事件带 `tree: true` 与两页的版本号，`links` 事件到达；数据库断言 `expectSubtreeRestored`（放置与名称；节点活着、没有标记、由恢复者在变更集的时刻更新；正文、版本与条目活着；一个 `trash_restore` 变更集，每个节点一项恢复的条目）、`expectAssetsRestored`（行活着、文件在）、`expectIndexedLinks`；`getAsset` 新签的地址下载 200；先删子页面、再删它的上级、恢复子页面 → 根下（`parent_in_trash`）；原位置新建同名的页之后恢复 → "Notes 2"；带附件的一组（"Stale"，审查 B-8）以 `trashedAgo` 移到期限与清理的截止之间（删除于 `now − purge_retention + 30 分钟`：已过 `restorable_until`，又在清理的截止之后，e2e 每 2 秒一次的清理碰不到它），组仍完整（节点、正文、版本、条目、附件的行）→ 列表不列，单项与恢复都答 409 `page.restore_expired`，节点仍已删。
  - 页面版本（TB1 (page)）：打开要删的页，从树的菜单删除（确认框写明回收站、59 天、子页面与附件的数目），外壳去它的上级；左栏的"{notebook}的更多"进回收站（标题、文档标题、行的文字）；打开预览，文章里的链接点到活着的页、返回，面包屑回到回收站；从列表恢复，状态行与"打开"，`h1` 取得焦点，列表空了；同一个标签页里树中看得到它（`treeTitles` 轮询），数据库断言；"打开"去它的页面，标题取得焦点。
  - 预览版本（TB1 (preview)，审查 A-I1）：事件流以已有的 `holdStream` 按住到预览显示（连上时的重读读过单项，`refresher` 还没读过），在预览里恢复：地址回到回收站，状态行，`h1` 取得焦点，列表空了；`MutationObserver` 断言"它已不在回收站里"从没出现；数据库断言 `expectSubtreeRestored`；单项重读答的 409 按答复的个数声明（`failedToLoad(409)`）。
- 夹具：`e2e/fixtures/trash.ts`（`listTrash`、`getTrashItem`、`restoreNode`、`trashPath`、`trashListed`、`restoreWith`、`trashedAgo`：以一组的删除时刻 T 为准整体移动，节点、正文、版本、条目、附件的行各 `deleted_at = T − interval WHERE deleted_at = T …`，一条带数据修改的 CTE 的语句，不用每条语句的 `now()`，同 3.10）；`assert/page.ts` 的 `expectTrashRoot(db, rootId, {pages, assets, height})` 与 `expectSubtreeRestored(db, rootId, ids, restorerId, {parentId, name})`；`assert/asset.ts` 的 `expectAssetsRestored(db, storageDir, assets)`；`fixtures/pages.ts` 的 `getVersionView`。

### 4.8 实施时定下的与审查带来的

三位审查者（A：回收站页、预览、数据流与无障碍；B：测试、假接口、e2e 与设计的符合；C：跨模块的接线与代码质量）与修复核对的处置见 `reviews/P2B-trash-web-review.md`。与上文最初的设计不同之处已改在各节里：

- **实施时定下的**（设计没写或留了余地；实施者的偏离与审查 B 列出的"不在偏离里"的，A、B 判过）：假接口只答网页到得了的、码照服务端的次序，放置按现在的树算（4.6）；附件的"打开"经路由状态 `Located`（`attachment-row.tsx` 的 `locatedOf`，行的链接带 `data-attachment`），定位在附件一节的 `useLocated`：只认这一节出现之后的读，读到的列表里没有它时交给这一节的标题（4.2）；列表的行与预览共用 `TrashFacts`，相对时刻在渲染时算、不走（4.2，打磨）；状态行里上级页面的名称取恢复之后重读的树（4.2）；期限经 `TreeContext` 交给各行的菜单（4.5）；说明页（`not_in_trash`、`restore_expired`）有自己的 `h1`"回收站"与文档标题"回收站 · 笔记本名"（4.3）；左栏的 `nav` 名为"{notebook}的更多"（4.5）；夹具另有 `fixtures/pages.ts` 的 `getVersionView` 与 `trash.ts` 的 `trashListed`（4.7）。实施者的偏离 1（单项只在拒绝之后重读：成功时重读会让预览先说"它已不在回收站里"）随 A-I1 的修复去掉。
- **审查与修复核对带来的**：从预览恢复的结果由 `PreviewShell` 持有，在途时留着显示的单项，成功时由它回到回收站，单项在每次答复之后重读（A-I1、C-I1，4.3）；预览的拒绝让单项不在时，说明页说出原因（只说一次）、它的 `h1` 接过离开的焦点；`not_trash_root` 跳到根的预览并说明，不带 `arrived`（A-M1，修复核对 N1、N2，4.3）；`page.not_trash_root` 在 P2 里就会有（4.2）；恢复之后的焦点与附件的定位看读者（`watchReader`，A-M2、B-1，4.2）；422 的一句覆盖每种界线（A-M3、修复核对 N3，4.2）；同一分钟删除的同名项带 id 的末六位（A-M4、B-2，4.2）；地址不是 UUID 时 404、不读，别的笔记本的项在 200 之后、在树里找不到时 404（A-N1、A-N3，4.3）；回收站的入口带 `end`（A-N2，4.5）；恢复之后树的读失败时再读一次（C 的细节 1，4.4）；`lib/twins.ts`、`BreadcrumbTrail`、`test/trash-server.ts`（C 的细节 3–5，4.1）；TB1 (preview) 与带附件的过期一组（A-I1、B-8，4.7）；测试的补足（B-3–B-7、B-9、C-M1、修复核对的三处缺口，4.6）。

## 5. 实施步骤

A（分支 `m8-p2a`）：

1. 迁移两条、schema 与补值的测试；`Change.Restored`、`OpRestore`、`Subtree.Counts`、`domain/trash.go`。
2. `RecordItems` 代替 `RecordItem`（`page/app/asset_test.go` 的调用序列、`store_test.go` 与 `purge_test.go` 的直接调用随之改）、`DeleteNodes` 的根、统计与 `stray`，适配器测试；现有的单元与整个程序上的测试照旧通过。
3. `Trash` 端口、查询与适配器、成本测试；`readableNode`；asset 的 `NewAttachmentInfos`、bootstrap 的适配、`Deps.Attachments` 与 `Deps.Retention`（`retention(cfg)`；保留期的下限在第 6 步）；`ListTrash`、`GetTrashItem`。这一步之后每一步都绿：`page.New` 对空端口的 panic 与接线同时进来。
4. `DecidedAll`、`Unit.Restore`、`RestoreNodes`、`RestoreNode` 与测试。
5. asset 的 `NodesRestored`；bootstrap 的观察者；`SourcePath` 与渲染路径的 `Deleted`。
6. 期限、保留期的下限、`InstanceInfo`；契约（`UserRef`、`TrashAttachment`、`root_id`）、处理器、`make gen`；网页的门禁与 S3。
7. 权限、矩阵、组合根；整个程序上的测试、不变式与交错；实测 `MaxUnitNodes` 并写回。
8. 反向对照、门禁、审查。

实施时的次序与上面有两处不同：`node.restore` 的规则与 `shared.Error.RootID` 在第 1 步（域的 `NotTrashRoot` 要它们，第 1 步才绿）；权限矩阵的行与观察者的扩展测试在第 6 步（路由在第 6 步进来，矩阵对没有行的路由失败）。第 8 步的反向对照另补了一个交错（3.10 的交错 5，清理停在版本与正文之间）。

B（分支 `m8-p2b`，P1B 与 A 合并之后）：

1. `readPages`；`VersionView` 与 `useLeftBehind` 移出；服务与 store（`removed`、`restore`）。
2. 回收站页、预览、左栏的入口、路由、事件。
3. 删除确认框与 `formatPeriod`；文字。
4. vitest、e2e TB1 的两个版本；反向对照、门禁、审查。

## 6. 测试与验证

- 门禁：`make lint-go lint-web knip gen-check test-go test-web build`、e2e 全量。
- 反向对照：A 的每个用例与查询（根的标记、统计、`stray`、`restored`/`revision` 的保留、恢复的五个数、期限的边界、放置的式子、序号、键的重算与组内的序号、等待合计、码的次序、观察者的行数、`SourcePath` 的不过滤已删）、B 的 `removed` 规则、列表的重读、恢复的每种答复、同一节点一个一个来，各改坏一处，测试要失败。
- 审查：每部分三位 Opus 审查者并行（A：数据与用例、契约与权限、测试与交错；B：界面与无障碍、状态与事件、测试与 e2e），之后 Opus 核对修复，直到一轮没有行为上的发现。

## 7. 完成标准

- 第 2 节的每一项做完，第 3.12、4.6、4.7 节的测试通过，门禁全绿；`MaxUnitNodes` 实测并写回。
- 审查记录：设计 `reviews/P2-design-review.md`，实施 `reviews/P2A-trash-server-review.md`、`reviews/P2B-trash-web-review.md`；00 号文档的进度表与本文档的"结果"一节更新。
- **P2 的文档提交做的改动**（随本文档一起）：
  - `00-M8-design.md`：2.1 的 13 条。
  - `v0.1-design.md`（00-M8 第 7 节"随这份设计一起改的"里与 P2 有关的，至今没有改）：3.5 树操作加"从回收站恢复（3.10）"；3.10 改写为：删除的子树以同一个时刻进回收站，回收站列出它们的根；恢复时原父节点在回收站里或放回去太深就放到根下，标题冲突加序号；可以恢复的期限是 `jobs.purge_retention` 减 1 小时（默认约 60 天），之后由清理任务物理清除；6.1 的内嵌显示名的例外加 M8 的版本（执行者）与回收站（删除者），问题的扩展成员加 `root_id`；7.2 的 `nodes` 加 `trash_root` 与统计，`changeset_items` 加 `restored`、`revision`；12.4 的观察者一行加"M8（恢复时附件的行跟着节点恢复）"。
  - v0.1 设计第 13 节的条目随实现改（A、B 各自的提交）：13.1 第 1 条（`OpRestore`、整步的条目、从上往下删）、第 4 条（三个操作的码的次序）、第 5 条（asset 的观察者恢复已删的行）、第 6 条（恢复的期限、保留期的下限）、第 8 条（`root_id`）、第 11 条（`asset.NewAttachmentInfos`）、第 15 条（保留期交给 page）、第 19 条（恢复在单元之前从大到小、等待合计）、第 21 条（最后一跳）、第 27 条（回收站的游标）、第 29 条（`OpRestore` 经编辑锁放行）、第 31 条（单元的界线），13.2 第 1、15、16、17、19、23、26 条（B），13.4 第 3、4、6、9 条。
  - M7 设计 4.6（`00-M7-design.md:189`）"M8 的恢复锁同样的行，与清理串行"一句改为"M8 的恢复与清理以一小时的间隔分开（M8 设计 4.11）"。
  - `01-P1-versions.md` 3.6 的 `ChangesetAuthor` 改为 `UserRef`（实施时）。
  - README 的"软删除与清理"一段。
  - 移交文件（`handoffs/M6-links.md`、`M7-assets-transfer.md`）不改：它们记的是当时移交的内容，落实记在 00-M8 的"移交的落实"表里（2.1 第 7 条）。

## 8. 交给后面的 Phase

- **P3**：撤销一次恢复用条目的 `restored`、`revision`；撤销里的恢复（C 删除的节点回来）同样设 `Change.Restored` 并清掉根的标记与统计；撤销里的删除经 `DeleteNodes` 记根与统计，**从上往下删**：一个单元里每棵要删的子树删一次、从它的最上层删，不先删子孙再删祖先（`DeleteNodes` 的 `stray` 会拒绝）；`DecidedAll` 与单元的界线共用（`DecidedAll(ctx, contents, tooLarge)` 的第三个参数是超过链接数时答的码，撤销传 `page.revert_too_large`；P2A 加的 `MaxUnitHistory` 量的是写回的版本与条目，撤销里恢复 C 删除的节点时同样要数）；`RecordItems` 一次只写一个变更集、一个时刻的条目，合并时保留第一次的 `restored`/`revision`；清理截掉的标记、空变更集的清理照 4.2（P2 没有标记被清理拿走的条目，3.5）。网页：左栏的 `NotebookPlaces` 加"历史"；撤销里恢复的页由 `removed` 的规则从这一代删除的页里去掉（之后的读列出它，4.4）。
- **P4**：分批恢复复用 `PlacementOf`、`children.free`、`RestoreNodes`（ids 可以是组的一部分，根的参数给这一批的根；五个数按这一批的期望核对）；途中停下时，新标上的根要写统计（任务读过组，手里有）；撤销的分批删除用同一时刻时，上层的根之下早先标上的根取消、统计重算（跨单元，`stray` 只管一个单元）；网页的 422 改为问是否在后台进行。分批除了节点、页数、字节与链接，还要按历史的行（每批的版本与条目至多 `MaxUnitHistory`）：历史的 422 同样转到后台；一页自己的历史超过 40 万行（约 20 万个版本）时按节点分不开，要按历史的行分批，否则它一直恢复不了，而删除没有界线（修复核对第一轮的合并注记）。
- **P5**：搜索结果的附件信息用 `AttachmentInfos`；附件的"打开去它所在的位置"沿用 B 的路由状态（`attachment-row.tsx` 的 `Located`、附件一节的 `useLocated`）。
- **M12**：删除没有界线：导入的上限 5 万个节点 1.8–2.1 秒，没有超过请求期限（3.11）；它也没有历史的界线，同一大项反复删除与恢复让历史增长到 422，缓存的语句的通用计划是一类风险，写进 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 14 项；反链、版本列表与回收站的三份"按已读的页数重读"收拢（v0.1 之后的打磨）；B 的其余打磨写进 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)第 22 条。

## 9. 结果

### 9.1 A：服务端（2026-10-14，合并 `25d250a`）

- 提交：实施：迁移与领域 `9710ff2`，整步的条目与根的标记 `3c97655`，回收站的读 `9cda81d`，恢复的单元 `c3bb0b6`，附件的行与回收站里的页的链接 `08a1d7f`，契约、期限、`UserRef` 与 `root_id` `e2907f5`，整个程序与 `MaxUnitNodes` 的实测 `99dca8a`，清理停在版本与正文之间的交错 `a55095a`（审查时在 `e178c64` 上，是 `0440e2f..16de05b`，之后变基到 `main` 的 `ce63b00`，唯一的冲突是 `page.service.ts`）；CI 的修补 `463d9e9`（镜像的冒烟期望 `trash_retention_seconds`）；审查的修复 `27fee6b`；修复核对第一轮的修复 `4bf696d`，第二轮的 `82351ad`。合并 `25d250a`。
- 实测（3.11）：`MaxUnitNodes = 10,000`（满负载的单元在 10,100 个节点处至多 2.7 秒）；`MaxUnitHistory = 400,000`（419,900 行时单元 2.9 秒无统计、4.3 秒活着时 ANALYZE）；导入上限的删除 1.8 / 2.1 秒。测量中改掉三处每行读一遍别的行的计划：`RestoreNodes` 的写法、M6 的 `SetResolutions`、`TrashHistory`（与 `RestoreBlobsOfNodes`）按参数规划。
- 审查：三位审查者（Opus），没有 Important。中低 1 条：历史的行没有界线，每页几千个版本的一组在请求里超时或答 500（A-M4，加了 `MaxUnitHistory`）。其余是低（恢复的变更带的状态、两个父节点下的同名、只被拿走当前的版本、节点的界线经组合根，这些测试的缺口；e2e 全量没跑；第 7 节的文档）与细节（两次读之间的竞态答 500、契约的措辞与码的次序、唯一索引的冲突答契约之外的码），另有 CI 的镜像冒烟。修复核对：第一轮中高 1 条（`TrashHistory` 作为缓存的语句，第 6 次起的通用计划在界线处 5 秒，I-1）、低 1 条（生产的 `MaxUnitHistory` 接线没有测试钉住）与细节 2 条；第二轮没有行为上的发现（测试与注释的几处）。逐条见[审查记录](reviews/P2A-trash-server-review.md)。
- 与设计不同之处已改在第 3 节（3.1、3.3–3.12），3.13 分实施时定下的与审查带来的列出；第 5 节的次序。
- 反向对照：实施之后 94 个（90 个被抓到，存活的 4 个与代码等价）；审查者 49 个（A 14、B 8、C 27，存活的即各条测试的缺口或等价）；审查的修复之后 32 个、修复核对第一轮的核对者 18 个（存活 2 个：M-1 与一个不算发现的错误分支）、它的修复之后 6 个，修复之后的都被抓到。
- 门禁：本机 `27fee6b` 上 `make lint-go gen-check test-go`、`make lint-web knip test-web build-web`（2,673 个测试）、整个 e2e 234 个、`make image-smoke` 通过；`4bf696d` 上 `make lint-go gen-check test-go` 通过。CI：`16de05b` 只有镜像的冒烟失败，`27fee6b` 四个 job 全绿。
- 交给后面的：B（`m8-p2b`）以这份契约为准，422 也可能来自历史（4.2）；P3 的 `DecidedAll` 与 `RecordItems`、P4 按历史的行分批（第 8 节）；M12 的性能移交第 14 项（删除的历史、反复删除与恢复、通用计划、界线测试在 CI 上的余量）。

### 9.2 B：前端（2026-10-14，合并 `5116194`）

- 提交：实施：回收站的列表与恢复、预览、页面树的 `removed`、删除的文字 `537365f`，TB1 `2ec1ec1`，反向对照第一轮漏掉的测试 `ef85768`，登出之后才答复的恢复不留未处理的拒绝 `7048cef`（审查时在 `m8-p2a` 的 `27fee6b` 之上，是 `84ac133`、`d7c6eed`、`e4824d3`、`79d7311`，之后变基到 `main` 的 `613adfc`，没有冲突）；审查的修复 `4daf892`；修复核对第一轮的修复 `7d5dfc2`，第二轮的测试 `13462f9`。合并 `5116194`。
- 审查：三位审查者（Opus）。高 1 条（A 与 C 各自发现）：从预览恢复时，恢复自己的事件先重读单项、答它已不在，预览停在"它已不在回收站里"，焦点落到 `body`，回不到回收站、没有状态行，对着真的服务端六次都如此（A-I1、C-I1）。中低 1 条：预览的恢复被拒、单项随之不在时原因不说、焦点落到 `body`（A-M1）。其余是低（恢复之后与附件定位的焦点不看读者；422 的文字对历史的界线自相矛盾；同一分钟删除的同名项名称相同；"重试"、"更多"的竞争与失败、`removed` 在删除自己的那次读上的边界没有测试；第 4 节没写的做法）与细节。修复核对：第一轮低 1 条（A-M1 的修复带进的回归：别人的改动引起的 `not_trash_root` 跳转也拿走焦点，N1）、细节 3 条（拒绝说两次、422 只列三种界线、后退再前进的预览不知道在途的恢复）与三处测试的缺口；第二轮没有行为上的发现，只有两处测试的缺口（只在合并之后才给标题焦点、合并的预览上的拒绝），补上（`13462f9`）。逐条见[审查记录](reviews/P2B-trash-web-review.md)。
- 与设计不同之处已改在第 4 节（4.1–4.7），4.8 分实施时定下的与审查带来的列出；4.2 更正了"`page.not_trash_root` 在 P2 里不会有"（别人恢复了它、之后又删除它的上级时就有）。
- 反向对照：实施之后 112 个 vitest 变异（第一轮 13 个存活，补了测试之后都被抓到）与 3 个 e2e 变异，都被抓到；审查者 B 35 个与 e2e 1 个、C 13 个（存活的即各条测试的缺口或等价），A 以复现代替；审查的修复之后第一轮的 107 个回跑、新的 47 个与 e2e 2 个，都被抓到（去掉 `reader.end()` 的一个先接受，第二轮由 `removeEventListener` 的测试抓到）；修复核对第一轮核对者 22 个（存活 8 个：三处测试的缺口在 `7d5dfc2` 补上，五个等价或防御）。
- 门禁（本机）：`make lint-web knip test-web build-web`（`4daf892`，153 个文件、2,813 个测试）与全仓库的格式检查；整个 e2e 237 个通过（`4daf892`）；TB1 连跑三次通过。这一阶段没有服务端与契约的改动。
- 交给后面的：P3：左栏的 `NotebookPlaces` 加"历史"，撤销里恢复的页由 `removed` 的规则覆盖；P4：422 改为问是否在后台进行；P5：附件的"打开"沿用 `Located` 与附件一节的 `useLocated`；打磨的十余条见审查记录，已写进 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)第 22 条。
- 负责人可以改判的取舍：从预览恢复之后回收站压进历史（返回落在"它已不在回收站里"，`replace` 可以跳过）；状态行随历史在刷新、返回时再说一次；同名的项只在已读的项里区分；回收站页开着时别处的删除只经事件进列表（A 细节 N4）；后退再前进的预览不知道在途的恢复（修复核对第一轮 N4）。

## 附：调研列出的 38 项的处置

| # | 项 | 处置 |
|---|---|---|
| 1 | `asset.NewMeta` 不存在 | `asset.NewAttachmentInfos(pool)`、page 的端口、bootstrap 适配、`Deps.Attachments`（3.4） |
| 2 | `AssetMeta` 撞名 | 改名 `AttachmentInfos`，修订 2.1 第 2 条 |
| 3 | 没有 "UnitObserver" | 用现有的 `PageObserver`；asset 的 `PageObserver` 加 `NodesRestored`（3.6） |
| 4 | `getInstanceInfo` | 是 `getInstance`，修订第 12 条 |
| 5 | `boot/actions_test.go` | 是 `bootstrap/actions_test.go`，修订第 11 条 |
| 6 | 恢复的 `TakeNow` 从未有过 | `Take`，从大到小，等待合计有界（3.5 第 5 步），修订第 7 条 |
| 7 | "100 页"与导入的 100 个节点、独自超过的一页 | 100 是页，附件计入树的操作；只有一页的组不受三项限制（3.11），修订第 8 条 |
| 8 | "父节点不在"不会发生 | 放置简化（3.4 的 `PlacementOf`），修订第 1 条 |
| 9 | `readableAny` 只认页面 | `readableNode` 不看类型（3.5） |
| 10 | P1B 不在分支上 | B 在 P1B 合并之后，复用并移出它的 `VersionView` 与 `useLeftBehind`（4.3） |
| 11 | 过时的文档 | 第 7 节的文档提交；`reviews/P1A-…` 已随 P1A 合并；设置的标签名归 P4 |
| 12 | `useNotice`、`readPages` 不是共用的 | 状态行用 `<output>`（同无主笔记本的接管），不造共用的 `useNotice`；`readPages` 新放在 `lib/`，只给回收站用（4.6） |
| 13 | 没有 `Change.Restored`、`OpRestore`，`Then` 会丢 | 加上，`Then` 取第一项的（3.6） |
| 14 | 观察者看不到删除时刻 | `Restored.DeletedAt`（3.6），修订第 3 条 |
| 15 | 观察者看不到类型 | `Restored.Asset`（3.6） |
| 16 | 防线的码 | 都是 409 `page.trash_changed`；`RestoreNodes` 另核对版本链；防线之外的两种写明（3.5 第 6 步、3.6） |
| 17 | `root_id` 是新的扩展成员 | `shared.Error`、`httpserver`、`common.yaml`、网页，与平台的测试（3.9） |
| 18 | 条目一个节点一条 | `RecordItems` 整步一条，所有操作共用；冲突时保留 `restored`/`revision`（3.3） |
| 19 | `DeleteNodes` 标不了根 | 交给它根与统计；`stray` 拒绝先删子孙再删祖先（3.3） |
| 20 | 补值与笔记本删除不一致 | 补值以同一时刻删除的变更集认出随笔记本删除的，不读 `notebooks`（3.2），修订第 5 条 |
| 21 | 回收站里的页的链接 | `Deleted` 经三个结构、`SourcePath`、`resolutions` 的 `known`；解析的含义写明并钉住；P1 的测试翻转（3.7） |
| 22 | 回收站里的页的属性 | 预览不显示（3.7） |
| 23 | 预览要版本号；"附件的信息"所指 | 单项带 `revision`、`notebook_id`；预览只显示根（3.9、4.3），修订第 6 条 |
| 24 | 恢复的 `pages` 事件带版本号 | 接受（3.6） |
| 25 | 重新编号之后的次序值 | 照原值，写明（3.5） |
| 26 | `free` 重核对名称 | 必然通过，不进码（3.5） |
| 27 | 解析的等待没有合计的界 | 合计至多 `parse_max_wait`，503（3.5） |
| 28 | 列表每项的读的代价 | 统计删除时记下；原位置一页一条语句；成本测试（3.2、3.4） |
| 29 | 保留期与间隔的去处 | `retention(cfg)` 给三处、`domain.TrashMargin`、bootstrap 算 `trash_retention_seconds`；`page.New` 的两个测试夹具随之改（3.8） |
| 30 | `PurgeConfig.Clock` 到不了测试 | 不加，测试给截止时刻（3.8），修订第 9 条 |
| 31 | 保留期的下限 | 1 天，改动的文件（3.8） |
| 32 | `trash_retention_seconds` 的固定值、按天写 | 改动的文件（3.8）；`formatPeriod`（4.5） |
| 33 | 契约与删除文字的措辞 | `updated_at` 的描述（3.9）；删除确认框（4.5） |
| 34 | 不变式 | `checkPages` 两条、`checkAssets` 一条（3.12） |
| 35 | 权限矩阵的种子 | `matrixTrash()`；先建后删；`gone-nb` 的删除循环只动未删的行（3.10） |
| 36 | 网页的状态与界面 | `removed` 的规则、`restore`、单独的 `nav`、同名的项、新键与连上时的重读、预览的附件行（第 4 节） |
| 37 | e2e | 夹具与断言、`stories/trash/`、期限的一组放在间隔里（4.7） |
| 38 | 活着的笔记本里的变更集从不清理 | P3 的事（4.2 的空变更集清理），P2 不改 |
