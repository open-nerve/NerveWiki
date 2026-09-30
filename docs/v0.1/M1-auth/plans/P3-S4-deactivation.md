# M1/P3/S4 停用与扩展点：实施计划

上级：[P3 文档](../03-P3-accounts-tokens.md) 3.6、3.9。

## 任务

1. `app/deactivation.go`：`Deactivation{UserID, Email, At}`、`DeactivationVetoer`、`DeactivationSubscriber`。
2. `app/deactivate.go`：`CredentialLock` → 否决者 → 停用与撤销全部会话（`deactivated`）→ 订阅者，一个事务；写入部分单独成方法，供 P4 的命令行复用。
3. `app/accounts.go` 与模块根的 `accounts.go`：`ShareActiveAccount`（`FOR SHARE`，停用 403、不存在 404），`NewAccounts(pool)`。
4. 模块：`Deps` 加否决者与订阅者；模块根以类型别名公开扩展点的类型；bootstrap 传空列表。
5. 契约：`deactivateMe`；handler。

## 测试

- 用例：否决者按顺序调用，第一个否决即停止、不写入；订阅者收到锁下的邮箱与用例的时刻；订阅者失败即返回错误。
- 真实数据库（测试替身只凭连接池构造）：否决答出它的码且账户、会话不变；订阅者在事务内看到 `is_active = false`；订阅者失败整体回滚。
- `FOR SHARE` 的两个方向（P3 文档 3.9），`WaitForLockWaits` 强制顺序。
- 停用之后：会话的访问令牌、刷新令牌（续期不读 `is_active`，靠撤销失效，P2 审查 N3）、PAT 都答 401，登录答 403。
- 反向对照：订阅者在提交之后调用 → 回滚的测试失败；`ShareActiveAccount` 不取 `FOR SHARE` → 交错失败。

## 完成检查

`make check`、`make gen-check` 为绿。
