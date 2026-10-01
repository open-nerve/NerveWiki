# M3/P2/S4 端到端：实施计划

上级：[P2 文档](../02-P2-notebook-members.md) 3.10。

## 任务

1. `e2e/fixtures/notebook-members.ts`：列、加、改、移出、离开；`assert/notebook.ts` 加 `expectNotebookMember`。
2. `n2-private-notebook.spec.ts`：第二位成员。
3. `n4-notebook-members.spec.ts`：N4 的接口版本。
4. `n5-leave-notebook.spec.ts`：N5 的接口版本。

## 测试

- 新故事 `--repeat-each 3` 通过。
- 反向对照：移出不写 `ended_at` → N4 的落库断言失败。

## 完成检查

`make e2e` 为绿。
