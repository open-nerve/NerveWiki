# M4/P1/S5 端到端：实施计划

上级：[P1 文档](../01-P1-page-module-pipeline.md) 3.14。

## 任务

1. `e2e/fixtures/pages.ts`：`createPage`、`renameNode`、`getPage`、`listNodes`（成功即返回数据的与只返回答复的两种）。
2. `e2e/fixtures/assert/page.ts`：`expectNewPage`、`expectRenamed`、`expectPagesDeletedWith`，时刻在 SQL 里比较。
3. `e2e/fixtures/purge.ts`：从 `n13-workspace-deletion.spec.ts` 移出"把删除推到保留期之前"，加上页面的表，从叶到根推；n13 改用它。
4. 故事（`e2e/stories/page/`，接口版本）：`pg1-create-page`、`pg2-rename-page`、`pg11-navigation`、`pg12-page-permissions`、`pg13-notebook-deletion-pages`（P1 文档 3.14 的内容）。

## 测试

- 单个 spec：`make build`，再 `cd e2e && NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/page/<spec>`；最后 `make e2e` 跑全部故事。
- 反向对照：`expectNewPage` 不核对客户端（把断言改成 `web`，接口版本失败）；`purge.ts` 不推页面的表（PG13 的清理等不到）。

## 完成检查

`make e2e` 全部通过；e2e 的 lint、format、types 与 `make knip` 为绿。
