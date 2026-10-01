# M2/P1/S4 接口与用例：实施计划

上级：[P1 文档](../01-P1-access-workspaces.md) 3.7、3.8。

## 任务

1. 契约：
   - `api/modules/workspace.yaml`：四个操作与 `Workspace`、`WorkspaceList`、`WorkspaceCreate`、`SlugAvailability`；
   - 在 `api/openapi.yaml` 登记；
   - `instance.yaml` 加 `workspace_creation_enabled`；
   - `make gen`。
2. `workspace/app`：
   - `ports.go`：仓储、`Accounts`（`ShareActiveAccount`）、`shared.Authorizer`、`TxManager`、时钟；
   - `create_workspace.go`、`get_workspace.go`、`list_workspaces.go`、`check_slug.go`。
3. `workspace/adapter/http`：处理器、`main_test.go`（`apitest.Main`）。
4. `workspace/module.go`：`Deps`、`New`、`Register`、`PublicOperations`、`Actions()`。
5. 配置：
   - `workspace.creation_enabled`：内置的 `config.yaml` 带注释，`LogValue` 列出；
   - instance 的 `Deps` 加同一个值。
6. 组合根：`deps.go` 加 `workspaceDeps`；`wire.go` 装配 access 与 workspace，挂上路由。
7. `bootstrap/actions_test.go`：各模块 `Actions()` 的并集 = `access.RuleKeys()`。
8. e2e S3 的实例信息断言加 `workspace_creation_enabled`。

## 测试

- 用例：
  - 创建：开关关闭答 403，先于取值的 422；取值的 422 一次列全；`ShareActiveAccount` 的拒绝原样答出；slug 冲突答 409；成员行的角色为管理员；日志只有 id。
  - 读取：`ErrNotVisible` 译成 `workspace.not_found`。
  - 检查 slug：四种答复。
- HTTP：每个声明的码经 `apitest.CheckResponse` 答出一次；`identity.account_deactivated` 经端口的替身答出。
- 整个程序的测试自动覆盖新操作，包括 PAT 与 400 的用例。
- 配置：默认值、`LogValue`。
- 反向对照：
  - `getWorkspace` 不经 `Authorize`，"从来不是成员"的测试失败；
  - 去掉规则表的一行，`actions_test` 失败。

## 完成检查

`make check`、`make gen-check` 为绿。
