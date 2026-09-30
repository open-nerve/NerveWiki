# M1/P3/S5 端到端：实施计划

上级：[P3 文档](../03-P3-accounts-tokens.md) 3.10。

## 任务

1. `e2e/fixtures/auth.ts`：`createToken`、按 PAT 调用的辅助。
2. `assert/identity.ts`：`api_tokens` 的断言（哈希与字段、撤销、`last_used_at`）；按原因的会话撤销。
3. 故事：A7、A8、A9、A10、A11 的 PAT 接口版本；A3 补停用账户的登录。
4. README：PAT 的用法、账户操作、`password_user`。

## 测试

- `make e2e` 全部通过；`--repeat-each 3` 与 `--workers 1` 各跑一轮，没有偶发失败。
- 过期用短期限与 `expect.poll`，不固定等待。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
