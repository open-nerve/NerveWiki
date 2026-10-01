# M2/P5/S3 工作区设置：实施计划

上级：[P5 文档](../05-P5-web-shell-workspaces.md) 3.6。

## 任务

1. `pages/workspace/settings-layout.tsx`：导航（常规）与内容；`/:slug/settings` 转到 `general`；左栏的"设置"链接。
2. `pages/workspace/general-page.tsx`：管理员的改名表单（"已保存"一直挂载、只换文字）；非管理员只读；slug 只读；删除的区块。
3. `ConfirmDialog` 的 `typedConfirmation: { label, value }`：输入一致之前确认不可用，关闭时清空。
4. 删除成功之后转到 `/`。
5. 文案（两种语言）。

## 测试

- 改名：本地检查、422 在字段下方、已保存、左栏的名称随之更新。
- 删除：输入 slug 之前按钮不可用；成功之后到落点；403、404 时对话框保持并显示原因；关闭之后输入清空。
- 非管理员：只读的名称，没有删除。
- 反向对照：删除不要求输入、非管理员看到改名表单，各自的测试失败。

## 完成检查

`make check` 为绿。
