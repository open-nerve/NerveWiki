# M2/P1/S2 权限框架：实施计划

上级：[P1 文档](../01-P1-access-workspaces.md) 3.4。

## 任务

1. `shared/authorize.go`：
   - `WorkspaceRole` 与三个值、`WorkspaceRoles()`；
   - `Action`、`Target`、`Grant`、`Authorizer`、`ErrNotVisible`。
2. `modules/access`：
   - `domain/rules.go`：`Level`、`Rule`、由函数返回的规则表，本 Phase 只有 `workspace.read`；`RuleFor`、`RuleKeys`；
   - `domain/decide.go`：`Decide`、`Facts`、`Membership`；
   - `app/ports.go`：`WorkspaceMemberships`；
   - `app/authorizer.go`；
   - `module.go`：`New(Deps)`、`RuleKeys()`。

## 测试

- `Decide` 的表格：三种角色 × 允许与不允许、不是有效成员、数据库 CHECK 之外的角色值。
- `RuleFor` 返回副本：改动它不影响规则表。
- `Authorizer` 的测试替身：
  - 读调用者在目标工作区里的角色，并在调用方的上下文里读；
  - 每次调用都读；
  - 没有规则时是内部错误，不碰端口；
  - 端口的错误原样返回。
- 架构测试：access 只导入 `shared` 与自己的层。

## 完成检查

`make check` 为绿。
