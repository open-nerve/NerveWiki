```yaml
status: open
from: M2/P4
to: M7
created: 2026-10-01
```

# 附件的清理

M2/P4 建了软删除的清理注册表（总体设计 12.4、13.1 第 6 条；[M2/P4 文档](../../M2-workspace/04-P4-deactivation-commands-purge.md) 3.4）。附件的行背后有卷上的文件，所以它的清理器多一步。

M7 加附件的清理器时：

1. **先删文件，再删行**：行一旦删掉，就没有东西指向那个文件了。附件的行因此绝不能被父行的 `ON DELETE CASCADE` 连带删掉，否则文件留在卷上（M2 总设计第 10 节的风险）。附件的父表在别的模块（节点或笔记本），父表的清理器看不到附件的表，所以附件指向它的外键用 `ON DELETE RESTRICT`（总体设计 13.1 第 6 条，[M2 收尾审查](../../M2-workspace/reviews/M2-closeout-review.md) A-I1）；附件的清理器排在它的父表之前，父行仍被引用时父表的那一批删除失败，之后的运行再完成。
2. **事务边界**：删文件不在数据库的事务里。先删文件、后删行，中途失败时最多留下指向已删文件的行，下一次运行再删一次文件（不存在就算成功）就能清掉；反过来会留下没有行的文件。测试覆盖"文件删了、行没删"之后的下一次运行。
3. M3 的移交（[M3/handoffs/M2-workspace.md](../../M3-notebook/handoffs/M2-workspace.md)第 4 项）里清理器的其余约束同样适用。

## 处理进展

- M7/P2（2026-10-09，合并 `48c62c0`）：三项都已落实。附件的行经复合外键 `ON DELETE RESTRICT` 指向节点，`asset.Purgers` 排在 page 之前；一批一个事务，先删文件、再删行，删不掉的文件让这一批失败；"文件已删、行没删"之后的下一次运行由 `TestThePurgeDeletesTheAttachmentsFilesThenRows` 守住；没有行的文件由每天的孤儿清扫删掉（[P2 文档](../02-P2-assets-server.md) 3.8）。M7 收尾时关闭。
