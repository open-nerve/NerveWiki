# M5/P6/S3 前端：阅读视图里的勾选：实施计划

上级：[P6 文档](../06-P6-task-items.md) 3.5；[M5 总设计](../00-M5-design.md) 4.12。

## 任务

1. `services/page.service.ts`：`toggleTask(pageId, {baseRevision, offset, checked})`。
2. `reading/enhancement.ts`：`ReadingContext` 加 `toggleTask(offset, checked)` 与 `report(error)`。
3. `reading/task-toggle.ts`：任务勾选的增强（能写的人放开、`preventDefault`、同时一个、撤销）；注册在 `readingEnhancements` 的最后。
4. `pages/page/reading-view.tsx`：给出 `toggleTask`（勾选、重读视图；409 `page.revision_mismatch` 重读；409 `page.locked` 重读锁）与 `report`（交给 `PageShell` 的提示）；换 HTML 前后保住 `[data-task]` 的焦点。`page-layout.tsx`：`ReadingView` 的报错接到 `refusal`。
5. `test/page-server.ts`：勾选的路由（改 `contents` 与 `views`，带锁与版本的判断）。

## 测试

P6 文档第 5 节"前端"各项：`task-toggle.test.ts`（假的上下文）、`reading-view` 的焦点与报错、经组合根的页面测试（注册表交空时失败）。反向对照：给阅读者放开；不 `preventDefault`；同时两个；焦点不回来；锁的拒绝不重读锁；没注册。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；新的组件测试另连跑 10 次。
