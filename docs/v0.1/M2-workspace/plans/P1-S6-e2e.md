# M2/P1/S6 端到端：实施计划

上级：[P1 文档](../01-P1-access-workspaces.md) 3.11。

## 任务

1. `e2e/fixtures/assert/workspace.ts`：`workspaceRows(db, …)`、`memberRows(db, …)`，读数据库，只比较业务列。
2. `e2e/stories/workspace/w1-create-workspace.spec.ts`：PAT 接口版本。
   - 检查 slug 的四种答复；
   - 创建，落库；
   - 已占用答 409，保留的答 422；
   - 读取与列表的回答。
3. `e2e/stories/workspace/w2-creation-switch.spec.ts`：用 `nervewikiWith` 关闭创建。
   - 创建答 403，数据库不变；
   - `GET /instance` 答 `workspace_creation_enabled: false`。

## 完成检查

`make e2e` 全部通过（M0、M1 的故事照旧）；`--repeat-each=3` 稳定。
