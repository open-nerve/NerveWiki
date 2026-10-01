# M3/P5/S4 端到端：实施计划

上级：[P5 文档](../05-P5-web-ownerless.md) 3.7；[M3 总设计](../00-M3-design.md)第 3 节 N7–N11、N13。

## 任务

1. `e2e/fixtures/ownerless-pages.ts`：无主页的清单、接管、删除、审计（含加载更多），各返回请求的答复。
2. 故事的页面版本：N7–N11、N13；N8 带首页提醒。断言与接口版本共用 `assert/notebook.ts`。

## 测试

每个故事的页面版本；页面安静（13.4 第 3 条）。反向对照：接管之后不重读笔记本列表 → N9 失败；离开工作区的文案去掉 → N7 失败。

## 完成检查

`make e2e` 全部通过。

## 实现注记

- 另加 `anotherPage`（`fixtures/test.ts`）与 `newOnboardedTeam`（`fixtures/workspaces.ts`）；N2 的第二个标签页改用 `anotherPage`。
- N8 的停用拒绝单独成测试；N10 按审查 Q2 改为先经"加载更多"读完 51 条再删除。
- 反向对照另有：无主页渲染时 `console.warn` → N7 经 `anotherPage` 的安静检查失败。
