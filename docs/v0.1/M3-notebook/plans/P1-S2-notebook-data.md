# M3/P1/S2 数据：实施计划

上级：[P1 文档](../01-P1-notebooks-access.md) 3.4、3.5、3.6。

## 任务

1. 迁移：`00009_notebook_notebooks.sql`、`00010_notebook_notebook_members.sql`（3.6 的列、约束与索引名）。
2. `sqlc.yaml` 加 notebook 一条，只列这两条迁移；`deploy/runtime-grants.sql` 给两张表 DML。
3. `migrations/schema_test.go`：新的约束与索引名；CHECK 的反例（3.6"其他"）。
4. `modules/notebook`：
   - `domain/notebook.go`：`Notebook`（id、工作区、名称、开放程度、时刻）、`NewNotebook`；`errors.go`：`notebook.not_found`、同码的 `workspace.not_found`；
   - `adapter/postgres`：查询（插入笔记本与成员行、按 id 读、加锁读、改、软删除笔记本与成员行、列表、成员数、事实）与仓储；
   - `facts.go`：`NewFacts(pool)`，实现 access 的 `NotebookFacts`，经 `postgres.DB(ctx, pool)` 进入调用方的事务。
5. `modules/workspace`：`workspaces.go` 的 `NewWorkspaces(pool)`（`FindBySlug` 答 id、`ShareByID`，用已有的 `ShareWorkspaceByID`）。
6. 清理：`queries/purge.sql`、`purgers.go`；组合根 `purgers(pool)` 把笔记本的排在工作区之前；`bootstrap/purge_test.go` 加 `TestCrossModuleForeignKeysToPurgedTablesRestrict`，清理的整个程序测试带上笔记本。迁移一加上表，清理的注册表测试就要求它们有清理器，所以在这一步。
7. 组合根：`bootstrap/notebook_facts.go` 把 `notebook.Fact` 转为 `access.NotebookFact`，`access.Deps.Notebooks` 接上。

## 测试

- 仓储（真实数据库）：
  - 插入笔记本与管理员成员行，同一时刻；
  - 列表的过滤：已删除的笔记本、已结束的成员关系不算，访客没有默认角色，`none` 只给显式成员；排序：名称不分大小写，再名称，再 id；
  - 成员数只数有效的成员关系；
  - 加锁读：已删除的读到 0 行；别的事务持锁时等待；
  - 软删除笔记本连带全部成员行（含已结束的），同一时刻。
- 事实：显式角色、已结束的成员关系没有角色、已删除的笔记本不存在、在调用方的事务里读。
- `ShareByID`：事务之外报错；已删除的工作区答没有；slug 不合格式时 `FindBySlug` 不查库。
- schema：每个 CHECK 的反例被拒绝；约束与索引名与清单一致。
- 清理：保留期之前的删除、之后与边界上的不动；按批；跳过别的事务持有的行；成员行被跳过时笔记本留到下一次。
- 反向对照：
  - 列表不过滤已结束的成员关系、成员数数上已结束的、开放程度不看 `none`，列表的测试失败；
  - 事实不看 `ended_at`、不在调用方的事务里读，事实的测试失败；
  - 删除笔记本不连带成员行，删除的测试失败；
  - `FindBySlug` 不先判断 slug 的格式、`ShareByID` 不检查事务，`Workspaces` 的测试失败；
  - `notebooks.workspace_id` 改为 CASCADE，跨模块外键的检查失败；清理器的顺序反过来，顺序的检查与清理的整个程序测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
