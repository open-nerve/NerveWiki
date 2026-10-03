# M4/P5/S2 树与页面外壳：实施计划

上级：[P5 文档](../05-P5-tree-reading.md) 3.4–3.6、3.8（不含增强）。

## 任务

1. `services/page.service.ts`；`stores/page-tree.ts` 的纯函数；`stores/page-tree.store.ts`（读、队列、重读、`changesAnswered`、`removed`）；`RootStore.pagesOf(notebook)`。
2. 路由 `notebooks/:id/pages/:pageId`（懒加载）；`PageLayout`（找到才显示、本标签页删掉的经 `removedTo` 去父页或首页、不在树里 404）；面包屑、`h1`、子页面列表。
3. 左栏的页面树（只读）：`nav`、嵌套列表、展开按钮、当前页、祖先自动展开、按笔记本重新挂载、`NotLoaded`。
4. 笔记本首页：根下的页面列表。
5. `ReadingView`：挂上服务端 HTML（`["page-view", id]`），`NotLoaded` 与重读；增强的管线在 S4。
6. 文案。

## 测试

- store：读、按代按笔记本缓存、`removed`；纯函数的用例表。
- 组件：树的展开与当前页、祖先自动展开、换笔记本重新挂载；外壳找不到页面时 404、树没读到时 `NotLoaded`；面包屑与子页面列表；首页列表；阅读视图的 `NotLoaded` 与重读显示别处的改动。
- 反向对照：外壳不等树就显示；树不按笔记本重新挂载（展开状态串到别的笔记本）。

## 完成检查

`make check` 为绿。
