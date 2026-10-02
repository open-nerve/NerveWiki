# M4/P4/S2 正文：实施计划

上级：[P4 文档](../04-P4-content-sessions.md) 3.4、3.6、3.8、3.9。

## 任务

1. `app/unit_content.go`：`ContentWrite`、`Unit.WriteContent`（P4 文档 3.4 的顺序；会话的部分在 S3 接上端口之前先以"没有会话"实现，S3 补全）；`appender.WriteContent`；`Step.EditSessionID`。
2. `app/content.go`：`checkedContent`（`CheckContent`、哈希、`Parse`，事务之前）；`unit_create.go` 与 `create_page.go` 带正文；`get_page_content.go`、`put_page_content.go`（日志 `page content written`）。
3. 平台：`APIConfig.BodyLimits`（`NewAPI` 核对为正，中间件取路由与全局中大的）与测试；`adapter/http/limits.go` 的 `BodyLimits()`、模块根的 `(*Module).BodyLimits()`，`wire.go` 交给 `apiConfig`。
4. 契约：`getPageContent`、`putPageContent`、`PageContent`、`PageContentWrite`、`PageCreate.content`；`make gen`；`handler.go` 两个处理函数，`handler_test.go` 答出新码。
5. 模块根接线；文案 `page.revision_mismatch`；矩阵两行（读正文给三种角色，写正文给编辑者）。

## 测试

- 用例（假的端口）：码的次序（正文的 422 先于 404、403；`base_revision` 不一致 409）；与当前正文相同不写、不调用守卫与观察者、不论 `base_revision`；写出的版本与哈希、字节数；改动带解析结果，守卫与观察者收到；新建带正文；`Appender.WriteContent` 经守卫、加版本、同一变更集、不调用参与者；日志。
- 平台：路由的上限放宽、全局更大时取全局、非正的值 `NewAPI` 报错。
- 反向对照：不比较 `base_revision`；相同正文仍写；取值检查放到判定之后；`BodyLimits` 不进中间件。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check`、前端的 lint、format、types 与 `make knip` 为绿。
