# M3/P3/S3 无主的操作与审计：实施计划

上级：[P3 文档](../03-P3-cascade-ownerless.md) 3.3、3.4、3.7。

## 任务

1. 契约：`api/common.yaml` 的 `Limit`、`Cursor` 参数；`api/modules/notebook.yaml` 的四个操作、`OwnerlessNotebook`、`NotebookAuditEvent` 与列表的结构；`openapi.yaml` 注册路径。
2. 规则表：`notebook_ownerless.list`、`notebook_ownerless.take_over`、`notebook_ownerless.delete`、`notebook_audit.list`，工作区级，只给管理员。
3. 用例：`ListOwnerlessNotebooks`、`TakeOverNotebook`、`DeleteOwnerlessNotebook`、`ListNotebookAuditEvents`；按 id 的两个把 403 与看不到都转为 `notebook.not_found`；删除无主复用删除笔记本的写与事件。
4. 笔记本的活动：`NotebookActivity`、`NotebookActivitySource`，`Deps.ActivitySources`，组合根 `notebookExtensions.activitySources`（M3 为空）。
5. HTTP 与生成的代码；文案（没有新码）。
6. 矩阵：新列"无主笔记本的剩余成员"与种子 `orphan`；四行与三种变体；P1、P2 各行补这一列。

## 测试

- 用例：判定之前不读无主；锁下不再是无主 → 404；接管的三种情形（没有行、已结束、有效的较低角色）；审计的值；活动的合并（各来源的字节相加，时刻取最晚，没有来源时为 `updated_at` 与 0）。
- 存储：清单的次序（`ownerless_since`，再 `id`）；审计分页的次序与边界（同一时刻的两行按 `id`）。
- 矩阵：第 3.7 节。
- 反向对照：清单给成员看 → 矩阵失败；按 id 的操作对成员答 403 → 矩阵失败；接管不清除无主 → 不变量失败；分页的比较漏掉 `id` → 同一时刻的分页测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
