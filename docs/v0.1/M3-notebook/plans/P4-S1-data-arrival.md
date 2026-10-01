# M3/P4/S1 数据与到达：实施计划

上级：[P4 文档](../04-P4-web-notebooks.md) 3.2、3.5；13.2 第 1、15、17 条。

## 任务

1. `services/notebook.service.ts`：`NotebookService`（list、create、get、update、remove、leave），类型由它转出（`Notebook`、`NotebookCreate`、`NotebookUpdate`、`WorkspaceAccess`、`NotebookRole`）。`services/notebook-member.service.ts`：`NotebookMemberService`（list、add、update、remove），转出 `NotebookMember`。
2. `stores/order.ts`：`byName`（从 `workspace.store.ts` 移出），工作区与笔记本共用。
3. `stores/notebook.store.ts`：`NotebookStore`（load、create、update、remove、leave、bySlug→`byId`、`wasRemoved`）；`groupNotebooks`。`stores/notebook-member.store.ts`：`NotebookMemberStore`（load、add、changeRole、remove）。
4. `stores/root.store.ts`：`notebooksOf(workspace)`、`notebookMembersOf(notebook)`；`stores/context.tsx`：`useNotebooks`、`useNotebookMembers`。
5. `app/arrival.ts`：`arrived`、`useArrivalFocus()`。工作区外壳、`LandingPage` 转交、创建工作区页与工作区首页的主标题、邀请页的接受。

## 测试

- store 的写按表格驱动（13.2 第 1 条）：重叠的读丢弃；放进列表的写去重、排序；`remove` 答 404 `notebook.not_found` 当作完成、答 403 保留并抛出；`leave` 之后再读答 404 移出并记下、答 200 换掉那一项；成员的 `remove` 答 404 `notebook.member_not_found` 当作完成。
- `groupNotebooks` 的表格（私密且一人、私密两人、开放一人、开放多人）。
- `root.store.test.ts`：两个缓存的同一与不同。
- 到达：删除、离开工作区之后落点的主标题取得焦点（经 `LandingPage` 的转交）；接受邀请之后同样；没有到达状态时不抢焦点。
- 反向对照：`leave` 不再读 → 测试失败；删除答 404 时不移出 → 测试失败；`LandingPage` 不转交 → 焦点测试失败。

## 完成检查

`make check` 为绿。
