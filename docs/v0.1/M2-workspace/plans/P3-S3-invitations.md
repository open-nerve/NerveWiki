# M2/P3/S3 邀请的用例、接口与矩阵：实施计划

上级：[P3 文档](../03-P3-invitations.md) 3.3–3.5、3.7、3.10。

## 任务

1. `app/extension.go`：`MembershipRestore`、`MembershipRestoreSubscriber`。
2. 五个用例，各自只依赖要用的端口，顺序照 3.3；令牌在任何查询之前核对；日志不记令牌与邮箱。
3. `MembershipEnder`：否决者之后、写成员行之前，删除发给这个账户邮箱的待接受邀请；`DeleteWorkspace` 在成员行之前删除工作区的邀请。
4. 契约：五个操作与它们的结构；`WorkspaceMember.created_at` 的描述；`make gen`。
5. HTTP 适配器；模块根的 `Deps`、`PublicOperations()`；组合根并进公开操作、交出恢复事件的注册者（空）。
6. 前端的文案表加两个新码，两种语言。
7. 矩阵：三个操作的行；种子加邀请；豁免加"凭所持判定"一类与反例。

## 测试

- 用例：照 P3 文档第 5 节的单元一行。
- 扩展点：恢复事件的替身测试；成员关系结束删除邀请、工作区删除连带邀请（真实数据库）。
- HTTP：每个声明的码答出一次；预览不带令牌也能调用（公开）。
- 反向对照：接受不比较邮箱，用例测试失败；`MembershipEnder` 不删邀请，替身测试失败；规则表的创建邀请加上成员，矩阵失败。

## 完成检查

`make check`、`make gen-check` 为绿。
