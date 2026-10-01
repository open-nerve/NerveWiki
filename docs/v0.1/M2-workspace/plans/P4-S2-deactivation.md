# M2/P4/S2 停用的注册者：实施计划

上级：[P4 文档](../04-P4-deactivation-commands-purge.md) 3.1、3.2。

## 任务

1. `MembershipEnder`：`Veto(ctx, e)`、`Write(ctx, e, email)`；`End` 由两者组成，移出与离开不变。
2. `app/deactivation.go`：`Deactivated{UserID, Email, At}`；`Deactivation` 的 `VetoDeactivation`（锁工作区 → 锁下重读 → 规则二 → 成员身份结束的否决者）与 `AccountDeactivated`（重读 → `Write`）；没有有效成员关系时什么都不做。
3. 模块根 `deactivation.go`：`NewDeactivation(pool, endVetoers, endSubscribers)`。
4. 组合根：`deactivationRegistrants(pool)`；适配器 `deactivation`；`identityDeps` 与 `Users` 传入连接池。
5. 契约：`deactivateMe` 的 `x-problem-codes` 加 `workspace.sole_admin`；identity 的 HTTP 测试用替身用例返回否决者的错误，答出它。
6. 组合检查：`Users` 与 `newApp` 到达 `workspaceRegistrants`。

## 测试

- 用例（替身）：锁在判定之前；规则二拒绝时没有写；否决者在写之前，拒绝时答出它的码；订阅者在写之后；邮箱取自停用；空集合不调用注册者。
- 模块根（真实数据库）：停用经注册者结束他的成员关系、删除发给他邮箱的待接受邀请；成员身份结束的替身否决者让整个停用回滚（模块测试不运行 identity：账户仍可用、会话仍在由整个程序上的规则二测试核对）。
- 整个程序（M1 移交第 1 项）：经 `POST /me/deactivate` 答 409 `workspace.sole_admin`，会话照旧可用；经 `Users(… DeactivateUser …)` 失败、原因带 slug，账户照旧可用；他只有一人的工作区不挡停用。
- 反向对照：`deactivationRegistrants` 返回空，整个程序的两条测试失败；规则二恒为允许，用例与整个程序的测试失败；订阅阶段不删邀请，模块根的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
