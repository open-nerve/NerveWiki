```yaml
status: done
from: M3/P2
to: M5
created: 2026-10-02
```

# 可见性变化事件的第一个注册者

> 已全部处理（2026-10-04）：第 1 项是 `bootstrap/events_access_test.go` 的 `TestEveryVisibilityChangeResetsItsStreams`，每条路径一行（收尾时补了"笔记本对成员关闭"与"以管理员接受邀请"两行），把组合根交给可见性的订阅者换成空时失败；第 2 项：`Reached` 时 hub 按工作区重置流；第 3 项：订阅者只发 `NOTIFY`，随提交才送达；第 4 项：关闭事件流用不到执行者，不加。经[M5 收尾审查](../reviews/M5-closeout-review.md)核实。

M3/P2 建了可见性变化事件（[M3 总设计](../../M3-notebook/00-M3-design.md)第 4、8 节；[P2 文档](../../M3-notebook/02-P2-notebook-members.md) 3.6）：一个工作区、一组账户 id、"默认角色所及"的标记 `Reached`、时刻；在引起变化的写入之后、同一事务内调用，错误整体回滚。M3 没有注册者，组合根的 `notebookRegistrants()` 返回空集合，交给每一条发布路径；把 `deps.go` 里交给 workspace 的两组订阅者、交给 notebook 的可见性订阅者换成 nil，测试照样全部通过（[P2 审查](../../M3-notebook/reviews/P2-notebook-members-review.md) Q1，与 [M2 移交](../../M3-notebook/handoffs/M2-workspace.md)第 1 项、[M4 的移交](../../M4-pages/handoffs/M3-P1-notebook-deletion.md)同一种情形）。

M5 注册第一个订阅者（关闭这些账户的事件流）时：

1. **每条触发路径一个整个程序上的行为测试**，任一路径没交注册者时对应的测试失败：
   - 笔记本自己的（P2）：建笔记本（私密与开放）、`workspace_access` 跨过 `none`、添加与恢复成员、移出、离开；
   - 工作区的加入与角色变化（P2）：接受邀请插入新行（管理员与成员发，访客不发）、工作区角色跨过访客；
   - 工作区成员关系的结束与恢复（P3）：移出、离开、停用（接口与命令行）、接受邀请恢复、`workspaces reactivate-member`；
   - 无主（P3）：接管、归还。

   P3 的模块根已证明结束、恢复、接管在事务内、值对、失败回滚（`notebook/cascade_test.go`、`visibility_test.go`）；组合根 `registrants.go` 交给结束、恢复注册者的可见性订阅者换成 nil，测试照样通过（[P3 审查](../../M3-notebook/reviews/P3-cascade-ownerless-review.md) T1 的 R03），这一跳同样由上面的行为测试守住。
2. **`Reached` 为真时 `UserIDs` 可以为空**（开放程度跨过 `none`）：订阅者在同一个事务里按这个工作区的有效管理员与成员解析出账户。
3. **订阅者被调用之后，事务仍可能回滚**：添加笔记本成员、改工作区成员的角色在事件之后才读资料；接受邀请在加入事件之后才用掉邀请（与 M2 的恢复事件次序一致）。订阅者对外的效果必须随提交生效（例如 `NOTIFY`，或提交之后的钩子），不能在回调里直接关连接。
4. **值没有执行者**：关闭事件流用不到它。这是总体设计 13.1 第 21 条"事件的值带时刻与执行者"的例外，M3 总设计第 8 节写明；M5 需要执行者时，加字段并补齐每个触发点。
