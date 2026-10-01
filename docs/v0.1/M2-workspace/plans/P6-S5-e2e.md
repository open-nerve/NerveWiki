# M2/P6/S5 端到端：实施计划

上级：[P6 文档](../06-P6-web-members-invitations.md) 3.7。

## 任务

1. 夹具：`fixtures/member-pages.ts`、`fixtures/invitation-pages.ts`；剪贴板的权限。
2. 故事：W5–W10 加页面版本，落库断言与接口版本共用。

## 测试

- 新写的页面版本另跑 `--repeat-each 3`；全部故事通过。
- 反向对照：自己的一行有角色控件，W8 失败；注册不带邀请，W7 失败；停用对话框不换说法，W10 失败。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
