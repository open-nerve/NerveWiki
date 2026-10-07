```yaml
status: open
from: M6 收尾
to: M7
created: 2026-10-06
```

# 链接与附件、导入、导出

M6 的链接只解析到页面（[M6 总设计](../../M6-links/00-M6-design.md) 4.4；[P3 文档](../../M6-links/03-P3-index.md)第 2 节），附件、导入、导出留给 M7。各项的来源写在条目里，[M6 收尾审查](../../M6-links/reviews/M6-closeout-review.md) C-I2、C-I3 汇总。

1. **附件嵌入的渲染：改由 M7 建立**（M6 收尾审查 C-I2；负责人可以改判）。M6 总设计第 8 节原写"M6 建立、交空"，收尾时改判：它的形状取决于附件能被解析，而 M6 的解析只答页面，现在建只能是一个整个程序上走不到的接口。
   - 接缝放在 `obsidian.Options`，与 `Resolve` 并列：`Resolve`（组合根经 linking 的 `ResolveLinks` 给出）要能答"解析到一个附件"，渲染的参数按它写出内联的标记（图片、PDF 等），交空时嵌入照链接渲染（现在的 `nw-embed`）。标记写进 `Markup`，`CheckHTML`、`CheckSize` 与样例集随之更新。
   - 总体设计 12.4 的那一行改为"M7 建立并注册"；最后一跳照总体设计 13.1 第 21 条：经 `markdownExtensions(resolve)` 到达阅读视图，组合根交空时失败。
   - 这一项取代 [M4 的扩展移交](M4-extensions.md)第 1 项"附件内联是独立的扩展"：现在的图片仍是核心的 `<span class="nw-image">`（替代文字，地址可用时加链接，[P3 文档](../../M6-links/03-P3-index.md) B 部分），附件的内联走这里。
2. **附件进解析**（M6 总设计 4.4、[P3 文档](../../M6-links/03-P3-index.md)第 2 节末）：照 Obsidian，最后一段带 `.` 的先找文件名恰好如此的附件，没有再找加上 `.md` 的页，并入"只读作一种"。要改的地方：
   - page 给 linking 的读端口（`page.NewLinkTargets`：`LinkTargets`、`All`）只读页；
   - 接口的 `LinkTargetKind` 只有 `page`（[P5 文档](../../M6-links/05-P5-api.md)：另起名字，M7 改它时不与 `NodeKind` 撞）；
   - 索引的不变式 `checkLinks`（`bootstrap/links_test.go`）只看 `kind = 'page'`；
   - 附件的新建、删除、改名、移动要经观察者重新解析指向它们的链接；改名、移动是否像页那样改写嵌入（Obsidian 会），在 M7 的设计里定。
3. **落点**（[P6 文档](../../M6-links/06-P6-reading-view.md)第 7、15 节）：
   - 让落点拒绝读作附件的目标（答一个新的原因，前端照"没有落点"说明）；
   - 现在同名的附件会让"问落点 → `createPage` 答 409 `page.title_taken` → 再问又是它"一直循环，拒绝之后随之消失；
   - M7 之前 `[[report.pdf]]` 照样新建为页（同 Obsidian 新建同名的笔记），这些页之后照常是页。
4. **导入是多操作的单元**（M6 总设计 4.5）：改写的参与者要求单元开始前的索引是新的，所以单元里在改名、移动之前不做别的写，或者让观察者逐操作运行。导入的页经观察者进索引（每个正文写一次）；在单元里持锁解析时照参与者，不排队地取预算（`TakeNow`，[P2 文档](../../M6-links/02-P2-facts-budget.md)）。导入之后不需要 `nervewiki reindex`，但要 `ANALYZE` 导入写到的表：没有统计时，`LinksReached` 要把笔记本的链接扫一遍，读链接目标的递归查询收尾时改成了按主键逐步读（[M6 收尾审查](../../M6-links/reviews/M6-closeout-review.md) A-M2，`page/adapter/postgres/queries/links.sql`），别的查询仍要靠统计选计划。
5. **导出：没有正文、只有子页、被链接的页**（[P3 文档](../../M6-links/03-P3-index.md)第 2 节，[P4 文档](../../M6-links/04-P4-rewrite.md)，[P3A 审查](../../M6-links/reviews/P3A-index-review.md) C2-L3）：总体设计 3.5 写导出时这种页只有文件夹、不生成空的 `.md`。Obsidian 里没有它这个文件，解析到它的链接（从文件夹出发的那一步，以及文件夹自己的页）在 Obsidian 里解析到别处。M7 定：给被链接的这种页写一个空的 `.md`，或者照旧并写明差异。
6. **最后一跳与测试**：附件嵌入的渲染、附件的解析、导入的索引，各在整个程序上有行为测试（总体设计 13.1 第 21 条），组合根交空时失败；e2e 的故事调用 [`e2e/fixtures/assert/links.ts`](../../../../e2e/fixtures/assert/links.ts) 断言索引。
