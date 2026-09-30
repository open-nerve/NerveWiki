# M1/P4/S4 命令行：实施计划

上级：[P4 文档](../04-P4-admin-jobs.md) 3.6–3.8、3.11。

## 任务

1. `cmd/nervewiki`：`run` 增加标准输入；`users` 与五个子命令（`--email` 必填，`set-email` 另要 `--new-email`）。
2. 读取密码：终端经小接口（不回显、两次、ctx 取消时恢复终端）；否则读一行，只去掉行尾。`golang.org/x/term` 进 require。
3. bootstrap：`users.go`（配置 → `awaitDatabase` → `NewAdmin` → 执行 → 一行输出）；`registrants.go`（serve 与命令行共用）；`users.go` 的 `commandError`。
4. `archtest/composition_test.go`：从 `bootstrap.Users` 的静态调用图。

## 测试

- 命令：五个命令依次作用于一个账户，另一个账户不变；行尾；argon2 参数来自配置；每种失败；只输入 `users`。
- 读取密码：管道；终端替身的两次不同与 ctx 取消。
- 没有机密：debug 日志下 `create`、`reset-password` 的输出与日志查不到密码、哈希与派生密钥的六种写法。
- 组合检查；在 `-race` 下记录耗时。
- 反向对照：`Users` 调用 `identity.New`；把密码写进日志。

## 完成检查

`make check`、`make gen-check` 为绿。
