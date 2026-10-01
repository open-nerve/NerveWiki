# M2/P2/S3 接口与接线：实施计划

上级：[P2 文档](../02-P2-workspace-members.md) 3.4–3.7。

## 任务

1. `api/modules/workspace.yaml`：六个操作照 3.7；`WorkspaceMember`、`WorkspaceMemberList`、`WorkspaceUpdate`、`WorkspaceMemberUpdate`；参数 `WorkspaceMemberID`。`make gen`。
2. `adapter/http`：六个处理器；`WorkspaceMember` 的转换。
3. 模块根：`Deps` 加 `Profiles`、`MembershipEndVetoers`、`MembershipEndSubscribers`、`DeletionSubscribers`；扩展点与 `Profile` 的类型别名；`New` 装配六个用例。
4. 组合根：
   - `registrants.go`：`workspaceRegistrants()` 返回三组注册者，本 Phase 为空；
   - `deps.go`：`workspaceDeps` 交出注册者与经适配的 `identity.NewProfiles(pool)`（适配器在 `bootstrap/profiles.go`）。
5. 前端的文案表加三个新码，两种语言；`test/fakes.ts` 随类型更新。

## 测试

- HTTP（`apitest.Main`）：每个声明的码答出一次；204 没有正文；`WorkspaceMember` 的邮箱为空时是 `null`。
- 扩展点的替身测试（`workspace/extension_test.go`，真实数据库，`workspace.New` 装配，判定用一个按规则集合读 `NewMemberships` 的测试替身）：
  - 否决者在锁下读到已提交的状态；否决时成员关系不变，答出否决者的码；
  - 订阅者在同一事务里读得到 `ended_at`；订阅者失败时整体回滚；
  - 删除的订阅者读得到工作区的 `deleted_at`，与成员行的时刻相同；订阅者失败时什么都没有删除。
- 适配器：`bootstrap/profiles.go` 的转换由矩阵的成员列表行核对（显示名与邮箱）。
- 反向对照：`membershipEnder` 忽略否决者的错误，替身测试失败；删除不调用订阅者，替身测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
