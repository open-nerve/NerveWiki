```yaml
status: done
from: M6/P4
to: M9
created: 2026-10-05
```

# 链接改写：批量、关闭改写与大笔记本的改名

M6/P4 让改名、移动改写别的页里的链接（[P4 文档](../../M6-links/04-P4-rewrite.md)；`linking/app/rewrite.go` 的 `Rewrite.Participate`）。M9 的 MCP 是第一个会一次做很多操作、也能关掉改写的入口：

1. **批量里的改名、移动**：参与者的"之前"是单元开始之前的索引。同一个单元里第二个改名、移动看到的之前不准：第一个追加的写还没进索引（P4 文档 4.5）。M9 的 batch 要么每个改名、移动一个单元，要么在设计里写明怎样让单元开始前的索引是新的；加一个"同一单元两次改名、链接各跟着改"的整个程序的测试。
2. **`move` 的 `update_links=false`**：经写入选项 `Options.UpdateLinks` 交给参与者，它什么都不做，锁的预检也跳过，只由观察者重新解析（P4 文档第 1 节）。P4 只有单元测试（网页总是改写）；M9 加整个程序上的测试：关掉时链接不改、被锁的页不拒绝、索引照样维护。
3. **大笔记本的改名**：改写逐页读正文、解析、写回，持着笔记本行。审查时测得：每页 10 KB、各一条链接，500 页 1.27 s，2,500 页 10.3 s，5,000 页到 15 s 的请求期限答 500（P4 文档第 12 节，[P4 审查](../../M6-links/reviews/P4-rewrite-review.md) r2-3）。v0.1 的规模内够用。M9 的批量如果让一次操作改写的页更多，先在设计里给出提速的做法（一条语句读出各页的正文、合并写修订与正文），再放开规模。
4. **`backlinks` 与 `index` 工具**（总体设计 6.3 的工具表；M6 总设计第 7 节；[M6 收尾审查](../../M6-links/reviews/M6-closeout-review.md) C-I3 补记）：两个工具复用 linking 的查询，不另写 SQL。
   - `backlinks` 调反链的查询（`linking/app/backlinks.go`），分页、计数的上限（1000）、上下文的规则与没有上下文的情形同 `listBacklinks`（[P5 文档](../../M6-links/05-P5-api.md)第 3 节）。
   - `index` 按类型、标签筛选时读 `page_properties`、`page_tags`：标签照 `getTag` 含其下的标签，属性按值（jsonb），不在 linking 之外 JOIN 它的表；索引不是当前的页（升级之后、`nervewiki reindex` 之前）不在其中，写明。

## 处理进展

- M9/P2–P4（2026-10-19 至 10-23，合并 `804c3561`、`f19d0b6f`、`ed9e149f`）：第 1 项：单个 `move` 一步（`Unit.Relocate`、`RelocateNode`）；batch 照分开调用：计划经 `linking.NewBatchLinks` 改写 batch 写的页，推后的参与者在单元结束时以合并的改动调用一次，与分开调用的不同照 [M9 总设计](../00-M9-design.md) 4.11 的一条规则；`TestTheLinksOfABatchAreThoseOfSeparateCalls`（M9 收尾加了第 25 例，歧义的链接，`2017c402`）、`TestTheWaysABatchDiffersFromSeparateCalls`。第 2 项：`TestAMoveWithoutItsLinksLeavesThemAndTheIndex`。第 3 项：没有提速，这一项许可：batch 不放开规模，推后的改写在读树与任何解析之前数改写的页（`MaxBatchRewrites` 400，写了的页算 2）、链接（120,000）与字节（34,406,400），过界答 422；单个改名、移动照 M6 不受限，它的风险与一个共用的界线的建议在 [M12 的性能移交](../../M12-release/handoffs/M4-performance.md)第 18 项（4.11、4.14）。第 4 项：`ListBacklinks.AtPath`、`ListIndex` 与 `index.list`（迁移 00047）；`TestTheMCPBacklinksAreRESTs`、`TestTheIndexIsEachPagesReads`、MC7（4.9）。
- M9 收尾（2026-10-23）：核对过各项的落实（[M9 收尾审查](../reviews/M9-closeout-review.md)"handoff"，C 逐项对照代码），关闭。

