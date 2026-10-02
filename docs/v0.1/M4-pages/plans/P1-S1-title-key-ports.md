# M4/P1/S1 标题键与端口：实施计划

上级：[P1 文档](../01-P1-page-module-pipeline.md) 3.2、3.3、3.4。

## 任务

1. `internal/shared/title_key.go`：`TitleKey(s string) string`，`norm.NFC` → `cases.Fold()` → `norm.NFC`，`Caser` 每次调用新建；注释写明它是语言无关的完整折叠、与 Obsidian 的 `toLowerCase` 的差别、Unicode 数据升级要 `nervewiki reindex`（M6）。
2. `go.mod` 加 `golang.org/x/text/cases`（同一个模块 `golang.org/x/text`，版本不变）。
3. archtest：`isPureLibrary` 放行 `golang.org/x/text/cases`；`purity_test` 的传递依赖放行 `golang.org/x/text/` 前缀；规则的说明文字与 `rules_cases_test.go` 的常量同步；`page/domain → x/text/language` 的直接导入仍被拒。
4. `internal/modules/notebook`：
   - `queries/notebooks.sql` 加 `ShareNotebook`（`FOR SHARE`，带 `deleted_at IS NULL`）；改正"M4 的页面写只锁笔记本行"的注释（`notebooks.sql`、`app/extension.go`）。
   - store 加 `ShareNotebook`；模块根 `notebooks.go`：`Notebooks` 接口与 `NewNotebooks(pool)`（`WorkspaceOf`、`ShareByID`、`LockByID`；锁方法以 `postgres.InTx` 守卫；没有、已删除答 `false`）。
5. `migrations/schema_test.go`：索引描述加一个字母标出 `NULLS NOT DISTINCT`（`pg_index.indnullsnotdistinct`）；现有的名单不受影响（还没有这样的索引）。

## 测试

- `TitleKey` 的表格（P1 文档 3.2 的各例）。
- `NewNotebooks`（真实数据库，照 `workspace/workspaces_test.go` 与 `TestLockNotebookWaitsForTheRowAndSeesADeletion`）：找到、已删除、不存在；锁方法在事务之外报错；`ShareByID` 与持 `FOR NO KEY UPDATE` 的事务之间等待、对方提交删除之后读到没有；两个 `ShareByID` 互不等待。
- 反向对照：`TitleKey` 去掉第二次 NFC（折叠之后不是 NFC 的例子失败）；`ShareByID` 去掉事务检查；archtest 的直接导入放行整个 `x/text`（`x/text/language` 的反例失败）。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
