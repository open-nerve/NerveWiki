# M5/P6/S4 端到端：C10 与 PG5：实施计划

上级：[P6 文档](../06-P6-task-items.md) 3.6；[M5 总设计](../00-M5-design.md) 第 3 节（C10）。

## 任务

1. `e2e/fixtures/pages.ts`：`toggleTask(api, credential, pageId, body)`。
2. `e2e/stories/collab/c10-tasks.spec.ts`：C10 的接口版本与页面版本（P6 文档 3.6）。
3. `e2e/stories/page/pg5-reading-view.spec.ts`：复选框带 `data-task`，阅读者的不能点。

## 测试

全量 e2e；C10 的页面版本另以 `--repeat-each` 压测。反向对照（`e2emut.py`）：增强不注册（C10 页面失败）；位置偏一个字节（C10 接口失败）；勾选不经守卫（持锁的那一步失败）。

## 完成检查

`make check`、`make e2e` 为绿。
