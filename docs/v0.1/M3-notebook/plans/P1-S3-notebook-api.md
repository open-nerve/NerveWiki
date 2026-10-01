# M3/P1/S3 接口与用例：实施计划

上级：[P1 文档](../01-P1-notebooks-access.md) 3.7、3.8、3.9。

## 任务

1. 契约：`api/modules/notebook.yaml` 的五个操作与 `Notebook`、`NotebookList`、`NotebookCreate`、`NotebookUpdate`（实际没有 `minProperties: 1`：空的请求体答 200，P1 文档 3.7）；在 `api/openapi.yaml` 登记；`make gen`。
2. `notebook/app`：
   - `ports.go`：仓储、`Workspaces`（`FindBySlug`、`ShareByID`）、`shared.Authorizer`、`TxManager`、时钟；
   - `extension.go`：`NotebookDeletion`、`NotebookDeletionSubscriber`；
   - `authorize.go`：`ErrNotVisible` 按调用方点名的东西译成 404；
   - 五个用例；工作区删除的注册者（实际在 `extension.go`）。
3. `notebook/adapter/http`：处理器、`main_test.go`（`apitest.Main`）。
4. 模块根：`Deps`、`New`、`Register`；`NewWorkspaceDeletion`；`Actions()` 已在 S1，`NewFacts`、`Purgers` 已在 S2。
5. 组合根：`notebookDeps`；`workspaceRegistrants` 的删除订阅者加上笔记本的。
6. 前端的文案表：`notebook.not_found` 的中英文案。
7. 工作区删除的订阅者经 `deleteWorkspace` 的整个程序行为测试（M2 移交第 1 项的这一条路径）。

## 测试

- 用例：
  - 列表：不是成员 404；每一项的有效角色；
  - 创建：访客 403；取值的 422 在锁与判定之后，一次列全；回答带管理员与一个成员；日志只有 id；
  - 读取、改、删除：`ErrNotVisible` 译成 `notebook.not_found`；改的请求没有变化时不写；
  - 删除：成员行同一时刻软删除；订阅者在事务内，失败整体回滚。
- 工作区删除的注册者：笔记本与成员行以事件的时刻软删除；有笔记本时调用订阅者一次，带全部 id；没有时不调用。
- HTTP：每个声明的码经 `apitest.CheckResponse` 答出一次；整个程序的测试自动覆盖新操作。
- 反向对照：
  - 订阅者在提交之后调用，回滚的测试失败；
  - 组合根不交工作区删除的订阅者，整个程序的行为测试失败；
  - 去掉 `updateNotebook` 的工作区 `FOR SHARE`，交错 16 在 S4 失败（S4 记录）。

## 完成检查

`make check`、`make gen-check` 为绿。
