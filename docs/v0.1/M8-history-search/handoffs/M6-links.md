```yaml
status: done
from: M6 收尾
to: M8
created: 2026-10-06
```

# 链接索引与恢复、历史、还原

M6 的链接索引由页面领域事件的观察者在写入单元的事务里维护（[M6 总设计](../../M6-links/00-M6-design.md) 4.3、4.5），改名、移动由参与者改写别的页（4.6）。M8 的恢复、历史版本、还原与整组撤销都碰到它（M6 总设计第 7 节"移交的落实"；[M6 收尾审查](../../M6-links/reviews/M6-closeout-review.md) C-I3）。

1. **恢复要带提取结果**：删除子树时，被删页自己的索引行随之删掉。恢复时，观察者要为每一页重建索引，它只认写入带来的提取结果（`Facts`），不自己解析。所以恢复要为每一页解析正文、带上 `Facts`。在单元里持着笔记本的锁解析时，照参与者不排队地取预算（`TakeNow`，取不到答 503 `server_busy`，[P2 文档](../../M6-links/02-P2-facts-budget.md)），不要在锁下排队等预算。
2. **恢复只重新解析，不改写别的页**（M6 总设计 4.4 末）：恢复与新建一样，可能改变别的链接的指向（例如恢复一页之后，原来指向另一处同名页的链接改指它），这些链接经观察者重新解析，正文不动。只有改名与移动"保住指向"。
3. **历史版本的阅读视图即时解析**（M6 总设计 4.7、总体设计 4.3）：旧版本的正文不在索引里，它的链接按当前的树即时解析（`obsidian.Resolve` 对索引里没有的范围逐条即时解析，已经是这样）。
4. **还原、整组撤销碰上改名或移动**：一个变更集里的改名、移动被撤销时，要不要经参与者改写链接（保住指向），在 M8 的设计里定。经参与者时，引用它的页被锁（包括本人在编辑）整个答 409 `linking.pages_locked`，带 `locks` 列出每一页与编辑者（总体设计 6.1）。这个码要进还原、撤销接口的 `x-problem-codes`，前端照改名对话框的写法说明（[P4 文档](../../M6-links/04-P4-rewrite.md)第 6 节）。
5. **最后一跳与不变式**：恢复、还原、撤销各在整个程序上有行为测试：索引随之更新，`links` 事件到达流，结束时核对 `checkLinks`（`bootstrap/links_test.go`）；交错照 `interleavings_links_test.go`。

## 处理进展

- M8/P1（2026-10-13，合并 `12fc5d7`）：第 3 项：版本的阅读视图照 `getPageView` 的渲染路径，链接按当前的树即时解析（[M8 总设计](../00-M8-design.md) 4.3）；第 5 项里还原版本的一跳：`bootstrap/page_versions_test.go` 的 `TestRestoringAVersionThroughServe`（索引随之更新，`links` 事件到达流）。
- M8/P2（2026-10-14，合并 `25d250a`）：第 1 项改判：解析在单元之前、不加锁，按大小从大到小以 `Take` 取预算，各次的等待合计至多 `page.parse_max_wait`，用完答 503（4.6；`page/app/content.go` 的 `DecidedAll`，`page/app/restore_test.go` 的 `TestDecidedAllTakesTheLargestFirstWithinOneWait`），M7 设计审查 A-I2 去掉了锁下的 `TakeNow`；恢复的变更带 `Revision` 与 `Facts`，linking 照新建建索引；写解析的 `SetResolutions` 按参数规划（P2A 实测：缓存的语句的通用计划 53 秒；`linking/adapter/postgres/store_test.go` 的 `TestTheLinksReachedAreReadPlannedWithTheReach`）。第 2 项：`Relocation` 要求"前"不空，恢复只重新解析、不改写（4.1）。第 3 项另：回收站里的页按它被删时的路径解析（`Pages.SourcePath`；`bootstrap/trash_test.go` 的 `TestAPageInTheTrashResolvesFromWhereItWas`）。第 5 项里恢复的一跳：`TestATrashItemGoesAndComesBack`（索引随之更新，`links` 事件到达流）；`checkTrash` 经 `checkPages` 核对 `checkLinks`，交错在 `interleavings_trash_test.go`。
- M8/P3（2026-10-14，合并 `314d9ec`）：第 4 项改判：撤销不经改写的参与者（单元的 `Options` 为空），C 改写过的页逐字节写回，不答 `linking.pages_locked`，`revertChangeset` 的 `x-problem-codes` 不带它，被编辑的页答 `page.locked`（4.8、第 11 节第 4 条，总体设计 4.5；`bootstrap/history_test.go` 的 `TestEachChangeIsRevertedThroughServe`）。第 5 项里撤销的一跳同一测试，`checkHistory` 核对 `checkLinks`，交错在 `interleavings_history_test.go`。
- M8 收尾（2026-10-16）：核对过各项的落实（[M8 收尾审查](../reviews/M8-closeout-review.md) C-I1），关闭。
