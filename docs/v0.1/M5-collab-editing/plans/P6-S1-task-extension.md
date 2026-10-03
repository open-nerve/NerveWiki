# M5/P6/S1 任务项扩展：实施计划

上级：[P6 文档](../06-P6-task-items.md) 3.2；[M5 总设计](../00-M5-design.md) 4.12；[M4/P3 给 M6 的移交](../../M6-links/handoffs/M4-P3-markdown-extensions.md) 第 1–7 项。

## 任务

1. `platform/markdown/tasks`：`Extension()` 答 `markdown.Extension{Name: "tasks"}`：行内解析器（照 goldmark 的正则与位置条件，节点 `Task{Offset, Checked}`）、渲染器（`data-task`）、`Extract`（`[]Task`）、`Markup`（`input` 的 `data-task`）。
2. `internal/harden`：去掉 goldmark 的任务项解析器与渲染器；`harden_test` 的比较加上 goldmark 的任务项；`render_test`、`markdowntest` 的任务项样例带上 `tasks`。
3. `bootstrap/registrants.go`：`markdownExtensions()` 答 `tasks.Extension()`。
4. `markdowntest.Pathological` 加满是任务项的输入；`tools/md-fixtures/cases` 加任务项的样例（`make gen` 若重新生成它们的期望）。
5. 最后一跳：`bootstrap` 里经 `newApp` 读阅读视图，复选框带 `data-task`、位置对。

## 测试

P6 文档第 5 节"单元"的解析与渲染各项；`TestTheAppsMarkdownRendersCheckedHTML`、`TestTheAppsMarkdownCostsAboutItsSize`、harden 的比较为绿。反向对照：位置偏一个字节；渲染不带 `data-task`；`markdownExtensions()` 交空（最后一跳失败）；harden 留着 goldmark 的解析器（扩展的节点不出现：位置的测试失败）。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿；PG5 的 e2e 在 S4 改。
