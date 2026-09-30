# M1/P4/S3 管理用例：实施计划

上级：[P4 文档](../04-P4-admin-jobs.md) 3.6、3.11。

## 任务

1. 领域：`NewEmail(field, s)`（导出邮箱的检查）；`ErrEmailUnchanged`。
2. 查询：`LockUserByEmail`、`ChangeEmail`、`ActivateUser`、`RevokeAllAPITokens`（未撤销的，过期的也撤销）、`CountUsableAPITokens`；仓储方法，唯一冲突译为 `email_taken`。
3. `app/create_account.go`：注册与 `CreateUser` 共用的检查与哈希；注册改用它。
4. 用例：`CreateUser`、`ResetPassword`、`SetEmail`、`Activate`；`Deactivate` 增加按邮箱的入口（已停用则什么也不做）。
5. 登录：锁下读到的邮箱与找到账户时的不同，答 `invalid_credentials`。
6. 模块：`parts.go`（store、argon2、规则、锁）；`New` 的用例装配抽成辅助函数；`admin.go` 的 `NewAdmin`、`AdminDeps`、`Admin` 的五个方法与别名。

## 测试

- 用例：每个成功与每种失败；状态不变时的停用、启用不调用扩展点；重置撤销全部会话与 PAT。
- 仓储：每条新查询，其他账户不变。
- 交错（P4 文档 3.11）：重置与持锁的创建 PAT、与持锁的登录；重置之前校验旧密码的创建 PAT；改邮箱与登录。
- 模块：`NewAdmin` 只凭连接池构造，五个方法在真实数据库上各走一遍。
- 反向对照：重置先撤销后锁；登录不核对邮箱；已停用的再停用时跑扩展点。

## 完成检查

`make check`、`make gen-check` 为绿。
