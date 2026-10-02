# M4/P1/S2 页面的数据：实施计划

上级：[P1 文档](../01-P1-page-module-pipeline.md) 3.4、3.5、3.9、3.10。

## 任务

1. 迁移 `00012_page_nodes.sql` … `00016_page_page_revisions.sql`（3.4 的列、约束与索引名；`CREATE TABLE` 顶格写，清理测试从文件名取表的归属）。
2. `sqlc.yaml` 加 page 一条，只列这五条迁移（照 notebook 的 overrides）；`deploy/runtime-grants.sql` 给五张表 DML。
3. `migrations/schema_test.go`：新的约束与索引名（含复合外键要的 `nodes_notebook_id_id_key`、`NULLS NOT DISTINCT` 的标记）；每条 CHECK 一个反例（未知的 kind、空名称、256 字节、空键、`parent_id = id`、`revision 0`、哈希不是 32 字节、`byte_size` 与正文不符、超过 5 MB、未知的 kind 与客户端、空说明、条目的前后状态不成对、版本的 `base_revision ≥ revision`）。
4. `modules/page/domain`：`node.go`（`Node`、`MaxDepth`、`Title` 与 `CheckTitle` + `TitleKey`）、`order.go`（`Place`）、`tree.go`（`PreOrder`、`Depth`）、`change.go`（`TreeState`、`Change` 与合并、`Operation`）、`errors.go`（`page.not_found`、同码的 `notebook.not_found`、`page.title_taken`、`page.too_deep`）。
5. `modules/page/adapter/postgres`：查询与仓储：插入节点、正文、变更集、条目（插入或合并更新）、版本；按笔记本列出未删的节点；按 id 读节点（不加锁，带笔记本与工作区所需的列）；一个父页下未删的兄弟（按次序）；重新编号一组兄弟；改名；祖先的递归查询；页面的正文元数据；笔记本删除连带的软删除（只动未删的行）；23505 的翻译。
6. 清理：`queries/purge.sql`（五个清理语句，`nodes` 的"删叶子"带跟随的行的 `NOT EXISTS`）、`purge.go`（`nodes` 在一次调用里循环）、模块根 `purgers.go`；组合根 `purgers(pool)` 改为 `page`、`notebook`、`workspace`（这一步就要接，迁移一加上表，注册表测试就要求它）。整个清理任务的测试在 S4 加页面的种子。

## 测试

- 领域：`Place` 的表格（空、最前、最后、中间、连续插入 200 次严格递增、间隔耗尽时整组重排）；`PreOrder`（父在子前、兄弟按次序与 id）；`Change` 的合并；`Title`（规则来自 `CheckTitle`，键来自 `TitleKey`）。
- 仓储（真实数据库）：插入的一组行同一时刻；列出只含未删的；兄弟按次序；祖先链从根到父页；重名的插入与改名答 `page.title_taken`；根下的重名同样被拒（`NULLS NOT DISTINCT`）；笔记本删除的软删除不动已在回收站里的行。
- 清理（照 `notebook/adapter/postgres/purge_test.go`）：保留期前后与边界；`batch = 1` 的调用次数；三层的树一次调用清完；子页被持有时别的叶子照删、被持有的子页与父页留下、不等锁，放开之后清完；正文被持有时它的节点留下；变更集等条目与版本清完才删。
- 反向对照：`nodes` 的清理不循环（三层树的测试失败）；"删叶子"不看跟随的行（正文被持有的测试失败或等锁超时）；去掉 `NULLS NOT DISTINCT`（根下重名的测试与名单失败）；复合外键改为普通的自引用（约束名不变时名单照样通过，失败的是 `TestAParentInAnotherNotebook`，P1 审查 D9）。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
