```yaml
status: done
from: M3
to: M4
created: 2026-10-02
```

# 笔记本留给 M4 的部分

> 已全部处理（2026-10-03）：第 1–3 项在 M4/P5 落实（外壳只有一个 `main`，左栏在它外面；`app/effective-role.ts` 与成员行的有效角色；合并成 `app/member-row.tsx` 与 `app/rename-form.tsx`）；第 4 项的推理不变：页面的写以 `FOR SHARE` 锁工作区行、不改成员行，心跳与结束只碰会话行；第 5 项是 `bootstrap/page_registrants_test.go` 的 `TestTheOwnerlessListShowsThePagesActivity`。经 [M4 收尾审查](../reviews/M4-closeout-review.md)核实。

M3 的审查把几件与页面树一起才定得下的事留给 M4，收尾审查（[M3 收尾审查](../../M3-notebook/reviews/M3-closeout-review.md)）把它们集中在这里。笔记本删除事件的第一个注册者另见[它的移交](M3-P1-notebook-deletion.md)。

1. **外壳的 `main`**：布局的 `<main>`（`web/apps/web/src/app/layout.tsx`）包着工作区的外壳，左栏的 `nav` 因此在 `main` 里；组标题 `h2` 在页面的 `h1` 之前。合法，但页面区域是否单独作为 `main`、左栏是否移到它外面，与页面树的布局一起定（[P4 审查](../../M3-notebook/reviews/P4-web-notebooks-review.md) Q2）。
2. **成员行的有效角色**：笔记本的成员列表显示显式角色；对工作区开放为"可编辑"的笔记本里，列为"读者"的成员实际能编辑。是否在行上提示有效角色，与 M4 的编辑权限一起定（P4 审查 Q3）。
3. **合并重复的组件**：`NotebookMemberRow` 与工作区的 `MemberRow`、工作区与笔记本的两个改名表单形状相同。页面改名时再写一个之前，先把它们合并（P4 审查 N3）。
4. **`LockHoldings` 的前提**：成员身份结束的否决者在锁笔记本行的同一条语句里数有效的管理员与成员（`notebook/adapter/postgres/queries/cascade.sql`），等锁之后用的是语句开始时的快照；计数可信只因为工作区行先挡住了一切成员写。M4 若加只锁笔记本行、又改成员行的写，要重做这一推理（[P3 审查](../../M3-notebook/reviews/P3-cascade-ownerless-review.md) Q6；总体设计 13.1 第 5 条）。
5. **笔记本活动的第一个来源**：只读的扩展点 `NotebookActivitySource`（M3 总设计第 8 节）在调用方的读取里执行，不加锁；M3 在模块根证明来源经 `notebook.New` 到达无主清单（`notebook/activity_test.go`：字节数相加、取最晚的写入、来源出错答 500）。M4 注册页面的来源时，在整个程序上经 `listOwnerlessNotebooks` 加一个行为测试：组合根没交来源时它失败（M3 收尾审查 A-M1）。附件的来源另见 [M7 的移交](../../M7-assets-transfer/handoffs/M3-notebook-activity.md)。
