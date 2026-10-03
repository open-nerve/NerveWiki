# M5/P6/S2 勾选：用例、接口与最后一跳：实施计划

上级：[P6 文档](../06-P6-task-items.md) 3.3、3.4；[M5 总设计](../00-M5-design.md) 4.4、4.12、第 5、8 节。

## 任务

1. `page/domain`：`ActionToggleTask = "page.toggle_task"`；`Flip(content, offset, checked)`。`access/domain/rules.go`：同写正文的规则行。
2. `page/app`：`Markdown.Tasks(Parsed) []Task`；`ToggleTask` 用例（P6 文档 3.3 的次序；第二次解析不再判定）；日志。`adapter/markdown`：`Tasks`。
3. 契约：`api/modules/page.yaml` 的 `toggleTask` 与请求体，`api/openapi.yaml` 的路径；`make gen`。
4. `adapter/http`：处理器、`UseCases` 的字段、`module.go` 的接线；处理器测试里每个码各答一次。
5. 权限矩阵：种子加一页带任务项的，`toggleTask` 一行（写）。
6. 最后一跳（`bootstrap`，`newApp`）：勾选经守卫（他人持锁、本人在别处：409 `page.locked`）与观察者（`pages` 事件）；`markdownExtensions()` 交空时勾选答 422。
7. 前端的问题文案：码都已有文案（`problem-messages.test.ts` 核对）。

## 测试

P6 文档第 5 节"用例"的表格测试，"整个程序"的守卫与观察者。反向对照：422 排在 409 之前；已经是那个状态照样写；不判断新正文；勾选不经守卫；`Flip` 改了别的字节。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check` 为绿。
