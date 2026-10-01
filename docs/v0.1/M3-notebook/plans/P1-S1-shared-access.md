# M3/P1/S1 共用的规则与权限的笔记本级：实施计划

上级：[P1 文档](../01-P1-notebooks-access.md) 3.2–3.4。

## 任务

1. `shared`：
   - `title.go`：`CheckTitle(field, s)`；
   - `notebook_role.go`：`NotebookRole` 与三个值、`NotebookRoles()`；`WorkspaceAccess` 与三个值、`WorkspaceAccesses()`；`EffectiveNotebookRole`；
   - `authorize.go`：`Target.NotebookID`、`Grant.NotebookRole`。
2. `modules/access`：
   - `domain/rules.go`：`LevelNotebook`、`Rule.Notebook`；本 Phase 的五行；
   - `domain/decide.go`：`Facts.Notebook`（`Notebook{Found, Access, Role}`）、笔记本级的判定；
   - `app/ports.go`：`NotebookFacts`、`NotebookFact`；
   - `app/authorizer.go`：笔记本级先读笔记本，工作区与目标不同时当作不存在，再读工作区的角色；
   - `module.go`：`Deps.Notebooks`。
3. `modules/notebook`：`domain/actions.go` 与模块根的 `Actions()`；`bootstrap/actions_test.go` 的并集加上它（规则表的五行与操作名同一步加，一致性测试一直是绿的）。

## 测试

- `CheckTitle` 的表格（P1 文档 3.2）。
- `EffectiveNotebookRole` 的表格：三种显式角色与空 × 三种开放程度 × 三种工作区角色，三个值以外的值。
- 规则表：每一行的级别已知，角色是本级别的，不带另一级别的角色。
- `Decide` 笔记本级：显式角色、默认角色、两者取高、没有角色（含工作区管理员）、访客、不存在（带着角色也不算）、工作区成员关系已结束。
- `Authorizer`：笔记本级先读笔记本、再读工作区，都在调用方的上下文里；路径的工作区不是笔记本的工作区时不可见；工作区级不读笔记本；两个端口的错误原样返回。
- 反向对照：
  - 去掉 `Found` 的判断，"不存在"的用例失败；
  - 去掉工作区的比较，"经别的工作区"的用例失败；
  - 默认角色给了访客，`EffectiveNotebookRole` 的表格失败；
  - 没有角色时答 403，"不可见"的用例失败；
  - 工作区级也读笔记本，"工作区级不读笔记本"失败；
  - 规则表的笔记本行带工作区角色、或者带三个以外的角色，规则表的测试失败；
  - 并集少一个操作名，`actions_test` 失败。

## 完成检查

`make check` 为绿。
