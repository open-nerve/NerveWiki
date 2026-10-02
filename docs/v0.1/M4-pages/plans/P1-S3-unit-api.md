# M4/P1/S3 写入单元与接口：实施计划

上级：[P1 文档](../01-P1-page-module-pipeline.md) 3.5–3.9、3.11。

## 任务

1. `modules/page/domain/actions.go`：`node.list`、`page.read`、`page.create`、`node.rename` 与 `Actions()`；`access/domain/rules.go` 加 `writers()` 与四行；`bootstrap/actions_test.go` 的并集加 `page.Actions()`。
2. `modules/page/app`：
   - `ports.go`（`Workspaces.ShareByID`、`Notebooks`、仓储的端口、`Clock`）；`extension.go`（`WriteGuard`、`PageObserver`、`Participant` 与它们的值；`NotebookDeletion` 的注册者）；
   - `unit.go`（`Writer.Run`、`Unit`、`UnitSpec`、`Client`、`Options`；P1 文档 3.6 的顺序；已有事务时报错）；`authorize.go`；`view.go`；
   - 用例 `list_nodes.go`、`create_page.go`、`get_page.go`、`rename_node.go`，日志。
3. 模块根：`module.go`（`New(Deps)`、`Register`、`Actions()`、扩展点的类型别名）、`deletion.go`（`NewNotebookDeletion(pool)`）。
4. 契约：`api/modules/page.yaml`（四个操作；`TreeNode`、`Page`、请求体；码）、`api/openapi.yaml` 登记；手写 `adapter/http/gen/oapi-codegen.yaml`；`make gen`。
5. `adapter/http`：`handler.go`（客户端按 `Actor` 定；`parent_id` 的显式空；`after_id` 的省略与 `null`）、`main_test.go`（`apitest.Main`）、`handler_test.go`（每个码答出一次；id 不是 uuid 时 400）。
6. 组合根：`pageDeps`、`wire.go` 的注册；`registrants.go`：`notebookRegistrants(pool)` 交出 `pageNotebookDeletion`，两个调用处同步；`pageRegistrants()` 交空的守卫、观察者、参与者。
7. 前端文案：`problem-messages.ts`、`en.ts`、`zh-CN.ts` 加 `page.not_found`、`page.title_taken`（`page.too_deep` 若在本 Phase 的契约里出现也加；不要加 `page.locked`：文案测试拿它当没有文案的码）。
8. 矩阵的四行与种子的页面在本步的同一个提交序列里（`page.yaml` 一进入 `api/dist`，覆盖检查就要求它们；细节见 S4 计划的第 1 项，本步先加上能让覆盖检查通过的行）。

## 测试

- 用例（假的端口）：码的次序（404 → 403 → 422 → 409 → 守卫）；判定在锁之后（锁的端口记下调用的先后）；改名成同一个名字不写、不调用观察者；只差大小写的改名照常写；新建的次序（省略在最后、`null` 在最前、在某个兄弟之后）；层级上限；日志（id 在、标题不在；失败时没有日志）；一个单元只读一次时钟。
- 模块根（`extension_test.go`，接好线的模块与真实数据库）：守卫拒绝时整体回滚并答出它的码、看得到前后状态；观察者在事务里收到一次事件、出错回滚答 500；参与者追加的改名经守卫、进同一变更集、并进同一次事件；测试里的两操作单元一个变更集、一次事件；客户端（会话的访问令牌 `web`、PAT `api`）；`Run` 在已有事务里报错；笔记本删除的注册者以事件的时刻软删除。
- 契约：`apitest.Main` 核对每个声明的码。
- 反向对照：先调守卫再做检查（守卫看到未校验的标题的测试失败）；观察者每个操作调一次（两操作单元的测试失败）；参与者追加的写不经守卫；客户端写死 `web`。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check`、前端的 `check:lint`、`check:format`、`check:types`、`make knip` 为绿。
