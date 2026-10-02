# M4/P2/S4 端到端：实施计划

上级：[P2 文档](../02-P2-tree-operations.md) 3.9。

## 任务

1. `e2e/fixtures/pages.ts`：`postMove`（答复）、`moveNode`（核对 200 之后返回节点）、`deleteNode`（答复）；`assert/page.ts`：`expectMoved`、`expectSubtreeDeleted`。
2. 故事：`pg3-move-page`、`pg4-delete-subtree`（管理员建页，工作区成员移动与删除，执行者才比较得出来）；`pg12-page-permissions` 加移动与删除。

## 测试

- 单个 spec：`make build`，再 `cd e2e && NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/page/<spec>`；最后 `make e2e`。
- 反向对照（服务端变异之后重新 `make build`）：`RecordItem` 不写 `after_parent_id`、`MoveNode` 不写审计列，PG3 失败；删除的条目不写 `deleted_at`，PG4 失败（`expectSubtreeDeleted` 核对删除自己的条目）。

## 完成检查

`make e2e` 全部通过；e2e 的 lint、format、types 与 `make knip` 为绿。
