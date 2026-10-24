```yaml
status: open
from: M8 收尾
to: M10
created: 2026-10-16
```

# 来源区的守卫与撤销、还原、恢复

M8 让写入守卫看得到一个单元是不是撤销、还原或恢复（[M8 总设计](../../M8-history-search/00-M8-design.md) 4.1"守卫看得到撤销"；[M8 设计审查](../../M8-history-search/reviews/M8-design-review.md) A-M13、C-M10）：M10 的来源区要把它们与普通的写分开（[总体设计](../../v0.1-design.md) 5.4："来源只能新增，不能修改正文。删除和移动来源只允许笔记本管理员做……系统的链接改写不算修改"）。M8 总设计把这一项写在"写给后面的 M"，没有写成移交，M8 收尾补上（[M8 收尾审查](../../M8-history-search/reviews/M8-closeout-review.md) C-I2）。

1. **守卫看到的**：`page/app/extension.go` 的 `Write`（守卫的 `Step` 与观察者都带着它）有 `Kind`（这个单元的变更集的类型）与 `RevertsID`（撤销的是哪一个变更集，只在撤销时不为零）。类型有五种（迁移 `00034` 的 `changesets_kind_check`）：`edit`（普通的写）、`import`（导入的单元）、`version_restore`（还原一个版本）、`trash_restore`（从回收站恢复，同步的与后台的每一批）、`revert`（整组撤销，同步的与后台的每一批）。
2. **撤销的每一步都经守卫**：撤销在一个单元里放回（改名与移动改回）、删除 C 新建的、恢复 C 删除的、写回正文，每一步照它本来的操作（`OpMove`、`OpRename`、`OpDelete`、`OpRestore`、`OpContent`）经守卫，带着 `Kind` = `revert`、`RevertsID` = C（`page/extension_test.go` 的 `TestTheObserversFollowARevert` 钉住）；后台撤销的每一批同样（[M8/P4](../../M8-history-search/04-P4-background.md) 3.8）。撤销不经链接改写的参与者：C 是改名或移动、改写过别的页的链接时，撤销把那些页的正文逐字节写回（M8 总设计 4.8），在守卫看来是 `revert` 里的一次 `OpContent`，不是参与者的改写。
3. **要 M10 定的**：来源区的规则碰上这几种写时怎么判，写进 M10 的设计，在整个程序上各有行为测试：
   - 撤销一次改名或移动，写回 C 改写过的来源页（C 的改写是系统的链接改写，不算修改）：撤销的写回算不算"修改来源"；
   - 还原一个来源页的旧版本（`version_restore` 的 `OpContent`）：算修改来源，还是来源区一律不给还原；
   - 从回收站恢复来源页或来源的附件（`trash_restore` 的 `OpRestore`）、撤销一次删除：算"新增"，还是同删除一样要管理员；"内容哈希与已有来源相同时拒绝放入"对恢复是否适用；
   - 撤销一次新建来源（删掉它）或一次移动：照删除、移动只允许管理员，还是撤销另算；
   - 守卫拒绝时整个单元回滚：同步的撤销与恢复整组拒绝，后台的任务在那一批停下；被拒的码进这些操作的 `x-problem-codes`，网页照说明。

## 处理进展

- M10 开工（2026-10-24）：照[M10 总设计](../00-M10-design.md)第 7 节"移交的落实"的"M8 来源区的守卫与撤销、还原、恢复"一行做：守卫与保护的页在 P1 的 S2，预检与后台的拒绝的去处在 P1 的 S3，来源的规则与 `Step.Rewrite` 在 P2 的 S1，去重在 P2 的 S2（4.6、4.7）。仍是 `open`，做完的 Phase 改为 `done`，收尾核对。
