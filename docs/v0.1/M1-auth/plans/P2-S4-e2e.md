# M1/P2/S4 端到端：实施计划

上级：[P2 文档](../02-P2-sessions-ratelimit.md) 3.10。

## 任务

1. `e2e/fixtures/auth.ts` 增加 `login`、`refresh`（退出在故事中直接调用）；`assert/identity.ts` 增加按会话的断言。
2. 故事的接口版本：A3 登录、A4 续期、A5 重复使用检测、A6 退出、A14 登录限流（`nervewikiWith` 起小桶的服务）。
3. README 的账户与认证一节补登录、续期、退出与限流的配置。

## 测试

- `make e2e`，`--repeat-each 3` 与 `--workers 1` 各一遍；M0 与 A1、A2 的故事照旧通过。

## 完成检查

`make check`、`make e2e`、`make image-smoke` 为绿。
