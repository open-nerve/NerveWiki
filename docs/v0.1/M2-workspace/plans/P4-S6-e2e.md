# M2/P4/S6 端到端：实施计划

上级：[P4 文档](../04-P4-deactivation-commands-purge.md) 3.6、3.7。

## 任务

1. 夹具：`fixtures/users.ts` 的命令运行泛化为任意子命令（`nervewiki workspaces` 共用），断言放进 `fixtures/assert/workspace.ts`（成员关系的结束与恢复、清理之后的行）。
2. 故事：W2 加命令行版本；W10（接口、命令行）；W12（后台任务）。
3. README：`nervewiki workspaces` 两条命令；停用与规则二、恢复的步骤；清理的保留期与间隔。
4. `docs/v0.1/M7-assets-transfer/handoffs/M2-P4-insert-only-client.md`。

## 测试

- 新故事另跑 `--repeat-each 3`；全部故事通过。
- 反向对照：规则二恒为允许，W10 失败；清理不看保留期，W12 失败。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
