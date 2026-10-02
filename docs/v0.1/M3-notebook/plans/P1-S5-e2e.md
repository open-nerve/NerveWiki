# M3/P1/S5 端到端：实施计划

上级：[P1 文档](../01-P1-notebooks-access.md) 3.12。

## 任务

1. `e2e/fixtures/notebooks.ts`：经接口建、改、删。
2. `e2e/fixtures/assert/notebook.ts`：`expectNewNotebook`、`expectNotebook`、`expectNotebookDeletedWithItsMembers`、`countNotebooks`；读数据库，只比较业务列，时刻在 SQL 里比较。
3. 故事：`e2e/stories/notebook/` 的 N1、N3、N6 的接口版本，N2 的私密部分，N13 的笔记本部分。

## 测试

- 每个故事断言回答与落库；N13 经数据库把时刻推到保留期的边界。
- 反向对照：`expectNotebookDeletedWithItsMembers` 在成员行没有软删除时失败（临时改用例，看到失败后恢复）。

## 完成检查

`make e2e` 为绿。
