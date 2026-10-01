# M2/P3/S5 端到端：实施计划

上级：[P3 文档](../03-P3-invitations.md) 3.11。

## 任务

1. 夹具：`fixtures/invitations.ts`（邀请、列出、删除、预览、接受，接口版本）；`fixtures/assert/workspace.ts` 加邀请与成员关系的断言（令牌不在库里、接受之后邀请消费、成员关系结束之后邀请删除）。
2. 故事：W5、W6、W7、W8、W9 的接口版本；W4 补上非管理员的 403 与删除连带邀请。W7 用 `nervewikiWith` 关闭注册。
3. README：邀请一节（链接、令牌只在片段、换签名密钥之后链接失效）。

## 测试

- 新故事另跑 `--repeat-each 3`；全部故事通过。
- 反向对照：成员关系结束不删邀请，W9 失败。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
