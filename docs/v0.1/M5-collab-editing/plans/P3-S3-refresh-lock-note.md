# M5/P3/S3 `<EventStream/>` 与刷新、阅读视图的键、页面树的读、正在编辑与解除锁定：实施计划

上级：[P3 文档](../03-P3-event-stream-web.md) 3.8–3.10；[M5 总设计](../00-M5-design.md) 4.11。

## 任务

1. `app/event-stream.tsx`：会话 `signed-in` 且 hub 在时启动、清理时停止；`pages`、`lock`、`connected` 到 SWR 的路由；正文经重读的合并，比较缓存里的 `revision`。放进 `app/providers.tsx`。
2. 阅读视图的键改为 `["page-view", notebookId, pageId]`（`reading-view.tsx`、`page-edit.tsx`）。
3. `stores/page-tree.store.ts`：读按开始的次序生效。
4. `pages/page/edit-lock-note.tsx`：正在编辑（别人、自己）、管理员的解除锁定（确认对话框、失败的文案）、到期之后重读；`page-layout.tsx` 在不编辑时显示它。
5. 文案（中英）。

## 测试

P3 文档第 5 节的"组件"一项与页面树的单元测试。反向对照：连上之后不整体刷新；`revision` 不比较；页面树的读按答复覆盖。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿。
