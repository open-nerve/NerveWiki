# M4/P4/S3 编辑会话：实施计划

上级：[P4 文档](../04-P4-content-sessions.md) 3.4、3.5、3.7、3.9。

## 任务

1. `app/extension.go`：`EditSessionVetoer`、`EditSessionSubscriber`、`SessionOpening`、`SessionEnded`；`WriterDeps` 加会话的端口、否决者与订阅者；`NotebookDeletion` 删会话并告诉订阅者。
2. `app/unit_session.go`：`Unit.OpenSession`；`ended`（删掉的会话里活着的，以原因调用订阅者）。`Unit.WriteContent` 接上会话：锁会话行、检查、会话的变更集与"夹进别人的写另起一个"、`TouchChangeset`、`SetSessionWrite`。`Unit.Delete` 删子树各页的会话。
3. 用例：`open_edit_session.go`、`heartbeat_edit_session.go`、`end_edit_session.go`、`cleanup_edit_sessions.go`；日志。
4. 定时任务：`adapter/river/cleanup.go`（照 identity）；配置 `page.edit_session_cleanup_interval`（`PageConfig`、校验至少 1 秒、`LogValue`、各 profile）；模块根的 `Jobs()`，`wire.go` 把它交给任务的运行器（运行器移到模块建好之后）。
5. 模块根：`Deps` 的新字段；`NewNotebookDeletion(pool, subscribers)`；`registrants.go` 的 `pageRegistrants()` 加空的否决者与订阅者，`notebookRegistrants(pool)` 从它取订阅者。
6. 契约：三个操作与 `EditSession`；`make gen`；HTTP；文案 `page.edit_session_ended`、`page.edit_session_not_found`；矩阵三行（心跳与结束另有"别人的会话"）。

## 测试

- 用例（假的端口与假时钟）：开启的码与否决者（在正文行的锁之后调用，错误答出、没有会话）；租约的时钟（开启的到期、恰好到期即过期、到期前一刻的心跳从那一刻起算、结束过期的 404）；心跳的码的次序（别人的、过期、看不到笔记本 404，阅读者 403）；结束不判定、只看本人；带会话的写：连续保存一个变更集与一个版本行（`base_revision` 是第一次的）、夹进别人的写另起一个、别人的与别的页的与过期的会话 409；删除子树与笔记本删除时订阅者只收活着的会话、原因与执行者；清理分批、不调用订阅者、删了才记日志。
- 模块根的 `extension_test.go`：否决者与订阅者的测试替身经 `Deps` 到达。
- 反向对照：会话的变更集不看 `revision`；会话的检查各去掉一项；删除不删会话；订阅者收到过期的会话；否决者在加锁之前调用。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check`、前端的 lint、format、types 与 `make knip` 为绿。
