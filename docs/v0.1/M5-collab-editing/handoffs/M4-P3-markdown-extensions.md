```yaml
status: done
from: M4/P3
to: M5
created: 2026-10-03
```

# Markdown 扩展的约束：任务项是第一个注册者

> 已全部处理（2026-10-04）：第 1 项：任务项在 `platform/markdown/tasks`，架构规则不必改；第 2 项：两项检查都用 `markdownExtensions()` 的实例；第 3 项：`TestAReadingViewsCheckboxesCarryTheirPositions` 经 `newApp` 的阅读视图，`TestATaskToggleGoesByTheLockAndReachesTheStreams` 经守卫与观察者，`markdownExtensions()` 交空时都失败（`Extract` 的结果经 `Parsed` 到达守卫与观察者，由勾选的用例间接证明）；第 4 项：`ReadingContext` 加了 `toggleTask` 与 `report`（M5/P6）。经[M5 收尾审查](../reviews/M5-closeout-review.md)核实。

M5 的任务项（字节位置、勾选写回正文）是 `platform/markdown` 的第一个注册者（总体设计 12.4），比 M6 的 Obsidian 方言先到。扩展的注册、分隔符式语法、链接的两条约束、耗时与输出的检查、渲染函数、架构规则，都写在给 M6 的 [Markdown 的扩展](../../M6-links/handoffs/M4-P3-markdown-extensions.md)第 1–7 项，同样适用于 M5；M5 开工时照那份办，不另抄一份（[M4 收尾审查](../../M4-pages/reviews/M4-closeout-review.md) A-M1）。M5 先碰到的是：

1. **架构规则**（那份的第 6 项）：任务项的 `Extract` 要导入 goldmark 的 `ast`，先在 `archtest` 的 `markdownLibrariesStayInMarkdown` 给注册者开口子。
2. **耗时与输出**（第 4、5 项）：`TestTheAppsMarkdownCostsAboutItsSize` 与 `TestTheAppsMarkdownRendersCheckedHTML` 用注册了扩展的实例跑；任务项的标记写进 `Markup`。
3. **最后一跳**：经 `GET /api/v0/pages/{page_id}/view` 用组合根的实例（`bootstrap/wire.go` 的 `newApp`）证明扩展到达阅读视图，并证明 `Extract` 的结果经 `Parsed` 到达守卫与观察者；`bootstrap/markdown_app_test.go` 自己建实例，不算最后一跳。`markdownExtensions()` 交空时这些测试失败。
4. **阅读视图里的勾选**：`ReadingContext`（`web/apps/web/src/reading/enhancement.ts`）没有写入的手段，见[编辑器的移交](M4-P6-editor.md)第 5 项。
