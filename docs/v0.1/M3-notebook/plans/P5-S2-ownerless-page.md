# M3/P5/S2 无主笔记本页：实施计划

上级：[P5 文档](../05-P5-web-ownerless.md) 3.3。

## 任务

1. `app/routes.tsx`：`/:slug/settings/ownerless`；`pages/workspace/settings-layout.tsx`：第三项只给工作区管理员。
2. `pages/workspace/ownerless-page.tsx`、`ownerless-row.tsx`：清单（各列、空、`NotLoaded`）；接管（一次一个、焦点、状态与打开的链接、重读笔记本与审计）；删除（输入名称、`focusAfter`）；404 与 403 的拒绝。成员与访客看到说明。
3. `pages/workspace/audit-section.tsx`：三种句子、时刻、加载更多与它的失败、空。
4. `pages/workspace/workspace-home.tsx`：管理员的无主提醒。
5. 文案。

## 测试

- 无主页：列、空、接管、删除、404、403、读不到与重读；成员与访客。
- 审计：句子、加载更多、失败与重试、空、读不到与重读。
- 首页提醒：有、无、成员不读（不发请求）。
- 设置导航：管理员三项，成员与访客两项。
- 反向对照：导航给成员第三项 → 测试失败；首页提醒给成员读 → 测试失败；接管之后不重读笔记本列表 → 左栏的断言失败。

## 完成检查

`make check` 为绿。

## 实现注记

- 接管以答复放进笔记本列表（`NotebookStore.receive`），不重读。
- 审查修复（P5 审查 M1、N2、N3、m1、m2）：404 时另重读审计与工作区列表，三处读答 403 时重读工作区列表（`app/follow-role.ts`）；按钮的名称带原所有者；假服务端的接管答接管之后的成员数；双击、按工作区的测试。
