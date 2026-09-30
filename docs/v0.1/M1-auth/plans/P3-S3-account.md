# M1/P3/S3 账户操作：实施计划

上级：[P3 文档](../03-P3-accounts-tokens.md) 3.3–3.5、3.9。

## 任务

1. 领域：显示名的规则（去掉两端空白，1–100 个字符，不含控制字符）；引导步骤 id 的规则。
2. `updateMe`：空的补丁不写库；一条语句改显示名并返回账户。
3. `recordOnboardingStep`：一条语句，已记录的不改，否则追加；仓储把 `users_onboarding_steps_check` 的违反译成 422（`step`，`out_of_range`）。
4. `changePassword`：密码规则（邮箱取自账户）→ 当前密码 → 锁下写哈希、撤销其他会话（`password_changed`）。
5. 契约：`updateMe`、`recordOnboardingStep`、`changePassword`；handler；`changePassword` 经 `password_user`。
6. `interleavings_test.go`：可控的哈希器与写入闸门；交错一至三。

## 测试

- 用例：会话调用保留当前会话，PAT 调用撤销全部；规则错误先于密码校验；PAT 不受影响。
- 引导步骤：幂等且不改 `updated_at`；格式错误；第 33 个不同的步骤 422。
- 交错一至三（P3 文档 3.9），每个等待都有期限。
- 反向对照：改密码不锁账户行 → 交错一失败；`CredentialLock` 不复核 → 交错二失败。

## 完成检查

`make check`、`make gen-check` 为绿。
