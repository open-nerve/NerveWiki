# M1/P4/S6 端到端：实施计划

上级：[P4 文档](../04-P4-admin-jobs.md) 3.12。

## 任务

1. `e2e/fixtures/server.ts`：`runNervewiki` 接受标准输入与额外的环境变量。
2. `fixtures/users.ts`：`nervewikiUsers`、`nervewikiUsersFails`，以及查不到密码的断言（四种写法）。
3. 故事：A11 的命令行一段、A12、A13。
4. README：`nervewiki users`、后台任务、会话清理。

## 测试

- `make e2e` 全部通过；`--repeat-each 3` 与 `--workers 1` 各跑一轮。
- 清理用 `expect.poll`，不固定等待。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
