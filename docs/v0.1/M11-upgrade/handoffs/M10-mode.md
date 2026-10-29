```yaml
status: open
from: M10/P1
to: M11
created: 2026-10-27
```

# 模式的契约表、模式变化的订阅者与冻结的拒绝

M10/P1 建成了模式与模板、模式守卫的框架、预检与后台拒绝的去处，以及网页的模式变化处理（[P1 设计](../../M10-llm-wiki/01-P1-contract-frame.md)第 8–10 节；[S4 审查](../../M10-llm-wiki/reviews/P1-S4-web-review.md)）。这里移交 P1 实施后明确的契约与接入位置；M11 的升级流程尚未开始。原有的[事件移交](M5-events.md)、[回滚一次升级](M8-P4-background.md)与[MCP、冻结及回滚调度移交](M9-mcp-upgrade.md)继续适用。

1. **模板与模式的契约表**：`wiki/domain.ContractOf` 是同一份契约表；普通模式没有模板，LLM Wiki 的两种语言模板依次写来源根 `raw` 与 `AGENTS`。模块根的 `wiki.TemplatePages(mode, locale)` 从它取模板，交回副本；组合根与跨模块接入经模块根，不直接导入别的模块的 domain。升级读取已有契约，不另存一套模板或从当前笔记本的正文推断契约（`wiki/domain/contract.go`、`wiki/module.go`；[P1](../../M10-llm-wiki/01-P1-contract-frame.md) 3.8–3.9）。

2. **先写行，再在同一个事务里通知订阅者**：`notebook.ModeSubscriber.ModeChanged(ctx, ModeChange)` 在笔记本的行写成新模式之后调用，仍在写行的事务内；按登记次序调用，第一处错误停止并回滚整件事。`ModeChange` 带工作区、笔记本、前后模式、执行者与同一个修改时刻。M11 的模式变化也经过这条订阅链，不能只写行或在事务之后另发事件。当前降级经 `SetMode` 写行后再 `publishMode`；`TestADowngradePassesTheModeSubscribers` 在真库里核对订阅者读到新行、事务、审计列与失败回滚（`notebook/app/extension.go`、`notebook/app/downgrade_notebook.go`、`notebook/extension_test.go`）。

3. **沿用同一个 `mode` 事件**：组合根的 `notebookEvents.ModeChanged` 经事件发布者发布 `mode`，按笔记本过滤。发布数据是 `{}`，帧里的 `workspace_id`、`notebook_id` 由事件流补入；不把前后模式另放进帧。服务端最后一跳是 `TestTheModeEventReachesTheStreams`，契约是 `EventMode`。网页的真实 `modeChanged` 已登记，重读该工作区的笔记本列表；`app/event-stream.test.tsx` 经实际处理表核对最后一跳，通用扩展测试的编造类型已改为 `later`。M11 沿用它，其他升级状态事件的细节留给 M11；原[M5 事件移交](M5-events.md)在 M11 开工时一并处理。

4. **模式与来源根一次写入；普通模式可先指定根**：`notebook/adapter/postgres/queries/notebooks.sql` 的 `SetNotebookMode` 用一条 `UPDATE` 同时写 `mode`、`source_root_id`、`updated_by_id`、`updated_at`，经存储的 `SetMode` 在调用者的事务与行锁下使用。`00048_notebook_notebooks_mode.sql` 的检查实际是 `mode <> 'llm_wiki' OR source_root_id IS NOT NULL`：它要求 LLM Wiki 有来源根，**不要求普通笔记本没有来源根**，为 M11 先定来源根、后批准模式留出了位置。来源根没有外键，理由与清理次序见该迁移。当前普通创建不给来源根、降级清为 null，是这两个用例的行为，不能反推为更强的数据库约束。升级状态仍是 M11 自己的状态，不增加 `notebooks_mode_check` 的模式值；批准时实际的写入与事务编排由 M11 设计。

5. **冻结守卫的授权与拒绝沿用单元的边界**：`Write.Grants` 是单元自己的额外授权备忘，以指针随步骤传递，不能把答案留在共用的守卫上。当前由模式守卫与单元锁下的检查填写，其他扩展可读；锁外预检单独授权，不把答案带进单元。M11 的冻结扩展以这条边界为基础。拒绝经 `page/domain.Refused` 标出节点并 `Unwrap` 原问题；`batch` 的失败项、重新计划与后台任务都已有处理，模块根 `page.Refusal` 给组合根读码与节点，transfer 的 `Refused` 保留原问题。`TransferRefusal.code` 是自由字符串，新增冻结码不必扩这个字段的枚举；新操作的声明、错误文字与各入口的错误契约仍照通用约定落实（[P1](../../M10-llm-wiki/01-P1-contract-frame.md) 3.19、3.22、3.26；[M9 移交](M9-mcp-upgrade.md)第 2 条）。

`notebook.change_mode` 给发起与批准升级、根下保留标题与 `AGENTS` 的处理，以及冻结的次序、lint 的显式契约与来源范围、批准时哈希的重建、内容保全、说明和降级的关系，已有[M10 总设计](../../M10-llm-wiki/00-M10-design.md)第 7 节“写给后面的 M”和 [P1 第 9 节](../../M10-llm-wiki/01-P1-contract-frame.md)的要求。本移交指向它们，不把 P2–P5 后续才提供的能力写成 P1 已经实现。

### M10/P2补充（来源契约；升级仍未开始）

P2/S1–S2已完成来源事实/rewrite/hash；S3网页与后续集成见[P2结果](../../M10-llm-wiki/02-P2-source-area.md#103-s3-完整来源网页)。批准变为LLM时须在同事务重建指定根严格后代source_hash并清区外；普通模式虽可预指定SourceRootID，hash仍NULL。保全revision.rewrite，旧无标记/Down-Up按保守规则；SourceHashes、ModeSubscriber顺序与free refusal.code沿既有接口。此为已实现契约的交接，不设计升级流程；status仍open。SourceRootID旧过强注释Nit继续M10收尾，不顺手加强SQL约束。
