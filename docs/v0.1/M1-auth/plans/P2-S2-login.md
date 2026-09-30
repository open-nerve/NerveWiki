# M1/P2/S2 登录：实施计划

上级：[P2 文档](../02-P2-sessions-ratelimit.md) 3.2、3.4、3.8。

## 任务

1. 查询：按邮箱读登录所需的账户、锁账户行（`FOR NO KEY UPDATE`）、改密码哈希；生成。
2. `app`：账户行锁的端口；`login.go`（哑哈希、事务外校验与重新哈希、锁下核对快照、重试一次）；错误码 `identity.invalid_credentials`、`identity.account_deactivated`。
3. 模块的桶：`limits.go`（`login_ip` 与 `login_ip_email` 全取或全不取、`register_ip`）；`register` 接入。
4. 契约：`login`（描述写明模块的桶）；生成；handler 改为 `UseCases` 加 `Settings`（模块的桶、日志）。

## 测试

- 用例：不存在的邮箱对哑哈希恰好校验一次；快照重试（替身在校验与加锁之间改哈希）。
- 真实数据库：成功、错误的密码、不存在的邮箱（同一个回答）、停用的账户、参数变化后重新哈希（并发时结果一致）；会话记录经可信代理的客户端地址。
- handler 测试答出每个声明的码；登录与注册的桶答 429。
- 反向对照：不存在的邮箱跳过哑哈希 → 测试失败；锁下不核对快照 → 重试测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
