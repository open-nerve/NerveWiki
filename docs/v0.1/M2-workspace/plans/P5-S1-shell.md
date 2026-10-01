# M2/P5/S1 外壳：实施计划

上级：[P5 文档](../05-P5-web-shell-workspaces.md) 3.2、3.3、3.4、3.8。

## 任务

1. `services/workspace.service.ts`：`list`、`create`、`rename`、`remove`、`checkSlug`，转出所用的类型。
2. `stores/workspace.store.ts`：`list`、`bySlug`、`load`（丢弃被写答复越过的读）、`create`、`rename`（按 `lower(name), name, id` 排序）、`remove`、`checkSlug`；`RootStore.workspaces`；`useWorkspaces`。
3. `PreferencesStore`：`lastWorkspace()`、`setLastWorkspace(slug)`（`nwiki.workspace`；存储不可用时退回内存）。
4. `app/landing.ts` 的 `landingPath`；`pages/landing.tsx`。
5. 路由：`/` 换成落点；`/:slug` 的 `WorkspaceLayout`（左栏、切换器、导航、404、记下最后访问、`useWorkspace`）与工作区首页；`main` 的 `has-[[data-shell]]:p-0`。
6. `HomePage` 与 `home.*` 删除；用户菜单显示版本。
7. 保留名单：`create-workspace` 移到 `[app]`，路由先挂一个占位（S2 换成创建页），名单测试照旧通过。

## 测试

- `landingPath` 的表格；`PreferencesStore` 的最后访问。
- `WorkspaceStore`：读与写的交错；排序；删除；`RootStore` 新一代的 `workspaces` 从空开始。
- 路由：`/` 的三种落点；`/:slug` 不是成员时显示 404；加载失败与重试。
- 外壳：切换器列出全部、当前选中（单选的菜单项，审查 Q3）、选一项转到它；创建关闭时没有入口；访问之后 `nwiki.workspace` 是这个 slug。
- 用户菜单的版本。
- 反向对照：落点不看最后访问、布局不记下 slug、读覆盖写、不是成员照样渲染，各自的测试失败。

## 完成检查

`make check` 为绿。
