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
- P1/S2（2026-10-25，合并见 [00 号文档](../00-M10-design.md)第 12 节）：模式的守卫与保护的页，撤销的每一步照它本来的操作判（第 2 项），守卫的码进各操作的 `x-problem-codes`（第 3 项的最后一条）。守卫 `page.NewModeGuard` 排在编辑锁之前，在每一步上以单元在笔记本行的锁下读的模式与来源根（`Write.Notebook`）判 `CheckContract`：撤销的放回照普通的改名、移动判，撤销的删除照删除判（`AGENTS` 不能改名、移动、删除，来源根不能删除，写者撤销管理员对 `raw` 的改名答 `page.source_admin_only`）；守卫拒绝时同步的撤销整组拒绝（`TestTheModeGuardReachesEveryPath`：撤销模板的变更集答 `page.protected`，写者撤销来源根的改名答 403、管理员的成功）；`page.protected`、`page.source_admin_only` 进 `revertChangeset` 的 `x-problem-codes`，网页有文字。S2 与 S3 之间同步的撤销没有契约的预检、后台的撤销被拒时以 `internal` 停下（[01-P1](../01-P1-contract-frame.md) 第 0 节的窗口）：预检与后台的去处（`refused`）在 P1 的 S3，第 3 项的来源的四个问题是 P2/S1 的；仍是 `open`。
- P1/S3（2026-10-26，合并见 [00 号文档](../00-M10-design.md)第 12 节）：预检与后台的拒绝的去处（`refused`）；上传的 `Check` 与撤销的两遍共用 `CheckContract`，`startRevert` 的开始时预检和任务的计划拒绝照同一套码，批里的守卫拒绝经 `revertDefect`、`treeError` 进入报告的 `refusal`（码与节点）。`TestPrechecksAnswerTheGuardsCodes` 核对同码和编辑锁/模式拒绝的混合形状，`TestTheSourceRootsPutBackIsRefusedInALaterBatch` 核对第一批保留、第二批 refused；B-M1 的共用 finalizer 修复让 timeout/interrupted/cancelled 的终态清掉 refusal，四行回归与第 86 条变异核对。第 3 项的来源的四个问题是 P2/S1 的，去重是 P2/S2 的；仍是 `open`。
- P1/S4（2026-10-27，合并见 [00 号文档](../00-M10-design.md)第 12 节）：网页通过持有的对话框和串行 store 执行已有降级；LW12 的排队导入守住 `transfer.busy`，不扩大历史/来源的服务端范围。S3 的拒绝报告不变；来源四个问题与后续历史事项仍归 P2–P5，本移交保持 `open`。
- P2/S1（2026-10-27，合并见[00号文档](../00-M10-design.md)第12节）：P2/S1补rewrite持久/全撤销链、version_restore/session不可改来源、来源管理与临时步骤规则、同步原子回滚/后台refused，后台事实按compact/名义批界/实际同组动作，与同步Ops分开。AfterParents表达恢复森林；C-M1正文位置与C-M2删除管理位置第一组b750已提交并增量核验；C-M3逐步管理位置为第二组，各ID最终原C复核见[S1审查](../reviews/P2-S1-source-rules-review.md)。S1 main `bf8d6625daba554ee467aef2003346459657a060`/CI `40501936360` 与记录 `6a4f33b8fb621c66d97d38b3fea4fea0adbfc565`/CI `40506120884` 均十项成功；两main三份Test均值328/313.5/394 s已核，S2已释放，不能替作S2 CI。source_duplicate服务端增量已验，最终S2签字与S3网页解释仍待完成；本移交保持open，P2/S3验收后才done，不沿用旧P2–P5笼统文字推迟闭合。
- P2/S2：source duplicate、起始预检与后台refused/同步rollback增量已核；请求/worker预检、首批/非零前批后guard分层。决定43保留名义预检不搜索其他分法、缩批可无并发改变来源结果且先前提交保留。S2最终验收与S3网页解释未完成，本移交open。 代码已推进至 `33b4da5a`，本机与精确分支11项CI已验；B-N1原审已关闭，main集成未决状态见[P2 §10.2](../02-P2-source-area.md#102-s2-来源-hash去重与验证)与[S2审查](../reviews/P2-S2-source-hash-review.md)。
