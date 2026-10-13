# M8/P2 回收站：架构设计 & 实施规划

## 0. 分两部分

照 P1 的做法分成 A（服务端：迁移、删除的标记与整组的条目、回收站的读、恢复、观察者、回收站里的页的链接、期限、契约、权限）与 B（前端：回收站页、预览、页面树的 `removed`、删除确认框、左栏的入口；e2e TB1），各自一个分支、一次审查与修复核对、一次合并。B 以 A 合并之后的契约为准，并在 P1B 合并之后开始：预览复用 P1B 的只读阅读视图（`getPageVersionView`，版本号取回收站单项答复里的 `revision`）。设计审查的处置见 `reviews/P2-design-review.md`。

## 1. 基线

- A 从 `main` 的 `4891f87` 开始（P1A 已合并，`12fc5d7`）；B 从 P1B 合并之后的 `main` 开始。
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
| `page/domain/trash.go` | `TrashRoot`、`TrashMember`、`TrashCursor`、`TrashFrom`、`Placement`、`PlacementOf`、`TrashMargin`（1 小时）、`RestorableUntil`、`Restorable`；单元的界线 `MaxUnitNodes`、`MaxUnitPages`、`MaxUnitBytes`、`MaxUnitLinks` |
| `page/domain/errors.go` | `ErrNotInTrash`、`NotTrashRoot(rootID)`、`ErrRestoreExpired`、`ErrRestoreTooLarge`、`ErrTrashChanged` |
| `page/domain/actions.go` | `ActionRestore = "node.restore"` |
| `page/app/ports.go` | `Trash` 端口；`NodeWriter.DeleteNodes(ctx, Deletion)`、`RestoreNodes`；`ChangesetWriter.RecordItems`（代替 `RecordItem`）；`AttachmentInfos`；`AnyNode.TrashRoot`；`PageRef.Deleted` |
| `page/app/unit.go` | `apply` 一步的条目一次写（`recordItems`，没有要记的就不写） |
| `page/app/unit_delete.go` | 根与统计交给 `DeleteNodes` |
| `page/app/unit_restore.go` | `Unit.Restore` |
| `page/app/readable.go` | `readableNode`（不看类型）；`readableAny` 在它之上要求页面 |
| `page/app/content.go` | `ContentParser.DecidedAll`（从大到小、等待合计有界） |
| `page/app/trash.go`、`list_trash.go`、`get_trash_item.go`、`restore_node.go` | 共用的读与放置；三个用例 |
| `page/app/get_version_view.go` | 回收站里的页设 `PageRef.Deleted` |
| `page/adapter/postgres/queries/trash.sql`、`trash.go` | `Trash` 端口的查询与实现 |
| `page/adapter/postgres/queries/nodes.sql`、`changesets.sql`、`links.sql` | `DeleteNodes`、`RestoreNodes`、`RecordItems`、`SourcePath` |
| `page/adapter/markdown/markdown.go` | `PageRef.Deleted` → `markdown.Page.Deleted` |
| `page/link_targets.go` | `SourcePath` |
| `page/adapter/http/trash.go`、`handler.go`、`versions.go` | 三个处理器；`UserRef` |
| `page/module.go` | `Deps.Retention`、`Deps.Attachments`、`Deps.MaxUnitNodes`（0 即 `domain.MaxUnitNodes`，只有实测抬高它）；导出 `ErrTrashChanged`、`TrashMargin`、`Restored` |
| `platform/markdown/markdown.go`、`budget.go` | `Page.Deleted`；`Budget.Wait()` |
| `linking/app/ports.go`、`views.go`、`resolver.go`；`linking/adapter/markdown/views.go` | `Page.Deleted`；`Pages.SourcePath`；`resolutions` 接受已读到路径的来源 |
| `asset/lifecycle.go`、`asset/app/follow.go`、`asset/app/ports.go`、`asset/adapter/postgres/queries/lifecycle.sql` | `NodesRestored`、`RestoreBlobsOfNodes` |
| `asset/attachment_infos.go`、`asset/adapter/postgres/queries/infos.sql` | `NewAttachmentInfos(pool)`（只凭连接池） |
| `instance/domain/info.go`、`instance/app/get_info.go`、`instance/module.go`、`instance/adapter/http/handler.go` | `TrashRetention` |
| `transfer/adapter/http/server.go` | `UserRef` |
| `shared/error.go`、`platform/httpserver/apierrors.go`、`problem.go` | 扩展成员 `root_id` |
| `platform/config/validate.go`、`server/configs/config.yaml`、`README.md` | 保留期的下限 1 天 |
| `access/domain/rules.go` | `node.restore` |
| `api/common.yaml`、`api/modules/page.yaml`、`instance.yaml`、`transfer.yaml` | 三个操作、`UserRef`、`TrashAttachment`、`root_id`、`trash_retention_seconds` |
| `bootstrap/deps.go`、`assets.go`、`linking.go` | `retention(cfg)`、`Attachments`、观察者的恢复、`SourcePath`、`trash_retention_seconds` |
| `web/apps/web/src/app/problem-messages.ts`、`i18n/messages/{en,zh-CN}.ts`、`test/fakes.ts`、`services/page.service.ts`；`e2e/stories/smoke/s3-instance-info.spec.ts` | 契约带来的门禁：五个新码的文字（`problem-messages.test.ts` 读契约里每个操作的码，P1A 的 `23be652` 为 `page.version_not_found` 做过同样的事）；`instanceJSON` 的新字段；`EditLockHolder` 改为 `UserRef` 的别名；S3 对 `InstanceInfo` 的整体比较 |

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

  空值照 `SetResolutions` 的写法用零 id 与 `has_*` 标出（根下的父节点也是空，所以"有没有前"要单独一列）。`restored`、`revision` 不在 `DO UPDATE` 里：合并时保留第一次的。适配器从 `Change` 取：`restored = Restored != nil`，`revision = Revision`（只在"前"空、"后"不空时，新建的页是 1）。

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
  SELECT root_id, id, name, deleted_at, up::integer AS up FROM chain ORDER BY root_id, up DESC;
  ```

  原位置就是这些名称。`domain.PlacementOf(line, height)`（列表、单项与恢复共用）：线为空（原来在根下）→ `original`，`parent_id` 为空；父节点（`up = 1`）活着且 `len(line) + height ≤ domain.MaxDepth`（`Depth(line) + h - 1 ≤ 10` 的同一个式子）→ `original`；父节点已删 → 根下，`parent_in_trash`；否则根下，`too_deep`。
- `TrashRootOf`：从节点往上，直到 `trash_root`（非根的已删节点，父节点一定是同一时刻删除的）；列名都带表的别名（sqlc 对 `id = sqlc.arg(id)` 报歧义）：

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

- `TrashGroup`：根（`FindTrashRoot` 的行）之后按层读，每层一条语句、以它的父节点规划（`postgres.Planned`，同 `Subtree`）：

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
- `TrashContents`：`SELECT node_id, content FROM page_contents WHERE node_id = ANY($1) AND deleted_at = $2`。
- `Nodes.FindAnyNode` 多读 `trash_root`（`AnyNode.TrashRoot`）。时刻的比较一律 `time.Time.Equal`（pgx 答的时刻在本地时区，`==` 连时区也比）。

**附件的信息**（4.1，P5 的搜索也用）：asset 的根加只凭连接池的 `asset.NewAttachmentInfos(pool) AttachmentInfos`，`Of(ctx, nodeIDs) (map[uuid.UUID]AttachmentInfo, error)`，`AttachmentInfo{MIME string; Bytes int64}`：

```sql
-- name: AttachmentInfos :many
SELECT node_id, mime, byte_size FROM asset_blobs WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]);
```

不按 `deleted_at` 过滤（回收站里的附件要读）；在调用者的读里、走 `asset_blobs_node_id_key`。page 的端口 `app.AttachmentInfos`，bootstrap 的 `attachmentInfos` 适配，`page.Deps.Attachments`；`page.New` 对空的端口 panic（组合根交空即接线的错，同 `markdownadapter.NewBudget`）。不叫 `AssetMeta`：`page.AssetMeta` 已是守卫看到的新附件的文件（含 SHA-256），两者用途不同。

计划：`ListTrash`、`TrashLines` 各有一个"一个连接上读 8 次、无统计"的成本测试（`//go:build !race`，照 `subtree_cost_test.go`：一本笔记本 10 万个节点、其中 1 万个回收站的根），时限写在测试里。

### 3.5 用例

- **`ListTrash`**（`GET /notebooks/{notebook_id}/trash?limit&cursor`）：游标先核对（400）；笔记本的 `WorkspaceOf` → 以 `page.history` 判定（看不到答 404 `notebook.not_found`）；`limit`（422，`shared.PageSize`）；`since = now − (Retention − TrashMargin)`；读 `limit + 1` 项；`TrashLines`（一条）、删除者的显示名（`Names`，一次）、附件根的信息（`Attachments`，一次）。每项：`TrashRoot`、原位置、删除者、`RestorableUntil`、统计、放置。
- **`GetTrashItem`**（`GET /nodes/{node_id}/trash-item`）：`readableNode(id, page.history)`（不看类型；404 `page.not_found`）→ 活着的 409 `page.not_in_trash` → 不是根的 409 `page.not_trash_root`（`root_id` 由 `TrashRootOf`）→ 期限之外 409 `page.restore_expired` → 同列表的一项。
- **`RestoreNode`**（`POST /nodes/{node_id}/restore`，没有请求体），码照 4.14 的次序：
  1. `FindAnyNode`，笔记本活着，未加锁：没有、笔记本已删 404 `page.not_found`；`Writer.Allowed`（`node.restore`）：没有角色 404、读者 403。
  2. 活着 → 409 `page.not_in_trash`；不是根 → 409 `page.not_trash_root{root_id}`；`!Restorable(deletedAt, now, Retention)` → 409 `page.restore_expired`。
  3. `FindTrashRoot` 的统计：`pages + assets > MaxUnitNodes` → 422 `page.restore_too_large`（不读组）。
  4. `TrashGroup`（未加锁）：有页的正文已不在（版本号为 0）→ 409 `page.trash_changed`（不去解析空的正文）；多于一页时，页数 `> MaxUnitPages`（100）或正文字节 `> MaxUnitBytes`（8 MiB）→ 422。
  5. `TrashContents`，`ContentParser.DecidedAll`：按大小从大到小 `Take`，各次的等待合计至多 `parse_max_wait`（`page.New` 以 `Budget.Wait()` 交给 `ContentParser`，不另加配置；每次 `Take` 的期限是剩下的等待；它因这一期限失败、请求本身未到期时答 503 `server_busy`）；多于一页时边解析边数链接，超过 `MaxUnitLinks` 就停下、放回预算、答 422（链接数只能解析之后知道，所以它可能排在 503 之后，契约写明）。
  6. `Writer.Run(UnitSpec{NotebookID, Action: node.restore, Tree: true, Client, NotFound: ErrNotFound, Kind: trash_restore}, u.Restore(…))`。`Options` 留空：改写链接的参与者对没有"前"的变更什么都不做（`Relocation` 答 false）。
  7. 答 `NodeRestored`；记日志 `node restored`（ids、`nodes`、`pages`、`assets`、放置的原因、根是否改名、组内改名的个数、客户端；不记名称）。

**`Unit.Restore`**（`unit_restore.go`，笔记本行 `FOR NO KEY UPDATE` 之下）：

1. 重读根（`FindAnyNode`）：仍以 T 删除（`Equal`）、仍是根，否则 409 `page.trash_changed`（这期间别人恢复了它，或别的变化：一律"这一项变了"，网页重读）。以单元的时刻再核对期限（409 `page.restore_expired`）。
2. 重读 `TrashGroup`，与用例第 4 步未加锁读到的逐项比较（节点、类型、页的版本号；页的版本号为 0 即正文已被清理）：不同则 409 `page.trash_changed`。
3. **放在哪里**：`PlacementOf`，父节点是否活着由单元在锁下重读（`lineUnder`：已删的父节点是 `errNotAPage`，即 `parent_in_trash`），层数用第 2 步的组。原处时次序值用它原来的（不重算：兄弟这期间被重新编号过时，它落在按值排的位置，与兄弟同值时按 id，写明即可，调研第 25 项）；根下时放在末尾（`placeAmong(根的子节点, len − 1)`，可能重新编号根的子节点）。
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
           sort_order = CASE WHEN n.id = sqlc.arg(root) THEN sqlc.arg(sort_order) ELSE n.sort_order END
       FROM u
       WHERE n.id = u.id AND n.deleted_at = sqlc.arg(deleted_at)::timestamptz
       RETURNING n.id
   ), contents AS (
       UPDATE page_contents c SET deleted_at = NULL
       WHERE c.node_id IN (SELECT id FROM back) AND c.deleted_at = sqlc.arg(deleted_at)::timestamptz
       RETURNING c.node_id, c.revision
   ), revisions AS (
       UPDATE page_revisions r SET deleted_at = NULL
       WHERE r.node_id IN (SELECT id FROM back) AND r.deleted_at = sqlc.arg(deleted_at)::timestamptz
       RETURNING r.node_id, r.revision, r.base_revision
   ), items AS (
       UPDATE changeset_items i SET deleted_at = NULL
       WHERE i.node_id IN (SELECT id FROM back) AND i.deleted_at = sqlc.arg(deleted_at)::timestamptz
   )
   SELECT (SELECT count(*) FROM back)::integer AS nodes,
       (SELECT count(*) FROM contents)::integer AS contents,
       (SELECT count(*) FROM contents c JOIN revisions r USING (node_id, revision))::integer AS current_versions,
       (SELECT count(*) FROM revisions WHERE base_revision IS NULL)::integer AS creations,
       (SELECT count(*) FROM revisions r WHERE r.base_revision IS NOT NULL AND NOT EXISTS (
           SELECT 1 FROM revisions p WHERE p.node_id = r.node_id AND p.revision = r.base_revision))::integer AS gaps;
   ```

   版本行与条目在所有变更集里、`deleted_at` 等于 T 的都恢复（含删除它们的变更集的条目）。正文的 `updated_*` 不动（正文没变）。期望：`nodes` 是组的节点数；`contents`、`current_versions`、`creations` 都是页数；`gaps` 为零（每页恰有一个创建的版本行，之后每一行的 base 是前一行的版本号：`unit_content.go` 的写与 P1 的链守卫）。不符 → 409 `page.trash_changed`。理由：清理与这条语句之间没有锁，清理先删条目、版本（每批 1,000 行、不排序），后删正文；只数正文会放过"有正文、版本断了链"的页（`checkPages` 的"版本不连成链"，P1 的 `FindVersion` 会以下一版答链缝里的版本号）。兄弟的唯一索引逐行核对：组里的其余节点在更新之前仍是已删的（不在部分索引里），根与组员的名称已由第 4 步定下，不会途中撞上。
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

  bootstrap 的 `assetObserver`：删除的照旧交 `NodesDeleted`；`Restored` 不空且 `Asset` 的按 `DeletedAt` 分组交 `NodesRestored`，答的行数不等于这一组的附件数 → `page.ErrTrashChanged`（清理拿走了行：恢复的附件会没有行，下载 404、`checkAssets` 失败）。asset 不知道 page 的码，由组合根换成它。行与节点的 `deleted_at` 出自同一个单元的 `At`，`Restored.DeletedAt` 从节点读回，与行的相等。
- **linking**：不改。恢复的页的变更没有"前"、带 `Revision` 与 `Facts`，观察者 `ReplacePage` 并重新解析以这些名称为目标的链接（`links` 事件照常）。
- **`pages` 事件**：恢复的页带它（未变的）版本号出现在 `pages` 里、`tree: true`：网页把它当作这一页在那个版本写过，重读至多多一次视图，接受（调研第 24 项）。
- `page/extension_test.go` 的 `TestTheObserversFollowAUnit` 加恢复：观察者看到 `Restored` 的时刻与是否附件。

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
- **清理的时钟**：不加 `PurgeConfig.Clock`。交错测试直接调用清理器（`tm.purge`），截止时刻由测试给：`tm.purge` 改为 `tm.purgeBefore(t, before)`（原来的调用传 `time.Now()`），期限两侧的测试传 `now − R` 或更晚的时刻（模拟时钟偏差）。期限的边界（恰在期限的那一刻不能恢复）在 `page/app` 以假时钟测；整个程序上的测试以 SQL 移动一组的时刻（3.10 的 `tm.trashedAgo`）。
- **随之改的固定值**：`platform/config/validate.go`、`validate_test.go:124`；`server/configs/config.yaml`、README；`instance/adapter/http/handler_test.go`（整个 JSON）、`instance/app/get_info_test.go`；`bootstrap/api_test.go`；`platform/httpserver/apitest/apitest_test.go` 的 `instanceJSON`（契约是封闭的）；`web/apps/web/src/test/fakes.ts` 的 `instanceJSON`；e2e `s3-instance-info.spec.ts`（默认 60 天减 1 小时，5,180,400）；`bootstrap/app_test.go` 的 1440h 与 `configs/embed_test.go` 的 60 天不变。

### 3.9 契约

- **`listTrash`**：`TrashItemPage {data: TrashItem[], next_cursor}`；`[bad_request, notebook.not_found, validation_failed]`。
- **`getTrashItem`**：`TrashItem`；`[page.not_found, page.not_in_trash, page.not_trash_root, page.restore_expired]`。
- **`restoreNode`**：没有请求体；答 200 `NodeRestored`；`[page.not_found, forbidden, page.not_in_trash, page.not_trash_root, page.restore_expired, page.restore_too_large, server_busy, page.trash_changed]`；描述写放置的规则、序号、期限、一个单元的界线（只有一页时不限页数、字节与链接），以及链接数的 422 可能排在 503 之后。
- **结构**：
  - `TrashItem {id, notebook_id, kind: NodeKind, name, ancestors: Ancestor[]（原位置，含回收站里的祖先）, deleted_at, deleted_by: UserRef, restorable_until, pages, assets（组里的，含根）, restore_to: TrashPlacement, revision: integer|null（页的正文的版本号，预览以它读 getPageVersionView；附件为 null）, asset: TrashAttachment|null}`。`asset` 照 `EditLock.holder` 写成 `anyOf: [{$ref: TrashAttachment}, {type: 'null'}]`（契约的编写规则不认内联的可空对象）。
  - `TrashAttachment {mime, byte_size}`（`additionalProperties: false`）。
  - `TrashPlacement {parent_id: uuid|null, reason: original | parent_in_trash | too_deep}`：`original` 时 `parent_id` 是原父节点（原来就在根下则为 null），其余为 null（根下）。
  - `NodeRestored {id, kind, name, renamed, placed: TrashPlacement, changeset_id}`。
- **`UserRef`**（`common.yaml`，`{user_id, display_name}`，`additionalProperties: false`）：`TrashItem.deleted_by` 用它；P1A 的 `ChangesetAuthor` 与形状完全相同的 `EditLockHolder`、`TransferStarter` 一并改用它（P1A 审查 C6：一种形状一个名字，第四个同形的结构不再出现；各处的说明移到引用它的属性上）。审查以仓库的 `oapi-codegen` 与 `openapi-typescript` 实测过：生成 `apigen.UserRef`，`EditLock.Holder` 是 `nullable.Nullable[externalRef0.UserRef]`，`bodyshape.gen.go` 不变。Go 里要改的：`page/adapter/http/handler.go:330,333`、`versions.go:99`、`transfer/adapter/http/server.go:99`（各加 `apigen` 的 import：archtest 只拦模块自己的 `gen` 被别处 import）。网页只改 `services/page.service.ts` 的转出（`export type EditLockHolder = UserRef`），`edit-session.ts` 与 P1B 的分支只用字段。`Problem.ended_by` 形状相同但留作内联：`Problem` 与 `httpserver.Problem` 逐字段对照，不动它。合并次序：P1B 若先合并，P2A 在它之上改 `page.service.ts` 这一行。
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
  5. 恢复时清理拿走了正文（模拟时钟偏差）：恢复在笔记本行上等着时，清理以 `now + 2h` 只运行条目、版本、正文三个清理器；放开之后 409 `page.trash_changed`，什么都没恢复。
  6. 恢复附件的行与附件的清理（M7 的移交第 2 项）：`holdRow` 拿着一个组里附件的行（清理的批），恢复在 `asset_blobs` 上等；拿着的一方删掉文件与行并提交 → 409 `page.trash_changed`、整个单元回滚；拿着的一方回滚 → 恢复 200、文件在、可以下载。
  7. 嵌套的根与它父节点的组（`notebookRow`）：D 先被删、之后它的父节点 P 被删；恢复 D 与恢复 P 交错：P 在先 → D 回到 P 下（`original`）；D 在先 → D 到根下（`parent_in_trash`）。
  8. 原父节点这期间被移深（`notebookRow`）：移动在先 → 恢复到根下（`too_deep`，单元在锁下重算，不用未加锁时的预测）；恢复在先 → 回到原处，之后的移动照它自己的层数核对。
  9. 恢复等着时删除组里的一页：删除未加锁的 `FindNode` 就答 404，不等；放开之后恢复 200；再删除恢复了的那个子页，它成为新的根、带它自己的统计，根的不变式成立。

### 3.11 一个单元的大小与实测

- 界线（`domain`，P3 的撤销同用）：`MaxUnitPages = 100`、`MaxUnitBytes = 8 MiB`、`MaxUnitLinks = 20,000`（与导入的一批相同；只有一页的组不受这三项限制）、`MaxUnitNodes`（树的操作：组里的页与附件，实测之前取 2,000）。
- **实测**：`bootstrap/trash_cost_test.go`（`//go:build !race`）里的基准，手动运行：`go test -run '^$' -bench 'BenchmarkTrash' -benchtime 3x ./internal/bootstrap/`。
  - **经单元、进程内**：基准自己组一个 page 模块：`pageDeps(...)`（真的观察者：事件、索引、附件；真的预算），`Deps.MaxUnitNodes` 抬到足够大（生产的 `pageDeps` 不设它），路由挂在 `httpservertest` 的 API 上，进程内经 `restoreNode` 的处理器计时。不经正式的 API：它的界线是实测之前的 2,000，更大的档都会先答 422。
  - **组**：每组至多 100 页（根与 99 个子页面）。两种页的负载：轻（每页 8 KiB、20 条链接）与满（三项界线的上限：100 页合计 8 MiB、20,000 条链接，每页约 82 KiB、200 条，linking 每页至多记 10,000 条，所以至少两页）；附件 0 / 1,000 / 2,000 / 5,000 / 10,000 个（节点与行以 SQL 成批写入，分在各页之下）。数据经同一模块删除，无统计与 `ANALYZE` 之后各一遍。
  - **计时与取值**：单元（恢复的请求减去解析）、解析（满负载另记，对照 15 秒减 `parse_max_wait` 的 13 秒）、删除各记。`MaxUnitNodes` 取满负载下单元不超过 5 秒的最大"页 + 附件"数，向下取整（15 秒的请求期限的三分之一，留给解析的等待与解析本身、更慢的服务器）。每档的数字写回本文第 9 节与常量的注释。
- 守住它：同一文件里一个普通测试以满负载在 `MaxUnitNodes` 处（至多 100 页）恢复一次，时限 10 秒（CI 比开发机慢）。删除若在默认的导入上限（5 万个节点）超过 15 秒，写进 M12 的移交（今天删除没有界线，P2 只让它更快）。

### 3.12 测试

- **domain**：`Then` 保留第一项的 `Restored`；`Subtree.Counts`；`Restorable` 的边界（期限当时不可恢复）；`PlacementOf` 的四种（原来在根下、原处、`parent_in_trash`、`too_deep`）；单元的界线（只有一页时不限）。
- **用例**（`page/app`，假的端口与时钟）：`RestoreNode` 的每个码与次序（含链接数的 422 在 503 之后的例外、未加锁时正文已不在的 409）、锁下的每种 `page.trash_changed`、放置与序号（附件"x 2.png"）、键的重算与组内同键的序号、变更的 `Restored` 与 `Revision`、`Kind` 是 `trash_restore`、`Options` 为空；`DecidedAll` 的从大到小、等待合计（假的预算：第一次等去大半，第二次超出剩下的 → 503）、请求到期时不是 503；`ListTrash`、`GetTrashItem` 的游标、期限的过滤、放置、显示名、附件信息；`Delete` 交给 `DeleteNodes` 的根与统计；`recordItems` 一步一次、没有要记的不写、重复的节点是错误；编辑锁对 `OpRestore` 放行。`page/app/asset_test.go` 的调用序列（`"RecordItem in tx"`）随之改名。
- **适配器**（真的 PostgreSQL）：`DeleteNodes` 只标根并记统计、更早删除的子孙保留它的、先删子孙再删祖先时 `stray` 不为零；`RecordItems` 的空值（根下、没有前、没有后）、冲突时保留 `restored` 与 `revision`，`store_test.go`、`purge_test.go` 里直接调用 `RecordItem` 的改为 `RecordItems`、旧的合并行为照旧；`RestoreNodes` 的五个数（删掉一行正文、当前的版本行、创建的版本行、中间的一行版本之后各自不符）；`ListTrash` 的次序、游标、`since`、`TrashFrom`；`TrashLines` 经已删的祖先；`TrashGroup` 不含更早删除的子孙；`TrashRootOf`；`SourcePath` 经已删的祖先；`AttachmentInfos` 含已删的行；`RestoreBlobsOfNodes` 只认同一时刻；两个成本测试（3.4）。
- **linking**：`Views.Resolve` 对 `Deleted` 的来源用 `SourcePath`、不读索引；`resolutions` 的 `known`；`Index.resolve` 对缺失的来源照旧报错。
- **handler 与平台**：契约的结构、路径参数、各码、`root_id` 只在 `page.not_trash_root` 上；`root_id` 的平台测试（3.9）。
- **整个程序**（`bootstrap/trash_test.go`；13.1 第 21 条：组合根交空时失败）：删除带附件与链接的子树 → 列表与单项（统计、原位置、放置、显示名、附件信息、`notebook_id`）→ 回收站里的页的视图从原路径解析（3.7 的三例）→ 恢复：正文、版本、条目（`restored`、`revision`）、附件的行（`GET /assets/{id}` 200、新签的地址下载 200、无主列表的大小算上它们，扩展 `assets_lifecycle_test.go`）、索引（`checkLinks`）、`pages`（`tree: true`）与 `links` 事件；放置的四种与序号；422 的各项界线；`trash_retention_seconds`。
- **不变式**：`checkPages` 加"活着，却有已删的正文、版本或条目"（`"deleted, with what follows it not deleted"` 的反面）与"已删的节点：是根 ⇔ 父节点为空或不是同一时刻删除、且不是随笔记本删除的"；`checkAssets` 加"已删的行的时刻等于它的节点的"。
- TB1 的接口版本在 B 里做（4.7），用到 A 的契约。

## 4. B：前端

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/services/page.service.ts` | `trash`、`trashItem`、`restoreNode` 与类型（`TrashItem`、`TrashItemPage`、`NodeRestored`） |
| `web/apps/web/src/stores/page-tree.store.ts` | `removed` 记下答复时读的序号与去掉规则；`restore`（按节点一个一个、经写入队列）；`trash`、`trashItem` 读 |
| `web/apps/web/src/lib/read-pages.ts` | "按已读的页数重读"，回收站的列表用（反链与版本列表的两份私有实现不动，留给 v0.1 之后的打磨） |
| `web/apps/web/src/pages/page/version-view.tsx` | 从 P1B 的 `version-page.tsx` 移出的只读阅读视图（参数是页 id、版本号与名称）与 `useLeftBehind`（版本页与预览共用） |
| `web/apps/web/src/pages/notebook/trash-page.tsx`、`trash-row.tsx` | 回收站页 |
| `web/apps/web/src/pages/notebook/trash-preview.tsx` | 预览 |
| `web/apps/web/src/pages/notebook/notebook-places.tsx`、`notebook-layout.tsx` | 左栏的入口（P3 再加"历史"） |
| `web/apps/web/src/app/routes.tsx` | `notebooks/:id/trash`、`trash/:nodeId` |
| `web/apps/web/src/events/handlers.ts`、`app/event-stream.tsx` | 新键的重读、连上时的重读 |
| `web/apps/web/src/pages/notebook/page-tree.tsx`、`page-tree-item.tsx`、`pages/page/attachments-section.tsx` | 删除确认框；期限在 `PageTree` 读一次 |
| `web/apps/web/src/i18n/format.ts` | `formatPeriod` |
| `web/apps/web/src/pages/page/attachment-row.tsx` | 导出类型图标（`AssetIcon`），预览的附件行用；路由状态里点名的附件一行取得焦点 |
| `web/apps/web/src/pages/page/unresolved-link.tsx` | 只读视图的说明不只说"版本" |
| `web/apps/web/src/i18n/messages/{zh-CN,en}.ts` | 文字（前缀 `trash.`） |
| `web/apps/web/src/test/page-server.ts` | 假接口的回收站路由 |

### 4.2 回收站页

- 路由 `/:slug/notebooks/:id/trash`，在笔记本的外壳里、页面的外壳之外（页面的外壳对不在树里的页答 404）。文档标题"回收站 · 笔记本名"；标题 `h1`"回收站"。
- 列表：SWR 键 `["trash", nb]`，`readPages`（每页 50 项，"更多"用 `useLoadMore`）；列表有名称；每项一行：类型图标、名称（链到预览）、原位置（"原位置：A / B"，根下写"笔记本首页"）、删除者与相对时刻、"可恢复至 …"（`restorable_until` 的完整时刻）、"含 N 个页面、M 个附件"、恢复到哪里（"恢复到「Y」下"，原来就在根下时"恢复到笔记本首页"；"恢复到根下：原上级页面在回收站里"；"恢复到根下：放回原处会超过 10 层"）。
- **同名的项**：已读到的项里名称与原位置都相同的（`twinsOf`，同无主笔记本的清单），按钮与名称链接的可及名称加删除时刻，"恢复「X」（原位置：A / B，删除于 …）"；不同名的只带原位置。
- 写者的按钮"恢复「X」（原位置：A / B）"：`pages.restore(id)`。它在恢复出去期间 `aria-disabled` 且 `aria-busy`；store 里同一节点的恢复一个一个来（`oneAtATimeById`，同 P1B 的还原），连点两下只发一个请求。
  - 不论答什么（200、409、422、403、404），之后都重读 `["trash", nb]` 与缓存里的 `["trash-item", nb, id]`：不只靠事件（事件流断开、重连时，列表会留着已恢复的项）。
  - 成功：留在回收站，状态行"已恢复「X」到「Y」下 [打开]"（改名时"已恢复为「X 2」…"），焦点回到 `h1`（同无主笔记本的接管）；`<output>` 先空着渲染、答复之后再填字（同 P1B 的页面外壳），读屏才会念出。"打开"：页去它的页面；附件去它所在的位置：父页的页面、附件一节里它那一行取得焦点（路由状态点名它），根下的去笔记本首页的附件一节（同 00-M8 4.12 的搜索结果，P5 照用）。
  - 409 `page.not_in_trash`、`page.trash_changed`：说明"这一项在这期间变了"；`page.restore_expired`：说明已过期限；`page.not_trash_root`（P2 里不会有，P3、P4 之后可能）：说明它已并入另一项；422：说明"这一项太大（N 个页面、M 个附件），不能在这里恢复"（P4 改为问是否在后台进行）；503：照现有的忙的说明。
  - 403（这期间被降为读者）、404（笔记本没了或看不到了）：`errorText`，并重读 `["notebooks", ws]`（同无主笔记本页的 `refused()`）。
- 读者只看列表与预览。

### 4.3 预览

- 路由 `/:slug/notebooks/:id/trash/:nodeId`；SWR 键 `["trash-item", nb, nodeId]`。文档标题"X · 回收站 · 笔记本名"。单项的 `notebook_id` 与地址里的笔记本不同时答 `NotFoundPage`（别的笔记本的项不在这本的外壳里显示）。
- 页：信息（原位置、删除者与时刻、期限、数目、恢复到哪里），一句说明"这是回收站里的页，链接按今天的树解析，链到同一次删除的页要恢复之后才回来"（3.7），下面是 `VersionView`（`getPageVersionView(nodeId, item.revision)`，键 `["page-version-view", nb, nodeId, revision]`，P1B 的重读规则照用：`tree: true` 时重读；`links` 事件不会点到回收站里的页，它没有索引）：只读，不能勾任务，未解析的链接只说明"这是只读的视图"。离开预览时 `useLeftBehind` 把这一页的视图从缓存去掉（几十个预览不把各自的 HTML 留在内存里、也不在每次 `tree: true` 时重读）。
- 附件：名称、类型（`assetKind` 的文字与图标）、大小（`formatBytes`），"恢复之后可以下载"；不签发地址。
- 页的 `revision` 为空、附件的 `asset` 为空（期限之内不会有：正文或行已被清理）：显示"无法预览"，不去读视图。
- 写者的"恢复"：流程与答复同列表；成功之后回到回收站页（路由状态带状态行），焦点在 `h1`。
- `page.not_trash_root`：`replace` 到 `root_id` 的预览；`page.not_in_trash`：说明"它已不在回收站里"并给出打开它的链接（树里有它时）；`page.restore_expired`：说明并给回收站的链接；404：`NotFoundPage`。

### 4.4 页面树

- `removed` 改为 `Map<string, {to: string | null; after: number}>`：`remove` 在删除的答复之后记下 `after = readsStarted`；`read()` 保留一次读（`readKept`）时，`read > after` 且这次读把它列为活着的页的条目去掉（与替换 `nodes` 同一个 `runInAction`，`sameNodes` 时也做）。同步恢复、别的标签页与别人的恢复、P4 的任务、P3 的撤销都是"之后的一次读列出它"，一条规则覆盖；早于答复开始的读去不掉它（它被 `changesAnswered` 丢弃或 `read ≤ after`）。`removedTo` 答 `entry?.to`。
- `restore(id)`：`this.restores(id, () => this.write(() => this.service.restoreNode(id)))`：同一节点一个一个来，再经树的写入队列（13.2 第 1 条）；422 与 409 不记什么。

### 4.5 删除确认框与左栏的入口

- 页面与附件两个确认框的正文改为"移到回收站，{period}内可以恢复；能读这本笔记本的人在回收站里仍看得到它。"（附件另加"嵌入它的页面会显示它缺失"）；子页面与附件数目的两句不变（PG4、AS4 的断言照旧）。`{period}` 来自 `InstanceInfo.trash_retention_seconds`：`formatPeriod` 满一天写整天数、不满写整小时数，都向下取整（不把期限说长；默认写"59 天"）；信息未读到时用不带期限的一句。页面的确认框不在每一行读 `InstanceInfo`：`PageTree` 读一次（`useSWR("instance")`），经属性交给各行的菜单；附件一节已经读它。
- 左栏：`NotebookLayout` 的栏里，页面树之后一个单独的 `<nav aria-label="{notebook} 的更多">`，`NavItem`"回收站"。不放进页面树的 `nav`：vitest 的帮助函数与 e2e 的 `treeTitles()` 把那里的每个链接当作页面。

### 4.6 事件、列表的模式与测试

- 事件：`pages` 的 `tree: true` 经 `refresher` 重读 `["trash", nb]` 与这本笔记本的 `["trash-item", nb, *]`；`refreshedOnConnect` 的最内一层加 `trash`、`trash-item`。
- `readPages` 新放在 `lib/read-pages.ts`，只给回收站的列表用；13.2 第 19 条按 4.13 改写（"按已读的页数重读"）。
- **vitest**：`readPages`；`PageTreeStore`：`removed` 被之后的读去掉、不被之前的读去掉、恢复经队列、同一节点的第二次恢复排在第一次之后、422 不记；回收站页：行的文字与名称、放置的四种、同名的项的可及名称、"更多"、恢复的每种答复（200、409 四种、422、503、403、404）与焦点、每种答复之后列表重读（事件流断开时）、连点两下只发一个请求、恢复的附件的"打开"去它所在的位置、读者没有按钮、事件与连上时的重读；预览：页的阅读视图以单项的版本号读、离开时视图的键从缓存去掉、附件的信息、"无法预览"、`notebook_id` 不符时的 `NotFoundPage`、`not_trash_root` 的跳转、`not_in_trash`、期限、预览里的恢复回到回收站并带状态行与焦点；删除确认框的文字与期限、未读到时的文字；`formatPeriod`；左栏的入口不在树的 `nav` 里；经组合根到达。
- 删除确认框的旧断言随文字改：`attachments-section.test.tsx:634` 的整句。

### 4.7 e2e

- **TB1**（`e2e/stories/trash/tb1-trash.spec.ts`；回收站是自己的一块，P4 的 TB2 也放这里；接口版本与页面版本都在 B 里做，照 P1 的 HI1、HI2）：
  - 接口版本：PAT 建页（含链接）与附件，删除子树；数据库断言 `expectTrashRoot`（只有根有 `trash_root` 与统计，页数、附件数、层数对）；`listTrash` 列出它与数目；`getTrashItem` 的 `revision` 读视图，链接解析到活着的页；恢复：放回原处，数据库断言 `expectSubtreeRestored`、`expectAssetsRestored`（行活着、文件在）、`expectIndexedLinks`；`getAsset` 新签的地址下载 200；`pages` 与 `links` 事件到达；先删子页面、再删它的上级、恢复子页面 → 根下（`parent_in_trash`）；原位置新建同名的页之后恢复 → "名称 2"；一组以 `trashedAgo` 移到期限与清理的截止之间（删除于 `now − purge_retention + 30 分钟`：已过 `restorable_until`，又在清理的截止之后，e2e 每 2 秒一次的清理碰不到它）→ 列表不列、恢复 409 `page.restore_expired`。
  - 页面版本：树的菜单删除（确认框写明回收站与天数）；左栏进回收站；打开预览，链接可以点到活着的页；从列表恢复，状态行与"打开"；同一个标签页里树中看得到它（`treeTitles` 轮询），打开它的页。
- 夹具：`e2e/fixtures/trash.ts`（`listTrash`、`getTrashItem`、`restoreNode`、`trashPath`、`restoreWith`、`trashedAgo`：以一组的删除时刻为准整体移动，节点、正文、版本、条目、附件的行各 `deleted_at = deleted_at − interval WHERE deleted_at = T …`，不用每条语句的 `now()`，同 3.10）；`assert/page.ts` 的 `expectTrashRoot(db, rootId, {pages, assets, height})` 与 `expectSubtreeRestored(db, rootId, ids, restorerId, {parentId, name})`；`assert/asset.ts` 的 `expectAssetsRestored(db, storageDir, assets)`。

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
  - v0.1 设计第 13 节的条目随实现改（A、B 各自的提交）：13.1 第 1 条（`OpRestore`、整步的条目、从上往下删）、第 4 条（三个操作的码的次序）、第 5 条（asset 的观察者恢复已删的行）、第 6 条（恢复的期限、保留期的下限）、第 8 条（`root_id`）、第 11 条（`asset.NewAttachmentInfos`）、第 15 条（保留期交给 page）、第 19 条（恢复在单元之前从大到小、等待合计）、第 21 条（最后一跳）、第 27 条（回收站的游标）、第 29 条（`OpRestore` 经编辑锁放行）、第 31 条（单元的界线），13.2 第 1、19、23 条（B），13.4 第 3、4、6、9 条。
  - M7 设计 4.6（`00-M7-design.md:189`）"M8 的恢复锁同样的行，与清理串行"一句改为"M8 的恢复与清理以一小时的间隔分开（M8 设计 4.11）"。
  - `01-P1-versions.md` 3.6 的 `ChangesetAuthor` 改为 `UserRef`（实施时）。
  - README 的"软删除与清理"一段。
  - 移交文件（`handoffs/M6-links.md`、`M7-assets-transfer.md`）不改：它们记的是当时移交的内容，落实记在 00-M8 的"移交的落实"表里（2.1 第 7 条）。

## 8. 交给后面的 Phase

- **P3**：撤销一次恢复用条目的 `restored`、`revision`；撤销里的恢复（C 删除的节点回来）同样设 `Change.Restored` 并清掉根的标记与统计；撤销里的删除经 `DeleteNodes` 记根与统计，**从上往下删**：一个单元里每棵要删的子树删一次、从它的最上层删，不先删子孙再删祖先（`DeleteNodes` 的 `stray` 会拒绝）；`DecidedAll` 与单元的界线共用；`RecordItems` 合并时保留第一次的 `restored`/`revision`；清理截掉的标记、空变更集的清理照 4.2（P2 没有标记被清理拿走的条目，3.5）。
- **P4**：分批恢复复用 `PlacementOf`、`children.free`、`RestoreNodes`（ids 可以是组的一部分，根的参数给这一批的根；五个数按这一批的期望核对）；途中停下时，新标上的根要写统计（任务读过组，手里有）；撤销的分批删除用同一时刻时，上层的根之下早先标上的根取消、统计重算（跨单元，`stray` 只管一个单元）；网页的 422 改为问是否在后台进行。
- **P5**：搜索结果的附件信息用 `AttachmentInfos`；附件的"打开去它所在的位置"沿用 B 的路由状态。
- **M12**：删除没有界线（实测若超过请求期限，见 3.11）；反链、版本列表与回收站的三份"按已读的页数重读"收拢（v0.1 之后的打磨）。

## 9. 结果

（完成后补写；`MaxUnitNodes` 的实测数字与每档的耗时写在这里。）

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
