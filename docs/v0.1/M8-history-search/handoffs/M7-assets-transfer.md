```yaml
status: done
from: M7 收尾
to: M8
created: 2026-10-10
```

# 附件、导入与恢复、历史、搜索

M7 加了附件节点与它们的文件（[M7 总设计](../../M7-assets-transfer/00-M7-design.md) 4.2、4.6），以及一次写很多节点的导入（4.11）。M8 的恢复、历史、整组撤销与搜索都碰到它们（M7 总设计第 7 节"写给后面的 M"；[M7 收尾审查](../../M7-assets-transfer/reviews/M7-closeout-review.md) C-I1）。

1. **恢复附件的行**：删除子树或笔记本时，asset 的观察者（`asset/app/follow.go` 的 `NodesDeleted`、`NotebooksDeleted`）在同一个事务里把附件的行（`asset_blobs`）记成已删除，时刻与节点相同。恢复节点时要经同一个观察者把这些行恢复回来（按节点 id、同一个删除时刻），否则恢复的附件节点没有文件、下载答 404。整个程序上的行为测试：删除、恢复之后附件照常下载，无主列表的大小算上它们。
2. **恢复与清理的先后**：附件的清理器（`asset.Purgers`，排在 page 之前）以 `FOR UPDATE SKIP LOCKED` 锁住到期的行，先删文件、再删行，一批一个事务（[P2 文档](../../M7-assets-transfer/02-P2-assets-server.md) 3.8）。恢复要先锁住这些行（`FOR UPDATE`）再改：清理器持着时恢复等它，之后行已不在，这个附件恢复不了（文件已删），恢复要说明、不能留下没有行的附件节点；恢复先拿到行时清理器跳过它。交错测试照 `interleavings_assets_test.go` 的写法。
3. **附件没有版本**：附件写入之后不变，要换就删了再传（M7 总设计第 2 节）。历史与还原只看页面的正文；还原到旧版本时，正文里指向已删附件的嵌入照常显示为未解析。
4. **导入的整组撤销**：一次导入是一个类型为 `import` 的变更集，可能有几万个节点；它由很多个单元写成，每个单元至多 100 个节点、8 MiB 正文、20,000 条链接（总体设计 13.1 第 31 条），各单元并入第一个的变更集（13.1 第 2 条）。撤销同样要分成有上限的单元、在单元之间放开笔记本的锁，不能一个事务删几万个节点；撤销途中失败时说明已撤销的部分（[P6 文档](../../M7-assets-transfer/06-P6-import.md)第 8 节）。
5. **撞名时的序号**：恢复或撤销时名称撞上已有的兄弟，若要加序号，附件的序号加在扩展名之前（"a 2.png"），同导入（`page/domain.Numbered`，P6 文档 3.4）。
6. **搜索是否列出附件**：节点名称的三元组索引（M4）包括附件节点。M8 的搜索要定是否列出附件、列出时怎样显示（类型、大小、打开即下载），以及正文的全文搜索只看页面。

## 处理进展

- M8/P1（2026-10-13，合并 `12fc5d7`）：第 3 项：版本只有页面的正文（`page_revisions`），附件的版本不做（[M8 总设计](../00-M8-design.md)第 2 节、4.3；对附件读版本、还原答 404，`page/app/versions_test.go`）；旧版本里指向已删附件的嵌入照常未解析。
- M8/P2（2026-10-14，合并 `25d250a`、`5116194`）：第 1 项：asset 的观察者 `NodesRestored` 按节点 id 与删除的时刻恢复行，行数不符时单元答 409 `page.trash_changed`（`bootstrap/trash_test.go` 的 `TestATrashItemGoesAndComesBack`：新签的地址下载 200；`bootstrap/assets_lifecycle_test.go` 的 `TestTheOwnerlessListShowsTheAttachmentsActivity`：恢复与撤销回来的附件计入大小）。第 2 项改判：不逐表 `FOR UPDATE`，恢复只认删除晚于"现在减保留期加 1 小时"的组，与清理以一小时的间隔分开，保留期的下限改为 1 天（4.11）；清理持着附件的行时恢复等它，删了就 409、什么都不恢复（`bootstrap/interleavings_trash_test.go` 的 `TestRestoringAnAttachmentAPurgeHolds`、`TestRestoringAndPurgingAtTheEndOfThePeriod`）；M7 总设计 4.6 那一句随之改写。第 5 项：恢复撞名加序号，附件加在扩展名之前（4.6；`page/app/restore_test.go` 的 `TestRestoreNodePlacesAndNumbers`、`bootstrap/trash_test.go` 的 `TestARestoreGoesBackWhereItCan`）；撤销不加序号，撞名是冲突 `title_taken`（第 11 节第 6 条）。
- M8/P4（2026-10-15，合并 `2f8077ec`、`947812e6`）：第 4 项：超过一个单元的撤销在后台分批，每批一个单元、批之间放开笔记本的锁，撤销的删除一个最上层一个单元，途中停下时已撤销的留着、报告写明（4.10）；导入另有等于 `MaxJobItems` = 60,000 的节点上限（`too_many_nodes`），能导入的都能撤销（`bootstrap/background_test.go` 的 `TestABackgroundRevertOfAnImportThroughServe`）。后台的恢复同样按 4.6 加序号。
- M8/P5（2026-10-16，合并 `760757aa`、`ed22439a`）：第 6 项：搜索列出附件，只按名称（名称层），带类型与大小；网页打开它所在的位置（父页的附件一节或笔记本首页），不签发下载地址；正文只搜页面（4.12、第 11 节第 10 条；`bootstrap/search_test.go` 的 `TestTheSearchFindsEachWriteAtOnce`）。"名称的三元组索引 M4 已建"与代码不符：只有 `pg_trgm` 扩展，没有索引（M8 设计审查 C-N9）。
- M8 收尾（2026-10-16）：核对过各项的落实（[M8 收尾审查](../reviews/M8-closeout-review.md) C-I1），关闭。
