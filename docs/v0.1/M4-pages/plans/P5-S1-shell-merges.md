# M4/P5/S1 外壳与合并：实施计划

上级：[P5 文档](../05-P5-tree-reading.md) 3.2、3.3。

## 任务

1. 外壳的 `main`：`WorkspaceLayout` 的路由带 `handle: { shell: true }`；`Layout` 用 `useMatches()` 判断，有外壳时不包 `main`；外壳的内容区是 `<main>`，左栏是 `<div>`。W3 的断言改为内容区的 `main`。
2. 合并成员行：`app/member-row.tsx`，参数是角色列表、文案前缀、移除的标题与正文；工作区与笔记本的成员页改用它，删掉两份旧的。
3. 合并改名表单：`app/rename-form.tsx`，参数是检查、`fieldTexts`、提示、`autoComplete`、提交与文案；工作区与笔记本的设置页改用它。
4. 有效角色：`app/effective-role.ts`（`shared.EffectiveNotebookRole` 的规则）；笔记本成员页读工作区成员列表，显式角色低于有效角色时注明；`test/notebook-server.ts` 的规则改用同一个函数。文案的中英两份。

## 测试

- 原有的成员页、设置页、布局的测试照旧通过；新增：有外壳时只有一个 `main`、左栏不在 `main` 里，没有外壳的页面仍有 `main`；有效角色的规则（与服务端 `notebook_role_test.go` 同样的用例表）；注明出现与不出现（工作区成员列表没读到时不注明）。
- 反向对照：`Layout` 不看 `handle`（两个 `main`，测试失败）；有效角色不比较访客（注明错误，测试失败）。

## 完成检查

`make check`（前端的 lint、format、types、knip、vitest）为绿；`make build` 之后 W3、N 系列与成员相关的 e2e 通过。
