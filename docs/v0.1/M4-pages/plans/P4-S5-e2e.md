# M4/P4/S5 端到端：实施计划

上级：[P4 文档](../04-P4-content-sessions.md) 3.12。

## 任务

1. `e2e/fixtures/pages.ts`：`putContent`、`getContent`、`openSession`、`heartbeat`、`endSession`，`createPage` 可带正文；`assert/page.ts`：`expectContentWritten`、`expectOneSessionRevision`、`expectSessionGone`。
2. 故事（接口版本）：`pg5-reading-view`、`pg6-render-safety`、`pg7-edit-save`、`pg8-save-conflict`、`pg9-byte-fidelity`、`pg10-edit-sessions`、`pg14-notebook-activity`；`pg12-page-permissions` 加正文与会话。页面版本在 P5、P6 加进同一文件。

## 测试

- 单个 spec：`make build`，再 `cd e2e && NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/page/<spec>`；最后 `make e2e`。
- 反向对照（服务端变异之后重新 `make build`）：带会话的写不用会话的变更集（PG7 失败）；不比较 `base_revision`（PG8 失败）；活动来源不登记（PG14 失败）。

## 完成检查

`make e2e` 全部通过；e2e 的 lint、format、types 与 `make knip` 为绿。
