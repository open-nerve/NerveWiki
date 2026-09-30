# M1/P1/S5 端到端：实施计划

上级：[P1 文档](../01-P1-identity-foundation.md) 3.10。

## 任务

1. `e2e/fixtures/auth.ts`：`emailFor`、`register`、`bearer`。
2. `e2e/fixtures/assert/identity.ts`：`expectNewAccount`、`expectNewSession`（在测试侧解析刷新令牌，核对代数与哈希）、`expectNothingAdded`。
3. `e2e/stories/identity/a1-sign-up.spec.ts`、`a2-sign-up-refused.spec.ts` 的接口版本；注册关闭用 `nervewikiWith` 起一个关闭注册的服务。
4. S3 的断言加上 `signup_enabled`；README 的账户一节（注册开关、签名私钥）。

## 测试

- `make e2e` 本地通过，`--repeat-each 3` 与 `--workers 1` 也通过。
- 反向对照：注册不写会话 → A1 失败。

## 完成检查

`make check`、`make e2e` 为绿；持续集成为绿。
