# M3/P4/S2 左栏与笔记本外壳：实施计划

上级：[P4 文档](../04-P4-web-notebooks.md) 3.3、3.4；[M2 移交](../handoffs/M2-workspace.md)第 5 项。

## 任务

1. `pages/workspace/workspace-layout.tsx`：外层改为不带地标的容器；`nav` 里是首页、设置与 `NotebookNav`。去掉 `workspace.sidebar`。
2. `pages/workspace/notebook-nav.tsx`：两组（标题与有名称的列表）、空的组不显示、两组都空时的说明；"新建笔记本"只给管理员与成员。
3. `pages/workspace/create-notebook-dialog.tsx`、`app/notebook-name.ts`：名称（必填、255 字节）、开放程度（缺省私密）、规则的提示；成功之后关闭并进入新笔记本（到达）。
4. `pages/workspace/workspace-home.tsx`：两组的卡片、空时的说明与新建。
5. `app/routes.tsx`：`notebooks/:id` 与它的子路由；`pages/notebook/notebook-layout.tsx`（`useNotebook`、找到、404、删除或离开之后回到工作区首页）、`notebook-home.tsx`（主标题接受到达）、`settings-layout.tsx`。
6. 文案。

## 测试

- 左栏：两组与次序、空的组不显示、访客没有"新建"、读不到时说明与重试、再读一次显示别处的变化（13.2 第 7 条）。
- 新建对话框：本地检查、422 在字段下方、成功之后进入新笔记本且主标题取得焦点。
- 笔记本外壳：找到、404、`wasRemoved` 回到工作区首页并聚焦；从一本直接到另一本，子页重新挂载。
- 反向对照：访客也给"新建" → 测试失败；分组只看 `workspace_access` → 测试失败。

## 完成检查

`make check` 为绿。

## 实现注记

- 两组由共用的 `pages/workspace/notebook-groups.tsx` 画出（左栏与首页）。
- `settings-layout.tsx` 与首页的设置链接在 S3 随两页一起做；外壳的 `wasRemoved` 测试经 S3 的删除与离开。
- 新建之后焦点的测试记录 `focusin`：新首页的路由还在加载时，Radix 的焦点归还先于主标题挂载，只看最后的焦点测不出缺了 `onCloseAutoFocus`。
- 测试替身的 `byRoute`：`*` 改为一段路径；`signedInApp` 对每个工作区的笔记本缺省答空列表。
