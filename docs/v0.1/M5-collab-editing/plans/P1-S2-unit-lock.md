# M5/P1/S2 单元、用例与锁：实施计划

上级：[P1 文档](../01-P1-edit-lock.md) 3.5。

## 任务

1. `app/extension.go`：`SessionOpening.TakeOver`；`SessionOpened` 与 `EditSessionSubscriber.EditSessionOpened`；注释写明 M5 的形状。
2. `app/unit_session.go`：`OpenSession(ctx, id, takeOver)` 的四步；`Unlock(ctx, id)`；`tellEnded` 的"更新之前都活着"的变体。
3. `app/unit_content.go`：`writersSession` 的次序（本人、客户端、页 → 墓碑 → 活着）。
4. 用例：`open_edit_session.go` 带 `takeOver`；`heartbeat_edit_session.go`、`end_edit_session.go` 认出墓碑；新的 `get_edit_lock.go`、`release_edit_lock.go`；日志（强制解锁记执行者与结束的会话数）。
5. `app/edit_lock.go`：`EditLock`（否决者与守卫）；模块根 `edit_lock.go` 的 `NewEditLock(pool, names)`；`Deps.Names`。
6. 组合根：`pageRegistrants(pool)`（锁是否决者与守卫）；`notebookRegistrants(pool)` 与 `pageDeps` 从它取；`page_names.go` 把 identity 的目录转成 `page.Names`。

## 测试

- 用例（假端口与假时钟）：P1 文档第 5 节"用例"各条。
- 守卫的表格测试（`app/edit_lock_test.go`）：写正文带与不带会话、持锁的是本人与别人；删除时子树里本人与别人的、按层的第一个；改名、移动、新建放行；时刻取 `Step.At`。
- 模块根的 `extension_test.go`：订阅者的"开启"经 `Deps` 到达。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿。
