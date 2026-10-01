# M3/P5/S3 对话框与说明：实施计划

上级：[P5 文档](../05-P5-web-ownerless.md) 3.4。

## 任务

1. `pages/workspace/members-page.tsx`：离开工作区的 `notebook.sole_admin` 文案（`texts`）；移出的说明加上无主；移出之后重读无主清单（管理员）。
2. `pages/settings/deactivate-dialog.tsx`：停用的 `notebook.sole_admin` 文案。
3. `pages/notebook/members-page.tsx`：成员里没有管理员时的说明；工作区管理员另有到无主页的链接。
4. 文案。

## 测试

- 离开工作区：`notebook.sole_admin` 与 `workspace.sole_admin` 各自的文案。
- 停用：两种文案。
- 移出：说明里有无主；移出之后无主清单重读。
- 笔记本成员页：没有管理员时的说明，管理员有链接、成员没有。
- 反向对照：离开的文案去掉 → 测试失败；没有管理员的判断写成"成员为空" → 测试失败。

## 完成检查

`make check` 为绿。
